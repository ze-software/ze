#!/bin/bash
# Resolves a stable, per-session identifier for hook marker files.
# Usage: source this file, then call _session_id
#
# Order of preference:
#   1. The Claude CLI's own --session-id, read from the process tree. This is the
#      canonical session id (the same UUID that appears in the session's job/tmp
#      paths), it is unique per session, and it stays constant across the many
#      short-lived hook subprocesses. It does NOT depend on any env var, and it
#      does NOT depend on the CLI process being named `claude` -- its argv[0] is
#      the version string (e.g. `.../versions/2.1.206`), which is why an older
#      name-based match failed and fell through to an unstable $PPID.
#   2. CLAUDE_CODE_SESSION_ACCESS_TOKEN (JWT session_id), when present.
#   3. A fixed constant, last resort only (shared across sessions, but stable --
#      $PPID is not, and an unstable id defeats marker matching entirely).

# _session_id_from_argv reads a null- or space-separated argv stream on stdin and
# prints the value following --session-id (both `--session-id X` and
# `--session-id=X` forms), then stops.
_session_id_from_argv() {
    awk '
        /^--session-id=/ { sub(/^--session-id=/, ""); print; exit }
        prev == "--session-id" { print; exit }
        { prev = $0 }'
}

_session_id() {
    local sid pid ppid

    # 1) --session-id from the process tree.
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
        [ -n "$sid" ] && { echo "$sid"; return; }
        [ -z "$ppid" ] && break
        pid=$ppid
    done

    # 2) JWT access token session_id.
    if [ -n "$CLAUDE_CODE_SESSION_ACCESS_TOKEN" ]; then
        local payload mod
        payload=$(echo "$CLAUDE_CODE_SESSION_ACCESS_TOKEN" | cut -d. -f2 | tr '_-' '/+')
        mod=$((${#payload} % 4))
        [ "$mod" -eq 2 ] && payload="${payload}=="
        [ "$mod" -eq 3 ] && payload="${payload}="
        sid=$(echo "$payload" | base64 -d 2>/dev/null | grep -o '"session_id": *"[^"]*"' | head -1 | cut -d'"' -f4)
        [ -n "$sid" ] && { echo "$sid"; return; }
    fi

    # 3) Last resort: a fixed constant. Not per-session, but stable -- unlike
    #    $PPID, which changes on every hook subprocess and breaks marker matching.
    echo "claude-session-fallback"
}
