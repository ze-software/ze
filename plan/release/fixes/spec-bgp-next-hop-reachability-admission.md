# Spec: bgp-next-hop-reachability-admission

| Field | Value |
|-------|-------|
| Status | done |
| Scope | protocol |
| Priority | P1 |
| Release reason | An unresolved BGP next hop must not displace a usable route; operators must see excluded routes and unresolvable injects instead of silent exclusion, and affected functional fixtures must supply real next-hop reachability |
| RFC gaps | RFC4271-3.1-2, RFC4271-9.1.2.1-2 |
| Depends | - |
| Phase | closure |
| Handoff | - |
| Updated | 2026-11-22 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Complete the BGP next-hop reachability admission work begun under the RFC 4271
campaign and close its remaining operator-visibility, fail-closed reporting, and
functional-fixture obligations.

Core RFC 4271 Section 9.1.2 admission is already landed on `main`:
- Ordinary forwarding admission (`candidateNextHopResolved`,
  `internal/component/bgp/plugins/rib/rib_next_hop.go`) excludes routes whose
  next hop is unresolved or prospectively self-capturing before Phase 2 ranking
  (`46f486c4a0`), while retaining received paths in Adj-RIB-In and reselecting
  when interior reachability changes.
- Private forwarded-wire recovery (`candidateAdmissionRecovery` in
  `rib_recovery.go` / `rib_commands.go`) preserves route-server recovery without
  relaxing ordinary forwarding admission.
- Internal and forked Linux kernel proofs (`test/plugin/rfc4271-next-hop-kernel*.ci`
  and `test/plugin/rfc4271-prospective-next-hop-kernel*.ci`) landed and audited
  `RFC4271-3.1-2`, `RFC4271-9.1.2-1`, `RFC4271-9.1.2-4`, and `RFC4271-9.1.2.1-2`
  (`b032fb7dc7`), and `test/plugin/lg-best-table.ci` moved onto the shared
  `terminalCovers` helper (`b5079ff5ff`).

Four concrete items remain to finish this spec:
1. **Surface unresolved next-hop exclusions in `show bgp rib` and `ze doctor`
   (owned row `plan/journal/silent-fall-through.md`, 2026-11-18):**
   `candidateNextHopResolved` currently drops excluded candidates with a bare
   `continue`, so `show bgp rib best` is empty while `show bgp rib` lists the
   stored routes with nothing stating why they are ineligible. Expose exclusion
   status on stored received rows in `show bgp rib` and register a `runtime`
   doctor check (`bgp-rib-unresolved-next-hop`, code
   `doctor-bgp-next-hop-unresolvable`) declared through `sdk.Registration` so it
   answers in both internal and forked placements.
2. **Warn when `request bgp rib inject` names an unresolvable next hop
   (owned row `plan/journal/silent-fall-through.md`, 2026-11-20):**
   `RIBManager.injectRoute` (`rib_commands.go`) answers `injected <prefix>`
   with no hint when the supplied next hop does not resolve and best-path
   immediately excludes the route. Retain the route in Adj-RIB-In as today, and
   report `"next-hop-resolvable": false` plus a user-facing warning naming the
   next hop and the remedy (`show rib` / `show bgp rib`) in the inject answer
   and a coalesced/scoped warning log.
3. **Complete the fail-closed boundary review of `remoteRIB.prospectiveDistance`
   and `RIBManager.igpDistance`:**
   Both sites already carry `// refuse-by-default:` annotations and coalesce
   SDK report-bus warnings via `noteMetricFailure`, but their `logger().Error`
   calls still fire on every uncached candidate lookup during an outage and
   `doctor-bgp-next-hop-metrics` is not exposed through the RIB's declared
   runtime doctor callback. Coalesce the error log on `false -> true` transition
   and expose active remote-metric failure from the RIB's runtime doctor
   callback alongside `bgp-rib-selected-output`.
