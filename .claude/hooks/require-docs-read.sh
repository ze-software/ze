#!/bin/bash
# PreToolUse hook: Require architecture docs read before spec writing
# WARNING: Reminds about keyword->doc mapping (planning.md)

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // empty')

# Only process Write for spec files
if [[ "$TOOL_NAME" != "Write" ]]; then
    exit 0
fi

if [[ ! "$FILE_PATH" =~ plan/spec-.*\.md$ ]]; then
    exit 0
fi

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

RED='\033[31m'
YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

ERRORS=()
WARNINGS=()

# Load helpers for per-spec state file resolution
source .claude/hooks/lib/state-file.sh

SESSION_STATE=$(_state_file)

# Check if session state exists
if [[ ! -f "$SESSION_STATE" ]]; then
    ERRORS+=("No session state ($SESSION_STATE) - cannot verify docs were read")
    ERRORS+=("-> Create session state and list docs read before writing spec")
fi

# Extract keywords from spec content to suggest docs
KEYWORDS=""

# FlowSpec keywords
if echo "$CONTENT" | grep -qiE 'flowspec|traffic.?filter'; then
    KEYWORDS="$KEYWORDS flowspec"
    if [[ -f "$SESSION_STATE" ]] && ! grep -q "nlri-flowspec.md" "$SESSION_STATE" 2>/dev/null; then
        WARNINGS+=("FlowSpec spec but nlri-flowspec.md not in session-state")
        WARNINGS+=("-> Read: docs/architecture/wire/nlri-flowspec.md")
    fi
fi

# API keywords
if echo "$CONTENT" | grep -qiE '(^|[^[:alnum:]_])api([^[:alnum:]_]|$)|command|plugin|announce|withdraw'; then
    KEYWORDS="$KEYWORDS api"
    if [[ -f "$SESSION_STATE" ]] && ! grep -q "api/architecture.md" "$SESSION_STATE" 2>/dev/null; then
        WARNINGS+=("API spec but api/architecture.md not in session-state")
        WARNINGS+=("-> Read: docs/architecture/api/architecture.md")
    fi
fi

# NLRI keywords
if echo "$CONTENT" | grep -qiE '(^|[^[:alnum:]_])nlri([^[:alnum:]_]|$)|prefix|mp.?reach'; then
    KEYWORDS="$KEYWORDS nlri"
    if [[ -f "$SESSION_STATE" ]] && ! grep -q "wire/nlri.md" "$SESSION_STATE" 2>/dev/null; then
        WARNINGS+=("NLRI spec but wire/nlri.md not in session-state")
        WARNINGS+=("-> Read: docs/architecture/wire/nlri.md")
    fi
fi

# Output errors (blocking)
if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo -e "${RED}${BOLD}BLOCKED: Docs not verified${RESET}" >&2
    echo "" >&2
    for err in "${ERRORS[@]}"; do
        echo -e "  ${RED}x${RESET} $err" >&2
    done
    echo "" >&2
    echo -e "  ${YELLOW}See ai/rules/planning.md (keyword->doc mapping)${RESET}" >&2
    exit 1
fi

# Output warnings (non-blocking)
if [[ ${#WARNINGS[@]} -gt 0 ]]; then
    echo -e "${YELLOW}Architecture docs may not be read:${RESET}" >&2
    for warn in "${WARNINGS[@]}"; do
        echo -e "  ${YELLOW}${warn}${RESET}" >&2
    done
    echo -e "  ${YELLOW}-> Add doc names to session state after reading${RESET}" >&2
fi

exit 0
