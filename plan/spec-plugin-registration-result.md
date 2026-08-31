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
| Plugin → registry | A direct call at `init()`, in-process, no serialization | Yes -- `memlock_linux.go -- init` calls `RecordSetup`; `TestMemlockRecordsItsOutcome` reads the row back out of the registry rather than a package variable |
| Registry → hub | A direct read at the first statement of `run` | Yes -- `hardSetupFailure()` is the first statement of `run` (`cmd/ze/hub/main.go`), ahead of both `storage.BlobStoreFrom` and `openStateOnlyStore`; `TestRunRefusesOnHardSetupFailure` asserts no `database.zefs` exists after the refusal |
| Registry → CLI | Local-data handler, rendered through `ProcessPipes` | Yes -- `test/parse/show-plugins.ci` drives `ze cli -c "show plugins | json"` in a real process, PASS at 268/319 |
| Host → doctor | `unix.Getrlimit`, `/proc/self/exe` and `/proc/self/status`, read at check time | Yes -- `TestReadMemlockEnvironmentReadsThisHost` drives the real reader; the four verdict cases drive `memlockLimitDiagnostics` with an injected reader |

### Integration Points
- `internal/component/plugin/registry` - the result map lives beside the plugin map, sharing `mu`.
- `internal/component/plugin/register.go` - `dataPlugins` joins the record onto the rows it already answers with.
- `cmd/ze/hub/main.go` - one refusal, in the existing idiom.
- `internal/plugins/memlock/register.go` - one `DoctorCheckDef` on the existing `Registration`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | `TestTheSetupGateHasOneCallerAndItIsRun` parses every non-test file in the hub package with `go/ast`, so a build tag cannot hide a second caller, and asserts the one caller is `run`. `hardSetupFailure` is unexported. |
| No unintended coupling (components stay isolated) | Yes | The registry gains a map beside the one it already holds and shares its `mu`. No package imports memlock for a symbol; the composition root blank-imports it, which is the registration pattern, and memlock imports the registry, which is the direction the tier rule requires. |
| No duplicated functionality (extends existing, does not recreate) | Yes | `show plugins setup` existed for one commit and was DELETED in `a9c584c40` rather than kept beside `show plugins`; the outcome is a column on the rows that command already answered. |
| Zero-copy preserved where applicable (refs, not copies) | N/A | Nothing here touches wire encoding or a pool buffer. `SetupResults` returns a slice built once per command invocation, which is a control-plane read, not a per-event path. |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The row set is DERIVED from the registry union, so a plugin appears by recording, not by an edit to a list. `RecordSetup` is keyed by name, so no field is added to `Registration` and no plugin is spelled in a shared package. |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every daemon path reaches `hub.run`, and no CLI verb does | Read of `cmd/ze/hub/main.go -- Run`, `RunWithManagedClient`, and `cmd/ze/dispatch.go -- dispatchMain` | A hard failure either fails to stop a daemon or wrongly stops a CLI verb | AC-5 functional test invoking a CLI verb with a hard failure recorded | confirmed -- `TestTheSetupGateHasOneCallerAndItIsRun` (AST over the whole package) and `TestCLIVerbUnaffectedByHardSetupFailure`, which drives `command.ServeLocal` |
| A-2 | `RunWebOnly` bypasses `run` and therefore skips the check | Read of `cmd/ze/hub/main.go -- RunWebOnly`, which calls `webBuildStandalone` directly | `ze start --web-only` runs with a hard failure unreported | A deliberate decision row, not a defect: recorded in Known Limitations | confirmed -- `RunWebOnly` calls `webBuildStandalone` directly and never reaches `run`; it is the one of eight `hub.Run*` call sites that bypasses the gate, and it is recorded in Known Limitations |
| A-3 | Recording from `init()` is order-independent of `Register` | Name-keyed map; neither write reads the other | A plugin whose files sort the other way records against nothing | AC-1 with the record written from a file sorting after `register.go` | confirmed -- `TestRecordSetupIsOrderIndependent` (`internal/component/plugin/registry/setup_test.go`) |
| A-4 | No existing plugin records a hard failure in the functional test environment | No call sites exist yet; only `memlock` (soft) migrates in this spec | The suite goes red wholesale | Run the functional suite after the memlock migration | confirmed -- `./le functional parse` at 319 tests: `show-plugins` and `show-plugins-memlock` both PASS, and the seven failures are pre-existing and unrelated (`bcrypt-placeholder-rejected`, `cli-validate-config`, `config-dump-masks-bcrypt`, `geodns-config`, `iface-router-advertisement`, `ntp-config`, `prefix-per-family-parse`) |
| A-5 | The size of `/proc/self/exe` is a FLOOR for the mapped size charged against `RLIMIT_MEMLOCK` | The loader maps at least the whole file, and adds bss and page padding the file does not carry | The pre-flight check warns on a host that could in fact lock | The check claims only the floor: a limit BELOW it cannot hold the executable, and a limit above it is left unjudged | confirmed -- `TestMemlockCheckWarnsWhenTheLimitCannotHoldTheBinary` and its three sibling cases drive `memlockLimitDiagnostics` with an injected environment, so each verdict is forced rather than read off this host |

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

## Implementation Summary

### What Was Implemented
- `internal/component/plugin/registry/setup.go`: `SetupOutcome` (zero value `SetupUnknown`), `SetupResult`, `RecordSetup`, `SetupResults`, `HardSetupFailures`. A second name-keyed map beside the plugin map, under the same `mu`. `RecordSetup` panics with a `BUG:` prefix on an empty name, on `SetupUnknown`, and on a value outside the enumeration, so a row that says nothing cannot be written.
- `cmd/ze/hub/startup_gate.go`: `hardSetupFailure` computes the error naming every plugin that recorded a hard failure. It prints nothing and exits nothing; `run` (`cmd/ze/hub/main.go`) applies the refusal in the stderr plus `logStartupFailure` plus `return 1` idiom the other startup stages use.
- `internal/component/plugin/register.go`: `pluginRow`, `pluginRows` and `dataPlugins`. `pluginRows` iterates `SetupResults`, which is the union of the registered set and the recorded set, and looks each name up in `InternalPluginInfo`. A name that recorded and never registered keeps its row under `descriptionUnregistered`.
- `internal/component/plugin/yang/ze-plugin-show.yang`: the `show plugins` node, its `outcome` and `reason` columns, and the description that names `show health` and the temporal distinction. `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` states the same distinction from the `show health` side.
- `internal/plugins/memlock/memlock_linux.go`: `init` calls `lockexe.OnFault` and hands the error to `setupOutcome`, which answers the outcome and the reason. The package holds no outcome variable of its own.
- `internal/plugins/memlock/doctor_linux.go`: `checkMemlockLimit` and `memlockLimitDiagnostics`, the pre-flight probe. It is silent when `CAP_IPC_LOCK` is held or when the limit covers the executable, warns under `doctor-memlock-rlimit-low` when it cannot, and answers `doctor-memlock-rlimit-unknown` when `readMemlockEnvironment` returns an error.
- `test/parse/show-plugins.ci` and `test/parse/show-plugins-memlock.ci`, plus the `show plugins` walk in `internal/test/localdatacoverage/localdatacoverage.go`, which fails a row whose outcome is empty or `invalid`.

