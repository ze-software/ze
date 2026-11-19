# Handover 48: l2tp session identity, land the l2tp chunk, then finish and close

Spec: `plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md` (in-progress; the
claim is held by session a0dbf7da, which hands off here: claim it in the new session with
`./le spec claim plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md`).
Owner instruction for this work: "complete it fully".
Per-spec state: `tmp/session/2026-11-19-133437/state/session-state-l2tp-session-identity-and-publish-stall-a0dbf7da-0df3-4dcf-b508-ccd3962ff523.md`.

## Landed (2026-11-19)

| Commit | What |
|--------|------|
| 93ff938cdb | AC-4: RADIUS Accounting-Stop sent from the session's own accounting task; original endpoint across reload; shutdown joins owed Stops in 10 s; `doctor-radius-accounting-unacknowledged` |
| 2da7aca1df, c726eab7be | Slow-accounting fixture (LAC CDN on the reactor goroutine) and its decision unit tests |
| 668a254aec | AC-8 orchestrator: withdrawal at the consumer instance that accepted the route, released only on confirmation |
| 54fce2e657 | AC-8 reactor: a queued accepted original is marked retired by its withdrawal and dropped at the drain; accepted withdrawals take their original's rail |
| 8faa08025f, 13cc4a94e5 | Spec records |

AC-4 guest proof: the draft `test/draft/l2tp/slow-accounting-control.ci` passed in Ze's guest
on the claude server at 27b60e5401 and went red with `<-sess.finished` after `sess.end` in
`onSessionDown` ("healthy HELLO lacked its ACK before the LAC retry").

## Uncommitted, verified, to land first

All tests below passed on the Mac under the race detector; the l2tp, l2tp/cmd, cos and
authradius packages pass. An independent ze-test-judge approved and recorded (commit session
8c1d834f approval file) the six migrated l2tp tests and removal of the dead fake
`TeardownSession` methods.

1. AC-1: `pppSessionLocked` takes the event name; one `refusePPPEvent` Debug line per refused
   stale PPP event (internal/component/l2tp/reactor_kernel.go). Test
   `TestLatePPPEventIsRefusedOnAReusedSessionID` (late_ppp_event_test.go).
2. AC-2: `TestRouteObserverKeepsSameTupleOnTwoListenersApart` (route_identity_test.go).
3. AC-3: `TestSessionTimersCannotEndReplacement` (session_timer_replacement_test.go); the
   tuple-only `teardownSessionByID` / `Subsystem.TeardownSession` / `Service.TeardownSession`
   deleted (teardown.go, subsystem_snapshot.go, service_locator.go); `CaptureSession`
   not-found text names `local-sid=N` (session_capture.go); migrated tests in
   teardown_cause_test.go, session_event_order_test.go, session_route_withdrawal_test.go.
4. Journal rows: `handleTick` enqueues recovered kernel setups, and `handle`/`handleTick`
   deliver owed session downs before tunnel events (internal/component/l2tp/reactor.go).
   Tests `TestTunnelCallbackRunsAfterOwedSessionDowns` (passes) and
   `TestTickEnqueuesRecoveredKernelSetup` (reactor_tick_kernel_setup_linux_test.go, Linux
   only: run it in the guest on the claude server).
5. Docs: ai/digests/subscriber.md, docs/guide/l2tp.md, docs/architecture/l2tp/bng-1-radius-attributes.md,
   docs/architecture/l2tp/subscriber-session-model.md, docs/research/l2tpv2-ze-integration.md,
   features/l2tpv2-bng.md, two comments in internal/test/fixture/tunnel_fixture_l2tp_session_clear.go.

Shared files, commit only this work's hunks (`./le commit create ... hunks <path> <patch>`):
- internal/component/l2tp/reactor.go: hunks 3, 4 and 6 of its diff are ours (the handle
  order move and handleTick); hunks 1, 2 and 5 are another session's unfinished hidden-AVP
  edit (`revealSCCRQ`, `hiddenLogLastSent`), which needs its other uncommitted files.
  Prepared patch (rebuild it if the file moved since 2026-11-19 17:30):
  `tmp/session/2026-11-19-133437/scratch/l2tp-reactor-ours.patch`.
