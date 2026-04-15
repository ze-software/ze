#!/bin/bash
# Resolves the per-session state file path.
# Usage: source this file (after session-id.sh), then call _state_file
#
# Reads the session marker (tmp/session/.session-<ID>) to find this session's spec.
# Returns tmp/session/session-state-<spec-stem>-<SID>.md (session-scoped).
# Multiple sessions on different specs each get their own state file.
# Sessions on the same spec also get separate files (avoids write races).

# Ensure session-id helper is loaded
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if ! type _session_id &>/dev/null; then
    # shellcheck source=session-id.sh
    source "$SCRIPT_DIR/session-id.sh"
fi

_state_file() {
    local sid spec stem marker
    sid=$(_session_id)
    mkdir -p tmp/session
    marker="tmp/session/.session-${sid}"
    if [ -f "$marker" ]; then
        spec=$(head -1 "$marker" 2>/dev/null)
    fi
    if [ -n "$spec" ] && [ "$spec" != "unassigned" ]; then
        # Strip spec- prefix and .md suffix to form the stem
        stem=$(echo "$spec" | sed 's/^spec-//; s/\.md$//')
        echo "tmp/session/session-state-${stem}-${sid}.md"
    else
        echo "tmp/session/session-state-${sid}.md"
    fi
}

# Find the most recent session state file for a given spec stem.
# Used at session start to recover state from a previous session.
# Checks new per-session format first, then falls back to old per-spec format.
_find_latest_state_for_spec() {
    local stem="$1"
    mkdir -p tmp/session
    # New format: session-state-<stem>-<SID>.md (per-session)
    local latest
    latest=$(ls -t tmp/session/session-state-${stem}-*.md 2>/dev/null | head -1)
    if [ -n "$latest" ]; then
        echo "$latest"
        return
    fi
    # Legacy: check old .claude/ location
    latest=$(ls -t .claude/session-state-${stem}-*.md 2>/dev/null | head -1)
    if [ -n "$latest" ]; then
        echo "$latest"
        return
    fi
    # Old format: session-state-<stem>.md (per-spec, no SID)
    if [ -f ".claude/session-state-${stem}.md" ]; then
        echo ".claude/session-state-${stem}.md"
    fi
}

# Write this session's spec to its marker file.
# Called by session-start to claim a spec.
_claim_spec() {
    local spec="$1"
    local sid marker
    sid=$(_session_id)
    mkdir -p tmp/session
    marker="tmp/session/.session-${sid}"
    echo "$spec" > "$marker"
}

# Remove this session's marker (called at session end).
_release_session() {
    local sid marker
    sid=$(_session_id)
    marker="tmp/session/.session-${sid}"
    rm -f "$marker"
}

# Clean up stale markers and state files (sessions that ended without cleanup).
# Removes markers and per-session state files older than 24 hours.
_cleanup_stale_markers() {
    mkdir -p tmp/session
    find tmp/session/ -maxdepth 1 -name '.session-*' -mmin +1440 -delete 2>/dev/null
    find tmp/session/ -maxdepth 1 -name '.compaction-detected-*' -mmin +1440 -delete 2>/dev/null
    # Also clean legacy .claude/ location
    find .claude/ -maxdepth 1 -name '.session-*' -mmin +1440 -delete 2>/dev/null
    find .claude/ -maxdepth 1 -name '.compaction-detected-*' -mmin +1440 -delete 2>/dev/null
    # Clean up orphaned session state files (no matching session marker)
    for state in tmp/session/session-state-*-*.md; do
        [ -f "$state" ] || continue
        # Extract SID from filename: session-state-<stem>-<SID>.md or session-state-<SID>.md
        local fname sid marker
        fname=$(basename "$state" .md)
        sid="${fname##*-}"
        marker="tmp/session/.session-${sid}"
        # If marker doesn't exist and state file is older than 24h, remove
        if [ ! -f "$marker" ]; then
            find "$state" -mmin +1440 -delete 2>/dev/null
        fi
    done
}
