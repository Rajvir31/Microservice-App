# Build context must be repo root (../..) so gen/ is available
FROM golang:1.22-alpine AS builder
WORKDIR /workspace
COPY gen ./gen
COPY services/gateway ./services/gateway
WORKDIR /workspace/services/gateway
RUN go mod download && CGO_ENABLED=0 go build -o /gateway .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /gateway .
EXPOSE 8080
ENTRYPOINT ["./gateway"]
