# Spec: attribute Len and WriteTo must agree for every address form

| Field | Value |
|-------|-------|
| Status | verification |
| Scope | protocol |
| Depends | - |
| Phase | 4/4 |
| Deferral shard | `plan/deferrals/fixit-attribute-length-from-family.md` |
| Handoff | verify |
| Updated | 2026-08-23 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later Opus 5 session reviews that commit and closes. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Four path attributes answer a size query from one source and write from another.
`Len()` counts octets from the address FAMILY, or from a constant, while `WriteTo()`
copies `netip.Addr.AsSlice()`, whose length is a property of the VALUE. The two
answers differ for an IPv6 address, for an IPv4-in-IPv6 address, and for the zero
`netip.Addr`.

The symptom is a write outside the region the size query reserved:

- `(*Aggregator).WriteTo` returns 8 for every input, and copies up to 16 address
  octets at `off+4`. An IPv6 `Address` puts 12 octets past the reserved region. The
  returned count still says 8, so no caller can detect it.
- `(*AS4Aggregator).WriteTo` has the same shape.
- `OriginatorID.WriteTo` returns `copy(...)`, so an IPv6 value writes 16 octets into
  the 4 that `Len()` promised.
- `(*NextHop).Len` returns 4 for the zero `netip.Addr` while `WriteTo` writes none.

The goal is one rule per attribute: `WriteTo` returns exactly `Len()` and touches no
octet beyond it, for an IPv4 address, an IPv4-in-IPv6 address, an IPv6 address, and
the zero `netip.Addr`.

The direction of each fix is decided by the RFC that fixes the field width, not by
convenience. AGGREGATOR, AS4_AGGREGATOR and ORIGINATOR_ID carry an address field the
RFC fixes at four octets, so `Len()` is right and the WRITE must be bounded. NEXT_HOP
has no fixed width in Ze (4 or 16), so the WRITE is right and `Len()` must follow the
value, exactly as `(*MPReachNLRI).nextHopOctets` already does after the MP_REACH
next-hop desync fix.

This is a latent contract violation today, not a live wire defect. No producer in the
tree can build any of these attributes with a non-IPv4 or zero address (see Current
Behavior, "Producer survey"). The structs are exported with public fields, so the next
caller that sets one directly reaches the defect, and the MP_REACH instance proves the
class ships.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### Architecture Docs
- [ ] `docs/architecture/wire/attributes.md` - the `// Design:` anchor of both changed
  source files; Sections 3, 7, 9 and 18 hold the wire layout of the four attributes
  → Constraint: the AGGREGATOR, AS4_AGGREGATOR and ORIGINATOR_ID address field is four
  octets. The document states no encoder behavior for a non-IPv4 address, so no claim
  in it becomes stale from this change.
- [ ] `ai/rules/performance.md` - this is wire-encoding code
  → Constraint: buffer-first `WriteTo(buf, off) int`. Keep that shape, add no
  allocation, and add no error return to `WriteTo`.
- [ ] `ai/rules/interop-and-goal-validation.md` - "Prove the test discriminates"
  → Constraint: every new test must be shown RED with its fix reverted. A test that
  asserts an absence (no octet past the region) is a named vacuity trap, so each test
  writes a canary pattern and asserts the canary survives.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc6793.md` - AS4_AGGREGATOR and the four-octet AGGREGATOR form
  → Constraint: `[RFC6793-6-2]` "AS4_AGGREGATOR in an UPDATE SHALL be considered
  malformed if the attribute length is not 8 (Section 6)". A sender that emits any
  other length produces an attribute the receiver must treat as malformed, so 8 is a
  ceiling on the write as well as a floor.
- [ ] `rfc/short/rfc4271.md` - AGGREGATOR (Section 5.1.7), NEXT_HOP (Section 5.1.3)
  → Constraint: AGGREGATOR carries an AS number and an IP address; NEXT_HOP carries a
  four-octet IPv4 address. Ze also accepts 16 octets in NEXT_HOP for RFC 4760
  compatibility, which is why NEXT_HOP has no single fixed width here.
- [ ] `rfc/short/rfc4456.md` - ORIGINATOR_ID (Section 8, Type Code 9)
  → Constraint: "This attribute is 4 bytes long". `Len()` is correct; the write must
  be bounded to match.
- [ ] `rfc/short/rfc4760.md` - MP_REACH_NLRI next hop, the precedent case
  → Constraint: the fix that landed for MP_REACH derives the count from the write and
  refuses an address with no wire form. This spec applies the same pair to NEXT_HOP.

**Key insights:** (minimal context to resume after compaction)
- The invariant is per attribute, and the direction differs per attribute. Bound the
  write where the RFC fixes the width. Derive the length where the value decides it.
- The existing invariant test compares only the returned COUNT, never the touched
  region, and its fixtures are IPv4 only. That is why the defect survived.
- The announce plan's count check cannot see the AGGREGATOR case at all: the write
  returns the promised 8 after it wrote 20.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/core/bgp/attribute/simple.go` - `(*NextHop).Len` returns 16 when
  `Addr.Is6()` and 4 otherwise, so the zero `Addr` counts 4 while `(*NextHop).WriteTo`
  returns `copy(buf[off:], n.Addr.AsSlice())`, which is 0. `(*Aggregator).Len` returns
  a constant 8; `(*Aggregator).WriteTo` writes the ASN then
  `copy(buf[off+4:], a.Address.AsSlice())` and returns 8 unconditionally.
  `(*Aggregator).WriteToWithContext` repeats the copy in both branches: at `off+4`
  returning 8, and at `off+2` returning 6 when the destination context is not ASN4.
  `OriginatorID.Len` returns 4; `OriginatorID.WriteTo` returns
  `copy(buf[off:], netip.Addr(o).AsSlice())`.
