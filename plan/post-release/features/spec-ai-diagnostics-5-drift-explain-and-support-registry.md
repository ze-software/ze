# Spec: ai-diagnostics-5-drift-explain-and-support-registry

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli, plugin, docs |
| Priority | P3 |
| Release reason | Enables an AI over MCP to detect silent intent-vs-actual dataplane divergence across both internal and forked plugins, trace firewall and routing decisions, pull offending bad wire packets as test reproducers, and collect live daemon snapshots via registrable support modules; existing shipped behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Part 5 (final part) of the AI diagnostics architecture series. Subtle networking bugs rarely panic: they silently diverge between intended in-memory state and programmed kernel/VPP state, or drop routes/packets in a filter pipeline without obvious errors. Meanwhile, `internal/component/support/modules.go` uses a hardcoded central `moduleRegistry` map that omits live daemon runtime state (`log-ring`, `report`, `event-ring`, `plugins`, `reload-history`, `health`, `watchdog`, `bad-packets`, `drift`, `goroutines`).

This spec delivers four cohesive diagnostic capabilities:

1. **Standardized read-only dataplane drift seam (`internal/core/drift` + `ze-plugin-callback:drift-check`) + `show system drift` + `show doctor`:**
   - A pure core leaf package (`internal/core/drift`) defining value-type drift `Report` / `Finding` types, a local in-process registration seam (`drift.Register`), and a daemon-level remote collector hook (`drift.RegisterRemoteCollector`) so daemon-side queries transparently fan out to **both internal and forked (`process separate`) plugins** over the standard plugin RPC boundary.
   - A generic optional callback RPC **`ze-plugin-callback:drift-check`** (`pkg/plugin/rpc`, `pkg/plugin/sdk` `OnDriftCheck`, `internal/core/ipc/yang/ze-plugin-callback.yang`) declared via Stage 1 `DriftSubsystems []string`. First-wave dataplane plugins (`firewall`, `fib-kernel`, `interface`) declare their drift subsystem and answer read-only `OnDriftCheck` queries identically in-process (`DirectBridge`) and forked (`TLS + MuxConn`).
   - Exposed uniformly through **`show system drift`** (`ze-cmd:show-system-drift`) with command-owned pipe filter **`| subsystem <name>`** (following "the pipe wins" and full-word naming), plus a `diagnostic.DoctorPhaseRuntime` `show doctor` check (`dataplane-drift`, code `doctor-dataplane-drift`) surfacing drifted or errored subsystems.
2. **Read-only decision-trace (`explain`) for firewall rule evaluation (aligned with existing BGP explain):**
   - BGP already ships two structured decision-trace commands: `show bgp rib best | reason` (`SelectBestExplain` + `bestReasonTerminal` in `internal/component/bgp/plugins/rib`) and `show policy test peer <sel> <direction> update <hex>` (`internal/component/bgp/plugins/cmd/policy`). Both are preserved unchanged and referenced from the AI diagnostics skill.
   - Firewall currently has no decision-trace evaluator. Add a pure read-only abstract-model packet evaluator (`firewall.EvaluatePacket`) over `firewall.LastApplied()` in `internal/component/firewall/explain.go`, exposed as **`show firewall trace`** (`ze-firewall:show-trace`, backend-independent across kernel and VPP) accepting firewall-vocabulary leaves (`table`, `chain`, `protocol`, `source-address`, `destination-address`, `source-port`, `destination-port`, `input-interface`, `output-interface`, `connection-state`) and returning structured per-term evaluation steps, matched actions/jumps/gotos, and final verdict/default-policy.
3. **Always-on "last bad packets" ring (`internal/core/badpacket`) + `show system bad-packets`:**
   - A tiny pure core leaf circular ring (`internal/core/badpacket`) storing the last 16 offending wire frames (each capped at 4096 bytes, 64 KiB max total memory, zero allocations on happy paths) with monotonic IDs (`pkt-<N>`), timestamp, protocol, peer, message type, action, reason, original length, truncation flag, and lowercase hex payload.
   - Wired on cold error/malformed/panic paths only: BGP RFC 7606 actions (`treat-as-withdraw`, `attribute-discard`, `session-reset`) in `session_validation.go`, BGP header/length parse errors on BOTH the standard read path (`session_read.go`) and the default coalesced read path (`session_coalesce.go`), and `taskguard` panic recovery (`taskguard.Context.RawFrame` in `internal/core/taskguard/guard.go`, populated race-free from `finishReadBuffer` ONLY when `!*kept` — never reading a transferred/poisoned buffer — before `returnReadBuffer` recycles it).
   - Exposed through **`show system bad-packets [count <N>]`** (`ze-cmd:show-system-bad-packets`) with command-owned pipe filter **`| protocol <name>`**, plus `bad-packet-count` in `show system triage` `runtime`.
4. **Registrable support bundle modules (`internal/core/supportreg` + `internal/component/support`) + live runtime modules + MCP `ze://triage` alignment:**
   - Replace the hardcoded `moduleRegistry` map in `internal/component/support/modules.go` with a pure stdlib leaf registry `internal/core/supportreg` (`supportreg.Register(name, fn) func()`), keeping all 20 existing static modules registered in `init()`.
   - Register core-tier live runtime modules (`log-ring`, `report`, `health`, `watchdog`, `bad-packets`, `drift`, `goroutines`) inside `internal/component/support/runtime_collectors.go`, and daemon-backed live modules (`event-ring`, `plugins`, `reload-history`) from `internal/component/plugin/server` (`StartWithContext`/`Stop`) via `internal/core/supportreg`, so `plugin/server` never imports `support` and `support` never imports `plugin/server` (strictly honoring `docs/architecture/core-design.md` Section 19).
   - Expose read-only **`ze://triage`** in `internal/component/mcp/resources.go` (built into `s.cachedResources` at `NewStreamable` when `cfg.Dispatch != nil`, keeping `listResources()` embedded-only; served by `resources/read` via `"show system triage"` after enforcing `checkCommandScope`), with `stampCacheHints` remaining the **sole** cache-hint site and introducing a `ttlLiveStateMs = 5000` (5 s) freshness class for `ze://` reads (explicitly departing from `docs/architecture/mcp/overview.md` `Cacheable Results`, which previously classified all `resources/read` as 1 h embedded assets), leaving `subscriptions/listen` and push notifications to `plan/post-release/features/spec-mcp2026-5-state-resources.md`.

## Existing Mechanisms (MANDATORY)

Every existing Ze mechanism touched, extended, or reused by this spec:

| Mechanism | Location | Current role | How this spec extends or reuses it |
|-----------|----------|--------------|------------------------------------|
| Firewall drift audit | `internal/component/firewall/audit.go` (`AuditTables`) | Compares `LastApplied()` vs `backend.ListTables()` and raises/clears `firewall-stale-table` and `firewall-drift` report warnings | Preserved unchanged for periodic warning management; `firewall` adds a read-only `OnDriftCheck` handler over `LastApplied()` / `backend.ListTables()` without mutating `report` warnings |
| BGP decision-trace (`reason` & `policy test`) | `internal/component/bgp/plugins/rib/bestpath.go` (`SelectBestExplain`), `rib_pipeline_best.go` (`bestReasonTerminal`), `internal/component/bgp/plugins/cmd/policy/handler.go` (`show policy test`) | Explains BGP best-path selection steps in structured JSON (`show bgp rib best \| reason`) and dry-runs import/export filters (`show policy test`) | Preserved unchanged (zero duplication); cross-referenced beside `show firewall trace` in `ze-diagnostics.md` |
| Plugin callback RPC & Stage 1 declarations | `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_callbacks.go`, `internal/core/ipc/yang/ze-plugin-callback.yang`, `internal/component/plugin/server/{stage1_checks.go,doctor_worker.go}` | Routes `doctor-check` and `runtime-inspect` identically across `DirectBridge` (internal) and TLS `MuxConn` (forked plugins) | Extended with Stage 1 `DriftSubsystems` declaration and optional `ze-plugin-callback:drift-check` (`sdk.OnDriftCheck`) so dataplane drift checks work identically in internal and forked plugins |
| Runtime `show doctor` registry | `internal/core/diagnostic/doctor_registry.go` (`RegisterDoctorCheck`, `DoctorPhaseRuntime`, `DoctorCheck`) | Runs `DoctorPhaseRuntime` checks during `show doctor` and surfaces structured `Diagnostic` entries | Registers `dataplane-drift` (`Order: 860`, `Codes: [CodeDoctorDataplaneDrift]`, `Diagnostic` with `SeverityWarn` and `Fix: "ze show system drift"`) via `diagnostic.Register` + `RegisterDoctorCheck` |
| Task panic containment | `internal/core/taskguard/guard.go` (`Guard`, `Task.Contain`, `Context`) & `internal/component/bgp/reactor/{session.go,session_read.go,session_coalesce.go}` (`finishReadBuffer`, `bgpPanics`) | Contains network-input panics, logs, counts, captures flight-recorder snapshot, writes crash report, raises report error | Extended with optional `Context.RawFrame []byte`; `finishReadBuffer` captures owned bytes (`!*kept` only, before `returnReadBuffer`) into atomic `Session.panicFrame`, and `Guard.record` saves them to `badpacket` |
| Support bundle archive | `internal/component/support/{modules.go,support.go}`, `internal/plugins/support/register.go` | Hardcoded 20-entry `moduleRegistry` map powering `request support archive` (local file and daemon `statestore` routes) | Replaces hardcoded map with core leaf `internal/core/supportreg`; keeps all 20 static modules and adds 7 core + 3 daemon-registered bounded live runtime modules |
| Live triage snapshot | `internal/component/cmd/show/triage.go` (`handleShowSystemTriage`, `triageRuntime`) | Single-call bounded live daemon snapshot (`show system triage`) | Adds O(1) atomic `bad-packet-count` to `runtime` and serves the same command read-only through MCP `ze://triage` |
| MCP resources & caching hints | `internal/component/mcp/{resources.go,caching.go,streamable.go,streamable_tools.go}` | `resources/list` and `resources/read` serve embedded `ui://` assets with single-site `stampCacheHints` (`3600000` ms) | Keeps `listResources()` for `ui://`, appends `ze://triage` to `cachedResources` when `Dispatch != nil`, enforces `checkCommandScope` on `ze://triage` reads, and keeps `stampCacheHints` as the single site adding a `5000` ms live-state TTL class for `ze://` reads |

## Required Reading

### Architecture Docs
- [x] `docs/architecture/diagnostics/production-diagnostics.md` -- AI diagnostics architecture, triage, flight recorder, correlation, watchdog
- [x] `docs/architecture/diagnostics/crash-capture.md` -- `taskguard` panic containment contract
- [x] `docs/architecture/doctor-and-health-checks.md` -- `diagnostic.Register`, `RegisterDoctorCheck`, `DoctorPhaseRuntime`, plugin doctor callbacks
- [x] `docs/architecture/api/commands.md` -- command dispatch, pipe filter ownership, `show` handlers
- [x] `docs/architecture/api/process-protocol.md` -- plugin callback RPCs, Stage 1 declarations, internal vs forked transport symmetry
- [x] `docs/architecture/cli/root-namespace-grammar.md` and `ai/rules/cli.md` -- R1–R9 grammar rules, "the pipe wins", YANG command schema rules
- [x] `docs/architecture/mcp/overview.md` -- MCP `resources/list`, `resources/read`, `Cacheable Results` single-site `stampCacheHints`, scope enforcement
- [x] `docs/architecture/core-design.md` -- module tier boundaries (Section 19), registration pattern, firewall model, support bundle
- [x] `docs/architecture/buffer-architecture.md` -- deliberate wire-path copy table ("When a copy is deliberate")
- [x] `docs/architecture/memory/lifetime-contracts.md` -- `WireUpdate` borrow/transfer boundary and `transferredReadBufferPanic`
- [x] `docs/features/interfaces.md` -- interface backend abstraction and desired state reconciliation
- [x] `plan/post-release/features/spec-mcp2026-5-state-resources.md` -- `ze://` URI namespace design and future subscription boundary

**Key insights:**
- `internal/core/` packages are strictly leaf packages (stdlib + `internal/core/*` only, no upward imports). Putting `drift`, `badpacket`, and `supportreg` in `internal/core/` avoids every cross-component import and keeps `plugin/server` compliant with `core-design.md` Section 19.
- Every plugin (`firewall`, `fib-kernel`, `interface`) can run internal OR forked (`process separate`). Routing plugin drift checks over `ze-plugin-callback:drift-check` (`sdk.OnDriftCheck`) guarantees forked plugins answer identically to internal ones — matching `ai/doors/system.md` and `doctor-and-health-checks.md`.
- `ze.bgp.reactor.coalesce` defaults to `true`, so production BGP reads run through `session_coalesce.go` (`readAndProcessCoalesced` + `flushCoalesce`) as well as `session_read.go` (`readFrame` + `dispatchFrame`). Both share `finishReadBuffer(buf, frameLen, &kept)`. Copying panic-unwind bytes ONLY inside `finishReadBuffer` when `cause != nil && !*kept` (after unwrapping `transferredReadBufferPanic`) guarantees we NEVER read a transferred or `memguard.Poison`ed buffer, and storing it in an `atomic.Pointer[[]byte]` with a separate `describeCancel` prevents any race with the cancel goroutine.
- `docs/architecture/buffer-architecture.md` requires every wire-path copy to be recorded in its "When a copy is deliberate" table; adding the cold-path bad-packet / panic-unwind capture row keeps that contract accurate.
- `docs/architecture/mcp/overview.md` (`Cacheable Results`) mandates ONE single hint-producing site (`stampCacheHints` in `caching.go`) and previously classified all `resources/read` as 1 h embedded assets. Extending `stampCacheHints(req, resp)` to assign `ttlLiveStateMs = 5000` for `ze://` reads keeps the single-site invariant intact while explicitly departing from the 1 h embedded-only classification for `resources/read`.

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/component/firewall/audit.go`, `model.go`, `register.go`, `cmd_show_conntrack.go`, `yang/ze-firewall-cmd.yang`
- [x] `internal/plugins/fib/kernel/fibkernel.go`, `backend.go`
- [x] `internal/component/iface/register.go`, `backend.go`, `config_apply.go`
- [x] `internal/component/bgp/plugins/rib/bestpath.go`, `rib_pipeline_best.go`, `internal/component/bgp/plugins/cmd/policy/handler.go`
- [x] `internal/component/bgp/reactor/session_validation.go`, `session_read.go`, `session_coalesce.go`, `session.go`
- [x] `internal/core/taskguard/guard.go`, `internal/core/diagnostic/doctor_registry.go`
- [x] `internal/component/support/modules.go`, `support.go`, `internal/plugins/support/register.go`
- [x] `internal/component/cmd/show/triage.go`, `yang/ze-cli-show-cmd.yang`
- [x] `internal/component/mcp/resources.go`, `resources_test.go`, `streamable.go`, `streamable_tools.go`, `caching.go`, `caching_test.go`
- [x] `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_callbacks.go`, `sdk_dispatch.go`, `internal/core/ipc/yang/ze-plugin-callback.yang`, `internal/component/plugin/server/stage1_checks.go`

**Behavior to preserve:**
- `firewall.AuditTables()` continues to run and raise/clear `firewall-stale-table` and `firewall-drift` warnings identically.
- `show bgp rib best | reason` and `show policy test` keep their exact grammar and JSON output.
- All 20 existing `request support archive` modules remain registered and unchanged.
- `ui://` embedded MCP resources keep their exact URI validation, `listResources()` output (`TestResources_ListWalksEmbeddedFS`), MIME sniffing, and `ttlEmbeddedAssetMs` (3600000 ms) caching hints.
- Zero allocations added to BGP UPDATE / message receive happy paths (both coalesced and non-coalesced).

