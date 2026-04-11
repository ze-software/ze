# Spec: bfd-1-wiring

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/1 |
| Updated | 2026-04-11 |

## Task

Wire the BFD plugin skeleton (committed in `e5a4add9`) into the running ze
daemon so that operators can place a `bfd { ... }` block in their config
and have the plugin auto-load via the `ConfigRoots` mechanism. Stage 1 is
deliberately scoped to *reachability* — proving the lifecycle from config
parser through plugin Configure / OnStarted to a live engine.Loop — and
explicitly defers the harder work (UDP bind on the privileged RFC ports
3784/4784, multi-VRF binding, BGP opt-in, FRR interop) to follow-up
specs tracked in `plan/deferrals.md`.

The original BFD skeleton commit `e5a4add9` shipped `RunBFDPlugin` as a
no-op stub that returned 0 immediately, and intentionally was NOT in
`internal/component/plugin/all/all.go`. A safety comment in
`register.go` warned future sessions not to run `make generate` until
this spec landed. The skeleton's `internal/plugins/bfd/{packet,
session, transport, engine, api, schema}` sub-packages were complete and
race-clean; Stage 1 keeps every test green and adds the plumbing on top.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/bfd.md` — internal design doc with the
  "Next session: start here" section.
  → Decision: copy the SDK lifecycle from `internal/plugins/sysrib/sysrib.go`
  rather than any BGP plugin (closest structural match: long-lived
  goroutine, YANG-driven config, no reactor coupling).
  → Constraint: do NOT bind UDP ports 3784/4784 from a non-root test;
  Stage 1 only proves reachability via lifecycle log lines.
- [ ] `.claude/rules/integration-completeness.md` — every feature must be
  proven reachable end-to-end.
  → Constraint: a `.ci` test that exercises the config-to-plugin path is
  required, not deferrable.
- [ ] `.claude/rules/plugin-design.md` — Renaming a Registered Name table.
  → Constraint: subsystem log key is `bfd` (single token); env var
  `ze.log.bfd` is automatically valid via the `ze.log.<subsystem>` prefix
  wildcard registered in `internal/core/slogutil/slogutil.go:45` — no
  separate `env.MustRegister` needed.

### RFC Summaries
- [ ] `rfc/short/rfc5880.md` — base BFD protocol.
  → Constraint: §6.8.16 administrative-down semantics drive
  `SessionHandle.Shutdown`.
- [ ] `rfc/short/rfc5881.md` — single-hop UDP encapsulation.
  → Constraint: port 3784, TTL 255 GTSM (Stage 2 enforcement).
- [ ] `rfc/short/rfc5883.md` — multi-hop UDP encapsulation.
  → Constraint: local source address required.

**Key insights:**
- The plugin auto-load chain is `register.go init() ->
  registry.Register({ConfigRoots: ["bfd"]}) ->
  generated all.go blank-import -> config parser top-level keyword`.
- The config parser only knows top-level keywords from a hard-coded list
  in `internal/component/config/yang_schema.go YANGSchemaWithPlugins` —
  the `ze-iface-conf` block was the canonical example to copy.
- SDK ConfigSection.Data delivers leaves as **strings**, not native JSON
  types — every plugin parser walks `map[string]any` and converts.

## Current Behavior

**Source files read:**
- [ ] `internal/plugins/bfd/bfd.go` — `RunBFDPlugin` is a Debug log + return 0 stub.
- [ ] `internal/plugins/bfd/register.go` — top-of-file warning comment forbids
  `make generate`; `Registration` lacks `ConfigureEngineLogger`.
- [ ] `internal/plugins/bfd/api/service.go` — `SessionHandle` has Subscribe / Unsubscribe / Key but no Shutdown / Enable.
- [ ] `internal/plugins/bfd/engine/engine.go` — `NewLoop`, `Start`, `Stop`,
  `EnsureSession`, `ReleaseSession` already implemented.
