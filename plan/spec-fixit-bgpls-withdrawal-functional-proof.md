# Spec: fixit-bgpls-withdrawal-functional-proof

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/ad-hoc-2026-08-08-ci-31225029268.md` |
| Updated | 2026-08-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The route-server opaque (BGP-LS) withdrawal fix landed 2026-08-08 is proven
by UNIT tests only.

`test/plugin/rfc7606-54-bgpls-override-propagates.ci` CANNOT catch the defect on
any platform. It was run with the fix disabled and still passed, because its
assertions stop at the announcement and no route-server `.ci` ever closes a
peer. Only a running daemon proves the daemon
(`ai/rules/interop-and-goal-validation.md`).

The wire-visible defect itself is CLOSED: without the fix, BGP-LS routes a
departing peer contributed were never withdrawn from the other route-server
clients, and the unit tests that now cover it are proven to discriminate. What
is missing is functional proof.

**Two traps an earlier attempt paid for, worth inheriting:** the `.ci` phase
counter is SHARED by every connection of one `ze-peer`, so an assertion at
seq=2 holds the phase below an `action=close` at seq=3 and the test deadlocks.
On darwin the two peers need 15s or more to establish because of the local bind
of 127.0.0.2, which is plausibly the same asymmetry that made this class of
failure appear only on linux CI.

RFC 9552 Section 5.2 requires unknown Link-State NLRI to be handled as opaque
objects and to be preserved and propagated, so the new test should carry the
matching `RFC requirement:` tag.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` rule grammar this proof is written in
  -> Decision: two SEPARATE `ze-peer` processes, not one process with two connections.
     `Checker.groupMessages` flattens every `(conn, seq)` pair of ONE peer into a
     single linear cursor, and `nextCloseAction` fires only when `close` heads the
     CURRENT group. An unmet rule anywhere below the close is therefore a deadlock,
     not a failure. Separate processes give separate cursors.
  -> Constraint: every rule that gates the close must be one an OBSERVER can satisfy
     on demand. Both of this test's source-side waits are barrier routes ze
     originates to that peer alone, never assertions about ze's relay behavior.
- [ ] `test/draft/README.md` - draft-first workflow
  -> Decision: authored under `test/draft/plugin/`, promoted with `mv` once green.
  -> Constraint: no repo-wide gate sees a draft, so promotion is when the accept-only,
     sleep-ratchet, frame-length and dispatch-command checks start applying.

**Key insights:** (minimal context to resume after compaction)
- The requirement is `RFC9552-5.2-8`, in `rfc/short/rfc9552.md`: "An implementation
  MUST handle unknown Link-State NLRI types as opaque objects and MUST preserve and
  propagate them." Before this spec it carried unit evidence only
  (`rfc/requirements/rfc9552.md`).
- The defect's visible signature, reproduced on demand by disabling
  `appendParsedRecords`'s `*nlri.WireNLRI` branch:
  `update-route failed ... command="update text nlri bgp-ls/bgp-ls del wire[bgp-ls/bgp-ls](23 bytes)" error="family not supported in text mode: bgp-ls/bgp-ls"`.
- A route server drops an UPDATE from a client that arrives before the OTHER client
  is up (`forward matched no target ... 127.0.0.2=not-yet-up`), and peer-up replay
  cannot recover it when no `bgp-adj-rib-in` is loaded. Any fixture of this shape
  must order the announcement behind the route server's own view of readiness.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rs/server_inventory.go` - records the NLRI, now as hex for opaque families
  -> Decision: `appendParsedRecords` is the producing function, and its
     `*nlri.WireNLRI` branch is the whole mechanism. Disabling that one branch
     restores the pre-fix behavior exactly, which is what makes the discrimination
     run honest rather than an approximation of it.
  -> Constraint: `appendOpaqueRecords` splits the blob with `message.GetNLRISizeFunc`,
     so one 23-octet section becomes TWO records. The withdrawal on the wire
     therefore names both Link-State NLRIs, and the fixture asserts both.
- [ ] `internal/component/bgp/plugins/rs/server_handlers.go` - emits the batched withdrawal command
  -> Decision: `handleStateDown` is the entry point the departing peer reaches, and
     `sendBatchedWithdrawals` picks `update hex nlri` over `update text nlri` from
     the record's `wireForm` flag.
  -> Constraint: `sendBatchedWithdrawals` sorts each batch, so the same peer-down
     emits the same bytes every run. Without that sort the exact-frame assertion
     this fixture uses could not exist.
