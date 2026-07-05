# Spec: as112-bgp-redistribute

| Field | Value |
|-------|-------|
| Status | done |
| Depends | `spec-redistribute-late-join-replay` **LANDED** (`ba0085324`/`7eabc2b08`) — R-6 resolved via the `ReplayID uint64` opaque-token mechanism (producers subscribe to `redistevents.ReplayRequestEvent` and echo the token into `RouteChangeBatch.ReplayID`; there is no `Replay:true` bool — AC-14 / Phase 4 language predates the landed API and is reconciled to `ReplayID`) |
| Phase | 8/8 |
| Updated | 2026-07-05 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/learned/634-bgp-redistribute.md`, `plan/learned/641-l2tp-7c-redistribute.md` - redistribute producer pattern
4. `plan/learned/1034-as112-3-bgp-integration.md`, `plan/learned/1035-as112-0-umbrella.md` - AS112 layering rule + covering prefixes
5. Source: `internal/core/redistevents/events.go`, `internal/component/bgp/redistribute/consumer.go`, `internal/component/config/redistribute/registry.go`, `internal/plugins/as112/{config,health}.go`, `internal/component/bgp/plugins/cmd/update/update_text.go`

## Task

Let the `as112` plugin originate its four covering prefixes into BGP as a **virtual
router with its own ASN**, redistributed like `static`/`connected`: the routes enter
the main BGP RIB only when the operator writes `redistribute { destination bgp { import
as112 } }`. The originated routes carry an operator-chosen origin ASN (default 112) and
optional well-known community, both configured in the `service { as112 { ... } }` block.
Announcement is health-gated by a `watchdog` toggle that defaults to true in code (only
announce while the DNS node is serving). This replaces the hand-authored `bgp { update
{ nlri } watchdog }` composition as the *easy path*; the hand-authored composition
remains documented as the *full-control path* for per-peer community / origin-AS /
dedicated-peer-group policy.

**Layering invariant (preserved from the original AS112 design):** `as112` MUST NOT read
`bgp {}` config, and BGP MUST NOT hardcode AS112 knowledge. as112 emits generic
route-change events and registers a generic named redistribute source; BGP imports
"as112" the same way it imports "static". The origin-ASN + community capability added to
the redistribute path is generic (any source may set it), not AS112-specific.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->

- [ ] `plan/learned/634-bgp-redistribute.md` - the redistribute egress producer pattern this feature joins
  → Constraint: a producer joins with two init-time calls -- `events.Register[*RouteChangeBatch](<name>, redistevents.EventType)` for a local typed handle, and `redistevents.RegisterProducer(redistevents.RegisterProtocol(<name>))` -- plus `redistribute.RegisterSource`. No bgp-redistribute changes needed to add a source.
  → Constraint: payload is VALUE TYPES ONLY on the hot path; strings rejected (per-event alloc), pointers across plugin boundaries rejected. AFI/SAFI stored as raw uint16/uint8 to keep `redistevents` a zero-coupling leaf.
  → Decision: canonical inject command is `update text origin incomplete nhop <self|addr> nlri <fam> add <prefix>`; withdraw is `update text nlri <fam> del <prefix>`. `nhop self` triggers per-peer LocalAddress substitution.
- [ ] `plan/learned/641-l2tp-7c-redistribute.md` - closest precedent: a plugin that became a redistribute producer
  → Constraint: typed event handle lives in a `<plugin>/events/` subpackage (l2tp/events, sysrib/events precedent), importable by producer and consumer without pulling the full subsystem.
  → Constraint: per-family emission (one batch per family), not one mixed batch; on teardown only families that were up get a remove batch (prevents spurious withdrawals).
  → Constraint: `ReleaseBatch` zeroes the batch after Emit; subscribers must not retain the payload past dispatch. A test fake that captures the pointer must deep-copy in Emit.
  → Decision: ship a `fake<plugin>` test producer registered in production `all.go` so `.ci` tests drive the real pipeline without the real subsystem; zero runtime cost until invoked.
- [ ] `plan/learned/1034-as112-3-bgp-integration.md` - the current hand-authored BGP integration + layering rule
  → Constraint: neither as112 reads BGP config nor BGP hardcodes AS112; the doctor coordination checks live in the neutral `internal/component/doctor` for exactly this reason.
  → Constraint: `parseCommunityText` now delegates to `attribute.ParseCommunity` (full well-known-name table incl. nopeer/no-export); the runtime `update text` grammar accepts well-known community names, not just 3 hardcoded ones.
- [ ] `plan/learned/1035-as112-0-umbrella.md` - covering prefixes vs host addresses; origin-AS foot-gun
  → Constraint: BGP announces the four /24//48 COVERING prefixes (192.175.48.0/24, 192.31.196.0/24, 2620:4f:8000::/48, 2001:4:112::/48), NOT the /32//128 host addresses bound on lo. Conflating them is the documented easy mistake.
  → Decision: the old design defaulted origin to the operator's own ASN (local-use mirror), 112 as per-group opt-in. THIS SPEC CHANGES THE DEFAULT to 112 (user decision) since the redistribute source models an AS112 virtual router; unset asn means 112, not "local origin".
- [ ] `ai/rules/config-surface.md` - YANG vs env var for the new leaves
  → Constraint: default answer is YANG config; asn/community/watchdog are operator-facing feature toggles visible in `show configuration` -> YANG leaves, not env vars.
- [ ] `ai/patterns/config-option.md` - YANG leaf structure + validation
  → Constraint: every leaf gets max native validation; asn uses an ASN type with range, community uses the ze-types community type or a validated string, watchdog is boolean with `default true`.
- [ ] `ai/rules/plugin-self-containment.md` - as112 owns its config/schema/help; nothing AS112 in generic packages
  → Constraint: the origin-ASN + community fields added to `redistevents`/consumer must be generic (no "as112" spelling); as112 supplies the values, the pipeline stays protocol-agnostic.
- [ ] `ai/rules/doctor-checks.md` - the coordination doctor check update
  → Constraint: the new `asn 112 + import as112` uncoordinated-global-origin path belongs in the same neutral `internal/component/doctor` check as the existing `local 112 + replace-as` one.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7534.md` - AS112 Nameserver Operations
  → Constraint: §3.3 -- SHOULD NOT advertise the service prefix while DNS is not running (the watchdog default-true gate); §3.4 -- SHOULD restrict advertisement with a prefix filter + AS_PATH filter to dedicated peers (community + AS-PATH-origin are the match handles).
  → Constraint: §3.2/§5 -- coordinate before running a globally-reachable node (the doctor warning on `asn 112` to public peers).
