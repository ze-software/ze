#!/bin/bash
# PreToolUse hook: Block string `+` concatenation in Go source files.
# BLOCKING: Use textbuf.Buffer or standalone textbuf functions instead.
# Reference: ai/rules/no-sprintf-alloc.md "String + Concatenation is BANNED"
#
# Detects patterns like:
#   "literal" + expr
#   expr + "literal"
#   s + s (where context suggests string)
#
# Allows: arithmetic (i + 1), compile-time const literals, test files.

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // .tool_input.new_string // empty')

# Only process Write/Edit for Go files.
if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi

if [[ ! "$FILE_PATH" =~ \.go$ ]]; then
    exit 0
fi

# Skip test files.
if [[ "$FILE_PATH" =~ _test\.go$ ]]; then
    exit 0
fi

# Skip empty content.
if [[ -z "$CONTENT" ]]; then
    exit 0
fi

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

# Detect string concatenation: "..." + expr  OR  expr + "..."
# Exclude: const declarations, comment lines, filepath/path.Join
MATCHES=$(echo "$CONTENT" | grep -nE '("[^"]*"\s*\+\s*[^"=]|[^"=]\s*\+\s*"[^"]*")' \
    | grep -vE '^\s*//' \
    | grep -vE '(^[0-9]+:)?\s*const\s' \
    | grep -vE '"[^"]*"\s*\+\s*"[^"]*"' \
    | grep -vE '//.*"[^"]*"\s*\+' \
    | grep -vE 'filepath\.(Join|Dir|Base)' \
    | grep -vE 'path\.(Join|Dir|Base)' \
    | head -5 || true)

if [[ -n "$MATCHES" ]]; then
    echo -e "${RED}${BOLD}BLOCKED: string + concatenation in ${FILE_PATH}${RESET}" >&2
    echo "" >&2
    while IFS= read -r line; do
        [[ -n "$line" ]] && echo -e "  ${RED}x${RESET} $line" >&2
    done <<< "$MATCHES"
    echo "" >&2
    echo -e "  ${YELLOW}Use textbuf.Buffer chain instead of + concatenation.${RESET}" >&2
    echo -e "  ${YELLOW}Rule: ai/rules/no-sprintf-alloc.md${RESET}" >&2
    exit 2
fi

exit 0
