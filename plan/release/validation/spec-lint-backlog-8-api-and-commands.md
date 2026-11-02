# Spec: lint-backlog-8-api-and-commands

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | 6 findings of the full lint in the web UI, MCP and the command model; the full lint holds the release (`plan/release/validation/spec-lint-backlog-0-umbrella.md`) |
| RFC gaps | - |
| Depends | - |
| Phase | 3/3 |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Admission uses the one-off WIP exception in `plan/release/validation/spec-lint-backlog-0-umbrella.md`, "Owner's one-off WIP exception".

Child of `plan/release/validation/spec-lint-backlog-0-umbrella.md`. Clear the
findings CI's full `./le go lint run` reported on 2026-11-02 in the web UI and
MCP (door `ai/doors/api-surfaces.md`) and in the command model (door
`ai/doors/commands-cli.md`). By linter: goconst 3, exhaustive 3. Line numbers
are the CI run's.

| File | Line | Linter |
|------|------|--------|
| `internal/component/command/surface.go` | 48 | goconst |
| `internal/component/command/usage.go` | 68 | goconst |
| `internal/component/mcp/tools.go` | 1025 | goconst |
| `internal/component/web/editor.go` | 586 | exhaustive |
| `internal/component/web/fragment.go` | 446 | exhaustive |
| `internal/component/web/workbench_enrich.go` | 144 | exhaustive |

### Refreshed findings

The parent reported a fourth current issue in the ongoing scoped baseline:
modernize requests `maps.Copy` in `internal/component/command/node.go`,
`copyNodeForOverlay` (reported at line 346). The run has not finished, so this
is an added finding, not a complete refreshed inventory or a passing result.

| File | Producer | Linter |
|------|----------|--------|
| `internal/component/command/node.go` | `copyNodeForOverlay` | modernize |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/api-surfaces.md` - thin adapters over the dispatcher
  → Constraint: an adapter never copies a fact the YANG model or the registry holds; a goconst in `mcp/tools.go` that names a command word reads it from the registry where it exists
- [ ] `ai/doors/commands-cli.md` - the command model is infrastructure
  → Constraint: the command model holds no owner's word; `surface.go` and `usage.go` constants stay generic
  → Constraint: `command.AnswerSurface` is the one surface predicate, judged by `TestEveryCommandOnEverySurface`
- [ ] `docs/contributing/ze-go-style.md` - the `exhaustive` C/O/P classification

**Key insights:**
- The three web switches are over editor and fragment kinds; classify each before listing members.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/command/surface.go` - repeated enum diagnostic strings in the `String` methods
- [ ] `internal/component/command/node.go` - `copyNodeForOverlay`, which copies the children map
- [ ] `internal/component/web/editor.go` - the exhaustive site
- [ ] `.golangci.yml` - the linter settings

**Behavior to preserve:**
- Which surfaces offer each command; every web page's rendering.

**Behavior to change:**
- Each finding above, fixed at its source.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Lint runner | `./le go lint run scope <packages>` | Lints a package scope | Applies: the evidence for this child |
| Enum diagnostics | `internal/component/command/surface.go` `Surface.String`, `RegistrationKind.String` | Names enum values in diagnostic output | Reuses a constant for repeated diagnostic text, preserving the text |
| Surface parity | `cmd/ze/hub/surface_parity_test.go` `TestEveryCommandOnEverySurface` | Compares each surface producer with command eligibility | Applies as regression evidence, not as the location of the goconst fix |
| Overlay copy | `internal/component/command/node.go` `copyNodeForOverlay` | Copies the children map while sharing its child pointers | Modernizes the copy while preserving map ownership and child sharing |
| Style rule table and C/O/P | `docs/contributing/ze-go-style.md` | Names the fix each linter expects | Applies |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le go lint run scope "./internal/component/command/... ./internal/component/mcp/... ./internal/component/web/..."`

### Transformation Path
1. Read each finding; fix, or annotate with the reason the linter is wrong there.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None | Source edits inside the packages | No |

### Integration Points
- None beyond the packages listed.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The CI list is current for these packages | CI run of 2026-11-02 | Lines drifted | Source audit found explained web projections already fixed; baseline added the node-copy finding | broken |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A constant shared between surface and usage code couples two concerns | A reviewer sees one constant naming two concepts | One constant per concept, in the file that owns it |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le go lint run scope` over the packages | → | golangci-lint | zero findings in the scoped run |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The scoped lint over the three packages | Zero findings |
| AC-2 | The package tests with their tags, `TestEveryCommandOnEverySurface` included | Pass |
| AC-3 | Any `//nolint` or `//exhaustive:ignore` added | Names its linter and its reason |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| The existing package tests | `internal/component/command`, `mcp`, `web` | AC-2 | |
| `TestEveryCommandOnEverySurface` | `cmd/ze/hub/surface_parity_test.go` (requires `ze_ssh && ze_web`) | AC-2 | |

