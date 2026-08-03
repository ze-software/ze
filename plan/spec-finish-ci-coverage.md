# Spec: finish-ci-coverage

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | op-1 Tier-1 commands |
| Updated | 2026-08-03 |

**Phase in hand: the op-1 Tier-1 command `.ci` item only.** Its acceptance
criteria are filled below. The other four work items in `## Task` stay captured
intent and are not designed yet; each moves into this table when someone picks
it up, one phase at a time.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `git log -p plan/deferrals.md` (pre-2026-07-06) - original deferral rows + evidence

## Task

Write the deferred `.ci` functional tests whose feature code already exists and is unit-tested. No hard infra blocker - this is per-knob/per-command runner plumbing that was batched-deferred.

This is a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Each item below was confirmed still-open against the codebase with a producing `file:line`. Split into phases when picked up; the sections after Task are lightweight scaffolding to be filled at design time.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **Env-knob `.ci` (L215)** - 0 of ~12 exist: openwait, announce-delay, pid-file, pprof, l2tp-log-level, bridge-ack, migration-env. Code reads env directly; unit tests cover plumbing.
- **op-1 Tier-1 command `.ci` (L217)** - ~4 of 10 exist. Missing: system-cpu, system-date, interface-type, interface-errors, generate-wireguard-keypair.
- **cli-dispatch `.ci` (L83)** - validate-config done; missing `set interface create` and `update peeringdb`.
- **no-congestion-initial chaos `.ci` (L118)** - UNBLOCKED - ze-chaos multi-peer orchestration now exists (`mk/test-chaos.mk --peers`); just needs writing.
- **gRPC-over-wire `.ci` (L40)** - engine path covered by `test/plugin/grpc-execute.ci`; a true gRPC-wire test needs grpcio/grpcurl vendored (tooling gate).

## Required Reading

### Source files / docs

- [ ] `internal/test/runner/` (functional runner conventions)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `test/scripts/ze_api.py` (test plugin API)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `test/plugin/grpc-execute.ci` (existing engine-path gRPC coverage)
  -> Constraint: verify current behaviour against this source before designing.

## Current Behavior (MANDATORY)

**Source files read** (re-verified 2026-08-03 for the op-1 phase):

- [ ] `internal/component/cmd/show/system.go` -- `handleShowSystemCPU` and
  `handleShowSystemDate`, registered as `ze-show:system-cpu` and
  `ze-show:system-date` in `internal/component/cmd/show/show.go`.
- [ ] `internal/component/iface/cmd/show_interface.go` -- `showInterfaceByType`,
  `showInterfaceErrors`, `showInterfaceBrief`, and the RPC registrations.
- [ ] `internal/component/iface/yang/ze-iface-interface-cmd.yang` -- the
  `ze:command` declarations the dispatcher registers command keys from.
- [ ] `internal/plugins/diag/diag.go` -- `RunWgKeypair`, registered as the local
  command `generate wireguard keypair` in `internal/plugins/diag/register.go`.
- [ ] `internal/component/plugin/server/command.go` -- `LoadBuiltinsWithAliases`,
  `Dispatcher.updateSortedKeys`, `matchBuiltinTokens`, `matchCommandTokens`.
- [ ] `internal/test/runner/runner_exec.go` -- how a `.ci` command is executed
  (working directory, `option=env`, per-command `exit=`).

**Behavior to preserve:**
- Every other `show` and `generate` handler, and the whole `.ci` runner contract.
- `show interface` with no argument still lists every interface in full detail.

