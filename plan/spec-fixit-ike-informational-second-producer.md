# Spec: fixit-ike-informational-second-producer

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-07-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Found on 2026-07-30 by the phase-2b sweep for `plan/learned/1313-rfcgate-1b-rfc7296-pilot.md`.
Design: `tmp/uncovered-producers-design.md`, sections 1.1 and 2. Owner ruling the same
day: "Remove the fallthrough, and fix the leak with it."

## Task

**A cleartext IKE Delete tears an established tunnel down without any key material, and a
leaked SATable entry keeps that path reachable forever.**

`handleInformational` (`internal/component/ike/engine/inbound.go`) is a second handler for
post-establishment INFORMATIONAL messages. It takes no transport, so it answers nothing.
It also never decrypts. It walks `msg.Payloads`, which is the OUTER chain, and
`wire.Message.ReadFrom` does not decrypt an SK payload. A genuine encrypted INFORMATIONAL
therefore shows one entry, a `*wire.PayloadSK`, and no Delete is ever seen.

The reverse case is the defect. A **cleartext** datagram that names the right SPI pair and
carries a plaintext IKE Delete reaches `sa.State = StateDead` in `handleDeletePayload`. No
key material is consulted. An off-path attacker tears the tunnel down.

RFC 7296 Section 2.4 forbids this. `RFC7296-2.4-3` says an endpoint MUST NOT conclude the
peer failed from an unauthenticated message. Its tagged tests cover the IKE_SA_INIT
producer alone (`internal/component/ike/engine/responder_test.go`), so this second producer
is untagged and unproven.

**Why the path is live.** `routeInbound`
(`internal/component/ike/engine/register.go`) hands a packet to the owner loop only when
`ps.ownedSA.Load() == sa`. Every other packet goes to `handleInbound`
(`internal/component/ike/engine/fsm.go`), whose `case StateEstablished` calls
`handleEstablishedInbound`, which calls `handleInformational`.

**Why the window is unbounded.** `runInitiator`
(`internal/component/ike/engine/fsm.go`) never calls `table.Remove` on any exit path. Every
failed initiator cycle leaks one `StateEstablished` entry. `dispatchInbound` keeps routing
into that zombie, and `IPsecMetrics.Update` (`internal/component/ike/engine/metrics.go`)
counts it, so `ze_ipsec_tunnel_up` reports a dead peer as up.

## Required Reading

| Document | Why |
|----------|-----|
| `tmp/uncovered-producers-design.md` sections 1.1 and 2 | The trace, the four corrections, and the ruling |
| `rfc/full/rfc7296.txt` sections 1.4, 2.1 and 2.4 | The obligations in play |
| `internal/component/ike/engine/inbound.go` | The owner-loop handler that stays, and the dead-end handlers that go |
| `internal/component/ike/engine/register.go` | `routeInbound` and the queue-full drop |

## Current Behavior (MANDATORY)

Source files read on 2026-07-30:

- [ ] `internal/component/ike/engine/inbound.go`
- [ ] `internal/component/ike/engine/fsm.go`
- [ ] `internal/component/ike/engine/register.go`
- [ ] `internal/component/ike/engine/established.go`
- [ ] `internal/component/ike/engine/table.go`

**Behavior to preserve:** `handleOwnedInbound` and every handler it calls stay exactly as
they are. The responder path keeps its cached-response replay. The queue-full `default:`
drop in `routeInbound` keeps its current shape.

**Behavior to change:** an established-SA packet that the owner loop does not hold is
dropped with a log line instead of reaching a second handler. The initiator removes its SA
from the SATable when its lifecycle ends.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

A UDP datagram for an established SA, read by `dispatchInbound`
(`internal/component/ike/engine/register.go`) on the shared dispatch goroutine.

### Transformation Path

1. `dispatchInbound` looks the SA up in the SATable by SPI pair.
2. `routeInbound` compares `ps.ownedSA.Load()` with that SA.
3. On a match the packet goes to the owner loop, and `handleOwnedInbound` decrypts it.
4. Without a match the packet goes to `handleInbound`. After this work its
   `StateEstablished` arm drops the packet and records why.
5. The peer retransmits, and the retransmission reaches the owner loop.

### Boundaries Crossed

| Boundary | Direction | Carries |
|----------|-----------|---------|
| dispatch goroutine -> owner loop | in | Every post-establishment packet, through one handler |
| engine -> SATable | out | The initiator removal that ends the leak |

