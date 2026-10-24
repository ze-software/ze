# Spec: fault-investigation-0-umbrella

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Priority | P1 |
| Release reason | A production NOS must leave an investigable record of every fault it suffers. Today a process killed by SIGKILL or the OOM killer is filed as a clean exit and leaves nothing, and a hang leaves nothing, so field failures cannot be diagnosed. |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Every fault Ze can suffer is recorded with enough context that an AI connected
over MCP, or a human over the CLI, can investigate it after the fact. Builds on
the existing crash capture (`docs/architecture/diagnostics/crash-capture.md`),
which already records Go runtime fatals, unrecovered panics and kernel panics.

Shape agreed at the SCOPE gate (2026-10-24): an umbrella with children, in the
release bucket at P1.

| Child | Owns |
|-------|------|
| 1 | One fault record model, and a structured CLI/MCP investigation surface over the records that already exist |
| 2..N | One child per new fault source: hang/deadlock, abnormal exit (SIGKILL, OOM, plugin child), recovered panic, resource leak |

The child list and boundaries are settled at the RESEARCH and DESIGN gates.

## Required Reading

- [ ] `docs/architecture/diagnostics/crash-capture.md`
  -> Constraint: the runtime writes fatal output with the world stopped, so only a file handed over by `debug.SetCrashOutput` at start can capture it; no in-process collector can.
  -> Constraint: kernel records are a second artifact kind in the same directory, rotation, listing and support module; a new fault source joins that model, never a parallel system.
- [ ] `internal/core/crashlog/crashout.go`
  -> Constraint: `harvestPending` deletes a pending file holding only its header as "a clean exit", but nothing marks a clean exit, so a SIGKILL or OOM kill is indistinguishable and its record is destroyed. This is a defect (silently wrong value), owned by the abnormal-exit child.
- [ ] `internal/plugins/crashes/cmd/show.go`
  -> Constraint: `show crashes` answers a list plus raw report text; MCP tools are generated from registered RPCs (`docs/architecture/diagnostics/production-diagnostics.md`), so a structured investigation surface reaches MCP by being a registered command, with no MCP code.
- [ ] `internal/core/slogutil/ring.go` `ringHandler.Handle`
  -> Constraint: a ring entry keeps time, level, subsystem and message only; every attribute is dropped, so the panic value and stack that recovered-panic log lines carry never reach the ring or any crash file.
- [ ] `internal/core/crashlog/stderr.go` `panicPattern`, `crashlog.go` `appendValue`
  -> Constraint: stderr detection opens on `goroutine N [running]:`, so the preceding `panic: <value>` line is never captured; `appendValue` writes `<unknown panic value>` for any value that is not a string or error. Both are silently lossy and owned by child 1.
- [ ] `internal/component/plugin/process/process.go` `monitorCmd`, `relayStderrFrom`
  -> Constraint: a plugin child's exit status is discarded (`_ = cmd.Wait()`); its stderr reaches the engine log only, and with relay disabled a panic trace is dropped.
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
| Main-goroutine panic | Yes (A) | `HandlePanic` crash file with header, stack, last 64 ring entries | Ring entries lack attributes; non-string panic values lost (V) |
| Panic trace printed to `os.Stderr` | Partly (V) | Relay detects and writes crash file | `panic: <value>` line lost (V) |
| Kernel panic or oops | Yes (A) | pstore record harvested on next boot | Shares rotation (`ze.crash.keep` 5) with Go panics, so panics evict kernel records (A) |
| SIGKILL, OOM kill, power loss, self reboot | No (V) | Header-only pending file deleted as "clean exit" | Indistinguishable from clean exit; no boot id; `oom_kill` in vmstat not compared at start (A) |
| Daemon stop on plugin `FailureFatal` | No (A) | Exit 0, gokrazy does not restart | Appliance stays down, reason unrecorded |
| Plugin child crash or kill | Partly (V) | Engine log line, doctor `plugin-crash` row | Exit status discarded; trace dropped when relay off; no crash file |
| Recovered panic (48 sites) | No (A) | slog only, 21 with stack, 14 value only, 6 to error, 4 callback; 1 swallowed silently (bfd) | No shared helper, no record, no count, ring drops value and stack |
| Hang, deadlock, wedged daemon | No (A) | Nothing; no watchdog, `/health` check has no timeout | Undetected, unrestarted, unrecorded |
| Goroutine, fd, memory leak | No (A) | Exported as Prometheus metrics only | Nothing watches a trend or threshold |
| VPP crash | Partly (A) | Log line with exit status, restarted with backoff | No record |

