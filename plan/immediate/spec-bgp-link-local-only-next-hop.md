# Spec: bgp-link-local-only-next-hop

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | plan/immediate/spec-bgp-unowned-rfc-rows.md (row 3 and its scenario `bgp-linklocal-capability-interface-frr`, which proves AC-8's receive direction) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Prove what is left of `draft-ietf-idr-linklocal-capability` once the code that
performs it exists: the re-read of three rows that used to rest on an
unreachable form (AC-7), and an interop exchange of the 16-octet link-local
next hop with another implementation (AC-8), where Ze relays a received
link-local-only next hop unchanged. Ze originating its own link-local as the
only next hop is out of scope and recorded as a gap (owner, 2026-10-11).
Nothing else.

Owner decision, 2026-10-11, on the round-2 design review's I5 ("the narrowed
skeleton owns no open row"): "Rewrite to AC-7/AC-8". This spec was written on
2026-09-01, when Ze advertised capability 77 and performed none of the draft.
That is no longer true, and the earlier text claimed rows the ledger shows met:

| Earlier claim | What the source and the ledger say now |
|---------------|----------------------------------------|
| `parseCapability` holds no case for 77 and no negotiated state is recorded | `internal/core/bgp/capability/capability.go` declares `CodeLinkLocalNextHop` (77) and parses it as a typed `LinkLocalNextHop`; `Negotiate` in `internal/core/bgp/capability/negotiated.go` records the negotiated `LinkLocalNextHop` |
| Seven MUST-level rows (`-3-1`, `-4-1`, `-4-3`, `-4-4`, `-4-7`, `-4-9`, `-4-5`, `-4-6`) have no producer | `rfc/short/draft-ietf-idr-linklocal-capability.md` "Enrolment reason" records each as met and names its producer: `Peer.linkLocalOnlyNextHopPermitted` and `buildMPReach` for `-3-1` and `-5-1`; `egressNextHopIsLinkLocalOnly` and `sameLinkLayerSegment` for `-4-5`/`-4-6`; a route whose next hop has no wire form refused for `-4-1`/`-4-4`/`-4-7`/`-4-9` |
| The draft's open rows are this spec's | The summary carries `{gap}` on two rows only, `-1-2` and `-6-1`, and both are owned by `plan/immediate/spec-bgp-unowned-rfc-rows.md` (its rows 3 and 4) |

So the ACs that built the feature (the former AC-1 to AC-6 and AC-9) are
dropped as already met by other work, and this spec keeps AC-7 and AC-8.

## Required Reading

- `rfc/short/draft-ietf-idr-linklocal-capability.md` and
  `rfc/drafts/draft-ietf-idr-linklocal-capability.txt` (revision -06)
  → Constraint: the draft text is the authority; the summary is derived. Rows
    `-1-2` and `-6-1` are not this spec's.
- `internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_send_test.go`,
  `draft_ietf_idr_linklocal_capability_test.go`,
  `draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go`,
  `draft_ietf_idr_linklocal_capability_multihop_test.go`
  → Constraint: these already carry `RFC requirement:` tags for `-5-1`, `-4-2`
    and `-4-8`, and `rfc/discrimination/draft-ietf-idr-linklocal-capability.json`
    holds records for all three (grep, 2026-10-11). `-4-2` carries positive and
    negative tags (`draft_ietf_idr_linklocal_capability_test.go` lines 73 and 84;
    `draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go` lines 155 and
    158). AC-7 re-reads them; it writes new tests only where a re-read finds one
    that does not drive the reachable form.
- `plan/immediate/spec-bgp-update-propagation-rfc-defects.md`, D6
  → Constraint: D6 is fixed. Its row reads "Fixed, including mixed post-policy
    output and section-specific withdrawals ... D6 implementation and independent
    round 3 review complete" (round-3 I1, read 2026-10-11). The 2026-09-27 record
    of `-4-2` as unmet is superseded, so AC-7 re-reads `-4-2` like the other two
    rows.
- `docs/contributing/rfc-conformance-gates.md`
  → Constraint: a `{gap}` row with a positive or negative tag is refused as a
    stale annotation, and an author-written `{gap}` is not an escape from the
    coverage ratchet, so a met row with tags is never moved to `{gap}` to clear a
    finding.
- `docs/architecture/testing/interop.md`
  → Constraint: the scenario is named, owes a recorded red with the fix reverted
    and the artifact rebuilt.
- `test/interop/scenarios/bgp-linklocal-only-multihop-withdraw-frr/`
  → Decision: AC-8's session runs over IPv4, as this scenario's does (`frr.conf`
    `neighbor 172.30.0.2 capability link-local`, `ze.conf` `link-local
    fe80::be:ef:2`), because Ze refuses a link-local session address
    (`reactor_peers.go`, "link-local addresses not supported for local-address").
  → Decision: AC-8 reuses this scenario's injector (`inject.msg`, `inject-args`,
    a `peer injector` Ze dials with `accept false`), placed on the SAME link as
    Ze and FRR, so the relayed link-local is a third-party next hop FRR can use
    (round-4 I1).
