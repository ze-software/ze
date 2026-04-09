#!/bin/bash
# Extracts a stable session identifier from CLAUDE_CODE_SESSION_ACCESS_TOKEN (JWT).
# Falls back to PPID if token unavailable.
# Usage: source this file, then call _session_id

_session_id() {
    if [ -z "$CLAUDE_CODE_SESSION_ACCESS_TOKEN" ]; then
        # Walk up the process tree to find the Claude CLI process.
        # Its PID is stable for the entire session, unlike $PPID which
        # varies across hook subprocesses.
        local pid=$$
        while [ "$pid" -gt 1 ] 2>/dev/null; do
            # Check argv[0] from /proc/pid/cmdline (first NUL-delimited field)
            local argv0
            argv0=$(tr '\0' '\n' < "/proc/$pid/cmdline" 2>/dev/null | head -1)
            if [ "$argv0" = "claude" ]; then
                echo "$pid"
                return
            fi
            pid=$(awk '/^PPid:/ {print $2}' "/proc/$pid/status" 2>/dev/null)
            [ -z "$pid" ] && break
        done
        echo "$PPID"
        return
    fi
    # JWT payload is the second dot-separated segment (URL-safe base64)
    local payload
    payload=$(echo "$CLAUDE_CODE_SESSION_ACCESS_TOKEN" | cut -d. -f2)
    # Convert URL-safe base64 to standard base64 and add padding
    payload=$(echo "$payload" | tr '_-' '/+')
    local mod=$((${#payload} % 4))
    [ "$mod" -eq 2 ] && payload="${payload}=="
    [ "$mod" -eq 3 ] && payload="${payload}="
    # Extract session_id field
    local sid
    sid=$(echo "$payload" | base64 -d 2>/dev/null | grep -o '"session_id": *"[^"]*"' | head -1 | cut -d'"' -f4)
    if [ -n "$sid" ]; then
        echo "$sid"
    else
        echo "$PPID"
    fi
}
