#!/bin/bash
# Resolves the per-spec session state file path.
# Usage: source this file (after session-id.sh), then call _state_file
#
# Reads the session marker (.claude/.session-<ID>) to find this session's spec.
# Returns .claude/session-state-<spec-stem>.md when a spec is active,
# or .claude/session-state.md as fallback.

# Ensure session-id helper is loaded
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if ! type _session_id &>/dev/null; then
    # shellcheck source=session-id.sh
    source "$SCRIPT_DIR/session-id.sh"
fi

_state_file() {
    local sid spec stem marker
    sid=$(_session_id)
    marker=".claude/.session-${sid}"
    if [ -f "$marker" ]; then
        spec=$(head -1 "$marker" 2>/dev/null)
    fi
    if [ -n "$spec" ] && [ "$spec" != "unassigned" ]; then
        # Strip spec- prefix and .md suffix to form the stem
        stem=$(echo "$spec" | sed 's/^spec-//; s/\.md$//')
        echo ".claude/session-state-${stem}.md"
    else
        echo ".claude/session-state.md"
    fi
}

# Write this session's spec to its marker file.
# Called by session-start to claim a spec.
_claim_spec() {
    local spec="$1"
    local sid marker
    sid=$(_session_id)
    marker=".claude/.session-${sid}"
    echo "$spec" > "$marker"
}

# Remove this session's marker (called at session end).
_release_session() {
    local sid marker
    sid=$(_session_id)
    marker=".claude/.session-${sid}"
    rm -f "$marker"
}

# Clean up stale markers (sessions that ended without cleanup).
# Removes markers older than 24 hours.
_cleanup_stale_markers() {
    find .claude/ -maxdepth 1 -name '.session-*' -mmin +1440 -delete 2>/dev/null
}