### Bugs Found/Fixed
- The new YANG module collided with the one `internal/core/ipc/yang` already declares: same module name `ze-plugin-cmd`, same namespace, same file key, so every YANG load failed with a duplicate-module error. Fixed in `388367016`, which moved module, namespace, file and embedded symbol to `ze-plugin-show`. Covered by `internal/component/plugin/yang/schema_test.go` and by `./le docvalid`.
- Every shell completion script extracted plugin names by matching `"Name":"` against JSON whose tag is `name` and whose encoder indents, so plugin-name completion had always been empty. The producer is the embedded script in `internal/plugins/completion/bash.go` and its siblings; fixed in `878c086b6`.
- Two acceptance criteria had tests that could not go red (AC-6 and AC-3). Both fixed in `ad1dfee03`, covered by `TestSetupOutcomeAnswersBothBranches` and `TestTheRefusalReachesTheLogAndNotOnlyStderr`.
- The round 1 fix for AC-3 read the oldest end of the log ring rather than this run's record, because `LogRing.Snapshot` (`internal/core/slogutil/ring.go`) answers newest-first. Fixed in `191d4a31b`, which selects the entry by timestamp.
- The `.ci` split dropped two expectations rather than moving them. Fixed in `191d4a31b`; `test/parse/show-plugins-memlock.ci` now carries all four renderings.

### Documentation Updates
- `docs/features.md`: a Plugin Setup Results row and a rewritten Memory Lock row, anchored to `setup.go -- RecordSetup, SetupResults, HardSetupFailures`, `register.go -- pluginRows`, `startup_gate.go -- hardSetupFailure`, `memlock_linux.go -- init` and `doctor_linux.go -- checkMemlockLimit`.
- `docs/architecture/doctor-and-health-checks.md`: the third tier in the tier table, and the memlock two-tier split as its worked example.
- `docs/architecture/plugin/plugin-system.md`: the registration contract gains `RecordSetup` and the outcome table.
- `docs/guide/plugins.md`, `docs/guide/status.md`, `docs/guide/docker.md`, `docs/guide/command-reference.md`, `docs/plugin-overview.md`, `docs/architecture/api/commands.md`, `docs/architecture/hub-architecture.md`, `docs/functional-tests.md` (the local-data command checklist), `ai/patterns/cli-command.md` (the three gaps the research found).
- `./le doc check verify` exits 1 over the whole tree, not over this work. Its source-anchor stage reports 26 findings and none names a file or a symbol this spec touched; its digest-anchor stage resolves all 3025 anchors. The failing stage is the generated site in the sibling `../gh-pages` checkout, where all 409 commands in the catalog carry the same finding.
- One page was WRONG and is repaired in the closure commit: `docs/architecture/api/architecture.md` said `module` six times for what `RecordSetup` calls a `plugin`. `a9c584c40` renamed the vocabulary across Go, YANG and `.ci` and missed the page that is the declared design document for `registry.go`. See the Mistake Log and the Review Gate.

