#!/bin/bash
# Resolves the per-session state file path.
# Usage: source this file (after session-id.sh), then call _state_file
#
# Reads the session marker (tmp/session/.session-<ID>) to find this session's spec.
# Returns <session-dir>/state/session-state-<spec-stem>-<SID>.md, where
# <session-dir> is tmp/session/<YYYY-MM-DD>-<SID> resolved by the one shell rule
# in lib/session-dir.sh. The digest lives in the directory of the session that
# wrote it, beside that session's bin/ and scratch/.
# Multiple sessions on different specs each get their own state file.
# Sessions on the same spec also get separate files (avoids write races).
#
# Cross-session reading costs nothing: _find_latest_state_for_spec below walks
# every session directory's state/ (plan/spec-session-bin-directory.md, AC-20
# and AC-21).

# Ensure session-id helper is loaded
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if ! type _session_id &>/dev/null; then
    # shellcheck source=session-id.sh
    source "$SCRIPT_DIR/session-id.sh"
fi
if ! type _session_dir &>/dev/null; then
    # shellcheck source=session-dir.sh
    source "$SCRIPT_DIR/session-dir.sh"
fi

_state_file() {
    local sid spec stem marker dir
    sid=$(_session_id)
    mkdir -p tmp/session
    marker="tmp/session/.session-${sid}"
    if [ -f "$marker" ]; then
        spec=$(head -1 "$marker" 2>/dev/null)
    fi
    # Every caller either writes this file or tests it for existence, so the
    # directory is created here rather than in each of them.
    dir="$(_session_dir "$sid")/state"
    mkdir -p "$dir"
    if [ -n "$spec" ] && [ "$spec" != "unassigned" ]; then
        # Strip spec- prefix and .md suffix to form the stem
        stem=$(echo "$spec" | sed 's/^spec-//; s/\.md$//')
        echo "$dir/session-state-${stem}-${sid}.md"
    else
        echo "$dir/session-state-${sid}.md"
    fi
}

# Find the most recent session state file for a given spec stem.
# Used at session start, and at agent spawn, to recover state a previous session
# or an earlier phase wrote.
#
# FOUR locations are read, newest first across the first two:
#   1. <session-dir>/state/ of EVERY session -- where the digest lands today.
#   2. tmp/session/ flat -- where every digest written before the state/ move
#      still sits. Dropping this branch makes those unreachable, and a resolver
#      that returns nothing looks exactly like a spec with no prior phase.
#   3. .claude/session-state-<stem>-<SID>.md -- the location before tmp/session/.
#   4. .claude/session-state-<stem>.md -- the per-spec form before the SID.
# Nothing is migrated: a digest is read where its writer left it.
_find_latest_state_for_spec() {
    local stem="$1"
    mkdir -p tmp/session
    local latest
    # An unmatched glob reaches ls as its literal self; ls reports it on stderr
    # and still lists the patterns that did match, so one call orders both
    # locations by mtime.
    latest=$(ls -t tmp/session/????-??-??-*/state/session-state-${stem}-*.md \
                   tmp/session/session-state-${stem}-*.md 2>/dev/null | head -1)
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

# Remove this session's marker. Called ONLY from `spec-session.sh release`, the
# operator-invoked step /ze-close runs when a spec closes. No hook releases a
# claim, and nothing removes a marker on a timer: a claim whose session is gone
# ages out of relevance in place, and block-premature-stop.sh refreshes a live
# one with `touch -c` so its mtime says which is which.
_release_session() {
    local sid marker
    sid=$(_session_id)
    marker="tmp/session/.session-${sid}"
    rm -f "$marker"
}
