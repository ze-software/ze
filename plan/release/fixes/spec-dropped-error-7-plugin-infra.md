# Spec: dropped-error-7-plugin-infra

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Priority | P1 |
| Release reason | 16 remaining errors across 7 files in the plugin process manager, the plugin server, the managed config server, the public RPC transport and the SDK are caught and logged only below WARN: a plugin process group that cannot be signalled, an unbounded stderr drain, an RPC reply or managed config-fetch answer that never reaches a live peer, and a failed runtime connection abort on request-ID collision, each with no trace at the default level |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child of `plan/release/fixes/spec-dropped-error-0-umbrella.md`, which holds the
resolution policy and the tree-wide acceptance. Door: `ai/doors/plugin-infra.md`.

Population: every `error-dropped-below-warn` finding of
`./le arch refuse-by-default report` under `internal/component/plugin/`,
`pkg/plugin/` and `pkg/ze/`. Audited against HEAD on 2026-11-26: **16 instances
in 7 files** (down from 20 instances in 8 files on 2026-11-02):

- `internal/component/plugin/process/process.go`: 5 instances
- `internal/component/plugin/server/dispatch.go`: 4 instances
- `internal/component/plugin/server/request_runtime.go`: 2 instances (added since 2026-11-02)
- `internal/component/plugin/server/managed_serve.go`: 1 instance (`pushConfigChanged` is already fixed at HEAD)
- `internal/component/plugin/server/startup_removal.go`: 1 instance
- `pkg/plugin/rpc/panic_guard.go`: 2 instances
- `pkg/plugin/sdk/sdk_async_callback.go`: 1 instance (consolidated from `sdk_dispatch.go` and `sdk_source_policy.go`)

Each instance is resolved by exactly one answer, judged in this order
(umbrella, Key Design Decisions):

| Order | When | Answer |
|-------|------|--------|
| 1 | Going on is wrong: the caller would act on the failure | Return the error, or fix the behavior the drop hid |
| 2 | Going on is right and an operator can act | WARN with what, why and the fix (`ai/rules/user-facing-errors.md`) naming the plugin or managed client; under `internal/component/plugin/`, emit a registered doctor `code` attribute and raise a report-bus event under that same code |
| 3 | Going on is right and nobody can act (teardown close, an `ESRCH` on an already-exited group, or a reply whose peer connection is already closed/stopping) | `// refuse-by-default: guards <Symbol> -- <why>` on the line above, or an explicit branch separating the benign condition from the actionable failure |

**`doctor-coverage` commit gate requirement:** `./le arch doctor-coverage check`
and `./le commit create` refuse any added or reworded `Warn` or `Error` log line
under `internal/component/plugin/` that lacks a covered `code` attribute. Every
new `Warn` or `Error` log added under `internal/component/plugin/` MUST:

1. Emit `"code", <CodeConstant>` on the log line using a code registered via
   `diagnostic.Register` in the owning package's doctor-codes file
   (`internal/component/plugin/process/doctor_codes.go` or
   `internal/component/plugin/server/doctor_codes.go`, following the
   `register_doctor_codes.go` ownership rule alongside
   `internal/component/plugin/doctor/register_doctor_codes.go`) and verified by
   `TestPluginInfraDoctorCodesDescribed` in
   `internal/component/plugin/doctor/doctor_codes_test.go`.
2. Declare that code in `diagnostic.RegisterEventCode` (`processEventCodes` in
   `internal/component/plugin/process/register.go` or `serverEventCodes` in
   `internal/component/plugin/server/register.go`) when it is a transient event
   code rather than an existing check's code.
3. Raise a matching transient event on the report bus (`internal/core/report`)
   under that same code so `ze show doctor` and `ze show errors` report it
   (`docs/architecture/doctor-and-health-checks.md`, "Event codes": every site
   carrying a registered event code raises it).

`pkg/plugin/rpc` and `pkg/plugin/sdk` are public APIs outside the
`doctor-coverage` call-site roots (`internal/component`, `internal/plugins`,
`cmd/ze`): no exported signature changes and no `internal/core/diagnostic`
import are permitted there. A live-connection send failure in `pkg/plugin/sdk`
logs at `WARN` through `slog.Warn` (which `relayStderrFrom` relays into engine
logs at `WARN`).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, public `pkg/` boundary, user-facing errors
  → Constraint: `pkg/plugin/rpc` and `pkg/plugin/sdk` are published contracts; no exported name, JSON tag, or signature changes here
- [ ] `ai/doors/plugin-infra.md` - plugin process, server, RPC and SDK boundaries
  → Constraint: a field or method name in `rpc` is wire; no exported name or signature changes in `pkg/`
  → Constraint: one plugin failure stops the startup tier, by owner ruling; every server error path calls `PluginFailed` and `proc.Stop()`, so no answer here may make a failure survivable
  → Constraint: the SDK stays generic; no plugin-specific branch is added to report an error
- [ ] `docs/architecture/api/process-protocol.md` - error answers, shutdown, panic containment
  → Constraint: an error answer is a line on the plugin's connection; when the connection is closed or stopping, process supervision reports the plugin's exit, whereas a send failure on a live connection leaves the peer waiting without its answer
- [ ] `docs/architecture/doctor-and-health-checks.md` - "Every problem site names a doctor code" and "Event codes"
  → Constraint: every new `Warn`/`Error` log under `internal/component/plugin/` carries a covered `code` attribute registered in the owning package's `doctor_codes.go` / `register_doctor_codes.go` and declared via `diagnostic.RegisterEventCode` in `register.go`, and raises its event on the report bus under that code
- [ ] `docs/architecture/fleet-config.md` - "What doctor reads from the managed listener" and "Protocol"
  → Constraint: managed listener doctor events are documented in the doctor table and name the client and cause without leaking secrets
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: every operator-facing WARN names what went wrong in operator terms, why it matters, the exact next step/command, and its doctor `code`
- [ ] `docs/contributing/ze-go-style.md` - "Error dropped below WARN"
  → Constraint: return, count, or WARN; the annotation names a visible guard symbol other than the enclosing function

**Key insights:**
- Reading `pkg/plugin/rpc/conn.go` (`SendResult`, `writeFrame`) disproves the earlier assumption that a failed `SendError`/`SendResult` always means the peer connection is closed: `json.Marshal` errors, `rpc.MaxMessageSize` overflows, and write deadlines fail while the peer connection stays open and the caller waits. Reply-send sites must therefore separate closed/stopping sessions (Answer 3 naming the supervision guard) from live-connection delivery failures (Answer 2 with `CodeDoctorPluginAnswerUndelivered` or `CodeDoctorManagedConfigFetchUndelivered` / `slog.Warn`).
- Reading `pkg/plugin/rpc/mux.go` (`MuxConn.Close`) shows `closeOnce.Do` runs `m.conn.Close()` at most once, so a non-`net.ErrClosed` error from `conn.AbortRuntime` in `request_runtime.go:37` will not be retried by `cleanupProcess`; it must warn under `diagnostic.CodeDoctorPluginRequestRefused`.
- Reading `internal/component/plugin/sysproc.go` (`KillProcessGroup`) shows `syscall.ESRCH` is the normal outcome when a stopped plugin exited with no surviving descendants, while any other error (such as `syscall.EPERM`) means the process group was not signalled and may still hold resources.

## Current Behavior (MANDATORY)

