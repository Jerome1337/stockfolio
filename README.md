# Stockfolio

Create report of your stock portfolio using Google Sheet as source of truth.

Use CSV of your broker to create and update the portfolio, generate weekly / monthly / yearly report of you assets.

Automatize using n8n or whatever and get the report directly in your own Discord channel.

## Details

### Supported brokers

- Degiro
- Trade Republic

### Supported Output

- Raw export

### Supported Currencies

- EUR
- USD

### Supported Alerting

- Discord


## Self-hosting

Two entrypoints share the same pipeline (`src/app`):

- `cmd/report` — CLI, prints the report and exits.
- `cmd/server` — HTTP handler, `POST /stockfolio/report/generate` runs the same thing and returns the text as JSON.

### Configuration

| Variable | Required | Default | Notes |
| --- | --- | --- | --- |
| `GOOGLE_CREDENTIALS_PATH` | yes | — | Service account JSON, must have access to the sheet |
| `SHEET_ID` | yes | — | Google Sheet used as source of truth |
| `HISTORY_DIR` | no | `./history` | Weekly snapshots, needed for week-over-week deltas |
| `PORT` | no | `8080` | Server only |
| `DISCORD_ENABLED` | no | `false` | Set to `true` to post the report |
| `DISCORD_WEBHOOK_URL` | if enabled | — | Discord webhook |
| `IMPORTS_DIR` | no | `./imports` | Broker CSVs, `cmd/import_csv` only |

The CLI reads `.env` and fails without it. The server prefers the environment and only warns when `.env` is missing, so a container needs no file.

`isin_map.json` at the working directory maps an ISIN to its Yahoo symbol (`"FR0013412269": "PANX.PA"`). It is optional: unknown ISINs fall back to a Yahoo symbol search, but an entry pins the listing and saves a request.

### Local run

```bash
go build -o bin/report ./cmd/report
./bin/report
```

Run it from the repository root so `.env`, `isin_map.json` and `history/` resolve.

### Docker

```bash
docker build -t stockfolio .

docker run -d --name stockfolio -p 8080:8080 \
  -e GOOGLE_CREDENTIALS_PATH=/app/credentials.json \
  -e SHEET_ID=your_sheet_id \
  -e DISCORD_ENABLED=true \
  -e DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/xxx/yyy \
  -v $PWD/credentials.json:/app/credentials.json:ro \
  -v $PWD/isin_map.json:/app/isin_map.json:ro \
  -v stockfolio-history:/app/history \
  stockfolio
```

Credentials, the ISIN map and the snapshots stay out of the image and are mounted at runtime. `GOOGLE_CREDENTIALS_PATH` must point inside the container, not at a host path. Mount `history/` as a volume or every run starts with no previous snapshot and reports no deltas.

### Reverse proxy

```caddyfile
stockfolio.example.com {
    reverse_proxy localhost:8080
}
```

The endpoint has no authentication of its own: anyone who can reach it triggers a run, a sheet write and a Discord post. Keep it off the public internet, or put a check in front of it — Caddy `basic_auth`, an IP filter, or a token header.

### Scheduling

Anything that can send a POST works — cron, a CI schedule, n8n:

```bash
curl -X POST https://stockfolio.example.com/stockfolio/report/generate
```


## Contributing

Pull requests are welcome.

## License

[MIT](https://choosealicense.com/licenses/mit/)
