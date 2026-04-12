# Spec: wasi-support

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-plugin-tcp-transport |
| Phase | - |
| Updated | 2026-03-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-plugin-tcp-transport.md` - prerequisite spec (TCP connect-back transport)
4. `internal/component/plugin/ipc/socketpair.go` - Unix-specific IPC
5. `internal/component/plugin/ipc/fdpass.go` - Unix-specific fd passing
6. `cmd/ze/main.go` - main entry point and subcommand dispatch

## Task

Enable compilation of Ze to WASI (`GOOS=wasip1 GOARCH=wasm`), producing a `ze.wasm` binary that runs under Wasmtime or other WASI runtimes on any platform. This requires: (1) build-tagging Unix-specific code with WASI stubs, (2) a WASI-specific entry point excluding TUI-dependent subcommands, and (3) a Makefile target.

### Motivation

- WASI provides a portable, sandboxed execution environment
- Ze's CLI tools (validate, schema, yang, bgp decode/encode) are useful standalone without a full daemon
- WASI plugins could connect to a native Ze engine via TCP (once `spec-plugin-tcp-transport` is done)
- Single `.wasm` binary runs on any OS/arch with a WASI runtime

### Scope

**In scope:**
- Build-tag IPC files (`fdpass.go`, `socketpair.go`) with WASI stubs
- Create `cmd/ze-wasi/main.go` entry point with WASI-safe subcommands only
- Makefile target `ze-wasi` producing `bin/ze.wasm`
- Makefile target `ze-wasi-test` running basic validation under Wasmtime

**Out of scope:**
- Full daemon mode under WASI (requires TCP transport from `spec-plugin-tcp-transport`)
- SSH server under WASI (TUI dependency, no clear use case)
- TinyGo compilation (standard Go covers WASI)
- WASI preview 2 (`wasip2`) -- not yet in Go's supported target list as of Go 1.26

### Dependencies

- `spec-plugin-tcp-transport`: required for WASI plugins to connect back to a native engine
  - Without it: WASI build supports CLI tools only (validate, decode, encode, schema, yang)
  - With it: WASI build can also run plugins that connect to a remote engine via TCP

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/system-architecture.md` - overall system structure
  → Constraint: subcommands are independent packages; entry point just dispatches

### RFC Summaries (MUST for protocol work)
- Not protocol work

**Key insights:**
- Go 1.26 supports `GOOS=wasip1 GOARCH=wasm` (confirmed in `go tool dist list`)
- `GOOS=wasip2` is NOT in Go 1.26's supported targets
- Three build failure categories: IPC (Ze code), termenv (Charmbracelet TUI), clipboard (atotto)
- TUI deps are pulled by: `cli`, `config edit/set/diff/history/rollback`, `bgp plugin cli`, `yang tree`
- TUI deps cannot be fixed with build tags (third-party code)
- Privilege dropping already has build tags (`drop_unix.go` / `drop_other.go`)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/main.go` - single entry point, imports all 17 subcommand packages + `plugin/all`
  → Constraint: importing `cli` package pulls in bubbletea/termenv/clipboard (WASI-incompatible)
  → Constraint: importing `config` package pulls TUI for interactive subcommands (edit, set, diff, history, rollback)
  → Constraint: `plugin/all` blank import triggers all plugin `init()` registrations
- [ ] `internal/component/plugin/ipc/fdpass.go` - uses `golang.org/x/sys/unix` (UnixRights, CmsgSpace, ParseSocketControlMessage, ParseUnixRights, CloseOnExec, Close)
  → Constraint: no build tags, fails on WASI
- [ ] `internal/component/plugin/ipc/socketpair.go` - uses `syscall.Socketpair`
  → Constraint: no build tags, platform-agnostic types mixed with Unix-specific functions
  → Constraint: `SocketPair`, `DualSocketPair`, `Close()`, `NewInternalSocketPairs()` are WASI-safe
  → Constraint: `NewExternalSocketPairs()`, `PluginFiles()`, `newUnixSocketPair()`, `connToFile()` are Unix-only
- [ ] `internal/core/privilege/drop_unix.go` - already has `//go:build linux || darwin || freebsd || openbsd || netbsd`
- [ ] `internal/core/privilege/drop_other.go` - already has `//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd`
  → Decision: privilege code already handles non-Unix platforms; WASI falls into `drop_other.go`

**WASI-safe subcommands** (do not import TUI):
- `bgp decode`, `bgp encode` (wire parsing -- not `bgp plugin cli`)
- `validate` (config parsing, text output)
- `schema` (metadata queries)
- `yang doc`, `yang completion` (not `yang tree`)
- `exabgp` (migration tooling)
- `completion` (shell completions)
- `version`

