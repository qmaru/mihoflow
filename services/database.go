package services

import (
	"database/sql"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Config struct {
	ListenAddr   string
	ClashURL     string
	ClashAPIKey  string
	ClashTimeout time.Duration
	DBPath       string
	Debug        bool
	CollectEvery time.Duration
	FlushEvery   time.Duration
	HistoryDays  int
	CleanupDays  int
}

type Service struct {
	cfg    Config
	db     *sql.DB
	client *http.Client
	ui     fs.FS

	mu            sync.RWMutex
	connections   map[string]Connection
	devices       map[string]*Device
	pendingClosed []Connection
	date          string

	sseMu   sync.Mutex
	clients map[chan []byte]struct{}
}

type Connection struct {
	ID          string             `json:"id"`
	Chains      []string           `json:"chains"`
	ChainValue  string             `json:"chainValue"`
	Upload      int64              `json:"upload"`
	Download    int64              `json:"download"`
	Metadata    ConnectionMetadata `json:"metadata"`
	Rule        string             `json:"rule"`
	RulePayload string             `json:"rulePayload"`
	Start       string             `json:"start"`
	RecordDate  string             `json:"recordDate,omitempty"`
	ClosedAt    string             `json:"closedAt,omitempty"`
}

type ConnectionMetadata struct {
	DestinationIP   string `json:"destinationIP"`
	DestinationPort string `json:"destinationPort"`
	DNSMode         string `json:"dnsMode"`
	Host            string `json:"host"`
	Network         string `json:"network"`
	ProcessPath     string `json:"processPath"`
	SourceIP        string `json:"sourceIP"`
	SourcePort      string `json:"sourcePort"`
	Type            string `json:"type"`
}

type Device struct {
	UploadToday       int64 `json:"uploadToday"`
	DownloadToday     int64 `json:"downloadToday"`
	UploadSpeed       int64 `json:"uploadSpeed"`
	DownloadSpeed     int64 `json:"downloadSpeed"`
	ActiveConnections int   `json:"activeConnections"`
}

type DeviceResponse struct {
	IP string `json:"ip"`
	*Device
}

type EventPayload struct {
	Devices     []DeviceResponse `json:"devices"`
	Connections []Connection     `json:"connections"`
}

func New(cfg Config, ui fs.FS) (*Service, error) {
	if cfg.ClashTimeout <= 0 {
		cfg.ClashTimeout = 5 * time.Second
	}
	if cfg.HistoryDays < 1 {
		cfg.HistoryDays = 90
	}
	if cfg.CleanupDays < cfg.HistoryDays {
		cfg.CleanupDays = 97
	}
	db, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	service := &Service{
		cfg: cfg, db: db, client: &http.Client{Timeout: cfg.ClashTimeout}, ui: ui, date: today(),
		connections: make(map[string]Connection), devices: make(map[string]*Device),
		pendingClosed: make([]Connection, 0), clients: make(map[chan []byte]struct{}),
	}
	if err := service.initDB(); err != nil {
		db.Close()
		return nil, err
	}
	if err := service.loadToday(); err != nil {
		db.Close()
		return nil, err
	}
	if err := service.cleanup(); err != nil {
		log.Printf("startup cleanup: %v", err)
	}
	log.Printf("mihoflow started")
	log.Printf("server listen address: %s", cfg.ListenAddr)
	log.Printf("server web: http://%s/ui", cfg.ListenAddr)
	log.Printf("debug mode: %t", cfg.Debug)
	log.Printf("clash url: %s", service.normalizeURL())
	log.Printf("clash endpoint: GET %s/connections", service.normalizeURL())
	log.Printf("clash request timeout: %s", cfg.ClashTimeout)
	log.Printf("database path: %s", cfg.DBPath)
	log.Printf("collect interval: %s", cfg.CollectEvery)
	log.Printf("flush interval: %s", cfg.FlushEvery)
	log.Printf("history retention: %d days", cfg.HistoryDays)
	log.Printf("cleanup retention: %d days", cfg.CleanupDays)
	return service, nil
}

func (s *Service) Close() error { return s.db.Close() }

func (s *Service) debugf(format string, args ...any) {
	if s.cfg.Debug {
		log.Printf("DEBUG "+format, args...)
	}
}

func today() string { return time.Now().Format("2006-01-02") }

func (s *Service) normalizeURL() string { return strings.TrimRight(s.cfg.ClashURL, "/") }

func (s *Service) initDB() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS connection_details (
		id TEXT PRIMARY KEY, record_date TEXT NOT NULL, closed_at TEXT NOT NULL DEFAULT '',
		start TEXT NOT NULL DEFAULT '', chains TEXT NOT NULL DEFAULT '[]', chain_value TEXT NOT NULL DEFAULT '',
		upload INTEGER NOT NULL DEFAULT 0, download INTEGER NOT NULL DEFAULT 0,
		destination_ip TEXT NOT NULL DEFAULT '', destination_port TEXT NOT NULL DEFAULT '', dns_mode TEXT NOT NULL DEFAULT '',
		host TEXT NOT NULL DEFAULT '', network TEXT NOT NULL DEFAULT '', process_path TEXT NOT NULL DEFAULT '',
		source_ip TEXT NOT NULL DEFAULT '', source_port TEXT NOT NULL DEFAULT '', type TEXT NOT NULL DEFAULT '',
		rule TEXT NOT NULL DEFAULT '', rule_payload TEXT NOT NULL DEFAULT ''
	)`)
	return err
}

func (s *Service) loadToday() error {
	rows, err := s.db.Query(`SELECT source_ip, COALESCE(SUM(upload),0), COALESCE(SUM(download),0) FROM connection_details WHERE record_date = ? GROUP BY source_ip`, s.date)
	if err != nil {
		return err
	}
	defer rows.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	for rows.Next() {
		var ip string
		var d Device
		if err := rows.Scan(&ip, &d.UploadToday, &d.DownloadToday); err != nil {
			return err
		}
		if ip != "" {
			s.devices[ip] = &d
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	activeRows, err := s.db.Query(`SELECT id, record_date, upload, download, source_ip FROM connection_details WHERE closed_at = ''`)
	if err != nil {
		return err
	}
	defer activeRows.Close()
	for activeRows.Next() {
		var c Connection
		if err := activeRows.Scan(&c.ID, &c.RecordDate, &c.Upload, &c.Download, &c.Metadata.SourceIP); err != nil {
			return err
		}
		s.connections[c.ID] = c
	}
	return activeRows.Err()
}

func (s *Service) flush() error {
	s.mu.Lock()
	values := make([]Connection, 0, len(s.connections)+len(s.pendingClosed))
	for _, c := range s.connections {
		values = append(values, c)
	}
	closedCount := len(s.pendingClosed)
	values = append(values, s.pendingClosed...)
	s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO connection_details (id, record_date, closed_at, start, chains, chain_value, upload, download, destination_ip, destination_port, dns_mode, host, network, process_path, source_ip, source_port, type, rule, rule_payload) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET record_date=excluded.record_date, closed_at=excluded.closed_at, start=excluded.start, chains=excluded.chains, chain_value=excluded.chain_value, upload=excluded.upload, download=excluded.download, destination_ip=excluded.destination_ip, destination_port=excluded.destination_port, dns_mode=excluded.dns_mode, host=excluded.host, network=excluded.network, process_path=excluded.process_path, source_ip=excluded.source_ip, source_port=excluded.source_port, type=excluded.type, rule=excluded.rule, rule_payload=excluded.rule_payload`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, c := range values {
		chains, _ := json.Marshal(c.Chains)
		if _, err = stmt.Exec(c.ID, c.RecordDate, c.ClosedAt, c.Start, string(chains), c.ChainValue, c.Upload, c.Download, c.Metadata.DestinationIP, c.Metadata.DestinationPort, c.Metadata.DNSMode, c.Metadata.Host, c.Metadata.Network, c.Metadata.ProcessPath, c.Metadata.SourceIP, c.Metadata.SourcePort, c.Metadata.Type, c.Rule, c.RulePayload); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.mu.Lock()
	if closedCount > len(s.pendingClosed) {
		closedCount = len(s.pendingClosed)
	}
	s.pendingClosed = s.pendingClosed[closedCount:]
	s.mu.Unlock()
	s.debugf("flush: saved %d connection records", len(values))
	return nil
}

func (s *Service) cleanup() error {
	cutoff := time.Now().AddDate(0, 0, -s.cfg.CleanupDays).Format("2006-01-02")
	if _, err := s.db.Exec(`DELETE FROM connection_details WHERE record_date < ?`, cutoff); err != nil {
		return err
	}
	if _, err := s.db.Exec(`VACUUM`); err != nil {
		return err
	}
	s.debugf("cleanup: removed records before %s and vacuumed database", cutoff)
	return nil
}

func (s *Service) rollover(newDate string) error {
	if err := s.flush(); err != nil {
		return err
	}
	s.mu.Lock()
	s.connections = make(map[string]Connection)
	s.devices = make(map[string]*Device)
	s.pendingClosed = nil
	s.date = newDate
	s.mu.Unlock()
	return s.cleanup()
}

func scanConnections(rows *sql.Rows) ([]Connection, error) {
	result := make([]Connection, 0)
	for rows.Next() {
		var c Connection
		var chains string
		if err := rows.Scan(&c.ID, &c.RecordDate, &c.ClosedAt, &c.Start, &chains, &c.ChainValue, &c.Upload, &c.Download, &c.Metadata.DestinationIP, &c.Metadata.DestinationPort, &c.Metadata.DNSMode, &c.Metadata.Host, &c.Metadata.Network, &c.Metadata.ProcessPath, &c.Metadata.SourceIP, &c.Metadata.SourcePort, &c.Metadata.Type, &c.Rule, &c.RulePayload); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(chains), &c.Chains); err != nil {
			c.Chains = []string{}
		}
		result = append(result, c)
	}
	return result, rows.Err()
}
