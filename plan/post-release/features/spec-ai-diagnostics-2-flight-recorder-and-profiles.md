# Spec: ai-diagnostics-2-flight-recorder-and-profiles

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli, plugin, docs |
| Priority | P3 |
| Release reason | Adds AI-readable profile summaries with mutex/block contention sampling, always-on pre-incident Go flight-recorder snapshots, a severity-split log ring, and richer event records so an AI assistant over MCP or CLI can diagnose unexpected bugs and lock wedges without waiting for a second reproduction; existing shipped behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | - |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-11-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Part 2 of the 5-part AI diagnostics architecture series (following closed `spec-ai-diagnostics-1-triage-and-fault-visibility`). Even with `show system triage` in place, four deep runtime-diagnosis blind spots remain when an unexpected bug, CPU spike, or lock wedge happens in production:

1. `show system profile` (`internal/component/cmd/show/profile.go`) supports only `cpu`, `heap`, `goroutine`, and `allocs` (omitting `mutex` and `block` contention profiles) and returns only an opaque `pprof-base64` binary blob that an AI assistant over MCP cannot read natively.
2. Go's stdlib `runtime/trace.FlightRecorder` is unused, so when an unexpected panic or slow plugin delivery happens, the preceding seconds of scheduler, goroutine, syscall, and GC execution trace are lost forever unless the operator can reproduce the bug under manual tracing.
3. `slogutil.LogRing` (`internal/core/slogutil/ring.go`) stores all log levels in one 512-entry circular buffer, so a burst of `info` or `debug` entries evicts earlier `warn` and `error` entries before triage or `show log recent level err` can inspect them.
4. `EventRing` (`internal/component/plugin/server/event_ring.go`) records only `{Timestamp, Namespace, EventType}`, omitting the event's source (`stampedSender`, emitter name, or `"engine"`), subject (`peerAddress`), and `direction` even though `deliverEventCaptured` already holds those strings in scope with zero allocation.

This spec solves all four gaps using Go's standard library alone (zero new external dependencies):
- Extend `show system profile` with `type mutex|block` and a command-owned `| summary` pipe operation (`command.RegisterPipeFilters` on `"show system profile"`) backed by a pure-stdlib gzipped pprof protobuf summary parser, and enable conservative low-overhead mutex/block sampling in daemon startup (`cmd/ze/hub/main.go`).
- Add core leaf package `internal/core/flightrec/` wrapping `runtime/trace.FlightRecorder` with a bounded in-memory snapshot ring, automatic cooldown-throttled capture on `taskguard` panic and slow plugin delivery, daemon lifecycle start/stop in `cmd/ze/hub/main.go` (non-fatal warning if start fails), and YANG commands `show system trace` and `request system trace snapshot`.
- Split `slogutil.LogRing` internally into separate `general` (`debug`/`info`) and `faults` (`warn`/`error`) circular buffers with a shared sequence counter so `info`/`debug` bursts never evict `warn`/`error` entries.
- Enrich `EventRecord` with `Source`, `Subject`, and `Direction`, populated zero-allocation in `deliverEventCaptured` and returned by `show event recent`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/diagnostics/procfs-diagnostics.md` - runtime profiling via `runtime/pprof`, CPU profile `TryLock` guard, goroutine modes
  → Decision: Extend `show system profile` in `internal/component/cmd/show/profile.go` with `mutex` and `block` profile types and a command-owned `| summary` pipe operation (`command.RegisterPipeFilters`) parsed by a pure-stdlib pprof protobuf reader (`profile_summary.go`), preserving bare `show system profile` (`pprof-base64`) as the default for existing callers and `test/plugin/system-profile-show.ci`.
  → Constraint: Do not vendor `github.com/google/pprof` or change the default `show system profile heap` payload shape.
- [ ] `docs/architecture/diagnostics/production-diagnostics.md` - single command registry feeding CLI and MCP, structured log ring, live triage
  → Decision: Register `show system trace` (`ze-cmd:show-system-trace`) and `request system trace snapshot` (`ze-cmd:request-system-trace-snapshot`) in `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` and `show.go`, so CLI, SSH, web, REST, gRPC, and MCP all expose them automatically.
  → Constraint: `container request > container system` in `ze-cli-show-cmd.yang` MUST carry byte-identical `description` and `ze:help` to `internal/plugins/host-cmd/yang/ze-host-request-cmd.yang` for merged-node parity.
- [ ] `docs/architecture/diagnostics/event-history-rings.md` - global event ring tapping `deliverEvent()`, zero-alloc overwrite-oldest append before subscriber dispatch
  → Decision: Add `Source`, `Subject`, and `Direction` string fields to `EventRecord` and `AppendWithMeta(...)` called at the existing pre-dispatch tap in `deliverEventCaptured` using strings already in scope.
  → Constraint: Keep append O(1), non-blocking, zero-allocation on the emit path, and strictly BEFORE subscriber dispatch.
- [ ] `docs/architecture/diagnostics/crash-capture.md` - `taskguard.Guard.record` recovered panic boundary
  → Decision: Call `flightrec.CaptureAuto("taskguard-panic", p.origin())` from `taskguard.Guard.record` so recovered panics automatically freeze the preceding seconds of Go execution trace (subject to cooldown).
  → Constraint: Never fail or delay panic recovery if flight recording is disabled or in cooldown; keep `internal/core/taskguard` importing only core leaf packages (`internal/core/flightrec`).
- [ ] `docs/architecture/api/commands.md` - verb-first command grammar, pipe rules, command-owned `RegisterPipeFilters`, dispatch keys as YANG paths
  → Decision: Register `summary` as a command-owned pipe operation on `"show system profile"` via `command.RegisterPipeFilters` (`Argument: command.ArgNone`) so `show system profile type <t> | summary` (and MCP `pipe: [{"name": "summary"}]`) transforms the captured pprof bytes inside `handleShowSystemProfile` via `validated.PipeOperations()` before rendering, keeping presentation strictly on the pipe side (`| summary`, `| summary | json`, `| summary | table`).
  → Constraint: Do not add a `leaf mode` presentation argument to `show system profile`; every handler returns structured `plugin.Map` with lowercase kebab-case keys, and refusals use `plugin.OperatorErrorResponse(opererr.New(...))`.

**Key insights:**
- Go `1.27.4` (`go.mod`) includes `runtime/trace.FlightRecorder` (`NewFlightRecorder`, `Start`, `Stop`, `Enabled`, `WriteTo`) in stdlib with zero external dependencies.
- `pprof.Profile.WriteTo(&buf, 0)` and `pprof.StartCPUProfile(&buf)` both emit the identical gzipped `perftools.profiles.Profile` protobuf format, so one small pure-stdlib protobuf reader (`compress/gzip` + `encoding/binary.Uvarint`) summarizes all 6 profile types (`cpu`, `heap`, `goroutine`, `allocs`, `mutex`, `block`) with zero external dependencies.
- Placing `internal/core/flightrec/` in `internal/core/` allows `internal/core/taskguard`, `internal/component/plugin/process`, `internal/component/cmd/show`, and `cmd/ze/hub` to share it without any upward import or cycle.
- Storing flight-recorder snapshots in a bounded in-memory ring (`4` snapshots x `4 MiB` max) avoids adding any new filesystem/runtime dependency and works identically on read-only appliances, containers, and tests.
- `cmd/ze/hub/main.go` `run(...)` is reached exclusively by the daemon process (never by offline CLI commands), making it the exact lifecycle owner for `flightrec.Start()` / `defer flightrec.Stop()` and conservative mutex/block sampling defaults.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cmd/show/profile.go` - implements `handleShowSystemProfile` (`cpu`, `heap`, `goroutine`, `allocs` only), returning `pprof-base64` with `cpuProfileMu.TryLock()`.
- [ ] `internal/component/cmd/show/profile_test.go` - tests `heap` wiring, `cpuProfileMu` contention, CPU duration bounds, and unknown arg fallback.
- [ ] `internal/component/cmd/show/goroutines_yang_test.go` - demonstrates `configyang.EnumValues(...)` parity check between YANG and Go handlers.
- [ ] `internal/component/cmd/show/show.go` - registers `ze-cmd:show-*` RPCs in `init()` and implements `handleShowEventRecent` emitting `{timestamp, namespace, event-type}`.
- [ ] `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` - declares `show > system > profile` (`type` enum of 4 values, `duration` leaf) and sibling `show > system` commands.
- [ ] `internal/plugins/host-cmd/yang/ze-host-request-cmd.yang` - declares `container request > container system` with `description` and `ze:help` that any merging module must match byte-for-byte.
- [ ] `internal/core/slogutil/ring.go` - implements `LogRing` with one 512-entry circular buffer shared across all levels, plus `ringHandler` and `GlobalLogRing()`.
- [ ] `internal/core/slogutil/ring_test.go` - verifies `Recent`, `Snapshot`, overflow, `ringHandler.Enabled`, concurrent attributes, and reentrant values.
- [ ] `internal/component/plugin/server/event_ring.go` - implements `EventRecord{Timestamp, Namespace, EventType}` and `EventRing` (`Append`, `Snapshot`, `Count`, `NamespaceCounts`, `SetOnAppend`).
- [ ] `internal/component/plugin/server/dispatch.go` - `deliverEventCaptured` calls `s.eventRing.Append(namespace, eventType)` at line 409 before subscriber dispatch, while already holding `stampedSender`, `emitter`, `peerAddress`, and `direction`.
- [ ] `internal/component/plugin/process/doctor_records.go` - `Process.noteBatch` detects slow plugin delivery (`DeliverySlow(elapsed)`, > 500ms) and logs `code=CodeDoctorPluginDeliveryFailed`.
- [ ] `internal/core/taskguard/guard.go` - `Guard.record` increments panic metric, writes crash file, raises `report.CodeInternalDefect`, and logs `ERROR`.
- [ ] `cmd/ze/hub/main.go` - `run(...)` boots the daemon after `hardSetupFailure()` passes.
- [ ] `cmd/ze/hub/service_web.go` - `wireEventRingToBroker` consumes `EventRecord` via `SetOnAppend`.
- [ ] `internal/plugins/skills/data/ze-diagnostics.md` - live runtime triage playbook referencing `show system profile` and `show event recent`.

**Behavior to preserve:**
- `show system profile` without `| summary` (e.g., `show system profile heap`, `show system profile type cpu duration 2s`) keeps returning the exact existing `format: "pprof-base64"` and `data` fields (`test/plugin/system-profile-show.ci` passes unchanged).
- `show system profile` CPU concurrency guard (`cpuProfileMu.TryLock()`) and duration bounds (`1s..60s`, default `10s`) stay unchanged.
- `slogutil.ringHandler.Enabled` keeps delegating to `inner.Enabled` (`TestRingHandlerEnabled` passes unchanged); `LogRing.Recent` and `LogRing.Snapshot` signatures, newest-first ordering, level-spelling aliases (`err`/`error`/`warning`), and invalid-level refusal stay unchanged.
- `EventRing.Append(namespace, eventType)` and `SetOnAppend(func(EventRecord))` stay compatible for existing callers and `wireEventRingToBroker`.
- Offline CLI commands (`ze version`, `ze config validate`, etc.) do NOT start `runtime/trace.FlightRecorder` or enable mutex/block profiling.
- `taskguard` with no panic stays completely silent and zero-cost; `taskguard` with flight recorder stopped still records panics cleanly with no error.

