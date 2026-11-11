# Spec: doctor-coverage-0-umbrella

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P2 |
| Release reason | `ai/META-DECISIONS.md` (Goal and scope): `ze doctor` reports every problem an operator can hit, with its cause and the command that fixes it, and a problem class doctor does not report is a gap owed a check. A shipped NOS whose faults surface only in logs cannot be operated, so this is release validation of the operator surface |
| RFC gaps | - |
| Depends | - |
| Phase | umbrella: children 1 and 2 designed; area children created by child 1 |
| Handoff | - |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Thomas: "create a spec to add all those to ze doctor so it can be used to find
any problem", then "Not only the one you have, but all problems. So we will
have to audit the code for all the things we need to be added to it."

The goal: `ze doctor` (and `show doctor` on every daemon surface) finds any
problem the box has, names its cause in plain words, and prints the command that
fixes it. Today it reports host readiness, config references, listeners and a
set of plugin checks; it does not report most runtime faults, which leave only a
log line. The home-CPE lab work of 2026-11-07 met eight such classes in one day.

This umbrella splits the work by owning area, as `ai/rules/planning.md` asks of
a work item too large for one spec:

| Child | Spec | Work it owns | Status |
|-------|------|--------------|--------|
| 1 | `spec-doctor-coverage-1-audit-and-gate` | The audit of the whole tree, the inventory file, the `runtime` check phase, the fix field, the local route asking the daemon, and the gate that refuses a new problem class with no doctor check | design |
| 2 | `plan/release/validation/spec-doctor-coverage-2-home-cpe-batch.md` | The eight classes the home-CPE lab found, one registered check each | design |
| 3..N | `plan/release/validation/spec-doctor-coverage-<n>-<area>.md`, one per door area (`ai/doors/README.md`) holding inventory gaps | The remaining gaps of that area, each a registered check | created by child 1 at the end of its audit (child 1 AC-6) |

The area children cannot be written before the audit: their content is the
audit's output. Child 1 owes their creation, so the work stays visible as specs
(`plan/README.md`, "Deferred work remains a visible spec").

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/doctor-and-health-checks.md` - the three tiers, phases, registration owners, the two tests a check carries
  → Decision: `ze doctor` and `show doctor` run one producer, `doctor.Answer`; the daemon adds the doctor checks of every plugin it runs (`HandleDoctor`). Every child extends that producer, never a second one
  → Constraint: a check is registered by the package that owns the dependency (table "Where each owner registers its check"); `internal/component/doctor` holds only checks with no narrower owner
  → Constraint: each check carries a unit test in the owning package and a functional test through `ze doctor --json` or `show doctor`
- [ ] `ai/doors/system.md` - the shared design
  → Constraint: "A readiness check: a doctor check registered by the package that owns the dependency" (Shared machinery table); a central list of checks is the defect
  → Constraint: a plugin can run in another process, so a check reading another component's memory by a Go call answers nothing when the plugin runs forked
- [ ] `ai/rules/user-facing-errors.md` - message shape
  → Constraint: problem first, then why, then the fixing command on a line of its own; no bare errno or Go error chain as the whole message
- [ ] `ai/META-DECISIONS.md` - scope rules
  → Decision: every operator command, `ze doctor` included, runs through the daemon on every command surface; the local route stays for an unreachable daemon
  → Decision: a problem class found in code, a lab or a test that doctor does not report is a gap owed a doctor check (the standing rule; no owner question remains about it)
  → Constraint: day-to-day work is judged by the tests of the components it changed; no agent runs `./le verify`

**Key insights:**
- Two classes of check exist: config or kernel readable (works on the local route), and daemon-state readable (works only inside the daemon). Child 1 makes the second class explicit with a `runtime` phase, so a local run never answers healthy for state it cannot see
- The gate judges what a commit adds (an added problem site with no doctor code), which `ai/META-DECISIONS.md` lets stand as a refusal
- Plugin diagnostics once lost everything but code, severity and message on the way to the operator; child 1 gave `rpc.DoctorCheckDiagnostic` a `Fix` field (ce58dddf53), and `collectDoctorChecksFrom` now carries each plugin's fix to the operator. Since 10c6647f6d the doctor-coverage gate refuses a `diagnostic.Diagnostic` finding with no `Fix`; it does not yet judge an `rpc.DoctorCheckDiagnostic` (`spec-doctor-coverage-12-plugin-infra` AC-46)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/doctor/answer.go` - `Answer(validated, runtime RuntimeChecks)`: runs `runChecks(configPath)`, appends the daemon-supplied runtime diagnostics when `runtime` is non-nil; the local route passes nil
- [ ] `internal/component/doctor/cmd/show.go` - `HandleDoctor` runs `doctor.RunRuntimeChecks` and `collectDoctorChecksFrom`, which asks each plugin the daemon runs (the caller excepted) for every check it declared at Stage 1, whatever its phase, ordered by `Order`, each under `perCheckTimeout`, truncated at 64 diagnostics, carrying each finding's `Fix`; since 149497ce12 this is the only route that runs a declared check, and a compiled-in plugin's `registry.DoctorCheckDef` runs through `doctor.Answer` at a config phase only
- [ ] `internal/core/diagnostic/doctor_registry.go` - `RegisterDoctorCheck`, `DoctorCheck{Name, Phase, Order, Component, Dependencies, Platforms, Codes, Check}`, `DoctorCheckContext{Tree, ConfigDir, Plugins, Store, Platform}`
- [ ] `pkg/plugin/rpc/types.go` - `DoctorCheckPhase` (`pre-config`, `missing-config`, `parsed-config`, `post-config`) with `Valid`; `DoctorCheckDiagnostic{Code, Severity, Message, Fix}` (the fix field landed with child 1, ce58dddf53); the phase set includes `runtime`
- [ ] `internal/core/diagnostic/types.go` - `Diagnostic` carries `Help`, `FixSafety`, `Repair`; `DoctorResult{SchemaVersion, Ready, Diagnostics}` is the `--json` envelope
- [ ] `internal/core/report/report.go` - the operational report bus behind `show warnings` and `show errors`: state warnings raised and cleared, error events in a ring, process-local

