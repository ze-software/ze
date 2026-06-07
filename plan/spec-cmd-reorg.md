# Spec: cmd-reorg

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 4/4 |
| Updated | 2026-06-07 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/plugin-self-containment.md` - the removal test
4. `ai/rules/derive-not-hardcode.md` - no static help strings
5. `ai/patterns/registration.md` - registration architecture

## Task

Move domain logic out of `cmd/ze/` into self-registering `internal/` packages,
using the project's existing `subdispatch.Dispatcher` pattern (see `cmd/ze/install/`,
`internal/core/subdispatch/`). The unified binary refactor (commit `eac6ec186`)
folded all binaries under one `main()` but copied code into `cmd/ze/` verbatim
instead of placing it where Ze's architecture says it belongs: each feature is a
package that registers itself from its own folder.

**Target pattern** (extends install/uninstall; install lives in `cmd/ze/install/` but domain code belongs in `internal/`):
- `internal/<feature>/dispatch.go` - creates `subdispatch.Dispatcher`, exports `Register`, `Dispatch`
- `internal/<feature>/register.go` - `init()` calls `registry.MustRegisterRootHandler` + registers subcommands
- `internal/<feature>/<subcommand>.go` - domain code, registers via `init()` or populate function
- `cmd/ze/ze_<feature>_register.go` - build-tagged, blank import only (`import _ "internal/<feature>"`)

Note: install lives under `cmd/ze/install/` because it predates this convention. This spec places new packages in `internal/` for consistency with Ze's domain/wiring separation.

**Four phases, each independently committable (ordered by risk: clean wins first):**

1. Move analyze domain code to self-registering `internal/analyze/` with `subdispatch.Dispatcher`
2. Move test mock servers to `internal/test/mock/`
3. Move perf to self-registering `internal/perf/` with `subdispatch.Dispatcher` (report/track already done)
4. Move chaos orchestrator logic to `internal/chaos/orchestrator/` (highest risk: tight coupling, config struct needed)

Phase 5 (registry dispatch) from the original design is absorbed into Phases 1 and 3:
`subdispatch.Dispatcher` replaces the static switch/case AND derives help text.

**`binarySetup` lifecycle:** Phases 1 and 3 eliminate `binarySetup` for analyze and perf (init()-based registration makes it unnecessary; `dispatch.go:143` handles nil). The `binarySetup` variable and its dispatch.go plumbing survive this spec because ze-test and ze-chaos still use it.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugin-self-containment.md` - the removal test for feature surfaces
  -> Constraint: domain logic belongs to the owner package, cmd/ holds only CLI wiring
- [ ] `ai/rules/derive-not-hardcode.md` - usage/help text derived from registries
  -> Constraint: no second hardcoded copy of enumerated data
- [ ] `ai/patterns/registration.md` - all Ze registries and how they compose
  -> Decision: subcommands register via a registry, not static switches
- [ ] `ai/rules/file-modularity.md` - one concern per file, >600 needs review
  -> Constraint: split if over 600 lines
- [ ] `docs/architecture/core-design.md` - where cmd vs internal boundary sits
  -> Decision: cmd/ is thin CLI wiring; internal/ is domain logic

