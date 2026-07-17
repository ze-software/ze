# Spec: fixit-tombstone-code-point-split

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-fixit-tombstone-ebgp-transitive |
| Phase | - |
| Updated | 2026-07-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/drafts/draft-mangin-idr-attr-tombstone-00.txt` Sections 4.1, 5.1, 8 (read only; never edit)
4. `internal/core/bgp/attribute/attribute.go`, `internal/component/bgp/message/attr_discard.go`, `internal/component/bgp/wireu/tombstone.go`

## Task

**Ze has TWO wire code points for ONE attribute.** The draft was renamed from
`attr-discard` to `attr-tombstone` and two half-implementations were left behind that
do not recognise each other.

| Producer | Code | Verified at |
|----------|------|-------------|
| Egress: `WriteTombstone` writes `attribute.AttrTombstone` | 252 (0xFC) | `internal/component/bgp/wireu/tombstone.go:77`; const at `internal/core/bgp/attribute/attribute.go:66` |
| Receive: `applyInPlace` stamps `attrCodeAttrDiscard` | 253 (0xFD) | `internal/component/bgp/message/attr_discard.go:119` (inside `applyInPlace`, `:110-129`); const at `:30` |
| Upstream merge: `ExtractUpstreamAttrDiscard` searches | 253 only | `internal/component/bgp/message/attr_discard.go:135` (`AttrFind` on `attrCodeAttrDiscard`) |

Consequence, verified at the producer: `ExtractUpstreamAttrDiscard` searches for 253
only, so **a 252 marker ze itself wrote is invisible to ze's own upstream-merge**, and
draft Section 5.1's merge rule silently fails against ze's own egress marker.

**The EGRESS path is not broken today, but the MERGE path is.** Commit `706b77b7d`
closed the LIVE egress bug by teaching the egress path to recognise both codes:
`attrTombstoneLegacy = 253` (`wireu/tombstone.go:26`) and `isTombstoneCode` (`:29-31`)
returns true for either. That shim covers ONLY wireu egress recognition; the message-tier
merge path is still split and two breakages are LIVE:
- `ExtractUpstreamAttrDiscard` (`attr_discard.go:135`) searches `attrCodeAttrDiscard`
  (253) only, so a 252 marker a ze speaker wrote via `WriteTombstone` is invisible to
  a second ze speaker's merge. An RFC 7606 Section 5.1 attr-discard merge between two
  ze speakers therefore FAILS NOW (this is AC-2's red case).
- `rebuildWithAttrDiscard` (`attr_discard.go:181`) removes only `attrCodeAttrDiscard`
  (253) before re-inserting, so a received 252 marker is not stripped and DUPLICATES on
  rebuild.
This spec **unifies the code points** (which fixes both merge breakages) and follows
the rename through the file and symbol names, so the dual-code compatibility shim can be
deleted.

Points to complete:

| # | Point |
|---|-------|
| 1 | Pick ONE code point (see Open Question O-1) and make every producer and consumer use it |
| 2 | Delete the `attrTombstoneLegacy` / `isTombstoneCode` dual-recognition shim once one code remains |
| 3 | Make `ExtractUpstreamAttrDiscard` search the unified code, so Section 5.1 merge works against ze's own egress marker |
| 4 | Follow the rename through: `attr_discard.go` (filename), `attrCodeAttrDiscard` (symbol), `ATTR_DISCARD` (comments, log strings), `DiscardEntry` / `ApplyAttrDiscard` / `rebuildWithAttrDiscard` naming |
| 5 | Update the one `.ci` carrying a live `FD` marker byte in an enforced wire expectation |
| 6 | Add a red-to-green test proving a ze-written marker is recognised by ze's own upstream-merge (the case that silently fails today) |

→ Constraint: this is a code-point unification, NOT a behavior change. The Section 5.3
egress Transitive-clear landed in `706b77b7d` and must keep working.

### Open Questions

| ID | Question | Status |
|----|----------|--------|
| O-1 | Which code point is correct, 252 or 253? | **The draft does not settle it.** Section 4.1 says "Attribute Code: TBD (see Section 8)". Section 8 requests allocation from the IANA "BGP Path Attributes" registry and states "Any unassigned value is acceptable", excluding only 255 ("Reserved for development" per RFC 2042). Both 252 and 253 are therefore draft-conformant provisional stand-ins. The choice is arbitrary and needs a ruling; pick one and record it. |
| O-2 | Should the provisional constant live in `internal/core/bgp/attribute` (where 252 is) or in `internal/component/bgp/message` (where 253 is)? Two constants in two tiers is what allowed the drift. | ~~Open~~ RESOLVED (see below) |

→ AUTONOMOUS DEFAULT (2026-07-17) [STAKES: protocol] — O-1: **Unify onto 252
(`attribute.AttrTombstone`).** PROVISIONAL — override before the draft's single TBD
IANA code point is allocated (draft Section 8). Rationale: the on-wire risk is
symmetric — both 252 and 253 are provisional, unallocated stand-ins for the draft's
one TBD code, and no third-party daemon interops with either (Assumption A-3), so
neither is "more irreversible" than the other. The tiebreaker is therefore
least-churn plus canonical placement, and both point to 252:
1. **Fewer on-wire emitters flip.** Unifying onto 252 changes ONE production
   on-wire emitter — the receive-time stamp in `attr_discard.go` (253→252). Unifying
   onto 253 would change TWO — `WriteTombstone`'s output at
   `internal/component/bgp/wireu/aspath_rewrite.go:515` and
   `internal/component/bgp/wireu/aspath_transcode.go:246`, both of which emit 252
   today — plus the exported constant and its display name.
2. **252 is already the canonical constant.** It is the exported
   `attribute.AttrTombstone` in the core attribute registry
   (`internal/core/bgp/attribute/attribute.go:66`), the only one carrying a display
   name (`ATTR_TOMBSTONE` at `:90`; 253 renders as `UNKNOWN(253)` in decode/dump/BMP),
   and its symbol matches the CURRENT draft name. 253's symbol
   `attrCodeAttrDiscard` matches the renamed-away `draft-mangin-idr-attr-discard-00`.
3. **The codebase already treats 252 as canonical, 253 as legacy.** The sibling's
   `internal/component/bgp/wireu/tombstone_forward_test.go` names its subtests
   `/AttrTombstone` (252) and `/AttrDiscardLegacy` (253), and `wireu/tombstone.go:21`
   names the 253 const `attrTombstoneLegacy`.
Thomas: override to 253 only if the receive-time value was deliberately chosen.

→ AUTONOMOUS DEFAULT (2026-07-17) — O-2: **The single surviving constant lives in
`internal/core/bgp/attribute` (252's home).** Rationale: `ai/rules/module-tiers.md`
— a wire constant shared by two components belongs in the lowest tier both import,
and `wireu` and `message` already both import `internal/core/bgp/attribute`. Delete
the message-tier duplicate `attrCodeAttrDiscard` and the wireu shim
`attrTombstoneLegacy`; message-tier code consumes `attribute.AttrTombstone` directly.
This permanently closes the two-constants-in-two-tiers drift seam. Follows directly
from O-1.

### Blast Radius (MEASURED 2026-07-16, not estimated)

The deferrals row estimated "5 files plus hex fixtures plus any `.ci` carrying `FD`
bytes". Measured by grep over `internal/`, `pkg/`, `cmd/`, `test/`, `docs/` for the
attribute-tombstone symbols (`AttrTombstone`, `attrCodeAttrDiscard`, `ATTR_DISCARD`,
`ATTR_TOMBSTONE`, `AttrDiscard`, `clearTombstoneTransitive`, `WriteTombstone`,
`attr-tombstone`, `attr_discard`, `attr-discard`):

| Category | Count | Files |
|----------|-------|-------|
| Non-test Go | **7** | `message/attr_discard.go` (51 refs), `wireu/tombstone.go` (17), `reactor/session_validation.go` (7), `wireu/aspath_rewrite.go` (3), `core/bgp/attribute/attribute.go` (2), `message/rfc7606.go` (2), `wireu/aspath_transcode.go` (1) |
| Test Go | **5** | `message/attr_discard_test.go`, `wireu/aspath_rewrite_test.go`, `wireu/aspath_transcode_test.go`, `wireu/tombstone_forward_test.go`, `wireu/tombstone_test.go` |
| `.ci` | **5** | `test/plugin/remove-private-as-export.ci` (LIVE hex, see below), plus `bgp-rs-asn4-transcode.ci`, `bgp-rs-fastpath-ebgp-shared.ci`, `bgp-rs-reactor-fastpath.ci`, `remove-private-as-replace-peer.ci` (comments only) |
| Docs | **2** | `docs/architecture/route-selection.md:54` (names the draft, no code point), `ai/DOCS-TO-CODE.md` (generated index; never hand-edit per `ai/rules/canonical-sources.md`) |

**Verdict: the 5-file estimate is LOW.** Non-test Go alone is 7; the full Go surface is
12 files, and 17 counting `.ci`.

→ Decision: exactly ONE `.ci` carries a live `FD` marker in an enforced wire
expectation: `test/plugin/remove-private-as-export.ci:49`
(`...4001010040020E02030000FDE80000FBF00000FBF140030401010101C0FD0405010000180A0000`,
the `C0FD0405010000` marker). The other four mention `ATTR_DISCARD` or `C0FD` in
comments only.

→ Constraint: **`FD` grep hits in `test/exabgp-compat/encoding/conf-mvpn.ci` and
`test/encode/mvpn.ci` are FALSE POSITIVES.** Those bytes are IPv6 address material
(`fd00::`, `fd12::`) and ASN 65000 (`0xFDE8`), not tombstone markers. A future agent
must not "fix" them. No `0xFC` (252) fixture exists anywhere in `test/`, because
nothing emits 252 into an asserted wire today.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/route-selection.md` - names the draft at `:54`; check whether the claim survives the rename
  → Constraint: the doc names the draft, not a code point, so it needs no code-point edit.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7606.md` - attribute discard is the action the marker records
  → Constraint: RFC 7606 "attribute discard" is what triggers marker generation; the marker itself is the draft's, not RFC 7606's.
- [ ] `rfc/short/rfc4271.md` - Section 5 optional attribute flags, Partial bit, merge rule
  → Constraint: Section 5.1 of the draft cites RFC 4271 Section 5 for the upstream merge.
- [ ] `rfc/drafts/draft-mangin-idr-attr-tombstone-00.txt` Sections 4.1, 5.1, 8 - the code point and the merge rule (READ ONLY; `rfc/drafts/` is Thomas's IETF work, never edit)
  → Decision: Section 8 "Any unassigned value is acceptable" means the draft does NOT pick between 252 and 253 (O-1).

**Key insights:** (summary of all checkpoint lines — minimal context to resume after compaction)
- The draft does not settle 252-vs-253; the split is a rename artifact, not a spec disagreement.
- The live bug is already closed by a dual-recognition shim; this spec removes the shim.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/core/bgp/attribute/attribute.go` - declares `AttrTombstone AttributeCode = 252` at `:66`, commented "draft-mangin-idr-attr-tombstone-00 (provisional)"
- [ ] `internal/component/bgp/message/attr_discard.go` - declares `attrCodeAttrDiscard uint8 = 253` at `:30`; its own comment at `:27-29` already records the split ("ze's second provisional value ... The two producers disagree and must be unified"). `applyInPlace` (`:110-129`) stamps 253 at `:119`. `ExtractUpstreamAttrDiscard` (`:134-148`) searches 253 at `:135`. `rebuildWithAttrDiscard` (`:163-275`) removes and re-inserts 253 at `:181`, `:247`, `:257`
- [ ] `internal/component/bgp/wireu/tombstone.go` - `WriteTombstone` (`:69`) writes `attribute.AttrTombstone` (252) at `:77`; `attrTombstoneLegacy = 253` at `:26`; `isTombstoneCode` (`:29-31`) accepts both; `clearTombstoneTransitive` at `:50`
- [ ] `internal/component/bgp/reactor/session_validation.go` - calls the marker path; logs "RFC 7606 upstream ATTR_DISCARD before merge" at `:119`; comment at `:109` cites draft Section 5.1
- [ ] `internal/component/bgp/message/rfc7606.go` - `RFC7606ActionAttributeDiscard` (`:25`) is the trigger; builds `DiscardEntry` list at `:166-171`

