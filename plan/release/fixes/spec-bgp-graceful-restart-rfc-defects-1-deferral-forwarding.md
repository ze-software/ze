# Spec: bgp-graceful-restart-rfc-defects-1-deferral-forwarding

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | RFC 4724 Section 4.1 requires a Restarting Speaker to retain BGP forwarding state across restart while route selection is deferred; competing IGP routes currently overwrite retained BGP routes in the kernel during deferral. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Split from `plan/release/fixes/spec-bgp-graceful-restart-rfc-defects.md` (D7,
recorded 2026-11-24 from journal row
`reconciler-claims-state-another-owner-installed`).

When `ze` restarts as an RFC 4724 Section 4.1 Restarting Speaker with
`flush-on-stop` false and `bgp update-delay max-delay` configured, `fib-kernel`
retains the previous run's protocol-250 kernel routes and waits for BGP's
`SelectionComplete` before sweeping them, and `bgp-rib` defers BGP route
selection per family (`selectionDeferral.isDeferred`) until End-of-RIB from all
non-excluded GR peers or `Selection_Deferral_Timer` expiry.

During that deferral window, `bgp-rib` publishes nothing to the daemon's unified
Loc-RIB (`internal/core/rib/locrib/`). Because `locrib.Default()` holds no BGP
candidate for a prefix P whose BGP route the previous run left in the kernel,
any competing IGP (OSPF, IS-IS) or static route for P wins selection in `locrib`
immediately even when BGP's administrative distance beats it. `sysrib` (internal
or forked) then publishes an Add for the IGP route to `fib-kernel`, where
`ownsSlotLocked` sees P's table in `staleTables` and calls `replaceOwnedRoute`,
installing the IGP route and deleting or replacing the retained BGP route before
BGP's selection runs. Traffic shifts off BGP's path for the entire deferral
window, violating RFC 4724 Section 4.1:

- "When the Restarting Speaker restarts, it MUST retain, if possible, the
  forwarding state for the BGP routes in the Loc-RIB and MUST mark them as
  stale. It MUST NOT differentiate between stale and other information during
  forwarding." (`RFC4724-4.1-1`, `RFC4724-4.1-3`)
- "It is noted that prior to route selection, the speaker has no routes to
  advertise to its peers and no routes to update the forwarding state."
- "After the BGP speaker performs route selection, the forwarding state of the
  speaker MUST be updated and any previously marked stale information MUST be
  removed." (`RFC4724-4.1-6`)

The fix mirrors FRR zebra's candidate model inside Ze's unified Loc-RIB
architecture without guessing protocol ownership from kernel metrics and without
violating process boundaries for either `bgp-rib` or `sysrib`:

1. On clean daemon stop when forwarding state is preserved
   (`registry.ForwardingStatePreserved` evaluated against the live running
   config tree) and the stop is NOT an OS reboot (`request reboot` clears the
   kernel FIB, so reboot explicitly skips snapshotting and deletes any existing
   snapshot), snapshot BGP's selected Loc-RIB paths from `locrib.Default()` into
   the daemon's managed `statestore` under a new registered ZeFS key
   (`zefs.KeyBGPRetainedRoutes`, `"meta/bgp/retained-routes"`). This runs
   independently of whether GR `maxRestart > 0`, once per stop (before
   `StopForRestart` on `request restart`, during `OnDrain` on `SIGTERM`/`request
   shutdown`, with a `runBGPEngine` defer fallback if drain is skipped). When
   forwarding state is not preserved (`flush-on-stop` true), delete any existing
   snapshot.
2. On daemon startup, synchronously BEFORE Tier-0 (`bgp-rib`, `policy`,
   `interface`, `sysctl`) and Tier-1 (`bgp`, `rib`/`sysrib`, `ospf`, `isis`)
   plugins spawn (via a generic `Registration.BeforeStart` pre-tier hook in
   `plugin/registry` and `plugin/server`), read and unconditionally remove
   `zefs.KeyBGPRetainedRoutes` once (matching `grmarker.Read` + `Remove`). When
   `bgp update-delay max-delay > 0` and forwarding state is preserved, seed each
   saved BGP path into `locrib.Default()` via `locrib.InsertStale`
   (`Path.Stale = true`), competing at BGP's real administrative distance
   (`ebgp` or `ibgp`, obeying `rib { distance }`) and `Metric`, and immediately
   register the EventBus deferral-sweep handlers before `bgp-rib` can arm or
   complete deferral.
3. While a prefix's selected `Best` in `locrib` carries `Stale == true`:
   - Any competing IGP route with a worse distance loses in `locrib.selectBest`,
     so `sysrib` never programs it and the kernel's retained BGP route remains
     untouched.
   - Any competing IGP route with a better distance (or a prefix with no
     retained BGP candidate) wins in `locrib.selectBest` with `Stale == false`
     and is programmed into the kernel immediately.
   - `sysrib` (both internal `local-authority` and forked `selected-feed` via
     `rpc.RouteViewPath.Stale`) records an unprogrammed `Stale == true` winner
     in `s.best` without emitting an initial FIB install, without tracking its
     next-hops in `trackNextHopGroup` / `applyFIBImport`, and without
     programming it in `cascadeRecompute` or `fibStateChange` (mirroring
     `osInstalled`, because the kernel already holds that forwarding state and
     RFC 4724 Section 4.1 forbids updating forwarding state prior to selection),
     leaving `fib-kernel`'s `retained`/`staleTables` un-refreshed until BGP's
     real selection arrives. When fresh BGP (`Stale == false`) replaces an
     unprogrammed stale winner (`prev.retainedStale != winner.retainedStale`),
     `recomputeForwarding` treats it as non-no-op even when
     `verdict == fibUnproved` so an Add refresh always reaches `fib-kernel`.
   - Redistribution and BGP origination skip `Stale == true` paths so nothing is
     advertised to peers before selection.
4. When BGP finishes deferred selection for any family (`evaluateReleasesLocked`
   and `releaseAll` return every newly released family, dirty or zero-route),
   `bgp-rib` runs `reselectDeferredFamily(fam)` for dirty families (publishing
   fresh `Stale == false` BGP paths that replace re-learned stale candidates and
   trigger a real FIB refresh) and unconditionally emits
   `ribevents.SelectionFamilyComplete{Family: fam}` on the EventBus for every
   released family regardless of whether it received any UPDATEs; when all
   deferred families finish or `Selection_Deferral_Timer` expires, `bgp-rib`
   emits `ribevents.SelectionComplete` as today. The daemon's EventBus
   subscriber (attached pre-tier before `bgp-rib` starts, bounded by a fallback
   timer of `maxDelay` + margin, never reading
   `ribevents.ActiveSelectionDeferral`) sweeps remaining `Stale == true` BGP
   candidates per family on `SelectionFamilyComplete` and across all families on
   `SelectionComplete`, promoting any waiting IGP route or letting `fib-kernel`
   sweep unrefreshed kernel leftovers.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - process boundary, `statestore` + `pkg/zefs/keys.go`, public `pkg/` contracts
  → Constraint: persistent runtime state uses `statestore` under a registered `pkg/zefs/keys.go` key, never raw files; `bgp-rib` and `sysrib` both work internal or forked, so deferral completion arrives over the EventBus and stale flag crosses `route-view-*` RPC
- [ ] `ai/doors/core.md` - `internal/core/rib/locrib/`, `internal/core/bgp/ribevents/`, `pkg/zefs/keys.go`
  → Constraint: `internal/core/` never imports `internal/component/` or `internal/plugins/`; `locrib` stays protocol-agnostic (`Path.Stale` + `InsertStale` + `SweepStale(source)` / `SweepStaleFamily(source, fam)` carry no BGP import)
- [ ] `ai/doors/forwarding.md` - `sysrib`, `fib-kernel`, kernel route ownership, QEMU proof requirement
  → Constraint: observed in the Linux kernel route table inside QEMU, never only in a backend mock unit test; protocols never write the kernel directly
- [ ] `ai/doors/bgp-engine.md` and `ai/doors/bgp-plugins.md` - `bgp/config`, `bgp/plugin`, `bgp/grmarker`, `bgp/plugins/rib`
  → Constraint: `bgp` and `bgp-rib` compile out with `ze_bgp`; no always-on package imports them
- [ ] `ai/doors/plugin-infra.md` - `pkg/plugin/rpc/route_view.go`, `internal/component/plugin/registry/registry.go`, `internal/component/plugin/server/startup.go`
  → Constraint: `RouteViewPath` JSON tags are a wire contract (adding optional `stale` with `omitempty` preserves compatibility); `plugin/registry` and `plugin/server` never import `bgp`; `Registration.BeforeStart` is a generic pre-tier hook invoked for selected registrations before Tier 0 spawns
- [ ] `ai/doors/hub-engine.md` - daemon startup and shutdown wiring (`cmd/ze/hub/infra_setup.go`, `internal/component/config/infra/bgp.go`)
  → Constraint: `cmd/ze/hub` carries no BGP knowledge; BGP snapshot save/restore hooks live in `bgp/plugin/register.go` and `bgp/config/register.go`, reached across the existing `infra.WriteGRMarker` seam and generic `Registration.BeforeStart` / `OnDrain`
- [ ] `ai/doors/config-yang.md` - `internal/component/config/infra/bgp.go`
  → Constraint: `config/infra` carries zero BGP imports; `GRMarkerWriter` takes only always-on types (`[]plugin.InjectedCapability`, `storage.Storage`, `map[string]any`, `bool`)
- [ ] `docs/architecture/hub-architecture.md` - Startup Sequence, Boot effects, stop drain, `request restart` vs `request reboot`
  → Constraint: `BootEffect` runs after `eng.Start`, so pre-tier seeding uses a new generic `Registration.BeforeStart` hook before Tier 0 spawns; `request reboot` clears the kernel FIB and skips retained-route snapshotting
- [ ] `docs/architecture/plugin/plugin-relationships.md` - Startup Tiers (`bgp-rib` Tier 0; `bgp`, `rib`, `ospf`, `isis` parallel Tier 1) and Bus Topic Map
  → Constraint: pre-tier `BeforeStart` eliminates both the Tier-1 parallel seeding race and the Tier-0 `bgp-rib` early-completion race without altering the plugin dependency tier graph
- [ ] `docs/architecture/rib/unified-locrib.md` - unified Loc-RIB authority, `Path`, `Reselect`, internal and forked `sysrib`
  → Decision: Thomas's 2026-11-12 ruling requires internal and external (`selected-feed`) `sysrib` to behave identically; cross-protocol selection belongs solely in daemon `locrib.RIB`
- [ ] `docs/architecture/zefs-format.md` - key registry and value bounds
  → Constraint: register `KeyBGPRetainedRoutes` as its own var block in `pkg/zefs/keys.go` with an explicit `ValueMax` bound
- [ ] `docs/architecture/api/process-protocol.md` - System routing views and BGP-RIB event types
  → Constraint: document `stale` on `RouteViewPath` and `selection-family-complete` on `bgp-rib` events (emitted for every released family, including zero-route family EORs)
