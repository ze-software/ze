# Spec: srv6-bestpath-resolvability -- SRv6 Service SID Resolvability Before Best-Path

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 9252 Section 5 (`rfc/full/rfc9252.txt`, read 2026-09-08):

> "Therefore, the ingress PE MUST perform a resolvability check for the SRv6
> Service SID before considering the received prefix for the BGP best path
> computation. The resolvability is evaluated as per [RFC4271]. If the SRv6 SID
> is reachable via more than one forwarding table, local policy is used to
> determine which table to use."

Section 6 repeats that sentence verbatim for EVPN over SRv6, so the obligation
covers every family that can carry an SRv6 Service SID, not L3VPN alone.

`rfc/short/rfc9252.md` records `RFC9252-5-2` as a `{gap}`. Ze acts as an SRv6
ingress PE: `srv6SIDFromResult` reconstructs the winning path's Service SID
(`internal/component/bgp/plugins/rib/rib_bestchange.go`) and the FIB plugins
install it as a kernel SEG6 encap route
(`internal/plugins/fib/kernel/nexthop_linux.go`) or a VPP SR steering policy
(`internal/plugins/fib/vpp/srv6.go`). No resolvability check runs anywhere in
`internal/component/bgp`: `isSRv6Ineligible` gates candidacy on SID EXTRACTION
validity only.

### The operator-visible defect

The only component that knows reachability is sysrib. `srv6SIDResolvable`
(`internal/component/sysrib/sysrib.go`) asks exactly this question and answers it
correctly, but it runs AFTER both best-path computations: BGP's own, and the
Loc-RIB's cross-protocol arbitration. `fibEntry`
(`internal/component/sysrib/sysrib.go`) then returns "nothing owed" for a prefix
whose SID does not resolve.

So a path with an unresolvable SID still WINS the prefix and is then declined at
the FIB. Two consequences follow, and both are what an operator meets:

| Consequence | Why it happens |
|-------------|----------------|
| The prefix is dark, with no fallback | Losing at `fibEntry` is not losing best-path. The runner-up BGP path never gets the prefix, and a static route at a worse administrative distance stays unprogrammed while the blackholed BGP path holds the Loc-RIB entry |
| The dark path is re-advertised | The winner is the path Ze propagates to its own peers, so Ze tells its neighbours to send it traffic it cannot forward |

The RFC's placement (BEFORE best path computation) is the fix for both: an
unresolvable SID must cost the path its candidacy, not its FIB entry.

### Relationship to RFC9252-5-1

Two requirements, two pieces of work, and this spec is only the second.

| Requirement | Its sentence | Property tested | State today |
|-------------|--------------|-----------------|-------------|
| `RFC9252-5-1` | "The path having any such Prefix-SID attribute without any valid SRv6 SID information MUST be considered ineligible during the selection of the best path for the corresponding prefix" (`rfc/full/rfc9252.txt`, Section 7) | SEMANTIC validity of the SID information carried in the attribute | Implemented by `isSRv6Ineligible`, proven both polarities by `TestIsSRv6Ineligible_ValidSID` and `TestIsSRv6Ineligible_InvalidSID` (`internal/component/bgp/plugins/rib/srv6_ineligible_test.go`) and by `TestSRv6TranspositionWiderThanLabelFieldIsIneligible`, recorded in `rfc/requirements/rfc9252.md` |
| `RFC9252-5-2` | Section 5, quoted above | REACHABILITY of the SID address, "evaluated as per [RFC4271]" | `{gap}` -- this spec |

They share ONE code site: the candidate filter in `gatherCandidatesLocked`
already calls `isSRv6Ineligible`, and this spec's check goes beside it. The
5-1 tests MUST stay green and MUST NOT be edited: a path with a valid but
unreachable SID is 5-1-eligible and 5-2-ineligible, which is precisely the
distinction the new tests have to hold.

### Not in this spec

`internal/component/sysrib` carries no SRv6 test of any kind (`grep -l SRv6
internal/component/sysrib/*_test.go` returns nothing; the only SRv6 test in that
tree is in the separate `internal/component/sysrib/events` package). Four SRv6
branches there are therefore unproven: the `Track`/`Untrack` pair in
`fibimport.go`, the import-time gate in `fibimport.go`, the winner gate in
`fibEntry`, and the `Track`/`Untrack` pair in the winner-change path of
`sysrib.go`. Thomas directed on 2026-09-08 that this coverage is written in the
per-protocol FIB withholding work, not here. This spec MUST NOT claim it and
MUST NOT edit those tests.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component isolation, the seam pattern, sysrib's place
  → Constraint: a component never imports another component; a fact that has to cross registers into a leaf package both sides import
  → Decision: the resolvability oracle is published OUT of sysrib rather than imported IN by BGP, because BGP MUST NOT depend on `internal/component/sysrib` (`./le tier check`)
- [ ] `docs/architecture/plugin/rib-storage-design.md` - declared by `rib_commands.go`, `rib_bestchange.go` and `rib.go`
  → Constraint: candidate gathering runs under `RIBManager.peerMu.RLock`, so anything the filter calls MUST NOT take a BGP lock or re-enter the RIB
- [ ] `docs/architecture/rib/unified-locrib.md` - the shared Loc-RIB, its change notifications
  → Constraint: `locrib.RIB.OnChange` handlers run synchronously under the owning shard's WRITE lock and MUST NOT re-enter Insert/Remove on the same RIB, so a re-evaluation trigger hands off to a worker and does no work inline
