#!/bin/bash
# PreToolUse hook: Block temporary debug statements
# BLOCKING: Use slogutil, not fmt.Print* for debugging (go-standards.md)
#
# Debug logging is PERMANENT - use slog, it stays in the code forever.
# Controlled by env vars (ze.log.*), not by adding/removing print statements.

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

# Skip test files (tests can use debug output)
if [[ "$FILE_PATH" =~ _test\.go$ ]]; then
    exit 0
fi

# Skip cmd/ files that legitimately use fmt for CLI output
if [[ "$FILE_PATH" =~ cmd/ ]]; then
    exit 0
fi

# Skip scripts/ - //go:build ignore build tools that legitimately use fmt for output
if [[ "$FILE_PATH" =~ /scripts/ ]]; then
    exit 0
fi

# Skip register.go — plugin init() uses fmt.Fprintf(os.Stderr) for fatal errors
if [[ "$FILE_PATH" =~ /register\.go$ ]]; then
    exit 0
fi

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()

# Block println() - always wrong in production code
PRINTLN=$(echo "$CONTENT" | grep -nE '^\s*println\s*\(' | head -2 || true)
if [[ -n "$PRINTLN" ]]; then
    ERRORS+=("println() found (use slogutil):")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  $line")
    done <<< "$PRINTLN"
fi

# Block fmt.Print* to stderr (always debug output)
STDERR_PRINT=$(echo "$CONTENT" | grep -nE 'fmt\.Fprint.*os\.Stderr' | head -2 || true)
if [[ -n "$STDERR_PRINT" ]]; then
    ERRORS+=("fmt.Fprint* to Stderr found (use slogutil):")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  $line")
    done <<< "$STDERR_PRINT"
fi

# Block fmt.Print* with debug-like content
DEBUG_PRINT=$(echo "$CONTENT" | grep -niE 'fmt\.Print.*"(DEBUG|debug|TRACE|trace|>>>|<<<|---|\*\*\*|XXX|FIXME)' | head -2 || true)
if [[ -n "$DEBUG_PRINT" ]]; then
    ERRORS+=("Debug fmt.Print* found (use slogutil):")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  $line")
    done <<< "$DEBUG_PRINT"
fi

# Block bare fmt.Println with simple string (likely debug)
# Exception: strings containing "error", "fail", "warn", "usage", "help"
BARE_PRINTLN=$(echo "$CONTENT" | grep -nE 'fmt\.Println\s*\(\s*"[^"]{1,50}"\s*\)' | grep -viE 'error|fail|warn|usage|help|version' | head -2 || true)
if [[ -n "$BARE_PRINTLN" ]]; then
    ERRORS+=("Bare fmt.Println found (use slogutil for debug, or proper error handling):")
    while IFS= read -r line; do
        [[ -n "$line" ]] && ERRORS+=("  $line")
    done <<< "$BARE_PRINTLN"
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}❌ BLOCKED: Temporary debug statement${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}✗${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "  ${YELLOW}Debug logging is PERMANENT. Use slogutil:${RESET}" >&2
    echo -e "  ${YELLOW}  var logger = slogutil.Logger(\"subsystem\")${RESET}" >&2
    echo -e "  ${YELLOW}  logger.Debug(\"message\", \"key\", value)${RESET}" >&2
    echo -e "  ${YELLOW}${RESET}" >&2
    echo -e "  ${YELLOW}Enable at runtime: ze.log.subsystem=debug${RESET}" >&2
    echo -e "  ${YELLOW}See .claude/rules/go-standards.md${RESET}" >&2
    exit 2
fi

exit 0
