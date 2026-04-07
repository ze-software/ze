#!/bin/bash
# PreToolUse hook: Block panic for error handling
# BLOCKING: panic() should not be used for error handling (go-standards.md)

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

# Skip test files - tests may use panic for test failures
if [[ "$FILE_PATH" =~ _test\.go$ ]]; then
    exit 0
fi

# Skip scripts/ - //go:build ignore build tools may use panic for fatal errors
if [[ "$FILE_PATH" =~ /scripts/ ]]; then
    exit 0
fi

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()

# Check for panic() calls (but not recover())
# Allow panic("unreachable") and panic("not implemented") as valid uses
PANIC_LINES=$(echo "$CONTENT" | grep -nE '\bpanic\s*\(' | grep -vE 'panic\s*\(\s*"(unreachable|not implemented|unimplemented|TODO|BUG|impossible)' || true)

if [[ -n "$PANIC_LINES" ]]; then
    ERRORS+=("panic() used for error handling")
    ERRORS+=("→ Return error instead of panicking")
    ERRORS+=("→ Allowed: panic(\"unreachable\"), panic(\"not implemented\")")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  Line: $line")
    done <<< "$(echo "$PANIC_LINES" | head -3)"
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}❌ Return error, don't panic()${RESET}" >&2
    exit 2
fi

exit 0