- [ ] `rfc/short/rfc1997.md` - COMMUNITIES well-known values; `rfc/short/rfc3765.md` - NOPEER
  → Constraint: nopeer/no-export are the RFC-recommended communities for AS112 routes; the config `community` leaf must accept these well-known names.

**Key insights:**
- The plugin-to-BGP route-origination pipeline already exists end to end (`redistevents` producer -> `redistribute-orchestrator` -> `BGPConsumer.InjectRoute` -> SDK `UpdateRoute` -> reactor `AnnounceNLRIBatch`). The only missing generic capability is carrying an **origin ASN + community** on the redistributed route; the `update text` grammar already parses `as-path` and `community` (incl. well-known names).
- as112 becomes a producer exactly like L2TP/connected: an `events/` subpackage handle + `RegisterSource` + emit covering-prefix batches, self-gated on serving state.
- The layering rule holds: as112 never reads BGP config; BGP never learns "as112" (imports a generic named source).
- Two operator paths result: redistribute (this spec, easy) and the existing hand-authored `bgp { update } + watchdog + healthcheck` (full per-peer control). Docs must present both and when to use each.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/redistevents/events.go` - value-typed route-change payload. `RouteChangeEntry{Action, Prefix, NextHop, Metric, Table}`; `RouteChangeBatch{Protocol, AFI, SAFI, Entries}`. No ASN/community fields today.
  → Constraint: adding origin-ASN/community must keep value-type discipline; a `[]uint32` community slice is consistent with the existing `Entries []RouteChangeEntry` (read-only during synchronous dispatch, released after Emit) but must be nil for producers that don't set it (no alloc for L2TP/connected hot path).
- [ ] `internal/component/bgp/redistribute/consumer.go` - `BGPConsumer.InjectRoute` formats `update text origin incomplete nhop <self|addr> nlri <fam> add <prefix>` and dispatches to `UpdateRoute(ctx, "*", cmd)`. Hardcodes `origin incomplete`, no as-path, no community, selector `*` (all peers).
  → Constraint: this is where `as-path <asn>` and `community <...>` tokens must be appended when the (generic) route metadata carries them. Origin should become `igp` when an origin-ASN is set (locally-originated AS112 route), else preserve `incomplete`.
- [ ] `internal/component/config/redistribute/registry.go` - `RouteSource{Name, Protocol, Description}` + `RegisterSource` (idempotent same name+protocol, conflict on differing protocol). `LookupSource` gates `import <name>` validity.
  → Constraint: as112 registers `RouteSource{Name:"as112", Protocol:"as112"}`; the origin-ASN/community are per-route runtime values (from as112 config), NOT static source fields.
- [ ] `internal/component/config/loader_redistribute.go` - `ExtractRedistributeRules` reads `redistribute.destination[].import[]`, validates source names against the registry, collects optional `family` filters. Rule = `{Source, Families}` only; no policy/community/peer-selection.
  → Constraint: `import as112` parses for free once as112 registers the source; no loader change needed for the basic trigger. Any per-import attribute override is out of scope (attributes come from the as112 block).
- [ ] `internal/component/bgp/plugins/cmd/update/update_text.go` - runtime `update text` grammar. Supports `as-path <asn>...` (`kwASPath`, parses to `ASPath []uint32`, `SetASPath`), `community <tok>...` (`kwCommunity`, `parseCommunityText` -> `attribute.ParseCommunity`, well-known names incl. nopeer/no-export), `origin`, `nhop`, `nlri`.
  → Constraint: no grammar change needed -- the consumer just emits the tokens. as-path of length 1 = single origin ASN; the reactor prepends the local AS for eBGP peers on top.
- [ ] `internal/plugins/as112/config.go` - `as112Config{Enabled, AddressFamily, Hostname, Facility, Location, AllowFrom}` parsed from `service.as112`. `parseConfig` is the single source of truth (offline verifier + engine).
  → Constraint: add `ASN uint32` (default 112 when key absent), `Community []uint32` (parsed via canonical community parser), `Watchdog bool` (default true when key absent). Mirror the existing default-in-code pattern (`AddressFamily: addressFamilyBoth`).
- [ ] `internal/plugins/as112/health.go` - `runHealthQuery` issues one SOA query to a target (loopback by default), exit 0 iff NOERROR+SOA. On-demand, not a continuous FSM.
  → Constraint: the self-gate is a SERVING-STATE gate (server up + zones loaded + loopback answers), NOT a continuous anycast-path probe. Full anycast-path liveness stays with the manual healthcheck-probe path (documented limitation, reinforces two-paths story).
- [ ] `internal/plugins/as112/register.go` / `server.go` / `zones.go` - as112 runs in-process, binds host addresses via the iface registry, serves zones. No redistribute/redistevents/UpdateRoute/EventBus wiring today.
  → Constraint: as112 already refuses to run out-of-process; emitting on the in-process EventBus is consistent with that (same-process bus, no cross-boundary pointer).
- [ ] `internal/component/doctor/checks_as112_coordination.go` - `checkAS112GlobalOriginCoordination` warns on `asn.local 112 + replace-as` to a non-private remote ASN. `as112CoveringPrefixes` constant lives here.
  → Constraint: extend to also warn when `asn 112` (as112 block, incl. the default) is redistributed via `import as112` toward eBGP/public peers -- same uncoordinated-global-origin risk via the new path.

**Behavior to preserve:**
- The hand-authored `bgp { update { nlri } watchdog { withdraw true } }` + healthcheck composition keeps working unchanged (it is the full-control path). No change to watchdog/healthcheck plugins.
- as112 DNS serving, host-address binding, `allow-from`, `show as112`, `request as112 healthcheck` unchanged.
- Existing redistribute producers (l2tp/connected/static/fakeredist) keep their exact current wire output: no origin-ASN, no community, `origin incomplete`, when they don't set the new fields.
- The value-typed, pool-based `redistevents` contract (no per-event alloc for existing producers).

**Behavior to change:** (user-requested)
- as112 gains `asn` (default 112), `community` (optional), `watchdog` (default true) config leaves.
- as112 becomes a redistribute source/producer emitting its covering prefixes.
- The redistribute event payload + BGP consumer gain a generic origin-ASN + community capability.
- The doctor coordination check gains the `asn 112 + import as112` case.
- Docs present redistribute (easy) vs hand-authored (full-control) as two paths.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Config: `service { as112 { enabled true; asn <N>; community [ <name> ]; watchdog <bool> } }` (attributes + gate) AND `redistribute { destination bgp { import as112 } }` (the trigger).
- Runtime: as112 serving-state transitions (server up/down) drive emit.

### Transformation Path
1. as112 `parseConfig` reads asn/community/watchdog (defaults: asn=112, watchdog=true).
2. as112 reaches serving state -> acquires a `RouteChangeBatch` per family, sets Protocol=as112, OriginASN, Community, Entries=covering prefixes with Action=Add -> `Emit` on the as112 event handle -> `ReleaseBatch`.
3. `redistribute-orchestrator` (`redistribute_egress`) receives the batch, applies `configredist.Global().Accept(route, "bgp")` (gated by `import as112` rule) -> for each accepted entry `BGPConsumer.InjectRoute`.
4. `BGPConsumer.InjectRoute` formats `update text origin igp as-path <asn> [community <...>] nhop self nlri <fam> add <prefix>` -> `UpdateRoute(ctx, "*", cmd)`.
5. Engine dispatches to `ze-bgp:peer-update` -> `ParseUpdateText` -> `AnnounceNLRIBatch` -> reactor advertises; per-peer eBGP prepend adds the local AS on top of `[asn]`.
6. On serving-state loss / shutdown / `enabled false`: as112 emits Action=Remove -> `WithdrawRoute` -> `update text nlri <fam> del <prefix>`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| as112 plugin -> EventBus | value-typed `*RouteChangeBatch` (in-process pointer from bus, not producer memory retained) | [ ] |
| EventBus -> redistribute-orchestrator | synchronous fan-out per redistevents contract | [ ] |
| orchestrator -> reactor | text `update text ...` via SDK `UpdateRoute` | [ ] |
| config -> as112 | `service.as112` JSON section via OnConfigure/verifier | [ ] |
| config -> redistribute gate | `redistribute` block via `ExtractRedistributeRules` -> evaluator | [ ] |

### Integration Points
- `redistevents.RegisterProtocol/RegisterProducer` + `events.Register[*RouteChangeBatch]` - as112 producer registration.
- `redistribute.RegisterSource{Name:"as112",Protocol:"as112"}` - makes `import as112` valid.
- `BGPConsumer.InjectRoute` / `formatAnnounce` - the generic origin-ASN + community emission point.
- `internal/component/doctor/checks_as112_coordination.go` - coordination warning for the new path.

### Architectural Verification
- [ ] No bypassed layers (as112 -> bus -> orchestrator -> reactor, no direct reactor call from as112)
- [ ] No unintended coupling (as112 does not import bgp config; consumer changes carry no "as112" spelling)
- [ ] No duplicated functionality (reuses redistevents/orchestrator; does not re-implement watchdog pools)
- [ ] Zero-copy preserved where applicable (new batch fields are value types; nil for non-setting producers)
- [ ] Registration over hardcoding (as112 registers a source + producer; BGP discovers it via the registry; no per-feature switch in a core package)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A single-ASN AS_PATH on a locally-originated redistributed route yields `[asn]` to iBGP peers and `[localAS, asn]` to eBGP peers after the normal prepend | `update_text.go` SetASPath + reactor EBGP prepend | virtual-router semantics wrong; origin AS not as intended | interop test asserting AS_PATH on FRR for iBGP + eBGP | **BROKEN as originally designed, then RESOLVED via a new `origin-as` primitive.** The initial `as-path <asn>` approach does NOT prepend on eBGP: the announce path sends an explicit as-path VERBATIM (route-server transparency; `TestBuildBatchASPath_Explicit`), so eBGP would receive `[112]` and reject it (enforce-first-as). Replaced with an `origin-as` directive the reactor synthesizes to `[asn]` (iBGP) / `[localAS, asn]` (eBGP) -- unit-tested (`TestBuildBatchASPath_OriginAS_iBGP/_eBGP`); interop `as112-redistribute-origin-frr` asserts iBGP `[112]` + eBGP `[65001,112]` on the wire (CI). |
| A-2 | A `[]uint32` community field on `RouteChangeBatch`, nil for non-setting producers, adds no allocation to the L2TP/connected hot path | `redistevents/events.go` value-type contract; nil slice = no backing array | perf regression on burst redistribute | existing `bgp-redistribute-burst.ci` still passes; benchmark unchanged | **confirmed** -- nil `Community` = no backing array; existing producers' wire output unchanged (`TestBGPConsumerInjectRoute` back-compat green); `ReleaseBatch` nils (never clears) the producer-owned slice |
| A-3 | `import as112` needs no `loader_redistribute.go` change beyond as112 registering the source | `loader_redistribute.go` validates against registry only | `import as112` rejected as unknown source | `.ci`: `import as112` accepted, routes announced | **confirmed** -- `registerAS112Sources()` in init makes the source visible to the loader's registry lookup; no loader change; `redistribute-as112-announce.ci` drives `import as112` |
| A-4 | Well-known community names (nopeer/no-export) survive config -> uint32 -> `update text community` round-trip | `1034` learned (parseCommunityText -> attribute.ParseCommunity) | community dropped or misparsed on the wire | interop test asserting COMMUNITIES on FRR | **confirmed** -- config parses via `attribute.ParseCommunity` (`TestParseConfig_CommunityWellKnown`); consumer renders `<hi>:<lo>` which round-trips (NO_EXPORT 0xFFFFFF01 -> 65535:65281, `TestBGPConsumerInjectRouteCommunity`); interop `as112-redistribute-community-frr` for the wire proof (CI) |
| A-5 | as112's serving state (server up + zones loaded + loopback SOA answers) is a sufficient gate signal without a new continuous probe loop | `health.go` runHealthQuery is on-demand; server lifecycle has up/down transitions | announce fires before serving / never withdraws | `.ci`: announce after enable+ready, withdraw on disable | **confirmed** -- producer.apply gates on `serving` (mgr.apply success); unit-tested announce-when-serving / withhold-until-serving / withdraw-on-loss / watchdog-false (`TestAS112Producer_*`) |
| A-6 | The orchestrator/consumer path re-announces to a peer that establishes AFTER injection | agent trace of the redistribute -> reactor -> rib chain | AS112 route missing on late-joining / dynamic peers | code trace + `.ci` late-join test | **RESOLVED** -- dependency `spec-redistribute-late-join-replay` LANDED (`ba0085324`); as112 subscribes to `redistevents.ReplayRequestEvent` and re-emits the current set with the echoed `ReplayID` (`TestAS112Producer_ReemitOnReplay`) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Redistribute injects to `*` (all peers) -- AS112 routes reach peers the operator did not intend (no dedicated-peer-group restriction in the redistribute path) | route seen on a transit peer in interop/`.ci` | Document that per-peer restriction is via egress prefix/community filters or the full-control hand-authored path; community + AS-PATH-origin give operators the match handles; doctor warns on public-peer `asn 112` |
| R-2 | Serving-state gate is not anycast-path liveness (H1 false-positive class from 1034): loopback healthy but anycast path down | node announces while anycast unreachable | Documented limitation; operators needing anycast-path gating use the manual healthcheck-probe path (two-paths docs) |
| R-3 | `asn` default 112 + `import as112` toward public/eBGP peers = uncoordinated global AS112 origin (RFC 7534 §3.2/§5) | doctor warning; interop shows 112 origin to public peer | doctor-check extension warns; docs hard-warn; default is safe only for local-use/bilateral |
| R-4 | Adding fields to the shared `RouteChangeBatch` touches every producer/consumer (l2tp/connected/static/fakeredist) -- exhaustive-switch or struct-literal breakage | build/lint failure in fib-kernel/fib-vpp/fakeredist (see 641 gotcha) | Additive fields with zero-value defaults; grep all `RouteChangeBatch{`/`RouteChangeEntry{` literals; run all redistribute `.ci` |
| R-5 | Community as `[]uint32` risks retaining producer-owned backing array past dispatch | consumer reads stale/zeroed community after ReleaseBatch | Consumer consumes within the synchronous InjectRoute call; ReleaseBatch zeroes after Emit (641 gotcha) -- covered by the read-only-during-dispatch contract |
| R-6 | **CONFIRMED GAP: redistribute-injected routes are NOT delivered to a peer that first establishes AFTER injection.** Fan-out is to reactor-map peers at call time only (`reactor_api_batch.go:30-32`); `ribOut` is per-peer, populated only from `update direction sent` for peers actually sent (`rib.go:789-823`); new-peer establishment replays only `ribOut[peerAddr]` (nil for a fresh peer, `rib.go:1080`->`rib_replay.go:52-54`) + config `StaticRoutes` (`peer_initial_sync.go:65-326`). Affects dynamic/passive/template peers and config-added-after-emit peers. Static config peers present in the map at emit ARE covered (QueueAnnounce -> opQueue drain on establish). This is a GENERIC redistribute gap (l2tp/connected share it), not as112-specific; it is the concrete form of the deferred central-store idea in `634-bgp-redistribute.md` "Open Question". | any dynamic/late peer establishes and lacks the AS112 covering prefix; `.ci` late-join test fails | **RESOLVED (user decision): split into a dependency spec.** `spec-redistribute-late-join-replay` closes the gap generically (redistribute sources replayed to newly-establishing peers); this spec `Depends` on it and MUST NOT be claimed production-complete until it lands. Rejected: (A) folding the RIB fix into this spec (mixes concerns); (C) as112-specific re-emit on peer-up (duplicates watchdog, couples as112 to BGP peer events, violates no-workarounds + layering). Interim: functional/interop tests here establish peers before emit (the covered case); the late-join `.ci` belongs to the dependency spec. |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `redistribute { destination bgp { import as112 } }` + as112 enabled+serving | → | as112 producer Emit -> orchestrator -> `BGPConsumer.InjectRoute` -> `UpdateRoute` | `redistribute-as112-announce.ci` |
| `import as112` absent, as112 enabled | → | orchestrator evaluator rejects as112 batch (nothing announced) | `redistribute-as112-not-imported.ci` |
| as112 `asn 112` (default) | → | consumer emits `as-path 112` | interop `NN-as112-redistribute-origin-frr` (iBGP+eBGP AS_PATH) |
| as112 `community [ nopeer ]` | → | consumer emits `community nopeer` | interop `NN-as112-redistribute-community-frr` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | as112 enabled + serving, `import as112` configured | The four covering prefixes (per address-family) are announced to BGP peers; host /32//128 addresses are NOT announced |
| AC-2 | as112 enabled + serving, NO `import as112` | No AS112 route is announced (producer emits, evaluator rejects) |
| AC-3 | `service.as112.asn` unset | Routes originate with AS_PATH origin 112 (default-in-code) |
| AC-4 | `service.as112.asn 65001` | Routes originate with AS_PATH origin 65001 |
| AC-5 | `service.as112.community [ nopeer ]` | Announced routes carry the NOPEER well-known community |
| AC-6 | `service.as112.community` unset | Announced routes carry no COMMUNITIES attribute |
| AC-7 | `watchdog` unset (default) or true; DNS not yet serving | Route is withheld until serving; withdrawn when serving is lost / `enabled false` / shutdown |
| AC-8 | `watchdog false` | Route announced as soon as enabled + imported, without the serving-state gate |
| AC-9 | as112 disabled or plugin shutdown while imported | Previously announced covering prefixes are withdrawn |
| AC-10 | Existing redistribute producers (l2tp/connected/static) with new payload fields unset | Wire output unchanged: `origin incomplete`, no as-path, no community |
| AC-11 | `asn` 112 (incl. default) + `import as112` on an eBGP session to a non-private ASN | `ze doctor` emits a coordination warning (`doctor-as112-global-origin-uncoordinated` or a new sibling code) |
| AC-12 | asn is a validated ASN type; community is a validated well-known/standard community; watchdog is boolean | Invalid values rejected at config validation with a clear error |
| AC-13 | Lab scenario run | For every scenario (default-112, custom-asn, private-asn, community-set, health up->announce / down->withdraw, not-imported), the BGP announcement (or its absence) is observed on a real peer AND the as112 node answers an authoritative DNS query |
| AC-14 | a BGP peer establishes AFTER as112 has emitted, `import as112` configured (requires `spec-redistribute-late-join-replay`) | the new peer receives the covering prefixes: the as112 producer subscribes to `redistevents.ReplayRequest` and re-emits its current set with `Replay:true`, and the orchestrator targets the new peer. (Moved here from `spec-redistribute-late-join-replay` AC-5; closes R-6 for as112.) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables as112 + `import as112`, sees AS112 routes on a peer | config -> as112 emit -> orchestrator -> InjectRoute -> reactor -> peer | `redistribute-as112-announce.ci` |
| 2 | Sets `asn 65001`, peer sees origin 65001 | as112 config -> OriginASN on batch -> `as-path 65001` -> wire | interop `NN-as112-redistribute-origin-frr` |
| 3 | Sets `community [ no-export ]`, peer sees COMMUNITIES | as112 config -> Community on batch -> `community no-export` -> wire | interop `NN-as112-redistribute-community-frr` |
| 4 | Stops DNS (watchdog true), route is withdrawn | serving-state loss -> Remove batch -> WithdrawRoute -> wire withdraw | `redistribute-as112-watchdog-withdraw.ci` |
| 5 | Queries the node for `10.in-addr.arpa` SOA and gets the AS112 answer while the route is up | DNS server serving on anycast/loopback | lab scenario (AC-13) |
| 6 | Wants per-peer control, follows docs to the hand-authored path instead | docs two-paths section | `docs/guide/as112.md` review |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseConfig_ASNDefault112` | `internal/plugins/as112/config_test.go` | asn unset -> 112 | |
| `TestParseConfig_ASNExplicit` | `internal/plugins/as112/config_test.go` | asn 65001 parsed | |
| `TestParseConfig_WatchdogDefaultTrue` | `internal/plugins/as112/config_test.go` | watchdog unset -> true | |
| `TestParseConfig_CommunityWellKnown` | `internal/plugins/as112/config_test.go` | `nopeer`/`no-export` parsed to correct uint32 | |
| `TestParseConfig_InvalidASN` / `_InvalidCommunity` | `internal/plugins/as112/config_test.go` | rejected with error | |
| `TestEmit_CoveringPrefixesPerFamily` | `internal/plugins/as112/<producer>_test.go` | Add batch carries the 4 covering prefixes, OriginASN, Community, per family | |
| `TestEmit_WithdrawOnServingLoss` | `internal/plugins/as112/<producer>_test.go` | Remove batch only for families that were up | |
| `TestInjectRoute_EmitsAsPathAndCommunity` | `internal/component/bgp/redistribute/consumer_test.go` | OriginASN/Community set -> `origin igp as-path <n> community <...>`; unset -> `origin incomplete`, no tokens | |
| `TestRouteChangeBatch_ZeroValueBackCompat` | `internal/core/redistevents/*_test.go` | zero OriginASN + nil Community -> legacy behavior | |
| `TestDoctor_AS112RedistributeOriginUncoordinated` | `internal/component/doctor/checks_as112_coordination_test.go` | asn 112 + import as112 to public ASN warns; private ASN exempt | |
| `TestAS112ProducerReEmitsOnReplayRequest` | `internal/plugins/as112/<producer>_test.go` | on `redistevents.ReplayRequest` (from `spec-redistribute-late-join-replay`) the as112 producer re-emits its covering prefixes with `Replay:true`; unhandled when late-join not yet present = no-op. (Moved here from `spec-redistribute-late-join-replay`.) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `asn` | 1 .. 4294967295 (0 reserved) | 4294967295 | 0 | N/A (uint32 max) |
| `community` (standard) | `asn:value`, each 0..65535 | 65535:65535 | N/A | 65536:0 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `redistribute-as112-announce` | `test/plugin/*.ci` | enable + import -> 4 covering prefixes on peer | |
| `redistribute-as112-not-imported` | `test/plugin/*.ci` | enabled, no import -> nothing announced | |
| `redistribute-as112-origin-default-112` | `test/plugin/*.ci` | asn unset -> AS_PATH origin 112 | |
| `redistribute-as112-origin-custom` | `test/plugin/*.ci` | asn 65001 -> AS_PATH origin 65001 | |
| `redistribute-as112-community` | `test/plugin/*.ci` | community nopeer -> COMMUNITIES on wire | |
| `redistribute-as112-watchdog-withdraw` | `test/plugin/*.ci` | serving loss / disable -> withdraw | |
| `redistribute-as112-watchdog-off` | `test/plugin/*.ci` | watchdog false -> announce without gate | |
| `redistribute-as112-hostaddr-not-announced` | `test/plugin/*.ci` | /24 announced, /32 not | |
| `redistribute-existing-producers-unchanged` | reuse `bgp-redistribute-*.ci` | l2tp/connected wire output unchanged | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-as112-redistribute-origin-frr` | `test/interop/scenarios/` | FRR | AS_PATH origin 112 (iBGP) and `[localAS,112]` (eBGP), plus custom/private asn variants | |
| `NN-as112-redistribute-community-frr` | `test/interop/scenarios/` | FRR/BIRD | COMMUNITIES (nopeer/no-export) present on the wire | |

### Lab (demonstration -- AC-13)
**Form (user decision): one `test/interop/scenarios/` dir with a real peer daemon (FRR/BIRD), bundling every scenario, each step observing the BGP announcement (or its absence) on the peer AND issuing an authoritative DNS query against the as112 node.** Matches the existing `as112-origin-as-frr/` / `as112-community-frr/` pattern and runs in CI. Proposed dir: `test/interop/scenarios/NN-as112-redistribute-lab/`.

| Scenario in the lab | Demonstrates |
|---------------------|--------------|
| default (asn unset) | BGP announce with origin 112 + DNS SOA answer |
| custom operator asn | BGP announce with operator origin + DNS answer |
| private asn (+ note remove-private-as on borders) | BGP announce with private origin + DNS answer |
| community set | COMMUNITIES on announced route + DNS answer |
| health up -> announce, health down -> withdraw | watchdog default-true gate + DNS answer state |
| not imported | no announcement while DNS still answers |

### Future (if deferring any tests)
- None planned; all scenarios covered by functional + interop + lab.

## Files to Modify
- `internal/core/redistevents/events.go` - add generic `OriginASN uint32` + community field to `RouteChangeBatch` (value-typed; nil/zero default)
- `internal/component/config/redistribute/consumer.go` - carry OriginASN/community to the consumer `RouteEntry`
- `internal/component/bgp/redistribute/consumer.go` - `formatAnnounce` emits `origin igp`/`as-path`/`community` when set (`// Design: docs/architecture/core-design.md`)
- `internal/component/bgp/plugins/redistribute_egress/redistribute.go` - thread batch OriginASN/community through `handleBatch` -> `dispatchEntryToConsumer`
- `internal/plugins/as112/config.go` - parse `asn` (default 112), `community`, `watchdog` (default true)
- `internal/plugins/as112/yang/ze-as112-conf.yang` - new leaves with native validation
- `internal/plugins/as112/register.go` / `server.go` - register source + producer; emit on serving-state transitions
- `internal/component/doctor/checks_as112_coordination.go` - warn on `asn 112 + import as112` to public/eBGP peers
- `internal/core/diagnostic/codes.go` - register the new diagnostic code if a new one is added
- `internal/component/plugin/all/all.go` - blank-import the as112 fake producer for `.ci` (regen via `make generate`)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] Yes | `internal/plugins/as112/yang/ze-as112-conf.yang` - `asn`, `community`, `watchdog` leaves |
| YANG validation constraints | [ ] Yes | asn: ASN type/range; community: ze-types community type or validated pattern; watchdog: boolean `default true` |
| YANG custom validators | [ ] Maybe | community well-known-name completion via `CompleteFn` if native enum insufficient |
| CLI commands/flags | [ ] No | no new verb; `show as112` MAY gain announce state (optional, see Known Limitations) |
| CLI grammar (action before identifier) | [ ] N/A | no new command grammar |
| Editor autocomplete | [ ] Yes | automatic for enum/boolean leaves; community completion via validator if custom |
| Functional test for new RPC/API | [ ] Yes | `test/plugin/redistribute-as112-*.ci` |
| Pipe completeness | [ ] N/A | no new output-producing command |
| Env var registration | [ ] N/A | all new settings are YANG operator-facing |
| Doctor check for runtime dependencies | [ ] Yes | extend `checks_as112_coordination.go`; code in `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | [ ] Yes | reuse `ze_bgp_redistribute_*`; consider as112-source announce/withdraw counters; list names during design |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] Yes | `docs/features.md` (AS112 row: redistribute origination) |
| 2 | Config syntax changed? | [ ] Yes | `docs/guide/configuration.md` (as112 asn/community/watchdog; `import as112`) |
| 3 | CLI command added/changed? | [ ] No | grep `docs/guide/command-reference.md` for as112 to confirm |
| 4 | API/RPC added/changed? | [ ] No | no new RPC (reuses UpdateRoute) |
| 5 | Plugin added/changed? | [ ] Yes | `docs/guide/plugins.md` (as112 now a redistribute source) |
| 6 | Has a user guide page? | [ ] Yes | `docs/guide/as112.md` - add redistribute (easy) path + keep hand-authored (full-control) path; state when to use each |
| 7 | Wire format changed? | [ ] No | as-path/community are existing attributes |
| 8 | Plugin SDK/protocol changed? | [ ] Maybe | if `redistevents` payload gains fields: `ai/rules/plugin-design.md`, process-protocol doc |
| 9 | RFC behavior implemented? | [ ] Yes | `rfc/short/rfc7534.md` (§3.3/§3.4), rfc1997/rfc3765 cross-refs |
| 10 | Test infrastructure changed? | [ ] Yes | `docs/functional-tests.md` (fake as112 producer, lab scenario) |
| 11 | Affects daemon comparison? | [ ] Maybe | `docs/comparison.md` if redistribute-from-service is a comparison row |
| 12 | Internal architecture changed? | [ ] Yes | `docs/architecture/core-design.md` (redistribute origin-ASN/community capability) |
| 13 | Route metadata keys added/changed? | [ ] No | no meta keys; attributes on the route itself |
| 14 | Prometheus counters added/changed? | [ ] Maybe | if as112-source counters added: telemetry doc |
| 15 | Registered plugin/source/producer changed? | [ ] Yes | `docs/plugin-overview.md`/`docs/features/plugins.md` (new redistribute source "as112") |
| 16 | Changed source referenced by doc source anchors? | [ ] Yes | grep `docs/` for anchors on edited files |
| 17 | Existing docs show config/CLI examples for this area? | [ ] Yes | verify `docs/guide/as112.md` examples against new YANG |

## Files to Create
- `internal/plugins/as112/events/events.go` (+ `_test.go`) - typed EventBus handle (l2tp/events precedent)
- `internal/plugins/as112/<producer>.go` (+ `_test.go`) - emit covering-prefix batches on serving-state transitions
- `internal/test/plugins/fakeas112/` - test-only producer for `.ci` (fakel2tp/fakeredist precedent), blank-imported in production all.go
- `test/plugin/redistribute-as112-*.ci` - functional tests (announce, not-imported, origin-default/custom, community, watchdog-withdraw, watchdog-off, hostaddr-not-announced)
- `test/interop/scenarios/NN-as112-redistribute-origin-frr/` and `NN-as112-redistribute-community-frr/` - on-the-wire AS_PATH + COMMUNITIES
- `test/interop/scenarios/NN-as112-redistribute-lab/` - the AC-13 lab: FRR/BIRD peer, all scenarios, each with BGP announcement observation + DNS query steps

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-verify` |
| 7-10. Critical review loop | Critical Review Checklist |
| 11. Deliverables review | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 14. Present summary | Executive Summary |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** - register `RouteSource{as112,as112}` + protocol/producer + as112 `events/` handle; add `import as112` accept path; write failing `redistribute-as112-announce.ci` + `redistribute-as112-not-imported.ci`.
   - Tests: `redistribute-as112-announce.ci`, `redistribute-as112-not-imported.ci`
   - Files: `as112/register.go`, `as112/events/`
   - Verify: `import as112` accepted by config validation; wiring test fails because as112 emits nothing yet.
2. **Phase: redistribute payload origin-ASN + community (generic)** - add fields to `RouteChangeBatch`, thread through orchestrator to consumer `RouteEntry`, `formatAnnounce` emits tokens; keep existing producers unchanged.
   - Tests: `TestInjectRoute_EmitsAsPathAndCommunity`, `TestRouteChangeBatch_ZeroValueBackCompat`, reuse `bgp-redistribute-*.ci`
   - Files: `redistevents/events.go`, `config/redistribute/consumer.go`, `bgp/redistribute/consumer.go`, `redistribute_egress/redistribute.go`
   - Verify: tokens emitted only when set; existing producers unchanged.
3. **Phase: as112 config leaves** - parse asn (default 112) / community / watchdog (default true) with YANG validation.
   - Tests: config_test.go rows above
   - Files: `as112/config.go`, `as112/yang/ze-as112-conf.yang`
   - Verify: defaults + validation.
4. **Phase: as112 producer + serving-state gate** - emit Add on serving, Remove on loss/disable/shutdown; per-family batches. Also subscribe to `redistevents.ReplayRequest` (from `spec-redistribute-late-join-replay`) and re-emit the current covering-prefix set with `Replay:true` so peers establishing after emit receive the routes (AC-14; closes R-6 for as112).
   - Tests: emit tests + `redistribute-as112-*` functional tests + `TestAS112ProducerReEmitsOnReplayRequest`
   - Files: `as112/<producer>.go`, `as112/server.go`
   - Verify: announce/withdraw fire on the right transitions; a late-joining peer receives the covering prefixes via the replay path.
5. **Phase: doctor coordination extension** - warn on asn 112 + import as112 to public peers.
   - Tests: `TestDoctor_AS112RedistributeOriginUncoordinated`
   - Files: `checks_as112_coordination.go`, `diagnostic/codes.go`
6. **Phase: interop + lab** - FRR/BIRD scenarios for AS_PATH + community; lab bundling all scenarios + DNS resolution.
7. **Docs** - two-paths `docs/guide/as112.md`, features/plugins/config/core-design updates with source anchors.
8. **Full verification** - `make ze-verify`; **Complete spec** - audit tables + learned summary; two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | Every user story path connected; lab covers every scenario + DNS |
| Correctness | AS_PATH origin correct for iBGP vs eBGP; withdraw fires on every serving-loss path |
| Naming | YANG kebab-case; no "as112" spelling in generic redistevents/consumer code |
| Data flow | as112 never imports bgp config; consumer stays protocol-agnostic |
| CLI grammar | N/A (no new command) |
| Registration over hardcoding | as112 source/producer via registry; no core switch case |
| Doctor checks | new/extended coordination warning registered + tested |
| YANG validation | asn/community/watchdog all have max native constraints |
| Prometheus counters | redistribute counters cover as112; any new counters registered |
| Rule: no-layering | generic capability carries no AS112 knowledge into BGP |
| Rule: back-compat | existing redistribute producers' wire output unchanged (AC-10) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| as112 source registered | `import as112` validates; source appears in the registry |
| covering prefixes announced | `redistribute-as112-announce.ci` pass |
| origin ASN on wire | interop scenario AS_PATH assertion |
| community on wire | interop scenario COMMUNITIES assertion |
| watchdog gate | withdraw `.ci` pass |
| existing producers unchanged | `bgp-redistribute-*.ci` pass |
| doctor warning | `TestDoctor_AS112RedistributeOriginUncoordinated` pass |
| lab | lab scenario run demonstrates all BGP cases + DNS resolution |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | asn/community rejected when malformed at config time, not at emit |
| Route leak | `*` selector + no dedicated-peer-group restriction documented; doctor warns on public-peer 112 |
| Resource exhaustion | per-family batch is bounded (4 prefixes); no unbounded emit loop |
| Origin spoofing | community/origin-AS are operator config, not attacker-controllable |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails behavior mismatch | Re-read Current Behavior source |
| Existing producer `.ci` regressed | Back-compat broken -> fix additive-field defaults |
| Lint failure | Fix inline; if architectural -> DESIGN |
| 3 fix attempts fail | STOP, report, ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The `as-path <asn>` token would get the normal eBGP localAS prepend ("no grammar change needed", spec Key insights) | The announce path sends an explicit `as-path` VERBATIM with no eBGP prepend (route-server transparency / ExaBGP exact-path); only the FORWARD path prepends (`TestBuildBatchASPath_Explicit`, `writeMandatoryAttrs`) | Code review of `buildBatchASPath`/`writeMandatoryAttrs` + the existing test; raised in review | Added a new `origin-as` reactor primitive (grammar token + `NLRIBatch.OriginAS` + `buildBatchASPath`/`writeASPath` branch) so a redistributed origin AS gets the normal iBGP/eBGP rule; `as-path` verbatim contract preserved |
| The as112 producer was wired into the engine | `prod.apply(cfg, serving)` was never called from `OnConfigure` -- producer created but never driven | Interop reviewer code inspection | Added the `prod.apply` call in `OnConfigure`; the real-plugin path is guarded by interop, not unit tests |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Generic redistribute as-is (no attributes) | drops community + injects to `*` with origin incomplete -> regresses RFC 7534 §3.4 policy | add generic origin-ASN + community to the redistribute path |
| as112 calls `UpdateRoute` directly, ungated by config | breaks the "import as112 triggers it" requirement; as112 cannot read bgp config to self-gate | producer pattern: as112 always emits, `import as112` gates via the evaluator |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- The origin-ASN + community capability is genuinely generic: `import static { }` could originate as an AS too. as112 is just the first consumer; the feature is "redistribute a source as if it were a neighbor AS."
- Two operator paths fall out naturally and should be documented as such: redistribute (easy, source-level attributes, process-health gate) vs hand-authored update-block (full per-peer policy, anycast-path probe gate).

## Core Insight
Modeling as112 as a virtual router with an ASN turns the earlier per-peer `replace-as` origin hack into an intrinsic, single-knob property of the route source, and gives operators an AS_PATH match handle for egress policy in place of AS112-specific BGP knowledge.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| as112 as a redistevents producer | direct ungated UpdateRoute; extend redistribute rules with policy | producer pattern satisfies the `import as112` trigger and preserves layering (as112 never reads bgp config) |
| origin-ASN + community on `RouteChangeBatch` (batch-level) | per-entry; per-source registry lookup | attributes are constant per as112 source; batch-level is value-typed, low-frequency, reaches the consumer via existing translation |
| default asn = 112 | default = operator local AS (old design) | user decision: the source models an AS112 virtual router, so 112 is the natural default; operator/private are explicit overrides |
| serving-state gate for watchdog | continuous anycast-path probe in as112 | minimal; the anycast-path probe already exists as the manual healthcheck path (two-paths story); avoids re-implementing the healthcheck FSM |
| community as `[]uint32` well-known/standard | large/extended communities | RFC 7534 recommends standard well-known (nopeer/no-export); large/extended out of scope |

## Known Limitations
- **Late-join gap (R-6): until `spec-redistribute-late-join-replay` lands, peers that first establish AFTER as112 emits (dynamic/passive/template peers, config-added-later peers) do not receive the covering prefixes.** Static config peers present at emit are covered. This spec is BLOCKED from a production-complete claim on that dependency; do not close it as done while R-6 is open.
- Serving-state gate is process/loopback health, not full anycast-path liveness (use the manual healthcheck-probe path for that).
- Redistribute injects to all peers (`*`); dedicated-peer-group restriction is via egress filters or the full-control path, not the redistribute block.
- Large/extended communities out of scope for the as112 `community` leaf.
- `show as112` announce-state surfacing is optional (nice-to-have), not required by an AC.

## RFC Documentation
Add `// RFC 7534 Section 3.3/3.4: "<quoted requirement>"` above the serving-state gate and the community/AS-PATH emission; `// RFC 3765` above NOPEER handling.

## Implementation Summary

### What Was Implemented
- Generic origin-ASN + community capability on the redistribute path (`RouteChangeBatch.OriginASN`/`.Community`), threaded orchestrator → consumer `RouteEntry`; `formatAnnounce` emits `origin igp origin-as <asn> [community [<hi>:<lo> ...]]`; existing producers byte-for-byte unchanged.
- New reactor primitive `origin-as` (update-text token → `NLRIBatch.OriginAS` → `buildBatchASPath`/`writeASPath`): AS_PATH `[asn]` to iBGP, `[localAS, asn]` to eBGP (normal export rule); verbatim `as-path` (route-server transparency) untouched.
- as112 config leaves `asn` (default 112) / `community` / `watchdog` (default true) + YANG; producer reconciler emitting the four covering prefixes, gated on enabled + live serving state; late-join replay via `redistevents.ReplayRequestEvent` → `reemitAll`.
- Runtime serving-state gate: the DNS server's anycast listener transitions (bind / Stop / crash) drive `onServingChanged`; the producer reads serving live and withdraws/re-announces.
- Doctor `checkAS112RedistributeOriginCoordination` + diagnostic code `doctor-as112-redistribute-origin-uncoordinated`.
- `fakeas112` test producer + 5 functional `.ci`; 4 interop scenarios (FRR/BIRD AS_PATH + community + DNS SOA, CI); two-paths docs.

### Bugs Found/Fixed
- **Feature-not-wired**: producer was created but `OnConfigure` never called `prod.apply` (caught by the interop reviewer via code inspection) → wired.
- **Runtime serving loss never withdrew** (review): only config-apply gated → wired the DNS listener transitions to the producer.
- **Concurrent-edge serving-snapshot race** (review): a stored `serving` bool could be set inconsistently by two concurrent listener edges → removed the snapshot; producer reads serving live (converges), guarded by a `-race` concurrency test.

### Documentation Updates
- `docs/guide/as112.md` (two-paths: redistribute vs hand-authored), `docs/guide/configuration.md`, `docs/guide/plugins.md`, `docs/features.md`, `docs/architecture/core-design.md`, `docs/plugin-overview.md`; every factual claim source-anchored; `ze-doc-test` source-anchor check passes (an unrelated `ze-show:errors`/`ze-show:interface` drift from a concurrent session is the only doc-test red).

### Deviations from Plan
- The spec's Key insight "no grammar change needed -- the consumer just emits the tokens" was WRONG: the announce path sends an explicit `as-path` VERBATIM (route-server transparency / ExaBGP exact-path), so `as-path <asn>` would reach an eBGP peer as `[asn]` and be rejected by enforce-first-as. Per user decision, added a NEW `origin-as` reactor primitive so a redistributed origin AS gets the normal iBGP/eBGP rule while `as-path` verbatim is preserved.
- AC-14 / Phase-4 `Replay:true` language reconciled to the landed `ReplayID` opaque-token API.
- The watchdog gate is a listener-liveness gate (config-apply + runtime listener up/down/crash), not a per-query/anycast-path health probe; the latter stays with the hand-authored healthcheck path (R-2, two-paths story).

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
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| as112 announces covering prefixes via `import as112` | functional test | `redistribute-as112-announce.ci` |
| origin ASN (default 112 / custom / private) on the wire | interop test | `NN-as112-redistribute-origin-frr` |
| well-known community on the wire | interop test | `NN-as112-redistribute-community-frr` |
| watchdog default-true health gate | functional test | `redistribute-as112-watchdog-withdraw.ci` |
| all scenarios + DNS resolution demonstrated | lab | lab scenario run (AC-13) |

## Review Gate

Adversarial `/ze-review` run to convergence (2 full passes + 2 focused delta passes).

### Run 1 (two full passes: correctness/wiring + security/perf/RFC/coverage)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | watchdog serving gate only ran at config-apply; runtime listener loss never withdrew (contradicts AC-7) | as112/register.go | Fixed → runtime serving wiring (see Run 2) |
| 2 | ISSUE | seam comments said consumer emits `as-path`; it emits `origin-as` | redistevents/events.go, config/redistribute/consumer.go | Fixed comments |
| 3 | NOTE | community leaf-list unbounded (UPDATE-size overflow) | as112/yang, as112/config.go | Fixed: `max-elements 32` + parse-time count guard + test |
| 4 | NOTE | no as112-specific late-join `.ci` | test/plugin | Accepted: covered by unit test + generic `redistribute-late-join-configadd.ci` |
| 5 | NOTE | per-UPDATE slice alloc on origin-as path; fakeas112 accepts asn 0 | reactor_api_batch.go, fakeas112 | Accepted: low-volume (not hot path); test-only |

### Run 2 (delta re-review of the runtime-serving wiring)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | `onServing` bool snapshot could be set inconsistently by two concurrent listener edges (announced-while-not-serving) | as112/server.go, redistribute.go | Fixed → removed the snapshot; producer reads LIVE serving via `servingFn`=`mgr.serving` on each reconcile (converges); + concurrency `-race` test |

### Run 3 (delta re-review of the live-read fix)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | `servingFn` field write after `subscribeReplay` (race-free only by non-obvious argument) | as112/register.go | Fixed: wired before `subscribeReplay` |
| 2 | NOTE | concurrency convergence + real `p.mu→s.mu` nesting untested | as112/redistribute_test.go | Fixed: `TestAS112Producer_ConcurrentServingConverges` (lock-taking servingFn, concurrent under `-race`) |

### Final status
- [x] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE (Run 3 confirmed the prior finding resolved with no new BLOCKER/ISSUE)
- [x] All NOTEs recorded above (fixed or accepted with rationale)

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
- [ ] AC-1..AC-13 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Lab demonstrates all BGP scenarios + DNS resolution (AC-13)
- [ ] Documentation two-paths section present
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (origin-ASN/community generic capability has as112 as first user; static a plausible second)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (layering invariant holds)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for asn/community
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for AS_PATH + community on the wire
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all checks documented
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-as112-bgp-redistribute.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-as112-bgp-redistribute.md`