- [ ] `internal/core/bgp/attribute/nexthop_form.go` (`ValidateGlobalNextHop`),
  `internal/component/bgp/reactor/session_connection.go` (`connectedLocalAddress`),
  `internal/component/bgp/reactor/peer.go` (`canUseNextHopFor`)
  → Constraint: Ze has no path that writes its OWN link-local as the only next
    hop: next-hop self resolves to the IPv4 session address, refused for IPv6
    unicast without extended next hop; an explicit `fe80::` next hop is refused
    at every route-level entry point ("RFC 2545 Section 3"); the `session
    link-local` leaf fills only the second address after a global. The only
    producer of the 16-octet form is a relay of a received one with the next hop
    unchanged (round-4 I1).

## Current Behavior (MANDATORY)

Source files read:

- [ ] `internal/core/bgp/capability/capability.go` - `CodeLinkLocalNextHop`, the typed `LinkLocalNextHop` case
- [ ] `internal/core/bgp/capability/negotiated.go` - `Negotiate`, the `LinkLocalNextHop` field
- [ ] `internal/component/bgp/reactor/peer.go` - `Peer.linkLocalOnlyNextHopPermitted`
- [ ] `internal/component/bgp/message/update_build.go` - `buildMPReach`, the 16-octet and 32-octet forms
- [ ] `internal/component/bgp/reactor/forward_next_hop.go` - `egressNextHopIsLinkLocalOnly`
- [ ] `internal/component/bgp/reactor/link_scope.go` - `sameLinkLayerSegment`
- [ ] `internal/component/bgp/reactor/reactor_peers.go` - the refusal of a link-local `local-address`

Capability 77 is advertised, parsed, negotiated and acted on for the send side
and the reflection path. `test/interop/scenarios/bgp-linklocal-only-multihop-withdraw-frr/`
exists and covers the multihop withdraw. No scenario shows another
implementation installing the 16-octet link-local next hop Ze sends on a shared
link.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

An operator sets `link-local-nexthop` and `link-local` on a peer facing a
speaker that also advertises capability 77, over an IPv4 session on the shared
link.

### Transformation Path

Send: OPEN capability 77 negotiated, `linkLocalOnlyNextHopPermitted`,
`buildMPReach` writes the 16-octet link-local in the IPv6 unicast MP_REACH.
Receive: MP_REACH_NLRI with a 16-octet link-local, carried with the session
interface to the FIB, which `plan/immediate/spec-bgp-unowned-rfc-rows.md` row 3
builds and its scenario `bgp-linklocal-capability-interface-frr` proves.

### Boundaries Crossed

The BGP wire, against a peer daemon that implements the draft.

### Integration Points

