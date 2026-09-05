# Spec: the relayed RFC 2545 next hop, and three more relay gaps

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`spec-fixit-bgp-egress-rail-divergence` recorded four smaller relay gaps on
2026-07-25 and closed without them. Its spec file is no longer on disk. This spec
is their destination. The four are stated below in the row's own words, then
against what the tree does today.

**The row, verbatim.** "Four smaller relay gaps: RFC 2545 32-byte next hop
(global + link-local) truncated to 16. **CITATION CORRECTED 2026-08-07, verified
at the producer: it is NOT `nhopHexFromAddr`,** which takes one `netip.Addr` and
truncates nothing. The link-local is dropped one layer earlier by
`MPReachWire.NextHop` (`internal/component/bgp/wireu/mpwire.go`), whose `afi == 2
&& nhLen >= 16` branch copies `nhBytes[:16]` under the comment 'may have
link-local after, take first'. Start the fix there and at the stored
representation. RFC 2545 is now enrolled (`rfc/short/rfc2545.md`), and Section 3
binds this: the link-local 'shall be included ... if and only if' the speaker
shares a subnet with both the next-hop entity and the peer. Today the loss is
observability only, because `RawRoute.NHopHex` is read at two command-output
sites and never re-encoded onto the wire; it becomes a Section 3 violation the
moment a stored route is RELAYED in the route-server case, which is the one
deployment that sends the 32-octet form at all; complex families
(VPN/EVPN/Flowspec) store the WHOLE MP_REACH NLRI block so a replay re-announces
every NLRI of the originating UPDATE; no backpressure on in-flight relays (each
pins a read-pool buffer); `Coordinator.RelayStoredRoute` has no test."

The row's destination note, verbatim: "Thomas: one spec for (c), which is now
unblocked; (d) is largely answered by the congestion controller". The letters are
the source spec's own lettering, not a lettering of the list above.

**What the tree does today, verified at the producers on 2026-09-05.** Two halves
of the first gap have landed since the row was written, and the row's premise
about it has gone stale in one direction and turned live in another.

Landed: the STORED representation now keeps the whole field.
`AdjRIBInManager` stores `hex.EncodeToString(mpReach.NextHopBytes())`
(`internal/component/bgp/plugins/adj_rib_in/rib.go`), `NextHopBytes` returns the
Network Address of Next Hop field entire, and `RawRoute.NHopHex` documents itself
as "the WHOLE next-hop field as the source framed it", 4 to 32 octets.
`MPReachWire.NextHop` still copies `nhBytes[:16]`, and now says so: its doc
comment directs a caller that will put the bytes back on the wire to
`NextHopBytes` instead. There is a functional test
(`test/plugin/adj-rib-in-replay-rfc2545-next-hop.ci`) and an interop scenario
(`bgp-rfc2545-linklocal-nexthop-frr`, `internal/le/interoplab/bgp/check_rfc.go`)
carrying RFC2545-3-2 and RFC2545-3-3 tags in both arms.

Stale: "read at two command-output sites and never re-encoded onto the wire" is
no longer true. `relayPayload` (`internal/component/bgp/reactor/reactor_api_relay.go`)
hex-decodes `route.NextHopHex` into the relayed UPDATE.

Live, and the reason this spec exists: the relay re-encodes the stored field
VERBATIM and never asks Section 3's question about the destination peer. RFC 2545
Section 3 says the link-local "shall be included in the Next Hop field if and
only if the BGP speaker shares a common subnet with the entity identified by the
global IPv6 address carried in the Network Address of Next Hop field and the peer
the route is being advertised to." Ze holds that decision in `linkScope`
(`internal/component/bgp/reactor/link_scope.go`): `newLinkScopeFrom` settles the
peer half once per session against the interface table, and `linkLocalNextHop`
answers the next-hop half per route. `relayPayload` consults neither. So a route
stored with a 32-octet next hop is relayed with its link-local half to a peer
that shares no subnet with it, which is the "in all other cases" branch Section 3
forbids. That is the route-server case the row predicted, and it has arrived.

The other three gaps stand as written and were not re-verified in depth:

