#!/bin/bash
# PreToolUse hook: Block edits to generated files
# BLOCKING: Rejects Write/Edit to files generated from canonical sources

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi

BASENAME=$(basename "$FILE_PATH")

case "$BASENAME" in
    CLAUDE.md|AGENTS.md)
        echo "BLOCKED: $BASENAME is generated from ai/INSTRUCTIONS.md" >&2
        echo "" >&2
        echo "Edit ai/INSTRUCTIONS.md, then run: make ze-ai-instructions" >&2
        exit 2
        ;;
esac

exit 0
