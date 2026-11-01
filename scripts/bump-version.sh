#!/usr/bin/env bash
# Bump VERSION and sync defaults in main.go + Formula.
# Usage: ./scripts/bump-version.sh 0.2.0
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
NEW="${1:-}"
if [[ -z "$NEW" || ! "$NEW" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-].+)?$ ]]; then
  echo "usage: $0 <semver>   e.g. $0 0.2.0" >&2
  exit 1
fi

OLD="$(tr -d '[:space:]' <"$ROOT/VERSION")"
echo "$NEW" >"$ROOT/VERSION"
echo "VERSION: ${OLD} → ${NEW}"

# Default fallback in main (ldflags override at release build time).
if [[ -f "$ROOT/cmd/linker/main.go" ]]; then
  perl -i -pe "s/var version = \"[^\"]+\"/var version = \"${NEW}\"/" "$ROOT/cmd/linker/main.go"
fi

# README badge line if present.
if grep -q '^\*\*Version:\*\*' "$ROOT/README.md" 2>/dev/null; then
  perl -i -pe "s/^\*\*Version:\*\*.*/**Version:** ${NEW}/" "$ROOT/README.md"
fi

# Refresh formula tag/revision for the new version (tag must exist for revision).
"$ROOT/scripts/update-homebrew-formula.sh" "$NEW" || {
  echo "note: formula version string updated; run update-homebrew-formula.sh after tagging v${NEW}" >&2
  perl -i -pe "s/version \"[^\"]+\"/version \"${NEW}\"/" "$ROOT/Formula/linker.rb"
  perl -i -pe "s/tag:\\s+\"v[^\"]+\"/tag:      \"v${NEW}\"/" "$ROOT/Formula/linker.rb"
}

echo "Next:"
echo "  git commit -am \"chore: bump version to v${NEW}\""
echo "  git tag -a v${NEW} -m \"linker v${NEW}\""
echo "  git push && git push origin v${NEW}"
echo "  ./scripts/update-homebrew-formula.sh   # pin formula revision to the tag"
