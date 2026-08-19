# Deferrals: cli-order-pipe

Rows deferred from spec-cli-order-pipe, which closed on 2026-08-19 and is no
longer in the tree. Each row names where the work goes, so nothing is recorded
without a destination.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-19 | spec-cli-order-pipe | Row ordering: an operator that sorts the ROWS by a field's value, as distinct from `display` and `fill`, which order and select the COLUMNS | Ze has no row-ordering operator at all today; `knownPipeOps` holds fourteen names and none of them sorts rows. It is a separate operation over a different axis, so it needs its own name and its own semantics rather than being folded into `display` | `plan/future/spec-cli-pipe-column-modifiers.md` | deferred |
| 2026-08-19 | spec-cli-order-pipe | Exclusion syntax, so an operator can say "every column except these two" rather than naming the other seventeen | Judged the modifier worth adding next, ahead of any positional wildcard variant. Left out here to keep the first version of the operator to the syntax the owner specified | `plan/future/spec-cli-pipe-column-modifiers.md` | deferred |
