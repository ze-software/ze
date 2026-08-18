---
kind: note
level:
stage:
---
This is enforced, not honor-system: `scripts/dev/commit_helper.py create` reads
`tmp/ze-verify-failures.json` (which `verify_run.go` rewrites after every run) and
refuses to prepare a script while any structural gate is red, even with
`--unverified` (`structural_gate_reds` / `STRUCTURAL_GATES`). A green verify
rewrites the artifact, so a fixed-and-reverified gate clears automatically.