**Behavior to preserve:**
- `ze doctor --json` envelope `{schema-version, ready, diagnostics}` and exit 1 when not ready
- every existing check, code and phase
- `ze doctor kernel-capabilities` and its exit rule

**Behavior to change:**
- doctor reports every problem class the audit finds, through registered checks (children 2..N)
- runtime state is checked inside the daemon; local `ze doctor` asks a reachable daemon, and says when it could not reach one (child 1, O-1)
- every Warn and Error log line carries a doctor code, with no transient marker; a transient class shows as a recent-event row from the report bus (child 1, O-2)

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Doctor check registry | `internal/core/diagnostic/doctor_registry.go` `RegisterDoctorCheck` | Owner packages register phase-ordered checks with their codes | extends: every new check registers here or through the plugin field below; no check is called by hand |
| Plugin doctor checks | `internal/component/plugin/registry/doctor.go` `DoctorCheckDef`, `Registration.DoctorChecks`; `internal/component/doctor/checks_plugin_registry.go` | An internal plugin declares config-phase checks; doctor bridges them per phase. Since 149497ce12 a check takes one route: the registry refuses a `runtime`-phase `DoctorCheckDef` (`validateDoctorCheckDef`), a check over live plugin state is declared in the Stage 1 `sdk.Registration` `DoctorChecks` and answered by `OnDoctorCheck`, and Stage 1 refuses a name declared on both routes (`refuseDoctorCheckDeclaredTwice`) | extends: plugin-owned config and host classes register here; live-state classes go through the Stage 1 callback |
| Daemon doctor RPC | `internal/component/doctor/cmd/show.go` `HandleDoctor`, `collectDoctorChecksFrom` | `show doctor` runs the same producer in the daemon and adds every running plugin's checks | extends: the daemon route is where `runtime` checks run (child 1) |
| Doctor phases | `pkg/plugin/rpc/types.go` `DoctorCheckPhase`; page "The phase decides what a check may read" | Four phases, declared once, aliased by `diagnostic` | extends: child 1 adds `runtime`, declared in the same place |
| Diagnostic code registry | `internal/core/diagnostic/registry.go` `Register`, `ze show diagnostic <code>` | Each code has a title, description and example commands | extends: every new code registers from its owner with its fix command |
| Kernel capability enrolment | `internal/component/kernelcap` `MustRegister` | One owner call puts a kernel feature on doctor, start, reload and validate | does not apply to the new classes: none is a missing kernel feature; reused where the audit finds one |
| Live health registry | `internal/core/health/registry.go` `Register`, `RegisterWhen`; `show health` | Per-subsystem healthy or degraded with a reason, daemon only | does not replace doctor: health has no code and no fix contract; the audit records each health row and the doctor check that owes its cause and fix |
| Report bus | `internal/core/report/report.go` `RaiseWarning`, `ClearWarning` | Operator warnings and errors with codes, `show warnings` | extends: child 1's gate counts each raise as a problem site that needs a doctor code |
| Dependency inventory test | `internal/component/doctor/doctor_checks_test.go` `TestDoctorDependencyInventory` | Holds an expected total of runtime dependencies | does not apply as the gate: a count cannot tell which problem lacks a check; child 1's gate names the site |
| Enumeration gate | `internal/le/arch/enumeration`, `./le arch enumeration check` | AST scan refusing literal copies of registry contents, with an exempt marker | extends as a model: child 1's gate is a sibling AST scan under `internal/le/arch/` |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator types `ze doctor [--json] [config <file>]` locally, or `show doctor` over SSH, MCP, REST, gRPC or the web

