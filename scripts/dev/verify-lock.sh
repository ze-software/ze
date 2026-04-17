#!/usr/bin/env bash
# verify-lock.sh -- acquire the global ze-verify lock, write owner info,
# then run the given command.
#
# Blocks if another verify variant (ze-verify, ze-verify-fast, ze-verify-changed,
# ze-chaos-verify) is already running. Ensures only ONE verify-class run at a
# time across concurrent Claude sessions and human invocations.
#
# The `flock -o` (close-on-exec) is load-bearing: flock stays alive as the
# parent and holds the lock fd. The command it exec-s -- and any grandchild
# subprocesses -- never inherits the fd, so orphaned plugin subprocesses
# cannot wedge the lock.
#
# On acquisition we re-enter ourselves in __inner__ mode to write
# tmp/.ze-verify.lock.owner (pid, label, started, cmd) and clear it on exit.
# Readers of that file (including the "waiting" banner below) see who
# holds the lock and for how long.
#
# Usage: scripts/dev/verify-lock.sh LABEL CMD [ARGS...]

set -e

LOCKFILE="tmp/.ze-verify.lock"
OWNER_FILE="tmp/.ze-verify.lock.owner"

# ---- Inner mode: invoked by flock once lock is acquired ----
if [ "${1:-}" = "__inner__" ]; then
    shift
    LABEL="$1"; shift
    mkdir -p tmp
    {
        printf 'LABEL=%s\n' "$LABEL"
        printf 'PID=%s\n' "$$"
        printf 'STARTED=%s\n' "$(date +%s)"
        printf 'CMD=%s\n' "$*"
    } > "$OWNER_FILE"
    trap 'rm -f "$OWNER_FILE"' EXIT INT TERM
    "$@"
    exit $?
fi

# ---- Outer mode ----
LABEL="$1"
shift || true

if [ -z "${LABEL:-}" ] || [ "$#" -eq 0 ]; then
    echo "usage: $0 LABEL CMD [ARGS...]" >&2
    exit 2
fi

if ! command -v flock >/dev/null 2>&1; then
    echo "error: flock required (Linux util-linux package)" >&2
    exit 1
fi

mkdir -p tmp

# Print a detailed "waiting" banner only when the lock is actually held now.
if ! flock -n -o "$LOCKFILE" true 2>/dev/null; then
    if [ -f "$OWNER_FILE" ]; then
        # shellcheck disable=SC1090
        held_label=$(awk -F= '/^LABEL=/{sub(/^LABEL=/,""); print; exit}' "$OWNER_FILE" 2>/dev/null || echo unknown)
        held_pid=$(awk   -F= '/^PID=/{print $2; exit}'                   "$OWNER_FILE" 2>/dev/null || echo 0)
        started=$(awk    -F= '/^STARTED=/{print $2; exit}'               "$OWNER_FILE" 2>/dev/null || echo 0)
        now=$(date +%s)
        elapsed=$(( now - started ))
        if [ "$held_pid" != "0" ] && kill -0 "$held_pid" 2>/dev/null; then
            printf '\033[33m[%s] waiting: %s running (pid %s, %ds elapsed)...\033[0m\n' \
                "$LABEL" "$held_label" "$held_pid" "$elapsed"
        else
            printf '\033[33m[%s] waiting (owner pid %s not alive; lock should clear shortly)...\033[0m\n' \
                "$LABEL" "$held_pid"
        fi
    else
        printf '\033[33m[%s] waiting for lock (no owner file)...\033[0m\n' "$LABEL"
    fi
fi

# Acquire lock and re-enter in __inner__ mode so owner-file bookkeeping runs
# *after* the lock is held.
exec flock -o "$LOCKFILE" "$0" __inner__ "$LABEL" "$@"