**Behavior to change:**
- Add `internal/core/flightrec/` wrapping `runtime/trace.FlightRecorder` with bounded in-memory snapshot ring (`maxSnapshots = 4`, `maxSnapshotBytes = 4 << 20`, `captureCooldown = 30 * time.Second`), `Start`/`Stop`/`Status`/`Snapshots`/`SnapshotByID`/`CaptureAuto`/`CaptureNow`.
- Wire `flightrec.Start()` (logging a warning and continuing daemon startup if it ever errors) / `defer flightrec.Stop()` and conservative mutex/block profiling (`runtime.SetMutexProfileFraction(10)`, `runtime.SetBlockProfileRate(100_000)`) in `cmd/ze/hub/main.go` `run(...)`.
- Wire `flightrec.CaptureAuto("taskguard-panic", p.origin())` in `internal/core/taskguard/guard.go` `Guard.record` and `flightrec.CaptureAuto("slow-plugin-delivery", p.config.Name)` in `internal/component/plugin/process/doctor_records.go` `Process.noteBatch`.
- Add `show system trace` (`ze-cmd:show-system-trace`) and `request system trace snapshot` (`ze-cmd:request-system-trace-snapshot`) in `internal/component/cmd/show/trace.go`, `show.go`, `yang/ze-cli-show-cmd.yang`, and `wire-methods.snapshot`.
- Extend `show system profile` in `internal/component/cmd/show/profile.go`, `profile_summary.go`, and `yang/ze-cli-show-cmd.yang` with `type mutex|block` and command-owned `| summary` pipe operation (`command.RegisterPipeFilters` on `"show system profile"`, pure-stdlib pprof protobuf summary parser).
- Split `slogutil.LogRing` (`internal/core/slogutil/ring.go`) into `general` (`debug`/`info`) and `faults` (`warn`/`error`) circular buffers with shared sequence numbers.
- Enrich `EventRecord` (`internal/component/plugin/server/event_ring.go`, `dispatch.go`, `internal/component/cmd/show/show.go`) with `Source`, `Subject`, and `Direction`.
- Update `internal/plugins/skills/data/ze-diagnostics.md` and architecture/command docs.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Runtime profile command & command-owned pipe filters | `internal/component/cmd/show/profile.go` (`handleShowSystemProfile`, `profileCPU`, `profileSnapshot`), `ze-cli-show-cmd.yang` (`container profile`), `internal/component/command/pipe_filter.go` (`RegisterPipeFilters`) | Captures `cpu`, `heap`, `goroutine`, `allocs` profiles as `pprof-base64`; commands declare command-owned pipe operations via `RegisterPipeFilters` read in handlers via `ValidatedArgs.PipeOperations()`. | Extends: adds `mutex` and `block` types and declares `\| summary` via `command.RegisterPipeFilters` on `"show system profile"` backed by a pure-stdlib pprof protobuf summary decoder while preserving bare `pprof-base64` default. |
| Shared command registry and YANG tree | `internal/component/cmd/show/show.go` (`init`), `ze-cli-show-cmd.yang`, `ze-host-request-cmd.yang` (`container request > container system`) | Exposes process-global daemon commands uniformly over CLI, SSH, web, REST, gRPC, and MCP. | Extends: registers `show system trace` (`ze-cmd:show-system-trace`) and `request system trace snapshot` (`ze-cmd:request-system-trace-snapshot`) under the same YANG tree with byte-identical merged container text. |
| Global log ring | `internal/core/slogutil/ring.go` (`LogRing`, `ringHandler`, `GlobalLogRing`) | Stores 512 recent log records across all levels in one circular buffer. | Extends: splits storage inside `LogRing` into `general` (512 `debug`/`info`) and `faults` (256 `warn`/`error`) rings merged by sequence number so `info`/`debug` bursts cannot evict `warn`/`error` entries. |
| Global event history ring | `internal/component/plugin/server/event_ring.go` (`EventRing`, `EventRecord`), `dispatch.go` (`deliverEventCaptured`), `show.go` (`handleShowEventRecent`) | Records `{Timestamp, Namespace, EventType}` before subscriber dispatch for `show event recent` and web SSE. | Extends: adds `Source`, `Subject`, and `Direction` to `EventRecord` and `AppendWithMeta`, filled zero-allocation in `deliverEventCaptured` and emitted by `show event recent`. |
| Panic containment & slow-delivery doctor records | `internal/core/taskguard/guard.go` (`Guard.record`), `internal/component/plugin/process/doctor_records.go` (`Process.noteBatch`) | Records recovered panics and slow plugin delivery batches (> 500ms). | Extends: triggers cooldown-throttled `flightrec.CaptureAuto` on both events so pre-incident execution traces are preserved automatically. |
| Daemon startup lifecycle | `cmd/ze/hub/main.go` (`run`) | Boots the daemon process alone (never reached by offline CLI verbs). | Extends: starts/defers-stop on `flightrec` and sets conservative mutex/block profiling rates once at daemon start. |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | system door | "The CLI, SSH, the web UI, REST, gRPC and MCP dispatch into the same command registry." | agrees | `show system profile` extensions, `show system trace`, and `request system trace snapshot` register once in YANG + `internal/component/cmd/show`, reaching every interface with zero MCP edits. |
| `ai/doors/core.md` | Boundaries | `internal/core/flightrec/flightrec.go`, `internal/core/slogutil/ring.go`, `internal/core/taskguard/guard.go` | "Never import upward." | agrees | `internal/core/flightrec` imports Go stdlib only; `internal/core/taskguard` imports only sibling core package `internal/core/flightrec`. |
| `ai/doors/commands-cli.md` | Boundaries | `internal/component/cmd/show/profile.go`, `internal/component/cmd/show/trace.go`, `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` | "A handler answers data, never text." | agrees | Every profile summary, trace metadata/payload, and enriched event record returns structured `plugin.Map` with lowercase kebab-case keys routed through `ApplyPipes`. |
| `ai/doors/hub-engine.md` | Boundaries | `cmd/ze/hub/main.go` | "No always-on file imports a gated package." | agrees | `cmd/ze/hub/main.go` imports only always-on core leaf package `internal/core/flightrec` and stdlib `runtime`. |
| `ai/doors/plugin-infra.md` | Where it sits | `internal/component/plugin/server/event_ring.go`, `internal/component/plugin/server/dispatch.go`, `internal/component/plugin/process/doctor_records.go`, `internal/component/plugin/all/testdata/wire-methods.snapshot` | "It is infrastructure and cannot be compiled out." | agrees | Enriching `EventRecord`, triggering `flightrec.CaptureAuto` on slow delivery, and adding two wire methods to the snapshot keep plugin infrastructure self-contained and BGP-free. |
| `ai/doors/system-plugins.md` | Boundaries | `internal/plugins/skills/skills_test.go` | "A plugin owns its whole surface." | silent | Only embedded Markdown guidance in `internal/plugins/skills/data/ze-diagnostics.md` and its test are updated; plugin boundaries and commands are unchanged. |
| `ai/doors/le-tooling.md` | Boundaries | `internal/test/fixture/plugin_system_profile_summary_show.go`, `internal/test/fixture/plugin_system_trace_show.go`, `test/plugin/system-profile-summary-show.ci`, `test/plugin/system-trace-show.ci` | "A compiled Go fixture returns an error when an assertion fails" | agrees | Both new `.ci` functional tests and compiled Go fixtures drive the real daemon and return errors on any schema or value failure. |
| `docs/architecture/hub-architecture.md` | Overview | `cmd/ze/hub/main.go` | "" | silent | Starting/stopping `flightrec` and setting mutex/block profiling defaults in `cmd/ze/hub/main.go` `run` adds no plugin or config startup phase change. |
| `docs/architecture/api/architecture.md` | Design decisions | `cmd/ze/hub/main.go` | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Does not touch `\| implied` or config display, preserving inline `# implied:` behavior unchanged. |
| `docs/architecture/api/architecture.md` | Design decisions | `cmd/ze/hub/main.go` | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | Does not touch bus participant naming or registration defaults. |
| `docs/architecture/api/architecture.md` | Design decisions | `cmd/ze/hub/main.go` | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Does not touch `send > to` routing. |
| `docs/architecture/api/architecture.md` | Design decisions | `cmd/ze/hub/main.go` | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Does not touch `receive > from` filtering. |
| `docs/architecture/api/architecture.md` | Design decisions | `cmd/ze/hub/main.go` | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Does not touch bus send/receive allow-lists. |
| `docs/architecture/api/architecture.md` | Design decisions | `cmd/ze/hub/main.go` | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Does not touch bus filter compilation. |
| `docs/architecture/api/architecture.md` | Design decisions | `cmd/ze/hub/main.go` | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Does not touch bus participant arbitration. |
| `docs/architecture/api/architecture.md` | Design decisions | `cmd/ze/hub/main.go` | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Does not touch bus delivery display. |
| `docs/architecture/api/commands.md` | Design decisions | `internal/component/cmd/show/yang/ze-cli-show-cmd.yang`, `internal/component/cmd/show/show.go` | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | `show system profile`, `show system trace`, and `request system trace snapshot` carry no presentation/format arguments; profile summarization is registered as a command-owned pipe operation (`\| summary`) via `command.RegisterPipeFilters` on `"show system profile"`, read inside `handleShowSystemProfile` via `validated.PipeOperations()` and composable with `\| json`, `\| yaml`, and `\| table`. |
| `docs/architecture/api/commands.md` | Design decisions | `internal/component/cmd/show/yang/ze-cli-show-cmd.yang`, `internal/component/cmd/show/show.go` | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | None of the new or extended commands overrides any generic pipe operation. |
| `docs/architecture/api/commands.md` | Design decisions | `internal/component/cmd/show/yang/ze-cli-show-cmd.yang`, `internal/component/cmd/show/show.go` | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | `show system profile`, `show system trace`, and `show event recent` remain strictly read-only, while `request system trace snapshot` is an explicit `request` action verb before any pipe runs. |
| `docs/architecture/api/process-protocol.md` | Design decisions | `internal/component/plugin/server/dispatch.go`, `internal/component/plugin/process/doctor_records.go` | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | `deliverEventCaptured` only records existing in-scope strings into `EventRing` and preserves engine operation authorization and wire framing unchanged. |
| `docs/architecture/api/process-protocol.md` | Design decisions | `internal/component/plugin/server/dispatch.go`, `internal/component/plugin/process/doctor_records.go` | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Triggering `flightrec.CaptureAuto` in `Process.noteBatch` runs after slow-batch observation and never drops or delays queued plugin events. |
| `docs/architecture/appliance/remote-operations.md` | Decisions | `cmd/ze/hub/main.go` | "" | silent | Does not change remote config push, SSH upload, or appliance OTA operations. |
| `docs/architecture/behavior/signals.md` | Signal Handling | `cmd/ze/hub/main.go` | "" | silent | Does not change SIGHUP/SIGTERM/SIGINT handling; `defer flightrec.Stop()` runs on normal daemon shutdown. |
| `docs/architecture/cli/command-completion.md` | The producers | `cmd/ze/hub/main.go` | "" | silent | Completion derives automatically from the updated YANG command tree. |
| `docs/architecture/config/syntax.md` | Overview | `cmd/ze/hub/main.go` | "" | silent | Does not change configuration syntax or parsing. |
| `docs/architecture/config/yang-config-design.md` | 1. Key Principle | `cmd/ze/hub/main.go` | "" | silent | Does not change config YANG schemas or validation. |
| `docs/architecture/core-design.md` | Design decisions | `cmd/ze/hub/main.go`, `internal/component/cmd/show/show.go` | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Does not touch kernel/VPP dataplane compatibility rules. |
| `docs/architecture/core-design.md` | Design decisions | `cmd/ze/hub/main.go`, `internal/component/cmd/show/show.go` | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Does not touch kernel/VPP config mismatch verdicts. |
| `docs/architecture/core-design.md` | Design decisions | `cmd/ze/hub/main.go`, `internal/component/cmd/show/show.go` | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Does not touch VPP firewall verification. |
| `docs/architecture/core-design.md` | Design decisions | `cmd/ze/hub/main.go`, `internal/component/cmd/show/show.go` | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Does not touch data-plane backend selectors. |
| `docs/architecture/core-design.md` | Design decisions | `cmd/ze/hub/main.go`, `internal/component/cmd/show/show.go` | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Does not touch BGP next-hop rewriting. |
| `docs/architecture/core-design.md` | Design decisions | `cmd/ze/hub/main.go`, `internal/component/cmd/show/show.go` | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Does not touch BGP policy next-hop forms. |
| `docs/architecture/core-design.md` | Design decisions | `cmd/ze/hub/main.go`, `internal/component/cmd/show/show.go` | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Does not touch BGP policy chain compilation. |
| `docs/architecture/core-design.md` | Design decisions | `cmd/ze/hub/main.go`, `internal/component/cmd/show/show.go` | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Does not touch BGP export next-hop folding. |
| `docs/architecture/core-design.md` | Design decisions | `cmd/ze/hub/main.go`, `internal/component/cmd/show/show.go` | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Does not touch BGP import/export next-hop validation. |
| `docs/architecture/core-design.md` | Design decisions | `cmd/ze/hub/main.go`, `internal/component/cmd/show/show.go` | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Does not touch BGP route origination next-hop handling. |
| `docs/architecture/core-design.md` | Design decisions | `cmd/ze/hub/main.go`, `internal/component/cmd/show/show.go` | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Does not touch implied filter chains. |
| `docs/architecture/core-design.md` | Design decisions | `cmd/ze/hub/main.go`, `internal/component/cmd/show/show.go` | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | If `flightrec.Start()` ever returns an error at daemon boot, `cmd/ze/hub/main.go` logs a warning and continues starting Ze — flight recording never prevents the daemon from starting. |
| `docs/architecture/core-design.md` | Design decisions | `cmd/ze/hub/main.go`, `internal/component/cmd/show/show.go` | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Does not touch kernel/VPP startup policy. |
| `docs/architecture/diagnostics/crash-capture.md` | Recovered panics | `internal/core/taskguard/guard.go`, `docs/architecture/diagnostics/crash-capture.md` | "A recovered panic is never silent." | agrees | Triggering `flightrec.CaptureAuto` adds pre-incident execution trace capture alongside the existing ERROR log, panic metric, crash record, and report-bus raise, and `crash-capture.md` documents it. |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | `internal/component/plugin/process/doctor_records.go`, `internal/component/plugin/server/dispatch.go` | "" | silent | `Process.noteBatch` keeps its exact slow-delivery threshold, doctor code, and window accounting unchanged while additionally triggering `flightrec.CaptureAuto`. |
| `docs/architecture/fleet-config.md` | Design decisions | `cmd/ze/hub/main.go` | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | Does not touch fleet managed listener certificates. |
| `docs/architecture/fleet-config.md` | Design decisions | `cmd/ze/hub/main.go` | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | Does not touch fleet device web provisioning. |
| `docs/architecture/pki/pki-store.md` | Decisions | `cmd/ze/hub/main.go` | "" | silent | Does not touch PKI certificate storage. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | `cmd/ze/hub/main.go` | "The looking glass may serve plain HTTP, because what it publishes is public, and one looking glass serves HTTP and HTTPS at the same time. Thomas's words, as voice-transcribed: "We should allow the looking glass to be HTTP. The information is public." and "We should be able to serve on both HTTP and HTTPS, not either." Asked where the choice lives and what happens to `ze.looking-glass.tls`, he answered "1a 2a": the choice is the `tls` leaf of each `server` entry, default true, and the environment variable is removed. The no-store rule applies to each entry: an entry that left `tls` unset serves HTTP with a warning on a daemon with no `database/` store and no named certificate, an entry that says `tls true` there is refused at start and at reload, and a reload does what a restart with the same config does" | agrees | Does not touch looking-glass HTTP/HTTPS listeners. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | `cmd/ze/hub/main.go` | "The web and looking glass use the shared `listenerTLSMaterial` precedence. The approved Fleet reload design shares `managedCertificate` with startup because the daemon authority issues its unnamed leaf." | agrees | Does not touch TLS material precedence. |
| `docs/architecture/pki/tls-listeners.md` | Design decisions | `cmd/ze/hub/main.go` | "A certificate that cannot be presented never stops ze from starting. A web server whose certificate name does not resolve is not started. The looking glass is judged one `server` entry at a time: an entry that says `tls true` with no certificate to present stays down, every entry that asks for TLS stays down when the named certificate does not resolve, and the other entries serve. Each is reported with its fix by an error line in the log under a doctor code and by `ze doctor`. A reload whose config cannot be served stays refused. This supersedes the start half of the row of 2026-11-23 above, which says a `tls true` entry "is refused at start": at start that entry stays down and the others serve, and at reload the config is still refused. Thomas's words, as voice-transcribed: "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; "reload stay refused"; "Should only that endpoint stay down while the others serve: yes"" | agrees | Does not touch TLS listener startup degradation. |
| `docs/architecture/storage-backends.md` | Opening and ownership | `cmd/ze/hub/main.go` | "" | silent | Does not touch ZeFS/file storage backends; flight recorder snapshots stay in bounded memory. |
| `docs/architecture/system-architecture.md` | Overview | `cmd/ze/hub/main.go` | "" | silent | Keeps the single-daemon architecture and tier direction intact. |
| `docs/architecture/testing/ci-format.md` | Overview | `cmd/ze/hub/main.go`, `internal/component/plugin/server/dispatch.go` | "" | silent | Reuses existing `.ci` format and compiled fixture pattern without changing the runner. |
| `docs/architecture/web-interface.md` | Source Files | `cmd/ze/hub/main.go` | "" | silent | Does not touch web UI routes or headers; `wireEventRingToBroker` remains compatible with `EventRecord`. |
| `docs/architecture/diagnostics/production-diagnostics.md` | CLI first, MCP for free | `internal/component/cmd/show/trace.go`, `internal/core/slogutil/ring.go`, `docs/architecture/diagnostics/production-diagnostics.md` | "Every diagnostic is a YANG RPC registered as a CLI command. The MCP tool list is generated from the registered RPCs, so a new diagnostic appears to an AI agent with no MCP code change." | agrees | `show system trace`, `request system trace snapshot`, and `show system profile` extensions register via YANG + `pluginserver.RegisterRPCs` with zero MCP changes, and `production-diagnostics.md` is updated. |
| `docs/architecture/diagnostics/event-history-rings.md` | The decisions | `internal/component/plugin/server/event_ring.go`, `internal/component/plugin/server/dispatch.go`, `docs/architecture/diagnostics/event-history-rings.md` | "Fixed-size circular buffer, overwrite-oldest, non-blocking append." | agrees | `EventRing.AppendWithMeta` stays fixed-size, overwrite-oldest, non-blocking, zero-allocation, and still runs before subscriber dispatch; `event-history-rings.md` is updated for the new metadata fields. |
| `docs/architecture/plugin/rib-storage-design.md` | Overview | `internal/component/plugin/server/dispatch.go` | "" | silent | Does not touch RIB storage or route structures. |
| `docs/architecture/diagnostics/procfs-diagnostics.md` | The decisions | `internal/component/cmd/show/profile.go`, `docs/architecture/diagnostics/procfs-diagnostics.md` | "The CPU profile guard is `sync.Mutex.TryLock()`." | agrees | `profileCPU` keeps `cpuProfileMu.TryLock()` unchanged with or without `\| summary`, and `procfs-diagnostics.md` is updated to document `mutex`/`block` types, `\| summary`, and `flightrec`. |
| `docs/features/ai-first.md` | Bundled Skills | `internal/plugins/skills/skills_test.go` | "The installed binary serves the workflow guides embedded when it was built." | agrees | Updating `internal/plugins/skills/data/ze-diagnostics.md` keeps version-matched profile summary and trace snapshot guidance inside the binary. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- **Profile query:** Operator or AI runs `show system profile [type cpu|heap|goroutine|allocs|mutex|block] [duration 1s..60s] [| summary]` over CLI, SSH, web, REST, gRPC, or MCP (`ze_show_system` action `profile`, with `pipe: [{"name": "summary"}]`).
- **Flight recorder lifecycle & triggers:** `cmd/ze/hub/main.go` `run(...)` calls `flightrec.Start()` (logging a warning and continuing daemon startup if `Start()` ever errors) / `defer flightrec.Stop()` and sets mutex/block profiling rates; `taskguard.Guard.record` and `process.Process.noteBatch` (on slow delivery) call `flightrec.CaptureAuto(trigger, detail)`; operator or AI runs `request system trace snapshot` (`flightrec.CaptureNow`) or `show system trace [id <id>]`.
- **Log capture:** Existing `slogutil.ringHandler.Handle` appends entries to `slogutil.LogRing`, now routed by severity into `faults` (`warn`/`error`) or `general` (`debug`/`info`).
- **Event history:** `plugin/server` `deliverEventCaptured` calls `s.eventRing.AppendWithMeta(namespace, eventType, source, subject, direction)` before subscriber dispatch; `show event recent` returns the enriched entries.

### Transformation Path
1. **Profile summary path:** `handleShowSystemProfile` parses `type` (default `heap`) and `duration` (default `10s`, validated `1s..60s`), and checks whether `validated.PipeOperations()` carries the command-owned `"summary"` pipe operation (registered via `command.RegisterPipeFilters([]string{"show system profile"}, command.PipeFilter{Name: "summary", Argument: command.ArgNone, ...})`). `profileCPU` or `profileSnapshot` writes standard gzipped pprof bytes into `bytes.Buffer`. Without `| summary`, it base64-encodes `buf` (`format: "pprof-base64"`, existing shape). With `| summary`, `summarizePprof(profileType, buf.Bytes())` gunzips with a hard decompressed byte cap (`maxProfileDecompressBytes = 32 << 20`), decodes the minimal protobuf `Profile` wire messages (`string_table`, `function`, `location`, `sample`, `sample_type`), aggregates flat and cumulative sample totals per function using the profile's canonical value index (e.g. `inuse_space` for `heap`, `alloc_space` for `allocs`, `samples`/`cpu` for `cpu`, `contentions`/`delay` for `mutex`/`block`, `goroutine` count for `goroutine`), sorts descending by `cum` then `flat`, and returns up to `32` top functions (`format: "summary"`).
2. **Flight recorder path:** `flightrec.Start()` creates and starts `trace.NewFlightRecorder(trace.FlightRecorderConfig{MinAge: 10 * time.Second, MaxBytes: 4 << 20})`. If `flightrec.Start()` ever returns an error at daemon boot, `cmd/ze/hub/main.go` logs a warning and continues starting Ze so flight recording never blocks startup. `CaptureAuto` checks `Enabled()` and `time.Since(lastAutoCapture) >= 30 * time.Second` under a mutex; `CaptureNow` bypasses the cooldown. Both serialize `fr.WriteTo` into a capped buffer (`maxSnapshotBytes = 4 << 20`), assign a monotonic ID (`trace-<seq>`), and store the immutable `Snapshot` in the 4-slot circular ring (evicting oldest). `show system trace` returns recorder status and snapshot metadata (without trace bytes); `show system trace id <id>` returns one snapshot's metadata plus `format: "go-trace-base64"` and base64 `data`.
3. **Severity-split log ring path:** `LogRing.append` assigns `seq++` under `r.mu` and writes `warn`/`error` into `r.faults` (cap 256 by default) and all other levels into `r.general` (cap 512 by default). Level-filtered queries read only the matching sub-ring; unfiltered queries (`Recent`, `Snapshot(limit, "", comp)`) perform an O(N) two-pointer merge newest-first by `seq`.
4. **Enriched event ring path:** `deliverEventCaptured` selects `source` from existing `stampedSender` / `emitter.Name()` / `"engine"`, `subject` from `peerAddress`, and `direction` from `direction`, passing them to `s.eventRing.AppendWithMeta(...)` (zero allocation, pre-dispatch). `handleShowEventRecent` includes `"source"`, `"subject"`, and `"direction"` when non-empty.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| MCP / CLI / SSH / Web / API ↔ Daemon dispatcher | Standard YANG paths `show system profile`, `show system trace`, `request system trace snapshot`, `show event recent` | No |
| `cmd/ze/hub` ↔ `internal/core/flightrec` | Direct startup/shutdown lifecycle call `flightrec.Start()` / `defer flightrec.Stop()` | No |
| `internal/core/taskguard` ↔ `internal/core/flightrec` | Direct core-to-core call `flightrec.CaptureAuto("taskguard-panic", p.origin())` | No |
| `internal/component/plugin/process` ↔ `internal/core/flightrec` | Direct component-to-core call `flightrec.CaptureAuto("slow-plugin-delivery", p.config.Name)` | No |
| `internal/component/cmd/show` ↔ `internal/core/flightrec` | Direct component-to-core reads `Status()`, `Snapshots()`, `SnapshotByID(id)`, `CaptureNow(...)` | No |

### Changed-Wire Census

**Search evidence:** Searched `internal/component/cmd/show/`, `internal/component/plugin/server/`, `internal/component/plugin/all/testdata/wire-methods.snapshot`, `internal/plugins/host-cmd/yang/`, and `test/plugin/` for `show-system-profile`, `show-event-recent`, `container system`, and `EventRecord` (positive controls: `ze-cmd:show-system-profile` in `profile.go`, `ze-cli-show-cmd.yang`, `wire-methods.snapshot`, `test/plugin/system-profile-show.ci`; `EventRecord` in `event_ring.go`, `show.go`, `cmd/ze/hub/service_web.go`). No existing default response shape breaks: bare `show system profile` keeps returning `pprof-base64`; `| summary` is a new command-owned pipe operation (`command.RegisterPipeFilters`); `show event recent` adds optional `source`/`subject`/`direction` keys alongside existing `timestamp`/`namespace`/`event-type`; `show system trace` and `request system trace snapshot` are new additive commands.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `ze-cmd:show-system-profile` RPC | writer | `internal/component/cmd/show/profile.go` (`handleShowSystemProfile`), `profile_summary.go` (`summarizePprof`), `ze-cli-show-cmd.yang` | Existing RPC extended with `type mutex\|block` and command-owned `\| summary` pipe filter (`command.RegisterPipeFilters`) | Preserve default `pprof-base64`; additive `\| summary` pipe and new types |
| `ze-cmd:show-system-profile` RPC | reader | `test/plugin/system-profile-show.ci` (`plugin16SystemProfile`), CLI/MCP/web callers | Asserts `type=heap`, `format=pprof-base64`, `data` | Preserve unchanged; add new `system-profile-summary-show.ci` |
| `ze-cmd:show-system-trace` & `ze-cmd:request-system-trace-snapshot` RPCs | writer | `internal/component/cmd/show/trace.go` (`handleShowSystemTrace`, `handleRequestSystemTraceSnapshot`), `show.go`, `ze-cli-show-cmd.yang` | New additive YANG commands | Additive; update `internal/component/plugin/all/testdata/wire-methods.snapshot` |
| `ze-cmd:show-system-trace` & `ze-cmd:request-system-trace-snapshot` RPCs | reader | `test/plugin/system-trace-show.ci` (`pluginSystemTrace`), CLI/MCP/web callers | Asserts `enabled: true`, `snapshots`, `id`, `format: "go-trace-base64"`, decoded non-empty trace bytes | Additive; new `system-trace-show.ci` and `TestHandleSystemTraceSnapshotAndFetch` prove the wire contract |
| `ze-cmd:show-event-recent` JSON payload | writer | `internal/component/cmd/show/show.go` (`handleShowEventRecent`) | Existing row `{timestamp, namespace, event-type}` | Additive optional keys `source`, `subject`, `direction` when non-empty |
| `ze-cmd:show-event-recent` JSON payload | reader | `internal/component/cmd/show/show_test.go` (`TestShowEventRecentIncludesEnrichedMetadata`), CLI/MCP/web callers | Reads `timestamp`, `namespace`, `event-type`, and optional `source`, `subject`, `direction` | Backwards-compatible; existing consumers ignore new optional keys |
| `EventRecord` Go struct (`SetOnAppend`) | writer | `internal/component/plugin/server/event_ring.go` (`Append`, `AppendWithMeta`), `dispatch.go` (`deliverEventCaptured`), `cmd/ze/hub/service_web.go` (`buildWebService`, `runWebOnly`) | Populates `Timestamp`, `Namespace`, `EventType`, `Source`, `Subject`, `Direction` | Preserve `Append(ns, et)` compatibility; zero-alloc `AppendWithMeta` on dispatch and web startup |
| `EventRecord` Go struct (`SetOnAppend`) | reader | `cmd/ze/hub/service_web.go` (`wireEventRingToBroker`) | Reads `Timestamp`, `Namespace`, `EventType` | Preserve existing fields unchanged; additive fields are backwards-compatible |

### Integration Points
- `internal/core/flightrec/flightrec.go` - new core leaf package wrapping `runtime/trace.FlightRecorder` with bounded snapshot ring and cooldown.
- `cmd/ze/hub/main.go` - start/stop `flightrec` and set conservative mutex/block profiling sampling rates in `run(...)`.
- `internal/core/taskguard/guard.go` - trigger `flightrec.CaptureAuto("taskguard-panic", p.origin())` in `Guard.record`.
- `internal/component/plugin/process/doctor_records.go` - trigger `flightrec.CaptureAuto("slow-plugin-delivery", p.config.Name)` when `DeliverySlow(elapsed)`.
- `internal/core/slogutil/ring.go` - split `LogRing` into `general` and `faults` circular buffers merged by sequence number.
- `internal/component/plugin/server/event_ring.go` & `dispatch.go` - add `Source`, `Subject`, `Direction` to `EventRecord` and `AppendWithMeta`.
- `internal/component/cmd/show/show.go`, `profile.go`, `profile_summary.go`, `trace.go`, `yang/ze-cli-show-cmd.yang` - register and implement profile summaries + trace commands + enriched `show event recent`.
- `internal/component/plugin/all/testdata/wire-methods.snapshot` - add `ze-cmd:request-system-trace-snapshot` and `ze-cmd:show-system-trace`.
- `internal/plugins/skills/data/ze-diagnostics.md` - document `show system profile type ... | summary`, `show system trace`, and `request system trace snapshot`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | All new/extended commands dispatch through YANG + `pluginserver.RegisterRPCs`; daemon lifecycle lives in `cmd/ze/hub/main.go` `run(...)`; MCP receives every command automatically via `tools/list`. |
| No unintended coupling (components stay isolated) | Yes | `internal/core/flightrec` imports stdlib only; `taskguard` imports only core leaf packages; `plugin/process` and `cmd/show` import core downward only; zero edits to `internal/component/mcp`. |
| No duplicated functionality (extends existing, does not recreate) | Yes | Extends existing `handleShowSystemProfile`, `slogutil.LogRing`, `pluginserver.EventRing`, `taskguard.Guard.record`, and `process.Process.noteBatch`. |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `EventRing.AppendWithMeta` reuses existing in-scope strings with zero allocation; `flightrec` is idle/off in offline CLI and copies trace bytes only on explicit or cooldown-throttled capture. |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | `ze-cmd:show-system-trace` and `ze-cmd:request-system-trace-snapshot` register in `pluginserver.RegisterRPCs` and `ze-cli-show-cmd.yang`. |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `internal/component/cmd/show`, `internal/component/mcp`, `internal/component/command`, and `internal/component/plugin/all`; only the derived golden `wire-methods.snapshot` records the two new wire methods, while help, completion, `ze_reference`, and MCP tools derive from YANG + `Dispatcher.Commands()`. |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Go `1.27.4` stdlib `runtime/trace.FlightRecorder` (`NewFlightRecorder`, `Start`, `Stop`, `Enabled`, `WriteTo`) is available in this toolchain and safe to run concurrently with `pprof.StartCPUProfile`. | `go.mod` specifies `go 1.27.4` and `go doc runtime/trace.FlightRecorder` confirmed all five methods. | Would need to gate or fallback if Go version lacked `FlightRecorder`. | `./le test unit pkg path ./internal/core/flightrec/` under `-race` | confirmed |
| A-2 | Pure-stdlib gzipped pprof protobuf parsing (`sample`, `location`, `function`, `string_table`, `sample_type`) decodes profiles produced by Go's `pprof.Profile.WriteTo(&buf, 0)` and `pprof.StartCPUProfile(&buf)` across all 6 profile types (`cpu`, `heap`, `goroutine`, `allocs`, `mutex`, `block`). | Go stdlib `runtime/pprof` emits standard `perftools.profiles.Profile` wire protobuf compressed with gzip. | Some profile type could use a protobuf wire feature (e.g. packed varints in `location_id` or `value`) that a naive decoder misses. | Implement packed + unpacked repeated varint handling and unit-test `summarizePprof` against real `pprof.Lookup` output for all 6 types plus `StartCPUProfile` | confirmed |
| A-3 | `runtime.SetMutexProfileFraction(10)` and `runtime.SetBlockProfileRate(100_000)` in daemon startup (`cmd/ze/hub/main.go`) do not regress BGP update throughput or trip any existing test. | 100 µs block threshold ignores all uncontended fast-path ops; 1-in-10 contended mutex sampling is Go's standard low-overhead production setting. | Could lower to higher threshold (e.g. 1 ms) if measurable overhead appeared. | Unit tests + functional `.ci` tests (`system-profile-summary-show.ci`) | confirmed |
| A-4 | Declaring `container request > container system` in `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` with byte-identical `description` and `ze:help` to `internal/plugins/host-cmd/yang/ze-host-request-cmd.yang` merges cleanly with zero YANG mismatch warnings and passes `./le doc yang-contract command-contract` and `./le cli grammar`. | `ze-host-request-cmd.yang:11-15` defines `container request > container system`; `ze-log-cmd.yang` already declares both `container show` and `container request` in one module. | Would move `request system trace snapshot` to a dedicated YANG file in `internal/component/cmd/show/yang/` if module conventions preferred it. | `./le doc yang-contract command-contract`, `./le cli grammar`, `./le doc check all` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Only one `runtime/trace.FlightRecorder` may be active per Go process (`go doc runtime/trace.FlightRecorder`); concurrent tests calling `flightrec.Start()` in parallel could collide. | `TestFlightRec*` fails with "flight recorder already enabled". | Serialize `Start`/`Stop` behind `Recorder.mu` in `internal/core/flightrec`, make `Start()` idempotent when already enabled on the global recorder, and document test cleanup (`t.Cleanup(flightrec.Stop)`). |
| R-2 | A malformed or huge gzipped pprof payload could cause unbounded memory growth or panics inside `summarizePprof`. | OOM or slice bounds panic on crafted input. | Bound `gzip.Reader` with `io.LimitReader(gz, maxProfileDecompressBytes+1)` (32 MiB), check every protobuf field length and string-table index before indexing, and write unit tests for truncated/corrupt/oversized inputs (never panic on bad bytes). |
| R-3 | Frequent automatic triggers (e.g., a tight panic loop or flapping slow plugin) could repeatedly call `FlightRecorder.WriteTo`, spiking CPU/memory. | Repeated snapshots every millisecond under a fault storm. | Enforce `captureCooldown = 30 * time.Second` on `CaptureAuto`, cap each snapshot at `maxSnapshotBytes = 4 << 20`, and keep at most `maxSnapshots = 4` snapshots in the circular ring. |
| R-4 | Splitting `slogutil.LogRing` into `general` and `faults` changes capacity math for tests that construct small rings (`newLogRing(3)`, `newLogRing(32)`). | Existing `TestLogRingOverflow` or `TestRingHandlerConcurrentAttributes` fails. | When `capacity < defaultLogRingCapacity`, give both sub-rings `capacity` slots (matching existing test expectations); when `capacity >= defaultLogRingCapacity`, give `general` `capacity` (512) and `faults` `defaultFaultRingCapacity` (256), and preserve newest-first merge by monotonic `seq`. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Profile summary could misrank functions, flight-recorder capture could return empty/error on demand, or `LogRing`/`EventRing` ordering could shift. No routing, forwarding, config syntax, or wire-protocol peer path is altered. |
| How is it reverted? | Single commit revert; `flightrec` is a new leaf package, `show system trace` / `request system trace snapshot` are additive commands, and `show system profile` preserves its default `raw` mode. |
| Who else touches this path? | `internal/core/flightrec`, `internal/core/slogutil`, `internal/core/taskguard`, `internal/component/plugin/server`, `internal/component/plugin/process`, `internal/component/cmd/show`, `cmd/ze/hub/main.go`, `internal/plugins/skills`, and `internal/component/plugin/all/testdata/wire-methods.snapshot`. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show system profile type heap\|mutex\|block \| summary` | → | `ze-cmd:show-system-profile` + `RegisterPipeFilters("show system profile", "summary")` -> `handleShowSystemProfile` -> `summarizePprof` -> structured JSON `top-functions` | `test/plugin/system-profile-summary-show.ci`, `TestProfileSummaryAllTypes`, `TestProfileTypesMatchTheModel` |
| `request system trace snapshot` & `show system trace [id <id>]` | → | `ze-cmd:request-system-trace-snapshot` / `ze-cmd:show-system-trace` -> `internal/core/flightrec` (`Start` in `cmd/ze/hub/main.go`) -> captured Go trace snapshot | `test/plugin/system-trace-show.ci`, `TestHandleSystemTraceSnapshotAndFetch` |
| Automatic `flightrec` trigger on `taskguard` panic & slow plugin delivery | → | `Guard.record` & `Process.noteBatch` -> `flightrec.CaptureAuto` -> snapshot visible in `flightrec.Snapshots()` | `TestGuardTriggersFlightRecorderCapture`, `TestSlowDeliveryTriggersFlightRecorderCapture` |
| `slogutil.LogRing` severity split under `info` burst | → | `ringHandler.Handle` -> `LogRing.append` (`faults` vs `general`) -> `Snapshot(0, "warn"\|"err", "")` | `TestLogRingFaultsSurviveInfoBurst` |
| `show event recent` enriched metadata | → | `deliverEventCaptured` -> `EventRing.AppendWithMeta` -> `handleShowEventRecent` (`source`, `subject`, `direction`) | `TestShowEventRecentIncludesEnrichedMetadata` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `show system profile` invoked with `type mutex` or `type block` (bare or with `\| summary`) or with `\| summary` (or MCP `pipe: [{"name": "summary"}]`) on any of the 6 types (`cpu`, `heap`, `goroutine`, `allocs`, `mutex`, `block`) | Returns `StatusDone` with `format: "summary"`, `type`, `sample-type`, `sample-unit`, `total`, `top-functions` (at most `32` entries sorted descending by `cum` then `flat`, each with `function`, `file`, `line`, `flat`, `flat-pct`, `cum`, `cum-pct`), and `truncated` flag; `mutex`/`block` include sampling-rate metadata; bare `show system profile heap` (without `\| summary`) still returns `format: "pprof-base64"` and `data` unchanged; `TestProfileTypesMatchTheModel` confirms YANG `type` enum matches Go handler constants in both directions and `summary` is registered via `command.RegisterPipeFilters`. | `top-functions` may be empty (`[]`) on `mutex`/`block` when zero contention occurred since startup; `cpu` summary still respects `duration` (`1s..60s`) and `cpuProfileMu.TryLock()`. |
| AC-2 | `summarizePprof` receives truncated, corrupt, non-gzip, or oversized (>32 MiB decompressed) input | Returns a non-nil Go error (`errProfileEmpty`, `errProfileTooLarge`, `errProfileCorruptWire`, `errProfileInvalidStringIndex`, or gzip read error) and a nil summary map (and `handleShowSystemProfile` wraps it in `plugin.OperatorErrorResponse`); rejects malformed bytes without panicking or allocating past the 32 MiB decompressed cap. | None — malformed bytes must never panic the daemon. |
| AC-3 | Daemon boots through `cmd/ze/hub/main.go` `run(...)`, then operator/AI calls `request system trace snapshot`, `show system trace`, and `show system trace id <id>` | `flightrec` starts non-fatally at boot (logging a warning and continuing daemon startup if `flightrec.Start()` ever errors); when enabled (`enabled: true`), `request system trace snapshot` captures a non-empty Go trace snapshot and returns its metadata (`id`, `captured-at`, `trigger: "operator-request"`, `bytes`), `show system trace` lists snapshots without raw trace bytes, and `show system trace id <id>` returns `format: "go-trace-base64"` and valid non-empty base64 Go trace bytes; calling `request system trace snapshot` when recorder is stopped or requesting an unknown `id` returns an explicit `plugin.OperatorErrorResponse` (`opererr`) refusal with a concrete fix command. | At most `4` snapshots (each capped at `4 MiB`) are retained in memory; older snapshots are evicted FIFO when the ring fills. |
| AC-4 | A guarded task panics (`taskguard.Task.Contain`) or a plugin batch exceeds `DeliverySlowThreshold` (`Process.noteBatch`) while `flightrec` is running | `flightrec.CaptureAuto` captures a trace snapshot with trigger `"taskguard-panic"` or `"slow-plugin-delivery"` and non-empty detail; a second automatic trigger inside the `30s` cooldown window is throttled cleanly without error; when `flightrec` is stopped, both paths remain no-ops with zero errors. | Automatic triggers inside the `30s` cooldown are skipped to prevent capture storms; manual `CaptureNow` always captures immediately. |
| AC-5 | `slogutil.LogRing` receives `warn` and `error` entries followed by more than `defaultLogRingCapacity` (512) `info`/`debug` entries | `Snapshot(0, "warn", "")`, `Snapshot(0, "err", "")`, and `Recent(0)` still return all retained `warn`/`error` entries (up to `defaultFaultRingCapacity = 256`) in newest-first sequence order, while `info`/`debug` entries rotate inside their own 512-slot buffer; `ringHandler.Enabled` continues to delegate to `inner.Enabled`. | Older `warn`/`error` entries beyond `256` rotate within the `faults` ring; older `info`/`debug` entries beyond `512` rotate within the `general` ring. |
| AC-6 | An event is delivered through `plugin/server` `deliverEventCaptured` (from engine or plugin, with or without peer address and direction) and queried via `show event recent` | `EventRecord` captures `Source` (`stampedSender`, emitter name, or `"engine"`), `Subject` (`peerAddress`), and `Direction` before subscriber dispatch with zero extra allocation on the emit path, and `show event recent` includes `"source"`, `"subject"`, and `"direction"` when non-empty alongside `"timestamp"`, `"namespace"`, and `"event-type"`; existing `Append(ns, et)` and `SetOnAppend` callers continue working unchanged. | Empty `subject` or `direction` (e.g., non-peer-scoped events) are omitted from the JSON row rather than emitted as empty strings. |
| AC-7 | Operator or AI reads `show skill ze-diagnostics` | Includes `show system profile ... \| summary`, `mutex`/`block` contention profiling, `show system trace`, and `request system trace snapshot` in the live runtime triage playbook while preserving all existing Part 1 and config-validation content. | Unrelated skills stay unchanged. |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Investigates a CPU spike or lock contention over MCP without `go tool pprof` | `show system profile type heap\|mutex\|block \| summary` -> `ze-cmd:show-system-profile` -> `summarizePprof` -> top 32 functions in JSON | `test/plugin/system-profile-summary-show.ci`, `TestProfileSummaryAllTypes` |
| 2 | Captures and inspects a pre-incident Go execution trace from a live daemon | `cmd/ze/hub` `run` starts `flightrec` -> `request system trace snapshot` -> `show system trace` -> `show system trace id <id>` | `test/plugin/system-trace-show.ci`, `TestHandleSystemTraceSnapshotAndFetch` |
| 3 | Diagnoses a bug after a burst of `info` logs and plugin events | `slogutil.LogRing` keeps `warn`/`error` in `faults` sub-ring + `deliverEventCaptured` records `source`/`subject`/`direction` -> `show log recent level err` & `show event recent` | `TestLogRingFaultsSurviveInfoBurst`, `TestShowEventRecentIncludesEnrichedMetadata` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFlightRecStartCaptureRingAndCooldown` | `internal/core/flightrec/flightrec_test.go` | AC-3, AC-4, R-1, R-3: start/stop lifecycle, `CaptureNow` ring eviction past 4 snapshots, `SnapshotByID`, `CaptureAuto` cooldown throttling, and disabled refusal | |
| `TestFlightRecSnapshotByteCap` | `internal/core/flightrec/flightrec_test.go` | AC-3 / R-3: snapshot writer respects `maxSnapshotBytes` boundary cleanly | |
| `TestGuardTriggersFlightRecorderCapture` | `internal/core/taskguard/guard_test.go` | AC-4: recovered panic triggers `flightrec.CaptureAuto("taskguard-panic", ...)` when enabled and stays silent when disabled | |
| `TestSlowDeliveryTriggersFlightRecorderCapture` | `internal/component/plugin/process/doctor_records_test.go` | AC-4: `noteBatch` over `DeliverySlowThreshold` triggers `flightrec.CaptureAuto("slow-plugin-delivery", ...)` when enabled | |
| `TestLogRingFaultsSurviveInfoBurst` | `internal/core/slogutil/ring_test.go` | AC-5 / R-4: 600+ `info` entries do not evict earlier `warn`/`error` entries; `Recent` merges both rings newest-first by sequence; fault ring rotates cleanly at 256 | |
| `TestEventRingAppendWithMeta` | `internal/component/plugin/server/event_ring_test.go` | AC-6: `AppendWithMeta` stores `Source`, `Subject`, `Direction`, preserves `Append` compatibility, and allocates 0 bytes on append (`testing.AllocsPerRun`) | |
| `TestShowEventRecentIncludesEnrichedMetadata` | `internal/component/cmd/show/show_test.go` | AC-6: `handleShowEventRecent` emits `source`, `subject`, `direction` when non-empty and omits empty optional fields | |
| `TestProfileSummaryAllTypes` | `internal/component/cmd/show/profile_summary_test.go` | AC-1: `summarizePprof` decodes real Go profiles for `heap`, `goroutine`, `allocs`, `mutex`, `block`, and `cpu` (`1s`) into sorted bounded `top-functions` JSON | |
| `TestProfileSummaryMalformedInput` | `internal/component/cmd/show/profile_summary_test.go` | AC-2 / R-2: truncated, corrupt, non-gzip, out-of-range string indices, and oversized (>32 MiB) gzip payloads return errors without panicking | |
| `TestProfileTypesMatchTheModel` | `internal/component/cmd/show/profile_test.go` | AC-1: YANG `show/system/profile/type` enum matches Go handler constants in both directions, and `"summary"` is registered as a pipe filter on `"show system profile"` | |
| `TestHandleSystemTraceSnapshotAndFetch` | `internal/component/cmd/show/trace_test.go` | AC-3: `request system trace snapshot`, `show system trace`, `show system trace id <id>`, disabled refusal (`opererr`), and unknown-ID refusal (`opererr`) | |
| `TestSkillsDiagnosticsIncludesPart2Commands` | `internal/plugins/skills/skills_test.go` | AC-7: `show skill ze-diagnostics` includes profile summary, `mutex`/`block`, and trace snapshot commands | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `show system profile` `duration` | `1s..60s` | `1s`, `60s` | `500ms` (refused) | `61s` (refused) |
| `summarizePprof` decompressed input size | `<= 32 MiB` | `32 MiB` | `0` bytes (error) | `32 MiB + 1` (error, no OOM) |
| `summarizePprof` `top-functions` cap | `0..32` | `32` (`truncated: false`) | `0` (`[]`, `truncated: false`) | `33+` (`32` returned, `truncated: true`) |
| `flightrec` snapshot ring capacity | `0..4` | `4` kept | `0` (`[]`) | `5th` evicts oldest (`4` kept) |
| `slogutil.LogRing` fault ring cap | `0..256` | `256` faults | `0` | `257` faults (oldest fault evicted; 600+ infos evict 0 faults) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `system-profile-summary-show` | `test/plugin/system-profile-summary-show.ci` | Live daemon answers `show system profile type heap \| summary`, `type mutex \| summary`, and `type block \| summary` with structured `top-functions` JSON | |
| `system-trace-show` | `test/plugin/system-trace-show.ci` | Live daemon starts `flightrec` at boot, captures a snapshot via `request system trace snapshot`, lists it via `show system trace`, and fetches base64 trace bytes via `show system trace id <id>` | |