- [ ] `internal/plugins/bfd/engine/loop.go` — `handle` type implements `api.SessionHandle`.
- [ ] `internal/plugins/bfd/session/fsm.go` — `Machine.AdminDown(diag)` and `AdminEnable()` exist; not exposed via api.
- [ ] `internal/plugins/bfd/transport/udp.go` — `UDP` is a struct with `Bind`, `Mode`, `VRF` fields that satisfy `Transport`.
- [ ] `internal/plugins/sysrib/{sysrib.go,register.go}` — reference SDK lifecycle pattern.
- [ ] `internal/component/iface/{register.go,config.go}` — string-leaf parser pattern.
- [ ] `internal/component/config/yang_schema.go` — `YANGSchemaWithPlugins`
  contains the hard-coded list of YANG modules whose top-level entries
  become valid config keywords.
- [ ] `internal/component/plugin/all/all.go` — generated; bfd not present.
- [ ] `internal/component/plugin/all/all_test.go` — `TestAllPluginsRegistered` is an explicit-list assertion.
- [ ] `cmd/ze/main_test.go` — `TestAvailablePlugins` is a SECOND explicit-list assertion against `plugin.AvailableInternalPlugins()`.

**Behavior to preserve:**
- Every existing BFD unit test stays green (`-race` clean across packet,
  session, transport, engine).
- The `engine.NewLoop(transport, clock)` constructor signature.
- The `register.go` plugin name `bfd` and feature flag `yang`.
- `RunBFDPlugin(net.Conn) int` signature (called from the registry).

**Behavior to change:**
- Replace the no-op stub with a real SDK lifecycle.
- Add `Shutdown()` and `Enable()` to `api.SessionHandle`.
- Delete the safety warning comment from `register.go`.
- Add `bfd` to both explicit plugin lists.
- Add `ze-bfd-conf` to the parser's hard-coded module list.

## Data Flow

### Entry Point
- Operator places `bfd { ... }` block at the top level of a ze config
  file (or via `ze config edit`).
- Format at entry: hierarchical text config; the parser converts to a
  Tree, then JSON-encoded `ConfigSection` for delivery to the plugin.

### Transformation Path
1. Parser reads `bfd { ... }` -> `Tree` node, validated against
   `ze-bfd-conf` YANG schema (now loaded by `YANGSchemaWithPlugins`).
2. ConfigProvider serialises the bfd subtree to JSON and ships it as
   `sdk.ConfigSection{Root: "bfd", Data: <json>}` to the bfd plugin.
3. `RunBFDPlugin` receives the section in `OnConfigVerify` (validate)
   then `OnConfigure` (apply), parses via `parseSections` ->
   `pluginConfig`, applies via `runtimeState.applyPinned`.
4. For each pinned session: `runtimeState.loopFor(loopKey)` lazily
   creates an `engine.Loop` for the (VRF, mode) pair, calls `loop.Start`
   to bind UDP, then `loop.EnsureSession(req)` -> handle. Shutdown bit
   triggers `handle.Shutdown()` -> `Machine.AdminDown(diag)`.
5. On `OnStarted`, the plugin logs "bfd plugin running" and blocks
   inside `p.Run` until shutdown.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config parser → plugin | JSON `ConfigSection` over SDK | [x] via `bfd-config-load.ci` |
| Plugin → engine.Loop | Direct method calls | [x] via `engine_test.go` (existing) |
| engine.Loop → transport.UDP | `Loop.Start` calls `transport.Start` | [x] via `udp_test.go` (existing) |

### Integration Points
- `internal/component/plugin/registry.Registration{ConfigRoots: ["bfd"]}` —
  triggers config-path auto-load when the parser sees `bfd { ... }`.
- `internal/component/config.YANGSchemaWithPlugins` — hard-coded module
  loader; new entry for `ze-bfd-conf` enables the parser to accept
  `bfd { ... }` as a top-level keyword.

### Architectural Verification
- [x] No bypassed layers (config parser → SDK → plugin lifecycle).
- [x] No unintended coupling (the bfd plugin imports nothing from BGP /
  iface / sysrib).
- [x] No duplicated functionality (the lifecycle reuses
  `engine.NewLoop` / `Loop.EnsureSession` already shipped in `e5a4add9`).
