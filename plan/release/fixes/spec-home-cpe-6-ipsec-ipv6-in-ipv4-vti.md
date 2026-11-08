# Spec: home-cpe-6-ipsec-ipv6-in-ipv4-vti

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P1 |
| Release reason | Route-based tunnels are shipped; a peer that proposes both families and is narrowed to IPv6 over an IPv4 WAN must carry IPv6 end to end. Today no test covers the IPv6-inner datapath and child 20 shows it cannot work. Validation plus whatever fix the scenario exposes. |
| RFC gaps | - |
| Depends | spec-home-cpe-20-xfrm-cross-family-tunnel-state.md |
| Phase | 3/3 |
| Handoff | - |
| Updated | 2026-11-08 |

Parent: plan/release/fixes/spec-home-cpe-0-umbrella.md

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

### Dependency and closure order (shared with child 20)

| Item | Owner | Rule |
|------|-------|------|
| Interop scenario `vti-ipv6-inner-ipv4-outer` | this spec, sole owner | This spec creates, runs and red-walks it. Child 20 creates and edits no file of it, and cites this spec's recorded run at its own closure |
| Implementation prerequisite | child 20 (`spec-home-cpe-20-xfrm-cross-family-tunnel-state.md`) | Child 20's `XFRM_STATE_AF_UNSPEC` fix must be COMMITTED in HEAD before Phase 2 here runs. Child 20 need not be closed: its fix and its namespace proof land before its closure |
| Integrated proof | this spec | AC-5 green with child 20's fix; AC-6 red with that fix reverted and the Ze image rebuilt; the red run is recorded in this spec's commit body, which is the evidence child 20 cites |
| Closure order | both | 1. child 20 implemented and committed; 2. this spec implemented, scenario green, red walk recorded, this spec closed; 3. child 20 closed |

