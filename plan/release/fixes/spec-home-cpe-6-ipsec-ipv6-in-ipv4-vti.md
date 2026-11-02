# Spec: home-cpe-6-ipsec-ipv6-in-ipv4-vti

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Route-based tunnels are shipped; a peer that proposes both families and is narrowed to IPv6 over an IPv4 WAN must carry IPv6 end to end. Today no test covers the IPv6-inner datapath and child 20 shows it cannot work. Validation plus whatever fix the scenario exposes. |
| RFC gaps | - |
| Depends | spec-home-cpe-20-xfrm-cross-family-tunnel-state.md |
| Phase | 0/3 |
| Handoff | - |
| Updated | 2026-10-31 |

Parent: plan/home-cpe/spec-home-cpe-0-umbrella.md

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The home CPE's management path is one Child SA on a VTI (`vti { bind vti0 }`,
xfrm interface with an if_id). Ze, the initiator, proposes `0.0.0.0/0` and `::/0`
on both sides; the strongSwan responder narrows the answer to `::/0 === ::/0`.
The outer path is IPv4 (a PPPoE WAN address to `vpn.example.net`). Management
traffic is IPv6 between `fd00:db8:1::1` on the CPE's vti0 and `fd00:db8:1::2` at
the responder.

Deliver and prove, through the real entry points:

1. A peer configuration that proposes both families in one Child SA (two
   traffic-selector entries) is accepted by `ze config validate` and sent on the
   wire in that order.
2. A responder answer narrowed to the IPv6 pair alone is accepted, recorded, and
   installed: IPv6 policies in both directions carrying the VTI's if_id, no IPv4
   policy.
3. An IPv6 route via vti0 carries a ping to the responder and back.
4. An interop scenario against strongSwan proves 1 to 3, and goes red when child
   20's fix is reverted.

Where any step fails at the source, the fix is in this spec.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/access-security.md` - IKE door
  → Constraint: readers of IKE state use `ipsecinventory` and `ikeprobe`, never `ike/engine`.
- [ ] `docs/guide/ipsec.md` - "Traffic selectors and narrowing" (about lines 1376-1395) and the VTI paragraph (about lines 1479-1486)
  → Decision: a dual-stack peer writes one IPv4 entry and one IPv6 entry; the guide states that list "already names every combination" and `validateSelectorCombinations` accepts it.
  → Constraint: "The interface has to exist when the peer's first Child SA is created: a binding that names no xfrm interface, or one configured with `if-id 0`, fails the Child SA with an error naming the interface."
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - Child SA install, policies per negotiated pair
- [ ] `docs/architecture/testing/interop.md` - the IPsec suite, scenario layout, the vacuity traps
  → Constraint: a scenario is named, never numbered; it asserts state, not timing; it is broken once on purpose and recorded red.
- [ ] `plan/release/validation/spec-ipsec-0-umbrella.md` - parent of the IPsec scenarios
  → Decision: the new scenario is listed there as well as here, so the IPsec umbrella's inventory stays whole.
- [ ] `plan/release/fixes/spec-ipsec-vti-bind-installs-if-id-zero.md` - the if_id-zero defect on the same binding
  → Constraint: this spec does not re-fix it; if that spec is still open when this scenario runs, its fix is a precondition and the scenario exercises it.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - present
  → Constraint: RFC 7296 Section 2.9: "When the responder chooses a subset of the traffic proposed by the initiator, it narrows the Traffic Selectors to some subset of the initiator's proposal (provided the set does not become the null set)."
  → Constraint: RFC 7296 Section 2.9 also: "IKEv2 allows the responder to choose a subset of the traffic proposed by the initiator." The CPE MUST accept the IPv6-only subset and MUST NOT install the IPv4 pair it was not given.
- [ ] `rfc/short/rfc4301.md` - present
  → Constraint: RFC 4301 Section 5.1.2.1: "IPsec allows the IP version of the encapsulating header to be different from that of the inner header."

**Key insights:**
- Selector negotiation for two families already exists; the datapath is unproven and child 20 shows the kernel refuses it today.
- Without configured selectors the wildcard follows the outer family, so an IPv4 WAN proposes only `0.0.0.0/0`. The CPE configuration therefore names both entries.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/config.go` `validateSelectorCombinations` (about line 247) - refuses a list missing a same-family combination; a list of one IPv4 entry and one IPv6 entry passes.
- [ ] `internal/component/ike/engine/rekey.go` `proposeChildTSPayloads`, `anyChildTSPayloads` (about line 237) - a peer with a configured list proposes exactly that list in order; a peer with none proposes the wildcard of the family of `net.ParseIP(RemoteAddress)`, which is IPv4 for an IPv4 literal and also IPv4 for a hostname (ParseIP nil, `isV6` false).
- [ ] `internal/component/ike/engine/ts_narrow.go` `recordInitiatorSelectorsIn` (about line 829) - checks the responder's answer against the recorded proposal (a subset is accepted, a widening refused) and records `sa.NegotiatedPairs`.
- [ ] `internal/component/ike/engine/ts_mixed_family_test.go` - a pair that mixes an IPv4 TSi with an IPv6 TSr is refused with TS_UNACCEPTABLE; the IPv6-only subset of a two-family proposal has no test of its own.
- [ ] `internal/component/ike/engine/established.go` `resolveIfID` (about line 1051) - the bound interface's if_id, an error for a non-xfrm binding or if_id 0.
- [ ] `internal/component/ike/engine/child.go` `childPolicies`, `childPairPolicy` (about lines 751-900) - one policy per negotiated pair and direction, carrying the Child SA's if_id.
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - tunnel state installed with no selector and no AF_UNSPEC flag (child 20).
- [ ] `test/interop-ipsec/scenarios/` - no scenario carries IPv6 inner traffic; `mtu-tunnel-sizing-strongswan` and `ike-padded-probe-strongswan` use an xfrm interface with IPv4 inner.

