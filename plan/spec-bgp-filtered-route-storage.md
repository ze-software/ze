# Spec: BGP filtered-route retention and a real routes_filtered

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - (was spec-bgp-peer-settings-reload-ignored for AC-3; that spec closed 2026-08-13 and a policy change now reaches a running peer) |
| Phase | - |
| Updated | 2026-07-16 |

Anchor refresh (2026-07-22 plan review, design unchanged; all citations below
updated in-body to the verified current lines): reject gate
`reactor_notify.go`, `recentUpdates.Add` `:528`, RS fast path `:553`,
`deliverChan` `:613`, ownership doc comment `:221`, per-peer counters `:247`,
rebind block `:471+`, ingress pass `:405-468`. The LG anchors
(`handler_api.go,254,521,540,555`) verified exact. (Re-verified
2026-07-23 after the origin/main fast-forward to 822029463: `deliverChan`
moved `:588` -> `:613`, whole-body `Payload()` take `:415` -> `:432`; the
rest held.)

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/research/bird-bgp-reference.md` lines 1276-1281 (`rte_update` / `REF_FILTERED`) - the design this spec copies
4. `docs/architecture/core-design.md` lines 629-665 ("Ingress Filter Pipeline")
5. `internal/component/bgp/reactor/reactor_notify.go` (the reject gate, line 468), `internal/component/bgp/plugins/rib/rib_structured.go`
6. `spec-bgp-peer-settings-reload-ignored` - the sibling this depended on for AC-3.
   CLOSED 2026-08-13, so the dependency is discharged: a policy edit now reaches
   the running peer through `peerSettingsSwapPlan`
   (`internal/component/bgp/reactor/peer_settings_apply.go`), which is what AC-3
   needed. It built no pre-policy store, so nothing here is superseded.

## Task

Make the birdwatcher `routes_filtered` field real. Today it is always 0 because Ze does not
retain import-filtered routes at all: the inbound filter reject gate drops a rejected route
before it is cached or dispatched, so no current-state count of "routes this peer sent that
our policy rejected" can exist.

**Design: copy BIRD's `import keep filtered`.** When retention is enabled for a peer, a route
rejected by import policy is RETAINED and marked filtered, is visible to `show`/Looking Glass,
and is never eligible for best-path selection. When retention is off (the default), the reject
gate keeps its current zero-cost drop and `routes_filtered` stays honestly 0.

A second, independent capability is also in scope:

**Option B - cumulative import-reject counter.** A per-peer monotonic count of NLRI rejected at
the gate, exposed under a distinct `*-total` key. It is a CUMULATIVE diagnostic, NOT the
current-state semantics `routes_filtered` carries, so it never substitutes for retention. It has
no storage, config, or reload surface and can land independently of everything else here.

### ⚠ Terminology hazard (this spec exists because two sessions fell into it)

"Soft reconfiguration" is overloaded, **including inside this repo**:

| Term | Meaning | Source |
|------|---------|--------|
| BIRD `import keep filtered` | retain only the REJECTED copy, flagged `REF_FILTERED` in the MAIN table, hidden from best-path. **This spec.** | BIRD v2.19.0 `nest/config.Y:318`, `nest/route.h:274/280`, `doc/bird.sgml:1145-1150` |
| BIRD `import table` | a SEPARATE, unrelated knob with its own `->in_table` channel field | BIRD v2.19.0 `nest/config.Y:718-720` |
| BIRD "soft reconfig" (`bgp_reconfigure`) | reconfigure a LIVE session: compare configs, swap filter pointers atomically, session survives. **Not a store at all.** | `docs/research/bird-bgp-reference.md` (BIRD 3.x, second-hand) |
| Cisco "soft-reconfiguration inbound" | STORE ALL received routes pre-policy, re-run policy locally | `rfc/short/rfc2918.md` ("use soft-reconfiguration (store all routes)") |

These are FOUR different mechanisms. An earlier draft of this spec asserted that "BIRD's
`import keep filtered` and its soft reconfiguration are two views of ONE store", concluded this
spec was superseded by a pre-policy store, deleted its design, and carried a user decision on
that basis. **The claim was fabricated** - I wrote it into this spec with no citation, and a
later search then found it *here* and reported it back as repo evidence. BIRD v2.19.0 disproves
it outright: `nest/rt-table.c:1687-1697` keeps only the rejected copy behind a flag.
→ Constraint: this spec needs NO pre-policy store. Do not reintroduce one. If a future session
wants Cisco-style soft reconfiguration, that is a separate spec needing its own justification.
→ Constraint: **never attach a daemon's name to a mechanism without citing that daemon.** The
source is checked out at `~/Code/gitlab.nic.cz/labs/bird` (v2.19.0) and `~/Code/others/bgp/bird`
(v1.3.8). Read it instead of recalling it.

## BIRD ground truth (PRIMARY SOURCE - verified 2026-07-16)

**Read this before the repo's BIRD doc.** Checkout: `~/Code/gitlab.nic.cz/labs/bird`, tag
`v2.19.0-4-g02d082a7` (current stable). Every line below was read at the source. This section
exists because two earlier drafts of this spec asserted BIRD behavior from memory and got it
wrong twice; the design now rests on primary source, not recollection.

| Fact | BIRD v2.19.0 source |
|------|--------------------|
| The knob | `nest/config.Y:318` — `IMPORT KEEP FILTERED bool { this_channel->in_keep_filtered = $4; }` |
| **Default: off** | `doc/bird.sgml:1150` — "Default: off." |
| Semantics | `doc/bird.sgml:1146-1150` — "Usually, if an import filter rejects a route, the route is forgotten. When this option is active, these routes are kept in the routing table, but they are **hidden and not propagated to other protocols**. But it is possible to show them using `show route filtered`." |
| The flag | `nest/route.h:274` — `#define REF_FILTERED 2 /* Route is rejected by import filter */` |
| Isolation from selection | `nest/route.h:280` — `rte_is_valid(rte *r) { return r && !(r->flags & REF_FILTERED); }`; best-path walks `for (rte *r = net->routes; rte_is_valid(r); r = r->next)` (`nest/rt-table.c:959`, `:977`) |
| Drop vs keep | `nest/rt-table.c:1687-1697` — `if (filter == FILTER_REJECT) { stats->imp_updates_filtered++; if (! c->in_keep_filtered) goto drop; new->flags \|= REF_FILTERED; }` |
| **The gauge = `routes_filtered`** | `nest/protocol.h:146` — `u32 filt_routes; /* Number of routes rejected in import filter but kept in the routing table */` |
| Gauge maintenance | `nest/rt-table.c:1447-1449` — `rte_is_filtered(new) ? stats->filt_routes++ : stats->imp_routes++;` and the matching `--` on remove |
| **Conditional display** | `nest/proto.c:2070-2076` — when `in_keep_filtered` is off BIRD prints "%u imported, %u exported, %u preferred"; only when ON does it print "%u imported, **%u filtered**, %u exported, %u preferred". **BIRD omits the field entirely rather than printing 0** |
| Gauge vs cumulative are DIFFERENT | `filt_routes` (gauge, `protocol.h:146`) vs `imp_updates_filtered` (cumulative, `rt-table.c:1689`), shown on separate lines (`proto.c:2077-2080`: "Route change stats: received rejected filtered ignored accepted") |
| `import table` is a SEPARATE feature | `nest/config.Y:718-720` — `IMPORT TABLE channel_arg { if (!$4->in_table) cf_error("No import table in channel %s.%s", ...) }`. A distinct `->in_table` field, unrelated to `in_keep_filtered` |
| Limits ALSO flag filtered | `nest/rt-table.c:1418-1421` — on an import-limit overrun: `if (c->in_keep_filtered) new->flags \|= REF_FILTERED; else { rte_free_quick(new); new = NULL; }` (see A-7) |
| Filtered routes count AGAINST the limit | `nest/rt-table.c:1383` — `u32 all_routes = stats->imp_routes + stats->filt_routes;` |
| BIRD does NOT re-run policy when the knob changes | `nest/proto.c:843` — `/* FIXME: better handle these changes, also handle in_keep_filtered */`. BIRD has an open FIXME here; do not assume BIRD parity gives AC-3 for free |