**Source files read (HEAD 2026-11-26):**
- [ ] `internal/le/arch/refusebydefault/dropped.go` - `droppedError`, `belowWarn`, `drops`
- [ ] `internal/le/arch/refusebydefault/annotation.go` - `// refuse-by-default: guards <Symbol> -- <why>`, visible-symbol check, enclosing-function check
- [ ] `internal/component/plugin/process/process.go` - 5 findings at lines 750, 939, 1068, 1100, 1378
- [ ] `internal/component/plugin/process/doctor_codes.go` and `register.go` - existing process doctor code and event-code registrations
- [ ] `internal/component/plugin/sysproc.go` and `sysproc_linux.go` - `KillProcessGroup` (`syscall.Kill(-pid, syscall.SIGKILL)`)
- [ ] `internal/component/plugin/server/dispatch.go` - 4 findings at lines 115, 133, 669, 675
- [ ] `internal/component/plugin/server/dispatch_registry.go` - refactored into `answerEngineOpJSON` (returns send/write errors; zero findings at HEAD)
- [ ] `internal/component/plugin/server/request_runtime.go` - 2 findings at lines 37, 44
- [ ] `internal/component/plugin/server/managed_serve.go` - 1 remaining finding at line 737 (`handleRequest`); `pushConfigChanged` (lines 924-964) already fixed via `notifyDropped`
- [ ] `internal/component/plugin/server/managed_doctor.go` - `notifyDropped` (WARN + `CodeDoctorManagedClientNotifyDropped` + `report.RaiseErrorFix`)
- [ ] `internal/component/plugin/server/doctor_codes.go` and `register.go` - `CodeDoctorPluginAnswerUndelivered`, `CodeDoctorPluginRequestRefused`, `CodeDoctorManagedClientNotifyDropped`, `serverEventCodes`
- [ ] `internal/component/plugin/doctor/doctor_codes_test.go` - `TestPluginInfraDoctorCodesDescribed`
- [ ] `internal/component/plugin/server/startup_removal.go` - 1 finding at line 232 (`stopRemovedProcess`)
- [ ] `internal/component/plugin/ipc/rpc.go` - `AbortRuntime`, `SendResult`, `SendError`
- [ ] `pkg/plugin/rpc/conn.go` and `mux.go` - `writeFrame`, `SendResult`, `SendError`, `MuxConn.Close` (`closeOnce`)
- [ ] `pkg/plugin/rpc/panic_guard.go` - 2 findings at lines 72, 85
- [ ] `pkg/plugin/sdk/sdk_async_callback.go` - 1 finding at line 85 (`answerAsyncCallback`, replacing the 2026-11-02 `sdk_dispatch.go` and `sdk_source_policy.go` sites)
- [ ] `pkg/plugin/sdk/sdk_dispatch.go` and `sdk_source_policy.go` - confirmed zero `error-dropped-below-warn` findings at HEAD

**HEAD audit — retired/already-resolved sites since 2026-11-02:**
- `ManagedServer.pushConfigChanged` (`managed_serve.go:924-964`): already fixed at HEAD; calls `notifyDropped` (`managed_doctor.go:35-44`), which logs at `WARN` with `CodeDoctorManagedClientNotifyDropped` and raises on the report bus via `report.RaiseErrorFix`.
- `dispatch_registry.go` (`serveEngineOpJSON` lines 252, 261, 266): refactored into `answerEngineOpJSON` (lines 405-428), which returns `sendErr`/`writeErr` rather than dropping below WARN; zero findings in `dispatch_registry.go`.
- `sdk_dispatch.go` (`answerDrain` lines 380, 392) and `sdk_source_policy.go` (`answerSourcePolicy` line 278): unified into `pkg/plugin/sdk/sdk_async_callback.go:85` (`Plugin.answerAsyncCallback`).

**HEAD disposition of all 16 remaining instances:**

| # | File and Line (HEAD) | Function | Resolution | Doctor Code or Guard |
|---|----------------------|----------|------------|----------------------|
| 1 | `internal/component/plugin/process/process.go:750` | `Process.startInternal.func.func` | Answer 3 | `guards closeConns` — teardown close of in-process `net.Pipe` after runner returned or panicked; `closeConns` and `endSelectedEngine` drive shutdown and supervision |
| 2 | `internal/component/plugin/process/process.go:939` | `Process.startChildCmd` | Answer 2 | `CodeDoctorPluginStderrDrainUnbounded` — closing parent's stderr write fd failed after fork, so child exit hits `stderrDrainGrace` instead of EOF; extract small testable helper `closeParentStderrWrite` |
| 3 | `internal/component/plugin/process/process.go:1068` | `stderrRelayReader.extendDeadline` | Answer 3 on `os.ErrClosed`; Answer 2 (once per reader) otherwise | `os.ErrClosed`: `guards drainStderrRelay`; other errors: `CodeDoctorPluginStderrDrainUnbounded` (post-reap idle deadline unarmed) |
| 4 | `internal/component/plugin/process/process.go:1100` | `drainStderrRelay` | Answer 3 | `guards monitorCmd` — teardown close of read-only stderr pipe end after child was reaped and `<-relayDone` completed |
| 5 | `internal/component/plugin/process/process.go:1378` | `pluginGroup.signalLocked` | Silent/guarded on `syscall.ESRCH`; Answer 2 otherwise | `ESRCH`: `guards pluginGroup.exit` (group already gone); other errors (e.g. `EPERM`): `CodeDoctorPluginGroupSignalFailed` |
| 6 | `internal/component/plugin/server/dispatch.go:115` | `Server.dispatchPluginRPCContext.func` | Shared `noteReplyDeliveryFailure`: Answer 3 when server stopping or plugin ended; Answer 2 when live | Stopping/ended: `guards handleSingleProcessCommandsRPC`; live: `CodeDoctorPluginAnswerUndelivered` (rate-limited per plugin) |
| 7 | `internal/component/plugin/server/dispatch.go:133` | `Server.dispatchPluginRPCContext` | Same shared `noteReplyDeliveryFailure` | Stopping/ended: `guards handleSingleProcessCommandsRPC`; live: `CodeDoctorPluginAnswerUndelivered` |
| 8 | `internal/component/plugin/server/dispatch.go:669` | `Server.handleCodecRPC` (`SendError`) | Same shared `noteReplyDeliveryFailure` | Stopping/ended: `guards handleSingleProcessCommandsRPC`; live: `CodeDoctorPluginAnswerUndelivered` |
| 9 | `internal/component/plugin/server/dispatch.go:675` | `Server.handleCodecRPC` (`SendResult`) | Same shared `noteReplyDeliveryFailure` | Stopping/ended: `guards handleSingleProcessCommandsRPC`; live: `CodeDoctorPluginAnswerUndelivered` |
| 10 | `internal/component/plugin/server/request_runtime.go:37` | `Server.serveRuntimeRequests` (`AbortRuntime`) | Answer 3 on `net.ErrClosed`/`io.ErrClosedPipe`; Answer 2 otherwise | Closed: `guards cleanupProcess`; non-closed error: `diagnostic.CodeDoctorPluginRequestRefused` (`MuxConn.Close` uses `sync.Once` so teardown does not retry) |
| 11 | `internal/component/plugin/server/request_runtime.go:44` | `Server.serveRuntimeRequests.func` (`SendError`) | Same shared `noteReplyDeliveryFailure` | Stopping/ended: `guards handleSingleProcessCommandsRPC`; live: `CodeDoctorPluginAnswerUndelivered` |
| 12 | `internal/component/plugin/server/managed_serve.go:737` | `ManagedServer.handleRequest` (`config-fetch` `SendResult`) | Answer 3 when hub stopping or client closed; Answer 2 when live | Stopping/closed: `guards ManagedServer.serve`; live: `CodeDoctorManagedConfigFetchUndelivered` |
| 13 | `internal/component/plugin/server/startup_removal.go:232` | `Server.stopRemovedProcess` (`proc.Wait(s.Context())`) | Answer 3 | `guards Server.Stop` — `proc.Wait(s.Context())` errors only when server context cancels during shutdown; `releaseStoppedProcess` runs next |
| 14 | `pkg/plugin/rpc/panic_guard.go:72` | `Conn.endAfterPanic` (`c.Close()`) | Answer 3 | `guards Conn.readLoop` — panic already logged/counted/crash-recorded by `taskguard` and stored in `readerErr`; `readLoop` closes `readerDone` next |
| 15 | `pkg/plugin/rpc/panic_guard.go:85` | `MuxConn.endAfterPanic` (`m.Close()`) | Answer 3 | `guards MuxConn.readLoop` — panic already logged/counted/crash-recorded by `taskguard` and stored in `readerErr`; `readLoop` ends pending answers and closes `done` next |
| 16 | `pkg/plugin/sdk/sdk_async_callback.go:85` | `Plugin.answerAsyncCallback` | Answer 3 when loop stopping or conn closed; Answer 2 when live | Stopping/closed: `guards Plugin.eventLoop`; live: `slog.Warn` naming plugin, request ID, why and fix (relayed via stderr; `pkg/` is outside `doctor-coverage` `siteRoots`) |