**Behavior to preserve:**
- A peer with no configured selectors keeps proposing the outer family's wildcard.
- The mixed-family pair refusal stays.
- Narrowing to the IPv4 pair alone works the same way as narrowing to the IPv6 pair.

**Behavior to change:**
- Whatever the scenario exposes between the narrowed answer and the delivered IPv6 packet. Known today: child 20's state flag.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `vpn ipsec site-to-site peer <name> traffic-selector 1 { local prefix 0.0.0.0/0 remote prefix 0.0.0.0/0 } traffic-selector 2 { local prefix ::/0 remote prefix ::/0 } vti { bind vti0 }`, and `interface xfrm vti0 { if-id 7 }` with an IPv6 address.
- Wire: the responder's IKE_AUTH response with TSi and TSr holding `::/0` only.

### Transformation Path
1. Config parse into `ipsec.SiteToSitePeer` traffic selectors; `validateSelectorCombinations`.
2. `proposeChildTSPayloads` writes both entries to TSi and TSr and records the proposal.
3. IKE_AUTH response: `recordInitiatorSelectorsIn` accepts the IPv6-only subset into `sa.NegotiatedPairs`.
4. `createFirstChildSA` resolves the if_id, installs the state pair (with child 20's flag) and one IPv6 policy per direction.
5. A static IPv6 route via vti0 sends the packet into the xfrm interface; the kernel matches the if_id-bound policy and encrypts.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ IKE engine | parsed YANG tree, `ipsec.IPsecConfig` | No |
| Ze ↔ strongSwan | IKEv2 TS payloads | No |
| Ze ↔ kernel | XFRM state and policy with if_id | No |
| Static routes ↔ xfrm interface | route via interface name | No |

### Integration Points
- `proposeChildTSPayloads`, `recordInitiatorSelectorsIn`, `childPolicies`, `resolveIfID`, the dataplane install.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | Uses the existing negotiation and install path |
| No unintended coupling | Yes | No new import |
| No duplicated functionality | Yes | Validation of existing paths |
| Zero-copy preserved where applicable | N-A | |
| Registration over hardcoding, outbound | N-A | |
| Registration over hardcoding, inbound | N-A | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `recordInitiatorSelectorsIn` accepts an answer that drops one family entirely | Subset rule in `ts_narrow.go` | Fix the subset check; new unit test | `TestInitiatorAcceptsNarrowingToOneFamily` | unvalidated |
| A-2 | Only the IPv6 pair produces policies; nothing installs a leftover IPv4 policy from the proposal | `childPolicies` reads negotiated pairs | IPv4 traffic would match a policy whose SA does not carry it | `TestNarrowedChildInstallsOnlyTheNegotiatedFamily` | unvalidated |
| A-3 | An xfrm interface accepts an IPv6 address and a static IPv6 route via it | iface YANG `xfrm` unit | Iface or static fix lands in this spec | `vti-ipv6-inner-ipv4-outer` scenario | unvalidated |
| A-4 | strongSwan narrows to `::/0 === ::/0` when its `local_ts`/`remote_ts` are `::/0` and Ze proposes both | strongSwan behaviour, source not present locally | Scenario configuration changes, design does not | First scenario run, `swanctl --list-sas` | unvalidated |
| A-5 | Child 20 makes the kernel carry IPv6 inside the IPv4 SA | Kernel source read in child 20 | Scenario red for another cause | Child 20's integration test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Scenario passes by IPv6 leaking outside the tunnel | Ping succeeds with the SA deleted | Assert ESP counters on both sides advance with the ping, and that ping fails with the Child SA down |
| R-2 | Child rekey re-proposes both families and the responder narrows again differently | Rekey scenario logs a TS change | RFC 7296 Section 2.9.2 rule already enforced; add a rekey step to the scenario |
| R-3 | The CPE's IPv4 traffic is affected by a v4 wildcard proposal never installed | IPv4 to the internet lost | A-2 test; the proposal itself installs nothing |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The management tunnel carries nothing; IPv4 internet is unaffected because no IPv4 policy is installed |
| How is it reverted? | Test-only for most phases; any product fix is a single commit |
| Who else touches this path? | child 20 (state flag), `spec-ipsec-vti-bind-installs-if-id-zero.md`, `spec-ipsec-0-umbrella.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config with two selector entries and `vti bind` | → | `proposeChildTSPayloads`, `recordInitiatorSelectorsIn`, `childPolicies` | `TestDualFamilyProposalNarrowedToIPv6InstallsVTIPolicy` |
| Real IKE exchange with strongSwan | → | full stack | interop `vti-ipv6-inner-ipv4-outer` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer with entries `0.0.0.0/0` and `::/0` (both sides) | `ze config validate` clean; IKE_AUTH request TSi and TSr carry IPv4 then IPv6 |
| AC-2 | Responder answers `::/0` only | Child SA established; `sa.NegotiatedPairs` holds one IPv6 pair |
| AC-3 | After AC-2 | Installed policies: IPv6 in, out (and fwd where Ze installs it) with the vti0 if_id; no IPv4 policy for this Child SA |
| AC-4 | Responder answers a wider set than proposed | Refused, as today (no regression) |
| AC-5 | IPv6 route `fd00:db8:1::/112` via vti0, ping `fd00:db8:1::2` | Echo reply received; ESP counters on both ends advance |
| AC-6 | Child 20's fix reverted, scenario rerun | AC-5 fails; red run recorded |
| AC-7 | `show vpn ipsec sa` | Shows the negotiated IPv6 selectors and the if_id |
| AC-8 | Child SA rekey | Same IPv6 selectors after rekey; ping continues |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Manages the CPE over IPv6 through the tunnel | config → IKE → narrowing → XFRM with if_id → vti0 route | `vti-ipv6-inner-ipv4-outer` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInitiatorAcceptsNarrowingToOneFamily` | `internal/component/ike/engine/ts_mixed_family_test.go` | IPv6-only subset of a two-family proposal accepted and recorded | |
| `TestNarrowedChildInstallsOnlyTheNegotiatedFamily` | `internal/component/ike/engine/child_test.go` | policies for the IPv6 pair only, each with the if_id | |
| `TestDualFamilyProposalNarrowedToIPv6InstallsVTIPolicy` | `internal/component/ike/engine/vti_dual_family_test.go` | wiring from parsed config through to dataplane calls on a fake dataplane | |
| `TestDualFamilySelectorListValidates` | `internal/component/ike/engine/config_test.go` | AC-1 validation half | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no new numeric input | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-dual-family-selectors-validate` | `test/parse/` | the CPE peer block parses and validates | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `vti-ipv6-inner-ipv4-outer` | `test/interop-ipsec/scenarios/` | strongSwan 6.1.0, responder narrowing to `::/0` | AC-1 to AC-8: dual proposal, narrowing, if_id policy, IPv6 ping, red with child 20 reverted | |

## Files to Modify
- `internal/component/ike/engine/ts_narrow.go`, `child.go` - only if A-1 or A-2 breaks
- `docs/guide/ipsec.md` - a dual-family VTI example: IPv6 inside an IPv4 tunnel, narrowed by the peer
- `plan/release/validation/spec-ipsec-0-umbrella.md` - list the scenario (main thread edits it; named here so it is not lost)

## Files to Create
- `test/interop-ipsec/scenarios/vti-ipv6-inner-ipv4-outer/` - `ze.conf`, `swanctl.conf`, `strongswan.conf`, the scenario's checks
- `internal/component/ike/engine/vti_dual_family_test.go`
- `test/parse/ipsec-dual-family-selectors-validate.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | existing leaves |
| YANG validation constraints | No | |
| YANG custom validators | No | |
| CLI commands/flags | No | |
| CLI grammar | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | No | |
| Pipe completeness | No | |
| Env var registration | No | |
| Doctor check for runtime dependencies | No | |
| Prometheus counters/metrics | No | |
| BGP family surface | N-A | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | Feature scope or evidence changed? | Yes | the IPsec feature file under `features/` gains the IPv6-inner evidence |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior newly proven? | Yes | `rfc/short/rfc7296.md` Section 2.9 rows gain the scenario as evidence where they carry a test link |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys | No | |
| 14 | Prometheus counters | No | |
| 15 | Registered plugin/event changed? | No | |
| 16 | Changed source referenced by doc anchors? | Yes | derived by `./le spec citation anchors spec plan/home-cpe/spec-home-cpe-6-ipsec-ipv6-in-ipv4-vti.md` |
| 17 | Existing examples for this area? | Yes | verify the VTI example in `docs/guide/ipsec.md` against the YANG |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)**
   - Tests: `TestDualFamilyProposalNarrowedToIPv6InstallsVTIPolicy`, `TestInitiatorAcceptsNarrowingToOneFamily`
   - Files: the new unit test files
   - Verify: they run against a fake dataplane; any red names the defect at its producer
