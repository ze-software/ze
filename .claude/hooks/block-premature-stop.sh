#!/bin/bash
# Stop hook: Block premature stopping when work remains
# BLOCKING: Catches ownership-dodging, permission-seeking, and premature handoff
#
# Exit 2 forces Claude to continue instead of stopping.
# Exit 1 warns but allows the stop.
# Exit 0 allows the stop silently.

set -eo pipefail

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

INPUT=$(cat)
TEXT=$(echo "$INPUT" | jq -r '.last_assistant_message // empty' 2>/dev/null)
[ -z "$TEXT" ] && exit 0

REASONS=()

# --- Stop phrase detection ---
# Each line is a grep -iE pattern. First match wins.

PHRASES=(
    # Ownership-dodging: offering instead of doing
    "let me know if you"
    "would you like me to"
    "feel free to"
    "if you.d like me to"
    "if you want me to"
    "happy to help"
    "I can [a-z]+ .* if you"
    # Premature handoff
    "I.ll stop here"
    "I will stop here"
    "I.ll pause here"
    "I will pause here"
    "that.s all for now"
    "I.ll leave .* to you"
    "I will leave .* to you"
    # Permission-seeking: asking instead of doing
    "should I (proceed|continue|go ahead)"
    "do you want me to"
    "^want me to"
    "want me to .* or"
    "shall I (proceed|continue|go ahead|start|keep)"
    "before I proceed"
    "ready for me to"
    "or (leave|skip|ignore) (them|it|this|that)"
    "or should I"
    # Choice-offering: presenting options instead of deciding
    "what would you like"
    "what do you want to do"
    "or something else"
    "what.s next"
    "what next"
)

for pattern in "${PHRASES[@]}"; do
    if echo "$TEXT" | grep -iqE "$pattern"; then
        REASONS+=("Stop phrase: $pattern")
        break
    fi
done

# --- State check: spec in-progress ---

source .claude/hooks/lib/state-file.sh 2>/dev/null || true
SID=$(_session_id 2>/dev/null || echo "")
if [ -n "$SID" ]; then
    MARKER="tmp/session/.session-${SID}"
    if [ -f "$MARKER" ]; then
        SPEC=$(head -1 "$MARKER" 2>/dev/null || true)
        if [ -n "$SPEC" ] && [ "$SPEC" != "unassigned" ] && [ -f "plan/$SPEC" ]; then
            # Closure gate: block if this session's spec is implemented but the
            # second closure commit (git rm the spec) never ran. The detector
            # exits 3 for exactly that "commit A ran, commit B skipped" state.
            # See ai/rules/planning.md "Spec Closure".
            CLOSURE_RC=0
            CLOSURE_MSG=$(python3 scripts/dev/spec-closure-check.py --spec "plan/$SPEC" 2>&1) || CLOSURE_RC=$?
            if [ "$CLOSURE_RC" -eq 3 ]; then
                {
                    echo "BLOCKED: spec implemented but not closed."
                    echo "$CLOSURE_MSG"
                } >&2
                exit 2
            fi
            # Fixed from a dead `^Status:` grep: specs write status as a
            # `| Status | in-progress |` metadata table, so the old anchored
            # pattern never matched a single spec.
            if grep -qE "^\| Status \|.*in-progress" "plan/$SPEC" 2>/dev/null; then
                REASONS+=("Spec '$SPEC' still in-progress")
            fi
            # Delegation nudge: this session claimed a spec and worked it without
            # ever spawning an agent, so it ran the phase inline instead of
            # supervising it (ai/rules/spec-delegation.md). mark-agent-spawned.sh
            # writes the marker on every Agent/Task call.
            #
            # Warn, never block. A session can legitimately claim a spec and do
            # one mechanical edit, and a Stop hook that traps such a session is a
            # worse failure than the one it is catching.
            if [ ! -f "tmp/session/.agent-spawned-${SID}" ]; then
                REASONS+=("Delegation: spec '$SPEC' worked with no subagent spawned")
            fi
        fi
    fi
fi

# --- Decision ---

if [ ${#REASONS[@]} -eq 0 ]; then
    exit 0
fi

HAS_PHRASE=false
HAS_STATE=false
for r in "${REASONS[@]}"; do
    case "$r" in
        "Stop phrase:"*) HAS_PHRASE=true ;;
        "Spec "*) HAS_STATE=true ;;
        "Delegation:"*) HAS_STATE=true ;;
    esac
done

# Stop phrases always block
if [ "$HAS_PHRASE" = true ]; then
    {
        echo "BLOCKED: Premature stop detected."
        for r in "${REASONS[@]}"; do
            echo "  - $r"
        done
        echo "You have work remaining. Continue without asking permission."
    } >&2
    exit 2
fi

# State reasons without stop phrases: warn, don't block.
# The header stays generic because more than one state reason can fire (spec
# still in-progress, phase never delegated) and they are listed individually
# below; naming only one of them in the header misreports the other.
if [ "$HAS_STATE" = true ]; then
    {
        echo "Warning: stopping with open session state."
        for r in "${REASONS[@]}"; do
            echo "  - $r"
        done
    } >&2
    exit 1
fi

exit 0
