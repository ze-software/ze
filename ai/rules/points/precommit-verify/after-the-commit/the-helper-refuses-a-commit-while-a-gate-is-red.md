---
kind: note
level:
stage:
---
This is enforced, not honor-system: `scripts/dev/commit_helper.py create` reads
`tmp/ze-verify-failures.json` (which `verify_run.go` rewrites after every run) and
refuses to prepare a script while a structural gate red is charged to this
commit, even with `--unverified` (`structural_gate_reds` / `STRUCTURAL_GATES`).
A red is charged unless every file its failure groups name lies outside the
commit's `--file` list, and a group that names no file is charged as it always
was ("Reading A Red", above). A green verify rewrites the artifact, so a
fixed-and-reverified gate clears automatically.
