#!/bin/bash
# PreToolUse hook: detect observer-exit antipattern in .ci files.
#
# WARNING (exit 1): Python observer block uses sys.exit(1) without runtime_fail.
# This is a silent false-positive: the runner checks ze's exit code, ze has
# already exited 0 from the clean shutdown by the time sys.exit(1) runs.
# See ai/rules/testing.md "Observer-Exit Antipattern" and the cmd-4 fix
# (1fc98747).
#
# The hook is non-blocking (exit 1) so legitimate edits to known-broken files
# can proceed during migration. New tests added by Claude in this session
# should never trigger it -- if they do, fix the test first.

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Only check .ci files
if [[ "$FILE_PATH" != *.ci ]]; then
    exit 0
fi

# Resolve content depending on tool. For Write, the full content is in
# tool_input.content. For Edit, the new content is tool_input.new_string;
# we apply the heuristic against the new fragment alone.
case "$TOOL_NAME" in
    Write)
        CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // empty')
        ;;
    Edit|MultiEdit)
        CONTENT=$(echo "$INPUT" | jq -r '.tool_input.new_string // empty')
        ;;
    *)
        exit 0
        ;;
esac

# Heuristic: presence of sys.exit(1) inside the Python observer area AND
# absence of runtime_fail. tmpfs=*.run lines mark observer script blocks.
if ! echo "$CONTENT" | grep -q 'sys\.exit(1)'; then
    exit 0
fi
if echo "$CONTENT" | grep -q 'runtime_fail'; then
    exit 0
fi
# Tests with explicit stderr assertions can also be valid -- the assertion
# carries the failure signal. Skip if the file uses expect=stderr or
# reject=stderr directives.
if echo "$CONTENT" | grep -qE '^(expect|reject)=stderr'; then
    exit 0
fi

YELLOW='\033[33m'
BOLD='\033[1m'
RESET='\033[0m'

echo -e "${YELLOW}${BOLD}WARN: observer-exit antipattern in $FILE_PATH${RESET}" >&2
echo "" >&2
echo -e "  ${YELLOW}Found:${RESET} sys.exit(1) inside Python observer block" >&2
echo -e "  ${YELLOW}Missing:${RESET} runtime_fail() call AND expect/reject=stderr assertion" >&2
echo "" >&2
echo -e "  This pattern is a silent false-positive: the runner checks ze's exit" >&2
echo -e "  code, but ze exits 0 from the clean shutdown before the Python observer" >&2
echo -e "  reaches sys.exit(1). The test passes regardless of the assertion." >&2
echo "" >&2
echo -e "  ${YELLOW}Fix one of:${RESET}" >&2
echo -e "    1. Use runtime_fail('reason') from ze_api -- emits ZE-OBSERVER-FAIL" >&2
echo -e "       sentinel that runner_validate.go detects as a failure." >&2
echo -e "    2. Add expect=stderr:pattern=<production log line> on the daemon's" >&2
echo -e "       own decision log (preferred -- tests production code path)." >&2
echo "" >&2
echo -e "  ${YELLOW}See:${RESET} ai/rules/testing.md \"Observer-Exit Antipattern\"" >&2
echo -e "  ${YELLOW}Reference fix:${RESET} 1fc98747 (cmd-4 prefix filter)" >&2

# Non-blocking warning so legitimate migrations can proceed.
exit 1
