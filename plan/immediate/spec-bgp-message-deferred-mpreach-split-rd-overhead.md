# Spec: MP_REACH splitter counts the VPN Route Distinguisher in its per-chunk overhead

| Field | Value |
|-------|-------|
| Status | done |
| Scope | protocol |
| Depends | - |
| Phase | 4/4 |
| Handoff | verify |
| Updated | 2026-09-05 |

<!-- Handoff `verify`: the implementation session commits its work, sets Status to
     `verification`, and stops. A later Opus 5 session reviews that commit and closes. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`SplitMPReachNLRIWithAddPath` (`internal/component/bgp/message/update_split.go`) computes
the per-chunk next-hop overhead with its own loop over the next hops: four octets for an
IPv4 address, sixteen for anything else. That loop has no Route Distinguisher term.

The single authority on the encoded next-hop size is `(*MPReachNLRI).nextHopOctets`
(`internal/core/bgp/attribute/mpnlri.go`), which returns `RDSize + n` when the SAFI is
`SAFIVPN` (128). RFC 4364 Section 4.3.4 puts an 8-octet Route Distinguisher, set to zero,
in front of the address in a VPN next hop.

So under SAFI 128 the splitter under-states the overhead by 8 octets for each next hop. It
then over-states the NLRI space each chunk can hold by the same amount, and can return a
chunk whose encoded attribute is larger than the `maxAttrSize` it was given. On the forward
rail that chunk becomes an UPDATE larger than the destination peer's maximum message size,
which RFC 8654 requirement RFC8654-4-2 forbids.

Goal: the splitter derives the overhead from the attribute's own arithmetic, so a size
query and a write can never come to different answers, for every SAFI.

## Required Reading

<!-- NEVER tick [ ] to [x]. -->

### Architecture Docs
- [ ] `docs/architecture/update-building.md` - the scratch contract shared by `UpdateBuilder` and `Splitter`, and the source anchor over `update_split.go`
  → Constraint: the splitter writes each chunk into a bounded scratch buffer reserved from the same budget; a chunk larger than the budget can reach past the region reserved for it.
  → Decision: the doc describes the scratch contract and the split entry points, not the overhead arithmetic, so this fix changes no statement in it.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4364.md` - VPN next-hop encoding
  → Constraint: Section 4.3.4 encodes the VPN next hop as an RD of 8 zero octets followed by the IP address, so a VPN-IPv4 next hop is 12 octets and a VPN-IPv6 next hop 24.
- [ ] `rfc/short/rfc4760.md` - MP_REACH_NLRI wire format
  → Constraint: the attribute value is AFI(2) + SAFI(1) + Length of Next Hop(1) + next hops + Reserved(1) + NLRI, so every octet not in the NLRI is per-chunk overhead.
- [ ] `rfc/short/rfc8654.md` - message size ceiling
  → Constraint: RFC8654-4-2 requires a built message to stay inside the maximum message size the session negotiated. Both polarities are already tagged over `BuildUnicastWithMaxSize` in `internal/component/bgp/message/update_build_test.go`; the splitter is the second producer of the same obligation.

**Key insights:** (minimal context to resume after compaction)
- `(*MPReachNLRI).Len()` is exported and returns `2 + 1 + 1 + nhLen + 1 + len(m.NLRI)`, where `nhLen` sums `nextHopOctets` over the next hops. `Len() - len(NLRI)` is therefore the exact overhead for every SAFI, and it needs no new exported API.
- One site in the BGP packages still derives a next-hop size from the address family: the loop inside `SplitMPReachNLRIWithAddPath`. A grep of `Is4()` over `internal/component/bgp` and `internal/core/bgp` returns no other size derivation.
- The splitter's MP path is reached with a real split from the forward rail (`fwdSplitParsedUpdate`, `internal/component/bgp/reactor/forward_body.go`), where an UPDATE received on a session with a larger maximum message size is re-chunked for a destination with a smaller one.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/message/update_split.go` - `SplitMPReachNLRIWithAddPath` sums 4 or 16 octets per next hop from `nh.Is4()`, adds `2 + 1 + 1 + nhLen + 1`, refuses when that total is at or above `maxAttrSize`, and passes the remainder to `ChunkMPNLRI` as the NLRI space. `splitUpdateWithMP` reserves a chunk region of `maxMPAttrValue` bytes in the scratch buffer and parks the higher-coded base attributes above it; `emitMPChunk` writes each chunk at that reserved offset.
- [ ] `internal/core/bgp/attribute/mpnlri.go` - `(*MPReachNLRI).Len`, `nextHopLen` and `nextHopOctets` are the single authority on the encoded size. `nextHopOctets` measures the address with `netip.Addr.AsSlice`, returns 0 for an address with no wire form, and adds `RDSize` (8) when the SAFI is `SAFIVPN`. `WriteTo` writes the RD for the same condition.
- [ ] `internal/component/bgp/reactor/forward_body.go` - `fwdSplitParsedUpdate` calls `Splitter.SplitCompliant` with the destination's maximum message size when `destUpdate.Len(nil)` exceeds it.
- [ ] `internal/component/bgp/reactor/peer_send.go` - `sendUpdateWithSplit` calls `Splitter.Split` with the peer's maximum message size.
- [ ] `internal/component/bgp/message/update_split_test.go` - `TestSplitMPReachNLRI_VPN` is the only VPN split test. It asserts more than one chunk and that the concatenated NLRI equals the input. It asserts nothing about the size of a chunk.

**Behavior to preserve:** every item below is unchanged by this spec.
- The signature and the exported name of `SplitMPReachNLRIWithAddPath`, and the `splitMPReachNLRI` wrapper.
- `ErrMPOverheadTooLarge` returned when the overhead is at or above `maxAttrSize`, with the same wrapped message shape.
- A single chunk, or an empty NLRI, returns the input attribute unchanged rather than a copy.
- Chunk boundaries for every SAFI other than 128, add-path included.
- The concatenated NLRI of the chunks equals the input NLRI, and every chunk carries the input AFI, SAFI and next hops.
- `MP_UNREACH_NLRI` splitting, which carries no next hop and no RD.

**Behavior to change:**
- Under SAFI 128 the per-chunk overhead grows by 8 octets for each next hop, so each chunk carries fewer NLRI bytes and an over-size chunk is no longer produced.
- Under SAFI 128 a `maxAttrSize` between the old and the new overhead now returns `ErrMPOverheadTooLarge` rather than proceeding to a chunking pass that could not encode.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An UPDATE carrying an MP_REACH_NLRI attribute with SAFI 128, reaching `Splitter.Split` or `Splitter.SplitCompliant` with a maximum message size smaller than the UPDATE.
- Two rails produce it: `sendUpdateWithSplit` (`internal/component/bgp/reactor/peer_send.go`) for a locally built announcement, and `fwdSplitParsedUpdate` (`internal/component/bgp/reactor/forward_body.go`) for an UPDATE relayed to a destination whose maximum message size is smaller than the one it arrived under.
- Format at entry: a `*message.Update` whose `PathAttributes` hold the raw MP_REACH_NLRI value.