**Behavior to preserve:**
- The process protocol wire format, the five startup stages, the startup-tier failure ruling, every exported `pkg/` type and function signature, and all healthy runtime paths.

**Behavior to change:**
- Each of the 16 remaining instances is resolved according to the table above so zero unannotated `error-dropped-below-warn` findings remain under `internal/component/plugin/`, `pkg/plugin/` and `pkg/ze/`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| refuse-by-default dropped-error shape | `internal/le/arch/refusebydefault/dropped.go` `droppedError` | Lists the 16 remaining instances under plugin-infra paths | Applies unchanged: the acceptance check for AC-1 |
| Guard annotation | `internal/le/arch/refusebydefault/annotation.go` `annotations`, `parseGuard` | Accepts `refuse-by-default: guards <Symbol> -- <why>` naming a visible non-enclosing symbol | Extends its use: applied to teardown and already-closed/stopping branches (Answer 3) |
| `doctor-coverage` commit gate | `internal/le/arch/doctorcoverage/` (`docs/architecture/doctor-and-health-checks.md`, "Every problem site names a doctor code") | Refuses any added/reworded `Warn`/`Error` under `internal/component`, `internal/plugins`, `cmd/ze` lacking a declared doctor `code` | Applies: every new `Warn` under `internal/component/plugin/` carries a registered doctor code and raises on the report bus |
| Process supervision | `internal/component/plugin/process/process.go` (`closeConns`, `endSelectedEngine`, `monitorCmd`, `PluginEnded`) and `internal/component/plugin/server/dispatch.go` (`handleSingleProcessCommandsRPC`, `cleanupProcess`) | Reports plugin exit and drives connection/process teardown | Applies: the guard named by Answer 3 annotations on teardown closes and closed-connection replies |
| Plugin server doctor codes and report-bus helpers | `internal/component/plugin/server/doctor_codes.go` (`CodeDoctorPluginAnswerUndelivered`), `register.go` (`serverEventCodes`), `server_events.go` (`raiseServerEvent`), `dispatch_report.go` (`refusePluginRequest`) | Declares `doctor-plugin-answer-undelivered` and `doctor-plugin-request-refused` and raises transient error events with rate limiting | Extends: reuses `CodeDoctorPluginAnswerUndelivered` for live reply failures in `dispatch.go`/`request_runtime.go`, `CodeDoctorPluginRequestRefused` for `AbortRuntime` failures, and adds `CodeDoctorManagedConfigFetchUndelivered` for live `config-fetch` send failures |
| Plugin process doctor codes and event-code registration | `internal/component/plugin/process/doctor_codes.go`, `register.go` (`processEventCodes`), `internal/component/plugin/doctor/doctor_codes_test.go` (`TestPluginInfraDoctorCodesDescribed`) | Registers process-owned doctor code metadata and report-bus event codes | Extends: adds `CodeDoctorPluginGroupSignalFailed` and `CodeDoctorPluginStderrDrainUnbounded` and verifies them in `TestPluginInfraDoctorCodesDescribed` |
| Managed config-changed push warning (already fixed at HEAD) | `internal/component/plugin/server/managed_serve.go` (`pushConfigChanged`, lines 924-964) and `managed_doctor.go` (`notifyDropped`, lines 35-44) | Already logs `WARN` with `CodeDoctorManagedClientNotifyDropped` and calls `report.RaiseErrorFix` naming the client, cause and fix | Does not apply (already fixed at HEAD): retired from this spec's edit set; `handleRequest` line 737 remains in scope |
| Engine-op JSON answer return (already refactored at HEAD) | `internal/component/plugin/server/dispatch_registry.go` (`answerEngineOpJSON`, lines 405-428) | Refactored from `serveEngineOpJSON` to return send/write errors instead of dropping below WARN | Does not apply (zero findings at HEAD): retired from this spec's edit set |
| SDK unified async callback answer | `pkg/plugin/sdk/sdk_async_callback.go` (`Plugin.answerAsyncCallback`, lines 76-89) | Consolidated the earlier `sdk_dispatch.go` (`answerDrain`) and `sdk_source_policy.go` (`answerSourcePolicy`) answer paths into one function | Extends: resolves the single remaining SDK site at `sdk_async_callback.go:85` without altering exported signatures |
| Panic containment | `pkg/plugin/rpc/panic_guard.go` (`readerPanics`) and `internal/core/taskguard` | Logs contained panics at ERROR, increments `ze_task_panics_total`, and writes crash records before `endAfterPanic` closes the connection | Applies: makes `endAfterPanic` `Close()` a non-actionable teardown close (Answer 3) |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | "A change under `pkg/` is a change to a published contract. A type, a field, a callback name and the bytes on the wire all count." | agrees | No exported type, field, callback name, function signature, or wire byte changes in `pkg/plugin/rpc` or `pkg/plugin/sdk`; only internal logging and annotations change. |
| `ai/doors/plugin-infra.md` | Traps | the door covering `internal/component/plugin/*`, `pkg/plugin/rpc/*`, `pkg/plugin/sdk/*` | "One plugin failure stops the startup tier, by owner ruling." | agrees | No startup error path is relaxed or made survivable; every edit resolves a dropped error below WARN via WARN+doctor-code or a guard annotation on teardown/closed-session paths. |
| `docs/architecture/api/process-protocol.md` | Panic containment | `// Design:` page of `process.go`, `dispatch.go`, `request_runtime.go`, `startup_removal.go`, `panic_guard.go`, `sdk_async_callback.go`, `server/register.go`, `process/register.go` | "A panic in the engine's side of the protocol ends one unit, never the daemon." | agrees | Preserves every panic containment boundary (`engine-op`, `rpc-read`, `rpc-route`, `run`), annotating the post-panic teardown `Close()` in `panic_guard.go` and warning only when a reply to a live plugin fails. |
| `docs/architecture/doctor-and-health-checks.md` | Every problem site names a doctor code | `// Design:` page of `process/doctor_codes.go`, `server/doctor_codes.go`, `doctor/doctor_codes_test.go`; modified by this spec | "A site is covered when its code is one that a doctor check declares." | agrees | Every new `Warn` log under `internal/component/plugin/` carries a declared doctor `code` attribute, registers its metadata and `diagnostic.EventCode`, raises its event on the report bus, and updates the `### Event codes` table in this page. |
| `docs/architecture/fleet-config.md` | What doctor reads from the managed listener | `// Design:` page of `managed_serve.go`; modified by this spec | "A `config-changed` a connected client was not given (the notification could not be built, or the client did not accept it)" | agrees | Preserves the existing `pushConfigChanged`/`notifyDropped` behavior and extends the managed doctor table with `doctor-managed-config-fetch-undelivered` when a live client's `config-fetch` answer cannot be sent, without leaking any client secret. |
| `docs/architecture/plugin/plugin-system.md` | The SDK stays generic | anchored to `internal/component/plugin/process/process.go` | "`pkg/plugin/sdk/` carries no plugin-specific code." | agrees | `Plugin.answerAsyncCallback` stays callback-agnostic and logs generically via `slog.Warn` using `p.name` and request ID with no plugin-specific branch. |
| `docs/architecture/api/ipc_protocol.md` | Backpressure | anchored to `internal/component/plugin/process/process.go` | N/A | silent | Page specifies IPC wire framing and queue backpressure; silent on below-WARN error logging and guard annotations in process teardown, RPC reply failure handling, and SDK worker answers. |
| `docs/architecture/api/architecture.md` | Subprocess (Process) | anchored to `process.go` and `managed_serve.go` | N/A | silent | Page describes API surface architecture, encoding, and socket client topology; silent on below-WARN dropped error logging and doctor-code emission in plugin-infra internals. |
| `docs/architecture/api/capability-contract.md` | Startup Protocol (✅ DONE) | anchored to `internal/component/plugin/process/process.go` | N/A | silent | Page defines capability negotiation and startup protocol timeouts; silent on below-WARN error drops in teardown closes and process-group signal failures. |
| `docs/architecture/api/commands.md` | The dispatcher takes the line with its pipe chain | anchored to `internal/component/plugin/server/dispatch.go` | N/A | silent | Page covers command registration, pipe dispatch, and answer surfaces; silent on below-WARN error logging for failed RPC replies on `dispatch.go`. |
| `docs/architecture/config/environment.md` | Env Var Registry | anchored to `internal/component/plugin/process/process.go` | N/A | silent | Page documents `env.MustRegister` and environment inheritance in `startExternal`; silent on `stderrWrite.Close` and process-group signal error handling. |
| `docs/architecture/config/syntax.md` | What a reader receives | anchored to `internal/component/plugin/server/startup_removal.go` | N/A | silent | Page documents configuration syntax and `ConfigurationFor` scope rules; silent on `stopRemovedProcess` wait error logging during daemon shutdown. |
| `docs/architecture/config/transaction-protocol.md` | 6. Transaction Exclusion | anchored to `internal/component/plugin/server/startup_removal.go` | N/A | silent | Page defines config verify/apply/rollback and removal recovery exclusion; silent on the shutdown-context `proc.Wait` debug log annotation in `stopRemovedProcess`. |
| `docs/architecture/core-design.md` | Inside Ze | anchored to `internal/component/plugin/server/managed_serve.go` | N/A | silent | Page covers system architecture and component boundaries; silent on below-WARN error logging of `mc.SendResult` failures for `config-fetch` in `managed_serve.go`. |
| `docs/architecture/diagnostics/event-history-rings.md` | Event History and FSM Transition Rings | anchored to `internal/component/plugin/server/dispatch.go` | N/A | silent | Page covers the event ring tap in `deliverEvent`; silent on RPC reply send error handling in `dispatchPluginRPCContext` and `handleCodecRPC`. |
| `docs/architecture/hub-architecture.md` | What the daemon does at start | anchored to `process.go` and `managed_serve.go` | N/A | silent | Page covers hub startup and environment inheritance; silent on below-WARN error drops in `process.go` teardown/signal paths and `managed_serve.go` `config-fetch` replies. |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | anchored to `internal/component/plugin/process/process.go` | N/A | silent | Page covers IKEv2 engine architecture and its in-process execution via `startInternal`; silent on `bridgedPluginSide.Close()` teardown annotation in `process.go`. |
| `docs/architecture/pki/tls-listeners.md` | Decision: doctor reads the parsed config, not the live store | anchored to `internal/component/plugin/server/managed_serve.go` | N/A | silent | Page covers TLS certificate resolution (`managedCertificate`); silent on `config-fetch` `SendResult` delivery failure logging in `handleRequest`. |
| `docs/architecture/plugin/rib-storage-design.md` | Retained selection for ordinary advertisement | anchored to `internal/component/plugin/server/dispatch.go` | N/A | silent | Page covers BGP RIB storage and `cleanupProcess` retention; silent on below-WARN reply send errors in `dispatch.go`. |
| `docs/architecture/plugin/show-enrichers.md` | Constraints | anchored to `internal/component/plugin/process/process.go` | N/A | silent | Page covers show-command enricher lifecycle anchored to `Process.Stop`; silent on stderr relay and process-group signal error handling in `process.go`. |
| `docs/architecture/testing/ci-format.md` | The compiled observer API | anchored to `process.go` and `dispatch.go` | N/A | silent | Page documents `.ci` test directives and stderr relay observation; silent on below-WARN error handling in `process.go` and `dispatch.go`. |
| `docs/architecture/update-cache.md` | Cache Consumer Model | anchored to `internal/component/plugin/process/process.go` | N/A | silent | Page covers UPDATE cache consumer flags on `Process`; silent on stderr pipe, process-group signalling, and teardown close error handling in `process.go`. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 on `process-protocol.md` | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Adds no engine operation and modifies no authorization profile or filter check. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 on `process-protocol.md` | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Does not touch event queuing or delivery paths; lossless event delivery is unchanged. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Does not modify `\| implied` formatting or config rendering. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | Does not modify bus participant naming or default message registration. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Does not touch `send > to` compilation or message routing. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Does not touch `receive > from` authorization or filtering. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Does not touch bus send/receive allow-lists or enforcement. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Does not touch bus filter compilation or commit-time refusal checks. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Introduces no bus arbitration or exclusive-role change. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 on `api/architecture.md` | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Does not touch bus display commands or effective-entry output. |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 on `plugin-system.md` | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | Does not touch external-plugin query mode or offline config parsing. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 on `commands.md` | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Adds no CLI command and changes no command/pipe grammar. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 on `commands.md` | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Does not change pipe operator overrides or command answer formats. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 on `commands.md` | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Does not modify pipe operation semantics or command execution. |
| `docs/architecture/fleet-config.md` | Design decisions | decision of 2026-11-23 on `fleet-config.md` | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | Does not modify managed listener certificate reload or TLS handshake behavior. |
| `docs/architecture/fleet-config.md` | Design decisions | decision of 2026-11-25 on `fleet-config.md` | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | Does not touch fleet web provisioning or offline device configuration commits. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Does not touch kernel/VPP plane validation or FIB/sysctl commit checks. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Does not modify data-plane mismatch validation at start, commit, reload, or validate. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Does not touch VPP firewall verification or YANG backend annotations. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Does not touch data-plane selector naming or backend registrations. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Does not touch BGP export chains or next-hop rewriting. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Does not touch `policychain` or next-hop policy authoring. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Does not touch BGP filter-chain compilation or `FilterInsertion`. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Does not touch BGP UPDATE forwarding or raw override provenance. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Does not touch IPv4/IPv6 next-hop validation or policy import/export checks. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Does not touch forwarded or locally originated BGP route next hops. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Does not touch implied filter-chain publication or `\| implied` display. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 on `core-design.md` | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Makes no service failure stop `ze start`, preserves reload refusal semantics, and follows the doctor-code + fix reporting contract for every new `Warn` line under `internal/component/plugin/`. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 on `core-design.md` | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Does not touch kernel/VPP startup gating or `ze doctor` mismatch reporting. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | decision of 2026-11-23 on `tls-listeners.md` | "The looking glass may serve plain HTTP, because what it publishes is public, and one looking glass serves HTTP and HTTPS at the same time. Thomas's words, as voice-transcribed: "We should allow the looking glass to be HTTP. The information is public." and "We should be able to serve on both HTTP and HTTPS, not either." Asked where the choice lives and what happens to `ze.looking-glass.tls`, he answered "1a 2a": the choice is the `tls` leaf of each `server` entry, default true, and the environment variable is removed. The no-store rule applies to each entry: an entry that left `tls` unset serves HTTP with a warning on a daemon with no `database/` store and no named certificate, an entry that says `tls true` there is refused at start and at reload, and a reload does what a restart with the same config does" | agrees | Does not touch looking-glass HTTP/HTTPS listener configuration or startup. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | decision of 2026-11-23 on `tls-listeners.md` | "The web and looking glass use the shared `listenerTLSMaterial` precedence. The approved Fleet reload design shares `managedCertificate` with startup because the daemon authority issues its unnamed leaf." | agrees | Does not modify `managedCertificate` or `listenerTLSMaterial` precedence. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | decision of 2026-11-26 on `tls-listeners.md` | "A certificate that cannot be presented never stops ze from starting. A web server whose certificate name does not resolve is not started. The looking glass is judged one `server` entry at a time: an entry that says `tls true` with no certificate to present stays down, every entry that asks for TLS stays down when the named certificate does not resolve, and the other entries serve. Each is reported with its fix by an error line in the log under a doctor code and by `ze doctor`. A reload whose config cannot be served stays refused. This supersedes the start half of the row of 2026-11-23 above, which says a `tls true` entry "is refused at start": at start that entry stays down and the others serve, and at reload the config is still refused. Thomas's words, as voice-transcribed: "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; "reload stay refused"; "Should only that endpoint stay down while the others serve: yes"" | agrees | Does not modify TLS certificate startup or reload behavior on web, looking glass, or managed listeners. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A failed send, signal, deadline set, close, abort, or wait inside the plugin process manager, the plugin server, the managed config server, the public RPC transport, or the SDK.

