# spec-fixit-redistribute-establishment-stall

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-fixit-migrate-sleeps-infra (P0 carve-out); spec-redistribute-late-join-replay |
| Phase | 1/1 (root cause found 2026-07-16; fix proposed, not implemented) |
| Updated | 2026-07-16 |

## ROOT CAUSE (2026-07-16) -- there is no engine stall; two test-harness defects

-> Decision (2026-07-16, evidence below): **the premise of this spec is false.** An
external observer's engine RPCs do NOT stall BGP establishment. In the affected tests the
session never establishes because **ze-peer exits before it ever binds a listening
socket**, so every ze dial hits `connection refused` and backs off 5->10->20->40s exactly
as designed. The blind `time.sleep` versions pass because a second harness defect makes
the runner skip every BGP-level assertion. The deterministic conversion did not break
these tests: it was the first code to ever LOOK at peer state, and it found the test had
been green-but-vacuous since it was written. Nothing in `internal/component/bgp/` or the
plugin engine is implicated. Both defects live in `internal/test/`.

### D1 -- a `expect=json`-only peer block makes ze-peer exit without listening

| # | Hop | Cite |
|---|-----|------|
| 1 | `LoadExpectFile` passes ONLY `expect=bgp:` / `action=` lines to ze-peer; `expect=json` is dropped ("Ignore json, stderr, syslog - handled by test runner") | `internal/test/peer/expect.go:60-65` |
| 2 | a check-mode peer with `len(config.Expect) == 0` prints `no test data available to test against` and returns 1 | `internal/test/cli/cmd_peer.go:44-48` |
| 3 | so `Run` is never reached: the listener at `lc.Listen` never binds and never prints `listening on` | `internal/test/peer/peer.go:211,215,217` |
| 4 | the runner skips its peer-bind barrier for exit-code tests, so ze starts against a dead port and no error is raised | `internal/test/runner/runner_exec.go:766` (`if rec.ExpectExitCode == nil`), `:770` |

`bgp-redistribute-announce.ci`'s peer block carries exactly one expectation, an
`expect=json` (`:18`). Its ze-peer therefore never listens.

### D2 -- `expect=exit:code=N` silently disables ALL BGP validation in a peer test

`internal/test/runner/runner_exec.go:1116` computes `selfValidated` as
`rec.ExpectExitCode != nil || (!hasPeer && hasOutputAssertion)`, and the whole BGP peer
path runs only under `if !selfValidated`. So a non-nil `ExpectExitCode` alone skips
`:1121` (the peer `successful` check) and `:1142` (`validateJSON`). All 7
`bgp-redistribute-*.ci` declare
`expect=exit:code=0`, so the peer's wire expectations and `validateJSON`
(`runner_validate.go:34`, whose `:114-116` no-match error is real and would have fired)
are never evaluated. The test passes on ze's exit code alone.