**Behavior to change (found by this phase, fixed in it):**
- `brief`, `type`, `errors`, and `rate` each declared `ze:command
  "ze-show:interface"`, the same wire method as their parent container.
  `LoadBuiltinsWithAliases` registers one dispatcher key per YANG path, and
  `matchBuiltinTokens` tries the LONGEST key first, so the key consumed the
  keyword and `handleShowInterface` was handed the tokens after it. Its
  `switch args[0]` therefore never saw the keyword: `show interface errors`
  answered with EVERY interface, `show interface brief` with full detail,
  `show interface rate` with the interface list, and `show interface type <t>`
  with its usage text. Each now has its own wire method and handler, matching
  the sibling `scan` / `detail` / `counters` pattern in the same file.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze-test` functional runner executing `.ci` files against a live daemon

### Transformation Path
1. An already-shipped, unit-tested feature is selected
2. A `.ci` test drives it end-to-end through `ze cli` / plugin dispatch
3. The test asserts the observable daemon behaviour

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `.ci` runner -> daemon | plugin dispatch / CLI one-shot | [ ] |
| test plugin -> engine | `ze_api.py` API commands | [ ] |

### Integration Points
- `internal/test/runner/`
- `test/scripts/ze_api.py`
- the feature handlers under test

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - new commands/views/families/handlers register and are core-discovered, not hardcoded into a core/shared package (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The verified `file:line` evidence in the Task items still holds at design time | 2026-07-06 backlog triage | Re-scope the item | grep/LSP at design time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Scope drift when the umbrella is split into per-item specs | Item needs its own design doc | Split into a dedicated spec and re-point |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `.ci` dispatches `show system cpu` | → | `handleShowSystemCPU` | `test/plugin/show-system-cpu.ci` |
| `.ci` dispatches `show system date` | → | `handleShowSystemDate` | `test/plugin/show-system-date.ci` |
| `.ci` dispatches `show interface type <t>` | → | `handleShowInterfaceType` -> `showInterfaceByType` | `test/plugin/show-interface-type.ci` |
| `.ci` dispatches `show interface errors` | → | `handleShowInterfaceErrors` -> `showInterfaceErrors` | `test/plugin/show-interface-errors.ci` |
| `.ci` runs `ze generate wireguard keypair` | → | `RunWgKeypair` | `test/parse/cli-generate-wireguard-keypair.ci` |
| unit test shims `wg` on PATH | → | `RunWgKeypair` genkey -> pubkey pipe | `TestRunWgKeypair_PipesGenkeyIntoPubkey` |

**Still not wired, and named so it is not mistaken for done:** the env-knob,
cli-dispatch, chaos, and gRPC-over-wire work items in `## Task` have no test and
no phase yet.

## Acceptance Criteria

