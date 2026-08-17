# spec-fixit-redistribute-establishment-stall

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-fixit-migrate-sleeps-infra (P0 carve-out); spec-redistribute-late-join-replay (closed, learned 1062) |
| Phase | 4/6 |
| Updated | 2026-07-22 |

Phase note (cell corrected 2026-07-22; the old "fix proposed, not implemented"
was stale): F1-F3 are implemented (`internal/test/runner/peer_contract.go` with
`isSelfValidated`/`validatePeerBlocks`, shared `peer.ConsumesLine` in
`expect.go`) and F4 is complete (converted `eor-sent`/`local-preference`
expectations in `bgp-redistribute-announce.ci` and
`forward-mpreach-nexthop-self-two-peer.ci`). F5 and F6 remain open.

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
| 1 | `LoadExpectFile` passes ONLY `expect=bgp:` / `action=` lines to ze-peer; `expect=json` is dropped ("Ignore json, stderr, syslog - handled by test runner") | `internal/test/peer/expect.go` |
| 2 | a check-mode peer with `len(config.Expect) == 0` prints `no test data available to test against` and returns 1 | `internal/test/cli/cmd_peer.go` |
| 3 | so `Run` is never reached: the listener at `lc.Listen` never binds and never prints `listening on` | `internal/test/peer/peer.go,215,217` |
| 4 | the runner skips its peer-bind barrier for exit-code tests, so ze starts against a dead port and no error is raised | `internal/test/runner/runner_exec.go` (`if rec.ExpectExitCode == nil`), `:770` |

`bgp-redistribute-announce.ci`'s peer block carries exactly one expectation, an
`expect=json`. Its ze-peer therefore never listens.

### D2 -- `expect=exit:code=N` silently disables ALL BGP validation in a peer test

`internal/test/runner/runner_exec.go` computes `selfValidated` as
`rec.ExpectExitCode != nil || (!hasPeer && hasOutputAssertion)`, and the whole BGP peer
path runs only under `if !selfValidated`. So a non-nil `ExpectExitCode` alone skips
`:1121` (the peer `successful` check) and `:1142` (`validateJSON`). All 7
`bgp-redistribute-*.ci` declare
`expect=exit:code=0`, so the peer's wire expectations and `validateJSON`
(`runner_validate.go`, whose `:114-116` no-match error is real and would have fired)
are never evaluated. The test passes on ze's exit code alone.

-> Constraint: D2 MASKS D1. The comment at `runner_exec.go` already worries
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
| `bgp-redistribute-announce.ci` (1-peer, "stalls") | NO -- json only | YES | peer never listens; nothing enforced -> a VACUOUS test |

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
(`peer.go`) and commit 44ad25d23 are unrelated to this symptom and neither caused nor
masked it.

### Firewall head-of-line-blocking link: REFUTED

