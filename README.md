# Reliability Lab

Microservice lab with observability stack (Postgres, OTel, Prometheus, Grafana, Tempo, Loki, Promtail). Full business logic: gateway REST API, orders (Postgres + idempotency), payments (idempotency + fault injection), notifications, and k6 load testing.

## Prerequisites

- Docker and Docker Compose
- (Optional) Go 1.22+ and golangci-lint for local `make test` / `make lint`

## Quick start

From the repo root:

```bash
cp .env.example .env   # optional; defaults work
make up
```

## Step 1 Verification

### 1. Bring up the stack

```bash
make up
```

Expect all containers to build and start. Default ports:

| Service        | Port(s)   |
|----------------|-----------|
| gateway        | 8080      |
| orders         | 50051, 8081 |
| payments       | 50052, 8082 |
| notifications  | 50053, 8083 |
| postgres       | 5432      |
| grafana        | 3000      |
| prometheus     | 9090      |
| tempo          | 3200      |
| loki           | 3100      |

### 2. Confirm containers are running

```bash
docker compose -f deploy/compose/docker-compose.yml ps
```

All services should be `Up` (or `running`). Optional: `docker compose -f deploy/compose/docker-compose.yml logs gateway` to tail gateway logs.

### 3. URLs

- **Grafana:** http://localhost:3000 (default login: admin / admin)
- **Prometheus:** http://localhost:9090
- **Tempo:** http://localhost:3200 (e.g. /ready)
- **Loki:** http://localhost:3100 (e.g. /ready)

### 4. Open the dashboard

1. Open http://localhost:3000 and log in (admin / admin).
2. Go to **Dashboards** (left menu) → **Reliability Lab - Overview**.
3. Panels should load (gateway request rate, service stub counters). If no data yet, hit http://localhost:8080/ once and refresh.

### 5. Health and metrics

- Gateway: http://localhost:8080/healthz , http://localhost:8080/readyz , http://localhost:8080/metrics
- Orders: http://localhost:8081/healthz , http://localhost:8081/metrics
- Payments: http://localhost:8082/healthz , http://localhost:8082/metrics
- Notifications: http://localhost:8083/healthz , http://localhost:8083/metrics

### 6. Tear down

```bash
make down
```

---

**Defaults (documented):** Postgres user `reliability`, password `reliability_secret`, DB `reliability_lab`. Grafana admin/admin. OTel collector at `http://otel-collector:4317`. Service discovery uses compose names: `orders:50051`, `payments:50052`, `notifications:50053`.

---

## Code generation (`make gen`)

Proto-generated Go code lives in `gen/` and is committed. To regenerate (requires Docker):

```bash
make gen
```

This runs `buf generate` in a container from `proto/` and writes to `gen/`. No local `protoc` needed. See `proto/buf.gen.yaml` and `scripts/gen.sh` / `scripts/gen.ps1`.

---

## Step 2 Verification

### 1. Generate and bring up

```bash
make gen   # optional if gen/ already present
make up
```

### 2. Make demo (idempotency and GET)

```bash
make demo
```

Example: first `POST /orders` with `idempotency_key=demo-123` returns e.g. `{"order_id":"...", "order_status":"CREATED", "payment_success":true, "payment_code":"APPROVED"}`. Repeating the same POST returns the **same** `order_id`. Then `GET /orders/{id}` returns the order JSON. On Windows use PowerShell or WSL; or run the three steps manually:

```bash
curl -s -X POST http://localhost:8080/orders -H "Content-Type: application/json" -d "{\"user_id\":\"u123\",\"amount_cents\":1299,\"currency\":\"USD\",\"idempotency_key\":\"demo-123\"}"
# repeat same curl; order_id should match
curl -s http://localhost:8080/orders/<order_id_from_above>
```

### 3. Grafana and dashboard

- **Grafana:** http://localhost:3000 (admin / admin)
- **Dashboard:** **Reliability Lab - Overview** (Dashboards → Reliability Lab - Overview)
- Panels: Gateway RPS, Gateway 5xx rate (%), Gateway p95 latency, Payments decline rate (%), Orders create rate. Generate traffic with `make load` or repeated `make demo` to see data.

### 4. View a trace in Tempo

1. Send a request: `curl -s -X POST http://localhost:8080/orders -H "Content-Type: application/json" -d "{\"user_id\":\"u1\",\"amount_cents\":1000,\"currency\":\"USD\",\"idempotency_key\":\"trace-demo\"}"`
2. In Grafana, open **Explore** (compass icon), choose **Tempo**.
3. Search by **Trace ID** or **Service Name** (e.g. `gateway`). Use the trace ID from gateway logs if needed, or query by time range and service `gateway` to find the trace.

### 5. Correlated logs in Loki

In Grafana **Explore** → **Loki**, use a query to find receipt logs:

```logql
{container_name=~"notifications.*"} |= "receipt_sent"
```

Or by trace ID (from Tempo):

```logql
{topic="notifications"} | json | trace_id="<trace_id>"
```

Adjust labels (e.g. `service=notifications` or `container_name`) to match your Promtail scrape config.

### 6. Trigger payments faults

Env vars (defaults): `PAYMENTS_LATENCY_MS=0`, `PAYMENTS_ERROR_RATE=0.0`, `PAYMENTS_FORCE_FAIL=false`.

- **Fixed latency:** e.g. 500 ms before each charge:
  ```bash
  PAYMENTS_LATENCY_MS=500 docker compose -f deploy/compose/docker-compose.yml up -d payments
  ```
  Or set in `.env`: `PAYMENTS_LATENCY_MS=500` then `docker compose -f deploy/compose/docker-compose.yml up -d payments`.

- **Random decline rate:** e.g. 30% of charges return DECLINED:
  ```bash
  PAYMENTS_ERROR_RATE=0.3 docker compose -f deploy/compose/docker-compose.yml up -d payments
  ```

- **Force all declines:**
  ```bash
  PAYMENTS_FORCE_FAIL=true docker compose -f deploy/compose/docker-compose.yml up -d payments
  ```

Restart payments with new env:

```bash
docker compose -f deploy/compose/docker-compose.yml up -d --force-recreate payments
```

Then run `make demo` or POST /orders again; with `PAYMENTS_FORCE_FAIL=true` you should see `payment_success: false`, `payment_code: "DECLINED"`.

### 7. Load test

```bash
make load
```

Runs k6 against `POST /orders` (default 60s, 5 VUs; override with `K6_DURATION`, `K6_VUS`). Requires [k6](https://k6.io/) installed. Gateway URL defaults to `http://localhost:8080` (set `GATEWAY_URL` if needed).

### 8. Tests

```bash
make test
```

- **Payments:** unit test for Charge idempotency (same idempotency_key returns same result).
- **Orders:** integration test for CreateOrder idempotency (same order_id, single DB row); requires **Docker** (testcontainers-go). Run from repo root so that `gen` and `services/orders` are in the module path.