### Transformation Path
1. Command dispatch reaches `doctor.Answer` (local) or `HandleDoctor` (daemon)
2. `runChecks` runs registered checks phase by phase; the daemon adds plugin checks and, after child 1, `runtime` checks
3. Each check reads its owner's state and returns diagnostics with code, severity, message and fix
4. `renderDoctor` draws text, or the caller's `| json` renders `DoctorResult`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator to daemon | `show doctor` command dispatch | Yes, `HandleDoctor` exists |
| Engine to plugin | `ze-plugin-callback:doctor-check` RPC per declared check | Yes, `collectDoctorChecksFrom` |

### Changed-Wire Census

**Search evidence:** the umbrella changes no wire itself, so it carries no census rows; child 1 owned the one wire change (`rpc.DoctorCheckDiagnostic` gains a fix field, and the phase set gains `runtime`, landed in ce58dddf53 with its census), and each area child carries the census of the wires it changes.

### Integration Points
- `doctor.Answer` and `HandleDoctor` - every child's checks reach the operator through them

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | all checks reach the operator through `doctor.Answer` |
| No unintended coupling (components stay isolated) | Yes | each check lives in its owner |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing Mechanisms table |
| Zero-copy preserved where applicable (refs, not copies) | Yes | no hot path |
| Registration over hardcoding, outbound | Yes | checks register from owners |
| Registration over hardcoding, inbound | Yes | `runChecks` and `collectDoctorChecksFrom` iterate registries; no list learns a check name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Splitting by door area keeps each child within one context's door reads | `ai/META-DECISIONS.md`, spec doors gate: "a spec whose edits need many doors is split into narrower child specs" | children too large | child 1 sizes each area from its inventory rows | unvalidated |
| A-2 | Every problem class can be read at doctor time from owner state, kernel state or a bounded record the owner keeps | the eight classes of child 2 each have a readable state | a class with no readable state needs the owner to record it | child 1 inventory column "state it reads" | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The audit grows without end (about 3,200 Warn and Error log call sites under `internal/component`, `internal/plugins` and `cmd/ze`, counted by grep on 2026-11-08) | child 1 inventory not converging | the gate's report enumerates the mechanical sites; the manual walk covers silent states per area; rows are grouped into classes, never one row per log line |
| R-2 | A runtime check answers healthy on the local route because it cannot see daemon state | a local run with a broken daemon reads ready | child 1: `runtime` phase never runs in the local process; the local route asks a reachable daemon for it and, when none answers, reports that it did not check (owner decision 2026-11-08, child 1 O-1) |
| R-3 | Doctor turns slow with many runtime checks | `show doctor` over the per-check timeout | each check keeps the existing per-check timeout; reads are snapshots, no probes on a hot path |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | doctor reports a false fault or misses one; no forwarding behavior changes |
| How is it reverted? | single commit revert per check |
| Who else touches this path? | home-CPE children 1, 4, 12, 13, 20, 23 (state the checks read); `spec-operator-errors-name-the-fix` (error text) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze doctor --json` | → | `doctor.Answer` running the registered checks | `TestDoctorRuntimePhaseNotRunLocally` (child 1) |
| `show doctor` | → | `HandleDoctor` with `runtime` checks | `test/plugin/show-doctor-runtime-checks.ci` (child 1) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Child 1 closed | the inventory file lists every problem class of the tree with owner, state read, and check or gap; the gate refuses a commit that adds an uncovered problem site | none |
| AC-2 | Child 2 closed | the eight home-CPE classes each report through `ze doctor` and `show doctor` with cause and fix command | none |
| AC-3 | Every area child closed | every inventory row names the doctor check and code that reports its class, and `./le arch doctor-coverage report` lists no uncovered site; no row reads gap | a row whose state Linux cannot provide, named with evidence (`ai/META-DECISIONS.md`) |
| AC-4 | Any check in any child | states the cause in plain words and the exact fixing command on its own line, reaches every surface through the daemon, and has a test that goes red when the condition is not reported | none |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| per child | each child spec | each check | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none at umbrella level | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| per child | each child spec | operator runs `ze doctor` and reads the fault | |

## Files to Modify
- `internal/component/doctor/answer.go` - through child 1 (runtime phase rendering, local route)
- children name their own files

## Files to Create
- `spec-doctor-coverage-1-audit-and-gate`
- `plan/release/validation/spec-doctor-coverage-2-home-cpe-batch.md`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | `show doctor` exists; no new RPC at umbrella level |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | no new leaf |
| CLI commands/flags | N-A | children own any change |
| CLI grammar (keyword before value) | N-A | no new grammar |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | N-A | children own their tests |
| Pipe completeness | N-A | `show doctor` already pipes |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | the whole umbrella; children name each check |
| Prometheus counters/metrics | N-A | doctor is read-time |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP family |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the doctor feature page under `features/`, by each child |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none at umbrella level |
| 4 | API/RPC added/changed? | No | child 1 owns the rpc change |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/health-checks.md`, by each child |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | child 1 |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/doctor-and-health-checks.md`, by child 1 |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | No | the umbrella edits no source |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/health-checks.md` examples, by each child |

