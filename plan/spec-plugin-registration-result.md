# Spec: plugin-registration-result

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 6/6 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-31 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Symptom.** A module that tries to set itself up and fails is invisible. Ze has
no place where a module says what happened, so a user cannot ask why a feature
is absent. A survey on 2026-08-31 found 21 features that degrade silently or
log-only at startup. Seven are invisible even in `show log`, because
`fmt.Fprintf(os.Stderr, ...)` never reaches the slog ring that command reads.

**Why the existing surfaces cannot answer it.** `internal/core/health` stores a
probe rather than an outcome and runs it at read time, so a module that fails
before reaching its `health.Register` line produces no row at all, and absence
reads as "not enabled". `internal/core/report` records outcomes but is
negative-only, and no setup site raises onto it. `ze doctor` is an offline poll
that re-probes the environment and keeps no memory of a start.

**Goal.** Every registered module records, from its own `init()`, the outcome of
its setup: succeeded, soft failure, or hard failure. The engine checks that
registry once, at daemon start, and does not continue when a module recorded a
hard failure. A CLI command lists every module's recorded outcome.

**Owner decisions, 2026-08-31.** Given by Thomas at this spec's scope gate.

| # | Decision |
|---|----------|
| 1 | Not `ze doctor`. It is optional, so it cannot carry a verdict the daemon depends on |
| 2 | A new engine-side registry of registration results, consultable with a command |
| 3 | Three outcomes. Soft failure is for a module the daemon runs correctly without, and `memlock` is the exemplar |
| 4 | The module records what happened in its own `init()` |
| 5 | A hard failure stops the engine: it does not continue when the registry is checked |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/plugin-system.md` - the registration contract every module already follows
  → Constraint: one `registry.Registration` per `init()`; the registry is the only discovery path, and the core never imports a module package.
  → Decision: the result registry lives beside the plugin registry, so a module makes both declarations from the one `init()` it already has.
- [ ] `docs/architecture/doctor-and-health-checks.md` - the two inspection tiers this must not become a third of
  → Constraint: `ze doctor` is offline pre-start readiness; `show health` is an in-daemon live probe. Neither records a past event, which is the fact this spec adds.
- [ ] `docs/architecture/command-ownership.md` - who may own the new command
  → Constraint: a command stays in the central `show` schema only when it has no single removable owner. The plugin registry is a removable owner, so the node is carved out to `internal/component/plugin/yang/`, where `show plugins` already lives.
- [ ] `ai/patterns/cli-command.md` - the local-data command route
  → Decision: local-data, not RPC. The record is written by `init()`, so it is readable in any `ze` process without a daemon, exactly like `show plugins`.
  → Constraint: the page never names `MustRegisterLocalData`, `RegisterShape` or `RegisterColumns`, and its functional-test row is wrong for this route. The binding gate is `TestEveryLocalDataRegistrationHasAFunctionalCase`. This page is repaired by this spec.
- [ ] `ai/rules/no-layering.md` - the strongest hazard here
  → Constraint: `show health` renders name/status/reason over healthy|degraded|down, which maps one-to-one onto the new triple. The two are distinguished by TIME, not by shape: health evaluates now, this registry replays what `init()` recorded once. Both descriptions must say so, or the pair reads as duplication.
- [ ] `ai/rules/cli.md` - grammar and pipe completeness
  → Constraint: keyword before value, and the handler returns structured data so `| json`, `| yaml` and `| table` each render it. A handler that returns finished text fails `ApplyPipes`.

**Key insights:**
- Go runs every linked package's `init()` before `main()`, so the registry is complete before the daemon's first statement.
- The record and the registration are two writes keyed by module name, in either order. Neither may depend on the other, because within a package Go initialises files in filename order and that is not a contract a module author should have to know.
- A hard failure must stop the DAEMON and must never stop a CLI verb. The two are separated by which function they reach, not by a flag.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/registry/registry.go` - `Registration` is 30+ static declaration fields; `Register` validates and stores under `mu`; `All`/`Names` sort the map keys. No runtime state, no outcome, no timestamp.
- [ ] `internal/component/plugin/register.go` - `dataPlugins` returns `Map{"plugins": InternalPluginInfo()}`; registered with `MustRegisterLocalData("show plugins", ...)` plus `RegisterShape` and `RegisterColumns`. This is the template for the new command.
- [ ] `internal/component/plugin/server/startup.go` - `runPluginPhase` logs `plugin startup failed` and calls `rollbackStartupProcess`, which calls `pm.RemoveProcess` and `unmarkPluginLoaded`. The failed plugin is erased from every runtime surface.
- [ ] `cmd/ze/hub/main.go` - `run` is the choke point for `Run` and `RunWithManagedClient`; its first statement is the `storage.BlobStoreFrom` block. `logStartupFailure` writes the kmsg-visible half of a refusal; 27 sites use the stderr + `logStartupFailure` + `return 1` idiom.
- [ ] `internal/core/health/registry.go` - `Register(name, CheckFunc)` stores the func; `Check` invokes each at read time. 11 registrants.
- [ ] `internal/plugins/memlock/memlock_linux.go` - `init` records `lockedOctets, lockErr` in package vars that only its own doctor check reads.