**Behavior to change:**
- Additive diagnostic surfaces (`internal/core/drift`, `internal/core/badpacket`, `internal/core/supportreg`, `ze-plugin-callback:drift-check`, `show system drift`, `show system bad-packets`, `show firewall trace`, `dataplane-drift` doctor check, `taskguard.Context.RawFrame`, live support modules, and read-only MCP `ze://triage` with 5s live-state TTL).

## Data Flow (MANDATORY)

### Entry Points
1. CLI / SSH / REST / MCP `show system drift [| subsystem <name>]` -> `ze-cmd:show-system-drift` -> `drift.CheckAll(ctx)` / `drift.Check(ctx, name)` -> local `drift.Register` checkers + daemon remote collector (`plugin/server` `ze-plugin-callback:drift-check` over `DirectBridge` or TLS `MuxConn` to `firewall`, `fib-kernel`, `interface`).
2. `show doctor` -> `diagnostic.RegisterDoctorCheck` (`dataplane-drift`, `DoctorPhaseRuntime`) -> `drift.CheckAll(ctx)` -> `[]diagnostic.Diagnostic` (`CodeDoctorDataplaneDrift`, `SeverityWarn`).
3. CLI / SSH / REST / MCP `show firewall trace table <t> [chain <c>] ...` -> `ze-firewall:show-trace` -> `firewall.EvaluatePacket(firewall.LastApplied(), query)` (pure read-only evaluation).
4. BGP cold paths (`session_validation.go` RFC 7606, `session_read.go` & `session_coalesce.go` header/length errors, `finishReadBuffer` panic unwind when `!*kept` -> `atomic.Pointer` `panicFrame` -> `describeRead` -> `taskguard.Guard.record`) -> `badpacket.Record(...)` -> 16-slot ring -> `show system bad-packets [count <N>] [| protocol <name>]` (`ze-cmd:show-system-bad-packets`) and `show system triage` `runtime.bad-packet-count`.
5. `request support archive` -> `support.Answer` -> `filterModules` (reading `internal/core/supportreg`) -> `collect` / `collectToMemory` (20 static + 7 core live + 3 daemon-registered live modules).
6. MCP HTTP POST `resources/list` & `resources/read` (`uri: "ze://triage"`) -> `Streamable.resourcesList` / `Streamable.resourcesRead` -> `checkCommandScope("show system triage")` -> `s.cfg.Dispatch` -> `stampCacheHints(req, resp)` stamps `ttlMs: 5000`, `cacheScope: "private"`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Daemon ↔ internal or forked dataplane plugins (`firewall`, `fib-kernel`, `interface`) | Stage 1 `DriftSubsystems` + `ze-plugin-callback:drift-check` (`DirectBridge` internal, TLS `MuxConn` forked) bridged into `internal/core/drift` via `RegisterRemoteCollector` | Yes (unit + forked-plugin test + `.ci`) |
| `internal/core/badpacket` ↔ BGP reactor & `taskguard` | Leaf `badpacket.Record(...)` on cold paths only; `finishReadBuffer` copies owned bytes (`!*kept`) before `returnReadBuffer` | Yes (unit `-race` + `.ci`) |
| `internal/core/supportreg` ↔ `internal/component/support` & `internal/component/plugin/server` | Pure stdlib leaf registry `supportreg.Register(name, Collector) func()` (`plugin/server` never imports `support`) | Yes (unit `-race` + `.ci`) |
| `internal/component/mcp` ↔ command dispatcher | `resources/read` (`ze://triage`) enforces `checkCommandScope` then dispatches `"show system triage"`; `stampCacheHints` stamps `ttlLiveStateMs` | Yes (unit + HTTP test) |

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | `drift`, `badpacket`, and `supportreg` live in `internal/core/`; plugin drift checks cross the real RPC boundary (`ze-plugin-callback:drift-check`); `ze://triage` enforces `checkCommandScope` + `s.cfg.Dispatch`. |
| No unintended coupling | Yes | `plugin/server` imports only `internal/core/drift` and `internal/core/supportreg` (never `support`); `support` never imports `plugin/server`; `drift` and `badpacket` never import components. |
| No duplicated functionality | Yes | Reuses `show bgp rib best \| reason` and `show policy test` for BGP; reuses `firewall.LastApplied()`, `fibKernel.listZeRoutes()`, and `iface` `desiredState()`/`ListInterfaces()`. |
| Zero-copy / zero-alloc hot path preserved | Yes | `badpacket.Record` and `finishReadBuffer` panic capture run strictly on malformed/error/panic cold paths; transferred buffers (`*kept == true`) are never read; deliberate cold-path copy recorded in `buffer-architecture.md`. |
| Registration over hardcoding | Yes | `drift.Register` / Stage 1 `DriftSubsystems` and `supportreg.Register` replace hardcoded lists with thread-safe registration + unregister closures. |

## Design

### Pillar 1: `internal/core/drift` + `ze-plugin-callback:drift-check` + First-Wave Checkers + `show system drift` + `show doctor`

- **`internal/core/drift/drift.go` (core leaf — stdlib + `internal/core/{diagnostic,textbuf}` only):**
  - `Status`: `StatusClean = "clean"`, `StatusDrifted = "drifted"`, `StatusUnavailable = "unavailable"`, `StatusError = "error"`.
  - `Kind`: `KindMissing = "missing"`, `KindExtra = "extra"`, `KindMismatch = "mismatch"`.
  - `MaxFindingsPerSubsystem = 64`; `defaultCheckTimeout = 5 * time.Second`.
  - `Finding { Subject string, Kind Kind, Expected string, Actual string, Detail string }` (JSON kebab-case tags).
  - `Report { Subsystem string, Status Status, Checked int, Drifted int, Truncated bool, Findings []Finding, Error string }`.
  - `CheckFunc func(ctx context.Context) Report` and `RemoteCollectorFunc func(ctx context.Context, subsystem string) []Report`.
  - `Register(subsystem string, fn CheckFunc) func()`: thread-safe local registration for daemon-resident components (panics on empty `subsystem` or nil `fn`, monotonic token unregister).
  - `RegisterRemoteCollector(fn RemoteCollectorFunc) func()`: thread-safe daemon fan-out hook filled by `plugin/server` on `StartWithContext` and cleared on `Stop` (exact same leaf-inversion shape as `internal/core/ipsecinventory`).
  - `CheckAll(ctx context.Context) []Report`: runs local checkers (each with 5s timeout) and remote collector (with `subsystem == ""`), merges by sorted subsystem name (deduplicating if a subsystem is registered locally and remotely, preferring the remote/live plugin answer), normalizes `Findings` to non-nil `[]Finding{}`, enforces `MaxFindingsPerSubsystem` cap.
  - `Check(ctx context.Context, subsystem string) (Report, bool)`: checks remote collector first (if registered) then local registry with 5s timeout.
  - `Subsystems() []string`: sorted union of local and remotely declared subsystems (via optional `RegisterRemoteLister(fn func() []string) func()`).
- **`internal/core/drift/register.go` (real `internal/core/diagnostic` API):**
  - `CodeDoctorDataplaneDrift = "doctor-dataplane-drift"`.
  - Registers `CodeDoctorDataplaneDrift` via `diagnostic.Register` ONLY (NOT `RegisterEventCode`, since this check is read-only and never emits transient `report.Raise*` events).
  - Registers runtime doctor check via `diagnostic.RegisterDoctorCheck`:
    ```go
    diagnostic.DoctorCheck{
        Name:      "dataplane-drift",
        Phase:     diagnostic.DoctorPhaseRuntime,
        Order:     860,
        Component: "drift",
        Platforms: []string{diagnostic.DoctorPlatformAny},
        Codes:     []string{CodeDoctorDataplaneDrift},
        Check:     checkDataplaneDrift,
    }
    ```
  - `checkDataplaneDrift(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic`: calls `CheckAll(context.Background())` (bounded 5s/subsystem); for each report with `StatusDrifted` or `StatusError`, emits a `diagnostic.Diagnostic` with `Code: CodeDoctorDataplaneDrift`, `Severity: diagnostic.SeverityWarn`, `Component: "drift"`, clear `Message` naming the subsystem and count/error, and `Fix: "ze show system drift"`. Returns `nil` when all subsystems are clean or unavailable.
- **Process-boundary plugin RPC (`pkg/plugin/rpc`, `pkg/plugin/sdk`, `ze-plugin-callback.yang`, `plugin/server`):**
  - `pkg/plugin/rpc/types.go`: add `DriftSubsystems []string \`json:"drift-subsystems,omitempty"\`` on `DeclareRegistrationInput`, plus value types `DriftFinding`, `DriftCheckInput { Subsystem string }`, `DriftCheckOutput { Subsystem, Status, Checked, Drifted, Truncated, Findings, Error }`.
  - `internal/core/ipc/yang/ze-plugin-callback.yang`: declare `rpc drift-check`.
  - `internal/component/plugin/server/stage1_checks.go`: validate `DriftSubsystems` at Stage 1 (name charset `[a-z0-9-]+`, bounded length/count, no duplicates within a plugin or across running plugins).
  - `pkg/plugin/sdk/sdk_dispatch.go` + `sdk_callbacks.go`: add `callbackDriftCheck = "ze-plugin-callback:drift-check"` and generic `OnDriftCheck(fn func(subsystem string) (rpc.DriftCheckOutput, error))`.
  - `internal/component/plugin/server/drift.go`: tracks active plugins' `DriftSubsystems`; registers `drift.RegisterRemoteCollector` and `drift.RegisterRemoteLister` in `Server.StartWithContext` and unregisters in `Server.Stop()`. Queries matching live plugins via `proc.CallRPC(ctx, "ze-plugin-callback:drift-check", ...)` (5s timeout per plugin, working identically across `DirectBridge` and external TLS `MuxConn`!).
- **First-wave dataplane plugin checkers (all via `sdk.OnDriftCheck` — internal + forked safe):**
  1. **`firewall`** (`internal/component/firewall/drift.go` + `engine.go`): declares `DriftSubsystems: []string{"firewall"}`, answers `OnDriftCheck` by comparing `LastApplied()` vs `GetBackend().ListTables()` (read-only; reports missing tables, extra `ze_*` tables, chain-count and per-chain term-count mismatches; never mutates `report` warnings).
  2. **`fib-kernel`** (`internal/plugins/fib/kernel/drift.go` + `fibkernel.go`): declares `DriftSubsystems: []string{"fib-kernel"}`, answers `OnDriftCheck` by snapshotting `f.installed` under `f.mu.RLock()`, releasing the lock, calling `f.backend.listZeRoutes()`, and comparing main-table (`table == 0`) desired vs actual routes (`missing`, `extra`, `mismatch` on next-hop), capped at 64.
  3. **`interface`** (`internal/component/iface/drift.go` + `register.go`): declares `DriftSubsystems: []string{"interface"}`, answers `OnDriftCheck` by reading `activeCfg.Load()`, calling `GetBackend().ListInterfaces()`, and comparing `cfg.desiredState(devices)` (`managed`, `desiredAddrs`, configured non-zero MTU) against live `InterfaceInfo`s (`missing`, `mismatch`), capped at 64.
- **CLI / MCP command (`internal/component/cmd/show/drift.go` + `ze-cli-show-cmd.yang`):**
  - `show system drift` (`ze-cmd:show-system-drift`) under `container system`.
  - Command-owned pipe filter: `| subsystem <name>` (`command.RegisterPipeFilters("show system drift", []command.PipeFilter{{Name: "subsystem", Argument: command.ArgText, Help: "Filter drift check to one registered subsystem."}})`).
  - Unknown subsystem returns `plugin.OperatorErrorResponse(opererr.New(...))` naming registered subsystems.
  - JSON response: `{"status": "clean|drifted|error", "subsystems": [...], "count": N, "drifted-count": M}`.

### Pillar 2: Firewall Read-Only Decision-Trace (`show firewall trace`) + BGP Alignment

- **`internal/component/firewall/explain.go`:**
  - `TraceQuery`: `Table string`, `Chain string`, `Protocol string`, `SourceIP netip.Addr`, `DestinationIP netip.Addr`, `SourcePort uint16`, `DestinationPort uint16`, `InInterface string`, `OutInterface string`, `ConnState ConnState`, `DSCP *uint8`, `Mark *uint32`.
  - `TraceStep`: `Chain string`, `Term string`, `Matched bool`, `Unevaluated bool` (`omitempty`), `Actions []string` (`omitempty`), `Reason string` (`omitempty`).
  - `TraceResult`: `Table string`, `Family string`, `Chain string`, `Verdict string`, `MatchedChain string` (`omitempty`), `MatchedTerm string` (`omitempty`), `DefaultPolicy bool` (`omitempty`), `Incomplete bool` (`omitempty`), `Steps []TraceStep`.
  - `EvaluatePacket(tables []Table, query TraceQuery) (*TraceResult, *opererr.Error)`:
    - Finds target `Table` by bare name (`StripZeTablePrefix(t.Name) == query.Table`); if not found, returns `opererr.New` listing applied tables.
    - Selects entry chain: `query.Chain` if given (error listing chains if absent), else first base chain sorted by ascending `Priority` (error if table has no chains).
    - Walks terms sequentially with bounded call-stack depth (`maxTraceDepth = 16`):
      - Evaluates `MatchSourceAddress`, `MatchDestinationAddress`, `MatchProtocol`, `MatchSourcePort`, `MatchDestinationPort`, `MatchInputInterface`, `MatchOutputInterface`, `MatchInSet` (IPv4/IPv6 host/CIDR/`RawIntervals`, port, ifname), `MatchConnState`, `MatchDSCP`, `MatchMark`.
      - Predicates not evaluable offline (`MatchDestinationLocal`, `MatchIPsecTransform`, `MatchUnicast`, `MatchTCPFlags`, `MatchICMPType`, `MatchICMPv6Type`, `MatchConnMark`, `MatchIPv4TTLBelow`, `MatchIPv6HopLimitBelow`, ARP/ND matches): marks step `Matched: false, Unevaluated: true` with explicit `Reason` and sets `result.Incomplete = true` so the trace never fabricates a false verdict.
      - Applies `term.Actions` in order: non-terminating modifiers recorded on the step (with `SetMark`/`SetDSCP` updating effective trace packet metadata); `Jump` pushes depth frame and resumes on return; `Goto` transfers without returning; `Return` pops frame; terminal verdicts (`Accept`, `Drop`, `Reject`, `SNAT`, `DNAT`, `Masquerade`, `Redirect`) stop evaluation and set `Verdict`, `MatchedChain`, `MatchedTerm`.
      - Falls through to base chain `Policy.String()` with `DefaultPolicy: true` (or `"return"` for non-base chains).
- **`internal/component/firewall/cmd_trace.go` + `internal/component/firewall/yang/ze-firewall-cmd.yang`:**
  - Registers `ze-firewall:show-trace` via `pluginserver.RegisterRPCs` (no `ze:backend` restriction — works on kernel and VPP).
  - YANG container `show firewall trace` with leaves matching `ze-firewall-conf.yang`: `table` (mandatory), `chain`, `protocol` (`tcp|udp|icmp|icmpv6`), `source-address`, `destination-address`, `source-port` (`1..65535`), `destination-port` (`1..65535`), `input-interface`, `output-interface`, `connection-state` (`new|established|related|invalid`).

