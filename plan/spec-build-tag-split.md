# Spec: build-tag-split

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/6 |
| Updated | 2026-06-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/cli/plugin-modes.md` - CLI dispatch and build tag design
4. `cmd/ze/setup_features_full.go` - current full-build imports
5. `cmd/ze/setup_features_stripped.go` - current stripped-build stub
6. `cmd/ze/appliance_import.go` - current appliance import

## Task

Replace the negative `ze_stripped` / `!ze_stripped` build tag with positive, composable tags: `ze_appliance`, `ze_linux`, `ze_setup`. This produces a clean binary split between `ze` (on-device network OS) and `ze-setup` (operator build/provision tooling) from the same `cmd/ze/` source, controlled only by which tags the Makefile passes.

Today, `ze_stripped` is a negative gate: the default build gets everything, and the gokrazy build strips things out. This inverts the safety model (new code is included everywhere unless someone remembers to gate it) and conflates two different concerns (on-device operations vs build-host tooling) into a single binary.

Three positive tags fix both problems. Each tag maps to exactly one binary:

| Tag | Binary | Commands |
|-----|--------|----------|
| `ze_linux` | `bin/ze` | install local, install systemd, uninstall, connect |
| `ze_appliance` | `bin/ze-appliance` | appliance init/build/iso/clone/passwd/config/... |
| `ze_setup` | `bin/ze-setup` | install remote (PXE provision), future kernel/initrd build |

Binaries produced:

| Binary | Tags | Purpose |
|--------|------|---------|
| `bin/ze` | `ze_linux` | On-device network OS + local install |
| `bin/ze-appliance` | `ze_appliance` | Image builder (operator workstation) |
| `bin/ze-setup` | `ze_setup` | PXE provisioning (operator workstation) |
| `bin/ze-stripped` | (none) | Gokrazy appliance image (minimal) |
| dev build | `ze_appliance,ze_linux,ze_setup` | Developer: all commands in one binary |

The `ze_stripped` tag and its `!ze_stripped` counterpart are deleted. The gokrazy config switches from `"GoBuildTags": ["ze_stripped"]` to `"GoBuildTags": []` (no tags = stripped).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/cli/plugin-modes.md` - CLI dispatch and build tag system
- [ ] `plan/learned/850-appliance-command-plugin.md` - appliance moved to internal/appliance/ with root handler registration
- [ ] `plan/learned/834-stripped-build-and-iso-coverage.md` - ze_stripped build tag rationale

### Learned Summaries
- [ ] `plan/learned/851-install-10-pxe-staging.md` - PXE staging just completed, provision code in cmd/ze/provision/

