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

**Symptom.** A plugin that tries to set itself up and fails is invisible. Ze has
no place where a plugin says what happened, so a user cannot ask why a feature
is absent. A survey on 2026-08-31 found 21 features that degrade silently or
log-only at startup. Seven are invisible even in `show log`, because
`fmt.Fprintf(os.Stderr, ...)` never reaches the slog ring that command reads.

**Why the existing surfaces cannot answer it.** `internal/core/health` stores a
probe rather than an outcome and runs it at read time, so a plugin that fails
before reaching its `health.Register` line produces no row at all, and absence
reads as "not enabled". `internal/core/report` records outcomes but is
negative-only, and no setup site raises onto it. `ze doctor` is an offline poll
that re-probes the environment and keeps no memory of a start.

**Goal.** Every registered plugin records, from its own `init()`, the outcome of
its setup: succeeded, soft failure, or hard failure. The engine checks that
registry once, at daemon start, and does not continue when a plugin recorded a
hard failure. `show plugins` carries every plugin's recorded outcome on its own
row.

**Owner decisions, 2026-08-31.** Given by Thomas at this spec's scope gate.

| # | Decision |
|---|----------|
| 1 | Not `ze doctor`. It is optional, so it cannot carry a verdict the daemon depends on |
| 2 | A new engine-side registry of registration results, consultable with a command |
| 3 | Three outcomes. Soft failure is for a plugin the daemon runs correctly without, and `memlock` is the exemplar |
| 4 | The plugin records what happened in its own `init()` |
| 5 | A hard failure stops the engine: it does not continue when the registry is checked |
| 6 | One vocabulary. The registry says PLUGIN everywhere, as the registry it lives in already does |
| 7 | One command. `show plugins` carries the outcome on its own rows rather than a second command listing the same set |
| 8 | `memlock` keeps a doctor check, but a PRE-FLIGHT one: whether the host can lock the executable at all, which the registry cannot answer |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/plugin-system.md` - the registration contract every plugin already follows
  → Constraint: one `registry.Registration` per `init()`; the registry is the only discovery path, and the core never imports a plugin package.
  → Decision: the result registry lives beside the plugin registry, so a plugin makes both declarations from the one `init()` it already has.
- [ ] `docs/architecture/doctor-and-health-checks.md` - the inspection tiers this must not duplicate
  → Constraint: `ze doctor` is offline pre-start readiness; `show health` is an in-daemon live probe. Neither records a past event, which is the fact this spec adds.
  → Decision: the doctor tier stays, for the ENVIRONMENT question only. `memlock` shows the split: can this host lock the executable (doctor, before ze runs) against did this process lock it (the record, after).
- [ ] `docs/architecture/command-ownership.md` - who may own the command
  → Constraint: a command stays in the central `show` schema only when it has no single removable owner. The plugin registry is a removable owner, so the node is carved out to `internal/component/plugin/yang/`, where `show plugins` already lives.
- [ ] `ai/patterns/cli-command.md` - the local-data command route
  → Decision: local-data, not RPC. The record is written by `init()`, so it is readable in any `ze` process without a daemon, exactly like `show plugins`.
  → Constraint: the page never names `MustRegisterLocalData`, `RegisterShape` or `RegisterColumns`, and its functional-test row is wrong for this route. The binding gate is `TestEveryLocalDataRegistrationHasAFunctionalCase`. This page is repaired by this spec.
- [ ] `ai/rules/no-layering.md` - the strongest hazard here
  → Constraint: `show health` renders name/status/reason over healthy|degraded|down, which maps one-to-one onto the new triple. The two are distinguished by TIME, not by shape: health evaluates now, this registry replays what `init()` recorded once. Both descriptions must say so, or the pair reads as duplication.
  → Decision: the setup outcome is a COLUMN on `show plugins`, not a second command. `InternalPluginInfo` and `SetupResults` walk the same map, so two commands over one set is the duplication this rule forbids.
- [ ] `ai/rules/cli.md` - grammar and pipe completeness
  → Constraint: keyword before value, and the handler returns structured data so `| json`, `| yaml` and `| table` each render it. A handler that returns finished text fails `ApplyPipes`.
- [ ] `ai/rules/writing.md` - habit 1, synonym rotation
  → Constraint: one concept, one name. The registry's map, `Names()`, `All()` and `Registration.Name` all say PLUGIN, so the record beside them says PLUGIN too.

**Key insights:**
- Go runs every linked package's `init()` before `main()`, so the registry is complete before the daemon's first statement.
- The record and the registration are two writes keyed by plugin name, in either order. Neither may depend on the other, because within a package Go initialises files in filename order and that is not a contract a plugin author should have to know.
- A hard failure must stop the DAEMON and must never stop a CLI verb. The two are separated by which function they reach, not by a flag.
- The record set and the registration set are equal by construction, with one legitimate divergence: a plugin that RECORDED and then never completed its `Register` call. That is the loudest case, so the join keeps its row.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/registry/registry.go` - `Registration` is 30+ static declaration fields; `Register` validates and stores under `mu`; `All`/`Names` sort the map keys. No runtime state, no outcome, no timestamp.
- [ ] `internal/component/plugin/register.go` - `dataPlugins` returns `Map{"plugins": InternalPluginInfo()}`; registered with `MustRegisterLocalData("show plugins", ...)` plus `RegisterShape` and `RegisterColumns`. This is the command the outcome joins onto.
- [ ] `internal/component/plugin/server/startup.go` - `runPluginPhase` logs `plugin startup failed` and calls `rollbackStartupProcess`, which calls `pm.RemoveProcess` and `unmarkPluginLoaded`. The failed plugin is erased from every runtime surface.
- [ ] `cmd/ze/hub/main.go` - `run` is the choke point for `Run` and `RunWithManagedClient`; its first statement is the `storage.BlobStoreFrom` block. `logStartupFailure` writes the kmsg-visible half of a refusal; 27 sites use the stderr + `logStartupFailure` + `return 1` idiom.
- [ ] `internal/core/health/registry.go` - `Register(name, CheckFunc)` stores the func; `Check` invokes each at read time. 11 registrants.
- [ ] `internal/plugins/memlock/memlock_linux.go` - `init` records `lockedOctets, lockErr` in package vars that only its own doctor check reads.
- [ ] `internal/core/privilege/check_linux.go` - `effectiveCaps` reads `CapEff` from `/proc/self/status`. The pre-flight memlock check reads `CAP_IPC_LOCK` the same way, so root and a capability-granted container are not warned.

