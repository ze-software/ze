# Spec: dropped-error-9-api-cli-config

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | cli |
| Priority | P1 |
| Release reason | 8 errors in the looking glass, the web editor, the pipe resolver and `config dump` are caught and logged only below WARN: a pipe reverse lookup that fails answers as if nothing matched, a dump that cannot mask a value goes on, each with no trace at the default level |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child of `plan/release/fixes/spec-dropped-error-0-umbrella.md`, which holds the
resolution policy and the tree-wide acceptance. Doors: `ai/doors/api-surfaces.md`,
`ai/doors/commands-cli.md`, `ai/doors/config-yang.md`.

Population: every `error-dropped-below-warn` finding of
`./le arch refuse-by-default report` under
`internal/component/{api,web,lg,gnmi,mcp,telemetry}`,
`internal/component/{command,cmd,cli,ssh}` and `internal/component/config`
(outside `config/system`, child 8). At HEAD on 2026-11-02: **8 instances in 7
files** (looking glass 5, web 1, command 1, config CLI 1).

Each instance is resolved by exactly one answer, judged in this order
(umbrella, Key Design Decisions):

| Order | When | Answer |
|-------|------|--------|
| 1 | Going on is wrong: the caller would act on the failure | Return the error. `ReverseLookup` (pipe resolution) and `maskValue` (`config dump`) answer an operator's command, so a failure reaches that operator |
| 2 | Going on is right and an operator can act | WARN with what, why and the fix (`ai/rules/user-facing-errors.md`) |
| 3 | Going on is right and nobody can act (an HTTP response write to a browser that left) | `// refuse-by-default: guards <Symbol> -- <why>` on the line above |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/api-surfaces.md`
  → Constraint: the surfaces are thin adapters; an error answer goes through the dispatcher's answer, never a surface-local format
  → Constraint: the LG JSON contract (`docs/architecture/api/birdwatcher-compat.md`) is normative; no error answer changes its envelope
- [ ] `ai/doors/commands-cli.md`
  → Constraint: a handler answers data, never text; an error from pipe resolution travels as the command's error
- [ ] `ai/doors/config-yang.md`
  → Constraint: `config dump` must never print a secret it failed to mask; a masking failure refuses the value rather than printing it
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: what, why, fix
- [ ] `docs/contributing/ze-go-style.md` - "Error dropped below WARN"
  → Constraint: return, count, or WARN; the annotation names a guard

**Key insights:**
- The LG `write*` helpers fail only when the HTTP client has gone; answer 3 naming the request context or the server's own logging.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/refusebydefault/dropped.go` - the shape
- [ ] `internal/component/lg/server.go` - `writeJSONError`: a failed response write is logged at Debug

**Behavior to preserve:**
- Every surface's answer format, the LG contract, and that `config dump` never prints an unmasked secret.

**Behavior to change:**
- Each listed instance resolved by one answer.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| refuse-by-default dropped-error shape | `internal/le/arch/refusebydefault/dropped.go` `droppedError` | Lists these 8 | Applies: the acceptance check |
| Guard annotation | `internal/le/arch/refusebydefault/annotation.go` | Accepts a named guard | Applies: answer 3 only |
| Command error surface | `docs/architecture/cli/error-surface.md` | Where a fault appears on every surface | Applies: answer-1 errors follow it |
| Config secret masking | `internal/component/config/cli/cmd_dump.go` `maskValue` (`docs/architecture/config/syntax.md`) | Masks sensitive leaves in a dump | Applies: a mask failure must refuse, not print |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator's command (pipe, `config dump`), a browser request to the LG or web editor.

### Transformation Path
1. The branch catches the error.
2. It returns it to the command's answer, WARNs, or carries the annotation.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Surface ↔ operator | Command error answer, HTTP response | No |