**Key insights:**
- `cmd/ze/main.go` is the shared entry point; all dispatch goes through `cmdregistry`
- Blank imports in `setup_features_*.go` and `appliance_import.go` control what registers
- `ze_stripped` is referenced in: Makefile (2 targets), gokrazy config, 4 Go files, 1 integration test mk
- The subdispatch pattern (`cmd/ze/install/dispatch.go`) routes subcommands; registration is in per-subcommand `register.go` files
- `cmd/ze/plugins_zetest.go` uses a separate `zetest` tag -- unaffected by this change

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/setup_features_full.go` - `!ze_stripped` build tag; imports install/{local,remote,systemd}, uninstall/{local,systemd}, connect
- [ ] `cmd/ze/setup_features_stripped.go` - `ze_stripped` build tag; empty file (no imports)
- [ ] `cmd/ze/appliance_import.go` - `!ze_stripped`; blank import of internal/appliance
- [ ] `cmd/ze/install/register.go` - registers "install" root handler unconditionally
- [ ] `cmd/ze/install/dispatch.go` - subdispatch for install subcommands
- [ ] `cmd/ze/install/local/register.go` - registers "local" under install
- [ ] `cmd/ze/install/remote/register.go` - registers "remote" under install, delegates to cmd/ze/provision.Run
- [ ] `cmd/ze/install/systemd/register.go` - registers "systemd" under install
- [ ] `Makefile` lines 80-106 - bin/ze (no tag = full), bin/ze-stripped (ze_stripped tag)
- [ ] `gokrazy/ze/config.json` line 26 - GoBuildTags: ["ze_stripped"]
- [ ] `mk/test-integration.mk` line 164 - builds ze-stripped for QEMU tests

**Behavior to preserve:**
- All command registration patterns (root handlers, subdispatch)
- `cmd/ze/main.go` unchanged (shared entry point)
- `zetest` tag unaffected
- `pprof` / `pprof_tinygo` tags unaffected
- Doctor checks (linux/other split) unaffected
- All existing commands continue to work in their respective binaries
- QEMU integration tests still build a minimal binary

**Behavior to change:**
- Delete `ze_stripped` and `!ze_stripped` build tags entirely
- Replace with three positive tags: `ze_linux`, `ze_appliance`, `ze_setup`
- New `bin/ze-setup` Makefile target
- Gokrazy config: empty GoBuildTags (no tags = minimal)
- `bin/ze` built with `ze_linux` (not the full build anymore)
- Dev build gets a new tag combination or a convenience target

## Data Flow (MANDATORY)

### Entry Point
- Makefile `go build -tags <tags> -o bin/<name> ./cmd/ze`
- Build tags control which `setup_features_*.go` files compile
- Compiled files execute `init()` which blank-imports command packages
- Command packages register handlers via `cmdregistry` from their own `init()`

### Transformation Path
1. Makefile passes tags to `go build`
2. Go compiler selects source files matching build constraints
3. Selected `setup_features_*.go` files import command packages
4. Command package `register.go` init() calls `cmdregistry.MustRegisterRootHandler` or `subdispatch.Register`
5. At runtime, `cmd/ze/main.go` dispatches to registered handlers

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Makefile -> Go compiler | -tags flag | [ ] |
| Build tag -> source selection | //go:build constraint | [ ] |
| Blank import -> init() registration | Go init() ordering | [ ] |

### Integration Points
- `cmd/ze/install/register.go` registers the "install" root handler unconditionally -- needs to only register when at least one subcommand is available, or be split by tag
- `cmd/ze/uninstall/` has the same dispatch pattern as install

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `go build -tags ze_linux ./cmd/ze` | -> | binary has install local, no appliance | `TestZeLinuxBinaryCommands` |
| `go build -tags ze_appliance ./cmd/ze` | -> | binary has appliance, no install | `TestZeApplianceBinaryCommands` |
| `go build -tags ze_setup ./cmd/ze` | -> | binary has install remote, no install local, no appliance | `TestZeSetupBinaryCommands` |
| `go build ./cmd/ze` (no tags) | -> | minimal binary, no install/appliance | `TestZeStrippedBinaryCommands` |
| `go build -tags ze_appliance,ze_linux,ze_setup ./cmd/ze` | -> | all commands present | `TestZeFullBinaryCommands` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `go build -tags ze_linux -o /tmp/ze ./cmd/ze` then `/tmp/ze install local --help` | Shows install local help |
| AC-2 | Same ze_linux binary, `/tmp/ze appliance --help` | "unknown command" or not present |
| AC-3 | Same ze_linux binary, `/tmp/ze install remote --help` | "unknown command" or not present |
| AC-4 | `go build -tags ze_appliance -o /tmp/ze-appliance ./cmd/ze` then `/tmp/ze-appliance appliance --help` | Shows appliance help |
| AC-5 | `go build -tags ze_setup -o /tmp/ze-setup ./cmd/ze` then `/tmp/ze-setup install remote --help` | Shows provision help |
| AC-6 | Same ze-setup binary, `/tmp/ze-setup install local --help` | "unknown command" or not present |
| AC-7 | `go build -o /tmp/ze-min ./cmd/ze` (no tags) then `/tmp/ze-min install --help` | "unknown command" or not present |
| AC-8 | `go build -tags ze_appliance,ze_linux,ze_setup -o /tmp/ze-dev ./cmd/ze` | All commands present: install local, install remote, appliance |
| AC-9 | gokrazy config.json | GoBuildTags is empty array [] |
| AC-10 | grep for `ze_stripped` across entire repo | Zero matches |
| AC-11 | `make build` produces bin/ze, bin/ze-appliance, bin/ze-setup, bin/ze-stripped | All four binaries exist |
| AC-12 | QEMU integration test builds a minimal binary | Uses no-tag build (same as old ze_stripped) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestZeLinuxBinaryCommands` | `cmd/ze/build_tag_test.go` | ze_linux binary has local/systemd, lacks appliance/remote | |
| `TestZeApplianceBinaryCommands` | `cmd/ze/build_tag_test.go` | ze_appliance binary has appliance, lacks install/remote | |
| `TestZeSetupBinaryCommands` | `cmd/ze/build_tag_test.go` | ze_setup binary has remote, lacks local/appliance | |
| `TestZeStrippedBinaryCommands` | `cmd/ze/build_tag_test.go` | no-tag binary has no install/appliance | |
| `TestZeFullBinaryCommands` | `cmd/ze/build_tag_test.go` | all-tags binary has everything | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A -- no numeric inputs | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `build-tag-ze-linux` | `test/install/build-tag-ze-linux.ci` | ze binary built with ze_linux has install local | |
| `build-tag-ze-setup` | `test/install/build-tag-ze-setup.ci` | ze-setup binary has appliance and provision | |