- [ ] `docs/architecture/plugin/rib-storage-design.md` - RFC 4724 Section 4.1 per-family selection deferral
  → Constraint: `reselectDeferredFamily` publishes fresh best changes synchronously for dirty families before `SelectionFamilyComplete` / `SelectionComplete` fires; zero-route released families still emit `SelectionFamilyComplete`

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4724.md` and `rfc/full/rfc4724.txt` Section 4.1
  → Constraint: `RFC4724-4.1-1` (retain BGP Loc-RIB forwarding state and mark stale), `RFC4724-4.1-3` (do not differentiate stale during forwarding), `RFC4724-4.1-6` (update forwarding state and remove remaining stale routes after route selection)
  → Constraint: `TestRFC4724RestartingSpeakerForwardingState` (`internal/plugins/fib/kernel/rfc4724_restarting_fib_test.go`) is NOT edited; D7's proofs are new tests

**Key insights:**
- `sysrib` no longer performs cross-protocol arbitration; the daemon's `locrib.RIB` is the single selection authority for both internal and forked `sysrib`. Seeding retained BGP routes into `locrib.Default()` solves D7 at the true authority and works identically whether `bgp-rib` or `sysrib` runs internal or forked.
- Because `locrib` holds the stale BGP route as `Best` during deferral and `sysrib` emits no FIB install for a losing IGP route, `fib-kernel`'s `replaceOwnedRoute` is never invoked for the losing IGP route, so D7 composes cleanly with `plan/release/fixes/spec-fib-kernel-label-ownership.md` regardless of landing order.
- Never rely on `ribevents.ActiveSelectionDeferral()` (process-global atomic) for sweep coordination: a forked `bgp-rib` process cannot set the daemon's atomic, so sweep coordination uses the EventBus (`SelectionFamilyComplete`, `SelectionComplete`) bounded by a timer derived from the daemon's own `bgp update-delay max-delay` config.

## Current Behavior (MANDATORY)

**Source files read (verified at HEAD on 2026-11-26):**
- [ ] `internal/core/rib/locrib/candidate.go` - `Path` holds `Source`, `Instance`, `AdminDistance`, `DistanceOverride`, `HasDistanceOverride`, `Metric`, `NextHop`, `Interface`, `OnLink`, `Weight`, `Labels`, `SRv6SID`, `IsEBGP`, `IsBGP`, `ECMP`, `RouteType`, `Hidden`, etc.; has no `Stale` field
- [ ] `internal/core/rib/locrib/entry.go` - `PathGroup.upsert` and `selectBest` arbitrate by `AdminDistance` then `Metric`, skipping `Hidden` paths
- [ ] `internal/core/rib/locrib/manager.go` - `RIB.Insert` / `InsertForward` / `Remove` manage sharded `PathGroup`s and dispatch `Change` events; has no `InsertStale`, `SweepStale`, or `SweepStaleFamily`
- [ ] `internal/core/rib/locrib/distance.go`, `policy.go` - `Reselect` re-runs `admit(snap, ..., producerPath(&g.Paths[j]))` and `resolvedDistance`; `producerPath` copies `*p` and clears `Hidden`
- [ ] `internal/core/bgp/ribevents/ribevents.go` - declares `SelectionDeferred` and `SelectionComplete`; has no per-family `SelectionFamilyComplete` event
- [ ] `internal/component/bgp/plugins/rib/selection_deferral.go` - `selectionDeferral.isDeferred` suppresses per-family selection; `reselectDeferredFamily` publishes fresh best changes for `fam`; `evaluateReleasesLocked` (L248-283) only appends families in `dirtyFamilies` (skipping zero-route family EORs); `releaseAll` (L298-320) likewise iterates only `dirtyFamilies`; neither emits a per-family completion event
- [ ] `internal/component/bgp/grmarker/grmarker.go`, `internal/component/bgp/config/register.go`, `internal/component/config/infra/bgp.go`, `cmd/ze/hub/infra_setup.go`, `internal/component/bgp/plugin/register.go`, `internal/component/plugin/registry/registry.go`, `internal/component/plugin/server/startup.go` - `BootEffect` runs after `eng.Start` (`cmd/ze/hub/main.go:1388`), while `bgp-rib` starts in Tier 0 and `bgp`, `rib`/`sysrib`, `ospf`, `isis` start in parallel in Tier 1; `writeGRMarker` currently takes only `(caps, store)` (no running `tree`, no `reboot` flag), returns early when `maxRestart <= 0`, and is shared by `request restart` and `request reboot` (`infra_setup.go:272-285`); `bgp` declares no `OnDrain` handler and `runBGPEngine` does not persist or restore BGP's Loc-RIB routes
- [ ] `pkg/zefs/keys.go` - registers `KeyGRMarker` (`meta/bgp/gr-marker`) and bounded keys such as `KeyIRRDataset` (`ValueMax: IRRDatasetValueMax = 12_000_000`); has no key for retained BGP Loc-RIB routes
- [ ] `pkg/plugin/rpc/route_view.go` - `RouteViewPath` carries every selected-path forwarding field over `route-view-*` RPC; has no `Stale` field
- [ ] `internal/component/sysrib/route_view_path.go`, `projection_path.go`, `route_view_forwarding.go`, `sysrib.go`, `fibimport.go` - `routeViewCopyEntry` copies `locrib.Path` into `rpc.RouteViewPath`; `projectionPath` decodes it back into `locrib.Path`; `prepareRouteViewSelectedMetadata` builds `protocolRoute`; `recomputeForwarding` (L353-507) emits an Add for any new winner (`prev == nil`), runs `defer s.trackNextHopGroup(key)` (L357), and treats `verdict == fibUnproved` with unchanged fields as a no-op (L468-481); `cascadeRecompute` (L732-795) and `fibimport.go` (`trackNextHopGroup`, `applyFIBImport`, `fibStateChange`, L235-440) guard `osInstalled` and `!fibPermitted` but have no guard for an unprogrammed retained-stale winner
- [ ] `internal/plugins/fib/kernel/fibkernel.go`, `backend_linux.go`, `rfc4724_restarting_fib_test.go` - `startupSweep` records leftover protocol-250 routes in `retained`/`staleTables`; `waitAndSweepStale` sweeps unrefreshed routes on `SelectionComplete`; `replaceOwnedRoute` deletes other protocol-250 priorities when `sysrib` sends an Add over a `staleTables` prefix

**Behavior to preserve:**
- A cold start (`KeyBGPRetainedRoutes` absent, `flush-on-stop` true, or `bgp update-delay max-delay == 0`) installs IGP and BGP routes immediately, identically to today.
- During deferral, a prefix Q with no retained BGP candidate takes an IGP/static route immediately.
- During deferral, a prefix R whose retained BGP candidate loses on administrative distance (for example, static distance 1 or `rib { distance }` ranking OSPF below eBGP) takes the winning IGP route immediately.
- `fib-kernel`'s `startupSweep`, `waitAndSweepStale`, `staleTables`, `retained`, and `TestRFC4724RestartingSpeakerForwardingState` are untouched.
- Internal (`local-authority`) and forked (`selected-feed`) `sysrib`, and internal and forked `bgp-rib`, exhibit identical selection and forwarding behavior.
- Composition with `plan/release/fixes/spec-fib-kernel-label-ownership.md` is preserved in either landing order.

**Behavior to change:**
- On restart with `flush-on-stop` false and `bgp update-delay max-delay > 0`, retained BGP routes from the previous run's Loc-RIB compete in `locrib.Default()` as `Stale: true` candidates at BGP's real administrative distance until BGP's deferred selection replaces or sweeps them.
- `sysrib` emits no FIB install for an initial `Stale: true` winner (keeping the kernel's retained forwarding state untouched and un-refreshed) and sweeps/promotes waiting IGP routes as soon as `locrib` sweeps unrefreshed stale BGP candidates after selection completes.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Unified Loc-RIB selection authority | `internal/core/rib/locrib/manager.go` `RIB.Insert`, `InsertForward`, `Remove`; `entry.go` `PathGroup`, `selectBest`; `distance.go` `resolvedDistance`, `Reselect`; `docs/architecture/rib/unified-locrib.md` | Arbitrates cross-protocol routes by `AdminDistance` (honoring `rib { distance }` and `IsEBGP`) then `Metric`, serving both internal and forked `sysrib` | Extends: adds `Path.Stale`, `InsertStale`, `SweepStaleFamily`, and `SweepStale` so retained BGP routes compete at BGP's real distance during deferral without duplicating selection logic in `sysrib` |
| Selected routing views (internal + forked `sysrib`) | `pkg/plugin/rpc/route_view.go` `RouteViewPath`; `internal/component/sysrib/route_view_path.go` `routeViewCopyEntry`; `projection_path.go` `projectionPath`; `route_view_forwarding.go` `prepareRouteViewSelectedMetadata`; `sysrib.go` `recomputeForwarding`, `cascadeRecompute`; `fibimport.go` `trackNextHopGroup`, `applyFIBImport`, `fibStateChange` | Streams authoritative selected paths from daemon `locrib` to internal or forked `sysrib` and emits `(system-rib, best-change)` batches to `fib-kernel`, guarding `osInstalled` and `!fibPermitted` across live updates, cascades, and FIB-import sweeps | Extends: carries `Stale` (`json:"stale,omitempty"`) across `RouteViewPath` into `protocolRoute.retainedStale`; suppresses initial FIB writes and next-hop tracking for an unprogrammed retained-stale winner across `recomputeForwarding`, `cascadeRecompute`, `trackNextHopGroup`, `applyFIBImport`, and `fibStateChange` (mirroring `osInstalled`); treats `prev.retainedStale != winner.retainedStale` as non-no-op in `recomputeForwarding` even when `verdict == fibUnproved` |
| RFC 4724 Section 4.1 selection deferral & bus events | `internal/component/bgp/plugins/rib/selection_deferral.go` `selectionDeferral`, `evaluateReleasesLocked`, `releaseAll`, `reselectDeferredFamily`, `emitSelectionDeferred`, `emitSelectionComplete`; `internal/core/bgp/ribevents/ribevents.go` `SelectionDeferred`, `SelectionComplete` | Holds BGP selection per family until EOR or `Selection_Deferral_Timer`, publishes fresh best changes per dirty released family, and announces deferral start/end on the EventBus | Extends: `evaluateReleasesLocked` and `releaseAll` return every newly released family (dirty or zero-route), run `reselectDeferredFamily(fam)` for dirty families, and emit `ribevents.SelectionFamilyComplete{Family: fam}` for every released family regardless of whether it received any UPDATEs |
| GR restart marker & `ForwardingStatePreserved` | `internal/component/bgp/grmarker/grmarker.go` `Write`, `Read`, `Remove`; `internal/component/config/infra/bgp.go` `GRMarkerWriter`, `WriteGRMarker`; `cmd/ze/hub/infra_setup.go` `stopForRestart`; `internal/component/bgp/config/register.go` `writeGRMarker`; `internal/component/plugin/registry/registry.go` `ForwardingStatePreserved`; `pkg/zefs/keys.go` `KeyGRMarker` | Persists the restart marker under `zefs` on restart/reboot and detects whether the forwarding plane preserves routes across restart (`flush-on-stop` false) | Extends: adds `zefs.KeyBGPRetainedRoutes` and once-per-stop snapshot helpers in `internal/component/bgp/grmarker/retained_routes.go`; passes live `GetConfigTree()` and `reboot bool` through `infra.WriteGRMarker` so `writeGRMarker` snapshots/clears independently of `maxRestart <= 0` before `StopForRestart` and explicitly skips snapshotting (removing any existing key) on `request reboot`; registers `OnDrain` (`DrainBudget: 5`) + `runBGPEngine` defer fallback in `bgp/plugin/register.go` for `SIGTERM`/`request shutdown` |
| Pre-tier daemon startup & stop drain | `internal/component/plugin/registry/registry.go` `Registration`; `internal/component/plugin/server/startup.go` `runPluginStartup`; `internal/component/plugin/server/stop_hooks.go` `OnStop`; `docs/architecture/hub-architecture.md`; `docs/architecture/plugin/plugin-relationships.md` | Starts plugins in dependency tiers (Tier 0 `bgp-rib`, `policy`, `interface`, `sysctl`; Tier 1 parallel `bgp`, `rib`, `ospf`, `isis`; Tier 2 `static`, `fib-kernel`), runs `BootEffect` after `eng.Start`, and runs parallel `OnDrain` before plugin teardown | Extends: adds optional `Registration.BeforeStart(tree, store, eb) func()` invoked synchronously by `runPluginStartup` for selected registrations BEFORE Tier 0 spawns (registering returned cleanup via `Server.OnStop`), eliminating both the Tier-1 parallel seeding race and the Tier-0 `bgp-rib` early-completion race without BGP knowledge in `plugin/server` or `hub` |
| Startup retention & deferred sweep in `fib-kernel` | `internal/plugins/fib/kernel/fibkernel.go` `startupSweep`, `retained`, `staleTables`, `waitAndSweepStale`; `backend_linux.go` `replaceOwnedRoute` | Keeps previous run's protocol-250 routes untouched at startup and sweeps unrefreshed ones once `SelectionComplete` arrives | Reuses untouched: because `locrib` holds the retained BGP candidate as best and `sysrib` emits no FIB write for losing IGP routes or untouched stale winners, `fib-kernel` keeps the kernel route untouched during deferral and sweeps unrefreshed ones at `SelectionComplete` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "Every plugin, ours included, runs either internal (a goroutine in the daemon) or external (a forked subprocess that talks only over TLS and RPC). The SDK hides which. Code that only works internal is broken." | agrees | Deferral release reaches the daemon over the EventBus (`SelectionFamilyComplete` / `SelectionComplete`), never via `ribevents.ActiveSelectionDeferral`; `RouteViewPath.Stale` crosses `route-view-*` RPC so both internal and forked `bgp-rib` and `sysrib` work identically |
| `docs/architecture/zefs-format.md` | Key Registry | `// Design:` of `pkg/zefs/keys.go`; modified by this spec | "All known key patterns are registered in `pkg/zefs/keys.go` via `MustRegister()`, following the same pattern as `env.MustRegister()` for environment variables. Each `KeyEntry` has a Pattern, Description, Private flag, and ValueMax." | agrees | `KeyBGPRetainedRoutes` (`meta/bgp/retained-routes`) registers via `MustRegister()` in `pkg/zefs/keys.go` with an explicit `BGPRetainedRoutesValueMax` bound, and this spec updates `zefs-format.md` |
| `docs/architecture/container/image-store.md` | State keys | anchored to `pkg/zefs/keys.go` | - | silent | This page specifies OCI container image storage and its `meta/container/*` ZeFS keys; this spec registers only `meta/bgp/retained-routes` and touches no container storage |
| `docs/architecture/diagnostics/debug-filtering.md` | One ZeFS trap | anchored to `pkg/zefs/keys.go` | - | silent | This page specifies diagnostic debug-filter profiles stored in ZeFS; this spec touches neither debug filtering nor diagnostic profile keys |
| `docs/architecture/fleet-config.md` | Architecture | anchored to `pkg/zefs/keys.go` | - | silent | This page specifies post-release fleet hub/client configuration management and its ZeFS keys; this spec touches only local BGP restarting-speaker route retention |
| `docs/architecture/irr-owner.md` | Stored keys and notices | anchored to `pkg/zefs/keys.go` | - | silent | This page specifies IRR dataset fetching and storage under `meta/irr-data/*`; this spec borrows the 12 MiB `ValueMax` precedent only and does not touch IRR keys or workers |
| `docs/architecture/storage-backends.md` | Opening and ownership | anchored to `pkg/zefs/keys.go` | "`Open(configDir)` opens the live tree and holds `configDir/database.lock` until `Close`." | agrees | `WriteRetainedRoutes`, `ReadRetainedRoutes`, and `RemoveRetainedRoutes` operate solely through the daemon's already-open `statestore` / ZeFS store handle without opening a second handle |
| `ai/doors/core.md` | Where it sits | door covering `pkg/zefs/keys.go`, `internal/core/rib/locrib/*` | "It imports neither `internal/component/` nor `internal/plugins/`." | agrees | `locrib`'s `Path.Stale`, `InsertStale`, `SweepStaleFamily`, and `SweepStale` are protocol-agnostic (`Source uint8`) and add zero upward imports from `internal/core/` |
| `docs/architecture/api/process-protocol.md` | System routing views | `// Design:` of `pkg/plugin/rpc/route_view.go`, `ribevents.go`; anchored to `bgp/plugin/register.go`; modified by this spec | "`selected-feed` is the default. It transfers only the daemon's selected routes, with resolved distance, metric, tag/AIGP presence, target ownership, multipath, labels, Service SID and repair metadata." | agrees | `RouteViewPath` carries the additive optional `stale` field (`json:"stale,omitempty"`) across `selected-feed`, and `bgp-rib:selection-family-complete` is emitted for every released family (dirty or zero-route EOR) and documented in `process-protocol.md` |
| `docs/architecture/api/ipc_protocol.md` | Overview | anchored to `pkg/plugin/rpc/route_view.go` | - | silent | This page specifies IPC line/width framing and the answer protocol; this spec adds only an optional JSON boolean (`stale`) inside the existing `RouteViewPath` payload and leaves IPC framing unchanged |
| `docs/architecture/rib/unified-locrib.md` | Owner requirement: internal and external system RIB | `// Design:` of `locrib/*`, `sysrib/*`; anchored to `rpc/route_view.go`; modified by this spec | "Both execution modes must preserve distances, next-hop resolution, ECMP, candidate inspection, configuration changes, FIB output, and recovery." | agrees | Cross-protocol arbitration stays solely in daemon `locrib.RIB`; `RouteViewPath.Stale` keeps internal and forked `sysrib` identical across `recomputeForwarding` (including `fibUnproved` stale->fresh Add refresh), `cascadeRecompute`, `trackNextHopGroup`, `applyFIBImport`, and `fibStateChange`, and this spec updates `unified-locrib.md` |
| `ai/doors/plugin-infra.md` | Who depends on it from outside | door covering `pkg/plugin/rpc/route_view.go` | "A field or method name in `rpc` is wire, not Go: renaming a JSON tag breaks every plugin written in another language." | agrees | `RouteViewPath.Stale` is purely additive with `json:"stale,omitempty"`, renaming or removing zero existing JSON fields |
| `docs/architecture/isis/isis-9-spf-rib.md` | Decision: install is Loc-RIB insertion, not a redistribute event | anchored to `locrib/candidate.go`, `locrib/manager.go`, `sysrib/route_view_path.go` | - | silent | This page specifies IS-IS SPF computation and Loc-RIB insertion; IS-IS continues inserting ordinary `Stale: false` paths into `locrib` unchanged and competes naturally against retained BGP candidates by administrative distance |
| `docs/architecture/rib/forward-handle.md` | The decisions | `// Design:` of `internal/core/rib/locrib/manager.go` | "A non-BGP producer leaves `Forward == nil`; a BGP control-plane re-election with no source UPDATE also leaves it nil." | agrees | `InsertStale` inserts snapshot-restored candidates with no source UPDATE (`Forward == nil`), leaving `ForwardHandle` refcounting and `InsertForward` untouched |
| `docs/architecture/forked-route-install.md` | The wire carries a name, the engine resolves it | anchored to `locrib/manager.go`, `sysrib/route_view_path.go`, `sysrib/route_view_forwarding.go` | "The RPCs are `ze-plugin-engine:route-install` and `:route-remove`, both with a batch payload. The engine applies the batch to the real `locrib.Default()`." | agrees | Forked `bgp-rib` still writes fresh BGP paths into daemon `locrib.Default()` via `route-install` (replacing `Stale: true` candidates cleanly), and whole-daemon restart snapshots from `locrib.Default()` without changing mid-run subprocess `cleanupProcess` semantics |
| `ai/doors/bgp-engine.md` | Where it sits | door covering `ribevents`, `bgp/config`, `bgp/plugin`, `bgp/grmarker` | "Every package under `internal/component/bgp` is gated by `ze_bgp` through the generated `internal/component/plugin/all/all_ze_bgp.go`." | agrees | Snapshot persistence (`grmarker/retained_routes.go`), startup seeding (`bgp/config/loader.go`), and sweep subscriptions (`bgp/plugin/register.go`) stay inside `ze_bgp`-gated BGP packages; always-on `locrib` and `sysrib` never import BGP |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | anchored to `bgp/plugins/rib/register.go` | - | silent | This page specifies `ze doctor` check phases and severities; this spec adds no new doctor check or code |
| `docs/architecture/plugin/plugin-relationships.md` | 4. Dependency Graph | anchored to `bgp/plugins/rib/register.go`, `bgp/plugin/register.go`; modified by this spec | "Plugins load in dependency order. Each tier completes before the next starts." | agrees | `bgp-rib` (Tier 0) adds `bgp-rib:selection-family-complete` to `ribBusSends` and `bgp` (Tier 1) adds both deferral-completion events to `bgpBusReceives` and `OnDrain`; `bgp`'s pre-tier `BeforeStart` seeds retained routes and registers the EventBus sweep before Tier 0 spawns, eliminating both the Tier-1 parallel seeding race (`bgp` vs `ospf`/`isis`/`rib`) and the Tier-0 `bgp-rib` early-completion race without changing the tier graph; this spec updates `plugin-relationships.md` |
| `ai/doors/bgp-plugins.md` | Boundaries | door covering `bgp/plugins/rib/register.go`, `selection_deferral.go` | "Reach the engine through the bus, DirectBridge or a text command, never a plain call into `reactor` or another component." | agrees | `bgp-rib` communicates per-family and full deferral completion to the daemon exclusively over the EventBus (`SelectionFamilyComplete`, `SelectionComplete`), never via the process-local `ActiveSelectionDeferral` atomic |
| `docs/architecture/plugin/rib-storage-design.md` | Overview | `// Design:` of `bgp/plugins/rib/selection_deferral.go` | - | silent | This page documents BGP Adj-RIB-In attribute-pool storage and JSON reconciliation; this spec touches only `selection_deferral.go` EventBus completion emission and daemon-side `locrib` stale retention, leaving BGP Adj-RIB-In pool storage unchanged |
| `docs/architecture/config/syntax.md` | Overview | `// Design:` of `bgp/config/loader.go`, `bgp/config/register.go`; anchored to `bgp/plugin/register.go` | - | silent | This page specifies JUNOS-like config syntax and YANG-driven parsing; this spec introduces no config keywords or syntax changes, reusing existing `bgp update-delay max-delay` and `fib kernel flush-on-stop` |
| `docs/architecture/api/commands.md` | Overview | anchored to `bgp/config/loader.go` | - | silent | This page specifies the CLI/API command grammar and pipe catalog; this spec adds or modifies no CLI commands or pipe operators |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to `bgp/config/loader.go`, `bgp/plugin/register.go` | - | silent | This page specifies config verify/apply/rollback transactions; this spec adds no transaction phases and runs retained-route restore once at startup and snapshot once at daemon stop |
| `docs/architecture/config/yang-config-design.md` | 1. Key Principle | anchored to `bgp/config/loader.go`, `bgp/config/register.go` | - | silent | This page specifies YANG module authoring and schema loading; this spec adds no YANG modules or leaves |
| `docs/architecture/core-design.md` | 1. System Architecture | `// Design:` of `bgp/plugin/register.go`, `sysrib/sysrib.go`; anchored to `bgp/config/*`, `sysrib/route_view_forwarding.go` | "SPF reads the LSDB, resolves next-hops, inserts `locrib.Path` values into the shared Loc-RIB, and lets sysrib/fibkernel own kernel FIB programming." | agrees | Competing IGP routes continue inserting into the shared Loc-RIB, where retained `Stale: true` BGP paths compete at BGP's administrative distance while `sysrib` and `fib-kernel` remain the sole FIB programming path |
| `docs/architecture/hub-architecture.md` | Startup Sequence | anchored to `bgp/config/register.go`, `cmd/ze/hub/infra_setup.go`; modified by this spec | "After host tuning and before the smart manager starts, the hub runs the boot effect every registered plugin declared (`Registration.BootEffect`), once, in plugin name order, with the config text the daemon started from and the name it read that text from." | agrees | Because `BootEffect` runs after `eng.Start` (after plugin tiers have already started), this spec adds a generic `Registration.BeforeStart(tree, store, eb)` pre-tier hook invoked synchronously by `Server.runPluginStartup` before Tier 0 spawns (keeping `hub` and `plugin/server` free of BGP knowledge), extends `infra.WriteGRMarker` with the live running tree and `reboot` flag so `request reboot` skips snapshotting while `request restart`/`SIGTERM` snapshot through the daemon's single owned store, and updates `hub-architecture.md` |
| `docs/architecture/api/architecture.md` | RIB Ownership | anchored to `bgp/plugin/register.go` | "The engine is a minimal BGP speaker." | agrees | BGP route selection and deferral stay in `bgp-rib`; the daemon only snapshots BGP's already-selected `locrib` winners across restart and sweeps stale `locrib` entries on `bgp-rib`'s EventBus completion events |
| `docs/architecture/config/apply-ordering.md` | The design | anchored to `bgp/plugin/register.go` | - | silent | This page specifies five-phase IP address move ordering between interface providers and address binders; this spec does not alter interface/address commit ordering |
| `docs/architecture/plugin/plugin-system.md` | Cross-boundary value types | anchored to `bgp/plugin/register.go` | "A payload that crosses a plugin or component boundary is a self-contained value type." | agrees | `SelectionFamilyCompleteEvent` carries only value-type `family.Family` and `RouteViewPath.Stale` is a plain `bool`; no pointer crosses any plugin or component boundary |
| `ai/doors/forwarding.md` | Where it sits | door covering `internal/component/sysrib/*`, `internal/plugins/fib/kernel/*` | "Protocols insert paths into the shared Loc-RIB, which applies import policy and selects the system route. `sysrib` follows that selection through local reads or its authenticated routing view and publishes `(system-rib, best-change)`." | agrees | Retained BGP routes compete inside the shared Loc-RIB, `sysrib` suppresses initial FIB writes for untouched stale winners on both `local-authority` and `selected-feed`, and verification runs against the real Linux kernel inside QEMU |
| `docs/architecture/api/text-format.md` | Current Format | anchored to `internal/component/sysrib/sysrib.go` | - | silent | This page specifies human-readable text formatting for BGP updates and API commands; this spec does not alter text formatting |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Decision | anchored to `internal/component/sysrib/sysrib.go` | - | silent | This page records the pull-model Prometheus metrics decision; this spec adds no metrics or collection changes |
| `docs/architecture/wire/nlri-evpn.md` | Common Header | anchored to `internal/component/sysrib/sysrib.go` | - | silent | This page specifies BGP EVPN NLRI wire encoding; this spec makes no BGP wire or EVPN NLRI changes |
| `docs/architecture/api/update-syntax.md` | Overview | anchored to `internal/component/plugin/registry/registry.go` | - | silent | This page specifies BGP UPDATE command syntax for API and config; this spec adds only a generic `Registration.BeforeStart` hook and touches no UPDATE syntax |
| `docs/architecture/bgp/structural-forwarding.md` | Four costs, and what replaced them | anchored to `internal/component/plugin/registry/registry.go` | - | silent | This page specifies reactor-native route-server forwarding; this spec does not touch route-server forwarding or `workItem` dispatch |
| `docs/architecture/dns/geodns.md` | Config model | anchored to `internal/component/plugin/registry/registry.go` | - | silent | This page specifies the GeoDNS plugin and its config model; this spec does not touch GeoDNS |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | anchored to `internal/component/plugin/registry/registry.go`, `internal/component/plugin/server/startup.go` | - | silent | This page specifies the IKEv2 state machine and SA lifecycle; this spec does not touch IKEv2 or IPsec |
| `docs/architecture/wire/nlri.md` | TL;DR (Read This First) | anchored to `internal/component/plugin/registry/registry.go` | - | silent | This page specifies packed-bytes NLRI wire formats; this spec makes no BGP wire or NLRI codec changes |
| `docs/architecture/cli/command-completion.md` | The producers | anchored to `internal/component/plugin/server/startup.go` | - | silent | This page specifies interactive CLI completion producers; this spec adds or modifies no CLI commands or completion producers |
| `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` | Rule 1: the backend loads on demand | anchored to `internal/component/plugin/server/startup.go` | - | silent | This page specifies firewall nft table ownership and shutdown flush; this spec does not touch firewall tables or backends |
| `docs/architecture/l2tp/cpe-1-pppoe-client.md` | Decisions | anchored to `internal/component/plugin/server/startup.go`, `cmd/ze/hub/infra_setup.go` | - | silent | This page specifies the PPPoE client interface kind; this spec does not touch L2TP or PPPoE |
| `docs/architecture/aaa-tacacs.md` | A registry, not a hardcoded chain | anchored to `cmd/ze/hub/infra_setup.go` | - | silent | This page specifies the pluggable AAA chain and TACACS+ backend; this spec touches only `stopForRestart` in `infra_setup.go` and leaves AAA/SSH wiring untouched |
| `ai/rules/plugins.md` | Directives | `// Design:` of `internal/component/config/infra/bgp.go` | "A plugin MUST own its entire feature surface. Removing the plugin MUST make every one of its features disappear; every other plugin and the core MUST keep working." | agrees | `infra.WriteGRMarker` remains a nil-safe inversion-of-control seam using only always-on types (`map[string]any`, `bool`), and `Registration.BeforeStart` is a generic hook with zero BGP imports so compiling out `ze_bgp` leaves core and other plugins working cleanly |
| `ai/doors/config-yang.md` | Where it sits | door covering `internal/component/config/infra/bgp.go` | "It holds zero BGP imports: the dependency runs from the generic loader into `bgp/config` (`plan/learned/DESIGN-HISTORY.md`, Configuration, load-bearing invariants)." | agrees | Extending `infra.GRMarkerWriter` with `tree map[string]any` and `reboot bool` adds zero BGP imports to `internal/component/config/infra` |
| `ai/doors/hub-engine.md` | Boundaries | door covering `cmd/ze/hub/infra_setup.go` | "The hub and the Engine carry no BGP knowledge." | agrees | `cmd/ze/hub/infra_setup.go` passes the live config tree and `reboot` flag through the existing `infra.WriteGRMarker` seam without naming any BGP type or adding BGP logic to `cmd/ze/hub` |
| `docs/architecture/fleet-config.md` | Design decisions | decision of 2026-11-23 | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | This spec does not touch managed hub listeners, TLS certificates, or `pki` configuration |
| `docs/architecture/fleet-config.md` | Design decisions | decision of 2026-11-25 | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | This spec touches no fleet provisioning or web Config tab behavior |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | `RouteViewPath.Stale` extends the existing authorized `ze-sysrib:route-view-read` operation, and `bgp-rib:selection-family-complete` is declared in `ribBusSends` and `bgpBusReceives` so compiled participant filters authorize it |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Lossless event delivery ensures `SelectionFamilyComplete` and `SelectionComplete` reach subscribers in order without drops; this spec adds a bounded fallback timer only for plugin crash/absence |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | This spec adds no command path or selector words |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | This spec adds or modifies no pipe overrides |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | This spec adds or modifies no pipe operations |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | `locrib` stale retention and `sysrib` suppression are data-plane agnostic and work beside any `ProgramsFIB` plugin (`fib-kernel`, `fib-vpp`, `fib-dummy`) without changing backend gates |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This spec introduces no data-plane backend mismatch or validation path divergence |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This spec touches no firewall or VPP configuration or verifier |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This spec introduces no data-plane selector or backend naming |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | This spec touches no BGP export filter chain or egress next-hop rewriting |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This spec does not touch `policychain.Sets` or export chain next-hop authoring |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This spec does not touch `policychain.Chain` or built-in `next-hop:*` steps |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | This spec does not touch export chains or raw-override next-hop provenance |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This spec does not touch policy next-hop validation or config loading of `policy { modify }` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | This spec does not touch BGP export next-hop actions |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This spec adds no filter-chain insertion and leaves `filterapi.ImpliedFilter` untouched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | A corrupt, expired, wrong-version, or oversized `KeyBGPRetainedRoutes` entry logs a warning, deletes the key, and degrades to normal startup without ever preventing `ze start` (AC-5) |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | This spec introduces no kernel/VPP mismatch and no startup refusal |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | This spec does not modify config rendering or `| implied` inline comments |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | `bgp-rib` declares `bgp-rib:selection-family-complete` in `ribBusSends` and `bgp` declares both `bgp-rib:selection-family-complete` and `bgp-rib:selection-complete` in `bgpBusReceives` so default registration wires deferral sweeps with zero extra operator config |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | No participant `send > to` replacement semantics are altered; the daemon's fallback timer prevents permanent stale routes even if a custom bus filter blocks `selection-complete` |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | No participant `receive > from` enforcement is altered by this spec |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | `bgp-rib:selection-family-complete` is declared in `ribBusSends` and `bgpBusReceives`, so bus allow-lists enforce it uniformly with every other registered message |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | This spec touches no bus-filter compilation or commit validation |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Cross-protocol route arbitration stays in `locrib.RIB`, never on the bus |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | `bgp-rib:selection-family-complete` appears automatically in the bus effective-filter display through `ribBusSends` and `bgpBusReceives` |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | Offline config parsing and query-mode external plugin validation are untouched; retained-route reading runs only during daemon startup (`loader.go` live boot path) |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- **Daemon stop (`request restart` / `request reboot`):** `cmd/ze/hub/infra_setup.go::stopForRestart` -> `infra.WriteGRMarker(caps, store, runningTree, reboot)` -> `bgp/config/register.go::writeGRMarker` (runs BEFORE `r.StopForRestart()` drops BGP sessions).
- **Daemon stop (`SIGTERM` / `request shutdown` / `quit`):** `bgp/plugin/register.go::runBGPEngine` `OnDrain` handler (`DrainBudget: 5`, runs BEFORE any plugin stops) and `runBGPEngine` shutdown `defer` fallback (if drain is skipped, runs in Tier 1 before `bgpReactor.Stop()` and before Tier-0 `bgp-rib` stops).
- **Daemon start (pre-tier, before Tier 0 spawns):** `internal/component/plugin/server/startup.go::runPluginStartup` invokes `Registration.BeforeStart(tree, store, eb)` for selected plugins -> `bgp/plugin/register.go::beforeBGPStart` seeds `locrib.Default()` and subscribes on `eb` before `bgp-rib` (Tier 0) or `ospf`/`isis`/`rib` (Tier 1) start.
- **During deferral:** IGP/static routes entering `locrib.Default()`, and `sysrib` reading `locrib.Default()` via `local-authority` or `selected-feed` across `recomputeForwarding`, `cascadeRecompute`, and `fibimport.go` (`trackNextHopGroup`, `applyFIBImport`, `fibStateChange`).
- **Deferral release:** `bgp-rib`'s `evaluateReleasesLocked()` / `releaseAll()` returning every newly released family (dirty or zero-route), running `reselectDeferredFamily(fam)` for dirty families, and emitting `ribevents.SelectionFamilyComplete{Family: fam}` for every released family followed by `ribevents.SelectionComplete`.