### Integration Points
- The dispatcher's error answer and the surfaces' loggers.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Errors leave through the command answer |
| No unintended coupling (components stay isolated) | Yes | No new import |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing error surface |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Control paths |
| Registration over hardcoding, outbound | N-A | Nothing registers |
| Registration over hardcoding, inbound | N-A | No list learns a name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 8 are still the population | Report of 2026-11-02 | Work from the fresh report | Report filtered to this child's paths before the first edit | unvalidated |
| A-2 | `maskValue`'s failure path still masks or omits the value | Read of `cmd_dump.go` needed | A secret printed after a failed mask is a P0 leak, fixed first | Read `maskValue` before any change | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A returned pipe error changes output a script reads | A `test/ui` `.ci` turns red | That `.ci` asserted the empty answer; the change goes through the `ze-test-judge` |
| R-2 | `internal/component/command` and `config/cli` carry another session's uncommitted edits | `git status` shows them modified | Commit only this child's hunks (`docs/contributing/committing.md`, "Committing some hunks of a shared file") |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A command that newly answers an error, or log noise |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `spec-command-surface-parity` (in progress in `command` and `config/cli`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default report` | → | `droppedError` over this child's paths | Report filtered to this child's paths: zero findings of the shape |
| A pipe whose reverse lookup fails | → | `ReverseLookup` | `TestReverseLookupFailureReachesTheCaller` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le arch refuse-by-default report '|' json`, filtered to this child's paths | No `findings` entry with rule `error-dropped-below-warn` |
| AC-2 | `ReverseLookup` fails | The command answers an error naming the pipe and the fix |
| AC-3 | `maskValue` fails | `config dump` never prints the unmasked value; the failure is reported |
| AC-4 | Each annotation | Names a guard the file can see |
| AC-5 | Package tests and the `test/ui`, web and LG `.ci` the change reaches | Green |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReverseLookupFailureReachesTheCaller` | `internal/component/command/pipe_resolve_test.go` | AC-2 | |
| `TestDumpMaskFailureNeverPrintsTheSecret` | `internal/component/config/cli/cmd_dump_test.go` | AC-3 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| unannotated findings under this child's paths | 0 | 0 | N/A | 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test/ui` and LG `.ci` the change reaches | `test/` | Unchanged answers; the error where a `.ci` drives it | |

## Files to Modify
- `internal/component/command/pipe_resolve.go` - 1: `ReverseLookup` 60
- `internal/component/config/cli/cmd_dump.go` - 1: `maskValue` 90
- `internal/component/lg/handler_graph.go` - 1: `writeText` 161
- `internal/component/lg/handler_ui.go` - 1: `LGServer.handleUIEvents` 598
- `internal/component/lg/render.go` - 1: `LGServer.writeHTML` 184
- `internal/component/lg/server.go` - 2: `writeJSONError` 740; `writeSVG` 766
- `internal/component/web/editor.go` - 1: `EditorManager.evictInactive` 828

Lines are those of HEAD on 2026-11-02; the report is the current list.

## Files to Create
- Test additions in the packages above; no new source file.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command added; existing commands may answer an error |
| CLI grammar (keyword before value) | N-A | No grammar change |
| Editor autocomplete | N-A | No command |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | Yes | A pipe resolution failure answers through the pipe error path (`ai/rules/cli.md`) |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No dependency |
| Prometheus counters/metrics | N-A | No counter |
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
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared `// Design:` docs: `docs/architecture/config/syntax.md`, `docs/architecture/web-interface.md`. A log level changes neither; the dump masking behavior is checked against `syntax.md` with `./le spec citation anchors spec plan/release/fixes/spec-dropped-error-9-api-cli-config.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring** -- run the report filtered to this child's paths; read `maskValue` first (A-2); write the two tests red.
2. **Phase: Classify and resolve** -- every instance gets its answer and reason in Design Insights; tests first; annotations last.
3. **Phase: Prove** -- AC-1 to AC-5.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every instance has its answer at file:line |
| Correctness | No unmasked secret on any path |
| Rule: user-facing-errors | Each error names the command or page, why and the fix |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero findings under this child's paths | `./le arch refuse-by-default report '|' json` filtered by path |
| Package tests | `./le job run label surfaces command go test -race ./internal/component/command/ ./internal/component/config/cli/ ./internal/component/lg/ ./internal/component/web/` |
| Functional | `./le test functional ui` and the LG `.ci` the change reaches |
| Change set judged | `./le verify changed` after the closing commit |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Error leakage | `config dump` printing a secret after a failed mask |

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
| One child for three small doors | One child per door | 8 instances; three doors read once is the cheaper split |
| Command-path failures are returned | WARN in the daemon log | The operator who ran the command is the one who can act |

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
- [ ] **Commit B:** `remove plan/release/fixes/spec-dropped-error-9-api-cli-config.md` only, in the same `./le commit create` script