**Key insights:**
- cmd/ files should contain only build-tagged blank imports that pull in self-registering internal/ packages
- Help text is derived by `subdispatch.Dispatcher.usage()` from registered subcommands (already implemented in `internal/core/subdispatch/`)
- Each feature's code lives in the package that owns it; removing the package removes the feature
- Existing pattern to follow: `cmd/ze/install/` + `cmd/ze/setup_features_setup.go` + `internal/core/subdispatch/`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/ze_analyze_register.go` - static switch dispatch, hardcoded usage
  -> Constraint: `zeAnalyzeRootHandler` uses switch/case, `zeAnalyzeUsage` is a string literal
- [ ] `cmd/ze/ze_analyze_mrt.go` - 526L MRT wire parser, RFC 6396, stdlib only
- [ ] `cmd/ze/ze_analyze_mrt_test.go` - 1015L MRT parsing tests (bulk of analyze test surface)
- [ ] `cmd/ze/ze_analyze_density.go` - 596L analysis logic, stdlib only
- [ ] `cmd/ze/ze_analyze_attributes.go` - 589L analysis logic, stdlib only
- [ ] `cmd/ze/ze_analyze_communities.go` - 417L community analysis, stdlib only
- [ ] `cmd/ze/ze_analyze_aspath.go` - 398L AS path analysis, stdlib only
- [ ] `cmd/ze/ze_analyze_count.go` - 79L attribute count, stdlib only
- [ ] `cmd/ze/ze_analyze_dump.go` - 61L MRT dump, stdlib only
- [ ] `cmd/ze/ze_analyze_download.go` - 196L MRT download, stdlib only
- [ ] `cmd/ze/ze_perf_register.go` - static switch dispatch, hardcoded usage
  -> Constraint: `zePerfRootHandler` uses switch/case, `zePerfUsage` is a string literal
- [ ] `cmd/ze/ze_perf_run.go` - 273L flag parsing + result formatting (printPerfHumanResult, writePerfJSONResult)
- [ ] `cmd/ze/ze_perf_report.go` - 86L, already thin wrapper calling internal/perf/report
- [ ] `cmd/ze/ze_perf_track.go` - 120L, already thin wrapper calling internal/perf/report
- [ ] `cmd/ze/ze_perf_test.go` - 142L, tests for perf subcommands
- [ ] `cmd/ze/ze_test_register.go` - map-based dispatch, derived usage (precursor to subdispatch pattern)
  -> Decision: uses local map + `zeTestAdd()`, not yet `subdispatch.Dispatcher`; full migration is a follow-up
- [ ] `cmd/ze/ze_test_cymru.go` - 123L DNS mock server implementation in cmd/
- [ ] `cmd/ze/ze_test_irr.go` - 89L whois mock server implementation in cmd/
- [ ] `cmd/ze/ze_test_rpki.go` - 186L RPKI mock server in cmd/
- [ ] `cmd/ze/ze_test_rtr_mock.go` - 275L RTR protocol mock in cmd/
- [ ] `cmd/ze/ze_test_tacacs_mock.go` - 327L TACACS+ mock (RFC 8907) in cmd/
- [ ] `cmd/ze/ze_test_peeringdb.go` - 125L PeeringDB HTTP mock in cmd/
- [ ] `cmd/ze/ze_test_helpers.go` - 37L shared test helper (findBaseDir); stays in cmd/ (used by runners)
- [ ] `cmd/ze/ze_chaos_run.go` - 937L orchestrator + CLI wiring
- [ ] `cmd/ze/ze_chaos_orchestrator_run.go` - 607L orchestration logic
- [ ] `cmd/ze/ze_chaos_orchestrator.go` - 189L types (ChaosConfig, RouteConfig, orchestratorConfig, establishedState, EventProcessor)
- [ ] `cmd/ze/ze_chaos_conflict.go` - 171L listener conflict validation (uses config.ListenerEndpoint)
- [ ] `cmd/ze/ze_chaos_conflict_test.go` - 174L tests for conflict validation
- [ ] `cmd/ze/ze_chaos_fork.go` - 182L process fork/management
- [ ] `cmd/ze/ze_chaos_scheduler.go` - 296L scheduling logic
- [ ] `cmd/ze/ze_chaos_subcommand.go` - 238L replay/shrink/diff actions + network utilities (flag-driven, not subdispatch subcommands)
- [ ] `cmd/ze/ze_chaos_orchestrator_test.go` - 275L orchestrator tests
- [ ] `cmd/ze/ze_chaos_main_test.go` - 323L integration tests

**Behavior to preserve:**
- All CLI interfaces (flags, output format, exit codes)
- Binary sizes (no new deps pulled in)
- Build tag isolation (ze_test, ze_chaos, ze_perf, ze_analyze)
- Test behavior and expectations
- Multi-call dispatch: ze-test/ze-chaos/ze-perf/ze-analyze binary names

**Behavior to change:**
- Static switch dispatch -> `subdispatch.Dispatcher` for perf and analyze (self-registering from internal/)
- Hardcoded usage strings -> derived by `subdispatch.Dispatcher.usage()` from registered subcommands
- Domain code location: cmd/ze/ -> internal/
- cmd/ze register files become blank imports (like install/uninstall pattern)
- `binarySetup` eliminated for analyze and perf (registration via init() blank import)

## Data Flow (MANDATORY)

### Entry Point
- User runs `ze-analyze density file.gz` or `ze-perf run --dut-addr ...` or `ze-test bgp -a`
- Multi-call dispatch prepends personality prefix, looks up root handler

### Transformation Path
1. `dispatch.go:dispatchMain` -> multi-call prefix -> `defaultDispatch` -> root command lookup in registry
2. Root handler (registered by `internal/<feature>/register.go` init()) -> `subdispatch.Dispatcher.Dispatch()`
3. Dispatcher looks up subcommand in registered handlers -> calls `Run<Name>(args)`
4. Domain logic returns exit code

Note: `binarySetup` becomes unnecessary for analyze/perf (root handler is registered via init() blank import).
For chaos, `binarySetup` may still be needed if the handler needs setup beyond registration.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| cmd/ze -> internal/ | Blank import triggers init() registration | [ ] |
| CLI flags -> domain config | Self-contained in internal/ (analyze) or struct construction (perf/chaos) | [ ] |

### Integration Points
- `internal/analyze/` self-registers via `registry.MustRegisterRootHandler` + `subdispatch.Dispatcher`; cmd/ze blank-imports it
- `internal/perf/` already called from `cmd/ze/ze_perf_run.go`; Dispatcher registration added
- `internal/test/mock/` functions called from `cmd/ze/ze_test_register.go` via `zeTestAdd()` (full self-registration is a follow-up)
- `internal/chaos/orchestrator/` called from `cmd/ze/ze_chaos_run.go` (no subcommands, no Dispatcher needed)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze-analyze density file.gz` | -> | `internal/analyze.Dispatch()` -> `runDensity()` | existing `ze_analyze_mrt_test.go` (moved) |
| `ze-perf run --dut-addr ...` | -> | `internal/perf.Dispatch()` -> `perf.RunBenchmark()` | existing `ze_perf_test.go` |
| `ze-test cymru --port 0` | -> | `internal/test/mock/cymru.Run()` | existing .ci tests via ze-test |
| `ze-chaos --peers 4` | -> | `internal/chaos/orchestrator.Run()` | existing `ze_chaos_main_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `go build -tags ze_analyze -o bin/ze-analyze ./cmd/ze/` | Builds, binary size within 10% of current |
| AC-2 | `go test -tags ze_analyze ./cmd/ze/ ./internal/analyze/` | All tests pass |
| AC-3 | `go build -tags ze_perf -o bin/ze-perf ./cmd/ze/` | Builds, size unchanged |
| AC-4 | `go test -tags ze_perf ./cmd/ze/ ./internal/perf/...` | All tests pass |
| AC-5 | `go build -tags ze_test -o bin/ze-test ./cmd/ze/` | Builds, size unchanged |
| AC-6 | `go test -tags ze_test ./cmd/ze/` | All tests pass |
| AC-7 | `go build -tags ze_chaos -o bin/ze-chaos ./cmd/ze/` | Builds, size unchanged |
| AC-8 | `go test -tags ze_chaos ./cmd/ze/` | All tests pass |
| AC-9 | `ze-analyze help` | Usage derived by `subdispatch.Dispatcher.usage()` from registered subcommands |
| AC-10 | `ze-perf help` | Usage derived by `subdispatch.Dispatcher.usage()` (same pattern as AC-9) |
| AC-11 | `cmd/ze/ze_analyze_register.go` | Build-tagged blank import of `internal/analyze` only (no domain code, no switch, no handler) |
| AC-11b | `cmd/ze/ze_perf_register.go` | Build-tagged blank import of `internal/perf` only (same pattern as AC-11) |
| AC-12 | `cmd/ze/ze_test_{cymru,irr,rpki,rtr_mock,tacacs_mock,peeringdb}.go` | Only thin wrappers calling `internal/test/mock/<name>/` (syslog excluded: already wraps `internal/test/syslog`) |
| AC-13 | `cmd/ze/ze_chaos_{orchestrator_run,orchestrator,conflict,fork,scheduler,subcommand}.go` deleted | Logic + types live in `internal/chaos/orchestrator/`; tests move with code |
| AC-14 | `go build -o bin/ze ./cmd/ze/` (default, no tags) | Builds, no regressions |
| AC-15 | All 8 variant builds pass | Build + test clean |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Existing MRT tests | `internal/analyze/mrt_test.go` (moved) | MRT parsing unchanged | |
| Existing perf tests | `internal/perf/*_test.go` (unchanged) | Perf logic unchanged | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing .ci tests | `test/` | All test runner behaviors preserved | |

## Files to Modify

### Phase 1: Analyze -> self-registering internal/analyze/
- `cmd/ze/ze_analyze_register.go` - becomes blank import: `import _ "internal/analyze"` (no binarySetup, no handler, no switch)
- `cmd/ze/ze_analyze_mrt.go` -> DELETE (moved to `internal/analyze/`)
- `cmd/ze/ze_analyze_density.go` -> DELETE (moved)
- `cmd/ze/ze_analyze_attributes.go` -> DELETE (moved)
- `cmd/ze/ze_analyze_communities.go` -> DELETE (moved)
- `cmd/ze/ze_analyze_aspath.go` -> DELETE (moved)
- `cmd/ze/ze_analyze_count.go` -> DELETE (moved)
- `cmd/ze/ze_analyze_dump.go` -> DELETE (moved)
- `cmd/ze/ze_analyze_download.go` -> DELETE (moved)
- `cmd/ze/ze_analyze_mrt_test.go` -> DELETE (moved)

### Phase 2: Test mocks -> internal/test/mock/
- `cmd/ze/ze_test_cymru.go` - thin wrapper
- `cmd/ze/ze_test_irr.go` - thin wrapper
- `cmd/ze/ze_test_rpki.go` - thin wrapper
- `cmd/ze/ze_test_rtr_mock.go` - thin wrapper
- `cmd/ze/ze_test_tacacs_mock.go` - thin wrapper
- `cmd/ze/ze_test_peeringdb.go` - thin wrapper
- NOT `ze_test_syslog.go` - already wraps `internal/test/syslog`, no extraction needed
- NOT `ze_test_helpers.go` - shared by runners (stays in cmd/)

### Phase 3: Perf -> self-registering internal/perf/ with subdispatch
- `cmd/ze/ze_perf_register.go` - becomes blank import: `import _ "internal/perf"` (no binarySetup)
- `cmd/ze/ze_perf_run.go` -> DELETE (moved to `internal/perf/cli/cmd_run.go`)
- `cmd/ze/ze_perf_report.go` -> DELETE (already thin, moves to `internal/perf/cli/cmd_report.go`)
- `cmd/ze/ze_perf_track.go` -> DELETE (already thin, moves to `internal/perf/cli/cmd_track.go`)
- `cmd/ze/ze_perf_test.go` -> DELETE (moved)
- `internal/perf/cli/dispatch.go` - creates `subdispatch.Dispatcher`, exports Dispatch
- `internal/perf/cli/register.go` - init() registers root handler + populates subcommands
- NOTE: `printPerfHumanResult` and `writePerfJSONResult` moved to `internal/perf/report/human.go` as `report.PrintHuman` and `report.WriteJSON`. CLI exit-code logic stays in `cmd_run.go` as `writeJSONResult`.
- NOTE: Perf dispatch/register/cmd files live in `internal/perf/cli/` (not `internal/perf/`) to avoid an import cycle: `perf` -> `perf/report` -> `perf`. The `cli` sub-package imports both `perf` and `perf/report` without creating a cycle.

### Phase 4: Chaos orchestrator -> internal/chaos/orchestrator/
- `cmd/ze/ze_chaos_run.go` - keeps flag parsing + config struct construction, calls `orchestrator.Run(cfg)`
- `cmd/ze/ze_chaos_orchestrator_run.go` -> DELETE (moved)
- `cmd/ze/ze_chaos_orchestrator.go` -> DELETE (types + EventProcessor moved)
- `cmd/ze/ze_chaos_conflict.go` -> DELETE (listener validation moved)
- `cmd/ze/ze_chaos_conflict_test.go` -> DELETE (moved with code)
- `cmd/ze/ze_chaos_fork.go` -> DELETE (moved)
- `cmd/ze/ze_chaos_scheduler.go` -> DELETE (moved)
- `cmd/ze/ze_chaos_subcommand.go` -> DELETE (moved)
- `cmd/ze/ze_chaos_orchestrator_test.go` -> DELETE (moved with code)
- `cmd/ze/ze_chaos_main_test.go` - stays (integration test for cmd/ wiring)

**Config struct bridge (Phase 4 prerequisite):** `ze_chaos_run.go` (937L) interleaves flag parsing with orchestrator setup, web dashboard wiring, and scenario generation. The types it constructs (`ChaosConfig`, `RouteConfig`, `orchestratorConfig`) live in `ze_chaos_orchestrator.go` and must move to `internal/chaos/orchestrator/`. The bridge is: `ze_chaos_run.go` parses flags, constructs an exported `orchestrator.RunConfig` (aggregating all the existing config types), and calls `orchestrator.Run(cfg)`. The `RunConfig` struct design must be settled during implementation; it wraps the existing types plus the flag-derived values currently threaded through local variables (web addr, metrics addr, pprof addr, mcp addr, event log path, quiet/verbose/debug flags).

### Phase 5: REMOVED (absorbed into Phases 1 and 3)
- `subdispatch.Dispatcher` in Phases 1 and 3 replaces static switch dispatch AND derives usage text
- ze-test already uses a map with derived usage; no change needed

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | No protocol changes |
| CLI commands/flags | No | Same interfaces |
| Documentation Update Checklist | Yes | Source anchors |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 12 | Internal architecture changed? | Yes | `docs/architecture.md` source layout table |
| 16 | Source anchors changed? | Yes | Grep for moved file paths |

## Files to Create

### Phase 1
- `internal/analyze/dispatch.go` - `subdispatch.New("analyze", "...")`, exports Register/Dispatch (like install pattern)
- `internal/analyze/register.go` - `init()` registers root handler via `registry.MustRegisterRootHandler` + populates subcommands
- `internal/analyze/mrt.go` - MRT parsing (from ze_analyze_mrt.go, exported)
- `internal/analyze/mrt_test.go` - MRT tests (from ze_analyze_mrt_test.go)
- `internal/analyze/density.go` - density analysis (registered via dispatcher)
- `internal/analyze/attributes.go` - attribute analysis
- `internal/analyze/communities.go` - community analysis
- `internal/analyze/aspath.go` - AS path analysis
- `internal/analyze/count.go` - attribute count
- `internal/analyze/dump.go` - MRT dump
- `internal/analyze/download.go` - MRT download

### Phase 2
- `internal/test/mock/cymru/cymru.go` - Cymru DNS mock
- `internal/test/mock/irr/irr.go` - IRR whois mock
- `internal/test/mock/rpki/rpki.go` - RPKI mock
- `internal/test/mock/rtr/rtr.go` - RTR mock
- `internal/test/mock/tacacs/tacacs.go` - TACACS+ mock
- `internal/test/mock/peeringdb/peeringdb.go` - PeeringDB mock
- NOT syslog: already lives in `internal/test/syslog/`

### Phase 3
- `internal/perf/cli/dispatch.go` - `subdispatch.New("perf", "...")`, exports Register/Dispatch
- `internal/perf/cli/register.go` - `init()` registers root handler + populates run/report/track subcommands
- `internal/perf/cli/cmd_run.go` - run subcommand (from ze_perf_run.go, flag parsing + benchmark invocation)
- `internal/perf/cli/cmd_report.go` - report subcommand (from ze_perf_report.go)
- `internal/perf/cli/cmd_track.go` - track subcommand (from ze_perf_track.go)
- `internal/perf/report/human.go` - `printPerfHumanResult` + `writePerfJSONResult` (from ze_perf_run.go)

### Phase 4
- `internal/chaos/orchestrator/run.go` - orchestrator entry point + `RunConfig` struct
- `internal/chaos/orchestrator/types.go` - ChaosConfig, RouteConfig, orchestratorConfig, establishedState
- `internal/chaos/orchestrator/event.go` - EventProcessor, isLifecycleEvent
- `internal/chaos/orchestrator/conflict.go` - listener conflict validation
- `internal/chaos/orchestrator/conflict_test.go` - conflict tests
- `internal/chaos/orchestrator/fork.go` - process management
- `internal/chaos/orchestrator/scheduler.go` - scheduling
- `internal/chaos/orchestrator/subcommand.go` - replay/shrink/diff
- `internal/chaos/orchestrator/orchestrator_test.go` - orchestrator tests

## Implementation Steps

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase 1: Analyze extraction** -- self-registering `internal/analyze/` with `subdispatch.Dispatcher`
   - Create `internal/analyze/dispatch.go` (like `cmd/ze/install/dispatch.go`) with `subdispatch.New()`
   - Create `internal/analyze/register.go` with `init()` that registers root handler + subcommands
   - Move domain code: MRT parser types/functions become exported, each `run<Name>` becomes `cmd<Name>`
   - Move tests alongside code
   - `cmd/ze/ze_analyze_register.go` becomes blank import: `import _ "internal/analyze"` (no binarySetup)
   - Delete all `cmd/ze/ze_analyze_*.go` except the register file
   - `internal/analyze/` has NO build tag (importable by any binary)
   - Flag parsing stays inside subcommand functions (only `communities` and `download` use flags; they are self-contained tools)
   - Verify: `go build -tags ze_analyze`, `go test -tags ze_analyze ./cmd/ze/ ./internal/analyze/`

2. **Phase 2: Test mock extraction** -- move mock servers to `internal/test/mock/<name>/`
   - Each mock becomes a package with `func Run(args []string) int`
   - `ze_test_<name>.go` becomes: `func zeTestCymruCmd(args []string) int { return cymru.Run(args) }`
   - Verify: `go build -tags ze_test`, `go test -tags ze_test ./cmd/ze/`

3. **Phase 3: Perf extraction** -- self-registering `internal/perf/` with `subdispatch.Dispatcher`
   - Create `internal/perf/cli/dispatch.go` with `subdispatch.New()` (like analyze)
   - Create `internal/perf/cli/register.go` with `init()` that registers root handler + run/report/track subcommands
   - Move `ze_perf_run.go` -> `internal/perf/cli/cmd_run.go` (flag parsing + benchmark invocation)
   - Move `ze_perf_report.go` -> `internal/perf/cli/cmd_report.go` (already thin)
   - Move `ze_perf_track.go` -> `internal/perf/cli/cmd_track.go` (already thin)
   - Extract `printPerfHumanResult` + `writePerfJSONResult` to `internal/perf/report/human.go` (check existing report/ exports first)
   - `cmd/ze/ze_perf_register.go` becomes blank import (no binarySetup)
   - Delete all `cmd/ze/ze_perf_*.go` except the register file
   - Verify: `go build -tags ze_perf`, `go test -tags ze_perf ./cmd/ze/ ./internal/perf/...`

4. **Phase 4: Chaos orchestrator extraction** -- move orchestrator to `internal/chaos/orchestrator/`
   - Design `RunConfig` struct: wraps existing `ChaosConfig` + `RouteConfig` + service addrs + output flags
   - Move types first (`ze_chaos_orchestrator.go` types -> `internal/chaos/orchestrator/types.go`)
   - Move run logic, fork management, scheduler, replay/shrink/diff, event processor, conflict validation
   - Move `ze_chaos_orchestrator_test.go` and `ze_chaos_conflict_test.go` alongside code
   - Keep `ze_chaos_main_test.go` in cmd/ze (integration test for wiring)
   - `ze_chaos_run.go` keeps flag parsing + `RunConfig` construction, calls `orchestrator.Run(cfg)`
   - Verify: `go build -tags ze_chaos`, `go test -tags ze_chaos ./cmd/ze/ ./internal/chaos/orchestrator/`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every moved file has its replacement in internal/ |
| Correctness | All function signatures preserved, all tests pass |
| Naming | Exported names follow Go conventions |
| Data flow | cmd/ only does blank imports; domain code self-registers from internal/ |
| Rule: no-layering | Old code in cmd/ fully deleted after move |
| Rule: derive-not-hardcode | Usage text derived by `subdispatch.Dispatcher` from registered subcommands |
| Pattern: self-registration | Each feature package registers via init(); cmd/ file is blank import only |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `internal/analyze/` self-registers | `ls internal/analyze/{dispatch,register,mrt,density,attributes,communities,aspath,count,dump,download}.go` |
| `internal/test/mock/` packages exist | `ls internal/test/mock/*/` |
| `internal/chaos/orchestrator/` exists | `ls internal/chaos/orchestrator/*.go` |
| `internal/perf/cli/` self-registers | `ls internal/perf/cli/{dispatch,register,cmd_run,cmd_report,cmd_track}.go` |
| All 4 builds pass | Build loop over all tags |
| All tests pass | Test loop over all tags |
| cmd/ze register files are blank imports | `wc -l cmd/ze/ze_{analyze,perf}_register.go` shows ~10 lines each (down from 89/66) |
| No domain code in cmd/ze | `grep -c 'func ' cmd/ze/ze_analyze_*.go cmd/ze/ze_perf_*.go` shows only register files |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| No new attack surface | Pure code move, no new inputs |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `internal/analyze/` not `internal/mrt/` | Could split MRT parser from analysis | Analysis tools are the only MRT consumer; keeping together avoids premature abstraction |
| Use `subdispatch.Dispatcher` | Roll own map + usage | Project already has `internal/core/subdispatch/` used by install/uninstall; reuse avoids divergent patterns |
| Self-registering packages via init() | Thin wrappers calling internal/ | Matches install/uninstall pattern; removes all domain code from cmd/; blank import is the only coupling |
| Build tags stay on cmd/ze files only | Could tag internal/ packages too | internal/ packages should be importable without tags; tags gate binary composition not library availability |
| Phase-by-phase commits | Single big-bang commit | Each phase is independently verifiable and revertible |
| Flag parsing location varies by feature | Uniform pattern | Perf uses config-struct construction (flag parsing in cmd_run.go). Analyze subcommands are self-contained tools where flag parsing is inseparable from domain logic. Both live in internal/ after the move. |
| ze-test runners (bgp, editor, web, peer) NOT moved | Could move all to internal/ | They contain significant CLI-specific logic (build management, signal handling) that straddles cmd/internal boundary. Follow-up spec. |
| Mock servers moved, test tools NOT moved | Move everything | Mocks are standalone servers (listen + respond) with no test-runner logic. Test tools (mcp, peer, l2tp-scale) orchestrate test execution. Boundary: if it only listens on a port and replies, it's a mock. |
| One package per mock (`internal/test/mock/<name>/`) | Single `mock` package with one file per mock | Each mock has distinct deps (dns, net, http); separate packages keep import graphs clean and match syslog precedent (`internal/test/syslog/`) |
| Syslog excluded from Phase 2 | Could move it | Already wraps `internal/test/syslog`; extraction already done. |
| `ze_test_helpers.go` stays in cmd/ | Could move to internal/ | Only used by test runners which stay in cmd/. |

## Known Limitations
- Phase 4 (chaos orchestrator) is the hardest due to tight coupling between CLI flags and orchestrator state. `ze_chaos_run.go` (937L) interleaves flag parsing with orchestrator setup, web dashboard wiring, and scenario generation. A `RunConfig` struct bridges cmd/ and internal/; its design is settled during implementation (see Phase 4 in Files to Modify). Also has the most files (10 including tests, ~3400L total).
- `ze_chaos_run.go` imports 10 `internal/chaos/*` packages. Moving orchestrator logic does not reduce this import fan-out; it changes which package owns the code, not the dependency graph.
- The ze-test runners (bgp, editor, web, peer, exabgp) are NOT moved in this spec. They contain significant CLI-specific logic that straddles cmd/internal. Follow-up spec.
- ze-test tools (mcp, peer, l2tp-scale) are NOT moved. They orchestrate test execution rather than acting as standalone mock servers.
- ze-chaos has one monolithic command (no subcommands), so derive-not-hardcode for usage text does not apply to it. Its usage is inherently static. The "subcommand" file (`ze_chaos_subcommand.go`) contains replay/shrink/diff actions dispatched by flags, not `subdispatch` subcommands.
- `internal/test/mock/` packages have no build tags (intentional: importable without tags). The build tag on the cmd/ze wrapper file prevents compilation into untagged binaries. `go test ./internal/...` will compile them, which is acceptable since the deps (e.g., `github.com/miekg/dns`) are already in go.mod.
- Perf dispatch/register/cmd files live in `internal/perf/cli/` (not `internal/perf/`) because `perf/report` imports `perf` for the `Result` type. Placing cmd files in `perf` would create `perf` -> `perf/report` -> `perf`. The `cli` sub-package breaks the cycle.
- `internal/perf/report/human.go` added with `PrintHuman` and `WriteJSON`. No collisions with existing exports (HTML, Markdown, PerformanceDoc, Trend).
- ze-test subcommand self-registration (each mock package registers itself via init() into a shared registry) is a follow-up. This spec moves mock domain code to internal/ but registration still goes through `zeTestAdd()` in cmd/ze.