2. **Phase: Scenario**
   - Tests: `vti-ipv6-inner-ipv4-outer`
   - Files: the scenario directory
   - Verify: green with child 20 landed; revert child 20, rebuild the Ze image, confirm red, restore, confirm green, record the red
3. **Phase: Docs and evidence**
   - Files: `docs/guide/ipsec.md`, the feature file
   - Verify: `./le spec citation anchors` clean

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has evidence |
| Correctness | No IPv4 policy installed from an IPv6-only answer |
| Rule: interop | Scenario recorded red once |
| Rule: vacuity | Ping fails with the Child SA deleted |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Scenario directory | `ls test/interop-ipsec/scenarios/vti-ipv6-inner-ipv4-outer` |
| Red run recorded | commit body |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Policy scope | Narrowed answer is the only installed policy; widening refused |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Ping fails with child 20 landed | Trace: policy, state, route, interface address; fix in this spec |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

The negotiation half of this tunnel was built and tested; the datapath half never
carried a packet of the other family. The scenario is the first thing that can
tell them apart.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| - | - | None found for IPv6 inner traffic (grep of `plan/journal/` on 2026-10-31). Child 20 owns the kernel-state row |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The CPE configures both selector entries explicitly | Make a VTI-bound peer with no list propose both wildcards | Changes the wire of every existing VTI configuration; the explicit list is already supported and the mapping guide (child 10) writes it |
| Prove with strongSwan narrowing, not Ze-to-Ze | Ze responder | The real responder is strongSwan; Ze's responder narrowing has its own tests |
| Datapath fix lives in child 20 | Fold it here | Child 20 is a dataplane defect that hits any cross-family tunnel, VTI or not |

## Known Limitations
- IPv6 OUTER (IKE over IPv6) is not in scope: all IKE sockets are `udp4` (`plan/journal/unwired-feature.md`, 2026-09-04 row, second finding).
- A peer with no configured list still proposes only the outer family.

## RFC Documentation (Scope: protocol)

Any change to `ts_narrow.go` carries the RFC 7296 Section 2.9 quote above the
statement it touches.

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
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] The targeted tests for what changed pass
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A** and **Commit B** per `ai/rules/planning.md`
