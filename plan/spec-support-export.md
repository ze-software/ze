# Spec: support-export

> WARNING: Critical review is required before implementation and commit. Do not start code until the review resolves the syslog framing, support-owned config shape, and save-path security checks.

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | cli, config, tooling, docs |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Handoff | verify |
| Updated | 2026-08-17 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later Opus 5 session reviews that commit and closes. `-` closes in the same session. -->

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     Deferral shard: every deferred item lands there (ai/rules/planning.md)
     and closure must resolve its rows, so name the file from the start. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Operators need to save or send a complete support bundle during an incident without
remembering remote collection details. Ze already has the local `ze support`
tech-support equivalent. This spec adds a `save` pipe sink and a support-owned
destination list, so support output and other CLI output can be saved under the
Ze config/data directory, streamed to syslog, or copied by SSH.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - establishes small core plus registration.
  → Decision: Reuse the support component and command pipe machinery. Do not add a second support bundle producer.
  → Constraint: Command and config surfaces must register through their owning packages.
- [ ] `docs/guide/command-catalogue.md` - names command grammar categories and vendor equivalents.
  → Decision: The operator-facing feature is a `save` pipe sink plus a support archive producer. The catalogue's planned `generate tech-support archive` row must be reconciled with shipped `ze support`.
  → Constraint: User-supplied paths and remote names need a closed keyword before the value.
- [ ] `ai/patterns/cli-command.md` - structural template for CLI command grammar.
  → Decision: The pipe syntax uses keyword-value pairs: `save file path`, `save ssh destination`, and `save syslog destination`.
  → Constraint: All command output continues to support existing format and filter pipes.
- [ ] `ai/patterns/config-option.md` - structural template for YANG-backed configuration.
  → Decision: Remote destinations are named config entries. SSH credentials are referenced by name or existing SSH credential storage, not embedded as raw private keys in visible config.
  → Constraint: YANG validation handles type, length, enumeration, and pattern rules before custom validators.
- [ ] `internal/core/paths/paths.go` - resolves Ze config/data directory across Linux, gokrazy appliance, and explicit `ze.config.dir`.
  → Decision: Relative local save paths resolve under `paths.DefaultConfigDir()`.
  → Constraint: Local save must reject path traversal outside the resolved directory.

### RFC Summaries (Scope: protocol)
- [ ] N-A - this feature copies local files or formatted command output. It does not change a protocol Ze implements.
  → Constraint: No RFC documentation block applies.