**Behavior to preserve:**
- `show plugins`, `show health`, `show status` and `show system subsystem list` keep their current output. This spec adds a command; it changes none of them.
- Every existing `registry.Register` call site keeps working unchanged. Recording a result is additive and optional at the call site.
- A CLI verb never consults the result registry, so no CLI invocation can be refused by it.

**Behavior to change:**
- `memlock` stops holding its outcome in package vars and records it in the registry instead. Its doctor check and the `doctor-memlock-not-locked` diagnostic code are removed, because they answer for the wrong process and `ze doctor` was ruled out as the tier for this fact.
- `hub.run` gains a refusal at its first statement when any module recorded a hard failure.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Write: a module's `init()`, before `main()`, calling `registry.RecordSetup`.
- Read (daemon): `cmd/ze/hub/main.go -- run`, first statement.
- Read (user): `show module list`, from any `ze` process.

### Transformation Path
1. Module `init()` attempts its setup and calls `RecordSetup` with an outcome and a reason.
2. `RecordSetup` stores the result in a name-keyed map under the registry's existing mutex.
3. `run` calls `HardSetupFailures`; a non-empty answer refuses the start.
4. `dataModules` reads `SetupResults`, joined against `Names`, and renders one row per registered module.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Module → registry | A direct call at `init()`, in-process, no serialization | No |
| Registry → hub | A direct read at the first statement of `run` | No |
| Registry → CLI | Local-data handler, rendered through `ProcessPipes` | No |

