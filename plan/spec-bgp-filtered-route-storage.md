# Spec: BGP filtered-route storage and real routes_filtered

| Field | Value |
|-------|-------|
| Status | blocked |
| Depends | spec-bgp-peer-settings-reload-ignored (AC-3 cannot be built until changed peer settings actually apply on reload) |
| Phase | 1 (Decision) complete; audit complete; implementation NOT started |
| Updated | 2026-07-16 |

## BLOCKER (recorded 2026-07-16, user-approved sequencing)

**Status: audit complete, no feature code written. Blocked on `spec-bgp-peer-settings-reload-ignored`.**

AC-3 ("policy loosened → routes_filtered drops") cannot be implemented because changing a peer's
import policy has **no effect at all** on a running peer today. Verified chain, each link read at
the producer:
1. `filter_ordered.go:138` — `filters := peer.settings.ImportFilters`, read live per message off the peer's settings pointer.
2. `peer.go:318` — `settings` is assigned once in `NewPeer`; no setter and no `p.settings =` write exists anywhere in the reactor (grep-verified empty).
3. `reactor_api.go:780-825` — `peerSettingsEqual` omits `ImportFilters`, so `reconcilePeersJournaled` (`reactor_api.go:477`, `:498-513`) puts a filter-only change in neither `toRemove` nor `toAdd`; the newly parsed settings are discarded.

There is also no re-evaluation hook to build on: no BGP plugin registers `OnConfigApply`; the RIB
plugin registers only startup-scoped `OnConfigure` (`rib.go:600`); RIB event kinds
(`plugins/rib/events/events.go`) carry no policy/config-change kind; and route refresh
(`SoftClearPeer`, `reactor_api.go:236`) is reachable only from the `ze-bgp:peer-clear-soft` RPC.
Structurally, the import chain runs BEFORE storage (`reactor_notify.go:448`), so there is no
pre-policy Adj-RIB-In to replay and soft reconfiguration from stored state is impossible today.

**User decision 2026-07-16:** fix the reload defect first under its own spec, then resume A+B here.
Do NOT implement AC-1/2/4/5 in isolation and do NOT drop AC-3 — the scope reduction was offered and
declined (`ai/rules/no-partial-completion.md`).