- [ ] `internal/component/bgp/reactor/forward_next_hop.go` - the RFC 4271 Section 5.1.3 withholding
  -> Constraint: `originatedNextHopIsPeerOwn` withholds any originated route whose
     NEXT_HOP is the destination peer's own address, with no exemption. Both barrier
     routes carry next-hop 1.1.1.1 and every peer block names different addresses on
     its two ends, or the barriers would be swallowed and the close never reached.

**Behavior to preserve:**
- The landed fix and its unit tests. This spec adds proof, it does not revisit the fix.
- Every existing test. No assertion was weakened and no `// test-relax:` token added.

**Behavior to change:**
- Nothing in the product. The gap this spec closes is in the EVIDENCE, not the code:
  the withdrawal path had unit proof and no functional proof, and the one `.ci` that
  touched BGP-LS relaying passed unchanged with the mechanism removed.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A route-server client's TCP session ends with no NOTIFICATION. The reactor reports
the peer down, and the `rs` plugin receives it as a `state` event.

### Transformation Path
`handleState` marks the peer down, then `handleStateDown` drains that peer's workers
and takes its withdrawal set. `sendBatchedWithdrawals` groups the set by family and
by command form, sorts each group, and dispatches `update hex nlri bgp-ls/bgp-ls del
<hex> del <hex>` with a selector that excludes the departing peer. The update command
parses the hex back into two NLRIs, and the reactor writes each surviving client one
MP_UNREACH_NLRI carrying the original bytes.

The set the withdrawal reads was filled on the way IN: `extractWireNLRIRecords` ran
before forwarding, `appendParsedRecords` saw one `*nlri.WireNLRI` for the whole NLRI
section, and `appendOpaqueRecords` split it into one hex record per NLRI.

### Boundaries Crossed

| From | To | What crosses |
|------|----|--------------|
| reactor session teardown | `rs` plugin | the peer-down `state` event |
| `rs` plugin | update command plugin | a text command line carrying hex NLRIs |
| update command plugin | reactor egress | two parsed Link-State NLRIs to withdraw |
| reactor egress | surviving client | one MP_UNREACH_NLRI, AFI 16388 / SAFI 71 |

### Integration Points
The route server's own readiness view, `show bgp rs peers` (`peerStatus`), is the
signal the fixture polls before letting the source client announce. It reports
`PeerState.Up`, written in the same critical section that makes a peer a forward
target, which the reactor's session state does not.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| a route-server client's TCP close | -> | `handleStateDown` / `sendBatchedWithdrawals` / `appendOpaqueRecords` | `test/plugin/rfc9552-52-rs-opaque-withdraw-peer-down.ci` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `TestOpaqueNLRIRecordedAsSplitWireBytes` | `internal/component/bgp/plugins/rs/server_opaque_withdrawal_test.go` | the inventory keeps the wire bytes, one record per NLRI |
| `TestPeerDownWithdrawsOpaqueNLRIAsWireCommand` | `internal/component/bgp/plugins/rs/server_opaque_withdrawal_test.go` | the peer-down command uses the hex form and parses |

Both landed with the fix on 2026-08-08. Neither runs a daemon, and the defect was a
DISPATCH refusal, so neither could see it. This spec adds the missing tier.

### Functional Tests
- [ ] A new `.ci` that closes a peer and asserts the surviving client receives the BGP-LS withdrawal
- [ ] It must be shown RED with the fix disabled and GREEN with it restored

## Files to Modify

| File | Change |
|------|--------|
| `test/plugin/rfc9552-52-rs-opaque-withdraw-peer-down.ci` | new: the functional proof, tagged `RFC requirement: RFC9552-5.2-8 positive` |
| `plan/spec-fixit-bgpls-withdrawal-functional-proof.md` | this spec, filled from what the work found |

No product file changes. `internal/component/bgp/plugins/rs/server_inventory.go` was
edited twice to disable and restore the mechanism for the discrimination runs, and is
byte-identical to HEAD.

