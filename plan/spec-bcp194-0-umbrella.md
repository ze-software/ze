# Spec: BCP 194 umbrella -- follow RFC 7454 and RFC 8195

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/bcp194-0-umbrella.md` |
| Updated | 2026-08-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make Ze follow RFC 7454 (BCP 194, BGP Operations and Security), RFC 8195 (Use of
BGP Large Communities) and RFC 7999 (BLACKHOLE Community). This umbrella
coordinates six children and holds the gap inventory, the ledger decisions, and
the execution order.

RFC 7999 joined the set on 2026-08-08, on the owner's instruction to bring it in
if required. It was required. Its source text was absent from the repository, so
its summary was written against nothing and captured none of the four MUSTs the
document carries, and Ze parses the BLACKHOLE community into a field no code
reads.

The work started from vyos.dev/T9173, which asks a network OS to derive RFC 8195
Function 3 tagging from the RFC 9234 local-role rather than making operators
hand-maintain community lists. The audit that followed found the same lever
unused far more widely, and found live defects on the path.

**Two facts shape every child.**

First, RFC 7454 is addressed to network administrators, not to protocol
implementers, and it carries no MUST-level obligation. Its only capitalised MUST
is the RFC 2119 key-words sentence in §1.1. So "Ze follows RFC 7454" resolves
into two separate questions for each recommendation: can an operator express it
at all, and does the shipped default help or fight it. A recommendation Ze cannot
express is a gap. One Ze can express but defaults against is a weaker and
different finding. No child may conflate them.

Second, the RFC 9234 role already declares the peering relationship, and Ze uses
it for the OTC procedures alone. The role constants have no consumer outside the
role plugin. Four RFC 7454 recommendations and the whole of the T9173 request are
derivable from that one declaration, and the role names in words every exception
the RFC spells out.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/core-design.md` - the wire and forwarding model every
      child changes
  → Constraint: buffer-first encoding. An UPDATE no filter modifies stays
    byte-identical on the forwarding rail, and that property is what the interop
    tests of children 1 and 2 assert against.

- [ ] `docs/guide/bgp-policy.md` - the operator-facing policy surface
  → Constraint: the filter chain is the only composition mechanism. Children add
    filters and defaults, never a new policy language.

- [ ] `ai/rules/architecture.md` - where new code belongs
  → Decision: no child adds a per-feature field or switch case to a core package.
    Everything registers, and the core discovers it.

### RFC Summaries (Scope: protocol)

- [ ] `rfc/short/rfc7454.md` - written 2026-08-08, 64 requirements, 0 gated
  → Constraint: 42 SHOULD, 12 SHOULD NOT, 8 RECOMMENDED, 1 MAY, 1 OPTIONAL, and
    no MUST of any kind. Nothing here can be gated by `./le rfc check`, so the
    proof of conformance is the mechanism plus the default, recorded per child.

- [ ] `rfc/short/rfc8195.md` - conventions, not protocol
  → Constraint: §3.2 and §4.1.1 say an AS "could assign" a function number, so
    every number is configurable and the RFC's own values are defaults.

- [ ] `rfc/short/rfc9234.md` - the role the policy derives from
  → Decision: `resolvePeerRole` is the single producer of "what is this peer to
    me". Every child reads it rather than the configured leaf.

- [ ] `rfc/short/rfc7947.md` - route-server transparency
  → Constraint: `RFC7947-x-1` is a SHOULD NOT, and it bounds what children 1 and
    2 may do on a route-server session.

**Key insights:** (minimal context to resume after compaction)

- RFC 7454 obligations bind operators. Ze's obligation is to make each one
  expressible and to ship a default that does not fight it.
- `runIngressPolicyChain` accepts on an empty filter chain, deliberately. That
  one line is the root of every "default fights the recommendation" verdict, and
  no child changes it. The answer is shipped lists and role-derived defaults, not
  an implicit deny.