**Behavior to preserve:**
- `show health`, `show status` and `show system subsystem list` keep their current output. This spec changes none of them.
- Every existing `registry.Register` call site keeps working unchanged. Recording a result is additive and optional at the call site.
- A CLI verb never consults the result registry, so no CLI invocation can be refused by it.
- Every existing `show plugins` key keeps its name and its meaning. The command gains `outcome` and `reason`; it renames nothing.

**Behavior to change:**
- `memlock` stops holding its outcome in package vars and records it in the registry instead. Its OLD doctor check and the `doctor-memlock-not-locked` diagnostic code are removed, because they answered for the doctor process rather than the daemon.
- `memlock` gains a NEW doctor check that answers what the registry cannot: whether `RLIMIT_MEMLOCK` on this host can hold the executable at all, read before ze locks anything.
- `hub.run` gains a refusal at its first statement when any plugin recorded a hard failure.
- `show plugins` rows gain `outcome` and `reason`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Write: a plugin's `init()`, before `main()`, calling `registry.RecordSetup`.
- Read (daemon): `cmd/ze/hub/main.go -- run`, first statement.
- Read (user): `show plugins`, from any `ze` process.
- Read (pre-flight): `ze doctor`, for the environment question only.

### Transformation Path
1. Plugin `init()` attempts its setup and calls `RecordSetup` with an outcome and a reason.
2. `RecordSetup` stores the result in a name-keyed map under the registry's existing mutex.
3. `run` calls `HardSetupFailures`; a non-empty answer refuses the start.
4. `pluginRows` joins `SetupResults` against `InternalPluginInfo` and renders one row per plugin.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin → registry | A direct call at `init()`, in-process, no serialization | No |
| Registry → hub | A direct read at the first statement of `run` | No |
| Registry → CLI | Local-data handler, rendered through `ProcessPipes` | No |
| Host → doctor | `unix.Getrlimit`, `/proc/self/exe` and `/proc/self/status`, read at check time | No |