### Deviations from Plan
- The spec planned one command. `6ff7353d7` shipped `show module list`, and `a9c584c40` deleted it and made the outcome a column on `show plugins` instead, on Thomas's instruction. Both the second command and the `module` spelling were removed.
- Files to Modify names `internal/component/plugin/yang/ze-plugin-show.yang`. The file was created as `ze-plugin-cmd.yang` in `878c086b6` and renamed in `388367016` after the namespace collision above.
- Files to Create names one functional test. Two exist: `show-plugins.ci` keeps the platform-neutral rows, and `show-plugins-memlock.ci` carries the memlock assertions behind `option=skip-os:value=darwin`, because memlock is linux-only and the single file failed rather than skipped on darwin.
- Round 1 recorded that `./le verify worktree` at closure would answer the open verification-debt rows. That gate is running in another process and its rows are still open. See Pre-Commit Verification.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | A test was written against already-working code and never had a red phase. `TestMemlockRecordsItsOutcome` switched on whichever branch this host took, so the AC-6 soft-failure reason was never asserted anywhere | The assertion would have passed against an `init()` that recorded only success | Review round 1, test-discrimination lens | `setupOutcome` extracted so both branches are driven by `TestSetupOutcomeAnswersBothBranches`, with the red phase forced |
| approach | The same shape again in the same diff: AC-3's `logStartupFailure` clause was unasserted, and the stderr assertion could not reach it because `fmt.Fprintln` writes the same two words | The test passed with `logStartupFailure` deleted | Review round 1 | `TestTheRefusalReachesTheLogAndNotOnlyStderr` asserts the slog rendering of the stage attribute, which only the logger produces |
| approach | The FIX for that finding was itself undiscriminating: it sliced `entries[before:]` from a ring that answers newest-first, so it read the oldest entries and passed only because a neighbouring test leaves a matching entry there | `LogRing.Snapshot` (`internal/core/slogutil/ring.go`) walks its index backwards from head | Review round 2, scoped to round 1's fixes | The entry is selected by timestamp and the miss stays fatal |
| approach | A `.ci` file was split and the ledger row in `test/weakened.md` claimed every expectation moved verbatim. Two did not move at all: a `yaml` block's `contains=memlock`, and a json `contains="memlock"` whose quotes are load-bearing | The split lost coverage while a committed record asserted it had not | Review round 2 | The four renderings restored in `show-plugins-memlock.ci`; the false claim corrected in the `191d4a31b` commit body rather than by rewriting a per-commit ledger row |
| approach | The spec's own Boundaries Crossed, Architectural Verification and Assumptions tables were left at `No` and `unvalidated` while the evidence existed | The work was done and the record did not say so | Review round 1, spec-conformance lens | Filled in `ad1dfee03` from evidence read at the producer |
| approach | A test depended on ambient `ze.log*` environment, so a developer shell with logging disabled or colored reddened it for a reason that is not the product | `getLogEnv` and `UseColor` (`internal/core/slogutil`) decide whether a record exists and whether the rendered attribute is contiguous | Review round 3 | `pinLogEnv` in `cmd/ze/hub/startup_gate_test.go` pins the three keys |
| approach | `a9c584c40` renamed `module` to `plugin` across Go, YANG and `.ci`, and left `docs/architecture/api/architecture.md` behind. The page is the declared design document for `registry.go`, and it now disagrees with `RecordSetup`, whose parameter is `plugin string` | The Critical Review row "no surface of this feature says module for a plugin" was false at one surface | This closure, Documentation Verified | FIXED in the closure commit. `ai/rules/documentation.md` puts the page edit in the work that broke it, and closure is the last chance this work has |
| escalation | Three of the eight findings across rounds 1 to 3 are one class: a test whose assertion cannot distinguish the behavior it names, written because the code already worked. Two of them were in the FIX for the first one | A forced red phase is the only thing that separates the two cases, and it was skipped each time | Rounds 1, 2 and 3 | `plan/journal/` already collects this class; the closure commit adds a row rather than a new rule |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Every registered plugin records the outcome of its setup from its own `init()` | Done | `registry.RecordSetup` (`internal/component/plugin/registry/setup.go`); `init` (`internal/plugins/memlock/memlock_linux.go`) | The mechanism is complete. One plugin records so far, which Known Limitations states |
| Three outcomes, soft failure for a plugin the daemon runs correctly without | Done | `SetupOutcome` and its four constants (`setup.go`) | `SetupUnknown` is a stored state, never a valid argument |
| Owner decision 1: not `ze doctor` | Done | `hardSetupFailure` (`cmd/ze/hub/startup_gate.go`) reads the plugin registry, not the doctor registry | |
| Owner decision 2: a new engine-side registry, consultable with a command | Done | `SetupResults` (`setup.go`), `dataPlugins` (`internal/component/plugin/register.go`) | |
| Owner decision 4: the plugin records in its own `init()` | Done | `init` (`internal/plugins/memlock/memlock_linux.go`) | |
| Owner decision 5: a hard failure stops the engine | Done | `run` (`cmd/ze/hub/main.go`) applies `hardSetupFailure` as its first statement | |
| Owner decision 6: one vocabulary, PLUGIN everywhere | Done | `a9c584c40` across Go, YANG and `.ci`, and the closure commit for `docs/architecture/api/architecture.md` | The page was the one surface `a9c584c40` missed. Found by the closure audit, repaired before the commit rather than reported |
| Owner decision 7: one command, the outcome on the `show plugins` rows | Done | `pluginRows` (`internal/component/plugin/register.go`) | `show module list` was deleted, not renamed |
| Owner decision 8: `memlock` keeps a PRE-FLIGHT doctor check | Done | `checkMemlockLimit` and `memlockLimitDiagnostics` (`internal/plugins/memlock/doctor_linux.go`) | |
| Owner decision 3 and the goal's third clause: `show plugins` carries every plugin's outcome on its own row | Done | `pluginRows` iterates the union | AC-4 and AC-9 are the two absence cases |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestShowPluginsCarriesTheRecordedSetupOutcome`; `test/parse/show-plugins-memlock.ci` | |
| AC-2 | Done | `TestHardSetupFailuresSelectsOnlyHard`, `TestRunProceedsOnSoftFailure` | |
| AC-3 | Done | `TestRunRefusesOnHardSetupFailure`, `TestTheRefusalReachesTheLogAndNotOnlyStderr` | The log half was unasserted until `ad1dfee03` and read the wrong ring end until `191d4a31b` |
| AC-4 | Done | `TestSetupResultsNamesEveryRegisteredPlugin`, `TestShowPluginsNamesAPluginThatRecordedNothing`, `show-plugins.ci` | |
| AC-5 | Done | `TestCLIVerbUnaffectedByHardSetupFailure`, `TestTheSetupGateHasOneCallerAndItIsRun` | |
| AC-6 | Done | `TestSetupOutcomeAnswersBothBranches`, `TestMemlockRecordsItsOutcome` | |
| AC-7 | Done | `TestShowPluginsRendersTheOutcomeInEveryFormat`; three pipe blocks in each `.ci` | |
| AC-8 | Done | `TestHardSetupFailuresNamesEveryFailure`, `TestRunRefusalNamesEveryHardFailure` | |
| AC-9 | Done | `TestSetupResultsKeepsARecordFromAnUnregisteredPlugin`, `TestShowPluginsKeepsAPluginThatRecordedAndDidNotRegister` | |
| AC-10 | Done | `TestMemlockCheckWarnsWhenTheLimitCannotHoldTheBinary` | |
| AC-11 | Done | `TestMemlockCheckIsSilentWhenCAPIPCLockIsHeld` | |
| AC-12 | Done | `TestMemlockCheckSaysSoWhenItCannotReadTheHost` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRecordSetupIsVisibleToSetupResults` | Done | `internal/component/plugin/registry/setup_test.go` | |
| `TestSetupResultsNamesEveryRegisteredPlugin` | Done | same | |
| `TestRecordSetupIsOrderIndependent` | Done | same | |
| `TestHardSetupFailuresSelectsOnlyHard` | Done | same | |
| `TestHardSetupFailuresNamesEveryFailure` | Done | same | |
| `TestSetupOutcomeZeroValueIsUnknown` | Done | same | |
| `TestSetupResultsKeepsARecordFromAnUnregisteredPlugin` | Done | same | |
| `TestRunRefusesOnHardSetupFailure` | Done | `cmd/ze/hub/startup_gate_test.go` | |
| `TestRunProceedsOnSoftFailure` | Done | same | |
| `TestMemlockRecordsItsOutcome` | Done | `internal/plugins/memlock/memlock_linux_test.go` | |
| `TestShowPluginsCarriesTheRecordedSetupOutcome` | Done | `internal/component/plugin/register_test.go` | |
| `TestShowPluginsNamesAPluginThatRecordedNothing` | Done | same | |
| `TestShowPluginsRendersTheOutcomeInEveryFormat` | Done | same | |
| `TestShowPluginsKeepsAPluginThatRecordedAndDidNotRegister` | Done | same | |
| `TestShowPluginsRowsAgreeWithInternalPluginInfo` | Done | same | |
| `TestMemlockCheckIsSilentWhenTheLimitCoversTheBinary` | Done | `internal/plugins/memlock/doctor_linux_test.go` | |
| `TestMemlockCheckWarnsWhenTheLimitCannotHoldTheBinary` | Done | same | |
| `TestMemlockCheckIsSilentWhenCAPIPCLockIsHeld` | Done | same | |
| `TestMemlockCheckSaysSoWhenItCannotReadTheHost` | Done | same | |
| `TestReadMemlockEnvironmentReadsThisHost` | Done | same | |
| `TestMemlockDoctorCheckIsRegistered` | Done | same | |
| Ten tests beyond the plan | Changed | the same four files | `TestRecordSetupRefusesAnOutcomeThatSaysNothing`, `TestRecordSetupReplacesRatherThanAccumulates`, `TestRecordSetupCarriesAnErrorTextVerbatim`, `TestRunRefusalNamesEveryHardFailure`, `TestTheRefusalReachesTheLogAndNotOnlyStderr`, `TestTheSetupGateHasOneCallerAndItIsRun`, `TestCLIVerbUnaffectedByHardSetupFailure`, `TestSetupOutcomeAnswersBothBranches`, `TestMemlockNeverRecordsAHardFailure`, `TestTheRecordedLockIsALockTheKernelHolds` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| The seven Files to Create | Done | All exist. `test/parse/show-plugins-memlock.ci` is an eighth, added by the darwin split |
| `internal/component/plugin/registry/registry.go` | Changed | The result map lives in the new `setup.go` beside it and shares `mu`, rather than in `registry.go` |
| `internal/component/plugin/register.go` | Done | |
| `internal/component/plugin/yang/ze-plugin-show.yang` | Done | Created as `ze-plugin-cmd.yang`, renamed after the namespace collision |
| `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` | Done | The `show health` container states the temporal distinction |
| `cmd/ze/hub/main.go` | Done | |
| `internal/plugins/memlock/memlock_linux.go`, `register.go` | Done | |
| `internal/core/diagnostic/codes.go` | Done | `doctor-memlock-not-locked` removed, the two pre-flight codes added |
| `internal/plugins/memlock/memlock_linux_test.go` | Done | |
| `internal/test/localdatacoverage/localdatacoverage.go` | Done | The walk fails a row whose outcome is empty or `invalid` |
| `internal/component/command/registry/local_data_functional_coverage_test.go` | Done | |
| `test/ui/pipe-local-command.ci` | Done | Carries `COVERED: show plugins [done]` |
| `ai/patterns/cli-command.md` | Done | Names `MustRegisterLocalData`, `RegisterShape`, `RegisterColumns` and `TestEveryLocalDataRegistrationHasAFunctionalCase` |
| The eight documentation pages | Done | Seven were correct at HEAD; `docs/architecture/api/architecture.md` said `module` and is repaired in the closure commit |
| `plan/spec-kernel-capability-gate.md` | Done | Carries the 2026-08-31 amendment |

