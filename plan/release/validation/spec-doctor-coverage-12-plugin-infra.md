# Spec: doctor-coverage-12-plugin-infra

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Priority | P2 |
| Release reason | `ai/META-DECISIONS.md`, "Goal and scope": `ze doctor` reports every problem an operator can hit, each with its cause and the command that fixes it; O-3 of `spec-doctor-coverage-1-audit-and-gate` makes every area child release work |
| RFC gaps | - |
| Depends | `spec-doctor-coverage-1-audit-and-gate`: the `runtime` doctor phase, `Diagnostic.Fix`, the `recent-events` check (code `doctor-recent-events`, `internal/component/doctor/checks_recent_events.go`) and the doctor-coverage gate, landed in commits ce58dddf53 and fc96556110; commit 149497ce12 then made a plugin's runtime check run only through its Stage 1 callback (`validateDoctorCheckDef` refuses a `registry.DoctorCheckDef` at `runtime`, `refuseDoctorCheckDeclaredTwice` refuses a check declared on both routes), so every runtime check here registers through `diagnostic.RegisterDoctorCheck`. `plan/release/fixes/spec-doctor-reports-failed-plugin-startup.md` owns the five-stage startup-failure classes (`doctor-plugin-startup-phase-failed`, see Task); this spec registers their code and owns the `init()` setup-failure class (`doctor-plugin-setup-failed`, AC-45). This spec relies on no sibling area child for its checks; AC-40 waits on child 11 AC-34 for `doctor-config-reload-refused` (A-5). Every other area child depends on this spec's phase P0a (shared codes, event-code registration, fix convention) and cites it by this spec's path: children 3 to 11 and 13 to 15 (`plan/release/validation/spec-doctor-coverage-3-access-security.md` to `spec-doctor-coverage-11-hub-engine.md`, `spec-doctor-coverage-13-routing-protocols.md`, `spec-doctor-coverage-14-system-plugins.md`, `spec-doctor-coverage-15-system.md`). Phase P0b (plugin report route) is needed only by a child whose producers run inside a plugin that can be forked; children 4, 8 and 11 need only P0a, because their producers run in the daemon process and raise through `internal/core/report` directly |
| Phase | P0a to P8 committed; functional tests promoted; R-8, R-9, R-10, R-11 and R-12 resolved; the restart command is reachable from the CLI (ce86ac4a64) and was hardened after two independent reviews (325ec5d0d6); open: AC-40 (owner asked 2026-11-12; default is to wait on child 11 AC-34), the Review Gate and closure |
| Handoff | - |
| Updated | 2026-11-12 |

WIP cap: the owner granted a one-off claim above the WIP cap for this spec on 2026-11-11 (15 of 15 in flight at claim). The claim was taken again on 2026-11-12 above the cap for closure work (14 in progress, cap 12), under the same owner permission.

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent: `plan/release/validation/spec-doctor-coverage-0-umbrella.md`. Source: the plugin-infra and core section of the audit (`plan/audits/doctor-problem-inventory/plugin-infra.md`, slice A10) and the index `plan/audits/doctor-problem-inventory.md` ("Canonical codes", "Shared mechanisms the area children need", "Not problem classes", "Defects found" 21 to 23).

## Task

Thomas: "Not only the one you have, but all problems. So we will have to audit
the code for all the things we need to be added to it."

`ze doctor` reports every problem class of the plugin infrastructure
(`internal/component/plugin/**`) and of the core silent states (env, report
bus) with its cause and the command that fixes it. Every Warn and Error line in
the area names its doctor code (O-2), so the doctor-coverage gate counts it
covered.

This spec also OWNS the mechanisms every other area child uses:

| # | Work | Phase | Who depends on it |
|---|------|-------|-------------------|
| 1 | The cross-cutting codes, registered with metadata: `doctor-internal-defect`, `doctor-plugin-stopped`, `doctor-plugin-crashed`, `doctor-plugin-config-unapplied`, `doctor-plugin-setup-failed`, `doctor-plugin-startup-phase-failed`, `doctor-plugin-external-unsupported`, `doctor-audit-record-failed`, `doctor-plugin-request-refused`, `doctor-plugin-stop-cleanup-late` | P0a | every area child |
| 2 | The event-code registration: a transient code an owner registers is declared by the `recent-events` check, so a line or raise carrying it counts covered | P0a | every "recent event" row of every child |
| 3 | The fix convention: a report-bus raise carries its own fix command, rendered in `Diagnostic.Fix` | P0a | child 6 (bgp-engine) and every child raising on the bus |
| 4 | The plugin report route: a forked plugin raises and clears on the daemon's report bus, one SDK method pair and one `engineOps` entry, mirroring `metrics-publish` | P0b | children 7 (bgp-plugins) and 14 (system-plugins); replaces the P0 raise route child 7 planned for itself (its R-1) |
| 5 | The "runs in-process only" registration field and its check (`doctor-plugin-external-unsupported`) | P4 | children 3 (AC-94, `l2tp-auth-radius`), 7 (AC-38, epe and the BGP-LS exporter), 10 (AC-36, `iface-dhcp`) and 14 (AC-1, 18 registrations) declare the field on their plugins; the inventory's item 6 names no plugin of children 2, 13 or 15 |

Rows not re-specified here:

| Class | Owner | Reason |
|-------|-------|--------|
| A plugin failed its five-stage startup (`doctor-plugin-startup-phase-failed`) | `plan/release/fixes/spec-doctor-reports-failed-plugin-startup.md` AC-1 | that spec was written first for the same class; this spec registers the code (AC-1) |
| Boot left a later plugin phase failed (`doctor-plugin-startup-phase-failed`) | the same spec, AC-2, and its journal row `plan/journal/one-failure-skips-independent-startup-work.md` (2026-11-10) | same; the code is registered here (AC-1) |

`doctor-plugin-setup-failed` is not a startup class: it is a plugin's `init()` registration or setup failure, the outcome `registry.RecordSetup` records and `registry.SetupResults` replays (`internal/component/plugin/registry/setup.go`). This spec owns its check (AC-45, R-6).
| AC-9 rows (checks reading daemon state outside runtime) | none in this area | the three plugin checks (`plugin-binaries`, `plugin-shell`, `hub-managed-listener`) read the config tree, PATH and the host shell only; core registers only `internal/core/probe`, at `pre-config` |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - every plugin may run forked
  → Constraint: a direct `report.RaiseError` from a forked plugin writes the subprocess's own bus and reaches no doctor row; the raise reaches the daemon only through the plugin protocol
- [ ] `ai/doors/plugin-infra.md` - engine-op table, SDK generic, wire names
  → Constraint: "One engine-op entry per RPC"; the report route is one `engineOps` entry, and its method and JSON names are wire
  → Constraint: "One plugin failure stops the startup tier, by owner ruling"; nothing here makes a startup failure survivable
- [ ] `ai/doors/core.md` - core leaf registries, env
  → Constraint: a core registry answers "not registered" rather than an empty list; the env and report changes stay in core with no upward import
- [ ] `ai/doors/le-tooling.md` - the gate is le tooling
  → Constraint: `internal/le` is never imported by product code; the gate change stays in `internal/le/arch/doctorcoverage`
- [ ] `ai/doors/access-security.md` - `internal/core/audit` owns the audit log
  → Constraint: AC-33 reads the audit log's own last write outcome; transports keep using its `Recorder`
- [ ] `ai/doors/appliance.md` - a zefs key an image writes is a field contract
  → Constraint: the last-stop key (AC-8) is new; the previous image never reads it, so it needs no migration
- [ ] `ai/doors/system-plugins.md` - static owns its doctor checks
  → Constraint: AC-34 removes `static-fib-writer` only in favour of a check keyed on the registration, never a central list of producers
- [ ] `docs/architecture/doctor-and-health-checks.md` - phases, owners
  → Decision: a check reading live daemon state registers at `runtime`; the local route never runs it and reports `doctor-runtime-not-checked` when no daemon answers (child 1, O-1)
  → Constraint: "The runtime phase": a `registry.DoctorCheckDef` at `runtime` is refused (`validateDoctorCheckDef`, `internal/component/plugin/registry/doctor.go`, 149497ce12), so the class checks here are `diagnostic.DoctorCheck` values registered from `internal/component/plugin/doctor`'s `init()` and run by `RunRuntimeChecks`; a runtime check is handed the platform alone, never a tree
  → Constraint: "Every problem site names a doctor code": the gate reads covered codes from the parsed source (`coveredCodes` over the `checkDeclarations` table, `internal/le/arch/doctorcoverage/codes.go`), never from a linked registry, and a code must be a literal or a constant the scan resolves
- [ ] `docs/architecture/api/process-protocol.md` - five stages, runtime RPCs
  → Constraint: a runtime RPC from a plugin is refused before the handshake ends; the report route is a runtime RPC like `metrics-publish`
- [ ] `plan/learned/DESIGN-HISTORY.md`, Plugin system, "ANY plugin failure aborts the whole startup tier"
  → Decision: fail-closed boot is the owner ruling; a check reports state, it never keeps a failed plugin's tier running

