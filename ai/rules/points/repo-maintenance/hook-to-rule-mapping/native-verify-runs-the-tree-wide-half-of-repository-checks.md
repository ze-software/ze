---
kind: note
level:
stage:
---
`./le verify worktree` separately runs `./le repository tree-check`, which executes the three tree-wide checks in `internal/le/repository`. The other two checks are changed-file scoped; `./le repository check` runs all five over the current tree.