### Pillar 3: `internal/core/badpacket` + Cold-Path Hooks + `show system bad-packets`

- **`internal/core/badpacket/badpacket.go` (pure core leaf — stdlib + `internal/core/textbuf` only):**
  - Constants: `MaxRecords = 16`, `MaxPayloadBytes = 4096`.
  - `PacketRecord`: `ID string`, `CapturedAt time.Time`, `Protocol string`, `Peer string`, `MessageType string`, `Action string`, `Reason string`, `Length int`, `Truncated bool`, `Hex string`.
  - `CopyBounded(raw []byte) []byte`: returns a newly owned copy of at most `MaxPayloadBytes` bytes (nil if empty). Used by panic-unwind paths to detach bytes before a pooled buffer is recycled.
  - `Record(protocol, peer, messageType, action, reason string, raw []byte) string`: copies/hex-encodes up to `MaxPayloadBytes` (setting `Truncated: len(raw) > MaxPayloadBytes`), stores into 16-slot circular ring under mutex, increments atomic counter, returns `"pkt-<seq>"`.
  - `Snapshot(limit int, protocol string) []PacketRecord`: newest-first, case-insensitive protocol filter, non-nil slice.
  - `Count() uint64`: atomic total recorded count.
- **Cold-path capture sites (zero happy-path allocation, strict lifetime-contract safety):**
  1. `internal/component/bgp/reactor/session_validation.go`: helper `s.recordBadUpdate(action, reason string, raw []byte)` called on all non-`RFC7606ActionNone` branches in `applyRFC7606` (synchronous copy inside `badpacket.Record` before `receivedWU` leaves scope).
  2. `internal/component/bgp/reactor/session_read.go` (`readFrame`) AND `internal/component/bgp/reactor/session_coalesce.go` (`readAndProcessCoalesced`): record `"HEADER"` into `badpacket` on both `message.ParseHeader` error and `hdr.ValidateLengthWithMax` error (covering both standard and default-enabled coalesced read loops!).
  3. `internal/core/taskguard/guard.go`: adds optional `RawFrame []byte` to `taskguard.Context`; in `Guard.record`, when `len(p.RawFrame) > 0`, calls `badpacket.Record(p.Component, p.Unit, p.MessageType, "task-panic", panicText, p.RawFrame)` and attaches `"bad-packet-id"` to `detail` and `attrs`.
  4. `internal/component/bgp/reactor/session.go`, `session_read.go`, `session_coalesce.go`:
     - `Session` adds `panicFrame atomic.Pointer[[]byte]` (lock-free, race-free).
     - `finishReadBuffer(buf BufHandle, frameLen int, kept *bool)` (shared by BOTH `dispatchFrame` in `session_read.go` AND `readAndProcessCoalesced` / `flushCoalesce` in `session_coalesce.go`):
       - On normal return (`cause == nil`): zero overhead, zero allocations, returns `buf` if `!*kept`.
       - On panic unwind (`cause != nil`): first unwraps `transferredReadBufferPanic` and sets `*kept = true` if the buffer was transferred. **ONLY when `!*kept` AND `frameLen > 0`** (proving the read goroutine still exclusively owns `buf` and it has NOT been transferred or `memguard.Poison`ed), copies `buf.Buf[:min(frameLen, len(buf.Buf))]` via `badpacket.CopyBounded` and stores it via `s.panicFrame.Store(&copied)`, THEN calls `s.returnReadBuffer(buf)` and re-panics `cause`.
     - `s.describeRead()` swaps `s.panicFrame.Swap(nil)` into `Context.RawFrame`.
     - The cancel goroutine (`session.go:1554`) uses `s.describeCancel()` (returning `taskguard.Context{Unit: s.settings.Address.String()}` without touching `receivedType` or `panicFrame`), eliminating any cross-goroutine data race under `-race`.
  5. `docs/architecture/buffer-architecture.md`: add deliberate-copy trigger row to `### When a copy is deliberate` covering cold-path malformed-frame and unowned-buffer panic-unwind capture into `badpacket`.
- **CLI / MCP command (`internal/component/cmd/show/bad_packets.go` + `ze-cli-show-cmd.yang`):**
  - `show system bad-packets [count <N>]` (`ze-cmd:show-system-bad-packets`, `count` range `1..16`, default `16`).
  - Command-owned pipe filter: `| protocol <name>` (`command.ArgText`).
  - `show system triage` `runtime` adds `"bad-packet-count": badpacket.Count()`.

### Pillar 4: Registrable Support Modules (`internal/core/supportreg`) + Live Runtime Modules + MCP `ze://triage`

- **`internal/core/supportreg/registry.go` (pure stdlib core leaf — zero upward imports, honoring `docs/architecture/core-design.md` Section 19):**
  - `CollectOptions { ConfigPath string, Since string, Sensitive bool, SinceTime time.Time }`.
  - `Collector func(opts *CollectOptions) (any, error)`.
  - `Register(name string, fn Collector) func()`: thread-safe `sync.RWMutex` registry with monotonic token; panics on empty `name`, nil `fn`, or duplicate live `name`; returned unregister closure removes only if token matches (idempotent, restart-safe).
  - `Names() []string` (sorted), `Snapshot() map[string]Collector` (shallow copy).
- **`internal/component/support/modules.go`:**
  - Delegates `ModuleNames()`, `ModuleList()`, and `filterModules()` to `internal/core/supportreg`.
  - Registers all 20 existing static modules into `supportreg` in `init()`.
- **`internal/component/support/runtime_collectors.go`:**
  - Registers 7 core-tier bounded live runtime modules into `supportreg` in `init()`: `log-ring` (<= 200 entries, honors `opts.SinceTime`), `report`, `health`, `watchdog`, `bad-packets`, `drift` (10s overall cap), `goroutines` (count + 64 KiB capped stack dump).
- **`internal/component/plugin/server/support_modules.go` (wired from `Server.StartWithContext` / `Server.Stop`):**
  - Imports ONLY `internal/core/supportreg` (never `internal/component/support`) and registers 3 daemon-backed live modules on start and unregisters on stop: `event-ring` (<= 200 entries), `plugins` (live plugin health summary), `reload-history` (32-entry reload journal).