### Integration Points
- `internal/component/plugin/registry` - the result map lives beside the plugin map, sharing `mu`.
- `internal/component/plugin/register.go` - `dataPlugins` joins the record onto the rows it already answers with.
- `cmd/ze/hub/main.go` - one refusal, in the existing idiom.
- `internal/plugins/memlock/register.go` - one `DoctorCheckDef` on the existing `Registration`.

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
| A-3 | Recording from `init()` is order-independent of `Register` | Name-keyed map; neither write reads the other | A plugin whose files sort the other way records against nothing | AC-1 with the record written from a file sorting after `register.go` | unvalidated |
| A-4 | No existing plugin records a hard failure in the functional test environment | No call sites exist yet; only `memlock` (soft) migrates in this spec | The suite goes red wholesale | Run the functional suite after the memlock migration | unvalidated |
| A-5 | The size of `/proc/self/exe` is a FLOOR for the mapped size charged against `RLIMIT_MEMLOCK` | The loader maps at least the whole file, and adds bss and page padding the file does not carry | The pre-flight check warns on a host that could in fact lock | The check claims only the floor: a limit BELOW it cannot hold the executable, and a limit above it is left unjudged | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A fatal registry verdict at a startup barrier has been tried here before and reverted. `verifyAdvertisedClaims` (`internal/component/plugin/server/startup_claims.go`) calls itself "a guard that cannot deny and therefore must speak" after making it fatal turned 25 functional tests red | Functional suite goes red in tests unrelated to this spec | Only `memlock` (soft) migrates in this spec, so no hard failure is reachable in the suite. A plugin is promoted to hard only with its own evidence |
| R-2 | `show health` and the setup outcome answer with the same three words over the same plugin names, and a reader cannot tell them apart | A reviewer, or a user, asks which one to look at | Both YANG descriptions state the temporal distinction, and `show plugins` names `show health` explicitly |
| R-3 | The result is recorded but nothing forces a plugin to record it, so the registry stays as empty as `health.Register` is today | The rows list mostly `unknown` | The row set is DERIVED from the registry, so a plugin that recorded nothing is visible as `unknown` rather than absent. `unknown` is the signal that the plugin owes a record |
| R-4 | 108 sites call `os.Exit(1)` from inside `init()` on a registration error, which kills CLI verbs too | Any of them fires | Out of scope and stated in Known Limitations: those fire on a malformed `Registration`, which is a programmer error, not an environment-dependent setup failure. The two must not be conflated |
| R-5 | The pre-flight memlock check warns on a host that locks memory fine, because the rlimit does not apply to a privileged process | A false warning on every appliance, where ze runs as root | The check reads `CAP_IPC_LOCK` from `CapEff` and stays silent when it is held. `mlock(2)`: "a privileged process (CAP_IPC_LOCK) can lock as much memory as it likes" |
| R-6 | The join drops a plugin that recorded and never registered, because it has no `InternalPluginInfo` entry | Its row disappears, which is exactly the absence this spec removes | `pluginRows` iterates `SetupResults`, which is the union, and marks the unregistered row in its `description`. `TestShowPluginsRowsAgreeWithInternalPluginInfo` pins the one allowed divergence |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A daemon refuses to start when it should run. That is worse than the silence being fixed, which is why only a soft exemplar migrates here |
| How is it reverted? | Single commit revert. No config migration, nothing on the wire, no persisted state |
| Who else touches this path? | `plan/spec-kernel-capability-gate.md` (status `ready`) plans the same refusal through the doctor registry. It must be amended to call this registry instead, or the two become parallel paths to one verdict |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze cli -c "show plugins"` | → | `dataPlugins` and `pluginRows` (`internal/component/plugin/register.go`) | `TestShowPluginsCarriesTheRecordedSetupOutcome` |
| A plugin `init()` calling `RecordSetup` | → | `registry.RecordSetup` | `TestRecordSetupIsVisibleToSetupResults` |
| `hub.run` first statement | → | `registry.HardSetupFailures` | `TestRunRefusesOnHardSetupFailure` |
| `memlock` `init()` on a locked executable | → | `registry.RecordSetup` with `SetupSucceeded` | `TestMemlockRecordsItsOutcome` |
| `ze doctor` on a host whose limit is too small | → | `checkMemlockLimit` (`internal/plugins/memlock/doctor_linux.go`) | `TestMemlockDoctorCheckIsRegistered` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A plugin records `SetupSucceeded` in its `init()` | `show plugins` names the plugin with outcome `succeeded` and an empty reason |
| AC-2 | A plugin records a soft failure with a reason | `show plugins` names it with outcome `soft-failure` and that reason, and the daemon starts normally |
| AC-3 | A plugin records a hard failure with a reason | The daemon refuses to start: a stderr line names the plugin and the reason, `logStartupFailure` records the same, and the exit code is 1 |
| AC-4 | A registered plugin records nothing | `show plugins` names it with outcome `unknown`, never omits it |
| AC-5 | A hard failure is recorded and a CLI verb is invoked | The CLI verb runs and answers normally. No CLI invocation is refused by the registry |
| AC-6 | `memlock` `init()` fails to lock because RLIMIT_MEMLOCK is too small | It records a soft failure carrying the mlockexe error, and the daemon starts |
| AC-7 | `show plugins \| json`, `\| yaml`, `\| table` | Each renders the same rows in its own format, outcome and reason included |
| AC-8 | Two plugins record hard failures | The refusal names both, not only the first |
| AC-9 | A name recorded an outcome and never completed its `Register` call | `show plugins` keeps its row, and the row says its registration is absent |
| AC-10 | `RLIMIT_MEMLOCK` is below the size of `/proc/self/exe` and `CAP_IPC_LOCK` is not held | `ze doctor` warns under `doctor-memlock-rlimit-low`, naming both numbers and the remedy |
| AC-11 | `CAP_IPC_LOCK` is held, whatever the limit | The pre-flight check emits no diagnostic |
| AC-12 | The host cannot be read | The pre-flight check emits `doctor-memlock-rlimit-unknown` rather than passing silently |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Asks why a feature is missing | `show plugins` → `dataPlugins` → `pluginRows` → `SetupResults` → rendered rows | `test/parse/show-plugins.ci` |
| 2 | Starts a daemon whose required plugin could not set up | `hub.run` → `HardSetupFailures` → stderr + `logStartupFailure` + exit 1 | `TestRunRefusesOnHardSetupFailure` |
| 3 | Runs a CLI verb on a host where a plugin hard-failed | `dispatchMain` → command registry, never reaching `hub.run` | `TestCLIVerbUnaffectedByHardSetupFailure` |
| 4 | Prepares a host and asks whether ze will be able to lock its executable | `ze doctor` → `checkMemlockLimit` → `doctor-memlock-rlimit-low` | `TestMemlockCheckWarnsWhenTheLimitCannotHoldTheBinary` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRecordSetupIsVisibleToSetupResults` | `internal/component/plugin/registry/setup_test.go` | A recorded result is returned, with its outcome and reason intact | |
| `TestSetupResultsNamesEveryRegisteredPlugin` | `internal/component/plugin/registry/setup_test.go` | AC-4: a registered plugin that recorded nothing appears as `unknown` | |
| `TestRecordSetupIsOrderIndependent` | `internal/component/plugin/registry/setup_test.go` | A-3: recording before and after `Register` both work | |
| `TestHardSetupFailuresSelectsOnlyHard` | `internal/component/plugin/registry/setup_test.go` | AC-2 and AC-3: soft and unknown are not refusals | |
| `TestHardSetupFailuresNamesEveryFailure` | `internal/component/plugin/registry/setup_test.go` | AC-8 | |
| `TestSetupOutcomeZeroValueIsUnknown` | `internal/component/plugin/registry/setup_test.go` | The zero value is never a valid outcome (`docs/contributing/ze-go-style.md`) | |
| `TestSetupResultsKeepsARecordFromAnUnregisteredPlugin` | `internal/component/plugin/registry/setup_test.go` | AC-9 at the registry | |
| `TestRunRefusesOnHardSetupFailure` | `cmd/ze/hub/startup_gate_test.go` | AC-3: the refusal fires, in the existing idiom, before anything irreversible | |
| `TestRunProceedsOnSoftFailure` | `cmd/ze/hub/startup_gate_test.go` | AC-2 | |
| `TestMemlockRecordsItsOutcome` | `internal/plugins/memlock/memlock_linux_test.go` | AC-6: the outcome reaches the registry, not a package var | |
| `TestShowPluginsCarriesTheRecordedSetupOutcome` | `internal/component/plugin/register_test.go` | Wiring: the row carries what the plugin recorded | |
| `TestShowPluginsNamesAPluginThatRecordedNothing` | `internal/component/plugin/register_test.go` | AC-4 at the command | |
| `TestShowPluginsRendersTheOutcomeInEveryFormat` | `internal/component/plugin/register_test.go` | AC-7 | |
| `TestShowPluginsKeepsAPluginThatRecordedAndDidNotRegister` | `internal/component/plugin/register_test.go` | AC-9 at the command | |
| `TestShowPluginsRowsAgreeWithInternalPluginInfo` | `internal/component/plugin/register_test.go` | R-6: the two registry walks agree apart from the one allowed divergence | |
| `TestMemlockCheckIsSilentWhenTheLimitCoversTheBinary` | `internal/plugins/memlock/doctor_linux_test.go` | The check does not warn on a correct host | |
| `TestMemlockCheckWarnsWhenTheLimitCannotHoldTheBinary` | `internal/plugins/memlock/doctor_linux_test.go` | AC-10 | |
| `TestMemlockCheckIsSilentWhenCAPIPCLockIsHeld` | `internal/plugins/memlock/doctor_linux_test.go` | AC-11, R-5 | |
| `TestMemlockCheckSaysSoWhenItCannotReadTheHost` | `internal/plugins/memlock/doctor_linux_test.go` | AC-12 | |
| `TestReadMemlockEnvironmentReadsThisHost` | `internal/plugins/memlock/doctor_linux_test.go` | The real reader works, which an injected probe alone cannot prove | |
| `TestMemlockDoctorCheckIsRegistered` | `internal/plugins/memlock/doctor_linux_test.go` | The check reaches `ze doctor` under both codes | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `SetupOutcome` | 0-3 | 3 (`SetupFailedHard`) | N/A (0 is `SetupUnknown`, a valid stored state and never a valid ARGUMENT to `RecordSetup`) | 4 rejected |
| `RLIMIT_MEMLOCK` against the executable size | 0 to the largest uint64 | equal to the executable size is silent | one octet below the executable size warns | `RLIM_INFINITY` is silent |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-plugins` | `test/parse/show-plugins.ci` | A user lists every plugin and its recorded setup outcome | |
| `show-plugins-pipes` | `test/parse/show-plugins.ci` | AC-7: the same rows through `\| json`, `\| yaml` and `\| table` | |

### Interop Tests (Scope: protocol)
N/A. Nothing wire-visible changes: no protocol behavior, no peer, no encoding.

## Files to Modify
- `internal/component/plugin/registry/registry.go` - the result map beside the plugin map, sharing `mu`
- `internal/component/plugin/register.go` - the setup outcome joined onto the `show plugins` rows, and the two new columns
- `internal/component/plugin/yang/ze-plugin-show.yang` - the `show plugins` description, naming `show health` and the temporal distinction
- `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` - the `show health` description, naming the other side of that distinction
- `cmd/ze/hub/main.go` - the refusal at the first statement of `run`
- `internal/plugins/memlock/memlock_linux.go` - record into the registry instead of package vars
- `internal/plugins/memlock/register.go` - remove the old doctor check, declare the pre-flight one
- `internal/core/diagnostic/codes.go` - remove `doctor-memlock-not-locked`, add the two pre-flight codes
- `internal/plugins/memlock/memlock_linux_test.go` - replace the doctor-check tests with registry tests
- `internal/test/localdatacoverage/localdatacoverage.go` - the `show plugins` walk asserts an outcome on every row
- `internal/component/command/registry/local_data_functional_coverage_test.go` - the registration and evidence counts
- `test/ui/pipe-local-command.ci` - the coverage marker
- `ai/patterns/cli-command.md` - repair the three gaps this spec's research found
- `docs/guide/command-reference.md` - the two new keys on `show plugins`
- `docs/features.md` - the memlock row and the setup-results row
- `docs/architecture/doctor-and-health-checks.md` - the third tier, and the two-tier memlock split
- `docs/guide/docker.md` - the memlock row, which names the reporting surface
- `plan/spec-kernel-capability-gate.md` - amend so its refusal calls this registry rather than doctor

**Design documents declared by the files above.** Each file's `// Design:` header
names a page, so each is judged here rather than at closure.

