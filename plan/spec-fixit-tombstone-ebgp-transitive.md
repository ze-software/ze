# Spec: fixit-tombstone-ebgp-transitive

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/drafts/draft-mangin-idr-attr-tombstone-00.txt` Sections 4.2, 5.1, 5.2, 5.3, 5.5, 5.7
4. `internal/component/bgp/wireu/aspath_rewrite.go`, `internal/component/bgp/wireu/tombstone.go`, `internal/component/bgp/message/attr_discard.go`

## Task

Honour the ATTR_TOMBSTONE forwarding-policy MUST at the EBGP boundary, and repair
nine dead normative citations in `internal/component/bgp/message/attr_discard.go`
that name a draft (`draft-mangin-idr-attr-discard-00`) which no longer exists,
one of them citing a `Section 5.10` that never existed.

The rule (draft-mangin-idr-attr-tombstone-00 Section 5.3, "inherit", the default
policy), quoted verbatim:

> At the originating AS's EBGP boundary, the sending speaker controls propagation.
> Under the "inherit" policy, a recognizing EBGP speaker MUST clear the Transitive
> bit before forwarding the marker to the EBGP peer.  This prevents the peer from
> propagating the marker further.

### Inherited work item (from the F4 shard, 2026-07-16): reconcile how tests treat RFC-invalid LOCAL_PREF

**A `.ci` expectation in HEAD will break when this spec lands, and it is the same
decision this spec is making.** Three F4 tests fed LOCAL_PREF on an EBGP session --
which RFC 4271 Section 5.1.5 forbids ("MUST NOT ... included in UPDATE messages sent
to external peers") -- and ze correctly discards it per RFC 7606 Section 7.5
(`message/rfc7606.go:442-449`, `validateLocalPrefAttr`: `!isIBGP` -> `AttributeDiscard`
/ `DiscardReasonEBGPInvalid`), stamping the marker in place (`attr_discard.go:96-115`).
Two agents fixing sibling tests in parallel reached OPPOSITE conclusions:

| Test | Approach | State |
|------|----------|-------|
| `bgp-rs-fastpath-ebgp-shared.ci` | removed the invalid LOCAL_PREF from the INPUT frame | passes |
| `remove-private-as-replace-peer.ci` | removed it from the INPUT frame | passes |
| `remove-private-as-export.ci:49` | KEPT the invalid input and expects the marker on the wire: `C0FD0405010000` | passes |

-> Constraint: **that third expectation is coupled to this spec's outcome.**
`attrDiscardFlags` (`attr_discard.go`, re-read 2026-07-16) computes
`0x80 | (original_flags & 0x50)`; LOCAL_PREF is well-known transitive (`0x40`), so the
marker is stamped `0xC0` at RECEIVE time, which is what that test asserts. Its own doc
comment says Section 5.3's egress rule "is enforced per destination on the EBGP wire
path, in `wireu.rewriteASPathPrepend`, not here" -- i.e. exactly what this spec adds.
Once the Transitive bit is cleared for an EBGP destination the marker becomes `0x80`,
and `remove-private-as-export.ci:49` goes RED.

-> Decision needed (do not defer past this spec's closure): either (a) update that
expectation to the post-fix flags, or (b) adopt the input-side precedent its two
siblings already use and remove the invalid LOCAL_PREF from its source frame. (b) is
preferred: the test's subject is AS_PATH rewriting, and carrying an orthogonal RFC-7606
concern in it is what coupled it to this spec in the first place. Either way the repo
must stop handling the same invalid input two contradictory ways.

-> **RULED 2026-07-16 (Thomas): take (b)** -- remove the invalid LOCAL_PREF from the
source frame -- **as the interim answer "until we re-engineer how we deal with
attributes"**. So (b) is adopted here on the test-hygiene argument, NOT as a settled
verdict on attribute handling: the re-engineering may revisit where discard markers
belong, and this test must not be the thing that pins that decision. Recording the
caveat so the next reader does not mistake a scoped test fix for an architectural
ruling.

-> The change is byte-mechanical and its shape is already proven by the sibling that
took the same route. `remove-private-as-replace-peer.ci:39` carries the exact frame
(b) produces here -- same AS_PATH `[64496 64512 64497]`, same NEXT_HOP, no LOCAL_PREF,
`length=0x0037`, `attr-len=0x001C`:

| Line | From | To |
|------|------|-----|
| `remove-private-as-export.ci:21` (input) | `...003E020000002340010100 40020E02030000FBF00000FC000000FBF1 40030401010101 40050400000064 180A0000` | `...0037020000001C40010100 40020E02030000FBF00000FC000000FBF1 40030401010101 180A0000` |
| `remove-private-as-export.ci:49` (expect) | `...003E020000002340010100 40020E02030000FDE80000FBF00000FBF1 40030401010101 C0FD0405010000 180A0000` | `...0037020000001C40010100 40020E02030000FDE80000FBF00000FBF1 40030401010101 180A0000` |

Dropping the 7-byte LOCAL_PREF takes `attr-len` 0x23 (35) -> 0x1C (28) and message
length 0x3E (62) -> 0x37 (55) on both frames. With no invalid LOCAL_PREF arriving,
no ATTR_DISCARD marker is produced at all, so the expectation stops depending on this
spec's outcome in either direction -- which is the point of (b).

-> Constraint honoured: the load-bearing AS_PATH assertion `[65000 64496 64497]`
(`0000FDE8 0000FBF0 0000FBF1`) is byte-identical before and after. (b) removes an
orthogonal attribute from the frame; it does not weaken the assertion this test exists
to make. The comment block at `:36-46` must be rewritten to the siblings' "NO
LOCAL_PREF: this test's UPDATE is sourced from an EBGP peer..." wording
(`remove-private-as-replace-peer.ci:10-20`) rather than left describing a marker the
frame no longer produces.

-> Constraint: this is NOT licence to weaken the test. `remove-private-as-export.ci`'s
AS_PATH assertion `[65000 64496 64497]` is load-bearing -- it was unsatisfiable until
`afb068cc0` fixed the double-filter bug and must stay byte-exact.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/memory-architecture.md` - buffer ownership on the forward path
  → Constraint: "Forwarding (same context) | Same pool buffer | Zero-copy: ContextID match means wire bytes are reusable". The received wire is shared by every destination, so it MUST NOT be mutated per destination.
  → Decision: copies are legal only at the listed boundaries; "ContextID mismatch on forward" is one of them, and the EBGP prepend is exactly that boundary.
