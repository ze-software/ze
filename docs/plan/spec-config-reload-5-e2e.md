# Spec: config-reload-5-e2e

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `docs/functional-tests.md` — functional test format reference
4. `docs/architecture/testing/ci-format.md` — .ci file format
5. `internal/test/runner/runner.go` — test runner orchestration

**Parent spec:** `spec-reload-lifecycle-tests.md` (umbrella)
**Depends on:** All sub-specs 1-4 (full pipeline must be working)

## Task

Write end-to-end functional tests that verify the complete config reload pipeline works from the user's perspective. These tests exercise the full chain: config change → SIGHUP → coordinator → plugin verify → plugin apply → observable behavior change.

Test scenarios:
1. **Verify reject** — plugin rejects invalid config, reload aborts, running unchanged
2. **Multi-plugin** — multiple plugins verified before any apply
3. **Plugin config delivery** — plugin receives updated config sections on reload
4. **Concurrent reload** — rapid successive reloads handled correctly

## Required Reading

### Architecture Docs
- [ ] `docs/functional-tests.md` — .ci file format, test runner capabilities
- [ ] `docs/architecture/testing/ci-format.md` — detailed .ci format reference

### Source Files (MUST read)
- [ ] `internal/test/runner/runner.go` — runOrchestrated(), process tracking, tmpfs
- [ ] `internal/test/runner/record.go` — parseAction(), parseCmd(), record format
- [ ] `internal/test/peer/peer.go` — Checker, action execution
- [ ] `test/reload/add-peer.ci` — existing reload test from sub-spec 3 (pattern to follow)

**Key insights:**
- Functional tests in test/reload/ use the orchestrated runner
- Tests can use tmpfs for config file rewriting
- action=rewrite and action=sighup may need to be added (see sub-spec 3 functional tests)
- Tests need to verify both success and failure paths

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/runner/runner.go` — runOrchestrated() starts peer + daemon, waits for completion
- [ ] `internal/test/runner/record.go` — parses .ci files into Records with expects, commands, tmpfs files
- [ ] `test/reload/add-peer.ci` — basic reload test (if created in sub-spec 3)

**Behavior to preserve:**
- All existing .ci tests must pass unchanged
- Test runner infrastructure unchanged
- Existing action types (notification, send) unchanged

**Behavior to change:**
- Add new .ci tests in test/reload/ for advanced scenarios
- May need test infrastructure enhancements for verify-reject testing

## Data Flow (MANDATORY)

### Entry Point
- `ze-test bgp reload --all` runs all .ci files in test/reload/
- Each .ci file defines config, expects, and actions

### Transformation Path
1. Runner parses .ci file → Record
2. Runner starts test peer + daemon processes
3. Daemon loads initial config, establishes BGP sessions
4. Test peer verifies initial state (expected messages)
5. Test triggers config rewrite (action=rewrite) + SIGHUP (action=sighup)
6. Daemon receives SIGHUP → coordinator → verify → apply
7. BGP sessions change (new connections, dropped connections, new routes)
8. Test peer verifies final state (expected messages on new connections)
9. Test passes if all expects matched

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Test → Daemon | Config file + SIGHUP signal | [ ] |
| Daemon → Plugins | config-verify / config-apply RPCs | [ ] |
| Daemon → Test Peer | BGP TCP sessions | [ ] |

### Integration Points
- test/reload/ directory — new .ci test files
- action=rewrite, action=sighup — test peer actions (from sub-spec 3)
- test runner tmpfs — config file rewriting area
- ze-test bgp reload — test execution command

### Architectural Verification
- [ ] No bypassed layers (tests exercise full pipeline end-to-end)
- [ ] No unintended coupling (tests are black-box — config in, behavior out)
- [ ] No duplicated functionality (each test covers a unique scenario)
- [ ] Zero-copy preserved where applicable (N/A — tests)

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| N/A — this spec is entirely functional tests | N/A | N/A | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A — no new numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `reload-verify-reject` | `test/reload/verify-reject.ci` | Plugin rejects config change, daemon keeps running with old config | |
| `reload-multi-plugin` | `test/reload/multi-plugin.ci` | Two plugins both verified before any config applied | |
| `reload-config-delivery` | `test/reload/config-delivery.ci` | Plugin receives correct config sections after reload | |
| `reload-rapid-sighup` | `test/reload/rapid-sighup.ci` | Two SIGHUPs in rapid succession, daemon handles correctly | |

## Files to Modify
- `docs/functional-tests.md` — document reload test category and new action types
- `docs/architecture/testing/ci-format.md` — document action=rewrite, action=sighup syntax

## Files to Create
- `test/reload/verify-reject.ci` — functional test: plugin verify rejection
- `test/reload/multi-plugin.ci` — functional test: multi-plugin coordination
- `test/reload/config-delivery.ci` — functional test: config section delivery
- `test/reload/rapid-sighup.ci` — functional test: rapid successive reloads

## Implementation Steps

### Step 1: Write verify-reject test
Create .ci test where config change would cause a plugin to reject verification. Verify daemon continues with old config, no sessions disrupted.

### Step 2: Write multi-plugin test
Create .ci test with two plugins that receive config. Verify both are verified before either is applied.

### Step 3: Write config-delivery test
Create .ci test that verifies plugin receives correct config sections (per WantsConfigRoots) after reload.

### Step 4: Write rapid-sighup test
Create .ci test that sends two SIGHUPs quickly. Verify daemon handles both correctly (second may be no-op if config unchanged between them).

### Step 5: Update documentation
Document new reload test category in docs/functional-tests.md. Document action=rewrite and action=sighup syntax in ci-format.md.

### Step 6: Verify
Run `make functional` — all tests pass including new reload tests.

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Verify reject test | | | |
| Multi-plugin test | | | |
| Config delivery test | | | |
| Rapid SIGHUP test | | | |
| Documentation update: functional-tests.md | | | |
| Documentation update: ci-format.md | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| reload-verify-reject.ci | | | |
| reload-multi-plugin.ci | | | |
| reload-config-delivery.ci | | | |
| reload-rapid-sighup.ci | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `docs/functional-tests.md` | | |
| `docs/architecture/testing/ci-format.md` | | |
| `test/reload/verify-reject.ci` | | |
| `test/reload/multi-plugin.ci` | | |
| `test/reload/config-delivery.ci` | | |
| `test/reload/rapid-sighup.ci` | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Design
- [x] No premature abstraction (each test is a concrete scenario)
- [x] No speculative features (tests cover implemented pipeline)
- [x] Single responsibility (each .ci file tests one scenario)
- [x] Explicit behavior (expected messages clearly defined)
- [x] Minimal coupling (black-box tests — config in, behavior out)
- [x] Next-developer test (follows existing .ci test patterns)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Feature code integrated into codebase
- [ ] Functional tests verify end-user behavior

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes
