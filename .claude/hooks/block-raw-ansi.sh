#!/bin/bash
# PreToolUse hook: Block raw ANSI escape codes in Go production code.
# Rule: ai/rules/no-sprintf-alloc.md (textbuf.Buffer section)
#
# ANSI codes must come from textbuf.Color* constants or textbuf.C,
# never as literal "\033[" strings in application code.
# Exception: textbuf.go itself (defines the constants), helpfmt.go
# (uses textbuf constants via const aliases), test files.

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

# Allow textbuf.go (defines the constants) and helpfmt.go (uses them via const)
if [[ "$FILE_PATH" =~ textbuf\.go$ || "$FILE_PATH" =~ helpfmt\.go$ ]]; then
    exit 0
fi

CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // .tool_input.new_string // empty')

if [[ -z "$CONTENT" ]]; then
    exit 0
fi

HITS=$(echo "$CONTENT" \
    | grep -nE '\\033\[|\\x1b\[|\\e\[' \
    | grep -vE '//.*\\033' \
    | head -5 || true)

if [[ -n "$HITS" ]]; then
    echo -e "\033[31m\033[1m✘ BLOCKED: raw ANSI escape code in ${FILE_PATH}\033[0m" >&2
    echo "" >&2
    while IFS= read -r line; do
        [[ -n "$line" ]] && echo -e "  \033[31mx\033[0m $line" >&2
    done <<< "$HITS"
    echo "" >&2
    echo -e "  \033[33mUse textbuf.C.BoldCyan (or textbuf.ColorBoldCyan) instead of raw codes.\033[0m" >&2
    echo -e "  \033[33mSet tb.SetColor(bool) on the buffer, then tb.Colored(c.BoldCyan).\033[0m" >&2
    echo -e "  \033[33mRule: ai/rules/no-sprintf-alloc.md (textbuf.Buffer section)\033[0m" >&2
    exit 2
fi

exit 0