None new: AC-7 reads and tests existing producers; AC-8 adds a scenario and its
checker.

## Risks & Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | FRR implements the draft's capability 77 and the 16-octet form | FRR source: `bgpd/bgp_attr.c` (`PEER_HAS_LINK_LOCAL_CAPABILITY`), also `bgp_nht.c` and `bgp_updgrp_packet.c` (round-3 review N2); the existing `bgp-linklocal-only-multihop-withdraw-frr` scenario already negotiates capability 77 between FRR 10.4.1 and Ze | AC-8 has no peer, which `ai/rules/interop-and-goal-validation.md` refuses; the owner is asked which implementation to use | the existing scenario's negotiation | holds |
| A-2 | The existing tagged tests for `-5-1`, `-4-2`, `-4-8` drive the form Ze now emits | tags and discrimination records exist (grep, 2026-10-11) | a test that passes because the form is unreachable is vacuous; AC-7 replaces it | AC-7 | unvalidated |

| ID | Risk | Mitigation |
|----|------|-----------|
| R-1 | The `-4-2` re-read finds that a received link-local forwarded under next-hop auto or unchanged reaches a peer more than one hop away again, a regression of D6 | it is a defect in an implemented capability and is fixed in this spec (`ai/rules/completion.md`): the producer changes, the tagged tests go red on the regression and green on the fix, and the row stays met; the ledger is never moved to `{gap}` to absorb it |
| R-2 | AC-8's receive direction rests on row 3 of the unowned-rows spec | the receive direction is proven by that spec's scenario `bgp-linklocal-capability-interface-frr`, and this spec's scenario asserts the send direction only, so it lands on its own |

## Blast Radius

Tests and the ledger only, unless AC-7 finds a vacuous test (then that test is
rewritten) or a regression of `-4-2` (then its producer is fixed here). No other
production code is owed by this spec.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An operator sets `link-local-nexthop` on a peer, and the peer advertises capability 77 | → | `Peer.linkLocalOnlyNextHopPermitted`, `buildMPReach` | `bgp-linklocal-only-next-hop-frr` interop scenario |

AC-7 has no row here: it adds no entry point, it re-reads tests over existing
producers (round-3 N3).

## Acceptance Criteria

| ID | When | Then |
|----|------|------|
| AC-7 | `-5-1`, `-4-2` and `-4-8` are re-read against the producers the summary names | each row's tagged test is shown, by its discrimination record and by reading its body, to drive the form Ze now emits rather than rest on the form being unreachable; a test that does not is rewritten and re-recorded. For `-4-2` the re-read covers the D6 forms: a received link-local forwarded under next-hop auto and under unchanged, to a peer on the shared segment and to one more than one hop away. If a form regressed, the producer is fixed in this spec and the tagged test proves it (R-1); the row stays met. `./le rfc check` exits clean for the stem. `-6-1` and `-1-2` are not in this list |
| AC-8 | The interop scenario `bgp-linklocal-only-next-hop-frr` runs: the injector (the `bgp-linklocal-only-multihop-withdraw-frr` pattern), Ze and FRR on one shared link, each BGP session over IPv4 addresses on that link (single hop), the IPv6 unicast family negotiated with capability 77 on both sessions, no global IPv6 address on the link, Ze relaying with `next-hop unchanged`; the injector announces a prefix whose MP_REACH Next Hop is 16 octets holding only the injector's own link-local on the shared interface (owner decision 2026-10-11, relay only) | Ze relays the route to FRR with that 16-octet link-local unchanged: the checker asserts the captured Next Hop field of Ze's UPDATE to FRR equals the injector's link-local, and FRR installs the route via that link-local through the shared interface, read from FRR's RIB and kernel. The receive direction, FRR's 16-octet link-local next hop installed by Ze with the session interface, is AC-3 of `plan/immediate/spec-bgp-unowned-rfc-rows.md` and its scenario `bgp-linklocal-capability-interface-frr`; this scenario does not repeat it. The revert, rebuild, red, restore, green walk is recorded |