### Audit Summary
- **Total items:** 60 (10 requirements, 12 acceptance criteria, 22 test rows, 16 file rows)
- **Done:** 57
- **Partial:** 0. The two the closure audit found (owner decision 6 and the documentation page row, both the same defect) were repaired in the closure commit rather than carried, so no item needs user approval and no scope was reduced
- **Skipped:** 0
- **Changed:** 3 (the result map in `setup.go` rather than `registry.go`; the second `.ci` file; ten tests beyond the plan). Each recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Every registered plugin records, from its own `init()`, the outcome of its setup | functional, over the real path | `test/parse/show-plugins-memlock.ci` seq 4 runs `ze cli -c "show plugins \| match memlock \| json"` in a real process and asserts `not:contains=unknown`. That assertion reds the moment recording stops while the list keeps rendering, which no assertion over the list alone can tell apart. PASS at 267/319 in `./le functional parse` run at closure |
| A plugin that fails before it registers is still visible, so absence never reads as "not built in" | functional and unit | `TestShowPluginsKeepsAPluginThatRecordedAndDidNotRegister` and `TestSetupResultsKeepsARecordFromAnUnregisteredPlugin` both PASS. `TestShowPluginsRowsAgreeWithInternalPluginInfo` pins that this is the ONE divergence between the two registry walks |
| The engine checks the registry once, at daemon start, and does not continue on a hard failure | unit, with the wiring proven over the AST | `TestRunRefusesOnHardSetupFailure` PASS: it asserts the exit code is 1 and that no `database.zefs` exists in the pinned config directory, so the refusal precedes the first irreversible act. `TestTheSetupGateHasOneCallerAndItIsRun` PASS: it parses every non-test file in `cmd/ze/hub` with `go/ast`, so a build tag cannot hide a second caller |
| A hard failure stops the DAEMON and never a CLI verb | negative test | `TestCLIVerbUnaffectedByHardSetupFailure` PASS: it records a hard failure and then drives `command.ServeLocal`, which answers normally. This is the goal that makes the feature usable, because the command that reports the fault has to work on the host that will not boot |
| `show plugins` carries every plugin's recorded outcome on its own row, in every format | functional, over the real path | `test/parse/show-plugins.ci` drives `ze cli -c` three times, once per pipe operator, and asserts `outcome` in each. PASS at 268/319. `internal/test/localdatacoverage` walks the JSON answer and fails a row whose outcome is empty or `invalid`, which is the assertion that stops the column from rendering as a blank cell |
| An operator can tell before ze runs whether the host can lock the executable | unit over an injected environment, plus one over the real host | The four verdict cases (`TestMemlockCheckIsSilentWhenTheLimitCoversTheBinary`, `...WarnsWhenTheLimitCannotHoldTheBinary`, `...IsSilentWhenCAPIPCLockIsHeld`, `...SaysSoWhenItCannotReadTheHost`) drive `memlockLimitDiagnostics` with a supplied environment, so each verdict is forced rather than read off this host. `TestReadMemlockEnvironmentReadsThisHost` drives the real reader. All PASS |
| Interop | not applicable | Nothing wire-visible changes: no protocol behavior, no peer, no encoding. `ai/rules/interop-and-goal-validation.md` exempts a feature with no protocol peer |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| None. The spec's `Deferral shard` field is `-` and no shard names it | n/a | `grep -rn 'plugin-registration-result' plan/deferrals/` returns nothing, exit 1. Control for the pattern per `ai/rules/evidence.md`: `grep -rln 'spec-' plan/deferrals/` returns `followup-rfc-enrollment.md`, `pki-full-chain.md`, `plugin-registers-pipe-operations.md` and more, so the corpus was asked and the zero is real. No shard to remove, and no foreign shard emptied by this work |

## Review Gate