- [ ] `docs/features/srv6.md` - the published SRv6 feature description
  → Decision: its "SID resolvability check | 5 | Implemented (Loc-RIB LPM)" row over-claims today, because the check runs at FIB installation and not before best-path; the row is corrected by this work, not lowered (`ai/rules/rfc-compliance.md`)
- [ ] `ai/rules/goroutine-lifecycle.md` - before writing the re-evaluation worker
  → Constraint: one long-lived worker on a channel, started and stopped with the owner; never a goroutine per notification

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9252.md` - `RFC9252-5-1`, `RFC9252-5-2`, and the `## Meta` Support rows
  → Constraint: the `{gap}` annotation on `RFC9252-5-2` and the `Support remaining` paragraph both name this gap, and both are rewritten when the behavior exists
- [ ] `rfc/full/rfc9252.txt` Sections 5, 6 and 7 - the authoritative sentences
  → Constraint: the same MUST appears in Section 5 (L3) and Section 6 (EVPN), so the check binds every family carrying a Service SID
- [ ] `docs/contributing/rfc-conformance-gates.md` - what a new tag owes
  → Constraint: a tag ADDED in this change owes a discrimination record in the SAME change, observed by `./le rfc discriminate-record`, or `./le rfc check` refuses it

**Key insights:**
- The oracle already exists and is a pure function of shared state: `nhResolver.Resolve` (`internal/component/sysrib/nhresolver.go`) reads only `locrib.RIB.LPM`, walks at most `maxRecursionDepth` hops, and stops on a self-referencing route. Nothing else in sysrib feeds it.
- The BGP RIB plugin already holds the Loc-RIB (`RIBManager.locRIB`, wired by `SetLocRIB`) and already registers subscriptions there, so the re-evaluation trigger needs no new wiring point.
- `internal/core/rib/igpcost` is the existing precedent for publishing a sysrib answer to BGP best-path: sysrib calls `igpcost.Set(resolver.IGPMetric)` in `SetLocRIB`, BGP calls `igpcost.Lookup` from `bestpath.go`, and the leaf belongs to neither side.
- The full SID needs the transposition merge for the VPN families, so the check runs on the SID that `srv6SIDFromResult` reconstructs, not on the partial SID `pool.ExtractSRv6SID` returns.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - `gatherCandidatesLocked` walks `r.bgpPeers`, skips a candidate when `isSRv6Ineligible` says so, then skips a self next-hop. This is the only candidate filter and the only non-test caller of `isSRv6Ineligible`
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - `isSRv6Ineligible` reports ineligible only when the Prefix-SID carries SRv6 Service TLVs and `pool.ExtractSRv6SID` yields no valid SID; `srv6SIDFromResult` merges the transposed label bits back into the partial SID for the VPN families; `mirrorToLocRIB` writes the winner into the Loc-RIB and skips non-CIDR families
- [ ] `internal/component/bgp/plugins/rib/rib.go` - `RIBManager.locRIB` holds the shared Loc-RIB, may be nil, and `SetLocRIB` is where subscriptions over it are registered and released
- [ ] `internal/component/sysrib/sysrib.go` - `srv6SIDResolvable` returns `r.Resolve(sid).Resolved`, and permissively returns true when no resolver is wired; `fibEntry` returns "nothing owed" for an unresolvable SID; `SetLocRIB` publishes `igpcost.Set(resolver.IGPMetric)`
- [ ] `internal/component/sysrib/nhresolver.go` - `nhResolver.Resolve` performs the recursive LPM walk over the Loc-RIB and depends on nothing else in sysrib
- [ ] `internal/core/rib/igpcost/igpcost.go` - the seam precedent: a `Func` type, an atomic pointer, `Set` from the producer, `Lookup` from the consumer, and a documented neutral value when unset
- [ ] `internal/core/rib/locrib/change.go` - `ChangeAdd`, `ChangeUpdate` and `ChangeRemove`, dispatched to subscribers
- [ ] `internal/core/rib/locrib/manager.go` - `OnChange` registers a handler into every shard and returns an unsubscribe function; handlers run under the shard write lock
- [ ] `internal/component/bgp/plugins/rib/srv6_ineligible_test.go` - the `RFC9252-5-1` tagged tests that must stay green
- [ ] `docs/features/srv6.md` - publishes "SID resolvability check | 5 | Implemented (Loc-RIB LPM)" and names `IsSRv6Ineligible()` with the wrong case
- [ ] `test/plugin/fib-srv6-kernel.ci` - the existing SRv6 functional test shape: plugin block, fakefib, kernel FIB assertions
- [ ] `test/interop/scenarios/bgp-srv6-frr/ze.conf` - the existing named SRv6 interop scenario, an `ipv6/mpls-vpn` session against FRR

**Behavior to preserve:**
- `isSRv6Ineligible` and its `RFC9252-5-1` verdict, unchanged, including both tagged tests.
- The sysrib check at `fibEntry`, unchanged, as defence in depth: the FIB must never program an encap through an unreachable locator even if the RIB-side check is inactive.
- Permissive behavior when no resolver exists: with no Loc-RIB wired, every path stays a candidate exactly as today.
- The self-next-hop filter and every other candidate rule in `gatherCandidatesLocked`.