### Transformation Path
1. `splitByShape` finds the MP attribute ranges in `PathAttributes`.
2. `splitUpdateWithMP` copies the base attributes into scratch, computes `maxMPAttrValue` from the message budget, and reserves that region.
3. `attribute.ParseMPReachNLRI` produces the attribute; `SplitMPReachNLRIWithAddPath` computes the per-chunk overhead and asks `ChunkMPNLRI` for NLRI chunks that fit `maxAttrSize` minus that overhead.
4. `emitMPChunk` writes each chunk into the reserved region with `attribute.WriteAttrToWithLen`, which uses `(*MPReachNLRI).Len` and `WriteTo`, and emits the UPDATE.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `message` package ↔ `core/bgp/attribute` | `SplitMPReachNLRIWithAddPath` asks the attribute for its own encoded size instead of re-deriving it | Yes: the overhead is `mp.Len() - len(mp.NLRI)` (`internal/component/bgp/message/update_split.go`), and no `Is4()` remains in that file |
| Reactor ↔ `message` splitter | `Splitter.Split` / `SplitCompliant` with the peer's maximum message size | Yes: `TestSplitUpdate_VPNChunksFitMaxMessageSize` drives `Splitter.Split` at 4096, and `./internal/component/bgp/reactor` is green |
| ze ↔ peer | the emitted UPDATE on the wire, which must stay inside the negotiated maximum | Yes: every emitted `Update.Len(nil)` is asserted at or below 4096. With the fix reverted, UPDATE 0 measured 4100 |

