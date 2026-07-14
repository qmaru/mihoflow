# Mihoflow Agent Guide

## Project scope

Mihoflow is a small Go backend for collecting Clash/Mihomo `/connections` data. Keep the implementation lightweight:

- Go standard library, `database/sql`, and `github.com/mattn/go-sqlite3`
- No web framework, ORM, Redis, queue, authentication, or WebSocket
- Backend code belongs in `services/`
- Future embedded frontend assets belong in `ui/`

## Backend responsibilities

- `main.go`: read environment variables and start the service.
- `services/clash.go`: poll `/connections`, calculate per-connection deltas, update device totals/speeds, and publish SSE snapshots.
- `services/database.go`: own the single `connection_details` SQLite table, periodic UPSERT, retention cleanup, and `VACUUM`.
- `services/server.go`: expose the HTTP API and SSE stream.

The collector must remain the source of truth for live traffic. A connection is identified by `id`; its complete details should be retained where practical. `chainValue` uses only the first and last entries of `chains`, joined with ` -> `.

## API contract

`GET /api/devices` returns current device summaries:

```json
[
  {
    "ip": "192.168.1.10",
    "uploadToday": 0,
    "downloadToday": 0,
    "uploadSpeed": 0,
    "downloadSpeed": 0,
    "activeConnections": 0
  }
]
```

`GET /api/connections?days=30` returns connection details. The default range is controlled by `HISTORY_DAYS`; the response can be grouped by IP, host, rule, `chainValue`, network, or any metadata field.

`GET /api/history?ip=192.168.1.10&days=30` returns traffic totals grouped by date and source IP. Both query parameters are optional.

`GET /api/events` is an SSE stream. Each event is a complete snapshot:

```json
{
  "devices": [],
  "connections": []
}
```

The frontend should use SSE for live updates and reconnect when the stream closes.

## Minimum frontend

The future UI only needs to render:

1. A device summary area showing IP, today upload/download, current upload/download speed, and active connections.
2. A connection table showing at least `sourceIP`, `host`, `destinationIP`, `chainValue`, upload, download, and start time.
3. A simple grouping control for fields such as IP, host, rule, and `chainValue`.
4. A history view using `/api/history`, with optional IP and day-range filters.
5. Connection status and a basic empty/loading/error state.

Do not add dashboard-specific backend endpoints for these views. Build grouping and presentation from `/api/connections` and `/api/events`.

## Configuration

Supported environment variables are documented in `README.md` and `.env.example`:

`LISTEN_ADDR`, `CLASH_URL`, `CLASH_API_KEY`, `DB_PATH`, `DEBUG`, `COLLECT_INTERVAL`, `FLUSH_INTERVAL`, `HISTORY_DAYS`, and `CLEANUP_DAYS`.

The application does not load `.env` automatically; use `source .env` before running locally.

## Validation

Run after backend changes:

```bash
gofmt -w main.go services/*.go
go test ./...
go vet ./...
```

Keep API field names stable and avoid unrelated refactors.
