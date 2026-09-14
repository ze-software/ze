# Spec: a literal restates a registry

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A Go literal that enumerates what a live registry already holds is a second
declaration of one fact. It drifts, and the reader cannot tell which side is
wrong. `ai/rules/principles.md` bans it in two always-on directives, and nothing
in the tree measures it.

A sweep over the whole tree on 2026-09-11 found 53 such literals across the BGP
engine, the components, config, the plugins, `internal/core/` and `cmd/`. At
least eight already disagree with the registry they copy: the RIB spells the
FlowSpec SAFI `flowspec` where the family registry spells it `flow`; `readOnlyVerbs`
omits `resolve`, which `command.Verbs` classifies as a read verb, so agents are
told `resolve` is daemon-mode; `attrCodeNames` seeds plugin-owned attribute codes
so `Recognized()` answers true with the owning plugin compiled out.

The goal is a gate that refuses the next one. Its key corpus is DERIVED in-process
from the real registries, so the gate carries no list of its own to go stale.

## Required Reading

### Architecture Docs
- [ ] `ai/patterns/registration.md` - every registry that exists and what each holds
  → Constraint: the corpus is read from the registry at runtime, never transcribed. A gate holding its own copy of the key set is the defect it judges
  → Decision: the registries that can answer are the plugin registry, the family registry, `command.Verbs`, the YANG model, and the diagnostic code registry
- [ ] `docs/architecture/core-design.md` - the `le` to product import direction
  → Constraint: `le` MAY link `internal/component/plugin/all`; `ze` never links `internal/le`, and `TestNormalZeLinksNoInternalLe` (`cmd/ze/ze_le_personality_test.go`) measures it
- [ ] `docs/contributing/ze-go-style.md` - repository tooling shape
  → Constraint: a development workflow is an `./le <area> <action>` backed by a callable Go package, with fixtures under that package's `testdata/`
- [ ] `ai/rules/evidence.md` - what a gate owes
  → Constraint: a guard fails closed or says something; a walk that read nothing MUST fail rather than pass

**Key insights:**
- `internal/le/cligrammar` is the shape to copy: it blank-imports the product composition root AND walks the AST in the same package.
- No shared Go AST walker exists in `internal/le/`. Thirty-four non-test files each parse for themselves, and the three that expose a per-file scanner each return their own findings type.
- Thirteen non-test `internal/le` packages already blank-import `internal/component/plugin/all`, so the cost is established and no tier rule restricts it.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/plugin/boundary/register.go`, `actions.go`, `report.go` - the worked example of a check area: `leroot.Register` plus `leroot.RegisterActions`, an `Answer(args []string) (any, int)`, findings as a named slice with a `Text()` renderer, and `scanFloor = 400` so a walk that read nothing fails
- [ ] `internal/le/leaction/leaction.go` - `Action` carries `Verb`, `Why`, `Writes`, `Parameters` and exactly one answer function; `New` panics at init on a table that cannot dispatch
- [ ] `internal/le/inventory/inventory.go` - reaches `registry.All()`, `registry.FamilyMap()`, `registry.CapabilityMap()` and `yang.Modules()` in-process behind a blank import of `internal/component/plugin/all`
- [ ] `internal/component/command/verbs.go` - `Verbs` is the declared canonical CLI vocabulary, and its doc comment states "there is no second hardcoded list"
- [ ] `internal/core/diagnostic/codes.go` - `RegisterBuiltinCodes` has exactly one non-test caller, `cmd/ze/ze_core_dispatch.go`, so a tool that only blank-imports the composition root sees an empty builtin code set
- [ ] `internal/le/plugin/imports/pluginimports.go` - the generated composition root, discovering four registrar kinds under `pluginDirs`; `pluginDirs` is written out deliberately because it states where a plugin is ALLOWED to live
- [ ] `internal/le/population/population.go` - `Exemptions` accounting: an empty reason is no excuse, and a rule that matched nothing comes back unexcused
- [ ] `internal/le/doc/check/citation.go` - `markerReason` requires a non-empty parenthesised reason, and `suppressed` suppresses only when the reason is non-empty
- [ ] `internal/le/verify/engine/stages.go` - `fullStages` is the execution-ordered stage list a new gate joins

**Behavior to preserve:**
- `./le plugin imports check` keeps refusing a composition root that omits any of its four current registrar kinds, and `pluginDirs` stays a written-out policy.
- Every `ze <root>` command reachable today stays reachable.

**Behavior to change:**
- A new area answers `./le enumeration check` and `./le enumeration report`.
- `./le plugin imports` discovers a fifth registrar kind, so a package registering a CLI root handler is named in the generated composition root rather than in a hand list under `cmd/ze/`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le enumeration check` from a shell, and the same action as a stage inside `./le verify worktree`.
- Input at entry: the checkout path, and the live registries inside the tool's own process.

### Transformation Path
1. Registration graph loads: the blank import of `internal/component/plugin/all` runs every product `init()`.
2. Corpus build: each corpus reads its registry and yields a key set, with a per-corpus floor so an empty corpus fails rather than passing everything.
3. Tree walk: one `token.FileSet` over non-test Go files, grouped by package directory.
4. Candidate extraction: composite literals, `const` blocks and `switch` statements whose string constants are collected per syntactic unit.
5. Judgement: a unit holding two or more distinct keys of one corpus, in a package that does not own that registry, is a finding unless an exemption marker suppresses it.
6. Exemption accounting: the marker set is assessed by `internal/le/population`, so a marker that suppresses nothing is itself a finding.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `le` → product registries | blank import of `internal/component/plugin/all`, then registry calls | No |
| Corpus → tree walk | key sets as plain string sets, no product types cross into the walker | No |
| Check → verify engine | a stage row in `fullStages` | No |