### Round 1
Independent context, Opus 5, over the whole diff at `878c086b6`, `f8cb8eb3b`, `6ff7353d7`, `a9c584c40`. Eight lenses.

| Scope | Lens | Findings | Severity |
|-------|------|----------|----------|
| `memlock_linux_test.go -- TestMemlockRecordsItsOutcome` | test discrimination | AC-6 had no deterministic test: the assertion switched on whichever branch the host took, so on any host that can lock, the soft-failure reason went unasserted and would have passed against an `init()` recording only success | ISSUE, fixed in `ad1dfee03` |
| `test/parse/show-plugins.ci` | platform correctness | Four of five blocks asserted `contains=memlock`, but memlock is `//go:build linux`, so on darwin the test failed rather than skipping | ISSUE, fixed in `ad1dfee03` |
| Spec tables | spec conformance | Boundaries Crossed, Architectural Verification and A-1..A-5 all read `No`/`unvalidated` with empty evidence, though the evidence existed | ISSUE, fixed in `ad1dfee03` |
| `plan/verification-debt/64948495.md` | verification | Three rows say no full native verification covers this commit's Go | ISSUE, `./le verify worktree` run at closure |
| `startup_gate_test.go -- TestRunRefusesOnHardSetupFailure` | test discrimination | AC-3's `logStartupFailure` clause was unasserted, and the stderr assertion could not reach it: `fmt.Fprintln` carries the same two words | NIT, fixed in `ad1dfee03` |
| `ai/INDEX.md` | discoverability | No keyword routed "setup outcome", "plugin setup" or "show plugins" to the doctor page | NIT, fixed in `ad1dfee03` |
| Registry concurrency, startup gate, wiring, simplicity, naming, documentation | five lenses | Nothing. Every read and write under `mu` including the union walk; the gate is the first statement of `run` with an AST test pinning its one caller; every exported symbol has a non-test caller; no `module`/`plugin` drift; no page names a retired command | none |
| `SetupOutcome.String()` fifth return | simplicity, judged on request | KEEP. Not dead machinery: `localdatacoverage` fails the walk on `invalid` and `show-plugins.ci` asserts `not:contains` it, so it is a fail-closed guard with two consumers | none |

### Round 2
Independent context, Opus 5, scoped to `ad1dfee03` and the call sites its fixes touch. Two of the three findings are defects in the round 1 FIXES, which is what a second round exists to catch.

| Scope | Lens | Findings | Severity |
|-------|------|----------|----------|
| `startup_gate_test.go -- TestTheRefusalReachesTheLogAndNotOnlyStderr` | correctness of the fix | `LogRing.Snapshot` answers NEWEST-first, so slicing `entries[before:]` read the OLDEST entries, not this run's. It passed only because another test in the package leaves an `ERROR "startup failed"` as the oldest hub entry; a hub WARN there would have reddened it for the wrong reason | ISSUE, fixed |
| `show-plugins-memlock.ci`, `test/weakened.md` | losslessness of the split | The split dropped two expectations rather than moving them: the `yaml` block's `contains=memlock`, and the plain-json `contains="memlock"` whose QUOTES are load-bearing, since `expect=stdout:contains=` takes the literal rest of the line. The ledger row's claim that every expectation moved verbatim was false | ISSUE, fixed |
| Spec, Boundaries Crossed | evidence discipline | The round 1 fix introduced hand-typed line numbers into the `run` citation. `ai/rules/evidence.md` bans a line number no generator maintains, and the bare form evades `writeLineCitation` because its regex needs a filename before the colon, so nothing would have caught it | ISSUE, fixed |
| `show-plugins.ci` header | stale comment | `VALIDATES: AC-1, AC-2, AC-4 and AC-7` outlived the split; AC-1 and AC-2 moved with the memlock rows | NIT, fixed |
| Spec, No unintended coupling | accuracy | "No package imports memlock" is false as written: the composition root blank-imports it, which is the registration pattern the same cell praises | NOTE, fixed |
| Extraction behaviour, `.ci` idiom and discovery, sibling call sites, spec table citations, introduced defects | five checks | Nothing. `setupOutcome`'s reason text is byte-identical across the extraction; the new file's `option=` placement matches 34 siblings and needs no registration; `setupOutcome` has two callers; memlock is the only linux-only plugin any `.ci` asserts; every other filled cell is true at the producer | none |
| `TestSetupOutcomeAnswersBothBranches`, `TestTheRefusalReachesTheLogAndNotOnlyStderr` | test discrimination | Both discriminate. The log test does tell the two producers apart: `logStartupFailure` resolves its logger at call time, so it renders `stage="plugin setup"` into the test's replaced stderr, which `fmt.Fprintln` cannot produce | none |

### Round 3
Independent context, Opus 5, scoped to `191d4a31b` and the three fixes it made. No finding above NIT. All three NITs are fixed in the working tree and are uncommitted at the time this section was written.

| Scope | Lens | Findings | Severity |
|-------|------|----------|----------|
| `startup_gate_test.go -- TestTheRefusalReachesTheLogAndNotOnlyStderr` | ambient dependency | The test judged the shell it was started from as well as the product. `ze.log` and `ze.log.hub` reach `getLogEnv` (`internal/core/slogutil`), and a value that parses as disabled makes `Logger` answer with `discardHandler`, which writes no record; `ze.log.color` makes `UseColor` pick the color handler, which breaks the `stage="plugin setup"` substring into non-contiguous pieces. Either is a false RED | NIT, fixed by `pinLogEnv` (`cmd/ze/hub/startup_gate_test.go`) |
| Spec, Round 2 table | evidence discipline | The row REPORTING the removed line numbers quoted them, so the spec re-introduced what it recorded removing | NIT, fixed |
| `show-plugins-memlock.ci` header | accuracy | The comment said the moved expectations arrived "verbatim", but each block also gained a `contains=outcome` the original did not carry | NIT, fixed |
| The three round 2 fixes, re-read at the producer | correctness of the fixes | Nothing. The timestamp scan reads this run's record rather than a neighbour's; all four renderings are present in `show-plugins-memlock.ci` with the json quotes intact; the Boundaries Crossed cell names `storage.BlobStoreFrom` and `openStateOnlyStore` and carries no line number | none |

### Round 3 follow-up, found at closure
Two findings this closure raised while re-verifying the tables. Neither is fixed: this closure agent holds edit rights on the spec alone.

