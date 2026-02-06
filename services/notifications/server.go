package main

import (
	"context"

	"github.com/reliability-lab/gen/notifications"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type notificationsServer struct {
	notifications.UnimplementedNotificationsServer
}

func (s *notificationsServer) SendReceipt(ctx context.Context, req *notifications.SendReceiptRequest) (*notifications.SendReceiptResponse, error) {
	span := trace.SpanFromContext(ctx)
	traceID := ""
	spanID := ""
	if span.SpanContext().IsValid() {
		traceID = span.SpanContext().TraceID().String()
		spanID = span.SpanContext().SpanID().String()
	}
	log.Info().
		Str("event", "receipt_sent").
		Str("order_id", req.OrderId).
		Str("user_id", req.UserId).
		Str("trace_id", traceID).
		Str("span_id", spanID).
		Msg("receipt_sent")
	return &notifications.SendReceiptResponse{Ok: true}, nil
}

var _ notifications.NotificationsServer = (*notificationsServer)(nil)