### Transformation Path
1. The branch catches the error.
2. When the condition is benign teardown or an already-closed/stopping session (or `syscall.ESRCH` on an exited process group), the branch returns or logs at Debug under a `// refuse-by-default: guards <Symbol> -- <why>` annotation naming the supervision/teardown guard.
3. When the condition leaves a live plugin, managed client, stderr drain, or process group degraded, the branch logs at `WARN` with what, why, and the fix, and under `internal/component/plugin/` attaches its registered doctor `code` attribute and raises a transient error event on the report bus (`internal/core/report`).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ plugin | RPC error/result replies over `PluginConn` and `MuxConn`; SDK stderr relayed via `relayStderrFrom` | No |
| Hub ↔ managed edge | `pkg/fleet` `config-fetch` reply over `MuxConn` | No |
| Plugin infra ↔ `ze doctor` | `diagnostic.Register`, `diagnostic.RegisterEventCode`, and `report.RaiseErrorFix` / `raiseServerEvent` | No |

### Changed-Wire Census

No wire format changes. **Search evidence:** inspected all 16 remaining `error-dropped-below-warn` sites returned by `./le arch refuse-by-default report '|' json` under `internal/component/plugin/`, `pkg/plugin/`, and `pkg/ze/` (control: `pkg/plugin/rpc/conn.go` `SendResult`/`SendError` and `internal/component/plugin/server/managed_serve.go` `handleRequest`); every edit changes only local error logging, report-bus raises, doctor code metadata/event-code registrations, or `refuse-by-default` guard comments, preserving every RPC method name, JSON field tag, frame format, and exported `pkg/` signature unchanged.