| Scope | Lens | Findings | Severity |
|-------|------|----------|----------|
| `docs/architecture/api/architecture.md` | naming, documentation | The page says `module` six times for this feature: the `HardSetupFailures()` table row, and five uses in the paragraph that opens "The setup record is a SECOND map". `a9c584c40` renamed the vocabulary across Go, YANG and `.ci` and left this page behind, and the page is the declared design document for `registry.go`. The producer's parameter is `plugin string` (`RecordSetup`, `internal/component/plugin/registry/setup.go`), so the page disagrees with the code its own anchor names. The Critical Review Checklist row "no surface of this feature says module for a plugin" was false here | ISSUE, fixed in the closure commit |
| `cmd/ze/hub/startup_gate_test.go` | stale comment | The uncommitted `pinLogEnv` helper was inserted between `pinConfigDir`'s doc comment and `pinConfigDir` itself, so that comment now documents `pinLogEnv`, and `pinConfigDir` has none. The `pinLogEnv` comment also said "the two logging variables" where the loop pins three keys | ISSUE, fixed in the closure commit: the comment is back on `pinConfigDir` and `pinLogEnv` says three |

| Field | Value |
|-------|-------|
| Artifact | Not recorded. `./le spec session review record` writes a hash-pinned verdict, and the two OPEN findings above make CLEAN false today. Record it after they are fixed |
| `./le spec session review check` | not run, for the same reason |
| Rounds | 3 complete, plus this closure pass. Under the five-pass cap, so no `rounds-reason` and no `owner-authorised` is owed |
| Reviewer lenses used | Round 1, eight lenses over the whole diff: test discrimination, platform correctness, spec conformance, verification, discoverability, registry concurrency, wiring, simplicity, naming, documentation. Round 2, scoped to round 1's fixes: correctness of the fix, losslessness, evidence discipline, stale comments, extraction behavior, `.ci` idiom, sibling call sites, spec citations, introduced defects, test discrimination. Round 3, scoped to round 2's fixes: ambient dependency, evidence discipline, accuracy, correctness of the fixes. Closure: files, acceptance criteria, wiring, assumptions, documentation, security |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | AC-6 had no deterministic test: the assertion switched on whichever branch the host took, so the soft-failure reason went unasserted | `internal/plugins/memlock/memlock_linux_test.go -- TestMemlockRecordsItsOutcome` | `ad1dfee03`: `setupOutcome` extracted, `TestSetupOutcomeAnswersBothBranches` drives both branches, red phase forced |
| 2 | ISSUE | Four of five blocks asserted a linux-only plugin, so the test failed rather than skipped on darwin | `test/parse/show-plugins.ci` | `ad1dfee03`: split into `show-plugins.ci` and `show-plugins-memlock.ci`, the second behind `option=skip-os:value=darwin` |
| 3 | ISSUE | Boundaries Crossed, Architectural Verification and A-1 to A-5 read `No` and `unvalidated` while the evidence existed | this spec | `ad1dfee03`: filled from evidence read at the producer |
| 4 | ISSUE | Three verification-debt rows say no full native verification covers this work's Go | `plan/verification-debt/64948495.md` | OPEN. `./le verify worktree` is running in another process; the rows clear through `./le commit debt-clear` when it exits 0 |
| 5 | ISSUE | The log-ring assertion sliced the oldest end of a newest-first ring and passed on a neighbouring test's entry | `cmd/ze/hub/startup_gate_test.go -- TestTheRefusalReachesTheLogAndNotOnlyStderr` | `191d4a31b`: the entry is selected by timestamp, the miss stays fatal, and a third assertion counts the plugin name twice in stderr |
| 6 | ISSUE | The `.ci` split dropped two expectations and a committed ledger row claimed it had not | `test/parse/show-plugins-memlock.ci`, `test/weakened.md` | `191d4a31b`: all four renderings restored; the false claim corrected in the commit body rather than by rewriting a per-commit ledger row |
| 7 | ISSUE | The round 1 fix introduced hand-typed line numbers into the Boundaries Crossed citation | this spec | `191d4a31b`: removed; the cell names the two symbols instead |
| 8 | ISSUE | `docs/architecture/api/architecture.md` said `module` for this feature, against `RecordSetup`, whose parameter is `plugin` | `docs/architecture/api/architecture.md` | Fixed in the closure commit: the `HardSetupFailures()` row and the five uses in the SECOND-map paragraph now say `plugin` |
| 9 | ISSUE | `pinConfigDir`'s doc comment had drifted onto `pinLogEnv`, and the `pinLogEnv` comment said two variables where the loop pins three | `cmd/ze/hub/startup_gate_test.go` | Fixed in the closure commit |

## Pre-Commit Verification