- [ ] `internal/core/bgp/attribute/as4.go` - `(*AS4Aggregator).Len` returns 8 with the
  RFC 6793 Section 6 quote in its godoc; `(*AS4Aggregator).WriteTo` writes the ASN then
  `copy(buf[off+4:], a.Address.AsSlice())` and returns 8.
  `(*AS4Aggregator).WriteToWithContext` delegates to `WriteTo`.
- [ ] `internal/core/bgp/attribute/mpnlri.go` - `(*MPReachNLRI).nextHopOctets` is the
  precedent: it counts `len(nh.AsSlice())` and documents that a family test and a write
  disagree for the zero `Addr`. `(*MPReachNLRI).ValidateNextHops` is the refusal half of
  that rule and returns `ErrUnencodableNextHop`, whose message names MP_REACH.
- [ ] `internal/core/bgp/attribute/len_writeto_test.go` - `TestLenMatchesWriteTo`
  asserts only `Len() == WriteTo(...)` into a 65536-octet buffer. It never inspects the
  octets after the value, and every address fixture is IPv4, so the AGGREGATOR and
  ORIGINATOR_ID overruns pass it today. `TestLenMatchesWriteToWithContext` has the same
  shape for the context-dependent AGGREGATOR rows.
- [ ] `internal/component/bgp/reactor/announce_build.go` - `announceAttrs.add` asks any
  attribute satisfying `announceNextHopValidator` (`ValidateNextHops() error`) first,
  then takes the value length from `attribute.ValueLenWithContext`, reserves it in the
  pooled `scratch` region, calls `WriteToWithContext`, and refuses when the write
  returns a different count. The refusal happens AFTER the write, so the check reports
  a mismatch the overrun already caused. It never fires for AGGREGATOR or
  AS4_AGGREGATOR: that write returns the promised 8 after it wrote 20.
- [ ] `internal/component/bgp/message/update_build.go` - the NEXT_HOP rail is guarded
  by `p.NextHop.Is4()`; ORIGINATOR_ID is built from `netip.AddrFrom4`.
- [ ] `internal/component/bgp/rib/commit.go` - `useTraditionalNLRI` gates the NEXT_HOP
  attribute on `nextHop.Is4()`.

**Producer survey:** (why this is latent) `ParseAggregator` and `ParseAS4Aggregator`
build the address from a four-octet slice, so they cannot produce a wide or zero
address. `(*AS4Aggregator).ToAggregator` carries that parsed address across. Every
ORIGINATOR_ID producer in `internal/component/bgp/message/update_build.go` and its
family-specific siblings calls `netip.AddrFrom4`. Every NEXT_HOP producer guards on
`Is4`, including `(*Builder).SetNextHopAddr`. No path from a received UPDATE reaches
any of the four defects today.

**Behavior to preserve:** (callers depend on these)
- The emitted octets for every attribute an existing producer can build. An IPv4
  address encodes exactly as it does today, in every method touched.
- `(*Aggregator).WriteToWithContext` keeps its two branches: 8 octets for a nil or
  ASN4 destination context, 6 octets with the AS_TRANS substitution otherwise. The
  6-octet branch carries the two-octet ASN form of RFC 6793. It must survive in its return
  value and in its ASN handling.
- `(*Aggregator).LenWithContext` keeps returning 8 and 6 on the same condition.
- Every `CheckedWriteTo` keeps its signature, its `wire.ErrBufferTooSmall` return, and
  its existing bound.
- `ErrUnencodableNextHop` stays the sentinel every `errors.Is` caller matches:
  `internal/component/bgp/reactor/peer_rib_routes.go` and
  `internal/component/bgp/reactor/reactor_api_batch.go`.
- The buffer-first shape: `WriteTo(buf, off) int`, no error return, no allocation.

**Behavior to change:** (only what the user asked for)
- `(*Aggregator).WriteTo`, `(*Aggregator).WriteToWithContext`,
  `(*AS4Aggregator).WriteTo` and `OriginatorID.WriteTo` write exactly four octets of
  address. An address with an IPv4 form writes those four octets. An address without
  one writes four zero octets. No case writes past the declared value length.
- `(*NextHop).Len` returns 0 for an address that is not valid, 4 for an IPv4 address,
  and 16 otherwise. `(*NextHop).WriteTo` is unchanged.
- `(*NextHop)` gains `ValidateNextHops() error`, so the announce plan refuses an
  unencodable NEXT_HOP instead of planning a zero-length one.
- `ErrUnencodableNextHop`'s message stops naming MP_REACH, and
  `(*MPReachNLRI).ValidateNextHops` adds the word to its own wrap so no message loses
  information.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A route reaches an announce rail as a stored route or an API announce, and the rail
  assembles `attribute.Attribute` values. `announceAttrs.add`
  (`internal/component/bgp/reactor/announce_build.go`) is the single point those
  attributes reach the wire through.
- A second entry point is `attribute.WriteAttrTo`, used by
  `(*CommitService).buildMPReachNLRI` and by the attribute `Builder`.

### Transformation Path
1. The rail contributes an attribute value to `announceAttrs.add`.
2. `add` calls `ValidateNextHops` when the attribute satisfies the interface, and
   abandons the plan with the cause when it returns an error.
3. `add` takes the value length from `attribute.ValueLenWithContext`, which reaches
   `LenWithContext` when the attribute has one and `Len()` otherwise.
4. `add` reserves that many octets of the pooled scratch region and calls
   `WriteToWithContext` at the current offset.
5. `add` compares the returned count against the reserved length and refuses on a
   mismatch. It then records the plan entry and advances the offset.
6. The plan entries are copied into the UPDATE body with the attribute headers.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor announce plan ↔ attribute codec | `announceAttrs.add` calls `ValueLenWithContext` then `WriteToWithContext` on the same value, and both must answer the same number | No |
| Attribute codec ↔ pooled scratch buffer | `WriteTo` writes into a shared region sized by the size query; an overrun lands in the next attribute's octets, or is silently truncated by `copy` at the end of the slice | No |
| Attribute codec ↔ peer | The value length is written into the attribute header, so a disagreement desynchronises the attribute block for everything after it | No |

