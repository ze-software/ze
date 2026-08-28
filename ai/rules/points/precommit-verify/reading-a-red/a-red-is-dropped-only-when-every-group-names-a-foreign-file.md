---
kind: directive
level: MUST NOT
stage:
---
**A structural gate red is charged to your commit unless EVERY file its failure groups name lies outside your `--file` list. You MUST NOT expect attribution to drop a red whose groups name no file at all.** `structural_gate_reds` (`internal/le/commit`) reports three sets: `charged` refuses the commit, `foreign` names each gate the file list ruled out, and `unattributed` names each group that carries a check name, a suite name or the stage's own name. A group that names nothing is charged exactly as before, and the refusal prints which one it was. Attribution reaches the gates whose groups name a file or a package directory, and `./le doc-wiring` is one of them: its sub-checks declare their own failure groups (`declare_failure_group`, `internal/le/docwiring/docwiring.go`). Its ci-sleep ratchet and its delegated targets judge a population rather than a file, so those two still charge.