### Interop Tests
Not applicable: build system change, no protocol behavior.

## Files to Modify
- `cmd/ze/setup_features_full.go` - delete (replaced by tag-specific files)
- `cmd/ze/setup_features_stripped.go` - delete (no-tag default is now stripped)
- `cmd/ze/setup_features_stripped_test.go` - delete or update
- `cmd/ze/appliance_import.go` - change tag from `!ze_stripped` to `ze_appliance`
- `cmd/ze/install/register.go` - conditional registration or split by tag
- `Makefile` - add bin/ze-appliance and bin/ze-setup targets, change bin/ze to use ze_linux, change bin/ze-stripped to no tags
- `gokrazy/ze/config.json` - GoBuildTags: [] (empty)
- `mk/test-integration.mk` - change ze_stripped to no-tag build

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] | N/A |
| CLI commands/flags | [x] | No new commands, but registration gating changes |
| Functional test for new RPC/API | [ ] | N/A |
| Doctor check for runtime dependencies | [ ] | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- ze-setup binary |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [x] | `docs/guide/ze-install.md` -- ze-setup replaces ze for provisioning |
| 6 | Has a user guide page? | [x] | `docs/guide/ze-install.md` -- update command examples |
| 12 | Internal architecture changed? | [x] | `docs/architecture/cli/plugin-modes.md` -- build tag system |
| 16 | Any changed source file is referenced by existing doc source anchors? | [x] | grep docs/ for source anchors |