## End-to-End User Stories

| Story | Path |
|-------|------|
| An operator runs Ze as a route server or transit on a shared link with IPv4 sessions, carries IPv6 unicast routes, and configures no global IPv6 address on the link; a speaker on that link announces a route with only its link-local as next hop | every speaker advertises 77; Ze relays the route to the other speaker with that 16-octet link-local unchanged, and FRR installs a usable route via the announcing speaker through the link (`bgp-linklocal-only-next-hop-frr`); Ze advertising its own link-local as the only next hop is not offered (gap, owner decision 2026-10-11); FRR's 16-octet link-local next hop is installed by Ze through the link (`bgp-linklocal-capability-interface-frr`, unowned-rows spec) |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| the existing `-5-1`, `-4-2`, `-4-8` tagged tests, re-read | `internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_*_test.go` | AC-7 | |

### Interop

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-linklocal-only-next-hop-frr` | `test/interop/scenarios/bgp-linklocal-only-next-hop-frr` | FRR | AC-8: the 16-octet link-local Ze relays unchanged from a speaker on the same link is accepted and installed by FRR | |
| `bgp-linklocal-capability-interface-frr` (created by the unowned-rows spec) | `test/interop/scenarios/bgp-linklocal-capability-interface-frr` | FRR | AC-8 receive direction, by reference | |

## Files to Modify

- `rfc/short/draft-ietf-idr-linklocal-capability.md`, `docs/features/rfc-status.md` - AC-7, only where the re-read corrects a claim; and the origination gap (round-4 I1): the support-remaining line names Ze's own link-local as the only next hop as an implementation gap with the §4 item 2 MAY and the owner's 2026-10-11 decision, plus `excluded-kind: feature-out-of-scope` on any row conditional on it
- `internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_*_test.go` - AC-7, only a test the re-read finds vacuous
- `internal/component/bgp/reactor/forward_next_hop.go` - AC-7, only if the `-4-2` re-read finds a regression (R-1)
- `internal/le/interoplab/bgp/check_special.go` - map entry for `bgp-linklocal-only-next-hop-frr`
- `docs/architecture/testing/interop.md` - a paragraph for `bgp-linklocal-only-next-hop-frr` beside the existing `bgp-linklocal-only-multihop-withdraw-frr` one

## Files to Create

- `test/interop/scenarios/bgp-linklocal-only-next-hop-frr/`
- `internal/le/interoplab/bgp/check_linklocal_only.go` and its `_test.go` - the scenario checker

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no new leaf; `link-local-nexthop` and `link-local` exist |
| YANG validation constraints | N-A | none new |
| YANG custom validators | N-A | none new |
| CLI commands/flags | N-A | no command changes |
| CLI grammar (keyword before value) | N-A | no command changes |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no new runtime dependency |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | capability 77 and IPv6 unicast exist; the work is interop and a re-read |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | the feature exists; AC-8 adds interop evidence recorded in the summary (item 9) |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | no behavior changes |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/draft-ietf-idr-linklocal-capability.md` (the interop evidence, and any claim AC-7 corrects) and `docs/features/rfc-status.md` where AC-7 corrects a claim |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md`: the new scenario's paragraph |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | No | none, unless the R-1 fix lands: then `docs/architecture/core-design.md` (egress route decisions on the forward rails), the page `forward_next_hop.go` declares |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/testing/interop.md`, declared by `check_special.go`: the new checker gets its `<!-- source: -->` line; `docs/architecture/core-design.md`, declared by `forward_next_hop.go`, only if the R-1 fix lands |
| 17 | Existing docs show config/CLI/API examples for this area? | No | none changed |

## Implementation Steps

