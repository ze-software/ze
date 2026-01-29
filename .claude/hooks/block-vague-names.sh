#!/bin/bash
# PostToolUse hook: Warn about vague variable names
# WARNING: Use precise names (design-principles.md)

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

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

# Check if file exists
if [[ ! -f "$FILE_PATH" ]]; then
    exit 0
fi

YELLOW='\033[33m'
RESET='\033[0m'

# Check for vague variable names (exported or struct fields)
# These are the worst offenders per design-principles.md
VAGUE_NAMES=$(grep -nE '\b(Data|Info|Result|Item|Thing|Temp|Tmp|Val|Obj)\s+\w+\s*=' "$FILE_PATH" 2>/dev/null | head -3 || true)

if [[ -n "$VAGUE_NAMES" ]]; then
    echo -e "${YELLOW}⚠️  Vague variable names detected:${RESET}" >&2
    while IFS= read -r line; do
        [[ -n "$line" ]] && echo -e "  ${YELLOW}$line${RESET}" >&2
    done <<< "$VAGUE_NAMES"
    echo -e "  ${YELLOW}→ Use precise names: wireBytes, peerConfig, parseResult${RESET}" >&2
    echo -e "  ${YELLOW}→ See .claude/rules/design-principles.md${RESET}" >&2
fi

exit 0