- [ ] `ai/rules/buffer-first.md` - no `make`/`append` in encoding code
  → Constraint: the fix must write into a buffer that already exists. It does: it masks one byte already copied into the per-destination EBGP buffer.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/drafts/draft-mangin-idr-attr-tombstone-00.txt` Section 4.2 - Transitivity Derivation
  → Constraint: `new_flags = 0x80 | (original_flags & 0x50)` at GENERATION. Optional bit MUST be 1; Partial MUST be 0 at generation; Extended Length MUST match the length encoding. So the egress clear must touch ONLY 0x40.
- [ ] Section 5.1 - Generating (Sender)
  → Constraint: an implementation MAY clear Transitive at GENERATION (`0x80 | (original_flags & 0x10)`), and "The choice between preserving and clearing the Transitive bit SHOULD be configurable, with the default being to preserve." Ze preserves. That is the default and is correct.
  → Constraint: "an implementation MUST NOT set the Transitive bit if it was not set in all discarded attributes" (generation only).
- [ ] Section 5.3 - Forwarding Policy
  → Decision: three policies. "inherit" (default), "strip", "propagate". Only "inherit" is implemented here; the configurable policy the draft says implementations SHOULD provide is deliberately NOT built (see Known Limitations).
  → Constraint: under "inherit", transitive markers travel within the AS via IBGP, and MUST have the Transitive bit cleared at the EBGP boundary.
- [ ] Section 5.5 - Interaction with Confederation Boundaries
  → Constraint: "At confederation boundaries [RFC5065], ATTR_TOMBSTONE markers are handled according to their Transitive bit, per standard RFC 5065 processing. The forwarding policy (Section 5.3) MAY be applied." A confederation member-AS boundary is therefore NOT an AS boundary for the Section 5.3 MUST. Clearing there would be a new bug.
- [ ] Section 5.7 - Multiple Discards
  → Constraint: all transitive → 0xC0; all non-transitive → 0x80; mixed → MUST 0x80. This is the rule the dead `Section 5.10` citations were reaching for.

**Key insights:**
- The Section 5.3 rule is an EGRESS property (per destination). The Section 4.2 derivation is a GENERATION property (per received UPDATE). Ze conflated them by stamping once at receive.
- Ze does not implement BGP confederations as a session type: `grep -rl confederation internal/` finds AS_PATH segment handling (`ASConfedSequence`/`ASConfedSet`) and ASPA verification only, no confederation peer/member-AS session. So Section 5.5 imposes no code here: there is no confed egress path that could wrongly clear.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/message/attr_discard.go` - `attrDiscardFlags` (:46) returns `0x80 | (originalFlags & 0x50)`, mirroring the discarded attribute's Transitive bit. `applyInPlace` (:96-115) writes it into `pathAttrs`, a slice of the RECEIVED body.
  → Constraint: this is correct per Section 4.2 for generation. It is NOT the bug; it must not be changed.