→ **Decision (D-1 rests on this):** BIRD's model is ONE table, a per-route FLAG on the rejected
copy, excluded from selection by being *invalid*. It is NOT a pre-policy store. It retains only
what the filter rejected.
→ **Constraint:** BIRD runs its import filter INSIDE the table write path (`rte_update`), after
decode, PER PREFIX. Ze runs it in the REACTOR, before dispatch, PER UPDATE MESSAGE. The
observable behavior is copyable; the placement is not. This asymmetry is the whole difficulty.
→ **Constraint:** the repo's `docs/research/bird-bgp-reference.md` describes **BIRD 3.x**, not
the local checkouts: it cites `lib/route.h:270-278` and `nest/rt-table.c:2639`, neither of which
resolves in v2.19.0 (`route.h` is `nest/route.h` there) or in the v1.3.8 checkout at
`~/Code/others/bgp/bird`. Its *descriptions* match v2.19.0's semantics, but **its line numbers
are not verifiable locally.** Cite this section, not that doc.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint: annotations. -->
- [ ] `docs/research/bird-bgp-reference.md` lines 1276-1281 - a SECOND-HAND description of BIRD 3.x's `rte_update`. Consistent with v2.19.0's semantics, but superseded for this spec by the primary-source table above.
  → Constraint: this file also contains a FALSE claim about Ze at `:1614` ("ze's filter reload path already handles the common case without bouncing sessions"). It does not - see `spec-bgp-peer-settings-reload-ignored`. Do not trust this file's Ze claims.
- [ ] `docs/architecture/core-design.md` lines 629-665 - "Ingress Filter Pipeline" (the doc the original skeleton should have cited)
  → Constraint: "inbound filtering runs in the reactor on every received UPDATE, **before** the bytes are cached and **before** the StructuredEvent is dispatched"; "`accept=false` drops the route (no caching, no dispatch)". Retention therefore cannot be added in the RIB plugin alone - the RIB never sees a rejected route.
  → Constraint: "After the pass, the cached `WireUpdate` is the **canonical post-filter representation** that every downstream consumer sees." Stored routes are POST-modification.
  → Constraint: filter order is `Protocol < Policy (in-process) < Annotation/OTC < external per-peer chain` over `r.orderedIngressSteps`. A route may be rejected at ANY step, not only the external policy chain (`reactor_notify.go` is shared by both kinds). "Filtered" must mean "rejected by the ingress pass", and the spec should record WHICH step rejected it if that is cheap.