-> Constraint: D2 MASKS D1. The comment at `runner_exec.go:1104-1106` already worries
about this exact class ("a peer test may also declare file checks, and it must still fail
when the BGP exchange mismatches rather than passing on the file checks alone"); the
`rec.ExpectExitCode != nil` disjunct at `:1116` reintroduces it for exit-code tests.

### Evidence (all reproduced on this darwin host, scratch tree, `bin/ze` zetest build)

| # | Experiment | Result |
|---|-----------|--------|
| E1 | `ze-test peer --port P <announce peer block>` | `no test data available to test against`, exit 1, **never listens** |
| E2 | `ze-test peer --port P <as112 peer block>` | `listening on 127.0.0.1:P` -- binds and waits |
| E3 | committed `bgp-redistribute-announce.ci` | PASS 5.2s |
| E4 | same, `expect=json` NLRI changed to `203.0.113.77/32` (never announced) | **PASS** -- assertion is vacuous |
| E5 | observer converted to `established`->`eor-sent`->`updates-sent` | FAIL 8/8, 45s, `timeout waiting for peer established`, PEER OUTPUT empty |
| E6 | E5 **plus one `expect=bgp:conn=1:seq=1:contains=4003047F000001`** so the peer listens | **PASS 5/5, ~1.2s** |
| E7 | E6 with the exit-code directive removed and the impossible NLRI | FAIL -- `no received message with NLRI` (validation now bites) |
| E8 | E6 with the exit-code directive removed, correct NLRI | FAIL: `message 101: JSON mismatch: added: local-preference = 100` |
| E9 | E8 with `"local-preference":100` added to the expectation | **PASS 6/6, ~1.3s**, every assertion enforced |

-> Constraint (E8, independent defect): the committed `expect=json` in
`bgp-redistribute-announce.ci` does not match the UPDATE ze actually sends. The real
UPDATE is `...400101024002004003047F00000140050400000064200A000001` = origin incomplete,
NEXT_HOP 127.0.0.1, **LOCAL_PREF 100**, NLRI 10.0.0.1/32. The expectation omits
`local-preference`. It has never matched reality; D2 is why nobody found out. The
redistribute FEATURE is correct; only its assertion is wrong.

### Why 1-peer and 2-peer differ (the asymmetry that drove the old theory)

-> Decision: peer COUNT is irrelevant, and so is explicit-vs-auto orchestrator (the
2026-07-14 bisection was right to exclude both, then attributed the residue to the wrong
cause). The differentiator is which directives the `.ci` uses:

| Test | `expect=bgp:` in peer block? | `expect=exit:code=0`? | Consequence |
|------|------------------------------|------------------------|-------------|
| `redistribute-as112-announce.ci` (2-peer, "unaffected") | YES (`:29-30,43-44`) | NO | peer listens; runner enforces the exchange -> a REAL test |
| `bgp-redistribute-announce.ci` (1-peer, "stalls") | NO -- json only (`:18`) | YES (`:126`) | peer never listens; nothing enforced -> a VACUOUS test |

as112 was never "immune to a concurrency bug"; it is simply the only one of the pair that
was ever actually running BGP.

### Production reachability: NO (positive evidence, not absence of evidence)

-> Decision: this cannot happen in production, and the spec's outcome (b) now HAS the
positive evidence Thomas required rather than resting on "we found no bug".
1. Both defects are in `internal/test/peer/`, `internal/test/cli/`, `internal/test/runner/`.
   No production file is involved. A real daemon has no `expect=json` and no ze-peer.
2. E6/E9 are the direct positive proof of the production-shaped scenario: an observer
   plugin polling `show bgp peer <n> detail` over `ze-plugin-engine:dispatch-command` in a
   tight loop THROUGHOUT the connect/establish window, single peer, redistribute config.
   The session establishes and the test passes in ~1.2s, deterministically, 5/5 and 6/6.
   A monitoring plugin polling during convergence does NOT stall peering.
3. It is also FASTER than the blind-sleep original (1.2-1.3s vs 5.2s): the 4s of sleep was
   hiding the fact that establishment takes ~1s once a listener exists.
4. Corroborates the 2026-07-15 dump already recorded below: 122 goroutines, zero
   `[semacquire]`. There was never a blocked goroutine because there was never a stall.

-> Constraint: the `connection refused` + 5->10->20->40s backoff in the 2026-07-15 log is
the reactor behaving CORRECTLY against a port with no listener. `DefaultReconnectMin`
(`peer.go:81`) and commit 44ad25d23 are unrelated to this symptom and neither caused nor
masked it.

### Firewall head-of-line-blocking link: REFUTED

-> Decision: `plan/spec-fixit-firewall-concurrency-deadlock.md` and this spec share NO
root cause. That spec's mechanism is a real lock-discipline chain in production plugin
code: ddos-local holds `r.mu` (`internal/plugins/ddos/local/responder.go:64,136`) across
an unbounded netlink `Flush` (`internal/plugins/firewall/nft/backend_linux.go:74` ->
`nftables conn.go:266-274`, no deadline set at `backend_linux.go:31`), while
`handleShowDdosLocal` (`show.go:31` -> `responder.go:199`) needs the same lock on the
dispatch path. This spec's mechanism involves no lock, no dispatch handler, no shared
goroutine, and no production code at all: a test peer exits before binding. The
resemblance was only the surface shape ("an observer's command does not get what it wants
during a long operation"). Both specs must proceed independently; neither unblocks the
other.

## F1-F3 IMPLEMENTED 2026-07-16 (F4 open); measured blast radius below

-> Decision: F1, F2 and F3 are implemented and green (`make ze` OK,
`make ze-lint-changed` 0 issues). No production file was touched. F4 (fixing the
`.ci` tests) is deliberately NOT done here and is fanned out; see
`plan/deferrals.md` (5 rows, 2026-07-16) for the sharded work.

| # | Landed | Where | Proof |
|---|--------|-------|-------|
| F1 | `isSelfValidated(rec, hasCheckPeer)`: a check-mode ze-peer ALWAYS governs; an exit-code assertion is additive | `internal/test/runner/peer_contract.go`, called at `runner_exec.go` (replaces the `:1116` expression) | `TestIsSelfValidated` -- the `peer plus exit code` row FAILS against the old expression (verified by reverting it: `expected: false, actual: true`) and passes now |
| F2 | `validatePeerBlocks`: parse-time rejection of a check-mode peer block with no ze-peer-consumed directive, naming file + remedy | `internal/test/runner/peer_contract.go`, called from `record_parse.go` parseAndAdd | `TestParseAndAdd_CheckPeerWithOnlyJSONExpectRejected` (+ no-expect-at-all, and 3 accept-side controls incl. sink mode) |
| F3 | Bind barrier no longer skipped for exit-code tests; a non-binding peer is `FailTypePeerNeverBound` with a message naming the "no test data" cause | `runner_exec.go` (the `:766` guard), `peerBindFailure`, `record.go` | `TestPeerBindFailure_NamesNoTestDataCause` |

-> Decision (drift control): the ze-peer-consumed set now has ONE definition,
`peer.ConsumesLine` (`internal/test/peer/expect.go`), used by BOTH `LoadExpectFile`
and F2's guard. A copy in the runner would have re-opened the same defect.
`TestLoadExpectFileMatchesConsumesLine` pins the agreement.

-> Constraint (found while measuring, corrects F1 as originally proposed): keying
the peer-governs rule off `hasPeer` (ze-peer present) is WRONG. sink/echo/inject
peers loop until killed and never print `successful` (`peer.go:265-267` continues
without checking completion), so requiring it of them asserts nothing and fails
valid scaffolding tests -- `event-predicate-wait` (`--mode echo`) went red until the
rule was scoped to `hasCheckPeer`. Only ModeCheck validates, exactly as
`cmd_peer.go` guards only ModeCheck.

### Measured blast radius (real parser + baseline A/B, not an estimate)

Baseline = `ze-test` built from HEAD sources (`git archive`), run from the same
tree, so ONLY the harness differs. `test/plugin`: baseline 2 reds (both flaky, they
pass on re-run), fixed 69 deterministic reds (red in both of two `-p 3` runs).
Confined to `test/plugin`: `bgp encode` (55 real BGP tests) and `vrrp` stay green;
`flow-export` (8) and `bgp reload` (1) fail IDENTICALLY at baseline (pre-existing,
not ours).

| Class | Count | Read |
|-------|-------|------|
| F2 parse-rejected, peer block json-only | 9 | (b) correct test, wrong assertion. The `expect=json` never ran AND does not match reality (E8) |
| F2 parse-rejected, peer block declares nothing | 21 | (b) peer is scaffolding mis-declared as check mode. `rib-graph.ci`'s block SAYS SO: `# No wire expectations from ze-peer (routes injected via API)`. Fix = `--mode sink` |
| F1 runtime red, peer declares ONLY the ipv4/unicast EOR | 33 | (b) EOR unreachable by construction: the `.run` plugin calls `request shutdown` at post-startup, so ze closes during the OPEN handshake. `api-rib-inject` peer log: `open recv`/`open sent` -> `connection closed before completion`, while the run-script prints `OK: rib inject + show verified` |
| F1 runtime red, peer validates REAL wire content and MISMATCHED | 6 | Possible PRODUCT BUGS, highest value. `bgp-rs-fastpath-ebgp-shared` expected an announce of 10.0.0.0/24, peer received a WITHDRAW (exclude its failing external IRR lookup first). Also `remove-private-as-{export,replace-peer}`, `api-route-refresh`, `rib-pipe-filter`, `test-pipe-first-last` |
| Load-sensitive, NOT ours | 7 | (c) flip red/green between runs; pass in isolation. Do not "fix" |

-> Constraint: the sibling scan's "23 blocks / 21 files" UNDERCOUNTED. The real
parser rejects 30 files; it missed `bgp-gtsm-reject`, `bmp-lg-{bestpath-isolation,
disconnect,ingest}`, `bmp-receiver-{messages,session}`, `family-no-plugin-failure`,
`show-bmp-sessions`, `show-rr-status`. Vacuity is now proven for far more than
`bgp-redistribute-announce`: 30 peers provably never bound (parse-time, by the
guard that mirrors ze-peer's own bail) and 30 more declared an unreachable EOR.

-> Constraint (for F4): do NOT close a red by editing its expectation to match
observed behavior, and do NOT add `expect=exit` to silence a peer check. The
EOR-only class has two honest fixes -- `--mode sink` (peer is scaffolding) or make
the test reach Established before shutdown (the stronger test). Pick per file with
the test's VALIDATES line in hand.

### The exact 69: every newly-red test, by class (all in `test/plugin`)

Produced by the real parser + two `-p 3` runs intersected (red in BOTH), A/B'd
against a HEAD-built baseline. Not an estimate.

**Class A1 -- F2 parse-rejected, peer block json-only (9).** (b) correct test,
wrong assertion: the `expect=json` never ran (D2) AND does not match the real UPDATE
(missing `local-preference`, E8). Fix: add a peer-consumed expectation, drop
`expect=exit:code=0`, correct the JSON.
   1. `bgp-redistribute-announce.ci`
   2. `bgp-redistribute-burst.ci`
   3. `bgp-redistribute-explicit-nhop.ci`
   4. `bgp-redistribute-nexthop-self.ci`
   5. `bgp-redistribute-withdraw.ci`
   6. `forward-mpreach-nexthop-self-two-peer.ci`
   7. `redistribute-l2tp-announce.ci`
   8. `redistribute-l2tp-multi-peer-nexthop.ci`
   9. `redistribute-l2tp-withdraw.ci`

**Class A2 -- F2 parse-rejected, peer block declares nothing (21).** (b) peer is
scaffolding mis-declared as check mode; the `.run`/stderr assertions are real and
never needed it (they passed at baseline with a peer that never bound). Fix:
`--mode sink`.
   1. `bgp-gtsm-reject.ci`
   2. `bmp-lg-bestpath-isolation.ci`
   3. `bmp-lg-disconnect.ci`
   4. `bmp-lg-ingest.ci`
   5. `bmp-receiver-messages.ci`
   6. `bmp-receiver-session.ci`
   7. `family-no-plugin-failure.ci`
   8. `policy-chain-plain-names.ci`
   9. `policy-test-as4path-suppress.ci`
  10. `policy-test-configured-export.ci`
  11. `policy-test-configured-import.ci`
  12. `policy-test-errors.ci`
  13. `policy-test-reject-bad-hex.ci`
  14. `policy-test-remove-private-as.ci`
  15. `rest-peer-set-delete-lifecycle.ci`
  16. `rib-best-selection.ci`
  17. `rib-graph.ci`
  18. `rib-graph-best.ci`
  19. `rib-graph-filtered.ci`
  20. `show-bmp-sessions.ci`
  21. `show-rr-status.ci`

**Class B1 -- F1 runtime, peer declares ONLY the ipv4/unicast EOR (33).** (b) the EOR
is unreachable by construction: the `.run` plugin calls `request shutdown` at
post-startup, so ze closes during the OPEN handshake. Fix per file: `--mode sink`,
or reach Established before shutdown (stronger).
   1. `adj-rib-in-query.ci`
   2. `adj-rib-in-replay-on-peerup.ci`
   3. `api-commit-lifecycle.ci`
   4. `api-commit-workflow.ci`
   5. `api-rib-clear-in.ci`
   6. `api-rib-clear-out.ci`
   7. `api-rib-inject.ci`
   8. `api-rib-show-in.ci`
   9. `api-rib-show-out.ci`
  10. `api-rib-withdraw.ci`
  11. `bestpath-reason.ci`
  12. `bgp-rs-asn4-transcode.ci`
  13. `bgp-summary-route-counts.ci`
  14. `cli-grammar-action-first.ci`
  15. `config-adj-rib.ci`
  16. `cursor-replay.ci`
  17. `dispatch-command-single-decode.ci`
  18. `forward-overflow-two-tier.ci`
  19. `gr-cli-restart.ci`
  20. `lg-csv-download.ci`
  21. `multipath-basic.ci`
  22. `nexthop-self.ci`
  23. `nexthop-self-ipv6-forward.ci`
  24. `nexthop-unchanged.ci`
  25. `rib-clear-out-family.ci`
  26. `rib-forward-handle-observed.ci`
  27. `rib-inject-rfc5549.ci`
  28. `rib-show-filter.ci`
  29. `rpf-multicast.ci`
  30. `rpki-cache-connect.ci`
  31. `rr-basic.ci`
  32. `rr-ipv6-config.ci`
  33. `show-l2tp-sessions.ci`

**Class B2 -- F1 runtime, peer validates REAL wire content and MISMATCHED (6).**
POSSIBLE PRODUCT BUGS. The only reds where a substantive BGP assertion is enforced
for the first time. Do NOT edit the expectation to match observed behavior.
   1. `api-route-refresh.ci`
   2. `bgp-rs-fastpath-ebgp-shared.ci`
   3. `remove-private-as-export.ci`
   4. `remove-private-as-replace-peer.ci`
   5. `rib-pipe-filter.ci`
   6. `test-pipe-first-last.ci`

-> Constraint: `bgp-rs-fastpath-ebgp-shared` expected an UPDATE announcing
10.0.0.0/24 (ORIGIN IGP, AS_PATH [65000], NEXT_HOP 1.1.1.1, LOCAL_PREF 100) and the
peer received a WITHDRAW of 10.0.0.0/24. Its client log also shows a failing
external IRR lookup (`irr: lookup failed ... read tcp`), so RULE OUT the network
dependency before concluding a route-server fastpath bug.

### Proposed fix (F1-F3 now implemented, see above; F4 open)

| # | Change | File | Why |
|---|--------|------|-----|
| F1 | Fail loudly instead of passing: a record with a ze-peer AND `ExpectExitCode` must still run the peer `successful` check + `validateJSON`. Narrow `selfValidated` to `!hasPeer && (rec.ExpectExitCode != nil \|\| hasOutputAssertion)` | `internal/test/runner/runner_exec.go:1116` | removes D2, the masking defect; makes every affected test honest. Expect reds: that is the point |
| F2 | Reject a check-mode peer block whose expectations are all runner-side, at PARSE time, naming the file | `internal/test/runner/record_parse.go` (peer-block validation) | removes D1's silent mode; `expect.go:60-65` dropping `expect=json` is correct behavior, the bug is that nothing notices the peer is left with nothing to do |
| F3 | Make ze-peer's "nothing to check" exit unmistakable in the runner's report (it is currently only visible as the peer's stderr in a failure dump) | `internal/test/cli/cmd_peer.go:46` + runner report | a peer that never binds should never look like a passing test |
| F4 | Then fix the 7 `bgp-redistribute-*.ci`: add a ze-peer-consumed expectation, drop `expect=exit:code=0`, correct the JSON to include `local-preference`, and convert the sleeps (recipe proven at E9) | `test/plugin/bgp-redistribute-*.ci` | AC-3/AC-4; converts the bucket AND makes it assert for the first time |

-> Constraint: F1 before F4. Fixing the tests first would leave the harness able to hide
the next one. F1 will likely turn other tests red across the 21 files listed below; each
red is a pre-existing false green, not a regression, and must be triaged not silenced
(`ai/rules/no-workarounds-for-missing-behavior.md`).

### Blast radius (verified by scan + spot-checked with `ze-test peer`)

23 check-mode peer blocks in 21 files spawn a ze-peer that exits without listening (D1).
Spot-verified E1-style (all four printed `no test data available to test against`):
`rib-graph.ci`, `policy-test-configured-import.ci`, `redistribute-l2tp-announce.ci`,
`forward-mpreach-nexthop-self-two-peer.ci`.

- json-only (7 files): `bgp-redistribute-{announce,burst,explicit-nhop,nexthop-self,withdraw}.ci`,
  `forward-mpreach-nexthop-self-two-peer.ci` (peer1+peer2), `redistribute-l2tp-{announce,withdraw}.ci`,
  `redistribute-l2tp-multi-peer-nexthop.ci` (peer1+peer2)
- no peer-consumed expectation at all (11 files): `policy-chain-plain-names.ci`,
  `policy-test-{as4path-suppress,configured-export,configured-import,errors,reject-bad-hex,remove-private-as}.ci`,
  `rest-peer-set-delete-lifecycle.ci`, `rib-{best-selection,graph-best,graph-filtered,graph}.ci`
- D2 additionally applies to `bgp-redistribute-{filtered-out,metrics}.ci`: these DO carry
  `expect=bgp:...hex=` (an EOR) so their peers listen, but `expect=exit:code=0` means the
  exchange is still never enforced.

-> Constraint: "the peer never listens" does not automatically mean each of the 21 is
vacuous overall -- some may assert via other surfaces. Vacuity is PROVEN only for
`bgp-redistribute-announce.ci` (E4). The rest need per-file triage under F1.

## Task

> **SUPERSEDED 2026-07-16 by the ROOT CAUSE section above. Kept for history.** The
> premise below ("observer engine activity prevents establishment") is FALSE. The peer
> never establishes because ze-peer exits before binding (D1); the observer is innocent.
> The task is now: fix the two harness defects (F1-F3), then convert the bucket (F4).
> There is no engine stall and no production bug.

Root-cause and fix a reproducible, config-specific BGP establishment stall: when an
external observer plugin issues ANY engine activity (dispatch / quiesce / show-poll,
or even a bare `wait_for_event` callback read) during BGP session establishment, a
**single-peer redistribute session never establishes** (`connections-established`
stays 0). Only the original blind `time.sleep` (observer fully idle, not even reading
its callback connection) lets it establish. This blocks deterministic-wait conversion
of the redistribute test bucket under spec-fixit-migrate-sleeps-infra:
`bgp-redistribute-{announce,burst,explicit-nhop,filtered-out,metrics,nexthop-self,withdraw}.ci`
(7 tests, ~16 sleeps), plus `api-raw.ci` / `api-route-refresh.ci` if they share the trigger.

## Investigation Findings (2026-07-15)

A long investigation this session. Net: **part fixed and committed; a deeper part
diagnosed but not fixed.** Two of my intermediate hypotheses were disproven -- both
recorded here so the next session does not repeat them.

### Fixed + committed
- **`fix(bgp): reconnect backoff floor 5s, not 120s connect-retry` (commit 44ad25d23).**
  `internal/component/bgp/reactor/peer.go` NewPeer(:294) set `reconnectMin :=
  settings.ConnectRetry` (default 120s, RFC 4271 ConnectRetryTimer, `peersettings.go:66`),
  while `reconnectMax = DefaultReconnectMax` (60s). So the backoff floor (120s) exceeded
  its ceiling (60s), contradicting the design in `peer_run.go:24` ("min 5s, max 60s") and
  `DefaultReconnectMin` (5s, peer.go:81). A failed first connect stranded the peer
  `connecting` for 2 minutes. Fix: `reconnectMin := DefaultReconnectMin`. ConnectRetry keeps
  its real role as a connect timeout (`reactor_dynamic.go:343`). Reconnect unit tests use
  `SetReconnectDelay` overrides, so unaffected. Verified: converted `announce` 25/25 (was
  ~80% flaky pre-fix); reactor package unit tests green.

### Regression caused by that fix (RESOLVED 2026-07-16 -- fixed in `runner.go`, see end of section)
44ad25d23 widened `internal/chaos/inprocess` `TestInProcessChaosReconnect`, already logged
flaky-under-`-race` in `plan/known-failures.md` since 2026-07-08, into a failure that also
reproduces WITHOUT `-race` when the test runs in isolation.

Measured with the Makefile's build tags (bare `go test` is not equivalent, see
`plan/known-failures.md` "BEFORE LOGGING ANYTHING HERE"), by patching `peer.go` in place and
restoring it:

| Build | Result |
|-------|--------|
| pre-44ad25d23 backoff (`reconnectMin := settings.ConnectRetry`) | PASS 2/2, ~4.3s |
| HEAD (`reconnectMin := DefaultReconnectMin`) | FAIL, 92.00s, `established==1` (`runner_test.go:688`) |

Order-dependent: a full-package `-race` run of `./internal/chaos/inprocess/` PASSED once, so
`make ze-chaos-unit-test` (`mk/test-chaos.mk:27`, `go test -race ./internal/chaos/...`) may
still be green. The isolation failure is the reliable reproducer.

This is chaos-harness work, NOT a BGP defect: the harness itself documents the intended 5s
backoff (`runner_test.go:246`, `runner.go:518-522` "DefaultReconnectMin = 5s virtual"). The
test was green only because the 120s bug parked the retry loop outside the window.

**RESOLVED 2026-07-16. The open question ("where does the real time go inside `vc.Advance`")
rested on a false premise: none of it is spent there, and the advance loop is not slow.**

A goroutine dump taken 30s into the freeze answered it in one run:

| Goroutine | Where |
|---|---|
| runner | `simWg.Wait()` (`runner.go:594`) -- the advance loop had ALREADY finished |
| ze session | `VirtualClock.Sleep` (`virtualclock.go:49`) from `session.go:767` |

The advance loop costs exactly what `runner.go:427-430` implies (~0.6s real for 60s virtual)
and then EXITS -- after which nothing advances the clock. `session.Run()` polls for its
connection with `s.clock.Sleep(10ms)` (`session.go:762-768`) and `VirtualClock.Sleep` is a
bare `<-ch` (`virtualclock.go:47-50`). `clock.Clock.Sleep` takes no ctx, so `simCancel()`
cannot reach a goroutine parked there -- only `Advance` can. ze's session was stranded
mid-sleep, never completed the handshake, the simulator blocked forever on the reply that
never came (`executeReconnectStorm` -> `readMsg`, `simulator_actions.go:233`), and
`simWg.Wait()` hung until the 90s context tore the sockets down. That is the 92.00s, and
`established==1` because the peer was asleep -- not because reconnect was broken.

44ad25d23 is exonerated as a cause and stays: it only changed WHEN ze lands in that sleep.
Because the advance loop finishes in ~0.6s real, a chaos action firing late in the virtual
window is still mid-handshake when time stops; the 120s floor parked the retry outside the
window and hid a defect that predates it.

Fix (`runner.go`, Run's teardown): keep advancing the virtual clock until both the simulators
and the reactor are down. Real time does not stop while a system shuts down, and neither may
virtual time. Verified 3/3 PASS in 3.70s, matching the 3.69s measured at `8f5f2ff4b`
(pre-regression) vs 92.00s broken; `./internal/chaos/...` green; target test 2/2 green under
`-race`; `make ze-lint-changed` 0 issues. `plan/known-failures.md` entry closed.

Do NOT "fix" this by reverting 44ad25d23: the 120s floor exceeded the 60s ceiling and
contradicts `peer_run.go:19-25`, which documents this loop as deliberately replacing the RFC
4271 ConnectRetryTimer with "min 5s, max 60s".

### CONFIRMED (evidence)
- NOT a mutex deadlock: a 122-goroutine dump has zero `[semacquire]`. Refutes H1-H3 as
  *deadlock* hypotheses.
- `announce` (auto-loads the orchestrator) converts to a deterministic
  `wait_until(state=='established')` + `quiesce()` recipe and passes 25/25.
- The 6 *explicit* `internal redistribute-orchestrator` tests (explicit-nhop, filtered-out,
  withdraw, burst, metrics, nexthop-self) DO NOT pass when converted: the peer stays
  `connecting`; the reactor.peer log shows every dial `connection refused` with a doubling
  backoff (5->10->20->40s), so it never establishes inside the test window.
- The committed **blind-sleep** versions of those 6 PASS with a proper build; my observer-only
  conversion (any dispatch during establishment) is what breaks them. Confirmed via HEAD vs
  converted A/B with the proper `zetest` build.
- Full untruncated startup timeline (manual harness): `StartPeers` fires FAST (~0.26s after
  startup begins), and with a LONG-LIVED peer the session establishes on the 2nd dial (5s
  backoff). So the failure is a TIMING interaction with the ze-test peer's lifetime, not a
  slow StartPeers.

### DISPROVEN (do not repeat)
- "Committed plugin-startup regression (closed pipe)": FALSE. Artifact of running with
  `ZE_TEST_NO_BUILD=1 ZE_BIN=bin/ze` where `bin/ze` was a `make ze` **core** build lacking the
  `zetest` fake plugins (fakeredist/fakefib). With the default `zetest` build (buildZe,
  `cmd_bgp.go:458` uses `runner.TestBuildTags()`), `redistribute-as112-announce` passes.
  Always let ze-test build (do not pin ZE_BIN to a core binary) for the `bgp plugin` suite.
- "~20s StartPeers delay from observer polling": FALSE. That number conflated the ze-test
  `go build` time with startup. The manual full-log capture shows StartPeers is fast.
- "The reactor drops or rejects an inbound connection while the peer's outbound retry loop is
  cycling, so a faster backoff starves establishment": FALSE, and it is NOT a missing feature.
  The accept-while-cycling path exists and is wired: `peer_run.go:126-137` selects the backoff
  on `p.inboundNotify` alongside `p.clock.After(delay)` and restarts `runOnce` immediately
  WITHOUT doubling the delay; `reactor_connection.go:151-163` (`acceptOrReject`) buffers the
  connection via `peer.SetInboundConnection` rather than closing it on `ErrNotConnected` /
  `ErrSessionTearingDown` / `ErrAlreadyConnected` for a passive peer, commented as handling
  "the race where the remote reconnects faster than our session teardown";
  `peer_connection.go:67-80` stores the conn and signals the size-1 notify channel. Do not go
  looking for a missing inbound-accept mechanism in the reactor: read these three first.

### REMAINING (the real open question) -- ANSWERED 2026-07-16, see ROOT CAUSE at the top
~~Why does the ze-test check-mode peer never accept a connection when the observer polls during
establishment, while a long-lived sink peer does?~~ **Answered.** This section asked exactly the
right question and its instinct was correct: *"Every dial is `connection refused`, so the
ze-test peer is not listening at dial time -- pin ... whether the ze-test peer exits early."*
It does exit early, before binding: a check-mode peer whose only expectation is `expect=json`
is left with an empty `config.Expect` (`internal/test/peer/expect.go:60-65`) and bails at
`internal/test/cli/cmd_peer.go:44-48` before `Listen` (`peer.go:211`). The observer's dispatch
does not shift ze's timing at all.

-> Decision: neither proposed remedy is needed. (a) an event-driven observer wait is
unnecessary -- polling `dispatch-command` throughout the connect window is provably harmless
(E6/E9); (b) there is no engine change to make -- a plugin's dispatch during establishment does
not perturb peer connect timing, and the evidence for that is positive, not inferred. The
redistribute tests stay blind-sleep only until F1-F4 land.

## Required Reading

- `internal/component/bgp/plugins/redistribute_egress/register.go` (peer-up state subscription + replay trigger)
- `internal/component/bgp/plugins/redistribute_egress/replay.go` (replay-on-request flow + coordinator)
- `internal/component/bgp/reactor/session_read.go` (`processMessage`, establishment path)
- `internal/component/bgp/reactor/reactor_notify.go` (message counters, `notifyMessageReceiver`)
- `plan/spec-fixit-migrate-sleeps-infra.md` (Mistake Log / Failed Approaches: the bisection)
- `plan/spec-redistribute-late-join-replay.md` (the behavior the fix must not regress)
- `ai/rules/diagnosis-before-fix.md`, `ai/rules/no-fabrication.md`

## Current Behavior

Source files read during investigation:
- [ ] `internal/component/bgp/plugins/redistribute_egress/register.go`
- [ ] `internal/component/bgp/plugins/redistribute_egress/replay.go`
- [ ] `internal/component/bgp/reactor/session_read.go`
- [ ] `internal/component/bgp/reactor/reactor_notify.go`

**Read 2026-07-16 (the files that actually produce the behavior):**
- [ ] `internal/test/peer/expect.go` (:60-65) — `LoadExpectFile` forwards only
  `expect=bgp:` / `action=` to ze-peer; `expect=json` is dropped for the runner.
  -> Constraint: this is CORRECT behavior. The defect is that nothing notices the peer is
  then left with zero expectations.
- [ ] `internal/test/cli/cmd_peer.go` (:44-48) — check mode + empty `config.Expect` prints
  `no test data available to test against` and returns 1, BEFORE `peer.Run`.
  -> Decision: this single early-return is the producer of the entire "stall".
- [ ] `internal/test/peer/peer.go` (:190-286) — `Run` binds at `:211`, prints
  `listening on` at `:217` (the runner's readiness token), `defer ln.Close()` at `:215`;
  in ModeCheck returns as soon as `checker.Completed()` (:276-278).
- [ ] `internal/test/runner/runner_exec.go` (:766) — peer-bind barrier skipped when
  `rec.ExpectExitCode != nil`; (:1116) `selfValidated` skips the peer path incl.
  `validateJSON` (:1142).
- [ ] `internal/test/runner/runner_exec_util.go` (:142,:165,:173) — `syncWriter` waits for
  the literal `listening on`.
- [ ] `internal/test/runner/runner_validate.go` (:34,:114-116) — `validateJSON` DOES fail
  on an unmatched NLRI. It is sound; it simply never runs for these tests.

Behavior to preserve: late-join replay (spec-redistribute-late-join-replay) must keep
working: a peer that establishes AFTER a redistribute injection must still receive the
current redistribute route set via the peer-up->ReplayRequest->targeted-inject path.
`redistribute-as112-announce.ci` (2-peer) must keep passing. Plain-BGP polling tests
(`nexthop-self`) must keep passing. The fix changes only the pathological single-peer +
active-observer stall, not the replay contract.

## Data Flow

### Entry Point
A BGP peer reaches Established; the reactor emits a `state` (down->up) event. In parallel,
an external observer plugin (the `.ci` test's `.run`) may call the plugin engine
(`dispatch-command`, `quiesce`) or read its callback connection (`wait_for_event`).

### Transformation Path
`register.go:83` subscribes the redistribute plugin to `["state"]`. On the down->up edge
(`register.go:92-93`, `OnStructuredEvent`) it calls `coord.onPeerUp(bus, peerAddr)`, which
allocates a monotonic replayID and emits `redistevents.ReplayRequest{replayID}`
(`replay.go:6-11`). Producers re-emit `RouteChangeBatch{ReplayID}`; the coordinator looks
up replayID->peer and injects the current redistribute route set to that ONE peer
(`replay.go:11-14`). So establishment synchronously drives a plugin-facing state dispatch
plus a replay injection back toward the reactor.

### Boundaries Crossed

| From | To | Shared point |
|------|----|--------------|
| Reactor session goroutine (establishment) | plugin engine RPC serialization | plugin-engine command/dispatch lock |
| Redistribute plugin state-event callback | replay coordinator inject | reactor forward pool |
| External observer engine RPC (`dispatch`/`quiesce`/`wait_for_event`) | same plugin-engine serialization | the establishing window |

### Integration Points
- `redistribute_egress` state subscription (`register.go:83`).
- Reactor establishment / forward-pool drain that `quiesce()` waits on.
- The plugin-engine command/dispatch serialization shared by observer and plugins.

## Wiring Test

| Entry Point | Feature Code | Test |
|-------------|--------------|------|
| Single-peer redistribute peer reaches Established while the observer polls the engine (`wait_for_event`/dispatch) during establishment | -> reactor establishment path + redistribute peer-up replay (`register.go:92`, `replay.go` coordinator) | new `.ci` / reactor test asserting the peer establishes; FAILS (stall, `connections-established: 0`) before the fix, PASSES after |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| Reactor establishment completes while a plugin issues an engine RPC during the establishing window | `internal/component/bgp/reactor/*_test.go` (new) | no lock-order / re-entrancy stall on the single-peer establishment path |
| Replay-on-peer-up still fires exactly once after establishment | `internal/component/bgp/plugins/redistribute_egress/replay_test.go` | late-join replay contract preserved (AC-5) |

### Functional Tests

| Test | Validates |
|------|-----------|
| `bgp-redistribute-announce.ci` converted to deterministic waits | establishment + eor-sent + updates-sent poll replaces `time.sleep`; passes 3x + concurrently |

## Files to Modify

Settled by the 2026-07-16 root-cause. **No production file is modified.**

| File | Change | Ref |
|------|--------|-----|
| `internal/test/runner/runner_exec.go` (:1116) | narrow `selfValidated` so a test WITH a ze-peer always runs the peer check + `validateJSON` | F1 |
| `internal/test/runner/record_parse.go` | reject at parse time a check-mode peer block with no ze-peer-consumed expectation | F2 |
| `internal/test/cli/cmd_peer.go` (:46) + runner report | surface "peer had nothing to check / never bound" as a first-class failure | F3 |
| `test/plugin/bgp-redistribute-*.ci` (7) + `api-raw.ci` / `api-route-refresh.ci` | add a peer-consumed expectation, drop `expect=exit:code=0`, correct the JSON (`local-preference`), convert the sleeps | F4 |
| the other 20 files in Blast radius | per-file triage once F1 makes them honest | F1 |
| `test/.ci-sleep-baseline` | ratchet down as F4 lands | — |

-> Decision: `internal/component/bgp/reactor/` and
`internal/component/bgp/plugins/redistribute_egress/` are NOT touched. The original
"exact file determined by AC-2 root-cause" is resolved: AC-2's root cause is in
`internal/test/`, and the reactor behaved correctly throughout.

## Implementation Steps

1. Reproduce deterministically (single-peer redistribute + observer calling
   `wait_for_event` once during establishment). Capture goroutine dumps at the stall.
2. Confirm/refute H1-H3 (below) from the dumps; cite the producing lock/queue `file:line`.
3. Fix at the owning layer per `ai/rules/diagnosis-before-fix.md` (likely: async /
   non-re-entrant peer-up replay dispatch, or decouple `quiesce` drain from
   peer-established state). Never weaken the test.
4. Convert the 7 redistribute tests (+ api-raw/route-refresh) to the proven
   `established -> eor-sent (show bgp summary) -> updates-sent (show bgp peer detail,
   reactor_notify.go:268)` recipe; verify each 3x + concurrently; ratchet the baseline.
5. Confirm no regression in `redistribute-as112-announce.ci` and the replay tests.

### Hypotheses -- ALL THREE REFUTED 2026-07-16 (AC-2 satisfied)
- H1 **REFUTED**: no contention exists. The redistribute peer-up replay
  (`redistribute_egress/register.go:92`, `replay.go`) is never reached in the affected
  tests, because no peer ever establishes to trigger it. E6 proves the observer's
  `dispatch-command` polling runs concurrently with establishment with no ill effect.
- H2 **REFUTED**: no circular wait. `quiesce`/forward-pool drain was never on the critical
  path; the converted test at E9 does not call `quiesce()` at all and passes in ~1.3s.
- H3 **REFUTED**: no `state`-delivery race. The observer never misses a peer-up edge; the
  edge is never emitted, because ze is dialling a port with no listener.
-> Constraint: all three hypotheses assumed a live BGP session. None of them could have
been true, and no goroutine dump would ever have shown them (the 2026-07-15 dump's zero
`[semacquire]` was already telling us this). The keystone fact nobody read was what
ze-peer does with a peer block containing only `expect=json`
(`internal/test/peer/expect.go:60-65` -> `internal/test/cli/cmd_peer.go:44-48`).

## Acceptance Criteria

- AC-1: a regression test reproduces the stall and fails before the fix.
- AC-2: root cause cited to `file:line`, with H1-H3 confirmed/refuted from goroutine evidence.
- AC-3: a single-peer redistribute session establishes while the observer polls the engine.
- AC-4: the 7 `bgp-redistribute-*` tests (+ api-raw/route-refresh if affected) converted off
  `time.sleep`, verified 3x + concurrently, baseline ratcheted.
- AC-5: no regression in `redistribute-as112-announce.ci` or the replay tests.

## Risks & Assumptions

- A-1 (unvalidated): the stall is a re-entrancy/lock-order bug, not a protocol requirement.
  Validate via goroutine dump before any fix.
- R-1: reordering establishment vs. replay dispatch could regress late-join replay; guard
  with spec-redistribute-late-join-replay tests.
- R-2: as of 2026-07-14 a concurrent session's `internal/component/iface` break prevents
  `make ze`; this spec cannot be implemented until the tree builds.

## Checklist

- [ ] Stall reproduced with a failing regression test (AC-1)
- [ ] Root cause cited `file:line`, H1-H3 resolved (AC-2)
- [ ] Fix applied at owning layer; single-peer establishes with active observer (AC-3)
- [ ] 7 redistribute tests + api-raw/route-refresh converted, verified, baseline ratcheted (AC-4)
- [ ] as112-announce + replay tests still green (AC-5)
- [ ] Tests written (stall regression test + converted functional tests)
- [ ] Tests FAIL before the fix (stall reproduced, `connections-established: 0`)
- [ ] Tests PASS after the fix
- [ ] `make ze-test` green
- [ ] Review Gate: `/ze-review` clean (0 BLOCKER, 0 ISSUE)
