---
kind: note
level: MUST
stage:
---
`./le verify worktree` is the **final gate**, not a development tool. Use targeted commands and component groups during iteration.
On failure, `./le verify worktree` writes the compact index `tmp/ze-verify-failures.log`.
Read that file first. The next run MUST be the listed `Rerun` command for the
failed stage, or an even narrower single test/package from the detail log. If
multiple failures are listed, clear each one with its focused rerun. Only after
all focused reruns pass may you rerun the whole suite or gate as final
confirmation. The combined log is `tmp/ze-verify.log`, and automation can read
`tmp/ze-verify-failures.json`.
