# Deferrals: spec-plugin-declares-answer-shape

Rows deferred from spec-plugin-declares-answer-shape, which is in progress. Each
row names where the work goes, so nothing is recorded without a destination.

The spec's Failure Routing says what lands here: a defect found in a phase that
does NOT block that phase gets a destination and a row. A defect that blocks the
phase is fixed in it.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-24 | spec-plugin-declares-answer-shape | A daemon-backed source for the published command catalog, so `make ze-command-list` and `ze help command --json` report a plugin's declared shape and column order | Carried forward from `plan/deferrals/plugin-registers-pipe-operations.md`, row 2, 2026-08-22, which recorded the same gap for a plugin's pipe aliases. Both readers load the compiled tree in their own process and start no plugin, so neither can see a declaration that only ever reaches a running daemon. The running daemon answers through completion and `command help`, which is where an operator looks, so the gap costs the published page and costs an operator nothing | a spec of its own, not yet written; the same one that row names | deferred |
