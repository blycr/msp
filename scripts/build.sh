#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROFILES_FILE="$SCRIPT_DIR/build-profiles.json"
root="$(cd "$SCRIPT_DIR/.." && pwd)"
logFile="$SCRIPT_DIR/build.log"

PLATFORMS=""
ARCHITECTURES=""
SKIP_TESTS="false"
SKIP_LINT="false"
PRESET=""
SHOW_HELP="false"
LIST_PRESETS="false"

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

show_help() {
  cat <<'EOF'

  MSP Build Script - Production Build Tool

  Usage:
    ./scripts/build.sh [options]

  Preset Mode (recommended):
    ./scripts/build.sh -P <name>

  Available Presets:
    all        All platforms and architectures
    release    Release build (same as all)
    linux      Linux all architectures (amd64, arm64, armv7)
    macos      macOS all architectures (amd64, arm64)
    darwin     macOS all architectures (alias)
    windows    Windows all architectures (amd64, x86)
    arm        ARM all architectures (arm64, armv7)
    server     Server deploy (Linux amd64 + arm64)
    desktop    Desktop (Windows + macOS)
    quick      Quick build (skip tests and lint)

  Custom Build:
    ./scripts/build.sh -p linux,windows -a x64,arm64

  Parameters:
    -P <name>     Preset config (long: --preset)
    -p <list>     Target platforms, comma-separated (long: --platforms)
    -a <list>     Target architectures, comma-separated (long: --architectures)
    -t            Skip Go tests (long: --skip-tests)
    -l            Skip code checks (long: --skip-lint)
    -L            List all available presets (long: --list-presets)
    -h            Show this help (long: --help)

  Examples:
    ./scripts/build.sh -P all
    ./scripts/build.sh -P windows
    ./scripts/build.sh -P quick
    ./scripts/build.sh -P server -t
    ./scripts/build.sh -p linux -a x64

EOF
}

list_presets() {
  if [[ ! -f "$PROFILES_FILE" ]]; then
    echo "  Profiles config not found: $PROFILES_FILE"
    return 1
  fi

  printf "\n  Available Presets:\n\n"
  printf "  %-12s %-36s %s\n" "Name" "Description" "Targets"
  printf "  %-12s %-36s %s\n" "----" "-----------" "-------"

  if command -v jq >/dev/null 2>&1; then
    jq -r '.presets | to_entries | sort_by(.key)[] | "\(.key)\t\(.value.description // "")\t\(.value.platforms | join(","))\t\(.value.architectures | join(","))\t\(.value.skipTests // false)\t\(.value.skipLint // false)"' "$PROFILES_FILE" | \
    while IFS=$'\t' read -r name desc platforms archs skipTests skipLint; do
      flags=""
      [[ "$skipTests" == "true" ]] && flags+=" [skipTests]"
      [[ "$skipLint" == "true" ]] && flags+=" [skipLint]"
      target="$platforms | $archs$flags"
      printf "  %-12s %-36s %s\n" "$name" "$desc" "$target"
    done
  elif command -v python3 >/dev/null 2>&1; then
    python3 -c "
import json, sys
with open('$PROFILES_FILE') as f:
    data = json.load(f)
for name in sorted(data['presets']):
    p = data['presets'][name]
    desc = p.get('description', '')
    platforms = ','.join(p['platforms'])
    archs = ','.join(p['architectures'])
    flags = ''
    if p.get('skipTests'): flags += ' [skipTests]'
    if p.get('skipLint'): flags += ' [skipLint]'
    target = f'{platforms} | {archs}{flags}'
    print(f'  {name:<12} {desc:<36} {target}')
"
  else
    echo "  jq or python3 required to parse profiles config"
    return 1
  fi
  echo ''
}

