#!/bin/bash
# Resolves a stable, per-session identifier for hook marker files.
# Usage: source this file, then call _session_id
#
# Order of preference:
#   1. $CLAUDE_CODE_SESSION_ID -- the CLI exports this session's UUID into the
#      environment of every process it spawns, so it reaches each short-lived hook
#      subprocess for free: no ps walk, no truncation risk, no parsing. Subagents
#      and forks inherit the PARENT session's value (verified 2026-07-16), which is
#      what we want: a subagent must see the markers its parent wrote (.lsp-invoked-*,
#      .source-read-*), or fail-closed gates would block work that was in fact done.
#   2. The Claude CLI's own --session-id, read from the process tree. Same canonical
#      UUID, but only present when the CLI was invoked with the flag -- an interactive
#      `claude` has no --session-id in argv at all, which is why strategies 1+2 both
#      missed before this env lookup existed and every session collapsed onto the
#      strategy-4 constant. Kept as a fallback for older CLIs that do not export the
#      env var. Does NOT depend on the CLI being named `claude` -- its argv[0] may be
#      the version string (e.g. `.../versions/2.1.206`).
#   3. CLAUDE_CODE_SESSION_ACCESS_TOKEN (JWT session_id), when present. Empty for
#      subscription auth, so it cannot be relied on.
#   4. A fixed constant, last resort only (shared across sessions, but stable --
#      $PPID is not, and an unstable id defeats marker matching entirely).
#
# An id from any source is used only when it is safe as a filename component
# ([A-Za-z0-9._-]); anything else falls through rather than being rewritten, so
# this function and its Python port in pretool-writeedit.py cannot disagree.

# _session_id_from_argv reads a null- or space-separated argv stream on stdin and
# prints the value following --session-id (both `--session-id X` and
# `--session-id=X` forms), then stops.
_session_id_from_argv() {
    awk '
        /^--session-id=/ { sub(/^--session-id=/, ""); print; exit }
        prev == "--session-id" { print; exit }
        { prev = $0 }'
}

# _sid_safe prints its argument when it is usable as a filename component,
# otherwise prints nothing. Mirrors _SID_SAFE_RE in pretool-writeedit.py.
_sid_safe() {
    case "$1" in
        "") return ;;
        *[!A-Za-z0-9._-]*) return ;;
        *) echo "$1" ;;
    esac
}

_session_id() {
    local sid pid ppid

    # 1) The session UUID the CLI exports into our environment.
    sid=$(_sid_safe "$CLAUDE_CODE_SESSION_ID")
    [ -n "$sid" ] && { echo "$sid"; return; }

    # 2) --session-id from the process tree.
    pid=$$
    while [ "$pid" -gt 1 ] 2>/dev/null; do
        if [ -r "/proc/$pid/cmdline" ]; then
            # Linux: /proc/<pid>/cmdline is NUL-separated.
            sid=$(tr '\0' '\n' < "/proc/$pid/cmdline" 2>/dev/null | _session_id_from_argv)
            ppid=$(awk '/^PPid:/ {print $2}' "/proc/$pid/status" 2>/dev/null)
        else
            # macOS / BSD: no /proc, use ps (space-separated argv, best effort).
            sid=$(ps -o command= -p "$pid" 2>/dev/null | tr ' ' '\n' | _session_id_from_argv)
            ppid=$(ps -o ppid= -p "$pid" 2>/dev/null | tr -d ' ')
        fi
        sid=$(_sid_safe "$sid")
        [ -n "$sid" ] && { echo "$sid"; return; }
        [ -z "$ppid" ] && break
        pid=$ppid
    done

    # 3) JWT access token session_id.
    if [ -n "$CLAUDE_CODE_SESSION_ACCESS_TOKEN" ]; then
        local payload mod
        payload=$(echo "$CLAUDE_CODE_SESSION_ACCESS_TOKEN" | cut -d. -f2 | tr '_-' '/+')
        mod=$((${#payload} % 4))
        [ "$mod" -eq 2 ] && payload="${payload}=="
        [ "$mod" -eq 3 ] && payload="${payload}="
        sid=$(echo "$payload" | base64 -d 2>/dev/null | grep -o '"session_id": *"[^"]*"' | head -1 | cut -d'"' -f4)
        sid=$(_sid_safe "$sid")
        [ -n "$sid" ] && { echo "$sid"; return; }
    fi

    # 4) Last resort: a fixed constant. Not per-session, but stable -- unlike
    #    $PPID, which changes on every hook subprocess and breaks marker matching.
    #    Reaching this now means the CLI exported no session id AND passed no
    #    --session-id AND issued no access token: concurrent sessions collapse onto
    #    one marker set again, which is the bug strategy 1 exists to prevent.
    echo "claude-session-fallback"
}
