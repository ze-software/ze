#!/bin/bash
# SessionStart hook - compact status summary with rule reminders
# Reads this session's spec from its own marker (set via scripts/dev/spec-session.sh).

cd "$CLAUDE_PROJECT_DIR" 2>/dev/null || cd "$(dirname "$0")/../.."

# Load helpers
source .claude/hooks/lib/state-file.sh

# SessionStart supplies the canonical session id in its JSON payload. Validate the
# decoded JSON value before shell command substitution can remove trailing newlines.
SAFE_PAYLOAD_SID=$(python3 .claude/hooks/lib/session_id.py --hook-session-id 2>/dev/null) \
    || SAFE_PAYLOAD_SID=""
if [ -n "$SAFE_PAYLOAD_SID" ]; then
    export CLAUDE_CODE_SESSION_ID="$SAFE_PAYLOAD_SID"
    if [ -n "${CLAUDE_ENV_FILE:-}" ]; then
        printf 'export CLAUDE_CODE_SESSION_ID=%s\n' "$SAFE_PAYLOAD_SID" \
            >> "$CLAUDE_ENV_FILE" 2>/dev/null || true
    fi
fi

# This hook DELETES NOTHING. It once swept tmp/ and tmp/session/ on an age
# timer, and reaped the dated session directories of sessions that never fired
# SessionEnd. Every one of those removed the operator's own files without being
# asked, and one of them deleted the tracked tmp/go.mod sentinel on every start.
# Cleanup is now operator-invoked only: `make ze-scratch-clean` for the tmp/ root,
# `make ze-session-clean BEFORE=<YYYY-MM-DD>` for the dated session directories
# (owner decision, 2026-08-03).

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
    for status in in-progress verification ready design skeleton blocked deferred; do
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

# Verification debt: gates a past commit owed and nobody has run yet. This is
# the "check it next session" half of the ledger -- the session that pays the
# debt is rarely the one that owed it, and `--push` refuses while a row is open,
# so the first session to want a push must see this before it gets there.
if [ -d plan/verification-debt ]; then
    DEBT_OPEN=$(awk -F'|' 'NF == 8 && tolower($7) ~ /^ *open *$/ { n++ } END { print n+0 }' \
        plan/verification-debt/*.md 2>/dev/null)
    if [ -n "$DEBT_OPEN" ] && [ "$DEBT_OPEN" -gt 0 ]; then
        echo "Warning: verification debt: $DEBT_OPEN gate(s) owed, --push is refused until cleared"
        awk -F'|' 'NF == 8 && tolower($7) ~ /^ *open *$/ {
            gsub(/^[ \t]+|[ \t]+$/, "", $5); gsub(/^[ \t]+|[ \t]+$/, "", $4)
            printf "   - %s  (%s)\n", $5, $4
        }' plan/verification-debt/*.md 2>/dev/null | head -5
    fi
fi

# Derived documentation indexes. Both are gitignored (.gitignore), so a fresh
# clone has neither, and a rule that tells an agent to read one would point at
# nothing. Build only what is MISSING: in steady state this costs nothing, and
# a full rebuild of both takes under 3 seconds. Staleness is not chased here --
# `make ze-generated-files-update` owns that, and a derived file nothing gates
# on is cheaper to rebuild than to police.
for idx in ai/DOCS-TO-CODE.md:ze-discovery-index-update ai/CODE-TO-DOCS.md:ze-doc-index-update; do
    if [ ! -f "${idx%%:*}" ]; then
        make "${idx##*:}" >/dev/null 2>&1 && echo "Built ${idx%%:*} (derived, not tracked)"
    fi
done

# Generated agent files drift check. CLAUDE.md / AGENTS.md / skills mirrors
# are gitignored, so git never shows drift; compare content instead.
if ! scripts/dev/skill_sync.sh --check >/dev/null 2>&1; then
    echo "Warning: generated agent files are stale (CLAUDE.md / AGENTS.md / skills mirrors)"
    echo "   -> run: make ze-generated-files-update"
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
