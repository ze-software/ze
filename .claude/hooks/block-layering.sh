#!/bin/bash
# PreToolUse hook: Block layering/compatibility patterns
# BLOCKING: No backwards compatibility, no hybrid systems (no-layering.md)

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

# Check for layering patterns in comments and code
# These indicate keeping old system alongside new

PATTERNS=(
    "backwards.?compatib"
    "backward.?compatib"
    "for.?compatibility"
    "legacy.?support"
    "fallback.?to"
    "hybrid.?approach"
    "gradual.?migration"
    "temporary.?shim"
    "compat.?layer"
    "deprecated.?but.?kept"
)

for pattern in "${PATTERNS[@]}"; do
    MATCHES=$(echo "$CONTENT" | grep -niE "$pattern" || true)
    if [[ -n "$MATCHES" ]]; then
        ERRORS+=("Layering pattern detected: '$pattern'")
        while IFS= read -r line; do
            [[ -n "$line" ]] && ERRORS+=("  $line")
        done <<< "$(echo "$MATCHES" | head -2)"
    fi
done

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}❌ BLOCKED: Layering/compatibility pattern${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "  ${YELLOW}Ze has no users - no backwards compatibility needed${RESET}" >&2
    echo -e "  ${YELLOW}DELETE old code, don't layer new on top${RESET}" >&2
    echo -e "  ${YELLOW}See .claude/rules/no-layering.md${RESET}" >&2
    exit 2
fi

exit 0
