#!/bin/sh

set -eu

SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
WORKTREE=$(git -C "$SCRIPT_DIR" rev-parse --show-toplevel 2>/dev/null) || {
    echo "error: presentation tools are not inside a git worktree" >&2
    exit 1
}
COMMON=$(git -C "$SCRIPT_DIR" rev-parse --path-format=absolute --git-common-dir)
MAIN=$(dirname "$COMMON")

if [ -d "$MAIN/internal" ]; then
    printf '%s\n' "$MAIN"
elif [ -d "$WORKTREE/internal" ]; then
    printf '%s\n' "$WORKTREE"
else
    echo "error: could not locate the Ze source worktree" >&2
    exit 1
fi
