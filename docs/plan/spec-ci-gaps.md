# Spec: ci-gaps — Close All .ci Functional Test Gaps

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/5 |
| Updated | 2026-03-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` — workflow rules
3. `docs/ci-test-coverage.md` — gap analysis with full list
4. `docs/architecture/testing/ci-format.md` — .ci file format reference
5. `docs/functional-tests.md` — test runner reference

## Task

Write .ci functional tests for all 42 features identified in `docs/ci-test-coverage.md` as lacking .ci coverage. Organized in 5 phases by priority. No feature code changes expected — features already exist; tests prove they are wired and usable.

**Source:** `docs/ci-test-coverage.md` (cross-reference of features vs .ci tests)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` — .ci file syntax
  → Constraint: all .ci tests use key=value format with stdin blocks, cmd lines, expect lines
- [ ] `docs/functional-tests.md` — test runner usage and patterns
  → Constraint: two patterns — simple CLI (cmd=foreground + expect=exit/stdout/stderr) and plugin API (ze-peer + Python script + config + orchestration)
- [ ] `docs/ci-test-coverage.md` — the gap analysis this spec closes
  → Decision: 42 gaps across CLI commands, API commands, config behavior, plugin behavior

**Key insights:**
- Simple CLI tests: stdin config + `cmd=foreground:exec=ze <cmd>` + `expect=exit:code=N` + `expect=stdout/stderr:contains=`
- Plugin API tests: ze-peer (background) + Python script (tmpfs) + config (stdin) + orchestration (cmd=background/foreground)
- API command tests need a running daemon with ze-peer — use the plugin test pattern
- Config behavior tests need a peer session to verify the config option has runtime effect

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/config/main.go` — config subcommand dispatch (check, fmt, set, dump, diff, migrate, edit)
- [ ] `cmd/ze/schema/main.go` — schema subcommand dispatch (list, show, handlers, methods, events, protocol)
- [ ] `cmd/ze/signal/main.go` — signal dispatch + RunStatus for ze status
- [ ] `cmd/ze/cli/main.go` — CLI dispatch with --run flag for single command execution
- [ ] `cmd/ze/show/main.go` — show dispatch using BuildCommandTree(readOnly=true)
- [ ] `cmd/ze/run/main.go` — run dispatch using BuildCommandTree(readOnly=false)
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` — peer list/show/add/remove/pause/resume handlers
- [ ] `internal/component/bgp/plugins/cmd/cache/cache.go` — cache list/retain/release/expire/forward handlers
- [ ] `internal/component/bgp/plugins/cmd/commit/commit.go` — commit start/end/eor/rollback/show/withdraw handlers
- [ ] `internal/component/bgp/plugins/cmd/subscribe/subscribe.go` — subscribe/unsubscribe handlers
- [ ] `internal/component/bgp/plugins/cmd/raw/raw.go` — raw message injection handler
- [ ] `internal/component/bgp/plugins/route_refresh/handler/handler.go` — route-refresh command handler

**Behavior to preserve:**
- All existing .ci tests continue to pass
- Test patterns follow existing conventions (stdin blocks, ze-peer, Python ze_api)
- No changes to test runner or framework

**Behavior to change:**
- None — adding new tests only

## Data Flow (MANDATORY)

### Entry Point
- CLI commands: user invokes `ze <command>` → stdout/stderr + exit code
- API commands: process sends text command via stdin → engine dispatches → peer receives wire bytes or JSON response
- Config behavior: config option set → daemon starts → peer session negotiates → wire bytes reflect config

### Transformation Path
1. Test runner parses .ci file (stdin blocks, tmpfs files, commands, expectations)
2. Runner executes background processes (ze-peer), then foreground (ze daemon or CLI tool)
3. Runner collects output (exit code, stdout, stderr, wire bytes from ze-peer)
4. Runner validates against expect= lines

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| User → CLI | Command-line args + config file | expect=exit + expect=stdout |
| Process → Engine | Text commands via stdin pipe | ze-peer captures wire output |
| Config → Wire | Config option → negotiation → wire encoding | expect=bgp:hex= |
| Engine → Plugin | JSON events via socket | Python script validates structure |

