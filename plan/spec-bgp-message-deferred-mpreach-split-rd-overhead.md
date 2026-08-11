# Spec: MP_REACH splitter counts the VPN Route Distinguisher in its per-chunk overhead

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/fixit-mpreach-split-undercounts-rd.md` |
| Handoff | verify |
| Updated | 2026-08-11 |

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
| `message` package ↔ `core/bgp/attribute` | `SplitMPReachNLRIWithAddPath` asks the attribute for its own encoded size instead of re-deriving it | No |
| Reactor ↔ `message` splitter | `Splitter.Split` / `SplitCompliant` with the peer's maximum message size | No |
| ze ↔ peer | the emitted UPDATE on the wire, which must stay inside the negotiated maximum | No |

### Integration Points
- `(*MPReachNLRI).Len()` - already exported, already the size the writer uses. The splitter subtracts `len(mp.NLRI)` from it to get the overhead.
- `ChunkMPNLRI` - unchanged; it receives a smaller NLRI space under SAFI 128.
- `emitMPChunk` - unchanged; its reserved-region invariant holds again once the chunk is inside the budget.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `(*MPReachNLRI).Len()` counts exactly the octets `WriteTo` writes | `internal/core/bgp/attribute/mpnlri.go`: `Len`, `nextHopLen`, `nextHopOctets` and `WriteTo` share `nextHopOctets` | the new overhead is wrong for some SAFI and chunks are mis-sized in the other direction | `TestSplitUpdate_VPNChunksFitMaxMessageSize` measures the ENCODED chunk, not a re-derived number | unvalidated |
| A-2 | Each chunk carries the parent's AFI, SAFI and next hops, so one overhead figure is right for every chunk | `SplitMPReachNLRIWithAddPath` builds each chunk with `attribute.NewMPReachNLRI` from the parent's fields | one chunk could need more overhead than the budget allowed | the size assertion runs over EVERY chunk, not the first | unvalidated |
| A-3 | The splitter is the only remaining site that derives an MP_REACH next-hop size from the address family | grep of `Is4()` over `internal/component/bgp` and `internal/core/bgp`: the one size derivation is the loop in `SplitMPReachNLRIWithAddPath` | the same defect stays live on another rail | re-run the grep at implementation time and record the hits in the commit message | unvalidated |
| A-4 | No existing test pins a VPN chunk count that the smaller NLRI space changes | `TestSplitMPReachNLRI_VPN` asserts chunk count greater than one and NLRI preservation only | an existing test goes red and the fix looks like a regression | run `make ze-test-pkg PKG=./internal/component/bgp/message` before and after the edit | unvalidated |

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
| Who else touches this path? | `internal/component/bgp/reactor/forward_body.go` and `internal/component/bgp/reactor/peer_send.go` call the splitter but are not edited here. The deferral shard `plan/deferrals/fixit-mpreach-split-undercounts-rd.md` holds this issue and no other session owns it |

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
| `TestSplitMPReachNLRI_VPNChunkFitsMaxAttrSize` | `internal/component/bgp/message/update_split_test.go` | AC-1: every chunk of a SAFI 128 split with an IPv4 next hop encodes within `maxAttrSize`, NLRI preserved | |
| `TestSplitMPReachNLRI_VPNIPv6NextHopChunkFitsMaxAttrSize` | `internal/component/bgp/message/update_split_test.go` | AC-2: the same with a 24-octet VPN-IPv6 next hop | |
| `TestSplitUpdate_VPNChunksFitMaxMessageSize` | `internal/component/bgp/message/update_split_test.go` | AC-3: `Splitter.Split` at a 4096-octet ceiling emits only UPDATEs that fit it | |
| `TestSplitMPReachNLRI_VPNOverheadTooLargeCountsRD` | `internal/component/bgp/message/update_split_test.go` | AC-5: the overhead guard counts the RD | |
| `TestSplitMPReachNLRI_Overflow`, `TestSplitMPReachNLRI_VPN`, `TestSplitUpdate_FlowSpec_Split`, `TestSplitUpdateWithAddPath_IPv6` (existing, unedited) | `internal/component/bgp/message/update_split_test.go` | AC-4: non-VPN behavior unchanged, VPN NLRI still preserved | |
| `TestSplitUpdateWithMPPreservesIPv4Fields`, `TestSplitUpdateWithMPEmitsOneFieldPerChunk` (existing, unedited) | `internal/component/bgp/message/update_split_mixed_test.go` | AC-4: the mixed-shape rail is unchanged | |

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
| N-A, not applicable: no `.ci` can reach this path today | `test/` | A `.ci` would have to make ze re-split a SAFI 128 UPDATE, which needs an ingress UPDATE larger than the destination's ceiling. The bulk injector that builds such an UPDATE, `option=update:value=send-bulk` (`internal/test/peer/expect.go`), emits IPv4 unicast prefixes only, and the announce rail's builders never hand the splitter an UPDATE above the peer's own maximum. Extending the injector to MP and VPN NLRI is test infrastructure, not this fix; it is recorded under Known Limitations. `TestSplitUpdate_VPNChunksFitMaxMessageSize` drives the same entry point the daemon calls, `Splitter.Split`, with the same 4096-octet ceiling | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `23-vpn-frr` (existing, run as a regression check) | `test/interop/scenarios/` | FRR | The VPNv4 encode rail still produces UPDATEs FRR accepts, with the RD intact and the session stable. Command: `make ze-interop-test INTEROP_SCENARIO=23-vpn-frr` | |
| No new scenario | `test/interop/scenarios/` | - | The change IS wire-visible: chunk boundaries move under SAFI 128, and today an over-size UPDATE would draw a NOTIFICATION from the peer. A scenario that reached it would have to negotiate RFC 8654 extended messages on the ingress session and a 4096-octet ceiling on the egress one. No scenario in `test/interop/scenarios/` negotiates extended messages, and the peer-side injector cannot yet build a VPN UPDATE above 4096 octets, so a scenario written today would split nothing and pass with the defect in place. That is the vacuity trap `ai/rules/interop-and-goal-validation.md` names, so the proof is placed at the splitter entry point instead | |

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
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/update-building.md` carries a source anchor over `update_split.go` naming `Splitter` and `Split`. Re-read that anchor's claims after the edit and confirm each still holds; the expected outcome is no edit |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No example shows split arithmetic |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the entry point reaches the defect before changing it
   - Tests: `TestSplitUpdate_VPNChunksFitMaxMessageSize`, `TestSplitMPReachNLRI_VPNChunkFitsMaxAttrSize`
   - Files: `internal/component/bgp/message/update_split_test.go`
   - Verify: run `make ze-test-pkg PKG=./internal/component/bgp/message`. Both tests MUST FAIL against the unchanged splitter, and the failure MUST be the size assertion: a chunk `Len()` above the `maxAttrSize` given (47 against 40 for the fixture in the TDD table), and an emitted `Update.Len(nil)` above 4096. Paste that output. A failure for any other reason (a parse error, a chunk count, a panic) means the fixture is wrong, not the code: fix the fixture and repeat
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
   - Tests: `make ze-test-pkg PKG=./internal/component/bgp/message`, then `make ze-lint-changed`
   - Files: the two files above, plus this spec and the deferral shard row
   - Verify: prepare the commit with `scripts/dev/commit_helper.py create`, run the script it prints, then `make ze-tracked-build-check` because the commit carries Go. Set this spec's Status to `verification` in the same commit and STOP: `Handoff | verify` gives the close to a later Opus 5 session

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
| Four new tests exist and run | `make ze-test-pkg PKG=./internal/component/bgp/message` names each of them in a `-run` filter and passes |
| The chunk-size property holds at the daemon's own entry point | `TestSplitUpdate_VPNChunksFitMaxMessageSize` asserts `Update.Len(nil)` at or below 4096 for EVERY emitted chunk |
| Non-VPN behavior unchanged | The existing split tests pass unedited; no assertion in them is relaxed |
| The VPN encode rail still interoperates | `make ze-interop-test INTEROP_SCENARIO=23-vpn-frr` passes |
| The deferral row is resolved | `plan/deferrals/fixit-mpreach-split-undercounts-rd.md` names this spec as its Destination |

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
| The overhead derivation in `SplitMPReachNLRIWithAddPath` | RFC 4760 Section 3 for the attribute layout, and RFC 4364 Section 4.3.4 for the 8-octet RD in front of a VPN next hop, stating that the count comes from the attribute so the two can never disagree |
| `TestSplitUpdate_VPNChunksFitMaxMessageSize` | The tag `// RFC requirement: RFC8654-4-2 positive` followed by what the test holds, in the form `scripts/dev/rfc_tagged_scope.py` reads. The requirement is already listed in `rfc/short/rfc8654.md` and already carries both polarities over the builder rail; this adds the splitter rail as evidence and removes no kind, so the evidence ratchet is unaffected |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes, or the scoped gates plus an attribution are recorded per `ai/rules/git-safety.md` for a shared checkout
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
- [ ] Functional `.ci` tests for end-to-end behavior, or the recorded reason why none can reach this path
- [ ] Interop tests for protocol features: `23-vpn-frr` re-run, no new scenario, reason recorded

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