### Integration Points
- `(*MPReachNLRI).Len()` - already exported, already the size the writer uses. The splitter subtracts `len(mp.NLRI)` from it to get the overhead.
- `ChunkMPNLRI` - unchanged; it receives a smaller NLRI space under SAFI 128.
- `emitMPChunk` - unchanged; its reserved-region invariant holds again once the chunk is inside the budget.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The splitter reads the encoded size through the attribute's own `Len()`. It no longer inspects next-hop addresses at all |
| No unintended coupling (components stay isolated) | Yes | `message` learns nothing about the Route Distinguisher. `RDSize` and `nextHopOctets` stay unexported inside `core/bgp/attribute` |
| No duplicated functionality (extends existing, does not recreate) | Yes | The second size derivation is deleted, not wrapped: `grep -n "Is4()" internal/component/bgp/message/update_split.go` returns nothing |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `Len()` reads fields and allocates nothing, and the chunk path is untouched. `TestSplitter_ZeroAllocAfterWarmup` still passes |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Nothing registers, and the change REMOVES a per-family branch rather than adding one |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `(*MPReachNLRI).Len()` counts exactly the octets `WriteTo` writes | `internal/core/bgp/attribute/mpnlri.go`: `Len`, `nextHopLen`, `nextHopOctets` and `WriteTo` share `nextHopOctets` | the new overhead is wrong for some SAFI and chunks are mis-sized in the other direction | `TestSplitUpdate_VPNChunksFitMaxMessageSize` measures the ENCODED chunk, not a re-derived number | confirmed 2026-08-23: `WriteTo` sets the Length of Next Hop octet from `nextHopLen()`, writes the RD under `SAFIVPN`, skips an address with no wire form, then writes Reserved(1) and the NLRI. Its `pos - off` matches `Len()` term for term |
| A-2 | Each chunk carries the parent's AFI, SAFI and next hops, so one overhead figure is right for every chunk | `SplitMPReachNLRIWithAddPath` builds each chunk with `attribute.NewMPReachNLRI` from the parent's fields | one chunk could need more overhead than the budget allowed | the size assertion runs over EVERY chunk, not the first | confirmed 2026-08-23: each chunk is built by `attribute.NewMPReachNLRI(mp.AFI, mp.SAFI, nhs, chunk)` from the parent's fields, and each new test asserts over every chunk in the returned slice |
| A-3 | The splitter is the only remaining site that derives an MP_REACH next-hop size from the address family | grep of `Is4()` over `internal/component/bgp` and `internal/core/bgp`: the one size derivation is the loop in `SplitMPReachNLRIWithAddPath` | the same defect stays live on another rail | re-run the grep at implementation time and record the hits in the commit message | confirmed 2026-08-23: `grep -rn "Is4()" internal/component/bgp internal/core/bgp` re-run. Every other hit chooses a BRANCH (which attribute to add, which encode path to take, which prefix to match), never a byte count. The one size derivation was the loop in `SplitMPReachNLRIWithAddPath`, and it is gone |
| A-4 | No existing test pins a VPN chunk count that the smaller NLRI space changes | `TestSplitMPReachNLRI_VPN` asserts chunk count greater than one and NLRI preservation only | an existing test goes red and the fix looks like a regression | run `go test -race ./internal/component/bgp/message` before and after the edit | confirmed 2026-08-23: before the edit only the four NEW tests failed; after it the whole package passes, with no unrelated expectation change |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A VPN announcement now needs more UPDATEs, because each chunk carries 8 fewer NLRI octets per next hop | chunk counts rise in the VPN tests | Correct and intended: the old count was reached by over-filling. No mitigation |
| R-2 | A next hop with no wire form contributes 0 octets to `Len()`, so the overhead drops rather than rises | a VPN chunk sized as if it had no next hop | `ValidateNextHops` refuses such an attribute before the announce rails encode it (`internal/core/bgp/attribute/mpnlri.go`); the splitter inherits that refusal and adds nothing |
| R-3 | The `overhead >= maxAttrSize` guard now refuses inputs it accepted, so a caller with a very small `maxAttrSize` sees `ErrMPOverheadTooLarge` where it saw a chunking error before | a test that expected `ErrNLRITooLarge` returns `ErrMPOverheadTooLarge` | Correct: the attribute could not have encoded at that size. AC-5 pins the new boundary |
| R-4 | The fix is written as a second overhead calculation beside the loop | two arithmetic sites in one function | `ai/rules/no-layering.md`: delete the loop, then compute the overhead from `Len()`. A review that finds both is a BLOCKER |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A VPNv4 or VPNv6 UPDATE relayed to a peer with a smaller maximum message size exceeds that maximum. The peer answers with a NOTIFICATION (Message Header Error, Bad Message Length) and the session resets. Inside ze, a chunk larger than the region `splitUpdateWithMP` reserved can also reach the stashed higher-coded base attributes when that stash is shorter than the undercount, which corrupts the attributes of the following chunks |
| How is it reverted? | Single commit revert. No config migration, no state on disk, no peer-visible negotiation |
| Who else touches this path? | `internal/component/bgp/reactor/forward_body.go` and `internal/component/bgp/reactor/peer_send.go` call the splitter but are not edited here. The deferral shard the retired deferral shard "fixit-mpreach-split-undercounts-rd" holds this issue and no other session owns it |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `Splitter.Split` with an over-size SAFI 128 UPDATE at a 4096-octet ceiling (the call `sendUpdateWithSplit` and `fwdSplitParsedUpdate` both make) | → | `splitUpdateWithMP` then `SplitMPReachNLRIWithAddPath` then `emitMPChunk` | `TestSplitUpdate_VPNChunksFitMaxMessageSize` |
| `SplitMPReachNLRIWithAddPath` called directly with a SAFI 128 attribute and an IPv4 next hop | → | the overhead derivation inside `SplitMPReachNLRIWithAddPath` | `TestSplitMPReachNLRI_VPNChunkFitsMaxAttrSize` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A SAFI 128 MP_REACH_NLRI with one IPv4 next hop and NLRI too large for one attribute, split at `maxAttrSize` 40 | Every returned chunk reports `Len()` at or below 40, and the concatenated chunk NLRI equals the input NLRI |
| AC-2 | The same attribute with one IPv6 next hop (AFI 2, SAFI 128, a 24-octet encoded next hop) | Every returned chunk reports `Len()` at or below the `maxAttrSize` given |
| AC-3 | An UPDATE carrying ORIGIN, AS_PATH and a SAFI 128 MP_REACH_NLRI whose NLRI exceeds 4096 octets, passed to `Splitter.Split` with `maxSize` 4096 | Every emitted UPDATE reports `Len(nil)` at or below 4096, and the concatenated MP_REACH NLRI of the emitted UPDATEs equals the input NLRI |
| AC-4 | The same splits for SAFI 1 (IPv6 unicast), SAFI 133 (flowspec) and an add-path IPv6 family | Chunk counts and chunk boundaries are identical to those the current code produces |
| AC-5 | A SAFI 128 MP_REACH_NLRI with one IPv4 next hop (encoded overhead 17 octets), split at `maxAttrSize` 17 | `ErrMPOverheadTooLarge` is returned, and no chunk is produced |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Relays VPNv4 routes through ze to a peer whose maximum message size is 4096 octets, when the routes arrived in one larger UPDATE | wire, `fwdUpdateForDestination`, `fwdSplitParsedUpdate`, `Splitter.SplitCompliant`, `splitUpdateWithMP`, `SplitMPReachNLRIWithAddPath`, `emitMPChunk`, peer | `TestSplitUpdate_VPNChunksFitMaxMessageSize` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSplitMPReachNLRI_VPNChunkFitsMaxAttrSize` | `internal/component/bgp/message/update_split_test.go` | AC-1: every chunk of a SAFI 128 split with an IPv4 next hop encodes within `maxAttrSize`, NLRI preserved | passing; red with the fix reverted (chunk 0 encoded 47 against a 40-octet budget) |
| `TestSplitMPReachNLRI_VPNIPv6NextHopChunkFitsMaxAttrSize` | `internal/component/bgp/message/update_split_test.go` | AC-2: the same with a 24-octet VPN-IPv6 next hop | passing; red with the fix reverted (chunk 0 encoded 69 against a 64-octet budget) |
| `TestSplitUpdate_VPNChunksFitMaxMessageSize` | `internal/component/bgp/message/update_split_test.go` | AC-3: `Splitter.Split` at a 4096-octet ceiling emits only UPDATEs that fit it | passing; red with the fix reverted (UPDATE 0 measured 4100 octets) |
| `TestSplitMPReachNLRI_VPNOverheadTooLargeCountsRD` | `internal/component/bgp/message/update_split_test.go` | AC-5: the overhead guard counts the RD, and the Boundary Tests row (17, 31, 32) | passing; red with the fix reverted, and red again under a separate mutation that subtracts one from the new overhead |
| `TestSplitMPReachNLRI_Overflow`, `TestSplitMPReachNLRI_VPN`, `TestSplitUpdate_FlowSpec_Split`, `TestSplitUpdateWithAddPath_IPv6` (existing, unedited) | `internal/component/bgp/message/update_split_test.go` | AC-4: non-VPN behavior unchanged, VPN NLRI still preserved | passing, unedited |
| `TestSplitUpdateWithMPPreservesIPv4Fields`, `TestSplitUpdateWithMPEmitsOneFieldPerChunk` (existing, unedited) | `internal/component/bgp/message/update_split_mixed_test.go` | AC-4: the mixed-shape rail is unchanged | passing, unedited |

Fixture arithmetic the new tests rely on, so no number in them is a guess:

| Quantity | Value | Where it comes from |
|----------|-------|---------------------|
| Encoded overhead, SAFI 128, one IPv4 next hop | 17 octets | AFI(2) + SAFI(1) + NH_Len(1) + RD(8) + IPv4(4) + Reserved(1) |
| Overhead the current loop computes for the same attribute | 9 octets | the loop adds 4 for an IPv4 address and no RD |
| Encoded overhead, SAFI 128, one IPv6 next hop | 29 octets | AFI(2) + SAFI(1) + NH_Len(1) + RD(8) + IPv6(16) + Reserved(1) |
| One VPN NLRI entry for a /24 prefix | 15 octets | length octet + label(3) + RD(8) + prefix(3); the length octet states 112 bits |

At `maxAttrSize` 40 with 15-octet entries: the correct NLRI space is 23 octets, so a chunk
holds one entry and encodes 32 octets. The current code computes an NLRI space of 31
octets, packs two entries, and encodes 47 octets, which is 7 octets over the ceiling it was
given. That gap is what makes the test discriminate.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `maxAttrSize`, SAFI 128, one IPv4 next hop, 15-octet VPN NLRI entries | 18..65535 | 32 is the smallest value that returns chunks, each encoding 32 octets | 17 and below: `ErrMPOverheadTooLarge`, because the overhead alone fills the budget | 31: the overhead fits but one NLRI entry does not, so `ErrNLRITooLarge` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A, not applicable: no `.ci` can reach this path today | `test/` | A `.ci` would have to the retired make ze-build (current: go build -o bin/ze ./cmd/ze) re-split a SAFI 128 UPDATE, which needs an ingress UPDATE larger than the destination's ceiling. The bulk injector that builds such an UPDATE, `option=update:value=send-bulk` (`internal/test/peer/expect.go`), emits unicast prefixes only, and the announce rail's builders never hand the splitter an UPDATE above the peer's own maximum. Extending the injector to VPN NLRI is test infrastructure, not this fix; it is recorded under Known Limitations. `TestSplitUpdate_VPNChunksFitMaxMessageSize` drives the same entry point the daemon calls, `Splitter.Split`, with the same 4096-octet ceiling | confirmed 2026-08-23, with one correction to the reason. `parseBulkSpec` derives the family from the prefix (`internal/test/peer/inject.go`), so the injector reaches IPv6 unicast through MP_REACH too (`buildV6Unicast`, AFI 2 SAFI 1), not IPv4 alone. It has no SAFI 128 path, so the conclusion is unchanged: no `.ci` reaches a VPN split |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-vpn-frr` (existing, run as a regression check) | `test/interop/scenarios/` | FRR | The VPNv4 encode rail still produces UPDATEs FRR accepts, with the RD intact and the session stable. Command: `./le integration interop INTEROP_SCENARIO=bgp-vpn-frr` | PASS 2026-08-23: session Established, ipv4/vpn negotiated, 10.99.0.0/24 and 10.99.1.0/24 present at FRR, RD 65001:100 verified, session stable |
| No new scenario | `test/interop/scenarios/` | - | The change IS wire-visible: chunk boundaries move under SAFI 128, and today an over-size UPDATE would draw a NOTIFICATION from the peer. A scenario that reached it would have to negotiate RFC 8654 extended messages on the ingress session and a 4096-octet ceiling on the egress one. No scenario in `test/interop/scenarios/` negotiates extended messages, and the peer-side injector cannot yet build a VPN UPDATE above 4096 octets, so a scenario written today would split nothing and pass with the defect in place. That is the vacuity trap `ai/rules/interop-and-goal-validation.md` names, so the proof is placed at the splitter entry point instead | confirmed 2026-08-23: the injector has no SAFI 128 path (`internal/test/peer/inject.go` builds IPv4 unicast or AFI 2 SAFI 1), so no new scenario is written |

