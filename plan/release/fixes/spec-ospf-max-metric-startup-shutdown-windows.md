# Spec: ospf-max-metric-startup-shutdown-windows

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P1 |
| Release reason | Accepted OSPF timed windows must change Router-LSA behavior for their configured duration. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the operator is promised.** Two leaves of `internal/plugins/ospf/yang/ze-ospf-conf.yang` fail for one reason and share this spec. `max-metric/router-lsa/on-startup` carries `ze:help` "Advertise as a stub router for N seconds after startup (0 disables)." and `max-metric/router-lsa/on-shutdown` carries `ze:help` "Advertise as a stub router for N seconds during a graceful shutdown (0 disables)." Both take 0 to 86400 seconds. An operator reads each as a timed window: Ze originates its Router-LSA with MaxLinkMetric (`0xffff`) on non-stub links for that many seconds, then restores the configured link costs. RFC 6987 Section 1 names "Graceful introduction and removal of the router to/from the network" as a reason to advertise the stub router, which is the case both windows serve.

**What Ze does instead at HEAD.** `parseMaxMetric` (`internal/plugins/ospf/config.go`) stores the two values in `maxMetricConfig.OnStartupSec` and `OnShutdownSec`, and `parseOSPFConfig` does not inherit `cfg.MaxMetric` into the OSPFv3 sub-configs (`cfg.V6`, `cfg.V6Extra`), even though `container max-metric` exists only under top-level `container ospf` and not inside `grouping ospf-af-topology`. Two readers of the timed fields exist, and neither counts a second: `deriveRICapabilities` (`internal/plugins/ospf/ri.go`) raises `caps.StubRouter` (RFC 7770 informational capability bit 2, announcing support rather than an active window), and `processSummary` (`internal/plugins/ospf/show_summary.go`) echoes both values while setting `Active` from `RouterLSAAlways` alone. Origination reads `RouterLSAAlways` alone as well: `originateSelfLSAs` (`internal/plugins/ospf/instance.go`) passes `cfg.MaxMetric.RouterLSAAlways` to `lsdb.OriginateFromTopology` and `v6OriginateSelf`. No timer, no clock, and no shutdown hook reads either field. `plan/audits/ospf-v2-must-audit.md` records the same gap under "Support on-startup and on-shutdown max-metric operation" (an audit only; it owns no fix).

**The approved design (owner decisions of 2026-10-13 unfolded).**
1. **Startup window (`ospf max-metric router-lsa on-startup`):** Built in full inside the OSPF plugin for both OSPFv2 and OSPFv3. `parseOSPFConfig` inherits `cfg.MaxMetric` unconditionally into `cfg.V6` and `cfg.V6Extra` (matching `ReferenceBandwidth`), so one top-level `max-metric` block drives every address family. Each engine arms its one-shot startup timer once per engine lifetime in `openInterfaces`, after `e.gr.maybeUnplannedRestart()` has run:
   - On a cold/non-GR start (base engine at boot, or an OSPFv2 instance or OSPFv3 address family added by reload), the window opens for `OnStartupSec` seconds; `originateSelfLSAs` and `processSummary` both read one window-aware helper.
   - On an RFC 3623 / RFC 5187 graceful restart (planned via NVS fact or unplanned via `planned-and-unplanned`, where `e.gr.inRestart()` is true), no startup window opens because forwarding state is preserved (owner Q-2).
   - `clear ospf process all` does not reopen the startup window, and reloading `on-startup` on a running engine does not arm or re-arm it (owner Q-2).
   - `show ospf` reports `stub-router.active: true` while the window is open alongside `stub-router.on-startup-remaining-seconds`.
2. **Shutdown drain (`ospf drain-on-shutdown`):** Under the owner's Q-1 (a) decision ("we need a drain phase for every protocol adding routes to the FIB/RIB"), the daemon-wide pre-stop drain lifecycle is owned by `plan/release/fixes/spec-daemon-drain-before-stop.md`. Phases 1–3 of that spec (hub drain orchestration in `cmd/ze/hub/drain.go`, SDK `OnDrain` and `drainWorker` in `pkg/plugin/sdk/sdk_drain.go`, and plugin server `BeginDrain` in `internal/component/plugin/server/drain.go`) have already landed at HEAD; OSPF joining that phase is Phase 5 of that spec. Under Q-6 of that spec (owner, 2026-10-13), `ospf max-metric router-lsa on-shutdown` is deleted and replaced by `ospf drain-on-shutdown` directly under `container ospf`, with no compatibility shim. Whichever of this spec or `spec-daemon-drain-before-stop.md` Phase 5 lands first performs the Q-6 rename. Until Phase 5 wires OSPF's `OnDrain`, this spec keeps `ospf drain-on-shutdown` in YANG and refuses any value above 0 in `validateConfigAF` with an `opererr` naming the leaf, why, and the fix (owner Q-3). To comply with the `Ze always starts` ruling (2026-11-26), the interim refusal also registers `doctor-ospf-drain-on-shutdown-unsupported` (`internal/plugins/ospf/register.go` and `doctor.go`): if a startup config carries `ospf drain-on-shutdown > 0`, `OnConfigure` logs an `ERROR` line under `doctor-ospf-drain-on-shutdown-unsupported` with the fix and leaves only OSPF unstarted while `ze` runs its other services, and `ze doctor` (`ospf-config-sanity` / `ospfConfigDiagnostics`) reports the same code and fix.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

Re-verified against HEAD on 2026-11-26. Collision check: `spec-daemon-drain-before-stop.md` Phases 1–3 landed at HEAD (hub/SDK/server drain plumbing); its Phases 4–6 (BGP/OSPF/IS-IS drain + Q-6 OSPF rename) remain open and ready.

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/routing-protocols.md`, `ai/doors/le-tooling.md` - subsystem doors covering every path this spec touches
  → Constraint: exact-or-reject (`ai/doors/system.md`); OSPFv2/OSPFv3 parity and FRR interop with discrimination reverts (`ai/doors/routing-protocols.md`); `option=needs-linux:caps=net-admin` for raw-socket `.ci` tests and scoped `./le test unit pkg` commands (`ai/doors/le-tooling.md`).
- [ ] `docs/architecture/ospf/ospf-13-cli-diag-interop.md` - the only page describing the two max-metric timed leaves
  → Constraint: its Traps bullet "`max-metric router-lsa on-startup` and `on-shutdown` are parsed and armed nowhere" also claims `stub-router.active` reads `always` only. Both sentences become false when the startup window and Q-6 rename land, so the page is updated in the same phase as the timer (recorded as `departs` in `## Design documents`).
- [ ] `docs/architecture/ospf/ospf-4-component-config.md` - OSPF config resolution and top-level inheritance into OSPFv3 address families
  → Constraint: router-wide leaves declared only under top-level `container ospf` (`reference-bandwidth`, `graceful-restart`, `fast-reroute`, `router-information`) are unconditionally inherited into `cfg.V6` and `cfg.V6Extra` in `parseOSPFConfig`; `max-metric` follows the exact same pattern.
- [ ] `docs/architecture/ospf/ospf-ext-9-graceful-restart.md` - planned and unplanned GR restarter lifecycle in `openInterfaces`
  → Constraint: `initializeState` calls `gr.resumeFromNVS()` and `openInterfaces` calls `gr.maybeUnplannedRestart()` before `releaseHelloHold()`. Arming the startup window AFTER `maybeUnplannedRestart()` lets `gr.inRestart()` gate both planned and unplanned GR starts.
- [ ] `docs/architecture/ospf/ospf-ext-11-ldp-igp-sync.md` and `internal/plugins/ospf/ldp_sync.go` - the existing injectable engine timer pattern
  → Decision: the startup window copies the LDP-sync timer shape: an injectable `afterFunc` seam returning a stoppable timer (`stop() bool` / `ldpSyncTimer`), an injectable `now` clock, and a monotonic `epoch` so a late callback after shutdown is a no-op. No new goroutine is spawned (`time.AfterFunc` + `originateSelfLSAsDeferred`).