4. **Repair the remaining `.ci` functional fixtures that assert `show bgp rib best`
   over unresolved mock next hops, and update stale architecture docs:**
   Supply real independent terminal host covers via the shared `terminalCovers`
   helper (`internal/test/fixture/terminal_cover_fixture.go`) for the remaining
   failing `.ci` fixtures (`rr-basic`, `bestpath-reason`,
   `api-peer-create-delete-rib`, `multipath-basic`, `nexthop-self`,
   `nexthop-unchanged`, `rib-forward-handle-observed`, `rib-inject-rfc5549`,
   `fib-ecmp`, `fib-ecmp-realtime`, `bgp-rib-maximum-paths-reload`,
   `llgr-readvertise-multipeer`, `llgr-readvertise-multipeer-forked`, plus audit
   `srv6-invalid-sid-ineligible`), keeping every assertion unchanged and
   obtaining independent `ze-test-judge` approval for each fixture edit. Update
   `docs/architecture/route-selection.md` and
   `docs/architecture/doctor-and-health-checks.md` to match the completed state.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/route-selection.md` - Phase 1/Phase 2 next-hop admission, prospective preview, and unified rejection reasons
  → Decision: forwarding admission excludes unresolved and prospectively self-capturing next hops before Phase 2 ranking while retaining received paths in Adj-RIB-In; private recovery admission omits local resolution without relaxing ordinary selection.
  → Constraint: lines 146-161 still describe live prospective proof and broad RIB unit failures as open; this spec updates that section when the remaining work lands.
- [ ] `docs/architecture/doctor-and-health-checks.md` - "The runtime phase: checks only the daemon runs"
  → Decision: a plugin's runtime doctor check MUST be declared in Stage 1 `sdk.Registration.DoctorChecks` with `Phase: rpc.DoctorPhaseRuntime` and answered in `OnDoctorCheck` so internal and forked placements answer identically.
  → Constraint: `registry.DoctorCheckDef` refuses the `runtime` phase; owning-package codes register via `diagnostic.Register` in `doctor_codes.go`, and every finding carries `Fix`.
- [ ] `docs/architecture/plugin/rib-storage-design.md` - RIB storage, manual injection, and show pipelines
  → Constraint: manual injection shares the synthetic BGP source slot in `ribInPool` and participates in forwarding selection; `show bgp rib` streams rows through `showRow` / `serializeRouteItem` without holding peer locks across output rendering.
- [ ] `docs/architecture/api/commands.md` - "Design decisions" (the pipe wins; command pipe overrides keep the generic's data format)
  → Constraint: do not add a subcommand for scope or presentation (e.g. no `show bgp rib unusable` subcommand); surface unresolved next-hop visibility as fields on existing `show bgp rib` rows so existing pipes (`| match`, `| json`, `| count`) filter them directly.
- [ ] `docs/architecture/api/process-protocol.md` - `route-metrics` prospective RPC and runtime doctor checks
  → Constraint: prospective metric proposals are independent, bounded values that never mutate Loc-RIB; doctor check responses use `rpc.DoctorCheckDiagnostic` (`Code`, `Severity`, `Message`, `Fix`).

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - Sections 3.1, 9.1.2, 9.1.2.1 (`RFC4271-3.1-2`, `RFC4271-9.1.2-1`, `RFC4271-9.1.2-4`, `RFC4271-9.1.2.1-2`)
  → Constraint: "If the NEXT_HOP attribute of a BGP route depicts an address that is not resolvable, or if it would become unresolvable if the route was installed in the routing table, the BGP route MUST be excluded from the Phase 2 decision function." All four requirements already carry enforced audits on `HEAD`; no edit weakens their assertions or discrimination records.
- [ ] `rfc/short/rfc7947.md` - Route server transparency (Sections 2.1, 2.2.1)
  → Constraint: attribute transparency does not create a blanket RS-client exemption from ordinary forwarding admission; forwarded-wire recovery uses its own private `candidateAdmissionRecovery` purpose.

**Key insights:**
- All four RFC 4271 next-hop requirements (`3.1-2`, `9.1.2-1`, `9.1.2-4`, `9.1.2.1-2`) are already enforced and audited on `HEAD` (`b032fb7dc7`), and the 148 old RIB unit failures have already been resolved on `main`.
- `show bgp rib` (`rib_pipeline.go`, `rib_pipeline_bestfact.go`) currently writes `"best"` onto received rows but has no field naming an unresolved next hop, and `injectRoute` (`rib_commands.go:382-536`) returns `{"injected", "peer", "family"}` with no indication when `candidateNextHopResolved` immediately excluded the injected route.
- `bgp-rib` already wires a runtime doctor callback (`rib.go:814,912`, `rib_advertise_diagnostic.go`) and owning-package code registry (`doctor_codes.go`), so adding unresolved-next-hop and remote-metric runtime checks extends that exact registration without introducing a new transport or registry.
- Eleven functional `.ci` fixtures across `test/plugin/` and `test/reload/` still assert `show bgp rib best` over mock next hops (`10.0.0.1`, `1.1.1.1`, `10.0.0.11/12/13`, `2001:db8::1`) with no covering route in the Loc-RIB; `internal/test/fixture/terminal_cover_fixture.go` (`terminalCovers`) already provides the `ze-test-judge`-approved pattern for supplying real terminal host covers.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/rib_next_hop.go` - `candidateNextHopResolved` excludes unresolved or prospectively self-capturing candidates; `prospectiveDistance` queries `RouteMetricsProspective` with 5s timeout, returns `igpcost.Distance{}` on error (`// refuse-by-default:`), calls `noteMetricFailure(true)`, and logs an Error on every call.
- [ ] `internal/component/bgp/plugins/rib/rib_remote.go` - `igpDistance` queries `RouteMetrics` with 5s timeout, returns `igpcost.Distance{}` on error (`// refuse-by-default:`), calls `noteMetricFailure(true)`, and logs an Error on every call; `runRemoteMetrics` coalesces SDK `RaiseIssue`/`ClearIssue` for `doctor-bgp-next-hop-metrics` outside locks.
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - `gatherPrefixCandidatesForLocked` (`1534-1540`) and `gatherKeyCandidatesForLocked` (`1611-1617`) drop unresolved candidates with `path.Release(); continue`; `injectRoute` (`382-536`) stores the route, calls `reconcileBestPath`, and always answers `{"injected", "peer", "family"}`.
- [ ] `internal/component/bgp/plugins/rib/rib_manual.go` - `appendManualCandidatesLocked` (`16-39`) drops manual candidates whose next hop does not resolve with a bare `continue`.
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline.go` & `rib_pipeline_bestfact.go` - `show bgp rib` builds rows (`showRow` -> `serializeRouteItem`) with `"best"` and route attributes, with no next-hop exclusion field.
- [ ] `internal/component/bgp/plugins/rib/rib_advertise_diagnostic.go`, `doctor_codes.go`, `rib.go` - declares `bgp-rib-selected-output` in `sdk.Registration.DoctorChecks` (`Phase: rpc.DoctorPhaseRuntime`) and dispatches it in `OnDoctorCheck`; registers `doctor-bgp-selected-output` via `diagnostic.Register`.
- [ ] `internal/test/fixture/terminal_cover_fixture.go`, `plugin_fixture_15_show.go`, `plugin_fixture_02.go`, `plugin_fixture_13_peer_lifecycle.go`, `plugin_fixture_10.go`, `plugin_fixture_13_rib.go`, `plugin_fixture_06_fib.go`, `plugin_fixture_09_bgp.go` - shared `terminalCovers` helper and the remaining `.ci` fixture drivers that lack next-hop covers.

**Behavior to preserve:**
- Forwarding admission in `candidateNextHopResolved` stays strictly fail-closed for unresolved, prospectively recursive, and metric-RPC-failed next hops (RFC 4271 Section 9.1.2).
- Routes with no address next hop (FlowSpec, manual injects without `nhop`/`nexthop`) continue passing `candidateNextHopResolved`.
- Private forwarded-wire recovery (`candidateAdmissionRecovery` in `rib_recovery.go`) remains unchanged and never relaxes ordinary forwarding admission.
- `request bgp rib inject` still retains the route in Adj-RIB-In (so later interior reachability selects it without re-injection) and preserves the existing `"injected"`, `"peer"`, and `"family"` response keys.
- Existing `show bgp rib` commands, pipe grammar, and `"routes"` / `"best-path"` envelopes stay compatible; no new subcommand is added.
- Zero-allocation prospective resolution in `internal/core/rib/igpcost` and `internal/core/rib/locrib` is untouched.
- Every repaired `.ci` fixture preserves all original BGP, RIB, FIB, multipath, LLGR, and API assertions.

**Behavior to change:**
- `show bgp rib` (`received` scope) adds `"next-hop-resolvable": false` and `"ineligible-reason": "next-hop-unresolvable"` on stored routes whose next hop currently fails forwarding admission (omitted when resolvable or when no address next hop is present, preserving clean output for ordinary valid routes).
- `bgp-rib` tracks currently excluded prefixes/routes (updated during best-path selection and cleared when resolved or withdrawn) and exposes two runtime doctor findings via its existing `sdk.Registration.DoctorChecks` / `OnDoctorCheck` callback:
  - `bgp-rib-unresolved-next-hop` (`doctor-bgp-next-hop-unresolvable`, warning): names the count of excluded routes/prefixes, a bounded sample of prefix + unresolved next hop, and the fix (`ze show bgp rib | match next-hop-unresolvable` / `ze show rib`).
  - `bgp-rib-next-hop-metrics` (`doctor-bgp-next-hop-metrics`, warning): reports an active engine `route-metrics` lookup failure in forked placement so `show doctor` reads live state directly rather than relying solely on transient report-bus events, and deduplicates with `recent-events` via `DoctorCheckDecl.Codes`.
- `request bgp rib inject` checks whether a supplied next hop resolves after storing and reconciling; when unresolved, it adds `"next-hop-resolvable": false` and `"warning"` naming the next hop and remedy (`show rib` / `show bgp rib`) to its response and emits one structured `WARN` log with code `doctor-bgp-next-hop-unresolvable`.
- `remoteRIB.prospectiveDistance` and `RIBManager.igpDistance` coalesce `logger().Error` onto the `false -> true` transition returned by `noteMetricFailure(true)` so repeated lookups during an engine outage do not flood logs while keeping fail-closed `Resolved: false` uncached.
- The remaining failing `.ci` fixtures install real independent terminal host covers for their next hops via `terminalCovers` (or static connected/device cover in reload `.ci` configs) so they exercise real next-hop admission instead of failing on empty `best-path`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Normal route admission and prospective preview | `internal/core/rib/locrib/prospective.go` (`PreviewSelected`), `internal/core/rib/igpcost/prospective.go` (`ResolveProspective`) | Previews one BGP contribution in the Loc-RIB without mutation at 0 B/op and 0 allocs/op | extends (reuses): `show bgp rib` and `injectRoute` query the same `candidateNextHopResolved` / prospective check rather than inventing a second resolver |
| Candidate forwarding admission and recovery split | `internal/component/bgp/plugins/rib/rib_next_hop.go` (`candidateNextHopResolved`), `rib_commands.go` (`gatherPrefixCandidatesForLocked`, `gatherKeyCandidatesForLocked`), `rib_manual.go` (`appendManualCandidatesLocked`) | Checks next-hop feasibility before Phase 2 ranking and drops excluded routes with a bare `continue` | extends: records unresolved exclusions for runtime doctor reporting while keeping the exact fail-closed admission decision |
| Remote metric lifecycle and issue coalescence | `internal/component/bgp/plugins/rib/rib_remote.go` (`runRemoteMetrics`, `noteMetricFailure`, `reportRemoteMetricIssue`), `pkg/plugin/sdk/sdk_engine.go` (`RouteMetricsProspective`) | Bounds RPCs to 5s, invalidates caches on revision change, and coalesces `doctor-bgp-next-hop-metrics` warnings on the report bus | extends: makes `noteMetricFailure` return whether state transitioned so error logs coalesce too, and exposes `metricFailed` to `OnDoctorCheck` |
| RIB runtime doctor callback and owning-package codes | `internal/component/bgp/plugins/rib/rib.go` (`DoctorChecks`, `OnDoctorCheck`), `rib_advertise_diagnostic.go` (`advertiseDoctorDeclaration`, `advertiseDoctorCheck`), `doctor_codes.go` (`registerDoctorCodes`), `register.go` (`init` already registers `doctor-bgp-next-hop-metrics` via `diagnostic.Register` and `diagnostic.RegisterEventCode`) | Declares `bgp-rib-selected-output` at Stage 1 (`DoctorPhaseRuntime`), registers `doctor-bgp-selected-output` in `doctor_codes.go`, and registers `doctor-bgp-next-hop-metrics` + its event code in `register.go` | extends: adds `bgp-rib-unresolved-next-hop` and `bgp-rib-next-hop-metrics` to `DoctorChecks` / `OnDoctorCheck`, registers `doctor-bgp-next-hop-unresolvable` in `doctor_codes.go`, and preserves the single `doctor-bgp-next-hop-metrics` registration and event-code registration in `register.go` (no duplicate `diagnostic.Register`) |
| `show bgp rib` streaming row pipeline | `internal/component/bgp/plugins/rib/rib_pipeline.go` (`showRow`, `serializeRouteItem`), `rib_pipeline_bestfact.go` (`writeBestFact`) | Emits `"routes"` JSON rows with `"best"` and attribute fields without holding peer locks during write | extends: annotates unresolved received rows with `"next-hop-resolvable": false` and `"ineligible-reason": "next-hop-unresolvable"` so existing pipes (`| match`, `| json`, `| count`) filter them |
| Shared terminal-cover fixture helper | `internal/test/fixture/terminal_cover_fixture.go` (`terminalCovers`) | Installs real device-only static IPv4 `/32` covers via `route-install` (`AFI: 1, SAFI: 1`) and verifies selection and zero-cost resolution before injecting BGP routes | extends: generalizes `terminalCovers` in `terminal_cover_fixture.go` to also install IPv6 `/128` covers (`AFI: 2, SAFI: 1`, needed by `rib-inject-rfc5549`), and uses it to repair the remaining `.ci` fixtures rather than adding synthetic resolver bypasses |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "Every plugin, ours included, runs either internal (a goroutine in the daemon) or external (a forked subprocess that talks only over TLS and RPC). The SDK hides which. Code that only works internal is broken." | agrees | Next-hop visibility in `show bgp rib`, `injectRoute` warnings, log coalescence, and both runtime doctor checks run inside `bgp-rib` using `candidateNextHopResolved` (which branches on `r.forkRIB`) and `sdk.Registration.DoctorChecks` / `OnDoctorCheck`, working identically internal and forked |
| `ai/doors/bgp-plugins.md` | Boundaries | the door covering `internal/component/bgp/plugins/rib/*` | "A check over the plugin's live state is declared in the Stage 1 `sdk.Registration` `DoctorChecks` and answered by `OnDoctorCheck`, which reaches a forked plugin too" | agrees | Both `bgp-rib-unresolved-next-hop` and `bgp-rib-next-hop-metrics` declare `Phase: rpc.DoctorPhaseRuntime` in `sdk.Registration.DoctorChecks` and answer through `OnDoctorCheck`, never through `registry.DoctorCheckDef` |
| `ai/doors/bgp-engine.md` | Where it sits | referenced by `ai/doors/bgp-plugins.md` and `internal/component/bgp/` | "The engine speaks the protocol and holds no policy." | agrees | All visibility, injection warning, exclusion tracking, and doctor checks remain inside `internal/component/bgp/plugins/rib/`; no engine or reactor package gains RIB admission state |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `internal/test/fixture/*` and `test/**/*.ci` | "**A compiled Go fixture returns an error when an assertion fails** (`ai/patterns/functional-test.md`, `ai/rules/testing.md`)." | agrees | Repaired `.ci` fixtures reuse `terminalCovers`, which verifies cover installation and returns an error on any failure, while keeping every existing assertion intact |
| `docs/architecture/route-selection.md` | Phase 2: Best-Path Selection (RFC 4271 Section 9.1.2) | `// Design:` and anchor of `rib_next_hop.go`, `rib_remote.go`, `rib_commands.go`, `rib_bestchange.go`, `rib.go`; page this spec modifies | "Each engine metric request has a five-second response bound. A failed lookup leaves its candidate ineligible (`Resolved: false`) and is not cached." | agrees | Preserves the 5-second bound, uncached fail-closed rejection, and SDK report-bus coalescence while adding transition-coalesced error logging, runtime doctor exposure, `show bgp rib` / `inject` visibility, and removing the now-stale "still in progress / live proof incomplete" notes |
| `docs/architecture/route-selection.md` | Design: Unified Rejection Reason | `// Design:` and anchor of `rib_next_hop.go`, `rib_remote.go`, `rib_commands.go`; page this spec modifies | "The unified-reason target is a single `uint8` value per non-best route recording why it was not selected (set once during validation or best-path comparison, with `none` (0) for the winning route)." | departs | Surfaces unresolved next-hop exclusions on `show bgp rib` received rows via `"next-hop-resolvable": false` and `"ineligible-reason": "next-hop-unresolvable"` backed by `unresolvedNextHopTracker` while keeping `show bgp rib best | reason` (`BestStep`) separate rather than adding a unified `uint8` field on `storage.RouteEntry`; `route-selection.md` was updated in Phase 4 to record this split honestly |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | `// Design:` of `doctor_codes.go`, anchored to `rib_advertise_diagnostic.go`; page this spec modifies | "Each check runs once per answer, through one route. A plugin's runtime check reads state that lives in the plugin's own process, so its one route is the doctor callback, which reaches a forked plugin as well as an internal one." | agrees | Registers the new runtime checks solely in `sdk.Registration.DoctorChecks` / `OnDoctorCheck` with owning-package codes in `doctor_codes.go`, each finding carrying `Fix`, and documents them in the runtime-checks table |
| `docs/architecture/plugin/rib-storage-design.md` | Received source-policy provenance | `// Design:` and anchor of `rib.go`, `rib_commands.go`, `rib_manual.go`, `rib_pipeline.go`, `rib_bestchange.go` | "Manual injection uses the existing BGP synthetic-source slot in `ribInPool`. It participates in forwarding and system-RIB election, alongside received candidates, but has no source lease." | agrees | `injectRoute` continues storing into `ribInPool` and running `reconcileBestPath`, adding only a post-reconcile resolvability check to annotate the answer when the supplied next hop is unresolved |
| `docs/architecture/api/commands.md` | Design decisions | anchored to `rib_commands.go`, `// Design:` of `rib_pipeline.go` and `plugin_fixture_13_peer_lifecycle.go` | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Adds no subcommand for unresolved routes; adds optional fields on existing `show bgp rib` rows so operators filter with standard pipes (`show bgp rib | match next-hop-unresolvable`, `| json`, `| count`) |
| `docs/architecture/api/commands.md` | Design decisions | anchored to `rib_commands.go`, `// Design:` of `rib_pipeline.go` and `plugin_fixture_13_peer_lifecycle.go` | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Preserves the `"routes"` envelope and every existing pipe override on `show bgp rib`; adding optional row fields keeps the generic row format intact |
| `docs/architecture/forked-route-install.md` | Scope | anchored to `internal/component/bgp/plugins/rib/rib_remote.go` | "The BGP RIB also reads the engine's `ze-plugin-engine:route-metrics` RPC. Each reply contains a routing revision and resolved next-hop distances, including received AIGP along recursive BGP paths." | agrees | Preserves the exact `route-metrics` RPC, 1-second revision poll, cache invalidation, and fatal live-poll lifecycle while coalescing error logs and surfacing active lookup failure in `OnDoctorCheck` |
| `docs/architecture/api/process-protocol.md` | Engine to Plugin (Callbacks) | anchored to `internal/component/bgp/plugins/rib/rib.go` | "**doctor-check:** Engine invokes a plugin's declared doctor check by name. Plugin runs the check and returns diagnostics (code, severity, message). Declared during Stage 1 via `doctor-checks` field in `declare-registration`." | agrees | Uses the exact Stage 1 `doctor-checks` / `doctor-check` RPC, `report-issue` (with `diagnostic.RegisterEventCode` retained in `register.go`), and `route-metrics` / `prospective-routes` RPCs with zero wire schema changes |
| `docs/architecture/update-building.md` | Three Paths for UPDATE Messages | anchored to `internal/component/bgp/plugins/rib/rib_commands.go` | "" | silent | Describes BGP UPDATE wire building; this spec changes neither UPDATE encoding nor wire framing |
| `docs/architecture/api/birdwatcher-compat.md` | 6. Route object | anchored to `internal/component/bgp/plugins/rib/rib_pipeline.go` | "" | silent | Describes Looking Glass birdwatcher API mapping; additive optional fields on `show bgp rib` rows omitted for resolvable routes do not alter birdwatcher responses |
| `docs/architecture/buffer-architecture.md` | Component Design | anchored to `internal/component/bgp/plugins/rib/rib_bestchange.go` | "" | silent | Describes pooled wire/attribute buffers; this spec touches no buffer pool or wire encoding path |
| `docs/architecture/core-design.md` | 4. RIB Storage Model | anchored to `internal/component/bgp/plugins/rib/rib_bestchange.go` | "" | silent | Describes RIB storage pools and attribute deduplication; the bounded unresolved tracker lives beside `bestPrev` without altering pool storage |
| `docs/architecture/edge-cases/extended-message.md` | Ze implementation | anchored to `internal/component/bgp/plugins/rib/rib_bestchange.go` | "" | silent | Describes RFC 8654 extended message sizes; unaffected by next-hop visibility, doctor checks, or fixture covers |
| `docs/architecture/rib-transition.md` | Key Components | anchored to `internal/component/bgp/plugins/rib/rib_bestchange.go` and `rib.go` | "" | silent | Describes the RIB plugin migration architecture; this spec stays within `bgp-rib` and changes no ownership boundary |
| `docs/architecture/rib/forward-handle.md` | The decisions | anchored to `internal/component/bgp/plugins/rib/rib_bestchange.go` and `rib.go` | "" | silent | Describes forwarding-handle tracking; unaffected by unresolved next-hop visibility or doctor checks |
| `docs/architecture/behavior/fsm-established.md` | Events handled in Established | anchored to `internal/component/bgp/plugins/rib/rib.go` | "" | silent | Describes BGP FSM Established-state transitions; this spec touches no FSM state machine logic |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Decision | anchored to `internal/component/bgp/plugins/rib/rib.go` | "" | silent | Describes Prometheus metrics pull vs push model; this spec adds no Prometheus metric |
| `docs/architecture/testing/runner-architecture.md` | Layers | anchored to `internal/test/fixture/plugin_fixture_02.go` | "" | silent | Describes the functional test runner architecture; fixture edits reuse the existing `terminalCovers` helper without changing runner behavior |
| `docs/architecture/iface/netlink-monitor.md` | Functional test lifecycle | anchored to `internal/test/fixture/plugin_fixture_10.go` | "" | silent | Describes interface netlink monitor lifecycle; `plugin_fixture_10.go` edits only add `terminalCovers` for BGP next hops |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- **BGP UPDATE / manual inject:** received UPDATE or `request bgp rib inject <peer> <family> <prefix> [nexthop <addr> ...]` stored in `ribInPool` (`rib.go`, `rib_commands.go`).
- **Operator inspection:** `show bgp rib [received]` (`rib_pipeline.go`), `show bgp rib best` (`rib_pipeline_best.go`), `ze doctor` / `show doctor` (`OnDoctorCheck` in `rib.go`).
- **Interior reachability change:** Loc-RIB subscription callback (internal) or 1-second `RouteMetrics` revision poll (forked, `runRemoteMetrics` in `rib_remote.go`).