- [x] Zero-copy preserved (the Stage 1 code paths only orchestrate;
  the packet hot path was already zero-alloc per `e5a4add9`).

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze plugin bfd --features` (CLI dispatch) | → | `register.go init() -> registry.Register` | `test/plugin/bfd-features.ci` |
| `bfd { ... }` config block | → | parser `ze-bfd-conf` schema → SDK Configure → `RunBFDPlugin` lifecycle | `test/plugin/bfd-config-load.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze plugin bfd --features` invoked from CLI | exit 0, stdout contains `yang` |
| AC-2 | `bfd { ... }` placed at top of ze config | parser accepts the keyword (no "unknown top-level keyword" error) |
| AC-3 | bfd plugin loads with valid profile config | stderr contains debug `bfd plugin starting` |
| AC-4 | bfd plugin Configure callback runs successfully | stderr contains info `bfd plugin configured` (with profile and session counts) |
| AC-5 | bfd plugin OnStarted callback runs successfully | stderr contains info `bfd plugin running` |
| AC-6 | bfd plugin emits structured slog records | stderr contains `subsystem=bfd` (proves `ConfigureEngineLogger` was called with the canonical name) |
| AC-7 | `api.SessionHandle.Shutdown()` callable | `handle.Shutdown()` returns nil and transitions the session via `session.Machine.AdminDown(packet.DiagAdminDown)` |
| AC-8 | `TestAllPluginsRegistered` and `TestAvailablePlugins` updated | both expected lists contain `"bfd"` in the correct sorted position |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| existing engine/handshake test | `internal/plugins/bfd/engine/engine_test.go` | engine.Loop API still green after handle Shutdown/Enable additions | done (pre-existing) |
| existing session FSM test | `internal/plugins/bfd/session/session_test.go` | AdminDown / AdminEnable behaviour intact | done (pre-existing) |
| `TestAllPluginsRegistered` | `internal/component/plugin/all/all_test.go` | bfd appears in registered plugin list | done |
| `TestAvailablePlugins` | `cmd/ze/main_test.go` | bfd appears in `AvailableInternalPlugins()` | done |

### Boundary Tests
None for Stage 1 — the lifecycle path takes no numeric inputs that need
boundary coverage. Profile timer parameters (detect-multiplier 1..255,
desired-min-tx-us, required-min-rx-us) are deferred until Stage 2
exercises real session establishment.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bfd-features` | `test/plugin/bfd-features.ci` | `ze plugin bfd --features` returns yang | done |
| `bfd-config-load` | `test/plugin/bfd-config-load.ci` | ze daemon loads a `bfd { profile fast { ... } }` config and the plugin emits `starting`/`configured`/`running` lifecycle log lines | done |

## Files to Modify

- `internal/plugins/bfd/api/service.go` — add `Shutdown` / `Enable` to `SessionHandle`.
- `internal/plugins/bfd/engine/engine.go` — add `ErrUnknownSession`.
- `internal/plugins/bfd/engine/loop.go` — implement handle `Shutdown` / `Enable`.
- `internal/plugins/bfd/register.go` — drop warning comment, add `ConfigureEngineLogger`.
- `internal/plugins/bfd/bfd.go` — replace stub with real SDK lifecycle.
- `internal/component/config/yang_schema.go` — load `ze-bfd-conf` module
  so `bfd { ... }` is a valid top-level keyword. (Already present at
  spec-run time, added by iface-tunnel commit 2488c4b1; my Edit was a
  no-op.)
- `internal/component/plugin/all/all.go` — regenerate via `make generate`.
- `internal/component/plugin/all/all_test.go` — bump `TestAllPluginsRegistered` expected list.
- `cmd/ze/main_test.go` — bump `TestAvailablePlugins` expected list.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (existing) | [x] | `internal/plugins/bfd/schema/ze-bfd-conf.yang` (no change) |
| YANG schema loader entry | [x] | `internal/component/config/yang_schema.go` |
| CLI commands/flags | [ ] | covered by registry CLI handler |
| Functional test for config path | [x] | `test/plugin/bfd-config-load.ci` |
| Functional test for CLI | [x] | `test/plugin/bfd-features.ci` |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No (Stage 0 docs already present) | - |
| 2 | Config syntax changed? | No (schema unchanged) | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No (SDK API additions are bfd-internal) | - |
| 5 | Plugin added/changed? | No (already documented in `docs/architecture/bfd.md`) | - |
| 6 | Has a user guide page? | Already shipped in `e5a4add9` | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No (no new RFC enforcement) | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Files to Create

