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

Ze implements only the default, **"inherit"**, as of commit `706b77b7d` (which added
the Section 5.3 eBGP Transitive-clear at the egress funnel). There is no config
surface: no YANG leaf selects a policy today.

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
| 4 | "inherit" — already the shipped behavior; it becomes the explicit default of the new leaf |
| 5 | (re-homed 2026-07-22 from `spec-fixit-tombstone-ebgp-transitive`, closed as learned 1239) **eBGP RS-clients bypass the prepend funnel**, so Section 5.3's Transitive-clear does not reach them: `forward_rs.go` and `reactor_api_forward.go` hand out `update.WireUpdate` (the received wire) with no per-destination buffer; clearing the bit there would corrupt the shared wire for every other peer. Honoring 5.3 for RS-clients needs a third pooled slot mirroring `ebgpSlotASN4` plus release plumbing at `recent_cache.go,527` — a performance-versus-conformance decision for Thomas |
| 6 | (re-homed 2026-07-22 from the same source; RULED by Thomas 2026-07-16, edit not yet applied) **Apply the input-side LOCAL_PREF precedent to `test/plugin/remove-private-as-export.ci`**: remove the RFC-invalid LOCAL_PREF from the source frame instead of blessing the tombstone marker in the expectation (`:51` still carries `C0FC0405010000`). Byte-mechanical; the target frame shape is proven at `remove-private-as-replace-peer.ci`. The full ruling and before/after hex table are in git history of the closed spec and in `plan/deferrals/fixit-tombstone-ebgp-transitive.md` |

→ Constraint: **"strip" needs a rebuild rather than an in-place mask.** The draft
allows either a real removal or a Transitive-clear, but is explicit that they are not
equivalent: "This MAY be achieved by removing ATTR_TOMBSTONE attributes from the
forwarded path attributes (requires rebuild), or by clearing the Transitive bit
(converting to non-transitive so that non-recognizing speakers silently ignore it per
RFC 4271 Section 5). Note that clearing the Transitive bit does not remove the marker
from the wire; a recognizing peer will still see it. If complete removal is required,
the implementation MUST rebuild the path attributes." Ze's egress path masks bits in a
pooled per-destination buffer (`clearTombstoneTransitive`, `wireu/tombstone.go`,
reached from `rewriteASPathPrepend`); it does not rebuild there. So "strip" as an
operator would read the word (the marker is gone) is a new capability at that seam, not
a flag on the existing one.

## BLOCKING: Draft Ambiguities (Thomas's to resolve as the draft's author)

**These two questions are NOT implementer's judgement calls and MUST NOT be resolved by
an implementation session.** They are ambiguities in the draft text itself. Thomas is
the author of `draft-mangin-idr-attr-tombstone-00`; resolving them means deciding what
the draft should say, and the answer belongs in the draft first, then in ze. An agent
that "picks the sensible reading" and implements it is inventing protocol.

**This spec cannot leave `skeleton` until both are answered.**

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

### What remains, and it is narrower than this spec assumed

The mechanism is already implemented for every destination that passes through
the prepend funnel. **One path does not**, and it is the live gap:

eBGP RS-clients bypass the funnel entirely. `forward_rs.go` and
`reactor_api_forward.go` hand out `update.WireUpdate`, the received wire, with
no per-destination buffer, so `clearTombstoneTransitive` never runs for them and
the marker reaches an RS-client with Transitive intact. Under this ruling that
is now unambiguously wrong rather than a policy question: the mechanism is
defined and one path skips it.