### Integration Points
- `internal/component/plugin/registry` - the result map lives beside the plugin map, sharing `mu`.
- `internal/component/plugin/register.go` - the new local-data registration sits beside `dataPlugins`.
- `cmd/ze/hub/main.go` - one refusal, in the existing idiom.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every daemon path reaches `hub.run`, and no CLI verb does | Read of `cmd/ze/hub/main.go -- Run`, `RunWithManagedClient`, and `cmd/ze/dispatch.go -- dispatchMain` | A hard failure either fails to stop a daemon or wrongly stops a CLI verb | AC-5 functional test invoking a CLI verb with a hard failure recorded | unvalidated |
| A-2 | `RunWebOnly` bypasses `run` and therefore skips the check | Read of `cmd/ze/hub/main.go -- RunWebOnly`, which calls `webBuildStandalone` directly | `ze start --web-only` runs with a hard failure unreported | A deliberate decision row, not a defect: recorded in Known Limitations | unvalidated |
| A-3 | Recording from `init()` is order-independent of `Register` | Name-keyed map; neither write reads the other | A module whose files sort the other way records against nothing | AC-1 with the record written from a file sorting after `register.go` | unvalidated |
| A-4 | No existing module records a hard failure in the functional test environment | No call sites exist yet; only `memlock` (soft) migrates in this spec | The suite goes red wholesale | Run the functional suite after the memlock migration | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A fatal registry verdict at a startup barrier has been tried here before and reverted. `verifyAdvertisedClaims` (`internal/component/plugin/server/startup_claims.go`) calls itself "a guard that cannot deny and therefore must speak" after making it fatal turned 25 functional tests red | Functional suite goes red in tests unrelated to this spec | Only `memlock` (soft) migrates in this spec, so no hard failure is reachable in the suite. A module is promoted to hard only with its own evidence |
| R-2 | `show health` and `show module list` answer with the same three words over the same module names, and a reader cannot tell them apart | A reviewer, or a user, asks which one to look at | Both YANG descriptions state the temporal distinction, and the new command's description names `show health` explicitly |
| R-3 | The result is recorded but nothing forces a module to record it, so the registry stays as empty as `health.Register` is today | The command lists mostly `unknown` | The list is DERIVED from `registry.Names`, so a module that recorded nothing is visible as `unknown` rather than absent. `unknown` is the signal that the module owes a record |
| R-4 | 108 sites call `os.Exit(1)` from inside `init()` on a registration error, which kills CLI verbs too | Any of them fires | Out of scope and stated in Known Limitations: those fire on a malformed `Registration`, which is a programmer error, not an environment-dependent setup failure. The two must not be conflated |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A daemon refuses to start when it should run. That is worse than the silence being fixed, which is why only a soft exemplar migrates here |
| How is it reverted? | Single commit revert. No config migration, nothing on the wire, no persisted state |
| Who else touches this path? | `plan/spec-kernel-capability-gate.md` (status `ready`) plans the same refusal through the doctor registry. It must be amended to call this registry instead, or the two become parallel paths to one verdict |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze cli -c "show module list"` | → | `dataModules` (`internal/component/plugin/register.go`) | `TestShowModuleListReachesTheRegistry` |
| A module `init()` calling `RecordSetup` | → | `registry.RecordSetup` | `TestRecordSetupIsVisibleToSetupResults` |
| `hub.run` first statement | → | `registry.HardSetupFailures` | `TestRunRefusesOnHardSetupFailure` |
| `memlock` `init()` on a locked executable | → | `registry.RecordSetup` with `SetupSucceeded` | `TestMemlockRecordsItsOutcome` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A module records `SetupSucceeded` in its `init()` | `show module list` names the module with outcome `succeeded` and an empty reason |
| AC-2 | A module records a soft failure with a reason | `show module list` names it with outcome `soft-failure` and that reason, and the daemon starts normally |
| AC-3 | A module records a hard failure with a reason | The daemon refuses to start: a stderr line names the module and the reason, `logStartupFailure` records the same, and the exit code is 1 |
| AC-4 | A registered module records nothing | `show module list` names it with outcome `unknown`, never omits it |
| AC-5 | A hard failure is recorded and a CLI verb is invoked | The CLI verb runs and answers normally. No CLI invocation is refused by the registry |
| AC-6 | `memlock` `init()` fails to lock because RLIMIT_MEMLOCK is too small | It records a soft failure carrying the mlockexe error, and the daemon starts |
| AC-7 | `show module list \| json`, `\| yaml`, `\| table` | Each renders the same rows in its own format |
| AC-8 | Two modules record hard failures | The refusal names both, not only the first |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Asks why a feature is missing | `show module list` → `dataModules` → `SetupResults` → rendered rows | `show-module-list.ci` |
| 2 | Starts a daemon whose required module could not set up | `hub.run` → `HardSetupFailures` → stderr + `logStartupFailure` + exit 1 | `TestRunRefusesOnHardSetupFailure` |
| 3 | Runs a CLI verb on a host where a module hard-failed | `dispatchMain` → command registry, never reaching `hub.run` | `TestCLIVerbUnaffectedByHardSetupFailure` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRecordSetupIsVisibleToSetupResults` | `internal/component/plugin/registry/setup_test.go` | A recorded result is returned, with its outcome and reason intact | |
| `TestSetupResultsNamesEveryRegisteredModule` | `internal/component/plugin/registry/setup_test.go` | AC-4: a registered module that recorded nothing appears as `unknown` | |
| `TestRecordSetupIsOrderIndependent` | `internal/component/plugin/registry/setup_test.go` | A-3: recording before and after `Register` both work | |
| `TestHardSetupFailuresSelectsOnlyHard` | `internal/component/plugin/registry/setup_test.go` | AC-2 and AC-3: soft and unknown are not refusals | |
| `TestHardSetupFailuresNamesEveryFailure` | `internal/component/plugin/registry/setup_test.go` | AC-8 | |
| `TestSetupOutcomeZeroValueIsUnknown` | `internal/component/plugin/registry/setup_test.go` | The zero value is never a valid outcome (`docs/contributing/ze-go-style.md`) | |
| `TestRunRefusesOnHardSetupFailure` | `cmd/ze/hub/startup_gate_test.go` | AC-3: the refusal fires, in the existing idiom, before anything irreversible | |
| `TestRunProceedsOnSoftFailure` | `cmd/ze/hub/startup_gate_test.go` | AC-2 | |
| `TestMemlockRecordsItsOutcome` | `internal/plugins/memlock/memlock_linux_test.go` | AC-6: the outcome reaches the registry, not a package var | |
| `TestShowModuleListReachesTheRegistry` | `internal/component/plugin/register_test.go` | Wiring: the handler answers from the registry | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `SetupOutcome` | 0-3 | 3 (`SetupFailedHard`) | N/A (0 is `SetupUnknown`, a valid stored state and never a valid ARGUMENT to `RecordSetup`) | 4 rejected |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-module-list` | `test/plugin/show-module-list.ci` | A user lists every module and its recorded setup outcome | |
| `show-module-list-pipes` | `test/plugin/show-module-list.ci` | AC-7: the same rows through `\| json`, `\| yaml` and `\| table` | |

### Interop Tests (Scope: protocol)
N/A. Nothing wire-visible changes: no protocol behavior, no peer, no encoding.

## Files to Modify
- `internal/component/plugin/registry/registry.go` - the result map beside the plugin map, sharing `mu`
- `internal/component/plugin/register.go` - the `show module list` local-data registration beside `dataPlugins`
- `internal/component/plugin/yang/ze-plugin-show.yang` - the `show module list` node, with a description naming `show health` and the temporal distinction
- `cmd/ze/hub/main.go` - the refusal at the first statement of `run`
- `internal/plugins/memlock/memlock_linux.go` - record into the registry instead of package vars
- `internal/plugins/memlock/register.go` - remove the doctor check and its `Codes` entry
- `internal/core/diagnostic/codes.go` - remove `doctor-memlock-not-locked`
- `internal/plugins/memlock/memlock_linux_test.go` - replace the doctor-check tests with registry tests
- `internal/test/localdatacoverage/localdatacoverage.go` - the `Evidence` row the local-data gate requires
- `ai/patterns/cli-command.md` - repair the three gaps this spec's research found
- `docs/guide/command-reference.md` - the new command
- `docs/features.md` - the memlock row, which currently names the removed doctor code
- `docs/architecture/doctor-and-health-checks.md` - the third tier, and what distinguishes it from the two the page describes
- `plan/spec-kernel-capability-gate.md` - amend so its refusal calls this registry rather than doctor

**Design documents declared by the files above.** Each file's `// Design:` header
names a page, so each is judged here rather than at closure.