### Integration Points
- `internal/component/plugin/process` logger, `doctor_codes.go`, `register.go` (`processEventCodes`), and `internal/core/report`
- `internal/component/plugin/server` logger, `managedLogger`, `doctor_codes.go`, `register.go` (`serverEventCodes`), `server_events.go` (`raiseServerEvent`), and `dispatch_report.go` (`refusePluginRequest`)
- `internal/component/plugin/doctor/doctor_codes_test.go` (`TestPluginInfraDoctorCodesDescribed`)
- `pkg/plugin/sdk` `slog.Warn` (relayed by `relayStderrFrom` at `WARN`)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Uses existing `slogutil` loggers, `internal/core/report` bus, and `diagnostic` registries |
| No unintended coupling (components stay isolated) | Yes | No new cross-component imports; `internal/component/plugin/` still imports nothing from `internal/component/bgp/`; `pkg/plugin/` still imports nothing from `internal/core/diagnostic` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Extends `CodeDoctorPluginAnswerUndelivered`, `CodeDoctorPluginRequestRefused`, `raiseServerEvent`, `doctor_codes.go`, and `processEventCodes`/`serverEventCodes` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Error-handling control paths only |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | New doctor codes self-register from owning package `init()` via `diagnostic.Register` and `diagnostic.RegisterEventCode` |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `internal/component/plugin/` and `internal/le/arch/doctorcoverage/`; `recent-events` doctor check discovers registered `EventCode` values via `diagnostic.EventCodes()` and `doctor-coverage` scans `diagnostic.EventCode` literals directly; only the metadata test list in `doctor_codes_test.go` enumerates plugin-infra codes for `ze show diagnostic` verification |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 16 instances across 7 files are the complete HEAD population under `internal/component/plugin/`, `pkg/plugin/`, and `pkg/ze/` | `./le arch refuse-by-default report '|' json` run on 2026-11-26 | Work from the fresh report if any commit landed before implementation starts | Re-run `./le arch refuse-by-default report '|' json` filtered to this child's paths before the first edit | confirmed |
| A-2 | Live-connection reply send failures (marshal error, `MaxMessageSize` overflow, write deadline) are distinguishable from closed/stopping sessions via `proc.PluginEnded()`, `s.Context().Err()`, `loopCtx.Err()`, `isConnectionClosed`, `net.ErrClosed`, `io.ErrClosedPipe`, and `rpc.ErrMuxConnClosed` | Read `pkg/plugin/rpc/conn.go` (`SendResult`, `writeFrame`), `mux.go` (`MuxConn.Close`), `process.go` (`PluginEnded`), and `sdk_dispatch.go` (`isConnectionClosed`) | A closed-peer reply emits a noisy WARN or a live-peer failure is silently annotated | Unit tests exercising both a live-connection send failure (must WARN) and a closed/stopping session (must not WARN) | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A change in `pkg/` alters an exported contract | `./le plugin declarations check` or package tests break | No exported signature, field, or type changes in `pkg/plugin/rpc` or `pkg/plugin/sdk` |
| R-2 | A WARN per failed RPC reply or per `stderrRelayReader.Read` floods logs and overwrites the report error ring | Repeated test calls emit unbounded WARN lines or report events | Guard `stderrRelayReader.extendDeadline` with `atomic.Bool` (warn once per reader); rate-limit server `CodeDoctorPluginAnswerUndelivered` raises per plugin with `report.RaiseErrorFixOnce` (matching `refusePluginRequest`) |
| R-3 | A new `Warn` line under `internal/component/plugin/` omits a covered doctor `code` or lacks `ze show diagnostic` metadata | `./le arch doctor-coverage check` or `TestPluginInfraDoctorCodesDescribed` fails | Register every new code in `doctor_codes.go` + `register.go` (`EventCode`) + `doctor_codes_test.go`, raise on the report bus, and run `./le arch doctor-coverage check` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Spurious WARN/doctor noise on normal plugin shutdown, or a missed failure trace when a live reply/signal/stderr-deadline fails |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Active work in `internal/component/plugin/server` or `process` touching `dispatch.go`, `request_runtime.go`, `managed_serve.go`, or `process.go` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default report` over `internal/component/plugin/`, `pkg/plugin/`, `pkg/ze/` | → | `droppedError` (`internal/le/arch/refusebydefault/dropped.go`) | Report filtered to this child's paths produces zero `error-dropped-below-warn` findings |
| Non-`ESRCH` failure signalling a plugin's process group | → | `pluginGroup.signalLocked` (`internal/component/plugin/process/process.go:1378`) | `TestPluginGroupSignalFailureLogsWarnAndRaisesDoctorCode` (`internal/component/plugin/process/process_signal_test.go`) |
| Failure arming post-reap stderr drain deadline or closing parent stderr write end | → | `stderrRelayReader.extendDeadline` and `closeParentStderrWrite` (`internal/component/plugin/process/process.go:939,1068`) | `TestStderrDrainUnboundedLogsWarnAndRaisesDoctorCode` (`internal/component/plugin/process/stderr_relay_test.go`) |
| Failed RPC error/result send to a live plugin | → | `Server.dispatchPluginRPCContext`, `Server.handleCodecRPC`, `Server.serveRuntimeRequests` via `noteReplyDeliveryFailure` (`dispatch.go:115,133,669,675`, `request_runtime.go:44`) | `TestDispatchReplyFailureOnLivePluginLogsWarnWithDoctorCode` (`internal/component/plugin/server/dispatch_reply_warn_test.go`) |
| Non-closed failure aborting a request-ID-collided runtime connection | → | `Server.serveRuntimeRequests` (`internal/component/plugin/server/request_runtime.go:37`) | `TestServeRuntimeRequestsCollisionAbortErrorLogsWarnWithDoctorCode` (`internal/component/plugin/server/dispatch_reply_warn_test.go`) |
| Failed `config-fetch` `SendResult` to a live connected managed client | → | `ManagedServer.handleRequest` (`internal/component/plugin/server/managed_serve.go:737`) | `TestManagedConfigFetchSendFailureLogsWarnWithDoctorCode` (`internal/component/plugin/server/managed_serve_test.go`) |
| Failed async callback answer send on a live SDK connection | → | `Plugin.answerAsyncCallback` (`pkg/plugin/sdk/sdk_async_callback.go:85`) | `TestAnswerAsyncCallbackLiveSendFailureLogsWarn` (`pkg/plugin/sdk/sdk_async_callback_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `./le arch refuse-by-default report '|' json` filtered to `internal/component/plugin/`, `pkg/plugin/`, and `pkg/ze/` | Report completes cleanly with zero unannotated `findings` entries carrying rule `error-dropped-below-warn`, while every deliberate teardown/closed-session drop appears in `annotated` with a resolved `guard` symbol | Guard-annotated teardown/closed-session branches listed in the Current Behavior table (items 1, 3 closed branch, 4, 5 `ESRCH` branch, 6-12 closed/stopping branches, 13, 14, 15, 16 closed/stopping branch) remain permitted because supervision or shutdown already handles those exits and nobody can act on them |
| AC-2 | Every new or modified `Warn` log site under `internal/component/plugin/` (`process.go`, `dispatch.go`, `request_runtime.go`, `managed_serve.go`) | Emits a `WARN` record naming the plugin or managed client, what failed, why it matters, the exact fix, and a registered doctor `code` attribute, AND raises a report-bus event under that same code; `./le arch doctor-coverage check` passes with zero uncovered sites and `TestPluginInfraDoctorCodesDescribed` passes | None: every `Warn` under `internal/component/plugin/` must pass `doctor-coverage` and carry `ze show diagnostic` metadata |
| AC-3 | Unit tests for live-connection failures vs closed/stopping sessions across `process`, `server`, and `pkg/plugin/sdk` | Live-connection failures emit the expected `WARN` line (with doctor code and report-bus event under `internal/component/plugin/`, and `slog.Warn` with plugin/ID/why/fix in `pkg/plugin/sdk`), while closed/stopping sessions and `syscall.ESRCH` emit zero spurious `WARN` lines and zero report-bus events | None: both live-failure and closed/stopping paths are proven by unit tests |
| AC-4 | Every `// refuse-by-default: guards <Symbol> -- <why>` annotation added by this spec | Passes `./le arch refuse-by-default check` naming a visible non-enclosing symbol that actually owns the guarded lifecycle/supervision | None: invalid or self-naming annotations are refused by `annotation.go` |
| AC-5 | Scoped main-thread verification: `./le job run label plugin command go test -race ./internal/component/plugin/... ./pkg/plugin/...`, `./le plugin declarations check`, `./le arch refuse-by-default check`, `./le arch doctor-coverage check`, and the plugin/managed `.ci` suites reached by `./le repo changed scope print both` | All scoped unit tests, architecture checks, declaration checks, and reached `.ci` tests pass green | None: every scoped check in the changed surface must pass |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPluginGroupSignalFailureLogsWarnAndRaisesDoctorCode` | `internal/component/plugin/process/process_signal_test.go` | AC-2, AC-3 (non-`ESRCH` warns with `CodeDoctorPluginGroupSignalFailed` and raises report event; `ESRCH` is silent) | |
| `TestStderrDrainUnboundedLogsWarnAndRaisesDoctorCode` | `internal/component/plugin/process/stderr_relay_test.go` | AC-2, AC-3 (`SetReadDeadline` non-closed error warns once per reader with `CodeDoctorPluginStderrDrainUnbounded` and raises event; `os.ErrClosed` stays silent; `closeParentStderrWrite` error warns and raises event) | |
| `TestDispatchReplyFailureOnLivePluginLogsWarnWithDoctorCode` | `internal/component/plugin/server/dispatch_reply_warn_test.go` | AC-2, AC-3 (live `SendError`/`SendResult` failure warns with `CodeDoctorPluginAnswerUndelivered` and raises rate-limited event; ended/stopping plugin stays silent) | |
| `TestServeRuntimeRequestsCollisionAbortErrorLogsWarnWithDoctorCode` | `internal/component/plugin/server/dispatch_reply_warn_test.go` | AC-2, AC-3 (non-closed `AbortRuntime` error on ID collision warns with `CodeDoctorPluginRequestRefused` and raises event; `net.ErrClosed` stays silent) | |
| `TestManagedConfigFetchSendFailureLogsWarnWithDoctorCode` | `internal/component/plugin/server/managed_serve_test.go` | AC-2, AC-3 (live `config-fetch` `SendResult` failure warns with `CodeDoctorManagedConfigFetchUndelivered` and raises event naming client, why and fix; closed/stopping client stays silent) | |
| `TestPluginInfraDoctorCodesDescribed` | `internal/component/plugin/doctor/doctor_codes_test.go` | AC-2 (`CodeDoctorPluginGroupSignalFailed`, `CodeDoctorPluginStderrDrainUnbounded`, `CodeDoctorManagedConfigFetchUndelivered` carry title, description, examples) | |
| `TestAnswerAsyncCallbackLiveSendFailureLogsWarn` | `pkg/plugin/sdk/sdk_async_callback_test.go` | AC-3 (live send failure logs `slog.Warn` naming plugin, request ID, why and fix; stopped loop or closed connection stays silent) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Unannotated `error-dropped-below-warn` findings under this child's paths | 0 | 0 | N/A | 1 |
| Uncovered `doctor-coverage` sites added by this change | 0 | 0 | N/A | 1 |
| `extendDeadline` WARN count across repeated reads on a non-deadline fd | 1 | 1 | 0 | 2 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Plugin and managed `.ci` suites reached by `./le repo changed scope print both` | `test/plugin/`, `test/managed/` | Normal plugin startup, dispatch, shutdown, and managed client config sync run with zero spurious WARN lines | |

## Files to Modify
- `internal/component/plugin/process/process.go` - 5 sites: `Process.startInternal.func.func` 750; `Process.startChildCmd` 939; `stderrRelayReader.extendDeadline` 1068; `drainStderrRelay` 1100; `pluginGroup.signalLocked` 1378
- `internal/component/plugin/process/doctor_codes.go` - register `CodeDoctorPluginGroupSignalFailed` and `CodeDoctorPluginStderrDrainUnbounded`
- `internal/component/plugin/process/register.go` - add `CodeDoctorPluginGroupSignalFailed` and `CodeDoctorPluginStderrDrainUnbounded` to `processEventCodes`
- `internal/component/plugin/server/dispatch.go` - 4 sites: `Server.dispatchPluginRPCContext.func` 115; `Server.dispatchPluginRPCContext` 133; `Server.handleCodecRPC` 669, 675; shared `noteReplyDeliveryFailure` helper
- `internal/component/plugin/server/request_runtime.go` - 2 sites: `Server.serveRuntimeRequests` 37 (`AbortRuntime`); `Server.serveRuntimeRequests.func` 44 (`SendError`)
- `internal/component/plugin/server/managed_serve.go` - 1 remaining site: `ManagedServer.handleRequest` 737 (`config-fetch` `SendResult`; `pushConfigChanged` is already fixed at HEAD)
- `internal/component/plugin/server/doctor_codes.go` - register `CodeDoctorManagedConfigFetchUndelivered`
- `internal/component/plugin/server/register.go` - add `CodeDoctorManagedConfigFetchUndelivered` to `serverEventCodes`
- `internal/component/plugin/server/startup_removal.go` - 1 site: `Server.stopRemovedProcess` 232
- `internal/component/plugin/doctor/doctor_codes_test.go` - verify `ze show diagnostic` metadata for the 3 new doctor codes
- `pkg/plugin/rpc/panic_guard.go` - 2 sites: `Conn.endAfterPanic` 72; `MuxConn.endAfterPanic` 85
- `pkg/plugin/sdk/sdk_async_callback.go` - 1 site: `Plugin.answerAsyncCallback` 85
- `docs/architecture/doctor-and-health-checks.md` - document the new plugin process/server event codes in `### Event codes`
- `docs/architecture/fleet-config.md` - document `doctor-managed-config-fetch-undelivered` in `### What doctor reads from the managed listener`

