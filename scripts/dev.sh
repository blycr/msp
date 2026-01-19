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

# 日志颜色颜色
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
  echo -e "${BLUE}[dev]${NC} $1"
}

# 清理函数
cleanup() {
  log "Stopping services..."
  if [[ -n "$BACKEND_PID" ]]; then
    kill "$BACKEND_PID" 2>/dev/null || true
  fi
  if [[ -n "$FRONTEND_PID" ]]; then
    kill "$FRONTEND_PID" 2>/dev/null || true
  fi
  exit 0
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
  
  # 简单检查配置内容（这里使用 jq 或 python 来确保配置正确比较复杂，
  # 考虑到这是一个轻量脚本，且 config.example.json 通常是完整的，
  # 这里的修改逻辑简化为：如果需要可以通过 sed 替换，但 dev.ps1 的逻辑比较重。
  # 为了保持 bash 脚本轻量，我们假设用户会自己管理 config.json 
  # 或者它从 example 复制过来就是可用的。
  # 必须要做的修改是端口，可以使用临时文件替换）
  
  if command -v jq >/dev/null 2>&1; then
    tmp=$(mktemp)
    jq --argjson port "$BACKEND_PORT" \
       '.port = $port | 
        if .blacklist == null then .blacklist = {} else . end |
        if .blacklist.extensions == null then .blacklist.extensions = [] else . end |
        if .blacklist.filenames == null then .blacklist.filenames = [] else . end |
        if .blacklist.folders == null then .blacklist.folders = [] else . end |
        if .blacklist.sizeRule == null then .blacklist.sizeRule = "" else . end' \
       "$CONFIG_FILE" > "$tmp" && mv "$tmp" "$CONFIG_FILE"
  else
    log "Warning: 'jq' not found. Skipping config.json auto-update (port setting, etc)."
    log "Please ensure '$CONFIG_FILE' has port set to $BACKEND_PORT manually."
  fi
}

build_backend() {
  log "Building backend..."
  (cd "$ROOT_DIR" && go build -o "$BACKEND_EXE" ./cmd/msp)
}

start_backend() {
  if [[ -n "$BACKEND_PID" ]]; then
    kill "$BACKEND_PID" 2>/dev/null || true
    wait "$BACKEND_PID" 2>/dev/null || true
  fi

  init_config
  
  log "Starting backend on port $BACKEND_PORT..."
  export MSP_NO_AUTO_OPEN="1"
  (cd "$BIN_DIR" && "$BACKEND_EXE") &
  BACKEND_PID=$!
}

start_frontend() {
  if ! command -v pnpm >/dev/null 2>&1; then
    log "pnpm not found. Enabling corepack..."
    corepack enable || {
      log "Error: pnpm not found and corepack enable failed."
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
}

# 获取所有 Go 文件的最新修改时间戳
get_latest_mtime() {
  if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    find "$ROOT_DIR" -name "*.go" -type f -print0 | xargs -0 stat -f %m 2>/dev/null | sort -rn | head -n1
  else
    # Linux
    find "$ROOT_DIR" -name "*.go" -type f -print0 | xargs -0 stat -c %Y 2>/dev/null | sort -rn | head -n1
  fi
}

# 主流程
log "Root: $ROOT_DIR"
log "Backend Port: $BACKEND_PORT"

build_backend
start_backend
start_frontend

log "Watching for Go file changes..."

LAST_MTIME=$(get_latest_mtime)

while true; do
  sleep 1
  CURRENT_MTIME=$(get_latest_mtime)
  
  if [[ "$CURRENT_MTIME" != "$LAST_MTIME" ]]; then
    log "Change detected. Rebuilding backend..."
    if build_backend; then
      start_backend
      LAST_MTIME=$(get_latest_mtime) # Update timestamp after successful build
    else
      log "Build failed. Waiting for fix..."
      # Keep the old backend running or fail? 
      # Usually better to fail fast or wait. 
      # Let's just update timestamp so we don't loop efficiently on the same error state
      # unless the user changes the file again.
      LAST_MTIME=$CURRENT_MTIME 
    fi
  fi
done
