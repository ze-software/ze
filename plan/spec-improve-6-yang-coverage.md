# Spec: improve-6 -- YANG Coverage Report

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-improve-0-umbrella.md` -- set context
4. `internal/component/config/yang/loader.go` -- module loading
5. `internal/le/command/ownership/commandownership.go` -- precedent for ownership checks

## Task

Ze's YANG surface is registry-driven: plugins and domains register their modules, a
generator produces embed/register glue, and the loader resolves whatever registered.
Nothing reports what that adds up to. There is no single view answering: how many nodes
does each module define, which config roots own them, which leaves lack native
constraints (`range`/`length`/`pattern`/`enum`), which registered nodes no code
consumes, and which modules a build tag compiles out. A cheap per-module node-count
report keeps schema honesty visible as the surface grows.

Ze implements its own modules, not IETF ones, so "coverage" here means
schema-vs-implementation consistency, not standards coverage: a `ze yang coverage`
report (and a internal/le/ verifier where rules are mechanical) producing
per-module node counts, per-leaf constraint status, config-root ownership mapping, and
orphan detection. Wire the mechanical subset into the existing check infrastructure;
the report itself is a developer/agent tool.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - YANG handling design
  → Constraint: four module categories (type-lib / extensions / config-schema / API-schema); the report must grade config-schema modules only and label the others, never grade a type-lib module for "missing constraints"; all walking happens on the RESOLVED Entry tree (GetEntry), never raw modules
  → Constraint: `ze:validate` custom validators are a legitimate constraint carrier -- a bare `type string` leaf with a ze:validate extension is NOT a violation; grading must read the extension
- [ ] `ai/patterns/config-option.md` - constraint expectations per leaf
  → Constraint: every leaf must carry maximum native validation; the report measures exactly this
- [ ] `ai/rules/evidence.md` - report derives from loaded modules, no second list
  → Constraint: node inventory comes from the goyang-resolved tree, never a parallel list
- [ ] `ai/rules/plugins.md` - build-tag-gated module groups the report must label
  → Constraint: `feature-gates.txt` is the single source of truth (<tag> <package>; generator gates pkg AND pkg/yang together); compiled-out modules are invisible to a live loader, so the gating column MUST derive from the manifest file, never a parallel list

### RFC Summaries (MUST for protocol work)
- Not protocol work; RFC 7950 (YANG) semantics come via goyang.

**Key insights:**
- `internal/le/` already hosts mechanical schema/ownership checks
  (`command_ownership.go`); this extends that family, not a new infrastructure.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/config/yang/loader.go` - `DefaultLoader` loads embedded bootstrap modules then registered modules, best-effort resolve (:20-28); `LoadEmbedded` covers ze-extensions/ze-types only (:48-52)
- [ ] `internal/component/plugin/registry/registry.go` - Registration carries per-plugin `YANG` schema content (:64) and `ConfigRoots` (:50)
- [ ] `internal/le/yang/glue/yangglue.go` - generates embed/register glue for YANG files (verify exact behavior during design)
- [ ] `internal/le/command/ownership/commandownership.go` - existing ownership-check precedent (read during design)
- [ ] `internal/component/gnmi/capabilities.go` - `loadModels` walks `loader.ModuleNames()` (:39-52): existing module enumeration the report can share