load_preset() {
  local name="$1"
  local lower
  lower="$(echo "$name" | tr '[:upper:]' '[:lower:]')"

  if [[ ! -f "$PROFILES_FILE" ]]; then
    log "Profiles config not found: $PROFILES_FILE" "WARN"
    return 1
  fi

  if command -v jq >/dev/null 2>&1; then
    if ! jq -e ".presets.\"$lower\"" "$PROFILES_FILE" >/dev/null 2>&1; then
      log "Unknown preset: $name" "ERROR"
      echo ""
      echo "  Available presets:"
      jq -r '.presets | to_entries | sort_by(.key)[] | "    \(.key)  - \(.value.description // "")"' "$PROFILES_FILE"
      echo ""
      exit 1
    fi
    PLATFORMS="$(jq -r ".presets.\"$lower\".platforms | join(\",\")" "$PROFILES_FILE")"
    ARCHITECTURES="$(jq -r ".presets.\"$lower\".architectures | join(\",\")" "$PROFILES_FILE")"
    local preset_skip_tests preset_skip_lint
    preset_skip_tests="$(jq -r ".presets.\"$lower\".skipTests // false" "$PROFILES_FILE")"
    preset_skip_lint="$(jq -r ".presets.\"$lower\".skipLint // false" "$PROFILES_FILE")"
    [[ "$preset_skip_tests" == "true" && "$SKIP_TESTS" != "true" ]] && SKIP_TESTS="true"
    [[ "$preset_skip_lint" == "true" && "$SKIP_LINT" != "true" ]] && SKIP_LINT="true"
    local desc
    desc="$(jq -r ".presets.\"$lower\".description // \"\"" "$PROFILES_FILE")"
    log "Using preset: $lower ($desc)" "INFO"
  elif command -v python3 >/dev/null 2>&1; then
    local result
    result="$(python3 -c "
import json, sys
with open('$PROFILES_FILE') as f:
    data = json.load(f)
presets = data.get('presets', {})
p = presets.get('$lower')
if not p:
    print('ERROR:Unknown preset: $name', file=sys.stderr)
    for n in sorted(presets):
        d = presets[n].get('description', '')
        print(f'    {n}  - {d}', file=sys.stderr)
    sys.exit(1)
print(','.join(p['platforms']))
print(','.join(p['architectures']))
print(str(p.get('skipTests', False)).lower())
print(str(p.get('skipLint', False)).lower())
print(p.get('description', ''))
" 2>&1)" || {
      log "Unknown preset or parse failed: $name" "ERROR"
      exit 1
    }
    PLATFORMS="$(echo "$result" | sed -n '1p')"
    ARCHITECTURES="$(echo "$result" | sed -n '2p')"
    local preset_skip_tests preset_skip_lint desc
    preset_skip_tests="$(echo "$result" | sed -n '3p')"
    preset_skip_lint="$(echo "$result" | sed -n '4p')"
    desc="$(echo "$result" | sed -n '5p')"
    [[ "$preset_skip_tests" == "true" && "$SKIP_TESTS" != "true" ]] && SKIP_TESTS="true"
    [[ "$preset_skip_lint" == "true" && "$SKIP_LINT" != "true" ]] && SKIP_LINT="true"
    log "Using preset: $lower ($desc)" "INFO"
  else
    log "jq or python3 required to parse profiles config" "ERROR"
    exit 1
  fi

  log "  Platforms: $PLATFORMS" "INFO"
  log "  Architectures: $ARCHITECTURES" "INFO"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -P|--preset)
      PRESET="$2"
      shift 2
      ;;
    -p|--platforms)
      PLATFORMS="$2"
      shift 2
      ;;
    -a|--architectures)
      ARCHITECTURES="$2"
      shift 2
      ;;
    -t|--skip-tests)
      SKIP_TESTS="true"
      shift
      ;;
    -l|--skip-lint)
      SKIP_LINT="true"
      shift
      ;;
    -h|--help)
      SHOW_HELP="true"
      shift
      ;;
    -L|--list-presets)
      LIST_PRESETS="true"
      shift
      ;;
    *)
      echo "Unknown argument: $1"
      exit 1
      ;;
  esac
done

if [[ "$SHOW_HELP" == "true" ]]; then
  show_help
  exit 0
fi

if [[ "$LIST_PRESETS" == "true" ]]; then
  list_presets
  exit 0
fi

if [[ -n "$PRESET" ]]; then
  load_preset "$PRESET"
fi

if [[ -z "$PLATFORMS" && -z "$PRESET" ]]; then
  PLATFORMS="windows"
fi
if [[ -z "$ARCHITECTURES" && -z "$PRESET" ]]; then
  ARCHITECTURES="x64"
fi

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
    a="${a,,}"
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
