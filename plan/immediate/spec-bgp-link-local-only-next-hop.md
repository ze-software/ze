# Spec: bgp-link-local-only-next-hop

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Implement the procedures of `draft-ietf-idr-linklocal-capability`, which Ze
advertises and does not perform.

`extractLLNHCapabilities` (`internal/component/bgp/plugins/llnh/llnh.go`)
advertises BGP capability code 77 for any peer or group whose config carries a
`link-local-nexthop` key that is not `disable`. `parseCapability`
(`internal/core/bgp/capability/capability.go`) holds **no case for 77**, so a
received one becomes an `Unknown` nothing reads, no negotiated state is
recorded, and no Ze path branches on the capability.

Section 2 makes every later procedure conditional on the capability being
negotiated by BOTH speakers, and Ze reaches that condition. So these obligations
are not dormant: they bind on any session where the peer also implements the
draft, which is the deployment the capability exists to create.

Found by the extraction walk of 2026-09-01
(`plan/pre-release/spec-rfcgate-6-supported-extraction-signoff.md`). The public row read
`Supported` until that day and now reads `Partial` (`7c3cc15ed`), which
discloses the gap without owing it to anybody. This spec owes it.

## Required Reading

- `internal/component/bgp/plugins/llnh/llnh.go` — `extractLLNHCapabilities`
  → Constraint: the advertisement exists and is config-driven. Nothing else in
    the draft does.
- `internal/core/bgp/capability/capability.go` — `parseCapability`
  → Decision: code 77 needs a case and a negotiated-state field the reactor can
    branch on. Everything downstream depends on that one answer existing.
- `internal/component/bgp/reactor/link_scope.go` — `linkScope.linkLocalNextHop`,
  `applyLinkLocalNextHop`
  → Constraint: these emit the RFC 2545 forms only, a 16-octet GLOBAL address or
    the 32-octet global-then-link-local pair. The draft's form is a 16-octet
    LINK-LOCAL, which no producer emits.
- `internal/core/bgp/attribute/nexthop_form.go` — `ValidateGlobalNextHop`
  → Constraint: it refuses a link-local address in the first slot, so the send
    path actively rejects the draft's encoding today.
- `internal/core/bgp/attribute/mpnlri.go` — `parseNextHops`
  → Constraint: no `fe80::/10` test. A 16-octet link-local next hop is read as a
    Global IPv6 next hop, so nothing downstream can tell them apart.
- `internal/component/bgp/reactor/peer_forward_facts.go` — `precomputeNextHop`
  → Constraint: it settles a next hop from config for every IPv6 peer, so the
    draft's "no next hop remains" outcome is unreachable and no code decides it.
- `internal/component/bgp/plugins/rr/` — the whole package
  → Constraint: it holds no link-local or `fe80` reference at all, so §4's two
    route-reflector rules have no producer.
- `rfc/short/draft-ietf-idr-linklocal-capability.md` and
  `rfc/drafts/draft-ietf-idr-linklocal-capability.txt` (revision -06)
  → Constraint: 13 gated MUST-level rows. The draft text is the authority; the
    summary is derived.

## Current Behavior (MANDATORY)

Source files read:

- [ ] `internal/component/bgp/plugins/llnh/llnh.go`
- [ ] `internal/core/bgp/capability/capability.go`
- [ ] `internal/component/bgp/reactor/link_scope.go`
- [ ] `internal/core/bgp/attribute/nexthop_form.go`
- [ ] `internal/core/bgp/attribute/mpnlri.go`
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go`

Ze advertises the capability and implements none of what it licenses. Seven
MUST-level obligations have no producer:

| Id | Section | Obligation | Why unmet |
|----|---------|-----------|-----------|
| `-3-1` | 3 | Next Hop length 16, holding only the Link-Local address | `linkLocalNextHop` emits 16-octet GLOBAL or the 32-octet pair; `ValidateGlobalNextHop` refuses a link-local first slot |
| `-4-1` | 4 | No IPv6 next hop remains, so the route is not advertised | `precomputeNextHop` always settles one; the empty outcome is unreachable |
| `-4-3` | 4 | A directly-connected route, or one whose next hop is the internal peer's address, includes the speaker's own Link-Local | `linkLocalNextHop` decides on RFC 2545 §3's shared-subnet test alone, reading neither the connected status nor the peer's address |
| `-4-4`, `-4-7`, `-4-9` | 4 | Three treat-as-withdraw outcomes when no eligible next hop remains | same unreachable-antecedent as `-4-1` |
| `-4-5`, `-4-6` | 4 | A Route Reflector must not reflect a link-local-only next hop off-segment, and must next-hop-self or withhold | `internal/component/bgp/plugins/rr` names neither |

Three more are met VACUOUSLY and must not be recorded as met once the feature
exists: `-5-1` (Ze never emits the form the draft forbids off-negotiation),
`-6-1` (`parseNextHops` refuses every IPv6 length but 16 and 32 with
`ErrInvalidNextHopLen`), and `-4-2`/`-4-8` (`peerOnLink`, set from
`network.SharesSubnet`, gates the link-local append).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

An operator sets `link-local-nexthop` on a peer or group; the session negotiates
capability 77 with a peer that also advertises it.

### Transformation Path

OPEN → `parseCapability` → **negotiated state (absent today)** → the reactor's
forward path → `precomputeNextHop` → `linkScope.linkLocalNextHop` /
`applyLinkLocalNextHop` → `ValidateGlobalNextHop` → the MP_REACH_NLRI encoder.
On receive: MP_REACH_NLRI → `parseNextHops` → **`fe80::/10` classification
(absent today)** → next-hop resolvability → the decision process.

### Boundaries Crossed

The BGP wire, in both directions, and the peer's own conformance: this is
capability-gated, so every behavior is observable only against a peer that
advertises 77. Interop is therefore the evidence, not a unit test.

### Integration Points

`llnh` reads the operator's `link-local-nexthop` key and advertises the
capability; the capability parser records what the peer answered; the reactor's
forward path reads that state to choose an encoding; the attribute layer encodes
and decodes the Next Hop field; and the route reflector reads the decoded form
to decide eligibility. No new component and no new plugin: each seam already
exists and carries one more fact.

## Risks & Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Capability 77 negotiated state can be carried where the reactor already reads negotiated capabilities | the same shape serves ASN4, Add-Path and Extended Next Hop | the encode and receive paths need a new channel each | AC-1 | unvalidated |
| A-2 | An interop peer implementing the draft exists to test against | FRR and BIRD are already in `test/interop/scenarios/` | the wire behavior is proven only against Ze itself, which `ai/rules/interop-and-goal-validation.md` refuses | AC-8 | unvalidated |
| A-3 | The three vacuously-met rows stay met once the form is emitted | each rests on Ze never producing the form | a fix creates the very case they assume away | AC-7 | unvalidated |

| ID | Risk | Mitigation |
|----|------|-----------|
| R-1 | Emitting a 16-octet link-local next hop to a peer that did not negotiate 77 breaks a working session | `-5-1` is the rule against exactly that; the encode is gated on negotiated state, and AC-2 asserts the un-negotiated case still sends 32 octets |
| R-2 | The receive-side `fe80::/10` classification changes how existing routes resolve | `-4-13` makes a link-local-only next hop unusable for forwarding; the change is observable and AC-6 pins it |
| R-3 | The route-reflector rules touch a package with no link-local concept | `-4-5` and `-4-6` are the two the walk found; both are wire-visible and owed an interop case |

## Blast Radius

`internal/core/bgp/capability/`, `internal/core/bgp/attribute/`,
`internal/component/bgp/reactor/` and `internal/component/bgp/plugins/rr/`. The
receive-side classification changes how an existing 16-octet next hop is read,
so every IPv6 peer is in scope even where the capability is off.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An operator sets `link-local-nexthop` on a peer, and the peer advertises capability 77 | → | `linkScope.linkLocalNextHop` (`internal/component/bgp/reactor/link_scope.go`) | `link-local-only-next-hop` interop scenario |
| A peer's OPEN carries capability 77 | → | `parseCapability` (`internal/core/bgp/capability/capability.go`) | `TestLinkLocalCapabilityNegotiatedStateRecorded` |

## Acceptance Criteria

| ID | When | Then |
|----|------|------|
| AC-1 | A peer advertises capability 77 | `parseCapability` records it and the session exposes the negotiated state |
| AC-2 | The capability is NOT negotiated | the Next Hop is encoded as 32 octets, per `-5-1` |
| AC-3 | The capability IS negotiated and only a link-local address is available | the Next Hop field is 16 octets and holds the link-local address, per `-3-1` |
| AC-4 | No eligible IPv6 next hop remains after the §4 procedures | the route is not advertised, per `-4-1`, `-4-4`, `-4-7` and `-4-9` |
| AC-5 | A Route Reflector holds a route with a link-local-only next hop | it is withheld from a client off the segment, and next-hop-self or withheld for all others, per `-4-5` and `-4-6` |
| AC-6 | A route arrives with a link-local-only next hop | it is classified as such and treated as unusable for forwarding, per `-4-13` |
| AC-7 | `-5-1`, `-6-1`, `-4-2` and `-4-8` are re-read after the feature exists | each is still met, and each carries a test rather than resting on the form being unreachable |
| AC-8 | The interop scenario runs | Ze exchanges a link-local-only next hop with a peer daemon that implements the draft |
| AC-9 | `./le rfc check` is run | no violation names `draft-ietf-idr-linklocal-capability`, and its `docs/features/rfc-status.md` row states what is now true |

## End-to-End User Stories

| Story | Path |
|-------|------|
| An operator peers over IPv6 with no global addresses configured on the link | both speakers advertise 77, Ze sends a 16-octet link-local next hop, and the peer installs a usable route |
| A route reflector serves clients on different segments | a link-local-only route reaches only the clients sharing the advertiser's segment; the rest get next-hop-self or nothing |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLinkLocalCapabilityNegotiatedStateRecorded` | `internal/core/bgp/capability/capability_test.go` | AC-1 | |
| `TestNextHopIs32OctetsWithoutTheCapability` | `internal/component/bgp/reactor/link_scope_test.go` | AC-2 | |
| `TestNextHopIs16OctetLinkLocalWithTheCapability` | `internal/component/bgp/reactor/link_scope_test.go` | AC-3 | |
| `TestRouteWithheldWhenNoNextHopRemains` | `internal/component/bgp/reactor/link_scope_test.go` | AC-4 | |
| `TestReflectorWithholdsLinkLocalOffSegment` | `internal/component/bgp/plugins/rr/rr_test.go` | AC-5 | |
| `TestReceivedLinkLocalOnlyNextHopIsUnusable` | `internal/core/bgp/attribute/mpnlri_test.go` | AC-6 | |