### Integration Points

`internal/component/ike/engine/metrics.go` counts SATable entries, so the removal corrects
`ze_ipsec_sa_count` and `ze_ipsec_tunnel_up` as well.

## Key Design Decisions

| Decision | Over | Because |
|----------|------|---------|
| Delete the second handler | Teaching it to answer | An answer writes `cacheResponse` and reads `SKKeys`. Those belong to `maintainSA` alone. An answer on the dispatch goroutine is the race the single-owner model removes |
| Drop the packet and let the peer retransmit | Blocking or queueing it | `RFC7296-2.1-7` makes the initiator retransmit. `routeInbound` already relies on that for its queue-full arm |
| Remove the initiator SA from the SATable | A pruner over the whole table | `runResponder` already removes its own SA the same way. One shape for both roles |
| Keep the queue-full drop as it is | A blocking send | A blocking send stalls every peer on the shared goroutine |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A legitimate peer request is dropped in the establish window and answered on the retransmit, about 500ms later |
| How is it reverted? | A single commit revert |
| Who else touches this path? | `internal/component/ike/engine/inbound.go` and `rekey.go` are under concurrent edit. Anchor every change on the function name |

## Risks & Assumptions

| Id | Statement | Basis | Validation |
|----|-----------|-------|------------|
| A-1 | Nothing outside `inbound.go` calls the deleted functions | A grep over `internal/` returns one caller each | The build after the deletion |
| A-2 | No test references the deleted functions | A grep over `--include=*_test.go` returns only the owned variants | `go test ./internal/component/ike/...` |
| A-3 | A deferred `table.Remove` reads the responder SPI at return time | `handleSAInitResponse` fills it and re-keys the table through `UpdateKey` | A test that runs one initiator cycle to failure and asserts an empty table |
| R-1 | A double removal | `SATable.Remove` deletes a map key and returns the old value, so a second call is a no-op |
| R-2 | A duplicate `sa-down` event | `runInitiator` owns its SA, and `runResponder` handles the other role. The removal clears `ps.sa` the way `runResponder` does |

## Acceptance Criteria

| Id | Criterion |
|----|-----------|
| AC-1 | A cleartext INFORMATIONAL carrying a plaintext IKE Delete leaves an established, unowned SA in `StateEstablished` |
| AC-2 | The same packet draws no datagram |
| AC-3 | An encrypted INFORMATIONAL request to an unowned SA writes no owner-only state |
| AC-4 | The same Delete, authenticated and delivered on the owner loop, still reaches `StateDead` |
| AC-5 | An initiator cycle that ends leaves no SATable entry |
| AC-6 | The queue-full drop carries an RFC 7296 Section 2.1 citation at the site |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | | Feature Code | Test |
|-------------|---|--------------|------|
| `handleInbound` with an established, unowned SA (`fsm.go`) | -> | the drop arm | `TestRteUnownedEstablishedSATrustsNothing` |
| `runInitiator` returning (`fsm.go`) | -> | the SATable removal | `TestRteInitiatorCycleLeavesNoTableEntry` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | Proves |
|------|--------|
| `TestRteUnownedEstablishedSATrustsNothing` | AC-1, AC-2, AC-3, AC-4 |
| `TestRteInitiatorCycleLeavesNoTableEntry` | AC-5 |

### Functional Tests

| Test | Role |
|------|------|
| `test/ipsec/ipsec-sa-installed.ci` | Proves an SA still establishes with the routing change |
| `test/ipsec/ipsec-clear-reestablish.ci` | Proves teardown and re-establishment still work |

The ipsec suite runs on every push, because `ipsec` is in `all_suites`
(`mk/test-functional.mk`).

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/ike/engine/inbound.go` | Delete the three dead-end handlers |
| `internal/component/ike/engine/fsm.go` | Drop an established-SA packet the owner loop does not hold, and remove the initiator SA from the SATable |
| `internal/component/ike/engine/register.go` | Cite RFC 7296 Section 2.1 at the queue-full drop |

## Files to Create

| File | Holds |
|------|-------|
| `internal/component/ike/engine/rfc7296_routing_test.go` | The tagged pair and the SATable test |

## Implementation Steps

1. Write the failing tests, and record the red output.
2. Delete the three dead-end handlers, and drop the packet in `handleInbound`.
3. Remove the initiator SA from the SATable when its lifecycle ends.
4. Cite RFC 7296 Section 2.1 at the queue-full drop.
5. Confirm green, then mutation-verify the tagged pair and the removal.

## Test Evidence

Tests FAIL, before the fix (`go test -run TestRte ./internal/component/ike/engine/ -v`):

```
=== RUN   TestRteUnownedEstablishedSATrustsNothing/cleartext_IKE_Delete
    rfc7296_routing_test.go:94: the SA moved to dead, want it left established