**Behavior to change:**
- A path whose reconstructed SRv6 Service SID has no covering route in the Loc-RIB is dropped from best-path candidacy, so the runner-up wins the prefix.
- A change to the Loc-RIB that alters a tracked SID's reachability re-runs best-path for every prefix that depends on it, in both directions.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A BGP UPDATE carrying a Prefix-SID attribute (code 40) with an SRv6 Service TLV, from any peer, in any family the peer negotiated. Format at entry: wire bytes, stored as opaque `OtherAttrs` pool data on the peer's Adj-RIB-In entry.
- A Loc-RIB change for an IPv6 prefix that covers, or stops covering, a tracked SID. Format at entry: a `locrib.Change` delivered to an `OnChange` subscriber.

### Transformation Path
1. UPDATE parsed; the Prefix-SID attribute is interned into the `OtherAttrs` pool and the entry lands in the peer's Adj-RIB-In.
2. `checkBestPathChange` calls `gatherCandidates` for the prefix.
3. `gatherCandidatesLocked` reconstructs each SRv6 candidate's Service SID, records the SID-to-prefix dependency, asks the reachability seam, and skips the candidate when the seam answers "not resolvable".
4. Best-path selection runs over the surviving candidates; the winner is mirrored into the Loc-RIB and emitted to peers and to sysrib.
5. Separately: a Loc-RIB change reaches the BGP subscriber, which enqueues the changed prefix; the worker maps it to the dependent BGP prefixes and re-runs step 2 for each.
6. sysrib's `fibEntry` still asks `srv6SIDResolvable` before programming, and now sees only paths the RIB already admitted.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| sysrib → reachability seam | `Set` from `sysrib.SetLocRIB`, beside the existing `igpcost.Set` | No |
| BGP best-path → reachability seam | `Lookup` from `gatherCandidatesLocked` | No |
| Loc-RIB → BGP RIB | `locrib.RIB.OnChange` handler registered in `RIBManager.SetLocRIB` | No |
| BGP RIB → its own best-path | worker channel to `checkBestPathChange` | No |
| BGP RIB → peers | the winner is what `ribOut` advertises, so an excluded path is never propagated | No |

