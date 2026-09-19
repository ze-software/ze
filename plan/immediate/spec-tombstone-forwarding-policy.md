# Spec: tombstone-forwarding-policy

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-08-07 |

> **BLOCKER CLEARED 2026-08-07.** D-1 and D-2 are both answered by Thomas. The
> spec said it could not leave `skeleton` until they were, so it moves to
> `design`. Read "### Rulings (2026-08-07)" under the Open Questions before
> designing: D-2's answer is a propagation rule that does not answer the Partial
> question as posed, and it changes the shape of the work.

## Task

The configurable forwarding policy that `draft-mangin-idr-attr-tombstone-00`
Section 5.3 says implementations SHOULD provide: **inherit / strip / propagate**,
configurable per neighbor or peer-group.

Ze has no policy-selection surface and no live Section 5.3 eBGP Transitive-clear.
Commit `706b77b7d` implemented the old prepend-funnel clear; that whole-payload
rail has since been removed. The current forward and route-server rails use
`ASPathEdit.Record` plus `buildModifiedPayload`, and neither applies the clear.
This spec owns restoring the default on ordinary and RS-client eBGP egress
alongside the configurable policies. It must not preserve the missing clear as
the default's byte-level contract.

Quoting the draft (Section 5.3): "Implementations SHOULD provide a configurable policy
to override this default, with at least the following options", listing "inherit"
(default), "strip", and "propagate". And: "The policy SHOULD be configurable per
peer-group or per neighbor".

Points to complete:

| # | Point |
|---|-------|
| 1 | A YANG config surface selecting the policy, per neighbor and per peer-group |
| 2 | "strip" — needs a **rebuild**, not an in-place mask (see the constraint below) |
| 3 | "propagate" — set the Transitive bit if clear, and clear the Partial bit |
| 4 | `inherit` becomes the explicit default and implements the 2026-08-07 ruling: clear Transitive at eBGP egress and leave received Partial unchanged |
| 5 | Restore that eBGP clear on both `forwardUpdateCore` and `reactorForwardRS`, for ordinary peers and RS clients, without mutating the shared received wire. The original RS-only diagnosis understated the defect. The recorded RS performance-versus-conformance decision remains for Thomas before this design becomes ready; neither peer class is exempt from the acceptance contract |
| 6 | The inherited 2026-07-16 input-side LOCAL_PREF correction is present in `test/plugin/remove-private-as-export.ci`: the source frame omits LOCAL_PREF and the comments record the owner ruling. Preserve that corrected AS_PATH-policy fixture; it supplies no tombstone-forwarding proof |

→ Constraint: **"strip" needs a rebuild rather than an in-place mask.** The draft
allows either a real removal or a Transitive-clear, but is explicit that they are not
equivalent: "This MAY be achieved by removing ATTR_TOMBSTONE attributes from the
forwarded path attributes (requires rebuild), or by clearing the Transitive bit
(converting to non-transitive so that non-recognizing speakers silently ignore it per
RFC 4271 Section 5). Note that clearing the Transitive bit does not remove the marker
from the wire; a recognizing peer will still see it. If complete removal is required,
the implementation MUST rebuild the path attributes." The current egress rails
already have a one-pass rebuild mechanism, `buildModifiedPayload`. The design
must express marker removal through that mechanism and preserve the no-marker
fast path, rather than reviving the retired prepend funnel.

## Historical draft ambiguities, resolved 2026-08-07

D-1 and D-2 below preserve the questions put to Thomas and the draft text that
prompted them. The Rulings subsection governs the design; these questions no
longer block it. Draft revisions remain Thomas's to make, and agents must not
invent a different interpretation while implementing the recorded answers.

### D-1: "not forwarded" under inherit, which ze cannot do without a rebuild

Section 5.3's default-behavior list says, verbatim:

> "*  Non-transitive ATTR_TOMBSTONE: not forwarded."

and the inherit bullet repeats it:

