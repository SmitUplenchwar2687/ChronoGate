# ChronoGate

ChronoGate is a production-style HTTP API that validates Chrono's public SDK end-to-end.

Chrono SDK packages integrated in this project:

- `github.com/SmitUplenchwar2687/Chrono/pkg/cli`
- `github.com/SmitUplenchwar2687/Chrono/pkg/clock`
- `github.com/SmitUplenchwar2687/Chrono/pkg/config`
- `github.com/SmitUplenchwar2687/Chrono/pkg/limiter`
- `github.com/SmitUplenchwar2687/Chrono/pkg/recorder`
- `github.com/SmitUplenchwar2687/Chrono/pkg/replay`
- `github.com/SmitUplenchwar2687/Chrono/pkg/server`
- `github.com/SmitUplenchwar2687/Chrono/pkg/storage`

## 1) Setup

```bash
go mod tidy
go test ./...
```

## 2) Run API Server

```bash
ALGORITHM=token_bucket RATE=5 WINDOW=10s BURST=5 ADDR=:8080 go run ./cmd/chronogate serve
```

Default env values if omitted:

- `ALGORITHM=token_bucket`
- `RATE=10`
- `WINDOW=1m`
- `BURST=10`
- `ADDR=:8080`

## 3) API Endpoints

- `GET /health` (unlimited)
- `GET /public` (unlimited)
- `GET /api/profile` (rate-limited)
- `POST /api/orders` (rate-limited)
- `GET /api/recordings/export` (export captured request traffic as JSON)
- `GET|PUT|POST /api/storage/demo` (memory storage demo for read/write/increment/expiry)

### Rate-limit behavior

Protected routes use key resolution:

1. `X-API-Key`
2. first IP in `X-Forwarded-For`
3. client IP from `RemoteAddr`

On deny (`429`), response includes JSON and headers:

- `X-RateLimit-Limit`
- `X-RateLimit-Remaining`
- `X-RateLimit-Reset` (Unix epoch seconds)
- `Retry-After`

## 4) Quick Manual Checks

```bash
curl -i http://localhost:8080/health
curl -i http://localhost:8080/public
curl -i -H 'X-API-Key: client-a' http://localhost:8080/api/profile
curl -i -X POST -H 'X-API-Key: client-a' -H 'Content-Type: application/json' -d '{"item":"book"}' http://localhost:8080/api/orders
```

Expected:

- first protected requests return `200/201`
- over-limit protected requests return `429`
- `/health` and `/public` always return `200`

## 5) Load Test Script (Concurrent)

```bash
./scripts/load.sh
CONCURRENCY=50 REQUESTS=500 API_KEY=client-a ./scripts/load.sh
ROUTE=/api/orders METHOD=POST CONCURRENCY=20 REQUESTS=200 ./scripts/load.sh
```

`load.sh` prints `2xx`, `429`, `other`, elapsed time, and RPS.

## 6) Export Recordings

After traffic is generated:

```bash
curl -s http://localhost:8080/api/recordings/export > recordings.json
```

Expected output file: JSON array of records with `timestamp`, `key`, and `endpoint`.

## 7) Replay Recorded Traffic

Using CLI command:

```bash
go run ./cmd/chronogate replay --file recordings.json --algorithm token_bucket --rate 5 --window 10s --burst 5 --speed 0
```

Using script:

```bash
FILE=recordings.json ALGORITHM=token_bucket RATE=5 WINDOW=10s BURST=5 SPEED=0 ./scripts/replay.sh
```

Expected replay summary output includes:

- `Total`
- `Replayed`
- `Allowed`
- `Denied`
- `Per-key` breakdown

You can filter replay input:

```bash
go run ./cmd/chronogate replay --file recordings.json --keys client-a,client-b --endpoints /api/profile
```

## 8) Storage Demo Endpoint

Write with TTL:

```bash
curl -s -X PUT http://localhost:8080/api/storage/demo \
  -H 'Content-Type: application/json' \
  -d '{"key":"feature_flag","value":"on","ttl":"5s"}'
```

Read:

```bash
curl -s 'http://localhost:8080/api/storage/demo?key=feature_flag'
```

Increment counter:

```bash
curl -s -X POST http://localhost:8080/api/storage/demo \
  -H 'Content-Type: application/json' \
  -d '{"key":"orders_counter","delta":1,"ttl":"1m"}'
```

If you read after TTL expiry, `exists` becomes `false`.

## 9) Optional Embedded Chrono Server Mode

Run ChronoGate and Chrono SDK server side-by-side:

```bash
go run ./cmd/chronogate serve --embed-chrono --chrono-addr :9090
```

Compare behavior:

```bash
curl -i -H 'X-API-Key: client-a' http://localhost:8080/api/profile
curl -i "http://localhost:9090/api/check/client-a"
```

This lets you compare ChronoGate middleware responses versus Chrono server decisions directly.

## 10) Chrono CLI Passthrough

ChronoGate exposes Chrono's public CLI as a subcommand:

```bash
go run ./cmd/chronogate chrono-sdk --help
```