### Integration Points
- `gatherCandidatesLocked` (`internal/component/bgp/plugins/rib/rib_commands.go`) - the one candidate filter; the new check sits beside the existing `isSRv6Ineligible` call.
- `srv6SIDFromResult` (`internal/component/bgp/plugins/rib/rib_bestchange.go`) - reconstructs the SID the check asks about.
- `RIBManager.SetLocRIB` (`internal/component/bgp/plugins/rib/rib.go`) - where the `OnChange` subscription and the worker start and stop.
- `sysrib.SetLocRIB` (`internal/component/sysrib/sysrib.go`) - where the seam producer registers, one line beside `igpcost.Set`.
- `nhResolver.Resolve` (`internal/component/sysrib/nhresolver.go`) - the oracle, reached through `srv6SIDResolvable`, unchanged.

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
| A-1 | `nhResolver.Resolve` is a pure function of the shared Loc-RIB and needs no other sysrib state | `internal/component/sysrib/nhresolver.go`: the struct holds `rib` plus a tracking map, and `Resolve` reads `r.rib.LPM` alone | The seam would publish an answer that is only correct inside sysrib, and BGP would see a different verdict from the FIB | Read the producer; unit test asserting seam and `srv6SIDResolvable` agree on the same Loc-RIB | unvalidated |
| A-2 | The locator bits of a partial SID can differ from the merged SID, so the check must run on the `srv6SIDFromResult` output | `rfc/full/rfc9252.txt` Section 3.2.1 bounds transposition offset+length only against LBL+LNL+FL+AL, so the window is not forbidden from overlapping the locator | Checking the partial SID would resolve the wrong address for a transposed route and admit or reject the wrong path | Unit test with a transposition window overlapping the locator | unvalidated |
| A-3 | Loc-RIB `OnChange` fires for every change that can alter a SID's resolvability, including the recursive case | `internal/core/rib/locrib/change.go` defines Add/Update/Remove per prefix, and `nhResolver.Resolve` walks the same RIB | A SID could become reachable with no notification, leaving the prefix permanently excluded, which is worse than today | Unit test that inserts a covering route and asserts re-evaluation; the recursive case gets its own test | unvalidated |
| A-4 | A prefix excluded for an unresolvable SID leaves no best-path record, so the dependency index must be written by the FILTER and not by the winner path | `gatherCandidatesLocked` skips the candidate before selection, and `mirrorToLocRIB` only ever sees a winner | Recovery would be impossible: the excluded path would never be reconsidered | Unit test asserting recovery from full exclusion (AC-3) | unvalidated |
| A-5 | The BGP RIB plugin runs in-process for every deployment where the check must be active, and `r.locRIB` is nil only when no Loc-RIB is wired | `internal/core/rib/locrib/default.go` documents `SetLocRIB(locrib.Default())` as "nil-safe; skips mirroring in forked mode" | The check would silently vanish in forked mode with no operator signal | AC-6 test plus the startup log line | unvalidated |
| A-6 | `RFC9252-5-1`'s tests keep passing unchanged, because a valid-but-unreachable SID is a new third case rather than a reclassification | `internal/component/bgp/plugins/rib/srv6_ineligible_test.go` drives `isSRv6Ineligible` directly, which this spec does not modify | The 5-1 proof would need rewording and a fresh discrimination record | Run the package tests before and after | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Lock-order inversion: the `OnChange` handler runs under the Loc-RIB shard write lock, and taking `RIBManager.peerMu` there while `gatherCandidatesLocked` holds `peerMu` and reads the Loc-RIB closes a cycle | A deadlock under concurrent UPDATE and static-route churn; the race detector on the new tests | The handler takes NO BGP lock and only sends to a buffered channel; all re-evaluation happens on the worker |
| R-2 | Re-entrancy: re-running best-path inside the handler calls `locRIB.InsertForward` on the RIB whose shard lock is held, which `OnChange` documents as forbidden | Immediate self-deadlock on the first re-evaluation | Same worker hand-off as R-1 |
| R-3 | A dropped or coalesced-away notification leaves a prefix permanently dark or permanently ineligible, with no red anywhere | A prefix that never recovers after its locator returns | The queue coalesces into a pending SET with a wake signal rather than dropping; a counter records every enqueue and every drain, and a non-zero drop counter is a defect |
| R-4 | Oscillation: a SID whose covering route is itself a BGP prefix makes the re-evaluation feed itself | Rising re-evaluation counters with no configuration change | `nhResolver` already bounds the walk at `maxRecursionDepth` and refuses a self-referencing route; the worker drains a prefix once per pass, and best-path emits only on an actual change |
| R-5 | Unbounded growth of the SID-to-prefix dependency index under a full VPN table | Memory growth proportional to SRv6 prefixes | The index is keyed by SID address, entries are removed when the last candidate carrying that SID leaves, and a test asserts the index empties on withdraw |
| R-6 | The seam is unset in a deployment where it should be set, and every path is silently admitted | No log line and no counter | `Lookup` returns a second value distinguishing "no producer" from "not resolvable"; the plugin logs once at start when the check is inactive |
| R-7 | Concurrent edits to `internal/component/sysrib/sysrib.go` collide with the per-protocol FIB withholding work (`spec-per-protocol-fib-import`, landed 2026-09-09 as `000e70eec` and closed) | A conflicting hunk at commit time | The sysrib edit is ONE line beside `igpcost.Set` in `SetLocRIB`; implement it last and rebase onto the withholding work rather than around it |
| R-8 | Excluding a path from candidacy changes what Ze advertises, so a peer sees a withdraw where it saw a route | An interop scenario showing an unexpected withdraw | That is the required behavior; the interop scenario asserts it explicitly against FRR so the change is proven rather than discovered |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Best-path selection for every family carrying an SRv6 Service SID. A false "not resolvable" withdraws working VPN routes from the FIB and from peers; a deadlock in the notification path stalls every best-path computation in the daemon |
| How is it reverted? | Single commit revert. No config migration, no state format change. Peers see the routes return |
| Who else touches this path? | `spec-per-protocol-fib-import` rewrote `internal/component/sysrib/sysrib.go` and closed on 2026-09-09 (`000e70eec`), so rebase onto it rather than around it; `plan/immediate/spec-fib-depth.md` owns `nhresolver.go`; `plan/immediate/spec-srv6-evpn-label-width.md` and `plan/spec-srv6-labeled-unicast.md` touch the same SRv6 extraction code |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BGP UPDATE with a Prefix-SID SRv6 Service TLV whose locator has no covering route | → | `gatherCandidatesLocked` resolvability filter | `TestGatherCandidatesSkipsUnresolvableSRv6SID` |
| Static route for the locator committed by the operator, reaching the Loc-RIB | → | `RIBManager` `OnChange` handler → re-evaluation worker → `checkBestPathChange` | `TestLocRIBChangeReevaluatesSRv6Prefix` |
| sysrib wiring a Loc-RIB at startup | → | reachability seam `Set` in `sysrib.SetLocRIB` | `TestReachableSeamPublishedBySysrib` |
| Operator commits an unresolvable-SID peer and a static route, over the real daemon | → | the whole chain, ending at the kernel FIB | `test/plugin/srv6-bestpath-static-wins.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A path carries a valid SRv6 Service SID whose address has no covering route in the Loc-RIB | The path is not a best-path candidate: the prefix's best is the next-best path, or the prefix has no best when there is none |
| AC-2 | The same prefix also carries a second path with no SRv6 SID and a worse LOCAL_PREF | The second path wins the prefix and is advertised to peers; the excluded path is not advertised |
| AC-3 | A covering route for a previously unresolvable SID is added to the Loc-RIB | The dependent prefixes are re-evaluated, and the SRv6 path wins if best-path prefers it |
| AC-4 | The covering route for a resolvable SID is removed from the Loc-RIB | The dependent prefixes are re-evaluated, and the SRv6 path loses best-path |
| AC-5 | A path carries a valid SRv6 Service SID whose address resolves | The path is a candidate and best-path is unchanged from today |
| AC-6 | No reachability producer is registered (no Loc-RIB wired) | Every path is admitted exactly as today, and the plugin logs once that the resolvability check is inactive |
| AC-7 | A BGP path with an unresolvable SID and a static route for the same prefix at a worse administrative distance | The static route is programmed into the FIB; today the prefix is dark |
| AC-8 | A path with an SRv6 Service SID whose transposition window overlaps the locator | The check resolves the MERGED SID from `srv6SIDFromResult`, not the partial SID |
| AC-9 | A path with no Prefix-SID attribute, and a path with SRv6 TLVs but no valid SID | Behavior is unchanged: the first is a candidate, the second stays ineligible under `RFC9252-5-1` |
| AC-10 | An EVPN path carrying an SRv6 Service SID whose address does not resolve | The path is not a best-path candidate, because Section 6 repeats the Section 5 MUST |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Receives a VPN route from a PE whose SRv6 locator is not in the IGP | wire → Adj-RIB-In → `gatherCandidatesLocked` filter → runner-up wins → Loc-RIB → sysrib → FIB | `test/plugin/srv6-bestpath-unresolvable.ci` |
| 2 | Adds the missing locator route, and expects the VPN route to take over | config commit → Loc-RIB change → BGP subscriber → worker → best-path → Loc-RIB → FIB | `test/plugin/srv6-bestpath-resolvable.ci` |
| 3 | Keeps a static backup for a prefix an SRv6 PE also advertises, and expects it to carry traffic while the SRv6 path is unusable | static config → Loc-RIB; BGP path excluded → static wins arbitration → kernel route | `test/plugin/srv6-bestpath-static-wins.ci` |
| 4 | Peers with FRR advertising SRv6 L3VPN and expects Ze not to re-advertise a route it cannot forward | FRR UPDATE → Ze best-path exclusion → no advertisement back to the FRR peer | `bgp-srv6-resolvability-frr` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGatherCandidatesSkipsUnresolvableSRv6SID` | `internal/component/bgp/plugins/rib/srv6_resolvability_test.go` | AC-1: the filter drops the candidate | planned |
| `TestGatherCandidatesAdmitsResolvableSRv6SID` | `internal/component/bgp/plugins/rib/srv6_resolvability_test.go` | AC-5: the filter admits the candidate | planned |
| `TestUnresolvableSRv6LosesToNonSRv6Path` | `internal/component/bgp/plugins/rib/srv6_resolvability_test.go` | AC-2: the runner-up wins the prefix | planned |
| `TestLocRIBChangeReevaluatesSRv6Prefix` | `internal/component/bgp/plugins/rib/srv6_resolvability_test.go` | AC-3: recovery after the locator appears | planned |
| `TestLocRIBRemovalReevaluatesSRv6Prefix` | `internal/component/bgp/plugins/rib/srv6_resolvability_test.go` | AC-4: withdrawal after the locator goes | planned |
| `TestResolvabilityCheckInactiveWithoutProducer` | `internal/component/bgp/plugins/rib/srv6_resolvability_test.go` | AC-6: unset seam admits every path and says so | planned |
| `TestResolvabilityUsesMergedTransposedSID` | `internal/component/bgp/plugins/rib/srv6_resolvability_test.go` | AC-8: the merged SID is the address checked | planned |
| `TestEVPNUnresolvableSRv6SIDIneligible` | `internal/component/bgp/plugins/rib/srv6_resolvability_test.go` | AC-10: Section 6 families are covered | planned |
| `TestSRv6DependencyIndexEmptiesOnWithdraw` | `internal/component/bgp/plugins/rib/srv6_resolvability_test.go` | R-5: the index does not grow without bound | planned |
| `TestReachableSeamUnsetIsNotUnresolvable` | `internal/core/rib/reachable/reachable_test.go` | R-6: an unset seam is distinguishable from a negative answer | planned |
| `TestReachableSeamPublishedBySysrib` | `internal/component/sysrib/reachable_seam_test.go` | The producer registers on `SetLocRIB` and clears on nil | planned |
| `TestReachableSeamAgreesWithSysribCheck` | `internal/component/sysrib/reachable_seam_test.go` | A-1: seam and `srv6SIDResolvable` give one answer over one Loc-RIB | planned |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Recursion depth of the locator resolution chain | 1-8 hops (`maxRecursionDepth`) | 8 | N/A | 9 hops resolves to unreachable, so the path is excluded |
| Re-evaluation queue depth | 1-capacity | capacity | N/A | Over capacity coalesces into the pending set, never drops |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `srv6-bestpath-unresolvable` | `test/plugin/srv6-bestpath-unresolvable.ci` | AC-1, AC-2: the route with the unreachable locator does not win and is not advertised | planned |
| `srv6-bestpath-resolvable` | `test/plugin/srv6-bestpath-resolvable.ci` | AC-3: the locator route appears and the SRv6 path takes the prefix | planned |
| `srv6-bestpath-static-wins` | `test/plugin/srv6-bestpath-static-wins.ci` | AC-7: the static backup is programmed instead of a dark prefix | planned |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-srv6-resolvability-frr` | `test/interop/scenarios/bgp-srv6-resolvability-frr` | FRR | FRR advertises an SRv6 L3VPN route whose locator Ze cannot reach; Ze does not select it and does not re-advertise it, and selects it once the locator is reachable. Sibling of the existing `bgp-srv6-frr` scenario | planned |

## Files to Modify
- `internal/component/bgp/plugins/rib/rib_commands.go` - the resolvability filter in `gatherCandidatesLocked`, beside the `isSRv6Ineligible` call
- `internal/component/bgp/plugins/rib/rib_bestchange.go` - reconstruct the merged SID for a candidate and record the SID-to-prefix dependency; the RFC comment quoting Section 5
- `internal/component/bgp/plugins/rib/rib.go` - the `OnChange` subscription, the dependency index field, and the worker's start and stop in `SetLocRIB`
- `internal/component/sysrib/sysrib.go` - one registration in `SetLocRIB`, beside `igpcost.Set`
- `docs/architecture/plugin/rib-storage-design.md` - declared by three of the files above; document the resolvability filter and the re-evaluation worker
- `docs/architecture/core-design.md` - declared by `sysrib.go` and by the new seam; document the reachability seam beside the IGP cost seam
- `docs/features/srv6.md` - correct the resolvability row, which currently publishes the FIB-time check as the Section 5 behavior, and fix the `IsSRv6Ineligible()` casing
- `rfc/short/rfc9252.md` - `RFC9252-5-2` loses its `{gap}`, and the `## Meta` `Support remaining` paragraph drops it from the gap list

