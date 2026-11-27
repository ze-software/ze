# Spec: ai-diagnostics-1-triage-and-fault-visibility

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli, plugin, docs |
| Priority | P3 |
| Release reason | Adds a single-call live triage surface, wires recovered task panics onto the report bus, and updates agent diagnostic guidance so an AI assistant over MCP or CLI can diagnose runtime bugs in one round-trip; existing shipped behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | - |
| Phase | 4/4 |
| Handoff | - |
| Updated | 2026-11-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

When a software bug causes Ze to misbehave at runtime, an AI assistant connected over MCP or CLI currently has to issue many separate queries (`show version --extended`, `show health`, `show doctor`, `show errors`, `show warnings`, `show crashes`, `show reload-status`, `show log recent`) to assemble an initial picture, and three key gaps remain:

1. Recovered network-input panics contained by `taskguard.Task.Contain` (`internal/core/taskguard/guard.go`) write a crash file, bump `ze_task_panics_total`, and log an `ERROR` line without a `code` attribute, and they do not raise an error event on `internal/core/report`. As a result, `show errors` and `show doctor`'s `recent-events` check do not surface contained internal defects, and the log line lacks the `doctor-internal-defect` code that `ai/rules/user-facing-errors.md` requires.
2. There is no single read-only command that returns a bounded, structured live triage snapshot combining build provenance, component health, daemon doctor findings, report-bus errors and warnings, recent crashes, unhealthy plugin state, reload status, recent warning/error log entries, and process runtime vitals.
3. The bundled `ze-diagnostics` skill (`internal/plugins/skills/data/ze-diagnostics.md`) and `ze` guides cover only configuration validation, leaving AI assistants with no embedded live bug-triage workflow.

This spec is Part 1 of the 5-part AI diagnostics architecture series:
- Part 1 (this spec): single-call `show system triage` (auto-exposed to MCP via the shared command registry with self-describing YANG help/description), `taskguard` panic reporting onto `report` with `doctor-internal-defect`, structured build identity helper in `internal/core/version`, and a live-triage playbook in the bundled skills.
- Part 2 (`plan/post-release/features/spec-ai-diagnostics-2-flight-recorder-and-profiles.md`): readable `show system profile` summaries with `mutex`/`block`, Go `runtime/trace.FlightRecorder`, severity-split log ring, and richer `EventRecord`.
- Part 3 (`plan/post-release/features/spec-ai-diagnostics-3-external-plugin-introspection.md`): SDK runtime inspection callbacks for forked external plugins.
- Part 4 (`plan/post-release/features/spec-ai-diagnostics-4-correlation-and-wedge-survival.md`): cross-boundary operation correlation IDs, reload transaction journal, and lock-free diagnostic fast-path / watchdog.
- Part 5 (`plan/post-release/features/spec-ai-diagnostics-5-drift-explain-and-support-registry.md`): dataplane drift checks, routing/policy explain commands, always-on bad-packet rings, and registrable support modules.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/diagnostics/production-diagnostics.md` - single command registry feeding CLI and MCP, structured log ring, health registry
  → Decision: `show system triage` registers as a standard YANG command under `show system` (`ze-cmd:show-system-triage`), so CLI, SSH, web, REST, gRPC, and MCP all receive it automatically through `tools/list` and `ze_reference` with zero edits to `internal/component/mcp`.
  → Constraint: Do not add a bespoke MCP-only tool or hardcode command names inside `internal/component/mcp`.
- [ ] `docs/architecture/diagnostics/crash-capture.md` - recovered panics via `taskguard.Task.Contain` and crash file listing via `crashlog.CrashListFields`
  → Decision: Extend `taskguard.Guard.record` to add `"code", report.CodeInternalDefect` to the ERROR log line and call `report.RaiseErrorFix` with `ze show crashes name <file>` (or `ze show log recent level error` when no crash file was written).
  → Constraint: Preserve the existing crash file creation, metric increment (`ze_task_panics_total`), log fields (`component`, `task`, `unit`, `message-type`, `panic`, `restart`, `next`, `stack`), and `end(p)` callback order.
- [ ] `docs/architecture/doctor-and-health-checks.md` - three-tier self-diagnosis, `recent-events` check, `PluginDoctorSnapshot`, and `doctor-internal-defect`
  → Decision: Reuse `querySystemMap(ctx, "show doctor")` and `pluginserver.PluginDoctorSnapshot()` inside `show system triage` rather than importing `internal/component/doctor` or duplicating any check.
  → Constraint: `internal/core/taskguard` cannot import `internal/core/diagnostic` (which imports `internal/component/host`); use `report.CodeInternalDefect` (already declared in `internal/core/report` for core leaf packages).
- [ ] `docs/architecture/api/commands.md` - verb-first command grammar, pipe rules, dispatch keys as YANG paths
  → Decision: Place `triage` under `show system` (`show system triage`) with no positional arguments; callers narrow or format the structured JSON payload with standard pipe operators (`| json`, `| yaml`, `| table`).
  → Constraint: Response payload must be structured `plugin.Map` with lowercase kebab-case keys, never pre-formatted text.
- [ ] `docs/features/ai-first.md` - bundled skills (`show skills`, `show skill <name>`) and structured diagnostics
  → Decision: Extend `internal/plugins/skills/data/ze-diagnostics.md`, `ze.md`, and `ze-full.md` with the live runtime bug-triage workflow starting from `show system triage`.
  → Constraint: Keep existing config-validation commands, diagnostic shape fields, and agent-setup instructions (`ze create agent skill`) intact.

**Key insights:**
- `internal/core/report` already declares `const CodeInternalDefect = "doctor-internal-defect"` specifically so core leaf packages (`internal/core/events`, `internal/core/taskguard`) can raise internal defects without importing `internal/core/diagnostic`.
- `internal/component/plugin/server/register.go` already registers `diagnostic.CodeDoctorInternalDefect` as an event code for the `recent-events` doctor check, and `internal/core/diagnostic/codes_plugin_infra.go` already documents it for `ze show diagnostic doctor-internal-defect`.
- `internal/component/cmd/show` already imports `internal/core/health`, `internal/core/report`, `internal/core/version`, `internal/core/slogutil`, and `internal/component/plugin/server`, and already has `querySystemMap(ctx, name)` in `system.go` to call daemon show commands through the caller's dispatcher without extra component imports.
- `internal/core/version/version.go` already reads `vcs.revision`, `vcs.modified`, `vcs.time`, `GoVersion`, and `/perm/ze/build.json` via `readInfo()` and `readImageManifest()`, but only formats them into multi-line text in `Extended()`; exposing a structured value method `BuildIdentity()` lets `show system triage` emit machine-readable provenance without re-parsing text.
- `internal/component/mcp/discover.go` must not hardcode command names from the registry (`ai/doors/api-surfaces.md`); MCP clients discover `show system triage` automatically via `tools/list`, `ze_reference`, and `show skill ze-diagnostics`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/taskguard/guard.go` - `Guard.record` increments `ze_task_panics_total`, writes a recovered crash record via `crashlog.RecordRecoveredPanic`, and logs an `ERROR` line without a `code` attribute and without raising an error on `internal/core/report`.
- [ ] `internal/core/report/report.go` - provides `RaiseErrorFix`, `FitMessage`, `CodeInternalDefect` (`"doctor-internal-defect"`), `Warnings()`, `Errors(limit)`, and `Losses()`.
- [ ] `internal/core/version/version.go` - provides `Release()`, `BuildDate()`, `Short()`, `Extended()`, and private `readInfo()` / `readImageManifest()` reading Go build info and appliance manifest.
- [ ] `internal/core/health/registry.go` - provides `health.Check()` returning `health.Report{Status, Components, CheckedAt}`.
- [ ] `internal/core/crashlog/list.go` - provides `crashlog.CrashListFields()` and `crashlog.CrashDir()`.
- [ ] `internal/core/slogutil/ring.go` - provides `slogutil.GlobalLogRing().Snapshot(limit, level, component)` returning structured `LogEntry` values with `Attributes()`.
- [ ] `internal/component/plugin/server/doctor_snapshot.go` - provides `pluginserver.PluginDoctorSnapshot()` returning value-type `DoctorSnapshot` (configured/running/down/disabled plugins, startup errors, delivery problems, enricher failures, delivery disagreements, dormant receptions, unbacked claims, engine handler panics).
- [ ] `internal/component/plugin/server/reload_generation.go` - provides `Server.ReloadStatus()` returning `(generation, outcome, at)`.
- [ ] `internal/component/cmd/show/show.go` - registers process-global `ze-cmd:show-*` RPCs (`show-health`, `show-warnings`, `show-errors`, `show-version`, `show-uptime`, etc.).
- [ ] `internal/component/cmd/show/system.go` - implements `show system memory/cpu/date/platform/subsystem list` and `querySystemMap(ctx, name)` for calling another daemon `show` command with the caller's context and authorization.
- [ ] `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` - declares `container show { container system { ... } }` command tree.
- [ ] `internal/plugins/skills/data/ze-diagnostics.md`, `ze.md`, `ze-full.md` - bundled Markdown skills served by `show skill`.

**Behavior to preserve:**
- Every existing `show` command (`show health`, `show doctor`, `show errors`, `show warnings`, `show crashes`, `show reload-status`, `show log recent`, `show version`) keeps its exact path, arguments, and payload schema.
- `taskguard.Task.Contain` keeps its existing panic value, stack capture, crash file naming and rotation, metric series `ze_task_panics_total{component,task}`, `end(*Panic)` invocation, and all existing log attribute keys (`component`, `task`, `unit`, `message-type`, `panic`, `restart`, `next`, `stack`).
- `taskguard` with no panic remains silent (zero logs, zero report raises, zero crash files).
- `internal/component/mcp` stays untouched; `server/discover` and tool generation continue to derive commands solely from the live registry and YANG schema.
- Bundled skill names (`ze`, `ze-diagnostics`, `ze-config`, `ze-commands`, `ze-agent`), existing config-validation commands, and `ze create agent skill` instructions stay intact.

