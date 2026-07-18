# Multi-stage build

# Stage 1: Build Frontend
FROM oven/bun:1.3-alpine AS frontend-builder
WORKDIR /app/web
COPY web/package.json web/bun.lockb ./
RUN bun install --frozen-lockfile
COPY web/ ./
RUN bun run build

# Stage 2: Build Backend
FROM golang:1.25-alpine AS backend-builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Copy built frontend assets
COPY --from=frontend-builder /app/web/dist ./web/dist

# Build with CGO disabled (using glebarez/sqlite, pure Go)
RUN CGO_ENABLED=0 GOOS=linux go build -o msp-server ./cmd/msp

# Stage 3: Runtime
FROM alpine:3.21
WORKDIR /app

# No extra sqlite libs needed for glebarez/sqlite

COPY --from=backend-builder /app/msp-server .
# Create data directory
RUN mkdir -p /data
# Set environment variable to disable auto-open browser
ENV MSP_NO_AUTO_OPEN=1

# Expose default port
EXPOSE 8099

# Volume for data (config, db) and media
VOLUME ["/data", "/media"]

# Run as non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

CMD ["./msp-server"]