### Transformation Path
1. **Snapshot on stop (`grmarker/retained_routes.go::SaveOrClearRetainedRoutesOnce`, `WriteRetainedRoutes`, `RemoveRetainedRoutes`):**
   - Three coordinated entry points share one per-stop latch (`grmarker.SaveOrClearRetainedRoutesOnce`, reset when a new daemon/reactor starts):
     - **`request restart` / `request reboot` (`cmd/ze/hub/infra_setup.go` -> `infra.WriteGRMarker(caps, store, runningTree, reboot)` -> `bgp/config/register.go::writeGRMarker`):** passes the live running config tree (`apiSrv.Reactor().GetConfigTree()`, reflecting any committed reload of `flush-on-stop`) and `reboot` (`false` for `SetRestartFunc`, `true` for `SetRebootFunc`). Runs BEFORE `r.StopForRestart()` drops sessions.
       - Writes `KeyGRMarker` only when `maxRestart > 0` (unchanged).
       - Runs retained-route snapshot/clear **independently of `maxRestart <= 0`**: when `reboot` is true (OS reboot clears the Linux kernel FIB) OR `!registry.ForwardingStatePreserved(runningTree)` (`flush-on-stop` true), calls `RemoveRetainedRoutes(store)` and explicitly skips snapshotting; otherwise snapshots live `locrib.Default()` into `zefs.KeyBGPRetainedRoutes`.
     - **`SIGTERM` / `request shutdown` / `quit` (`bgp/plugin/register.go` `OnDrain`, `DrainBudget: 5`):** runs during `runStopDrain` BEFORE any plugin stops (while `locrib.Default()` still holds every selected BGP route). Reads live `GetConfigTree()` from `server.ReactorAny()` (falling back to the last `Configure` map) and `store := attachedCoord.Bootstrap().Store`. Treats `input.Kind == rpc.DrainKindRestart && input.RestartCause == rpc.DrainRestartReboot` as `reboot == true` (skipping snapshotting and removing the key); otherwise calls `SaveOrClearRetainedRoutesOnce` based on `registry.ForwardingStatePreserved(tree)`.
     - **Fallback (`runBGPEngine` shutdown `defer`, only when `attachedCoord != nil`):** if drain was skipped (e.g., Ctrl+C during config fence or direct `Server.Stop()` in tests) and `SaveOrClearRetainedRoutesOnce` has not latched yet, runs the same live-tree `ForwardingStatePreserved` snapshot/remove before `bgpReactor.Stop()` and before Tier-0 `bgp-rib` stops.
   - When snapshotting: iterates `locrib.Default()` across all families and collects every prefix whose selected `Best` has `best.IsBGP && !best.Stale` (recording family, prefix, `IsEBGP`, `Metric`, `NextHop`, `Interface`, `OnLink`, `Weight`, `Labels`, `SRv6SID`, `RouteType`, `DistanceOverride`, `HasDistanceOverride`, `ECMP`). Encodes a compact versioned JSON snapshot (version 1, expiry timestamp `now + max(maxRestart, configuredMaxDelay, 300s)`, bounded to `BGPRetainedRoutesValueMax = 12_000_000` bytes; if empty, removes the key) and writes it atomically to `zefs.KeyBGPRetainedRoutes` (`"meta/bgp/retained-routes"`).