### Interop Tests (Scope: protocol)
N-A — this spec adds no peer-facing wire protocol behavior.

## Files to Modify
- `cmd/ze/hub/main.go` - start/stop `flightrec` and set conservative mutex/block profiling sampling rates in `run(...)`
- `internal/core/taskguard/guard.go` - call `flightrec.CaptureAuto("taskguard-panic", p.origin())` in `Guard.record`
- `internal/core/taskguard/guard_test.go` - add `TestGuardTriggersFlightRecorderCapture`
- `internal/core/slogutil/ring.go` - split `LogRing` into `general` and `faults` circular buffers merged by sequence number
- `internal/core/slogutil/ring_test.go` - add `TestLogRingFaultsSurviveInfoBurst`
- `internal/component/plugin/process/doctor_records.go` - call `flightrec.CaptureAuto("slow-plugin-delivery", p.config.Name)` when `DeliverySlow(elapsed)`
- `internal/component/plugin/process/doctor_records_test.go` - add `TestSlowDeliveryTriggersFlightRecorderCapture`
- `internal/component/plugin/server/event_ring.go` - add `Source`, `Subject`, `Direction` to `EventRecord` and `AppendWithMeta`
- `internal/component/plugin/server/event_ring_test.go` - add `TestEventRingAppendWithMeta`
- `internal/component/plugin/server/dispatch.go` - call `s.eventRing.AppendWithMeta` with `source`, `peerAddress`, `direction`
- `internal/component/cmd/show/show.go` - register `ze-cmd:show-system-trace` and `ze-cmd:request-system-trace-snapshot`; emit `source`, `subject`, `direction` in `handleShowEventRecent`
- `internal/component/cmd/show/show_test.go` - add `TestShowEventRecentIncludesEnrichedMetadata`
- `internal/component/cmd/show/profile.go` - add `mutex`/`block` types, register command-owned `| summary` pipe filter via `command.RegisterPipeFilters`, and add `sampling-rate` metadata
- `internal/component/cmd/show/profile_test.go` - add `TestProfileTypesMatchTheModel` and `| summary` pipe / type tests
- `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` - extend `show > system > profile` with `mutex`/`block`; add `show > system > trace` and `request > system > trace > snapshot`
- `internal/component/plugin/all/testdata/wire-methods.snapshot` - add `ze-cmd:request-system-trace-snapshot` and `ze-cmd:show-system-trace`
- `internal/plugins/skills/data/ze-diagnostics.md` - document profile summaries (`| summary`), `mutex`/`block` profiles, and trace snapshots
- `internal/plugins/skills/skills_test.go` - add `TestSkillsDiagnosticsIncludesPart2Commands`
- `docs/architecture/diagnostics/procfs-diagnostics.md` - document `mutex`/`block`, `| summary` pipe operation, and `flightrec`
- `docs/architecture/diagnostics/production-diagnostics.md` - document `show system trace`, `request system trace snapshot`, and severity-split log ring
- `docs/architecture/diagnostics/event-history-rings.md` - document `Source`, `Subject`, `Direction` on `EventRecord`
- `docs/architecture/diagnostics/crash-capture.md` - document `taskguard` auto-capturing a flight-recorder snapshot
- `docs/architecture/api/commands.md` - add `show system trace` and `request system trace snapshot` to System commands
- `docs/guide/command-reference.md` - document updated `show system profile`, `show system trace`, `request system trace snapshot`, and enriched `show event recent`
- `ai/INDEX.md` - add discovery row for flight recorder, `show system trace`, and profile summary

## Files to Create
- `internal/core/flightrec/flightrec.go` - core leaf wrapper over Go `runtime/trace.FlightRecorder` with bounded in-memory snapshot ring and cooldown
- `internal/core/flightrec/flightrec_test.go` - unit tests for lifecycle, ring eviction, byte cap, and cooldown
- `internal/component/cmd/show/profile_summary.go` - pure-stdlib gzipped pprof protobuf summary parser
- `internal/component/cmd/show/profile_summary_test.go` - unit and malformed-input tests for `summarizePprof`
- `internal/component/cmd/show/trace.go` - `handleShowSystemTrace` and `handleRequestSystemTraceSnapshot`
- `internal/component/cmd/show/trace_test.go` - unit tests for trace commands and `opererr` refusals
- `internal/test/fixture/register_system_profile_summary_show.go` - fixture registration for `plugin/system-profile-summary-show`
- `internal/test/fixture/plugin_system_profile_summary_show.go` - functional fixture asserting `show system profile ... | summary`
- `internal/test/fixture/register_system_trace_show.go` - fixture registration for `plugin/system-trace-show`
- `internal/test/fixture/plugin_system_trace_show.go` - functional fixture asserting `request system trace snapshot` and `show system trace`
- `test/plugin/system-profile-summary-show.ci` - functional test for profile summary and `mutex`/`block`
- `test/plugin/system-trace-show.ci` - functional test for live flight recorder capture and retrieval

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` (`show > system > profile` leaves, `show > system > trace`, `request > system > trace > snapshot`) |
| YANG validation constraints | Yes | `leaf type` enum (6 values), `leaf duration` pattern, `leaf id` `string { length "1..64"; }`; `\| summary` declared via `command.RegisterPipeFilters` (`Argument: command.ArgNone`) |
| YANG custom validators | N-A | Native YANG enums, pattern, and length suffice |
| CLI commands/flags | Yes | Online daemon commands (`show system profile`, `show system trace`, `request system trace snapshot`); no offline flags |
| CLI grammar (keyword before value) | Yes | Verified against R1-R9 (`show system profile type <t> \| summary`, `show system trace id <id>`, `request system trace snapshot`); verified by `./le cli grammar` |
| Editor autocomplete | Yes | Automatic from YANG command nodes and enums |
| Functional test for new RPC/API | Yes | `test/plugin/system-profile-summary-show.ci`, `test/plugin/system-trace-show.ci` |
| Pipe completeness | Yes | All handlers return structured `plugin.Map` routed through `ApplyPipes` (`\| json`, `\| yaml`, `\| table`) |
| Env var registration | N-A | No new env vars; bounded defaults in `internal/core/flightrec` and `internal/core/slogutil` |
| Doctor check for runtime dependencies | N-A | In-memory flight recorder and stdlib pprof add zero host/filesystem/socket/external-binary dependencies |
| Doctor code per problem class | N-A | Reuses existing `doctor-internal-defect` and `doctor-plugin-delivery-failed`; trace refusals are operator `opererr` responses |
| Prometheus counters/metrics | N-A | Existing `ze_task_panics_total` and `ze_plugin_delivery_slow_batches_total` already count both trigger classes |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not a BGP family change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Additive diagnostic commands and internal ring improvements; no feature maturity row in `features/` changes |
| 2 | Config syntax changed? | No | No config YANG module changed |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | No | No plugin added or removed; only embedded skill Markdown updated |
| 6 | Has a user guide page? | No | Covered by `docs/guide/command-reference.md` |
| 7 | Wire format changed? | No | No protocol wire format changed |
| 8 | Plugin SDK/protocol changed? | No | Deferred to Part 3 (`plan/post-release/features/spec-ai-diagnostics-3-external-plugin-introspection.md`) |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Not protocol/RFC work |
| 10 | Test infrastructure changed? | No | Reuses existing `test/plugin/` runner and fixture registration |
| 11 | Affects daemon comparison? | No | Internal diagnostic surface |
| 12 | Internal architecture changed? | Yes | `docs/architecture/diagnostics/procfs-diagnostics.md`, `docs/architecture/diagnostics/production-diagnostics.md`, `docs/architecture/diagnostics/event-history-rings.md`, `docs/architecture/diagnostics/crash-capture.md` |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | No | No new Prometheus counter |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `internal/component/plugin/all/testdata/wire-methods.snapshot`, `docs/guide/command-reference.md`, `ai/INDEX.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED by `./le spec citation anchors spec plan/post-release/features/spec-ai-diagnostics-2-flight-recorder-and-profiles.md`. **Updated:** `docs/architecture/diagnostics/procfs-diagnostics.md`, `docs/architecture/diagnostics/production-diagnostics.md`, `docs/architecture/diagnostics/event-history-rings.md`, `docs/architecture/diagnostics/crash-capture.md`, `docs/architecture/api/commands.md`, `docs/guide/command-reference.md`. **Declared `// Design:` pages, unaffected:** `docs/architecture/api/process-protocol.md` (`dispatch.go` wire protocol and lossless delivery unchanged), `docs/architecture/doctor-and-health-checks.md` (`doctor_records.go` slow threshold and doctor codes unchanged), `docs/architecture/hub-architecture.md` (`cmd/ze/hub/main.go` boot/reload orchestration unchanged), `docs/features/ai-first.md` (bundled skill serving mechanism unchanged). |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify `docs/guide/command-reference.md` and `internal/plugins/skills/data/ze-diagnostics.md` examples match YANG |

## Design presentation

### What changes

An AI assistant or operator investigating a CPU spike, lock contention, or unexpected runtime bug gains four connected capabilities with zero external dependencies:

1. **AI-readable profiles plus contention sampling:** `show system profile` adds `type mutex` and `type block` (with low-overhead daemon startup sampling in `cmd/ze/hub/main.go`) and a command-owned `| summary` pipe operation (`command.RegisterPipeFilters` on `"show system profile"`, composable with `| json`/`| table` and MCP `pipe: [{"name": "summary"}]`), which decodes any of the six profile types (`cpu`, `heap`, `goroutine`, `allocs`, `mutex`, `block`) through a pure-stdlib pprof protobuf parser into the top 32 functions (`function`, `file`, `line`, `flat`, `flat-pct`, `cum`, `cum-pct`) in JSON. Bare `show system profile` (`pprof-base64`) stays unchanged for `go tool pprof` and existing callers.
2. **Always-on pre-incident Go flight recorder:** New core leaf package `internal/core/flightrec` runs Go's `runtime/trace.FlightRecorder` (`10s` / `4 MiB` rolling window) in the daemon, keeps up to 4 snapshots in memory, auto-captures (with a 30s cooldown) on `taskguard` panic and slow plugin delivery, and exposes `request system trace snapshot` and `show system trace [id <id>]` over CLI, SSH, web, REST, gRPC, and MCP.
3. **Severity-split log ring:** `slogutil.LogRing` separates `warn`/`error` (256 slots) from `debug`/`info` (512 slots) with a shared sequence counter, so `info`/`debug` log bursts never evict earlier warnings or errors.
4. **Enriched event history:** `EventRecord` carries `Source`, `Subject`, and `Direction` (captured zero-allocation before subscriber dispatch), surfaced in `show event recent`.

### Before and after

1. **Diagnosing high CPU or a mutex contention wedge over MCP:**
   - Before: `show system profile type heap` returns a large opaque `pprof-base64` blob an AI cannot read, and `mutex`/`block` profiles are unavailable.
   - After: `show system profile type mutex | summary` (or `heap`, `cpu`, `block`, `goroutine`, `allocs`, and over MCP `pipe: [{"name": "summary"}]`) returns a compact JSON table of the top 32 functions with file, line, flat/cumulative totals, and percentages.

2. **Capturing what happened immediately before an unexpected panic or slow plugin stall:**
   - Before: By the time an operator notices the fault, the preceding scheduler/goroutine/GC execution timeline is gone unless the bug reproduces under manual tracing.
   - After: `taskguard.Guard.record` and slow plugin delivery automatically freeze the preceding seconds from `runtime/trace.FlightRecorder` into the 4-slot snapshot ring; operators and AIs can also capture on demand with `request system trace snapshot`, list captures with `show system trace`, and fetch the Go trace payload with `show system trace id <id>`.

3. **Reading `show log recent level err` and `show event recent` after noisy traffic:**
   - Before: 600 `info` logs evict every earlier `warn`/`error` from the shared 512-slot ring, and `show event recent` shows only `timestamp`, `namespace`, and `event-type` with no emitter or peer.
   - After: `warn`/`error` entries stay preserved in their dedicated 256-slot fault ring regardless of `info`/`debug` volume, and `show event recent` includes `source`, `subject`, and `direction`.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Vendor `github.com/google/pprof/profile` to parse profiles | Adds a large third-party module (`ai/META-DECISIONS-SITUATIONAL.md`, Third-party dependencies) when parsing the 5 protobuf fields (`string_table`, `function`, `location`, `sample`, `sample_type`) takes ~150 lines of pure Go stdlib (`compress/gzip` + `encoding/binary.Uvarint`). |
