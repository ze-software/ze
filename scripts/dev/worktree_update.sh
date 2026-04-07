#!/bin/bash
# Update a git worktree from main: stash → rebase main → stash pop
# SAFETY: Only runs inside a git worktree, never on the main working tree.
#
# Usage:
#   scripts/dev/worktree_update.sh                  # update current worktree
#   scripts/dev/worktree_update.sh <worktree-path>  # update specific worktree
#   scripts/dev/worktree_update.sh --all            # update all worktrees

set -euo pipefail

MAIN_DIR="$(git -C "$(dirname "$0")/.." rev-parse --show-toplevel)"

die() { echo "error: $*" >&2; exit 1; }

is_worktree() {
    local dir="$1"
    # A worktree has a .git file (not directory) pointing to the main repo's worktrees/
    [ -f "$dir/.git" ] || return 1
    # Double-check: git itself confirms it's a worktree
    local wt_root
    wt_root="$(git -C "$dir" rev-parse --show-toplevel 2>/dev/null)" || return 1
    local main_root
    main_root="$(git -C "$dir" worktree list --porcelain | head -1 | sed 's/^worktree //')"
    [ "$wt_root" != "$main_root" ]
}

update_worktree() {
    local wt="$1"

    if ! is_worktree "$wt"; then
        die "not a worktree: $wt"
    fi

    local branch
    branch="$(git -C "$wt" branch --show-current)"
    echo "── updating $wt (branch: $branch)"

    # Check if there are changes to stash
    local needs_stash=false
    if ! git -C "$wt" diff --quiet || ! git -C "$wt" diff --cached --quiet; then
        needs_stash=true
    fi

    if [ "$needs_stash" = true ]; then
        echo "   stashing changes..."
        git -C "$wt" stash push -m "worktree-update: auto-stash before rebase"
    fi

    echo "   rebasing onto main..."
    if ! git -C "$wt" rebase main; then
        echo "   rebase conflict — aborting rebase" >&2
        git -C "$wt" rebase --abort
        if [ "$needs_stash" = true ]; then
            echo "   restoring stash..." >&2
            git -C "$wt" stash pop
        fi
        die "rebase failed for $wt — resolve manually"
    fi

    if [ "$needs_stash" = true ]; then
        echo "   popping stash..."
        if ! git -C "$wt" stash pop; then
            echo "   stash pop conflict — stash preserved as stash@{0}" >&2
            die "stash pop failed for $wt — resolve manually"
        fi
    fi

    local new_head
    new_head="$(git -C "$wt" rev-parse --short HEAD)"
    echo "   done (HEAD: $new_head)"
}

# --- Main ---

if [ "${1:-}" = "--all" ]; then
    # Update every worktree except the main working tree
    while IFS= read -r wt_path; do
        [ "$wt_path" = "$MAIN_DIR" ] && continue
        update_worktree "$wt_path"
    done < <(git worktree list --porcelain | grep '^worktree ' | sed 's/^worktree //')
    echo "all worktrees updated"

elif [ -n "${1:-}" ]; then
    # Specific worktree path
    update_worktree "$1"

else
    # Current directory
    update_worktree "$(pwd)"
fi