2. **Pre-tier restore & EventBus subscription (`plugin/server/startup.go::runPluginStartup` -> `bgp/plugin/register.go::beforeBGPStart`):**
   - `Server.runPluginStartup()` resolves the selected startup set (`configPhase` and `explicitPhase`) and, synchronously **before Phase 1 spawns Tier 0 (`bgp-rib`, `policy`, `interface`, `sysctl`) or Tier 1 (`bgp`, `rib`/`sysrib`, `ospf`, `isis`)**, calls `reg.BeforeStart(tree, store, s)` in plugin-name order for each selected registration declaring `BeforeStart`, registering any returned cleanup closure with `s.OnStop`.
   - `bgp`'s `beforeBGPStart(tree, store, eb)` resets the per-stop snapshot latch and **unconditionally** reads and removes `zefs.KeyBGPRetainedRoutes` once (so no snapshot ever leaks into a later cold start).
   - If `registry.ForwardingStatePreserved(tree)` is true AND `bgp update-delay max-delay > 0` (parsed from `tree["bgp"]`) AND the snapshot is valid and unexpired:
     - For each saved route, constructs `locrib.Path` with `Source: redistevents.ProtocolIDOf("bgp")`, `Instance: 0`, `IsBGP: true`, `IsEBGP`, `Metric`, forwarding fields, and `Stale: true`, and calls `locrib.Default().InsertStale(fam, prefix, p)`.
     - `InsertStale` stores `p` with `Stale: true` in `PathGroup` (using the same `admit` and `resolvedDistance` pipeline as `Insert`; `Path.Equal` compares `Stale` while `Path.key()` ignores it; `producerPath` preserves `Stale` across `Reselect()` so Tier-0 `policy` and Tier-1 `sysrib` `rib { distance }` changes re-rank retained candidates accurately).
     - Immediately subscribes on `eb` (`apiServer`, BEFORE Tier-0 `bgp-rib` starts, eliminating any early-completion race even with a tiny `maxDelay`):
       - `ribevents.SelectionFamilyComplete` -> calls `locrib.Default().SweepStaleFamily(bgpProtoID, ev.Family)`.
       - `ribevents.SelectionComplete` -> calls `locrib.Default().SweepStale(bgpProtoID)`, cancels the fallback timer, and unsubscribes both handlers.
     - Arms a bounded fallback timer (`maxDelay + 30s`, matching `fib-kernel`'s deferral safety margin) that calls `locrib.Default().SweepStale(bgpProtoID)` and unsubscribes if `bgp-rib` never emits `SelectionComplete` (for example, if `bgp-rib` crashes or is not loaded).
     - Returns a cleanup closure (registered on `Server.OnStop`) that stops the fallback timer and unsubscribes both handlers on daemon shutdown.
3. **Selection and forwarding while deferred (`locrib`, `sysrib.go`, `fibimport.go`):**
   - **Prefix P with retained BGP route (eBGP, distance 20) + arriving OSPF route (distance 110):** because `BeforeStart` already seeded P before Tier 0/1 started, `locrib.selectBest` keeps the `Stale: true` eBGP path as `Best` from the very first moment; OSPF stays stored as a non-best candidate; `locrib` dispatches NO `Change` for P; `sysrib` emits NO FIB change; the kernel keeps the previous run's BGP route untouched at its MED priority.
   - **Initial snapshot/scan, live updates, cascades, and FIB-import sweeps seeing an unprogrammed `Stale: true` winner:**
     - Internal `sysrib` reads `change.Best.Stale`; forked `sysrib` reads `rpc.RouteViewPath.Stale` -> `projectionPath` -> `locrib.Path.Stale`.
     - `prepareRouteViewSelectedMetadata` copies `path.Stale` into `protocolRoute.retainedStale`.
     - In `sysrib.go::recomputeForwarding`:
       - When `winner.retainedStale && !s.programmedByZe(key)`: records `s.best[key] = winner` (keeping LPM and `show rib` accurate), releases previous tracking if `prev != nil`, does NOT write `s.installed[key]`, and returns `nil` (no Add/Update emitted to `fib-kernel`, mirroring `recordOSInstalledWinner`/`recordWithheldWinner`). If `s.programmedByZe(key)` WAS true (a higher-priority IGP route won during deferral and then withdrew before deferral ended), emits an Update restoring the retained BGP route so forwarding is not blackholed.
       - In the no-op comparison (`recomputeForwarding` L468-481): includes `prev.retainedStale == winner.retainedStale` so a fresh BGP route (`retainedStale: false`) replacing an unprogrammed stale winner (`retainedStale: true`) with identical gateway/metric/labels ALWAYS falls through and emits an `Add` refresh even when `verdict == fibUnproved`.
     - In `sysrib.go::cascadeRecompute` (L732-795): guards `best.retainedStale && !s.programmedByZe(key)` -> returns `nil` (mirroring `best.osInstalled` and `!s.fibPermitted(best.protocol)`).
     - In `fibimport.go`:
       - `trackNextHopGroup` (L235-262): skips next-hop tracking when `winner.retainedStale && !s.programmedByZe(key)` (mirroring `winner.osInstalled`).
       - `applyFIBImport` (L338-380): skips `s.trackNextHops` on newly permitted routes when `route.retainedStale && !s.programmedByZe(key)`.
       - `fibStateChange` (L430-440): returns `nil` without calling `fibInstall` when `route.retainedStale && !s.programmedByZe(key)` (mirroring `route.osInstalled`).
   - **Prefix Q with no retained BGP route + arriving IGP route:** `locrib` selects IGP (`Stale: false`) -> `sysrib` publishes Add -> `fib-kernel` installs immediately.
   - **Prefix R with retained BGP route (distance 20) + arriving IGP/static route with better distance (e.g. `rib { distance }` ranking OSPF 10 < eBGP 20):** Tier-1 `sysrib` `OnConfigure` applies `rib { distance }` (`Reselect` preserves `Stale` and re-ranks R before `sysrib` publishes) -> `locrib.selectBest` selects IGP (`Stale: false`) -> `sysrib` publishes Add -> `fib-kernel` installs IGP immediately.
   - **Redistribution observers:** `internal/component/bgp/redistribute/producer.go` publishes only from `bgp-rib`'s `BestChangeBatch` (which is suppressed during `selectionDeferral`), not from `locrib.OnChange`, so `InsertStale` never redistributes or re-advertises retained routes to BGP peers before BGP selection runs.
4. **Deferral completion (`selection_deferral.go` -> `SweepStaleFamily` / `SweepStale`):**
   - When family F's EORs arrive (or `Selection_Deferral_Timer` expires), `evaluateReleasesLocked()` and `releaseAll()` return **every newly released family** (`[]releasedFamily{family, dirty}` — including families that received zero UPDATEs and only an End-of-RIB marker).
   - For each released family `r`:
     - If `r.dirty` is true: `reselectDeferredFamily(r.family)` calls `publishBestChanges(changes, r.family)` synchronously (updating `locrib.Default()` with fresh `Stale: false` BGP paths, internally or via `route-install` RPC).
     - For every prefix P that BGP re-learned: `InsertForward` replaces the `(bgp, 0)` `Stale: true` path with `Stale: false`. Because `Path.Equal` compares `Stale` and `recomputeForwarding` checks `prev.retainedStale == winner.retainedStale`, `locrib` dispatches `ChangeUpdate` (`Stale: false`) -> `sysrib` publishes Add (even on `fibUnproved`) -> `fib-kernel` refreshes the kernel route and drops it from `staleTables` (`RFC4724-4.1-6`).
     - Unconditionally (whether `r.dirty` was true or false — including zero-route family EORs): `bgp-rib` emits `ribevents.SelectionFamilyComplete{Family: r.family}` on the EventBus.
   - Once all families are released (or `Selection_Deferral_Timer` expires), `bgp-rib` emits `ribevents.SelectionComplete` as today.
   - The daemon's pre-tier subscriber calls `locrib.Default().SweepStaleFamily(bgpProtoID, ev.Family)` (and `SweepStale(bgpProtoID)` on `SelectionComplete`):
     - Every unrefreshed `Stale: true` BGP path for that family is removed immediately (even when the peer sent zero UPDATEs and only an EOR for that family, without waiting for other families or `Selection_Deferral_Timer`).
     - If prefix P had a waiting IGP candidate, `selectBest` promotes the IGP route (`Stale: false`) -> `ChangeUpdate` -> `sysrib` publishes Add -> `fib-kernel` installs IGP and removes the leftover BGP route.
     - If prefix S had no other candidate, `locrib` dispatches `ChangeRemove` -> `sysrib` clears `s.best[S]` (no Withdraw because `!programmedByZe(S)`), and `fib-kernel`'s `waitAndSweepStale` removes the leftover kernel route on `SelectionComplete`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Daemon shutdown/start ↔ ZeFS `statestore` | `grmarker.SaveOrClearRetainedRoutesOnce` / `WriteRetainedRoutes` / `ReadRetainedRoutes` / `RemoveRetainedRoutes` under registered `zefs.KeyBGPRetainedRoutes` | No |
| Always-on hub/plugin-server ↔ `ze_bgp`-gated BGP lifecycle | Existing `infra.WriteGRMarker(caps, store, runningTree, reboot)` seam (`config/infra/bgp.go`), generic `Registration.BeforeStart(tree, store, eb)` hook (`plugin/registry`, `plugin/server/startup.go`), and existing `OnDrain` RPC (`bgp/plugin/register.go`) | No |
| `bgp-rib` (internal or forked) ↔ Daemon `locrib` sweep | EventBus `ribevents.SelectionFamilyComplete` (emitted for every released family, dirty or zero-route) and `ribevents.SelectionComplete` (and existing `route-install` RPC for fresh BGP paths) | No |
| Daemon `locrib` ↔ `sysrib` (internal or forked) | `locrib.Path.Stale` directly (`local-authority`) and `rpc.RouteViewPath.Stale` (`json:"stale,omitempty"`) over `ze-sysrib:route-view-read` (`selected-feed`) | No |

### Changed-Wire Census

**Search evidence:** Searched `pkg/plugin/rpc/route_view.go`, `internal/component/sysrib/`, `internal/core/bgp/ribevents/`, `internal/component/bgp/plugins/rib/`, and `pkg/zefs/keys.go` for `RouteViewPath`, `routeViewCopyEntry`, `projectionPath`, `EventSelectionComplete`, and `KeyGRMarker` (positive controls: `RouteViewPath.SRv6SID` written in `sysrib/route_view_path.go::routeViewCopyEntry` and read in `sysrib/projection_path.go::projectionPath`; `SelectionComplete` emitted in `rib/selection_deferral.go::emitSelectionComplete` and subscribed in `fib/kernel/fibkernel.go::waitAndSweepStale`). Three wires change (all additive): (1) optional `stale` boolean on `rpc.RouteViewPath`, (2) new EventBus event `(bgp-rib, selection-family-complete)`, and (3) new ZeFS state key `meta/bgp/retained-routes`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `ze-sysrib:route-view-read` `RouteViewPath.stale` | writer | `internal/component/sysrib/route_view_path.go::routeViewCopyEntry` (schema `pkg/plugin/rpc/route_view.go::RouteViewPath`) | Copies `best.Stale` into `entry.Path.Stale` (`json:"stale,omitempty"`); omitted for every ordinary non-stale route | Change: additive optional bool; zero overhead on non-stale routes; preserves existing JSON for all other paths |
| `ze-sysrib:route-view-read` `RouteViewPath.stale` | reader | `internal/component/sysrib/projection_path.go::projectionPath` -> `internal/component/sysrib/route_view_forwarding.go::prepareRouteViewSelectedMetadata` -> `sysrib.go` (`recomputeForwarding`, `cascadeRecompute`) and `fibimport.go` (`trackNextHopGroup`, `applyFIBImport`, `fibStateChange`) | Decodes `wire.Stale` into `value.path.Stale` -> `protocolRoute.retainedStale`; suppresses initial FIB write/tracking/cascade/fib-import for unprogrammed stale winners while treating `prev.retainedStale != winner.retainedStale` as non-no-op; unknown JSON field is safely ignored by older third-party consumers | Change: internal and forked `sysrib` share identical `recomputeForwarding`, `cascadeRecompute`, and `fibimport.go` rules |
| EventBus `(bgp-rib, selection-family-complete)` | writer | `internal/component/bgp/plugins/rib/selection_deferral.go::emitSelectionFamilyComplete` (called from `evaluateReleasesLocked` and `releaseAll` for every released family, dirty or zero-route; declared in `rib/register.go::ribBusSends` and `ribevents/ribevents.go::SelectionFamilyComplete`) | Emitted for every newly released family (after `reselectDeferredFamily(fam)` for dirty families, and immediately on zero-route family EOR); payload carries `family` | Change: additive EventBus event; existing `selection-deferred` and `selection-complete` payloads unchanged |
| EventBus `(bgp-rib, selection-family-complete)` | reader | `internal/component/bgp/plugin/register.go::beforeBGPStart` (pre-tier daemon deferral-sweep subscriber, declared in `bgpBusReceives`) | Subscribes on daemon `EventBus` BEFORE Tier-0 `bgp-rib` starts to sweep unrefreshed `Stale: true` BGP paths for `ev.Family` via `locrib.Default().SweepStaleFamily`; works identically for internal and forked `bgp-rib` | Change: EventBus-only coordination (never process-global atomics); zero startup subscription race |
| ZeFS key `meta/bgp/retained-routes` | writer | `internal/component/bgp/grmarker/retained_routes.go::SaveOrClearRetainedRoutesOnce` / `WriteRetainedRoutes` (called from `bgp/config/register.go::writeGRMarker`, `bgp/plugin/register.go` `OnDrain` and `runBGPEngine` defer; key registered in `pkg/zefs/keys.go::KeyBGPRetainedRoutes`) | Writes versioned JSON snapshot of BGP's selected Loc-RIB paths on clean non-reboot stop when live `ForwardingStatePreserved` is true (independent of `maxRestart <= 0`); deletes when false or on `request reboot` | Change: new key with `ValueMax: BGPRetainedRoutesValueMax`; absent key on first boot, reboot, or upgrade reads cleanly as no retained routes |
| ZeFS key `meta/bgp/retained-routes` | reader | `internal/component/bgp/grmarker/retained_routes.go::ReadRetainedRoutes` called from `internal/component/bgp/plugin/register.go::beforeBGPStart` (via `Registration.BeforeStart` in `plugin/server/startup.go::runPluginStartup`) | Reads and removes key once before Tier 0/1 plugins spawn; corrupt, expired, or wrong-version payload logs a warning and degrades safely to empty (no crash, never prevents `ze start`) | Change: single-use read-and-delete matches `KeyGRMarker` lifecycle; pre-tier execution eliminates Tier-1 seeding race |

### Integration Points
- `pkg/zefs/keys.go::KeyBGPRetainedRoutes`
- `internal/core/rib/locrib` (`Path.Stale`, `InsertStale`, `SweepStaleFamily`, `SweepStale`)
- `internal/core/bgp/ribevents/ribevents.go` (`EventSelectionFamilyComplete`, `SelectionFamilyCompleteEvent`, `SelectionFamilyComplete`)
- `pkg/plugin/rpc/route_view.go` (`RouteViewPath.Stale`)
- `internal/component/plugin/registry/registry.go` (`Registration.BeforeStart`)
- `internal/component/plugin/server/startup.go` (`runPluginStartup` pre-tier `BeforeStart` invocation + `Server.OnStop` cleanup registration)
- `internal/component/config/infra/bgp.go` (`GRMarkerWriter`, `WriteGRMarker` carrying live `tree` and `reboot bool`)
- `cmd/ze/hub/infra_setup.go` (`stopForRestart` passing live `GetConfigTree()` and `reboot` flag for `request restart` vs `request reboot`)
- `internal/component/bgp/plugins/rib/register.go` (`ribBusSends` and `events.RegisterNamespace` / `RegisterMessage` for `selection-family-complete`)
- `internal/component/bgp/plugin/register.go` (`BeforeStart`, `OnDrain` with `DrainBudget: 5`, and `bgpBusReceives` for `bgp-rib:selection-family-complete` and `bgp-rib:selection-complete`)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (design) | Cross-protocol arbitration stays in `locrib.RIB`; forwarding evaluation stays in `sysrib` (`recomputeForwarding`, `cascadeRecompute`, `fibimport.go`); kernel programming stays in `fib-kernel` |
| No unintended coupling (components stay isolated) | Yes (design) | `locrib` learns no BGP types (`Stale`, `InsertStale`, `SweepStale(source)` are protocol-generic); `plugin/registry`, `plugin/server`, `config/infra`, and `cmd/ze/hub` carry zero BGP imports or BGP-specific knowledge (`BeforeStart` is generic; `WriteGRMarker` extends an existing always-on seam with `map[string]any` and `bool`) |
| No duplicated functionality (extends existing, does not recreate) | Yes (design) | Reuses `locrib.selectBest`, `locrib.Reselect`, `route-view-*`, `grmarker`'s `zefs` store lifecycle, `OnDrain`, `Server.OnStop`, and `ribevents` deferral events |
| Zero-copy preserved where applicable (refs, not copies) | Yes (design) | `Path.Stale` is one bool fitting existing `Path` padding; zero allocation added to normal `Insert`/`InsertForward` hot path |
| Registration over hardcoding, outbound | Yes (design) | `zefs.KeyBGPRetainedRoutes` registers in `pkg/zefs/keys.go`; `SelectionFamilyComplete` registers via `events.Register` and `ribBusSends`; `beforeBGPStart` registers via `Registration.BeforeStart` |
| Registration over hardcoding, inbound | Yes (design) | Searched `locrib/distance.go`, `sysrib/route_view_path.go`, `sysrib/projection_path.go`, and `plugin/server/startup.go`: no protocol switch or hardcoded plugin list added; `runPluginStartup` invokes every selected `Registration.BeforeStart` generically |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `reselectDeferredFamily(fam)` publishes all fresh BGP best changes into daemon `locrib.Default()` synchronously before `emitSelectionFamilyComplete(fam)` fires | `selection_deferral.go:449` calls `publishBestChanges(changes, fam)` synchronously (internal `InsertForward` or RPC `route-install` waits for reply) | `SelectionFamilyComplete` could sweep a prefix before its fresh BGP insert lands | Unit test `TestSelectionDeferralEmitsFamilyCompleteBeforeAllComplete` driving both dirty and zero-route family EORs | unvalidated |
| A-2 | `writeGRMarker` (before `StopForRestart` on `request restart`), `bgp` `OnDrain` (before plugin teardown on `SIGTERM`/`request shutdown`), and `runBGPEngine` defer fallback (Tier 1 before Tier-0 `bgp-rib` stops) all see BGP's selected paths still present in `locrib.Default()` | `cmd/ze/hub/infra_setup.go:274` runs `WriteGRMarker` before `StopForRestart()`; `cmd/ze/hub/drain.go:220` runs `OnDrain` before `Server.Stop()`; `shutdown.go` tears down plugins in reverse tier order (Tier 1 `bgp` before Tier 0 `bgp-rib`) | Snapshot could see an empty `locrib` if routes were withdrawn earlier | Unit test `TestBeforeStartSeedsBeforeTiersAndStopSkipsReboot` and Linux QEMU restart test | unvalidated |
| A-3 | Snapshot size fits within `BGPRetainedRoutesValueMax` (12 MiB) for typical restarting-speaker route tables; if a table exceeds the cap, truncation is refused/warned rather than corrupting ZeFS | `pkg/zefs/keys.go` `IRRDatasetValueMax = 12_000_000` precedent | Oversized snapshot would fail `store.WriteFile` | Boundary test for snapshot exceeding `ValueMax` logs a warning and removes stale key rather than failing shutdown | unvalidated |
| A-4 | `Path.Stale` fits in existing struct alignment padding of `locrib.Path` | `candidate.go` already carries multiple adjacent `bool` fields (`IsEBGP`, `IsBGP`, `AIGPPresent`, `MetricRecursive`, `HasTag`, `Hidden`) | Minor struct size increase | `unsafe.Sizeof(locrib.Path{})` check in unit test | unvalidated |
| A-5 | `Server.runPluginStartup()` already has the lowered config tree (`busConfigTree`), the live `storage.Storage` (`Coordinator.Bootstrap().Store`), and `EventBus` (`s`) wired before Tier 0 spawns | `cmd/ze/hub/main.go:1041,1126,1159` wire bootstrap store, bus config tree, and EventBus before `eng.Start` | `BeforeStart` would see nil tree or store and fail to seed | Unit test `TestBeforeStartSeedsBeforeTiersAndStopSkipsReboot` driving `Server.StartWithContext` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Tier-0 `bgp-rib` finishes deferral early (tiny `max-delay`) or Tier-1 `ospf`/`isis`/`rib` starts in parallel with `bgp` and overwrites retained BGP routes before seeding completes | Unit test `TestBeforeStartSeedsBeforeTiersAndStopSkipsReboot` | `Registration.BeforeStart` runs synchronously inside `Server.runPluginStartup` BEFORE Tier 0 spawns, seeding `locrib.Default()` AND subscribing to `SelectionFamilyComplete`/`SelectionComplete` on `EventBus` before any plugin starts |
| R-2 | `bgp-rib` crashes or is disabled on restart, so `SelectionComplete` never arrives and `Stale: true` candidates would block IGP routes forever | `TestRetainedStaleSweepFallbackTimer` | Daemon arms a fallback timer (`maxDelay + 30s`) in `BeforeStart` (cleaned up on `Server.OnStop`) that calls `locrib.Default().SweepStale(bgpProtoID)` unconditionally |
| R-3 | `request reboot` writes `KeyBGPRetainedRoutes` even though OS reboot clears the kernel FIB, blackholing traffic on the rebooted host during deferral; or `ze` crashes (`SIGKILL`) | `TestBeforeStartSeedsBeforeTiersAndStopSkipsReboot` (reboot subtest) | `cmd/ze/hub/infra_setup.go` passes `reboot = true` to `infra.WriteGRMarker` on `request reboot`, and `OnDrain` checks `DrainRestartReboot`: both explicitly skip snapshotting and call `RemoveRetainedRoutes(store)`. `SIGKILL` is documented under Known Limitations (RFC 4724 Section 4.1 "retain, if possible") |
| R-4 | A higher-priority IGP route beats the stale BGP candidate during deferral, installs into the kernel (replacing the BGP route), and then withdraws BEFORE deferral ends; OR a fresh BGP route with identical `fibUnproved` gateway replaces an unprogrammed stale winner | Unit test `TestSysRIBDeferralRetainsBGPWinnerOverIGP_LocalAndForked` | In `recomputeForwarding`, if `s.programmedByZe(key)` is true when `winner.retainedStale` takes back `Best`, emit an Update restoring the retained BGP route; and include `prev.retainedStale == winner.retainedStale` in the no-op check so fresh BGP replacing an unprogrammed stale winner always emits an Add refresh even when `verdict == fibUnproved` |
| R-5 | Interaction with `plan/release/fixes/spec-fib-kernel-label-ownership.md` | Both touch restarting-speaker forwarding retention | Disjoint files: this spec edits `locrib`, `sysrib`, `grmarker`, `ribevents`, `zefs`, `rpc/route_view.go`, `plugin/registry`, `plugin/server/startup.go`, `config/infra/bgp.go`, and `cmd/ze/hub/infra_setup.go`, and does NOT edit `fib-kernel`; when BGP wins during deferral `fib-kernel` receives no IGP install at all |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Retained BGP routes displaced by IGP during deferral (current bug), or stale BGP candidates lingering after deferral and blocking IGP failover (mitigated by `SelectionFamilyComplete`, `SelectionComplete`, and fallback timer) |
| How is it reverted? | Single commit revert per phase; `KeyBGPRetainedRoutes` is single-use and ignored/absent after revert |
| Who else touches this path? | `plan/release/fixes/spec-fib-kernel-label-ownership.md` (disjoint files in `fib-kernel`; composes cleanly in either landing order); parent `plan/release/fixes/spec-bgp-graceful-restart-rfc-defects.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Real `ze` restart (`flush-on-stop` false, `bgp update-delay max-delay` set) with competing IGP/static route on Linux kernel | → | `WriteRetainedRoutes` -> `ReadRetainedRoutes` -> `locrib.InsertStale` -> `sysrib.recomputeForwarding` -> `reselectDeferredFamily` -> `SelectionFamilyComplete`/`SelectionComplete` -> `SweepStale` | `TestFIBKernelRetainsBGPDuringDeferralOverIGP` (Linux/QEMU integration test in `internal/plugins/fib/kernel/rfc4724_deferral_igp_integration_linux_test.go`, both internal and forked `sysrib`) |
| `locrib` selection + `sysrib` (internal `local-authority` and forked `selected-feed`) during deferral | → | `locrib.InsertStale` -> `routeViewCopyEntry` -> `projectionPath` -> `recomputeForwarding` -> `SweepStaleFamily` / `SweepStale` | `TestSysRIBDeferralRetainsBGPWinnerOverIGP_LocalAndForked` (`internal/component/sysrib/deferral_retained_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Previous run left an eBGP route for prefix P in the kernel (protocol 250, priority = MED); `ze` restarts with `flush-on-stop` false and `bgp update-delay max-delay` set, and an OSPF or static route for P (worse distance than eBGP) enters `locrib` before BGP's selection for P's family is released | `locrib` keeps the `Stale: true` eBGP candidate as `Best` and holds the IGP route as a losing candidate; `sysrib` (both internal and forked) emits zero FIB changes for P; the Linux kernel table retains the previous run's BGP route at its MED priority throughout the deferral window | Competing IGP candidates stay stored in `locrib` ready for immediate promotion if BGP omits P at release; unrelated prefixes are unaffected |
| AC-2 | In the same restart, BGP's deferral ends for P's family (`evaluateReleasesLocked` / `releaseAll` -> `reselectDeferredFamily` for dirty families -> `SelectionFamilyComplete` for every released family -> `SelectionComplete`): case (a) BGP re-learns a best path for P (including identical `fibUnproved` gateway); case (b) BGP does not re-learn P (dirty family or zero-route family EOR) | In case (a), fresh BGP (`Stale: false`) replaces the stale candidate, `sysrib` treats `prev.retainedStale != winner.retainedStale` as non-no-op and emits an Add/Update (even when `verdict == fibUnproved`), and `fib-kernel` refreshes P's kernel route (RFC 4724 Section 4.1-6); in case (b) — including a zero-route family EOR where the peer sends zero UPDATEs for that family while another family stays deferred — `SelectionFamilyComplete` triggers `SweepStaleFamily`, removing the stale BGP candidate immediately, `locrib` promotes the waiting IGP route, `sysrib` publishes it, and `fib-kernel` installs the IGP route and removes the leftover BGP route | If prefix S had neither a re-learned BGP route nor an IGP route, `SweepStale` removes S from `locrib` and `fib-kernel`'s `waitAndSweepStale` deletes the leftover kernel route on `SelectionComplete` |
| AC-3 | In the same restart (negative polarity): prefix Q has NO retained BGP route, and prefix R has a retained BGP route whose administrative distance loses to the arriving IGP/static route (for example, static distance 1 or `rib { distance }` ranking OSPF below eBGP) | Both Q and R select the IGP/static route immediately during deferral, `sysrib` publishes both immediately, and the Linux kernel installs both immediately without waiting for `SelectionComplete` | Retained BGP route for R stays in `locrib` as a losing stale candidate until swept at deferral end |
| AC-4 | `bgp-rib` never emits `SelectionComplete` (fault or crash) after retained BGP routes were seeded; OR `bgp-rib` finishes deferral early in Tier 0 before Tier-1 `bgp` finishes `OnConfigure` | Pre-tier `BeforeStart` registers the EventBus sweep handlers before Tier 0 spawns so an early Tier-0 `SelectionFamilyComplete` / `SelectionComplete` sweeps immediately (never waiting for the fallback timer); if `bgp-rib` crashes and never emits `SelectionComplete`, the bounded fallback timer (`maxDelay + 30s`) calls `locrib.SweepStale`, promoting waiting IGP routes so stale BGP candidates never permanently block convergence | Non-stale routes inserted by live protocols remain untouched |
| AC-5 | `ze` stops with `flush-on-stop` true (including toggled live via config reload), stops via `request reboot` (even with `flush-on-stop` false), stops with `maxRestart == 0` and `flush-on-stop` false, starts with `bgp update-delay max-delay` unset (0), or reads a corrupt/expired `zefs.KeyBGPRetainedRoutes` entry | `flush-on-stop` true and `request reboot` both explicitly skip snapshotting and delete `KeyBGPRetainedRoutes` (since OS reboot clears the kernel FIB); `maxRestart == 0` with `flush-on-stop` false still writes the snapshot on non-reboot clean stop (`request restart` / `SIGTERM`); `max-delay == 0` removes any leftover snapshot and seeds zero stale routes; a corrupt or expired snapshot logs a warning, removes the key, and `ze` starts normally without failing (`core-design.md`, "Ze always starts") | All non-BGP routes and normal BGP startup proceed unchanged |
| AC-6 | Both internal (`local-authority`) and forked (`selected-feed`) `sysrib`, and both internal and forked `bgp-rib`, exercise AC-1 through AC-3 | Both placements produce identical `locrib` selection, identical `sysrib` FIB suppression during deferral, and identical post-deferral promotion/refresh; `TestRFC4724RestartingSpeakerForwardingState` remains untouched and green | Existing `route-view-*` snapshot, delta, and reset guarantees remain unchanged |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Restarts `ze` (with `flush-on-stop` false and `bgp update-delay max-delay` configured) while OSPF converges faster than BGP | `WriteRetainedRoutes` -> `ReadRetainedRoutes` -> `locrib.InsertStale` -> OSPF loses to stale eBGP -> kernel keeps BGP route through deferral -> BGP EOR refreshes or sweeps to OSPF | `TestFIBKernelRetainsBGPDuringDeferralOverIGP` (QEMU Linux integration) |
| 2 | Configures `rib { distance }` ranking OSPF better than eBGP and restarts `ze` | `locrib.Reselect` ranks stale eBGP below OSPF -> OSPF installs into kernel immediately during deferral | `TestFIBKernelRetainsBGPDuringDeferralOverIGP` (prefix R subtest) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLocRIBInsertStaleCompetesAndSweeps` | `internal/core/rib/locrib/stale_test.go` | AC-1, AC-2, AC-3: `InsertStale` beats worse-distance IGP, loses to better-distance IGP, survives `Reselect` with new `rib { distance }`, fresh `Insert` flips `Stale` to false and dispatches `ChangeUpdate`, `SweepStaleFamily` removes only that family's unrefreshed stale paths and promotes waiting IGP, `SweepStale` removes all remaining | new |
| `TestRetainedRoutesRoundTripAndGuards` | `internal/component/bgp/grmarker/retained_routes_test.go` | AC-5: `WriteRetainedRoutes` / `ReadRetainedRoutes` / `RemoveRetainedRoutes` / `SaveOrClearRetainedRoutesOnce` round-trip, expiry, corrupt JSON warning+degrade, `ValueMax` cap guard, single-use removal, once-per-stop deduplication | new |
| `TestSelectionDeferralEmitsFamilyCompleteBeforeAllComplete` | `internal/component/bgp/plugins/rib/rfc4724_selection_deferral_test.go` | AC-2: (a) dirty family EOR publishes fresh best changes synchronously before `SelectionFamilyComplete` fires for IPv4 while IPv6 stays deferred; (b) zero-route family EOR (negotiated family receiving zero UPDATEs) still emits `SelectionFamilyComplete{Family: fam}` immediately before `SelectionComplete` fires when remaining families finish | new |
| `TestSysRIBDeferralRetainsBGPWinnerOverIGP_LocalAndForked` | `internal/component/sysrib/deferral_retained_test.go` | AC-1, AC-2, AC-3, AC-6: both internal (`local-authority`) and forked (`selected-feed`) `sysrib` suppress FIB Add for untouched `Stale: true` BGP winner, install prefix Q and distance-winning prefix R immediately, (a) emit Add refresh when identical `fibUnproved` fresh BGP (`Stale: false`) replaces unprogrammed stale winner (`Stale: true`), (b) ignore covering-route cascade (`cascadeRecompute`) and skip next-hop tracking for unprogrammed stale winner during deferral, (c) skip FIB install and next-hop tracking during `applyFIBImport` (`fibStateChange`) for unprogrammed stale winner during deferral, promote OSPF on sweep, and restore stale BGP via Update if a higher-priority IGP wins and withdraws mid-deferral | new |
| `TestRetainedStaleSweepFallbackTimer` | `internal/component/bgp/plugin/retained_sweep_test.go` | AC-4, AC-5: pre-tier `BeforeStart` seeds `locrib.Default()` and subscribes BEFORE Tier-0 `bgp-rib` / Tier-1 `ospf`/`isis`/`rib` start (handling early Tier-0 `SelectionComplete` without waiting for fallback timer), fallback timer sweep when `SelectionComplete` never arrives, live-tree `ForwardingStatePreserved` snapshot independent of `maxRestart <= 0`, and `request reboot` (`reboot == true` / `DrainRestartReboot`) explicitly skipping snapshotting and removing any existing key | new |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `KeyBGPRetainedRoutes` encoded size | `0..12_000_000` bytes (`BGPRetainedRoutesValueMax`) | `12_000_000` | N/A | > `12_000_000`: logs warning, removes stale key, does not fail shutdown |
| `bgp update-delay max-delay` | `0..3600` seconds (existing `selectionDeferralMaxSeconds`) | `3600` (`0` disables seeding) | `< 0` refused by existing parser | `> 3600` refused by existing parser |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestFIBKernelRetainsBGPDuringDeferralOverIGP` (Linux/QEMU kernel integration) | `internal/plugins/fib/kernel/rfc4724_deferral_igp_integration_linux_test.go` | Real netlink kernel route table in isolated Linux netns (QEMU): seeds protocol-250 BGP route for P and R, starts `sysrib` (both internal and forked) + `fib-kernel` with `flush-on-stop` false and retained BGP candidates in `locrib`, injects competing IGP routes for P (loses to eBGP), Q (no retained BGP), and R (beats eBGP via `rib { distance }`); asserts kernel keeps P's BGP route untouched at its MED priority while Q and R install IGP immediately; then tests both post-deferral outcomes (BGP refresh keeps P; BGP omission sweeps stale candidate, installs IGP, and removes leftover BGP route; prefix S with no IGP is swept by `fib-kernel`) — tagged `RFC4724-4.1-1` positive and negative | new |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A (Linux kernel forwarding plane) | - | Linux kernel (QEMU) | D7 is a local Restarting Speaker forwarding-plane interaction between `locrib`, `sysrib`, and the Linux kernel FIB (zero BGP wire change); proven against the real Linux kernel in QEMU per `ai/doors/forwarding.md` | N-A |

## Files to Modify
- `pkg/zefs/keys.go` - register `KeyBGPRetainedRoutes` (`meta/bgp/retained-routes`) with `BGPRetainedRoutesValueMax`
- `pkg/plugin/rpc/route_view.go` - add optional `Stale bool` (`json:"stale,omitempty"`) to `RouteViewPath`
- `internal/core/rib/locrib/candidate.go` - add `Stale bool` to `Path` (included in `Equal`, excluded from `key()`)
- `internal/core/rib/locrib/manager.go` - add `InsertStale`, `SweepStaleFamily`, and `SweepStale` on `RIB`
- `internal/core/rib/locrib/policy.go` - preserve `Path.Stale` in `producerPath` and `admit`
- `internal/core/bgp/ribevents/ribevents.go` - declare `EventSelectionFamilyComplete`, `SelectionFamilyCompleteEvent`, and `SelectionFamilyComplete` handle
- `internal/component/plugin/registry/registry.go` - add optional `BeforeStart func(tree map[string]any, store storage.Storage, eb ze.EventBus) func()` pre-tier hook on `Registration`
- `internal/component/plugin/server/startup.go` - invoke `Registration.BeforeStart` synchronously for selected registrations in `runPluginStartup` before Tier 0 spawns, registering returned cleanup with `Server.OnStop`
- `internal/component/config/infra/bgp.go` - extend `GRMarkerWriter` and `WriteGRMarker` to pass the live running config tree (`map[string]any`) and `reboot bool`
- `cmd/ze/hub/infra_setup.go` - pass live `apiSrv.Reactor().GetConfigTree()` and `reboot` (`false` on `request restart`, `true` on `request reboot`) to `infra.WriteGRMarker`
- `internal/component/bgp/plugins/rib/register.go` - declare `bgp-rib:selection-family-complete` in `ribBusSends` and `events.RegisterNamespace`/`RegisterMessage`
- `internal/component/bgp/plugins/rib/selection_deferral.go` - return every newly released family (dirty or zero-route) from `evaluateReleasesLocked()` and `releaseAll()`, run `reselectDeferredFamily(fam)` for dirty families, and emit `SelectionFamilyComplete{Family: fam}` for every released family
- `internal/component/bgp/config/register.go` - in `writeGRMarker(caps, store, tree, reboot)`, write or clear `KeyBGPRetainedRoutes` via `grmarker.SaveOrClearRetainedRoutesOnce` independently of `maxRestart <= 0`, skipping snapshotting (and deleting any key) when `reboot` is true or `!ForwardingStatePreserved(tree)`
- `internal/component/bgp/plugin/register.go` - register `bgp`'s `BeforeStart` hook (single-use read+remove of `KeyBGPRetainedRoutes`, `locrib.Default().InsertStale`, EventBus sweep subscription before Tier-0 `bgp-rib` starts, `maxDelay + 30s` fallback timer + cleanup closure), declare `bgp-rib:selection-family-complete` and `bgp-rib:selection-complete` in `bgpBusReceives`, and register `OnDrain` (`DrainBudget: 5`) + `runBGPEngine` defer fallback reading live `GetConfigTree()` and `coord.Bootstrap().Store` (skipping on `DrainRestartReboot`)
- `internal/component/sysrib/route_view_path.go` - copy `best.Stale` into `rpc.RouteViewPath.Stale` in `routeViewCopyEntry`
- `internal/component/sysrib/projection_path.go` - decode `wire.Stale` into `locrib.Path.Stale` in `projectionPath`
- `internal/component/sysrib/route_view_forwarding.go` - copy `path.Stale` into `protocolRoute.retainedStale` in `prepareRouteViewSelectedMetadata`
- `internal/component/sysrib/sysrib.go` - add `retainedStale bool` to `protocolRoute`, suppress initial FIB writes in `recomputeForwarding` when `winner.retainedStale && !s.programmedByZe(key)`, treat `prev.retainedStale != winner.retainedStale` as non-no-op in `recomputeForwarding` even when `verdict == fibUnproved`, and guard `best.retainedStale && !s.programmedByZe(key)` in `cascadeRecompute`
- `internal/component/sysrib/fibimport.go` - skip next-hop tracking in `trackNextHopGroup` and `applyFIBImport` and skip FIB install in `fibStateChange` when `route.retainedStale && !s.programmedByZe(key)` (mirroring `osInstalled`)
- `docs/architecture/rib/unified-locrib.md` - document `Path.Stale`, `InsertStale`, `SweepStaleFamily`/`SweepStale`, and `sysrib` suppression/cascade/fib-import guards for untouched retained winners
- `docs/architecture/zefs-format.md` - document `meta/bgp/retained-routes` key
- `docs/architecture/api/process-protocol.md` - document `RouteViewPath.stale` and `(bgp-rib, selection-family-complete)` (including zero-route family EORs)
- `docs/architecture/hub-architecture.md` - document `Registration.BeforeStart` pre-tier hook beside `BootEffect`, and `request reboot` skipping retained-route snapshotting
- `docs/architecture/plugin/plugin-relationships.md` - document `bgp` pre-tier `BeforeStart` seeding/subscription before Tier 0/1, `bgp` `OnDrain`, and `bgp-rib:selection-family-complete`/`selection-complete` subscriptions
- `docs/guide/graceful-restart.md` - document Restarting Speaker forwarding-state retention across selection deferral (and `request reboot` clearing kernel forwarding state)

## Files to Create
- `internal/component/bgp/grmarker/retained_routes.go` - versioned ZeFS snapshot encoder/decoder (`WriteRetainedRoutes`, `ReadRetainedRoutes`, `RemoveRetainedRoutes`)
- `internal/component/bgp/grmarker/retained_routes_test.go` - unit and boundary tests for snapshot persistence and error handling
- `internal/core/rib/locrib/stale_test.go` - unit tests for `InsertStale`, `Reselect`, `SweepStaleFamily`, `SweepStale`
- `internal/component/sysrib/deferral_retained_test.go` - unit tests for both internal and forked `sysrib` deferral behavior
- `internal/component/bgp/plugin/retained_sweep_test.go` - unit tests for EventBus sweep handlers and fallback timer
- `internal/plugins/fib/kernel/rfc4724_deferral_igp_integration_linux_test.go` - Linux/QEMU kernel-table integration proof for AC-1, AC-2, AC-3, AC-6 (`RFC4724-4.1-1`)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Uses existing `bgp update-delay max-delay` and `fib kernel flush-on-stop` leaves |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | No | None added |
| CLI grammar (keyword before value) | N-A | None added |
| Editor autocomplete | N-A | None added |
| Functional test for new RPC/API | Yes | `internal/plugins/fib/kernel/rfc4724_deferral_igp_integration_linux_test.go` (QEMU Linux kernel proof, internal and forked `sysrib`) |
| Pipe completeness | N-A | No output command added |
| Env var registration | No | None added |
| Doctor check for runtime dependencies | No | No new runtime dependency |
| Doctor code per problem class | No | Uses existing startup/warning log path; corrupt snapshot degrades safely without refusing start |
| Prometheus counters/metrics | No | None added |
| BGP family surface (new SAFI / capability / attribute) | No | No family, capability, or attribute added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Fixes RFC 4724 Section 4.1 compliance defect on existing Restarting Speaker deferral |
| 2 | Config syntax changed? | No | None |
| 3 | CLI command added/changed? | No | None |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/process-protocol.md` (`RouteViewPath.stale` and `bgp-rib:selection-family-complete`) |
| 5 | Plugin added/changed? | No | No new plugin; `bgp-rib` and `sysrib` extend existing deferral/view behavior |
| 6 | Has a user guide page? | Yes | `docs/guide/graceful-restart.md` (Restarting Speaker forwarding retention during selection deferral) |
| 7 | Wire format changed? | No | No BGP protocol wire change |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` (`RouteViewPath.stale`, `selection-family-complete`) |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | New `RFC4724-4.1-1` integration proof and discrimination record (existing `TestRFC4724RestartingSpeakerForwardingState` untouched) |
| 10 | Test infrastructure changed? | No | Uses existing QEMU Linux integration runner |
| 11 | Affects daemon comparison? | No | None |
| 12 | Internal architecture changed? | Yes | `docs/architecture/rib/unified-locrib.md`, `docs/architecture/zefs-format.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/plugin/plugin-relationships.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `bgp-rib:selection-family-complete` event registered in `ribevents`, `rib/register.go`, and `bgp/plugin/register.go`; `Registration.BeforeStart` pre-tier hook added in `plugin/registry/registry.go`; `bgp` declares `OnDrain` (`DrainBudget: 5`) |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Checked with `./le spec citation anchors`: modified pages listed in `## Files to Modify` (`docs/architecture/rib/unified-locrib.md`, `docs/architecture/zefs-format.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/plugin/plugin-relationships.md`, `docs/guide/graceful-restart.md`); declared `// Design:` pages unaffected: `docs/architecture/config/syntax.md` (no config keyword or syntax changes), `docs/architecture/core-design.md` (daemon composition and BGP/sysrib boundaries unchanged; stale Loc-RIB retention belongs on `unified-locrib.md`), `docs/architecture/rib/forward-handle.md` (`InsertStale` carries `Forward == nil` and does not change `ForwardHandle` refcounting or `InsertForward`), `docs/architecture/api/architecture.md` (plugin registry model unchanged; pre-tier `BeforeStart` belongs on `hub-architecture.md`), and `ai/rules/plugins.md` (plugin ownership and compile-out rules unchanged; `infra.WriteGRMarker` stays an always-on inversion-of-control seam) |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No config or CLI syntax changes |

## Design presentation

### What changes

When `ze` stops cleanly (not an OS `request reboot`, which clears the kernel
FIB) with `flush-on-stop` false (evaluated against the live running config tree,
independently of `maxRestart <= 0`), the previous run's selected BGP Loc-RIB
routes are saved once into ZeFS (`meta/bgp/retained-routes`) before BGP sessions
or plugins tear down (`writeGRMarker` before `StopForRestart` on `request
restart`, `bgp`'s `OnDrain` before plugin teardown on `SIGTERM`/`request
shutdown`, with a `runBGPEngine` defer fallback). On startup with
`flush-on-stop` false and `bgp update-delay max-delay > 0`, `bgp`'s pre-tier
`Registration.BeforeStart` hook runs synchronously inside `runPluginStartup`
**before Tier-0 (`bgp-rib`) and Tier-1 (`bgp`, `rib`/`sysrib`, `ospf`, `isis`)
plugins spawn**: it reads and removes `meta/bgp/retained-routes` once, seeds the
saved BGP paths into `locrib.Default()` as `Stale: true` candidates, and
registers the EventBus deferral-sweep handlers before `bgp-rib` can arm or
complete deferral. During deferral, retained BGP candidates compete at BGP's
real administrative distance (`ebgp` or `ibgp`, honoring `rib { distance }`
overrides): a lower-priority IGP route loses in `locrib` and never overwrites
the kernel's retained BGP route, while a prefix with no retained BGP route or a
higher-priority IGP route installs immediately. `sysrib` (both internal and
forked, via `RouteViewPath.stale`) suppresses initial FIB writes, next-hop
tracking, cascades (`cascadeRecompute`), and FIB-import installs
(`fibimport.go`) for untouched stale winners while treating `prev.retainedStale
!= winner.retainedStale` as non-no-op even on `fibUnproved` so fresh BGP always
emits an Add refresh; once `bgp-rib` releases any family (dirty or zero-route
EOR) via `SelectionFamilyComplete` or completes all families via
`SelectionComplete`, the daemon sweeps unrefreshed stale BGP candidates so
waiting IGP routes install and `fib-kernel` cleans up obsolete kernel leftovers.

### Before and after

1. **In-place restart (`request restart` or `SIGTERM`) with `flush-on-stop` false, `bgp update-delay max-delay 60`, eBGP prefix P (distance 20, MED 50) in kernel, OSPF route for P (distance 110, metric 10) converges during deferral:**
   - Before: `locrib` holds only OSPF, `sysrib` emits Add for OSPF, and `fib-kernel` installs OSPF at priority 10 and deletes the retained eBGP route at priority 50 — traffic shifts to OSPF for up to 60 seconds.
   - After: pre-tier `BeforeStart` seeds P's `Stale: true` eBGP candidate (distance 20) before Tier 0/1 plugins start; `locrib` keeps stale eBGP beating OSPF (distance 110); `sysrib` emits no FIB write (and skips cascade/fib-import tracking) for P; the kernel continues forwarding on the retained eBGP route at priority 50 until BGP's deferral ends, at which point fresh BGP refreshes P (emitting Add even when `fibUnproved`) or `SweepStaleFamily` / `SweepStale` promotes OSPF (including on a zero-route family EOR).
2. **Same restart, prefix Q (no retained BGP route) and prefix R (`rib { distance }` ranks OSPF 10 < eBGP 20):**
   - Before and after: Q and R install OSPF in the kernel immediately during deferral.
3. **OS reboot (`request reboot`) with `flush-on-stop` false:**
   - After: `request reboot` explicitly skips snapshotting and removes `meta/bgp/retained-routes`, so the rebooted host (whose kernel FIB is empty) seeds zero stale BGP routes and installs IGP routes immediately.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Inferring protocol ownership of kernel leftover routes in `fib-kernel` from `rtm_protocol` or metric | Ze writes every FIB route under `rtprotZE` (250) and metric is protocol-specific (MED vs OSPF cost), so the kernel table cannot identify BGP or distinguish eBGP from iBGP distance |
| Holding back all IGP installs in `sysrib` or `fib-kernel` during `selectionDeferral` | Violates AC-3 (prefixes with no retained BGP route or with a better-distance IGP route must install immediately) and violates `unified-locrib.md` (cross-protocol selection belongs in `locrib`, not `sysrib`) |
| Reading `ribevents.ActiveSelectionDeferral()` atomic in daemon `locrib` | Breaks when `bgp-rib` runs as a forked subprocess (`ai/doors/system.md`, "A plugin can run in another process") |
| Seeding retained BGP routes inside Tier-1 `bgp` `OnConfigure` or post-start `BootEffect` | `BootEffect` runs after `eng.Start` (too late), and Tier-1 `bgp` runs in parallel with Tier-1 `ospf`, `isis`, and `rib`/`sysrib` and AFTER Tier-0 `bgp-rib` (leaving both a seeding race against IGPs and an early-completion race against `bgp-rib`); generic `Registration.BeforeStart` before Tier 0 eliminates both races with zero BGP knowledge in `plugin/server` or `hub` |
| Adding `StartAfter: ["bgp"]` edges onto `ospf`, `isis`, `rib`, and `bgp-rib` | Fragile across future or third-party route-producing plugins and reshapes the plugin tier graph unnecessarily when pre-tier `BeforeStart` solves it once for every producer |
| Adding per-protocol `rtm_protocol` numbers in `fib-kernel` | Would still not preserve `IsEBGP` vs `IsIBGP` distance, `DistanceOverride`, or work across non-kernel data planes (`fib-vpp`, `fib-dummy`), and would still require `locrib` arbitration |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register `zefs.KeyBGPRetainedRoutes` (`pkg/zefs/keys.go`), `ribevents.SelectionFamilyComplete` (`ribevents.go`, `rib/register.go`, `bgp/plugin/register.go`), `Path.Stale` (`locrib/candidate.go`), `RouteViewPath.Stale` (`rpc/route_view.go`), `Registration.BeforeStart` (`plugin/registry/registry.go`, `plugin/server/startup.go`), and extend `infra.WriteGRMarker` (`config/infra/bgp.go`, `cmd/ze/hub/infra_setup.go`, `bgp/config/register.go`); add failing wiring tests (`TestLocRIBInsertStaleCompetesAndSweeps`, `TestSysRIBDeferralRetainsBGPWinnerOverIGP_LocalAndForked`).
2. **Phase: Unified Loc-RIB stale candidates & sweep** -- implement `InsertStale`, `SweepStaleFamily`, `SweepStale` in `locrib/manager.go` and preserve `Stale` in `locrib/policy.go`. Tests: `TestLocRIBInsertStaleCompetesAndSweeps`.
3. **Phase: Snapshot persistence, pre-tier seeding & EventBus sweep coordination** -- implement `grmarker/retained_routes.go` (`WriteRetainedRoutes`, `ReadRetainedRoutes`, `RemoveRetainedRoutes`, `SaveOrClearRetainedRoutesOnce`), wire stop-time write/clear (skipping `request reboot`, independent of `maxRestart <= 0`, reading live `GetConfigTree()`) in `bgp/config/register.go::writeGRMarker`, `bgp/plugin/register.go` (`OnDrain` with `DrainBudget: 5` and `runBGPEngine` defer fallback), wire pre-tier `beforeBGPStart` via `Registration.BeforeStart` in `bgp/plugin/register.go` (seeding `locrib.Default()` and subscribing on `EventBus` before Tier-0 `bgp-rib` starts), and update `rib/selection_deferral.go` (`evaluateReleasesLocked` and `releaseAll`) to release every family (dirty or zero-route EOR), reselect dirty families, and emit `SelectionFamilyComplete{Family: fam}` for every released family. Tests: `TestRetainedRoutesRoundTripAndGuards`, `TestSelectionDeferralEmitsFamilyCompleteBeforeAllComplete`, `TestRetainedStaleSweepFallbackTimer`.
4. **Phase: `sysrib` internal & forked stale-winner suppression** -- plumb `Stale` through `route_view_path.go`, `projection_path.go`, `route_view_forwarding.go`, `sysrib.go` (`recomputeForwarding` suppression + `prev.retainedStale == winner.retainedStale` non-no-op check for `fibUnproved` refreshes + `cascadeRecompute` guard), and `fibimport.go` (`trackNextHopGroup`, `applyFIBImport`, `fibStateChange` guards). Tests: `TestSysRIBDeferralRetainsBGPWinnerOverIGP_LocalAndForked`.
5. **Phase: Linux/QEMU kernel proof & documentation** -- add `internal/plugins/fib/kernel/rfc4724_deferral_igp_integration_linux_test.go` (red before fix, green after, internal and forked `sysrib`), record `RFC4724-4.1-1` discrimination record, update `docs/architecture/rib/unified-locrib.md`, `docs/architecture/zefs-format.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/plugin/plugin-relationships.md`, and `docs/guide/graceful-restart.md`, and mark the `reconciler-claims-state-another-owner-installed` (2026-11-24) journal row `fixed:` in Commit A.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-6 all implemented and proven against real Linux kernel in QEMU |
| Feature completeness | Both internal and forked `sysrib`, and internal and forked `bgp-rib`, pass |
| Correctness | `KeyBGPRetainedRoutes` is removed on read (single-use), removed on `flush-on-stop` true (including live reload), removed on `request reboot`, and written independently of `maxRestart <= 0`; `BeforeStart` runs before Tier 0 spawns; `Reselect` preserves `Stale`; `recomputeForwarding` emits Add on `fibUnproved` stale->fresh refresh; `cascadeRecompute` and `fibimport.go` skip unprogrammed stale winners; zero-route family EOR emits `SelectionFamilyComplete`; fallback timer guarantees sweep if `SelectionComplete` never arrives; higher-priority IGP withdrawal mid-deferral restores stale BGP |
| Data flow | Zero reads of `ribevents.ActiveSelectionDeferral()` in daemon sweep logic; `locrib`, `plugin/registry`, `plugin/server`, `config/infra`, and `cmd/ze/hub` import nothing from `bgp`; `fib-kernel` receives zero IGP installs for losing IGP routes during deferral |
| Rule: performance | `Path.Stale` adds zero allocations to normal `Insert`/`InsertForward`; `omitempty` omits `stale` on every non-stale `RouteViewPath` |
| Rule: rfc-compliance | Enforcing code quotes RFC 4724 Section 4.1; existing `TestRFC4724RestartingSpeakerForwardingState` is untouched |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `locrib` stale candidate unit tests pass | `./le test unit pkg path ./internal/core/rib/locrib/` |
| `sysrib` internal & forked deferral tests pass | `./le test unit pkg path ./internal/component/sysrib/` |
| `grmarker`, `bgp/rib`, `bgp/plugin`, `plugin/server`, `config/infra` tests pass | `./le test unit pkg path ./internal/component/bgp/grmarker/`, `./internal/component/bgp/plugins/rib/`, `./internal/component/bgp/plugin/`, `./internal/component/plugin/server/`, `./internal/component/config/infra/` |
| Real Linux kernel deferral proof passes in QEMU | `./le test qemu pkg ./internal/plugins/fib/kernel/ run TestFIBKernelRetainsBGPDuringDeferralOverIGP` |
| Existing RFC4724-4.1-1 tagged test untouched | `git diff internal/plugins/fib/kernel/rfc4724_restarting_fib_test.go` is empty |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Corrupt, truncated, wrong-version, or oversized `KeyBGPRetainedRoutes` blob is rejected with a warning and deleted, never panicking or blocking `ze start` |
| Resource exhaustion | Snapshot size bounded by `BGPRetainedRoutesValueMax` (12 MiB); fallback timer bounds lifetime of every `Stale: true` candidate |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read source in Current Behavior; if misunderstood → RESEARCH |
| QEMU Linux integration test fails | Check kernel route table output and `sysrib` publication trace |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `reconciler-claims-state-another-owner-installed` | 2026-11-24 | `reselectDeferredFamily` (`rib/selection_deferral.go`), `locrib`, `sysrib`, `fib-kernel` `replaceOwnedRoute` (D7, RFC4724-4.1-1) |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Persist BGP's selected `locrib` paths in ZeFS (`KeyBGPRetainedRoutes`) on graceful non-reboot stop and seed them as `Stale: true` candidates in `locrib.Default()` before Tier 0 spawns via `Registration.BeforeStart` | Guess BGP ownership from kernel `rtm_protocol` / metric in `fib-kernel`; suppress all IGP routes in `sysrib` during deferral; seed in Tier-1 `bgp` `OnConfigure` or post-start `BootEffect` | Ze writes all FIB routes with `rtprotZE` (250), so kernel listing cannot identify eBGP/iBGP distance; `locrib` is the single cross-protocol selection authority for both internal and forked `sysrib` (`docs/architecture/rib/unified-locrib.md`); running pre-tier before Tier 0 eliminates both the Tier-1 parallel seeding race (`bgp` vs `ospf`/`isis`/`rib`) and the Tier-0 `bgp-rib` early-completion race with zero BGP knowledge in `plugin/server` or `hub` |
| Plumb `Stale` over `RouteViewPath` and suppress initial FIB writes, next-hop tracking, cascades, and FIB-import installs for untouched `Stale: true` winners in `sysrib.go` + `fibimport.go`, while treating `prev.retainedStale != winner.retainedStale` as non-no-op even on `fibUnproved` | Emit initial FIB Add for stale BGP winners; guard only `recomputeForwarding` | RFC 4724 Section 4.1 forbids updating forwarding state prior to route selection, and emitting an Add (or letting `cascadeRecompute`/`applyFIBImport` install it) would mark leftover routes refreshed in `fib-kernel` before BGP selects; checking `prev.retainedStale != winner.retainedStale` ensures identical `fibUnproved` fresh BGP routes always emit an Add refresh |
| Drive per-family and full sweep from EventBus `SelectionFamilyComplete` (emitted for every newly released family, dirty or zero-route EOR) and `SelectionComplete` subscribed pre-tier in `BeforeStart`, plus a bounded fallback timer | Read process-global `ribevents.ActiveSelectionDeferral()`; emit `SelectionFamilyComplete` only for `dirtyFamilies` | Works identically whether `bgp-rib` runs internal or forked (`ai/doors/system.md`), never misses an early Tier-0 completion, and releases stale routes immediately on a zero-route family EOR without waiting for other families |
| Evaluate live running `ForwardingStatePreserved(GetConfigTree())` independently of `maxRestart <= 0` across `writeGRMarker` (before `StopForRestart`) and `bgp` `OnDrain` / `runBGPEngine` defer, and explicitly skip snapshotting (removing any existing key) on `request reboot` | Snapshot inside `writeGRMarker` after `maxRestart <= 0` guard or snapshot on `request reboot` too | OS reboot clears the Linux kernel FIB (seeding stale routes after reboot would blackhole traffic during deferral); live `GetConfigTree()` reflects reloaded `flush-on-stop`; running independently of `maxRestart <= 0` covers restarts without GR capability restart-time |

## Known Limitations

- An unclean process kill (`SIGKILL`) cannot run the stop-time `WriteRetainedRoutes` hook; RFC 4724 Section 4.1 explicitly scopes forwarding retention to "retain, if possible". An OS `request reboot` clears the Linux kernel FIB, so `request reboot` intentionally skips snapshotting and removes any existing `KeyBGPRetainedRoutes` rather than seeding stale routes against an empty kernel table. Planned in-place restarts (`SIGTERM`, `request restart`) with `flush-on-stop` false always write the snapshot.

## RFC Documentation (Scope: protocol)

Add `// RFC 4724 Section 4.1: "<quoted requirement>"` above enforcing code in
`grmarker/retained_routes.go`, `locrib/manager.go` (`InsertStale`,
`SweepStaleFamily`, `SweepStale`), `rib/selection_deferral.go`
(`evaluateReleasesLocked`, `releaseAll`, `emitSelectionFamilyComplete`),
`sysrib/sysrib.go` (`recomputeForwarding`, `cascadeRecompute`), and
`sysrib/fibimport.go` (`trackNextHopGroup`, `applyFIBImport`, `fibStateChange`).

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 4724 Section 4.1 (RFC4724-4.1-1, 4.1-3, 4.1-6) | With `flush-on-stop` false (default) and `bgp update-delay max-delay > 0`, retained BGP Loc-RIB routes stay in the kernel through deferral against lower-priority IGP routes and are refreshed or swept on selection completion | `flush-on-stop true` flushes and removes any snapshot; `bgp update-delay max-delay 0` disables deferral; `rib { distance }` ranking IGP ahead of eBGP installs IGP immediately during deferral | `TestLocRIBInsertStaleCompetesAndSweeps`, `TestSysRIBDeferralRetainsBGPWinnerOverIGP_LocalAndForked`, `TestFIBKernelRetainsBGPDuringDeferralOverIGP` | `TestRetainedRoutesRoundTripAndGuards`, prefix Q and prefix R subtests of `TestFIBKernelRetainsBGPDuringDeferralOverIGP` |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, checkboxes kept unticked
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks every row Yes / No / N-A with reason

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted unit tests and QEMU Linux integration test pass in the main checkout (run by the main thread; GitHub CI runs whole-tree verification on push)
- [ ] Feature code integrated (`internal/*`, `pkg/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)
- [ ] Boundary tests for all numeric inputs
- [ ] Linux/QEMU kernel integration test proving kernel route table behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + journal row closure
- [ ] **Commit B:** `remove plan/release/fixes/spec-bgp-graceful-restart-rfc-defects-1-deferral-forwarding.md` only, in the same `./le commit create` script