--- FAIL: TestRteUnownedEstablishedSATrustsNothing (0.34s)
    --- FAIL: TestRteUnownedEstablishedSATrustsNothing/cleartext_IKE_Delete (0.11s)
    --- PASS: TestRteUnownedEstablishedSATrustsNothing/protected_INFORMATIONAL_request (0.12s)
    --- PASS: TestRteUnownedEstablishedSATrustsNothing/cleartext_CREATE_CHILD_SA (0.05s)
    --- PASS: TestRteUnownedEstablishedSATrustsNothing/protected_IKE_Delete_on_the_owner_loop (0.05s)
=== RUN   TestRteInitiatorCycleLeavesNoTableEntry
    rfc7296_routing_test.go:145: the ended initiator cycle left 1 SATable entries, want 0
--- FAIL: TestRteInitiatorCycleLeavesNoTableEntry (0.02s)
FAIL
```

Tests PASS, after the fix:

```
--- PASS: TestRteUnownedEstablishedSATrustsNothing (0.11s)
    --- PASS: TestRteUnownedEstablishedSATrustsNothing/cleartext_IKE_Delete (0.03s)
    --- PASS: TestRteUnownedEstablishedSATrustsNothing/protected_INFORMATIONAL_request (0.03s)
    --- PASS: TestRteUnownedEstablishedSATrustsNothing/cleartext_CREATE_CHILD_SA (0.03s)
    --- PASS: TestRteUnownedEstablishedSATrustsNothing/protected_IKE_Delete_on_the_owner_loop (0.03s)
=== RUN   TestRteInitiatorCycleLeavesNoTableEntry
--- PASS: TestRteInitiatorCycleLeavesNoTableEntry (0.01s)
PASS
ok  	github.com/ze-software/ze/internal/component/ike/engine	0.279s
```

Three mutations, one per claim. Each was reverted, and the tests are green again.

| Mutation | Red | Green |
|----------|-----|-------|
| The `StateEstablished` arm of `handleInbound` sets `StateDead` from the outer chain again | `cleartext IKE Delete`, the `RFC7296-2.4-3 positive` case | the other three cases |
| `handleDeletePayload` no longer writes `StateDead` | `protected IKE Delete on the owner loop`, the `RFC7296-2.4-3 negative` case | the other three cases |
| The `table.Remove` of `runInitiator` is removed | `TestRteInitiatorCycleLeavesNoTableEntry` | every case of the routing test |

The first two are the pair, and each polarity fails alone. A single mutation does not carry
both, so neither case is a passenger of the other.

## Goal Gates

`make ze-verify`.

## Quality Gates

`make ze-lint-changed`, `make ze-rfc-check`, `python3 scripts/dev/ste_check.py --check`.

## RFC Documentation (Scope: protocol)

No new row. This work gives `RFC7296-2.4-3` a second proven producer, and the id keeps its
text. `RFC7296-1.4-4` keeps its current evidence, which is `handleInformationalOwned`.

## Checklist

- [ ] Tests written first, run, and Tests FAIL output pasted into the spec
- [ ] The three dead-end handlers are gone
- [ ] The initiator removes its SATable entry
- [ ] Tests PASS output pasted into the spec
- [ ] The tagged pair mutation-verified
- [ ] `make ze-verify` green

## Known Limitations

A legitimate peer request that arrives in the establish window is dropped. The peer
retransmits it, so the answer is late and never lost.

---

## Implementation Summary

### What Was Implemented
- The three dead-end handlers are gone. `grep -rn "func handleInformational\b|func
  handleEstablishedInbound|func handleDeletePayload\b"` over `internal/component/ike/`
  returns nothing.
- `handleInbound`'s `case StateEstablished` (`internal/component/ike/engine/fsm.go`) drops the
  packet and logs why. `handleOwnedInbound` is now the only handler for a post-establishment
  message, and it decrypts first.
- `runInitiator` (`fsm.go`) carries `defer table.RemoveByPeer(sa.PeerName)`, registered AFTER
  `defer sa.forgetKeys()` so it RUNS BEFORE it.