### Integration Points
- `leroot.Register` / `leroot.RegisterActions` - the area registers like every other `le` area.
- `internal/le/population.Exemptions` - exemption accounting, reused rather than rewritten.
- `internal/le/verify/engine.fullStages` - the gate joins the verification run.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Two distinct corpus keys in one syntactic unit discriminates a copy from a coincidence | the 53 sweep findings each hold three or more | the gate is noisy or blind, and the threshold needs a per-corpus value | run the check over the tree and compare its findings against the sweep list | confirmed with three amendments to the threshold, all of them narrowings: a declaration is not judged, a closed enumeration is copied whole or not at all, and a registry's keys must be at least 8 percent of the literal. Measured 2026-09-14: 259 findings, of which 234 are genuine and 25 are one namespace's words coinciding with a registry's. The 25 carry markers, so `report` now answers 230 rows and every one of them is a copy this spec believes in. The sweep's 53 were a hand-made lower bound, not a ceiling |
| A-2 | The `le` binary's build tags expose the whole registry | `feature-gates.txt` line 16 gives the `le` launcher every tag | a differently tagged build sees a short corpus and the gate silently weakens | a corpus floor per corpus, asserted in a unit test | confirmed: `TestCorporaMeetTheirFloors` reads 89 plugin names, 23 family names, 13 verbs, 329 enumeration values in 123 enumerations and 154 diagnostic codes in the `le` process |
| A-3 | Every package registering a CLI root handler is discoverable by an AST search for the registrar calls | `internal/le/plugin/imports` already discovers four kinds this way | the generated composition root omits a package and `ze <root>` answers "unknown command" | a test asserting the generated list is a superset of today's hand list in `cmd/ze/ze_core_dispatch.go` | unvalidated |
| A-4 | The YANG model exposes enumeration values through the loader | `internal/component/config/yang_schema.go` reads `entry.Type.Enum.Names()` | the enum corpus cannot be built and that corpus is dropped from the first cut | a unit test building the enum corpus and asserting a known value is present | confirmed: `yangEnumerations` walks the `yang.DefaultLoader()` entry tree and `TestYANGEnumCopyIsReported` reports a copy of the console speed enumeration |
| A-5 | A literal stating POLICY is distinguishable from a literal stating what EXISTS only by its author's intent | `pluginimports.go` states the distinction in its own comment | the gate fires on legitimate policy lists and the exemption list grows without bound | count the exemptions the first run needs; more than the 53 known findings breaks this | BROKEN as stated, and the landing posture changed rather than the rule. Three narrowings of the JUDGEMENT removed what a marker was going to be spent on, so the 24 markers the tree now carries each name another namespace rather than excusing a copy. The 234 genuine findings are NOT marked: `check` judges the change set and `report` keeps them in the open (owner decision, 2026-09-14). The measurement behind the last narrowing is in Design Insights: no ratio separates `validatedSections`, a copy at five strings of fifteen, from `rtProtoNames`, kernel RTPROTO names at five of fifteen |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Generating the CLI-root blank imports drops a package, so a root command vanishes | `ze <root>` answers "unknown command"; the superset test goes red | the superset test in A-3 runs before the hand list is deleted, and the two lists are compared in the same change |
| R-2 | The diagnostic corpus is empty because `RegisterBuiltinCodes` was never called | the corpus floor for codes fails | the check calls `RegisterBuiltinCodes` explicitly, and the floor asserts a non-trivial count |
| R-3 | The exemption markers rot as the fix pass lands, leaving suppressions that suppress nothing | `population.Exemptions` reports an unexcused rule | already the mitigation: a dead exemption turns the gate RED rather than being ignored |
| R-4 | The walk silently reads nothing after a repository move | the gate passes with zero findings | a scan floor, copied from `pluginboundary.scanFloor` |
| R-5 | The gate blocks every session's verify run on day one because the 53 exemptions are incomplete | the first full `./le verify worktree` after landing goes red on unrelated work | the check is written and run in report mode first, its findings reconciled against the sweep, and only then is the stage row added |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Two different things. A wrong gate blocks every session's verification, which costs developer time only. A wrong composition-root generation removes a `ze` root command from the shipped binary, which an operator meets |
| How is it reverted? | The gate is one stage row and one package: revert the commit. The generated composition root is regenerated by `./le plugin imports write` from whatever the discovery finds, so a revert of the discovery change restores the hand list in the same commit |
| Who else touches this path? | Any session running `./le verify`; the `plugin imports` generator is shared with every plugin author |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le enumeration check` typed in a shell | → | `enumeration.Answer` via the registered action table | `TestEnumerationActionIsRegistered` |
| a stage inside `./le verify worktree` | → | `fullStages` names `enumeration/check` | `TestEnumerationStageIsInFullStages` |
| `./le plugin imports check` over a tree whose CLI-root owner is unimported | → | the fifth registrar kind in `discoverRootHandlers` | `TestUnimportedRootHandlerIsReported` |
| `ze <root>` for every root reachable today | → | the generated composition root | `TestGeneratedImportsCoverTodaysHandList` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A Go file outside the owning package holds a map, slice, const block or switch naming two or more registered plugin names | `./le enumeration check` reports it with file, symbol and the corpus it restates, and exits 1 |
| AC-2 | The same, for two or more registered family names | reported, exit 1 |
| AC-3 | The same, for two or more keys of `command.Verbs` | reported, exit 1 |
| AC-4 | The same, for two or more values of one YANG `enumeration` | reported, exit 1 |
| AC-5 | The same, for two or more registered diagnostic codes | reported, exit 1 |
| AC-6 | A corpus yields fewer keys than its declared floor | the check exits 2 with an error naming the corpus, and reports no findings |
| AC-7 | A walk that reads fewer Go files than the scan floor | the check exits 2 naming the floor, rather than passing |
| AC-8 | A flagged literal carries the exemption marker with a non-empty parenthesised reason | the finding is suppressed |
| AC-9 | A flagged literal carries the marker with an empty or absent reason | the finding is NOT suppressed, and the check says the marker states no reason |
| AC-10 | An exemption marker that matches nothing in the tree | reported as an unexcused rule, exit 1 |
| AC-11 | A package whose `init()` registers a CLI root handler and which no blank import reaches | `./le plugin imports check` reports it, exit 1 |
| AC-12 | `./le plugin imports write` over the current tree | the generated composition root names every package the hand list in `cmd/ze/ze_core_dispatch.go` names, and that hand list is deleted |
| AC-13 | A doctor check function defined in `internal/component/doctor` and reached only by a hand-written call | `./le enumeration check` reports it as a hand-called check, exit 1 |
| AC-14 | `./le enumeration check` over a change set that introduces no new violation | exit 0, whatever the tree already holds (owner decision, 2026-09-14: the check BLOCKS on what the change set introduces, and the backlog stays visible in `report` rather than behind markers) |
| AC-15 | A finding in a package the change set did not select | `check` does not report it and does not block; `report` still names it |
| AC-16 | `./le enumeration report` over the whole tree | every finding in the tree, exit 0, because a measurement is not a gate |
| AC-17 | A change set that selected no package at all | the check exits 2 naming the change set, rather than passing every literal in the tree |
| AC-18 | A widened change set, which is what the selector answers when it cannot tell what moved | the working tree against HEAD answers instead, so the rows are the files this session changed rather than every package; the answer NAMES the route it took and the selector's reason for widening |
| AC-19 | A widened change set AND a working tree that cannot be read | exit 2. Two silences are not an answer, and a gate that cannot tell what moved MUST NOT report that nothing did |
| AC-20 | Any answer, whatever the route | it carries the scope that produced it, so an empty row set from a change set is never read as a clean tree |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPluginNameCopyIsReported` | `internal/le/enumeration/enumeration_test.go` | AC-1 over a `testdata/` fixture | green; red under `keyThreshold = 3` |
| `TestFamilyNameCopyIsReported` | same | AC-2 | green; red under `keyThreshold = 3` |
| `TestVerbCopyIsReported` | same | AC-3 | green; red under `keyThreshold = 3` |
| `TestYANGEnumCopyIsReported` | same | AC-4 | green; red with `collectEnums` not called |
| `TestDiagnosticCodeCopyIsReported` | same | AC-5 | green; red under `keyThreshold = 3` |
| `TestCorpusFloorFailsClosed` | same | AC-6: a corpus stubbed short exits 2 | green |
| `TestScanFloorFailsClosed` | same | AC-7 | green |
| `TestMarkerWithReasonSuppresses` | same | AC-8 | green; red under `keyThreshold = 3` |
| `TestMarkerWithoutReasonDoesNotSuppress` | same | AC-9 | green; red under `keyThreshold = 3` |
| `TestDeadExemptionIsReported` | same | AC-10 | green |
| `TestSingleKeyIsNotAFinding` | same | the two-key threshold, so a coincidence is not a finding | green |
| `TestOwningPackageIsNotAFinding` | same | the registry's own package may enumerate its own keys | green |
| `TestUnimportedRootHandlerIsReported` | `internal/le/plugin/imports/pluginimports_test.go` | AC-11 | |
| `TestGeneratedImportsCoverTodaysHandList` | same | AC-12, run BEFORE the hand list is deleted | |
| `TestHandCalledDoctorCheckIsReported` | `internal/le/enumeration/enumeration_test.go` | AC-13 | green; `TestDoctorRunnerRefusesAnUnreadablePackage` and `TestHandCalledDoctorChecksOverTheCheckout` hold the two anchors closed |
| `TestEnumerationActionIsRegistered` | same | the wiring row | green |
| `TestEnumerationStageIsInFullStages` | `internal/le/verify/engine/verifyengine_test.go` | the wiring row | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| key threshold per unit | 2-N | 2 | 1 (not a finding) | N/A |
| corpus floor, plugins | 1-N | declared floor | floor minus one exits 2 | N/A |
| scan floor, files read | 1-N | declared floor | floor minus one exits 2 | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | - | The user is a developer running `./le`; the action's own tests over `testdata/` are the end-to-end path, and `ai/rules/testing.md` asks for a `.ci` where an operator reaches the behavior. No operator reaches a `le` gate | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | Scope is tooling; no wire-visible behavior changes | |