That gap is the subject of `plan/deferrals/fixit-tombstone-ebgp-transitive.md`
and is recorded in `skip-blocked.md` as B-4, where a further ruling of Thomas's
is still being confirmed.

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
- Two draft ambiguities block this spec. They are the author's to settle, not an implementer's.
- Ze's Partial behavior is accidentally compliant with one reading of D-2; that is luck, not design.
- "strip" and "inherit"-for-non-transitive both need a rebuild ze does not have on the egress path.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/wireu/tombstone.go` - `clearTombstoneTransitive` masks the Transitive bit in a pooled per-destination buffer; ~~`isTombstoneCode` recognises 252 and 253~~ (superseded 2026-07-22: learned 1237 deleted `isTombstoneCode` and the dual-recognition shim; the single code point is `attribute.AttrTombstone = 252` and the egress funnel gates on it directly, `aspath_rewrite.go`); `WriteTombstone` writes the marker. This is the whole of ze's Section 5.3 implementation: mask-only, no rebuild, no policy selection
- [ ] `internal/component/bgp/message/attr_discard.go` - `attrDiscardFlags` is `0x80 | (originalFlags & 0x50)`, clearing Partial; comment at `:53` states it. Comment at `:55-59` records the architectural reason the egress rule lives in `wireu` and not here: "The marker is stamped at receive time, where the destination is not yet known ... Section 5.3's egress rule ... is enforced per destination on the EBGP wire path, in wireu.rewriteASPathPrepend, not here"
- [ ] `internal/core/bgp/attribute/attribute.go` - `FlagPartial = 0x20`, `IsPartial`; `AttrTombstone = 252`
- [ ] `internal/component/bgp/wireu/aspath_rewrite.go` - `rewriteASPathPrepend`, the single eBGP egress funnel where the mask is applied per destination

**Behavior to preserve:** (unless user explicitly said to change)
- The Section 5.3 eBGP Transitive-clear from `706b77b7d`. Whatever policy surface is added, today's behavior must remain reachable and remain the default (it is "inherit").
- Zero-copy forwarding for UPDATEs that carry no marker. A policy leaf must not put a rebuild on the general path.
- `attrDiscardFlags`'s Section 4.2 generation rule (`0x80 | (originalFlags & 0x50)`).
- The per-destination pooled-buffer model: one received wire is shared by many peers, so per-peer policy MUST NOT mutate the shared wire.

**Behavior to change:** (only if user explicitly requested)
- None until D-1 and D-2 are answered. Under D-1 reading (a), the default path gains a rebuild for non-transitive markers, which is a behavior change to the shipped default.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Per-neighbor / per-peer-group configuration selecting a forwarding policy (does not exist today — this is the surface to build).
- A received UPDATE carrying an ATTR_TOMBSTONE marker, being forwarded to a peer.

### Transformation Path
1. Config: a new YANG leaf under the neighbor / peer-group container resolves into the peer's runtime config
2. Receive: the marker is stamped in place (`message/attr_discard.go`) or arrives from upstream, in the shared received wire
3. Forward decision: per destination, the eBGP funnel `rewriteASPathPrepend` (`wireu/aspath_rewrite.go`) copies attributes into a pooled per-destination buffer
4. Policy application: `isTombstoneCode` (`tombstone.go`) identifies the marker; today `clearTombstoneTransitive` unconditionally masks Transitive (the "inherit" eBGP rule)
5. Policy branch to build: inherit → mask (today); strip → **omit the attribute entirely, requiring a rebuild of the attributes section**; propagate → set Transitive, clear Partial
6. The pooled buffer goes on the wire; the shared received wire is untouched

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ reactor | YANG tree → resolved peer config → per-destination forwarding decision | [ ] |
| Shared wire ↔ per-destination buffer | pooled buffer at `rewriteASPathPrepend`; the only place a per-peer decision may write | [ ] |
| Mask ↔ rebuild | today's egress can mask but not resize; strip needs resize (new capability at this seam) | [ ] |

### Integration Points
- `rewriteASPathPrepend` (`wireu/aspath_rewrite.go`) - the single eBGP egress funnel; the natural policy application point.
- `clearTombstoneTransitive` (`wireu/tombstone.go`) - today's inherit implementation; becomes one branch of three.
- `rebuildWithAttrDiscard` (`message/attr_discard.go`) - an EXISTING rebuild, but on the receive path, not the per-destination egress path. Study it before writing a second rebuild; do not duplicate it.

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
| A-2 | `rewriteASPathPrepend` is the only eBGP egress funnel where policy can apply | `706b77b7d` states it, gated on `facts.isEBGP && !facts.rsClient` | Policy is unenforceable on some paths | Read `received_update.go`, `forward_rs.go`, `reactor_api_forward.go` | unvalidated |
| A-3 | eBGP RS-clients bypass this funnel entirely | Recorded in `plan/spec-fixit-tombstone-ebgp-transitive.md` Known Limitations and its deferrals row: `forward_rs.go` and `reactor_api_forward.go` hand out `update.WireUpdate` with no per-destination buffer | Policy silently does not apply to RS-clients, an operator-visible hole | Read those two call sites | unvalidated |
| A-4 | "inherit" as shipped is conformant for transitive markers | `706b77b7d` implemented the Section 5.3 MUST | The default is wrong, raising priority | Re-read Section 5.3 against `clearTombstoneTransitive` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An implementer resolves D-1/D-2 by picking the reading that is easiest to code, and ze ships an interpretation the draft never made | A design doc explains what the draft "clearly means" | This spec's BLOCKING section. Do not start until Thomas answers |
| R-2 | Adding a rebuild to the egress funnel costs the zero-copy fast path for all marker-bearing UPDATEs | Benchmark regression on marker-bearing traffic | Rebuild only under strip (and under D-1 reading (a)); never on the no-marker path |
| R-3 | The policy does not reach RS-clients (A-3), so an operator sets `strip` and markers still leave the box | Config accepted, markers still on the wire to RS-clients | Either reject the config for RS-clients or resolve the RS zero-copy trade-off first. That trade-off is already deferred to `plan/spec-fixit-tombstone-ebgp-transitive.md` as Thomas's call, so the two are coupled |
| R-4 | The draft's own Section 5.3 wording is what generated the two ambiguities, so a code-only fix leaves the next implementer to rediscover them | — | Deliverable 1 may be a draft revision |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Neighbor config sets the tombstone forwarding policy to strip | → | the strip branch at the eBGP egress funnel rebuilds without the marker | `test/plugin/tombstone-policy-strip.ci` |
| Neighbor config sets the policy to propagate | → | the propagate branch sets Transitive, clears Partial | `test/plugin/tombstone-policy-propagate.ci` |
| No policy configured | → | `clearTombstoneTransitive` (today's inherit behavior) | `test/plugin/remove-private-as-export.ci` (existing) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | No policy configured | Inherit. Byte-for-byte identical to today's shipped behavior |
| AC-2 | Policy `strip`, marker-bearing UPDATE forwarded | No marker in the forwarded UPDATE at all — rebuilt, not masked (draft Section 5.3: "If complete removal is required, the implementation MUST rebuild the path attributes") |
| AC-3 | Policy `propagate`, non-transitive marker | Transitive bit SET before forwarding ("the implementation MUST set the Transitive bit before forwarding") |
| AC-4 | Policy `propagate`, marker with Partial set by an upstream non-recognizing speaker | Partial CLEARED ("MUST clear the Partial bit (setting it to 0), even if it was set by an intermediate non-recognizing speaker") |
| AC-5 | Policy configured per peer-group; a neighbor overrides it | The neighbor value wins ("The policy SHOULD be configurable per peer-group or per neighbor") |
| AC-6 | Transitive marker under inherit, Partial bit | **Blocked on D-2.** No AC can be written until Thomas rules |
| AC-7 | Non-transitive marker under inherit | **Blocked on D-1.** Either removed (rebuild) or forwarded non-transitive; the draft supports both readings |
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
| `TestForwardPolicyInheritIsDefault` | `internal/component/bgp/wireu/tombstone_forward_test.go` | AC-1: unconfigured is byte-identical to today | |
| `TestForwardPolicyStripRebuilds` | `internal/component/bgp/wireu/tombstone_forward_test.go` | AC-2: marker absent, attributes section resized | |
| `TestForwardPolicyPropagateSetsTransitive` | `internal/component/bgp/wireu/tombstone_forward_test.go` | AC-3 | |
| `TestForwardPolicyPropagateClearsPartial` | `internal/component/bgp/wireu/tombstone_forward_test.go` | AC-4 | |
| `TestForwardPolicyNoMarkerNoRebuild` | `internal/component/bgp/wireu/tombstone_forward_test.go` | AC-8: the fast path is untouched | |

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
| `tombstone-policy-inherit-default` | `test/plugin/tombstone-policy-inherit-default.ci` | Unconfigured neighbor behaves as today | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-tombstone-policy-strip` | `test/interop/scenarios/` | FRR or BIRD | A non-recognizing peer sees no marker under strip, and is undisturbed by one under propagate. The code point is provisional and unallocated (draft Section 8), so no third-party daemon recognises the attribute; that is exactly what makes the non-recognizing-peer test meaningful | |

