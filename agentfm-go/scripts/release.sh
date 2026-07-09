#!/usr/bin/env bash
set -euo pipefail

# Cross-compile the agentfm mesh node for every supported platform,
# strip + trim, and checksum into dist/. One binary covers every role
# (worker / api / relay / witness) via -mode.
HERE="$(cd "$(dirname "$0")" && pwd)"
GO_ROOT="$(cd "$HERE/.." && pwd)"
OUT="$GO_ROOT/dist"

cd "$GO_ROOT"
mkdir -p "$OUT"
rm -f "$OUT"/agentfm-* "$OUT"/checksums.txt 2>/dev/null || true

VERSION="$(grep -oE '"[0-9][^"]*"' internal/version/version.go | head -1 | tr -d '"')"
echo "Building agentfm ${VERSION}"

declare -a TARGETS=("darwin/arm64" "darwin/amd64" "linux/amd64" "windows/amd64")

for target in "${TARGETS[@]}"; do
  IFS='/' read -r os arch <<< "$target"
  ext=""
  [ "$os" = "windows" ] && ext=".exe"
  out="$OUT/agentfm-${os}-${arch}${ext}"
  echo "  -> $out"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags="-s -w" -o "$out" "./cmd/agentfm"
done

cd "$OUT"
if command -v shasum >/dev/null; then
  shasum -a 256 agentfm-* > checksums.txt
fi
echo ""
echo "Artifacts in $OUT:"
ls -lh "$OUT"