## Files to Modify
- `internal/le/plugin/imports/pluginimports.go` - the fifth registrar kind
- `cmd/ze/ze_core_dispatch.go` - delete the hand-written blank import list
- `internal/component/plugin/all/all.go` - regenerated, not hand-edited
- `internal/le/verify/engine/stages.go` - the stage row
- `ai/INDEX.md` - the dev-tools row for the new action
- `ai/patterns/registration.md` - two stale sentences found during the sweep: the "Existing gap" paragraph naming a map in `cmd/ze/ze_test_register.go` that no longer exists, and the claim that the blank imports are hand-listed in `cmd/ze/main.go`

**Design documents declared by the files above** (`// Design:` headers, so each is named here whether or not it changes):
- `docs/architecture/command-ownership.md` - declared by `cmd/ze/ze_core_dispatch.go`. CHANGES: it describes how a root command's owner package is linked, and the hand list becomes a generated one
- `docs/architecture/system-architecture.md` - declared by `cmd/ze/ze_core_dispatch.go`. Verified during implementation: unaffected unless it names the hand list, and the composition of the binary is unchanged
- `docs/architecture/testing/verify-freshness-scope.md` - declared by `internal/le/verify/engine/stages.go`. CHANGES only if it enumerates the stages; the new stage adds no new freshness class

