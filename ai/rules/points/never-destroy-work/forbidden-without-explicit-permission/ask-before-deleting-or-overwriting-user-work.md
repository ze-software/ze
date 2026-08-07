---
kind: table
level:
stage:
---
| Operation | Scope | Replacement |
|-----------|-------|-------------|
| `rm <path>` on user-visible files | Any path not already in git's untracked-trash-bin category | Ask for permission before deleting it. If deletion is the correct fix, do not leave the file behind as a workaround. |
| `git restore <path>` | Any modified working-tree file | Already in `ai/rules/git-safety.md`; same rule |
| `git reset --hard`, `git clean -f` | Any | Already forbidden |
| Overwriting an existing file with content that drops user edits | Overwriting unsaved changes | Read the current file; merge or ask |
| Truncating / overwriting log files, session state, tmp/ artifacts the user might be inspecting | Any | Leave it |
