#!/usr/bin/env bash
# verify-lock.sh -- acquire the global ze-verify lock, then exec the given command.
#
# Blocks if another verify variant (ze-verify, ze-verify-fast, ze-verify-changed,
# ze-chaos-verify) is already running. Ensures only ONE verify-class run at a time
# across concurrent Claude sessions and human invocations.
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

# Open fd on the lock file. Try non-blocking first so we can print a waiting
# message; fall back to blocking flock if another verify holds it.
exec {LOCKFD}>"$LOCKFILE"
if ! flock -n "$LOCKFD"; then
	printf '\033[33m[%s] another verify run in progress, waiting for lock...\033[0m\n' "$LABEL"
	flock "$LOCKFD"
	printf '[%s] lock acquired.\n' "$LABEL"
fi

# Record our PID inside the lock file for diagnostics (best-effort).
printf '%s\n' "$$" >&"$LOCKFD" 2>/dev/null || true

exec "$@"