1. AC-7 re-read of `-5-1`, `-4-2`, `-4-8`; correct any vacuous test, and fix a `-4-2` regression at its producer if the re-read finds one. Own commit.
2. AC-8 scenario `bgp-linklocal-only-next-hop-frr` (send direction), with the recorded red. Own commit.

## Design Insights

- The feature was built by other work after this spec was written; what remains
  is proof, and a proof that rests on an unreachable form proves nothing.

## Key Design Decisions

- Interop is the evidence for AC-8. A capability-gated wire format proven only
  against Ze proves the encoder agrees with itself (`ai/rules/interop-and-goal-validation.md`).
- AC-8 runs over an IPv4 session on the shared link, because Ze refuses a
  link-local session address; "no global next hop" means no global IPv6 address
  on the link, not an IPv6 session (round-3 I2).
- The receive direction is proven once, by the unowned-rows spec's
  `bgp-linklocal-capability-interface-frr`; a second scenario asserting the same
  install would be a second copy of one proof (round-3 I2).
- AC-8 proves a relay, not origination (round-4 I1, owner 2026-10-11): the
  only path in Ze that writes a 16-octet link-local next hop is a received one
  passed through unchanged, so the scenario's producer is the injector on the
  shared link and Ze's part is the relay. The draft permits that form as a
  third-party next hop for an external peer: "Similarly, the speaker can use
  the received Link-Local IPv6 address, provided that peer X is directly
  attached." (§4, item 2).
- Origination is an OPTIONAL feature recorded as a gap, not built here. The
  draft's default arm for an external peer: "If the BGP speaker is one IP hop
  away, the Link-Local IPv6 address SHOULD be included, and MAY be the only next
  hop address in the next hop." (§4, item 2). Ze never makes its own link-local
  the only next hop. The summary records it in its support-remaining line as an
  implementation gap naming this scope decision, and any row whose obligation
  is conditional on Ze sending its own link-local as the only next hop takes
  `excluded-kind: feature-out-of-scope` with that quote, its MAY, and the
  owner's words. A MUST that binds whether or not Ze offers the feature is never
  relabeled: the AC-7 walk re-reads `-4-3` ("If the route is directly connected
  to the speaker, ... the next hop MUST include its own Link-Local IPv6
  address.", §4, item 1) for an internal peer on a link with no global address,
  and if it binds there and Ze sends nothing, it is a counted conformance gap
  reported to the owner, not an exclusion.
- A `-4-2` regression is a defect fixed here, never a ledger move to `{gap}`:
  the gates refuse a tagged `{gap}`, and a met row that broke is a defect in an
  implemented capability (round-3 I1).

## Owner decisions (2026-10-11)

| Question | Owner's words | What it means here |
|----------|---------------|--------------------|
| Round-2 I5: the narrowed skeleton owns no open row; close it or rewrite it? | "Rewrite to AC-7/AC-8" | This spec keeps AC-7 and AC-8 only; the false claims about capability 77 and the seven rows are removed |
| Round-4 I1: AC-8 names no path that produces Ze's own link-local as the only next hop; relay a received one, or build origination? | "Relay only, for now (Recommended)" | AC-8 is a relay from a speaker on the same link; origination is out of scope and recorded as an implementation gap with the draft's §4 MAY and this decision (Key Design Decisions) |

## Known Limitations

- The draft is revision -06 and not yet an RFC. A later revision can change the
  wire format, and the summary records the revision the walk read.
- Ze does not originate a link-local-only next hop: on a link with no global
  IPv6 address it relays one it received and never advertises its own
  (owner decision 2026-10-11, "Relay only, for now"; gap recorded in the
  summary).

## RFC Documentation (Scope: protocol)

`rfc/short/draft-ietf-idr-linklocal-capability.md` carries the gated rows.
`rfc/drafts/draft-ietf-idr-linklocal-capability.txt` is the authority; the
summary is derived and is never the evidence.

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
- [ ] AC-7 and AC-8 demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket
