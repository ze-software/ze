---
kind: table
level:
stage:
---
| Gate | Where | Fires when |
|------|-------|-----------|
| Detector | `scripts/dev/spec-closure-check.py` | `--list` reports completed-but-not-closed specs in two tiers; `--spec <s>` exits 3 only for a high-confidence one. High confidence = a **committed** journal row in `plan/journal/*.md` whose Spec cell exactly equals the spec stem, or a `plan/learned/NNN-<slug>.md` whose slug exactly equals the stem, while the spec is still `in-progress` and is **not an umbrella** (commit A ran, commit B did not). Weaker `[umbrella]` / `[weak-match]` candidates are listed under NEEDS VERIFICATION. Only the high-confidence set triggers the `--spec` block. |
| Stop-hook block | `.claude/hooks/block-premature-stop.sh` | This session CLAIMED a spec, the detector exits 3 for it, and no ack exists. The hook refuses the session an end (exit 2). Escape: record why the spec is genuinely open in `tmp/session/.closure-ack-<stem>`. A session that claimed no spec is never asked to close one. The gate carries no retry bound on purpose: a refused stop leaves it armed next turn, and it has two escapes of its own (run commit B, or write the ack). |
| Commit reminder | `scripts/dev/commit_helper.py` | A commit adds a journal row or learned summary but removes no spec: it prints the closure-commit reminder to stderr. |
