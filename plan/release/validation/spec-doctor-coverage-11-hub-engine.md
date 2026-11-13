# Spec: doctor-coverage-11-hub-engine

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | cli |
| Priority | P2 |
| Release reason | `ai/META-DECISIONS.md`, "Goal and scope": `ze doctor` reports every problem an operator can hit, each with its cause and the command that fixes it; O-3 of `spec-doctor-coverage-1-audit-and-gate` makes every area child release work |
| RFC gaps | - |
| Depends | `spec-doctor-coverage-1-audit-and-gate` (the `runtime` phase, `Diagnostic.Fix`, the `recent-events` check and the gate, landed in ce58dddf53 and fc96556110); `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` phase P0a (the cross-cutting codes `doctor-internal-defect` and `doctor-audit-record-failed`, the event-code registration and the rule that a report-bus raise carries its own fix): this spec uses it, builds none of its own, and it comes before this spec's P2 and P3. Its P0b (the report route for a forked plugin) is not needed: every producer here runs in the daemon process (or, for `cmd/ze` start and push, writes a zefs key) and raises through `internal/core/report` directly; child 12 no longer depends on this spec: it registered `doctor-config-reload-refused` itself (3ce015b3ff), and this spec builds on that code (AC-13, AC-34). `plan/release/validation/spec-doctor-coverage-7-bgp-plugins.md` AC-76 does not: its sites carry `config-plugin-verify`, which the `config-semantics` check (`internal/component/config/doctor.go`) already declares; `plan/release/validation/spec-doctor-coverage-4-api-surfaces.md`, which registers `doctor-mgmt-listener-not-serving` and `doctor-mgmt-user-list-unreadable`, and whose AC-10 check `gnmi-tls-material` declares `doctor-gnmi-tls-material` (landed in 537b3a87f9); `plan/release/validation/spec-doctor-coverage-9-config-yang.md`, which registers `doctor-config-confirm-pending` and `doctor-config-confirm-revert-failed`, and whose AC-4 check `config-passwords` owns `doctor-config-password-plaintext` (AC-20 here only codes the hub's log line); `plan/release/validation/spec-doctor-coverage-8-commands-cli.md`, which registers `doctor-cli-editor-unavailable` |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent: `plan/release/validation/spec-doctor-coverage-0-umbrella.md`. Source: the hub-engine section of the audit (`plan/audits/doctor-problem-inventory/hub-engine.md`, slice A4, scope `cmd/ze`, `cmd/ze/hub`, `internal/component/managed`, `internal/component/engine`, `internal/component/hub`).

## Task

Thomas: "Not only the one you have, but all problems. So we will have to audit
the code for all the things we need to be added to it."

The audit placed the daemon start, reload, service, AAA wiring and managed
(fleet) problem sites in 24 classes: the 22 of the merged inventory (21 gap, 1
covered) plus the two the main thread assigned here (the boot decision over a
management listener that does not start, and the confirmed-commit window left
open). Eight groups of lines are not problem classes. This spec closes every
gap:

| # | Work |
|---|------|
| 1 | `ze doctor` (local) and `show doctor` (every daemon surface) report every hub-engine problem class with its cause in plain words and the exact command that fixes it on its own line |
| 2 | Every Warn and Error line, and every report-bus raise, in `cmd/ze`, `cmd/ze/hub` and `internal/component/managed` names a doctor code that a registered check declares (O-2), so `./le arch doctor-coverage report` counts the area covered |
| 3 | The doctor `listeners` check stops reading a healthy daemon's own listeners as "cannot bind" (AC-9 of the parent) |
| 4 | Four defects the audit walked into are fixed: the listeners probe inside the daemon, REST and gRPC stopping the daemon where every other listener does not, gNMI falling back to clear text (landed in 537b3a87f9 and ad9fc4abe2, AC-7), extra plugin-hub servers never bound |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - outside parties, plugin process boundary, registration, shared machinery
  → Constraint: a check registers from `init()` in the package that owns the state it reads; no edit to a central list
  → Constraint: runtime state persists through the store under a key declared in `pkg/zefs/keys.go`, never `os.WriteFile`
  → Constraint: every operator command runs through the daemon on every surface; `show doctor` is that surface for runtime state
- [ ] `ai/doors/hub-engine.md` - boot, reload, managed, flavors
  → Constraint: one reload path, `doReload`; the reload outcome record is written there and nowhere else
  → Constraint: no always-on hub file imports a gated package; a gated service reports through the report bus
  → Constraint: something built only at startup ignores every reload: the service-status raise is cleared by the reload that brings a service up
- [ ] `ai/doors/core.md` - `internal/core` is the leaf tier, `pkg/zefs` is public
  → Constraint: the route field on `DoctorCheckContext` names no component; a new zefs key is a published-contract change made on purpose, with `docs/architecture/zefs-format.md`
- [ ] `ai/doors/api-surfaces.md`, `ai/doors/config-yang.md`, `ai/doors/le-tooling.md` - the readers of the codes and the gate
  → Constraint: the management listener classes after start are the api-surfaces child's; this spec owns the boot decision only
  → Constraint: the gate reads a site's code from its `code` attribute; nothing here changes the gate
- [ ] `docs/architecture/doctor-and-health-checks.md` - phases, owners, the `runtime` phase, the fix line
  → Decision: a `runtime` check receives `Platform` only and reads its owner's live state, never `Tree` (parent spec, Data Flow 1b)
  → Constraint: a transient code counts as covered only when its owner's check declares it; `recent-events` then reports it (parent spec, Data Flow 2a)
- [ ] `spec-doctor-coverage-1-audit-and-gate` - Task, O-1 to O-3, AC-6, AC-9, AC-14, Data Flow amendments
  → Decision: O-1, the local route asks the daemon over SSH and falls back with `doctor-runtime-not-checked`; O-2, no transient marker; O-3, release work
  → Constraint: AC-9, a check that reads daemon state from a non-runtime phase moves to `runtime` or splits, so no route reads empty or self-held state as healthy
- [ ] `plan/release/validation/spec-doctor-coverage-2-home-cpe-batch.md` - sibling child
  → Constraint: it owns no hub-engine row (its `cmd/ze/hub/main_system.go` resolver edit is its own); nothing here respecs it
- [ ] `plan/release/validation/spec-doctor-coverage-6-bgp-engine.md` - sibling child, the format this spec follows
  → Constraint: the BGP listener half of the bind probe (`doctor-bgp-listen`) is read there; AC-1 here changes the probe's reading, not its listener set
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: problem first, why, then the fixing command on a line of its own
- [ ] `ai/rules/spec-no-code.md` - no code in a spec
- [ ] `ai/rules/plugins.md`, `docs/architecture/fleet-config.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/system-architecture.md`, `docs/architecture/web-interface.md`, `docs/features/ai-first.md` - the pages the changed files declare in their `// Design:` headers
  → Constraint: each page that describes a behaviour this spec changes (managed session state, listener failure at boot, the doctor route) is edited in the same phase as the code