> "Non-transitive ATTR_TOMBSTONE markers are silently ignored by non-recognizing
> speakers per RFC 4271 Section 5 and are not forwarded by recognizing speakers under
> the default "inherit" policy.  (The "propagate" policy overrides this; see Section
> 5.3, Paragraph 4, Item 3.)"

**The ambiguity:** "not forwarded" by a *recognizing* speaker means the marker must be
absent from the UPDATE ze sends. Ze forwards the received wire zero-copy and can only
mask bits in a pooled per-destination buffer at the eBGP funnel. Removing an attribute
is a rebuild, and ze does not rebuild on that path. So ze is non-conformant with its own
draft's default policy right now, for non-transitive markers.

Question for Thomas: does "not forwarded" mean (a) genuinely removed, forcing a rebuild
on the default path (a real performance cost on every marker-bearing UPDATE), or (b) is
it satisfied by the marker being non-transitive, since a non-recognizing peer ignores it
anyway, in which case the draft should say so and the RFC 4271 Section 5 framing carries
the weight? Note the draft already draws exactly this distinction for "strip" ("clearing
the Transitive bit does not remove the marker from the wire; a recognizing peer will
still see it"), which is evidence the two are meant to be different, and therefore that
(a) is the literal reading.

### D-2: the Partial bit is ambiguous for a recognizing speaker under inherit

Section 5.3's default-behavior list says, verbatim:

> "*  Transitive ATTR_TOMBSTONE: forwarded to peers with Partial bit set (RFC 4271
> Section 5)."

But the "propagate" bullet says, verbatim:

> "A recognizing speaker that explicitly forwards the attribute MUST clear the Partial
> bit (setting it to 0), even if it was set by an intermediate non-recognizing speaker,
> because the forwarding speaker recognizes the attribute (RFC 4271 Section 5 requires
> Partial only for unrecognized attributes)."

**The ambiguity:** these two point opposite ways for a recognizing speaker under
*inherit*. The first says forward with Partial **set**; the second gives the reason
Partial must be **clear** for a recognizing forwarder, and that reason (RFC 4271
Section 5 requires Partial only for unrecognized attributes) is not specific to
"propagate" at all. If the reasoning is sound it applies under inherit too, and the
inherit bullet is describing what a *non-recognizing* speaker does.

**Ze sets Partial nowhere.** Verified 2026-07-16: `attrDiscardFlags`
(`internal/component/bgp/message/attr_discard.go`) computes
`0x80 | (originalFlags & 0x50)`; the mask `0x50` keeps only Transitive (0x40) and
Extended Length (0x10), so Partial (0x20, `attribute.FlagPartial`, `attribute.go`)
is **cleared** at generation. Its own doc comment at `:53` says so: "Sets Optional bit,
preserves Transitive and Extended Length bits, clears Partial." No other producer in
`wireu/`, `message/`, or `attribute/` sets 0x20 on a marker.

So ze currently behaves as D-2's *second* reading, and does so by accident rather than
by decision: `attrDiscardFlags` is a Section 4.2 generation rule, not a Section 5.3
forwarding decision.