**Key insights:**
- Existing support collection lives in `internal/component/support.Run`. It writes a local tar.gz bundle and can print a JSON manifest, but it does not use the command pipe engine.
- Existing pipe parsing lives in `internal/component/command/pipe.go`. It handles text and JSON transformations, not binary archive delivery.
- The interactive CLI, offline `ze pipe`, web CLI, and SSH exec paths each call pipe helpers at different layers. A `save` side-effect sink must be added once and reached by each intended entry point.
- SSH CLI credentials already live in ZeFS under `meta/ssh/{host}/{port}/...` and are read by `internal/core/ssh/client`. Config should reference stored material instead of carrying private key bytes.
- Syslog currently has one scalar `environment log destination` for daemon logging. Support export needs a named destination list so incident destinations do not overwrite daemon logging.
- `system archive` already has named config archive destinations with URL locations. It is a precedent for naming and validation, not an automatic reuse point, because its trigger and file naming semantics are config-archive-specific.
- `paths.DefaultConfigDir()` maps gokrazy `/user/ze` to `/perm/ze`, standard system prefixes to `/etc/ze`, and honors explicit `ze.config.dir`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/support/support.go` - `Run` parses `ze support` flags, collects modules, writes an archive with `writeArchive`, and optionally prints `SupportManifest` JSON.
- [ ] `internal/component/support/modules.go` - `moduleRegistry`, `ModuleNames`, and `filterModules` define the available support bundle modules.
- [ ] `internal/plugins/support/register.go` - registers root command `support` with help text and offline mode.
- [ ] `cmd/ze/ze_core_dispatch.go` - registers root handlers and dispatches local and YANG command paths.
- [ ] `internal/component/command/pipe.go` - parses and applies existing pipe operators such as json, table, yaml, match, count, first, last, resolve, log, and origin.
- [ ] `cmd/ze/ze_core_pipe.go` - implements the offline `ze pipe` command by applying pipe operators to stdin.
- [ ] `internal/component/cli/client/main.go` - interactive CLI calls `command.ProcessPipesChecked` before sending a command to the daemon.
- [ ] `internal/component/cli/model_mode.go` - TUI model calls `command.ProcessPipesChecked` and applies the returned formatter to command output.
- [ ] `internal/component/web/cli_terminal.go` - web CLI uses `ProcessPipesDefaultFormatChecked` before dispatching operational commands.
- [ ] `internal/core/ssh/client/client.go` - reads stored SSH credentials and executes or streams commands through SSH.
- [ ] `internal/plugins/connect/main.go` - writes and lists stored SSH remote credentials in ZeFS.
- [ ] `pkg/zefs/keys.go` - defines SSH credential ZeFS key patterns.
- [ ] `internal/component/config/mask.go` - masks `ze:sensitive` and `ze:bcrypt` leaves in config display.
- [ ] `internal/core/slogutil/syslog.go` - creates a syslog handler from a destination address.
- [ ] `internal/component/hub/yang/ze-hub-conf.yang` - defines existing daemon log backend and destination leaves.
- [ ] `internal/component/config/system/yang/ze-system-conf.yang` - defines existing named `system archive` destinations for config archival.
- [ ] `internal/core/paths/paths.go` - resolves the Ze config/data directory.
- [ ] `internal/component/doctor/checks_storage.go` - verifies writable persistent destinations.
- [ ] `internal/component/doctor/checks_tls.go` - shows the existing doctor pattern for stored material referenced by config.
- [ ] `docs/guide/command-reference.md` - documents `ze support` as the support archive command.
- [ ] `docs/features.md` - marks Tech-Support Bundle as supported and describes current support modules.
- [ ] `docs/guide/command-catalogue.md` - still marks Tech-support archive as planned and names vendor equivalents.

**Behavior to preserve:**
- `ze support` remains the local tech-support archive command and continues to collect the current module set.
- Support archives remain privacy-by-default. Sensitive config and secrets stay redacted unless `--sensitive` is explicitly set.
- Existing pipe operators keep their current semantics and ordering.
- Existing SSH credential commands keep their storage format and precedence rules.
- Existing daemon logging configuration continues to configure daemon logs only.

**Behavior to change:**
- Add a `save` pipe sink that can write command output or a support archive to a local relative path, syslog destination, or SSH destination.
- Add support-owned named SSH and syslog destinations to config.
- Let operators use a configured destination during an incident without retyping host, port, path, facility, or credential details.
- Update docs so shipped `ze support` and the catalogue agree.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Interactive CLI input, web CLI input, SSH exec input, and offline `ze pipe` stdin can contain a `save` pipe operator.
- `ze support` remains an offline root command that produces a support archive path and manifest.
- Config enters through YANG-backed `support` configuration for named export destinations.

### Transformation Path
1. Pipe parsing in `internal/component/command/pipe.go` recognizes `save` as a terminal sink and records the destination kind and keyword-value arguments.
2. Normal command execution renders text or JSON as it does today. A support archive execution produces an artifact path plus manifest.
3. The save sink receives either rendered bytes or artifact bytes. It writes once to the chosen destination.
4. Local file save resolves a relative path under `paths.DefaultConfigDir()`, rejects empty, absolute, or escaping paths, and creates the target file or directory according to the command grammar.
5. Syslog save resolves a named destination or explicit address and streams the same bytes the sink would send to file or SSH. If the payload needs framing or chunking, the stream includes enough metadata for reconstruction.
6. SSH save resolves a named destination or explicit host/path, loads stored credentials through `internal/core/ssh/client`, and streams the bytes to the remote path without logging secrets.
7. Result output tells the operator where the data was saved. Error output names the failing destination and operation.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI input to pipe parser | `ProcessPipesChecked` and `ProcessPipesDefaultFormatChecked` parse `save` | Unit tests in `internal/component/command/pipe_test.go` |
| Offline command to support component | `ze support` continues to call `support.Run` | Functional UI test for local save |
| Config to destination resolver | YANG tree parses named destinations into a typed resolver | Config parse test and unit resolver test |
| Destination resolver to ZeFS SSH credentials | Named SSH destination references stored credential material | Unit test with fake store |
| Save sink to filesystem | Writer resolves a relative path under `paths.DefaultConfigDir()` and creates the target | Unit and functional test |
| Save sink to syslog | Writer streams full payload bytes with framing or chunking when required | Unit test with fake syslog dialer |
| Save sink to SSH | Writer streams to remote path using stored credentials | Unit test with fake SSH writer and functional command test with a local fake target if infrastructure exists |

### Integration Points
- `internal/component/command/pipe.go` - add parse, validation, and metadata for `save`.
- `cmd/ze/ze_core_pipe.go` - let offline stdin output use the new sink.
- `internal/component/cli/client/main.go`, `internal/component/cli/model_mode.go`, and `internal/component/web/cli_terminal.go` - ensure each CLI path executes the sink exactly once.
- `internal/component/support/support.go` - expose archive artifact metadata so a pipe sink can send the archive, not only the printed manifest.
- `internal/component/config/system/yang/ze-system-conf.yang` - reuse the existing archive naming pattern where it fits, but do not make support export depend on config-archive triggers.
- `internal/core/ssh/client` - reuse existing credential loading and streaming primitives.
- `internal/component/doctor` - add checks for configured file paths, syslog endpoints, SSH destinations, and referenced credentials.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Save extends `internal/component/command/pipe.go`; support collection remains in `internal/component/support` |
| No unintended coupling (components stay isolated) | Needs design review | Support export needs SSH and syslog writers. Place the writer behind an interface owned by the command or support component, not in core dispatch |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `support.Run`, `moduleRegistry`, pipe parsing, and SSH credential storage |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Support archive delivery is file streaming. Normal command output is already a rendered string |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Needs design review | Config roots and commands must be owned by the support or command component, not by central dispatch switches |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `ze support` is the existing tech-support equivalent to extend. | User said show tech-support or equivalent; docs and support source show `ze support` generates the archive. | A separate `show tech-support` producer would duplicate support collection. | Source read of `support.Run` and docs. | validated |
| A-2 | Config must store named SSH and syslog destinations, but not raw private key bytes. | User asked for pre-saved locations and saved SSH credential key; SSH credentials already use ZeFS. | Visible config could leak secrets or force key material through config diffs. | Source read of `zefs` SSH keys and config masking. | validated |
| A-3 | Local file save uses a path relative to Ze config/data directory. The CLI grammar remains `save file path <relative-path>` because repo CLI rules require `path` before a user value. | User selected relative local paths; `paths.DefaultConfigDir()` maps Linux and appliance locations. | Saving to cwd or arbitrary absolute paths would behave differently on Linux and appliances. | User confirmation and source read of `paths.DefaultConfigDir()`. | validated |
| A-4 | `save` is a terminal pipe operator. | A save sink has side effects and does not produce data for later pipe transforms. | Chaining after save could run transformations on a status message instead of the original data. | Source review of pipe processors and design review; parser tests enforce it during implementation. | validated |
| A-5 | Syslog export streams the same payload data as file and SSH, regardless of format. | User selected streaming data to syslog whatever the format. | Syslog needs framing or chunking for large archives, and receiver-side reconstruction must be documented. | User confirmation plus syslog framing tests. | validated |
| A-6 | Support export gets its own named destination list. Existing `system archive` destinations are only a naming precedent. | User selected support export list; `ze-system-conf.yang` names config archive locations and triggers. | Reusing config archive entries could make support export inherit commit/daily/hourly behavior that incident export does not need. | User confirmation and design review against config-archive source. | validated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A pipe save writes formatted manifest text instead of the support tar.gz. | Functional test finds remote file contains JSON manifest only. | Add artifact metadata from support and make save prefer artifact bytes when present. |
| R-2 | Configured credentials leak through `show configuration`, logs, or errors. | Tests find key bytes or passwords in output. | Store secret material in ZeFS and config stores only names. Mask any sensitive leaves. |
| R-3 | Syslog is a difficult transport for a tar.gz archive. | Payload exceeds message size or becomes unreadable after chunking. | Stream full data as requested, but make framing explicit, bounded, tested, and documented. |
| R-4 | Save side effects run twice in web or TUI paths. | Functional test observes duplicate files or syslog messages. | Execute sinks in one pipe application layer only. |
| R-5 | Remote SSH save blocks the interactive CLI without progress. | Manual smoke test hangs on unreachable host. | Use timeouts, clear errors, and no credential prompt in non-interactive paths. |
| R-6 | Command catalogue remains stale. | Docs still say tech-support archive is planned after code lands. | Update `docs/guide/command-catalogue.md` with the shipped Ze command shape. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | CLI pipe behavior, support bundle delivery, config validation, and incident workflows. It does not alter BGP, config commit semantics, or daemon protocol behavior. |
| How is it reverted? | Single feature commit revert if no config migration is added. If new config syntax ships, retain parser compatibility or document migration in the revert plan. |
| Who else touches this path? | CLI, web CLI, support component, config schema, doctor checks, SSH credential storage, syslog logging utility, and command docs. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze pipe save file path <relative-path>` with stdin | → | save sink writes local bytes under Ze config/data directory | `TestRunPipeSaveFileWritesInput` and `test/ui/pipe-save-file.ci` | <!-- doc-links: ignore (planned by this spec, written when implemented) -->
| interactive command with `| save file path <relative-path>` | → | CLI pipe parser and sink run through command execution | `test/ui/pipe-save-command-output.ci` | <!-- doc-links: ignore (planned by this spec, written when implemented) -->
| support archive command with `| save file path <relative-path>` | → | support archive metadata reaches save sink | `test/ui/support-save-file.ci` | <!-- doc-links: ignore (planned by this spec, written when implemented) -->
| config with a named SSH destination | → | destination resolver loads config and referenced credential name | `TestSupportExportSSHConfiguredDestinationResolves` |
| config with a named syslog destination | → | destination resolver loads config and syslog endpoint | `TestSupportExportSyslogConfiguredDestinationResolves` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Operator runs a supported command with `| save file path <relative-path>` | Ze writes the command output under the resolved Ze config/data directory and prints a success message naming the file. |
| AC-2 | Operator runs the support bundle command with `| save file path <relative-path>` | Ze writes the support archive bytes under the resolved Ze config/data directory, not only the manifest text. |
| AC-3 | Operator runs a supported command or support bundle with `| save syslog destination <name>` | Ze resolves the support-owned named syslog destination and streams the full payload there with documented framing when required. |
| AC-4 | Operator runs a supported command with `| save ssh destination <name>` | Ze resolves the named SSH destination and stored credential reference, then writes the output to the configured remote path. |
| AC-5 | Operator supplies an ad-hoc SSH destination with closed keywords | Ze accepts host, port, user, path, and credential reference only through keywords, never through ambiguous positionals. |
| AC-6 | A configured destination references missing credential material | Validation or doctor reports a clear error naming the destination and missing reference. |
| AC-7 | A configured destination has a malformed host, port, syslog address, or path | Config parsing or validation rejects it before use. |
| AC-13 | Operator supplies an empty, absolute, or escaping local file path | Ze rejects it and names the invalid path constraint. |
| AC-8 | Command output includes secrets or sensitive config by default | Save output uses the same redaction behavior as the command being saved. `ze support --sensitive` remains the explicit opt-in for secrets. |
| AC-9 | Save delivery fails after the command output is produced | Ze returns a non-zero result for the save operation and names the failed destination and cause. |
| AC-10 | Existing pipe operators are used without `save` | Existing behavior and tests remain unchanged. |
| AC-11 | A user asks for help or completion | Help and completion list `save` syntax and configured destination names where the CLI supports dynamic completion. |
| AC-12 | Documentation describes tech-support export | Command reference, feature table, and command catalogue show the shipped command shape and named destination configuration. |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Saves support archive locally during an incident | CLI command -> support collector -> artifact metadata -> save file sink | `test/ui/support-save-file.ci` | <!-- doc-links: ignore (planned by this spec, written when implemented) -->
| 2 | Sends show output to configured syslog | CLI command -> pipe parser -> destination resolver -> syslog writer | `test/ui/pipe-save-syslog.ci` or unit-backed functional substitute if syslog capture is unavailable | <!-- doc-links: ignore (planned by this spec, written when implemented) -->
| 3 | Sends support archive to configured SSH destination | CLI command -> support collector -> destination resolver -> SSH credential loader -> SSH writer | `test/ui/support-save-ssh.ci` or fake SSH integration test if a daemon fixture exists | <!-- doc-links: ignore (planned by this spec, written when implemented) -->
| 4 | Admin stores a remote support destination in config | YANG parse -> validator -> doctor check -> completion/help | `test/parse/support-export-destination.ci` | <!-- doc-links: ignore (planned by this spec, written when implemented) -->

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParsePipeSaveFile` | `internal/component/command/pipe_test.go` | Parser accepts `save file path <path>` and records a terminal sink | planned |
| `TestParsePipeSaveRejectsMissingKeyword` | `internal/component/command/pipe_test.go` | Parser rejects `save file <path>` to preserve keyword-before-value grammar | planned |
| `TestApplyPipesSaveTerminal` | `internal/component/command/pipe_test.go` | Later transforms after save are rejected or never accepted by grammar | planned |
| `TestSupportRunExposesArchiveArtifact` | `internal/component/support/support_test.go` | Support command exposes archive path for the save sink | planned |
| `TestSupportExportDestinationParse` | owning config package test | YANG config parses named file, syslog, and SSH destinations | planned |
| `TestSupportExportSecretsMasked` | owning config package test | Credential references do not reveal private key or password material | planned |
| `TestSupportExportDoctorMissingCredential` | `internal/component/doctor/doctor_test.go` | Doctor reports missing stored credential references | planned |
| `TestSaveSSHUsesStoredCredential` | owning save package test | SSH writer loads stored credentials and never prompts in non-interactive paths | planned |
| `TestSaveSyslogBoundsPayload` | owning save package test | Syslog writer enforces the design's size or chunking rule | planned |
| `TestSaveFileResolvesConfigDir` | owning save package test | Relative local paths resolve under `paths.DefaultConfigDir()` | planned |
| `TestSaveFileRejectsTraversal` | owning save package test | Empty, absolute, and `..` paths are rejected | planned |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| SSH port | 1-65535 | 65535 | 0 | 65536 |
| Destination name length | 1-64 | 64 characters | empty | 65 characters |
| Syslog message chunk size if chunking is chosen | design-defined | design-defined | 0 | above configured maximum |
| Local path length | OS-limited under `paths.DefaultConfigDir()` | valid relative path under test config directory | empty path, absolute path, or `..` escape | path too long if validation defines a bound |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pipe-save-file` | `test/ui/pipe-save-file.ci` | User pipes JSON stdin through `ze pipe save file path <tmp>` and file contains original data | planned | <!-- doc-links: ignore (planned by this spec, written when implemented) -->
| `pipe-save-command-output` | `test/ui/pipe-save-command-output.ci` | User saves a simple command output through CLI pipe | planned | <!-- doc-links: ignore (planned by this spec, written when implemented) -->
| `support-save-file` | `test/ui/support-save-file.ci` | User saves support archive through save pipe and archive exists with expected manifest entry | planned | <!-- doc-links: ignore (planned by this spec, written when implemented) -->
| `support-export-destination` | `test/parse/support-export-destination.ci` | Config with named SSH and syslog destinations parses and rejects malformed values | planned | <!-- doc-links: ignore (planned by this spec, written when implemented) -->
| `pipe-save-completion` | `test/ui/pipe-save-completion.ci` | Completion or help advertises `save` and destination subcommands | planned | <!-- doc-links: ignore (planned by this spec, written when implemented) -->
| `support-save-ssh` | `test/ui/support-save-ssh.ci` or `test/integration/support-save-ssh.ci` | User sends output to a fake or local SSH destination using stored credentials | planned | <!-- doc-links: ignore (planned by this spec, written when implemented) -->
| `pipe-save-syslog` | `test/ui/pipe-save-syslog.ci` or unit-backed integration fixture | User sends output to configured syslog destination and receiver observes it | planned | <!-- doc-links: ignore (planned by this spec, written when implemented) -->

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | No protocol behavior changes | N-A |

