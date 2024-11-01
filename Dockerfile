FROM golang:1.23-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Development build
FROM builder AS dev-builder
RUN CGO_ENABLED=1 go build \
    -o wallet \
    -gcflags="all=-N -l" \
    cmd/server/main.go

# Production build
FROM builder AS prod-builder
ARG VERSION=1.0.0
RUN CGO_ENABLED=1 go build \
    -o wallet \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.BuildTime=$(date -u +%Y-%m-%d_%H:%M:%S)" \
    cmd/server/main.go

# Development image
FROM alpine:3.18 AS development
RUN apk add --no-cache \
    curl \
    tzdata \
    wget

WORKDIR /app

COPY --from=dev-builder /build/wallet .

RUN mkdir -p /app/migrations && \
    adduser -D -u 1000 appuser && \
    chown -R appuser:appuser /app

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=60s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

CMD ["./wallet"]

# Production image
FROM alpine:3.18 AS production
RUN apk add --no-cache \
    tzdata \
    wget

WORKDIR /app

COPY --from=prod-builder /build/wallet .

RUN mkdir -p /app/migrations && \
    adduser -D -u 1000 appuser && \
    chown -R appuser:appuser /app

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=60s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

CMD ["./wallet"]