-> Decision: `plan/spec-fixit-firewall-concurrency-deadlock.md` and this spec share NO
root cause. That spec's mechanism is a real lock-discipline chain in production plugin
code: ddos-local holds `r.mu` (`internal/plugins/ddos/local/responder.go,136`) across
an unbounded netlink `Flush` (`internal/plugins/firewall/nft/backend_linux.go` ->
`nftables conn.go`, no deadline set at `backend_linux.go`), while
`handleShowDdosLocal` (`show.go` -> `responder.go`) needs the same lock on the
dispatch path. This spec's mechanism involves no lock, no dispatch handler, no shared
goroutine, and no production code at all: a test peer exits before binding. The
resemblance was only the surface shape ("an observer's command does not get what it wants
during a long operation"). Both specs must proceed independently; neither unblocks the
other.

## F1-F3 IMPLEMENTED 2026-07-16 (F4 open); measured blast radius below

-> Decision: F1, F2 and F3 are implemented and green (`make ze-build` OK,
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
peers loop until killed and never print `successful` (`peer.go` continues
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
never reaches the peer. ~~unreachable by construction: the `.run` plugin calls
`request shutdown` at post-startup, so ze closes during the OPEN handshake. Fix per
file: `--mode sink`, or reach Established before shutdown (stronger).~~

-> Correction (2026-07-16): **the strikethrough above is WRONG, and all three fixing
agents disproved it independently.** These sessions do NOT die in the OPEN handshake:
all 33 reach Established (peer logs show `open recv`/`open sent`, observers print their
OK line and find routes in the RIB). The real cause is a **teardown race**. ze holds the
EOR behind the initial-sync barrier -- `peer_initial_sync.go` sleeps 500ms then
`waitForAPISync(2 * time.Second)` when `apiSyncExpected > 0`, and the EOR is written at
`:334` -- while the observer finishes in milliseconds and calls `request shutdown`,
closing the session first. The peer then reports `connection closed before completion`.

-> Constraint: **`state=established` is NOT a sufficient gate**, because the EOR is sent
AFTER establishment. Gate on `eor-sent >= 1` read from `show bgp peer <sel> detail`
(`peer.go`), which is the counter incremented at the EOR WRITE itself rather than at
scheduling. `api.quiesce()` is the alternative barrier and is equally sufficient AFTER
establishment (`peer_run.go` sets `sendingInitialRoutes` at Established; `:404` clears
it only after the EOR; `peer.go` `PendingSync()` and `reactor_api.go`
`DrainPeerSync` block on exactly that) -- but quiesce ALONE is not an establishment
barrier, since `reactor_api.go` skips a down/idle peer with an empty queue. So:
poll established, THEN quiesce; or gate on `eor-sent`.

-> Constraint: the "rr-basic EOR-wait pattern" this spec named **does not exist**.
`rr-basic.ci` carried only a peer-block comment plus an `expect=bgp` line acting as a
TCP-hold so the peer would not close while the observer polled. It held the PEER, not ze,
and contains no established-wait. Two agents were sent to reuse it and both found nothing
to reuse.

-> Decision: `--mode sink` was the honest answer for **zero** of the 33. Sink requires
DELETING the EOR expectation, which is banned; the `eor-sent` gate deletes nothing and is
free. Fixes are additions only.
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

#### B2.2 `bgp-rs-fastpath-ebgp-shared.ci` -- INVESTIGATED 2026-07-16. Not a product bug.

-> Decision: IRR is RULED OUT and the constraint above mis-attributes it. The `.ci`
declares no IRR filter, contains zero occurrences of `irr`, and 5/5 reproductions
produce zero `irr` lines in the client log. The `irr: lookup failed` line belongs to
another test's log. No further IRR work is needed for THIS file, and no other Class B2
file's diagnosis should assume the IRR confounder without re-checking its own log.

-> Decision: ze's wire output is CORRECT for the sequence the harness actually creates.
The test's expectation is NOT stale (its content is right, see below); the test's
MECHANISM cannot create the precondition it asserts. Root cause is test design.

Producer chain for the observed WITHDRAW (every step read, not inferred):
1. `internal/test/peer/peer.go` -- a **check-mode** ze-peer accepts one
   connection, waits for it to complete, and only then accepts the next
   (`if p.config.Mode != ModeCheck { continue }`; sink/echo do accept concurrently).
   `peer.go` closes each connection via `defer c.Close()` when its script ends.
   So `option=tcp_connections:value=2` in check mode means SEQUENTIAL sessions:
   conn 1 is CLOSED BEFORE conn 2 is accepted. The two RS clients are never
   simultaneously established, so RS forwarding cannot occur regardless of ze.
2. `internal/test/peer/checker.go` -- `conn=N` binds to the Nth **accepted**
   connection (`connectionIDs[0]` popped in order). Nothing correlates `conn=N` to a
   source IP. Observed: 127.0.0.2 (receiver-peer) is accepted FIRST in 5/5 runs, so
   `action=send:conn=1` fires at the RECEIVER, not the intended source at 127.0.0.1.
3. `rs/server_forward.go` -- `selectForwardTargets` skips peers with `!peer.Up`.
   The other client is not up yet, so targets is empty.
4. `rs/server_forward.go` -- empty targets -> `releaseCache`, no forward.
   The announce is correctly dropped: an RS forwards to OTHER clients; there were none.
5. `rs/server_withdrawal.go` -- the withdrawal map is updated regardless of
   whether the forward happened, recording 10.0.0.0/24 against the source peer.
6. conn 1 closes (step 1) -> `rs/server_handlers.go` `case "down"` ->
   `handleStateDown` -> `sendBatchedWithdrawals` emits
   `update text nlri ipv4/unicast del 10.0.0.0/24` to every peer except the source.
7. conn 2 (127.0.0.1) then establishes and receives that withdraw, then EOR.

-> Constraint: step 6 is a DEDUCTIVE proof, not ordering inference.
`server_handlers.go` (`buf.WriteString(" del ")`) is the ONLY withdraw emitter in
the whole rs plugin; `sendBatchedWithdrawals` has exactly ONE caller
(`server_handlers.go`), and rs's `handleStateDown` has exactly ONE caller
(`server_handlers.go`, `state == "down"`). The source never sent a withdraw.
Therefore the received withdraw PROVES a peer-down was processed with the prefix in
the withdrawal map. Withdrawing a downed peer's routes is required behavior, not a bug.

-> Decision: the expectation's CONTENT (AS_PATH prepend to [65000]) is correct for this
config and must NOT be "fixed". `rs/yang/ze-rs-conf.yang` defines `rs-client`
(default **false**) as "transparent AS-path forwarding. RFC 7947 Section 2.2.2: the
route server MUST NOT modify AS_PATH ... When true, the reactor SKIPS AS-path
prepending". The `.ci` sets `rs-fast-path enable` but NOT `rs-client`, so prepend is
the designed behavior here. RFC 7947 non-prepend does not apply. The expectation is
right; it is simply unreachable.

-> Constraint: do NOT close this by rewriting the hex to the observed withdraw. That
would enshrine a scenario that exercises no forwarding at all. The banned move is
exactly what the `contains=` sibling below already did by accident.

**Adjacent false green found (NOT fixed, different file, reported only).**
`test/plugin/bgp-rs-reactor-fastpath.ci` asserts
`expect=bgp:conn=2:seq=1:contains=180A0000`. It is structurally IDENTICAL to this test
(same single check-mode ze-peer, `tcp_connections:value=2`, `bind 0.0.0.0`, same two
peers, same conn=1 send / conn=2 expect) and it PASSES -- but `checker.go`
matches `contains:` with a plain `strings.Contains` on the hex stream, and the withdraw
wire `...001B020004180A00000000` CONTAINS `180A0000`. So it is satisfied by the very
same WITHDRAW that fails this test byte-for-byte. Its PASS is not evidence that RS
forwarding works. `bgp-rs-fastpath-ebgp-shared` is only "red" because its
byte-equality assertion is honest enough to notice.

-> Decision: recommended fix is to make the harness create two CONCURRENT sessions,
per the documented multi-peer pattern in `docs/architecture/testing/ci-format.md`
("Example (Multi-Peer)"): two separate `cmd=background` ze-peer processes, one per
loopback (`--bind 127.0.0.2`), each with its own `conn=1`, instead of one check-mode
ze-peer multiplexing both via `tcp_connections:value=2`. That removes BOTH the
accept-order race (step 2) and the sequential-close blocker (step 1), and makes the
announce expectation reachable so it can be tested honestly. This is a test-infra
change beyond a single expectation edit, so it is left for Thomas to schedule.
`bgp-rs-reactor-fastpath.ci` needs the same restructure plus a tightened assertion.

-> AUTONOMOUS DEFAULT (2026-07-17): ADOPT the two-concurrent-`cmd=background`-ze-peer
restructure above as the PLAN OF RECORD for B2.2 (`bgp-rs-fastpath-ebgp-shared.ci`),
B2.3/B2.4 (`remove-private-as-{export,replace-peer}.ci`) and `bgp-rs-reactor-fastpath.ci`,
superseding "left for Thomas to schedule" for planning purposes. It was already the recorded
RECOMMENDATION and is proven to work (the WITHDRAW vanishes 26/26 with real announces
produced -- see below and the B2.3/B2.4 measurement, "confirmed to work"), and the pattern is
a documented, supported harness idiom (`docs/architecture/testing/ci-format.md`
"Example (Multi-Peer)", re-verified 2026-07-17). Rationale: this is a SCHEDULING / test-infra
choice, not an irreversible on-wire decision, so the conservative default is to unblock a
future implementer with the smaller self-contained option rather than leave it parked.
Thomas: override if wrong. SCOPE (unchanged, do NOT overread this adoption): the restructure
is the HARNESS fix ONLY. It does NOT by itself close B2.3/B2.4 -- the private-ASN leak is a
separate real PRODUCT bug and the restructured tests stay red ~17% of runs until it is fixed
(see the B2.3/B2.4 "PRODUCT BUG -- private ASN leak" finding and its "do NOT close either
file until the leak is resolved" constraint). It also does NOT resolve the genuine
PRODUCT-CODE design calls that remain Thomas's: the EOR-delay remedy (B2.1/B2.5 "design call
for Thomas"), the EBGP-egress ATTR_DISCARD-marker question (B2.3/B2.4), and whether
Adj-RIB-Out should reflect config statics (B2.5 secondary finding). Those are
design-preference, not implementation-blocking, and are intentionally left open.

#### B2.1 `api-route-refresh.ci` + B2.5 `rib-pipe-filter.ci` -- INVESTIGATED 2026-07-16. SHARED root cause. Contains a REAL product bug.

-> Decision: **B2.1 and B2.5 share a root cause with EACH OTHER; B2.6 does NOT.** The
natural grouping ("the two pipe tests are the pair") is WRONG. `rib-pipe-filter` pairs
with `api-route-refresh`, not with `test-pipe-first-last`. Both are red for one reason:
**ze does not send its EOR until 2.5s after establishment, and both tests kill the
daemon before then.** In both, every OTHER asserted byte is produced correctly.

Wire evidence -- the asserted CONTENT is byte-correct in both; only the EOR is missing:
- B2.1: peer received `FFFF..FF:0017:05:00010001` = the ROUTE-REFRESH, **byte-identical**
  to the `:12` expectation. The EOR never arrived.
- B2.5: peer received `FFFF..FF:0030:02:00000015400101004002004003040A0000014005040000006418C0A801`
  = the 192.168.1.0/24 UPDATE, **byte-identical** to `:22`. The EOR never arrived.
- Expectation ORDER is not asserted and is a red herring: `checker.go` matches a
  received message against ANY unmatched expectation in the same (conn,seq) group, and
  both files put every expectation at `conn=1:seq=1`.

Producer chain for the missing EOR (every step read, not inferred):
1. `peer_run.go` -- on Established, `apiSyncExpected` = count of ProcessBindings
   with `SendUpdate` (i.e. every `send [ update ]` process).
2. `peer_initial_sync.go` -- if that count > 0: `clock.Sleep(500ms)` then
   `waitForAPISync(2 * time.Second)`.
3. `peer_initial_sync.go` -- the EOR for every negotiated family is sent ONLY
   AFTER that wait (RFC 4724 S2, "including the case when there is no update to send").
4. `peer.go` -- `waitForAPISync` returns on `<-ready` or the FULL timeout
   (`:441-444`, "API sync timeout"). Nothing shortens it.
5. `peer.go` -- `Peer.SignalAPIReady` (the only thing that closes `ready`) is
   reachable ONLY from `api_sync.go` `SignalPeerAPIReady`, driven by the
   `peer <addr> plugin session ready` command.
6. That command has exactly TWO emitters: `rib_replay.go` and `:276`.
   (`rib_commands.go` and `rib_replay.go` document that they deliberately do NOT.)

Why neither test ever signals -- TWO different upstream reasons, one shared consequence:
- **B2.1**: the bound `send [ update ]` process is `bgp-route-refresh`, which contains NO
  signalling code at all (grep over `plugins/route_refresh/`: the only `SignalAPIReady` is
  a no-op in `handler/mock_reactor_test.go`). And `config/peers.go` REQUIRES
  route-refresh to be bound with `send [ update ]`, so `apiSyncExpected >= 1` is
  UNAVOIDABLE for every route-refresh peer.
- **B2.5**: the bound process is `bgp-rib`, which HAS the signal but cannot reach it when
  the peer's Adj-RIB-Out is empty. `rib_replay.go` signals ready when
  `len(groups)==0`, but `collectGroupedRibOutRoutesFiltered` returns **nil** on BOTH empty
  paths (`:54-56` no ribOut map for the peer; `:98-100` no matching routes) and NEVER an
  empty non-nil slice -- while both callers gate on `if replayGroups != nil`
  (`rib.go` handleStructuredState, `rib.go` handleState).

-> Decision (**PRODUCT BUG**, B2.5's chain): `rib_replay.go` is **dead code from
the peer-up path**. It exists precisely to signal "nothing to replay, proceed", and the
nil-vs-empty conflation at both call sites defeats it; the dead branch is the author's
stated intent, unreachable. Net: `bgp-rib` signals ready ONLY when it has >= 1 route to
replay, and a FRESH peer never does. Blast radius far beyond this test: **every fresh BGP
session with `bgp-rib` bound `send [ update ]` and an empty Adj-RIB-Out delays its EOR by
500ms + 2000ms = 2.5s.** Not a protocol violation (RFC 4724 S2 sets no EOR deadline), but
a real operator-visible convergence delay on the normal startup path. Fix belongs at
`rib.go`/`:1115` (call replay whenever the peer came up and let `rib_replay.go`
handle empty), or by returning an empty non-nil slice. NOT fixed here (characterise-only).

-> Constraint: fixing that guard does NOT fix B2.1. `bgp-route-refresh` has no signaller
at all, so a route-refresh peer WITHOUT `bgp-rib` still burns the full 2s. The deeper
mismatch: `peer_run.go` counts plugins that MAY send updates, but only `bgp-rib` ever
SIGNALS. The counter set and the signaller set disagree. Whether the remedy is "count only
plugins that signal", "make every `send [ update ]` plugin signal", or "shorten the
fallback" is a design call for Thomas, not a mechanical fix.

Direct evidence (debug via `option=env:var=ze.log.bgp.routes:value=debug`, which MUST go
ABOVE the `stdin=peer` header -- inside it the runner rejects it):

| # | Experiment | Result |
|---|-----------|--------|
| E1 | `api-route-refresh`, committed `time.sleep(1.0)` | FAIL. `sleeping for API routes duration=500ms` @14.187, `waiting for API sync expected=1` @14.688, route-refresh sent @14.690. NO `API sync complete`, NO `sent EOR`. |
| E2 | same, sleep 4.0 / 3.0 / 2.0 | **PASS all three.** The EOR arrives and both expectations match. Proves the EOR is LATE, not absent. |
| E3 | `rib-pipe-filter`, committed | FAIL. `route sent 192.168.1.0/24` @10.703, `sleeping for API routes` @10.703, `waiting for API sync expected=1` @11.203, no completion. |
| E4 | same + `time.sleep(3.0)` before shutdown | **PASS.** Peer log now shows BOTH `...:0030:02:...18C0A801` AND `...:0017:02:00000000` (the EOR). |
| E5 | `api-route-refresh` with `send [ update ]` removed from `process route-refresh` | INVALID as a probe: ze REJECTS the config (`peers.go`, "route-refresh requires process with send [ update ]"). Recorded because it looks like an obvious experiment and is not one. |

-> Decision (verdict B2.1 `api-route-refresh`): **ze's ROUTE-REFRESH is CORRECT. The
test's expectation CONTENT is correct. The test's TIMING is wrong.** RFC 2918 S3: the
ROUTE-REFRESH message is 23 bytes, type 5, AFI(2) + Reserved(1) + SAFI(1); ze sent
`0017 05 0001 00 01` = length 23, IPv4/unicast, Reserved 0. Correct. RFC 7313 **is**
negotiated here (ze's OPEN carries `4600` = capability 70; `capability.go`
"CodeEnhancedRouteRefresh Code = 70 // RFC 7313 Section 3.1"; the ze-peer mirrors it), and
RFC 7313 S4 redefines that Reserved octet as Message Subtype -- but subtype 0 IS the
normal refresh REQUEST; BoRR/EoRR (1/2) are the RESPONDER's duty. ze is the requester, so
subtype 0 is correct under BOTH RFCs. The test fails only because its blind
`time.sleep(1.0)` tears the daemon down at ~1s while ze's EOR is structurally due at ~2.5s.

-> Decision (verdict B2.5 `rib-pipe-filter`): **BOTH are wrong.** The asserted wire content
is correct and ze produced it byte-for-byte; the missing EOR is the REAL ze latency bug
above; AND the test's timing is wrong. Fixing only the test would permanently hide the bug.

-> Constraint: do NOT close either by bumping the sleep. That converts a 2.5s product
delay into a permanently-hidden one and re-adds a blind sleep that
spec-fixit-migrate-sleeps-infra is removing. The honest test fix is the E9 recipe (poll
`show bgp peer <n> detail` for eor-sent) AND a separate decision on the EOR delay itself.

**Secondary finding (B2.5, NOT the peer-check failure, NOT currently enforced).**
`rib-pipe-filter`'s python asserts config-static routes appear in Adj-RIB-Out: `:147-148`
(`show bgp rib sent` -> key `adj-rib-out`), `:166` (`count-total` exact=2), `:174`
(`count-sent` exact=1), `:194-196` (`prefix-filter` exact=1). All four FAIL today
(`missing key 'adj-rib-out' in data: []`, `count=1 != expected 2`, `count=0 != expected 1`,
`count=0 != expected 1`) and all four contradict a DELIBERATE design:
`peer_initial_sync.go` sets `sendingConfigStatic`, `reactor_notify.go` tags the
sent event, and `rib_structured.go` skips ribOut storage for it ("Storing them in
ribOut would cause duplicates (config re-send + RIB replay)");
`docs/architecture/bgp/on-demand-origination.md` corroborates
`config-static` as the established Meta consumer. The file's own header `:5` ("Config
static route populates adj-rib-out via RIB plugin") is therefore a FALSE premise. These
never failed the test because `sys.exit(1)` sets the PLUGIN's exit code while
`expect=exit:code=0` checks ZE's. Whether Adj-RIB-Out SHOULD reflect config statics is a
design question; the assertion is disproven from the design record, not from ze's output.

#### B2.6 `test-pipe-first-last.ci` -- INVESTIGATED 2026-07-16. INDEPENDENT of B2.1/B2.5. Test is unmatchable by construction; feature never tested.

-> Decision: shares NO root cause with B2.1/B2.5. Its EOR arrives NORMALLY (peer log shows
`...:0017:02:00000000`) precisely because its `dispatch_until(..., attempts=40)` poll burns
more wall-clock than the 2.5s the other two miss. Same class of red, unrelated cause.

-> Constraint: `first`/`last` here are **CLI PIPE operators** bounding RIB query output
(`show bgp rib received first 3`, `... last 2 count`), per the header `:1-3`. Nothing in
this test concerns filter-chain ordering. Do not go looking for an ordering bug.

Three independent, provable defects in the 5 UPDATE expectations (`:10,12,14,16,18`):
1. They use `.{8}` (regex) for NEXT_HOP. **The matcher has no regex.** `checker.go`
   `matchRule` supports only `prefix:`, `contains:`, and `strings.EqualFold`;
   `internal/test/peer/` imports no `regexp` at all. These can NEVER match any byte string.
   This is the ONLY file in `test/plugin` + `test/encoding` using `.{N}`.
2. The declared BGP length is WRONG: the hex declares `002E` = 46 but the message is 48
   bytes (19 header + 2 withdrawn-len + 2 attr-len + 21 attrs + 4 NLRI). Self-inconsistent:
   no encoder produced this, it was hand-written.
3. The NLRI is WRONG: `18 010100` = **1.1.0.0/24**, but the matching injection line `:9`
   says **10.1.0.0/24**, which encodes `18 0A0100`. Off by the first octet, in all five.

And the routes are never injected at all:
4. `:9,11,13,15,17,19` are `cmd=api:...` lines INSIDE a `stdin=peer` block. ze-peer IGNORES
   them: `expect.go` (`case "cmd": // Ignore - documentation only`) and `consumes()`
   (`:32-44`) returns false for `cmd`. `cmd=api` is an ENCODE-suite directive
   (`record_parse.go` sets `msg.Cmd` for the runner to drive ze's API); in a
   plugin-suite peer block it is inert. Proven by the plugin's own log:
   `FAIL: first 3 count returned 0, expected 3` -- the RIB is EMPTY.

-> Decision (verdict): **the test's expectation is wrong, comprehensively; ze is NOT
implicated.** Proven from the harness's own matcher and from encoder-independent
arithmetic, NOT from ze's output. But the consequence is NOT merely "fix the test":
**the `first`/`last` pipe operators have NEVER been exercised.** `first 3` returned 0 on an
EMPTY RIB, which is correct-on-empty and proves nothing. The feature is **UNVERIFIED** --
not proven good, not proven broken. Closing this file without first making the RIB
non-empty would re-ship the same false green in a new costume.

-> Constraint: the direction is also inverted. The header `:5` and the python
(`show bgp rib received count == 5`, i.e. Adj-RIB-**In**) intend the PEER to SEND 5 routes
to ze; but `expect=bgp` asserts what the peer **RECEIVES**. With an empty ze RIB the peer
would receive only the EOR, so even a regex-capable matcher would never see those 5
UPDATEs. An honest fix must make the peer SEND (e.g. `option=update:value=send-route:prefix=...`,
`expect.go`), drop the 5 receive-side expectations, and only then assert first/last
on a populated RIB.

-> Constraint (scope, 2026-07-16): B2.1/B2.5/B2.6 were characterised only. No production
file was modified; no expectation was edited; no `expect=exit:code=0` was re-added. The
three `.ci` files are byte-identical to HEAD (experiments were reverted from
`tmp/*.ci.orig` backups). B2.3/B2.4 (`remove-private-as-*`) were NOT investigated here.

#### B2.3 `remove-private-as-export.ci` + B2.4 `remove-private-as-replace-peer.ci` -- INVESTIGATED 2026-07-16. SHARED root cause with EACH OTHER and with B2.2. Contains a REAL product bug (private ASN leak).

Reproduced at HEAD via a pristine `git archive` tree (the working tree does not build:
`internal/component/command/pipe.go sessionFormat` undefined, another session's WIP).
Runner `bin/ze-test bgp plugin <name>` (`make ze-functional-plugin-test`).

-> Decision: **NEITHER expectation was edited and both stay red.** The AS_PATH each test
asserts is byte-exact CORRECT. Neither red is a remove-private-as bug.

-> Decision: B2.3 and B2.4 share ONE root cause with each other, and it is **B2.2's**
root cause, not a filter bug. Same shape: one check-mode `ze-peer`,
`tcp_connections:value=2`, `bind 0.0.0.0`, two ze peers on 127.0.0.1/127.0.0.2. B2.2's
producer chain (`peer.go` sequential accept, `server_handlers.go,92` ->
`sendBatchedWithdrawals`) explains the observed WITHDRAW here verbatim; it is not
re-derived. Corroborating measurement: 127.0.0.2 is accepted first in 8/10 runs, so
`action=send:conn=1` injects into the RECEIVER, and the conn=2 expectation then sees the
peer-down withdraw `...001B020004180A00000000`. **New evidence FOR B2.2's recommended
restructure:** rebuilding both files as two concurrent `cmd=background` ze-peer processes
(`--bind 127.0.0.2`, `tcp_connections:value=1` each) makes the WITHDRAW vanish in 26/26
runs and produces real announces. B2.2's fix is confirmed to work.

**The AS_PATH rewrite is CORRECT. Both tests. Byte-exact.** Once the sessions are
concurrent, ze emits (`ze bgp decode --update`):

| Test | Expected AS_PATH | ze actual | Verdict |
|------|------------------|-----------|---------|
| B2.3 STRIP | `[65000 64496 64497]` | `[65000 64496 64497]` | MATCH |
| B2.4 REPLACE peer-as | `[65000 64496 65002 64497]` | `[65000 64496 65002 64497]` | MATCH |

64512 is the only RFC 6996 private ASN in the fixture path (64496/64497 are RFC 5398
documentation ASNs, correctly retained). Producer read, not inferred:
`rewritePrivateASSegments` (`internal/component/bgp/reactor/filter_delta.go`)
drops the ASN when `mode != peer-as` and substitutes `peerAS` when it is;
`isRFC6996PrivateASN` implements 64512-65534 / 4200000000-4294967294 exactly.
`filter_ordered.go` passes `destPeerAS` for export (65002 = the receiver's remote AS),
which is why REPLACE yields 65002 and not the local 65000. Governing text: RFC 6996 S4
(the MUST) + S5 (the ranges); the remove-vs-replace choice is vendor policy, unspecified
by RFC, so `replace-with peer-as` is a design decision, not an RFC requirement.

**Finding: the ONLY wire divergence is LOCAL_PREF, and the EXPECTATION is wrong.**
Both tests expect `40 05 04 00000064` (LOCAL_PREF 100) to reach the receiver. ze instead
emits `C0 FD 04 05010000` -- same 7 bytes, which is why total lengths still match.
That is attr 253 `attrCodeAttrDiscard` (`message/attr_discard.go`,
`draft-mangin-idr-attr-discard-00`), value = code 0x05 (LOCAL_PREF), reason 0x01
(`DiscardReasonEBGPInvalid`). Producer chain, every step read:
`validateLocalPrefAttr` (`internal/component/bgp/message/rfc7606.go`) returns
`RFC7606ActionAttributeDiscard` + `DiscardReasonEBGPInvalid` when `!isIBGP` ->
`reactor/session_validation.go` calls `message.ApplyAttrDiscard` -> `applyInPlace`
(`message/attr_discard.go`) overwrites LOCAL_PREF in place.
Proven from RFC, NOT from ze's output: the fixture sends LOCAL_PREF INTO an EBGP session
(source-peer local 65000 / remote 65001), which **RFC 7606 S7.5** requires the receiver to
discard; and the receiver is EBGP (65000/65002), to which **RFC 4271 S5.1.5** forbids
sending LOCAL_PREF at all. The expectation is unsatisfiable under either RFC.
-> Decision: NOT rewritten to the observed bytes. Correcting it means ruling on whether an
EBGP-facing ATTR_DISCARD marker is intended egress output. `applyInPlace` computes
`0x80 | (origFlags & 0x50)`, so LOCAL_PREF's 0x40 makes the marker **optional TRANSITIVE
(0xC0)**, i.e. propagated onward by every conforming speaker. That is a design call for
Thomas. RFC 7606's attribute-discard is a RECEIVE-side error action; whether the marker
should survive to EBGP egress is exactly the open question.

**Finding: PRODUCT BUG -- private ASN leak. The configured export filter is BYPASSED on a race.**
With `filter { export [ remove-private-as:REPLACE ] }` configured, ze delivered to the
EBGP receiver 127.0.0.2:
`0037:02:0000001C4001010040020E02030000FBF00000FC000000FBF140030401010101180A0000`
= `AS_PATH [64496 64512 64497]`: private **64512 NOT removed**, local **65000 NOT
prepended**, filter not applied. **RFC 6996 S4 is a MUST** ("Private Use ASNs MUST be
removed from AS path attributes ... before being advertised to the global Internet").
Non-prepend is also wrong here, and B2.2 already proved why: `rs/yang/ze-rs-conf.yang`
makes `rs-client` default **false**, so the reactor is supposed to prepend; RFC 7947 S2.2.2
transparency does not apply to this config.
Measured 3/18 concurrent-session runs, only when ze connected to the receiver BEFORE the
UPDATE arrived on the source. **`rs-fast-path` is NOT the discriminator** (leak 1/10 with
it DISABLED, 0/10 with it enabled), so this is not simply the documented fast path; that
hypothesis was tested and killed.
-> Constraint: mechanism is HYPOTHESIS, NOT VERIFIED. `forward_rs.go` already
intends to skip peers with `exportFilters` and hand them to bgp-rs `ForwardCached`, so the
leak implies that snapshot looked empty. `peerForwardFacts.exportFilters` is populated only
by `refreshForwardFacts` (`reactor/peer_forward_facts.go,116`), which its own comment
says runs at `setEncodingContexts` / `resolveDynamicPeerSettings`. A snapshot taken before
config/registry attaches the filter refs, never refreshed, would produce exactly this. The
LEAK IS MEASURED; that mechanism is not. Do not spec the mechanism before reading whether
`refreshForwardFacts` is re-run after filter refs resolve.
-> Constraint: both files' own headers already name this bug -- B2.3 `:4`
"PREVENTS: export policy rewrite being lost by the EBGP wire cache". The assertion was
written to catch it and never ran. Do NOT close either file until the leak is resolved:
restructuring them per B2.2 makes them red ~17% of runs, and "fixing" the LOCAL_PREF
expectation alone would leave a flaky test whose flake IS the leak.

-> Constraint (scope, 2026-07-16): B2.3/B2.4 characterised only. No production file
modified. Neither `.ci` edited; both byte-identical to HEAD. No `expect=exit:code=0`
re-added. All experiments ran in a throwaway `tmp/` export, never the working tree.

### Proposed fix (F1-F3 now implemented, see above; F4 open)

| # | Change | File | Why |
|---|--------|------|-----|
| F1 | Fail loudly instead of passing: a record with a ze-peer AND `ExpectExitCode` must still run the peer `successful` check + `validateJSON`. Narrow `selfValidated` to `!hasPeer && (rec.ExpectExitCode != nil \|\| hasOutputAssertion)` | `internal/test/runner/runner_exec.go` | removes D2, the masking defect; makes every affected test honest. Expect reds: that is the point |
| F2 | Reject a check-mode peer block whose expectations are all runner-side, at PARSE time, naming the file | `internal/test/runner/record_parse.go` (peer-block validation) | removes D1's silent mode; `expect.go` dropping `expect=json` is correct behavior, the bug is that nothing notices the peer is left with nothing to do |
| F3 | Make ze-peer's "nothing to check" exit unmistakable in the runner's report (it is currently only visible as the peer's stderr in a failure dump) | `internal/test/cli/cmd_peer.go` + runner report | a peer that never binds should never look like a passing test |
| F4 | Then fix the 7 `bgp-redistribute-*.ci`: add a ze-peer-consumed expectation, drop `expect=exit:code=0`, correct the JSON to include `local-preference`, and convert the sleeps (recipe proven at E9) | `test/plugin/bgp-redistribute-*.ci` | AC-3/AC-4; converts the bucket AND makes it assert for the first time |

-> Constraint: F1 before F4. Fixing the tests first would leave the harness able to hide
the next one. F1 will likely turn other tests red across the 21 files listed below; each
red is a pre-existing false green, not a regression, and must be triaged not silenced
(`ai/rules/completion.md`).

-> Status (2026-07-22, F4 COMPLETE): all 7 `bgp-redistribute-*` and the four
`redistribute-l2tp-*` tests pass with the converted expectations (committed earlier).
The last outstanding F4 file, `forward-mpreach-nexthop-self-two-peer.ci`, was converted
this session. Reading RFC 8950 (`rfc/short/rfc8950.md`) overturned the "enable Extended
Next Hop" shortcut: RFC 8950 defines only IPv4-NLRI-with-an-IPv6-next-hop (the reverse of
what the committed IPv6 config asked -- an IPv4 next-hop in an IPv6 MP_REACH), so that
combination is undefined and ze correctly refuses it (`reactor/peer.go` `ErrNextHopIncompatible`).
Rewritten to `ipv4/multicast`, an MP family (RFC 4760) that legitimately carries a 4-byte
IPv4 next-hop, keeping the two existing IPv4 loopback peers and next-hop-self, so each peer's
MP_REACH next-hop resolves to its own session address (`7f000001` vs `7f000002`). Byte-exact
`expect=bgp:hex=` on both peers + `expect=json`; mutation-verified (peer2 expecting peer1's
next-hop -> RED). This exercises the same per-session MP_REACH next-hop-self path WITHOUT the
second bindable IPv6 loopback the earlier "left red on purpose" note deferred; IPv6 MP_REACH
next-hop-self stays covered by `redistribute-as112-announce.ci`. Independently reviewed:
SOUND (one stale-comment defect fixed). F5 and F6 remain open.

| F5 (open, 2026-07-16) | **F2's own remedy text can produce a vacuous green.** `validatePeerBlocks` tells the author to "run the peer with `--mode sink`". Doing so makes `hasCheckPeer` false (`peer_contract.go`, re-read 2026-07-16); `isSelfValidated` returns false ONLY for a check peer, so with the peer sinked the bare `rec.ExpectExitCode != nil` at `:72` makes it TRUE. `runner_exec.go` gates the whole BGP branch on `!isSelfValidated(...)`, and `validateJSON` sits inside it at `:1141` (its own comment: "peer path only"). A file whose real assertions are `expect=json` therefore asserts NOTHING once sinked. The guard built to stop vacuous greens hands out a remedy that creates one -- the fail-open shape `ai/rules/evidence.md` names | `internal/test/runner/peer_contract.go` (remedy text + `isSelfValidated`), `runner_exec.go,1141` (the gate) | Found by the test-219 F4 shard |

-> Evidence (F5, reproduced not inferred): `forward-mpreach-nexthop-self-two-peer.ci`
turns **PASS in 8.2s while asserting nothing** when sinked, because its only real
expectations are `expect=json`. An agent following the guard's literal
advice will "fix" that file into a green that checks nothing.

-> Decision (F5): two candidate fixes. (a) make the remedy text state the `expect=json`
consequence so the author chooses knowingly -- a [workaround], it only warns. (b) evaluate
`validateJSON` outside the peer branch so runner-side JSON assertions survive sinking --
the [source] fix, since `expect=json` is consumed by the RUNNER, not by ze-peer
(`internal/test/peer/expect.go` ignores `cmd`; json never reaches the peer), so its
evaluation has no business being gated on the peer path at all.

-> Constraint (F5, scope): this does NOT affect the 8 tests sinked in `4ce173e32` /
`282663671`. All 8 were checked: **none declares `expect=json`**, so the skipped
`validateJSON` costs them nothing, and their surviving assertions were proven live by
deliberately breaking `show-l2tp-statistics`'s `expect=stderr:contains` (red) and
restoring it byte-exactly (green). The hole is real; those files do not sit in it.

| F6 (open, 2026-07-16) | **Audit the 40 `test/plugin/*.ci` that carry `cmd=api`, where it is INERT.** The D-4 analysis above proves it for `test-pipe-first-last`; the CLASS is unsized. `cmd=api` is an encode-suite directive (`record_parse.go` sets `msg.Cmd`; the ONLY reader is `report.go`, the reporter), and ze-peer discards it (`internal/test/peer/expect.go`: `case "cmd": // Ignore - documentation only`). In `test/encode` the runner drives ze's API from it; in `test/plugin` nothing does, so any test relying on it to inject routes asserts against an EMPTY RIB. `grep -rl "cmd=api" test/plugin/` = **40 files**. Split them: RELIES-ON (vacuous, must be converted to a real injection path such as `option=update:value=send-route:`) vs DOCUMENTS-ONLY (harmless, injection happens elsewhere) | `test/plugin/*.ci` (40 files); contract at `internal/test/peer/expect.go`, `record_parse.go`, `report.go` | Found by the test-506 F4 shard |

-> Constraint (F6): this is a THIRD vacuity class, distinct from F1 and F2. F1 = the peer
never bound. F2 = the peer declared nothing to check. F6 = the peer binds and asserts
correctly, but the test's PREMISE (that routes exist) was never established, so a
correct assertion is made against an empty RIB. F1/F2 cannot detect it: nothing about the
peer block is malformed.

-> Constraint (F6): do NOT bulk-convert. Whether a `cmd=api` line is load-bearing or
decorative is per-file judgement -- `test-pipe-first-last` needed real injection;
another file may inject via its `.run` script and carry `cmd=api` as a comment. Read each
test's VALIDATES line, as F4 requires.

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
  settings.ConnectRetry` (default 120s, RFC 4271 ConnectRetryTimer, `peersettings.go`),
  while `reconnectMax = DefaultReconnectMax` (60s). So the backoff floor (120s) exceeded
  its ceiling (60s), contradicting the design in `peer_run.go` ("min 5s, max 60s") and
  `DefaultReconnectMin` (5s, peer.go). A failed first connect stranded the peer
  `connecting` for 2 minutes. Fix: `reconnectMin := DefaultReconnectMin`. ConnectRetry keeps
  its real role as a connect timeout (`reactor_dynamic.go`). Reconnect unit tests use
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
| HEAD (`reconnectMin := DefaultReconnectMin`) | FAIL, 92.00s, `established==1` (`runner_test.go`) |

Order-dependent: a full-package `-race` run of `./internal/chaos/inprocess/` PASSED once, so
`make ze-chaos-unit-test` (`mk/test-chaos.mk`, `go test -race ./internal/chaos/...`) may
still be green. The isolation failure is the reliable reproducer.

This is chaos-harness work, NOT a BGP defect: the harness itself documents the intended 5s
backoff (`runner_test.go`, `runner.go` "DefaultReconnectMin = 5s virtual"). The
test was green only because the 120s bug parked the retry loop outside the window.

**RESOLVED 2026-07-16. The open question ("where does the real time go inside `vc.Advance`")
rested on a false premise: none of it is spent there, and the advance loop is not slow.**

A goroutine dump taken 30s into the freeze answered it in one run:

| Goroutine | Where |
|---|---|
| runner | `simWg.Wait()` (`runner.go`) -- the advance loop had ALREADY finished |
| ze session | `VirtualClock.Sleep` (`virtualclock.go`) from `session.go` |

The advance loop costs exactly what `runner.go` implies (~0.6s real for 60s virtual)
and then EXITS -- after which nothing advances the clock. `session.Run()` polls for its
connection with `s.clock.Sleep(10ms)` (`session.go`) and `VirtualClock.Sleep` is a
bare `<-ch` (`virtualclock.go`). `clock.Clock.Sleep` takes no ctx, so `simCancel()`
cannot reach a goroutine parked there -- only `Advance` can. ze's session was stranded
mid-sleep, never completed the handshake, the simulator blocked forever on the reply that
never came (`executeReconnectStorm` -> `readMsg`, `simulator_actions.go`), and
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
contradicts `peer_run.go`, which documents this loop as deliberately replacing the RFC
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
  `ZE_TEST_NO_BUILD=1 ZE_BIN=bin/ze` where `bin/ze` was a `make ze-build` **core** build lacking the
  `zetest` fake plugins (fakeredist/fakefib). With the default `zetest` build (buildZe,
  `cmd_bgp.go` uses `runner.TestBuildTags()`), `redistribute-as112-announce` passes.
  Always let ze-test build (do not pin ZE_BIN to a core binary) for the `bgp plugin` suite.
- "~20s StartPeers delay from observer polling": FALSE. That number conflated the ze-test
  `go build` time with startup. The manual full-log capture shows StartPeers is fast.
- "The reactor drops or rejects an inbound connection while the peer's outbound retry loop is
  cycling, so a faster backoff starves establishment": FALSE, and it is NOT a missing feature.
  The accept-while-cycling path exists and is wired: `peer_run.go` selects the backoff
  on `p.inboundNotify` alongside `p.clock.After(delay)` and restarts `runOnce` immediately
  WITHOUT doubling the delay; `reactor_connection.go` (`acceptOrReject`) buffers the
  connection via `peer.SetInboundConnection` rather than closing it on `ErrNotConnected` /
  `ErrSessionTearingDown` / `ErrAlreadyConnected` for a passive peer, commented as handling
  "the race where the remote reconnects faster than our session teardown";
  `peer_connection.go` stores the conn and signals the size-1 notify channel. Do not go
  looking for a missing inbound-accept mechanism in the reactor: read these three first.

### REMAINING (the real open question) -- ANSWERED 2026-07-16, see ROOT CAUSE at the top
~~Why does the ze-test check-mode peer never accept a connection when the observer polls during
establishment, while a long-lived sink peer does?~~ **Answered.** This section asked exactly the
right question and its instinct was correct: *"Every dial is `connection refused`, so the
ze-test peer is not listening at dial time -- pin ... whether the ze-test peer exits early."*
It does exit early, before binding: a check-mode peer whose only expectation is `expect=json`
is left with an empty `config.Expect` (`internal/test/peer/expect.go`) and bails at
`internal/test/cli/cmd_peer.go` before `Listen` (`peer.go`). The observer's dispatch
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
- `plan/future/spec-fixit-migrate-sleeps-infra.md` (Mistake Log / Failed Approaches: the bisection)
- `plan/spec-redistribute-late-join-replay.md` (the behavior the fix must not regress)
- `ai/rules/completion.md`, `ai/rules/evidence.md`

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
`register.go` subscribes the redistribute plugin to `["state"]`. On the down->up edge
(`register.go`, `OnStructuredEvent`) it calls `coord.onPeerUp(bus, peerAddr)`, which
allocates a monotonic replayID and emits `redistevents.ReplayRequest{replayID}`
(`replay.go`). Producers re-emit `RouteChangeBatch{ReplayID}`; the coordinator looks
up replayID->peer and injects the current redistribute route set to that ONE peer
(`replay.go`). So establishment synchronously drives a plugin-facing state dispatch
plus a replay injection back toward the reactor.

### Boundaries Crossed

| From | To | Shared point |
|------|----|--------------|
| Reactor session goroutine (establishment) | plugin engine RPC serialization | plugin-engine command/dispatch lock |
| Redistribute plugin state-event callback | replay coordinator inject | reactor forward pool |
| External observer engine RPC (`dispatch`/`quiesce`/`wait_for_event`) | same plugin-engine serialization | the establishing window |

### Integration Points
- `redistribute_egress` state subscription (`register.go`).
- Reactor establishment / forward-pool drain that `quiesce()` waits on.
- The plugin-engine command/dispatch serialization shared by observer and plugins.

## Wiring Test

| Entry Point | Feature Code | Test |
|-------------|--------------|------|
| Single-peer redistribute peer reaches Established while the observer polls the engine (`wait_for_event`/dispatch) during establishment | -> reactor establishment path + redistribute peer-up replay (`register.go`, `replay.go` coordinator) | new `.ci` / reactor test asserting the peer establishes; FAILS (stall, `connections-established: 0`) before the fix, PASSES after |

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
3. Fix at the owning layer per `ai/rules/completion.md` (likely: async /
   non-re-entrant peer-up replay dispatch, or decouple `quiesce` drain from
   peer-established state). Never weaken the test.
4. Convert the 7 redistribute tests (+ api-raw/route-refresh) to the proven
   `established -> eor-sent (show bgp summary) -> updates-sent (show bgp peer detail,
   reactor_notify.go)` recipe; verify each 3x + concurrently; ratchet the baseline.
5. Confirm no regression in `redistribute-as112-announce.ci` and the replay tests.

### Hypotheses -- ALL THREE REFUTED 2026-07-16 (AC-2 satisfied)
- H1 **REFUTED**: no contention exists. The redistribute peer-up replay
  (`redistribute_egress/register.go`, `replay.go`) is never reached in the affected
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
(`internal/test/peer/expect.go` -> `internal/test/cli/cmd_peer.go`).

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
  `make ze-build`; this spec cannot be implemented until the tree builds.

## Checklist

- [ ] Stall reproduced with a failing regression test (AC-1)
- [ ] Root cause cited `file:line`, H1-H3 resolved (AC-2)
- [ ] Fix applied at owning layer; single-peer establishes with active observer (AC-3)
- [ ] 7 redistribute tests + api-raw/route-refresh converted, verified, baseline ratcheted (AC-4)
- [ ] as112-announce + replay tests still green (AC-5)
- [ ] Tests written (stall regression test + converted functional tests)
- [ ] Tests FAIL before the fix (stall reproduced, `connections-established: 0`)
- [ ] Tests PASS after the fix
- [ ] `make ze-standard-test` green
- [ ] Review Gate: `/ze-review` clean (0 BLOCKER, 0 ISSUE)
