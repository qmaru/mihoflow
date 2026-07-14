package services

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type clashConnections struct {
	Connections   []Connection `json:"connections"`
	DownloadTotal int64        `json:"downloadTotal"`
	UploadTotal   int64        `json:"uploadTotal"`
}

func (s *Service) Run(ctx context.Context) error {
	go s.collect(ctx)
	go s.flushLoop(ctx)
	go s.serve(ctx)
	<-ctx.Done()
	return s.flush()
}

func (s *Service) collect(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.CollectEvery)
	defer ticker.Stop()
	s.poll(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.poll(ctx)
		}
	}
}

func (s *Service) poll(ctx context.Context) {
	s.mu.RLock()
	currentDate := s.date
	s.mu.RUnlock()
	if current := today(); current != currentDate {
		if err := s.rollover(current); err != nil {
			log.Printf("date rollover: %v", err)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.normalizeURL()+"/connections", nil)
	if err != nil {
		log.Printf("connections request: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.ClashAPIKey)
	resp, err := s.client.Do(req)
	if err != nil {
		log.Printf("connections request: %v", err)
		return
	}
	var payload clashConnections
	err = json.NewDecoder(resp.Body).Decode(&payload)
	resp.Body.Close()
	if err != nil {
		log.Printf("connections response: %v", err)
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("connections returned %s", resp.Status)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	s.mu.Lock()
	for _, d := range s.devices {
		d.UploadSpeed = 0
		d.DownloadSpeed = 0
		d.ActiveConnections = 0
	}
	seen := make(map[string]struct{}, len(payload.Connections))
	for _, item := range payload.Connections {
		if item.ID == "" || item.Metadata.SourceIP == "" {
			continue
		}
		item.ChainValue = chainValue(item.Chains)
		item.RecordDate = s.date
		seen[item.ID] = struct{}{}
		old, exists := s.connections[item.ID]
		up, down := item.Upload-old.Upload, item.Download-old.Download
		if !exists || up < 0 {
			up = item.Upload
		}
		if !exists || down < 0 {
			down = item.Download
		}
		d := s.devices[item.Metadata.SourceIP]
		if d == nil {
			d = &Device{}
			s.devices[item.Metadata.SourceIP] = d
		}
		d.UploadToday += up
		d.DownloadToday += down
		d.UploadSpeed += up
		d.DownloadSpeed += down
		s.connections[item.ID] = item
	}
	for id, old := range s.connections {
		if _, ok := seen[id]; !ok {
			old.ClosedAt = now
			s.pendingClosed = append(s.pendingClosed, old)
			delete(s.connections, id)
		}
	}
	for _, c := range s.connections {
		if d := s.devices[c.Metadata.SourceIP]; d != nil {
			d.ActiveConnections++
		}
	}
	connections := s.connectionSnapshotLocked()
	devices := s.snapshotDevicesLocked()
	s.mu.Unlock()
	s.debugf("poll: connections=%d devices=%d totals(up=%d down=%d)", len(connections), len(devices), payload.UploadTotal, payload.DownloadTotal)
	s.publish(EventPayload{Devices: devices, Connections: connections})
}

func chainValue(chains []string) string {
	switch len(chains) {
	case 0:
		return ""
	case 1:
		return chains[0]
	default:
		return chains[0] + " -> " + chains[len(chains)-1]
	}
}

func (s *Service) flushLoop(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.FlushEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.flush(); err != nil {
				log.Printf("flush: %v", err)
			}
		}
	}
}
