#!/usr/bin/env bash
set -euo pipefail

# Cross-compile the agentfm Go binary for the platforms the Electron app
# supports, naming each output with Node's platform/arch convention
# (process.platform-process.arch) so it matches what backend-manager.ts
# resolves at runtime: `agentfm-${process.platform}-${process.arch}`.
#
# Go's GOARCH "amd64" is Node's "x64", and GOOS "windows" is Node's "win32" —
# mismatching these leaves Intel/Windows installs unable to find the backend.
#
# Pass "mac" as the first arg to build only the macOS binaries.

HERE="$(cd "$(dirname "$0")" && pwd)"
DESKTOP_ROOT="$(cd "$HERE/.." && pwd)"
CORE_ROOT="$(cd "$DESKTOP_ROOT/../agentfm-go" && pwd)"
OUT_DIR="$DESKTOP_ROOT/resources"

if [ ! -d "$CORE_ROOT" ]; then
  echo "Error: agentfm-go not found at $CORE_ROOT" >&2
  exit 1
fi

mkdir -p "$OUT_DIR"

# Each entry: GOOS/GOARCH/NODE_PLATFORM/NODE_ARCH
declare -a TARGETS=(
  "darwin/arm64/darwin/arm64"
  "darwin/amd64/darwin/x64"
  "linux/amd64/linux/x64"
  "windows/amd64/win32/x64"
)

if [ "${1:-}" = "mac" ]; then
  TARGETS=(
    "darwin/arm64/darwin/arm64"
    "darwin/amd64/darwin/x64"
  )
  echo "Building macOS binaries only."
  rm -f "$OUT_DIR"/agentfm-* 2>/dev/null || true
fi

cd "$CORE_ROOT"

for target in "${TARGETS[@]}"; do
  IFS='/' read -r goos goarch nodeos nodearch <<< "$target"
  ext=""
  [ "$goos" = "windows" ] && ext=".exe"
  output="$OUT_DIR/agentfm-${nodeos}-${nodearch}${ext}"
  echo "Building $output  (GOOS=$goos GOARCH=$goarch)..."
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags="-s -w" -o "$output" ./cmd/agentfm
done

echo ""
echo "Built binaries in $OUT_DIR:"
ls -lh "$OUT_DIR"/agentfm-*