## Files to Modify

- `internal/component/command/pipe.go` - parse and represent `save` as a terminal sink.
- `internal/component/command/pipe_test.go` - unit coverage for syntax, validation, and existing pipe preservation.
- `cmd/ze/ze_core_pipe.go` - offline stdin save path.
- `cmd/ze/ze_core_dispatch.go` - only if local support pipe handling must be routed before a registered root handler; avoid central feature switches if a registry hook can do it.
- `internal/component/cli/client/main.go` - SSH exec CLI save path.
- `internal/component/cli/model_mode.go` - TUI save path.
- `internal/component/web/cli_terminal.go` - web terminal save path or explicit unsupported error if browser file writes are not reachable.
- `internal/component/support/support.go` - expose archive metadata for save.
- `internal/component/support/modules.go` - preserve module registry and module list behavior.
- `internal/plugins/support/register.go` - help text and command metadata.
- `internal/component/config/yang` or `internal/component/support/yang` - support export destination schema. <!-- doc-links: ignore (planned by this spec, written when implemented) -->
- `internal/component/config/validators.go` or owning validator file - destination validation if native YANG rules are insufficient.
- `internal/component/doctor` - destination and credential checks.
- `internal/core/ssh/client/client.go` - streaming or non-interactive write helper if existing functions cannot write a file without executing an unsafe shell.
- `internal/core/slogutil/syslog.go` - reuse or safely factor syslog dialing for support export.
- `docs/guide/command-reference.md` - document save pipe and support export syntax.
- `docs/features.md` - update Tech-Support Bundle row.
- `docs/guide/command-catalogue.md` - replace planned tech-support row with shipped Ze syntax.