## Files to Create
- `internal/core/rib/reachable/reachable.go` - the leaf seam: a `Func` type, `Set` for the producer, and a `Lookup` returning both the verdict and whether a producer exists
- `internal/core/rib/reachable/reachable_test.go` - the seam's own tests
- `internal/component/bgp/plugins/rib/srv6_resolvability_test.go` - the unit tests above, carrying the `RFC9252-5-2` tags
- `internal/component/sysrib/reachable_seam_test.go` - the producer-side tests
- `test/plugin/srv6-bestpath-unresolvable.ci`, `test/plugin/srv6-bestpath-resolvable.ci`, `test/plugin/srv6-bestpath-static-wins.ci` - the functional tests
- `test/interop/scenarios/bgp-srv6-resolvability-frr/` - the interop scenario, with `ze.conf` and `frr.conf`
- `rfc/discrimination/rfc9252.json` - written by `./le rfc discriminate-record`, not by hand

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No operator-facing knob in this spec. The local-policy override is the open question below; if Thomas asks for a config option, this row becomes Yes and the leaf lands under the BGP RIB plugin's YANG |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | No | The verdict is observable through existing `show` output for the prefix; no new verb |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | Yes | `test/plugin/srv6-bestpath-unresolvable.ci`, `test/plugin/srv6-bestpath-resolvable.ci`, `test/plugin/srv6-bestpath-static-wins.ci` |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | No new environment leaf |
| Doctor check for runtime dependencies | No | No new file path, socket, port, module, or binary: the seam is in-process and the Loc-RIB already exists |
| Prometheus counters/metrics | Yes | Counters for paths excluded as unresolvable, re-evaluations enqueued, and re-evaluations drained, registered in the BGP RIB plugin's metrics; the enqueue and drain pair is how R-3 is detected |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new SAFI, capability, or attribute: this changes candidacy for families that already exist |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- the SRv6 row states when a route is usable |
| 2 | Config syntax changed? | No | No new syntax |
| 3 | CLI command added/changed? | No | No new command |
| 4 | API/RPC added/changed? | No | No new RPC |
| 5 | Plugin added/changed? | No | No plugin added or removed; the BGP RIB plugin gains behavior, covered by row 12 |
| 6 | Has a user guide page? | Yes | `docs/features/srv6.md`. It carries an OVER-CLAIM this spec's work is what makes true: line 195 publishes `\| SID resolvability check \| 5 \| Implemented (Loc-RIB LPM) \|`, and the check runs at FIB install rather than before best-path, which is what Section 5 requires. Measured 2026-09-08. `ai/rules/rfc-compliance.md` allows one repair and it is this spec: PROVE the claim. Lowering the row instead is banned, because it would leave Ze exactly as non-conformant and only the ledger would change. The row is corrected to describe the real placement when this spec closes, never before |
| 7 | Wire format changed? | No | No encoding changes; only which path is selected |
| 8 | Plugin SDK/protocol changed? | No | The seam is internal, not part of `pkg/plugin` |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9252.md` (`RFC9252-5-2` and the `## Meta` rows); `rfc/requirements/rfc9252.md` and `docs/features/rfc-status.md` regenerate through `./le rfc index-update` |
| 10 | Test infrastructure changed? | No | Existing `.ci` and interop harnesses, new cases only |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- SRv6 resolvability is a behavior other daemons implement |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (the seam) and `docs/architecture/plugin/rib-storage-design.md` (the filter and the worker) |
| 13 | Route metadata keys added/changed? | No | No metadata key added |
| 14 | Prometheus counters added/changed? | Yes | The three counters above, in the BGP RIB telemetry doc |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing new registers in an inventory a doc lists |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/immediate/spec-srv6-bestpath-resolvability.md`. Known now: `rib_commands.go`, `rib_bestchange.go` and `rib.go` declare `docs/architecture/plugin/rib-storage-design.md`; `sysrib.go` declares `docs/architecture/core-design.md`; `docs/features/srv6.md` carries source anchors on `rib_bestchange.go` and `sysrib.go`. All are named above |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/features/srv6.md` shows the best-path and resolvability sequence; verify it against the new order |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the seam exists and both ends reach it
   - Tests: `TestReachableSeamUnsetIsNotUnresolvable`, `TestReachableSeamPublishedBySysrib`
   - Files: `internal/core/rib/reachable/reachable.go`, `internal/component/sysrib/sysrib.go`
   - Verify: the seam is registered by sysrib and readable from the BGP RIB package; the wiring tests fail first because `Lookup` answers nothing
