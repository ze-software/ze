# Spec: data-store-maintenance-through-the-daemon

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Priority | P1 |
| Release reason | A shipped `ze doctor` report tells the operator to remove a store key with `ze data rm`, and that command is refused while the daemon that answered the report runs, so the warning cannot be cleared on a running box. The owner rule that every operator command runs through the daemon makes the daemon route owed work. |
| RFC gaps | - |
| Depends | `spec-single-owner-lifecycle-5-session-end-locals` (closed) (its fix (a) rewrites the same FlowSpec report for the configured case; this spec edits the removed-section text after that lands). The FlowSpec record and its two checks landed in bec9096156. |
| Phase | 5/7 (phases 1 to 4 committed, phase 7 in test, phases 5 and 6 remain; claimed above the WIP cap on Thomas's order of 2026-11-14, a one-off exception for this spec: 16 in flight against a cap of 12) |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The DDoS FlowSpec plugin records the rule it announces under the store key
`meta/ddos-flowspec/announcement` (bec9096156). While the `ddos flowspec`
section is not configured, the registered check `orphanRecordCheck`
(`internal/plugins/ddos/flowspec/doctor.go` `checkOrphanRecord`) reports a
stored record as `doctor-ddos-flowspec-orphan-announcement`. Its fix withdraws
the rule at the peers, and its message tells the operator to remove the record
with `ze data rm meta/ddos-flowspec/announcement`.

What the operator meets: on a box where the daemon runs, `ze data rm` is
refused with "store is owned by another process: `<dir>`; close its owner
before maintenance". No other command removes the key, so the warning stays
until the operator stops the daemon. A record removed while the daemon is
stopped clears the report, but the operator has to take the box down to clear
a warning.

Thomas decided on 2026-11-12 who clears an orphan FlowSpec record (AC-16 of
`spec-single-owner-lifecycle-5-session-end-locals`). The report's fix withdrew
the rule at the peers and nothing then removed the record, so the warning stood
after the operator did what it said. His words: "ok a + c".

- (a) while the section is configured, the fix is
  `ze request plugin restart ddos-flowspec`: the start path withdraws the
  recorded rule from its own record and removes it. Built in `85507073e8`
  under that spec (closed, `plan/learned/046-session-end-locals.md`).
  Not this spec.
- (c) when the section has been removed, the operator withdraws with the
  command the report prints, then removes the record with `ze data rm`
  through the running daemon. This spec builds (c). The removed-section half
  of that spec's AC-16 ("The report clears when the record is removed") moves
  here.

The owner rule it is owed by (`ai/META-DECISIONS.md`, "Goal and scope"):
"Every operator command runs through the daemon on every command surface
(MCP, REST, gRPC, SSH, web) with the same output as the local `ze` command,
including the ones that answer without a daemon (`ze doctor`, `show config`,
`show yang` and the rest), so a remote operator or AI can debug the box. The
local route stays for when the daemon is unreachable. A command whose result
lives on the caller's side gets a daemon form designed for it, never an
exclusion."

### Scope and design, as Thomas decided them on 2026-11-14

The spec builds the daemon route of every `ze data` verb that lacks one. Five
daemon commands, all acting on the store handle the daemon already publishes to
`request data backup`:

| Local command | Daemon command | What it does through the daemon |
|---------------|----------------|---------------------------------|
| `ze data rm <key>...` | `delete data key <key>` | Removes one runtime-state key |
| `ze data write <key> <file>` | `update data key <key> content <base64>` | Writes one runtime-state key, creating or replacing it |
| `ze data import <file>...` | `update data config name <name> content <base64>` | Imports one config as a new version, through the reload when it is the daemon's own config |
| `ze data repair --output <path>` | `request data repair path <absolute-output>` | Writes a salvage copy of the store on the daemon's host |
| `ze data check` | `show data check` | Answers the store's integrity verdict |

Four rules bind them:

1. Runtime state only for the two key commands: a key under `meta/` outside
   `meta/config/`. A configuration key is refused, and the refusal names the
   command that manages configuration.
2. A key whose owning plugin runs is refused. The store knows which process
   name each runtime key is granted to, and the daemon knows which plugin
   processes run. The refusal names the plugin and the command that makes it
   manage its own state. No command names a feature.
3. Caller-side bytes travel base64-encoded in the `content` argument, on every
   surface. The content never reaches the audit log or the accounting record.
4. With a daemon running, the local `ze data rm`, `ze data write` and
   `ze data import` send their work to the daemon themselves and print what the
   local route prints. With no daemon they act locally, as today. `ze data
   repair` and `ze data check` are not refused beside a daemon, so they stay
   local.

The FlowSpec report then names one removal command that works in both states.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - where a command is served; "Served by the daemon too" and "Local commands already served by the daemon under the same path"
  → Constraint: a local-data command (`command.MustRegisterLocalData`) is answered by `ze cli -c` in the caller's own process before the daemon is asked, so a store mutation registered that way would open a second writer and be refused. A mutation of the live store is a builtin RPC with a `ze:command` YANG node, as `request data backup` is.
  → Constraint: authorization reads the verb (`IsReadOnlyPath`): any path whose verb is not `show`, `monitor` or `resolve` is judged in the edit section, which the built-in read-only profile denies by default. A removal under `request` or `delete` needs no extra marking to be refused to that profile.
  → Constraint: the dispatch key is the YANG path, declared once; every surface (SSH, REST, gRPC, MCP, web) reaches it with no edit to an interface package. A token that starts with `-` is refused before the handler, so the daemon form takes no `--path`.
- [ ] `docs/architecture/cli/command-verbs.md` - "Choosing the verb", "The four state-changing verbs"
  → Constraint: `delete` answers "something that existed stops existing" and is tested before the residual `request`; `request` states that none of create, delete or update describes the command. Today `delete` holds four commands, each a config tree node or a live kernel resource. Which verb a store key removal takes is a design-gate decision.
- [ ] `docs/architecture/zefs-format.md` - the store and the `ze data` CLI
  → Constraint: a key under `meta/` outside `meta/config/` is runtime state and its removal takes the runtime lock only; every other key (`file/...`, `meta/config/...`) is configuration and takes the configuration writer lock. A raw removal of a configuration key bypasses the version, pointer and history machinery.
- [ ] `docs/architecture/hub-architecture.md` - the `request data` RPCs beside `request reload`
  → Constraint: the daemon publishes its one owned store handle to the data RPCs with `InstallDataRPC` after its reload exists and clears it before the store closes (`cmd/ze/hub/main.go`). A handler with no target refuses with "this daemon serves no store". The same handle is the one `statestore.SetStore` publishes to plugin state calls.
- [ ] `docs/architecture/api/process-protocol.md` - "Persistent runtime state"
  → Constraint: a plugin's identity for a state call is the daemon's configured process name, and a request cannot choose another owner. The running-owner guard reads the same grants by the same name, so a plugin run under another name neither holds the grant nor trips the guard.
  → Constraint: a plugin state value carries at most one MiB. The operator's content bound (195,840 bytes) is smaller, so a value a plugin wrote can be larger than an operator can rewrite through the daemon; the refusal names the offline command.
- [ ] `docs/architecture/ssh/fixit-bcrypt-hash-credential.md` - "Decisions", "Residual exposure"
  → Constraint: a command line an operator typed has exactly one redaction entry point, `redact.Command`, which decides by the NAME of the keyword before a value, never by the schema. A surface leaks because it did not ask, so the content is protected by a keyword that function answers for, and by no mechanism beside it.
  → Decision: redaction runs on the full command before any truncation, so a content cut by a log limit cannot half-leak.
- [ ] `docs/guide/config-editor.md` - the `ze config import` paragraph, and "An offline writer refuses while the daemon owns that store"
  → Constraint: `ze config import` chooses its destination store with `--dir`, then `ze.config.dir`, then the default folder, independently of its inputs; it refuses duplicate names before any write and replaces an existing config only once confirmed (a terminal answer, or `--yes`). The route to the daemon applies only when the destination is the store a running daemon owns, and only after those checks and that confirmation. The page's sentence about the offline writer changes for import, in the same phase.
- [ ] `docs/features/ai-first.md` - "Structured Diagnostics"
  → Constraint: a doctor diagnostic's `fix` is one command held alone, drawn on its own line and carried as `fix` in JSON. A second step belongs in the message, in order, never in the fix field. `ze show diagnostic <code>` prints the code description, which is a text site of its own.
- [ ] `docs/guide/ddos-mitigation.md` - the FlowSpec record and the orphan report
  → Constraint: three places in the page quote the removal sentence (the "Not configured" row, the unrun row, the corrupt-record paragraph) and its source anchor names `recordRemovalToday`; each changes with the report text.
- [ ] `ai/doors/system.md`, `ai/doors/config-yang.md`, `ai/doors/commands-cli.md`, `ai/doors/system-plugins.md`, `ai/doors/le-tooling.md`, `ai/doors/hub-engine.md`
  → Constraint: a plugin reaches the store only through the SDK state calls (`StatePut`, `StateGet`, `StateRemove`), confined to the keys granted to its process name (`statestore.RegisterPluginKeys`); the daemon route for an operator removal is a daemon builtin, never a call into the plugin.

**Key insights:**
- The store has one writer. The daemon holds the owner lock for its whole life, so any second writer is refused by design. A daemon route acts through the daemon's own handle; it does not lift the lock. The lock is per open file description, so even a handler inside the daemon that opened the store afresh would be refused: the handler must use the published handle.
- The removal already exists on that handle (`Storage.RemoveKey`), the same method the local `ze data rm` calls. No store capability is added.
- The report text was rewritten after this skeleton (2026-11-13): it no longer names a refused command, it says `ze data rm` removes the record only while ze is stopped. The defect that remains is that a warning cannot be cleared without stopping the daemon.
- `rm` is the only data write verb with no caller-side payload. `write` and `import` read bytes on the caller's side, `repair` writes a new store at an output path: each daemon form is a different design.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/storage/cli/register.go` - registers the `data` root (`registry.MustRegisterRootHandler`, Mode `offline`) and the local-data commands `show data list`, `show data registered`, `show data cat`
- [ ] `internal/component/config/storage/cli/main.go` - `subcommandHandlers`: `backup`, `restore`, `import`, `write`, `rm`, `list`, `cat`, `registered`, `check`, `repair`, `encode`; `cmdRm` opens the store with `openStore(path, true)`
- [ ] `internal/component/config/storage/open.go` - `OpenTree` sends the live tree to `Open`, then `openLive`, whose `lockOwner` calls `lockStoreFile`; `lockStoreFile` returns `ErrBusy` ("close its owner before maintenance") when the owner lock is held
- [ ] `internal/component/config/storage/store.go` - `ErrBusy` = "store is owned by another process"
- [ ] `internal/component/config/storage/ownership.go` - `ownershipLock` returns the other `ErrBusy` text ("stop its daemon before maintenance"), reached from `TransferOwnership` and `AcquireMaintenance`, not from `ze data rm`
- [ ] `internal/component/config/storage/cli/data_rpc.go`, `register_data_rpc.go` - the daemon RPCs `ze-config:data-backup` and `ze-config:data-restore` act on the live store through `DataRPCTarget.Store`
- [ ] `internal/component/config/storage/cli/cmd_backup.go` - `cmdBackup` refused with `ErrBusy` prints a hint naming the live form `request data backup path <absolute-file>`
- [ ] `internal/plugins/ddos/flowspec/doctor.go` - read 2026-11-14, after its 2026-11-13 rewrite. No `orphanDiagnostic` exists. `recordRemovalToday` writes one sentence, "`ze data rm meta/ddos-flowspec/announcement` removes the record only while ze is stopped, because the store refuses a second writer while the daemon owns it", into three reports: `unconfiguredOrphanDiagnostic` (section absent from the saved file; fix is the withdrawal `withdrawalCommand` builds), `corruptRecordDiagnostic` (record does not decode; fix is `ze data cat meta/ddos-flowspec/announcement`, shared by all three checks, the configured-section ones included), and `unrunCases` (the body of `unrunUndecidedDiagnostic` and `noConfigurationUnrunDiagnostic`; fix is `ze show config running`). `checkOrphanRecord` reads the record through the read-only handle doctor opened; `checkStoppedRecord` reads it through `statestore`
  → Constraint: `checkOrphanRecord` judges the SAVED configuration, never the running one, so the "not configured" report also fires on a daemon that still runs the plugin from a running configuration holding the section, with the rule as a live mitigation (file header, and the comment above `unconfiguredOrphanDiagnostic`)
- [ ] `internal/plugins/ddos/flowspec/register.go`, `responder.go` - `announcementKey` is registered by the plugin and granted to the process name `ddos-flowspec` alone (`statestore.RegisterPluginKeys`). The plugin writes it in `announce` (`Record`), removes it after a withdrawal the engine took (`withdrawThrough`, `withdrawRecordedRule`, through `Forget`), and holds an in-memory `recorded` flag only while its responder runs
- [ ] `internal/core/diagnostic/codes.go` - the description of `CodeDoctorDDoSFlowspecOrphanAnnouncement` (shown by `ze show diagnostic`) ends "send the withdrawal the report prints, then, while ze is stopped, remove the record with ze data rm meta/ddos-flowspec/announcement": a fourth site of the same sentence
- [ ] `internal/component/config/storage/yang/ze-data-cmd.yang` - `request data backup` and `request data restore` are `ze:command` containers under `request` > `data`, their arguments leaves of the command container
- [ ] `internal/component/config/storage/cli/register_data_rpc.go` - `pluginserver.RegisterRPCs` binds the two wire methods to their handlers
- [ ] `internal/core/statestore/statestore.go`, `permissions.go` - `Delete` removes through the published handle and answers an absent key as success; `PluginKeyAllowed` confines a plugin to its granted keys, and a grant must name a `meta/` key. No notion of a protected key exists anywhere
- [ ] `internal/component/doctor/local_route.go` - `ze doctor` asks the daemon first (`askDaemonOverSSH`: listener probe, credentials without a prompt, then the command over SSH) and runs its checks locally only when the daemon does not answer: the one local command that tries the daemon and then acts locally
- [ ] `internal/component/authz/authz.go` - `builtinReadOnlyProfile`: the run section allows by default, the edit section denies by default

What each `ze data` verb does under a running daemon, as read:

| Verb | How it opens the live store | Under a running daemon | Daemon route today |
|------|-----------------------------|------------------------|--------------------|
| `cat`, `list`, `registered` | reader (`openStore(path, false)`) | answers | yes: `show data cat`, `show data list`, `show data registered` are local-data commands the daemon also serves |
| `check` | `zefs.CheckPath` on the store, then a reader (`checkStoreHistory`) | not read: whether `CheckPath` takes a lock is unverified | none |
| `rm` | writer (`cmdRm`): one or more literal keys, no pattern; prints `removed <key>` per key and `<n> entries removed from <store>`; exit 2 when the open fails or nothing was removed, 0 on a partial success; a key that is absent is an error line | refused, `ErrBusy`, exit 2, no hint | none |
| `write` | writer (`cmdWrite`, `openOrCreateStore`); the bytes come from a file or stdin on the caller's side | refused, `ErrBusy` | none |
| `import` | writer (`cmdImport`, `openOrCreateStore`); the files are on the caller's side | refused, `ErrBusy` | none |
| `repair` | reads the live store (`zefs.RepairPath`) and opens the NEW store at `--output` as writer (`repairStoreHistory`) | not read: whether `RepairPath` is refused beside the owner is unverified | none |
| `backup` | writer (`cmdBackup`) | refused, with a hint to the live form | `request data backup` |
| `restore` | writer for `config` (`cmdRestoreConfig`); a full restore replaces the store and is offline only | refused | `request data restore ... config` |
| `encode` | not read | not read | not read |

`write` and `import` share the gap `rm` has under the same owner rule: refused
beside a running daemon, with no daemon form. `repair` and `check` are not
refused, since neither takes the store lock, but neither has a daemon form, so
a remote operator cannot run them. `encode` touches no store. Thomas decided on
2026-11-14 that this spec builds the daemon form of all of them (Task, "Scope
and design").

Research corrections to the Task text above (2026-11-14, read at the producers):

- The report no longer tells the operator to run a refused command. Since 2026-11-13 it says the removal works only while ze is stopped. What the operator meets is unchanged: the warning cannot be cleared on a running box.
- Four texts carry that sentence, not two: the three reports `recordRemovalToday` feeds and the code description in `codes.go`.
- The corrupt-record report's fix is the read command, and the report is shared with the configured-section checks, so a change to it changes those reports too.
- When the section is removed while the daemon runs, the plugin withdraws the rule and removes the record itself before its process is stopped (`withdrawForConfig`). An orphan record with the section absent therefore exists only after a withdrawal the engine did not take, or after the section was removed while the plugin was not running.

**Behavior to preserve:**
- The local `ze data rm` with the daemon stopped, unchanged in output and exit code.
- One writer: the daemon's owner lock is never lifted or bypassed.
- `request data backup` and `request data restore`, unchanged.
- The plugin's own removal of its record (start, withdrawal, session end), unchanged.

**Behavior to change:**
- `ze data rm` reaches a running daemon and removes the key through the daemon's own store.
- Every report and description that says the record can be removed only while ze is stopped names the route that works while it runs, with the two steps in order and why the order matters.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `request data` daemon RPCs | `internal/component/config/storage/cli/data_rpc.go` `DataRPCTarget`, `InstallDataRPC`, `handleDataBackup`; `register_data_rpc.go`; `yang/ze-data-cmd.yang` | Builtin RPCs that act on the daemon's own store handle for backup and config restore, refuse when the daemon serves no store, and answer a map | extends: the removal, the key write, the config import and the repair are four more builtins on the same target, registered and declared in the same module. The config import reuses the candidate-and-reload path of the restore |
| Local-data commands served by the daemon | `docs/architecture/api/commands.md`, "Served by the daemon too"; `command.MustRegisterLocalData` | `show data cat`, `list`, `registered` answer locally and through the daemon with one handler, each opening its own reader | does not apply: `ze cli -c` answers a local-data command in the caller's process first, where a writer open is refused. A store mutation cannot take this shape |
| The offline refusal hint | `internal/component/config/storage/cli/cmd_backup.go` `cmdBackup`, `daemonAddress`; `cmd_restore.go` | A refused offline backup or restore names the live form and the daemon's address; each writes its own `hint: a daemon owns the store` prefix | does not apply to `rm`, `write` and `import`: they send their work to the daemon instead of hinting. `daemonAddress` is reused in the refusal of a busy store whose daemon does not answer. Backup and restore keep their hint |
| A local command that asks the daemon first | `internal/component/doctor/local_route.go` `askDaemonOverSSH`, `answerLocalWith` | `ze doctor` probes the daemon's listener, logs in without a prompt, runs the daemon form over SSH, and acts locally only when the daemon does not answer | extends: a busy `rm`, `write` or `import` takes the same steps (address, login without a prompt, one command with the raw pipe), in the opposite order of preference: local first, the daemon when the store is busy |
| The single-writer lock | `internal/component/config/storage/open.go` `openLive`, `lockStoreFile` | Refuses a second writer while the daemon owns the store | does not change: the daemon route writes through the owner |
| Plugin key grants | `internal/core/statestore/permissions.go` `RegisterPluginKeys`, `PluginKeyAllowed` | Confines a plugin's state calls to the `meta/` keys granted to its process name | does not apply to an operator: an operator is not a plugin. It is the reason nothing but a process named `ddos-flowspec` can rewrite the record after its removal |
| The runtime-state predicate | `internal/component/config/storage/store.go` `runtimeKey` | Splits keys into runtime state (`meta/` outside `meta/config/`) and configuration, to pick the lock a write takes | extends: the same predicate names what `delete data key` and `update data key` may reach, so the command and the store agree on what runtime state is |
| The offline config import | `internal/component/config/cli/cmd_import.go` `cmdImportWithStorage`; `internal/component/config/storage/restore.go` `RestoreConfig` | `ze config import` replaces an existing config through a new active version and keeps the previous one as rollback; refused beside a running daemon | extends: `ze data import` replaces an existing config the same way offline, and the daemon form commits through the same version machinery |
| The argument length bound | `internal/component/command/argvalidate.go` `argLengthMax` | A leaf that declares a length may hold up to 1,048,576 characters; `request peer notify` carries hex data this way | extends: the `content` leaf declares its length and carries base64. No input channel is added |
| The artifact path rules | `internal/component/config/storage/backup.go` `CheckArtifactPath` | Refuses a relative path, a `..` element and a symlink for a file a daemon RPC acts on | extends: `request data repair path` is judged by the same rules |
| Command-line redaction | `internal/core/redact/redact.go` `Command`, `isCredentialKey`; `internal/component/config/mask.go` `DisplayCommand` | Replaces the value after a credential keyword in every recorded command line: accounting, audit, the SSH exec log, the session transcript | extends: `content` joins the command-line keyword set |
| The plugin snapshot | `internal/component/plugin/server/doctor_snapshot.go` `PluginDoctorSnapshot` | Answers each plugin's running state as values, and an error when no server is published or its plugins have not started | extends: the running-owner guard is a second reader |
| Authorization by verb | `internal/component/plugin/server/command.go` `IsReadOnlyPath`; `internal/component/authz/authz.go` `builtinReadOnlyProfile` | A command whose verb is not a read verb is judged in the edit section, which the read-only profile denies | applies unchanged: a removal under `request` or `delete` is denied to the read-only profile with no new marking |
| The one sentence that says what removes the record | `internal/plugins/ddos/flowspec/doctor.go` `recordRemovalToday` | Writes "removes the record only while ze is stopped" into three reports | replaces: the sentence names the daemon command and the order of the two steps |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A daemon surface (SSH, REST, gRPC, MCP, web, an interactive `ze cli` session): one of the five commands, as text, reaches the dispatcher.
- The local `ze data rm`, `ze data write` or `ze data import` on the daemon's host: the `data` root handler in `internal/component/config/storage/cli`.

### Transformation Path
1. Local command, store free: it opens the store as writer and acts, as today. Nothing below runs.
2. Local command, store busy (the open answers `storage.ErrBusy`): it reads its input, if any, on the caller's side and base64-encodes it, finds the daemon's address, logs in without a prompt, and sends the daemon command over SSH asking for the dispatcher's JSON. These are the steps `ze doctor` takes to reach `show doctor`.
3. Dispatcher: authorization and accounting judge the command by its verb (edit section for `delete`, `update` and `request`; run section for `show`), with the `content` value replaced by the placeholder in the accounting and audit record. The validator judges the arguments against the YANG leaves, the declared length of `content` included.
4. Handler (a builtin RPC in `internal/component/config/storage/cli`): it loads the published store target, and refuses when the daemon serves no store.
5. Key commands: the handler refuses a key that is not runtime state, then a key whose granted plugin runs, then decodes `content` (for `update`), then calls the store's removal or write on the daemon's own handle.
6. Config import: the handler refuses a name that is not a plain config name, decodes `content`, then for the daemon's own config stages it as the candidate and runs the reload (the path `request data restore ... config` takes, confirmed-commit window included), and for another name commits a new active version.
7. Repair: the handler checks the output path by the rules of a backup path, then salvages the live tree into it.
8. Check: a local-data handler runs the frame check and the history walk over a reader handle and answers rows. `ze cli -c` runs it in the caller's process, the daemon runs the same handler for every other surface.
9. Answer: a map (the key, the bytes written, the version stamp, the counts) or a refusal. The local command decodes it and prints the lines the local route prints.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI ↔ daemon | SSH exec with the raw pipe, as `ze doctor` reaches `show doctor` (`sshclient.DaemonAddress`, `LoadCredentialsNoPrompt`, `ExecCommandRaw`) | No |
| Daemon surface ↔ dispatcher | The command text, one string, on every surface | No |
| Handler ↔ store | The handle `InstallDataRPC` publishes; never a second open | No |
| Handler ↔ plugin state | Read only: the key grants (`statestore`) and the plugin snapshot (`pluginserver.PluginDoctorSnapshot`), both value types | No |
| Config import ↔ reload | The reload function the store target already carries | No |

### Changed-Wire Census

**Search evidence:** 2026-11-14, grep over `internal`, `cmd`, `pkg`, `docs`, `test`, `website` and `features`, in Go, Markdown, `.ci`, shell, Python, YANG and JSON, for `ze data rm`, `data rm `, `only while ze is stopped`, `recordRemovalToday` and `ze-config:data-`. Controls: the known reader `TestRemovedSectionReportSaysWhatRemovesTheRecord` and the known writer `register_data_rpc.go` were both found. No shell, Python or `.ci` carrier reads any of the three wires. The store's on-disk format does not change: a removal unlinks the key's file and syncs its folder (`store.go` `guard.remove`), which the local route already does.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Doctor report text | writer | `internal/plugins/ddos/flowspec/doctor.go` `recordRemovalToday`, called by `unconfiguredOrphanDiagnostic`, `corruptRecordDiagnostic` and `unrunCases` | Read in full, 2026-11-14 | Rewritten: it names the daemon command. All three callers change with it |
| Doctor report text | reader | `internal/plugins/ddos/flowspec/doctor_test.go` `TestRemovedSectionReportSaysWhatRemovesTheRecord`, `TestCorruptRecordReportNamesTheReadCommand` | grep: the first pins "removes the record only while ze is stopped" | The pinned sentence becomes false, so the test is rewritten to pin the new one. That is a test change the `ze-test-judge` route judges |
| Doctor report text | reader | `docs/guide/ddos-mitigation.md` (rows at lines 674 and 695), `internal/core/diagnostic/codes.go` (the description of the orphan code, line 402) | grep | Edited in the same phase as the report text |
| The daemon's command set | writer | `internal/component/config/storage/cli/register_data_rpc.go` `init`, `internal/component/config/storage/yang/ze-data-cmd.yang` | Read in full | Four builtin commands and one local-data command added. `request data backup` and `request data restore` are preserved unchanged |
| The recorded command line | writer | `internal/core/redact/redact.go` `Command`, reached through `internal/component/config/mask.go` `DisplayCommand` from `internal/component/plugin/server/command.go` `BeginAccounting` and `recordCommandAudit` | Read 2026-11-14 | Changes for a command that carries `content`: the value becomes the placeholder. Every other command line is recorded as today |
| The recorded command line | reader | the audit log (`show audit`), the TACACS+ accounting server, the SSH exec log, the CLI session transcript | `docs/architecture/ssh/fixit-bcrypt-hash-credential.md`, "Four shapes of input" | Each already reads the redacted copy; an accounting server outside the repository receives the placeholder in place of a value it never received before |
| The local `ze data write` and `ze data import` output | writer | `internal/component/config/storage/cli/main.go` `cmdWrite`, `cmdImport` | Read in full | Preserved on both routes: the lines and exit codes the local route prints today. `cmdImport` over an existing config commits a version instead of a raw write |
| The local `ze data write` and `ze data import` output | reader | `test/plugin/ddos-flowspec-stopped-orphan-doctor.ci` and the fixtures under `internal/test/fixture` that seed keys with `ze data write` and `ze data import` while ze is stopped | grep `data write`, `data import` over `test` and `internal/test` (research agent, 2026-11-14) | Preserved: they run with ze stopped, on the local route |
| The daemon's command set | reader | `internal/component/plugin/all/testdata/wire-methods.snapshot` (rows 114 and 115 hold the two existing methods), and every remote surface through `command.AnswerSurface` | grep; `docs/architecture/api/commands.md`, "Which surfaces offer a command" | Four snapshot rows added. No surface is edited: each derives its list from the registration |
| The refused local `ze data rm` output | writer | `internal/component/config/storage/cli/main.go` `cmdRm` | Read in full: it prints `error: <err>` and exits 2, with no hint | Changes: the refused command either names the daemon form (as a refused backup does) or sends the removal to the daemon itself. Which is a design-gate decision; the output with the daemon stopped is preserved either way |
| The refused local `ze data rm` output | reader | `internal/component/config/storage/cli/main_test.go` `TestDataRefusesOwnedStoreMutation`; `docs/guide/operations.md` line 128, `docs/guide/command-reference.md` line 2756, `docs/architecture/fleet-config.md` line 484 | grep | The test pins that a second writer is refused and the key survives; it keeps that for the store open itself. If the design forwards a refused removal to the daemon, the test's fixture has no daemon to answer and still sees the refusal. The pages gain the daemon form |
| Plugin state calls | writer | `internal/plugins/ddos/flowspec/register.go` `sdkDispatcher` (`StatePut`, `StateGet`, `StateRemove`) | Read 2026-11-14 | Preserved unchanged: the operator removal is a second caller of the same store handle, not a change to the plugin protocol |
| Plugin state calls | reader | `internal/core/statestore/statestore.go` `Write`, `Read`, `Delete` | Read 2026-11-14 | Preserved unchanged |

### Integration Points
- `internal/component/config/storage/yang/ze-data-cmd.yang`: five command nodes, merged onto the `delete`, `update`, `request` and `show` verb trees from the owner's module.
- `internal/component/config/storage/cli/register_data_rpc.go`: four builtin RPC registrations. None declares `RequiresSelector`: that guard waits for a leaf named `selector` (the peer selector), so a command that declared it without that leaf would be refused on every call. The mandatory `key` leaf is the guard, as for `delete interface name` (found in phase 1). A command that changes store state declares its audit action here (`RPCRegistration.AuditAction`).
- `internal/component/config/storage/cli/register.go`: `show data check` as a local-data command with its shape and columns.
- `internal/core/statestore/permissions.go`: one new reader that answers which plugin names a key is granted to.
- `internal/core/redact/redact.go` `Command`: the one entry point that makes an operator's command line safe to record. The dispatcher's accounting and audit records reach it through `config.DisplayCommand` (`internal/component/plugin/server/command.go` `BeginAccounting`, `recordCommandAudit`), and the SSH exec log and the CLI session transcript call it directly. It replaces the token that follows a credential keyword (`isCredentialKey`). The keyword `content` joins that command-line set, so the value after it reads as the placeholder on all four records. No second mechanism is added.
- `internal/plugins/ddos/flowspec/doctor.go` and `internal/core/diagnostic/codes.go`: the report texts.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes, by design | Every write goes through the dispatcher and the daemon's own store handle; the config import goes through the reload, never a raw key write |
| No unintended coupling (components stay isolated) | Yes, by design | The handlers read plugin state as value types from two registries and name no plugin, key or feature |
| No duplicated functionality (extends existing, does not recreate) | Yes, by design | The store target, the removal and write methods, `RestoreConfig`, the candidate-and-reload path, `CheckArtifactPath`, and the SSH route of `ze doctor` are all reused |
| Zero-copy preserved where applicable (refs, not copies) | N-A | An operator command, off every hot path; the content is decoded once |
| Registration over hardcoding, outbound | Yes, by design | Commands register from the owner's `init()` and its YANG module; no surface is edited |
| Registration over hardcoding, inbound | Yes, by design | The running-plugin guard derives the owner from the key grants a plugin registered; a new plugin's keys are guarded with no edit here |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Nothing rewrites the FlowSpec record after an operator removes it with the section removed | Only a process named `ddos-flowspec` holds the key grant (`register.go` `init`), and a reload that removes the section stops that process | BROKEN in one case the code documents: doctor judges the saved file, so the "not configured" report also fires while the daemon still runs the plugin from a running configuration that holds the section. There the removal takes the record from under a live rule: a crash then leaves the rule at the peers with no record, and the plugin's in-memory `recorded` flag stays true | AC-5: both key commands refuse a key whose owning plugin runs, and AC-2: the report says so | holds in the plain case, broken in the saved-file case, which AC-5 closes |
| A-2 | A removal through the daemon's handle is seen by the next `ze doctor` at once | Doctor opens a fresh read-only handle per run (`doctor.go` `resolveStorageWithDiag`, reported by the research agent, not read here); a single-key read waits for no writer (`store.go` `ReadKey`) | The report stays after the removal until some later moment, and AC-1 fails | The AC-1 functional test | confirmed 2026-11-14: `test/plugin/ddos-flowspec-removed-section-orphan-cleared.ci` reads no orphan finding in the `ze doctor` run that follows the removal |
| A-3 | The withdrawal the report prints (`send bgp * update text mp ipv4/flow del ...`) answers success on a daemon with no flow-capable peer | `WithdrawNLRIBatch` (`internal/component/bgp/reactor/reactor_api_batch.go`) answers `route.ErrNoPeersMatch` when the selector matches no peer at all; the research agent read that a peer that is down queues the withdrawal and answers success | The AC-1 test fails at its withdrawal step | The AC-1 test configures one BGP peer, which may stay down | BROKEN for a daemon with no peer: the test needs one peer. Confirmed 2026-11-14 with one down peer: the withdrawal answers success in `ddos-flowspec-removed-section-orphan-cleared.ci` |
| A-4 | `ze data check` and `ze data repair` are not refused beside a running daemon | `zefs.CheckPath` walks the tree through `walkFrameTree` with read-only opens and no lock; `cmdRepair` reads the source the same way and opens only its output as writer (`cmd_integrity.go`, `pkg/zefs/check.go`) | - | Read at the producers, 2026-11-14 | holds |
| A-5 | An SSH exec line carries a command of about 1 MiB | Not read: the argument validator admits a declared length up to 1,048,576 characters (`argLengthMax`), the SSH exec line limit is unread | The local `write` and `import` cannot send a large file though the validator would accept it | A unit test of the bound over the SSH route, and the functional test of a content near the bound | BROKEN, read at the producer 2026-11-14: one SSH packet carries at most 256 KiB (`vendor/golang.org/x/crypto/ssh/cipher.go` `maxPacket = 256 * 1024`, as OpenSSH caps it), an exec request is one packet, and nothing in the tree changes the cap. Thomas chose one lower bound on every surface the same day: 261,120 base64 characters, 195,840 bytes of content, which fits one packet with a 512-character key. Every bound in this spec is that one |
| A-6 | A raw write of an existing config's `file/active` key is shadowed by its active pointer | The comment in `internal/component/config/cli/cmd_import.go` above its `RestoreConfig` call; the reader that prefers the pointer is not read | The offline `ze data import` of an existing name is correct today and AC-9 fixes nothing | Read the pointer reader in the implementation's first phase | unvalidated |
| A-7 | The salvage walk of `request data repair` tolerates the daemon's own writes | `walkFrameTree` reads one directory entry at a time and opens each; a key removed between the two fails the walk | A repair on a busy daemon fails with an open error | The handler holds the store's configuration guard for the walk, as a live backup does, and a test removes a key during it | confirmed with a correction, 2026-11-14: the configuration guard alone leaves runtime-state writers free (`store.go` `acquire(false)` takes the configuration lock only), so the walk runs under `storage.HoldWrites`, which holds both locks (`TestHoldWritesKeepsEveryWriterOffTheWalk`). The local `ze data check` and `ze data repair` beside a busy daemon stay unguarded, as before this spec |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A removal on every remote surface reaches keys that hold secrets or the login material (`meta/auth`, `meta/ssh`, `meta/web/key`), and removing one can lock the operator out | A removal of such a key succeeds in a test | The command is denied to the read-only profile by its verb. Confining it to runtime state does not exclude these keys, which are runtime state: see R-6 |
| R-2 | A raw removal of a configuration key (`file/...`, `meta/config/...`) bypasses versions, pointers and history, and desynchronises them | `ze data check` reports a history fault after a removal | The two key commands refuse a configuration key (AC-4); configuration changes through `update data config`, which keeps versions |
| R-3 | On a hub, a removed client config key notifies nobody: the write observer hears written keys only (`store.go` `guard.Release`) | A managed client keeps a config its hub no longer holds | Follows R-2: a removal confined to runtime state never reaches a client config key |
| R-4 | An absent key answers differently on the two routes: the store's removal returns not-exist, which the local `rm` prints as an error, while `statestore.Delete` answers success | The daemon form and the local form disagree in a test of an absent key | The daemon form answers what the local route answers (AC-3) |
| R-5 | The removal takes the record from under a running plugin that holds the rule live (A-1's broken case) | The orphan report and a running `ddos-flowspec` process on the same daemon | `delete data key` and `update data key` refuse a key granted to a plugin whose process runs (AC-5). The report says so and names the plugin restart for that case |
| R-6 | Runtime state holds the login and HTTPS keys (`meta/auth`, `meta/ssh`, `meta/web/key`), so an admin can remove or overwrite them through the daemon and lose access | An operator locked out after a removal | Thomas accepted this at the research gate: the commands are denied to the read-only profile and an admin holds the offline command already. The command reference says which keys hold credentials |
| R-7 | The `content` of `update data key` can be a credential, and every dispatched command is recorded by the audit log and sent to TACACS+ accounting, the SSH exec log and the session transcript. Command redaction goes by keyword name, and `content` is not one it answers for today | A credential in clear in `show audit`, on the accounting server, or in a transcript file | `content` joins the command-line credential keywords of `redact.Command`, the one entry point all four records pass through (AC-8) |
| R-8 | A content larger than the bound is refused by the validator with a message about bytes, which does not say what to do | An operator with a large config cannot import it and is not told how | The refusal names the bound and the route for a larger config: `request data restore path <file> config` from a backup on the daemon's host (AC-12) |
| R-9 | A config import through the daemon reloads the daemon: a config the reload refuses, or a confirmed-commit window that is open, must leave everything as it was | The active config changes after a refused import | The import takes the path `request data restore ... config` takes, with its window refusal and its candidate withdrawal (AC-10) |
| R-10 | The store target is cleared before the store closes; a command that arrives during shutdown finds none | A nil dereference at daemon stop | Each handler loads the target once and refuses with "this daemon serves no store", as the two existing RPCs do |
| R-11 | Two operators remove and write the same key at once | An answer that reports a removal the other operator's write undid | The store's guard serialises them; each answer reports what its own call did, and no command reads then writes |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A remote admin removes or overwrites runtime state a running plugin depends on (guarded by AC-5), a config import leaves the daemon on a config the reload refused (guarded by AC-10), or a credential lands in the audit log (guarded by AC-8). The store's single-writer rule cannot break: no handler opens the store |
| How is it reverted? | The commands are additive: removing the five YANG nodes and their registrations removes them from every surface. The local commands fall back to today's refusal. A config import is reverted with the config rollback, a removed or written runtime key is not |
| Who else touches this path? | `spec-single-owner-lifecycle-5-session-end-locals` (closed) wrote the FlowSpec report this spec rewrites. `cmd/ze/hub/main.go` installs the store target and is not changed |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `delete data key <key>` over `ze cli -c`, daemon running | → | the removal handler in `internal/component/config/storage/cli/data_rpc.go` | `test/plugin/data-rm-live.ci` |
| `ze data rm <key>` on the daemon's host, daemon running | → | `cmdRm` forwarding in `internal/component/config/storage/cli/main.go` | `test/plugin/data-rm-refused-live.ci` |
| `update data key <key> content <base64>` over `ze cli -c` | → | the write handler in `data_rpc.go` | `test/plugin/data-write-live.ci` |
| `ze data write <key> <file>` on the daemon's host, daemon running | → | `cmdWrite` forwarding in `main.go` | `test/plugin/data-write-live.ci` |
| `update data config name <name> content <base64>` over `ze cli -c` | → | the import handler in `data_rpc.go` | `test/plugin/data-import-live.ci` |
| `ze data import <file>` on the daemon's host, daemon running | → | `cmdImport` forwarding in `main.go` | `test/plugin/data-import-live.ci` |
| `ze config import <file>` on the daemon's host, daemon running | → | `cmdImportWithStorage` in `internal/component/config/cli/cmd_import.go`, on the route `daemon_route.go` provides | `test/plugin/config-import-live.ci` |
| `request data repair path <absolute-output>` over `ze cli -c` | → | the repair handler in `data_rpc.go` | `test/plugin/data-repair-live.ci` |
| `show data check` over `ze cli -c` and over SSH exec | → | the check local-data handler in `internal/component/config/storage/cli/storage_data.go` | `test/plugin/data-check-live.ci` |
| `ze doctor` with the section removed and a record stored | → | `checkOrphanRecord` and `recordRemoval` in `internal/plugins/ddos/flowspec/doctor.go` | `test/plugin/ddos-flowspec-removed-section-orphan-cleared.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The daemon runs, the `ddos flowspec` section has been removed, and the store holds a record under `meta/ddos-flowspec/announcement` (moved from AC-16 of `spec-single-owner-lifecycle-5-session-end-locals`) | `ze doctor` reports `doctor-ddos-flowspec-orphan-announcement`. The operator runs the withdrawal the report prints, then removes the record through the running daemon, and the next `ze doctor` run reports no `doctor-ddos-flowspec-orphan-announcement` finding | Every other doctor finding stays: the removal clears only this record's report |
| AC-2 | The same condition as AC-1 | The report's text names the two steps in that order: withdraw the rule with the printed command, then remove the record with `ze data rm meta/ddos-flowspec/announcement`, which it says works while ze runs. It says why the order matters: removing the record first leaves the rule at the peers with no trace in the store and no report to find it. It says that the removal is refused while a `ddos-flowspec` plugin runs, and names the plugin restart for that case. The code description `ze show diagnostic doctor-ddos-flowspec-orphan-announcement` prints, and the two unrun reports, say the same about the removal | The fix field stays the withdrawal: it is the first step. The restart reports of a configured section are not changed |
| AC-3 | The daemon runs. An admin runs `delete data key <key>` on a runtime-state key that exists, from any daemon surface, or `ze data rm <key>` on the daemon's host | The key is gone from the store, read back through a reader handle. The daemon command answers the key it removed. The local command prints `removed <key>` and `1 entries removed from <store>` and exits 0, the lines and code of the local route | The local route with the daemon stopped answers as it does today. A key that does not exist is an error on both routes, exit 2 locally |
| AC-4 | The daemon runs. `delete data key` or `update data key` names a key that is not runtime state: under `file/`, under `meta/config/`, or outside both namespaces | The command is refused before the store is touched. The refusal says the key is configuration, that the two commands reach runtime state only (a key under `meta/` outside `meta/config/`), and names `update data config name <name> content <base64>` for a config file | Every key is still readable with `show data cat`. The offline `ze data rm` and `ze data write` with ze stopped keep their reach |
| AC-5 | The daemon runs a plugin process whose name holds the grant for the key, and an admin runs `delete data key` or `update data key` on that key | The command is refused and the key is unchanged. The refusal names the plugin, says it manages that key while it runs, and names `request plugin restart <name>` | A key granted to a plugin that does not run, and a key no plugin holds a grant for, are removed and written. A daemon that has not started its plugins yet refuses, saying it cannot tell yet whether the owner runs |
| AC-6 | A record that does not decode (`corruptRecordDiagnostic`), daemon running | The report's fix stays the command that prints the record, and its text says: withdraw the rule the printed bytes name at the peers, then remove the record with `ze data rm meta/ddos-flowspec/announcement`, which works while ze runs when no `ddos-flowspec` plugin runs; while one runs, its next announcement replaces the record. After the removal the next `ze doctor` run does not report the code | The step that finds the rule at the peers stays manual: nothing in the store can name it |
| AC-7 | The daemon runs. An admin runs `update data key <key> content <base64>` on a runtime-state key, from any daemon surface, or `ze data write <key> <file>` on the daemon's host, the file or stdin holding up to 195,840 bytes | The key holds exactly the decoded bytes, read back through a reader handle, whether or not the key existed. The daemon command answers the key and the byte count. The local command prints `wrote <key> (<n> bytes) into <store>` and exits 0 | A content that is not valid base64 is refused, naming the argument and the encoding. The local route with the daemon stopped answers as it does today |
| AC-8 | Any surface sends `update data key` or `update data config` with a `content` | The accounting record, the audit log entry, the SSH exec log line and the session transcript of the command hold the command path, the key or name, and the redaction placeholder in place of the content. The content appears in none of them, nor in a refusal that echoes an argument | Every other argument of every other command is recorded as today |
| AC-9 | ze is stopped. `ze data import <file>` names a config that already has an active version | The import commits a new active version and keeps the previous one as rollback, as `ze config import` does, and prints `imported <key> (<n> bytes)`. A daemon started after it reads the imported config | A config name with no active version is written as today |
| AC-10 | The daemon runs. An admin runs `update data config name <name> content <base64>`, or `ze data import <file>` on the daemon's host | For the daemon's own config: the content is staged and the daemon reloads; the answer carries the name and the version stamp, and `show config history` lists the version. A config the reload refuses leaves the active config, its pointers and the running daemon unchanged, and the refusal carries the reload's reason. An open confirmed-commit window refuses the import with the window's own refusal. For another name: a new active version is committed and no reload runs | A name that is not a plain config name (a path, `.`, `..`) is refused. A change already staged refuses the import, naming commit or discard |
| AC-11 | The daemon runs. An admin runs `request data repair path <absolute-output>` | A new store exists at the path on the daemon's host holding every key that reads, and the answer carries the recovered and skipped counts and the path. A relative path, a `..` element, a symlink, an existing output and a name the store owns are refused, each naming the rule | The live store is not changed. `ze data repair --output <path>` on the daemon's host works beside the daemon as it does today |
| AC-12 | A `content` longer than 261,120 characters, on `update data key` or `update data config` | The command is refused before the handler. The refusal names the bound in bytes of content (195,840) and, for a config, `request data restore path <file> config` as the route for a larger one. The local `ze data write` and `ze data import` refuse a larger input the same way, before they connect | A content at the bound is accepted |
| AC-13 | The daemon runs. Anyone allowed `show` runs `show data check`, from any daemon surface, or `ze data check` on the daemon's host | The answer is the store's integrity verdict: the entry count when clean, one row per corrupt entry and per history finding otherwise, with a non-zero status when any is an error. `\| json` and `\| table` render the same rows | `ze data check` keeps its lines and exit codes |
| AC-14 | The read-only profile runs `delete data key`, `update data key`, `update data config` or `request data repair` | Each is denied by authorization before its handler runs. `show data check` is allowed | An admin is allowed all five |
| AC-16 | The daemon runs. An operator runs `ze config import <file>` on the daemon's host, with `--name` or without | The import sends `update data config` for each input and prints the lines the offline route prints (`imported` or `replaced`, the name, the byte count, the file count). Replacing an existing config still asks for confirmation or `--yes` before anything is sent | With ze stopped the command acts on the store as today. `--dir` naming a store no daemon owns stays offline |
| AC-15 | The doctor store integrity finding (`doctor-store-integrity`) | Its description names `ze data check` and `ze data repair --output <path>` as commands that run while ze runs, names `show data check` and `request data repair path <file>` for a remote operator, and says that installing the repaired store needs ze stopped | The finding's code and severity are unchanged |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sees the orphan report on a running box whose config no longer holds `ddos flowspec`, runs the printed withdrawal, then `ze data rm meta/ddos-flowspec/announcement`, then `ze doctor` | `ze doctor` → daemon `show doctor` → `checkOrphanRecord`; `ze cli -c` → `send bgp`; `ze data rm` → busy store → SSH → `delete data key` → the daemon's store; `ze doctor` again | `test/plugin/ddos-flowspec-removed-section-orphan-cleared.ci` |
| 2 | From a remote API client, clears a stale runtime key of a plugin that no longer runs | REST or MCP → dispatcher → `delete data key` handler → the daemon's store | `test/plugin/data-rm-live.ci` |
| 3 | Restores a lost credential key on a running box from a backup file, without stopping ze | `ze data write <key> <file>` → busy store → SSH → `update data key` → the daemon's store; the audit log shows no content | `test/plugin/data-write-live.ci` |
| 4 | Imports a new version of the running config from a file | `ze data import <file>` → busy store → SSH → `update data config` → candidate → reload | `test/plugin/data-import-live.ci` |
| 5 | Sees the store integrity warning from a remote session, checks and salvages | `show data check`; `request data repair path <file>` → salvage on the daemon's host | `test/plugin/data-check-live.ci`, `test/plugin/data-repair-live.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDeleteKeyRemovesRuntimeKey` | `internal/component/config/storage/cli/data_rpc_test.go` | AC-3: the key is gone from the live store and the answer names it | |
| `TestDeleteKeyAbsentIsAnError` | same | AC-3: an absent key answers an error, not success | |
| `TestKeyCommandsRefuseConfigurationKey` | same | AC-4: `file/`, `meta/config/` and un-namespaced keys are refused with the config command named, for delete and for update | |
| `TestKeyCommandsRefuseKeyOfRunningPlugin` | same | AC-5: refused while the owner runs, allowed while it does not, refused while the snapshot is not available | |
| `TestUpdateKeyWritesDecodedContent` | same | AC-7: create and replace, exact bytes | |
| `TestUpdateKeyRefusesBadBase64` | same | AC-7: the refusal names the argument and the encoding | |
| `TestUpdateConfigStagesAndReloads` | same | AC-10: own config through the reload, version stamp answered | |
| `TestUpdateConfigRejectedLeavesPointers` | same | AC-10: a refused reload leaves pointers as they were | |
| `TestUpdateConfigRefusedDuringConfirmWindow` | `internal/component/config/storage/cli/data_rpc_confirm_test.go` | AC-10: the window's refusal | |
| `TestUpdateConfigOtherNameCommitsVersion` | `data_rpc_test.go` | AC-10: another name gets a version and no reload | |
| `TestRepairRPCPathRules` | same | AC-11: each refused path names its rule; the salvage holds the keys | |
| `TestDataCommandsRefuseWithNoStore` | same | R-10: every handler answers "this daemon serves no store" | |
| `TestDataRmForwardsToDaemonWhenBusy` | `internal/component/config/storage/cli/main_test.go` | AC-3: a busy store sends the daemon command and prints the local lines, over a stubbed ask | |
| `TestDataRmBusyWithNoDaemonNamesTheOwner` | same | AC-3 survivor: a busy store with no daemon answering keeps the refusal and says why the daemon did not answer | |
| `TestDataWriteRefusesOversizeBeforeConnecting` | same | AC-12 | |
| `TestDataImportReplacesThroughAVersion` | same | AC-9: an existing name gets a new active version offline | |
| `TestDataCheckAnswersRows` | `internal/component/config/storage/cli/storage_data_test.go` | AC-13: clean and corrupt stores, rows and status | |
| `TestPluginKeyOwners` | `internal/core/statestore/permissions_test.go` | AC-5: the owners of a granted key, a templated key, and an ungranted key | |
| `TestCommandRedactsContentValue` | `internal/core/redact/redact_test.go` | AC-8: the safe copy of both update commands holds the placeholder and not the content; a command with no credential keyword is unchanged | |
| `TestAccountingAndAuditRecordMaskedContent` | `internal/component/plugin/server/command_test.go` | AC-8: the accounting tokens and the audit detail of a dispatched update command hold the placeholder | |
| `TestRemovedSectionReportNamesTheRemovalThatWorksLive` | `internal/plugins/ddos/flowspec/doctor_test.go` | AC-2: replaces `TestRemovedSectionReportSaysWhatRemovesTheRecord`, whose pinned sentence becomes false; the change goes to a `ze-test-judge` agent | |
| `TestCorruptRecordReportNamesTheRemoval` | same | AC-6 | |
| `TestOrphanCodeDescriptionNamesTheLiveRemoval` | `internal/core/diagnostic/codes_test.go` | AC-2, AC-15: the two code descriptions | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `content` length, in base64 characters | 4 to 261,120 | 261,120 | 0 (an empty content is refused: a key is removed with `delete data key`) | 261,121 |
| Decoded content, in bytes | 1 to 195,840 | 195,840 | 0 | 195,841 (refused by the local command before it connects) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ddos-flowspec-removed-section-orphan-cleared` | `test/plugin/ddos-flowspec-removed-section-orphan-cleared.ci` | AC-1 over the real daemon: a record seeded while stopped (as `test/plugin/ddos-flowspec-stopped-orphan-doctor.ci` seeds it), a running configuration without `ddos flowspec`, the orphan report, the printed withdrawal, the removal through the daemon, then a doctor answer without the code | |
| `data-rm-live` | `test/plugin/data-rm-live.ci` | AC-3 over the real daemon: a key removed through the daemon and read back absent through a reader handle, on the fixture `test/plugin/data-backup-live.ci` uses | |
| `data-rm-refused-live` | `test/plugin/data-rm-refused-live.ci` | AC-3, AC-4, AC-5: the local `ze data rm` beside a running daemon removes a runtime key and prints the local lines; a configuration key and a key of a running plugin are refused with their reasons | |
| `data-write-live` | `test/plugin/data-write-live.ci` | AC-7, AC-8: `ze data write` beside a running daemon, the key read back, and `show audit` holding the placeholder and not the content | |
| `data-import-live` | `test/plugin/data-import-live.ci` | AC-10: `ze data import` of the running config, the new version in `show config history`, and a config the reload refuses leaving the active one | |
| `config-import-live` | `test/plugin/config-import-live.ci` | AC-16: `ze config import --yes` of the running config beside the daemon, the offline lines printed, the new version in `show config history` | |
| `data-import-existing` | `test/plugin/data-import-existing.ci` | AC-9: offline import over an existing config, then a daemon start that reads it | |
| `data-repair-live` | `test/plugin/data-repair-live.ci` | AC-11: `request data repair path` on a running daemon, the output checked with `ze data --path <output> check` | |
| `data-check-live` | `test/plugin/data-check-live.ci` | AC-13: `show data check` through the daemon, plain and `\| json` | |
| `data-authz-read-only` | `test/plugin/data-authz-read-only.ci` | AC-14: a read-only user is denied the four writes and allowed the check | |

Each `.ci` is drafted under `test/draft/plugin/` and promoted when it passes under load (`ai/rules/testing.md`).

### Interop Tests (Scope: protocol)
N/A: Scope is cli. No wire-visible protocol behavior changes: the withdrawal the report prints is an existing command.

## Files to Modify
- `internal/component/config/storage/yang/ze-data-cmd.yang` - the five command nodes and their leaves; the module description
- `internal/component/config/storage/cli/data_rpc.go` - the removal, write, config import and repair handlers; the runtime-state and running-owner refusals
- `internal/component/config/storage/cli/register_data_rpc.go` - four registrations
- `internal/component/config/storage/cli/register.go` - `show data check` local-data registration, shape and columns; the `data` root's sub list
- `internal/component/config/storage/cli/storage_data.go` - the check handler answering rows
- `internal/component/config/storage/cli/main.go` - `cmdRm`, `cmdWrite`, `cmdImport`: the busy-store route to the daemon; `cmdImport`'s versioned replace; the usage page and its two examples, which name keys `filePathToKey` never produces
- `internal/component/config/storage/cli/cmd_integrity.go` - `cmdCheck` printing from the same rows the handler answers
- `internal/component/config/cli/cmd_import.go` - `cmdImportWithStorage`: a busy store sends each input to the daemon after the name checks and the confirmation (AC-16)
- `internal/core/statestore/permissions.go` - the reader that answers a key's owners
- `internal/core/redact/redact.go` - `isCredentialKey` answers true for `content`, in the command-line set only, so a JSON payload keyed `content` elsewhere is not touched
- `docs/architecture/ssh/fixit-bcrypt-hash-credential.md` - the list of names redaction answers for gains `content` on a command line, with the reason
- `internal/plugins/ddos/flowspec/doctor.go` - `recordRemovalToday` and its three callers (AC-2, AC-6)
- `internal/core/diagnostic/codes.go` - the descriptions of the orphan code and the store integrity code (AC-2, AC-15)
- `internal/component/plugin/all/testdata/wire-methods.snapshot` - four rows
- `internal/test/localdatacoverage/` - the evidence row and counts `show data check` owes
- `docs/architecture/cli/command-verbs.md` - question 3 of "Choosing the verb" admits a store key under `delete`; the `update` sentence for a store key; the `delete` and `create` rows of the verb table, which miscount today

## Files to Create
- `internal/component/config/storage/cli/daemon_route.go` - the local commands' route to the daemon: address, login without a prompt, one command, the decoded answer
- `test/plugin/data-rm-live.ci`, `data-rm-refused-live.ci`, `data-write-live.ci`, `data-import-live.ci`, `data-import-existing.ci`, `data-repair-live.ci`, `data-check-live.ci`, `data-authz-read-only.ci`, `ddos-flowspec-removed-section-orphan-cleared.ci`
- The compiled fixtures those tests drive, beside `internal/test/fixture/storage_live_fixture.go`, whose `storageLiveClient` and `storageLiveRequest` they reuse

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/config/storage/yang/ze-data-cmd.yang`: five `ze:command` nodes |
| YANG validation constraints | Yes | same file: `key` and `name` mandatory with a length; `content` mandatory, length 4 to 261,120, a base64 pattern; `path` as the backup path leaf |
| YANG custom validators | No | the runtime-state and running-owner refusals depend on live state, so they are the handler's |
| CLI commands/flags | Yes | the five daemon commands; no new offline flag. `ze data rm`, `write`, `import` gain a route, not a flag |
| CLI grammar (keyword before value) | Yes | every value follows a keyword (`key`, `name`, `content`, `path`); `key` and `name` are selector keywords of the grammar. Judged by `./le cli grammar` |
| Editor autocomplete | Yes | derived from the YANG nodes; `./le test functional ui` covers completion of the new paths |
| Functional test for new RPC/API | Yes | the nine `.ci` files in the test plan |
| Pipe completeness | Yes | the four writes answer a map through the dispatcher; `show data check` registers its shape and columns and owes its `internal/test/localdatacoverage` row |
| Env var registration | No | no new tunable: the content bound is the validator's |
| Doctor check for runtime dependencies | No | no new runtime dependency: the commands use the store handle the daemon already owns |
| Doctor code per problem class | N-A | each command's refusal is answered to the operator who typed it, with its fix; none is a state of the box that outlives the command. The two existing codes whose text changes are `doctor-ddos-flowspec-orphan-announcement` and `doctor-store-integrity` |
| Prometheus counters/metrics | No | operator commands, already counted by the dispatcher's command metrics |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP family |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/backup-and-restore.md`: live store maintenance beside backup and restore, its level read from `./le feature report` |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (the `ze data` block and the `request data` block), `docs/guide/operations.md` (the `ze data` table) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (a command that carries caller bytes in a `content` argument; its value redacted in every record of the command), `docs/architecture/hub-architecture.md` (the store RPCs beside `request reload`) |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/ddos-mitigation.md` (the three places that quote the removal sentence, and the source anchor), `docs/guide/authentication.md` (credential recovery no longer needs ze stopped), `docs/guide/config-editor.md` (`ze config import` beside a running daemon), `docs/guide/audit.md` (the placeholder) |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if the new live fixtures add a scenario family it lists |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/zefs-format.md` (the `ze data` CLI and its daemon forms), `docs/architecture/storage-backends.md` (a live salvage walk under the configuration guard), `docs/architecture/cli/command-verbs.md` (the `delete` and `update` wording, the verb table counts) |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | the command inventory: `internal/component/plugin/all/testdata/wire-methods.snapshot`, and the catalog `./le cli catalog check` compares |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/guide/ddos-mitigation.md` anchors `recordRemovalToday`; `./le doc check all` finds the rest |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/command-reference.md`, `docs/guide/operations.md`, `docs/architecture/fleet-config.md` (its `ze data rm` row) |

## Implementation Steps

Each phase lands as its own commit, with its tests, its pages and the verb catalog edit it needs. The release fix is phases 1 to 3: they clear the FlowSpec warning on a running box and stand without the later ones.

1. **Phase: Wiring (MANDATORY FIRST)** -- `delete data key`: the YANG node, the registration, a handler that removes a runtime-state key through the published store, and `test/plugin/data-rm-live.ci` red first, then green. Input: the store target. Output: the key gone, the answer naming it. Constraint: no store open in the handler. AC-3 (daemon half), AC-4 for delete, AC-14 for delete.
2. **Phase: the running-owner guard** -- the reader of a key's owners in `statestore`, and the refusal in the handler from the plugin snapshot. Constraint: the handler names no plugin, key or feature. AC-5 for delete.
3. **Phase: the local route and the reports** -- `daemon_route.go`, `cmdRm` sending a busy removal to the daemon and printing the local lines; `recordRemovalToday` and its three reports, the orphan code description, `docs/guide/ddos-mitigation.md`; the doctor test change through a `ze-test-judge` agent; `test/plugin/data-rm-refused-live.ci` and `ddos-flowspec-removed-section-orphan-cleared.ci`. AC-1, AC-2, AC-3 (local half), AC-6.
4. **Phase: the masked argument** -- `content` joins the command-line credential keywords of `redact.Command`, which the accounting record, the audit log, the SSH exec log and the session transcript already pass every command through. Built and proven before any command carries a content. AC-8.
5. **Phase: `update data key`** -- the node, the handler (runtime state, running owner, base64 decode, write), `cmdWrite` on a busy store, the bound refusals, `test/plugin/data-write-live.ci`. AC-4 and AC-5 for update, AC-7, AC-12.
6. **Phase: the config import** -- read the pointer reader first (A-6). `cmdImport` replacing through a version offline; `update data config`, its handler on the candidate-and-reload path, `cmdImport` on a busy store; `ze config import` on a busy store taking the same route, which places the route where both `internal/component/config/cli` and `internal/component/config/storage/cli` reach it; `test/plugin/data-import-existing.ci`, `data-import-live.ci` and `config-import-live.ci`. AC-9, AC-10, AC-16.
7. **Phase: repair and check** -- `request data repair` under the configuration guard (A-7); `show data check` as a local-data command with `cmdCheck` printing from its rows; the store integrity code description; `test/plugin/data-repair-live.ci`, `data-check-live.ci`, `data-authz-read-only.ci`. AC-11, AC-13, AC-14, AC-15.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No handler opens the store; each acts through the published handle. A config never reaches the store by a raw key write through the daemon |
| Registration over enumeration | The handlers and the guard name no plugin, key or feature. A plugin that registers a new runtime key is guarded with no edit in `internal/component/config/storage` |
| Process boundary | The guard reads grants and the plugin snapshot as values; a plugin that runs in another process is guarded the same way |
| Operator errors | Each refusal says what is wrong, why, and the exact command: the configuration key (the config command), the running owner (the plugin restart), the oversize content (the bound and the backup route), the busy store with no daemon answering (why the daemon did not answer) |
| Same output | The local `rm`, `write` and `import` print the same lines and exit codes whichever route ran |
| Stale comments | The comments in `doctor.go` that say the removal works only while ze is stopped, and the package comment of `register.go` in `storage/cli` that calls mutation exclusive-ownership only |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Five daemon commands on every surface | `./le cli catalog check`, `./le doc yang-contract command-contract`, `./le cli grammar`, `./le cli dispatch check`, and the wire-method snapshot test |
| The warning clears on a running box | `./le test functional plugin` run on `ddos-flowspec-removed-section-orphan-cleared` |
| The local commands work in both states | `data-rm-refused-live`, `data-write-live`, `data-import-live`, and the existing offline `.ci` files of `ze data` still green |
| No content in the audit log | `data-write-live` reads `show audit` |
| Pages agree with the code | `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Authorization | The four writes are denied to the read-only profile on every surface, by their verb; `show data check` reveals key names only where `show data list` already does |
| Secrets in records | The `content` of both update commands is absent from the audit log, the accounting record, the daemon log at every level, and every refusal that echoes an argument |
| Input bounds | `content` is bounded by its declared length before the handler; `key` and `name` are bounded and validated as store keys and plain config names; a decoded content is never trusted as a path |
| Path confinement | `request data repair path` follows the backup path rules and cannot write inside the store's folder entries |
| Lockout | The command reference names the runtime keys that hold credentials, so an admin knows what a removal of one does |

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

- The removal needed no new store capability. The whole design is routing: a command the daemon already knows how to perform reaches the handle that may perform it.
- Confining the key commands to runtime state does not protect credentials: the login and HTTPS keys are runtime state. What protects them is authorization, and the command redaction protects their values in the records.
- The report's defect was never its wording. Rewording it on 2026-11-13 made it honest and left the operator where they were. The fix is the route.
- `ze config import` has the same gap as `ze data import`: refused beside a running daemon, with no daemon form. `update data config` is the daemon command it sends. Thomas brought it into this spec at the final gate (AC-16).
- Triple challenge. Simplicity: five commands reuse one target, one module and existing store methods; the two additions are one keyword in the command redaction, which the content forces, and the owner reader, which the saved-file case forces. Uniformity: builtin RPCs beside the two that exist, the local route `ze doctor` takes, the path rules of a backup. Performance: operator commands off every hot path; the content is decoded once into one allocation bounded by the argument length.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `fix-command-refused-in-the-state-it-reports.md` | 2026-11-12 | FlowSpec orphan report names `ze data rm`, refused under the running daemon |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Clear a removed section's record with the withdrawal, then `ze data rm` through the daemon (Thomas, 2026-11-12, option (c)) | (a) plugin restart, which cannot run without the section; (b) a plugin command that withdraws and forgets in one step, a new surface | Owner decision; the owner rule already owes the daemon route |
| This spec builds the daemon route of all four data write verbs: `rm`, `write`, `import` and `repair` (Thomas, research gate, 2026-11-14) | `rm` only, with a spec each for the other three (the recommendation put to him); `rm`, `write` and `import` | Owner decision. The other three are owed by the same owner rule, and he keeps them in one spec |
| The daemon removal is `delete data key <key>` (Thomas, research gate, 2026-11-14) | `request data remove key <key>`, beside backup and restore | The verb catalog tests `delete` ("something that existed stops existing") before the residual `request`. `docs/architecture/cli/command-verbs.md` gains a store key as a third kind of thing `delete` acts on |
| With a daemon running, the local `ze data rm` sends the removal to the daemon itself (Thomas, research gate, 2026-11-14) | Refuse with a hint naming the daemon form, as a refused backup does | The command works in both states with the same output; the local route stays for when no daemon answers. It follows the route `ze doctor` already takes to its daemon form |
| The daemon removal reaches runtime state only: a key under `meta/` outside `meta/config/` (Thomas, research gate, 2026-11-14) | Plugin-granted keys only; any key, as the offline command allows | A configuration key removed raw bypasses versions, pointers and history and tells no managed client. The store's own runtime-state split (`runtimeKey`) names the set, and a refused configuration key names the command that manages it |

| Caller-side bytes travel base64-encoded in a `content` argument (Thomas, design gate, 2026-11-14) | A path on the daemon's host, as `request data restore path` takes; both arguments | No mechanism carries caller bytes to the daemon today. A declared-length argument already carries data on every surface (`request peer notify`), so no new channel is built. The bound is what one SSH packet carries, the same on every surface: 261,120 base64 characters, 195,840 bytes of content (Thomas, 2026-11-14, after A-5 broke; the alternatives set aside were a larger bound for the remote surfaces only, content on the exec session's stdin, and a patched SSH packet cap). `content` stays the argument name though its masking also hides one token after the two config path elements of that name in command records (Thomas, same day) |
| The daemon write is `update data key <key> content <base64>`, create-or-replace (Thomas, design gate, 2026-11-14) | `create data key` and `update data key`, each refusing the other's case; `set data key` | It is what `ze data write` does. The verb catalog gains the sentence that `update` on a store key writes the key whether or not it existed |
| The daemon import is a versioned import through the reload, `update data config name <name> content <base64>` (Thomas, design gate, 2026-11-14) | A raw write of the config key, as the offline `ze data import` does | A raw write of an existing config is shadowed by its active pointer, so the daemon would keep the old config while the import reported success. The offline command gets the same fix |
| `request data repair path <absolute-output>` and `show data check` are both built (Thomas, design gate, 2026-11-14) | Repair only, with check in a spec of its own | A remote operator who sees the store integrity warning can check and salvage. The doctor text that says to stop the daemon for both is corrected |
| `ze config import` beside a running daemon sends itself to `update data config` (Thomas, final gate, 2026-11-14) | A spec of its own; leave it offline only | The same owner rule owes it, and the daemon command exists by phase 6 |
| The `content` value is redacted in every record of the command, by `content` joining the keywords command redaction answers for (confirmed by Thomas, final gate, 2026-11-14) | A schema marking on the leaf honoured by a new mask | The design page gives a command line one redaction entry point, by keyword name. One keyword covers the accounting record, the audit log, the SSH exec log and the session transcript |
| A key whose owning plugin runs is refused by both key commands (confirmed by Thomas, final gate, 2026-11-14) | Leave the guard out and only warn in the report text | The removed-section report can fire while the plugin still runs with a live rule, because doctor judges the saved configuration. A warning does not stop an operator who follows the report |

## Known Limitations
- A content above 195,840 bytes cannot travel in one command. A larger config is restored from a backup file on the daemon's host with `request data restore path <file> config`.
- `request data repair` writes a salvage copy. Installing a repaired tree still needs the daemon stopped: no command replaces a live tree with another tree.
- `update data config` imports one config per command. The local `ze data import` with several files sends one command per file, so a refusal of the third leaves the first two imported, as the offline command does.

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
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le test unit pkg path ./internal/component/config/storage/cli/`, `./le test unit pkg path ./internal/plugins/ddos/flowspec/`, `./le test unit pkg path ./internal/core/statestore/`, `./le test unit pkg path ./internal/core/diagnostic/`, the package that holds the dispatcher's accounting record, and the nine functional tests of the test plan by name. No agent runs any `./le verify` command
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded with `./le spec review record`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-data-store-maintenance-through-the-daemon.md` only, in the same `./le commit create` script