| Parse `pprof.Profile.WriteTo(w, 1)` text output instead of protobuf | `pprof.StartCPUProfile` only writes binary gzipped protobuf (never `debug=1` text), so a single pure-Go protobuf reader handles all 6 profile types uniformly including `cpu`. |
| Write flight-recorder trace snapshots to disk under `ze.crash.dir` | Adds a host filesystem dependency, disk-space rotation concerns, and fails on read-only containers/test hosts; a bounded 4-slot in-memory ring (`<= 16 MiB` worst case, `0 B` idle) works everywhere with zero runtime dependencies. |
| Add a `leaf mode raw\|summary` argument to `show system profile` instead of a pipe operation | Violates `docs/architecture/api/commands.md` ("The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it"); registering `\| summary` via `command.RegisterPipeFilters` runs inside `handleShowSystemProfile` before base64 encoding, keeps bare `pprof-base64` intact, and composes cleanly across CLI and MCP. |

### Documents departed from

none

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Daemon startup mutex/block profiling defaults in `cmd/ze/hub/main.go` | (a) Enable low-overhead defaults at daemon start (`SetMutexProfileFraction(10)`, `SetBlockProfileRate(100_000)` = 100 µs) so `show system profile type mutex\|block` works immediately on any production wedge without restart (Recommended); (b) Leave mutex/block sampling off (`0`) by default and require a future runtime toggle | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register new YANG nodes, stubs, wire snapshot entries, and failing TDD tests
   - Tests: `TestFlightRecStartCaptureRingAndCooldown`, `TestLogRingFaultsSurviveInfoBurst`, `TestEventRingAppendWithMeta`, `TestProfileSummaryAllTypes`, `TestProfileTypesMatchTheModel`, `TestHandleSystemTraceSnapshotAndFetch`, `TestSkillsDiagnosticsIncludesPart2Commands`, plus draft `.ci` tests in `test/draft/plugin/`
   - Files: `internal/core/flightrec/flightrec.go` (stub), `internal/component/cmd/show/yang/ze-cli-show-cmd.yang`, `internal/component/cmd/show/show.go`, `internal/component/cmd/show/trace.go` (stub), `internal/component/cmd/show/profile_summary.go` (stub), `internal/component/plugin/all/testdata/wire-methods.snapshot`, test & fixture files
   - Verify: YANG/CLI/snapshot contracts pass and all new unit tests fail RED on behavior.
2. **Phase: Core flight recorder, severity-split log ring, and enriched event ring**
   - Tests: `TestFlightRecStartCaptureRingAndCooldown`, `TestFlightRecSnapshotByteCap`, `TestGuardTriggersFlightRecorderCapture`, `TestSlowDeliveryTriggersFlightRecorderCapture`, `TestLogRingFaultsSurviveInfoBurst`, `TestEventRingAppendWithMeta`, `TestShowEventRecentIncludesEnrichedMetadata`
   - Files: `internal/core/flightrec/flightrec.go`, `flightrec_test.go`, `cmd/ze/hub/main.go`, `internal/core/taskguard/guard.go`, `guard_test.go`, `internal/core/slogutil/ring.go`, `ring_test.go`, `internal/component/plugin/process/doctor_records.go`, `doctor_records_test.go`, `internal/component/plugin/server/event_ring.go`, `event_ring_test.go`, `dispatch.go`, `internal/component/cmd/show/show.go`, `show_test.go`, `docs/architecture/diagnostics/event-history-rings.md`, `docs/architecture/diagnostics/crash-capture.md`
   - Verify: unit tests pass under `-race`; `./le arch tier check` passes.
3. **Phase: Profile summary parser, `mutex`/`block`, and trace commands**
   - Tests: `TestProfileSummaryAllTypes`, `TestProfileSummaryMalformedInput`, `TestProfileTypesMatchTheModel`, `TestHandleSystemTraceSnapshotAndFetch`
   - Files: `internal/component/cmd/show/profile.go`, `profile_test.go`, `profile_summary.go`, `profile_summary_test.go`, `trace.go`, `trace_test.go`, `docs/architecture/diagnostics/procfs-diagnostics.md`, `docs/architecture/diagnostics/production-diagnostics.md`, `docs/architecture/api/commands.md`, `docs/guide/command-reference.md`, `ai/INDEX.md`
   - Verify: unit tests pass; `./le cli grammar`, `./le cli ownership`, `./le doc yang-contract command-contract`, `./le doc check all` pass.
4. **Phase: Agent skills and end-to-end functional proof**
   - Tests: `TestSkillsDiagnosticsIncludesPart2Commands`, `test/plugin/system-profile-summary-show.ci`, `test/plugin/system-trace-show.ci`
   - Files: `internal/plugins/skills/data/ze-diagnostics.md`, `skills_test.go`, fixture files, `.ci` files (drafted, stress-tested under load, then promoted to `test/plugin/`)
   - Verify: scoped unit tests, load stress tests, and promoted `.ci` tests pass.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-7 has an implementation at file:line and a passing test |
| Feature completeness | All three end-to-end user stories work through their real entry points |
| Correctness | `summarizePprof` handles packed and unpacked repeated varints and rejects corrupt/oversized input with errors (never panics); `flightrec` cooldown and 4-slot eviction work under `-race`; `LogRing` merges `general` + `faults` strictly newest-first |
| Naming | Every JSON key in profile summaries, trace commands, and enriched events is lowercase kebab-case |
| Data flow | `internal/core/flightrec` imports stdlib only; `EventRing.AppendWithMeta` allocates 0 bytes on emit and runs before subscriber dispatch; `cmd/ze/hub/main.go` alone starts `flightrec` |
| Operator errors and doctor codes | `request system trace snapshot` (when disabled) and `show system trace id <unknown>` return `plugin.OperatorErrorResponse(opererr.New(...))` with a concrete fix command |
| Rule: cli | `show system profile` preserves bare `pprof-base64` default, registers `\| summary` via `command.RegisterPipeFilters`, matches YANG `type` enum in both directions, and passes `./le cli grammar` and merged-node parity |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `internal/core/flightrec` lifecycle, cooldown, ring eviction | `./le test unit pkg path ./internal/core/flightrec/` |
| `taskguard` and `plugin/process` auto-capture triggers | `./le test unit pkg path ./internal/core/taskguard/` and `./le test unit pkg path ./internal/component/plugin/process/` |
| Severity-split `slogutil.LogRing` | `./le test unit pkg path ./internal/core/slogutil/` |
| Enriched `EventRecord` (zero-alloc) | `./le test unit pkg path ./internal/component/plugin/server/` |
| Profile summaries (`mutex`/`block` + `\| summary`) & trace commands | `./le test unit pkg path ./internal/component/cmd/show/` |
| Bundled skill update | `./le test unit pkg path ./internal/plugins/skills/` |
| End-to-end daemon functional tests | `./le --name diag2 test bgp plugin --pattern system-profile-summary-show` and `--pattern system-trace-show` |
| Architecture, CLI grammar, YANG contract, and docs gates | `./le arch tier check`, `./le cli grammar`, `./le doc yang-contract command-contract`, `./le doc check all` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Denial of service / allocation safety | `summarizePprof` enforces `maxProfileDecompressBytes = 32 MiB` via `io.LimitReader`, bounds all protobuf slices and string-table lookups, and caps `top-functions` to `32`; `flightrec` caps each snapshot at `4 MiB` and keeps at most `4` snapshots with a `30s` automatic-capture cooldown |
| Secret exposure | Profiles, execution traces, and `EventRecord` (`source`, `subject`, `direction`) expose no config secrets, tokens, or credentials; `show system trace` returns metadata only unless a caller explicitly fetches `id <id>` |
| Authorization | `show system profile`, `show system trace`, and `show event recent` stay `RoleRead` under `show`; `request system trace snapshot` runs under `request` (`RoleAction`) through the shared dispatcher |

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

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Pure-stdlib gzipped pprof protobuf summary parser (`profile_summary.go`) | Vendoring `github.com/google/pprof`; parsing `debug=1` text | Zero external dependencies, works uniformly across all 6 profile types including `cpu` (which never emits `debug=1` text), and stays bounded (`32 MiB` decompress cap, `32` top functions). |
| Core leaf `internal/core/flightrec` with bounded 4-slot in-memory snapshot ring | Writing trace files to disk under `ze.crash.dir`; placing flightrec in `internal/component/cmd/show` | Core leaf lets `taskguard`, `plugin/process`, `cmd/show`, and `cmd/ze/hub` all share it with zero upward imports; in-memory ring needs no host filesystem dependency and works on read-only appliances. |
| Split `slogutil.LogRing` internally into `general` (512) and `faults` (256) merged by `seq` | Raising single ring to 4096; forcing `ringHandler.Enabled` to true for all levels | Preserves `ringHandler.Enabled` semantics and memory budget while guaranteeing `info`/`debug` bursts never evict `warn`/`error` entries. |
| Enrich `EventRecord` with in-scope `Source`, `Subject`, `Direction` pre-dispatch | Capturing post-dispatch delivery counts or JSON payload snippets | Preserves `event-history-rings.md`'s zero-allocation and pre-dispatch invariants while immediately telling operators and AIs which sender and peer produced each event. |

## Known Limitations
- Inspecting goroutines, profiles, and logs inside forked external plugin processes lives in `plan/post-release/features/spec-ai-diagnostics-3-external-plugin-introspection.md`.
- Cross-boundary operation correlation IDs, reload transaction history, and lock-free diagnostic fast-paths live in `plan/post-release/features/spec-ai-diagnostics-4-correlation-and-wedge-survival.md`.
- Dataplane drift checks, routing/policy `explain` commands, always-on bad-packet rings, and registrable support modules live in `plan/post-release/features/spec-ai-diagnostics-5-drift-explain-and-support-registry.md`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [x] Metadata table present, with a valid Status, Depends, Phase and Updated
- [x] `ai/INDEX.md` keyword table checked
- [x] An `rfc/short/` summary exists for every RFC referenced
- [x] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [x] No code snippets
- [x] Files to Modify names feature code, not only tests
- [x] Current Behavior and Data Flow sections completed
- [x] AC-N rows carry testable assertions
- [x] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [x] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [x] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [x] AC-1..AC-7 all demonstrated
- [x] Every user story has a working path and a passing test
- [x] Wiring Test table complete: every row a concrete test name, none deferred
- [x] The targeted tests for what changed pass, run in the main checkout by the main thread
- [x] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [x] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [x] Architectural Verification table filled, including registration over hardcoding
- [x] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [x] Every A-N confirmed or broken, none `unvalidated`
- [x] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [x] Tests written
- [x] Tests FAIL (paste output)
- [x] Tests PASS (paste output)
- [x] Boundary tests for all numeric inputs
- [x] Functional `.ci` tests for end-to-end behavior
- [x] Interop tests for protocol features (N-A: not protocol work)

### Closure
- [x] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [x] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [x] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [x] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [x] **Commit B:** `remove plan/post-release/features/spec-ai-diagnostics-2-flight-recorder-and-profiles.md` only, in the same `./le commit create` script

---

## Implementation Summary

### What Was Implemented
- Added core leaf package `internal/core/flightrec/` wrapping Go's stdlib `runtime/trace.FlightRecorder` (`10s` / `4 MiB` rolling window) with a bounded 4-slot in-memory snapshot ring (`4 MiB` per snapshot cap via `limitedBuffer`), 30-second `CaptureAuto` cooldown, `CaptureNow`, `Status` (including `AutoCaptureErrors`), `Snapshots`, `SnapshotByID`, and `SetBlockProfileRate`/`BlockProfileRate` atomic rate tracking.
- Wired daemon startup lifecycle in `cmd/ze/hub/main.go` (`runYANGConfig`) before `eng.Start(startCtx)`: `runtime.SetMutexProfileFraction(10)`, `flightrec.SetBlockProfileRate(100_000)`, and non-fatal `flightrec.Start()` (logging `WARN` with `diagnostic.CodeDoctorInternalDefect` on error) paired with `defer flightrec.Stop()`.
- Wired automatic cooldown-throttled flight-recorder capture in `internal/core/taskguard/guard.go` (`Guard.record`: `flightrec.CaptureAuto("taskguard-panic", p.origin())`) and `internal/component/plugin/process/doctor_records.go` (`Process.noteBatch`: `flightrec.CaptureAuto("slow-plugin-delivery", p.config.Name)`).
- Extended `show system profile` (`internal/component/cmd/show/profile.go`, `profile_summary.go`, `yang/ze-cli-show-cmd.yang`) with `type mutex` and `type block`, `sampling-rate` metadata on both `mutex` and `block` (bare and `| summary`), and command-owned `| summary` pipe operation (`command.RegisterPipeFilters` on `"show system profile"`) backed by a pure-stdlib gzipped pprof protobuf summary parser (`summarizePprof`, capped at `32 MiB` decompressed and `32` top functions, supporting packed and unpacked repeated varints).
- Added `show system trace [id <id>]` (`ze-cmd:show-system-trace`) and `request system trace snapshot` (`ze-cmd:request-system-trace-snapshot`) in `internal/component/cmd/show/trace.go`, `show.go`, `yang/ze-cli-show-cmd.yang` (byte-identical `request > system` text to `ze-host-request-cmd.yang`), and `internal/component/plugin/all/testdata/wire-methods.snapshot`, with `opererr` refusals for stopped recorder and unknown IDs.
- Split `slogutil.LogRing` (`internal/core/slogutil/ring.go`) internally into `general` (512 `debug`/`info`) and `faults` (256 `warn`/`error`) circular buffers with shared monotonic sequence merging (`mergeSubRings`), preserving `ringHandler.Enabled` delegation and small-ring test semantics.
- Enriched `EventRecord` (`internal/component/plugin/server/event_ring.go`, `dispatch.go`, `cmd/ze/hub/service_web.go`, `internal/component/cmd/show/show.go`) with `Source`, `Subject`, and `Direction`, populated zero-allocation via `AppendWithMeta` pre-dispatch and emitted conditionally by `show event recent`.
- Updated bundled skill `internal/plugins/skills/data/ze-diagnostics.md` (`skills_test.go`), added promoted functional tests `test/plugin/system-profile-summary-show.ci` and `test/plugin/system-trace-show.ci` with compiled Go fixtures, cleared `spec-ai-diagnostics-3-external-plugin-introspection.md`'s `Depends` cell, and added operator news fragment `website/changes/fragments/ai-diagnostics-2-flight-recorder-and-profiles.md`.