**The audit below is complete and verified; resume from it — do not re-derive.** Every prior
`→ Constraint:` annotation was re-based on producers after three were found fabricated.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/project-knowledge.md` - "No filtered/noexport route tracking" entry
4. `internal/component/bgp/reactor/reactor_notify.go`, `internal/component/lg/handler_api.go`

## Task

Make the birdwatcher `routes_filtered` field real. Today it is hardcoded to 0 because
Ze does not retain import-filtered routes at all: the inbound filter reject gate drops
rejected routes before they are cached or dispatched, so there is no current-state count
of "routes this peer sent that we filtered". Two distinct capabilities are in scope, and
the spec MUST decide between them (they have different semantics and cost):

- Option A - filtered-route storage: retain rejected routes in a filtered scope of the
  Adj-RIB-In (equivalent to BIRD `import keep filtered on`) so a true current-state
  `routes_filtered` count exists and drops when policy is loosened. Large feature: new
  storage, memory cost, a config knob, and lifecycle on policy reload.
- Option B - cumulative import-reject counter: increment a per-peer counter at the reject
  gate as a diagnostic. Cheap, but CUMULATIVE (monotonic), not the current-state semantics
  the birdwatcher `routes_filtered` field means, so it is NOT a substitute for Option A.

This is a deferred item from `spec-bgp-summary-route-counts` (Known Limitations). It was
left honest at 0 rather than faked. This spec captures the decision surface so a future
session implements the right thing rather than mapping a cumulative counter onto a
current-state field.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint: annotations. -->
- [ ] `ai/rules/project-knowledge.md` - "No filtered/noexport route tracking"
  → Constraint: rejects are dropped at the gate and never stored; a real count needs new storage, not a read of existing state.
- [ ] ~~`docs/architecture/bgp/rib.md`~~ **DOES NOT EXIST** (nor does `docs/architecture/bgp/`). The skeleton attributed a Constraint to a fabricated path. Real docs: `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/rib-transition.md`, `docs/architecture/route-selection.md`.
- [ ] `docs/architecture/route-selection.md` - **DESIGN DOC, NOT IMPLEMENTED REALITY. Do not build on it.**
  → Constraint: it describes a "unified rejection reason (`uint8`) on every non-best route", but the producer `internal/component/bgp/plugins/rib/storage/routeentry.go:31-54` has NO reason field (only `StaleLevel`, `AttrFingerprint`, `AttrLen`, `Bundle`, `ASPath`). The doc is aspirational; a "store it with reason=import-filtered" design has nothing to hook into. Verified against the producer, not the doc (`ai/rules/no-fabrication.md`).
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go:1039-1056` - `gatherCandidatesLocked`, the ONLY producer of best-path candidates
  → Decision: isolation is achieved STRUCTURALLY, by storing filtered routes outside `r.bgpPeers`. Selection iterates `bgpPeers` alone and does a point `Lookup` per peer; it never iterates `ribInPool`. A filtered map of `*storage.PeerRIB` parallel to `ribInPool` is therefore invisible to selection with NO change to the hot loop (satisfies AC-5 by construction).
  → Constraint: `StaleLevel` is the WRONG model to copy. It does not isolate: it is read into `Candidate.StaleLevel` (`bestpath.go:115`) and only reorders preference in `comparePair` step 0 (`bestpath.go:307-322`). Stale routes still compete.
  → Precedent: `ribInPool` (`rib.go:221-227`) is exactly this pattern; `rib_inject.go:28-30` states its purpose is holding monitored routes "without entering best-path selection", and `rib_inject.go:46-52` actively rejects protocol `"bgp"` injection to keep routes out of a best-path-invisible slot.
- [ ] `docs/architecture/meta/README.md` - **the skeleton mis-described this as "route-count key semantics". It is not.** It documents per-UPDATE *route metadata* (`map[string]any` keys such as `src-role` that ingress filters set and egress filters read). It says nothing about `routes_filtered` or count semantics, so the Constraint the skeleton hung on it was unsourced.
  → Constraint (real, and it applies): IF the design marks a rejected route via the meta map (the gate already builds `ingressMeta`/`routeMeta`, `reactor_notify.go:416`/`:394`), the key MUST be added to the Key Registry (`meta/README.md:10-12`) — "Two plugins using the same key silently overwrite each other" (`:19`) — and readers MUST type-assert and treat wrong-type as absent (`:21`).
  → Constraint: `routes-filtered` is a **summary JSON key, NOT a route metadata key**. Doc-checklist item 13 as written is therefore wrong; it only applies if a genuine meta key is added.

**Provenance warning for future sessions:** three of this skeleton's Required Reading citations
were fabricated — a non-existent `docs/architecture/bgp/rib.md`, an aspirational design in
`route-selection.md` contradicted by `routeentry.go:31-54`, and this mis-described `meta/README.md`
— each carrying an invented "→ Constraint:". Every annotation above has now been re-derived from
producers. Do not trust an un-reverified annotation in this spec's history (`ai/rules/no-fabrication.md`).

### RFC Summaries
- [ ] RFC 4271 Section 3.2 (Adj-RIB-In) - cited in prose only
  → Constraint: filtered routes are conceptually pre-Adj-RIB-In; storing them is an implementation choice, not a protocol requirement.

**Key insights:**
- The birdwatcher semantics come from BIRD: `routes_filtered` is how many currently-held routes were rejected by import policy but retained. Without retention there is no honest non-zero value.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - the import filter reject gate (line 448) returns `false` immediately on `!res.accept`, so a rejected route is neither cached nor dispatched nor counted; nothing about it survives.
- [ ] `internal/component/lg/handler_api.go` - `transformProtocols` (starts line 521) ALREADY maps the field end-to-end: `filtered := getNum(peer, "routes-filtered")` (line 540) feeds both the flat `"routes_filtered"` (line 553) and the nested `"routes"."filtered"` (line 560). The pipe is laid and simply never fed, so it reports 0.
  → Decision: `transformProtocols` needs NO change for AC-1. Feeding `routes-filtered` into the summary row is sufficient to light up the Looking Glass.