2. **Phase: The candidate filter** -- an unresolvable SID costs candidacy
   - Tests: `TestGatherCandidatesSkipsUnresolvableSRv6SID`, `TestGatherCandidatesAdmitsResolvableSRv6SID`, `TestUnresolvableSRv6LosesToNonSRv6Path`, `TestResolvabilityUsesMergedTransposedSID`, `TestEVPNUnresolvableSRv6SIDIneligible`, `TestResolvabilityCheckInactiveWithoutProducer`
   - Files: `internal/component/bgp/plugins/rib/rib_commands.go`, `internal/component/bgp/plugins/rib/rib_bestchange.go`
   - Verify: the `RFC9252-5-1` tests still pass untouched; the new tagged tests carry the RFC comment quoting Section 5
3. **Phase: The dependency index** -- every SRv6 candidate records what it depends on, whichever way the verdict fell
   - Tests: `TestSRv6DependencyIndexEmptiesOnWithdraw`
   - Files: `internal/component/bgp/plugins/rib/rib_bestchange.go`, `internal/component/bgp/plugins/rib/rib.go`
   - Verify: an EXCLUDED path is in the index (A-4), which is what makes recovery possible at all
4. **Phase: Re-evaluation** -- a Loc-RIB change re-runs best-path for the dependents
   - Tests: `TestLocRIBChangeReevaluatesSRv6Prefix`, `TestLocRIBRemovalReevaluatesSRv6Prefix`
   - Files: `internal/component/bgp/plugins/rib/rib.go`
   - Verify: the handler takes no BGP lock and does no work inline (R-1, R-2); run the package under the race detector