### Integration Points
- `announceNextHopValidator` (`internal/component/bgp/reactor/announce_build.go`) -
  `(*NextHop)` starts satisfying this existing interface. No new interface, no new
  call site, no change in `announce_build.go`.
- `attribute.ErrUnencodableNextHop` - the refusal cause both announce rails already map
  to an operator-facing message. A NEXT_HOP refusal joins the MP_REACH one.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The fix stays inside the four codec methods and the existing validator interface. No rail changes |
| No unintended coupling (components stay isolated) | Yes | `internal/core/bgp/attribute` gains no import. The reactor gains no attribute-specific spelling |
| No duplicated functionality (extends existing, does not recreate) | Yes | One unexported helper replaces five copies of the same four-octet address write. `(*NextHop).Len` adopts the rule `nextHopOctets` already states |
| Zero-copy preserved where applicable (refs, not copies) | Yes | The helper writes through `As4()`, an array value. No slice is materialised, so the change removes allocation pressure rather than adding it |
| Registration over hardcoding | N-A | No new command, view, family, or handler. `(*NextHop)` is discovered through the existing `announceNextHopValidator` assertion, not through a new switch case |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No producer can build AGGREGATOR, AS4_AGGREGATOR or ORIGINATOR_ID with a non-IPv4 address today | Producer survey above: `ParseAggregator`, `ParseAS4Aggregator`, `ToAggregator`, and the `netip.AddrFrom4` call sites in `internal/component/bgp/message/update_build.go` and its siblings | The defect is live, not latent, and the fix is urgent rather than preventive. The fix itself does not change | Re-run the producer greps named in the survey before you write code | confirmed (2026-08-23): `ParseAggregator` and `ParseAS4Aggregator` build the address from a four-octet slice; every ORIGINATOR_ID producer calls `netip.AddrFrom4` (`internal/component/bgp/message/update_build.go` and its grouped, labeled, evpn, vpn and flowspec siblings). The defect is latent, and the fix is unchanged |
| A-2 | Writing four zero octets is the correct fallback for an address with no IPv4 form | RFC 4271 Section 5.1.7 makes the AGGREGATOR address the speaker's BGP Identifier, a four-octet value; RFC 4456 Section 8 fixes ORIGINATOR_ID at four octets. `WriteTo` has no error return, so a deterministic value is the only option that keeps the buffer-first shape | An alternative needs an error path through `WriteTo`, which `ai/rules/performance.md` forbids | The Key Design Decisions table records the rejected alternatives; the unit tests pin the zero fill | confirmed (2026-08-23): `writeIPv4AddressField` (`internal/core/bgp/attribute/simple.go`) keeps the buffer-first signature and no error return; the zero fill is pinned by the `IPv6` and `zero Addr` rows of `TestAggregatorWriteToStaysWithinLen`, `TestOriginatorIDWriteToStaysWithinLen` and `TestAS4AggregatorWriteToStaysWithinLen` |
| A-3 | `len(Addr.AsSlice())` is 0 for an invalid address, 4 for `Is4`, and 16 otherwise, for every `netip.Addr` | `net/netip` documents `AsSlice` as returning nil, a 4-octet slice, or a 16-octet slice by the address's own form, and `(*MPReachNLRI).nextHopOctets` already relies on it | `(*NextHop).Len` disagrees with `WriteTo` for some form | A unit test asserts `Len()` equals `len(Addr.AsSlice())` for all four forms | confirmed (2026-08-23): `TestNextHopLenMatchesWriteToForEveryAddressForm` asserts that equality, and the returned count, for IPv4, IPv4-in-IPv6, IPv6 and the zero `Addr` |
| A-4 | Making `(*NextHop)` satisfy `announceNextHopValidator` changes no rail behavior for a valid next hop | `announceAttrs.add` calls the method only when the assertion succeeds, and the method returns nil for any valid address | A valid NEXT_HOP is refused, dropping announcements | The reactor wiring tests cover both a valid IPv4 NEXT_HOP (planned) and the zero one (refused) | confirmed (2026-08-23): `TestAnnouncePlanKeepsValidNextHopAttribute` plans a four-octet value and reports no refusal cause; the whole `internal/component/bgp/reactor` package is green |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `(*NextHop).Len` returning 0 lets a zero-length NEXT_HOP attribute onto the wire, which RFC 4271 Section 5.1.3 does not admit. Today the announce plan refuses it only because the counts disagree | The reactor wiring test for the zero `Addr` shows a planned attribute instead of a refusal | `(*NextHop).ValidateNextHops` lands in Phase 1, BEFORE `Len` changes in Phase 3. The phase order is load-bearing, not cosmetic |
| R-2 | A caller outside the announce plan writes a NEXT_HOP through `attribute.WriteAttrTo` and gets a zero-length attribute | `(*CommitService).buildMPReachNLRI` is the known rail that asks no validator; `mpnlri.go` documents it | Out of scope here and unchanged by this spec. `plan/deferrals/fixit-commit-rail-nexthop-unvalidated.md` and `plan/spec-bgp-rib-deferred-commit-nexthop-validation.md` already own that rail. Record it in Known Limitations |
| R-3 | The `ErrUnencodableNextHop` message change breaks a caller that matched on text | Every use in the tree is `errors.Is`, and no test asserts the string | Keep the sentinel identity. Only the message text moves, and `(*MPReachNLRI).ValidateNextHops` regains the word MP_REACH in its own wrap |
| R-4 | The zero fill hides a producer defect that used to be visible as a garbage address | A future UPDATE carries `0.0.0.0` as an aggregator identity | Accepted. A deterministic wire-legal value beats stale pooled octets, and the producers are guarded. The alternative, refusing inside `WriteTo`, has no error channel |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A wrong bound writes the wrong AGGREGATOR, AS4_AGGREGATOR or ORIGINATOR_ID address into every UPDATE that carries one, which peers read as a wrong aggregator or a wrong route-reflector originator. A wrong `(*NextHop).Len` desynchronises the attribute block of an IPv4 UPDATE, which peers answer with a NOTIFICATION. Both are wire-visible on the normal announce path |
| How is it reverted? | A single commit revert. No config migration, no persisted state, no capability negotiated on it |
| Who else touches this path? | `internal/component/bgp/reactor/announce_build.go` (the plan), both announce rails (`internal/component/bgp/reactor/reactor_api_batch.go`, `internal/component/bgp/reactor/peer_rib_routes.go`), `internal/component/bgp/rib/commit.go`, and the commit rail recorded in `plan/deferrals/fixit-commit-rail-nexthop-unvalidated.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An announce whose attributes carry an AGGREGATOR with an IPv6 `Address`, planned through `announceAttrs.add` | → | `(*Aggregator).WriteToWithContext` | `TestAnnouncePlanAggregatorStaysInsideReservedRegion` |
| An announce whose attributes carry a NEXT_HOP with the zero `netip.Addr` | → | `(*NextHop).ValidateNextHops` | `TestAnnouncePlanRefusesUnencodableNextHopAttribute` |
| An announce whose attributes carry a NEXT_HOP with a valid IPv4 address | → | `(*NextHop).ValidateNextHops`, `(*NextHop).Len` | `TestAnnouncePlanKeepsValidNextHopAttribute` |

No `.ci` row: N/A for this spec. The surface is an encoder invariant with no operator
command, no config leaf, and no new output. The reachable entry point is the announce
plan, and the three Go tests above drive it from that entry point rather than calling
the codec directly.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `(*Aggregator).WriteTo` with `Address` set to an IPv4 address, an IPv4-in-IPv6 address, an IPv6 address, or the zero `netip.Addr` | Returns 8, and modifies exactly the eight octets from the offset. The address field holds the four IPv4 octets when the address has an IPv4 form, and four zero octets when it does not. Octets before the offset and after the eighth are unchanged |
| AC-2 | `(*Aggregator).WriteToWithContext` with each of the same four address forms, under a nil context, an ASN4 context, and a non-ASN4 context | Returns `LenWithContext` for that context (8, 8, 6), and modifies exactly that many octets. The 6-octet branch still substitutes AS_TRANS for an ASN of more than 65535 |
| AC-3 | `(*AS4Aggregator).WriteTo` and `WriteToWithContext` with each of the same four address forms | Returns 8, and modifies exactly the eight octets from the offset, with the same address-field rule as AC-1 |
| AC-4 | `OriginatorID.WriteTo` with each of the same four address forms | Returns 4, and modifies exactly the four octets from the offset, with the same address-field rule as AC-1 |
| AC-5 | `(*NextHop).Len` and `(*NextHop).WriteTo` with each of the same four address forms | `Len()` returns 4, 16, 16 and 0 respectively, `WriteTo` returns the same number, and it modifies exactly that many octets |
| AC-6 | An announce plan is built with a NEXT_HOP attribute whose `Addr` is the zero value | The plan is refused, its refusal cause matches `attribute.ErrUnencodableNextHop`, and no attribute is planned. A valid IPv4 NEXT_HOP in the same position is planned with a four-octet value |
| AC-7 | Any changed `Len`, `WriteTo` or `WriteToWithContext` is called in a loop | Zero heap allocations per call, measured with `testing.AllocsPerRun` |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Announces an IPv4 route to a peer, the normal path that carries a NEXT_HOP attribute | announce rail → `announceAttrs.add` → `(*NextHop).ValidateNextHops` → `(*NextHop).Len` → `(*NextHop).WriteTo` → UPDATE body | `TestAnnouncePlanKeepsValidNextHopAttribute` |

No other operator-visible operation changes. The remaining ACs are encoder invariants
reached from the same rail.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAggregatorWriteToStaysWithinLen` | `internal/core/bgp/attribute/simple_test.go` | AC-1: four address forms, canary buffer, exact returned count, zero fill for an address with no IPv4 form | green; red under M1 |
| `TestAggregatorWriteToWithContextStaysWithinLen` | `internal/core/bgp/attribute/simple_test.go` | AC-2: the same four forms across nil, ASN4 and non-ASN4 contexts, including the 6-octet AS_TRANS branch | green; red under M1 |
| `TestOriginatorIDWriteToStaysWithinLen` | `internal/core/bgp/attribute/simple_test.go` | AC-4: four address forms, canary buffer, returned count always 4 | green; red under M1 |
| `TestNextHopLenMatchesWriteToForEveryAddressForm` | `internal/core/bgp/attribute/simple_test.go` | AC-5: `Len()` equals `WriteTo()` and equals `len(Addr.AsSlice())` for all four forms, canary buffer | green; red under M2 |
| `TestAS4AggregatorWriteToStaysWithinLen` | `internal/core/bgp/attribute/as4_test.go` | AC-3: four address forms through both `WriteTo` and `WriteToWithContext` | green; red under M1 |
| `TestAttributeWriteToAllocatesNothing` | `internal/core/bgp/attribute/simple_test.go` | AC-7: `testing.AllocsPerRun` is 0 for each changed method. Follow the shape already in `internal/core/bgp/attribute/span_test.go` | green; 0 allocs/op on all eight calls |
| `TestLenMatchesWriteTo` (extend the existing table) | `internal/core/bgp/attribute/len_writeto_test.go` | AC-1, AC-3, AC-4, AC-5: add the IPv6, IPv4-in-IPv6 and zero-`Addr` fixtures for `NextHop`, `Aggregator`, `AS4Aggregator` and `OriginatorID`, so the package-wide invariant covers the forms it never covered | green; the NextHop rows go red under M2, the rest stay green under M1 (the count is blind to the overrun) |
| `TestLenMatchesWriteToWithContext` (extend the existing table) | `internal/core/bgp/attribute/len_writeto_test.go` | AC-2: the same three extra address forms for the context-dependent AGGREGATOR rows | green; the added `LenWithContext` assertion is what makes the row more than a count check |
| `TestAnnouncePlanAggregatorStaysInsideReservedRegion` | `internal/component/bgp/reactor/announce_build_attr_region_test.go` | Wiring: the plan's reserved region is intact after an AGGREGATOR with an IPv6 address | green; red before the fix on the IPv6, IPv4-in-IPv6 and zero-`Addr` forms <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `TestAnnouncePlanRefusesUnencodableNextHopAttribute` | `internal/component/bgp/reactor/announce_build_attr_region_test.go` | AC-6: the refusal cause is `attribute.ErrUnencodableNextHop`, not a count mismatch | green; red before the fix (nil cause) and red under M3 (the attribute is emitted) <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |
| `TestAnnouncePlanKeepsValidNextHopAttribute` | `internal/component/bgp/reactor/announce_build_attr_region_test.go` | AC-6 negative half and A-4: a valid IPv4 NEXT_HOP is still planned, with a four-octet value | green; red under M4 <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) --> |

**Canary rule for every new codec test:** fill the destination buffer with a non-zero
pattern, write at a non-zero offset, then assert three things: the returned count, the
octets inside the declared value region, and that every octet outside that region still
holds the pattern. Without the third assertion the test cannot see the defect, because
the returned count is already what the caller expects.

**Red evidence required** (`ai/rules/interop-and-goal-validation.md`, "Prove the test
discriminates"). Revert each fix in turn and record the failure:

Four mutations were applied on 2026-08-23, each to `internal/core/bgp/attribute/simple.go`,
a file no other session had modified. Each was verified present with `grep` before the
run, each run was a real execution reporting a duration (never `ok (cached)`), and each
file was restored by `cp` from a copy saved beforehand and confirmed byte-identical by
SHA-256. `git checkout`, `git restore` and `git stash` were not used.

| Mutation | What it reverts |
|----------|-----------------|
| M1 | `writeIPv4AddressField` back to `copy(buf[off:], addr.AsSlice())` |
| M2 | `(*NextHop).Len` back to the `Is6` family test |
| M3 | `(*NextHop).ValidateNextHops` back to `return nil` for every address |
| M4 | `(*NextHop).ValidateNextHops` widened to refuse every address (the discriminating half) |

| Test | What red looks like with the fix reverted | Observed |
|------|-------------------------------------------|----------|
| `TestAggregatorWriteToStaysWithinLen`, IPv6 form | The canary octets after the eighth hold address octets instead of the pattern. Twelve octets past the region | M1: address field is `20 01 0d b8` not zeros, and "scratch octet 8 lies outside the declared value region and was written" |
| `TestAggregatorWriteToStaysWithinLen`, zero-`Addr` form | The four address octets still hold the canary pattern instead of zeros: nothing was written and the count still said 8 | M1: the field reads `aa aa aa aa` in the plan test and `5a 5a 5a 5a` in the unit test, both the canary |
| `TestOriginatorIDWriteToStaysWithinLen`, IPv6 form | The returned count is 16 rather than 4, and twelve canary octets are overwritten | M1: red on the IPv6, IPv4-in-IPv6 and zero-`Addr` rows |
| `TestNextHopLenMatchesWriteToForEveryAddressForm`, zero-`Addr` form | `Len()` is 4 while `WriteTo` returns 0 | M2: "expected 0, actual 4", and `TestLenMatchesWriteTo/NextHop_zero_Addr` reports "Len()=4 but WriteTo()=0" |
| `TestAnnouncePlanRefusesUnencodableNextHopAttribute` | The plan fails with the generic "attribute size query disagreed with its own write" and a nil cause, so the `errors.Is` assertion against `ErrUnencodableNextHop` is false | Two reds, both recorded. Before the fix: the nil cause, exactly as predicted. Under M3, with the counts already agreeing at zero: the plan EMITS the attribute, which is R-1 demonstrated |
| `TestAnnouncePlanAggregatorStaysInsideReservedRegion` | The octets after the reserved region hold IPv6 address bytes, and no existing check reports anything: the write returned the promised 8 | Red before the fix on three of four forms, with `plan.emit` reporting success throughout |
| `TestAnnouncePlanKeepsValidNextHopAttribute` | (the discriminating half: it must go red if the refusal widens) | M4: "a valid IPv4 NEXT_HOP must be planned: attribute next hop has no wire form" |

Two facts the mutations establish beyond the per-test reds:

- `TestLenMatchesWriteTo` and `TestLenMatchesWriteToWithContext` stayed GREEN under M1
  for every AGGREGATOR, AS4_AGGREGATOR and ORIGINATOR_ID row. A count-only invariant
  cannot see this defect, which is why the canary assertion is the discriminating one.
- `TestAnnouncePlanKeepsValidNextHopAttribute` stayed green under M3, so the refusal
  test's red is attributable to the validator rather than to the plan refusing broadly.

### Boundary Tests (numeric inputs)
The numeric input here is the octet count of the address field. The boundary is the
address FORM, because the form is what changes the count.

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| AGGREGATOR address field | 4 octets, fixed (RFC 4271 Section 5.1.7) | IPv4 address, 4 octets written | zero `netip.Addr`, 0 octets written today, must become 4 zero octets | IPv6 address, 16 octets written today, must become 4 |
| AS4_AGGREGATOR address field | 4 octets, fixed (RFC 6793 Section 6, total 8) | IPv4 address, 4 octets written | zero `netip.Addr` | IPv6 address, and IPv4-in-IPv6, which must write its 4 unmapped octets |
| ORIGINATOR_ID value | 4 octets, fixed (RFC 4456 Section 8) | IPv4 address, 4 octets | zero `netip.Addr` | IPv6 address |
| NEXT_HOP value | 0, 4 or 16 octets, decided by the value | IPv6 address, 16 octets | zero `netip.Addr`: `Len` must be 0 and the announce must be refused | N/A -- no form exceeds 16 |
| AGGREGATOR total, non-ASN4 context | 6 octets | ASN 65535, written as two octets | N/A | ASN 65536, written as AS_TRANS 23456, still 6 octets |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A -- not applicable | - | No user-facing surface changes: no command, no config leaf, no new output, and no wire change for any input a producer can build today. The reachable entry point is the announce plan, covered by the three reactor wiring tests above, which carry the end-to-end proof for an encoder invariant | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| None owed | - | - | No scenario is owed, and adding one would be vacuous. Every encoding a current producer can reach is byte-identical before and after the fix (see the producer survey), so a peer daemon cannot distinguish the two trees. That is the first vacuity trap named in `ai/rules/interop-and-goal-validation.md`: reverting the change leaves the peer's routing table identical. The discriminating evidence is the canary assertion, which no peer can make | |

## Files to Modify
- `internal/core/bgp/attribute/simple.go` - bound the address write in
  `(*Aggregator).WriteTo` and both branches of `(*Aggregator).WriteToWithContext`;
  bound `OriginatorID.WriteTo` and return 4; derive `(*NextHop).Len` from the value;
  add `(*NextHop).ValidateNextHops`; add one unexported helper that writes a four-octet
  IPv4 address field, used by all four call sites. Update the godoc of every method
  whose rule changes (`ai/rules/stale-comments.md`).
- `internal/core/bgp/attribute/as4.go` - bound the address write in
  `(*AS4Aggregator).WriteTo` through the same helper, and update its godoc.
- `internal/core/bgp/attribute/mpnlri.go` - generalise the `ErrUnencodableNextHop`
  message and its doc comment so it covers NEXT_HOP as well as MP_REACH, and add the
  word MP_REACH to the wrap inside `(*MPReachNLRI).ValidateNextHops` so no message
  loses information.
- `internal/core/bgp/attribute/simple_test.go` - the five new unit tests.
- `internal/core/bgp/attribute/as4_test.go` - the AS4_AGGREGATOR unit test.
- `internal/core/bgp/attribute/len_writeto_test.go` - extend both existing tables with
  the IPv6, IPv4-in-IPv6 and zero-`Addr` fixtures.
- `docs/architecture/wire/attributes.md` - only if a claim there is contradicted by the
  fix. Read Sections 3, 7, 9 and 18 and record the check either way.

## Files to Create
- `internal/component/bgp/reactor/announce_build_attr_region_test.go` - the three <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
  wiring tests. Copy the plan construction from the existing
  `internal/component/bgp/reactor/announce_build_cause_test.go`, which already builds a
  plan and reads its refusal cause.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No config leaf. The change is inside the encoder |
| YANG validation constraints | No | No new leaf |
| YANG custom validators | No | No new leaf |
| CLI commands/flags | No | No command surface |
| CLI grammar (keyword before value) | No | No command surface |
| Editor autocomplete | No | No config leaf |
| Functional test for new RPC/API | No | No RPC or API added |
| Pipe completeness | No | No command output |
| Env var registration | No | No environment leaf |
| Doctor check for runtime dependencies | No | No new file path, socket, service, module, port, or certificate |
| Prometheus counters/metrics | No | The NEXT_HOP refusal reaches the existing announce refusal path, which already logs and counts in `internal/component/bgp/reactor/reactor_api_batch.go` and `internal/component/bgp/reactor/peer_rib_routes.go`. No new metric |
| BGP family surface (new SAFI / capability / attribute) | No | No new SAFI, capability, or attribute type code. Four existing attributes change their length rule only |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The fix is invisible to an operator whose producers are guarded |
| 2 | Config syntax changed? | No | No config surface |
| 3 | CLI command added/changed? | No | No command surface |
| 4 | API/RPC added/changed? | No | No API surface |
| 5 | Plugin added/changed? | No | No plugin touched |
| 6 | Has a user guide page? | No | No operator-facing behavior changes |
| 7 | Wire format changed? | No | Every encoding a current producer can build is byte-identical. Only inputs no producer can build change, and those were malformed before |
| 8 | Plugin SDK/protocol changed? | No | `pkg/` is untouched |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No requirement changes classification, and no `rfc/short/` row moves. The `[RFC6793-6-2]` tags in `internal/core/bgp/attribute/rfc6793_as4_test.go` cover the reception side and stay as they are. See Key Design Decisions for why no new `RFC requirement:` tag is added |
| 10 | Test infrastructure changed? | No | New tests use the existing package harness |
| 11 | Affects daemon comparison? | No | No feature added or removed |
| 12 | Internal architecture changed? | N-A | The rule "derive the count from the write, or bound the write to the count" is already stated in the `internal/core/bgp/attribute/mpnlri.go` godoc. Extend that godoc rather than adding an architecture page |
| 13 | Route metadata keys added/changed? | No | No metadata key |
| 14 | Prometheus counters added/changed? | No | No counter |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registers |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Three documents carry `source:` anchors on the changed files: `docs/DESIGN.md` (the NEXT_HOP, MED, LOCAL_PREF anchor on `simple.go`), `docs/architecture/wire/attributes.md` (the AS4 anchor in the AS4_PATH section), and `docs/architecture/edge-cases/as4.md` (four anchors on `as4.go`). Read each surrounding claim and confirm none states that the encoder writes `AsSlice()` unbounded. Update only a claim the fix contradicts, and record the check in the closure section either way. **Checked 2026-08-23: no claim is contradicted, so no doc changed.** `docs/architecture/wire/attributes.md` Section 3 states NEXT_HOP is "4 bytes (IPv4)" and routes every other family to MP_REACH_NLRI; Section 7 states AGGREGATOR is "6 bytes (2-byte AS) or 8 bytes (4-byte AS)"; Section 9 states ORIGINATOR_ID is "4 bytes"; Section 18 states AS4_AGGREGATOR is "8 bytes". `docs/architecture/edge-cases/as4.md` draws "Aggregator IP (4 bytes)". The `docs/DESIGN.md` anchors carry a type-code table and no encoder claim. The fix makes each of those statements true for every address form, where the AGGREGATOR and ORIGINATOR_ID ones were false for an IPv6 value |
| 17 | Existing docs show config/CLI/API examples for this area? | No | The area has no config, CLI, or API example |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the announce plan able to refuse a
   NEXT_HOP that has no wire form, before any length rule changes
   - Tests: `TestAnnouncePlanRefusesUnencodableNextHopAttribute`,
     `TestAnnouncePlanKeepsValidNextHopAttribute`,
     `TestAnnouncePlanAggregatorStaysInsideReservedRegion`
   - Files: `internal/core/bgp/attribute/simple.go` (`(*NextHop).ValidateNextHops`),
     `internal/core/bgp/attribute/mpnlri.go` (sentinel message and both doc comments),
     `internal/component/bgp/reactor/announce_build_attr_region_test.go` <!-- doc-links: ignore (planned by this spec, written when the spec is implemented) -->
   - Verify: the refusal test fails first because the plan reports a count mismatch
     with a nil cause, and passes once `(*NextHop)` satisfies `announceNextHopValidator`.
     The AGGREGATOR region test still fails: it is Phase 2's job
   - Order note: this phase MUST land before Phase 3. Changing `(*NextHop).Len` first
     lets a zero-length NEXT_HOP through the plan (R-1)
2. **Phase: Bound the fixed-width address writes** -- AGGREGATOR, AS4_AGGREGATOR,
   ORIGINATOR_ID
   - Tests: `TestAggregatorWriteToStaysWithinLen`,
     `TestAggregatorWriteToWithContextStaysWithinLen`,
     `TestAS4AggregatorWriteToStaysWithinLen`,
     `TestOriginatorIDWriteToStaysWithinLen`, and the extended
     `TestLenMatchesWriteTo` and `TestLenMatchesWriteToWithContext` tables
   - Files: `internal/core/bgp/attribute/simple.go`,
     `internal/core/bgp/attribute/as4.go`, and the three attribute test files
   - Verify: write the canary tests first and watch each fail on the octets past the
     region, then add the helper and route all four call sites through it. The
     6-octet AGGREGATOR branch keeps returning 6 with the AS_TRANS substitution.
     `TestAnnouncePlanAggregatorStaysInsideReservedRegion` turns green here
3. **Phase: Derive the NEXT_HOP length from the value**
   - Tests: `TestNextHopLenMatchesWriteToForEveryAddressForm`, plus the `NextHop` rows
     added to `TestLenMatchesWriteTo`
   - Files: `internal/core/bgp/attribute/simple.go`
   - Verify: `Len()` returns 0 for an invalid address, 4 for `Is4`, and 16 otherwise,
     and the test pins that this equals `len(Addr.AsSlice())` for every form. The
     Phase 1 refusal test must still pass: it is what keeps a zero-length NEXT_HOP off
     the wire
4. **Phase: Allocation and gates**
   - Tests: `TestAttributeWriteToAllocatesNothing`
   - Files: `internal/core/bgp/attribute/simple_test.go`
   - Verify: `go test -race ./internal/core/bgp/attribute`,
     `go test -race ./internal/component/bgp/reactor`,
     `./le changed scope`, `./le rfc check`, and `./le doc check verify` if any doc
     changed. Then record the red evidence table by reverting each fix in turn

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation, named by file and symbol, and AC-6 names the reactor test rather than a codec test |
| Feature completeness | All five write sites go through the one helper: `(*Aggregator).WriteTo`, both branches of `(*Aggregator).WriteToWithContext`, `(*AS4Aggregator).WriteTo`, `OriginatorID.WriteTo`. A site left behind leaves the class half-fixed |
| Correctness | The 6-octet AGGREGATOR branch writes the address two octets in and returns 6. The AS_TRANS substitution for an ASN of more than 65535 is unchanged |
| Correctness | An IPv4-in-IPv6 address writes its four unmapped octets, never sixteen and never zeros |
| Naming | The helper is unexported and names what it writes (a four-octet IPv4 address field), not who calls it |
| Data flow | `internal/component/bgp/reactor/announce_build.go` is NOT edited. `(*NextHop)` is discovered through the existing interface |
| Rule: `ai/rules/performance.md` | No allocation added, no error return added to `WriteTo`, the buffer-first signature unchanged |
| Rule: `ai/rules/stale-comments.md` | The godoc of `(*NextHop).Len`, `(*Aggregator).WriteTo`, `(*Aggregator).WriteToWithContext`, `(*AS4Aggregator).WriteTo`, `OriginatorID.WriteTo` and `ErrUnencodableNextHop` describes the new rule |
| Rule: `ai/rules/interop-and-goal-validation.md` | Every new test has a recorded red, and each red is the canary or the count, never a compile error |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No `AsSlice()` write remains in a fixed-width attribute field | `grep -n "AsSlice()" internal/core/bgp/attribute/simple.go internal/core/bgp/attribute/as4.go` shows no copy into an AGGREGATOR, AS4_AGGREGATOR or ORIGINATOR_ID field |
| The invariant holds package-wide | `go test -race ./internal/core/bgp/attribute` |
| The announce plan refuses an unencodable NEXT_HOP | `go test -race ./internal/component/bgp/reactor` |
| Red evidence for every new test | The red evidence table in this spec, filled with the actual failure lines |
| The RFC ledger is unchanged | `./le rfc check` |
| Lint clean | `./le changed scope` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | No new parse path. The change is on the write side only, and it makes a malformed value write a bounded, deterministic result instead of an out-of-region one |
| Memory safety | The overrun writes into a pooled scratch region shared by the whole announce, so today the failure mode is silent corruption of a later attribute or silent truncation by `copy`. Confirm the fix removes the write, rather than making the buffer larger |
| Resource exhaustion | None. No allocation, no unbounded loop, no attacker-controlled size |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| A new test fails for the wrong reason (compile, fixture) | Fix the test setup. A red that is not the canary or the count is not the red this spec asks for |
| `./le rfc check` goes red after a test edit | Remove any `RFC requirement:` tag you added and keep the plain RFC citation comment. The ledger is out of scope for this spec |
| An existing reactor test fails on the NEXT_HOP refusal | Re-read A-4. A valid address must return nil from `ValidateNextHops` |
| A doc anchor claim is contradicted by the fix | Update that claim with a source anchor, in the same commit |
| 3 fix attempts failed | STOP. Report all 3 approaches in the handoff. Do not weaken a test to reach green |

## Design Insights

- A size query and a write disagree whenever they read different sources. The MP_REACH
  fix already wrote that rule down in `internal/core/bgp/attribute/mpnlri.go`; these
  four attributes are the same rule unapplied.
- An invariant test that compares only the returned COUNT cannot see a write that
  returns the promised number and writes more. `TestLenMatchesWriteTo` passed over an
  attribute that writes twelve octets past its region.
- The announce plan's count check is a backstop for the count, not for the region. It
  is structurally blind to the AGGREGATOR case.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Bound the write for AGGREGATOR, AS4_AGGREGATOR and ORIGINATOR_ID; derive the length for NEXT_HOP | Apply one direction everywhere | The RFC decides. RFC 6793 Section 6 fixes AS4_AGGREGATOR at 8 octets and RFC 4456 Section 8 fixes ORIGINATOR_ID at 4, so the length is not free to move. NEXT_HOP is 4 or 16 in Ze, by RFC 4271 plus RFC 4760 compatibility, so the value decides the count |
| An address with no IPv4 form writes four zero octets | Return a shorter count; return an error from `WriteTo`; panic | A shorter count reopens the disagreement this spec exists to close. `WriteTo` has no error channel and `ai/rules/performance.md` fixes its signature. A panic on the encode path is worse than a wire-legal zero. The producers are guarded, so the fallback is unreachable today and exists to be deterministic |
| `(*NextHop).Len` branches on `IsValid` and `Is4` rather than calling `len(Addr.AsSlice())` | Mirror `nextHopOctets` literally | The two are equal for every `netip.Addr`, and a unit test pins that equality. The branch materialises no slice, which keeps NEXT_HOP allocation-free on the hottest encode path in the daemon (`ai/rules/performance.md`) |
| `(*NextHop)` gains `ValidateNextHops` in the same change | Change `Len` alone | Changing `Len` alone converts a refused announce into a zero-length NEXT_HOP attribute, which RFC 4271 Section 5.1.3 does not admit. The refusal half is part of the same rule, exactly as `mpnlri.go` documents for MP_REACH |
| No new `RFC requirement:` tag | Tag the new AS4_AGGREGATOR test with `RFC6793-6-2` | The requirement is already gated with both polarities on the reception side in `internal/core/bgp/attribute/rfc6793_as4_test.go`, and this spec changes no classification. A sender-side tag on a reception-worded requirement is a ledger decision, not a defect fix. The enforcing code still carries its RFC citation comment (`ai/rules/rfc-compliance.md`, "RFC MUST Comments") |

## Known Limitations
- `(*CommitService).buildMPReachNLRI` (`internal/component/bgp/rib/commit.go`) writes
  through `attribute.WriteAttrTo` rather than through the announce plan, so it asks no
  validator. That rail is unchanged by this spec and is already owned by
  `plan/deferrals/fixit-commit-rail-nexthop-unvalidated.md`, whose destination is
  `plan/spec-bgp-rib-deferred-commit-nexthop-validation.md`.
- The zero fill makes an unset AGGREGATOR address encode as `0.0.0.0` rather than
  failing. This spec adds no producer-side guard against setting one, because the
  exported struct has no constructor to guard.

## RFC Documentation (Scope: protocol)

Add the citation above the enforcing code:

| Site | Comment to add |
|------|----------------|
| The four-octet address helper in `internal/core/bgp/attribute/simple.go` | `// RFC 4271 Section 5.1.7`, `// RFC 4456 Section 8`, and `// RFC 6793 Section 6`, naming why the field is four octets and cannot grow |
| `(*AS4Aggregator).WriteTo` | `// RFC 6793 Section 6: "The AS4_AGGREGATOR attribute in an UPDATE message SHALL be considered malformed if the attribute length is not 8."` -- the sender side of the same sentence |
| `(*NextHop).Len` | `// RFC 4271 Section 5.1.3` for the four-octet IPv4 form and `// RFC 4760` for the sixteen-octet one, with the rule that the count comes from the value |
| `(*NextHop).ValidateNextHops` | `// RFC 4271 Section 5.1.3` -- a zero-length NEXT_HOP is not a wire form the RFC admits, so the attribute is refused rather than encoded |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes, or the shared-checkout evidence path in `ai/rules/git-safety.md` is followed with attribution
- [ ] Feature code integrated (`internal/*`), not test-only
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
- [ ] Functional `.ci` tests for end-to-end behavior, or the recorded N/A reason
- [ ] Interop tests for protocol features (or N-A with a reason)
- [ ] Red evidence table filled: each new test shown failing with its own fix reverted

### Closure
- [ ] Status set to `verification` after the implementation commit, and the session stops (Handoff `verify`)
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
