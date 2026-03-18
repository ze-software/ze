#!/bin/bash
# PostToolUse hook: Warn when Go files exceed size thresholds
# Non-blocking warning at >600, strong warning at >1000 (file-modularity.md)

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi
if [[ ! "$FILE_PATH" =~ \.go$ ]]; then
    exit 0
fi
if [[ "$FILE_PATH" =~ _test\.go$ ]]; then
    exit 0
fi
if [[ ! -f "$FILE_PATH" ]]; then
    exit 0
fi

YELLOW='\033[33m'
RED='\033[31m'
BOLD='\033[1m'
RESET='\033[0m'

LINES=$(wc -l < "$FILE_PATH" | tr -d ' ')

if [[ "$LINES" -gt 1000 ]]; then
    echo -e "${RED}${BOLD}⚠️  File too large: $(basename "$FILE_PATH") ($LINES lines > 1000)${RESET}" >&2
    echo -e "  ${YELLOW}Almost certainly needs splitting by responsibility${RESET}" >&2
    echo -e "  ${YELLOW}See .claude/rules/file-modularity.md${RESET}" >&2
    exit 1
elif [[ "$LINES" -gt 600 ]]; then
    echo -e "${YELLOW}⚠️  File growing: $(basename "$FILE_PATH") ($LINES lines > 600)${RESET}" >&2
    echo -e "  ${YELLOW}Review for multiple concerns${RESET}" >&2
    exit 1
fi

exit 0