| Page | Declared by | Verdict |
|------|-------------|---------|
| `docs/architecture/api/architecture.md` | `internal/component/plugin/registry/registry.go` | UPDATE. The page describes the plugin registry, which gains a second map and three functions |
| `docs/architecture/api/commands.md` | `internal/component/plugin/register.go` | UPDATE. The page answers "where a command is served", and this adds a local-data command beside `show plugins` |
| `docs/architecture/hub-architecture.md` | `cmd/ze/hub/main.go` | UPDATE. The daemon gains a refusal at the first statement of `run`, which is boot order the page describes |
| `docs/features/ai-first.md` | `internal/core/diagnostic/codes.go` | UNAFFECTED. The page describes the diagnostic-code mechanism and names no individual code, so removing one code changes nothing it states |

## Files to Create
- `internal/component/plugin/registry/setup.go` - `SetupOutcome`, `SetupResult`, `RecordSetup`, `SetupResults`, `HardSetupFailures`
- `internal/component/plugin/registry/setup_test.go` - the unit tests above
- `cmd/ze/hub/startup_gate.go` - the refusal helper `run` calls
- `cmd/ze/hub/startup_gate_test.go` - its tests
- `test/plugin/show-module-list.ci` - the functional test

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/plugin/yang/ze-plugin-show.yang`, a `show module list` node beside `show plugins` |
| YANG validation constraints | N-A | The command takes no value; there is no leaf to constrain |
| YANG custom validators | N-A | No operator-supplied value |
| CLI commands/flags | Yes | `internal/component/plugin/register.go`, via `MustRegisterLocalData` |
| CLI grammar (keyword before value) | Yes | `show module list` is three keywords and no value; gated by `./le cli-grammar` |
| Editor autocomplete | N-A | Derived from the YANG tree; no dynamic value to complete |
| Functional test for new RPC/API | Yes | `test/plugin/show-module-list.ci`, plus the mandatory `localdatacoverage` Evidence row |
| Pipe completeness | Yes | `RenderLocalAnswer` with the real path, plus `RegisterShape` and `RegisterColumns` so `\| table` orders columns |
| Env var registration | N-A | No env var: the registry is populated by code, not configuration |
| Doctor check for runtime dependencies | N-A | This spec REMOVES a doctor check rather than adding one. The runtime dependency it covered is reported by the registry instead, which is decision 1 |
| Prometheus counters/metrics | No | Deliberate: a per-module gauge is a second declaration of the same fact. Reconsidered only if an operator asks to alert on it |
| BGP family surface (new SAFI / capability / attribute) | N-A | Nothing BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (a Module Setup Results row, and the memlock row loses the doctor code) |
| 2 | Config syntax changed? | N-A | No config |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | N-A | Local-data, not an RPC; no wire method, no snapshot row |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, how a plugin records its setup outcome |
| 6 | Has a user guide page? | Yes | `docs/guide/status.md` |
| 7 | Wire format changed? | N-A | Nothing on the wire |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/plugin/plugin-system.md`, the registration contract gains the result |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC governs this |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, the localdatacoverage Evidence row |
| 11 | Affects daemon comparison? | No | No competitor claim changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/doctor-and-health-checks.md`, the third tier |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | N-A | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation time: `./le spec citation anchors spec plan/spec-plugin-registration-result.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/features.md` memlock row names `doctor-memlock-not-locked`, which this spec removes |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register the entry points, write failing wiring tests
   - Tests: `TestShowModuleListReachesTheRegistry`, `TestRecordSetupIsVisibleToSetupResults`, `TestRunRefusesOnHardSetupFailure`
   - Files: `internal/component/plugin/registry/setup.go` (signatures only), `internal/component/plugin/register.go`, `internal/component/plugin/yang/ze-plugin-show.yang`, `cmd/ze/hub/startup_gate.go`
   - Verify: the command resolves and the refusal is reachable; every wiring test fails because the bodies are stubs
2. **Phase: the registry** -- `RecordSetup`, `SetupResults`, `HardSetupFailures`, and the outcome type whose zero value is `SetupUnknown`
   - Tests: the six `setup_test.go` tests
   - Files: `internal/component/plugin/registry/setup.go`
   - Verify: order-independence and the derived list both proven
3. **Phase: the refusal** -- `run` calls the gate at its first statement, in the 27-site idiom
   - Tests: `TestRunRefusesOnHardSetupFailure`, `TestRunProceedsOnSoftFailure`, `TestCLIVerbUnaffectedByHardSetupFailure`
   - Files: `cmd/ze/hub/main.go`, `cmd/ze/hub/startup_gate.go`
   - Verify: the refusal precedes `openStateOnlyStore`, the first irreversible act
4. **Phase: the command** -- `dataModules`, shape and columns, the functional test and the localdatacoverage Evidence row
   - Tests: `show-module-list.ci`, `TestEveryLocalDataRegistrationHasAFunctionalCase`
   - Files: `internal/component/plugin/register.go`, `test/plugin/show-module-list.ci`, `internal/test/localdatacoverage/localdatacoverage.go`
   - Verify: all three pipe operators render
5. **Phase: migrate memlock, remove the doctor check** -- the soft exemplar
   - Tests: `TestMemlockRecordsItsOutcome`
   - Files: `internal/plugins/memlock/*`, `internal/core/diagnostic/codes.go`
   - Verify: nothing still reads the removed diagnostic code
6. **Phase: documentation and the adjacent spec** -- every checklist row above, and the `spec-kernel-capability-gate` amendment
   - Files: the docs listed in Files to Modify, `plan/spec-kernel-capability-gate.md`
   - Verify: `./le doc wiring` and `./le spec citation` clean

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The refusal precedes every irreversible act in `run`, and names every failing module rather than the first |
| Correctness | A CLI verb reaches no code path that consults the registry |
| Naming | The new command's description states the temporal distinction from `show health`, and so does `show health`'s |
| Data flow | `RecordSetup` and `Register` are independent writes; neither reads the other |
| Rule: `ai/rules/no-layering.md` | The memlock doctor check is REMOVED, not left beside the registry record |
| Rule: `ai/rules/principles.md` | A module that recorded nothing is `unknown` and listed, never absent |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The registry answers for every registered module | `TestSetupResultsNamesEveryRegisteredModule` |
| The daemon refuses on a hard failure | `TestRunRefusesOnHardSetupFailure` |
| A CLI verb is never refused | `TestCLIVerbUnaffectedByHardSetupFailure` |
| The command renders in three formats | `test/plugin/show-module-list.ci` |
| The removed diagnostic code has no readers | `grep -rn doctor-memlock-not-locked` returns nothing |
| The adjacent spec no longer plans a second path | `grep -n RegisterDoctorCheck plan/spec-kernel-capability-gate.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The reason string is module-supplied and reaches CLI output. It must be rendered as data, never interpreted, and must not carry a secret: the recording site is responsible for what it puts in a reason |
| Denial of service | A module cannot record unboundedly: the map is keyed by module name, so a repeated record replaces rather than accumulates |
| Fail-open | `HardSetupFailures` returning empty must mean "nothing recorded a hard failure", never "the registry could not answer". There is no error path that could be mistaken for empty |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Functional suite goes red wholesale | R-1 has fired. Stop, report which module recorded hard, and do not weaken the gate |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A separate result registry beside the plugin registry | A field on `Registration`; a new `internal/core` package | A field would be written at `Register` time, which forces the setup to complete before registration. Within a package Go initialises files in filename order, so a `Registration` field silently depends on a module author's filenames. A separate name-keyed write is order-independent. A core package cannot be it: `internal/core` may not import the component tier, and the module set is the plugin registry's |
| Local-data command, not an RPC | An RPC like `show health` | The record is written by `init()`, so it exists in every `ze` process with no daemon running. That is precisely what local-data is for, it is the route `show plugins` took, and it costs no `wire-methods.snapshot` row |
| A distinct command rather than extending `show health` | Adding a column to `show health` | The facts differ in time: health evaluates a probe now, this replays what `init()` recorded once. Merging them would make `show health` re-render a past event as a present verdict, and a module that failed before registering has no probe to run |
| Remove the memlock doctor check | Keep it beside the registry record | It reports the doctor PROCESS's own lock, not the daemon's, which its own comment admits. Decision 1 rules the doctor tier out for this fact, and keeping both is the parallel path `ai/rules/no-layering.md` forbids |
| The 108 `os.Exit(1)` calls in `init()` stay | Retire them in this spec | They fire on a malformed `Registration`: a duplicate name or a nil `RunEngine`. That is a programmer error, always reproducible, never environment-dependent. A setup failure is the opposite. Conflating them would make a build defect and a host condition indistinguishable |

## Known Limitations
- Only `memlock` migrates here, and it is soft. No module records a hard failure yet, so AC-3 and AC-8 are proven by tests rather than by a shipped module. This is deliberate: R-1 records what happened the last time a startup registry verdict became fatal.
- `RunWebOnly` bypasses `hub.run`, so `ze start --web-only` does not consult the registry. It runs no protocol and programs nothing, so a hard failure cannot mislead a peer there.
- Components are out of scope. They have no registry to derive a module list from, so covering them means building one.
- The 21 features the survey found are not migrated. Each needs its own evidence for whether its failure is soft or hard, and that judgement is per-feature work.

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
- [ ] AC-1..AC-8 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

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

## Review Gate

### Round 1
| Scope | Lens | Findings | Severity |
|-------|------|----------|----------|

### Round 2
| Scope | Lens | Findings | Severity |
|-------|------|----------|----------|
