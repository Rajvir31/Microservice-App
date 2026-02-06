# Build context must be repo root (../..) so gen/ is available
FROM golang:1.22-alpine AS builder
WORKDIR /workspace
COPY gen ./gen
COPY services/notifications ./services/notifications
WORKDIR /workspace/services/notifications
RUN go mod download && CGO_ENABLED=0 go build -o /notifications .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /notifications .
EXPOSE 50053 8083
ENTRYPOINT ["./notifications"]