Lines are those of HEAD on 2026-11-26; re-run `./le arch refuse-by-default report` before the first edit if lines drift.

## Files to Create
- `internal/component/plugin/process/process_signal_test.go` - unit tests for `pluginGroup.signalLocked` (`ESRCH` vs non-`ESRCH` WARN + doctor code + report-bus raise)
- `internal/component/plugin/server/dispatch_reply_warn_test.go` - unit tests for live vs closed/stopping RPC reply send failures and `AbortRuntime` collision close failures
- `pkg/plugin/sdk/sdk_async_callback_test.go` - unit tests for `Plugin.answerAsyncCallback` live send failure WARN vs stopped/closed silence
- Additional test cases in existing `internal/component/plugin/process/stderr_relay_test.go` and `internal/component/plugin/server/managed_serve_test.go`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config or RPC added |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No command |
| Functional test for new RPC/API | N-A | No new RPC |
| Pipe completeness | N-A | No output change |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Doctor code per problem class | Yes | Existing `CodeDoctorPluginAnswerUndelivered` and `diagnostic.CodeDoctorPluginRequestRefused` (`internal/component/plugin/server/doctor_codes.go`, `register.go`) plus 3 new event codes: `CodeDoctorPluginGroupSignalFailed` and `CodeDoctorPluginStderrDrainUnbounded` (`internal/component/plugin/process/doctor_codes.go`, `register.go`), and `CodeDoctorManagedConfigFetchUndelivered` (`internal/component/plugin/server/doctor_codes.go`, `register.go`), all reported by the runtime `recent-events` doctor check via `diagnostic.RegisterEventCode` and raised on `internal/core/report` |
| Prometheus counters/metrics | N-A | No new counter planned |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Error-visibility fix only |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | No exported SDK or wire protocol change |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared `// Design:` docs: `docs/architecture/api/process-protocol.md` (unaffected: wire protocol, shutdown, and panic containment unchanged), `docs/architecture/doctor-and-health-checks.md` (updated: `### Event codes` lists the new event codes), `docs/architecture/fleet-config.md` (updated: `### What doctor reads from the managed listener` lists `doctor-managed-config-fetch-undelivered`). Confirm anchors with `./le spec citation anchors spec plan/release/fixes/spec-dropped-error-7-plugin-infra.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Design presentation

### What changes

Every one of the 16 remaining `error-dropped-below-warn` sites across `internal/component/plugin/`, `pkg/plugin/rpc/`, and `pkg/plugin/sdk/` is resolved cleanly so the default `WARN` log level and `ze show doctor` surface genuine failures without emitting noise on normal shutdowns. Specifically, failures that leave a live peer or host resource degraded (non-`ESRCH` failure signalling a plugin's process group, failure closing the parent stderr write end or arming the post-reap stderr drain deadline, failed RPC replies to a live plugin, failed `AbortRuntime` close on a request-ID collision, failed `config-fetch` replies to a live managed client, and failed SDK async callback answers on a live connection) now log at `WARN` with what failed, why it matters, and the exact next step; every site under `internal/component/plugin/` emits a registered doctor `code` and raises a report-bus event so `ze show doctor` and `ze show errors` report it. Benign teardown closes, `syscall.ESRCH` on an already-exited process group, and replies to already-closed or stopping sessions carry explicit `// refuse-by-default: guards <Symbol> -- <why>` annotations naming the supervision or teardown path that handles them. No exported `pkg/` API, wire frame, or startup-tier failure ruling changes.