**Behavior to change:**
- Add `version.Identity` value struct and `version.BuildIdentity()` in `internal/core/version/version.go` sharing `readInfo()` and `readImageManifest()`.
- Add `show system triage` (`ze-cmd:show-system-triage`) in `internal/component/cmd/show/triage.go` and `internal/component/cmd/show/yang/ze-cli-show-cmd.yang`, and update `internal/component/plugin/all/testdata/wire-methods.snapshot`.
- Update `taskguard.Guard.record` in `internal/core/taskguard/guard.go` to add `"code", report.CodeInternalDefect` to the `ERROR` log line and raise a bounded error event on `report.RaiseErrorFix` under `report.CodeInternalDefect` with structured detail (`task`, `unit`, `message-type`, `panic`, `crash-report`) and `fix`.
- Expand `internal/plugins/skills/data/ze-diagnostics.md`, `ze.md`, and `ze-full.md` with the live runtime bug-triage workflow starting from `show system triage`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Shared command registry and YANG RPC dispatch | `internal/component/cmd/show/show.go` (`pluginserver.RegisterRPCs`), `internal/component/cmd/show/yang/ze-cli-show-cmd.yang`, `docs/architecture/diagnostics/production-diagnostics.md` ("CLI first, MCP for free") | Registers process-global `show` commands once so CLI, SSH, web, REST, gRPC, and MCP all dispatch them identically. | Extends: registers `show system triage` (`ze-cmd:show-system-triage`) in the same package and YANG tree so MCP gains it automatically via `tools/list` and `ze_reference` with zero MCP adapter changes. |
| Nested daemon show query helper | `internal/component/cmd/show/system.go` (`querySystemMap`) | Dispatches another daemon `show` command using the caller's `CommandContext`, preserving cancellation and authorization without cross-component imports. | Extends: `handleShowSystemTriage` calls `querySystemMap(ctx, "show doctor")` to include live daemon + plugin doctor findings without importing `internal/component/doctor`. |
| Operational report bus | `internal/core/report/report.go` (`RaiseErrorFix`, `FitMessage`, `CodeInternalDefect`, `Warnings`, `Errors`, `Losses`) | Holds bounded active warnings and recent error events across subsystems, consumed by `show warnings`, `show errors`, and `show doctor`'s `recent-events` check. | Extends: `taskguard.Guard.record` raises recovered panics via `report.RaiseErrorFix` under `report.CodeInternalDefect`, and `show system triage` snapshots bounded errors, warnings, and bus loss counters. |
| Plugin doctor snapshot | `internal/component/plugin/server/doctor_snapshot.go` (`PluginDoctorSnapshot`) | Publishes value-type plugin lifecycle state, startup errors, delivery problems, enricher failures, delivery disagreements, and engine handler panics for runtime checks. | Extends: `show system triage` reads `PluginDoctorSnapshot()` directly to report plugin counts and unhealthy/noteworthy plugins without adding new server locks or state. |
| Health registry, log ring, crash listing, reload fence, build info | `internal/core/health/registry.go` (`Check`), `internal/core/slogutil/ring.go` (`GlobalLogRing`), `internal/core/crashlog/list.go` (`CrashListFields`), `internal/component/plugin/server/reload_generation.go` (`ReloadStatus`), `internal/core/version/version.go` (`readInfo`) | Each independently holds one slice of daemon runtime state. | Extends: `version` adds a structured `BuildIdentity()` value reader over `readInfo()`, and `show system triage` aggregates bounded snapshots from each existing producer into one response. |
| Panic containment boundary | `internal/core/taskguard/guard.go` (`Task.Contain`, `Guard.record`) | Recovers panics at network-input task entries, writes a recovered crash file, increments `ze_task_panics_total`, and logs an ERROR line. | Extends: adds `"code", report.CodeInternalDefect` to the log line and calls `report.RaiseErrorFix` so recovered panics surface immediately in `show errors`, `show doctor`, and `show system triage`. |
| Bundled agent skills | `internal/plugins/skills/data/ze-diagnostics.md`, `ze.md`, `ze-full.md` | Guides AI assistants on Ze's config validation workflow and entry points. | Extends: adds the live runtime bug-triage workflow starting from `show system triage`. |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | system door | "The CLI, SSH, the web UI, REST, gRPC and MCP dispatch into the same command registry." | agrees | `show system triage` is declared once in `ze-cli-show-cmd.yang` and `internal/component/cmd/show`, reaching every surface through the shared dispatcher without editing `internal/component/mcp`. |
| `ai/doors/core.md` | Boundaries | `internal/core/version/version.go`, `internal/core/taskguard/guard.go` | "Never import upward." | agrees | `internal/core/taskguard` imports only `internal/core/report` (same core leaf tier), using `report.CodeInternalDefect` rather than importing `internal/core/diagnostic`. |
| `ai/doors/commands-cli.md` | Boundaries | `internal/component/cmd/show/triage.go`, `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` | "A handler answers data, never text." | agrees | `handleShowSystemTriage` returns structured `plugin.Map` with lowercase kebab-case keys routed through `ApplyPipes`. |
| `ai/doors/plugin-infra.md` | Where it sits | `internal/component/plugin/all/testdata/wire-methods.snapshot` | "It is infrastructure and cannot be compiled out." | silent | Only the generated wire-methods snapshot adds `ze-cmd:show-system-triage`; plugin infrastructure behavior and wire protocol are unchanged. |
| `ai/doors/system-plugins.md` | Boundaries | `internal/plugins/skills/skills_test.go` | "A plugin owns its whole surface." | silent | Only embedded Markdown skill text inside `internal/plugins/skills/data/` and its skill tests are expanded; plugin boundaries and commands are unchanged. |
| `ai/doors/le-tooling.md` | Boundaries | `internal/test/fixture/plugin_system_triage_show.go`, `test/plugin/system-triage-show.ci` | "A compiled Go fixture returns an error when an assertion fails" | agrees | The new `.ci` functional test and compiled fixture exercise the real daemon and return an error if any triage field assertion fails. |
| `docs/architecture/config/syntax.md` | Schema Stamp | `internal/core/version/version.go` | "" | silent | `syntax.md` anchors `version.CompareReleases` and `version.IsNewerRelease` for config schema stamps; adding `BuildIdentity()` does not change release comparison or stamping. |
| `docs/architecture/web-interface.md` | Security Headers | `internal/core/version/version.go` | "" | silent | `web-interface.md` anchors `version.HTTPHeaderHidden` for `X-Ze-Version`; adding `BuildIdentity()` does not change HTTP banner behavior. |
| `docs/architecture/api/commands.md` | Design decisions | `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | `show system triage` names the subject and stops with no rendering flags or scope subcommands; formatting and filtering use standard pipes. |
| `docs/architecture/api/commands.md` | Design decisions | `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | `show system triage` defines no custom pipe override and lets the generic pipe catalog render its `plugin.Map` payload. |
| `docs/architecture/api/commands.md` | Design decisions | `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | `show system triage` is strictly read-only and produces one structured document before pipes operate on it. |
| `docs/architecture/diagnostics/production-diagnostics.md` | CLI first, MCP for free | `internal/component/cmd/show/triage.go`, `docs/architecture/diagnostics/production-diagnostics.md` | "Every diagnostic is a YANG RPC registered as a CLI command. The MCP tool list is generated from the registered RPCs, so a new diagnostic appears to an AI agent with no MCP code change." | agrees | `show system triage` follows this exact pattern with zero edits to `internal/component/mcp`, and updates `production-diagnostics.md` to document it. |
| `docs/architecture/diagnostics/crash-capture.md` | Recovered panics | `internal/core/taskguard/guard.go`, `docs/architecture/diagnostics/crash-capture.md` | "A recovered panic is never silent." | agrees | Adding the `doctor-internal-defect` log code and `report.RaiseErrorFix` makes recovered panics visible in `show errors`, `show doctor`, and `show system triage`, and `crash-capture.md` is updated to document the report-bus output row. |
| `docs/features/ai-first.md` | Bundled Skills | `internal/plugins/skills/skills_test.go` | "The installed binary serves the workflow guides embedded when it was built." | agrees | Extending the embedded Markdown guides keeps version-matched triage guidance inside the binary. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- **Triage query:** Operator or AI assistant invokes `show system triage` via CLI (`ze cli -c "show system triage | json"`), SSH, web CLI, REST, gRPC, or MCP (`ze_show_system` action `triage` / `ze_execute`).
- **Recovered panic:** Any goroutine guarded by `defer task.Contain(describe, end)` panics while processing input.
- **Agent discovery:** AI assistant calls `ze_reference` / `tools/list` (seeing `show system triage` with its self-describing YANG help/description) or runs `show skill ze` / `show skill ze-diagnostics`.

### Transformation Path
1. **Triage query path:** Dispatcher matches YANG path `show system triage` -> `ze-cmd:show-system-triage` -> `handleShowSystemTriage(ctx, validated)`.
2. `handleShowSystemTriage` gathers bounded, non-blocking snapshots from each existing producer: `zeversion.BuildIdentity()`, `health.Check()`, `querySystemMap(ctx, "show doctor")` (when dispatcher is present), `report.Errors(32)` + `report.Warnings()` (capped at 64) + `report.Losses()`, `crashlog.CrashListFields()` (capped at 16) + `crashlog.CrashDir()`, `pluginserver.PluginDoctorSnapshot()` (summarized counts + unhealthy/noteworthy plugin rows + disagreements + handler panics), `ctx.Server.ReloadStatus()`, `slogutil.GlobalLogRing()` (newest 32 `warn`/`error` entries combined, sorted newest-first), and compact `runtime.MemStats` + goroutine/CPU/uptime summary.
3. Each optional sub-source (`doctor`, `plugins`, `reload`) that cannot answer reports an explicit `available: false` with its `reason` or `error` rather than returning an empty healthy list.
4. The handler returns `plugin.Response{Status: plugin.StatusDone, Data: plugin.Map(...)}`, which flows through the standard pipe renderer and MCP content wrapper.
5. **Recovered panic path:** `taskguard.Task.Contain` recovers panic -> `Guard.record` increments `ze_task_panics_total`, writes recovered crash record via `crashlog.RecordRecoveredPanic`, calls `report.RaiseErrorFix` under `report.CodeInternalDefect` (fitting subject/message/fix to report bounds), and logs one `ERROR` line carrying `"code", report.CodeInternalDefect` alongside all existing fields.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| MCP / CLI / SSH / Web / API ↔ Daemon dispatcher | Standard YANG command path `show system triage` -> `ze-cmd:show-system-triage` | Yes |
| `internal/component/cmd/show` ↔ `internal/component/doctor` | Existing `querySystemMap(ctx, "show doctor")` over the caller's dispatcher (no direct Go import) | Yes |
| `internal/core/taskguard` ↔ `internal/core/report` | Direct core-to-core call `report.RaiseErrorFix` using `report.CodeInternalDefect` | Yes |
| Daemon ↔ External plugins (for doctor checks inside triage) | Existing `show doctor` -> `CallDoctorCheck` RPC / `DirectBridge` | Yes |

### Changed-Wire Census

**Search evidence:** Searched `internal/component/cmd/show/`, `internal/core/taskguard/`, `internal/component/plugin/all/testdata/wire-methods.snapshot`, and `test/` for `ze-cmd:show-system-` and `taskguard.` (positive controls: `ze-cmd:show-system-goroutines` found in `goroutines.go`, `ze-cli-show-cmd.yang`, `wire-methods.snapshot`, `test/plugin/system-goroutines-show.ci`). No existing wire protocol frame, RPC parameter, or existing command JSON schema changes: `ze-cmd:show-system-triage` is an additive read-only command, and `taskguard` adds one structured `"code"` log attribute and a standard `report.Issue` entry using existing `report.Issue` wire fields.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `ze-cmd:show-system-triage` command RPC | writer | `internal/component/cmd/show/triage.go` (`handleShowSystemTriage`) + `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` | New additive YANG command under `show system` following `show-system-goroutines` | Additive; update `internal/component/plugin/all/testdata/wire-methods.snapshot` |
| `ze-cmd:show-system-triage` command RPC | reader | `internal/component/plugin/server/command.go` (`Dispatcher.Dispatch`), `internal/component/mcp/tools.go` (`groupCommands`), CLI/SSH/web/REST/gRPC clients | Derived automatically from YANG + `pluginserver.RegisterRPCs` | Preserve: zero adapter changes needed |
| `report.Issue` JSON (`show errors`, `show doctor`) | writer | `internal/core/taskguard/guard.go` (`Guard.record`) -> `internal/core/report/report.go` (`RaiseErrorFix`) | Existing `report.Issue` struct unchanged; uses existing `report.CodeInternalDefect` | Preserve existing `report.Issue` schema; additive error events on recovered panic |
| `report.Issue` JSON (`show errors`, `show doctor`) | reader | `internal/component/cmd/show/show.go` (`handleShowErrors`), `internal/component/doctor/checks_recent_events.go` (`checkRecentEvents`) | Existing readers render `CodeInternalDefect` as `doctor-internal-defect` with `fix` | Preserve unchanged |

### Integration Points
- `internal/core/version/version.go` - add exported `Identity` struct and `BuildIdentity()` reusing `readInfo()` and `readImageManifest()`.
- `internal/core/taskguard/guard.go` - import `internal/core/report`, include `"code", report.CodeInternalDefect` in `attrs`, and call `report.RaiseErrorFix` with bounded subject/message/fix/detail.
- `internal/component/cmd/show/triage.go` - register `ze-cmd:show-system-triage` in `init()` and implement `handleShowSystemTriage`.
- `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` - add `container triage` under `container show > container system`.
- `internal/component/plugin/all/testdata/wire-methods.snapshot` - add `ze-cmd:show-system-triage`.
- `internal/plugins/skills/data/ze-diagnostics.md`, `ze.md`, `ze-full.md` - add live runtime bug-triage workflow.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | `show system triage` dispatches through the shared YANG/RPC dispatcher and queries `show doctor` via `querySystemMap`; MCP derives the tool from the live command registry with zero MCP edits. |
| No unintended coupling (components stay isolated) | Yes | `internal/core/taskguard` imports only `internal/core/report` (no upward import); `internal/component/cmd/show` does not import `internal/component/doctor` or any protocol package. |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `version.readInfo`, `health.Check`, `show doctor`, `report`, `crashlog.CrashListFields`, `PluginDoctorSnapshot`, `ReloadStatus`, and `slogutil.GlobalLogRing`. |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Neither triage nor panic recovery sits on the BGP wire hot path; `taskguard` remains zero-cost when no panic occurs. |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | `ze-cmd:show-system-triage` registers via `pluginserver.RegisterRPCs` and `ze-cli-show-cmd.yang`; MCP discovers it at `tools/list` time. |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `internal/component/cmd/show`, `internal/component/mcp`, `internal/component/command`, and `internal/component/plugin/all`; only the derived snapshot `wire-methods.snapshot` records the new wire method, while completion, help, `ze_reference`, and MCP tools derive from YANG + `Dispatcher.Commands()`. |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `internal/core/taskguard` importing `internal/core/report` introduces no import cycle and passes `./le arch tier check`. | `internal/core/report/report.go` imports only `internal/core/env` and `internal/core/slogutil`; `internal/core/taskguard/guard.go` imports `crashlog` and `metrics`. | Would need a seam or callback instead of direct import. | `go test ./internal/core/taskguard/...` and `./le arch tier check` | confirmed |
| A-2 | Calling `querySystemMap(ctx, "show doctor")` from `handleShowSystemTriage` preserves authorization and does not deadlock when invoked via CLI, SSH, or MCP (where `ctx.Process` is nil or a plugin calling triage). | `internal/component/cmd/show/system.go:152` already uses `querySystemMap` for `show system date` -> NTP; `doctorCheckSourceOf` skips `callerName` when a plugin calls `show doctor`. | A plugin calling `show system triage` could deadlock if `show doctor` called back into it; `ctx.Process` is already forwarded on `ctx`. | Unit test with fake dispatcher + functional `.ci` test driving `show system triage` from a plugin fixture | confirmed |
| A-3 | `report.RaiseErrorFix` silently drops raises whose `source`, `subject`, `message`, or `fix` exceed `report.SourceMax`/`SubjectMax`/`MessageMax`/`FixMax`. | `internal/core/report/report.go:300-339` (`validFields`, `FitMessage`). | A long panic value or unit name would cause the error event to be dropped silently from the report bus. | Bound `source`, `subject`, `message`, and `fix` explicitly in `taskguard.Guard.record` and add a boundary unit test with oversized panic/unit strings | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `show system triage` payload grows huge on a host with hundreds of warnings, plugins, or log entries, overflowing MCP context windows. | Triage JSON exceeds a few tens of kilobytes under load. | Cap every list in triage (`maxTriageErrors=32`, `maxTriageWarnings=64`, `maxTriageCrashes=16`, `maxTriageLogs=32`), emit summary counts plus only unhealthy/noteworthy plugin details, and report `total-count` + `truncated` so the AI knows when to drill down. |
| R-2 | `show doctor` fails or times out when called inside `show system triage`, causing the whole triage call to fail and hide health/errors/crashes/logs. | Triage returns `StatusError` when a single doctor check or dispatcher is unavailable. | Isolate each triage section: if `show doctor` fails or `ctx.Dispatcher()` is nil, set `doctor: {"available": false, "error": ...}` and still return every other section. |
| R-3 | `taskguard` panic recovery runs on a panic with a huge value or empty `Unit` / `Component`, causing `report.validFields` to reject the raise silently. | `report.Errors(0)` stays empty in a panic test with empty `Unit` or 2 KB panic text. | Default empty `source`/`subject` to `"unknown"`, truncate `source`/`subject`/`fix` at UTF-8 rune boundaries to `report.SourceMax`/`SubjectMax`/`FixMax`, and pass `message` through `report.FitMessage`. |
| R-4 | A caller invoking `show system triage` on a nil `ctx` or without a running plugin server gets misleading zero/empty sections that look healthy. | Empty `plugins: []` or `reload: {}` returned offline. | Report explicit `available: false` and `reason` on `doctor`, `plugins`, and `reload` whenever their runtime source is absent (`ai/rules/principles.md`). |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | At worst, `show system triage` returns an incomplete or malformed JSON section, or `taskguard` raises a noisy/truncated error event on a recovered panic. No routing, forwarding, config commit, or session hot path is altered. |
| How is it reverted? | Single commit revert; additive command, additive report raise, and Markdown skill updates. |
| Who else touches this path? | `internal/component/cmd/show`, `internal/core/taskguard`, `internal/core/version`, `internal/plugins/skills`, and `internal/component/plugin/all/testdata/wire-methods.snapshot`. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Daemon command `show system triage` (CLI / plugin / MCP dispatcher) | → | `ze-cmd:show-system-triage` -> `handleShowSystemTriage` -> structured JSON payload with version, health, doctor, errors, warnings, crashes, plugins, reload, recent-logs, runtime | `test/plugin/system-triage-show.ci` and `TestHandleShowSystemTriagePopulated` |
| Guarded task panic via `taskguard.Task.Contain` | → | `Guard.record` -> `report.RaiseErrorFix` (`doctor-internal-defect`) -> visible in `show errors`, `show doctor`, and `show system triage` | `TestGuardRaisesReportErrorAndLogsDoctorCode` |
| `show skill ze-diagnostics` / `show skill ze` | → | `skills.showSkill` -> embedded live-triage playbook referencing `show system triage` | `TestSkillsDiagnosticsIncludesLiveTriagePlaybook` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `show system triage` runs against a live daemon (via CLI, plugin dispatch, or MCP `ze_show_system` / `ze_execute`) | Returns `StatusDone` with structured lowercase kebab-case `plugin.Map` containing `version`, `health`, `doctor`, `errors`, `warnings`, `crashes`, `plugins`, `reload`, `recent-logs`, and `runtime`, and renders cleanly through `\| json`, `\| yaml`, and `\| table`. | Unpopulated lists (such as zero crashes or zero errors on a healthy daemon) return empty JSON arrays `[]` with `total-count: 0`, because absence of faults is a real healthy observation when the source is available. |
| AC-2 | `show system triage` runs when `ctx` has no dispatcher, no running server, or `show doctor` returns an error | Still returns `StatusDone` with `version`, `health`, `errors`, `warnings`, `crashes`, `recent-logs`, and `runtime` populated, and marks unavailable sections (`doctor`, `plugins`, `reload`) with `available: false` and a non-empty `reason` or `error` field. | Sections that depend on an absent daemon handle remain unavailable with an explicit reason; no other section may silently substitute empty data as healthy. |
| AC-3 | `show system triage` runs when the report bus, crash directory, or log ring holds more entries than the triage caps (`32` errors, `64` warnings, `16` crashes, `32` warn/error logs) | Returns at most the capped count newest-first in each section, sets `total-count` to the full pre-truncation count, and sets `truncated: true` when entries were omitted. | Older entries beyond the triage cap are omitted from the triage overview because the caller can fetch full sets via `show errors`, `show warnings`, `show crashes`, or `show log recent`. |
| AC-4 | `show system triage` reads `version.BuildIdentity()` | Emits structured `release`, `build-date`, `go`, `os`, `arch`, plus `commit`, `modified`, `vcs-time`, and `image` when present from Go build info or `/perm/ze/build.json`. | `commit`, `vcs-time`, and `image` are omitted when the running binary has no VCS stamp or appliance manifest, matching `version.Extended()`. |
| AC-5 | A panic is recovered by `taskguard.Task.Contain` | `Guard.record` logs `"code": "doctor-internal-defect"` on the `ERROR` line and raises an error event on `internal/core/report` with source = component, code = `doctor-internal-defect`, subject = bounded task/unit origin, message = bounded panic summary, fix = `ze show crashes name <file>` (or `ze show log recent level error` when no crash file was written), and structured detail (`task`, `unit`, `panic`, and `message-type`/`crash-report` when present); the event appears in `show errors`, `show doctor` (`recent-events`), and `show system triage`. | Oversized panic values or unit names are truncated at UTF-8 rune boundaries to fit `report` field limits rather than causing `report.RaiseErrorFix` to drop the event; no panic still produces zero raises and zero logs. |
| AC-6 | An agent runs `show skill ze`, `show skill ze full`, or `show skill ze-diagnostics` | The returned Markdown guides include the live runtime bug-triage workflow starting from `show system triage` while preserving all existing config-validation commands, diagnostic shape fields, and `ze create agent skill` setup instructions. | Unrelated skills (`ze-config`, `ze-commands`, `ze-agent`) stay unchanged because they govern config syntax, command reference, and edit loops rather than diagnostics. |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Connects an AI assistant over MCP or CLI to a misbehaving Ze daemon and asks what went wrong | `tools/list` / `ze_reference` exposes `show system triage` (`ze_show_system`, action `triage`) -> `Dispatcher` -> `handleShowSystemTriage` -> single structured JSON snapshot | `test/plugin/system-triage-show.ci`, `TestHandleShowSystemTriagePopulated` |
| 2 | A bug triggers a recovered panic inside a BGP/OSPF/L2TP task boundary | `taskguard.Task.Contain` -> `Guard.record` -> `crashlog.RecordRecoveredPanic` + `report.RaiseErrorFix(doctor-internal-defect)` + `ERROR` log with `code` -> immediately visible in `show errors`, `show doctor`, and `show system triage` | `TestGuardRaisesReportErrorAndLogsDoctorCode`, `TestHandleShowSystemTriageSurfacesTaskguardPanic` |
| 3 | Loads bundled skill `ze-diagnostics` to follow a bug-investigation playbook | `show skill ze-diagnostics` -> embedded `data/ze-diagnostics.md` -> ordered triage commands from `show system triage` through logs, crashes, goroutines, and profiles | `TestSkillsDiagnosticsIncludesLiveTriagePlaybook` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildIdentityFields` | `internal/core/version/version_test.go` | AC-4: `BuildIdentity()` returns structured release, build-date, go, os, arch, and optional image manifest | PASS |
| `TestGuardRaisesReportErrorAndLogsDoctorCode` | `internal/core/taskguard/guard_test.go` | AC-5: recovered panic logs `code=doctor-internal-defect` and raises a `report.Issue` with fix `ze show crashes name <file>` | PASS |
| `TestGuardReportRaiseFallbackFixWithoutCrashDir` | `internal/core/taskguard/guard_test.go` | AC-5: when no crash directory exists, raised `report.Issue` carries fallback fix `ze show log recent level err` instead of a nonexistent crash file | PASS |
| `TestGuardReportRaiseBoundsOversizedFields` | `internal/core/taskguard/guard_test.go` | AC-5 / R-3: oversized unit/panic values are UTF-8 truncated to `report` limits so the raise is never silently dropped | PASS |
| `TestHandleShowSystemTriagePopulated` | `internal/component/cmd/show/triage_test.go` | AC-1, AC-4: triage aggregates version, health, doctor, errors, warnings, crashes, plugins, reload, recent-logs (warn/error only), and runtime | PASS |
| `TestHandleShowSystemTriageUnavailableSources` | `internal/component/cmd/show/triage_test.go` | AC-2 / R-4: nil context/server and failing doctor dispatch mark only those sections `available: false` with reasons while returning the remaining sections | PASS |
| `TestHandleShowSystemTriageCapsAndTruncation` | `internal/component/cmd/show/triage_test.go` | AC-3 / R-1: enforces caps (32 errors, 64 warnings, 16 crashes, 32 logs) with accurate `total-count` and `truncated` flags | PASS |
| `TestHandleShowSystemTriageSurfacesTaskguardPanic` | `internal/component/cmd/show/triage_test.go` | AC-1, AC-5: end-to-end in-process proof that a `taskguard` panic appears in triage `errors`, `crashes`, and `recent-logs` | PASS |
| `TestSkillsDiagnosticsIncludesLiveTriagePlaybook` | `internal/plugins/skills/skills_test.go` | AC-6: `show skill ze-diagnostics`, `show skill ze`, and `show skill ze full` include live triage instructions while preserving existing config validation and setup guidance | PASS |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `report` field truncation in `taskguard.Guard.record` (`SubjectMax`=256, `MessageMax`=1024, `FixMax`=256) | 1..max bytes | 256 / 1024 / 256 bytes | empty (`""` -> `"unknown"`) | >256 / >1024 bytes (UTF-8 rune-safe truncation; raise still accepted) |
| Triage section caps (`errors`=32, `warnings`=64, `crashes`=16, `recent-logs`=32) | 0..cap returned | cap entries (`truncated: false`) | 0 entries (`[]`, `total-count: 0`, `truncated: false`) | cap+1 entries (cap returned, `total-count: cap+1`, `truncated: true`) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `system-triage-show` | `test/plugin/system-triage-show.ci` | Running daemon answers `show system triage` with structured JSON containing version, health, doctor, errors, warnings, crashes, plugins, reload, recent-logs, and runtime | PASS |

