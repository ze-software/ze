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
            if grep -q "^Status:.*in-progress" "plan/$SPEC" 2>/dev/null; then
                REASONS+=("Spec '$SPEC' still in-progress")
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

# Spec in-progress without stop phrases: warn, don't block
if [ "$HAS_STATE" = true ]; then
    {
        echo "Warning: stopping with in-progress spec."
        for r in "${REASONS[@]}"; do
            echo "  - $r"
        done
    } >&2
    exit 1
fi

exit 0
