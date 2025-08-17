# Simple single-stage build for fast development
FROM golang:1.21-alpine

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy everything
COPY . .

# Simple fast build without optimizations
RUN go build -o main ./cmd/hh-bot

# Create config directory
RUN mkdir -p config

# Set timezone
ENV TZ=Europe/Moscow

CMD ["./main"]