- `routeInbound` (`internal/component/ike/engine/register.go`) cites RFC 7296 Section 2.1 at
  its queue-full drop, and says which direction the drop costs.

### Bugs Found/Fixed
- A cleartext datagram naming the right SPI pair with a plaintext IKE Delete reached
  `sa.State = StateDead`. Covered by the `cleartext IKE Delete` case of
  `TestRteUnownedEstablishedSATrustsNothing`, which is the `RFC7296-2.4-3 negative` binding.
- Every failed initiator cycle leaked one `StateEstablished` SATable entry, so
  `ze_ipsec_tunnel_up` reported a dead peer as up. Covered by
  `TestRteInitiatorCycleLeavesNoTableEntry` and, for a cycle that actually established,
  by `TestIcyEstablishedCycleLeavesNoTableEntry` (`initiator_cycle_test.go`).

### Documentation Updates
- No doc change needed. `grep -rln "source: internal/component/ike/engine/inbound.go" docs/`
  returns nothing. `docs/features/rfc-status.md`'s RFC 7296 row already states it: "Section
  2.4 refuses a verdict about the peer from an unauthenticated message, because an
  established SA is served by one handler that decrypts first."
- `docs/guide/ipsec.md` and `docs/DESIGN.md` carry `source:` anchors for
  `internal/component/ike/engine/register.go`. Both were re-read: neither describes
  `routeInbound`'s queue-full arm, which is the only part of that file this spec changed.

### Deviations from Plan
- None. The three handlers named in Files to Modify were the three that existed.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The first shape of the SATable removal was a deferred `table.Remove(sa.InitiatorSPI, sa.ResponderSPI)` | Go evaluates a deferred call's ARGUMENTS where the defer is written, so the responder SPI was captured as the zero `newInitiatorSA` left, and the removal deleted a key `UpdateKey` had already replaced | Reading A-3's validation: a cycle that establishes and then ends | Removed by PEER NAME instead. The 10-line comment at the defer records why |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| No post-establishment message may reach a handler that has not decrypted | Done | `fsm.go` `handleInbound`, `case StateEstablished` | The three dead-end handlers are deleted, not guarded |
| An initiator cycle must not leak an SATable entry | Done | `fsm.go` `runInitiator`, `defer table.RemoveByPeer` | |
| The queue-full drop must carry its RFC citation | Done | `register.go` `routeInbound` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRteUnownedEstablishedSATrustsNothing/cleartext_IKE_Delete` asserts the SA stays `StateEstablished` | The drop arm reads nothing from the outer chain |
| AC-2 | Done | The same case asserts no datagram is written | The arm has no transport in hand and calls only `log.Debug` |
| AC-3 | Done | `.../protected_INFORMATIONAL_request` | No owner-only state (`cacheResponse`, `SKKeys`) is touched |
| AC-4 | Done | `.../protected_IKE_Delete_on_the_owner_loop` reaches `StateDead` | This is the `RFC7296-2.4-3 positive` binding: the SAME bytes, authenticated, still work |
| AC-5 | Done | `TestRteInitiatorCycleLeavesNoTableEntry` (stopped cycle) and `TestIcyEstablishedCycleLeavesNoTableEntry` (established cycle, and an IKE-rekeyed one) | Two files. See the Review Gate note on which one discriminates the by-name choice |
| AC-6 | Done | `register.go` `routeInbound`, the `default:` arm of the select: "RFC 7296 Section 2.1 makes that survivable in both directions, but only because both are retransmitted" | It also names the case the citation does NOT cover: a self-initiated request that nothing repeats |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRteUnownedEstablishedSATrustsNothing` | Done | `internal/component/ike/engine/rfc7296_routing_test.go` | Four sub-cases; carries both polarities of `RFC7296-2.4-3` |
| `TestRteInitiatorCycleLeavesNoTableEntry` | Done | same file | Proves the removal exists |
| `TestIcyEstablishedCycleLeavesNoTableEntry` | Done (beyond plan) | `internal/component/ike/engine/initiator_cycle_test.go` | Proves the removal is correct for a cycle that reached `UpdateKey`, and for an IKE-rekeyed one |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/ike/engine/inbound.go` | Done | Three handlers deleted |
| `internal/component/ike/engine/fsm.go` | Done | Drop arm, plus the deferred removal |
| `internal/component/ike/engine/register.go` | Done | The Section 2.1 citation |
| `internal/component/ike/engine/rfc7296_routing_test.go` | Done | Created |

### Audit Summary
- **Total items:** 16 (3 requirements, 6 ACs, 3 tests, 4 files)
- **Done:** 16
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An off-path attacker must not tear an established tunnel down with a cleartext Delete | functional, negative | `TestRteUnownedEstablishedSATrustsNothing/cleartext_IKE_Delete`. It discriminates: restoring the outer-chain `StateDead` write reddens exactly this case, and the mutation table records it |
| The same Delete, authenticated, must still work | functional, positive | `.../protected_IKE_Delete_on_the_owner_loop`. Removing `StateDead` from `handleDeletePayload` reddens exactly this case, so the two polarities are not passengers of each other |
| No second producer may exist for a post-establishment message | structural | `grep -rn "func handleInformational\b\|func handleEstablishedInbound\|func handleDeletePayload\b" --include=*.go internal/component/ike/` returns nothing. A guard would have left the producer reachable; deletion does not |
| A failed initiator cycle must leave no SATable entry | functional | `TestRteInitiatorCycleLeavesNoTableEntry` for a cycle that never started; `TestIcyEstablishedCycleLeavesNoTableEntry` for one that established and one that was IKE-rekeyed. The second is what discriminates removal-by-name from removal-by-SPI-pair |
| The daemon still establishes and re-establishes with the routing change | functional (`.ci`) | `test/ipsec/ipsec-sa-installed.ci`, `test/ipsec/ipsec-clear-reestablish.ci`, both in the `ipsec` suite `mk/test-functional.mk` runs on every push |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| (no shard) | n/a | The spec's metadata row is `-` and no `plan/deferrals/fixit-ike-informational-second-producer.md` exists. This spec deferred nothing |

No shard is removed by this closure. No FOREIGN shard was emptied by it.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-ike-informational-second-producer-c4c78ddb-c47b-4f1a-a85d-5911d7c65455.md` |
| `review_gate.py check` | clean (`review_gate: OK (0 code files, clean, hashes match ...)`) |
| Reviewer lenses used | Three independent subagents through `/ze-review`: logic+wiring+removed-behaviour; security+edge-cases+test-vacuity; RFC 7296 conformance+interop |

