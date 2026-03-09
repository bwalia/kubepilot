#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_NAME="${1:-kubepilot}"
OUT_PATH="${2:-$ROOT_DIR/dist/$BIN_NAME}"
CMD_PKG="${3:-./cmd/kubepilot}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command '$1' not found in PATH" >&2
    exit 1
  fi
}

parse_go_directive() {
  awk '/^go[[:space:]]+[0-9]+\.[0-9]+(\.[0-9]+)?$/ {print $2; exit}' "$ROOT_DIR/go.mod"
}

version_ge() {
  local installed="$1"
  local required="$2"
  local ia ib ic ra rb rc

  IFS='.' read -r ia ib ic <<<"$installed"
  IFS='.' read -r ra rb rc <<<"$required"
  ic="${ic:-0}"
  rc="${rc:-0}"

  (( ia > ra )) && return 0
  (( ia < ra )) && return 1
  (( ib > rb )) && return 0
  (( ib < rb )) && return 1
  (( ic >= rc ))
}

require_cmd go

GO_REQUIRED="$(parse_go_directive)"
if [[ -z "$GO_REQUIRED" ]]; then
  echo "error: could not read a valid 'go' directive from go.mod" >&2
  exit 1
fi

GO_INSTALLED="$(go version | awk '{print $3}' | sed 's/^go//')"
if [[ -z "$GO_INSTALLED" ]]; then
  echo "error: unable to detect installed Go version" >&2
  exit 1
fi

if ! version_ge "$GO_INSTALLED" "$GO_REQUIRED"; then
  echo "error: go $GO_REQUIRED+ is required, found go $GO_INSTALLED" >&2
  echo "hint: install/update Go (for macOS: brew install go)" >&2
  exit 1
fi

mkdir -p "$(dirname "$OUT_PATH")"

# Force internal linking to avoid dyld LC_UUID errors observed on some macOS setups.
export CGO_ENABLED=0
export GOTOOLCHAIN=auto

echo "Building $CMD_PKG -> $OUT_PATH"
(
  cd "$ROOT_DIR"
  go build -trimpath -o "$OUT_PATH" "$CMD_PKG"
)

if [[ "$(uname -s)" == "Darwin" ]] && command -v otool >/dev/null 2>&1; then
  if ! otool -l "$OUT_PATH" | grep -q "LC_UUID"; then
    echo "warning: built binary has no LC_UUID (may fail to launch on macOS)" >&2
  fi
fi

echo "Build complete: $OUT_PATH"
