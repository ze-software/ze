# Deferrals: fixit-declared-commands-without-leaves

One issue, recorded not fixed (owner instruction, 2026-08-08). The aggregate
live backlog is folded on read from `plan/deferrals/` by `/ze-status`. Nothing
stores it (`ai/rules/planning.md`).

**Issue:** 30 declared commands whose value placeholder has no YANG leaf

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-08 | ad-hoc (found while repairing trailing-value resolution in `ze <verb>`) | 30 unique declared command paths carry a `<value>` placeholder in their YANG description with NO leaf, so they have no completion and no daemon-side argument validation. `extractArgDefs` (`internal/component/config/yang/command.go`) builds `ArgDefs` from `ze:command` leaves only, and a placeholder in a description reaches neither surface | Resolution is FIXED for all 30: `endsDeclaredCommand` (`cmd/ze/internal/cmdutil/cmdutil.go`) keys the trailing boundary on `cli.AbsoluteVerbPath`, the registrations the daemon dispatcher already uses, so no leaf is needed to reach the daemon. What is missing is the operator's completion and the argument type-check. Declaring the leaves is per-command DESIGN work rather than a sweep: `extractArgDefs` sorts by name and cannot express positional order, so a two-value command cannot be declared correctly today | needs a destination spec | deferred |