- **MCP `ze://triage` (`internal/component/mcp/{resources.go,caching.go,streamable.go,streamable_tools.go}`):**
  - Constants: `zeScheme = "ze://"`, `triageResourceURI = "ze://triage"`, `ttlLiveStateMs = 5000`.
  - Keep `listResources()` walking ONLY embedded `ui://` assets (preserving `TestResources_ListWalksEmbeddedFS`).
  - In `NewStreamable`: build `cachedResources` once at construction from `listResources()`, appending the `ze://triage` entry when `cfg.Dispatch != nil` (immutable for server lifetime, so `resources/list` stays immutable/1 h cacheable!).
  - In `Streamable.resourcesRead(ctx, scope, req, remoteAddr)`:
    - If `params.URI == triageResourceURI`:
      - If `s.cfg.Dispatch == nil`: return `resource not found`.
      - Enforce `s.checkCommandScope(req.ID, scope.Identity, "show system triage")` (identical RBAC to `tools/call`).
      - Dispatch `"show system triage"` through `s.cfg.Dispatch` (read-only) and return `contents: [{"uri": "ze://triage", "mimeType": "application/json", "text": jsonText}]`. Sets NO cache hints inside the handler!
    - If `strings.HasPrefix(params.URI, zeScheme)` (any other `ze://` URI): return `resource not found`.
    - Otherwise delegate to `readResource(params.URI)` (`ui://` embedded assets, untouched).
  - **Single-site caching hints (`internal/component/mcp/caching.go`):**
    - `stampCacheHints(req *request, resp *response) *response` remains the **SOLE** site that writes `ttlMs` and `cacheScope`!
    - `cacheTTLByMethod` stays the exact 4-method closed table (`server/discover`, `tools/list`, `resources/list`, `resources/read`).
    - When `req.Method == methodResourcesRead`, `stampCacheHints` inspects `params.uri`: URIs with prefix `ze://` receive `ttlLiveStateMs` (`5000` ms); `ui://` URIs receive `ttlEmbeddedAssetMs` (`3600000` ms). Both receive `cacheScope: "private"`.
    - Explicitly updates `docs/architecture/mcp/overview.md` (`Cacheable Results` & `Resources Capability`) to document the third compile-time freshness class (`ttlLiveStateMs = 5000` for `ze://` live-state reads).

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | system door; `internal/component/support/*`, `pkg/plugin/*` | "Every plugin, ours included, runs either internal (a goroutine in the daemon) or external (a forked subprocess that talks only over TLS and RPC). The SDK hides which. Code that only works internal is broken." | agrees | Plugin drift checks route through `ze-plugin-callback:drift-check` (`sdk.OnDriftCheck`) over `DirectBridge` (internal) and TLS `MuxConn` (external), so internal and forked plugins behave identically |
| `docs/architecture/diagnostics/crash-capture.md` | Recovered panics | `// Design:` of `internal/core/taskguard/guard.go`; modified doc | "A recovered panic is never silent." | agrees | Adding optional `RawFrame` to `taskguard.Context` keeps the boundary at task entry and adds a bounded bad-packet record beside the existing five outputs |
| `ai/doors/core.md` | Where it sits | door covering `internal/core/{taskguard,drift,badpacket,supportreg}` | "It imports neither `internal/component/` nor `internal/plugins/`." | agrees | `internal/core/drift`, `internal/core/badpacket`, and `internal/core/supportreg` import stdlib + `internal/core/*` only; components register into them |
| `ai/doors/forwarding.md` | Who depends on it from outside | door covering `firewall`, `fib/kernel`, `iface` | "Ze owns only what it marked. It owns routes by `rtm_protocol` (`internal/core/rtproto`, `FIBKernel` 250, `Static` 251) and nft tables by the `ze_` name prefix" | agrees | `firewall`, `fib-kernel`, and `interface` drift checkers are strictly read-only and inspect only Ze-owned tables, routes, and configured interfaces |
| `docs/architecture/core-design.md` | 19. Component Boundaries | `// Design:` of `bgp/reactor`, `fib/kernel`, `support`; modified doc | "Each component under `internal/component/` is independently removable." | agrees | `drift`, `badpacket`, and `supportreg` live in `internal/core/` as leaf registries so `plugin/server` registers live support modules without importing `support` |
| `docs/architecture/buffer-architecture.md` | The wire-to-wire buffer path | `// Design:` of `internal/component/bgp/reactor/session.go`; modified doc | "A copy on the wire path MUST match one of the triggers below. A copy that fits none of them is a defect until this table names the new trigger." | agrees | Cold-path malformed-frame and unowned-buffer panic-unwind copies into `badpacket` are explicitly added to the "When a copy is deliberate" table in `docs/architecture/buffer-architecture.md` |
| `docs/architecture/api/architecture.md` | Command parameter values | anchored to `bgp/reactor/session.go`, `plugin/server/server.go` | "The shared API engine resolves structured parameters against one current command declaration before authorization and dispatch." | agrees | New diagnostic commands declare YANG leaves and dispatch through the shared command registry without adding protocol logic to API adapters |
| `docs/architecture/behavior/fsm-active.md` | TL;DR | anchored to `bgp/reactor/session.go` | - | silent | Active-state FSM transitions are unchanged; `session.go` only stores atomic panic-unwind frame bytes on abnormal read-loop unwind |
| `docs/architecture/behavior/fsm-connect.md` | TL;DR | anchored to `bgp/reactor/{session,session_read}.go` | - | silent | Connect-state FSM transitions are unchanged; header-error capture runs after header parse failure |
| `docs/architecture/behavior/fsm-established.md` | TL;DR | anchored to `bgp/reactor/{session,session_read}.go` | - | silent | Established-state FSM transitions are unchanged; RFC 7606 capture only records already-rejected frames |
| `docs/architecture/behavior/fsm-idle.md` | TL;DR | anchored to `bgp/reactor/session.go` | - | silent | Idle-state FSM transitions are unchanged |
| `docs/architecture/behavior/fsm-open-confirm.md` | TL;DR | anchored to `bgp/reactor/{session,session_read}.go` | - | silent | OpenConfirm FSM transitions are unchanged |
| `docs/architecture/behavior/fsm-open-sent.md` | TL;DR | anchored to `bgp/reactor/{session,session_read}.go` | - | silent | OpenSent FSM transitions are unchanged |
| `docs/architecture/behavior/fsm.md` | TL;DR (Read This First) | anchored to `bgp/reactor/{session,session_read}.go` | - | silent | BGP FSM state machine transitions and events are unchanged |
| `docs/architecture/behavior/peer-lifecycle.md` | TL;DR | anchored to `bgp/reactor/{session,session_read}.go` | - | silent | Peer lifecycle connect/teardown/backoff behavior is unchanged |
| `docs/architecture/chaos-web-dashboard.md` | Architecture Overview | anchored to `bgp/reactor/session.go` | - | silent | Chaos dashboard is unaffected |
| `docs/architecture/congestion-industry.md` | Sources | anchored to `bgp/reactor/session.go` | - | silent | Congestion control and write-path backpressure are unaffected |
| `docs/architecture/edge-cases/extended-message.md` | TL;DR | anchored to `bgp/reactor/{session,session_read}.go` | - | silent | RFC 8654 length validation behavior and NOTIFICATION codes are unchanged; rejected header bytes are recorded into `badpacket` on both standard and coalesced paths |
| `docs/architecture/encoding-context.md` | TL;DR (Read This First) | anchored to `bgp/reactor/{session,session_read}.go` | - | silent | `EncodingContext` negotiation and `ContextID` forwarding are unchanged |
| `docs/architecture/forward-congestion-pool.md` | Invariant | anchored to `bgp/reactor/{session,session_read}.go` | - | silent | Forward congestion pool is unaffected |
| `docs/architecture/memory/lifetime-contracts.md` | TL;DR | anchored to `bgp/reactor/{session,session_read}.go` | "A borrow is valid only until a named **boundary** (a handler return, a dispatch completion, a handle release). Reading past the boundary reads recycled memory." | agrees | `finishReadBuffer` unwraps `transferredReadBufferPanic` first and copies bytes into atomic `panicFrame` ONLY when `!*kept` (still owned by the read goroutine, before `returnReadBuffer`), never reading a transferred or poisoned buffer |
| `docs/architecture/message-buffer-design.md` | Overview | anchored to `bgp/reactor/session.go` | - | silent | Message buffer pool layout and lifecycle are unchanged |
| `docs/architecture/rib-transition.md` | Executive Summary | anchored to `bgp/reactor/session.go` | - | silent | RIB transition flow is unaffected |
| `docs/architecture/update-building.md` | TL;DR (Read This First) | anchored to `bgp/reactor/session.go` | - | silent | UPDATE serialization is unchanged |
| `ai/doors/bgp-engine.md` | Boundaries | door covering `internal/component/bgp/reactor/*` | "A received UPDATE points into a reusable read buffer, so `RawMessage.IsAsyncSafe()` is false and anything that hands it to another goroutine copies first" | agrees | `badpacket.Record` copies up to 4 KiB synchronously on cold error paths, and `finishReadBuffer` copies owned bytes (`!*kept`) before releasing the read buffer on panic unwind |
| `docs/architecture/edge-cases/as4.md` | Overview | anchored to `bgp/reactor/{session_read,session_validation}.go` | - | silent | AS4 attribute validation is unchanged; malformed frames are additionally captured by `badpacket` |
| `docs/architecture/wire/attributes.md` | TL;DR (Read This First) | anchored to `bgp/reactor/{session_read,session_validation}.go` | - | silent | Wire attribute parsing and RFC 7606 verdicts are unchanged; rejected frames are additionally captured by `badpacket` |
| `docs/architecture/wire/messages.md` | TL;DR (Read This First) | anchored to `bgp/reactor/{session_read,session_validation}.go` | - | silent | Wire message framing and header validation are unchanged; rejected header bytes are additionally captured by `badpacket` |
| `docs/architecture/rfc-may-decisions.md` | RFC 4760 Section 7 - Non-Negotiated AFI/SAFI Handling | anchored to `bgp/reactor/session_validation.go` | - | silent | No RFC MAY decision changes |
| `docs/architecture/route-selection.md` | Overview | anchored to `bgp/reactor/session_validation.go` | - | silent | Existing `show bgp rib best \| reason` (`SelectBestExplain`) is preserved unchanged |
| `docs/architecture/decisions/001-pull-model-metrics.md` | The Question | anchored to `internal/plugins/fib/kernel/fibkernel.go` | - | silent | `fib-kernel` metrics stay pull-model and unchanged |
| `docs/features/interfaces.md` | Interface Discovery | `// Design:` of `internal/component/iface/register.go` | "The `DiscoverInterfaces` function enumerates interfaces via `ListInterfaces`" | agrees | `interface` drift checker answers `OnDriftCheck` by comparing last-applied `desiredState()` against `Backend.ListInterfaces()` read-only |
| `docs/architecture/config/apply-ordering.md` | The requirement | anchored to `internal/component/iface/register.go` | - | silent | Config apply ordering is unchanged |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to `internal/component/iface/register.go` | - | silent | Config transaction verify/apply/rollback protocol is unchanged |
| `docs/architecture/iface/logical-name-resolution.md` | Where the resolver lives, and what it returns | anchored to `internal/component/iface/register.go` | - | silent | `interface` drift checker reuses resolved logical-to-OS device names from `cfg.desiredState(devices)` |
| `docs/architecture/iface/management.md` | Two layers, not one flat list | anchored to `internal/component/iface/register.go` | - | silent | Interface reconciliation and ownership semantics are unchanged; drift check is strictly read-only |
| `docs/architecture/l2tp/cpe-1-pppoe-client.md` | RFC obligations carried by this code | anchored to `internal/component/iface/register.go` | - | silent | PPPoE client lifecycle is unaffected |
| `docs/architecture/plugin/plugin-relationships.md` | 1. System Plugins | anchored to `internal/component/iface/register.go` | - | silent | Plugin dependency ordering is unchanged |
| `docs/architecture/plugin/plugin-system.md` | Registration fields | anchored to `internal/component/iface/register.go` | "A refused startup registration in `init()` is a compile-time Ze defect and panics with `panic("BUG: ...")`, never `os.Exit`." | agrees | `drift.Register` and `supportreg.Register` follow Ze's registration pattern and panic on invalid/duplicate static registration |
| `docs/architecture/traffic/cos-plugin.md` | Decisions | anchored to `internal/component/iface/register.go` | - | silent | Class-of-service plugin is unaffected |
| `docs/architecture/api/commands.md` | Command Categories | `// Design:` of `cmd/show/show.go`; modified doc | "The daemon registers each built-in handler under its YANG command path, derived from the tree (`LoadBuiltins`: `d.RegisterWithOptions(wireToPath[wireMethod], ...)`)." | agrees | `show system drift` (`\| subsystem`), `show system bad-packets` (`\| protocol`), and `show firewall trace` declare YANG nodes and register built-in RPC handlers |
| `ai/doors/commands-cli.md` | Boundaries | door covering `internal/component/cmd/show/*` | "A handler answers data, never text." | agrees | All three new commands declare YANG containers/leaves, register RPC handlers, and return structured `plugin.Map` payloads |
| `docs/architecture/diagnostics/production-diagnostics.md` | CLI first, MCP for free | `// Design:` of `cmd/show/triage.go`; modified doc | "Every diagnostic is a YANG RPC registered as a CLI command. The MCP tool list is generated from the registered RPCs, so a new diagnostic appears to an AI agent with no MCP code change." | agrees | `show system drift`, `show system bad-packets`, and `show firewall trace` register as YANG RPCs; `ze://triage` adds a standard read-only MCP resource aligned with `spec-mcp2026-5-state-resources.md` |
| `docs/architecture/api/process-protocol.md` | Runtime RPCs | `// Design:` of `plugin/server/server.go`, `pkg/plugin/sdk/*`; modified doc | "Engine to Plugin (Callbacks)" | agrees | `ze-plugin-callback:drift-check` is added beside `doctor-check` and `runtime-inspect` in `ze-plugin-callback.yang` with Stage 1 `drift-subsystems` validation and works identically across `DirectBridge` and external TLS `MuxConn` |
| `docs/architecture/api/ipc_protocol.md` | Overview | `// Design:` of `pkg/plugin/rpc/types.go`; anchored to `ze-plugin-callback.yang`, `sdk_dispatch.go`, `sdk_callbacks.go` | - | silent | IPC framing and width-framed answer wire encoding are unchanged; `drift-check` uses the existing JSON-RPC callback framing |
| `docs/architecture/api/wire-format.md` | Transport | anchored to `pkg/plugin/rpc/types.go` | - | silent | Wire-format transport layer is unchanged |
| `docs/architecture/firewall/firewall-irr.md` | Decisions | anchored to `pkg/plugin/rpc/types.go` | - | silent | Firewall IRR set population is unaffected |
| `docs/architecture/forked-route-install.md` | The wire carries a name, the engine resolves it | anchored to `pkg/plugin/rpc/types.go` | - | silent | Forked route-install RPC is unaffected |
| `docs/architecture/plugin-manager-wiring.md` | Problem | anchored to `pkg/plugin/rpc/types.go` | - | silent | Plugin manager wiring is unaffected |
| `docs/architecture/system-architecture.md` | System RIB process boundary | anchored to `pkg/plugin/rpc/types.go` | - | silent | System RIB process boundary is unaffected |
| `docs/architecture/wire/nlri.md` | TL;DR (Read This First) | anchored to `pkg/plugin/rpc/types.go` | - | silent | NLRI wire codecs are unaffected |
| `docs/architecture/firewall/firewall-domain-group.md` | Why a plugin | anchored to `pkg/plugin/sdk/sdk_callbacks.go` | - | silent | Firewall domain group enricher is unaffected |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | anchored to `pkg/plugin/sdk/sdk_callbacks.go` | - | silent | IKEv2 engine is unaffected |
| `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` | Rule 1: the backend loads on demand | anchored to `internal/component/firewall/engine.go` | - | silent | Firewall table ownership, on-demand backend loading, and shutdown flush are unchanged; `firewall` `OnDriftCheck` is strictly read-only |
| `docs/architecture/config/yang-config-design.md` | 1. Key Principle | anchored to `internal/component/plugin/server/server.go` | - | silent | YANG config schema loading and publication are unchanged |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | anchored to `plugin/server/server.go`; modified doc | "A plugin's runtime check reads state that lives in the plugin's own process, so its one route is the doctor callback, which reaches a forked plugin as well as an internal one." | agrees | Plugin drift checks run inside each plugin's own process via `ze-plugin-callback:drift-check` (reaching forked and internal plugins identically), and `internal/core/drift` registers `dataplane-drift` in `diagnostic.DoctorPhaseRuntime` via `diagnostic.RegisterDoctorCheck` |
| `docs/architecture/hub-architecture.md` | Vision | anchored to `internal/component/plugin/server/server.go` | - | silent | Hub startup and service lifecycle are unchanged |
| `docs/architecture/overview.md` | 1. What is Ze? | anchored to `internal/component/plugin/server/server.go` | - | silent | High-level system overview is unchanged |
| `docs/architecture/testing/runner-architecture.md` | `.ci` and `.et` formats | anchored to `internal/component/plugin/server/server.go` | "config/process tests: tmpfs blocks, `cmd=` process orchestration, and `expect=` assertions (exit code, stdout/stderr, JSON, syslog, file, HTTP)." | agrees | `test/plugin/diag-drift-explain-and-support.ci` proves live `show system drift`, `show firewall trace`, `show system bad-packets`, and `request support archive` end to end |
| `docs/architecture/update-cache.md` | Purpose | anchored to `internal/component/plugin/server/server.go` | - | silent | Update cache behavior is unchanged |
| `ai/doors/plugin-infra.md` | Boundaries | door covering `internal/component/plugin/server/*`, `pkg/plugin/*` | "A new callback is one `On*` method in `sdk_callbacks.go`." | agrees | `OnDriftCheck` is one generic `On*` callback in `sdk_callbacks.go`, and `plugin/server` registers its live support modules via `internal/core/supportreg` without importing `support` |
| `docs/architecture/mcp/overview.md` | Cacheable Results | `// Design:` of `mcp/{resources,caching,streamable_tools}.go`; modified doc | "Embedded asset: `cachedResources` is built once at construction and the bytes come from `//go:embed`, so they are fixed for the binary's lifetime" | departs | `resources/read` now serves live `ze://triage` beside static `ui://` embedded assets: `stampCacheHints` stays the sole hint-producing site, assigning a new compile-time `ttlLiveStateMs = 5000` (5 s) class to `ze://` reads while `ui://` reads and `resources/list` (still built once at construction) keep `3600000` (1 h) and `cacheScope: "private"` |
| `ai/doors/api-surfaces.md` | Boundaries | door covering `internal/component/mcp/*` | "Never add a command in an adapter." | agrees | `ze://triage` is an MCP resource adapter that delegates to `"show system triage"` through `s.cfg.Dispatch` after enforcing `checkCommandScope`, owning no diagnostic logic of its own |
| `ai/doors/le-tooling.md` | Boundaries | door covering `test/plugin/diag-drift-explain-and-support.ci` | "A compiled Go fixture returns an error when an assertion fails" | agrees | `test/plugin/diag-drift-explain-and-support.ci` follows `.ci` state-assertion rules and is stress-proven under `./le test stress-repro` before promotion |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` of `bgp/reactor`, `fib/kernel`, `support`; modified doc | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Unchanged; `show firewall trace` is backend-independent (no `ze:backend "kernel"`), and `fib-kernel` answers `drift-check` only when `fib-kernel` actually runs |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` of `bgp/reactor`, `fib/kernel`, `support`; modified doc | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Unchanged; this spec alters no commit, reload, or config-validation data-plane mismatch gate |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` of `bgp/reactor`, `fib/kernel`, `support`; modified doc | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Unchanged; `show firewall trace` reads the abstract `firewall.Table` model and carries no `ze:backend` annotation |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` of `bgp/reactor`, `fib/kernel`, `support`; modified doc | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Unchanged; this spec introduces no new data-plane selector leaf or backend name |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` of `bgp/reactor`, `fib/kernel`, `support`; modified doc | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Unchanged; this spec touches no BGP egress next-hop rewriting or session configuration |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` of `bgp/reactor`, `fib/kernel`, `support`; modified doc | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Unchanged; this spec touches no BGP policy modify or export chain configuration |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` of `bgp/reactor`, `fib/kernel`, `support`; modified doc | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Unchanged; this spec touches no BGP policy chain compilation |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` of `bgp/reactor`, `fib/kernel`, `support`; modified doc | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Unchanged; this spec touches no BGP export chain evaluation |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` of `bgp/reactor`, `fib/kernel`, `support`; modified doc | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Unchanged; this spec touches no BGP next-hop validation |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` of `bgp/reactor`, `fib/kernel`, `support`; modified doc | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Unchanged; this spec touches no BGP route origination or forwarding |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` of `bgp/reactor`, `fib/kernel`, `support`; modified doc | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Unchanged; this spec introduces no automatic filter-chain insertions |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` of `bgp/reactor`, `fib/kernel`, `support`; modified doc | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Drift checks and support module registration never refuse or abort daemon startup; drift findings are reported non-fatally through `show doctor` (`doctor-dataplane-drift`) and `show system drift` |
| `docs/architecture/core-design.md` | Design decisions | `// Design:` of `bgp/reactor`, `fib/kernel`, `support`; modified doc | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Unchanged; when a feature stays off at start, its plugin is not running so no drift error is raised |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `bgp/reactor/session.go`, `plugin/server/server.go` | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Unchanged; this spec touches no `\| implied` config formatting |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `bgp/reactor/session.go`, `plugin/server/server.go` | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | Unchanged; no bus participant routing semantics change |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `bgp/reactor/session.go`, `plugin/server/server.go` | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Unchanged; no bus routing filters change |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `bgp/reactor/session.go`, `plugin/server/server.go` | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Unchanged; no bus routing filters change |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `bgp/reactor/session.go`, `plugin/server/server.go` | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Unchanged; no bus allow-lists change |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `bgp/reactor/session.go`, `plugin/server/server.go` | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Unchanged; no bus config compilation changes |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `bgp/reactor/session.go`, `plugin/server/server.go` | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Unchanged; no bus arbitration is added |
| `docs/architecture/api/architecture.md` | Design decisions | anchored to `bgp/reactor/session.go`, `plugin/server/server.go` | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Unchanged; bus routing display is untouched |
| `docs/architecture/update-building.md` | Design decisions | anchored to `bgp/reactor/session.go` | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | Unchanged; FlowSpec UPDATE emission is untouched |
| `docs/features/interfaces.md` | Design decisions | `// Design:` of `internal/component/iface/register.go` | "`ze init` claims every discovered device by default, including the ones another program owns. `ze init --confirm` asks about each discovered device and writes only the ones the operator confirms" | agrees | Unchanged; `ze init` discovery behavior is untouched |
| `docs/features/interfaces.md` | Design decisions | `// Design:` of `internal/component/iface/register.go` | "`ze init --confirm` takes only an explicit answer: `yes`, `y`, `no` or `n`. Anything else, an empty line included, is not accepted, and the same device is asked again" | agrees | Unchanged; `ze init --confirm` prompt handling is untouched |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | anchored to `internal/component/iface/register.go` | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | Unchanged; offline config parsing and `request support archive` never start plugin run loops |
| `docs/architecture/api/commands.md` | Design decisions | `// Design:` of `cmd/show/show.go`; modified doc | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | `show system drift` narrows by `\| subsystem <name>` and `show system bad-packets` narrows by `\| protocol <name>` instead of adding extra command-path subtrees |
| `docs/architecture/api/commands.md` | Design decisions | `// Design:` of `cmd/show/show.go`; modified doc | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Neither `\| subsystem` nor `\| protocol` is a generic catalog operator; both are command-owned narrowing filters returning the command's own structured data shape |
| `docs/architecture/api/commands.md` | Design decisions | `// Design:` of `cmd/show/show.go`; modified doc | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | `\| subsystem <name>` narrows `show system drift` to one subsystem and `\| protocol <name>` narrows `show system bad-packets` to one protocol without mutating state |
| `docs/architecture/api/process-protocol.md` | Design decisions | `// Design:` of `plugin/server/server.go`, `pkg/plugin/sdk/*`; modified doc | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | `ze-plugin-callback:drift-check` is a daemon-to-plugin callback (parallel to `doctor-check`), leaving engine operations and plugin authorization profiles unchanged |
| `docs/architecture/api/process-protocol.md` | Design decisions | `// Design:` of `plugin/server/server.go`, `pkg/plugin/sdk/*`; modified doc | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Unchanged; `event-ring` support module reads a bounded snapshot without affecting lossless event delivery |

## Design presentation