5. **Phase: Functional and interop proof**
   - Tests: the three `.ci` files and the `bgp-srv6-resolvability-frr` scenario
   - Files: `test/plugin/`, `test/interop/scenarios/bgp-srv6-resolvability-frr/`
   - Verify: revert the filter, rebuild the daemon, confirm each goes RED, restore, confirm GREEN, and record the RED (`ai/rules/interop-and-goal-validation.md`)
6. **Phase: The ledger and the pages**
   - Tests: `./le rfc check`
   - Files: `rfc/short/rfc9252.md`, `docs/features/srv6.md`, `docs/features.md`, `docs/comparison.md`, `docs/architecture/core-design.md`, `docs/architecture/plugin/rib-storage-design.md`
   - Verify: `./le rfc discriminate stem rfc9252 report <path>` proposes the breaks, `./le rfc discriminate-record` observes one and writes `rfc/discrimination/rfc9252.json`, and `./le rfc index-update` regenerates `rfc/requirements/rfc9252.md`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation and a named test; AC-7 is proven over the FIB, not over the RIB alone |
| Feature completeness | The recovery direction (AC-3) works from FULL exclusion, where no best-path record exists to key on |
| Correctness | The address checked is the merged SID from `srv6SIDFromResult`, and the check runs for every family carrying a Service TLV, EVPN included |
| Naming | The seam mirrors `igpcost`: a `Func` type, `Set` for the producer, `Lookup` for the consumer |
| Data flow | The `OnChange` handler holds no BGP lock, calls no RIB mutation, and only enqueues |
| Rule: `ai/rules/principles.md` | The unset seam is not readable as "unresolvable": `Lookup` reports whether a producer exists, and the caller's permissive default is written once |
| Rule: `ai/rules/rfc-compliance.md` | The tagged claim states what the test body checks and no more; the discrimination record is in the same change |
| Rule: `ai/rules/goroutine-lifecycle.md` | One long-lived worker on a channel, stopped in `SetLocRIB` and at plugin shutdown |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The seam package exists and has one producer and one consumer | `grep -rn "rib/reachable" internal/ --include='*.go'` shows the sysrib `Set` and the BGP `Lookup` |
| `RFC9252-5-2` is no longer a gap | `grep -n "RFC9252-5-2" rfc/short/rfc9252.md rfc/requirements/rfc9252.md` shows both polarities and no `{gap}` |
| Both polarities are tagged | `grep -rn "RFC requirement: RFC9252-5-2" internal/` returns a positive and a negative |
| The discrimination record exists and is honest | `./le rfc check` passes with `rfc/discrimination/rfc9252.json` present |
| The interop scenario is discovered by name | `./le integration` lists `bgp-srv6-resolvability-frr` |
| No test of `RFC9252-5-1` was edited | `git diff --stat internal/component/bgp/plugins/rib/srv6_ineligible_test.go` is empty |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The SID comes from a peer's attribute bytes; the check must not extend the parsing surface, only read the already-reconstructed address |
| Resource exhaustion | A peer advertising many distinct SIDs grows the dependency index; the index is bounded by the prefixes in the Adj-RIB-In and is emptied on withdraw (R-5) |
| Denial by a neighbour | A peer cannot force unbounded re-evaluation: notifications come from Loc-RIB changes, and the worker coalesces |
| Fail-closed or say something | An unset seam admits every path, which is the pre-existing behavior, and it is ANNOUNCED once in the log rather than left silent (R-6) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| `./le tier check` refuses the new package | The seam is in the wrong tier: it MUST be a leaf under `internal/core/`, imported by both sides and importing neither |
| Deadlock in the re-evaluation path | R-1 or R-2: the handler is doing work it must hand to the worker |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The circular dependency the earlier skeleton feared is not one. Best-path for prefix P asks about the SID's covering prefix L, which is a different prefix in a different table, learned by IGP, static, or another protocol. Only a locator covered by a prefix that itself resolves through P is circular, and `nhResolver.Resolve` already bounds that with `maxRecursionDepth` and a self-reference test.
- The RFC's "before best path computation" is not satisfied by checking earlier in the same pipeline: it is satisfied by making the path lose candidacy, which is what lets the runner-up win. That is the whole behavioral difference from today.
- `rfc/short/rfc9252.md` and the generated `rfc/requirements/rfc9252.md` place `RFC9252-5-1` at section 5, but its sentence is in Section 7 (Error Handling) of `rfc/full/rfc9252.txt`. The requirement itself is right; only the section annotation is wrong. Reported to the main thread rather than fixed here, because this spec writes no file but itself.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Publish reachability from sysrib through a new leaf seam `internal/core/rib/reachable`, mirroring `internal/core/rib/igpcost` | (a) BGP resolves against `RIBManager.locRIB` itself with `locrib.RIB.LPM`; (b) move `nhResolver` down into a leaf both components construct | (a) duplicates the recursive walk `nhResolver.Resolve` owns, so the RIB and the FIB could disagree about one SID, which `ai/rules/principles.md` forbids. (b) is the cleanest end state but moves a file `plan/immediate/spec-fib-depth.md` declares as its design and that the withholding work is editing, so it buys a conflict for no behavior. The seam is the proven shape: `igpcost` already carries a sysrib answer to BGP best-path with no dependency inversion |
| `Lookup` returns the verdict AND whether a producer is registered | A single bool with "unset means resolvable" | A bare bool makes "no producer" indistinguishable from a real answer, which is the exact failure `ai/rules/principles.md` names first. The permissive default is then written once, at the one caller, where it is visible |
| The `OnChange` handler enqueues and returns; a single worker re-runs best-path | Re-running best-path inline in the handler | `locrib.RIB.OnChange` documents that handlers run under the shard write lock and must not re-enter Insert/Remove. Best-path calls `InsertForward`, so inline work is an immediate self-deadlock (R-2) and a lock-order inversion against `peerMu` (R-1) |
| The dependency index is written by the FILTER, for excluded and admitted candidates alike | Recording only the winner's SID, as sysrib does | An excluded path leaves no best-path record, so a winner-keyed index can never bring it back. Recovery (AC-3) is impossible without this |
| The check runs on the merged SID from `srv6SIDFromResult` | The partial SID from `pool.ExtractSRv6SID`, which `isSRv6Ineligible` already computes | Section 3.2.1 does not forbid a transposition window overlapping the locator, so the partial SID can name a different address (A-2). The merged SID is what the FIB installs, so it is what must resolve |
| Keep the sysrib check at `fibEntry` unchanged | Remove it as redundant once the RIB filters | It is the last gate before a kernel encap route, it is correct, and it still runs when the seam has no producer. Removing a working guard to avoid a second check is a removed guard (`ai/rules/planning.md`) |