- The RFC 9234 role is the unifying lever, and it is currently used once.
- Ze already ships community function numbers (0, 1, and 101 to 103). RFC 8195
  proposes 3, 4 and 6. Both must keep working.

## Current Behavior (MANDATORY)

**Source files read:** (read during the audit that produced this umbrella)

- [ ] `internal/component/bgp/reactor/filter_ordered.go` - `runIngressPolicyChain`
      accepts on an empty chain and fails closed when filters exist but the API
      server does not.
- [ ] `internal/component/bgp/wireu/community.go` - `ParseCommunityPolicy` is live
      on both route-server rails. `ShouldForwardTo` is enforced. `PrependCount`
      and `RFC7999Blackhole` are written and never read.
- [ ] `internal/component/bgp/plugins/role/role.go` and `otc.go` - the five role
      values, `peerRoleComplement`, and `resolvePeerRole`. No consumer outside the
      plugin.
- [ ] `internal/core/network/network.go` - `RealListenerFactory` carries
      `MD5Peers` and an outgoing `ListenTTL`. Its `Listen` sets the MD5 signature
      and the outgoing TTL, and never a minimum receive TTL, although
      `setIPMinTTL` exists in the same package.
- [ ] `internal/plugins/copp/translate.go` - `translatePolicy` emits accept terms
      for hand-typed trusted sources and a rate-limit term, and no drop term.