Every row below was re-verified by running a command in this closure context. Nothing is carried over from the audit above it.

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/plugin/registry/setup.go` | Yes | `ls -l`: 5881 octets, 2026-08-31 |
| `internal/component/plugin/registry/setup_test.go` | Yes | `ls -l`: 11875 octets |
| `cmd/ze/hub/startup_gate.go` | Yes | `ls -l`: 1478 octets |
| `cmd/ze/hub/startup_gate_test.go` | Yes | `ls -l`: 13981 octets |
| `test/parse/show-plugins.ci` | Yes | `ls -l`: 2283 octets |
| `test/parse/show-plugins-memlock.ci` | Yes | `ls -l`: 2364 octets. Not in Files to Create; added by the darwin split |
| `internal/plugins/memlock/doctor_linux.go` | Yes | `ls -l`: 6898 octets |
| `internal/plugins/memlock/doctor_linux_test.go` | Yes | `ls -l`: 6213 octets |
| `internal/component/plugin/yang/ze-plugin-show.yang` | Yes | `ls -l`: 1876 octets. `ze-plugin-cmd.yang` is absent, which is the rename in `388367016` |

### AC Verified (grep/test)
Two commands produced this evidence. `go test -tags ze_core,ze_bgp -count=1 -v` over the four packages, and `./le functional parse` at 319 tests.

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | A recorded success renders as an outcome on the row | `--- PASS: TestShowPluginsCarriesTheRecordedSetupOutcome (0.00s)`; `1.1s 267/319 PASS 267 show-plugins-memlock` |
| AC-2 | A soft failure carries its reason and the daemon starts | `--- PASS: TestHardSetupFailuresSelectsOnlyHard (0.00s)`, `--- PASS: TestRunProceedsOnSoftFailure (0.00s)` |
| AC-3 | The refusal names the plugin on stderr and in the log, and exits 1 | `--- PASS: TestRunRefusesOnHardSetupFailure (0.00s)`, `--- PASS: TestTheRefusalReachesTheLogAndNotOnlyStderr (0.00s)` |
| AC-4 | A registered plugin that recorded nothing is `unknown`, never omitted | `--- PASS: TestSetupResultsNamesEveryRegisteredPlugin (0.00s)`, `--- PASS: TestShowPluginsNamesAPluginThatRecordedNothing (0.00s)`; `show-plugins.ci` asserts `contains="unknown"` and PASSES at 268 |
| AC-5 | A CLI verb is never refused | `--- PASS: TestCLIVerbUnaffectedByHardSetupFailure (0.00s)`, `--- PASS: TestTheSetupGateHasOneCallerAndItIsRun (0.02s)` |
| AC-6 | `memlock` records a soft failure carrying the mlockexe error | `--- PASS: TestSetupOutcomeAnswersBothBranches (0.00s)`, `--- PASS: TestMemlockRecordsItsOutcome (0.00s)` |
| AC-7 | The three pipe operators each render the rows | `--- PASS: TestShowPluginsRendersTheOutcomeInEveryFormat (0.00s)`; both `.ci` files drive `\| json`, `\| yaml` and `\| table` in separate blocks and PASS |
| AC-8 | The refusal names both failures | `--- PASS: TestHardSetupFailuresNamesEveryFailure (0.00s)`, `--- PASS: TestRunRefusalNamesEveryHardFailure (0.00s)` |
| AC-9 | A recorder that never registered keeps its row | `--- PASS: TestSetupResultsKeepsARecordFromAnUnregisteredPlugin (0.00s)`, `--- PASS: TestShowPluginsKeepsAPluginThatRecordedAndDidNotRegister (0.00s)` |
| AC-10 | `ze doctor` warns under `doctor-memlock-rlimit-low` | `--- PASS: TestMemlockCheckWarnsWhenTheLimitCannotHoldTheBinary (0.00s)`; `grep -rn doctor-memlock-rlimit` names both codes in `internal/core/diagnostic/codes.go` |
| AC-11 | `CAP_IPC_LOCK` silences the check | `--- PASS: TestMemlockCheckIsSilentWhenCAPIPCLockIsHeld (0.00s)`; `memlockLimitDiagnostics` returns nil on `host.PrivilegedLock` before it compares anything |
| AC-12 | An unreadable host answers `doctor-memlock-rlimit-unknown` | `--- PASS: TestMemlockCheckSaysSoWhenItCannotReadTheHost (0.00s)`; `readMemlockEnvironment` returns an error rather than a zero environment, because a zero limit beside a zero size compares equal |

### Wiring Verified (end-to-end)
Each `.ci` was READ, not inferred from its name.

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze cli -c "show plugins"` | `test/parse/show-plugins.ci` | Yes. Three `cmd=foreground` blocks each exec `ze cli -c "show plugins \| <op>"` in a real process, one per operator, and assert `outcome` and a known row name. The json block also asserts `not:contains="invalid"`, which is the fail-closed guard on `SetupOutcome.String()`. PASS at 268/319 |
| A plugin `init()` calling `RecordSetup` | `test/parse/show-plugins-memlock.ci` | Yes. seq 4 runs `show plugins \| match memlock \| json` and asserts `not:contains=unknown`, so the row must carry what `init()` recorded. Behind `option=skip-os:value=darwin`, because `register.go` and `memlock_linux.go` both carry `//go:build linux`. PASS at 267/319 |
| `hub.run` first statement | no `.ci`; unit plus AST | Yes. `hardSetupFailure()` is the first statement of `run` (`cmd/ze/hub/main.go`), ahead of the `storage.BlobStoreFrom` block. `TestTheSetupGateHasOneCallerAndItIsRun` parses every non-test file in the package with `go/ast` and PASSES, so a build tag cannot hide a second caller. A `.ci` cannot drive this: the refusal needs a plugin that records a hard failure, and none does |
| `memlock` `init()` on a locked executable | `test/parse/show-plugins-memlock.ci` | Yes, as above, plus `TestTheRecordedLockIsALockTheKernelHolds`, which compares the octets `init()` reported against `VmLck` in `/proc/self/status` |
| `ze doctor` on a host whose limit is too small | no `.ci`; unit | Yes. `TestMemlockDoctorCheckIsRegistered` PASSES over both codes, so the check reaches the doctor registry. The verdict itself is driven by the four injected-environment cases, because the running host's rlimit cannot be lowered from inside the test |
| Local-data registration contract | `test/ui/pipe-local-command.ci` | Yes. It carries `COVERED: show plugins [done]`, and `TestEveryLocalDataRegistrationHasAFunctionalCase` PASSES (1.88s), deriving every production registration from the Go AST |

