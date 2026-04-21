#!/bin/bash
# PostToolUse hook: Warn about missing boundary tests
# ADVISORY: Numeric validation needs boundary tests (tdd.md)

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Only process Write/Edit for Go files
if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi

if [[ ! "$FILE_PATH" =~ \.go$ ]]; then
    exit 0
fi

# Skip test files (we're checking if tests exist)
if [[ "$FILE_PATH" =~ _test\.go$ ]]; then
    exit 0
fi

# Check if file exists
if [[ ! -f "$FILE_PATH" ]]; then
    exit 0
fi

YELLOW='\033[33m'
RESET='\033[0m'

WARNINGS=()

# Patterns that indicate numeric validation needing boundary tests
VALIDATION_PATTERNS=(
    'if .* > [0-9]'
    'if .* < [0-9]'
    'if .* >= [0-9]'
    'if .* <= [0-9]'
    'if .* > 0x'
    'if .* < 0x'
    'return .*Invalid.*Range'
    'return .*OutOfBounds'
    'return .*Exceeds'
)

FOUND_VALIDATION=""
for pattern in "${VALIDATION_PATTERNS[@]}"; do
    if grep -qE "$pattern" "$FILE_PATH" 2>/dev/null; then
        FOUND_VALIDATION="yes"
        break
    fi
done

if [[ -n "$FOUND_VALIDATION" ]]; then
    # Check for corresponding test file
    TEST_FILE="${FILE_PATH%%.go}_test.go"

    if [[ ! -f "$TEST_FILE" ]]; then
        WARNINGS+=("Numeric validation found but no test file: $TEST_FILE")
    else
        # Check if test file has boundary tests
        if ! grep -qiE 'boundary|invalid.*above|invalid.*below|max.*valid|min.*valid' "$TEST_FILE" 2>/dev/null; then
            WARNINGS+=("Numeric validation found but no boundary tests in $TEST_FILE")
        fi
    fi
fi

if [[ ${#WARNINGS[@]} -gt 0 ]]; then
    echo -e "${YELLOW}⚠️  Missing boundary tests:${RESET}" >&2
    for warn in "${WARNINGS[@]}"; do
        echo -e "  ${YELLOW}$warn${RESET}" >&2
    done
    echo -e "  ${YELLOW}→ Add tests for: last valid, first invalid below, first invalid above${RESET}" >&2
    echo -e "  ${YELLOW}→ See ai/rules/tdd.md${RESET}" >&2
fi

exit 0