### Interop

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `link-local-only-next-hop` | `test/interop/scenarios/link-local-only-next-hop` | FRR or BIRD | AC-8: the 16-octet form is accepted and installed by another implementation | |

## Files to Modify

- `internal/core/bgp/capability/capability.go` — a case for code 77
- `internal/core/bgp/attribute/mpnlri.go` — `fe80::/10` classification on receive
- `internal/core/bgp/attribute/nexthop_form.go` — permit the link-local-only form when negotiated
- `internal/component/bgp/reactor/link_scope.go` — emit the 16-octet form
- `internal/component/bgp/reactor/peer_forward_facts.go` — let "no next hop" be an outcome
- `internal/component/bgp/plugins/rr/` — the two reflector rules
- `rfc/short/draft-ietf-idr-linklocal-capability.md`, `docs/features/rfc-status.md`

## Files to Create

- `test/interop/scenarios/link-local-only-next-hop/`
- the retired deferral shard "bgp-link-local-only-next-hop"

## Implementation Steps

1. Negotiated state for capability 77, and the un-negotiated 32-octet rule
   (`-5-1`) pinned first, so nothing later can regress a working session.
2. The receive-side classification, which changes how an existing 16-octet next
   hop is read and so is the widest blast radius.
3. The send-side 16-octet form, gated on negotiation.
4. The §4 eligibility procedures and their four treat-as-withdraw outcomes.
5. The two route-reflector rules.
6. The interop scenario, then the tagged tests and discrimination records.

## Design Insights

- The capability is advertised without the procedures, which is the shape
  `ai/rules/principles.md` calls a feature that registers itself and does
  nothing. A peer that trusts the advertisement gets no benefit and no error.
- Every obligation is conditional on negotiation, so nothing here changes a
  session with a peer that does not implement the draft — except step 2, which
  is why it is sequenced early and alone.

## Key Design Decisions

- Interop is the evidence. A capability-gated wire format proven only against Ze
  proves the encoder agrees with itself (`ai/rules/interop-and-goal-validation.md`).

## Known Limitations

- The draft is revision -06 and not yet an RFC. A later revision can change the
  wire format, and the summary records the revision the walk read.

## RFC Documentation (Scope: protocol)

`rfc/short/draft-ietf-idr-linklocal-capability.md` carries the 13 gated rows and
is the checklist this spec closes. `rfc/drafts/draft-ietf-idr-linklocal-capability.txt`
is the authority; the summary is derived and is never the evidence.

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
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated, not library-only
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket
