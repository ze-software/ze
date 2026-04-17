#!/usr/bin/env bash
# verify-status.sh -- read/write the last-verify fingerprint.
#
# Commands:
#   write <exit_code>   Write tmp/ze-verify.status for the current tree state.
#   check               Print FRESH if current tree_hash == last PASS hash,
#                       else STALE with reason. Exit 0 if FRESH, 1 if STALE.
#   show                Dump the current status file (human-readable).
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

tree_hash() {
    {
        git rev-parse HEAD 2>/dev/null || echo "NO_HEAD"
        git diff HEAD 2>/dev/null || true
        git ls-files -o --exclude-standard 2>/dev/null | LC_ALL=C sort | while IFS= read -r f; do
            printf '%s\n' "$f"
            if [ -f "$f" ]; then
                sha256sum -- "$f" 2>/dev/null | awk '{print $1}'
            else
                echo "MISSING"
            fi
        done
    } | sha256sum | awk '{print $1}'
}

cmd="${1:-}"

case "$cmd" in
    write)
        code="${2:-1}"
        mkdir -p tmp
        {
            printf 'exit=%s\n' "$code"
            printf 'timestamp=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
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
        current=$(tree_hash)
        if [ "$current" = "$tree_hash" ]; then
            echo "FRESH: tree unchanged since PASS at $timestamp (sha $git_sha)"
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
    *)
        echo "usage: $0 {write EXIT_CODE|check|show}" >&2
        exit 2
        ;;
esac