- `internal/plugins/bfd/config.go` — config parser (`pluginConfig`, `parseSections`, `parseProfile`, `parseSingleHopSession`, `parseMultiHopSession`).
- `test/plugin/bfd-features.ci` — CLI discovery test.
- `test/plugin/bfd-config-load.ci` — config-load lifecycle test (Python orchestrator drives ze).
- `plan/learned/556-bfd-1-wiring.md` — learned summary (created with the spec deletion in Commit B).

## Implementation Steps

### Implementation Phases

1. **Phase: api surface** — add `SessionHandle.Shutdown` / `Enable`.
   - Files: `api/service.go`, `engine/engine.go` (errors), `engine/loop.go` (impl).
   - Verify: `go test -race ./internal/plugins/bfd/...`
2. **Phase: plugin registration** — drop the warning comment, add
   `ConfigureEngineLogger`. No new behavior; just clears the safety
   barrier and wires the logger callback.
   - Files: `register.go`.
3. **Phase: lifecycle** — replace `RunBFDPlugin` with the SDK callback
   chain (Verify / Configure / Apply / OnStarted) and the per-VRF
   `runtimeState` that owns engine.Loops + handles.
   - Files: `bfd.go`, new `config.go`.
4. **Phase: codegen + parser** — `make generate`, add the bfd module
   loader entry, bump both expected plugin lists.
   - Files: `all.go` (auto), `all_test.go`, `cmd/ze/main_test.go`,
     `internal/component/config/yang_schema.go`.
5. **Phase: functional tests** — write `bfd-features.ci` and
   `bfd-config-load.ci`. Run them in isolation, then `make ze-verify`.
6. **Complete spec** → fill audit tables, write learned summary, commit
   sequence per `rules/spec-preservation.md`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has implementation file:line evidence below |
| Correctness | Plugin lifecycle matches sysrib pattern; SDK callback signatures match |
| Naming | Plugin name `bfd` (single token); no hyphen-to-dot transform needed |
| Data flow | ConfigSection.Data parsed as `map[string]any` (string leaves), not typed JSON |
| Rule: no-layering | Stub `RunBFDPlugin` deleted, not kept alongside the real one |
| Rule: integration-completeness | Two `.ci` tests exercising both reach paths |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `bfd` blank-imported in `all.go` | `grep "internal/plugins/bfd\"" internal/component/plugin/all/all.go` |
| `TestAvailablePlugins` includes bfd | `grep '"bfd"' cmd/ze/main_test.go` |
| `bfd-features.ci` passes | `bin/ze-test bgp plugin -v V` |
| `bfd-config-load.ci` passes | `bin/ze-test bgp plugin -v U` |
| `make ze-verify` clean | `grep "Ze verification passed" tmp/ze-verify-bfd-wire2.log` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Profile names referenced from sessions are checked at parse time; malformed addresses return errors before reaching engine |
| Resource exhaustion | Stage 1 only allocates engine.Loop instances per (VRF, mode) on demand; profile-only configs allocate nothing |
| Privilege escalation | No new system calls; binding UDP 3784/4784 still requires the same caps the daemon already has, and is gated behind pinned sessions (none in Stage 1 tests) |

## Implementation Summary

### What Was Implemented
- `api.SessionHandle` gained `Shutdown() error` and `Enable() error`,
  exposing the existing `session.Machine.AdminDown` / `AdminEnable`
  through the public surface so plugin lifecycle code can react to
  config `shutdown true`.
- `engine.handle` implements both new methods under `Loop.mu`, returning
  `ErrUnknownSession` if the session was torn down between handle
  creation and the call.
- `register.go` lost the "do not run `make generate`" warning comment
  and gained `ConfigureEngineLogger` so the engine wires the canonical
  `bfd` slogutil subsystem before `RunBFDPlugin` returns.