## Files to Modify
- `internal/component/bgp/message/update_split.go` - `SplitMPReachNLRIWithAddPath`: delete the next-hop loop, derive the overhead from the attribute, and update the comment above the calculation to name the RD and RFC 4364 Section 4.3.4
- `internal/component/bgp/message/update_split_test.go` - the four new tests

## Files to Create
- None. Both surfaces already exist

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config surface changes; the fix is arithmetic inside the encoder |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | N-A | No command changes |
| CLI grammar (keyword before value) | N-A | No command changes |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | N-A | No new RPC or API; see the Functional Tests row for why no `.ci` reaches this path |
| Pipe completeness | N-A | No command output |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module, binary or certificate |
| Prometheus counters/metrics | No | The splitter exports no metric today, and a counter for a defect that is being removed would have no reader |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new SAFI, capability or attribute. SAFI 128 already exists and its codec is unchanged |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | A defect fix inside the encoder; no feature to list in `docs/features.md` |
| 2 | Config syntax changed? | No | No config surface |
| 3 | CLI command added/changed? | No | No command surface |
| 4 | API/RPC added/changed? | No | The exported signature of `SplitMPReachNLRIWithAddPath` is unchanged |
| 5 | Plugin added/changed? | No | No plugin touched |
| 6 | Has a user guide page? | No | No operator-visible behavior change beyond messages that now fit |
| 7 | Wire format changed? | No | The MP_REACH_NLRI format is unchanged. Only where a chunk boundary falls changes, and `docs/architecture/wire/update-packing.md` states no chunk arithmetic (checked by grep for "overhead" and "NH_Len") |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | No `rfc/short/` edit is owed: RFC8654-4-2 is already a listed requirement with both polarities, and RFC 4364's requirements stay `{not-applicable}`. The new size test carries the tag `// RFC requirement: RFC8654-4-2 positive` so the splitter rail is named as evidence beside the builder rail. `docs/features/rfc-status.md` needs no row change, because no Status and no gap count moves |
| 10 | Test infrastructure changed? | No | The tests use the existing package harness |
| 11 | Affects daemon comparison? | No | No feature added or removed |
| 12 | Internal architecture changed? | No | `docs/architecture/update-building.md` describes the scratch contract and the split entry points, neither of which moves |
| 13 | Route metadata keys added/changed? | N-A | No metadata |
| 14 | Prometheus counters added/changed? | N-A | No counter |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registers |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes, and neither doc needs an edit | `docs/architecture/update-building.md` carries a source anchor over `update_split.go` naming `Splitter` and `Split`; re-read after the edit, every claim still holds, because the entry points and the scratch contract do not move. `docs/architecture/wire/messages.md` is the `// Design:` header of `update_split.go`; it describes the BGP message types and states nothing about splitting, per-chunk overhead, the Length of Next Hop Network Address field or SAFI 128, so no statement in it changes |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No example shows split arithmetic |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the entry point reaches the defect before changing it
   - Tests: `TestSplitUpdate_VPNChunksFitMaxMessageSize`, `TestSplitMPReachNLRI_VPNChunkFitsMaxAttrSize`
   - Files: `internal/component/bgp/message/update_split_test.go`
   - Verify: run `go test -race ./internal/component/bgp/message`. Both tests MUST FAIL against the unchanged splitter, and the failure MUST be the size assertion: a chunk `Len()` above the `maxAttrSize` given (47 against 40 for the fixture in the TDD table), and an emitted `Update.Len(nil)` above 4096. Paste that output. A failure for any other reason (a parse error, a chunk count, a panic) means the fixture is wrong, not the code: fix the fixture and repeat
2. **Phase: Overhead derivation** -- one source for the encoded size
   - Tests: the two above, plus `TestSplitMPReachNLRI_VPNIPv6NextHopChunkFitsMaxAttrSize` and `TestSplitMPReachNLRI_VPNOverheadTooLargeCountsRD`
   - Files: `internal/component/bgp/message/update_split.go`
   - Do: DELETE the `nh.Is4()` loop and the `2 + 1 + 1 + nhLen + 1` expression, then set the overhead to the attribute's own encoded length minus its NLRI length. `ai/rules/no-layering.md`: the loop goes first, and no second calculation stays beside the new one. Update the comment above it to state the RD term and cite RFC 4364 Section 4.3.4
   - Verify: all four new tests pass, the whole package passes, and the existing VPN, flowspec and add-path tests are untouched and green
