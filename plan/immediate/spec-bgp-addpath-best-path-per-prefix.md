# Spec: ADD-PATH Best Path Elected per Prefix, Labels Held per Path


| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-10-04 |

Closure owner: parent session `01a10694-3795-71f2-8250-25e3a877f4cf`.
Implementation was authorized for all ADD-PATH families on 2026-10-03.
The independent closure phase preserves the five completed review rounds;
it does not authorize a sixth product review or unrelated capability work.

## Design (chosen 2026-10-03, Option A)

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Select once per prefix; storage stays keyed (path id, prefix); `PeerRIB.AppendPrefixPaths` appends every path of a prefix into a caller-owned (stack-backed) slice | Keep per-path-id selection and compare across ids afterwards | RFC 8277 Section 3.1 and RFC 7911 Section 2: the id names a path, it never partitions selection. FRR `bgp_best_selection` walks every `pi` of the dest; BIRD `rte_recalculate` walks the net |
| Each peer is asked by PREFIX, never with another session's wire key | Re-frame the key per peer | Closes the cross-mode defect: an ADD-PATH key `00000007 08 0a` parsed as `0/0` in a non-ADD-PATH peer (store.NLRIToPrefix ignores trailing bytes) |
| `Candidate` carries `PathID` and `AddPath`; winner-dependent reads use `candidatePath`, while show-best framing uses the winner's identifier and route | Re-read with the triggering UPDATE's key | The triggering UPDATE may be another path or peer; CIDR reads by prefix avoid escaping wire-key buffers |
| `bestPrevSet` and the `multi` store deleted; one `bestPrevRecord{rec, pathID, addPath}` per prefix; the best-change `PathID`/`AddPath` are the WINNER's | Keep both stores (layering, banned) | One best per prefix; record grows 8 to 16 bytes per prefix (footprint bench 27.5 to 35.5 heap bytes/entry), the ADD-PATH per-prefix heap slice is gone |
| Loc-RIB mirror Instance 0 (`bgpLocRIBInstance`), `locrib.Path.Instance` comment corrected | Instance = path id (old code, contradicted the comment) | Two BGP paths in the Loc-RIB would be re-ranked by distance and MED alone, overriding RFC 4271 |
| Final tie-break: lowest path id (`BestStepPathID`, `lost-path-id`) | Storage order | Ze choice, documented as not RFC text: deterministic election |
| Peer-down: `emitPurgedWithdraws` re-elects each purged route from the remaining paths | Leave the route withdrawn until the survivor's next UPDATE | Per-prefix records made the existing gap reachable for ADD-PATH (the survivor path no longer had its own record) |
| Labels: label handle per path in `pathEntry`; `setLabels` replaces and releases it, while `remove`/`releaseAll` release removed paths | Parallel per-(id,prefix) BART | One store per fact; retaining the old handle through `upsert` prevents an unbound interval between insertion and rebind |
| AC-7 reads Ze's election through the RFC 9069 Loc-RIB stream decoded by pmacct (`bgp-addpath-best-path-pmacct`): one ADD-PATH session carries two paths of 10.0.0.0/24; pmacct identifies the MED 10 path by AS_PATH 65004 65010 and must never report the MED 50 path, AS_PATH 65004 65050 (accepted by the main thread on the owner's authority, 2026-10-03) | FRR receives Ze's best over BGP, the original proposal | Ze's RIB does not re-advertise and its route server relays without choosing. The Loc-RIB stream exposes Ze's election to a third-party decoder, but omits MED, so AS_PATH distinguishes the paths |
| Opaque-key families (VPN, EVPN, ...): strip the path id from the opaque key, add a per-path value layer, same per-key selection | Leave keyed by full bytes | Same RFC 8277 Section 3.1 defect for VPN (RFC 8277 labels in RFC 4364 NLRI) and every other family |

R-1 audit (2026-10-03): no consumer outside `bgp-rib` reads `BestChangeEntry.PathID`
(grep of every importer of `ribevents.BestChange*`: sysrib, fib kernel/vpp/p4,
bmp_locrib, redistribute, flowexport, static, isis, fakefib). The ADD-PATH send
side keys on sent UPDATEs (`ribOutKey`, `rib.go`), not on best-change. R-1 holds.

### Phases
The phase rows record their state when they landed. Later review repairs
and the closure audit supersede temporary wrappers, gaps and open proof work.