Phase op-1 Tier-1 commands. One AC per missing command, plus the defect the
phase uncovered.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show system cpu` on a running daemon | Answers `done` with `num-cpu`, `num-goroutines`, `max-procs` (each an integer >= 1) and a `go-version` starting `go`. On a platform where host inventory is supported, exactly one of `hardware` / `hardware-error` is present, and `hardware.logical-cpus` is >= `num-cpu` |
| AC-2 | `show system date` on a running daemon | Answers `done` with `time`, `unix`, `unix-nano`, `timezone`, `utc-offset-secs`. The three renderings agree: `unix-nano / 1e9 == unix`, the RFC3339 `time` parses to the same epoch and carries `utc-offset-secs`, and `unix` is the caller's own wall clock |
| AC-3 | `show interface type <t>`, with `<t>` read off the running host | Answers `done` with an `interfaces` wrapper holding exactly the interfaces of that type in `show interface`, and nothing else |
| AC-4 | `show interface type <unmatched>` | REFUSED with `unknown interface type`, and the refusal lists the types the running set actually has |
| AC-5 | `show interface errors` | Answers `done` with an `interfaces` wrapper whose rows and four counter values equal exactly the links `show interface` reports with a non-zero `rx-errors` / `rx-dropped` / `tx-errors` / `tx-dropped`. Links with all-zero counters are excluded |
| AC-6 | `ze generate wireguard keypair extra-arg` | Exits 1, says `no arguments accepted`, prints the usage, and prints no key |
| AC-7 | `ze generate wireguard keypair` with a `wg` on PATH | Runs `wg genkey`, feeds THAT private key to `wg pubkey` on stdin, and prints `private: <k>` / `public:  <k>` |
| AC-8 | `show interface brief`, `type`, `errors`, `rate` dispatched by their full command text | Each reaches its own handler. They shared one wire method with their parent container, so the dispatcher consumed the keyword and `handleShowInterface` never saw it. Proven end to end for `type`, `errors` and `rate`; `brief` goes through the identical registration and is proven at the handler by `TestHandleShowInterfaceBrief`, with no `.ci` of its own (it was not in the op-1 missing list, and a fourth end-to-end proof of one mechanism buys little) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRunWgKeypair_PipesGenkeyIntoPubkey` | `internal/plugins/diag/diag_test.go` | AC-7. Replaces a `LookPath("wg")` skip that never ran | done |
| `TestRunWgKeypair_ReportsMissingWg` | `internal/plugins/diag/diag_test.go` | exit 1 and no partial output when `wg` is absent | done |
| `TestHandleShowInterfaceRejectsStrayToken` | `internal/component/iface/cmd/show_interface_test.go` | AC-8. The bare handler owns the no-argument form only | done |
| `TestHandleShowInterfaceBrief` | `internal/component/iface/cmd/show_interface_test.go` | AC-8. Brief returns the compact shape, not full detail | done |
| `TestHandleShowInterfaceType*` | `internal/component/iface/cmd/show_interface_test.go` | AC-3, AC-4 at the handler | done |
| `TestHandleShowInterfaceErrorsShape` | `internal/component/iface/cmd/show_interface_test.go` | AC-5 wrapper shape | done |
| `TestIfaceInterfaceCmdSchemaOwnsInterface` | `internal/component/iface/yang/show_cmd_schema_test.go` | the four new wire methods are declared by the owning module | done |
| `TestShowYANGDoesNotOwnRelocatedCommands` | `internal/component/cmd/show/yang/self_containment_test.go` | and are NOT declared centrally | done |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-system-cpu.ci` | `test/plugin/` | AC-1: an operator reads CPU state off a running daemon | done |
| `show-system-date.ci` | `test/plugin/` | AC-2: an operator reads the daemon clock | done |
| `show-interface-type.ci` | `test/plugin/` | AC-3, AC-4: filter interfaces by type, and be told what is valid | done |
| `show-interface-errors.ci` | `test/plugin/` | AC-5: find the links with errors or drops | done |
| `cli-generate-wireguard-keypair.ci` | `test/parse/` | AC-6: the offline CLI command resolves and rejects arguments | done |
| `show-interface-rate.ci` (corrected) | `test/plugin/` | AC-8: its assertion had pinned the aliasing defect | done |

Every one is proven by mutation: each was re-run with the behaviour under test
broken at the producer and observed to FAIL. The interface pair needs no
synthetic mutation to prove it either way, because both were RED against the
unfixed dispatcher and turned green only with the wire methods split.

## Files to Modify

- `internal/test/runner/` - see Task work items
- `internal/component/cmd/show/show.go` - see Task work items

## Implementation Steps

1. **Phase: split** - if the umbrella covers unrelated items, split into per-item specs first.
2. **Phase: design** - for the chosen item, re-verify the `file:line` evidence and fill the Data Flow / Wiring / AC sections above.
3. **Phase: wiring** - register entry points, write the failing wiring test.
4. **Phase: implement (TDD)** - write test, fail, implement, pass, per work item.
5. **Full verification** - `make ze-verify`.
6. **Complete spec** - fill audit tables, write `plan/learned/NNN-<name>.md`, two-commit closure.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] Every chosen work item has feature code + test
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Notes
- Skeleton = captured intent, not a designed spec (see `ai/rules/planning.md`). Moves to `design` when someone picks it up.

### Post-wave corrections (2026-07-10)

- New obligation for the chaos `.ci` work item (no-congestion-initial, L118): the chaos orchestrator now validates listener/port-range conflicts at entry. `ValidateConfigRangeConflicts` (`internal/chaos/orchestrator/conflict.go`) derives the BGP and listen port range bases from the profile list and delegates to `ValidateRangeConflicts` (`conflict.go`), rejecting web/metrics/mcp listener endpoints that fall inside the derived per-peer port ranges; it is invoked at orchestrator entry (`internal/chaos/orchestrator/run.go`). Any new chaos `.ci` config must place its web/metrics/mcp listeners outside the derived BGP/listen ranges or the orchestrator rejects the run before starting.
