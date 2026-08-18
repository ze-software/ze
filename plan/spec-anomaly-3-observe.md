# Spec: anomaly-3-observe (behavioral anomaly incident lifecycle store + `show anomaly observe`)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | 1048 |
| Phase | 1/11 |
| Updated | 2026-08-17 |

Child 3 of the behavioral-anomaly umbrella (`plan/spec-anomaly-0-umbrella.md`, the
`observe` row of the Child Spec Roadmap). Adds the incident **lifecycle** store
(open on `AnomalyDetected`, finalize on `AnomalyCleared` with `EndTime`/`Active`)
plus a NEW `show anomaly observe` query surface the detect recent-incident ring
(Detected-only, no `EndTime`, no persistence beyond 128 entries) lacks.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-anomaly-0-umbrella.md` (child 3 scope, verified constraints, fact/judgment/response line)
4. `internal/plugins/ddos/observe/{store.go,register.go,show.go,config.go}` - the skeleton to reuse
5. `internal/core/anomalyevent/event.go` - the event contract subscribed to
6. `internal/plugins/anomaly/detect/{register.go,show.go,detector.go}` - the sibling producer + its show wiring

## Task

The behavioral anomaly detector (`anomaly/detect`, closed as learned 1048) emits
`anomalyevent` incidents and keeps only a bounded, in-memory, Detected-only ring
(`detector.inc`, capped at `incidentRingSize = 128`, `detector.go,297-300`)
surfaced by `show anomaly detect`. That ring holds no incident **lifecycle**: it
never records when an incident cleared, so an operator cannot see a finalized
incident's duration or query recent history.

This child adds a self-contained `anomaly-observe` system plugin under
`internal/plugins/anomaly/observe/` that:

1. subscribes to the SAME `anomalyevent` typed events (`Detected`/`Ongoing`/`Cleared`)
   the responder consumes, keyed on the SOURCE `netip.Prefix` entity;
2. maintains a bounded incident **lifecycle** ring: `open()` on `AnomalyDetected`,
   `finalize()` on `AnomalyCleared` (sets `EndTime` + `Active=false`), stale-sweep
   for open incidents that never clear;
3. exposes a NEW `show anomaly observe` query surface (wire method
   `ze-show:anomaly-observe`) returning the lifecycle list, including finalized
   incidents with their `EndTime` - the history the detect ring cannot show;
4. AUGMENTS `/ad:anomaly` with `container observe` (config: ring size + stale
   timeout), exactly as `ddos/observe` augments `/dd:ddos`.

It is NOT a bare mirror of `ddos/observe`: the template's incident struct is a
destination `VectorTuple`; this child's is a source `netip.Prefix`. It subscribes
to `anomalyevent`, not `ddosevent`. Crucially it also FIXES a latent template gap:
`ddos/observe.sweepStale()` is never wired to a ticker in production
(`store_test.go` is its only caller), so its `stale-incident-timeout` leaf is
dead. This child wires the stale sweep so open-but-never-cleared incidents finalize.

In-memory ring only (no durable store exists in the repo; verified). No web card
(none exists for ddos or anomaly anywhere; out of scope).

## Required Reading

### Architecture Docs
- [ ] `ai/patterns/plugin.md` - system-plugin structural template
  → Constraint: logger is `atomic.Pointer[slog.Logger]` (not a plain var) because tests run in-process plugin instances concurrently; `register.go` init() calls `registry.Register`; `make generate` regenerates `all.go` blank imports.
  → Constraint: plugins NEVER import sibling plugins; cross-plugin data rides typed events / DispatchCommand. `anomaly-observe` reaches the detector's output ONLY via the `ze.EventBus` typed events, never by importing `anomaly/detect`.
- [ ] `ai/patterns/registration.md` - all registration mechanisms
  → Decision: the show command uses TWO registries at once: the RPC handler via `pluginserver.RegisterRPCs(RPCRegistration{WireMethod, Handler})` in `show.go`, and the CLI path via a `ze:command "ze-show:anomaly-observe"` node in a `cmd/yang/` module (`configyang.RegisterModule`). WireMethod→CLI-path is unioned by the YANG loader.
  → Constraint: a NEW `<plugin>/yang/` and `<plugin>/cmd/yang/` package (each with a `register.go` importing `config/yang`) is auto-discovered; run `go run scripts/codegen/plugin_imports.go` (or `make generate`) to refresh `all.go`, then `-update` the `plugins`/`wire-methods` snapshots.
- [ ] `ai/rules/plugins.md` - the removal test
  → Constraint: deleting `internal/plugins/anomaly/observe/` + its `all.go` imports MUST remove the config subtree, the `show anomaly observe` node, its handler, its YANG, and its store together, leaving `anomaly/detect`, `anomaly/shape`, and the core building green. The command schema lives in the plugin's own `cmd/yang/`, NOT any central `show` verb schema.
  → Constraint: `anomaly` is a shared namespace container: `show anomaly detect` (detect plugin), `show anomaly shape` (shape plugin), and `show anomaly observe` (this plugin) each container-merge one child node onto the same `container show { container anomaly {...} }`. Add a `self_containment_test.go` in `cmd/yang/` asserting the `ze:command` + container tokens are declared here (mirror `ddos/observe/cmd/yang/self_containment_test.go`).
- [ ] `ai/rules/repo-maintenance.md` - runtime-dependency checks
  → Decision: N/A. The store is in-memory subscribed to the in-process event bus; it introduces no file path, socket, listen port, kernel module, external binary, cert, procfs/sysctl, or netlink use, so no runtime-dependency doctor check applies. `ddos/observe` (the mirror) registers none. The "detect disabled" case is already covered by `anomaly-detect`'s own `anomaly-detect-feature-source` check (an empty store is harmless).

### Reference Implementations (grounding, not doc)
- [ ] `internal/plugins/ddos/observe/store.go` - the ring skeleton to reuse
  → Constraint: reuse `newStore(cap, staleTimeout)`, the `mu`/`ring`/`cap`/`nextID`/`staleTimeout` fields, and the method set `open`/`finalize`/`activeCount`/`count`/`list`/`sweepStale`/`evictOldest` VERBATIM in shape, changing ONLY the incident struct (dest `VectorTuple` → source `netip.Prefix`) and the finalize match key (`Target.DstPrefix` → `Entity`). `list()` returns newest-first (`store.go`); `evictOldest()` drops the oldest FINALIZED incident, falling back to the head (`store.go`).
  → Constraint: `sweepStale()` exists (`store.go`) but is DEAD in the ddos template (only `store_test.go` calls it). This child MUST wire it to a ticker so `stale-incident-timeout` is functional.
- [ ] `internal/plugins/ddos/observe/register.go` - the SDK lifecycle + subscribe pattern
  → Constraint: `activeStore atomic.Pointer[store]` publishes the live ring to the in-process show handlers (`register.go`). Subscribe in `OnConfigure`; re-subscribe (unsub old, new store, subscribe) in `OnConfigApply`; `defer activeStore.Store(nil)`. `ConfigureEventBus` stores `eventBusPtr`; `loadBus()` errors if unset. `Run` budgets: `VerifyBudget: 2`, `ApplyBudget: 10`.
- [ ] `internal/plugins/ddos/observe/show.go` - the in-process show handler pattern
  → Constraint: handler reads `activeStore.Load()`; nil → `{enabled:false, incidents:[]incident{}}`; else `{enabled:true, active-count, incidents: s.list()}` in a `plugin.Map`, `Status: plugin.StatusDone`. Register via `pluginserver.RegisterRPCs` in a `show.go` init().
- [ ] `internal/plugins/ddos/observe/config.go` - config parse/validate + wrapping
  → Constraint: `configRoot = "anomaly/observe"`; the section is delivered wrapped as `{"anomaly":{"observe":{...}}}` by `ExtractConfigSubtree`; `ParseConfig` unwraps both levels. Defaults `IncidentRingSize=1000`, `StaleIncidentTimeout=3600`; `Validate` ranges `[1,100000]` and `[1,86400]`. No `enabled` leaf - the store is always-on when the plugin is loaded (uniform with `ddos/observe`).
- [ ] `internal/core/anomalyevent/event.go` - the event contract subscribed to
  → Constraint: namespace `"anomaly-detect"` (`event.go`); `Detected`/`Ongoing`/`Cleared` registered via `events.Register[T]` (`event.go`); subscribe with `anomalyevent.Detected.Subscribe(bus, func(*AnomalyDetected))`. `AnomalyDetected` carries `Interface,Entity netip.Prefix,Cohort,FiredFeatures []FeatureSignal,Score,Severity,At time.Time,Observable` (`event.go`). `AnomalyCleared` carries `Entity netip.Prefix,Observable` (`event.go`) - finalize matches on `Entity`. `AnomalyOngoing` carries `Entity,Score,Observable` (`event.go`).
- [ ] `internal/plugins/anomaly/detect/register.go` + `show.go` + `detector.go` - the sibling producer
  → Constraint: detect emits from `activate`/`emitOngoing`/`emitCleared` (`detector.go`); the recent ring is `detector.inc []AnomalyDetected` capped at 128 (`detector.go,297-300`), surfaced by `handleShowAnomaly` / wire method `ze-show:anomaly` (`show.go`). This is the Detected-only surface with NO lifecycle - the exact gap this child fills.
  → Decision: for a background ticker inside the SDK lifecycle, mirror detect's `startTicker`/`stopTicker` (`register.go`): a `stopCh chan struct{}` + `sync.WaitGroup` with `wg.Go`, `time.NewTicker`, closed and waited on reconfigure/shutdown. Use this for the stale-sweep goroutine.
- [ ] `internal/plugins/anomaly/detect/chain_integration_test.go` - the end-to-end injection pattern
  → Decision: for the Go integration proof, publish synthetic flows via `observation.Global().Publish(...)` (`chain_integration_test.go`) through a real `trafficfeature.NewService` into a real detector on a `chainTestBus` (`chain_integration_test.go`), then assert the observe store captured and finalized. Gate with `if testing.Short()` (real 1s ticks, ~10s).
  → Constraint: the `.ci` harness CANNOT synthesize per-source feature deviations without a traffic generator (stated in `anomaly-show.ci`), so the `.ci` proves wiring (`show anomaly observe` resolves, `enabled=true`, empty list); the lifecycle (open→finalize→EndTime) is proven by unit + Go integration tests.

**Key insights:**
- Copy the `ddos/observe` skeleton verbatim; change only (a) the incident struct key (source prefix), (b) the event namespace (`anomalyevent`), (c) wire the stale-sweep ticker the template left dead.
- Reachability: the show handler reads a process-global `atomic.Pointer[store]` because plugins run as in-process goroutines; no RPC round-trip to the plugin.
- One new wire method (`ze-show:anomaly-observe`) and one new plugin name (`anomaly-observe`) - both are snapshot-gated (`wire-methods.snapshot`, `plugins.snapshot`).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ddos/observe/store.go` - bounded incident ring keyed on dest `VectorTuple`; `open`/`finalize(target)`/`activeCount`/`count`/`list`(newest-first)/`sweepStale`/`evictOldest`(oldest finalized, else head).
  → Constraint: preserve the ring semantics; change only the key type and the event type. `sweepStale` is prod-dead in this template.