### Interop Tests (Scope: protocol)
N-A — this spec adds no wire protocol behavior and changes no peer-facing protocol frames.

## Files to Modify
- `internal/core/version/version.go` - add `Identity` struct and `BuildIdentity()` helper sharing `readInfo()` and `readImageManifest()`
- `internal/core/version/version_test.go` - unit tests for `BuildIdentity()`
- `internal/core/taskguard/guard.go` - add `"code", report.CodeInternalDefect` log field and `report.RaiseErrorFix` call with UTF-8 field bounding
- `internal/core/taskguard/guard_test.go` - unit tests for report raise, doctor code log attribute, fallback fix, and oversized field bounds
- `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` - add `container triage` with `ze:command "ze-cmd:show-system-triage"` under `show system`
- `internal/component/plugin/all/testdata/wire-methods.snapshot` - add `ze-cmd:show-system-triage`
- `internal/plugins/skills/data/ze-diagnostics.md` - add live runtime bug-triage workflow alongside config validation
- `internal/plugins/skills/data/ze.md` - add `show system triage` to agent entry points
- `internal/plugins/skills/data/ze-full.md` - add live triage loop alongside config diagnostic loop
- `internal/plugins/skills/skills_test.go` - verify updated bundled skills contain live triage workflow and preserve existing setup/config content
- `docs/architecture/diagnostics/production-diagnostics.md` - document `show system triage` with source anchors
- `docs/architecture/diagnostics/crash-capture.md` - document `taskguard` raising `doctor-internal-defect` on the report bus and carrying `code` in its ERROR log line
- `docs/guide/command-reference.md` - add `show system triage` entry
- `ai/INDEX.md` - add discovery row for `show system triage`

