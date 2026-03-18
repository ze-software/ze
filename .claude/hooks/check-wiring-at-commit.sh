#!/bin/bash
# PreToolUse hook: Warn when committing plugin code without functional tests
# WARNING (exit 1): New plugin code without .ci tests suggests unwired feature
# This is the #1 recurring failure mode (integration-completeness.md)

INPUT=$(cat)
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

# Only trigger on git commit commands
if [[ "$COMMAND" != *"git commit"* ]]; then
    exit 0
fi

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || exit 0

# Get staged files (added or modified)
STAGED=$(git diff --cached --name-only --diff-filter=AM 2>/dev/null || true)
if [[ -z "$STAGED" ]]; then
    exit 0
fi

YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

# Check: new/modified Go files in plugin directories (not tests, not register.go)
PLUGIN_GO=$(echo "$STAGED" | grep -E '^internal/plugins/.*\.go$' \
    | grep -v '_test\.go$' \
    | grep -v 'register\.go$' \
    | grep -v '/schema/' \
    | grep -v 'doc\.go$' || true)

if [[ -z "$PLUGIN_GO" ]]; then
    exit 0
fi

# Check: any .ci files also staged?
CI_FILES=$(echo "$STAGED" | grep -E '\.ci$' || true)

if [[ -z "$CI_FILES" ]]; then
    echo -e "${YELLOW}${BOLD}⚠️  Plugin code staged without functional tests${RESET}" >&2
    echo "" >&2
    echo -e "  Plugin files:" >&2
    echo "$PLUGIN_GO" | while read -r f; do echo -e "    -> $f" >&2; done
    echo "" >&2
    echo -e "  ${YELLOW}No .ci functional tests in this commit.${RESET}" >&2
    echo -e "  ${YELLOW}Is this feature reachable by a user through config/CLI/API?${RESET}" >&2
    echo -e "  ${YELLOW}See .claude/rules/integration-completeness.md${RESET}" >&2
    # Exit 1 = warning, not blocking (refactors/bugfixes may be legitimately code-only)
    exit 1
fi

exit 0