**Round 1 scope, written before the round ran:** the whole five-spec IKE changeset at HEAD
(`git log --oneline -14 -- internal/component/ike/`), including the sibling call sites of
every changed function. Three lenses, because the ask was "re-verify before closing".

### Findings fixed
<!-- Only BLOCKER and ISSUE. Every finding was reproduced against source by the
     closing agent before it was graded; a reviewer can be wrong. -->
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE, **refuted on verification** | `TestRteInitiatorCycleLeavesNoTableEntry` closes `stopCh` before `runInitiator`, so the cycle never reaches `handleSAInitResponse`, the responder SPI is still zero, and a deferred by-SPI-pair removal would pass identically. The reviewer concluded AC-5 had no discriminating test | `internal/component/ike/engine/rfc7296_routing_test.go` | Nothing to fix. The reading of THAT test is right; the conclusion is not. `TestIcyEstablishedCycleLeavesNoTableEntry` (`internal/component/ike/engine/initiator_cycle_test.go`) already drives a full established cycle AND an IKE-rekeyed one through `icyRunCycle`, and its docstring names the by-SPI-pair trap verbatim. Both sub-cases pass. The reviewer searched only the test file this spec names, which is why the spec now names the sibling file too |

Verified clean by the logic/wiring lens on this spec's own hunts: no behaviour was lost with
the three deleted handlers, and `defer table.RemoveByPeer` cannot reach another cycle's SA
because `matchResponderPeer` (`register.go`) matches only `ConnectionRespond` peers. Verified
CONFORMANT by the RFC lens: the drop arm concludes nothing from an unauthenticated datagram,
and the test drives it from the entry point while showing the same bytes authenticated still
reach `StateDead`.

