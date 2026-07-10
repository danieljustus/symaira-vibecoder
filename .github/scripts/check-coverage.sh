#!/bin/bash
set -euo pipefail

# Run the Go test suite with coverage and enforce the repo-configured threshold
# stored in .github/coverage-config.json.

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

THRESHOLD=$(awk -F': ' '/threshold/ { gsub(/[^0-9.]/, "", $2); print $2 }' .github/coverage-config.json)
if [ -z "$THRESHOLD" ]; then
  echo "Could not read coverage threshold from .github/coverage-config.json"
  exit 1
fi

CGO_ENABLED=0 go test ./... -coverprofile=coverage.out
TOTAL=$(go tool cover -func=coverage.out | grep '^total:' | awk '{print $3}' | tr -d '%')

echo "Total coverage: ${TOTAL}%"
echo "Threshold: ${THRESHOLD}%"

awk -v total="$TOTAL" -v threshold="$THRESHOLD" 'BEGIN {
  if (total < threshold) {
    printf "Coverage %.1f%% is below threshold %.1f%%\n", total, threshold
    exit 1
  }
  printf "Coverage %.1f%% meets threshold %.1f%%\n", total, threshold
}'