## Files to Create
- `internal/component/cmd/show/triage.go` - `ze-cmd:show-system-triage` registration and `handleShowSystemTriage`
- `internal/component/cmd/show/triage_test.go` - unit tests for `handleShowSystemTriage`
- `internal/test/fixture/plugin_system_triage_show.go` - functional test fixture driving `show system triage` against a live daemon
- `test/plugin/system-triage-show.ci` - functional test exercising `show system triage` end-to-end

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` (`container show > container system > container triage`, `ze:command "ze-cmd:show-system-triage"`) |
| YANG validation constraints | N-A | `show system triage` takes no arguments or config leaves |
| YANG custom validators | N-A | No input leaves to validate |
| CLI commands/flags | Yes | Online daemon command `show system triage` in `internal/component/cmd/show/triage.go`; no offline flags |
| CLI grammar (keyword before value) | Yes | Verified against R1-R9: `show system triage` is verb + subject + noun with zero positional values; verified by `./le cli grammar` |
| Editor autocomplete | Yes | Automatic from YANG `container triage` under `show system` |
| Functional test for new RPC/API | Yes | `test/plugin/system-triage-show.ci` |
| Pipe completeness | Yes | `handleShowSystemTriage` returns structured `plugin.Map` routed through `ApplyPipes` (`\| json`, `\| yaml`, `\| table`) |
| Env var registration | N-A | No new environment variable; reuses existing `ze.report.*` and `ze.crash.*` env keys |
| Doctor check for runtime dependencies | N-A | Adds no new host/runtime dependency; reuses existing `show doctor` and `recent-events` check |
| Doctor code per problem class | Yes | `taskguard.Guard.record` logs and raises existing registered `doctor-internal-defect` (`report.CodeInternalDefect` / `diagnostic.CodeDoctorInternalDefect`), already declared by `recent-events` via `plugin/server/register.go` |
| Prometheus counters/metrics | N-A | Existing `ze_task_panics_total{component,task}` already counts recovered panics; triage is a read-only aggregation |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not a BGP family change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Additive diagnostic command and skill expansion; does not change a feature maturity row in `features/` |
| 2 | Config syntax changed? | No | No config YANG module changed |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (mention `show system triage` under System/Reports or diagnostic commands) |
| 5 | Plugin added/changed? | No | No plugin added or removed; only embedded Markdown in `internal/plugins/skills/data/` |
| 6 | Has a user guide page? | No | Covered by `docs/guide/command-reference.md` and `docs/features/ai-first.md` |
| 7 | Wire format changed? | No | No protocol wire format changed |
| 8 | Plugin SDK/protocol changed? | No | No `pkg/plugin/sdk` or `pkg/plugin/rpc` change (deferred to Part 3) |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Not protocol/RFC work |
| 10 | Test infrastructure changed? | No | Reuses existing `test/plugin/` runner and fixture registration |
| 11 | Affects daemon comparison? | No | Internal diagnostic surface |
| 12 | Internal architecture changed? | Yes | `docs/architecture/diagnostics/production-diagnostics.md` and `docs/architecture/diagnostics/crash-capture.md` |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | No | Existing `ze_task_panics_total` unchanged |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `internal/component/plugin/all/testdata/wire-methods.snapshot` and `docs/guide/command-reference.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED by `./le spec citation anchors spec plan/post-release/features/spec-ai-diagnostics-1-triage-and-fault-visibility.md`. **Updated:** `docs/architecture/diagnostics/crash-capture.md`, `docs/architecture/diagnostics/production-diagnostics.md`, `docs/architecture/api/commands.md`. **Declared / advisory, unaffected:** `docs/architecture/config/syntax.md` (`version.go` release comparison unchanged), `docs/architecture/web-interface.md` (`version.go` `HTTPHeaderHidden` unchanged), `docs/guide/configuration.md`, `docs/guide/web-interface.md`, `docs/guide/ze-install.md` (`version.go` existing functions unchanged), `docs/guide/monitoring.md`, `docs/plugin-development/metrics.md` (`ze_task_panics_total` metric unchanged). |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify `docs/features/ai-first.md` and `docs/guide/command-reference.md` examples stay accurate |

