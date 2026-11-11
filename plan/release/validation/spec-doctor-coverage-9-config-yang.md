# Spec: doctor-coverage-9-config-yang

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | config |
| Priority | P2 |
| Release reason | `ai/META-DECISIONS.md`, "Goal and scope": `ze doctor` reports every problem an operator can hit, each with its cause and the command that fixes it; O-3 of `spec-doctor-coverage-1-audit-and-gate` makes every area child release work |
| RFC gaps | - |
| Depends | `spec-doctor-coverage-1-audit-and-gate`: the `runtime` doctor phase, `Diagnostic.Fix`, the `doctor-recent-events` check (`internal/component/doctor/checks_recent_events.go`) and the doctor-coverage gate, landed in commits ce58dddf53 and fc96556110. `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` phase P0a (the cross-cutting codes `doctor-internal-defect`, `doctor-plugin-stopped`, `doctor-plugin-config-unapplied`, the event-code registration, the raise-carries-its-fix convention) and its AC-9 and AC-22 checks, cited by name and never rebuilt here. `plan/release/validation/spec-doctor-coverage-11-hub-engine.md`: its `hub-runtime` check reports the confirm window (AC-23) and declares `doctor-config-reload-refused` and `config-plugin-verify` (AC-34; `config-plugin-verify` already has metadata in `codes.go` and is already declared by this component's `config-semantics`, and a second check may declare it, A-3); this spec registers the confirm codes it uses and duplicates neither check. `plan/release/validation/spec-doctor-coverage-4-api-surfaces.md` AC-22 carries this spec's `doctor-config-password-weak` on the REST, gRPC and gNMI commit advisory line |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent: `plan/release/validation/spec-doctor-coverage-0-umbrella.md`. Source: the config-yang section of the audit, `plan/audits/doctor-problem-inventory/config-yang.md` (slice A3, 18 class rows, all gaps, canonical codes from the inventory's code table), and the per-site list `./le arch doctor-coverage report` prints for the seven config packages. On 2026-11-11, after commit 10c6647f6d made the gate judge every doctor finding's `Fix`, it lists 63 uncovered sites: config 20 (16 log sites, and 4 `diagnostic.Diagnostic` findings with no `Fix` in `validate_semantic.go`), archive 8 (6 log sites, and 2 findings with no `Fix` in `archive/doctor.go`), confirm 2, infra 1, storage 3, transaction 24 (21 log sites, 3 raises), yang 5. The 2026-11-10 count of 57 predates the finding judgement. An eighth package, `config/cli` (`cmd_validate.go`, `ze config validate`), holds 7 more findings with no `Fix`, which this door covers and no child owned; AC-30 takes them. `warnWeakPassword` logs a variable message with no attributes, which the gate does not see as a site until child 12 AC-43 lands, so it is in none of these counts and AC-5 still codes it.

## Task

`ze doctor` reports every problem class of the config pipeline with its cause
and the command that fixes it, and every Warn or Error line and report-bus raise
in `internal/component/config` (the root package, `archive`, `confirm`, `infra`,
`storage`, `transaction`, `yang`) carries the doctor code of its class (O-2 of
the parent), so `./le arch doctor-coverage check` counts it covered.

The classes this area owns, by the main thread's brief:

| # | Class family | Where the operator meets it today |
|---|--------------|-----------------------------------|
| 1 | A config the editor or a commit accepted that the daemon later refused or rolled back | `show errors` only (`commit-aborted`, `commit-rollback`), no doctor row |
| 2 | Storage failures: candidate, rollback, archive, staged leftovers | Warn lines in the daemon log |
| 3 | Config accepted but not in effect: the file out of sync with the running config, a listen host overridden, a config recovered from history after a downgrade, a weak or plaintext password, an authz rule that names no command | Warn or Error lines once at load, or nothing |

Also owned: the defect `archive.ChangeTracker.HasChanged` records the new hash
before the archive write, so a failed `on-change` archive is never retried until
the config changes again (journal row in "Owned journal rows", fixed by AC-12).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/config-yang.md`, `ai/doors/core.md`, `ai/doors/hub-engine.md`, `ai/doors/api-surfaces.md`, `ai/doors/le-tooling.md` - the doors this work touches
  → Constraint: the config component holds no BGP import and registers its checks from its own `init()`; a check lives in the package that owns the state it reads (`doctor.go` in `internal/component/config` and `config/archive` are the precedent)
  → Constraint: runtime state in the daemon persists through `statestore` under a key in `pkg/zefs/keys.go`, never `os.WriteFile`; `pkg/zefs` is public, so a new key is a contract change made with its page
  → Constraint: `cmd/ze/hub` owns `daemonConfirmWindow`; the confirm window's doctor rows are the hub child's (`plan/release/validation/spec-doctor-coverage-11-hub-engine.md` AC-23)
  → Constraint: a compiled Go fixture returns an error when an assertion fails (`ai/doors/le-tooling.md`); the converted fixture keeps that shape
- [ ] `docs/architecture/doctor-and-health-checks.md` - phases, the runtime phase, recent events
  → Decision: a transient class (a commit that aborted once, an archive delivery that failed) is a report-bus raise rendered by `doctor-recent-events` under its own code (parent O-2)
  → Constraint: the local route never reads empty daemon state as healthy: a check reading daemon memory registers at `runtime`
- [ ] `docs/architecture/config/transaction-protocol.md` - verify, apply, rollback, the events plugins receive
  → Constraint: the abort, rollback, committed and applied events are what a plugin uses to keep its applied config equal to the running one; a lost event is the `doctor-plugin-config-unapplied` class
- [ ] `docs/guide/config-archive.md`, `docs/guide/config-editor.md` - the operator pages for archives and commit confirmed
  → Constraint: each gets the new doctor rows in the same change (documentation rule)
- [ ] `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` (P0a, AC-2, AC-3, AC-9, AC-22) and `spec-doctor-coverage-11-hub-engine.md` (AC-20, AC-23, AC-34)
  → Decision: shared codes, event-code registration and the raise fix field come from child 12 P0a; plugin stopped and plugin config unapplied checks are child 12's; this spec feeds them records and carries their codes

**Key insights:**
- The parent's AC-9 (checks reading daemon state outside `runtime`) has no row in this area: `config-semantics` reads the config tree, `archive-destinations` probes over the network, and the store checks read the store on disk.
- Every one of the 57 log and raise sites reports a real problem, so no Warn line in this area drops to Info. The other 6 sites are existing doctor findings that set no `Fix` (AC-29).
- No health row in this area, and no health-row filter keys on `commit-aborted`, `commit-rollback` or `commit-save-failed` (searched `HealthProbe` users); the renamed codes' only readers are `internal/test/fixture/plugin_fixture_06_events.go` and `test/plugin/errors-config-abort-show.ci`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/doctor.go` - `config-semantics` (post-config), declares `config-mcp-invalid`, `config-gnmi-invalid`, `config-plugin-verify`, `config-hub-invalid`
- [ ] `internal/component/config/archive/doctor.go` - `archive-destinations` (post-config), probes HTTP destinations, declares `doctor-archive-unreachable`, `doctor-archive-invalid`
- [ ] `internal/component/config/archive/scheduler.go` - `Scheduler.fireAll` (boot) and `fireByTrigger` (daily, hourly): read config, archive, prune; every failure is a Warn line and nothing is kept
- [ ] `internal/component/config/archive/archive.go` - `ChangeTracker.HasChanged` stores the new hash before it compares, so the caller's later failure cannot undo it
- [ ] `internal/component/config/archive/cmd/archive.go` - `handleArchiveTrigger`, wire method `ze-config:archive-trigger` (`config archive <name>`)
- [ ] `internal/component/config/loader.go` - `warnPlaintextOnDisk` (one Warn naming the plaintext leaves still in the source), `warnWeakPassword` (one Warn per weak password hashed at load)
- [ ] `internal/component/config/password_hash.go` - `CheckBcryptLeaves`, `ApplyPasswordHashing`, `HashedPassword` (`Path`, `Weakness`), `PasswordWeaknessWarnings`
- [ ] `internal/component/config/loader_extract.go` - web `insecure` and MCP `bind-remote false` rewrite every server host to 127.0.0.1 with an Error line
- [ ] `internal/component/config/stamp.go` - `RecoverConfig`: a config stamped by a newer release walks rollback history; six failure lines end boot, one Warn reports the recovery
- [ ] `internal/component/config/yang_schema.go` - `validatorFromResolved` logs `CheckAllValidatorsRegistered` and `CheckAllReferenceSourcesRegistered` failures and goes on
- [ ] `internal/component/config/listener.go` - `RegisterListenerProtocols` refusals, called from `init()` with literals
- [ ] `internal/component/config/yang/command.go` - command-tree declaration defects logged at load
- [ ] `internal/component/config/confirm/confirm.go` - `Window.deadlineRevert` logs two Error lines on a failed revert; `Status` (`User`, `Deadline`, `RevertFailed`, `Retry`) and `Status.Line`
- [ ] `internal/component/config/confirm/store.go` - `StoreRecorder` keeps the open window under `KeyConfigConfirmPending`
- [ ] `cmd/ze/hub/confirm_window.go` - `daemonConfirmWindow`, `startConfirmWindow`
- [ ] `internal/component/config/infra/authz.go` - `validateMatchEntries` Warns per non-regex match entry that prefixes no command
- [ ] `internal/component/config/storage/pointer.go` - `PromoteCandidate` mirrors the active version into the config file and only Warns when that write fails; `ReadActiveConfig`
- [ ] `internal/component/config/storage/tree.go` - `sweepStages` and `warnLeftover`: a staged frame the sweep cannot remove is a Warn carrying `why` and `fix`; `stagePrefix`
- [ ] `internal/component/config/storage/open.go` - the two `sweepStages` callers at store open: the offline tree sweeps `stagedFor` its own name, the live owner sweeps `liveLeftover` under its lock
- [ ] `internal/component/config/transaction/orchestrator.go` - `publishAbort`, `publishRollback`, `publishCommitted`, `publishApplied`, `collectRollbackAcks`, `handleRollbackAck`, `writeConfigFile`; report codes `reportCodeCommitAborted`, `reportCodeCommitRollback`, `reportCodeCommitSaveFail`
- [ ] `internal/component/config/transaction/executor.go` - the five `trySend*Ack` drop lines
- [ ] `internal/component/api/config_session.go` - `logCommitWarnings` (api-surfaces child's line)
- [ ] `internal/component/doctor/doctor_checks.go` - `store-integrity` (pre-config, `doctorCheckStoreIntegrity`), `disk-space`
- [ ] `internal/component/doctor/checks_recent_events.go` - `recentEventsCheck`, `recentEventsDiagnostic` renders a described code under its own name
- [ ] `internal/core/diagnostic/codes.go` - the `Code*` constants and `builtinCodes` metadata (`config-plugin-verify` the precedent)
- [ ] `pkg/zefs/keys.go` - `KeyConfigConfirmPending` (`meta/config/{name}/confirm-pending`, private) the precedent for a per-config record
- [ ] `internal/test/fixture/plugin_fixture_06_events.go` - `fixture06ErrorsConfigAbort` matches `code == "commit-aborted"` in `show errors`

**Behavior to preserve:**
- `show errors` keeps listing every aborted, rolled-back and save-failed commit with its phase and reason
- the load still hashes plaintext passwords and accepts weak ones (a config ze accepted keeps working)
- `insecure` and `bind-remote false` still force loopback
- `RecoverConfig` still recovers the newest compatible version and keeps the newer one as a backup version
- the store still opens over a staged leftover it cannot remove

**Behavior to change:**
- every class below gets a doctor row with its cause and fix command
- every site in the seven packages carries its doctor code
- the three commit report codes take their canonical `doctor-` names
- a failed archive is retried on the next trigger (AC-12)

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Doctor check registry and phases | `internal/core/diagnostic` `RegisterDoctorCheck`, `DoctorPhaseRuntime` (child 1) | Phased checks; `runtime` runs only in the daemon | extends: each new check registers from its owning package's `init()`, at the phase its AC names |
| Code metadata | `internal/core/diagnostic/codes.go`, `builtinCodes`, `Lookup` | Describes each code for `ze show diagnostic` | extends: every code of this area gets a constant and metadata (AC-1) |
| `Diagnostic.Fix` | child 1, `internal/core/diagnostic` `Diagnostic` | A fix command per finding | extends: every row here fills it |
| `config-semantics` | `internal/component/config/doctor.go` `semanticsDoctorCheck` | Runs `ValidateSemantics` over the tree | does not apply: it reports what a commit refuses; the classes here are what a commit accepted. Kept as is |
| `archive-destinations` | `internal/component/config/archive/doctor.go` | Probes HTTP destinations for reach | extends beside it: delivery and prune results are a `runtime` check in the same package (AC-10, AC-11); reach stays this check's |
| `store-integrity` | `internal/component/doctor/doctor_checks.go` `doctorCheckStoreIntegrity` | Checks the store on disk | extends: declares `doctor-store-staged-leftover` and reads the storage package's leftover list (AC-18) |
| Recent-events bridge | `internal/component/doctor/checks_recent_events.go` `checkRecentEvents` | Renders bus events under a described code | extends: the renamed commit codes are registered as event codes (child 12 P0a AC-2), so their events render under their own code with their own fix (child 12 AC-3) |
| Report bus | `internal/core/report/report.go` `RaiseError`, `RaiseWarning` | Coded events, error ring | extends: the three commit raises rename, archive delivery failures raise (AC-2, AC-10) |
| Child 12 plugin checks | `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` AC-9 (`doctor-plugin-stopped`), AC-22 (`doctor-plugin-config-unapplied`) | Planned runtime checks over the plugin server snapshot | extends: the transaction coordinator records which plugin missed which event and feeds that record (AC-21, AC-22); no second check |
| Child 11 confirm-window rows | `plan/release/validation/spec-doctor-coverage-11-hub-engine.md` AC-23 over `cmd/ze/hub` `daemonConfirmWindow` and `confirm.Window.Status` | Planned runtime and local rows for the window | does not apply as a check here: this spec registers the two confirm codes and codes the revert lines (AC-13, AC-14) |
| Child 11 plaintext row | `plan/release/validation/spec-doctor-coverage-11-hub-engine.md` AC-20 | Points at this spec's `config-passwords` (AC-4) and adds no check of its own; the hub's `password leaf format invalid` Warn line names `doctor-config-password-plaintext` | resolved (R-1): `CheckBcryptLeaves` lives in this component, so the one check for `doctor-config-password-plaintext` is this spec's `config-passwords` (AC-4), and child 11 AC-20 now says so |
| Password hashing at load | `password_hash.go` `ApplyPasswordHashing`, `HashedPassword.Weakness` | Knows each weak password only at the moment it hashes it | extends: the daemon records the weak result beside the config (AC-5) |
| Daemon runtime state | `statestore`, keys in `pkg/zefs/keys.go` (precedent `KeyConfigConfirmPending`) | Persists daemon state per config | extends: one private key `meta/config/{name}/password-weak` for the weak-password record (AC-5) |
| Store pointers | `storage/pointer.go` `ReadActiveConfig`, `PromoteCandidate` | Active version and its mirror into the file | extends: the out-of-sync check compares them (AC-16) |
| Staged-write sweep | `storage/tree.go` `sweepStages`, `stagedFor`; `storage/open.go` `liveLeftover` | Removes crashed staged writes at open | extends: the same predicates list what is left for the check (AC-18) |
| Recovery from history | `stamp.go` `RecoverConfig`, `ScanStampRelease`, `version.IsNewerRelease` | Walks rollback history at boot | extends: the check calls the same walk read-only (AC-7, AC-8) |
| Authz match validation | `infra/authz.go` `validateMatchEntries` | Logs entries that prefix no command | extends: the walk returns its findings, read by the log and the check (AC-15) |
| Registration integrity | `yang.CheckAllValidatorsRegistered`, `yang.CheckAllReferenceSourcesRegistered` | Run once at schema build, logged | extends: a local check runs them (AC-9) |
| Doctor-coverage gate | `internal/le/arch/doctorcoverage`, `CoveredCodes` | Refuses uncovered additions, reports sites | extends: no change; AC-28 is its report over these packages |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze doctor` (local route) and `show doctor` (daemon route), reaching `doctor.Answer` with the config tree, the store and the config directory
- the producers: config load, commit (`TxCoordinator`), the archive scheduler, the confirm window, the store open sweep

### Transformation Path
1. A producer meets a problem. It logs a line carrying `code` (the doctor code of its class) and, for a transient class, raises it on the report bus under the same code with its fix (child 12 P0a).
2. A producer whose problem persists keeps a value-type record: the archive scheduler's per-archive last result (AC-3), the weak-password record in the store (AC-5), the coordinator's per-plugin missed-event record (AC-21).
3. Local checks (`pre-config`, `post-config`) read the config file, the tree and the store: plaintext and weak passwords, listen host override, newer release and recovered config, registration integrity, authz match, file out of sync, staged leftovers.
4. Runtime checks read daemon memory: archive delivery and prune; the commit events render through `doctor-recent-events`.
5. Each finding is one `Diagnostic` with severity, code, message naming the object, and `Fix`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Daemon to local doctor | the store on disk (weak-password record, versions, pointers, staged frames); the daemon's `show doctor` answer through child 1's local route | No |
| Config component to plugin server | the coordinator's record published as a value type for child 12's snapshot | No |
| Report bus to doctor | `doctor-recent-events` reads `report.Errors` and `report.Warnings` | No |

### Changed-Wire Census

**Search evidence:** scope the whole tree (`internal`, `cmd`, `pkg`, `test`, `docs`); terms `commit-aborted`, `commit-rollback`, `commit-save-failed`, `reportCodeCommit`, `password-weak`; positive control: the `orchestrator.go` constants and the fixture's literal were both found. Two wires change: the values of the report-bus `code` field, and one new zefs key.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| report-bus `code` field | writer | `internal/component/config/transaction/orchestrator.go` `publishAbort`, `publishRollback`, `writeConfigFile` | `report.RaiseError` with `reportCodeCommitAborted`, `reportCodeCommitRollback`, `reportCodeCommitSaveFail` | change: renamed to `doctor-commit-aborted`, `doctor-commit-rollback`, `doctor-commit-save-failed` (no release shipped the old names) |
| report-bus `code` field | reader | `internal/test/fixture/plugin_fixture_06_events.go` `fixture06ErrorsConfigAbort` | matches the literal `commit-aborted` in `show errors` JSON | change: converted in the same commit |
| report-bus `code` field | reader | `test/plugin/errors-config-abort-show.ci` | names the code | change: converted in the same commit |
| report-bus `code` field | reader | `internal/component/config/transaction/orchestrator_test.go` (asserts the raised codes) and the `PREVENTS:` comment in `orchestrator_shutdown_test.go` | name the old codes | change: converted in the same commit |
| report-bus `code` field | reader | `internal/component/doctor/checks_recent_events.go` `recentEventsDiagnostic` | reads `Issue.Code` generically through `Lookup` | preserve: renders the new names once registered |
| zefs key `meta/config/{name}/password-weak` | writer | the daemon's config load and commit paths after `ApplyPasswordHashing` (`internal/component/config`) | new | new private key in `pkg/zefs/keys.go`, listed by `ze data registered` |
| zefs key `meta/config/{name}/password-weak` | reader | the `config-passwords` check (`internal/component/config`) through `ctx.Store` | new | new, read-only |

### Integration Points
- `diagnostic.RegisterDoctorCheck` from `init()` in `internal/component/config`, `config/archive`, `config/infra`
- child 12 P0a event-code registration for the three commit codes and the two archive codes
- child 12's plugin snapshot (AC-5 there) carries the coordinator record of AC-21

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | transient classes go through the report bus and `doctor-recent-events`; persistent ones through the store or a published record |
| No unintended coupling (components stay isolated) | Yes | every check lives in the package owning its state; `cmd/ze/hub` keeps the confirm window rows |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing Mechanisms: recovery walk, sweep predicates, authz walk, `ReadActiveConfig` are reused; one plaintext check, child 11 AC-20 points at it (R-1, resolved) |
| Zero-copy preserved where applicable (refs, not copies) | N-A | no wire encoding |
| Registration over hardcoding, outbound | Yes | each check registers from its own `init()`; codes declared in `codes.go` and the check's `Codes` |
| Registration over hardcoding, inbound | Yes | searched `checks_recent_events.go` (generic over `Lookup`), `doctorcoverage.CoveredCodes` (reads the registry); neither learns a name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The tree `ze doctor` hands a post-config check is the hashed one, so a plaintext sibling is gone from it | `loader.go` comment: `hashPlaintextSibling` deletes the plaintext leaf before `warnPlaintextOnDisk` | AC-4 reads the tree instead of the source | read the doctor config-tree loader in P1 | unvalidated |
| A-2 | Weakness cannot be judged from a bcrypt hash, so a record taken at hash time is the only source | `password_hash.go` `HashedPassword.Weakness` exists only on the hashing result | AC-5 needs no record | read `PasswordWeaknessWarnings` in P1 | unvalidated |
| A-3 | Several checks may declare one shared code (`doctor-internal-defect`) | `DoctorCheck.Codes` is a per-check list | AC-9 raises on the bus instead | read `RegisterDoctorCheck` in P0 | confirmed 2026-11-11: `validateDoctorCheckCodesReg` (`internal/core/diagnostic/doctor_registry.go`) refuses a duplicate code only inside one check's `Codes`, and `config-plugin-verify` is already declared by `config-semantics` while child 11 AC-34 declares it again |
| A-4 | A commit from the editor rewrites the config file from the committed version, so it is the fix for an out-of-sync file | `PromoteCandidate` mirrors the active version into the file | AC-16's fix names the `ze data` `write` subcommand instead | the AC-16 unit test | unvalidated |
| A-5 | Child 12 P0a lands before P0 here | child 12 Depends line | P0 waits | `./le spec status` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Child 11 AC-20 planned a `hub-local` row for `doctor-config-password-plaintext` over `CheckBcryptLeaves`, which this component owns, so two checks would answer one class | two `Codes` lists naming the code | this spec's `config-passwords` reports both forms (canonical leaf not a hash, plaintext sibling in the source) (`ai/rules/no-layering.md`). Resolved: `plan/release/validation/spec-doctor-coverage-11-hub-engine.md` AC-20 now points at this spec's AC-4 `config-passwords` and adds no check |
| R-2 | The weak-password record goes stale when a password changes | a row for a password already changed | the record stores the bcrypt value it judged; the check reports only while the leaf still holds that value |
| R-3 | The local `ze doctor` process writes the weak record during its own load | a record written outside the daemon | only the daemon writes it (the statestore is set only there; `ai/doors/core.md`, `statestore.Put` trap) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | doctor rows only, plus the archive retry (AC-12) and the renamed bus codes a script reading `show errors` JSON may match |
| How is it reverted? | single commit revert per phase |
| Who else touches this path? | child 11 (confirm window, plaintext row, reload refused), child 12 (shared codes, plugin checks), child 4 (commit advisory line) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show doctor` after a commit a plugin refuses at verify | → | `publishAbort` raise, `doctor-recent-events` | `test/plugin/errors-config-abort-show.ci` (converted to the new code, plus the `show doctor` row) |
| `show doctor` after an archive to an unreachable destination | → | `archive-delivery` runtime check | `test/plugin/doctor-archive-delivery-failed.ci` |
| `ze doctor` on a config holding `plaintext-password` | → | `config-passwords` check | `test/ui/doctor-config-password-plaintext.ci` |
| `ze doctor` with the config file edited behind the store | → | `config-file-sync` check | `test/ui/doctor-config-file-out-of-sync.ci` |

## Acceptance Criteria

Every row: severity, code, the fix line, the state read, and the route. "Event" means a report-bus raise rendered by `doctor-recent-events` under the named code (child 12 P0a). Messages follow `ai/rules/user-facing-errors.md`: the problem in the operator's terms, why, then the fix on its own line.

### Shared mechanisms (P0)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `ze show diagnostic <code>` for each code this spec names: `doctor-config-password-plaintext`, `doctor-config-password-weak`, `doctor-config-listen-host-overridden`, `doctor-config-newer-release`, `doctor-config-recovered-from-rollback`, `doctor-archive-write-failed`, `doctor-archive-prune-failed`, `doctor-config-confirm-revert-failed`, `doctor-config-confirm-pending`, `doctor-authz-match-unknown-command`, `doctor-commit-save-failed`, `doctor-config-file-out-of-sync`, `doctor-store-staged-leftover`, `doctor-commit-aborted`, `doctor-commit-rollback` | answers title, description and example; each is declared by the check its AC names, by child 11 `hub-runtime` (the two confirm codes), or by child 12's event-code registration (the event codes) | the shared codes are child 12's and are not re-registered here |
| AC-2 | a commit aborts, rolls back, or applies and cannot write the file | the bus carries `doctor-commit-aborted`, `doctor-commit-rollback`, `doctor-commit-save-failed` (renamed from `commit-aborted`, `commit-rollback`, `commit-save-failed`), registered as event codes, each raise carrying its fix: the refusal's own fix line when the reason carries one, else `ze config validate` (aborted, rollback), and AC-16's fix (save failed); the fixture and `errors-config-abort-show.ci` match the new names; the comment in `writeConfigFile` names the new code | none: no release shipped the old names, and no health-row filter keys on them |
| AC-3 | the archive scheduler finishes an archive or a prune | it keeps, per archive name, the last attempt time, the last delivered time, the last write error and the last prune error, published as a value-type snapshot from the `archive` package that the runtime check of AC-10 and AC-11 reads; with no scheduler running the snapshot answers "not running", never an empty healthy list | none |

### Config load (P1)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-4 | the config source holds a `plaintext-<leaf>` sibling of a `ze:bcrypt` leaf, or a `ze:bcrypt` leaf whose value is not a bcrypt hash (`CheckBcryptLeaves`) | local post-config check `config-passwords` (`internal/component/config`) reports Warning per leaf "the config file `<file>` still holds the plaintext password of `<leaf>`; ze hashed it at load, but anyone who reads the file reads the secret", code `doctor-config-password-plaintext`, fix the commit that writes the hashed form (A-4) or `ze passwd` for the canonical leaf; the `warnPlaintextOnDisk` line carries the code | a config read from stdin names "the loaded config" instead of a file, because there is no file |
| AC-5 | the daemon hashes a weak password at load or at commit (`HashedPassword.Weakness` set) | the daemon writes a weak-password record (leaf path, the bcrypt value judged, the rule broken, never the password) under `meta/config/{name}/password-weak`; `config-passwords` reports Warning per leaf whose current value equals the recorded one "the password of `<leaf>` is weak: `<rule>`", code `doctor-config-password-weak`, fix the `set` command for that password leaf; a strong password set later drops the record; `warnWeakPassword` carries the code; the REST, gRPC and gNMI advisory line carries it through child 4 AC-22 | a password the record does not hold is not judged, because a hash cannot be judged (A-2) |
| AC-6 | `environment web` sets `insecure` and a server host is not 127.0.0.1, or `environment mcp` leaves `bind-remote` false and a server host is not 127.0.0.1 | local post-config check `config-listeners` reports Warning per server "`<service>` server `<name>` listens on 127.0.0.1, not on `<host>` as configured, because `<insecure / bind-remote false>`", code `doctor-config-listen-host-overridden`, fix `delete environment web insecure` or `set environment mcp bind-remote true`, or the `set` that makes the host 127.0.0.1; both `loader_extract.go` lines carry the code | a host already 127.0.0.1 is not reported, because nothing is overridden |
| AC-7 | the config file's stamp release is newer than this ze (`ScanStampRelease`, `version.IsNewerRelease`), and the store holds no history, or no rollback version this release parses | local pre-config check `config-release` reports Error "`<file>` was written by ze `<r1>`; this ze is `<r2>`, and no version in its history loads, so the daemon does not start", code `doctor-config-newer-release`, fix install ze `<r1>` or newer, or `show config history` to pick a version; the seven `RecoverConfig` lines that end boot or announce the walk carry the code | a compatible version in history is AC-8, because boot recovers it |
| AC-8 | the active version was recovered from history after a downgrade: the store holds a backup version whose stamp is newer than this release and newer than the active version | `config-release` reports Warning "ze `<r2>` runs the version of `<date>` recovered from history; your newest config, written by ze `<r1>`, is kept as version `<stamp>` and is not in effect", code `doctor-config-recovered-from-rollback`, fix install ze `<r1>` or newer, or commit the config again; the recovery Warn carries the code | an active version newer than the backup is not reported, because the operator committed since |
| AC-9 | the build holds a `ze:validate` with no registered validator or a `ze:reference` with no registered source | local pre-config check `config-registrations` runs `CheckAllValidatorsRegistered` and `CheckAllReferenceSourcesRegistered` and reports Error naming the leaf and the missing name "every value of `<leaf>` is refused at commit", code `doctor-internal-defect`, fix install a complete ze build; both `yang_schema.go` lines carry the code | none |

### Archive (P2)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-10 | a boot or scheduled archive was not delivered, or the scheduler could not read the config | runtime check `archive-delivery` (`config/archive`) reports Warning per archive "archive `<name>` to `<location>` failed at `<time>`: `<error>`; the last delivered archive was `<time or never>`", code `doctor-archive-write-failed`, fix the archive trigger `config archive <name>` once the cause is fixed; the failure is also an event under the same code; the four `scheduler.go` read and write lines carry the code | a later delivered archive clears the row, because the destination holds the config |
| AC-11 | a prune after a delivered archive failed | `archive-delivery` reports Warning "old archives of `<name>` at `<location>` were not removed: `<error>`; the destination keeps growing", code `doctor-archive-prune-failed`, fix the archive trigger after the cause is fixed; both prune lines carry the code | a later prune that succeeds clears it |
| AC-12 | an `on-change` archive, or the boot archive, fails, then the next trigger fires with the config unchanged | the archive is attempted again: the change tracker records the hash only after a delivered archive, so a failed write is retried at every trigger until it lands (journal row in "Owned journal rows") | an unchanged config after a delivered archive is still skipped, because `on-change` means it |

### Confirm and authz (P3)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-13 | a confirmed commit timed out and its revert failed, retrying or out of retries | both `deadlineRevert` Error lines carry `doctor-config-confirm-revert-failed`, registered here with metadata; the row is child 11 AC-23's `hub-runtime` over `Status.RevertFailed` | the check is child 11's, because `cmd/ze/hub` owns the window |
| AC-14 | a confirmed commit is pending | `doctor-config-confirm-pending` is registered here with metadata; the row is child 11 AC-23's | no log line, by design: the class is silent until the deadline |
| AC-15 | an authz profile's non-regex `match` entry is a prefix of no command and no command is its prefix | local post-config check `authz-match` (`config/infra`) reports Warning per entry "authorization profile `<profile>` `<section>` entry `<match>` names no command, so the rule grants or denies nothing", code `doctor-authz-match-unknown-command`, fix the `delete` of that entry; `validateMatchEntries` returns its findings to both the load-time line (which carries the code) and the check | a regex entry is not judged, because a prefix test cannot judge it |

### Storage (P4)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-16 | the config file differs from the store's active version (`ReadActiveConfig`) | local post-config check `config-file-sync` (`internal/component/config`) reports Error "`<file>` differs from the committed version `<stamp>` that ze runs; a restart loads the file, not the running config", code `doctor-config-file-out-of-sync`, fix the commit that rewrites the file (A-4) after the write cause is fixed (`disk-space` and the directory permissions name it); both `PromoteCandidate` mirror lines carry the code | a config with no store history is not judged, because there is no committed version |
| AC-17 | a commit applied and `writeConfigFile` failed | the Warn line and the raise carry `doctor-commit-save-failed`; `show doctor` shows the event row with AC-16's fix, and AC-16 reports the file locally once the daemon is down | none |
| AC-18 | the store folder holds a staged frame (`stagePrefix`, matched by the sweep's own `stagedFor` and `liveLeftover` predicates) that the open sweep could not remove | `store-integrity` declares `doctor-store-staged-leftover` and reports Warning per path, read from a storage function that lists leftovers with the sweep's predicates, "a crashed write left `<path>`; it holds no published data", fix with ze stopped `rm <path>`; `warnLeftover` carries the code | a frame the sweep removed is gone and not reported |

### Transaction (P5)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-19 | a commit aborted before apply, seen from another surface | `show doctor` shows a Warning event row under `doctor-commit-aborted` naming the transaction, phase and reason, with the raise's fix (AC-2) | the box stays ready, because nothing was applied |
| AC-20 | a commit failed during apply and was rolled back | `show doctor` shows a Warning event row under `doctor-commit-rollback` naming the reason, with its fix | none |
| AC-21 | a transaction's abort, rollback, committed or applied event could not be emitted, a rollback ack timed out, or a plugin reported broken during rollback and was restarted | the six lines carry `doctor-plugin-config-unapplied`; the rollback-timeout line names the plugins still owing an ack, not only their count; the coordinator records, per plugin, the transaction whose event it missed or whose rollback it did not ack, published for child 12's snapshot, so child 12 AC-22's check names the plugin and the transaction | the check is child 12's |
| AC-22 | a plugin broken during rollback could not be restarted | the line carries `doctor-plugin-stopped`, and the coordinator raises it on the bus naming the plugin, so child 12 AC-9 reports it down | the check is child 12's |

### Not problem classes (P6)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-23 | the three listener registry refusals (`listener.go` `RegisterListenerProtocols` and the default registration) | each line carries `doctor-internal-defect` | none: every caller is an `init()` with literals, a build defect |
| AC-24 | the five YANG command-tree declaration defects (`yang/command.go`) | each line carries `doctor-internal-defect` | none: compiled-in YANG |
| AC-25 | the four transaction event marshal failures (`orchestrator.go` `publish*`) | each line carries `doctor-internal-defect` | none: a fixed struct |
| AC-26 | the eight ack-channel-full drops (`executor.go` `trySend*Ack`, `orchestrator.go`) | each line carries `doctor-internal-defect` | none: a plugin sent more acks than the transaction expects; the timeout it causes is AC-21 |
| AC-27 | the rollback tier computation failure (`collectRollbackAcks`) | the line carries `doctor-internal-defect` | none |
| AC-28 | `./le arch doctor-coverage report` after P6 | zero uncovered sites in `internal/component/config`, `config/archive`, `config/confirm`, `config/infra`, `config/storage`, `config/transaction`, `config/yang`, doctor findings included (AC-29) | none: every site of the area reports a real problem, so none drops to Info |
| AC-29 | the existing doctor findings that set no `Fix`: the four `ValidateSemantics` findings in `validate_semantic.go` (`config-mcp-invalid`, `config-gnmi-invalid`, `config-plugin-verify`, `config-hub-invalid`) and the two `archive-destinations` findings in `archive/doctor.go` (`doctor-archive-invalid`, `doctor-archive-unreachable`) | each sets `Fix` to the command or config statement that clears it: the refusal's own fix line where the error carries one, else `ze config validate` for the four semantic findings; the `set` or `delete` of the archive `location` for the invalid entry; the archive trigger `config archive <name>` once the destination answers for the unreachable one; the archive message drops the fix it spells inline today, since the renderer draws `Fix` on its own line | none: commit 10c6647f6d makes a finding with no `Fix` an uncovered site |
| AC-30 | the seven `diagnostic.Diagnostic` literals with no `Fix` in `internal/component/config/cli/cmd_validate.go`, the `ze config validate` command this door covers and no other doctor child owns (counted 2026-11-11): the four `validationResult` helpers `addError`, `addErrorLine`, `addErrorWithRepair` and `addWarning`, the YANG failure finding in the section walk (`yangErrorCode`), `config-listener-conflict`, and the RFC 8212 warning of `addRFC8212Warning` | each sets `Fix`: the four helpers take the fix as an argument, so every caller names the command that clears its own finding and no helper builds a finding without one; a YANG failure's fix is `set <path> <expected>` when the failure names an expected value, else `delete <path>`; the listener conflict's is the `set` of a free address or port on one of the two listeners it names; the RFC 8212 warning's is `a.Fix`, the editor command it carries today in `Help` and spells inline in its message, which drops that text since the renderer draws `Fix` on its own line | none: a finding with no `Fix` is an uncovered site, and `ze config validate` is the offline route an operator runs first |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConfigYangCodesDescribed` | `internal/core/diagnostic/codes_test.go` | AC-1 | |
| `TestCommitReportCodesRenamed` | `internal/component/config/transaction/orchestrator_test.go` | AC-2, AC-17, AC-19, AC-20 | |
| `TestArchiveSnapshotRecordsLastResult` | `internal/component/config/archive/scheduler_test.go` | AC-3 | |
| `TestConfigPasswordsPlaintext` | `internal/component/config/doctor_test.go` | AC-4 | |
| `TestConfigPasswordsWeakRecord` | `internal/component/config/doctor_test.go` | AC-5, R-2 | |
| `TestConfigListenersOverridden` | `internal/component/config/doctor_test.go` | AC-6 | |
| `TestConfigReleaseNewerAndRecovered` | `internal/component/config/stamp_test.go` | AC-7, AC-8 | |
| `TestConfigRegistrationsIntegrity` | `internal/component/config/doctor_test.go` | AC-9 | |
| `TestArchiveDeliveryRows` | `internal/component/config/archive/doctor_test.go` | AC-10, AC-11 | |
| `TestSchedulerRetriesFailedArchive` | `internal/component/config/archive/scheduler_test.go` | AC-12 (fails today: the second trigger skips) | |
| `TestConfirmRevertLinesCarryCode` | `internal/component/config/confirm/confirm_failure_test.go` | AC-13 | |
| `TestAuthzMatchUnknownCommand` | `internal/component/config/infra/authz_test.go` | AC-15 | |
| `TestConfigFileOutOfSync` | `internal/component/config/doctor_test.go` | AC-16, A-4 | |
| `TestStoreStagedLeftoverListed` | `internal/component/config/storage/sweep_stages_test.go` | AC-18 | |
| `TestRollbackRecordNamesPlugins` | `internal/component/config/transaction/orchestrator_test.go` | AC-21, AC-22 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| release stamp comparison | equal, older, newer | equal release (not reported) | N/A | newer by one patch (reported) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `errors-config-abort-show` | `test/plugin/errors-config-abort-show.ci` | a refused commit shows in `show errors` and in `show doctor` under `doctor-commit-aborted` | |
| `doctor-archive-delivery-failed` | `test/plugin/` | an archive to an unreachable file location shows its row, then clears after the trigger delivers | |
| `doctor-config-password-plaintext` | `test/ui/` | a config with `plaintext-password` makes `ze doctor` name the leaf and the fix | |
| `doctor-config-file-out-of-sync` | `test/ui/` | a file edited behind the store makes `ze doctor` report it | |
| `doctor-config-listen-host-overridden` | `test/ui/` | MCP host 0.0.0.0 without `bind-remote` is reported | |

### Interop Tests (Scope: protocol)
N-A: no protocol behavior changes.

## Files to Modify
- `internal/core/diagnostic/codes.go` - constants and metadata for the AC-1 codes
- `internal/component/config/transaction/orchestrator.go` - renamed codes, raise fixes, coded lines, rollback record (AC-2, AC-17, AC-19 to AC-22, AC-25 to AC-27)
- `internal/component/config/transaction/executor.go` - coded lines (AC-26)
- `internal/component/config/archive/archive.go`, `scheduler.go`, `doctor.go`, `register.go` - snapshot, retry fix, runtime check (AC-3, AC-10 to AC-12)
- `internal/component/config/loader.go`, `loader_extract.go`, `password_hash.go`, `stamp.go`, `yang_schema.go`, `listener.go`, `doctor.go` - coded lines, weak record, local checks (AC-4 to AC-9, AC-16, AC-23)
- `internal/component/config/yang/command.go` - coded lines (AC-24)
- `internal/component/config/confirm/confirm.go` - coded lines (AC-13)
- `internal/component/config/infra/authz.go` - findings walk (AC-15); the package has no `init()` and no register file today, so the check registers from the new `infra/doctor.go`
- `internal/component/config/validate_semantic.go` - `Fix` on the four semantic findings (AC-29); `archive/doctor.go` carries the archive half
- `internal/component/config/storage/pointer.go`, `tree.go`, `open.go` - coded lines, leftover list (AC-16, AC-18)
- `internal/component/doctor/doctor_checks.go` - `store-integrity` declares the leftover code (AC-18)
- `pkg/zefs/keys.go` - the weak-password record key (AC-5)
- `internal/test/fixture/plugin_fixture_06_events.go`, `test/plugin/errors-config-abort-show.ci` - renamed code (AC-2)
- `docs/guide/config-archive.md`, `docs/guide/config-editor.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/zefs-format.md` - rows, codes, key

## Files to Create
- `internal/component/config/infra/doctor.go` (+ test) - the `authz-match` check and its `init()` (AC-15)
- `test/plugin/doctor-archive-delivery-failed.ci`
- `test/ui/doctor-config-password-plaintext.ci`, `test/ui/doctor-config-file-out-of-sync.ci`, `test/ui/doctor-config-listen-host-overridden.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new config or RPC |
| YANG validation constraints | No | none changed |
| YANG custom validators | No | none added |
| CLI commands/flags | No | fixes name existing commands |
| CLI grammar (keyword before value) | N-A | no command added |
| Editor autocomplete | N-A | no leaf added |
| Functional test for new RPC/API | N-A | no RPC; doctor rows are proven by the `.ci` files above |
| Pipe completeness | N-A | `show doctor` output unchanged in shape |
| Env var registration | No | no env var |
| Doctor check for runtime dependencies | Yes | this spec is the doctor checks of the area; codes in `codes.go` |
| Prometheus counters/metrics | No | none |
| BGP family surface | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | doctor rows extend an existing feature |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/config-archive.md` (delivery rows, retry), `docs/guide/config-editor.md` (commit event rows, out-of-sync file) |
| 7 | Wire format changed? | No | report-bus code values only, covered by row 12 |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/doctor-and-health-checks.md` (the config checks and their phases), `docs/architecture/zefs-format.md` (new key) |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | the `// Design:` pages the changed files declare: `docs/features/ai-first.md` (`config/doctor.go`: updated with the new config checks), `docs/architecture/config/yang-config-design.md` (`yang_schema.go`: updated, the registration-integrity check), `docs/architecture/config/syntax.md` (loader files: unaffected, no syntax changes), `docs/architecture/config/apply-ordering.md` (transaction files: unaffected, apply order unchanged); the rest derived at implementation with `./le spec citation anchors spec plan/release/validation/spec-doctor-coverage-9-config-yang.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | no page under `docs/` or `website/` names `commit-aborted`, `commit-rollback` or `commit-save-failed` (grep 2026-11-11); the in-tree readers are the census rows |

## Implementation Steps

### Implementation Phases

Sized for one agent of about 60 tool calls each. Every phase ends with the scoped package test under `./le job run` and its commit.

| Phase | Work | ACs | Needs | Parallel |
|-------|------|-----|-------|----------|
| P0 shared | codes and metadata; commit code rename with event registration, raise fixes, fixture and `.ci` conversion; archive snapshot | AC-1, AC-2, AC-3 | child 12 P0a | first |
| P1 config load | `config-passwords` (plaintext, weak record and key), `config-listeners`, `config-release`, `config-registrations` | AC-4 to AC-9 | P0 | ∥ P2, P3, P4 |
| P2 archive | `archive-delivery`, retry fix | AC-10 to AC-12 | P0 | ∥ P1 |
| P3 confirm and authz | coded revert lines; `authz-match` | AC-13 to AC-15 | P0 | ∥ P1 |
| P4 storage | `config-file-sync`, save-failed line, leftover list in `store-integrity` | AC-16 to AC-18 | P0 | ∥ P1 |
| P5 transaction | event rows, rollback record, plugin-stopped raise | AC-19 to AC-22 | P0, child 12 AC-5 snapshot | after P0 |
| P6 not problem classes | coded lines; `Fix` on the existing findings; coverage report | AC-23 to AC-29 | P0 to P5 | last |

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Correctness | the weak record never stores the password; a row clears when its state clears (AC-5, AC-8, AC-10, AC-11) |
| Naming | every code is a canonical name from the inventory table; JSON keys kebab-case |
| Data flow | each check lives in the package owning its state; no check in `internal/component/doctor` reads config internals except through a function the owner exports |
| Rule: no layering | the old commit code names are deleted, not aliased; one plaintext check (R-1) |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| zero uncovered sites in the seven packages | `./le arch doctor-coverage report` |
| renamed codes have no reader left | grep `commit-aborted` over the tree finds only `doctor-commit-aborted` |
| retry fix | `TestSchedulerRetriesFailedArchive` fails before, passes after |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Secret leakage | no row, record or log line carries a password, plaintext or hash; the plaintext row names the leaf only |
| Error leakage | archive errors may name a URL: strip credentials from it as `archive-destinations` does |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood, back to research |
| Lint failure | Fix inline. If architectural, back to design |
| Functional test fails | Check the AC: wrong AC to design, correct AC to implement |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- "Config accepted, then refused by the daemon" reaches the operator as `doctor-commit-aborted` or `doctor-commit-rollback` at run time, or as a boot failure `config-semantics` and `ze config validate` cover locally; no separate class exists.
- A weak password is knowable only while ze holds the plaintext, so its doctor row needs a record taken at that moment.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/record-written-before-the-operation-succeeds.md` | 2026-11-10 | archive `ChangeTracker.HasChanged` records the hash before the write (AC-12) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Commit codes renamed, not aliased | keep `commit-*` beside `doctor-commit-*` | `ai/rules/no-layering.md`; no release shipped the old names |
| One `config-passwords` check owns both plaintext forms | child 11's `hub-local` row plus a second config row | `CheckBcryptLeaves` and the load-time detection live in this component; one class, one check (R-1) |
| Weak password from a store record | report-bus event at load | an event vanishes at restart and never reaches the local route; the record survives and clears itself |
| Archive delivery as a runtime check plus an event | local check probing the destination | reach is already `archive-destinations`; delivery is a result only the scheduler knows |

## Known Limitations
- The confirm-window rows and the plugin-state checks are other children's (11 and 12), by design of the cross-child split; this spec feeds them.

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
- [ ] AC-1..AC-29 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests (`./le job run label config command go test -race ./internal/component/config/...`, and `./internal/component/doctor/`, `./internal/core/diagnostic/`) and the named `.ci` files through `./le test functional plugin` and `./le test functional ui`, each run with the test names above only. No agent runs any `./le verify` command
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests: N-A, no protocol change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + journal row status
- [ ] **Commit B:** `remove plan/release/validation/spec-doctor-coverage-9-config-yang.md`
