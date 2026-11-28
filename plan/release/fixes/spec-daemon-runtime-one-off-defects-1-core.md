# Spec: Daemon runtime one-off defects (1) - Core logging, Loc-RIB metrics, textbuf, and store resolution

| Field | Value |
|-------|-------|
| Status | design |
| Scope | core |
| Priority | P1 |
| Release reason | Fixes five open core-tier defects: repeat `slogutil.Logger` calls detaching runtime log levels, forked plugin loggers ignoring dotted `ze.log.<parent>` keys, unwired Loc-RIB per-shard metrics, zero-value `textbuf.Buffer` skipping its inline array, and `resolve.StoreDir` ignoring `ze.config.dir` for bare config names |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent umbrella: `plan/release/fixes/spec-daemon-runtime-one-off-defects.md`. This child owns the five open `internal/core/` (plus `internal/component/policy/register.go` metrics hook) journal rows routed from the umbrella.

## Task

Fix five core-tier runtime defects verified open at HEAD on 2026-11-26:

1. **Repeat `slogutil.Logger` calls detach runtime log level (`second-logger-detaches-runtime-level`, 2026-10-28).** `slogutil.Logger(subsystem)` (`internal/core/slogutil/slogutil.go`) allocates a fresh `&slog.LevelVar{}` on every call and overwrites `levelRegistry.Store(subsystem, lv)` (and `filterRegistry.Store(subsystem, fh)`). Package-level panic guards (`bmp`, `dhcpserver`, `ldp`, `tftpserver`, `rsvpte`, `vrrp`) call `slogutil.Logger(<subsystem>)` a second time, detaching the first logger's `LevelVar` so `slogutil.SetLevel` / `request log level` never reaches it. Reuse and update the existing `*slog.LevelVar` (via `levelRegistry.LoadOrStore`) and existing `*filterHandler` (via `filterRegistry.LoadOrStore`) for the same subsystem name.
2. **Forked plugin logger ignores dotted `ze.log.<parent>` keys (`second-logger-detaches-runtime-level`, 2026-11-02).** `slogutil.PluginLogger(subsystem, cliLevel)` (`internal/core/slogutil/slogutil.go`) passes the raw hyphenated registry name (for example `bgp-role`, `fib-kernel`, `ddos-flowspec`) directly to `getLogEnv(subsystem)`, which splits on `.` only. Consequently `ze.log.bgp` never reaches a forked `bgp-role` process and stderr logs `subsystem=bgp-role` instead of `subsystem=bgp.role`. Canonicalize hyphens to dots (`strings.ReplaceAll(subsystem, "-", ".")`) at the entry of `slogutil.PluginLogger`, matching `plugin.CanonicalSubsystemName`, so both `getLogEnv` hierarchy walking and stderr `"subsystem"` tags agree between internal and forked modes for every plugin without editing 30+ plugin `register.go` files.
3. **Loc-RIB per-shard metrics never exported (`unwired-feature`, 2026-10-30).** `locrib.SetMetricsRegistry` (`internal/core/rib/locrib/metrics.go`) has zero production callers, so per-shard insert, remove, lookup, and depth metrics are never exported. Wire `locrib.SetMetricsRegistry(reg)` alongside `locrib.SetPolicyMetricsRegistry(reg)` inside the `ConfigureMetrics` hook of `internal/component/policy/register.go` (which already binds Loc-RIB policy metrics when the metrics registry is injected).
4. **Zero-value `textbuf.Buffer` skips its 128-byte inline array (`fast-path-type-used-without-its-initializer`, 2026-09-02).** Calling write methods on a bare `var b textbuf.Buffer` without `b.Reset()` leaves `b.b == nil`, causing `append` to heap-allocate on the first write. In `(*Buffer).mustBeWritable()` (`internal/core/textbuf/textbuf.go`), lazily point `b.b = b.inlineSlice()` whenever `b.b == nil`. `New()` sets `b.b = b.arr[:0]` (non-nil, still heap-allocated so `TestNewSliceOutlivesCaller` stays safe), `Get()` and `Reset()` already set `b.b` non-nil, and every bare `var b textbuf.Buffer` across the repository immediately gets zero-allocation inline storage. Update `docs/architecture/textbuf-string-building.md` and `textbuf.go` doc comments to record that the zero value now uses the inline array directly.
5. **Bare config name resolves to working directory and ignores `ze.config.dir` (`supplied-path-outranks-the-configured-store`, 2026-11-19).** `resolve.StoreDir(configPath)` (`internal/core/resolve/resolve.go`) returns `filepath.Dir(configPath)` (which is `.`) for any non-empty `configPath != "-"`, even when `configPath` is a bare name like `client-edge-99.conf` with no path separator and no file in the current directory while `ze.config.dir` is set. When `configPath` is a bare filename (contains no path separator and `filepath.Base(configPath) == configPath`), check whether `configPath` exists as a file in the working directory; if it does not and `env.Get("ze.config.dir")` is non-empty, return `env.Get("ze.config.dir")` (otherwise keep existing `filepath.Dir` / `DefaultConfigDir` behavior so loose files in cwd and explicit relative/absolute paths like `./foo.conf` or `/etc/ze/foo.conf` stay unchanged).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, public API, core leaf boundaries
- [ ] `ai/doors/core.md` - core import direction, env keys, shared libraries
- [ ] `docs/architecture/config/environment.md` - env var precedence and `ze.log.*` hierarchy
- [ ] `docs/architecture/rib/unified-locrib.md` - Loc-RIB shard architecture and metrics
- [ ] `docs/architecture/textbuf-string-building.md` - inline backing array, `noescape`, and `New()` heap rule
- [ ] `docs/features/ai-first.md` - shared storage and config resolution

