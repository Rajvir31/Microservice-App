package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/reliability-lab/gen/notifications"
	"github.com/reliability-lab/gen/orders"
	"github.com/reliability-lab/gen/payments"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type handler struct {
	ordersClient        orders.OrdersClient
	paymentsClient      payments.PaymentsClient
	notificationsClient notifications.NotificationsClient
}

func initTracer(ctx context.Context) (func(), error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "otel-collector:4317"
	}
	endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://")
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("gateway"),
		)),
	)
	otel.SetTracerProvider(tp)
	return func() { _ = tp.Shutdown(ctx) }, nil
}

func dialGRPC(target string) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return grpc.DialContext(ctx, target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
	)
}

func main() {
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	ctx := context.Background()
	shutdown, err := initTracer(ctx)
	if err != nil {
		shutdown = func() {}
	}
	defer shutdown()

	ordersAddr := os.Getenv("ORDERS_GRPC_ADDR")
	if ordersAddr == "" {
		ordersAddr = "orders:50051"
	}
	paymentsAddr := os.Getenv("PAYMENTS_GRPC_ADDR")
	if paymentsAddr == "" {
		paymentsAddr = "payments:50052"
	}
	notificationsAddr := os.Getenv("NOTIFICATIONS_GRPC_ADDR")
	if notificationsAddr == "" {
		notificationsAddr = "notifications:50053"
	}

	ordersConn, err := dialGRPC(ordersAddr)
	if err != nil {
		log.Fatal().Err(err).Str("target", ordersAddr).Msg("dial orders failed")
	}
	defer ordersConn.Close()

	paymentsConn, err := dialGRPC(paymentsAddr)
	if err != nil {
		log.Fatal().Err(err).Str("target", paymentsAddr).Msg("dial payments failed")
	}
	defer paymentsConn.Close()

	var notificationsConn *grpc.ClientConn
	notificationsConn, err = dialGRPC(notificationsAddr)
	if err != nil {
		log.Warn().Err(err).Str("target", notificationsAddr).Msg("notifications optional: dial failed")
		notificationsConn = nil
	} else if notificationsConn != nil {
		defer notificationsConn.Close()
	}

	var notifClient notifications.NotificationsClient
	if notificationsConn != nil {
		notifClient = notifications.NewNotificationsClient(notificationsConn)
	}

	h := &handler{
		ordersClient:        orders.NewOrdersClient(ordersConn),
		paymentsClient:      payments.NewPaymentsClient(paymentsConn),
		notificationsClient: notifClient,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if !grpcReachable(ordersAddr) || !grpcReachable(paymentsAddr) {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			h.handleCreateOrder(w, r)
			return
		}
		httpRequestsTotal.WithLabelValues("", r.Method, "405").Inc()
		w.WriteHeader(http.StatusMethodNotAllowed)
	})
	mux.HandleFunc("/orders/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.handleGetOrder(w, r)
			return
		}
		httpRequestsTotal.WithLabelValues("GET /orders/:id", r.Method, "405").Inc()
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	handler := otelhttp.NewHandler(mux, "gateway")
	port := os.Getenv("GATEWAY_HTTP_PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("http server failed")
		}
	}()

	log.Info().Str("port", port).Msg("gateway started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func grpcReachable(addr string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
