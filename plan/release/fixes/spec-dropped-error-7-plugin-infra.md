# Spec: dropped-error-7-plugin-infra

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Priority | P1 |
| Release reason | 20 errors in the plugin server, the process manager, the public RPC transport and the SDK are caught and logged only below WARN: an RPC error answer that never reaches its plugin, a managed config push that fails, a plugin process group that cannot be signalled, each with no trace at the default level |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child of `plan/release/fixes/spec-dropped-error-0-umbrella.md`, which holds the
resolution policy and the tree-wide acceptance. Door: `ai/doors/plugin-infra.md`.

Population: every `error-dropped-below-warn` finding of
`./le arch refuse-by-default report` under `internal/component/plugin/`,
`pkg/plugin/` and `pkg/ze/`. At HEAD on 2026-11-02: **20 instances in 8 files**
(plugin server 10, process 5, `pkg/plugin/rpc` 2, `pkg/plugin/sdk` 3).

Each instance is resolved by exactly one answer, judged in this order
(umbrella, Key Design Decisions):

| Order | When | Answer |
|-------|------|--------|
| 1 | Going on is wrong: the caller would act on the failure | Return the error, or fix the behavior the drop hid |
| 2 | Going on is right and an operator can act | WARN with what, why and the fix (`ai/rules/user-facing-errors.md`) naming the plugin |
| 3 | Going on is right and nobody can act (an error answer to a plugin whose connection is already closed) | `// refuse-by-default: guards <Symbol> -- <why>` on the line above |

