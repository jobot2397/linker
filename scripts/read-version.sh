#!/usr/bin/env bash
# Print the project version from VERSION (single source of truth).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
tr -d '[:space:]' <"$ROOT/VERSION"