## Files to Create
- `internal/component/command/save.go` or owning package equivalent - save sink implementation if keeping it in `pipe.go` would mix parsing and I/O. <!-- doc-links: ignore (planned by this spec, written when implemented) -->
- `internal/component/command/save_test.go` or owning package equivalent - sink unit tests. <!-- doc-links: ignore (planned by this spec, written when implemented) -->
- `internal/component/support/yang/ze-support-conf.yang` or config-owned YANG module - named support export destinations if no existing support config root exists. <!-- doc-links: ignore (planned by this spec, written when implemented) -->
- `test/ui/pipe-save-file.ci` - local file save functional test. <!-- doc-links: ignore (planned by this spec, written when implemented) -->
- `test/ui/pipe-save-command-output.ci` - command output save functional test. <!-- doc-links: ignore (planned by this spec, written when implemented) -->
- `test/ui/support-save-file.ci` - support archive save functional test. <!-- doc-links: ignore (planned by this spec, written when implemented) -->
- `test/parse/support-export-destination.ci` - config syntax test. <!-- doc-links: ignore (planned by this spec, written when implemented) -->
- `test/ui/pipe-save-completion.ci` - help or completion test. <!-- doc-links: ignore (planned by this spec, written when implemented) -->

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | Support-owned export destinations need a YANG config root. Existing `system archive` is a naming precedent only. |
| YANG validation constraints | Yes | Destination name, kind, host, port, relative path, and syslog address need native constraints. |
| YANG custom validators | Yes | Credential references and writable path or endpoint reachability need custom validation or doctor checks. |
| CLI commands/flags | Yes | `save` pipe syntax and support help text change. |
| CLI grammar (keyword before value) | Yes | `save file path <relative-path>`, `save ssh destination <name>`, `save syslog destination <name>`. |
| Editor autocomplete | Yes | Config destination names should complete where the CLI has dynamic completion. |
| Functional test for new RPC/API | N-A | No new RPC or plugin API is required. |
| Pipe completeness | Yes | Every command output pipe path must still support existing operators and new `save`. |
| Env var registration | No | This feature should use config and ZeFS, not environment variables. |
| Doctor check for runtime dependencies | Yes | SSH destinations, syslog endpoints, local writable directories, and missing credentials are runtime dependencies. |
| Prometheus counters/metrics | No | One-shot export does not add persistent observable state. |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP family behavior changes. |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, config syntax docs if support config gets a new root |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | No | N-A |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` only if support plugin documentation enumerates root commands |
| 6 | Has a user guide page? | Yes | Add or update support section in `docs/guide/command-reference.md`; separate guide only if examples outgrow the reference |
| 7 | Wire format changed? | No | N-A |
| 8 | Plugin SDK/protocol changed? | No | N-A |
| 9 | RFC behavior implemented, changed, or newly proven? | No | N-A |
| 10 | Test infrastructure changed? | Maybe | Only if syslog or SSH fixture support is added |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if tech-support feature comparison exists |
| 12 | Internal architecture changed? | Maybe | `docs/architecture/core-design.md` only if a new shared save sink package is introduced |
| 13 | Route metadata keys added/changed? | No | N-A |
| 14 | Prometheus counters added/changed? | No | N-A |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md` or generated command catalogue if it lists registered commands |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep docs for source anchors to changed support, command, and dispatch files |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/command-reference.md`, `docs/guide/command-catalogue.md`, `docs/features.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - add failing tests for pipe syntax, local file save, support artifact save, and config destination parsing.
   - Tests: `TestParsePipeSaveFile`, `TestRunPipeSaveFileWritesInput`, `test/ui/support-save-file.ci`, `test/parse/support-export-destination.ci` <!-- doc-links: ignore (planned by this spec, written when implemented) -->
   - Files: `internal/component/command/pipe_test.go`, `cmd/ze/ze_core_pipe.go`, support config test files
   - Verify: tests fail because `save` is unknown or artifact metadata is absent.