| Page | Declared by | Verdict |
|------|-------------|---------|
| `docs/architecture/api/architecture.md` | `internal/component/plugin/registry/registry.go` | UPDATE. The page describes the plugin registry, which gains a second map and three functions |
| `docs/architecture/api/commands.md` | `internal/component/plugin/register.go` | UPDATE. The page answers "where a command is served", and `show plugins` now answers two questions on one row |
| `docs/architecture/hub-architecture.md` | `cmd/ze/hub/main.go` | UPDATE. The daemon gains a refusal at the first statement of `run`, which is boot order the page describes |
| `docs/architecture/doctor-and-health-checks.md` | `internal/plugins/memlock/doctor_linux.go` | UPDATE. The page owns the tier table, and the memlock split is the worked example of why a tier is not derivable from another |
| `docs/features/ai-first.md` | `internal/core/diagnostic/codes.go` | UNAFFECTED. The page describes the diagnostic-code mechanism and names no individual code |

## Files to Create
- `internal/component/plugin/registry/setup.go` - `SetupOutcome`, `SetupResult`, `RecordSetup`, `SetupResults`, `HardSetupFailures`
- `internal/component/plugin/registry/setup_test.go` - the unit tests above
- `cmd/ze/hub/startup_gate.go` - the refusal helper `run` calls
- `cmd/ze/hub/startup_gate_test.go` - its tests
- `test/parse/show-plugins.ci` - the functional test
- `internal/plugins/memlock/doctor_linux.go` - the pre-flight rlimit check
- `internal/plugins/memlock/doctor_linux_test.go` - its tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/plugin/yang/ze-plugin-show.yang`, the `show plugins` description |
| YANG validation constraints | N-A | The command takes no value; there is no leaf to constrain |
| YANG custom validators | N-A | No operator-supplied value |
| CLI commands/flags | Yes | `internal/component/plugin/register.go`, via `MustRegisterLocalData` |
| CLI grammar (keyword before value) | Yes | `show plugins` is two keywords and no value; gated by `./le cli-grammar` |
| Editor autocomplete | N-A | Derived from the YANG tree; no dynamic value to complete |
| Functional test for new RPC/API | Yes | `test/parse/show-plugins.ci`, plus the mandatory `localdatacoverage` Evidence row |
| Pipe completeness | Yes | `RenderLocalAnswer` with the real path, plus `RegisterShape` and `RegisterColumns` so `\| table` orders columns |
| Env var registration | N-A | No env var: the registry is populated by code, not configuration |
| Doctor check for runtime dependencies | Yes | `internal/plugins/memlock/doctor_linux.go`, the pre-flight rlimit probe, with two codes in `internal/core/diagnostic/codes.go`. It answers the ENVIRONMENT question the registry cannot, and replaces a check that answered for the wrong process |
| Prometheus counters/metrics | No | Deliberate: a per-plugin gauge is a second declaration of the same fact. Reconsidered only if an operator asks to alert on it |
| BGP family surface (new SAFI / capability / attribute) | N-A | Nothing BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (a Plugin Setup Results row, and the memlock row) |
| 2 | Config syntax changed? | N-A | No config |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, the two new `show plugins` keys |
| 4 | API/RPC added/changed? | N-A | Local-data, not an RPC; no wire method, no snapshot row |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, how a plugin records its setup outcome |
| 6 | Has a user guide page? | Yes | `docs/guide/status.md`, `docs/guide/docker.md` |
| 7 | Wire format changed? | N-A | Nothing on the wire |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/plugin/plugin-system.md`, the registration contract gains the result |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC governs this |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, the localdatacoverage Evidence row |
| 11 | Affects daemon comparison? | No | No competitor claim changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/doctor-and-health-checks.md`, the third tier and the memlock split |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | N-A | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation time: `./le spec citation anchors spec plan/spec-plugin-registration-result.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/features.md` and `docs/guide/docker.md` name the reporting surface |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register the entry points, write failing wiring tests
   - Tests: `TestShowPluginsCarriesTheRecordedSetupOutcome`, `TestRecordSetupIsVisibleToSetupResults`, `TestRunRefusesOnHardSetupFailure`
   - Files: `internal/component/plugin/registry/setup.go` (signatures only), `internal/component/plugin/register.go`, `internal/component/plugin/yang/ze-plugin-show.yang`, `cmd/ze/hub/startup_gate.go`
   - Verify: the command resolves and the refusal is reachable; every wiring test fails because the bodies are stubs
2. **Phase: the registry** -- `RecordSetup`, `SetupResults`, `HardSetupFailures`, and the outcome type whose zero value is `SetupUnknown`
   - Tests: the seven `setup_test.go` tests
   - Files: `internal/component/plugin/registry/setup.go`
   - Verify: order-independence and the derived list both proven
3. **Phase: the refusal** -- `run` calls the gate at its first statement, in the 27-site idiom
   - Tests: `TestRunRefusesOnHardSetupFailure`, `TestRunProceedsOnSoftFailure`, `TestCLIVerbUnaffectedByHardSetupFailure`
   - Files: `cmd/ze/hub/main.go`, `cmd/ze/hub/startup_gate.go`
   - Verify: the refusal precedes `openStateOnlyStore`, the first irreversible act
4. **Phase: the command** -- `pluginRows`, the two columns, the functional test and the localdatacoverage walk
   - Tests: `test/parse/show-plugins.ci`, `TestEveryLocalDataRegistrationHasAFunctionalCase`, `TestShowPluginsRowsAgreeWithInternalPluginInfo`
   - Files: `internal/component/plugin/register.go`, `test/parse/show-plugins.ci`, `internal/test/localdatacoverage/localdatacoverage.go`
   - Verify: all three pipe operators render, and the join keeps the unregistered recorder
5. **Phase: migrate memlock, replace the doctor check** -- the soft exemplar, and the pre-flight tier
   - Tests: `TestMemlockRecordsItsOutcome`, the five `doctor_linux_test.go` tests
   - Files: `internal/plugins/memlock/*`, `internal/core/diagnostic/codes.go`
   - Verify: nothing still reads the removed diagnostic code, and `ze doctor` warns under a lowered limit
6. **Phase: documentation and the adjacent spec** -- every checklist row above, and the `spec-kernel-capability-gate` amendment
   - Files: the docs listed in Files to Modify, `plan/spec-kernel-capability-gate.md`
   - Verify: `./le doc wiring` and `./le spec citation` clean

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The refusal precedes every irreversible act in `run`, and names every failing plugin rather than the first |
| Correctness | A CLI verb reaches no code path that consults the registry |
| Naming | One vocabulary. No surface of this feature says "module" for a plugin |
| Naming | `show plugins` states the temporal distinction from `show health`, and so does `show health` |
| Data flow | `RecordSetup` and `Register` are independent writes; neither reads the other |
| Rule: `ai/rules/no-layering.md` | The old memlock doctor check is REMOVED, and the new one answers a question the record cannot |
| Rule: `ai/rules/no-layering.md` | One command over one set: the outcome is a column on `show plugins`, not a second listing |
| Rule: `ai/rules/principles.md` | A plugin that recorded nothing is `unknown` and listed, never absent |
| Rule: `ai/rules/principles.md` | The pre-flight check says so when it cannot read the host, rather than passing |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The registry answers for every registered plugin | `TestSetupResultsNamesEveryRegisteredPlugin` |
| The daemon refuses on a hard failure | `TestRunRefusesOnHardSetupFailure` |
| A CLI verb is never refused | `TestCLIVerbUnaffectedByHardSetupFailure` |
| The command renders in three formats | `test/parse/show-plugins.ci` |
| One set, one command | `TestShowPluginsRowsAgreeWithInternalPluginInfo` |
| The removed diagnostic code has no readers | `grep -rn doctor-memlock-not-locked` returns nothing |
| No surface says "module" for a plugin | `grep -rn "show module list\|dataModules\|\.Module\b"` returns nothing outside vendor |
| The pre-flight check reaches `ze doctor` | `TestMemlockDoctorCheckIsRegistered`, plus `ze doctor` under a lowered `ulimit -l` |
| The adjacent spec no longer plans a second path | `grep -n RegisterDoctorCheck plan/spec-kernel-capability-gate.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The reason string is plugin-supplied and reaches CLI output. It must be rendered as data, never interpreted, and must not carry a secret: the recording site is responsible for what it puts in a reason |
| Denial of service | A plugin cannot record unboundedly: the map is keyed by plugin name, so a repeated record replaces rather than accumulates |
| Fail-open | `HardSetupFailures` returning empty must mean "nothing recorded a hard failure", never "the registry could not answer". There is no error path that could be mistaken for empty |
| Fail-open | The pre-flight check must not read a failed probe as a passing host. A zero limit beside a zero executable size compares equal, so the reader returns an error and the check reports it |
| Information disclosure | The pre-flight warning names the octet counts of the limit and the executable. Neither is a secret, and both are readable by any local process |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Functional suite goes red wholesale | R-1 has fired. Stop, report which plugin recorded hard, and do not weaken the gate |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A separate result registry beside the plugin registry | A field on `Registration`; a new `internal/core` package | A field would be written at `Register` time, which forces the setup to complete before registration. Within a package Go initialises files in filename order, so a `Registration` field silently depends on a plugin author's filenames. A separate name-keyed write is order-independent. A core package cannot be it: `internal/core` may not import the component tier, and the plugin set is the plugin registry's |
| Local-data, not an RPC | An RPC like `show health` | The record is written by `init()`, so it exists in every `ze` process with no daemon running. That is precisely what local-data is for, it is the route `show plugins` took, and it costs no `wire-methods.snapshot` row |
| The outcome is a column on `show plugins` | A second command listing the same set; a column on `show health` | `InternalPluginInfo` walks `registry.All` and `SetupResults` walks the same map, so two commands would list one set twice, which is what `ai/rules/no-layering.md` forbids. `show health` is the wrong home for the opposite reason: its facts differ in TIME, it evaluates a probe now, and a plugin that failed before registering has no probe to run |
| `SetupResults` decides the row set, not `InternalPluginInfo` | Iterating the registrations and looking each outcome up | The two agree by construction with one divergence: a plugin that recorded and never registered. Iterating the registrations would silently drop it, and that row is the loudest case this feature exists to show |
| Remove the OLD memlock doctor check, add a pre-flight one | Keep the old check; add nothing | The old check read `lockErr`, a process-local variable, so in the `ze doctor` process it reported that process's lock rather than the daemon's, and once the registry held the fact it was an exact duplicate. The new check answers a question the registry cannot: whether the HOST can lock the executable at all, before ze runs. Two tiers, neither derivable from the other |
| The pre-flight check compares against the file size of `/proc/self/exe` | Reading the mapped size from `/proc/self/maps` | The file size is a FLOOR, which is enough to prove a limit CANNOT hold the executable. Claiming the exact mapped size would need the loader's own arithmetic, and the check would then be wrong in the other direction on any host whose mapping differs |
| The 108 `os.Exit(1)` calls in `init()` stay | Retire them in this spec | They fire on a malformed `Registration`: a duplicate name or a nil `RunEngine`. That is a programmer error, always reproducible, never environment-dependent. A setup failure is the opposite. Conflating them would make a build defect and a host condition indistinguishable |

## Known Limitations
- Only `memlock` migrates here, and it is soft. No plugin records a hard failure yet, so AC-3 and AC-8 are proven by tests rather than by a shipped plugin. This is deliberate: R-1 records what happened the last time a startup registry verdict became fatal.
- `RunWebOnly` bypasses `hub.run`, so `ze start --web-only` does not consult the registry. It runs no protocol and programs nothing, so a hard failure cannot mislead a peer there.
- Components are out of scope. They have no registry to derive a plugin list from, so covering them means building one.
- The 21 features the survey found are not migrated. Each needs its own evidence for whether its failure is soft or hard, and that judgement is per-feature work.
- The pre-flight memlock check is Linux-only, like the rest of the plugin. On another platform there is no `RLIMIT_MEMLOCK` to read and no lock to take.

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
- [ ] AC-1..AC-12 all demonstrated
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
