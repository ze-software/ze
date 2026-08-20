#!/usr/bin/env bash
# changed-pkgs.sh -- emit the Go package directories that scoped verification
# (`make ze-precommit-verify-changed`) must cover.
#
# This script HOLDS NO SELECTION LOGIC. The change set is computed by exactly
# one program, scripts/checks/verify_scope_selector.go
# (plan/spec-verify-scope-2-change-set-selector.md), which answers with a
# tag-aware reverse import graph bounded at depth 2 and a classification table
# for the non-Go kinds. The transitive, untagged expansion this file used to
# carry is DELETED rather than kept beside it: two implementations of one change
# set drift, and one of them is always the wrong one (ai/rules/no-layering.md).
#
# What is left here is the dispatch between the two ways a caller reaches that
# one answer:
#
#   1. Inside a verify run. scripts/status/verify_run.go runs the selector ONCE
#      before the first stage, writes the answer into the run's own artifact
#      directory, and exports ZE_VERIFY_SCOPE_PACKAGES naming that file. Every
#      scoped stage then reads a file instead of paying for its own `go list`,
#      and every stage in one run scopes to the same tree.
#   2. Outside a verify run. A direct `make ze-lint-changed` has no precomputed
#      answer, so the selector runs here and costs its measured 2.4 to 2.9s.
#
# Fail open, on every route. The selector refusing to answer MUST NOT read as
# "no package to verify": that turns a fail-open into a fail-closed and the
# scoped gate would verify nothing while reporting success. Every failure route
# below prints ./... and exits 0 (ai/rules/evidence.md -- a zero value must
# never be a valid-looking answer).
#
# Arguments are passed through to the selector (--depth=, --paths-from=,
# --drop-log=). An argument asks a different question than the one the run
# precomputed, so the precomputed answer is used only when there is none.
#
# Output: one `./`-prefixed package directory per line, sorted and unique, or
# the single word `./...` when the run must widen. Empty output means no changed
# path is compiled or read by a Go package, and the selector says so on stderr.
#
# Env:
#   ZE_VERIFY_SCOPE_PACKAGES  file holding this run's precomputed answer
#   ZE_VERIFY_STATUS_FILE     verify status file the selector reads the green
#                             baseline from (default: tmp/ze-verify.status)

set -u

# EVERY_PACKAGE is the widest answer. go test, go build and golangci-lint all
# accept it, and it is the same word the selector itself widens with.
EVERY_PACKAGE='./...'

widen() {
	printf 'changed-pkgs: %s, so every package is selected\n' "$1" >&2
	printf '%s\n' "$EVERY_PACKAGE"
	exit 0
}

if [ $# -eq 0 ] && [ -n "${ZE_VERIFY_SCOPE_PACKAGES:-}" ]; then
	if [ ! -r "$ZE_VERIFY_SCOPE_PACKAGES" ]; then
		widen "ZE_VERIFY_SCOPE_PACKAGES names $ZE_VERIFY_SCOPE_PACKAGES, which cannot be read"
	fi
	cat -- "$ZE_VERIFY_SCOPE_PACKAGES"
	exit 0
fi

# The selector is named by an absolute path: a caller can run this script from
# any directory, and the directory it runs in is the repository the answer is
# about.
repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd -P)
selector="$repo_root/scripts/checks/verify_scope_selector.go"

if ! packages=$(CGO_ENABLED=0 go run "$selector" --print=packages "$@"); then
	widen "the selector could not answer"
fi

# An empty answer is an answer, and printf would turn it into a blank line that
# a caller reading `$(...)` cannot tell from a package name.
[ -n "$packages" ] && printf '%s\n' "$packages"
exit 0