- [ ] `internal/component/bgp/plugins/rpki/` - Valid, Invalid and NotFound are all
      computed. Invalid is rejected by default. No local-preference write exists.
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` - every `FirstAS` use is
      MED comparison, not ingress validation.
- [ ] `internal/le/rfc/rfc.go` - `check_new_summaries` and
      `source_keyword_count`.

**Behavior to preserve:**

- The empty-chain accept. No child introduces an implicit deny that would black
  hole an existing deployment on upgrade.
- Every community spelling operators send today.
- Max prefixes per peer, which is already complete and conformant with RFC 7454
  §8 including its defaults, and own-AS loop rejection, which is on by default.
- RFC 7947 transparency, except where a client explicitly requests otherwise.

**Behavior to change:** see the inventory below. Each row is owned by exactly one
child.

## Data Flow (MANDATORY)

### Entry Point

- Received UPDATE wire bytes, at the ingress filter chain (children 1, 2, 4).
- Forwarded UPDATE wire bytes, per destination (children 1, 2).
- TCP connection setup, at the listener and the dialer (child 3).
- Configuration, as the unflattened BGP subtree (all children).

### Transformation Path

1. Configuration resolves to per-peer state, including the RFC 9234 role.
2. The listener and dialer apply socket options before the session exists.
3. Ingress filters run in declared stage order and can rewrite or reject.
4. Egress filters run per destination and can suppress or record attribute edits.
5. The rebuilt UPDATE is planned, sized and written.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ runtime | unflattened BGP subtree, walked per plugin | No |
| Socket ↔ session | socket options set before bind and before accept | No |
| Wire ↔ filter | payload bytes in, modified payload bytes out | No |
| Reactor ↔ accumulator | recorded operations applied after all filters pass | No |
| Ze ↔ registry feed | IRR and RPKI data fetched on a refresh interval | No |

### Integration Points

- `filterapi.Register` and `filterapi.RegisterAttrModHandler` for every new filter.
- `resolvePeerRole` for every role-derived default.
- `RealListenerFactory` and the dialer for every socket option.
- `rfc/not-enrolled.txt` and `docs/features/rfc-status.md` for the ledger.

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Gap Inventory

Every row was verified against the producing function during the audit of
2026-08-08. The Owner column is the only place a row is assigned.

| # | Gap | Producer that proves it | Owner |
|---|-----|-------------------------|-------|
| C1 | RFC 7454 §11 scrub by own Global Administrator is inexpressible | `ingressStripCommunities` matches whole 12-byte literals; `parseLargeWire` requires integer fields | child 1 |
| C2 | Prepend control community parsed and discarded | `PrependCount` has no non-test caller | child 1 |
| C3 | RFC 7999 BLACKHOLE parsed and discarded | `RFC7999Blackhole` assigned, never read | child 1 |
| B1 | RFC 7999 §3.2 propagation guard absent: a BLACKHOLE-tagged route gains no NO_EXPORT or NO_ADVERTISE | no producer exists | child 1 |
| B2 | RFC 7999 §3.3's two conditions for accepting and honouring a blackhole announcement are unimplemented, and Ze discards no traffic on one | no producer exists | child 6 |
| B3 | `rfc/short/rfc7999.md` captured 6 requirements and zero MUSTs while the source carries 4, anchored every id `x`, and held a Ze-context section the rules forbid in a summary | the summary was written with no `rfc/full/rfc7999.txt` present | child 1, DONE 2026-08-08 |
| B4 | rfc7999 is not enrolled, so none of its four MUSTs is gated | `rfc/not-enrolled.txt` | child 6 |
| C4 | Large control communities forwarded to route-server clients | both rails remove attribute code 8 only | child 1 |
| C5 | RFC 8195 §4.4 only in standard communities; a 4-byte-ASN route server cannot express its own allow tag | `StripControlCommunities` matches the low sixteen bits | child 1 |
| C6 | RFC 8195 Function 3 relation tagging absent | no producer exists | child 1 |
| C7 | RFC 8195 Functions 4 and 6 absent on the general eBGP rail | both call sites are gated on the route-server fact | child 1 |
| L1 | RFC 7454 and RFC 8195 carry no ledger disposition matching the owner decision | `rfc/not-enrolled.txt` | child 1 |
| L2 | A new summary with zero gated MUSTs fails on RFC 2119 boilerplate | `source_keyword_count` counts the key-words sentence | child 1 |
| P4 | enforce-first-as absent (§9) | no ingress first-AS check exists | child 2 |
| P5 | Private ASNs from non-customers can be stripped but not rejected (§9) | `filter_remove_private_as` strips only | child 2 |
| P6 | Ingress next-hop rewrite to the sending peer absent (§10) | `filter_modify` takes a static IPv4 literal | child 2 |
| P9 | Per-relationship inbound filtering is not derived from the role (§6.2) | the role has no consumer outside its plugin | child 2 |
| S1 | No minimum receive TTL on the listen socket (§5.2) | `RealListenerFactory` sets outgoing TTL only | child 3 |
| S2 | CoPP cannot express the ACL §4 describes, and defaults to accept over limit | `translatePolicy` emits no drop term | child 3 |
| P1 | No shipped special-purpose or bogon lists (§6.1.1) | no list exists in the tree | child 4 |
| P2 | No maximum prefix length bound (§6.1.3) | only `prefixEntry.le` | child 4 |
| P3 | RPKI NotFound is accepted at full preference (§6.1.2.2.2) | no local-preference write under the RPKI plugin | child 4 |
| P8 | Locally originated prefixes are not derived for reuse on import (§6.1.4); no IXP LAN concept (§6.1.5) | neither exists in the tree | child 4 |
| P7 | No route flap dampening (§7) | the `damp` symbols are RFC 4271 §8.1.2 session oscillation | child 5 |

Already conformant, and in scope only as regression guards: RFC 7454 §8 maximum
prefixes, including its defaults, and §9's own-AS rejection, which ships on.

Out of scope for the whole set, with a reason: TCP-AO (RFC 5925, §5.1). The
recommendation is conditional on the implementation existing, and it is a large
separate feature. It needs its own spec before it can be an obligation.

## Child Specs

| Spec | Scope | Owns |
|------|-------|------|
| `plan/spec-bcp194-1-communities.md` | RFC 7454 §11 and all of RFC 8195 | C1 to C7, L1, L2 |
| `plan/spec-bcp194-2-role-policy.md` | role-derived policy | P4, P5, P6, P9 |
| `plan/spec-bcp194-3-session.md` | RFC 7454 §4 and §5 | S1, S2 |
| `plan/spec-bcp194-4-prefix.md` | RFC 7454 §6.1 and §8 | P1, P2, P3, P8 |
| `plan/spec-bcp194-5-damping.md` | RFC 7454 §7 | P7 |
| spec-bcp194-6-blackhole (CLOSED 2026-08-13) | RFC 7999 §3.3 and §4 | B2, delivered. RFC 7999 is enrolled and every MUST-level row carries both polarities: `rfc/short/rfc7999.md`, `rfc/enrolled.txt` |

## Execution Order

| Order | Child | Why here |
|-------|-------|----------|
| 1 | communities | Holds the three live defects (C2, C3, C4) and the red compliance gate (L2). It also answers the request the set started from |
| 2 | role policy | Builds the role-consumer seam the rest reuses. Child 1 reads the role but adds no general mechanism |
| 3 | session | Independent of the other four. Can run in parallel with 2 |
| 4 | prefix | Largest data component. Needs the role seam from child 2 for the per-relationship defaults |
| 5 | damping | Starts with an owner decision on whether absence is the conformant answer |
| 6 | blackhole | Starts with an owner decision, as §3.1 makes ignoring the community conformant. Honouring it reaches the FIB and the firewall, and its origin-validation caveat interlocks with child 4 |

## Ledger Decisions

Owner decision, 2026-08-08: both RFCs are recorded in `rfc/not-enrolled.txt` with
kind `non-normative`, and neither is enrolled. Each reason states a property of
the source text and does not judge what Ze owes, which is what the disposition
gate requires.

| RFC | Reason states |
|-----|---------------|
| rfc7454 | Published as BCP 194 with IETF category Best Current Practice. The only capitalised MUST-level keyword site in the source is the RFC 2119 key-words sentence in §1.1. All 64 captured requirements are SHOULD-level or weaker |
| rfc8195 | IETF category Informational. The source contains no RFC 2119 keyword. It defines conventions, not wire behavior. The LARGE_COMMUNITY wire obligations belong to RFC 8092, which is enrolled with seven MUST-level requirements |

RFC 7999 is a different case and takes no `non-normative` row, because it carries
four real MUSTs. Its recorded kind was `blocked`, and the reason given was that
no source text existed. That reason stopped being true when the source was
downloaded on 2026-08-08, so child 1 corrects the kind to `backlog` and states
what is now owed: the extraction sign-off `rfc/extraction/rfc7999.json`.

**Owner decision, 2026-08-08: RFC 7999 IS enrolled.** It differs from the other
two, which impose no MUST-level obligation on a speaker. RFC 7999 imposes four,
so enrolment gates something real.

Enrolment is a deliverable of child 6, not of this umbrella and not of child 1.
The reason is mechanical: enrolment requires every MUST-level requirement to be
classified, and all four sit in child 6's territory. Child 1 touches RFC 7999
only at §3.2, which is a SHOULD and is not gated. Doing the enrolment earlier
would mean classifying obligations for a feature nobody has decided the shape of
yet.

| Step | What it requires |
|------|------------------|
| Extraction sign-off | `./le rfc extraction-create STEM=rfc7999` writes an unclassified skeleton, and every derived site and section is then classified by hand at `rfc/extraction/rfc7999.json`. A generated skeleton always fails the check, so only the walk makes it pass |
| Register | Derived, and a stronger claim than the source supports is refused. RFC 7999 carries capitalised RFC 2119 keywords, so it takes the `rfc2119` register and needs no `register-reason` |
| Disposition move | The stem leaves `rfc/not-enrolled.txt` by arriving in `rfc/enrolled.txt`. A deletion alone returns it to the undeclared state, which the gate also refuses |
| Public row | `docs/features/rfc-status.md` gains an RFC 7999 row in the same change, per `check_status_completeness` |
| The four MUSTs | Each needs a tagged positive and negative test pair, or an owner-authorised annotation. The Tx obligation in §3.1, the two Rx conditions of §3.3, and the origin-validation obligation of §3.3 |
| Source text | `rfc/full/rfc7999.txt`, required by `check_enrolment`. Present since 2026-08-08 |

Enrolment is not reversible in practice. Six ratchets begin to judge the stem
from that commit on: enrolment monotonicity, proof monotonicity, requirement
retirement, evidence monotonicity per tier, extraction monotonicity, and public
disclosure. Child 6 must reach the enrolment with its MUSTs already proven or
already annotated, because a stem that enrols and then loses a polarity is what
those ratchets exist to refuse.

The working tree currently records rfc7454 as `backlog`, which the research phase
wrote before the owner decided. Child 1 carries the correction as AC-15.

Enrolling either RFC was rejected. Neither specifies wire behavior, so a gated
MUST invented for one would be a claim stronger than its text, and RFC 7454's
obligations bind operators, where the gate cannot distinguish a mechanism Ze
lacks from a mechanism an operator did not configure.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The mechanism-and-default pair is the right proof shape for a BCP with no MUSTs | RFC 7454 §1.1 keyword scan, 2026-08-08 | Children prove the wrong thing and conformance stays unmeasured | Owner review of child 1's evidence table at its Review Gate | unvalidated |
| A-2 | `resolvePeerRole` is a sound single producer for every role-derived default | `internal/component/bgp/plugins/role/otc.go`, read 2026-08-08 | Children 1 and 2 both build on a resolver that answers wrongly for some peers | Unit tests in child 1, reused by child 2 | unvalidated |
| A-3 | Shipping curated prefix lists does not require a new distribution mechanism | `filter_prefix` accepts named lists today | Child 4 grows a fetcher and a refresh timer it did not budget for | Child 4 research phase | unvalidated |
| A-4 | No child needs to change the empty-chain accept | `runIngressPolicyChain`, read 2026-08-08 | A child proposes an implicit deny, which would black hole existing deployments on upgrade | Review of each child's design gate | unvalidated |
| A-5 | Splitting into six children keeps each Review Gate scope bounded | `ai/rules/planning.md` bounding rules | A child's gate cannot converge and the set stalls | First child closure | unvalidated |
| A-6 | The RFC 7999 §3.2 propagation guard can be added without committing Ze to honouring the community | §3.1 states that accepting and honouring, or ignoring, is each operator's choice | Child 1 lands a half-feature that implies blackhole support Ze does not have | Read §3.1 and §3.2 together at child 1's research gate, then the child 6 owner decision | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The set is read as a conformance claim that Ze "implements BCP 194" | A doc or release note says Ze is BCP 194 compliant | The ledger decision forbids enrolment. Documentation states mechanism and default per recommendation, never a blanket claim |
| R-2 | Children 1 and 2 both touch role resolution and collide | Two specs editing the role plugin at once | Child 1 reads the resolver and adds no mechanism to it. Child 2 owns any change to it |
| R-3 | Shipped default lists go stale and become worse than none | An operator reports a bogon list rejecting allocated space | Child 4 must ship a refresh path with any list derived from a registry, per §6.1.2.1 |
| R-4 | Four skeleton children are never written and become invisible scope reduction | The umbrella stays `ready` while children stay `skeleton` for weeks | Each child is a real file with a `## Task` on disk, so `./le spec-status` counts it |
| R-5 | The audit's verdicts go stale before a child is implemented | A child's research finds a producer that has changed | Each child re-verifies its own rows at its research gate. The inventory records the producer, not the conclusion alone |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing in this file alone. It is a roadmap. Each child carries its own blast radius, and children 1, 2 and 4 are all wire-visible or route-visible |
| How is it reverted? | Single commit revert. No runtime effect |
| Who else touches this path? | `plan/spec-pol-0-umbrella.md`, the policy roadmap whose own header says its five children are gone and it needs reconciliation. This set does not depend on it and must not be folded into it until that reconciliation happens |