## Files to Create
- `cmd/ze/setup_features_linux.go` - `//go:build ze_linux`; imports install/local, install/systemd, uninstall/*, connect
- `cmd/ze/setup_features_setup.go` - `//go:build ze_setup`; imports install/remote
- `cmd/ze/setup_features_appliance.go` - `//go:build ze_appliance`; imports internal/appliance (replaces appliance_import.go)
- `test/install/build-tag-ze-linux.ci` - functional test
- `test/install/build-tag-ze-setup.ci` - functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | make ze-lint && make ze-unit-test && make ze-functional-test |
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

1. **Phase: Wiring** -- create new tag files, verify compilation with each tag combination
   - Tests: TestZeLinuxBinaryCommands, TestZeSetupBinaryCommands, TestZeStrippedBinaryCommands
   - Files: new setup_features_*.go files
   - Verify: each tag combination compiles and registers the correct commands

2. **Phase: Tag migration** -- replace ze_stripped with positive tags
   - Delete setup_features_full.go, setup_features_stripped.go, appliance_import.go
   - Update Makefile targets
   - Update gokrazy config
   - Update mk/test-integration.mk
   - Verify: `make build` produces all three binaries

3. **Phase: Install dispatch split** -- gate install/uninstall root handler registration
   - The "install" root handler registers unconditionally today; needs to only register when at least one subcommand (local, remote, systemd) is present
   - Same for "uninstall"
   - Verify: no-tag build has no "install" command

4. **Phase: Documentation** -- update architecture docs and install guide
5. **Functional tests** -- verify binary contents via .ci tests
6. **Full verification** -- make ze-verify

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-12 has implementation |
| Correctness | Each tag combination produces exactly the right command set |
| Naming | Tags are ze_linux, ze_appliance, ze_setup (lowercase, underscore) |
| Data flow | Build tags flow from Makefile through Go compiler to init() registration |
| No regressions | Existing tests still pass with new tag structure |
| ze_stripped eliminated | Zero occurrences of ze_stripped in repo |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| setup_features_linux.go | grep "go:build ze_linux" cmd/ze/ |
| setup_features_setup.go | grep "go:build ze_setup" cmd/ze/ |
| setup_features_appliance.go | grep "go:build ze_appliance" cmd/ze/ |
| ze_stripped deleted | grep -r "ze_stripped" . returns nothing |
| bin/ze-appliance in Makefile | grep "ze-appliance" Makefile |
| bin/ze-setup in Makefile | grep "ze-setup" Makefile |
| gokrazy empty tags | grep "GoBuildTags" gokrazy/ze/config.json shows [] |
| All four binaries build | make build succeeds |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Command availability | Verify provisioning commands (root-required) are not in the on-device binary by default |
| Build tag bypass | Verify no command registers unconditionally that should be gated |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior; RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural, DESIGN phase |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Positive build tags over negative | Keep ze_stripped with !ze_stripped | Positive is additive/composable; negative inverts safety (new code included by default) |
| Three tags over two | ze_device + ze_setup only | Appliance and provisioning are different concerns; operator may want one without the other |
| Same cmd/ze/ entry point over separate cmd/ze-setup/ | New main.go for ze-setup | Avoids code duplication; tags are the Go-native mechanism for this |
| No-tag default is stripped over requiring a strip tag | Default build gets everything | Matches gokrazy (no tags needed in config); new code is excluded by default until explicitly tagged in |

## Known Limitations
- Developer convenience: `go build ./cmd/ze` now produces a stripped binary, not a full one. Dev workflow needs `make build` or explicit tags.
- Three-tag cardinality: 8 possible combinations, but only 4 are meaningful (stripped, linux, setup+appliance, all). Invalid combinations (e.g. ze_setup alone without ze_appliance) are harmless but produce a binary with only provision commands.

## Implementation Summary

### What Was Implemented
- Three positive build tags (ze_linux, ze_appliance, ze_setup) replace negative ze_stripped
- New tag files: setup_features_ondevice.go, setup_features_setup.go, setup_features_appliance.go
- Internal component files migrated: !ze_stripped -> ze_linux, ze_stripped -> !ze_linux
- Redundant stripped package files deleted (doc_stripped.go, register_stripped.go)
- Makefile targets: bin/ze (ze_linux), bin/ze-appliance (ze_appliance), bin/ze-setup (ze_setup), bin/ze-stripped (no tags)
- Gokrazy config: GoBuildTags: []
- QEMU integration: no-tag build
- Build tag validation tests for all five combinations
- All ze_stripped references eliminated from code, docs, and learned summaries

### Bugs Found/Fixed
- Go treats _linux.go filename suffix as implicit GOOS=linux constraint. setup_features_linux.go was silently excluded on macOS. Renamed to setup_features_ondevice.go.
- Build tag tests with negative assertions fail when all tags combined. Fixed with compound constraints (ze_linux && !ze_appliance && !ze_setup).

### Documentation Updates
- docs/features.md: updated build tag references and source anchors
- plan/learned/850-appliance-command-plugin.md: updated file references
- plan/learned/834-stripped-build-and-iso-coverage.md: updated tag references
- ai/CODE-TO-DOCS.md: updated file mapping
- ai/LEARNED-INDEX.md: added 853 entry

### Deviations from Plan
- Spec did not list internal/component/ files using ze_stripped (config/system/backend_ze*.go, selfupdate.go, cmd/update/firmware_stripped_test.go, etc.). These were discovered during audit and migrated.
- Phase 3 (install dispatch split) was unnecessary: install root handler is only pulled in transitively through subcommand imports, so it naturally absent in no-tag builds.
- Files renamed from *_stripped* to *_minimal* to eliminate ze_stripped substring from documentation references.
- setup_features_linux.go renamed to setup_features_ondevice.go due to Go filename convention.

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
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| ze and ze-setup are distinct binaries with correct command sets | functional test | build-tag-ze-linux.ci, build-tag-ze-setup.ci |
| ze_stripped fully eliminated | grep evidence | zero matches across repo |
| gokrazy builds without tags | build evidence | gokrazy config + make ze-stripped |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

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
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered
- [ ] Architecture docs and guides updated

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit over implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests N/A (build system change)
- [ ] Goal Validation table filled

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Commit A: code + tests + docs + spec + learned summary
- [ ] Commit B: git rm plan/spec-build-tag-split.md