### Future (if deferring any tests)
- AC-6 and AC-7 have no tests until D-2 and D-1 are answered. This is a blocker, not a deferral.

## Files to Modify
- `internal/component/bgp/wireu/tombstone.go` - `clearTombstoneTransitive` becomes one branch of a three-way policy
- `internal/component/bgp/wireu/aspath_rewrite.go` - `rewriteASPathPrepend`, the funnel that must consult per-destination policy
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
| 1. Read spec | This file — start at the BLOCKING Draft Ambiguities section |
| 2. Audit | Files to Modify; validate A-1..A-4 |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `./le verify current mode full` |
| 13. /ze-review gate | Review Gate section |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Resolve the draft ambiguities (BLOCKING, Thomas only)** — answer D-1 and D-2; revise the draft if the answer is that the text is unclear
   - Tests: none
   - Files: `rfc/drafts/` (Thomas edits; agents never do)
   - Verify: D-1 and D-2 have recorded answers, and AC-6/AC-7 can be written. **No code before this.**
2. **Phase: Wiring (MANDATORY FIRST)** — YANG policy leaf + resolve it into per-destination forwarding facts + failing wiring test
   - Tests: `TestForwardPolicyInheritIsDefault`
   - Files: neighbor/peer-group YANG, BGP config resolve, `aspath_rewrite.go`
   - Verify: the leaf is settable and reaches the funnel; unconfigured is byte-identical to today
