#!/bin/bash
# PreToolUse hook: Block YAGNI violations
# BLOCKING: No speculative features (design-principles.md)

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // .tool_input.new_string // empty')

# Only process Write/Edit for Go files
if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi

if [[ ! "$FILE_PATH" =~ \.go$ ]]; then
    exit 0
fi

# Skip test files
if [[ "$FILE_PATH" =~ _test\.go$ ]]; then
    exit 0
fi

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()

# Check for YAGNI violation patterns in comments
YAGNI_PATTERNS=(
    'in case we need'
    'might be useful'
    'for future use'
    'someday'
    'just in case'
    'maybe later'
    'could be extended'
    'placeholder for'
    'reserved for future'
    'not yet implemented.*TODO'
)

for pattern in "${YAGNI_PATTERNS[@]}"; do
    MATCHES=$(echo "$CONTENT" | grep -niE "$pattern" | head -2 || true)
    if [[ -n "$MATCHES" ]]; then
        ERRORS+=("YAGNI violation: '$pattern'")
        while IFS= read -r line; do
            [[ -n "$line" ]] && ERRORS+=("  $line")
        done <<< "$MATCHES"
    fi
done

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}❌ BLOCKED: YAGNI violation${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "  ${YELLOW}Build what's needed NOW, not what MIGHT be needed${RESET}" >&2
    echo -e "  ${YELLOW}See ai/rules/design-principles.md${RESET}" >&2
    exit 2
fi

exit 0
