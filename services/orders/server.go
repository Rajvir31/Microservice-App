package main

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reliability-lab/gen/orders"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ordersServer struct {
	orders.UnimplementedOrdersServer
	db *pgxpool.Pool
}

func (s *ordersServer) CreateOrder(ctx context.Context, req *orders.CreateOrderRequest) (*orders.CreateOrderResponse, error) {
	ctx, span := otel.Tracer("orders").Start(ctx, "CreateOrder")
	defer span.End()

	if req.UserId == "" || req.AmountCents <= 0 || req.Currency == "" || req.IdempotencyKey == "" {
		return nil, status.Error(grpccodes.InvalidArgument, "missing or invalid required fields")
	}

	start := time.Now()
	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	// INSERT with ON CONFLICT DO UPDATE SET status = orders.status to force RETURNING the existing row
	q := `INSERT INTO orders (id, user_id, amount_cents, currency, status, idempotency_key, created_at)
	      VALUES ($1, $2, $3, $4, 'CREATED', $5, $6)
	      ON CONFLICT (idempotency_key) DO UPDATE SET status = orders.status
	      RETURNING id, status`
	var outID, outStatus string
	err := s.db.QueryRow(ctx, q, id, req.UserId, req.AmountCents, req.Currency, req.IdempotencyKey, now).Scan(&outID, &outStatus)
	dbQueryDurationSeconds.WithLabelValues("create_order").Observe(time.Since(start).Seconds())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, status.Error(grpccodes.Internal, "failed to create order")
	}
	return &orders.CreateOrderResponse{OrderId: outID, Status: outStatus}, nil
}

func (s *ordersServer) GetOrder(ctx context.Context, req *orders.GetOrderRequest) (*orders.GetOrderResponse, error) {
	ctx, span := otel.Tracer("orders").Start(ctx, "GetOrder")
	defer span.End()

	if req.OrderId == "" {
		return nil, status.Error(grpccodes.InvalidArgument, "order_id required")
	}

	start := time.Now()
	q := `SELECT id, user_id, amount_cents, currency, status, idempotency_key, created_at FROM orders WHERE id = $1`
	var id, userID, currency, orderStatus, idemKey, createdAt string
	var amountCents int64
	err := s.db.QueryRow(ctx, q, req.OrderId).Scan(&id, &userID, &amountCents, &currency, &orderStatus, &idemKey, &createdAt)
	dbQueryDurationSeconds.WithLabelValues("get_order").Observe(time.Since(start).Seconds())
	if err != nil {
		if err == pgx.ErrNoRows {
			span.SetStatus(codes.Error, "not found")
			return nil, status.Error(grpccodes.NotFound, "order not found")
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, status.Error(grpccodes.Internal, "failed to get order")
	}
	return &orders.GetOrderResponse{
		OrderId:        id,
		UserId:         userID,
		AmountCents:    amountCents,
		Currency:       currency,
		Status:         orderStatus,
		IdempotencyKey: idemKey,
		CreatedAt:      createdAt,
	}, nil
}

func initDB(ctx context.Context, connStr string) (*pgxpool.Pool, error) {
	ctx, span := otel.Tracer("orders").Start(ctx, "initDB")
	defer span.End()

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, err
	}
	q := `CREATE TABLE IF NOT EXISTS orders (
		id UUID PRIMARY KEY,
		user_id TEXT NOT NULL,
		amount_cents INT NOT NULL,
		currency TEXT NOT NULL,
		status TEXT NOT NULL,
		idempotency_key TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL
	);
	CREATE UNIQUE INDEX IF NOT EXISTS orders_idempotency_key_key ON orders (idempotency_key);`
	_, err = pool.Exec(ctx, q)
	if err != nil {
		pool.Close()
		return nil, err
	}
	span.SetAttributes(attribute.String("db", "ready"))
	return pool, nil
}

var _ orders.OrdersServer = (*ordersServer)(nil)
