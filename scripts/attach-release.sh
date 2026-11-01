#!/usr/bin/env bash
# Create (or update) a GitHub release and attach dist/ binaries.
# Requires: gh (authenticated), artifacts already built in dist/
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${VERSION:-0.1.0}"
TAG="v${VERSION}"
DIST="${DIST:-$ROOT/dist}"

if ! command -v gh >/dev/null 2>&1; then
  cat <<EOF
gh is not installed or not on PATH.

1. Build binaries:  ./scripts/build-release.sh
2. Install GitHub CLI: https://cli.github.com/
3. Authenticate:      gh auth login
4. Re-run:            ./scripts/attach-release.sh

Or create the release in the browser and upload everything under:
  ${DIST}
EOF
  exit 1
fi

if [[ ! -d "$DIST" ]] || ! compgen -G "${DIST}/linker_*" >/dev/null; then
  echo "No binaries in ${DIST}. Run ./scripts/build-release.sh first."
  exit 1
fi

notes="$(cat <<EOF
## linker ${TAG}

Cross-platform symlink manager for dotfiles (\`linker.toml\`).

### Install
Download the binary for your OS/arch, rename to \`linker\` (or \`linker.exe\`), and put it on your \`PATH\`.

### Windows
Enable Developer Mode (or run an elevated shell) to create symlinks.

### Checksums
See \`SHA256SUMS\` attached to this release.
EOF
)"

if gh release view "$TAG" >/dev/null 2>&1; then
  echo "Release ${TAG} exists — uploading assets…"
  gh release upload "$TAG" "${DIST}"/* --clobber
else
  echo "Creating release ${TAG}…"
  gh release create "$TAG" "${DIST}"/* \
    --title "linker ${TAG}" \
    --notes "$notes"
fi

echo "Done: $(gh release view "$TAG" --json url -q .url)"
