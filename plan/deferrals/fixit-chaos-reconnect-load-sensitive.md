# Deferrals: fixit-chaos-reconnect-load-sensitive

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-03 | spec-fixit-chaos-reconnect-load-sensitive "A sibling instance, same class, different suite" | `test/web/commit-flow.wb` fails under load for the same reason the chaos test did: it asserts on elapsed time instead of on state. It carries `option=timeout:value=45s` and two blind `action=wait:ms=1000` steps. Measured 2026-07-30 at 36.8s under a full run against 14.1s standalone, a slowdown of 2.6 times that the test does not survive | The chaos spec's Files to Modify covers `internal/chaos/` only and no AC of it reaches the web suite, so the sibling was recorded there as guidance and never homed | plan/future/spec-fixit-migrate-sleeps-infra.md | done |
| 2026-08-07 | spec-fixit-migrate-sleeps-infra "Session 2026-08-07 -- test/web/commit-flow.wb converted" | `option=timeout` is inert in every `.wb` test. `parseWBOption` writes `WBTestCase.Timeout` and nothing reads it (two references, both inside `internal/component/web/testing/parser.go`), and `zeTestRunWebTest` applies no wall-clock bound, so all 87 web tests declare a budget that enforces nothing. The only real bounds are `agentTimeout` at 30s per browser command and `expectDeadline` at 15s per retried assertion. Two ways to fix it and the choice is the owner's: wire it with `ParallelTimeoutHeadroom` the way the `.ci` runner does, or delete the directive from the format. Wiring it naively at the declared values would redden the suite, because a full parallel run has taken a single test past 36s | Found while converting `commit-flow.wb`, whose 45s budget the conversion was asked to justify. An inert surface is neither wired nor rejected, so `ai/rules/completion.md` requires one of the two, and neither is a test conversion. Both options change how all 87 `.wb` tests are bounded, which the goal of that conversion does not depend on | plan/future/spec-fixit-migrate-sleeps-infra.md | deferred |

Closed on 2026-08-07 under `plan/future/spec-fixit-migrate-sleeps-infra.md`. Both blind waits are
gone and the 45-second budget is 30 seconds.

The second wait was the one that mattered, and the state it hid was not on the page. Discard
answers with an HX-Redirect (`handleConfigDiscard`, `internal/component/web/handler_config_commit.go`),
so the browser leaves the page and the pending change leaves its DOM whether the server
discarded it or not. The old `expect=html:not-contains=` passed on the page it landed on. It
was proven vacuous: with `EditorManager.Discard` (`internal/component/web/editor.go`) stubbed
to `return nil`, the old test still PASSED. The migrated test polls the server's own readback
(`action=wait-until:path=/config/diff:contains=Review changes (0)`), and against the same
stub it FAILS at line 28. The first wait needed no primitive: a positive `expect=` already
polls the DOM (`retryPositive`, `internal/component/web/testing/expect.go`).

`action=wait-until` is new in `internal/component/web/testing/runner.go` (`Browser.WaitUntil`).
It re-opens a path until the served HTML carries the text, bounded by the same
`expectDeadline` every other browser retry uses. No existing directive reaches server state:
`action=wait` settles on the browser's in-flight counter, and a retried `expect=` re-reads the
DOM the page already holds.

Written on 2026-08-03 by a bookkeeping audit. The spec's metadata row named this file and
the file did not exist.

Two things were checked before the row was written, rather than taken from the spec's prose.
The waits are still there: `test/web/commit-flow.wb` carries the 45-second budget and both
one-second waits at HEAD. And this is NOT the commit-flow failure that
`plan/known-failures/RESOLVED.md` records as resolved on 2026-07-29: that one was positive
expectations sampled once against an asynchronous page, a harness defect in `checkElement`
and `checkHTML`. This failure was measured the day after, and its mechanism is the elapsed
time the test allows itself.
