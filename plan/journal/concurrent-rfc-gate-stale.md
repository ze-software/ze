| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-07-30 | - | workflow | `make ze-rfc-check` failed because another session shifted tagged test line numbers | regenerated `ai/RFC-REQUIREMENTS.md` before verify |
| 2026-07-30 | - | workflow | same staleness from a different concurrent session's edits | diffed regenerated ledger to identify owning package |
| 2026-08-11 | rfc-ledger-per-rfc-shards | git | commit 7ec29b6e6 was an unrelated IPsec fix. It absorbed the split `ai/RFC-REQUIREMENTS.md` while the 177 `rfc/requirements/` files stayed untracked. HEAD then held an index citing files no commit provided | the closure commits those files. History is not rewritten |
