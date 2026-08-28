# Deferrals: spec-plugin-registers-pipe-operations

Rows deferred from spec-plugin-registers-pipe-operations, which is in progress.
Each row names where the work goes, so nothing is recorded without a
destination.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-21 | spec-plugin-registers-pipe-operations | A declaration channel that lets a plugin name a per-command COLUMN ORDER, so `\| display <partial>` over a plugin command offers its field names | `completeDisplayFields` reads the column registry, which only in-tree code can write. The alias this spec adds works without it, and the alias NAME completes, because that comes from the alias registry instead. So the gap costs a plugin command its `display` completion and costs it nothing else. Ordering is a second declaration channel with its own collision and inheritance rules, and folding it into a spec whose subject is aliases would give both halves one set of tests | a spec of its own, not yet written | deferred |
| 2026-08-22 | spec-plugin-registers-pipe-operations | A daemon-backed source for the published command catalog, so `./le command-list` and `ze help command --json` report a plugin's pipe aliases | Both read the compiled tree in their own process and start no plugin, so neither can see a declaration that only ever reaches a running daemon. The wiki catalog therefore lists a plugin's commands without its aliases. The running daemon answers through completion and `command help`, which is where an operator looks, so the gap costs the published page and costs an operator nothing | a spec of its own, not yet written | deferred |