- [ ] `internal/component/lg/handler_api.go` - the hardcoded `"routes_filtered": 0` at line 238 is inside `transformBMPProtocols` (starts line 210), NOT `transformProtocols`. It serves BMP-monitored peers, whose routes never pass the reactor import gate.
  → Constraint: BMP-monitored peers are out of scope; their 0 stays honest and hardcoded. An earlier draft of this spec misattributed line 238 to `transformProtocols`.
- [ ] `internal/component/lg/handler_api.go` - `handleAPIRoutesFiltered` (line 254) is a genuine stub returning an empty filtered-routes list; it is the only handler needing new code here.
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - `status()` (lines 707-718) counts only accepted routes held in the Adj-RIB-In via `peerRIB.Len()` / `FamilyLen(scopeFam)`; there is no filtered scope to count.
- [ ] `internal/component/bgp/plugins/cmd/peer/summary.go` - `mergeRibRouteCounts` (line 140) emits only `routes-received`, `routes-accepted`, `routes-sent`. The comment at line 137 states `routes-filtered` is "deliberately never emitted".

**Existing invariant this spec MUST renegotiate (BLOCKING, missed by the skeleton):**
Three artifacts assert `routes-filtered` is NEVER emitted, as a deliberate AC of the prior
`spec-bgp-summary-route-counts`:
| Artifact | Assertion |
|----------|-----------|
| `internal/component/bgp/plugins/cmd/peer/summary.go:137` | comment: "routes-filtered is deliberately never emitted" |
| `internal/component/bgp/plugins/cmd/peer/summary_test.go:605-606` | `assert.False(t, hasFiltered, "routes-filtered is never emitted (AC-4)")` |
| `test/plugin/bgp-summary-route-counts.ci:88-89` | FAILs if `routes-filtered` is present in a peer row |
→ Decision (user-approved): the invariant becomes CONDITIONAL, not deleted. The key stays
absent when retention is off (preserving the prior spec's intent and AC-2 here) and is
emitted only when retention is enabled.
→ **Re-verified 2026-07-16 — the impact is far smaller than first feared: NO existing assertion
needs weakening or deleting.** Both tests exercise the retention-OFF default, where "key absent"
remains the CORRECT expectation under D-5:
| Artifact | Verified state | Action |
|----------|----------------|--------|
| `test/plugin/bgp-summary-route-counts.ci:88-90` | asserts absence under a DEFAULT config (no keep-filtered leaf) → assertion stays TRUE under D-5 | keep the assertion; update only the stale rationale comment at `:87` |
| `summary_test.go:605-606` | asserts absence for a `counts` map carrying no filtered data — the retention-off case | keep as-is; ADD a retention-on case asserting the key IS present |
| `summary.go:137` | comment claiming the key is "deliberately never emitted" | reword to the conditional rule |
→ Constraint: this is a comment/rationale update plus NEW coverage, never a relaxation. If any
edit would make an existing assertion weaker (e.g. "present or absent, both fine"), STOP — that is
the exact shape `ai/rules/no-workarounds-for-missing-behavior.md` forbids, and
`scripts/dev/audit-test-relaxation.py` (run by `/ze-review` step 0) will inspect these edits.

**Behavior to preserve:**
- `routes_filtered` MUST stay 0 (honest) whenever filtered-route retention is disabled; never fabricate a number.
- Best-path selection MUST NOT see filtered routes; any filtered scope is isolated from selection.
- The reject gate's fast drop path stays the default; retention is opt-in so the zero-cost path is unchanged for operators who do not enable it.

**Behavior to change:**
- With retention enabled (Option A), rejected routes are stored in a filtered scope and `routes_filtered` reports the current count.
- Optionally (Option B), a per-peer cumulative reject counter is exposed under a distinct key/metric as a diagnostic.

## Key Design Decisions

| ID | Decision | Rationale | Approved |
|----|----------|-----------|----------|
| D-1 | Implement **both Option A and Option B** | User sign-off 2026-07-16 (Phase 1 decision gate). A delivers the real current-state `routes_filtered`; B adds the cumulative reject diagnostic alongside it. | user |
| D-2 | Option B is exposed under a distinct `*-total` key/metric, NEVER as `routes_filtered` | The birdwatcher field is current-state (BIRD semantics). Mapping a monotonic counter onto it would fabricate a number (R-2, `ai/rules/no-workarounds-for-missing-behavior.md`). | user (implied by D-1 + R-2) |
| D-3 | Retention defaults OFF; the reject-gate fast drop stays the default path | A-3: operators accept memory cost only when they opt in. Preserves the zero-cost path (AC-2). | spec |
| D-4 | `transformProtocols` is NOT modified | It already maps `routes-filtered` -> `routes_filtered` (handler_api.go:540/553/560). Feeding the summary row lights up the LG with no LG change. | verified in code |
| D-5 | The "routes-filtered never emitted" invariant becomes CONDITIONAL, not deleted | Key absent when retention off (prior spec's intent preserved), present when on. Renegotiated with user sign-off, not weakened. | user |
| D-6 | BMP-monitored peers keep their hardcoded 0 (`transformBMPProtocols`) | Their routes never traverse the reactor import gate, so 0 is honest, not a stub. | verified in code |
| D-7 | Option B's counter must be lock-free (atomic), following the existing `peer.IncrUpdatesReceived()` pattern | `reactor_notify.go:239-264` already increments per-peer counters with "lock-free atomics" in this exact function. Option B extends an established pattern rather than inventing one. | verified in code |
| D-8 | The reject gate is **per-UPDATE-message, not per-prefix**; both options MUST count/store NLRI, not messages | `reactor_notify.go:415` filters `payload := wireUpdate.Payload()` (the whole UPDATE) and `:448` accepts/rejects it wholesale. No per-NLRI split happens upstream: `notifyMessageReceiver` is wired straight to `peer.messageCallback` (`reactor_peers.go:121`, `reactor_dynamic.go:81`) and sees the wire UPDATE intact. One rejected UPDATE can carry many prefixes, so counting rejected *messages* would under-report `routes_filtered` and violate AC-1's "M rejected routes". | verified in code |

**Granularity constraint (D-8) — the skeleton missed this and it drives both options:**
`routes_filtered` and the Option B counter must be derived by enumerating the NLRI of a
rejected UPDATE via `wireUpdate.NLRIIterator(addPath)` (`wireu/wire_update.go:223`), plus
`MPReach()` (`:125`) for MP-BGP families, NOT by counting reject events. Withdrawals
(`WithdrawnIterator`, `:238` / `MPUnreach()`, `:149`) must not inflate the count: a rejected
UPDATE carrying withdrawals removes filtered state rather than adding to it.
→ Constraint: the reject-gate comment at `reactor_notify.go:240-241` states "NLRI-level
counters (announce vs withdraw per prefix) belong in the RIB plugin", not the engine. Option B's
counter is NLRI-level, so this comment points it at the RIB plugin. Resolve this placement
question against `ai/rules/plugin-self-containment.md` before writing the counter.

**Ordering note:** Option B ships first (cheap, no storage), then Option A. Per "Future" in
the TDD plan, `routes_filtered` stays honestly 0 until Option A lands. Option A unblocks
`spec-bgp-per-peer-received-counter.md`, which declares `Depends | spec-bgp-filtered-route-storage`
because its received-vs-accepted gap cannot be attributed while filtered routes are untracked.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Peer sends an UPDATE whose route is rejected by inbound import policy at the reactor reject gate (`reactor_notify.go:448`).
- Format at entry: parsed `WireUpdate` plus the filter result (`res.accept == false`).

### Transformation Path
1. Import filter evaluates the route; result is reject (`reactor_notify.go:448`).
2. Option B: increment a per-peer cumulative reject counter (lock-free) and continue dropping.
3. Option A: if keep-filtered is enabled for the peer/family, insert the route into a filtered scope of the Adj-RIB-In instead of dropping it.
4. `rib_commands.status()` counts the filtered scope per peer and emits `route-counts.filtered`.
5. The peer summary handler merges `routes-filtered` into each row; `transformProtocols` maps it to the birdwatcher field.
6. On policy reload, the filtered scope is re-evaluated: newly-accepted routes move to the accepted scope, and the count drops.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reject gate <-> filtered storage | opt-in insert into a separate scope, never into best-path | [ ] |
| RIB plugin <-> peer summary | `route-counts.filtered` key merged into the summary row | [ ] |
| Summary JSON <-> Looking Glass | kebab-case `routes-filtered` read by `transformProtocols` | [ ] |

### Integration Points (verified against producers 2026-07-16)
- `internal/component/bgp/plugins/rib/` - a filtered map of `*storage.PeerRIB` parallel to `ribInPool` (`rib.go:221-227`), keyed by `netip.Addr` like `bgpPeers` (`rib.go:229-232`). Reuses `PeerRIB.Len()` / `FamilyLen(fam)` (`storage/peerrib.go:97`, `:109`) for the count — no new counting code.
- `internal/component/bgp/plugins/cmd/peer/summary.go:140` - `mergeRibRouteCounts` adds `routes-filtered` (same shape as the existing keys), conditional per D-5.
- Config surface - `keep-filtered` leaf. **Owner: the RIB plugin**, via `augment`, following the `rs` plugin precedent (`internal/component/bgp/plugins/rs/yang/ze-rs-conf.yang:39-69`), whose header states the intent: "removing this plugin removes the leaves from the schema" (`ze-rs-conf.yang:9-14`). Satisfies `ai/rules/plugin-self-containment.md`.
  → Constraint: an `augment` cannot target a grouping, so it needs THREE paths to match what core `peer-fields` gets free: `/bgp:bgp/bgp:peer/bgp:<container>`, `/bgp:bgp/bgp:group/bgp:peer/bgp:<container>`, `/bgp:bgp/bgp:group/bgp:<container>` (`ze-rs-conf.yang:63-69`). Miss one and group inheritance breaks.
  → Decision: plugin owns the SCHEMA; the reactor owns the PARSE and the `PeerSettings` field. This split is the established `rs-client` pattern (`ze-rs-conf.yang:36-38`), not a new invention.

### Config plumbing hops (each is hand-written; there is no codegen)
| Hop | Producer |
|-----|----------|
| YANG leaf (plugin-owned augment) | new `plugins/rib/yang/`, modeled on `rs/yang/ze-rs-conf.yang` |
| Group→peer 3-layer merge | `bgp/config/resolve.go:43` `ResolveBGPTree`. **No change needed**: `resolve.go:57-61` states "No field whitelist needed: unknown fields are harmlessly ignored by PeersFromTree downstream" — a new leaf inherits group→peer precedence for free. |
| YANG defaults stamped | `bgp/config/peers.go:399` `applyPeerSchemaDefaults` |
| Map→struct parse | `bgp/reactor/config.go:48` `parsePeerFromTree`, hand-written `mapBool` read (`config.go:879-885`; the resolved tree is stringly-typed, so `mapBool` compares to `"true"`/`"1"`) |
| Struct field | `bgp/reactor/peersettings.go:215` `PeerSettings` |
| Runtime read | `bgp/reactor/peer.go:337` `Peer.Settings()` |

**Reload landmine (BLOCKING, missed by the skeleton) — `peerSettingsEqual`:**
`reconcilePeersJournaled` (`reactor_api.go:477`) bounces a peer only when `peerSettingsEqual`
(`reactor_api.go:780-825`) returns false, and that function compares a **hand-maintained field
list**. `RouteReflectorClient`, `ASOverride`, `RSClient`, `RSFastPath`, `AcceptSRv6PrefixSID`,
`ClusterID`, `NextHopMode`, and `ImportFilters`/`ExportFilters` are all ABSENT from it.
→ Constraint: a new `KeepFiltered` bool on `PeerSettings` will **silently no-op on reload**
unless it is added to `peerSettingsEqual`'s behavior block (`reactor_api.go:803-809`). This is
a fail-open guard of exactly the shape `ai/rules/fail-closed-guards.md` warns about: the
omission looks like working code.
→ Decision: add `KeepFiltered` to `peerSettingsEqual` and cover it with a test that toggles the
leaf and asserts the peer is marked changed. There is no in-place mutation path: a changed peer
is removed and re-added (`reactor_api.go:516-546`, `peer.Stop()` at `:524`), so toggling the knob
bounces the session and the filtered scope rebuilds naturally.

### Architectural Verification
- [ ] No bypassed layers (filtered routes flow through the RIB plugin, not a side channel).
- [ ] No unintended coupling (filtered scope isolated from selection).
- [ ] No duplicated functionality (extends the Adj-RIB-In with a scope, does not fork storage).
- [ ] Zero-copy preserved where applicable (filtered routes stored as refs, matching accepted storage).
- [ ] Registration over hardcoding - the count key and any config leaf register through existing RIB/summary/YANG paths; no per-feature switch/case in a core/shared struct (`ai/rules/plugin-self-containment.md`).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Rejects are dropped and never stored today | `reactor_notify.go:448` returns before caching | The feature is smaller than assumed | re-read producer | **validated** 2026-07-16: producer read; `if !res.accept { return false // Route rejected by filter; don't cache or dispatch. }` returns before cache/dispatch/count |
| A-2 | A filtered scope can be isolated from best-path selection | ~~`docs/architecture/bgp/rib.md`~~ (does not exist); re-based on `rib_commands.go:1039-1056` | Filtered routes could leak into selection | RIB unit test asserting filtered routes are never selected | **confirmed** 2026-07-16: `gatherCandidatesLocked` iterates `r.bgpPeers` ALONE and point-`Lookup`s per peer. Storing filtered routes in a map outside `bgpPeers` is invisible to selection with zero change to the hot loop. Precedent: `ribInPool` (`rib.go:221-227`, purpose stated `rib_inject.go:28-30`). |
| A-3 | Operators accept the memory cost only when they opt in | config-surface convention (default off) | Always-on retention bloats memory on large tables | benchmark memory with retention on vs off | unvalidated |
| A-4 | R-1's "cap the filtered scope" can reuse an existing admission-control mechanism | assumed by R-1's mitigation | The cap must be built from scratch, enlarging the spec | audit the RIB for prefix limits / caps | **broken** 2026-07-16: there is NO admission control anywhere in the Adj-RIB-In. `PeerRIB.Insert`/`InsertEntry` (`storage/peerrib.go:49`, `:61`) return nothing and cannot refuse a route. The only limits found are display truncation (`rib_pipeline.go:706`, `:742`) and `graph.MaxNodes` (`rib_topology.go:60`). R-1's mitigation needs a NEW mechanism: a counting/refusing insert path for the filtered map. |
| A-5 | Toggling a new per-peer leaf takes effect on reload | assumed by AC-3 / the config-surface convention | The knob silently no-ops on reload | read `peerSettingsEqual` | **broken** 2026-07-16: `peerSettingsEqual` (`reactor_api.go:780-825`) compares a hand-maintained field list that omits `RouteReflectorClient`, `RSClient`, `RSFastPath`, `ImportFilters`, `ExportFilters`. A new bool no-ops on reload unless explicitly added there. Existing leaves are ALREADY affected — toggling `route-reflector-client` and reloading is a no-op today. |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Memory blow-up on peers sending many rejected routes | RSS growth in a filtered-heavy scenario | cap the filtered scope size; document the cap; default retention off |
| R-2 | Cumulative counter (Option B) mistaken for current-state | operators see routes_filtered only grow | expose Option B under a distinct `*-total` key, never as `routes_filtered` |
| R-3 | Policy-reload lifecycle leaves stale filtered entries | filtered count does not drop after loosening policy | re-evaluate the filtered scope on reload, covered by a reload `.ci` |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| Peer sends a route rejected by import policy, keep-filtered enabled | -> | filtered scope insert + `route-counts.filtered` | `test/plugin/bgp-filtered-route-storage.ci` |
| Same peer, keep-filtered disabled (default) | -> | route dropped, `routes_filtered` stays 0 | `test/plugin/bgp-filtered-route-default-zero.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | keep-filtered enabled; peer sends N routes, M rejected by import policy | `show bgp summary` reports routes_filtered = M (current-state) |
| AC-2 | keep-filtered disabled (default); peer sends rejected routes | routes_filtered stays 0; fast drop path unchanged |
| AC-3 | Policy loosened so a filtered route is now accepted | routes_filtered drops; the route appears in accepted |
| AC-4 | Option B enabled | a per-peer cumulative reject counter increments and is exposed under a `*-total` key/metric, distinct from routes_filtered |
| AC-5 | Filtered routes present | best-path selection never selects a filtered route |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables keep-filtered and sees how many routes a peer sent that policy rejected | reject gate -> filtered scope -> RIB count -> summary -> LG | `test/plugin/bgp-filtered-route-storage.ci` |
| 2 | Leaves the default and sees an honest 0, no memory cost | reject gate fast drop -> summary routes_filtered=0 | `test/plugin/bgp-filtered-route-default-zero.ci` |
| 3 | Loosens policy and watches routes_filtered drop | reload -> filtered scope re-eval -> count update | `test/reload/bgp-filtered-reeval.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFilteredScopeIsolatedFromSelection` | `internal/component/bgp/plugins/rib/rib_commands_test.go` | filtered routes never selected as best path | |
| `TestRejectCounterCumulative` | `internal/component/bgp/reactor/reactor_notify_test.go` | Option B counter is monotonic per peer | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| routes_filtered | 0..scope cap | scope cap | N/A | rejected beyond cap are dropped, count clamps at cap |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-filtered-route-storage` | `test/plugin/bgp-filtered-route-storage.ci` | keep-filtered on: routes_filtered = M | |
| `bgp-filtered-route-default-zero` | `test/plugin/bgp-filtered-route-default-zero.ci` | default off: routes_filtered = 0, no retention | |
| `bgp-filtered-reeval` | `test/reload/bgp-filtered-reeval.ci` | policy loosened: routes_filtered drops | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-keep-filtered-bird` | `test/interop/scenarios/` | BIRD | routes_filtered matches BIRD's keep-filtered count for the same policy | |

### Future (if deferring any tests)
- Option A and Option B may land in separate commits; if Option B ships first, its `.ci` lands first and routes_filtered stays 0 until Option A.

## Files to Modify
- `internal/component/bgp/reactor/reactor_notify.go` - at the reject gate, increment the cumulative counter (Option B) and/or route to the filtered scope (Option A).
- `internal/component/bgp/plugins/rib/rib_commands.go` - add the filtered scope and count it in `status()`.
- `internal/component/bgp/plugins/cmd/peer/summary.go` - merge `routes-filtered` into the summary row.
- `internal/component/lg/handler_api.go` - implement `handleAPIRoutesFiltered` (line 254 stub) ONLY. Per D-4, `transformProtocols` already maps `routes-filtered` -> `routes_filtered` and must not be touched; per D-6, `transformBMPProtocols`'s hardcoded 0 stays.
- `internal/component/bgp/plugins/cmd/peer/summary_test.go` - update `TestMergeRibRouteCounts` (line 605) to assert the CONDITIONAL invariant per D-5 (absent when retention off, present when on).
- `test/plugin/bgp-summary-route-counts.ci` - update the presence check (lines 88-89) to assert absence only in the retention-off default, per D-5.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (keep-filtered leaf) | [ ] | owning plugin `yang/`; read `ai/rules/config-surface.md` |
| Prometheus counters/metrics | [ ] | Option B cumulative reject counter (distinct `*-total` metric) |
| Functional test for new RPC/API | [x] | `test/plugin/bgp-filtered-route-storage.ci` |
| Config reload lifecycle | [x] | `test/reload/bgp-filtered-reeval.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` (keep-filtered) |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` (BIRD keep-filtered parity) |
| 13 | Route metadata keys added/changed? | [x] | `docs/architecture/meta/README.md` (routes-filtered) |
| 14 | Prometheus counters added/changed? | [x] | `docs/plugin-development/metrics.md` (reject counter) |

## Files to Create
- `test/plugin/bgp-filtered-route-storage.ci` - keep-filtered retention count.
- `test/plugin/bgp-filtered-route-default-zero.ci` - default-off honesty.
- `test/reload/bgp-filtered-reeval.ci` - policy-reload lifecycle.
- `plan/learned/NNN-bgp-filtered-route-storage.md` - learned summary at closure.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 13. /ze-review gate | Review Gate section |

### Implementation Phases
1. **Phase: Decision (MANDATORY FIRST)** - choose Option A, Option B, or both, and the config default. This is a design decision requiring user sign-off before code.
   - Tests: none (decision phase)
   - Files: this spec (record the decision in Key Design Decisions)
   - Verify: user has approved the scope.
2. **Phase: Wiring** - add the config leaf and/or counter, plus a failing `.ci`.
   - Tests: `bgp-filtered-route-default-zero.ci`
   - Files: YANG, `reactor_notify.go`
   - Verify: default path unchanged, `.ci` for retention fails (stub).
3. **Phase: Filtered scope (Option A)** - store rejected routes in an isolated scope, count them.
   - Tests: `bgp-filtered-route-storage.ci`, `TestFilteredScopeIsolatedFromSelection`
   - Files: `rib_commands.go`, `summary.go`, `lg/handler_api.go`
   - Verify: routes_filtered = M, selection unaffected.
4. **Phase: Reload lifecycle** - re-evaluate the filtered scope on policy change.
   - Tests: `bgp-filtered-reeval.ci`
   - Files: RIB plugin reload path
   - Verify: count drops when policy loosens.
5. **Functional + interop tests** -> `.ci` pass; BIRD interop matches.
6. **Full verification** -> `make ze-verify-changed` when other sessions run.
7. **Complete spec** -> audit tables, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has file:line implementation |
| Correctness | routes_filtered is current-state; cumulative counter uses a distinct key |
| Data flow | filtered scope isolated from best-path selection |
| Registration over hardcoding | count key and config leaf register through existing paths, no core switch/case |
| Rule: no-workarounds | routes_filtered stays 0 when retention off; never faked (`ai/rules/no-workarounds-for-missing-behavior.md`) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| filtered scope with count | run `test/plugin/bgp-filtered-route-storage.ci` |
| default-off honesty | run `test/plugin/bgp-filtered-route-default-zero.ci` |
| selection isolation | `go test -run TestFilteredScopeIsolatedFromSelection` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | filtered scope is capped; a peer cannot exhaust memory by sending rejectable routes |
| Input validation | rejected routes are already parsed; storage reuses accepted-path validation |

### Failure Routing
| Failure | Route To |
|---------|----------|
| routes_filtered non-zero with retention off | fast-drop path regressed; restore the default gate |
| filtered route selected as best | scope not isolated; fix RIB scoping |
| count does not drop on reload | reload re-eval missing; add lifecycle hook |
| 3 fix attempts fail | STOP, report, ask user |

## Known Limitations
- Option B (cumulative reject counter) is a diagnostic with different semantics from `routes_filtered`; it never substitutes for Option A and must be surfaced under a distinct `*-total` key.
- Filtered-route retention has a memory cost; it is opt-in and capped. Default remains off, so `routes_filtered` is 0 for operators who do not enable it.
- Until this spec lands, `routes_filtered` is honestly 0 across the CLI and Looking Glass.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete - every row has a concrete test name
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes (or `make ze-verify-changed` when scoped, with rationale)
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### Quality Gates (SHOULD pass)
- [ ] Interop test vs BIRD keep-filtered
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
- [ ] Registration over hardcoding verified

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Minimal coupling