## Implementation Steps

1. **Phase: Wiring** -- child 1 phase 1 (runtime phase and local route)
2. **Phase: Audit and gate** -- child 1
3. **Phase: home-CPE batch** -- child 2 (depends on child 1 phase 1 for the `runtime` phase and the fix field)
4. **Phase: area children** -- each created by child 1 and run in order of its priority

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every inventory row reads covered or a named Linux limit |
| Correctness | no check reads healthy for state it could not see |
| Rule: user-facing-errors | each check prints the fixing command on its own line |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Inventory file | `plan/audits/doctor-problem-inventory.md` exists, no `gap` row once all children close |
| Gate | `./le arch doctor-coverage check` refuses a fixture adding an uncovered site |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Information exposure | `show doctor` answers through authz like every command; a check must not print secrets (keys, passwords) in its message |

### Failure Routing

| Failure | Route To |
|---------|----------|
| A child cannot read a class's state | back to child 1 inventory: the owner records the state first |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The home-CPE lab day showed the pattern: each fault was found by reading logs and kernel state by hand, and each had a readable state doctor could have reported.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Umbrella with children by door area | one spec for the whole tree | the spec doors gate and context economy: one context cannot read every door and hold the whole audit |
| Placement in `release/validation` | `post-release/engineering` | the owner rule makes doctor coverage part of operating the shipped NOS; Thomas decided on 2026-11-08 (child 1, O-3) that every area child is release work, P2 in `release/validation` |

## Known Limitations
- The area children do not exist yet; child 1 AC-6 creates them from the inventory.

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every child closed
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket
- [ ] Each child's targeted tests pass in the main checkout: the doctor package (`./le job run label doctor command go test -tags <tags> ./internal/component/doctor/...`, with the tags `./le repo changed scope print both` names) and `./le test functional plugin` for the `show doctor` `.ci` tests, plus the owning packages each child names

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** edited spec
- [ ] **Commit B:** `remove plan/release/validation/spec-doctor-coverage-0-umbrella.md` only
