# Build context must be repo root (../..) so gen/ is available
FROM golang:1.22-alpine AS builder
WORKDIR /workspace
COPY gen ./gen
COPY services/payments ./services/payments
WORKDIR /workspace/services/payments
RUN go mod download && CGO_ENABLED=0 go build -o /payments .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /payments .
EXPOSE 50052 8082
ENTRYPOINT ["./payments"]
