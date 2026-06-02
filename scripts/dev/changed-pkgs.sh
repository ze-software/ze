#!/usr/bin/env bash
# changed-pkgs.sh -- emit the Go package directories that scoped
# verification (`make ze-verify-changed`) must cover.
#
# The set is the union of:
#   1. packages with uncommitted .go changes (unstaged + staged + untracked)
#   2. packages changed by commits made SINCE the last GREEN `make ze-verify`
#
# Why (2): ze-verify-changed historically derived its package set from the
# working-tree diff alone. Once a change was committed it left that diff, so
# a scoped verify on the now-clean tree tested NOTHING in that package and
# reported green even when the committed package's tests were red (a real
# regression slipped in this way -- see
# plan/learned/842-scoped-verify-committed-gap.md). Diffing against the SHA
# recorded by the last passing verify closes that hole. The baseline is used
# ONLY when the last verify PASSED (exit=0) and its SHA is a reachable commit
# in this repository; otherwise we have no trusted green point and fall back
# to the working-tree-only set (no worse than the historical behaviour).
#
# Output: one `./`-prefixed, existing package directory per line, sorted and
# de-duplicated. Empty output means there is nothing to verify.
#
# Env:
#   ZE_VERIFY_STATUS_FILE  verify status file to read the green baseline from
#                          (default: tmp/ze-verify.status)

STATUS_FILE="${ZE_VERIFY_STATUS_FILE:-tmp/ze-verify.status}"

collect_files() {
	git diff --name-only -- '*.go' 2>/dev/null || true
	git diff --cached --name-only -- '*.go' 2>/dev/null || true
	git ls-files --others --exclude-standard -- '*.go' 2>/dev/null || true

	# Committed-since-last-green: only with a trusted green baseline.
	if [ -f "$STATUS_FILE" ]; then
		last_exit=$(sed -n 's/^exit=//p' "$STATUS_FILE" | head -1)
		last_sha=$(sed -n 's/^git_sha=//p' "$STATUS_FILE" | head -1)
		if [ "$last_exit" = "0" ] && [ -n "$last_sha" ] && [ "$last_sha" != "unknown" ] &&
			git cat-file -e "${last_sha}^{commit}" 2>/dev/null; then
			git diff --name-only "$last_sha" HEAD -- '*.go' 2>/dev/null || true
		fi
	fi
}

collect_files |
	sort -u |
	while IFS= read -r file; do
		[ -n "$file" ] || continue
		dir=$(dirname "$file")
		[ -d "$dir" ] && printf './%s\n' "$dir"
	done |
	sort -u