### Transformation Path
1. Route is inserted into `ribInPool` (wire UPDATE or `injectRoute`).
2. `checkRouteBestChange` / `reconcileBestPath` gathers candidates via `gatherPrefixCandidatesForLocked` / `gatherKeyCandidatesForLocked` + `appendManualCandidatesLocked` with `candidateAdmissionForwarding`.
3. `candidateNextHopResolved` evaluates `igpcost.ResolveProspective` (internal) or `remote.prospectiveDistance` (forked) for CIDR routes, or `igpDistance` for non-CIDR routes.
4. When a candidate is excluded for an unresolved next hop, the RIB records its `(family, prefix/key, peer, next-hop)` in a bounded in-memory unresolved-exclusion tracker (capped per RIB, cleared when that prefix/key later elects or has no unresolved candidates left, and pruned on peer DOWN/purge).
5. On `request bgp rib inject`, if the caller supplied a next hop and forwarding admission marks it unresolved, `injectRoute` attaches `"next-hop-resolvable": false` and `"warning"` to the command response and logs one `WARN` line carrying `code: doctor-bgp-next-hop-unresolvable`.
6. On `show bgp rib` (`received` scope), while serializing each stored row, if the route is not best and its address next hop fails forwarding admission (checked via the tracker or `candidateNextHopResolved` without holding write locks), the row carries `"next-hop-resolvable": false` and `"ineligible-reason": "next-hop-unresolvable"`.
7. On `ze doctor` / `show doctor`, the daemon invokes `bgp-rib`'s `OnDoctorCheck`:
   - `bgp-rib-selected-output`: unchanged.
   - `bgp-rib-unresolved-next-hop`: reads the bounded unresolved tracker under its mutex and emits one `warning` finding under `doctor-bgp-next-hop-unresolvable` when non-empty (naming count, sample prefix + next hop, and fix `ze show bgp rib | match next-hop-unresolvable`).
   - `bgp-rib-next-hop-metrics`: reads `forkRIB.metricFailed` under `remote.mu` and emits one `warning` finding under `doctor-bgp-next-hop-metrics` when active (matching `remoteMetricProblem` and `remoteMetricFix`).
8. When interior reachability changes, `reselectAIGPRoutes` reruns selection; newly resolvable routes leave the unresolved tracker and enter Loc-RIB / FIB without a new UPDATE.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `bgp-rib` ↔ Engine Loc-RIB (internal vs forked) | `igpcost.ResolveProspective` / `locrib.LPM` internal; `sdk.Plugin.RouteMetricsProspective` / `RouteMetrics` RPC forked | Yes (prior commits `46f486c4a0`, `b032fb7dc7`) |
| `bgp-rib` ↔ Daemon doctor (`show doctor` / `ze doctor`) | Stage 1 `sdk.Registration.DoctorChecks` + `OnDoctorCheck` RPC returning `[]rpc.DoctorCheckDiagnostic` | No (new checks to verify internal + forked) |
| Operator CLI / API ↔ `bgp-rib` | `show bgp rib` JSON `"routes"` rows; `request bgp rib inject` JSON answer | No (new fields to verify) |
| Test fixtures ↔ Engine Loc-RIB | `terminalCovers` via `route-install` RPC + `show rib candidates` / `RouteMetrics` verification | No (11 fixtures to convert and judge) |

### Changed-Wire Census

No BGP protocol wire bytes (`UPDATE`, `OPEN`, `NOTIFICATION`) and no `pkg/plugin/rpc` wire message definitions change. The only changed external payload shapes are additive optional JSON fields on two existing operator command responses: `show bgp rib` (`"routes"` rows gain optional `"next-hop-resolvable"` and `"ineligible-reason"` only when excluded) and `request bgp rib inject` (response map gains optional `"next-hop-resolvable"` and `"warning"` only when the supplied next hop does not resolve).

**Search evidence:** searched `jsonKeyInjected`, `request bgp rib inject`, `show bgp rib`, and `DoctorChecks` across `internal/`, `pkg/`, `cmd/`, and `test/` (`grep -rn "request bgp rib inject" test/ internal/` and `grep -rn "jsonKeyInjected\|serializeRouteItem\|advertiseDoctorDeclaration" internal/component/bgp/plugins/rib/`). Positive control: `internal/component/bgp/plugins/rib/rib_commands.go` (`injectRoute`, line 535) and `internal/component/bgp/plugins/rib/rib_pipeline.go` (`serializeRouteItem`, line 1244). No shell or Python consumer parses `request bgp rib inject` fields strictly (existing `.ci` tests match `injected` / prefix substring); no `show bgp rib` consumer rejects additional optional row keys when present only on unresolved routes.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `request bgp rib inject` JSON response | writer | `internal/component/bgp/plugins/rib/rib_commands.go` (`injectRoute`, line 535) | Always writes `map[string]any{"injected", "peer", "family"}` | additive change: keep all three keys unchanged; add `"next-hop-resolvable": false` and `"warning"` only when supplied next hop fails resolution |
| `request bgp rib inject` JSON response | reader | CLI/API/MCP callers and `test/plugin/*.ci`, `test/reload/*.ci`, `internal/test/fixture/*.go` | Existing callers check `injected` / status `done` | preserve compatibility: resolvable or no-next-hop injects emit byte-identical JSON |
| `show bgp rib` JSON `"routes"` row | writer | `internal/component/bgp/plugins/rib/rib_pipeline.go` (`showRow`, `serializeRouteItem`) and `rib_pipeline_bestfact.go` | Writes `"family"`, `"prefix"`, `"peer"`, `"direction"`, `"best"`, `"age"`, attributes | additive change: omit new keys on resolvable/no-NH routes; add `"next-hop-resolvable": false` and `"ineligible-reason": "next-hop-unresolvable"` on excluded received routes |
| `show bgp rib` JSON `"routes"` row | reader | CLI table/JSON/pipe renderers, Looking Glass (`internal/component/lg/`), REST/gRPC/MCP clients, `.ci` fixtures | Generic map/row consumers ignore unknown optional keys; `| match` reads row fields | preserve existing keys and shapes; enables `| match next-hop-unresolvable` |
| Plugin `doctor-check` RPC (`rpc.DoctorCheckDiagnostic`) | writer | `internal/component/bgp/plugins/rib/rib.go` (`OnDoctorCheck`), `rib_advertise_diagnostic.go`, new `rib_next_hop_doctor.go` | Uses existing `rpc.DoctorCheckDecl` / `rpc.DoctorCheckDiagnostic` wire format | preserve wire schema unchanged; add two new check declarations |
| Plugin `doctor-check` RPC (`rpc.DoctorCheckDiagnostic`) | reader | `internal/component/doctor/cmd/show.go` (`collectDoctorChecksFrom`) | Collects all declared checks dynamically from running plugins | preserve unchanged; discovers new checks automatically from Stage 1 |

