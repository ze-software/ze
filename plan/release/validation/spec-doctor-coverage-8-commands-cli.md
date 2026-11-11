# Spec: doctor-coverage-8-commands-cli

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | cli |
| Priority | P2 |
| Release reason | `ai/META-DECISIONS.md`, "Goal and scope": `ze doctor` reports every problem an operator can hit, each with its cause and the command that fixes it; O-3 of `spec-doctor-coverage-1-audit-and-gate` makes every area child release work |
| RFC gaps | - |
| Depends | `spec-doctor-coverage-1-audit-and-gate` (the `runtime` phase, `Diagnostic.Fix`, the `recent-events` check and the doctor-coverage gate, landed in ce58dddf53 and fc96556110); `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md`, whose phase P0a registers the cross-cutting codes (`doctor-internal-defect`), the event-code registration and the convention that a report-bus raise carries its own fix: it comes before this spec's P3 to P5. Its P0b (the report route for a forked plugin) is not needed here: the SSH server and the `cli` sessions run in the daemon process and raise through `internal/core/report` directly, and the `ze cli` client lines (AC-13) raise nothing; `plan/release/validation/spec-doctor-coverage-4-api-surfaces.md`, which owns the management listener classes (SSH, web and MCP after start), the SSH public-key class (its AC-13) and, by the cross-child decision recorded in its header, every SSH management-auth row of this area's inventory: the login failures (its AC-2, AC-11), the unreadable user list (its AC-12) and the audit record (its AC-4), with `doctor-mgmt-auth-failed`, `doctor-mgmt-user-list-unreadable` and the SSH lines that carry them; `plan/release/validation/spec-doctor-coverage-9-config-yang.md`, which owns the editor commit and store classes (`doctor-store-integrity`, `doctor-store-permissions`, `doctor-store-staged-leftover`); `plan/release/validation/spec-doctor-coverage-11-hub-engine.md` reuses `doctor-cli-editor-unavailable`, which this spec registers (its AC-22), and owns the SSH boot half of the listener class (its AC-8) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent: `plan/release/validation/spec-doctor-coverage-0-umbrella.md`. Source: the commands-cli section of the audit (`plan/audits/doctor-problem-inventory/commands-cli.md`, slice A3, scope `internal/component/command`, `internal/component/cmd`, `internal/component/cli`, `internal/component/cli/client`, `internal/component/ssh`).

## Task

Thomas: "Not only the one you have, but all problems. So we will have to audit
the code for all the things we need to be added to it."

