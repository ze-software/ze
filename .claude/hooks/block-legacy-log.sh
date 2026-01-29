#!/bin/bash
# PreToolUse hook: Block legacy log package usage
# BLOCKING: Must use slog, not log package (go-standards.md)

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

# Skip test files - they may legitimately test log output
if [[ "$FILE_PATH" =~ _test\.go$ ]]; then
    exit 0
fi

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()

# Check for legacy log package import
if echo "$CONTENT" | grep -qE '"log"'; then
    # Make sure it's not "log/slog"
    if ! echo "$CONTENT" | grep -qE '"log/slog"'; then
        ERRORS+=("Legacy log package import detected")
        ERRORS+=("→ Use 'log/slog' instead of 'log'")
    fi
fi

# Check for log.Print/Printf/Println/Fatal usage
if echo "$CONTENT" | grep -qE '\blog\.(Print|Printf|Println|Fatal|Fatalf|Fatalln|Panic|Panicf|Panicln)\b'; then
    ERRORS+=("Legacy log.Print/Fatal/Panic usage detected")
    ERRORS+=("→ Use slog.Info/Warn/Error/Debug instead")
    ERRORS+=("→ See .claude/rules/go-standards.md")
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}❌ Use slog, not log package${RESET}" >&2
    exit 2
fi

exit 0