**Design-phase research completed (2026-07-10; producers read by research agent, loader/registry re-read directly this session):**
- `internal/le/yang/glue/yangglue.go` verified (spec asked for this): discovers `internal/**/yang/` dirs (:137-176, skips internal/test + the registry pkg); generates embed.go (:193-208) and register.go calling `configyang.RegisterModule("<filename>.yang", ...)` (:210-227) -- registration key is the FILENAME, not the YANG module name; `--check` only byte-compares regenerated content (:65-91), says nothing about handlers or coverage
- Loader surface re-read directly: `Resolve`=modules.Process (`loader.go`), `GetEntry`=yang.ToEntry resolved (:109-117), `ModuleNames` (:119-126), `ConfModuleNames` selects the "-conf" name suffix (:128-131) -- the report's config-module scoping mechanism
- Walker precedent to reuse: `internal/component/config/yang/validator.go` `walkTree` recurses entry.Dir (:616-659); native constraints read as `yangType.Kind/Length/Pattern/Range` (:220-234, :254, :268, :361,:426) -- validates assumption A-1's feasibility
- Registry join re-read directly: `YANGSchemas()` (`registry.go`), `ConfigRootsMap()` (:560-573). Module name is NOT stored; must be parsed from the `module <name>` statement in content. TWO registration channels exist: `Registration.YANG` (~74 plugins) vs the loader's `RegisterModule` init channel; `-cmd.yang`/`-api.yang` modules flow ONLY through the loader channel -- the ownership join must handle loader-only modules
- CLI pattern: no `ze yang` root exists; nearest sibling `ze schema` registers via `MustRegisterRootHandler` in an internal owner package (`internal/component/config/schema/cli/register.go`, Run switch `main.go`); command_ownership check requires the root handler live in an internal owner pkg
- JSON: `ai/rules/cli.md` kebab-case keys with explicit tags; sibling checks emit bare `--json` arrays (`command_ownership.go`, `port_defaults.go`)

**Behavior to preserve:** (unless user explicitly said to change)
- Loader semantics (best-effort resolve, bootstrap vs registered split) unchanged; the
  report is read-only over the resolved module set.
- Existing checks in `internal/le/` and their exit-code contract unchanged.

**Behavior to change:** (only if user explicitly requested)
- None; purely additive tooling. (If the constraint check lands in verify as blocking,
  pre-existing violations are burned down or explicitly waived during design.)

## Data Flow (MANDATORY)

### Entry Point
- `ze yang coverage` CLI command (developer/agent tool) and/or `./le yang leaf-mentions report`;
  mechanical subset runs as a check inside the existing verify stage family.

### Transformation Path
1. Load the full resolved module set via the existing loader (`DefaultLoader`).
2. Walk each module's schema tree via goyang: count containers/lists/leaves; record each leaf's type and native constraints.
3. Join with registry data: which plugin registered the module, which ConfigRoots claim its subtrees, which build tags gate it.
4. Emit per-module report rows (nodes, constrained %, owner, gating) and violation lists (unconstrained bare-string leaves, unowned subtrees, registered-but-unresolvable modules).
5. Mechanical violations exit non-zero in check mode; report mode prints tables/JSON.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Loader ↔ report | read-only walk of resolved goyang modules | [ ] |
| Registry ↔ report | module-to-plugin/ConfigRoots join | [ ] |
| Report ↔ verify | check-mode exit code, same contract as internal/le/ | [ ] |

### Integration Points
- `DefaultLoader` (`loader.go`) - module source.
- `registry` module/ConfigRoots metadata - ownership join.
- `internal/le/` + `internal/le/` targets - check-mode wiring.