### What changes
1. **Unified process-boundary-safe dataplane drift registry (`internal/core/drift`, `ze-plugin-callback:drift-check`, `show system drift`, `show doctor`):** Pure core leaf registry with bounded 5s per-checker timeout and 64-finding cap, backed by a generic plugin callback RPC (`sdk.OnDriftCheck`) so internal and forked (`process separate`) plugins answer identically. First-wave checkers cover `firewall`, `fib-kernel`, and `interface`, surfaced via `show system drift [| subsystem <name>]` and a `DoctorPhaseRuntime` `show doctor` check (`doctor-dataplane-drift`).
2. **Read-only firewall decision-trace (`firewall.EvaluatePacket`, `show firewall trace`):** Evaluates `firewall.LastApplied()` term-by-term (addresses, ports, protocols, interfaces, named sets, conntrack state, jumps/gotos, modifiers, default policy) without touching nftables or VPP, marking kernel-only predicates explicitly as `unevaluated: true`. Existing BGP explain (`show bgp rib best | reason` and `show policy test`) is preserved unchanged and cross-referenced in `ze-diagnostics.md`.
3. **Always-on bad-packet ring (`internal/core/badpacket`, `show system bad-packets`):** Fixed 16-slot x 4 KiB circular ring (64 KiB max) wired on BGP RFC 7606 malformed UPDATEs, BGP header/length errors across BOTH standard (`session_read.go`) and default coalesced (`session_coalesce.go`) read paths, and `taskguard` panic recovery (`finishReadBuffer` copies owned bytes ONLY when `!*kept` before `returnReadBuffer`, stored via atomic `panicFrame`, with separate `describeCancel` so there is zero race and zero poison read). Exposed via `show system bad-packets [count <N>] [| protocol <name>]` and `show system triage` `runtime.bad-packet-count`.
4. **Registrable support bundle modules (`internal/core/supportreg`) + MCP `ze://triage`:** Replaces hardcoded `support.moduleRegistry` map with pure stdlib core leaf `internal/core/supportreg`, adds 7 core live runtime modules (`log-ring`, `report`, `health`, `watchdog`, `bad-packets`, `drift`, `goroutines`) plus 3 daemon-registered modules (`event-ring`, `plugins`, `reload-history`), and exposes read-only `ze://triage` over MCP `resources/list` + `resources/read` with `checkCommandScope` and single-site `stampCacheHints` stamping `ttlLiveStateMs = 5000` (5 s).

### Before and after
| Area | Before | After |
|------|--------|-------|
| Dataplane drift visibility | Only `firewall.AuditTables()` raised warnings; no unified query or `fib-kernel`/`interface` drift check | `internal/core/drift` + `ze-plugin-callback:drift-check` (`firewall`, `fib-kernel`, `interface`, internal + forked) + `show system drift [\| subsystem <name>]` + `show doctor` (`doctor-dataplane-drift`) |
| Decision tracing | BGP had `show bgp rib best \| reason` and `show policy test`; firewall had no packet trace | Firewall adds backend-independent read-only `show firewall trace` alongside existing BGP explain commands |
| Malformed wire reproducers | Malformed BGP frames / panic frames were logged as summaries and discarded | Always-on 16-slot 4 KiB-capped `internal/core/badpacket` ring queried via `show system bad-packets [\| protocol <name>]` |
| Support archive & MCP resources | Hardcoded 20-module map omitting live runtime state; MCP `resources/read` served only static `ui://` assets with 1 h TTL | `internal/core/supportreg` leaf + 10 live runtime modules (7 core, 3 daemon); MCP serves scope-checked read-only `ze://triage` (`ttlMs: 5000`) alongside `ui://` (`ttlMs: 3600000`) |

### Alternatives set aside
- **Process-local `drift.Register` for SDK plugins (`fib-kernel`, `interface`, `firewall`):** Rejected because forked (`process separate`) plugins run in a child process and would be invisible to the daemon's `show system drift` and `show doctor`. Using `ze-plugin-callback:drift-check` (`sdk.OnDriftCheck`) makes internal and forked plugins identical.
- **Per-subsystem `show <subsystem> drift` commands:** Set aside in favor of one `show system drift | subsystem <name>` command so "the pipe wins" holds and every future plugin declaring `DriftSubsystems` is exposed automatically without new YANG commands.
- **Stubbing drift checkers for `vpp`, `ipsec`, `mpls`:** Set aside so every shipped checker performs real intent-vs-actual readback; optional engines plug into `OnDriftCheck` / `drift.Register` once their readback seams are ready.
- **Adding a redundant `show bgp explain` command:** Set aside because `show bgp rib best | reason` (`SelectBestExplain`) and `show policy test` already expose structured BGP decision traces.
- **Implementing `subscriptions/listen` push for `ze://triage`:** Left to `plan/post-release/features/spec-mcp2026-5-state-resources.md`; this spec delivers read-only on-demand `ze://triage` over existing `resources/list` + `resources/read`.

### Documents departed from
- **`docs/architecture/mcp/overview.md` (`Cacheable Results`):**
  - **Before:** All `resources/read` results belonged unconditionally to the `ttlEmbeddedAssetMs` (`3600000` ms / 1 h) freshness class because `resources/read` served only compile-time `//go:embed` `ui://` assets.
  - **After:** `stampCacheHints` (`internal/component/mcp/caching.go`) remains the sole hint-producing site, and `cacheTTLByMethod` stays the exact 4-method closed table, but `resources/read` distinguishes two compile-time freshness classes by URI scheme: `ze://` live-state reads receive `ttlLiveStateMs = 5000` (5 s) and `ui://` embedded-asset reads keep `ttlEmbeddedAssetMs = 3600000` (1 h), both with `cacheScope: "private"`. `docs/architecture/mcp/overview.md` is updated in the same change.

### Choices for Thomas
1. **Drift command surface:** Single `show system drift` with `| subsystem <name>` (recommended — follows "the pipe wins" and full-word naming, backed by `ze-plugin-callback:drift-check` so internal and forked plugins behave identically) vs separate `show <subsystem> drift` commands per subsystem.
2. **First-wave drift checkers:** Ship real read-only checkers for `firewall`, `fib-kernel`, and `interface` now, leaving `vpp`, `ipsec`, and `mpls` to adopt `OnDriftCheck` / `drift.Register` as follow-up work (recommended — avoids stub checkers).
3. **MCP `ze://triage` cache TTL departure (`docs/architecture/mcp/overview.md`):** Keep `stampCacheHints` as the sole hint site and add `ttlLiveStateMs = 5000` (5 s) for `ze://` `resources/read` responses while `ui://` assets keep `3600000` ms (1 h).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `internal/core/supportreg` keeps `plugin/server` and `support` decoupled with zero component-tier import between them | `internal/core/supportreg` imports stdlib only; both `support` and `plugin/server` import only core leaves (`core-design.md` Section 19) | Adjust imports if any tier rule rejects | `go test` + `./le arch tier check` | confirmed |
| A-2 | `firewall.LastApplied()` contains full `Table`/`Chain`/`Term`/`Set` definitions needed for read-only packet tracing | Confirmed in `internal/component/firewall/model.go` and `cmd_show.go` | Read from compiled table cache instead | `TestEvaluatePacket_*` + `.ci` | confirmed |
| A-3 | `show bgp rib best \| reason` and `show policy test` already satisfy BGP best-path and policy decision-trace needs | Confirmed in `rib_pipeline_best.go:534-627` and `cmd/policy/handler.go` | Add missing export trace command | Code inspection | confirmed |
| A-4 | MCP `Mcp-Name` header validation accepts `ze://triage` as a valid `params.uri` mirror on `resources/read` | Verified in `internal/component/mcp/headers.go:229-231` | Adjust `headers.go` scheme check | `TestResources_TriageOverHTTP` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A slow or wedged kernel netlink/nftables backend or forked plugin blocks `show system drift` or `show doctor` | Command hangs > 5s | Bounded 5s per-subsystem/per-plugin timeout in `drift.CheckAll` / `plugin/server/drift.go`; `show system triage` does NOT call `drift.CheckAll` directly. |
| R-2 | `badpacket` allocates on BGP happy path, reads a transferred/poisoned buffer, or races with the cancel goroutine | Benchmark alloc regression or `-race` / debug `memguard.Poison` failure | `finishReadBuffer` copies ONLY when `cause != nil && !*kept` (before `returnReadBuffer`); `panicFrame` uses `atomic.Pointer[[]byte]`; cancel goroutine uses `describeCancel`; fixed 16 x 4 KiB ring (64 KiB max). |
| R-3 | Unauthorized MCP client reads `ze://triage` via `resources/read` (since `ui://` assets are ungated) | RBAC test fails | `resourcesRead` calls `s.checkCommandScope(req.ID, scope.Identity, "show system triage")` before dispatching. |
| R-4 | `request support archive` daemon-routed archive exceeds `command.BlobRemoteOctetsMax` with new live modules | Store refusal on daemon route | All live modules are strictly bounded (`log-ring` <= 200 entries, `event-ring` <= 200 entries, `goroutines` <= 64 KiB, `drift` <= 64 findings/subsystem, `bad-packets` <= 16 frames). |
| R-5 | Existing `TestResources_ListWalksEmbeddedFS` or `TestCacheableResultsCarryHints` fails when `ze://triage` is added | `internal/component/mcp` unit tests fail | Keep `listResources()` walking embedded `ui://` only; build `cachedResources` once in `NewStreamable` appending `ze://triage` when `cfg.Dispatch != nil`; keep `cacheTTLByMethod` untouched and handle `ze://` inside `stampCacheHints`. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Confined to new diagnostic commands (`show system drift`, `show system bad-packets`, `show firewall trace`), `ze-plugin-callback:drift-check`, `show doctor` runtime check, `request support archive` live modules, and MCP `ze://triage` read path. No packet forwarding or BGP protocol happy path is altered. |
| How is it reverted? | Single commit revert; every new package, RPC, and command is additive. |
| Who else touches this path? | `firewall`, `fib-kernel`, `iface`, `bgp/reactor`, `taskguard`, `support`, `plugin/server`, `pkg/plugin/{rpc,sdk}`, `cmd/show`, `mcp`. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show system drift` and `show system drift \| subsystem firewall` | → | `ze-cmd:show-system-drift` → `drift.CheckAll` → `ze-plugin-callback:drift-check` | `test/plugin/diag-drift-explain-and-support.ci` |
| Forked plugin `ze-plugin-callback:drift-check` over `MuxConn` | → | `plugin/server/drift.go` → `sdk.OnDriftCheck` | `internal/component/plugin/server/drift_test.go` (`TestDriftCheckOverMuxConn`) |
| `show firewall trace table <t> ...` | → | `ze-firewall:show-trace` → `firewall.EvaluatePacket` | `test/plugin/diag-drift-explain-and-support.ci` |
| `show system bad-packets` and `\| protocol bgp` | → | `ze-cmd:show-system-bad-packets` → `badpacket.Snapshot` | `test/plugin/diag-drift-explain-and-support.ci` |
| `request support archive module log-ring,health,watchdog,bad-packets,drift,event-ring,plugins,reload-history` | → | `support.Answer` → `internal/core/supportreg` live modules | `test/plugin/diag-drift-explain-and-support.ci` |
| MCP POST `resources/list` + `resources/read` (`ze://triage`) | → | `Streamable.resourcesList` / `resourcesRead` → `show system triage` → `stampCacheHints` | `internal/component/mcp/resources_test.go` (`TestResources_TriageOverHTTP`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show system drift` on a running daemon (internal or forked dataplane plugins) | Returns structured JSON with declared subsystems (`firewall`, `fib-kernel`, `interface`), per-subsystem `status`, `checked`, `drifted`, and `findings` (non-nil array). |
| AC-2 | `show system drift \| subsystem firewall` (and unknown subsystem) | Valid subsystem returns only that subsystem's report; unknown subsystem returns an `opererr` naming registered subsystems. |
| AC-3 | A drift checker reports `StatusDrifted` or `StatusError` | `show doctor` (`DoctorPhaseRuntime` check `dataplane-drift`) returns a `Diagnostic` with `Code: doctor-dataplane-drift`, `Severity: SeverityWarn`, and `Fix: "ze show system drift"`. |
| AC-4 | `show firewall trace table <t> protocol tcp source-address <ip> destination-address <ip> destination-port <p>` | Evaluates `LastApplied()` in exact term order (including named sets, jumps/gotos, and chain default policy), returning structured `verdict`, `matched-chain`, `matched-term`, `default-policy`, and per-term `steps` without mutating nftables. |
| AC-5 | Firewall term contains an offline-unevaluatable match predicate (e.g. `MatchDestinationLocal`) | Step reports `unevaluated: true` with explicit reason and `incomplete: true` on the trace result rather than silently fabricating match/no-match. |
| AC-6 | BGP receives an RFC 7606 malformed UPDATE, header error (standard or coalesced read path), or suffers a `taskguard` panic on an unowned (`!*kept`) read buffer | `badpacket.Record` stores the frame (capped at 4096 bytes, 16-slot ring) without ever reading a transferred/poisoned buffer or racing with the cancel goroutine; `show system bad-packets` and `\| protocol bgp` return it newest-first with lowercase `hex`; `show system triage` `runtime.bad-packet-count` increments. |
| AC-7 | `request support archive` (local and daemon-routed) | `internal/core/supportreg` powers the registry; all 20 existing modules plus `log-ring`, `report`, `health`, `watchdog`, `bad-packets`, `drift`, `goroutines` (and daemon-registered `event-ring`, `plugins`, `reload-history`) are selectable and collected within bounded sizes. |
| AC-8 | MCP `resources/list` and `resources/read` for `ze://triage` | Listed alongside `ui://` assets when `Dispatch != nil`; `resources/read` enforces caller scope (`checkCommandScope`), returns `show system triage` JSON with single-site `stampCacheHints` stamping `ttlMs: 5000` and `cacheScope: "private"`; `ui://` assets keep `ttlMs: 3600000`. |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | AI checks for silent dataplane drift after a routing/firewall anomaly | `show system drift` / `show doctor` → `drift.CheckAll` → `ze-plugin-callback:drift-check` | `test/plugin/diag-drift-explain-and-support.ci` + `TestDriftCheckOverMuxConn` |
| 2 | AI traces why a TCP packet is dropped by firewall rules | `show firewall trace` → `firewall.EvaluatePacket` → step-by-step verdict | `test/plugin/diag-drift-explain-and-support.ci` |
| 3 | AI pulls exact offending wire bytes after a BGP malformed UPDATE or panic | BGP RFC 7606 / header error / `finishReadBuffer` panic capture → `badpacket.Record` → `show system bad-packets \| protocol bgp` | `test/plugin/diag-drift-explain-and-support.ci` + reactor `-race` unit tests |
| 4 | AI collects a bounded live support bundle with runtime rings | `request support archive module log-ring,plugins,reload-history,bad-packets,drift` | `test/plugin/diag-drift-explain-and-support.ci` |
| 5 | MCP client discovers and reads `ze://triage` as a standard MCP resource | `resources/list` + `resources/read` (`ze://triage`) → scope check → `show system triage` → `stampCacheHints` | `TestResources_TriageOverHTTP` |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDriftRegisterUnregisterAndCheckAll` & `TestDriftDoctorCheck` | `internal/core/drift/drift_test.go` | AC-1, AC-2, AC-3 | |
| `TestOnDriftCheckCallback` | `pkg/plugin/sdk/sdk_callbacks_test.go` | AC-1 | |
| `TestDriftCheckOverMuxConn` & `TestServerRegistersLiveSupportModules` | `internal/component/plugin/server/{drift_test.go,support_modules_test.go}` | AC-1, AC-7 | |
| `TestFirewallDriftCheck` | `internal/component/firewall/drift_test.go` | AC-1 | |
| `TestEvaluatePacket_*` (match, set, jump/goto, default policy, unevaluated, depth bound, invalid inputs) | `internal/component/firewall/explain_test.go` | AC-4, AC-5 | |
| `TestFibKernelDriftCheck` | `internal/plugins/fib/kernel/drift_test.go` | AC-1 | |
| `TestIfaceDriftCheck` | `internal/component/iface/drift_test.go` | AC-1 | |
| `TestBadPacketRingBoundsAndFilter` | `internal/core/badpacket/badpacket_test.go` | AC-6 | |
| `TestTaskGuardRecordsBadPacketRawFrame` | `internal/core/taskguard/guard_test.go` | AC-6 | |
| `TestSessionBadPacketCapture` (RFC 7606, standard & coalesced header errors, panic unwind owned vs transferred buffer) | `internal/component/bgp/reactor/session_badpacket_test.go` | AC-6 | |
| `TestShowSystemDriftAndBadPackets` | `internal/component/cmd/show/drift_bad_packets_test.go` | AC-1, AC-2, AC-6 | |
| `TestSupportRegRegisterAndUnregister` | `internal/core/supportreg/registry_test.go` | AC-7 | |
| `TestSupportRuntimeCollectors` | `internal/component/support/runtime_collectors_test.go` | AC-7 | |
| `TestResources_TriageOverHTTP`, `TestResources_TriageScopeDenied`, `TestCacheHintsLiveStateVsEmbedded` | `internal/component/mcp/{resources_test.go,caching_test.go}` | AC-8 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `show system bad-packets count` | `1..16` | `1`, `16` | `0`, `-1` | `17` |
| `badpacket` payload bytes | `0..4096` | `4096` | - | `4097` (truncated, `Truncated: true`) |
| `badpacket` ring capacity | `16` slots | `16` | - | `17` (oldest evicted) |
| `show firewall trace` port (`source-port`, `destination-port`) | `1..65535` | `1`, `65535` | `0` | `65536` |
| `firewall.EvaluatePacket` jump/goto depth | `0..16` | `16` | - | `17` (loop guard stops cleanly) |
| `drift.MaxFindingsPerSubsystem` | `0..64` | `64` | - | `65` (truncated, `Truncated: true`) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `diag-drift-explain-and-support` | `test/plugin/diag-drift-explain-and-support.ci` | Live daemon `show system drift`, `show firewall trace`, `show system bad-packets`, `request support archive` live modules, and `show system triage` | |

## Files to Modify

- `internal/core/taskguard/guard.go` -- optional `RawFrame []byte` on `Context`, record to `badpacket` on panic
- `internal/core/ipc/yang/ze-plugin-callback.yang` -- declare `rpc drift-check`
- `pkg/plugin/rpc/types.go` -- add `DriftSubsystems` on `DeclareRegistrationInput` and `DriftFinding`/`DriftCheckInput`/`DriftCheckOutput`
- `pkg/plugin/sdk/sdk_dispatch.go` -- dispatch `ze-plugin-callback:drift-check`
- `pkg/plugin/sdk/sdk_callbacks.go` -- add `OnDriftCheck` callback registration
- `pkg/plugin/sdk/sdk_callbacks_test.go` -- test `OnDriftCheck` over `MuxConn`
- `internal/component/plugin/server/server.go` -- wire drift remote collector and daemon live support modules in `StartWithContext`/`Stop`
- `internal/component/plugin/server/stage1_checks.go` -- validate Stage 1 `drift-subsystems` declarations
- `internal/component/plugin/server/stage1_checks_test.go` -- cover `drift-subsystems` Stage 1 check
- `internal/component/firewall/engine.go` -- declare `DriftSubsystems: []string{"firewall"}` and wire `OnDriftCheck`
- `internal/component/firewall/yang/ze-firewall-cmd.yang` -- add `show firewall trace` command schema
- `internal/component/firewall/yang/self_containment_test.go` -- assert `ze-firewall:show-trace` ownership
- `internal/component/bgp/reactor/session.go` -- add `panicFrame atomic.Pointer[[]byte]` and separate `describeCancel()`
- `internal/component/bgp/reactor/session_read.go` -- record header/length parse errors to `badpacket`; update `finishReadBuffer` to capture owned (`!*kept`) frame on panic unwind
- `internal/component/bgp/reactor/session_coalesce.go` -- record header/length parse errors to `badpacket`; pass `frameLen` into `finishReadBuffer`
- `internal/component/bgp/reactor/session_validation.go` -- record RFC 7606 malformed UPDATEs to `badpacket`
- `internal/plugins/fib/kernel/fibkernel.go` -- declare `DriftSubsystems: []string{"fib-kernel"}` and wire `OnDriftCheck`
- `internal/component/iface/register.go` -- declare `DriftSubsystems: []string{"interface"}` and wire `OnDriftCheck`
- `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` -- add `show system drift` and `show system bad-packets` schemas
- `internal/component/cmd/show/show.go` -- register `ze-cmd:show-system-drift` and `ze-cmd:show-system-bad-packets` RPCs and pipe filters
- `internal/component/cmd/show/triage.go` -- add `bad-packet-count` to `triageRuntime`
- `internal/component/support/modules.go` -- delegate registry to `internal/core/supportreg` and register all 20 static modules in `init()`
- `internal/component/support/support.go` -- consume `supportreg.CollectOptions` and `supportreg.Collector`
- `internal/component/mcp/resources.go` -- add read-only `ze://triage` resource listing helper and scope-checked reading
- `internal/component/mcp/streamable.go` -- build `cachedResources` once in `NewStreamable` appending `ze://triage` when `cfg.Dispatch != nil`
- `internal/component/mcp/caching.go` -- keep `stampCacheHints(req, resp)` as sole hint site; add `ttlLiveStateMs = 5000` for `ze://` reads
- `internal/component/mcp/streamable_tools.go` -- pass `ctx`, `scope`, `remoteAddr` into `s.resourcesRead` and pass `req` into `stampCacheHints`
- `internal/component/mcp/resources_test.go` -- add `ze://triage` list, read, TTL, and RBAC denial tests
- `internal/component/mcp/caching_test.go` -- add live-state `ze://` vs embedded `ui://` TTL verification
- `internal/component/plugin/all/testdata/wire-methods.snapshot` -- add `ze-cmd:show-system-bad-packets`, `ze-cmd:show-system-drift`, `ze-firewall:show-trace`
- `internal/plugins/skills/builtins/ze-diagnostics.md` -- document drift, firewall trace, BGP explain, bad-packets, live support modules, and `ze://triage`
- `docs/architecture/diagnostics/production-diagnostics.md` -- document Part 5 components and commands
- `docs/architecture/diagnostics/crash-capture.md` -- document `taskguard.Context.RawFrame` bad-packet capture
- `docs/architecture/doctor-and-health-checks.md` -- document `doctor-dataplane-drift` and `internal/core/drift`
- `docs/architecture/api/commands.md` -- document new show commands and pipe filters (`| subsystem`, `| protocol`)
- `docs/architecture/api/process-protocol.md` -- document `ze-plugin-callback:drift-check` and Stage 1 `drift-subsystems`
- `docs/architecture/buffer-architecture.md` -- add cold-path bad-packet / panic-unwind copy trigger to "When a copy is deliberate"
- `docs/architecture/mcp/overview.md` -- document `ze://triage` read-only resource and `ttlLiveStateMs = 5000` departure in `Cacheable Results`
- `docs/architecture/core-design.md` -- document `internal/core/supportreg`, `internal/core/drift`, and `firewall.EvaluatePacket`
- `docs/features/interfaces.md` -- note `interface` drift checker (`OnDriftCheck`)
- `docs/guide/command-reference.md` -- document `show system drift`, `show system bad-packets`, `show firewall trace`, and live support modules

