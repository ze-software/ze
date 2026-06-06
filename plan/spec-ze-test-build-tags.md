# Spec: ze-test-build-tags

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/7 |
| Updated | 2026-06-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/cli/plugin-modes.md` - build-tag variant architecture
4. `docs/architecture/testing/ci-format.md` - test runner architecture
5. `cmd/ze/main.go` - ze dispatch and import structure
6. `cmd/ze-test/main.go` - ze-test dispatch table
7. `scripts/codegen/plugin_imports.go` - existing codegen pattern

## Task

Build ze-test as a build-tag variant of `cmd/ze` instead of a separate `cmd/ze-test` binary. Unify the dispatch pattern so ze-test subcommands register via `command/registry.MustRegisterRootHandler`. Use `go:generate` to produce different import files for the ze-test variant that exclude `plugin/all` and other unnecessary dependencies, achieving a minimal binary. The `ze-test` binary name remains a Makefile alias.

Two phases:
1. **Unify dispatch:** Move ze-test subcommands to register via ze's registry, guarded by `//go:build ze_test`. Delete `cmd/ze-test/`.
2. **Minimal binary via codegen:** Extend the codegen system to produce build-tag-guarded import files so `ze_test` builds exclude `plugin/all` and unneeded daemon infrastructure.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/cli/plugin-modes.md` - build-tag variant system for ze
  → Decision: Build-tag files in cmd/ze/ control which features compile in. Each tag gets a `setup_features_<name>.go` with blank imports.
  → Constraint: Build tags use underscore convention: `ze_distro`, `ze_appliance`, `ze_setup`.
- [ ] `docs/architecture/testing/ci-format.md` - test runner architecture
  → Constraint: Test subcommands use `internal/test/runner` for .ci suites and `internal/test/peer` for BGP peer simulation.

**Key insights:**
- Build-tag variant pattern: `setup_features_*.go` with `//go:build ze_<name>` and blank imports
- Existing `zetest` tag (no underscore) is for test plugins in ze DUT, distinct from the new `ze_test` tag
- `plugin_imports.go` codegen already scans for `register.go` files and generates blank-import aggregators
- `editor.go` in ze-test imports `plugin/all` for YANG schema registration needed by editor testing
- `RootHandler` type is `func(rctx *RuntimeContext, args []string) int`; current ze-test handlers use `func() int` reading `os.Args` directly. Each handler must be adapted.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/main.go` (1207L) - ze entry point; dispatches via YANG verbs, `dispatchRegisteredRoot()`, `registry.LookupLocal()`. Unconditionally imports `plugin/all` (line 47). Has ~30 blank imports for CLI command owners, AAA backends.
  → Constraint: `plugin/all` is imported at the binary entry point to avoid import cycles (format -> plugin -> all -> bgp-rs -> format).
- [ ] `cmd/ze/setup_features_distro.go` (14L) - `//go:build ze_distro`: imports connect, local, systemd plugins + install/uninstall commands.
- [ ] `cmd/ze/setup_features_appliance.go` (10L) - `//go:build ze_appliance`: empty body, reserved for on-device runtime features.
- [ ] `cmd/ze/setup_features_setup.go` (12L) - `//go:build ze_setup`: imports install, appliance, provision.
- [ ] `cmd/ze/plugins_zetest.go` (10L) - `//go:build zetest`: imports `internal/test/plugins/all` for DUT test plugins.
- [ ] `cmd/ze-test/main.go` (87L) - Own dispatch table: `map[string]subcommand{}` with `register(name, desc, handler)` pattern. Shifts os.Args before dispatch. 25 subcommands across 28 source files.
- [ ] `cmd/ze-test/ci_runner.go` (149L) - Shared CI runner logic. Creates `runner.NewEncodingTests()`, discovers .ci files, runs via `runner.NewRunner()`. Used by 15 CI test runner subcommands.
- [ ] `cmd/ze-test/editor.go` (200L) - Editor test runner. Imports `plugin/all` (only ze-test file that does). Needs YANG schemas for `editortesting.RunETFile()`.
- [ ] `cmd/ze-test/peer.go` (8.9K) - BGP test peer (sink/echo/check/inject modes). Imports `internal/test/peer`, `internal/core/env`.
- [ ] `cmd/ze-test/bgp.go` (18K) - Largest subcommand. CI runner + inline DUT server + peer modes (parse, encode, decode, plugin tests).
- [ ] `internal/component/plugin/all/gen.go` (5L) - `//go:generate go run ../../../../scripts/codegen/plugin_imports.go`
- [ ] `internal/component/plugin/all/all.go` (253L) - Generated. Blank imports for ~84 plugins, ~111 schemas, RPC packages, event namespaces.
- [ ] `scripts/codegen/plugin_imports.go` (501L) - Scans plugin directories for `register.go`, generates `all.go`. Supports `--check` mode.
- [ ] `internal/component/command/registry/registry.go` - `RootHandler = func(rctx *RuntimeContext, args []string) int`. `MustRegisterRootHandler(name, handler, Meta)`. `LookupRoot(name) RootHandler`.