→ Line-number reconciliation (2026-07-17, verified against HEAD; earlier anchors have
drifted, the BEHAVIORS are all real): in `reactor/session_validation.go` the
receive-time apply is `message.ApplyAttrDiscard(pathAttrs, result.DiscardEntries)` at
`:145` (the spec's Data Flow says `:109`/`:117`); the "RFC 7606 upstream ATTR_DISCARD
before merge" log is at `:140` (spec says `:119`); the draft-Section-5.1 comment is at
`:130`. `rfc7606.go` `RFC7606ActionAttributeDiscard` is declared at `:25` (confirmed).
The two 252 egress emitters are confirmed at `wireu/aspath_rewrite.go:515` and
`wireu/aspath_transcode.go:246` (`WriteTombstone(dst, n, payload[off],
attribute.AttrAggregator, ...)`), and the egress recognition + Section 5.3 clear at
`wireu/aspath_rewrite.go:528` (`isTombstoneCode`) / `:542` (`clearTombstoneTransitive`).

**Behavior to preserve:** (unless user explicitly said to change)
- The Section 5.3 eBGP Transitive-clear landed in `706b77b7d` (`clearTombstoneTransitive`, reached from `rewriteASPathPrepend`) must keep working.
- `attrDiscardFlags` derivation `0x80 | (originalFlags & 0x50)` (draft Section 4.2), at `attr_discard.go:60-62`.
- The rebuild-vs-in-place decision in `ApplyAttrDiscard` (`:72-94`) and the Section 5.7 merged-flags transitivity rule (`:221-230`).
- Reason codes `DiscardReason*` (`:33-39`) and their values.
- `test/plugin/remove-private-as-export.ci` must still assert a real marker on the wire; only the code-point byte changes.

**Behavior to change:** (only if user explicitly requested)
- One code point replaces two. `ExtractUpstreamAttrDiscard` then finds a marker ze wrote itself, which it cannot do today.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Received BGP UPDATE wire bytes carrying a malformed attribute that RFC 7606 says to discard, arriving at the reactor's session validation path.
- A received UPDATE already carrying an upstream marker (252 or 253) in its path attributes section.

### Transformation Path
1. `message/rfc7606.go` validates attributes, yields `RFC7606ActionAttributeDiscard` + `DiscardEntry{Code, Reason}` list (`:166-171`)
2. `reactor/session_validation.go:109` calls the marker application per draft Section 5.1
3. `message/attr_discard.go` `ApplyAttrDiscard` (`:72`) checks for an upstream marker via `ExtractUpstreamAttrDiscard` (`:78`), which searches **253 only**
4. Single entry, no merge: `applyInPlace` (`:110`) overwrites the attribute in place, stamping **253** at `:119`, preserving wire layout for zero-copy forwarding
5. Merge or short value: `rebuildWithAttrDiscard` (`:163`) allocates a new attributes section carrying **253**
6. Egress eBGP funnel: `wireu/aspath_rewrite.go` `rewriteASPathPrepend` copies attributes into a pooled per-destination buffer; `isTombstoneCode` (`tombstone.go:29`) recognises 252 **or** 253 and `clearTombstoneTransitive` (`:50`) masks the Transitive bit
7. Egress generation: `wireu/tombstone.go` `WriteTombstone` (`:69`) writes **252** at `:77`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire ↔ message parse | `attribute.AttrFind` / `AttrIterator` over the path attributes section, zero-copy subslices | [ ] |
| message ↔ wireu | two independent code-point constants in two module tiers (`internal/core/bgp/attribute` and `internal/component/bgp/message`) — this is the drift seam | [ ] |
| Receive ↔ egress | the stamped byte lives in the shared received wire; egress re-encodes into a pooled per-destination buffer | [ ] |

### Integration Points
- `attribute.AttrFind` / `attribute.NewAttrIterator` - marker lookup; both take an `AttributeCode`, so a unified constant flows through unchanged.
- `attrCodeNames` map (`attribute.go:69+`) - check whether the unified code needs a display name entry.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — new commands, CLI/monitor views, families, and handlers register via the existing registry and the core discovers them; no new per-feature field, switch case, or factory is added to a core/shared package (small-core/registration; `ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The draft does not mandate 252 or 253 | `rfc/drafts/draft-mangin-idr-attr-tombstone-00.txt` Section 4.1 ("Attribute Code: TBD") and Section 8 ("Any unassigned value is acceptable") | O-1 answers itself and the choice is forced | Re-read draft Sections 4.1 + 8 | ~~unvalidated~~ validated 2026-07-17: draft Section 4.1 "Attribute Code: TBD (see Section 8)" and Section 8 "Any unassigned value is acceptable" (excl. 255) confirmed by re-reading the draft; the choice is forced, resolved as O-1 |
| A-2 | Exactly one `.ci` asserts a marker byte on the wire | grep for `C0FD` in `test/`: only `remove-private-as-export.ci:49` is an `expect=bgp:...hex=` line | A second test goes red unexpectedly | `grep -rn "C0FD" test/` | ~~unvalidated~~ validated 2026-07-17: only `test/plugin/remove-private-as-export.ci:49` carries `C0FD` in an `expect=bgp:...hex=` line; the other `C0FD` hits (`bgp-rs-fastpath-ebgp-shared.ci:22`, `remove-private-as-replace-peer.ci:17`, `remove-private-as-export.ci:45`, `bgp-rs-asn4-transcode.ci:32`) are comments only. No `C0FC` fixture exists |
| A-3 | No external implementation interops with ze's provisional code today | Both values are provisional stand-ins for a TBD allocation | Changing the code breaks a peer | Ask Thomas | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Deleting the dual-recognition shim re-opens the bug `706b77b7d` closed, if any producer is missed | `wireu/tombstone_forward_test.go` goes red, or a marker survives the eBGP boundary transitive | Do not delete `isTombstoneCode` until grep proves one code remains; keep the Section 5.3 tests green throughout |
| R-2 | The rename churns 12 Go files and buries the one-byte semantic change in noise | Review diff is unreadable | Land the code-point unification and the rename as separate commits, unification first |
| R-3 | `attr_discard.go` is ~330 lines and 51 of its refs are the old name; a mechanical rename may hit `DiscardEntry`/`DiscardReason*`, which the DRAFT still calls "discard" (Section 4.4 reason codes) | Renaming reason codes contradicts the draft | Rename the ATTRIBUTE (`ATTR_DISCARD` → `ATTR_TOMBSTONE`), keep the draft's "discard reason" vocabulary |
| R-4 | **DEPENDS on `spec-fixit-tombstone-ebgp-transitive`.** That in-progress sibling owns an unresolved decision (its lines 58-63) on `test/plugin/remove-private-as-export.ci:49`. Its preferred option (b) removes the invalid LOCAL_PREF from the fixture's source frame, so no marker is produced and the `C0FD0405010000` marker byte is deleted from the fixture. That would strand THIS spec's AC-5, Assumption A-2, Wiring Test row 2, and the `remove-private-as-export` functional-test row — all of which key on the fixture asserting a marker on the wire | Sibling closes with option (b), fixture loses its marker line | Do not update the fixture hex until the sibling resolves its decision; if (b) lands, re-home this spec's fixture-based ACs onto a marker-bearing test the sibling keeps |
| R-5 | **`C0FD`-vs-clear inconsistency — resolve during design, BEFORE Phase 1.** The fixture asserts `C0FD0405010000` (flags `0xC0`, Transitive SET) on an eBGP wire, yet `706b77b7d` added a Transitive clear at `wireu/aspath_rewrite.go:528-542` (`clearTombstoneTransitive` in `rewriteASPathPrepend`) and did NOT touch the fixture. So either the test is RED at HEAD, or this scenario bypasses `rewriteASPathPrepend` via a second egress funnel. The sibling spec (its lines 55-56) confirms the tension: clearing the bit for an eBGP destination makes `remove-private-as-export.ci:49` go RED. This also contradicts this spec's claim that "only the code-point byte changes" (the flags byte may change too) | Rerunning the fixture at HEAD is RED, or a grep shows a second egress path | Determine during design which funnel this fixture exercises and whether its expected flags are `0xC0` or `0x80`; reconcile with the sibling before editing the fixture hex |

### Open-decision resolutions (2026-07-17)

→ AUTONOMOUS DEFAULT (2026-07-17) — R-4 (fixture coupled to the in-progress sibling):
**Decouple this spec's on-wire proof from `remove-private-as-export.ci`.** Verified
against HEAD 2026-07-17: the sibling's Thomas-ruled decision (b) has NOT landed —
`test/plugin/remove-private-as-export.ci:21` still carries the input LOCAL_PREF
(`40050400000064`) and `:49` still asserts the marker (`C0FD0405010000`). Because (b)
is ruled and will land (it deletes that marker by removing the invalid input, so no
marker is generated), this spec MUST NOT hinge on that fixture asserting a marker.
Resolution: (i) re-home AC-5's wire-byte proof to a self-contained assertion this
spec owns — the unified code byte (`0xFC`) is emitted by `WriteTombstone` and stamped
by `applyInPlace`, asserted in `internal/component/bgp/wireu/tombstone_test.go` and
`internal/component/bgp/message/attr_discard_test.go`; (ii) the `.ci` edit is
conditional and append-only — if the marker still exists at implementation time,
change ONLY the code byte `C0FD`→`C0FC` at `:49` (and the matching comment at
`:44-45`); if (b) has already removed it, the fixture carries no marker and the unit
assertion alone satisfies AC-5. Either landing order leaves this spec implementable.
Thomas: override if you want AC-5 kept pinned on a `.ci` fixture.

→ AUTONOMOUS DEFAULT (2026-07-17) — R-5 (fixture flags `0xC0`-vs-`0x80` / which egress
funnel): **Out of scope for THIS spec; it belongs to the sibling.** This spec changes
only the attribute CODE byte (`0xFD`→`0xFC`, at wire offset `dst[n+1]`), never the
FLAGS byte (`dst[n]`). The `0xC0`-vs-`0x80` question is the sibling's Section 5.3
Transitive-clear concern and is orthogonal to the code point. Per the R-4 resolution
this spec no longer asserts a specific marker in `remove-private-as-export.ci`, so
R-5's "is the fixture RED at HEAD / which funnel does it exercise" tension cannot
block this spec. The funnel/flags reconciliation stays with
`spec-fixit-tombstone-ebgp-transitive`.
→ Constraint corrected: the earlier "only the code-point byte changes" claim is now
scoped explicitly to the CODE byte; the FLAGS byte is the sibling's to reconcile.

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Received UPDATE carrying a marker ze itself wrote | → | `ExtractUpstreamAttrDiscard` finds it and merges per Section 5.1 | `TestExtractUpstreamFindsZeOwnEgressMarker` |
| eBGP egress of a marker-bearing UPDATE | → | `clearTombstoneTransitive` via `rewriteASPathPrepend` | `test/plugin/remove-private-as-export.ci` (existing; hex updated to the unified code) |

→ AUTONOMOUS DEFAULT (2026-07-17): Wiring Test row 2 is governed by the R-4
resolution. If the sibling's decision (b) has removed the marker from
`remove-private-as-export.ci`, this row is satisfied instead by the re-homed
unified-code-byte assertion in `internal/component/bgp/wireu/tombstone_test.go`. Row 1
(`TestExtractUpstreamFindsZeOwnEgressMarker`) is unaffected by the sibling and remains
the primary red-to-green wiring proof for this spec (a `WriteTombstone`-produced 252
marker is found by the unified merge search).

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | grep the tree for the tombstone code point | Exactly one constant declares it; `attrTombstoneLegacy` no longer exists |
| AC-2 | Path attributes carrying a marker written by `WriteTombstone` | `ExtractUpstreamAttrDiscard` returns its (code, reason) pairs (fails today) |
| AC-3 | An UPDATE with an upstream marker plus a fresh local discard | `ApplyAttrDiscard` takes the rebuild path and emits ONE merged marker per Section 5.1 |
| AC-4 | eBGP egress of a transitive marker | Transitive bit cleared per Section 5.3 (behavior from `706b77b7d` preserved) |
| AC-5 | `test/plugin/remove-private-as-export.ci` | Asserts the marker on the wire with the unified code byte |

→ AUTONOMOUS DEFAULT (2026-07-17) — AC-5 proof re-homed (see R-4 resolution): AC-5's
on-wire proof is the unified CODE byte (`0xFC` = 252), demonstrated PRIMARILY by a
self-contained unit assertion (`WriteTombstone`/`applyInPlace` emit/stamp `0xFC`),
because the sibling's ruled decision (b) removes the marker from
`remove-private-as-export.ci`. If that marker still exists at implementation time,
AC-5 is ADDITIONALLY demonstrated by the `.ci` byte change `C0FD`→`C0FC` at `:49`; if
(b) has landed, the unit assertion alone demonstrates AC-5. The chosen code (252)
means the target byte is `0xFC`, not `0xFD`.

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Peer A sends ze a malformed LOCAL_PREF on eBGP; ze marks it and forwards to peer B; peer B (also ze) receives the marker and discards another attribute | wire → rfc7606 validate → ApplyAttrDiscard → egress → wire → ExtractUpstreamAttrDiscard → merged marker | `TestExtractUpstreamFindsZeOwnEgressMarker` + `test/plugin/remove-private-as-export.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExtractUpstreamFindsZeOwnEgressMarker` | `internal/component/bgp/message/attr_discard_test.go` | AC-2: a `WriteTombstone`-produced marker is found by the merge path (red before the fix) | |
| `TestTombstoneCodePointIsUnified` | `internal/component/bgp/wireu/tombstone_test.go` | AC-1: one code point; the legacy shim is gone | |
| `TestApplyAttrDiscardMergesUpstream` | `internal/component/bgp/message/attr_discard_test.go` | AC-3: Section 5.1 rebuild-and-merge | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Tombstone attribute code | 1-254 (0 Reserved per draft Section 6; 255 excluded per Section 8) | 254 | 0 | 255 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `remove-private-as-export` | `test/plugin/remove-private-as-export.ci` | Operator sees the marker on the eBGP wire with the unified code byte at `:49` | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A — no interop test | (none) | (none) | Opt-out justified: the code point is provisional and unallocated by IANA (draft Section 8), so no third-party daemon recognises it; interop value is limited to "the marker does not break a non-recognizing peer", already exercised by the sibling's forwarding tests and the unit suite. This spec changes only WHICH provisional byte ze uses (252 vs 253) and adds no interop-observable behavior, so there is nothing new to interop against | N/A |

### Future (if deferring any tests)
- None deferred. Every test in this plan (`TestExtractUpstreamFindsZeOwnEgressMarker`,
  `TestTombstoneCodePointIsUnified`, `TestApplyAttrDiscardMergesUpstream`, plus the
  re-homed unified-code-byte wire assertion per the R-4 resolution) is implemented
  within this spec's scope. The interop test is opted out on the provisional-code-point
  ground above, not deferred.

## Files to Modify
- `internal/core/bgp/attribute/attribute.go` - the 252 constant (`:66`)
- `internal/component/bgp/message/attr_discard.go` - the 253 constant (`:30`), the stamp (`:119`), the search (`:135`), the rebuild (`:181`, `:247`, `:257`); filename + symbol rename
- `internal/component/bgp/wireu/tombstone.go` - `attrTombstoneLegacy` (`:26`), `isTombstoneCode` (`:29-31`), `WriteTombstone` (`:77`)
- `internal/component/bgp/reactor/session_validation.go` - `ATTR_DISCARD` log string (`:119`) and comments
- `internal/component/bgp/wireu/aspath_rewrite.go` - marker recognition at the eBGP funnel
- `internal/component/bgp/wireu/aspath_transcode.go` - one reference
- `internal/component/bgp/message/rfc7606.go` - `Related:` header comment (`:3`), `DiscardEntry` construction
- `test/plugin/remove-private-as-export.ci` - the live `C0FD0405010000` hex expectation (`:49`)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] | Not expected — no config surface changes |
| CLI commands/flags | [ ] | Check whether `ze bgp decode` names the attribute |
| Functional test for new RPC/API | [ ] | `test/plugin/remove-private-as-export.ci` (existing) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 7 | Wire format changed? | [ ] | The on-wire code-point byte changes; check `docs/architecture/wire/*.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | [ ] | `docs/features/rfc-status.md` if it carries a draft row |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/route-selection.md:54` names the draft |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] | Grep `docs/` for anchors on the renamed file |

## Files to Create
- None beyond test additions. No new source files: the `attr_discard.go` →
  tombstone-named file change is a `git mv` (rename), tracked under Files to Modify.
  Test additions land in existing files: `TestExtractUpstreamFindsZeOwnEgressMarker`
  and `TestApplyAttrDiscardMergesUpstream` in
  `internal/component/bgp/message/attr_discard_test.go`;
  `TestTombstoneCodePointIsUnified` plus the re-homed unified-code-byte wire assertion
  (per the R-4 resolution) in `internal/component/bgp/wireu/tombstone_test.go`.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Blast Radius table |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-verify` |
| 13. /ze-review gate | Review Gate section |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** — write `TestExtractUpstreamFindsZeOwnEgressMarker`, prove it RED against today's split
   - Tests: `TestExtractUpstreamFindsZeOwnEgressMarker`
   - Files: `internal/component/bgp/message/attr_discard_test.go`
   - Verify: red for the right reason (252 marker not found by a 253 search)
2. **Phase: Unify the code point** — answer O-1, collapse to one constant, delete the shim
   - Tests: `TestTombstoneCodePointIsUnified`, phase 1 test turns green
   - Files: `attribute.go`, `attr_discard.go`, `tombstone.go`, `remove-private-as-export.ci`
   - Verify: `706b77b7d`'s Section 5.3 tests stay green
3. **Phase: Follow the rename** — `attr_discard.go` → tombstone-named file, `attrCodeAttrDiscard` and `ATTR_DISCARD` → tombstone vocabulary, keeping the draft's "discard reason" terms (R-3)
   - Tests: existing suites stay green
   - Files: the 12 Go files in the Blast Radius table, plus the 4 comment-only `.ci`
   - Verify: `grep -rn "ATTR_DISCARD\|attrCodeAttrDiscard"` returns nothing outside the draft's reason-code vocabulary
4. **Functional tests** → `test/plugin/remove-private-as-export.ci` hex updated and green
5. **RFC refs** → `// draft-mangin-idr-attr-tombstone-00 Section X.Y` comments kept accurate through the rename
6. **Full verification** → `make ze-verify`
7. **Complete spec** → learned summary, two commits

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | One code point; merge finds ze's own marker; Section 5.3 clear still fires |
| Naming | The draft's reason-code vocabulary ("discard reason") is NOT renamed; only the attribute is |
| Data flow | Zero-copy in-place stamp preserved; no new allocation on the happy path |
| Registration over hardcoding | New commands/views/families/handlers register and are core-discovered; no per-feature switch case added to a core/shared package (`ai/rules/plugin-self-containment.md`) |
| Rule: no-layering | `attrTombstoneLegacy` and `isTombstoneCode` fully deleted, not left dormant |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| One code-point constant | `grep -rn "252\|253" internal/component/bgp/wireu/tombstone.go internal/component/bgp/message/` |
| Shim deleted | `grep -rn "attrTombstoneLegacy\|isTombstoneCode" internal/` returns nothing |
| Live `.ci` updated | `grep -n "C0FD\|C0FC" test/plugin/remove-private-as-export.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | A hostile peer can send either code point; the merge parser reads (code, reason) pairs from attacker-controlled bytes (`attr_discard.go:141-146`). Confirm odd-length and oversized values stay bounded |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails behavior mismatch | Re-read source from Current Behavior |
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

- A rename that lands as "new name in a new file, old name left where it was" produces two implementations that pass their own tests and never meet. The split survived because each half was internally consistent: `wireu` round-tripped 252 and `message` round-tripped 253, so no test crossed the seam.
- `attr_discard.go:27-29` already documented the split in a comment before this spec existed. A comment recording a known defect is not a tracker; the work had no home until this spec.

## Core Insight
One wire attribute needs exactly one code point, owned at the lowest module tier that
both producers import. The 252/253 split was never a protocol disagreement — the draft
leaves the code TBD and accepts any unassigned value (Section 8) — but a rename
artifact: two internally-consistent halves (`wireu` round-tripped 252, `message`
round-tripped 253) that never shared a constant, so no test ever crossed the seam.
Unification is therefore a tier-placement fix — a single `attribute.AttrTombstone` in
`internal/core/bgp/attribute`, consumed by both tiers — not a behavior change. The only
behavior that changes as a side effect is the one that was silently broken:
`ExtractUpstreamAttrDiscard` can finally find a marker ze's own egress path wrote.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- The chosen code point stays provisional until IANA allocates (draft Section 8). Whatever is picked here will change again at allocation, so the value must be a single named constant, easy to move.

## RFC Documentation

Add `// draft-mangin-idr-attr-tombstone-00 Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: the code point's provisional status, the Section 5.1 merge rule, the Section 5.3 egress rule.

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
| One code point for one attribute | functional test + grep | (fill during implementation) |
| Ze's merge sees ze's own marker | unit test red-to-green | (fill during implementation) |

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
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
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
- [ ] **Commit B:** `git rm plan/spec-fixit-tombstone-code-point-split.md` only