### Integration Points
- `cmd/ze-test/` — test runner binary
- `cmd/ze-peer/` — BGP test peer (--sink, --echo, --port)
- `pkg/testing/ze_api/` — Python API helpers (ready, send, wait_for_ack)

### Architectural Verification
- [ ] No bypassed layers — tests exercise the real entry points
- [ ] No unintended coupling — each .ci test is independent
- [ ] No duplicated functionality — each test covers a unique gap
- [ ] Zero-copy preserved — tests don't change engine code

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config check <file>` | → | `cmd/ze/config/` check handler | `test/parse/cli-config-check.ci` |
| `ze config fmt <file>` | → | `cmd/ze/config/` fmt handler | `test/parse/cli-config-fmt.ci` |
| `ze config set` | → | `cmd/ze/config/` set handler | `test/parse/cli-config-set.ci` |
| `ze schema handlers` | → | `cmd/ze/schema/` handlers | `test/parse/cli-schema-handlers.ci` |
| `ze schema protocol` | → | `cmd/ze/schema/` protocol | `test/parse/cli-schema-protocol.ci` |
| `ze status <config>` | → | `cmd/ze/signal/` RunStatus | `test/plugin/cli-status.ci` |
| `ze cli --run` | → | `cmd/ze/cli/` Execute | `test/plugin/cli-run-command.ci` |
| `ze show <cmd>` | → | `cmd/ze/show/` dispatch | `test/plugin/cli-show.ci` |
| `ze run <cmd>` | → | `cmd/ze/run/` dispatch | `test/plugin/cli-run.ci` |
| `ze exabgp migrate` | → | `cmd/ze/exabgp/` migrate | `test/parse/cli-exabgp-migrate.ci` |
| Process → `bgp peer * list` | → | peer cmd handler | `test/plugin/api-peer-list.ci` |
| Process → `bgp peer * show` | → | peer cmd handler | `test/plugin/api-peer-show.ci` |
| Process → `bgp summary` | → | peer cmd handler | `test/plugin/api-bgp-summary.ci` |
| Process → `bgp peer * add` | → | peer cmd handler | `test/plugin/api-peer-add.ci` |
| Process → `bgp peer * remove` | → | peer cmd handler | `test/plugin/api-peer-remove.ci` |
| Process → `bgp peer * pause/resume` | → | peer cmd handler | `test/plugin/api-peer-pause-resume.ci` |
| Process → `bgp peer * capabilities` | → | peer cmd handler | `test/plugin/api-peer-capabilities.ci` |
| Process → `rib show-in` | → | rib cmd handler | `test/plugin/api-rib-show-in.ci` |
| Process → `rib show-out` | → | rib cmd handler | `test/plugin/api-rib-show-out.ci` |
| Process → `rib clear-in` | → | rib cmd handler | `test/plugin/api-rib-clear-in.ci` |
| Process → `rib clear-out` | → | rib cmd handler | `test/plugin/api-rib-clear-out.ci` |
| Process → `cache list/retain/release/forward` | → | cache cmd handler | `test/plugin/api-cache-ops.ci` |
| Process → `subscribe` | → | subscribe cmd handler | `test/plugin/api-subscribe.ci` |
| Process → `unsubscribe` | → | subscribe cmd handler | `test/plugin/api-unsubscribe.ci` |
| Process → `commit start/end/eor` | → | commit cmd handler | `test/plugin/api-commit-workflow.ci` |
| Process → `commit rollback/show/withdraw` | → | commit cmd handler | `test/plugin/api-commit-lifecycle.ci` |
| Process → `route-refresh` | → | route_refresh handler | `test/plugin/api-route-refresh.ci` |
| Process → `bgp peer * raw` | → | raw cmd handler | `test/plugin/api-raw.ci` |
| Config `connection passive` | → | FSM connect behavior | `test/plugin/config-connection-mode.ci` |
| Config `router-id` per-peer | → | OPEN message | `test/encode/router-id-override.ci` |
| Config `group-updates disable` | → | UPDATE grouping | `test/plugin/config-group-updates.ci` |
| Config `add-path send` per-family | → | ADD-PATH negotiation | `test/plugin/config-addpath-mode.ci` |
| Config `nexthop` per-family | → | Extended NH capability | `test/plugin/config-ext-nexthop.ci` |
| Config `role-strict` | → | Role validation | `test/plugin/config-role-strict.ci` |
| Config `adj-rib-in` | → | Adj-RIB storage | `test/plugin/config-adj-rib.ci` |
| Reload → persist | → | bgp-persist plugin | `test/reload/persist-across-restart.ci` |
| API → adj-rib-in query | → | bgp-adj-rib-in plugin | `test/plugin/adj-rib-in-query.ci` |
| Config → role strict reject | → | role plugin | `test/plugin/role-strict-enforcement.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Phase 1 complete (11 CLI .ci tests) | All 11 tests pass in `make ze-functional-test` |
| AC-2 | Phase 2 complete (10 API peer/summary .ci tests) | All 10 tests pass |
| AC-3 | Phase 3 complete (11 API ops .ci tests) | All 11 tests pass |
| AC-4 | Phase 4 complete (7 config behavior .ci tests) | All 7 tests pass |
| AC-5 | Phase 5 complete (3 plugin behavior .ci tests) | All 3 tests pass |
| AC-6 | Full suite | `make ze-verify` passes with all 42 new tests |
| AC-7 | No regressions | All existing ~269 .ci tests still pass |
| AC-8 | Coverage doc updated | `docs/ci-test-coverage.md` gaps cleared |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| No new unit tests | N/A | This spec creates only .ci functional tests — features already have unit tests |