**ze-test subcommand registry (25 subcommands):**

| Category | Name | File | Key imports from internal/ |
|----------|------|------|---------------------------|
| CI runner | bgp | bgp.go | test/runner, test/peer |
| CI runner | editor | editor.go | component/cli/testing, **plugin/all**, test/runner |
| CI runner | exabgp | exabgp.go | test/runner |
| CI runner | firewall | firewall.go | test/runner |
| CI runner | flow-export | flowexport.go | test/runner |
| CI runner | install | install.go | test/runner |
| CI runner | l2tp | l2tp.go | test/runner |
| CI runner | l2tp-wire | l2tp_wire.go | test/runner |
| CI runner | managed | managed.go | test/runner |
| CI runner | policy | policy.go | test/runner |
| CI runner | static | static.go | test/runner |
| CI runner | traffic | traffic.go | test/runner |
| CI runner | ui | ui.go | test/runner |
| CI runner | vpp | vpp.go | test/runner, test/peer |
| CI runner | web | web.go | component/web/testing, core/textbuf |
| Mock | cymru | cymru.go | (stdlib only) |
| Mock | irr | irr.go | (stdlib only) |
| Mock | peeringdb | peeringdb.go | (stdlib only) |
| Mock | rpki | rpki.go | (stdlib only) |
| Mock | rtr-mock | rtr_mock.go | (stdlib only) |
| Mock | syslog | syslog.go | test/syslog |
| Mock | tacacs-mock | tacacs_mock.go | component/tacacs |
| Tool | peer | peer.go | test/peer, core/env |
| Tool | mcp | mcp.go | core/stringsx, core/textbuf |
| Tool | text-plugin | text_plugin.go | core/env |
| Tool | l2tp-scale | l2tp_scale.go | component/l2tp, component/radius |

**Handler signature migration:**

Current ze-test pattern:
```
func bgpCmd() int { /* reads os.Args */ }
var _ = register("bgp", "desc", bgpCmd)
```

Target ze registry pattern:
```
registry.MustRegisterRootHandler("bgp", func(rctx *registry.RuntimeContext, args []string) int {
    return bgpMain(args)
}, registry.Meta{...})
```

Each handler must stop reading `os.Args` directly and accept args from the dispatcher.

**Behavior to preserve:**
- `ze-test <subcommand> [options]` CLI must keep working (same invocation pattern)
- All 25 subcommands remain available with identical flags and behavior
- CI pipeline uses `bin/ze-test` extensively
- `zetest` build tag (no underscore) for DUT test plugins remains separate
- Existing .ci test files and test runner internals unchanged

**Behavior to change:**
- ze-test dispatch moves from own `register()` to `command/registry.MustRegisterRootHandler`
- ze-test binary built from `cmd/ze` with `-tags ze_test` instead of `go build ./cmd/ze-test`
- `cmd/ze-test/` directory deleted
- Binary size reduced by excluding `plugin/all` and daemon infrastructure
- Handlers adapted from `func() int` (reads os.Args) to `func(rctx, args) int`

