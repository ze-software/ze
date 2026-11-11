# Spec: doctor-coverage-7-bgp-plugins

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Priority | P2 |
| Release reason | Thomas's rule that `ze doctor` reports every problem an operator can hit, each with its cause and the command that fixes it (`ai/META-DECISIONS.md`, "Goal and scope"), and owner decision O-3 of the parent: every area child is release work, P2 in `plan/release/validation/`, whatever release profile its feature sits in |
| RFC gaps | - |
| Depends | `spec-doctor-coverage-1-audit-and-gate`: the `runtime` doctor phase, `Diagnostic.Fix` and the doctor-coverage gate (landed in commits ce58dddf53 and fc96556110), and its `doctor-recent-events` check (`internal/component/doctor/checks_recent_events.go`). `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` phase P0a (the cross-cutting codes, the transient-code registration that `doctor-recent-events` reads, a raise carrying its fix) and phase P0b (the plugin-protocol report-bus raise route a forked plugin uses; it replaces the P0 raise route this spec once planned for itself). From child 12 also the codes it owns (P0a: `doctor-internal-defect`, `doctor-plugin-setup-failed`, `doctor-plugin-stopped`, `doctor-plugin-external-unsupported`) and the "runs in-process only" registration field with its check (child 12 phase P4, its AC-24; inventory, "Shared mechanisms", item 6). `plan/release/validation/spec-doctor-coverage-11-hub-engine.md`: the `config-plugin-verify` declaration of its `hub-runtime` check (its AC-34, for AC-76 here) and the route field on `DoctorCheckContext` (its AC-1 and A-2, for the local halves of AC-4 and AC-25). `plan/release/validation/spec-doctor-coverage-15-system.md` AC-19 adds `doctor-bgp-irr-stale-data` to the `filter_irr` runtime check of phase P6; whichever spec lands first creates that check. `plan/release/fixes/spec-bmp-route-action-redistribute-acts-as-monitor.md` owns the BMP `route-action redistribute` row; its design deletes the warning line, so AC-85 reads that line gone only after that spec lands (a closure-only dependency) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent: `spec-doctor-coverage-1-audit-and-gate` (AC-6, AC-9). Umbrella: `plan/release/validation/spec-doctor-coverage-0-umbrella.md`.

## Task

`ze doctor` reports every problem class of the BGP plugins (`internal/component/bgp/plugins/*`, door `ai/doors/bgp-plugins.md`) with its cause in plain words and the command that fixes it, on every surface through the daemon. Every Warn and Error line in these packages names its doctor code (O-2 of the parent: no exempt marker), so `./le arch doctor-coverage` counts it covered.

The rows come from `plan/audits/doctor-problem-inventory.md`, section "bgp-plugins" (slices A6 and A7): 66 class rows, 65 gap and 1 covered. This spec takes them as follows:

| Rows | Count | Disposition |
|------|------:|-------------|
| A6 classes for `rib`, `adj_rib_in`, `rs`, `rr`, `bmp` | 32 | 30 get an AC; the `statistics-timeout` row is false (Current Behavior) and becomes the defect AC-69; `route-action redistribute` is owned by `spec-bmp-route-action-redistribute-acts-as-monitor` |
| A7 classes B2 to B35 | 34 | 33 get an AC; B25 is covered by `bgp-peer-no-role` and gets no AC (its log line still carries the code, AC-72) |
| Plugin stopped (A6 first row, A7 B1) | merged | plugin-infra child 12, "A configured plugin is not running" (inventory, "Merged rows"); the lines carry its code (AC-79) |
| OPEN unreadable (A6) | merged | bgp-engine area child, A5 C5 (inventory, "Merged rows") |
| "Not problem classes" groups | 12 | one AC per group (AC-73 to AC-84): each line carries the named code |
| AC-9 rows | 0 | none: `bgp-bmp-collectors`, `bgp-peer-no-role` and `bgp-rpki-cache-servers` read only the config tree and a host probe, so `post-config` is right for all three (both slices, "AC-9") |

Every runtime check here reads the plugin's own state inside the plugin's own process, reached through the plugin protocol (`ze-plugin-callback:doctor-check`), never through a Go call from the daemon: each of these plugins can run as a forked process (`ai/doors/system.md`, "A plugin can run in another process").

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the process boundary, registration, user-facing errors
  → Constraint: a plain Go call into a plugin's package works internal and silently reads an empty copy when the plugin runs forked; a check reaches plugin state through the protocol
  → Constraint: "A backend that cannot apply a config exactly refuses it at verify. It never approximates" (AC-34, AC-57, AC-66 refuse at verify as well as report)
- [ ] `ai/doors/bgp-plugins.md` - the BGP plugins
  → Constraint: "Every one can run in another process", so a runtime doctor check is declared in the `sdk.Registration` the running plugin sends at Stage 1 and answered by `OnDoctorCheck`
  → Constraint: the door's "A plugin declares twice, and both must agree" holds for commands and pipe aliases (`./le plugin declarations check` compares only those), and NOT for doctor checks: each doctor check has exactly one declaration route since commit 149497ce12. A config-phase check is a `registry.DoctorCheckDef` (or a `diagnostic.RegisterDoctorCheck`, as `bgp-bmp-collectors` and `bgp-rpki-cache-servers` are); a live-state check is a Stage 1 `rpc.DoctorCheckDecl`. `validateDoctorCheckDef` refuses a `registry.DoctorCheckDef` in the `runtime` phase, and `refuseDoctorCheckDeclaredTwice` refuses a Stage 1 check whose name the plugin's registry row also holds
  → Constraint: "A barrier or a report is about timing, never about permission": the session-ready rows report a fence held, never a refusal
- [ ] `ai/doors/plugin-infra.md` - SDK and process protocol
  → Constraint: "One engine-op entry per RPC" (`plugin/server/dispatch_registry.go` `engineOps`); the event-raise RPC is one entry there, built by child 12 phase P0b, and this spec adds none, with its YANG in `internal/core/ipc/yang/ze-plugin-engine.yang`
  → Constraint: "A field or method name in `rpc` is wire"; the plugin protocol owes no backward compatibility before the first release (`ai/META-DECISIONS.md`), so every in-tree plugin is rebuilt with it
  → Constraint: "The SDK stays generic": the raise is one SDK method, carrying no BGP knowledge
- [ ] `ai/doors/core.md` - core registries
  → Constraint: a registry is the only list; each new code is declared once in `internal/core/diagnostic/codes.go` and named by the declaring check's `Codes`
- [ ] `ai/doors/bgp-engine.md` - the engine the plugins extend
  → Constraint: "Never name a plugin package from the reactor": the route-server grant check is child 6's (its AC-25, in `bgp/config`, which already imports `reactor`), so no plugin package reads the reactor's grant predicate and none moves (AC-16)
- [ ] `ai/doors/system-plugins.md` - the model plugin `ddos-detect`
  → Constraint: "A plugin owns its whole surface": its doctor checks live in its folder
- [ ] `ai/doors/le-tooling.md` - the doctor-coverage gate and the test runner
  → Constraint: a package test runs through `./le job run` with its feature tags (`ze_bgp`, plus `ze_bmp` for `bmp`)
- [ ] `ai/doors/forwarding.md`, "The report bus is process-local too"
  → Constraint: "a warning a forked plugin raises never reaches `show warnings`"; a transient class raised in a forked plugin needs child 12 phase P0b's raise route to reach the daemon's bus
- [ ] `docs/architecture/doctor-and-health-checks.md` - phases, owners, the two tests per check
  → Constraint: a unit test in the owning package proves the check fires only on its condition and emits its registered code; a functional test exposes it through `show doctor` or `ze doctor --json`
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: problem, why, then the fixing command on its own line (`Diagnostic.Fix`, the command alone)
- [ ] `spec-doctor-coverage-1-audit-and-gate`, Data Flow amendments
  → Decision: 2a (amended): `doctor-recent-events` renders a report-bus event under its own code only when that code is registered in `internal/core/diagnostic/codes.go`; a bus code counts as covered for the gate only when its owner's check declares it
  → Decision: 1b: a `runtime` registry check gets only `Platform`; a runtime check reads its owner's live state, never a config tree

