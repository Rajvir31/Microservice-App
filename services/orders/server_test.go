package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/reliability-lab/gen/orders"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestCreateOrder_Idempotency(t *testing.T) {
	ctx := context.Background()
	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
	)
	if err != nil {
		t.Fatalf("postgres: %v", err)
	}
	defer func() { _ = pgContainer.Terminate(ctx) }()

	host, err := pgContainer.Host(ctx)
	if err != nil {
		t.Fatalf("host: %v", err)
	}
	port, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("port: %v", err)
	}
	connStr := fmt.Sprintf("postgres://test:test@%s:%s/testdb?sslmode=disable", host, port.Port())

	db, err := initDB(ctx, connStr)
	if err != nil {
		t.Fatalf("initDB: %v", err)
	}
	defer db.Close()

	srv := &ordersServer{db: db}
	idemKey := "idem-test-123"
	req := &orders.CreateOrderRequest{
		UserId:         "u1",
		AmountCents:    1299,
		Currency:       "USD",
		IdempotencyKey: idemKey,
	}

	resp1, err := srv.CreateOrder(ctx, req)
	if err != nil {
		t.Fatalf("first CreateOrder: %v", err)
	}
	resp2, err := srv.CreateOrder(ctx, req)
	if err != nil {
		t.Fatalf("second CreateOrder: %v", err)
	}

	if resp1.OrderId != resp2.OrderId {
		t.Errorf("order_id mismatch: first=%q second=%q", resp1.OrderId, resp2.OrderId)
	}
	if resp1.Status != resp2.Status {
		t.Errorf("status mismatch: first=%q second=%q", resp1.Status, resp2.Status)
	}

	var count int
	err = db.QueryRow(ctx, "SELECT COUNT(*) FROM orders WHERE idempotency_key = $1", idemKey).Scan(&count)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row for idempotency_key, got %d", count)
	}
}