### Bugs Found/Fixed
- **Startup race in `cmd/ze/hub/main.go` (`runYANGConfig`):** Load stress testing `system-trace-show` (64 burners, 16 parallel, 80 iterations) caught a failure on invocation 67/80 because `flightrec.Start()` ran after `eng.Start(startCtx)`, allowing early RPC requests to arrive before the recorder started. Fixed by moving `runtime.SetMutexProfileFraction(10)`, `flightrec.SetBlockProfileRate(100_000)`, and `flightrec.Start()` / `defer flightrec.Stop()` ahead of `eng.Start(startCtx)`; re-run passed 80/80 under load.
- **Run 1 review ISSUE 1 (`block` `sampling-rate` omitted):** Added `flightrec.SetBlockProfileRate`/`BlockProfileRate` and `addProfileSamplingRate` in `profile.go`; covered by `TestBlockProfileRateTracking`, `TestShowSystemProfileSummaryPipe`, and `pluginSystemProfileSummary` (`system-profile-summary-show.ci`).
- **Run 1 review ISSUE 2 (`AppendWithMeta` unwired cross-package export):** Updated `cmd/ze/hub/service_web.go` (`buildWebService`, `runWebOnly`) to call `ring.AppendWithMeta("web", "server.started", "web", "", "")`, clearing `./le repo check`.
- **Run 1 review ISSUE 3 (`summarizePprof` boundary & unpacked varints):** Added `TestProfileSummaryBoundaryAndUnpackedVarints` in `profile_summary_test.go` testing `0`, `32`, and `33` functions (packed) and `33` functions (unpacked `protoWireVarint`).

### Documentation Updates
- `docs/architecture/diagnostics/procfs-diagnostics.md`: documented `mutex`/`block` types, `SetBlockProfileRate`/`BlockProfileRate` sampling-rate reporting, `| summary` pipe operation, `summarizePprof`, and `internal/core/flightrec` (`<!-- source: internal/component/cmd/show/profile.go -- handleShowSystemProfile, profileCPU, profileSnapshot -->`, `<!-- source: internal/component/cmd/show/profile_summary.go -- summarizePprof -->`, `<!-- source: internal/core/flightrec/flightrec.go -- Start, Stop, Status, Snapshots, SnapshotByID, CaptureAuto, CaptureNow, SetBlockProfileRate, BlockProfileRate -->`).
- `docs/architecture/diagnostics/production-diagnostics.md`: documented `show system trace`, `request system trace snapshot`, and severity-split `slogutil.LogRing` (`<!-- source: internal/component/cmd/show/trace.go -- handleShowSystemTrace, handleRequestSystemTraceSnapshot -->`, `<!-- source: internal/core/flightrec/flightrec.go -- Start, Stop, CaptureAuto, CaptureNow -->`, `<!-- source: internal/core/slogutil/ring.go -- LogRing, GlobalLogRing -->`).
- `docs/architecture/diagnostics/event-history-rings.md`: documented `Source`, `Subject`, `Direction` on `EventRecord` and `AppendWithMeta` in `deliverEventCaptured` (`<!-- source: internal/component/plugin/server/event_ring.go -- EventRecord, AppendWithMeta -->`, `<!-- source: internal/component/plugin/server/dispatch.go -- deliverEventCaptured -->`).
- `docs/architecture/diagnostics/crash-capture.md`: documented `taskguard` automatic `flightrec.CaptureAuto("taskguard-panic", p.origin())` (`<!-- source: internal/core/taskguard/guard.go -- Guard.record -->`).
- `docs/architecture/api/commands.md`: Part 2 System commands row and `trace.go` source anchor already landed on HEAD in commit `7ae25509b6` (verified via `git show 7ae25509b6`).
- `docs/guide/command-reference.md`: initial Part 2 entries landed in commit `7ae25509b6`; updated in this commit to note both `mutex` and `block` report `sampling-rate`.
- `ai/INDEX.md`: added discovery row for flight recorder, `show system trace`, `request system trace snapshot`, `show system profile | summary`, `mutex` profile, and `block` profile.
- `./le doc check all` & `./le doc check links`: 0 broken source anchors and 0 broken links across all Part 2 docs (remaining tree-wide findings belong to unrelated pre-existing / concurrent-session files; `doc-drift` row 198 reflects HEAD-built `./le` vs uncommitted `ze-cli-show-cmd.yang`).

### Deviations from Plan
- Added `flightrec.SetBlockProfileRate` and `flightrec.BlockProfileRate` (backed by `atomic.Int64`) so `profileSnapshot` can report `sampling-rate` for `block` profiles despite `runtime.SetBlockProfileRate` returning void.
- Updated `cmd/ze/hub/service_web.go` (`buildWebService`, `runWebOnly`) to use `EventRing.AppendWithMeta` so web startup events carry `source: "web"` and `./le repo check` sees a cross-package non-test caller.
- Surfaced `AutoCaptureErrors` on `flightrec.StatusInfo` and `"auto-capture-errors"` on `show system trace` rather than leaving `autoCaptureErrors` write-only.
- Part 2 edits to `docs/architecture/api/commands.md` and initial `docs/guide/command-reference.md` landed in commit `7ae25509b6`; only the `block` `sampling-rate` doc correction in `docs/guide/command-reference.md` rides in Commit A.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Initially placed `flightrec.Start()` after `eng.Start(startCtx)` in `cmd/ze/hub/main.go`, leaving a window where early plugin RPCs raced recorder activation. | `flightrec.Start()` and contention profiling defaults must activate before `eng.Start(startCtx)` so startup panics, slow deliveries, and early RPCs see an enabled recorder. | 80-iteration stress test (`stress-repro` under 64 burners / 16 parallel) failed on invocation 67/80. | Moved `SetMutexProfileFraction`, `SetBlockProfileRate`, and `flightrec.Start()` + `defer flightrec.Stop()` ahead of `eng.Start(startCtx)`; re-ran `stress-repro` to 80/80 PASS. |
| approach | `profileSnapshot` initially emitted `sampling-rate` only for `mutex`, omitting `block` because `runtime.SetBlockProfileRate` returns void. | AC-1 required `sampling-rate` metadata for both `mutex` and `block`; tracking the configured rate atomically in `flightrec.SetBlockProfileRate`/`BlockProfileRate` satisfies it cleanly. | Independent `/ze-close` Run 1 review. | Added `SetBlockProfileRate`/`BlockProfileRate` in `flightrec`, wired `cmd/ze/hub/main.go` and `profile.go`, extended unit/functional tests and docs. |
| approach | `EventRing.AppendWithMeta` was exported with non-test callers only inside package `server`. | `./le repo check` flags exported symbols lacking a cross-package non-test caller. | Independent `/ze-close` Run 1 review (`./le repo check`). | Updated `cmd/ze/hub/service_web.go` to call `AppendWithMeta("web", "server.started", "web", "", "")`, clearing `./le repo check`. |
| approach | `profile_summary_test.go` initially tested only live Go runtime profiles (packed varints, uncontrolled function count). | Spec Boundary Tests required deterministic verification at `0`, `32`, and `33` functions (`truncated: false/false/true`) plus unpacked `protoWireVarint` samples. | Independent `/ze-close` Run 1 review. | Added `TestProfileSummaryBoundaryAndUnpackedVarints` in `profile_summary_test.go`. |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Extend `show system profile` with `type mutex\|block`, `\| summary` pipe filter, pure-stdlib pprof parser, and daemon startup sampling | Done | `internal/component/cmd/show/profile.go:47-209`, `profile_summary.go:125-778`, `cmd/ze/hub/main.go:1315-1316`, `internal/core/flightrec/flightrec.go:104-118` | Bare `pprof-base64` preserved; `mutex` and `block` report `sampling-rate` |
| Add core leaf `internal/core/flightrec` with bounded snapshot ring, auto-capture triggers, daemon start/stop, and trace commands | Done | `internal/core/flightrec/flightrec.go`, `cmd/ze/hub/main.go:1317-1322`, `internal/core/taskguard/guard.go:190`, `internal/component/plugin/process/doctor_records.go:346`, `internal/component/cmd/show/trace.go` | Pure stdlib leaf; 4-slot / 4 MiB ring; 30s auto cooldown |
| Split `slogutil.LogRing` into `general` and `faults` rings merged by sequence | Done | `internal/core/slogutil/ring.go:54-234` | `general` 512, `faults` 256; O(N) two-pointer merge newest-first |
| Enrich `EventRecord` with `Source`, `Subject`, `Direction` zero-alloc pre-dispatch and surface in `show event recent` | Done | `internal/component/plugin/server/event_ring.go:13-76`, `dispatch.go:408-419`, `cmd/ze/hub/service_web.go:159,224`, `internal/component/cmd/show/show.go:395-405` | `0 allocs/op` verified via `testing.AllocsPerRun` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestProfileSummaryAllTypes`, `TestProfileSummaryBoundaryAndUnpackedVarints`, `TestProfileTypesMatchTheModel`, `TestShowSystemProfileSummaryPipe`, `TestBlockProfileRateTracking`, `test/plugin/system-profile-summary-show.ci`, `test/plugin/system-profile-show.ci` | All 6 types, `\| summary`, `0/32/33` cap, `sampling-rate` on `mutex` & `block`, bare `pprof-base64` preserved |
| AC-2 | Done | `TestProfileSummaryMalformedInput` | Empty, non-gzip, truncated gzip/submsg/varint, corrupt wire type, out-of-range string index, >32 MiB rejected without panic |
| AC-3 | Done | `TestFlightRecStartCaptureRingAndCooldown`, `TestFlightRecSnapshotByteCap`, `TestHandleSystemTraceSnapshotAndFetch`, `test/plugin/system-trace-show.ci` | Non-fatal boot start, on-demand snapshot, metadata-only list, `go-trace-base64` fetch, `opererr` refusals |
| AC-4 | Done | `TestGuardTriggersFlightRecorderCapture`, `TestSlowDeliveryTriggersFlightRecorderCapture`, `TestFlightRecStartCaptureRingAndCooldown` | Auto-capture on panic and >500ms delivery, 30s cooldown, silent no-op when stopped |
| AC-5 | Done | `TestLogRingFaultsSurviveInfoBurst`, existing `TestRingHandlerEnabled` | 612 `info` logs evict 0 `warn`/`error` entries; 256-slot fault rotation; `ringHandler.Enabled` unchanged |
| AC-6 | Done | `TestEventRingAppendWithMeta`, `TestShowEventRecentIncludesEnrichedMetadata` | Zero-alloc `AppendWithMeta` pre-dispatch; `source`/`subject`/`direction` emitted when non-empty |
| AC-7 | Done | `TestSkillsDiagnosticsIncludesPart2Commands`, `TestSkillsDiagnosticsIncludesLiveTriagePlaybook` | Bundled `ze-diagnostics` covers `\| summary`, `mutex`/`block`, `show system trace`, `request system trace snapshot` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestFlightRecStartCaptureRingAndCooldown` | Done | `internal/core/flightrec/flightrec_test.go:24` | PASS (`-race`) |
| `TestFlightRecSnapshotByteCap` | Done | `internal/core/flightrec/flightrec_test.go:112` | PASS (`-race`) |
| `TestBlockProfileRateTracking` | Done | `internal/core/flightrec/flightrec_test.go:192` | Added in Run 1 fix; PASS (`-race`) |
| `TestGuardTriggersFlightRecorderCapture` | Done | `internal/core/taskguard/guard_test.go:470` | PASS (`-race`) |
| `TestSlowDeliveryTriggersFlightRecorderCapture` | Done | `internal/component/plugin/process/doctor_records_test.go:170` | PASS |
| `TestLogRingFaultsSurviveInfoBurst` | Done | `internal/core/slogutil/ring_test.go:291` | PASS (`-race`) |
| `TestEventRingAppendWithMeta` | Done | `internal/component/plugin/server/event_ring_test.go:112` | PASS (`0 allocs/op`) |
| `TestShowEventRecentIncludesEnrichedMetadata` | Done | `internal/component/cmd/show/show_test.go:125` | PASS (`-race`) |
| `TestProfileSummaryAllTypes` | Done | `internal/component/cmd/show/profile_summary_test.go:29` | PASS (`-race`) |
| `TestProfileSummaryMalformedInput` | Done | `internal/component/cmd/show/profile_summary_test.go:70` | PASS (`-race`) |
| `TestProfileSummaryBoundaryAndUnpackedVarints` | Done | `internal/component/cmd/show/profile_summary_test.go:202` | Added in Run 1 fix; PASS (`-race`) |
| `TestProfileTypesMatchTheModel` | Done | `internal/component/cmd/show/profile_test.go:100` | PASS (`-race`) |
| `TestShowSystemProfileSummaryPipe` | Done | `internal/component/cmd/show/profile_test.go:154` | Added for pipe + `sampling-rate`; PASS (`-race`) |
| `TestHandleSystemTraceSnapshotAndFetch` | Done | `internal/component/cmd/show/trace_test.go:28` | PASS (`-race`) |
| `TestSkillsDiagnosticsIncludesPart2Commands` | Done | `internal/plugins/skills/skills_test.go:288` | PASS |
| `test/plugin/system-profile-summary-show.ci` | Done | `test/plugin/system-profile-summary-show.ci` | 80/80 under load + promoted PASS |
| `test/plugin/system-trace-show.ci` | Done | `test/plugin/system-trace-show.ci` | 80/80 under load + promoted PASS |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `cmd/ze/hub/main.go` | Done | Modified |
| `cmd/ze/hub/service_web.go` | Changed | Added in Run 1 fix for `AppendWithMeta` cross-package caller (recorded in Deviations) |
| `internal/core/taskguard/guard.go` & `guard_test.go` | Done | Modified |
| `internal/core/slogutil/ring.go` & `ring_test.go` | Done | Modified |
| `internal/component/plugin/process/doctor_records.go` & `doctor_records_test.go` | Done | Modified |
| `internal/component/plugin/server/event_ring.go`, `event_ring_test.go`, `dispatch.go` | Done | Modified |
| `internal/component/cmd/show/show.go`, `show_test.go`, `profile.go`, `profile_test.go`, `yang/ze-cli-show-cmd.yang` | Done | Modified |
| `internal/component/plugin/all/testdata/wire-methods.snapshot` | Done | Modified |
| `internal/plugins/skills/data/ze-diagnostics.md` & `skills_test.go` | Done | Modified |
| `docs/architecture/diagnostics/{procfs-diagnostics,production-diagnostics,event-history-rings,crash-capture}.md`, `ai/INDEX.md` | Done | Modified |
| `docs/architecture/api/commands.md` & `docs/guide/command-reference.md` | Done | Initial edits landed in `7ae25509b6`; `command-reference.md` updated for `block` `sampling-rate` in Commit A |
| `internal/core/flightrec/{flightrec.go,flightrec_test.go}` | Done | Created |
| `internal/component/cmd/show/{profile_summary.go,profile_summary_test.go,trace.go,trace_test.go}` | Done | Created |
| `internal/test/fixture/{register,plugin}_system_{profile_summary,trace}_show.go` | Done | Created |
| `test/plugin/{system-profile-summary-show,system-trace-show}.ci` | Done | Created & promoted |

