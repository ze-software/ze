#!/bin/bash
# PreToolUse hook: Block Go file writes without proper preparation
# BLOCKING: Rejects writes to internal/**/*.go without session state

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Only process Write/Edit tools
if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" ]]; then
    exit 0
fi

# Only process Go files in internal/
if [[ ! "$FILE_PATH" =~ ^.*/internal/.*\.go$ ]]; then
    exit 0
fi

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

# Load helpers for per-spec state file resolution
source .claude/hooks/lib/state-file.sh

SESSION_STATE=$(_state_file)
SELECTED_SPEC=""
SID=$(_session_id)
MARKER=".claude/.session-${SID}"
if [ -f "$MARKER" ]; then
    SELECTED_SPEC=$(head -1 "$MARKER" 2>/dev/null)
    [ "$SELECTED_SPEC" = "unassigned" ] && SELECTED_SPEC=""
fi

ERRORS=()
WARNINGS=()

# Check 1: Session state MUST exist for Go work
if [[ ! -f "$SESSION_STATE" ]]; then
    ERRORS+=("No session state found ($SESSION_STATE)")
    ERRORS+=("-> Create session state before writing Go code")
    ERRORS+=("-> Fill in: docs read, decisions made, current task")
fi

# Check 2: If active spec, session state must confirm it was read
if [[ -n "$SELECTED_SPEC" ]]; then
    SPEC_PATH="plan/$SELECTED_SPEC"
    if [[ -f "$SPEC_PATH" ]]; then
        if [[ ! -f "$SESSION_STATE" ]] || ! grep -q "$SELECTED_SPEC" "$SESSION_STATE" 2>/dev/null; then
            ERRORS+=("Active spec '$SELECTED_SPEC' not confirmed read in $SESSION_STATE")
            ERRORS+=("-> Read: $SPEC_PATH")
            ERRORS+=("-> Add spec name to $SESSION_STATE after reading")
        fi
    fi
fi

# Check 3: For new files, check if similar exists (warning only, but logged)
if [[ "$TOOL_NAME" == "Write" && ! -f "$FILE_PATH" ]]; then
    BASENAME=$(basename "$FILE_PATH" .go)
    SIMILAR=$(find internal/ -name "*${BASENAME}*" 2>/dev/null | grep -v "_test.go" | head -5)
    if [[ -n "$SIMILAR" ]]; then
        WARNINGS+=("Creating new file - similar files exist:")
        while IFS= read -r f; do
            [[ -n "$f" ]] && WARNINGS+=("  -> $f")
        done <<< "$SIMILAR"
    fi
fi

# Output (compact)
if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}No session state ($SESSION_STATE) - see post-compaction.md${RESET}" >&2
    exit 2
fi

[[ ${#WARNINGS[@]} -gt 0 ]] && echo -e "${YELLOW}Similar files exist - check first${RESET}" >&2

exit 0
