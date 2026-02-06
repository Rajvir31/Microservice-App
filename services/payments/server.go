package main

import (
	"context"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/reliability-lab/gen/payments"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type chargeResult struct {
	success bool
	code    string
	at      time.Time
}

var (
	idemMu   sync.RWMutex
	idemMap  = make(map[string]chargeResult)
	randSeed = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func (s *paymentsServer) Charge(ctx context.Context, req *payments.ChargeRequest) (*payments.ChargeResponse, error) {
	ctx, span := otel.Tracer("payments").Start(ctx, "Charge")
	defer span.End()

	// Idempotency: return cached result if we've seen this key before
	idemMu.RLock()
	if cached, ok := idemMap[req.IdempotencyKey]; ok {
		idemMu.RUnlock()
		return &payments.ChargeResponse{Success: cached.success, Code: cached.code}, nil
	}
	idemMu.RUnlock()

	// Fault injection: latency
	if ms := getPaymentsLatencyMs(); ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}

	// Fault injection: force fail or random error rate
	var success bool
	var code string
	if getPaymentsForceFail() {
		success = false
		code = "DECLINED"
		paymentsDeclinedTotal.Inc()
	} else if randFloat() < getPaymentsErrorRate() {
		success = false
		code = "DECLINED"
		paymentsDeclinedTotal.Inc()
	} else {
		success = true
		code = "APPROVED"
	}

	res := chargeResult{success: success, code: code, at: time.Now()}
	idemMu.Lock()
	idemMap[req.IdempotencyKey] = res
	idemMu.Unlock()

	return &payments.ChargeResponse{Success: success, Code: code}, nil
}

func getPaymentsLatencyMs() int {
	s := os.Getenv("PAYMENTS_LATENCY_MS")
	if s == "" {
		return 0
	}
	n, _ := strconv.Atoi(s)
	if n < 0 {
		return 0
	}
	return n
}

func getPaymentsErrorRate() float64 {
	s := os.Getenv("PAYMENTS_ERROR_RATE")
	if s == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	if f <= 0 || f > 1 {
		return 0
	}
	return f
}

func getPaymentsForceFail() bool {
	s := os.Getenv("PAYMENTS_FORCE_FAIL")
	return s == "true" || s == "1"
}

func randFloat() float64 {
	randMu.Lock()
	defer randMu.Unlock()
	return randSeed.Float64()
}

var randMu sync.Mutex

type paymentsServer struct {
	payments.UnimplementedPaymentsServer
}

var _ payments.PaymentsServer = (*paymentsServer)(nil)