| Gap | State |
|-----|-------|
| Complex families (VPN, EVPN, Flowspec) store the whole MP_REACH NLRI block, so a replay re-announces every NLRI of the originating UPDATE | `installComplexNLRIs` and the `NLRIFramingSourceWire` branch in `rib.go` still store one whole NLRI section per route |
| No backpressure on in-flight relays: each pins a read-pool buffer | Thomas's note says the congestion controller largely answers this |
| `Coordinator.RelayStoredRoute` (`internal/component/plugin/coordinator.go`) has no test | Unchanged |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - the RFC 2545 Section 3 link-local next-hop condition, which `link_scope.go` declares
  → Constraint: the peer half is settled per session, the next-hop half per route
- [ ] `docs/architecture/wire/` - the MP_REACH_NLRI next-hop field
  → Constraint: [fill at design time]

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2545.md` - Section 3, the if-and-only-if condition
  → Constraint: the link-local is included only when the speaker shares a subnet with BOTH the next-hop entity and the peer
- [ ] `rfc/short/rfc4760.md` - the Network Address of Next Hop field
  → Constraint: [fill at design time]

**Key insights:** (minimal context to resume after compaction)
- The storage half is done; the relay half is not
- `linkScope` already holds the whole Section 3 decision, so the fix is to reach it from the relay path, not to rebuild it

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/wireu/mpwire.go` - `NextHopBytes` returns the whole next-hop field. `NextHop` parses the first 4 octets for AFI 1 and the first 16 for AFI 2, so a 32-octet field yields the global address alone, and the doc comment says so and names `NextHopBytes` as the alternative
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` - the MP_REACH install path stores `hex.EncodeToString(mpReach.NextHopBytes())`, so `RawRoute.NHopHex` holds 4 to 32 octets. The complex-family branch stores a whole NLRI section under `NLRIFramingSourceWire` and skips every NLRI after the first
- [ ] `internal/component/bgp/reactor/reactor_api_relay.go` - `relayPayload` hex-decodes `AttrHex`, `NextHopHex` and `NLRIHex` into one pooled scratch buffer and writes the next-hop field into the relayed UPDATE unchanged. It reads no interface table and no `linkScope`
- [ ] `internal/component/bgp/reactor/link_scope.go` - `newLinkScopeFrom` settles `peerOnLink` with `network.SharesSubnet`, and `linkLocalNextHop` returns the link-local to write or the zero Addr for Section 3's else-branch. Its callers are the session paths in `peer_forward_facts.go` and `reactor_iface.go`, not the relay path
- [ ] `internal/component/plugin/coordinator.go` - `Coordinator.RelayStoredRoute` forwards to the reactor and returns `ErrNoReactor` when none is attached

**Behavior to preserve:**
- The stored field stays the source's whole framing; the fix is at emission, not at storage
- The single-pooled-buffer decode in `relayPayload`, which a peer-up replay runs per route

**Behavior to change:**
- The relayed next-hop field must answer Section 3 for the DESTINATION peer

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A plugin calls `Coordinator.RelayStoredRoute` with a destination address and a slice of `rpc.StoredRoute`, which is the peer-up replay in the route-server case.
- Format at entry: `rpc.StoredRoute` carrying `AttrHex`, `NextHopHex` and `NLRIHex` as hex strings, plus the path identifier and the NLRI framing.

### Transformation Path
1. `Coordinator.RelayStoredRoute` (`internal/component/plugin/coordinator.go`) forwards to the reactor
2. `Reactor.RelayStoredRoute` selects the session for the destination
3. `relayPayload` (`reactor_api_relay.go`) decodes the three hex fields into a pooled scratch buffer
4. The attribute block is scanned, MP_REACH is re-synthesized, and the UPDATE goes to the wire

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ Engine | `rpc.StoredRoute` over the coordinator | No |
| Engine ↔ Wire | the re-synthesized MP_REACH_NLRI attribute | No |

### Integration Points
- `linkScope.linkLocalNextHop` (`link_scope.go`) - the Section 3 decision the relay must reach
- `getReadBuf` / `ReturnReadBuffer` (`reactor_api_relay.go`) - the pooled buffer the backpressure gap is about

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The relay session can reach its peer's `linkScope` without a second kernel read | `peer.go` holds `llScope atomic.Pointer[linkScope]` | The relay pays a per-route interface-table read | Read the relay session lookup | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Shortening the next-hop field at relay time changes the attribute length after the buffer was sized | A truncated or over-long UPDATE on the wire | Size the payload after the Section 3 decision, not before |
| R-2 | A peer-up replay under load exhausts the read pool, since each in-flight relay pins a buffer | Relay refusals with `errRelayBufferPool` | The congestion controller, per Thomas's note |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Routes are mis-encoded on the wire for every route-server client, and a conforming peer can reject the UPDATE |
| How is it reverted? | Single commit revert; nothing is persisted |
| Who else touches this path? | Any spec working `internal/component/bgp/reactor/` relay or the adj-rib-in store |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `Coordinator.RelayStoredRoute` with a 32-octet stored next hop to an off-link peer | → | the Section 3 decision inside `relayPayload` | [test name, fill at design time] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A stored route with a 32-octet next hop is relayed to a peer sharing no subnet with the global next hop | The relayed MP_REACH carries the 16-octet global address alone |
| AC-2 | The same route is relayed to a peer sharing a subnet with both the next-hop entity and itself | The relayed MP_REACH carries all 32 octets |
| AC-3 | `Coordinator.RelayStoredRoute` is called with no reactor attached | It returns `ErrNoReactor`, proven by a test |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs Ze as a route server and a client comes up | stored route → `RelayStoredRoute` → `relayPayload` → wire | [test name] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [fill at design time] | `internal/component/bgp/reactor/reactor_api_relay_test.go` | the Section 3 decision at relay time | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Next-hop field length (AFI 2) | 16 or 32 octets | 32 | 15 | 33 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [fill at design time] | `test/plugin/*.ci` | A relayed route reaches a client with the right next-hop form | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-rfc2545-linklocal-nexthop-frr` | `test/interop/scenarios/` | FRR | The existing scenario, extended to the RELAY path | |

## Files to Modify
- `internal/component/bgp/reactor/reactor_api_relay.go` - reach the Section 3 decision before the next-hop field is written
- `internal/component/bgp/reactor/relay_payload.go` - the payload length, which moves with the decision
- `internal/component/plugin/coordinator.go` - the untested entry point

## Files to Create
- [fill at design time]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | |
| YANG validation constraints | | |
| YANG custom validators | | |
| CLI commands/flags | | |
| CLI grammar (keyword before value) | | |
| Editor autocomplete | | |
| Functional test for new RPC/API | | `test/plugin/*.ci` |
| Pipe completeness | | |
| Env var registration | | |
| Doctor check for runtime dependencies | | |
| Prometheus counters/metrics | | relay refusals under buffer exhaustion |
| BGP family surface (new SAFI / capability / attribute) | | N-A: no new family, capability or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | |
| 2 | Config syntax changed? | | |
| 3 | CLI command added/changed? | | |
| 4 | API/RPC added/changed? | | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | | |
| 6 | Has a user guide page? | | |
| 7 | Wire format changed? | | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | | `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | | `rfc/short/rfc2545.md` |
| 10 | Test infrastructure changed? | | |
| 11 | Affects daemon comparison? | | |
| 12 | Internal architecture changed? | | `docs/architecture/core-design.md` |
| 13 | Route metadata keys added/changed? | | |
| 14 | Prometheus counters added/changed? | | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | |
| 16 | Any changed source file referenced by existing doc source anchors? | | DERIVED: `./le spec citation anchors spec plan/immediate/spec-rfc2545-next-hop-truncated-to-sixteen-bytes.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- reach the destination peer's `linkScope` from the relay path, write failing wiring tests
   - Tests: [wiring test names]
   - Files: `reactor_api_relay.go`
   - Verify: the relay can ask the Section 3 question; the wiring test fails because the answer is not used yet
2. **Phase: [name]** -- [fill at design time]

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | The payload length is computed after the Section 3 decision, never before |
| Rule: `ai/rules/rfc-compliance.md` | The enforcing line carries `// RFC 2545 Section 3:` with the quoted sentence |
| Rule: `ai/rules/interop-and-goal-validation.md` | The test goes RED with the fix reverted and the artifact rebuilt |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `Coordinator.RelayStoredRoute` has a test | `grep -r RelayStoredRoute --include=*_test.go` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | Each in-flight relay pins a read-pool buffer, and a peer-up replay is unbounded |

### Failure Routing
| Failure | Route To |
|---------|----------|
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- [fill at closure]

## RFC Documentation (Scope: protocol)

Add `// RFC 2545 Section 3: "<quoted requirement>"` above the code that decides
whether the link-local half is written, quoting the if-and-only-if sentence and
its "in all other cases" branch.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only
