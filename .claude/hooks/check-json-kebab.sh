#!/bin/bash
# PreToolUse hook: Block non-kebab-case JSON field tags in Go files
# BLOCKING: All JSON keys must be lowercase kebab-case (json-format.md)

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // .tool_input.new_string // empty')

# Only Go files via Write/Edit
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

# Check for camelCase in json tags: json:"camelCase" or json:"someField,omitempty"
CAMEL_MATCHES=$(echo "$CONTENT" | grep -nE '`.*json:"[a-z]+[A-Z]' || true)

# Check for snake_case in json tags: json:"some_field"
SNAKE_MATCHES=$(echo "$CONTENT" | grep -nE '`.*json:"[a-z]+_[a-z]' || true)

if [[ -n "$CAMEL_MATCHES" ]]; then
    ERRORS+=("camelCase JSON tag (must be kebab-case):")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  $line")
    done <<< "$(echo "$CAMEL_MATCHES" | head -3)"
fi

if [[ -n "$SNAKE_MATCHES" ]]; then
    ERRORS+=("snake_case JSON tag (must be kebab-case):")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  $line")
    done <<< "$(echo "$SNAKE_MATCHES" | head -3)"
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}❌ BLOCKED: Non-kebab-case JSON tags${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "  ${YELLOW}All JSON keys must be lowercase kebab-case: json:\"my-field\"${RESET}" >&2
    echo -e "  ${YELLOW}See ai/rules/json-format.md${RESET}" >&2
    exit 2
fi

exit 0
