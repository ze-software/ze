# Spec: ADD-PATH Best Path Elected per Prefix, Labels Held per Path

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 3/4 |
| Handoff | - |
| Updated | 2026-10-03 |

<!-- Not claimed: the implementing session (ff3776cb) holds another claim
     (plan/spec-rfc-test-file-naming.md). Thomas authorised the fix agent and
     the scope (ALL ADD-PATH families in one spec) on 2026-10-03. -->

## Design (chosen 2026-10-03, Option A)

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Select once per prefix; storage stays keyed (path id, prefix); `PeerRIB.AppendPrefixPaths` appends every path of a prefix into a caller-owned (stack-backed) slice | Keep per-path-id selection and compare across ids afterwards | RFC 8277 Section 3.1 and RFC 7911 Section 2: the id names a path, it never partitions selection. FRR `bgp_best_selection` walks every `pi` of the dest; BIRD `rte_recalculate` walks the net |
| Each peer is asked by PREFIX, never with another session's wire key | Re-frame the key per peer | Closes the cross-mode defect: an ADD-PATH key `00000007 08 0a` parsed as `0/0` in a non-ADD-PATH peer (store.NLRIToPrefix ignores trailing bytes) |
| `Candidate` gains `PathID` and `AddPath`; every winner-dependent read (next hop, labels, SRv6 SID, blackhole, show-best row) goes through `candidateNLRI` to the winner's own key | Re-read with the triggering UPDATE's key | The triggering UPDATE may be another path or another peer |
| `bestPrevSet` and the `multi` store deleted; one `bestPrevRecord{rec, pathID, addPath}` per prefix; the best-change `PathID`/`AddPath` are the WINNER's | Keep both stores (layering, banned) | One best per prefix; record grows 8 to 16 bytes per prefix (footprint bench 27.5 to 35.5 heap bytes/entry), the ADD-PATH per-prefix heap slice is gone |
| Loc-RIB mirror Instance 0 (`bgpLocRIBInstance`), `locrib.Path.Instance` comment corrected | Instance = path id (old code, contradicted the comment) | Two BGP paths in the Loc-RIB would be re-ranked by distance and MED alone, overriding RFC 4271 |
| Final tie-break: lowest path id (`BestStepPathID`, `lost-path-id`) | Storage order | Ze choice, documented as not RFC text: deterministic election |
| Peer-down: `emitPurgedWithdraws` re-elects each purged route from the remaining paths | Leave the route withdrawn until the survivor's next UPDATE | Per-prefix records made the existing gap reachable for ADD-PATH (the survivor path no longer had its own record) |
| Labels: label handle per path (in `pathEntry` under ADD-PATH), released in `pathSet.remove`/`upsert` | Parallel per-(id,prefix) BART | One store per fact; `sizeof_test` guards `RouteEntry`, so the handle lives in `pathEntry` |
| Opaque-key families (VPN, EVPN, ...): strip the path id from the opaque key, add a per-path value layer, same per-key selection | Leave keyed by full bytes | Same RFC 8277 Section 3.1 defect for VPN (RFC 8277 labels in RFC 4364 NLRI) and every other family |

R-1 audit (2026-10-03): no consumer outside `bgp-rib` reads `BestChangeEntry.PathID`
(grep of every importer of `ribevents.BestChange*`: sysrib, fib kernel/vpp/p4,
bmp_locrib, redistribute, flowexport, static, isis, fakefib). The ADD-PATH send
side keys on sent UPDATEs (`ribOutKey`, `rib.go`), not on best-change. R-1 holds.

### Phases

| Phase | Content | State |
|-------|---------|-------|
| 1 | CIDR families: per-prefix gather, one record per prefix, winner-keyed reads, Loc-RIB Instance 0, tie-break, peer-down re-election, docs | done, committed |
| 2 | Labels per path (AC-4, RFC8277-2.5-2 same-id case, RFC8277-2.5-3 and its `{gap}` removal): handle in `pathEntry` under ADD-PATH, released in `pathSet.upsert`/`remove`/`releaseAll`, `pathSet.refresh` keeps it; `TestRFC8277AddPathLabelsBoundPerPath` replaces the gap pin, red against HEAD storage by overlay, records for 2.5-2 and 2.5-3; `gatherCandidates` split into `gatherCandidates` and `gatherFramedCandidates` | done, committed |
| 3 | Opaque-key families (VPN, EVPN, ...): path id out of the key, per-path value layer, VPN and EVPN twins | open |
| 4 | Functional `.ci` (`show rib best` one best for two path ids), interop (extend `bgp-addpath-frr` or the rail-agreement pattern) with revert-rebuild-red recorded | open |

