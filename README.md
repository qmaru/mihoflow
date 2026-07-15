# Mihoflow

## Build

```shell
# build ui
cd ui
pnpm build

# build server
go build
```

## Environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `LISTEN_ADDR` | `:8080` | HTTP listen address |
| `CLASH_URL` | `http://192.168.46.2:9090` | Clash/Mihomo API URL |
| `CLASH_API_KEY` | empty | External Controller API key |
| `CLASH_TIMEOUT` | `5s` | Clash HTTP request timeout |
| `DB_PATH` | `mihoflow.db` | SQLite database path |
| `DEBUG` | `false` | Enable debug logs |
| `COLLECT_INTERVAL` | `1s` | `/connections` polling interval |
| `FLUSH_INTERVAL` | `5m` | Database flush interval |
| `HISTORY_DAYS` | `90` | Default query range in days |
| `CLEANUP_DAYS` | `97` | Delete records older than this many days |

The application does not load `.env` automatically. Load it before starting:

```bash
source .env
go run .
```

## Embedded UI

The Go binary embeds the Vite production build from `ui/dist`. Build the UI before
building or running the backend:

```bash
cd ui
pnpm build
cd ..
go run .
```

Open `http://127.0.0.1:8080/ui/`. The root path redirects to `/ui/`.

## API

### `GET /api/devices`

Returns current device traffic totals, speeds, and active connection counts.

```bash
curl http://127.0.0.1:8080/api/devices
```

### `GET /api/connections`

Returns connection details from the last `HISTORY_DAYS` days. Use `days` to override the range.

```bash
curl 'http://127.0.0.1:8080/api/connections?days=30'
```

The response includes `chains`, `chainValue`, `metadata`, `rule`, `start`, and traffic fields.

### `GET /api/history`

Returns traffic totals grouped by date and source IP. The `ip` and `days` query parameters are optional.

```bash
curl 'http://127.0.0.1:8080/api/history?ip=192.168.46.10&days=30'
```

### `GET /api/events`

SSE stream with the complete real-time snapshot:

```bash
curl -N http://127.0.0.1:8080/api/events
```

```json
{
  "devices": [],
  "connections": []
}
```