## Current Behavior (MANDATORY)

Verified at HEAD on 2026-11-26:

- [ ] `internal/core/slogutil/slogutil.go` - `Logger` (L211-228) creates a fresh `&slog.LevelVar{}` and overwrites `levelRegistry.Store(subsystem, lv)` and `filterRegistry.Store(subsystem, fh)` on every call; `PluginLogger` (L234-256) passes raw hyphenated `subsystem` straight to `getLogEnv` without replacing `-` with `.`.
- [ ] `internal/core/rib/locrib/metrics.go` & `internal/component/policy/register.go` - `locrib.SetMetricsRegistry` (L32-55) has zero callers; `policy/register.go` (L105) binds only `ConfigureMetrics: locrib.SetPolicyMetricsRegistry`.
- [ ] `internal/core/textbuf/textbuf.go` - `mustBeWritable()` (L206-210) checks only `b.done` and leaves `b.b == nil` on a bare `var b textbuf.Buffer`, so the first write heap-allocates instead of using `b.arr[:0]`.
- [ ] `internal/core/resolve/resolve.go` - `StoreDir` (L25-35) returns `filepath.Dir(configPath)` (`.`) for any bare filename before consulting `env.Get("ze.config.dir")`.

**Behavior to preserve:**
- `slogutil.Logger` returns a discard logger without registering when `ze.log.*` holds an unrecognized level word; `SetLevel` and `ListLevels` continue to operate on `levelRegistry`.
- `slogutil.PluginLogger` keeps CLI flag `--log-level` precedence over env vars and `DefaultLevel`, and always writes to `os.Stderr`.
- `textbuf.New()` remains heap-allocated (`b.b = b.arr[:0]`) so `Slice()` returned from `New()` never dangles (`TestNewSliceOutlivesCaller`); `String()` continues to empty the buffer at every size.
- `resolve.StoreDir` continues to honor explicit paths with directory components (`./ze.conf`, `/etc/ze/ze.conf`), stdin (`-`), empty path (`""`), and existing loose files in the current working directory.

**Behavior to change:**
- Repeat `slogutil.Logger(subsystem)` calls share one `*slog.LevelVar` and `*filterHandler` per subsystem (updating the level rather than replacing the pointer).
- `slogutil.PluginLogger` canonicalizes `-` to `.` in `subsystem` before env hierarchy lookup and log tagging.
- `internal/component/policy/register.go` `ConfigureMetrics` calls both `locrib.SetPolicyMetricsRegistry(reg)` and `locrib.SetMetricsRegistry(reg)`.
- `textbuf.Buffer.mustBeWritable()` lazily initializes `b.b = b.inlineSlice()` when `b.b == nil`.
- `resolve.StoreDir` falls through to `ze.config.dir` for a bare config name that does not exist as a file in the working directory.

## Existing Mechanisms (MANDATORY)