## Wiring Test (MANDATORY -- NOT deferrable)

The umbrella coordinates children and ships no code of its own. Each row names
the child that owns the wiring and the test that proves it.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| peer config enables relation tagging | → | ingress relation filter | child 1: `TestRelationTagWiring` |
| route carries own-GA Function 4 naming the destination | → | egress suppression | child 1: `TestFunction4SuppressWiring` |
| new summary with zero gated MUSTs | → | `check_new_summaries` | child 1: `test_new_summary_boilerplate_only_passes` |
| eBGP peer sends a route whose first AS is not its own | → | ingress first-AS check | child 2: `TestEnforceFirstASWiring` |
| peer with a customer role and no explicit chain | → | role-derived import defaults | child 2: `TestRoleDerivedImportWiring` |
| inbound connection with a TTL below the configured minimum | → | listen socket minimum TTL | child 3: `TestListenerMinTTLWiring` |
| CoPP enabled with a source outside the peer set | → | drop term | child 3: `TestCoppDropTermWiring` |
| peer with the shipped bogon list bound on import | → | prefix list evaluation | child 4: `TestBogonListWiring` |
| RPKI NotFound route | → | local-preference degrade | child 4: `TestRPKINotFoundPrefWiring` |
| flapping route with dampening enabled | → | dampening state | child 5: `TestDampeningWiring` |
| eBGP peer sends a route carrying BLACKHOLE | → | ingress propagation guard | child 1: `TestBlackholePropagationGuardWiring`, then `test/plugin/community-blackhole-noexport.ci` |
| BLACKHOLE route meeting both §3.3 conditions | → | traffic discard | child 6: `TestBlackholeHonourWiring` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The set is complete | Every row of the Gap Inventory names exactly one owning child, and every child file exists in `plan/` |
| AC-2 | A reader asks what Ze does for an RFC 7454 recommendation | The owning child records the mechanism verdict and the default verdict separately, each naming a producing function |
| AC-3 | `rfc/not-enrolled.txt` after child 1 lands | rfc7454 and rfc8195 both read `non-normative` with text-property reasons, and neither appears in `rfc/enrolled.txt` |
| AC-4 | `./le rfc check` after child 1 lands | Passes, and neither rfc7454 nor rfc8195 is enrolled |
| AC-7 | `rfc/enrolled.txt` after child 6 lands | rfc7999 is present, `rfc/extraction/rfc7999.json` carries a classified walk, `docs/features/rfc-status.md` has an RFC 7999 row, and every one of the four MUSTs has a tagged positive and negative pair or an owner-authorised annotation |
| AC-5 | Documentation after any child lands | No file claims Ze is BCP 194 compliant. Each claim is scoped to a recommendation, a mechanism and a default |
| AC-6 | A child is closed | Its inventory rows are marked done here, and any row it could not close has a deferral row with a destination spec that exists on disk |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Reads the RFC 7454 support position for one recommendation | `rfc/short/rfc7454.md` requirement id → owning child → mechanism and default verdict | child Review Gate evidence table |
| 2 | Declares peering relationships once and gets BCP-aligned policy | `role import` → `resolvePeerRole` → derived defaults across children 1 and 2 | child 2: `TestRoleDerivedImportWiring` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBCP194InventoryOwnership` | `internal/le/` | Every Gap Inventory row names a child spec that exists in `plan/` | |

The umbrella ships no runtime code. Every other unit test belongs to a child and
is listed there.

### Boundary Tests (numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Gap Inventory rows with an owner | 1-21 | 21 | 0 rows means the inventory is empty | a row with two owners is invalid |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | N-A | The umbrella ships no user-facing behavior. Functional tests belong to the children | |

### Interop Tests (Scope: protocol)

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | The umbrella changes no wire behavior. Children 1, 2 and 4 carry the interop scenarios | |

## Files to Modify

- `rfc/not-enrolled.txt` - the two disposition rows, applied by child 1
- `docs/features/rfc-status.md` - the RFC 7947 conditional note, applied by child 1
- `plan/spec-bcp194-1-communities.md` through `plan/spec-bcp194-5-damping.md` -
  status transitions as each child starts

## Files to Create

- `plan/deferrals/bcp194-0-umbrella.md` - created when the first row is deferred

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | The umbrella ships no config. Children 1 to 5 each answer this row |
| YANG validation constraints | N-A | Same |
| YANG custom validators | N-A | Same |
| CLI commands/flags | N-A | No verb |
| CLI grammar (keyword before value) | N-A | No verb |
| Editor autocomplete | N-A | No leaf |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No command output |
| Env var registration | N-A | No leaf under `environment/` |
| Doctor check for runtime dependencies | No | The umbrella adds no runtime dependency. Child 4 adds registry feeds and answers this row |
| Prometheus counters/metrics | N-A | No runtime code |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new SAFI, capability or attribute in the whole set |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Children each answer for their own features |
| 2 | Config syntax changed? | No | Children own their leaves |
| 3 | CLI command added/changed? | No | No verb in the set |
| 4 | API/RPC added/changed? | No | No RPC in the set |
| 5 | Plugin added/changed? | No | Children own their plugins |
| 6 | Has a user guide page? | Yes | `docs/guide/bgp-policy.md` gains the BCP 194 position, written once the first child lands |
| 7 | Wire format changed? | No | No new encoding in the set |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface change |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/not-enrolled.txt` and `docs/features/rfc-status.md`, applied by child 1 |
| 10 | Test infrastructure changed? | No | Existing runners cover every child |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`, once more than one child has landed |
| 12 | Internal architecture changed? | No | No new layer or seam |
| 13 | Route metadata keys added/changed? | No | Children answer for themselves |
| 14 | Prometheus counters added/changed? | No | Children answer for themselves |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Children answer for themselves |
| 16 | Any changed source file referenced by existing doc source anchors? | No | The umbrella changes no source file |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/bgp-policy.md` documents the control-community spellings that child 1 changes |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the set exists and is discoverable
   - Tests: `TestBCP194InventoryOwnership`
   - Files: this file and the five child specs
   - Verify: `./le spec-status` lists all six, and every inventory row names a
     child that exists on disk
