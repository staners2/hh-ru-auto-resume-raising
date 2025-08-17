# Multi-stage build for optimized production image
# Stage 1: Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk --no-cache add git ca-certificates tzdata

WORKDIR /app

# Copy go mod and sum files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/

# Build with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o main ./cmd/hh-bot

# Stage 2: Runtime stage
FROM alpine:3.19

# Install only runtime dependencies
RUN apk --no-cache add ca-certificates tzdata && \
    adduser -D -s /bin/sh -u 1000 appuser

# Copy timezone data and binary
COPY --from=builder /app/main /usr/local/bin/main

# Create config directory with proper permissions
RUN mkdir -p /app/config && \
    chown -R appuser:appuser /app

WORKDIR /app

# Set timezone
ENV TZ=Europe/Moscow

# Run as non-root user for security
USER appuser

CMD ["/usr/local/bin/main"]
