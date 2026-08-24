#!/bin/bash
# The ONE shell definition of a session's directory under tmp/session/.
# Usage: source this file, then call _session_dir <session-id>.
#
# Every session owns one directory, tmp/session/<YYYY-MM-DD>-<session-id>/, and
# everything that session writes lands in a subdirectory of it:
#
#   bin/      this session's binaries, and the etc/ze they resolve (mk/helper-session.mk,
#             internal/test/sessionpath)
#   scratch/  ad-hoc logs, probes, captures (scripts/dev/session-scratch.sh)
#   state/    the per-spec digest session-state-<stem>-<sid>.md (lib/state-file.sh)
#
# Those three are the NAMED subdirectories, not the whole population: a
# functional run also drops its own throwaway roots here (testbin-<id>/ from
# mk/test-functional.mk, ze-functional-*/ from sessionpath.EnsureScratchRoot).
# They are siblings of the three, and they are what makes the directory the one
# place an operator has to look.
#
# THE DIRECTORY IS LOOKED UP, NEVER RECOMPUTED. <YYYY-MM-DD>-<sid> is not a pure
# function of the id, so this helper takes the single directory matching
# tmp/session/????-??-??-<sid>, and names a new one with today's date only on a
# miss. Recomputing from today's date would move a session's directory at
# midnight and orphan the binaries that session is running.
#
# make (mk/helper-session.mk), Go (internal/test/sessionpath) and the Write/Edit hook
# (.claude/hooks/pretool-writeedit.py session_dir()) implement the same rule for
# their own callers. This file is what stops the SHELL half being spelled twice:
# scripts/dev/session-scratch.sh and .claude/hooks/lib/state-file.sh both call it.
# TestMakeAndGoAgreeOnBinDir (scripts/dev/session_bin_dir_test.py) is what stops
# the language copies drifting.
#
# It CREATES NOTHING. The caller decides whether to mkdir what it names, so a
# reader can ask where a digest would live without minting a directory.
#
# Paths are relative to the checkout root, so a caller runs `cd` to the root
# first (the hooks use $CLAUDE_PROJECT_DIR; session-scratch.sh resolves it).

# The ONE root for per-session state: the dated directories sit beside the flat
# marker files the hooks write (.sid-by-pid-<clipid>, .closure-ack-<stem>, and
# the gate markers).
ZE_SESSION_ROOT="tmp/session"

_session_dir() {
    local sid="$1" d
    # An unmatched glob expands to the literal pattern, which `[ -d ]` rejects,
    # so the loop falls through to today's date with no nullglob needed. A sid
    # is only ever used after _sid_safe accepted it, so it carries no glob
    # character.
    for d in "$ZE_SESSION_ROOT"/????-??-??-"$sid"; do
        if [ -d "$d" ]; then
            printf '%s\n' "$d"
            return 0
        fi
    done
    printf '%s\n' "$ZE_SESSION_ROOT/$(date +%Y-%m-%d)-$sid"
}
