#!/usr/bin/env bash
set -euo pipefail

# Cross-compile the agentfm mesh node + the standalone relay for every
# supported platform, strip + trim, and checksum into dist/.
HERE="$(cd "$(dirname "$0")" && pwd)"
GO_ROOT="$(cd "$HERE/.." && pwd)"
OUT="$GO_ROOT/dist"

cd "$GO_ROOT"
mkdir -p "$OUT"
rm -f "$OUT"/agentfm-* "$OUT"/relay-* "$OUT"/checksums.txt 2>/dev/null || true

VERSION="$(grep -oE '"[0-9][^"]*"' internal/version/version.go | head -1 | tr -d '"')"
echo "Building agentfm + relay ${VERSION}"

declare -a TARGETS=("darwin/arm64" "darwin/amd64" "linux/amd64" "windows/amd64")

for target in "${TARGETS[@]}"; do
  IFS='/' read -r os arch <<< "$target"
  ext=""
  [ "$os" = "windows" ] && ext=".exe"
  for cmd in agentfm relay; do
    out="$OUT/${cmd}-${os}-${arch}${ext}"
    echo "  -> $out"
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
      go build -trimpath -ldflags="-s -w" -o "$out" "./cmd/${cmd}"
  done
done

cd "$OUT"
if command -v shasum >/dev/null; then
  shasum -a 256 agentfm-* relay-* > checksums.txt
fi
echo ""
echo "Artifacts in $OUT:"
ls -lh "$OUT"
