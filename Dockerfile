# Build stage for frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend

# Install pnpm
RUN corepack enable && corepack prepare pnpm@latest --activate

# Copy frontend package files
COPY frontend/package.json frontend/pnpm-lock.yaml* ./

# Install dependencies
RUN pnpm install --frozen-lockfile

# Copy frontend source
COPY frontend/ ./

# Build frontend
RUN pnpm build

# Build stage for backend
FROM golang:1.21-alpine AS backend-builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Copy built frontend assets
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

# Build binary with CGO enabled for SQLite
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-linkmode external -extldflags "-static"' -o gosip ./cmd/gosip

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata sqlite

# Create non-root user
RUN addgroup -g 1000 gosip && \
    adduser -u 1000 -G gosip -s /bin/sh -D gosip

WORKDIR /app

# Create data directories
RUN mkdir -p /app/data/recordings /app/data/voicemails /app/data/backups && \
    chown -R gosip:gosip /app/data

# Copy binary from builder
COPY --from=backend-builder /app/gosip .
COPY --from=backend-builder /app/migrations ./migrations

# Copy frontend assets
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

# Set ownership
RUN chown -R gosip:gosip /app

# Switch to non-root user
USER gosip

# Expose ports
# 8080 - HTTP API
# 5060 - SIP UDP
# 5060 - SIP TCP
EXPOSE 8080 5060/udp 5060/tcp

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/health || exit 1

# Environment variables
ENV GOSIP_DATA_DIR=/app/data \
    GOSIP_DB_PATH=/app/data/gosip.db \
    GOSIP_HTTP_PORT=8080 \
    GOSIP_SIP_PORT=5060 \
    GOSIP_LOG_LEVEL=info \
    TZ=America/New_York

# Volume for persistent data
VOLUME ["/app/data"]

# Run the application
ENTRYPOINT ["./gosip"]
