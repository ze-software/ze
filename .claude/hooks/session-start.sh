#!/bin/bash
# SessionStart hook - compact status summary with rule reminders
# Creates per-session marker file mapping session to its spec.

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

# Load helpers
source .claude/hooks/lib/state-file.sh

# Clean up stale markers from dead sessions
_cleanup_stale_markers

# Clean up old tmp/ scratch files (>24h) silently
find tmp/ -maxdepth 1 -type f -mmin +1440 -delete 2>/dev/null || true
find tmp/session/ -maxdepth 1 -type f -mmin +1440 -delete 2>/dev/null || true

# --- Claim spec for this session ---
# Read all specs from selected-spec
mkdir -p tmp/session
SELECTED_SPECS=$(grep -v '^#' tmp/session/selected-spec 2>/dev/null | grep -v '^$')
SELECTED_COUNT=$(echo "$SELECTED_SPECS" | grep -c . 2>/dev/null || true)

# Find which specs are already claimed by other sessions
UNCLAIMED=""
while IFS= read -r spec; do
    [ -z "$spec" ] && continue
    CLAIMED=false
    for marker in tmp/session/.session-*; do
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

# --- Auto-transition spec status to in-progress when claimed ---
SID_CHECK=$(_session_id)
MARKER_CHECK="tmp/session/.session-${SID_CHECK}"
CLAIMED_SPEC_NAME=""
if [ -f "$MARKER_CHECK" ]; then
    CLAIMED_SPEC_NAME=$(head -1 "$MARKER_CHECK" 2>/dev/null)
fi
if [ -n "$CLAIMED_SPEC_NAME" ] && [ "$CLAIMED_SPEC_NAME" != "unassigned" ]; then
    SPEC_FILE="plan/$CLAIMED_SPEC_NAME"
    if [ -f "$SPEC_FILE" ]; then
        SPEC_STATUS=$(sed -n 's/^| Status | *\([a-z-]*\).*/\1/p' "$SPEC_FILE" | head -1)
        if [ "$SPEC_STATUS" = "ready" ]; then
            TODAY=$(date +%Y-%m-%d)
            sed -i '' "s/^| Status | *ready.*/| Status | in-progress |/" "$SPEC_FILE"
            sed -i '' "s/^| Updated | *[0-9-]*.*/| Updated | $TODAY |/" "$SPEC_FILE"
            echo "Status: $CLAIMED_SPEC_NAME: ready -> in-progress"
        fi
    fi
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

# Per-session state reminder.
# First check our own state file, then look for previous sessions on the same spec.
STATE_FILE=$(_state_file)
FOUND_STATE=""
if [ -f "$STATE_FILE" ]; then
    FOUND_STATE="$STATE_FILE"
else
    # Look for a previous session's state for the same spec
    SID=$(_session_id)
    MARKER="tmp/session/.session-${SID}"
    CLAIMED_SPEC=""
    if [ -f "$MARKER" ]; then
        CLAIMED_SPEC=$(head -1 "$MARKER" 2>/dev/null)
    fi
    if [ -n "$CLAIMED_SPEC" ] && [ "$CLAIMED_SPEC" != "unassigned" ]; then
        STEM=$(echo "$CLAIMED_SPEC" | sed 's/^spec-//; s/\.md$//')
        PREV=$(_find_latest_state_for_spec "$STEM")
        if [ -n "$PREV" ]; then
            FOUND_STATE="$PREV"
        fi
    fi
fi

if [ -n "$FOUND_STATE" ]; then
    LAST_UPDATE=$(head -5 "$FOUND_STATE" | grep -o '20[0-9][0-9]-[0-9][0-9]-[0-9][0-9]' | head -1)
    PHASE=$(grep '^Phase:' "$FOUND_STATE" 2>/dev/null | head -1 | sed 's/^Phase:\s*//')
    if [ -n "$PHASE" ]; then
        echo "Session state: $FOUND_STATE (phase: $PHASE)"
    elif [ -n "$LAST_UPDATE" ]; then
        echo "Session state: $FOUND_STATE (updated: $LAST_UPDATE)"
    else
        echo "Session state: $FOUND_STATE"
    fi
fi

# Blocking reminders
echo "Warning: BLOCKING: ToolSearch select:LSP -- load BEFORE any work"
echo "Warning: RULE: Read spec + source files BEFORE writing any code"

# Suggest /ze-status when no spec is selected
if [ "$SELECTED_COUNT" -eq 0 ] && [ "$SPEC_COUNT" -gt 0 ]; then
    echo "Tip: /ze-status for a cross-project attention view"
fi
