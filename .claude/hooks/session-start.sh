#!/bin/bash
# SessionStart hook - compact status summary with rule reminders
# Creates per-session marker file mapping session to its spec.

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

# Load helpers
source .claude/hooks/lib/state-file.sh

# Clean up stale markers from dead sessions
_cleanup_stale_markers

# --- Claim spec for this session ---
# Read all specs from selected-spec
SELECTED_SPECS=$(grep -v '^#' .claude/selected-spec 2>/dev/null | grep -v '^$')
SELECTED_COUNT=$(echo "$SELECTED_SPECS" | grep -c . 2>/dev/null || true)

# Find which specs are already claimed by other sessions
UNCLAIMED=""
while IFS= read -r spec; do
    [ -z "$spec" ] && continue
    CLAIMED=false
    for marker in .claude/.session-*; do
        [ -f "$marker" ] || continue
        if [ "$(head -1 "$marker" 2>/dev/null)" = "$spec" ]; then
            CLAIMED=true
            break
        fi
    done
    if [ "$CLAIMED" = false ]; then
        UNCLAIMED="${UNCLAIMED:+$UNCLAIMED
}$spec"
    fi
done <<< "$SELECTED_SPECS"

UNCLAIMED_COUNT=$(echo "$UNCLAIMED" | grep -c . 2>/dev/null || true)

# If exactly one unclaimed spec, claim it automatically
if [ "$UNCLAIMED_COUNT" -eq 1 ]; then
    _claim_spec "$(echo "$UNCLAIMED" | head -1)"
elif [ "$UNCLAIMED_COUNT" -eq 0 ] && [ "$SELECTED_COUNT" -eq 1 ]; then
    # Only one spec total and it may be ours (re-attached session)
    _claim_spec "$(echo "$SELECTED_SPECS" | head -1)"
else
    # Multiple unclaimed or zero specs: create empty marker, AI will claim later
    _claim_spec "unassigned"
fi

# --- Display status ---

# Git status summary (compact)
STATUS=$(git status --porcelain 2>/dev/null)
if [ -n "$STATUS" ]; then
    TOTAL=$(echo "$STATUS" | wc -l | tr -d ' ')
    MODIFIED=$(echo "$STATUS" | grep -c '^ M' 2>/dev/null || true)
    ADDED=$(echo "$STATUS" | grep -c '^??' 2>/dev/null || true)
    : "${MODIFIED:=0}" "${ADDED:=0}"
    echo "Warning: ${TOTAL} uncommitted: ${MODIFIED}M ${ADDED}A"
else
    echo "Clean tree"
fi

# Spec display
SPEC_COUNT=$(find plan -maxdepth 1 -name "spec-*.md" 2>/dev/null | wc -l | tr -d ' ')

if [ "$SELECTED_COUNT" -gt 1 ]; then
    echo "Warning: ${SELECTED_COUNT} Claude sessions active on specs:"
    echo "$SELECTED_SPECS" | while read -r spec; do
        [ -f "plan/$spec" ] && echo "   - $spec" || echo "   - $spec (missing)"
    done
    echo "   -> READ your spec BEFORE any work"
elif [ "$SELECTED_COUNT" -eq 1 ] && [ -f "plan/$SELECTED_SPECS" ]; then
    echo "SPEC: $SELECTED_SPECS (+$((SPEC_COUNT-1)) others)"
    echo "   -> READ plan/$SELECTED_SPECS BEFORE any work"
elif [ "$SPEC_COUNT" -gt 0 ]; then
    echo "${SPEC_COUNT} specs, none selected"
fi

# Spec status summary (compact counts by status)
if [ "$SPEC_COUNT" -gt 0 ]; then
    COUNTS=""
    for status in in-progress ready design skeleton blocked deferred; do
        N=$(grep -l "| Status | *${status}" plan/spec-*.md 2>/dev/null | wc -l | tr -d ' ')
        [ "$N" -gt 0 ] && COUNTS="${COUNTS:+$COUNTS, }${N} ${status}"
    done
    [ -n "$COUNTS" ] && echo "   ($COUNTS)"
fi

# Per-spec session state reminder
STATE_FILE=$(_state_file)
if [ -f "$STATE_FILE" ]; then
    LAST_UPDATE=$(head -5 "$STATE_FILE" | grep -o '20[0-9][0-9]-[0-9][0-9]-[0-9][0-9]' | head -1)
    PHASE=$(grep '^Phase:' "$STATE_FILE" 2>/dev/null | head -1 | sed 's/^Phase:\s*//')
    if [ -n "$PHASE" ]; then
        echo "Session state: $STATE_FILE (phase: $PHASE)"
    elif [ -n "$LAST_UPDATE" ]; then
        echo "Session state: $STATE_FILE (updated: $LAST_UPDATE)"
    else
        echo "Session state: $STATE_FILE"
    fi
fi

# Blocking reminders
echo "Warning: BLOCKING: ToolSearch select:LSP -- load BEFORE any work"
echo "Warning: RULE: Read spec + source files BEFORE writing any code"
