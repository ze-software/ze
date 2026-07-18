#!/bin/bash
# Print (creating it) this session's private scratch directory under tmp/.
#
# Concurrent Claude sessions share ONE working tree, hence one tmp/ (keyed
# per-checkout, not per-session -- scripts/dev/ensure-links.py). Ad-hoc scratch
# written with fixed names at the tmp/ root (tmp/out.log, tmp/stdout, ...)
# therefore collides with a sibling session's identical name and is never
# cleaned when a session ends.
#
# This helper hands each session its OWN subdir, tmp/s/<session-id>/, so:
#   * two sessions never clobber each other's scratch, and
#   * the whole session's scratch is one `rm -rf` at session end
#     (.claude/hooks/session-end-scratch.sh); `--reap` (from session-start.sh)
#     is the backstop for sessions that crash before SessionEnd fires.
#
# <session-id> is the canonical id the hooks resolve (.claude/hooks/lib/session-id.sh),
# so this helper and the cleanup hook agree on the path whenever the CLI exports
# $CLAUDE_CODE_SESSION_ID (the normal case; the --reap backstop covers the rest).
#
# Usage (run from the checkout root; the printed path is root-relative):
#   dir=$(scripts/dev/session-scratch.sh)          # tmp/s/<id>/, created for you
#   make ze-unit-test-changed > "$dir/unit.log" 2>&1
#   scripts/dev/session-scratch.sh --path          # print the path WITHOUT creating it
#   scripts/dev/session-scratch.sh --reap          # remove dead sessions' dirs (backstop)
#   scripts/dev/session-scratch.sh --clean         # remove THIS session's dir (`make clean`)
#
# No `set -u`: session-id.sh reads $CLAUDE_CODE_SESSION_ID and
# $CLAUDE_CODE_SESSION_ACCESS_TOKEN without defaults (matches spec-session.sh).

# Resolve the id helper relative to THIS script (the real checkout), so it is
# found no matter the caller's cwd -- that is what lets the tests run isolated.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../../.claude/hooks/lib/session-id.sh
source "$SCRIPT_DIR/../../.claude/hooks/lib/session-id.sh"

# Operate at the checkout root. Prefer $CLAUDE_PROJECT_DIR so this agrees with
# the SessionEnd cleanup hook (which uses it); otherwise the git toplevel of the
# caller's cwd (a test's throwaway repo resolves to itself here).
root="${CLAUDE_PROJECT_DIR:-}"
if [ -z "$root" ]; then
    root=$(git rev-parse --show-toplevel 2>/dev/null) || root="$(cd "$SCRIPT_DIR/../.." && pwd)"
fi
cd "$root" || exit 1

# reap_dead removes per-session scratch dirs left by sessions that ended without
# firing SessionEnd (crash/kill). It reaps a dir only when NOTHING inside was
# modified in the last 24h -- keyed on file activity (find -mmin), never the
# dir's own mtime, which an append/overwrite into an existing file does not
# update. So a session still writing is never reaped mid-run; a session idle for
# 24h is treated as dead and self-heals (mkdir -p recreates the dir on next use).
reap_dead() {
    [ -d tmp/s ] || return 0
    local d
    for d in tmp/s/*/; do
        [ -d "$d" ] || continue
        if [ -z "$(find "$d" -mmin -1440 -print -quit 2>/dev/null)" ]; then
            rm -rf "$d"
        fi
    done
}

if [ "${1:-}" = "--reap" ]; then
    reap_dead
    exit 0
fi

sid=$(_session_id)
# Refuse an id that is empty, path-bearing, or a dot entry, so we can never
# escape tmp/s/ (mirrors session-end-scratch.sh). _sid_safe already drops '/',
# globs and whitespace, but it permits '.' and '..'.
case "$sid" in
    "" | */* | . | ..)
        echo "session-scratch: unsafe session id '${sid}'" >&2
        exit 1
        ;;
esac

dir="tmp/s/${sid}"

case "${1:-}" in
    --clean) rm -rf "$dir"; exit 0 ;; # remove THIS session's scratch, print nothing
    --path) ;;                        # print only, do not create
    "") mkdir -p "$dir" || { echo "session-scratch: cannot create $dir" >&2; exit 1; } ;;
    *) echo "usage: session-scratch.sh [--path|--reap|--clean]" >&2; exit 2 ;;
esac

printf '%s\n' "$dir"
