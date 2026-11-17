# Spec: container-7-followup

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P3 |
| Release reason | A post-release feature. It is a child of the container work, which the owner placed after the first release (O-3 of `spec-container-0-umbrella.md`). No shipped behavior is wrong without it. |
| RFC gaps | - |
| Depends | spec-container-2-image-store, spec-container-3-network |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | strongest tier |
| Updated | 2026-11-17 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The container work (`spec-container-0-umbrella.md`) passed its closing test on 2026-11-17: the published Redis image runs as a Ze container and serves a client beyond the router (`test/container/container-redis.ci`). Everything that remains is listed in the umbrella's "Remaining work" table, one row per item with the child spec that owns it.

A few items found while building it are owned by no child spec. This spec holds exactly those, so none of them lives only in a handover or a conversation. An item that an existing child already specifies is never restated here: it stays in that child.

The goal: each item below is either built and proven, or moved by an owner ruling into the child spec it belongs to.

| # | Item | What is true today | Why no child owns it |
|---|------|--------------------|----------------------|
| F-1 | CI runs the container suite with the seeded image | No workflow under `.github/workflows/` names the container suite or `./le test container-seed fetch`. `container-redis.ci` fails, and names the seed command, on a machine with no seed. Not determined: whether a generic suite step of a workflow reaches `test/container/` | The image store child says "CI runs it before the tests" and names a workflow caller for its own QEMU action, but no step, test or file of any child adds the seed to a workflow |
| F-2 | A plugin command declares its time bound | `create container image reference` answers after 20 seconds that the fetch still runs, because a plugin command has no way to declare a longer bound. `TestFetchCommandDeclaresItsBound` is red by design until the declaration exists | The image store child records the need as assumption A-7 and in its Deviations (a wire key `timeout`, whole seconds, proposed). No child has an implementation step, a file or an acceptance criterion for the plugin protocol field. The field lives in `pkg/plugin/rpc` and `internal/component/plugin/server`, where the owner has work in progress |
| F-3 | A sibling reads the applied firewall ruleset | `doctor-container-port-blocked` judges the committed firewall config, so a rule written with another tool is not seen. A sibling's doctor answer makes no engine call, and the engine client has no ruleset read | The network child specifies the check against "the applied ruleset listing" and assumes the read exists. The read itself, an engine request a sibling may call, is specified nowhere |
| F-4 | A layer entry written through a link into an opaque folder | Opaque folder markers are handled and tested (`TestApplyOpaqueDirectoryKeepsSameLayerEntries`). The record of what a layer wrote is keyed by the folder's path, so a child the same layer wrote through a link to that folder may be removed with the lower layer's entries. Not verified beyond the keying | Recorded only in the handover's track table. No spec row, journal row or page holds it |
| F-5 | Every doctor fix line of the container work parses against the schema | `FixLargestImage` holds `set container store largest-image SIZE`, and the schema has no `store` container. The leaf is owned by the operator surface child (table S-3). Nothing checks that a fix line names words that exist, so the next such line is found by an operator | The leaf has an owner. The guard, one test over every fix line the plugin and its siblings declare, has none |
| F-6 | The seed holds more than one pinned image | The seed action reads one pin, `test/interop-container/redis.pin`, and the registry fixture serves that one image. A guest test of a second image, for example one that starts as root and drops its user, needs the public registry | The image store child specifies the seed for the closing test's image only |
| F-7 | Tooling defects found during the work | The handover (`plan/handover/42-container-support-implementation.md`, "Owed housekeeping") names two defects of the `le` tooling to be filed with an owning spec | They are not container behavior, and no spec has them |

Not in this spec, and why:

| Item | Where it is |
|------|-------------|
| Spec text the code now contradicts (the closing test's suite, the restart count, the applied ruleset, the fetch answer) | Recorded as deviations in the child that holds the text, and listed in the umbrella's "Remaining work" table |
| Every config word, command, doctor check, metric and page still to build | The child spec the umbrella's table names for it |
| The storage cap | After the release, by the owner's ruling of 2026-11-17, recorded in the umbrella |

Everything else is open and is decided at design: the order of the items, which of them an owner ruling moves into a child, and for F-2 and F-3 the shape of the plugin protocol change, which waits for the owner's work in that area to land.

## Required Reading

### Architecture Docs
- [ ] `plan/post-release/features/spec-container-0-umbrella.md` - the approved container design, the owner's rulings and the "Remaining work" table
  → Constraint: an item a child spec already owns is never restated in this spec
- [ ] `plan/post-release/features/spec-container-2-image-store.md` - assumption A-7 and the Deviations on the command's time bound; the seed table under "The owner's closing test"
  → Constraint: the fetch command's bound is a declaration of the command, never a constant in the engine
- [ ] `plan/post-release/features/spec-container-3-network.md` - "Published ports" and the judgment of `doctor-container-port-blocked`
  → Constraint: the check judges two matches only (output interface the bridge, destination the instance's address), whatever it reads them from
- [ ] `plan/handover/42-container-support-implementation.md` - the track states and "Owed housekeeping"
  → Decision: the handover is the source of F-4 and F-7 until this spec's research replaces it

**Key insights:** (minimal context to resume after compaction)
- To be filled at research.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/container/network/policy.go` - the port-blocked check and what it reads
- [ ] `internal/plugins/container/imagestore/apply_linux.go`, `apply_rules.go` - the opaque folder and the record of what a layer wrote
- [ ] `internal/plugins/container/doctor_store.go` - the fix lines
- [ ] `internal/le/test/containerseed/seed.go`, `internal/test/fixture/register_container_registry.go` - the seed and the fixture
- [ ] `internal/plugins/container/image_cmd.go`, `imagestore/fetch.go` - the fetch command's answer wait

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
| A-1 | No workflow reaches `test/container/` today | A search of `.github/workflows/` for the suite's name and the seed found neither. A generic "all suites" step was not ruled out | F-1 is already half done, or the suite is already red on CI | Reading each workflow's suite list | unvalidated |
| A-2 | The opaque folder gap of F-4 is real | The handover's track table. The code was read only as far as the record's key | F-4 is dropped | A unit test that writes a child through a link into an opaque folder | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | F-2 or F-3 changes the plugin protocol while the owner's work in the same files is uncommitted | A cherry-pick of this work conflicts in `pkg/plugin/rpc` or `internal/component/plugin/server` | Neither item starts until the owner says his work there has landed |
| R-2 | CI fetches the seed from the public registry on every run and is rate-limited | A workflow run fails at the seed step with a 429 answer | The seed is cached by the pin's digest, so a run with an unchanged pin fetches nothing |

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
| [test] | [the `container` suite, `test/container/`] | [what the user expects to happen] | |

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
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: `./le test unit plugins` for the package tests, and `./le test container` for the functional suite. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit
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