3. **Phase: propagate** — the flag-only branch, no resize, so it lands first
   - Tests: `TestForwardPolicyPropagateSetsTransitive`, `TestForwardPolicyPropagateClearsPartial`
   - Files: `tombstone.go`
   - Verify: red → implement → green
4. **Phase: strip** — the rebuild branch; study `rebuildWithAttrDiscard` (`message/attr_discard.go`) first and do not duplicate it
   - Tests: `TestForwardPolicyStripRebuilds`, `TestForwardPolicyNoMarkerNoRebuild`
   - Files: `tombstone.go`, `aspath_rewrite.go`, pooled buffer plumbing
   - Verify: the no-marker fast path allocates nothing new (AC-8)
5. **Phase: inherit under the resolved D-1** — only if reading (a) forces a rebuild for non-transitive markers
   - Tests: (fill after D-1)
   - Files: (fill after D-1)
   - Verify: (fill after D-1)
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
| D-1 and D-2 answered and recorded | This spec's BLOCKING section names the ruling |
| strip really removes | `test/plugin/tombstone-policy-strip.ci` asserts the marker's bytes are absent from the wire |
| Default unchanged | `test/plugin/remove-private-as-export.ci` still green, unmodified |

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

- Ze's Partial-bit behavior is accidentally compliant with one reading of D-2: `attrDiscardFlags` clears Partial as a Section 4.2 *generation* rule, which happens to match what the propagate bullet demands of a *forwarding* recognizing speaker. Accidental compliance is not compliance; it will drift the moment someone edits the generation mask for a generation reason.
- The draft is ze's own. That makes ambiguity cheaper to fix than usual (revise the text) and more dangerous to paper over (an implementer's guess becomes, silently, the normative reading).

## Core Insight
(fill during design)

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- Related, and coupled: `plan/spec-fixit-tombstone-ebgp-transitive.md` records that eBGP RS-clients bypass the prepend funnel entirely, so Section 5.3 does not reach them (A-3, R-3). That spec holds the performance-versus-conformance ruling for RS zero-copy, which is also Thomas's. Any policy built here inherits that hole until it is resolved.
- The code point remains provisional and split across two values; `plan/spec-fixit-tombstone-code-point-split.md` owns that. `isTombstoneCode` recognising both is the current shim any policy branch must go through.

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
- [ ] D-1 and D-2 answered by Thomas before any code
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
- [ ] **Commit B:** `git rm plan/spec-tombstone-forwarding-policy.md` only
