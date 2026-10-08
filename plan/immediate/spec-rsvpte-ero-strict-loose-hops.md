# Spec: RSVP-TE strict-hop adjacency and loose-hop expansion (RFC 3209 Sections 4.3.3.1, 4.3.4.2)

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-08 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later session reviews that commit and closes, on Opus when the model is an Anthropic one. `-` closes in the same session. -->

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     The file's DIRECTORY carries the release bucket: plan/immediate/ for a defect
     an operator meets, plan/pre-release/ for work the release cannot go out
     without, plan/ for everything else (plan/README.md). -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The started implementation replaces the old ERO-only next-hop selection with `internal/plugins/rsvpte/routing.go::resolveExplicitPath` and the transport's native route query. Strict hops must resolve to an adjacent member of their abstract node. Loose hops insert the resolved adjacent next hop before the remaining loose subobject; expansion is bounded. `resolveReceivedPath` validates and removes local subobjects before forwarding. This is source coverage awaiting Main's strict/loose wire scenarios and native RFC discrimination, not a claim of CSPF or full RFC 3209 support.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC3209-4.3.3.1-1 | "The path between a strict node and its preceding node MUST include only network nodes from the strict node and its preceding abstract node." (§4.3.3.1) | `routing.go::resolveExplicitPath` rejects a strict subobject whose resolved next hop is outside that abstract node |
| RFC3209-4.3.4.2-1 | "Each subobject in this series MUST denote an abstract node that is a subset of the current abstract node." (§4.3.4.2) | `routing.go::resolveReceivedPath` removes locally satisfied subobjects; `resolveExplicitPath` expands the remaining route using native adjacency information; source and error paths require validation |

## Required Reading

| Source | Why |
|--------|-----|
| `rfc/full/rfc3209.txt` Sections 4.3.3.1, 4.3.4.1, 4.3.4.2 | The two sentences the rows quote, and step 5b for loose hops |
| `docs/contributing/rfc-conformance-gates.md` "The discrimination record" | Tag form, record fields, the revert route |
| `docs/architecture/rsvpte/mpls-rsvp-te.md` | Configured explicit routing |

## Current Behavior

- [ ] `internal/plugins/rsvpte/routing.go` -- `resolveReceivedPath`, `resolveExplicitPath`
- [ ] `internal/plugins/rsvpte/peer_identity.go` -- `peerInPrefix`, abstract-node membership
- [ ] `internal/plugins/rsvpte/engine.go` -- the PATH handler turning a selection error into PathErr 24

The producer was committed in 2dc7ec1802; this work adds proof only.

## Data Flow

### Entry Point
An RSVP PATH carrying an EXPLICIT_ROUTE object reaching `engine.handlePacket` at a transit.

### Transformation Path
`DecodeMessage` -> `resolveReceivedPath` -> `resolveExplicitPath` (native `Transport.ResolveRoute`) -> forwarded PATH via `SendPath`, or `sendPathErr` with Routing Problem.

### Boundaries Crossed
Wire bytes in and out; the native route query on the transport.

### Integration Points
`transport.ResolveRoute`, `transport.SendPath`, `engine.sendPathErr`.

## Wiring Test

| Entry Point | -> Feature Code | -> Test |
|-------------|-----------------|---------|
| `engine.handlePacket` with an encoded PATH | -> `routing.go::resolveExplicitPath` | -> `rfc3209_explicit_hops_test.go`, decoding the PATH or PathErr the fake transport sent |

## 🧪 TDD Test Plan

### Unit Tests

| Test | Requirement | Polarity |
|------|-------------|----------|
| `TestRFC3209StrictHopReachedDirectly` | RFC3209-4.3.3.1-1 | positive |
| `TestRFC3209StrictHopThroughOutsideNodeRefused` | RFC3209-4.3.3.1-1 | negative |
| `TestRFC3209InteriorHopStaysInAbstractNode` | RFC3209-4.3.4.2-1 | positive |
| `TestRFC3209InteriorHopLeavingAbstractNodeRefused` | RFC3209-4.3.4.2-1 | negative |
| `TestRFC3209LooseHopExpandedWithNextHop` | none (step 5b contrast) | untagged |

## Files to Modify

| File | Change |
|------|--------|
| `internal/plugins/rsvpte/rfc3209_explicit_hops_test.go` | New: the five wire tests |
| `rfc/discrimination/rfc3209.json` | Four revert-route records |
| `rfc/short/rfc3209.md` | Two rows lose `{gap}`; Support remaining updated |

## Implementation Steps

1. Write each test as a received PATH with a fixture route, decoding what the transit sends.
2. Record each tag with `./le rfc discriminate-record` (revert route).
3. Prove the strict refusal by Go overlay mutation of the `!inside && !target.Loose` guard.
4. Remove the two `{gap}` annotations.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL under a break of the producer (the discrimination records)
- [ ] Tests PASS
- [ ] `./le verify worktree` (owed by the main thread)

### Integration Checklist

- [ ] `./le rfc check` reports no stale or owed record for RFC 3209

### Documentation Update Checklist

- [ ] No page describes behavior this work changed: no producer code changed