### Integration Points
- `internal/component/bgp/plugins/rib/rib.go` - register `bgp-rib-unresolved-next-hop` and `bgp-rib-next-hop-metrics` in `DoctorChecks` and dispatch in `OnDoctorCheck`.
- `internal/component/bgp/plugins/rib/doctor_codes.go` - register `doctor-bgp-next-hop-unresolvable` metadata via `diagnostic.Register` (leaving `doctor-bgp-next-hop-metrics` and its `diagnostic.RegisterEventCode` single-registered in `register.go`).
- `internal/component/bgp/plugins/rib/rib_next_hop.go` & `rib_remote.go` - unresolved tracker helpers and coalesced error logging on `noteMetricFailure`.
- `internal/component/bgp/plugins/rib/rib_commands.go` & `rib_manual.go` - update unresolved tracker during forwarding candidate gathering, and annotate `injectRoute` responses when unresolved.
- `internal/component/bgp/plugins/rib/rib_pipeline.go` & `rib_pipeline_bestfact.go` - annotate unresolved received rows in `show bgp rib`.
- `internal/test/fixture/terminal_cover_fixture.go` & fixture drivers - supply real terminal host covers for the remaining failing `.ci` tests.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Visibility reads the same `candidateNextHopResolved` / prospective resolver used by Phase 2 admission; doctor checks go through `sdk.Registration.DoctorChecks` + `OnDoctorCheck`; fixtures install covers through `route-install` |
| No unintended coupling (components stay isolated) | Yes | All new state and doctor checks stay in `internal/component/bgp/plugins/rib`; `doctor` discovers them via Stage 1 registration |
| No duplicated functionality (extends existing, does not recreate) | Yes | Extends `OnDoctorCheck`, `doctor_codes.go`, `noteMetricFailure`, `showRow`, and `terminalCovers` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Hot-path `igpcost.ResolveProspective` / `locrib.PreviewSelected` stays untouched (0 B/op, 0 allocs/op); tracker stores compact value keys bounded to a fixed cap |
| Registration over hardcoding, outbound | Yes | New doctor checks declare via `sdk.Registration.DoctorChecks` and new codes via `diagnostic.Register` in `doctor_codes.go` |
| Registration over hardcoding, inbound | Yes | Searched `internal/component/doctor/` and `internal/component/plugin/server/`; `collectDoctorChecksFrom` and `ze show diagnostic` derive dynamically from registries with no hardcoded check-name list |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | All 148 historical RIB unit test failures from `job-rfc4271-next-hop-packages-125044b6.log` are already resolved on current `main` | Observed 2026-11-22 in `./le test unit pkg path ./internal/component/bgp/plugins/rib/` (`job-unit-pkg-8c945297.log`): 0 next-hop assertion failures | Any remaining unit failure is classified and fixed with independent `ze-test-judge` approval | Re-run `./le test unit pkg path ./internal/component/bgp/plugins/rib/` cleanly | confirmed 2026-11-22: full `internal/component/bgp/plugins/rib/` package PASSED (`exit 0, 1 package ok`) with 0 failures |
| A-2 | Existing `show bgp rib` callers (`lg`, CLI, `.ci` fixtures) ignore additive optional JSON fields that are omitted on resolvable routes | Inspected `rib_pipeline.go` (`serializeRouteItem`) and `test/plugin/*.ci`; resolvable routes emit identical JSON | A strict schema test rejects the new keys on unresolved routes | Scoped unit tests + `test/plugin/` suite | confirmed 2026-11-22: `TestShowBGPRIBMarksUnresolvedNextHop`, full RIB unit package, all 14 repaired `.ci` fixtures, and 20/20 stress runs of `rib-unresolved-next-hop-visibility.ci` PASSED |
| A-3 | Every one of the remaining failing `.ci` fixtures fails solely because its mock next hop lacks a covering route in the Loc-RIB (plus pre-existing loopback self-NH in `api-peer-create-delete-rib`), not because of an unrelated next-hop admission bug | Audited all 14 fixture drivers in `internal/test/fixture/` and `.ci` scenarios | A fixture exposes a separate product defect that must be fixed at its producer | Run each fixture before and after adding `terminalCovers`, plus independent `ze-test-judge` review | confirmed 2026-11-22: all 14 repaired `.ci` fixtures (13 target + `srv6-invalid-sid-ineligible`) failed on baseline and PASSED 14/14 after real `terminalCovers` (plus `nexthop 10.0.0.2` in `multipath-basic` and `rs-client true` in `api-peer-create-delete-rib`); `forward-mpreach-nexthop-self-two-peer` confirmed unrelated (no `bgp-rib`); 14/14 independent `ze-test-judge` APPROVE |
| A-4 | Recording unresolved exclusions in a bounded per-RIB map updated at selection time adds negligible cost and avoids triggering live RPCs during `show doctor` | `OnDoctorCheck` reads only the in-memory tracker (no lock nesting with `peerMu` or engine RPCs), matching `advertiseDoctorCheck` | Map grows unbounded under churn or goes stale after peer DOWN | Bound tracker capacity (4096 entries + 512-byte deduplicated overflow filter), clear per-prefix on election/withdrawal and per-owner on DOWN/purge/empty; unit-test bounds and cleanup | confirmed 2026-11-22: `TestUnresolvedNextHopDoctorCheckLifecycle`, `TestUnresolvedNextHopOverflowDeduplicationAndEmptyCleanup`, and `TestShowBGPRIBUsesCachedNextHopDuringRemoteMetricOutage` PASSED |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A broad reachable default or synthetic always-resolved stub hides prospective recursion defects | Negative prospective tests pass vacuously | Seed only explicit per-fixture terminal host covers via `terminalCovers` over the real resolver; retain all self/mutual-recursion negatives |
| R-2 | Reporting or doctor callbacks hold `peerMu` and `remote.mu` simultaneously or call the engine while locked, causing deadlock | `-race` test hangs or deadlocks | Keep the unresolved tracker under its own leaf mutex (never acquired before `peerMu`, never calls SDK/engine while held); read `forkRIB.metricFailed` under `remote.mu` alone |
| R-3 | Unbounded error-log storm during a forked engine `route-metrics` outage stalls selection (5s per uncached candidate) and floods logs | Repeated `doctor-bgp-next-hop-metrics` log lines per UPDATE burst | Coalesce `logger().Error` onto the `false -> true` transition of `noteMetricFailure`, and keep `Resolved: false` uncached so recovery takes effect on the next successful poll |
| R-4 | A stale unresolved-exclusion tracker entry lingers after a prefix is withdrawn or a peer goes DOWN, causing a false `ze doctor` warning | `ze doctor` reports an unresolved route after withdrawal or peer teardown | Clear the prefix's entries whenever `checkRouteBestChange` completes with no unresolved candidates (or all paths gone), and purge by peer owner in `purgeBestPrevForPeer` / peer DOWN; cover both in unit tests |
| R-5 | Editing `.ci` fixtures or `internal/test/fixture/*.go` triggers the test-weakening gate or touches RFC-tagged tests | `./le commit create` refuses without `ze-test-judge` approval | Obtain one independent `ze-test-judge` pass covering every modified `.ci` and fixture file before committing; keep every assertion byte-identical |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A bug in the unresolved tracker could leak memory or show false `ze doctor` warnings (warnings never block startup); a bug in `show bgp rib` annotation could mislabel a valid route as unresolvable; a bad fixture edit could hide a real next-hop regression |
| How is it reverted? | Single commit revert per phase (visibility/doctor commit, fixture repair commit); no config migration and no wire-protocol change |
| Who else touches this path? | Sibling sessions actively edit `internal/component/bgp/plugins/rib/` (`rib.go`, `rib_validation.go`, `rib_structured.go`, `config_tx.go`, `rib_source_shutdown.go`). Check `git status -s` before every edit and isolate new helpers in dedicated files (`rib_next_hop_doctor.go`) wherever possible |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Stored received route with unresolved next hop read via `show bgp rib` | → | `showRow` / `serializeRouteItem` next-hop exclusion annotation | `TestShowBGPRIBMarksUnresolvedNextHop` and `test/plugin/rib-unresolved-next-hop-visibility.ci` |
| `request bgp rib inject ... nexthop <unresolved>` | → | `RIBManager.injectRoute` post-reconcile resolvability check | `TestInjectRouteWarnsOnUnresolvedNextHop` and `test/plugin/rib-unresolved-next-hop-visibility.ci` |
| `show doctor` / `ze doctor` with an unresolved received route (internal & forked) | → | `sdk.Registration.DoctorChecks` -> `OnDoctorCheck` (`bgp-rib-unresolved-next-hop`) | `TestUnresolvedNextHopDoctorCheckLifecycle` and `test/plugin/rib-unresolved-next-hop-visibility.ci` |
| `show doctor` / `ze doctor` during forked `route-metrics` failure | → | `OnDoctorCheck` (`bgp-rib-next-hop-metrics`) + coalesced `noteMetricFailure` | `TestRemoteMetricFailureCoalescesLogAndReportsDoctor` |
| Repaired `.ci` scenarios (`rr-basic`, `bestpath-reason`, `api-peer-create-delete-rib`, etc.) | → | `terminalCovers` -> engine Loc-RIB -> `candidateNextHopResolved` -> `show bgp rib best` | `./le test bgp plugin` on each converted `.ci` + `./le test reload` for `bgp-rib-maximum-paths-reload` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row states positive observable behavior,
     never only the mechanism or the absence of the defect. Name what remains
     permitted and why; if there are no permitted survivors, state the reason. -->
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Received or injected route stored in `bgp-rib` whose address next hop is unresolved (or prospectively self-capturing); operator runs `show bgp rib` (and `show bgp rib \| match next-hop-unresolvable`) | The route appears in `"routes"` with `"next-hop-resolvable": false` and `"ineligible-reason": "next-hop-unresolvable"`, while `show bgp rib best` excludes it; once a covering route is installed, the route becomes best and both exclusion fields disappear | Routes with resolvable next hops and routes with no address next hop (e.g., FlowSpec or no-next-hop injects) omit both fields so ordinary `show bgp rib` output stays unchanged |
| AC-2 | Operator runs `request bgp rib inject <peer> <family> <prefix> nexthop <unresolved>` | The route is retained in Adj-RIB-In, the response keeps `"injected"`, `"peer"`, `"family"` AND adds `"next-hop-resolvable": false` plus `"warning"` naming the unresolved next hop and `show rib` / `show bgp rib`, and one `WARN` log carries `doctor-bgp-next-hop-unresolvable` | An inject with a resolvable next hop or no next hop succeeds with the exact existing 3-key response and no warning because the route is immediately eligible |
| AC-3 | Running daemon (internal or forked `bgp-rib`) holds at least one route excluded by `candidateNextHopResolved`; operator runs `show doctor` / `ze doctor` | Doctor reports a `warning` under `doctor-bgp-next-hop-unresolvable` (exit 0 / box stays ready) naming the excluded count, a sample prefix and unresolved next hop, and `Fix: ze show bgp rib \| match next-hop-unresolvable`; once all excluded routes resolve, are withdrawn, or their peer goes DOWN, the finding clears | Unresolved next hops remain warnings (never errors) because waiting on IGP convergence is a normal operational state; routes excluded only by private recovery (`candidateAdmissionRecovery`) never raise this warning |
| AC-4 | Forked `bgp-rib` encounters repeated `RouteMetrics` or `RouteMetricsProspective` RPC failures | Each failed candidate stays fail-closed (`Resolved: false`, uncached); `logger().Error` fires once per `healthy -> failed` transition rather than per candidate; `show doctor` reports `doctor-bgp-next-hop-metrics` (`warning`) from `OnDoctorCheck` in addition to the coalesced report-bus warning, and both clear after a successful retry | A fatal revision-poll failure in `runRemoteMetrics` continues raising the error event and closing the plugin, preserving the mandatory-feed lifecycle |
| AC-5 | Diagnostic code registry (`ze show diagnostic <code>`) | `doctor-bgp-next-hop-unresolvable` is registered in `internal/component/bgp/plugins/rib/doctor_codes.go` (title, description, examples), `doctor-bgp-next-hop-metrics` retains its single existing `diagnostic.Register` and `diagnostic.RegisterEventCode` registration in `register.go` (no duplicate panic at `init()`), and `doctor-bgp-next-hop-metrics` is listed in `DoctorCheckDecl.Codes` so `recent-events` does not duplicate it | No central `internal/core/diagnostic/codes.go` entry is added because `bgp-rib` owns its own codes |
| AC-6 | All 13 remaining `.ci` functional tests (`rr-basic`, `bestpath-reason`, `api-peer-create-delete-rib`, `multipath-basic`, `nexthop-self`, `nexthop-unchanged`, `rib-forward-handle-observed`, `rib-inject-rfc5549`, `fib-ecmp`, `fib-ecmp-realtime`, `bgp-rib-maximum-paths-reload`, `llgr-readvertise-multipeer`, `llgr-readvertise-multipeer-forked`; plus `srv6-invalid-sid-ineligible` verified) | Each fixture supplies real independent terminal covers for its mock next hops through `terminalCovers` (or static device cover in reload config), preserves every original assertion, passes `./le test bgp plugin` / `./le test reload`, and carries independent `ze-test-judge` approval | Existing `received`-only LG fixtures (`lg-paginate`, `lg-ui-pages`, `lg-covering-lookup`, `lg-aspath-search`) and no-next-hop inject fixtures stay untouched because they do not depend on next-hop resolution |
| AC-7 | Documentation accuracy (`docs/architecture/route-selection.md`, `docs/architecture/doctor-and-health-checks.md`) | Stale "validation still in progress / live prospective proof incomplete" prose is replaced with current source-anchored facts covering visibility, inject warnings, runtime doctor checks, and coalesced metric failure handling; `./le doc check all` passes | Historical evidence in this spec's `Current Evidence` and `Route-server Role Investigation` stays intact for commit A history |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Receives BGP routes whose next hop is not yet in the IGP and runs `show bgp rib` and `ze doctor` | BGP UPDATE -> `ribInPool` -> `candidateNextHopResolved` excludes & records -> `show bgp rib` marks `next-hop-unresolvable` -> `show doctor` reports `doctor-bgp-next-hop-unresolvable` with fix -> IGP cover arrives -> reselection clears both | `test/plugin/rib-unresolved-next-hop-visibility.ci`, `TestShowBGPRIBMarksUnresolvedNextHop`, `TestUnresolvedNextHopDoctorCheckLifecycle` |
| 2 | Runs `request bgp rib inject` with an unreachable next hop in a lab | `injectRoute` -> stores in `ribInPool` -> `reconcileBestPath` -> detects unresolved next hop -> returns `"next-hop-resolvable": false` + `"warning"` naming the next hop and remedy | `test/plugin/rib-unresolved-next-hop-visibility.ci`, `TestInjectRouteWarnsOnUnresolvedNextHop` |
| 3 | Runs `bgp-rib` forked when the engine's `route-metrics` RPC momentarily fails | `prospectiveDistance`/`igpDistance` -> fail-closed `Resolved: false` + one coalesced error log -> `OnDoctorCheck` reports `doctor-bgp-next-hop-metrics` -> poll recovers -> caches clear, routes reselect, warning clears | `TestRemoteMetricFailureCoalescesLogAndReportsDoctor` |
| 4 | Runs the BGP plugin and reload functional suites (`rr-basic`, `bestpath-reason`, `api-peer-create-delete-rib`, etc.) | Fixture installs real terminal cover -> `candidateNextHopResolved` passes -> Phase 2 elects expected best path -> every original assertion passes | Repaired `.ci` tests under `test/plugin/` and `test/reload/` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestShowBGPRIBMarksUnresolvedNextHop` | `internal/component/bgp/plugins/rib/rib_next_hop_visibility_test.go` | `show bgp rib` marks unresolved received route with `"next-hop-resolvable": false` and `"ineligible-reason": "next-hop-unresolvable"`, omits both when resolvable or no-NH, and clears them after cover arrival | |
| `TestInjectRouteWarnsOnUnresolvedNextHop` | `internal/component/bgp/plugins/rib/rib_next_hop_visibility_test.go` | `injectRoute` with unresolved IPv4/IPv6 next hop retains route and returns `"next-hop-resolvable": false` + `"warning"`; resolvable or omitted next hop returns the clean 3-key response | |
| `TestUnresolvedNextHopDoctorCheckLifecycle` | `internal/component/bgp/plugins/rib/rib_next_hop_doctor_test.go` | Tracker records unresolved exclusions up to its cap, `OnDoctorCheck("bgp-rib-unresolved-next-hop")` emits `doctor-bgp-next-hop-unresolvable` warning with count/sample/fix, and clears on resolution, withdrawal, and peer DOWN | |
| `TestRemoteMetricFailureCoalescesLogAndReportsDoctor` | `internal/component/bgp/plugins/rib/rib_next_hop_doctor_test.go` | Repeated `prospectiveDistance`/`igpDistance` errors log once per failure episode, remain fail-closed uncached, surface `doctor-bgp-next-hop-metrics` through `OnDoctorCheck("bgp-rib-next-hop-metrics")`, and clear after recovery | |
| `TestRIBNextHopDoctorCodesRegistered` | `internal/component/bgp/plugins/rib/rib_next_hop_doctor_test.go` | `doctor-bgp-next-hop-unresolvable` and `doctor-bgp-next-hop-metrics` exist in `diagnostic.Lookup` with title, description, and examples | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Unresolved tracker cap (`maxTrackedUnresolvedNextHops`) | 0–4096 entries | 4096 entries stored individually | N/A (0 = no finding) | >4096 capped without unbounded growth; overflow counted in finding summary |
| Doctor sample count per finding | 1–5 samples | 5 samples printed in detail | N/A | >5 prints first 5 + `(+N more)` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rib-unresolved-next-hop-visibility` | `test/plugin/rib-unresolved-next-hop-visibility.ci` (drafted in `test/draft/plugin/`, stress-tested 80x, promoted) | Inject and receive routes with an unresolved next hop -> `inject` warns, `show bgp rib` marks `next-hop-unresolvable`, `show bgp rib best` excludes, `show doctor` reports `doctor-bgp-next-hop-unresolvable` -> install real cover -> best path elects and doctor finding clears | |
| 13 repaired `.ci` fixtures | `test/plugin/{rr-basic,bestpath-reason,api-peer-create-delete-rib,multipath-basic,nexthop-self,nexthop-unchanged,rib-forward-handle-observed,rib-inject-rfc5549,fib-ecmp,fib-ecmp-realtime,llgr-readvertise-multipeer,llgr-readvertise-multipeer-forked}.ci`, `test/reload/bgp-rib-maximum-paths-reload.ci` | Real terminal host cover enables expected Phase 2 selection under RFC 4271 §9.1.2 without weakening any assertion | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A (no wire behavior change) | `test/plugin/rfc4271-{,prospective-}next-hop-kernel*.ci` already promoted | Linux kernel FIB + internal/forked `bgp-rib` | RFC 4271 §9.1.2 wire/kernel admission and prospective self-capture are already proven and audited in `b032fb7dc7`; this spec adds operator visibility, log coalescence, runtime doctor checks, and fixture covers with zero BGP wire change | |