No BLOCKER and no surviving ISSUE, so the loop ended at round 1.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/ike/engine/rfc7296_routing_test.go` | Yes | `ls -la`: 6065 bytes |
| `internal/component/ike/engine/initiator_cycle_test.go` | Yes | `grep -n "^func "` returns `icyRunCycle`, `TestIcyEstablishedCycleLeavesNoTableEntry`, `TestIcyRekeyRepointsTheSessionAtTheNewSA` |
| `test/ipsec/ipsec-sa-installed.ci` | Yes | `ls test/ipsec/` |
| `test/ipsec/ipsec-clear-reestablish.ci` | Yes | `ls test/ipsec/` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-4 | The unowned established arm trusts nothing, and the owner loop still works | `--- PASS: TestRteUnownedEstablishedSATrustsNothing (0.14s)`. Read `fsm.go` `case StateEstablished`: the whole arm is one `log.Debug` |
| AC-5 | No SATable entry survives a cycle | `--- PASS: TestRteInitiatorCycleLeavesNoTableEntry (0.01s)`; `--- PASS: TestIcyEstablishedCycleLeavesNoTableEntry/a_plain_established_cycle (0.04s)`; `--- PASS: .../a_cycle_whose_IKE_SA_was_rekeyed (0.06s)` |
| AC-5 (producer) | The removal is by peer name and deferred | Read `fsm.go` `runInitiator`: `defer table.RemoveByPeer(sa.PeerName)`, registered after `defer sa.forgetKeys()` |
| AC-6 | The queue-full drop carries the citation | Read `register.go` `routeInbound`: the `default:` arm names RFC 7296 Section 2.1 and states which direction the drop costs |
| (deletion) | The three handlers are gone | `grep -rn "func handleInformational\b\|func handleEstablishedInbound\|func handleDeletePayload\b" --include=*.go internal/component/ike/` returns nothing |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `handleInbound` with an established, unowned SA -> the drop arm | `test/ipsec/ipsec-clear-reestablish.ci` | Read the file: it clears an SA and requires re-establishment, which drives the parallel half-open path `routeInbound` distinguishes. The unowned-established case itself is proven in Go, since no `.ci` can force the owner loop not to hold an SA |
| `runInitiator` returning -> the SATable removal | `test/ipsec/ipsec-sa-installed.ci` | Read the file: it establishes an SA and asserts the dataplane state, so a leaked entry would not be visible there. The removal is proven in Go by the two cycle tests, one of which drives a real handshake through `icyRunCycle` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | The build is green after the deletion: `ok github.com/ze-software/ze/internal/component/ike/engine` |
| A-2 | confirmed | No test references the deleted functions; the package compiles and its tests pass |
| A-3 | **broken** | A deferred `table.Remove` does NOT read the responder SPI at return time. Go evaluates a deferred call's arguments where the defer is written, so it captured the zero. Recorded in the Mistake Log; the fix is the removal by peer name |
| R-1 | confirmed harmless | `SATable.Remove` deletes a map key, so a second call is a no-op. `RemoveByPeer` returns a count and is idempotent the same way |
| R-2 | confirmed harmless | `runInitiator` owns its SA and `runResponder` handles the other role. `register.go` `matchResponderPeer` matches only `ConnectionRespond` peers, so a `ConnectionInitiate` session never shares a peer name with a responder SA |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| RFC compliance disclosure for Section 2.4 | `docs/features/rfc-status.md` RFC 7296 row states it verbatim | Yes |
| `docs/guide/ipsec.md` and `docs/DESIGN.md` anchor `register.go` | Both re-read on 2026-08-03. Neither describes `routeInbound`'s queue-full arm, the only part of that file this spec changed | Yes |
| No anchored claim points at `inbound.go` or `fsm.go`'s changed arm | `grep -rln "source: internal/component/ike/engine/inbound.go" docs/` returns nothing; `fsm.go`'s anchor in `docs/guide/ipsec.md` describes the handshake, not the drop arm | Yes |
| Metrics | `ze_ipsec_sa_count` and `ze_ipsec_tunnel_up` become CORRECT rather than changing meaning, so no doc claim about them is now stale | Yes |
| Config syntax, CLI, API, plugin SDK, wire format, comparison table | No change | Yes |

## Core Insight

The decrypt was not missing from `handleInformational`. `handleInformational` could never
have had one: it walked `msg.Payloads`, which `wire.Message.ReadFrom` fills from the OUTER
chain, and a real peer's Delete lives inside the SK payload. So the only payloads it could
ever see were the ones an attacker had sent in the clear. A handler that reads the outer
chain after establishment is a cleartext-only handler whatever its author intended, and that
is the shape to hunt for. Adding a decrypt would have made it a second owner of `SKKeys` on
the wrong goroutine; the fix was to have one producer, not a safer second one.