| Investigation surface | State |
|-----------------------|-------|
| `show crashes` list | name, size, kind; kind derived from name, `panic` covers every Go kind (A) |
| `show crashes latest / name` | Raw text; empty answer for both missing and unreadable file (A) |
| MCP | Generated tools reach `show crashes` live; no output cap (V), no read-only marking (A), offline commands unreachable (A) |
| Build identity | Appliance builds pass no ldflags, so version reads `dev`; `vcs.revision` likely absent in gok builds (I); image manifest in `/perm` never read by crash headers (A) |
| Supporting live inputs | Log ring (512, message only), event and FSM history, health, goroutine dump, pprof, kernel log: all live-daemon only (A) |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A fault occurs in the daemon, a plugin child, VPP or the kernel. Investigation enters through `show crashes` (CLI, offline fallback) or an MCP tool call (live daemon only).

### Transformation Path
1. Capture at fault time: runtime crash output file, `HandlePanic`, stderr relay, or kernel pstore.
2. Harvest at next start or boot: pending file or pstore record becomes `crash-*.log` in the crash directory.
3. Rotation keeps `ze.crash.keep` files across all kinds.
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
| R-9 | Hang restart drops every BGP session on a false positive | Hang records on a loaded but healthy box | Threshold several times the heartbeat tick and tied to observed stall; scheduler latency only warns |
| R-10 | The external prober itself wedges or kills a healthy ze | Prober-initiated SIGQUIT records with healthy loops in the dump | Prober uses a long timeout (watchfrr default 90s) and requires consecutive misses |
| R-8 | Renaming `show crashes` breaks callers (support bundle module, docs, functional tests, completion) | grep for `show crashes` / `ze-crashes` after the rename | Rename is one child-1 change with every consumer in the same commit (no alias, no-layering) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Diagnostics only: a fault is unrecorded or misfiled. The exit-code fix changes gokrazy restart behaviour (a plugin-fatal stop now restarts) |
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
| AC-5 | A plugin marked fatal stops the daemon | Daemon exits non-zero (gokrazy restarts it) and a record names the plugin and reason |
| AC-6 | A plugin child or VPP panics or is killed | A record carries its exit status and trace, whatever the relay setting |
| AC-7 | A panic is recovered at any recovery site | A record exists with value and stack; a repeating panic is deduplicated and counted, not written per occurrence |
| AC-8 | The daemon stops making progress beyond the hang threshold | A hang record with a full goroutine dump exists after restart |
| AC-9 | Goroutines, fds or heap grow past threshold | A leak record with a snapshot exists |
| AC-10 | Many Go panics after a kernel panic | The kernel record survives (per-kind retention) |
| AC-12 | The whole ze process freezes (no goroutine runs) | The external prober's SIGQUIT produces a runtime dump record of every goroutine, and gokrazy restarts ze |
| AC-13 | A leak record is written | Ze keeps running; no restart |
| AC-11 | Any read surface | Secrets that the support sanitizer removes are absent from the answer |

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
- 48 recovery sites (child 4)
- `internal/component/mcp/tools.go` - output paging (child 1)
- `internal/appliance/`, `gokrazy/ze/config.json` - build identity stamping (child 1)
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
| Doctor check for runtime dependencies | Yes | `/dev/kmsg` read at start (child 2), watchdog device if used (child 5) |
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

1. **Child 1: `spec-fault-investigation-1-record`** -- build identity stamped into appliance images (first, proven on a built image); declared record grammar and one read-time parser for every kind; lossy-spot fixes; ring keeps attributes; per-kind retention; `show faults` replacing `show crashes` with structured, paged, sanitized answers, offline included; MCP output paging and offline reach.
2. **Child 2: `spec-fault-investigation-2-abnormal-exit`** -- clean-exit marker at the `main` choke point and the four side exits; killed and oom kinds with boot id and kmsg evidence; non-zero exit and stop-reason record on plugin-fatal stop.
3. **Child 3: `spec-fault-investigation-3-child-process`** -- plugin child and VPP exit status and trace become records, independent of the relay setting.
4. **Child 4: `spec-fault-investigation-4-recovered-panic`** -- one shared recovery helper for all 48 sites, dedup by stack signature and rate limit, per-site metric; the silently swallowed bfd site fixed.
5. **Child 5: `spec-fault-investigation-5-hang`** -- per-loop progress heartbeats checked by one in-process watchdog goroutine; first loops: `Reactor.metricsUpdateLoop` (canary for `r.mu`), dispatcher entry/exit, commit coordinator `runTxCoordinator`, plugin server `ManagedServer.serve`. On a stall past threshold: full goroutine dump into the crash directory as a hang record, then non-zero exit so gokrazy restarts. `/health` gains a per-check timeout. Scheduler-latency signals warn only (FRR `event_call` model). An external prober (separate gokrazy package, watchfrr model) probes liveness and sends SIGQUIT on no answer, so a whole-process freeze is dumped by the armed crash output file. Appliance sets full traceback so every fatal records every goroutine. Thresholds set in the child spec.
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
| Appliance runs with full traceback | Default single-goroutine traceback | The goal is debugging every fault; a user panic otherwise records one goroutine |

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
- [ ] The targeted tests for what changed pass: per child, `./le test unit` over the packages it reaches, `./le test functional plugin` for the `.ci` tests, and `./le verify current fast`
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