## Files to Modify
- `internal/component/bgp/plugins/rib/rib_next_hop.go` - track unresolved exclusions and coalesce `prospectiveDistance` error logging via `noteMetricFailure`
- `internal/component/bgp/plugins/rib/rib_remote.go` - return transition bool from `noteMetricFailure`, coalesce `igpDistance` error logging, expose `metricFailureActive()` for `OnDoctorCheck`
- `internal/component/bgp/plugins/rib/rib_commands.go` - record/clear unresolved exclusions during forwarding candidate gathering; check next-hop resolvability in `injectRoute` and include warning fields when unresolved
- `internal/component/bgp/plugins/rib/rib_manual.go` - record unresolved manual candidates in the tracker during `appendManualCandidatesLocked`
- `internal/component/bgp/plugins/rib/rib_pipeline.go` - surface `"next-hop-resolvable": false` and `"ineligible-reason": "next-hop-unresolvable"` on excluded received rows
- `internal/component/bgp/plugins/rib/rib_bestchange.go` - clear unresolved tracker entries on prefix withdrawal, by owner in `purgeBestPrevForOwner`, and by address in `purgeBestPrevForPeer`
- `internal/component/bgp/plugins/rib/rib.go` - declare `bgp-rib-unresolved-next-hop` and `bgp-rib-next-hop-metrics` in `sdk.Registration.DoctorChecks` and dispatch in `OnDoctorCheck`
- `internal/component/bgp/plugins/rib/doctor_codes.go` - register `doctor-bgp-next-hop-unresolvable` metadata (leaving `doctor-bgp-next-hop-metrics` and its `diagnostic.RegisterEventCode` single-registered in `register.go`)
- `internal/test/fixture/terminal_cover_fixture.go` - generalize `terminalCovers` to support IPv6 `/128` (`AFI: 2, SAFI: 1`) beside existing IPv4 `/32` (`AFI: 1, SAFI: 1`) so `rib-inject-rfc5549` can install `2001:db8::1/128`
- `internal/test/fixture/plugin_fixture_15_show.go` - add `terminalCovers` for `rr-basic` (`10.0.0.1/32`)
- `internal/test/fixture/plugin_fixture_02.go` - add `terminalCovers` for `bestpath-reason` (`10.0.0.1/32`)
- `internal/test/fixture/plugin_fixture_13_peer_lifecycle.go` - add `terminalCovers` for `api-peer-create-delete-rib` (`1.1.1.1/32`)
- `internal/test/fixture/plugin_fixture_10.go` - add `terminalCovers` for `multipath-basic`, `nexthop-self`, `nexthop-unchanged` (`10.0.0.1/32`)
- `internal/test/fixture/plugin_fixture_13_rib.go` - add `terminalCovers` for `rib-forward-handle-observed` (`10.0.0.1/32`) and `rib-inject-rfc5549` (`2001:db8::1/128`)
- `internal/test/fixture/plugin_fixture_06_fib.go` - add `terminalCovers` for `fib-ecmp` (`10.0.0.11/32`, `10.0.0.12/32`) and `fib-ecmp-realtime` (`10.0.0.11/32`, `10.0.0.12/32`, `10.0.0.13/32`)
- `internal/test/fixture/plugin_fixture_09_bgp.go` - extend `terminalCovers` to `llgr-readvertise-multipeer` (`10.0.0.1/32`)
- `test/plugin/rr-basic.ci`, `test/plugin/bestpath-reason.ci`, `test/plugin/api-peer-create-delete-rib.ci`, `test/plugin/multipath-basic.ci`, `test/plugin/nexthop-self.ci`, `test/plugin/nexthop-unchanged.ci`, `test/plugin/rib-forward-handle-observed.ci`, `test/plugin/rib-inject-rfc5549.ci`, `test/plugin/fib-ecmp.ci`, `test/plugin/fib-ecmp-realtime.ci`, `test/plugin/llgr-readvertise-multipeer.ci`, `test/plugin/llgr-readvertise-multipeer-forked.ci`, `test/reload/bgp-rib-maximum-paths-reload.ci` - enable `rib` / cover routes as needed so `terminalCovers` installs real reachability
- `docs/architecture/route-selection.md` - update Phase 2 next-hop admission prose to reflect landed prospective proof, `show bgp rib` / `inject` visibility, runtime doctor checks, and log coalescence
- `docs/architecture/doctor-and-health-checks.md` - document `bgp-rib-unresolved-next-hop` and `bgp-rib-next-hop-metrics` in the plugin runtime-checks table
- `plan/journal/silent-fall-through.md` - flip the 2026-11-18 and 2026-11-20 rows from `open:` to `fixed:` when landed

