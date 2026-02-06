# Reliability Lab - Step 2
# Run from repo root. Compose file is in deploy/compose.

COMPOSE_FILE := deploy/compose/docker-compose.yml
GATEWAY_URL ?= http://localhost:8080

.PHONY: gen up down logs demo load test lint

gen:
	@docker run --rm -v "$$(pwd):/workspace" -w /workspace/proto bufbuild/buf:latest generate 2>/dev/null || \
	 (echo "If on Windows: docker run --rm -v \"%cd%:/workspace\" -w /workspace/proto bufbuild/buf:latest generate"; exit 1)

up:
	docker compose -f $(COMPOSE_FILE) up --build

down:
	docker compose -f $(COMPOSE_FILE) down -v

logs:
	docker compose -f $(COMPOSE_FILE) logs -f gateway

demo:
	@echo "=== 1) POST /orders with idempotency_key=demo-123 ==="
	@curl -s -X POST $(GATEWAY_URL)/orders -H "Content-Type: application/json" -d "{\"user_id\":\"u123\",\"amount_cents\":1299,\"currency\":\"USD\",\"idempotency_key\":\"demo-123\"}" | tee /tmp/demo1.json
	@echo ""
	@echo "=== 2) Repeat same POST (idempotent - same order_id) ==="
	@curl -s -X POST $(GATEWAY_URL)/orders -H "Content-Type: application/json" -d "{\"user_id\":\"u123\",\"amount_cents\":1299,\"currency\":\"USD\",\"idempotency_key\":\"demo-123\"}" | tee /tmp/demo2.json
	@echo ""
	@echo "=== 3) GET /orders/{id} ==="
	@OID=$$(cat /tmp/demo1.json | grep -o '"order_id":"[^"]*"' | cut -d'"' -f4); curl -s "$(GATEWAY_URL)/orders/$$OID"

load:
	k6 run -e GATEWAY_URL=$(GATEWAY_URL) -e K6_DURATION=$${K6_DURATION:-60} -e K6_VUS=$${K6_VUS:-5} loadtest/k6/orders.js

test:
	go test ./services/... ./gen/...

lint:
	golangci-lint run --config golangci-lint.yml ./services/... ./gen/...