- [ ] `internal/component/bgp/reactor/session_validation.go` - `enforceRFC7606` (:26) computes `pathAttrs := body[offset : offset+attrLen]` (:59) and calls `message.ApplyAttrDiscard(pathAttrs, ...)` (:117) at RECEIVE, before callback dispatch. Destination is unknown here.
  → Constraint: the stamp lands in the shared received wire. Any per-destination change here is impossible without breaking zero-copy.
- [ ] `internal/component/bgp/wireu/aspath_rewrite.go` - `rewriteASPathPrepend` (:63) is the EBGP re-encode. It copies every attribute from `payload` into `dst` (loop :472-532), with a `default:` case that copies verbatim (:527-528). ATTR_TOMBSTONE fell into `default` and was forwarded byte-identical, Transitive bit intact.
  → Decision: this loop is the seam. The copy already happens; clearing costs one byte-mask.
- [ ] `internal/component/bgp/wireu/tombstone.go` - `WriteTombstone` (:36) writes the marker with `attribute.AttrTombstone` (252) and cites the CORRECT draft.
  → Constraint: ze has TWO code points for one attribute (252 here, 253 in `message/attr_discard.go`). Both must be recognized on the egress path until unified.
- [ ] `internal/component/bgp/reactor/received_update.go` - `EBGPWire` (:115) calls `wireu.RewriteASPath(dst.Buf, ...)` (:138) into a pooled buffer (`getReadBuf`), caches per `dstASN4`, and shares `SourceCtxID`.
  → Decision: the EBGP wire is already a separate, per-destination-class, pooled buffer. Clearing inside the rewrite is free and cannot leak to IBGP peers.
- [ ] `internal/component/bgp/reactor/forward_rs.go` - `getEBGPWire` (:141) and the peer loop (:351-359): `peerWire := update.WireUpdate`, replaced by the rewritten wire only when `facts.isEBGP && !facts.rsClient`.
  → Constraint: an EBGP RS-client with matching ASN4 gets the RECEIVED wire, shared, with no re-encode at all. See Known Limitations.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - same structure (:527-535).
- [ ] `internal/component/bgp/wireu/aspath_transcode.go` - `TranscodeASPath` (:35).
  → Constraint: called from `forward_body.go:145` which is NOT EBGP-gated (it transcodes per destination encoding context). The clear MUST NOT go inside TranscodeASPath.

**Verified chain (the bug, end to end):**
1. LOCAL_PREF (code 5, flags 0x40) arrives on an EBGP session. `message.ValidateUpdateRFC7606` returns attribute-discard (RFC 7606 Section 7.5).
2. `enforceRFC7606` (`session_validation.go:117`) calls `ApplyAttrDiscard` → `applyInPlace` (`attr_discard.go:96`).
3. `attrDiscardFlags(0x40)` = `0x80 | (0x40 & 0x50)` = **0xC0**. Marker stamped in place: `C0 FD 04 05 01 00 00`. This matches draft Section 4.3 Example 3 exactly.
4. Forward to an EBGP peer: `forward_rs.go:353` → `EBGPWire` → `rewriteASPathPrepend`, whose `default:` case copied the 0xC0 marker verbatim into the EBGP wire.
5. The EBGP peer receives an optional-TRANSITIVE marker and propagates it further per RFC 4271 Section 5. Section 5.3's MUST is violated.