2. **Phase: Child 1** -- communities, per its own Implementation Steps
   - Verify: its Review Gate is clean and `./le rfc check` passes
3. **Phase: Child 2** -- role-derived policy
   - Verify: the role has a consumer outside its own plugin
4. **Phase: Child 3** -- session and speaker hardening, may run beside child 2
   - Verify: the listen socket applies a minimum receive TTL
5. **Phase: Child 4** -- prefix defaults
   - Verify: an operator who configures nothing gets a default the BCP endorses
6. **Phase: Child 5** -- dampening, starting with the owner decision
   - Verify: the position is recorded, whether or not code is written

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every Gap Inventory row has exactly one owner, and no row is orphaned by a child closing |
| Correctness | Mechanism and default are recorded separately for every RFC 7454 row. A row that conflates them is wrong even if its verdict is right |
| Correctness | No row claims conformance for a recommendation whose producer was not read |
| Naming | Child filenames follow the set prefix pattern in `ai/rules/planning.md` |
| Rule: `ai/rules/rfc-compliance.md` | No enrolment, and no invented MUST for either RFC |
| Rule: `ai/rules/planning.md` | Every deferred row names a destination spec that exists on disk |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Seven spec files exist | `ls plan/spec-bcp194-*.md` returns seven paths |
| Every inventory row has an owner | Read the Owner column. No cell is empty and none names two children |
| Neither RFC is enrolled | `grep -E "^rfc(7454\|8195)" rfc/enrolled.txt` returns nothing |
| The RFC gate is green after child 1 | `./le rfc check` |
| No blanket compliance claim exists | `grep -ri "BCP 194 compliant" docs/` returns nothing |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Guard failing open | Children 2 and 4 add guards over untrusted routing input. Each must reject rather than accept when it cannot evaluate |
| Default posture change | No child may turn an accept into a deny for an existing deployment without an explicit owner decision recorded in that child |
| Registry trust | Child 4 introduces external data as a filter input. The failure mode when a feed is unavailable must be stated per feed |

