package main

import (
	"context"
	"os"
	"testing"

	"github.com/reliability-lab/gen/payments"
)

func TestCharge_Idempotency(t *testing.T) {
	s := &paymentsServer{}
	ctx := context.Background()
	req := &payments.ChargeRequest{
		OrderId:        "order-1",
		AmountCents:    1000,
		Currency:       "USD",
		IdempotencyKey: "idem-key-123",
	}

	resp1, err := s.Charge(ctx, req)
	if err != nil {
		t.Fatalf("first Charge: %v", err)
	}
	resp2, err := s.Charge(ctx, req)
	if err != nil {
		t.Fatalf("second Charge: %v", err)
	}

	if resp1.Success != resp2.Success {
		t.Errorf("Success mismatch: first=%v second=%v", resp1.Success, resp2.Success)
	}
	if resp1.Code != resp2.Code {
		t.Errorf("Code mismatch: first=%q second=%q", resp1.Code, resp2.Code)
	}
}

func TestCharge_ForceFail(t *testing.T) {
	os.Setenv("PAYMENTS_FORCE_FAIL", "true")
	defer os.Unsetenv("PAYMENTS_FORCE_FAIL")

	s := &paymentsServer{}
	ctx := context.Background()
	req := &payments.ChargeRequest{
		OrderId:        "order-2",
		AmountCents:    1000,
		Currency:       "USD",
		IdempotencyKey: "idem-force-fail",
	}
	resp, err := s.Charge(ctx, req)
	if err != nil {
		t.Fatalf("Charge: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false when PAYMENTS_FORCE_FAIL=true")
	}
	if resp.Code != "DECLINED" {
		t.Errorf("expected code=DECLINED, got %q", resp.Code)
	}
}