**Behavior to preserve:**
- The receive-time stamp and its Section 4.2 derivation (0xC0 for a transitive original). IBGP peers must keep seeing the transitive marker.
- The marker's length field and value bytes on the EBGP wire (Section 5.1 step 7: "The length field MUST NOT be modified").
- Zero-copy forwarding for every UPDATE that carries no marker.

**Behavior to change:**
- On the EBGP prepend wire only, the marker's Transitive bit is cleared.

## Data Flow (MANDATORY)

### Entry Point
Wire bytes: an UPDATE received on an EBGP session carrying an attribute that RFC 7606 says to discard, or already carrying an upstream ATTR_TOMBSTONE.

### Transformation Path
1. `Session.processMessage` → `enforceRFC7606` (`session_validation.go:26`) — stamps the marker in the received wire (Section 4.2 derivation, destination unknown).
2. `ReceivedUpdate` holds that wire zero-copy; IBGP peers forward it unchanged (marker stays 0xC0 — Section 5.3 "forwarded within the AS via IBGP").
3. EBGP peers: `forward_rs.go:353` / `reactor_api_forward.go:528` → `EBGPWire`/`getEBGPWire` → `wireu.RewriteASPath(dst.Buf, ...)` into a POOLED PER-DESTINATION buffer.
4. `rewriteASPathPrepend` attribute copy loop — **the Section 5.3 clear happens here**, on `dst`, after the verbatim copy.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Received wire ↔ EBGP wire | `wireu.RewriteASPath` into `getReadBuf` pool buffer | [x] `received_update.go:131-148` |
| Received wire ↔ IBGP wire | none: same buffer, zero-copy | [x] `forward_rs.go:351` |

### Integration Points
- `rewriteASPathPrepend` (`aspath_rewrite.go:63`) — the single funnel for `RewriteASPath` and `RewriteASPathDual`, i.e. all four EBGP re-encode call sites.