### Added acceptance criteria

| AC ID | Input / Condition | Expected Behavior | Evidence |
|-------|-------------------|-------------------|----------|
| AC-8 | Non-ADD-PATH peer holds 0.0.0.0/0, ADD-PATH peer holds 10/8 path 7 | 0/0 is never a candidate for 10/8 | `TestAddPathMixedModeKeyNeverReadsAsAnotherPrefix` |
| AC-9 | Path 7 LP 200 MED 50, path 9 LP 100 MED 10 | One BGP Loc-RIB path for the prefix, path 7's | `TestAddPathLocRIBHoldsOneBGPPath` |
| AC-10 | Withdraw non-best, then best | Non-best: no change; best: path 9 promoted (Update, PathID 9); last: Withdraw naming 9 | `TestAddPathWithdrawalKeepsOrPromotes`, `TestPurgeBestPrevForPeerAddPath` (peer-down) |
| AC-11 | VPN and EVPN twins of AC-1 | One election per NLRI-without-path-id | phase 3 |

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

## Evidence

| Item | Value |
|------|-------|
| Failing probe | `internal/component/bgp/plugins/rib/rfc8277_addpath_comparable_red_test.go`, `TestRFC8277AddPathRoutesOnOneSessionAreComparable` (untracked, untagged, red on purpose) |
| Shape | one ADD-PATH peer sends 10.0.0.0/8 as path 7 (label 100, MED 10) and path 9 (label 200, MED 20). Under either path key the candidate set must hold 2 routes and best-path selection must pick MED 10. Today each key holds 1 candidate, and under path 9's key the MED 20 route is elected |
| Producer, selection | `RIBManager.gatherCandidatesLocked` (`internal/component/bgp/plugins/rib/rib_commands.go`) looks up each peer's RIB with the path-id-keyed NLRI; the best-change store in `internal/component/bgp/plugins/rib/rib_bestchange.go` keeps a per-prefix path-id to record map (`bestPrevSet`), one best per path id |
| Producer, labels | `PeerRIB.SetLabelsIfRouteExists` and `PeerRIB.RemoveLabels` (`internal/component/bgp/plugins/rib/storage/peerrib.go`) drop the path id; `FamilyRIB.SetLabels` (`internal/component/bgp/plugins/rib/storage/familyrib.go`) holds one handle per prefix |
| Author record | `plan/handover/rfc-verdict-test-fix-pass/bgp/c28-author.md`, rows RFC8277-3.1-1 and RFC8277-2.5-2 |
| Requirements | RFC8277-3.1-1 (weak), RFC8277-2.5-2 (weak), RFC8277-2.5-3 ({gap}) in `rfc/short/rfc8277.md`; the first two are Blocked-by rows in `plan/pre-release/spec-rfc-verdict-fix-bgp.md` |

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
- [ ] `docs/architecture/<doc>.md` - [why relevant]
  → Decision: [specific architectural decision that constrains this spec]
  → Constraint: [specific rule from the doc that applies here]

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc8277.md` - rows 2.5-2, 2.5-3, 3.1-1
  → Constraint: [specific RFC rule that applies here]
- [ ] `rfc/short/rfc7911.md` - path identifier semantics, best route SHOULD
  → Constraint: [specific RFC rule that applies here]

**Key insights:** (minimal context to resume after compaction)
- [insight from docs]

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - [what it currently does]
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - [what it currently does]
- [ ] `internal/component/bgp/plugins/rib/storage/peerrib.go` - [what it currently does]
- [ ] `internal/component/bgp/plugins/rib/storage/familyrib.go` - [what it currently does]

**Behavior to preserve:** (unless the user explicitly said to change it)
- [output format, function signature, or `.ci` expectation callers depend on]

**Behavior to change:** (only what the user asked for)
- [list, or "None - preserve all existing behavior"]

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A received UPDATE reaches `bgp-rib` as a structured event (`handleReceivedStructured`, `rib_structured.go`), one NLRI at a time, with the session's ADD-PATH flag for the family
- NLRI wire bytes: `[path-id:4][prefix-len:1][prefix]` under ADD-PATH, `[prefix-len:1][prefix]` otherwise

### Transformation Path
1. The route is stored in the peer's Adj-RIB-In keyed (path id, prefix) (`FamilyRIB.Insert`, `pathSet` under ADD-PATH)
2. `checkBestPathChange` parses the prefix and gathers every path of it from every peer (`gatherPrefixCandidatesLocked`, `PeerRIB.AppendPrefixPaths`)
3. `SelectMultipath` elects one best; winner reads go through `candidateNLRI`; one `bestPrevRecord` per prefix; Loc-RIB Instance 0; one best-change per prefix naming the winner's path id

Design documents changed with the code: `docs/architecture/plugin/rib-storage-design.md` (per-prefix record, gather by prefix, Loc-RIB instance, peer-down re-election), `docs/architecture/route-selection.md` (ADD-PATH paragraph, `lost-path-id` step), `docs/architecture/rib/unified-locrib.md` (BGP Instance 0).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | [JSON format, command syntax] | No |

### Integration Points
- [Existing function/type this connects to] - [how it integrates]

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

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
| A-1 | [what this design assumes] | [where the assumption comes from] | [impact on design] | [test/grep/user confirmation] | unvalidated |

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
| How is it reverted? | [single commit revert / needs config migration / not revertible once peers see it] |
| Who else touches this path? | [other plugins, components, or specs working the same files] |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [config/CLI/event that triggers it] | → | [function that actually runs] | [test name proving the chain] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | One ADD-PATH session sends one prefix as path 7 (MED 10) and path 9 (MED 20) | Both paths are candidates of one selection for the prefix, and MED 10 is best, whichever path key the lookup starts from |
| AC-2 | Same as AC-1 for a family without labels (plain IPv4 unicast with ADD-PATH) | Same single election per prefix: the defect is not label-specific |
| AC-3 | `TestRFC8277AddPathRoutesOnOneSessionAreComparable` | Becomes the tagged failing-first test for RFC8277-3.1-1: red before the fix (recorded), green after, with a discrimination record |
| AC-4 | ADD-PATH session: path 7 label 100, path 9 label 200, then path 9 re-advertised with label 300 | Path 7 still reads label 100, path 9 reads 300; withdrawing path 9 leaves path 7's label in place |
| AC-5 | RFC8277-3.1-1 and RFC8277-2.5-2 | Verdicts `enforced`: tagged units in both polarities with discrimination records, re-judged by an agent that did not write them; both removed from the Blocked-by list of `plan/pre-release/spec-rfc-verdict-fix-bgp.md` |
| AC-6 | RFC8277-2.5-3 | The `{gap}` annotation is removed and the row reaches `enforced` the same way as AC-5 |
| AC-7 | Interop | A named scenario (proposed `bgp-addpath-best-path-frr`, or an extension of `bgp-addpath-frr`) where FRR sends two paths of one prefix over one ADD-PATH session and Ze elects and advertises the better one; shown red with the fix reverted |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | [for example "receives SR-Policy UPDATE from peer"] | [wire -> mpnlri -> splitter -> Parse -> RIB] | [test name] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC8277AddPathRoutesOnOneSessionAreComparable` | `internal/component/bgp/plugins/rib/rfc8277_addpath_comparable_red_test.go` | AC-1, RFC8277-3.1-1 | red |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| [field] | [min-max] | [value] | [value or N/A] | [value or N/A] |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-xxx` | `test/.../*.ci` | [what the user expects to happen] | |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-addpath-best-path-frr` | `test/interop/scenarios/` | FRR | two paths of one prefix on one ADD-PATH session are compared, one best per prefix | |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/component/bgp/plugins/rib/rib_commands.go` - candidate gathering per prefix across every path id
- `internal/component/bgp/plugins/rib/rib_bestchange.go` - one best record per prefix
- `internal/component/bgp/plugins/rib/storage/peerrib.go`, `familyrib.go` - label side-data keyed per path

## Files to Create
- `internal/...` - [new feature file]
- `test/.../*.ci` - [functional test for end-user behavior]

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | `internal/component/<name>/yang/` or the owning plugin's `yang/`. Read `ai/rules/config.md` (YANG vs env var) and `ai/rules/config.md` (naming) |
| YANG validation constraints | | Every leaf takes maximum native validation: `range`, `length`, `pattern`, `enumeration`, `type` from `ze-types.yang`. See `ai/patterns/config-option.md` |
| YANG custom validators | | Where native constraints are insufficient: `ze:validate` + `ValidateFn` + `CompleteFn` for completion |
| CLI commands/flags | | `cmd/ze/*/main.go` or subcommand files |
| CLI grammar (keyword before value) | | `ai/rules/cli.md` |
| Editor autocomplete | | Automatic for YANG enum/type leaves. Dynamic values need `CompleteFn` |
| Functional test for new RPC/API | | `test/plugin/*.ci` or `test/decode/*.ci` |
| Pipe completeness | | Route output through `ApplyPipes`/`ProcessPipes` per `ai/rules/cli.md` |
| Env var registration | | YANG leaves under `environment/` need a matching `ze.<name>.<leaf>` via `env.MustRegister()` |
| Doctor check for runtime dependencies | | Any new file path, socket, service, kernel module, listen port, procfs/sysctl, netlink, binary, or certificate: owning-package check + `internal/core/diagnostic/codes.go` + unit and functional test (`ai/rules/repo-maintenance.md`) |
| Prometheus counters/metrics | | Observable state: define, register, and list the metric names and labels here |
| BGP family surface (new SAFI / capability / attribute) | | The 12-section checklist in `ai/patterns/bgp-family.md` -- read it and record the answers there, not inline |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | `docs/features.md` |
| 2 | Config syntax changed? | | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | | `rfc/short/rfcNNNN.md` and the `docs/features/rfc-status.md` row, with source anchors |
| 10 | Test infrastructure changed? | | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | | `docs/comparison.md` |
| 12 | Internal architecture changed? | | `docs/architecture/core-design.md` or subsystem doc |
| 13 | Route metadata keys added/changed? | | `docs/architecture/meta/README.md`, `docs/architecture/meta/<plugin>.md` |
| 14 | Prometheus counters added/changed? | | `docs/plugin-development/metrics.md` or subsystem telemetry doc |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | | DERIVED, do not answer from memory: `./le spec citation anchors spec plan/<this-spec>.md` lists them. A doc DECLARED by a changed file's `// Design:` header BLOCKS until named here; a doc that only `<!-- source: -->` mentions it is advisory. Naming it as unaffected, with the reason, satisfies the check |
| 17 | Existing docs show config/CLI/API examples for this area? | | Verify examples against YANG/parser/handler and update stale syntax |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- register entry points, write failing wiring tests
   - Tests: [wiring test names from the Wiring Test table]
   - Files: [register.go, handler skeleton, route registration]
   - Verify: the entry point exists and is reachable. The wiring test fails because the feature is a stub
2. **Phase: [name]** -- [what to implement]
   - Tests: [test names from the TDD Plan]
   - Files: [files from Files to Modify]
   - Verify: tests fail → implement → tests pass → wiring test progresses

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
| Rule: performance | No new per-UPDATE allocation on the gather (stack-backed path slice) or the winner key (stack buffer) |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| [concrete thing that must exist] | [grep/ls/test command] |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | [what inputs need validation and how] |

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

## Design Insights
<!-- LIVE: write immediately when you learn something. Route each lesson to its
     governing surface under ai/rules/planning.md. A problem-class journal row
     is appropriate only when no surface governs the lesson yet; closure alone
     requires no lesson artifact. -->

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- [What was deliberately not done and why]

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
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
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
