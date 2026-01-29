#!/bin/bash
# PreToolUse hook: Block ignored errors
# BLOCKING: Must not ignore errors with `_, _ =` pattern (go-standards.md)

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

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()

# Check for ignored error patterns
# Pattern: _, _ = or _ = (when right side is likely error-returning)
# Common patterns: _, _ = fmt.Fprintf, _ = f.Close()

# Direct error ignore: `_ = something()`  where something likely returns error
IGNORED=$(echo "$CONTENT" | grep -nE '^\s*_\s*=\s*\w+\.(Close|Write|Read|Flush|Sync|Remove|Mkdir|Chmod)\s*\(' || true)
if [[ -n "$IGNORED" ]]; then
    ERRORS+=("Ignored error from Close/Write/Read/etc:")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  $line")
    done <<< "$(echo "$IGNORED" | head -3)"
fi

# Two blanks: `_, _ = something()`
DOUBLE_BLANK=$(echo "$CONTENT" | grep -nE '^\s*_\s*,\s*_\s*=' || true)
if [[ -n "$DOUBLE_BLANK" ]]; then
    ERRORS+=("Double blank assignment (ignoring value AND error):")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  $line")
    done <<< "$(echo "$DOUBLE_BLANK" | head -3)"
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}❌ Handle errors: if err != nil { }${RESET}" >&2
    exit 2
fi

exit 0