## Data Flow (MANDATORY)

### Entry Point
- User runs `ze-test <subcommand> [args]`
- Binary is `cmd/ze` compiled with `-tags ze_test -o bin/ze-test`
- No argv[0] detection needed; build tag controls which subcommands exist

### Transformation Path
1. `main()` in `cmd/ze/main.go` runs identically for both ze and ze-test binaries
2. Global flag parsing runs (same as regular ze)
3. `dispatchRegisteredRoot(arg, rctx, rest)` finds handler registered by ze-test subcommand's `init()`
4. Handler receives `*registry.RuntimeContext` and args, executes subcommand logic
5. For CI runners: creates `runner.NewEncodingTests()`, discovers .ci files, runs via `runner.NewRunner()`
6. For mocks: starts a server on configured port, serves deterministic responses
7. For tools: runs the tool logic (peer simulation, MCP client, etc.)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Build system -> binary | Build tags control which subcommands compile in | [ ] |
| CLI dispatch -> subcommand | `registry.MustRegisterRootHandler` + `dispatchRegisteredRoot()` | [ ] |
| Subcommand -> test runner | `internal/test/runner` package API | [ ] |

### Integration Points
- `command/registry.MustRegisterRootHandler` - where ze-test subcommands register
- `dispatchRegisteredRoot()` in `cmd/ze/main.go` - dispatch path
- `internal/test/runner` - test execution engine (unchanged)
- `internal/test/peer` - BGP peer simulation (unchanged)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze-test bgp --help` | -> | bgp handler registered via `MustRegisterRootHandler` | `TestZeTestBgpHelp` |
| `ze-test peer --help` | -> | peer handler registered via `MustRegisterRootHandler` | `TestZeTestPeerHelp` |
| `go build -tags ze_test ./cmd/ze` | -> | binary contains test subcommands | `TestBuildZeTestVariant` |
| `go build ./cmd/ze` (no tag) | -> | binary does NOT contain test subcommands | `TestBuildZeNoTestSubcommands` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `go build -tags ze_test ./cmd/ze` | Produces a binary with all 25 ze-test subcommands registered |
| AC-2 | `bin/ze-test bgp -a` | Runs all BGP functional tests (same behavior as today) |
| AC-3 | `bin/ze-test --help` | Lists all 25 subcommands with descriptions |
| AC-4 | `go build ./cmd/ze` (no ze_test tag) | Binary does NOT contain any test subcommands |
| AC-5 | `bin/ze-test` binary size | Smaller than current 47MB (does not include plugin/all except for editor) |
| AC-6 | `cmd/ze-test/` directory | Deleted; all subcommand code lives under `cmd/ze/` guarded by `//go:build ze_test` |
| AC-7 | `make generate` | Regenerates import files for both regular and ze_test configurations |
| AC-8 | `bin/ze-test editor -a` | Still works (editor needs plugin/all for YANG schemas) |
| AC-9 | `zetest` build tag | Still works independently for DUT test plugins (no collision with `ze_test`) |
| AC-10 | Makefile `bin/ze-test` target | Builds via `go build -tags ze_test ./cmd/ze` |
| AC-11 | `bin/ze-test` dispatch | Test subcommands dispatch as root commands via `dispatchRegisteredRoot()`; no special argv[0] logic needed |
| AC-12 | All internal/ imports from ze-test subcommands | Only test-relevant packages compiled in; daemon infrastructure excluded |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestZeTestSubcommandRegistration` | `cmd/ze/ze_test_register_test.go` | All 25 subcommands register when ze_test tag is set | |
| `TestZeTestHelpOutput` | `cmd/ze/ze_test_help_test.go` | --help lists all test subcommands | |
| `TestZeTestBuildExclusion` | `cmd/ze/ze_test_build_test.go` | Test subcommands absent from regular ze build | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-ze-test-build` | `test/build/ze-test-build.ci` | `go build -tags ze_test` succeeds, binary has test subcommands | |
| `test-ze-test-bgp` | existing `test/encode/*.ci` | ze-test bgp -a still passes all existing tests | |
| `test-ze-test-editor` | existing `test/editor/*.ci` | ze-test editor -a still passes | |

