#!/bin/bash
# PreToolUse hook: Require // Design: comment in .go files
# Blocking (exit 2) — prevents writing .go files without Design references
# See ai/rules/design-doc-references.md

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

# Skip exempt files per design-doc-references.md
BASE=$(basename "$FILE_PATH")
if [[ "$BASE" =~ _test\.go$ ]] || \
   [[ "$BASE" =~ _gen\.go$ ]] || \
   [[ "$BASE" == "register.go" ]] || \
   [[ "$BASE" == "embed.go" ]] || \
   [[ "$BASE" == "doc.go" ]]; then
    exit 0
fi

YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

# For Write: check the content being written
if [[ "$TOOL_NAME" == "Write" ]]; then
    CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // empty')
    if echo "$CONTENT" | grep -q '// Design:'; then
        exit 0
    fi
    # Skip generated files
    CHECK=$(echo "$CONTENT" | head -c 500)
    if echo "$CHECK" | grep -qE 'Code generated|DO NOT EDIT'; then
        exit 0
    fi
fi

# For Edit: check the file on disk (before edit)
if [[ "$TOOL_NAME" == "Edit" ]]; then
    if [[ -f "$FILE_PATH" ]] && grep -q '// Design:' "$FILE_PATH"; then
        exit 0
    fi
    # Maybe the edit is adding the Design comment
    NEW_STRING=$(echo "$INPUT" | jq -r '.tool_input.new_string // empty')
    if echo "$NEW_STRING" | grep -q '// Design:'; then
        exit 0
    fi
    # Skip generated files
    if [[ -f "$FILE_PATH" ]] && head -c 500 "$FILE_PATH" | grep -qE 'Code generated|DO NOT EDIT'; then
        exit 0
    fi
fi

# Missing Design comment — BLOCKING
RED='\033[31m'
echo -e "${RED}${BOLD}✘ BLOCKED: Missing // Design: comment${RESET}" >&2
echo "" >&2
echo -e "  ${RED}!${RESET} File: $FILE_PATH" >&2
echo -e "  ${RED}→${RESET} Add: // Design: docs/architecture/<doc>.md — topic" >&2
echo -e "  ${RED}→${RESET} See ai/rules/design-doc-references.md" >&2
exit 2