## Files to Create

- `internal/core/drift/drift.go` -- read-only dataplane drift types, local registry, and remote collector hook
- `internal/core/drift/register.go` -- `doctor-dataplane-drift` diagnostic code and `DoctorPhaseRuntime` check
- `internal/core/drift/drift_test.go` -- unit tests for drift registry, remote collector merge, and doctor check
- `internal/core/badpacket/badpacket.go` -- 16-slot 4 KiB-capped bad wire packet ring + `CopyBounded`
- `internal/core/badpacket/badpacket_test.go` -- unit tests for ring bounds, truncation, filtering
- `internal/core/supportreg/registry.go` -- pure stdlib core leaf registry for support bundle modules
- `internal/core/supportreg/registry_test.go` -- unit tests for support module registration and unregister safety
- `internal/component/plugin/server/drift.go` -- daemon `drift-check` RPC fan-out across internal and forked plugins
- `internal/component/plugin/server/drift_test.go` -- unit tests for `drift-check` over `MuxConn` and timeout handling
- `internal/component/firewall/drift.go` -- `firewall` read-only `OnDriftCheck` evaluator
- `internal/component/firewall/drift_test.go` -- unit tests for firewall drift checker
- `internal/component/firewall/explain.go` -- pure read-only firewall packet decision-trace evaluator
- `internal/component/firewall/explain_test.go` -- unit tests for firewall trace evaluator
- `internal/component/firewall/cmd_trace.go` -- `ze-firewall:show-trace` handler registration
- `internal/plugins/fib/kernel/drift.go` -- `fib-kernel` read-only `OnDriftCheck` evaluator
- `internal/plugins/fib/kernel/drift_test.go` -- unit tests for fib-kernel drift checker
- `internal/component/iface/drift.go` -- `interface` read-only `OnDriftCheck` evaluator
- `internal/component/iface/drift_test.go` -- unit tests for interface drift checker
- `internal/component/bgp/reactor/session_badpacket_test.go` -- unit tests for BGP RFC 7606, standard & coalesced header errors, and panic-unwind owned vs transferred buffer capture
- `internal/component/cmd/show/drift.go` -- `show system drift` handler
- `internal/component/cmd/show/bad_packets.go` -- `show system bad-packets` handler
- `internal/component/cmd/show/drift_bad_packets_test.go` -- unit tests for `show system drift` and `show system bad-packets`
- `internal/component/support/runtime_collectors.go` -- 7 core live runtime support modules
- `internal/component/support/runtime_collectors_test.go` -- unit tests for runtime collectors
- `internal/component/plugin/server/support_modules.go` -- 3 daemon-backed live support modules (`event-ring`, `plugins`, `reload-history`) registered via `internal/core/supportreg`
- `internal/component/plugin/server/support_modules_test.go` -- unit tests for server support module registration
- `test/plugin/diag-drift-explain-and-support.ci` -- live `.ci` functional test
- `website/changes/fragments/ai-diagnostics-drift-explain-support.md` -- operator news fragment

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG command schema | Yes | `internal/component/cmd/show/yang/ze-cli-show-cmd.yang`, `internal/component/firewall/yang/ze-firewall-cmd.yang` |
| Plugin callback YANG & SDK | Yes | `internal/core/ipc/yang/ze-plugin-callback.yang`, `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/{sdk_dispatch,sdk_callbacks}.go` |
| Wire methods snapshot | Yes | `internal/component/plugin/all/testdata/wire-methods.snapshot` |
| Command-owned pipe filters | Yes | `show system drift` (`\| subsystem`), `show system bad-packets` (`\| protocol`) |
| `show doctor` runtime check | Yes | `internal/core/drift/register.go` (`DoctorPhaseRuntime`, `doctor-dataplane-drift`) |
| `show system triage` | Yes | `internal/component/cmd/show/triage.go` (`runtime.bad-packet-count`) |
| AI diagnostics skill | Yes | `internal/plugins/skills/builtins/ze-diagnostics.md` |
| MCP resources & caching | Yes | `internal/component/mcp/{resources,streamable,caching,streamable_tools}.go` (`ze://triage`) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New operational commands added? | Yes | `docs/guide/command-reference.md`, `docs/architecture/api/commands.md` |
| 2 | New diagnostic architecture components? | Yes | `docs/architecture/diagnostics/production-diagnostics.md`, `docs/architecture/diagnostics/crash-capture.md` |
| 3 | New doctor code / check? | Yes | `docs/architecture/doctor-and-health-checks.md` |
| 4 | New plugin callback RPC & Stage 1 field? | Yes | `docs/architecture/api/process-protocol.md` |
| 5 | New wire-path deliberate copy trigger? | Yes | `docs/architecture/buffer-architecture.md` |
| 6 | MCP resources / caching departure? | Yes | `docs/architecture/mcp/overview.md` |
| 7 | Core registration seams changed? | Yes | `docs/architecture/core-design.md`, `docs/features/interfaces.md` |
| 8 | Operator-facing news fragment needed? | Yes | `website/changes/fragments/ai-diagnostics-drift-explain-support.md` |
| 9 | `docs/architecture/api/ipc_protocol.md` (`// Design:` of `pkg/plugin/rpc/types.go`) affected? | No | Unaffected: width-framed IPC wire framing does not change; `drift-check` uses the existing JSON-RPC callback envelope. |

## Implementation Steps

1. **Phase 1 (Core leaf packages & plugin RPC seam — parallel):**
   - Create `internal/core/drift` (`drift.go`, `register.go`, `drift_test.go`), `internal/core/badpacket` (`badpacket.go`, `badpacket_test.go`), `internal/core/supportreg` (`registry.go`, `registry_test.go`), and wire `taskguard.Context.RawFrame` in `internal/core/taskguard/guard.go`.
   - Add `ze-plugin-callback:drift-check` and `DriftSubsystems` in `internal/core/ipc/yang/ze-plugin-callback.yang`, `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/{sdk_dispatch,sdk_callbacks,sdk_callbacks_test}.go`.
2. **Phase 2 (Dataplane plugin checkers, firewall trace & BGP bad-packet hooks — parallel):**
   - `internal/component/firewall/{drift.go,explain.go,cmd_trace.go,engine.go,yang/ze-firewall-cmd.yang}` + unit tests.
   - `internal/plugins/fib/kernel/{drift.go,fibkernel.go}` + unit tests.
   - `internal/component/iface/{drift.go,register.go}` + unit tests.
   - `internal/component/bgp/reactor/{session.go,session_read.go,session_coalesce.go,session_validation.go,session_badpacket_test.go}`.
3. **Phase 3 (Server wiring, show commands, support collectors & MCP `ze://triage` — parallel):**
   - `internal/component/plugin/server/{server.go,drift.go,drift_test.go,stage1_checks.go,stage1_checks_test.go,support_modules.go,support_modules_test.go}`.
   - `internal/component/cmd/show/{yang/ze-cli-show-cmd.yang,show.go,drift.go,bad_packets.go,triage.go,drift_bad_packets_test.go}` + `wire-methods.snapshot`.
   - `internal/component/support/{modules.go,support.go,runtime_collectors.go,runtime_collectors_test.go}`.
   - `internal/component/mcp/{resources.go,streamable.go,caching.go,streamable_tools.go,resources_test.go,caching_test.go}`.
4. **Phase 4 (Docs, skills, news fragment & `.ci` functional test):**
   - Update all 9 docs, `ze-diagnostics.md` skill, and news fragment.
   - Draft, stress-prove (80 iterations, 64 burners, 16 parallel), and promote `test/plugin/diag-drift-explain-and-support.ci`.
5. **Phase 5 (Verification & `/ze-close` Review Gate):**
   - Run scoped `-race` tests, `plugin/all` snapshots, arch/cli/style/lint/doc checks, `GOOS=darwin go vet`, independent `/ze-close` Review Gate, and land Commit A + Commit B.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Process-boundary symmetry | `firewall`, `fib-kernel`, and `interface` drift checks answer via `ze-plugin-callback:drift-check` identically for internal and forked plugins. |
