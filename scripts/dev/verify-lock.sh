#!/usr/bin/env bash
# verify-lock.sh -- acquire the global ze-verify lock, write owner info,
# then run the given command.
#
# Blocks if another verify variant (ze-verify, ze-verify-changed,
# ze-chaos-verify) is already running. Ensures only ONE verify-class run at a
# time across concurrent Claude sessions and human invocations.
#
# The `flock -o` (close-on-exec) is load-bearing: flock stays alive as the
# parent and holds the lock fd. The command it exec-s -- and any grandchild
# subprocesses -- never inherits the fd, so orphaned plugin subprocesses
# cannot wedge the lock.
#
# Stuck-run recovery: if the current holder has been running longer than
# MAX_LOCK_AGE seconds (env: ZE_VERIFY_MAX_LOCK_AGE, default 1800), the
# waiting invocation SIGTERMs then SIGKILLs the holder's entire process
# group and acquires the lock. ze-verify targets ~2 min (common case,
# two-pass strategy), so 30 min is well past "something
# is wrong" -- without this, a single hung test (e.g. ze-test bgp plugin
# --all) wedges every concurrent session forever.
#
# On acquisition we re-enter ourselves in __inner__ mode to write
# tmp/.ze-verify.lock.owner (pid, pgid, label, started, cmd) and clear it
# on exit. Readers of that file (including the "waiting" banner below)
# see who holds the lock and for how long.
#
# Usage: scripts/dev/verify-lock.sh LABEL CMD [ARGS...]

set -e

LOCKFILE="tmp/.ze-verify.lock"
OWNER_FILE="tmp/.ze-verify.lock.owner"
MAX_LOCK_AGE="${ZE_VERIFY_MAX_LOCK_AGE:-1800}"

# ---- Inner mode: invoked by flock once lock is acquired ----
if [ "${1:-}" = "__inner__" ]; then
    shift
    LABEL="$1"; shift
    mkdir -p tmp
    pgid=$(ps -o pgid= -p $$ | tr -d ' ')
    {
        printf 'LABEL=%s\n' "$LABEL"
        printf 'PID=%s\n' "$$"
        printf 'PGID=%s\n' "$pgid"
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

# If the lock is held, decide: wait, or break-and-take (stuck holder).
if ! flock -n -o "$LOCKFILE" true 2>/dev/null; then
    held_label="unknown"; held_pid=0; held_pgid=0; started=0
    if [ -f "$OWNER_FILE" ]; then
        held_label=$(awk -F= '/^LABEL=/{sub(/^LABEL=/,""); print; exit}' "$OWNER_FILE" 2>/dev/null || echo unknown)
        held_pid=$(awk   -F= '/^PID=/{print $2; exit}'                   "$OWNER_FILE" 2>/dev/null || echo 0)
        held_pgid=$(awk  -F= '/^PGID=/{print $2; exit}'                  "$OWNER_FILE" 2>/dev/null || echo 0)
        started=$(awk    -F= '/^STARTED=/{print $2; exit}'               "$OWNER_FILE" 2>/dev/null || echo 0)
    fi
    now=$(date +%s)
    elapsed=$(( now - started ))

    owner_alive=0
    if [ "$held_pid" != "0" ] && kill -0 "$held_pid" 2>/dev/null; then
        owner_alive=1
    fi

    if [ "$owner_alive" = "1" ] && [ "$started" -gt 0 ] && [ "$elapsed" -gt "$MAX_LOCK_AGE" ]; then
        # Stuck run: break the lock by killing the holder's process group.
        # Fall back to PID if PGID was not recorded (older owner file).
        target=""
        if [ "$held_pgid" != "0" ] && [ -n "$held_pgid" ]; then
            target="-$held_pgid"
        elif [ "$held_pid" != "0" ]; then
            target="$held_pid"
        fi
        if [ -n "$target" ]; then
            printf '\033[31m[%s] breaking stuck lock: %s (pid %s, pgid %s, %ds elapsed > %ds max)\033[0m\n' \
                "$LABEL" "$held_label" "$held_pid" "$held_pgid" "$elapsed" "$MAX_LOCK_AGE" >&2
            kill -TERM -- "$target" 2>/dev/null || true
            # Give the holder a chance to release; 3s is plenty for flock to drop the fd.
            for _ in 1 2 3; do
                if ! kill -0 "$held_pid" 2>/dev/null; then break; fi
                sleep 1
            done
            kill -KILL -- "$target" 2>/dev/null || true
            sleep 1
            rm -f "$OWNER_FILE"
        fi
    elif [ "$owner_alive" = "1" ]; then
        printf '\033[33m[%s] waiting: %s running (pid %s, %ds elapsed)...\033[0m\n' \
            "$LABEL" "$held_label" "$held_pid" "$elapsed" >&2
    elif [ "$held_pid" != "0" ]; then
        printf '\033[33m[%s] waiting (owner pid %s not alive; lock should clear shortly)...\033[0m\n' \
            "$LABEL" "$held_pid" >&2
    else
        printf '\033[33m[%s] waiting for lock (no owner file)...\033[0m\n' "$LABEL" >&2
    fi
fi

# Acquire lock and re-enter in __inner__ mode so owner-file bookkeeping runs
# *after* the lock is held.
exec flock -o "$LOCKFILE" "$0" __inner__ "$LABEL" "$@"
