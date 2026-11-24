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
| Updated | 2026-11-24 |

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
| AC-5 | child 2 (non-zero exit on plugin-fatal stop) | child 1 (the recorder restarts the daemon) |
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
| AC-22 | none yet: open point below | every child |
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

| Decision | What each answer changes |
|----------|--------------------------|
| AC-22, R-18 and R-19 need the fault scenarios proven on a stock Debian or Ubuntu host under systemd. The standing rule on test guests (`ai/META-DECISIONS.md`, "Test systems and labs") says: "VMs, test guests and the lab Docker host run Ze's own kernel and system, never Alpine or a stock distribution kernel." Its one exception: "Ze running on a standard distribution kernel can load the kernel modules it needs with `modprobe`, as an optional feature of `ze` itself." and "It acts only when the running kernel is not Ze's, and is proven on booted Debian, Ubuntu, Rocky and Fedora VMs: the one exception to Ze's kernel in test guests, and only for behaviour that exists off Ze's kernel." | Extend the exception to this feature: AC-22 is proven on booted Debian and Ubuntu VMs. Or prove the systemd path on Ze's own system, and record the stock-kernel behaviour (systemd-pstore, no reserved RAM) as unproven |

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
- [ ] `cmd/ze/hub/main.go` `runYANGConfig`, `exitForced`; `cmd/ze/main.go` `main`
  -> Constraint: `main` is the single choke point (dispatchMain, flushCrashlog, os.Exit) except `exitForced`, `startPprof`, `env.mustBeRegistered` and the reboot path, each of which needs its own clean-exit marker write.
  -> Constraint: `runYANGConfig` returns 0 whatever stopped the daemon, including a plugin `FailureFatal`; gokrazy restarts only on non-zero, so a fatal plugin leaves the appliance down with no record.
- [ ] `internal/component/mcp/tools.go` `run`
  -> Constraint: command output is returned whole with no size cap or paging; `ReadOnly` is set by the hub but never read by MCP; offline/local commands (support archive, offline `show crashes`) are not MCP tools.
- [ ] `internal/component/cmd/show/goroutines.go` `handleShowSystemGoroutines`
  -> Decision: a live goroutine dump already exists and is the building block for a hang record; it needs dispatch alive, so a hang detector must take its dump in-process, not via the command path.

### Architecture Docs
- [ ] `docs/guide/mcp/overview.md`
  -> Constraint: MCP serves dispatcher commands as tools; offline/local commands are not dispatched, so offline fault reading over MCP needs a route child 1 designs.
- [ ] `docs/architecture/api/process-protocol.md`
  -> Constraint: plugin child lifecycle is host-side; child 3 records exit status without changing the protocol unless it decides otherwise.
- [ ] `docs/architecture/system-architecture.md`
  -> Constraint: describes daemon stop and supervision; child 2's non-zero exit on plugin-fatal stop changes it.

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
| Daemon stop on plugin `FailureFatal` | No (A) | Exit 0, gokrazy does not restart | Appliance stays down, reason unrecorded |
| Plugin child crash or kill | Partly (V) | Engine log line; `ReportCrash` raises `doctor-plugin-crashed` carrying the recorded exit status (`p.exit.record`, `ExitStatus`); an internal plugin's panic is contained by `taskguard` and recorded | Trace discarded when relay off (`relayStderrFrom`); no crash file for a forked child (V) |
| Recovered panic | Partly (V) | Shared boundary `taskguard` `Task.Contain` (62 uses): ERROR line with value and stack, `ze_task_panics_total{component,task}`, `crash-<time>-recovered.log`. 55 other `recover()` sites recover on their own; 1 swallowed silently (bfd `trySendStateChange`) | No dedup by stack signature, no rate limit; of the 55 sites outside the boundary only the main-goroutine site (`cmd/ze/ze_core_dispatch.go`, `HandlePanic`) writes a record (V) |
| Hang, deadlock, wedged daemon | No (A) | Nothing; no watchdog, `/health` check has no timeout | Undetected, unrestarted, unrecorded |
| Goroutine, fd, memory leak | No (A) | Exported as Prometheus metrics only | Nothing watches a trend or threshold |
| VPP crash | Partly (A) | Log line with exit status, restarted with backoff | No record |

