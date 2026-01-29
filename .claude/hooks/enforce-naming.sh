#!/bin/bash
# PreToolUse hook: Enforce file naming conventions
# BLOCKING: Wrong naming conventions (documentation.md)

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Only process Write tool (new files)
if [[ "$TOOL_NAME" != "Write" ]]; then
    exit 0
fi

# Only for new files
if [[ -f "$FILE_PATH" ]]; then
    exit 0
fi

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()
BASENAME=$(basename "$FILE_PATH")

# Markdown files: must be lowercase-hyphens (except README, INDEX, CLAUDE)
if [[ "$FILE_PATH" =~ \.md$ ]]; then
    # Allow uppercase special files
    if [[ "$BASENAME" =~ ^(README|INDEX|CLAUDE|LICENSE|CONTRIBUTING|CHANGELOG)\.md$ ]]; then
        exit 0
    fi

    # Check for uppercase or underscores
    if [[ "$BASENAME" =~ [A-Z] ]]; then
        ERRORS+=("Markdown files must be lowercase: $BASENAME")
        ERRORS+=("→ Use: $(echo "$BASENAME" | tr '[:upper:]' '[:lower:]')")
    fi

    if [[ "$BASENAME" =~ _ ]]; then
        ERRORS+=("Markdown files use hyphens, not underscores: $BASENAME")
        ERRORS+=("→ Use: $(echo "$BASENAME" | tr '_' '-')")
    fi
fi

# Go files: must be snake_case (lowercase with underscores)
if [[ "$FILE_PATH" =~ \.go$ ]]; then
    # Check for hyphens
    if [[ "$BASENAME" =~ - ]]; then
        ERRORS+=("Go files use underscores, not hyphens: $BASENAME")
        ERRORS+=("→ Use: $(echo "$BASENAME" | tr '-' '_')")
    fi

    # Check for uppercase (except in build tags like _linux.go)
    if [[ "$BASENAME" =~ [A-Z] ]] && [[ ! "$BASENAME" =~ ^[a-z_]+_[A-Z] ]]; then
        ERRORS+=("Go files must be lowercase: $BASENAME")
        ERRORS+=("→ Use: $(echo "$BASENAME" | tr '[:upper:]' '[:lower:]')")
    fi
fi

# Shell scripts: must be lowercase-hyphens
if [[ "$FILE_PATH" =~ \.sh$ ]]; then
    if [[ "$BASENAME" =~ [A-Z] ]]; then
        ERRORS+=("Shell scripts must be lowercase: $BASENAME")
    fi
    if [[ "$BASENAME" =~ _ ]]; then
        ERRORS+=("Shell scripts use hyphens: $BASENAME")
        ERRORS+=("→ Use: $(echo "$BASENAME" | tr '_' '-')")
    fi
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}❌ BLOCKED: Naming convention violation${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "  ${YELLOW}See .claude/rules/documentation.md${RESET}" >&2
    exit 1
fi

exit 0
