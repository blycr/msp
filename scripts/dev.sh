#!/usr/bin/env bash
set -euo pipefail

# 默认参数
BACKEND_PORT=8099

# 解析参数
while [[ $# -gt 0 ]]; do
  case "$1" in
    -BackendPort|--backend-port)
      BACKEND_PORT="$2"
      shift 2
      ;;
    *)
      echo "Unknown argument: $1"
      exit 1
      ;;
  esac
done

# 路径定义
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
WEB_DIR="$ROOT_DIR/web"
BIN_DIR="$ROOT_DIR/bin/dev"
BACKEND_EXE="$BIN_DIR/msp-dev"
CONFIG_FILE="$BIN_DIR/config.json"
EXAMPLE_CONFIG="$ROOT_DIR/config.example.json"

# 全局变量用于进程管理
BACKEND_PID=""
FRONTEND_PID=""

# 日志颜色
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

log() {
  local level="${2:-INFO}"
  local color="$BLUE"
  case "$level" in
    "ERROR") color="$RED" ;;
    "WARN") color="$YELLOW" ;;
    "SUCCESS") color="$GREEN" ;;
  esac
  echo -e "${color}[dev]${NC} $1"
}

# 清理函数
cleanup() {
  log "Shutting down development server..."
  if [[ -n "$FRONTEND_PID" ]]; then
    kill "$FRONTEND_PID" 2>/dev/null || true
    wait "$FRONTEND_PID" 2>/dev/null || true
    FRONTEND_PID=""
  fi
  if [[ -n "$BACKEND_PID" ]]; then
    # 尝试优雅关闭
    kill "$BACKEND_PID" 2>/dev/null || true
    # 等待最多5秒
    local count=0
    while kill -0 "$BACKEND_PID" 2>/dev/null && [[ $count -lt 10 ]]; do
      sleep 0.5
      count=$((count + 1))
    done
    # 如果还在运行，强制终止
    if kill -0 "$BACKEND_PID" 2>/dev/null; then
      log "Backend did not exit gracefully, forcing termination..." "WARN"
      kill -9 "$BACKEND_PID" 2>/dev/null || true
    fi
    BACKEND_PID=""
  fi
  log "Cleanup completed." "SUCCESS"
}

# 注册信号处理
trap cleanup EXIT INT TERM

# 初始化配置
init_config() {
  mkdir -p "$BIN_DIR"

  if [[ ! -f "$CONFIG_FILE" ]]; then
    if [[ -f "$EXAMPLE_CONFIG" ]]; then
      cp "$EXAMPLE_CONFIG" "$CONFIG_FILE"
    else
      echo "{}" > "$CONFIG_FILE"
    fi
  fi

  # 使用 jq 更新配置
  if command -v jq >/dev/null 2>&1; then
    local tmp
    tmp=$(mktemp)
    jq --argjson port "$BACKEND_PORT" \
       '.port = $port |
        if .blacklist == null then .blacklist = {} else . end |
        if .blacklist.extensions == null then .blacklist.extensions = [] else . end |
        if .blacklist.filenames == null then .blacklist.filenames = [] else . end |
        if .blacklist.folders == null then .blacklist.folders = [] else . end |
        if .blacklist.sizeRule == null then .blacklist.sizeRule = "" else . end' \
       "$CONFIG_FILE" > "$tmp" && mv "$tmp" "$CONFIG_FILE"
    log "Updated dev config with port $BACKEND_PORT"
  else
    log "Warning: 'jq' not found. Skipping config.json auto-update." "WARN"
    log "Please ensure '$CONFIG_FILE' has port set to $BACKEND_PORT manually." "WARN"
  fi
}

build_backend() {
  log "Building backend..."
  (cd "$ROOT_DIR" && go build -o "$BACKEND_EXE" ./cmd/msp)
  log "Backend built successfully." "SUCCESS"
}

start_backend() {
  # 停止现有后端
  if [[ -n "$BACKEND_PID" ]]; then
    kill "$BACKEND_PID" 2>/dev/null || true
    wait "$BACKEND_PID" 2>/dev/null || true
    BACKEND_PID=""
  fi

  init_config

  log "Starting backend on port $BACKEND_PORT..."
  export MSP_NO_AUTO_OPEN="1"
  (cd "$BIN_DIR" && "$BACKEND_EXE") &
  BACKEND_PID=$!
  log "Backend started (pid=$BACKEND_PID)" "SUCCESS"
  # 等待后端启动
  sleep 2
}

start_frontend() {
  # 停止现有前端
  if [[ -n "$FRONTEND_PID" ]]; then
    kill "$FRONTEND_PID" 2>/dev/null || true
    wait "$FRONTEND_PID" 2>/dev/null || true
    FRONTEND_PID=""
  fi

  if ! command -v pnpm >/dev/null 2>&1; then
    log "pnpm not found. Enabling corepack..." "WARN"
    corepack enable || {
      log "Error: pnpm not found and corepack enable failed." "ERROR"
      exit 1
    }
  fi

  cd "$WEB_DIR"

  if [[ ! -d "node_modules" ]]; then
    log "Installing frontend dependencies..."
    pnpm install
  fi

  log "Starting frontend (Vite)..."
  export MSP_DEV_BACKEND="http://127.0.0.1:$BACKEND_PORT"
  pnpm run dev &
  FRONTEND_PID=$!
  log "Frontend started (pid=$FRONTEND_PID)" "SUCCESS"
}

# 获取所有 Go 文件的最新修改时间戳
get_latest_mtime() {
  if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    find "$ROOT_DIR" -name "*.go" -type f -print0 2>/dev/null | xargs -0 stat -f %m 2>/dev/null | sort -rn | head -n1 || echo "0"
  else
    # Linux
    find "$ROOT_DIR" -name "*.go" -type f -print0 2>/dev/null | xargs -0 stat -c %Y 2>/dev/null | sort -rn | head -n1 || echo "0"
  fi
}

# 主流程
log "Root: $ROOT_DIR"
log "Backend Port: $BACKEND_PORT"

# 检查依赖
if ! command -v go >/dev/null 2>&1; then
  log "Go not found. Please install Go." "ERROR"
  exit 1
fi

# 初始构建和启动
build_backend
start_backend
start_frontend

log "Development server is running. Press Ctrl+C to stop."

LAST_MTIME=$(get_latest_mtime)

# 主循环
while true; do
  sleep 1
  CURRENT_MTIME=$(get_latest_mtime)

  if [[ "$CURRENT_MTIME" != "$LAST_MTIME" && "$CURRENT_MTIME" != "0" ]]; then
    log "Change detected. Rebuilding backend..."
    if build_backend; then
      start_backend
      LAST_MTIME=$(get_latest_mtime)
    else
      log "Build failed. Waiting for fix..." "WARN"
      LAST_MTIME=$CURRENT_MTIME
    fi
  fi
done
