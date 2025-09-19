FROM golang:1.24.4-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git build-base linux-headers

WORKDIR /app

# Copy go files (you need the source code, not .exe)
COPY go.mod go.sum ./
COPY . .

# Build for Linux (not Windows .exe)
RUN CGO_ENABLED=1 GOOS=linux go build -o packetForwarding ./cmd/test/packetForwarding.go

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    iptables \
    iproute2 \
    iputils \
    tcpdump \
    net-tools \
    bash

WORKDIR /app

# Copy the Linux binary (not .exe)
COPY --from=builder /app/packetForwarding .

# Create a startup script
COPY docker-entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]