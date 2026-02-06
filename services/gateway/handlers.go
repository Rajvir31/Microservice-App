package main

import (
	"context"
	"encoding/json"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/reliability-lab/gen/notifications"
	"github.com/reliability-lab/gen/orders"
	"github.com/reliability-lab/gen/payments"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const grpcTimeout = 10 * time.Second

type createOrderRequest struct {
	UserID         string `json:"user_id"`
	AmountCents    int64  `json:"amount_cents"`
	Currency       string `json:"currency"`
	IdempotencyKey string `json:"idempotency_key"`
}

type createOrderResponse struct {
	OrderID        string `json:"order_id"`
	OrderStatus   string `json:"order_status"`
	PaymentSuccess bool   `json:"payment_success"`
	PaymentCode   string `json:"payment_code"`
}

func (h *handler) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("gateway").Start(r.Context(), "POST /orders")
	defer span.End()
	start := time.Now()
	route := "POST /orders"
	method := "POST"

	var req createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		recordHTTP(route, method, "400")
		return
	}
	if req.UserID == "" || req.AmountCents <= 0 || req.Currency == "" || req.IdempotencyKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing or invalid required fields: user_id, amount_cents (>0), currency, idempotency_key"})
		recordHTTP(route, method, "400")
		return
	}

	orderCtx, cancel := context.WithTimeout(ctx, grpcTimeout)
	defer cancel()
	createResp, err := h.ordersClient.CreateOrder(orderCtx, &orders.CreateOrderRequest{
		UserId:         req.UserID,
		AmountCents:    req.AmountCents,
		Currency:       req.Currency,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		span.RecordError(err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		recordHTTP(route, method, "500")
		return
	}

	chargeResp, err := h.chargeWithRetry(ctx, createResp.OrderId, req.AmountCents, req.Currency, req.IdempotencyKey)
	if err != nil {
		span.RecordError(err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		recordHTTP(route, method, "500")
		return
	}

	if h.notificationsClient != nil {
		notifCtx, notifCancel := context.WithTimeout(ctx, grpcTimeout)
		_, _ = h.notificationsClient.SendReceipt(notifCtx, &notifications.SendReceiptRequest{
			OrderId: createResp.OrderId,
			UserId:  req.UserID,
		})
		notifCancel()
	}

	writeJSON(w, http.StatusOK, createOrderResponse{
		OrderID:         createResp.OrderId,
		OrderStatus:    createResp.Status,
		PaymentSuccess: chargeResp.Success,
		PaymentCode:   chargeResp.Code,
	})
	recordHTTP(route, method, "200")
	httpRequestDurationSeconds.WithLabelValues(route, method).Observe(time.Since(start).Seconds())
}

func (h *handler) chargeWithRetry(ctx context.Context, orderID string, amountCents int64, currency, idemKey string) (*payments.ChargeResponse, error) {
	ctx, span := otel.Tracer("gateway").Start(ctx, "payments.Charge")
	defer span.End()
	const maxRetries = 2
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, grpcTimeout)
		resp, err := h.paymentsClient.Charge(callCtx, &payments.ChargeRequest{
			OrderId:        orderID,
			AmountCents:    amountCents,
			Currency:       currency,
			IdempotencyKey: idemKey,
		})
		cancel()
		if err == nil {
			if !resp.Success && resp.Code == "DECLINED" {
				return resp, nil
			}
			return resp, nil
		}
		lastErr = err
		st, ok := status.FromError(err)
		if !ok {
			return nil, err
		}
		if st.Code() != grpccodes.Unavailable && st.Code() != grpccodes.DeadlineExceeded {
			return nil, err
		}
		if attempt < maxRetries {
			jitter := time.Duration(rand.Intn(50)+25) * time.Millisecond
			time.Sleep(time.Duration(1<<uint(attempt))*100*time.Millisecond + jitter)
		}
	}
	return nil, lastErr
}

func (h *handler) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("gateway").Start(r.Context(), "GET /orders/:id")
	defer span.End()
	start := time.Now()
	route := "GET /orders/:id"
	method := "GET"

	id := strings.TrimPrefix(r.URL.Path, "/orders/")
	if id == "" || strings.Contains(id, "/") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid order id"})
		recordHTTP(route, method, "400")
		return
	}

	callCtx, cancel := context.WithTimeout(ctx, grpcTimeout)
	defer cancel()
	resp, err := h.ordersClient.GetOrder(callCtx, &orders.GetOrderRequest{OrderId: id})
	if err != nil {
		if status.Code(err) == grpccodes.NotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
			recordHTTP(route, method, "404")
			return
		}
		span.RecordError(err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		recordHTTP(route, method, "500")
		return
	}

	span.SetAttributes(
		attribute.String("order_id", resp.OrderId),
	)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"order_id":        resp.OrderId,
		"user_id":         resp.UserId,
		"amount_cents":    resp.AmountCents,
		"currency":        resp.Currency,
		"status":          resp.Status,
		"idempotency_key": resp.IdempotencyKey,
		"created_at":      resp.CreatedAt,
	})
	recordHTTP(route, method, "200")
	httpRequestDurationSeconds.WithLabelValues(route, method).Observe(time.Since(start).Seconds())
}

func recordHTTP(route, method, status string) {
	httpRequestsTotal.WithLabelValues(route, method, status).Inc()
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
