#!/bin/bash
# PreToolUse hook: Block silent ignore of config/errors
# BLOCKING: Must fail on unknown, no silent ignore (config-design.md, go-standards.md)

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

# Skip test files
if [[ "$FILE_PATH" =~ _test\.go$ ]]; then
    exit 0
fi

# Skip cmd/ files (CLI routing uses default: with error on next line)
if [[ "$FILE_PATH" =~ cmd/ ]]; then
    exit 0
fi

# Skip internal/test/ files (test code uses switch/default for command dispatch)
if [[ "$FILE_PATH" =~ internal/test/ ]]; then
    exit 0
fi

# Skip scripts/ - //go:build ignore build tools may use switch/default freely
if [[ "$FILE_PATH" =~ /scripts/ ]]; then
    exit 0
fi

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()

# Check for silent ignore patterns (comments + explicit continues/returns)
SILENT_PATTERNS=(
    'continue[[:space:]]*//[[:space:]]*ignore'
    'return[[:space:]]*nil[[:space:]]*//[[:space:]]*ignore'
    '//[[:space:]]*silently[[:space:]]*ignore'
    '//[[:space:]]*skip[[:space:]]*unknown'
)

for pattern in "${SILENT_PATTERNS[@]}"; do
    MATCHES=$(echo "$CONTENT" | grep -niE "$pattern" | head -2 || true)
    if [[ -n "$MATCHES" ]]; then
        # Don't trigger on pattern itself in comments about what NOT to do
        if ! echo "$MATCHES" | grep -qiE '(forbidden|wrong|bad|dont|do not)'; then
            ERRORS+=("Silent ignore pattern detected:")
            while IFS= read -r line; do
                [[ -n "$line" ]] && ERRORS+=("  $line")
            done <<< "$MATCHES"
        fi
    fi
done

# Empty default: case (body is nothing but a closing brace, optionally
# preceded by blank lines or comments). A default: with a real body is
# NOT flagged. Body on the same line (`default: return ...`) is NOT
# flagged. Only the silently-ignore-unknown shape is caught.
EMPTY_DEFAULT=$(echo "$CONTENT" | awk '
    /^[[:space:]]*default:[[:space:]]*$/ {
        in_default = 1
        default_line = NR
        next
    }
    in_default {
        if (/^[[:space:]]*$/ || /^[[:space:]]*\/\//) next
        if (/^[[:space:]]*}[[:space:]]*$/) {
            print "line " default_line ": empty default: (silent ignore)"
        }
        in_default = 0
    }
')
if [[ -n "$EMPTY_DEFAULT" ]]; then
    ERRORS+=("Empty default case (silently ignores unknown values):")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  $line")
    done <<< "$EMPTY_DEFAULT"
fi

# Config parsing: additionally forbid `default: break` and `default: //` on
# the same line (config MUST fail on unknown keys). Empty default: is already
# caught above; this catches the explicit-break shapes.
if [[ "$FILE_PATH" =~ /config/ ]]; then
    if echo "$CONTENT" | grep -qE 'default:[[:space:]]*(//|break[[:space:]]*$)'; then
        ERRORS+=("Config parsing with break/comment-only default case")
        ERRORS+=("→ Must fail on unknown keys, not silently ignore")
    fi
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}❌ BLOCKED: Silent ignore pattern${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "  ${YELLOW}Errors must be handled, not silently ignored${RESET}" >&2
    echo -e "  ${YELLOW}Config must fail on unknown keys${RESET}" >&2
    echo -e "  ${YELLOW}See .claude/rules/config-design.md, go-standards.md${RESET}" >&2
    exit 2
fi

exit 0