## Design presentation

### What changes

An AI assistant or operator investigating a misbehaving Ze daemon can now run one command, `show system triage` (available over CLI, SSH, web, REST, gRPC, and MCP as `ze_show_system` with action `triage` — discovered automatically via `tools/list` and `ze_reference` with zero changes to `internal/component/mcp`), and receive a single bounded JSON document containing build provenance, component health, live daemon doctor findings, active warnings, recent error events, crash records, unhealthy plugin state, reload status, recent warning/error log entries, and process runtime vitals. In addition, every network-input panic recovered by `taskguard` now carries `code=doctor-internal-defect` on its error log line and raises an error event on the report bus, so recovered internal bugs appear immediately in `show errors`, `show doctor`, and `show system triage`. Finally, the bundled `ze-diagnostics` and `ze` skills now include the live bug-triage workflow starting from `show system triage`.

### Before and after

1. **Triaging a misbehaving daemon over MCP or CLI:**
   - Before: An AI assistant must guess and issue 7–8 separate commands (`show version`, `show health`, `show doctor`, `show errors`, `show warnings`, `show crashes`, `show reload-status`, `show log recent`) to assemble an initial picture of why Ze is misbehaving.
   - After: One call to `show system triage` (MCP tool `ze_show_system`, action `triage`) returns a single bounded JSON object with `version`, `health`, `doctor`, `errors`, `warnings`, `crashes`, `plugins`, `reload`, `recent-logs`, and `runtime`, with explicit `available: false` reasons if any runtime source is unreachable.

2. **A software bug panics inside a guarded protocol task (such as a BGP session read loop):**
   - Before: `taskguard.Task.Contain` writes `crash-*-recovered.log`, increments `ze_task_panics_total`, and logs an `ERROR` line without a `code` field. `show errors` returns `count: 0`, and `show doctor` reports `ready: true` with no mention of the recovered panic.
   - After: `taskguard.Task.Contain` logs `"code": "doctor-internal-defect"` and calls `report.RaiseErrorFix` with `fix: "ze show crashes name <file>"`. `show errors`, `show doctor` (`recent-events`), and `show system triage` all immediately surface the bug, its unit, its panic value, and the exact crash record command.

3. **Reading the bundled diagnostics skill (`ze show skill ze-diagnostics`):**
   - Before: `ze-diagnostics` covers only offline/online config validation (`show config validation`, `show diagnostic`, `ze config fix`).
   - After: `ze-diagnostics` keeps the config validation section intact and adds a "Live Runtime Triage" playbook walking from `show system triage` to targeted drill-down commands (`show crashes name`, `show errors`, `show log recent`, `show event recent`, `show system goroutines`, `show system profile`) and the fields needed for a complete bug report.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Add a handcrafted MCP-only `ze_triage` tool or hardcode `"show system triage"` in `internal/component/mcp/discover.go` | Violates `ai/doors/system.md` ("One schema, one command registry, every interface") and `ai/doors/api-surfaces.md` ("Never copy a fact the YANG model or the registry already holds: a command name"); keeping MCP untouched preserves pure registry derivation. |
| Extend `request support archive` to return inline JSON instead of adding `show system triage` | `request support archive` is a heavy local-data command that forks/reads host files (`dmesg`, `sysctl`, `conntrack`, `firewall`, `config` tree) and writes a `.tar.gz` archive; triage must be fast, bounded, read-only, and JSON-native for instant AI inspection. Registrable support modules belong in Part 5. |
| Create a brand-new doctor code `doctor-task-panic-recovered` for `taskguard` | `doctor-internal-defect` (`report.CodeInternalDefect` / `diagnostic.CodeDoctorInternalDefect`) already exists, is already registered as an event code for `recent-events`, is already accessible to core leaf packages without an import cycle, and its description matches a recovered Ze panic. |

### Documents departed from

none

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Release bucket for this 5-spec AI diagnostics series | (a) Keep in `plan/post-release/features/` so it does not add new first-release blockers while still being implemented now (Recommended); (b) Move to `plan/release/features/` as a first-release commitment | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register `show system triage` entry point and failing tests
   - Tests: `TestHandleShowSystemTriagePopulated`, `TestGuardRaisesReportErrorAndLogsDoctorCode`, `TestSkillsDiagnosticsIncludesLiveTriagePlaybook`, `test/plugin/system-triage-show.ci`
   - Files: `internal/component/cmd/show/yang/ze-cli-show-cmd.yang`, `internal/component/cmd/show/triage.go` (stub), `internal/component/plugin/all/testdata/wire-methods.snapshot`, test files
   - Verify: YANG command routes to `ze-cmd:show-system-triage` and all new tests fail because the feature behavior is not yet implemented.
2. **Phase: Core provenance and panic reporting** -- `version.BuildIdentity()` and `taskguard` report-bus integration
   - Tests: `TestBuildIdentityFields`, `TestGuardRaisesReportErrorAndLogsDoctorCode`, `TestGuardReportRaiseFallbackFixWithoutCrashDir`, `TestGuardReportRaiseBoundsOversizedFields`
   - Files: `internal/core/version/version.go`, `internal/core/version/version_test.go`, `internal/core/taskguard/guard.go`, `internal/core/taskguard/guard_test.go`, `docs/architecture/diagnostics/crash-capture.md`
   - Verify: unit tests pass under `-race`, `./le arch tier check` passes, and `./le arch doctor-coverage check` passes on changed files.
3. **Phase: `show system triage` handler** -- bounded aggregation across all 10 runtime sources
   - Tests: `TestHandleShowSystemTriagePopulated`, `TestHandleShowSystemTriageUnavailableSources`, `TestHandleShowSystemTriageCapsAndTruncation`, `TestHandleShowSystemTriageSurfacesTaskguardPanic`
   - Files: `internal/component/cmd/show/triage.go`, `internal/component/cmd/show/triage_test.go`, `docs/architecture/diagnostics/production-diagnostics.md`, `docs/architecture/api/commands.md`, `docs/guide/command-reference.md`, `ai/INDEX.md`
   - Verify: unit tests pass under `-race`; `./le cli grammar`, `./le cli ownership`, `./le doc yang-contract command-contract`, and `./le doc check all` pass.