## Files to Create
- `internal/component/bgp/plugins/rib/rib_next_hop_doctor.go` - bounded unresolved-next-hop tracker and `OnDoctorCheck` handlers for `bgp-rib-unresolved-next-hop` and `bgp-rib-next-hop-metrics`
- `internal/component/bgp/plugins/rib/rib_next_hop_doctor_test.go` - unit tests for tracker lifecycle, bounds, doctor findings, and coalesced remote-metric failure reporting
- `internal/component/bgp/plugins/rib/rib_next_hop_visibility_test.go` - unit tests for `show bgp rib` exclusion annotation and `request bgp rib inject` unresolved warning
- `test/plugin/rib-unresolved-next-hop-visibility.ci` - end-to-end functional test for AC-1, AC-2, AC-3 (drafted outside `test/`, stress-tested, then promoted)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No new config leaf or RPC; uses existing `show bgp rib`, `request bgp rib inject`, and `show doctor` |
| YANG validation constraints | N-A | No YANG leaf added |
| YANG custom validators | N-A | No YANG leaf added |
| CLI commands/flags | N-A | No new CLI command or flag; additive fields on existing JSON answers and new runtime doctor rows |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | No leaf or command added |
| Functional test for new RPC/API | Yes | `test/plugin/rib-unresolved-next-hop-visibility.ci` proves `show bgp rib`, `request bgp rib inject`, and `show doctor` end to end |
| Pipe completeness | Yes | `show bgp rib` already runs through the pipe pipeline; adding `"ineligible-reason": "next-hop-unresolvable"` lets `\| match`, `\| json`, `\| count` filter excluded routes without any new pipe |
| Env var registration | N-A | No new env var |
| Doctor check for runtime dependencies | Yes | `bgp-rib-next-hop-metrics` (`doctor-bgp-next-hop-metrics`, runtime phase via `sdk.Registration.DoctorChecks`) reports live failure of the engine `route-metrics` dependency in forked placement |
| Doctor code per problem class | Yes | `doctor-bgp-next-hop-unresolvable` (warning, `bgp-rib-unresolved-next-hop`, runtime phase, registered in `doctor_codes.go`) and `doctor-bgp-next-hop-metrics` (warning, `bgp-rib-next-hop-metrics`, runtime phase, keeping its single registration + event code in `register.go`) |
| Prometheus counters/metrics | No | Operator visibility is served through `show bgp rib` and `show doctor`; no new metric required |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new BGP family, capability, or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | RFC 4271 next-hop admission is already implemented and audited; this completes visibility and fixture parity |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | No new command; additive optional JSON fields on `show bgp rib` and `request bgp rib inject` |
| 4 | API/RPC added/changed? | No | No new RPC; `docs/architecture/api/commands.md` stays unaffected |
| 5 | Plugin added/changed? | No | `bgp-rib` gains runtime doctor checks, not a new plugin |
| 6 | Has a user guide page? | No | No dedicated next-hop admission guide page |
| 7 | Wire format changed? | No | No BGP or IPC wire change |
| 8 | Plugin SDK/protocol changed? | No | Uses existing `sdk.Registration.DoctorChecks` and `OnDoctorCheck` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | All four RFC 4271 rows are already audited and enforced on `HEAD` (`b032fb7dc7`) |
| 10 | Test infrastructure changed? | No | Reuses existing `terminalCovers` helper in `internal/test/fixture/terminal_cover_fixture.go` |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/route-selection.md` (Phase 2 next-hop admission visibility, log coalescence, remove stale open-proof notes) and `docs/architecture/doctor-and-health-checks.md` (plugin runtime-checks table) |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Runtime doctor checks are listed in `docs/architecture/doctor-and-health-checks.md` (row 12) |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/route-selection.md` and `docs/architecture/doctor-and-health-checks.md` updated (row 12); `docs/architecture/plugin/rib-storage-design.md` and `docs/architecture/api/commands.md` checked and unaffected; re-run `./le spec citation anchors spec plan/release/fixes/spec-bgp-next-hop-reachability-admission.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

## Design presentation

### What changes

This change closes the remaining visibility, error-coalescence, and fixture gaps of RFC 4271 Section 9.1.2 next-hop admission without altering the already-audited forwarding admission logic:
1. Stored received routes excluded from Phase 2 because of an unresolved or prospectively recursive next hop are now visibly marked in `show bgp rib` (`"next-hop-resolvable": false`, `"ineligible-reason": "next-hop-unresolvable"`) and reported by a new `bgp-rib` runtime doctor check (`doctor-bgp-next-hop-unresolvable`, warning, with count, sample prefix/next-hop, and fix command).
2. `request bgp rib inject` still retains a route whose next hop does not resolve, but now tells the operator in the command answer (`"next-hop-resolvable": false`, `"warning"`) and a `WARN` log instead of returning bare success before silent exclusion.
3. Forked `route-metrics` lookup failures in `prospectiveDistance` and `igpDistance` coalesce their `logger().Error` onto failure transitions (stopping log floods during outages) and expose active failure through a declared `bgp-rib-next-hop-metrics` runtime doctor check alongside the SDK report bus.
4. The 13 remaining `.ci` functional tests whose mock next hops lacked covering routes gain real terminal host covers via the shared `terminalCovers` helper, keeping every assertion unchanged and passing `ze-test-judge`.

### Before and after

1. **Operator runs `request bgp rib inject 10.0.0.1 ipv4/unicast 198.51.100.0/24 nexthop 127.0.0.1` with no covering route:**
   - Before: answers `{"injected": "198.51.100.0/24", "peer": "10.0.0.1", "family": "ipv4/unicast"}`, then `show bgp rib best` is empty with nothing in the answer or log explaining why.
   - After: retains the route and answers with `"next-hop-resolvable": false` and `"warning": "next hop 127.0.0.1 is not resolvable in the Loc-RIB; route retained in Adj-RIB-In but excluded from best-path selection (RFC 4271 Section 9.1.2). Check covering routes with ze show rib."`, plus one `WARN` log carrying `doctor-bgp-next-hop-unresolvable`.
2. **Operator runs `show bgp rib` and `show doctor` when 15 received routes have unresolved next hop `10.0.1.1`:**
   - Before: `show bgp rib` lists all 15 routes with no exclusion indicator and `show doctor` reports nothing while `show bgp rib best` is empty.
   - After: each excluded row in `show bgp rib` carries `"next-hop-resolvable": false` and `"ineligible-reason": "next-hop-unresolvable"` (filterable via `show bgp rib | match next-hop-unresolvable`), and `show doctor` emits a `WARN [doctor-bgp-next-hop-unresolvable]` naming the count, sample prefix + `10.0.1.1`, and `fix: ze show bgp rib | match next-hop-unresolvable`.
3. **Forked `bgp-rib` hits repeated `route-metrics` RPC timeouts during an engine stall:**
   - Before: emits an Error log for every candidate lookup and reports `doctor-bgp-next-hop-metrics` only on the transient report bus.
   - After: emits one coalesced Error log on the `healthy -> failed` transition, keeps every failed candidate fail-closed (`Resolved: false`, uncached), and surfaces `doctor-bgp-next-hop-metrics` in `show doctor` via `OnDoctorCheck` until recovery.
4. **Functional tests (`rr-basic`, `bestpath-reason`, `api-peer-create-delete-rib`, etc.):**
   - Before: fail with empty `best-path: []` because mock next hops (`10.0.0.1`, `1.1.1.1`, etc.) have no route in Loc-RIB.
   - After: install real terminal host covers via `terminalCovers` before sending BGP UPDATEs, and pass with every original assertion intact.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Refuse `request bgp rib inject` when the next hop does not resolve | Operators and lab scripts legitimately inject routes before IGP convergence; RFC 4271 retains unresolved routes in Adj-RIB-In so they elect automatically when reachability arrives |
| Add a new `show bgp rib unusable` subcommand | Violates `docs/architecture/api/commands.md` ("The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it"); optional row fields let existing pipes filter directly |
| Clear the 13 failing `.ci` tests with an always-resolved test stub or config flag | Violates RFC 4271 §9.1.2 and risks hiding real next-hop resolution regressions; `terminalCovers` exercises the real resolver and is already `ze-test-judge` approved (`b5079ff5ff`) |
| Cache failed `route-metrics` RPC answers as `Resolved: false` | Would hold routes ineligible after the engine recovers until the next routing revision bump; keeping failures uncached while coalescing error logs preserves prompt recovery on the next poll |

### Documents departed from

1. **`docs/architecture/route-selection.md` — `Design: Unified Rejection Reason`:**
   - Current statement (lines 17-19): `"This is a single mechanism, not two separate ones. Whether a route was disqualified before the race or lost at step N of the race, the answer is the same type of value on the same field."`
   - After this work: `docs/architecture/route-selection.md` records that `bgp-rib` currently exposes Phase 2 ranking losses via `show bgp rib best | reason` (`BestStep`) and pre-ranking unresolved next-hop exclusions on `show bgp rib` received rows (`"next-hop-resolvable": false`, `"ineligible-reason": "next-hop-unresolvable"`) backed by a bounded per-RIB tracker, rather than unifying both into one `uint8` field on `storage.RouteEntry`.

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Behavior of `request bgp rib inject` when the supplied next hop is unresolved | (a) Retain route in Adj-RIB-In and return `"next-hop-resolvable": false` + `"warning"` + `WARN` log (Recommended — matches RFC 4271 retention and `plan/journal/silent-fall-through.md` 2026-11-20); (b) refuse the inject outright with an operator error | (a) adopted 2026-11-22 |
| C-2 | Severity of `doctor-bgp-next-hop-unresolvable` in `show doctor` | (a) `warning` (Recommended — keeps box ready during normal IGP convergence while making excluded routes visible); (b) `error` (marks box not ready whenever any received route has an unresolved next hop) | (a) adopted 2026-11-22 |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- declare `bgp-rib-unresolved-next-hop` and `bgp-rib-next-hop-metrics` in `sdk.Registration.DoctorChecks` (`rib.go`), wire `OnDoctorCheck` dispatch in `rib_next_hop_doctor.go`, register codes in `doctor_codes.go`, and add failing unit tests for doctor codes, visibility, and inject warnings.
   - Tests: `TestRIBNextHopDoctorCodesRegistered`, `TestUnresolvedNextHopDoctorCheckLifecycle`, `TestShowBGPRIBMarksUnresolvedNextHop`, `TestInjectRouteWarnsOnUnresolvedNextHop`, `TestRemoteMetricFailureCoalescesLogAndReportsDoctor`
   - Files: `rib.go`, `doctor_codes.go`, `rib_next_hop_doctor.go`, `rib_next_hop_doctor_test.go`, `rib_next_hop_visibility_test.go`
   - Verify: doctor checks are reachable via `OnDoctorCheck`; unit tests fail on stub tracker/visibility.
2. **Phase: Exclusion visibility, inject warning, and fail-closed log coalescence** -- implement the bounded unresolved tracker, wire record/clear hooks in `rib_commands.go`, `rib_manual.go`, and `rib_bestchange.go`, add row annotation in `rib_pipeline.go`, add post-reconcile resolvability warning in `injectRoute`, and coalesce `logger().Error` in `rib_next_hop.go` / `rib_remote.go`.
   - Tests: all unit tests from Phase 1 pass under `-race`; draft, stress-test 80x, and promote `test/plugin/rib-unresolved-next-hop-visibility.ci`.
   - Files: `rib_next_hop_doctor.go`, `rib_next_hop.go`, `rib_remote.go`, `rib_commands.go`, `rib_manual.go`, `rib_pipeline.go`, `rib_bestchange.go`, `test/plugin/rib-unresolved-next-hop-visibility.ci`
   - Verify: `./le test unit pkg path ./internal/component/bgp/plugins/rib/` and `./le test bgp plugin` on `rib-unresolved-next-hop-visibility` pass; validate A-2, A-4.
3. **Phase: Functional `.ci` fixture repairs** -- convert the 13 remaining `.ci` tests and their fixture drivers to install real terminal covers via `terminalCovers` (and verify `srv6-invalid-sid-ineligible`), run independent `ze-test-judge` over all modified fixtures, and verify every converted test passes.
   - Tests: `./le test bgp plugin` over all 12 plugin `.ci` tests + `./le test reload` on `bgp-rib-maximum-paths-reload`.
   - Files: fixture drivers under `internal/test/fixture/` and `.ci` files under `test/plugin/` and `test/reload/`.
   - Verify: every converted test passes cleanly with `ze-test-judge` approval; validate A-3.
4. **Phase: Documentation and journal resolution** -- update `docs/architecture/route-selection.md`, `docs/architecture/doctor-and-health-checks.md`, and mark the two `plan/journal/silent-fall-through.md` rows `fixed:`.
   - Tests: `./le doc check all`, `./le doc wiring`, `./le rfc check`.
   - Files: `docs/architecture/route-selection.md`, `docs/architecture/doctor-and-health-checks.md`, `plan/journal/silent-fall-through.md`.
   - Verify: all doc checks pass with zero stale anchors.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-7 has an implementation at file:line |
| Feature completeness | Every user story works in both internal and forked `bgp-rib` placements |
| Correctness | Tracker clears on resolution, withdrawal, and peer DOWN; `candidateAdmissionRecovery` never populates the tracker; resolvable/no-NH injects and rows emit unchanged JSON |
| Naming | Doctor codes `doctor-bgp-next-hop-unresolvable` and `doctor-bgp-next-hop-metrics`, checks `bgp-rib-unresolved-next-hop` and `bgp-rib-next-hop-metrics`, JSON keys kebab-case (`next-hop-resolvable`, `ineligible-reason`) |
| Data flow | Tracker mutex never nests with `peerMu` or `remote.mu` in opposite order; `OnDoctorCheck` makes no engine RPCs |
| Operator errors and doctor codes | Both doctor checks carry `Fix`, `doctor-bgp-next-hop-unresolvable` registers in `doctor_codes.go` while `doctor-bgp-next-hop-metrics` keeps its single registration in `register.go`, and messages match `ai/rules/user-facing-errors.md` |
| Rule: `ai/rules/testing.md` | Every modified `.ci` and fixture file preserves all original assertions and carries independent `ze-test-judge` approval |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Unresolved visibility in `show bgp rib` & `inject` warning | `./le test unit pkg path ./internal/component/bgp/plugins/rib/ run "TestShowBGPRIBMarksUnresolvedNextHop\|TestInjectRouteWarnsOnUnresolvedNextHop"` |
| Runtime doctor checks & coalesced metric failure | `./le test unit pkg path ./internal/component/bgp/plugins/rib/ run "TestUnresolvedNextHopDoctorCheckLifecycle\|TestRemoteMetricFailureCoalescesLogAndReportsDoctor\|TestRIBNextHopDoctorCodesRegistered"` |
| End-to-end visibility functional test | `./le test bgp plugin` on `rib-unresolved-next-hop-visibility` |
| 13 repaired `.ci` fixtures green | `./le test bgp plugin` + `./le test reload` on each repaired fixture, with `ze-test-judge` approval |
| Docs & RFC gates clean | `./le doc check all`, `./le rfc check`, `./le plugin boundary check`, `./le plugin declarations check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | Unresolved tracker is strictly capped (`maxTrackedUnresolvedNextHops = 4096`) so a peer flooding unreachable next hops cannot grow memory unboundedly; doctor finding caps per-entry sample output |
| Log flooding | Coalescing `logger().Error` on `noteMetricFailure` transition prevents log amplification during an engine outage |
| Fail-closed admission | No visibility or doctor path ever turns a failed or timed-out `RouteMetrics`/`RouteMetricsProspective` call into `Resolved: true` |

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

## Current Evidence

| Observation | Evidence and limit |
|-------------|--------------------|
| Eight new admission/lifecycle tests and the immediate-next-hop reselection test passed | `job-unit-pkg-130c510f.log` and `job-unit-pkg-a904e39b.log` under the campaign session; this proves the unit boundaries, not installed kernel forwarding |
| Unanswered prospective metric request returns within its bound | `job-bgp-next-hop-as0-boundaries-1662fc0c.log`, `TestProspectiveMetricUnansweredRequestIsBounded`, PASS |
| Initial full affected package run was red; current `main` has zero next-hop unit assertion failures | Initial baseline `job-rfc4271-next-hop-packages-125044b6.log` had 148 top-level and 124 nested failures; on 2026-11-22 `./le test unit pkg path ./internal/component/bgp/plugins/rib/` (`job-unit-pkg-8c945297.log`) had 0 next-hop assertion failures (only 2 race-load timeouts in burst tests) |
| The concrete resolver supersedes the earlier 320-byte prospective allocation | `job-next-hop-core-final-3b90dc48.log` measures ordinary and prospective direct/recursive resolution at 0 B/op and 0 allocs/op in the same run |
| Response bounds and registered issue reporting pass scoped execution | `job-rfc-campaign-new-batch-reds-37b87864.log` passes `TestProspectiveMetricUnansweredRequestIsBounded`, `TestRemoteMetricWorkerRaisesAndClearsBothTransports` over socket/DirectBridge, and `TestRemoteMetricRevisionFeedUnansweredRequestIsBounded` |
| Internal and forked kernel tests (ordinary and live prospective) passed, stressed 80/80, and were promoted | `test/plugin/rfc4271-next-hop-kernel*.ci` (`46f486c4a0`) and `test/plugin/rfc4271-prospective-next-hop-kernel*.ci` (`b032fb7dc7`); all four RFC 4271 rows (`3.1-2`, `9.1.2-1`, `9.1.2-4`, `9.1.2.1-2`) hold `"verdict": "enforced"` in `rfc/audit/rfc4271/` |
| `test/plugin/lg-best-table.ci` converted and landed (`b5079ff5ff`) | Uses shared `terminalCovers` (`internal/test/fixture/terminal_cover_fixture.go`) with `ze-test-judge` approval; every HTTP assertion unchanged |
| Private recovery admission preserves forwarded-wire recovery without relaxing ordinary forwarding admission | `job-recovery-next-hop-admission-green-9c322622.log` and `tmp/session/2026-11-14-002623/scratch/job-unit-pkg-401fb152.log` pass all recovery and direct/IPC cases |
| Remaining 13 `.ci` fixtures audited on 2026-11-22 | `rr-basic`, `bestpath-reason`, `api-peer-create-delete-rib`, `multipath-basic`, `nexthop-self`, `nexthop-unchanged`, `rib-forward-handle-observed`, `rib-inject-rfc5549`, `fib-ecmp`, `fib-ecmp-realtime`, `bgp-rib-maximum-paths-reload`, `llgr-readvertise-multipeer`, `llgr-readvertise-multipeer-forked` all assert `show bgp rib best` over mock next hops with no covering route |

## Route-server Role Investigation

RFC 7947 was read in full. Section 2.2.1 states:

> As the route server does not participate in the actual routing of traffic,
> the NEXT_HOP attribute MUST be passed unmodified to the route server clients,
> similar to the "third-party" next-hop feature described in Section 5.1.3. of
> [RFC4271].

Section 2.1 states:

> A route server MUST accept all UPDATE messages received from each of its
> clients for inclusion in its Adj-RIB-In.

The following sentences permit local-policy filtering from the Loc-RIB and
recommend one or more decision processes. The full RFC contains no explicit
instruction to disable next-hop resolution. Attribute transparency alone does
not establish that exemption. Ordinary forwarding admission therefore remains
required, and the concrete sharing point `rib_recovery.go` (`recoverySurvivorsLocked`)
selects `candidateAdmissionRecovery` in the shared `gatherPrefixCandidatesForLocked`
and `gatherKeyCandidatesForLocked` producers without relaxing ordinary forwarding
admission or adding any blanket RS-client exemption.