## Known Limitations

- "If the SRv6 SID is reachable via more than one forwarding table, local policy is used to determine which table to use" has no trigger in Ze: there is one Loc-RIB and no VRF plumbing to install VPN routes into, which `mirrorToLocRIB` states as the reason it skips non-CIDR families (`internal/component/bgp/plugins/rib/rib_bestchange.go`). The sentence becomes reachable when Ze gains VRF forwarding tables, and it is conditional on that absent feature rather than outstanding work here.
- The `RFC9252-5-1` section annotation defect in `rfc/short/rfc9252.md` is reported, not repaired here.
- The four unproven SRv6 branches in `internal/component/sysrib` were given to
  `spec-per-protocol-fib-import` by Thomas's direction of 2026-09-08. That spec
  closed on 2026-09-09 and added `internal/component/sysrib/sysrib_srv6_test.go`,
  the package's first SRv6 tests, six of them.

## Open Question For Thomas (BLOCKING before implementation)

RFC 9252 Section 5 also says:

> "The result of an SRv6 Service SID resolvability (e.g., when provided via IGP
> Flexible Algorithm) can be ignored if the ingress PE has a local policy that
> allows an alternate steering mechanism to reach the egress PE."

That is a permission, not an obligation, and `ai/rules/rfc-compliance.md` says a
MAY clause is put to the user: implement it, skip it, or make it a config
option. This spec does not pick. The three readings are: no override at all, an
override tied to an existing SR Policy steering configuration, or an explicit
per-peer or per-family knob. The rest of the spec is unaffected by the answer,
because the override would sit in front of the same filter.

## RFC Documentation (Scope: protocol)

The filter carries a comment naming RFC 9252 Section 5 and quoting "the ingress
PE MUST perform a resolvability check for the SRv6 Service SID before
considering the received prefix for the BGP best path computation", and states
that Section 6 repeats it for EVPN. The two tagged tests are:

| Polarity | Tag line | Location |
|----------|----------|----------|
| Negative | `RFC requirement: RFC9252-5-2 negative -- a path whose SRv6 Service SID has no covering route in the Loc-RIB is excluded from best-path candidate selection` | `internal/component/bgp/plugins/rib/srv6_resolvability_test.go` |
| Positive | `RFC requirement: RFC9252-5-2 positive -- a path whose SRv6 Service SID resolves in the Loc-RIB is admitted to best-path candidate selection` | `internal/component/bgp/plugins/rib/srv6_resolvability_test.go` |

The claim states what the test body checks and no more. Both tags are NEW, so
the change owes a discrimination record from `./le rfc discriminate-record`.

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
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket
- [ ] Thomas has answered the Open Question, and the answer is recorded here

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)
- [ ] The revert-rebuild-RED walk recorded for each functional and interop test
- [ ] `./le rfc discriminate-record` has written `rfc/discrimination/rfc9252.json`

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary routed to the surface that governs it (`ai/rules/planning.md`)
- [ ] **Commit A:** code + tests + docs + spec + journal row
- [ ] **Commit B:** `git rm plan/immediate/spec-srv6-bestpath-resolvability.md` only (commit A preserves the spec in history)
