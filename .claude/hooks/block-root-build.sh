#!/bin/bash
# PreToolUse hook: Block go build without -o bin/ (prevents binaries in project root)
# Blocking (exit 2)

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')

if [[ "$TOOL_NAME" != "Bash" ]]; then
    exit 0
fi

COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

# Only check commands that contain "go build"
if ! echo "$COMMAND" | grep -qE '(^|[^[:alnum:]_])go[[:space:]]+build([^[:alnum:]_]|$)'; then
    exit 0
fi

# Allow if -o flag points to bin/
if echo "$COMMAND" | grep -qE '\-o[[:space:]]+bin/'; then
    exit 0
fi

# Allow "go build ./..." (check-only, no binary output)
if echo "$COMMAND" | grep -qE 'go[[:space:]]+build[[:space:]]+\./\.\.\.'; then
    exit 0
fi

# Allow "go build -v ./..." style checks
if echo "$COMMAND" | grep -qE 'go[[:space:]]+build[[:space:]]+(-[[:alnum:]_]+[[:space:]]+)*\./\.\.\.'; then
    exit 0
fi

# Block: go build without -o bin/
RED='\033[31m'
BOLD='\033[1m'
RESET='\033[0m'
echo -e "${RED}${BOLD}✘ BLOCKED: go build without -o bin/${RESET}" >&2
echo "" >&2
echo -e "  ${RED}→${RESET} Use: go build -o bin/<name> ./cmd/<name>" >&2
echo -e "  ${RED}→${RESET} Or: make ze / make chaos / make test-runner" >&2
exit 2