### Failure Routing

| Failure | Route To |
|---------|----------|
| A child's research contradicts an inventory row | Correct the row here, and record what changed. The audit is dated, not permanent |
| A child cannot close a row it owns | Deferral row with a destination spec that exists. Never prose |
| Two children need the same mechanism | The earlier child in Execution Order owns it. The later one reads it |
| A recommendation turns out to bind the implementation rather than the operator | Raise it with the owner. The mechanism-and-default framing does not apply to it |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- A BCP addressed to operators cannot be audited the way a Standards Track RFC
  can. The mechanism-and-default pair is what replaces a MUST-level checklist,
  and it is stricter in one respect: a mechanism that exists but defaults the
  wrong way is a finding, where a keyword scan would have recorded support.
- Two independent audits reached the same conclusion from different directions:
  the community request in T9173 and the prefix and path recommendations in RFC
  7454 both resolve to the same unused declaration.
- Reading the producer rather than the documentation is what found C2 and C3.
  Both look supported from the type and unsupported from the docs, and only the
  call graph settles it.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Five children under an umbrella | One large spec; two coarser specs | RFC 7454 §6 alone spans registry feeds, prefix bounds and RPKI preference. A single Review Gate over that plus sockets plus communities cannot converge. Owner decision, 2026-08-08 |
