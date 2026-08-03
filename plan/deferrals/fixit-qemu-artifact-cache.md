# Deferrals: fixit-qemu-artifact-cache

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-16 | spec-fixit-qemu-artifact-cache (Option C decision) | `plan/learned/988-kernel-build-consolidation.md` needs an ADDITIVE correction pointer recording that its "the make path must stay Ze-binary-free" rule was deliberately reversed by the user on 2026-07-16 (Option C: `run.py` asks the host `ze-host` binary for the cache key, so Go stays the single source of truth for `kernelCacheVariantFor`, `internal/appliance/cache.go`). Without the pointer, a future agent reads 988, believes the rule still holds, and "restores" it | 988 was explicitly out of scope for the decision-recording pass, and a learned summary should be corrected at the closure of the spec that reversed it, not mid-design. The reversal IS recorded prominently in the spec itself (header banner + at the decision) meanwhile, so nothing is lost if this row is actioned late | `plan/learned/988-kernel-build-consolidation.md` (correction pointer added 2026-07-21 at spec closure: the Ze-binary-free rule was reversed by Option C, now live at `mk/gokrazy.mk`) | done |

