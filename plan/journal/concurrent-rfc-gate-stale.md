| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-07-30 | - | workflow | `make ze-rfc-check` failed because another session shifted tagged test line numbers | regenerated `ai/RFC-REQUIREMENTS.md` before verify |
| 2026-07-30 | - | workflow | same staleness from a different concurrent session's edits | diffed regenerated ledger to identify owning package |
