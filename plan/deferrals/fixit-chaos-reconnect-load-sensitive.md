# Deferrals: fixit-chaos-reconnect-load-sensitive

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-03 | spec-fixit-chaos-reconnect-load-sensitive "A sibling instance, same class, different suite" | `test/web/commit-flow.wb` fails under load for the same reason the chaos test did: it asserts on elapsed time instead of on state. It carries `option=timeout:value=45s` and two blind `action=wait:ms=1000` steps. Measured 2026-07-30 at 36.8s under a full run against 14.1s standalone, a slowdown of 2.6 times that the test does not survive | The chaos spec's Files to Modify covers `internal/chaos/` only and no AC of it reaches the web suite, so the sibling was recorded there as guidance and never homed | plan/spec-fixit-migrate-sleeps-infra.md | deferred |

Written on 2026-08-03 by a bookkeeping audit. The spec's metadata row named this file and
the file did not exist.

Two things were checked before the row was written, rather than taken from the spec's prose.
The waits are still there: `test/web/commit-flow.wb` carries the 45-second budget and both
one-second waits at HEAD. And this is NOT the commit-flow failure that
`plan/known-failures/RESOLVED.md` records as resolved on 2026-07-29: that one was positive
expectations sampled once against an asynchronous page, a harness defect in `checkElement`
and `checkHTML`. This failure was measured the day after, and its mechanism is the elapsed
time the test allows itself.