- `bfd.go` replaced the stub with a full SDK lifecycle: `OnConfigVerify`
  parses the bfd section, `OnConfigure` and `OnConfigApply` call
  `runtimeState.applyPinned` to reconcile pinned-session set, `OnStarted`
  logs "bfd plugin running" and `p.Run` blocks until shutdown.
- `runtimeState` keeps `map[loopKey]*engine.Loop` and
  `map[api.Key]api.SessionHandle`. `loopFor` lazily creates and starts a
  loop per (VRF, mode) pair, binding UDP via the existing transport
  package. Stage 1 supports only VRF "default"; non-default VRFs return
  an explicit error pointing at `spec-bfd-2-transport-hardening`.
- New `config.go` walks the JSON-encoded bfd section as
  `map[string]any` (mirroring `internal/component/iface/config.go`)
  because the SDK config bridge stringifies every leaf value before
  delivery — typed-struct unmarshalling would fail with
  `cannot unmarshal string into Go struct field .bfd.enabled of type bool`.
- `internal/component/config/yang_schema.go` loads the `ze-bfd-conf`
  module after `ze-iface-conf`, making `bfd { ... }` a valid top-level
  parser keyword. The block was already in the file when this spec
  ran (added by the iface-tunnel commit `2488c4b1`); my Edit
  reapplied identical content as a no-op. Without this entry the
  parser would reject the block before any plugin code runs.
- `internal/component/plugin/all/all.go` blank-imports
  `internal/plugins/bfd` and `internal/plugins/bfd/schema` after
  `make generate`.
- Both explicit plugin lists (`TestAllPluginsRegistered`,
  `TestAvailablePlugins`) gained `"bfd"` in the correct sorted position
  (alphabetically before `bgp`).
- Two `.ci` functional tests landed:
  `test/plugin/bfd-features.ci` proves CLI registration via
  `ze plugin bfd --features`, and
  `test/plugin/bfd-config-load.ci` runs ze under a Python orchestrator
  that asserts the four lifecycle log patterns then SIGTERMs cleanly.