| Child 1 leads | Lead with role-derived policy, which builds the shared seam first | Child 1 holds three live defects and a red compliance gate. A defect on the path is fixed before new mechanism is added. Owner decision, 2026-08-08 |
| Both RFCs recorded `non-normative`, neither enrolled | Enrol both by manual walk; enrol RFC 8195 only; decide at closure | Neither specifies wire behavior, so an invented gated MUST would be a claim stronger than the text. RFC 7454's obligations bind operators, where the gate cannot tell an absent mechanism from an unconfigured one. Owner decision, 2026-08-08 |
| The empty-chain accept is preserved | Ship an implicit default-deny import chain | An implicit deny would black hole every existing deployment on upgrade. The BCP is served by shipping usable lists and role-derived defaults an operator opts into |
| TCP-AO is out of scope for the set | Fold it into child 3 | RFC 7454 §5.1's preference for TCP-AO is conditional on the implementation existing. Ze has none, so the recommendation does not yet bind. It is a feature, not a conformance gap, and needs its own spec |

## Known Limitations

- The audit is dated 2026-08-08. Every inventory row records the producing
  function so a child can re-verify it rather than trust the conclusion.
- RFC 7454 §6.1.5.2 (carrying a route for the IXP LAN so path MTU discovery
  survives loose uRPF) is operator routing guidance with no Ze code behind it.
  Child 4 answers it with documentation only.
- The set makes no claim about RFC 7454 §12, which is a summary of the document
  rather than a recommendation.
- RFC 8195 Functions 5, 7 and 8 to 12 are out of scope. The LOCAL_PREF family is
  a separate design problem that §4.3.3 and RFC 4264 both warn about.

## RFC Documentation (Scope: protocol)

The umbrella encodes no requirement in code. Each child adds the quoted
requirement above its own enforcing sites, and records the RFC 7454 requirement
id from `rfc/short/rfc7454.md` beside each mechanism it adds.

## Checklist

### Goal Gates (MUST pass)

- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify current mode full` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD

- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior, owned by the children
- [ ] Interop tests for protocol features, owned by children 1, 2 and 4

### Closure

- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/speclifecycle/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