- [ ] `internal/plugins/ddos/observe/register.go` - `activeStore` pointer, `OnConfigure`/`OnConfigVerify`/`OnConfigApply`/`OnConfigRollback`, subscribe/unsubscribe closures, `ConfigureEventBus`.
- [ ] `internal/plugins/ddos/observe/show.go` - two wire methods (`ze-show:ddos-status`, `ze-show:ddos-incidents`); this child uses ONE (`ze-show:anomaly-observe`) to match the single-node `show anomaly detect`/`show anomaly shape` siblings.
- [ ] `internal/plugins/ddos/observe/cmd/yang/ze-ddos-cmd.yang` + `self_containment_test.go` - the `ze:command` node + the presence test proving the command lives in the plugin's own schema.
- [ ] `internal/core/anomalyevent/event.go` - the event structs subscribed to (fields cited in Required Reading).
- [ ] `internal/plugins/anomaly/detect/detector.go` - the Detected-only 128-ring this child supersedes with a lifecycle store (`detector.go,297-300,349-355`).
- [ ] `internal/plugins/anomaly/detect/cmd/yang/ze-anomaly-cmd.yang` - the shared `container show { container anomaly { container detect ... } }`; this child adds a sibling `container observe`.
- [ ] `internal/plugins/anomaly/shape/yang/ze-anomaly-shape-conf.yang` - the AUGMENT pattern (`augment "/ad:anomaly" { container shape {...} }`, `import ze-anomaly-detect-conf { prefix ad; }`); this child copies its shape for `container observe`.
- [ ] `internal/component/plugin/all/all.go` + `all_test.go` + `testdata/{plugins,wire-methods}.snapshot` - blank-import list + golden snapshots gating plugin names and wire methods.