**Key insights:**
- `collectDoctorChecksFrom` (`internal/component/doctor/cmd/show.go`, called by `answerDaemonDoctor` under `HandleDoctor`) "calls every check a running plugin declared, whatever its phase", through `CallDoctorCheck`, internal or forked, and runs on the daemon route only; that is the route every runtime check here takes. `runPluginRegistryChecks` runs the `registry.DoctorCheckDef` checks of the config phases (a runtime one is refused at registration) in whichever process answers: the operator's own on the local route, and the daemon on the daemon route, where `doctor.Answer` runs `runChecks` too. So the config checks here register there, and each of them also runs inside the daemon on `show doctor`
- `internal/plugins/ddos/detect/register.go` is the in-tree model of a plugin carrying both kinds, under two different check names: a `registry.DoctorCheckDef` (`ddos-detect-flow-source`, post-config, config tree) and an `sdk.Registration` `DoctorChecks` entry (`rateFeedCheckDecl`) answered by `p.OnDoctorCheck` (runtime state). One name declared on both routes is refused at Stage 1 (`refuseDoctorCheckDeclaredTwice`, commit 149497ce12), so a plugin here that has a local and a runtime half of one class carries two checks with two names
- Stage 1 bounds a plugin's declared checks (`validateDoctorCheckDecls`, `internal/component/plugin/server/startup.go`): at most 16 checks, each with 1 to 16 codes, every code `doctor-` prefixed, the name kebab-case, the order 0 to 9999
- A config check and a runtime check that report one fact under one code both run on `show doctor` (the config phase runs in the daemon too), so such a pair would draw two rows there; the config half answers on route `local` only, through the route field on `DoctorCheckContext` that `spec-doctor-coverage-11-hub-engine` adds (its AC-1, A-2). A bind probe run inside the daemon fails on every port the daemon itself serves (`docs/architecture/doctor-and-health-checks.md`, "The phase decides what a check may read"), which makes this mandatory for AC-25
- `report.RaiseError` and `report.RaiseWarning` (`internal/core/report/report.go`) write the bus of the calling process. No RPC in `pkg/plugin/rpc` carries a raise (grep of `pkg/plugin/sdk` for a raise or report method finds none), so every transient class of a forked plugin is invisible to `doctor-recent-events` today. `metrics-publish` (`engineOps`, `ze-plugin-engine.yang`) is the precedent for a forked plugin publishing state the daemon keeps
- `doctor-recent-events` draws every row as a warning whatever the event's severity ("the event already happened and its owner may have recovered"), so a transient AC below never makes the box read not ready

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/registry/doctor.go` - `DoctorCheckDef{Name, Phase, Order, Dependencies, Platforms, Codes, Check}`, `PluginDoctorChecks`; `DoctorCheckContext{Tree, ConfigDir, Platform}` (no route); `validateDoctorCheckDef` requires the `doctor-` prefix on every code and refuses phase `runtime` ("declare it in the sdk.Registration DoctorChecks and answer it with OnDoctorCheck", commit 149497ce12)
- [ ] `internal/component/doctor/cmd/show.go` - `HandleDoctor` calls `answerDaemonDoctor`, which runs `doctor.Answer` (the config phases, compiled-in plugin `registry.DoctorCheckDef` checks included) with a runtime step of `doctor.RunRuntimeChecks` then `collectDoctorChecksFrom`; `collectDoctorChecksFrom` calls each Stage 1 declared check through `doctorCheckSource.CallDoctorCheck`, skipping the calling plugin, under `perCheckTimeout` (5 s) and `maxDiagnosticsPerCheck` (64)
- [ ] `internal/component/plugin/server/startup.go` - Stage 1: `validateDoctorCheckDecls` (bounds above) and `refuseDoctorCheckDeclaredTwice` (a Stage 1 check whose name the plugin's registry row holds is refused)
- [ ] `internal/component/bgp/plugins/bmp/bmp_locrib.go` - a Loc-RIB Route Monitoring write that fails toward a collector logs Warn and raises an error event on the bus, both under the generic `doctor-recent-events` code (commit eb3efdecee)
- [ ] `internal/component/bgp/plugins/rs/worker.go` - `workerPool.backpressure` is clear-on-read (`BackpressureDetected`, dispatch polling); `workerPool.inBackpressure` holds the keys in backpressure now, cleared at the 10% low-water mark
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` - `OnDoctorCheck(DoctorCheckHandler)` answers `ze-plugin-callback:doctor-check`
- [ ] `pkg/plugin/rpc/types.go` - `DoctorCheckDecl{Name, Phase, Order, Dependencies, Platforms, Codes}`; `DoctorPhaseRuntime` "run only on the daemon route"
- [ ] `internal/plugins/ddos/detect/register.go` - the model above (`OnDoctorCheck`, `sdk.Registration.DoctorChecks`)
- [ ] `internal/core/report/report.go` - `RaiseWarning`, `RaiseError`, `Warnings`: a package store of the calling process
- [ ] `internal/component/doctor/checks_recent_events.go` - `recentEventsCheck` (runtime, `doctor-recent-events`), `checkRecentEvents`, `groupRecentEvents`, `recentEventsDiagnostic`: one warning row per (severity, source, code, subject) with count and last time; the row keeps the bus code only when `diagnostic.Lookup` knows it
- [ ] `internal/component/plugin/server/dispatch_registry.go` - `engineOps`, "the single source of truth for plugin->engine runtime RPCs", holding `metrics-publish`, `update-route`, `dispatch-command`
- [ ] `internal/core/ipc/yang/ze-plugin-engine.yang` - `rpc metrics-publish` and the other plugin-to-engine RPCs
- [ ] `internal/core/diagnostic/codes.go` - code metadata entries (`doctor-rpki-unreachable`, `doctor-bmp-unreachable`, `CodeDoctorRecentEvents`)
- [ ] `internal/component/bgp/plugins/bmp/doctor.go` - `bmpDoctorCheck` (`bgp-bmp-collectors`, post-config, `doctor-bmp-unreachable`), registered with `diagnostic.RegisterDoctorCheck`: warns only when no configured collector accepts a TCP connection from the host
- [ ] `internal/component/bgp/plugins/bmp/receiver.go` - `applyReceiverConfig` logs Warn for `route-action redistribute` and acts as `monitor`; `startListenerLocked` logs Error on a failed bind and keeps no record; `acceptLoop` logs Warn at `max-sessions` and on an accept failure, and keeps the active count in a local variable
- [ ] `internal/component/bgp/plugins/bmp/statistics.go` - `setStatisticsTimeout` starts one ticker per interval; `sendStatisticsReports` writes one Statistics Report per established peer to every collector (commit 10d66f9dbd)
- [ ] `internal/component/bgp/plugins/bmp/sender_config.go` - the sender apply calls `setStatisticsTimeout` with the `statistics-timeout` leaf
- [ ] `internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang` - the `statistics-timeout` description says "Ze sends no Statistics Report (RFC 7854 Section 4.8) at any value. The periodic timer this leaf configures is not implemented": false since 10d66f9dbd (journal row below)
- [ ] `internal/component/bgp/plugins/rs/server.go` - `RunRouteServer` logs `rr plugin failed` when its SDK run fails (journal row below)
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - `rsRelaySendGranted`, `rsRelayReceiveGranted`, `bindingReceivesPeerUpdates`, `reportRSRelayGrants`: the reactor warns once per snapshot when a fast-path peer lacks `receive [ update-received ]` or `send [ update ]` toward a `registry.RelaysOnRSFastPath` process; no commit refuses it, so the A6 grants row stays (the slice's open question, answered)
- [ ] `internal/component/plugin/types_bgp.go` - `PeerProcessBinding{ReceiveAll, Receive, Implicit, SendAll, Send}`: implicit grants exist, so a grant check must apply the same resolution the reactor does
- [ ] `internal/component/bgp/plugins/role/register.go` - `bgp-peer-no-role` as a `registry.DoctorCheckDef` (post-config, Order 2130); `InProcessConfigVerifier: verifyRoleConfig`
- [ ] `internal/component/bgp/plugins/filter_irr/config.go` - `peersMissingRouteRefresh`, `peersMissingRefreshGrant` already compute B9's answer from the parsed config
- [ ] `internal/component/bgp/plugins/rpki/rtr_session.go` - the non-fatal RTR Error Report line carries the attribute `code` holding the RTR error code, which the gate reads as a doctor code (inventory, "Shared mechanisms", item 4)

The A6 and A7 slice rows (`tmp/session/2026-11-09-223019/scratch/audit/A6-bgp-plugins-rib.md`, `A7-bgp-plugins-rest.md`) and the inventory name each class's source sites and the state it reads; this spec cites their files and symbols, and the implementer re-reads each producer before its phase.

**Behavior to preserve:**
- the three existing checks, their names, codes, phases and orders: `bgp-bmp-collectors`, `bgp-peer-no-role`, `bgp-rpki-cache-servers`
- the `doctor-check` callback output shape and every `show doctor` JSON key
- route handling: no check changes what a plugin stores, selects, forwards or sends

**Behavior to change:**
- one finding per class row, under the code each AC names, from the plugin that owns the state
- every Warn and Error line in `internal/component/bgp/plugins/` carries a doctor code
- a forked plugin's coded event reaches the daemon's report bus (through child 12 phase P0b)
- the two defects in "Owned journal rows" fixed

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Existing BGP plugin checks | `bmp/doctor.go` `bmpDoctorCheck`; `role/register.go` `bgp-peer-no-role`; `rpki/doctor.go` `bgp-rpki-cache-servers` | Config-tree checks with a host probe | extends: kept as they are; AC-28 adds the per-collector runtime half beside `doctor-bmp-unreachable`, AC-64 the RTR session half beside `doctor-rpki-unreachable` |
| Plugin doctor declaration, local | `internal/component/plugin/registry/doctor.go` `DoctorCheckDef`; bridged by `internal/component/doctor/checks_plugin_registry.go` `runPluginRegistryChecks`; or `diagnostic.RegisterDoctorCheck` from the plugin's `register.go`, as `bgp-bmp-collectors` and `bgp-rpki-cache-servers` are | Config-phase checks a compiled-in plugin declares, run in whichever process answers (local route, and the daemon route through `doctor.Answer`); a `runtime` `DoctorCheckDef` is refused | extends: every local AC registers here in its plugin's `register.go`; a local half that must not answer on the daemon route registers through `diagnostic.RegisterDoctorCheck`, whose context carries child 11's route field, because `registry.DoctorCheckContext` carries none |
| Plugin doctor declaration, runtime | `pkg/plugin/rpc/types.go` `DoctorCheckDecl` with `DoctorPhaseRuntime`; `pkg/plugin/sdk/sdk_callbacks.go` `OnDoctorCheck`; called by `collectDoctorChecksFrom` | A running plugin answers its own checks over the protocol, internal or forked; used by `ddos-detect`; the only route a plugin runtime check may take (149497ce12) | extends: every runtime AC is a check of this kind in its owning plugin, under a name its registry row does not hold. This is the "command" route the brief requires, so no daemon-side Go call reads plugin state |
| Doctor runtime phase and fix field | parent spec P1a; `Diagnostic.Fix`, `DoctorCheckDiagnostic` `fix` | Daemon-only phase; fix command on its own line | extends: every finding sets `fix` |
| Report bus and recent events | `internal/core/report/report.go` `RaiseError`, `RaiseWarning`; `internal/component/doctor/checks_recent_events.go` `checkRecentEvents` | Coded events of the calling process; one `runtime` row per recent event | extends: every transient AC raises its event under its code; child 12 phase P0b carries a forked plugin's raise to the daemon's bus, and each transient code is registered through child 12 phase P0a's transient-code registration, which `doctor-recent-events` reads (parent amendment 2a) |
| Plugin-to-engine RPC table | `internal/component/plugin/server/dispatch_registry.go` `engineOps`; `internal/core/ipc/yang/ze-plugin-engine.yang`; precedent `metrics-publish` | One entry per plugin-to-engine RPC; JSON, direct and typed paths derived from it | uses: child 12 phase P0b adds the one coded-event raise entry; this spec adds none and calls it |
| Diagnostic code registry | `internal/core/diagnostic/codes.go`; `ze show diagnostic <code>` | Explains each code | extends: each new code below is registered once there, so `ze show diagnostic` explains it and `doctor-recent-events` keeps it as the row code |
| Health registry | `internal/core/health` | Degraded rows | does not apply: no package in this area registers a health row (both slices) |
| Reactor relay grant report | `internal/component/bgp/reactor/peer_forward_facts.go` `reportRSRelayGrants`, `rsRelaySendGranted`, `rsRelayReceiveGranted` | Warns once per snapshot when the fast-path grants are missing | does not apply here: the route-server half of AC-16 is child 6's AC-25 check over these predicates, and the reactor's Warn line is child 6's site |
| IRR refresh facts | `filter_irr/config.go` `peersMissingRouteRefresh`, `peersMissingRefreshGrant` | Computed at config time, logged | extends: AC-36's check calls them; no second walk |
| Role config verify | `role/register.go` `InProcessConfigVerifier: verifyRoleConfig` | Parses role config at commit | extends: AC-66 makes it refuse the invalid forms it now logs |
| Plugin live state | `role/metrics.go` `metricRouteRejects`, `metricRouteSuppressions`; `bmp` `BMPPlugin` listeners, sessions and `peerUps`; `rpki` `ROACache`, `aSPACache`; `filter_irr` per-ASN state | Metrics and live state inside each plugin | extends: the runtime checks read them in the plugin's process |
| Plugin crash and stop codes | `internal/component/plugin/process/manager.go` `ReportCrash` (`plugin-crash`, `plugin-down`) | Report-bus raise on a plugin stop | does not apply: owned by plugin-infra child 12 (inventory "Merged rows"); the bgp-plugins run-failure lines carry its `doctor-plugin-stopped` code (AC-79) |
| Doctor-coverage gate | `internal/le/arch/doctorcoverage`, `./le arch doctor-coverage report` and `check` | Lists uncovered sites | extends: the closing evidence is its report over `internal/component/bgp/plugins/` (AC-85) |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `show doctor` on SSH, MCP, REST, gRPC and web; `ze doctor` locally (asks the daemon, parent O-1); `ze doctor config <file>` for the local checks

### Transformation Path
1. Local checks (config tree): `runPluginRegistryChecks` runs each plugin's `registry.DoctorCheckDef` at `post-config`, in the operator's process on the local route and in the daemon on the daemon route (`doctor.Answer`); the check walks the tree and returns findings with code, severity, message and fix
2. Runtime checks (plugin state): `HandleDoctor` runs `collectDoctorChecksFrom` (through `answerDaemonDoctor`); the plugin server sends `ze-plugin-callback:doctor-check` to the running plugin; the plugin's `OnDoctorCheck` handler reads its own state in its own process and answers `DoctorCheckDiagnostic` rows
3. Transient classes: at the failure site the plugin logs the line with its `code` attribute and raises the event through the SDK method of child 12 phase P0b; internal, the raise reaches the daemon's bus through the direct path of the engine op; forked, it is one `ze-plugin-engine:` RPC answered by the same engine op, which raises on the daemon's bus with the plugin's name as source; `doctor-recent-events` renders it under its code
4. `doctor.Answer` merges and draws every row with its fix line

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Daemon to plugin (runtime checks) | `ze-plugin-callback:doctor-check`, existing | Yes: `collectDoctorChecksFrom` read; used by `ddos-detect` |
| Plugin to daemon (event raise) | new `ze-plugin-engine:` RPC, one `engineOps` entry | No: added by child 12 phase P0b |

### Changed-Wire Census

**Search evidence:** scope `pkg/plugin/`, `internal/component/plugin/server/`, `internal/component/doctor/`, `internal/core/ipc/yang/`, `test/`; terms `ze-plugin-engine:`, `engineOps`, `RaiseError`, `doctor-check`, `non-fatal error from cache`, `rr plugin failed`. Positive control: `engineOps` in `dispatch_registry.go` holds the existing plugin-to-engine RPCs (`metrics-publish`, `update-route`, `dispatch-command`). No `.ci` file or Go test matches either log line whose text or attribute changes. The event-raise RPC is a new wire owned and censused by `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` phase P0b; this spec only calls its SDK method, so it carries no row for it.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `doctor-check` callback output | writer | each plugin's `OnDoctorCheck` handler | `DoctorCheckOutput{Diagnostics}` | preserve shape |
| `doctor-check` callback output | reader | `internal/component/doctor/cmd/show.go` `collectDoctorChecksFrom` | maps code, severity, message, fix | preserve |
| `show doctor` / `ze doctor --json` | writer | `diagnostic.Diagnostic` through `doctor.Answer` | new rows under new codes, no new key | preserve keys |
| `show doctor` / `ze doctor --json` | reader | `internal/component/doctor/doctor.go` `outputText`, `outputJSON`; API clients outside the repository (MCP, REST, gRPC, SSH); `test/plugin/*doctor*.ci` | rows are a list; no key added | preserve: an existing `.ci` that asserts a whole doctor answer is re-checked in the phase that adds a row its config triggers |
| RTR non-fatal error log line | writer | `rpki/rtr_session.go` | attribute `code` holds the RTR error code | rename the attribute `error-code`; add `code` = `doctor-rpki-cache-error-report` |
| RTR non-fatal error log line | reader | an operator's log tooling outside the repository; the doctor-coverage gate (`internal/le/arch/doctorcoverage`) | no `.ci` or Go test matches the line | the gate reads the new `code`; an outside log reader sees a renamed key, recorded in the commit body |
| Route-server failure log line | writer | `rs/server.go` `RunRouteServer` | message `rr plugin failed` | message names bgp-rs, carries `doctor-plugin-stopped` |
| Route-server failure log line | reader | an operator's log tooling outside the repository; the doctor-coverage gate | no `.ci` or Go test matches the line | preserve the error attribute; the corrected message is the fix |

### Integration Points
- each plugin's `register.go` (`DoctorChecks` for the local checks) and its runner (`sdk.Registration.DoctorChecks` plus `p.OnDoctorCheck` for the runtime checks)
- `internal/core/diagnostic/codes.go` for every new code

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | runtime state is read by the owning plugin over the doctor callback; events reach the bus through one engine op |
| No unintended coupling (components stay isolated) | Yes | no doctor or engine package names a BGP plugin; no plugin reads the reactor's grant predicate |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing Mechanisms table; B9 reuses the IRR functions; AC-16's route-server half is child 6's one check, not a second |
| Zero-copy preserved where applicable (refs, not copies) | Yes | no hot-path change; failure records are written only on the failure branch |
| Registration over hardcoding, outbound | Yes | each check registers from its plugin; each code in the code registry |
| Registration over hardcoding, inbound | Yes | the doctor runner, the gate and `doctor-recent-events` read the registries; searched `internal/component/doctor/` and `internal/le/arch/doctorcoverage/` for plugin or code names of this area: none, and none is added |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A runtime check declared in `sdk.Registration.DoctorChecks` with phase `runtime` and answered by `OnDoctorCheck` runs on `show doctor` for an internal and a forked plugin alike | `collectDoctorChecksFrom` comment: "It calls every check a running plugin declared, whatever its phase"; `ddos-detect` declares one; since 149497ce12 it is the only route for a plugin runtime check | the runtime ACs need another route | `TestBGPPluginRuntimeChecksAnswerBothTransports` and `./le mock plugin-external bgp-rib` in P1 | unvalidated |
| A-2 | One plugin in this area raises on the report bus today: bmp's Loc-RIB collector write failure (`bmp_locrib.go`, commit eb3efdecee), under the generic `doctor-recent-events` code; the A6 slice ("none of these packages raises on the report bus") predates it | grep of `report.Raise` under `internal/component/bgp/plugins/` on 2026-11-11: that one site | another raise exists that P4 must recode | the same grep in P1 | unvalidated |
| A-3 | The verify paths do not already refuse the invalid forms of AC-34 (IRR `as-set`), AC-57 (watchdog), AC-66 (role) | the slices found the forms logged at apply | the refusal half of that AC is already met; its test asserts the existing refusal | a commit test per form in P6, P8, P9 | unvalidated |
| A-4 | Each fix command named in an AC exists in the YANG schema or the command registry with the words used | the slices name config paths; this spec does not invent commands | a fix line names a command that fails | each check's unit test asserts the fix text, and the implementer resolves each path against the YANG before writing it | unvalidated |
| A-5 | The route-action redistribute row is answered by its own fix spec, whose design deletes the Warn line and the `RouteAction` field | `plan/release/fixes/spec-bmp-route-action-redistribute-acts-as-monitor.md` (design, P1; its A-1 waits on the owner's answer to its Q-1) | the Warn line in `applyReceiverConfig` stays, uncovered, and AC-85 cannot read covered | AC-79 lists that line; this spec closes only after that spec lands | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Two raise routes for forked plugins | two `engineOps` entries for one job | resolved: the one route is `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` phase P0b, this spec builds none and its transient ACs wait for P0b (`ai/rules/no-layering.md`) |
| R-2 | A flapping condition fills the report-bus ring and evicts other events | many rows of one code | the ring is bounded (`ze.report.errors.max`) and grouped by (source, code, subject); a transient class raises once per failure, never per route |
| R-3 | A runtime check blocks on a lock the hot path holds | `show doctor` slow under load | each check reads a snapshot or an atomic; the per-check timeout in `show.go` bounds it |
| R-4 | A failure record grows without bound (one entry per peer or prefix) | memory per peer | each record is keyed by peer, collector, probe or cache, holds last failure, count and time, and is removed when its key leaves the config |
| R-5 | Seventy checks slow review | review rounds | phases are cut by plugin, each with its own tests |
| R-6 | AC-16's reflector half has no owning check or code: the route-server half is child 6 AC-25 (2026-11-11 main-session decision, one fault one row), and child 7's `doctor-bgp-relay-grant-missing` is dropped, but `bgp-rr` does not declare `registry.RelaysOnRSFastPath`, so child 6's predicate never judges a reflector attach block | a reflector peer missing `send [ update ]` reported by nothing | open, main-session decision: widen child 6 AC-25's check to every plugin whose forward or replay needs the grants (one check, one code, a registration field rather than a plugin name), or give the reflector fault its own code here; the obligation stays in AC-16 until then |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a doctor row missing or false; a verify refusal on a config that used to be accepted (AC-34, AC-57, AC-66). No route handling changes |
| How is it reverted? | single commit revert per phase |
| Who else touches this path? | `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` (P0a shared codes and transient-code registration, P0b raise route, P4 in-process field), the bgp-engine child `spec-doctor-coverage-6-bgp-engine` (OPEN refusals; its AC-25 owns the one check and code `doctor-bgp-route-server-not-attached` for the route-server fast-path grant fault, inventory A5 C27, which covers the route-server half of the A6 grants row, AC-16 here), `spec-doctor-coverage-11-hub-engine` (route field), `spec-doctor-coverage-15-system` (filter_irr stale row), `spec-bmp-route-action-redistribute-acts-as-monitor` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a forked plugin raising a coded event, then `show doctor` | → | child 12 phase P0b's engine op, then `doctor-recent-events` | child 12 owns the route's wiring test (its AC-4); here each transient AC's own test raises through it from a bgp plugin |
| `show doctor` with bgp-rib internal | → | bgp-rib `OnDoctorCheck` runtime check | `test/plugin/doctor-bgp-rib-runtime.ci` |
| `show doctor` with bgp-rs forked | → | bgp-rs `OnDoctorCheck` over the callback | `test/plugin/doctor-bgp-rs-runtime-external.ci` |
| `show doctor` with a BMP receiver on a busy port | → | bmp runtime check, AC-25 | `test/plugin/doctor-bmp-receiver-not-listening.ci` |
| `ze doctor config <file>` with an invalid IRR `as-set` and an invalid role | → | filter_irr and role local checks | `test/ui/doctor-bgp-plugins-config.ci` |
| `show doctor` with an RPKI cache refusing RTR | → | rpki runtime check, AC-64 | `test/plugin/doctor-rpki-cache-down.ci` |
| `show doctor` with a failing healthcheck probe | → | healthcheck runtime check, AC-51 | `test/plugin/doctor-bgp-healthcheck-down.ci` |
| each plugin's declarations | → | every doctor check declared on one route only: a config check in `registry.Registration.DoctorChecks` (or `diagnostic.RegisterDoctorCheck`), a live-state check in the Stage 1 `sdk.Registration.DoctorChecks`, so Stage 1 (`validateDoctorCheckDecls`, `refuseDoctorCheckDeclaredTwice`) accepts the plugin | `TestBGPPluginRuntimeChecksAnswerBothTransports` (the plugin starts on both transports and answers its checks) |

## Acceptance Criteria

Each row names what doctor reports (finding, severity, code, fix), the state it reads, and the route: `local` is a `registry.DoctorCheckDef` at `post-config` over the config tree (it runs on both routes); `runtime` is a `runtime` check the plugin declares at Stage 1 and answers through `OnDoctorCheck`, under a check name its registry row does not hold; where a row has a local and a runtime half under one code, the local half answers on route `local` only (Key insights); `recent event` is a raise under the code through child 12 phase P0b's route, rendered by `doctor-recent-events` as a warning row, the code registered through child 12 phase P0a's transient-code registration that `doctor-recent-events` reads. Every code is registered in `internal/core/diagnostic/codes.go`. Every fix is the command alone on its own line.

### Shared mechanisms (phase P0)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A bgp plugin of this area, internal and forked, raises a transient code of this spec | it raises through the SDK method of `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` phase P0b (built there, never here); the event is on the daemon's bus with source = the plugin's name, and `show doctor` shows a `doctor-recent-events` row under that code | none |
| AC-2 | Every code this spec names | `ze show diagnostic <code>` explains it; a runtime or local code is declared in `Codes` by its owning plugin's check, and a transient code is registered through child 12 phase P0a's transient-code registration | none |
| AC-3 | The RTR non-fatal Error Report line in `rpki/rtr_session.go` | carries the RTR error code as `error-code` and the doctor code of AC-65 as `code` | none |

### RIB and Adj-RIB-In (`rib`, `adj_rib_in`)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-4 | A peer's configured or negotiated family has no registered NLRI splitter (`nlrisplit.Get`, `Supported`) | `doctor-bgp-family-unstorable`, warning, names peer and family and what is lost (not stored, selected, shown, re-sent); local from the config `family` list (route `local` only), runtime from `RIBManager.unstorableFamilies` (`rib/rib_unstorable.go`); fix names the plugin that provides the family's codec, or removing the family from the peer | a family a running plugin registers at Stage 1 is storable and not reported |
| AC-5 | The store cannot split a received NLRI section (`adj_rib_in/rib_complex.go`, `rib/rib.go`, `rib/rib_validation.go`, `rs/server_inventory.go`) | recent event `doctor-bgp-nlri-unsplittable` naming peer and family; the runtime check answers the per-peer last split failure and count until the peer resets | none |
| AC-6 | `AdjRIBInManager.terminal` set by `failValidationOwner` | `doctor-bgp-adj-rib-in-stopped`, error: the receive store refuses every route until restart; runtime; fix names the daemon restart command | none |
| AC-7 | `sweepExpiredPending` promotes routes to NotValidated after the validation timeout | `doctor-bgp-validation-timeout`, warning, with per-peer promotion count and last time, kept in adj_rib_in; runtime; fix names the RPKI show command and the cache config | a promotion older than the record's window (one hour) is not reported |
| AC-8 | A FlowSpec gate or validation gate not announced, or the gate query not sent (`rib/rib.go`, `rib/rib_validation.go`, `adj_rib_in/rib_gate_events.go`, `rr/rr.go`, `rs/server.go`) | `doctor-bgp-rib-gate-unannounced`, warning, naming the gate and the reader that lacks it; runtime from the `ribevents` gate state each process holds | none |
| AC-9 | `bestPrevInterner` overflow latch set (peers, next hops or metrics; `rib/rib_bestchange.go`) | `doctor-bgp-best-change-untracked`, error, naming which table is full; runtime | none |
| AC-10 | A best-change, FlowSpec, validation or replay-request event not emitted, or a best change without its attributes (`rib/rib_bestchange.go`, `rib/rib_flowspec_validation.go`, `bmp/bmp_locrib.go`) | recent event `doctor-bgp-rib-event-not-emitted` naming the event type and the consumers that diverge | none |
| AC-11 | An Adj-RIB-In candidate whose next hop is this speaker's own address (`rib/rib_commands.go`) | `doctor-bgp-next-hop-self-excluded`, warning, naming peer, prefix count and the next hop; runtime | none: the RFC 4271 Section 6.3 exclusion is correct behavior, and the row says why the route is not used |
| AC-12 | A route injected for a peer with no session or no encoding context (`rib/rib_commands.go`) | recent event `doctor-bgp-inject-no-session` naming peer and that the extended-next-hop check was skipped | none |
| AC-13 | A forked RIB's IGP metric lookup failed or its mandatory feed failed (`rib/rib_remote.go` `runRemoteMetrics`) | `doctor-bgp-igp-metric-unavailable`, error when the feed failed and the RIB closed, warning when a lookup used distance 0; runtime | an internal RIB, which reads the metrics in process, is not reported |

### Route server and route reflector (`rs`, `rr`)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-14 | `request bgp adj-rib-in claim-replay` refused or not answered (`rr/rr.go`, `rs/server_handlers.go`) | `doctor-bgp-replay-unclaimed`, warning: peer-up routes may be announced twice; runtime from the kept claim result | none |
| AC-15 | `bgp-rs` configured and the plugin registry has no `bgp-adj-rib-in` | `doctor-bgp-rs-replay-store-absent`, warning: a new client gets no existing routes, only FlowSpec; local | none |
| AC-16 | A peer attaches the route-server or reflector process without `receive [ update-received ]` or `send [ update ]`, after implicit grants are applied | route server: covered by `plan/release/validation/spec-doctor-coverage-6-bgp-engine.md` AC-25, the one check for this fault (`bgp-route-server-attach`, code `doctor-bgp-route-server-not-attached`, over the reactor's `rsRelaySendGranted`/`rsRelayReceiveGranted`); this spec adds no check and no code for it, and its `rs` grant lines carry `doctor-bgp-route-server-not-attached`. Reflector: the reactor predicates answer only for a plugin declaring `registry.RelaysOnRSFastPath`, which only `bgp-rs` sets (`rs/register.go`), so `bgp-rr`'s own grant need (`rr/rr.go`, the replay owner a destination must grant `send [ update ]`) is not that fault; it stays owed here, open in R-6 | none |
| AC-17 | A peer-up replay failed or ended incomplete (relay error, catch-up gave up at the cut, delta or FlowSpec replay failed; `adj_rib_in/rib.go`, `rr/rr.go`, `rr/validation.go`, `rs/server_handlers.go`, `rs/server_validation.go`) | `doctor-bgp-peer-up-replay-incomplete`, warning, naming peer and reason, kept per peer until that peer resets, and raised as a recent event; fix names the peer reset command | none |
| AC-18 | A session-ready report not delivered, or no peer-up receipt (`rr/rr.go`, `rs/server_handlers.go`, `adj_rib_in/rib.go`, `rib/rib_replay.go`, `watchdog/watchdog.go`) | `doctor-bgp-session-ready-failed` (canonical), warning: the peer's live forwards stay fenced behind the replay; recent event, and the runtime check answers peers whose report is still owed | none: per the door the report is timing only and never holds End-of-RIB, so the row says delayed, not refused |
| AC-19 | The engine refused `update-route` or `dispatch peer-action` (`rr/rr.go`, `rib/rib.go`) | recent event `doctor-bgp-forward-refused` naming destination peer and the engine's reason | none |
| AC-20 | A validation change not relayed to a target (`rr/validation.go`, `rs/server_validation.go`) | recent event `doctor-bgp-validation-relay-failed` naming target and prefix count | none |
| AC-21 | A FlowSpec rule relayed before its UPDATE, or dropped because its source went down first (`rs/server_validation.go`) | recent event `doctor-bgp-rs-flowspec-out-of-order` naming source peer | none |
| AC-22 | An UPDATE forward discarded: no destination matched, or the source is down (`rs/server_forward.go`, `rs/server_withdrawal.go`) | recent event `doctor-bgp-rs-forward-discarded`; a discarded withdrawal is raised as an error event, a discarded announcement as a warning | none |
| AC-23 | `routeServer.pausedPeers` holds a source, or `workerPool.inBackpressure` holds its key (`rs/worker.go`; the check never reads `workerPool.backpressure`, which `BackpressureDetected` clears on read for dispatch) | `doctor-bgp-rs-source-paused`, warning, naming the source and how long it has been paused; runtime | a pause shorter than one second is not reported, because backpressure that drains is normal |
| AC-24 | A withdrawal inventory not updated (records not extracted, unknown encoding context, VPN path id or identity rejected; `rs/server_withdrawal.go`, `rs/server_inventory.go`, `rr/withdrawal.go`) | recent event `doctor-bgp-withdrawal-untracked`, error event: clients may keep stale routes; fix names the source peer reset command | none |

### BMP (`bmp`)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-25 | A configured receiver address not bound, or its accept loop ended (`receiver.go` `startListenerLocked`, `acceptLoop`) | `doctor-bmp-receiver-not-listening`, error, naming address and the bind error; runtime from a listener record kept beside `BMPPlugin.listeners`; on route `local` only, a port probe on the configured addresses with the same code (on the daemon route the bind fails on every address bmp serves, so the probe never judges there) | a listener closed by a reload (`listenerIsCurrent` false) is not reported |
| AC-26 | Active sessions at `max-sessions` and a router refused | `doctor-bmp-receiver-full`, warning, naming address, cap and refused count; runtime; fix names `max-sessions` | none |
| AC-27 | A monitored router sends malformed BMP, or a monitored UPDATE is too short (`bmp/bmp.go`, `rib/rib_inject.go`) | recent event `doctor-bmp-malformed-message` naming router and what was malformed | none |
| AC-28 | A configured collector whose session is not up (connect or Initiation failed, retried with backoff; `bmp/sender.go`) | `doctor-bmp-collector-down`, warning per collector, from the sender session state in the plugin's process; runtime | `doctor-bmp-unreachable` stays the local probe; a collector in its first connect attempt is not reported |
| AC-29 | A collector not reading its stream: session reset and full re-dump (`bmp/sender_drain.go`) | recent event `doctor-bmp-collector-slow` naming collector and queued bytes against the limit | none |
| AC-30 | Initial Adj-RIB snapshot aborted for a collector (`bmp/bmp_replay.go` `abortAdjReplay`) | `doctor-bmp-snapshot-aborted`, warning, naming collector and the BGP peers whose routes it lacks until they restart; runtime | none |
| AC-31 | `loc-rib true` while `bgp router-id` or `bgp session asn local` is missing | `doctor-bmp-loc-rib-no-identity`, warning: no Loc-RIB Peer Up, route monitoring or Peer Down is sent; local; fix names the missing leaf | none |
| AC-32 | Loc-RIB monitoring inert or lossy: no event bus, Loc-RIB OPEN not built, one best change not encoded (`bmp/bmp_locrib.go` `startLocRIB`) | `doctor-bmp-loc-rib-inert`, error when inert (runtime), recent event when one change failed | none |
| AC-33 | Peer Up or Route Monitoring for an Established peer not sent (OPEN cache miss, peer header not built; `bmp/bmp_events.go`) | `doctor-bmp-peer-not-reported` naming the peer; runtime from `BMPPlugin.peerUps` against established peers, plus a recent event per failure | none |

### Announce, commit, EPE, BGP-LS export, filter-family, IRR config (`cmd/announce`, `cmd/commit`, `epe`, `ls_export`, `filter_family`, `filter_irr`)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-34 | B8: a peer's `session irr as-set` fails `irr.ValidateASSetName` (`filter_irr/config.go`) | the commit is refused naming the value and the valid form (exact-or-reject); `doctor-bgp-irr-as-set-invalid`, error, from `ze doctor config <file>`; local | none |
| AC-35 | B2: a timed announcement expired and its withdraw was refused (`cmd/announce/registry.go`) | recent event `doctor-bgp-timed-withdraw-refused` (canonical), error event: the route stays on the peer untracked; fix names the withdraw command for that route | none |
| AC-36 | B9: an IRR-enrolled peer missing the refresh grant or Route Refresh (`peersMissingRefreshGrant`, `peersMissingRouteRefresh`) | `doctor-bgp-irr-no-refresh`, warning, naming each peer and which is missing; fix the `attach process bgp-filter-irr { send [ refresh ] }` line or the route-refresh capability leaf; local | none |
| AC-37 | B3: a named commit transaction open with queued routes (`transaction.CommitManager`; `cmd/commit/commit.go`) | `doctor-bgp-commit-open`, warning, naming the commit, its age and queued count; runtime through the command plugin; fix names the end-commit command | a commit open for less than one minute is not reported |
| AC-38 | B4: `bgp-epe` or the BGP-LS exporter configured as `plugin external` | `doctor-plugin-external-unsupported` (canonical, plugin-infra), error, through the "runs in-process only" field both plugins declare; local | none |
| AC-39 | B5: EPE snapshot replay failed (`epe/epe_plugin.go`) | recent event `doctor-bgp-epe-replay-failed` | none |
| AC-40 | B6: a BGP-LS topology snapshot refused (over the route limit, encode failure), the snapshot request failed, or an advertisement to a BGP-LS peer failed (`ls_export/export_plugin.go`, `ls_export/export_state.go`) | `doctor-bgp-ls-export-stale`, warning, naming source or peer and reason, from `topologyExporter` per-source last error; runtime | none |
| AC-41 | B7: `filter-family` asked about a filter it holds no instance for (`filter_family/handler.go`) | recent event `doctor-bgp-filter-family-no-instance`, error event: every route through that filter is rejected; the runtime check names filters the peers' chains reference that `instancesByName` lacks | none |

### IRR lists, modify and community filters (`filter_irr`, `filter_modify`, `filter_community`)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-42 | B10: an ASN with no IRR list or a failed lookup (`st.lastErr`, `st.list` nil, `st.lastOK`; `filter_irr/filter_irr.go`) | `doctor-bgp-irr-list-missing`, error: the ASN's prefixes are rejected (fail closed); names ASN, last error and last success; runtime, declared in the `filter_irr` runner's `sdk.Registration` `DoctorChecks` and answered by its `OnDoctorCheck`: one runtime check shared with AC-43 and with `plan/release/validation/spec-doctor-coverage-15-system.md` AC-19 (`doctor-bgp-irr-stale-data`); fix names `show bgp irr` | an ASN within its first lookup attempt is not reported |
| AC-43 | B11: `readmitter.full` set (`filter_irr/readmit.go`) | `doctor-bgp-irr-readmit-full`, warning, with held count against `heldPrefixMax`; runtime, from the same `filter_irr` runtime check as AC-42 (Stage 1 declaration, `OnDoctorCheck`) | none |
| AC-44 | B12: a modify filter with `del { med; }` referenced from an export chain (`filter_modify/filter_modify.go`) | `doctor-bgp-modify-med-ignored`, warning, naming filter and chain: the deletion is not applied on export (RFC 4271 Section 5.1.4); local | none |
| AC-45 | B13: blackhole-tagged routes reach a destination that did not agree to the BLACKHOLE community (`filter_community/community_filter.go`) | `doctor-bgp-blackhole-withheld`, warning, naming the destination and the agreement leaf as the fix; local | none: withholding is RFC 7999 behavior, and the row explains it |

### Graceful Restart (`gr`)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-46 | B14: a GR retention verdict not encoded or not emitted (`gr/gr.go`) | recent event `doctor-bgp-gr-retention-not-sent`, error event: bgp-rr and bgp-rs withdraw a restarting peer's routes | none |
| AC-47 | B15: a mark-stale, attach-community or purge command to the RIB failed (`gr/gr.go`) | recent event `doctor-bgp-gr-rib-command-failed`, error event, naming peer and command | none |
| AC-48 | B16: a configured GR family left out of the advertised capability (over the RFC 4724 maximum, unknown family, not storable) (`gr/gr_capability.go`) | `doctor-bgp-gr-family-not-advertised`, warning, naming peer, family and reason; runtime, because runtime families come from plugins | none |
| AC-49 | B17: LLGR egress state not loaded in the process running the egress filter (`gr/gr_egress.go`) | `doctor-bgp-llgr-egress-unloaded`, error: stale routes are withdrawn or depreferenced as if no destination were LLGR-capable; runtime | none |
| AC-50 | B18: a peer's OPEN carried capability 64 or 71 that the gr plugin could not read (`gr/gr.go`) | `doctor-bgp-gr-capability-unreadable`, warning, naming the peer: helper mode is not applied when it restarts; runtime from a per-peer record; the five Debug sites become Warn under this code | none |

### Healthcheck and watchdog (`healthcheck`, `watchdog`)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-51 | B19: a probe in DOWN, or timing out (`healthcheck/probe.go`) | `doctor-bgp-healthcheck-down`, warning, naming probe, state, last failure and the route effect; runtime; a probe exiting non-zero with no output still records its exit status | a probe DISABLED by config is not reported |
| AC-52 | B20: a state hook command failed or timed out (`healthcheck/hooks.go`) | recent event `doctor-bgp-healthcheck-hook-failed` naming probe and hook | none |
| AC-53 | B21: healthcheck could not drive bgp-watchdog (dispatch failed or non-done status; `healthcheck/healthcheck.go`) | recent event `doctor-bgp-healthcheck-watchdog-failed`, error event: the state change did not reach the route | none |
| AC-54 | B22: a probe `group` names no watchdog pool any peer configures (`watchdog/server.go`, `healthcheck/healthcheck.go`) | `doctor-bgp-healthcheck-group-unused`, warning: announce and withdraw affect zero peers; local | none |
| AC-55 | B23: a service IP not added to, or not removed from, its interface (`healthcheck/ip.go`) | `doctor-bgp-healthcheck-ip-unapplied`, warning, comparing `ipTracker.added` with the configured IPs; runtime | none |
| AC-56 | B35: the engine refused a watchdog announce or withdraw for a peer (`watchdog/server.go`) | recent event `doctor-bgp-watchdog-refused` naming peer, pool and reason; fix names the `attach process` send grant when that is the reason | none |
| AC-57 | B34: a watchdog route with a duplicate key or a family the index does not know (`watchdog/config.go`) | the commit is refused naming the route and the reason (exact-or-reject); `doctor-bgp-watchdog-route-dropped`, error, from `ze doctor config <file>`; local | none |

### Role (`role`)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-58 | B26: role or OTC checks drop routes (`role/metrics.go` `metricRouteRejects`, `metricRouteSuppressions` by reason and peer) | `doctor-bgp-role-routes-dropped`, warning, naming peer, reason and count in the last hour; runtime | none: a leak drop is the procedure working; the row lets the operator see it |
| AC-59 | B27: a session refused with Role Mismatch (`role/role.go` `applyValidateOpen`) | `doctor-bgp-role-mismatch`, error, naming peer, local and received role; runtime from a per-peer last verdict kept in the role plugin; fix names the peer's `role` leaf | none |
| AC-60 | B24, the address half: a role-configured peer with no usable remote ip (a name that is not an address; `role/config.go`) | `doctor-bgp-role-config-unapplied`, error: no Role capability and no OTC gate on that peer; local | a group placeholder peer whose remote ip is filled per member is not reported |

### RPKI (`rpki`, `rpki_decorator`)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-61 | B30: VRPs or ASPA records dropped at the 1,000,000 caps, or routes not tracked for revalidation (`rpki/roa_cache.go`, `rpki/aspa_cache.go`, `rpki/aspa_tracker.go`, `rpki/origin_tracker.go`) | `doctor-rpki-table-full`, error, naming the table and its size; runtime | none |
| AC-62 | B31: an action override on a peer with no static remote ip, or `blackhole-exempt` with no blackhole community (`rpki/rpki_config.go`) | `doctor-rpki-peer-config-unapplied`, warning, naming peer and leaf; local | none |
| AC-63 | B32: a peer with `remote as 23456`, asn4 disabled and ASPA enabled (`rpki/aspa_verify.go`) | `doctor-rpki-aspa-as-trans`, warning: every route from it is ASPA Invalid; local; fix names enabling asn4 | none |
| AC-64 | B28: an RTR session group with no cache in sync (no VRPs loaded, or serial stale; `rpki/rtr_session.go`) | `doctor-rpki-cache-down`, error, naming each cache, its retry time and last sync; runtime from the session group | `doctor-rpki-unreachable` stays the local host probe |
| AC-65 | B29: a cache sent a non-fatal Error Report (`rpki/rtr_session.go`) | recent event `doctor-rpki-cache-error-report` naming cache and RTR error code | none |
| AC-66 | B24, the value half: an invalid role, or an unrecognized or non-string export token (`role/config.go`) | the commit is refused by `verifyRoleConfig` naming the value and the valid forms (exact-or-reject); `doctor-bgp-role-config-unapplied`, error, from `ze doctor config <file>`; local | none |
| AC-67 | B33: a batch validation command or an RPKI or decorated event emit failed (`rpki/rpki.go`, `rpki_decorator/decorator.go`) | recent event `doctor-rpki-verdict-undelivered`, error event: the engine or subscribers miss validation state | none |

### Defects owned here

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-68 | bgp-rs's SDK run returns an error (`rs/server.go` `RunRouteServer`) | the log line names bgp-rs and carries `doctor-plugin-stopped` | none |
| AC-69 | The `statistics-timeout` YANG description in `bmp/yang/ze-bmp-conf.yang` | states what `setStatisticsTimeout` does: zero sends none, a non-zero value sends one Statistics Report per established peer to every collector each interval, and a change bounces the sender's sessions; `docs/guide/bmp.md` agrees with it | none |

### Every check

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-70 | Every check above | message states the cause in plain words and why it matters; `fix` holds the exact command; reachable by `show doctor` on every surface; its unit test goes red when the detection is removed (recorded at implementation) | none |
| AC-71 | Every runtime check with its plugin run forked (`./le mock plugin-external <name>`) | answers the same rows as internal | none: the reason this spec routes through the callback |
| AC-72 | Every Warn or Error site the inventory places in a class row of this area (B25's `role/roleless.go` included) | carries that class's code as its `code` attribute | none |

### Not problem classes: each line carries the named code

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-73 | A6 engine-to-plugin event refused, 50 sites (`adj_rib_in/rib.go`, `adj_rib_in/rib_gate_events.go`, `rib/rib.go`, `rib/rib_structured.go`, `rib/rib_validation.go`, `rib/rib_event_sdk.go`, `rr/rr.go`, `rs/server.go`, `rs/server_handlers.go`) | each line carries `doctor-internal-defect` and raises it as a recent event | none |
| AC-74 | A6 self-reported defects, 11 sites (`adj_rib_in/rib_eligibility.go`, `rib/rib_bestchange.go`, `rib/rib_event_sdk.go`, `rib/rib_inject.go`, `rib/rib_pipeline_best.go`, `rib/rib_sent_lifecycle.go`, `rr/withdrawal.go`, `rs/worker.go`) | `doctor-internal-defect`, raised as a recent event | none |
| AC-75 | A6 registration failures at init, 5 sites (`adj_rib_in/register.go`, `bmp/register.go`, `rib/rib_commands_community.go`, `rib/rib_inject.go`) | `doctor-plugin-setup-failed` | the three `os.Exit` sites reach stderr only, before any doctor runs: they carry the code for the gate and raise nothing |
| AC-76 | Config refusals answered to the committer: 4 A6 sites (`bmp/bmp.go`) and 1 A7 site (`watchdog/config_tx.go`) | each line carries `config-plugin-verify` as its `code`, the code of a plugin's config verify refusal, which the `config-semantics` check already declares (`semanticsDoctorCheck`, `internal/component/config/doctor.go`) | none |
| AC-77 | A6 shutdown race, 2 sites (`rs/server.go`) | `doctor-internal-defect` | none |
| AC-78 | A6 benign notes, 2 sites (`rib/rib_replay.go` duplicate replay suppressed, `adj_rib_in/rib_validation.go` early verdict expired) | logged at Info: nothing is wrong and no operator action changes them | none: `doctor-bgp-rib-note` (CODES.md) is not registered, because a code for a non-problem would put a healthy box in doctor |
| AC-79 | A7 registration failures, 27 sites (every `register.go` line, `gr/register.go`, `rpki/register.go`, `nlri/flowspec/register.go`, `nlri/srpolicy/register.go`); every plugin's SDK-run failure line (`rr/rr.go`, `aigp/aigp.go`, `gr/gr.go`, `rpki/rpki.go`, `epe/epe_plugin.go`, `ls_export/export_plugin.go`, `bmp/bmp.go`, `rib/rib.go`, `adj_rib_in/rib.go` and the rest of B1); the `route-action redistribute` line in `bmp/receiver.go` | the 23 plugin registrations carry `doctor-plugin-missing` (declared by the existing plugin-infra check, `internal/component/plugin/doctor/register.go`), the 4 auxiliary ones `doctor-plugin-setup-failed`, the run-failure lines `doctor-plugin-stopped`; the redistribute line is deleted by `spec-bmp-route-action-redistribute-acts-as-monitor` (A-5), so this spec gives it no code | none |
| AC-80 | A7 engine-contract invariants, 23 sites (`filter_community/handler.go`, `filter_irr/filter_irr.go`, `gr/gr.go`, `gr/gr_egress.go`, `gr/gr_capability.go`, `hostname/hostname.go`, `llnh/llnh.go`, `softver/softver.go`, `role/config.go`, `role/otc.go`, `role/removal.go`, `role/role.go`, `rpki/rpki.go`, `rpki/validate.go`) | `doctor-internal-defect`, raised as a recent event | none |
| AC-81 | A7 clamps behind YANG bounds, 4 sites (`gr/gr_capability.go`, `gr/gr_llgr.go`, `hostname/hostname.go`) | `doctor-internal-defect` | none |
| AC-82 | A7 RTR panic-guard follow-ups, 4 sites (`rpki/panic_guard.go`) | `doctor-internal-defect`, raised as a recent event | none |
| AC-83 | A7 RPKI log-only policy output, 3 sites (`rpki/rpki.go`) | logged at Info: it is the audit trail the operator chose with `log-only` | none: `doctor-rpki-log-only-retained` is not registered, for the reason of AC-78 |
| AC-84 | A7 outcomes reported to the caller: withdraw withheld (`cmd/announce/registry.go`), EPE teardown at stop (`epe/epe_plugin.go`) | the withdraw-withheld line at Info (the command succeeded); the EPE stop line `doctor-internal-defect` | none: `doctor-bgp-withdraw-withheld` is not registered, for the reason of AC-78 |
| AC-85 | `./le arch doctor-coverage report` after phase P10 and after `spec-bmp-route-action-redistribute-acts-as-monitor` has landed (A-5) | lists every Warn and Error site under `internal/component/bgp/plugins/` as covered, each with the code an AC above names | none |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs `show doctor` while a BMP receiver port is taken | `HandleDoctor` → callback → bmp `OnDoctorCheck` → listener record | `test/plugin/doctor-bmp-receiver-not-listening.ci` |
| 2 | runs `ze doctor config <file>` before loading a config with a bad IRR `as-set` | local route → `runPluginRegistryChecks` → filter_irr check | `test/ui/doctor-bgp-plugins-config.ci` |
| 3 | runs `show doctor` after a forked bgp-rs failed a peer-up replay | bgp-rs raise → engine op → bus → `doctor-recent-events`; bgp-rs runtime check | `test/plugin/doctor-bgp-rs-runtime-external.ci` |
| 4 | runs `show doctor` with the RPKI cache down | rpki `OnDoctorCheck` → session group | `test/plugin/doctor-rpki-cache-down.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| the first transient AC's raise test (the route itself is tested by child 12 AC-4) | the raising plugin's package | AC-1, both transports | |
| none here: the authenticated-source assertion is child 12's `TestReportRaiseRouteBothTransports` (`internal/component/plugin/server/dispatch_report_test.go`, its AC-4) | child 12 | AC-1 relies on it | |
| `TestBGPPluginCodesRegistered` | `internal/core/diagnostic/codes_test.go` | AC-2 | |
| `TestRTRErrorReportLogAttributes` | `internal/component/bgp/plugins/rpki/rtr_session_test.go` | AC-3 | |
| `TestBGPPluginRuntimeChecksAnswerBothTransports` | the `doctor_test.go` of each plugin with a runtime check | A-1, AC-71 | |
| `TestDoctorRIB*`, `TestDoctorAdjRIBIn*` (one per AC) | `rib/doctor_test.go`, `adj_rib_in/doctor_test.go` | AC-4 to AC-13 | |
| `TestDoctorRouteServer*`, `TestDoctorReflector*` | `rs/doctor_test.go`, `rr/doctor_test.go` | AC-14 to AC-24 | |
| `TestDoctorBMP*` | `bmp/doctor_test.go` | AC-25 to AC-33 | |
| `TestBMPStatisticsTimeoutDescription` | `bmp/yang_test.go` | AC-69: the description names the report it sends | |
| `TestDoctorAnnounce*`, `TestDoctorCommitOpen`, `TestDoctorEPE*`, `TestDoctorLSExport*`, `TestDoctorFilterFamily*` | the owning packages' `doctor_test.go` | AC-35, AC-37 to AC-41 | |
| `TestIRRASSetRefusedAtVerify`, `TestDoctorIRR*` | `filter_irr/doctor_test.go` | AC-34, AC-36, AC-42, AC-43 | |
| `TestDoctorModifyMEDIgnored`, `TestDoctorBlackholeWithheld` | `filter_modify/doctor_test.go`, `filter_community/doctor_test.go` | AC-44, AC-45 | |
| `TestDoctorGR*` | `gr/doctor_test.go` | AC-46 to AC-50 | |
| `TestDoctorHealthcheck*`, `TestWatchdogRouteRefusedAtVerify`, `TestDoctorWatchdog*` | `healthcheck/doctor_test.go`, `watchdog/doctor_test.go` | AC-51 to AC-57 | |
| `TestDoctorRole*`, `TestRoleInvalidFormsRefusedAtVerify` | `role/doctor_test.go` | AC-58 to AC-60, AC-66 | |
| `TestDoctorRPKI*` | `rpki/doctor_test.go` | AC-61 to AC-65, AC-67 | |
| `TestRouteServerRunFailureNamesBgpRs` | `rs/server_test.go` | AC-68 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| AC-23 paused duration before a row | 1 s and up | 1 s reported | 999 ms not reported | N/A |
| AC-37 open commit age before a row | 60 s and up | 60 s reported | 59 s not reported | N/A |
| AC-61 VRP table at cap | 0 to 1,000,000 | 999,999 not reported | N/A | 1,000,000 reported |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| child 12's external raise `.ci` (its AC-4, phase P0b) | named in `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` | a forked plugin's event shows in `show doctor`; this spec adds no route test | |
| `doctor-bgp-rib-runtime` | `test/plugin/doctor-bgp-rib-runtime.ci` | an unstorable family is reported | |
| `doctor-bgp-rs-runtime-external` | `test/plugin/doctor-bgp-rs-runtime-external.ci` | forked bgp-rs answers its runtime check | |
| `doctor-bmp-receiver-not-listening` | `test/plugin/doctor-bmp-receiver-not-listening.ci` | taken receiver port reported with fix | |
| `doctor-bgp-plugins-config` | `test/ui/doctor-bgp-plugins-config.ci` | local checks over a config file | |
| `doctor-rpki-cache-down` | `test/plugin/doctor-rpki-cache-down.ci` | RTR cache refusing reported | |
| `doctor-bgp-healthcheck-down` | `test/plugin/doctor-bgp-healthcheck-down.ci` | failing probe reported | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | no wire-visible BGP, BMP or RTR change: this spec reads state and reports it | - |

## Files to Modify
- `internal/core/diagnostic/codes.go` - every new code (P0, extended per phase)
- `internal/component/bgp/plugins/rib/`, `adj_rib_in/` - failure records, checks, codes (P2)
- `internal/component/bgp/plugins/rs/`, `rr/` - failure records, checks, codes; `rs/server.go` `RunRouteServer` message (P3)
- `internal/component/bgp/plugins/bmp/` - listener and collector records, checks; `yang/ze-bmp-conf.yang` description (P4)
- `internal/component/bgp/plugins/cmd/announce/`, `cmd/commit/`, `internal/component/bgp/transaction/`, `epe/`, `ls_export/`, `filter_family/` (P5)
- `internal/component/bgp/plugins/filter_irr/`, `filter_modify/`, `filter_community/` (P6)
- `internal/component/bgp/plugins/gr/` (P7)
- `internal/component/bgp/plugins/healthcheck/`, `watchdog/` (P8)
- `internal/component/bgp/plugins/role/`, `rpki/`, `rpki_decorator/` (P9)
- every remaining file the AC-73 to AC-84 groups name (P10)
- `docs/architecture/doctor-and-health-checks.md`, `docs/guide/bmp.md` (the raise method's pages, `docs/architecture/api/process-protocol.md` and `docs/plugin-development/`, are child 12 phase P0b's)

## Files to Create
- `internal/component/bgp/plugins/<plugin>/doctor.go` (+ `doctor_test.go`) for each plugin that has none
- the six `.ci` files this spec owns in the Functional Tests table (the raise route's `.ci` is child 12's)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-bmp-conf.yang` description text; the `ze-plugin-engine.yang` raise RPC is child 12 phase P0b's |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | the verify refusals (AC-34, AC-57, AC-66) live in each plugin's config verify |
| CLI commands/flags | N-A | no new command; fixes name existing ones |
| CLI grammar (keyword before value) | N-A | no new command |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | No | this spec adds no RPC; the raise RPC and its `.ci` are child 12 phase P0b's |
| Pipe completeness | N-A | `show doctor` output unchanged in shape |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | this spec |
| Prometheus counters/metrics | No | the failure records are doctor state; existing role metrics are read, none added |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | doctor coverage grows; no feature file states doctor coverage per plugin |
| 2 | Config syntax changed? | No | refusals of already-invalid forms; syntax unchanged |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | this spec adds no RPC; the raise RPC and its `process-protocol.md` section are child 12 phase P0b's |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` if it lists doctor checks per plugin; checked in P10 |
| 6 | Has a user guide page? | Yes | `docs/guide/bmp.md` (statistics-timeout, AC-69) |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | the SDK raise method is child 12 phase P0b's; this spec only calls it and declares Stage 1 doctor checks in an existing field |
| 9 | RFC behavior implemented, changed, or newly proven? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/doctor-and-health-checks.md`: plugin runtime checks through the callback (the raise route for forked plugins is documented by child 12 phase P0b) |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/core-design.md` section 15 (the report bus: its process-local statement gains the forked raise route in child 12 phase P0b, and this spec's sites change nothing it says); `docs/features/ai-first.md` (declared by `bmp/doctor.go` and `registry/doctor.go`: the plugin-owned doctor checks it describes gain the runtime kind); the rest derived per phase with `./le spec citation anchors spec plan/release/validation/spec-doctor-coverage-7-bgp-plugins.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/bmp.md` statistics example checked against the YANG in P4 |

## Implementation Steps

Phases are cut for one agent of about 60 tool calls each. Child 12 phases P0a and P0b (`plan/release/validation/spec-doctor-coverage-12-plugin-infra.md`) come before every AC that raises; P1 before every runtime check.

1. **Phase P0: codes** -- code registration (AC-2, transient codes through child 12 P0a), the RTR attribute rename (AC-3); AC-1 is proved by the first transient AC that raises, after child 12 P0b lands
   - Tests: `TestBGPPluginCodesRegistered`, `TestRTRErrorReportLogAttributes`
   - Files: `codes.go`, `rpki/rtr_session.go`
   - Verify: `ze show diagnostic` explains every code
2. **Phase P1: wiring** -- each plugin with a runtime AC declares its runtime check in its `sdk.Registration` and answers it with `OnDoctorCheck`, empty at first
   - Tests: `TestBGPPluginRuntimeChecksAnswerBothTransports`, `test/plugin/doctor-bgp-rib-runtime.ci` (red until P2)
   - Files: the runners of rib, adj_rib_in, rs, rr, bmp, cmd/commit, ls_export, filter_family, filter_irr, gr, healthcheck, role, rpki
   - Verify: each plugin's Stage 1 registration accepted (no `validateDoctorCheckDecls` or `refuseDoctorCheckDeclaredTwice` refusal: every runtime check name differs from the names in the plugin's registry row); each check reachable internal and forked. `./le plugin declarations check` judges commands and pipe aliases only, so it proves nothing about doctor checks
3. **Phase P2: RIB and Adj-RIB-In** -- AC-4 to AC-13, the per-peer split and promotion records
4. **Phase P3: route server and reflector** -- AC-14 to AC-24 (AC-16's route-server half is child 6 AC-25; its reflector half waits on R-6), the per-peer replay record, AC-68
5. **Phase P4: BMP** -- AC-25 to AC-33, listener and collector records, AC-69
6. **Phase P5: announce, commit, EPE, BGP-LS export, filter-family** -- AC-35, AC-37 to AC-41 (AC-38 once child 12 phase P4's in-process field, its AC-24, exists)
7. **Phase P6: IRR, modify, community filters** -- AC-34, AC-36, AC-42 to AC-45. `filter_irr` carries two checks: a config check (AC-34, AC-36) and a runtime check (AC-42, AC-43) that `spec-doctor-coverage-15-system` AC-19 joins with `doctor-bgp-irr-stale-data`; whichever spec lands first creates the runtime check
8. **Phase P7: GR** -- AC-46 to AC-50
9. **Phase P8: healthcheck and watchdog** -- AC-51 to AC-57
10. **Phase P9: role and RPKI** -- AC-58 to AC-67
11. **Phase P10: codes on every remaining line** -- AC-72 to AC-85; closes with `./le arch doctor-coverage report` over the area

Each of P2 to P9 follows the same order: write the check's unit test against a fixture state (red), keep the state the check reads, write the check and its code, raise the transient events, put the code on the class's log lines, then the phase's `.ci`.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has an implementation at file and symbol |
| Correctness | a runtime check reads only its own process's state; no check reads `ctx.Tree` at the runtime phase |
| Correctness | a transient raise happens once per failure, never per route |
| Naming | every code starts `doctor-`, kebab-case, registered once |
| Data flow | no daemon package names a BGP plugin; no plugin reads the reactor's grant predicate |
| Rule: user-facing-errors | every message gives problem, why, fix on its own line |
| Rule: no-layering | one raise route; one check and one code for the route-server grant fault (child 6 AC-25) |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| every code registered | `ze show diagnostic <code>` for each, in `TestBGPPluginCodesRegistered` |
| every site in the area covered | `./le arch doctor-coverage report` over `internal/component/bgp/plugins/` |
| runtime checks on both transports | `./le mock plugin-external <name>` per plugin with a runtime check |
| journal rows closed | both rows' Fix cells read `fixed:` in commit A |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | the raise engine op takes the source from the authenticated connection, never from the payload; fields bounded by the bus's limits |
| Resource exhaustion | failure records keyed by configured entities and removed with them; the bus ring bounded |
| Error leakage | messages name peers, collectors and caches the operator configured; no secret (TCP-MD5 key, RTR credentials) in a message |

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

- The A6 `statistics-timeout` row read the YANG description, not the producer: `setStatisticsTimeout` has sent the report since 10d66f9dbd. The row's real defect is the description (journal row below)

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/error-answered-with-another-errors-code.md` | 2026-11-10 | `RunRouteServer` logs `rr plugin failed` |
| `plan/journal/help-text-describes-superseded-behaviour.md` | 2026-11-10 | BMP `statistics-timeout` YANG description |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Runtime checks are answered by the owning plugin through `ze-plugin-callback:doctor-check` | a daemon-side `diagnostic.RegisterDoctorCheck` runtime check reading the plugin's package | the daemon-side check reads an empty copy when the plugin runs forked (`ai/doors/system.md`) |
| A forked plugin's transient event reaches the daemon bus through one SDK raise RPC | each plugin keeps its own recent-failure list and answers it from its runtime check | an internal plugin's raise already lands on the daemon bus, so a per-plugin list would show it twice; one bus keeps `doctor-recent-events` the only reader (parent O-2) |
| Invalid config forms (AC-34, AC-57, AC-66) are refused at verify and also reported from a config file | a doctor warning only | exact-or-reject (`ai/doors/system.md`); the doctor half lets `ze doctor config <file>` name them before a boot load |
| Benign notes, log-only RPKI output and withdraw-withheld drop to Info | register `doctor-bgp-rib-note`, `doctor-rpki-log-only-retained`, `doctor-bgp-withdraw-withheld` | a Warn that reports no problem puts a healthy box in doctor; Info is the level of output the operator asked for |
| The A6 session-ready rows and the A7 `watchdog` session-ready line share `doctor-bgp-session-ready-failed` | one code per plugin | one class: the report is timing only |
| New codes are named here | a central naming pass | the inventory: "A gap row with no name gets its code from its area child"; the names CODES.md already holds are used as written |

## Known Limitations

- `route-action redistribute` is owned by `plan/release/fixes/spec-bmp-route-action-redistribute-acts-as-monitor.md`
- The plugin-stopped class is owned by the plugin-infra child 12, and the OPEN refusal class by the bgp-engine child (inventory, "Merged rows")

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
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests (`-tags ze_bgp`, plus `ze_bmp` for `bmp`), the component groups and functional suites `./le repo changed scope print both` reaches: `./le test unit bgp` for the plugin packages, `./le test functional plugin` for the `test/plugin/*doctor*.ci` files and `./le test functional ui` for `test/ui/doctor-bgp-plugins-config.ci`. No agent runs any `./le verify` command
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
- [ ] Interop tests: N-A, no wire change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + the two journal rows marked `fixed:`
- [ ] **Commit B:** `remove plan/release/validation/spec-doctor-coverage-7-bgp-plugins.md` only, in the same `./le commit create` script