## Files to Create
- `internal/le/enumeration/enumeration.go` - the walker and the judgement
- `internal/le/enumeration/corpus.go` - the five corpora and their floors
- `internal/le/enumeration/actions.go` - the action table
- `internal/le/enumeration/register.go` - area registration
- `internal/le/enumeration/report.go` - findings and their renderer
- `internal/le/enumeration/enumeration_test.go` - the unit tests above
- `internal/le/enumeration/testdata/` - one fixture per corpus

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | a `le` action carries no operator config |
| YANG validation constraints | N-A | as above |
| YANG custom validators | N-A | as above |
| CLI commands/flags | Yes | `internal/le/enumeration/register.go` and `actions.go`, registered through `leroot` |
| CLI grammar (keyword before value) | Yes | the action's parameters follow `leaction.Parameter`, keyword before value |
| Editor autocomplete | N-A | `le` actions complete through their own registered action table |
| Functional test for new RPC/API | N-A | no RPC; the gate's own tests over `testdata/` are the path |
| Pipe completeness | Yes | the answer is structured data rendered by `command.RenderLocalAnswer`, as every `le` area is |
| Env var registration | N-A | no new environment leaf |
| Doctor check for runtime dependencies | N-A | the gate reads the checkout and its own process; no new file path, socket, port, module or binary |
| Prometheus counters/metrics | N-A | a development gate emits no runtime metric |
| BGP family surface (new SAFI / capability / attribute) | N-A | no protocol surface changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | no operator-facing behavior changes |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | `ze` commands are unchanged; `le` actions are documented in `ai/INDEX.md`, named under Files to Modify |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if the verify stage list is described there; verified during implementation, not from memory |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `ai/patterns/registration.md`, whose two stale sentences are named under Files to Modify |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | the generated composition root gains packages; `docs/plugin-overview.md` checked during implementation |
| 16 | Any changed source file referenced by existing doc source anchors? | DERIVED | `./le spec citation anchors spec plan/spec-a-literal-restates-a-registry.md`, run during implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/INDEX.md` dev-tools table gains a row |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the area exists, registers, and answers
   - Tests: `TestEnumerationActionIsRegistered`
   - Files: `register.go`, `actions.go`, a stub `Answer`
   - Verify: `./le enumeration check` dispatches and the wiring test fails because the walker is a stub
2. **Phase: corpora and their floors** -- the five key sets, each with a floor
   - Tests: `TestCorpusFloorFailsClosed`
   - Files: `corpus.go`
   - Verify: each corpus yields keys from the live registries; a stubbed-short corpus exits 2
3. **Phase: the walker** -- files, packages, syntactic units, string constants
   - Tests: `TestScanFloorFailsClosed`, `TestSingleKeyIsNotAFinding`, `TestOwningPackageIsNotAFinding`
   - Files: `enumeration.go`
   - Verify: a fixture tree yields the units it should and the floor fails closed
4. **Phase: judgement and report** -- the two-key rule, the finding, the renderer
   - Tests: the five per-corpus tests, AC-1 to AC-5
   - Files: `enumeration.go`, `report.go`
   - Verify: each fixture is reported with its corpus named
5. **Phase: exemptions** -- the marker, its required reason, and the accounting
   - Tests: `TestMarkerWithReasonSuppresses`, `TestMarkerWithoutReasonDoesNotSuppress`, `TestDeadExemptionIsReported`
   - Files: `enumeration.go`
   - Verify: a reasonless marker suppresses nothing, a dead marker is a finding
6. **Phase: the structural checks** -- hand-called doctor checks, and the fifth registrar kind
   - Tests: `TestHandCalledDoctorCheckIsReported`, `TestUnimportedRootHandlerIsReported`, `TestGeneratedImportsCoverTodaysHandList`
   - Files: `enumeration.go`, `internal/le/plugin/imports/pluginimports.go`
   - Verify: the superset test passes BEFORE the hand list is deleted
7. **Phase: reconcile and land** -- run over the tree, exempt the known findings, join the stage list
   - Tests: `TestEnumerationStageIsInFullStages`
   - Files: the exemption markers at each known violation, `cmd/ze/ze_core_dispatch.go`, `internal/le/verify/engine/stages.go`
   - Verify: the run's findings are reconciled against the 2026-09-11 sweep, every exemption names a reason, and the gate exits 0 on the tree as it stands

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:symbol |
| Correctness | The gate's own package holds no transcribed key set: every corpus is a registry call |
| Correctness | Each exemption marker in the tree names a reason a reader can check, not "known violation" |
| Data flow | The corpus crosses into the walker as plain string sets, so no product type reaches the AST code |
| Naming | Findings name the corpus, not the mechanism: a reader learns which registry the literal restates |
| Rule: `ai/rules/evidence.md` | Both floors fail closed, and a test proves each one |
| Rule: `ai/rules/principles.md` | The gate does not become the thing it judges |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The action is registered | `./le enumeration` prints its verbs |
| The gate is in the verification run | `./le verify` stage list names `enumeration/check` |
| The hand list under `cmd/ze/` is gone | `cmd/ze/ze_core_dispatch.go` holds the composition root import plus the documented exceptions only, each carrying its reason on the line. Measured 2026-09-14: 24 module-internal imports became 4, and a count of zero is unreachable because `_ "internal/component/plugin/all"` is the only link to the generated root in a `ze_core` build |
| Every known violation carries a reason | the check's own dead-exemption accounting, run clean |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The walker parses Go files from the checkout only; a parse error names the file and fails the run rather than being skipped |
| Fail-open | Both floors, and the exemption accounting, each turn silence into a red rather than a pass |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| The gate fires on a legitimate policy list | Do not widen the exemption list first: state the policy-versus-observation distinction in the finding, and check A-5 |
| More exemptions needed than the sweep found | STOP. A-5 is broken and the threshold needs redesign |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- `pluginimports.go` states the distinction this whole gate turns on: a list that states what is ALLOWED is policy and is legitimately written out, while a list that states what EXISTS is a copy. A list derived from the tree would agree with whatever the tree contains, so it would stop being a gate.
- `fullStages` in `internal/le/verify/engine/stages.go` is itself a written-out list, and it is policy by that same test: it declares execution ORDER, which no registry holds.
- A literal that FEEDS a registry is a declaration, and only a literal that READS the same key space is a copy. That distinction is worth more than any threshold: it removed the plugin registrations, the doctor checks and the per-plugin diagnostic code blocks without a single exemption.
- Whether a const block can BE a declaration depends on the registry, not on the syntax. A diagnostic code enters the registry as the const itself, so the const is where it comes from. A family name is composed by the registrar from an AFI name and a SAFI name, so every Go const holding the joined string is a second spelling and none of them is the declaration. `Corpus.WrittenWhole` carries that difference.
- **The key-share ratio separates only the tail, and this is the most useful thing the implementation measured.** A ratio of registry keys to the literal's strings cleanly removes a literal about another namespace when the share is tiny: `services_table.go` holds six plugin names among 4954 IANA service names. It cannot separate the middle. At five strings of fifteen sit `validatedSections`, which IS a copy of the plugin registry, and `rtProtoNames`, which is the kernel's RTPROTO names. Same ratio, opposite verdicts. Measured windows: 0.6 loses `IsReadOnlyPath` (three of seven) and nine of the 27 family rows; 0.1 loses three literals that are copies; 0.08 loses none and still drops fourteen rows. The threshold is therefore 8 percent, and the residue above it is not reachable by any ratio.
- A gate over a whole tree and a gate over a change set are different products. This one had to become the second, because 234 genuine findings cannot be marked before one is fixed, and a first state that is mostly suppression teaches every later reader that the markers are noise.
- **A change-set gate needs a second way to ask what moved, because the package selector WIDENS.** `changed.Packages` answers every package when no verify run has recorded a green commit, which is this checkout's normal state while 920 verification gates are owed. Reading that widening as "everything is new" makes a change-set gate judge the whole backlog on every run, which is whole-tree blocking under another name, and a gate that is red for every session gets disarmed: `./le yang leaf-mentions` is where that ends. Reading it as "nothing changed" is the silent zero. So the widened route asks the working tree against HEAD instead, which needs no baseline and excludes the backlog by construction. Measured on this checkout: 230 rows whole-tree, **2 rows under the fallback**, both of them pre-existing copies in files this session had edited.
- **A gate that narrows its own scope MUST say so in its answer.** `CheckReport.Scope` is published in the text and the JSON, and it names the route and the selector's reason for widening. A quiet gate and a clean tree are otherwise the same output, which is the defect this gate is named after, one level up.
- Generating a blank import can close an import cycle in the TEST binary that does not exist in the shipped one: a package whose internal test file imports `internal/component/plugin/all` cannot be named by the composition root. The repair is the external test package, and it removes the constraint rather than avoiding it. Three files needed it: `internal/component/bgp/cli`, `internal/component/config/cli` and `internal/component/doctor`. `internal/component/config/cli/all_import_test.go` had recorded the hazard in a comment, and no gate enforced what the comment knew.
- Four owners cannot be discovered, and each is a different reason rather than one class: two reach `plugin/all` themselves and would cycle, one would panic on a duplicate root registration against `internal/test/cli`, and one is a second composition root that registers backends rather than a command. An exception table with three derivable reasons is what keeps the hand list from growing back.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The corpus is read from the live registries in the tool's own process | parse the registrations out of source; keep a generated key file | Both alternatives are second declarations of the key set, which is the defect the gate exists to find |
| Two distinct keys in one syntactic unit is the threshold | one key; a per-corpus threshold | One key fires on every mention of a plugin name. All 53 known findings hold three or more, so two has margin and stays explainable |
| The gate lands blocking, with the known violations exempted | report-only until the tree is clean | A gate that cannot fail is a report. `./le yang leaf-mentions` always exits 0, sits in no verify stage, and has 80 findings nobody acts on |
| Exemption accounting via `internal/le/population` | a plain allowlist | A suppression that suppresses nothing must go red, or the list rots as the fix pass lands |
| The fifth registrar kind extends `./le plugin imports` | a new check in the enumeration area | The generator that must EMIT the imports already lives there; a second discoverer would be the duplication this spec judges |

## Known Limitations
- A literal that restates a registry using only ONE of its keys is invisible to this gate. The threshold buys explainability at that cost, and A-5 records it.
- A key set no registry holds cannot be judged, because there is nothing to compare against. The tree holds several, and each one is a literal this gate either misreads as another registry's keys or cannot see at all. Each needs a registry to exist first, and each is its own spec:
  - **Functional test suite names** (`internal/le/functional/suites.go`, `internal/le/qemu/alltests.go`). Already a journal row: `plan/journal/plugin-list-hardcoded.md`, 2026-09-09, where a hand-written suite-name slice meant a new `.ci` test ran nowhere.
  - **Kernel RTPROTO names** (`internal/plugins/iface/netlink/route_linux.go`) and the VPP FIB source names aligned with them (`internal/plugins/iface/vpp/fib.go`).
  - **IANA protocol names and numbers** (`internal/component/firewall/protocol.go`), which the YANG firewall enumeration also declares.
  - **ExaBGP API control words** (`internal/exabgp/bridge`).
  - **iproute2 subcommands and verbs** (`internal/le/deployment/netns.go`, `internal/le/qemu/guest_linux.go`).
  - **BGP-LS protocol-ID names** (`internal/component/bgp/plugins/nlri/ls/types.go`), which RFC 7752 declares and no Ze registry holds.
- An exemption marker suppresses a syntactic unit, not a corpus. `internal/component/firewall/protocol.go` is one literal carrying two findings: the plugin-names row is a coincidence and the YANG row is real, so it carries NO marker and its false row stands. A per-corpus marker would fix it and is not built.
- The copies already in the tree are not fixed here. The three narrowings of the
  judgement removed what most of the 53 sweep findings would have spent a marker on,
  so the tree carries 24 markers naming another namespace rather than excusing a copy,
  and the 230 genuine rows stay in the open in `report` (owner decision, 2026-09-14).
  The fix pass is its own spec: `plan/immediate/spec-the-fix-pass-for-restated-registries.md`,
  written at closure once the gate had reported its reconciled list. It sits in
  `immediate/` because two of its rows already answer an operator wrongly.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- `internal/le/enumeration`: the walker, the five corpora and their floors, the two
  actions, the report, and the hand-called doctor-check finding. The corpus is read
  from the live registries behind a blank import of the product composition root, so
  the gate carries no key set of its own.
- `internal/le/plugin/imports`: a fifth registrar kind, the CLI command owner, plus
  `compositionRootImporters` which computes the cycling set from the import graph
  rather than from a marker.
- `internal/le/changed.WorkingTreePaths`: the working-tree query the gate falls back
  to when the package selector widens, sharing the selector's own git queries.
- `cmd/ze/ze_core_dispatch.go`: 24 module-internal blank imports became 6, each
  survivor carrying the reason the generated root cannot name it.
- 24 exemption markers, each naming another namespace the literal belongs to.
- `internal/le/verify/engine/stages.go`: the `enumeration/check` stage row.

### Bugs Found/Fixed
- **The widened change set did not survive the verify run (BLOCKER, found at review).**
  `changed.WriteScopePackages` publishes the packages and not the `Widened` flag, so
  inside `./le verify worktree` the stage read a published `./...` as a narrow answer,
  matched every finding through `anyPrefix`'s empty-prefix arm, and exited 1 with all
  230 rows. Fixed by `widening` (`internal/le/enumeration/actions.go`), which reads the
  fact off the package answer that survives the round trip. Covered by
  `TestAPublishedWidenedScopeStillFallsBackToTheWorkingTree`, observed red against the
  unfixed producer.
- **`closedGroupKeysMin` had no discriminating test (ISSUE, found at review).** Lowering
  it to 2 turned no test red. `TestATwoValueEnumerationIsNotAFinding` now holds it,
  observed red at 2.
- **`commandRegistrars` could lose a registrar in silence (ISSUE, found at review).**
  A registrar added to `internal/component/command/registry` and not added to that list
  makes every owner using it invisible, and `ze <command>` answers "unknown command"
  with every gate green. `TestEveryCommandRegistrarIsMatched` now fails instead,
  observed red with `.MustRegisterRootHandler(` removed.
- **`ai/patterns/registration.md` miscounted the hand list (NOTE, found at review).**
  It said four packages where `ze_core_dispatch.go` hand-imports five, and its reason
  list dropped `traffic/cli`. Corrected.

### Documentation Updates
- `ai/INDEX.md`: the dev-tools row and a keyword row for the new action.
- `ai/patterns/registration.md`: the Phase 7 "hand-listed in `cmd/ze/main.go`" claim
  replaced by the generated route, and the hand-list count corrected at review.
- `ai/patterns/cli-command.md`: the owner-linking checklist row now names
  `./le repository generate`.
- `docs/architecture/command-ownership.md`: the fifth discovery kind, and the table of
  the three reasons a command owner stays hand-imported.

### Deviations from Plan
- A-5 broke as stated. The landing posture changed rather than the rule: three
  narrowings of the JUDGEMENT (a declaration is not judged, a closed enumeration is
  copied whole or not at all, and a registry's keys must be 8 percent of the literal)
  removed what the markers were going to be spent on. `check` judges the change set
  and `report` keeps the backlog open (owner decision, 2026-09-14).
- The spec named `plan/spec-the-fix-pass-for-restated-registries.md`; it was written at
  closure into `plan/immediate/`, because two of its rows answer an operator wrongly.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-5 assumed a marker per known finding would be affordable | 234 genuine findings cannot be marked before one is fixed, and a first state that is mostly suppression teaches every reader that markers are noise | running the check over the tree | the gate became a change-set gate; the backlog lives in `report` |
| approach | The change-set fallback trusted `ScopeReport.Widened` | the flag does not survive `WriteScopePackages`, which publishes packages only, so inside a verify run the gate judged the whole tree | review round 1, reproduced by publishing a `./...` scope file and running the action | `widening` reads the fact off the package answer; regression test observed red |
| escalation | A threshold shipped with no test that fails when it moves | `closedGroupKeysMin` could be lowered to 2 with every test still green | review round 1, by lowering it | a fixture test now holds it; the other three thresholds were each broken and each turned a test red |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A gate that refuses the next literal restating a registry | Done | `internal/le/enumeration/actions.go`, `runCheck` | judges the change set, exit 1 on a new copy |
| Its key corpus DERIVED in-process from the real registries | Done | `internal/le/enumeration/corpus.go`, `readCorpora` | five registry calls, no transcribed key |
| A measurement of what the tree already holds | Done | `internal/le/enumeration/actions.go`, `runReport` | 230 rows at exit 0 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestPluginNameCopyIsReported` | red at `keyThreshold = 3`, observed |
| AC-2 | Done | `TestFamilyNameCopyIsReported` | red at `keyThreshold = 3`, observed |
| AC-3 | Done | `TestVerbCopyIsReported` | red at `keyThreshold = 3`, observed |
| AC-4 | Done | `TestYANGEnumCopyIsReported` | red with `collectEnums` a no-op, observed |
| AC-5 | Done | `TestDiagnosticCodeCopyIsReported` | red at `keyThreshold = 3`, observed |
| AC-6 | Done | `TestCorpusFloorFailsClosed`, `TestCorpusFloorNamesTheCorpus` | exit 2 through `walkTree` |
| AC-7 | Done | `TestScanFloorFailsClosed`, `TestScanFloorNamesTheFloor` | exit 2, both counts named |
| AC-8 | Done | `TestMarkerWithReasonSuppresses` | red at `keyThreshold = 3`, observed |
| AC-9 | Done | `TestMarkerWithoutReasonDoesNotSuppress` | red at `keyThreshold = 3`, observed |
| AC-10 | Done | `TestDeadExemptionIsReported` | `population.Exemptions` accounting |
| AC-11 | Done | `TestUnimportedRootHandlerIsReported` | check exits 1 over a fixture tree |
| AC-12 | Done | `TestGeneratedImportsCoverTodaysHandList` | every remaining hand import is a composition root, cycles, or is `codegen:skip` |
| AC-13 | Done | `TestHandCalledDoctorCheckIsReported`, `TestDoctorRunnerRefusesAnUnreadablePackage`, `TestHandCalledDoctorChecksOverTheCheckout` | both anchors fail closed |
| AC-14 | Done | `TestAPublishedNarrowScopeJudgesOnlyThatPackage` | added at review; exit 0 through the action itself |
| AC-15 | Done | `TestCheckJudgesOnlyTheChangeSet` | the far row is dropped, the diff route is never asked |
| AC-16 | Done | `TestReportAnswersTheWholeTreeAtExitZero` | added at review |
| AC-17 | Done | `TestEmptyChangeSetFailsClosed` | `ErrNoChangeSet` |
| AC-18 | Done | `TestWidenedChangeSetFallsBackToTheWorkingTree`, `TestAPublishedWidenedScopeStillFallsBackToTheWorkingTree` | the second is the route a verify run really takes |
| AC-19 | Done | `TestBothRoutesUnavailableFailsClosed` | exit 2 on two silences |
| AC-20 | Done | `CheckReport.Scope`, asserted in all four scope tests | the answer names the scope that produced it |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| the twelve unit tests named in the plan | Done | `internal/le/enumeration/enumeration_test.go` | |
| `TestUnimportedRootHandlerIsReported`, `TestGeneratedImportsCoverTodaysHandList` | Done | `internal/le/plugin/imports/` | |
| `TestEnumerationStageIsInFullStages` | Changed | `internal/le/verify/engine/verifyengine_test.go`, `TestFullStagesMatchesNativeActionPopulation` | the existing population test already holds every stage row; a second test would be a second declaration of the same list |
| `TestATwoValueEnumerationIsNotAFinding`, `TestEveryCommandRegistrarIsMatched`, and the three scope tests | Done | added at review | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/enumeration/*.go` and `testdata/` | Done | six non-test files, two test files, fifteen fixture trees |
| `internal/le/plugin/imports/pluginimports.go` | Done | fifth kind |
| `cmd/ze/ze_core_dispatch.go` | Done | 24 module-internal blank imports became 6 |
| `internal/component/plugin/all/all.go` | Done | regenerated |
| `internal/le/verify/engine/stages.go` | Done | stage row |
| `ai/INDEX.md`, `ai/patterns/registration.md` | Done | |

### Audit Summary
- **Total items:** 20 ACs, 4 wiring rows, 17 planned tests, 6 file groups
- **Done:** all but one
- **Partial:** none
- **Skipped:** none
- **Changed:** 1 (`TestEnumerationStageIsInFullStages`, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A gate refuses the NEXT literal that restates a registry | functional, through the action | `TestAPublishedNarrowScopeJudgesOnlyThatPackage` runs `runCheck` over a published change set and gets exit 0; `./le enumeration check` over this checkout exits 1 naming the two copies in files this session touched. The discrimination is proved by breaking `keyThreshold` to 3, which turns six tests red |
| The corpus is DERIVED, so the gate carries no list to go stale | unit, against the live registries | `TestCorporaMeetTheirFloors` reads 89 plugin names, 23 family names, 13 verbs, 329 enumeration values in 123 enumerations and 154 diagnostic codes in this process. Neutering `collectEnums` drops the YANG corpus to 0 and the floor refuses the run |
| The gate does not commit its own defect class: a silent zero | unit, on every floor | `TestCorpusFloorFailsClosed` and `TestScanFloorFailsClosed` each drive `walkTree` to exit 2; `TestEmptyChangeSetFailsClosed` and `TestBothRoutesUnavailableFailsClosed` refuse rather than report a clean tree; `TestAPublishedWidenedScopeStillFallsBackToTheWorkingTree` closes the one path where the gate did judge the whole tree in silence |
| A command owner cannot be left unlinked | functional, over a fixture tree | `TestUnimportedRootHandlerIsReported` gets exit 1; `TestGeneratedImportsCoverTodaysHandList` refuses a hand import the derivation names or cannot explain; `TestEveryCommandRegistrarIsMatched` refuses a registrar the discovery does not match |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| The 230 copies already in the tree | The owner decided on 2026-09-14 that the backlog stays visible in `report` rather than behind markers, because 234 genuine findings cannot be marked before one is fixed | `plan/immediate/spec-the-fix-pass-for-restated-registries.md` |
| A per-corpus exemption marker | One literal can carry two findings, one real and one a coincidence, and a unit-wide marker suppresses both. `internal/component/firewall/protocol.go` is the case, and it carries no marker, so its false row stands | `plan/immediate/spec-the-fix-pass-for-restated-registries.md` |
| A registry for the six key sets no registry holds | Nothing to judge a literal against until the registry exists | `plan/immediate/spec-the-fix-pass-for-restated-registries.md` |

## Review Gate

**Round 1 scope:** the whole diff of this spec's work, as named in the closure brief:
`internal/le/enumeration/` including `testdata/`; `internal/le/plugin/imports/` (4
files); `internal/le/changed/selector.go`; `internal/le/register.go`;
`cmd/ze/ze_core_dispatch.go`; `internal/component/plugin/all/` (2 files);
the four test-cycle repairs; the eight `codegen:skip` edits; the 24 exemption markers;
`internal/le/verify/engine/` (2 files); and the four documentation
pages. Files outside that list belong to other sessions sharing this checkout and were
neither reviewed nor edited.

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/a-literal-restates-a-registry-2689b5a6-3b93-45df-a100-95ee8d0e6081.md`, 75 files, verdict clean |
| `./le spec session review check` | `review_gate: OK (clean, hashes match)` |
| Rounds | 1 |
| Reviewer lenses used | the gate's own defect class turned on itself (every floor and both scope routes read at their producer); threshold discrimination (each of the four constants broken and the resulting red observed); the eight always-in-scope classes; the exemption markers read against their files; the composition-root change verified at each producer |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | Inside a verify run the gate judged the WHOLE tree and exited 1 with all 230 rows. `changed.WriteScopePackages` publishes the packages and not `Widened`, so `Scope.fromFile` hands a widened answer back with the flag clear; the package route then took `./...`, whose prefix is empty, and `anyPrefix` matched every finding on it. Reproduced by publishing a scope file naming `./...` and running `./le enumeration check`: exit 1, 230 rows, scope line claiming "1 package(s)" | `internal/le/enumeration/actions.go`, `inChangeSet` and `anyPrefix` | `widening` reads the widening off the package answer, which survives the round trip; `anyPrefix`'s match-all arm removed. `TestAPublishedWidenedScopeStillFallsBackToTheWorkingTree` observed red against the unfixed producer |
| 2 | ISSUE | `closedGroupKeysMin` decided every closed-corpus verdict and no test failed when it moved | `internal/le/enumeration/enumeration.go` | `testdata/two-value-enum/` and `TestATwoValueEnumerationIsNotAFinding`, observed red at `closedGroupKeysMin = 2` |
| 3 | ISSUE | `commandRegistrars` is matched as text, so a registrar added to `internal/component/command/registry` and not added there leaves every owner using it unlinked, with `ze <command>` answering "unknown command" and every gate green | `internal/le/plugin/imports/pluginimports.go` | `TestEveryCommandRegistrarIsMatched` derives the population from the registry package and names the one non-command registrar with its reason; observed red with `.MustRegisterRootHandler(` removed |
| 4 | ISSUE | AC-14, AC-16 and AC-20 had no test through the action | `internal/le/enumeration/actions.go` | `TestAPublishedNarrowScopeJudgesOnlyThatPackage` and `TestReportAnswersTheWholeTreeAtExitZero` |
| 5 | NOTE | `ai/patterns/registration.md` said the dispatch root hand-imports four packages; it hand-imports five, and the reason list dropped `traffic/cli` | `ai/patterns/registration.md` | corrected against `cmd/ze/ze_core_dispatch.go` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/enumeration/enumeration.go` | Yes | `find internal/le/enumeration -type f` lists it with `corpus.go`, `actions.go`, `register.go`, `report.go`, `doctorcheck.go` |
| `internal/le/enumeration/enumeration_test.go`, `report_test.go` | Yes | same listing |
| `internal/le/enumeration/testdata/` | Yes | fifteen fixture trees, `two-value-enum/` added at review |
| `plan/immediate/spec-the-fix-pass-for-restated-registries.md` | Yes | written at closure for the Work Not Done rows |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-5 | each corpus's copy is reported | `go test` over `./internal/le/enumeration/...` is `ok` at 13.1s; breaking `keyThreshold` to 3 turns AC-1, AC-2, AC-3, AC-5, AC-8 and AC-9's tests red, and neutering `collectEnums` turns AC-4's red |
| AC-6, AC-7 | both floors refuse | `TestCorpusFloorFailsClosed` and `TestScanFloorFailsClosed` assert code 2 and zero findings through `walkTree` |
| AC-11, AC-12 | the fifth kind and the hand list | `go test` over `./internal/le/plugin/imports/...` is `ok` at 1.4s |
| AC-14..AC-20 | the change-set routes | the four scope tests pass; the widened one was observed red against the unfixed `widening` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le enumeration check` typed in a shell | none; a `le` gate has no operator path (`ai/rules/testing.md`) | Yes: the command was run over this checkout and answered exit 1 with two rows and its scope line |
| `./le enumeration report` | as above | Yes: `TestReportAnswersTheWholeTreeAtExitZero` drives `runReport` and gets 230 rows at exit 0 |
| a stage inside `./le verify worktree` | as above | Yes: `TestFullStagesMatchesNativeActionPopulation` holds `enumeration/check` in `fullStages` |
| `ze <root>` for every root reachable today | as above | Yes: `TestGeneratedImportsCoverTodaysHandList` over the real checkout |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed, with three narrowings | 230 rows after them, each one a copy the spec believes in |
| A-2 | confirmed | `TestCorporaMeetTheirFloors` logs the five counts in the `le` process |
| A-3 | confirmed | `TestGeneratedImportsCoverTodaysHandList`: every hand import the derivation does not name is a composition root, cycles, or carries `codegen:skip`. `TestEveryCommandRegistrarIsMatched` closes the registrar half |
| A-4 | confirmed | `TestYANGEnumCopyIsReported` reports the console speed enumeration |
| A-5 | BROKEN | recorded in Deviations and the Mistake Log; the landing posture changed, not the rule |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `ai/patterns/registration.md` linking paragraph | read against `cmd/ze/ze_core_dispatch.go`: six module-internal blank imports, one of them `plugin/all` | Yes, corrected at review from "four" to "five" |
| `docs/architecture/command-ownership.md` three-reason table | each row read at its producer: `config/yang/cli/tree.go` imports `plugin/all`; `completion/words.go` imports `cli/client`, which imports `plugin/all`; `internal/perf/cli/register.go` carries `codegen:skip`; `internal/component/aaa/all` registers backends | Yes |
| `ai/INDEX.md` dev-tools row | the description is the one `leroot.Register` publishes, so the two cannot disagree | Yes |
| The `codegen:skip` reasons | `internal/test/cli/register.go` registers suite roots named `bgp`, `firewall`, `l2tp` and `traffic`; `cmd/ze/dispatch_bgp.go`, `dispatch_l2tp.go`, `setup_features_setup.go`, `ze_analyze_register.go` and `ze_perf_register.go` all exist | Yes |

## Core Insight

A gate that narrows its own scope has two jobs, and the second is the one this work
nearly shipped broken: it must say what it judged, and it must not believe a narrow
answer it was handed. The `Widened` flag and the `./...` package answer are one fact
written twice, and the publication carries only one of them. That is the same defect
the gate was built to find, met inside the gate itself, one level up: a second
declaration where the two copies disagree about what the selector knew.
