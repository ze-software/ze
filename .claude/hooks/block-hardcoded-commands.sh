#!/bin/bash
# PreToolUse hook: Block hardcoded command/subcommand string lists in Go code.
# Rule: ai/rules/derive-not-hardcode.md
#
# Catches []string{} literals that look like hardcoded command or subcommand
# lists. These should be derived from cmdregistry, yangVerbs, or similar.
# Exception: test files, YANG files, register.go (Subs field is OK if derived).

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

CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // .tool_input.new_string // empty')

if [[ -z "$CONTENT" ]]; then
    exit 0
fi

# Detect []string{ ... } literals (single- OR multi-line) that contain 4+
# command-like words -- the signature of a hardcoded command catalog that
# should be derived from the registry.
#
# The previous version grep'd line-by-line, so it (a) missed the idiomatic
# gofmt multi-line literal entirely and (b) false-flagged short arg slices like
# []string{"bgp", "show", "summary"}. This awk scan accumulates each []string{}
# block across lines and flags only when the command-word count reaches the
# threshold, fixing both the false negatives and the false positives.
HITS=$(echo "$CONTENT" | awk '
    BEGIN { rx = "\"(show|set|del|update|validate|monitor|clear|help|config|bgp|cli|schema|plugin|doctor|version|signal|completion|status|init)\"" }
    /^[[:space:]]*\/\// { next }            # skip comment-only lines
    {
        line = $0
        if (!inblk && line ~ /\[\]string\{/) { inblk = 1; words = 0; startln = NR; first = line }
        if (inblk) {
            words += gsub(rx, "&", line)
            if (line ~ /\}/) {
                if (words >= 4) print startln ": " first
                inblk = 0
            }
        }
    }
' | head -3 || true)

if [[ -n "$HITS" ]]; then
    echo -e "\033[31m\033[1m✘ BLOCKED: possible hardcoded command list in ${FILE_PATH}\033[0m" >&2
    echo "" >&2
    while IFS= read -r line; do
        [[ -n "$line" ]] && echo -e "  \033[31mx\033[0m $line" >&2
    done <<< "$HITS"
    echo "" >&2
    echo -e "  \033[33mDerive command lists from cmdregistry.ListRoot(), yangVerbs, or similar.\033[0m" >&2
    echo -e "  \033[33mSee host/register.go sectionList() for the pattern.\033[0m" >&2
    echo -e "  \033[33mRule: ai/rules/derive-not-hardcode.md\033[0m" >&2
    exit 2
fi

exit 0
