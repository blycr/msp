# Multi-stage build

# Stage 1: Build Frontend
FROM oven/bun:1.3-alpine AS frontend-builder
ENV BUN_CONFIG_REGISTRY=https://registry.npmmirror.com
WORKDIR /app/web
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile
COPY web/ ./
RUN bun run build

# Stage 2: Build Backend
FROM golang:1.25-alpine AS backend-builder
ENV GOPROXY=https://goproxy.cn,direct
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

# FFmpeg for transcode/thumbnails/probing; no sqlite libs needed (pure Go driver)
RUN apk add --no-cache ffmpeg

# Set environment variable to disable auto-open browser
ENV MSP_NO_AUTO_OPEN=1

# Binary lives in /opt so /app can be bind-mounted as the data volume
COPY --from=backend-builder /app/msp-server /opt/msp-server

# Expose default port
EXPOSE 8099

CMD ["./msp-server"]
