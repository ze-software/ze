#!/bin/bash
# PreToolUse hook: Check for existing tests before creating new ones
# WARNING: Avoid duplicate tests (before-writing-code.md)

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Only process Write for new test files
if [[ "$TOOL_NAME" != "Write" ]]; then
    exit 0
fi

# Only for test files
if [[ ! "$FILE_PATH" =~ _test\.go$ ]] && [[ ! "$FILE_PATH" =~ \.ci$ ]]; then
    exit 0
fi

# Only for new files
if [[ -f "$FILE_PATH" ]]; then
    exit 0
fi

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

YELLOW='\033[33m'
RESET='\033[0m'

WARNINGS=()

# Extract base name for searching
BASENAME=$(basename "$FILE_PATH" | sed 's/_test\.go$//' | sed 's/\.ci$//')

# Search for similar test files
if [[ "$FILE_PATH" =~ _test\.go$ ]]; then
    SIMILAR=$(find internal/ -name "*${BASENAME}*_test.go" 2>/dev/null | head -5)
    if [[ -n "$SIMILAR" ]]; then
        WARNINGS+=("Similar test files exist:")
        while IFS= read -r f; do
            [[ -n "$f" ]] && WARNINGS+=("  → $f")
        done <<< "$SIMILAR"
    fi
fi

# Search for similar .ci files
if [[ "$FILE_PATH" =~ \.ci$ ]]; then
    SIMILAR=$(find test/ -name "*${BASENAME}*.ci" 2>/dev/null | head -5)
    if [[ -n "$SIMILAR" ]]; then
        WARNINGS+=("Similar functional tests exist:")
        while IFS= read -r f; do
            [[ -n "$f" ]] && WARNINGS+=("  → $f")
        done <<< "$SIMILAR"
    fi
fi

if [[ ${#WARNINGS[@]} -gt 0 ]]; then
    echo -e "${YELLOW}⚠️  Check for duplicate tests:${RESET}" >&2
    for warn in "${WARNINGS[@]}"; do
        echo -e "  ${YELLOW}${warn}${RESET}" >&2
    done
    echo -e "  ${YELLOW}→ Consider extending existing tests instead${RESET}" >&2
    echo -e "  ${YELLOW}→ See ai/rules/before-writing-code.md${RESET}" >&2
fi

exit 0