3. **Phase: Discrimination proof** -- show the tests would catch the defect again
   - Tests: `TestSplitUpdate_VPNChunksFitMaxMessageSize`
   - Files: none. Revert the one-line change in the working tree, re-run the package, capture the red, restore the fix, re-run the package, capture the green
   - Verify: paste both outputs into the commit message. `ai/rules/interop-and-goal-validation.md` requires the reverted-red evidence, and a test that stays green with the fix reverted is worth nothing
4. **Phase: Land it** -- gates, commit, stop
   - Tests: `go test -race ./internal/component/bgp/message`, then `./le changed scope`
   - Files: the two files above, plus this spec and the deferral shard row
   - Verify: prepare the commit with `internal/le/commit/prepare.go create`, run the script it prints, then `./le repository tracked-build check` because the commit carries Go. Set this spec's Status to `verification` in the same commit and STOP: `Handoff | verify` gives the close to a later Opus 5 session

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-5 each name a test in the TDD table, and each test exists and runs |
| No layering | The `Is4()` loop is GONE from `SplitMPReachNLRIWithAddPath`. Two overhead calculations in one function is a BLOCKER, not a cleanup note |
| Correctness | The overhead is the attribute's encoded length minus its NLRI length, so it stays right for a family added later with another next-hop rule |
| Data flow | The `message` package asks `attribute` for the size; it does not learn what an RD is |
| Discrimination | The new size test fails with the fix reverted, and the pasted red names a size, not a parse error |
| Rule: `ai/rules/stale-comments.md` | The comment above the calculation no longer implies a family-derived next-hop size |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The family-derived loop is gone | `grep -n "Is4()" internal/component/bgp/message/update_split.go` returns nothing |
| Four new tests exist and run | `go test -race ./internal/component/bgp/message` names each of them in a `-run` filter and passes |
| The chunk-size property holds at the daemon's own entry point | `TestSplitUpdate_VPNChunksFitMaxMessageSize` asserts `Update.Len(nil)` at or below 4096 for EVERY emitted chunk |
| Non-VPN behavior unchanged | The existing split tests pass unedited; no assertion in them is relaxed |
| The VPN encode rail still interoperates | `./le integration interop INTEROP_SCENARIO=bgp-vpn-frr` passes |
| The deferral row is resolved | the retired deferral shard "fixit-mpreach-split-undercounts-rd" names this spec as its Destination |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The next hops and the SAFI come from a peer's UPDATE on the forward rail. The new overhead is bounded by the attribute's own encoded length, which `ParseMPReachNLRI` already bounded, so no attacker-chosen value can make the overhead negative or unbounded |
| Resource exhaustion | A larger overhead means more chunks for the same NLRI. The count stays bounded by the NLRI length divided by the smallest entry, which is the bound that already applied |
| Memory safety | An over-size chunk could reach past the region `splitUpdateWithMP` reserved and into the stashed base attributes. The fix restores that invariant; the review MUST confirm no chunk can exceed `maxMPAttrValue` |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| A new test fails for a reason other than the size assertion | The fixture is wrong. Recompute it from the TDD arithmetic table |
| An existing VPN test goes red | Read its assertion. If it pinned a chunk count that the smaller NLRI space changed, the count was reached by over-filling: correct the number and say so in the commit message. Never relax a size assertion |
| Lint failure | Fix inline |
| The reverted-fix run stays green | The test does not discriminate. Return to Phase 1 and boundary the fixture per `ai/rules/interop-and-goal-validation.md` |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- A size query and a write that derive the same number twice will disagree the moment a family adds a term. `nextHopOctets` was made the single authority for exactly this reason; the splitter was the last reader that had not been moved onto it.
- The exported `Len()` already carries the whole answer, so the fix needs no new exported API and no new argument.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Derive the overhead as the attribute's encoded length minus its NLRI length | Add an RD term to the existing loop when the SAFI is 128 | The loop would still be a second copy of `nextHopOctets`, and the next family with its own next-hop rule would break it again. Subtracting the NLRI from `Len()` is exact for every SAFI |
| Keep the calculation inside `SplitMPReachNLRIWithAddPath` | Export a next-hop overhead method on `MPReachNLRI` | A new exported method for one caller is machinery the problem does not need (`ai/rules/simplicity.md`). `Len()` is already exported and already correct |
| Prove the property at `Splitter.Split` as well as at the helper | Test the helper only | A guard is driven from its entry point, not from the helper alone (`ai/rules/evidence.md`). The daemon calls `Split`, and the reserved-region invariant lives there |

## Known Limitations
- No `.ci` and no interop scenario exercises an MP split, because no test surface builds a SAFI 128 UPDATE larger than the destination's maximum message size. Closing that needs an MP and VPN mode in the peer injector (`option=update:value=send-bulk`, `internal/test/peer/expect.go`) and an interop scenario that negotiates RFC 8654 extended messages on one session only. That is test infrastructure with its own spec, and it is NOT a deferral of any acceptance criterion in this one: AC-1..AC-5 are each proven by a test named above.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.

| Site | Comment to carry |
|------|------------------|
| The overhead derivation in `SplitMPReachNLRIWithAddPath` | RFC 4760 Section 3 for the attribute layout, and for the 8-octet RD in front of a VPN next hop, RFC 4364 Section **4.3.2** and RFC 4659 Section 3.2.1.1, stating that the count comes from the attribute so the two can never disagree. Correction 2026-08-23: this row said Section 4.3.4, which is "How VPN-IPv4 NLRI Is Carried in BGP" and states the NLRI encoding, not the next hop. Section 4.3.2 is the one that says a VPN-IPv4 next hop "is encoded as a VPN-IPv4 address with an RD of 0". Ten existing comments carry the same wrong section and are recorded in `plan/journal/reference-checked-claim-unchecked.md` |
| `TestSplitUpdate_VPNChunksFitMaxMessageSize` | The tag `// RFC requirement: RFC8654-4-2 positive` followed by what the test holds, in the form `internal/le/` reads. The requirement is already listed in `rfc/short/rfc8654.md` and already carries both polarities over the builder rail; this adds the splitter rail as evidence and removes no kind, so the evidence ratchet is unaffected |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes, or the scoped gates plus an attribution are recorded per `ai/rules/git-safety.md` for a shared checkout
- [ ] Feature code integrated (`internal/*`), not test-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior, or the recorded reason why none can reach this path
- [ ] Interop tests for protocol features: `bgp-vpn-frr` re-run, no new scenario, reason recorded

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- `SplitMPReachNLRIWithAddPath` (`internal/component/bgp/message/update_split.go`) no longer
  derives the per-chunk next-hop size from the address family. The `nh.Is4()` loop and the
  `2 + 1 + 1 + nhLen + 1` expression are deleted, and the overhead is now
  `mp.Len() - len(mp.NLRI)`: the attribute's own encoded size minus its NLRI. The comment
  above it cites RFC 4760 Section 3, RFC 4364 Section 4.3.2 and RFC 4659 Section 3.2.1.1.