Verified at HEAD on 2026-11-26.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Subsystem logger level and filter registries | `internal/core/slogutil/slogutil.go` `Logger`, `levelRegistry`, `filterRegistry`, `SetLevel` | Stores a fresh `LevelVar` and `filterHandler` on each `Logger` call, detaching prior loggers for the same subsystem | Extends: uses `LoadOrStore` so repeat calls share one `LevelVar` and `filterHandler` per subsystem |
| Forked plugin logger and canonical naming | `internal/core/slogutil/slogutil.go` `PluginLogger`, `getLogEnv`; `internal/component/plugin/inprocess.go` `CanonicalSubsystemName` | `inprocess.go` replaces `-` with `.` for internal plugins, while `PluginLogger` leaves hyphens so `getLogEnv` misses parent dotted keys | Extends: canonicalizes `-` to `.` inside `PluginLogger` so forked and internal loggers follow one naming rule |
| Policy plugin metrics hook | `internal/component/policy/register.go` `ConfigureMetrics`; `internal/core/rib/locrib/metrics.go` `SetMetricsRegistry`; `policy_metrics.go` `SetPolicyMetricsRegistry` | Wires `locrib.SetPolicyMetricsRegistry` when the daemon injects the metrics registry; `locrib.SetMetricsRegistry` is never called | Extends: binds both `SetPolicyMetricsRegistry` and `SetMetricsRegistry` in the existing `ConfigureMetrics` hook |
| Inline backing array on `textbuf.Buffer` | `internal/core/textbuf/textbuf.go` `inlineSlice`, `mustBeWritable`, `Reset`, `New`, `Get` | `Reset` and `Get` point `b.b` at `b.inlineSlice()`, `New` points `b.b` at heap `b.arr[:0]`, bare zero values leave `b.b == nil` | Extends: `mustBeWritable` initializes `b.b = b.inlineSlice()` when `b.b == nil` so zero values use the inline array automatically |
| Shared store directory resolver | `internal/core/resolve/resolve.go` `StoreDir`, `StorageFor` | Returns `filepath.Dir(configPath)` for any non-empty, non-`-` path before checking `ze.config.dir` | Extends: checks whether a bare filename exists in cwd before overriding `ze.config.dir` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Shared machinery you use rather than rebuild | the door every change reads; the door covering `internal/component/policy/register.go` | "`env.MustRegister`, `slogutil.Logger`" | agrees | Repairs `slogutil.Logger` and `slogutil.PluginLogger` in place with no second logger registry. |
| `ai/doors/core.md` | Where it sits | the door covering `internal/core/slogutil/`, `internal/core/textbuf/`, `internal/core/resolve/` | "A core package is a library with no config-driven lifecycle and no registry side effect" | agrees | All core edits stay inside leaf core libraries and add zero upward imports to `internal/component/` or `internal/plugins/`. |
| `ai/doors/le-tooling.md` | Where it sits | the door covering `test/plugin/forked-plugin-dotted-log-env.ci` | "Functional `.ci` files live under `test/<suite>/` and run through `./le test <suite>` on the functional runner" | agrees | Adds the forked-plugin logging proof as a standard `.ci` under `test/plugin/`. |
| `docs/architecture/config/environment.md` | Overview | `// Design:` page of `internal/core/slogutil/slogutil.go` | "Ze environment variables are registered centrally in" | agrees | Preserves centralized env registration and restores dotted `ze.log.<parent>` inheritance in `slogutil.PluginLogger`. |
| `docs/architecture/config/environment-block.md` | TL;DR | anchored to `internal/core/slogutil/slogutil.go` | "YANG container `environment { }` sets process options from the config file." | agrees | Fixing `slogutil.Logger` reuse and `PluginLogger` hyphen canonicalization ensures `environment { log { ... } }` reaches both repeat loggers and forked plugins. |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | anchored to `internal/core/slogutil/slogutil.go` | silent (page covers Docker interop suite definitions and peer scenarios, not `slogutil.Logger` level reuse) | silent | This spec touches no Docker interop suite or scenario. |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | No interop suite capability check is altered or widened; changes stay in core logging, metrics wiring, textbuf, and store resolution. |
| `docs/guide/plugins.md` | Config-Driven Loading | `// Design:` page of `internal/component/policy/register.go` | "BGP itself is a config-driven plugin." | agrees | Wires `locrib.SetMetricsRegistry` inside the existing `policy` plugin `ConfigureMetrics` hook without changing plugin loading semantics. |
| `docs/architecture/rib/unified-locrib.md` | The decisions | anchored to `internal/component/policy/register.go` | "**The external selected projection is a reader, not a second Loc-RIB.**" | agrees | Exports the existing single Loc-RIB's per-shard counters and gauges via `locrib.SetMetricsRegistry` without creating any second RIB structure. |
| `docs/architecture/textbuf-string-building.md` | How Buffer stays off the heap | `// Design:` page of `internal/core/textbuf/textbuf.go`; modified by this spec | "`Buffer` is a chainable builder with a 128-byte inline backing array." | agrees | Makes zero-value `Buffer` lazily bind its inline 128-byte array on first write while preserving `New()` heap allocation (`TestNewSliceOutlivesCaller`), and updates this page in the same change. |
| `docs/features/ai-first.md` | Shared Registrations | `// Design:` page of `internal/core/resolve/resolve.go` | "The core design point is broader than AI." | agrees | Repairs shared `resolve.StoreDir` in one place so every CLI consumer resolves bare config names consistently against `ze.config.dir`. |
| `docs/architecture/web-interface.md` | Component Structure | anchored to `internal/core/resolve/resolve.go` | silent (page covers templ components, routes, and the web UI, not `resolve.StoreDir` bare-name fallback) | silent | Web callers passing `""` or explicit paths see unchanged `resolve.StoreDir` behavior. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `slogutil.Logger(subsystem)` and `slogutil.PluginLogger(subsystem, cliLevel)` at startup and runtime log-level changes (`request log level`).
- `policy` registration `ConfigureMetrics` hook during daemon metrics injection (`internal/component/plugin/inprocess.go`).
- Any `textbuf.Buffer` write method (`Str`, `Byte`, `Uint`, `Int`, `Write`, etc.).
- `resolve.StoreDir(configPath)` called by CLI config commands (`ze config edit`, `history`, `rollback`, `diff`, `doctor`).

