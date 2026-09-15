# 022 - A row that asserts nothing derives its state

**Spec:** spec-rfc-ledger-rollup-annotation, closed 2026-09-15
**Class:** `plan/journal/gate-excludes-part-of-its-population.md`

## What the work built

A sixth annotation kind, `{rollup: <targets>; why}`, for a requirement that is
true exactly when the rows it names are true. `parseRollup` (`summary.go`)
holds each target to the id or stem form, `checkRollupTargets`
(`check_core.go`) holds every target against the corpus and refuses the row
itself and a cycle, and `rollupDeriver.fill` writes `Requirement.Derived` and
`DerivedCause` from the state `evaluate` already computed: met, gap, or
unproven, with the first deciding target named. `CoverageRows` leaves the row
out of every count, the ledger prints the derived state after the reason, and
the site shows it in its own bucket outside every partition sum.
`RFC4302-5-1` derives gap from `RFC4302-2.5-5`; `RFC4302-5-2` derives met.

## Decisions

**A rollup raises no finding of its own (owner, 2026-09-15).** The spec first
said a derived gap or unproven rollup raises one finding naming its cause.
That finding was a second report of one fact: the `{gap}` target is disclosed
by the Remaining cell under its own id, and the unproven target is reported by
the "has no test and no annotation" arm. The cause lives on the row instead.

**Outside the denominator, not inside it and outside the numerator.** Every
other kind asserts something about its own row, so its obligation is Ze's and
it stays gated. A rollup names rows already counted once, so counting it would
bill a document twice and hold a fully conformant RFC one row short of 100%.

**A stem target names a whole enrolled summary.** `RFC4302-5-1` binds all of
RFC 4301; a list of that summary's ids would go stale on its first new row.

## Trap for the next session

The fill covered the population the GATE reads (enrolled rows) while the
renderer prints every summary, and `DerivedMark` panics on a rollup nobody
derived. Nothing red said so, because every test and the corpus held enrolled
rollups only. When a producer fills a field a consumer panics without, the
producer's population is the consumer's, not the gate's.
