# Deferrals: spec-plugin-registers-pipe-operations

Rows deferred from spec-plugin-registers-pipe-operations, which is in progress.
Each row names where the work goes, so nothing is recorded without a
destination.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-21 | spec-plugin-registers-pipe-operations | A declaration channel that lets a plugin name a per-command COLUMN ORDER, so `\| display <partial>` over a plugin command offers its field names | `completeDisplayFields` reads the column registry, which only in-tree code can write. The alias this spec adds works without it, and the alias NAME completes, because that comes from the alias registry instead. So the gap costs a plugin command its `display` completion and costs it nothing else. Ordering is a second declaration channel with its own collision and inheritance rules, and folding it into a spec whose subject is aliases would give both halves one set of tests | a spec of its own, not yet written | deferred |
