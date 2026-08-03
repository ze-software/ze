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

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

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