## Risks & Assumptions

| Item | Note |
|------|------|
| Not CSPF | Next-hop selection is the native route query; no TE path computation is claimed |
| Interop | Independent RSVP-peer interoperability for strict and loose EROs remains unproven |

## Acceptance Criteria

Evidence: `internal/plugins/rsvpte/rfc3209_explicit_hops_test.go`, which drives
a PATH into a transit and decodes the PATH or PathErr it sends. Each tagged
test carries a revert-route record in `rfc/discrimination/rfc3209.json`.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 (RFC3209-4.3.3.1-1) | ERO [self/32, strict 10.0.0.9/32] with native next hop 10.0.0.9; separately, next hop 10.0.0.7, outside both nodes | PATH sent to 10.0.0.9 with ERO [10.0.0.9/32] and no PathErr (`TestRFC3209StrictHopReachedDirectly`); no PATH, and PathErr 24/2 to the ingress (`TestRFC3209StrictHopThroughOutsideNodeRefused`) |
| AC-2 (RFC3209-4.3.4.2-1) | ERO [10.1.0.0/16 holding the transit, strict 10.9.0.9/32] with next hop 10.1.0.2 inside the /16; separately, next hop 10.2.0.2 outside it | PATH sent to 10.1.0.2, every subobject ahead of the strict node a subset of 10.1.0.0/16 (`TestRFC3209InteriorHopStaysInAbstractNode`); no PATH, and PathErr 24/2 (`TestRFC3209InteriorHopLeavingAbstractNodeRefused`) |
| AC-3 (loose hop, Section 4.3.4.1 step 5b) | The AC-1 refusal topology with the L bit set | PATH sent to 10.0.0.7 with ERO [10.0.0.7/32, loose 10.0.0.9/32] (`TestRFC3209LooseHopExpandedWithNextHop`, untagged because no MUST row binds it) |
| AC-4 | `rfc/short/rfc3209.md` | Both rows carry no `{gap}`; Support remaining no longer calls their proof open |

Semantic discrimination beyond the revert records, by Go overlay on 2026-10-08:
disabling the `!inside && !target.Loose` refusal in `resolveExplicitPath`
reddens both negative tests. No producer defect was exposed and no producer
code changed.

## Closure Review (2026-10-08): closure withheld on interop

Independent review of 48c344b081 by a session that wrote none of it.

| Check | Result |
|-------|--------|
| Claim against assertion, 4 tagged tests | Each `RFC requirement:` claim states what its test body asserts and no more; the untagged loose test asserts step 5b expansion only |
| RFC quotes | Both quotes found verbatim in `rfc/full/rfc3209.txt` (Sections 4.3.3.1, 4.3.4.2) |
| `./le rfc check` | No violation names rfc3209 |
| Scoped tests | `go test ./internal/plugins/rsvpte/` green, `go vet` clean |

**Interop gap: this spec does not close.** `ai/rules/interop-and-goal-validation.md`
requires a scenario against another implementation, and this spec's own Risks
row says strict and loose ERO interop is unproven. The only RSVP-TE carrier,
`freertr_interop_integration_linux_test.go::TestRSVPFreeRouterInterop`, makes
Ze the egress: the peer's ERO ends at Ze, so no strict or loose transition is
resolved at a Ze transit. Scenario needed: freeRouter ingress signalling
through Ze as transit to a second node, asserting at the peers (1) a strict hop
to a node Ze reaches directly: the downstream peer receives the PATH with the
trimmed ERO and the LSP comes up, (2) a strict hop whose native next hop is
outside both abstract nodes: the ingress peer receives PathErr 24/2, and (3)
the same hop loose: the downstream peer receives the PATH with the expanded
ERO. The carrier needs root, so it was not run in this review.

## Interop status (2026-10-08): loose expansion proven at the peer, strict has no originator

`./le test integration interop-rsvpte` (`internal/le/interoplab/rsvpte/`,
catalog `rsvpte/transit-loose-ero-expansion`) runs freeRouter ingress, Ze
transit and freeRouter egress in Docker without root.

| Needed | Status |
|--------|--------|
| (3) loose hop expanded | Observed at the egress's own capture: the ingress sends `[Ze loose, 198.51.100.4 loose]`, the egress receives `[172.29.81.14 strict, 198.51.100.4 loose]`. Discrimination: with the replacement in `resolveExplicitPath` disabled the check goes red on the ERO (`rsvpte-run5-break.log`), restored it passes the ERO check |
| LSP comes up | Red: Ze's transit swap install fails (EINVAL, Linux AF_MPLS refuses the path MTU `addMPLSSwap` attaches) and Ze sends ResvErr code 22, `plan/journal/kernel-refuses-what-the-installer-sends.md` |
| (1) strict direct hop, (2) strict hop outside -> PathErr 24/2 | No originator: freeRouter encodes every ERO hop it originates as loose (`clntMplsTeP2p.workDoer`, `ipFwdTab.fillRsvpPack`, pinned revision and upstream master) |

Closure needs the owner to decide the path-MTU fix and which peer originates a
strict ERO.
