module github.com/reliability-lab/services/payments

go 1.22

require (
	github.com/reliability-lab/gen v0.0.0
	github.com/prometheus/client_golang v1.18.0
	github.com/rs/zerolog v1.32.0
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.48.0
	go.opentelemetry.io/otel v1.24.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.24.0
	go.opentelemetry.io/otel/sdk v1.24.0
	go.opentelemetry.io/otel/semconv/v1.24.0 v1.24.0
	google.golang.org/grpc v1.62.0
)

replace github.com/reliability-lab/gen => ../../gen
