#!/bin/bash
# SessionStart hook - compact status summary with rule reminders
# Reads this session's spec from its own marker (set via scripts/dev/spec-session.sh).

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

# Load helpers
source .claude/hooks/lib/state-file.sh

# Clean up stale markers from dead sessions
_cleanup_stale_markers

# Clean up old tmp/ scratch files (>24h) silently
find tmp/ -maxdepth 1 -type f -mmin +1440 -delete 2>/dev/null || true
find tmp/session/ -maxdepth 1 -type f -mmin +1440 -delete 2>/dev/null || true

# Backstop: reap per-session scratch dirs (tmp/s/<sid>/) from sessions that
# ended without firing SessionEnd (crash/kill). SessionEnd is the primary
# cleanup; --reap removes only dirs with no file activity in the last 24h, so a
# live long-running session is never reaped. See scripts/dev/session-scratch.sh.
scripts/dev/session-scratch.sh --reap 2>/dev/null || true

# --- Read this session's claimed spec (set via scripts/dev/spec-session.sh) ---
# Each session records its spec in its OWN marker; there is no shared file, so
# many agents editing main concurrently never collide. The marker is written
# when work begins (skills call `spec-session.sh claim`), not at session start,
# so a brand-new session has no spec until it claims one.
mkdir -p tmp/session
SID_CHECK=$(_session_id)
MARKER_CHECK="tmp/session/.session-${SID_CHECK}"
CLAIMED_SPEC=""
if [ -f "$MARKER_CHECK" ]; then
    CLAIMED_SPEC=$(head -1 "$MARKER_CHECK" 2>/dev/null)
fi
[ "$CLAIMED_SPEC" = "unassigned" ] && CLAIMED_SPEC=""

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

if [ -n "$CLAIMED_SPEC" ] && [ -f "plan/$CLAIMED_SPEC" ]; then
    echo "SPEC: $CLAIMED_SPEC (+$((SPEC_COUNT-1)) others)"
    echo "   -> READ plan/$CLAIMED_SPEC BEFORE any work"
elif [ "$SPEC_COUNT" -gt 0 ]; then
    echo "${SPEC_COUNT} specs, none claimed by this session"
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
elif [ -n "$CLAIMED_SPEC" ]; then
    # Look for a previous session's state for the same spec
    STEM=$(echo "$CLAIMED_SPEC" | sed 's/^spec-//; s/\.md$//')
    PREV=$(_find_latest_state_for_spec "$STEM")
    if [ -n "$PREV" ]; then
        FOUND_STATE="$PREV"
    fi
fi

if [ -n "$FOUND_STATE" ]; then
    LAST_UPDATE=$(head -5 "$FOUND_STATE" | grep -o '20[0-9][0-9]-[0-9][0-9]-[0-9][0-9]' | head -1)
    PHASE=$(grep '^Phase:' "$FOUND_STATE" 2>/dev/null | head -1 | sed 's/^Phase:[[:space:]]*//')
    if [ -n "$PHASE" ]; then
        echo "Session state: $FOUND_STATE (phase: $PHASE)"
    elif [ -n "$LAST_UPDATE" ]; then
        echo "Session state: $FOUND_STATE (updated: $LAST_UPDATE)"
    else
        echo "Session state: $FOUND_STATE"
    fi
fi

# Generated agent files drift check. CLAUDE.md / AGENTS.md / skills mirrors
# are gitignored, so git never shows drift; compare content instead.
if ! scripts/dev/skill_sync.sh --check >/dev/null 2>&1; then
    echo "Warning: generated agent files are stale (CLAUDE.md / AGENTS.md / skills mirrors)"
    echo "   -> run: make ze-regen"
fi

# Blocking reminders
echo "Warning: BLOCKING (no task-type exception): ToolSearch query=\"select:LSP\" MUST be your FIRST tool call."
echo "Warning:   Do NOT skip because the task looks shell-only, docs-only, or trivial."
echo "Warning:   See .claude/rules/session-start.md 'LSP Load (step 1) -- no-exceptions clause'."
echo "Warning: RULE: Read spec + source files BEFORE writing any code"
echo "Rules: ai/rules/INDEX.md is a one-line overview of every rule -- scan it, read the listed file in full before acting on a topic it covers"

# Suggest /ze-status when this session has no spec claimed
if [ -z "$CLAIMED_SPEC" ] && [ "$SPEC_COUNT" -gt 0 ]; then
    echo "Tip: /ze-status for a cross-project attention view"
fi