`rfc/requirements/rfc9552.md` is regenerated by `make ze-rfc-index`, which adds the
new tag as functional evidence on the `RFC9552-5.2-8` row. It is a generated file
and is not authored by hand.

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No. No product file changed: `git status --porcelain -- internal/` names no path of this work | -- |
| 2 | Config syntax changed? | No. The fixture uses the `attach process` grammar that `06c95f65d` landed, and adds none | -- |
| 3 | CLI command added/changed? | No | -- |
| 4 | API/RPC added/changed? | No. The observer uses `show bgp rs peers` and `update text nlri`, both already documented | -- |
| 5 | Plugin added/changed? | No | -- |
| 6 | Has a user guide page? | No | -- |
| 7 | Wire format changed? | No. The fixture asserts the format ze already emits | -- |
| 8 | Plugin SDK/protocol changed? | No | -- |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes, newly PROVEN at daemon level | `rfc/requirements/rfc9552.md`, regenerated. `docs/features/rfc-status.md` keeps its RFC 9552 row: the row already claims propagation of unknown NLRI types, the Status stays `Partial`, and the nine `{gap}` rows are untouched, so an edit would change no published claim |
| 10 | Test infrastructure changed? | No. The fixture uses existing `.ci` rules only. `internal/test/` holds no edit of this work | -- |
| 11 | Affects daemon comparison? | No | -- |
| 12 | Internal architecture changed? | No | -- |
| 13 | Route metadata keys added/changed? | No | -- |
| 14 | Prometheus counters added/changed? | No. `updates-sent` and `eor-sent` are read, not added | -- |
| 15 | Registered plugin, event, send type, command, capability, or inventory changed? | No | -- |
| 16 | Any changed source file referenced by existing doc source anchors? | No. The 39 anchors naming `internal/component/bgp/plugins/rs/` all point at files this work did not touch (`docs/architecture/api/text-parser.md`, `text-format.md`, `docs/guide/route-reflection.md` and six others) | -- |
| 17 | Existing docs show config/CLI/API examples for this area? | No update owed. `docs/architecture/api/update-syntax.md` already carries the `update hex ... nlri <family> add/del <hex>` form the route server dispatches on peer-down | -- |

## Implementation Steps

1. Reproduce, then name the root cause at its producing function.
2. Fix at the owning layer, never at the symptom.
3. Prove the fix discriminates: red before, green after.

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The functional `.ci` exists and is a route-server peer-down proof | `ls test/plugin/rfc9552-52-rs-opaque-withdraw-peer-down.ci` and read the `action=close` rule |
| It passes through a running daemon | `ze-test bgp plugin --pattern rfc9552-52 -v` |
| It goes RED when the mechanism is disabled | build `ze` with the `*nlri.WireNLRI` branch of `appendParsedRecords` removed, and run the same command against that binary |
| The RFC tag resolves and is counted as functional evidence | `make ze-rfc-check` |
| No product code changed | `git status --porcelain -- internal/` |
| No test weakened | `python3 scripts/dev/audit-test-relaxation.py` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | The fixture feeds ze a hand-written UPDATE holding an NLRI type ze parses (type 1) and one it does not (type 99). The daemon must frame both by length alone and must not read inside the unknown one |
| Resource exhaustion | The observer polls with bounded attempt counts, and every peer block carries `option=timeout`, so no wait can hang a suite |
| Untrusted length fields | `appendOpaqueRecords` splits the blob with `message.GetNLRISizeFunc`. A length that runs past the end of the section must stop the walk, not index out of the slice |
| Test isolation | The four loopback addresses and the tmpfs name must not collide with another fixture running in parallel |

## Acceptance Criteria

| ID | Criterion | Evidence |
|----|-----------|----------|
| AC-1 | One functional `.ci` proves, through a running daemon, that a departing route-server client's BGP-LS routes are withdrawn from the surviving client | `test/plugin/rfc9552-52-rs-opaque-withdraw-peer-down.ci`, receiver rule at seq=2: the exact MP_UNREACH frame |
| AC-2 | The unknown Link-State NLRI type is withdrawn, not only the type ze parses | the same rule names both NLRIs, type 1 and type 99, byte-identical to what the source announced |
| AC-3 | The test carries the `RFC requirement:` tag for RFC 9552 Section 5.2, quoting the obligation | header line: `RFC requirement: RFC9552-5.2-8 positive` |
| AC-4 | The test is proven to DISCRIMINATE: red with the mechanism disabled, green with it restored | disabling the `*nlri.WireNLRI` branch of `appendParsedRecords` reddens seq=2 only, with `family not supported in text mode: bgp-ls/bgp-ls` in the daemon log; restoring it returns the run to green |
| AC-5 | The red is confined to the withdrawal, so the pass is not vacuous | in the red run the receiver's seq=1 announcement rule still matched: the fixture asserts the positive and the negative on one session in one run |
| AC-6 | The test is deterministic under load | 80 of 80 invocations green with 32 burners at parallelism 8 (`scripts/dev/stress-repro.py`) |
| AC-7 | No existing test weakened, no `// test-relax:` token, no product code changed | `git diff` over `internal/` is empty for this work |