### Audit Summary
- **Total items:** 47 (4 task requirements, 7 ACs, 17 tests, 19 file groups)
- **Done:** 46
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (`cmd/ze/hub/service_web.go` added to wire `AppendWithMeta` cross-package)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| AI-readable runtime profile summaries (`\| summary`) and `mutex`/`block` contention sampling without external dependencies | Functional + unit tests | `test/plugin/system-profile-summary-show.ci` (PASS, 80/80 under load, asserts `format=summary`, `top-functions`, `truncated`, and `sampling-rate > 0` on `mutex` and `block`); `test/plugin/system-profile-show.ci` (PASS, bare `pprof-base64` preserved); `TestProfileSummaryAllTypes`, `TestProfileSummaryMalformedInput`, `TestProfileSummaryBoundaryAndUnpackedVarints`, `TestProfileTypesMatchTheModel`, `TestShowSystemProfileSummaryPipe`, `TestBlockProfileRateTracking` (all PASS under `-race`). |
| Always-on pre-incident Go execution flight recorder with bounded memory, automatic triggers, and CLI/MCP commands | Functional + unit tests | `test/plugin/system-trace-show.ci` (PASS, 80/80 under load, proves boot activation, `request system trace snapshot`, `show system trace`, and `show system trace id <id>` non-empty `go-trace-base64` decode); `TestFlightRecStartCaptureRingAndCooldown`, `TestFlightRecSnapshotByteCap`, `TestGuardTriggersFlightRecorderCapture`, `TestSlowDeliveryTriggersFlightRecorderCapture`, `TestHandleSystemTraceSnapshotAndFetch` (all PASS under `-race`). |
| Severity-split `slogutil.LogRing` preventing `info`/`debug` bursts from evicting `warn`/`error` entries | Unit tests | `TestLogRingFaultsSurviveInfoBurst` (`go test -race ./internal/core/slogutil/` PASS: 612 `info` logs evict 0 `warn`/`error` records; fault ring rotates cleanly at 256; `Recent(0)` merges newest-first by sequence). |
| Zero-allocation enriched `EventRecord` metadata (`source`, `subject`, `direction`) surfaced in `show event recent` | Unit + alloc tests | `TestEventRingAppendWithMeta` (`./le test unit pkg path ./internal/component/plugin/server/ run 'TestGlobalEventRing\|TestEventRing'` PASS, `testing.AllocsPerRun == 0`); `TestShowEventRecentIncludesEnrichedMetadata` (`./le test unit pkg path ./internal/component/cmd/show/` PASS). |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | All in-scope requirements of Part 2 are complete. Follow-on Parts 3, 4, and 5 remain in their own existing specs (`plan/post-release/features/spec-ai-diagnostics-3-external-plugin-introspection.md`, `plan/post-release/features/spec-ai-diagnostics-4-correlation-and-wedge-survival.md`, `plan/post-release/features/spec-ai-diagnostics-5-drift-explain-and-support-registry.md`). | `plan/post-release/features/spec-ai-diagnostics-3-external-plugin-introspection.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/ai-diagnostics-2-flight-recorder-and-profiles-fe7f3030-a1b7-461b-a30f-54df5bbef2c9.md` |
| `./le spec review check` | `clean` (`review_gate: OK (29 code files, clean, hashes match tmp/review/ai-diagnostics-2-flight-recorder-and-profiles-fe7f3030-a1b7-461b-a30f-54df5bbef2c9.md)`) |
| Rounds | `3` |
| Reviewer lenses used | logic+wiring, security+bounds+edge-cases, design-documents (31/31 pages, 31/31 decision rows), ze-go-style (all 7 questions), spec-completeness, operator-error surface |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `show system profile type block` omitted `sampling-rate` metadata required by AC-1 | `internal/component/cmd/show/profile.go` (`profileSnapshot`), `cmd/ze/hub/main.go` (`runYANGConfig`) | Added `flightrec.SetBlockProfileRate`/`BlockProfileRate`, wired `cmd/ze/hub/main.go` and `addProfileSamplingRate` in `profile.go`, added assertions in `TestBlockProfileRateTracking`, `TestShowSystemProfileSummaryPipe`, `pluginSystemProfileSummary`, and updated docs |
| 2 | ISSUE | `./le repo check` flagged exported `EventRing.AppendWithMeta` as having no cross-package non-test caller | `internal/component/plugin/server/event_ring.go` (`AppendWithMeta`) | Updated `cmd/ze/hub/service_web.go` (`buildWebService`, `runWebOnly`) to call `ring.AppendWithMeta("web", "server.started", "web", "", "")`; `./le repo check` now reports 0 Part 2 findings |
| 3 | ISSUE | `summarizePprof` boundary caps (`0`, `32`, `33` functions) and unpacked `protoWireVarint` sample encoding lacked deterministic unit tests | `internal/component/cmd/show/profile_summary_test.go` | Added `TestProfileSummaryBoundaryAndUnpackedVarints` covering `0`, `32`, `33` packed and `33` unpacked synthetic profiles |
| 4 | ISSUE | `./le arch operator-error check` flagged `handleShowSystemProfile` duration-bound refusal for returning a plain string error instead of `plugin.OperatorErrorResponse(opererr.New(...))` | `internal/component/cmd/show/profile.go` (`handleShowSystemProfile`) | Upgraded both `d < time.Second` and `d > maxCPUProfileDuration` refusals to `plugin.OperatorErrorResponse(opererr.New(...))` with concrete fix `ze show system profile type cpu duration 10s` |

### Run 3
Independent review over the `profile.go` duration-bound `opererr.New` fix and the complete 39-file Commit A non-spec population: **0 BLOCKER, 0 ISSUE, 0 NOTE (CLEAN)**.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/flightrec/flightrec.go` | Yes | `ls -1` verified 2026-11-27 |
| `internal/core/flightrec/flightrec_test.go` | Yes | `ls -1` verified 2026-11-27 |
| `internal/component/cmd/show/profile_summary.go` | Yes | `ls -1` verified 2026-11-27 |
| `internal/component/cmd/show/profile_summary_test.go` | Yes | `ls -1` verified 2026-11-27 |
| `internal/component/cmd/show/trace.go` | Yes | `ls -1` verified 2026-11-27 |
| `internal/component/cmd/show/trace_test.go` | Yes | `ls -1` verified 2026-11-27 |
| `internal/test/fixture/register_system_profile_summary_show.go` | Yes | `ls -1` verified 2026-11-27 |
| `internal/test/fixture/plugin_system_profile_summary_show.go` | Yes | `ls -1` verified 2026-11-27 |
| `internal/test/fixture/register_system_trace_show.go` | Yes | `ls -1` verified 2026-11-27 |
| `internal/test/fixture/plugin_system_trace_show.go` | Yes | `ls -1` verified 2026-11-27 |
| `test/plugin/system-profile-summary-show.ci` | Yes | `ls -1` verified 2026-11-27 |
| `test/plugin/system-trace-show.ci` | Yes | `ls -1` verified 2026-11-27 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | `mutex`/`block` types, `\| summary` pipe filter, top-32 summary JSON, `sampling-rate` on both `mutex` & `block`, bare `pprof-base64` preserved | `profile.go` (`handleShowSystemProfile`, `addProfileSamplingRate`), `profile_summary.go` (`summarizePprof`), `go test -race ./internal/component/cmd/show/` PASS (`7.532s`), `system-profile-show` & `system-profile-summary-show` PASS |
| AC-2 | Malformed / >32 MiB gzipped pprof inputs return error without panicking | `profile_summary.go` (`gunzipProfile`, `pprofString`, `readProtoDelimited`), `TestProfileSummaryMalformedInput` PASS (`-race`) |
| AC-3 | Non-fatal boot `flightrec.Start()`, `request system trace snapshot`, `show system trace [id <id>]`, `opererr` refusals | `cmd/ze/hub/main.go:1315-1322`, `trace.go` (`handleShowSystemTrace`, `handleRequestSystemTraceSnapshot`), `TestHandleSystemTraceSnapshotAndFetch` PASS, `system-trace-show.ci` PASS (80/80 load + promoted) |
| AC-4 | `taskguard` panic and slow plugin delivery auto-capture with 30s cooldown, silent when stopped | `guard.go:190`, `doctor_records.go:346`, `TestGuardTriggersFlightRecorderCapture` & `TestSlowDeliveryTriggersFlightRecorderCapture` PASS |
| AC-5 | Severity-split `LogRing` preserves `warn`/`error` through >512 `info` burst | `internal/core/slogutil/ring.go` (`LogRing`, `mergeSubRings`), `go test -race ./internal/core/slogutil/` PASS |
| AC-6 | Zero-alloc pre-dispatch `EventRecord` metadata (`source`, `subject`, `direction`) in `show event recent` | `event_ring.go` (`AppendWithMeta`), `dispatch.go:408-419`, `show.go:395-405`, `TestEventRingAppendWithMeta` (`0 allocs/op`) & `TestShowEventRecentIncludesEnrichedMetadata` PASS |
| AC-7 | `show skill ze-diagnostics` documents Part 2 commands while preserving Part 1 & config validation | `internal/plugins/skills/data/ze-diagnostics.md:69-71`, `./le test unit pkg path ./internal/plugins/skills/` PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `show system profile type heap\|mutex\|block \| summary` | `test/plugin/system-profile-summary-show.ci` | Read `.ci` and `plugin_system_profile_summary_show.go`: drives live `ze` daemon with external plugin fixture, asserts `format=summary`, `top-functions`, `truncated`, and `sampling-rate > 0` on `mutex` and `block`; 3/3 functional run PASS |
| `request system trace snapshot` & `show system trace [id <id>]` | `test/plugin/system-trace-show.ci` | Read `.ci` and `plugin_system_trace_show.go`: drives live `ze` daemon proving boot activation of `flightrec`, on-demand snapshot capture, metadata listing (`enabled=true`), and `go-trace-base64` decode; 3/3 functional run PASS |
| Bare `show system profile heap` preservation | `test/plugin/system-profile-show.ci` | Read `.ci`: unchanged default `pprof-base64` verified; 3/3 functional run PASS |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Go `1.27.4` stdlib `runtime/trace.FlightRecorder` compiles and passes `go test -race ./internal/core/flightrec/` (`1.117s`) and live daemon `test/plugin/system-trace-show.ci` (80/80 under load). |
| A-2 | confirmed | Pure-stdlib `summarizePprof` decodes real Go `pprof.Lookup` output for `heap`, `goroutine`, `allocs`, `mutex`, `block`, and `StartCPUProfile` (`cpu`) plus synthetic packed and unpacked varints (`TestProfileSummaryAllTypes`, `TestProfileSummaryBoundaryAndUnpackedVarints`, `TestProfileSummaryMalformedInput` all PASS under `-race`). |
| A-3 | confirmed | `runtime.SetMutexProfileFraction(10)` and `flightrec.SetBlockProfileRate(100_000)` in `cmd/ze/hub/main.go` pass all unit tests, `system-profile-summary-show.ci` (80/80 under load), and `system-trace-show.ci` (80/80 under load) with zero regressions. |
| A-4 | confirmed | `container request > container system` in `ze-cli-show-cmd.yang` is byte-identical to `ze-host-request-cmd.yang`; `./le --name diag2 cli grammar` (549 commands OK), `./le --name diag2 cli ownership` (OK), and `./le --name diag2 doc yang-contract command-contract` (581/581 OK) all passed. |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| CLI command reference (`docs/guide/command-reference.md`) | `show system profile` (`mutex`, `block`, `\| summary`, `sampling-rate`), `show system trace`, `request system trace snapshot`, `show event recent` verified against `profile.go`, `trace.go`, `show.go`, and commit `7ae25509b6` + Commit A hunk | Yes |
| API/RPC docs (`docs/architecture/api/commands.md`) | System row (`show system trace`, `request system trace snapshot`) and `trace.go` source anchor verified in `git show 7ae25509b6` | Yes |
| Architecture diagnostics (`procfs-diagnostics.md`, `production-diagnostics.md`, `event-history-rings.md`, `crash-capture.md`) | Every factual claim and `<!-- source: ... -->` anchor verified against `profile.go`, `profile_summary.go`, `flightrec.go`, `trace.go`, `ring.go`, `event_ring.go`, `dispatch.go`, `guard.go`; 0 broken anchors/links in `./le doc check all` / `./le doc check links` | Yes |
| Discovery & inventory (`ai/INDEX.md`, `wire-methods.snapshot`, `ze-diagnostics.md`) | `ai/INDEX.md` row present; `wire-methods.snapshot` sorted and passes `TestRegisteredWireMethods`; `ze-diagnostics.md` verified by `TestSkillsDiagnosticsIncludesPart2Commands` | Yes |
| Unaffected categories (config syntax, wire format, plugin SDK, RFC compliance, features/, comparison, Prometheus counters) | `git diff --name-only` confirms zero changes to config YANG, wire codecs, `pkg/plugin/`, `rfc/`, `features/`, or metric declarations; `git grep` of `docs/` source anchors shows unaffected `// Design:` pages (`process-protocol.md`, `doctor-and-health-checks.md`, `hub-architecture.md`, `web-interface.md`, `ai-first.md`) remain accurate | Yes |
