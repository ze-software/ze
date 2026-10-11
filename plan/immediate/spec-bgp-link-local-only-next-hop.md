# Spec: bgp-link-local-only-next-hop

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | plan/immediate/spec-bgp-unowned-rfc-rows.md (row 3, for AC-8's receive half) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Prove what is left of `draft-ietf-idr-linklocal-capability` once the code that
performs it exists: the re-read of three rows that used to rest on an
unreachable form (AC-7), and an interop exchange of the 16-octet link-local
next hop with another implementation (AC-8). Nothing else.

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
    holds records for all three (grep, 2026-10-11). AC-7 re-reads them; it
    writes new tests only where a re-read finds one that does not drive the
    reachable form.
- `plan/immediate/spec-bgp-update-propagation-rfc-defects.md`, D6
  → Constraint: on 2026-09-27 this spec recorded `-4-2` as unmet for a received
    link-local forwarded under next-hop auto or unchanged (D6 there), while the
    summary now records `-4-2` as met. AC-7 settles which is true at the
    producer.
- `docs/architecture/testing/interop.md`
  → Constraint: the scenario is named, owes a recorded red with the fix reverted
    and the artifact rebuilt.

## Current Behavior (MANDATORY)

Source files read:

- [ ] `internal/core/bgp/capability/capability.go` - `CodeLinkLocalNextHop`, the typed `LinkLocalNextHop` case
- [ ] `internal/core/bgp/capability/negotiated.go` - `Negotiate`, the `LinkLocalNextHop` field
- [ ] `internal/component/bgp/reactor/peer.go` - `Peer.linkLocalOnlyNextHopPermitted`
- [ ] `internal/component/bgp/message/update_build.go` - `buildMPReach`, the 16-octet and 32-octet forms
- [ ] `internal/component/bgp/reactor/forward_next_hop.go` - `egressNextHopIsLinkLocalOnly`
- [ ] `internal/component/bgp/reactor/link_scope.go` - `sameLinkLayerSegment`

Capability 77 is advertised, parsed, negotiated and acted on for the send side
and the reflection path. `test/interop/scenarios/bgp-linklocal-only-multihop-withdraw-frr/`
exists and covers the multihop withdraw. No scenario exchanges the 16-octet
link-local form in both directions with another implementation.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

An operator sets `link-local-nexthop` on a peer facing a speaker that also
advertises capability 77.

### Transformation Path

Send: OPEN capability 77 negotiated, `linkLocalOnlyNextHopPermitted`,
`buildMPReach` writes the 16-octet link-local. Receive: MP_REACH_NLRI with a
16-octet link-local, carried with the session interface to the FIB, which
`plan/immediate/spec-bgp-unowned-rfc-rows.md` row 3 builds.

### Boundaries Crossed

The BGP wire in both directions, against a peer daemon that implements the
draft.

### Integration Points

None new: AC-7 reads and tests existing producers; AC-8 adds a scenario and its
checker.

## Risks & Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | FRR implements the draft's capability 77 and the 16-octet form | `plan/immediate/spec-bgp-unowned-rfc-rows.md` names FRR's `PEER_HAS_LINK_LOCAL_CAPABILITY` for its `bgp-linklocal-capability-interface-frr` scenario | AC-8 has no peer, which `ai/rules/interop-and-goal-validation.md` refuses; the owner is asked which implementation to use | build the FRR scenario and capture the OPEN | unvalidated |
| A-2 | The existing tagged tests for `-5-1`, `-4-2`, `-4-8` drive the form Ze now emits | tags and discrimination records exist (grep, 2026-10-11) | a test that passes because the form is unreachable is vacuous; AC-7 replaces it | AC-7 | unvalidated |

| ID | Risk | Mitigation |
|----|------|-----------|
| R-1 | The `-4-2` re-read finds D6 still open, so the summary's "met" is a false public claim | the summary row and its `docs/features/rfc-status.md` row are corrected in the same change (`ai/rules/rfc-compliance.md`); the fix stays D6's, in its own spec |
| R-2 | AC-8's receive half runs before row 3 of the unowned-rows spec lands | the Depends field; the send half can land first and the receive assertion follows row 3 |

## Blast Radius

Tests and the ledger only, unless AC-7 finds a vacuous test (then that test is
rewritten) or a false met claim (then the summary is corrected). No production
code is owed by this spec.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An operator sets `link-local-nexthop` on a peer, and the peer advertises capability 77 | → | `Peer.linkLocalOnlyNextHopPermitted`, `buildMPReach` | `bgp-linklocal-only-next-hop-frr` interop scenario |

## Acceptance Criteria

| ID | When | Then |
|----|------|------|
| AC-7 | `-5-1`, `-4-2` and `-4-8` are re-read against the producers the summary names | each row's tagged test is shown, by its discrimination record and by reading its body, to drive the form Ze now emits rather than rest on the form being unreachable; a test that does not is rewritten and re-recorded. `-4-2` is settled against D6 of `plan/immediate/spec-bgp-update-propagation-rfc-defects.md`: where a received link-local forwarded under next-hop auto or unchanged is still sent to a peer more than one hop away, the summary's met claim is corrected to `{gap}` naming D6 in the same change. `./le rfc check` exits clean for the stem. `-6-1` and `-1-2` are not in this list |
| AC-8 | The interop scenario `bgp-linklocal-only-next-hop-frr` runs, both speakers advertising capability 77 on one link with no global next hop available | Ze's UPDATE carries a 16-octet link-local Next Hop (capture asserted) and FRR installs the route through the shared interface; FRR's 16-octet link-local next hop reaches Ze and the kernel route Ze installs names the session interface (row 3 of the unowned-rows spec). The revert, rebuild, red, restore, green walk is recorded |

## End-to-End User Stories

| Story | Path |
|-------|------|
| An operator peers over IPv6 with no global addresses configured on the link | both speakers advertise 77, each sends a 16-octet link-local next hop, and each installs a usable route through the link |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| the existing `-5-1`, `-4-2`, `-4-8` tagged tests, re-read | `internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_*_test.go` | AC-7 | |

### Interop

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-linklocal-only-next-hop-frr` | `test/interop/scenarios/bgp-linklocal-only-next-hop-frr` | FRR | AC-8: the 16-octet form is sent, accepted and installed in both directions | |

## Files to Modify

- `rfc/short/draft-ietf-idr-linklocal-capability.md`, `docs/features/rfc-status.md` - AC-7, only where the re-read corrects a claim
- `internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_*_test.go` - AC-7, only a test the re-read finds vacuous
- `internal/le/interoplab/bgp/check_special.go` - map entry for `bgp-linklocal-only-next-hop-frr`

## Files to Create

- `test/interop/scenarios/bgp-linklocal-only-next-hop-frr/`
- `internal/le/interoplab/bgp/check_linklocal_only.go` and its `_test.go` - the scenario checker

## Implementation Steps

1. AC-7 re-read of `-5-1`, `-4-2`, `-4-8`; correct any vacuous test or false claim. Own commit.
2. AC-8 send half once A-1 holds; receive half after row 3 of the unowned-rows spec lands. Own commit, with the recorded red.

## Design Insights

- The feature was built by other work after this spec was written; what remains
  is proof, and a proof that rests on an unreachable form proves nothing.

## Key Design Decisions

- Interop is the evidence for AC-8. A capability-gated wire format proven only
  against Ze proves the encoder agrees with itself (`ai/rules/interop-and-goal-validation.md`).

## Owner decisions (2026-10-11)

| Question | Owner's words | What it means here |
|----------|---------------|--------------------|
| Round-2 I5: the narrowed skeleton owns no open row; close it or rewrite it? | "Rewrite to AC-7/AC-8" | This spec keeps AC-7 and AC-8 only; the false claims about capability 77 and the seven rows are removed |

## Known Limitations

- The draft is revision -06 and not yet an RFC. A later revision can change the
  wire format, and the summary records the revision the walk read.

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
