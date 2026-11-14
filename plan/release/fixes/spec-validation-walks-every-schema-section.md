# Spec: validation-walks-every-schema-section

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | A `ze:validate` declared outside a hand list loads, passes the startup check and never runs, so `ze config validate` and `ze start` accept a value the schema refuses. That is accepted-but-ineffective configuration in a shipped capability, a release defect (`plan/README.md`, "Split fixes from features") |
| RFC gaps | - |
| Depends | Split from `plan/release/fixes/spec-yang-single-source-of-truth.md` (piece B). Siblings: `plan/release/fixes/spec-yang-defaults-applied-once.md` (piece C), `plan/release/fixes/spec-review-typed-config-decode.md` (piece A, uint64). The choice and `when` half of the mandatory walk stays with `plan/release/fixes/spec-config-yang-when-unique-choice.md` |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Problem.** `ValidateCustomSections` runs `ze:validate`, range, length and
pattern checks only inside the 21 top-level sections that `validatedSections`
names by hand. A `ze:validate` under any other section is never run. The
mandatory check reaches every present section through `mandatoryFailures`, but
it reads the same hand list to decide which sections it skips.

**Why the list exists.** It is not an oversight: three validators answer from a
registry that does not hold the value they are asked about at the moment the
walk runs, so widening the list would refuse valid shipped configs. The list is
a workaround for those three defects.

**Goal.** The walk covers every section the compiled schema derives, the hand
list is deleted, and the three validators answer correctly when the walk runs.
`ValidatorSectionCoverage` stays the derivation of which sections declare a
validator; it stops subtracting a hand list.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md`, "Where a validator actually runs" and "Startup Integrity Check"
  → Constraint: one walk, three callers (`LoadConfig`, `ze config validate`, the editor commit through `sectionErrors`); a second walk is the defect the shared function closed
  → Decision: the derivation reads the resolved model, never the YANG source, because a `ze:validate` in a grouping lands wherever the grouping is used
- [ ] `docs/architecture/config/yang-config-design.md`, "Layer 4: The BGP peer pipeline"
  → Constraint: `bgp` cross-plugin rules (role against filter chains) stay in the peer pipeline; only the per-leaf `ze:validate` and type checks join the one walk
- [ ] `ai/doors/config-yang.md`, Traps: "A `ze:validate` can be declared and never run"

**Key insights:**
- The population is already derived: `ValidatorSectionCoverage` and `TestEveryValidatorSectionIsWalkedOrExcused` compute the declaring sections from the schema. Only the walk itself still reads a hand list.
- The blockers are validator defects, not section properties.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/validate_sections.go` `validatedSections` - 21 names, six of which (web, ssh, dns, looking-glass, mcp, managed) are not top-level sections, so their iterations do nothing; its comment records 25 real sections absent of 36
- [ ] `internal/component/config/validate_sections.go` `knownUnwalkedValidatorSections` - five excused sections: `bgp`, `redistribute` (address family defect), `policy` (user-visible: four `policy route ... rule ... from` leaves accept a value their annotation forbids), `static`, `control-plane-protection` (each owes a measurement run)
- [ ] `internal/component/config/validate_sections.go` `ValidatorSectionCoverage` - derives `Declaring` from `loader.ConfModuleNames()` entries and subtracts `validatedSections` and the excused map; `mandatoryFailures` skips every section in `validatedSections` plus `bgp`
- [ ] `internal/component/config/validators.go` `AddressFamilyValidator` - reads `registry.FamilyMap()`, which `registry.go` builds from plugin registrations only; `registry.AllFamilies` also holds the engine-builtin families
- [ ] `internal/component/config/validators.go` `SendMessageValidator` - accepts `bgpevents.BaseSendTypes()` statically, and plugin send types only through `events.IsValidSendType`
- [ ] `internal/component/plugin/resolve.go` `RegisterPluginSendTypes` - copies every `registry.All()` entry's `SendTypes` into the events registry once; called from `plugin/server` `NewServer` and from `bgp/reactor/config.go` `PeersFromTree` (which also calls `RegisterPluginEventTypes`), both after `LoadConfig` runs the walk. The source data is in the registry from `init()`; only the copy is late
- [ ] `internal/component/config/infra/validator_coverage_test.go` `TestEveryValidatorSectionIsWalkedOrExcused` - fails on a declaring section neither walked nor excused

**Behavior to preserve:**
- Every shipped config under `test/` and `etc/` that is valid under its YANG still loads.
- `bgp` cross-plugin checks keep running in `PeersFromConfigTree`, with the same messages.
- The mandatory walk keeps every refusal it makes today (`TestRFC7950MandatoryRefusesBGPConfig`, `TestMandatoryBGPPeerAndGroupEntriesAreWalked`).

**Behavior to change:**
- A `ze:validate`, range, length or pattern violation under any top-level section is refused by validate, start, reload and the editor commit.
- `AddressFamilyValidator` accepts every builtin family; the send-type and receive-event validators answer from registries filled before `LoadConfig`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The one validation walk | `validate_sections.go` `ValidateCustomSections`; `yang-config-design.md`, "Where a validator actually runs" | Runs validators and constraints over the listed sections for its three callers | Extends: same function, population from the schema |
| Section derivation | `validate_sections.go` `ValidatorSectionCoverage`, `subtreeDeclaresValidator` | Derives the declaring sections from the resolved model | Extends: becomes the population check of the walk; stops subtracting a hand list |
| Schema section list | `config/yang/validator_mandatory.go` `(*Validator).Sections` | Lists the top-level sections the compiled schema derives; `mandatoryFailures` iterates it | Extends: the walk iterates it for every check, not only the mandatory one |
| Coverage test | `config/infra/validator_coverage_test.go` `TestEveryValidatorSectionIsWalkedOrExcused` | Refuses a declaring section neither walked nor excused | Extends: asserts every declaring section is walked, with no excuse map left |
| Validator existence check | `config/yang/validator_registry.go` `CheckAllValidatorsRegistered` | Refuses a `ze:validate` naming no registered function at startup | Does not apply: it answers whether a function exists, which this spec does not change |
| Hand lists | `validate_sections.go` `validatedSections`, `knownUnwalkedValidatorSections` | Name the walked and the excused sections | Replaces: both deleted (`ai/rules/no-layering.md`) |
| Family registry | `plugin/registry/registry.go` `FamilyMap`, `AllFamilies` | Plugin families only; plugin plus builtin families | Extends: the validator reads the complete set |
| Send-type and event registration | `plugin/resolve.go` `RegisterPluginSendTypes`, `RegisterPluginEventTypes` | Copy `registry.All()` declarations into the events registry, after load | Extends: the copy runs before the walk (or the validators read the registry the copy reads) |
| BGP peer pipeline | `bgp/config/peers.go` `PeersFromConfigTree`; `yang-config-design.md`, "Layer 4" | Cross-plugin peer rules; per the `validatedSections` comment it also re-checks every bgp-bound validator name | Does not apply for cross-plugin rules. If the walk and the pipeline then report one bgp leaf twice, one site keeps it (R-2) |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A config file at `ze start`, SIGHUP reload, `ze config validate`, an editor `commit`.

### Transformation Path
1. Parse to `Tree`, prune inactive nodes (`LoadConfig`, `sectionErrors`).
2. `ValidateCustomSections` runs tree validators, then walks every section `(*Validator).Sections` derives, applying custom validators, constraints and the mandatory check.
3. Failures become `SectionValidationError`, rendered by `joinSectionRefusals`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to plugin registry | Validators read `registry.AllFamilies` and the send and event registries | No |

### Changed-Wire Census

No wire changes: the work changes which sections an in-process walk reads and what three validator functions accept. No message, file format or plugin protocol field is added or changed. Search: `ValidateCustomSections`, `validatedSections` and `SectionValidationError` callers under `internal/` and `cmd/`; all are in-process Go.

### Integration Points
- `LoadConfig` (`config/loader.go`), `runValidation` (`config/cli/cmd_validate.go`), `sectionErrors` (`cli/validator.go`): unchanged callers.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | One walk keeps its three callers |
| No unintended coupling (components stay isolated) | No | Validators read registries, never import plugins |
| No duplicated functionality (extends existing, does not recreate) | No | `ValidatorSectionCoverage` extended, hand lists deleted |
| Zero-copy preserved where applicable (refs, not copies) | No | Config load is not a hot path |
| Registration over hardcoding, outbound | No | Send types and events already arrive by `registry.Registration` |
| Registration over hardcoding, inbound | No | Searched: `validatedSections`, `knownUnwalkedValidatorSections`; `bgpSessionPathOf` is kept (the owner ruling's leaf set, owned by `spec-config-yang-when-unique-choice`) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Fixing the three validators removes every refusal of a valid shipped config | `validatedSections` comment: the widening blockers are 742 + 33 family sites and two late registries | Some shipped config is refused | parse, install, ui and reload suites after the widening | unvalidated |
| A-2 | Every plugin send and event type is in `registry.All()` before `LoadConfig` | `RegisterPluginSendTypes` reads only `registry.All()`, filled from `init()` | A type is known only after a plugin starts | read every `SendTypes` and event declaration source | unvalidated |
| A-3 | The `policy` section's four `from` leaves hold wrong values today, not valid ones | excused map text | Shipped configs carry such values | measure the 71 policy configs before the widening | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An external plugin's send type is unknown at load | functional test with an external plugin declaring a send type | Refused at load, never accepted silently; external declarations at load belong to `spec-plugin-external-yang-at-startup` |
| R-2 | Double refusal for a bgp leaf the peer pipeline also checks | duplicated lines in `ze config validate` output | One site keeps the check |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A config is refused at load or commit |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `spec-config-yang-when-unique-choice` (mandatory choice and `when`), `spec-yang-defaults-applied-once`, `spec-gates-share-one-semantic-population` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` on a config with a `policy route ... rule ... from` prefix its `ze:validate` refuses | → | `ValidateCustomSections` over the derived sections | `test/parse/validate-walks-every-section.ci` |
| `ze config validate` with a builtin bgp family | → | `AddressFamilyValidator` reading the complete family set | `TestAddressFamilyValidatorAcceptsBuiltinFamilies` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A `ze:validate` failure under any top-level section (old AC-3), reproduced under `policy`, `static`, `control-plane-protection` and a test module's new section | `ze config validate`, `ze start`, reload and the editor commit refuse it with the same message; `validatedSections` and `knownUnwalkedValidatorSections` no longer exist | None: an excused section is the defect. A completion-only validator (nil `ValidateFn`) refuses nothing, as designed |
| AC-2 | The coverage test | `TestEveryValidatorSectionIsWalked` (renamed from `TestEveryValidatorSectionIsWalkedOrExcused`) passes with every declaring section walked, and asserts the sections it expects are present before reading an empty answer | A section a feature tag compiled out is absent from that build; the test asserts the present set |
| AC-3 | A bgp or redistribute family the engine builds in, and one a plugin registers | Both accepted by the walk; an unknown family refused naming the value and the known families | None |
| AC-4 | A plugin send type and a plugin event type in `process` bindings | Accepted at `LoadConfig`, before any plugin starts; an unknown one refused | An external plugin's type declared only at Stage 1 is refused at load until external declarations reach load (`plan/post-release/features/spec-plugin-external-yang-at-startup.md`) |
| AC-5 | The mandatory walk (old AC-4) after the list is deleted | Every refusal today's mandatory tests prove still refuses at validate and start alike | The choice-case and `when` gaps stay with `spec-config-yang-when-unique-choice` |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes a malformed match prefix in a policy route rule | file → `LoadConfig` → `ValidateCustomSections` → refusal with fix | `test/parse/validate-walks-every-section.ci` |
| 2 | Configures `family ipv4 unicast` on a peer | file → walk → `AddressFamilyValidator` → accepted | `test/parse/validate-walks-every-section.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestValidateCustomSectionsCoversEverySchemaSection` | `internal/component/config/validate_sections_test.go` | AC-1 with a test module section | |
| `TestEveryValidatorSectionIsWalked` | `internal/component/config/infra/validator_coverage_test.go` | AC-2 | |
| `TestAddressFamilyValidatorAcceptsBuiltinFamilies` | `internal/component/config/validators_test.go` | AC-3 | |
| `TestSendAndEventTypesKnownBeforeLoad` | `internal/component/config/validators_test.go` | AC-4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | the spec adds no numeric input | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `validate-walks-every-section` | `test/parse/validate-walks-every-section.ci` | a value refused in a formerly unwalked section, a builtin family accepted | |

### Interop Tests (Scope: protocol)
N-A: config validation only, no wire behavior.

## Files to Modify
- `internal/component/config/validate_sections.go` - delete the hand lists; walk `(*Validator).Sections`
- `internal/component/config/validators.go` - family, send-type and event validators read complete registries
- `internal/component/plugin/resolve.go` - send and event types copied before the walk
- `internal/component/bgp/config/peers.go` - only if R-2 shows a double report
- `internal/component/config/infra/validator_coverage_test.go` - the coverage test
- `docs/architecture/config/yang-config-design.md` - "Where a validator actually runs", "Layer 4"
- `ai/doors/config-yang.md` - the hand-list trap

## Files to Create
- `test/parse/validate-walks-every-section.ci` - functional proof

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No node added |
| YANG validation constraints | No | Existing constraints start running |
| YANG custom validators | Yes | `validators.go` |
| CLI commands/flags | No | None |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | No | Completion unchanged |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No output |
| Env var registration | No | None |
| Doctor check for runtime dependencies | No | None |
| Doctor code per problem class | No | Refusals reuse the config validation codes |
| Prometheus counters/metrics | No | None |
| BGP family surface | No | Validation only |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior changed? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md`: the walk's population, the three validators, Layer 4 |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | No | |
| 15 | Registered inventory changed? | No | |
| 16 | Changed source referenced by doc anchors? | Yes | derive with `./le spec citation anchors spec plan/release/fixes/spec-validation-walks-every-schema-section.md`. Declared by changed files and unaffected: `docs/architecture/api/process-protocol.md` (no protocol change: send and event types keep their Stage 1 declaration) and `docs/architecture/config/syntax.md` (no grammar change) |
| 17 | Existing docs show examples for this area? | Yes | `ai/doors/config-yang.md` trap text on the hand list |

## Implementation Steps

1. **Phase: Wiring** - write the `.ci` and `TestValidateCustomSectionsCoversEverySchemaSection`; both fail on today's list.
2. **Phase: validators** - complete family set, send and event types before the walk (AC-3, AC-4).
3. **Phase: measurement** - run the parse, install, ui and reload suites with the walk widened; each newly refused shipped config is either a wrong config (fixed in the repository) or a validator defect (fixed in the validator).
4. **Phase: delete the hand lists** - walk `(*Validator).Sections`; repoint `ValidatorSectionCoverage` and the coverage test (AC-1, AC-2, AC-5).
5. **Phase: docs** - the design page and the door trap.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Correctness | No shipped valid config is newly refused; each refusal names the leaf and the fix |
| Data flow | One walk, three callers, no second walk added |
| Rule: no-layering | No residual allowlist or excuse map |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Hand lists deleted | `grep -n validatedSections internal/component/config` answers nothing |
| Every section walked | `TestEveryValidatorSectionIsWalked` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A sensitive leaf's refusal stays redacted (`SectionValidationError.Message`) in newly walked sections |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Shipped config refused | Decide wrong config or wrong validator, at the producer |
| Late registry | Move the copy before the walk |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `gate-excludes-part-of-its-population.md` | 2026-09-02 | `validatedSections` hand list: `ze:validate` outside it never runs |
| `gate-excludes-part-of-its-population.md` | 2026-08-11 | section hand list beside the schema-derived sections (now `validatedSections`) |
| `gate-excludes-part-of-its-population.md` | 2026-08-11 | three validators answer from a registry not holding the asked value |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Walk every section `(*Validator).Sections` derives | Keep a list and widen it section by section | A list is a second declaration of the schema's sections (`ai/rules/principles.md`) |
| Fix the three validators | Keep excusing the sections they break | The excuse hides `policy` refusals operators need |

## Known Limitations
- Choice, `when` and `unique` evaluation stay with `plan/release/fixes/spec-config-yang-when-unique-choice.md`.

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

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests pass, run by the main thread: `./le test unit pkg path ./internal/component/config/`, `./le test unit pkg path ./internal/component/config/infra/`, `./le test functional parse`, `./le test functional reload`, and the suites `./le repo changed scope print both` reaches. No `./le verify` command
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests N-A: no wire change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + journal rows marked `fixed:`
- [ ] **Commit B:** `remove plan/release/fixes/spec-validation-walks-every-schema-section.md` only