### Architectural Verification
- [ ] No bypassed layers (report reads via loader, not raw .yang file parsing of its own)
- [ ] No unintended coupling (tool depends on loader + registry read APIs only)
- [ ] No duplicated functionality (shares module enumeration with capabilities' loadModels approach)
- [ ] Registration over hardcoding -- module inventory derives from registration; no hardcoded module list (`ai/rules/plugins.md`, `ai/rules/evidence.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | goyang exposes enough resolved-tree detail (types, ranges, patterns) to grade constraints | goyang powers existing validation; CONFIRMED direction: `validator.go` already reads `yangType.Kind/Length/Pattern/Range` off the resolved tree (:220-234, :254, :268, :361,:426) | Constraint grading incomplete; report ships counts only in v1 | Reuse the walkTree approach; prototype over 2 modules in phase 2 | unvalidated (feasibility evidenced) |
| A-2 | Module-to-plugin ownership is derivable from Registration.YANG + ConfigRoots without new metadata | registry fields exist (:50, :64); REFINED 2026-07-10: module name must be parsed from the `module <name>` statement in content (not stored); `-cmd`/`-api` modules flow only through the loader's RegisterModule channel, so their ownership joins via the generated `yang/register.go` location (owning dir), not Registration.YANG | Need a module-name annotation in Registration | Cross-check 5 plugins' modules during phase 3, including one loader-channel-only module | unvalidated (mechanics specified) |
| A-3 | Existing schema violations (unconstrained leaves) are few enough to burn down before check mode blocks | config-option pattern has been enforced for a while | Check mode starts advisory with a waiver list | First full report run during implementation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Report becomes a stale second source of truth if it caches anything | drift between report and loader behavior | always compute live from the loader; no stored inventories |
| R-2 | "Coverage" naming misleads (suggests IETF-standards coverage) | user/docs confusion | name and docs say schema-consistency report; comparison docs handle standards claims separately |
| R-3 | Check mode blocks unrelated PRs on legacy violations (A-3) | verify failures on untouched modules | advisory first; blocking only for CHANGED modules (matches ./le changed scope philosophy) |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| ze yang coverage | → | loader walk + registry join + report | TestYANGCoverageReport |
| check mode on a module with an unconstrained leaf (fixture) | → | violation detection, non-zero exit | TestYANGCoverageCheckFailsOnBareLeaf |
| ./le yang leaf-mentions report | → | make target runs the tool | test/plugin/yang-coverage.ci |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze yang coverage` | Per-module table: node counts, constrained-leaf %, owning plugin, build-tag gating |
| AC-2 | Module with a bare `type string` leaf (test fixture) | Listed as a violation with module, path, and what to add |
| AC-3 | Registered module that fails to resolve | Reported explicitly (today it is silently skipped by best-effort resolve) |
| AC-4 | Subtree with no owning ConfigRoot | Flagged as unowned |
| AC-5 | Check mode, violations present | Non-zero exit with the internal/le/ output contract |
| AC-6 | JSON output flag | Machine-readable per `ai/rules/cli.md` |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Agent adds a YANG leaf and runs coverage before committing | loader -> walk -> constraint grade -> violation or clean | test/plugin/yang-coverage.ci |
| 2 | Maintainer audits what a build tag removes from the schema surface | report's gating column | TestYANGCoverageReport |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestYANGCoverageReport | coverage tool package test | counts, ownership join, gating labels | |
| TestYANGCoverageCheckFailsOnBareLeaf | same | AC-2, AC-5 | |
| TestYANGCoverageUnresolvedModule | same | AC-3 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| (none numeric; N/A -- reporting tool) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| yang-coverage | `test/plugin/yang-coverage.ci` | command produces the report and supports pipes per `ai/rules/cli.md` | |

### Interop Tests (MANDATORY for protocol features)
- N/A: developer tooling, no wire behavior.

## Files to Modify
- `internal/le/` - `ze-yang-coverage` target; verify wiring per R-3 decision
- `docs/guide/command-reference.md` - document the command
- ~~`cmd/ze/` command tree registration~~ superseded 2026-07-10: root handler
  registers in an internal owner package beside `internal/component/config/yang/`
  via `MustRegisterRootHandler` (sibling: `schema/cli/register.go`);
  command_ownership requires internal-owner registration, not cmd/ze

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | N/A | report adds no config surface |
| YANG validation constraints | N/A | no new leaves |
| YANG custom validators | N/A | no new leaves |
| CLI commands/flags | Yes | `yang` root handler in internal owner package (Key Design Decisions) |
| CLI grammar (action before identifier) | Yes | verify `yang coverage` against `ai/rules/cli.md` + cli_grammar check at implementation; rename subcommand if a grammar rule fires |
| Editor autocomplete | N/A | no config leaves; command completion comes from registration |
| Functional test for new RPC/API | Yes | `test/plugin/yang-coverage.ci` (already in TDD plan) |
| Pipe completeness | Yes | report output through pipes per `ai/rules/cli.md` (wiring row exists) |
| Env var registration | N/A | none |
| Doctor check for runtime dependencies | N/A | reads embedded/registered schema only; no runtime dependency introduced |
| Prometheus counters/metrics | N/A | developer/agent tool; no runtime state |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/command-reference.md` (dev/agent tool; no separate guide page) |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | command-reference suffices |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | read-only over registry/loader |
| 9 | RFC behavior implemented, changed, or newly proven? | No | not protocol work |
| 10 | Test infrastructure changed? | Yes (if check mode joins verify per R-3) | verify/make-target docs named at implementation |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- schema-consistency tooling note |
| 12 | Internal architecture changed? | No | additive tool |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | Yes | command inventory row per `ai/rules/repo-maintenance.md` + `ai/INDEX.md` keyword row |
| 16 | Any changed source file is referenced by existing doc source anchors? | Check at implementation | grep `docs/` for anchors |
| 17 | Existing docs show config/CLI/API examples for this area? | No | none exist yet |

## Files to Create
- coverage walker + report (location per module tiers during design; likely beside `internal/component/config/yang/`)
- check-mode entry in `internal/le/` family if design separates it
- `test/plugin/yang-coverage.ci` - functional test

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - command + make target registered, failing wiring tests
2. **Phase: module walk + counts** (A-1 prototype hardened)
3. **Phase: ownership/gating join + violations** (AC-2..AC-4)
4. **Phase: check mode + JSON output** (AC-5, AC-6; R-3 decision applied)
5. `./le verify current mode full`, learned summary, two-commit closure

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-6 with file:line |
| Correctness | inventory derives live from loader; zero hardcoded module lists |
| Naming | "coverage" framed as schema consistency, not standards conformance (R-2) |
| Registration over hardcoding | command registered via dispatch; derives from registry (`ai/rules/plugins.md`) |
| Pipe completeness | report output supports all pipe operators |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | none from network; tool reads embedded/registered schema only |
| Resource exhaustion | walk bounded by schema size; no issue expected |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Constraint Grading Rules (added 2026-07-10 at design gate, per user request)

Grading applies to leaves of config-schema (`-conf`) modules only. Three grades;
check mode (advisory-first, changed-modules-only per R-3) gates only VIOLATION.

| Leaf type (resolved, typedefs followed) | Grade rule |
|------------------------------------------|-----------|
| boolean, empty | PASS by construction (finite domain) |
| enumeration, identityref, bits | PASS by construction |
| leafref | PASS here (domain = referent's); referent graded where defined |
| string | PASS if any constraint carrier present: `length`, `pattern`, a typedef that carries one (ze-types.yang), or `ze:validate`. Bare `type string` with none = VIOLATION |
| int8..uint64, decimal64 | PASS if `range` present or `ze:validate` present; no range = ADVISORY (full native span may be semantically exact, e.g. uint16 port; humans review) |
| binary | `length` present = PASS; absent = ADVISORY |
| union | graded per member; worst member's grade wins |
| leaf-list | leaf rule for the element type; missing `max-elements` adds an ADVISORY row |
| list | keys are mandatory per YANG (not graded); missing `max-elements` = ADVISORY row |

Rules for the walk: grade off the RESOLVED Entry tree (`GetEntry`); a typedef's
constraints count for the leaves using it; `ze:validate` is read from the extension
on the leaf (Required Reading constraint above). `default`/`mandatory` are counted
in report statistics, never graded. The phase-2 prototype validates these rules on
2 real modules; any promotion/demotion of a grade class is recorded in Key Design
Decisions with the module that motivated it.

## Design Insights
- Grading feasibility is not speculative: the validator already reads
  Kind/Length/Pattern/Range off the resolved tree (`validator.go, :254,
  :268, :361,:426`); the report reuses that access pattern read-only.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Schema-consistency report, not IETF coverage | a standards-coverage counter over IETF modules | Ze implements its own modules; the honest local guardrail is constraint/ownership consistency |
| `yang` root handler registered in an internal owner package beside `internal/component/config/yang/` | subcommand under existing `ze schema`; cmd/ze registration | Sibling pattern is `ze schema` (`schema/cli/register.go`); command_ownership requires internal-owner registration; `schema` owns hub schema introspection, `yang` owns module/coverage introspection -- distinct concerns |
| Config-module scoping via the `-conf` suffix convention (`ConfModuleNames`, `loader.go`) + category labels for the rest | grade everything uniformly | Four module categories exist by design; grading a type-lib for "missing constraints" would be all noise |
| Constraint grading counts `ze:validate` extensions as constraints | native-only grading | A bare leaf with a custom validator IS validated at runtime; native-only grading would report false violations |
| Gating column derives from `feature-gates.txt` | live-loader observation; hand list | Compiled-out modules are invisible to a live loader; the manifest is the declared SSOT |
| Check-mode output follows sibling checks (bare `--json` array + exit contract); CLI report mode uses the json-format envelope | one format for both | Check mode must match the `internal/le/` family contract for verify integration; the operator-facing CLI follows `ai/rules/cli.md` |
| Handler-claim ENFORCEMENT excluded -- owned by `spec-improve-7-yang-handler-gate.md` | fold enforcement into this report's check mode | One spec per property: improve-6 reports and advises, improve-7 blocks; prevents double ownership of orphan detection (same pattern as the port-defaults carve-out) |

## Known Limitations
- Says nothing about standards conformance; `docs/comparison.md` and interop tests own
  that story.

## Implementation Summary

### What Was Implemented
- (fill during implementation)

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
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Post-wave corrections (2026-07-10)

- A mechanical YANG-default check subset of this spec ALREADY LANDED in the followup wave: `internal/le/portdefaults/portdefaults.go` compares each service's YANG `refine port { default N }` (regex `refinePortRe` at `port_defaults.go`, extraction `yangPortDefault` at `:143`) against the hand-maintained Go listener-defaults table, wired as `./le port-defaults check` (the retired `Makefile:327-329` (current producers: `internal/le/` native action tables)) and run by the live verify stage list in both branches (`internal/le/verify/engine/run.go`, `:140`).
- The proposed `ze yang coverage` check mode must NOT duplicate that coverage: port-default consistency is owned by `./le port-defaults check`. Design the coverage tool as a sibling in the existing `internal/le/` family (now `command_ownership.go`, `iface_resolution.go`, `plugin_process_boundary.go`, `port_defaults.go`, `cli_grammar.go`) and scope its constraint grading to what the port gate does not already check.
- Loader evidence re-verified, not stale: `DefaultLoader` (`internal/component/config/yang/loader.go`, best-effort `LoadRegistered`/`Resolve` at `:25-26`) and the `LoadEmbedded` bootstrap set covering ze-extensions/ze-types still match the Current Behavior citations.

### Design-phase corrections (2026-07-10)

- **Carve-out vs improve-7 (new sibling, this session):** handler-claim enforcement
  (every config root claimed by a plugin / no phantom claims, blocking gate + doctor
  check) was owned by `spec-improve-7-yang-handler-gate`, which shipped that gate and
  closed on 2026-08-29: `./le config claims` (`internal/le/config/claims/configclaims.go`)
  now runs in both verify-stage populations. This spec's orphan
  detection (AC-4 unowned subtrees, unconsumed nodes) stays REPORT-only; its check
  mode never gates on claim completeness. Scope boundary recorded in both specs.
- **Umbrella A-1 partially validated for this finding:** the reviewed daemon's
  coverage tool was read at primary source this session
  (`holo-tools/src/bin/yang_coverage.rs:65-150`: per-module markdown split
  config/state/RPC/notifications, published in its README). Ze's reframing to
  schema-consistency (Key Design Decisions) stands.
- **AC-4 note:** with improve-7 owning enforcement, AC-4's "flagged as unowned"
  remains a report row here; the same condition failing a build belongs to improve-7.