### Before and after

1. **Plugin process group cannot be signalled (`pluginGroup.signalLocked`, `process.go:1378`):**
   - Before: a non-`ESRCH` failure such as `EPERM` from `KillProcessGroup` writes only a `DEBUG` line (`signal a plugin's process group`), invisible at default `WARN` level while the plugin or its children keep running.
   - After: `ESRCH` stays quiet because the group is already gone; any other error logs at `WARN` with `"code", "doctor-plugin-group-signal-failed"`, naming the plugin, what failed, why (the plugin process group could not be signalled with SIGKILL so child processes may remain running), and the next step (`ze show log recent component plugin.relay` / `ze show errors`), and raises the event on the report bus so `ze show doctor` reports it.
2. **Post-reap stderr drain deadline or parent write-end close fails (`process.go:939, 1068`):**
   - Before: `stderrWrite.Close()` and `r.file.SetReadDeadline(...)` log only at `DEBUG`, hiding a leaked write fd or an unarmed idle drain bound that can stall stop/restart behind a lingering grandchild.
   - After: `os.ErrClosed` on `extendDeadline` stays silent as a teardown race; any other failure logs at `WARN` (at most once per reader for `extendDeadline`) with `"code", "doctor-plugin-stderr-drain-unbounded"`, naming the plugin, why, and the next step, and raises on the report bus.
3. **RPC reply cannot be sent to a live plugin (`dispatch.go:115,133,669,675`, `request_runtime.go:44`):**
   - Before: `SendError` and `SendResult` failures (such as marshal error, `MaxMessageSize` overflow, or write deadline) log only at `DEBUG` even when the plugin connection is still live and waiting for its reply.
   - After: if the server is stopping or `proc.PluginEnded()` is true, the drop is annotated against `handleSingleProcessCommandsRPC` (no spurious WARN on shutdown); if the plugin is still live, the server logs at `WARN` with `"code", "doctor-plugin-answer-undelivered"` naming the plugin, method, why, and fix, and raises a rate-limited event on the report bus.
4. **Managed client `config-fetch` answer cannot be sent (`managed_serve.go:737`):**
   - Before: `mc.SendResult` failure logs only at `DEBUG`, leaving no trace at default level when a live managed client does not receive its fetched configuration (meanwhile `pushConfigChanged` is already fixed at HEAD via `notifyDropped`).
   - After: if the hub is stopping or the client connection is already closed, the drop is annotated against `ManagedServer.serve`; if the client is still connected, the hub logs at `WARN` with `"code", "doctor-managed-config-fetch-undelivered"` naming the managed client, why, and fix, and raises on the report bus.