| Read-only drift checks | No drift checker ever mutates nftables, netlink routes, interfaces, or `report` warnings. |
| Hot-path & lifetime safety | `finishReadBuffer` copies ONLY when `cause != nil && !*kept`; both `session_read.go` and `session_coalesce.go` are wired; cancel goroutine uses `describeCancel`; `panicFrame` uses `atomic.Pointer`. |
| Bounded memory & output | 16-slot x 4 KiB badpacket ring; 64 findings/subsystem drift cap; 5s per-checker timeout; bounded support runtime collectors. |
| MCP RBAC & single-site caching | `ze://triage` checks `checkCommandScope("show system triage")`; `stampCacheHints` is the sole hint site (`5000` ms for `ze://`, `3600000` ms for `ui://`). |
| Firewall trace honesty | Unevaluatable kernel-only predicates set `unevaluated: true` and `incomplete: true` instead of false match/no-match. |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `internal/core/drift` + `ze-plugin-callback:drift-check` + `show system drift` + `show doctor` check | Unit tests (`-race`, incl. `MuxConn`) + `test/plugin/diag-drift-explain-and-support.ci` |
| `firewall.EvaluatePacket` + `show firewall trace` | Unit tests (`-race`) + `test/plugin/diag-drift-explain-and-support.ci` |
| `internal/core/badpacket` + standard/coalesced/panic hooks + `show system bad-packets` | Unit tests (`-race`) + `test/plugin/diag-drift-explain-and-support.ci` |
| `internal/core/supportreg` + 10 live runtime modules | Unit tests (`-race`) + `test/plugin/diag-drift-explain-and-support.ci` |
| MCP `ze://triage` resource & live-state caching | `internal/component/mcp` HTTP unit tests (`-race`) |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| MCP resource authorization | `ze://triage` MUST NOT bypass `checkCommandScope`; unauthorized caller receives scope denial. |
| Support bundle redaction | Live runtime modules (`log-ring`, `event-ring`, `report`, `bad-packets`) carry no config secrets; `config` module redaction remains untouched. |
| Memory exhaustion / DoS | Flood of malformed BGP frames cannot grow `badpacket` past 64 KiB; cyclic firewall jumps cannot stack-overflow (`maxTraceDepth = 16`); Stage 1 `drift-subsystems` declarations are length/charset/count bounded. |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Stage 1 conformance test failure | Align `stage1_checks.go` and `stage1_checks_test.go` |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `ze-plugin-callback:drift-check` (`sdk.OnDriftCheck`) + `internal/core/drift` remote collector hook | Process-local `drift.Register` called from SDK plugins | Plugins can run internal or forked (`process separate`); routing through the standard callback RPC works identically across both transports. |
| Single `show system drift` + `\| subsystem <name>` | Per-subsystem `show <subsystem> drift` commands | Follows "the pipe wins" (`docs/architecture/api/commands.md`), full-word pipe naming, and automatically surfaces every plugin declaring `DriftSubsystems` with one uniform JSON schema. |
| First-wave drift checkers for `firewall`, `fib-kernel`, `interface` | Stubbing `vpp`, `ipsec`, `mpls` | Only ship real read-only checkers where both desired snapshot and live backend readback exist; optional engines can declare `DriftSubsystems` when their readback seams are ready. |
| Preserve existing BGP explain (`show bgp rib best \| reason`, `show policy test`) and build `show firewall trace` | Adding redundant `show bgp explain` command | Avoids duplicating production evaluators already shipped in `rib` and `cmd/policy`; fills the real gap in firewall rule evaluation. |
| Capture BGP panic frames inside `finishReadBuffer` when `!*kept` into `atomic.Pointer` `panicFrame` + separate `describeCancel` | Holding a borrowed slice across `returnReadBuffer` or reading in `describeRead` from cancel goroutine | Strictly preserves `memory/lifetime-contracts.md` (never reads a transferred or `memguard.Poison`ed buffer), covers both `session_read.go` and default `session_coalesce.go`, and is 100% `-race` clean. |
| `internal/core/supportreg` leaf with daemon modules registered from `plugin/server` `StartWithContext`/`Stop` | Importing `support` from `plugin/server` or keeping hardcoded map | Strictly honors `docs/architecture/core-design.md` Section 19 (`plugin/server` imports only `aaa, audit` among components) while keeping `support` offline-safe. |
| Single-site `stampCacheHints` stamping `ttlLiveStateMs = 5000` for `ze://` reads (`departs` from `mcp/overview.md` embedded-only `resources/read`) | Handler-set `ttlMs` or 1 h TTL on live triage | Preserves single-site cache-hint enforcement and MCP 2026-07-28 conformance while preventing 1 h caching of live triage state. |

## Known Limitations

- Optional/conditional engines (`vpp`, `ipsec`/XFRM, `mpls`) do not ship drift checkers in this first wave; each can declare `DriftSubsystems` + `OnDriftCheck` (or `drift.Register`) in its own spec once its live readback API is wired.
- `show firewall trace` evaluates the abstract `firewall.Table` model offline; kernel-runtime-only predicates (`destination-local`, `ipsec-transform`, `unicast`, `tcp-flags`, `icmp-type`, `icmpv6-type`, `connection-mark`, TTL/hop-limit, ARP/ND) are reported as `unevaluated: true` rather than guessed.
- `ze://triage` is read-only on demand (`resources/list` + `resources/read`); `subscriptions/listen` and `notifications/resources/updated` remain owned by `plan/post-release/features/spec-mcp2026-5-state-resources.md`.

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-8 all demonstrated
- [x] Every user story has a working path and a passing test
- [x] Wiring Test table complete: every row a concrete test name, none deferred
- [x] Feature code integrated, not library-only
- [x] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [x] Architectural Verification table filled
- [x] Critical Review passes
- [x] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [x] Unit tests written and passing (`-race`)
- [x] Boundary tests for all numeric inputs
- [x] Functional `.ci` test proven under 80-iteration `stress-repro` and promoted

### Closure
- [x] `/ze-close` Review Gate clean, recorded via `./le spec review record`
- [ ] **Commit A:** code + tests + docs + news fragment
- [ ] **Commit B:** remove `plan/post-release/features/spec-ai-diagnostics-5-drift-explain-and-support-registry.md`

---

## Implementation Summary

### What Was Implemented
- **Pillar 1 (`internal/core/drift`, `ze-plugin-callback:drift-check`, `show system drift`, `show doctor`):**
  - Core leaf package `internal/core/drift/{drift.go,register.go}` with `Report`/`Finding`/`Status`/`Kind`, `MaxFindingsPerSubsystem = 64`, `5s` timeout, local `Register`, daemon `RegisterRemoteCollector`/`RegisterRemoteLister`, and `DoctorPhaseRuntime` check `dataplane-drift` (`Order: 860`, `CodeDoctorDataplaneDrift = "doctor-dataplane-drift"`, fix `ze show system drift`).
  - Generic plugin callback `ze-plugin-callback:drift-check` (`internal/core/ipc/yang/ze-plugin-callback.yang`, `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/{sdk_dispatch.go,sdk_callbacks.go}`, `internal/component/plugin/ipc/rpc.go`), Stage 1 `drift-subsystems` validation (`internal/component/plugin/server/stage1_checks.go`) and atomic cross-plugin conflict-free registration/unregistration (`internal/component/plugin/server/{drift.go,startup.go,restart.go,server.go}`).
  - First-wave read-only `OnDriftCheck` checkers in `firewall` (`internal/component/firewall/{drift.go,engine.go}`), `fib-kernel` (`internal/plugins/fib/kernel/{drift.go,register.go}`), and `interface` (`internal/component/iface/{drift.go,register.go}`).
  - `show system drift` (`ze-cmd:show-system-drift`) with command-owned pipe filter `| subsystem <name>` (`internal/component/cmd/show/{drift.go,show.go,yang/ze-cli-show-cmd.yang}`).
- **Pillar 2 (`firewall.EvaluatePacket` & `show firewall trace`):**
  - Backend-independent offline read-only packet decision-trace evaluator in `internal/component/firewall/explain.go` over `firewall.LastApplied()`, handling CIDR/port/interface/named-set/connstate/DSCP/mark predicates, `Jump`/`Goto`/`Return` with `maxTraceDepth = 16`, chain default policy, and honest `unevaluated: true` + `incomplete: true` for kernel-only predicates.
  - `show firewall trace` (`ze-firewall:show-trace`, no `ze:backend` restriction) in `internal/component/firewall/{cmd_trace.go,register.go,yang/ze-firewall-cmd.yang}`. Preserved existing BGP `show bgp rib best | reason` and `show policy test`.
- **Pillar 3 (`internal/core/badpacket`, cold-path hooks, `show system bad-packets`, triage counter):**
  - 16-slot x 4 KiB circular ring (`internal/core/badpacket/badpacket.go`, 64 KiB max total memory, `CopyBounded`, newest-first `Snapshot`, atomic `Count`).
  - Cold-path capture in BGP RFC 7606 (`session_validation.go`), standard (`session_read.go`) and default coalesced (`session_coalesce.go`) header/length errors, and `finishReadBuffer` owned-buffer (`!*kept`) panic unwind via `atomic.Pointer[[]byte]` `panicFrame`, `panicFrameSuppressed atomic.Bool`, separate `describeCancel()`, and `taskguard.Context.RawFrame` (`internal/core/taskguard/guard.go`).
  - `show system bad-packets [count <1..16>]` (`ze-cmd:show-system-bad-packets`) with command-owned `| protocol <name>` (`internal/component/cmd/show/bad_packets.go`) and `runtime.bad-packet-count` in `show system triage` (`internal/component/cmd/show/triage.go`).
- **Pillar 4 (`internal/core/supportreg`, live runtime support modules, MCP `ze://triage`):**
  - Pure stdlib core leaf registry `internal/core/supportreg/registry.go` replacing hardcoded `support.moduleRegistry`; 20 static modules + 7 core live modules (`log-ring`, `report`, `health`, `watchdog`, `bad-packets`, `drift`, `goroutines`) registered in `internal/component/support/{register.go,modules.go,runtime_collectors.go}`, plus 3 daemon live modules (`event-ring`, `plugins`, `reload-history`) registered/unregistered by `internal/component/plugin/server/{support_modules.go,server.go}` without importing `support`.
  - Read-only MCP `ze://triage` listed in `Streamable.cachedResources` when `Dispatch != nil`, scope-checked via `checkCommandScope("show system triage")`, and stamped solely by `stampCacheHints` with `ttlLiveStateMs = 5000` ms (`cacheScope: "private"`), keeping `ui://` at `3600000` ms (`internal/component/mcp/{resources.go,streamable.go,caching.go,streamable_tools.go}`).

### Bugs Found/Fixed
- **`firewall/explain.go` `Goto` fallthrough bug:** `case ActionGoto` returned `evalChain(...)` directly so `outcomeFallthrough` resumed caller's remaining terms; fixed to convert `outcomeFallthrough` to `outcomeReturn` (`TestEvaluatePacketJumpAndGoto`).
- **Review Gate Finding 1 (Major):** `refuseSinkChecks` wrote `driftSubsystems` before Stage 1 finished and skipped cross-plugin conflict refusal; replaced with atomic `registerPluginDriftSubsystems` / `unregisterPluginDriftSubsystems` inside `onRegistration` under `startupRegistrationMu` with full rollback and cleanup in `releasePluginRegistrations` (`TestDriftSubsystemsCrossPluginConflictAndRestart`).
- **Review Gate Finding 2 (Medium):** Outer `readAndProcessCoalesced` `finishReadBuffer` could misattribute Packet 2 if inner `flushCoalesce` panicked with `transferredReadBufferPanic`; added `panicFrameSuppressed atomic.Bool` set on transferred panic, checked in `capturePanicFrame`, cleared in `describeRead` (`TestSessionBadPacketPanicCapture/coalesced-flush-transferred-panic-does-not-misattribute-outer-frame`).
- **Operator error inventory & lint fixes:** Wrapped MCP `resources.go` `s.fail` messages with `opererr` fixes, moved new `init()` calls to `register.go` files (`firewall/register.go`, `support/register.go`), and resolved 4 `golangci-lint` findings (`cmd_trace.go`, `explain.go`, `caching.go`, `supportreg/registry_test.go`).