4. **Phase: Agent skills and end-to-end functional proof** -- live-triage skill and `.ci` test
   - Tests: `TestSkillsDiagnosticsIncludesLiveTriagePlaybook`, `test/plugin/system-triage-show.ci`
   - Files: `internal/plugins/skills/data/ze-diagnostics.md`, `ze.md`, `ze-full.md`, `internal/plugins/skills/skills_test.go`, `internal/test/fixture/plugin_system_triage_show.go`, `test/plugin/system-triage-show.ci`
   - Verify: scoped unit tests and `./le test functional plugin` (matching `system-triage-show`) pass.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-6 has an implementation at file:line and a passing test |
| Feature completeness | All three end-to-end user stories work through their real entry points |
| Correctness | Unavailable runtime sources report `available: false` with a reason rather than empty healthy values; oversized `taskguard` panic strings are UTF-8 truncated so `report.RaiseErrorFix` never drops them |
| Naming | Every JSON key in `show system triage` and `version.Identity` is lowercase kebab-case |
| Data flow | `internal/core/taskguard` does not import `internal/core/diagnostic`; `internal/component/cmd/show` reaches `show doctor` only via `querySystemMap`; `internal/component/mcp` is untouched |
| Operator errors and doctor codes | `taskguard.Guard.record` logs `"code", report.CodeInternalDefect` and raises `report.CodeInternalDefect` with a concrete `fix` command on the last line (`ai/rules/user-facing-errors.md`) |
| Rule: cli | `show system triage` returns structured `plugin.Map`, works with `\| json`, `\| yaml`, `\| table`, and passes `./le cli grammar` |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `version.BuildIdentity()` returns structured provenance | `./le test unit pkg path ./internal/core/version/` |
| `taskguard` raises `doctor-internal-defect` on `report` and logs `code` | `./le test unit pkg path ./internal/core/taskguard/` |
| `show system triage` aggregates all 10 bounded sections | `./le test unit pkg path ./internal/component/cmd/show/` |
| Bundled skills carry live triage workflow | `./le test unit pkg path ./internal/plugins/skills/` |
| End-to-end daemon functional test | `./le test functional plugin` (`system-triage-show`) |
| Architecture, CLI grammar, YANG contract, and docs gates | `./le arch tier check`, `./le cli grammar`, `./le doc yang-contract command-contract`, `./le doc check all` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Secret exposure | `show system triage` must never include raw config secrets, credentials, tokens, or TLS keys; recent logs inherit `slogutil`'s redaction, and `version.BuildIdentity()` exposes only build/image metadata |
| Authorization | `show system triage` goes through the shared dispatcher (`RoleRead` under `show`), and its nested `querySystemMap(ctx, "show doctor")` carries the caller's original `CommandContext` and identity |
| Resource exhaustion | Every list in `show system triage` is strictly capped (`32` errors, `64` warnings, `16` crashes, `32` logs) and reads existing in-memory bounded rings without unbounded allocations |

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
| Register `show system triage` in `internal/component/cmd/show` via YANG and leave `internal/component/mcp` untouched | MCP-only `ze_triage` tool; hardcoding `"show system triage"` in `mcp/discover.go`; extending `request support archive` | Follows "CLI first, MCP for free" (`production-diagnostics.md`) and `ai/doors/api-surfaces.md` ("Never copy a fact the YANG model or the registry already holds: a command name"); every operator surface gets one bounded JSON triage view with zero adapter coupling. |
| Reuse `report.CodeInternalDefect` in `taskguard.Guard.record` | Adding a new `doctor-task-panic-recovered` code | `report.CodeInternalDefect` is already declared in `internal/core/report` specifically to avoid a core->diagnostic import cycle, is already described in `codes_plugin_infra.go`, and is already registered as an event code for `recent-events`. |
| Call `show doctor` from `show system triage` via `querySystemMap(ctx, "show doctor")` | Importing `internal/component/doctor` directly from `internal/component/cmd/show` | Preserves component isolation, runs the full daemon + plugin doctor path, and retains caller authorization and plugin deadlock avoidance. |

## Known Limitations
- Pre-incident execution tracing (`runtime/trace.FlightRecorder`), readable profile summaries (`mutex`/`block`), severity-split log rings, and richer `EventRecord` payloads live in `plan/post-release/features/spec-ai-diagnostics-2-flight-recorder-and-profiles.md`.
- Inspecting goroutines, profiles, and logs inside forked external plugin processes lives in `plan/post-release/features/spec-ai-diagnostics-3-external-plugin-introspection.md`.
- Cross-boundary operation correlation IDs, reload transaction history, and lock-free diagnostic fast-paths live in `plan/post-release/features/spec-ai-diagnostics-4-correlation-and-wedge-survival.md`.
- Dataplane drift checks, routing/policy `explain` commands, always-on bad-packet rings, and registrable support modules live in `plan/post-release/features/spec-ai-diagnostics-5-drift-explain-and-support-registry.md`.

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
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread
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
- [ ] Interop tests for protocol features (N-A: not protocol work)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/features/spec-ai-diagnostics-1-triage-and-fault-visibility.md` only, in the same `./le commit create` script


---

## Implementation Summary

### What Was Implemented
- Added `Identity` value struct and `BuildIdentity() Identity` in `internal/core/version/version.go` sharing `readInfo()` and `readImageManifest()`, with unit coverage in `TestBuildIdentityFields` (`internal/core/version/version_test.go`).
- Updated `taskguard.Guard.record` (`internal/core/taskguard/guard.go`) to emit `"code", report.CodeInternalDefect` on its `ERROR` log line and raise a bounded error event via `report.RaiseErrorFix` with `boundReportField` UTF-8 truncation, structured detail (`task`, `unit`, `panic`, optional `message-type` and `crash-report`), and `fix` set to `ze show crashes name <file>` or fallback `ze show log recent level err`. Added `TestGuardRaisesReportErrorAndLogsDoctorCode`, `TestGuardReportRaiseFallbackFixWithoutCrashDir`, `TestGuardReportRaiseBoundsOversizedFields`, and extended `TestGuardNoPanicIsSilent` (`internal/core/taskguard/guard_test.go`).
- Added `show system triage` (`ze-cmd:show-system-triage`): declared `container triage` in `internal/component/cmd/show/yang/ze-cli-show-cmd.yang`, registered the RPC in `internal/component/cmd/show/show.go`, updated `internal/component/plugin/all/testdata/wire-methods.snapshot`, and implemented `handleShowSystemTriage` in `internal/component/cmd/show/triage.go` aggregating 10 bounded lowercase kebab-case sections (`version`, `health`, `doctor`, `errors`, `warnings`, `crashes`, `plugins`, `reload`, `recent-logs`, `runtime`) with explicit `available: false` degradation. Added unit tests in `internal/component/cmd/show/triage_test.go` and end-to-end daemon functional test `test/plugin/system-triage-show.ci` + `internal/test/fixture/{register_system_triage_show.go,plugin_system_triage_show.go}`.
- Expanded bundled agent skills (`internal/plugins/skills/data/ze-diagnostics.md`, `ze.md`, `ze-full.md`) with the live runtime triage playbook starting from `show system triage`, verified by `TestSkillsDiagnosticsIncludesLiveTriagePlaybook` (`internal/plugins/skills/skills_test.go`).
- Wrote operator news fragment `website/changes/fragments/ai-diagnostics-1-triage-and-fault-visibility.md`.

### Bugs Found/Fixed
- Caught invalid log-level spelling (`level error` vs YANG enum `enum err` in `internal/plugins/log/yang/ze-log-cmd.yang:41`) before shipping; `fallbackFixCommand` in `internal/core/taskguard/guard.go:48`, `docs/architecture/diagnostics/crash-capture.md:173`, and the bundled skills all use `level err`. Covered by `TestGuardReportRaiseFallbackFixWithoutCrashDir`.
- Caught 5 `goconst` findings and unexposed `rec.Attributes()` decode error in `internal/component/cmd/show/triage.go` during Phase 4b lint/review; extracted key constants and surfaced decode errors as `attributes-error`. Verified by `./le go lint run scope ./internal/component/cmd/show/...` (0 issues).
- Fixed minor doc count drift in `docs/architecture/diagnostics/production-diagnostics.md:16` ("all seven diagnostic areas" -> "all eight diagnostic areas") after adding the eighth row (`Live triage`).

### Documentation Updates
- `docs/architecture/diagnostics/production-diagnostics.md`: added `Live triage` row and `### Single-call live triage (show system triage)` section with anchors `<!-- source: internal/component/cmd/show/triage.go -- handleShowSystemTriage -->` and `<!-- source: internal/core/version/version.go -- BuildIdentity -->`, and updated the area count from seven to eight.
- `docs/architecture/diagnostics/crash-capture.md`: updated `## Recovered panics` from three outputs to four, added `code` (`doctor-internal-defect`) to the ERROR log line row, added the report-bus error event row with `<!-- source: internal/core/taskguard/guard.go -- Guard.record -->`, and documented the `ze show log recent level err` fallback.
- `docs/architecture/api/commands.md`: added `show system triage` to the `System` category with `<!-- source: internal/component/cmd/show/triage.go -- handleShowSystemTriage -->`.
- `docs/guide/command-reference.md`: added `ze show system triage` to the `show system` command block and top-level keys table with `<!-- source: internal/component/cmd/show/triage.go -- handleShowSystemTriage -->`.
- `ai/INDEX.md`: added keyword row for `show system triage, live triage, triage snapshot, runtime bug triage, single-call diagnostics`.
- `./le doc check all`: 0 failures in any Spec 1 file (all Spec 1 source anchors, YANG command contract, usage contract, and command help shape passed; 3 unrelated pre-existing/foreign-session failures noted in `catalog.go` row 374 `show fleet alarm`, `ze-policy-conf` summary char-cap, and `commands.md:1241`).