## Checklist

### Goal Gates (MUST pass)
- [ ] A functional test proves the withdrawal against a running daemon
- [ ] The test is proven to discriminate, not merely to pass
- [ ] The RFC 9552 Section 5.2 requirement carries a tagged test

### TDD
- [ ] Tests written before the fix
- [ ] Tests FAIL without the fix
- [ ] Tests PASS with the fix
- [ ] `make ze-verify` green before commit

## Implementation Summary

### What Was Implemented

- `test/plugin/rfc9552-52-rs-opaque-withdraw-peer-down.ci`. Two route-server clients
  on SEPARATE `ze-peer` processes. The source announces one BGP-LS UPDATE holding two
  Link-State NLRIs, type 1 and type 99, then departs with a TCP close and no
  NOTIFICATION. The receiver asserts the whole MP_REACH on arrival and the EXACT
  MP_UNREACH frame after the departure. The tag is `RFC requirement: RFC9552-5.2-8
  positive`.
- The two waits that make the close reachable are BARRIERS an observer satisfies on
  demand, never assertions about ze. The first waits on `show bgp rs peers`, which is
  the route server's own forward-target view (`peerStatus`), because a client that
  announces before the other is a target has its UPDATE dropped and nothing replays
  it. The second holds the source open until ze has written the receiver the
  announcement.
- `rfc/requirements/rfc9552.md` regenerated, so `RFC9552-5.2-8` now carries functional
  evidence beside its two unit tags.

### Bugs Found/Fixed

None. The wire-visible defect was closed on 2026-08-08 by `408b8386c`. This spec adds
the tier of proof that defect never had, and it changes no product file.

### Documentation Updates

None owed. Every row of the Documentation Update Checklist above is No except row 9,
whose file is the generated `rfc/requirements/rfc9552.md`. The 39 doc anchors naming
`internal/component/bgp/plugins/rs/` point at files this work did not touch, and
`docs/architecture/api/update-syntax.md` already documents the `update hex ... nlri`
form the route server dispatches on peer-down.

### Deviations from Plan

- The closure run re-measured the discrimination with `go build -overlay` rather than
  by editing `server_inventory.go` in place. Several sessions build from this checkout,
  so a two-minute edit to a product file can redden another agent's gate. The overlay
  maps one path to a mutated COPY under `tmp/`, the working tree stays byte-clean, and
  the daemon the fixture drives is a real binary built from the mutated source.
- The spec was authored without the three planning checklists that `/ze-close` reads
  (Deliverables, Security Review, Documentation Update). They were written at closure
  from the finished work rather than the closure being abandoned.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| none | | | | |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A running daemon proves the withdrawal | Done | `test/plugin/rfc9552-52-rs-opaque-withdraw-peer-down.ci`, receiver rule at seq=2 | `ze-test bgp plugin --pattern rfc9552-52` passes in 7.3s |
