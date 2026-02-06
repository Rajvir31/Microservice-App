package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/reliability-lab/gen/payments"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

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
			semconv.ServiceName("payments"),
		)),
	)
	otel.SetTracerProvider(tp)
	return func() { _ = tp.Shutdown(ctx) }, nil
}

func metricsUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		code := codes.Code(0).String()
		if st, ok := status.FromError(err); ok {
			code = st.Code().String()
		}
		svc := "payments"
		method := info.FullMethod
		if len(method) > 0 && method[0] == '/' {
			method = method[1:]
		}
		rpcRequestsTotal.WithLabelValues(svc, method, code).Inc()
		rpcRequestDurationSeconds.WithLabelValues(svc, method).Observe(time.Since(start).Seconds())
		return resp, err
	}
}

func main() {
	ctx := context.Background()
	shutdown, err := initTracer(ctx)
	if err != nil {
		shutdown = func() {}
	}
	defer shutdown()

	grpcPort := os.Getenv("PAYMENTS_GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "50052"
	}
	httpPort := os.Getenv("PAYMENTS_HTTP_PORT")
	if httpPort == "" {
		httpPort = "8082"
	}

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatal().Err(err).Msg("listen failed")
	}
	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			otelgrpc.UnaryServerInterceptor(),
			metricsUnaryInterceptor(),
		),
	)
	payments.RegisterPaymentsServer(grpcSrv, &paymentsServer{})
	go func() {
		_ = grpcSrv.Serve(lis)
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/metrics", promhttp.Handler())
	httpSrv := &http.Server{Addr: ":" + httpPort, Handler: mux}
	go func() {
		_ = httpSrv.ListenAndServe()
	}()

	log.Info().Str("grpc", grpcPort).Str("http", httpPort).Msg("payments service started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	grpcSrv.GracefulStop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}
