package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"
)

func (s *Service) serve(ctx context.Context) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/devices", s.handleDevices)
	mux.HandleFunc("/api/connections", s.handleConnections)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/events", s.handleEvents)
	if s.ui != nil {
		uiHandler := http.FileServerFS(s.ui)
		mux.Handle("/ui/", http.StripPrefix("/ui/", uiHandler))
		mux.HandleFunc("/ui", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/ui/", http.StatusTemporaryRedirect)
		})
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/ui/", http.StatusTemporaryRedirect)
		})
	}
	server := &http.Server{Addr: s.cfg.ListenAddr, Handler: mux}
	s.debugf("listening on %s, collecting %s/connections", server.Addr, s.normalizeURL())
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("http server: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func (s *Service) handleDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	writeJSON(w, s.snapshotDevices())
}

func (s *Service) handleConnections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	days := queryDays(r, s.cfg.HistoryDays, s.cfg.CleanupDays)
	cutoff := dateDaysAgo(days - 1)
	ip := r.URL.Query().Get("ip")

	connectionSelect := `SELECT id, record_date, closed_at, start, chains, chain_value, upload, download, destination_ip, destination_port, dns_mode, host, network, process_path, source_ip, source_port, type, rule, rule_payload FROM connection_details`
	query := connectionSelect + ` WHERE record_date >= ?`
	args := []any{cutoff}
	if ip != "" {
		query += ` AND source_ip = ?`
		args = append(args, ip)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	defer rows.Close()
	result, err := scanConnections(rows)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	writeJSON(w, s.mergeConnectionDetails(result, cutoff, ip))
}

func (s *Service) mergeConnectionDetails(records []Connection, cutoff, ip string) []Connection {
	merged := make(map[string]Connection, len(records))
	for _, c := range records {
		merged[c.ID] = c
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, source := range []map[string]Connection{s.connections, connectionSliceToMap(s.pendingClosed)} {
		for id, c := range source {
			if c.RecordDate < cutoff || (ip != "" && c.Metadata.SourceIP != ip) {
				continue
			}
			merged[id] = c
		}
	}
	result := make([]Connection, 0, len(merged))
	for _, c := range merged {
		result = append(result, c)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].RecordDate == result[j].RecordDate {
			return result[i].Start > result[j].Start
		}
		return result[i].RecordDate > result[j].RecordDate
	})
	return result
}

func connectionSliceToMap(values []Connection) map[string]Connection {
	result := make(map[string]Connection, len(values))
	for _, c := range values {
		result[c.ID] = c
	}
	return result
}

func (s *Service) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	days := queryDays(r, s.cfg.HistoryDays, s.cfg.CleanupDays)
	query := `SELECT record_date, source_ip, COALESCE(SUM(upload),0), COALESCE(SUM(download),0), COUNT(*) FROM connection_details WHERE record_date >= ?`
	args := []any{dateDaysAgo(days - 1)}
	if ip := r.URL.Query().Get("ip"); ip != "" {
		query += ` AND source_ip = ?`
		args = append(args, ip)
	}
	query += ` GROUP BY record_date, source_ip ORDER BY record_date, source_ip`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		http.Error(w, "database error", 500)
		return
	}
	defer rows.Close()
	type historyRow struct {
		Date        string `json:"date"`
		IP          string `json:"ip"`
		Upload      int64  `json:"upload"`
		Download    int64  `json:"download"`
		Connections int64  `json:"connections"`
	}
	result := make([]historyRow, 0)
	for rows.Next() {
		var item historyRow
		if err := rows.Scan(&item.Date, &item.IP, &item.Upload, &item.Download, &item.Connections); err != nil {
			http.Error(w, "database error", 500)
			return
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "database error", 500)
		return
	}
	writeJSON(w, result)
}

func (s *Service) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch := make(chan []byte, 2)
	s.sseMu.Lock()
	s.clients[ch] = struct{}{}
	s.sseMu.Unlock()
	defer func() { s.sseMu.Lock(); delete(s.clients, ch); close(ch); s.sseMu.Unlock() }()
	initial := EventPayload{Devices: s.snapshotDevices(), Connections: s.connectionSnapshot()}
	if data, err := json.Marshal(initial); err == nil {
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case data := <-ch:
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (s *Service) publish(payload EventPayload) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	s.sseMu.Lock()
	defer s.sseMu.Unlock()
	for ch := range s.clients {
		select {
		case ch <- data:
		default:
		}
	}
}

func (s *Service) snapshotDevicesLocked() []DeviceResponse {
	result := make([]DeviceResponse, 0, len(s.devices))
	for ip, d := range s.devices {
		copy := *d
		result = append(result, DeviceResponse{ip, &copy})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].IP < result[j].IP })
	return result
}

func (s *Service) snapshotDevices() []DeviceResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshotDevicesLocked()
}

func (s *Service) connectionSnapshotLocked() []Connection {
	result := make([]Connection, 0, len(s.connections))
	for _, c := range s.connections {
		result = append(result, c)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (s *Service) connectionSnapshot() []Connection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connectionSnapshotLocked()
}

func queryDays(r *http.Request, fallback, max int) int {
	days, err := strconv.Atoi(r.URL.Query().Get("days"))
	if err != nil || days < 1 {
		return fallback
	}
	if days > max {
		return max
	}
	return days
}

func dateDaysAgo(days int) string { return time.Now().AddDate(0, 0, -days).Format("2006-01-02") }
