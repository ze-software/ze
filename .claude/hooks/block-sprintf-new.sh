#!/bin/bash
# PreToolUse hook: Block new fmt.Sprintf/Fprintf/Printf in Go production code.
# Rule: ai/rules/no-sprintf-alloc.md
#
# Checks only ADDED lines (new_string for Edit, content for Write).
# Allowed: fmt.Errorf (error wrapping), test files, fmt.Fprintf(os.Stdout/os.Stderr).

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

# Check 1: fmt.Sprintf/Fprintf/Printf (not Errorf, not os.Stdout/Stderr)
HITS=$(echo "$CONTENT" \
    | grep -nE 'fmt\.(Sprintf|Fprintf|Printf)\(' \
    | grep -vE 'fmt\.Fprintf\(os\.(Stdout|Stderr)' \
    | grep -vE '//.*fmt\.(Sprintf|Fprintf|Printf)' \
    | head -5 || true)

# Check 2: strconv.FormatUint/FormatInt (always use AppendUint/AppendInt or textbuf)
HITS2=$(echo "$CONTENT" \
    | grep -nE 'strconv\.Format(Uint|Int)\(' \
    | grep -vE '//.*strconv\.Format' \
    | head -5 || true)

if [[ -n "$HITS" || -n "$HITS2" ]]; then
    echo -e "\033[31m\033[1m✘ BLOCKED: banned format primitive in ${FILE_PATH}\033[0m" >&2
    echo "" >&2
    while IFS= read -r line; do
        [[ -n "$line" ]] && echo -e "  \033[31mx\033[0m $line" >&2
    done <<< "${HITS}${HITS2}"
    echo "" >&2
    echo -e "  \033[33mUse textbuf.Buffer or standalone textbuf.* functions instead.\033[0m" >&2
    echo -e "  \033[33mstrconv.FormatUint/FormatInt: use textbuf.Uint*/Int or AppendUint/AppendInt.\033[0m" >&2
    echo -e "  \033[33mAllowed: fmt.Errorf, fmt.Fprintf(os.Stdout/Stderr), test files.\033[0m" >&2
    echo -e "  \033[33mRule: ai/rules/no-sprintf-alloc.md\033[0m" >&2
    exit 2
fi

exit 0