| The proof discriminates | Done | same file | red against a daemon built with the `*nlri.WireNLRI` branch of `appendParsedRecords` removed |
| The RFC 9552 Section 5.2 obligation carries the tag | Done | the file header, and the `RFC9552-5.2-8` row of `rfc/requirements/rfc9552.md` | `make ze-rfc-check` resolves it as `functional/verify` |
| The two inherited traps are handled | Done | separate `ze-peer` processes, and both source-side waits are observer-satisfiable barriers | the shared `(conn, seq)` cursor cannot deadlock below the close |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | the receiver's seq=2 rule | the exact MP_UNREACH frame, asserted as `hex=`, not as a substring |
| AC-2 | Done | the same rule | both NLRIs, type 1 and type 99, byte-identical to the announcement |
| AC-3 | Done | the file header | `RFC requirement: RFC9552-5.2-8 positive` |
| AC-4 | Done | the overlay-built mutant run | `FAIL 482`, with `family not supported in text mode: bgp-ls/bgp-ls` in the daemon log |
| AC-5 | Done | the same run | the announcement rule still matched: the observer printed `OK: receiver was sent the BGP-LS announcement` before the red |
| AC-6 | Done | `scripts/dev/stress-repro.py` | 80 of 80 at parallelism 8 with 32 burners in the implementation phase. Re-measured at closure at 24 of 24, parallelism 4, 8 burners, `--any-failure`, on a box already carrying another session's suite |
| AC-7 | Done | `git status --porcelain -- internal/` and `audit-test-relaxation.py` | no product file of this work, no `// test-relax:` token |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestOpaqueNLRIRecordedAsSplitWireBytes` | Done, pre-existing | `internal/component/bgp/plugins/rs/server_opaque_withdrawal_test.go` | landed with the fix on 2026-08-08, unchanged here |
| `TestPeerDownWithdrawsOpaqueNLRIAsWireCommand` | Done, pre-existing | same file | unchanged here |
| The new functional `.ci` | Done | `test/plugin/rfc9552-52-rs-opaque-withdraw-peer-down.ci` | the tier this spec exists to add |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `test/plugin/rfc9552-52-rs-opaque-withdraw-peer-down.ci` | Done | new |
| `plan/spec-fixit-bgpls-withdrawal-functional-proof.md` | Done | this file |
| `rfc/requirements/rfc9552.md` | Done, generated | one row gains the functional tag |

### Audit Summary

- **Total items:** 7 acceptance criteria, 3 files
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** the discrimination method, recorded under Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A running daemon proves that a departing route-server client's BGP-LS routes are withdrawn from the surviving client | functional | `ze-test bgp plugin --pattern rfc9552-52 -v` prints `1/1 PASS 482`. The receiver's seq=2 rule is the exact 52-octet MP_UNREACH frame, AFI 16388 SAFI 71, carrying the 23 announced octets |
| The proof is not vacuous | mutation | a daemon built from a source overlay with the `*nlri.WireNLRI` branch of `appendParsedRecords` removed fails the same test at seq=2 only, logging `update-route failed ... error="family not supported in text mode: bgp-ls/bgp-ls"`. The announcement half stayed green in that run |
| The RFC 9552 Section 5.2 obligation has a tagged test at daemon tier | RFC gate | `make ze-rfc-check` exit 0: 2965 gated MUST-level requirements across 171 enrolled RFCs, 3539 tags resolved, `functional/verify 80` |

Interop: not owed. RFC 9552 Section 5.2 binds a PROPAGATOR, and this proof drives two
BGP speakers through ze's route server, which is the propagation path itself. No third
daemon originates or consumes BGP-LS in the interop lab, and ze originates none.

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| The opaque BGP-LS withdrawal fix is proven by UNIT tests only | done | this spec. `test/plugin/rfc9552-52-rs-opaque-withdraw-peer-down.ci` closes it, and the discrimination was measured both ways |
| `test/plugin/rib-graph.ci` never terminates | done, another spec | `spec-fixit-rib-graph-ci-never-terminates`, closed 2026-08-10 |
| `test/plugin/forward-overflow-two-tier.ci` failed once | done, another spec | `spec-fixit-forward-overflow-two-tier-flake`, closed 2026-08-15 |
| `learned_staleness.py` counts against the filesystem | assigned, still live | handed to another session on 2026-08-08. The shard is NOT removed at this closure: it still holds this live row |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-bgpls-withdrawal-functional-proof-55b89662-ed70-484b-8728-361629e96dbc.md` |
| `review_gate.py check` | OK, 1 code file, clean, hashes match |
| Rounds | 2. Round 1 ran two lenses over the whole change and found 0 BLOCKER and 0 ISSUE. Round 2 covered the comment correction round 1 asked for |
| Reviewer lenses used | A: wiring, logic, discrimination, project rules. B: security, edge cases, doc drift, RFC conformance, simplicity |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|

No BLOCKER and no ISSUE in either round. Ten NOTEs, of which two were factual
errors in the fixture's own hex annotation and are corrected: the NLRI-1 value was
written as 12 octets against a Length field of 11, and the annotation claimed ze
parses the type-1 NLRI when the default branch of `wireu.ParseNLRIs` hands the whole
section back as one `*nlri.WireNLRI`.

### Notes recorded, not acted on

