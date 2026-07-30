#!/usr/bin/env bash
# check-wire-contract.sh — Verifies Go and Swift JSON wire keys match the
# canonical contract in .github/wire-contract.json.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CONTRACT="$REPO_ROOT/.github/wire-contract.json"
STATUS=0

echo "=== Wire Contract Check ==="
echo "Contract: $CONTRACT"

# 1) Validate the contract JSON itself.
python3 -c "import json; json.load(open('$CONTRACT'))" || {
    echo "FAIL: contract JSON is invalid"; exit 1; }
echo "  ✓ Contract JSON is valid"

# 2) Run the Go contract test.
cd "$REPO_ROOT"
if go test ./internal/config/ -run TestWireContract -count=1 2>&1 | grep -q "^ok\s"; then
    echo "  ✓ Go JSON tags match contract"
else
    echo "  FAIL: Go JSON tags mismatch with contract"
    STATUS=1
fi

# 3) Check Swift source files for expected camelCase property names.
echo ""
echo "--- Swift Codable alignment ---"
SWIFT_CYCLE="$REPO_ROOT/client/Sources/SymvibeKit/Models/Cycle.swift"
SWIFT_EVENT="$REPO_ROOT/client/Sources/SymvibeKit/Models/Event.swift"

check_swift() {
    local file="$1" name="$2" expected="$3"
    local camel
    camel=$(python3 -c "
import re, sys
keys = sys.argv[1].split()
# Known abbreviation overrides (snake_case -> Swift camelCase).
overrides = {'run_id': 'runID', 'step_id': 'stepID'}
camel = [overrides.get(k, re.sub(r'_(.)', lambda m: m.group(1).upper(), k)) for k in keys]
print(' '.join(camel))
" "$expected")

    for prop in $camel; do
        if grep -Eq "(public\s+)?(let|var)\s+$prop(\s|:|\?|=)" "$file" 2>/dev/null; then
            :
        elif grep -qE "case\s+\w+\s*=\s*\"$prop\"" "$file" 2>/dev/null; then
            :
        else
            echo "  FAIL: $name missing Swift property '$prop'"
            STATUS=1
        fi
    done
}

if [ -f "$SWIFT_CYCLE" ] && [ -f "$SWIFT_EVENT" ]; then
    check_swift "$SWIFT_CYCLE" "Cycle"              "schema_version id name description phases"
    check_swift "$SWIFT_CYCLE" "Phase"              "id name order steps"
    check_swift "$SWIFT_CYCLE" "Step"               "id name order skill category agent prompt_suffix enabled model_override backend_override auto_skip requires_review depends_on parallel_safe status"
    check_swift "$SWIFT_CYCLE" "AutoSkip"           "sensor when"
    check_swift "$SWIFT_CYCLE" "RequiresReview"     "when"
    check_swift "$SWIFT_CYCLE" "StepModelOverride"  "id temperature variant fallback_models"
    check_swift "$SWIFT_EVENT" "Event"              "type run_id step_id status kind line state ts"
    check_swift "$SWIFT_EVENT" "RunState"           "state run_id current_step cycle mode"

    # Check StepStatus enum values.
    for status in pending in_progress done skipped failed blocked needs_review; do
        if ! grep -Eq "(case $status\b|case\s+\w+\s*=\s*\"$status\")" "$SWIFT_CYCLE" 2>/dev/null; then
            echo "  FAIL: StepStatus missing Swift case '$status'"
            STATUS=1
        fi
    done
    echo "  ✓ StepStatus values complete"
else
    echo "  SKIP: Swift source files not found"
fi

if [ "$STATUS" -eq 0 ]; then
    echo ""
    echo "✓ Wire contract check PASSED"
else
    echo ""
    echo "✗ Wire contract check FAILED — update contract or source files"
fi
exit $STATUS