## Files to Modify
- The files in the Task table.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG, CLI, env, doctor, metrics, BGP family surface | N-A | Lint fixes; no surface changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1-15, 17 | Feature, config, CLI, API, plugin, wire, SDK, RFC, test infrastructure, comparison, architecture, metadata, metrics, inventory, examples | No | Lint fixes change no documented behavior |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/release/validation/spec-lint-backlog-8-api-and-commands.md` |

## Implementation Steps

1. Scoped lint; record the current list.
2. Fix the historical and refreshed findings still present in the scoped run.
3. Scoped lint zero; tests pass.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Constant ownership | Each constant names one concept. Generic command code gains no owner-specific words or registry copies (R-1). |
| Rendering and surfaces | C/O/P fixes preserve web rendering and unknown outcomes. `AnswerSurface` retains every command's offered surfaces. |
| API evidence | Three scopes are lint-clean, tagged tests include `TestEveryCommandOnEverySurface`, and annotations state reasons (AC-1–3). |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Three API scopes cleared | Record zero scoped lint findings and tagged package-test results, including the refreshed node-copy finding (AC-1–2). |
| Surface parity retained | Run the separate hub parity test in the TDD checklist and inspect annotation reasons (AC-2–3). |

### Security Review Checklist

| Concern | What to verify |
|---------|----------------|
| Command exposure | Diagnostic-string extraction leaves command eligibility unchanged, as checked by the hub parity test. |
| Web input and overlay ownership | Switch fixes preserve safe unknown handling. Node-copy modernization keeps a separate children map and existing child sharing. |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le go lint run scope "<the three packages>"` zero; `./le job run label api command go test -race -tags <tags> ./internal/component/command/... ./internal/component/mcp/... ./internal/component/web/...`. Full-run evidence belongs to the umbrella's CI run.
- [ ] Select `<tags>` as `ze_core` plus all unique first-column `ze_` tags in `feature-gates.txt`, per `docs/contributing/running-commands.md`, "A bare go test is not ./le test unit". This includes `ze_ssh` and `ze_web`.
- [ ] Separately run `./le job run label api-parity command go test -race -tags "<tags>" ./cmd/ze/hub/ -run '^TestEveryCommandOnEverySurface$' -count=1` and record its result.

## Implementation Summary

### What Was Implemented
- `3895eccf22` extracts diagnostic and MCP result-key constants in `command/surface.go` and `mcp/tools.go`; `command/node.go` `copyNodeForOverlay` uses `maps.Copy` into the existing separately allocated map.
- The three web projection annotations already explain their selected cases. `usage.go` needs no edit after the surface literals are consolidated.

### Bugs Found/Fixed
- Four current lint findings cleared. No behavior defect found; existing overlay, MCP warning/refusal and surface-parity assertions remain unchanged.

### Documentation Updates
- None. Tracked source-anchor search found the command, completion and MCP pages and their guide/feature consumers. The reviewed output, eligibility, overlay and tool-discovery claims remain true: no command, schema, SDK, dependency, wire behavior, metrics or feature level changes.

