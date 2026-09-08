# Deployment

Two entrypoints share the same pipeline (`src/app`):

- `cmd/report` — CLI, prints the report and exits.
- `cmd/server` — HTTP handler, `POST /stockfolio/report/generate` runs the same thing and returns the text as JSON. `GET /health` answers `ok` for health checks.

## Configuration

| Variable | Required | Default | Notes |
| --- | --- | --- | --- |
| `GOOGLE_CREDENTIALS_JSON` | one of the two | — | The service account JSON itself, for containers with no file mount |
| `GOOGLE_CREDENTIALS_PATH` | one of the two | — | Path to the service account JSON. Ignored when `GOOGLE_CREDENTIALS_JSON` is set |
| `SHEET_ID` | yes | — | Google Sheet used as source of truth |
| `HISTORY_DIR` | no | `./history` | Weekly snapshots, needed for week-over-week deltas |
| `PORT` | no | `8080` | Server only |
| `DISCORD_ENABLED` | no | `false` | Set to `true` to post the report |
| `DISCORD_WEBHOOK_URL` | if enabled | — | Discord webhook |
| `IMPORTS_DIR` | no | `./imports` | Broker CSVs, `cmd/import_csv` only |

The service account, however it is provided, must have access to the sheet. The CLI reads `.env` and fails without it. The server prefers the environment and only warns when `.env` is missing, so a container needs no file.

`isin_map.json` at the working directory maps an ISIN to its Yahoo symbol (`"FR0013412269": "PANX.PA"`). It is optional: unknown ISINs fall back to a Yahoo symbol search, but an entry pins the listing and saves a request.

## Local run

```bash
go build -o bin/report ./cmd/report
./bin/report
```

Run it from the repository root so `.env`, `isin_map.json` and `history/` resolve.

## Docker

```bash
docker build -t stockfolio .

docker run -d --name stockfolio -p 8080:8080 \
  -e GOOGLE_CREDENTIALS_JSON="$(cat credentials.json)" \
  -e SHEET_ID=your_sheet_id \
  -e DISCORD_ENABLED=true \
  -e DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/xxx/yyy \
  -v $PWD/isin_map.json:/app/isin_map.json:ro \
  -v stockfolio-history:/app/history \
  stockfolio
```

Credentials, the ISIN map and the snapshots stay out of the image. If you mount the credentials as a file instead, `GOOGLE_CREDENTIALS_PATH` must point inside the container, not at a host path. Mount `history/` as a volume or every run starts with no previous snapshot and reports no deltas.

## Reverse proxy

```caddyfile
stockfolio.example.com {
    reverse_proxy localhost:8080
}
```

The endpoint has no authentication of its own: anyone who can reach it triggers a run, a sheet write and a Discord post. Keep it off the public internet, or put a check in front of it — Caddy `basic_auth`, an IP filter, or a token header.

## Coolify

Coolify builds the Dockerfile straight from the repository and puts its own proxy and TLS in front, so the reverse proxy section above only applies outside Coolify.

1. **New Resource → Application →** your Git repository, branch `main`.
2. **Build Pack:** `Dockerfile`. **Ports Exposes:** `8080`.
3. **Environment Variables:** everything from the table above, plus `HISTORY_DIR=/app/history`. Paste the whole service account JSON into `GOOGLE_CREDENTIALS_JSON` as a secret — one line, no file mount, nothing to go missing on redeploy.
4. **Storages → Add File Mount** (optional): `/app/isin_map.json` with the ISIN map. Skip it and unknown ISINs resolve through Yahoo search instead.
5. **Storages → Add Volume Mount:** any name, mounted at `/app/history`. Without it the snapshots die with each deployment and every report shows no deltas.
6. **Health Check:** path `/health`, port `8080`.
7. **Domains:** set the FQDN, Coolify issues the certificate.

Redeploys wipe the container filesystem, so anything that must survive one belongs in a file or volume mount.

**Scheduled Tasks** then replaces an external scheduler — add one on the application with the container command:

```bash
wget -q -O- --post-data='' http://localhost:8080/stockfolio/report/generate
```

Frequency `0 8 * * 1` runs it every Monday at 08:00.

## Scheduling

Anything that can send a POST works — cron, a CI schedule, n8n:

```bash
curl -X POST https://stockfolio.example.com/stockfolio/report/generate
```