### Future
- Binary size benchmarks comparing old vs new ze-test (defer: informational, not gating)

## Files to Modify

### Phase 1: Unify dispatch

- `cmd/ze/ze_test_*.go` (NEW, all `//go:build ze_test`) - One file per subcommand, registering via `MustRegisterRootHandler`
- `cmd/ze/ze_test_ci_runner.go` (NEW, `//go:build ze_test`) - Shared CI runner logic (from `cmd/ze-test/ci_runner.go`)
- `cmd/ze-test/` - DELETE entire directory after migration

### Phase 2: Minimal binary via codegen

- `scripts/codegen/plugin_imports.go` - Extend to generate tag-guarded import files for cmd/ze
- `cmd/ze/imports_full.go` (NEW, `//go:build !ze_test`) - Generated: imports plugin/all and all CLI command owners (current main.go blank imports)
- `cmd/ze/imports_ze_test.go` (NEW, `//go:build ze_test`) - Generated: imports only what test subcommands need
- `cmd/ze/main.go` - Remove direct blank imports (moved to generated files)
- `Makefile` - Update `bin/ze-test` target to `go build -tags ze_test -o bin/ze-test ./cmd/ze`

### Dependency analysis for generated import files

**`imports_full.go` (`//go:build !ze_test`) should import:**
- `internal/component/plugin/all` (84 plugins + 111 schemas)
- All CLI command owner packages (iface/cli, firewall/cli, sysctl/cli, etc.)
- `internal/component/aaa/all`

**`imports_ze_test.go` (`//go:build ze_test`) should import:**
- `internal/component/plugin/all` (needed by editor.go for YANG schemas)
- `internal/component/cli/testing`
- `internal/component/web/testing`

All other test imports (test/runner, test/peer, core/env, etc.) are direct imports in the subcommand files, not blank imports.