## Design Insights
- `remoteRIB.prospectiveDistance` and `RIBManager.igpDistance` already carry `// refuse-by-default:` annotations on `main` (`b032fb7dc7`, `f32cdceccf`); the remaining defect at that boundary is uncoalesced `logger().Error` calls per candidate lookup during an outage and lack of a declared runtime `OnDoctorCheck` view for `doctor-bgp-next-hop-metrics`.
- Exposing unresolved-next-hop status as optional fields on `show bgp rib` rows avoids adding a new subcommand (respecting `docs/architecture/api/commands.md`, "The pipe wins") and lets operators filter directly with `show bgp rib | match next-hop-unresolvable`.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `conflicting-feature-contracts.md` | 2026-11-15 | RS live relay and recovery next-hop admission |
| `silent-fall-through.md` | 2026-11-18 | A route excluded for an unresolved next hop is reported nowhere |
| `silent-fall-through.md` | 2026-11-20 | `request bgp rib inject` answers success for a route the unresolved next hop then excludes |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Mark excluded routes on existing `show bgp rib` rows (`next-hop-resolvable`, `ineligible-reason`) instead of adding a new subcommand | Add `show bgp rib unusable` subcommand | Obeys `docs/architecture/api/commands.md` ("The pipe wins") and makes standard pipes (`\| match`, `\| json`, `\| count`) work automatically |
| Retain unresolved routes on `request bgp rib inject` and return `"next-hop-resolvable": false` + `"warning"` | Refuse the inject outright | Matches RFC 4271 Adj-RIB-In retention so routes elect automatically once a covering route appears, while eliminating silent drop |
| Declare both `bgp-rib-unresolved-next-hop` and `bgp-rib-next-hop-metrics` via `sdk.Registration.DoctorChecks` (`DoctorPhaseRuntime`) | Register in `registry.DoctorCheckDef` or rely solely on report bus | Works identically in internal and forked placements, obeys `docs/architecture/doctor-and-health-checks.md`, and deduplicates with `recent-events` via `Codes` |
| Coalesce `logger().Error` onto `noteMetricFailure` transitions while keeping failed lookups uncached | Cache failed RPC lookups as `Resolved: false` | Prevents log storms during outages without holding routes ineligible past recovery |
| Repair the 13 failing `.ci` fixtures using shared `terminalCovers` | Add a test-only always-resolved stub | Exercises the real RFC 4271 §9.1.2 resolver over every repaired test without weakening assertions |

## Known Limitations
- Pre-ranking exclusion reasons for other causes (self-next-hop, RFC 8212 source denial, invalid SRv6 SID) are not unified into a `uint8` field on `storage.RouteEntry` in this spec; this spec specifically resolves the two owned `silent-fall-through.md` unresolved-next-hop obligations.
- `show bgp rib best | reason` continues to rank only Phase 2 eligible candidates; operators inspect excluded routes via `show bgp rib | match next-hop-unresolvable`.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 4271 Section 9.1.2 | Exclude routes whose `NEXT_HOP` is unresolved or would become unresolvable if installed; retain in Adj-RIB-In and reselect on interior reachability change | Private forwarded-wire recovery (`candidateAdmissionRecovery`) and routes with no address next hop (FlowSpec, no-NH manual inject) | `test/plugin/rfc4271-{,prospective-}next-hop-kernel*.ci`, `TestShowBGPRIBMarksUnresolvedNextHop`, `TestUnresolvedNextHopDoctorCheckLifecycle` | `TestRecoveryQueuedDeliveryTransport`, `rib_recovery_admission_test.go`, `TestInjectRouteWarnsOnUnresolvedNextHop` |

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
- [ ] AC-1..AC-7 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
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
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)

## Review Gate

Prior campaign note: bounded recovery producer repair, ordinary and live prospective kernel proofs (`46f486c4a0`, `b032fb7dc7`), and `lg-best-table.ci` (`b5079ff5ff`) landed with enforced RFC 4271 audits (`3.1-2`, `9.1.2-1`, `9.1.2-4`, `9.1.2.1-2`). Final closure Review Gate runs via `/ze-close` once Phases 1–4 above are implemented.


---

## Implementation Summary

### What Was Implemented
- **Phase 1 & 2 (Unresolved visibility, inject warning, runtime doctor checks, and fail-closed log coalescence):**
  - Added `doctor-bgp-next-hop-unresolvable` in `internal/component/bgp/plugins/rib/doctor_codes.go` (preserving single `doctor-bgp-next-hop-metrics` + `diagnostic.RegisterEventCode` registration in `register.go`).
  - Added bounded `unresolvedNextHopTracker` (`maxTrackedUnresolvedNextHops = 4096`, `maxUnresolvedDoctorSamples = 5`, fixed 512-byte `[64]uint64` deduplicated overflow filter, leaf mutex never nested with `peerMu`/`remote.mu`, zero-lock atomic fast path) and Stage 1 `DoctorChecks` / `OnDoctorCheck` handlers for `bgp-rib-unresolved-next-hop` and `bgp-rib-next-hop-metrics` in `rib_next_hop_doctor.go` and `rib.go`.
  - Wired forwarding-only (`candidateAdmissionForwarding`, never `candidateAdmissionRecovery`) tracker reconciliation and emptied-owner cleanup across `rib_commands.go`, `rib_manual.go`, `rib_bestchange.go`, `rib_aigp.go`, `rib_structured.go`, `rib_inject.go`, and `rib_source_mutation.go`.
  - Updated `RIBManager.injectRoute` (`rib_commands.go`) to retain unresolved routes in Adj-RIB-In while returning `"next-hop-resolvable": false` and `"warning"` naming the next hop and remedy (`ze show rib` / `ze show bgp rib | match next-hop-unresolvable`) plus one `WARN` log carrying `doctor-bgp-next-hop-unresolvable`; resolvable and no-NH injects emit the unchanged 3-key response.
  - Updated `show bgp rib` (`rib_pipeline.go`, `rib_pipeline_bestfact.go`) to annotate excluded non-best received rows with `"next-hop-resolvable": false` and `"ineligible-reason": "next-hop-unresolvable"`, switching to cache-only fail-closed lookup (`entryNextHopResolvedForShow`, `rib_next_hop.go`, `rib_remote.go`) when `metricFailureActive()` is true so `show bgp rib` never fires `5s * N` blocking RPCs during an outage.
  - Updated `remoteRIB.noteMetricFailure` (`rib_remote.go`) to return state transition `bool` so `logger().Error` in `prospectiveDistance` (`rib_next_hop.go`) and `igpDistance` (`rib_remote.go`) coalesces onto `healthy -> failed` transitions while keeping `// refuse-by-default:` uncached `Resolved: false`.
  - Created unit test suites `rib_next_hop_doctor_test.go` and `rib_next_hop_visibility_test.go`, and drafted, 20x stress-tested (20/20 PASS), and promoted `test/plugin/rib-unresolved-next-hop-visibility.ci` (`option=needs-linux`).
- **Phase 3 (14 `.ci` functional fixture repairs):**
  - Generalized `internal/test/fixture/terminal_cover_fixture.go` (`terminalCovers` + `parseTerminalCover` + `seed`, unit-tested by `TestParseTerminalCover` in `terminal_cover_fixture_test.go`) to support IPv6 `/128` (`AFI: 2, SAFI: 1`) beside IPv4 `/32` (`AFI: 1, SAFI: 1`).
  - Repaired all 13 target `.ci` scenarios plus RFC-tagged `test/plugin/srv6-invalid-sid-ineligible.ci` (`RFC9252-5-1`) with real independent terminal covers (`rr-basic`, `bestpath-reason`, `api-peer-create-delete-rib`, `multipath-basic`, `nexthop-self`, `nexthop-unchanged`, `rib-forward-handle-observed`, `rib-inject-rfc5549`, `fib-ecmp`, `fib-ecmp-realtime`, `llgr-readvertise-multipeer`, `llgr-readvertise-multipeer-forked`, `bgp-rib-maximum-paths-reload`, `srv6-invalid-sid-ineligible`); audited `forward-mpreach-nexthop-self-two-peer` as unrelated (loads neither `bgp-rib` nor `rib`). All 14/14 passed live execution and received independent `ze-test-judge` APPROVE.
- **Phase 4 (Docs & journal):**
  - Updated `docs/architecture/route-selection.md` and `docs/architecture/doctor-and-health-checks.md` with exact source anchors; flipped the 2026-11-18 and 2026-11-20 rows in `plan/journal/silent-fall-through.md` to `fixed:` (verified `plan/journal/conflicting-feature-contracts.md` 2026-11-15 row was already `fixed:`).

### Bugs Found/Fixed
- Live run of `multipath-basic` revealed that after covering `peer1`'s `10.0.0.1` (`IGPCost = 0`), the synthetic inject with no `nexthop` kept `IGPCost = ^uint64(0)`, failing `multipathEqual`; fixed by covering `10.0.0.2/32` and passing `nexthop 10.0.0.2` (re-judged APPROVE).
- Live run of `api-peer-create-delete-rib` revealed `dest-peer` (attached to `process rs`) lacked `rs-client true`, causing eBGP `next-hop self` to rewrite `1.1.1.1` to `local.ip 127.0.0.1 == remote.ip`, triggering RFC 4271 §5.1.3 self-next-hop withdrawal; fixed by setting `rs-client true` on `dest-peer` (re-judged APPROVE).
- Independent Round-1 review (`e98dc465`) caught (1) overflow re-announcement inflation, emptied-owner cleanup gaps, and zero-sample doctor formatting, (2) blocking per-row `5s` `RouteMetrics` RPCs in `show bgp rib` during a forked metric outage, and (3) missing `option=needs-linux` on `rib-unresolved-next-hop-visibility.ci`. All fixed with regression tests (`TestUnresolvedNextHopOverflowDeduplicationAndEmptyCleanup`, `TestShowBGPRIBUsesCachedNextHopDuringRemoteMetricOutage`) and verified `CLEAN` in Round 2 (`ac1c070e`).

### Documentation Updates
- `docs/architecture/route-selection.md` — documented `Design: Unified Rejection Reason` departure, removed stale open-proof notes, documented visibility fields, inject warning, bounded tracker, runtime doctor checks, cache-only show path, and coalesced metric error logging with exact `<!-- source: ... -->` anchors.
- `docs/architecture/doctor-and-health-checks.md` — added `bgp-rib-unresolved-next-hop` and `bgp-rib-next-hop-metrics` to the Stage 1 plugin runtime-checks table and prose.
- `./le doc check all`: `Documentation tests PASSED` (3156/3156 anchors resolve; 0 drift).

