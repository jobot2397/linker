#!/usr/bin/env bash
# Sync Formula/linker.rb version, tag, and git revision.
# Usage:
#   ./scripts/update-homebrew-formula.sh           # uses VERSION + tag v$VERSION
#   ./scripts/update-homebrew-formula.sh 0.2.0     # explicit version
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:-$(tr -d '[:space:]' <"$ROOT/VERSION")}"
TAG="v${VERSION}"
FORMULA="$ROOT/Formula/linker.rb"

if ! git -C "$ROOT" rev-parse "$TAG" >/dev/null 2>&1; then
  echo "tag ${TAG} not found locally — create it first (git tag -a ${TAG})" >&2
  # Still write version + tag; leave revision untouched if present.
  perl -i -pe "s/version \"[^\"]+\"/version \"${VERSION}\"/" "$FORMULA"
  perl -i -pe "s/tag:\\s+\"v[^\"]+\"/tag:      \"${TAG}\"/" "$FORMULA"
  exit 1
fi

REVISION="$(git -C "$ROOT" rev-parse "${TAG}^{commit}")"

perl -i -pe "s/version \"[^\"]+\"/version \"${VERSION}\"/" "$FORMULA"
perl -i -pe "s/tag:\\s+\"v[^\"]+\"/tag:      \"${TAG}\"/" "$FORMULA"
perl -i -pe "s/revision:\\s+\"[0-9a-f]+\"/revision: \"${REVISION}\"/" "$FORMULA"

echo "Updated ${FORMULA}"
echo "  version:   ${VERSION}"
echo "  tag:       ${TAG}"
echo "  revision:  ${REVISION}"
