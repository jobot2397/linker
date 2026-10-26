#!/usr/bin/env bash
# Cross-compile linker binaries for the v0.1.0 release matrix.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${VERSION:-0.1.0}"
OUT="${OUT:-$ROOT/dist}"
MODULE="github.com/jobot2397/linker/cmd/linker"

mkdir -p "$OUT"

targets=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
  "windows/arm64"
)

echo "Building linker v${VERSION} → ${OUT}"
failed=()
succeeded=()

for target in "${targets[@]}"; do
  os="${target%/*}"
  arch="${target#*/}"
  ext=""
  if [[ "$os" == "windows" ]]; then
    ext=".exe"
  fi
  name="linker_${VERSION}_${os}_${arch}${ext}"
  echo -n "  ${os}/${arch} … "
  if GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 \
      go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
      -o "${OUT}/${name}" "$ROOT/cmd/linker"; then
    echo "ok → ${name}"
    succeeded+=("$name")
  else
    echo "FAILED"
    failed+=("$target")
  fi
done

echo
echo "Succeeded (${#succeeded[@]}):"
printf '  %s\n' "${succeeded[@]:-}"
if ((${#failed[@]})); then
  echo "Failed (${#failed[@]}):"
  printf '  %s\n' "${failed[@]}"
  exit 1
fi

# Checksums
(
  cd "$OUT"
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 linker_* > SHA256SUMS
  elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum linker_* > SHA256SUMS
  fi
)

echo
echo "Artifacts in ${OUT}:"
ls -la "$OUT"