| Investigation surface | State |
|-----------------------|-------|
| `show crashes` list | name, size, kind; kind derived from name: `kernel`, `recovered`, and `panic` for every other Go kind (V: `crashlog.CrashKind`) |
| `show crashes latest / name` | Raw text; empty answer for both missing and unreadable file (A) |
| MCP | Generated tools reach `show crashes` live; no output cap (V: `run`); `CommandInfo.ReadOnly` is declared and never read (V); `request support archive` is a local-data command the daemon serves (V), so only offline `show crashes` with a dead daemon is unreachable |
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
| What breaks if this is wrong? | Diagnostics only: a fault is unrecorded or misfiled. The exit-code fix changes restart behaviour: on a plugin-fatal stop the recorder restarts the daemon |
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
- `cmd/ze/main.go`, `cmd/ze/hub/main.go` - clean-exit marker, non-zero exit on fatal stop (child 2)
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
- `docs/architecture/api/process-protocol.md` - plugin child exit handling, if child 3 changes what the host observes
- `docs/architecture/system-architecture.md` - exit-code and restart behaviour on plugin-fatal stop (child 2)

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

## Implementation Steps

Each child is specced through `/ze-spec` in its own session, then implemented. Children 2-6 depend on child 1.

1. **Child 1: `spec-fault-investigation-1-record`** -- the recorder first (amended 2026-10-25): a separate recorder process that is the parent of the ze daemon (amended 2026-10-25: the platform, gokrazy or systemd, runs `ze start`, which is the recorder; the recorder starts, watches and restarts the daemon as its child, so the exit status and killing signal come straight from `wait`, and the restart and crash-loop policy is Ze's own and identical on gokrazy and Debian/Ubuntu; the daemon survives the recorder's death, and a restarted recorder adopts it through a pidfd from its recorded PID, after which a death is seen but its exit status is not, and that degraded state is itself recorded), holding preallocated memory, receiving every record over a pipe ze opens at start (the runtime crash output included, via `debug.SetCrashOutput` on the pipe, pipe enlarged with `F_SETPIPE_SZ`), writing to the crash directory when the disk allows and to pstore pmsg in the reserved RAM region when it does not (enable `CONFIG_PSTORE_PMSG`, split the reservation, harvest pmsg on next boot like kernel records); inside ze, recording only enqueues into a bounded preallocated queue drained by one goroutine, never blocks the caller, and counts drops; if the recorder is gone, ze falls back to the pre-opened crash file. The recorder process is also the child 5 prober. Zero allocation on the fault path (amended 2026-10-25): the recorder allocates every buffer it will ever use at start (pipe read area, record slots, the disk and pmsg write buffers), locks it into RAM with `mlock` and touches every page so no page fault or overcommit can fail later; receiving, framing and writing a record then performs no heap allocation, no formatting through `fmt`, and no parsing, because structuring happens only at read time. On the ze side the enqueue path writes into preallocated slots, and stacks are captured into a preallocated buffer; the one exception is the panic value's own `Error()`/`String()`, which is caller code and is bounded by copying at most a fixed number of bytes. Process split (amended 2026-10-25): the recorder is two processes. The recorder core (daemon parent, pipe reader, disk and reserved-RAM writer, crash-loop and liveness checks) holds no network code and allocates nothing after start; the rule is measured by allocation-count tests, not by avoiding the standard library, so allocation-free calls (raw read and write syscalls, atomics, copies into preallocated buffers) are allowed and any allocating call is refused by the test. The fault reader is a separate process the core starts and restarts; it serves the HTTP endpoint with `net/http`, reads only what the core has persisted, never talks to the core, and may allocate freely, so its death or exhaustion costs a response and never a record. Wherever this step says the recorder serves, read: the fault reader serves. Access while ze is down (amended 2026-10-25): the recorder serves a small read-only HTTP endpoint (HTTP only, amended 2026-10-25: the Go standard library carries it, MCP would pull a protocol implementation into the process that must stay minimal; an AI reads it as plain HTTP) exposing the fault listing, single records and the recent log, authenticated against the same local credential store the serial login reads read-only (`docs/architecture/appliance-serial-login.md`); serving runs off the fault path, is bounded by a memory limit, and never touches the preallocated fault-path memory. The recorder keeps the recent ze log stream in a preallocated ring and persists it with every record (disk, or reserved RAM when the disk fails), so pre-crash log survives a reboot without remote syslog. The recorder detects ze's death by the pipe closing, records it, detects a crash loop (N deaths in a window) as its own record, and pins the first record of a loop so rotation never evicts it; ze records the recorder's absence when it finds the pipe closed. Then: build identity stamped into appliance images (first, proven on a built image); declared record grammar and one read-time parser for every kind; lossy-spot fixes; ring keeps attributes; per-kind retention; `show faults` replacing `show crashes` with structured, paged, sanitized answers, offline included; MCP output paging and offline reach.
2. **Child 2: `spec-fault-investigation-2-abnormal-exit`** -- clean-exit marker at the `main` choke point and the four side exits; killed and oom kinds with boot id and kmsg evidence; non-zero exit and stop-reason record on plugin-fatal stop.
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