- [ ] `docs/architecture/testing/interop.md` - scenario layout and declarative checker registration
  → Constraint: a new scenario is a named directory under `test/interop/scenarios/` with no numeric prefix, declarative config files only, and its assertions live in `scenarioOperations` in `internal/le/interoplab/bgp/checkers.go`.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc6987.txt` and `rfc/short/rfc6987.md` - OSPF stub router advertisement
  → Constraint: RFC 6987 Section 2: "To address both problems, router X announces its router-LSA to the neighbors with the cost of all non-stub links (links of the types other than 3) being set to MaxLinkMetric (defined in Section 3)." Applies to both OSPFv2 and OSPFv3. The existing encoders (`OriginateFromTopology`, `v6OriginateSelf`) are not touched.
  → Constraint: RFC 6987 Section 3: "It is defined to be the 16-bit binary value of all ones: 0xffff."
  → Constraint: RFC 6987 defines no timer. Section 1 names the use case only: "Graceful introduction and removal of the router to/from the network." New implementing functions quote Section 1 and Section 2 above the code (`ai/rules/rfc-compliance.md`).
  → Constraint: `rfc/short/rfc6987.md` is `Enrolment: backlog` with zero requirement rows, and RFC scope is frozen (2026-10-08), so no new `RFC requirement:` tag is added.
- [ ] `rfc/full/rfc7770.txt` Section 2.5 - Router Information stub-router capability bit
  → Decision: bit 2 ("OSPF Stub Router support [STUB]") announces support, not a window in force. `deriveRICapabilities` (`ri.go`) keeps raising it from any configured stub-router mode (`always`, `on-startup`, or `drain-on-shutdown`). `TestRICapabilityBitsFromState` (RFC7770-2.4-2) does not change.

**Key insights:**
- One window-aware flag source plus top-level `MaxMetric` inheritance into `V6`/`V6Extra` closes the entire startup-window gap for both address families; origination encoders already accept `maxMetric bool`.
- Checking `e.gr.inRestart()` after `e.gr.maybeUnplannedRestart()` in `openInterfaces` handles both planned and unplanned GR starts uniformly, while `clearProcess` never calls `openInterfaces`, matching Thomas's Q-2 ruling without special cases.
- The daemon drain infrastructure (Phases 1–3 of `spec-daemon-drain-before-stop.md`) is already landed at HEAD; OSPF joining it is Phase 5 of that spec. This spec performs the Q-6 leaf rename if it lands first and refuses `ospf drain-on-shutdown > 0` until Phase 5 replaces the refusal.

## Current Behavior (MANDATORY)

**Source files read at HEAD (2026-11-26):**
- [ ] `internal/plugins/ospf/yang/ze-ospf-conf.yang` - `container max-metric / container router-lsa` sits directly under `container ospf` (absent from `grouping ospf-af-topology`): `always` (boolean, default false), `on-startup` and `on-shutdown` (uint32, range `0..86400`, units seconds). Each timed leaf's `description` states Ze starts no timed window, while `ze:help` still promises timed startup and shutdown stub-router behavior.
  → Constraint: the only non-test readers of `OnStartupSec` and `OnShutdownSec` at HEAD are `parseMaxMetric`, `deriveRICapabilities`, and `processSummary`.
- [ ] `internal/plugins/ospf/config.go` - `maxMetricConfig` holds `RouterLSAAlways`, `OnStartupSec`, `OnShutdownSec`; `parseMaxMetric` fills it; `validateConfig` delegates to `validateConfigAF`, reached by `verifyOSPFConfigSections` (`InProcessConfigVerifier`, used by `ze config validate` and commit/reload verification) and `parseOSPFTransactionSections` (`sdk.ConfigTx`, used by `OnConfigVerify` and `OnConfigure`). `parseOSPFConfig` inherits `RouterID`, `RouterInformation`, `GracefulRestart`, `FastReroute`, and `ReferenceBandwidth` into `cfg.V6` and `cfg.V6Extra`, **but omits `cfg.MaxMetric`**, so top-level `max-metric` never reaches any OSPFv3 engine today.
  → Decision: inherit `cfg.MaxMetric` unconditionally into `cfg.V6` and `cfg.V6Extra` in `parseOSPFConfig` beside `ReferenceBandwidth`; place the interim `drain-on-shutdown > 0` refusal in `validateConfigAF` (returning an `opererr` wrapping a named sentinel error), not in the parser, so `TestOSPFMaxMetricConfig` continues proving parse behavior.
- [ ] `internal/plugins/ospf/instance.go` - `originateSelfLSAs` is the single self-LSA chokepoint for both families: checks `e.ctx.Err()`, skips while `e.gr.suppressOrigination()` holds, copies `e.cfg` under `e.mu`, then passes `cfg.MaxMetric.RouterLSAAlways` to `v6OriginateSelf` or `lsdb.OriginateFromTopology`. `originateSelfLSAsDeferred` coalesces a pass onto `originationNotify` for the maintenance worker and is safe to call under `mu` or from a timer callback. `openInterfaces` calls `initializeState()` (which calls `gr.resumeFromNVS()`), opens interfaces with Hellos held, calls `gr.maybeUnplannedRestart()`, calls `releaseHelloHold()`, and calls `updateLDPSyncMachines()`. `shutdown` stops `bgpls`, `ldpSync`, virtual interfaces, `gr.stop()`, `bfdStopAll()`, `srReplayUnsub`, `spf.Stop()`, `ipsec.Close()`, then cancels `e.ctx` under `spawnMu` and waits `e.wg`.
  → Constraint: `originateSelfLSAs` reads one window-aware helper for both families; `openInterfaces` arms the startup window once per engine lifetime AFTER `e.gr.maybeUnplannedRestart()` and skips when `e.gr.inRestart()` is true; timer expiry calls `originateSelfLSAsDeferred`, never `originateSelfLSAs` inline; `shutdown` stops the startup timer before `cancel`.
- [ ] `internal/plugins/ospf/lsdb/origination.go` - `OriginateFromTopology(router, maxMetric)` -> `OriginateRouter` sets every non-stub link (types 1, 2, 4) to `LSInfinity` (`0xffff`) when `maxMetric` is true while leaving stub (type 3) links at configured cost; `existingSelfBodyUnchanged` compares encoded body bytes so metric restoration bumps `nextOwnSequence` and floods (retried by the 1-second maintenance ticker if `MinLSInterval` defers it).
- [ ] `internal/plugins/ospf/origination_v6.go` - `v6OriginateSelf(router, maxMetric)` -> `v6OriginateRouter` applies `v6MaxLinkMetric` (`0xffff`) to non-stub OSPFv3 Router-LSA links and uses `lsdb.OriginateSelf` with the same body-diff and sequence-bump path.
- [ ] `internal/plugins/ospf/ri.go` - `deriveRICapabilities` sets `caps.StubRouter` when `RouterLSAAlways`, `OnStartupSec > 0`, or `OnShutdownSec > 0`.
- [ ] `internal/plugins/ospf/show_summary.go` - `stubRouterView` reports `always`, `on-startup-seconds`, `on-shutdown-seconds`, and `active` (set from `RouterLSAAlways` alone).
- [ ] `internal/plugins/ospf/clear.go` - `clearProcess` tears down adjacencies (`clearNeighbors`) and triggers SPF; it never calls `openInterfaces`.
- [ ] `internal/plugins/ospf/multi_instance.go` and `internal/plugins/ospf/register_multiaf.go` - `instanceManager.start`, `instanceManager.reconcile`, and `v6EngineSet.bringUpLocked` (called by `v6EngineSet.start` and `v6EngineSet.apply` for newly added AFs) are the sole production callers of `openInterfaces`.
- [ ] `internal/plugins/ospf/gr.go` and `internal/plugins/ospf/gr_restarter.go` - `grManager.inRestart()` returns true after `resumeFromNVS()` (planned GR with active NVS fact) or `maybeUnplannedRestart()` (unplanned GR when `planned-and-unplanned` is enabled), and false on a cold start.
- [ ] `internal/plugins/ospf/register.go`, `internal/plugins/ospf/doctor.go`, `internal/plugins/ospf/config_transaction.go`, `cmd/ze/hub/drain.go`, `pkg/plugin/sdk/sdk_drain.go`, `internal/component/plugin/server/drain.go` - the daemon drain phase (Phases 1–3 of `spec-daemon-drain-before-stop.md`) is already landed at HEAD (`drainCeiling = 60s`, `(*Plugin).OnDrain`, `Server.BeginDrain`). OSPF does not yet register `OnDrain`; `runOSPFEngine` calls `instances.shutdownAll()` and `v6set.shutdownAll()` when `p.Run` returns. `ospfDiagnosticCodes` and `registerOSPFDoctor()` (`register.go`) register the `ospf-config-sanity` post-config doctor check (`checkOSPFConfigSanity` → `ospfConfigDiagnostics` in `doctor.go`), which reuses `parseOSPFConfig` so offline `ze doctor` and daemon `show doctor` report the same config-sanity findings. `OnConfigure` currently returns `tx.Configure(sections)` directly without logging a doctor code when `validateConfigAF` rejects a startup config.
- [ ] `internal/plugins/ospf/rfc7770_ri_test.go`, `internal/plugins/ospf/rfc5709_config_test.go`, `internal/plugins/ospf/origination_v6_test.go`, `internal/plugins/ospf/show_summary_test.go`, `internal/plugins/ospf/doctor_test.go`, `test/ospf/ospf-show.ci` - existing tests covering `max-metric` and `ospf-config-sanity`.
- [ ] `internal/le/interoplab/bgp/checkers.go` - declarative `scenarioOperations` table (`opWaitContains`, `opRequireAbsent`, etc.) used for FRR interop scenarios.

**Behavior to preserve:**
- `always true` originates MaxLinkMetric (`0xffff`) on every non-stub link indefinitely, for both OSPFv2 and OSPFv3.
- `show ospf` JSON keys `always`, `on-startup-seconds`, and `active` keep their names and types (`on-shutdown-seconds` renames to `drain-on-shutdown-seconds` under Q-6 of `spec-daemon-drain-before-stop.md`).
- RI capability bit 2 stays router-wide and set whenever any stub-router mode (`always`, `on-startup > 0`, or `drain-on-shutdown > 0`) is configured (RFC 7770 Section 2.5).
- `TestRICapabilityBitsFromState` (RFC7770-2.4-2), `TestOSPFv6OriginateRouterLSAMaxMetric`, `TestLDPSyncP2PMaxMetricNotWithheld`, and `test/ospf/ospf-show.ci` pass unchanged; `TestOSPFMaxMetricConfig` stays green (parsing `drain-on-shutdown` once the Q-6 rename lands).
- Stub (type 3) links keep their configured cost in every mode (RFC 6987 Section 2).
- No self-LSA is originated while RFC 3623 / RFC 5187 `gr.suppressOrigination()` holds.
- Changing `max-metric` on reload restarts no interface runtime and drops no adjacency.

**Behavior to change:**
- `parseOSPFConfig` inherits `cfg.MaxMetric` unconditionally into `cfg.V6` and `cfg.V6Extra` so top-level `max-metric` reaches OSPFv3 engines as well as OSPFv2 engines.
- `on-startup N` (N > 0) opens a one-shot N-second startup window per engine in `openInterfaces` (after `gr.maybeUnplannedRestart()`) on every non-GR start (base engine at boot, reload-added OSPFv2 instance, reload-added OSPFv3 AF), originating `0xffff` on non-stub links until N seconds elapse, then re-originating with configured costs.
- A GR restart (`gr.inRestart() == true`), `clear ospf process all`, and a live config reload of `on-startup` on an already-running engine do NOT open or re-arm the startup window.
- `show ospf` reports `stub-router.active: true` while the startup window is open and adds `stub-router.on-startup-remaining-seconds` (omitted when 0 / outside the window).
- If `spec-daemon-drain-before-stop.md` Phase 5 has not yet landed, perform its Q-6 rename (`max-metric router-lsa on-shutdown` deleted; `ospf drain-on-shutdown` added directly under `container ospf`, `show ospf` key `drain-on-shutdown-seconds`, `ri.go` updated, published YANG fixtures updated) and refuse `ospf drain-on-shutdown > 0` in `validateConfigAF` with an `opererr` error naming `ospf drain-on-shutdown`, why, and the fix (`set ospf drain-on-shutdown 0` or delete it), until Phase 5 wires OSPF's `OnDrain` and removes the refusal. Register `doctor-ospf-drain-on-shutdown-unsupported` in `ospfDiagnosticCodes` and `registerOSPFDoctor` (`register.go`), log an `ERROR` line under that code and fix in `OnConfigure` when startup rejects `drain-on-shutdown > 0`, and emit an error finding with the same code and `Fix` from `ospfConfigDiagnostics` (`doctor.go`).

## Existing Mechanisms (MANDATORY)

Each row below was verified at its producer on 2026-11-26.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| OSPFv2 Router-LSA origination with a max-metric flag | `internal/plugins/ospf/lsdb/origination.go`, `(*LSDB).OriginateFromTopology(router, maxMetric)` | Regenerates each active area's Router-LSA (setting non-stub links to `0xffff` when `maxMetric` is true, keeping stub costs) and Network-LSAs; body-diffs and bumps sequence on change | Does not modify; extends caller in `originateSelfLSAs` to pass the window-aware flag |
| OSPFv3 self-LSA origination with a max-metric flag | `internal/plugins/ospf/origination_v6.go`, `(*engine).v6OriginateSelf(router, maxMetric)` | Regenerates OSPFv3 Link, Router, Network, and Intra-Area-Prefix LSAs; applies `v6MaxLinkMetric` (`0xffff`) to non-stub Router-LSA links when `maxMetric` is true | Does not modify; extends caller in `originateSelfLSAs` and config inheritance in `parseOSPFConfig` |
| Self-LSA chokepoint for both families | `internal/plugins/ospf/instance.go`, `(*engine).originateSelfLSAs` | Checks `ctx.Err()` and `gr.suppressOrigination()`, snapshots `cfg` under `mu`, passes `cfg.MaxMetric.RouterLSAAlways` to `v6OriginateSelf` or `OriginateFromTopology` | Extends: replaces both `cfg.MaxMetric.RouterLSAAlways` arguments with the single window-aware flag helper |
| Deferred, coalesced origination on the maintenance worker | `internal/plugins/ospf/instance.go`, `(*engine).originateSelfLSAsDeferred` and `startNeighborRetransmitLoop` | Queues a non-blocking notification onto `originationNotify` (safe under `mu` or from a timer); maintenance ticker retries changes deferred by `MinLSInterval` | Extends: timer expiry callback updates window state and calls `originateSelfLSAsDeferred` |
| Engine teardown | `internal/plugins/ospf/instance.go`, `(*engine).shutdown` | Stops `bgpls`, `ldpSync`, virtual interfaces, `gr.stop()`, `bfdStopAll()`, `srReplayUnsub`, `spf.Stop()`, `ipsec.Close()`, then cancels `ctx` under `spawnMu` and joins `wg` | Extends: stops the startup-window timer alongside `ldpSync.stop()` / `gr.stop()`, before `cancel` |
| Per-engine start & GR ordering | `internal/plugins/ospf/instance.go`, `(*engine).openInterfaces`; `gr_restarter.go`, `resumeFromNVS`, `maybeUnplannedRestart`; `gr.go`, `inRestart` | Calls `initializeState()` (`gr.resumeFromNVS()`), opens interfaces with Hellos held, runs `gr.maybeUnplannedRestart()`, releases Hellos, updates LDP-sync | Extends: arms the startup window once per engine lifetime right after `gr.maybeUnplannedRestart()`, skipping when `gr.inRestart()` is true |
| Multi-instance & multi-AF engine start/reload | `internal/plugins/ospf/multi_instance.go`, `(*instanceManager).start`, `reconcile`; `register_multiaf.go`, `(*v6EngineSet).start`, `apply`, `bringUpLocked` | Starts base and non-zero OSPFv2 engines and per-AF OSPFv3 engines by calling `openInterfaces` once on each newly created engine | Does not modify; all start and reload-add paths inherit the startup window automatically through `openInterfaces` |
| Process reset (`clear ospf process all`) | `internal/plugins/ospf/clear.go`, `(*engine).clearProcess` | Resets neighbor table (`clearNeighbors`) and triggers SPF without calling `openInterfaces` | Does not modify; naturally preserves Q-2 rule that `clear ospf process all` does not reopen the startup window |
| Router-wide config inheritance into OSPFv3 AFs | `internal/plugins/ospf/config.go`, `parseOSPFConfig` (lines 689–741) | Inherits `RouterID`, `RouterInformation`, `GracefulRestart`, `FastReroute`, and `ReferenceBandwidth` into `cfg.V6` and `cfg.V6Extra` | Extends: inherits `cfg.MaxMetric` unconditionally into `cfg.V6` and `cfg.V6Extra` beside `ReferenceBandwidth` |
| Max-metric config parse & OSPF semantic check | `internal/plugins/ospf/config.go`, `parseMaxMetric`, `maxMetricConfig`, `validateConfigAF`; `config_transaction.go`, `parseOSPFTransactionSections` | Parses `max-metric/router-lsa`; `validateConfigAF` runs all per-family semantic checks on `ze config validate`, commit/reload verify, and `OnConfigure` | Extends: performs Q-6 rename to top-level `drain-on-shutdown` (if not yet landed) and adds the interim `validateConfigAF` `opererr` refusal wrapping `ErrDrainOnShutdownUnsupported` for `drain-on-shutdown > 0` |
| OSPF config-sanity doctor check & diagnostic codes | `internal/plugins/ospf/doctor.go`, `checkOSPFConfigSanity`, `ospfConfigDiagnostics`; `internal/plugins/ospf/register.go`, `ospfDiagnosticCodes`, `registerOSPFDoctor`, `p.OnConfigure` | Registers `ospf-config-sanity` (`DoctorPhasePostConfig`) reusing `parseOSPFConfig` to report OSPF config errors in `ze doctor` / `show doctor`, and registers OSPF `diagnostic.CodeMeta` entries | Extends: registers `codeOSPFDrainOnShutdownUnsupported` (`doctor-ospf-drain-on-shutdown-unsupported`), emits an error finding with `Fix` from `ospfConfigDiagnostics`, and logs an `ERROR` line under that same code and fix in `OnConfigure` when startup rejects `drain-on-shutdown > 0` |
| Daemon pre-stop drain phase (landed Phases 1–3) | `cmd/ze/hub/drain.go`, `stopDrainDeadline`, `stopSignals`; `pkg/plugin/sdk/sdk_drain.go`, `(*Plugin).OnDrain`, `drainWorker`; `internal/component/plugin/server/drain.go`, `BeginDrain` | Already landed at HEAD from `spec-daemon-drain-before-stop.md` Phases 1–3: 60 s ceiling, parallel `drain` RPC fan-out to plugins that register `OnDrain` + `DrainBudget` | Does not modify: OSPF joining `OnDrain` is Phase 5 of `spec-daemon-drain-before-stop.md`, which removes this spec's interim refusal |
| Max-metric YANG leaves | `internal/plugins/ospf/yang/ze-ospf-conf.yang`, `container max-metric / container router-lsa` | Declares `always`, `on-startup`, and `on-shutdown` (0..86400 s) with stale `ze:help` promising unbuilt windows | Extends / replaces: rewrites `router-lsa` and `on-startup` docs/help; deletes `on-shutdown` and adds top-level `ospf drain-on-shutdown` (Q-6, if not yet landed) with refusal docs/help |
| RI stub-router capability bit | `internal/plugins/ospf/ri.go`, `(*engine).deriveRICapabilities` | Sets RFC 7770 bit 2 router-wide when `RouterLSAAlways`, `OnStartupSec > 0`, or `OnShutdownSec > 0` | Extends minimally: preserves bit-2 derivation; switches `OnShutdownSec` read to renamed `DrainOnShutdownSec` if Q-6 rename lands here |
| `show ospf` stub-router view | `internal/plugins/ospf/show_summary.go`, `(*engine).processSummary`, `stubRouterView` | Reports `always`, `on-startup-seconds`, `on-shutdown-seconds`, and `active` (from `RouterLSAAlways` alone) | Extends: reads `active` and new `on-startup-remaining-seconds` from the window-aware helper; renames `on-shutdown-seconds` to `drain-on-shutdown-seconds` on Q-6 rename |
| Injectable engine timer seam | `internal/plugins/ospf/ldp_sync.go`, `ldpSyncTimer`, `ldpSyncManager.afterFunc`, `now`, `epoch` | Deterministic unit testing of timers via `afterFunc` (default `time.AfterFunc`), `now` (default `time.Now`), and stale-callback `epoch` | Extends by reusing the exact same pattern (and `ldpSyncTimer` interface) for the startup-window timer |
| Declarative interop op table | `internal/le/interoplab/bgp/checkers.go`, `scenarioOperations` (`opWaitContains`, `opRequireAbsent`) | Runs named Docker interop scenarios declaratively against FRR without custom Go boilerplate | Extends: adds one `ospf-max-metric-startup-frr` entry to `scenarioOperations` |

## Design documents

Derived by `./le spec design documents spec plan/release/fixes/spec-ospf-max-metric-startup-shutdown-windows.md` on 2026-11-26 (20 design pages, 17 decision rows).

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A backend that cannot apply a config exactly refuses it at verify. It never approximates" | agrees | `on-startup` is implemented in full for OSPFv2 and OSPFv3, and `ospf drain-on-shutdown > 0` is refused at verify (`validateConfigAF`) with an `opererr` naming the leaf, reason, and fix until `spec-daemon-drain-before-stop.md` Phase 5 wires OSPF `OnDrain`. |
| `docs/architecture/ospf/ospf-4-component-config.md` | Decisions | `// Design:` and anchor of `instance.go`, `config.go`, `ze-ospf-conf.yang`, `rfc5709_config_test.go` | "OSPF is a self-contained plugin under `internal/plugins/ospf`, not a central component." | agrees | All startup-window state, config inheritance into `V6`/`V6Extra` (matching `reference-bandwidth`), `show ospf` fields, and `validateConfigAF` refusal stay inside `internal/plugins/ospf`. |
| `docs/architecture/ospf/ospf-11-stub-nssa.md` | Decisions | anchored to `instance.go` | - | silent | Covers stub/NSSA area Hello options, Type 7 origination, translator election, and Type 7 to Type 5 translation; says nothing about RFC 6987 max-metric router-LSA timed windows or `drain-on-shutdown`. |
| `docs/architecture/ospf/ospf-5-interface-ism.md` | Constraints on callers | anchored to `instance.go` | "`interfaceGlobalParamsChanged` reads the config OUTSIDE the interface's own block, and there the router id and the area type are the whole set, because they are what the runtime stamps into a packet" | agrees | Reloading `max-metric` or `on-startup` does not touch `interfaceGlobalParamsChanged` or `interfaceParamsEqual`, so no interface runtime restarts and no adjacency drops. |
| `docs/architecture/ospf/ospf-7-lsdb-flooding.md` | Constraints on callers | anchored to `instance.go` | "Neighbor and interface callbacks MUST enqueue self-LSA origination. Physical and virtual interfaces share one capacity-one notification channel." | agrees | Startup-window timer expiry clears window state and calls `originateSelfLSAsDeferred` (never inline origination); `shutdown` stops the timer before `spawnMu`/context cancellation; the 1-second maintenance ticker retries if `MinLSInterval` defers re-origination. |
| `docs/architecture/ospf/ospf-ext-16-ipsec-auth.md` | Decisions | anchored to `instance.go`, `checkers.go` | - | silent | Covers RFC 4552 manual IPsec AH/ESP SA and policy management for OSPFv3; says nothing about RFC 6987 max-metric startup windows or `drain-on-shutdown`. |
| `docs/architecture/ospf/ospf-ext-7-virtual-links.md` | Decisions | anchored to `instance.go` | - | silent | Covers virtual-link demux, transit SPF endpoint resolution, and Type-4 virtual-link records; says nothing about RFC 6987 timed windows (existing `OriginateRouter` already applies `MaxMetric` to Type-4 virtual links). |
| `docs/architecture/ospf/ospf-ext-9-graceful-restart.md` | Decisions | anchored to `instance.go` | "Self-LSA origination is gated at the shared origination chokepoint, which covers both address families." | agrees | `originateSelfLSAs` keeps `gr.suppressOrigination()` ahead of max-metric origination, and `openInterfaces` checks `e.gr.inRestart()` after `e.gr.maybeUnplannedRestart()` so a planned or unplanned GR restart (which preserves the FIB) skips arming the startup window (owner Q-2). |
| `docs/architecture/wire/ospf.md` | LSDB, origination, and flooding | anchored to `instance.go` | "RFC 6987 max-metric mode sets non-stub Router-LSA links to `0xffff` while preserving stub-network metrics." | agrees | Reuses `OriginateFromTopology` and `v6OriginateSelf` untouched; the startup window drives their existing `maxMetric` boolean so non-stub links carry `0xffff` while stub links keep their configured metrics. |
| `ai/doors/routing-protocols.md` | RFC and interop obligations | door covering `internal/plugins/ospf/*` | "**A wire-visible change owes an interop scenario.** OSPF, OSPFv3, IS-IS, LDP and BFD scenarios against FRR and BIRD live in `test/interop/scenarios/`, named for the protocol, the case and the peer." | agrees | Adds `test/interop/scenarios/ospf-max-metric-startup-frr/` against FRR with both discrimination reverts recorded, quotes RFC 6987 Sections 1 and 2 above the new functions, and inherits `MaxMetric` into `V6`/`V6Extra` for OSPFv2/OSPFv3 parity. |
| `docs/architecture/ospf/ospf-13-cli-diag-interop.md` | Traps | `// Design:` of `show_summary.go`; a page this spec modifies | "**`max-metric router-lsa on-startup` and `on-shutdown` are parsed and armed nowhere.** Origination reads the always form only, so the summary `stub-router.active` field is faithful. The timed windows are an unimplemented engine feature." | departs | This spec replaces that behavior: `on-startup` arms a timed stub-router window per non-GR engine start, `stub-router.active` and `on-startup-remaining-seconds` reflect the active window, and `on-shutdown` is replaced by `ospf drain-on-shutdown` (refused above 0 until `spec-daemon-drain-before-stop.md` Phase 5 wires OSPF `OnDrain`). This spec rewrites that bullet in `ospf-13-cli-diag-interop.md` in Phase 2. |
| `docs/architecture/core-design.md` | 1. System Architecture | anchored to `config.go` | "registers `ConfigRoots ["ospf"]`, embeds `ze-ospf-conf.yang`, and runs the SDK lifecycle through `runOSPFEngine`." | agrees | All startup-window state, config validation, and `show ospf` updates stay inside the OSPF edge plugin with no coupling to core components. |
| `docs/architecture/config/syntax.md` | ospf | anchored to `ze-ospf-conf.yang` | "Top-level native OSPFv2 configuration. The `ospf` container is a YANG config root and auto-loads the OSPF edge plugin." | agrees | `max-metric/router-lsa/on-startup` stays under the `ospf` YANG root and `drain-on-shutdown` sits directly under `container ospf`, parsed via the existing YANG-driven config loader. |
| `docs/architecture/ospf/ospf-ext-3-router-information.md` | Traps | `// Design:` and anchor of `ri.go` | "Graceful restart and stub router stay router-wide." | agrees | `deriveRICapabilities` keeps RFC 7770 bit 2 router-wide across every flooding scope whenever any stub-router mode (`always`, `on-startup`, or `drain-on-shutdown`) is configured. |
| `docs/architecture/api/commands.md` | Overview | anchored to `register.go` | "Ze uses verb-first command paths with JSON or text encoding." | agrees | Adds no new command path; `show ospf` retains its verb-first path and JSON/text rendering while adding `on-startup-remaining-seconds` and renaming `on-shutdown-seconds` to `drain-on-shutdown-seconds`. |
| `docs/architecture/command-ownership.md` | The Folder Test | anchored to `register.go` | "**Delete test:** delete a plugin folder, run codegen, and every one of its features disappears. Every other plugin and the core keep working. No dangling references, no broken builds." | agrees | All startup-window state, `validateConfigAF` refusal, `doctor-ospf-drain-on-shutdown-unsupported` registration, and `ospf-config-sanity` checks stay inside `internal/plugins/ospf`, preserving both folder tests. |
| `docs/architecture/ospf/bfd-client.md` | Decisions | anchored to `register.go` | - | silent | Covers BFD single-hop client integration for OSPF adjacency failure detection; says nothing about RFC 6987 max-metric startup windows, `ospf drain-on-shutdown`, or `doctor-ospf-drain-on-shutdown-unsupported`. |
| `docs/architecture/overview.md` | Non-Goals | anchored to `register.go` | "OSPF and IS-IS are self-contained plugins under `internal/plugins/ospf` and `internal/plugins/isis`. Each plugin owns its protocol engine and lifecycle." | agrees | Keeps the startup-window timer, config inheritance, `show ospf` fields, `OnConfigure` error logging, and doctor check inside `internal/plugins/ospf`, and replaces `on-shutdown` with `ospf drain-on-shutdown` without a pre-release compatibility shim. |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | `// Design:` and anchor of `checkers.go`; a page this spec modifies | "A scenario directory is NAMED and carries no numeric prefix. The name is the scenario's identity: `Discover` matches it exactly, `./le test integration` takes it as a scenario selector, and specs, journal rows and code comments cite it." | agrees | Creates `test/interop/scenarios/ospf-max-metric-startup-frr/` (named, no numeric prefix, declarative config files only), registers its op sequence in `scenarioOperations` (`checkers.go`), and updates `interop.md` with the scenario entry. |
| `ai/doors/le-tooling.md` | Boundaries | door covering `checkers.go`, `test/ospf/*.ci`, `internal/le/site/testdata/*` | "**A `.ci` that needs a Linux kernel feature carries `option=needs-linux`** and runs in the QEMU VM (`ai/rules/platform-linux.md`)." | agrees | `test/ospf/ospf-max-metric-startup.ci` carries `option=needs-linux:caps=net-admin` (raw proto-89 socket in a per-test netns, like `ospf-show.ci`); unit-test deliverables use scoped `./le test unit pkg` commands. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config.go` | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | OSPF max-metric startup windows operate purely in the OSPF control plane (Router-LSA metrics) and introduce no kernel-only data-plane programming or VPP mismatch. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config.go` | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Does not alter data-plane mismatch verification; OSPF's `drain-on-shutdown` refusal is a protocol-plugin semantic check in `validateConfigAF`, unrelated to kernel/VPP mismatch. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config.go` | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Touches no firewall verifier or `ze:backend` annotation; `ze-ospf-conf.yang` adds no `ze:backend` annotation. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config.go` | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Introduces no data-plane selector, backend registration, or `ze:backend` value. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config.go` | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Touches OSPF Router-LSA metrics only; makes no change to BGP next-hop rewriting or export filter chains. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config.go` | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Touches no BGP policy chain or `policychain.Sets` next-hop configuration. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config.go` | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Touches no BGP engine step, `policychain.Chain`, or `pkg/plugin/rpc.FilterInsertion`. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config.go` | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Touches no BGP export chain folding or UPDATE raw override provenance. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config.go` | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Touches no BGP policy `modify` rules or import/export next-hop validation. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config.go` | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Touches no BGP forwarding or locally originated route next-hop policy. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config.go` | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Adds no filter-chain member and does not touch the implied-filter seam or `\| implied` rendering. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config.go`, `register.go` | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | `validateConfigAF` refuses `ospf drain-on-shutdown > 0` at commit, reload, and `ze config validate` (keeping the running services intact on a refused reload); at `ze start`, `LoadConfig` does not exit over it, `OnConfigure` (`register.go`) logs an `ERROR` line under `doctor-ospf-drain-on-shutdown-unsupported` with its fix and leaves only OSPF unstarted while Ze runs its other services, and `ze doctor` (`ospf-config-sanity` / `ospfConfigDiagnostics` in `doctor.go`) reports an error finding under the exact same `doctor-ospf-drain-on-shutdown-unsupported` code with its fix. |
| `docs/architecture/core-design.md` | Design decisions | anchored to `config.go`, `register.go` | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | OSPF max-metric startup windows carry no kernel-vs-VPP data-plane mismatch and leave the VPP/kernel start and commit/reload verdicts untouched. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to `register.go` | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Adds no command path or selector subcommand; `show ospf` continues to route its `stub-router` payload through the shared pipe path. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to `register.go` | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Declares no custom pipe override; `show ospf` uses only the generic pipe operators without changing their data formats. |
| `docs/architecture/api/commands.md` | Design decisions | anchored to `register.go` | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Introduces no pipe operation and does not alter how pipes operate on `show ospf` answers. |
| `docs/architecture/testing/interop.md` | Design decisions | `// Design:` and anchor of `checkers.go`; modified page | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | `ospf-max-metric-startup-frr` runs plain IPv4 OSPF over Docker veth like `ospf-p2p-frr` and requires no unrelated kernel capabilities such as SRv6 or IPsec. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `ospf { max-metric { router-lsa { always <bool>; on-startup <0..86400>; } } drain-on-shutdown <seconds>; }` in the daemon config (with `ospf drain-on-shutdown` replacing `max-metric router-lsa on-shutdown` per `spec-daemon-drain-before-stop.md` Q-6), delivered to the OSPF plugin as a JSON config section through `verifyOSPFConfigSections` (`InProcessConfigVerifier`), `OnConfigVerify`, and `OnConfigure` / `OnConfigApply`.
- Time: each engine's start (`openInterfaces`, after `gr.maybeUnplannedRestart()`) and a one-shot timer expiry N seconds later.