**Behavior to preserve (do NOT regress):**
- The fact/judgment/response separation and the anomaly-vs-ddos domain split (separate event contract/namespace; no import of `ddos*` or of sibling anomaly plugins).
- `anomaly/detect`'s `show anomaly detect` (`ze-show:anomaly`) and its 128-entry Detected ring remain unchanged; this child ADDS a surface, it does not modify detect.
- `anomaly/shape`'s `show anomaly shape` (`ze-show:anomaly-shape`) unchanged.
- The `anomaly` config container is owned by `ze-anomaly-detect-conf`; siblings AUGMENT it. This child augments, never re-declares the parent.

**Behavior to change:** None existing. All new behavior is additive and opt-in via config presence.

## Data Flow (MANDATORY)

### Entry Point
- Typed events on the in-process `ze.EventBus`: `anomalyevent.Detected` / `Ongoing` / `Cleared`, emitted by `anomaly/detect` (`detector.go`). Operator query enters via `show anomaly observe` (CLI → `ze-show:anomaly-observe`). Operator tuning enters via config `anomaly { observe { incident-ring-size ...; stale-incident-timeout ... } }`.

### Transformation Path
1. Detector confirms an incident → `anomalyevent.Detected.Emit(bus, *AnomalyDetected)` (`detector.go`).
2. `anomaly-observe` subscribe closure `Detected.Subscribe(bus, s.open)` → `store.open(*AnomalyDetected)` appends a lifecycle `incident{Active:true, StartTime:=e.At}` keyed on `Entity`.
3. On resolve, detector emits `anomalyevent.Cleared` (`detector.go`) → `Cleared.Subscribe(bus, ...)` → `store.finalize(e.Entity)` sets `Active=false`, `EndTime=time.Now()` on the newest active incident for that entity.
4. Stale sweep ticker → `store.sweepStale()` finalizes open incidents older than `stale-incident-timeout` (covers a `Detected` with no matching `Cleared`, e.g. detector eviction).
5. `show anomaly observe` → `handleShowAnomalyObserve` reads `activeStore.Load()` → `store.list()` (newest-first) → `plugin.Map{enabled, active-count, incidents}`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| detect ↔ observe | `anomalyevent` typed events over `ze.EventBus` (Emit/Subscribe) | [ ] |
| observe ↔ CLI | `pluginserver.RegisterRPCs` wire method `ze-show:anomaly-observe` + `cmd/yang` `ze:command` node | [ ] |
| observe ↔ config | `ConfigRoots:["anomaly/observe"]`, section wrapped `{"anomaly":{"observe":{...}}}` | [ ] |
| observe ↔ show handler | process-global `atomic.Pointer[store]` (plugins are goroutines) | [ ] |

### Integration Points
- `anomalyevent.Detected/Ongoing/Cleared` - the judgment surface (subscribe only).
- `pluginserver.RegisterRPCs` - the RPC handler registry.
- `configyang.RegisterModule` - the YANG module registry (conf + cmd).
- `internal/component/plugin/all/all.go` - composition-root blank imports (regenerated).