- [ ] `docs/guide/looking-glass.md` lines 73-78 - the user-facing promise this spec changes
  → Constraint: it currently states `routes_filtered` is always 0 and `/routes/filtered/{name}` returns an empty list, with a source anchor to `summary.go`. Both claims become conditional; the doc MUST be updated (checklist #16).
- [ ] `ai/rules/repo-maintenance.md` line 15 - "No filtered/noexport route tracking"
  → Constraint: "if filtered tracking ever lands, point them at the real store" - pre-authorizes wiring `/routes/filtered/{name}`. The entry must be rewritten when this lands.
  → Constraint: `/routes/noexport/{name}` is EXPORT filtering, a different feature. Out of scope.
- [ ] `ai/rules/plugins.md` - placement of the store and the config leaf
  → Constraint: "a new feature must not require editing a `switch`, `case`, field list, or factory in a core or shared package - it registers and is discovered."

### RFC Summaries
- [ ] `rfc/short/rfc4271.md` Section 3.2 (Adj-RIB-In) - cited in prose only
  → Constraint: filtered routes are conceptually pre-Adj-RIB-In; retaining them is an implementation choice, not a protocol requirement. No wire behavior changes in this spec.
- [ ] `rfc/short/rfc2918.md` line 164 - only if AC-3 is in scope
  → Constraint: route refresh re-advertises the **Adj-RIB-Out** ("outbound"). It cannot re-apply a LOCAL INBOUND policy by itself; the peer must re-send. AC-3 therefore depends on the sibling spec's apply mechanism, not on anything here.

**Key insights:**
- The birdwatcher semantics come from BIRD: `routes_filtered` counts currently-held routes that import policy rejected but which were retained. Without retention there is no honest non-zero value, which is why it is 0 today rather than faked.
- Ze already carries the field end-to-end in the Looking Glass. Nothing is missing there; it is simply never fed.

## Current Behavior (MANDATORY)

**Source files read (each verified at the producer, 2026-07-16):**
<!-- NEVER tick [ ] to [x]. -->
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - the reject gate. At line 468 `if !res.accept { return false // Route rejected by filter; don't cache or dispatch. }`. Returns BEFORE `recentUpdates.Add` (:528), before the RS fast path (:553), and before `peer.deliverChan <-` (:613), so no plugin and no RIB ever sees the route.
  → Constraint: the `bool` returned by `notifyMessageReceiver` is **buffer ownership** (`kept`), NOT accept/reject - see its doc comment at :221 ("Returns true if buf ownership was taken"). `return false` at :468 means "pool buffer not retained"; the route is dropped as a side effect of skipping dispatch. Do not read this return value as a policy verdict.
  → Constraint: the gate is per-UPDATE-MESSAGE. The filter takes `payload := wireUpdate.Payload()` (:432), the whole UPDATE body, and accepts/rejects it wholesale. NLRI splitting happens later, inside the RIB plugin (`rib_structured.go` via `nlrisplit.Split`). One rejected UPDATE can carry many prefixes.
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - modification. At :471-475+ a non-nil `modifiedPayload` rebinds `payload`, `wireUpdate`, `msg.RawBytes`, `msg.WireUpdate`, `msg.AttrsWire`.
  → Constraint: **the ORIGINAL pre-filter payload is overwritten and retained nowhere.** No live reference survives past :455. Modifying filters exist and compose (`bgp-filter-community` `filter_community.go`; `bgp-role`/OTC `otc.go`; the external chain's raw override / text delta `filter_ordered.go`). The egress twin deliberately does the opposite (`filter_ordered.go`: "Unlike ingress, this reads the original payload ... the payload is never rewritten in the egress pass").
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - existing per-peer counters at :247+ ("Increment per-peer counters (lock-free atomics)"): `peer.IncrUpdatesReceived()`, `IncrEORReceived()`, `IncrKeepalivesReceived()`.
  → Decision: Option B extends this established pattern at this exact site rather than inventing one.
  → Constraint: the comment at :240-241 says "NLRI-level counters (announce vs withdraw per prefix) belong in the RIB plugin". Option B's counter IS NLRI-level, so this comment argues against the engine. Resolve placement explicitly (see D-7); do not silently contradict a stated architectural boundary.
- [ ] `internal/component/lg/handler_api.go` - `transformProtocols` (starts :521) ALREADY maps the field end-to-end: `filtered := getNum(peer, "routes-filtered")` (:540) feeds the flat `"routes_filtered"` (:553) and the nested `"routes"."filtered"` (:560).
  → Decision: **no change needed here for AC-1.** Feeding `routes-filtered` into the summary row lights up the Looking Glass. The pipe is laid and simply never fed.
- [ ] `internal/component/lg/handler_api.go` - the hardcoded `"routes_filtered": 0` at :238 is inside `transformBMPProtocols` (starts :210), NOT `transformProtocols`. An earlier draft misattributed it.
  → Constraint: BMP-monitored peers' routes never traverse the reactor import gate, so their 0 is permanently honest. Out of scope (D-6).
- [ ] `internal/component/lg/handler_api.go` - `handleAPIRoutesFiltered` (:254) is a genuine stub returning an empty list. The only handler needing new code here.
- [ ] `internal/component/bgp/plugins/cmd/peer/summary.go` - `mergeRibRouteCounts` (:140) emits `routes-received`, `routes-accepted`, `routes-sent` only. The comment at :137 states `routes-filtered` is "deliberately never emitted".
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - `status()` (:707-718) counts accepted routes via `peerRIB.Len()` / `FamilyLen(scopeFam)`. No filtered scope exists to count.
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - `gatherCandidatesLocked` (:1039-1056), the ONLY producer of best-path candidates.
  → Decision: it iterates `r.bgpPeers` alone and point-`Lookup`s per peer. Two isolation strategies are therefore available, and both are precedented (D-9).
  → Constraint: `StaleLevel` is the WRONG model to copy. It does NOT isolate: it feeds `Candidate.StaleLevel` (`bestpath.go`) and only reorders preference (`comparePair` step 0, `bestpath.go`). Stale routes still compete.
  → Precedent A (separate map): `ribInPool` (`rib.go`) holds whole `*storage.PeerRIB`s that `gatherCandidates` never visits; `rib_inject.go` states its purpose is holding monitored routes "without entering best-path selection".
  → Precedent B (per-route skip): `isSRv6Ineligible` (`rib_bestchange.go`) is a `continue` inside the gather loop. This is the closest analogue to BIRD's `REF_FILTERED`.
- [ ] `internal/component/bgp/plugins/rib/rib_structured.go` - the storage path: `handleReceivedStructured` lazily creates the `PeerRIB` (:138-160), parses attributes once via `storage.ParseRouteEntry` (:171), then splits NLRI and calls `peerRIB.InsertEntry` (:187/:189/:228/:230).
- [ ] `internal/component/bgp/plugins/rib/storage/routeentry.go` - `RouteEntry` (:31-54) holds `StaleLevel uint8`, `AttrFingerprint uint64`, `AttrLen uint32`, `Bundle attrpool.Handle`, `ASPath attrpool.Handle`.
  → Constraint: there is **no rejection-reason field**. `docs/architecture/route-selection.md` describes a "unified rejection reason (uint8) on every non-best route" - that doc is ASPIRATIONAL and contradicted by this producer. Do not build on it.
- [ ] `internal/component/bgp/plugins/rib/rib_inject.go` - `handleInjectWireRoute` (:35-135) inserts into `ribInPool`, but **explicitly rejects protocol `"bgp"`** at :46-52 ("must use the BGP UPDATE path, not protocol injection") and `ribInPool`'s inner map is string-keyed by design (`rib.go`).
  → Constraint: this mechanism cannot be reused as-is for BGP filtered routes.

**Behavior to preserve:**
- `routes_filtered` MUST stay 0 (honest) whenever retention is disabled. Never fabricate a number.
- The reject gate's zero-cost drop stays the DEFAULT path. Retention is opt-in; operators who do not enable it pay nothing (`ai/rules/completion.md`).
- Best-path selection MUST NOT see filtered routes.
- BMP-monitored peers keep their honest hardcoded 0.
- `/routes/noexport/{name}` stays an empty stub (export filtering is out of scope).
- The buffer-pooling contract: nothing may retain a `[]byte` aliasing a pooled read buffer without taking one of the two documented ownership contracts (see R-4).

**Behavior to change:**
- With retention enabled, a route rejected by the ingress pass is retained, marked filtered, counted, and shown; it is never selected as best.
- `routes-filtered` is emitted in the peer summary row when (and only when) retention is enabled.
- `handleAPIRoutesFiltered` returns the real filtered routes instead of an empty list.
- Independently, Option B exposes a per-peer cumulative reject counter under a distinct `*-total` key.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A peer sends an UPDATE whose route the ingress pass rejects (`reactor_notify.go`).
- Format at entry: the UPDATE body (`payload []byte`, aliasing a pooled read buffer) plus the pass's verdict.

### Transformation Path
1. The stage-ordered ingress pass runs over `r.orderedIngressSteps` (`reactor_notify.go`).
2. A step rejects (`res.accept == false`).
3. **Option B**: count the NLRI carried by the rejected UPDATE and add to a per-peer cumulative counter. Always cheap; no retention.
4. **Retention OFF (default)**: `return false` - unchanged fast drop. Buffer returns to the pool via `session_read.go`.
5. **Retention ON**: instead of dropping, the route continues to dispatch MARKED as filtered, so the RIB plugin receives it and stores it in the filtered state.
6. The RIB plugin stores it isolated from selection and counts it per peer/family.
7. `rib_commands.status()` reports the filtered count; the peer summary merges `routes-filtered`; `transformProtocols` (unchanged) maps it to the birdwatcher field.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reject gate <-> RIB plugin | rejected routes must be DISPATCHED (marked) instead of dropped; today they are never dispatched | [x] verified: `reactor_notify.go` precedes `:613` dispatch (re-anchored 2026-07-23) |
| Pooled buffer <-> retained route | retention needs an ownership contract | [x] verified: `WireUpdate.Snapshot()` (`wire_update.go`) or `ribForwardHandle` copy-on-AddRef (`rib/forward_handle.go`) |
| RIB plugin <-> peer summary | `route-counts.filtered` merged into the row | [ ] |
| Summary JSON <-> Looking Glass | kebab-case `routes-filtered` read by `transformProtocols:540` | [x] verified: mapping already exists |

### Integration Points
- `internal/component/bgp/reactor/reactor_notify.go` - the gate: mark-and-dispatch when retention is on; count for Option B.
- `internal/component/bgp/plugins/rib/` - the filtered state and its count (D-9 decides flag vs map).
- `internal/component/bgp/plugins/cmd/peer/summary.go` - conditional `routes-filtered`.
- `internal/component/lg/handler_api.go` - `handleAPIRoutesFiltered`.
- Config: a `keep-filtered` per-peer leaf, **owned by the RIB plugin** via `augment`, following the `rs` plugin precedent (`plugins/rs/yang/ze-rs-conf.yang`), whose header states the intent: "removing this plugin removes the leaves from the schema" (`:9-14`).
  → Constraint: an `augment` cannot target a grouping, so it needs THREE paths to match what core `peer-fields` gets free: `/bgp:bgp/bgp:peer/bgp:<container>`, `/bgp:bgp/bgp:group/bgp:peer/bgp:<container>`, `/bgp:bgp/bgp:group/bgp:<container>` (`ze-rs-conf.yang`). Miss one and group inheritance breaks.
  → Decision: plugin owns the SCHEMA; the reactor owns the PARSE and the `PeerSettings` field. Established `rs-client` split (`ze-rs-conf.yang`).

### Architectural Verification
- [ ] No bypassed layers (filtered routes reach storage through the normal dispatch path, not a side channel).
- [ ] No unintended coupling (filtered routes isolated from selection).
- [ ] No duplicated functionality (extends the existing Adj-RIB-In path; does not fork storage).
- [ ] Zero-copy preserved where applicable (retention takes an existing ownership contract; no new copy semantics invented).
- [ ] Registration over hardcoding - the config leaf registers via the owning plugin's `yang/`; no per-feature switch/case in a core/shared struct.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Rejects are dropped and never stored today | `reactor_notify.go` | The feature is smaller than assumed | re-read the producer | **confirmed** 2026-07-16: returns before cache and dispatch. Corroborated by `core-design.md` ("`accept=false` drops the route (no caching, no dispatch)") |
| A-2 | A filtered route can be isolated from best-path selection | `rib_commands.go` | Filtered routes leak into selection | RIB unit test | **confirmed** 2026-07-16: `gatherCandidatesLocked` iterates `r.bgpPeers` alone; two precedented isolation strategies exist (D-9) |
| A-3 | Operators accept the memory cost only when they opt in | config-surface convention; BIRD makes `import keep filtered` opt-in (UNVERIFIED - see A-6) | Always-on retention bloats memory | benchmark retention on vs off | unvalidated |
| A-4 | R-1's cap can reuse existing admission control | assumed by R-1 | The cap must be built from scratch | audit the RIB for limits | **broken** 2026-07-16: there is NO admission control. `PeerRIB.Insert`/`InsertEntry` (`storage/peerrib.go`, `:61`) return nothing and cannot refuse a route. Only display truncation (`rib_pipeline.go`, `:742`) and `graph.MaxNodes` (`rib_topology.go`) exist |
| A-5 | A new per-peer leaf takes effect on reload | config-surface convention | The knob silently no-ops | read `peerSettingsEqual` | **broken** 2026-07-16: `peerSettingsEqual` (`reactor_api.go`) compared ~15 of ~50 fields. This is the sibling spec's subject; a new `KeepFiltered` field must be covered by whatever guard that spec lands |
| A-6 | BIRD's `import keep filtered` is opt-in and retains ONLY rejected routes | BIRD v2.19.0 source + its own manual | The default-off argument loses its BIRD-parity justification | read BIRD source directly | **CONFIRMED 2026-07-16 against BIRD v2.19.0** (`~/Code/gitlab.nic.cz/labs/bird`, `v2.19.0-4-g02d082a7`). See "BIRD ground truth" below. Retains only the REJECTED copy; **Default: off** (`doc/bird.sgml:1150`); `import table` is a SEPARATE knob (`nest/config.Y:718-720`, distinct `->in_table`) |
| A-7 | Only import-POLICY rejects are counted as filtered | assumed by AC-7 | AC-7 is wrong for BIRD parity | read BIRD source | **BROKEN 2026-07-16.** BIRD ALSO flags routes ignored by an import LIMIT as `REF_FILTERED` when keep-filtered is on (`nest/rt-table.c:1418-1421`), so they land in `filt_routes` too. BIRD separates them only in the CUMULATIVE stats (`imp_updates_filtered` vs `imp_updates_ignored`, `nest/proto.c:2078-2080`), not in the gauge. AC-7 is corrected accordingly |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Memory blow-up from a peer sending many rejectable routes | RSS growth in a filtered-heavy scenario | cap the filtered state; document the cap; default off. NOTE A-4: the cap has no existing mechanism to build on |
| R-2 | Option B's cumulative counter mistaken for current-state | operators see the number only ever grow | expose under a distinct `*-total` key, never as `routes_filtered` (D-2) |
| R-3 | Retention changes the reject fast path for everyone | latency/alloc regression with retention OFF | the gate must branch on the per-peer flag BEFORE doing any work; benchmark the default path unchanged |
| R-4 | Retaining a pooled buffer corrupts memory | garbage/overwritten routes under load; race detector | MUST take an existing ownership contract: `WireUpdate.Snapshot()` (`wire_update.go`, eager copy) or `ribForwardHandle`'s copy-on-AddRef (`forward_handle.go`). The recycle is refcount-driven (`recent_cache.go`, `:526`), so a retained alias is silently reused |
| R-5 | "Filtered" conflated with other drops | operators see loop/RFC7606/prefix-limit drops counted as policy-filtered | the count MUST mean "rejected by the ingress pass". RFC 7606 (`session_read.go`) and prefix limits reject EARLIER and never reach the gate - verify they are excluded |
| R-6 | Storing post-modification bytes misrepresents what the peer sent | `show routes filtered` shows a mutated route | a REJECTED route was never modified by a later step (rejection is terminal), but an EARLIER step may have modified it before a LATER step rejected it (`reactor_notify.go`/`:429` compose). Decide and document which bytes are retained |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| Peer sends a route rejected by import policy, keep-filtered ENABLED | -> | mark-and-dispatch + filtered store + `route-counts.filtered` | `test/plugin/bgp-filtered-route-storage.ci` |
| Same peer, keep-filtered DISABLED (default) | -> | unchanged fast drop; `routes_filtered` stays 0; key absent | `test/plugin/bgp-filtered-route-default-zero.ci` |
| Option B enabled; peer sends rejected routes | -> | cumulative `*-total` counter increments | `test/plugin/bgp-reject-counter-total.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | keep-filtered enabled; peer sends N routes, M rejected by import policy | `show bgp` reports routes_filtered = M (current-state). M counts NLRI, not rejected UPDATE messages (D-8) |
| AC-2 | keep-filtered disabled (default); peer sends rejected routes | routes_filtered stays 0; `routes-filtered` key absent; reject fast path unchanged (no added allocation) |
| AC-3 | Policy loosened so a filtered route is now accepted | routes_filtered drops; the route appears in accepted. **Depends on the sibling spec** - see Known Limitations |
| AC-4 | Option B enabled | a per-peer cumulative reject counter increments and is exposed under a distinct `*-total` key/metric, never as routes_filtered |
| AC-5 | Filtered routes present | best-path selection never selects a filtered route |
| AC-6 | keep-filtered enabled; `/routes/filtered/{name}` queried | returns the real filtered routes, not an empty list |
| AC-7 | A route dropped by RFC 7606 malformed-attribute handling | NOT counted as filtered. It is rejected at `session_read.go`, BEFORE the ingress pass, and never reaches the gate. BIRD agrees: malformed routes count as `imp_updates_invalid` ("rejected"), a different bucket from "filtered" (`nest/proto.c:2078-2080`) |
| AC-7b | A route dropped by a PREFIX LIMIT | **OPEN — decide at the Phase 1 gate (A-7).** BIRD DOES flag limit-ignored routes `REF_FILTERED` and counts them in `filt_routes` when keep-filtered is on (`nest/rt-table.c:1418-1421`), and even counts filtered routes against the limit itself. Strict parity therefore says "count them". Ze's prefix limits reject at `session_read.go`, before the gate, so parity here is a deliberate choice with real work attached, not a freebie. Whichever way it goes, DOCUMENT it — an operator comparing Ze's number to BIRD's will notice |
| AC-8 | Retention enabled under sustained rejected-route load | no buffer-pool corruption; `-race` clean (R-4) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables keep-filtered and sees how many routes a peer sent that policy rejected | gate -> mark+dispatch -> filtered store -> count -> summary -> LG | `test/plugin/bgp-filtered-route-storage.ci` |
| 2 | Leaves the default and sees an honest 0 with no memory cost | gate fast drop -> key absent -> LG 0 | `test/plugin/bgp-filtered-route-default-zero.ci` |
| 3 | Lists the actual filtered routes in the Looking Glass | `/routes/filtered/{name}` -> filtered store | `test/plugin/bgp-filtered-route-storage.ci` (AC-6) |
| 4 | Wants a cheap reject diagnostic without enabling retention | gate -> atomic counter -> `*-total` key | `test/plugin/bgp-reject-counter-total.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFilteredRouteNeverSelectedAsBest` | `internal/component/bgp/plugins/rib/rib_commands_test.go` | AC-5: a filtered route is never a best-path candidate | |
| `TestFilteredCountPerPeerFamily` | `internal/component/bgp/plugins/rib/rib_commands_test.go` | AC-1: count is per peer and family-scoped | |
| `TestRejectCounterCountsNLRINotMessages` | `internal/component/bgp/reactor/reactor_notify_test.go` | D-8: one rejected multi-NLRI UPDATE increments by the NLRI count | |
| `TestRejectCounterMonotonic` | `internal/component/bgp/reactor/reactor_notify_test.go` | AC-4: cumulative, never decrements | |
| `TestRetentionOffDoesNotAllocate` | `internal/component/bgp/reactor/reactor_notify_test.go` | AC-2/R-3: default path unchanged (`testing.AllocsPerRun`) | |
| `TestMergeRibRouteCountsFilteredConditional` | `internal/component/bgp/plugins/cmd/peer/summary_test.go` | D-5: key absent when off, present when on | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| routes_filtered | 0..cap | cap | N/A (never negative) | rejects beyond the cap are dropped; the count clamps at cap and the drop is logged (never silently truncated) |
| filtered-cap (if a YANG leaf) | TBD at design | TBD | 0 = disabled? decide | TBD; needs a YANG `range` (`ai/patterns/config-option.md`) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-filtered-route-storage` | `test/plugin/bgp-filtered-route-storage.ci` | keep-filtered on: routes_filtered = M; `/routes/filtered/` lists them | |
| `bgp-filtered-route-default-zero` | `test/plugin/bgp-filtered-route-default-zero.ci` | default off: 0, key absent, no retention | |
| `bgp-reject-counter-total` | `test/plugin/bgp-reject-counter-total.ci` | Option B `*-total` increments, distinct from routes_filtered | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-keep-filtered-bird` | `test/interop/scenarios/` | BIRD | Ze's routes_filtered matches BIRD's `import keep filtered` count for the same policy and route set | |
<!-- This spec changes NO wire behavior, so interop is not strictly mandatory. It is included
     because the whole point is BIRD parity, and A-6 records that the repo cannot settle BIRD's
     exact semantics. An interop test against a real BIRD is the ONLY way to validate A-6. -->

### Future (if deferring any tests)
- Option B may land before retention; its `.ci` lands with it and `routes_filtered` stays 0 until retention lands. This is a real ordering, not a deferral of scope.

## Files to Modify
- `internal/component/bgp/reactor/reactor_notify.go` - the gate: Option B counter; mark-and-dispatch when retention is on.
- `internal/component/bgp/reactor/peer_settings.go` - a `KeepFiltered` field (+ the sibling spec's reload guard must cover it, A-5).
- `internal/component/bgp/reactor/config.go` - parse the leaf (hand-written `mapBool`, `:879-885`; the resolved tree is stringly-typed).
- `internal/component/bgp/plugins/rib/rib_structured.go` - store the marked route in the filtered state.
- `internal/component/bgp/plugins/rib/rib_commands.go` - count the filtered state in `status()`; isolate from `gatherCandidatesLocked` if D-9 picks the flag model.
- `internal/component/bgp/plugins/cmd/peer/summary.go` - conditional `routes-filtered` (:140); rewrite the stale comment at :137.
- `internal/component/lg/handler_api.go` - `handleAPIRoutesFiltered` (:254) ONLY. `transformProtocols` must NOT change (D-4).
- `internal/component/bgp/plugins/cmd/peer/summary_test.go` - add the retention-ON case (see D-5).
- `test/plugin/bgp-summary-route-counts.ci` - update the stale rationale comment at :87 only; the assertion at :88-90 stays valid (see D-5).

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (keep-filtered leaf, + cap) | [x] | NEW `internal/component/bgp/plugins/rib/yang/`, modeled on `plugins/rs/yang/ze-rs-conf.yang`. Read `ai/rules/config.md` |
| YANG validation constraints | [x] | `type boolean; default false;` for the knob; a `range` for any cap leaf (`ai/patterns/config-option.md`) |
| Prometheus counters/metrics | [x] | Option B's cumulative reject counter as a distinct `*_total` metric; `docs/plugin-development/metrics.md` |
| Functional test for new RPC/API | [x] | `test/plugin/*.ci` above |
| Doctor check | [ ] No - no new runtime dependency (no file, socket, port, or external binary) |
| CLI grammar | [ ] No new CLI verb; `show bgp rib ... filtered` scope wording is a follow-up if wanted |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` (currently zero hits for filtered routes) |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` (keep-filtered + cap) |
| 6 | Has a user guide page? | [x] | **`docs/guide/looking-glass.md`** - it currently promises `routes_filtered` is always 0 and `/routes/filtered/{name}` is empty. MUST become conditional |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` - verified 2026-07-16 to have NO row for filtered retention / keep-filtered (only RFC 2918/7313 capability rows at `:57-58`). Add one |
| 13 | Route metadata keys added/changed? | [x] IF the mark uses the meta map | `docs/architecture/meta/README.md` - the Key Registry; "Two plugins using the same key silently overwrite each other" (`:19`). NOTE: `routes-filtered` is a SUMMARY key, not a meta key - an earlier draft conflated these |
| 14 | Prometheus counters added/changed? | [x] | `docs/plugin-development/metrics.md` (Option B) |
| 16 | Changed files referenced by doc source anchors? | [x] | `docs/guide/looking-glass.md` anchors `summary.go -- fetchRibRouteCounts, mergeRibRouteCounts`; `docs/architecture/core-design.md` anchors `reactor_notify.go`. Both change |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` "Ingress Filter Pipeline" - "`accept=false` drops the route (no caching, no dispatch)" becomes conditional |
| - | Project knowledge stale | [x] | `ai/rules/repo-maintenance.md` "No filtered/noexport route tracking" - rewrite the filtered half, keep the noexport half |

## Files to Create
- `internal/component/bgp/plugins/rib/yang/` (module + embed + register) - the `keep-filtered` leaf.
- `test/plugin/bgp-filtered-route-storage.ci`
- `test/plugin/bgp-filtered-route-default-zero.ci`
- `test/plugin/bgp-reject-counter-total.ci`
- `test/interop/scenarios/NN-keep-filtered-bird/`
- `plan/learned/NNN-bgp-filtered-route-storage.md` - learned summary at closure.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Current Behavior (verified at producers - do not re-derive) |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 13. /ze-review gate | Review Gate section |

### Implementation Phases

1. **Phase: Design gate (BLOCKING - present to user before coding)** - settle D-9 (flag vs separate map) and R-6 (which bytes are retained), and confirm the cap shape (A-4 says it must be built from scratch).
   - Tests: none (decision phase)
   - Files: this spec (record in Key Design Decisions)
   - Verify: user has approved the storage model.
2. **Phase: Option B (independent, may ship first)** - count NLRI at the gate into a per-peer cumulative counter; expose under a distinct `*-total` key + Prometheus metric.
   - Tests: `TestRejectCounterCountsNLRINotMessages`, `TestRejectCounterMonotonic`, `bgp-reject-counter-total.ci`
   - Files: `reactor_notify.go` (or the RIB plugin - resolve D-7 first)
   - Verify: AC-4, AC-7. `routes_filtered` still 0.
3. **Phase: Wiring** - the `keep-filtered` YANG leaf + `PeerSettings` field + parse, plus a failing `.ci`.
   - Tests: `bgp-filtered-route-default-zero.ci` (passes), `bgp-filtered-route-storage.ci` (fails - stub)
   - Files: plugin `yang/`, `peer_settings.go`, `config.go`
   - Verify: AC-2 holds; default path unchanged; the retention `.ci` fails for the right reason.
4. **Phase: Retain + isolate** - mark-and-dispatch at the gate; store in the filtered state; isolate from selection; count.
   - Tests: `TestFilteredRouteNeverSelectedAsBest`, `TestFilteredCountPerPeerFamily`, `TestRetentionOffDoesNotAllocate`, `bgp-filtered-route-storage.ci`
   - Files: `reactor_notify.go`, `rib_structured.go`, `rib_commands.go`, `summary.go`
   - Verify: AC-1, AC-5, AC-8. Take a buffer-ownership contract (R-4).
5. **Phase: Looking Glass** - implement `handleAPIRoutesFiltered`.
   - Tests: `bgp-filtered-route-storage.ci` (AC-6)
   - Files: `lg/handler_api.go`
   - Verify: AC-6. `transformProtocols` untouched (D-4).
6. **Phase: Interop** - `NN-keep-filtered-bird`; the only way to validate A-6.
7. **Full verification** -> `make ze-precommit-verify-changed` when other sessions hold uncommitted work.
8. **Complete spec** -> audit tables, docs, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has file:line implementation |
| Correctness | routes_filtered is current-state; the cumulative counter uses a distinct key; M counts NLRI not messages (D-8) |
| Default path | retention OFF adds no allocation and no branch cost beyond one flag check (AC-2, R-3) |
| Data flow | filtered routes isolated from best-path (AC-5) |
| Memory safety | no retained alias of a pooled buffer; an ownership contract is taken (R-4) |
| Semantics | only ingress-pass rejects are "filtered"; RFC 7606 / prefix-limit / loop drops are not (R-5) |
| Registration over hardcoding | the leaf registers via the owning plugin's `yang/`; no core switch/case |
| Rule: no-workarounds | routes_filtered stays 0 when retention off; never faked |
| Doc honesty | `looking-glass.md` and `repo-maintenance.md` no longer claim a capability that changed |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| filtered retention + count | run `test/plugin/bgp-filtered-route-storage.ci` |
| default-off honesty | run `test/plugin/bgp-filtered-route-default-zero.ci` |
| selection isolation | `go test -run TestFilteredRouteNeverSelectedAsBest` |
| no pooled-buffer corruption | `go test -race` over the retention path |
| Option B distinct key | run `test/plugin/bgp-reject-counter-total.ci` |
| LG endpoint real | `curl /routes/filtered/<peer>` in the `.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | a peer must not exhaust memory by sending rejectable routes. The filtered state MUST be capped, and A-4 proves no admission control exists to reuse - the cap is new code. This is the primary security concern of this spec |
| Memory safety | R-4: a retained pooled-buffer alias is silently reused by a later read (`recent_cache.go`, `:526`) |
| Input validation | rejected routes are already parsed/validated upstream (RFC 7606 at `session_read.go` runs BEFORE the gate); retention must not bypass that ordering |
| Information leakage | `/routes/filtered/` exposes routes policy rejected - same trust boundary as `/routes/`, no new exposure, but confirm the LG auth path is identical |

### Failure Routing
| Failure | Route To |
|---------|----------|
| routes_filtered non-zero with retention off | fast-drop path regressed; restore the default gate |
| a filtered route selected as best | isolation broken; fix per D-9's model |
| count drifts under churn | implicit-withdraw handling; a re-announced prefix must replace, not add |
| garbage routes / race under load | R-4: a pooled buffer was retained without an ownership contract |
| count does not drop on reload | expected until the sibling spec lands; see Known Limitations, not a bug here |
| 3 fix attempts fail | STOP, report, ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| "BIRD's `import keep filtered` and its soft reconfiguration are two views of ONE store" | **Disproven at the primary source.** They are three different mechanisms: `import keep filtered` is a `REF_FILTERED` flag on the rejected copy in the MAIN table (`nest/route.h:274`, `nest/rt-table.c:1687-1697`); `import table` is a SEPARATE knob with its own `->in_table` field (`nest/config.Y:718-720`); BIRD's "soft reconfig" (`bgp_reconfigure`) is an atomic filter-pointer swap on a live session. **None is a pre-policy store.** | First by reading `docs/research/bird-bgp-reference.md` (which no spec had cited), then confirmed against BIRD v2.19.0 source once the user pointed out it is checked out locally | This spec was wrongly marked superseded and its design deleted; a large pre-policy-store design was proposed and a user decision taken on it. **The claim was invented — I wrote it in this spec with no citation, and a later agent then found it and reported it back as if it were repo evidence.** A fabrication laundered into a citation. Cost: one full redesign cycle and two reversed user decisions |
| "Soft reconfiguration (BIRD parity)" describes retaining a pre-policy Adj-RIB-In | BIRD's `bgp_reconfigure` swaps filters atomically on a live session; "store all routes" is CISCO's `soft-reconfiguration inbound` (`rfc/short/rfc2918.md`). The user was offered an option under the wrong daemon's name | Reading `bird-bgp-reference.md` after the decision was already taken | A user decision was made on a mislabelled option and had to be revisited. **Lesson: never attach a daemon's name to a mechanism without a citation to that daemon.** |
| "Soft reconfiguration" names one thing | The repo itself uses it in two senses: `rfc2918.md` = Cisco "store all routes"; `bird-bgp-reference.md` = BIRD live-session reconfigure | Same read | An option was presented to the user under the wrong label, and the decision made on it had to be revisited |
| `transformProtocols` hardcodes `routes_filtered: 0` (skeleton, line 61) | Line 238 is in `transformBMPProtocols`. `transformProtocols` already maps the field at `:540`/`:553`/`:560` | Reading the producer | The LG needs no change; the skeleton would have sent a session editing working code |
| `docs/architecture/bgp/rib.md` documents the storage model | The file does not exist; nor does `docs/architecture/bgp/` | `ls` | A `→ Constraint:` was invented and attributed to a non-existent doc |
| `route-selection.md`'s "unified rejection reason" is implemented | `routeentry.go` has no reason field. The doc is aspirational | Reading the producer after an agent reported the fields | Nearly designed "store with reason=import-filtered" on a field that does not exist |
| `docs/architecture/meta/README.md` documents route-count key semantics | It documents per-UPDATE route METADATA (`src-role` etc.). `routes-filtered` is a summary key, not a meta key | Reading the doc | Doc-checklist item 13 was wrong |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Pre-policy Adj-RIB-In (retain everything, re-run policy locally) | Not BIRD's design for either feature; far larger; doubles Adj-RIB-In memory; the original payload is not even retained today (`reactor_notify.go`) so it needs new copy semantics | BIRD's keep-filtered: retain only the REJECTED copy, marked, excluded from selection |
| Per-route "rejection reason" on `RouteEntry` | No such field exists; the doc describing it is aspirational | D-9: a dedicated flag or a separate map, both precedented |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Specs citing docs that do not exist, or asserting daemon behavior with no citation | 5+ instances across this spec and 2 siblings | `ai/rules/evidence.md` already covers it; the gap is that NOTHING CHECKS spec citations. A `validate-spec.sh` check that every `` `path` `` in Required Reading resolves on disk would have caught 2 of them mechanically | Propose at closure (`ai/rules/repo-maintenance.md`: adding an invariant means adding its gate) |
| Docs asserting a capability the code lacks | 1 (`bird-bgp-reference.md` claims "ze's filter reload path already handles the common case without bouncing sessions" - false) | source anchors exist but nothing verifies the CLAIM, only the path | Note in the sibling spec; that doc line is its subject |

## Design Insights
<!-- LIVE — write IMMEDIATELY when you learn something -->
- **The Looking Glass is already finished.** `transformProtocols` maps `routes-filtered` -> `routes_filtered` (flat and nested) at `handler_api.go/553/560`. Every previous framing treated the LG as work; it is not. The whole feature is upstream of it.
- **BIRD and Ze filter at different layers, and that is the entire difficulty.** BIRD filters inside the table write path (`rte_update`), so "keep the rejected copy" is a local decision at the point of insert. Ze filters in the reactor, before dispatch, so a rejected route must be deliberately CARRIED ACROSS a process/plugin boundary it currently never crosses. The semantics are copyable; the placement is not.
- **The reject gate is per-message; storage is per-prefix.** The filter takes the whole UPDATE body (`reactor_notify.go`) and NLRI splitting happens later inside the RIB plugin (`rib_structured.go`). Every count in this spec must therefore enumerate NLRI. Counting reject EVENTS would silently under-report by the average NLRI-per-UPDATE factor.
- **Ingress and egress have opposite payload discipline.** Ingress overwrites the payload on modify and keeps no original (`reactor_notify.go`); egress deliberately preserves it (`filter_ordered.go`). Any design that wants pre-modification bytes on ingress is fighting the existing architecture, which is a strong argument for keep-filtered (the rejected copy needs no un-modification) over a pre-policy store.

## Core Insight

`routes_filtered` was never a counting problem. The count is trivial once the route exists in a
store; the LG mapping is already written. The problem is that Ze's reject gate is a *boundary*,
not a filter: it decides whether a route is ever seen by anything downstream. Making
`routes_filtered` real means teaching that boundary to carry rejected routes across it, marked -
which is why every cheap-looking approach (a counter, a diff of two numbers, a query over an
existing store) fails to produce current-state semantics, and why BIRD, which filters *inside*
the table, gets this feature almost for free.

## Key Design Decisions

| ID | Decision | Alternatives considered | Rationale |
|----|----------|------------------------|-----------|
| D-1 | **Copy BIRD's `import keep filtered`**: retain only the REJECTED copy, marked, excluded from best-path | Pre-policy store (retain everything, re-run policy) | User sign-off 2026-07-16 (third gate, after two corrections). `bird-bgp-reference.md` is the model. A pre-policy store is Cisco's mechanism, is far larger, and is not needed for any AC here |
| D-2 | Option B is exposed under a distinct `*-total` key/metric, NEVER as `routes_filtered` | map it onto routes_filtered | The birdwatcher field is current-state. A monotonic counter mapped onto it fabricates a number (R-2). **BIRD parity confirmed:** BIRD keeps exactly this split — the gauge `filt_routes` (`nest/protocol.h:146`) and the cumulative `imp_updates_filtered` (`nest/rt-table.c:1689`) are different fields on different output lines (`nest/proto.c:2070-2080`). Option B IS Ze's `imp_updates_filtered` |
| D-3 | Retention defaults OFF; the reject fast drop stays the default | always-on | **BIRD parity confirmed from primary source:** `doc/bird.sgml:1150` "Default: off.", and `nest/rt-table.c:1692-1693` `if (! c->in_keep_filtered) goto drop;` is the default fast path. Independently justified by memory cost (R-1) with no admission control (A-4). A-6's "unverified" caveat is now RESOLVED |
| D-4 | `transformProtocols` is NOT modified | rewrite the LG mapping | It already maps the field (`handler_api.go/553/560`). Verified in code |
| D-5 | The "routes-filtered never emitted" invariant becomes CONDITIONAL, not deleted: key ABSENT when retention off, PRESENT when on | emit 0 when off; delete the invariant | **BIRD parity confirmed from primary source** — this is exactly what BIRD does: `nest/proto.c:2070-2076` prints "%u imported, %u exported, %u preferred" when the knob is off and "%u imported, **%u filtered**, %u exported, %u preferred" only when on. **BIRD omits the field rather than printing 0.** So "absent when off" is parity, not a Ze invention, and emitting 0 would be the deviation. **No existing assertion weakens**: `summary_test.go` and `bgp-summary-route-counts.ci` both exercise the retention-OFF default, where "absent" stays correct. Only the stale rationale comments (`summary.go`, `.ci`) change, plus a NEW retention-ON case. Expect `scripts/dev/audit-test-relaxation.py` to inspect these edits |
| D-6 | BMP-monitored peers keep their hardcoded 0 (`transformBMPProtocols`) | make it real too | Their routes never traverse the reactor import gate, so 0 is permanently honest, not a stub |
| D-7 | **OPEN**: Option B's counter placement - engine vs RIB plugin | - | `reactor_notify.go+` already does lock-free per-peer atomics HERE, but its comment at `:248-249` says "NLRI-level counters ... belong in the RIB plugin", and this counter IS NLRI-level. Rejected routes never reach the RIB plugin today, so the RIB placement would force mark-and-dispatch even when retention is off - defeating AC-2. Resolve at the Phase 1 design gate; if the engine wins, UPDATE the comment rather than silently contradict it |
| D-8 | Both counts MUST enumerate NLRI, not reject events | count reject events | The gate is per-UPDATE-message (`reactor_notify.go`, `:468`); no split happens upstream (`notifyMessageReceiver` is wired straight to `peer.messageCallback`, `reactor_peers.go`, `reactor_dynamic.go`). Use `wireUpdate.NLRIIterator(addPath)` (`wireu/wire_update.go`) + `MPReach()`. Withdrawals (`WithdrawnIterator` `:238`, `MPUnreach()` `:149`) must NOT inflate the count |
| D-9 | **OPEN**: isolation model - per-route flag vs separate map | - | Flag (closest to BIRD's `REF_FILTERED`; precedent `isSRv6Ineligible`, `rib_bestchange.go`, the only per-route skip in the gather loop) costs a check per candidate on the hot path. Separate map (precedent `ribInPool`, `rib.go`, purpose stated `rib_inject.go`) gives structural isolation at ZERO hot-path cost but duplicates per-peer/per-family bookkeeping. Decide at the Phase 1 gate |
| D-10 | Retention MUST take an existing buffer-ownership contract | invent new copy semantics | `WireUpdate.Snapshot()` (`wire_update.go`, eager copy) or `ribForwardHandle` copy-on-AddRef (`forward_handle.go`). The pool recycle is refcount-driven (`recent_cache.go`, `:526`); a retained alias is silently overwritten (R-4) |
| D-11 | The config leaf is owned by the RIB plugin via `augment` | put it in core `ze-bgp-conf.yang` | `ai/rules/plugins.md`; precedent `plugins/rs/yang/ze-rs-conf.yang` ("removing this plugin removes the leaves from the schema"). Plugin owns the schema; the reactor owns the parse and the `PeerSettings` field (`ze-rs-conf.yang`) |

## Known Limitations
- **AC-3 depends on `spec-bgp-peer-settings-reload-ignored`.** A policy change does not reach a running peer at all today: `peerSettingsEqual` (`reactor_api.go`) ignored `ImportFilters`, and `peer.settings` is assigned once in `NewPeer` (`peer.go`) with no setter, so `runIngressPolicyChain` (`filter_ordered.go`) reads a stale chain forever. Until that spec lands, "loosen policy" does nothing, so routes_filtered cannot drop. AC-1/2/4/5/6/7/8 do NOT depend on it and can land first.
- **Route refresh cannot substitute.** RFC 2918 re-advertises the **Adj-RIB-Out** (`rfc/short/rfc2918.md`); it cannot re-apply a local inbound policy without the peer re-sending.
- Option B is a CUMULATIVE diagnostic with different semantics from `routes_filtered`; it never substitutes for retention and must stay under a distinct `*-total` key.
- Retention has a real memory cost and MUST be capped; A-4 proves the cap is new code, since the Adj-RIB-In has no admission control at all.
- `/routes/noexport/{name}` (`handler_api.go`) stays an empty stub. Export-filtered tracking is a different feature.
- BMP-monitored peers keep an honest hardcoded 0 permanently (D-6).
- **AC-3 is not free even at full BIRD parity.** BIRD itself does not re-run policy when the knob or the filters change: `nest/proto.c:843` carries an open `/* FIXME: better handle these changes, also handle in_keep_filtered */`. So "BIRD parity" delivers AC-1/2/4/5/6 but NOT AC-3; AC-3 needs Ze's sibling spec (apply the changed policy) plus a re-import. Do not expect parity to supply it.
- A-6 is **CONFIRMED** against BIRD v2.19.0 source (see "BIRD ground truth"). The interop scenario now validates the IMPLEMENTATION against a running BIRD rather than validating the assumption itself.
- AC-7b (do prefix-limit drops count as filtered?) is deliberately left OPEN for the design gate. BIRD says yes (`nest/rt-table.c:1418-1421`); Ze rejects limits earlier (`session_read.go`), so parity costs real work.
- Line numbers in `docs/research/bird-bgp-reference.md` refer to BIRD 3.x and do not resolve against the local v2.19.0 or v1.3.8 checkouts. If BIRD 3 changed this mechanism (it moved to `ea_stored`/`EALS_FILTERED` per that doc's `:1059`), re-verify before claiming parity with BIRD 3.
- Until retention lands, `routes_filtered` stays honestly 0 across the CLI and Looking Glass.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated (AC-3 only after the sibling spec lands)
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete - every row has a concrete test name
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-standard-test` passes (or `make ze-precommit-verify-changed` when scoped, with rationale)
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (A-6 needs the interop test)

### Quality Gates (SHOULD pass)
- [ ] Interop test vs BIRD keep-filtered (the only validation of A-6)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed (the spec-citation gate proposal)

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