### Assumptions Resolved
Each re-verified in this context rather than copied from the table above.

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `grep` over `cmd/ze/hub/*.go` outside tests finds exactly two calls to `run`, both in `Run` and `RunWithManagedClient`. `TestTheSetupGateHasOneCallerAndItIsRun` PASSES over the AST, and `TestCLIVerbUnaffectedByHardSetupFailure` PASSES driving `command.ServeLocal` with a hard failure recorded |
| A-2 | confirmed | `RunWebOnly` (`cmd/ze/hub/main.go`) guards on `webBuildStandalone` being nil and returns its result directly. It never calls `run`, so it never reaches the gate. Recorded in Known Limitations, not a defect |
| A-3 | confirmed | `--- PASS: TestRecordSetupIsOrderIndependent (0.00s)`. The producer supports it: `RecordSetup` writes `setupResults[plugin]` and reads nothing from `plugins`, and `SetupResults` unions the two maps at read time |
| A-4 | confirmed | `./le functional parse` at closure: 319 tests, `show-plugins` PASS at 268 and `show-plugins-memlock` PASS at 267. Seven fail and all seven are pre-existing and unrelated: `bcrypt-placeholder-rejected`, `cli-validate-config`, `config-dump-masks-bcrypt`, `geodns-config`, `iface-router-advertisement`, `ntp-config`, `prefix-per-family-parse`. No plugin records a hard failure, so no test could go red for one |
| A-5 | confirmed | The floor claim is what the code makes: `memlockLimitDiagnostics` returns nil when `host.LimitOctets >= host.ExecutableOctets`, so a limit above the file size is left unjudged and only a limit below it warns. The comment on `memlockEnvironment.ExecutableOctets` states the same. All four verdict cases PASS with an injected environment, and `TestReadMemlockEnvironmentReadsThisHost` PASSES over the real reader |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| 1. New user-facing feature (`docs/features.md`) | The Plugin Setup Results row and the Memory Lock row both read back against `RecordSetup`, `pluginRows`, `hardSetupFailure`, `init` and `checkMemlockLimit`. Each claim holds at the producer, including "names every failing plugin rather than the first", which `hardSetupFailure` does by looping `HardSetupFailures` | Yes |
| 3. CLI command (`docs/guide/command-reference.md`) | The `show plugins` section documents the pipe operators and carries anchors to `dataPlugins`, `pluginRows` and `SetupResults`. Its example `ze show plugins \| ze pipe match memlock` matches what `show-plugins-memlock.ci` seq 4 actually runs | Yes |
| 5. Plugin added or changed (`docs/guide/plugins.md`) | The page states that the two writes are keyed by plugin name and neither reads the other, which `RecordSetup` and `Register` confirm, and names `memlock` as the worked example | Yes |
| 6, 15. User guide and inventory (`docs/guide/status.md`, `docs/guide/docker.md`, `docs/plugin-overview.md`) | `status.md` says the other plugins record nothing yet and list as `unknown`, which is what `SetupResults` produces. `docker.md` names `--ulimit memlock=-1` or `--cap-add IPC_LOCK` and `doctor-memlock-rlimit-low`, both of which `memlockLimitDiagnostics` produces | Yes |
| 8. Plugin SDK contract (`docs/architecture/plugin/plugin-system.md`) | The outcome table matches the four `SetupOutcome` constants, and its `SetupUnknown` row says "a stored state, never a valid ARGUMENT", which `RecordSetup`'s panic enforces | Yes |
| 10. Test infrastructure (`docs/functional-tests.md`) | `git blame` puts the "Adding a local-data command" section at `6ff7353d7`, one of this spec's commits. Its four rows match the four places `TestEveryLocalDataRegistrationHasAFunctionalCase` and `TestLocalDataCoverageEvidenceIsNonVacuousAndComplete` read, and both PASS | Yes |
| 12. Internal architecture (`docs/architecture/doctor-and-health-checks.md`) | The tier table's third row and the memlock split both read back against `checkMemlockLimit` and `init`. The claim that an unreadable host answers `doctor-memlock-rlimit-unknown` rather than passing holds at `memlockLimitDiagnostics` | Yes |
| 12. Internal architecture (`docs/architecture/api/architecture.md`) | The page said `module` six times for a feature whose producer parameter is `plugin`. Verified against `RecordSetup` (`internal/component/plugin/registry/setup.go`), then repaired in the closure commit | Yes |
| 2, 4, 7, 9, 11, 13, 14. Config, RPC, wire, RFC, comparison, route metadata, metrics | No update needed. `MustRegisterLocalData` registers no RPC, so no `wire-methods.snapshot` row exists to change; nothing is written to a socket; no RFC governs a local inspection command; `grep -rn 'doctor-memlock-not-locked'` over Go, Markdown, YANG and `.ci` returns only this spec's own three prose mentions of the removal, so the retired code has no readers | Yes |
| Source anchors | `./le doc check verify` source-anchor stage: 2262 code paths across 563 packages, 26 anchor symbols not declared where named. None of the 26 names a file or a symbol this spec touched. Digest anchors: 3025 checked, all resolve | Yes |
| Generated site | `./le doc check verify` exits 1 on `../gh-pages/reference/command-equivalents/index.html`. All 409 commands in the catalog carry the identical finding, `show plugins` among them, so the whole published surface is stale in the sibling checkout. Not this spec's, and not repairable from this tree | Not this work |
| Spec citations | `./le spec citation` exits 0: 199 specs, 51 baselined dangling, 10 line-token WARN, none of them this spec. `grep -rn 'plan/spec-plugin-registration-result' plan/` finds no path citation. `plan/spec-kernel-capability-gate.md` cites the bare stem, which `speccitation.Scan` does not resolve, so commit B leaves nothing dangling | Yes |

### Security Verified
| Check | Fresh evidence |
|-------|----------------|
| Input validation | The reason is plugin-supplied and reaches CLI output as a `pluginRow` field rendered by `RenderLocalAnswer`, so it is encoded as data by the pipe layer and never interpreted. `RecordSetup`'s doc comment states that a recording site MUST NOT put a secret in it |
| Denial of service | `RecordSetup` assigns `setupResults[plugin]`, so a repeated record replaces. `TestRecordSetupReplacesRatherThanAccumulates` PASSES |
| Fail-open, `HardSetupFailures` | It reads a package-level map that is always present and has no error path, so an empty answer can only mean no plugin recorded a hard failure. Confirmed by reading the function |
| Fail-open, the pre-flight check | `readMemlockEnvironment` returns an error rather than a zero environment on each of its three reads, and `memlockLimitDiagnostics` turns that error into `doctor-memlock-rlimit-unknown`. `TestMemlockCheckSaysSoWhenItCannotReadTheHost` PASSES |
| Information disclosure | The warning carries two octet counts, both readable by any local process through `getrlimit` and `stat` |
| Concurrency | Every read and write of `setupResults` is under the registry's `mu`, `SetupResults` and `HardSetupFailures` take the read lock, and `RecordSetup` takes the write lock. Confirmed by reading `setup.go` |

### Verification Debt
`./le verify worktree` is running in another process and holds golangci-lint for the tree, so no lint flavor could run here and this closure did not attempt one. Twelve rows in `plan/verification-debt/64948495.md` and three in `3201c77e.md` name this work's commits by subject line and stay `open`. They clear through `./le commit debt-clear` once that gate exits 0, and `./le commit create ... push` refuses while any row is open, which is where the debt is enforced.

## Core Insight

The three inspection tiers are not separated by their subject. They are separated by WHEN the answer is computed.

`ze doctor` computes at probe time, before ze runs, and answers about the ENVIRONMENT. `show health` computes at read time, inside the daemon, and answers about NOW. The setup record computes once, in a plugin's own `init()` before `main()`, and answers about a PAST event that nothing can re-run.

`memlock` is the worked example because it has a fact in two of the three tiers, and the two are not derivable from each other. Whether this host's `RLIMIT_MEMLOCK` can hold the executable is a question ze can answer before it starts. Whether this process locked it is a question only the process that tried can answer, and only at the moment it tried. The deleted doctor check read a process-local variable, which made it answer the second question in the first tier's process, so under `ze doctor` it reported the operator's shell rather than the daemon.

That is also why `show health` was the wrong home for the outcome, and why the shape test (name, status, reason over a three-value enumeration) misleads. Two surfaces with the same shape are duplication only when they answer at the same time. A plugin that fails before it reaches its `health.Register` line has no probe to run, so `show health` has no row to show, and the absence reads as "not enabled". That absence is the whole defect this feature removes.
