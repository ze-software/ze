# Spec: container-6-health-check

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P3 |
| Release reason | A post-release feature. It is a later child of the container work, which the owner placed after the first release (O-3 of `spec-container-0-umbrella.md`). No shipped behavior is wrong without it. |
| RFC gaps | - |
| Depends | spec-container-5-shell |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | strongest tier |
| Updated | 2026-11-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A container that Ze runs can stop doing its work while its entrypoint keeps running. Today Ze knows only whether the entrypoint runs: the container work (`spec-container-0-umbrella.md`) has no way to ask a container whether it is healthy.

A container health check is part of the operator model the owner asked Ze to follow: an operator names a command, and the system runs it inside the container on an interval and judges the container by its result. The owner ruled on 2026-11-16 that the health check is a later child of the container work, with a spec of its own. This is that spec.

Two things are already fixed by the sibling specs, and this spec takes both:

| Fixed by | What it fixes |
|----------|---------------|
| `spec-container-4-operator-surface.md`, table S-4 "Reference nodes left out" | The Ze form of the config: a presence container with a command, an integer interval, a timeout and a retry count. Its absence means no health check, so no slot holds a keyword and a number |
| `spec-container-5-shell.md`, "Bounded answer (one-shot surfaces)" | How the command runs: through the bounded run that child provides, which runs one program in a running container with a time bound and a size bound, and answers its exit status |

The goal: an operator configures a health check for a container, Ze runs its command through the bounded run on the configured interval, and the container's state, the listings and doctor report an unhealthy container with the command that shows why.

Everything else is open and is decided at design: the node's name and its leaves in the operator surface child's tables, what an unhealthy result does (report only, or restart under the restart policy), the health states an operator sees, the doctor code, and the metrics.

## Required Reading

### Architecture Docs
- [ ] `plan/post-release/features/spec-container-0-umbrella.md` - the approved container design and the owner's rulings
  → Decision: health checks are a later child of the container work ("Accepted defaults")
- [ ] `plan/post-release/features/spec-container-4-operator-surface.md` - the one table of config words and command words, and table S-4
  → Constraint: a node or a command this spec needs is a row of that child's tables first
- [ ] `plan/post-release/features/spec-container-5-shell.md` - the bounded run
  → Constraint: the health check command runs through the bounded run, never through a second way into a container

**Key insights:** (minimal context to resume after compaction)
- To be filled at research.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `plan/post-release/features/spec-container-4-operator-surface.md` - table S-4 records the Ze form the health check takes, and its Known Limitations name this spec
- [ ] `plan/post-release/features/spec-container-5-shell.md` - the bounded run: one program in a running container, with a time bound, a size bound and an exit status
- [ ] `plan/post-release/features/spec-container-0-umbrella.md` - the ruling that health checks are a later child. The container plugin does not exist yet: the runtime child creates it, and its source files are listed here at research

**Behavior to preserve:** (unless the user explicitly said to change it)
- To be filled at research.

**Behavior to change:** (only what the user asked for)
- To be filled at research.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| [mechanism] | [file + symbol, or page + section] | [what it does today] | [extends / replaces / does not apply: why] |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters: to be filled at design]

### Transformation Path
1. [To be filled at design]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| [boundary] | [how] | No |

### Changed-Wire Census

**Search evidence:** [scope, terms, positive control and source findings]

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [search finding] | [change or preserve, with reason and proof] |

### Integration Points
- [To be filled at design]

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | [what this design assumes] | [where the assumption comes from] | [impact on design] | [test/grep/user confirmation] | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | [what goes wrong] | [how we notice it] | [what we do about it] |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | [to be filled at design] |
| How is it reverted? | [to be filled at design] |
| Who else touches this path? | [to be filled at design] |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| [config/CLI/event that triggers it] | → | [function that actually runs] | [test name proving the chain] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | [what triggers the behavior] | [positive observable outcome] | [what remains permitted and why, or why none] |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | [user operation] | [path] | [test name] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [test] | [file] | [description] | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| [field] | [min-max] | [value] | [value or N/A] | [value or N/A] |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [test] | [the `container` suite the runtime child registers] | [what the user expects to happen] | |

## Files to Modify
- To be listed at design.

## Files to Create
- To be listed at design.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | |
| YANG validation constraints | | |
| YANG custom validators | | |
| CLI commands/flags | | |
| CLI grammar (keyword before value) | | |
| Editor autocomplete | | |
| Functional test for new RPC/API | | |
| Pipe completeness | | |
| Env var registration | | |
| Doctor check for runtime dependencies | | |
| Doctor code per problem class | | |
| Prometheus counters/metrics | | |
| BGP family surface (new SAFI / capability / attribute) | | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | | |
| 2 | Config syntax changed? | | |
| 3 | CLI command added/changed? | | |
| 4 | API/RPC added/changed? | | |
| 5 | Plugin added/changed? | | |
| 6 | Has a user guide page? | | |
| 7 | Wire format changed? | | |
| 8 | Plugin SDK/protocol changed? | | |
| 9 | RFC behavior implemented, changed, or newly proven? | | |
| 10 | Test infrastructure changed? | | |
| 11 | Affects daemon comparison? | | |
| 12 | Internal architecture changed? | | |
| 13 | Route metadata keys added/changed? | | |
| 14 | Prometheus counters added/changed? | | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | |
| 16 | Any changed source file referenced by existing doc source anchors? | | |
| 17 | Existing docs show config/CLI/API examples for this area? | | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- to be filled at design.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| [concrete thing that must exist] | [grep/ls/test command] |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | [what inputs need validation and how] |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- To be filled at design.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: `./le test unit plugins` for the package tests, and `./le test container` for the functional suite the runtime child registers. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** remove the spec's path in its bucket, in the same commit script