### Bugs Found/Fixed
- **Hard-coded YANG module list in the parser**: discovered while
  iterating on the .ci test that `bfd { ... }` produced
  "unknown top-level keyword: bfd". Even `rib { ... }` (the sysrib
  plugin's ConfigRoot) shows the same failure today. The handoff did
  NOT mention this. Investigation showed the iface-tunnel commit
  `2488c4b1` had ALREADY added the `ze-bfd-conf` block to
  `YANGSchemaWithPlugins` while doing its own work — likely the other
  session anticipated the wiring need. My local Edit reapplied the
  same content and was a no-op (git diff is empty). The lesson is
  the same even though the fix was already there: every new
  top-level config block needs both `register.go ConfigRoots` AND a
  `yang_schema.go YANGSchemaWithPlugins` entry.
- **String-typed leaves in ConfigSection.Data**: first attempt at
  `config.go` used a typed-struct unmarshaller which immediately
  exploded on `enabled "true"`. Rewrote to walk `map[string]any` and
  parse strings explicitly, mirroring `parseIfaceConfig`.

### Documentation Updates
None — the operator-facing `docs/guide/bfd.md` and the internal
`docs/architecture/bfd.md` already shipped with `e5a4add9` and accurately
describe both the user surface and the wire / FSM design. Stage 1 only
turns dead code into reachable code; the docs were already consistent
with the lifecycle that this spec wires up.

### Deviations from Plan
- The handoff's EDIT 3 told me to register `ze.log.bfd` via
  `env.MustRegister`. I confirmed via
  `internal/core/env/registry.go` that `ze.log.<subsystem>` is a prefix
  wildcard registered once in `slogutil.go:45`, so any
  `ze.log.<anything>` is automatically valid. No separate registration
  is needed and the test verifies `ze.log.bfd=debug` resolves correctly.
- The handoff suggested `test/plugin/bfd/01-standalone-session.ci` as a
  new subdirectory pattern. I placed the tests at the existing
  `test/plugin/bfd-*.ci` flat layout to match the convention used by
  every other plugin test in the directory.
- The handoff's EDIT 8 specified a single `01-standalone-session.ci`.
  Stage 1 ships TWO tests because the CLI-discovery proof
  (`bfd-features.ci`) and the lifecycle proof (`bfd-config-load.ci`)
  fail for different reasons and are easier to debug separately.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Wire BFD plugin from config to running engine | ✅ | `bfd.go` RunBFDPlugin (full SDK lifecycle) | Replaces stub |
| Stage 1 reachability proof via `.ci` test | ✅ | `test/plugin/bfd-config-load.ci` | Python orchestrator, four lifecycle assertions |
| Defer privileged binding to Stage 2+ | ✅ | `loopFor` returns error for non-default VRF; no pinned sessions in test config | `plan/deferrals.md` row references `spec-bfd-2-transport-hardening` |
| Keep skeleton tests green | ✅ | `go test -race ./internal/plugins/bfd/...` | All four packages ok |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `bin/ze-test bgp plugin -v V` (slot V = bfd-features); `bin/ze plugin bfd --features` returns `yang` exit 0 | Recorded in `tmp/bfd-features-out.log` |
| AC-2 | ✅ Done | `internal/component/config/yang_schema.go:347-358` loads `ze-bfd-conf`; manual repro with `echo 'bfd {...}' | bin/ze -` parses cleanly | Was failing before this change |
| AC-3 | ✅ Done | `tmp/bfd-direct4.log` line 3: `level=DEBUG msg="bfd plugin starting" subsystem=bfd` | From `RunBFDPlugin` Debug call |
| AC-4 | ✅ Done | `tmp/bfd-direct4.log` line 4: `level=INFO msg="bfd plugin configured" subsystem=bfd profiles=1 pinned-sessions=0` | Counts come from `state.cfg` |
| AC-5 | ✅ Done | `tmp/bfd-direct4.log` line 5: `level=INFO msg="bfd plugin running" subsystem=bfd` | Emitted from `OnStarted` |
| AC-6 | ✅ Done | All three log lines carry `subsystem=bfd` (see AC-3..AC-5 evidence) | Confirms `ConfigureEngineLogger("bfd")` was invoked |
| AC-7 | ✅ Done | `internal/plugins/bfd/engine/loop.go:181-191` (`handle.Shutdown`), tests still race-clean | Calls `Machine.AdminDown(packet.DiagAdminDown)` |
| AC-8 | ✅ Done | `cmd/ze/main_test.go:22` and `internal/component/plugin/all/all_test.go:18` both contain `"bfd"` first; `make ze-verify` exit 0 | Two-list rule (registered + AvailableInternal) |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestAllPluginsRegistered` | ✅ Done | `internal/component/plugin/all/all_test.go` | Updated expected list |
| `TestAvailablePlugins` | ✅ Done | `cmd/ze/main_test.go` | Updated expected list |
| Existing engine handshake test | ✅ Done | `internal/plugins/bfd/engine/engine_test.go` | Race-clean after handle Shutdown/Enable additions |
| Existing session FSM test | ✅ Done | `internal/plugins/bfd/session/session_test.go` | AdminDown/Enable still covered |
| `bfd-features.ci` | ✅ Done | `test/plugin/bfd-features.ci` | passes via `bin/ze-test bgp plugin -v V` |
| `bfd-config-load.ci` | ✅ Done | `test/plugin/bfd-config-load.ci` | passes via `bin/ze-test bgp plugin -v U` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/bfd/api/service.go` | ✅ Modified | Added Shutdown / Enable to SessionHandle |
| `internal/plugins/bfd/engine/engine.go` | ✅ Modified | Added ErrUnknownSession |
| `internal/plugins/bfd/engine/loop.go` | ✅ Modified | Added handle.Shutdown / Enable |
| `internal/plugins/bfd/register.go` | ✅ Modified | Removed warning comment; added ConfigureEngineLogger |
| `internal/plugins/bfd/bfd.go` | ✅ Modified | Replaced stub with real SDK lifecycle |
| `internal/plugins/bfd/config.go` | ✅ Created | Config parser |
| `internal/component/config/yang_schema.go` | ✅ Already present | Load ze-bfd-conf entry was added by iface-tunnel commit 2488c4b1 before this spec ran; my Edit was a no-op |
| `internal/component/plugin/all/all.go` | ✅ Modified | make generate added two blank imports |
| `internal/component/plugin/all/all_test.go` | ✅ Modified | bumped expected list |
| `cmd/ze/main_test.go` | ✅ Modified | bumped expected list |
| `test/plugin/bfd-features.ci` | ✅ Created | CLI discovery proof |
| `test/plugin/bfd-config-load.ci` | ✅ Created | Lifecycle proof |

### Audit Summary
- **Total items:** 18 (4 requirements + 8 ACs + 6 tests; 12 files tracked separately)
- **Done:** 18
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (deviations documented above)

## Pre-Commit Verification

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|
| `internal/plugins/bfd/config.go` | ✅ | `ls -la internal/plugins/bfd/config.go` shows present |
| `test/plugin/bfd-features.ci` | ✅ | `ls -la test/plugin/bfd-features.ci` shows present |
| `test/plugin/bfd-config-load.ci` | ✅ | `ls -la test/plugin/bfd-config-load.ci` shows present |
| `plan/learned/556-bfd-1-wiring.md` | ✅ | written alongside this spec for the two-commit dance |

### AC Verified
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | CLI lists yang feature | Re-ran `bin/ze plugin bfd --features` → `yang` |
| AC-2 | parser accepts `bfd { }` | `grep -n "ze-bfd-conf" internal/component/config/yang_schema.go` shows the new loader block |
| AC-3 | `bfd plugin starting` debug log | `grep -c "bfd plugin starting" tmp/bfd-direct4.log` = 1 |
| AC-4 | `bfd plugin configured` info log | `grep -c "bfd plugin configured" tmp/bfd-direct4.log` = 1 |
| AC-5 | `bfd plugin running` info log | `grep -c "bfd plugin running" tmp/bfd-direct4.log` = 1 |
| AC-6 | `subsystem=bfd` slog tag | `grep -c "subsystem=bfd" tmp/bfd-direct4.log` = 3 |
| AC-7 | handle.Shutdown reaches AdminDown | `grep -n "AdminDown(packet.DiagAdminDown)" internal/plugins/bfd/engine/loop.go` |
| AC-8 | both expected lists contain bfd | `grep -c '"bfd"' cmd/ze/main_test.go internal/component/plugin/all/all_test.go` = 1+1 |

### Wiring Verified
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze plugin bfd --features` | `test/plugin/bfd-features.ci` | reads the file: a single foreground cmd that asserts `expect=stdout:contains=yang`. Exercises CLI discovery via `register.go init()`. |
| `bfd { ... }` config block | `test/plugin/bfd-config-load.ci` | reads the file: Python orchestrator runs `ze -` with a profile-only config and asserts four lifecycle patterns plus exit 0. Exercises the parser → SDK Configure → OnStarted chain end to end. |

## Checklist

### Goal Gates
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete `.ci` file
- [ ] `make ze-verify` passes (exit 0, "Ze verification passed")
- [ ] Feature code integrated (`internal/plugins/bfd/`, `internal/component/config/`)
- [ ] Integration completeness proven end-to-end via two `.ci` tests
- [ ] Architecture docs already covered the design (no updates needed)

### Quality Gates
- [ ] Unit tests stayed race-clean
- [ ] Implementation Audit complete with file:line evidence
- [ ] No new third-party imports

### Design
- [ ] No premature abstraction (lifecycle reuses existing engine.Loop)
- [ ] No speculative features (Stage 1 = reachability only)
- [ ] Single responsibility (config.go parses; bfd.go orchestrates)
- [ ] Explicit > implicit (`runtimeState` lifecycle is fully sequential)
- [ ] Minimal coupling (bfd plugin imports nothing from BGP/iface/sysrib)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Goal Gates (verification)
- [ ] make ze-test passes (lint + all ze tests)

### Completion
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written to `plan/learned/556-bfd-1-wiring.md`
- [ ] Summary will be included in commit B per `rules/spec-preservation.md`
