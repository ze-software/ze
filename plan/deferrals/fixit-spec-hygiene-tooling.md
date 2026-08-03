# Deferrals: fixit-spec-hygiene-tooling

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-17 | spec-fixit-spec-hygiene-tooling ("Note (record, do not implement here)") | Two repo-state chores: (1) close `spec-ipsec-13-rekey-wire`, flagged HIGH-confidence completed-but-not-closed (verified 2026-07-17: the file exists and its Status field still reads `in-progress`); (2) prune the un-indexed `plan/learned/` files | Left undone by the source spec by design: it was BUILDING the hygiene checks, not consuming their output, and closing someone else's spec is not tooling work. Both are chores against repo state, so neither needs a design phase -- but item 1 must read `spec-ipsec-13-rekey-wire`'s Review Gate before closing (the flag is a signal, not evidence; `ai/rules/completion.md`) and item 2 must establish the un-indexed set first, since pruning a referenced summary destroys the only record of a lesson (`ai/rules/never-destroy-work.md`) | `plan/learned/1240-fixit-spec-hygiene-tooling-deferred-operational-cleanup.md` (both chores done 2026-07-21: ipsec-13 verified complete + closed; ze-regen-check green after regenerating stale CONDENSED.md/CODE-TO-DOCS.md) | done |