### Deviations from Plan
- **YANG-valid log level in fallback fix (`AC-5`):** Spec AC-5 prose wrote `ze show log recent level error`, whereas `internal/plugins/log/yang/ze-log-cmd.yang:41` declares `enum err` (not `error`). Implemented `fallbackFixCommand = "ze show log recent level err"` in `internal/core/taskguard/guard.go:48`, matching `docs/architecture/diagnostics/crash-capture.md` and bundled skills so operators receive an accepted command.
- **`init()` registration file placement:** Registered `ze-cmd:show-system-triage` inside existing `init()` in `internal/component/cmd/show/show.go:103-106` (rather than adding a second `init()` in `triage.go`) and placed fixture `init()` in `internal/test/fixture/register_system_triage_show.go` (rather than `plugin_system_triage_show.go`) to satisfy `./le go style check`, which rejects new `init()` outside `register*.go`.
- **`docs/architecture/api/commands.md` edited:** Listed in Documentation Update Checklist row 4 (`Yes`) and updated alongside the Files to Modify docs.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Spec AC-5 prose wrote `ze show log recent level error` for the fallback fix command when no crash file is written. | `internal/plugins/log/yang/ze-log-cmd.yang:41` defines `enum err`, so `level error` would be rejected by YANG argument validation. | Checked `ze-log-cmd.yang` during Phase 2/4 implementation. | Used `ze show log recent level err` in `taskguard.Guard.record`, tests, `crash-capture.md`, and bundled skills, and recorded the deviation above. |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| 1. `taskguard.Task.Contain` logs `code=doctor-internal-defect` and raises bounded error on `internal/core/report` | Done | `internal/core/taskguard/guard.go:179-256` (`Guard.record`, `boundReportField`) | Preserves all existing log fields, metric, crash file, and `end(p)` order |
| 2. Single read-only `show system triage` returning 10 bounded structured sections, auto-exposed to MCP with zero edits to `internal/component/mcp` | Done | `internal/component/cmd/show/triage.go:52-487` (`handleShowSystemTriage`), `internal/component/cmd/show/show.go:103-106`, `internal/component/cmd/show/yang/ze-cli-show-cmd.yang:111-119` | `internal/component/mcp` untouched |
| 3. Structured build identity helper in `internal/core/version` | Done | `internal/core/version/version.go:164-206` (`Identity`, `BuildIdentity`) | Reuses `readInfo()` and `readImageManifest()` |
| 4. Live-triage playbook in bundled `ze-diagnostics`, `ze`, and `ze-full` skills | Done | `internal/plugins/skills/data/ze-diagnostics.md:49-81`, `ze.md:13-23`, `ze-full.md:14-36` | Preserves existing config-validation and setup instructions |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `internal/component/cmd/show/triage.go:52-68` (`handleShowSystemTriage`); `TestHandleShowSystemTriagePopulated`; `test/plugin/system-triage-show.ci` | All 10 kebab-case sections, empty arrays `[]`, clean `json`/`yaml`/`table` pipe rendering |
| AC-2 | Done | `internal/component/cmd/show/triage.go:124-138,210-217,379-391` (`triageDoctor`, `triagePlugins`, `triageReload`); `TestHandleShowSystemTriageUnavailableSources` | Explicit `available: false` + `reason`/`error` while remaining sections succeed |
| AC-3 | Done | `internal/component/cmd/show/triage.go:25-30,142-206,407-452`; `TestHandleShowSystemTriageCapsAndTruncation` | Caps 32/64/16/32 newest-first with pre-truncation `total-count` and `truncated` |
| AC-4 | Done | `internal/core/version/version.go:164-206` (`BuildIdentity`); `internal/component/cmd/show/triage.go:72-97` (`triageVersion`); `TestBuildIdentityFields`; `TestHandleShowSystemTriagePopulated` | Optional `commit`, `modified`, `vcs-time`, `cgo`, `image` omitted when absent |
| AC-5 | Changed | `internal/core/taskguard/guard.go:48,179-256` (`Guard.record`, `boundReportField`); `TestGuardRaisesReportErrorAndLogsDoctorCode`, `TestGuardReportRaiseFallbackFixWithoutCrashDir`, `TestGuardReportRaiseBoundsOversizedFields`, `TestGuardNoPanicIsSilent`, `TestHandleShowSystemTriageSurfacesTaskguardPanic` | Fallback fix uses YANG-valid `level err` (recorded in Deviations) |
| AC-6 | Done | `internal/plugins/skills/data/{ze-diagnostics.md,ze.md,ze-full.md}`; `TestSkillsDiagnosticsIncludesLiveTriagePlaybook` | Live triage playbook added; existing config validation and `ze create agent skill` preserved |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestBuildIdentityFields` | Done | `internal/core/version/version_test.go:141` | Covers present + absent image manifest and JSON tags |
| `TestGuardRaisesReportErrorAndLogsDoctorCode` | Done | `internal/core/taskguard/guard_test.go:253` | Exact name |
| `TestGuardReportRaiseFallbackFixWithoutCrashDir` | Done | `internal/core/taskguard/guard_test.go:320` | Exact name; asserts `fallbackFixCommand` (`level err`) |
| `TestGuardReportRaiseBoundsOversizedFields` | Done | `internal/core/taskguard/guard_test.go:373` | Exact name; empty + multi-byte UTF-8 + boundary cases |
| `TestHandleShowSystemTriagePopulated` | Done | `internal/component/cmd/show/triage_test.go:37` | Exact name; includes kebab-case and `json`/`yaml`/`table` pipe checks |
| `TestHandleShowSystemTriageUnavailableSources` | Done | `internal/component/cmd/show/triage_test.go:194` | Exact name |
| `TestHandleShowSystemTriageCapsAndTruncation` | Done | `internal/component/cmd/show/triage_test.go:266` | Exact name; empty, exact-cap, and over-cap boundaries |
| `TestHandleShowSystemTriageSurfacesTaskguardPanic` | Done | `internal/component/cmd/show/triage_test.go:437` | Exact name |
| `TestSkillsDiagnosticsIncludesLiveTriagePlaybook` | Done | `internal/plugins/skills/skills_test.go:194` | Exact name |
| `system-triage-show` | Done | `test/plugin/system-triage-show.ci` | Promoted after 80/80 stress-repro pass |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/core/version/version.go` | Done | Modified (`Identity`, `BuildIdentity`) |
| `internal/core/version/version_test.go` | Done | Modified (`TestBuildIdentityFields`) |
| `internal/core/taskguard/guard.go` | Done | Modified (`Guard.record`, `boundReportField`) |
| `internal/core/taskguard/guard_test.go` | Done | Modified (3 new tests + `TestGuardNoPanicIsSilent` extended) |
| `internal/component/cmd/show/show.go` | Changed | Added `ze-cmd:show-system-triage` in existing `init()` (per style rule) |
| `internal/component/cmd/show/triage.go` | Done | Created (`handleShowSystemTriage`) |
| `internal/component/cmd/show/triage_test.go` | Done | Created (4 unit tests) |
| `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` | Done | Modified (`container triage`) |
| `internal/component/plugin/all/testdata/wire-methods.snapshot` | Done | Modified via `hunks` patch (`ze-cmd:show-system-triage`) |
| `internal/plugins/skills/data/ze-diagnostics.md` | Done | Modified (`## Live Runtime Triage`) |
| `internal/plugins/skills/data/ze.md` | Done | Modified (`show system triage` entry point) |
| `internal/plugins/skills/data/ze-full.md` | Done | Modified (`## Live Runtime Triage Loop`) |
| `internal/plugins/skills/skills_test.go` | Done | Modified (`TestSkillsDiagnosticsIncludesLiveTriagePlaybook`) |
| `internal/test/fixture/register_system_triage_show.go` | Changed | Created for fixture `init()` registration (per style rule) |
| `internal/test/fixture/plugin_system_triage_show.go` | Done | Created (`pluginSystemTriage`, `requireTriageListSection`) |
| `test/plugin/system-triage-show.ci` | Done | Created and promoted after 80/80 load proof |
| `docs/architecture/diagnostics/production-diagnostics.md` | Done | Modified with source anchors |
| `docs/architecture/diagnostics/crash-capture.md` | Done | Modified with source anchor |
| `docs/architecture/api/commands.md` | Done | Modified with source anchor (per Documentation Checklist row 4) |
| `docs/guide/command-reference.md` | Done | Modified with source anchor |
| `ai/INDEX.md` | Done | Modified with discovery keyword row |