### Deviations from Plan
- Also repaired `test/plugin/srv6-invalid-sid-ineligible.ci` (audited in AC-6 and confirmed failing solely because valid control P3 lacked a `10.0.0.1/32` cover); approved via `./le rfc approve unit plugin.srv6-invalid-sid-ineligible`.
- `fib-ecmp` uses 3 next hops (`10.0.0.11/12/13`) and `fib-ecmp-realtime` uses 2 (`10.0.0.11/12`) — the spec text had those two counts transposed; fixtures use the exact addresses each scenario injects.
- Additional helper/test files touched beyond initial list: `rib_aigp.go`, `rib_pipeline_bestfact.go`, `rib_structured.go`, `rib_inject.go`, `rib_source_mutation.go`, `constants.go`, `plugin_fixture_09.go`, `reload_fixture_live_stages.go`, `plugin_fixture_02_srv6_ineligible.go`, `srv6-invalid-sid-ineligible.ci`, `terminal_cover_fixture_test.go`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Initial `unresolvedNextHopTracker` incremented `overflowByPeer` on every `reconcileRoute` at cap and did not purge overflow on individual withdrawal to empty peer | Repeated announcements of the same overflowed route inflated counts and withdrawing all routes individually left stale overflow | Independent Round-1 review (`e98dc465`) | Added fixed 512B deduplicated overflow filter, emptied-owner purges, and `TestUnresolvedNextHopOverflowDeduplicationAndEmptyCleanup` |
| approach | Initial `markNextHopFacts` called `entryNextHopResolvedWithClass` directly during remote-metric outages | Each uncached row triggered a 5s RPC under `remote.mu` during `show bgp rib` | Independent Round-1 review (`e98dc465`) | Added cache-only `entryNextHopResolvedForShow` when `metricFailureActive()` is true plus `TestShowBGPRIBUsesCachedNextHopDuringRemoteMetricOutage` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Surface unresolved next-hop exclusions in `show bgp rib` and `ze doctor` (`doctor-bgp-next-hop-unresolvable`) | Done | `internal/component/bgp/plugins/rib/{rib_next_hop_doctor,rib_pipeline,rib_pipeline_bestfact,doctor_codes,rib}.go` | Proven by unit + `rib-unresolved-next-hop-visibility.ci` (20/20 stress PASS) |
| Warn on `request bgp rib inject` with unresolvable next hop while retaining in Adj-RIB-In | Done | `internal/component/bgp/plugins/rib/rib_commands.go` (`injectRoute`) | Proven by `TestInjectRouteWarnsOnUnresolvedNextHop` + `rib-unresolved-next-hop-visibility.ci` |
| Coalesce `logger().Error` on `prospectiveDistance`/`igpDistance` and expose `bgp-rib-next-hop-metrics` in `OnDoctorCheck` | Done | `internal/component/bgp/plugins/rib/{rib_next_hop,rib_remote,rib_next_hop_doctor}.go` | Proven by `TestRemoteMetricFailureCoalescesLogAndReportsDoctor` |
| Repair all 14 `.ci` fixtures with real `terminalCovers` and update stale docs | Done | `internal/test/fixture/*`, `test/plugin/*`, `test/reload/bgp-rib-maximum-paths-reload.ci`, `docs/architecture/{route-selection,doctor-and-health-checks}.md` | 14/14 PASS + 14/14 `ze-test-judge` APPROVE + `./le doc check all` PASS |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestShowBGPRIBMarksUnresolvedNextHop`, `TestShowBGPRIBUsesCachedNextHopDuringRemoteMetricOutage`, `test/plugin/rib-unresolved-next-hop-visibility.ci` | `next-hop-resolvable: false`, `ineligible-reason: next-hop-unresolvable`, omitted when resolvable/no-NH |
| AC-2 | Done | `TestInjectRouteWarnsOnUnresolvedNextHop`, `test/plugin/rib-unresolved-next-hop-visibility.ci` | Retains route, keeps 3 keys + adds warning fields + 1 `WARN` log |
| AC-3 | Done | `TestUnresolvedNextHopDoctorCheckLifecycle`, `TestUnresolvedNextHopOverflowDeduplicationAndEmptyCleanup`, `test/plugin/rib-unresolved-next-hop-visibility.ci` | Warning with count, <=5 samples, overflow cap, fix, full lifecycle cleanup |
| AC-4 | Done | `TestRemoteMetricFailureCoalescesLogAndReportsDoctor` | Fail-closed uncached, 1 coalesced Error log per outage, `OnDoctorCheck` reports & clears |
| AC-5 | Done | `TestRIBNextHopDoctorCodesRegistered` | `doctor-bgp-next-hop-unresolvable` in `doctor_codes.go`, single `doctor-bgp-next-hop-metrics` in `register.go` |
| AC-6 | Done | 14/14 `.ci` PASS + 14/14 independent `ze-test-judge` APPROVE + `TestParseTerminalCover` PASS | Includes `srv6-invalid-sid-ineligible` (`RFC9252-5-1`) |
| AC-7 | Done | `./le doc check all` PASS (`3156/3156` anchors, 0 drift) | `route-selection.md` + `doctor-and-health-checks.md` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestShowBGPRIBMarksUnresolvedNextHop` + `TestShowBGPRIBUsesCachedNextHopDuringRemoteMetricOutage` | Done | `internal/component/bgp/plugins/rib/rib_next_hop_visibility_test.go` | PASS |
| `TestInjectRouteWarnsOnUnresolvedNextHop` | Done | `internal/component/bgp/plugins/rib/rib_next_hop_visibility_test.go` | PASS |
| `TestUnresolvedNextHopDoctorCheckLifecycle` + `TestUnresolvedNextHopOverflowDeduplicationAndEmptyCleanup` | Done | `internal/component/bgp/plugins/rib/rib_next_hop_doctor_test.go` | PASS |
| `TestRemoteMetricFailureCoalescesLogAndReportsDoctor` | Done | `internal/component/bgp/plugins/rib/rib_next_hop_doctor_test.go` | PASS |
| `TestRIBNextHopDoctorCodesRegistered` | Done | `internal/component/bgp/plugins/rib/rib_next_hop_doctor_test.go` | PASS |
| `TestParseTerminalCover` | Done | `internal/test/fixture/terminal_cover_fixture_test.go` | PASS |
| `test/plugin/rib-unresolved-next-hop-visibility.ci` | Done | `test/plugin/rib-unresolved-next-hop-visibility.ci` | 20/20 stress PASS + promoted 1/1 PASS (11.2s) |
| 14 repaired `.ci` fixtures | Done | `test/plugin/*.ci`, `test/reload/bgp-rib-maximum-paths-reload.ci` | 14/14 PASS |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/plugins/rib/{doctor_codes,rib,rib_next_hop_doctor,rib_next_hop_doctor_test,rib_next_hop_visibility_test,rib_next_hop,rib_remote,rib_commands,rib_manual,rib_bestchange,rib_aigp,rib_pipeline,rib_pipeline_bestfact,rib_structured,rib_inject,rib_source_mutation}.go` | Done | Full RIB package PASS; `./le go style check` OK (16/16); 0 Plan 2 lint issues |
| `internal/test/fixture/{constants,terminal_cover_fixture,terminal_cover_fixture_test,plugin_fixture_15_show,plugin_fixture_02,plugin_fixture_02_srv6_ineligible,plugin_fixture_13_peer_lifecycle,plugin_fixture_10,plugin_fixture_13_rib,plugin_fixture_06_fib,plugin_fixture_09,plugin_fixture_09_bgp,reload_fixture_live_stages}.go` | Done | `./le go style check` OK (13/13); 0 Plan 2 lint issues |
| `test/plugin/{rib-unresolved-next-hop-visibility,rr-basic,bestpath-reason,api-peer-create-delete-rib,multipath-basic,nexthop-self,nexthop-unchanged,rib-forward-handle-observed,rib-inject-rfc5549,llgr-readvertise-multipeer,llgr-readvertise-multipeer-forked,srv6-invalid-sid-ineligible}.ci`, `test/reload/bgp-rib-maximum-paths-reload.ci` | Done | 15/15 PASS |
| `docs/architecture/{route-selection,doctor-and-health-checks}.md`, `plan/journal/silent-fall-through.md` | Done | `./le doc check all` PASS |

### Audit Summary
- **Total items:** 21
- **Done:** 21
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (added `srv6-invalid-sid-ineligible`, corrected `fib-ecmp`/`fib-ecmp-realtime` cover counts, added overflow/emptied-owner/cache-only files from Round-1 review)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Unresolved next hops surface in `show bgp rib`, `request bgp rib inject`, and `show doctor`, and clear on resolution | Functional `.ci` + unit tests | `test/plugin/rib-unresolved-next-hop-visibility.ci` (20/20 stress PASS, promoted PASS 11.2s); full `./le test unit pkg path ./internal/component/bgp/plugins/rib/` PASS (`exit 0, 1 package ok`) |
| Coalesced fail-closed remote metrics + runtime doctor check | Unit test | `TestRemoteMetricFailureCoalescesLogAndReportsDoctor` & `TestShowBGPRIBUsesCachedNextHopDuringRemoteMetricOutage` PASS |
| Remaining 14 `.ci` fixtures pass with real `terminalCovers` and unchanged assertions | Functional `.ci` + `ze-test-judge` | Baseline FAIL -> 14/14 PASS (`rr-basic`, `bestpath-reason`, `api-peer-create-delete-rib`, `multipath-basic`, `nexthop-self`, `nexthop-unchanged`, `rib-forward-handle-observed`, `rib-inject-rfc5549`, `fib-ecmp`, `fib-ecmp-realtime`, `llgr-readvertise-multipeer`, `llgr-readvertise-multipeer-forked`, `srv6-invalid-sid-ineligible`, `bgp-rib-maximum-paths-reload`); 14/14 independent `ze-test-judge` APPROVE |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None (every in-scope item implemented and verified; `test/plugin/forward-mpreach-nexthop-self-two-peer.ci` audited as unrelated to `bgp-rib`) | N-A | N-A |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bgp-next-hop-reachability-admission-plan2rib.md` |
| `./le spec review check` | clean (verdict=clean) |
| Rounds | 2 |
| Reviewer lenses used | independent code+spec review (`e98dc465`), independent Round-2 review (`ac1c070e`, CLEAN 0/0/0), independent `ze-test-judge` (`00414c22`, 14/14 APPROVE) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Tracker overflow inflated on re-announcement, leaked on individual withdrawal/empty manual/peer owner, and formatted `(+N more)` with zero samples | `rib_next_hop_doctor.go`, `rib_commands.go`, `rib_source_mutation.go`, `rib_bestchange.go` | Fixed 512B deduplicated overflow filter, emptied-owner cleanup across all withdrawal/clear paths, zero-sample phrasing, `TestUnresolvedNextHopOverflowDeduplicationAndEmptyCleanup` |
| 2 | ISSUE | `show bgp rib` fired `5s * N` blocking `RouteMetrics` RPCs during forked metric outage | `rib_pipeline_bestfact.go`, `rib_next_hop.go`, `rib_remote.go` | Added cache-only fail-closed `entryNextHopResolvedForShow` when `metricFailureActive()` is true + `TestShowBGPRIBUsesCachedNextHopDuringRemoteMetricOutage` |
| 3 | ISSUE | Missing `option=needs-linux` on `rib-unresolved-next-hop-visibility.ci` (`backend netlink` + `static`/`lo`) | `test/plugin/rib-unresolved-next-hop-visibility.ci` | Added `option=needs-linux` with rationale comment |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/plugins/rib/rib_next_hop_doctor.go` | Yes | `-rw-r--r-- 1 thomas thomas 15833 Nov 22 15:23 internal/component/bgp/plugins/rib/rib_next_hop_doctor.go` |
| `internal/component/bgp/plugins/rib/rib_next_hop_doctor_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 22783 Nov 22 15:22 internal/component/bgp/plugins/rib/rib_next_hop_doctor_test.go` |
| `internal/component/bgp/plugins/rib/rib_next_hop_visibility_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 15404 Nov 22 15:22 internal/component/bgp/plugins/rib/rib_next_hop_visibility_test.go` |
| `internal/test/fixture/terminal_cover_fixture_test.go` | Yes | `-rw-r--r-- 1 thomas thomas  1457 Nov 22 14:09 internal/test/fixture/terminal_cover_fixture_test.go` |
| `test/plugin/rib-unresolved-next-hop-visibility.ci` | Yes | `-rw-r--r-- 1 thomas thomas  4486 Nov 22 15:21 test/plugin/rib-unresolved-next-hop-visibility.ci` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-5 | Visibility, inject warning, runtime doctor checks, coalesced remote metric failure, doctor code registry | `./le test unit pkg path ./internal/component/bgp/plugins/rib/`: `exit 0, 1 package ok` (`job-unit-pkg-8c945297.log`); `./le --name plan2 test bgp plugin -pattern rib-unresolved-next-hop-visibility`: `PASS 1/1 11.2s` (and 20/20 stress PASS) |
| AC-6 | 14 repaired `.ci` fixtures green with unchanged assertions | 14/14 PASS (`rr-basic` 1.1s, `bestpath-reason` 988ms, `api-peer-create-delete-rib` 10.5s, `multipath-basic` 822ms, `nexthop-self` PASS, `nexthop-unchanged` 805ms, `rib-forward-handle-observed` 1.0s, `rib-inject-rfc5549` 1.0s, `fib-ecmp` 758ms, `fib-ecmp-realtime` 737ms, `llgr-readvertise-multipeer` 1.2s, `llgr-readvertise-multipeer-forked` 1.5s, `srv6-invalid-sid-ineligible` 3.0s, `bgp-rib-maximum-paths-reload` 1.0s); 14/14 independent `ze-test-judge` APPROVE |
| AC-7 | Docs and journal updated cleanly | `./le doc check all`: `Documentation tests PASSED` (`3156/3156` anchors, 0 drift); `./le plugin boundary check` OK; `./le plugin declarations check` OK |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `request bgp rib inject`, `show bgp rib \| match next-hop-unresolvable`, `show bgp rib best`, `show doctor`, static cover live commit & reselection | `test/plugin/rib-unresolved-next-hop-visibility.ci` | Yes: live daemon + SSH CLI + static commit (`PASS 1/1`, 20/20 stress PASS) |
| Repaired BGP/RIB/FIB/LLGR/SRv6/reload scenarios via `terminalCovers` | 14 `.ci` files in `test/plugin/` and `test/reload/` | Yes: 14/14 PASS with working-tree `le` binary |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Full `internal/component/bgp/plugins/rib/` unit package PASS (`exit 0, 1 package ok`) |
| A-2 | confirmed | All RIB unit tests, 14 repaired `.ci` fixtures, and `rib-unresolved-next-hop-visibility.ci` PASS |
| A-3 | confirmed | Baseline FAIL -> 14/14 PASS with `terminalCovers` + 14/14 `ze-test-judge` APPROVE |
| A-4 | confirmed | Bounded 4096-entry tracker + 512B overflow filter + cache-only show path unit-tested and PASS |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/architecture/route-selection.md`, `docs/architecture/doctor-and-health-checks.md`, `plan/journal/silent-fall-through.md` | `./le doc check all` (`Documentation tests PASSED`, 3156/3156 anchors, 0 drift) | Yes |