**WASI-unsafe subcommands** (import TUI via bubbletea/termenv/clipboard):
- `cli` (interactive TUI)
- `config edit`, `config set`, `config diff`, `config history`, `config rollback` (editor TUI)
- `bgp plugin cli` (interactive plugin TUI)
- `yang tree` (TUI formatting)

**Subcommands that compile but fail at runtime** (socket operations):
- `signal`, `status`, `show`, `run` (connect to daemon via Unix socket)
- `hub` (daemon mode -- TCP listener, plugin subprocess management)
- `init`, `db` (blob store -- may work if filesystem access is granted)

**Behavior to preserve:**
- Native `ze` binary unchanged -- no build tag contamination of normal builds
- All existing tests continue to pass on native platforms
- `plugin/all` blank import works on native platforms

**Behavior to change:**
- `fdpass.go` and `socketpair.go` gain build tags, Unix-specific code moves to `_unix.go` files
- New WASI stub files for IPC functions that return errors
- New `cmd/ze-wasi/main.go` entry point with subset of subcommands
- New Makefile targets for WASI build and test

## Data Flow (MANDATORY)

### Entry Point
- `cmd/ze-wasi/main.go` dispatches to WASI-safe subcommand packages
- User runs: `wasmtime --dir=. bin/ze.wasm -- validate config.conf`

### Transformation Path
1. WASI runtime loads `ze.wasm`, grants filesystem access via `--dir`
2. `main()` parses args, dispatches to subcommand (e.g., `validate`)
3. Subcommand reads config file via `os.Open` (mapped by WASI runtime to host filesystem)
4. Subcommand processes and outputs to stdout/stderr
5. Exit code returned to WASI runtime

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| WASI runtime ↔ Host filesystem | `--dir` flag grants access | [ ] |
| WASI module ↔ stdout/stderr | Standard fd 1/2 | [ ] |

### Integration Points
- `cmd/ze-wasi/main.go` imports a subset of `cmd/ze/` subcommand packages
- IPC stub files provide the same function signatures as Unix files (returning errors)
- `plugin/all` blank import still works (plugins register, but external launch returns error)

### Architectural Verification
- [ ] No bypassed layers -- WASI entry point uses same subcommand packages
- [ ] No unintended coupling -- WASI build is additive (new entry point + stubs)
- [ ] No duplicated functionality -- subcommand code is shared, only dispatch differs
- [ ] Zero-copy preserved -- not applicable (CLI tools, not hot path)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `GOOS=wasip1 GOARCH=wasm go build ./cmd/ze-wasi` | -> | Build succeeds, produces `.wasm` | `make ze-wasi` (build target) |
| `wasmtime bin/ze.wasm -- validate test.conf` | -> | Config validated, exit 0 | `make ze-wasi-test` |
| `wasmtime bin/ze.wasm -- version` | -> | Version printed | `make ze-wasi-test` |
| `wasmtime bin/ze.wasm -- bgp decode ...` | -> | BGP message decoded | `make ze-wasi-test` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `GOOS=wasip1 GOARCH=wasm go build ./cmd/ze-wasi` | Build succeeds, produces `ze.wasm` |
| AC-2 | `wasmtime --dir=. bin/ze.wasm -- version` | Prints version string, exits 0 |
| AC-3 | `wasmtime --dir=. bin/ze.wasm -- validate test.conf` | Validates config, exits 0 for valid config |
| AC-4 | `wasmtime --dir=. bin/ze.wasm -- validate bad.conf` | Reports error, exits 1 |
| AC-5 | `wasmtime --dir=. bin/ze.wasm -- bgp decode <hex>` | Decodes BGP message to JSON |
| AC-6 | Native `make ze-verify` still passes | No regression on native builds |
| AC-7 | `fdpass.go` and `socketpair.go` unchanged on native builds | Build tags only add WASI stubs, do not modify Unix code |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWASIFdpassStubs` | `internal/component/plugin/ipc/fdpass_wasip1_test.go` | Stubs return errors | |
| `TestWASISocketpairStubs` | `internal/component/plugin/ipc/socketpair_wasip1_test.go` | External socket creation returns error, internal (net.Pipe) still works | |

### Boundary Tests (MANDATORY for numeric inputs)
- Not applicable -- no new numeric inputs

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `wasi-build` | Makefile target `ze-wasi` | WASI binary compiles successfully | |
| `wasi-validate` | Makefile target `ze-wasi-test` | Config validation works under Wasmtime | |

### Future (if deferring any tests)
- WASI plugin connecting to native engine via TCP (requires `spec-plugin-tcp-transport`)
- Testing under Wasmer and wazero runtimes (Wasmtime is primary)

## Files to Modify
- `internal/component/plugin/ipc/fdpass.go` - add `//go:build !wasip1` build tag
- `internal/component/plugin/ipc/socketpair.go` - split: keep platform-agnostic types, move Unix functions to `socketpair_unix.go`
- `Makefile` - add `ze-wasi` and `ze-wasi-test` targets

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | No |
| RPC count in architecture docs | [ ] | No |
| CLI commands/flags | [ ] | No |
| CLI usage/help text | [ ] | No |
| API commands doc | [ ] | No |
| Plugin SDK docs | [ ] | No |
| Editor autocomplete | [ ] | No |
| Functional test for new RPC/API | [ ] | No -- build/runtime test only |