Question for Thomas: under inherit, does a recognizing speaker forward a transitive
marker with Partial set (inherit bullet) or clear (propagate bullet's reasoning)? And
is ze's accidental compliance the intended behavior?

~~→ Constraint: until D-1 and D-2 are answered, do not write code. The right first
deliverable of this spec may be a **draft revision**, not a Go change.~~

**SUPERSEDED 2026-08-07: both answered. See the rulings below.** The draft
revision half of that prediction holds; the Go change half largely does not,
because ze already implements the answer.

### Rulings (2026-08-07, Thomas)

**The mechanism, quoted:** *"it can be done by removing the transitive bit when
going to an ebgp peer"*.

That one sentence answers both ambiguities, and it does so by identifying the
lever. The Transitive bit is what governs onward propagation; the Partial bit is
not, and neither is physical removal of the attribute.

**D-1, "not forwarded", RESOLVED as reading (b).** Thomas first answered *"the
attribute is removed from the update and not passed to other peers, like
local-pref would be on an EBGP session"*, then named the mechanism. Taken
together: the INTENT is that the marker goes no further, and clearing the
Transitive bit achieves it, because a receiving speaker does not propagate an
optional non-transitive attribute (RFC 4271 Section 5). So "not forwarded"
does not require a rebuild.

This retires the cost the spec was most worried about. R-2 predicted that
reading (a) would add a rebuild to the egress funnel and cost the zero-copy fast
path for every marker-bearing UPDATE. It does not apply.

**The draft still needs the revision**, because Section 5.3 states the outcome
("not forwarded") without stating the mechanism, and the reader who implements
it literally arrives at a rebuild. Say that clearing Transitive at the eBGP
boundary is how the outcome is met.

**D-2, the Partial bit, ANSWERED BY NOT BEING THE QUESTION.** Asked whether a
recognizing speaker under inherit forwards a transitive marker with Partial set
or clear, Thomas answered with the propagation rule instead: *"We should only
forward IF it comes from an IBGP connection if it is EBGP it should never be
passed to other peers"*, then named the Transitive bit as the mechanism.

So Partial is not a lever this design pulls, and ze's current handling stands.
Verified at both producers on 2026-08-07:

| Moment | What ze does | Producer |
|--------|--------------|----------|
| Ze ORIGINATES a marker | Partial cleared: `0x80 \| (originalFlags & 0x50)` keeps Optional, Transitive and Extended Length only | `attrDiscardFlags`, `internal/component/bgp/message/attr_discard.go` |
| Ze FORWARDS a received marker | Partial untouched; only Transitive is cleared, `dst[flagsOff] &^= FlagTransitive` | `clearTombstoneTransitive`, `internal/component/bgp/wireu/tombstone.go` |

`FlagPartial` and `0x20` appear nowhere else in `wireu/`, so on the forward path
the bit arrives and leaves unchanged. The earlier note that ze complies "by
accident" is half right and should be read narrowly: the ORIGINATION clear is
deliberate and documented at its producer. The FORWARD pass-through is not a
decision anyone recorded, and it is now the ruled behaviour.

**The draft revision owed here** is to make the default-behaviour bullet say
which speaker it describes. "Transitive ATTR_TOMBSTONE: forwarded to peers with
Partial bit set (RFC 4271 Section 5)" reads as an instruction to the forwarder
and is meant as a description of how the bit came to be set by an upstream
non-recognizing speaker. That is what made it look like it contradicted the
propagate bullet.

### Remaining egress repair

The 2026-08-07 source snapshot above described a clear on the old prepend rail
and an RS-client bypass. The 2026-08-30 correction in the inherited work showed
that the old rail had no production callers. In the current tree the old
`rewriteASPathPrepend` and clear helpers are gone altogether. Ordinary eBGP and
RS-client eBGP destinations both need the ruled clear on the live rails.

The received payload remains shared. A per-destination change must be recorded
and materialised through the existing egress machinery, with unchanged peers
still able to use the original bytes. The earlier record explicitly left the RS
performance-versus-conformance ruling with Thomas. That pause is retained:
present the current live-rail buffer design for his decision before readiness,
without reviving a deleted prerequisite spec or treating the default repair as
optional.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/drafts/draft-mangin-idr-attr-tombstone-00.txt` Section 5.3 (READ ONLY — Thomas's IETF work, never edit)
4. `internal/component/bgp/wireu/tombstone.go`, `internal/component/bgp/message/attr_discard.go`

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/attributes.md` - the BGP path attribute wire format: header, flags, codes and ASN4 encoding
- [ ] `docs/architecture/route-selection.md` - names the draft at `:54`
  → Constraint: a marker does not change route selection; the route continues.
- [ ] `ai/rules/config.md` + `ai/rules/config.md` - the policy leaf is operator-facing per-neighbor config
  → Constraint: YANG, not env var; kebab-case; an `enumeration`, never a bare string.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - Section 5 optional attribute handling; the Partial bit rule both D-2 readings cite
  → Constraint: RFC 4271 Section 5 requires Partial only for **unrecognized** attributes. This is the fact D-2 turns on.
- [ ] `rfc/short/rfc7606.md` - the attribute-discard action that generates a marker
  → Constraint: marker generation is Section 4.2/5.1; forwarding is Section 5.3. Do not conflate them, which is exactly how D-2 arose.

**Key insights:** (summary of all checkpoint lines — minimal context to resume after compaction)
- D-1 and D-2 were resolved on 2026-08-07: inherit clears Transitive at eBGP egress and leaves received Partial unchanged.
- Marker generation clears Partial separately; that rule does not implement forwarding policy.
- `strip` removes the marker physically. The live one-pass rebuild is the implementation seam; every eBGP peer class needs the default repair.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/wireu/tombstone.go` - `WriteTombstone` generates code 252 with `0x80 | (origFlags & 0x50)`. It has no forwarding-policy or eBGP-clear helper.
- [ ] `internal/component/bgp/message/attr_discard.go` - receive-side marker generation; preserve its Section 4.2 flag rule.
- [ ] `internal/core/bgp/attribute/attribute.go` - `FlagPartial = 0x20`, `FlagTransitive = 0x40`, `AttrTombstone = 252`.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go`, `forward_rs.go` - both record per-destination edits and call `buildModifiedPayload`; AS_PATH intent is separate from tombstone policy and does not supply the clear.
- [ ] `internal/component/bgp/wireu/aspath_slot.go` - the live AS_PATH writer can generate an AGGREGATOR-discard marker but does not enforce tombstone propagation.

**Behavior to preserve:** (unless user explicitly said to change)
- Preserve the ruled meaning of inherit, rather than the current defect: the default clears Transitive on ordinary and RS-client eBGP egress while leaving received Partial untouched.
- Zero-copy forwarding for UPDATEs that carry no marker. A policy leaf must not put a rebuild on the general path.
- `attrDiscardFlags`'s Section 4.2 generation rule (`0x80 | (originalFlags & 0x50)`).
- The per-destination pooled-buffer model: one received wire is shared by many peers, so per-peer policy MUST NOT mutate the shared wire.

**Behavior to change:** (only if user explicitly requested)
- Add per-peer/group policy selection, implement strip and propagate, and restore inherit's eBGP clear through both live forward rails. No-marker output and unrelated attributes remain unchanged.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Per-neighbor / per-peer-group configuration selecting a forwarding policy (does not exist today — this is the surface to build).
- A received UPDATE carrying an ATTR_TOMBSTONE marker, being forwarded to a peer.

### Transformation Path
1. Config: a new YANG leaf under the neighbor / peer-group container resolves into the peer's runtime config
2. Receive: the marker is stamped in place (`message/attr_discard.go`) or arrives from upstream, in the shared received wire
3. Forward decision: `forwardUpdateCore` and `reactorForwardRS` resolve the destination policy against the payload that destination will receive, including any export replacement and generated marker.
4. Identify the marker by the single `attribute.AttrTombstone` code 252. Record the destination's edit through the existing egress machinery; no shared received byte may be changed.
5. Apply inherit's ruled eBGP Transitive-clear with received Partial unchanged; strip omits every marker from the rebuilt attribute section; propagate sets Transitive and clears Partial.
6. The pooled buffer goes on the wire; the shared received wire is untouched

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ reactor | YANG tree → resolved peer config → per-destination forwarding decision | [ ] |
| Shared wire ↔ per-destination buffer | Existing `buildModifiedPayload` materialisation on both forward rails; original bytes remain available to other peers | [ ] |
| Marker policy ↔ attribute writer | Flag changes and full suppression use the current one-pass writer; no retired whole-payload rail is restored | [ ] |

### Integration Points
- `forwardUpdateCore` (`reactor_api_forward.go`) and `reactorForwardRS` (`forward_rs.go`) are both required policy callers, for ordinary and RS-client destinations.
- `buildModifiedPayload` (`forward_build.go`) owns destination materialisation. The design must handle existing markers and markers generated by another edit, including repeated markers.
- `WriteTombstone` (`wireu/tombstone.go`) and receive-side discard generation retain their generation rules; forwarding policy must not be hidden in the receive writer.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — the policy is peer config resolved through the existing config path, not a new per-feature switch in a core struct (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Ze sets the Partial bit nowhere | `attrDiscardFlags` masks `0x50`, excluding `0x20` (`attr_discard.go`); no other 0x20 writer found in `wireu/`, `message/`, `attribute/` | D-2 already has a de-facto answer in code and the question changes shape | `grep -rn "0x20\|FlagPartial" internal/component/bgp/ internal/core/bgp/` | unvalidated |
| A-2 | The old prepend funnel supplies an implementation seam | Earlier design cited `rewriteASPathPrepend` | The design must use the live per-destination edit and materialisation paths | Source inspection of both forward rails | broken: old rail removed; current seam is `buildModifiedPayload` |
| A-3 | Only RS clients miss the clear | Original inherited RS-only diagnosis | Every ordinary eBGP destination also needs regression coverage | Source inspection of both forward rails and `tombstone.go` | broken: no live eBGP clear for either class |
| A-4 | Current unconfigured bytes already implement inherit | The old `706b77b7d` implementation was treated as still reachable | Preserving current bytes would preserve the defect | Compare the ruled flag matrix with peer-wire tests on both rails | broken at source; runtime proof remains owed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An implementer substitutes a new reading for D-1/D-2 | Default requires physical removal or changes received Partial | Use the recorded 2026-08-07 rulings and AC-6/AC-7 |
| R-2 | Marker policy adds allocations to traffic that carries no marker | No-marker benchmark regression | Record no marker operation when no marker exists or will be generated; retain AC-8 |
| R-3 | The performance concern becomes an undocumented RS-client exemption | Ordinary eBGP tests pass while RS clients retain Transitive | Resolve destination-buffer costs during design and prove both peer classes on both live rails; any proposed scope reduction requires Thomas |
| R-4 | The draft's own Section 5.3 wording is what generated the two ambiguities, so a code-only fix leaves the next implementer to rediscover them | — | Deliverable 1 may be a draft revision |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Neighbor config sets the tombstone forwarding policy to strip | → | the strip branch at the eBGP egress funnel rebuilds without the marker | `test/plugin/tombstone-policy-strip.ci` |
| Neighbor config sets the policy to propagate | → | the propagate branch sets Transitive, clears Partial | `test/plugin/tombstone-policy-propagate.ci` |
| No policy configured, ordinary or RS-client eBGP destination | → | live per-destination inherit clear on both forward rails | `test/plugin/tombstone-policy-inherit-default.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | No policy configured, on ordinary and RS-client eBGP destinations through both live forward rails | Same result as explicit inherit: clear Transitive on markers, preserve received Partial and unrelated bytes. Marker-bearing output changes where the inherited defect omitted the clear |
| AC-2 | Policy `strip`, marker-bearing UPDATE forwarded | No marker in the forwarded UPDATE at all — rebuilt, not masked (draft Section 5.3: "If complete removal is required, the implementation MUST rebuild the path attributes") |
| AC-3 | Policy `propagate`, non-transitive marker | Transitive bit SET before forwarding ("the implementation MUST set the Transitive bit before forwarding") |
| AC-4 | Policy `propagate`, marker with Partial set by an upstream non-recognizing speaker | Partial CLEARED ("MUST clear the Partial bit (setting it to 0), even if it was set by an intermediate non-recognizing speaker") |
| AC-5 | Policy configured per peer-group; a neighbor overrides it | The neighbor value wins ("The policy SHOULD be configurable per peer-group or per neighbor") |
| AC-6 | Received transitive marker under inherit, with Partial set and clear controls, to iBGP and both eBGP peer classes | Received Partial stays unchanged. Transitive clears on eBGP egress and stays unchanged on iBGP egress, per the 2026-08-07 ruling; locally generated markers retain their separate Partial-clear rule |
| AC-7 | Received non-transitive marker under inherit | It remains non-transitive without requiring physical removal or changing received Partial. Physical removal is the explicit strip policy, per D-1's resolved reading (b) |
| AC-8 | UPDATE carrying no marker, any policy | Zero-copy fast path unchanged; no rebuild, no added allocation |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator sets `strip` on a customer-facing eBGP neighbor and confirms no error-handling artifact reaches the customer | config → YANG → peer resolve → egress funnel rebuild → wire | `test/plugin/tombstone-policy-strip.ci` |
| 2 | Operator sets `propagate` to a research/measurement peer that wants the markers | config → YANG → peer resolve → egress funnel flags → wire | `test/plugin/tombstone-policy-propagate.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestForwardPolicyStripRebuilds` | `internal/component/bgp/reactor/tombstone_forward_test.go` | AC-2: every marker absent and attributes section resized on both live rails | |
| `TestForwardPolicyInheritIsDefault` | `internal/component/bgp/reactor/tombstone_forward_test.go` | AC-1: omitted policy equals explicit inherit, including the repaired ordinary and RS-client eBGP clear on both rails | |
| `TestForwardPolicyPropagateSetsTransitive` | `internal/component/bgp/wireu/tombstone_forward_test.go` | AC-3 | |
| `TestForwardPolicyPropagateClearsPartial` | `internal/component/bgp/wireu/tombstone_forward_test.go` | AC-4 | |
| `TestForwardPolicyNoMarkerNoRebuild` | `internal/component/bgp/wireu/tombstone_forward_test.go` | AC-8: the fast path is untouched | |
| `TestForwardPolicyInheritPreservesPartial` | `internal/component/bgp/reactor/tombstone_forward_test.go` | AC-6/AC-7: both received Partial values and both Transitive values across iBGP, ordinary eBGP and RS-client eBGP, without shared-wire mutation | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Policy leaf | enumeration (inherit, strip, propagate) | N/A — enum, not numeric | N/A | N/A |
| Rebuilt attributes section length | 0-65535 (RFC 4271 UPDATE bound) | 65535 | N/A | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `tombstone-policy-strip` | `test/plugin/tombstone-policy-strip.ci` | Operator strips markers toward a customer peer | |
| `tombstone-policy-propagate` | `test/plugin/tombstone-policy-propagate.ci` | Operator propagates markers toward a measurement peer | |
| `tombstone-policy-inherit-default` | `test/plugin/tombstone-policy-inherit-default.ci` | Omitted policy equals explicit inherit; ordinary and RS-client eBGP clear Transitive, while iBGP and received Partial controls stay unchanged | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-tombstone-policy-strip` | `test/interop/scenarios/` | FRR or BIRD | A non-recognizing peer sees no marker under strip, and is undisturbed by one under propagate. The code point is provisional and unallocated (draft Section 8), so no third-party daemon recognises the attribute; that is exactly what makes the non-recognizing-peer test meaningful | |

### Required direction coverage
- AC-6 and AC-7 follow the recorded rulings and require the same live-rail proof as AC-1. All policies must cover ordinary and RS-client eBGP destinations; no retired-helper test substitutes for peer-wire assertions.

## Files to Modify
- `internal/component/bgp/reactor/reactor_api_forward.go`, `forward_rs.go` - record policy on the live destination paths, including RS clients
- `internal/component/bgp/reactor/forward_build.go` and its registered attribute handlers - apply marker flag edits and suppression through the existing one-pass rebuild
- BGP peer config resolution - carry the policy into the per-destination forwarding facts (alongside `isEBGP` / `rsClient`)
- The BGP neighbor / peer-group YANG - the new policy leaf

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] | BGP neighbor + peer-group containers. Read `ai/rules/config.md` and `ai/rules/config.md` |
| YANG validation constraints | [ ] | `enumeration` (inherit/strip/propagate). A bare `type string` is a red flag |
| Editor autocomplete | [ ] | Automatic for a YANG enum leaf |
| Functional test for new RPC/API | [ ] | `test/plugin/tombstone-policy-*.ci` |
| Prometheus counters/metrics | [ ] | Consider a stripped/propagated marker counter: policy that silently does nothing (R-3) is otherwise invisible |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` — strip changes the attributes section length |
| 9 | RFC behavior implemented, changed, or newly proven? | [ ] | `docs/features/rfc-status.md` if it carries a draft row |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/route-selection.md` |

## Files to Create
- `test/plugin/tombstone-policy-strip.ci`
- `test/plugin/tombstone-policy-propagate.ci`
- `test/plugin/tombstone-policy-inherit-default.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file, starting with the 2026-08-07 rulings and the current egress repair scope |
| 2. Audit | Files to Modify; validate A-1..A-4 |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `./le verify current mode full` |
| 13. /ze-review gate | Review Gate section |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Design against the recorded rulings.** D-1 and D-2 are answered. Present the live-rail destination-buffer design and recorded RS performance concern to Thomas before readiness; do not drop the ordinary or RS-client default repair. Draft wording revisions remain Thomas's.
2. **Phase: Wiring (MANDATORY FIRST).** Add the per-neighbour/group policy and drive it into both live forward rails. `TestForwardPolicyInheritIsDefault` must fail against the current missing clear.
3. **Phase: Inherit repair.** Implement the default clear for ordinary and RS-client eBGP on both rails, preserve received Partial and iBGP controls, and prove AC-1, AC-6 and AC-7 without shared-wire mutation.
4. **Phase: Propagate.** Set Transitive and clear Partial in the destination's emitted marker, including markers generated during egress edits.
5. **Phase: Strip.** Omit every marker through the existing one-pass rebuild. Prove AC-2 and the no-marker no-allocation contract AC-8; do not restore `aspath_rewrite.go`.
6. **Functional tests** → the three `.ci`
7. **RFC refs** → `// draft-mangin-idr-attr-tombstone-00 Section 5.3: "<quoted requirement>"` above each branch
8. **Full verification** → `./le verify current mode full`
9. **Complete spec** → learned summary, two commits

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | strip REBUILDS (AC-2); a Transitive-mask masquerading as strip is the exact conflation the draft warns against |
| Data flow | The shared received wire is never mutated by a per-peer policy decision |
| Performance | No rebuild and no new allocation when the UPDATE carries no marker (AC-8) |
| Naming | YANG kebab-case; the enum values match the draft's words exactly (inherit/strip/propagate) |
| YANG validation | `enumeration`, not a bare string |
| Registration over hardcoding | Policy rides the existing peer-config resolution; no new per-feature field bolted onto a core reactor struct (`ai/rules/plugins.md`) |
| Rule: no-fabrication | Every Section 5.3 claim in the code comments quotes the draft verbatim |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| D-1 and D-2 honoured | The recorded rulings match AC-6/AC-7 and their live-rail assertions |
| strip really removes | `test/plugin/tombstone-policy-strip.ci` asserts the marker's bytes are absent from the wire |
| Default repaired without unrelated byte changes | `tombstone-policy-inherit-default.ci` proves the eBGP clear and iBGP/Partial controls; preserve the already-corrected `remove-private-as-export.ci` input fixture |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Under strip, the rebuild parses attacker-controlled attribute lengths to resize; bound it as `rebuildWithAttrDiscard` does |
| Resource exhaustion | A peer that sends every UPDATE with a marker forces a rebuild per UPDATE per destination under strip. Confirm that is bounded and that the operator chose it |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Draft says two things | STOP. This is D-1/D-2 territory. Ask Thomas. Do not pick a reading |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- Marker generation and forwarding have separate flag contracts. The 2026-08-07 ruling preserves received Partial under inherit; propagate explicitly clears it. A generation mask cannot prove either forwarding outcome.
- The draft is ze's own. That makes ambiguity cheaper to fix than usual (revise the text) and more dangerous to paper over (an implementer's guess becomes, silently, the normative reading).