### Deviations from Plan
- A-1 was broken: historical web findings were already fixed, and baseline added `copyNodeForOverlay`. Only three Go files needed edits.
- Owner-authorized Linux closure: default Linux and Linux integration lint each completed with `0 issues.`. The server restart interrupted Darwin; Darwin is unverified, and the whole matrix is not claimed complete. macOS-only checks wait post-release under `ai/META-DECISIONS.md`.
- No new TDD red/green cycle: this is a behavior-preserving lint cleanup, covered by existing regression tests. No tests or assertions were changed.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1 treated the old CI list as current | Web projections were explained already; node copy was also flagged | Source audit and scoped baseline | Fix current findings; preserve completed Linux results |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Clear API/command lint findings | Done | `3895eccf22`; Linux scoped lint | Both Linux variants: `0 issues.` |
| Preserve rendering and offered surfaces | Done | `AnswerSurface`, MCP result builders, `copyNodeForOverlay` | Existing tagged race and parity tests pass |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Scoped default Linux and integration lint: `0 issues.` each | Darwin interrupted and unverified; owner-authorized Linux scope |
| AC-2 | Done | Completed package verdicts and `TestEveryCommandOnEverySurface` PASS | Interrupted combined run is not a whole-run pass |
| AC-3 | Done | Commit diff adds no suppression | Three existing web annotations name projection reasons |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Tagged package race tests | Done | command, MCP, web | Completed results preserved below |
| Surface parity | Done | `cmd/ze/hub/surface_parity_test.go` `TestEveryCommandOnEverySurface` | PASS |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/command/surface.go` | Done | Diagnostic constant |
| `internal/component/command/usage.go` | Changed | No edit needed after surface consolidation |
| `internal/component/mcp/tools.go` | Done | Result-key constant |
| `internal/component/web/editor.go` | Changed | Explained projection already present |
| `internal/component/web/fragment.go` | Changed | Explained projection already present |
| `internal/component/web/workbench_enrich.go` | Changed | Explained projection already present |
| `internal/component/command/node.go` | Done | Refreshed modernize finding fixed |

### Audit Summary
- Task requirements: 2 done; acceptance criteria: 3 done; planned test groups: 2 done; files: 3 done, 4 changed as explained above. No partial or skipped Linux deliverable.
- Security: output keys, ordering, error flags and diagnostic strings are identical. The overlay still allocates its own map and shares existing child pointers. No new input, privilege, dependency, allocation bound or concurrency path.
- R-1 resolved: the surface diagnostic constant and MCP result-key constant each name their own concept; usage and elicitation are not coupled to them.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Clear findings in the API/command scopes | Scoped lint | `./le go lint run scope "./internal/component/command/... ./internal/component/mcp/... ./internal/component/web/..."`: default Linux `0 issues.`; Linux integration `0 issues.`; no Darwin verdict |
| Preserve command and rendering behavior | Regression | Tagged race package results below; `TestEveryCommandOnEverySurface` PASS; overlay ownership and MCP warning/refusal tests PASS |

## Work Not Done

No in-scope Linux work remains. Darwin lint is explicitly unverified under the owner's macOS-post-release policy; no whole-matrix success is claimed.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/lint-backlog-8-api-and-commands-opencode-lint-backlog-20261102.md` |
| `./le spec review check` | OK: 3 code files, clean, hashes match |
| Rounds | 1 |
| Reviewer lenses used | Independent correctness/security and tests/requirements; reviewer did not author implementation |
| Final status | 0 BLOCKER / 0 ISSUE |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| none | none | No source finding | Three files in `3895eccf22` | No closure code edits |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| Three implementation files | Yes | `git show 3895eccf22` names node.go, surface.go and tools.go; current diff against that commit is empty |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Linux scoped lint clean | Inspected native lint log: `Running ze linter...`, `0 issues.`, `Running ze linter (GOOS=linux, integration tag)...`, `0 issues.`; Darwin start only |
| AC-2 | Tagged race packages and parity pass | Inspected completed logs: command 27.940s, grammar 1.034s, registry 20.682s, MCP 17.463s; separate web 216.250s, web/testing and web/yang cached PASS; hub 4.905s, named parity test PASS 3.70s |
| AC-3 | Reasons retained | Source grep shows explained `exhaustive:ignore` projections in editor, fragment and workbench enrichment; commit adds none |

The commands used `CGO_ENABLED=1` and tags consisting of `ze_core` plus every unique first-column `ze_` tag in `feature-gates.txt`, including `ze_ssh` and `ze_web`. Native job commands ran `go test -race -v -tags <tags>` over command, MCP and web, then separately web, then `./cmd/ze/hub/ -run '^TestEveryCommandOnEverySurface$' -count=1`. The combined command stopped at 290 seconds with exit 143 after command/MCP verdicts; its web result is not claimed. The separate web and parity jobs completed with exit 0. These are regression results, not performance measurements. No passing test was repeated for closure.

Inspection inputs: `tmp/session/2026-11-02-150621/scratch/job-api-lint-fix-units-82abd790.log`, `job-api-lint-fix-web-b0ce61f6.log`, and `job-api-lint-fix-parity-f43cb6a6.log` in that directory; lint output `sh_19d0ae8710013wiQMKJGcfhca3.out` under the OpenCode shell session. The commands, canonical test producers and observed verdicts above are retained here rather than depending on those temporary logs.

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Scoped lint runner and surface producers | N-A: lint tooling and existing Go parity test | Both Linux lint variants zero; `TestEveryCommandOnEverySurface` compares produced command surfaces with eligibility and passed |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | Existing web annotations and refreshed node-copy diagnostic; recorded in deviations and mistake log |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| API result and eligibility contracts | `mcp/tools.go` result builders and `command/surface.go` `AnswerSurface`; source anchors in `docs/architecture/api/commands.md` and MCP overview | Unchanged |
| Overlay and completion | `command/node.go` `copyNodeForOverlay`; anchors in `docs/architecture/cli/command-completion.md` | Separate map ownership retained |
| Other checklist categories | Three-file diff changes only literal reuse and equivalent map copy | No documented behavior or inventory change; no documentation edit required |

Owner limited closure to metadata; no whole-tree checks, builds, lint reruns or tests were started. No RFC behavior or tagged test changed. Native review metadata uses the existing built launcher and the authorized parent claim with `ZE_AGENT_ID` unset.
