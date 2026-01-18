# Multi-stage build

# Stage 1: Build Frontend
FROM node:20-alpine AS frontend-builder
# Install pnpm
RUN corepack enable && corepack prepare pnpm@latest --activate
WORKDIR /app/web
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm run build

# Stage 2: Build Backend
FROM golang:1.24-alpine AS backend-builder
WORKDIR /app
# Install build tools (gcc needed for cgo/sqlite)
RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Copy built frontend assets
COPY --from=frontend-builder /app/web/dist ./web/dist

# Build with CGO disabled (using modernc.org/sqlite)
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o msp-server ./cmd/msp

# Stage 3: Runtime
FROM alpine:latest
WORKDIR /app

# No extra sqlite libs needed for modernc.org/sqlite

COPY --from=backend-builder /app/msp-server .
# Create data directory
RUN mkdir -p /data
# Set environment variable to disable auto-open browser
ENV MSP_NO_AUTO_OPEN=1

# Expose default port
EXPOSE 8099

# Volume for data (config, db) and media
VOLUME ["/data", "/media"]

# Run
CMD ["./msp-server"]