## Core Insight
(fill during design)

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- The inherited eBGP-clear repair belongs here for ordinary and RS-client destinations. The retired `spec-fixit-tombstone-ebgp-transitive` is provenance, not a live dependency or an exemption.
- The code point remains provisional, but the split is resolved: current code uses `attribute.AttrTombstone = 252`, with no dual-recognition shim.

## RFC Documentation

Add `// draft-mangin-idr-attr-tombstone-00 Section 5.3: "<quoted requirement>"` above each policy branch.
MUST document: the inherit MUST-clear-Transitive rule, the strip MUST-rebuild-for-complete-removal rule, the propagate MUST-set-Transitive and MUST-clear-Partial rules.

## Implementation Summary

### What Was Implemented
- (fill during implementation)

### Bugs Found/Fixed
- (fill during implementation)

### Documentation Updates
- (fill during implementation)

### Deviations from Plan
- (fill during implementation)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Section 5.3's SHOULD is satisfied: a configurable per-neighbor policy exists | functional test | (fill during implementation) |
| strip genuinely removes the marker | interop test against a non-recognizing daemon | (fill during implementation) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | (fill during implementation) | file:line | (fill during implementation) |

### Fixes applied
- (fill during implementation)

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] The 2026-08-07 D-1/D-2 rulings are reflected in the implementation and proof
- [ ] AC-1..AC-8 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `./le verify worktree` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/immediate/spec-tombstone-forwarding-policy.md` only

## Work Inherited From a Deferral Row

<!-- The deferral directory was deleted on 2026-09-05. A row that named this spec as
     its destination is reproduced here, so the item and the reasoning behind it
     survive the directory. Each row is outstanding work this spec owns. -->

### From `fixit-tombstone-ebgp-transitive.md`, 2026-07-16

Historical diagnosis, superseded by the current Task and producer notes above.
The eBGP repair remains owned here; the input-fixture correction is present.
The old helper names describe the 2026-08-30 snapshot rather than current entry points.

Deferred by spec-fixit-tombstone-ebgp-transitive (Known Limitations).

eBGP RS-clients bypass the prepend funnel, so draft Section 5.3's Transitive-clear does not reach them: `forward_rs.go` and `reactor_api_forward.go` hand out `update.WireUpdate`, the received wire, with no per-destination buffer. Clearing the bit there would corrupt the shared wire for every other peer. **CORRECTED 2026-08-30 at the producer: this row UNDERSTATES it. No eBGP peer gets the clear today, RS-client or not.** `clearTombstoneTransitiveInBody` (`internal/component/bgp/wireu/tombstone.go`) is called only from inside `rewriteASPathPrepend` (`aspath_rewrite.go`), and that rail is dead: its entry points `RewriteASPath` and `RewriteASPathDual` have no non-test callers left, because the egress rails moved to the one-pass writer in `internal/component/bgp/wireu/aspath_slot.go`, which carries no tombstone handling. `TestTombstoneCodePointIsUnified` stays green by calling the dead rail directly. The comment in `test/plugin/prefixsid-ebgp-discard-single-walk.ci` that says Section 5.3's clear "is applied per destination on the EBGP egress path (wireu.rewriteASPathPrepend)" is therefore false and needs correcting when this row is taken

### From `fixit-tombstone-ebgp-transitive.md`, 2026-07-16

Historical policy-scope record. D-1 and D-2 were resolved on 2026-08-07; the
current tree no longer implements the old default clear.

Deferred by spec-fixit-tombstone-ebgp-transitive (draft Section 5.3 SHOULD).

The configurable forwarding policy the draft says implementations SHOULD provide (inherit / strip / propagate, per neighbor or peer-group). Ze now implements only the default, "inherit". Two draft ambiguities ride along: 5.3 says a non-transitive marker is "not forwarded" under inherit, which ze cannot do without a rebuild; and the Partial bit is ambiguous for a recognizing speaker under inherit (5.3's inherit bullet says transitive markers are forwarded with Partial set, its propagate bullet says a recognizing forwarder MUST clear it), and ze sets Partial nowhere