2. **Phase: Destination model** - add YANG schema, typed resolver, validation, and doctor checks for named SSH and syslog destinations.
   - Tests: destination parse, missing credential, malformed address, doctor diagnostics
   - Files: support or config YANG, validators, doctor checks
   - Verify: config tests and doctor unit tests pass.
3. **Phase: Save sink** - implement local file, syslog, and SSH writers behind a small interface.
   - Tests: save sink unit tests with fake filesystem, fake syslog dialer, fake SSH writer
   - Files: command or support save implementation files
   - Verify: unit tests pass and no secret appears in captured logs or errors.
4. **Phase: CLI integration** - connect save sink to offline `ze pipe`, interactive SSH/TUI CLI, web CLI, and support archive artifact metadata.
   - Tests: functional UI tests and support save test
   - Files: pipe users, support runner, help metadata
   - Verify: functional tests pass through user entry points.
5. **Phase: Docs and generated surfaces** - update command docs, feature row, command catalogue, and any generated help data.
   - Tests: `make ze-doc-verify`, `make ze-doc-wiring-check`, and any generated-file check required by changed files
   - Files: docs listed above and generated outputs if required
   - Verify: doc checks pass.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation evidence and a test path. |
| Feature completeness | Support archive bytes, normal command output, local file, SSH, and syslog all have end-to-end evidence or an explicit approved design limitation. |
| Correctness | `save` is terminal, writes exactly once, and reports destination failures as command failures. |
| Naming | CLI syntax uses closed keywords before values. Config names align across YANG, Go structs, and docs. |
| Data flow | Support collection remains in support component. Pipe parsing remains in command component. Core dispatch does not gain a per-feature switch unless no registered hook exists. |
| Rule: cli | Every output command keeps existing pipe support; `save` does not break json, yaml, table, text, match, count, first, last, resolve, log, or origin. |
| Rule: config | Secret material is not stored in visible config. Native YANG constraints are used before custom code. |
| Rule: repo-maintenance | Doctor checks and docs exist for new runtime dependencies. |
| Rule: testing | Functional tests exercise user entry points and prove tests fail on a broken save path. |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `save` pipe syntax documented by parser tests | `make ze-unit-pkg-test PKG=./internal/component/command` |
| Local file save works through user entry point | `make ze-functional-ui-test` with `pipe-save-file` pattern |
| Support archive save writes archive bytes | `make ze-functional-ui-test` with `support-save-file` pattern |
| Named destinations parse and validate | `make ze-functional-parse-test` with `support-export-destination` pattern |
| SSH credential reference is checked | owning unit test and doctor test |
| Syslog behavior is bounded and tested | owning unit test and functional or fixture-backed test |
| Documentation reflects shipped command shape | `make ze-doc-verify` and `make ze-doc-wiring-check` |
| Changed Go code is linted | `make ze-lint-changed` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | Destination name, host, port, relative path, syslog address, and credential reference fail closed with useful errors. |
| Secret handling | Private keys, passwords, and `--sensitive` support bundle data do not leak to logs, config display, docs examples, or error output. |
| Remote command injection | SSH save does not build an unquoted remote shell command with user-controlled path. Prefer SFTP or a constrained write protocol. |
| File permissions | Local files are created under the Ze config/data directory with restrictive permissions for support archives. |
| Resource exhaustion | Syslog and SSH paths stream data and set limits or timeouts. No unbounded buffering of large archives. |
| Authorization | Web CLI and SSH CLI paths run under the same user identity and authorization rules as the command being saved. |
| Partial writes | Failures leave a clear error and do not claim success. Local writes use temp file plus rename if feasible. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood, return to RESEARCH |
| Lint failure | Fix inline. If architectural, return to DESIGN |
| Functional test fails | Check the AC: wrong AC means DESIGN, correct AC means IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- `ze support` is already the support bundle producer. Creating `show tech-support` as a separate producer would violate the no-duplication rule.
- The existing pipe engine is text and JSON oriented. Support archive delivery needs artifact metadata or a direct support destination path, otherwise the save sink can save only the manifest.
- Syslog is the least natural target for a full tar.gz archive. It is reliable for a text summary or manifest, but binary archive delivery needs chunking, encoding, or a narrower supported mode.
- SSH credentials are already a ZeFS concern. Config should name a destination and credential reference, not own the private key payload.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Extend `ze support`; do not create a second tech-support collector | Add `show tech-support` with a separate collector | Existing `support.Run` already owns collection, manifest, redaction, and archive writing. |
| Use `save` as a terminal pipe sink | Let later pipe operators transform the save success message | Side effects must happen once and on the intended bytes. Chaining after save is ambiguous. |
| Require closed keywords before values | Accept `save file <path>` because the user phrased it that way | Repo CLI rule requires a closed keyword before any user-supplied value. |
| Store remote destinations in config and credentials in ZeFS | Store private key bytes in YANG config | Existing SSH credential storage avoids secret leakage in config display and diffs. |
| Keep daemon log destination separate from support syslog destinations | Reuse `environment log destination` for support export | Daemon logging and support export have different lifecycles and should not overwrite each other. |

## Known Limitations

- Syslog support must stream the full payload as requested. The implementation must define framing, chunking, and reassembly metadata before code is written.
- Web CLI local file save can only write on the Ze host, not the browser machine. If that is unacceptable, the web surface needs a separate download path.
- This spec does not add a new protocol or change BGP behavior.

## RFC Documentation (Scope: protocol)

N-A. No protocol behavior changes.
## Checklist

#### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row has a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes
- [ ] Every A-N assumption confirmed or broken, none unvalidated
- [ ] Deferral shard resolved: no live row without a destination

#### TDD
- [ ] Tests written
- [ ] Tests FAIL, with output pasted into implementation notes
- [ ] Implementation begins only after tests prove the gap
- [ ] Tests PASS, output pasted into implementation notes
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
