#!/usr/bin/env bash
# verify-lock.sh -- acquire the global ze-verify lock, then exec the given command.
#
# Blocks if another verify variant (ze-verify, ze-verify-fast, ze-verify-changed,
# ze-chaos-verify) is already running. Ensures only ONE verify-class run at a time
# across concurrent Claude sessions and human invocations.
#
# The `flock -o` (close-on-exec) is load-bearing: flock itself stays alive as the
# parent process and holds the lock fd. The command it exec-s -- make, go test,
# bin/ze, external plugin subprocesses like test/plugin/lg-graph-lab/lg-lab.run --
# never inherits the fd. Without -o, orphaned plugin subprocesses (parent dies,
# they get reparented to init) keep the fd open long after their parent test
# finishes, blocking every subsequent verify run until an operator kills the
# orphan by hand. flock releases the lock automatically when flock itself exits
# (i.e. when the wrapped command returns).
#
# Usage: scripts/dev/verify-lock.sh LABEL CMD [ARGS...]

set -e

LABEL="$1"
shift

if [ -z "$LABEL" ] || [ "$#" -eq 0 ]; then
	echo "usage: $0 LABEL CMD [ARGS...]" >&2
	exit 2
fi

if ! command -v flock >/dev/null 2>&1; then
	echo "error: flock required (Linux util-linux package)" >&2
	exit 1
fi

mkdir -p tmp
LOCKFILE="tmp/.ze-verify.lock"

# Print the "waiting" banner only when the lock is actually held right now.
# The probe briefly takes the lock via `true`; if -n returns non-zero the
# lock is held and the real acquire below will block.
if ! flock -n -o "$LOCKFILE" true 2>/dev/null; then
	printf '\033[33m[%s] another verify run in progress, waiting for lock...\033[0m\n' "$LABEL"
fi

exec flock -o "$LOCKFILE" "$@"
