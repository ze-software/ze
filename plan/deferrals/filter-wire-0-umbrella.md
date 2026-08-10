# Deferrals: filter-wire-0-umbrella

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-15 | rib-arch-2 (Option A DONE, learned 1127) | Remove the text `Update` filter-IPC carrier entirely: rewrite the 7 text-parsing `filter_*` plugins to wire-decode attributes + rework the modify-delta apply (`applyFilterDelta`/`textDeltaToModOps`). The `.Raw` carrier is now binary `[]byte` (Option A, shipped); this is the remaining full text-path removal. | Ambiguous net perf (engine-side format saved, plugin-side wire-decode added) + high blast radius on the BGP filter control path; user chose Option A over the full rewrite. 2026-07-16: user elected to proceed with the full spec-driven redesign (wire-level filter-chain composition medium replacing the text input/composition/output path); to be scoped via `/ze-spec`. | `plan/spec-filter-wire-0-umbrella.md` (scoped 2026-07-16 via /ze-spec; its Goal is exactly this removal and it names all 7 plugins. Supersedes the interim destination, the rib-arch-2 filter-raw-bytes record, which recorded the shipped Option A and was a summary, not a tracker) | in-progress (spec) |