| # | Note | Why it stands |
|---|------|---------------|
| 1 | The observer takes `before = receiver_updates(api)` and waits for a value above it. Every transient inside `peer_counter` reads as 0, so a snapshot could record 0 and let the announcement already counted satisfy the poll. An absolute `>= 2` is immune, as `test/plugin/relay-withdraw-nexthop-self.ci` does it | The failure direction is RED, never a silent pass, so it is a rare rerun and never a false conformance claim. The edit is a LOGIC change to an RFC-tagged test, which `.claude/hooks/pretool-writeedit.py` refuses without the owner's recorded approval. Put to the owner at closure |
| 2 | The observer's poll budget is about 55s against a 60s test timeout, so on a slow darwin run a real failure can hit the timeout before `runtime_fail` names the assertion | Same hook, same ask. Raising the three `timeout` values to 90s, as `test/plugin/ddos-*.ci` do, is the fix |
| 3 | `reject=stderr:pattern=ZE-OBSERVER-FAIL` is redundant: `checkObserverSentinel` runs on every test from `runner_exec.go`, and `observerSentinelInSyslog` covers the syslog route | Cosmetic, and the same hook governs it |
| 4 | The type-1 NLRI body is not a well-formed Node NLRI: RFC 9552 Section 5.2.1.2 makes Local Node Descriptors mandatory in all three NLRI types, and the 11-octet body carries none | Immaterial to what the test asserts, because ze frames every bgp-ls NLRI by length alone on the relay path. It would matter the day a dedicated bgp-ls arm is added to `ParseNLRIs`, and the annotation no longer claims the type is parsed |
| 5 | `check_evidence_ratchet` keys on `{rid: kind/tier}`, so `RFC9552-5.2-8` now carries a `functional/verify` floor forever. Renaming or deleting this `.ci`, or swapping it for a unit test, fails the ratchet even at an unchanged tag count | Correct and intended. Recorded so the next holder of this file knows what the tag now owes |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/rfc9552-52-rs-opaque-withdraw-peer-down.ci` | yes | `ls -la` reports 14K, and the runner lists it as plugin test 482 |
| `rfc/requirements/rfc9552.md` | yes | tracked, one row changed |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | the withdrawal reaches the surviving client | `1/1 PASS 482 rfc9552-52-rs-opaque-withdraw-peer-down` |
| AC-2 | both NLRIs are named | the seq=2 needle ends `0001000B020000000000000000000000630004DEADBEEF`, the same 23 octets the source announced |
| AC-3 | the tag is present | line 3 of the fixture, and `make ze-rfc-check` counts it as `functional/verify` |
| AC-4 | the test discriminates | the overlay-built mutant run: `FAIL 482`, `family not supported in text mode: bgp-ls/bgp-ls` |
| AC-5 | the red is confined to the withdrawal | the same run printed `OK: receiver was sent the BGP-LS announcement` before failing |
| AC-6 | deterministic under load | `stress-repro.py "bgp plugin" --test "--pattern rfc9552-52" --any-failure`: `not reproduced in 24 invocation(s) under load` |
| AC-7 | nothing weakened | `audit-test-relaxation.py` names 4 files, none of them this spec's |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| a route-server client's TCP close | `test/plugin/rfc9552-52-rs-opaque-withdraw-peer-down.ci` | yes. `action=close:conn=1:seq=4` is what the source ends on, `handleStateDown` drains that peer, and `sendBatchedWithdrawals` dispatches `update hex nlri`. The mutant run proves the assertion depends on that path and on no other |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| none declared | not applicable | the spec carries no Risks and Assumptions section, so there is no A-N row to resolve |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No doc anchor goes stale | `grep -rn "source: internal/component/bgp/plugins/rs" docs/ ai/` returns 39 anchors, every one naming `register.go`, `server.go`, `server_text.go`, `server_forward.go` or the package directory. This work changed no file under `internal/` | yes |
| The RFC 9552 published row stays correct | `docs/features/rfc-status.md` claims unknown NLRI types are propagated byte-identically and lists nine MUST gaps. Neither the claim nor the gap count changes, and `check_gap_count_agreement` pins that spelled number | yes |
| The command form the withdrawal uses is documented | `docs/architecture/api/update-syntax.md` carries `update hex [attr set ...] nlri <family> add <hex-nlri>` | yes |

## Core Insight

A test can be mutation-proven without editing the tree it lives in. `go build
-overlay` maps one source path to a mutated copy under `tmp/`, so the daemon under
test is a real binary built from broken source while the working tree stays
byte-clean. In a checkout several sessions build from, that difference is the whole
distance between an honest discrimination run and a red gate somebody else has to
explain.
