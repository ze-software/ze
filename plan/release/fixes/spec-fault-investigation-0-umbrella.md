# Spec: fault-investigation-0-umbrella

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Priority | P1 |
| Release reason | A production NOS must leave an investigable record of every fault it suffers. Today a process killed by SIGKILL or the OOM killer is filed as a clean exit and leaves nothing, and a hang leaves nothing, so field failures cannot be diagnosed. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

| Topic | Decision |
|-------|----------|
| AC-22 stock Debian/Ubuntu test-VM exception | Extend the Debian/Ubuntu test-VM exception in `ai/META-DECISIONS.md` (currently limited to `modprobe` kernel-module tests) so AC-22 fault-recorder tests can run on real booted Debian/Ubuntu VMs under systemd (`systemd-pstore` and stock kernels without reserved ramoops RAM). |

## Where this stands (2026-11-14)

The umbrella owns no code. None of the six child specs exists yet. The next step is child 1, through `/ze-spec` in its own session. The facts below were re-read at the producer on 2026-11-14.

### What landed since 2026-10-25

| What | Where | Effect on this umbrella |
|------|-------|-------------------------|
| A shared panic boundary for network-input tasks: one ERROR log line, one increment of `ze_task_panics_total{component,task}`, and a `crash-<time>-recovered.log` record | `internal/core/taskguard/guard.go` `Task.Contain`, `Guard.record`; `internal/core/crashlog/recovered.go` `RecordRecoveredPanic` | Child 4's shared helper exists. Child 4 still owes dedup by stack signature and a rate limit (`RecordRecoveredPanic` has neither), the recovery sites that do not go through `Task.Contain`, and the silent bfd site (`trySendStateChange` in `internal/component/bfd/engine/engine.go`, whose recover sets `sent = false` and logs nothing). Counted today outside tests under `internal`, `cmd` and `pkg`: 57 lines with `recover()` (56 calls and one comment; one call is the boundary's own, so 55 sites recover on their own), and 63 lines with `.Contain(` (62 boundary uses and the doc comment in `guard.go`) |
| Recovered records are a third artifact kind and rotate apart from process crashes | `recovered.go` `rotateRecoveredFiles`; `internal/core/crashlog/persist.go` `rotateCrashFiles` skips recovered records only | Panic and kernel records still share `ze.crash.keep`, so R-4 and AC-10 (kernel record evicted by Go panics) still stand |
| Every report prints the panic value as `fmt` prints it | `internal/core/crashlog/crashlog.go` `appendValue` is now `fmt.Append` | The "non-string panic values lost" gap is closed |
| The log ring keeps attributes | `internal/core/slogutil/ring.go` `LogEntry.Attributes` | `crashlog.go` `buildCrashReport` still prints only time, level, component and message, so child 1's ring work shrinks to printing the attributes in the report |
| A plugin child's exit is recorded, and the crash report carries it | `internal/component/plugin/process/process.go` `p.exit.record` (in `monitorCmd` for a fork, after the runner for an internal plugin); `stopped.go` `ExitStatus`; `manager.go` `ReportCrash` (raises `doctor-plugin-crashed` with the status) | With relay off the child's stderr is still discarded (`relayStderrFrom`), and the host writes no crash file for a forked child. An internal plugin's panic is contained by `runnerPanics().run.Contain` and recorded. Child 3 starts from a recorded exit status |
| `request support archive` is a local-data command | `internal/plugins/support/register.go` `command.MustRegisterLocalData` | The daemon serves it on every surface, so the "offline commands unreachable over MCP" row is true only for offline `show crashes` with a dead daemon |

### AC ownership

Each AC has one owner, taken from the Wiring Test table and the Implementation Steps. Where two children are involved, the owner is the child whose test the Wiring Test table names.

