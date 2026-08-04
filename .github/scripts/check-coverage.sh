#!/bin/bash
set -euo pipefail

# Run the Go test suite with coverage and enforce the repo-configured threshold
# stored in .github/coverage-config.json.
#
# Additionally emits machine-readable artifacts in the repo root:
#   coverage.json  - total, threshold, commit SHA
#   badge.json     - shields.io endpoint-compatible badge payload
#   badge.svg      - static shields-style badge
# These are published to the orphan `coverage-data` branch by the
# publish-coverage-data CI job (main pushes only). The threshold check below
# is unchanged: a failing threshold still exits non-zero and fails CI.

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

THRESHOLD=$(awk -F': ' '/threshold/ { gsub(/[^0-9.]/, "", $2); print $2 }' .github/coverage-config.json)
if [ -z "$THRESHOLD" ]; then
  echo "Could not read coverage threshold from .github/coverage-config.json"
  exit 1
fi

# COVERAGE_PROFILE (optional): reuse an existing coverprofile instead of
# re-running the test suite (used by CI, where build-test already produced
# one with -race). Unset = run the suite locally as before.
PROFILE="${COVERAGE_PROFILE:-}"
if [ -z "$PROFILE" ]; then
  PROFILE=coverage.out
  CGO_ENABLED=0 go test ./... -coverprofile="$PROFILE"
fi
TOTAL=$(go tool cover -func="$PROFILE" | grep '^total:' | awk '{print $3}' | tr -d '%')

echo "Total coverage: ${TOTAL}%"
echo "Threshold: ${THRESHOLD}%"

SHA="${GITHUB_SHA:-$(git rev-parse HEAD)}"

COLOR=$(awk -v total="$TOTAL" -v threshold="$THRESHOLD" 'BEGIN {
  if (total < threshold) { print "red" } else { print "brightgreen" }
}')

cat > coverage.json <<EOF
{
  "total": ${TOTAL},
  "threshold": ${THRESHOLD},
  "commit": "${SHA}"
}
EOF

cat > badge.json <<EOF
{
  "schemaVersion": 1,
  "label": "coverage",
  "message": "${TOTAL}%",
  "color": "${COLOR}"
}
EOF

# Static shields-style SVG badge. Text width approximated at 6.5px/char (Verdana 11).
MSG="${TOTAL}%"
LABEL="coverage"
LABEL_W=$(awk -v s="$LABEL" 'BEGIN { printf "%d", length(s)*6.5 + 10 }')
MSG_W=$(awk -v s="$MSG" 'BEGIN { printf "%d", length(s)*6.5 + 10 }')
TOTAL_W=$((LABEL_W + MSG_W))
LABEL_CX=$((LABEL_W / 2))
MSG_CX=$((LABEL_W + MSG_W / 2))
FILL="#4c1"
if [ "$COLOR" = "red" ]; then FILL="#e05d44"; fi

cat > badge.svg <<EOF
<svg xmlns="http://www.w3.org/2000/svg" width="${TOTAL_W}" height="20" role="img" aria-label="${LABEL}: ${MSG}">
  <title>${LABEL}: ${MSG}</title>
  <linearGradient id="s" x2="0" y2="100%">
    <stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
    <stop offset="1" stop-opacity=".1"/>
  </linearGradient>
  <clipPath id="r"><rect width="${TOTAL_W}" height="20" rx="3" fill="#fff"/></clipPath>
  <g clip-path="url(#r)">
    <rect width="${LABEL_W}" height="20" fill="#555"/>
    <rect x="${LABEL_W}" width="${MSG_W}" height="20" fill="${FILL}"/>
    <rect width="${TOTAL_W}" height="20" fill="url(#s)"/>
  </g>
  <g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" text-rendering="geometricPrecision" font-size="110">
    <text aria-hidden="true" x="${LABEL_CX}0" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="$(( (LABEL_W - 10) * 10 ))">${LABEL}</text>
    <text x="${LABEL_CX}0" y="140" transform="scale(.1)" fill="#fff" textLength="$(( (LABEL_W - 10) * 10 ))">${LABEL}</text>
    <text aria-hidden="true" x="${MSG_CX}0" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="$(( (MSG_W - 10) * 10 ))">${MSG}</text>
    <text x="${MSG_CX}0" y="140" transform="scale(.1)" fill="#fff" textLength="$(( (MSG_W - 10) * 10 ))">${MSG}</text>
  </g>
</svg>
EOF

echo "Wrote coverage.json, badge.json, badge.svg (commit ${SHA})"

awk -v total="$TOTAL" -v threshold="$THRESHOLD" 'BEGIN {
  if (total < threshold) {
    printf "Coverage %.1f%% is below threshold %.1f%%\n", total, threshold
    exit 1
  }
  printf "Coverage %.1f%% meets threshold %.1f%%\n", total, threshold
}'
