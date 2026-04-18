#!/bin/bash
# PreToolUse hook: Block source / learned-summary edits when active spec is not in-progress.
#
# BLOCKING (exit 2): forces flipping spec status BEFORE writing code or the
# learned summary. The spec MUST be `in-progress` while implementation is
# happening, not flipped retroactively at the end.
#
# Why: rules/planning.md "When to Update (BLOCKING)" requires the flip at the
# START of implementation. This is the #1 recurring spec-workflow miss
# (rules/memory.md "Spec Status Updated at End Instead of Beginning"). The
# rule existed; only enforcement was missing.
#
# Triggers when ANY of these are written/edited while the session-selected
# spec is in `skeleton`, `design`, or `ready`:
#   - internal/**/*.go, pkg/**/*.go, cmd/**/*.go (Go source)
#   - test/** (functional tests, .ci files)
#   - plan/learned/** (the "wrote summary before flipping status" trap)
#
# Allowed regardless: editing the spec itself (plan/spec-*.md), docs,
# rules, hooks, scripts, the Makefile -- you need those to flip status,
# write the spec, and configure the workflow.

set -e

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

if [[ "$TOOL_NAME" != "Write" && "$TOOL_NAME" != "Edit" && "$TOOL_NAME" != "MultiEdit" ]]; then
    exit 0
fi

[ -z "$FILE_PATH" ] && exit 0

# Only enforce on source / test / learned-summary paths.
case "$FILE_PATH" in
    */internal/*.go) ;;
    */pkg/*.go) ;;
    */cmd/*.go) ;;
    */test/*) ;;
    */plan/learned/*) ;;
    *) exit 0 ;;
esac

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

# Resolve this session's selected spec via the session marker.
# shellcheck source=lib/state-file.sh
source .claude/hooks/lib/state-file.sh

SID=$(_session_id)
MARKER="tmp/session/.session-${SID}"
[ -f "$MARKER" ] || exit 0

SELECTED=$(head -1 "$MARKER" 2>/dev/null)
[ -z "$SELECTED" ] && exit 0
[ "$SELECTED" = "unassigned" ] && exit 0

SPEC_PATH="plan/$SELECTED"
[ -f "$SPEC_PATH" ] || exit 0

# Parse Status from the metadata table row: "| Status | <value> |"
STATUS=$(awk -F'|' '
    /^\| *Status *\|/ {
        gsub(/^[[:space:]]+|[[:space:]]+$/, "", $3)
        print $3
        exit
    }
' "$SPEC_PATH")
[ -z "$STATUS" ] && exit 0

case "$STATUS" in
    skeleton|design|ready)
        RED='\033[31m'; BOLD='\033[1m'; RESET='\033[0m'
        TODAY=$(date +%Y-%m-%d)
        {
            printf "%bBLOCKED:%b spec %b%s%b is %b\`%s\`%b\n" \
                "$RED$BOLD" "$RESET" "$BOLD" "$SELECTED" "$RESET" "$BOLD" "$STATUS" "$RESET"
            echo ""
            echo "  Editing source code, tests, or a learned summary requires the"
            printf "  spec to be %b\`in-progress\`%b first. Flipping the status AFTER\n" \
                "$BOLD" "$RESET"
            echo "  the work is the #1 recurring spec-workflow miss."
            echo ""
            echo "  Edit $SPEC_PATH NOW:"
            echo "    | Status  | $STATUS  ->  in-progress |"
            echo "    | Phase   | -        ->  1/N          |"
            echo "    | Updated | <date>   ->  $TODAY    |"
            echo ""
            echo "  Then retry the edit."
            echo "  Reference: rules/planning.md \"When to Update (BLOCKING)\","
            echo "             rules/memory.md \"Spec Status Updated at End...\""
        } >&2
        exit 2
        ;;
esac

exit 0