| Phase | Content | State |
|-------|---------|-------|
| 1 | CIDR families: per-prefix gather, one record per prefix, winner-keyed reads, Loc-RIB Instance 0, tie-break, peer-down re-election, docs | done, committed |
| 2 | Labels per path (AC-4, RFC8277-2.5-2 same-id case, RFC8277-2.5-3 and its `{gap}` removal): handle in `pathEntry`; `setLabels` releases a replaced binding, `remove`/`releaseAll` release removed paths, and `refresh` keeps it. The temporary gather wrappers were removed in review round 2 | done, committed; same-next-hop proof strengthened in round 5 |
| 3a | Opaque-key families (VPN, EVPN, MVPN, MUP, VPLS, BGP-LS, flowspec storage): path id out of the key (`FamilyRIB.opaqueRouteKey`), `opaqueMulti map[string]pathSet` per route key (`storage/familyrib_opaque.go`), `PeerRIB.AppendKeyPaths`, `gatherKeyCandidatesLocked` asks every peer by the route key and appends every path, bestPrev opaque keyed by route key with the winner's path id, `BestChangeEntry.NLRI` is the winner's framed NLRI with `AddPath`/`PathID`; VPN and EVPN twins of AC-1/AC-8/AC-10 (`addpath_opaque_best_per_route_test.go`), red against HEAD then green | done |
| 3b | Labels out of the VPN and EVPN route key. The label stack is still inside the opaque key, so (probe through a go test overlay, 2026-10-03) two PEs announcing one RD:prefix with labels 100 and 101 are two routes and never meet in one election, a re-advertisement with a new label stores a second route instead of replacing the first (RFC 8277 Section 2.5), and a withdrawal carrying the Compatibility value 0x800000 removes nothing (RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility field MUST be ignored."). Not ADD-PATH specific. Fix: the route key is derived through the per-family key registry the Adj-RIB-Out and reactor already use (`nlrisplit.GetPrefixKey`, wrapped by `storage.RouteKey`), so no second hook was added; each path keeps the wire route it was received with (`FamilyRIB.wire`, `PrefixPath.Route`, `Candidate.Route`, `opaqueBestPrev.route`), so walks and replay hand back labels unchanged; received withdrawals go through `PeerRIB.Withdraw` and `checkRouteBestChange(withdraw)`. Tests: `rfc8277_vpn_route_key_test.go`, `rfc8277_vpn_route_key_addpath_test.go`, red against HEAD by overlay, records for RFC8277-2.5-1 and 3.1-1. RFC8277-2.4-1 stays `{gap}`: labeled unicast (SAFI 4) withdrawals still go through `removeLabeled`/`ExtractLabels` | done |
| 3c | Labeled unicast (SAFI 4) withdrawals: every RIB withdrawal path (`rib_structured.go`, `removePoolNLRIs`, `rib_inject.go`) splits with `nlrisplit.SplitWithdrawn` (the family's withdrawal framing, `GetWithdraw`), and `storage.LabeledWithdrawnPrefix` skips the Compatibility field through `keyLabeled(withdraw)`, so a withdrawal with any Compatibility value removes the route and its label (RFC8277-2.4-1, `{gap}` removed). `removeLabeled` no longer reads `ExtractLabels`; the dead `RemoveLabels` chain (PeerRIB, FamilyRIB, pathSet) is deleted, because removing a route releases its label. Tests: `rfc8277_labeled_withdraw_test.go` (red against HEAD: 0x800000 and zero leave the route installed), `evpn_route_key_test.go` (EVPN relabel, and a withdrawal with another label and ESI; red against `28679bbd73^` by overlay) | done |
| 4 | Functional `.ci` (`show rib best` one best for two path ids), interop (extend `bgp-addpath-frr` or the rail-agreement pattern) with revert-rebuild-red recorded | `.ci` done: `test/plugin/show-rib-best-addpath.ci` with driver `internal/test/fixture/register_show_rib_best_addpath.go`, red by overlay twice (gather back to one path id: path id 1, MED 50 reported best; best rows keyed by path id: two rows), green. Interop done: `bgp-addpath-best-path-pmacct` (Decision above), pmacct reads the Loc-RIB best as the MED 10 path (AS_PATH 65004 65010) and never the MED 50 path (65004 65050); the paths are told apart by AS_PATH because Ze's Loc-RIB Route Monitoring drops MED (journal `constant-reported-as-measured-state`). Green image 60c27bd7c7c8, red by overlay (gather filtered to the triggering path id) image eff6f1c57246 at assertion 4 (Loc-RIB row carries `"as_path": "65004 65050"`), restored green (see the commit body) |

### Added acceptance criteria

| AC ID | Input / Condition | Expected Behavior | Evidence |
|-------|-------------------|-------------------|----------|
| AC-8 | Non-ADD-PATH peer holds 0.0.0.0/0, ADD-PATH peer holds 10/8 path 7 | 0/0 is never a candidate for 10/8 | `TestAddPathMixedModeKeyNeverReadsAsAnotherPrefix` |
| AC-9 | Path 7 LP 200 MED 50, path 9 LP 100 MED 10 | One BGP Loc-RIB path for the prefix, path 7's | `TestAddPathLocRIBHoldsOneBGPPath` |
| AC-10 | Withdraw non-best, then best | Non-best: no change; best: path 9 promoted (Update, PathID 9); last: Withdraw naming 9 | `TestAddPathWithdrawalKeepsOrPromotes`, `TestPurgeBestPrevForPeerAddPath` (peer-down) |
| AC-11 | VPN and EVPN twins of AC-1, AC-8 and AC-10 | One election per NLRI-without-path-id | `TestAddPathOpaqueOneElectionPerRoute`, `TestAddPathOpaqueMixedModeMeetInOneElection`, `TestAddPathOpaqueWithdrawalKeepsOrPromotes` |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later Opus 5 session reviews that commit and closes. `-` closes in the same session. -->

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     The file's DIRECTORY carries the release bucket: plan/immediate/ for a defect
     an operator meets, plan/pre-release/ for work the release cannot go out
     without, plan/ for everything else (plan/README.md). -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Record and resolve one best-path defect in the RIB (`bgp-rib`) that affects
every family received with ADD-PATH (RFC 7911). When one session carries two
paths for the same prefix under different Path Identifiers, the RIB never
compares them: it gathers candidates under the path-id-keyed NLRI and keeps one
best record per path id, so it elects one "best" per identifier instead of one
best per prefix. A second, related defect sits in the same per-path state:
MPLS label side-data is keyed on the prefix alone, so a second path's label
overwrites the first path's binding.

Found by spec-rfc-verdict-test-fix-pass (child spec-rfc-verdict-fix-bgp,
continuation 28, 2026-10-02). OWNER RULING 8 (c) in
`plan/handover/rfc-verdict-test-fix-pass/RULINGS.md` homes it here: "ADD-PATH
best path per (path id, prefix) (RFC8277-3.1-1, 2.5-2): home it in a new spec;
both ids go to the BGP child's Blocked by. Whether it runs now: ask the owner
when the spec exists."

Thomas resolved that scheduling question on 2026-10-02: run this spec after
`spec-rfc-verdict-test-fix-pass` and before release. The failing probe and the
BGP child's named dependencies stay intact; this schedule does not claim a fix.

## Historical Evidence (before implementation)

| Item | Value |
|------|-------|
| Failing probe | `internal/component/bgp/plugins/rib/rfc8277_addpath_comparable_red_test.go`, `TestRFC8277AddPathRoutesOnOneSessionAreComparable` (untracked, untagged, red on purpose) |
| Shape | one ADD-PATH peer sends 10.0.0.0/8 as path 7 (label 100, MED 10) and path 9 (label 200, MED 20). Under either path key the candidate set must hold 2 routes and best-path selection must pick MED 10. Today each key holds 1 candidate, and under path 9's key the MED 20 route is elected |
| Producer, selection | `RIBManager.gatherCandidatesLocked` (`internal/component/bgp/plugins/rib/rib_commands.go`) looks up each peer's RIB with the path-id-keyed NLRI; the best-change store in `internal/component/bgp/plugins/rib/rib_bestchange.go` keeps a per-prefix path-id to record map (`bestPrevSet`), one best per path id |
| Producer, labels | `PeerRIB.SetLabelsIfRouteExists` and `PeerRIB.RemoveLabels` (`internal/component/bgp/plugins/rib/storage/peerrib.go`) drop the path id; `FamilyRIB.SetLabels` (`internal/component/bgp/plugins/rib/storage/familyrib.go`) holds one handle per prefix |
| Author record | `plan/handover/rfc-verdict-test-fix-pass/bgp/c28-author.md`, rows RFC8277-3.1-1 and RFC8277-2.5-2 |
| Original requirements | RFC8277-3.1-1 and RFC8277-2.5-2 were weak, RFC8277-2.5-3 was a gap. All three are now independently re-judged `enforced` in `rfc/audit/rfc8277.json`; closure removes the first two from the BGP child's Blocked-by table |

### RFC text read for this skeleton

| Source | Section | Sentence |
|--------|---------|----------|
| `rfc/full/rfc8277.txt` | 3.1 | "the two UPDATEs are received on the same session, add-paths is used on that session, and the NLRIs of the two UPDATEs have different path identifiers." |
| `rfc/full/rfc8277.txt` | 3.1 | "These two routes MUST be considered to be comparable, even if they specify different labels." |
| `rfc/full/rfc8277.txt` | 3.1 | "Thus, the BGP best-path selection procedures (see Section 9.1 of [RFC4271]) are applied to select one of them as the better path." |
| `rfc/full/rfc8277.txt` | 2.5 | "If I1 is the same as I2, UPDATE U2 MUST be interpreted as meaning that L2 is now bound to P at N1 and that L1 is no longer bound to P at N1." |
| `rfc/full/rfc8277.txt` | 2.5 | "If I1 is not the same as I2, U2 MUST be interpreted as meaning that L2 is now bound to P at N1, but U2 MUST NOT be interpreted as meaning that L1 is no longer bound to P at N1." |
| `rfc/full/rfc7911.txt` | 2 | "a new identifier (termed \"Path Identifier\" hereafter) needs to be introduced so that a particular path for an address prefix can be identified by the combination of the address prefix and the Path Identifier." |
| `rfc/full/rfc7911.txt` | 5 | "Apart from the fact that this is now possible, the route advertisement rules of [RFC4271] are not changed." |
| `rfc/full/rfc7911.txt` | 5 | "A BGP speaker SHOULD include the best route [RFC4271] when more than one path is advertised to a neighbor, unless it is a path received from that neighbor." |

-> Constraint: the path id identifies a path (RFC 7911 Section 2); it never
partitions best-path selection. Storage stays keyed on (path id, prefix);
selection and the best-change record are per prefix, over every path of every
peer.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/plugin/rib-storage-design.md`
  → Decision: keep path identifiers in the value layer; gather by prefix or route key.
  → Constraint: winner-dependent reads must use the winner's own path and peer.
- [ ] `docs/architecture/route-selection.md`
  → Decision: preserve the existing decision criteria; add only a final path-id tie-break.
  → Constraint: the tie-break is a Ze choice, not an RFC requirement.
- [ ] `docs/architecture/rib/unified-locrib.md`
  → Decision: mirror one selected BGP path at Instance 0.
  → Constraint: a second path must not be re-ranked by cross-protocol distance and MED.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc8277.md`
  → Constraint: labels do not divide comparable routes; a same-id rebind replaces only that path.
- [ ] `rfc/short/rfc7911.md`
  → Constraint: an identifier names a path, not an independent election.

**Key insight:** route identity, received-path identity and forwarding labels
are different facts. The storage and selection keys must preserve that distinction.

## Current Behavior

`rib_commands.go::gatherPrefixCandidatesLocked` and
`gatherKeyCandidatesLocked` collect all eligible paths of one route across
peers. `rib_bestchange.go::checkRouteBestChange` stores one winner and reads
its next hop, labels and SID through `candidatePath`. `storage/pathset.go`
holds the labels of each ADD-PATH path; `familyrib_opaque.go::setRouteNLRI`
keeps each opaque path's latest forwarding fields outside its route key.

**Preserved:** negotiated framing, stored received path identifiers, existing
policy eligibility, command grammar, and event field names.

**Changed:** one election and best record per prefix or opaque route key;
independent label bindings; survivor promotion without a transient withdrawal.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A received UPDATE reaches `bgp-rib` as a structured event (`handleReceivedStructured`, `rib_structured.go`), one NLRI at a time, with the session's ADD-PATH flag for the family
- NLRI wire bytes: `[path-id:4][prefix-len:1][prefix]` under ADD-PATH, `[prefix-len:1][prefix]` otherwise

### Transformation Path
1. The route is stored in the peer's Adj-RIB-In keyed (path id, prefix) (`FamilyRIB.Insert`, `pathSet` under ADD-PATH)
2. `checkRouteBestChange` derives the prefix or opaque route key once and gathers every path through `gatherPrefixCandidates` or `gatherKeyCandidates`.
3. `SelectMultipath` elects one best; winner reads go through `candidatePath`; one `bestPrevRecord` per prefix; CIDR Loc-RIB Instance 0; one best-change per route naming the winner's path id.

Design documents changed with the code: `docs/architecture/plugin/rib-storage-design.md` (per-prefix record, gather by prefix, Loc-RIB instance, peer-down re-election), `docs/architecture/route-selection.md` (ADD-PATH paragraph, `lost-path-id` step), `docs/architecture/rib/unified-locrib.md` (BGP Instance 0).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | `handleReceivedStructured` carries session ADD-PATH context; the JSON rail retains its existing event shape | `TestRFC8277AddPathRoutesOnOneSessionAreComparable`, real-daemon `.ci` |
| RIB ↔ Loc-RIB | Selected CIDR path at Instance 0; opaque routes stay on the best-change event rail | `TestAddPathLocRIBHoldsOneBGPPath`, VPN/EVPN tests |
| Ze ↔ third-party collector | RFC 9069 Loc-RIB monitoring stream | `bgp-addpath-best-path-pmacct` |

### Integration Points
- `PeerRIB.AppendPrefixPaths` / `AppendKeyPaths` supply every path without cross-session wire-key reuse.
- `nlrisplit.GetPrefixKey` and `SplitWithdrawn` supply registered family identity and withdrawal framing.
- `candidatePath`, `lookupStoredPath` and the existing best-change bus carry the winner's fields.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| Intended data path | Yes | Received UPDATE → storage → `checkRouteBestChange` → existing event/Loc-RIB rails |
| Component isolation | Yes | Existing `ribevents` boundary; no new cross-component imports or plugin API |
| No duplicate mechanism | Yes | Registered family key operation reused; `bestPrevSet` and dead gather wrappers removed |
| Buffer lifetime | Yes | CIDR winner read uses `LookupPath`; opaque wire copies survive storage and bus lifetimes; rs extraction owns pooled scratch |
| Outbound registration | Yes | No new command or family; interop checker registered in `check_special.go` and fixture in `register_show_rib_best_addpath.go` |
| Inbound discovery | Yes | Existing family registry (`nlrisplit`) and fixture/scenario registries discover the new cases; no core per-family switch was added |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No best-change consumer needs a best record per received identifier | Existing `ribevents.BestChangeEntry` consumers and the original R-1 audit | Per-prefix publication would remove a caller's state | `checkRouteBestChange` supplies one selected path; `TestAddPathLocRIBHoldsOneBGPPath` and peer-down promotion pass | confirmed |
| A-2 | A downstream BGP peer can observe Ze's election | Original AC-7 proposal | FRR relay observations would prove FRR's election, not Ze's | `docs/guide/route-reflection.md` and the accepted pmacct alternative; recorded overlay turns assertion 4 red | broken; accepted alternative recorded |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The ADD-PATH send side and the best-change consumers depend on one best per path id | a consumer reading `BestChangeEntry.PathID` per path | Audited 2026-10-03: none does (see Design, R-1 audit) |
| R-2 | Peer-down left a prefix withdrawn while another path survives | `TestPurgeBestPrevForPeerAddPath` | `emitPurgedWithdraws` re-elects each purged route |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Best-path election for every family received with ADD-PATH; label bindings for labeled unicast and VPN |
| How is it reverted? | Revert the scoped implementation commits together; no config migration. A revert restores the known per-id election and label-clobber defects, so it is not a correctness fallback |
| Who else touches this path? | Link-local D6 and startup-race work share the checkout; their reactor changes and RFC rows are excluded from this closure |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Received ADD-PATH UPDATE | → | `handleReceivedStructured` → `checkRouteBestChange` | `TestRFC8277AddPathRoutesOnOneSessionAreComparable`; `show-rib-best-addpath.ci` |
| Labeled MP_UNREACH | → | `MPUnreachWire.NLRIs` → withdrawal framing → rs inventory | `labeled-withdraw-compatibility.ci`; `bgp-labeled-withdraw-compatibility-frr` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | One ADD-PATH session sends one prefix as path 7 (MED 10) and path 9 (MED 20) | Both paths are candidates of one selection for the prefix, and MED 10 is best, whichever path key the lookup starts from |
| AC-2 | Same as AC-1 for a family without labels (plain IPv4 unicast with ADD-PATH) | Same single election per prefix: the defect is not label-specific |
| AC-3 | `TestRFC8277AddPathRoutesOnOneSessionAreComparable` | Becomes the tagged failing-first test for RFC8277-3.1-1: red before the fix (recorded), green after, with a discrimination record |
| AC-4 | ADD-PATH session: path 7 label 100, path 9 label 200, then path 9 re-advertised with label 300 | Path 7 still reads label 100, path 9 reads 300; withdrawing path 9 leaves path 7's label in place |
| AC-5 | RFC8277-3.1-1 and RFC8277-2.5-2 | Verdicts `enforced`: tagged units in both polarities with discrimination records, re-judged by an agent that did not write them; both removed from the Blocked-by list of `plan/pre-release/spec-rfc-verdict-fix-bgp.md` |
| AC-6 | RFC8277-2.5-3 | The `{gap}` annotation is removed and the row reaches `enforced` the same way as AC-5 |
| AC-7 | Accepted interop alternative, 2026-10-03 | `bgp-addpath-best-path-pmacct`: one injector sends two paths of one prefix over ADD-PATH; pmacct independently decodes Ze's RFC 9069 Loc-RIB stream and observes the better path, never the worse path. The scenario must fail with selection reverted. This replaces the original FRR receiver proposal because Ze does not advertise its selected best path over BGP |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Receive two ADD-PATH routes and inspect `show bgp rib best` | Wire UPDATE → negotiated parser → RIB storage → per-route gather → best pipeline | `test/plugin/show-rib-best-addpath.ci` |
| 2 | Relabel or withdraw one labeled path | Structured UPDATE → per-path label store → targeted replacement/removal | `TestRFC8277AddPathSameNextHopKeepsIndependentLabels`; labeled-withdrawal `.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC8277AddPathRoutesOnOneSessionAreComparable` | `internal/component/bgp/plugins/rib/rfc8277_addpath_comparable_test.go` | AC-1/AC-3; both identifiers gathered, MED opposes path-id tie-break | Recorded pre-fix red; current package PASS |
| `TestAddPathUnicastOneElectionPerPrefix`, mixed-mode, Loc-RIB and withdrawal tests | `internal/component/bgp/plugins/rib/addpath_best_per_prefix_test.go` | AC-2/8/9/10 | Current package PASS |
| VPN/EVPN one-election, mixed-mode and withdrawal tests | `internal/component/bgp/plugins/rib/addpath_opaque_best_per_route_test.go` | AC-11 | Recorded pre-fix red; current package PASS |
| `TestRFC8277AddPathSameNextHopKeepsIndependentLabels` | `internal/component/bgp/plugins/rib/rfc8277_same_next_hop_test.go` | Same-next-hop label isolation and replacement | Current package PASS; two semantic overlays red |
| `TestInventoryGroupedExtractionAllocations` | `internal/component/bgp/plugins/rs/server_inventory_alloc_test.go` | Grouped extraction retains every record without per-NLRI scratch allocation | 58 allocations before versus bound 43; repaired package PASS |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Path identifier | Existing uint32 wire field, no new parser range | Existing framing retained | Short ADD-PATH framing is rejected by `opaqueRouteKey` before reading uint32 | No new numeric limit |
| Route identity | CIDR prefix or registered family key | Mixed ADD-PATH/non-ADD-PATH and VPN/EVPN twins | Default-route misread excluded by AC-8 | Relabel and Compatibility-field variants tested |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-rib-best-addpath` | `test/plugin/show-rib-best-addpath.ci` | One best row, path 2 and MED 10 after both received paths are visible | PASS in parent's current plugin suite, 2.6s; two recorded producer overlays red |
| `labeled-withdraw-compatibility` | `test/plugin/labeled-withdraw-compatibility.ci` | Actual daemon emits a prefix-only delete for Compatibility 0x800000 | PASS in parent's current plugin suite, 1.9s; recorded real-daemon overlay red |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-addpath-best-path-pmacct` | `test/interop/scenarios/bgp-addpath-best-path-pmacct/` | pmacct | Ze's selected path, distinguished by AS_PATH because the Loc-RIB stream omits MED | Accepted AC-7 alternative; green image 270a25262cd6, red image eff6f1c57246 |
| `bgp-labeled-withdraw-compatibility-frr` | `test/interop/scenarios/bgp-labeled-withdraw-compatibility-frr/` | FRR 10.3.1 | Ze's inventory forgets the withdrawn route: no duplicate withdrawal when the injector stops | Recorded red/green; post-repair PASS, image db194029be81 |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/component/bgp/plugins/rib/rib_commands.go` - candidate gathering per prefix across every path id
- `internal/component/bgp/plugins/rib/rib_bestchange.go` - one best record per prefix
- `internal/component/bgp/plugins/rib/storage/peerrib.go`, `familyrib.go` - label side-data keyed per path

## Files to Create
- `internal/component/bgp/plugins/rib/storage/familyrib_opaque.go` — path-id-free opaque storage and retained wire NLRI.
- `internal/component/bgp/plugins/rib/addpath_best_per_prefix_test.go` and `addpath_opaque_best_per_route_test.go` — CIDR and opaque acceptance proofs.
- `internal/component/bgp/plugins/rib/rfc8277_addpath_comparable_test.go` and `rfc8277_same_next_hop_test.go` — received-UPDATE comparability and independent labels.
- `internal/component/bgp/plugins/rs/server_inventory_alloc_test.go` — grouped-extraction allocation regression.
- `internal/test/fixture/register_show_rib_best_addpath.go` and `test/plugin/show-rib-best-addpath.ci` — real command-chain proof.
- `test/plugin/labeled-withdraw-compatibility.ci` — real withdrawal event.
- Both named interop scenario directories and `internal/le/interoplab/bgp/check_labeled_withdraw.go` — third-party observations.

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Existing ADD-PATH capability and RIB commands; no schema changed |
| YANG validation constraints | N-A | No new leaf or numeric configuration |
| YANG custom validators | N-A | No custom config validation added |
| CLI commands/flags | No | `show bgp rib best` output is corrected, grammar unchanged |
| CLI grammar (keyword before value) | N-A | No command registration change |
| Editor autocomplete | N-A | No new command or config value |
| Functional test for new RPC/API | Yes, existing command behavior | `show-rib-best-addpath.ci` drives the real command |
| Pipe completeness | Unchanged | `rib_pipeline_best.go` retains the existing pipeline and terminal |
| Env var registration | N-A | No environment variable added |
| Doctor check for runtime dependencies | N-A | No daemon dependency added; Docker/pmacct/FRR are existing interop harness dependencies |
| Prometheus counters/metrics | No | Corrected route state uses existing RIB observability; no new metric |
| BGP family surface (new SAFI / capability / attribute) | No | All existing families reuse their registered key operations; no new wire capability |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Corrects existing ADD-PATH; no new feature inventory entry or support level |
| 2 | Config syntax changed? | No | No YANG/parser changes; scenario config uses existing capability declarations |
| 3 | CLI command added/changed? | Output only | `docs/architecture/route-selection.md` and `plugin/rib-storage-design.md` describe one best; command grammar unchanged |
| 4 | API/RPC added/changed? | No | Existing `BestChangeEntry` shape; only winner identity is corrected |
| 5 | Plugin added/changed? | Existing plugins | `docs/architecture/core-design.md` documents per-path labels; no plugin registration inventory change |
| 6 | Has a user guide page? | Yes, unchanged syntax | `docs/guide/add-path.md` route identity remains valid; no new configuration |
| 7 | Wire format changed? | Withdrawal interpretation corrected | `docs/architecture/wire/nlri.md` and `api/json-format.md`, anchored to `ParseWithdrawnNLRIs` and `appendNLRIJSONValue` |
| 8 | Plugin SDK/protocol changed? | No new protocol surface | Existing JSON withdrawal fields and path-id preserved; corrected shape documented in `api/json-format.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc8277.md` coverage and removed 2.5-3 gap; independent audits and discrimination records. Generated RFC status is not hand-edited |
| 10 | Test infrastructure changed? | New cases only | `docs/architecture/testing/interop.md` scenario inventory; existing native harness |
| 11 | Affects daemon comparison? | No support-level change | A defect repair within existing ADD-PATH/labeled support; no new advertised capability |
| 12 | Internal architecture changed? | Yes | `rib-storage-design.md`, `route-selection.md`, `rib/unified-locrib.md`, `core-design.md`; source anchors name gather, stored path, label binding and Loc-RIB instance |
| 13 | Route metadata keys added/changed? | No | Existing PathID/AddPath fields now name the winner; replay metadata unchanged |
| 14 | Prometheus counters added/changed? | No | No telemetry definitions changed |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No daemon inventory | Test fixture and interop scenario registration only |
| 16 | Existing source anchors? | Read | Anchors for `rib_commands`, `rib_bestchange`, `pathset`, `familyrib_opaque`, `mpwire`, `prefix_key`, and `server_inventory` checked. Unchanged RPF/injection/policy commands retain their contracts |
| 17 | Existing config/CLI/API examples? | Read | The two `.ci` files and scenario declarations use current grammar; no syntax migration |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **CIDR selection:** per-prefix gather, winner-keyed reads, one best record and Loc-RIB Instance 0; AC-1/2/8/9/10.
2. **Label lifetime:** independent handles, same-id replacement and targeted removal; AC-4/5/6.
3. **Opaque families:** registered route keys, retained wire form and correct withdrawal framing; AC-11.
4. **Reachability and proof:** real show-best `.ci`, accepted pmacct election interop and FRR withdrawal inventory interop; AC-7.
5. **Bounded review repairs:** allocation, purge atomicity, live producer coverage, then independent RFC rejudgment. Review rounds 1–5 below record the actual sequence.

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Every winner-dependent read keys on the winner's (peer, path id); no read uses the triggering UPDATE's bytes for another path |
| Naming | `lost-path-id` / `path-id` step name; JSON `path-id` unchanged |
| Data flow | Selection per prefix in `gatherPrefixCandidatesLocked`; storage keyed (path id, prefix) |
| Rule: no-layering | `bestPrevSet`, the `multi` best-prev store and the prefix-keyed label store are deleted, not kept beside the new shape |
| Rule: performance | No new per-UPDATE allocation on the gather (stack-backed path slice) or the winner key. Review round 1 (I-1) measured the first landing at 5 allocs/op on a CIDR same-best re-run against 2 at `e059baccb8^`: the route-key scratch and the winner and sibling key buffers reached the family key operation (an indirect call) through `PeerRIB.Lookup` and moved to the heap. After the fix the CIDR winner is read by prefix (`storedPath`, `PeerRIB.LookupPath`), the route key is computed only for a non-CIDR family, and the count is back to 2 (one Candidate, one slice), pinned by `TestCIDRSameBestAllocations` |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| One winner per route, independent of path identifier | Read `gatherPrefixCandidatesLocked`, `gatherKeyCandidatesLocked`, `checkRouteBestChange`; named acceptance test PASS lines in current RIB log |
| Independent labels and correct withdrawal identity | Read `pathSet.setLabels/remove`, `RouteKey`, received-UPDATE tests, and current native discrimination records |
| Real user and third-party observations | Read both `.ci` files, fixture/checker source, recorded overlay failures and real-daemon/interop outputs |
| Independent RFC closure | Read `rfc/audit/rfc8277.json` enforced rows 2.5-2/2.5-3/3.1-1 and remove only AC-5 blockers |
| Durable architecture and honest remaining scope | Source-anchor review of the named pages; preserve baseline RFC weaknesses rather than claim general conformance |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Short or malformed received keys | `opaqueRouteKey` bounds the four-byte identifier; registered split/key operations frame withdrawals before storage access |
| Cross-session route confusion | Prefix/route-key gathering never asks a peer with another session's framed key; mixed-mode tests assert isolation |
| Label lifetime and concurrent purge | `upsert` retains old binding until `setLabels`; `withdrawIfUnheld` shares the election shard lock |
| Buffer ownership and resource growth | rs records and CIDR scratch share one pooled owner; records copy retained wire strings before forwarding; grouped test checks every record |
| Injection, authorization, secrets and external dependencies | No new daemon shell, file path, credential, privilege or authorization surface; fixed-address container queries remain test-only |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Review Round 1 (2026-10-03)

| ID | Finding | Disposition | Evidence |
|----|---------|-------------|----------|
| B-1 | A VPN withdrawal was gathered with the announcement framing and its stored best keyed with the withdrawal framing, so a Compatibility field of 0x800000 or 0x000000 withdrew the route another PE still carried | Fixed: `checkRouteBestChange` computes the key once and gathers by it (`gatherPrefixCandidates`, `gatherKeyCandidates`); `rfc/short/rfc8277.md` coverage and `rib-storage-design.md` corrected | `TestVPNWithdrawPromotesTheOtherPE`, both subtests red at HEAD (best records 0, want 1), green after |
| B-2 | Four RFC-tagged rib test files changed by the ADD-PATH commits without approval | Owner approved retroactively, 2026-10-03, "Approve": `bestpath_test.go`, `rfc4271_rib_bestchange_test.go`, `rib_test.go` and `rfc8277_test.go`, whose old assertions pinned the per-path-id defect. Recorded with `./le rfc approve` for each changed unit. b28b95a85d carries no `RFC-approved:` trailer: `./le commit create` attaches a row only for a unit the commit itself changes, and these units changed in earlier commits. `./le commit audit` lists the four files; that is the tooling defect journaled in `plan/journal/check-cannot-see-the-change-it-looks-for.md` (naming spec, round 3 N-3). No workaround applied | The approval rows in `tmp/commit-rfc-approved-1d41415e.md`; corrected in round 2 (R2-N2) |
| I-1 | Election buffers escaped to the heap, 5 allocs/op (6 with a second ADD-PATH peer) against 2 at base | Fixed: prefix-typed winner reads, branch-local route-key scratch | `TestCIDRSameBestAllocations` red at HEAD (5 allocs/op in both subtests), green after (2); `-gcflags=-m` no longer moves `winnerBuf` or `siblingBuf`, and `keyScratch` only on the non-CIDR branch |
| I-2 | `TestRFC8277AddPathRoutesOnOneSessionAreComparable` claimed MED decided, but MED was never compared (eBGP, empty AS_PATH) and path 7 won on the tie-break | Fixed, owner approved 2026-10-03 ("Approve"): iBGP session, path 7 MED 20 and path 9 MED 10, asserts path 9 | Green on the real code; red under the MED-inversion mutant (`a.MED > b.MED` in `comparePair`: MED 20 and path 7 win under both keys), which the earlier version survived. Discrimination record re-recorded by `./le rfc discriminate-record` for the new claim (revert of `gatherPrefixCandidatesLocked`) |
| I-3 | `emitPurgedWithdraws` published Withdraw and removed the Loc-RIB entry before re-electing, so a route with a survivor went Withdraw then Add | Fixed: the purge publishes nothing; the re-election runs first, a survivor is one Update, and only a route with no candidate is withdrawn and leaves the Loc-RIB | `TestPurgeBestPrevForPeerAddPath` (untagged) asserts one Update to path 3: red at HEAD (two changes), green after |
| I-4 | Under ADD-PATH `upsert` released a replaced path's label inside the insert lock, and `SetLabelsIfRouteExists` rebound it under a second lock | Fixed: `upsert` keeps the binding, and `setLabels` alone replaces and releases it, as the label store without ADD-PATH already did | `TestADDPathRelabelNeverUnbindsTheLabel` red at HEAD (about 19,990 unbound reads in 20,000 relabels), green after, also under `-race` |
| I-5 | The performance row recorded "no new per-UPDATE allocation" without a measurement | Fixed: the row carries the measured counts | Critical Review Checklist, performance row |
| N-1 | Some discrimination records use a non-behavioural break | Open: re-record behaviourally where `./le rfc discriminate-record` allows | The I-2 record stays on the revert route: the mutant route needs a gomu report holding the mutant as KILLED, and none exists for `bestpath.go` |
| N-2 | `newBestSource` keyed a CIDR election on its raw wire prefix | Fixed: a typed key on the parsed, masked `netip.Prefix`. Storage already canonicalises the keys it hands back, so no test goes red on the old form and none was added | `rib_pipeline_best.go`, `electionKey` |
| N-3 | Memory note | Noted | |
| N-4 | Accepted by the reviewer | Accepted | |

## Review Round 2 (2026-10-04)

| ID | Finding | Disposition | Evidence |
|----|---------|-------------|----------|
| R2-B1 | `wirePath`, `lookupSRv6SIDForBest` (`rib_bestchange.go`), `gatherCandidates` and `gatherFramedCandidates` (`rib_commands.go`) had no production caller, so the tests that drove them proved wrappers the daemon never runs | Fixed, owner approved 2026-10-03 ("Approve"): the four are deleted. A test gathers through `gatherCandidatesLocked` under `r.peerMu.RLock` (`gatherCandidatesHeld`, `rib_test.go`), the gather the show pipeline runs; the SRv6 tests name the stored path through `candidatePath` and ask `storedPathSRv6SID`, as `checkRouteBestChange` does (`storedSRv6SID`). Assertions unchanged. Twelve tagged units approved with `./le rfc approve` and carried as `RFC-approved:` trailers; the 16 discrimination records of the five units that had one re-recorded on their existing revert routes | d029f58129; `go test -race` over the touched tests green |
| R2-I1 | `emitPurgedWithdraws` checked "no best recorded" (`holdsBestPrev`) and removed the route from the Loc-RIB under two separate holds, so a concurrent election could record and mirror a best in between and the removal deleted it | Fixed: `withdrawIfUnheld` checks and removes under one hold of the bestPrev shard lock, the lock `checkRouteBestChange` records and mirrors under; `holdsBestPrev` deleted. `purgeRemoveHook` (production nil) lets a test land an election at the removal | `TestPurgeWithdrawKeepsABestElectedConcurrently` red before the fix ("the Loc-RIB holds the best the bgp-rib records": false), green after, and green `-race -count=30` |
| R2-I2 | `emitPurgedWithdraws` held every route of a family until the family's last election, then published them in one batch | Fixed: each route is published as soon as its election answers, one batch per route; a survivor stays one Update | `TestPurgePublishesEachRouteAsItIsElected` red before the fix ("should have 2 item(s), but has 1"), green after. The ordering against a concurrent later UPDATE is not asserted: an UPDATE publishes after its own election outside the shard lock, so the window is the same one two UPDATEs share |
| R2-I3 | `ai/digests/rib.md` said the purge emits withdraws and calls `r.locRIB.Remove`; `docs/features/srv6.md` named `lookupSRv6SIDForBest` | Fixed: both corrected, and `rib-storage-design.md` names `withdrawIfUnheld` and the per-route publication | d029f58129 (srv6.md), c6271f93e5 (digest, design page; also the R2-I1 and R2-I2 fixes and tests) |
| R2-N1 | A non-CIDR `candidatePath` allocates a framed NLRI on every election, same-best included | Recorded, not fixed: the framed NLRI is not only the emitted entry's. `bestCandidateNextHopAddr`, `storedPathSRv6SID` and the blackhole lookup ask the stored path by it before the same-best test, and `PeerRIB.Lookup` hands it to the family's registered key operation, an indirect call, so a stack buffer moves to the heap too (the `storedPath` comment). Removing it means reading the winner's entry from the gather rather than looking it up again, a change to every winner read | |
| R2-N2 | The B-2 row said the approvals were carried as `RFC-approved:` trailers | Corrected here: b28b95a85d carries no `RFC-approved:` trailer (`git log -1 --format=%B b28b95a85d`: 0), because `./le commit create` attaches a row only for a unit the commit changes. The audit lines for the four files are the tooling defect journaled in `plan/journal/check-cannot-see-the-change-it-looks-for.md` | |
| R2-N3 | `insertPoolNLRIs`/`removePoolNLRIs` (`rib.go`) have no SAFI 4 branch | Journaled: `plan/journal/silent-fall-through.md`, 2026-10-04 row; no existing row covered it | |

## Review Round 3 (2026-10-04)

| ID | Finding | Disposition | Evidence |
|----|---------|-------------|----------|
| R3-B1 | No functional or interop test drives a SAFI 4 withdrawal with Compatibility 0x800000 through a real daemon | Fixed: the `.ci` landed in 3fd196d06e (tag fn-addpath-b1-ci); the interop scenario `bgp-labeled-withdraw-compatibility-frr` (tag fn-addpath-b1-interop) announces 10.10.0.0/24 (label 100) and 10.11.0.0/24 (label 101), withdraws 10.10.0.0/24 with 0x800000, then stops the injector and requires FRR to log ONE withdrawal of 10.10.0.0/24 after bgp-rs's peer-down withdrawals. bgp-rs relays the withdrawal bytes unchanged, so the discriminating observable is the route inventory Ze's reading feeds | Interop green image sha256:bef721d155cb, red under the `MPUnreachWire.NLRIs` -> `ParseNLRIs` overlay image sha256:fef5fcdd2ebb ("after the injector stopped: FRR logged 2 withdrawals of 10.10.0.0/24 from 172.30.0.2, want 1"), restored green image sha256:a1af446febe2. The first single-prefix design passed under the same red and was discarded |
| R3-I1 | The route server keyed a labelled announcement by its hex (`appendOpaqueRecords`) and its INET withdrawal by `"10.0.0.0/8"` (`appendParsedRecords`), so the withdrawal never cancelled it | Fixed: both arms of a family `nlrisplit.KeysByCIDR` answers for are keyed by the `RouteCIDR` prefix and the Path Identifier (`recordKey`); the set's value (`withdrawalEntry`) keeps the latest announcement's hex for the peer-down withdrawal, so a relabel replaces it. The 14fc101e1d journal row's route server sentence corrected | Reviewer probe `scratch/mut/rsprobe.log` red at HEAD (2 entries after withdrawing one of two); `TestLabeledWithdrawalLeavesTheRouteServerSet` and `TestLabeledRelabelReplacesTheRouteServerEntry` green; record RFC8277-2.4-1 positive (revert `opaqueRouteCIDR`) |
| R3-I2 | No labelled withdrawal test prefix had the low bit of its last octet set, so mutant M4 (the `!isINET` guard in `appendNLRIJSONValue` disabled) survived | Fixed: `TestLabeledWithdrawalEventNeverReachesTheLabelDecoder` withdraws 10.1.3.0/24 | Red under M4 (`event lacks ... 10.1.3.0/24`, rendered as a label stack), green on the real code; record RFC8277-2.4-1 positive |
| R3-I3 | `json-format.md` had no withdrawal shape for labelled unicast; `nlri.md` said the default arm wraps the whole section in one `WireNLRI` and said nothing of the withdrawal framing | Fixed: both pages | |
| R3-I4 | Two RFC8955-6-5 records no longer verified (producer changed) | Re-recorded: the negative on `comparePair`; the positive on `checkRouteBestChange`, because the unit no longer executes `checkBestPathChange` (the tool refused it). STALE audit verdicts are left for an independent re-judge | `scratch/r3-records.log`, `r3-records2.log` |
| R3-N1 | `withdrawIfUnheld` created the family's shards to answer for a family that never elected | Fixed: `familyShards(fam, false)`; no shards answers "not held" with no removal | rib/... `-race` green |
| R3-N2 | `wrapNLRI` allocated a byte slice per withdrawn NLRI | Fixed: `ParseINET` over a stack array, and one scratch per section, since `RouteCIDR`'s indirect call moves it to the heap. `NewINET` was not used: it reports `HasAddPath` false, and the event then loses the `path-id` (`TestLabeledWithdrawalEventNamesThePrefix`) | `-gcflags=-m`: only the section scratch moves to the heap |
| R3-N3 | `test/weakened/1d41415e.md` miscounted the assertions of `TestPurgeBestPrevForPeerAddPath` | Fixed: four assertions became three, and the departed path id assertion named | |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bgp-addpath-best-path-per-prefix-01a10694-3795-71f2-8250-25e3a877f4cf.md`; owned source/test/doc/evidence population recorded by the native tool |
| `./le spec review check` | `review_gate: OK (62 code files, clean, hashes match)`; only this record's prose changed afterward and the native artifact was refreshed |
| Rounds | 5; no sixth round requested or authorized |
| Reviewer lenses used | Protocol/data flow, guards/security, test discrimination, allocation/style; independent closure traces acceptance and documentation producers |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| R1-B1/I1–I4 | BLOCKER/ISSUE | Withdrawal identity, CIDR escapes, MED-vacuous proof, transient withdrawal and unbound labels | RIB gathering and pathset | Round 1 source repairs and red/green tests |
| R2-B1/I1–I3 | BLOCKER/ISSUE | Dead wrappers, purge check/remove race, delayed publication and stale docs | RIB gathering and purge | Live producers, `withdrawIfUnheld`, per-route publication and docs |
| R3-B1/I1–I4 | BLOCKER/ISSUE | Missing end-to-end proof, mismatched rs keys, weak decoder proof and stale records | Inventory, format and interop | CIDR/path-id identity; `.ci` and FRR proof; discriminator renewals |
| R4-I1 | ISSUE | Escaping CIDR scratch per NLRI | `server_inventory.go::opaqueRouteCIDR` | Existing pool holder owns scratch; grouped allocation regression |

### Round 4 scope (2026-10-04)

Review commits `312da72376`, `3fd196d06e`, `196261ed14`, and `e27607c82b`,
plus the sibling call sites affected by those fixes. Two independent readers
cover the production withdrawal path and its functional/interop proof. Each
applies the protocol, guard, and test-proof lenses from `/ze-review`.
Round 4 found one ISSUE: `opaqueRouteCIDR` allocated scratch per NLRI,
including unsupported non-CIDR families. Two readers found no other
patch-introduced BLOCKER or ISSUE. The grouped extraction regression failed
at 58 allocations against a bound of 43 for both labeled and BGP-LS NLRIs
(`job-inventory-alloc-regression-before-c959a3f9.log`). The existing pool now
owns the scratch; the complete rs package passes under `-race`
(`job-inventory-alloc-after-6204bbaa.log`).

### Round 5 scope (2026-10-04)

Review only the pooled inventory scratch repair and migrated callers, plus
the RFC audit's same-next-hop label proof and owner-approved strengthening
of `TestLabeledRoutesWithDifferentLabelsAreComparable`. The owner selected
"Strengthen the proof": iBGP so MED is compared, one stored best path, and
preserved label-swap assertions. The approval is recorded by `./le rfc approve`
in this session. Independent readers `AddPathProductionReview` and
`AddPathProofReview` finished with **0 BLOCKER, 0 ISSUE**. The latter also
re-judged the strengthened RFC 8277 claims. Closure checked the conclusions
against `extractWireNLRIRecords`, migrated callers, `pathSet.setLabels`,
received-UPDATE assertions and the semantic-red logs.

Closure found one documentation-only NOTE: the storage page still said
`upsert` released a replaced label. It now names `setLabels` and explains why
`upsert` retains it. This repairs the record of round 1, not product behavior.

Security/style disposition: no new peer-reachable panic, privilege boundary,
secret or daemon external command. The extraction owner has paired lifetime
comments, fixed scratch and wire-bounded iteration. Its allocation test checks
every output record. Fixed container addresses and keepalive-based timing
remain test-only journaled NOTEs, not a reason for a sixth round.

## Design Insights

The storage design separates route identity, received-path identity and
forwarding labels. An interop proof must observe Ze's decision, not a
downstream daemon independently choosing the same route.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Registered key plus per-path wire form | Key every announcement byte | Relabel and withdrawal find one route; replay preserves received fields |
| One Loc-RIB BGP instance | One instance per identifier | Avoid re-ranking the BGP winner by distance and MED |
| pmacct observes Loc-RIB | Observe FRR's downstream best | bgp-rs relays without selecting; Loc-RIB exposes Ze's election |

## Known Limitations

This spec does not claim full RFC 8277 or RFC 4271 conformance.
RFC8277-2.4-1 and RFC4271-9.1.2.2-1 remain weak. The pre-existing
three-candidate MED defect was reproduced and journaled, not added to this
identity repair. Prefix-SID D1–D5 and SRv6 eligibility retain their original
scope. Opaque best-change events do not add VRF-capable Loc-RIB installation.
The recorded non-CIDR allocation and test timing/address NOTEs do not hide
omitted ADD-PATH acceptance work. R-1's consumer contract is preserved;
R-2's transient peer-down withdrawal is fixed.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Final Linux race `./le verify worktree` is owed to the parent after ADD-PATH, link-local D6 and startup-race closures, as the handover orders. `ai/rules/pre-release.md` forbids rerunning passed gates merely to permit a commit.
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented

- One prefix or opaque-route-key election across every received path.
- Winner-keyed forwarding data; one CIDR BGP Loc-RIB Instance 0.
- Independent labels, including same-next-hop replacement and withdrawal.
- Registered withdrawal framing and route identity, with per-path wire form.
- Peer-down survivor promotion and atomic held-best check/removal.
- Real command, withdrawal-event and interop proofs; pooled inventory scratch.

### Bugs Found/Fixed

Rounds 1–5 preserve each finding and source fix. The principal regressions
are covered by the CIDR and opaque acceptance tests,
`TestPurgeWithdrawKeepsABestElectedConcurrently`,
`TestADDPathRelabelNeverUnbindsTheLabel` and
`TestInventoryGroupedExtractionAllocations`.

### Documentation Updates

Implementation updated `docs/architecture/plugin/rib-storage-design.md`,
`route-selection.md`, `rib/unified-locrib.md`, `core-design.md`,
`wire/nlri.md`, `api/json-format.md` and `testing/interop.md`. Their anchors
name the storage, election, withdrawal and interop producers. Closure
corrected the storage page's stale `upsert` release claim.
`rfc/short/rfc8277.md` records coverage and removes the 2.5-3 gap; generated
RFC status was not hand-edited. `docs/features/srv6.md` discloses the separate
eligibility limitation. Parent owns final doc/structural checks; none reran here.

### Deviations from Plan

- AC-7 now states the already accepted pmacct Loc-RIB alternative. Ze does
  not advertise its selected best over BGP; an FRR receiver proves its own
  selection instead. The acceptance decision predates closure.
- VPN/EVPN forwarding labels and Compatibility-field framing had to leave
  route identity too; removing only the identifier could not fix relabels.
- Same-next-hop label cases and opposing fallback preferences strengthen the
  proof. Owner approval is in `tmp/commit-rfc-approved-de830ef0.md`; no
  assertion was weakened.

## Mistake Log
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | Original AC-7 proposed observing Ze's best over BGP | bgp-rs relays; bgp-rib does not re-advertise | Source and accepted collector alternative | A-2 broken; pmacct red/green proves the election |
| approach | MED tests could pass on a later tie-break | The intended decision must oppose the fallback | MED-inversion/skipped-MED overlays | iBGP fixtures, opposing preferences, exact winner |
| approach | Local scratch escaped through a registered decoder | Grouped extraction paid per NLRI | 58 allocations versus bound 43 | Existing pool owns scratch; every output remains asserted |
| approach | Storage prose retained the old release point | `upsert` retains; `setLabels` replaces/releases | Closure source read | Correct governing paragraph; no new product round |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| One best across paths of a route | Done | `gatherPrefixCandidatesLocked`, `gatherKeyCandidatesLocked`, `checkRouteBestChange` | CIDR and opaque; received identifiers preserved |
| Independent path labels | Done | `pathSet.setLabels/remove`, `setRouteNLRI` | Same-id replacement, different-id preservation |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRFC8277AddPathRoutesOnOneSessionAreComparable` | Both keys gather both paths; MED opposes path-id rank |
| AC-2 | Done | `TestAddPathUnicastOneElectionPerPrefix` | Plain IPv4, one stored best |
| AC-3 | Done | Tagged comparability test and native discrimination | Historical red renamed, not discarded |
| AC-4 | Done | `TestRFC8277AddPathLabelsBoundPerPath`, `TestRFC8277AddPathSameNextHopKeepsIndependentLabels` | Exact labels after rebind and withdrawal |
| AC-5 | Done | Audit 3.1-1/2.5-2, discrimination and BGP child edit | Enforced; only those blockers removed |
| AC-6 | Done | Audit 2.5-3 and both polarities | Gap removed; independently enforced |
| AC-7 | Changed, accepted | `bgp-addpath-best-path-pmacct` red/green | Accepted alternative, not closure scope reduction |
| AC-8 | Done | `TestAddPathMixedModeKeyNeverReadsAsAnotherPrefix` | 0/0 never enters the 10/8 election |
| AC-9 | Done | `TestAddPathLocRIBHoldsOneBGPPath` | LOCAL_PREF winner, one BGP instance |
| AC-10 | Done | `TestAddPathWithdrawalKeepsOrPromotes`, `TestPurgeBestPrevForPeerAddPath` | Survivor and last-path behavior |
| AC-11 | Done | Three `TestAddPathOpaque*` tests | VPN/EVPN election, mixed-mode, withdrawal twins |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| CIDR, opaque and labeled path proofs | Done | RIB tests named in TDD table | Current package PASS under race detector |
| Grouped inventory allocation | Done | `server_inventory_alloc_test.go` | Pre-fix red; repaired rs package PASS |
| Command and withdrawal event | Done | Both named `test/plugin/*.ci` | Current suite PASS |
| Third-party election and inventory | Done | pmacct/FRR scenarios | Real images and producer-overlay reds |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Planned RIB/storage producers | Done | Gather, one best, independent data |
| Added tests and fixture | Done | Source assertions and current PASS lines read |
| Interop scenarios/checkers | Done | Registered checker, third-party observations |
| Docs, RFC evidence, blocker/citer edits | Done | Claims match producers; historical citation preserved without a live link |

### Audit Summary
- **Acceptance items:** 11.
- **Done:** 10 as originally stated.
- **Changed:** AC-7, previously accepted alternative, implemented and proven.
- **Partial:** 0.
- **Skipped:** 0.

## Goal Validation (BLOCKING)

`S` denotes `tmp/session/2026-10-04-01a10694-3795-71f2-8250-25e3a877f4cf/scratch`.
`P` denotes `tmp/session/2026-10-03-ff3776cb-ce05-4491-9d8f-ba31bcc36641/scratch`.
These are existing execution logs, not checks run by the closure owner.

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| One election per prefix | Unit, actual command, third-party decode | `S/job-addpath-rib-after-bbc3c433.log`: named CIDR/comparability PASS; `S/functional-plugin-full.log`: show-best PASS 2.6s; `P/p4-interop-green5.log`: 1 passed, 0 failed, image `sha256:270a25262cd6fecea95feea633577f1aeb0405db201059047c0feb6ef037a911` |
| Election proof discriminates | Rebuilt overlay interop | `P/p4-interop-red.log`: assertion 4 finds forbidden AS_PATH `65004 65050`, exit 1, image `sha256:eff6f1c572461ac942ce750870accaf3fc1c5d4ea5cef634023a1ef8230f05b0` |
| Labels stay on their path | Same-next-hop test and semantic breaks | Current test PASS; `S/job-label-first-path-red-8d76bdc3.log` loses path 7 label; `S/job-label-retain-old-red-b0e822ad.log` retains obsolete path 9 label |
| Withdrawal uses route identity | Daemon event and FRR inventory | Current withdrawal `.ci` PASS 1.9s; parent post-repair FRR PASS 165.93s, image `sha256:db194029be810a19b102767aec29bb9bdc243a53292e3b5d2f4e3bf106516863`, peer `quay.io/frrouting/frr:10.3.1` |

## Work Not Done
| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None of the agreed product scope | AC-1–AC-11 are implemented and demonstrated, including accepted AC-7 | N-A; no in-scope item transferred |

Final cross-spec Linux race verification remains an ordered parent task, not
missing implementation. Existing MED, Prefix-SID and reflector limitations
retain their audit/journal records and original scopes.

## Pre-Commit Verification

### Files Exist (source reads)
| File | Exists | Evidence |
|------|--------|----------|
| `familyrib_opaque.go`, CIDR/opaque tests, added RFC8277 tests | Yes | Declarations, received-UPDATE calls and exact assertions read |
| `server_inventory_alloc_test.go` | Yes | Grouped extraction and all-record checks read |
| Command fixture and both named `.ci` files | Yes | Registration, receipt barrier and daemon assertions read |
| Both scenario directories and labeled checker | Yes | Registry, checker and real scenario evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1/2/3 | Paths meet in one election | Current tagged comparability and IPv4 PASS; gather producer appends every stored path |
| AC-4 | Labels are per path | Same-next-hop PASS; clobber/stale-label semantic probes red |
| AC-5/6 | Required verdicts enforced | Native-stamped audit, current records; only AC-5 blockers removed |
| AC-7 | Third-party election | Actual pmacct red/green logs read, distinct rebuilt images |
| AC-8/9/10 | Mixed mode, Loc-RIB, promotion | Named current PASS lines; winner/Instance 0 producers read |
| AC-11 | VPN/EVPN twins | All three opaque tests PASS for both families |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Received ADD-PATH → show best | `test/plugin/show-rib-best-addpath.ci` | Receipt barrier; exactly one row, path 2, MED 10; current PASS |
| Labeled MP_UNREACH → plugin event | `test/plugin/labeled-withdraw-compatibility.ci` | Literal Compatibility 0x800000, prefix-only delete; current PASS |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Existing event contract, one Instance 0 path and peer-down promotion |
| A-2 | broken | No best-path BGP re-advertisement; accepted, discriminating pmacct alternative |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Per-route gather and winner identity | `gatherPrefixCandidatesLocked`, `gatherKeyCandidatesLocked`, `candidatePath`, `newBestSource` | Architecture claims read against producers |
| Label lifetime | `pathSet.upsert/setLabels/remove` | Governing paragraph corrected |
| One Loc-RIB BGP instance | `checkRouteBestChange` writes `bgpLocRIBInstance` | Unified Loc-RIB/core-design agree |
| Withdrawal framing and JSON | `MPUnreachWire.NLRIs`, `ParseWithdrawnNLRIs`, exact fixture assertions | Wire/JSON pages describe the corrected boundary |
| Interop observation | pmacct Loc-RIB AS_PATH; FRR post-peer-down withdrawal count | Inventory describes real discriminators |
| Config, command, inventory, metrics | Owned population adds no YANG/daemon command/capability/metric | Existing syntax/support levels retained |

### Execution Status and Remaining Gates

- Complete RIB and rs packages pass under `-race`; closure reran none.
- Parent plugin suite: **718 PASS, 38 FAIL, 111 SKIP**, 867 discovered,
  192.6s. Both owned functional cases pass. Parent attributes 34 reds to
  missing host loopback aliases; other fixtures are `evpn-config-self`,
  `llgr-peer-stale-time-drives-timer`, `rfc4271-partial-unknown-transitive`
  and `role-otc-egress-filter`.
- Integrated RFC check: **111 unrelated residual violations**, no assigned
  ADD-PATH audit/discrimination stale. Evidence: `S/rfc-integrated-full.log`.
  This does not claim a green repository gate.
- Parent documentation gate exits 1 with 29 unrelated published catalog
  drift findings; `plan/journal/stale-artifact-reused.md` records them.
  The changed source anchors' documentation indexes were regenerated.
- Closure ran no build, lint, test, formatter, RFC or documentation gate.
  Native review record/check only binds the source review to its population.
- Parent owns structural/doc checks, scoped A/B commits, the remaining two
  closures, then final Linux race verification. No push or spec removal is
  authorized by this record alone.

## Core Insight

A path identifier and label describe a path; neither creates another election.
The durable design states the identity boundary and the label lifetime that
lets concurrent readers observe it safely.