### Documentation Updates
- `docs/architecture/diagnostics/production-diagnostics.md` -- Part 5 drift, firewall trace, bad-packet ring, supportreg live modules, `ze://triage` + source anchors.
- `docs/architecture/diagnostics/crash-capture.md` -- `taskguard.Context.RawFrame` + `badpacket.Record` on panic unwind + source anchors.
- `docs/architecture/doctor-and-health-checks.md` -- `doctor-dataplane-drift` (`DoctorPhaseRuntime`, `Order: 860`) + source anchors.
- `docs/architecture/api/commands.md` -- `show system drift | subsystem <name>` and `show system bad-packets | protocol <name>` + source anchors.
- `docs/architecture/api/process-protocol.md` -- Stage 1 `drift-subsystems` and `ze-plugin-callback:drift-check` (`registerPluginDriftSubsystems`, `SendDriftCheck`, `OnDriftCheck`) + source anchors.
- `docs/architecture/buffer-architecture.md` -- deliberate cold-path copy row for malformed wire frames & owned-buffer panic-unwind capture.
- `docs/architecture/mcp/overview.md` -- `ze://triage` read-only resource and `ttlLiveStateMs = 5000` departure in `Cacheable Results` + source anchors.
- `docs/architecture/core-design.md` -- `firewall.EvaluatePacket`, `internal/core/supportreg`, and `internal/core/drift` leaf boundaries + source anchors.
- `docs/features/interfaces.md` -- `interface` read-only `OnDriftCheck` checker + source anchor.
- `docs/guide/command-reference.md` -- `show system drift`, `show system bad-packets`, `show firewall trace`, and 10 live `request support archive` modules + source anchors.
- `internal/plugins/skills/data/ze-diagnostics.md` -- updated AI diagnostics workflow for drift, firewall trace, BGP explain, bad-packets, live support modules, and `ze://triage`.
- `website/changes/fragments/ai-diagnostics-5-drift-explain-and-support-registry.md` -- operator news fragment.
- `./le doc check all`: 0 errors on all Spec 5 docs, anchors, and YANG nodes (the sole reported drift item is an unrelated concurrent session's `ze-meta:bgp-plugin-format` wiki catalog entry).

### Deviations from Plan
- Skill file path is `internal/plugins/skills/data/ze-diagnostics.md` (actual repo directory is `data/`, not `builtins/`).
- `fib-kernel` wires `DriftSubsystems` and `OnDriftCheck` in `internal/plugins/fib/kernel/register.go` (where its SDK `Registration` is built) rather than `fibkernel.go`.
- `internal/component/support/support.go` needed no edits because `internal/component/support/modules.go` adapts `supportreg.CollectOptions` / `supportreg.Collector` directly; `internal/component/support/register.go` holds the `init()` registration to satisfy the `go style` rule requiring `init()` in `register*.go`.
- `internal/component/plugin/server/{startup.go,restart.go}` were updated per Review Gate Finding 1 for transactional Stage 1 conflict refusal and restart cleanup.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | First `refuseSinkChecks` recorded `driftSubsystems` before Stage 1 completed without checking cross-plugin duplicates | Stage 1 validation in `refuseSinkChecks` must stay read-only and cross-plugin ownership must be claimed transactionally inside `onRegistration` under `startupRegistrationMu` | Independent Review Gate Round 1 | Moved to atomic `registerPluginDriftSubsystems` in `startup.go` + `unregisterPluginDriftSubsystems` in `restart.go` with regression test `TestDriftSubsystemsCrossPluginConflictAndRestart` |
| approach | Nested `readAndProcessCoalesced` defer could capture outer Packet 2 if inner `flushCoalesce` panicked with `transferredReadBufferPanic` | Once an inner `finishReadBuffer` observes a transferred-buffer panic, outer defers must suppress frame capture while still recycling their own buffers | Independent Review Gate Round 1 | Added `panicFrameSuppressed atomic.Bool` in `session.go`/`session_read.go` with regression test `coalesced-flush-transferred-panic-does-not-misattribute-outer-frame` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Core leaf drift registry + `ze-plugin-callback:drift-check` + `firewall`/`fib-kernel`/`interface` checkers + `show system drift` + `show doctor` check | Done | `internal/core/drift/{drift.go,register.go}`, `pkg/plugin/{rpc/types.go,sdk/sdk_callbacks.go}`, `internal/component/plugin/server/drift.go`, `internal/component/{firewall,iface}/drift.go`, `internal/plugins/fib/kernel/drift.go`, `internal/component/cmd/show/drift.go` | Symmetric across `DirectBridge` and `MuxConn`; read-only |
| Firewall read-only `EvaluatePacket` + `show firewall trace`; BGP explain preserved | Done | `internal/component/firewall/{explain.go,cmd_trace.go,register.go,yang/ze-firewall-cmd.yang}` | Backend-independent across kernel & VPP; honest `unevaluated`/`incomplete` |
| Always-on 16x4KiB `badpacket` ring + BGP RFC 7606 / standard & coalesced header / owned-buffer panic hooks + `show system bad-packets` + triage count | Done | `internal/core/badpacket/badpacket.go`, `internal/core/taskguard/guard.go`, `internal/component/bgp/reactor/{session.go,session_read.go,session_coalesce.go,session_validation.go}`, `internal/component/cmd/show/{bad_packets.go,triage.go}` | Zero happy-path allocations; `ZE_MEMGUARD=poison` + `-race` clean |
| `internal/core/supportreg` leaf + 20 static + 7 core + 3 daemon live modules + MCP `ze://triage` with single-site 5s TTL | Done | `internal/core/supportreg/registry.go`, `internal/component/support/{register.go,modules.go,runtime_collectors.go}`, `internal/component/plugin/server/support_modules.go`, `internal/component/mcp/{resources.go,streamable.go,caching.go,streamable_tools.go}` | `plugin/server` never imports `support`; `checkCommandScope` enforced |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestDriftRegisterUnregisterAndCheckAll`, `TestDriftCheckOverMuxConn`, `TestDriftCheckOverDirectBridgeAndClosedConn`, `TestDriftSubsystemsCrossPluginConflictAndRestart`, `TestFirewallDriftCheck`, `TestFibKernelDriftCheck`, `TestIfaceDriftCheck`, `test/plugin/diag-drift-explain-and-support.ci` | Internal and forked plugins verified |
| AC-2 | Done | `TestShowSystemDrift`, `test/plugin/diag-drift-explain-and-support.ci` | Valid `\| subsystem` narrows; unknown returns `opererr` |
| AC-3 | Done | `TestDriftDoctorCheck`, `test/plugin/diag-drift-explain-and-support.ci` | `doctor-dataplane-drift` (`Order: 860`, fix `ze show system drift`) |
| AC-4 | Done | `TestEvaluatePacket*`, `TestHandleShowFirewallTrace`, `test/plugin/diag-drift-explain-and-support.ci` | Term order, named sets, jump/goto, default policy verified |
| AC-5 | Done | `TestEvaluatePacketUnevaluatedAndChainSelection`, `TestEvaluatePacketNamedSets` | `unevaluated: true` and `incomplete: true` set on kernel-only predicates |
| AC-6 | Done | `TestBadPacketRingBoundsAndFilter`, `TestTaskGuardRecordsBadPacketRawFrame`, `TestSessionBadPacketRFC7606Actions`, `TestSessionBadPacketHeaderErrors`, `TestSessionBadPacketPanicCapture`, `TestShowSystemBadPackets`, `TestShowSystemTriageBadPacketCount`, `test/plugin/diag-drift-explain-and-support.ci` | Includes `finishReadBuffer-happy-path-zero-allocs` and coalesced transferred-panic regression |
| AC-7 | Done | `TestSupportReg*`, `TestSupportRuntimeCollectors*`, `TestSupportModulesLifecycleAndPayloads`, `test/plugin/diag-drift-explain-and-support.ci` | Proves both local (27 modules) and daemon MCP (30 modules) routes |
| AC-8 | Done | `TestResources_TriageOverHTTP`, `TestResources_TriageScopeDenied`, `TestCacheHintsLiveStateVsEmbedded`, `test/plugin/diag-drift-explain-and-support.ci` | Live MCP `ze://triage` returns `ttlMs: 5000`, `cacheScope: "private"`; `ui://` keeps `3600000` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestDriftRegisterUnregisterAndCheckAll` & `TestDriftDoctorCheck` | Done | `internal/core/drift/drift_test.go` | PASS `-race` |
| `TestOnDriftCheckCallback` | Done | `pkg/plugin/sdk/sdk_callbacks_test.go` | PASS `-race` |
| `TestDriftCheckOverMuxConn`, `TestDriftSubsystemsCrossPluginConflictAndRestart`, `TestSupportModulesLifecycleAndPayloads` | Done | `internal/component/plugin/server/{drift_test.go,support_modules_test.go}` | PASS |
| `TestFirewallDriftCheck` & `TestEvaluatePacket*` | Done | `internal/component/firewall/{drift_test.go,explain_test.go}` | PASS `-race` |
| `TestFibKernelDriftCheck` & `TestIfaceDriftCheck` | Done | `internal/plugins/fib/kernel/drift_test.go`, `internal/component/iface/drift_test.go` | PASS `-race` |
| `TestBadPacketRingBoundsAndFilter`, `TestTaskGuardRecordsBadPacketRawFrame`, `TestSessionBadPacket*` | Done | `internal/core/badpacket/badpacket_test.go`, `internal/core/taskguard/guard_test.go`, `internal/component/bgp/reactor/session_badpacket_test.go` | PASS `-race` + `ZE_MEMGUARD=poison` |
| `TestShowSystemDrift` & `TestShowSystemBadPackets` | Done | `internal/component/cmd/show/drift_bad_packets_test.go` | PASS `-race` |
| `TestSupportReg*` & `TestSupportRuntimeCollectors*` | Done | `internal/core/supportreg/registry_test.go`, `internal/component/support/runtime_collectors_test.go` | PASS `-race` |
| `TestResources_TriageOverHTTP`, `TestResources_TriageScopeDenied`, `TestCacheHintsLiveStateVsEmbedded` | Done | `internal/component/mcp/{resources_test.go,caching_test.go}` | PASS `-race` |
| `diag-drift-explain-and-support.ci` | Done | `test/plugin/diag-drift-explain-and-support.ci` | 80/80 `stress-repro` PASS (`burners=64, parallel=16`) + in-place PASS (`17.7s`) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| All 44 whole modified files + 4 shared `hunks` files + 29 created files (77 total) | Done | Pin-checked in `tmp/review/ai-diagnostics-5-drift-explain-and-support-registry-f1a36472-8747-465b-8316-df184028508b.md`; see Deviations for `skills/data/`, `fib/kernel/register.go`, `support/register.go` |

### Audit Summary
- **Total items:** 4 pillars, 8 ACs, 14 unit/functional test groups, 77 files
- **Done:** All (100%)
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4 minor path/wiring adjustments recorded in Deviations from Plan

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Detect silent intent-vs-actual dataplane drift across internal & forked plugins via `show system drift` and `show doctor` | Unit + live `.ci` functional | `TestDriftCheckOverMuxConn`, `TestDriftCheckOverDirectBridgeAndClosedConn`, `TestDriftSubsystemsCrossPluginConflictAndRestart`, `TestDriftDoctorCheck`, `test/plugin/diag-drift-explain-and-support.ci` (80/80 stress-repro PASS + 17.7s live pass) |
| Trace firewall rule decisions read-only without mutating kernel/VPP | Unit + live `.ci` functional | `TestEvaluatePacket*`, `TestHandleShowFirewallTrace`, `test/plugin/diag-drift-explain-and-support.ci` (`show firewall trace`) |
| Capture malformed BGP wire frames & owned-buffer panic frames with zero happy-path allocations | Unit (`-race`, `ZE_MEMGUARD=poison`, `AllocsPerRun`) + live `.ci` functional | `TestBadPacketRingBoundsAndFilter`, `TestTaskGuardRecordsBadPacketRawFrame`, `TestSessionBadPacket*` (`finishReadBuffer-happy-path-zero-allocs` = 0.0 allocs), `test/plugin/diag-drift-explain-and-support.ci` |
| Replace hardcoded support bundle registry with `internal/core/supportreg` + 10 bounded live modules and expose MCP `ze://triage` with 5s TTL | Unit + live `.ci` functional | `TestSupportReg*`, `TestSupportRuntimeCollectors*`, `TestSupportModulesLifecycleAndPayloads`, `TestResources_TriageOverHTTP`, `TestResources_TriageScopeDenied`, `TestCacheHintsLiveStateVsEmbedded`, `test/plugin/diag-drift-explain-and-support.ci` (local 27-module + daemon MCP 30-module + live `ze://triage` `ttlMs: 5000`) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None — all in-scope deliverables completed | Optional engine checkers (`vpp`, `ipsec`, `mpls`) and MCP `ze://` subscriptions were explicitly out of scope in the approved design | `plan/post-release/features/spec-mcp2026-5-state-resources.md` (for `ze://` subscriptions) |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/ai-diagnostics-5-drift-explain-and-support-registry-f1a36472-8747-465b-8316-df184028508b.md` |
| `./le spec review check` | clean (77 files verified) |
| Rounds | 2 (Round 1 surfaced 2 product defects fixed before Round 2 CLEAN) |
| Reviewer lenses used | Independent Review Gate (`5b9285b4-d376-4a33-bb39-3ae9f738a6f4`): logic+wiring, security+RBAC, memory-lifetime/concurrency, spec completeness, style+design-docs |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | Major | `refuseSinkChecks` prematurely recorded `driftSubsystems` before Stage 1 succeeded and did not reject cross-plugin duplicate subsystem claims | `internal/component/plugin/server/{stage1_checks.go,drift.go,startup.go,restart.go}` | Moved to atomic `registerPluginDriftSubsystems` in `onRegistration` under `startupRegistrationMu` with full rollback + `unregisterPluginDriftSubsystems` in `releasePluginRegistrations`, covered by `TestDriftSubsystemsCrossPluginConflictAndRestart` |
| 2 | Medium | Outer `readAndProcessCoalesced` defer could capture unrelated Packet 2 when inner `flushCoalesce` panicked with `transferredReadBufferPanic` | `internal/component/bgp/reactor/{session.go,session_read.go}` | Added `panicFrameSuppressed atomic.Bool` set on transferred panic, checked in `capturePanicFrame`, cleared in `describeRead`, covered by `TestSessionBadPacketPanicCapture/coalesced-flush-transferred-panic-does-not-misattribute-outer-frame` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| All 29 created files (`internal/core/{drift,badpacket,supportreg}/*`, `internal/component/plugin/server/{drift,support_modules}*`, `internal/component/firewall/{drift,explain,cmd_trace}*`, `internal/plugins/fib/kernel/drift*`, `internal/component/iface/drift*`, `internal/component/bgp/reactor/session_badpacket_test.go`, `internal/component/cmd/show/{drift,bad_packets}*`, `internal/component/support/{register,runtime_collectors}*`, `test/plugin/diag-drift-explain-and-support.ci`, `website/changes/fragments/ai-diagnostics-5-drift-explain-and-support-registry.md`) | Yes | Verified by `git status --short` and `./le spec review record` (77 files hash-pinned, exit 0) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-3 | Drift registry, RPC fan-out across `DirectBridge`/`MuxConn`, cross-plugin conflict refusal, `show system drift`, `doctor-dataplane-drift` | `./le test unit pkg path ./internal/component/plugin/server/ run "TestValidateDriftSubsystemDecls\|TestDriftCheck\|TestDriftSubsystems\|TestJudgeDeclaration\|TestServerSupportModules"` PASS (`6.955s`); `-race` PASS on `internal/core/drift`, `internal/component/{firewall,iface,cmd/show}`, `internal/plugins/fib/kernel`; `test/plugin/diag-drift-explain-and-support.ci` PASS (`17.7s`) |
| AC-4..AC-5 | Read-only `firewall.EvaluatePacket`, `show firewall trace`, honest `unevaluated`/`incomplete` | `-race` PASS on `internal/component/firewall/...`; `test/plugin/diag-drift-explain-and-support.ci` PASS |
| AC-6 | `badpacket` 16x4KiB ring, RFC 7606 / standard & coalesced headers / owned-buffer panic unwind, `show system bad-packets`, triage count | `ZE_MEMGUARD=poison ./le test unit pkg path ./internal/component/bgp/reactor/ run "TestSessionBadPacket\|TestSessionReadPanic\|TestReceivePublicationFollowPanicPreservesPrimaryOwnership"` PASS (`2.974s`); `-race` PASS on `internal/core/{badpacket,taskguard}`, `internal/component/cmd/show`; `test/plugin/diag-drift-explain-and-support.ci` PASS |
| AC-7..AC-8 | `internal/core/supportreg` + 10 live support modules (local + daemon) + MCP `ze://triage` RBAC & 5s single-site TTL | `-race` PASS on `internal/core/supportreg`, `internal/component/{support,mcp}`; `test/plugin/diag-drift-explain-and-support.ci` PASS (local 27-module archive, daemon MCP 30-module archive, live MCP `ze://triage` `ttlMs: 5000`) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `show system drift` (`\| subsystem`), `show doctor` (`dataplane-drift`), `show firewall trace`, `show system bad-packets` (`\| protocol`), `show system triage`, `request support archive` (local + MCP daemon), MCP `resources/list` + `resources/read` (`ze://triage`) | `test/plugin/diag-drift-explain-and-support.ci` | Yes — 80/80 `stress-repro` (`burners=64, parallel=16`) PASS + in-place `./le test bgp plugin` PASS (`17.7s`) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `internal/core/{drift,badpacket,supportreg}` pass `./le arch tier check` with zero upward imports |
| A-2 | confirmed | `ze-plugin-callback:drift-check` verified symmetrically over `DirectBridge` and `MuxConn` in `internal/component/plugin/server/drift_test.go` |
| A-3 | confirmed | `finishReadBuffer` + `panicFrameSuppressed` + `describeCancel` verified under `-race` and `ZE_MEMGUARD=poison` with `0.0` happy-path allocs |
| A-4 | confirmed | `stampCacheHints` remains the sole cache-hint site (`ttlLiveStateMs = 5000` for `ze://`, `ttlEmbeddedAssetMs = 3600000` for `ui://`) verified in `internal/component/mcp/{caching_test.go,resources_test.go}` and live `.ci` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| All 10 architecture/feature/guide docs + `ze-diagnostics.md` skill + `ai-diagnostics-5-drift-explain-and-support-registry.md` news fragment | Source anchors verified by `./le doc check all` (0 errors on all Spec 5 docs, anchors, and YANG nodes); `docs/architecture/mcp/overview.md` departure updated cleanly | Yes |
