#!/usr/bin/env bash
set -euo pipefail

PLATFORMS="windows"
ARCHITECTURES="x64"
SKIP_TESTS="false"
SKIP_LINT="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    -Platforms|--platforms)
      PLATFORMS="$2"
      shift 2
      ;;
    -Architectures|--architectures)
      ARCHITECTURES="$2"
      shift 2
      ;;
    -SkipTests|--skip-tests)
      SKIP_TESTS="true"
      shift
      ;;
    -SkipLint|--skip-lint)
      SKIP_LINT="true"
      shift
      ;;
    *)
      echo "Unknown argument: $1"
      exit 1
      ;;
  esac
done

root="$(cd "$(dirname "$0")/.." && pwd)"
logFile="$(cd "$(dirname "$0")" && pwd)/build.log"

log() {
  local level="$2"
  local ts
  ts="$(date '+%Y-%m-%d %H:%M:%S')"
  local line="[$ts][$level] $1"
  case "$level" in
    "ERROR") echo -e "\033[0;31m$line\033[0m" ;;
    "WARN")  echo -e "\033[0;33m$line\033[0m" ;;
    "SUCCESS") echo -e "\033[0;32m$line\033[0m" ;;
    *) echo "$line" ;;
  esac
  printf "%s\n" "$line" >> "$logFile" || true
}

invoke_step() {
  local name="$1"
  shift
  log "$name" "INFO"
  if "$@"; then
    log "$name done." "SUCCESS"
  else
    log "$name failed." "ERROR"
    exit 1
  fi
}

new_dir() {
  p="$1"
  mkdir -p "$p"
}

build_go() {
  goos="$1"
  goarch="$2"
  out="$3"
  goarm="${4:-}"
  (
    cd "$root"
    export GOOS="$goos"
    export GOARCH="$goarch"
    export CGO_ENABLED=0
    if [[ -n "$goarm" ]]; then
      export GOARM="$goarm"
    else
      unset GOARM || true
    fi
    new_dir "$(dirname "$out")"
    go build -trimpath -ldflags="-s -w" -o "$out" ./cmd/msp
    log "Built: $out" "SUCCESS"
  )
}

write_checksum() {
  file="$1"
  out="$2"
  if command -v sha256sum >/dev/null 2>&1; then
    hash="$(sha256sum "$file" | awk '{print $1}')"
  else
    hash="$(shasum -a 256 "$file" | awk '{print $1}')"
  fi
  line="$hash  $(basename "$file")"
  new_dir "$(dirname "$out")"
  printf "%s\n" "$line" > "$out"
  log "Checksum: $out" "INFO"
}

should_build() {
  platform="$1"
  arch_or_variant="$2"
  IFS=',' read -r -a p_arr <<< "$PLATFORMS"
  IFS=',' read -r -a a_arr <<< "$ARCHITECTURES"

  normalize_arch() {
    local a="$1"
    a="${a,,}" # lowercase
    if [[ "$a" == "x64" ]]; then echo "amd64";
    elif [[ "$a" == "x86" ]]; then echo "386";
    else echo "$a"; fi
  }

  target="$(normalize_arch "$arch_or_variant")"
  platform_lower="${platform,,}"

  p_match="false"
  for p in "${p_arr[@]}"; do
    if [[ "${p,,}" == "$platform_lower" ]]; then
      p_match="true"
      break
    fi
  done
  if [[ "$p_match" != "true" ]]; then
    return 1
  fi
  for a in "${a_arr[@]}"; do
    if [[ "$(normalize_arch "$a")" == "$target" ]]; then
      return 0
    fi
  done
  return 1
}

# 检查依赖
if ! command -v go >/dev/null 2>&1; then
  log "Go not found. Please install Go." "ERROR"
  exit 1
fi

invoke_step "Build Frontend" bash -c "
  if ! command -v pnpm >/dev/null 2>&1; then
    log 'pnpm not found. Enabling corepack...' 'WARN'
    corepack enable || {
      log 'pnpm is not installed and corepack enable failed. Please install pnpm: npm install -g pnpm' 'ERROR'
      exit 1
    }
  fi
  cd '$root/web'
  if [[ ! -d node_modules ]]; then
    log 'Installing pnpm dependencies...' 'INFO'
    pnpm install
  fi
  log 'Building frontend...' 'INFO'
  pnpm run build
"

if [[ "$SKIP_TESTS" != "true" ]]; then
  invoke_step "Run Go Tests" bash -c "
    cd '$root'
    go test -v ./...
  "
fi

if [[ "$SKIP_LINT" != "true" ]]; then
  invoke_step "Run Go Vet" bash -c "
    cd '$root'
    go vet ./...
  "

  if command -v golangci-lint >/dev/null 2>&1; then
    invoke_step "Run golangci-lint" bash -c "
      cd '$root'
      golangci-lint run ./...
    "
  else
    log "golangci-lint not found, skipping lint check. Install from https://golangci-lint.run/" "WARN"
  fi
fi

invoke_step "Cross Build Artifacts" bash -c "
  binRoot='$root/bin'
  chkRoot='$root/checksums'

  build_configs=(
    'linux:amd64:msp-linux-amd64:'
    'linux:arm64:msp-linux-arm64:'
    'linux:arm:msp-linux-armv7:7'
    'darwin:amd64:msp-darwin-amd64:'
    'darwin:arm64:msp-darwin-arm64:'
    'windows:amd64:msp-windows-amd64.exe:'
    'windows:386:msp-windows-386.exe:'
  )

  for cfg in \"\${build_configs[@]}\"; do
    IFS=':' read -r platform arch outName goarm <<< \"\$cfg\"

    should_build_flag=false
    if [[ \"\$platform\" == 'linux' && \"\$arch\" == 'amd64' ]]; then
      should_build 'linux' 'amd64' || should_build 'linux' 'x64' && should_build_flag=true
    elif [[ \"\$platform\" == 'linux' && \"\$arch\" == 'arm64' ]]; then
      should_build 'linux' 'arm64' && should_build_flag=true
    elif [[ \"\$platform\" == 'linux' && \"\$arch\" == 'arm' ]]; then
      should_build 'arm' 'v7' && should_build_flag=true
    elif [[ \"\$platform\" == 'darwin' && \"\$arch\" == 'amd64' ]]; then
      should_build 'macos' 'amd64' || should_build 'macos' 'x64' && should_build_flag=true
    elif [[ \"\$platform\" == 'darwin' && \"\$arch\" == 'arm64' ]]; then
      should_build 'macos' 'arm64' && should_build_flag=true
    elif [[ \"\$platform\" == 'windows' && \"\$arch\" == 'amd64' ]]; then
      should_build 'windows' 'amd64' || should_build 'windows' 'x64' && should_build_flag=true
    elif [[ \"\$platform\" == 'windows' && \"\$arch\" == '386' ]]; then
      should_build 'windows' '386' || should_build 'windows' 'x86' && should_build_flag=true
    fi

    if [[ \"\$should_build_flag\" == 'true' ]]; then
      outPath=\"\$binRoot/\$platform/\$arch/\$outName\"
      build_go \"\$platform\" \"\$arch\" \"\$outPath\" \"\$goarm\"
      chkPath=\"\$chkRoot/\$outName.sha256\"
      write_checksum \"\$outPath\" \"\$chkPath\"
    fi
  done
"

log "Build completed." "SUCCESS"