### Architectural Verification
- [x] No bypassed layers — the clear sits on the existing egress re-encode, not a new pass.
- [x] No unintended coupling — `wireu` already owns the tombstone wire format (`tombstone.go`).
- [x] No duplicated functionality — reuses the existing copy loop.
- [x] Zero-copy preserved — the received wire is never mutated; no new buffer is taken.
- [x] Registration over hardcoding — N/A (no new command/family/handler).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `rewriteASPathPrepend` is only ever used to build wire for a true EBGP peer | It prepends the local ASN (RFC 4271 9.1.2). All callers: `received_update.go:138`, `forward_rs.go:197,199`, `reactor_api_forward.go:293,295,365,367` — every one gated on `facts.isEBGP && !facts.rsClient` | An IBGP peer would wrongly lose the transitive marker | `grep -rn "RewriteASPath" --include=*.go internal/` + reading each call site | confirmed |
| A-2 | Ze has no confederation member-AS session type, so Section 5.5 needs no code | `grep -rl confederation internal/` returns only AS_PATH segment + ASPA files; no peer/session confed concept | A confed boundary would wrongly clear | grep + `forward_rs.go` peer facts have `isEBGP`/`rsClient`/`rrClient` only, no confed | confirmed |
| A-3 | Clearing on the EBGP wire cannot affect IBGP peers | `EBGPWire` writes into `getReadBuf` pool buffer, distinct from `u.WireUpdate` | IBGP peers would lose the marker | `TestRewriteASPath_ClearsTombstoneTransitiveAtEBGPBoundary` asserts the source `marker[0]` is still 0xC0 after the rewrite | confirmed |
| A-4 | Both 252 and 253 must be recognized as ATTR_TOMBSTONE on egress | `attribute.AttrTombstone = 252` (`attribute.go:66`) vs `attrCodeAttrDiscard = 253` (`attr_discard.go`) | The real, receive-stamped marker (253) would escape unfixed | test covers both codes | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The 252/253 split means an operator sees two different attribute codes for one concept | BMP/dump shows both | Reported as a separate finding; not unified here (blast radius includes wire bytes baked into tests) |
| R-2 | EBGP RS-clients (transparent, no re-encode) still forward the transitive marker | An RS-client peer propagates a marker | Not fixed here: requires a per-update pooled copy where today there is literal zero-copy. Needs Thomas's decision (see Known Limitations) |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| EBGP UPDATE carrying a transitive ATTR_TOMBSTONE, forwarded to an EBGP peer | → | `rewriteASPathPrepend` default case → `clearTombstoneTransitive` | `TestRewriteASPath_ClearsTombstoneTransitiveAtEBGPBoundary` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Transitive marker (0xC0), code 252, forwarded to an EBGP peer | Marker flags become 0x80 on the EBGP wire (Section 5.3) |
| AC-2 | ~~Transitive marker (0xC0), code 253 (ze's receive-time spelling), forwarded to an EBGP peer → 0x80~~ **SUPERSEDED 2026-07-21** by `spec-fixit-tombstone-code-point-split` (learned 1237): code 253 is no longer a tombstone spelling; the wire code point is unified to 252, so AC-1 is now the sole tombstone-transitive-clear case. A 253 attribute is a generic optional-transitive attribute forwarded verbatim (its Transitive bit is correctly left untouched — `tombstone_forward_test.go`). Retired, not a regression. |
| AC-3 | Same UPDATE forwarded to an IBGP peer | Received wire untouched; marker stays 0xC0 (Section 5.3 "forwarded within the AS via IBGP") |
| AC-4 | Marker already non-transitive (0x80) | Forwarded unchanged; no other flag bit altered |
| AC-5 | Extended-length marker (0xD0, 2-byte length) | Becomes 0x90: Transitive cleared, Optional and Extended Length preserved (Section 4.2) |
| AC-6 | Marker value/length on the EBGP wire | Length field and (code, reason) pairs byte-identical (Section 5.1 step 7) |
| AC-7 | UPDATE with no marker | Wire byte-identical to today; no added allocation |
| AC-8 | Every citation in `attr_discard.go` | Names `draft-mangin-idr-attr-tombstone-00` and a section that exists (5.1..5.7) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRewriteASPath_ClearsTombstoneTransitiveAtEBGPBoundary` | `internal/component/bgp/wireu/tombstone_forward_test.go` | AC-1, AC-2, AC-3, AC-6 | RED observed, now PASS |
| `TestRewriteASPath_NonTransitiveTombstoneUnchanged` | same | AC-4 | PASS |
| `TestRewriteASPath_ClearsTombstoneTransitiveExtendedLength` | same | AC-5 | RED observed, now PASS |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| marker flags Transitive bit | 0x40 set/clear | 0x40 → cleared, 0x00 → no-op | N/A | N/A |
| marker value length | 0..65535 | 256 (extended length, `TestRewriteASPath_ClearsTombstoneTransitiveExtendedLength`) | N/A (length field untouched by the clear) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (none added) | - | See Known Limitations: `bgp-rs-fastpath-ebgp-shared.ci` is out of scope for this change and must not be touched | N/A |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (none) | - | - | ATTR_TOMBSTONE uses a provisional code point with no IANA allocation; no other daemon implements the draft, so there is nothing to interop against | N/A justified |

## Files to Modify
- `internal/component/bgp/wireu/tombstone.go` - `attrTombstoneLegacy`, `isTombstoneCode`, `clearTombstoneTransitive` (the Section 5.3 enforcement, with the quoted MUST)
- `internal/component/bgp/wireu/aspath_rewrite.go` - the `default:` case of the attribute copy loop calls the clear
- `internal/component/bgp/message/attr_discard.go` - nine dead citations corrected; note added that Section 5.3 is enforced on egress
- `internal/component/bgp/message/rfc7606.go` - one dead citation
- `internal/component/bgp/message/attr_discard_test.go` - three dead citations (two of them `Section 5.10`)
- `internal/component/bgp/reactor/session_validation.go` - three dead citations; note added about receive vs egress
- `docs/architecture/route-selection.md` - one dead citation

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 7 | Wire format changed? | Yes | `docs/architecture/wire/attributes.md` — ATTR_TOMBSTONE egress flag handling |
| 9 | RFC behavior implemented/changed? | Yes | `docs/features/rfc-status.md` if it carries a tombstone row |
| 12 | Internal architecture changed? | No | The seam is an existing function; `docs/architecture/route-selection.md` citation fixed |
| others | - | No | No config/CLI/API/plugin surface touched |

## Files to Create
- `internal/component/bgp/wireu/tombstone_forward_test.go` - the boundary behavior tests

## Implementation Steps

### Implementation Phases
1. **Phase: Test first** — write `tombstone_forward_test.go` with flags derived from the draft (0xC0 in, 0x80 out; 0xD0 in, 0x90 out) BEFORE running ze. Observe RED.
2. **Phase: Enforce** — `clearTombstoneTransitive` + `isTombstoneCode` in `tombstone.go`; call from the `rewriteASPathPrepend` copy loop. Observe GREEN.
3. **Phase: Mutation** — flip the mask to `FlagPartial`, confirm RED, restore.
4. **Phase: Citations** — repair all dead draft references.

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| `bgp-rs-fastpath-ebgp-shared.ci` is currently red because of a 0xC0 marker | The file's current revision carries NO LOCAL_PREF and so produces NO marker; the marker episode is recorded in its header comment as history (lines 15-26) and the input was fixed instead | Read the .ci file | The test is unaffected by this change; no re-derivation needed unless LOCAL_PREF is reintroduced |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Clear the Transitive bit at the receive-time stamp (`attrDiscardFlags`) | The destination is unknown at receive, and the stamped byte is in the wire shared zero-copy with IBGP peers. One byte cannot be transitive to IBGP and non-transitive to EBGP at once. Also violates Section 4.2's generation derivation | Clear per destination on the EBGP re-encode (`rewriteASPathPrepend`) |
| Clear inside `TranscodeASPath` | Not EBGP-gated: `forward_body.go:145` transcodes per encoding context, including for IBGP peers | Clear only in the prepend path, which is definitionally EBGP |

## Design Insights
- The draft's Section 4.2 (generation) and Section 5.3 (forwarding) are different lifecycle stages, and ze's zero-copy design makes the distinction load-bearing: the stamp is per-UPDATE, the policy is per-destination. The EBGP prepend is the only point where ze already pays for a per-destination wire, so it is the only place the egress rule is free.

## Core Insight
A wire-level marker whose meaning depends on the destination cannot be finalized at receive time. Ze's `EncodingContext`/`ContextID` machinery already encodes this truth: "same context = forward unchanged". Section 5.3 is, in ze's terms, a statement that EBGP-ness is part of the encoding context.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Clear in `rewriteASPathPrepend`'s copy loop | (a) receive-time stamp; (b) a separate post-pass over the EBGP wire; (c) inside `TranscodeASPath` | (a) is wrong (destination unknown, shared buffer); (b) costs an extra scan of every EBGP UPDATE; (c) is not EBGP-gated. The copy loop already touches every attribute in a per-destination buffer: the clear is one byte-mask, zero added allocation, zero added traversal |
| Recognize both 252 and 253 | Only 252 (the "canonical" constant) | The real receive-time marker is 253; recognizing only 252 would leave the actual bug unfixed |
| Do not implement the configurable "inherit"/"strip"/"propagate" policy | Build it now | Section 5.3 says implementations SHOULD provide it, and it needs a YANG surface, per-neighbor/peer-group resolution, and its own tests. That is a separate spec. "inherit" is the draft's default and is what ze now implements |
| Fix `remove-private-as-export.ci`'s coupling on the INPUT side: remove the RFC-invalid LOCAL_PREF from its source frame (option (b)) | (a) keep the invalid input and re-bless the expectation with the post-fix `0x80` marker flags | Ruled by Thomas 2026-07-16, explicitly as the interim answer "until we re-engineer how we deal with attributes". (b) decouples the test from this spec's outcome entirely: with no invalid LOCAL_PREF arriving, no marker is generated, so the expectation stops tracking marker flags in either direction. It also stops the repo handling one invalid input two contradictory ways -- the other two siblings already took (b). (a) would have kept an orthogonal RFC-7606 concern inside a test whose subject is AS_PATH rewriting, i.e. preserved the coupling that raised the question. The frame (b) produces is byte-identical to the proven `remove-private-as-replace-peer.ci:39` |

## Known Limitations
- **The configurable forwarding policy (Section 5.3) is not implemented.** Only the default "inherit" behavior is. "strip" (which needs a rebuild to remove the marker) and "propagate" (which sets the Transitive bit and clears Partial) are not available. Section 5.3 rates this a SHOULD and asks for per-peer-group/per-neighbor granularity: a config surface, and a separate spec.
- **Under "inherit", a NON-transitive marker should not be forwarded at all** ("Non-transitive ATTR_TOMBSTONE markers ... are not forwarded by recognizing speakers under the default 'inherit' policy"). Ze forwards the wire bytes as-is, so a non-transitive marker still occupies the EBGP wire. Removing it requires a rebuild (the same machinery "strip" needs). Not addressed here.
- **EBGP RS-clients are not covered.** When `facts.isEBGP && facts.rsClient` and no ASN transcode is needed (`forward_rs.go:351-360`, `reactor_api_forward.go:526-535`), the peer is handed `update.WireUpdate` — the received wire, shared, with no per-destination buffer. Clearing there would corrupt the marker for every other peer including IBGP ones. Honouring Section 5.3 for RS-clients requires a per-update pooled copy (a third cached slot on `ReceivedUpdate`, mirroring `ebgpSlotASN4`/`ebgpSlotASN2`, plus release plumbing at `recent_cache.go:461,527`). That trades RS zero-copy forwarding for conformance on marker-bearing UPDATEs, and is Thomas's call.
- **The Partial bit is not touched.** Section 5.3's "inherit" bullet says transitive markers are "forwarded to peers with Partial bit set (RFC 4271 Section 5)", while the "propagate" bullet says a recognizing forwarder MUST clear Partial. The draft is ambiguous for a recognizing speaker under "inherit"; ze sets Partial nowhere today. Out of scope, flagged for Thomas.
- **The 252/253 code-point split is not unified.** See Implementation Summary.

## RFC Documentation
`// draft-mangin-idr-attr-tombstone-00 Section 5.3: "<quoted requirement>"` sits above
`clearTombstoneTransitive` (`internal/component/bgp/wireu/tombstone.go`) and above the
call site in `rewriteASPathPrepend` (`internal/component/bgp/wireu/aspath_rewrite.go`).

## Implementation Summary

### What Was Implemented
- The Section 5.3 "inherit" EBGP-boundary Transitive clear, at the EBGP re-encode seam.
- Nine dead citations in `attr_discard.go` repaired, plus eight more in four neighbouring files.

### Bugs Found/Fixed
- **Section 5.3 MUST violated**: transitive markers escaped to EBGP peers. Fixed; test `TestRewriteASPath_ClearsTombstoneTransitiveAtEBGPBoundary`.
- **Dead normative references**: `draft-mangin-idr-attr-discard-00` (renamed away) cited 9x in `attr_discard.go`; `Section 5.10` cited 2x there and 2x in its test file, in a draft whose Section 5 ends at 5.7. The rule those citations described is Section 5.7.
- **FOUND, NOT FIXED — two code points for one attribute**: `attribute.AttrTombstone = 252` (`internal/core/bgp/attribute/attribute.go:66`, written by `wireu.WriteTombstone`) vs `attrCodeAttrDiscard = 253` (`message/attr_discard.go`, written by the receive-time RFC 7606 path). Ze emits the same marker under two different type codes depending on which producer fired, and `ExtractUpstreamAttrDiscard` (which searches for 253) cannot see a 252 marker written by `wireu`, so the Section 5.1 upstream-merge rule silently fails against ze's own egress marker. Blast radius of unification: `attr_discard.go`, `attr_discard_test.go` (hex fixtures), `rfc7606.go`, `session_validation.go`, `docs/architecture/route-selection.md`, plus any `.ci` expectation carrying `FD` marker bytes. Deserves its own spec.

### Deviations from Plan
- The rename of `attr_discard.go` / `attrCodeAttrDiscard` / `ATTR_DISCARD` to the tombstone spelling was NOT done: see Implementation Audit. The citations (the urgent, normative part) are fixed; the symbol rename is mechanical, touches wire-byte fixtures, and belongs with the 252/253 unification.

## Implementation Audit

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRewriteASPath_ClearsTombstoneTransitiveAtEBGPBoundary/AttrTombstone` | |
| AC-2 | RETIRED (superseded) | code 253 is no longer a tombstone spelling after `spec-fixit-tombstone-code-point-split` (learned 1237) unified the code point to 252; a 253 attribute is now forwarded verbatim (`tombstone_forward_test.go` asserts its Transitive bit is untouched). AC-1 is the sole tombstone-transitive-clear case. | |
| AC-3 | Done | same test, asserts source `marker[0]` still 0xC0 | |
| AC-4 | Done | `TestRewriteASPath_NonTransitiveTombstoneUnchanged` | |
| AC-5 | Done | `TestRewriteASPath_ClearsTombstoneTransitiveExtendedLength` | |
| AC-6 | Done | value/length assertions in the boundary test | |
| AC-7 | Done | existing `wireu` + `reactor` suites pass unchanged; the clear is inside the existing copy, now guarded by `code == attribute.AttrTombstone` (was `isTombstoneCode`, deleted by the code-point unification) | |
| AC-8 | Done | `grep -rn "attr-discard-00" internal/component/bgp/message/attr_discard.go` returns only the deliberate rename note | |

## Review Gate

### Run 1 (closure — independent verification, 2026-07-21)

The two named deliverables (the ATTR_TOMBSTONE eBGP-boundary Transitive-clear MUST, and the 9
dead-citation repairs in `attr_discard.go`) landed in commit `706b77b7d`. An independent
verification pass confirmed: all live ACs met (AC-1/3/4/5/6/7/8; AC-2 RETIRED, superseded by the
252 unification — see audit); the forwarding-policy clear is `clearTombstoneTransitive`
(`wireu/tombstone.go:38`) called from `aspath_rewrite.go:528` under `code == attribute.AttrTombstone`
(IBGP keeps the zero-copy received wire); all 9 draft citations corrected to
`draft-mangin-idr-attr-tombstone-00` with section numbers verified to exist (5.10 was phantom ->
5.7); repo-wide grep confirms zero dead citations survive in source. The one remaining spec-gate
item — Documentation Update Checklist #7, a missing `docs/architecture/wire/attributes.md`
ATTR_TOMBSTONE entry — was filled this session (a code-252 table row + an `## ATTR_TOMBSTONE`
section, draft citations verified). `go test` (wireu/message/reactor), `go vet`, and
`make ze-rfc-check` all green.

**Verdict: CLEAN — 0 BLOCKER, 0 ISSUE.** The configurable inherit/strip/propagate policy, the
EBGP RS-client zero-copy gap, non-transitive markers on the EBGP wire, and the Partial bit remain
deliberately deferred Known Limitations (not ACs; recorded below). Gate satisfied.

## Known Limitations Recap (for the reader in a hurry)
The MUST is honoured for ordinary EBGP peers. It is NOT honoured for EBGP
route-server clients, because ze hands them the received wire with no
per-destination buffer; fixing that costs RS zero-copy and needs a decision.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Mutation test: break the clear, confirm red, restore
- [ ] Interop tests for protocol features (N/A — provisional code point, no other implementation)

### Completion (BLOCKING — before ANY commit)
- [ ] Partial/Skipped items have user approval (the RS-client gap and the configurable
      policy are reported to Thomas as decisions, NOT silently deferred)
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