- Four tests in `internal/component/bgp/message/update_split_test.go`, each measuring the
  ENCODED size rather than a re-derived number, plus two NLRI fixture builders
  (`vpnIPv4NLRIEntries`, `vpnIPv6NLRIEntries`).
- `rfc/requirements/rfc8654.md` regained `TestSplitUpdate_VPNChunksFitMaxMessageSize` as a
  positive carrier of RFC8654-4-2 (generated by `./le rfc index-update`).
- `rfc/discrimination/rfc8654.json` records the proof for the tag this spec added. It was
  MISSING from the handoff commit and the Review Gate found it; see the Mistake Log.

### Bugs Found/Fixed
- The defect the spec exists for: under SAFI 128 the splitter understated the per-chunk
  overhead by 8 octets for each next hop, so a chunk could encode past the `maxAttrSize` it
  was given. Covered by `TestSplitMPReachNLRI_VPNChunkFitsMaxAttrSize`,
  `TestSplitMPReachNLRI_VPNIPv6NextHopChunkFitsMaxAttrSize`,
  `TestSplitUpdate_VPNChunksFitMaxMessageSize` and
  `TestSplitMPReachNLRI_VPNOverheadTooLargeCountsRD`.
- The second half of the same defect, found by the implementation and named in the commit
  body: a `netip.Addr` with no wire form is not `Is4`, so the old loop charged 16 octets
  where `WriteTo` writes none. `nextHopOctets` answers 0 for it, so the new expression
  matches the write in that direction too.
- Ten Go comments in the tree cite RFC 4364 Section 4.3.4 as the authority for the RD in
  front of a VPN NEXT HOP. That section covers the NLRI encoding. Recorded as a row in
  `plan/journal/reference-checked-claim-unchecked.md` by the implementation session; the
  four comments this spec adds cite 4.3.2 and RFC 4659 Section 3.2.1.1, which were read in
  `rfc/full/rfc4364.txt` and `rfc/full/rfc4659.txt` at those sections.

### Documentation Updates
- None. Three docs anchor `internal/component/bgp/message/update_split.go`:
  `docs/architecture/update-building.md` (`Splitter`, `Split`, `Splitter.SplitCompliant`),
  `docs/architecture/wire/rfc7606-relay-shape.md` (`Split`, `SplitCompliant`) and
  `docs/architecture/wire/mp-nlri-ordering.md` (UPDATE message splitting). None states the
  per-chunk overhead arithmetic:
  `grep -rn -i "overhead\|NH_Len\|Next Hop" docs/architecture/update-building.md
  docs/architecture/wire/mp-nlri-ordering.md docs/architecture/wire/rfc7606-relay-shape.md
  docs/architecture/wire/update-packing.md` returns four hits, all about next-hop rewrite,
  a route constructor, a `CtxID` field list and an interop anchor. The entry points and the
  scratch contract those pages do describe are unmoved.
- `./le doc check verify` not run: no doc file changed.

### Deviations from Plan
- The spec's Closure checklist asks for `plan/learned/NNN-<name>.md`. The current rule
  (`ai/rules/completion.md`, owner directive 2026-08-10) replaces the learned summary with
  one row in `plan/journal/<class>.md`. The row is in
  `plan/journal/helper-bypassed-by-an-open-coded-copy.md`.
- The spec's Implementation Steps name `internal/le/commit/prepare.go create` and
  `make ze-build`; both are pre-`le`-reorg spellings. The commit went through
  `./le commit create`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Documentation Update Checklist row 9 concluded that adding `// RFC requirement: RFC8654-4-2 positive` cost nothing, because the requirement already carried both polarities and the evidence ratchet was unaffected. | `ai/rules/rfc-compliance.md` requires a discrimination record for a tag you ADD, whatever the requirement already carries. `./le rfc check` could not catch the omission: it only refuses a tag the commit under test adds against `HEAD^`. | Review Gate round 1, checking the diff against `ai/rules/rfc-compliance.md` rather than against the gate. | `./le rfc discriminate-record` wrote `rfc/discrimination/rfc8654.json` with an observed red. Included in commit A. |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The splitter derives the overhead from the attribute's own arithmetic | Done | `SplitMPReachNLRIWithAddPath` (`internal/component/bgp/message/update_split.go`) | `overhead := mp.Len() - len(mp.NLRI)`; `grep -n "Is4()"` on that file returns nothing |