## Files to Create
- `internal/component/plugin/ipc/fdpass_unix.go` - existing `fdpass.go` Unix code with `//go:build !wasip1`
- `internal/component/plugin/ipc/fdpass_wasip1.go` - stub `SendFD`/`ReceiveFD` returning errors
- `internal/component/plugin/ipc/socketpair_unix.go` - Unix-specific functions (`NewExternalSocketPairs`, `PluginFiles`, `newUnixSocketPair`, `connToFile`) with `//go:build !wasip1`
- `internal/component/plugin/ipc/socketpair_wasip1.go` - stubs returning errors
- `cmd/ze-wasi/main.go` - WASI entry point with safe subcommands only

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `.claude/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/rfcNNNN.md` |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` or subsystem doc |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: IPC build tags** -- split fdpass.go and socketpair.go into Unix + WASI files
   - Tests: `TestWASIFdpassStubs`, `TestWASISocketpairStubs` (run on native to verify stubs compile)
   - Files: `fdpass.go` -> `fdpass_unix.go` + `fdpass_wasip1.go`; `socketpair.go` -> keep types + `socketpair_unix.go` + `socketpair_wasip1.go`
   - Verify: `make ze-verify` still passes (no native regression)

2. **Phase: WASI entry point** -- `cmd/ze-wasi/main.go` with safe subcommand subset
   - Tests: `GOOS=wasip1 GOARCH=wasm go build ./cmd/ze-wasi` succeeds
   - Files: `cmd/ze-wasi/main.go`
   - Verify: build produces `.wasm` file

3. **Phase: Makefile targets** -- `ze-wasi` and `ze-wasi-test`
   - Tests: `make ze-wasi` succeeds, `make ze-wasi-test` passes under Wasmtime
   - Files: `Makefile`
   - Verify: both targets work

4. **Phase: Native regression** -- full verification on native platform
   - Verify: `make ze-verify` passes with no changes to behavior

5. **Complete spec** -- audit tables, learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Build tags are correct: `//go:build !wasip1` on Unix files, `//go:build wasip1` on stubs |
| No regression | Native `make ze-verify` passes unchanged |
| File split | `socketpair.go` keeps only platform-agnostic code (types, Close, NewInternalSocketPairs) |
| WASI entry point | Only imports WASI-safe subcommands; no transitive TUI dependency |
| Rule: no-layering | Not replacing -- additive (new build target alongside native) |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `fdpass_unix.go` exists | `ls internal/component/plugin/ipc/fdpass_unix.go` |
| `fdpass_wasip1.go` exists | `ls internal/component/plugin/ipc/fdpass_wasip1.go` |
| `socketpair_unix.go` exists | `ls internal/component/plugin/ipc/socketpair_unix.go` |
| `socketpair_wasip1.go` exists | `ls internal/component/plugin/ipc/socketpair_wasip1.go` |
| `cmd/ze-wasi/main.go` exists | `ls cmd/ze-wasi/main.go` |
| WASI build succeeds | `GOOS=wasip1 GOARCH=wasm go build ./cmd/ze-wasi` |
| Native build still works | `make ze-verify` |
| Makefile targets exist | `grep ze-wasi Makefile` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | WASI entry point: same arg validation as native (no relaxation) |
| Build tag correctness | Unix code not compiled into WASI binary; WASI stubs not compiled into native |
| No secret exposure | WASI binary does not embed credentials or keys |

### Failure Routing

| Failure | Route To |
|---------|----------|
| WASI build fails on new dependency | Add build tag to offending import, or exclude subcommand |
| Native regression | Build tag is wrong -- `!wasip1` missing from Unix file |
| Wasmtime runtime error | Check `--dir` flags, verify subcommand doesn't use sockets |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

Not applicable -- build infrastructure, not BGP protocol.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