`pkg/plugin/rpc` and `pkg/plugin/sdk` are the public API: no exported
signature changes here. An instance there is answered inside the package
(WARN through the SDK's logger, or the annotation).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/plugin-infra.md`
  → Constraint: a field or method name in `rpc` is wire; no exported name or signature changes in `pkg/`
  → Constraint: one plugin failure stops the startup tier, by owner ruling; every server error path calls `PluginFailed` and `proc.Stop()`, so no answer here may make a failure survivable
  → Constraint: the SDK stays generic; no plugin-specific branch is added to report an error
- [ ] `docs/architecture/api/process-protocol.md` - error answers, shutdown
  → Constraint: an error answer is a line on the plugin's connection; when the connection is gone, the server's process supervision reports the plugin's exit
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: what, why, fix
- [ ] `docs/contributing/ze-go-style.md` - "Error dropped below WARN"
  → Constraint: return, count, or WARN; the annotation names a guard

**Key insights:**
- Most instances send an error answer to a plugin; failing that send means the connection is gone, which process supervision already reports, so these are answer 3 naming that guard, unless the read shows the plugin is still alive.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/refusebydefault/dropped.go` - the shape
- [ ] `internal/component/plugin/server/dispatch.go` - a failed `SendError` for an unknown method is logged at Debug

**Behavior to preserve:**
- The process protocol, the five stages, the startup-tier failure ruling and every exported `pkg/` name.

**Behavior to change:**
- Each listed instance resolved by one answer.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| refuse-by-default dropped-error shape | `internal/le/arch/refusebydefault/dropped.go` `droppedError` | Lists these 20 | Applies: the acceptance check |
| Guard annotation | `internal/le/arch/refusebydefault/annotation.go` | Accepts a named guard | Applies: answer 3 only |
| Process supervision | `internal/component/plugin/process/process.go`, `PluginFailed` in `plugin/server` | Reports a plugin that exited or failed | Applies: the guard an answer-3 annotation names where a send failed because the plugin is gone |
| Managed server | `internal/component/plugin/server/managed_serve.go` (`docs/architecture/fleet-config.md`) | Serves and pushes config to managed edges | Applies: a failed push is answer 2 naming the edge |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A failed send, signal or stop inside the plugin server, the process manager, the RPC transport or the SDK.

### Transformation Path
1. The branch catches the error.
2. It returns it, WARNs, or carries the annotation naming the supervision that reports the cause.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ plugin | RPC error answers over the connection | No |
| Hub ↔ managed edge | `pkg/fleet` push | No |

### Integration Points
- The plugin server and SDK loggers, process supervision.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Existing paths |
| No unintended coupling (components stay isolated) | Yes | No new import; `plugin` still imports nothing from `bgp` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Supervision already reports exits |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Control paths |
| Registration over hardcoding, outbound | N-A | Nothing registers |
| Registration over hardcoding, inbound | N-A | No list learns a name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 20 are still the population | Report of 2026-11-02 | Work from the fresh report | Report filtered to this child's paths before the first edit | unvalidated |
| A-2 | A failed error answer means the plugin's connection is closed and supervision reports it | Door: process boundary | A live plugin missing its answer waits forever; that is answer 2 or 1 | Read each send site's connection state handling | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A change in `pkg/` alters an exported contract | `./le plugin declarations check` or an external build breaks | No exported change; internal logging only |
| R-2 | A WARN per RPC answer under a misbehaving plugin floods | Plugin `.ci` log grows per call | WARN once per plugin and condition |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Log noise, or a startup path that now fails where it went on |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `spec-command-surface-parity` (in progress in `plugin/server`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default report` | → | `droppedError` over this child's paths | Report filtered to this child's paths: zero findings of the shape |
| A managed config push that fails | → | `ManagedServer.pushConfigChanged` | `TestManagedPushFailureIsWarned` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le arch refuse-by-default report '|' json`, filtered to this child's paths | No `findings` entry with rule `error-dropped-below-warn` |
| AC-2 | Each answer-1 instance | A unit test where the caller observes the error |
| AC-3 | Each file with an answer-2 WARN | A unit test asserts one WARN line names the plugin or edge, why and the fix |
| AC-4 | Each annotation | Names a guard the file can see |
| AC-5 | `go test -race` over the changed packages, `./le plugin declarations check`, and the plugin `.ci` the change reaches | Green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestManagedPushFailureIsWarned` | `internal/component/plugin/server/managed_serve_test.go` | AC-3 | |
| One test per answer-1 instance | The package of the instance | AC-2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| unannotated findings under this child's paths | 0 | 0 | N/A | 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test/plugin` and `test/managed` `.ci` the change reaches | `test/` | Unchanged plugin startup and dispatch | |

## Files to Modify
- `internal/component/plugin/process/process.go` - 5: `Process.startInternal.func.func` 570; `Process.startExternal` 700; `stderrRelayReader.extendDeadline` 817; `drainStderrRelay` 849; `pluginGroup.signalLocked` 1128
- `internal/component/plugin/server/dispatch.go` - 4: `Server.dispatchPluginRPC.func` 123; `Server.dispatchPluginRPC` 141; `Server.handleCodecRPC` 513,519
- `internal/component/plugin/server/dispatch_registry.go` - 3: `Server.serveEngineOpJSON` 252,261,266
- `internal/component/plugin/server/managed_serve.go` - 2: `ManagedServer.handleRequest` 423; `ManagedServer.pushConfigChanged` 493
- `internal/component/plugin/server/startup_removal.go` - 1: `Server.stopRemovedProcess` 83
- `pkg/plugin/rpc/panic_guard.go` - 2: `Conn.endAfterPanic` 72; `MuxConn.endAfterPanic` 85
- `pkg/plugin/sdk/sdk_dispatch.go` - 2: `Plugin.answerDrain` 380,392
- `pkg/plugin/sdk/sdk_source_policy.go` - 1: `Plugin.answerSourcePolicy` 278

Lines are those of HEAD on 2026-11-02; the report is the current list.

## Files to Create
- Test additions in the packages above; no new source file.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No command |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No output change |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No dependency |
| Prometheus counters/metrics | N-A | No counter planned |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | No exported change |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared `// Design:` docs: `docs/architecture/api/process-protocol.md`, `docs/architecture/fleet-config.md`. A log level changes neither; confirm with `./le spec citation anchors spec plan/release/fixes/spec-dropped-error-7-plugin-infra.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring** -- run the report filtered to this child's paths; write the managed push test red.
2. **Phase: Classify and resolve** -- every instance gets its answer and reason in Design Insights; tests first for answers 1 and 2; annotations last.
3. **Phase: Prove** -- AC-1 to AC-5.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every instance has its answer at file:line |
| Correctness | No exported `pkg/` change; no failure made survivable against the startup-tier ruling |
| Rule: user-facing-errors | Each WARN names the plugin or edge, why and the fix |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero findings under this child's paths | `./le arch refuse-by-default report '|' json` filtered by path |
| Package tests | `./le job run label plugin command go test -race ./internal/component/plugin/... ./pkg/plugin/...` |
| Declarations | `./le plugin declarations check` |
| Functional | `./le test bgp plugin <name>` for the `.ci` the change reaches |
| Change set judged | `./le verify changed` after the closing commit |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Error leakage | A WARN that prints the plugin hub token or CA material |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

The class row for the whole population is owned by the umbrella.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| No exported `pkg/` change | Return new errors from SDK methods | A changed signature breaks plugins outside this repository |
| The umbrella's three answers, in order | Raise every line to WARN | A WARN does not make a default correct |

## Known Limitations
- None.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, and `./le verify changed` over the closing commits
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason): N-A, no wire change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-dropped-error-7-plugin-infra.md` only, in the same `./le commit create` script
