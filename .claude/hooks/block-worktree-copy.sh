#!/bin/bash
# BLOCKING HOOK: Prevent copying files from worktrees into the main repo
# Worktree agents must commit and merge — never copy files directly.
# Copying from a worktree overwrites uncommitted changes by other sessions.
# Exit code 2 = BLOCK the operation

COMMAND="$CLAUDE_TOOL_INPUT_command"
WORKTREE_DIR=".claude/worktrees"

block() {
    echo "❌ Blocked: copying files from worktree to main repo" >&2
    echo "Worktree agents must commit their changes. Use git merge or cherry-pick." >&2
    echo "Direct file copying overwrites uncommitted work from other sessions." >&2
    exit 2
}

# --- Bash: block cp/mv/rsync/install from worktree paths ---
if [[ -n "$COMMAND" && "$COMMAND" == *"$WORKTREE_DIR"* ]]; then
    DESTRUCTIVE_CMDS=("cp " "cp -" "mv " "mv -" "rsync " "install ")
    for cmd in "${DESTRUCTIVE_CMDS[@]}"; do
        if [[ "$COMMAND" == *"$cmd"* ]]; then
            block
        fi
    done
    # Block shell redirections from worktree files (cat worktree/file > main/file)
    if [[ "$COMMAND" == *"$WORKTREE_DIR"* && "$COMMAND" =~ [[:space:]]\> ]]; then
        block
    fi
fi

exit 0