| A size query and a write can never come to different answers, for every SAFI | Done | `(*MPReachNLRI).Len` and `(*MPReachNLRI).WriteTo` (`internal/core/bgp/attribute/mpnlri.go`) | Both sum `nextHopOctets` over `NextHops.Slice()`; `WriteTo`'s `pos - off` matches `Len()` term for term |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestSplitMPReachNLRI_VPNChunkFitsMaxAttrSize` | PASS; red at 47 against 40 with the loop restored |
| AC-2 | Done | `TestSplitMPReachNLRI_VPNIPv6NextHopChunkFitsMaxAttrSize` | PASS; red at 69 against 64 with the loop restored |
| AC-3 | Done | `TestSplitUpdate_VPNChunksFitMaxMessageSize` | PASS; red at 4100 against 4096 with the loop restored |
| AC-4 | Done | `TestSplitMPReachNLRI_Overflow`, `TestSplitMPReachNLRI_VPN`, `TestSplitUpdate_FlowSpec_Split`, `TestSplitUpdateWithAddPath_IPv6`, `TestSplitUpdateWithMPPreservesIPv4Fields`, `TestSplitUpdateWithMPEmitsOneFieldPerChunk` | PASS, unedited: `git show --numstat` reports `229 0` for the test file, so no existing assertion moved |
| AC-5 | Done | `TestSplitMPReachNLRI_VPNOverheadTooLargeCountsRD` | PASS, three subtests at 17, 31 and 32 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestSplitMPReachNLRI_VPNChunkFitsMaxAttrSize` | Done | `internal/component/bgp/message/update_split_test.go` | |
| `TestSplitMPReachNLRI_VPNIPv6NextHopChunkFitsMaxAttrSize` | Done | same file | |
| `TestSplitUpdate_VPNChunksFitMaxMessageSize` | Done | same file | Carries `// RFC requirement: RFC8654-4-2 positive` |
| `TestSplitMPReachNLRI_VPNOverheadTooLargeCountsRD` | Done | same file | Boundary row 17 / 31 / 32 |
| Existing split tests, unedited | Done | `update_split_test.go`, `update_split_mixed_test.go` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/message/update_split.go` | Done | 25 lines changed in the handoff commit |
| `internal/component/bgp/message/update_split_test.go` | Done | 229 added, 0 removed |
| `rfc/discrimination/rfc8654.json` | Changed | Added at closure, not in the plan; owed by `ai/rules/rfc-compliance.md` for the tag the plan DID name |

### Audit Summary
- **Total items:** 14
- **Done:** 13
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (`rfc/discrimination/rfc8654.json`, recorded in Deviations and the Mistake Log)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The splitter never returns a chunk whose encoded attribute is larger than the `maxAttrSize` it was given, for every SAFI | functional, at the daemon's own entry point | `TestSplitUpdate_VPNChunksFitMaxMessageSize` drives `Splitter.Split` at 4096 and asserts `Update.Len(nil)` at or below 4096 for EVERY emitted UPDATE. Independently re-run 2026-09-05: PASS. With the `nh.Is4()` loop restored in the working tree, the same run answers `emitted UPDATE 0 is 4100 octets, past the 4096-octet maximum message size`, and the three other tests redden at 47 against 40, 69 against 64, and on the `ErrMPOverheadTooLarge` chain. The producer was restored byte-identically: `git status --porcelain internal/component/bgp/message/` returns nothing |
| A size query and a write can never come to different answers | data correctness | The overhead is read through `(*MPReachNLRI).Len`, the same function `attribute.WriteAttrToWithLen` uses to size the write. `grep -n "Is4()" internal/component/bgp/message/update_split.go` returns nothing, so the second derivation is gone rather than corrected (`ai/rules/no-layering.md`) |
| An UPDATE relayed to a peer with a smaller maximum message size stays inside RFC 8654's ceiling | RFC conformance, recorded | `rfc/discrimination/rfc8654.json` holds an OBSERVED red for RFC8654-4-2 positive at that test, under `body of SplitMPReachNLRIWithAddPath replaced by panic(...)`. `./le rfc check` reports 130 violations, all pre-existing and none naming rfc8654, rfc4760, rfc4364, rfc4659 or `update_split` |
| The VPN encode rail still interoperates | interop | `INTEROP_SCENARIO=bgp-vpn-frr ./le integration interop \| json` re-run 2026-09-05: `"passed": 1, "failed": 0, "code": 0`, scenario `bgp-vpn-frr` against `quay.io/frrouting/frr:10.3.1` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| A `.ci` or interop scenario that drives a real SAFI 128 split | No test surface can build one. `buildUpdates` (`internal/test/peer/inject.go`) dispatches only to `buildV4Unicast` and `buildV6Unicast`, so the bulk injector has no SAFI 128 path, and no scenario in `test/interop/scenarios/` negotiates RFC 8654 extended messages on one session only. A scenario written today would split nothing and pass with the defect in place, which is the vacuity trap `ai/rules/interop-and-goal-validation.md` names. The proof is placed at `Splitter.Split` instead | none yet: it is test infrastructure (an MP and VPN mode in the peer injector, plus an extended-message interop scenario) and it needs a spec of its own. Recorded in Known Limitations above. It is NOT a deferral of an acceptance criterion: AC-1..AC-5 are each proven by a named passing test |
| Correcting the ten `RFC 4364 Section 4.3.4` next-hop comments across four packages | A sweep that would cost this commit its single focus and its review scope (`ai/rules/rule-precedence.md`) | `plan/journal/reference-checked-claim-unchecked.md`, row dated 2026-08-23 |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bgp-message-deferred-mpreach-split-rd-overhead-zeclose-mpreach.md` |
| `./le spec session review check` | `review_gate: OK (2 code files, clean, hashes match ...)` |
| Rounds | 2. Round 1 found one ISSUE (a missing RFC discrimination record) and two NOTEs; round 2 covered the fix that ISSUE produced and found nothing |
| Reviewer lenses used | logic+wiring, security+edge-cases, RFC conformance. Run inline by the one closure agent, which authored none of the diff under review |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The change adds the tag `// RFC requirement: RFC8654-4-2 positive` and carries no discrimination record, which `ai/rules/rfc-compliance.md` requires in the same change. `./le rfc check` cannot catch it once the tag is in history: it only refuses a tag the commit under test adds against `HEAD^` | `TestSplitUpdate_VPNChunksFitMaxMessageSize` (`internal/component/bgp/message/update_split_test.go`) | `./le rfc discriminate-record id RFC8654-4-2 polarity positive route revert producer internal/component/bgp/message/update_split.go::SplitMPReachNLRIWithAddPath`, which observed the red and wrote `rfc/discrimination/rfc8654.json` |