The audit placed the CLI, editor and SSH problem sites in 8 classes, all gap
or partial, plus two groups of lines that are not problem classes. Reading the
producers for this spec corrected two of them and split one (see "Audit rows
corrected"). This spec closes every gap the area owns:

| # | Work |
|---|------|
| 1 | `ze doctor` (local) and `show doctor` (every daemon surface) report every commands-cli problem class with its cause in plain words and the exact command that fixes it on its own line |
| 2 | Every Warn and Error line, and every report-bus raise, in `internal/component/ssh`, `internal/component/cli` and `internal/component/cli/client` names a doctor code that a registered check declares (O-2), so `./le arch doctor-coverage report` counts the area covered |
| 3 | The existing `ssh-host-key` check reads the host key and certificate where the server reads them, the daemon store first (journal row `helper-bypassed-by-an-open-coded-copy`, 2026-11-10) |

### Audit rows corrected

| Audit row | What the producer shows | Correction |
|-----------|-------------------------|------------|
| "An editor session's state was not persisted: ... or a session's model failed to close" (sites: the `ssh/ssh.go` stop drain and session close) | `sessionCloseMiddleware` logs the error of `owner.closer.Close()`. The closer is the session's `cli.Model`, whose `Close` ends dispatch, calls `History.Close` (which returns nothing) and returns `TranscriptWriter.Close`, the close of the transcript file. So the session-close line reports a transcript file that did not close, never editor state | split: the stop-drain line stays in the editor-storage class (AC-8, AC-9); the session-close line is the transcript class (AC-10) |
| "Attaching the CLI gives a command-only session: the config editor could not be built (`unicli.NewModel` error)", state read "the editor's store and draft" | `NewModel` fails only when `NewCompleter` or `NewConfigValidator` fails, both on the YANG loader (`yang.DefaultLoader`, `config.YANGSchema`). The store and draft causes are `newSessionEditor`'s (`cmd/ze/hub/session_editor.go`), whose lines child 11 AC-22 codes | the check reads both: the schema load and the active config's editor build (AC-11) |
| "SSH is not serving a configured listener" | the boot half is `cmd/ze/hub/service_ssh.go` (`sshBuildImpl`, `sshBuildStandaloneImpl`); the after-start half is `serveListener` (`ssh/listener_start.go`) | owned by siblings, no AC here: boot by child 11 AC-8, after start by child 4 (management listener classes), both under `doctor-mgmt-listener-not-serving` |
| "SSH login refused", "running-config users unreadable", "SSH audit record failed" | the lines are `authenticatePasswordResult`, `authenticatePublicKeyResult` and `recordAuthFailure` (`ssh/passwordauth.go`, `ssh/pubkey.go`, `ssh/ssh.go`) | owned by `plan/release/validation/spec-doctor-coverage-4-api-surfaces.md` (AC-2, AC-11 login-failure record and `doctor-mgmt-auth-failed`; AC-12 `doctor-mgmt-user-list-unreadable`; AC-4 `doctor-audit-record-failed`), as its header's cross-child decision assigns; this spec's former AC-2, AC-3 and AC-6 coded and raised the same lines and are removed, so no line is coded by two specs |
| "An SSH public key that does not parse passes commit" | `parseConfiguredKey` runs only at login (`matchPublicKey`) | owned by `plan/release/validation/spec-doctor-coverage-4-api-surfaces.md` AC-13 (the verifier, the `ssh-public-keys` check, `doctor-ssh-public-key-invalid`, the `matchPublicKey` Warn line, the YANG description, and the journal row `validate-accepts-what-start-refuses`); no AC here |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - outside parties, plugin process boundary, registration, shared machinery
  → Constraint: a check registers from `init()` in the package that owns the state it reads; no edit to a central list
  → Constraint: every operator command runs through the daemon on every surface; `show doctor` is that surface for runtime state
- [ ] `ai/doors/commands-cli.md` - command model, editor, SSH server
  → Constraint: SSH compiles out (`ze_ssh`); the SSH checks register from `internal/component/ssh/register.go`, so a build without SSH drops them and still compiles; the editor cannot be compiled out, so the `cli` checks are always present
  → Constraint: one surface fixed means the other checked (trap "One surface fixed, the other not")
- [ ] `ai/doors/hub-engine.md`, `ai/doors/core.md`, `ai/doors/access-security.md`, `ai/doors/le-tooling.md` - the hub that builds the SSH server and session models, the core diagnostic and report packages, the audit recorder, the coverage gate
  → Constraint: the hub's own lines (`service_ssh.go`, `session_factory.go`, `session_editor.go`) are child 11's; this spec changes no `cmd/ze/hub` file
  → Constraint: `internal/core` is the leaf tier; the new codes add spellings to `internal/core/diagnostic/codes.go` and nothing else there
- [ ] `docs/architecture/doctor-and-health-checks.md` - the runtime phase, the local route, the fix line, the coverage gate, recent events
  → Decision: a `runtime` check is handed the platform alone, never a config tree; it reads its owner's live state
  → Constraint: a transient code counts as covered only when its owner's check declares it; `recent-events` then renders the raise under that code
  → Constraint: a site is covered when its code is a literal or constant a registered check declares in `Codes`
- [ ] `spec-doctor-coverage-1-audit-and-gate` - Task, O-1 to O-3, AC-6, AC-9, Data Flow amendments
  → Decision: O-2, every Warn and Error line carries a doctor code, no transient marker; O-3, release work
  → Constraint: AC-9, a check that reads daemon state from a non-runtime phase moves to `runtime`; this area has none (`ssh-host-key` reads the config and the store)
- [ ] `plan/release/validation/spec-doctor-coverage-2-home-cpe-batch.md` - sibling child
  → Constraint: it owns no commands-cli row; nothing here respecs it
- [ ] `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` - phases P0a and P0b, AC-33
  → Constraint: the shared codes, the event-code registration, the raise route and its fix convention are built there once and cited here by phase name (`ai/rules/no-layering.md`)
- [ ] `plan/release/validation/spec-doctor-coverage-11-hub-engine.md` - the format this spec follows, and the hub's SSH and editor lines
  → Constraint: its AC-8 owns the SSH boot listener lines, its AC-22 codes the hub's editor lines with `doctor-cli-editor-unavailable`, which this spec registers
- [ ] `ai/rules/user-facing-errors.md` - problem first, why, then the fixing command on its own line
- [ ] `ai/rules/spec-no-code.md` - no code in a spec

**Key insights:**
- the SSH login failure, unreadable user list and audit lines are child 4's (Audit rows corrected); this spec edits no line in `ssh/pubkey.go` or `ssh/passwordauth.go`
- the SSH server never reads the host key from the filesystem: `resolveHostKeyOption` reads it from `s.config.Storage` when the store holds it, and otherwise generates one (and writes it to the store when there is one); only the host certificate falls back to disk (`hostKeyWithCertOption`, `readFileFromDisk`)
- the editor history write (`History.publish`) and the transcript writes (`TranscriptWriter.Record`, `writeHeader`) discard their errors today, so no producer-side record exists for doctor to read; AC-8 and AC-10 add it
- the public-key class (no verifier calls `parseConfiguredKey`) is child 4's AC-13, not this spec's

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ssh/pubkey.go` - `authenticatePublicKeyResult` Warns "cannot read the running config users" when `s.users()` fails, and "SSH auth failure" (source public-key) on no match, each followed by `recordAuthFailure`; `matchPublicKey` Warns "SSH public key parse error" per configured key `parseConfiguredKey` refuses, and skips it
- [ ] `internal/component/ssh/passwordauth.go` - `authenticatePasswordResult` Warns "SSH auth failure" and calls `recordAuthFailure`
- [ ] `internal/component/ssh/ssh.go` - `recordAuthFailure` writes an `audit.ActionAuthFail` entry and Warns "audit record failed" on error; `Reconfigure` Warns when a removed listener does not close; `Stop` logs Error when `models.wait(modelDrainLimit)` gives up on an editor session's storage write; `sessionCloseMiddleware` Warns "SSH session close" with the error of the session model's `Close`; `resolveHostKeyOption` and `hostKeyWithCertOption` read the host key and certificate from `s.config.Storage` first (`store.Exists`), the certificate from disk otherwise
- [ ] `internal/component/ssh/listener_start.go` - `serveListener` logs Error "SSH server error" when `Serve` returns after start (sibling-owned, see Task)
- [ ] `internal/component/ssh/doctor.go` - `sshHostKeyDoctorCheck` (`ssh-host-key`, post-config, `doctor-ssh-hostkey-missing`): `checkSSHHostKey` runs `os.Stat` on the host key path and the host certificate path only; it never reads `ctx.Store`
- [ ] `internal/component/ssh/register.go` - `init` registers `sshHostKeyDoctorCheck` through `diagnostic.RegisterDoctorCheck` and panics on a refused declaration
- [ ] `internal/component/ssh/yang/ze-ssh-conf.yang` - `public-keys` list (key `name`, leaves `type` enumeration and `key` string) augments `system authentication user`; its description says a key that does not parse is logged and skipped
- [ ] `cmd/ze/hub/service_ssh.go` - `sshBuildImpl` and `sshBuildStandaloneImpl` hand the daemon store (`params.Store`, `in.Storage`) to the SSH server as `Config.Storage`
- [ ] `internal/component/cli/model.go` - `Model.Close` ends dispatch, calls `History.Close`, returns `TranscriptWriter.Close`; `NewModel` fails only on `NewCompleter` or `NewConfigValidator`
- [ ] `internal/component/cli/history.go` - `History.publish` writes the history file through a `syncutil.Coalescer` worker and discards the `WriteFile` error ("best-effort"); `Close` drains and returns nothing
- [ ] `internal/component/cli/transcript.go` - `TranscriptWriter.Record` and `writeHeader` discard write errors ("best-effort"); `Close` returns the file close error; `OpenTranscriptFile` prints a stderr warning and returns nil when the directory or file cannot be created
- [ ] `internal/component/cli/editor.go` - `Editor.Close` closes an owned offline store only
- [ ] `internal/component/cli/editor_commit_force.go` - `AckDiscardNotice` Warns "discard notice shown but not marked" when `writeDiscardNotice` fails; `startDiscardNotice` Warns "discard notice unreadable at session start; every line will be shown"
- [ ] `internal/component/cli/client/main.go` - `newAttachedModel` Warns "config editor unavailable" and falls back to `NewCommandModel` when `NewModel` fails; `buildRuntimeTree` and its dispatch twin Warn "system command list refused" when `decodeCommandList` fails and fall back to the static tree
- [ ] `cmd/ze/hub/session_factory.go`, `cmd/ze/hub/session_editor.go` - `newSessionEditor` fails on `ValidateUser`, `NewEditorWithStorage` or `LoadDraft`; the hub Warns and the session runs without config mode (child 11 AC-22 codes these lines)
- [ ] `internal/component/doctor/checks_recent_events.go` - `recentEventsCheck` renders `report.Errors` and `report.Warnings`, one row per (severity, source, code, subject), under the event's code when `diagnostic.Lookup` describes it
- [ ] `internal/core/diagnostic/codes.go` - the `CodeMeta` entries; `doctor-ssh-hostkey-missing` is registered there as a literal

**Behavior to preserve:**
- every existing check, its name, phase and code (`ssh-host-key`, `doctor-ssh-hostkey-missing`)
- the login decisions: a key that does not parse never matches; the other keys of that user still match
- the attached console falls back to the command-only console rather than failing
- `ze cli` falls back to its static command tree when the daemon's list does not decode
- the transcript remains opt-in (`ze.cli.transcript`) and never blocks a command

**Behavior to change:**
- every Warn and Error line in the area names its doctor code; the transient classes raise on the report bus by child 12's route
- `ssh-host-key` reads the store first
- the history publication, the discard notice and the transcript keep their last write failure for a runtime check
- the SSH session-close line names the transcript, the only thing that close reports

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| SSH host key check | `internal/component/ssh/doctor.go` `sshHostKeyDoctorCheck`, `checkSSHHostKey` | Post-config `os.Stat` of the host key and certificate paths | extends: reads `ctx.Store` first, as the server does (AC-7) |
| Doctor check registry and phases | `internal/core/diagnostic/doctor_registry.go` `RegisterDoctorCheck`, `DoctorCheck`, `DoctorCheckContext` (`Tree`, `ConfigDir`, `Store`, `Platform`) | Registered checks with codes, by phase | extends: `cli` registers `cli-editor` and `cli-session-storage`, each from its own `init()` |
| Diagnostic code registry | `internal/core/diagnostic/codes.go` and its `CodeMeta` entries | One spelling and metadata per code | extends: the codes this spec owns get one spelling and metadata each |
| `Diagnostic.Fix` | `internal/core/diagnostic/types.go` `Diagnostic`, page "Every finding names the command that fixes it" | The fix command alone, drawn on its own line | extends: every new row fills it |
| Report bus | `internal/core/report/report.go` `RaiseWarning`, `ClearWarning`, `RaiseError` | Active warnings and a ring of error events | extends: the attached console (`cli/client` `newAttachedModel`, in the daemon process) raises its event directly; the raise carries its fix by child 12's P0a convention |
| Recent-events check | `internal/component/doctor/checks_recent_events.go` `recentEventsCheck` | One runtime row per bus key, under the event's code when described | does not apply as a change: it renders the raises above once their owner's check declares the code |
| Health registry | `internal/core/health` `Register` | Health endpoint rows by name | does not apply: the slice found no health row in this area, and no row keys on a code this spec renames |
| Audit recorder | `internal/core/audit` `Recorder.Record`; child 12 AC-33 | Writes the audit entry; the SSH caller Warns on error | does not apply here: the SSH line is child 4's AC-4 |
| Public key parse | `internal/component/ssh/pubkey.go` `parseConfiguredKey` | The only parse of a configured key, at login | does not apply here: the verifier and the `ssh-public-keys` check over it are `plan/release/validation/spec-doctor-coverage-4-api-surfaces.md` AC-13 |
| Editor history | `internal/component/cli/history.go` `History.publish`, `History.Close` | Writes the per-user history file, discarding the error | extends: the error and a write slower than the SSH drain limit go to the session-storage record |
| Transcript writer | `internal/component/cli/transcript.go` `TranscriptWriter`, `OpenTranscriptFile` | Opt-in session record, errors discarded | extends: keeps its first open, write or close error for the session-storage record |
| Editor build | `cli.NewEditorWithStorage`, `cli.NewModel`, `NewCompleter`, `NewConfigValidator` | Build a session's editor and model | extends: `cli-editor` calls the same constructors, never a copy |
| Doctor-coverage gate | `internal/le/arch/doctorcoverage`, `./le arch doctor-coverage report` and `check` | Lists uncovered sites | does not apply as a change: it is the measure (AC-14) |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze doctor` (local route: the daemon's answer when it answers, else the local checks, O-1)
- `show doctor` on every daemon surface, which runs the local phases and the `runtime` phase in the daemon process

### Transformation Path
1. `ssh-host-key` judges the host key and certificate the way the server loads them: the key from `ctx.Store` only (the server generates a key the store does not hold, and never reads one from disk), the certificate from `ctx.Store` first and the filesystem otherwise.
2. (removed: the SSH login-failure and user-list raises are child 4's)
3. The `cli` package keeps one process-wide session-storage record: per (user, kind, path) the last failed or overdue write, with its error and time. `History.publish`, `AckDiscardNotice`, `startDiscardNotice`, `TranscriptWriter` and `OpenTranscriptFile` write it; a later successful write of the same path clears its entry.
4. `cli-session-storage` (runtime, `internal/component/cli`) reads that record.
5. `cli-editor` (post-config, `internal/component/cli`) builds the completer and the validator, then opens the active config from `ctx.Store` through `NewEditorWithStorage` without writing.
6. Each finding is a `Diagnostic` with code, severity, a message naming the user, key, path or cause, and `Fix` holding the exact command alone.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| SSH to doctor | `internal/component/ssh/register.go` registers its checks from `init()`; compiled out with `ze_ssh` | Yes: `ssh-host-key` already registers there |
| CLI to doctor | `internal/component/cli` registers its checks from `init()`; the runtime check reads a package record the daemon's sessions write | No: A-1 |
| Attached console to report bus | `report.RaiseError` called in the daemon process, fix by child 12 P0a's convention | No: A-3 |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `cmd/`, `test/`, `docs/`; terms the messages "SSH session close", "SSH public key parse error", "config editor unavailable", "discard notice", "system command list refused", "SSH auth failure", the code `doctor-ssh-hostkey-missing`, and the `show doctor` JSON shape. Positive controls: `checkSSHHostKey` writes `doctor-ssh-hostkey-missing` and `docs/guide/health-checks.md` lists it; `recentEventsDiagnostic` writes the recent-event row; `readinessPayload` (`internal/component/doctor/answer.go`) writes the doctor answer. That no `.ci` outside this area reads the reworded session-close text is A-4.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| log line `code` attribute | writer | the SSH, `cli` and `cli/client` Warn and Error lines named in the ACs | no `code` attribute in these packages today (slice notes) | change: one code per line |
| same | reader | `internal/le/arch/doctorcoverage` `codeAttr`; an operator's log tooling | the gate reads `code` | change; pre-release (`ai/META-DECISIONS.md`) |
| "SSH session close" message | writer | `internal/component/ssh/ssh.go` `sessionCloseMiddleware` | reports the transcript close only | change: names the transcript (AC-10) |
| same | reader | a test or `.ci` reading the text | A-4 | change those readers with it |
| `show doctor` JSON | writer | `internal/component/doctor/answer.go` `readinessPayload` | the daemon answer | preserve the shape; new rows only |
| same | reader | `internal/component/doctor/local_route.go` `decodeDaemonDoctor`; MCP, REST, gRPC, web; an API client outside the repository | the local route decodes the same shape | preserve |

### Integration Points
- `internal/component/cli` package `init` - registers `cli-editor` and `cli-session-storage`
- `internal/component/cli/history.go`, `transcript.go`, `editor_commit_force.go` - write the session-storage record

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | local checks in `runChecks`, runtime checks in `RunRuntimeChecks`, transient rows through `recent-events` |
| No unintended coupling (components stay isolated) | Yes | the doctor component names no SSH or CLI symbol; each owner registers into it |
| No duplicated functionality (extends existing, does not recreate) | Yes | the server's store lookup, `NewCompleter`, `NewConfigValidator` and `NewEditorWithStorage` are called, never copied |
| Zero-copy preserved where applicable (refs, not copies) | Yes | no hot path: the record is written once per failed write |
| Registration over hardcoding, outbound | Yes | each check registers from its owner's `init()` |
| Registration over hardcoding, inbound | Yes | searched `internal/component/doctor` (no SSH or CLI name added), `internal/core/report` (no code list), the gate (reads the registry) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `cli` package's `init()` runs in the daemon process that answers `show doctor`, and SSH sessions write the record there | the daemon builds every SSH session model (`buildSessionModelFactory`) in its own process | the runtime row would never see a failure | `TestCLISessionStorageCheckRegistered` | unvalidated |
| A-2 | `NewEditorWithStorage` opens the active config without writing to the store | it reads the config and the draft state | `cli-editor` would mutate the store from doctor | read `NewEditorWithStorage` at P4; if it writes, the check opens through a read-only view of `ctx.Store` | unvalidated |
| A-3 | Child 12 P0a (shared codes, fix convention) lands before P3 | cross-child decision of the main thread; the bus itself (`internal/core/report`) already serves an in-daemon raiser | P4's raise would carry no fix, and P5's `doctor-internal-defect` lines no declared code | the Depends row; P3 starts after it | unvalidated |
| A-4 | No `.ci` outside this area reads the reworded session-close text | the audit slice grep found no reader | a test turns red on wording | grep `test/` at P3 | unvalidated |
| A-5 | `doctor-cli-transcript-write-failed` is admitted as a canonical code | not in the audit's code table: the transcript class appeared only when the producer was read | the main thread names another code | main thread decision before P3 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A brute force fills the report bus error ring with login events | `show errors` dominated by SSH login rows | moved with the login rows to child 4 (its R-1) |
| R-2 | Two children register the same code spelling | duplicate constant at compile | one spelling in `internal/core/diagnostic/codes.go`; this spec reuses the spellings children 4 and 12 own |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | `show doctor` rows wrong or missing; `ssh-host-key` stops warning on a healthy store-backed box |
| How is it reverted? | single commit revert per phase |
| Who else touches this path? | child 4 (management listener and login codes), child 9 (store classes), child 11 (hub editor and SSH boot lines), child 12 (raise route, audit code) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show doctor` on a daemon whose host key lives in the store | → | `checkSSHHostKey` over `ctx.Store` | `test/ui/doctor-ssh-hostkey-in-store.ci` |
| an editor session whose history write fails, then `show doctor` | → | session-storage record, `cli-session-storage` | `test/editor/editor-storage-write-failed-show-doctor.ci` |

## Acceptance Criteria

Every finding is a `Diagnostic` with code, severity, a message that names the user, key, path or cause in plain words, and `Fix` holding the exact command alone. "Runtime" means `show doctor` in the daemon (and `ze doctor` when the daemon answers); "local" means a post-config check, which runs on both routes. Codes this spec registers are named in AC-1; the others are registered by the child named in Depends and reused by spelling. Every log line named below carries its code in the `code` attribute.

### Codes (P1)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The codes this spec owns | `doctor-cli-editor-unavailable` (declared by `cli-editor`), `doctor-editor-storage-write-failed` and `doctor-cli-transcript-write-failed` (declared by `cli-session-storage`, A-5) each have one spelling in `internal/core/diagnostic/codes.go` and metadata that `ze show diagnostic <code>` answers | none: child 11 AC-22 depends on `doctor-cli-editor-unavailable` |

### SSH authentication and keys (P1, P2)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-2, AC-3, AC-6 | (removed) SSH login failures, the unreadable user list, the SSH audit record | owned by `plan/release/validation/spec-doctor-coverage-4-api-surfaces.md` AC-2, AC-11, AC-12 and AC-4 (Audit rows corrected) | the IDs stay unused so siblings' citations do not shift |
| AC-7 | The SSH host key or host certificate is held in the daemon store, not on the filesystem (local) | `ssh-host-key` reads as the server loads: the host key from `ctx.Store` only, as `resolveHostKeyOption` does, and the certificate from `ctx.Store` first and the filesystem otherwise, as `hostKeyWithCertOption` does; it reports nothing for a path the store holds; a certificate neither the store nor the filesystem holds stays Error, fix `delete environment ssh host-certificate`; a host key the store does not hold is an Info row "SSH host key `<path>` will be generated at the next start", code kept, whether or not a file sits at that path, because the server never reads one from disk (fixes journal row `helper-bypassed-by-an-open-coded-copy`, 2026-11-10) | a run with no store (`ctx.Store` nil) reports the host key as the Info row (the server generates one) and probes the filesystem for the certificate, its only source then |

### Editor session storage and transcripts (P3)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-8 | A history publication or a forced-commit discard notice cannot be written or read | `History.publish`, `AckDiscardNotice` and `startDiscardNotice` write the session-storage record (user, kind, path, error, time); the two discard Warn lines carry `doctor-editor-storage-write-failed`; `cli-session-storage` (runtime) reports Warning "the editor could not save `<kind>` for `<user>` at `<path>`: `<error>` (`<time>`)", fix `ze data check` | a later successful write of the same path clears the entry, because the store then holds the state again |
| AC-9 | A history write takes longer than the SSH drain limit, or `Stop` gives up waiting on one | the slow write is recorded as an AC-8 entry with the elapsed time; the `Stop` Error line carries `doctor-editor-storage-write-failed` | a stall seen only at stop is reported only by that line: the record lives in the process that stops, and the store that would keep it is the one that stalled; on the next start `disk-space` (`doctor-disk-space`) and child 9's store checks answer a full or damaged store |
| AC-10 | A recorded session's transcript cannot be opened, written or closed (`ze.cli.transcript` enabled) | `OpenTranscriptFile`, `TranscriptWriter.Record`, `writeHeader` and `Close` write the record (kind `transcript`); the session-close Warn line reads "SSH session transcript not closed" and carries `doctor-cli-transcript-write-failed`; `cli-session-storage` reports Warning "the transcript of `<user>`'s session is incomplete: `<path>`: `<error>`", fix the `doctor-disk-space` fix when the filesystem holding the directory is full, else `chmod 700 <directory>` | the stderr warning of `OpenTranscriptFile` stays for a client run outside the daemon, where no record is read |

### CLI attach and config mode (P4)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-11 | The editor cannot be built for any session (local) | `cli-editor` (post-config) builds the completer and the validator, then opens the active config from `ctx.Store` through `NewEditorWithStorage` without writing (A-2); on failure Error "config mode is unavailable on every session: `<cause>`", code `doctor-cli-editor-unavailable`; fix `ze config validate` when the active config does not open, `show system update` when the YANG schema of this build does not load (a defect of the build, whose fix is a release whose schema loads) | a cause bound to one user (an invalid user name, that user's saved draft) is AC-12's event, because doctor runs as no session |
| AC-12 | One session gets the command-only console | the `newAttachedModel` Warn line carries `doctor-cli-editor-unavailable` and raises an error event, source `cli`, subject the user, message naming the cause; child 11 AC-22 does the same on the hub's three lines | none |

### Not problem classes, and coverage (P5)

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-13 | The two not-problem groups | the removed-listener close line (`ssh/ssh.go` `Reconfigure`) and the two "system command list refused" lines (`cli/client/main.go` `buildRuntimeTree` and its dispatch twin) carry `doctor-internal-defect` and keep their fallbacks; the decode line's message says the client and the daemon may come from different builds | none: no line in this area reports nothing wrong, so none drops to Info |
| AC-14 | `./le arch doctor-coverage report` over `internal/component/ssh`, `internal/component/cli`, `internal/component/cli/client`, `internal/component/command`, `internal/component/cmd` | zero uncovered Warn or Error sites, the sibling-owned lines (`serveListener`, the SSH login-failure, user-list, audit and `matchPublicKey` lines) included once child 4 codes them | none |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCodesCommandsCLIRegistered` | `internal/core/diagnostic/codes_test.go` | AC-1 | |
| `TestSSHHostKeyCheckReadsStore` | `internal/component/ssh/doctor_test.go` | AC-7, store-held key and certificate give no row; a key file on disk the store lacks still gives the Info row; nil store gives the key Info row and probes the filesystem for the certificate | |
| `TestCLISessionStorageCheckRegistered` | `internal/component/cli/doctor_test.go` | A-1, the runtime check declares both codes | |
| `TestHistoryPublishFailureRecorded`, `TestDiscardNoticeFailureRecorded` | `internal/component/cli/history_test.go`, `editor_commit_force_test.go` | AC-8, record set and cleared | |
| `TestHistorySlowWriteRecorded` | `internal/component/cli/history_test.go` | AC-9 | |
| `TestTranscriptFailureRecorded` | `internal/component/cli/transcript_test.go` | AC-10 | |
| `TestCLIEditorCheck` | `internal/component/cli/doctor_test.go` | AC-11, unopenable config and the healthy case | |
| `TestAttachedModelEditorUnavailableRaises` | `internal/component/cli/client/attached_model_test.go` | AC-12 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| history write time against the drain limit | 0 to the limit | equal to the limit (not slow) | N/A | the limit plus one tick (recorded) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `doctor-ssh-hostkey-in-store` | `test/ui/*.ci` | a store-backed box reads no host key row | |
| `editor-storage-write-failed-show-doctor` | `test/editor/*.ci` | a failed history write shows with `ze data check` | |

### Interop Tests (Scope: protocol)
N-A: no protocol behaviour changes.

## Files to Modify
- `internal/core/diagnostic/codes.go` - the three codes and their metadata (AC-1)
- `internal/component/ssh/ssh.go` - the `Stop` drain line, the session-close wording and the `Reconfigure` close line (AC-9, AC-10, AC-13); the `recordAuthFailure` audit line, `pubkey.go` and `passwordauth.go` are child 4's (AC-4, AC-11 to AC-13 there)
- `internal/component/ssh/doctor.go` - the store read of `ssh-host-key` (AC-7)
- `internal/component/cli/history.go`, `transcript.go`, `editor_commit_force.go` - the session-storage record (AC-8 to AC-10)
- `internal/component/cli/client/main.go` - coded lines and the raise (AC-12, AC-13)
- `docs/architecture/doctor-and-health-checks.md` ("Where each owner registers its check"), `docs/guide/health-checks.md` (the code table), `docs/guide/config-editor.md` (history and transcript failures), `docs/architecture/system-architecture.md` (the SSH page the changed files' `// Design:` headers name) - the pages these behaviours change

## Files to Create
- `internal/component/cli/doctor.go` (+ test) - `cli-editor`, `cli-session-storage` and the session-storage record
- the three `.ci` files of the Wiring Test table

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no RPC or leaf added |
| YANG validation constraints | No | none added |
| YANG custom validators | No | the SSH public-key validator is child 4 AC-13's |
| CLI commands/flags | No | `ze doctor` and `show doctor` exist |
| CLI grammar (keyword before value) | N-A | no command added |
| Editor autocomplete | N-A | no leaf |
| Functional test for new RPC/API | Yes | the three `.ci` files above |
| Pipe completeness | N-A | the doctor answer's pipes are unchanged |
| Env var registration | No | `ze.cli.transcript` is already registered |
| Doctor check for runtime dependencies | Yes | `cli-editor`, `cli-session-storage`, codes in `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | No | the record is read by doctor, not exported |
| BGP family surface | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | doctor's feature row is the parent's |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | `show doctor` shape unchanged |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/health-checks.md`, `docs/guide/config-editor.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/doctor-and-health-checks.md` (the SSH and CLI checks) |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived at implementation by `./le spec citation anchors spec plan/release/validation/spec-doctor-coverage-8-commands-cli.md`. Declared by the changed files' `// Design:` headers: `docs/architecture/core-design.md` (the transcript section: updated with AC-10, a failed transcript write is now recorded), `docs/architecture/zefs-format.md` (unaffected: no store key is added or changed), `docs/features/ai-first.md` (unaffected: the doctor answer shape AI clients read is unchanged) |
| 17 | Existing docs show config/CLI/API examples for this area? | No | no config example changes meaning (the `public-keys` validator is child 4 AC-13's) |

## Implementation Steps

Each phase is sized for one agent of about 60 tool calls. P1 first; P3 needs child 12 P0a and the main thread's decision on A-5. P2 is removed: its SSH raises are child 4's (Audit rows corrected).

| Phase | Files | ACs | Needs |
|-------|-------|-----|-------|
| P1 codes and host key (wiring first) | `codes.go`, `ssh/doctor.go`; `doctor-ssh-hostkey-in-store.ci` | AC-1, AC-7 | - |
| P3 session storage | new `cli/doctor.go` (record and `cli-session-storage`), `history.go`, `transcript.go`, `editor_commit_force.go`, `ssh/ssh.go`; `editor-storage-write-failed-show-doctor.ci` | AC-8, AC-9, AC-10 | P1; child 12 P0a; A-5 |
| P4 editor availability | `cli/doctor.go` (`cli-editor`), `cli/client/main.go` | AC-11, AC-12 | P1; child 12 P0a (the raise's fix) |
| P5 remaining lines and coverage | `ssh/ssh.go`, `cli/client/main.go`, docs | AC-13, AC-14 | P1 to P4 |

Gates each phase owes, run by the main thread: the scoped package tests (`./le job run label <label> command go test -tags <tags> <package>` for `internal/component/ssh` with `ze_ssh`, `internal/component/cli`, `internal/component/cli/client`, `internal/core/diagnostic`), the named `.ci` tests, `./le arch doctor-coverage report`, and `./le repo compiles check` after each Go commit.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named symbol |
| Correctness | `ssh-host-key` consults the store in the order the server does |
| Naming | every new code starts `doctor-`; one spelling in `codes.go`; codes children 4 and 12 own are reused, never respelled |
| Data flow | the runtime check reads the record, never `Tree`; the record is written only by the producers named in AC-8 to AC-10 |
| Rule: user-facing-errors | each fix is one command on its own line |
| Rule: no-layering | the discarded write errors become record writes; no second copy of the store lookup; no public-key check beside child 4 AC-13's |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| checks registered | `TestCLISessionStorageCheckRegistered`, `TestCLIEditorCheck` |
| zero uncovered sites | `./le arch doctor-coverage report` over the five packages |
| the owned journal row fixed | its Fix cell turns `fixed:` with its test |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Secret leakage | no row or event prints a password, a key body or a transcript line |
| Brute force | AC-2 events fold per user and source host; the bus cap bounds them (R-1) |
| Fail open | `ssh-host-key` never reports a host key healthy that the server cannot load from the store |
| Authorization | `show doctor` keeps its existing authorization |

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
- The audit named the SSH session-close line "editor state not saved"; the model's `Close` returns only the transcript close, so the class is the transcript. A line is classed by the producer of its error, not by its place in the lifecycle.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `helper-bypassed-by-an-open-coded-copy.md` | 2026-11-10 | doctor `ssh-host-key` stats the filesystem, the server reads the store |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One process-wide session-storage record in `cli` | a report-bus raise per failed write | a failed history write repeats on every command; a record keyed by path holds the current state and clears on success, where a raise per write floods the ring |
| The transcript class gets its own code (A-5) | reuse `doctor-editor-storage-write-failed` | the transcript is a file outside the store, opt-in, with another fix; one code for two facts is the defect the code table exists to prevent |

## Known Limitations
- The SSH listener classes (boot and after start) are owned by `plan/release/validation/spec-doctor-coverage-11-hub-engine.md` (AC-8) and `plan/release/validation/spec-doctor-coverage-4-api-surfaces.md`; this spec adds no listener row.
- The SSH public-key class (verifier, `ssh-public-keys` check, `doctor-ssh-public-key-invalid`, the `matchPublicKey` Warn line) is owned by `plan/release/validation/spec-doctor-coverage-4-api-surfaces.md` AC-13; this spec adds no public-key check.
- The store classes (`doctor-store-integrity`, `doctor-store-permissions`, `doctor-store-staged-leftover`) are owned by `plan/release/validation/spec-doctor-coverage-9-config-yang.md`.
- The report-bus raise route, its fix convention and the shared codes (`doctor-internal-defect`, `doctor-audit-record-failed`) are built by `plan/release/validation/spec-doctor-coverage-12-plugin-infra.md` phases P0a and P0b; this spec raises through them and builds no route of its own.

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
- [ ] AC-1 and AC-7 to AC-14 all demonstrated (AC-2 to AC-6 are owned by child 4, Audit rows corrected)
- [ ] Targeted tests pass: `./le test functional ui`, `./le test functional editor`, `./le test functional plugin` (the named `.ci` files), and the scoped package tests of P1 to P5
- [ ] Every user story has a working path and a passing test (N-A: Scope cli, the Wiring Test table carries the paths)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: no protocol behaviour changes)
