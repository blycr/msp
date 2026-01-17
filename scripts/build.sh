#!/usr/bin/env bash
set -euo pipefail

PLATFORMS="windows"
ARCHITECTURES="x64"

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
    *)
      shift
      ;;
  esac
done


root="$(cd "$(dirname "$0")/.." && pwd)"
logFile="$(cd "$(dirname "$0")" && pwd)/build.log"

log() {
  ts="$(date '+%Y-%m-%d %H:%M:%S')"
  line="[$ts][$2] $1"
  echo "$line"
  printf "%s\n" "$line" >> "$logFile" || true
}

invoke_step() {
  name="$1"
  shift
  log "$name" "INFO"
  "$@"
  log "$name done." "INFO"
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
    log "Built: $out" "INFO"
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

write_debug_copy() {
  file="$1"
  out="$2"
  new_dir "$(dirname "$out")"
  cp -f "$file" "$out"
  log "Debug copy: $out" "INFO"
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

log "Build Frontend" "INFO"
# Check if pnpm is installed
if ! command -v pnpm >/dev/null 2>&1; then
  log "pnpm not found. Enabling corepack..." "INFO"
  corepack enable || {
    log "pnpm is not installed and corepack enable failed. Please install pnpm: npm install -g pnpm" "ERROR"
    exit 1
  }
fi

cd "$root/web"
if [[ ! -d node_modules ]]; then
  log "Installing pnpm dependencies..." "INFO"
  pnpm install
fi
log "Building frontend..." "INFO"
pnpm run build
log "Build Frontend done." "INFO"

log "go test ./..." "INFO"
cd "$root"
go test ./...
log "go test ./... done." "INFO"

log "Cross Build Artifacts" "INFO"
binRoot="$root/bin"
chkRoot="$root/checksums"
dbgRoot="$root/debug"

if should_build linux amd64; then
  build_go linux amd64 "$binRoot/linux/amd64/msp-linux-amd64"
  write_checksum "$binRoot/linux/amd64/msp-linux-amd64" "$chkRoot/msp-linux-amd64.sha256"
  write_debug_copy "$binRoot/linux/amd64/msp-linux-amd64" "$dbgRoot/linux/amd64/msp-linux-amd64.debug"
fi

if should_build linux arm64; then
  build_go linux arm64 "$binRoot/linux/arm64/msp-linux-arm64"
  write_checksum "$binRoot/linux/arm64/msp-linux-arm64" "$chkRoot/msp-linux-arm64.sha256"
  write_debug_copy "$binRoot/linux/arm64/msp-linux-arm64" "$dbgRoot/linux/arm64/msp-linux-arm64.debug"
fi

if should_build arm v7; then
  build_go linux arm "$binRoot/arm/v7/msp-arm-v7" "7"
  write_checksum "$binRoot/arm/v7/msp-arm-v7" "$chkRoot/msp-arm-v7.sha256"
  write_debug_copy "$binRoot/arm/v7/msp-arm-v7" "$dbgRoot/arm/v7/msp-arm-v7.debug"
fi

if should_build arm v8; then
  build_go linux arm64 "$binRoot/arm/v8/msp-arm-v8"
  write_checksum "$binRoot/arm/v8/msp-arm-v8" "$chkRoot/msp-arm-v8.sha256"
  write_debug_copy "$binRoot/arm/v8/msp-arm-v8" "$dbgRoot/arm/v8/msp-arm-v8.debug"
fi

if should_build macos amd64; then
  build_go darwin amd64 "$binRoot/macos/msp-macos-amd64"
  write_checksum "$binRoot/macos/msp-macos-amd64" "$chkRoot/msp-macos-amd64.sha256"
  write_debug_copy "$binRoot/macos/msp-macos-amd64" "$dbgRoot/macos/msp-macos-amd64.debug"
fi

if should_build macos arm64; then
  build_go darwin arm64 "$binRoot/macos/msp-macos-arm64"
  write_checksum "$binRoot/macos/msp-macos-arm64" "$chkRoot/msp-macos-arm64.sha256"
  write_debug_copy "$binRoot/macos/msp-macos-arm64" "$dbgRoot/macos/msp-macos-arm64.debug"
fi

if should_build windows x64; then
  build_go windows amd64 "$binRoot/windows/x64/msp-windows-amd64.exe"
  write_checksum "$binRoot/windows/x64/msp-windows-amd64.exe" "$chkRoot/msp-windows-amd64.sha256"
  write_debug_copy "$binRoot/windows/x64/msp-windows-amd64.exe" "$dbgRoot/windows/x64/msp-windows-amd64.debug"
fi

if should_build windows x86; then
  build_go windows 386 "$binRoot/windows/x86/msp-windows-386.exe"
  write_checksum "$binRoot/windows/x86/msp-windows-386.exe" "$chkRoot/msp-windows-386.sha256"
  write_debug_copy "$binRoot/windows/x86/msp-windows-386.exe" "$dbgRoot/windows/x86/msp-windows-386.debug"
fi

log "Cross Build Artifacts done." "INFO"

log "Build completed." "INFO"