Two NOTEs, recorded and not fixed: `SplitMPReachNLRIWithAddPath` is exported with no
cross-package caller (pre-existing, the signature is unchanged by this diff, and
`./le repository check` accepts it), and `./le integration interop` prints `Failed: interop`
with no cause (already a row dated 2026-09-01 in `plan/journal/failing-gate-prints-no-cause.md`,
so no duplicate row was written).

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/message/update_split.go` | Yes | in `HEAD` at `07b4d3304`; `git status --porcelain internal/component/bgp/message/` returns nothing, so the tree matches the commit |
| `internal/component/bgp/message/update_split_test.go` | Yes | same |
| `rfc/discrimination/rfc8654.json` | Yes | `git status --porcelain rfc/` lists it as `?? rfc/discrimination/rfc8654.json`; `python3 -m json.tool` reads one record for RFC8654-4-2 |
| `test/interop/scenarios/bgp-vpn-frr/` | Yes | `ls` shows `frr.conf` and `ze.conf` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Every chunk of a SAFI 128 IPv4-next-hop split fits `maxAttrSize` | `go test -race -run TestSplitMPReachNLRI_VPNChunkFitsMaxAttrSize ./internal/component/bgp/message` answers `--- PASS` (2026-09-05) |
| AC-2 | The same for a 24-octet VPN-IPv6 next hop | `--- PASS: TestSplitMPReachNLRI_VPNIPv6NextHopChunkFitsMaxAttrSize` in the same run |
| AC-3 | Every UPDATE `Splitter.Split` emits at 4096 fits 4096 | `--- PASS: TestSplitUpdate_VPNChunksFitMaxMessageSize` in the same run |
| AC-4 | Non-VPN chunk boundaries unchanged | `go test -race -run 'TestSplitMPReachNLRI\|TestSplitUpdate\|TestSplitter\|TestSplitUpdateWithMP\|TestSplitUpdateWithAddPath' ./internal/component/bgp/message` answers `ok ... 1.204s`, and `git show --numstat` reports `229 0` for the test file |
| AC-5 | `ErrMPOverheadTooLarge` at `maxAttrSize` 17, `ErrNLRITooLarge` at 31, chunks of exactly 32 at 32 | `--- PASS: TestSplitMPReachNLRI_VPNOverheadTooLargeCountsRD` with all three subtests PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `Splitter.Split` with an over-size SAFI 128 UPDATE at 4096 | none reachable | Yes, by a unit test at the entry point. `TestSplitUpdate_VPNChunksFitMaxMessageSize` calls `collectChunks`, which calls `Splitter.Split`, the same function `sendUpdateWithSplit` (`internal/component/bgp/reactor/peer_send.go`) and `fwdSplitParsedUpdate` (`internal/component/bgp/reactor/forward_body.go`) call. The stack in the recorded discrimination red shows the real chain: `Splitter.Split`, `splitByShape`, `splitUpdateWithMP`, `SplitMPReachNLRIWithAddPath`. No `.ci` can reach it, because `buildUpdates` (`internal/test/peer/inject.go`) has no SAFI 128 path |
| `SplitMPReachNLRIWithAddPath` called directly with a SAFI 128 attribute | none | Yes: `TestSplitMPReachNLRI_VPNChunkFitsMaxAttrSize` calls `splitMPReachNLRI`, whose whole body is `return SplitMPReachNLRIWithAddPath(mp, maxAttrSize, false)` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `(*MPReachNLRI).Len` returns `2 + 1 + 1 + nhLen + 1 + len(m.NLRI)`, and `WriteTo` writes AFI(2), SAFI(1), NH_Len(1), the RD and address for each next hop through the same `nextHopOctets`, Reserved(1) and the NLRI. Both in `internal/core/bgp/attribute/mpnlri.go`, re-read at the producer 2026-09-05 |
| A-2 | confirmed | Each chunk is built by `attribute.NewMPReachNLRI(mp.AFI, mp.SAFI, nhs, chunk)` from the parent's fields inside `SplitMPReachNLRIWithAddPath`, and every new test asserts over every chunk in the returned slice |
| A-3 | confirmed | `grep -n "Is4()" internal/component/bgp/message/update_split.go` returns nothing. Re-run over `internal/component/bgp` and `internal/core/bgp`: every remaining hit chooses a BRANCH, never a byte count. Spot-checked the closest candidate, `(*UpdateBuilder).BuildVPN` (`internal/component/bgp/message/update_build_vpn.go`), which decides whether to append the legacy NEXT_HOP attribute |
| A-4 | confirmed | `go test -race` over the whole split test set passes with no existing expectation edited; the test file's diff is `229 0` |
| R-1 | accepted, intended | Chunk counts rise under SAFI 128 because the old counts were reached by over-filling |
| R-2 | mitigated | `nextHopOctets` answers 0 for an address with no wire form and `WriteTo` skips it, so the two still agree. `ValidateNextHops` (`internal/core/bgp/attribute/mpnlri.go`) refuses such an attribute on the announce rails, and `parseNextHops` refuses a next-hop length no `ValidNextHopLens` entry admits, so the forward rail cannot produce one either |
| R-3 | accepted, intended | AC-5 pins the new boundary at 17 / 31 / 32 |
| R-4 | held | The loop is deleted, not wrapped. One arithmetic site remains |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Rows 1-8, 10-15 and 17: No or N-A | The change is one expression inside the encoder. No config, CLI, RPC, plugin, SDK, metric, registration or example surface moves, and the exported signature of `SplitMPReachNLRIWithAddPath` is unchanged | Yes |
| Row 9, RFC behavior newly proven: Yes | `rfc/requirements/rfc8654.md` lists `TestSplitUpdate_VPNChunksFitMaxMessageSize` as a positive carrier of RFC8654-4-2. No `rfc/short/` Meta row moves: the requirement already carried both polarities, so no Support status, coverage or remaining count changes, and `docs/features/rfc-status.md` needs no row change. The record the tag owes is now `rfc/discrimination/rfc8654.json` | Yes, with the correction in the Mistake Log |
| Row 16, source anchors over changed files: Yes, no edit owed | `grep -rn "update_split.go" docs/` finds anchors in `docs/architecture/update-building.md` (two), `docs/architecture/wire/rfc7606-relay-shape.md` and `docs/architecture/wire/mp-nlri-ordering.md`. Each names `Splitter`, `Split` or `SplitCompliant`, and none states per-chunk overhead. A grep for `overhead`, `NH_Len` and `Next Hop` across those pages and `docs/architecture/wire/update-packing.md` returns four hits, none about split arithmetic | Yes |

### Gates run at closure
| Gate | Result | Attribution |
|------|--------|-------------|
| `go test -race` over the `./internal/component/bgp/message` split set | PASS | this session |
| `INTEROP_SCENARIO=bgp-vpn-frr ./le integration interop` | PASS (`passed: 1, failed: 0`) | this session |
| `./le rfc check` | 130 violations, all pre-existing | rfc5798 (VRRP, unimplemented), rfc8671 extraction, rfc7606 stale audits, rfc2385 / rfc5082 / rfc6793 stale discrimination records, and `ai/RFC-REQUIREMENTS.md` staleness. None names rfc8654, rfc4760, rfc4364, rfc4659 or `update_split` |
| `./le repository check` | 2 ISSUEs | `ParseNextHop` and `ParseOriginatorID` (`internal/core/bgp/attribute/simple.go`) have no cross-package non-test caller. Another session owns that package, and neither symbol is touched by this spec |
| `./le commit audit` | 4 WEAKENED | `test/encode/new-v6.ci`, `test/plugin/med-locally-set-reaches-peer.ci`, `test/ui/test-announce-forms-are-separate-commands.ci` and `test/ui/test-withdraw-forms-are-separate-commands.ci`. All four are another session's uncommitted work, and this spec changes no `.ci` |
| `./le verify worktree` | not run by this session | A peer's run against the same commit `4e220c76675f` was already in flight under `tmp/verify-worktree/`, started 11:51 and not owned by this session. Both closure commits carry no Go, and `ai/rules/pre-release.md` owes no green gate at a commit. `./le commit create` records the verification-debt row |

## Core Insight

The single-authority fix is not "compute the same number more carefully". It is to
SUBTRACT: the attribute already exports the total it will write, so the overhead is that
total minus the part the caller is about to replace. No new exported method, no new
argument, and a family added later with its own next-hop rule is right by construction.

The corollary the Review Gate found is about gates rather than code. A gate that fires on a
DIFF, such as `./le rfc check` refusing a tag the commit adds against `HEAD^`, cannot see
the same omission once the commit has landed. A two-session handoff walks straight past it.
The closure phase has to check the RULE, not the gate.