**Key insights:**
- `doctor.Answer` (the daemon route, reached from `HandleDoctor`) runs `runChecks`, every local phase included, inside the daemon process; `answerLocalWith` (the local route) runs the same checks only when the daemon did not answer. No check can tell which route it runs on
- the reload path keeps no outcome: a SIGHUP refusal is printed on stderr only, and four rows (refused, unwind failed, partial apply, audit lost) read one producer-side record
- `HealthRevert`, the pushed-config handling and the web, MCP and gNMI boot failures report on stderr or a plain `slog` line only; on gokrazy stderr never reaches the console (`logStartupFailure`, comment)
- the managed client keeps no connection or last-ack state

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/doctor/checks_listener.go` - `checkListeners` bind-probes every endpoint `collectAllListeners` derives (schema services plus the BGP, BFD, IPsec, TFTP, image and NTP extractors), reporting a failed bind under the listener's code (`doctor-listen-unavailable` for schema services); `bindProbe` sets no `SO_REUSEPORT`
- [ ] `internal/component/doctor/answer.go` - `Answer` (daemon route) runs `runChecks`, then the runtime checks
- [ ] `internal/component/doctor/cmd/show.go` - `HandleDoctor` calls `doctor.Answer` with `RunRuntimeChecks` and the plugins' checks
- [ ] `internal/component/doctor/doctor.go` - `runChecks` builds the check context per phase (pre-config, missing-config, parsed-config, post-config)
- [ ] `internal/component/doctor/local_route.go` - `answerLocalWith`, `askDaemonOverSSH` (`probeDaemonListener` tells "no daemon listens" from "a daemon listens but this account cannot log in"), `runtimeNotChecked`
- [ ] `internal/core/diagnostic/doctor_registry.go` - `DoctorCheckContext` holds `Tree`, `ConfigDir`, `Plugins`, `Store`, `Platform`; no route
- [ ] `internal/core/diagnostic/codes.go` - one exported spelling per code another package emits; `CodeDoctorRuntimeNotChecked`, `CodeDoctorRecentEvents`
- [ ] `cmd/ze/hub/main.go` - `runYANGConfig`: a REST or gRPC build error prints `error:`, logs `startup failed` (stage `start rest api` / `start grpc api`) and returns 1; `buildServices` then builds the gated services; `logStartupFailure`; `bootPowerUsers` Warns when `usersFromStore` fails; the plugin hub Warns "only first server listener is used"; the no-BGP AAA build logs Error and keeps the composed chain; the web `insecure` leaf not honoured is a Warn with a `remedy` attribute; a `recoverConfirmWindow` failure is fatal at start
- [ ] `cmd/ze/hub/service_registry.go` - `buildServices` logs `service build failed` at Warn and skips the service
- [ ] `cmd/ze/hub/service_ssh.go` - a `NewServer` or `Start` failure is a Warn and the daemon runs without SSH
- [ ] `cmd/ze/hub/service_web.go` - a `WaitReady` failure prints `warning: web server failed to start` on stderr only and returns no service; a later `ListenAndServe` error is an Error line
- [ ] `cmd/ze/hub/service_gnmi.go` - `gnmiBuildImpl`: more than one `gnmi server` Warns (child 4 AC-9 removes it); the TLS paths come from `zeconfig.GNMITLSPaths` and go to the server unread; when `Serve` returns before it binds, `serveGNMI` logs the refusal at Error under `doctor-gnmi-tls-material` (an operator error) or "gNMI server error", stderr prints `error:` and no service is returned (537b3a87f9, ad9fc4abe2); a bind that does not appear in five seconds prints a stderr warning and the service is kept
- [ ] `internal/component/gnmi/server.go` - `Serve` loads the TLS pair (`loadTLSCredentials`, `tls.go`) before it listens whenever either path is named, and returns an operator error for a half pair, an unreadable file or an invalid pair; it serves clear text only when no TLS path is named
- [ ] `cmd/ze/hub/service_lg.go` - a `WaitReady` failure returns an error, which `buildServices` logs at Warn
- [ ] `cmd/ze/hub/service_mcp.go` - a TLS load or bind failure prints `warning: MCP server disabled` on stderr only
- [ ] `cmd/ze/hub/main_reload.go` - `sighupReload` prints a refusal on stderr only; `runReloadContext` returns every refusal through `withUnwind`; a failed listener restore is an Error line; the system side effects (`applyHostTuningFromMap` through `reloadSmartManager`) return nothing; `recordDaemonReloadAudit` Warns on an audit failure
- [ ] `cmd/ze/hub/reload_unwind.go` - `unwoundError` carries the refusal and the failed `unwindStep`s (candidate cleanup, listener rollback, pki restore, rollback)
- [ ] `cmd/ze/hub/config_source.go` - `recordBootConfig` logs Error with an ad hoc `fix` attribute; the rebuilt-active Warn
- [ ] `cmd/ze/hub/listener_migrate.go` - `updateLGCertificate` Warns with a `remedy` attribute when the LG listener has no TLS handle; `attachSSH` Warns when the SSH handle cannot move; the sequenced-migration notice; the rollback Warn
- [ ] `cmd/ze/hub/infra_setup.go` - live user source unavailable (Warn), `CheckBcryptLeaves` findings (Warn per leaf), AAA build failure (Error)
- [ ] `cmd/ze/hub/main_servers.go` - `liveLocalUsers` Warns and refuses local authentication with no source or an unreadable one
- [ ] `cmd/ze/hub/session_editor.go`, `cmd/ze/hub/session_factory.go` - an editor or model build failure Warns and the session runs without config mode
- [ ] `cmd/ze/hub/managed_server.go` - a CA, build or start failure logs Error and the managed config server stays off
- [ ] `cmd/ze/hub/confirm_window.go` - `daemonConfirmWindow`, `startConfirmWindow`, `recoverConfirmWindow`; `confirm.Window.Status` (`internal/component/config/confirm/confirm.go`) answers user, deadline, last revert failure and retry time; the store record is `KeyConfigConfirmPending`
- [ ] `cmd/ze/hub/main_system.go` - `startArchiveScheduler` logs Error with an ad hoc `check` attribute holding `ze config validate`
- [ ] `cmd/ze/hub/drain.go` (`stopSignals.onSignal`, `runStopDrain`), `cmd/ze/hub/signal_announce.go` (`announceSignal`), `cmd/ze/hub/aaa_lifecycle.go` (`closeRetiredAAABundle`, `releaseAccountingBundle`), `cmd/ze/hub/register_boot_record_hold_zetest.go` (`holdBootRecord`) - the not-problem lines
- [ ] `internal/component/managed/client.go` - `RunManagedClient` Warns "connection lost, reconnecting"; `fetchAndProcess` Warns on a rejected config and a failed ack; `handleConfigChangedRequest` Warns on a bad payload and a failed fetch; nothing is kept
- [ ] `internal/component/managed/tls.go` - `clientTLSConfig` Warns when verification is disabled
- [ ] `internal/component/managed/doctor.go` - `hub-unreachable` (post-config) TCP-probes the first client's hub under `doctor-hub-unreachable`
- [ ] `cmd/ze/ze_core_start.go` - `extractManagedClientConfig` Warns on a read, parse or extract failure and starts no client; gokrazy auto-init failure is an Error line
- [ ] `cmd/ze/pushed_config.go` - `checkPushedConfig` and `writeConfigActiveHash` report on stderr only
- [ ] `cmd/ze/health_revert.go` - `HealthRevert` reverts on a peer close inside `healthCheckWindow`, logging plain `slog` lines; failures to save, read or write are Warn or Error
- [ ] `pkg/zefs/keys.go` - `KeyConfigPreviousActive`, `KeyConfigLastGoodPushed`, `KeyConfigActiveHash`, `KeyFileActive`, `KeyFileTemplate`, `KeyConfigConfirmPending`

**Behavior to preserve:**
- every existing check, its name, phase and code (`listeners` with `doctor-listen-unavailable` and the per-protocol listen codes, `hub-unreachable`, `doctor-archive-invalid`)
- the bind probe on the local route with no daemon, its freebind handling and its listener set
- the boot-time management-listener guard that refuses an unauthenticated non-loopback listener (a security refusal, not a bind failure)
- every reload refusal and its unwind order; the fail-closed API listeners after a failed listener rollback
- the AAA chain that keeps the backends that built
- the confirmed-commit revert at the deadline and at the next start

**Behavior to change:**
- the bind probe reads its result according to its route (AC-1)
- REST and gRPC listener failures stop no daemon (AC-6); every plugin-hub server is bound (AC-11); gNMI no longer falls back to clear text (AC-7, landed)
- new runtime and local checks for every gap class; every Warn and Error line names its doctor code; lines that report nothing wrong drop to Info
- the ad hoc `remedy` and `check` attributes become the `fix` attribute

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Listener bind probe | `internal/component/doctor/checks_listener.go` `checkListeners`, `collectAllListeners`, `bindProbe` | Post-config bind of every configured endpoint | extends: the probe reads its route from the check context and inverts its verdict inside the daemon (AC-1); its listener set is unchanged |
| Doctor check registry and phases | `internal/core/diagnostic/doctor_registry.go` `RegisterDoctorCheck`, `DoctorCheck`, `DoctorCheckContext` | Registered checks with codes, by phase | extends: `DoctorCheckContext` gains the route the answer runs on; the hub, `cmd/ze` and `managed` register their checks from their own `init()` |
| Daemon and local routes | `internal/component/doctor/answer.go` `Answer`, `local_route.go` `answerLocalWith`, `askDaemonOverSSH`, `doctor.go` `runChecks` | Daemon answers `show doctor`; local asks it first (O-1) | extends: each route sets the context route; the local route also records whether a daemon listens but did not answer |
| Diagnostic code registry | `internal/core/diagnostic/codes.go` constants and `diagnostic.Register` of `CodeMeta` | One spelling per code | extends: every code this spec owns gets one spelling and its metadata, registered beside its check |
| Report bus | `internal/core/report/report.go` `RaiseWarning`, `RaiseError`, `Issue` | Active warnings and a ring of error events | extends: the hub raises a management service that did not start (active warning, cleared when a reload starts it), and the transient hub and managed classes raise events; each raise carries its fix by the route `spec-doctor-coverage-12-plugin-infra` builds |
| Recent-events check | `internal/component/doctor/checks_recent_events.go` `recentEventsCheck` | One runtime row per bus key | does not apply as a change: it renders the raises above once their owner's check declares the codes |
| Reload path | `cmd/ze/hub/main_reload.go` `doReloadContext`, `runReloadContext`; `reload_unwind.go` `withUnwind`, `unwoundError` | One reload path for SIGHUP, commit and managed push; refusal plus failed unwind steps | extends: `runReloadContext` writes one last-reload outcome record from the values it already holds (AC-2) |
| Boot record | `cmd/ze/hub/config_source.go` `recordBootConfig`, `storage.BootRecord` | Holds the boot record error; later config writes are refused with it | extends: the runtime check reads the held error (AC-16) |
| Startup failure mirror | `cmd/ze/hub/main.go` `logStartupFailure` | Error line on the slog backend before exit | extends: it also writes the last-start-failure record (AC-4) |
| Listener migrator | `cmd/ze/hub/listener_migrate.go` `listenerMigrator` (`attachSSH`, `updateLGCertificate`) | Moves listeners on reload; notes the changes that need a restart | extends: the restart-needed notes become a held record the runtime check reads (AC-9) |
| Confirmed-commit window | `cmd/ze/hub/confirm_window.go` `daemonConfirmWindow`; `confirm.Window.Status`; `confirm.StoreRecorder` over `KeyConfigConfirmPending` | Live window and its store record | extends: runtime check over `Status`, local check over the store record (AC-23) |
| AAA bundle | `cmd/ze/hub/aaa_lifecycle.go` `swapAAABundle`; `buildAAABundle` | The live authenticator chain | extends: the bundle keeps which configured backend did not build, read by the runtime check (AC-18) |
| Password format walk | `zeconfig.CheckBcryptLeaves` | Read by the hub's Warn and by `ze config validate` | reuses: the one doctor check over this walk is `config-passwords` in `plan/release/validation/spec-doctor-coverage-9-config-yang.md` AC-4, because the config package owns `CheckBcryptLeaves`; this spec builds no check over it and only codes the hub's Warn line (AC-20) |
| Managed hub probe | `internal/component/managed/doctor.go` `hubDoctorCheck` | TCP reach of the first hub | extends: kept; a runtime check over the client's published session state joins it (AC-25, AC-26) |
| zefs keys | `pkg/zefs/keys.go` `KeyConfigPreviousActive`, `KeyConfigLastGoodPushed`, `KeyConfigActiveHash`, `KeyFileActive` | Push and revert state | extends: two keys for the last start failure and the last push outcome; the existing keys are read as they are |
| Doctor-coverage gate | `internal/le/arch/doctorcoverage`, `./le arch doctor-coverage report` and `check` | Lists uncovered sites | does not apply as a change: it is the measure |
| `opererr` attributes | `internal/core/opererr` `LogAttrs` | `why`, `fix`, `code` | extends: the hub's `remedy` and `check` attributes become `fix` |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze doctor` (local route: the daemon's answer when it answers, else the local checks, O-1)
- `show doctor` on every daemon surface (SSH, MCP, REST, gRPC, web), which runs the local phases and the `runtime` phase in the daemon process

### Transformation Path
1. The route sets the check context route: `daemon` in `Answer`, `local` in `answerLocalWith`, with the sub-state `daemon-unanswered` when `askDaemonOverSSH` found a daemon listening that did not answer.
2. `listeners`: on `local` a failed bind is the finding (today's verdict); on `daemon-unanswered` an address-in-use result is an Info row saying the running daemon probably holds it and the check could not tell; on `daemon` a bind that succeeds is the finding (the configured listener is not served), and address-in-use is healthy.
3. The hub keeps three records: the last reload outcome (written in `runReloadContext`), the services the boot did not start (report-bus active warnings, cleared by the reload that starts them), and the restart-needed notes (listener migrator, web start).
4. `cmd/ze` writes the last start failure (`logStartupFailure`, cleared when the ready file is written) and the last push outcome (`checkPushedConfig`, `writeConfigActiveHash`, `HealthRevert`) under zefs keys.
5. `managed` publishes its session state (connected, last error, last attempt, last ack and its version) as a value snapshot.
6. The checks: `hub-runtime` (runtime, `cmd/ze/hub`) reads the hub records, the AAA bundle, the confirm window and the boot record; `hub-local` (post-config, `cmd/ze/hub`) reads the config tree, the env and the store; `start-local` (post-config, `cmd/ze`) reads the zefs records; `managed-runtime` (runtime, `internal/component/managed`) reads the client snapshot; `managed-local` (post-config) reads the env and the stored config.
7. Each finding is a `Diagnostic` with code, severity, a message naming the service, file, user or hub and the cause, and `Fix` holding the exact command alone.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Hub to doctor | `cmd/ze/hub` registers its checks from `init()`; they read package state the running hub publishes | No: A-1 |
| Doctor route to checks | one field on `DoctorCheckContext`, set by the two route functions | No: A-2 |
| Managed client to doctor | `internal/component/managed` registers its runtime check; it reads a snapshot the client goroutine publishes | No |
| Gated service to hub | a gated `service_*.go` raises on the report bus; the always-on hub names no gated package | Yes: the bus is `internal/core/report` |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `cmd/`, `pkg/`, `test/`, `docs/`; terms the log attribute keys `"remedy"` and `"check"` under `cmd/ze/hub`, the `DoctorCheckContext` type, the zefs key registry in `pkg/zefs/keys.go`, and the stderr warnings `web server failed to start`, `MCP server disabled`, `gNMI server failed to bind`, `start rest api`. Positive controls: `updateLGCertificate` writes `remedy`; `startArchiveScheduler` writes `check`; `codeKey` (`internal/le/arch/doctorcoverage/sites.go`) reads `code`; `readinessPayload` (`internal/component/doctor/answer.go`) writes the doctor answer. No `.ci` file or Go test matched the four stderr strings (search of `test/` and `cmd/ze/hub/*_test.go` returned nothing).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `remedy` / `check` log attribute | writer | `cmd/ze/hub/listener_migrate.go` `updateLGCertificate`, `cmd/ze/hub/main.go` web insecure Warn, `cmd/ze/hub/main_system.go` `startArchiveScheduler` | three sites | change: `fix`, the one key `opererr` uses |
| same | reader | an operator's log tooling outside the repository | pre-release (`ai/META-DECISIONS.md`, "Ze is pre-release") | change, recorded in the commit body |
| zefs last-start-failure and last-push-outcome keys | writer | `cmd/ze/hub/main.go` `logStartupFailure`; `cmd/ze/pushed_config.go` `checkPushedConfig`; `cmd/ze/health_revert.go` `HealthRevert` | new keys in `pkg/zefs/keys.go` | new, documented in `docs/architecture/zefs-format.md` |
| same | reader | the `start-local` check (`cmd/ze`) | new | new |
| same | reader | a fleet tool reading zefs outside the repository | `pkg/zefs` is public API | new keys only; no existing key changes |
| `DoctorCheckContext` route | writer | `internal/component/doctor/answer.go` `Answer`, `local_route.go` `answerLocalWith`, `doctor.go` `runChecks` | the two routes build the context | change: one field added |
| same | reader | every registered check, `checkListeners` first | a check that ignores the field keeps today's behaviour | preserve for every check but `listeners` |
| stderr boot warnings | writer | `cmd/ze/hub/service_web.go`, `service_mcp.go`, `service_gnmi.go`, `cmd/ze/pushed_config.go` | stderr only | change: a coded slog line and a report-bus raise beside the stderr line |
| same | reader | an interactive operator's terminal and the gokrazy supervisor log | no `.ci` or Go test reads these strings (search above) | preserve the stderr line for an interactive operator |
| `show doctor` JSON | writer | `internal/component/doctor/answer.go` `readinessPayload` (`diagnostic.DoctorResult`) | the daemon answer | preserve the shape; new rows only |
| same | reader | `internal/component/doctor/local_route.go` `decodeDaemonDoctor`; MCP, REST, gRPC, web; an API client outside the repository | the local route decodes the same shape | preserve |

### Integration Points
- `internal/component/doctor/doctor.go` `runChecks` - takes the route and passes it in the context
- `cmd/ze/hub` package `init` - registers `hub-runtime` and `hub-local` and their code metadata
- `cmd/ze` package `init` (`ze_core` file) - registers `start-local`
- `internal/component/managed/register.go` - registers `managed-runtime` and `managed-local`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | local checks run in `runChecks`, runtime checks in `RunRuntimeChecks`, transient rows through `recent-events` |
| No unintended coupling (components stay isolated) | Yes | the doctor component names no hub or managed symbol; each owner registers into it |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing Mechanisms: the bind probe, `unwoundError`, `confirm.Window.Status`, `CheckBcryptLeaves`, the boot record and the report bus are reused |
| Zero-copy preserved where applicable (refs, not copies) | Yes | no hot path: every record is written once per reload, boot, push or reconnect |
| Registration over hardcoding, outbound | Yes | each check registers from its owner's `init()` |
| Registration over hardcoding, inbound | Yes | searched: `internal/component/doctor` (no hub or managed name added), `internal/core/report` (no code list), the gate (reads the registry); the route field names no feature |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `cmd/ze/hub`'s `init()` runs in the daemon process that answers `show doctor` | the daemon is `ze start` in the same binary; `HandleDoctor` runs `RunRuntimeChecks` there | the hub checks would never run | `TestHubRuntimeCheckRegistered` | unvalidated |
| A-2 | Every endpoint the probe derives is one the running daemon or its plugins serve | `collectAllListeners` derives the set from the config the daemon runs | a listener the daemon never serves reads "not served" on the daemon route, which is the true finding | `TestListenersDaemonRouteHealthyWhenHeld` | unvalidated |
| A-3 | A REST or gRPC listener failure is not a security refusal | the unauthenticated non-loopback refusal runs earlier in the management-listener guard (`checkMgmtListeners`, `cmd/ze/hub/mgmt_guard.go`) and stays fatal | AC-6 would keep running where it must refuse | read the guard in `cmd/ze/hub/mgmt_guard.go` at P3 | unvalidated |
| A-4 | `spec-doctor-coverage-12-plugin-infra` phase P0a lands the raise-with-fix convention (the `Issue` fix field and its `recent-events` rendering, its AC-3) before P3; its P0b (the forked-plugin report route) is not needed, because every hub producer runs in the daemon and raises through `internal/core/report` directly | cross-child decision of the main thread; child 12's Depends row | the hub raises render with `show warnings` as the fix until it lands | the Depends row; P3 starts after P0a | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The daemon-route inversion reads a UDP endpoint another process holds as served | a UDP listener reported healthy while ze does not serve it | the hub and plugin raises report ze's own bind failure independently; the row is worded as the bind result |
| R-2 | Two children register the same code spelling | duplicate constant at compile | one spelling in `internal/core/diagnostic/codes.go`; whichever child lands first adds it and the other reuses it |
| R-3 | Keeping the daemon running on a REST failure hides an operator's intent | an operator relied on the exit | the REST surface's runtime row (`spec-doctor-coverage-4-api-surfaces` AC-5) names the failed service and the fix; `docs/architecture/behavior/signals.md` states the rule |
| R-4 | One unserved management surface answers several rows: AC-1's inverted `listeners` probe, AC-3's report-bus warning, and `spec-doctor-coverage-4-api-surfaces` AC-5's runtime row | duplicate rows for one address in `show doctor` | resolved (2026-11-11, main session, from the owner rule that `ze doctor` reports every problem with its cause and fix, and one fault one row; child 4 R-6): one unserved surface yields one row, child 4's runtime check, for its eight surfaces (web, LG, REST, gRPC, MCP, gNMI, the Prometheus exporter, SSH). The hub writes the cause through the surface's setter, REST and gRPC through their `restBuild`/`grpcBuild` seam (`cmd/ze/hub/api_infra.go`), and raises nothing on the bus for them; AC-3's warning covers only the other services; AC-1's probe on route `daemon` skips an address a surface reports in its serving state |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | `show doctor` rows wrong or missing; a REST bind failure now leaves the daemon up instead of down; gNMI with an unreadable certificate now does not start |
| How is it reverted? | single commit revert per phase |
| Who else touches this path? | the api-surfaces child (management listener codes), the config-yang child (confirm codes), the system and appliance children (the per-subsystem reload side effects), `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` P0b (raise route) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show doctor` on a daemon serving web | → | `checkListeners` with route `daemon` | `test/plugin/show-doctor-listeners-held.ci` |
| SIGHUP with a config that fails validation | → | `runReloadContext` outcome record, `hub-runtime` | `test/reload/reload-refused-show-doctor.ci` |
| `ze start` with REST on a taken port | → | `runYANGConfig` REST arm, cause written through the REST surface setter, the REST runtime row (child 4 AC-5) | `test/plugin/rest-bind-failure-keeps-daemon.ci` |
| `ze start` with a gNMI certificate path that does not exist | → | `service_gnmi.go` start refusal over `zegnmi.Server.Serve` | `test/plugin/gnmi-tls-unreadable-cert-not-started.ci` (landed in 537b3a87f9) |
| `commit confirmed` then `show doctor` | → | `hub-runtime` over `daemonConfirmWindow` | `test/reload/confirm-window-show-doctor.ci` |
| managed edge whose hub refuses its token, then `show doctor` | → | `managed-runtime` | `test/managed/edge-refused-show-doctor.ci` |
| `ze doctor` after a failed start | → | `start-local` over the last-start-failure key | `test/ui/doctor-last-start-failure.ci` |

## Acceptance Criteria

Every finding is a `Diagnostic` with code, severity, a message that names the service, address, file, user or hub and the cause in plain words, and `Fix` holding the exact command alone. "Runtime" means `show doctor` in the daemon (and `ze doctor` when the daemon answers); "local" means a post-config check, which runs on both routes. Codes marked new are registered by this spec in `internal/core/diagnostic/codes.go`; the others are registered by the child named in Depends, and this spec reuses the spelling. Every log line named below carries its code in the `code` attribute.

### Shared mechanisms (P1, P2)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `show doctor` on a daemon that serves every configured listener (AC-9 row: `listeners`) | an address-in-use bind reads healthy on route `daemon`; a configured endpoint the probe binds is reported Warning "`<service>`: configured on `<addr>` but nothing serves it", code the listener's code, fix `ze signal reload`. On route `daemon` the probe skips an address a management surface reports in its serving state (child 4 AC-1), because that surface's runtime row already answers for it (R-4). On route `local` the verdict is today's; on `daemon-unanswered` an address-in-use result is one Info row per listener under `doctor-runtime-not-checked`, fix `show doctor` from an account that can log in (fixes journal row `gate-fires-outside-its-population`, 2026-11-10) | a UDP endpoint another process holds reads healthy on route `daemon` (R-1): the hub and plugin raises report ze's own bind failure |
| AC-2 | Any reload (SIGHUP, CLI commit, `request reload`, managed push) | `runReloadContext` writes one last-reload record: trigger, time, verdict (applied, refused, applied-partially), the refusal text, each failed unwind step, each failed side-effect step; `hub-runtime` declares the reload codes and reads it | the record holds the last reload only: an earlier refusal is superseded by a later success, which is the current state |
| AC-3 | A management service the boot did not start (any cause) | for web, LG, REST, gRPC, MCP, gNMI, the Prometheus exporter and SSH, the hub writes the cause through that surface's setter (REST and gRPC through the `restBuild`/`grpcBuild` seam) and raises nothing, so the surface's own runtime row (`spec-doctor-coverage-4-api-surfaces` AC-5) is the one row (R-4); for any other management service the hub raises an active report-bus warning, source `hub`, subject the service and address, code `doctor-mgmt-listener-not-serving`, with its fix; the reload that starts the service clears it | a service that started and later stopped serving is the api-surfaces child's class |
| AC-4 | A start that fails, then `ze doctor` | `logStartupFailure` writes the last-start-failure key (stage, error, time); writing the ready file clears it; `start-local` reports Error "ze did not start: stage `<stage>`: `<error>`", code `doctor-daemon-start-failed` (new), fix `ze start` once the cause is repaired, preceded by the cause's own fix when the error carries one | a failure before the store opens writes no key: `doctor-storage-unavailable` already reports that state |
| AC-5 | The managed client runs | it publishes a value snapshot: connected, hub address, last error and its time, last attempt, last ack (ok, error, version); the snapshot holds no token | none |

### Boot and listeners (P3)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-6 | REST or gRPC fails to build or bind at boot | the daemon keeps running, as it does for SSH, web, MCP, gNMI and the looking glass; the failure is a Warn line and the cause written through the REST or gRPC surface setter by the `restBuild`/`grpcBuild` seam (AC-3, R-4); `show doctor` reports it once, as the surface's runtime row of `spec-doctor-coverage-4-api-surfaces` AC-5 ("`rest`: not serving on `<addr>`: `<cause>`"), fix `ze signal reload` once the address is free (fixes journal row `one-failure-skips-independent-startup-work`, 2026-11-10) | the unauthenticated non-loopback refusal stays fatal (A-3): it is a security refusal |
| AC-7 | `gnmi tls cert` or `key` names a file that cannot be read, or only one of the two is named | gNMI does not start and the daemon keeps running; the refusal names the file, the setting and the read error, with fix `ze signal restart` once the file is restored, because a reload never rebuilds gNMI; `show doctor` and `ze doctor` report it through the post-config check `gnmi-tls-material`, code `doctor-gnmi-tls-material` (`plan/release/validation/spec-doctor-coverage-4-api-surfaces.md` AC-10) (fixes journal row `silent-fall-through`, 2026-11-10). LANDED in 537b3a87f9: `TestGNMIUnreadableCertNotStarted`, `TestGNMIReadableCertStarted`, `TestServeRefusesUnloadableTLSMaterial`, `TestServeTLSWithValidMaterial`, `test/plugin/gnmi-tls-unreadable-cert-not-started.ci`; the hub's refusal log line in `serveGNMI` names `doctor-gnmi-tls-material`, LANDED in ad9fc4abe2. Owed: the `silent-fall-through.md` gNMI row (2026-11-10) still reads `open:` and promises "a coded report"; at closure its Fix cell turns `fixed:` citing 537b3a87f9 and ad9fc4abe2, worded as what landed (a coded log line and the `gnmi-tls-material` row, no report-bus raise) | none: clear text over a config that asked for TLS is never served |
| AC-8 | SSH config error or start failure, a service factory error, web, gNMI, MCP or LG not bound at boot | each is a coded Warn line and the AC-3 raise; `show doctor` lists one row per service; the stderr lines stay for an interactive operator | none |
| AC-9 | A reload accepts a change that needs a restart: an SSH listener change with a handle that cannot move, an LG certificate on a plaintext listener | `hub-runtime` reports Warning "`<leaf>` accepted but not applied until ze restarts", code `doctor-config-restart-needed` (new), fix `ze signal restart`; both Warn lines name the code | a restart clears the note, because the record lives in the process |
| AC-10 | `environment web insecure true` not honoured (local) | `hub-local` reports Warning naming the leaf and why, code `doctor-config-web-insecure-ignored` (new), fix `set environment web enabled true` with the server address named in the block; the hub's `remedy` and the archive line's `check` attributes become `fix`, and the archive line names `doctor-archive-invalid`, its covered class | none |
| AC-11 | Two or more `plugin hub server` entries | the plugin hub binds every one; a server that cannot bind is a Warn line and a raise under `doctor-listen-unavailable`, fix `ze signal reload` once the address is free (fixes journal row `unwired-feature`, 2026-11-10) | none |
| AC-12 | The daemon died at any start stage, gokrazy store auto-init included | AC-4's row; the gokrazy auto-init Error line names `doctor-daemon-start-failed` | the store half keeps `doctor-storage-unavailable` and `doctor-platform-perm` |

### Reload (P2)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-13 | The last reload was refused | `hub-runtime` reports Error "the last reload (`<trigger>`, `<time>`) was refused: `<refusal>`; ze runs the configuration it had before", code `doctor-config-reload-refused`; fix the refusal's own fix line when it carries one, else `ze config validate`; the SIGHUP refusal is also a coded slog line, not stderr only; the transient error event under the same code already exists (3ce015b3ff, raised by the plugin server), and this row must not double it (AC-34) | none |
| AC-14 | A refused reload whose unwind failed | Error "the reload was refused and `<step>` failed: ze runs part of the old and part of the new configuration", code `doctor-config-reload-unwind-failed` (new), fix `ze signal restart`; the listener rollback Error line names the code | none |
| AC-15 | A reload that applied while a system side effect failed | Warning "the reload applied, but `<step>` failed: `<error>`", code `doctor-config-reload-partial` (new), fix `ze signal reload`; each `apply*` step returns its failure to the record | the per-subsystem checks of those steps stay with the system and appliance children |
| AC-16 | The boot configuration was not recorded | Error "configuration changes are refused until ze restarts: the boot configuration was not recorded: `<error>`", code `doctor-config-boot-record-failed` (new), fix `ze data check`; the Error line names the code and keeps `fix` | none |
| AC-17 | A daemon reload audit record fails | the Warn line names `doctor-audit-record-failed` and raises by the route `spec-doctor-coverage-12-plugin-infra` builds | none: the check is that child's |

### Authentication, sessions and the confirm window (P4)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-18 | A configured AAA backend did not build | the bundle keeps the backends that failed; `hub-runtime` reports Error "`<backend>` cannot authenticate: `<error>`", code `doctor-aaa-backend-unavailable` (new), fix `ze config validate`; the two Error lines name the code | none |
| AC-19 | Local authentication refused (no user source, or users unreadable) | the three Warn lines name `doctor-mgmt-user-list-unreadable` and raise an event; `hub-runtime` declares the code | none |
| AC-20 | A password leaf that is not a bcrypt hash | the doctor row is `config-passwords` (`plan/release/validation/spec-doctor-coverage-9-config-yang.md` AC-4), which owns `doctor-config-password-plaintext`; this spec adds no check for it, and the hub's `password leaf format invalid` Warn line in `infra_setup.go` names that code | none: one class, one check (`ai/rules/no-layering.md`) |
| AC-21 | The zefs power user cannot be read | `hub-local` reads `usersFromStore` and reports Warning "the break-glass account cannot log in: `<error>`", code `doctor-aaa-power-user-unavailable` (new), fix `ze data check`; `errAdminDisabledInZefs` stays silent | the admin-disabled declaration is not a fault |
| AC-22 | An attached console or SSH session gets no config mode | the three Warn lines name `doctor-cli-editor-unavailable` and raise an event naming the user | none |
| AC-23 | A confirmed-commit window is open | runtime: Warning "a confirmed commit by `<user>` reverts in `<n>`s unless confirmed", code `doctor-config-confirm-pending`, fix the `commit accept` command `contract.CommitCommand` spells; a failed revert is Error under `doctor-config-confirm-revert-failed` naming the choices `Status.Line` gives. Local with no daemon: Warning "an unconfirmed commit is recorded and reverts at the next start", same pending code, fix `ze start` | the deadline revert itself and its failure belong to the config-yang child's `config/confirm`; this spec reports the hub's window |

### Managed configuration and pushed configs (P5)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-24 | Managed clients configured and the hub-side server is off (CA, build, start) | `hub-runtime` reports Error "the managed config server is not running: `<cause>`", code `doctor-managed-server-disabled` (new), fix `ze signal reload` once the cause is repaired; the three Error lines name the code | none |
| AC-25 | The edge is not connected to its hub | `managed-runtime` reports Warning "not connected to hub `<addr>` since `<time>`: `<last error>`" (a TLS or token refusal named as such), code `doctor-managed-session-down` (new), fix `ze signal reload` once the hub or token is repaired; `hub-unreachable` stays the local TCP probe | a reconnect inside its backoff reads as down until it connects, which is the state |
| AC-26 | A hub-pushed config not applied (validation or commit refused, fetch failed, bad payload, ack not sent) | `managed-runtime` reports Error naming the hub version and the cause, code `doctor-managed-config-not-applied` (new), fix `ze config validate` on the hub's blob for this edge; the four Warn lines name the code | none |
| AC-27 | `ze.managed.tls.insecure` set | `managed-local` reports Warning "managed TLS certificate verification is disabled", code `doctor-managed-tls-insecure` (new), fix `unset ZE_MANAGED_TLS_INSECURE`, with the hub CA named in the client's `pki` block; the Warn line names the code | none |
| AC-28 | The managed client did not start (stored config unreadable, unparsable, hub block not extracted) | `managed-local` reports Error with the cause, code `doctor-managed-client-not-started` (new), fix `ze config validate`; the three Warn lines name the code | none |
| AC-29 | A fleet-pushed config ignored as invalid, not applied, or `config-active-hash` not written | the push outcome key records it; `start-local` reports Warning, code `doctor-config-push-not-applied` (new), fix `ze config validate /perm/ze/config-pushed.conf`; the stderr lines become coded slog lines | none |
| AC-30 | A pushed config auto-reverted after a session flap | `start-local` reports Warning "the pushed config was reverted at `<time>` after a BGP session closed: `<reason>`", code `doctor-config-push-reverted` (new), fix `show bgp peer` before the next push | a later healthy push clears it |
| AC-31 | The auto-revert could not complete (previous not saved, previous and seed unreadable or unwritable, last-known-good not written) | `start-local` reports Error naming the step, code `doctor-config-push-revert-failed` (new), fix `ze init --seed` (the seed restore `docs/guide/quickstart.md` documents) | none |

### Not problem classes, and coverage (P6)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-32 | The eight not-problem groups | AAA bundle close errors on swap (`aaa_lifecycle.go` `closeRetiredAAABundle`, `releaseAccountingBundle`; `main_reload.go`), the signal announcement not published (`signal_announce.go` `announceSignal`), the ephemeral SSH address file (`service_ssh.go`), the zetest boot-record hold (`register_boot_record_hold_zetest.go` `holdBootRecord`): `doctor-internal-defect`. The rebuilt-active notice (`config_source.go`): `doctor-store-integrity`. The `--web-only` resolver line (`service_web.go`): `doctor-dns-resolver`. The drain deadline line (`drain.go` `runStopDrain`): `doctor-recent-events`. Lines that report nothing wrong drop to Info: "drain skipped by signal" (`stopSignals.onSignal`, the operator's second signal) and "sequenced listener migration (brief gap expected)" | none |
| AC-34 | The last reload was refused, under the code `doctor-config-reload-refused` that already exists | the code is registered: `spec-doctor-coverage-12-plugin-infra` landed it in 3ce015b3ff (the constant `CodeDoctorConfigReloadRefused` and its metadata in `internal/core/diagnostic/codes_plugin_infra.go`) as an event code of the plugin server (`serverEventCodes`, `diagnostic.RegisterEventCode`), and the plugin server raises an error event under it, source `plugin-server`, subject `config-reload`, at four sites: the transaction that did not commit (`reloadConfig`) and three auto-load refusals (`selectAutoLoad`, two in `startAutoLoaded`); its fix is the refusal's own fix command when the error carries an operator error (`errors.AsType[*opererr.Error]`), else `ze show errors`. This spec owes: the `hub-runtime` state row of AC-13 declares the same code, and one fault gives one row by one of two routes, chosen in this spec's design: either the `recent-events` check skips an event whose code a state check declares when that check reports the same fault (today `checkRecentEvents` drops only an active warning whose code another runtime check declares, `warningsNoCheckOwns`: "Error events stay, because they are history no state check reports"), or the plugin server's raise is removed when the state row lands. `hub-runtime` also declares `config-plugin-verify`, the code of a plugin's config verify refusal: when the last reload was refused by a plugin's verify, the AC-13 row carries code `config-plugin-verify`, names the plugin and its refusal, and its fix is the refusal's own fix line, else `ze config validate`. Telling a verifier's refusal from another verify-phase failure (a timeout, a cancelled context, a lost connection all arrive as one wrapped error) needs a typed refusal carried from the plugin's answer, which no spec has built, so none of the four 3ce015b3ff sites carries `config-plugin-verify`. `config-plugin-verify` is already declared by the `config-semantics` check (`semanticsDoctorCheck`, `internal/component/config/doctor.go`), which reports the refusal of the configuration on disk; `hub-runtime` declares it too because its AC-13 row reports a different fact under it, the refusal of the last reload, and a check must declare every code it emits. The one-route rule (149497ce12) bounds how one check is declared, not how many checks name a code: `hub-runtime` is a `diagnostic.DoctorCheck` (`runtime` phase), whose registry refuses a duplicate code only inside one check (`validateDoctorCheckCodesReg`, `internal/core/diagnostic/doctor_registry.go`), and the coverage gate records every declaring check per code (`Covered.add`, `internal/le/arch/doctorcoverage/codes.go`). The shared codes and the raise convention with its fix come from child 12's phase P0a, cited by name, never rebuilt here | none: one refused reload gives one row |
| AC-33 | `./le arch doctor-coverage report` over `cmd/ze`, `cmd/ze/hub`, `internal/component/managed`, `internal/component/engine` | zero uncovered Warn or Error sites | none |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestListenersDaemonRouteHealthyWhenHeld` | `internal/component/doctor/checks_listener_test.go` | AC-1, a held listener on route `daemon` gives no row | |
| `TestListenersDaemonRouteReportsUnserved` | same | AC-1, a bind that succeeds on route `daemon` is the finding | |
| `TestListenersDaemonUnansweredIsInfo` | same | AC-1, address-in-use on `daemon-unanswered` | |
| `TestReloadOutcomeRecord` | `cmd/ze/hub/main_reload_test.go` | AC-2, AC-13 to AC-15 verdicts | |
| `TestHubRuntimeCheckRegistered` | `cmd/ze/hub/doctor_test.go` | A-1, every code this spec owns is declared and resolves | |
| `TestServiceNotStartedRaises` | `cmd/ze/hub/service_registry_test.go` | AC-3, AC-8 raise and clear | |
| `TestRESTBindFailureKeepsDaemon` | `cmd/ze/hub/main_test.go` | AC-6 | |
| `TestGNMIUnreadableCertNotStarted` | `cmd/ze/hub/service_gnmi_test.go` | AC-7 | |
| `TestPluginHubBindsEveryServer` | `cmd/ze/hub/main_test.go` | AC-11 | |
| `TestRestartNeededRecord` | `cmd/ze/hub/listener_migrate_test.go` | AC-9 | |
| `TestBootRecordFailureRow` | `cmd/ze/hub/config_source_test.go` | AC-16 | |
| `TestAAABackendUnavailableRow` | `cmd/ze/hub/infra_setup_test.go` | AC-18 | |
| `TestPowerUserUnavailableRow`, `TestWebInsecureIgnoredRow` | `cmd/ze/hub/doctor_test.go` | AC-10, AC-21 | |
| `TestConfirmWindowRows` | `cmd/ze/hub/confirm_window_test.go` | AC-23 runtime and local | |
| `TestManagedServerDisabledRow` | `cmd/ze/hub/managed_server_test.go` | AC-24 | |
| `TestManagedSessionSnapshot`, `TestManagedRuntimeRows` | `internal/component/managed/client_test.go`, `doctor_test.go` | AC-5, AC-25, AC-26 | |
| `TestManagedLocalRows` | `internal/component/managed/doctor_test.go` | AC-27, AC-28 | |
| `TestStartFailureKey`, `TestPushOutcomeRows` | `cmd/ze/pushed_config_test.go`, `cmd/ze/health_revert_test.go` | AC-4, AC-29 to AC-31 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| confirm window seconds left | 0 to the window length | 0 (reverting) | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-doctor-listeners-held` | `test/plugin/*.ci` | a healthy daemon's `show doctor` has no `cannot bind` row | |
| `reload-refused-show-doctor` | `test/reload/*.ci` | a refused SIGHUP shows in `show doctor` with its fix | |
| `rest-bind-failure-keeps-daemon` | `test/plugin/*.ci` | REST on a taken port: the daemon keeps running, `show doctor` names REST | |
| `gnmi-tls-unreadable-cert-not-started` | `test/plugin/*.ci` | gNMI with a missing cert serves nothing in clear text (landed in 537b3a87f9) | |
| `confirm-window-show-doctor` | `test/reload/*.ci` | an open window shows its countdown and `commit accept` | |
| `edge-refused-show-doctor` | `test/managed/*.ci` | an edge refused by its hub shows the refusal | |
| `doctor-last-start-failure` | `test/ui/*.ci` | `ze doctor` after a failed start names the stage | |

## Files to Modify
- `internal/core/diagnostic/doctor_registry.go` - the route field on `DoctorCheckContext`
- `internal/core/diagnostic/codes.go` - the new codes and their metadata spellings
- `internal/component/doctor/doctor.go`, `answer.go`, `local_route.go`, `checks_listener.go` - the route, the probe's reading (AC-1)
- `cmd/ze/hub/main.go` - REST and gRPC arm (AC-6), plugin hub servers (AC-11), start failure record (AC-4), web insecure (AC-10), AAA (AC-18), power user (AC-21)
- `cmd/ze/hub/main_reload.go`, `reload_unwind.go`, `main_system.go` - outcome record, coded lines, `apply*` returning failures (AC-2, AC-13 to AC-15)
- `cmd/ze/hub/service_registry.go`, `service_ssh.go`, `service_web.go`, `service_gnmi.go`, `service_lg.go`, `service_mcp.go`, `service_rest.go`, `service_grpc.go` - coded lines and raises (AC-3, AC-7, AC-8)
- `cmd/ze/hub/listener_migrate.go`, `config_source.go`, `infra_setup.go`, `main_servers.go`, `session_editor.go`, `session_factory.go`, `managed_server.go`, `confirm_window.go`, `aaa_lifecycle.go`, `drain.go`, `signal_announce.go`, `register_boot_record_hold_zetest.go` - coded lines and records
- `cmd/ze/ze_core_start.go`, `pushed_config.go`, `health_revert.go` - coded lines, push outcome key
- `internal/component/managed/client.go`, `tls.go`, `doctor.go`, `register.go` - snapshot, coded lines, checks
- `pkg/zefs/keys.go` - the two keys
- `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/zefs-format.md`, `docs/architecture/behavior/signals.md`, `docs/guide/fleet-config.md`, `docs/guide/config-editor.md` - the pages these behaviours change

## Files to Create
- `cmd/ze/hub/doctor.go` (+ test) - `hub-runtime`, `hub-local`, the reload record and the restart-needed record
- `cmd/ze/doctor_start.go` (+ test, `ze_core`) - `start-local`
- the seven `.ci` files of the Wiring Test table

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no config or RPC added |
| YANG validation constraints | No | none added |
| YANG custom validators | No | none |
| CLI commands/flags | No | `ze doctor` and `show doctor` exist |
| CLI grammar (keyword before value) | N-A | no command added |
| Editor autocomplete | N-A | no leaf |
| Functional test for new RPC/API | Yes | the seven `.ci` files above |
| Pipe completeness | N-A | the doctor answer's pipes are unchanged |
| Env var registration | No | `ze.managed.tls.insecure` is already registered; no new env |
| Doctor check for runtime dependencies | Yes | `hub-runtime`, `hub-local`, `start-local`, `managed-runtime`, `managed-local`, codes in `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | No | the records are read by doctor, not exported |
| BGP family surface | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | doctor's feature row is the parent's |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | `show doctor` shape unchanged |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/fleet-config.md` (managed rows), `docs/guide/config-editor.md` (confirm window row) |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/doctor-and-health-checks.md` (route field, hub checks), `docs/architecture/zefs-format.md` (two keys), `docs/architecture/behavior/signals.md` (reload outcome, listener failures keep the daemon) |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived at implementation by `./le spec citation anchors spec plan/release/validation/spec-doctor-coverage-11-hub-engine.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | any page stating that a REST bind failure stops ze is converted with AC-6 |

## Implementation Steps

Each phase is sized for one agent of about 60 tool calls. P1 first; P2 before P3 to P6.

| Phase | Files | ACs | Needs |
|-------|-------|-----|-------|
| P1 route and listeners (wiring first) | `internal/core/diagnostic/doctor_registry.go`, `internal/component/doctor/{doctor,answer,local_route,checks_listener}.go`; `show-doctor-listeners-held.ci` | AC-1 | - |
| P2 hub records and reload | new `cmd/ze/hub/doctor.go`, `main_reload.go`, `reload_unwind.go`, `main_system.go`, `config_source.go`, codes; `reload-refused-show-doctor.ci` | AC-2, AC-13 to AC-17 | P1; AC-17 needs child 12's code |
| P3 boot listeners | `main.go` (REST, gRPC, plugin hub), `service_*.go`, `listener_migrate.go`; `rest-bind-failure-keeps-daemon.ci` (AC-7's refusal and `gnmi-tls-unreadable-cert-not-started.ci` landed in 537b3a87f9; its coded log line remains) | AC-3, AC-6 to AC-11 | P2; child 12's raise-with-fix route; child 4's codes |
| P4 auth, sessions, confirm | `infra_setup.go`, `main_servers.go`, `aaa_lifecycle.go`, `session_*.go`, `confirm_window.go`; `confirm-window-show-doctor.ci` | AC-18 to AC-23 | P2 |
| P5 managed and start | `internal/component/managed/*`, `managed_server.go`, `cmd/ze/{ze_core_start,pushed_config,health_revert}.go`, `pkg/zefs/keys.go`, new `cmd/ze/doctor_start.go`; `edge-refused-show-doctor.ci`, `doctor-last-start-failure.ci` | AC-4, AC-5, AC-12, AC-24 to AC-31 | P2 |
| P6 remaining lines and coverage | `drain.go`, `signal_announce.go`, `register_boot_record_hold_zetest.go`, docs | AC-32, AC-33 | P2 to P5 |

Gates each phase owes, run by the main thread: the scoped package tests (`./le job run label <label> command go test -tags <tags> <package>` for `internal/component/doctor`, `cmd/ze/hub`, `cmd/ze`, `internal/component/managed`), the named `.ci` tests, `./le arch doctor-coverage report`, and `./le repo compiles check` after each Go commit.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named symbol |
| Correctness | AC-1 never reports a held listener as unbindable on route `daemon`; AC-6 keeps the security refusal fatal |
| Naming | every new code starts `doctor-`; one spelling in `codes.go`; a code another child owns is reused, never respelled |
| Data flow | runtime checks read records, never `Tree`; the reload record is written only in `runReloadContext` |
| Rule: user-facing-errors | each fix is one command on its own line |
| Rule: no-layering | the stderr-only paths become coded lines plus raises; the stderr echo stays only for an interactive operator |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| hub checks registered | `TestHubRuntimeCheckRegistered` |
| zero uncovered sites | `./le arch doctor-coverage report` over the four packages |
| four journal rows fixed | each row's Fix cell turns `fixed:` with its test |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail open | gNMI never serves clear text over a TLS config (AC-7); REST and gRPC failures keep the daemon up but never serve an unauthenticated listener |
| Secret leakage | no row prints a token, a password hash or a TLS key; the managed snapshot holds no secret |
| Authorization | `show doctor` keeps its existing authorization |

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
- Inside the daemon, a bind that succeeds is the finding, not the bind that fails: the probe's question inverts with the process it runs in.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `gate-fires-outside-its-population.md` | 2026-11-10 | doctor `listeners` bind probe run inside the daemon |
| `one-failure-skips-independent-startup-work.md` | 2026-11-10 | REST or gRPC bind failure stops the daemon, other listeners do not |
| `silent-fall-through.md` | 2026-11-10 | gNMI with an unreadable TLS file serves clear text |
| `unwired-feature.md` | 2026-11-10 | plugin hub binds only the first configured server |
| `guard-added-to-one-half-of-a-pair.md` | 2026-11-12 | managed hub rejection record keeps raw ack text and rows of clients a reload removed |
| `identity-default-hides-a-mapping.md` | 2026-11-13 | daemon `show doctor` judges the store's default-named config, not the file `ze start` loaded |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Every management listener failure keeps the daemon running (AC-6) | make SSH, web, MCP, gNMI and LG fatal like REST and gRPC | a router keeps routing when one management surface cannot bind; five of seven surfaces already behave so, and doctor plus the raise make the failure visible |
| The bind probe reads its route (AC-1) | move `listeners` to `runtime` and read each service's bound set | the runtime phase gets no `Tree`, and the plugin listeners (BGP, BFD, IPsec, TFTP, NTP) have no shared bound-set registry; the route-aware probe answers for all of them with one field |
| One last-reload record | one probe per reload row | the four reload rows share one producer; the record is built from values `runReloadContext` already holds |
| zefs keys for the start failure and the push outcome | a log scrape | the daemon is gone or not yet up when `ze doctor` asks; the store is what both sides read |
| Bind every plugin hub server (AC-11) | refuse more than one at verify | the managed server already serves several blocks; the list is the operator's intent |

## Known Limitations
- The per-subsystem reload side effects (tuning, console, hostname, conntrack, update checker, resolv.conf, SMART) get their own checks in the system and appliance children; AC-15 reports that a step failed, not its subsystem's detail.

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
- [ ] AC-1..AC-34 all demonstrated
- [ ] Targeted tests pass: `./le test functional reload`, `./le test functional plugin`, `./le test managed` (the named `.ci` files), and the scoped package tests of P1 to P6
- [ ] Every user story has a working path and a passing test (N-A: Scope cli, the Wiring Test table carries the paths)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
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
- [ ] Interop tests for protocol features (N-A: no protocol behaviour changes; the managed edge test runs ze against ze)