5. **SDK async callback answer cannot be sent on a live connection (`sdk_async_callback.go:85`):**
   - Before: `sendCallbackError`/`sendCallbackResult` failure logs only at `slog.Debug`, hiding a live-connection send failure (deadline or oversize receipt) while the engine waits for the callback receipt.
   - After: if the event loop stopped or the connection is closed, the drop is annotated against `Plugin.eventLoop`; if the connection is still live, the SDK logs at `slog.Warn` naming the plugin, request ID, why, and fix (relayed by `relayStderrFrom` at `WARN`).

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Annotate every reply-send site as Answer 3 assuming the connection is always closed | Disproven by `pkg/plugin/rpc/conn.go` (`SendResult`/`writeFrame`): marshal failures, `MaxMessageSize` overflows, and write deadlines fail on a live connection, leaving the peer waiting without its reply |
| Raise every one of the 16 sites unconditionally to `WARN` | Teardown closes (`bridgedPluginSide.Close`, `drainStderrRelay`, `endAfterPanic`), `ESRCH` on an already-exited group, and replies after a peer closed or during shutdown would flood logs and `ze show doctor` with false warnings |
| Change exported `pkg/plugin/rpc` or `pkg/plugin/sdk` signatures to return new errors | Violates `ai/doors/system.md` and `ai/doors/plugin-infra.md`: `pkg/` is a published external-plugin contract |
| Log `WARN` under `internal/component/plugin/` without a doctor code or report-bus raise | Refused by `./le arch doctor-coverage check` (`./le commit create`) and violates `docs/architecture/doctor-and-health-checks.md` ("every site carrying a registered event code raises it") |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- re-run `./le arch refuse-by-default report '|' json` filtered to this child's paths to confirm the 16-instance population; register the 3 new doctor codes (`CodeDoctorPluginGroupSignalFailed`, `CodeDoctorPluginStderrDrainUnbounded`, `CodeDoctorManagedConfigFetchUndelivered`) in `process/doctor_codes.go`, `process/register.go`, `server/doctor_codes.go`, `server/register.go`, and `doctor/doctor_codes_test.go`; write the failing wiring tests from the Wiring Test table.
2. **Phase: Process manager (`process.go`)** -- resolve all 5 instances in `internal/component/plugin/process/process.go` (Answer 2 with doctor codes + report-bus raises for `signalLocked` non-`ESRCH`, `startChildCmd` `stderrWrite.Close`, and `extendDeadline` non-`ErrClosed` once per reader; Answer 3 annotations for `startInternal` pipe close and `drainStderrRelay` read-end close). Prove `process_signal_test.go` and `stderr_relay_test.go` fail first, then pass.
3. **Phase: Plugin server & managed server (`dispatch.go`, `request_runtime.go`, `managed_serve.go`, `startup_removal.go`)** -- add the shared `noteReplyDeliveryFailure` helper in `dispatch.go` covering `dispatch.go:115,133,669,675` and `request_runtime.go:44`; resolve `request_runtime.go:37` (`AbortRuntime` non-closed WARN with `CodeDoctorPluginRequestRefused`), `managed_serve.go:737` (live `config-fetch` send WARN with `CodeDoctorManagedConfigFetchUndelivered`), and `startup_removal.go:232` (Answer 3 `guards Server.Stop`). Prove `dispatch_reply_warn_test.go` and `managed_serve_test.go` fail first, then pass.
4. **Phase: Public RPC & SDK (`panic_guard.go`, `sdk_async_callback.go`)** -- add Answer 3 annotations to `Conn.endAfterPanic` and `MuxConn.endAfterPanic` (`guards Conn.readLoop` / `guards MuxConn.readLoop`), and update `Plugin.answerAsyncCallback` to distinguish stopped/closed connections (Answer 3 `guards Plugin.eventLoop`) from live send failures (`slog.Warn` naming plugin, ID, why, fix). Prove `sdk_async_callback_test.go` fails first, then pass.
5. **Phase: Docs and Proof** -- update `docs/architecture/doctor-and-health-checks.md` (`### Event codes`) and `docs/architecture/fleet-config.md` (`### What doctor reads from the managed listener`); verify AC-1 through AC-5 with scoped main-thread checks.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every one of the 16 HEAD instances has its verified answer at file:line and zero findings remain in `./le arch refuse-by-default report` under this child's paths |
| Correctness | No exported `pkg/` change; no failure made survivable against the startup-tier ruling; `ESRCH` and closed/stopping sessions never emit spurious WARN lines |
| Operator errors and doctor codes | Every new `Warn` under `internal/component/plugin/` names the plugin or managed client, what failed, why it matters, the exact fix, and a registered doctor `code` attribute, raises on the report bus under that code, and passes `./le arch doctor-coverage check` and `TestPluginInfraDoctorCodesDescribed` |
| Flood protection | `stderrRelayReader.extendDeadline` warns at most once per reader; `CodeDoctorPluginAnswerUndelivered` report raises are rate-limited per plugin |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero findings under this child's paths | `./le arch refuse-by-default report '|' json` filtered to `internal/component/plugin/`, `pkg/plugin/`, `pkg/ze/` and `./le arch refuse-by-default check` |
| Doctor-coverage clean | `./le arch doctor-coverage check` |
| Scoped package tests | `./le job run label plugin command go test -race ./internal/component/plugin/... ./pkg/plugin/...` |
| Declarations and doc anchors | `./le plugin declarations check`, `./le doc check all`, and `./le spec citation anchors spec plan/release/fixes/spec-dropped-error-7-plugin-infra.md` |
| Scoped functional tests | `./le repo changed scope print both` followed by `./le test ...` for the reached plugin and managed `.ci` suites (run by main thread in the main checkout; GitHub CI verifies the pushed commit) |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Error leakage | No `WARN` line or report-bus event prints the plugin hub token, managed client secret, or CA private key material |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint or `doctor-coverage` failure | Fix inline (register/emit doctor code and raise event). If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Review Gate

Pending implementation: `/ze-review` and `/ze-review-spec` run in an independent context after implementation and before closure, with the clean review artifact recorded via `./le spec review record`.

## Design Insights

- 2026-11-26 HEAD audit: 4 of the original 20 instances (2026-11-02) were already resolved or consolidated on `main` (`ManagedServer.pushConfigChanged` via `notifyDropped`, `dispatch_registry.go` via `answerEngineOpJSON`, and `sdk_dispatch.go`/`sdk_source_policy.go` consolidated into `sdk_async_callback.go`), while 2 new instances landed in `request_runtime.go`, bringing the exact HEAD population to **16 instances across 7 files**.
- `pkg/plugin/rpc/conn.go` (`SendResult`, `writeFrame`) proved that reply send failures can occur on a live connection (marshal error, `MaxMessageSize` overflow, write deadline), refuting the original assumption A-2 that every failed reply means the peer already disconnected.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

The class row for the whole population is owned by the umbrella.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| No exported `pkg/` change | Return new errors from SDK methods | A changed signature breaks external plugins outside this repository (`ai/doors/system.md`, `ai/doors/plugin-infra.md`) |
| The umbrella's three answers, in order, splitting live-peer failures from closed/stopping sessions | Annotate all reply sends as Answer 3, or raise every line to WARN | `writeFrame`/`SendResult` can fail on live connections (leaving peers waiting), while closed/stopping sessions and `ESRCH` are benign and would flood logs if warned |
| Register dedicated doctor event codes in owning `doctor_codes.go` + `register.go` and raise on `internal/core/report` for every new `Warn` under `internal/component/plugin/` | Plain `Warn` without doctor code, or reusing unrelated codes | Required by `./le arch doctor-coverage check` (`./le commit create`) and `docs/architecture/doctor-and-health-checks.md` ("every site carrying a registered event code raises it") |

## Known Limitations
- None.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] AC-N rows carry testable assertions and positive observables with Permitted Survivors
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, `./le plugin declarations check`, `./le arch refuse-by-default check`, `./le arch doctor-coverage check`, and the functional suites `./le repo changed scope print both` reaches (GitHub CI verifies the pushed commit)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason): N-A, no wire change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-dropped-error-7-plugin-infra.md` only, in the same `./le commit create` script
