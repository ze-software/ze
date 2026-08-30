# Deferrals: fixit-declared-commands-without-leaves

One issue, recorded not fixed (owner instruction, 2026-08-08). The aggregate
live backlog is folded on read from `plan/deferrals/` by `/ze-status`. Nothing
stores it (`ai/rules/planning.md`).

**Issue:** 30 declared commands whose value placeholder has no YANG leaf

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-08 | ad-hoc (found while repairing trailing-value resolution in `ze <verb>`) | 30 unique declared command paths carry a `<value>` placeholder in their YANG description with NO leaf, so they have no completion and no daemon-side argument validation. `extractArgDefs` (`internal/component/config/yang/command.go`) builds `ArgDefs` from `ze:command` leaves only, and a placeholder in a description reaches neither surface | Resolution is FIXED for all 30: `endsDeclaredCommand` (`cmd/ze/internal/cmdutil/cmdutil.go`) keys the trailing boundary on `cli.AbsoluteVerbPath`, the registrations the daemon dispatcher already uses, so no leaf is needed to reach the daemon. What is missing is the operator's completion and the argument type-check. Declaring the leaves is per-command DESIGN work rather than a sweep: ~~`extractArgDefs` sorts by name and cannot express positional order, so a two-value command cannot be declared correctly today~~ **CORRECTED 2026-08-30 at the producer: positional order CAN be expressed.** `declaredLeafNames` (`internal/component/config/yang/command.go`) returns the leaves in module declaration order, and `extractArgDefs` consumes that order first, falling back to name order only for a leaf that reaches the entry from a grouping or an augment. So the stated reason for nobody picking this up was false. **Triaged the same day as an improvement, not a release defect:** resolution works for all 30 paths, so nothing is broken; what is missing is completion and an argument type-check, which is coverage and operator convenience | `plan/future/` (needs its own spec) | deferred |