**Key insights:**
- Two report-bus codes are raised today and declared by no doctor check: `plugin-crash` and `plugin-down` (`ProcessManager.ReportCrash`, `ProcessManager.Respawn`, `internal/component/plugin/process/manager.go`). `plugin-down` is raised only on the two respawn-limit paths.
- `deliveryDisagreements` and `unbackedClaims` already compute two classes; the checks call them.
- No runtime check can reach the plugin `Server` today: `DoctorCheckContext` (`internal/core/diagnostic/doctor_registry.go`) carries the tree, the config dir, the plugins config, the store and the platform, `RunRuntimeChecks` fills only the platform, and no process-wide handle to the running server exists.
- A setup-outcome registry already exists: `registry.RecordSetup`, `SetupResults`, `HardSetupFailures` (`internal/component/plugin/registry/setup.go`), read by `show plugin list` (`internal/component/plugin/register.go`, `declarations.go`) and by the boot gate `cmd/ze/hub/startup_gate.go` (`hardSetupFailure`); only `memlock` records into it today (`internal/plugins/memlock/register_lock.go`), and no doctor check reads it. It is filled in `init()`, before `main()`, so the local `ze doctor` process holds the same records as a daemon built from the same binary.
- A `registry.DoctorCheckDef` code must carry the `doctor-` prefix (`validateDoctorCheckDef`); a `diagnostic.DoctorCheck` code need only be lower kebab case, and two checks may declare one code (the gate's `Covered` set lists every declaring check per code).
- `config-plugin-verify` is already declared, by the `config-semantics` check (`internal/component/config/doctor.go`).
- No data-plane warning exists: `appendDataPlaneWriter` returns an operator error that refuses the whole boot or reload selection when a `NeedsDataPlane` producer has no writer (and an `UnloadedDependencyError` when the writer is compiled out); static's `static-fib-writer` reports the same condition at severity error.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/process/manager.go` - `ReportCrash` raises error `plugin-crash` on every unexpected exit; `Respawn` raises warning `plugin-down` only when the per-window or cumulative respawn limit disables a plugin, and clears it on a successful respawn
- [ ] `internal/component/plugin/process/register.go` - `health.Register("plugins", report.HealthProbeDown("plugin-down"))`
- [ ] `internal/component/plugin/process/process.go` - an internal runner returning non-zero logs Warn with attribute `code` set to the exit status; the stderr relay logs Error when it stops; `projectionErr` holds the first failed event application
- [ ] `internal/component/plugin/process/delivery.go` - a fire-and-forget batch failure logs Warn and sets `projectionErr`; `deliverViaConn` logs Warn `timing: slow deliverViaConn` for a batch over 500 ms; no failure or latency counter is kept
- [ ] `internal/component/plugin/server/failure_policy.go` - `applyFailurePolicy` calls `ReportCrash`, then restarts (`restartPlugin`, an error logged only), stops ze (`stopDaemonForPlugin`, log and SIGTERM, nothing persisted), or for `ignore` and unspecified logs Warn and raises nothing
- [ ] `internal/component/plugin/server/reload_tx.go` - `restartPlugin`; a failed `restoreCapabilities` after a refused reload logs Error
- [ ] `internal/component/plugin/server/startup.go` - `runPluginStartup`: phase 1 and `refuseBootAutoLoad` set `startupErr`; phases 2 to 5 log Error and return with `startupErr` unset (owned by the fixes spec, see Task)
- [ ] `internal/component/plugin/server/server.go` - `DoctorCheckPlugins` walks `AllProcesses`, the plugin-declared doctor checks
- [ ] `internal/component/plugin/server/dispatch_registry.go` - `engineOps`, the single table of plugin-to-engine RPCs, holds `metrics-publish`
- [ ] `internal/component/plugin/server/dispatch_metrics.go` - `opMetricsPublish`: bounds the payload, decodes, validates (`rpc.ValidateMetricsPublish`), refuses a nil owner, applies under the plugin's name
- [ ] `pkg/plugin/rpc/metrics.go`, `pkg/plugin/sdk/metrics.go`, `internal/core/ipc/yang/ze-plugin-engine.yang` - the `metrics-publish` wire types and bounds, the SDK publisher, the YANG RPC
- [ ] `internal/component/plugin/server/dispatch.go` - `resolveSubscriptionNamespace`: an unknown namespace skips the whole block with a Warn; a missing default namespace warns and continues
- [ ] `internal/component/plugin/server/startup_driver.go` - unparseable `ready` params warn and continue with no subscription
- [ ] `internal/component/plugin/server/delivery_reconcile.go` - `reconcileDelivery` only logs what `deliveryDisagreements` returns
- [ ] `internal/component/plugin/server/startup_claims.go` - `unbackedClaims` returns the advertised roles with no running claimant
- [ ] `internal/component/plugin/server/poststartup.go` - `reportPostStartupFailure` logs Warn; no delivered flag is kept
- [ ] `internal/component/plugin/server/enricher.go` - an enricher registration or call failure logs Warn
- [ ] `internal/component/plugin/server/startup_removal.go` - `completeRemovalRecovery` keeps `recoveryErr` on the removal attempt
- [ ] `internal/component/plugin/server/config_tx_bridge.go` - a failed `config-committed` send logs Error only
- [ ] `internal/component/plugin/server/command_registry.go` - `warnDeprecated` logs once per old name
- [ ] `internal/component/plugin/server/ensure.go` - a failed ensure-exists rollback logs Warn only
- [ ] `internal/component/plugin/server/command.go` - a failed `audit.Record` logs Warn only
- [ ] `internal/component/plugin/server/managed_serve.go` - a listener that cannot bind is skipped with an Error; a refused client, a refused config ack, a full notify queue and a failed notify build log Warn
- [ ] `internal/component/plugin/server/engine_event.go` - `invokeEngineHandler` recovers a panic and logs Error
- [ ] `internal/component/plugin/server/reload.go` - a config transaction that does not unwind at stop logs Warn
- [ ] `internal/component/plugin/server/startup_autoload.go` - `appendDataPlaneWriter` refuses the whole boot or reload selection (an `opererr`, or `UnloadedDependencyError` for a compiled-out writer) when a `NeedsDataPlane` producer is selected, no `fib` plugin is chosen, and no writer programs the configured data plane; it logs nothing at Warn
- [ ] `internal/component/plugin/ipc/tls.go` - `validPluginName` refuses a connect-back name; the acceptor logs Warn; the start then waits for its timeout
- [ ] `internal/component/plugin/yang/ze-plugin-conf.yang` - `list external` key `name` is a string of length 1..64 with no pattern
- [ ] `internal/component/plugin/registry/registry.go` - `Registration` has `NeedsDataPlane`, `DataPlane` and no "runs in-process only" field
- [ ] `internal/component/plugin/doctor/register.go` - the three post-config plugin checks
- [ ] `internal/plugins/static/doctor.go` - `static-fib-writer` (a post-config `registry.DoctorCheckDef`), code `doctor-static-no-fib-writer`, severity error, from `fibWriterDiagnostics`
- [ ] `internal/component/plugin/registry/setup.go` - `RecordSetup`, `SetupResults`, `HardSetupFailures`: the recorded `init()` setup outcome per plugin
- [ ] `internal/component/plugin/registry/doctor.go` - `validateDoctorCheckDef` refuses `Phase` runtime
- [ ] `internal/component/config/doctor.go` - `config-semantics` already declares `config-plugin-verify`
- [ ] `internal/core/report/report.go` - `RaiseWarning`, `RaiseError`, `ClearWarning`; `evictOldestWarning` drops the oldest warning past `warningCap`; the error ring overwrites silently; caps from `ze.report.warnings.max` and `ze.report.errors.max`
- [ ] `internal/core/env/env.go` - `GetInt`, `GetInt64`, `GetDuration` return the default on an unparsable value with no record; `GetBool` writes to stderr only
- [ ] `internal/core/diagnostic/codes.go` - the `builtinCodes` metadata table, registered by `RegisterBuiltinCodes` from the binary entry point (not `init()`), that `Lookup` (`registry.go`) reads
- [ ] `internal/component/doctor/checks_recent_events.go` - `recentEventsCheck` (name `recent-events`) renders a bus event under its own code when `Lookup` describes it, else under `doctor-recent-events`; its fix is always `show errors` or `show warnings`; its `Codes` holds only `doctor-recent-events`
- [ ] `internal/component/config/system/doctor_dns_source.go` - `checkDNSSourceSelections` reads `ProcessResolvers()` (`resolv.go`): the shape of a runtime check reading a process registry filled only in the daemon
- [ ] `pkg/zefs/keys.go` - the registered zefs key declarations (`MustRegister(KeyEntry{...})`)
- [ ] `internal/le/arch/doctorcoverage/sites.go` - `siteOf` needs a selector call; `looksLikeLog` refuses a message that is neither a constant, an `Error()`/`Problem()` call, nor followed by attributes
- [ ] `internal/le/arch/doctorcoverage/codes.go` - `coveredCodes` reads the covered set from composite literals of the `checkDeclarations` types (`diagnostic.DoctorCheck`, `registry.DoctorCheckDef`, `rpc.DoctorCheckDecl` and its SDK alias, `kernelcap.Capability`); `TestDoctorCoverageSourceHoldsEveryRegisteredCode` compares that table with the linked registries

**Behavior to preserve:**
- Fail-closed boot (owner ruling); `fatal` policy stops ze
- `show errors`, `show warnings` and the `plugins` health row name
- The `metrics-publish` route and every existing `engineOps` entry
- `doctor-hub-managed-collision` (the collision half of the managed listener class)

**Behavior to change:** the classes and defects below.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Doctor check registry and runtime phase | `internal/core/diagnostic` `RegisterDoctorCheck`, `DoctorPhaseRuntime`; `RunRuntimeChecks` (`internal/component/doctor/registry.go`); child 1 | Phased checks; runtime runs only in the daemon; since 149497ce12 a plugin `registry.DoctorCheckDef` cannot be `runtime` (`validateDoctorCheckDef`) | extends: every class check here is a `diagnostic.DoctorCheck` registered from `internal/component/plugin/doctor` at `runtime`, except the local ones named in their AC, which register at a config phase |
| Code metadata | `internal/core/diagnostic/codes.go` `builtinCodes`, `Lookup` (`registry.go`) | Describes each code for `ze show diagnostic` | extends: every code this spec names gets an entry (AC-1) |
| `Diagnostic.Fix` | child 1, `internal/core/diagnostic` | A fix command per finding | extends: every row here fills it; the bus carries it (AC-3) |
| Recent-events bridge | `internal/component/doctor/checks_recent_events.go` `checkRecentEvents`, `recentEventsDiagnostic` | Renders bus events under their own code when described | extends: declares registered event codes (AC-2) and renders a raise's own fix (AC-3) |
| Report bus | `internal/core/report/report.go` | Active warnings, an error ring, caps | extends: a fix field (AC-3), eviction accounting (AC-7) |
| Health probes on the bus | `report.HealthProbeDown`; `internal/component/plugin/process/register.go` | `plugins` row reads down while `plugin-down` is active | extends: filter follows the rename (AC-6) and reads degraded after an eviction (AC-7) |
| Plugin-to-engine RPC table | `internal/component/plugin/server/dispatch_registry.go` `engineOps`; precedent `opMetricsPublish` (`dispatch_metrics.go`), `rpc.MethodMetricsPublish`, `ze-plugin-engine.yang` `rpc metrics-publish`, `sdk.Plugin.FlushMetrics` | One entry per RPC, both transports derived | extends: one entry for report raise and clear (AC-4) |
| Plugin-declared doctor checks | `internal/component/plugin/server/server.go` `DoctorCheckPlugins`, read by `collectDoctorChecksFrom` (`internal/component/doctor/cmd/show.go`); `refuseDoctorCheckDeclaredTwice` (`startup.go`) | Bridges each running plugin's own Stage 1 checks through its doctor callback | does not apply: it lists checks of running plugins only, so it cannot report a plugin that is not running; the new checks read the server instead |
| Process registry read by a runtime check | `internal/component/config/system/doctor_dns_source.go` `checkDNSSourceSelections` over `ProcessResolvers` (`resolv.go`); core-leaf shape `internal/core/ipsecinventory` | A registry filled in the daemon, read at runtime | extends the shape: the plugin server publishes a value-type snapshot the runtime checks read (AC-5) |
| Plugin setup-outcome registry | `internal/component/plugin/registry/setup.go` `RecordSetup`, `SetupResults`, `HardSetupFailures`; boot gate `cmd/ze/hub/startup_gate.go` | Records each plugin's `init()` setup outcome; `show plugin list` replays it; a hard failure refuses boot | extends: a `pre-config` check over `SetupResults` declares and reports `doctor-plugin-setup-failed` (AC-45, R-6); every site carrying that code records its outcome through `RecordSetup` |
| Process manager state | `internal/component/plugin/process/manager.go` `disabled`, `GetProcess`, `configs`, `ze_plugin_status` gauge | Knows configured, running and disabled plugins | extends: the snapshot of AC-5 derives from it |
| Existing producers of two classes | `Server.deliveryDisagreements` (`delivery_reconcile.go`), `Server.unbackedClaims` (`startup_claims.go`) | Compute the finding, only logged | extends: the checks call them (AC-15, AC-16) |
| Removal recovery record | `startup_removal.go` `pluginRemoval.recoveryErr` | Kept until retry | extends: read by AC-28 |
| Runtime state store | `statestore` with a key in `pkg/zefs/keys.go` (`ai/doors/system.md`) | Daemon persistent state | extends: the last-stop record (AC-8) |
| Env registry | `internal/core/env` `EnvEntry.Type`, `AllEntries` | Declares each key's type | extends: invalid reads recorded and judged (AC-35) |
| Audit log | `internal/core/audit` `Log.Record` | Writes an audit entry, returns its error | extends: keeps its last write outcome for AC-33 |
| Data-plane writer resolution | `registry.NeedsDataPlane`, `registry.PluginForDataPlane`, `registry.GatedDataPlaneWriter`; `Server.appendDataPlaneWriter` | Refuses the boot or reload selection when no writer exists | extends: one check over every `NeedsDataPlane` producer reporting that refusal before start; replaces `static-fib-writer` (AC-34) |
| Managed collision check | `internal/component/plugin/doctor/register.go` `hub-managed-listener` | Collision with the plugin acceptor, local | does not apply to the other bind failures; kept, AC-25 adds the runtime half |
| Doctor-coverage gate | `internal/le/arch/doctorcoverage` `sitesOf`, `siteOf`, `looksLikeLog` (`sites.go`); `coveredCodes`, `checkDeclarations` (`codes.go`) | Finds problem sites, refuses uncovered additions; reads covered codes from declaration literals in the source | extends: two missed site shapes (AC-43), and the event-code registration's literal type as one more `checkDeclarations` row (AC-2, A-1) |
| Startup failure classes | `plan/release/fixes/spec-doctor-reports-failed-plugin-startup.md` | Skeleton owning setup-failed and phase-failed | does not apply: owned there; this spec registers the codes |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A plugin process event (exit, stall, refusal) in `process` or `server`; a forked plugin's report RPC; an env read; a bus raise
- `show doctor` (runtime route) or `ze doctor` (local route, asks the daemon first, child 1 O-1)

### Transformation Path
1. The producer decides a failure and records it where the class row names: the server snapshot (AC-5), a per-plugin field, the bus with a code and a fix (AC-3), or the persisted last-stop record (AC-8)
2. A forked plugin's raise arrives over the report route, is validated and raised on the daemon bus under the plugin's authenticated name (AC-4)
3. A runtime check reads the snapshot or record and answers rows with code, severity, message and fix
4. The `recent-events` check renders transient bus events under their registered code with the raise's own fix (AC-2, AC-3)
5. The local route answers the local checks (AC-11, AC-12, AC-23, AC-24, AC-34) and asks the daemon for the rest

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Forked plugin to engine | the report RPC over the plugin connection, JSON | No |
| Internal plugin to engine | the same entry through DirectBridge | No |
| Daemon to local CLI | `show doctor` over the daemon route (child 1) | No |
| Daemon restart | the last-stop record in the zefs store | No |

### Changed-Wire Census

**Search evidence:** scope `pkg/plugin/`, `internal/component/plugin/`, `internal/core/ipc/yang/`, `internal/core/report/`, `internal/component/doctor/`, `pkg/zefs/`, `test/`; terms `ze-plugin-engine:metrics-publish`, `engineOps`, `RaiseError(`, `RaiseWarning(`, `"plugin-crash"`, `"plugin-down"`, `HealthProbeDown`, `show errors`. Positive control: `metrics-publish` appears in the YANG, `rpc`, `sdk` and `engineOps`. Every in-tree raise of `plugin-crash` and `plugin-down` is in `process/manager.go`; the only probe is `process/register.go`. No raise method exists in `pkg/plugin/sdk`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Plugin report RPC | writer | `pkg/plugin/sdk` new raise and clear methods, and any third-party plugin outside the repository | none exists today | new; documented in `process-protocol.md` and `docs/plugin-development/` |
| Plugin report RPC | reader | `internal/component/plugin/server` new `engineOps` entry beside `opMetricsPublish` | mirrors `opMetricsPublish` | new |
| Report Issue JSON | writer | `internal/core/report` `Issue`, through `Warnings` and `Errors` | gains a `fix` key | additive key, omitted when empty like `detail` |
| Report Issue JSON | reader | `internal/component/doctor/checks_recent_events.go` `groupRecentEvents`; the `show errors` and `show warnings` handlers | read code, subject, message | extend to read the fix |
| Plugin bus codes | writer | `internal/component/plugin/process/manager.go` `ReportCrash`, `Respawn` | only raise sites | renamed to `doctor-plugin-crashed`, `doctor-plugin-stopped`; no release shipped the old names |
| Plugin bus codes | reader | `internal/component/plugin/process/register.go` `plugins` health row | only probe | renamed in the same commit |
| Last-stop record | writer | new key in `pkg/zefs/keys.go`, written by the plugin server before a plugin-caused stop | none exists | new key |
| Last-stop record | reader | the local fatal-stop and stop-cleanup checks (AC-11, AC-32) | none exists | new |

### Integration Points
- `applyFailurePolicy`, `Respawn`, `restartPlugin` - every down path updates the snapshot and the bus
- `opMetricsPublish` - the template for the report entry
- `recentEventsDiagnostic` - reads the raise's fix

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | a forked plugin's raise goes through `engineOps`, never a direct bus call in the subprocess |
| No unintended coupling (components stay isolated) | Yes | the checks live in `internal/component/plugin`; core stays a leaf (env and report import nothing new) |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing Mechanisms: every row extends; `static-fib-writer` is replaced, not doubled |
| Zero-copy preserved where applicable (refs, not copies) | Yes | N-A: no wire encoding path; snapshots are value types (`ai/doors/core.md`) |
| Registration over hardcoding, outbound | Yes | codes register in `codes.go`; event codes register from the owner's `init()`; the in-process field is on each `Registration` |
| Registration over hardcoding, inbound | Yes | searched `checks_recent_events.go`, `doctorcoverage` `coveredCodes` and `checkDeclarations`, `plugin/doctor/register.go` for code lists: the event-code registry and the registration field replace any list; the external-unsupported check never names a plugin |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The gate reads the linked doctor registry, so a code registered through the event-code registration counts covered | `internal/le/arch/doctorcoverage/codes.go` `coveredCodes` | AC-2 also edits the gate's covered set | read `coveredCodes` (2026-11-11) | broken: `coveredCodes` reads composite literals of the `checkDeclarations` types from the parsed source and never a linked registry (`docs/architecture/doctor-and-health-checks.md`, "Every problem site names a doctor code"); a code added to `recentEventsCheck.Codes` at run time is invisible to it. AC-2 therefore adds the event-code registration's literal type to `checkDeclarations` (its codes a literal or constant), and `TestDoctorCoverageSourceHoldsEveryRegisteredCode` gains the linked side of the same registration |
| A-2 | No command restarts one plugin today | grep of every YANG for a plugin restart RPC finds none | the AC-9 fix line names the existing command instead of adding one | grep in P2 (2026-11-11) | confirmed: no `.yang` held a plugin restart RPC; `request plugin restart <name>` was added (`internal/component/plugin/server/register_plugin_restart.go`, YANG in `internal/core/ipc/yang/ze-system-cmd.yang`), commit aa6ebb8d15; it was unreachable from the command line until ce86ac4a64 (Design Insights) |
| A-3 | `static-fib-writer` and the boot and reload refusal in `appendDataPlaneWriter` judge one operator problem (no writer for the selected data plane) | `internal/plugins/static/doctor.go`, `startup_autoload.go` `appendDataPlaneWriter` | keep static's check and add only epe's through the same registration | read static's check function in P5 (2026-11-11) | confirmed, with a difference: static's `fibWriterDiagnostics` judged only a main-table static route with no `fib` container, while the boot and reload refusal covers every selected `NeedsDataPlane` producer. The generic check (`plugin-dataplane-writer`, over `registry.DataPlaneWriter`, the one condition and text `appendDataPlaneWriter` now calls) follows boot's wider condition, so it also reports a named-table-only static config, `bgp-epe`, a compiled-out writer (`UnloadedDependencyError`) and an empty data plane, which the old check was silent on |
| A-4 | The command audit is written only through `audit.Log.Record` | `internal/component/plugin/server/command.go` | AC-33 covers each writer | grep `Record(` callers in P5 (2026-11-11) | confirmed: the only non-test `Recorder` is `(*audit.Log).Record`; its writers (REST, gRPC twice, web twice, SSH, MCP, `cli/model.go`, the hub reload, `plugin/server/command.go`) all reach the log's own last write outcome, so the two that discard the error (MCP, `cli/model.go`) are reported by the doctor row too |
| A-5 | The hub-engine area child registers `doctor-config-reload-refused`, and `config-plugin-verify` has a declaring check | inventory "Canonical codes" assigns it to hub-engine | AC-40 waits on that child; this spec registers neither | read child 11 and `internal/component/config/doctor.go` (2026-11-11) | confirmed: `plan/release/validation/spec-doctor-coverage-11-hub-engine.md` AC-34 registers `doctor-config-reload-refused`, declared by `hub-runtime`; `config-plugin-verify` is already declared today by `config-semantics` (`semanticsDoctorCheck`, whose `Codes` holds `diagnostic.CodeConfigPluginVerify`; re-read 2026-11-11), so only `doctor-config-reload-refused` waits on child 11 |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Two raise routes for forked plugins (child 7 planned its own P0) | a second `engineOps` entry for raises | P0b here is the one route (`ai/rules/no-layering.md`); resolved: child 7's own P0 is removed and `plan/release/validation/spec-doctor-coverage-7-bgp-plugins.md` and `spec-doctor-coverage-13-routing-protocols.md` use P0b |
| R-2 | A plugin floods the daemon bus through the route | bus at cap | the bus bounds apply per raise; AC-7 reports a full bus; the route refuses past a per-plugin rate |
| R-3 | Renaming `plugin-down` leaves a filter reading the old name | `plugins` row green while the check reports down | rename and filter in one commit, a test asserts the row reads down (AC-6) |
| R-4 | The startup-failure spec uses other code names | that spec's check emits a different code | resolved (2026-11-11): `plan/release/fixes/spec-doctor-reports-failed-plugin-startup.md` names `doctor-plugin-startup-phase-failed` for both of its classes and cites AC-1 here for the registration |
| R-5 | The persisted last-stop record survives and reports a stale reason | doctor reports a fatal stop after a clean restart | cleared on a clean start (AC-8) |
| R-6 | `doctor-plugin-setup-failed` carries two meanings across the children | children 7 (AC-75, AC-79) and 10 (AC-2, AC-104) put it on `init()` registration-failure lines, children 6 (AC-52) and 14 (AC-85) on an SDK run failing before ready | resolved (2026-11-11, main session, from the owner rule that `ze doctor` reports every problem an operator can hit with its cause and fix, and one fault one row): `doctor-plugin-setup-failed` means a plugin's `init()` registration or setup failure, the outcome `registry.RecordSetup` records, declared and reported by AC-45's check over `SetupResults`. A five-stage startup failure, an SDK run failing before ready included, is `doctor-plugin-startup-phase-failed`, declared by the startup check of `plan/release/fixes/spec-doctor-reports-failed-plugin-startup.md`. Child 6 AC-52 and child 14 AC-85 take `doctor-plugin-startup-phase-failed`; children 7 (AC-75, AC-79) and 10 (AC-2, AC-104) keep `doctor-plugin-setup-failed` |
| R-7 | The not-problem codes have no declaring check | AC-36 to AC-39 and AC-42 put `doctor-internal-defect`, `doctor-plugin-request-refused`, `doctor-plugin-answer-undelivered` and `doctor-plugin-drain-incomplete` on log lines, and AC-1 needs each declared by a check or by the AC-2 event-code registration; a log-only line never reaches the bus, so declaring it through `recent-events` is the false declaration the first Key Design Decision refuses | resolved (2026-11-11, main session, same owner rule): a code whose only sites are log lines is not reported by doctor, so every such site also raises an error event on the report bus under the same code, where `recent-events` renders it, and the code is declared through the AC-2 event-code registration. AC-36 to AC-39 and AC-42 say so, and AC-44 counts those sites covered |
| R-8 | AC-22's record never clears: `doctor-plugin-config-unapplied` reports while a plugin's last `config-committed` outcome failed, and the fix names `ze signal reload`, but whether a reload with an unchanged config re-sends `config-committed` is unverified (P3 handoff) | the row stays after the operator ran the fix | resolved (2026-11-12, 909da87de2), and worse than recorded: a reload cannot clear the record, because `reloadConfig` returns on an empty diff before any transaction and `runTxCoordinator` returns with no affected plugin. A plugin restart clears it (a new process has fresh records and is configured from the committed tree), but `request plugin restart <name>` refused a running plugin while six rows named it for a running plugin: AC-14, AC-17, AC-18, AC-19, AC-21 and AC-22. Fix: the restart accepts a running plugin (stopped and replaced through `restartPlugin`, the existing respawn path), refuses while a config transaction is in flight, and answers whether it stopped a running plugin (`handlePluginRestart`); AC-22's fix names the restart, and the subtest `TestDoctorClassChecks/AC-22 config unapplied` now matches the restart command exactly, which an independent judge refused while the restart still refused a running plugin and approved once the command was fixed (`Weakening-approved: doctor.TestDoctorClassChecks` in 909da87de2). Tests in `internal/component/plugin/server/plugin_restart_running_test.go`. Making the restart act on a running plugin (909da87de2) introduced a blocker an independent review found: the restart held the config transaction lock while it waited without bound for the old process, so a plugin restarting itself or a stuck engine hung the command and every later commit and reload; fixed in 325ec5d0d6, which bounds every wait of the restart and refuses a self-restart. What the reviews found beyond it is owned by `plan/release/fixes/spec-plugin-restart-bounded-and-configuration-true.md` (105d71fa11) |
| R-9 | `recordEnricherFailure` keeps 16 failures per plugin and drops the rest uncounted (P3 handoff), so the AC-21 row understates a plugin with many failing enrichers and says nothing of the loss | more than 16 enricher keys failing on one plugin | resolved (2026-11-12, 909da87de2): enricher failures past the bound are counted and the row says how many more fail. The same defect in three sibling bounded records was fixed the same way: subscription drops were counted but shown by no row (now shown, AC-14); engine handler panics past the bound (AC-18); the delivery ring overwrote uncounted, and the row now says "at least" while an overwritten problem is inside the window (AC-19). The engine handler panic record never cleared; it now has a five-minute window like the delivery window |
| R-10 | Producer lines no unit test drives, so deleting one leaves every test green. P3: `startup_driver.go` `sink.readyParamsRefused`, the `NoDefaultNamespace` drop in `dispatch.go`, `sendPostStartupTo` Pending and no-connection outcomes, the `makeProxyCall` enricher call-failure record and its clear, `dispatchCommitted` with no connection, the `safeDeliverBatch` panic `recordProjectionErr`. P5: the two `reload_tx.go` call sites of the capability-restore record (failed and clear), `manager.go` storing `cleanupLate`, and `server.cleanup` calling `recordLateEngines` | a red-line walk over these lines stays green | resolved (2026-11-12): a test drives every listed line (e329b99f01 for nine lines, in `server/doctor_producer_lines_test.go` and `process/doctor_producer_lines_test.go`; 909da87de2 for the enricher call-failure record and its clear and the no-connection commit record, in `server/doctor_delivery_bounds_test.go`), and `./le test redwalk check` answered `red` for all twelve breaks (a clean pass, then the unit's own failure with the one line removed). Found on the way: the no-connection branch of `dispatchCommitted` was unreachable, because the process lookup required a connection, so its record could never be written; `committedTarget` makes it reachable. One seam was added: `server/dispatch.go` reads the default event namespace through a package variable |
| R-11 | An external plugin that ignores its closed connection during a `request plugin restart` of a running plugin (R-8) | the old process keeps running beside its replacement | resolved (2026-11-12, 9325dde3da): the restart of a well-behaved external plugin, one that exits when its connection closes, is proven by `test/plugin/plugin-restart-external-running.ci` (ce86ac4a64). A plugin that ignores the stop is proven by `test/plugin/plugin-restart-external-ignores-stop.ci` (fixture mode `ignore-first-stop`, 9325dde3da): on the tree with 325ec5d0d6 it passed one live run and 80 of 80 stress invocations, and with the daemon's group kill removed (`process.go` `superviseExit`) it went red (clean pass 4.0 s, broken fail 13.7 s) |
| R-12 | A `show` in flight through the old process's proxy enricher when `request plugin restart` replaces a running plugin (R-8) | that `show` fails or answers from the stopped process | resolved for a plugin run in the daemon (ae5d1d84fd): `TestRestartEnricherReadBeforeRestartBlamesNoReplacement` goes red without the guard, and `TestRestartCutsEnricherCallInFlight`, green under that break, proves the cut call returns and the restart completes (`server/enricher_restart_test.go`). Remains: the forked path has no test |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | doctor rows wrong or missing; the `plugins` health row; a forked plugin's raise refused |
| How is it reverted? | commit revert per phase; the zefs key is unused after revert |
| Who else touches this path? | every area child (shared codes, route, fix convention); `spec-doctor-reports-failed-plugin-startup` (startup.go); child 7 (route) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show doctor` with a configured plugin stopped under `ignore` policy | → | plugin-stopped runtime check | `test/plugin/doctor-plugin-stopped.ci` |
| a forked plugin calls the SDK raise | → | report `engineOps` entry, `doctor-recent-events` | `test/plugin/doctor-plugin-report-raise-external.ci` |
| a bus raise carrying a fix | → | `recentEventsDiagnostic` fix | `TestRecentEventsRendersRaiseFix` |
| `ze doctor` local, config `plugin external` naming an in-process-only plugin | → | external-unsupported check | `test/plugin/doctor-plugin-external-unsupported.ci` |
| `ze doctor` after a fatal plugin stopped ze | → | last-stop record check | `test/plugin/doctor-plugin-fatal-stop.ci` |
| `show doctor` with an env value its type cannot parse | → | env invalid-value check | `test/plugin/doctor-env-invalid-value.ci` |
| `ze cli -c "request plugin restart <name>"` against a running daemon | → | restart command handler (`handlePluginRestart`) | `test/plugin/plugin-restart-external-running.ci` |

## Acceptance Criteria

Every row: severity, code, the fix line, the state read, and the route. "Event" means a report-bus raise rendered by `doctor-recent-events` under the named code.

### Shared mechanisms (P0a, P0b, P1)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `ze show diagnostic <code>` for each code in Task row 1 and every code this spec names | answers title, description and an example for each; each is declared by a registered doctor check or by the event-code registration of AC-2, so the gate counts its sites covered | none |
| AC-2 | an owner registers a transient code from its `init()` (the event-code registration, symbol named at implementation in `internal/core/diagnostic`) and raises it | the `recent-events` check declares every registered event code, renders the event under that code, and `./le arch doctor-coverage check` counts a line or raise carrying it covered; a code raised with no registration renders under `doctor-recent-events` as today | a code registered with no metadata is refused at registration, because `ze show diagnostic` must describe it |
| AC-3 | a raise carries a fix command (one convention in `internal/core/report`, an `Issue` fix field) | `show warnings` and `show errors` show it; the recent-events row's fix line is that command | a raise with no fix keeps `show errors` or `show warnings` as its fix line, because that command still lists the event |
| AC-4 | a forked plugin and an internal plugin each call the SDK raise and clear methods (one pair, one `engineOps` entry, the same shape as `metrics-publish`) | the event appears on the daemon bus with source set to the plugin's authenticated name, and in `show doctor` under its code with its fix; a clear removes the active warning | a raise with an unregistered code, an oversized field or before the handshake ends is refused with an answer naming the field and the bound, because the bus would drop it silently |
| AC-5 | a runtime check asks for plugin state | it reads a value-type snapshot the running server publishes (configured, running, stage, disabled, last error per plugin, and the per-class records below); with no server published it answers "not published", never an empty healthy list | none |
| AC-6 | a plugin crashes, then is disabled at the respawn limit | the bus carries `doctor-plugin-crashed` and `doctor-plugin-stopped` (renamed from `plugin-crash`, `plugin-down`); the `plugins` health row reads down on the new name; the internal-runner non-zero exit line logs `exit-code` and carries `doctor-plugin-crashed` | none: no release shipped the old names |
| AC-7 | the bus reaches its warning cap and evicts, or the error ring wraps | runtime warning `doctor-report-bus-full` names the cap, how many entries were lost and the fix `ze.report.warnings.max` / `ze.report.errors.max`; a `HealthProbe*` row whose code could have been evicted reads degraded "report bus full" instead of healthy | a bus below cap reads as today, because nothing was lost |
| AC-8 | ze stops because of a plugin (fatal policy, boot refusal) or a stop-time cleanup ran late | the reason, the plugin and the time are written to a zefs key before the stop; the next run's checks read it (AC-11, AC-32); a clean start clears it | none |
| AC-47 | a plugin's run function logs the error `sdk.Plugin.Run` returned (`pkg/plugin/sdk/sdk.go`), which today is a startup failure wrapped `stage N (...)` for stages 1 to 5 or a failure after ready, told apart only by message text | the SDK marks every error `Run` returns before stage 5 completes with one exported, generic mark a caller tests with `errors.Is` or `errors.As` (the symbol named at implementation), so a run-failure site logs a startup failure under `doctor-plugin-startup-phase-failed` and a later failure under `doctor-plugin-stopped`, each line with one static code; children 6 (AC-52), 7 (AC-79) and 14 (AC-85) use it (R-6) | a connection closed during stage 5 still returns nil, as today, because the engine already holds the ready request |

### Plugin lifecycle (P2)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-9 | a configured plugin is not running: respawn limit, `ignore` or unspecified policy exit, failed `restartPlugin`, failed handshake | runtime error `doctor-plugin-stopped` naming the plugin and why, with a fix naming the command that starts it again (added as `request plugin restart <name>` if A-2 holds; the same command also restarts a running plugin, R-8); every down path raises `doctor-plugin-stopped` on the bus, so the `plugins` health row reads down; a successful start clears it. The restart is refused for an unknown name, a plugin asking for its own restart, a pending removal, a config transaction in flight, and while the previous process still owes its cleanup, which the stopped row then says (325ec5d0d6) | a plugin removed by config is not reported, because it is not configured |
| AC-10 | a plugin process ended unasked, restarted or not | event `doctor-plugin-crashed` naming the plugin, its exit status and the policy applied; fix `show errors` | none |
| AC-11 | ze stopped itself because a `fatal` plugin failed | local error `doctor-plugin-fatal-stop` on the next run, from the AC-8 record, naming plugin, reason and time; fix names the plugin's log and the config change | none |
| AC-12 | config auto-load dependencies do not resolve in this build | local error `doctor-plugin-autoload-unresolved` from `registry.ResolveDependencies` over the config's auto-load set, naming plugin, dependency and build tag, the same text as the boot refusal | none |
| AC-45 | a plugin's `init()` registration or setup failed, recorded through `registry.RecordSetup` as `SetupFailedHard` or `SetupFailedSoft` | a `pre-config` check (`diagnostic.DoctorCheck` registered from `internal/component/plugin/doctor`) over `registry.SetupResults` reports `doctor-plugin-setup-failed` per failed plugin, error for a hard failure (the one the boot gate `hardSetupFailure` refuses) and warning for a soft one, naming the plugin and the recorded reason, with the fix the reason names; it runs on both routes, because the records are written before `main()` in every process of the binary; every site that carries `doctor-plugin-setup-failed` (here and in children 7 and 10) records its outcome through `RecordSetup`, so the check reports what the line logs | a plugin recorded `SetupSucceeded` or `SetupUnknown` is not reported, because nothing failed or nothing was recorded; a startup failure after `init()` is `doctor-plugin-startup-phase-failed`, owned by `plan/release/fixes/spec-doctor-reports-failed-plugin-startup.md` |
| AC-48 | `ProcessManager.Respawn` replaces a plugin's process and its wait for the replaced process to end exceeds that process's stop bound | an error event `doctor-plugin-restart-overlap`, declared through the AC-2 event-code registration, names the plugin and what had not finished, with a ze command as its fix; `recent-events` reports it (0becf0044f, text corrected in 325ec5d0d6) | a process that ends in time raises nothing |

### Registration and delivery (P3)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-13 | a running plugin's command or deprecated alias was refused at registration (name owned) | runtime warning `doctor-plugin-command-not-registered` naming plugin, command and current owner; fix names the conflicting plugin to remove or rename | none |
| AC-14 | a plugin's subscriptions were dropped (unknown namespace, no default namespace, bad `ready` params) | runtime error `doctor-plugin-subscription-dropped` naming plugin and cause, read from a per-process dropped-subscription record | none |
| AC-15 | `attach process` and declared events disagree | runtime warning `doctor-plugin-delivery-disagreement`, one row per finding of `deliveryDisagreements(nil)`, fix the `attach process` change | none |
| AC-16 | an exclusive role is claimed with no claimant running | runtime error `doctor-plugin-claim-unbacked` per token of `unbackedClaims()`, naming the role and the claimant | none |
| AC-17 | a running plugin never received its post-startup callback | runtime error `doctor-plugin-poststartup-undelivered` from a per-plugin delivered flag | a plugin still starting is not reported, because the callback is not yet owed |
| AC-18 | an event handler panicked and was recovered, or `projectionErr` is set | runtime error `doctor-plugin-panic` naming plugin (or engine handler namespace and type) and the first error | none |
| AC-19 | fire-and-forget event batches failed, or a batch took over 500 ms, in the recent window | runtime warning `doctor-plugin-delivery-failed` from a per-plugin failure and latency window (not a lifetime counter) | none |
| AC-20 | a plugin's stderr relay stopped | event `doctor-plugin-stderr-relay-stopped` naming plugin and cause | the descendant-held-stderr end stays Debug, because the plugin already exited |
| AC-21 | a `show` enricher failed to register or its call failed | runtime warning `doctor-plugin-enricher-unavailable` naming plugin, command and key | none |
| AC-22 | a plugin was not told a transaction committed | runtime warning `doctor-plugin-config-unapplied` from the per-plugin last committed transaction against the daemon's; fix names the plugin restart, `request plugin restart <name>`, which accepts a running plugin | none. A reload cannot clear the record: an unchanged config runs no transaction (R-8) |

### External plugins and fleet (P4)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-23 | `plugin external <name>` whose name `validPluginName` refuses; or a connect-back refused for a token or an unknown name | the commit refuses the name with the allowed form (one declaration of the pattern, read by both config and `ipc`); a config already on disk gets local error `doctor-plugin-connect-back-refused`; a runtime refusal is event `doctor-plugin-connect-back-refused` naming the claimed name | none |
| AC-24 | a plugin whose registration declares it runs in-process only (new `Registration` field) is configured `external` | local error `doctor-plugin-external-unsupported` naming the plugin, with fix "move it to an `internal` block"; the check reads the field, never a list | plugins without the field are unaffected; each area child sets the field on its own plugins |
| AC-25 | a managed listener address could not bind for a reason other than the acceptor collision | runtime error `doctor-managed-listener-unbound` naming address and the kernel's error text, from the bound set against `s.addrs` | the collision keeps `doctor-hub-managed-collision`, already covered |
| AC-26 | a managed client was refused (name connected) or rejected the pushed config | runtime warning `doctor-managed-client-rejected` from the per-client last ack record | none |
| AC-27 | a managed client was not told of a config change | event `doctor-managed-client-notify-dropped` naming client and cause | none |

### Reload, commands, stop and audit (P5)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-28 | a refused removal was not restored | runtime error `doctor-plugin-removal-unrecovered` from `recoveryErr`, fix the reload that retries it | none |
| AC-29 | capability declarations not restored after a refused reload | runtime error `doctor-plugin-capabilities-not-restored`, kept until the next committed reload | none |
| AC-30 | an ensure-exists parent was not rolled back | event `doctor-command-ensure-rollback-failed` naming the parent path, fix the delete command for it | none |
| AC-31 | a client used a deprecated command name | event `doctor-command-deprecated-used`, fix the new name | once per name per session, as today |
| AC-32 | a plugin engine or the config transaction did not finish cleanup at stop | `doctor-plugin-stop-cleanup-late` written to the AC-8 record, reported locally on the next run; each owner's leftover-state check stays the real cover | none |
| AC-33 | an audit record failed to write | runtime error `doctor-audit-record-failed` from the audit log's last write error and time, one code for every surface | none |
| AC-34 | a configured `NeedsDataPlane` producer has no writer for the data plane (static, bgp epe), or its writer is compiled out | local error `doctor-plugin-no-dataplane-writer` naming producer and data plane, the same condition and text as the refusal `appendDataPlaneWriter` returns, which stops boot and reload (an error, as `static-fib-writer` is today); `static-fib-writer` is removed and its users converted (A-3) | an explicit `fib` block is not reported, because its plugin is the writer |

### Core (P6)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-35 | the daemon's environment holds a value its registered type cannot parse (int, int64, duration, bool) | `env` records the key, the value and the type at the read; runtime warning `doctor-env-invalid-value` names key, value, type and the default used, fix the corrected export; also judged for every registered entry against `EnvEntry.Type` | a secret key's value is not echoed, because `IsSecret` marks it |

### Not problem classes (P7)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-36 | config-tx payload faults (24), engine faults (13), init registration faults (3) in `internal/component/plugin/server` | each line carries `doctor-internal-defect`, and each site also raises an error event on the report bus under that code (R-7), so `recent-events` reports it; the code is declared through the AC-2 event-code registration | none |
| AC-37 | a plugin's request refused (4) | each line carries `doctor-plugin-request-refused` and raises an error event under it, declared the same way (R-7) | none |
| AC-38 | an answer not delivered (6) | each line carries `doctor-plugin-answer-undelivered` and raises an error event under it, declared the same way (R-7) | none |
| AC-39 | a drain incomplete (5) | each line carries `doctor-plugin-drain-incomplete` and raises an error event under it, declared the same way (R-7) | none |
| AC-40 | a reload refused and rolled back (5) | each line carries `config-plugin-verify` where a verifier refused, else `doctor-config-reload-refused` (A-5) | none. **Status (2026-11-12): open, waits on child 11 AC-34 (A-5); owner asked 2026-11-12 whether this spec waits on child 11 or declares the code here.** `doctor-config-reload-refused` is declared nowhere in the tree, so the five lines carry no code: `reload.go` `Server.reloadConfig` ("config reload: transaction failed"); `startup_autoload.go` `Server.selectAutoLoad` (dependency resolution failed) and `Server.startAutoLoaded` (auto-load startup failed; the reactor refused the config after the auto-loaded plugins started); `failure_policy.go` `Server.stopDaemonOnStartupFailure` (a fatal plugin's startup ended by its tier or caller during a reload). None is purely a verifier refusal, so none takes `config-plugin-verify` |
| AC-41 | the operator's goroutine dump (`request quit`) | logged at Info | none: it is the operator's own action |
| AC-42 | core typed-event payload mismatch drops and the report cap clamp | the drops carry `doctor-internal-defect` and raise an error event under it (R-7); the clamp carries `doctor-env-invalid-value`, which AC-35's runtime check declares and reports | none |
| AC-44 | `./le arch doctor-coverage report` after P7 | zero uncovered sites in `internal/component/plugin`, `plugin/ipc`, `plugin/process`, `plugin/server`, and the `plugins` health row covered; every not-problem site is covered by a code doctor reports (R-7) | the startup.go sites owned by `spec-doctor-reports-failed-plugin-startup` until it lands. **Status (2026-11-12): met except the five AC-40 lines, on which `TestNotProblemLinesCarryCodes` is red (29f5ce89ed); open until AC-40 closes** |

### Gate (P8)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-43 | a fixture commit adding a Warn whose message is not a literal (concatenation, variable) with no attributes, and a call to a wrapper that forwards its message and attributes to Warn (the dhcpserver `warnLimited` shape) | `./le arch doctor-coverage check` reports both as sites, the wrapper's call judged at the caller for its `code` attribute; the report counts flow-export's concatenated Warn and dhcpserver's `warnLimited` callers | a Warn whose level the source does not spell is no site, as today |
| AC-46 | a fixture commit adding an `rpc.DoctorCheckDiagnostic` literal with no `Fix` (the type a plugin's doctor callback answers, `pkg/plugin/rpc/types.go`, whose `Fix` field `collectDoctorChecksFrom` carries to the operator) | `./le arch doctor-coverage check` refuses it as it refuses a `diagnostic.Diagnostic` with no Fix, and `./le arch doctor-coverage report` lists the existing ones (copp, static, geodns, ldp, rsvpte, ike and the rest) under their owners; today `findingsOf` and `isDiagnosticType` (`internal/le/arch/doctorcoverage/sites.go`) judge only `internal/core/diagnostic`'s `Diagnostic` (journal row `gate-excludes-part-of-its-population.md`, 2026-11-11) | none: a plugin finding reaches the operator through the same renderer |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCrossCuttingCodesDescribed` | `internal/core/diagnostic/codes_test.go` | AC-1 | pass |
| `TestEventCodeRegistrationDeclared` | `internal/component/doctor/checks_recent_events_test.go` | AC-2 | pass |
| `TestRegisterEventCodeRefusesUndescribed`, `TestDoctorChecksAddEventCodes` | `internal/core/diagnostic/event_codes_test.go` | AC-2 | pass |
| `TestDoctorCoverageReadsCodesFromSource`, `TestDoctorCoverageSourceHoldsEveryRegisteredCode` | `internal/le/arch/doctorcoverage/check_test.go` | AC-2 | pass |
| `TestRecentEventsRendersRaiseFix` | `internal/component/doctor/checks_recent_events_test.go` | AC-3 | pass |
| `TestIssueFixCarriedAndOmitted`, `TestIssueFixBounded` | `internal/core/report/report_fix_test.go` | AC-3 | pass |
| `TestReportRaiseRouteBothTransports`, `TestReportIssueHandlerRefusals`, `TestReportIssueRaiseRate` | `internal/component/plugin/server/dispatch_report_test.go` | AC-4 | pass |
| `TestValidateReportIssue` | `pkg/plugin/rpc/report_test.go` | AC-4 | pass |
| `TestRaiseIssueSendsReportIssue`, `TestRaiseIssueRefusedBeforeSend` | `pkg/plugin/sdk/report_test.go` | AC-4 | pass |
| `TestPluginSnapshotNotPublished` | `internal/component/plugin/server/doctor_snapshot_test.go` | AC-5 | pass |
| `TestPluginsHealthRowReadsRenamedCode` | `internal/component/plugin/process/register_test.go` | AC-6 | pass |
| `TestPluginCrashReportBus` | `internal/component/plugin/process/process_test.go` | AC-6 | pass |
| `TestEvictionDegradesProbe` | `internal/core/report/report_test.go` | AC-7 | pass |
| `TestReportBusFullRow` | `internal/component/plugin/doctor/check_report_bus_test.go` | AC-7 | pass |
| `TestLastStopRecordWrittenAndCleared` | `internal/component/plugin/server/last_stop_test.go` | AC-8 | pass |
| `TestPluginStoppedEveryDownPath` | `internal/component/plugin/doctor/checks_runtime_test.go` | AC-9 | pass |
| `TestStoppedRecordRaisesAndClears` | `internal/component/plugin/process/stopped_test.go` | AC-9 | pass |
| `TestRecentEventsSkipsWarningAStateCheckOwns` | `internal/component/doctor/checks_recent_events_test.go` | AC-9 (one fault, one row) | pass |
| `TestPluginRestartCompletesConfiguredPlugins` | `internal/component/plugin/server/arg_completion_test.go` | AC-9 (`request plugin restart` completion) | pass |
| `TestDaemonArgCompletionAsksTheDaemon` | `internal/component/cli/client/inject_arg_completion_test.go` | AC-9 (`ze cli` completion) | pass |
| `TestPluginExitClearsItsOwnWarnings`, `TestExitRecordFirstStatusStands` | `internal/component/plugin/process/stopped_test.go` | AC-10 | pass |
| `TestPluginFatalStopRowsNameRecord` | `internal/component/plugin/doctor/check_lifecycle_local_test.go` | AC-11, AC-32 (local half) | pass |
| `TestPluginAutoloadRowCarriesRefusal` | `internal/component/plugin/doctor/check_lifecycle_local_test.go` | AC-12 | pass |
| `TestPluginSetupFailedReportsRecordedOutcome` | `internal/component/plugin/doctor/check_setup_test.go` | AC-45 | pass |
| `TestRunMarksStartupFailure`, `TestRunPostReadyFailureUnmarked` | `pkg/plugin/sdk/startup_error_test.go` | AC-47 | pass |
| `TestDoctorClassChecks` (one subtest per AC-13 to AC-22, plus an empty snapshot), `TestDeliveryChecksAnswerNotPublished` | `internal/component/plugin/doctor/checks_delivery_test.go` | AC-13 to AC-22 | pass |
| `TestDoctorRecordsBoundedAndCopied` | `internal/component/plugin/process/doctor_records_test.go` | AC-13 to AC-22 | pass |
| `TestDoctorSnapshotCarriesRecords` | `internal/component/plugin/server/doctor_delivery_test.go` | AC-5, AC-13 to AC-22 | pass |
| `TestCommandRefusalRecordedAtRegistration` | `internal/component/plugin/server/doctor_delivery_test.go` | AC-13 | pass |
| `TestSubscriptionDropRecorded` | `internal/component/plugin/server/doctor_delivery_test.go` | AC-14 | pass |
| `TestPostStartupFailureRecorded` | `internal/component/plugin/server/doctor_delivery_test.go` | AC-17 | pass |
| `TestEngineHandlerPanicRecorded` | `internal/component/plugin/server/doctor_delivery_test.go` | AC-18 | pass |
| `TestDeliverySlowBoundary`, `TestDeliveryWindowForgetsOldProblems`, `TestDeliverBatchRecordsFireAndForgetFailure` | `internal/component/plugin/process/doctor_records_test.go` | AC-19 | pass |
| `TestRelayStoppedRaisesEvent` | `internal/component/plugin/process/doctor_records_test.go` | AC-20 | pass |
| `TestEnricherFailureRecorded` | `internal/component/plugin/server/doctor_delivery_test.go` | AC-21 | pass |
| `TestConfigUnappliedRecordedOnCommitFailure` | `internal/component/plugin/server/doctor_delivery_test.go` | AC-22 | pass |
| `TestRestartCommandReplacesARunningPlugin` | `internal/component/plugin/server/plugin_restart_running_test.go` | AC-9 (restart of a running plugin, R-8) | pass |
| `TestRestartCommandClearsDoctorRecords` | `internal/component/plugin/server/plugin_restart_running_test.go` | AC-14, AC-17, AC-18, AC-21, AC-22 (the restart clears each record, R-8) | pass |
| `TestRestartCommandOfRunningFatalPluginKeepsDaemonUp` | `internal/component/plugin/server/plugin_restart_running_test.go` | AC-9 (a `fatal` plugin's restart neither stops ze nor records it down, R-8) | pass |
| `TestRestartCommandRefusals` | `internal/component/plugin/server/plugin_restart_running_test.go` | AC-9 (refusals, a config transaction in flight included, R-8) | pass |
| `TestRestartCompletionOffersRunningPlugin` | `internal/component/plugin/server/plugin_restart_running_test.go` | AC-9 (completion offers a running plugin, R-8) | pass |
| `TestRestartCommandRunsFromTheOperatorLine`, `TestRestartCommandLineRefusalsAndCompletion` | `internal/component/plugin/server/plugin_restart_dispatch_test.go` | AC-9 (the command through the dispatcher, ce86ac4a64) | pass |
| `TestRestartCommandRefusesItsCaller`, `TestRestartCommandFailsWhenOldCleanupNeverEnds`, `TestRestartCommandFailsWhenReplacementNeverHandshakes`, `TestRestartCommandRefusesPendingRemoval` | `internal/component/plugin/server/plugin_restart_guard_test.go` | AC-9 (refusals and bounds, 325ec5d0d6) | pass |
| `TestRefusedRemovalKeepsPluginConfigured` | `internal/component/plugin/server/plugin_unconfigure_test.go` | AC-9 | pass |
| `TestPluginStoppedRowWhileCleanupOwed` | `internal/component/plugin/doctor/checks_runtime_test.go` | AC-9 | pass |
| `TestRespawnOfALiveProcessRacesNoMetricCallback`, `TestReaddedPluginRespawnsWithItsNewConfig`, `TestWaitRuntimeCleanupContextAnswersBothOutcomes`, `TestStopJoinBound` | `internal/component/plugin/process/respawn_live_test.go` | AC-9 | pass |
| `TestRespawnRecordsAnExpiredJoin`, `TestRestartOverlapTextNamesWhatWasUnfinished` | `internal/component/plugin/process/restart_overlap_test.go` | AC-48 | pass |
| `TestRestartCutsEnricherCallInFlight`, `TestRestartEnricherReadBeforeRestartBlamesNoReplacement` | `internal/component/plugin/server/enricher_restart_test.go` | AC-21 (R-12) | pass |
| `TestProxyEnricherFailureOfEndedProcessNotRecorded` | `internal/component/plugin/server/doctor_delivery_bounds_test.go` | AC-21 | pass |
| `TestRegisterNeverMovesAListAReaderHolds` | `internal/core/show/register_race_test.go` | AC-21 (029401af1b) | pass |
| `TestBoundedRecordsNameWhatTheyDropped` | `internal/component/plugin/doctor/checks_delivery_bounds_test.go` | AC-14, AC-18, AC-19, AC-21 (R-9) | pass |
| `TestEnricherFailuresPastTheBoundAreCounted`, `TestHandlerPanicsPastTheBoundReachTheSnapshot` | `internal/component/plugin/server/doctor_delivery_bounds_test.go` | AC-21, AC-18 (R-9) | pass |
| `TestDeliveryWindowSaysWhenTheRingOverwrote` | `internal/component/plugin/process/doctor_records_bounds_test.go` | AC-19 (R-9) | pass |
| `TestReadyParamsRefusedRecordsDrop`, `TestSubscriptionWithNoDefaultNamespaceRecordsDrop` | `internal/component/plugin/server/doctor_producer_lines_test.go` | AC-14 (R-10) | pass |
| `TestPostStartupNoConnectionRecordsUndelivered`, `TestPostStartupPendingUntilAnswered` | `internal/component/plugin/server/doctor_producer_lines_test.go` | AC-17 (R-10) | pass |
| `TestSafeDeliverBatchPanicRecordsProjectionError` | `internal/component/plugin/process/doctor_producer_lines_test.go` | AC-18 (R-10) | pass |
| `TestReloadRecordsAndClearsCapabilityRestore` | `internal/component/plugin/server/doctor_producer_lines_test.go` | AC-29 (R-10) | pass |
| `TestStopKeepsTheEnginesThatMissedTheirGrace` | `internal/component/plugin/process/doctor_producer_lines_test.go` | AC-32 (R-10) | pass |
| `TestCleanupRecordsTheEngineThatMissedItsGrace` | `internal/component/plugin/server/doctor_producer_lines_test.go` | AC-32 (R-10) | pass |
| `TestProxyEnricherCallFailureRecordedAndCleared` | `internal/component/plugin/server/doctor_delivery_bounds_test.go` | AC-21 (R-10) | pass |
| `TestConfigCommittedWithNoConnectionRecorded` | `internal/component/plugin/server/doctor_delivery_bounds_test.go` | AC-22 (R-10) | pass |
| `TestConfigUnappliedClearsOnLaterCommitNotOnUnchangedReload`, `TestRestartClearsConfigUnappliedRecord` | `internal/component/plugin/server/doctor_delivery_bounds_test.go` | AC-22 (R-8) | pass |
| `TestConfigUnappliedFixRestartsThePlugin` | `internal/component/plugin/doctor/checks_delivery_bounds_test.go` | AC-22 (R-8) | pass |
| `TestExternalPluginNameRefusedAtCommit` | `internal/component/plugin/ipc/tls_test.go` and `internal/component/config/external_plugin_name_test.go` | AC-23 | pass |
| `TestConnectBackRefusalNamesClaimNotToken` | `internal/component/plugin/ipc/tls_test.go` | AC-23 | pass |
| `TestConnectBackCodeMatchesDiagnostic` | `internal/component/plugin/ipc/connect_back_code_test.go` | AC-23 | pass |
| `TestExternalPluginNameCheck` | `internal/component/plugin/doctor/check_external_test.go` | AC-23 | pass |
| `TestExternalUnsupportedCheckReadsField` | `internal/component/plugin/doctor/check_external_test.go` | AC-24 | pass |
| `TestManagedListenerUnboundRecorded` | `internal/component/plugin/server/managed_doctor_test.go` | AC-25 | pass |
| `TestManagedChecksAnswerNotServing` | `internal/component/plugin/doctor/check_external_test.go` | AC-25, AC-26 | pass |
| `TestManagedClientRejectionRecorded` | `internal/component/plugin/server/managed_doctor_test.go` | AC-26 | pass |
| `TestManagedClientRejectedRow` | `internal/component/plugin/doctor/check_external_test.go` | AC-26 | pass |
| `TestManagedNotifyDroppedRaises` | `internal/component/plugin/server/managed_doctor_test.go` | AC-27 | pass |
| `TestEmptyExpectedTokenNeverAuthenticates` | `internal/component/plugin/ipc/tls_test.go` | AC-26 (empty-token auth fix, 75e0f5565e) | pass |
| `TestManagedUnknownNameEmptyTokenRefused`, `TestManagedUnknownNamesBounded` | `internal/component/plugin/server/managed_auth_refused_test.go` | AC-26 (empty-token auth fix, 75e0f5565e) | pass |
| `TestReloadDoctorRecords` | `internal/component/plugin/server/doctor_reload_records_test.go` | AC-28, AC-29 | pass |
| `TestReloadRecordRows` | `internal/component/plugin/doctor/check_reload_dataplane_test.go` | AC-28, AC-29 | pass |
| `TestEnsureRollbackFailureRaisesEvent` | `internal/component/plugin/server/command_events_test.go` | AC-30 | pass |
| `TestDeprecatedCommandRaisesEventOnce` | `internal/component/plugin/server/command_events_test.go` | AC-31 | pass |
| `TestStopTransactionLateWritesRecord`, `TestCleanupLateEnginesRecorded` | `internal/component/plugin/server/doctor_reload_records_test.go` | AC-32 | pass |
| `TestLogKeepsLastWriteFailure`, `TestPublishedWriteOutcome` | `internal/core/audit/write_outcome_test.go` | AC-33 | pass |
| `TestAuditRecordFailedRow` | `internal/component/doctor/checks_audit_test.go` | AC-33 | pass |
| `TestDataPlaneWriterRowMatchesRefusal` | `internal/component/plugin/doctor/check_reload_dataplane_test.go` | AC-34 | pass |
| `TestDataPlaneWriterCheckFromTree` | `internal/component/plugin/doctor/check_dataplane_writer_test.go` | AC-34 (from a config tree through `checkDataPlaneWriter`: a `NeedsDataPlane` producer over an unprogrammed data plane gives one error row `doctor-plugin-no-dataplane-writer` naming the plane, with or without an empty `fib` block; a `fib` block naming a FIB plugin gives none) | pass |
| `TestEnvInvalidValueRecorded`, `TestEnvInvalidRecordFollowsCurrentValue` | `internal/core/env/env_test.go` | AC-35 | pass |
| `TestEnvValuesCheckReportsUnusableValue`, `TestEnvValueDiagnosticWithholdsSecret` | `internal/component/doctor/checks_env_values_test.go` | AC-35 | pass |
| `TestEnvAboveMaximumRecorded`, `TestEnvBelowMinimumRecorded` | `internal/core/env/env_test.go` | AC-42 (env half) | pass |
| `TestCapClampRecordedForDoctor` | `internal/core/report/report_clamp_test.go` | AC-42 (env half) | pass |
| `TestNotProblemSitesRaiseEvents`, `TestInternalDefectCodeMatchesDiagnostic` | `internal/component/plugin/server/server_events_test.go` | AC-36 to AC-39 | pass |
| `TestTypedDropRaisesInternalDefect` | `internal/core/events/typed_drop_test.go` | AC-42 | pass |
| `TestNotProblemLinesCarryCodes` | `internal/le/arch/doctorcoverage/report_test.go` | AC-36 to AC-42, AC-44 | red on exactly the five AC-40 lines (no code until `doctor-config-reload-refused` is declared, A-5), green otherwise (29f5ce89ed) |
| `TestGateSeesWrapperAndNonLiteralWarn` | `internal/le/arch/doctorcoverage/judge_test.go` | AC-43 | pass |
| `TestDoctorCoverageCheckRefusesPluginFindingWithoutFix` | `internal/le/arch/doctorcoverage/judge_test.go` | AC-46 | pass |
| `TestDoctorCoverageReportListsPluginFindings` | `internal/le/arch/doctorcoverage/report_test.go` | AC-46 | pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| plugin name length | 1..64 | 64 | 0 | 65 |
| delivery latency threshold | 500 ms | 500 ms | N/A | 501 ms reported |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `doctor-plugin-stopped` | `test/plugin/doctor-plugin-stopped.ci` | an `ignore` plugin exits; `show doctor` names it, `plugins` row down | pass, promoted (c3692ce544) |
| `doctor-plugin-report-raise-external` | `test/plugin/doctor-plugin-report-raise-external.ci` | a forked plugin raises; `show doctor` shows it with its fix | pass, promoted (c1a939ebea) |
| `doctor-plugin-external-unsupported` | `test/plugin/doctor-plugin-external-unsupported.ci` | `ze doctor` reports an external in-process-only plugin | pass, promoted (c1a939ebea) |
| `doctor-plugin-fatal-stop` | `test/plugin/doctor-plugin-fatal-stop.ci` | a fatal plugin stops ze; the next `ze doctor` names it | pass, promoted (c3692ce544) |
| `doctor-env-invalid-value` | `test/plugin/doctor-env-invalid-value.ci` | a bad duration env; `show doctor` names key and default | pass, promoted (eb6e1d9a80) |
| `plugin-external-bad-name` | `test/parse/plugin-external-bad-name.ci` | commit refuses `plugin external a_b` | pass, promoted (c1a939ebea) |
| `plugin-restart-external-running` | `test/plugin/plugin-restart-external-running.ci` | an operator restarts a running external plugin with `ze cli`; the answer says it was stopped and started again, a new process serves, no crashed or stopped event follows, an unknown name is refused | pass, promoted (ce86ac4a64): red on the selector refusal before the fix, 80 of 80 stress invocations and one live run after it; rerun green after 325ec5d0d6 |
| `plugin-restart-external-ignores-stop` | `test/plugin/plugin-restart-external-ignores-stop.ci` | an operator restarts a running external plugin that ignores its stop; the restart answers, the old process is gone, the replacement serves, and no crashed, stopped or restart-overlap event follows | pass, promoted (9325dde3da) |
| `doctor-static-no-fib-block` | `test/ui/doctor-static-no-fib-block.ci` (existing, converted by P5) | `ze doctor` on a static route with no `fib` block reports `doctor-plugin-no-dataplane-writer` where it reported `doctor-static-no-fib-writer` (AC-34) | pass |

### Interop Tests (Scope: protocol)
N-A: no protocol peer; the plugin protocol's outside party is a plugin. The external route is proven over real TLS connect-back by a compiled fixture observer the test forks (`internal/test/fixture/doctor_plugin_report_raise.go`, plugin `doctor-report-raise-test`), not by `./le mock plugin-external`: no registered engine raises on the route yet (P0b).

## Files to Modify
- `internal/core/diagnostic/codes.go` - every code (AC-1); the event-code registration (AC-2)
- `internal/core/report/report.go` and its health probes - fix field (AC-3), eviction accounting (AC-7)
- `internal/core/env/env.go` - invalid-read record (AC-35)
- `internal/core/audit` - last write outcome (AC-33)
- `internal/component/doctor/checks_recent_events.go` - declared event codes, raise fix (AC-2, AC-3)
- `pkg/plugin/rpc` (new report types beside `metrics.go`), `pkg/plugin/sdk` (raise and clear methods), `internal/core/ipc/yang/ze-plugin-engine.yang` - the route (AC-4)
- `internal/component/plugin/server/dispatch_registry.go` - the `engineOps` entry (AC-4)
- `internal/component/plugin/process/manager.go`, `process.go`, `delivery.go`, `register.go` - renames, every down path, records (AC-6, AC-9, AC-18 to AC-20)
- `internal/component/plugin/server/failure_policy.go`, `reload_tx.go`, `dispatch.go`, `startup_driver.go`, `poststartup.go`, `enricher.go`, `startup_removal.go`, `config_tx_bridge.go`, `command_registry.go`, `ensure.go`, `command.go`, `managed_serve.go`, `engine_event.go`, `reload.go`, `startup_autoload.go` - records and codes per AC
- `internal/component/plugin/ipc/tls.go`, `internal/component/plugin/yang/ze-plugin-conf.yang` - name refused at commit (AC-23)
- `internal/component/plugin/registry/registry.go` - in-process-only field (AC-24)
- `internal/plugins/static/doctor.go` - `static-fib-writer` removed (AC-34)
- `pkg/zefs/keys.go` - last-stop key (AC-8)
- `internal/le/arch/doctorcoverage/sites.go` - two site shapes (AC-43)
- `internal/le/arch/doctorcoverage/codes.go` and `check_test.go` - the event-code registration as a `checkDeclarations` row and in `TestDoctorCoverageSourceHoldsEveryRegisteredCode` (AC-2, A-1)
- `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/api/process-protocol.md`, `docs/plugin-development/README.md` - pages these change

## Files to Create
- `internal/component/plugin/server/dispatch_report.go` (+ test) - the report entry
- `internal/component/plugin/server/doctor_snapshot.go` (+ test) - the published snapshot (AC-5)
- `internal/component/plugin/doctor/checks_runtime.go` (+ test) - the runtime class checks
- `internal/component/plugin/doctor/check_setup.go` (+ test) - the setup-outcome check over `registry.SetupResults` (AC-45)
- `internal/component/plugin/server/last_stop.go` (+ test) - AC-8
- the `.ci` files in the Functional Tests table

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-plugin-engine.yang` `rpc report-issue`; `ze-system-cmd.yang` `request plugin restart` (A-2 held) |
| YANG validation constraints | Yes | `ze-plugin-conf.yang` external name (AC-23) |
| YANG custom validators | Yes | the name validator reads the one pattern declaration |
| CLI commands/flags | Yes | `request plugin restart <name>` (AC-9; A-2 held), `internal/component/plugin/server/register_plugin_restart.go` |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md` for that command |
| Editor autocomplete | Yes | `request plugin restart <name>` completes the configured plugins that are not running, answered by the daemon: a builtin's `ArgCompleter` (`internal/component/plugin/server/arg_completion.go`), hung on the SSH tree by `WireArgCompletion` and on the `ze cli` tree by `wireDaemonArgCompletion` (`internal/component/cli/client/inject.go`) |
| Functional test for new RPC/API | Yes | `doctor-plugin-report-raise-external.ci` |
| Pipe completeness | N-A | doctor output already goes through the shared renderer |
| Env var registration | N-A | no new env key; `ze.report.*.max` exist |
| Doctor check for runtime dependencies | Yes | the whole spec |
| Prometheus counters/metrics | Yes | `ze_plugin_delivery_failures_total{plugin}` and `ze_plugin_delivery_slow_batches_total{plugin}` (AC-19, `internal/component/plugin/process/manager.go` `pluginMetrics`), documented in `docs/plugin-development/metrics.md` |
| BGP family surface | N-A | no family |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | the `features/` doctor entry, where one exists |
| 2 | Config syntax changed? | Yes | `docs/guide/plugins.md` (external name form) |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`request plugin restart`) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/monitoring.md` doctor section |
| 7 | Wire format changed? | No | no BGP wire |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md`, `docs/plugin-development/README.md`, `ai/rules/plugins.md` |
| 9 | RFC behavior? | No | none |
| 10 | Test infrastructure changed? | No | the gate's site shapes are documented in `doctor-and-health-checks.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/doctor-and-health-checks.md` |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, command changed? | Yes | `docs/plugin-overview.md` (in-process-only field) |
| 16 | Changed source files referenced by doc anchors? | Yes | declared by the changed files' `// Design:` headers: `docs/architecture/config/environment.md` (env invalid reads, AC-35: updated), `docs/architecture/zefs-format.md` (last-stop key, AC-8: updated), `docs/architecture/static-routes.md` (`static-fib-writer` removed, AC-34: updated), `docs/features/ai-first.md` (plugin doctor checks: updated with the new runtime checks), `docs/architecture/core-design.md` (no section changes: the checks register in their owners, no new import edge), `docs/architecture/api/architecture.md` (unaffected: the dispatcher shape is unchanged, one more `engineOps` entry is covered by `process-protocol.md`); rerun `./le spec citation anchors spec plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` at each phase |
| 17 | Existing examples for this area? | Yes | `docs/guide/plugins.md` external examples against the name pattern |

## Implementation Steps

### Implementation Phases

Sized for one agent of about 60 tool calls each. P0a and P0b are cited by other children by these names.

| Phase | Work | ACs | Depends | Parallel |
|-------|------|-----|---------|----------|
| **P0a shared codes** | codes in `codes.go`, the event-code registration and its gate count, the report fix convention and its rendering | AC-1, AC-2, AC-3 | child 1 | ∥ P8 |
| **P0b plugin report route** | SDK raise and clear, `rpc` types, YANG RPC, one `engineOps` entry mirroring `metrics-publish`, external `.ci` | AC-4 | P0a (fix field) | after P0a |
| P1 snapshot and bus | server snapshot, renames and health filter, bus capacity, last-stop record | AC-5 to AC-8 | P0a | - |
| P2 lifecycle | plugin-stopped (every down path), crashed, fatal-stop, autoload-unresolved, the setup-outcome check; restart command if A-2 | AC-9 to AC-12, AC-45 | P1 | - |
| P3 registration and delivery | the per-plugin records and checks | AC-13 to AC-22 | P1 | ∥ P4 |
| P4 external and fleet | name refusal, in-process field and check, managed rows | AC-23 to AC-27 | P0a, P1 | ∥ P3 |
| P5 reload, commands, stop, audit, data plane | the records and checks | AC-28 to AC-34 | P1 | - |
| P6 core env | invalid-read record and check | AC-35 | P0a | ∥ P2 to P5 |
| P7 code attributes | the not-problem groups, coverage report | AC-36 to AC-42, AC-44 | P0a | last |
| P8 gate | the two site shapes | AC-43 | none | any time |

Each phase: failing test, implement, pass, page edit in the same phase, commit.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N at file and symbol |
| Correctness | every down path of `applyFailurePolicy` and `Respawn` raises `doctor-plugin-stopped`; the probe reads the new name |
| Naming | every code from the canonical list; JSON keys kebab-case (`exit-code`, `fix`) |
| Data flow | a forked plugin's raise goes only through `engineOps`; the external-unsupported check reads the field, never a list |
| Rule: no-layering | one raise route; `static-fib-writer` deleted, not doubled |
| Rule: user-facing-errors | every row names the problem, why, and a fix command on its own line |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| codes described | `ze show diagnostic doctor-plugin-stopped` |
| route works | `test/plugin/doctor-plugin-report-raise-external.ci` |
| area covered | `./le arch doctor-coverage report` for the four packages; `./le arch doctor-coverage check` reports nothing in the plugin packages after 909da87de2 |
| structural gates | `./le arch refuse-by-default check` and `./le arch compound-guard check` clean for this spec's files after 909da87de2 (two compound guards split, three zero answers named as guards); `./le arch operator-error check` OK; `./le doc yang-contract help-shape` OK; `./le doc check all` passes for every page this spec touched; the scoped lint findings in this spec's code (an always-constant parameter, one `errors.As`) fixed in 20c8cf2b5e |
| scoped tests | `./le job run label plugin command go test -race ./internal/component/plugin/... ./pkg/plugin/...`, plus `internal/core/report`, `internal/core/env`, `internal/component/doctor`, `internal/le/arch/doctorcoverage` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the report route bounds every field (the bus limits) and refuses before the handshake |
| Resource exhaustion | a per-plugin raise rate; the bus cap reported, never grown |
| Identity | source is the authenticated plugin name, never a field the plugin sends |
| Error leakage | env secret values never echoed (AC-35); connect-back rows carry no token |

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

- **The event-code registration is the declaration of a transient code.** An owner calls `diagnostic.RegisterEventCode` from its `init()`; the `recent-events` check sets `DeclaresEventCodes` and so declares every registered code, and the doctor-coverage gate reads the same declaration from `diagnostic.EventCode` literals in the source, because it never reads a linked registry (A-1). A code with no metadata is refused at registration. A forked plugin can raise only a registered event code, so it can never raise a code another check owns, and doctor can always describe what it raised. Registering a code does not make its log lines true: each site that carries an event code must also raise it on the bus (R-7).
- **One fault gives one row.** A state check that reports a plugin from the snapshot (`doctor-plugin-stopped`) and `recent-events` rendering the same active warning would double the row, so `recent-events` skips an active warning whose code a state check declares (`TestRecentEventsSkipsWarningAStateCheckOwns`). Transient events with no state check behind them stay on `recent-events`.
- **P1 and P2 landed in one commit (aa6ebb8d15).** P1's lines carry `doctor-plugin-stopped` and `doctor-plugin-fatal-stop`, which only P2's checks declare; committed alone, P1 would have added log lines the gate counts uncovered and codes no check reports.
- **The managed fleet server let an unknown name in with an empty token.** It passes an empty shared secret to `AuthenticateWithLookup`, meaning per-client secrets only, but the check started from that empty secret and replaced it only on a lookup hit, so an unknown name sending an empty token compared equal. Found in P4 inside the function that phase changed, fixed in 75e0f5565e: an empty expected token never authenticates, for every caller. The test-only `Authenticate` and `authenticateWithName` share the shape and have no non-test caller.
- **The plugin cleanup hook list raced, a defect older than this spec.** It was an unguarded slice on an "init only" contract that the proxy enricher code broke by registering lazily; fixed in e442cb88c0: the list is locked and the hook registers at init (`TestProcessCleanupHooksRegisterWhileRunning`, `internal/component/plugin/server/rpc_register_cleanup_test.go`).
- **A foreground `ze` step in a `.ci` is started, not awaited.** A test whose next step must run after ze exits runs the daemon through a waited helper script (`doctor-plugin-fatal-stop`, c3692ce544).
- **`ready=` holds one state.** A step that needs two states (ze running, then the stop on the bus answered over SSH) waits on a state-polling fixture, `le test fixture daemon/await-answer` (`doctor-plugin-stopped`, c3692ce544).
- **A handler test proves nothing about reachability.** `request plugin restart <name>`, the fix line every doctor row prints, was refused by the daemon with "requires a selector" before its handler ran, from aa6ebb8d15 until ce86ac4a64: it was registered with `RequiresSelector`, a guard the dispatcher satisfies only from the leaf named `selector` or a peer supplied out of band (`command.go` `Dispatch`), while its YANG leaf is `name`, and every test called the handler directly. The registration no longer claims a selector; the mandatory `name` leaf is the guard, `request plugin restart name <name>` also works, and a bare line answers `required argument missing: name`. A command owes a test through the dispatcher or the CLI (`plugin_restart_dispatch_test.go`, `plugin-restart-external-running.ci`).
- **`TestDrainCallerLifecycle` failed once under full-package race load, from a sampling race in the test.** It read its flags when the process slot changed, before the in-respawn check had run; the product joins the old drain caller before it respawns. Fixed in 115b64f109 with no assertion changed. The test belongs to `plan/release/fixes/spec-daemon-drain-before-stop.md`.
- **Letting an operator command reach a path written for an internal caller needs that path's waits audited for bounds under whatever lock the command holds.** The restart path had three unbounded waits (the old process's cleanup, the replacement's handshake, the rollback's join) that were harmless while only a dead process reached them, and each could hang the command with the config transaction lock held (325ec5d0d6).
- **Independent review found what four rounds of authoring agents and green tests did not, twice:** first the unbounded cleanup wait and the self-restart hang, then the unbounded handshake left by the fix.
- **A text buffer's `String()` read twice bit this spec a second time:** the restart-overlap event's message and fix were empty on the bus while the log line was right (0becf0044f), caught by a test asserting on the raised event, not the log line.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/field-carries-two-meanings.md` | 2026-11-10 | internal runner exit status logged under `code` (AC-6) |
| `plan/journal/failed-start-reads-as-absent.md` | 2026-11-10 | `plugin-down` raised only at the respawn limit (AC-9) |
| `plan/journal/validate-accepts-what-start-refuses.md` | 2026-11-10 | external plugin name accepted, refused at connect-back (AC-23) |
| `plan/journal/zero-value-as-valid-answer.md` | 2026-11-10 | env `GetInt`/`GetInt64`/`GetDuration` silent default (AC-35) |
| `plan/journal/probe-inability-read-as-absence.md` | 2026-11-10 | report-bus eviction turns a health row healthy (AC-7) |
| `plan/journal/check-cannot-see-the-change-it-looks-for.md` | 2026-11-10 | doctor-coverage gate misses wrapper and non-literal Warn (AC-43) |
| `plan/journal/gate-excludes-part-of-its-population.md` | 2026-11-11 | doctor-coverage gate never judges the Fix of an `rpc.DoctorCheckDiagnostic` (AC-46) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Transient codes declared through an event-code registration read by `doctor-recent-events` | a stub check per owner declaring codes it never emits | a check declaring a code it never produces is a false declaration; the bridge does produce the row |
| One report route in this spec | each child its own | `ai/rules/no-layering.md`; children 7 and 14 depend on P0b |
| The fix travels on the raise | a code-to-fix table in doctor | the producer knows the fix for the subject; a table is a second declaration |
| Server publishes a value-type snapshot | pass the server into the check context | the check context is shared core; a snapshot keeps value types across the boundary (`ai/doors/core.md`) |
| A persisted last-stop record | log only | nothing survives the stop otherwise, so the next `ze doctor` could not name the cause |
| One generic data-plane writer check | add an epe check beside static's | the registry declares `NeedsDataPlane`; a per-plugin check is a central enumeration in disguise |

## Known Limitations
- The startup-failure classes are implemented by `plan/release/fixes/spec-doctor-reports-failed-plugin-startup.md`, not here.
- Scan roots outside the gate (`internal/appliance`, `internal/install`, `cmd/ze-installer`) and judging `internal/core/report` are child 1's and the owner's decision (inventory, "Shared mechanisms", item 5).
- Nine defects around restarting a plugin that two independent reviews found and this spec does not fix (the configured set never shrinks, the manager's lock held across the replacement's start, and seven more) are owned by `plan/release/fixes/spec-plugin-restart-bounded-and-configuration-true.md` (105d71fa11).
- A command registered as requiring a selector whose leaf is not named `selector` refuses every line, and nothing at registration refuses that declaration: owned by `plan/release/fixes/spec-requires-selector-needs-a-selector-leaf.md` (ad304d17e4).

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
- [ ] AC-1..AC-48 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le job run label plugin command go test -race ./internal/component/plugin/... ./pkg/plugin/...`, the scoped `internal/core/report`, `internal/core/env`, `internal/core/diagnostic`, `internal/component/doctor` and `internal/le/arch/doctorcoverage` packages, and the named `.ci` files through `./le test functional plugin` and `./le test functional parse` filtered to them. No agent runs any `./le verify` command
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
- [ ] Interop tests for protocol features (N-A, reason above)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` only, in the same `./le commit create` script
