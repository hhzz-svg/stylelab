# Style Lab

Single-binary web app for extracting, fusing, and editing novel style cards.

## Run

```bash
STYLELAB_DEV_INSECURE_KEY=1 go run ./cmd/stylelab
```

Then open `http://localhost:8080/api/health`.

`STYLELAB_DEV_INSECURE_KEY=1` uses a 32-byte zero master key and is for local/test only. Production must set `STYLELAB_MASTER_KEY` to 64 hex characters.

## Config

| Env | Default | Notes |
|---|---|---|
| `STYLELAB_ADDR` | `:8080` | Listen address |
| `STYLELAB_DATA_DIR` | `./data` | SQLite and blobs |
| `STYLELAB_MASTER_KEY` | required | 64 hex chars (32 bytes) |
| `STYLELAB_DEV_INSECURE_KEY` | off | `1` uses 32 zero bytes (test-only) |
| `STYLELAB_DEV_AUTO_LOGIN` | off | `1` enables auto-login |
| `STYLELAB_WORKERS` | `2` | In-process job concurrency |
