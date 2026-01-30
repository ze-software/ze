#!/bin/bash
# PreToolUse hook: Block functions with "And" in name
# BLOCKING: Single responsibility - no ParseAndValidate (design-principles.md)

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

# Check for function names with "And" (violates single responsibility)
# Pattern: func FooAndBar or func (x *T) FooAndBar
AND_FUNCS=$(echo "$CONTENT" | grep -nE '^func\s+(\([^)]+\)\s+)?[A-Z][a-zA-Z]*And[A-Z]' || true)

if [[ -n "$AND_FUNCS" ]]; then
    ERRORS+=("Function with 'And' in name (violates single responsibility):")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  $line")
    done <<< "$(echo "$AND_FUNCS" | head -3)"
    ERRORS+=("→ Split into separate functions")
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}❌ BLOCKED: Single responsibility violation${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "  ${YELLOW}Each function should do ONE thing${RESET}" >&2
    echo -e "  ${YELLOW}See .claude/rules/design-principles.md${RESET}" >&2
    exit 1
fi

exit 0