### Audit Summary
- **Total items:** 41 (4 Task requirements, 6 ACs, 10 TDD tests, 21 files)
- **Done:** 38
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (AC-5 fallback `level err`, `show.go` `init()` placement, `register_system_triage_show.go` `init()` placement — all recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Single-call bounded live triage snapshot (`show system triage`) over shared YANG/RPC registry (CLI and MCP with zero MCP edits) | Functional + unit tests | `test/plugin/system-triage-show.ci` (`./le --name triage test bgp plugin --pattern system-triage-show` -> 1/1 PASS 983ms; 80/80 stress-repro PASS); `TestHandleShowSystemTriagePopulated`, `TestHandleShowSystemTriageUnavailableSources`, `TestHandleShowSystemTriageCapsAndTruncation` (`./le test unit pkg path ./internal/component/cmd/show/` -> ok 24.3s); `./le doc yang-contract command-contract` + `./le cli grammar` OK |
| Recovered `taskguard` panics log `code=doctor-internal-defect` and raise bounded report-bus errors visible in `show errors`, `show doctor`, and `show system triage` | Unit + in-process integration tests | `TestGuardRaisesReportErrorAndLogsDoctorCode`, `TestGuardReportRaiseFallbackFixWithoutCrashDir`, `TestGuardReportRaiseBoundsOversizedFields`, `TestGuardNoPanicIsSilent` (`./le test unit pkg path ./internal/core/taskguard/` -> ok 5.3s); `TestHandleShowSystemTriageSurfacesTaskguardPanic` (`./le test unit pkg path ./internal/component/cmd/show/` -> ok); `./le arch doctor-coverage check` OK |
| Structured build and appliance provenance in `internal/core/version` | Unit test | `TestBuildIdentityFields` (`./le test unit pkg path ./internal/core/version/` -> ok 3.2s) verifying `Identity` fields and JSON tags with and without image manifest |
| Bundled `ze-diagnostics`, `ze`, and `ze-full` skills teach live runtime bug triage starting from `show system triage` | Unit test | `TestSkillsDiagnosticsIncludesLiveTriagePlaybook` (`./le test unit pkg path ./internal/plugins/skills/` -> ok 43.7s) verifying live triage sections and preserved config-validation/setup guidance |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none — every in-scope deliverable of Part 1 is complete; Parts 2–5 of the series were scoped into their own sibling specs from the start | Out of scope for Part 1 | `plan/post-release/features/spec-ai-diagnostics-2-flight-recorder-and-profiles.md`, `plan/post-release/features/spec-ai-diagnostics-3-external-plugin-introspection.md`, `plan/post-release/features/spec-ai-diagnostics-4-correlation-and-wedge-survival.md`, `plan/post-release/features/spec-ai-diagnostics-5-drift-explain-and-support-registry.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/ai-diagnostics-1-triage-and-fault-visibility-f1a36472-8747-465b-8316-df184028508b.md` |
| `./le spec review check` | clean |
| Rounds | 1 |
| Reviewer lenses used | independent inline `/ze-review` + `/ze-review-spec`: automated pre-checks (`./le repo check`, `./le test weakened check`), wiring, functional coverage, docs + design-documents lens, logic/edge-cases/security/allocation, simplicity, user-facing errors/doctor codes, 7-question Ze Go Style pass (`docs/contributing/ze-go-style.md`) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | NOTE | `docs/architecture/diagnostics/production-diagnostics.md:16` still read "all seven diagnostic areas" after adding the eighth row (`Live triage`). | `docs/architecture/diagnostics/production-diagnostics.md:16` | Updated to "all eight diagnostic areas" in closure step 4. |
| 2 | NOTE | AC-5 prose wrote `level error`; `internal/core/taskguard/guard.go:48` uses YANG-valid `level err` (`ze-log-cmd.yang:41`). | `internal/core/taskguard/guard.go:48` | Documented as an intentional deviation in Deviations from Plan and Mistake Log; no code defect. |
| 3 | NOTE | `internal/test/fixture/plugin_system_triage_show.go` verifies all 10 sections and list-section structure on a live daemon without asserting `doctor.available`/`plugins.available`/`reload.available` booleans directly (`triage_test.go` covers those). | `internal/test/fixture/plugin_system_triage_show.go:16-78` | Test-only NOTE; no product defect. |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/cmd/show/triage.go` | Yes | `-rw-r--r-- 1 thomas thomas 13174 Nov 27 06:33 internal/component/cmd/show/triage.go` |
| `internal/component/cmd/show/triage_test.go` | Yes | `-rw-r--r-- 1 thomas thomas 22024 Nov 27 06:09 internal/component/cmd/show/triage_test.go` |
| `internal/test/fixture/register_system_triage_show.go` | Yes | `-rw-r--r-- 1 thomas thomas 484 Nov 27 05:54 internal/test/fixture/register_system_triage_show.go` |
| `internal/test/fixture/plugin_system_triage_show.go` | Yes | `-rw-r--r-- 1 thomas thomas 2669 Nov 27 05:55 internal/test/fixture/plugin_system_triage_show.go` |
| `test/plugin/system-triage-show.ci` | Yes | `-rw-r--r-- 1 thomas thomas 1585 Nov 27 05:53 test/plugin/system-triage-show.ci` |
| `website/changes/fragments/ai-diagnostics-1-triage-and-fault-visibility.md` | Yes | Written and verified against `internal/le/commit/newsfragment.go` (`spec: ai-diagnostics-1-triage-and-fault-visibility`, `topic: Diagnostics`, 6 fixed headings) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | `show system triage` returns `StatusDone` with 10 kebab-case sections and renders via `json`/`yaml`/`table` | `internal/component/cmd/show/triage.go:52-68` (`handleShowSystemTriage`), `internal/component/cmd/show/show.go:103-106`, `ze-cli-show-cmd.yang:111-119`; `TestHandleShowSystemTriagePopulated` + `test/plugin/system-triage-show.ci` PASS |
| AC-2 | Unavailable `doctor`/`plugins`/`reload` return `available: false` + `reason`/`error` while remaining sections succeed | `internal/component/cmd/show/triage.go:124-138,210-217,379-391`; `TestHandleShowSystemTriageUnavailableSources` PASS |
| AC-3 | List caps (32/64/16/32) newest-first with `total-count` and `truncated` | `internal/component/cmd/show/triage.go:25-30,142-206,407-452`; `TestHandleShowSystemTriageCapsAndTruncation` PASS |
| AC-4 | `version.BuildIdentity()` returns structured provenance with optional fields omitted when absent | `internal/core/version/version.go:164-206` + `internal/component/cmd/show/triage.go:72-97`; `TestBuildIdentityFields` PASS |
| AC-5 | `taskguard.Task.Contain` logs `code=doctor-internal-defect` and raises bounded `report.RaiseErrorFix` visible in triage | `internal/core/taskguard/guard.go:48,179-256`; `TestGuardRaisesReportErrorAndLogsDoctorCode`, `TestGuardReportRaiseFallbackFixWithoutCrashDir`, `TestGuardReportRaiseBoundsOversizedFields`, `TestHandleShowSystemTriageSurfacesTaskguardPanic` PASS |
| AC-6 | `show skill ze`, `ze full`, `ze-diagnostics` include live triage playbook and preserve existing content | `internal/plugins/skills/data/{ze-diagnostics.md,ze.md,ze-full.md}`; `TestSkillsDiagnosticsIncludesLiveTriagePlaybook` PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Daemon command `show system triage` -> `ze-cmd:show-system-triage` -> `handleShowSystemTriage` | `test/plugin/system-triage-show.ci` (invokes `le test fixture plugin/system-triage-show` registered in `register_system_triage_show.go` -> `pluginSystemTriage` in `plugin_system_triage_show.go`, asserting all 10 sections and list-section structure) | Yes (read in full; 1/1 PASS + 80/80 stress-repro PASS) |
| `taskguard.Task.Contain` -> `Guard.record` -> `report.RaiseErrorFix` -> `show system triage` | In-process integration `TestHandleShowSystemTriageSurfacesTaskguardPanic` (`internal/component/cmd/show/triage_test.go:437`) + `TestGuardRaisesReportErrorAndLogsDoctorCode` (`guard_test.go:253`) | Yes (read in full; PASS) |
| `show skill ze-diagnostics` / `show skill ze` / `show skill ze full` -> `skills.showSkill` | `TestSkillsDiagnosticsIncludesLiveTriagePlaybook` (`internal/plugins/skills/skills_test.go:194`) via `command.ValidateModelArgs("show skill", ...)` | Yes (read in full; PASS) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `internal/core/taskguard/guard.go:32` imports `internal/core/report`; `./le test unit pkg path ./internal/core/taskguard/` PASS (5.3s) and `./le arch tier check` OK with zero import cycle |
| A-2 | confirmed | `handleShowSystemTriage` calls `querySystemMap(ctx, "show doctor")`; `TestHandleShowSystemTriagePopulated` and `test/plugin/system-triage-show.ci` (external plugin calling `show system triage` on live daemon, 80/80 stress-repro PASS) complete without deadlock |
| A-3 | confirmed | `internal/core/report/report.go:300-339` (`validFields`) confirmed; `boundReportField` and `report.FitMessage` in `internal/core/taskguard/guard.go:227-256` prevent silent drops, proven by `TestGuardReportRaiseBoundsOversizedFields` PASS |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/architecture/diagnostics/production-diagnostics.md` (`### Single-call live triage`) | `internal/component/cmd/show/triage.go` (`handleShowSystemTriage`) & `internal/core/version/version.go` (`BuildIdentity`); anchors verified by `./le doc check all` | Yes |
| `docs/architecture/diagnostics/crash-capture.md` (`## Recovered panics`) | `internal/core/taskguard/guard.go` (`Guard.record`); anchor verified by `./le doc check all` | Yes |
| `docs/architecture/api/commands.md` (`## Command Categories`) | `internal/component/cmd/show/triage.go` (`handleShowSystemTriage`); anchor verified by `./le doc check all` | Yes |
| `docs/guide/command-reference.md` (`show system triage`) | `internal/component/cmd/show/triage.go` (`handleShowSystemTriage`); anchor verified by `./le doc check all` | Yes |
| `ai/INDEX.md` (keyword discovery row) | Points to `internal/component/cmd/show/triage.go` and `docs/architecture/diagnostics/production-diagnostics.md` | Yes |
| Unaffected anchored docs (`syntax.md`, `web-interface.md`, `configuration.md`, `web-interface.md`, `ze-install.md`, `monitoring.md`, `metrics.md`, `ai-agent.md`, `ai-first.md`) | `git grep` over `docs/` verified existing anchors reference unchanged functions (`CompareReleases`, `HTTPHeaderHidden`, `Extended`, `MetricName`, `showSkill`, `Agent Entry Points`) | Yes |
