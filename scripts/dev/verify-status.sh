#!/usr/bin/env bash
# verify-status.sh -- read/write the last-verify fingerprint.
#
# Commands:
#   write <exit_code>   Write tmp/ze-verify.status for the current tree state,
#                       plus tmp/ze-verify-manifest.txt, a per-path fingerprint
#                       of everything that differs from HEAD.
#   check [PATH...]     With no PATH: print FRESH if current tree_hash == last
#                       PASS hash, else STALE with reason.
#                       With PATHs: ask the same question about THOSE paths only
#                       (a directory scopes to everything under it). Several
#                       sessions share this checkout, so a whole-tree answer is
#                       STALE almost always, and almost always for somebody
#                       else's file. A commit is scoped to a file list and its
#                       evidence must be scopeable to the same list.
#                       Exit 0 if FRESH, 1 if STALE.
#   show                Dump the current status file (human-readable).
#   tree_hash           Print the current tree_hash and nothing else. This is
#                       the one fingerprint of the working tree in the repo, and
#                       scripts/dev/ze-run.sh records it with each job so a
#                       later asker can tell whether a running job has already
#                       seen its code.
#
# tree_hash = sha256 of:
#   - git rev-parse HEAD
#   - git diff HEAD (staged + unstaged tracked changes)
#   - sorted list of untracked files + sha256 of each file's content
#
# Claude uses this to skip re-running verify when the working tree is
# byte-identical to the one the last PASS covered.

set -e

STATUS_FILE="tmp/ze-verify.status"

hash_file() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum -- "$1" | awk '{print $1}'
    else
        shasum -a 256 -- "$1" | awk '{print $1}'
    fi
}

hash_stream() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum | awk '{print $1}'
    else
        shasum -a 256 | awk '{print $1}'
    fi
}


tree_hash() {
    {
        git rev-parse HEAD 2>/dev/null || echo "NO_HEAD"
        git diff HEAD 2>/dev/null || true
        git ls-files -o --exclude-standard 2>/dev/null | LC_ALL=C sort | while IFS= read -r f; do
            printf '%s\n' "$f"
            if [ -f "$f" ]; then
                hash_file "$f"
            else
                echo "MISSING"
            fi
        done
    } | hash_stream
}

# Per-path fingerprints of everything that differs from HEAD, written beside the
# status file.
#
# Why this exists. `tree_hash` folds the WHOLE tree into one number, so any edit
# anywhere flips it. Several sessions share this checkout, and it routinely
# carries 300+ uncommitted files, so a whole-tree fingerprint goes STALE within
# seconds of a PASS and stays that way -- for work in files the asking session
# never touched. The commit is scoped to a file list; the evidence must be
# scopeable to the same list, or a session can never hold evidence about its own
# code (`ai/rules/git-safety.md`).
#
# Only paths that DIFFER from HEAD are recorded (a few hundred), never every
# tracked file (7600+). A path in neither manifest is identical to HEAD in both,
# which is the same answer as matching hashes. A path recorded on one side only
# has changed, which is what `manifest_scoped` compares.
MANIFEST_FILE="tmp/ze-verify-manifest.txt"

dirty_manifest() {
    {
        git diff HEAD --name-only 2>/dev/null || true
        git ls-files -o --exclude-standard 2>/dev/null || true
    } | LC_ALL=C sort -u | while IFS= read -r f; do
        [ -n "$f" ] || continue
        if [ -f "$f" ]; then
            printf '%s %s\n' "$(hash_file "$f")" "$f"
        else
            printf 'MISSING %s\n' "$f"
        fi
    done
}

# The manifest rows for the given paths, from a file or from the live tree.
# A directory argument scopes to everything under it.
manifest_scoped() {
    src="$1"
    shift
    for p in "$@"; do
        awk -v p="$p" '
            { path = $2 }
            path == p || index(path, p "/") == 1 { print }
        ' "$src"
    done | LC_ALL=C sort -u
}

cmd="${1:-}"

case "$cmd" in
    write)
        code="${2:-1}"
        mode="${3:-ze-verify}"
        mkdir -p tmp
        dirty_manifest > "$MANIFEST_FILE"
        {
            printf 'exit=%s\n' "$code"
            printf 'timestamp=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
            printf 'mode=%s\n' "$mode"
            printf 'skipped=%s\n' "${ZE_SKIP_SUITES:-}"
            printf 'git_sha=%s\n' "$(git rev-parse HEAD 2>/dev/null || echo unknown)"
            printf 'tree_hash=%s\n' "$(tree_hash)"
        } > "$STATUS_FILE"
        ;;
    check)
        if [ ! -f "$STATUS_FILE" ]; then
            echo "STALE: no status file (never verified)"
            exit 1
        fi
        # shellcheck disable=SC1090
        . "$STATUS_FILE"
        if [ "${exit:-1}" != "0" ]; then
            echo "STALE: last verify failed (exit=$exit, at $timestamp)"
            exit 1
        fi
        if [ -n "${skipped:-}" ]; then
            # A pass with suites skipped via ZE_SKIP_SUITES is partial: it must
            # not block a real verify before commit.
            echo "STALE: last pass skipped suites ($skipped) at $timestamp"
            exit 1
        fi
        shift
        if [ "$#" -gt 0 ]; then
            # Scoped check: did THESE paths change since the PASS? Everything the
            # run also covered may have moved without making this answer wrong,
            # because the caller is asking about its own file list.
            #
            # HEAD must match: a commit landing under a path rewrites what that
            # path's content means even when the working file is untouched.
            head_now=$(git rev-parse HEAD 2>/dev/null || echo unknown)
            if [ "$head_now" != "${git_sha:-unknown}" ]; then
                echo "STALE: HEAD moved since PASS at $timestamp ($git_sha -> $head_now)"
                exit 1
            fi
            if [ ! -f "$MANIFEST_FILE" ]; then
                echo "STALE: no per-path manifest (PASS predates scoped checking)"
                exit 1
            fi
            live=$(mktemp)
            trap 'rm -f "$live"' EXIT
            dirty_manifest > "$live"
            if [ "$(manifest_scoped "$MANIFEST_FILE" "$@")" = "$(manifest_scoped "$live" "$@")" ]; then
                echo "FRESH(${mode:-ze-verify}): $# scoped path(s) unchanged since PASS at $timestamp (sha $git_sha)"
                exit 0
            fi
            echo "STALE: a scoped path changed since last PASS at $timestamp"
            exit 1
        fi
        current=$(tree_hash)
        if [ "$current" = "$tree_hash" ]; then
            # mode qualifies the freshness: ze-precommit-verify-changed is a weaker pass
            # than full ze-precommit-verify (no full lint, no vet evidence, no cached
            # full unit pass). Status files from before the mode field default
            # to the full label.
            echo "FRESH(${mode:-ze-verify}): tree unchanged since PASS at $timestamp (sha $git_sha)"
            exit 0
        else
            echo "STALE: tree changed since last PASS at $timestamp"
            exit 1
        fi
        ;;
    show)
        if [ -f "$STATUS_FILE" ]; then
            cat "$STATUS_FILE"
        else
            echo "no status file at $STATUS_FILE"
            exit 1
        fi
        ;;
    tree_hash)
        tree_hash
        ;;
    *)
        echo "usage: $0 {write EXIT_CODE|check [PATH...]|show|tree_hash}" >&2
        exit 2
        ;;
esac
