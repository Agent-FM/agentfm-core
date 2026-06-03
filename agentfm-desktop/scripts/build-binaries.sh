#!/usr/bin/env bash
set -euo pipefail

# Cross-compile the agentfm Go binary for all platforms the Electron app supports.
# Run before `npm run package`.

HERE="$(cd "$(dirname "$0")" && pwd)"
DESKTOP_ROOT="$(cd "$HERE/.." && pwd)"
CORE_ROOT="$(cd "$DESKTOP_ROOT/../agentfm-core/agentfm-go" && pwd)"
OUT_DIR="$DESKTOP_ROOT/resources"

if [ ! -d "$CORE_ROOT" ]; then
  echo "Error: agentfm-core not found at $CORE_ROOT" >&2
  exit 1
fi

mkdir -p "$OUT_DIR"

declare -a TARGETS=(
  "darwin/arm64"
  "darwin/amd64"
  "linux/amd64"
  "windows/amd64"
)

cd "$CORE_ROOT"

for target in "${TARGETS[@]}"; do
  IFS='/' read -r os arch <<< "$target"
  ext=""
  [ "$os" = "windows" ] && ext=".exe"
  output="$OUT_DIR/agentfm-${os}-${arch}${ext}"
  echo "Building $output..."
  GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags="-s -w" -o "$output" ./cmd/agentfm
done

echo ""
echo "Built $(ls "$OUT_DIR" | wc -l | tr -d ' ') binaries:"
ls -lh "$OUT_DIR"
