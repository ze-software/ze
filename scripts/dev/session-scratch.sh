#!/bin/bash
# Print (creating it) this session's private scratch directory under tmp/.
#
# Concurrent Claude sessions share ONE working tree, hence one tmp/ (keyed
# per-checkout, not per-session -- scripts/dev/ensure-links.py). Ad-hoc scratch
# written with fixed names at the tmp/ root (tmp/out.log, tmp/stdout, ...)
# therefore collides with a sibling session's identical name and is never
# cleaned when a session ends.
#
# Every session owns one directory, tmp/session/<YYYY-MM-DD>-<session-id>/, and
# everything that session writes lands in a subdirectory of it:
#
#   bin/      this session's binaries, and the etc/ze they resolve (mk/session.mk,
#             internal/test/sessionpath)
#   scratch/  ad-hoc logs, probes, captures -- what this helper prints
#   state/    the per-spec digest session-state-<stem>-<sid>.md
#             (.claude/hooks/lib/state-file.sh)
#
# So two sessions never clobber each other's scratch, and an operator reads the
# age of a session off its name and removes it by date, with
# `make ze-clean-sessions BEFORE=<YYYY-MM-DD>`.
#
# NOTHING under tmp/session/ is ever removed automatically (owner decision,
# 2026-08-03): not at session end, not on an age timer, not by a hook. The price
# is growth; the price of the alternative was deleting the operator's data
# unasked. --clean below is the one removal this script performs, and only when
# a person or `make clean` asks for it.
#
# THE DIRECTORY IS LOOKED UP, NEVER RECOMPUTED, and the rule lives in ONE shell
# file, .claude/hooks/lib/session-dir.sh (_session_dir), which this helper and
# the hooks' state-file.sh both source. make (mk/session.mk) and Go
# (internal/test/sessionpath) implement the same rule for their own callers, and
# TestMakeAndGoAgreeOnBinDir (scripts/dev/session_bin_dir_test.py) is what stops
# the three drifting.
#
# <session-id> is the canonical id the hooks resolve (.claude/hooks/lib/session-id.sh),
# so this helper and every other consumer of the directory agree on the path.
#
# Usage (run from the checkout root; the printed path is root-relative):
#   dir=$(scripts/dev/session-scratch.sh)   # <session-dir>/scratch/, created for you
#   make ze-unit-test-changed > "$dir/unit.log" 2>&1
#   scripts/dev/session-scratch.sh --path   # print the path WITHOUT creating it
#   scripts/dev/session-scratch.sh --clean  # remove THIS session's dir (`make clean`)
#
# --clean removes the WHOLE session directory, binaries included: it is what
# `make clean` runs, and `make clean` removes bin/ for an off-session build.
#
# No `set -u`: session-id.sh reads $CLAUDE_CODE_SESSION_ID and
# $CLAUDE_CODE_SESSION_ACCESS_TOKEN without defaults (matches spec-session.sh).

# Resolve the id helper relative to THIS script (the real checkout), so it is
# found no matter the caller's cwd -- that is what lets the tests run isolated.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=../../.claude/hooks/lib/session-id.sh
source "$SCRIPT_DIR/../../.claude/hooks/lib/session-id.sh"
# shellcheck source=../../.claude/hooks/lib/session-dir.sh
source "$SCRIPT_DIR/../../.claude/hooks/lib/session-dir.sh"

# Operate at the checkout root. Prefer $CLAUDE_PROJECT_DIR so this agrees with
# the hooks (which use it); otherwise the git toplevel of the caller's cwd (a
# test's throwaway repo resolves to itself here).
root="${CLAUDE_PROJECT_DIR:-}"
if [ -z "$root" ]; then
    root=$(git rev-parse --show-toplevel 2>/dev/null) || root="$(cd "$SCRIPT_DIR/../.." && pwd)"
fi
cd "$root" || exit 1

sid=$(_session_id)
# Refuse an id that is empty, path-bearing, or a dot entry, so we can never
# escape tmp/session/ under --clean. _sid_safe already drops
# '/', globs and whitespace, but it permits '.' and '..'.
case "$sid" in
    "" | */* | . | ..)
        echo "session-scratch: unsafe session id '${sid}'" >&2
        exit 1
        ;;
esac

session=$(_session_dir "$sid")
dir="$session/scratch"

case "${1:-}" in
    # Remove THIS session's whole directory -- binaries and scratch -- and print
    # nothing. `make clean` runs this, and it removes bin/ off-session.
    --clean) rm -rf "$session"; exit 0 ;;
    --path) ;;                        # print only, do not create
    "") mkdir -p "$dir" || { echo "session-scratch: cannot create $dir" >&2; exit 1; } ;;
    *) echo "usage: session-scratch.sh [--path|--clean]" >&2; exit 2 ;;
esac

printf '%s\n' "$dir"