| AC | Owner | Depends on |
|----|-------|------------|
| AC-1 | child 1 | - |
| AC-2 | child 1 | - |
| AC-3 | child 1 | - |
| AC-4 | child 2 (killed and oom kinds) | child 1 (the recorder's `wait`) |
| AC-5 | child 2 (`crashlog` fault record on plugin-fatal stop; exit 1 and `LastStopEntry` already landed) | child 1 (the recorder restarts the daemon) |
| AC-6 | child 3 | - |
| AC-7 | child 4 | - |
| AC-8 | child 5 | - |
| AC-9 | child 6 | - |
| AC-10 | child 1 (per-kind retention) | - |
| AC-11 | child 1 (sanitized answers) | - |
| AC-12 | child 5 (`TestProberFrozenDaemonDumped`) | child 1 (the recorder is the prober) |
| AC-13 | child 6 | - |
| AC-14 | child 1 (pmsg) | - |
| AC-15 | child 1 (bounded queue, drop count) | - |
| AC-16 | child 1 (pre-opened file fallback) | - |
| AC-17 | child 1 | - |
| AC-18 | child 1 (enqueue path and its allocation test, R-15) | children 4, 5 and 6 (the records) |
| AC-19 | child 1 (fault reader) | - |
| AC-20 | child 1 (crash loop) | - |
| AC-21 | child 1 (recorder log ring) | - |
| AC-22 | child 1's design gate (crosses every child; Debian/Ubuntu VM proof approved 2026-11-24) | every child |
| AC-23 | child 2 (`TestHarvestKilledProcessRecorded`) | child 1 (the recorder's `wait`) |

### Open for child 1's design gate

These are not decisions. Each is a question the child-1 spec must answer.

- How the recorder receives the ze log stream.
- What "stack signature" means.
- The record grammar, and the section names used for paging.
- Where the port setting of R-20 lives when the recorder parses no config.
- Whether `ze start` stays one binary that forks the recorder core and the fault reader.
- Where the credential store lives on a Debian or Ubuntu host.
- Whether the fault reader's listing and single-record answers must have the same shape as `show faults`; no AC says so today.
- `internal/core/taskguard/guard.go` names the command `ze show crashes` in the constant `showCrashesCommand`, so the rename to `show faults` has that consumer too.
- Who owns AC-22, which crosses every child.

### Decisions that are Thomas's

None pending. The one question previously held here — whether AC-22, R-18 and R-19 may run on stock Debian and Ubuntu VMs under systemd (`systemd-pstore` and stock kernels without reserved ramoops RAM) despite the standing test-guest rule in `ai/META-DECISIONS-SITUATIONAL.md` ("Test systems and labs") — was decided by Thomas on 2026-11-24 (recorded in `## Owner decisions (2026-11-24)`): extend the existing `modprobe` test-VM exception so AC-22 fault-recorder tests run on booted Debian and Ubuntu VMs under systemd.

### Specs that overlap

| Spec | Status | Overlap |
|------|--------|---------|
| `plan/release/fixes/spec-crash-capture.md` | blocked | Kernel harvest and its OTA proof |
| `plan/release/validation/spec-doctor-coverage-14-system-plugins.md` | ready | Adds doctor checks for the `crashes` plugin |
| `plan/post-release/features/spec-crash-capture-memory-image.md` | skeleton | kdump, already in Known Limitations |
| `plan/release/validation/spec-plugin-smoke-run.md` | ready | Owns the journal row `recovered-panic-loses-its-value` (2026-11-01), and its D-3 moves the recover sites outside `taskguard` onto it: overlaps child 4 |
| `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md` | ready | Plugin exit, `ReportCrash` and the failure policy: children 2 and 3 |
| `plan/release/fixes/spec-daemon-drain-before-stop.md` | ready | The daemon stop path: child 2 |

## Task

Every fault Ze can suffer is recorded with enough context that an AI connected
over MCP, or a human over the CLI, can investigate it after the fact. Builds on
the existing crash capture (`docs/architecture/diagnostics/crash-capture.md`),
which already records Go runtime fatals, unrecovered panics and kernel panics.

Shape agreed at the SCOPE gate (2026-10-24): an umbrella with children, in the
release bucket at P1.

| Child | Owns |
|-------|------|
| 1 | The recorder process and fault reader, the record model and grammar, build identity, and the `show faults` investigation surface (Implementation Step 1 holds the detail) |
| 2..N | One child per new fault source: hang/deadlock, abnormal exit (SIGKILL, OOM, plugin child), recovered panic, resource leak |

The child list and boundaries are settled at the RESEARCH and DESIGN gates.

## Required Reading

- [ ] `docs/architecture/diagnostics/crash-capture.md`
  -> Constraint: the runtime writes fatal output with the world stopped, so only a file handed over by `debug.SetCrashOutput` at start can capture it; no in-process collector can.
  -> Constraint: kernel records are a second artifact kind in the same directory, rotation, listing and support module; a new fault source joins that model, never a parallel system.
  -> Constraint: the page's decision "Crash files are written on panic detection, not continuously. A continuous `current.log` needs a clean-shutdown marker, which conflicts with the many `os.Exit` paths in `main()`" is what child 2's clean-exit marker revisits, so child 2 corrects that page in the same work.
- [ ] `internal/core/crashlog/crashout.go`
  -> Constraint: `harvestPending` deletes a pending file holding only its header as "a clean exit", but nothing marks a clean exit, so a SIGKILL or OOM kill is indistinguishable and its record is destroyed. This is a defect (silently wrong value), owned by the abnormal-exit child.
- [ ] `internal/plugins/crashes/cmd/show.go`
  -> Constraint: `show crashes` answers a list plus raw report text; MCP tools are generated from registered RPCs (`docs/architecture/diagnostics/production-diagnostics.md`), so a structured investigation surface reaches MCP by being a registered command, with no MCP code.
- [ ] `internal/core/slogutil/ring.go` `ringHandler.Handle`, `LogEntry.Attributes`
  -> Constraint: a ring entry keeps its attributes (as immutable JSON, decoded by `LogEntry.Attributes`), but `crashlog.go` `buildCrashReport` prints only time, level, component and message, so the attributes never reach a crash file. Owned by child 1.
- [ ] `internal/core/crashlog/stderr.go` `panicPattern`, `crashlog.go` `appendValue`
  -> Constraint: stderr detection opens on `goroutine N [running]:`, so the preceding `panic: <value>` line is never captured; silently lossy and owned by child 1. `appendValue` is `fmt.Append` and prints every panic value.
- [ ] `internal/component/plugin/process/process.go` `monitorCmd`, `relayStderrFrom`
  -> Constraint: a plugin child's exit is recorded (`p.exit.record`, read by `stopped.go` `ExitStatus` and carried by `manager.go` `ReportCrash`); its stderr reaches the engine log only, with relay disabled the trace is discarded, and the host writes no crash file for a forked child.
- [ ] `internal/core/taskguard/guard.go` `Task.Contain`, `Guard.record`
  -> Constraint: the shared panic boundary for network-input tasks already logs, counts (`ze_task_panics_total{component,task}`) and records each contained panic; child 4 extends it and writes no second helper. Its `showCrashesCommand` constant spells `ze show crashes`, so the child 1 rename reaches it.
- [ ] `internal/core/crashlog/recovered.go` `RecordRecoveredPanic`, `rotateRecoveredFiles`
  -> Constraint: one file per recovered panic, with no dedup and no rate limit, rotated apart from process crashes under the same `ze.crash.keep` count; R-1 still stands until child 4 adds dedup and the limit.
- [ ] `cmd/ze/hub/main.go` `runYANGConfig`, `stopAPI`, `exitForced`; `cmd/ze/main.go` `main`; `internal/component/plugin/server/failure_policy.go` `stopDaemonForPlugin`
  -> Constraint: `main` is the single choke point (dispatchMain, flushCrashlog, os.Exit) except `exitForced`, `startPprof`, `env.mustBeRegistered` and the reboot path, each of which needs its own clean-exit marker write.
  -> Constraint: `stopDaemonForPlugin` already persists `LastStopEntry{Reason: LastStopFatalPolicy}` for `ze doctor` and `runYANGConfig` (`stopAPI` reading `apiServer.StopFailure()`) already exits 1 on a plugin `FailureFatal` stop; child 2 adds the `crashlog` fault record alongside them and child 1's recorder restarts the daemon.
- [ ] `internal/component/mcp/tools.go` `run`, `CommandInfo`; `internal/component/mcp/streamable_tools.go` `checkCommandScope`, `isCommandReadOnly`, `registeredReadOnly`
  -> Constraint: command output is returned whole with no size cap or paging; `CommandInfo.ReadOnly` is already read by `streamable_tools.go` for `bearer-list` scope enforcement; `request support archive` is served by the daemon as a local-data command, so only offline `show crashes` with a dead daemon is unreachable over MCP.
- [ ] `internal/component/cmd/show/goroutines.go` `handleShowSystemGoroutines`
  -> Decision: a live goroutine dump already exists and is the building block for a hang record; it needs dispatch alive, so a hang detector must take its dump in-process, not via the command path.

### Architecture Docs
- [ ] `docs/guide/mcp/overview.md`
  -> Constraint: MCP serves dispatcher commands as tools; offline/local commands are not dispatched, so offline fault reading over MCP needs a route child 1 designs.
- [ ] `docs/architecture/api/process-protocol.md`
  -> Constraint: plugin child lifecycle is host-side; child 3 records exit status without changing the protocol unless it decides otherwise; child 1 moves daemon restart supervision from systemd/gokrazy into the recorder parent.
- [ ] `docs/architecture/system-architecture.md`
  -> Constraint: describes `ze start` and daemon supervision; child 1's recorder-as-parent start changes it.

## Current Behavior (MANDATORY)

Source files read:
- [ ] `internal/core/crashlog/crashout.go`
- [ ] `internal/core/crashlog/stderr.go`
- [ ] `internal/core/crashlog/crashlog.go`
- [ ] `internal/core/slogutil/ring.go`
- [ ] `internal/plugins/crashes/cmd/show.go`
- [ ] `internal/plugins/crashes/crashes.go`
- [ ] `internal/component/plugin/process/process.go`
- [ ] `internal/component/plugin/server/failure_policy.go`
- [ ] `internal/component/mcp/tools.go`
- [ ] `cmd/ze/hub/main.go`

Facts below were reported by research agents and the load-bearing ones re-read at the producer (marked V); the rest are agent-read (A) or inferred (I).

| Fault | Recorded today? | Where it goes | Gap |
|-------|-----------------|---------------|-----|
| Unrecovered panic, Go fatal error, SIGQUIT | Yes (V) | Runtime crash output file, harvested on next start | No version, no build tags, no log ring; free text only |
| Main-goroutine panic | Yes (V) | `HandlePanic` crash file with header, panic value as `fmt` prints it, stack, last 64 ring entries | The ring keeps attributes, but `buildCrashReport` prints only time, level, component and message (V) |
| Panic trace printed to `os.Stderr` | Partly (V) | Relay detects and writes crash file | `panic: <value>` line lost (V) |
| Kernel panic or oops | Yes (A) | pstore record harvested on next boot | Shares rotation (`ze.crash.keep`) with Go panics: `rotateCrashFiles` leaves out only recovered records, so panics evict kernel records (V) |
| SIGKILL, OOM kill, power loss, self reboot | No (V) | Header-only pending file deleted as "clean exit" | Indistinguishable from clean exit; no boot id; `oom_kill` in vmstat not compared at start (A) |
| Daemon stop on plugin `FailureFatal` | Partly (V) | `stopDaemonForPlugin` persists `LastStopEntry{Reason: LastStopFatalPolicy}` for `ze doctor` and `runYANGConfig` (`stopAPI` reading `apiServer.StopFailure()`) exits 1 | No `crashlog` fault record for `show faults` or the fault reader; restarts still depend on platform supervisors until child 1's recorder takes over |
| Plugin child crash or kill | Partly (V) | Engine log line; `ReportCrash` raises `doctor-plugin-crashed` carrying the recorded exit status (`p.exit.record`, `ExitStatus`); an internal plugin's panic is contained by `taskguard` and recorded | Trace discarded when relay off (`relayStderrFrom`); no crash file for a forked child (V) |
| Recovered panic | Partly (V) | Shared boundary `taskguard` `Task.Contain` (62 uses): ERROR line with value and stack, `ze_task_panics_total{component,task}`, `crash-<time>-recovered.log`. 55 other `recover()` sites recover on their own; 1 swallowed silently (bfd `trySendStateChange`) | No dedup by stack signature, no rate limit; of the 55 sites outside the boundary only the main-goroutine site (`cmd/ze/ze_core_dispatch.go`, `HandlePanic`) writes a record (V) |
| Hang, deadlock, wedged daemon | No (A) | Nothing; no watchdog, `/health` check has no timeout | Undetected, unrestarted, unrecorded |
| Goroutine, fd, memory leak | No (A) | Exported as Prometheus metrics only | Nothing watches a trend or threshold |
| VPP crash | Partly (A) | Log line with exit status, restarted with backoff | No record |

| Investigation surface | State |
|-----------------------|-------|
| `show crashes` list | name, size, kind; kind derived from name: `kernel`, `recovered`, and `panic` for every other Go kind (V: `crashlog.CrashKind`) |
| `show crashes latest / name` | Raw text; empty answer for both missing and unreadable file (A) |
| MCP | Generated tools reach `show crashes` live; no output cap (V: `run`); `CommandInfo.ReadOnly` is read by `streamable_tools.go` (`checkCommandScope`, `isCommandReadOnly`, `registeredReadOnly`) for `bearer-list` scope enforcement (V); `request support archive` is a local-data command the daemon serves (V), so only offline `show crashes` with a dead daemon is unreachable |
| Build identity | Appliance builds pass no ldflags, so version reads `dev`; `vcs.revision` likely absent in gok builds (I); image manifest in `/perm` never read by crash headers (A) |
| Supporting live inputs | Log ring (512, message only), event and FSM history, health, goroutine dump, pprof, kernel log: all live-daemon only (A) |

| Recording I/O at fault time | State |
|-----------------------------|-------|
| Kernel record | Written by the kernel to reserved RAM (pstore ramoops); no disk at fault time (A) |
| Runtime fatal | Runtime writes to a file opened at start; the write itself can block on a wedged disk and the record is lost (V: `armCrashOutput`) |
| `HandlePanic` | Builds the report, then `os.WriteFile` synchronously on the faulting goroutine; a write error returns silently, so the record vanishes with no trace (V: `HandlePanic`, `writeCrashFile`) |
| Kernel config | `CONFIG_PSTORE` and `CONFIG_PSTORE_RAM` set; `CONFIG_PSTORE_PMSG` not set (V: `gokrazy/kernel/runtime.config`) |

## Existing Mechanisms (MANDATORY)

A design starts from the mechanisms Ze already has, and each row says how this umbrella's children extend or replace one.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Crash output arming | `internal/core/crashlog/crashout.go` `armCrashOutput` | Creates a locked pending file under `.pending/`, writes a header and hands it to `debug.SetCrashOutput` | Replaces: child 1 points `debug.SetCrashOutput` at the recorder pipe and keeps the pre-opened file as the fallback |
| Pending harvest | `crashout.go` `harvestPendingCrashes`, `harvestPending` | On the next start, turns a pending file with runtime output into a `crash-*.log`; deletes a header-only file as a clean exit | Extends: child 2 adds the clean-exit marker and the killed and oom kinds |
| `HandlePanic` | `internal/core/crashlog/crashlog.go` `HandlePanic` | Builds the report and writes it synchronously on the faulting goroutine | Extends: child 1 routes it through the enqueue path |
| Stderr relay panic detection | `internal/core/crashlog/stderr.go` `relayStderr`, `panicPattern` | Opens a trace on `goroutine N [running]:` in what Go code writes to `os.Stderr` | Extends: child 1 fixes the lost `panic: <value>` line |
| Recovered-panic boundary | `internal/core/taskguard/guard.go` `Task.Contain`; `internal/core/crashlog/recovered.go` `RecordRecoveredPanic` | Logs one ERROR line, increments `ze_task_panics_total{component,task}`, writes `crash-<time>-recovered.log`; no dedup, no rate limit | Extends: child 4 builds on it rather than writing a second helper; it adds dedup by stack signature and the rate limit, and moves the remaining recovery sites onto it |
| Kernel pstore harvest | `internal/core/crashlog/kernel.go`, `kernel_linux.go`; `internal/plugins/crashes/register.go` `BootEffect` (`harvestAtDaemonStart`) | Reads ramoops records on the next healthy boot and writes `crash-*-kernel.log` | Extends: pmsg harvest and the systemd-pstore archive (child 1) |
| Rotation | `internal/core/crashlog/persist.go` `rotateCrashFiles`; `recovered.go` `rotateRecoveredFiles` | Recovered records rotate on their own; panic and kernel records share one `ze.crash.keep` count | Extends: per-kind retention (child 1) |
| `show crashes` | `internal/plugins/crashes/cmd/show.go` and its YANG; offline fallback in `internal/plugins/crashes/register.go` | Lists name, size and kind (panic, kernel, recovered) and prints raw report text | Replaces: `show faults` (child 1) |
| Support bundle `crashes` module | `internal/component/support/support.go` `collectCrashes`; `sanitize.go` `sanitizeConfig` | Puts every crash report's raw text in the bundle; the sanitizer is applied to the config tree only (`collectConfig`), so crash text is not sanitized | Extends: R-5 and AC-11 need the sanitizer to cover crash text |
| Crash doctor checks | `internal/plugins/crashes/doctor.go` (`doctor-crash-capture-unarmed`, `doctor-crash-capture-pstore`, `doctor-crash-directory-unwritable`) | Reports the reservation, the record store and the crash directory | Extends: recorder and pmsg checks (child 1) |
| Plugin crash reporting | `internal/component/plugin/process/manager.go` `ReportCrash`; `internal/component/plugin/server/failure_policy.go` | Raises `doctor-plugin-crashed` on the report bus with the recorded exit status, then applies the failure policy | Extends: children 2 and 3 |
| Log ring | `internal/core/slogutil/ring.go` `LogEntry.Attributes` | Keeps each entry's attributes; the crash report prints only time, level, component and message (`crashlog.go` `buildCrashReport`) | Extends: crash reports print the attributes (child 1) |
| Goroutine dump | `internal/component/cmd/show/goroutines.go` `handleShowSystemGoroutines` | Live dump through the command dispatcher | Extends: the building block of the hang record (child 5), taken in process |
| `/health` | `internal/core/health/registry.go` `Registry.Handler`, mounted by `internal/component/telemetry/exporter/server.go` | Serves the registered health checks | Extends: a per-check timeout (child 5) |
| Runtime metrics (goroutines, fds, heap) | `internal/core/metrics/prometheus.go` `RegisterRuntimeCollectors` (`collectors.NewGoCollector`, `NewProcessCollector`) | Exports `go_*` and `process_*` metrics; nothing watches them | Does not apply: child 6 samples in process (Key Design Decisions, the leak signal row) |
| systemd unit | `internal/plugins/systemd/unit.go`, `native.go` | Generates and installs the unit that runs the daemon | Replaces: the unit supervises the recorder (child 1) |
| gokrazy supervision | `gokrazy/ze/config.json` (`ze start`); gokrazy `runWatchdog` (A-5) | gokrazy runs `ze start` and restarts it on a non-zero exit; its watchdog pets `/dev/watchdog` independently of ze | Replaces: the recorder owns restarts |
| Plugin-fatal stop and last-stop record | `internal/component/plugin/server/failure_policy.go` `Server.stopDaemonForPlugin`, `LastStopEntry{Reason: LastStopFatalPolicy}`; `cmd/ze/hub/main.go` `runYANGConfig`, `stopAPI`, `apiServer.StopFailure` | Writes `LastStopEntry` to the store for `ze doctor`, records `StopFailure`, triggers daemon shutdown, and exits 1 | Extends: child 2 writes a `crashlog` stop-reason fault record alongside `LastStopEntry` and exit 1, and child 1's recorder owns the restart (AC-5) |
| MCP tool runner and scope check | `internal/component/mcp/tools.go` `server.run`, `CommandInfo`; `internal/component/mcp/streamable_tools.go` `checkCommandScope`, `isCommandReadOnly`, `registeredReadOnly` | Dispatches commands via `s.dispatch.JSON` and returns rendered output whole with no size cap or paging; reads `CommandInfo.ReadOnly` for `bearer-list` scope enforcement | Extends: child 1 adds output paging for large answers (R-2, AC-2) while preserving existing `CommandInfo.ReadOnly` scope enforcement |
| VPP process wait and supervision | `internal/component/vpp/vpp.go` `waitVPPConnection`, `waitVPPConnectionWithStop` | Waits on the supervised VPP child (`cmd.Wait()`), classifies expected kills vs crashes, logs exit status, and restarts with backoff without writing a fault record | Extends: child 3 records VPP exit status and trace into `crashlog` on exit or crash (AC-6) without changing VPP supervision |
| Daemon start entry point | `cmd/ze/ze_core_start.go` `cmdStart` | Parses `ze start` flags and config source, opens the live store, and invokes `hub.Run` directly in the same process | Extends: child 1 turns `ze start` into the recorder parent process that spawns, watches and restarts the daemon child and starts the fault-reader HTTP process |
| Appliance kernel pstore config | `gokrazy/kernel/runtime.config` (`CONFIG_PSTORE=y`, `CONFIG_PSTORE_RAM=y`) | Enables kernel pstore and ramoops for kernel panics; `CONFIG_PSTORE_PMSG` is not set | Extends: child 1 enables `CONFIG_PSTORE_PMSG` and splits the reserved RAM region explicitly (R-13) for disk-failure fallback (AC-14) |

## Design documents

Derived from `./le spec design documents spec plan/release/fixes/spec-fault-investigation-0-umbrella.md` (41 design pages and 47 decision rows). Every departure below reflects owner decisions recorded on 2026-10-24 and 2026-10-25 (`## Key Design Decisions`); each child spec updates the affected design pages in the same commit as its code.

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A command is declared once and reaches every interface." | agrees | `show faults` is declared once in YANG and registered as a command so CLI, SSH, web, REST, gRPC and MCP derive it automatically; the fault-reader HTTP endpoint is a separate out-of-daemon survival surface for when the daemon is down (owner decision, 2026-10-25) |
| `docs/architecture/diagnostics/production-diagnostics.md` | Structured log fields | the `// Design:` page of `internal/core/slogutil/ring.go`; anchored to `ring.go`; a page this spec modifies | "Crash reports keep their existing message-only recent-log header rather than adding structured fields implicitly." | departs | Child 1 prints the structured attributes already stored in `slogutil.LogEntry.Attributes` into fault reports so AC-2 delivers log context with attributes |
| `docs/architecture/diagnostics/production-diagnostics.md` | CLI first, MCP for free | the `// Design:` page of `internal/core/slogutil/ring.go`; anchored to `ring.go`; a page this spec modifies | "Every diagnostic is a YANG RPC registered as a CLI command. The MCP tool list is generated from the registered RPCs, so a new diagnostic appears to an AI agent with no MCP code change." | agrees | `show faults` is a YANG RPC command so MCP derives its tool automatically; child 1 adds generic MCP output paging in `tools.go` and a separate HTTP fault reader for when the daemon is down |
| `ai/doors/core.md` | Where it sits | the door covering `internal/core/slogutil/ring.go`, `internal/core/taskguard/guard.go`, `internal/core/crashlog/recovered.go`, `internal/core/crashlog/kernel_linux.go` | "A core package is a library with no config-driven lifecycle and no registry side effect" | agrees | `crashlog`, `taskguard`, `slogutil` and `health` remain core leaf libraries importing neither `internal/component/` nor `internal/plugins/` |
| `docs/architecture/system-architecture.md` | How CLI Works | the `// Design:` page of `cmd/ze/main.go`; anchored to `cmd/ze/hub/main.go`, `internal/plugins/systemd/unit.go`; a page this spec modifies | "- `ze start config.conf` → starts as daemon, listens on SSH port" | departs | Child 1 makes `ze start` the recorder process, which starts, watches and restarts the daemon as its child and supervises the separate fault-reader HTTP process (owner decision, 2026-10-25) |
| `docs/architecture/core-design.md` | Command engine and composition roots | anchored to `cmd/ze/main.go`, `cmd/ze/hub/main.go` | "Every personality enters through `main` in `cmd/ze/main.go`, which seals the plugin registry (`registry.Seal`) before it dispatches, so a registration made after the `init()` functions ran is refused." | agrees | Child 2 adds the clean-exit marker at the `main` choke point and four side exits without altering `ze`/`le` composition separation or registry sealing |
| `docs/architecture/plugin/plugin-system.md` | Registration fields | anchored to `cmd/ze/main.go`, `internal/component/plugin/process/process.go`, `internal/component/plugin/process/manager.go` | "Crash capture uses it to harvest the kernel crash record the previous boot left" | agrees | Child 1 extends the existing `BootEffect` harvest to pmsg and systemd-pstore without changing plugin registration or discovery |
| `ai/doors/hub-engine.md` | Boundaries | the door covering `cmd/ze/main.go`, `cmd/ze/hub/main.go` | "No always-on file imports a gated package." | agrees | Recorder, crashlog and faults command respect feature gates; VPP and BGP stay behind their gates and hook through their own packages without always-on imports |
| `docs/architecture/hub-architecture.md` | Startup Sequence | the `// Design:` page of `cmd/ze/hub/main.go`; anchored to `hub/main.go`, `process.go`, `manager.go` | "The supervisor restarts exit 75; the generated systemd unit marks it successful and forces restart, and gokrazy's normal supervision loop retries nonzero exits other than 125." | departs | Child 1 makes the recorder the parent of the daemon, owning daemon restarts and crash-loop policy identically across gokrazy and Debian/Ubuntu, while the platform supervises only the recorder (owner decision, 2026-10-25) |
| `docs/architecture/api/architecture.md` | gNMI Transport | anchored to `cmd/ze/hub/main.go`, `process.go`, `internal/component/mcp/tools.go` | "the panic is logged, counted in `ze_task_panics_total{component="gnmi",task="rpc"}` and crash-recorded, and the listener and every other call keep running." | agrees | gNMI already routes unary and stream RPC panics through the shared `taskguard` boundary; child 4 extends that boundary with stack-signature dedup and per-site rate limiting without changing gNMI panic containment |
| `docs/architecture/api/commands.md` | Offline fallback (read-only commands without a daemon) | anchored to `cmd/ze/hub/main.go`, `manager.go`; the `// Design:` page of `internal/component/mcp/tools.go`; anchored to `tools.go` | "`show crashes` helps investigate a stopped daemon" | departs | Child 1 replaces `show crashes` with `show faults` with no alias (owner decision, 2026-10-24), while keeping the single-parser online/offline fallback parity model of AC-3 |
| `docs/architecture/api/process-protocol.md` | 5-Stage Startup Protocol | anchored to `cmd/ze/hub/main.go`, `unit.go`; the `// Design:` page of `process.go` and `manager.go`; a page this spec modifies | "Only a stop nothing failed in exits 0, so a service manager that restarts on failure (systemd `Restart=on-failure`) and an operator's script see the fatal stop for what it is." | departs | Child 1 moves daemon restart and crash-loop supervision from systemd (`Restart=on-failure`) and gokrazy into the recorder parent process identically across gokrazy and Debian/Ubuntu (owner decision, 2026-10-25); child 2 keeps the existing exit-1 return and `LastStopEntry` and adds the `crashlog` fault record alongside them (AC-5), and child 3 captures plugin child exit status and stderr trace on the host side without changing the wire protocol |
| `docs/architecture/api/process-protocol.md` | The stop drain | anchored to `cmd/ze/hub/main.go`, `unit.go`; the `// Design:` page of `process.go` and `manager.go`; a page this spec modifies | "The generated systemd unit accepts 75 as successful and forces a new start. Gokrazy's normal supervision loop also retries this nonzero exit; a foreground caller must start the next process itself." | departs | Child 1 makes the recorder parent supervise the daemon and own daemon restarts identically on gokrazy and Debian/Ubuntu, while the platform supervises only the recorder (owner decision, 2026-10-25) |
| `docs/architecture/appliance/remote-operations.md` | Decisions | anchored to `cmd/ze/hub/main.go` | "" | silent | Covers authenticated SSH config upload and the legacy boot inbox; this umbrella does not change remote config upload |
| `docs/architecture/behavior/signals.md` | Startup Paths | anchored to `cmd/ze/hub/main.go`, `internal/plugins/systemd/unit.go`; a page this spec modifies | "and `exec` keeps ze as the main PID, so `ExecReload` still signals ze." | departs | Child 1 makes `ze start` the recorder parent process, which becomes the main PID under systemd and gokrazy and spawns the daemon as its child, forwarding `SIGHUP`/`SIGTERM` to the daemon child (owner decision, 2026-10-25) |
| `docs/architecture/behavior/signals.md` | Ze Signal Mapping | anchored to `cmd/ze/hub/main.go`, `internal/plugins/systemd/unit.go`; a page this spec modifies | "Go runtime default (not caught -- useful for debugging)" | agrees | Child 5's external prober (the recorder) relies on uncaught SIGQUIT to dump every goroutine into the armed crash output on a whole-process freeze (AC-12) |
| `docs/architecture/cli/command-completion.md` | The producers | anchored to `cmd/ze/hub/main.go` | "Every producer asks one predicate what its surface owes a command:" | agrees | `show faults` and its dynamic record-name `CompleteFn` (child 1) flow through the existing command tree and `AnswerSurface` producers |
| `docs/architecture/config/syntax.md` | Overview | anchored to `cmd/ze/hub/main.go` | "" | silent | Covers config file grammar and keywords; children 1, 5 and 6 add ordinary YANG leaves without changing config syntax |
| `docs/architecture/config/yang-config-design.md` | 1. Key Principle | anchored to `cmd/ze/hub/main.go` | "" | silent | Covers YANG module loading and validation infrastructure; children 1, 5 and 6 author ordinary YANG modules under the existing system |
| `docs/architecture/diagnostics/crash-capture.md` | The decisions | anchored to `cmd/ze/hub/main.go`; the `// Design:` page of `guard.go`, `recovered.go`, `kernel_linux.go`; a page this spec modifies | "A file with only the header came from a clean exit and is removed." | departs | Child 1 moves primary runtime crash output to the recorder pipe (keeping the pre-opened file as fallback), and child 2 adds a clean-exit marker so a header-only pending file without that marker is recorded as `killed` or `oom` rather than discarded (AC-4, AC-23) |
| `docs/architecture/diagnostics/crash-capture.md` | The decisions | anchored to `cmd/ze/hub/main.go`; the `// Design:` page of `guard.go`, `recovered.go`, `kernel_linux.go`; a page this spec modifies | "A continuous `current.log` needs a clean-shutdown marker, which conflicts with the many `os.Exit` paths in `main()`." | departs | Child 2 adds the clean-exit marker at the `main` choke point and the four side exits so SIGKILL and OOM kills are distinguished from clean stops (owner decision, 2026-10-24) |
| `docs/architecture/diagnostics/crash-capture.md` | Constraints | anchored to `cmd/ze/hub/main.go`; the `// Design:` page of `guard.go`, `recovered.go`, `kernel_linux.go`; a page this spec modifies | "A crash file from the runtime carries no log ring for the same reason." | departs | Child 1's out-of-process recorder keeps the recent ze log stream in a preallocated ring and persists it with every record, runtime fatals included, so pre-crash logs survive reboot without remote syslog (AC-21, owner decision, 2026-10-25) |
| `docs/architecture/diagnostics/crash-capture.md` | Recovered panics | anchored to `cmd/ze/hub/main.go`; the `// Design:` page of `guard.go`, `recovered.go`, `kernel_linux.go`; a page this spec modifies | "listed by `show crashes` with kind `recovered`" | departs | Child 1 replaces `show crashes` with `show faults` with no alias and introduces per-kind retention (AC-10); child 4 adds dedup by stack signature and per-site rate limit to `taskguard` and `RecordRecoveredPanic` (AC-7) |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | anchored to `cmd/ze/hub/main.go`, `process.go`, `manager.go` | "Some readiness facts exist only in the running daemon:" | agrees | Each child registers a doctor check and code for every new dependency and problem class (recorder alive, pmsg unarmed, `/dev/kmsg`, hang restart, crash loop, adopted degraded state, queue drops, dead fault reader, plugin-fatal stop) in the existing doctor model |
| `docs/architecture/fleet-config.md` | Connection Lifecycle | anchored to `cmd/ze/hub/main.go`; a page this spec modifies | "The panic is counted in `ze_task_panics_total{component="plugin",task="managed-client"}`, logged at ERROR and recorded for `ze show crashes`" | departs | Child 1 replaces `ze show crashes` with `ze show faults` (no alias; owner decision, 2026-10-24) and updates this page in the same change; child 4 adds stack-signature deduplication and rate limiting to `taskguard` panic recording |
| `docs/architecture/pki/pki-store.md` | Decisions | anchored to `cmd/ze/hub/main.go` | "" | silent | Covers PKI certificate storage; this umbrella does not touch PKI |
| `docs/architecture/pki/tls-listeners.md` | Decision: a certificate that cannot be presented never stops ze | anchored to `cmd/ze/hub/main.go` | "" | silent | Covers PKI certificates on daemon TLS listeners; this umbrella does not change daemon TLS listeners, and the fault-reader survival endpoint was decided HTTP-only by Thomas (2026-10-25) |
| `docs/architecture/storage-backends.md` | Opening and ownership | anchored to `cmd/ze/hub/main.go` | "" | silent | Covers ZeFS configuration and runtime-state storage (`database/`); fault records remain files in the crash directory and reserved RAM (pstore/pmsg) |
| `docs/architecture/testing/ci-format.md` | Registered engines launched externally | anchored to `cmd/ze/hub/main.go`, `process.go` | "This exception applies to this test launcher only; runtime crash-file capture remains armed." | agrees | Child 1's recorder pipe (with pre-opened file fallback) and child 3's host-side stderr capture preserve `le mock plugin-external`'s stderr-pipe flush and armed crash capture; children 1 and 3 add ordinary `.ci` tests using the existing format |
| `docs/architecture/web-interface.md` | Starting the Web Server | anchored to `cmd/ze/hub/main.go` | "A second Ctrl+C during that wait stops waiting: the process drains its stderr relay (`crashlog.Flush`, at most 500 ms, and at once when no child process holds the stderr pipe), prints "forced exit" and exits 1." | agrees | Child 2 writes the clean-exit marker inside `exitForced` alongside `crashlog.Flush` so a forced Ctrl+C exit is not misfiled as a kill, without changing web-only shutdown behavior or the web UI |
| `docs/architecture/api/capability-contract.md` | Architecture | anchored to `internal/component/plugin/process/process.go` | "" | silent | Covers BGP capability negotiation and cache control between engine and API plugins; outside fault investigation |
| `docs/architecture/api/ipc_protocol.md` | Overview | anchored to `internal/component/plugin/process/process.go` | "" | silent | Covers plugin IPC framing and answer protocol; this umbrella does not change IPC framing |
| `docs/architecture/config/environment.md` | Overview | anchored to `internal/component/plugin/process/process.go` | "Every runtime lookup via `internal/core/env.Get*` MUST hit a registered key; unregistered keys abort the process." | agrees | Any new or renamed `ze.crash.*` / `ze.fault.*` or threshold env vars introduced by children 1, 5 and 6 register via `env.MustRegister` |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | anchored to `internal/component/plugin/process/process.go` | "Both are logged, counted in `ze_task_panics_total` and crash-recorded (`docs/guide/monitoring.md`, "Contained Panics")." | agrees | IKE already uses the shared `taskguard` boundary that logs, counts `ze_task_panics_total` and records contained panics; child 4 extends that shared boundary with stack-signature dedup and per-site rate limiting without changing IKE containment semantics |
| `docs/architecture/plugin/show-enrichers.md` | Constraints | anchored to `internal/component/plugin/process/process.go` | "The show command continues with partial data, which is safe because the enrich entry point recovers from panics." | agrees | Child 4 migrates the raw `recover()` sites in `internal/core/show/show.go` onto the shared `taskguard` boundary so enricher panics are logged, counted, deduplicated and crash-recorded while show commands continue returning partial data |
| `docs/architecture/update-cache.md` | Purpose | anchored to `internal/component/plugin/process/process.go` | "" | silent | Covers the BGP UPDATE cache; outside fault investigation |
| `ai/doors/plugin-infra.md` | Boundaries | the door covering `internal/component/plugin/process/process.go`, `internal/component/plugin/process/manager.go` | "A compiled-in and an external plugin differ only in how they reach the bus." | agrees | Child 3 captures exit status and stderr trace for both internal and forked plugins on the host side without altering the plugin protocol or `pkg/plugin` wire types |
| `docs/research/vpp-deployment-reference.md` | GoVPP Connection | the `// Design:` page of `internal/component/vpp/vpp.go` | "On API loss, the manager kills and waits for the exact VPP child it supervised before returning or restarting." | agrees | Child 3 only records VPP's exit status and trace into `crashlog` when VPP exits or crashes (AC-6), leaving VPP supervision and recovery unchanged |
| `docs/architecture/fib/fib-depth-4-srv6.md` | Where the SID comes from | anchored to `internal/component/vpp/vpp.go` | "" | silent | Covers SRv6 FIB programming on kernel and VPP; child 3 only observes VPP process exit and touches no FIB logic |
| `ai/doors/forwarding.md` | Where it sits | the door covering `internal/component/vpp/vpp.go` | "`vpp` supervises the VPP process and its GoVPP connection" | agrees | Child 3 records VPP exit status and trace into `crashlog` without changing VPP supervision, forwarding backends or route ownership |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Decision | anchored to `internal/component/plugin/process/manager.go` | "" | silent | Covers Prometheus pull-model metrics; child 6 samples process metrics in-process for leak detection and children 1 and 4 export ordinary Prometheus counters without changing the pull model |
| `docs/guide/mcp/overview.md` | Tools | the `// Design:` page of `internal/component/mcp/tools.go`; a page this spec modifies | "All MCP tools are **auto-generated** from the YANG command registry at `tools/list` time." | agrees | `show faults` becomes an MCP tool automatically through YANG RPC registration; child 1 adds generic MCP output paging in `tools.go` and documents it alongside the fault-reader HTTP endpoint |
| `docs/architecture/mcp/overview.md` | Auto-Generated Tools | anchored to `internal/component/mcp/tools.go` | "The command list is the dispatcher's, so every command the daemon serves is a tool:" | agrees | `show faults` reaches MCP through command registration with no hardcoded fault tool; child 1 adds generic output paging for large answers (R-2, AC-2) |
| `ai/doors/api-surfaces.md` | Where it sits | the door covering `internal/component/mcp/tools.go` | "None of them owns a command or reads protocol state." | agrees | `show faults` is derived from `CommandLister` with no fault-specific MCP code; the MCP change in child 1 is generic output paging |
| `ai/doors/appliance.md` | Who depends on it from outside | the door covering `gokrazy/ze/config.json` | "An image already in the field is the contract." | agrees | Child 1 stamps build identity into appliance builds, splits the existing ramoops reservation explicitly for `CONFIG_PSTORE_PMSG` (R-13), and preserves `/perm/ze/` and zefs compatibility |
| `docs/architecture/cli/plugin-modes.md` | Native installation commands | the `// Design:` page of `internal/plugins/systemd/unit.go` and `native.go`; anchored to `native.go`; a page this spec modifies | "On a running systemd host, the service owner records the selected service's active state before replacement. A compatible administrator unit and its drop-ins are retained." | agrees | Child 1 updates the generated systemd unit and its description so the unit supervises the recorder process (which owns daemon restarts), while preserving native installation trust, preflight and retention semantics |
| `ai/doors/system-plugins.md` | Boundaries | the door covering `internal/plugins/systemd/unit.go`, `internal/plugins/systemd/native.go` | "A plugin owns its whole surface." | agrees | The faults plugin (renamed from `crashes` in child 1) owns its YANG, commands, offline fallback, boot effect and doctor checks; `internal/plugins/systemd` owns the systemd unit |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/main.go`, `cmd/ze/hub/main.go` | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This umbrella does not change VPP/kernel data-plane mismatch validation |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/main.go`, `cmd/ze/hub/main.go` | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This umbrella does not change data-plane mismatch refusal behavior |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/main.go`, `cmd/ze/hub/main.go` | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This umbrella does not touch firewall verifiers or `ze:backend` annotations |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/main.go`, `cmd/ze/hub/main.go` | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This umbrella does not touch data-plane selectors |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/main.go`, `cmd/ze/hub/main.go` | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | This umbrella does not touch BGP next-hop rewriting |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/main.go`, `cmd/ze/hub/main.go` | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This umbrella does not touch BGP export chains or policy modify |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/main.go`, `cmd/ze/hub/main.go` | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This umbrella does not touch BGP export chain compilation |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/main.go`, `cmd/ze/hub/main.go` | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | This umbrella does not touch BGP export next-hop folding |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/main.go`, `cmd/ze/hub/main.go` | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This umbrella does not touch BGP policy next-hop validation |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/main.go`, `cmd/ze/hub/main.go` | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | This umbrella does not touch BGP originated-route next-hop policy |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/main.go`, `cmd/ze/hub/main.go` | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This umbrella does not touch implied filter-chain publication |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/main.go`, `cmd/ze/hub/main.go` | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Matches R-20 (fault-reader port collision is refused at commit, while a bind failure at start is a doctor finding that never stops ze) and AC-16 (if the recorder is absent, ze starts with pre-opened file fallback and doctor reports it); AC-5 covers runtime `FailureFatal` plugin crashes, not startup service misconfiguration |
| `docs/architecture/core-design.md` | Design decisions | anchored to `cmd/ze/main.go`, `cmd/ze/hub/main.go` | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | This umbrella does not change VPP/kernel feature startup or validation verdicts |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | anchored to `cmd/ze/main.go`, `process.go`, `manager.go` | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | This umbrella does not change offline config parsing or query-mode plugin interrogation |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `cmd/ze/hub/main.go`, `process.go`, `mcp/tools.go` | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | This umbrella does not change `\| implied` rendering |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `cmd/ze/hub/main.go`, `process.go`, `mcp/tools.go` | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | This umbrella does not change bus participant registration or message defaults |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `cmd/ze/hub/main.go`, `process.go`, `mcp/tools.go` | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | This umbrella does not change bus `send > to` semantics |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `cmd/ze/hub/main.go`, `process.go`, `mcp/tools.go` | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | This umbrella does not change bus `receive > from` semantics |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `cmd/ze/hub/main.go`, `process.go`, `mcp/tools.go` | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | This umbrella does not change bus send/receive allow-lists |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `cmd/ze/hub/main.go`, `process.go`, `mcp/tools.go` | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | This umbrella does not change bus filter compilation or commit conflict checks |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `cmd/ze/hub/main.go`, `process.go`, `mcp/tools.go` | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | This umbrella does not add bus arbitration |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `cmd/ze/hub/main.go`, `process.go`, `mcp/tools.go` | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | This umbrella does not change effective bus display |
| `docs/architecture/api/commands.md` | Design decisions | anchored to `cmd/ze/hub/main.go`, `manager.go`; `// Design:` and anchored to `mcp/tools.go` | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | `show faults` names the subject and routes its structured payload through `ApplyPipes` so formatting, filtering and paging stay pipe operations; child 1's design gate verifies record/section selection against this rule |
| `docs/architecture/api/commands.md` | Design decisions | anchored to `cmd/ze/hub/main.go`, `manager.go`; `// Design:` and anchored to `mcp/tools.go` | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | `show faults` returns structured `ResponseData` and does not override format, paging, `save` or `log` |
| `docs/architecture/api/commands.md` | Design decisions | anchored to `cmd/ze/hub/main.go`, `manager.go`; `// Design:` and anchored to `mcp/tools.go` | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | `show faults` is read-only and applies pipes purely to its answer |
| `docs/architecture/api/process-protocol.md` | Design decisions | anchored to `cmd/ze/hub/main.go`, `unit.go`; `// Design:` and anchored to `process.go`, `manager.go`; modified by this spec | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | This umbrella adds no unauthorized engine operation; child 3 captures plugin child exits on the host side |
| `docs/architecture/api/process-protocol.md` | Design decisions | anchored to `cmd/ze/hub/main.go`, `unit.go`; `// Design:` and anchored to `process.go`, `manager.go`; modified by this spec | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | This umbrella does not change engine-to-plugin event delivery; the bounded drop-counting queue of AC-15 applies solely to the daemon-to-recorder fault pipe |
| `docs/architecture/fleet-config.md` | Design decisions | anchored to `cmd/ze/hub/main.go` | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | This umbrella does not change managed fleet listener certificate reload |
| `docs/architecture/fleet-config.md` | Design decisions | anchored to `cmd/ze/hub/main.go` | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | This umbrella does not touch fleet web provisioning |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | anchored to `cmd/ze/hub/main.go` | "The looking glass may serve plain HTTP, because what it publishes is public, and one looking glass serves HTTP and HTTPS at the same time. Thomas's words, as voice-transcribed: "We should allow the looking glass to be HTTP. The information is public." and "We should be able to serve on both HTTP and HTTPS, not either." Asked where the choice lives and what happens to `ze.looking-glass.tls`, he answered "1a 2a": the choice is the `tls` leaf of each `server` entry, default true, and the environment variable is removed. The no-store rule applies to each entry: an entry that left `tls` unset serves HTTP with a warning on a daemon with no `database/` store and no named certificate, an entry that says `tls true` there is refused at start and at reload, and a reload does what a restart with the same config does" | agrees | This umbrella does not touch the looking glass |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | anchored to `cmd/ze/hub/main.go` | "The web and looking glass use the shared `listenerTLSMaterial` precedence. The approved Fleet reload design shares `managedCertificate` with startup because the daemon authority issues its unnamed leaf." | agrees | This umbrella does not change web, looking-glass or fleet TLS material loading |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | anchored to `cmd/ze/hub/main.go` | "A certificate that cannot be presented never stops ze from starting. A web server whose certificate name does not resolve is not started. The looking glass is judged one `server` entry at a time: an entry that says `tls true` with no certificate to present stays down, every entry that asks for TLS stays down when the named certificate does not resolve, and the other entries serve. Each is reported with its fix by an error line in the log under a doctor code and by `ze doctor`. A reload whose config cannot be served stays refused. This supersedes the start half of the row of 2026-11-23 above, which says a `tls true` entry "is refused at start": at start that entry stays down and the others serve, and at reload the config is still refused. Thomas's words, as voice-transcribed: "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; "reload stay refused"; "Should only that endpoint stay down while the others serve: yes"" | agrees | Consistent with R-20 (bind failures are doctor findings and never prevent ze from starting) and does not alter daemon TLS listener behavior |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/component/vpp/vpp.go` | "Ze refuses to run VPP unmanaged. When VPP is external, the operator accepts that Ze may terminate and restart it" | agrees | Child 3 only records VPP exit status and trace into `crashlog` (AC-6) and does not change VPP management |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/component/vpp/vpp.go` | "Ze knows how to manage and restart an external VPP: through a systemd unit, or by starting the binary directly, as it does with the embedded runtime" | agrees | Child 3 only observes VPP exit for fault recording and does not change VPP restart methods |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/component/vpp/vpp.go` | "After a failure, when the VPP process is alive, forwarding continued, and Ze reconnects and can read the state it configured, Ze continues from where it is" | agrees | Child 3 records VPP crashes/exits without changing VPP reconnect adoption |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/component/vpp/vpp.go` | "When VPP could have crashed or stopped forwarding, Ze reflects it in its own model and makes sure IGP and BGP peers are updated" | agrees | Child 3 records VPP exit status and trace without changing VPP loss modeling or peer updates |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/component/vpp/vpp.go` | "One timer: the alive test and the loss declaration use the same time, and a probe missed inside it is not a loss. This narrows the row "After a failure, when the VPP process is alive": Ze continues from where it is only through form (1) below, inside the timer, and a declared loss is never adopted" | agrees | Child 3 does not touch VPP alive-test timers |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/component/vpp/vpp.go` | "As soon as a loss is declared, Ze makes sure the process is terminal (kill through the retained pidfd, then wait for the exit proof), then restarts and reconfigures VPP. Ze is the only restarter: a systemd unit it drives does not restart VPP itself (Restart=no)" | agrees | Child 3 does not touch VPP kill/restart sequencing |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/component/vpp/vpp.go` | ""Recovered" has exactly two forms. (1) Inside the single timer Ze reconnects to the same pidfd-proven process and a full readback of what it installed (interfaces, LCP pairs, owned routes, PPPoE sessions) matches: Ze continues, with no kill and no withdrawal. Any mismatch means form (1) failed. (2) A forced restart (kill through the retained pidfd, exit proof) followed by a correct reconfiguration, exactly as at boot. Routes are withdrawn only when neither form succeeds within the recovery deadline and the crash-loop limit" | agrees | Child 3 does not touch VPP recovery forms or route withdrawal |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/component/vpp/vpp.go` | "Blackholing during the restart is accepted. Routes are withdrawn (VPP-backed interfaces down in Ze's model, so IGP and BGP react) only when neither recovery form succeeds within the recovery deadline and the crash-loop limit. This narrows the row "When VPP could have crashed or stopped forwarding": Ze updates IGP and BGP peers only then" | agrees | Child 3 does not touch VPP blackholing or withdrawal policy |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/component/vpp/vpp.go` | "systemd configuration: Ze cannot be sure that an OS update will not change the unit's settings, so it detects the settings and adapts. No unit policy is required: Ze reads the unit's effective settings each time it acts on VPP, and adapts its restart to them. This supersedes the second sentence of the row "As soon as a loss is declared" (Ze the only restarter, Restart=no) and its forbids clause "or leaving the restart to systemd or another supervisor"; correcting that row needs Thomas's decision-change" | agrees | Child 3 does not touch VPP systemd-unit adaptation |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/component/vpp/vpp.go` | "Two limits. The alive test and the loss declaration use the same time: a probe missed inside it is not a loss, and a failed alive test is the loss declaration. A second, separate limit then bounds the reconnect-and-verify attempt of form (1). When that limit expires without form (1), Ze makes the process terminal (kill through the retained pidfd, then wait for the exit proof), then restarts and reconfigures VPP. This supersedes the row "One timer" (its single timer, "a declared loss is never adopted" and its forbids clause "a second window after the declaration"), and narrows the row "As soon as a loss is declared": the kill follows the second limit, not the declaration" | agrees | Child 3 does not touch VPP loss-declaration or reconnect limits |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/component/vpp/vpp.go` | "Form (1) adopts on the presence of data ("We should take the presence of data as a sign that the software is still running and reconciliation may be then happening in the background."). Inside the reconnect-and-verify limit, the same pidfd-proven process answering over the API with the data Ze configured is the sign it is alive: Ze adopts it, with no kill and no withdrawal, and reconciles in the background, as the fib-vpp replay and sweep and the iface reconcile do. A mismatch the background reconciliation finds is repaired, not treated as a loss. Form (2) stays a forced restart followed by a correct reconfiguration, exactly as at boot. This narrows the row "Recovered has exactly two forms": a full readback match is no longer required, and a mismatch no longer fails form (1)" | agrees | Child 3 does not touch VPP adoption or background reconciliation |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/component/vpp/vpp.go` | "A recovery deadline and a crash-loop limit (N restarts inside a window) bound recovery. Past either, Ze withdraws: VPP-backed interfaces go down in Ze's model, so IGP and BGP react, and ze doctor reports it. Blackholing while Ze restarts inside both limits is accepted. The default values are proposed at design review for Thomas's approval. This adopts the recovery deadline and the crash-loop limit that the rows "Recovered has exactly two forms" and "Blackholing during the restart is accepted" carried as proposals, and narrows the row "When VPP could have crashed or stopped forwarding": Ze updates IGP and BGP peers only past a limit" | agrees | Child 3 does not touch VPP recovery deadline or crash-loop withdrawal |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/component/vpp/vpp.go` | "After a withdrawal Ze keeps retrying recovery with a growing backoff. When VPP recovers, the VPP-backed interfaces come up in Ze's model and the routes return by themselves, and ze doctor reports the state throughout. This narrows the crash-loop limit: past it Ze withdraws and keeps restarting" | agrees | Child 3 does not touch VPP post-withdrawal backoff or recovery |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/component/vpp/vpp.go` | "The VPP loss itself triggers no withdrawal: no VPP-backed interface goes down in Ze's model while Ze recovers. Ze's own protocol timers (BFD detection, OSPF and IS-IS dead intervals, BGP hold) keep running during recovery, as their RFCs say" | agrees | Child 3 does not touch VPP loss withdrawal or protocol timers |
| `docs/research/vpp-deployment-reference.md` | Design decisions | the `// Design:` page of `internal/component/vpp/vpp.go` | "A process on VPP's API socket whose pidfd does not resolve to the configured systemd unit is reported, never refused or killed ("We should report but not refuse if the customers have changed the software, for example. It may mean that we have an out of date configuration. We should make sure the customer updates it but not cause a routing outage."). ze doctor names the process, the configured unit, that the configuration may be out of date, and the exact fix, and Ze keeps working with that process" | agrees | Child 3 does not touch VPP systemd-unit ownership checks |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A fault occurs in the daemon, a plugin child, VPP or the kernel. Investigation enters through `show crashes` (CLI, offline fallback) or an MCP tool call (live daemon only).

### Transformation Path
1. Capture at fault time: runtime crash output file, `HandlePanic`, stderr relay, or kernel pstore.
2. Harvest at next start or boot: pending file or pstore record becomes `crash-*.log` in the crash directory.
3. Rotation keeps `ze.crash.keep` panic and kernel records together, and the same count of recovered records apart from them.
4. Read: `crashlog.CrashListFields` / `ReadCrash` feed the `show crashes` handler and the support `crashes` module.

### Boundaries Crossed
Process boundary (dead process to next start), boot boundary (kernel record), plugin process boundary (child stderr pipe), MCP transport (dispatcher command to JSON-RPC text result).

### Integration Points
`crashlog.Init` in `dispatchMain`; crashes plugin `BootEffect`; plugin process manager `ReportCrash` and failure policy; dispatcher command registry (MCP tool generation); support bundle modules; doctor checks.

### Architectural Verification
- [ ] New fault sources produce records through crashlog, never a parallel store
- [ ] Investigation surface is a registered command, so MCP gets it with no MCP code
- [ ] No hot-path allocation added (recording happens only on fault)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A clean-exit marker written at the `main` choke point plus four side exits covers every intended stop | Research of `cmd/ze/main.go`, `hub/main.go` | A clean stop is misfiled as a kill, flooding records | Test per exit path asserting no record after a clean stop | unvalidated |
| A-2 | Within the same boot the kernel's "Killed process" OOM line is still readable from `/dev/kmsg` at next start | Kernel ring behaviour | OOM cannot be told from other SIGKILLs | Lab test: OOM-kill ze in a Ze-image guest, restart, read record | unvalidated |
| A-3 | gok builds lack `vcs.revision` | Agent inference from build path | Source correlation already works | Read `debug.ReadBuildInfo` on an appliance image | unvalidated |
| A-4 | A plugin child launched via `ze plugin X` uses the parent's crash directory | Inherited environment | Child crash files land elsewhere or nowhere | Test: panic a child, list crashes in parent | unvalidated |
| A-5 | gokrazy pets the hardware watchdog, so whole-system hangs reboot but a wedged ze does not | gokrazy `runWatchdog` (init, `gokrazy.go`) opens `/dev/watchdog` and pings every 1s independently of ze | Hang design must own the hardware watchdog | Read gokrazy source at the pinned module version (2026-10-24) | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Recording on fault adds work on a path already failing (recovered panic in a hot loop floods the disk) | Crash dir growth under a panic loop test | Per-site rate limit and dedup by stack signature; count beyond the limit |
| R-2 | MCP returns multi-MB reports whole and breaks the client | Large `show crashes name` over MCP | Structured summary by default, paged sections on request |
| R-3 | Hang detector false positives on a busy box | Records on CI under load | Detector thresholds tied to observed stall, not wall time alone |
| R-4 | Shared rotation evicts kernel records | Kernel record missing after several panics | Per-kind retention |
| R-5 | Crash reports carry secrets (config text, peer passwords in panic values) and MCP exposes them | Grep of reports for config secrets | Reports pass through the support bundle's sanitizer before any read surface |
| R-6 | Appliance builds carry no commit or build tags, so every field stack points at unidentifiable code | `debug.ReadBuildInfo` on a built image shows no `vcs.revision` | Child 1 stamps identity into the gok build first and proves it on a built image |
| R-7 | The read-time parser misreads truncated or interleaved runtime output and presents a wrong faulting goroutine | Parser fixtures from real truncated dumps fail | Parser refuses what it cannot parse and returns the raw section with a stated reason, never a guess |
| R-8 | Renaming `show crashes` breaks callers (support bundle module, docs, functional tests, completion) | grep for `show crashes` / `ze-crashes` after the rename | Rename is one child-1 change with every consumer in the same commit (no alias, no-layering) |
| R-9 | Hang restart drops every BGP session on a false positive | Hang records on a loaded but healthy box | Threshold several times the heartbeat tick and tied to observed stall; scheduler latency only warns |
| R-10 | The external prober itself wedges or kills a healthy ze | Prober-initiated SIGQUIT records with healthy loops in the dump | Prober uses a long timeout (watchfrr default 90s) and requires consecutive misses |
| R-11 | The recorder pipe fills while the recorder is stalled, and the dying runtime blocks writing into it | Runtime dump truncated in a test with a paused recorder | Recorder reads into preallocated memory continuously and never does I/O on its read loop; pipe enlarged to the system maximum |
| R-12 | The recorder process dies or is never started, so records go nowhere | Doctor check; records missing after a forced recorder kill | ze detects the closed pipe and falls back to the pre-opened crash file; doctor check and diagnostic code for an absent recorder |
| R-13 | pmsg shares the ramoops reservation with kernel records, so one starves the other | Kernel record truncated after a pmsg write | Explicit split of the reserved region, sized in child 1 |
| R-14 | Preallocated memory is lazily mapped by the kernel, so the first write at fault time page-faults and fails under memory pressure (the OOM case) | Recorder RSS at start below its declared buffer size | `mlock` plus touching every page at start; doctor reports when `mlock` fails (RLIMIT_MEMLOCK) |
| R-15 | A later change adds an allocation on the fault path unnoticed | Allocation-count test goes red | Allocation tests for both paths are part of child 1 and stay in the unit suite |
| R-16 | The recorder's endpoint is a second network service and a second authentication path, so a defect there opens the box | Security review; endpoint reachable without credentials in a test | Read-only by construction (no handler mutates), same credential store and bcrypt check as the serial login, fail closed when the store is unreadable |
| R-17 | Serving requests starves or corrupts the fault path in the same process | Allocation test on the fault path fails while the endpoint is under load | Resolved by the process split (2026-10-25): serving runs in the fault reader, never in the core |
| R-18 | On Debian/Ubuntu `systemd-pstore.service` harvests and clears pstore at boot before ze runs, so ze finds an empty store and reports no kernel crash | Kernel panic on a Debian guest leaves no ze kernel record but a file under `/var/lib/systemd/pstore` | ze also reads systemd-pstore's archive; doctor reports which harvester owns pstore |
| R-19 | A stock Debian/Ubuntu kernel has no reserved ramoops region or pmsg, so the disk-failure fallback does not exist there | doctor on a Debian guest | doctor reports the fallback unarmed with the reason; never claimed armed |
| R-20 | The recorder's HTTP port collides with gokrazy's own web interface or another service | Bind failure at start | Port is a setting, refused at commit when taken; bind failure is a doctor finding, never silent |
| R-21 | The recorder dies and the daemon is adopted, so later deaths lose their exit status | Adopted state shown in the listing | Recorded as a degraded state; the recorder is kept small and its own crashes are records |
| R-22 | The fault reader cannot show a record the core has not yet persisted | A record visible in the core's queue but absent from the reader | Accepted: the window is the time to persist; the listing states the time of its last read |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Diagnostics only: a fault is unrecorded or misfiled. Moving supervision into the recorder parent changes restart behaviour so one Ze-owned restart policy runs across gokrazy and Debian/Ubuntu (plugin-fatal stops already exit 1) |
| How is it reverted? | Per child, single commit revert; no config migration (the env/YANG leaves added are new) |
| Who else touches this path? | crashlog, crashes plugin, plugin process manager, MCP tool generation, support bundle, appliance build |

## Wiring Test (MANDATORY -- NOT deferrable)

The umbrella owns no code; each row is delivered by the named child, whose spec refines the test name.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show faults` (CLI, daemon live) | → | faults command handler over the crashlog parser | `test/plugin/show-faults-structured.ci` (child 1) |
| `show faults` (CLI, daemon dead) | → | offline fallback over the same parser | `TestShowFaultsOfflineSameShape` (child 1) |
| MCP tool call for faults | → | generated tool from the registered command | `TestMCPFaultsToolPaged` (child 1) |
| SIGKILL of the daemon, then restart | → | harvest with clean-exit marker | `TestHarvestKilledProcessRecorded` (child 2) |
| Plugin child panics | → | process manager records exit status and trace | `test/plugin/fault-plugin-child-panic.ci` (child 3) |
| Recovered panic in a component | → | shared recovery helper | `TestRecoveredPanicRecorded` (child 4) |
| Daemon stalls | → | in-process watchdog writes hang record | `TestHangDetectorWritesDump` (child 5) |
| Prober gets no answer from a frozen ze | → | prober sends SIGQUIT, armed crash output captures the dump | `TestProberFrozenDaemonDumped` (child 5) |
| Goroutine count grows past threshold | → | leak watcher writes snapshot record | `TestLeakWatcherSnapshot` (child 6) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any recorded fault, listed over CLI or MCP | Listing shows kind, time, stack signature and the build (version, commit, modified flag, build tags) it came from |
| AC-2 | One record read over CLI or MCP | Answer is structured: panic value, faulting goroutine, other goroutines grouped by identical stack, log context with attributes, build identity; large records are paged by section |
| AC-3 | Daemon dead, record read over CLI | Same structured answer as with the daemon live |
| AC-4 | Daemon killed by SIGKILL or the OOM killer, then restarted | A record of kind killed or oom exists; a clean stop leaves none |
| AC-5 | A plugin marked fatal stops the daemon | Daemon exits non-zero, the recorder restarts it, and a record names the plugin and reason |
| AC-6 | A plugin child or VPP panics or is killed | A record carries its exit status and trace, whatever the relay setting |
| AC-7 | A panic is recovered at any recovery site | A record exists with value and stack; a repeating panic is deduplicated and counted, not written per occurrence |
| AC-8 | The daemon stops making progress beyond the hang threshold | A hang record with a full goroutine dump exists after restart |
| AC-9 | Goroutines, fds or heap grow past threshold | A leak record with a snapshot exists |
| AC-10 | Many Go panics after a kernel panic | The kernel record survives (per-kind retention) |
| AC-11 | Any read surface | Secrets that the support sanitizer removes are absent from the answer |
| AC-12 | The whole ze process freezes (no goroutine runs) | The external prober's SIGQUIT produces a runtime dump record of every goroutine, and the recorder restarts the daemon |
| AC-13 | A leak record is written | Ze keeps running; no restart |
| AC-14 | The crash directory's disk blocks every write | A fault is still recorded (in reserved RAM) and readable after the next boot; no ze goroutine waits on the disk |
| AC-15 | Recorded faults arrive faster than the recorder can write | Callers never block; the drop count is itself recorded and shown |
| AC-16 | The recorder process is dead | ze records to the pre-opened crash file and doctor reports the recorder missing |
| AC-17 | The recorder receives, frames and writes a record (disk or pmsg) | Zero heap allocations on that path, proven by an allocation-count test; the recorder's working memory is locked and resident from start |
| AC-18 | ze enqueues a record (recovered panic, hang dump, leak snapshot) | Zero heap allocations outside the panic value's own formatting, proven by an allocation-count test |
| AC-19 | The ze daemon is down (exited, crash-looping or never started) | An authenticated operator or AI reads the fault listing, single records and the recent log over the recorder's HTTP endpoint; an unauthenticated request is refused |
| AC-20 | ze dies at every start | A crash-loop record exists, and the first record of the loop survives however many restarts follow |
| AC-21 | ze crashes and the box reboots, with no remote syslog | The log lines leading up to the crash are readable after the reboot |
| AC-22 | The same fault scenarios on a gokrazy appliance and on a Debian/Ubuntu host under systemd | Same records, same listing, same HTTP endpoint; a capability the platform lacks (reserved RAM, pmsg) is reported unarmed by doctor, never silently absent |
| AC-23 | The daemon is killed by SIGKILL while the recorder is its parent | The record names SIGKILL from the wait status, not an inference |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An AI connects over MCP after a field crash and identifies the faulting code | MCP tool → faults command → parser → structured record with commit | `TestMCPFaultsToolPaged` (child 1) |
| 2 | Operator on a dead box reads why it died | CLI offline fallback → parser | `TestShowFaultsOfflineSameShape` (child 1) |
| 3 | Box OOMs; after restart operator sees why | harvest → kmsg OOM evidence → oom record → `show faults` | `TestHarvestKilledProcessRecorded` (child 2) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Per-child tests named in Wiring Test | each child spec | each AC | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Per-kind retention, hang threshold, leak thresholds, page size | defined in children 1, 5, 6 | child spec | child spec | child spec |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-faults-structured` | `test/plugin/` | Operator lists and reads a structured fault | |
| `fault-plugin-child-panic` | `test/plugin/` | A plugin panics; a record with its trace appears | |

### Interop Tests (Scope: protocol)
N-A: no wire-visible protocol change.

## Files to Modify
- `internal/core/crashlog/` - record grammar, parser, clean-exit marker, per-kind retention, lossy-spot fixes (children 1, 2)
- `internal/plugins/crashes/` - becomes the faults command owner; rename (child 1)
- `internal/core/slogutil/ring.go` - keep attributes (child 1)
- `cmd/ze/main.go`, `cmd/ze/hub/main.go` - clean-exit marker and `crashlog` fault record on plugin-fatal stop (child 2; exit 1 and `LastStopEntry` already landed)
- `internal/component/plugin/process/process.go`, `internal/component/vpp/vpp.go` - child exit records (child 3)
- `internal/component/plugin/process/manager.go` - `ReportCrash` carries the child's record (child 3)
- `internal/core/taskguard/guard.go`, `internal/core/crashlog/recovered.go` - dedup by stack signature and rate limit on the shared boundary (child 4)
- The 55 recovery sites that do not go through `taskguard`, counted on 2026-11-14 (child 4)
- `internal/component/mcp/tools.go` - output paging (child 1)
- `internal/appliance/`, `gokrazy/ze/config.json` - build identity stamping (child 1)
- `cmd/ze` start path (`cmd/ze/ze_core_start.go` `cmdStart`) - `ze start` becomes the recorder, which starts the daemon as its child (child 1)
- `internal/plugins/systemd/unit.go` - unit supervises the recorder; restart policy reviewed against the recorder owning daemon restarts (child 1)
- `internal/plugins/systemd/native.go` - native install of that unit (child 1)
- `docs/architecture/cli/plugin-modes.md` - systemd install describes the unit; updated for the recorder-as-parent start (child 1)
- `internal/core/crashlog/kernel_linux.go` - also read systemd-pstore's archive on Debian/Ubuntu (child 1)
- `docs/architecture/diagnostics/crash-capture.md`, `production-diagnostics.md` - per child
- `docs/guide/mcp/overview.md` - output paging and offline reach (child 1)
- `docs/architecture/api/process-protocol.md` - recorder-as-parent restart supervision (child 1) and plugin child exit handling if child 3 changes what the host observes
- `docs/architecture/system-architecture.md` - recorder-as-parent start and supervision (child 1)
- `docs/architecture/behavior/signals.md` - recorder-as-parent main PID forwarding `SIGHUP`/`SIGTERM` to the daemon child (child 1)
- `docs/architecture/fleet-config.md` - `ze show crashes` updated to `ze show faults` in `Connection Lifecycle` (child 1)

## Files to Create
- Child spec files (see Implementation Steps)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | faults command YANG replaces `ze-crashes-cmd.yang` (child 1); thresholds (children 5, 6) |
| YANG validation constraints | Yes | ranges on retention and thresholds (children 1, 5, 6) |
| YANG custom validators | No | native ranges suffice |
| CLI commands/flags | Yes | `show faults` (child 1) |
| CLI grammar (keyword before value) | Yes | `show faults name <n> section <s>` (child 1) |
| Editor autocomplete | Yes | record names need `CompleteFn` (child 1) |
| Functional test for new RPC/API | Yes | see Functional Tests |
| Pipe completeness | Yes | faults output through `ApplyPipes` (child 1) |
| Env var registration | Yes | `ze.crash.*` renamed with the concept if child 1 decides so; thresholds |
| Doctor check for runtime dependencies | Yes | recorder process alive and pmsg device present (child 1), `/dev/kmsg` read at start (child 2) |
| Doctor code per problem class | Yes | each child registers a doctor check, with its code, for each problem class it introduces: a hang restart, a crash loop, the adopted (degraded) recorder of R-21, queue drops of AC-15, a dead fault reader, a plugin-fatal stop. Standing rule (`ai/META-DECISIONS.md`, "Goal and scope"): `ze doctor` reports every problem an operator can hit, each with its cause and the command that fixes it |
| Prometheus counters/metrics | Yes | recovered panics by site, faults by kind (children 1, 4) |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP family change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/` crash capture entry (each child) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` (children 1, 5, 6) |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (child 1) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (child 1) |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` (child 1 rename) |
| 6 | Has a user guide page? | Yes | `docs/guide/appliance.md` |
| 7 | Wire format changed? | N-A | no wire change |
| 8 | Plugin SDK/protocol changed? | No | child exit handling is host-side (child 3 confirms) |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | no RFC |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/diagnostics/crash-capture.md` (every child), `production-diagnostics.md` (child 1: MCP paging, offline reach) |
| 13 | Route metadata keys added/changed? | N-A | |
| 14 | Prometheus counters added/changed? | Yes | subsystem telemetry doc (children 1, 4) |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md` (child 1 rename) |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived per child with `./le spec citation anchors` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | every `show crashes` example (child 1) |

## Design presentation

### What changes

Every fault Ze can suffer — unrecovered Go panic, runtime fatal, kernel panic, SIGKILL or OOM kill, plugin-fatal daemon stop, forked plugin or VPP crash, recovered panic, daemon hang or freeze, and resource leak — leaves a structured, sanitized, build-identified fault record that a human over the CLI or an AI over MCP can inspect after the fact, including while the daemon is down. Six child specs deliver this on top of `crashlog`: child 1 makes `ze start` an allocation-free, memory-locked recorder parent process that spawns and restarts the daemon, captures records and the recent ze log stream over a pipe (disk first, reserved-RAM pstore `pmsg` when the disk fails), spawns a separate authenticated read-only HTTP fault-reader process for down-daemon access, stamps build identity into appliance images, adds per-kind retention, replaces `show crashes` with structured paged sanitized `show faults`, and adds MCP output paging; children 2 through 6 add abnormal-exit detection (`killed`, `oom`, plugin-fatal stop record), child-process crash capture (forked plugins and VPP), deduplicated rate-limited recovered panics on `taskguard`, in-process and external-prober hang detection, and load-normalized resource leak snapshots. All fault scenarios are proven identically on gokrazy appliances and on booted Debian/Ubuntu VMs under systemd (per Thomas's 2026-11-24 decision).

### Before and after

1. **Listing and reading faults (CLI and MCP)**
   - Before: `show crashes` lists `name`, `size`, and `kind` (`panic`, `kernel`, `recovered`) and returns raw unsanitized text whole with no build version/tags and no log attributes; MCP returns multi-MB reports unpaged and cannot reach fault records when the daemon is down.
   - After: `show faults` replaces `show crashes` (no alias), lists `kind`, `time`, `stack signature`, and full build identity (version, commit, modified flag, build tags), answers structured sanitized sections (panic value, faulting goroutine, grouped goroutines, log context with attributes) paged over CLI and MCP, keeps identical shape offline, and remains readable over the fault reader's authenticated HTTP endpoint while the daemon is down.
2. **SIGKILL, OOM kill, and pre-crash logs**
   - Before: `harvestPending` deletes a header-only pending file as a clean exit, so a SIGKILL or OOM kill leaves no record; a Go runtime fatal carries no log ring and pre-crash logs vanish on reboot without remote syslog; a wedged disk blocks or loses runtime/panic writes.
   - After: the recorder parent observes the daemon's exit status and killing signal directly from `wait`, writes `killed` and `oom` records (using a clean-exit marker and `/dev/kmsg` OOM evidence), persists the recent ze log ring with every record, and falls back to reserved-RAM pstore `pmsg` without blocking ze or allocating on the fault path.
3. **Plugin child crashes, VPP exits, and recovered panics**
   - Before: forked plugin stderr is discarded when relay is off and produces no crash file; VPP exits log a line with no fault record; `taskguard` records recovered panics without stack-signature deduplication or rate limiting while 55 other `recover()` sites bypass it and `bfd` swallows one silently.
   - After: plugin child and VPP exits produce fault records with exit status and trace regardless of relay settings; every recovery site routes through `taskguard` with stack-signature deduplication and per-site rate limiting.
4. **Hangs and resource leaks**
   - Before: a hung or frozen daemon leaves no record and gokrazy's hardware watchdog does not detect a wedged ze; goroutine, fd, and heap leaks are exported only as Prometheus counters.
   - After: per-loop heartbeats plus the recorder's external SIGQUIT prober capture a full goroutine dump and restart the daemon on a stall or freeze; a load-normalized leak watcher records a snapshot without restarting ze.
5. **Retention and platform parity**
   - Before: Go panics evict kernel panic records under shared `ze.crash.keep`; systemd and gokrazy apply different restart policies and `systemd-pstore` clears pstore before ze runs on Debian/Ubuntu.
   - After: per-kind retention preserves kernel records and pins the first record of a crash loop; the recorder applies one Ze-owned restart policy on both gokrazy and Debian/Ubuntu, reads `systemd-pstore`'s archive, and `ze doctor` reports every unarmed capability or fault-path defect with its fix.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| One monolithic spec for all fault sources | Too large to close cleanly; child 1 delivers immediate value and fixes the record grammar and recorder boundary for children 2–6 |
| JSON written at capture time | The Go runtime and Linux kernel emit text Ze does not control, so a read-time parser is needed anyway and the raw file remains primary evidence |
| Keep `show crashes` or add a compatibility alias | Hangs, kills and leaks are not crashes; Ze is pre-release and follows one concept, one name with no layering alias |
| In-process writer goroutine only (no recorder process) | A faulting or OOMing process cannot reliably allocate or wait on a wedged disk, cannot survive its own death, and cannot reap its own exit status/signal |
| HTTP endpoint inside the recorder core (with soft memory limit) or MCP in the recorder | `net/http` allocates per request and shares heap/GC with the fault path (`debug.SetMemoryLimit` is soft); MCP pulls protocol machinery into a minimal survival process. Splitting recorder core and HTTP fault reader isolates the fault path |
| Restart on resource leak or fixed numeric leak thresholds | A restart erases leak evidence; fixed thresholds fail across deployments of different peer/route scale, whereas load-normalized rising ratios adapt |
| Hardware watchdog alone for hang detection | gokrazy's `runWatchdog` pets `/dev/watchdog` independently of ze, so it never detects a wedged ze control plane |
| Read-only MCP investigator role | Owner chose to keep full MCP access under existing authz so an AI may act during investigation |

### Documents departed from

- `docs/architecture/diagnostics/crash-capture.md` (`The decisions`, `Constraints`, `Recovered panics`): currently states that a header-only pending file came from a clean exit and is removed, that crash files are written only on panic detection without a clean-shutdown marker, that a runtime crash file carries no log ring, that recovered records are listed by `show crashes`, and that panic and kernel records share `ze.crash.keep`. After children 1, 2 and 4 (owner decisions of 2026-10-24 and 2026-10-25), the page describes the recorder pipe as primary runtime crash output with pre-opened file fallback, the clean-exit marker separating clean stops from `killed`/`oom`, the recorder-persisted pre-crash log ring on every record including runtime fatals, per-kind retention, deduplicated rate-limited recovered panics, and `show faults` replacing `show crashes`.
- `docs/architecture/diagnostics/production-diagnostics.md` (`Structured log fields`): currently states that crash reports keep their message-only recent-log header rather than adding structured fields. After child 1, the page states that fault reports include the structured attributes already stored in `slogutil.LogEntry.Attributes` (AC-2).
- `docs/architecture/system-architecture.md` (`How CLI Works`): currently states that `ze start config.conf` starts directly as the daemon. After child 1 (owner decision of 2026-10-25), the page states that `ze start` runs the recorder parent process, which starts, watches and restarts the daemon child and supervises the separate fault-reader HTTP process.
- `docs/architecture/hub-architecture.md` (`Startup Sequence`): currently states that the platform supervisor (systemd or gokrazy) restarts the daemon directly on exit 75 or nonzero exits. After child 1 (owner decision of 2026-10-25), the page states that the platform supervises only the recorder, and the recorder owns daemon restarts and crash-loop policy identically across gokrazy and Debian/Ubuntu.
- `docs/architecture/api/commands.md` (`Offline fallback (read-only commands without a daemon)`): currently names `show crashes` as the offline crash viewer. After child 1 (owner decision of 2026-10-24), the page names `show faults` (no alias) while keeping the shared online/offline parser parity contract.
- `docs/architecture/api/process-protocol.md` (`5-Stage Startup Protocol`, `The stop drain`): currently states that systemd (`Restart=on-failure` and exit 75) and gokrazy's supervision loop restart the daemon directly on failure or in-place restart. After child 1 (owner decision of 2026-10-25), the page states that the platform supervises only the recorder parent, and the recorder owns daemon restarts and crash-loop policy identically across gokrazy and Debian/Ubuntu (while keeping the existing exit-1 return and `LastStopEntry`, extended with a `crashlog` fault record in child 2).
- `docs/architecture/behavior/signals.md` (`Startup Paths`): currently states that systemd's `exec` keeps the `ze` daemon itself as the main PID for `ExecReload` and that gokrazy starts the daemon directly. After child 1 (owner decision of 2026-10-25), the page states that `ze start` runs the recorder parent as the main PID under systemd and gokrazy, spawning the daemon as its child and forwarding `SIGHUP`/`SIGTERM` to it.
- `docs/architecture/fleet-config.md` (`Connection Lifecycle`): currently states that a contained panic in a managed-client connection is recorded for `ze show crashes`. After child 1 (owner decision of 2026-10-24), the page names `ze show faults` (no alias).

### Choices for Thomas

none

## Implementation Steps

Each child is specced through `/ze-spec` in its own session, then implemented. Children 2-6 depend on child 1.

1. **Child 1: `spec-fault-investigation-1-record`** -- the recorder first (amended 2026-10-25): a separate recorder process that is the parent of the ze daemon (amended 2026-10-25: the platform, gokrazy or systemd, runs `ze start`, which is the recorder; the recorder starts, watches and restarts the daemon as its child, so the exit status and killing signal come straight from `wait`, and the restart and crash-loop policy is Ze's own and identical on gokrazy and Debian/Ubuntu; the daemon survives the recorder's death, and a restarted recorder adopts it through a pidfd from its recorded PID, after which a death is seen but its exit status is not, and that degraded state is itself recorded), holding preallocated memory, receiving every record over a pipe ze opens at start (the runtime crash output included, via `debug.SetCrashOutput` on the pipe, pipe enlarged with `F_SETPIPE_SZ`), writing to the crash directory when the disk allows and to pstore pmsg in the reserved RAM region when it does not (enable `CONFIG_PSTORE_PMSG`, split the reservation, harvest pmsg on next boot like kernel records); inside ze, recording only enqueues into a bounded preallocated queue drained by one goroutine, never blocks the caller, and counts drops; if the recorder is gone, ze falls back to the pre-opened crash file. The recorder process is also the child 5 prober. Zero allocation on the fault path (amended 2026-10-25): the recorder allocates every buffer it will ever use at start (pipe read area, record slots, the disk and pmsg write buffers), locks it into RAM with `mlock` and touches every page so no page fault or overcommit can fail later; receiving, framing and writing a record then performs no heap allocation, no formatting through `fmt`, and no parsing, because structuring happens only at read time. On the ze side the enqueue path writes into preallocated slots, and stacks are captured into a preallocated buffer; the one exception is the panic value's own `Error()`/`String()`, which is caller code and is bounded by copying at most a fixed number of bytes. Process split (amended 2026-10-25): the recorder is two processes. The recorder core (daemon parent, pipe reader, disk and reserved-RAM writer, crash-loop and liveness checks) holds no network code and allocates nothing after start; the rule is measured by allocation-count tests, not by avoiding the standard library, so allocation-free calls (raw read and write syscalls, atomics, copies into preallocated buffers) are allowed and any allocating call is refused by the test. The fault reader is a separate process the core starts and restarts; it serves the HTTP endpoint with `net/http`, reads only what the core has persisted, never talks to the core, and may allocate freely, so its death or exhaustion costs a response and never a record. Wherever this step says the recorder serves, read: the fault reader serves. Access while ze is down (amended 2026-10-25): the recorder serves a small read-only HTTP endpoint (HTTP only, amended 2026-10-25: the Go standard library carries it, MCP would pull a protocol implementation into the process that must stay minimal; an AI reads it as plain HTTP) exposing the fault listing, single records and the recent log, authenticated against the same local credential store the serial login reads read-only (`docs/architecture/appliance-serial-login.md`); serving runs off the fault path, is bounded by a memory limit, and never touches the preallocated fault-path memory. The recorder keeps the recent ze log stream in a preallocated ring and persists it with every record (disk, or reserved RAM when the disk fails), so pre-crash log survives a reboot without remote syslog. The recorder detects ze's death by the pipe closing, records it, detects a crash loop (N deaths in a window) as its own record, and pins the first record of a loop so rotation never evicts it; ze records the recorder's absence when it finds the pipe closed. Then: build identity stamped into appliance images (first, proven on a built image); declared record grammar and one read-time parser for every kind; lossy-spot fixes; ring keeps attributes; per-kind retention; `show faults` replacing `show crashes` with structured, paged, sanitized answers, offline included; MCP output paging and offline reach.
2. **Child 2: `spec-fault-investigation-2-abnormal-exit`** -- clean-exit marker at the `main` choke point and the four side exits; killed and oom kinds with boot id and kmsg evidence; `crashlog` stop-reason fault record on plugin-fatal stop alongside the existing exit-1 return and `LastStopEntry`.
3. **Child 3: `spec-fault-investigation-3-child-process`** -- plugin child and VPP exit status and trace become records, independent of the relay setting.
4. **Child 4: `spec-fault-investigation-4-recovered-panic`** -- the shared recovery helper exists (`internal/core/taskguard` `Task.Contain`, with its `ze_task_panics_total{component,task}` metric and its record through `crashlog.RecordRecoveredPanic`); this child extends it with dedup by stack signature and the rate limit, moves the remaining recovery sites onto it (55 on 2026-11-14), and fixes the silently swallowed bfd site.
5. **Child 5: `spec-fault-investigation-5-hang`** -- per-loop progress heartbeats checked by one in-process watchdog goroutine; first loops: `Reactor.metricsUpdateLoop` (canary for `r.mu`), dispatcher entry/exit, commit coordinator `runTxCoordinator`, plugin server `ManagedServer.serve`. On a stall past threshold: full goroutine dump into the crash directory as a hang record, then non-zero exit so the recorder restarts the daemon. `/health` gains a per-check timeout. Scheduler-latency signals warn only (FRR `event_call` model). The child 1 recorder process acts as the external prober (watchfrr model): it probes liveness and sends SIGQUIT on no answer, so a whole-process freeze is dumped by the armed crash output file. Appliance sets full traceback so every fatal records every goroutine. Thresholds set in the child spec.
6. **Child 6: `spec-fault-investigation-6-leak`** -- in-process sampler ring of per-resource ratios against load (goroutines per established peer plus running plugin, heap per route, fds per peer plus listener); a ratio rising over N consecutive windows with a flat denominator writes a leak record (goroutine profile grouped by stack, compressed heap profile, categorised `/proc/self/fd`, the triggering ratios) and Ze keeps running.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N is owned by exactly one child spec |
| Correctness | No surface answers "no faults" when it could not read the store |
| Data flow | Every new source writes through crashlog; no parallel store |
| Rule: no-layering | `show crashes` deleted when `show faults` lands, no alias |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Six child specs exist under `plan/release/fixes/` | `ls plan/release/fixes/spec-fault-investigation-*` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Record names refused with separators or `..` (preserved from today) |
| Resource exhaustion | Recording under a panic loop bounded by dedup and rate limit; parser bounded on kernel and runtime input |
| Error leakage | Secrets sanitized before any read surface (AC-11) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Umbrella with six children, child 1 first (owner, 2026-10-24) | One spec for everything | One spec would never close; child 1 is useful alone and fixes the shape every source writes into |
| Release bucket, P1 (owner, 2026-10-24) | Post-release | Undiagnosable field failures block shipping a production NOS; several gaps are defects |
| Text records with a declared section grammar, parsed on read (owner, 2026-10-24) | JSON written at capture | Runtime and kernel write text Ze cannot control, so a parser is needed anyway; the file stays the evidence |
| `show faults` replaces `show crashes` (owner, 2026-10-24) | Keep the old name | Hangs, kills and leaks are not crashes; one concept, one name; pre-release, no alias |
| MCP investigation keeps full access under existing authz (owner, 2026-10-24) | Read-only investigator role | Owner choice: the AI may act during investigation |
| Hang: record full dump, then exit non-zero so gokrazy restarts (owner, 2026-10-24) | Record only; configurable | A hung control plane serves nobody; BIRD `watchdog_sigalrm` aborts likewise. Costs every BGP session on a hang |
| Hang detection: in-process per-loop heartbeats plus an external prober (owner, 2026-10-24) | In-process only; hardware watchdog | gokrazy `runWatchdog` (init) holds `/dev/watchdog` and pets it independently of ze, so it never sees a wedged ze; the in-process checker misses a whole-process freeze, which the prober (watchfrr `wakeup_no_answer` model) catches via SIGQUIT into the armed crash file |
| Leak: record a snapshot, never restart (owner, 2026-10-24) | Restart past a ceiling | A leak degrades slowly and a restart erases the evidence |
| Leak signal is a load-normalised ratio rising over N windows, not a fixed threshold | Fixed thresholds | Goroutines scale with peers and heap with routes, so a fixed number is wrong on every deployment but one |
| Every record goes through an out-of-process recorder with preallocated memory, disk first, pstore pmsg in reserved RAM when the disk fails; part of child 1, and the same process is the child 5 prober (owner, 2026-10-25) | In-process writer goroutine only; a separate recorder child | A faulting process must never wait on a disk: a hang may itself be a wedged disk, and a recovered panic on a hot path must not stall behind I/O. A separate process survives ze's death and keeps writing; reserved RAM survives a warm reboot when no disk write is possible |
| No allocation while handling a fault: recorder memory allocated, locked and touched at start; ze's enqueue path writes into preallocated slots (owner, 2026-10-25) | Allocate on demand | A fault is often a memory fault: an OOM, a leak, a runtime that cannot grow the heap. Code that needs memory to record that it ran out of memory records nothing |
| The recorder is the parent of the daemon; the platform supervises only the recorder (owner, 2026-10-25) | Platform starts both; ze starts the recorder | Only a parent gets the exit status and killing signal; one restart policy in Ze's code instead of gokrazy's and systemd's, which both ignore exit 0; watchfrr and Erlang `heart` model. Costs: the recorder must be small enough never to die, and adoption after its death loses the exit status |
| Recorder core and fault reader are separate processes; the core allocates nothing after start and has no network code (owner, 2026-10-25) | HTTP inside the recorder with a soft memory limit; hand-written HTTP on preallocated buffers | `net/http` allocates per request and shares the heap and GC with the fault path; `debug.SetMemoryLimit` is a soft target, so a runtime out-of-memory would kill the fault path with it. Separate processes give hard isolation and keep the network surface away from the core |
| The fault-path rule is "no heap allocation, proven by allocation-count tests", not "no standard library" (owner, 2026-10-25) | Ban the standard library | The Go runtime is in every Go program, and many library calls allocate nothing; the test measures the property that matters |
| The recorder's endpoint is HTTP only (owner, 2026-10-25) | HTTP and MCP | HTTP is in the Go standard library; MCP is not, and the recorder must stay minimal |
| The recorder serves a read-only endpoint for records and logs while ze is down (owner, 2026-10-25) | Safe-mode ze; serial console and gokrazy page only | Works even when ze cannot start at all; costs a second network service, bounded by R-16 and R-17 |
| The recorder keeps and persists the recent ze log (owner, 2026-10-25) | Rely on kmsg and remote syslog | kmsg and the gokrazy ring are memory-only and lost on reboot |
| Appliance runs with full traceback | Default single-goroutine traceback | The goal is debugging every fault; a user panic otherwise records one goroutine |

Note: the 2026-10-24 hang row and the Blast Radius said gokrazy restarts the daemon; the 2026-10-25 recorder-as-parent row decides who restarts it, the recorder, as Implementation Step 5 already says.

## Known Limitations
- Whole-memory images (kdump) stay the existing opt-in amd64 feature; not extended here.

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
- [ ] The targeted tests for what changed pass: per child, `./le test unit` over the packages it reaches, `./le test functional plugin` for the `.ci` tests; GitHub CI verifies the pushed commit
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
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)
