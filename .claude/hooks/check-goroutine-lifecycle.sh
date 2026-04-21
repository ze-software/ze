#!/bin/bash
# PreToolUse hook: Block go func() in hot-path files
# BLOCKING: All goroutines must be long-lived workers, not per-event (goroutine-lifecycle.md)

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // .tool_input.new_string // empty')

if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi
if [[ ! "$FILE_PATH" =~ \.go$ ]]; then
    exit 0
fi
if [[ "$FILE_PATH" =~ _test\.go$ ]]; then
    exit 0
fi

# Only check hot-path files: reactor, event loop, dispatch, hub, wire, message processing
IS_HOT=0
if [[ "$FILE_PATH" =~ reactor ]]; then IS_HOT=1; fi
if [[ "$FILE_PATH" =~ /event ]]; then IS_HOT=1; fi
if [[ "$FILE_PATH" =~ /dispatch ]]; then IS_HOT=1; fi
if [[ "$FILE_PATH" =~ /hub/ ]]; then IS_HOT=1; fi
if [[ "$FILE_PATH" =~ /wire/ ]]; then IS_HOT=1; fi
if [[ "$FILE_PATH" =~ /message/ ]]; then IS_HOT=1; fi

if [[ "$IS_HOT" -eq 0 ]]; then
    exit 0
fi

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()

# Detect go func() pattern (per-event goroutine)
GO_FUNC_MATCHES=$(echo "$CONTENT" | grep -nE '^[[:space:]]*go func\(' | head -3 || true)

if [[ -n "$GO_FUNC_MATCHES" ]]; then
    ERRORS+=("go func() in hot-path file -- use channel + worker pattern:")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  $line")
    done <<< "$GO_FUNC_MATCHES"
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}❌ BLOCKED: Per-event goroutine in hot path${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "  ${YELLOW}Pattern: channel + worker. Start -> create chan + start worker. Hot path -> enqueue${RESET}" >&2
    echo -e "  ${YELLOW}go func() is OK for: component startup, test helpers, ProcessManager.Stop()${RESET}" >&2
    echo -e "  ${YELLOW}See ai/rules/goroutine-lifecycle.md${RESET}" >&2
    exit 2
fi

exit 0
