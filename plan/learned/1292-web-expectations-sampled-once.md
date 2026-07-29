# 1292 -- Web expectations sampled the DOM once, so `action=wait` could not save them

## Context

`test/web/commit-flow.wb` (web 20) had a `plan/known-failures/` shard calling it
host-specific and UNOWNED: it passed alone, failed in the full suite, and passed
87/87 on the GitHub runner. It reproduced here deterministically, 2 full-suite
runs out of 2, and `scenario-interface-setup.wb` (62) failed the same way, which
the shard never recorded.

The shard's suggested first move was `ze-test web -a -p 1`, "if it passes serially
the contention is the mechanism". That command cannot answer the question: `-p` is
`--pattern`, and the web suite exposes no parallelism flag at all (`cmd_web.go`
usage: "Run web browser functional tests (.wb files) in parallel"). It silently
runs the 19 tests whose id contains a `1`, still four-way.

The real mechanism was in the harness. `checkElement` and `checkHTML`
(`internal/component/web/testing/expect.go`) fetched one snapshot and asserted
against it. The `action=wait` that precedes them cannot cover an update that has
not begun: `WaitLoad` polls `inflightIdleExpr` (`runner.go`), which is
`no in-flight request AND quiet for 120ms` -- true after a request finishes and
equally true before one starts. `action=click` returns once the click is
dispatched, the htmx POST goes out asynchronously, and a single sample taken in
that window reads the pre-request DOM. The commit bar the test waits for is an
out-of-band swap on the save response (`handler_config_form.go` renders
`oob_save_ok`), so it can only land after that window.

## Decisions

- **Positive expectations poll; negative expectations do not.** `element:id`,
  `element:text` and `html:contains` retry to a deadline and return the LAST
  failure, so the error still carries a current snapshot. `not-id`, `not-text` and
  `not-contains` keep sampling once: an absence is satisfied by the first sample,
  so retrying could only ever turn a real failure into a pass by looking earlier.
  Chosen over making every expectation poll, which is the version that quietly
  weakens the suite.
- **The deadline is sized against the HARNESS, not the page.** Four tests share
  one agent-browser daemon; a single `snapshot` or `eval` round trip costs
  seconds under that load, and a ten-step test that runs in 3s alone takes 20-45s
  in the suite. 5s bought one or two samples and `commit-flow.wb` still lost. 15s
  gives five to seven, and stays well inside the per-test `option=timeout` (30-60s)
  so a genuinely missing element still fails its step.
- **The shard is deleted, not amended.** It reproduced, so it was never eligible
  to be recorded in the first place (`ai/rules/fix-dont-record.md`); the entry is
  archived in `plan/known-failures/RESOLVED.md` with the mechanism.

## Consequences

- The suite went from 85/87, 85/87, 84/87 (deterministic) to 87/87 three runs
  running, and got FASTER: 166-227s against 299-438s, because failing tests no
  longer burn their full per-test timeout.
- Every `.wb` written from now on can rely on a positive expectation waiting for
  the thing it names. The `action=wait:ms=N` sleeps scattered through the suite
  are now redundant for positives and are the next thing to remove.
- The negatives are still one-shot, which means a `not-contains` immediately after
  an action can still pass before the action lands. `commit-flow.wb` line 23 is
  exactly that shape and is only sound because line 19 proves the text was present
  first. A negative that needs to observe a TRANSITION wants a different directive,
  not a longer sleep.

## Gotchas

- **"Passes alone, fails in the suite" is a diagnosis, not an environment.** The
  shard read it as host-specific because CI was green; CI was green because CI is
  faster, not because CI is different in kind.
- **An accessibility snapshot only contains VISIBLE elements.** The commit bar
  renders with a `visible` class only when the change count is above zero, so a
  bar that exists in the DOM but is hidden looks identical to one that was never
  swapped in. Reading `GetHTML` rather than the snapshot distinguishes them.
- **A shard's own reproduction command must be re-run before it is trusted.**
  This one had never worked, and nobody noticed because nobody ran it.

## Files

- `internal/component/web/testing/expect.go` -- `retryPositive`, applied to the positive element and html checks
- `internal/component/web/testing/expect_retry_test.go` -- retries, gives up at the deadline, no cost on the happy path
- `plan/known-failures/ze-functional-test-web-commit-flow.md` -- deleted
- `plan/known-failures/RESOLVED.md` -- archived with the mechanism