### Architectural Verification
- [ ] No bypassed layers (subscribes to events; never re-measures facts, never reads the detector's internals)
- [ ] No unintended coupling (no import of `anomaly/detect`, `anomaly/shape`, or any `ddos*` package)
- [ ] No duplicated functionality (reuses the `ddos/observe` ring; supersedes the detect 128-ring with a lifecycle store rather than re-implementing scoring)
- [ ] Registration over hardcoding - new command, YANG, doctor(none), and plugin all register via existing registries; no new switch/field/factory in a core or shared package

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `ddos/observe` ring skeleton transfers with only key-type + event-type changes | `ddos/observe/store.go` read in full | more code than planned | port the store, run `TestObserveIncidentLifecycle` | unvalidated |
| A-2 | `AnomalyCleared` carries the source `Entity` prefix, so `finalize(entity)` can match the open incident | `anomalyevent/event.go` | cannot finalize; incidents leak active | unit test emitting Detected then Cleared for the same entity | unvalidated |
| A-3 | The section is delivered wrapped `{"anomaly":{"observe":{...}}}` (two-level unwrap) | `ddos/observe/config.go`, `detect/config.go` | config parses to defaults silently | `TestParseObserveConfig` with a wrapped payload | unvalidated |
| A-4 | A NEW `observe/yang` + `observe/cmd/yang` package is auto-discovered by `plugin_imports.go` and only needs a `make generate` + snapshot `-update` | `plugins.md` "How to carve", `all.go,113-114` | plugin/command unreachable; snapshot test fails | `make generate`; `TestRegisteredPluginNames`, `TestRegisteredWireMethods` after `-update` | unvalidated |
| A-5 | The `.ci` harness cannot inject feature deviations, so `.ci` proves wiring and unit/Go tests prove lifecycle | `anomaly-show.ci` states exactly this | over-scoped `.ci` flakes | write `.ci` for wiring-only; lifecycle in unit + `chain`-style Go test | unvalidated |
| A-6 | `anomaly-observe` needs no plugin `Dependencies` (events are pub/sub, order-independent) | `ddos/observe/register.go` declares none | load-order race, missed early events | daemon starts; `.ci` returns `enabled=true` | unvalidated |
| A-7 | No web card / durable store is in scope (none exists in the repo) | umbrella Known Limitations; web grep in umbrella A-2 | scope creep | grep confirms no ddos/anomaly web surface | confirmed (umbrella) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Open incidents that never receive a `Cleared` (detector evicts the entity after 10 idle ticks, `detector.go`) leak as permanently Active | `active-count` climbs and never falls under churn | wire the stale-sweep ticker (the template's dead `sweepStale`); `stale-incident-timeout` default 3600s; `TestObserveStaleSweep` |
| R-2 | Store memory grows unbounded | RSS climbs; `count()` == `cap` constantly | ring capped at `incident-ring-size` (default 1000, max 100000); `evictOldest` drops finalized-first; `TestObserveRingEviction` |
| R-3 | Duplicate `Detected` for an already-active entity opens a second incident (detector emits `activate` once per confirm, but re-confirm after clear is legitimate) | two active incidents for one entity | `open` always appends (each confirm is a distinct incident); `finalize` matches the NEWEST active for the entity (`ddos` `store.go` reverse scan) - a re-fire after clear is a new lifecycle, which is correct |
| R-4 | Event ordering: a `Cleared` arrives before its `Detected` (bus is synchronous in-process, `chain_integration_test.go`, so Detected-before-Cleared holds) - but a reconfigure that rebuilds the store mid-incident drops the open record, so a later `Cleared` no-ops | finalize finds no active incident (silent) | acceptable: reconfigure is operator-initiated and rare; `finalize` no-ops safely (reverse scan finds nothing); document in Known Limitations |
| R-5 | The new `ze-show:anomaly-observe` wire method or `anomaly-observe` plugin name is not added to the golden snapshots | `TestRegisteredPluginNames` / wire-method snapshot test fails in CI | run the `-update` step in Implementation Steps; both snapshots are in Files to Modify |
| R-6 | `show anomaly observe` collides with the shared `anomaly` container ownership (parent owned by detect cmd yang) | schema load error / duplicate container | container-MERGE (re-declare `container show { container anomaly { container observe ...}}` in a standalone module), NOT augment - same as `detect`/`shape` cmd yang; unique namespace/prefix |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show anomaly observe` CLI (dispatch-command) | → | `ze-show:anomaly-observe` → `handleShowAnomalyObserve` → `store.list()` | `test/plugin/anomaly-observe-show.ci` |
| `handleShowAnomalyObserve` with a populated store | → | reads process-global `activeStore` | `TestShowAnomalyObserveWithStore` (unit) |
| `handleShowAnomalyObserve` with nil store | → | `{enabled:false}` | `TestShowAnomalyObserveNoStore` (unit) |
| `anomalyevent.Detected` on the bus | → | subscribe closure → `store.open` | `TestObserveSubscribeOpensIncident` (unit, emits on a test bus) |
| `anomalyevent.Cleared` on the bus | → | subscribe closure → `store.finalize` | `TestObserveSubscribeFinalizesIncident` (unit) |
| stale-sweep ticker | → | `store.sweepStale` finalizes open incidents | `TestObserveStaleSweep` (unit) |
| plugin registered in composition root | → | `registry.Register` via `all.go` blank import | `TestRegisteredPluginNames` (snapshot incl. `anomaly-observe`) |
| wire method registered | → | `pluginserver.RegisterRPCs` | `TestRegisteredWireMethods` (snapshot incl. `ze-show:anomaly-observe`) |
| command schema owned by the plugin | → | `cmd/yang` `ze:command` node | `TestAnomalyObserveCmdSchemaOwnsShowObserve` (unit) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `store.open(*AnomalyDetected{Entity:P})` then `store.finalize(P)` | one incident, `Active` false after finalize, `EndTime` non-zero, `Entity==P`, `StartTime==e.At` |
| AC-2 | `open` N incidents into a ring of cap C (N>C), all finalized | `list()` len == C, oldest evicted, newest-first ordering preserved |
| AC-3 | open an incident, advance past `stale-incident-timeout`, `sweepStale()` | the open incident is finalized (`Active` false, `EndTime` set) with no `Cleared` event |
| AC-4 | `anomalyevent.Detected` emitted on a bus the store subscribes to | store opens a matching incident (event wiring, not a direct method call) |
| AC-5 | `anomalyevent.Cleared` emitted for the same entity | store finalizes that incident |
| AC-6 | `show anomaly observe` while plugin loaded, no traffic | status `done`, `enabled=true`, `incidents` an empty list, `active-count=0` |
| AC-7 | `show anomaly observe` with an active incident in the store | `enabled=true`, `incidents` contains the entity/cohort/score/severity/start-time and `active=true` |
| AC-8 | config `anomaly { observe { incident-ring-size 50 } }` | store built with cap 50; out-of-range (0 or >100000) rejected at `Validate` |
| AC-9 | `anomaly-observe` package directory + `all.go` imports removed | build stays green; `show anomaly observe` node, handler, YANG, config subtree all gone; `anomaly/detect` + `anomaly/shape` unaffected |
| AC-10 | end-to-end: synthetic cohort+outlier flows published, detector confirms, then outlier goes quiet | the observe store shows the incident with a `StartTime`, and (after clear/stale) an `EndTime` - the lifecycle the detect ring cannot show |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | enables `anomaly { detect { enabled true } observe {} }`, an incident fires, persists with a start time, then clears with an end time, all visible in `show anomaly observe` | facts → detector `activate` (Detected) → observe `open` → detector `emitCleared` (Cleared) → observe `finalize` → `show anomaly observe` | `TestChainObserveLifecycle` (Go integration) + `TestObserveIncidentLifecycle` (unit) + `anomaly-observe-show.ci` (wiring) |
| 2 | runs `show anomaly observe` on a fresh daemon (detect enabled, no traffic) | dispatch → `ze-show:anomaly-observe` → `handleShowAnomalyObserve` → empty `store.list()` | `anomaly-observe-show.ci` |
| 3 | leaves an incident open (source goes silent, no clear), waits past the stale timeout | stale-sweep ticker → `sweepStale` → finalize → `show anomaly observe` shows `active=false` + `end-time` | `TestObserveStaleSweep` (unit) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestObserveIncidentLifecycle` | `internal/plugins/anomaly/observe/store_test.go` | AC-1: open→finalize sets EndTime/Active, keyed on source prefix | |
| `TestObserveRingEviction` | `internal/plugins/anomaly/observe/store_test.go` | AC-2: cap enforced, oldest evicted, newest-first | |
| `TestObserveStaleSweep` | `internal/plugins/anomaly/observe/store_test.go` | AC-3: stale open incident finalized by timeout | |
| `TestObserveMultipleIncidents` | `internal/plugins/anomaly/observe/store_test.go` | multiple entities, partial finalize by entity | |
| `TestObserveSubscribeOpensIncident` | `internal/plugins/anomaly/observe/subscribe_test.go` | AC-4: `Detected` on a test bus → `store.open` | |
| `TestObserveSubscribeFinalizesIncident` | `internal/plugins/anomaly/observe/subscribe_test.go` | AC-5: `Cleared` on a test bus → `store.finalize` | |
| `TestParseObserveConfig` | `internal/plugins/anomaly/observe/config_test.go` | AC-8: two-level unwrap, defaults, range validation | |
| `TestShowAnomalyObserveNoStore` | `internal/plugins/anomaly/observe/show_test.go` | AC-6: nil store → enabled=false | |
| `TestShowAnomalyObserveWithStore` | `internal/plugins/anomaly/observe/show_test.go` | AC-7: populated store → entity/active in the map | |
| `TestAnomalyObserveCmdSchemaOwnsShowObserve` | `internal/plugins/anomaly/observe/cmd/yang/self_containment_test.go` | AC-9: `ze:command "ze-show:anomaly-observe"` + containers declared in the plugin's own schema | |
| `TestChainObserveLifecycle` | `internal/plugins/anomaly/observe/chain_test.go` | AC-10: real facts→detector→observe capture+finalize (short-gated) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| incident-ring-size | 1..100000 | 100000 | 0 | 100001 |
| stale-incident-timeout | 1..86400 | 86400 | 0 | 86401 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `anomaly-observe-show` | `test/plugin/anomaly-observe-show.ci` | operator runs `show anomaly observe`; wire method resolves, `enabled=true`, empty list with no traffic | |

### Interop Tests (MANDATORY for protocol features)
N/A - this child touches no wire protocol (no SAFI/capability/attribute; no BGP family). It subscribes to in-process events and adds a read-only CLI query. Justification per `ai/rules/interop-and-goal-validation.md`: no peer daemon behavior changes.

### Future (if deferring any tests)
- None. All ACs have a test above.

## Files to Modify
- `internal/component/plugin/all/all.go` - add 3 blank imports (`anomaly/observe`, `anomaly/observe/yang`, `anomaly/observe/cmd/yang`) via `make generate` / `go run scripts/codegen/plugin_imports.go` (generated file - never hand-edit).
- `internal/component/plugin/all/testdata/plugins.snapshot` - add `anomaly-observe` via `-update`.
- `internal/component/plugin/all/testdata/wire-methods.snapshot` - add `ze-show:anomaly-observe` via `-update`.
- `docs/guide/command-reference.md` - document `show anomaly observe`.
- `docs/plugin-overview.md` (and/or `docs/features/plugins.md`) - list the new `anomaly-observe` plugin in the registered-plugin inventory.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | Yes | `internal/plugins/anomaly/observe/yang/ze-anomaly-observe-conf.yang` - `augment "/ad:anomaly" { container observe {...} }`, `import ze-anomaly-detect-conf { prefix ad; }` |
| YANG validation constraints | Yes | `incident-ring-size` `uint32 { range "1..100000"; }`, `stale-incident-timeout` `uint32 { range "1..86400"; }` (native ranges, mirror `ze-ddos-observe-conf.yang`) |
| YANG custom validators | N/A | native ranges suffice; no dynamic completion needed |
| CLI commands/flags | Yes | `internal/plugins/anomaly/observe/cmd/yang/ze-anomaly-observe-cmd.yang` - `container show { container anomaly { container observe { ze:command "ze-show:anomaly-observe" } } }` |
| CLI grammar (action before identifier) | Yes | `show anomaly observe` (verb `show`, then namespace `anomaly`, then node `observe`) - matches `show anomaly detect`/`show anomaly shape` |
| Editor autocomplete | Yes | automatic from the `config false` YANG command tree (same as `show anomaly detect`) |
| Functional test for new RPC/API | Yes | `test/plugin/anomaly-observe-show.ci` |
| Pipe completeness | Yes | output routed through the standard dispatch-command response path, same as `show anomaly detect` (no custom pager); JSON map surfaces through `ApplyPipes` like the sibling show handlers |
| Env var registration | N/A | no `environment/` leaves added |
| Doctor check for runtime dependencies | N/A | in-memory store on the in-process bus; no file/socket/port/module/binary/cert/procfs/netlink dependency. `ddos/observe` mirror registers none; the "detect disabled" case is covered by `anomaly-detect-feature-source` |
| Prometheus counters/metrics | N/A | observable state (retained/active incident counts) is surfaced by `show anomaly observe`; the detector already exports `ze_anomaly_active`/`ze_anomaly_incidents_total`; `ddos/observe` defines none. A retained-count gauge is a noted future (Known Limitations) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The umbrella owns the `docs/features.md` "behavioral security anomaly detection" roll-up row (AC-8 of umbrella); this child adds a command, not a new top-level feature. Grep `docs/features.md` for `anomaly` to confirm the row already frames observe. |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` - `anomaly { observe { incident-ring-size; stale-incident-timeout } }` (if the guide already lists `anomaly { detect/shape }`) |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` - `show anomaly observe` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` - wire method `ze-show:anomaly-observe` (if the doc enumerates wire methods) |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` and/or `docs/plugin-overview.md` - new `anomaly-observe` plugin |
| 6 | Has a user guide page? | No | the umbrella owns `docs/guide/anomaly-detection.md`; add an `observe` paragraph there when the umbrella guide lands (AC-8), not a new page |
| 7 | Wire format changed? | No | no wire encoding touched |
| 8 | Plugin SDK/protocol changed? | No | uses existing SDK callbacks + `pluginserver.RegisterRPCs` |
| 9 | RFC behavior implemented? | No | no RFC |
| 10 | Test infrastructure changed? | No | uses existing `.ci` harness + `go test` |
| 11 | Affects daemon comparison? | No | no comparison-table feature |
| 12 | Internal architecture changed? | No | additive plugin; no core/subsystem doc change |
| 13 | Route metadata keys added/changed? | No | no route metadata |
| 14 | Prometheus counters added/changed? | No | none added (see Integration Checklist) |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` - new plugin + new `show anomaly observe` command |
| 16 | Any changed source file referenced by existing doc source anchors? | No | grep `docs/` for `source: internal/plugins/anomaly` - the new files did not exist before; confirm no stale anchor points at them |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | verify any `show anomaly` / `anomaly {}` example in `docs/guide/` still parses and add the `observe` node where the sibling nodes are listed |

## Files to Create
- `internal/plugins/anomaly/observe/config.go` - `Name="anomaly-observe"`, `configRoot="anomaly/observe"`, `Config{IncidentRingSize,StaleIncidentTimeout}`, `DefaultConfig`/`ParseConfig`/`Validate`, atomic logger.
- `internal/plugins/anomaly/observe/store.go` - the lifecycle ring keyed on source `netip.Prefix` (ported from `ddos/observe/store.go`).
- `internal/plugins/anomaly/observe/register.go` - `init()` `registry.Register` (Features "yang", YANG, ConfigRoots, RunEngine, ConfigureEngineLogger, ConfigureEventBus); `runEngine` subscribes + wires the stale-sweep ticker; `activeStore atomic.Pointer[store]`.
- `internal/plugins/anomaly/observe/show.go` - `init()` `pluginserver.RegisterRPCs(ze-show:anomaly-observe)` + `handleShowAnomalyObserve`.
- `internal/plugins/anomaly/observe/store_test.go` - lifecycle/eviction/stale/multi tests.
- `internal/plugins/anomaly/observe/subscribe_test.go` - event-wiring tests on a synchronous test bus.
- `internal/plugins/anomaly/observe/config_test.go` - parse/validate/boundary tests.
- `internal/plugins/anomaly/observe/show_test.go` - handler tests (nil + populated store).
- `internal/plugins/anomaly/observe/chain_test.go` - `TestChainObserveLifecycle` (short-gated Go integration mirroring `detect/chain_integration_test.go`).
- `internal/plugins/anomaly/observe/yang/embed.go` - generated `//go:embed` + `ZeAnomalyObserveConfYANG`.
- `internal/plugins/anomaly/observe/yang/register.go` - generated `configyang.RegisterModule("ze-anomaly-observe-conf.yang", ...)`.
- `internal/plugins/anomaly/observe/yang/ze-anomaly-observe-conf.yang` - the config augment.
- `internal/plugins/anomaly/observe/cmd/yang/embed.go` - generated `//go:embed` + `ZeAnomalyObserveCmdYANG`.
- `internal/plugins/anomaly/observe/cmd/yang/register.go` - generated `configyang.RegisterModule("ze-anomaly-observe-cmd.yang", ...)`.
- `internal/plugins/anomaly/observe/cmd/yang/ze-anomaly-observe-cmd.yang` - the `show anomaly observe` command node.
- `internal/plugins/anomaly/observe/cmd/yang/self_containment_test.go` - presence test for the command tokens.
- `test/plugin/anomaly-observe-show.ci` - functional wiring test.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella child 3 |
| 2. Audit | Files to Modify/Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7-10. Critical review loop | Critical Review Checklist |
| 11. Deliverables | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 14. Present summary | Executive Summary + learned summary |

### Implementation Phases

Each phase ends with a Self-Critical Review. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - register the plugin, the config root, the YANG conf + cmd modules, and the show wire method as stubs; write failing wiring tests.
   - Tests: `TestRegisteredPluginNames`, `TestRegisteredWireMethods`, `TestAnomalyObserveCmdSchemaOwnsShowObserve`, `anomaly-observe-show.ci` (fails until handler returns a store)
   - Files: `register.go` (stub store), `show.go`, `yang/*`, `cmd/yang/*`; then `make generate`; `-update` both snapshots
   - Verify: plugin name + wire method appear in snapshots; `.ci` reaches the handler
2. **Phase: Store (lifecycle ring)** - port `ddos/observe/store.go`, re-key on source `netip.Prefix`, map `AnomalyDetected` fields.
   - Tests: `TestObserveIncidentLifecycle`, `TestObserveRingEviction`, `TestObserveMultipleIncidents`
   - Files: `store.go`, `store_test.go`
   - Verify: open/finalize/evict pass
3. **Phase: Stale sweep** - wire the stale-sweep ticker in `runEngine` (mirror detect's `startTicker`/`stopTicker` lifecycle); make `stale-incident-timeout` functional.
   - Tests: `TestObserveStaleSweep`
   - Files: `register.go`, `store.go`
   - Verify: open-but-never-cleared incident finalizes after the timeout
4. **Phase: Event subscription** - subscribe `Detected`→`open`, `Cleared`→`finalize`, `Ongoing`→no-op (or refresh last-seen), with unsubscribe on reconfigure/shutdown.
   - Tests: `TestObserveSubscribeOpensIncident`, `TestObserveSubscribeFinalizesIncident`
   - Files: `register.go`, `subscribe_test.go`
   - Verify: events on a synchronous test bus drive the store
5. **Phase: Config** - `Config`, `ParseConfig` (two-level unwrap), `Validate` (ranges).
   - Tests: `TestParseObserveConfig` + boundary rows
   - Files: `config.go`, `config_test.go`
6. **Phase: Show handler** - `handleShowAnomalyObserve` (nil + populated store).
   - Tests: `TestShowAnomalyObserveNoStore`, `TestShowAnomalyObserveWithStore`
   - Files: `show.go`, `show_test.go`
   - Verify: `.ci` now passes end-to-end (wiring)
7. **Phase: Chain integration** - `TestChainObserveLifecycle` (short-gated), proving real facts→detector→observe capture+finalize.
   - Files: `chain_test.go`
8. **Functional tests** → `anomaly-observe-show.ci` green on the DUT.
9. **Docs** → command-reference, plugin inventory (Documentation Update Checklist Yes rows).
10. **Full verification** → `make ze-precommit-verify` (or lint + unit + functional).
11. **Complete spec** → audit tables, learned summary `plan/learned/NNN-anomaly-3-observe.md`, two commits (A: code+tests+docs+spec+summary; B: `git rm` spec).

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has code + a passing test at file:line |
| Feature completeness | `show anomaly observe` returns the lifecycle list incl. finalized incidents with `EndTime` - the surface the detect ring lacks (the whole point of the child) |
| Correctness | `finalize` matches the NEWEST active incident for the entity (reverse scan); `StartTime` uses `AnomalyDetected.At`; stale-sweep actually runs (ticker wired, not dead like the ddos template) |
| Naming | JSON keys kebab-case (`start-time`, `end-time`, `active-count`, `fired-features`); YANG kebab-case; plugin name `anomaly-observe`; wire method `ze-show:anomaly-observe` |
| Data flow | store fed ONLY by subscribed events; no import of `anomaly/detect` or any `ddos*`/sibling plugin |
| CLI grammar | `show anomaly observe` - verb then namespace then node |
| Registration over hardcoding | plugin/command/schema all register; no new switch/field/factory in a core or shared package |
| Doctor checks | none (justified N/A); confirm no runtime dependency slipped in |
| YANG validation | both leaves have native `range`; no bare `type string` |
| Prometheus counters | none added (justified); confirm no observable-state gauge was half-wired |
| Rule: self-containment | removing the plugin dir + `all.go` imports removes the whole surface, build green (AC-9) |
| Rule: no plugin→plugin import | `import` list of `observe/*` contains no `anomaly/detect`, `anomaly/shape`, `ddos/*` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `anomaly-observe` registered | `grep anomaly-observe internal/component/plugin/all/testdata/plugins.snapshot` |
| `ze-show:anomaly-observe` registered | `grep ze-show:anomaly-observe internal/component/plugin/all/testdata/wire-methods.snapshot` |
| Store lifecycle works | `go test ./internal/plugins/anomaly/observe/ -run TestObserve` |
| Event wiring works | `go test ./internal/plugins/anomaly/observe/ -run TestObserveSubscribe` |
| Show handler works | `go test ./internal/plugins/anomaly/observe/ -run TestShowAnomalyObserve` |
| `.ci` passes | run `anomaly-observe-show.ci` through the functional runner |
| Command owned by plugin | `go test ./internal/plugins/anomaly/observe/cmd/yang/` |
| Removal test | temporarily drop the `all.go` imports + dir; `go build ./...` green, no `anomaly-observe` token survives (do NOT commit this) |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | config ranges enforced at `Validate`; event payloads are trusted in-process typed structs (no untrusted wire parse) |
| Resource exhaustion | ring capped by `incident-ring-size`; stale-sweep bounds open-incident lifetime; `maxTracked`-style unbounded growth impossible |
| Error leakage | show handler returns only incident metadata already exposed by `show anomaly detect`; no secrets |
| Concurrency | single `sync.Mutex` guards the ring; `activeStore` is an `atomic.Pointer`; ticker goroutine stopped before store swap on reconfigure |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Snapshot test fails | Run the `-update` step; both snapshots in Files to Modify |
| `.ci` fails to reach handler | Check wire method + `cmd/yang` node + `all.go` imports (`make generate`) |
| Lifecycle test fails behavior mismatch | Re-read `anomalyevent/event.go` fields; check `finalize` key is `Entity` |
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
- The umbrella's A-2 said `ddos/observe` has "no show handler" and `list()` is test-only (`store.go`); that is now STALE against the code - `ddos/observe/show.go` registers `ze-show:ddos-status`/`ze-show:ddos-incidents` and `list()` is live (`show.go`). The child is therefore EASIER: a fuller template exists. The genuine divergences remain: source-prefix key vs dest-tuple, `anomalyevent` vs `ddosevent`, single-node `show anomaly observe` vs the ddos two-node split, and the stale-sweep wiring the ddos template lacks.
- The stale-sweep ticker is the one place this child does MORE than its template rather than less: it makes `stale-incident-timeout` a real control, closing the open-incident leak the detector's 10-idle-tick eviction (`detector.go`) would otherwise cause (Detected with no Cleared).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Single `show anomaly observe` node | ddos-style `status` + `incidents` split | matches the single-node `show anomaly detect`/`show anomaly shape` siblings in the same `anomaly` namespace; simpler surface |
| No `enabled` leaf; store always-on when plugin loaded | gate the store behind an `enabled` flag | uniform with `ddos/observe`; an empty store (detect disabled) is harmless and negligible memory |
| Wire the stale-sweep ticker | leave `sweepStale` prod-dead like the template | otherwise open-never-cleared incidents leak Active forever after detector eviction (R-1); makes the config leaf functional |
| `StartTime = AnomalyDetected.At` | `time.Now()` at open (ddos template) | the detector's confirm timestamp is more accurate than the store's receive time |
| No Prometheus metrics | add retained/active gauges | detector already exports `ze_anomaly_active`/`ze_anomaly_incidents_total`; the query surface exposes counts; ddos mirror has none |
| No doctor check | add an "observe enabled but detect disabled" warning | not a runtime dependency; `anomaly-detect-feature-source` already flags a dead pipeline |

## Known Limitations
- In-memory only; incidents are lost on daemon restart (no durable store exists in the repo).
- No web card (none exists for ddos or anomaly; out of scope).
- No Prometheus retention gauge yet (a future `ze_anomaly_observe_*` gauge could expose retained/active counts).
- A mid-incident reconfigure rebuilds the store and drops in-flight open records (R-4); a later `Cleared` then no-ops. Acceptable for an operator-initiated, rare event.
- Query is unfiltered (returns the whole ring); filtering by active/time is left to the caller reading the `active`/`end-time` fields.

## RFC Documentation
N/A - no RFC/protocol behavior.

## Implementation Summary
### What Was Implemented
- [filled at implementation]
### Bugs Found/Fixed
- [filled at implementation]
### Documentation Updates
- [filled at implementation]
### Deviations from Plan
- [filled at implementation]

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
| incident lifecycle store (open→finalize with EndTime/Active) | functional/unit test | `TestObserveIncidentLifecycle`, `TestChainObserveLifecycle` |
| NEW `show anomaly observe` query surface | functional test | `anomaly-observe-show.ci` |
| augments `/ad:anomaly` with `container observe` | schema test | `TestAnomalyObserveCmdSchemaOwnsShowObserve` + config load |
| self-contained (removal test) | build check | AC-9 removal verification |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed / deferred / acknowledged |
### Fixes applied
- [per finding]
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
- [ ] AC-1..AC-10 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete - every row has a concrete test name
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-standard-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/anomaly/observe/`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log

### Quality Gates (SHOULD pass - defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N/A - justified, non-protocol feature)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING - before ANY commit)
- [ ] Critical Review passes - all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-anomaly-3-observe.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-anomaly-3-observe.md`