### Boundary Tests (MANDATORY for numeric inputs)

No new numeric inputs — tests validate existing feature boundaries.

### Functional Tests

#### Phase 1 — CLI Commands (11 tests)

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cli-config-check` | `test/parse/cli-config-check.ci` | User runs `ze config check` on valid config → exit 0, on deprecated config → exit 1 with hint | ~~Done~~ |
| `cli-config-fmt` | `test/parse/cli-config-fmt.ci` | User runs `ze config fmt` → formatted output on stdout | ~~Done~~ |
| `cli-config-set` | `test/parse/cli-config-set.ci` | User runs `ze config set` to change a value → modified config | ~~Done~~ |
| `cli-schema-handlers` | `test/parse/cli-schema-handlers.ci` | User runs `ze schema handlers` → table of handler→module mappings | ~~Done~~ |
| `cli-schema-protocol` | `test/parse/cli-schema-protocol.ci` | User runs `ze schema protocol` → protocol version info | ~~Done~~ |
| `cli-status` | `test/plugin/cli-status.ci` | User runs `ze status` → exit 1 (no daemon running) | ~~Done~~ (moved to test/plugin/) |
| `cli-run-command` | `test/plugin/cli-run-command.ci` | Plugin dispatches "help" command via engine → response received | ~~Done~~ (tests dispatch-command path) |
| `cli-show` | `test/plugin/cli-show.ci` | User runs `ze show help` → list of read-only commands | ~~Done~~ |
| `cli-run` | `test/plugin/cli-run.ci` | User runs `ze run help` → list of all commands | ~~Done~~ |
| `cli-exabgp-migrate` | `test/parse/cli-exabgp-migrate.ci` | User runs `ze exabgp migrate` on ExaBGP config → ze format output | ~~Done~~ |
| `cli-signal-quit` | `test/reload/signal-quit.ci` | Send SIGQUIT to daemon → process exits (exit code captured) | ~~Skipped~~ (`ze signal` has no quit handler; test framework has no action=sigquit) |

#### Phase 2 — API Peer Management (10 tests)

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `api-peer-list` | `test/plugin/api-peer-list.ci` | Process sends `bgp peer * list` → receives peer addresses | |
| `api-peer-show` | `test/plugin/api-peer-show.ci` | Process sends `bgp peer 127.0.0.1 show` → receives peer state/stats | |
| `api-bgp-summary` | `test/plugin/api-bgp-summary.ci` | Process sends `bgp summary` → receives summary table | |
| `api-peer-add` | `test/plugin/api-peer-add.ci` | Process sends `bgp peer <addr> add <config>` → new peer connects | |
| `api-peer-remove` | `test/plugin/api-peer-remove.ci` | Process sends `bgp peer <addr> remove` → peer disconnects | |
| `api-peer-pause-resume` | `test/plugin/api-peer-pause-resume.ci` | Process sends `pause` then `resume` → peer resumes receiving | |
| `api-peer-capabilities` | `test/plugin/api-peer-capabilities.ci` | Process sends `bgp peer * capabilities` → negotiated caps listed | |
| `api-subscribe` | `test/plugin/api-subscribe.ci` | Process sends `subscribe update` → receives UPDATE events | |
| `api-unsubscribe` | `test/plugin/api-unsubscribe.ci` | Process subscribes then unsubscribes → stops receiving events | |
| `api-route-refresh` | `test/plugin/api-route-refresh.ci` | Process sends `route-refresh ipv4/unicast` → ze-peer receives ROUTE-REFRESH message | |

#### Phase 3 — API Operations (11 tests)

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `api-rib-show-in` | `test/plugin/api-rib-show-in.ci` | Process queries Adj-RIB-In → receives stored routes | |
| `api-rib-show-out` | `test/plugin/api-rib-show-out.ci` | Process queries Adj-RIB-Out → receives sent routes | |
| `api-rib-clear-in` | `test/plugin/api-rib-clear-in.ci` | Process clears Adj-RIB-In → subsequent show returns empty | |
| `api-rib-clear-out` | `test/plugin/api-rib-clear-out.ci` | Process clears Adj-RIB-Out → subsequent show returns empty | |
| `api-cache-ops` | `test/plugin/api-cache-ops.ci` | Process sends `cache list`, `cache retain`, `cache release` → expected responses | |
| `api-cache-forward` | `test/plugin/api-cache-forward.ci` | Process forwards cached message to peer → ze-peer receives it | |
| `api-commit-workflow` | `test/plugin/api-commit-workflow.ci` | Process does commit start → route add → commit end → ze-peer receives UPDATE | |
| `api-commit-lifecycle` | `test/plugin/api-commit-lifecycle.ci` | Process does commit start → add → rollback → no UPDATE sent | |
| `api-raw` | `test/plugin/api-raw.ci` | Process sends raw hex bytes → ze-peer receives exact bytes | |
| `cli-run-command-peer` | `test/plugin/cli-run-command-peer.ci` | `ze cli --run "peer list"` against running daemon → peer listed | |
| `cli-show-summary` | `test/plugin/cli-show-summary.ci` | `ze show summary` against running daemon → summary output | |

#### Phase 4 — Config Runtime Behavior (7 tests)

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `config-connection-mode` | `test/plugin/config-connection-mode.ci` | Config with `connection passive` → daemon does not initiate, waits for ze-peer | |
| `router-id-override` | `test/encode/router-id-override.ci` | Config with per-peer `router-id` → OPEN contains that router-id | |
| `config-group-updates` | `test/plugin/config-group-updates.ci` | Config with `group-updates disable` → each route in separate UPDATE | |
| `config-addpath-mode` | `test/plugin/config-addpath-mode.ci` | Config with `add-path send` → ADD-PATH capability negotiated, path-ID in NLRI | |
| `config-ext-nexthop` | `test/plugin/config-ext-nexthop.ci` | Config with extended-nexthop → capability in OPEN, IPv6 NH for IPv4 NLRI | |
| `config-role-strict` | `test/plugin/config-role-strict.ci` | Config with `role-strict` → peer without role gets NOTIFICATION | |
| `config-adj-rib` | `test/plugin/config-adj-rib.ci` | Config with adj-rib-in → routes stored, queryable via API | |

#### Phase 5 — Plugin Behavior (3 tests)

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `persist-across-restart` | `test/reload/persist-across-restart.ci` | bgp-persist plugin: reload → routes resent without peer re-advertising | |
| `adj-rib-in-query` | `test/plugin/adj-rib-in-query.ci` | adj-rib-in plugin: receive routes → query via API → routes returned | |
| `role-strict-enforcement` | `test/plugin/role-strict-enforcement.ci` | role plugin strict mode: peer without role → NOTIFICATION sent | |

### Future
- None — all 42 gaps addressed in this spec

## Files to Modify

- `docs/ci-test-coverage.md` — update gap tables as tests are written
- `cmd/ze/config/main.go` — verify check/fmt/set handlers exist (read-only, for test design)
- `cmd/ze/schema/main.go` — verify handlers/protocol subcommands exist (read-only)
- `cmd/ze/signal/main.go` — verify RunStatus works (read-only)
- `cmd/ze/cli/main.go` — verify --run flag works (read-only)
- `cmd/ze/show/main.go` — verify help output (read-only)
- `cmd/ze/run/main.go` — verify help output (read-only)
- `internal/component/bgp/plugins/cmd/peer/` — verify peer list/show/add/remove handlers (read-only)
- `internal/component/bgp/plugins/cmd/cache/` — verify cache handlers (read-only)
- `internal/component/bgp/plugins/cmd/commit/` — verify commit handlers (read-only)
- `internal/component/bgp/plugins/cmd/subscribe/` — verify subscribe handlers (read-only)
- `internal/component/bgp/plugins/cmd/raw/` — verify raw handler (read-only)
- `internal/component/bgp/plugins/route_refresh/handler/` — verify route-refresh handler (read-only)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A — testing existing RPCs |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | No | N/A — testing existing commands |
| CLI usage/help text | No | N/A |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | Yes | All 42 .ci files below |

## Files to Create

### Phase 1 — CLI Commands
- `test/parse/cli-config-check.ci`
- `test/parse/cli-config-fmt.ci`
- `test/parse/cli-config-set.ci`
- `test/parse/cli-schema-handlers.ci`
- `test/parse/cli-schema-protocol.ci`
- `test/plugin/cli-status.ci` (moved from test/parse/ -- parse runner imposes validation semantics)
- `test/plugin/cli-run-command.ci`
- `test/plugin/cli-show.ci`
- `test/plugin/cli-run.ci`
- `test/parse/cli-exabgp-migrate.ci`
- ~~`test/reload/signal-quit.ci`~~ — skipped: `ze signal` has no quit handler, test framework has no `action=sigquit`

### Phase 2 — API Peer Management
- `test/plugin/api-peer-list.ci`
- `test/plugin/api-peer-show.ci`
- `test/plugin/api-bgp-summary.ci`
- `test/plugin/api-peer-add.ci`
- `test/plugin/api-peer-remove.ci`
- `test/plugin/api-peer-pause-resume.ci`
- `test/plugin/api-peer-capabilities.ci`
- `test/plugin/api-subscribe.ci`
- `test/plugin/api-unsubscribe.ci`
- `test/plugin/api-route-refresh.ci`

### Phase 3 — API Operations
- `test/plugin/api-rib-show-in.ci`
- `test/plugin/api-rib-show-out.ci`
- `test/plugin/api-rib-clear-in.ci`
- `test/plugin/api-rib-clear-out.ci`
- `test/plugin/api-cache-ops.ci`
- `test/plugin/api-cache-forward.ci`
- `test/plugin/api-commit-workflow.ci`
- `test/plugin/api-commit-lifecycle.ci`
- `test/plugin/api-raw.ci`
- `test/plugin/cli-run-command-peer.ci`
- `test/plugin/cli-show-summary.ci`

### Phase 4 — Config Runtime Behavior
- `test/plugin/config-connection-mode.ci`
- `test/encode/router-id-override.ci`
- `test/plugin/config-group-updates.ci`
- `test/plugin/config-addpath-mode.ci`
- `test/plugin/config-ext-nexthop.ci`
- `test/plugin/config-role-strict.ci`
- `test/plugin/config-adj-rib.ci`

### Phase 5 — Plugin Behavior
- `test/reload/persist-across-restart.ci`
- `test/plugin/adj-rib-in-query.ci`
- `test/plugin/role-strict-enforcement.ci`

## Implementation Steps

Each phase is a standalone unit — can be implemented and committed independently.

1. **Phase 1: CLI Commands** (11 tests)
   - Read each CLI handler to understand expected output format
   - Write .ci tests using simple pattern: tmpfs config + cmd=foreground + expect=exit/stdout/stderr
   - `ze status` test: start no daemon, run status → expect exit 1
   - `ze cli --run`, `ze show`, `ze run` tests: need running daemon → use plugin test pattern
   - Run `make ze-functional-test` → verify all 11 pass
   - Self-Critical Review

2. **Phase 2: API Peer Management** (10 tests)
   - Read peer/subscribe/route_refresh command handlers for response format
   - Write .ci tests using plugin pattern: ze-peer + Python process + config
   - Python scripts use ze_api helpers (ready, send, subscribe, wait_for_ack)
   - Validate responses by sending signal routes (success = expected route, failure = unexpected route)
   - Run `make ze-functional-test` → verify all 10 pass
   - Self-Critical Review

3. **Phase 3: API Operations** (11 tests)
   - Read rib/cache/commit/raw handlers for response format
   - Write .ci tests — similar pattern to Phase 2
   - Commit workflow test: start → add route → end → verify ze-peer receives UPDATE
   - Rollback test: start → add route → rollback → verify no UPDATE
   - Run `make ze-functional-test` → verify all 11 pass
   - Self-Critical Review

4. **Phase 4: Config Runtime Behavior** (7 tests)
   - Read FSM and capability negotiation code to understand wire effects
   - Write .ci tests that verify config options affect wire output
   - Connection mode: use ze-peer as initiator with `connection passive`
   - Router-id: check OPEN message bytes contain configured router-id
   - ADD-PATH: check capability in OPEN and path-ID prefix in UPDATE NLRI
   - Run `make ze-functional-test` → verify all 7 pass
   - Self-Critical Review

5. **Phase 5: Plugin Behavior** (3 tests)
   - Persistence: reload test pattern with action=rewrite + action=sighup
   - Adj-RIB-In query: receive route → send API query → validate response
   - Role strict: configure role-strict → peer without role → expect NOTIFICATION
   - Run `make ze-functional-test` → verify all 3 pass
   - Self-Critical Review

6. **Update coverage doc** → clear gaps in `docs/ci-test-coverage.md`
7. **Full verification** → `make ze-verify`
8. **Critical Review** → all 6 checks from `rules/quality.md`
9. **Write learned summary** → `docs/learned/NNN-ci-gaps.md`

### Failure Routing

| Failure | Route To |
|---------|----------|
| .ci test fails — wrong expected output | Read handler code, fix expect= lines |
| .ci test fails — feature doesn't work | Feature is broken — file bug, fix code, then test |
| .ci test fails — test infrastructure issue | Check ci-format.md, compare with working test |
| Feature handler doesn't exist | Feature was never wired — implement handler first |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## Implementation Summary

### What Was Implemented

### Bugs Found/Fixed

### Documentation Updates

### Deviations from Plan

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |

### Tests from TDD Plan
| Test | Status | Location | Notes |

### Files from Plan
| File | Status | Notes |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
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

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `docs/learned/NNN-ci-gaps.md`
- [ ] **Summary included in commit** — NEVER commit implementation without the completed summary. One commit = code + tests + summary.