### Transformation Path
1. `Logger(subsystem)` resolves level -> `LoadOrStore` existing `LevelVar` (updating its level) and `filterHandler` -> wraps shared `filterHandler` -> subsequent `SetLevel(subsystem, ...)` reaches every logger for that subsystem.
2. `PluginLogger(subsystem, cliLevel)` replaces `-` with `.` -> checks `cliLevel` -> walks `getLogEnv` dotted prefixes -> emits stderr records tagged with canonical dotted `subsystem`.
3. Metrics injection calls `policy`'s `ConfigureMetrics(reg)` -> registers both Loc-RIB policy metrics and per-shard insert/remove/lookup/depth metrics in `reg`.
4. First write on a zero-value `textbuf.Buffer` hits `mustBeWritable()` -> sees `b.b == nil` -> sets `b.b = b.inlineSlice()` -> appends into the 128-byte stack array with zero heap allocation.
5. `resolve.StoreDir(configPath)` sees a bare name (no path separator) -> checks `os.Stat(configPath)` -> if absent and `ze.config.dir` is set, returns `ze.config.dir`; otherwise keeps `filepath.Dir(configPath)` (or `DefaultConfigDir`).

### Boundaries Crossed
- Core leaf tier (`internal/core/`) stays free of upward imports (`./le arch tier check` clean).
- Process boundary: `PluginLogger` canonicalization gives forked external plugins identical dotted env-key inheritance (`ze.log.bgp`, `ze.log.bgp.role`) as internal plugins.

### Integration Points
- `internal/core/slogutil` (`SetLevel`, `ListLevels`, `Logger`, `PluginLogger`).
- `internal/component/policy/register.go` (`ConfigureMetrics`) and `internal/core/rib/locrib/metrics.go`.
- `internal/core/textbuf` and `docs/architecture/textbuf-string-building.md`.
- `internal/core/resolve` (`StoreDir`, `StorageFor`).

### Changed-Wire Census

No wire format, RPC payload, event schema, or external protocol changes in this spec. All five fixes are internal Go runtime behavior (in-process logger sharing, stderr log tag / env lookup, Prometheus metric registration in the existing registry, stack buffer initialization, and local config-store path resolution).

### Architectural Verification
- [ ] `internal/core/` adds zero imports of `internal/component/` or `internal/plugins/` (`./le arch tier check`)
- [ ] `textbuf.New()` still heap-allocates (`TestNewSliceOutlivesCaller` green)
- [ ] Forked plugin inherits dotted `ze.log.<parent>` and `ze.log.<parent>.<child>` env keys identically to internal plugins

## Risks & Assumptions

### Assumptions
- `resolve.StoreDir` callers passing an explicit relative path (`./edge.conf`, `dir/edge.conf`) or an existing file in cwd expect `filepath.Dir(configPath)` to win over `ze.config.dir`; only a bare name absent from cwd should consult `ze.config.dir`.
- `policy` plugin is always linked when Loc-RIB is active, so wiring `locrib.SetMetricsRegistry` in `policy/register.go` reaches every daemon exposing metrics.

### Risks
- Lazy `inlineSlice()` inside `mustBeWritable()` must not interfere with `New()` (which already sets `b.b = b.arr[:0]`, non-nil) or `String()` (which must leave `b.b` non-nil via `inlineSlice()`, never `nil`, when emptying). Verify with `go test -race ./internal/core/textbuf/...` including `TestNewSliceOutlivesCaller` and `testing.AllocsPerRun`.
- Reusing `filterHandler` across repeat `slogutil.Logger` calls must remain data-race free under concurrent `Logger`, `SetLevel`, and log writes (`go test -race ./internal/core/slogutil/...`).

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Second `slogutil.Logger("bgp.rpki")` call then `slogutil.SetLevel("bgp.rpki", "debug")` | → | `slogutil.Logger` `LoadOrStore` | `TestLoggerRepeatCallSharesLevelVar` (`internal/core/slogutil/slogutil_test.go`) |
| Forked plugin started with `ze.log.bgp=debug` | → | `slogutil.PluginLogger` canonicalization | `TestPluginLoggerInheritsParentDottedEnv` (`internal/core/slogutil/slogutil_test.go`) + forked `.ci` under `test/plugin/` |
| `policy` `ConfigureMetrics(reg)` then Loc-RIB `Insert` | → | `locrib.SetMetricsRegistry` | `TestPolicyConfigureMetricsExportsLocRIBShards` (`internal/component/policy/register_test.go`) |
| Bare `var b textbuf.Buffer; b.Str("x").Bytes()` | → | `Buffer.mustBeWritable` lazy `inlineSlice` | `TestZeroValueBufferUsesInlineArray` (`internal/core/textbuf/textbuf_test.go`) |
| `resolve.StoreDir("client-edge-99.conf")` with `ze.config.dir=/tmp/store` and no local file | → | `resolve.StoreDir` | `TestStoreDirBareNameUsesConfiguredDir` (`internal/core/resolve/resolve_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Calling `slogutil.Logger("bgp.rpki")` twice followed by `slogutil.SetLevel("bgp.rpki", "debug")` (`second-logger-detaches-runtime-level`, 2026-10-28) | Both the first and second logger handles share one `*slog.LevelVar` and emit Debug records after `SetLevel`, passing `go test -race ./internal/core/slogutil/...` | A `slogutil.Logger` call given an unrecognized `ze.log.*` level word continues returning a discard logger outside `levelRegistry`, preserving fail-safe invalid-env handling |
| AC-2 | Starting `slogutil.PluginLogger("bgp-role", "")` or a forked plugin with only `ze.log.bgp=debug` or `ze.log.bgp.role=debug` set (`second-logger-detaches-runtime-level`, 2026-11-02) | `PluginLogger` resolves `ze.log.bgp.role` and parent `ze.log.bgp`, emits Debug records to stderr tagged with `subsystem=bgp.role`, and passes both `TestPluginLoggerInheritsParentDottedEnv` and `test/plugin/forked-plugin-dotted-log-env.ci` | An explicit CLI `--log-level` flag continues to outrank env vars and `DefaultLevel`, matching documented CLI precedence |
| AC-3 | Invoking the `policy` registration's `ConfigureMetrics(reg)` hook and inserting routes into Loc-RIB (`unwired-feature`, 2026-10-30) | `ConfigureMetrics` wires both `locrib.SetPolicyMetricsRegistry(reg)` and `locrib.SetMetricsRegistry(reg)`, and scraping `reg` after a Loc-RIB insert exports per-shard insert, remove, lookup, and depth metric series | Daemons built or started without a metrics registry leave Loc-RIB metric hooks nil and run with zero metric overhead |
| AC-4 | Writing up to 128 bytes into a bare zero-value `var b textbuf.Buffer` without calling `Reset()` (`fast-path-type-used-without-its-initializer`, 2026-09-02) | `b.mustBeWritable()` binds `b.b` to `b.inlineSlice()`, producing the exact written bytes with `testing.AllocsPerRun == 0`, while `TestNewSliceOutlivesCaller` stays green and `docs/architecture/textbuf-string-building.md` documents the zero-value inline behavior | Buffers that grow beyond 128 bytes or are allocated via `textbuf.New()` still heap-allocate so large strings and returned `Slice()` values remain memory-safe |
| AC-5 | Calling `resolve.StoreDir("client-edge-99.conf")` with `ze.config.dir` set and `client-edge-99.conf` absent from the working directory (`supplied-path-outranks-the-configured-store`, 2026-11-19) | `resolve.StoreDir` returns the configured `ze.config.dir` path | Explicit paths containing a directory separator (`./x.conf`, `/tmp/x.conf`), existing loose files in the current working directory, stdin (`-`), and empty path (`""`) retain their existing `filepath.Dir` / `DefaultConfigDir` resolution so explicit operator paths keep winning |

## TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| `TestLoggerRepeatCallSharesLevelVar` | `internal/core/slogutil/slogutil_test.go` | AC-1 |
| `TestPluginLoggerInheritsParentDottedEnv` | `internal/core/slogutil/slogutil_test.go` | AC-2 |
| `TestPolicyConfigureMetricsExportsLocRIBShards` | `internal/component/policy/register_test.go` | AC-3 |
| `TestZeroValueBufferUsesInlineArray` | `internal/core/textbuf/textbuf_test.go` | AC-4 |
| `TestStoreDirBareNameUsesConfiguredDir` | `internal/core/resolve/resolve_test.go` | AC-5 |

### Functional Tests

| Test | AC |
|------|----|
| Forked plugin `.ci` under `test/plugin/` setting `ze.log.<parent>.<child>=debug` (no `--log-level`) and observing debug output | AC-2 |

## Files to Modify
- `internal/core/slogutil/slogutil.go`
- `internal/core/slogutil/slogutil_test.go`
- `internal/component/policy/register.go`
- `internal/core/textbuf/textbuf.go`
- `internal/core/textbuf/textbuf_test.go`
- `internal/core/resolve/resolve.go`
- `internal/core/resolve/resolve_test.go`
- `docs/architecture/textbuf-string-building.md`

## Files to Create
- `internal/component/policy/register_test.go` (if absent; otherwise extend existing test file in `internal/component/policy/`)
- `test/plugin/forked-plugin-dotted-log-env.ci`

### Integration Checklist
- [ ] `slogutil` level changes reach existing loggers across repeat `Logger()` calls
- [ ] Forked plugin inherits dotted `ze.log.*` env hierarchy
- [ ] Metrics scrape includes Loc-RIB shard series
- [ ] Zero-value `textbuf.Buffer` achieves 0 allocs with `TestNewSliceOutlivesCaller` green
- [ ] `resolve.StoreDir` resolves bare names against `ze.config.dir` when absent from cwd

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/architecture/textbuf-string-building.md` - document that zero-value `Buffer` lazily binds its inline array on first write

## Design presentation

### What changes
- `slogutil.Logger` reuses the stored `*slog.LevelVar` and `*filterHandler` via `LoadOrStore` on repeat calls for the same subsystem.
- `slogutil.PluginLogger` canonicalizes hyphens to dots before `getLogEnv` and `.With("subsystem", ...)`.
- `internal/component/policy/register.go` `ConfigureMetrics` wires `locrib.SetMetricsRegistry` beside `locrib.SetPolicyMetricsRegistry`.
- `textbuf.Buffer.mustBeWritable` lazily sets `b.b = b.inlineSlice()` when `b.b == nil`.
- `resolve.StoreDir` consults `ze.config.dir` for bare config names that do not exist as files in the working directory.

### Before and after
- **Before:** Calling `slogutil.Logger` twice orphaned the first logger's `LevelVar`; forked plugins named `bgp-role` missed `ze.log.bgp`; `locrib.SetMetricsRegistry` had zero callers; bare `var b textbuf.Buffer` heap-allocated on first write; `resolve.StoreDir("edge.conf")` returned `.` even when `ze.config.dir` was set.
- **After:** Every logger handle for a subsystem shares one `LevelVar`; forked plugins obey the exact same dotted `ze.log.*` hierarchy as internal ones; Loc-RIB per-shard metrics are exported on scrape; every bare `textbuf.Buffer` uses its 128-byte stack array automatically; bare config names absent from cwd resolve to `ze.config.dir`.

### Alternatives set aside
- **Editing 30+ plugin `register.go` `CLIHandler` closures to call `CanonicalSubsystemName`:** Rejected because canonicalizing inside `slogutil.PluginLogger` fixes all internal, BGP, and system plugins in one place without cross-door churn.
- **Adding `b.Reset()` to 6,676 `var b textbuf.Buffer` call sites plus a vet linter:** Rejected because lazy initialization in `mustBeWritable()` makes the zero value genuinely zero-allocation across the whole codebase with one branch and zero call-site boilerplate.
- **Making `ze.config.dir` always outrank every supplied path in `resolve.StoreDir`:** Rejected because callers passing explicit paths (`./foo.conf`, `/etc/ze/foo.conf`) or an existing loose file in cwd must keep opening that file's store directory.

### Documents departed from
None.

### Choices for Thomas
none

## Implementation Steps
1. Write red unit tests for AC-1 to AC-5 and confirm each fails at HEAD.
2. Implement the `slogutil.Logger` `LoadOrStore` reuse and `slogutil.PluginLogger` hyphen-to-dot canonicalization in `internal/core/slogutil/slogutil.go`.
3. Wire `locrib.SetMetricsRegistry` alongside `locrib.SetPolicyMetricsRegistry` in `internal/component/policy/register.go`.
4. Add lazy `b.b = b.inlineSlice()` in `textbuf.Buffer.mustBeWritable()` and update `docs/architecture/textbuf-string-building.md`.
5. Update `resolve.StoreDir` for bare names absent from cwd when `ze.config.dir` is set.
6. Add the forked plugin `.ci` test for AC-2, verify all unit and functional tests pass, and close the five owned journal rows in `plan/journal/`.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `second-logger-detaches-runtime-level` | 2026-10-28 | panic guards calling `slogutil.Logger` twice |
| `second-logger-detaches-runtime-level` | 2026-11-02 | forked BGP plugin logger ignores the dotted `ze.log.<subsystem>` key |
| `unwired-feature` | 2026-10-30 | `locrib.SetMetricsRegistry` never called |
| `fast-path-type-used-without-its-initializer` | 2026-09-02 | `internal/core/textbuf` |
| `supplied-path-outranks-the-configured-store` | 2026-11-19 | `resolve.StoreDir` resolves a bare config name to the working directory and ignores `ze.config.dir` |

## Work Not Done

Sibling runtime defects routed from `plan/release/fixes/spec-daemon-runtime-one-off-defects.md` live in Children 2 through 7:
- `plan/release/fixes/spec-daemon-runtime-one-off-defects-2-config-plugin-map.md`
- `plan/release/fixes/spec-daemon-runtime-one-off-defects-3-plugin-infra.md`
- `plan/release/fixes/spec-daemon-runtime-one-off-defects-4-hub-runtime.md`
- `plan/release/fixes/spec-daemon-runtime-one-off-defects-5-api-surfaces.md`
- `plan/release/fixes/spec-daemon-runtime-one-off-defects-6-provision-ipxe.md`
- `plan/release/fixes/spec-daemon-runtime-one-off-defects-7-mock-help-flush.md`

## Review Gate

- [ ] Code review clean (`/ze-review`)
- [ ] Spec compliance verified (`/ze-review-spec`)
- [ ] All five owned journal rows marked `fixed:` in `plan/journal/`

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (red recorded per AC)
- [ ] Tests PASS
- [ ] Scoped runs: `./le job run label core-fixes command go test -race ./internal/core/slogutil/... ./internal/core/textbuf/... ./internal/core/resolve/... ./internal/core/rib/locrib/... ./internal/component/policy/...`, `./le arch tier check`, `./le test functional plugin`