### Transformation Path
1. `parseOSPFConfig` → `parseMaxMetric` stores `RouterLSAAlways` and `OnStartupSec` in `maxMetricConfig`, parses top-level `drain-on-shutdown` into `DrainOnShutdownSec`, and unconditionally inherits `cfg.MaxMetric` into `cfg.V6` and `cfg.V6Extra` (matching `ReferenceBandwidth`, because `grouping ospf-af-topology` carries no `max-metric` block).
2. `validateConfigAF` refuses `DrainOnShutdownSec > 0` with an `opererr` wrapping `ErrDrainOnShutdownUnsupported`, naming `ospf drain-on-shutdown`, explaining OSPF has not yet joined the daemon drain phase, and giving the fix (`set ospf drain-on-shutdown 0` or delete it), until `spec-daemon-drain-before-stop.md` Phase 5 wires OSPF `OnDrain` and removes the refusal. On `ze start`, `OnConfigure` (`register.go`) catches `errors.Is(err, ErrDrainOnShutdownUnsupported)` and logs an `ERROR` line carrying `"code", codeOSPFDrainOnShutdownUnsupported` and the fix before returning the error (so only OSPF stays unstarted while `ze` runs its other services), and `checkOSPFConfigSanity` → `ospfConfigDiagnostics` (`doctor.go`) emits an error finding under the same `doctor-ospf-drain-on-shutdown-unsupported` code with `Fix` populated for both offline `ze doctor` and daemon `show doctor`.
3. `openInterfaces` runs `initializeState()` (which runs `gr.resumeFromNVS()`), opens enrolled interfaces with Hellos held, runs `gr.maybeUnplannedRestart()`, and then calls `armMaxMetricStartup()` once per engine lifetime. If `e.gr.inRestart()` is true (planned or unplanned GR preserving the FIB) or `cfg.MaxMetric.OnStartupSec == 0`, no window opens. Otherwise it records the window end (`now + OnStartupSec`), increments the window epoch, and arms the one-shot timer via the injectable `afterFunc` seam.
4. `originateSelfLSAs` asks one engine helper (`maxMetricActiveLocked(now)`) whether max-metric is in force (`cfg.MaxMetric.RouterLSAAlways` or `now < startupUntil`) and passes that boolean to `OriginateFromTopology` (OSPFv2) or `v6OriginateSelf` (OSPFv3).
5. When the one-shot timer fires, its callback locks `e.mu`, checks the epoch and `e.ctx.Err()`, clears the window state, unlocks, and calls `originateSelfLSAsDeferred`. The maintenance worker runs `originateSelfLSAs` with configured link costs (retrying on the 1-second ticker if `MinLSInterval` defers it), and LSDB flooding carries the new Router-LSA instance to neighbours.
6. `shutdown` stops the startup-window timer and bumps the epoch before `spawnMu` / `e.cancel()`, so no late timer callback can trigger origination after teardown.
7. `processSummary` calls the same window helper under `e.mu` to populate `stub-router.active` and `stub-router.on-startup-remaining-seconds` (whole seconds remaining, ceiling or truncated down to at least 1 while open, omitted when inactive).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Daemon config → OSPF plugin | SDK config section JSON (`max-metric/router-lsa`, top-level `drain-on-shutdown`) | No |
| OSPF plugin → wire | Router-LSA in LS Update, non-stub link metric `0xffff` during the window, configured cost after | No |
| OSPF plugin → CLI | `show ospf` JSON `stub-router` object (`active`, `on-startup-remaining-seconds`, `drain-on-shutdown-seconds`) | No |

