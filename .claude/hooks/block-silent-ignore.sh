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

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()

# Check for silent ignore patterns
SILENT_PATTERNS=(
    'continue\s*//\s*ignore'
    'return\s*nil\s*//\s*ignore'
    '//\s*silently\s*ignore'
    '//\s*skip\s*unknown'
    'default:\s*$'  # Empty default in switch (may silently ignore)
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

# Check for swallowing errors in config parsing
if [[ "$FILE_PATH" =~ /config/ ]]; then
    if echo "$CONTENT" | grep -qE 'default:\s*//|default:\s*break|default:\s*$'; then
        ERRORS+=("Config parsing with empty default case")
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
