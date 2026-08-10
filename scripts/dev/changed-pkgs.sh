#!/usr/bin/env bash
# changed-pkgs.sh -- emit the Go package directories that scoped
# verification (`make ze-verify-changed`) must cover.
#
# The set is the union of:
#   1. packages with uncommitted .go changes (unstaged + staged + untracked)
#   2. packages changed by commits made SINCE the last GREEN `make ze-verify`
#   3. packages that IMPORT any package from (1)+(2) -- reverse dependencies;
#      a behavior change in a core package must retest its importers, not
#      only itself
#
# Why (2): ze-verify-changed historically derived its package set from the
# working-tree diff alone. Once a change was committed it left that diff, so
# a scoped verify on the now-clean tree tested NOTHING in that package and
# reported green even when the committed package's tests were red (a real
# regression slipped in this way). Diffing against the SHA recorded by the last
# passing verify closes that hole. The baseline is used ONLY when the last
# verify PASSED (exit=0) and its SHA is a reachable commit in this repository;
# otherwise we have no trusted green point and fall back to the working-tree-only
# set (no worse than the historical behaviour).
#
# Non-Go inputs that a Go test EXECUTES are collected too, and map to the
# package that runs them (see PYTHON_TEST_PKG below). Filtering every query on
# '*.go' meant a Python-only or corpus-only change selected ZERO packages and
# `make ze-verify-changed` exited 0 having tested nothing -- including for a
# change to scripts/dev/rfc_requirements.py itself, whose 295-test suite is run
# by scripts/dev/python_tests_test.go. Enrolling an RFC, committing an
# extraction sign-off or arming the drain rate had the same hole: those files
# are read by the tests at run time, so a corpus change can red them while the
# scoped verify looks only at .go paths and reports green.
#
# Output: one `./`-prefixed, existing package directory per line, sorted and
# de-duplicated. Empty output means there is nothing to verify.
#
# Env:
#   ZE_VERIFY_STATUS_FILE  verify status file to read the green baseline from
#                          (default: tmp/ze-verify.status)

STATUS_FILE="${ZE_VERIFY_STATUS_FILE:-tmp/ze-verify.status}"

# The Go package holding python_tests_test.go, which globs and runs every
# *_test.py under its pythonTestRoots (scripts/dev, test/scripts, test/perf).
# It is the only package that executes Python, so every .py maps here whatever
# root it lives in -- a test/scripts/*.py has no Go package of its own.
PYTHON_TEST_PKG="./scripts/dev"

# Pathspecs for the file kinds that can change what a test does.
#   *.go      the package's own sources
#   *.py      run by PYTHON_TEST_PKG
#   rfc       the RFC corpus the rfc_requirements suite reads live (enrolled.txt,
#             short/*.md, extraction/*.json, drain-budget.txt)
#
# An ARRAY, expanded as "${PATHSPECS[@]}". A single space-separated string
# expanded unquoted is word-split AND glob-expanded by the shell before git sees
# it, so `*.go` matched the repo-root tools.go and git was handed one literal
# path instead of the pattern -- silently narrowing the whole selection to that
# file. The globs must reach git verbatim; only git knows the whole tree.
PATHSPECS=('*.go' '*.py' 'rfc')

collect_files() {
	git diff --name-only -- "${PATHSPECS[@]}" 2>/dev/null || true
	git diff --cached --name-only -- "${PATHSPECS[@]}" 2>/dev/null || true
	git ls-files --others --exclude-standard -- "${PATHSPECS[@]}" 2>/dev/null || true

	# Committed-since-last-green: only with a trusted green baseline.
	if [ -f "$STATUS_FILE" ]; then
		last_exit=$(sed -n 's/^exit=//p' "$STATUS_FILE" | head -1)
		last_sha=$(sed -n 's/^git_sha=//p' "$STATUS_FILE" | head -1)
		if [ "$last_exit" = "0" ] && [ -n "$last_sha" ] && [ "$last_sha" != "unknown" ] &&
			git cat-file -e "${last_sha}^{commit}" 2>/dev/null; then
			git diff --name-only "$last_sha" HEAD -- "${PATHSPECS[@]}" 2>/dev/null || true
		fi
	fi
}

changed_dirs=$(
	collect_files |
		sort -u |
		while IFS= read -r file; do
			[ -n "$file" ] || continue
			# Never lint/test vendored third-party code: adding a new dependency
			# makes `go mod vendor` write many untracked files, but they are not
			# ours to verify (the full ze-lint/ze-unit-test skip vendor too).
			case "$file" in vendor/*) continue ;; esac
			# A Python file or an RFC corpus file is INPUT to a Go test rather
			# than a package member, so its own directory is not a Go package
			# (test/scripts, rfc/short). Map it to the package that runs it.
			case "$file" in
			*.py | rfc/*)
				printf '%s\n' "$PYTHON_TEST_PKG"
				continue
				;;
			esac
			dir=$(dirname "$file")
			[ -d "$dir" ] && printf './%s\n' "$dir"
		done |
		sort -u
)

[ -n "$changed_dirs" ] || exit 0

# Reverse dependencies: a package IMPORTING a changed package can break even
# when none of its own files changed; scoped verify must retest importers.
# Skipped when there is no Go module context (go list unavailable).
importer_dirs=""
module=$(go list -m 2>/dev/null | head -1)
if [ -n "$module" ]; then
	importer_dirs=$(
		{
			printf '%s\n' "$changed_dirs" | sed 's|^\./|CHANGED |'
			go list -f '{{.ImportPath}} {{range .Deps}}{{.}} {{end}}' ./... 2>/dev/null
		} | awk -v module="$module" '
			$1 == "CHANGED" {
				if ($2 == ".") changed[module] = 1
				else changed[module "/" $2] = 1
				next
			}
			{
				for (i = 2; i <= NF; i++) {
					if ($i in changed) { print $1; break }
				}
			}
		' | while IFS= read -r ip; do
			if [ "$ip" = "$module" ]; then
				dir="."
			else
				dir="${ip#"$module"/}"
			fi
			[ -d "$dir" ] && printf './%s\n' "$dir"
		done
	)
fi

combined=$(
	{
		printf '%s\n' "$changed_dirs"
		[ -n "$importer_dirs" ] && printf '%s\n' "$importer_dirs"
	} | sort -u
)

# Drop directories that do not form a buildable package (e.g. scripts/ build
# tools whose only .go files carry `//go:build ignore`): golangci-lint and
# `go test` fail on them with "build constraints exclude all Go files".
# Without a module context, emit the set unfiltered (matches earlier behavior).
module_name=$(go list -m 2>/dev/null | head -1)
if [ -n "$module_name" ] && [ "$module_name" != "command-line-arguments" ]; then
	root=$(pwd -P)
	# shellcheck disable=SC2086 # word-splitting of the dir list is intended
	go list -e -f '{{if not .Error}}{{.Dir}}{{end}}' $combined 2>/dev/null |
		awk -v root="$root" 'length($0) {
			if (index($0, root "/") == 1) { print "./" substr($0, length(root) + 2) }
			else if ($0 == root) { print "." }
		}' | sort -u
else
	printf '%s\n' "$combined"
fi
