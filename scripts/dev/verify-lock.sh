#!/usr/bin/env bash
# verify-lock.sh -- run a verify-class command under the shared job admission
# point, so only one verify-class run happens at a time across concurrent
# Claude sessions and human invocations.
#
# This is now an ALIAS. The mechanism it used to hold -- an flock on
# tmp/.ze-verify.lock, an owner file, and a duration history -- was generalised
# into scripts/dev/ze-run.sh, which admits EVERY heavy job rather than the
# three verify targets, and records each one in a registry under
# tmp/.ze-jobs/. The name stays because its callers do: Makefile
# (ze-precommit-verify, ze-precommit-verify-changed), mk/test-chaos.mk
# (ze-chaos-verify), docs/functional-tests.md and ai/rules/git-safety.md all
# reference this path and this interface.
#
# Behaviour a caller can still rely on, unchanged:
#   - the LABEL CMD [ARGS...] interface, and the command's exit code
#   - tmp/.ze-verify.lock.owner, naming the current holder while it runs
#   - tmp/.ze-verify-duration.txt, one appended row per completed run
#   - the "previous run took" line printed on acquisition
#
# Usage: scripts/dev/verify-lock.sh LABEL CMD [ARGS...]

exec "$(dirname "$0")/ze-run.sh" "$@"
