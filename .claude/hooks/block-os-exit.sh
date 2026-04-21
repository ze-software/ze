#!/bin/bash
# PreToolUse hook: Block os.Exit() in handler code
# BLOCKING: Return exit codes, never os.Exit() in handlers (cli-patterns.md)

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // .tool_input.new_string // empty')

if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi
if [[ ! "$FILE_PATH" =~ \.go$ ]]; then
    exit 0
fi
if [[ "$FILE_PATH" =~ _test\.go$ ]]; then
    exit 0
fi

# Allow os.Exit in main.go entry points
if [[ "$FILE_PATH" =~ /main\.go$ ]]; then
    exit 0
fi

# Allow os.Exit in register.go — plugin init() must abort on registration failure
if [[ "$FILE_PATH" =~ /register\.go$ ]]; then
    exit 0
fi

# Skip scripts/ - //go:build ignore build tools call os.Exit on fatal errors
if [[ "$FILE_PATH" =~ /scripts/ ]]; then
    exit 0
fi

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()

# Detect os.Exit() calls (excluding comments)
OS_EXIT_MATCHES=$(echo "$CONTENT" | grep -nE 'os\.Exit\(' | grep -viE '//.*os\.Exit' | head -3 || true)

if [[ -n "$OS_EXIT_MATCHES" ]]; then
    ERRORS+=("os.Exit() in handler code -- return exit codes instead:")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  $line")
    done <<< "$OS_EXIT_MATCHES"
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}❌ BLOCKED: os.Exit() in handler${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "  ${YELLOW}Return exit codes, never os.Exit() in handlers${RESET}" >&2
    echo -e "  ${YELLOW}See ai/rules/cli-patterns.md${RESET}" >&2
    exit 2
fi

exit 0