The metadata `Depends` names child 20 in the implementation sense only: this spec
needs its code, never its closure, so neither spec waits on the other's closure
before its own work.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/access-security.md` - IKE door
  → Constraint: readers of IKE state use `ipsecinventory` and `ikeprobe`, never `ike/engine`.
- [ ] `docs/guide/ipsec.md` - section "Traffic selectors and narrowing", and the VTI binding paragraph quoted in the constraint below
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
- [ ] `internal/component/ike/engine/config.go` `validateSelectorCombinations` - refuses a list missing a same-family combination; a list of one IPv4 entry and one IPv6 entry passes.
- [ ] `internal/component/ike/engine/rekey.go` `proposeChildTSPayloads`, `anyChildTSPayloads` - a peer with a configured list proposes exactly that list in order; a peer with none proposes the wildcard of the family of `net.ParseIP(RemoteAddress)`, which is IPv4 for an IPv4 literal and also IPv4 for a hostname (ParseIP nil, `isV6` false).
- [ ] `internal/component/ike/engine/ts_narrow.go` `recordInitiatorSelectorsIn` - checks the responder's answer against the recorded proposal (a subset is accepted, a widening refused) and records `sa.NegotiatedPairs`.
- [ ] `internal/component/ike/engine/ts_mixed_family_test.go` - a pair that mixes an IPv4 TSi with an IPv6 TSr is refused with TS_UNACCEPTABLE; the IPv6-only subset of a two-family proposal has no test of its own.
- [ ] `internal/component/ike/engine/established.go` `resolveIfID` - the bound interface's if_id, an error for a non-xfrm binding or if_id 0.
- [ ] `internal/component/ike/engine/child.go` `childPolicies`, `childPairPolicy` - one policy per negotiated pair and direction, carrying the Child SA's if_id.
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - tunnel state installed with no selector and no AF_UNSPEC flag (child 20).
- [ ] `test/interop-ipsec/scenarios/` - no scenario carries IPv6 inner traffic; `mtu-tunnel-sizing-strongswan` and `ike-padded-probe-strongswan` use an xfrm interface with IPv4 inner.

**Behavior to preserve:**
- A peer with no configured selectors keeps proposing the outer family's wildcard.
- The mixed-family pair refusal stays.
- Narrowing to the IPv4 pair alone works the same way as narrowing to the IPv6 pair.

**Behavior to change:**
- Whatever the scenario exposes between the narrowed answer and the delivered IPv6 packet. Known today: child 20's state flag.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Selector-list validation | `internal/component/ike/engine/config.go` `validateSelectorCombinations` | Refuses a list missing a same-family combination; one IPv4 and one IPv6 entry pass | does not apply: already accepts the dual-family list; this spec proves it through `ze config validate` |
| Child SA TS proposal | `internal/component/ike/engine/rekey.go` `proposeChildTSPayloads`, `anyChildTSPayloads` | Proposes the configured list in order, or the outer family's wildcard | does not apply: no assumption covers it; AC-1 checks it on the wire (`TestDualFamilyProposalNarrowedToIPv6InstallsVTIPolicy` asserts both entries, IPv4 then IPv6), and a missing or reordered entry is fixed in `proposeChildTSPayloads` in this spec |
| Initiator answer check | `internal/component/ike/engine/ts_narrow.go` `recordInitiatorSelectorsIn`; `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` "Decisions" | Accepts a subset of the proposal, refuses a widening, records `sa.NegotiatedPairs` | A-1 is this check: the IPv6-only subset gets its own test (`TestInitiatorAcceptsNarrowingToOneFamily`); if A-1 breaks, `recordInitiatorSelectorsIn` is fixed here |
| VTI if_id binding | `internal/component/ike/engine/established.go` `resolveIfID` | Returns the bound xfrm interface's if_id, refuses a non-xfrm binding or if_id 0 | does not apply: reused as is |
| Child SA policy set | `internal/component/ike/engine/child.go` `childPolicies`, `childPairPolicy` | One policy per negotiated pair and direction carrying the if_id | A-2 is this check (`TestNarrowedChildInstallsOnlyTheNegotiatedFamily`): expected to already emit only the IPv6 policies with an IPv4 template; if A-2 breaks, `childPolicies` is fixed here |
| Tunnel state AF_UNSPEC | `plan/release/fixes/spec-home-cpe-20-xfrm-cross-family-tunnel-state.md` | Child 20 sets `XFRM_STATE_AF_UNSPEC` on a selector-less tunnel state | does not apply here: child 20 owns the fix, this spec's scenario is its red-first proof |
| strongSwan interop lab | `test/interop-ipsec/scenarios/`, `docs/architecture/testing/interop.md` | Scenarios such as `mtu-tunnel-sizing-strongswan` run Ze against strongSwan over an xfrm interface with IPv4 inner traffic | extends: adds `vti-ipv6-inner-ipv4-outer` in the same layout |

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
- `internal/component/ike/engine/ts_narrow.go` `recordInitiatorSelectorsIn` - only if A-1 breaks
- `internal/component/ike/engine/child.go` `childPolicies` - only if A-2 breaks
- `internal/component/ike/engine/rekey.go` `proposeChildTSPayloads` - only if AC-1's wire check fails
- `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` - declared by `ts_narrow.go`. Nothing in it becomes wrong while `ts_narrow.go` is untouched: its decision "The initiator checks the answer, it does not trust it" already covers a subset answer. If A-1 forces a change to `recordInitiatorSelectorsIn`, that decision gains the single-family subset of a two-family proposal
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
| 16 | Changed source referenced by doc anchors? | Yes | derived by `./le spec citation anchors spec plan/release/fixes/spec-home-cpe-6-ipsec-ipv6-in-ipv4-vti.md` |
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
- [ ] The targeted tests for what changed pass: the scoped package test for `internal/component/ike/engine` through `./le job run label unit-pkg command go test -tags ...`, `./le test functional parse` for `ipsec-dual-family-selectors-validate.ci`, `IPSEC_INTEROP_SCENARIO=vti-ipv6-inner-ipv4-outer ./le test integration interop-ipsec` (green with child 20's fix, red once with it reverted and the Ze image rebuilt, the red recorded), and `./le verify changed` over the closing commits
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

## Implementation Summary

### What Was Implemented
- e8faddced1 (unit tests, interop scenario, docs), 3c23153b48 (strongSwan offers MODP_2048 so the PFS rekey of AC-8 succeeds), 5ee1ebb500 (parse test promoted to `test/parse/`). All three carry a `Spec-work: home-cpe-1-logical-interface-name` trailer by misattribution; they are this spec's work.
- No product code changed: A-1, A-2 and AC-1's wire check held, so `recordInitiatorSelectorsIn`, `childPolicies` and `proposeChildTSPayloads` stay as they were. The datapath fix is child 20's `XFRM_STATE_AF_UNSPEC` (`xfrmStateFromParams`, `internal/component/ike/dataplane/xfrm_linux.go`, 0db8414cfd).
- Scenario checker `checkVTIIPv6InnerIPv4Outer` (`internal/le/interoplab/ipsec/vtiipv6.go`), registered in `scenarioCheckers` with `writesPeerSysctl`.

### Bugs Found/Fixed
- strongSwan's child offered no D-H group while Ze's esp-group sets PFS, so Ze rightly refused the rekey with NO_PROPOSAL_CHOSEN: a lab config defect, fixed in 3c23153b48. No product bug.

### Documentation Updates
- `docs/guide/ipsec.md` "IPv6 inside an IPv4 tunnel on a VTI" (e8faddced1), `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` (dual-family narrowing paragraph), `docs/architecture/testing/interop.md` (scenario section, anchor on the scenario's `swanctl.conf`), `features/ipsec-interop-testing.md` (anchor `vtiipv6.go -- checkVTIIPv6InnerIPv4Outer`).
- `./le repo check`: all checks passed (2026-11-08).

### Deviations from Plan
- AC-6: the red walk (`c6-afunspec-off`, `state.AFUnspec = false`) fails the scenario at step 3, the af-unspec state check that precedes the ping, so the ping itself was not observed red. The ping step's own discrimination is its ESP counters: `verifyESPDirectionsToward` (`internal/le/interoplab/ipsec/helpers.go`) requires all four simplex SAs to advance across a lossless ping, and `xfrmi_xmit2` gives vti0 no clear exit. That replaces the Critical Review row "Ping fails with the Child SA deleted", which was not run.
- AC-7 is asserted through `show vpn ipsec sa | json` (`requireChildIfID`, `waitZeSelectors`) rather than the text form.
- `test/parse/ipsec-dual-family-selectors-validate.ci` carries no Linux marker, so on macOS the xfrm-on-darwin check refuses it; it was run in the Ze 7.3 QEMU guest. A macOS-only defect, which waits until after the first release (`ai/META-DECISIONS.md`, "Production is Linux").
- The IPsec umbrella row for the scenario was not written by this closure: the door gate refused the edit to `plan/release/validation/spec-ipsec-0-umbrella.md` (five further doors, and a producer it cites no longer exists). See Work Not Done.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The implementation commits carry `Spec-work: home-cpe-1-logical-interface-name` | They are home-cpe-6's work; the session's claim at commit time was child 1 | Reading e8faddced1, 3c23153b48, 5ee1ebb500 at closure | Named here and in commit A's body |
| approach | The spec stayed at `Status design`, `Phase 0/3` while all three phases were committed | Implementation was complete | Closure entry | Set to in-progress via `./le spec claim`, Phase 3/3 |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| 1. Dual-family list validates and is sent in order | Done | `vti_dual_family_test.go` `TestDualFamilyProposalNarrowedToIPv6InstallsVTIPolicy`; `config_test.go` `TestDualFamilySelectorListValidates`; `test/parse/ipsec-dual-family-selectors-validate.ci` | |
| 2. IPv6-only answer accepted, recorded, installed with if_id, no IPv4 policy | Done | `ts_mixed_family_test.go` `TestInitiatorAcceptsNarrowingToOneFamily`; `child_test.go` `TestNarrowedChildInstallsOnlyTheNegotiatedFamily` | |
| 3. IPv6 route via vti0 carries a ping | Done | `vtiipv6.go` `vtiIPv6Carries` | |
| 4. Interop against strongSwan, red with child 20 reverted | Done | `vti-ipv6-inner-ipv4-outer`; red under `c6-afunspec-off` | See Deviations, AC-6 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestDualFamilyProposalNarrowedToIPv6InstallsVTIPolicy` (`requireWireSelectors` IPv4 then IPv6); `ipsec-dual-family-selectors-validate` | |
| AC-2 | Done | `TestInitiatorAcceptsNarrowingToOneFamily`; scenario `waitChildSelectors(::/0, ::/0)` | |
| AC-3 | Done | `requireIPv6PoliciesOnly` (unit); `requireIPv6PoliciesOnIfID` out/in/fwd on if_id 7 (interop) | |
| AC-4 | Done | `TestInitiatorRefusesAWidenedFamilyOfATwoFamilyProposal` | |
| AC-5 | Done | `vtiIPv6Carries` → `verifyESPDirectionsToward` | |
| AC-6 | Done | `c6-vti-broken.log`: FAIL "lacks af-unspec" | Fails at the state check, see Deviations |
| AC-7 | Done | `requireChildIfID` over `show vpn ipsec sa \| json`; `waitZeSelectors` | |
| AC-8 | Done | `checkVTIIPv6InnerIPv4Outer` rekey step, then `vtiIPv6Negotiated` and `vtiIPv6Carries` again | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestInitiatorAcceptsNarrowingToOneFamily` | Done | `internal/component/ike/engine/ts_mixed_family_test.go` | both families |
| `TestNarrowedChildInstallsOnlyTheNegotiatedFamily` | Done | `internal/component/ike/engine/child_test.go` | |
| `TestDualFamilyProposalNarrowedToIPv6InstallsVTIPolicy` | Done | `internal/component/ike/engine/vti_dual_family_test.go` | |
| `TestDualFamilySelectorListValidates` | Done | `internal/component/ike/engine/config_test.go` | |
| `ipsec-dual-family-selectors-validate` | Done | `test/parse/` | |
| `vti-ipv6-inner-ipv4-outer` | Done | `test/interop-ipsec/scenarios/` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `ts_narrow.go`, `child.go`, `rekey.go` | Not needed | Conditional on A-1, A-2, AC-1 breaking; none broke |
| `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` | Not needed | Conditional on a `ts_narrow.go` change |
| `docs/guide/ipsec.md` | Done | e8faddced1, 67700fd37e |
| `plan/release/validation/spec-ipsec-0-umbrella.md` | Not done | Work Not Done |
| scenario directory, `vti_dual_family_test.go`, parse `.ci` | Done | |

### Audit Summary
- **Total items:** 18
- **Done:** 13
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 5 (four conditional files not needed, the umbrella row homed in its own spec)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Dual-family proposal accepted and sent in order | functional + unit | `ipsec-dual-family-selectors-validate` pass in the Ze 7.3 guest (`out-qb2/c6-clean.log`), red under `c6-combination-unchecked` ("seq 2: expected exit code 1, got 0", `out-qb2/c6-broken.log`) |
| IPv6-only answer installed on the if_id, no IPv4 policy | interop | `vti-ipv6-inner-ipv4-outer` step 2, `requireIPv6PoliciesOnIfID` |
| IPv6 ping crosses vti0 | interop | `c6-vti-clean.log`: PASS 1 scenario (11s), 2026-11-08 on the tree after 652591fc76 |
| Red when child 20 is reverted | interop | `c6-vti-broken.log`: FAIL "the state 172.28.0.2 -> 172.28.0.3 on if_id 0x7 lacks af-unspec" |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| List `vti-ipv6-inner-ipv4-outer` in the IPsec umbrella's test inventory | The door gate refused this closure's edit (five more doors, and a cited producer that no longer exists) | `plan/release/validation/spec-ipsec-0-umbrella.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/home-cpe-6-ipsec-ipv6-in-ipv4-vti-7e292f27-ced0-427f-a7b6-3a931db211f9.md` (24 files, verdict clean) |
| `./le spec review check` | OK (10 code files, clean, hashes match) |
| Rounds | 1 |
| Reviewer lenses used | wiring, logic of the xfrm dump parsers, vacuity of the red walk, style pass over the changed Go (no `panic()`, loops bounded by input, no new lifecycle), RFC 7296 Section 2.9 initiator side |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | No BLOCKER or ISSUE | - | - |

NOTEs: (1) the comment in `requireIPv6PoliciesOnIfID` says `requireInterfaceSPDCatchAll` judges the final entry, which this scenario does not call (`spd-vti-unmatched-no-exemptions` does); the skipped entry is a BLOCK, so it admits nothing. (2) The AC-6 red is observed at the state check, not the ping (Deviations).

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/interop-ipsec/scenarios/vti-ipv6-inner-ipv4-outer/` | Yes | `ls`: `swanctl.conf`, `ze.conf` |
| `internal/component/ike/engine/vti_dual_family_test.go` | Yes | `ls`: 9.0K |
| `test/parse/ipsec-dual-family-selectors-validate.ci` | Yes | `ls`: 3.7K |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-4 | unit tests exist and assert as stated | `grep -n '^func Test'` finds all five in `ts_mixed_family_test.go`, `child_test.go`, `config_test.go`, `vti_dual_family_test.go`; bodies read at closure |
| AC-5..AC-8 | scenario green, red under the revert | `c6-vti-clean.log` PASS; `c6-vti-broken.log` FAIL at af-unspec |
| all | scoped package test on today's tree | `go test -race`, all feature tags, HEAD 45025b3639, 2026-11-08 (`c6pkg.log`): `ike/dataplane` ok, `interoplab/ipsec` ok, `ike/engine` FAIL only in `TestMobikeParallelAuthKeepsPromotedPolicyEndpoints` and `TestMobikeParallelAuthRetiresChildRekey`, which bind 127.0.0.2 and fail on macOS without the loopback alias that `./le setup` adds (not this spec's code) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Config with two selector entries and `vti bind` | `test/parse/ipsec-dual-family-selectors-validate.ci` | Read: config 1 accepted, config 2 (third IPv4 entry) refused by the same validator |
| Real IKE exchange with strongSwan | `checkers.go` `"vti-ipv6-inner-ipv4-outer"` → `checkVTIIPv6InnerIPv4Outer` | Read |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestInitiatorAcceptsNarrowingToOneFamily` |
| A-2 | confirmed | `TestNarrowedChildInstallsOnlyTheNegotiatedFamily`, interop step 2 |
| A-3 | confirmed | interop `vtiIPv6Carries`: route via vti0 reached the kernel |
| A-4 | confirmed | interop `waitChildSelectors(::/0, ::/0)` and no `0.0.0.0/0` in `swanctl --list-sas` |
| A-5 | confirmed | clean PASS with 0db8414cfd, FAIL with `state.AFUnspec = false` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Feature file IPv6-inner evidence | `features/ipsec-interop-testing.md` anchor `vtiipv6.go -- checkVTIIPv6InnerIPv4Outer` | Yes |
| User guide | `docs/guide/ipsec.md` "IPv6 inside an IPv4 tunnel on a VTI", config matches the scenario's `ze.conf` | Yes |
| RFC rows (checklist 9) | `rfc/short/rfc7296.md` RFC7296-2.9-1 and 2.9-2 bind the responder and carry no inline test link; no new tag was added | No change owed |
| Anchors (checklist 16) | `./le repo check` passed | Yes |