- internal/component/l2tp/events/events.go: hunk 1 (TunnelUp/TunnelDown reader note) is
  ours; hunk 2 (`FramedRoute.Gateway`) is another session's.
- docs/guide/l2tp.md carries another session's hidden-AVP and clear-form paragraphs.
Before committing, `git diff` every file in the list above and confirm no other foreign
hunks (another session is active in internal/component/l2tp/).

Then remove the now-dead fake `TeardownSession` methods (approved: cos, web, shaper, pool,
l2tp, cmd in session_teardown_test.go and l2tp_test.go, authradius in four files) and decide
the dead fields they leave (web teardownSID/teardownErr, cmd teardownSessionErr,
legacyClears zero assertions, teardownAttempts): their removal is a further test change for
a judge. The judge also found TestRFC5176DisconnectLocatedButServiceStoppedIsNotRemovable
asserts `teardownAttempts`, which only the legacy method counted: strengthen it to
`svc.fakeL2TPService.teardowns.Load() == 0` (judge first). Fix the comment in
test/l2tp/session-clear-leaves-no-subscriber.ci that still names teardownSessionByID.

## Remaining plan, in order

1. Land the chunk above; then `./le doc check all` and `./le feature check`.
2. Spec bookkeeping (via an agent: the spec gate needs the doors and ~24 cited sources read):
   correct the false claims an audit found (AC-10 "source repair pending", Interop "red",
   accounting rows superseded by 93ff938cdb, four nonexistent Wiring test names); add the
   Goal Validation table; answer the Architectural Verification registration rows; mark the
   seven owned journal rows with their fix commits.
3. AC-4: fixture lint (refused four times by concurrent edits), 80-iteration stress of the
   draft on the claude server, promote out of test/draft.
4. AC-8: native run, stress and promotion of test/draft/l2tp/radius-framed-route-overflow.ci.
5. Selector draft test/draft/l2tp/session-selector-scope.ci: run, stress, promote.
6. Claude server: the six CQM Linux guest causal checks; `TestTickEnqueuesRecoveredKernelSetup`;
   the unchanged `l2tp-many-sessions-kl2tpd` lab (AC-9) with 93ff938cdb, 668a254aec and
   54fce2e657 in; the interop rerun.
7. Absorb or close spec-radius-acct-start-after-stop against 93ff938cdb (since closed: met by 93ff938cdb and 938cf958c1, its 2026-11-12 journal row fixed).
8. Scoped lint for l2tp root, cmd, cos, web, metrics; review gate; closure (commit A, commit B).

## Environment

- Guest tests run on the claude server only (owner instruction). Route: bundle
  `<server origin/main>..HEAD`, `git worktree add --detach .claude/worktrees/<name> <sha>`,
  scp the test/draft file in, run with
  `PATH=/usr/local/go/bin:$HOME/go/bin:$PATH GOPATH=$HOME/go GOMODCACHE=$HOME/Code/github.com/ze-software/ze/main/gokrazy/modcache`
  (read-only use of the main checkout's module cache, owner-approved); a fresh worktree's
  own gokrazy/modcache is empty and GOPROXY=off. A causal break is applied only inside that
  worktree (owner-approved), then the worktree and bundle are removed.
- Known reds not from this work: reactor `TestAIGPReplayKeepsSameAddressOriginals/native-rs`
  (its harness from ba0f3a3a7d never flushes the direct route-server write); web commit and
  discard tests ("binary built without the BGP engine"); fixture
  `TestToolFixturesReachTheirProducer`, `TestRulesFixtureRootAliasesReachOnlyOwnedTree`,
  `TestPluginObserversWaitForTheScriptedPeer`; doc check `forward_pool.go` anchor
  `requeuePolicyReplay`.
- The commit hook reads `rm` on the same line as a test path as a test deletion; split it.