→ Decision: editor.go still pulls in plugin/all. This means the ze_test binary still includes all plugins when editor is compiled in. Trimming editor's YANG dependency is a separate optimization. The binary is still smaller than full ze because daemon infrastructure (hub, managed, config dispatch, pprof, etc.) from main.go does not compile in.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | N/A |
| CLI commands/flags | Yes | cmd/ze/ ze_test_*.go files register via MustRegisterRootHandler |
| Functional test for new RPC/API | No | N/A (preserving existing tests) |
| Pipe completeness | No | N/A (test subcommands don't produce piped output) |
| Doctor check for runtime dependencies | No | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | N/A (internal restructure) |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | Same subcommands, same flags |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | No | N/A |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` - update build instructions for ze-test |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | Yes | `docs/architecture/cli/plugin-modes.md` - add ze_test variant |
| 13 | Route metadata keys added/changed? | No | N/A |
| 14 | Prometheus counters added/changed? | No | N/A |
| 15 | Registered plugin/event/command changed? | No | N/A |
| 16 | Changed source files referenced by doc anchors? | Yes | Grep docs/ for cmd/ze-test references |
| 17 | Existing docs show config/CLI/API examples? | Yes | Grep docs/ for `ze-test` build instructions |

## Files to Create

- `cmd/ze/ze_test_ci_runner.go` - Shared CI runner logic (`//go:build ze_test`)
- `cmd/ze/ze_test_bgp.go` - BGP test subcommand
- `cmd/ze/ze_test_peer.go` - BGP test peer
- `cmd/ze/ze_test_editor.go` - Editor test runner
- `cmd/ze/ze_test_mcp.go` - MCP client
- `cmd/ze/ze_test_web.go` - Web test runner
- `cmd/ze/ze_test_rpki.go` - RPKI mock server
- `cmd/ze/ze_test_cymru.go` - Cymru DNS mock
- `cmd/ze/ze_test_irr.go` - IRR whois mock
- `cmd/ze/ze_test_peeringdb.go` - PeeringDB mock
- `cmd/ze/ze_test_rtr_mock.go` - RTR mock server
- `cmd/ze/ze_test_syslog.go` - Syslog server
- `cmd/ze/ze_test_tacacs_mock.go` - TACACS+ mock
- `cmd/ze/ze_test_exabgp.go` - ExaBGP compat tests
- `cmd/ze/ze_test_firewall.go` - Firewall tests
- `cmd/ze/ze_test_flowexport.go` - Flow export tests
- `cmd/ze/ze_test_install.go` - Install tests
- `cmd/ze/ze_test_l2tp.go` - L2TP tests
- `cmd/ze/ze_test_l2tp_wire.go` - L2TP wire tests
- `cmd/ze/ze_test_l2tp_scale.go` - L2TP scale test
- `cmd/ze/ze_test_managed.go` - Managed tests
- `cmd/ze/ze_test_policy.go` - Policy tests
- `cmd/ze/ze_test_static.go` - Static route tests
- `cmd/ze/ze_test_traffic.go` - Traffic tests
- `cmd/ze/ze_test_ui.go` - UI tests
- `cmd/ze/ze_test_vpp.go` - VPP tests
- `cmd/ze/ze_test_text_plugin.go` - Text plugin tester
- `cmd/ze/imports_full.go` - Generated: full imports for regular ze build
- `cmd/ze/imports_ze_test.go` - Generated: minimal imports for ze-test build

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- Register one test subcommand via ze registry, write failing wiring test
   - Tests: `TestZeTestBgpHelp`, `TestBuildZeTestVariant`
   - Files: `cmd/ze/ze_test_bgp.go`
   - Verify: `ze-test bgp --help` works via registry dispatch; build-tag exclusion verified

2. **Phase: Migrate CI runner subcommands** -- Move all 15 CI runner subcommands + shared ci_runner logic
   - Tests: each subcommand's --help output matches current behavior
   - Files: `cmd/ze/ze_test_ci_runner.go`, `cmd/ze/ze_test_bgp.go`, `cmd/ze/ze_test_editor.go`, and 13 more
   - Verify: `ze-test <subcommand> --help` works for all CI runner subcommands

3. **Phase: Migrate mock server subcommands** -- Move all 7 mock servers
   - Tests: each mock's --help output matches
   - Files: `cmd/ze/ze_test_rpki.go`, `cmd/ze/ze_test_cymru.go`, and 5 more
   - Verify: `ze-test <subcommand> --help` works for all mock subcommands

4. **Phase: Migrate tool subcommands** -- Move peer, mcp, text-plugin, l2tp-scale
   - Tests: each tool's --help output matches
   - Files: `cmd/ze/ze_test_peer.go`, `cmd/ze/ze_test_mcp.go`, `cmd/ze/ze_test_text_plugin.go`, `cmd/ze/ze_test_l2tp_scale.go`
   - Verify: `ze-test <subcommand> --help` works for all tool subcommands

5. **Phase: Delete cmd/ze-test/** -- Remove old directory after all subcommands migrated
   - Tests: `go build ./cmd/ze-test` must fail (directory gone)
   - Files: delete `cmd/ze-test/` (28 files + 4 test files)
   - Verify: `go build -tags ze_test ./cmd/ze` still produces working binary

6. **Phase: Codegen for import splitting** -- Extend plugin_imports.go to generate tag-guarded import files
   - Tests: `TestBuildZeNoTestSubcommands`, binary size comparison
   - Files: `scripts/codegen/plugin_imports.go`, `cmd/ze/imports_full.go`, `cmd/ze/imports_ze_test.go`, `cmd/ze/main.go`
   - Verify: `go build ./cmd/ze` (no tag) produces regular ze; `go build -tags ze_test ./cmd/ze` produces minimal ze-test

7. **Phase: Makefile update** -- Update build targets
   - Files: `Makefile`
   - Verify: `make build` still produces both `bin/ze` and `bin/ze-test`

8. **Functional tests** -- Run full CI suite with new binary
9. **Full verification** -- `make ze-verify`
10. **Complete spec** -- Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | All 25 subcommands register and dispatch correctly; no subcommand lost in migration |
| Naming | Build tag is `ze_test` (underscore); files prefixed `ze_test_` to indicate tag guard |
| Data flow | Dispatch goes through `dispatchRegisteredRoot()`, same as other ze commands |
| Binary size | ze-test binary excludes daemon code; only editor pulls plugin/all |
| No duplicate dispatch | Old `cmd/ze-test/` dispatch table fully removed, no parallel system |
| Handler signature | All handlers adapted from `func() int` to `func(rctx, args) int`; no direct os.Args reads |
| Rule: no-layering | `cmd/ze-test/` fully deleted, not left as dead code |
| Rule: registration | Subcommands use `MustRegisterRootHandler`, same pattern as other ze commands |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| All 25 subcommands register via ze registry | `grep -c MustRegisterRootHandler cmd/ze/ze_test_*.go` |
| All files guarded by `//go:build ze_test` | `head -1 cmd/ze/ze_test_*.go` shows build tag |
| `cmd/ze-test/` deleted | `ls cmd/ze-test/` fails |
| `bin/ze-test` builds from cmd/ze | `grep 'ze_test' Makefile` shows `-tags ze_test ./cmd/ze` |
| Generated import files exist | `ls cmd/ze/imports_full.go cmd/ze/imports_ze_test.go` |
| `make generate` updates import files | `make generate && git diff --name-only` shows no change |
| `ze-test --help` lists all 25 subcommands | `bin/ze-test --help` output |
| Existing CI tests pass | `bin/ze-test bgp -a` exits 0 |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | No new user input paths; subcommands preserved as-is |
| Build isolation | `ze_test` subcommands must not compile into production ze binary (verify with `go build ./cmd/ze` + symbol check) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior, RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural, DESIGN phase |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| Import cycle from moving subcommand | Restructure: move heavy logic to internal/test/cmd/<name>/, keep thin wrapper in cmd/ze/ |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Build tag `ze_test` (underscore) | `ze_testbin`, `ze_ci`, `ze_runner` | Follows existing convention (`ze_distro`, `ze_setup`). User's choice. Distinct from existing `zetest` (no underscore) for DUT test plugins. |
| Unify with ze registry | Keep separate dispatch | Single dispatch system reduces maintenance. Test subcommands are just more root handlers. User's choice. |
| go:generate for import splitting | Manual build-tag files, separate main.go | Extends existing codegen pattern (`plugin_imports.go`). Keeps import lists in sync automatically. User's choice. |
| Accept plugin/all in editor | Trim editor YANG deps, separate editor tag | Simplicity. Editor needs full YANG tree. Trimming is a separate optimization. Binary is still smaller than full ze due to no daemon infrastructure. |
| No argv[0] detection | Busybox pattern, subcommand prefix | Unnecessary. Makefile builds `ze-test` as a separate binary via `-tags ze_test -o bin/ze-test`. The build tag alone controls which subcommands compile in. `main()` dispatch is identical for both binaries. |
| Adapt handler signature to RootHandler | Wrapper shim keeping old signature | Clean migration. All handlers receive args from dispatcher instead of reading os.Args. Consistent with how all other ze commands work. |

## Known Limitations
- editor subcommand still pulls in `plugin/all` (~84 plugins + 111 schemas) because `editortesting.RunETFile()` needs full YANG schema registration. A separate spec could create a test-only schema subset.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| ze-test built as build-tag variant of cmd/ze | build test | `go build -tags ze_test ./cmd/ze` produces working binary |
| All 25 subcommands work identically | functional test | `bin/ze-test bgp -a` and other CI suites pass |
| Minimal binary (no unnecessary deps) | binary size | `ls -la bin/ze-test` shows reduction from 47MB |
| cmd/ze-test/ removed | filesystem check | `ls cmd/ze-test/` fails |
| Existing ze unaffected | build test | `go build ./cmd/ze` produces identical binary |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [pending]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`cmd/ze/ze_test_*.go`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ze-test-build-tags.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ze-test-build-tags.md`