### Integration Points
- `parseOSPFConfig` (`config.go`) - inherits `cfg.MaxMetric` unconditionally into `cfg.V6` and `cfg.V6Extra`.
- `validateConfigAF` (`config.go`) - interim `opererr` refusal wrapping `ErrDrainOnShutdownUnsupported` for `ospf drain-on-shutdown > 0`.
- `p.OnConfigure`, `ospfDiagnosticCodes`, `registerOSPFDoctor` (`register.go`) and `ospfConfigDiagnostics` (`doctor.go`) - registers `doctor-ospf-drain-on-shutdown-unsupported`, logs an `ERROR` line under that code and fix when startup `OnConfigure` rejects `drain-on-shutdown > 0`, and emits an error finding with `Fix` in `ospf-config-sanity`.
- `openInterfaces` (`instance.go`) - arms the startup window once per engine lifetime after `e.gr.maybeUnplannedRestart()` when `!e.gr.inRestart()`.
- `originateSelfLSAs` (`instance.go`) - replaces both `cfg.MaxMetric.RouterLSAAlways` arguments with the window-aware helper.
- `shutdown` (`instance.go`) - stops the startup-window timer alongside `ldpSync.stop()` and `gr.stop()`, before `cancel`.
- `processSummary` (`show_summary.go`) - populates `active` and `on-startup-remaining-seconds` from the same window-aware helper.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (design) | The window-aware flag enters through the existing `maxMetric` parameter of `OriginateFromTopology` and `v6OriginateSelf`; no encoder or LSDB edit |
| No unintended coupling (components stay isolated) | Yes (design) | All product changes stay inside `internal/plugins/ospf`; daemon drain plumbing already landed in `spec-daemon-drain-before-stop.md` Phases 1–3 and OSPF joins it in Phase 5 |
| No duplicated functionality (extends existing, does not recreate) | Yes (design) | Reuses `OriginateFromTopology`, `v6OriginateSelf`, `originateSelfLSAsDeferred`, `gr.inRestart()`, and the `ldpSyncTimer` / `afterFunc` / `epoch` pattern from `ldp_sync.go` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No wire buffer handling changes |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes (design) | Nothing added to any core or shared package; all code lives inside `internal/plugins/ospf` |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes (design) | No core switch, validator table, or completion table changes: `on-startup` and `drain-on-shutdown` derive completion/help from `ze-ospf-conf.yang`; `show ospf` derives from the existing `ze-ospf:show-summary` RPC; `ospf-max-metric-startup-frr` adds one entry to the interop scenario registry `scenarioOperations` (`internal/le/interoplab/bgp/checkers.go`), which `Discover` reads |

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
| A-1 | `openInterfaces` runs once per engine lifetime and is reached by every start path (base engine, reload-added OSPFv2 instance, all OSPFv3 AF engines) | Verified at HEAD 2026-11-26: only production callers are `instanceManager.start`, `instanceManager.reconcile`, and `v6EngineSet.bringUpLocked` | A path skipping it starts no window; a double call re-arms it | Wiring tests per path plus `once` guard inside `armMaxMetricStartup` | unvalidated |
| A-2 | A Router-LSA whose only change is link metric gets a new sequence number and floods (retried on the 1-second ticker if `MinLSInterval` defers it) | Verified at HEAD in `lsdb/origination.go` (`OriginateRouter`, `existingSelfBodyUnchanged`) and `origination_v6.go` (`OriginateSelf`) | Window close would leave neighbours at `0xffff` until LSRefreshTime | `TestOSPFMaxMetricStartupWindowCloses` and `TestOSPFv6MaxMetricStartupWindow` assert configured cost and sequence bump after expiry | unvalidated |
| A-3 | Changing `on-startup` on a running engine or running `clear ospf process all` starts no window; a GR restart that preserved the FIB opens no window; an added instance/AF opens its own window | Settled by owner Q-2 decision (2026-10-13): "clear and instance follow FRR" and GR keeps FIB so opens no window | Implementation diverges from owner ruling | `TestOSPFMaxMetricStartupReloadDoesNotArm`, `TestOSPFMaxMetricStartupClearProcessDoesNotReopen`, `TestOSPFMaxMetricStartupSkippedDuringGR`, `TestOSPFMaxMetricStartupWindowNewInstanceAtReload` | unvalidated |
| A-4 | FRR's ospfd reports `65535` as the link metric for Ze's point-to-point link in `show ip ospf database router <ze-router-id>` | FRR prints TOS 0 Metric per link in router-LSA output | Checker must read another field or JSON output | First run of `ospf-max-metric-startup-frr` | unvalidated |
| A-5 | An FRR adjacency to Ze over a point-to-point link reaches Full well inside 60 seconds in the interop lab | Existing `ospf-p2p-frr` reaches Full well within its 90-second budget | Window could close before FRR observes `0xffff` | Scenario uses `on-startup 120` and asserts both phases; timing recorded on first run | unvalidated |
| A-6 | Daemon-wide pre-stop drain phase exists at HEAD (`spec-daemon-drain-before-stop.md` Phases 1–3) while OSPF joining it remains Phase 5 of that spec | Verified at HEAD 2026-11-26: `cmd/ze/hub/drain.go`, `pkg/plugin/sdk/sdk_drain.go`, `internal/component/plugin/server/drain.go` are landed; `internal/plugins/ospf/register.go` has no `OnDrain` yet | If Phase 5 lands before this spec, the Q-6 rename and drain handler are already present and Phase 4 of this spec is skipped | Check `internal/plugins/ospf/register.go` and `ze-ospf-conf.yang` at implementation start | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Timer fires after `shutdown` and touches torn-down engine state | Race detector or panic in shutdown test | `shutdown` stops the timer and bumps `epoch` before `cancel`; callback checks `epoch` and `ctx.Err()` and only calls `originateSelfLSAsDeferred`; proven by `TestOSPFMaxMetricStartupTimerStoppedOnShutdown` under `-race` |
| R-2 | v4 and v6 engines disagree because `cfg.MaxMetric` was not inherited into `V6`/`V6Extra` or one call site still reads `RouterLSAAlways` | v6 test sees configured cost inside the window | `parseOSPFConfig` unconditionally inherits `cfg.MaxMetric` into `V6` and `V6Extra`; both call sites in `originateSelfLSAs` read one helper; grep `RouterLSAAlways` outside `config.go`, `ri.go`, `show_summary.go`, and `max_metric_window.go` returns nothing |
| R-3 | `armMaxMetricStartup` runs before `gr.maybeUnplannedRestart()`, so an unplanned GR restart sees `inRestart() == false` and wrongly opens a startup window | `TestOSPFMaxMetricStartupSkippedDuringGR` fails on unplanned GR case | Call `armMaxMetricStartup` in `openInterfaces` strictly AFTER `e.gr.maybeUnplannedRestart()` (which runs after `initializeState` / `gr.resumeFromNVS()`); test both planned and unplanned GR paths |
| R-4 | Interim refusal of `ospf drain-on-shutdown > 0` rejects a stored config carrying it (and the Q-6 rename removes `max-metric router-lsa on-shutdown`) | Commit / `ze config validate` returns the refusal | Pre-release policy: no compatibility shim (`spec-daemon-drain-before-stop.md` Q-6); refusal is an `opererr` naming the leaf, reason, and fix (`set ospf drain-on-shutdown 0` or delete it); `ze start` does not exit over a single plugin's `OnConfigure` refusal |
| R-5 | `MinLSInterval` (default 5 s) delays Router-LSA re-origination when the startup window closes less than 5 s after the last origination | Short-window unit test sees `0xffff` immediately after timer fire | `originateSelfLSAsDeferred` triggers an immediate attempt and the 1-second maintenance ticker retries until `MinLSInterval` elapses; unit tests configure `MinLSIntervalMS: 0` or advance the LSDB clock past `MinLSInterval` when verifying immediate close |
| R-6 | Deadlock if the window helper acquires `e.mu` while a caller already holds `e.mu` | Test hang | Define `maxMetricActiveLocked(now)` and `startupRemainingLocked(now)` requiring `e.mu` held; both `originateSelfLSAs` and `processSummary` already hold `e.mu` when snapshotting `e.cfg` |
| R-7 | Real-time timers make unit tests slow or flaky | Unit test wall time above 1 s | Injectable `afterFunc` seam and `now` clock (copying `ldp_sync.go`); zero real sleeps in unit tests |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A router with `on-startup` set either never drains at boot (today's bug) or stays stuck at `0xffff` after the window closes; a GR restart that preserved the FIB could wrongly drain transit if `inRestart()` is checked too early; a config with `drain-on-shutdown > 0` is refused at commit/reload/validate |
| How is it reverted? | Single commit revert; no persistent state written, no wire format change |
| Who else touches this path? | `originateSelfLSAs` is shared with RFC 3623 GR suppression and LDP-IGP sync; `spec-daemon-drain-before-stop.md` Phase 5 later removes the interim `drain-on-shutdown` refusal and wires OSPF `OnDrain` |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config `on-startup 120` at daemon boot (base OSPFv2 engine via `instanceManager.start`) | → | `openInterfaces` arms the window; `originateSelfLSAs` reads `maxMetricActiveLocked` | `test/ospf/ospf-max-metric-startup.ci` and `TestOSPFMaxMetricStartupWindowOpensAtStart` |
| Config adds an OSPFv2 instance at reload (`instanceManager.reconcile`) | → | `openInterfaces` arms the window for the new engine only | `TestOSPFMaxMetricStartupWindowNewInstanceAtReload` |
| Top-level `on-startup` with `address-family ipv6` (`parseOSPFConfig` → `v6EngineSet.bringUpLocked`) | → | `parseOSPFConfig` inherits `MaxMetric`; `openInterfaces` arms; `v6OriginateSelf` receives the window flag | `TestOSPFv6MaxMetricInheritedFromTopLevel` and `TestOSPFv6MaxMetricStartupWindow` |
| Planned or unplanned GR restart (`openInterfaces` with `gr.inRestart() == true`) | → | `armMaxMetricStartup` skips arming because FIB is preserved | `TestOSPFMaxMetricStartupSkippedDuringGR` |
| `clear ospf process all` (`clearProcess`) | → | resets neighbors without calling `openInterfaces`; window stays closed | `TestOSPFMaxMetricStartupClearProcessDoesNotReopen` |
| Config `ospf drain-on-shutdown 60` at validate/commit | → | `validateConfigAF` `opererr` refusal | `test/ospf/ospf-drain-on-shutdown-refused.ci` and `TestOSPFDrainOnShutdownRefused` |
| Config `ospf drain-on-shutdown 60` at `ze start` and `ze doctor` | → | `OnConfigure` logs `ERROR` under `doctor-ospf-drain-on-shutdown-unsupported` with fix and leaves only OSPF down; `checkOSPFConfigSanity` / `ospfConfigDiagnostics` reports the same code and `Fix` | `TestOSPFDoctorDrainOnShutdownUnsupported`, `TestOSPFOnConfigureLogsDoctorCodeOnDrainOnShutdown`, and `test/ospf/ospf-drain-on-shutdown-refused.ci` |
| FRR neighbour over a point-to-point link | → | Router-LSA flooded with `0xffff`, then restored to configured cost | interop scenario `ospf-max-metric-startup-frr` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior | Proof |
|-------|-------------------|-------------------|-------|
| AC-1 | `on-startup N` (N > 0), `always false`, non-GR engine has just started | Every Router-LSA Ze originates carries `0xffff` (65535) as the metric of every non-stub link (types 1, 2, 4), and the configured cost on every stub (type 3) link | `TestOSPFMaxMetricStartupWindowOpensAtStart`; interop assertion 2 |
| AC-2 | Same, N seconds after start | Ze re-originates each Router-LSA with the configured costs, a higher sequence number, and floods it; no `0xffff` remains on any link | `TestOSPFMaxMetricStartupWindowCloses`; interop assertion 4 |
| AC-3 | A topology change (neighbour up, interface up) inside the window | The re-originated Router-LSA still carries `0xffff` on non-stub links | `TestOSPFMaxMetricStartupWindowSurvivesReorigination` |
| AC-4 | `on-startup 0` (default) | No window opens; Router-LSAs carry configured costs from the first origination | `TestOSPFMaxMetricStartupZeroNoWindow` |
| AC-5 | `always true` together with `on-startup N` | `0xffff` is advertised before, during, and after the window; window expiry does not restore configured costs | `TestOSPFMaxMetricAlwaysOverridesWindowExpiry` |
| AC-6 | Top-level `on-startup N` with `address-family ipv6` (and any RFC 5838 extra AF) | `parseOSPFConfig` inherits `MaxMetric` into `V6` and `V6Extra`; the OSPFv3 Router-LSA advertises `0xffff` during the window and restores configured link cost on expiry | `TestOSPFv6MaxMetricInheritedFromTopLevel`; `TestOSPFv6MaxMetricStartupWindow` |
| AC-7 | An OSPFv2 instance added by reload (`instanceManager.reconcile`) or an OSPFv3 AF added by reload (`v6EngineSet.apply`) | The new engine opens its own startup window from its own start; existing engines are unaffected | `TestOSPFMaxMetricStartupWindowNewInstanceAtReload` |
| AC-8 | `on-startup` changed from 0 to N on a running engine by reload | No window opens on the running engine; `show ospf` reports the new configured value and `active false` | `TestOSPFMaxMetricStartupReloadDoesNotArm` |
| AC-9 | `clear ospf process all` run after startup (window closed or expired) | Does not reopen the startup window (owner Q-2) | `TestOSPFMaxMetricStartupClearProcessDoesNotReopen` |
| AC-10 | Engine starts in RFC 3623 / RFC 5187 graceful restart (planned via active NVS fact or unplanned via `planned-and-unplanned`, preserving the FIB) | No startup window opens; when GR exits, Router-LSAs carry configured costs (unless `always true`). A cold start with an absent or expired GR fact opens the startup window normally (owner Q-2) | `TestOSPFMaxMetricStartupSkippedDuringGR`; `TestOSPFMaxMetricStartupOpensAfterColdStart` |
| AC-11 | `show ospf` inside and outside the startup window | Inside: `stub-router.active` is true and `stub-router.on-startup-remaining-seconds` is whole seconds left (> 0, <= N). Outside: `active` follows `always` and `on-startup-remaining-seconds` is omitted | `TestOSPFShowSummaryStartupWindow`; `test/ospf/ospf-max-metric-startup.ci` |
| AC-12 | Engine shutdown while the startup window is open | Timer is stopped before `cancel`, a stale timer callback originates nothing, and `-race` is clean | `TestOSPFMaxMetricStartupTimerStoppedOnShutdown` |
| AC-13 | Commit, reload, `ze config validate`, `ze start`, or `ze doctor` with `ospf drain-on-shutdown` > 0 (interim until `spec-daemon-drain-before-stop.md` Phase 5 lands; Q-6 rename performed here if not yet landed) | Commit, reload, and `ze config validate` refuse with an `opererr` wrapping `ErrDrainOnShutdownUnsupported`, naming `ospf drain-on-shutdown`, stating OSPF has not yet joined the daemon shutdown drain phase, and giving the exact fix (`set ospf drain-on-shutdown 0` or delete it). At `ze start`, `ze` keeps running its other services while OSPF stays unstarted and `OnConfigure` logs an `ERROR` line under `doctor-ospf-drain-on-shutdown-unsupported` with the fix; `ze doctor` (`ospf-config-sanity` / `ospfConfigDiagnostics`) reports an error finding under `doctor-ospf-drain-on-shutdown-unsupported` with `Fix` set | `TestOSPFDrainOnShutdownRefused`; `TestOSPFDoctorDrainOnShutdownUnsupported`; `TestOSPFOnConfigureLogsDoctorCodeOnDrainOnShutdown`; `test/ospf/ospf-drain-on-shutdown-refused.ci` |
| AC-14 | `ospf drain-on-shutdown 0` or absent | Accepted with no change | `TestOSPFDrainOnShutdownZeroAccepted` |
| AC-15 | Any stub-router mode configured (`always`, `on-startup > 0`, or `drain-on-shutdown > 0`) | RI capability bit 2 stays router-wide and unchanged (RFC 7770 Section 2.5) | existing `TestRICapabilityBitsFromState` passes unchanged |
| AC-16 | `ze:help` and `description` of `router-lsa`, `on-startup`, and `ospf drain-on-shutdown`, plus `docs/architecture/ospf/ospf-13-cli-diag-interop.md`, `docs/guide/ospf.md`, and `docs/architecture/testing/interop.md` | Accurately describe the startup window (anchored at non-GR engine start, skipped on GR/clear/reload) and the interim `ospf drain-on-shutdown` refusal | Reviewed in `/ze-review` against producers; `./le spec citation anchors` clean |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets `on-startup 120` and starts Ze so transit avoids it while routing converges | config → `parseMaxMetric` (inherited into V6/V6Extra) → `openInterfaces` arms window (if not GR) → `originateSelfLSAs` → Router-LSA `0xffff` → flood → FRR database; expiry → re-origination with configured costs | interop `ospf-max-metric-startup-frr` |
| 2 | Checks whether the router is still in its startup stub-router window | `show ospf` → `processSummary` → `stub-router.active`, `on-startup-remaining-seconds` | `test/ospf/ospf-max-metric-startup.ci` |
| 3 | Attempts to configure `ospf drain-on-shutdown 60` before OSPF joins the daemon drain phase | `ze config validate` / commit / reload → `validateConfigAF` → `opererr` refusal naming leaf, reason, and fix | `test/ospf/ospf-drain-on-shutdown-refused.ci` |

## 🧪 TDD Test Plan

### Unit Tests
**RFC-tagged tests that change: none.** RFC 6987 has no requirement rows (`Enrolment: backlog`) and RFC scope is frozen, so no new test carries an `RFC requirement:` tag and no discrimination record is owed. `TestRICapabilityBitsFromState` (RFC7770-2.4-2, configured with `always`) is not edited and stays green. New implementing functions carry the RFC 6987 Section 1 and Section 2 quotes in comments (`ai/rules/rfc-compliance.md`). Untagged tests that must stay green: `TestOSPFMaxMetricConfig` (updated only for the Q-6 leaf name if this spec performs the rename), `TestOSPFv6OriginateRouterLSAMaxMetric`, `TestLDPSyncP2PMaxMetricNotWithheld`, `test/ospf/ospf-show.ci`.

All timer tests drive the injectable `afterFunc` seam and an injected clock; none sleeps.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFMaxMetricStartupWindowOpensAtStart` | `internal/plugins/ospf/max_metric_window_test.go` | AC-1: first Router-LSA after `openInterfaces` has `0xffff` on type-1 and type-2 links and configured cost on type-3 stub link | |
| `TestOSPFMaxMetricStartupWindowCloses` | same | AC-2: firing the seam timer re-originates; LSDB copy has configured costs and higher sequence number | |
| `TestOSPFMaxMetricStartupWindowSurvivesReorigination` | same | AC-3: re-origination triggered inside the window keeps `0xffff` | |
| `TestOSPFMaxMetricStartupZeroNoWindow` | same | AC-4: `on-startup 0` arms no timer, configured costs from start | |
| `TestOSPFMaxMetricAlwaysOverridesWindowExpiry` | same | AC-5: `always true` + `on-startup N` keeps `0xffff` after timer fires | |
| `TestOSPFv6MaxMetricInheritedFromTopLevel` | same | AC-6: `parseOSPFConfig` inherits top-level `max-metric` into `V6` and `V6Extra` | |
| `TestOSPFv6MaxMetricStartupWindow` | `internal/plugins/ospf/origination_v6_test.go` | AC-6: OSPFv3 engine originates `0xffff` during startup window and restores configured cost on expiry | |
| `TestOSPFMaxMetricStartupWindowNewInstanceAtReload` | `internal/plugins/ospf/multi_instance_test.go` | AC-7: reconcile-created instance opens its own window; existing instance unaffected | |
| `TestOSPFMaxMetricStartupReloadDoesNotArm` | `internal/plugins/ospf/max_metric_window_test.go` | AC-8: reload changing `on-startup` from 0 to N arms no window on running engine | |
| `TestOSPFMaxMetricStartupClearProcessDoesNotReopen` | same | AC-9: `clearProcess()` after window expiry does not reopen the startup window | |
| `TestOSPFMaxMetricStartupSkippedDuringGR` | same | AC-10: planned GR (`resumeFromNVS` active fact) and unplanned GR (`planned-and-unplanned`) open no startup window | |
| `TestOSPFMaxMetricStartupOpensAfterColdStart` | same | AC-10: cold start (no GR fact or expired fact) opens the startup window | |
| `TestOSPFShowSummaryStartupWindow` | `internal/plugins/ospf/show_summary_test.go` | AC-11: `active` and `on-startup-remaining-seconds` inside and after the window | |
| `TestOSPFMaxMetricStartupTimerStoppedOnShutdown` | `internal/plugins/ospf/max_metric_window_test.go` | AC-12: after `shutdown` seam timer reports stopped; late fire (stale epoch) originates nothing; clean under `-race` | |
| `TestOSPFDrainOnShutdownRefused` | same | AC-13: `validateConfig` returns `opererr` wrapping `ErrDrainOnShutdownUnsupported` for `drain-on-shutdown` 1 and 60 (naming leaf, reason, and fix) | |
| `TestOSPFDoctorDrainOnShutdownUnsupported` | `internal/plugins/ospf/doctor_test.go` | AC-13: `ospfConfigDiagnostics` and `checkOSPFConfigSanity` emit `doctor-ospf-drain-on-shutdown-unsupported` (severity error, `Fix` populated) for `drain-on-shutdown > 0`, and stay silent for 0/absent; `diagnostic.Lookup(codeOSPFDrainOnShutdownUnsupported)` succeeds | |
| `TestOSPFOnConfigureLogsDoctorCodeOnDrainOnShutdown` | `internal/plugins/ospf/max_metric_window_test.go` | AC-13: when `OnConfigure` rejects `drain-on-shutdown > 0`, it logs an `ERROR` line carrying `code=doctor-ospf-drain-on-shutdown-unsupported` and the fix before returning the error | |
| `TestOSPFDrainOnShutdownZeroAccepted` | same | AC-14: `drain-on-shutdown 0` and absent accepted | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `on-startup` | 0..86400 s (YANG range, unchanged) | 86400 (and 1, shortest window) | N/A (uint32) | 86401, refused by YANG range |
| `drain-on-shutdown` (interim) | 0 only accepted until `spec-daemon-drain-before-stop.md` Phase 5 lands | 0 | N/A (uint32) | 1, refused by `validateConfigAF` |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-max-metric-startup` | `test/ospf/ospf-max-metric-startup.ci` | `option=needs-linux:caps=net-admin` (per-test netns, like `ospf-show.ci`): daemon boots with `on-startup 300`; `show ospf` reports `"active": true` and `on-startup-remaining-seconds`; `show ospf database router` shows self Router-LSA with metric `65535` on non-stub link | |
| `ospf-drain-on-shutdown-refused` | `test/ospf/ospf-drain-on-shutdown-refused.ci` | `ze config validate` of `ospf { drain-on-shutdown 60 }` fails non-zero with the `opererr` naming `ospf drain-on-shutdown`, reason, and fix; `ze doctor` on the same config reports `doctor-ospf-drain-on-shutdown-unsupported` with the fix | |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-max-metric-startup-frr` | `test/interop/scenarios/ospf-max-metric-startup-frr/` (`ze.conf` with point-to-point interface, cost 10, `on-startup 120`; `frr.conf` from `ospf-p2p-frr`) | FRR ospfd | 1: FRR adjacency Full. 2: FRR's `show ip ospf database router <ze-router-id>` shows `Metric: 65535` for Ze's point-to-point link while stub link keeps 10. 3: `show ospf` on Ze reports `"active": true`. 4: within 180 s the same FRR query no longer contains `65535` and shows the point-to-point link at 10. Declared as one `scenarioOperations` entry in `internal/le/interoplab/bgp/checkers.go` | |

Discrimination (`ai/rules/interop-and-goal-validation.md`): (1) revert the flag source in `originateSelfLSAs` to `RouterLSAAlways`, rebuild the Ze image, run the scenario and record assertion 2 red; restore and record green; (2) leave the expiry callback unarmed (window never closes), rebuild, run and record assertion 4 red; restore and record green. Both results go into closure evidence.

### Goal Validation

| Goal (from Task) | Evidence owed |
|------------------|---------------|
| `on-startup N` drains transit for N seconds after non-GR start, then restores costs | Interop `ospf-max-metric-startup-frr` log: FRR reads `65535` then `10` for Ze's point-to-point link, with both discrimination reds recorded; unit tests cover OSPFv3 inheritance, reload-added instance, `clearProcess`, and GR skip |
| Operator can observe the active window | `test/ospf/ospf-max-metric-startup.ci` asserting `active: true` and `on-startup-remaining-seconds` through real `show ospf` |
| No leaf promises a behavior nothing performs | `test/ospf/ospf-drain-on-shutdown-refused.ci` (negative test through `ze config validate`) and rewritten YANG `ze:help` / `description`, until `spec-daemon-drain-before-stop.md` Phase 5 wires OSPF `OnDrain` |

BIRD is not used: FRR already runs the OSPFv2 point-to-point scenarios and FRR is the vendor whose `max-metric router-lsa on-startup` the leaf mirrors.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/plugins/ospf/instance.go` - `originateSelfLSAs` reads `maxMetricActiveLocked(now)` for both families; `openInterfaces` calls `armMaxMetricStartup()` after `gr.maybeUnplannedRestart()`; `shutdown` stops the startup timer before `cancel`; engine fields for window state and injectable seams.
- `internal/plugins/ospf/show_summary.go` - `active` and new `on-startup-remaining-seconds` (omitempty) from the window helper; rename `on-shutdown-seconds` to `drain-on-shutdown-seconds` (if Q-6 rename lands here first); rewrite stale type comment.
- `internal/plugins/ospf/config.go` - inherit `cfg.MaxMetric` unconditionally into `cfg.V6` and `cfg.V6Extra` in `parseOSPFConfig`; perform Q-6 rename (`maxMetricConfig.OnShutdownSec` removed, top-level `DrainOnShutdownSec` parsed from `drain-on-shutdown`) if not yet landed; add `validateConfigAF` interim `opererr` refusal for `DrainOnShutdownSec > 0`.
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` - rewrite `description` and `ze:help` of `router-lsa` and `on-startup` (non-GR start anchor, GR/clear/reload behavior); delete `max-metric/router-lsa/on-shutdown` and add top-level `leaf drain-on-shutdown` (uint32, units seconds, default 0) under `container ospf` (if Q-6 rename lands here first) documenting the interim refusal.
- `internal/plugins/ospf/ri.go` - switch `deriveRICapabilities` from `mm.OnShutdownSec` to `cfg.DrainOnShutdownSec` (if Q-6 rename lands here first); update comment if needed; bit-2 semantics unchanged.
- `internal/plugins/ospf/doctor.go` - declare `codeOSPFDrainOnShutdownUnsupported = "doctor-ospf-drain-on-shutdown-unsupported"` and emit an error `diagnostic.Diagnostic` with `Fix` in `ospfConfigDiagnostics` when `cfg.DrainOnShutdownSec > 0`.
- `internal/plugins/ospf/register.go` - register `codeOSPFDrainOnShutdownUnsupported` in `ospfDiagnosticCodes` and `registerOSPFDoctor` (`ospf-config-sanity`); log an `ERROR` line under `codeOSPFDrainOnShutdownUnsupported` with its fix in `p.OnConfigure` when `tx.Configure` fails with `ErrDrainOnShutdownUnsupported`.
- `internal/plugins/ospf/doctor_test.go` - add `TestOSPFDoctorDrainOnShutdownUnsupported`.
- `internal/plugins/ospf/multi_instance_test.go` - add `TestOSPFMaxMetricStartupWindowNewInstanceAtReload`.
- `internal/plugins/ospf/origination_v6_test.go` - add `TestOSPFv6MaxMetricStartupWindow`.
- `internal/plugins/ospf/show_summary_test.go` - add `TestOSPFShowSummaryStartupWindow`.
- `internal/plugins/ospf/rfc5709_config_test.go` - update `TestOSPFMaxMetricConfig` to parse `drain-on-shutdown` if Q-6 rename lands here first.
- `internal/le/interoplab/bgp/checkers.go` - add `ospf-max-metric-startup-frr` entry to `scenarioOperations`.
- `internal/le/site/testdata/published-configuration.md` and `internal/le/site/testdata/published-yang-config-tree.json` - update published YANG fixtures if Q-6 rename lands here first.
- `docs/architecture/ospf/ospf-13-cli-diag-interop.md` - replace the "parsed and armed nowhere" Traps bullet.
- `docs/guide/ospf.md` - update `show ospf` paragraph to name `on-startup-remaining-seconds` and add a max-metric subsection documenting `always`, `on-startup` (non-GR start anchor, GR/clear/reload rules, OSPFv2/OSPFv3 inheritance), and `ospf drain-on-shutdown`.
- `docs/architecture/testing/interop.md` - add the `ospf-max-metric-startup-frr` scenario row.
- `plan/journal/unwired-feature.md` - update disposition of the two max-metric rows (2026-09-03 and 2026-09-04) at closure.

## Files to Create
- `internal/plugins/ospf/max_metric_window.go` - startup-window state, `armMaxMetricStartup`, `stopMaxMetricStartup`, `maxMetricActiveLocked`, `startupRemainingLocked` (RFC 6987 Sections 1 and 2 quoted above them).
- `internal/plugins/ospf/max_metric_window_test.go` - unit tests for AC-1..AC-5, AC-6 inheritance, AC-8..AC-10, AC-12..AC-14.
- `test/ospf/ospf-max-metric-startup.ci` - functional test for `on-startup` window and `show ospf`.
- `test/ospf/ospf-drain-on-shutdown-refused.ci` - functional test for `ospf drain-on-shutdown` refusal.
- `test/interop/scenarios/ospf-max-metric-startup-frr/ze.conf`, `frr.conf` - FRR interop scenario.

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang`: `router-lsa` and `on-startup` `description`/`ze:help` rewritten; `max-metric/router-lsa/on-shutdown` replaced by top-level `ospf drain-on-shutdown` (per `spec-daemon-drain-before-stop.md` Q-6) if not yet landed |
| YANG validation constraints | No | `on-startup` range unchanged (`0..86400`); `drain-on-shutdown` keeps uint32 default 0; interim refusal is a behavior gate in `validateConfigAF`, not a narrowed YANG range (owner Q-3) |
| YANG custom validators | No | Refusal lives in the plugin's `validateConfigAF`, reached via `InProcessConfigVerifier` and `OnConfigVerify`, like every other OSPF semantic check |
| CLI commands/flags | No | No new command; `show ospf` adds one JSON key (`on-startup-remaining-seconds`) and renames `on-shutdown-seconds` to `drain-on-shutdown-seconds` on Q-6 rename |
| CLI grammar (keyword before value) | N-A | No CLI command added |
| Editor autocomplete | N-A | Leaves are uint32; automatic from YANG |
| Functional test for new RPC/API | Yes | `test/ospf/ospf-max-metric-startup.ci` covers the `show ospf` keys; `test/ospf/ospf-drain-on-shutdown-refused.ci` covers the validation refusal |
| Pipe completeness | N-A | `show ospf` already routes through the existing pipe path; no new output command |
| Env var registration | N-A | No `environment/` leaf |
| Doctor check for runtime dependencies | N-A | No new file, socket, port, module, sysctl, or binary; an in-process timer only |
| Doctor code per problem class | Yes | `doctor-ospf-drain-on-shutdown-unsupported` (`codeOSPFDrainOnShutdownUnsupported`), registered in `ospfDiagnosticCodes` (`internal/plugins/ospf/register.go`) and `ospf-config-sanity` (`DoctorPhasePostConfig`, `registerOSPFDoctor`); emitted with `Fix` by `ospfConfigDiagnostics` (`internal/plugins/ospf/doctor.go`) and logged at `ERROR` with the same code and fix in `p.OnConfigure` (`register.go`) when startup rejects `ospf drain-on-shutdown > 0` (`Ze always starts`). Removed when `spec-daemon-drain-before-stop.md` Phase 5 wires OSPF `OnDrain`. |
| Prometheus counters/metrics | No | No new metric proposed; `show ospf` carries `active` and `on-startup-remaining-seconds` |
| BGP family surface (new SAFI / capability / attribute) | N-A | OSPF only |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | `docs/features.md` lists OSPF broadly with no per-leaf max-metric row (checked at HEAD 2026-11-26) |
| 2 | Config syntax changed? | Yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang` (Q-6 rename `ospf drain-on-shutdown` if landing before drain Phase 5), `docs/guide/ospf.md`, and published YANG fixtures under `internal/le/site/testdata/` |
| 3 | CLI command added/changed? | Yes | `show ospf` adds `on-startup-remaining-seconds` (and renames `on-shutdown-seconds` to `drain-on-shutdown-seconds` on Q-6): `docs/guide/ospf.md` `show ospf` paragraph |
| 4 | API/RPC added/changed? | No | No new RPC |
| 5 | Plugin added/changed? | No | Behavior inside the existing OSPF plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/ospf.md`: max-metric subsection (`always`, `on-startup` non-GR start window + OSPFv3 inheritance + GR/clear/reload rules, `ospf drain-on-shutdown` interim refusal) |
| 7 | Wire format changed? | No | Router-LSA format unchanged; only when the existing `0xffff` value is sent |
| 8 | Plugin SDK/protocol changed? | No | Daemon drain SDK (`OnDrain`) already landed in `spec-daemon-drain-before-stop.md` Phases 1–3; this spec touches no SDK or protocol files |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 6987 defines no timer and is `Enrolment: backlog`; RFC scope frozen, so `rfc/short/rfc6987.md` and `docs/features/rfc-status.md` are not edited |
| 10 | Test infrastructure changed? | No | Reuses existing `scenarioOperations` op kinds in `checkers.go` |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` makes no OSPF max-metric parity claim (checked at HEAD 2026-11-26) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ospf/ospf-13-cli-diag-interop.md` (rewrite the "parsed and armed nowhere" bullet) and `docs/architecture/testing/interop.md` (add `ospf-max-metric-startup-frr` row) |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared designs: `docs/architecture/ospf/ospf-13-cli-diag-interop.md` (modified) and `docs/architecture/testing/interop.md` (modified); `docs/architecture/ospf/ospf-4-component-config.md` (`instance.go`, `config.go`, `register.go`, `rfc5709_config_test.go`: unaffected, no max-metric text) and `docs/architecture/ospf/ospf-ext-3-router-information.md` (`ri.go`: unaffected, stub-router bit stays router-wide). Mentioned-only pages from `./le spec citation anchors` (all unaffected — cite unrelated symbols of `instance.go`, `config.go`, `register.go`, `ze-ospf-conf.yang`, `checkers.go`): `docs/DESIGN.md`, `docs/architecture/api/commands.md`, `docs/architecture/command-ownership.md`, `docs/architecture/config/syntax.md`, `docs/architecture/core-design.md`, `docs/architecture/ospf/bfd-client.md`, `docs/architecture/ospf/ospf-11-stub-nssa.md`, `docs/architecture/ospf/ospf-5-interface-ism.md`, `docs/architecture/ospf/ospf-7-lsdb-flooding.md`, `docs/architecture/ospf/ospf-ext-16-ipsec-auth.md`, `docs/architecture/ospf/ospf-ext-7-virtual-links.md`, `docs/architecture/ospf/ospf-ext-9-graceful-restart.md`, `docs/architecture/overview.md`, `docs/architecture/wire/ospf.md`, `docs/functional-tests.md`, `docs/guide/bfd.md`, `docs/guide/command-reference.md`, `docs/guide/configuration.md`, `docs/guide/plugins.md`, `docs/guide/redistribution.md`, `docs/guide/route-policy.md`, `docs/guide/status.md`, `docs/guide/vrrp.md`, `docs/plugin-development/metrics.md`, `docs/plugin-overview.md`. |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify `test/ospf/ospf-show.ci` and `docs/guide/ospf.md` max-metric examples against YANG |

## Design presentation

### What changes

Configuring `ospf { max-metric { router-lsa { on-startup <N>; } } }` now arms a real N-second stub-router window on every non-GR OSPF engine start (OSPFv2 base engine, reload-added OSPFv2 instances, and every OSPFv3 address family via top-level `MaxMetric` inheritance in `parseOSPFConfig`): non-stub Router-LSA links advertise `0xffff` (`65535`) until N seconds elapse, then re-originate with configured costs and flood. Graceful restarts (planned or unplanned, where the FIB is preserved), `clear ospf process all`, and reloading `on-startup` on a running engine do not open or re-arm the window. `show ospf` reports `stub-router.active: true` and `on-startup-remaining-seconds` while the window is open. For the shutdown half, `ospf max-metric router-lsa on-shutdown` is replaced by `ospf drain-on-shutdown` (per `spec-daemon-drain-before-stop.md` Q-6, if not yet landed) and any non-zero value is refused at validate/commit/reload with an actionable `opererr` until Phase 5 of `spec-daemon-drain-before-stop.md` wires OSPF into the daemon drain phase already landed at HEAD.

### Before and after

1. **Startup stub-router window (`on-startup 120`):**
   - Before: Ze parses `120`, sets RI bit 2, echoes `"on-startup-seconds": 120` and `"active": false` in `show ospf`, and originates Router-LSAs with configured link costs (e.g. `10`) from second 0; OSPFv3 engines never even receive `cfg.MaxMetric`.
   - After: On non-GR start, both OSPFv2 and OSPFv3 engines advertise `Metric: 65535` on non-stub links (keeping stub links at configured cost), `show ospf` reports `"active": true` and `"on-startup-remaining-seconds": <left>`, and at second 120 Ze re-originates Router-LSAs with configured costs (`10`) and omits `on-startup-remaining-seconds`.
2. **Graceful restart and `clear ospf process all` with `on-startup 120`:**
   - Before: Neither opened a window because no window existed.
   - After: A planned or unplanned RFC 3623 / RFC 5187 graceful restart (FIB preserved) and `clear ospf process all` still open no startup window; a cold start or a reload-added instance/AF opens its own N-second window.
3. **Shutdown drain leaf (`ospf drain-on-shutdown 60` before drain Phase 5 lands):**
   - Before: `ospf max-metric router-lsa on-shutdown 60` commits silently, `ze:help` promises a graceful-shutdown stub-router window, and nothing ever runs at shutdown.
   - After: `max-metric router-lsa on-shutdown` is replaced by `ospf drain-on-shutdown`; setting `ospf drain-on-shutdown 60` is refused by `ze config validate`, commit, and reload with an `opererr` naming `ospf drain-on-shutdown`, stating OSPF has not yet joined the daemon drain phase, and giving the fix (`set ospf drain-on-shutdown 0` or delete it). If present at `ze start`, `ze` keeps running its other services while OSPF stays unstarted, `OnConfigure` logs an `ERROR` line under `doctor-ospf-drain-on-shutdown-unsupported` with the fix, and `ze doctor` reports the same code and fix. Setting `0` or omitting it succeeds.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Build the shutdown drain inside this OSPF spec | Rejected by owner Q-1 (a): shutdown drain is a daemon-wide lifecycle shared by BGP, OSPF, and IS-IS (`spec-daemon-drain-before-stop.md`, Phases 1–3 already landed at HEAD; OSPF joins in Phase 5). |
| Refuse both `on-startup` and `drain-on-shutdown` | Discards the cheap startup half, since `OriginateFromTopology` and `v6OriginateSelf` already accept `maxMetric bool`. |
| Honour `drain-on-shutdown` only on reload-removed instances | Gives operators a leaf that drains on instance delete but not daemon stop, creating misleading semantics. |
| Count the startup window during RFC 3623 graceful restart | Rejected by owner Q-2: GR preserves forwarding state, so draining transit on a GR restart is wrong (matches IOS-XR NSF behavior). |
| Narrow the YANG range of `drain-on-shutdown` to `0..0` or delete the leaf until Phase 5 | Rejected by owner Q-3: keep the leaf and range in YANG and refuse non-zero values in `validateConfigAF` so Phase 5 only removes the refusal when wiring `OnDrain`. |
| Check a deadline lazily on origination with no timer, or poll with a dedicated goroutine/ticker | A lazy deadline never triggers re-origination on expiry; a dedicated goroutine violates `ai/rules/goroutine-lifecycle.md`; `time.AfterFunc` + `epoch` + `originateSelfLSAsDeferred` copies `ldp_sync.go`. |
| Custom special checker in `check_special.go` for FRR interop | Existing declarative op kinds in `scenarioOperations` (`checkers.go`) express both phases cleanly with no boilerplate. |

### Documents departed from

- `docs/architecture/ospf/ospf-13-cli-diag-interop.md`, section `Traps`:
  - **Current statement:** "`max-metric router-lsa on-startup` and `on-shutdown` are parsed and armed nowhere. Origination reads the always form only, so the summary `stub-router.active` field is faithful. The timed windows are an unimplemented engine feature."
  - **After this work:** The bullet is rewritten to state that `max-metric router-lsa on-startup` arms a one-shot startup stub-router window per non-GR engine start (inherited into OSPFv3 address families), `stub-router.active` and `on-startup-remaining-seconds` reflect the active window alongside `always`, and `ospf drain-on-shutdown` (renamed from `on-shutdown` per `spec-daemon-drain-before-stop.md` Q-6) is refused above 0 at verify until `spec-daemon-drain-before-stop.md` Phase 5 wires OSPF's `OnDrain` handler.

### Choices for Thomas

none

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- create `max_metric_window.go` with the window-aware flag helper (`maxMetricActiveLocked`, initially returning `RouterLSAAlways` only) and a no-op `armMaxMetricStartup`; wire `originateSelfLSAs` (both families) and `processSummary` to read the helper; inherit `cfg.MaxMetric` into `cfg.V6` and `cfg.V6Extra` in `parseOSPFConfig`; call `armMaxMetricStartup()` in `openInterfaces` after `gr.maybeUnplannedRestart()`.
   - Tests: `TestOSPFMaxMetricStartupWindowOpensAtStart`, `TestOSPFv6MaxMetricInheritedFromTopLevel`, `TestOSPFv6MaxMetricStartupWindow`, `TestOSPFMaxMetricStartupWindowNewInstanceAtReload` written and failing on the no-op window.
   - Files: `max_metric_window.go`, `instance.go`, `show_summary.go`, `config.go`, `max_metric_window_test.go`, `origination_v6_test.go`, `multi_instance_test.go`.
   - Verify: grep for `RouterLSAAlways` outside `config.go`, `ri.go`, `show_summary.go`, and `max_metric_window.go` returns nothing; existing OSPF tests stay green.
2. **Phase: Startup window & GR/clear/reload rules** -- implement `armMaxMetricStartup` (one-shot per engine lifetime via `afterFunc` + `now` + `epoch`, skipping when `e.gr.inRestart()` or `OnStartupSec == 0`), timer callback calling `originateSelfLSAsDeferred`, and `stopMaxMetricStartup` in `shutdown` before `cancel`. Rewrite the Traps bullet in `docs/architecture/ospf/ospf-13-cli-diag-interop.md`.
   - Tests: AC-1..AC-10 and AC-12 unit tests (`max_metric_window_test.go`, `origination_v6_test.go`, `multi_instance_test.go`).
   - Files: `max_metric_window.go`, `instance.go`, `docs/architecture/ospf/ospf-13-cli-diag-interop.md`.
   - Verify: tests fail → implement → pass under `-race`.
3. **Phase: Show** -- populate `stub-router.active` and `stub-router.on-startup-remaining-seconds` (`omitempty`) from the window helper; rewrite stale `stubRouterView` comments.
   - Tests: `TestOSPFShowSummaryStartupWindow`, `test/ospf/ospf-max-metric-startup.ci`.
   - Files: `show_summary.go`, `show_summary_test.go`, `test/ospf/ospf-max-metric-startup.ci`, `docs/guide/ospf.md`.
4. **Phase: `ospf drain-on-shutdown` interim refusal, doctor code & Q-6 rename** -- if `spec-daemon-drain-before-stop.md` Phase 5 has not yet landed, perform its Q-6 rename (`ze-ospf-conf.yang`, `config.go`, `ri.go`, `show_summary.go`, `rfc5709_config_test.go`, `internal/le/site/testdata/published-configuration.md`, `published-yang-config-tree.json`); add the `opererr` refusal wrapping `ErrDrainOnShutdownUnsupported` in `validateConfigAF` for `DrainOnShutdownSec > 0`; register `codeOSPFDrainOnShutdownUnsupported` (`doctor-ospf-drain-on-shutdown-unsupported`) in `ospfDiagnosticCodes` and `registerOSPFDoctor` (`register.go`), log an `ERROR` line under that code and fix in `p.OnConfigure` when startup rejects `drain-on-shutdown > 0`, and emit the matching error finding with `Fix` in `ospfConfigDiagnostics` (`doctor.go`); update `description` and `ze:help` for `router-lsa`, `on-startup`, and `drain-on-shutdown` and document in `docs/guide/ospf.md`.
   - Tests: `TestOSPFDrainOnShutdownRefused`, `TestOSPFDrainOnShutdownZeroAccepted`, `TestOSPFDoctorDrainOnShutdownUnsupported`, `TestOSPFOnConfigureLogsDoctorCodeOnDrainOnShutdown`, `test/ospf/ospf-drain-on-shutdown-refused.ci`.
   - Files: `config.go`, `doctor.go`, `register.go`, `yang/ze-ospf-conf.yang`, `ri.go`, `show_summary.go`, `doctor_test.go`, `rfc5709_config_test.go`, `max_metric_window_test.go`, `test/ospf/ospf-drain-on-shutdown-refused.ci`, `docs/guide/ospf.md`, `internal/le/site/testdata/*`.
5. **Phase: Interop** -- add `test/interop/scenarios/ospf-max-metric-startup-frr/` (`ze.conf`, `frr.conf`), add its declarative entry to `scenarioOperations` in `internal/le/interoplab/bgp/checkers.go`, and add its row to `docs/architecture/testing/interop.md`. Run with `INTEROP_SCENARIO=ospf-max-metric-startup-frr ./le test integration interop` and run both discrimination reverts.
   - Files: `test/interop/scenarios/ospf-max-metric-startup-frr/`, `internal/le/interoplab/bgp/checkers.go`, `docs/architecture/testing/interop.md`.

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-16 has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Stub (type 3) links never take `0xffff`; window end produces a new LSA instance; `parseOSPFConfig` inherits `MaxMetric` into `V6` and `V6Extra`; `armMaxMetricStartup` runs after `gr.maybeUnplannedRestart()` and skips when `gr.inRestart()`; `clearProcess` and `reconcile` never re-arm the window |
| Naming | JSON keys `on-startup-remaining-seconds` and `drain-on-shutdown-seconds` kebab-case, alongside `on-startup-seconds` |
| Data flow | Exactly one window-aware helper decides max-metric in force; v4, v6, and `show ospf` all read it |
| Operator errors and doctor codes | `drain-on-shutdown` refusal is an `opererr` wrapping `ErrDrainOnShutdownUnsupported` naming `ospf drain-on-shutdown`, the reason, and the fix on the last line; `codeOSPFDrainOnShutdownUnsupported` (`doctor-ospf-drain-on-shutdown-unsupported`) is registered in `ospfDiagnosticCodes` and `registerOSPFDoctor`, logged at `ERROR` with its fix in `OnConfigure` on startup refusal, and emitted with `Fix` in `ospfConfigDiagnostics` (`ai/rules/user-facing-errors.md`) |
| Rule: `ai/rules/goroutine-lifecycle.md` | No `go func()`; one-shot `afterFunc` timer callback only queues `originateSelfLSAsDeferred` and is stopped in `shutdown` before `cancel` |
| Rule: `ai/rules/stale-comments.md` | `stubRouterView` comment, `maxMetricConfig` field comments, YANG descriptions/`ze:help`, `deriveRICapabilities` comment all match the new behavior |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| Window-aware flag read by both origination calls and `show ospf` | grep `RouterLSAAlways` in `internal/plugins/ospf/` (non-test): only `config.go`, `ri.go`, `show_summary.go`, `max_metric_window.go` |
| Startup window, refusal & doctor unit tests | Main thread runs `./le test unit pkg path ./internal/plugins/ospf/ run 'MaxMetric|ShowSummaryStartup|DoctorConfigSanity|DoctorDrainOnShutdown|OnConfigureLogsDoctorCode'` |
| Functional tests | Main thread runs `./le test ospf` covering `ospf-max-metric-startup.ci`, `ospf-drain-on-shutdown-refused.ci`, and unchanged `ospf-show.ci` |
| Interop scenario | Main thread runs `INTEROP_SCENARIO=ospf-max-metric-startup-frr ./le test integration interop` green, plus both discrimination reds recorded |
| Docs & anchors | `./le spec citation anchors spec plan/release/fixes/spec-ospf-max-metric-startup-shutdown-windows.md` and `./le doc check all` clean |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | `on-startup` bounded by YANG to `0..86400`; duration computed from that bounded uint32 cannot overflow `time.Duration` |
| Resource exhaustion | At most one timer per engine, armed once per engine lifetime; reloads and `clear ospf process all` cannot stack timers |
| Fail open | A config that cannot be honoured (`ospf drain-on-shutdown > 0` before drain Phase 5 lands) is refused with `opererr`, never silently stored |

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

- RFC 6987 defines the stub-router mechanism and motivation, never the timers; startup anchor, GR interaction, and clear/reload semantics are product decisions settled by Thomas's Q-2 ruling.
- Shutdown drain is a daemon-wide lifecycle feature shared by BGP, OSPF, and IS-IS (`spec-daemon-drain-before-stop.md`, Phases 1–3 landed at HEAD; Q-6 unified the leaf name to `drain-on-shutdown`).
- Top-level OSPF leaves that have no counterpart inside `grouping ospf-af-topology` (like `reference-bandwidth` and `max-metric`) silently never reach OSPFv3 engines unless `parseOSPFConfig` explicitly inherits them into `cfg.V6` and `cfg.V6Extra`.

## Owned journal rows
<!-- Every open journal row this spec will fix, one line each; `./le commit create`
     refuses an open row no spec lists (plan/journal/README.md, "Owned rows"). -->
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/unwired-feature.md` | 2026-09-03 | `ospf/max-metric/router-lsa/on-startup` and `on-shutdown` (`ze-ospf-conf.yang`) |
| `plan/journal/unwired-feature.md` | 2026-09-04 | `ospf/max-metric/router-lsa/*` (shared multi-surface row; OSPF max-metric portion closed here) |

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Build `on-startup` in full here; rename `on-shutdown` to `ospf drain-on-shutdown` (if not yet landed) and refuse > 0 until `spec-daemon-drain-before-stop.md` Phase 5 wires OSPF `OnDrain` (owner Q-1 (a), Q-3, drain Q-6) | (1) Build shutdown drain in this spec; (2) Refuse both leaves; (3) Drain only on reload-removed instances | (1) Duplicates daemon drain lifecycle already built in `spec-daemon-drain-before-stop.md`; (2) discards the cheap startup half; (3) misleads operators about daemon stop behavior |
| Unconditionally inherit `cfg.MaxMetric` into `cfg.V6` and `cfg.V6Extra` in `parseOSPFConfig` | Leave `max-metric` IPv4-only or duplicate `container max-metric` inside `ospf-af-topology` | Matches `reference-bandwidth` inheritance and RFC 6987 Section 2 ("applies to both OSPFv2 and OSPFv3") without duplicating config syntax |
| Anchor the startup window at `openInterfaces` AFTER `gr.maybeUnplannedRestart()`, skipping when `gr.inRestart()` is true (owner Q-2) | Arm at plugin process start; arm before `maybeUnplannedRestart`; arm during GR | `openInterfaces` covers base, reload-added instances, and OSPFv3 AFs uniformly while leaving `clearProcess` and reloads untouched; checking `gr.inRestart()` after `maybeUnplannedRestart()` skips both planned and unplanned GR starts whose FIB is preserved |
| One window-aware flag helper (`maxMetricActiveLocked`) read by v4, v6, and `show ospf` | Separate checks at each call site | Prevents v4/v6/show disagreement (R-2) |
| One-shot timer via injectable `afterFunc` + `now` + `epoch` calling `originateSelfLSAsDeferred`, copying `ldp_sync.go` | Dedicated goroutine/ticker; lazy deadline with no timer | Lazy deadline never triggers re-origination on expiry; dedicated goroutine breaks goroutine-lifecycle rules |
| Keep RI capability bit 2 router-wide from any configured stub-router mode | Raise bit 2 only while a window is open | RFC 7770 Section 2.5 defines bit 2 as capability support, not active window state, and `TestRICapabilityBitsFromState` pins it |
| Declarative op entry in `scenarioOperations` (`checkers.go`) for FRR interop | Custom Go checker in `check_special.go` | Existing declarative op kinds express both phases cleanly |

## Owner Decisions

All questions were answered by Thomas on 2026-10-13 and are unfolded into the design above; no open questions remain.

| ID | Topic | Decision (2026-10-13) | Where unfolded |
|----|-------|-----------------------|----------------|
| Q-1 | Shutdown drain lifecycle | Option (a): "we need a drain phase for every protocol adding routes to the FIB/RIB" — owned by `plan/release/fixes/spec-daemon-drain-before-stop.md` (Phases 1–3 landed at HEAD; OSPF joins in Phase 5). Refuse non-zero values here until Phase 5 lands. | Task, Existing Mechanisms, AC-13, Phase 4 |
| Q-2 | Startup window triggers (GR, `clear ospf process`, reload-added instance) | "clear and instance follow FRR" (`clear ospf process all` does not reopen; reload-added instance/AF opens its own window); GR preserves FIB so opens no startup window ("do it correctly"). | Task, Data Flow, AC-7..AC-10, Phase 2 |
| Q-3 | YANG leaf/range during interim refusal | "do it correctly": keep the leaf and range in YANG, never silently accept/ignore, refuse any value above 0 at verify with an `opererr` naming the leaf, reason, and fix until Phase 5 wires `OnDrain`. | Task, AC-13..AC-14, Integration Checklist, Phase 4 |
| Drain Q-6 | Unified shutdown drain leaf name | `ospf max-metric router-lsa on-shutdown` deleted and replaced by `ospf drain-on-shutdown` (no compatibility shim); whichever of this spec or `spec-daemon-drain-before-stop.md` Phase 5 lands first performs the rename. | Task, Current Behavior, Files to Modify, Phase 4 |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- `ospf drain-on-shutdown` > 0 is refused until OSPF joins the daemon drain phase in Phase 5 of `plan/release/fixes/spec-daemon-drain-before-stop.md` (which deletes this spec's interim refusal and wires OSPF's `OnDrain` handler).
- A startup window tied to BGP convergence ("until BGP is established") is not offered; no YANG leaf promises it.
- OSPFv3 uses MaxLinkMetric (`0xffff`), as `always` does today; the R-bit alternative of RFC 6987 Section 2.1 is not used.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer constraints, message ordering, and every MUST/MUST NOT.

For this spec: `maxMetricActiveLocked` quotes RFC 6987 Section 2 ("router X announces its router-LSA to the neighbors with the cost of all non-stub links (links of the types other than 3) being set to MaxLinkMetric"), `armMaxMetricStartup` quotes RFC 6987 Section 1 ("Graceful introduction and removal of the router to/from the network.") and notes that the RFC defines no timer, and both origination call sites in `originateSelfLSAs` carry `// RFC 6987 Section 2`.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 6987 Section 2 & Section 3 | Default (`always false`, `on-startup 0`): Router-LSAs advertise configured link costs with no MaxLinkMetric window | `always true` advertises `0xffff` indefinitely on non-stub links (preserving stub costs); `on-startup N` advertises `0xffff` on non-stub links for N seconds after non-GR engine start | `TestOSPFMaxMetricStartupZeroNoWindow`, `TestOSPFMaxMetricStartupSkippedDuringGR` | `TestOSPFMaxMetricStartupWindowOpensAtStart`, `TestOSPFMaxMetricStartupWindowCloses`, `TestOSPFMaxMetricAlwaysOverridesWindowExpiry`, `TestOSPFv6MaxMetricStartupWindow`, `ospf-max-metric-startup-frr` |
| RFC 7770 Section 2.5 | Default (no stub-router mode configured): RI informational capability bit 2 clear | Any configured stub-router mode (`always`, `on-startup > 0`, `drain-on-shutdown > 0`) sets bit 2 router-wide | `TestRICapabilityBitsFromState` | `TestRICapabilityBitsFromState` (unchanged) |

### Discovery (`ai/rules/repo-maintenance.md`)

| Question | Answer |
|----------|--------|
| Where does an agent look first? | `ai/INDEX.md` OSPF row → `docs/guide/ospf.md` max-metric subsection and `docs/architecture/ospf/ospf-13-cli-diag-interop.md` |
| What prevents regression? | Unit tests in `max_metric_window_test.go`, `origination_v6_test.go`, `multi_instance_test.go`, `show_summary_test.go`, functional `.ci` tests, and `ospf-max-metric-startup-frr` with recorded discrimination reds |
| What inventory prevents drift? | YANG `description` and `ze:help` are the operator contract; `plan/journal/unwired-feature.md` rows close at closure |
| What proves it? | `ospf-max-metric-startup-frr` green log (with both discrimination reds) and the two functional tests |

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
- [ ] AC-1..AC-16 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-ospf-max-metric-startup-shutdown-windows.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
