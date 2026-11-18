# Spec: Web UI, CLI and config one-off defects from the journal

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Priority | P1 |
| Release reason | Each row is incorrect operator-facing behaviour: a secret echoed, a commit not audited, a list shown as empty, a YANG statement ignored, markup that does not render |
| Depends | - |
| Phase | E1 backup-folder commit activation implemented; unrelated parent work stopped |
| Handoff | verify |
| Updated | 2026-11-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The 2026-10-23 journal sweep left about 265 open rows that fit none of the
thirteen root-cause specs. This spec owns the 34 in the web UI, the SSH CLI and
editor, the command and pipe layer, YANG loading, and rendered markup (web pages,
website CSS, generated docs). Each row is a claim until a red test reproduces it
at HEAD.

The current correctness pass also owns read-only discard audit snapshots and
discarding saved leaf changes. Inspection must preserve durable bytes, and an
explicit discard must not let saved ownership reappear after reopening.

The instruction to implement this spec covers the whole agreed scope. Stopping
after the backup amendment did not satisfy that instruction. Following the
owner's correction on 2026-11-17, resume the remaining acceptance criteria and
owned journal rows; retain completed work and the explicit P6e release exception.

Left to root-cause specs: `yangTypeToValueType` and `validateYangType` (S1);
the `website/AI.md` checklist and the stranded-documentation rows (T2, T4).

## Required Reading

### Apply backup-folder changes on commit (2026-11-18)

Owner follow-up, verbatim, after asking about activation and permissions:

> please make sure the code does

The requested correction supersedes D1's restart-only selection and lazy
folder checks. It is the current authorized scope; all unrelated parent work
remains stopped. A successful config commit must select the new backup folder
without restarting Ze. Folder preparation must create missing directories and
prove the daemon can create private files before accepting the change. Existing
parent permissions, ownership and unrelated contents remain operator-owned.

| AC | Observable outcome | Permitted survivors and reason |
|----|--------------------|-------------------------------|
| E-AC-1 | A real accepted config commit changes where subsequent uploaded backups and system-download working files are stored, without restarting | Existing workspace identities and files stay usable in their original private roots; switching destinations must not destroy active work |
| E-AC-2 | Commit preparation creates a missing folder privately and proves actual daemon-user write access; a file, unusable path or live-store-owned namespace refuses with a useful error | A refused commit leaves the previously accepted directory and live store unchanged; pre-existing parent modes, owners and unrelated files survive |
| E-AC-3 | A later transaction or publication failure never activates its candidate directory | The previous accepted folder and all open backup work remain available because acceptance, rather than tentative apply, selects a destination |
| E-AC-4 | True OS overrides retain priority; removing the YANG setting restores the existing Linux/appliance/default policy | Other environment settings keep their existing behavior; YANG-derived values must not become permanent OS overrides |
| E-AC-5 | Repeated destination switches preserve private permissions, quotas, expiry, cleanup debt and final cleanup across all owned roots | User-owned parents and unrelated files remain; empty retired private roots must not accumulate |

Existing mechanisms under investigation: the server's original
`DeferReloadAcceptance` scope owns the whole reload/publication decision;
`ReloadConfig` owns serialized runtime transaction admission; artifact `Owner`
owns account-bound files and cleanup; storage `CheckBackupDirectory` owns
live-namespace protection. Ordinary tree validators and asynchronous provider
watches cannot provide a fallible commit-time preparation and accepted-only
activation. The implementation will extend the existing transaction boundary
through registration, keeping backup-specific names out of generic engine code
and avoiding a global environment reload redesign.

E1 implementation extends the original server reload acceptance scope with an
owner-registered native preparation callback. The callback receives active
candidate data and a per-server generation. It performs fallible folder
preparation before the no-diff exit, and the original outer scope completes the
prepared value only after its accepted or aborted decision. Backup activation
is an infallible scalar selection; a newer failed candidate cannot block an
older accepted one, and an older acceptance cannot overwrite a newer accepted
selection. Initial owner attachment reads the actual running tree; generic
server startup does not prepare backup paths before the live store is installed.

The backup owner creates no private root for an unused candidate. The next
allocation after acceptance uses the selected parent; existing item paths stay
in their original roots. Occupied roots remain bounded by the existing artifact
inventory, empty retired roots are pruned, and shutdown removes all owned roots.
The existing storage namespace check runs before parent creation and the private
write probe. User-owned parent directories are not removed or permission-repaired.

| E1 execution | Observed result |
|--------------|-----------------|
| Behavioral RED | Native `TestBackupDirectoryCommitSelection` compiled and reached real `ReloadConfig` plus outer acceptance in all three subcases. Accepted and outer-abort cases found no prepared folder; an unusable file was incorrectly accepted. `job-unit-pkg-6e3c832e.log` exited 1 before production changes. |
| Source boundary | Generic registration/acceptance implementation is isolated in plugin server; artifact/config CLI own directory parsing, probes, accepted selection and root lifetimes. No new RPC, plugin process, archive policy or other parent work is added. |
| Native transaction GREEN | The ten original `TestConfigPreparation` roots passed by name in `job-backup-commit-transaction-c5d35180.log` (0.799s); count-mode measurement covered 39/42 statements in the new producer. A further completion-error test was added to prove other owners still finish and remains in the final race selection. |
| Independent test oracle | Revised patch `70f407a7dc3fe57bff20964ad0e60b3d510926eef3af6de8611297d5cb0a0f84` was independently approved for the two existing YANG/OS tests. Both now receive the actual parsed running configuration through a real coordinator/attached HTTP owner. All original permission, sentinel, upload/download and cleanup assertions remain. |
| Initial full feature GREEN | Full artifact/config race suites passed in `job-backup-commit-owner-207e302c.log` (113.951s / 45.951s), including all six real-commit outcomes and actual permission-probe checks. |
| Reached acceptance GREEN | `job-backup-commit-reload-a368b669.log` passed all 24 selected roots: eleven native preparation roots, eight existing reload/rollback/shutdown roots, two actual hub candidate-publication roots and three artifact command roots. No skips. Native producer measurement reached 40/42 statements (95.2%). |
| Path-lifetime RED | Additional tests reproduced deletion of an occupied root when the configured parent contained `/./`, and acceptance of a linked parent containing `/../`. `job-unit-pkg-b50b71fd.log` exited 1. The source fix must normalize allowed absolute parents and refuse parent traversal before folder preparation; this is required to preserve E-AC-1/5. |
| Path/lifetime GREEN | The two actual path failures and three additional transfer/debt roots passed under race in `job-backup-commit-path-green-3f412b5a.log` (2.296s). No assertions were weakened. |
| Final workspace GREEN | Full artifact race tests passed after normalization in `job-backup-commit-final-owner-df60591d.log` (113.676s, 70.2% package statement coverage), including all six commit outcomes, unchanged existing workspace tests and every new transfer/privacy/cleanup case. Earlier config, hub, command and generic transaction source remains unchanged from its recorded passing runs. |
| Final focused GREEN | After the two lint corrections, all eleven native preparation roots and the real commit/YANG/OS selection roots passed under race in `job-backup-commit-lint-fix-tests-3b9a391a.log` (1.970s / 5.849s). |
| Final lint boundary | Host and Linux scoped lint have no findings in E1's 24-file commit manifest. Both retain six findings in unrelated server dispatch/route-admission files outside this change; `job-backup-commit-final-lint-d664c57d.log` remains red and is not represented as a full lint pass. E1 behavior and focused checks are complete; the broader parent remains open. |

| Lifecycle scenario | E1 disposition and proof |
|--------------------|--------------------------|
| Reload/commit during shutdown; cancellation during preparation | Selected: native preparation must obey the existing transaction admission fence and abort without activation; controlled server tests exercise the boundary without timing races |
| Verify/apply refusal, timeout or crashed participant | Selected return boundary: failed runtime reload aborts its local preparation; existing transaction failure tests plus deterministic native preparation tests observe preserved selection |
| Outer publication/listener failure | Selected: actual `DeferReloadAcceptance(false)` after successful inner reload leaves the old folder selected; controlled commit test observes open workspace/download bytes |
| Plugin process failure policies, respawn bounds, startup stages and tier barriers | Unreached: no process registration, spawning, failure policy, handshake or startup callback is changed |
| Plugin exits during stop; signal during teardown; startup command dispatch | Unreached process behavior: no shutdown drain/teardown or command-dispatch implementation changes; only existing server-owned artifact cleanup expands to its retained roots |

### Configure the backup working directory (2026-11-18)

Owner request, verbatim:

> can we set the backup folder in the yang as it can not be the place on linux and the appliance

This follow-up is the current authorized scope. It extends the completed C1
system-backup feature; unrelated parent work remains stopped. The setting
controls server-side temporary artifact/workspace files, not the browser's
download destination or a new archive/scheduling policy.

| Item | Decision or constraint |
|------|------------------------|
| Configuration | Add `environment/backup/directory` in the existing always-linked `ze-hub-conf.yang`; register `ze.backup.directory` and complete the environment extraction/plumbing |
| Existing lifecycle | These environment leaves apply at daemon startup, with OS environment taking precedence; changing the configured directory requires restarting Ze, as the existing environment contract states |
| Default | Reuse `paths.DefaultDataDir()` plus `backups`: installed Linux uses `/var/lib/ze/backups`, appliance uses `/perm/var/lib/ze/backups`; an unconfigured unknown developer layout retains the existing temporary-directory fallback |
| Explicit path | Require an absolute directory; invalid or unwritable explicit values refuse with a useful error and never fall back to a different location |
| Shared owner | All uploads, downloads and temporary artifact copies use the same configured parent and existing quotas/expiry/doctor reporting; a private random child remains owned by its original server |
| Preservation | Never chmod or delete an existing parent or unrelated contents; server shutdown removes only its own private root, and active workspaces are not relocated |
| Boundaries | No web-only setting, new module, new RPC, archiver, orphan sweep, whole-environment reload redesign or other parent implementation |

| AC | Observable outcome | Permitted survivors and reason |
|----|--------------------|-------------------------------|
| D-AC-1 | A real nested YANG setting reaches environment extraction/plumbing and a shared owner's actual upload and system-backup files appear beneath that parent | Parent sentinels and all unrelated entries remain, because only the private child belongs to this owner |
| D-AC-2 | An explicit OS value wins over the YANG value; absent settings resolve through the existing platform data policy | Existing unknown developer/test layouts retain their temporary behavior; no new production path mapping is copied into the artifact owner |
| D-AC-3 | Relative and unusable explicit directories refuse before retaining artifact bytes, with no hidden temporary-directory fallback | The source store and existing parent contents survive every refusal |
| D-AC-4 | Owned roots stay private and disappear on normal owner shutdown while their parent remains | Active owner roots keep their selected directory for their lifetime; a new daemon uses the new startup setting |

Data flow: existing hub YANG → config parser → `ExtractEnvironment` and
`ApplyEnvConfig` → the registered paths key → artifact `ensureRoot` → the
existing bounded file operations and cleanup. The directory string is the only
new configuration carrier; command responses and download formats are unchanged.

| Phase | Files and evidence |
|-------|--------------------|
| D1 tests first | New `artifact/directory_config_test.go` parses actual YANG and exercises real owner/HTTP operations; its missing schema/ignored directory assertions are observed red before implementation |
| D1 implementation | Existing hub YANG, config extraction/plumbing, core paths setting/resolver, artifact root selection; focused new platform/path/ownership tests |
| D1 docs | Existing web guide, web-interface architecture and environment/path reference pages receive the actual setting, platform defaults and restart semantics |
| D1 verification | Named directory tests, reached artifact/config/paths and backup regressions, scoped lint and affected schema checks; no whole-tree verification |

Risks: a missing extraction row would leave a valid-looking leaf inert, so the
test checks the real environment entries and file location. A writable default
must come from the existing platform policy, and explicit failures must not
silently select another folder. Directory preparation and cleanup must preserve
unrelated contents; tests retain a sentinel and reopen actual artifacts.

| D1 execution | Observed result |
|--------------|-----------------|
| Wiring RED | Native `test unit pkg path ./internal/component/config/artifact/ run '^TestBackupDirectory(YANGSelectsSharedArtifactRoot\|OSOverrideWins\|ExplicitRelativePathRefuses)$'` reached all three new tests: the two schema cases failed with `unknown field in environment: backup`, and the explicit relative directory was ignored instead of refused. No production implementation existed in that run. |
| Implementation boundary | Source is written; Main's formatting, named green tests, reached regressions and lint remain pending. No pass is inferred from source alone. |

D1 preservation addendum: the configured parent must be checked against the
actual installed live store before creating directories. Generic uploads bypass
the system-snapshot producer's output-name check, while every file beneath the
live database is interpreted as a raw stored key. Reuse the storage owner's
reserved-path policy through the existing shared server attachment; refuse the
live tree and store-owned canonical/staging names, including descendants and
resolved existing links. Ordinary sibling backup directories remain permitted.
This enforces D-AC-3; it introduces no new storage or reload feature.

| Additional D1 execution | Observed result |
|-------------------------|-----------------|
| Initial GREEN | Full verbose race tests of artifact, config and paths passed; all nine directory roots appeared by name in `job-backup-directory-green-728f713e.log`. This precedes the live-namespace guard. |
| Live-namespace RED | `TestBackupDirectoryPreservesLiveStoreNamespace` reached the actual shared-server upload with the actual installed store. Live tree, descendant, absent canonical blob name and its descendant incorrectly returned 303 instead of refusing before creation; native `job-unit-pkg-6505335a.log` exited 1. |
| Final feature GREEN | After preserving the existing injected-parent failure seam, full artifact race tests passed in `job-backup-directory-owner-final-5b42b201.log` (110.066s). All ten directory roots, all six live-namespace subcases, both existing doctor/error-path tests and the existing workspace regressions explicitly passed. No assertions were weakened. |
| Reached regression evidence | Full config and paths passed in the initial GREEN; full storage (33.879s), full storage CLI (9.020s), owner attachment and both registered system-snapshot command roots passed in `job-backup-directory-final-c54585ce.log`. |
| Broader CLI boundary | The full CLI package is not green: `TestRepositoryConfigsValidate` reported unrelated missing BGP process grants and reserved interface-name fixtures, and the cumulative package reached its eight-minute timeout while registering a running-config test server. Those sources are unchanged by D1; no parent completion or whole-tree verdict is claimed. Scoped lint remains pending. |
| Final scoped lint | Native lint of config, artifact, config CLI, storage, storage CLI and core paths exited 0: host and Linux/integration analyses each reported zero issues in `job-backup-directory-lint-31139001.log`. D1 verification is complete within its feature scope; the broader CLI boundary and open parent remain as recorded above. |

### Create and download a system backup (2026-11-18)

Owner request, verbatim, after identifying the missing browser creation action:

> ok please add this feature and nothing else do not drift

This amendment alone is the current implementation scope. The earlier Task
paragraph directing broad continuation is historical and does not authorize
resuming the other parent obligations. The existing uploaded-backup editor
remains intact. The parent stays open; this amendment does not close it.

| Item | Decision or constraint |
|------|------------------------|
| Requested behavior | Add **Create backup of this system** on `/backups/`; its POST downloads a complete `.zefs` of the hosting daemon's store |
| Existing producer | `storage.Backup` owns the coherent whole-store guard and copies every raw key, including drafts, history, credentials and binary data |
| Existing transfer authority | `config/artifact.Owner` owns account-bound snapshots, quotas, expiry, cleanup debt and shutdown; use its existing read/abort transport |
| Existing command boundary | Config CLI already adapts storage CLI producers; keep the web handler a thin authenticated command adapter |
| Selected approach | Add `request config artifact system snapshot`, returning the existing immutable transfer shape; add a browser POST adapter and native form |
| Rejected alternative | Making artifact identity optional on the existing private-artifact download command would blur source selection and weaken its current mandatory identity contract |
| Preservation | Uploaded workspaces, live committed and pending state, other accounts and original files remain unchanged; no restore, commit or routing action is invoked |
| Schema and discovery | Declare the command in `ze-config-artifact-cmd.yang`, register its RPC and document it beside existing artifact operations; no listener, feature flag, dependency or new process |
| Resource policy | Refuse oversized output and exhausted allowance using the shared owner; remove failed or completed private output through its existing cleanup-debt handling |
| Validation boundary | A complete backup need not contain a configuration; do not require the uploaded-editor configuration picker to accept a system snapshot |

#### C1 acceptance and wiring

| AC | Observable outcome | Permitted survivors and reason |
|----|--------------------|-------------------------------|
| C-AC-1 | An authorized user submits the native creation form and receives a `.zefs` attachment that reopens with all source keys byte-identical | Saved drafts, history, credentials and binary keys survive because this is a complete store backup |
| C-AC-2 | Creation and download leave the live store and all uploaded workspaces unchanged | All existing live and offline work survives because creation is read-only with respect to those sources |
| C-AC-3 | Anonymous, read-only, cross-origin, unsupported-method and unavailable-store requests refuse without disclosing source bytes | Existing authorized workspaces stay usable; a refused creation has no source mutation |
| C-AC-4 | Snapshots obey owner/account quotas and size limits; failed creation and completed, failed or cancelled downloads release their owned temporary output | Existing snapshots remain available until their own close/expiry; cleanup failures remain explicit admission debt |

Entry point: `pageBackupWorkspace` posts to the registered backup subtree;
`backupWorkspaces.serve` dispatches the declared snapshot command and streams
its returned transfer. The config RPC reaches `BackupSystemTo` on the installed
data target, through the shared artifact owner, and `storage.Backup` supplies
the whole-store copy. HTTP/native browser tests reopen the actual attachment;
owner and storage tests exercise limits and cleanup. Main runs scoped red and
green tests; source agents run no code.

| Phase | Files and evidence |
|-------|--------------------|
| C1 tests first | New `artifact_system_test.go`, `backup_system_test.go`, `backup_system_integration_test.go` and `system_snapshot_test.go`; registered route/command reaches a behavioral refusal before implementation |
| C1 implementation | Web backup route and page; artifact snapshot producer; config CLI handler/registration; storage CLI live-source adapter; the artifact command YANG declaration and its generated glue only as required |
| C1 documentation | Narrow additions to the existing web guide, web-interface/web-components architecture, storage-backends and command contract where changed |
| C1 verification | Scoped new snapshot tests plus existing backup/transfer tests; native browser creation/download/reopen; reached storage tests, lint and generated-template/YANG checks |

| Risk | Early signal | Mitigation |
|------|--------------|------------|
| C-R1 | Download is plain config text or omits a key class | Reopen the binary artifact and compare all source raw keys, including a non-UTF-8 value |
| C-R2 | Browser chooses a server path or bypasses ownership | Use only a server-owned private transfer and require normal authentication, edit authorization and same-origin POST |
| C-R3 | Large backup escapes quotas or failure leaves output | Enforce the admitted allowance in the shared owner/backup producer and retain failed cleanup as debt |
| C-R4 | Source commits race the copy | Reuse the source write guard held by the backup producer throughout the whole key walk |

Integration dispositions: command/YANG registration and its inventory are
required; no new configuration option, environment variable, runtime dependency,
metric or BGP behavior. Existing backup-storage diagnostic reporting covers
storage failures. Existing artifact expiry/abort/shutdown owns lifetime.
Changed-wire census is additive: the YANG command, config CLI RPC registration
and handler write the existing transfer response; `backupWorkspaces.command`
decodes it, `read` consumes its chunks and `abort` releases it. Existing download,
upload, source and external command callers retain their current grammar and
response fields. The wire-method inventory receives exactly the added RPC.

| C1 execution | Observed result |
|--------------|-----------------|
| Wiring RED | `TestArtifactSystemSnapshotRegistered`, run through `CGO_ENABLED=0 ./le --name web-cli-fixes test unit pkg path ./internal/component/config/cli/ run '^TestArtifactSystemSnapshotRegistered$'`, failed at the real dispatch with `unknown command` before implementation. The new test compiled and reached its intended assertion. |
| Storage and artifact regressions | Native `test unit pkg path ./internal/component/config/storage/ path ./internal/component/config/artifact/` passed both complete reached packages with race detection after the bounded producer and owner implementation. |
| Registered command GREEN | Native admitted `go test -race -v -count=1` with complete manifest feature tags and `-run '^TestArtifactSystemSnapshot' ./internal/component/config/cli/` passed `TestArtifactSystemSnapshotRegistered` and `TestArtifactSystemSnapshotRequiresEdit`. The former reopens the chunked binary artifact and compares every raw source key; the latter refuses a read-only caller. |
| Backend lint | Native `go lint run scope './internal/component/config/storage/ ./internal/component/config/storage/cli/ ./internal/component/config/artifact/ ./internal/component/config/cli/'` passed host and Linux/integration with zero issues after spelling-only corrections. |
| Additive expected outputs | Independent `ze-test-judge` approved exactly the new wire-method row and the creation-form additions to `backup_workspace--upload.html` and `get-backups.txt`; those approved changes are applied. Runtime inventory and golden comparisons remain separate. |
| Initial web build boundary, cleared by its owner | The first complete-feature integration attempt failed before execution because two concurrent L2TP fixtures still used `SessionEvents(uint16)`. Their owning task migrated them to `subscriberid.L2TPBirth`; this amendment changed neither file. |
| HTTP/browser GREEN | After that dependency changed, the unchanged native complete-feature integration selection `^(TestBackupSystemDownload.*\|TestBrowserBackupSystemDownload\|TestBrowserBackupWorkspace(Scroll)?)$` passed all seven named roots and all refusal/interruption subcases with race detection. The native browser downloaded and reopened the new system backup; the existing upload/edit/download and wheel journeys passed. |
| Wire inventory GREEN | `TestRegisteredWireMethods` passed against the actual complete-feature registry with the single approved new method. |
| Rendered expectations | Full `TestWebGoldenOutput` passed, including all backup variants. Handler cases `get-backups`, `get-backups-anonymous` and `get-backups-read-only` passed. The broader handler run remains red only on the concurrent `get-l2tp-session-json` capture's subscriber-identity changes; no unrelated capture was refreshed. |
| Page checks | Native assets and HTMX checks passed. Page generation succeeded; the broader template-output check reports only the concurrent `l2tp_detail_templ.go` as stale, and does not report the backup page. |
| Current validation boundary | Web lint produced no source-analysis verdict: its first launch lost shared cache entries, and its retry refused planning because concurrent `l2tp/observer.go` plus five dependency files changed. Backend lint and backup HTTP/browser acceptance are recorded above; web lint remains unverified until those inputs settle. No parent completion, whole-checkout green or deployment is claimed. |

### Offline backup browser workspace (2026-11-17)

Owner requirement, verbatim:

> web interface
>
> 5. Offline backup editing lacks a browser workspace.
>    Users need to upload a backup, inspect and edit it independently of the running system, then download the updated backup. That complete workflow is missing.

The owner approved this scope with "LGTM": upload a `.zefs` backup, select a
configuration, inspect and edit it, validate and review the changes, and download
the updated artifact with its other contents preserved. Workspace edits must
remain independent of the live configuration.

| Amendment field | State |
|-----------------|-------|
| Phase | Implementation 3/3: implemented, independently reviewed and scoped checks passed; parent verification handoff remains open |
| Owning mechanism | Existing web parity work in this spec; no separate backlog item |
| Parent status | Remains in-progress for its existing correctness work; this amendment does not change that work's acceptance or evidence |
| Implementation | Authorized by the owner's request to implement this spec; backup workspace phases below |

→ Constraint: Extend the shared configuration editor and artifact storage;
do not introduce a second configuration parser or a live publication callback.
→ Constraint: An artifact can contain configurations, history, pending changes,
credentials and unrelated keys. Inspect their producers before deciding which
records an edit may change.

#### Backup workspace research

Completed research checkpoints (the constraints below are retained design evidence):

- [x] `docs/guide/config-editor.md`, Backup mode; `internal/component/cli/editor.go`
  `NewOfflineSessionEditor` and `SetOfflinePromotion`.
  → Constraint: The offline editor already promotes candidate versions inside
  its supplied artifact. Reuse that publication mechanism; the constructor
  also loads an existing draft and labels its session origin local, so it is
  not an unchanged browser-session factory.
- [x] `docs/architecture/web-interface.md`, Per-User Editor;
  `internal/component/web/editor.go` `NewEditorManager` and `GetOrCreate`.
  → Constraint: A manager binds one store, configuration path and factory.
  Its username key alone cannot identify several independent backup workspaces.
- [x] `docs/architecture/web-components.md`, Design Principles;
  `internal/component/web/component_commit_bar.templ` `commitBar` and
  `component_breadcrumb.templ` `topbarActions`.
  → Constraint: Existing controls target absolute live-editor URLs. A mounted
  handler prefix alone cannot isolate backup requests. Audit navigation,
  redirects, forms, HTMX requests and chrome before reusing those controls.
- [x] `docs/architecture/storage-backends.md`, blob artifacts;
  `internal/component/config/storage/backup.go` `Backup` and `copyKeys`;
  `restore.go` `sourceConfigNames`.
  → Constraint: Preserve the complete artifact rather than export configuration
  text alone. The existing guarded key copy preserves binary values; the
  configuration-name census includes both active pointers and active mirrors.
- [x] `ai/doors/api-surfaces.md`; `internal/component/web/webroute.go`
  `WebRoute`; `cmd/ze/hub/service_web.go` `startWebServer`.
  → Constraint: Production discovers registered web routes and supplies their
  auth wrappers. The current route builder returns only a handler; artifact
  ownership and shutdown cleanup need an explicit lifecycle integration.
- [x] `cmd/ze/hub/editor_adapter.go` `newEditorFactory`;
  `internal/component/web/editor_history.go` `EditorManager.Close`.
  → Constraint: The live factory attaches the daemon's confirmation window;
  live service wiring also installs reload and publication callbacks. Backup
  editing must not inherit these. Manager close drains history but does not
  close its borrowed store or remove uploaded files.

**Research baseline:** Graphical import stages configuration text in the live
user's candidate. CLI backup mode edits a supplied artifact without a daemon.
There is no browser bridge between those workflows in the inspected producers.
The existing CLI/storage tests prove their own paths, not the requested browser
journey. No test was run for this research-only amendment.

| Existing mechanism | Producer | Research disposition |
|--------------------|----------|----------------------|
| Artifact editor and promotion | `cli.NewOfflineSessionEditor`, `Editor.SetOfflinePromotion` | Reuse the content-only editor and shared serializer, not the draft-resuming constructor; `PublishArtifactConfig` extends the ordinary guarded version/promotion producers without consuming pending intent |
| Per-user graphical editing | `web.NewEditorManager`, `EditorManager.GetOrCreate` | Reuse candidate methods and schema-backed inputs; add workspace identity outside the live manager |
| Complete artifact export | `storage.Backup`, `copyKeys` | Reuse guarded key preservation for download; do not substitute active configuration text |
| Configuration selection | `storage.sourceConfigNames` | Reuse the storage-owned census through an exported seam if needed; do not duplicate its key grammar in web |
| Production route registration | `web.RegisterWebRoute`, `hub.startWebServer` | Extend the registered surface and auth wrappers; explicitly own workspace lifetime |
| Existing graphical chrome and actions | `commitBar`, `topbarActions` | Reuse only after explicit URL scoping; live command dispatch and tools must not enter the artifact workflow |

**Required data flow:** authenticated upload → bounded private artifact →
integrity check and configuration selection → artifact-backed editor →
validation and diff review → publication in that artifact → complete binary
download → owned workspace cleanup. The user's original file remains on their
machine. A web-server process may host the workspace; no edit may require or
publish a live configuration change.

| ID | Assumption | Basis | If wrong | Validation method | State |
|----|------------|-------|----------|-------------------|-------|
| A-B1 | Open the committed configuration first and retain existing drafts separately | Other backup contents must survive; CLI backup mode currently loads a saved draft | A user expecting draft resumption sees a different starting point | `TestBrowserBackupWorkspace` opens committed text, excludes actual editor siblings from the picker and checks every preexisting pending/draft raw key after download | Confirmed by native browser and storage preservation runs |
| A-B2 | Inspection and validation use the hosting process's available schema | `backupPrepare` uses the shared validator and content editor; uploaded plugin declarations are never executed | A backup from another build may contain unsupported fields | `TestBackupHTTPMalformedSourceKeepsArtifact` and `TestBackupHTTPUnavailableSchemaKeepsArtifact` retain useful raw inspection, refuse validate/review/save with precise diagnostics and preserve every downloaded raw key | Confirmed in final named run; no target-host compatibility claim |

| ID | Risk | Early signal | Required mitigation |
|----|------|--------------|---------------------|
| R-B1 | A reused link or callback reaches live state | Any rendered backup control targets a live editor route; live sentinel changes | Explicit request scope and separate artifact owner; browser and callback-negative tests |
| R-B2 | Old pending changes are silently applied or lost | Opening a backup changes its source records or presents an unexpected candidate | Resolve A-B1; test saved drafts and multiple accounts/configurations |
| R-B3 | Credentials leak or uploads exhaust storage | Unauthorized download, raw keys in markup/logs, unbounded request or retained files | Authorize full-artifact access; private files; bounded size/count/disk use; cleanup on close, expiry and shutdown |
| R-B4 | Download races editing or cleanup | Inconsistent active pointers or a truncated artifact | Serialize export with edits and lifetime ownership; reopen downloaded artifact and verify unrelated keys |
| R-B5 | Offline validation is described as target-system acceptance | Host/runtime checks are implied by an artifact save | Label the validation boundary and preserve precise unsupported-schema refusals |

Mitigations now observed: the B3 raw-key/live-candidate proof addresses R-B1
and R-B2; HTTP authorization, size/debt and cleanup tests address R-B3;
serialized full-artifact export plus normal restore reopening addresses R-B4.
The UI and guide label R-B5's hosting-process boundary; the dedicated malformed
and unsupported committed-source cases now prove precise refusal and retention.

The implemented design uses an opaque URL per account-owned workspace, starts
from committed bytes, bounds uploads and retained artifacts, serializes edits
and export, and joins admitted HTTP work before cleanup. The producer/consumer
census below names its complete request surface. Native browser proof uploads,
selects, edits, validates, reviews, downloads and reopens a real artifact while a
separate live-store sentinel and unrelated binary artifact keys remain unchanged.

Discovery and documentation surfaces: `features/backup-and-restore.md`,
`ai/INDEX.md`, `docs/guide/web-interface.md`, `docs/guide/config-editor.md`,
`docs/architecture/web-interface.md`, `web-components.md` and
`docs/architecture/storage-backends.md` if its private census gains an exported
contract. The design's integration checklist must name the workspace storage
and cleanup diagnostics, authorization and same-origin wrappers, route lifetime,
template generation, browser transfer proof and reached CLI/storage regressions.
No CLI grammar, YANG syntax, plugin protocol, BGP wire format or new listener is
required by the approved scope.

Retained session state:
`tmp/session/2026-11-17-171730/state/session-state-web-cli-config-one-off-defects-d7cbdbd9-12ca-464f-b62b-76070d9eef5c.md`.
It holds implementation handoffs and may be removed after the backup amendment's
verification handoff is recorded durably and no active session needs it. The
superseded research digest was removed after its constraints entered this spec.

#### Backup workspace implementation plan

The owner requested implementation of this spec after the research report.
Use its existing mechanisms and the approved upload-to-download scope. The
backup amendment does not claim or take over the parent's unrelated journal
rows or other sessions' active corrections.

| Approach | Benefit | Cost and disposition |
|----------|---------|----------------------|
| Dedicated backup workspace shell over shared artifact and configuration producers | Gives the complete artifact workflow an explicit owner and bounded request surface | Selected: retain shared parsing, validation, diff and publication; choose controls appropriate to inspecting and editing a full configuration |
| Make every existing Finder control workspace-aware | Reuses the full graphical presentation | Rejected for this implementation: all absolute links, live CLI/tools and live callbacks would need a wider routing migration before isolation was reliable |

| Phase | Work | Required evidence |
|-------|------|-------------------|
| B1 | Audit the current tree; register the authenticated backup entry point and write its failing upload-to-workspace wiring test | Named route test reaches the registered production handler and fails on the missing artifact workflow |
| B2 | Implement bounded private artifact ownership, contained-configuration selection, independent editing, validation/review, artifact-only save, binary download and close/expiry/shutdown cleanup; add typed templ presentation and discovery | Focused positive and negative HTTP/storage tests; binary and draft preservation; concurrent-owner and cleanup tests; reached CLI/storage regressions |
| B3 | Prove the real browser upload/edit/review/save/download/reopen path, correct any findings, run scoped checks and record the handoff | Real browser journey plus permission/refusal cases; separate live-state sentinel; downloaded artifact reopened with expected active content and unrelated keys |

| AC | Input / condition | Expected behavior | Permitted survivors and why |
|----|-------------------|-------------------|-----------------------------|
| B-AC-1 | Authorized operator opens backup editing and uploads a valid `.zefs` artifact | A workspace lists its contained configurations and opens the selected committed configuration | Existing saved drafts remain separate and byte-preserved because they were not selected for editing |
| B-AC-2 | Operator edits the selected configuration | Shared parsing and validation explain invalid input; review shows the changes; accepting valid changes publishes a new version in that artifact | Unchanged settings and unsupported-but-retained raw backup keys survive; invalid edits remain available for correction |
| B-AC-3 | Operator downloads after a successful save | A complete artifact downloads and reopens with the saved active configuration and coherent version/mirror pointers | Other configurations, credentials, binary keys and original draft records survive because this operation changes only the selected committed configuration |
| B-AC-4 | A live candidate exists while two backup workspaces are edited | Each request and download uses its explicit workspace and authenticated owner; live committed and pending bytes stay unchanged | Separate tabs and configurations remain independent; no workspace selection cookie redirects another tab |
| B-AC-5 | Anonymous, read-only, cross-origin or wrong-owner access; malformed, corrupt or oversized upload | Requests are refused before unauthorized disclosure or mutation, with an actionable message | An authorized owner's valid artifact remains usable after a refused edit or unrelated request |
| B-AC-6 | Workspace closes, expires, fails during creation or server shuts down | Active artifact operations finish before owned handles and temporary files are removed; admission and disk use remain bounded | The user's original upload and unrelated sessions/files remain untouched |

### Critical Review Checklist

| Check | What to verify for the backup amendment |
|-------|------------------------------------------|
| Isolation | Trace every backup form, redirect, save callback and download to the artifact owner; prove live committed and candidate bytes survive |
| Preservation | Compare every unrelated raw key, selected-config preexisting draft and other configuration before and after export; reopen using the normal restore reader |
| Useful outcome | The registered production route supports actual upload, edit, refusal correction, review, save and binary download; no helper-only acceptance |
| Authorization | Full artifacts can contain credentials: require the edit-capable operator and exact workspace owner on reads and writes; enforce same-origin mutations |
| Lifetime | Serialize export/edit/close, bound artifact count and size, join active requests before deletion and remove only task-owned workspace files |
| Validation | Use existing configuration producers, refuse unavailable schema precisely and label offline validation without implying target-host acceptance |

#### Backup integration checklist

| Integration point | Applies? | Implemented disposition |
|-------------------|----------|-------------------------|
| YANG schema | No | No RPC or configuration leaf added; reuse hosting schema |
| YANG native constraints | Existing | `backupPrepare` and serialized-byte validation use `cli.ConfigValidator.ValidateTransition` |
| Custom validators | Existing | Shared validator registry remains authoritative; no backup-specific validator |
| CLI commands/flags | No | Existing `ze config edit --backup` remains unchanged |
| CLI grammar | No | Native browser forms add no command grammar |
| Editor autocomplete | No | This page offers a full-text editor; no completion contract changed |
| Functional proof for new API | Yes | Production `/backups/` registry, HTTP tests and native `TestBrowserBackupWorkspace` |
| Pipes | No | Binary HTTP attachment is not an operational-command pipe |
| Environment registration | No | No environment option added |
| Runtime dependencies/doctor | Yes | Private files owned by `backupWorkspaces`; `backup_diagnostic.go` raises `doctor-backup-workspace-storage` through existing recent-events doctor |
| Doctor problem classes | Yes | Storage/cleanup failures use that code; network cleanup retains its existing code through `ShutdownNetworkError`; ordinary input/auth refusals remain actionable request responses |
| Metrics | No | No new protocol or telemetry contract; bounded ownership and storage failures have their existing error/report paths |
| BGP families | No | No SAFI, capability, attribute or routing behavior changed |

#### Backup HTTP producer/consumer census

Search scope: `internal/component/web` Go, templ, JavaScript and CSS plus
`cmd/ze/hub/service_web.go`; terms `backupWorkspacePath`, `backupWorkspaceURL`,
`backup-*`, `backupReviewToken`, `NavigationURL`, `Direct`, `OwnHandler` and
`ShutdownNetworkError`. Known controls are the registered subtree and
`pageBackupWorkspace` form producer. Documentation/non-Go carrier searches for
`/backups/` and `updated-backup.zefs` find the guide and architecture contract;
there is no shell/Python production carrier or JavaScript consumer.

| Carrier | Writer | Reader and preservation |
|---------|--------|-------------------------|
| `GET/POST /backups/`, multipart `backup` | `pageBackupWorkspace` upload form | Registered `HandleBackupWorkspace`, `upload`/`receive`; streamed private copy and integrity check |
| `/backups/<id>/`, query `config` | Redirect from `backupWorkspaceURL`; native selection form | `serve` checks owner map/account, then `edit` uses storage-owned name census |
| POST `action`, `config`, `content`, `base`, `review` | Native edit/close forms | `edit` validates bounds, stale baseline and review digest; `backupPrepare`/serializer use shared configuration producers; `publish` changes only the private artifact |
| `GET /backups/<id>/download` | Native download link | `download`/`export` emit complete ZeFS attachment; native browser downloads it and `ReadRestoreSource` reopens it after documented client chmod0600 |
| `PortalService.Direct` | Registered backup route metadata | `NavigationURL` and `HandlePortal`; default false preserves embedded services |
| Owned handler close/error | `startWebServer` calls `OwnHandler` before wrappers | `WebServer.Shutdown` joins requests, closes owners, and preserves combined error; hub projects only network causes for its network diagnostic |

No ZeFS layout, CLI grammar, YANG syntax, plugin protocol, BGP wire format or
appliance listener changed.

#### Backup documentation checklist

| # | Question | Disposition |
|---|----------|-------------|
| 1 | User-facing feature/evidence | Updated `features/backup-and-restore.md`; existing maturity/counts retained, no fabricated feature-run record |
| 2 | Config syntax | Unchanged |
| 3 | CLI command | Unchanged; `docs/guide/config-editor.md` crosslinks the browser alternative |
| 4 | API/RPC | Browser form contract documented in `docs/architecture/web-interface.md`; no machine RPC changed |
| 5 | Plugin | No plugin added or changed |
| 6 | User guide | Updated `docs/guide/web-interface.md`, including client permissions and validation boundary |
| 7 | Wire format | ZeFS and network wire formats unchanged |
| 8 | Plugin SDK/protocol | Unchanged |
| 9 | RFC behavior/proof | No protocol behavior or RFC proof changed |
| 10 | Test infrastructure | Existing native-host route widened to complete `^TestBrowser` family in `docs/architecture/testing/runner-architecture.md` and QEMU exclusion reason; no runner mechanism added |
| 11 | Daemon comparison | Unchanged |
| 12 | Architecture | Updated web-interface, web-components and storage-backends pages |
| 13 | Route metadata | No packet-route metadata changed; internal portal `Direct` field documented in web components |
| 14 | Prometheus | No counter added or changed |
| 15 | Registration/discovery | WebRoute/Portal registration and `ai/INDEX.md` entry; no plugin/event payload inventory added; storage event uses existing doctor registry |
| 16 | Source anchors/Design headers | Owning pages above updated; `docs/architecture/doctor-and-health-checks.md` existing recent-events contract is unchanged. Full docs run names no changed backup anchor failure; external catalog drift, unrelated `validate_route.go` anchors and a stale derived index remain separately recorded below |
| 17 | Examples | Existing CLI backup mutual exclusions preserved; new native upload/edit/download sequence and `chmod 600 updated-backup.zefs` checked against handlers/storage opener |

| Phase evidence | Observed result |
|----------------|-----------------|
| B1 wiring red, 2026-11-17 | `TestBackupWorkspaceRegisteredWorkflow` executed through the production route registry and failed at valid upload with expected 303, actual 501. The artifact fixture opened successfully and the test reached the temporary handler; this is a behavioral red rather than a compile or fixture failure. Command: `CGO_ENABLED=0 ./le --name backup-workspace test unit pkg path ./internal/component/web/ run '^TestBackupWorkspaceRegisteredWorkflow$'`. |
| B2 collision red and correction | `TestArtifactLegacyAdoptionPreservesOccupiedHistory` first showed an occupied pending-version hash and its resolved bytes replaced by the legacy mirror. After bounded allocation was shared by forward commits and backward adoption, that test and pre/post-acceptance and release-failure tests passed by name. |
| B2 census red and correction | `TestArtifactConfigNamesExcludesEditorRecords` first offered seven names instead of five, including the actual draft and per-user change record. Browser-only filtering now omits those siblings, preserves explicit active-pointer authority and standalone suffix-bearing names, and leaves the restore reader's raw census unchanged. |
| B2 complete storage regression | `job-unit-pkg-3247d5e2.log`: full storage package passed in 32.968 seconds with named artifact, collision, failure, census and legacy explicit-restore tests. This is storage evidence, not browser acceptance. |
| B2 reached CLI regression | `job-unit-pkg-56417296.log`: `TestChangePathHelpers`, `TestEditBackupNoDaemon` and `TestConfigBackupFamily` passed by name in the CLI/config-CLI packages. |
| B2 HTTP, doctor and lifetime | `job-unit-pkg-456299f3.log`: every `TestBackup*` and `TestWebServerOwnedRouteClosesAfterRequests` passed, including the registered route, warnings, auth/origin/corrupt refusal, wrong owner/stale review, budgets, cleanup debt, expiry, close failures and runtime doctor visibility. The earlier doctor's missing builtin-code fixture initialization was corrected without changing its specific-code assertion. |
| B2 reached hub cleanup | `job-unit-pkg-2f3a6acb.log`: `TestOwnedRouteCleanupIsNotANetworkFailure`, `TestWebServiceOverdueHistoryRetainsStoreUntilEveryAcceptedModeDrains`, `TestWebServiceShutdownSealsCLIHistoryBeforeOwnedStoreRelease` and `TestWebNetworkCleanupFailureReportsDoctorEvent` passed by name. This does not claim that run's then-failing web doctor fixture passed. |
| B3 complete native browser family | `job-browser-web-bab9ad19.log`: full `^TestBrowser` family passed in 99.699 seconds, including backup upload/edit/save/download/reopen, existing CLI transfers, notifications, recovery and history. |
| B3 native scrolling red/green | `job-backup-scroll-red-30684c7a.log`: 390×640 native wheel test reached its visibility wait and failed. Shared hidden-overflow roots caused the failure. Page-only CSS restores document flow; `job-backup-scroll-green-91774cd4.log` then passed `TestBrowserBackupWorkspaceScroll` (2.90 seconds) and the full `TestBrowserBackupWorkspace` (2.48 seconds). No programmatic scrolling supplies that proof. |
| Final source, isolation and surface regressions | `job-backup-final-tests-de1ebb0a.log`: selected web tests passed (16.100 seconds), storage tests passed (5.972 seconds) and owned-route hub cleanup passed (1.056 seconds). Named new tests are `TestBackupHTTPSameAccountWorkspacesStayIndependent`, `TestBackupHTTPMalformedSourceKeepsArtifact` and `TestBackupHTTPUnavailableSchemaKeepsArtifact`. All backup/fault/warning/owned-request cases, recovery tests and the six previously red template/handler golden, HTMX/assets and typed-view guards passed. This is the selected final population, not a claim that the earlier whole web/QEMU run passed. |
| Final native browser | `job-backup-browser-final-91774cd4.log`: `TestBrowserBackupWorkspaceScroll` passed (2.70 seconds) and `TestBrowserBackupWorkspace` passed (2.02 seconds), after the final page and descriptor corrections. |
| Broader diagnostic regression | Earlier `job-unit-pkg-0deb954f.log` passed the full diagnostic package (1.297 seconds). Its whole web run also found the corrected surface omissions and the separate L2TP form-guard failure; QEMU's package census found separate missing COS/imagestore dispositions. Neither failed broader package is presented as green. |
| Measured coverage | Final `scratch/backup-coverage.out` is count-mode data. Visited statements/total: `artifact_workspace.go` 33/36; `editor_record.go` 14/15; `backup_edit.go` 113/169; `backup_transfer.go` 107/147; `backup_workspace.go` 108/131; `server_cleanup.go` 4/9. Error fallbacks remain unvisited; this is not 100% branch coverage. |
| Final lint and page checks | `job-backup-lint-final-7686f882.log` passed the six declared packages across host, Linux integration, FreeBSD, OpenBSD, DragonFly, capability, appliance and compile-out flavors, each with zero issues. Native assets check passed; HTMX check inspected 830 files with no unexplained findings. `job-backup-templ-final-3a6486f3.log` passed after regenerating the breadcrumb with the canonical internal-relative source base; the resulting diff contains only the reviewed menu call and its generated source columns. |
| Documentation boundary | `job-backup-doc-final-ecef0555.log` is red: 14 external wiki/gh-pages catalog findings and seven unrelated `validate_route.go` source-anchor findings. It names no changed backup anchor failure. Native `doc index update` subsequently refreshed the two untracked derived discovery indexes; no full docs-pass claim is made. |
| Light precheck boundary | Native `repo check` reports 32 findings and `test weakened check` reports ten other RFC-carrier changes. The cross-package heuristic also names the new `HandleBackupWorkspace` and `NavigationURL`; their actual non-test consumers are the same-package production `RegisterWebRoute.Build` and generated breadcrumb template respectively, verified in source and the passing route/browser tests. The other findings concern existing CLI exports and concurrent BGP/L2TP/plugin/VPP/tooling work. These whole-checkout prechecks are not claimed green. |
| Commit static-flow correction | The native error inventory required the already-extracted `*opererr.Error` to reach `http.Error` directly. The two-line binding correction preserves the refusal's HTTP status and text; the current Go toolchain's `Root.Close` returns nil, so the prior deferred join carries no additional cause. `job-unit-pkg-8e6a49a5.log` reran and passed `TestBackupHTTPRefusals` and all four named subcases. `job-backup-refusal-lint-dc8bdca8.log` then passed the web package's selected host, Linux integration, FreeBSD, OpenBSD, DragonFly, capability and compile-out flavors with zero issues. |

Logs above are under `tmp/session/2026-11-17-171730/scratch/`. Keep only the
evidence needed for this implementation handoff; remove superseded run output
after the durable record and final scoped checks are accepted. No source copy
or persistent preview server is part of the deliverable.

#### Backup goal validation

| Goal | Concrete evidence beyond helper assertions | State |
|------|-------------------------------------------|-------|
| Upload, inspect/edit independently, download updated backup | Registered native browser journey uploads an actual artifact, selects committed text, corrects a refused edit, validates/reviews/saves, downloads a binary artifact and reopens it through normal restore; parsed values and active/mirror/version consistency are checked | Observed passing in B3 logs above |
| Preserve live state and unrelated backup contents | Same browser journey snapshots every raw live key and its candidate before requests, traps live publication, then compares them; downloaded raw keys preserve real drafts, per-user changes, pending version/object, second configuration, credential and non-UTF-8 binary bytes; user's original file bytes remain identical | Observed passing; full storage regression also passes |
| Keep two same-account workspace URLs independent | `TestBackupHTTPSameAccountWorkspacesStayIndependent` opens both forms before either save, validates/reviews different contained configurations, saves both, reopens both downloads with their own expected parsed values and preserves every unrelated original key | Observed passing in final selected run; complements the native live-state sentinel proof for B-AC-4 |
| Preserve inspectable artifacts when hosting schema cannot accept their committed source | The malformed-source and unavailable-schema HTTP cases show the exact raw textarea, require actionable refusal for validate/review/save with retained text and unchanged artifact bytes, then integrity-check a download and compare its complete raw key map | Observed passing in final selected run; A-B2 confirmed within the hosting-schema boundary |
| Restrict credential-bearing artifacts and bound their lifetime/resources | HTTP negatives refuse anonymous/read-only/cross-origin/wrong-owner access; stale/unreviewed requests retain a usable artifact. Fault-injected cleanup tests seal new allocation until removal succeeds; controlled request barrier proves shutdown joins requests before owner Close | Observed passing; scoped lint and count coverage recorded above; whole-checkout findings remain separate |
| Keep full workflow reachable in ordinary browser interaction | Native wheel test at narrow viewport observes initially offscreen download, scrolls with real wheel input, then requires complete visible link and center hit-test | Observed red before page CSS correction and green after it |

| Independent implementation review | Scope declared before the pass | Result |
|-----------------------------------|--------------------------------|--------|
| Round 1 | New artifact publication and integrity/census helpers, reached pointer/restore behavior, backup HTTP/UI, registered ownership and shutdown wiring, and browser preservation fixtures | Found occupied-history overwrite, nonrepresentative draft fixtures, missing publication-failure proof and unaccounted failed cleanup. Each is addressed in source/new tests; execution and fix-only review remain separate. |
| Round 2 | Round 1 fixes and reached siblings: bounded allocation and failure paths; shared editor suffixes and browser-only census with unchanged restore behavior; real intent fixtures; cleanup-debt admission/expiry/close; serialized validation and warnings; direct portal navigation and typed shutdown cause classification | Independent source review found no additional BLOCKER or ISSUE in this scope. Execution remained separate. |
| Round 3 | Later narrow corrections and their siblings: actionable artifact-owner Close failure; the doctor fixture's real builtin-code initialization with its original specific-code assertion retained; approved client-permission preparation after native browser download; complete native browser host-route declaration | Independent source review found no additional BLOCKER or ISSUE in this scope. Execution remained separate. |
| Round 4 | Narrow page-root scrolling CSS correction and native wheel proof; malformed/unsupported committed-source and two-workspace isolation HTTP proofs; canonical empty page-assets registration, additive template/route golden enrollment and independently approved typed-component census correction, their documentation and reached siblings | Independent source pass found no BLOCKER or ISSUE across all three lenses. Execution remained separate. |
| Round 5 | Concrete lint corrections only: existing save constant, rooted private upload/download descriptors and paired cleanup/documentation; recovery route constant consumption and two stricter test-helper checks, plus their reached callers | Independent source pass found no BLOCKER or ISSUE across all three lenses. Final execution remains separate. |

### Configured-plugin conformance after release (2026-11-17)

Owner scope decision, verbatim, about browser-triggered checks of configured
plugins:

> I am fine without this feature

> it can be considered after release

P6e is removed from this spec's release acceptance. Consider the configured-name
daemon operation after release; this decision does not authorize implementation
then. The local shell checker remains available, and its remote-execution
refusal remains enforced. Preserve existing implementation and draft tests;
neither their removal nor a completed daemon capability is claimed. This
explicit exception qualifies the browser parity requirements below.

### Explicit browser recovery of unreadable changes (2026-11-17)

Owner requirement, verbatim:

> web interface - please fix
>
> 4. There is no complete browser recovery for corrupted pending changes.
>    If a stored change file cannot be parsed, the browser refuses to open or discard it. An administrator currently needs file access to repair it. We need an explicit, authorized recovery action available remotely.

Acceptance: Finder and Workbench failures offer an independent recovery form;
an authorized, same-origin, explicitly confirmed POST discards only the
authenticated account's unchanged unreadable change file. The warning names
all origins, including SSH and console, because unreadable ownership cannot be
selected. It also names structural instructions retained in that file after
saving: those cannot be preserved selectively either. The separate saved draft
file, committed configuration and other accounts survive. Readable candidates
and stale confirmations are refused. Storage
failures report the actual or uncertain removal outcome, and cached candidates
cannot expose or restore discarded intent. Positive evidence must include the
real browser journey from failure through cancellation and confirmed recovery
to a usable editor, plus unauthorized negative cases.

Implementation: the independent `/config/recover` form and registered POST use
the existing discard authorization, same-origin middleware and audit sink.
`TestBrowserConfigRecoveryFromBrokenFinder` passes the native browser journey
through cancellation, required confirmation, removal, reopening and a new edit
(2.80 seconds). `TestCorruptChangeRecovery*` covers guarded identity, readable
candidate refusal and injected storage outcomes. `TestConfigRecovery*` covers
both graphical shells, authorization, preserved files, busy-editor refusal and
failed-refresh fencing. The route and admitted-editor regressions each failed
before their fixes. Existing corrupt-file refusal/discard tests pass. The new
component and handler captures pass; no unrelated fixture was refreshed.
Fault injection also reproduced a false retention claim after uncertain removal
and a raw parser echo from a concurrently recreated malformed file. Recovery
now fences uncertain state without reparsing it and redacts the replacement's
contents while retaining the confirmed removal outcome.
The final backend/rendering run passed every selected case except one obsolete
warning sentence; its independently approved correction then passed its named
rerun. Repository-wide documentation checks report published command-catalog
drift outside this change. Scoped lint produced no plan because another session
changed `internal/component/plugin/server/dispatch.go` and two dependencies
during its input survey; a completed lint verdict is not claimed.
The regenerated documentation indexes are current. Source-anchor checks resolve
the recovery pages and report seven unrelated stale `validate_route.go` symbols
in the API-command and YANG-design pages.
The broader parent spec remains open for its other work.

### Correctness before further parity work (2026-11-17)

Owner priority, verbatim:

> the full parity, identify what is missing but first ensure that we exists is correct.

Verify the existing graphical editor and browser CLI through their real entry
points before implementing the remaining missing capabilities. Preserve the
existing validation command contract. Record missing implementation separately
from incorrect behavior and missing acceptance evidence.

Executed regressions confirm that successful graphical commits drop their
warnings, finite binary downloads lose bytes, operational `configure` ignores
an argument or pipe tail, and cold web editors hide preserved candidate intent.
They also confirm that save and commit cleanup remove another session's intent
from a shared per-user change file. A delayed configuration-change event can
erase an already displayed commit advisory. The download, control-grammar and
warning corrections have passing targeted web regressions. Both direct and
staged commits preserve the other session's exact pending data after correcting
the test's metadata-aware readback. Live-save isolation has an executed
baseline, and recovery integration is applied locally after independent
lifecycle-fixture assessment. Focused recovery and storage-failure retries now
pass, as do the shared CLI replay and confirmed-rollback regressions. The full
CLI package's only current failures were two Linux-only XFRM cases on macOS;
fresh unchanged Linux acceptance now passes all four direct/merge and
commit/candidate branches. The broader web run exposed a
real discard-audit regression: inspecting an evicted editor resumes custody and
rewrites its stored timestamps. Its read-only correction is applied locally,
and the unchanged evicted-user regression passes. All four injected draft,
change-file, required-listing and final-release failures also pass their
targeted regression. All eight snapshot cases and the reached shared CLI and
daemon recovery cases now pass. Eighteen CLI-bar cases pass with the race
detector. The finite download correction also keeps operational warnings
outside the selected stdout: isolated empty, exact-limit and mode/format
regressions pass, followed by all 24 reached download and HTTP golden cases.
The CLI/web lint rerun passes with zero issues across its selected build
flavours; the preceding CLI/web/hub matrix reported no hub finding. Saved-leaf
discard has an executed failure:
discarded saved ownership and values return after reopening. No complete
correctness or parity claim is made.
Configured-plugin conformance is a post-release consideration under the owner
scope decision above, not a blocker for these existing paths.

### Authorized web correction amendment (2026-11-15)

Owner requirement, verbatim:

> Can you please look at the web interface? The default web interface should be the one allowing us to view the Yang graphically and the other one we have not completed is the backup because it's not finished. Can you check that effectively we can do all the things we do in the CLI on the web interface? It should have a graphical mode where you can click and modify things graphically, a CLI mode which effectively is the same thing as CLI but on the web page, allowing people who do not have SSH to do it via the web and the new interface. Please analyze the state of the code and the current situation. Provide me a report to help decide how we will approach this change, whether things require work or what to do.

Owner authorization following the report, verbatim:

> Please perform these corrections and fix the web interface.

This amendment owns the authorized corrections from that analysis. It does not
claim that the unrelated journal obligations below have been implemented or
closed. The existing schema, editor manager, command dispatcher and web mode
selector remain the mechanisms; no parallel editor or command service is added.

The CI follow-up owns the discard audit snapshot's cache-miss handling. A missing
cached editor does not mean the user's durable intent is empty: the snapshot
opens the existing editor and propagates an opening failure through every web
discard transport before mutation or audit recording.

Initial source audit (2026-11-15): `web.parseUIMode` selects Workbench for an empty value,
although the environment registration and YANG leaf declare Finder as default.
`web.buildFragmentData` leaves multi-valued and freeform schema nodes without
editors. Browser tests for Finder are unconditionally skipped. The web terminal
shares the editor and dispatcher but has its own command adapter; its differences
from the SSH editor require correction and targeted coverage.

| AC | Positive observable behavior | Permitted survivors and reason |
|----|------------------------------|--------------------------------|
| AC-31 | An authenticated browser with no mode selection opens the graphical YANG editor. An explicit Workbench selection still opens the experimental Workbench, and CLI remains directly reachable. Config, env help, labels and selection tests describe that contract. | Existing explicit mode cookies continue to select the operator's chosen mode. Workbench remains available because it is the requested alternative. |
| AC-32 | Schema-backed graphical editing offers the missing multi-valued and freeform node editors through the existing per-user candidate and commit path. | Purpose-built Workbench pages are not promoted to complete by this correction; its incomplete workflows remain labelled experimental. |
| AC-33 | Browser CLI corrections use the existing editor and dispatcher, with tests proving the corrected command workflows and state changes. | Unrelated journal rows remain open; the amendment makes no universal parity claim from a selected set of tests. |
| AC-34 | Finder browser scenarios run by default, and Workbench scenarios select their mode explicitly. Existing edit, review, commit and navigation assertions continue to execute. | Scenarios requiring a separately unavailable service or an unfinished experimental Workbench workflow may keep an independently justified skip. The owner requested that unfinished Workbench remain the backup; mode selection alone is no longer a skip reason. |
| AC-35 | A browser CLI streaming command displays successive daemon answers and provides the operator a working stop action, using the existing dispatcher and bounded output. | A finite command may use its existing single response; streaming must not be represented as a single completed answer. |
| AC-36 | The Workbench firewall connections page displays actual connection rows when supplied by the running backend, and explicitly reports when connection state cannot be read. | An empty table is permitted only when a successful read found no connections; backend absence or a read failure is not reported as that result. |
| AC-37 | The graphical editor provides the advanced configuration actions available through its shared editor: structural copy/rename/order, activation, review/validation and confirmed-commit controls, with the same candidate, authorization and publication semantics. Web discard audit snapshots reopen an uncached user's durable intent; an opening failure stops the action before mutation or audit recording. | Workbench remains experimental; an action refused by the shared editor remains refused with its reason rather than being approximated by the graphical adapter. A successfully opened editor with no intent for its owner may produce an empty discard detail. |
| AC-38 | A legal schema-backed list key remains addressable through graphical navigation, form actions and the browser CLI context, including keys containing spaces or slashes. URL and context-path encoding preserves the original key tokens before schema validation. | Invalid schema keys and malformed encodings remain refused. Escaping is a transport operation, not permission to bypass the shared schema or editor. |

Implementation phases for this amendment:

1. Correct mode selection, labels, documentation and browser mode declarations;
   retain the existing authenticated routes and explicit switch cookie.
2. Complete the missing generic graphical editors and the identified browser CLI
   adapters, with independent test changes judged before replacing expectations.
3. Provide browser CLI streaming and a working stop action.
4. Replace the Workbench firewall connection placeholder with actual rows or an
   explicit unavailable/error answer from the owning backend.
5. Complete the advanced graphical configuration actions through shared editor
   methods and confirmed-commit machinery.
6. Correct generic URL and browser-context path encoding so legal schema keys
   round-trip without being split into extra path tokens or refused as YANG
   identifiers. Reuse one encoder/decoder across navigation and action adapters.
7. Run scoped unit and browser proofs, review the corrections and commit them.
   Keep this spec open for its unrelated existing obligations.

Default-selection wiring is `buildWebService` -> `GetUIMode` -> `parseUIMode`,
with `ReadUIModeFromRequest` deciding an explicit current switch cookie. Graphical
edits remain `HandleFragment` -> schema metadata -> config handlers ->
`EditorManager`; operational CLI commands retain the daemon dispatcher.

Pages governing this amendment: `docs/architecture/web-interface.md`,
`docs/architecture/web-components.md`, `docs/guide/config-editor.md`, and
`docs/architecture/api/commands.md`. Evidence is the main thread's scoped runs;
no check of an unrelated original acceptance criterion is implied.

The experimental Workbench peer-table tool chain remains unfinished: its row
actions do not carry the tool identifier and selected-peer context required by
the related-tool handler. The two existing peer-tool browser scenarios retain
their complete assertions and name that missing capability explicitly. They
were already skipped at HEAD and are not evidence of working functionality.
Operational commands remain available through the dedicated browser CLI. This
amendment does not promote the unfinished Workbench to the default interface.

The parity checks also own a small declaration-producer correction in the
synthetic L2TP fixture. Its six command declarations and runtime handlers must
come from one literal table so the dispatch census can read the real command
population. The checker and command behavior remain unchanged.
The real browser download proofs also need an explicit host route in the
integration coverage census. Their required native browser is unavailable in
the default QEMU guest; the documented host route must execute all eleven
tests and fail when that dependency is missing.

Initial AC-38 source audit (2026-11-15): `extractPath` splits the decoded request path,
and `ValidatePathSegments` applies identifier-shaped validation to every token.
A list key is data rather than a schema identifier. The path adapter correction
must preserve its token boundary through escaping and leave actual key
validation with the schema. The main thread owns the separate narrow
implementation package and must not claim universal graphical addressability
until its positive and malformed-input cases have run.

### Web amendment implementation and evidence (2026-11-16)

The amendment is implemented. The original AC-1 through AC-30 and journal
obligations remain open; this record does not close the owning spec.

| AC | Implemented result and scoped proof |
|----|------------------------------------|
| AC-31 | Graphical YANG is the default; explicit Workbench selection remains available and labeled experimental. Mode-selection tests and browser mode-switch journeys pass. |
| AC-32 | Collection and freeform editors use the shared candidate. Graphical member editing, ordering, refusal recovery and publication tests pass. |
| AC-33 | The dedicated browser CLI uses the shared editor and dispatcher. Completion, load, quoting, format, comparison, navigation and mutation tests pass. Missing saved comparisons and insertion into a new parent have recorded failing and passing CLI regressions. |
| AC-34 | The complete browser run passes all 114 runnable scenarios in 157.2 seconds. Six explicit skips cover two unavailable credential fixtures and four unfinished Workbench workflows. |
| AC-35 | Browser streaming, stop/resume, trusted identity, authorization and accounting tests pass. |
| AC-36 | The connection page reports unavailable state when the backend cannot provide connection rows. It does not represent that absence as a successful empty result. |
| AC-37 | Graphical copy/rename, activation, validation, confirmed acceptance/abort, stable history restoration and discard/fresh-view tests pass. |
| AC-38 | Canonical paths preserve legal key tokens. Slash, space, quoted and dot-key tests pass; malformed paths and separate parent-traversal tokens remain refused. |

The complete config package passes. The final complete web package passes in
116.974 seconds, including behavioral tests, typed views, goldens and the strict
historical comparison. Three browser journeys affected by the last shared
editor and typed-link fixes pass. New refusal tests prove browser recovery
instructions and preservation of the producer's correction. Stream failures
retain a complete recovery line within the display budget and remain valid
UTF-8. Generated templates and asset registrations agree with their sources.
Test expectation changes retain independent per-unit judgments through the
native commit approval route.

The full documentation gate reports broader registry and publication drift.
Earlier broad CLI, hub and terminal-demo runs also contain Linux-only fixture,
backend, privilege and PTY failures; those packages are not claimed wholly
green. The known scoped lint findings were corrected and their regressions
pass; final lint confirmation was blocked by concurrent dependency edits
during planning. Scoped local evidence is not deployment or release acceptance.
The existing verification handoff remains in place.

### Architecture Docs
- [ ] `ai/doors/system.md` and the web, CLI and config doors in `ai/doors/README.md`
- [ ] `docs/architecture/cli/color-system.md` (AC-2)
- [ ] `ai/rules/user-facing-errors.md`

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc7950.txt` Sections 7.13.1, 7.20.3.2 (AC-15)

## Current Behavior (MANDATORY)

### Authorized feature-parity continuation (2026-11-16)

Owner instruction, verbatim:

> ok, fix it and work for parity of features

This continuation fixes the browser-load corpus finding and closes differences
between the existing interactive CLI and the primary web interface. CLI mode
must offer the same editor and operational capabilities through their existing
producers. The graphical YANG editor must expose useful graphical equivalents
for configuration operations and inspection. Experimental Workbench remains an
alternative; its unfinished page designs do not replace the primary editor.
The original unrelated journal obligations remain separately open.

The corpus finding is a discovery defect: `runSweep` treats the intentionally
partial browser upload as a standalone configuration because it does not derive
the `.wb` load composition. The file must remain partial. Adding mandatory
baseline fields would remove the condition that the browser test proves.

#### Existing mechanisms and decisions

| Mechanism | Current producer | Continuation decision |
|-----------|------------------|-----------------------|
| Browser-case parser and daemon fixture | `web/testing.ParseWBFile`; `internal/test/cli` `daemonWebConfig` | Derive ordered browser load composition from parsed actions; share the actual fixture-config authority rather than copying its values |
| CLI command grammar and candidate editing | `cli.ExecuteConfigCommand`, `Model.applyLoad`, `Editor.LoadMerge` / `LoadReplace` | Reuse for corpus replay, browser CLI and graphical operations; no second merge interpreter |
| Command availability | `command.AnswerSurface`; `hub.TestEveryCommandOnEverySurface` | Extend the existing registry-derived parity proof; do not publish a handwritten duplicate command inventory |
| Durable command recall | `cli.History`; existing registered per-user/per-mode history keys | Extend the shared history owner and its lifetime; browser local storage is rejected because it duplicates persistence and retains raw commands |
| Sensitive command display | `config.DisplayCommand` and transcript masking | Reuse schema-aware masking for new history publication and returned legacy entries; do not persist uploaded content. A contextual configuration `set` recalls the equivalent absolute `top set <context> ...` so its target and safe nonsecret values survive reopening without changing the line-delimited storage format. |
| Graphical actions and candidate views | `web.HandleGraphicalAction`, `EditorManager.ConfigCommand` | Add forms and inspection controls over the same producers, preserving candidate isolation, authorization and commit semantics |

#### Acceptance criteria

| AC | Positive observable behavior | Permitted survivors and reason |
|----|------------------------------|--------------------------------|
| AC-39 | The corpus sweep discovers browser file/paste loads from parsed `.wb` actions and applies them over the actual daemon base, in action order, through the shared editor. Accepted loads count as staged drafts; an executed commit validates its exact candidate through the full publication validator. The existing partial upload passes staging and invalid typed loads remain refused. | An unsupported composition is an explicit finding and remains validated as a standalone input where applicable; no filename exception or schema weakening is permitted. An engine-invalid intermediate draft is never counted as a validated published configuration. |
| AC-40 | Browser command recall persists across reload and editor eviction, is bounded and deduplicated by the shared history producer, and remains isolated by authenticated user and effective CLI mode, including automatic entry into configuration mode. Up/Down restores unfinished input. Accepted writes drain before the borrowed store is released. | Abrupt process death retains the shared history producer's documented durability limit. Missing history is distinct from a failed read. Credentials and uploaded content are never returned or newly persisted in recall. |
| AC-41 | Every registered operational command reaches the same dispatcher from browser and SSH, including operational verbs also used by the config editor. Web completion uses live argument producers and shared long explanations. Clear, exit and mode transitions provide their browser equivalents. Daemon restart registers as `request restart`, uses real command authorization/accounting, and releases the existing GR/StopForRestart action only after its accepted response. | Browser navigation replaces closing an SSH connection; daemon shutdown/restart remains an explicitly authorized operational action, never an alias for clearing the display. Daemon restart never requests a host reboot; the service supervisor owns starting the process again. |
| AC-42 | The graphical editor can delete the selected node, discard selected-path changes, inspect candidate/confirmed/saved sources and metadata, compare supported targets, inspect/manage editing sessions, adjust inspection preferences and import a file or pasted content into the candidate. A stable-revision comparison retains every deleted flat statement when another setting changes; shared line comparison never consumes a deletion using indentation alone. | The shared editor may refuse an operation with its existing reason. Candidate import does not use the legacy directly publishing upload path. Read-only users retain inspection without mutation controls. Flat statements may show separate removal/addition markers because indentation does not establish node identity. |
| AC-43 | Browser acceptance runs with distinct synthetic administrator and read-only identities against authenticated daemon fixtures. Existing permission and raw-download denial cases execute, with positive allowed-read controls and unchanged enforcement. | A backend capability unavailable on the host remains explicitly unavailable, not a passed authorization test or an empty observed result. |
| AC-44 | Interactive monitoring capabilities available through CLI views have useful browser equivalents for sorting, selecting a peer, inspecting detail, returning to the live view and stopping updates. | Browser controls may differ from terminal keystrokes; the same data producers, identity and authorization remain authoritative. No protocol state is imported into the web adapter. |
| AC-45 | A source-derived capability matrix accounts for editor commands, operational commands, completion, explanations, history, streams, monitoring, graphical equivalents and authorization. Executable parity probes cover the concrete gaps alongside registry-derived offered/dispatch coverage, including shared CLI preservation of removed flat statements in revision comparisons. | Unsupported behavior on both CLI and web is reported as such and is not invented as a web-only feature. Unfinished Workbench layouts remain experimental when the equivalent capability is available in the primary interfaces. |
| AC-46 | Static dispatch validation reads all six synthetic L2TP command declarations from the same literal declaration/handler table used by offline registration, Stage 1 and runtime dispatch. Existing pool and emit commands retain their names, handlers and sorted help output. | This fixture maintenance does not claim completed L2TP web controls or a passing full dispatch check before other findings are resolved. |
| AC-47 | A successful graphical Review & Commit displays any warning and its recovery advice while retaining successful publication, audit and notification exactly once. HTMX and ordinary requests preserve clean successful behavior. | An applied commit with a warning is not reported as a refusal or retried. |
| AC-48 | A complete finite browser CLI download preserves the final selected stdout bytes, including binary Blob answers, without a display-string round trip. Displayed warnings remain outside the artifact and its byte budget. Real browser proof compares the downloaded bytes. | Denied, incomplete and oversized results never offer a partial successful export; explicit JSON formatting retains its declared Blob envelope. A successful empty output remains a valid empty artifact even with a displayed advisory. |
| AC-49 | Browser operational `configure` follows the shared CLI control grammar: only the exact control enters configuration mode; malformed argument and pipe tails are handled without being silently ignored. | Valid registered commands continue through the normal authenticated dispatcher. |
| AC-50 | Reopening an evicted or restarted user's web editor exposes its preserved resumable web candidate intent with honest review, pending counts and commit behavior. Another owner's changes remain isolated, and failed recovery refuses explicitly without overwriting preserved work. | Recovery reuses the shared session and replay machinery; a global saved draft is not blindly adopted as the user's candidate. |
| AC-51 | Shared CLI and web editor help advertises supported editing forms and retains ordinary subsection editing. | Wildcard template editing remains a shared unsupported operation; removing its inaccurate help form does not implement or change its refusal. |
| AC-52 | Discard audit inspection reads saved and unsaved intent, including saved addresses affected by later structural edits, without changing durable bytes, creating an editing session or caching an unresumed editor. Read, parse, required listing and final guard-release failures stop discard before mutation or audit recording. | Other owners' intent remains excluded from the masked detail and unchanged in storage; an empty successful snapshot is permitted only when no intent matches the selected owner. |
| AC-53 | Explicit discard removes saved changes to existing leaves as well as unsaved changes; reopening does not recover discarded ownership, and committed configuration remains byte-identical. | Pending intent outside the selected discard ownership remains intact. The CLI guide's session-only boundary and the current web account-wide boundary need reconciliation before changing cross-origin behavior. |

#### Implementation phases and wiring proof

| Phase | Work | Entry point and owed proof |
|-------|------|---------------------------|
| P1 | Corpus discovery and shared browser daemon-config authority | `TestSweepBrowserUploadUsesDaemonComposition`, invalid/replace/order controls, `TestRepositoryConfigsValidate`, existing browser partial-file load journey |
| P2 | Shared masked, bounded history ownership and browser recall | Authenticated CLI page/history projection plus command/terminal/stream submissions; storage-failure, mode/user isolation, reload, eviction and close/drain tests |
| P3 | Operational routing, live argument completion and explanations | Operational `delete agent skill` dispatch/auth tests; live plugin-name completion; completed-input explanation; clear/exit browser journey; shared daemon-restart registration, authorization/accounting, response completion and GR owner wiring |
| P4 | Graphical configuration operations and inspection/import | Selected-node delete/path discard, source/compare/session/preferences forms and candidate import; graphical-to-CLI readbacks and unchanged committed state before acceptance |
| P5 | Authenticated permission fixtures | Actual admin/read-only daemon setup; retained authorization/download browser cases and allowed-read controls |
| P6 | Interactive monitoring and complete capability evidence | Existing monitor/view producers plus browser sorting/detail/back/stop journeys; registry-derived inventory and executable gap probes |
| P6c | Caller-supplied configuration inspection through owned daemon commands | Registered supplied dump, validation, formatting, dependency graph and comparison commands; local-producer equivalence, bounded input, secret masking and shared dispatcher authorization/accounting probes |
| P6d | Declared-schema integration for configuration inspection | File, stored, history and supplied diff, dump and graph use the shared declared-schema step; published external modules remain usable, unavailable sections explain the daemon recovery, ordinary parse/range refusals retain their existing meaning, and no inspection starts a plugin |
| P6e | Post-release consideration: conformance of startup-configured plugins through the daemon | Removed from release acceptance by the owner scope decision above. If resumed, select the executable from immutable startup declarations and prove execution authorization, target selection, cancellation and daemon shutdown before claiming the capability. |
| P6f | Correctness of the existing graphical editor and browser CLI, before missing parity implementation | Reproduce and correct AC-47 through AC-53, then run the affected authenticated browser and backend acceptance paths; keep source coverage, runtime proof and absent features distinct |

P1 and the shared-history foundation P2a have independent source concerns.
P3 backend routing/completion can proceed alongside P2a, with the history owner
writing its model input sites first. Browser history wiring P2b follows P3's
shared terminal/frontend source handoff. P4 follows P3 where they share action
adapters; P5 follows P1's shared daemon fixture authority. Main runs all scoped
tests and native jobs. Each implementation phase updates its owning pages.

P6c closes the caller-input gap identified by AC-45's capability census. The
existing file-taking commands retain their path confinement, stdin behavior and
revision grammar. The config CLI owner registers the separate command namespace
`show config supplied`, with dump, validation, fmt, graph and diff commands
declared in its YANG module. The input is the existing command argument channel,
never a daemon file or an editor candidate. The registered handlers use the same
byte-based producers as the file readers.

| Input field or declaration | Contract |
|----------------------------|----------|
| `content` | Padded base64 bytes, bounded by the declared 261,120 encoded characters and 195,840 decoded bytes that fit one SSH command packet. One shared bounded codec serves the existing store writers and the supplied readers. |
| Single-source content | Native configuration bytes. An optional bounded `name` is a diagnostic label, never a filesystem path. |
| Comparison content | One encoded JSON object containing the native configuration strings in `before` and `after`. Both are required; duplicate fields, unknown fields, malformed JSON and trailing data are refused. The combined envelope has the same packet bound as a single source. |
| Command selections | Dump retains strip-private; fmt retains check and diff. The supplied forms have no write selection. |
| Supplied-input descriptor | Generic optional command metadata declares input count and envelope for actual supplied commands. The descriptor reads bounds and label constraints from the YANG arguments, so the browser can derive controls without a config-domain import or a second command inventory. |

The validation client contract is `show config supplied validation content
<base64> [name <label>]`. It shares the file command's validation producer and
`diagnostic.ValidateResult`: invalid configuration retains its diagnostics
payload beside the failed verdict. Malformed command arguments can be refused
before validation and carry no such payload. The daemon validates against the
module set published at startup, including external-plugin YANG. The local
route starts no plugin. This command, its arguments and its result shape remain
unchanged by P6d. Commit `b93ff713b3` contains the schema behavior from
`c7f081849d`. The validation client should use `552c08a28a`, which includes
both and fixes SSH exec retaining a validation payload beside its refusal.
That commit adds the daemon-first client and a result decoder without changing
the endpoint's arguments or result fields.

The one comparison envelope puts both inputs behind the already masked
`content` keyword. Authorization, accounting, caller context, cancellation and
pipes remain with the shared dispatcher. Reads do not modify candidate,
committed configuration, daemon environment, storage or files. New behavioral
tests first use the existing local-data lookup, model validator and remote
answer APIs; a missing registration or rejected model is the expected initial
failure. Browser forms follow the registered descriptor in a separate frontend
phase, with authenticated file/paste and denied-call acceptance still owed.

The shared local/supplied comparison resolves BGP values and compares
schema-declared ordered-list ordering. It does not compare other non-BGP values
such as `system host`. AC-45 records that shared boundary; the browser does not
invent a wider comparison or claim to repair the local command's scope.
Producer: `config/cli.parseAndResolve`, `diffSources` and `diffListOrders` in
`internal/component/config/cli/cmd_diff.go`.

#### Risks and assumptions

| ID | Kind | Basis or failure mode | Validation / mitigation | Status |
|----|------|-----------------------|-------------------------|--------|
| A-P1 | Assumption | The browser fixture is a partial absolute merge over the runner's complete daemon base. | Read the parsed case, `daemonWebConfig`, `Model.applyLoad` and `Editor.LoadMerge`; retain an actual failing corpus regression. | Confirmed; RED produced zero sequences and the two missing mandatory fields. |
| A-P2 | Assumption | Shared command registration establishes offering and dispatch availability, but not equivalent execution for every command. | Read `collectCommandRegistrations` and `dispatcherServes`; add targeted execution probes for concrete transport differences. | Confirmed by source audit. |
| R-P1 | Risk | History persistence leaks secrets or retains uploaded bodies. | Schema-aware command masking before append and legacy projection; relative/top/run secret controls; never record content payloads. | Open until P2 proof. |
| R-P2 | Risk | Per-user history workers outlive editor eviction or borrowed-store shutdown. | Explicit owner/admission close, detached bounded snapshots, blocked-I/O/drain tests and startup-failure cleanup. | Open until P2 proof. |
| R-P3 | Risk | Config convenience verbs swallow registered operational commands. | Mode/registry authority and direct operational dispatch/auth tests with configuration-delete preservation controls. | Confirmed defect; P3 owns it. |
| R-P4 | Risk | A nominal read-only browser fixture actually seeds its user as the administrator. | Distinct synthetic identities, real daemon authorization declarations and positive/negative browser controls. | Confirmed fixture defect; P5 owns it. |

#### Continuation implementation and remaining acceptance (2026-11-16)

The core continuation is committed in `b93ff713b34e0432525f26a8945c79905337e58c`.
The browser replay correction is committed in `df3b4101b3`, and the graphical
Review request boundary correction is committed in
`f1d64ff442a1125557c45517c4dfc3a11469bc75`.
This is implementation progress, not completed feature parity. The original
AC-1 through AC-30 remain open; the earlier amendment history is retained.

| AC | Committed implementation and scoped evidence | Remaining acceptance |
|----|---------------------------------------------|----------------------|
| AC-39 | Parsed browser file/paste composition uses the shared fixture and editor. After correcting error-dismissal replay, four focused units pass in `abefb5fb`, including three presentation branches, four unknown-control cases and seven refusal-declaration guards. | Full Ze Linux corpus acceptance after the remaining fixture corrections. No full corpus rerun or passing verdict is claimed. |
| AC-40 | Shared masked recall, per-user/per-mode owners, receipt bounds and strict cleanup are committed. Focused hub and web history/lifetime tests pass in `9a07406e`. Editor recall cases pass within the 10/10 reached editor run `8217a4d4`; standalone shutdown controls pass within the 4/4 UI run `0eeec7fc`. | Broader browser/package acceptance; no universal durability claim. |
| AC-41 | Shared operational routing, completion, explanations, controls and daemon restart are committed. Focused hub tests pass in `9a07406e` (66.122 seconds); shared CLI regressions pass in `562f980f`. Reached lifecycle, explanation and pipe functional cases pass in `8217a4d4` and `0eeec7fc`. The normal wire-method comparison passed in `f326753a`. Static dispatch validation passes over 585 registrations and 99 emitters. | Broader browser/package acceptance and current internal RPC contract reconciliation. |
| AC-42 | Selected-node actions, inspection/session/preferences forms and candidate import are committed. Earlier focused journeys pass. Review now sends its independent GET outside the mutation form; both affected browser journeys pass in `20fd5c71`. | Current broader browser acceptance and remaining consuming checks. |
| AC-43 | Distinct authenticated administrator/reader fixtures and permission/download cases are committed. Scoped import/auth recovery and allowed supplied-inspection controls pass. | Current broader authenticated browser acceptance. |
| AC-44 | Registered browser monitor views and sort/detail/back/stop controls are committed. All selected browser-view and dashboard regressions pass in `562f980f`. Removing only the real dashboard browser registration makes the capability guard fail with `[dashboard]`; restoring the exact source makes that named test pass in `210ef863`. The monitor-control test's typed nil assertion is corrected under independent judgment and passes alone in `adc4c398`. | Broader browser/package acceptance. |
| AC-45 | Registry-derived capability probes, supplied-inspection commands and file/paste forms are committed. All eleven real browser download tests pass in `8d598c23` (61.874 seconds); their truthful host route and passing integration coverage census are committed in `c1c8e306c0`. Both normal golden tests pass in `5bdc40ef`. HTMX checks pass over 787 files with 16 existing explained cases in `a81f3948`, without new exceptions. Fresh assets, generated-template and usage-contract checks pass. | Scoped lint, remaining command/YANG findings and broader executable acceptance. This does not establish execution parity for every registered command. |
| AC-46 | The literal declaration/handler producer correction is committed in `305fecf62d`. Static dispatch validation passes. All four unchanged route-emission fixtures pass in native run `94609`, including announce, withdraw and import controls. The unchanged pool reload fixture also passes in `e43c96e4`, with all seven verification steps, after its owner corrects optional absent-BGP startup. | These scoped runs do not establish every later BGP source-policy or release-level integration change. No fixture expectation was weakened. |

Every failure from the earlier 126/133 browser run now has a focused passing
rerun. `./le --name web-corrections test web` passed
`test/web/graphical-configuration-actions.wb` and
`test/web/graphical-field-enter-blur.wb` together in `e4899ad7`
(2/2, 25.3 seconds). `./le --name web-corrections test web --pattern scenario-firewall-setup -v` passed in `49d61119`
(1/1, 12.5 seconds), including all 74 steps. Together with `96015`, these clear
the seven earlier scenario failures; a current broader browser run remains
owed. Four explicitly unfinished experimental Workbench workflows remain skips.

After the Review correction, the component and handler golden comparisons
both pass in `5bdc40ef` (10.082 seconds), with nine independently approved
captures. The affected graphical community and configuration-actions browser
journeys pass 2/2 in `20fd5c71` (15.2 seconds).

The reached editor run `8217a4d4` passes all 10 selected cases, covering recall,
lifecycle confirmation, explanations, comparison and ordered commit/rollback.
The UI run `0eeec7fc` passes four selected cases in 4.9 seconds:
`web-only-interrupt-graceful`, `web-only-interrupt-force`,
`pipe-interactive-save` and `monitor-ping-pipe-resolve-log`. These are the
runner's stable basenames.

Shared-view discrimination produced the expected `[dashboard]` failure in
`76c622c7`. The exact original source was restored before the named
`TestBrowserViewRegistryCapability` passed in `210ef863`. No deliberate break
or test change remains from that proof.

Product correction `74da4cb516` checks a registered view's returned concrete
model before continuing. A new synthetic registered view first reproduced the
assertion panic, then proved explicit failure, cancellation, producer joining,
an unchanged caller context and preservation of the actual `Model.Close`
history-publication error. The existing browser-view regressions also pass.

Verification maintenance `836fae83a1` preserves the existing test inputs and
assertions while making response ownership and concrete result checks
explicit. Its independently judged monitor-control assertion now checks the
producer's nil refusal pointer directly; the old assertion boxed it into a
non-nil error interface. The same malformed and anonymous controls, successful
status and exact queued action remain. Host-route maintenance `c1c8e306c0`
records where the eleven real browser download tests execute; it does not
claim execution inside QEMU. Both changes retain their actual passing runs.

Acceptance still requires broader browser/package coverage, scoped lint and
the unresolved command/YANG findings. Fresh assets, generated-template and
usage-contract checks pass. Static dispatch discovery now passes after the
fixture declaration-producer correction. The grammar check still reports the
two development-tool namespace collisions `arch error-inventory` and
`arch error-wrap`; their registrations predate this amendment. Command-contract
validation reports three internal transport RPCs without CLI declarations
(`source-lease`, `cancel-request`, `select-routes`), which their owner is
reconciling without exposing them as invented CLI commands. Help-shape checks
found two missing full stops in those RPC summaries; this amendment's daemon
restart summary is corrected.

The 21-package scoped lint run `107d5015` completed with findings, including
formatting, unchecked assertions and response-body ownership checks. Owned
corrections retain their scoped regression evidence and independent test
judgments. The ten-package follow-up stopped during planning because a
concurrent peer-declaration test changed; it produced no analysis plan and is
not a lint pass or a new diagnostic result. Two stable-source split runs then
completed all feature rows with findings: `dabc8b99` covered web, CLI, its
contract, error inventory and systemd; `cb0913b4` covered hub, config CLI,
plugin server, faker and QEMU. Web, error inventory, faker and QEMU reported
no diagnostics in these runs. The remaining owned corrections preserve the
existing test assertions and inputs and have independent final source judgments.
Their focused CLI history, option, browser-view and graphical per-user
preference tests pass in `1638aec6`. Config CLI tests pass in `7d4351cf`, as
do graphical import, bootstrap credential/certificate and SSH listener controls.
That run's complete local/daemon command comparison failed on five supplied-input
help hints and the local-only plugin conformance command. The help fix is now
committed as `2ec7abaa6f`; all five original comparison cases pass by name in
`633cf5b4`. Arbitrary caller-supplied executable conformance still has no daemon
form, and its remote execution guard remains enforced. Systemd tests
pass on the host in `d5cb506e` and in Ze's Linux guest in `bcc1fcd6`, including
the original policy preservation and refusal cases. Plugin-server lint
findings are with the BGP owner. These
results do not establish a passing lint verdict for the complete scope.
The final five-package follow-up completed all rows and reported three
response-ownership findings in `web_monitor_pipe_test.go`. The independently
judged corrections retain the original stream reads, controls and
cancellation/join assertions. All four affected monitor units pass in
`d642f003`. Reached lint no longer reports response ownership findings; other
findings remain. The monitor cleanup is committed as `4bbafc7f45`.

The error-inventory classifier now recognizes operator errors extracted with
`errors.AsType`. Its two positive regression cases failed before the correction,
while seven negative controls passed; the complete classifier package and
scoped lint then passed. This is committed as `ae90d0a680`. The source cleanup
is committed as `a70b372de9`, with nine eligible independent test judgments.
The daemon missing-argument fix shares the existing local refusal renderer and
preserves the typed cause. Its new tests first failed to build before that
producer existed; the earlier command comparison provides the executed
behavioral failure. The exact-registration boundary and original selector
controls now pass in `e2811165`. A configured-plugin conformance operation is
under implementation, with daemon-owned executable selection. Eleven new hub
cases executed and failed because that command is not registered yet. A real
started-query test also failed because caller cancellation did not reach the
query's independent budget. The shared context fix now passes that regression,
preserving the offline no-spawn declaration rule from `d4fb5888c7`. Full plugin
and conformance package runs passed before the final pointer correction; the
current affected consumer checks pass in `7527ae72` (plugin 4.715 seconds,
conformance 2.007 seconds). Commit `2210546edb` fixes external declarations,
their executable and their schema until daemon restart; reload refuses changes.
The earlier reload-generation draft is superseded. The owner subsequently
removed P6e from release acceptance; it can be considered after release under
the scope decision above. Any resumed configured-target implementation must use
that startup authority. Neither that implementation nor full conformance
execution parity is claimed complete.

The broader source inventory also leaves these executable gaps open: a browser
workspace for a caller's offline backup artifact and daemon forms of standalone
root/flag workflows outside the registered-command census. Explicit remote
recovery of an unreadable pending change file is implemented under the amendment
above. Ordinary discard still refuses an unreadable audit snapshot; the separate
recovery action requires its own warning and confirmation. Registered-path
coverage alone does not establish the remaining parity work.

P6d is implemented locally. Its initial baseline reproduced twelve missing
offline explanations. The corrected published-schema fixture then passed all
eight positive controls. Current consumer checks in `7527ae72` pass all 32
schema cases, existing diff/secret/history and supplied-input checks, invalid
validation diagnostics and the new daemon-first client routes (22.598 seconds).
The independently judged recovery assertions follow the committed content
validation client in `552c08a28a`. Final scoped lint and documentation checks
remain separate from these execution results.
The complete
web package is not claimed to pass: the separately owned
`TestL2TPDisconnectRefusesAnUnguardedForm` currently receives HTTP 200 where its
guard regression requires 400. Its producer must be fixed before that broader
package acceptance.

The config-CLI and CLI Linux test binaries compiled in `1566da2d` and
`71976fb0`. The corrected Ze Linux guest then passed all four
`TestSessionEmptyEntryCommit` branches: direct/commit, merge/commit,
direct/candidate and merge/candidate. Native kernel/image admission, the real
Git runtime, canonical checkout and guest-only `/usr/bin` capacity correction
all succeeded before the tests.

Full `TestRepositoryConfigsValidate` then failed in 75.80 seconds. Its 30
findings comprise one browser replay refusal at `error-toggle` in
`test/web/graphical-import-paste.wb`, 28 BGP source/destination grant fixture
preconditions and one undeclared PPPoE `access0` interface. The browser replay
correction now passes its four focused units in `abefb5fb` (3.739 seconds).
The BGP grant fixture correction remains pending. The PPP owner has since
reported its promoted interface fixture passing; this session's full corpus
has not been rerun. Discovery counted 3,376 Ze inputs, 242 migrated inputs,
55 declared refusals, 672 exit refusals, 32 browser load sequences, 74 browser
steps and three staged draft loads; 80 legacy inputs were not validated and
there was no host skip. These are population/disposition counts, not 3,376
successful validation results. The first guest attempt stopped during setup
before any test ran; that setup refusal is not a product-test failure. The
spec remains `in-progress`.

#### Integration and documentation obligations

| Surface | Disposition |
|---------|-------------|
| YANG/config/env | Reuse existing schema, authorization and registered history keys; no new configuration option is planned. |
| Command grammar/registry | Reuse shared command and load parsers, surface predicate and argument completers; no adapter-owned operational command list. |
| Web routes and assets | Reuse authenticated CLI/action routes and external `cli.js`; any new response field has an explicit Go producer and JavaScript consumer census, with typed templ forms. |
| Runtime lifetime | P2 names every manager/history constructor, eviction/discard path, shutdown and startup-failure owner before adding a worker. |
| Doctor/errors | Reuse existing store/daemon diagnostics and actionable operator errors; a new failure class must name its owning diagnostic and recovery rather than silently return empty history. |
| Metrics | No protocol counter is added by these transport corrections; new bounded-history/read failures must remain observable through their owning error path. |
| Functional/goal validation | Browser journeys prove actual partial input, persistent recall, operational dispatch/completion, graphical candidate operations, permissions and monitoring controls; registry listing alone does not prove feature parity. |
| Pages | `docs/architecture/web-interface.md`, `web-components.md`, `docs/guide/config-editor.md`, `docs/architecture/cli/command-completion.md`, `docs/architecture/api/commands.md`, `docs/architecture/zefs-format.md`, `docs/architecture/testing/runner-architecture.md` and relevant `docs/contributing/testing.md` sections are updated in the phases that change their behavior. |

The strongest design concern is preserving lifetime and secrecy while making
browser recall durable. The shared history owner is the preferred mechanism;
duplicating it in browser storage or a new persistence service would create a
second lifecycle and an inconsistent source of history. The owner has authorized
this continuation; routine design choices follow that scope and the shared
mechanisms above. Independent release review remains separate from implementation.

Spot-checked on 2026-10-23: `serializeTreeAtPath` in
`internal/component/web/cli_terminal.go` still returns an empty string with no
error for a bare list name, so the 2026-10-10 `show <list>` row is present.

- [ ] `internal/component/web/cli_terminal.go` - `serializeTreeAtPath`, `terminalAuditSucceeded`, `terminalFeedback`
- [ ] `internal/component/web/cli.go` - `splitSetArgs`
- [ ] `internal/component/web/editor.go` - `Commit`, `Rollback`, `ChangeCount`
- [ ] `internal/component/config/yang_schema.go` - leaf-list builders
- [ ] `internal/component/config/yang/command.go` - `BuildCommandTree`
- [ ] `internal/component/config/yang/register.go` - `Module.Registrar`
- [ ] `internal/component/config/serialize_set.go` - `emitValueOrArrayNop`
- [ ] `internal/component/config/cli/cmd_set.go` - `cmdSetImpl`
- [ ] `internal/component/command/pipe.go` - `foldFilters`
- [ ] `internal/component/command/pipe_columns.go` - `selectRecord`
- [ ] `internal/component/plugin/server/command.go` - `tokenize`
- [ ] `internal/component/cli/model.go` - `outputBuf`
- [ ] `internal/component/cli/client/main.go` - local-serve transcript
- [ ] `cmd/ze/ze_core_usage.go` - `zeUsage`
- [ ] `internal/le/site/commands.go` - `markdownCell`
- [ ] `vendor/github.com/openconfig/goyang/pkg/yang/entry.go` - `ToEntry`, `ApplyDeviate`

### Standalone command equivalence amendment (2026-11-17)

Owner request after the standalone CLI audit, verbatim:

> Okay, resolve the problem you identified.

This amendment completes the standalone root and flag census under AC-45. It
retains the parent's other acceptance criteria and its verification handoff.
Each operation has one producer behind local and daemon entry points; web forms
derive from the registered operation and its modeled inputs.

| Phase | Work and positive acceptance |
|-------|------------------------------|
| S1 | Store registration lookup, repair and encode share typed results. An unknown registered-key pattern fails on both routes; unresolved repair history errors cannot report success; successful repairs retain recovered keys and finding severity. Prefix and pattern selections are declared in YANG. Empty, whitespace-only and binary encoder input remains valid on the web, MCP, REST and gRPC as it is locally; explicit empty values are distinct from absent arguments. Environment-only validation uses the existing registered environment validation authority rather than unconditional success. Doctor loader refusals retain their explanation and severity, provide the shared recovery command and withhold the runtime tree. |
| S2 | Current-config conversion, fix-plan generation, masked source-subtree inspection and ExaBGP config/environment conversion are registered supplied-input operations. Their file/stdin entry points use the same byte producers. Both valid inputs and refused inputs retain their useful output and diagnostics. List and history root adapters consume typed producers instead of repeating traversal. |
| S3 | Detached-file set/activate/deactivate, pending validation, rollback and stored-config rename have daemon forms for their actual source and publication target. File/paste controls return detached results without editing the live candidate. Local writes remain local adapters; formatting shares its producer and returns bytes for download remotely. |
| S4 | Backup artifact list/show/diff/set and explicit draft-session editing are reachable through owner-held daemon operations and browser adapters. Artifact identity, ownership, bounds, history and cleanup are shared. Committed content remains the default browser source; draft resumption is explicit. Full-store restore uses a verified shutdown/recovery lifecycle, never replacement beneath an active owner. SSH network grace expiry permits forced closure and a completed history drain; independent listener, transport, close and model-drain failures still refuse a clean restart. Retained private recovery resumes through the same importer with a durable source/target ownership proof and exact target `--path`; successful imported-owner close precedes private cleanup. Generic `RestoreBlob` keeps caller artifacts. Unknown or partially cleaned recovery-looking directories remain preserved with explicit owner handling that resolves admission debt. |
| S5 | Independent review checks the complete standalone census against local, dispatcher and browser entry points, including failure verdicts and artifact preservation. Focused main-thread tests prove the reached behavior; registered-path enumeration alone is insufficient. |

| Existing mechanism | Extension and preservation |
|--------------------|----------------------------|
| `command.MustRegisterLocalData`, `plugin/server` `loadLocalData` | Serve the same typed handler locally and through the daemon; retain route authorization, cancellation, secret masking and answer shapes. |
| `registry.Meta.SuppliedInput`, `web.HandleSuppliedInspection` | Discover new file/paste operations from owner metadata and real YANG leaves; no web-owned operation inventory. |
| `config/cli` byte producers and `cli.Editor` | Share parsing, validation, serialization and tree edits; preserve detached-file versus candidate publication. |
| `storage.Backup`, `RestoreConfig`, `PublishArtifactConfig`, `RepairHistory` | Keep storage locking and pointer commit boundaries; preserve unrelated artifact keys, pending intent and saved drafts. |
| `env.InvalidValues` | Use the registered environment authority for environment-only validation; retain explicit invalid-value and range findings. |
| Daemon restart and service shutdown owners | Establish a recovery handoff only after active store users have drained; keep the prior store recoverable on failure. |

The main thread owns test execution and commits. Each phase's implementation
agent writes its failing regression or wiring tests first and supplies exact
commands. New commands declare bounded inputs, useful successful answers and
actionable refusals. No RFC behavior changes are planned. The command reference,
API command architecture, web architecture and operation-specific guides change
with their producers. Temporary evidence and phase handoffs are owned by this
session's state record; obsolete logs are removed after results are recorded.

#### Standalone amendment acceptance evidence (2026-11-18)

Local commands, daemon operations and browser forms now call shared business
producers. File reading, upload, download, authentication and explicit
publication remain adapters at their owning boundaries. The evidence below is
scoped to S1-S5; the local pipe walk does not prove daemon transport or
authorization by itself. Commands identify reproducible native entry points.

| Phase | Observed evidence | Canonical tests and entry points |
|-------|-------------------|---------------------------------|
| S1 | Registered-key refusals, repair failure/warning grading and complete, empty, whitespace and binary encoding passed. Environment validation rejects an invalid registered value and accepts its correction. | `TestRegisteredPatternRefusalMatchesStandalone`, `TestRepairRPCUnresolvedHistoryRefusesSuccess`, `TestDataEncodeRegisteredMatchesStandalone`, `TestConfigEnvironmentValidationRejectsMalformedValue`. Storage entry point: `./le --name session test unit pkg path ./internal/component/config/storage/cli/ run '^Test(RegisteredPattern|Repair|DataEncodeRegistered)'`. |
| S2 | Supplied conversion, repair plans, masked source inspection and ExaBGP config/environment conversion passed against their local producers, including invalid input and cancellation. Unreadable inventories refuse without partial output. | `TestSuppliedConfigMigrationForms`, `TestSuppliedConfigFixPlanKeepsDiagnostics`, `TestSuppliedConfigSourceSubtreeRetainsMasking`, `TestSuppliedExaBGPMigrationDaemonDispatch`. Migration entry point: `./le --name session test unit pkg path ./internal/plugins/exabgp/ run '^TestSuppliedExaBGPMigration'`. |
| S3 | Detached mutation/replacement, rename and indexed rollback passed with source preservation, native refusal context and stopped-window recovery. The final nine activation/deactivation controls and actual rollback scenario passed. | `TestSuppliedConfigDetachedMutationsMatchFileCommands`, `TestSuppliedConfigReplacementRemainsDetached`, `TestConfigRenameRootMovesPointerOnlyHistory`, `TestConfigRollbackRootPreservesStoppedWindowRemedy`, `TestDetachedActivationRefusalKeepsProducerContext`. Binary entry point: `./le --name session test bgp parse cli-config-rollback`. |
| S4 | Real browser backup editing and raw-source check/salvage/download journeys passed, including access, cancellation and incomplete-transfer refusals. Each corrected restore/restart draft passed 80 load iterations before unchanged promotion; all three promoted live scenarios passed. Five final hub shutdown/history controls also passed. | `TestBrowserBackupWorkspace`, `TestBrowserBackupSourceRepair`, `TestBackupHTTPDownloadDetectsIncompleteTransfer`, `TestWebOnlyShutdownJoinsHistoryBeforeCommandOwner`. Lifecycle entry point: `./le --name session test bgp plugin data-restore-full data-restore-full-refused-live drain-restart-kind`. |
| S5 | Independent review and exact population guards cover the standalone roots and flags. The real local CLI pipe walk preserves 47 original invocations and adds 26: 73 invocations over 71 paths. Its final draft passed all 80 load iterations and the unchanged promoted live scenario passed. | `TestStandaloneParityEvidenceIncludesEveryNewRegistration`, `TestStandaloneReturnedConfigurationRequiresRequestedValueAndState`, `TestStandaloneSuppliedContentPreservesEmptyAndPrivateBytes`, `TestCommittedScenarioLaunchesOnlyTheCompiledHelper`. Live entry point: `./le --name session test ui --pattern pipe-local-command`. |

Normal comparisons passed for the complete `TestWebGoldenOutput` family, the six
changed `TestWebHandlerGoldenOutput` cases and `TestWebViewDataIsTyped`, after
independent oracle approval. The typed-view guard checked exact equality over
111 components and all field-type refusals in that run. Update-mode captures
are not these comparison results.

A reached daemon shutdown test exposed a false cleanup failure after SSH network
grace expired. Five additive native controls distinguish completed forced closure,
listener-error provenance, foreign deadlines and overdue history cleanup. Their
failed controls passed after the source repairs, as did the unchanged original
daemon shutdown test, all 129 SSH package units and seven hub-owner controls.
The retained SSH patch passed its exact application guard and the upstream
v0.4.3 reverse/forward comparison. A separate native loader-refusal regression
exposed an empty doctor recovery command; it passed unchanged after using the
shared message/Fix helper, together with its valid-config control and three
existing doctor controls. These repairs are committed in `f5961b331d` and
`c8fcc136e7`.

The registration regressions passed after their observed failures. The two
remaining live-contract failures then passed after linking the feature-gated
ExaBGP CLI owner into the checker. Actual HTTP shared-owner and flag controls
passed. `./le --name session doc yang-contract command-contract` passed over
544 modeled commands, 467 RPC handlers and 77 local handlers. Artifact
registration/model checks retain all 46 methods and exact ownership equality.
Parser-constant and lexical-shadow regressions each passed after their observed
failures, followed by eight existing flag-feeder controls. Owner declarations
now provide 29 flag specs on ten paths. The latest
`./le --name session cli grammar check` reports no amendment-owned flag findings;
its remaining three R9 findings concern the unrelated tool roots
`arch error-inventory`, `arch error-wrap` and `test container-seed`.

The implementation and proof chunks are committed in `b5a7d0dfe7`, `384ccd5025`,
`d7061f9e75`, `038af8da35`, `d022fe5ddf`, `c89278177b` and `1d53fb1133`.
The reviewed maintenance changes are committed in `57603016d7`. Explicit
pointer declarations retain the conservative error inventory's boundary proof;
their three local style exceptions name the overlapping checks and preserve
the declarations, initializers and error handling.
The four owned template outputs were regenerated; the native output comparison
reported only the other owner's stale `l2tp_detail_templ.go` in that comparison.
Topic documentation was source-reviewed. The full documentation-wiring
composite was not run: it has no per-page scope and includes a daemon catalog
build.

The parent status, verification handoff and other acceptance criteria remain
unchanged. Broader configuration-corpus findings involving BGP grant fixtures,
reserved CoS prefixes and the later corpus timeout are not passing evidence
here. Configured-plugin daemon conformance remains postponed under the owner's
decision above; its eleven draft failures remain preserved. This section claims
neither whole-tree acceptance nor release readiness.

Final scoped lint: PASS — 15 explicit packages, zero findings in all eight
native passes: host, Linux integration, FreeBSD, OpenBSD, DragonFly, capability,
appliance and compile-out. The preceding missing-cache run is superseded by
this complete successful retry. The current grammar check still reports only
the three unrelated R9 tool roots above, with zero flag-in-YANG, demo-launch or
flag-registration violations.

## Existing Mechanisms

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| Shared web view routing and path extraction | `cmd/ze/hub/service_web.go` `buildWebService`; `internal/component/web/fragment.go` `extractPath` | Routes `/show/` and `/monitor/` into the active UI; emitted `/config/edit/` URLs fall through and are parsed as YANG paths | Extends the shared routing to accept emitted edit URLs while preserving canonical `/show/` and authentication |
| Per-user candidate rendering | `internal/component/web/handler_workbench.go` `HandleWorkbench`; `internal/component/web/editor.go` `EditorManager.Tree` | Renders fields from the authenticated user's existing candidate | Reuses it for edit URL readback; no second editor or state store |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
An operator command in the SSH CLI, the web terminal, the CLI bar or `ze`
subcommands; a web page request; a YANG module load; the website build.

### Transformation Path
1. Tokenize and resolve against the YANG-derived command tree.
2. Apply to the candidate config or dispatch to a handler.
3. Render the answer (text, JSON, HTML) through the pipe chain.

### Boundaries Crossed
Browser to web handler; SSH to CLI model; CLI to daemon dispatch; vendored goyang to Ze's loader.

### Integration Points
The command registry, `DisplayValueAtPath` masking, the audit log, the editor commit hook.

### Architectural Verification
- [ ] Every display path masks `ze:sensitive` through one function

## Risks & Assumptions

### Assumptions
- A change to vendored goyang is kept as a patch against the upstream release (`ai/META-DECISIONS.md`).

### Risks
- `spec-finish-web-cli-ux` and `spec-web-testing-gaps` touch the same pages.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `set ... password X` in the web terminal | → | `terminalFeedback` | `.wb` under `test/web/` asserting the value is masked |
| `show bgp peer` in the web terminal | → | `serializeTreeAtPath` | `.wb` asserting a list rendering or an explicit empty answer |
| `uses g { augment ... }` in a test module | → | goyang `ToEntry` | unit asserting the augmented leaf exists |

## Acceptance Criteria

| AC | Rows | Observable |
|----|------|------------|
| AC-1 | `ze isis decode` stdin | The decode commands take a path or `-`, and refuse with usage at a terminal instead of blocking |
| AC-2 | dark theme BFD page | Generated plugin detail cards keep text contrast in the dark theme (colour from the color system), checked by a render test |
| AC-3 | YANG command descriptions x2 | Every YANG command node named in the rows has a description; `TestEveryCommandNodeHasASummary` green; `ze pipe json compact` emits no WARN |
| AC-4 | `selectRecord` uptime | A `display` column resolves to one record unambiguously when two records share a column name |
| AC-5 | looking glass golden mock | `mockDispatch` answers with the producer's types (numbers as numbers); golden updated from the real producer |
| AC-6 | set-format inactive leaf-list | A wholly inactive leaf-list round-trips through set format |
| AC-7 | `tokenize` quotes | A quote can be escaped and an unclosed quote is refused with the column |
| AC-8 | leaf-list range and length | `ValueOrArrayNode`, `BracketLeafListNode` and `MultiLeafNode` enforce YANG `range` and `length`; `copp protected-port 0` is refused |
| AC-9 | web `set <list> <key-leaf> <v>` | The web and CLI bar `set` builds the keyed entry as SSH `ensureListEntry` does |
| AC-10 | editor rollback, pending count, commit audit | Rollback fires the commit hook; the change count and diff body come from one producer; `commit confirmed` and CLI-bar commits write an audit row |
| AC-11 | `ze plugin command help help`, `ze help` duplicates | Help for help prints the generated usage; a name both root and YANG verb prints once |
| AC-12 | inert registrations, `ze:hidden` on commands, exported `Registrar` | Completion offers no no-op plugin; a hidden command node is hidden in help and completion; module registrar is read-only to callers |
| AC-13 | htmx `closest` | No PAGEERROR on `/show/system/identity/` at load or after save (browser test) |
| AC-14 | `send bgp flowspec` catalog | The command catalog, grammar and usage derive from one walk; `traffic-class` appears in all three |
| AC-15 | goyang `uses` augment, `deviate add` | RFC 7950 Section 7.13.1 multiple augments under `uses` are applied; `deviate add` of a single-instance property that already exists is refused (Section 7.20.3.2, quoted in the row; verify byte-exact at design) |
| AC-16 | `ze cli -c` transcript | Locally served commands are written to `ze.cli.transcript` like daemon-answered ones |
| AC-17 | rendered markup x4 | The toast class is emitted once; the firewall and command-reference tables render as tables; a CSS comment cannot close early (build check); `markdownCell` is used only for table cells |
| AC-18 | secrets echoed x2 | `ze config set` and the web terminal feedback mask `ze:sensitive` values |
| AC-19 | `foldFilters` default | An unknown pipe operator is refused, not silently placed on one side |
| AC-20 | web `show <list>`, web `compare` | A bare list name renders its entries or says it has none; `compare` diffs against the current committed config |
| AC-21 | empty OSPF area in the editor | `ze config validate` and the SSH editor commit agree on `area 0.0.0.0 { }` |
| AC-22 | live view buffer | A long `monitor event` session keeps a bounded buffer; memory stays flat over 100k events |
| AC-23 | `?` box | A key scrolls the explanation box, or `MaxLongHelpBytes` fits an 80x24 terminal |
| AC-24 | ssh `idle-timeout` | The leaf sets an idle timeout (`IdleTimeout`), and a test holds an active session past the value |
| AC-25 | output formats x3 (REST command list, SSH exec trailer, workbench pending marker) | REST keys are kebab-case; an SSH exec JSON answer is the only output; the pending marker lights for a pending path |
| AC-26 | `show bgp` children x2 | The alias barrier and the declared shapes derive from the registered children; both tests pass |
| AC-27 | CLI grammar R1 x2 | `explain`, `generate wireguard keypair`, `skills`, `support` and `validate config` follow R1 or carry a documented exemption; `TestTheRealCheckoutPassesAndWasRead` passes |
| AC-28 | `ze init` value flags, editor list keys, `TestValidateSyntaxMissingSemicolon` | `--web-cert` and `--web-cert-name` complete as value flags; `SetValue` refuses a key the editor cannot read back; the syntax test and the parser agree on a missing semicolon |
| AC-29 | web goldens x3, `ze-reference-mcp.ci` oracle | The goldens are regenerated from the page that holds the `bgp` subtree; the MCP test expects what `aihelp.Build` derives |
| AC-30 | cli `computeDiff` (moved from `spec-typed-identity-keys`, 2026-11-14) | SSH `show` piped to `compare`, with two siblings whose lines read the same and one of them changed, shows the change with its path; the SSH diff is rendered from the session's pending changes as path-qualified lines, by the one renderer the web `EditorManager.Diff` uses |
| AC-31 | missing mandatory argument (added 2026-11-17, walked into by `spec-plugin-external-yang-at-startup`) | A line whose lone fault is a missing mandatory argument is refused as a lone refused value is (`ArgumentFaults`, `newArgumentFaults`, `RestatedForLine` in `internal/component/command`): the refusal names the missing leaf, why the command needs it, and the whole corrected command as its fix, on every command route. `MissingArgumentError.Error()` (`argbind.go`) no longer stops at `required argument missing: <leaf>`. Every user of the old text is converted (counted by grep on 2026-11-17: 13 Go test files, 5 `.ci` files, 2 fixtures in `internal/test/fixture`, 5 pages under `docs/` and `examples/plugin/python/README.md`), each test change judged by the independent test judge |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| one red-first unit per AC | package of the producer | AC-1 to AC-22 |

### Functional Tests

| Test | AC |
|------|----|
| `.wb` under `test/web/`, `.et` under `test/editor/` | AC-9, AC-10, AC-13, AC-18, AC-20, AC-21 |

## Files to Modify
- The producer files under Current Behavior; a goyang patch file beside the vendored module.

### Integration Checklist
- [ ] Each fix reached through the CLI, web terminal or page the operator uses

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/guide/firewall.md`, `docs/guide/command-reference.md` (AC-17)

## Implementation Steps
1. Read each producer at HEAD; move a fixed row to "Rows found already fixed".
2. Red test, fix, green, page edit, journal cell closed with the commit.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `parser-refuses-a-form-its-producer-emits` | 2026-11-13 | web `/config/edit/` emitter and view routing |
| `parser-refuses-a-form-its-producer-emits` | 2026-10-27 | TestDiscardDeletesNewlyAddedValue unknown `bgp listen` |
| `package-red-not-reproduced` | 2026-10-27 | cli multi-key rename and batched load session records |
| `schema-constraint-not-enforced` | 2026-10-30 | leaf-list `range` ignored by `ParseTreeWithYANG` |
| `per-request-state-on-a-shared-object` | 2026-10-30 | web completer shared across users, tree set per request |
| `per-request-state-on-a-shared-object` | 2026-10-30 | `EditorManager.Tree` read after the session lock is released |
| `package-red-not-reproduced` | 2026-10-27 | TestEditorManagerRenameListEntry |
| `command-waits-for-input-it-was-not-given` | 2026-09-07 | `cmdDecode` |
| `dark-theme-cards-hide-their-text` | 2026-10-04 | generated BFD plugin page |
| `declared-summary-without-its-explanation` | 2026-10-24 | `request`, `show policy` summary in a stripped build |
| `declared-summary-without-its-explanation` | 2026-09-10 | `ze pipe json compact` |
| `declared-summary-without-its-explanation` | 2026-09-07 | `TestEveryCommandNodeHasASummary` |
| `field-carries-two-meanings` | 2026-08-19 | cli column selection |
| `fixture-encodes-an-impossible-state` | 2026-08-29 | looking glass golden capture |
| `format-cannot-hold-a-state-the-tree-holds` | 2026-10-11 | `emitValueOrArrayNop` |
| `format-cannot-hold-a-state-the-tree-holds` | 2026-10-21 | `tokenize` |
| `format-cannot-hold-a-state-the-tree-holds` | 2026-10-27 | `SerializeSetWithMeta` writers drop comments |
| `guard-added-to-one-half-of-a-pair` | 2026-10-13 | leaf-list builders `yang_schema.go` |
| `guard-added-to-one-half-of-a-pair` | 2026-09-03 | web editor rollback |
| `help-answers-unknown-for-its-own-command` | 2026-09-07 | `ze plugin command help` |
| `help-lists-a-root-verb-twice` | 2026-10-08 | `zeUsage` |
| `inert-registration-offered-as-a-choice` | 2026-10-12 | `InternalPluginNameValidator` |
| `inert-registration-offered-as-a-choice` | 2026-11-14 | attached console of `ze start --cli` offers offline-root local-data commands |
| `invariant-enforced-by-an-absent-call-site` | 2026-10-14 | `ze:hidden` on commands |
| `invariant-enforced-by-an-absent-call-site` | 2026-10-08 | `yang.Module.Registrar` |
| `listener-assumes-an-element-target` | 2026-08-18 | web workbench, htmx |
| `one-state-held-in-two-fields` | 2026-09-20 | `send bgp flowspec` catalog |
| `refactor-removes-feature` | 2026-10-07 | `ze cli -c` transcript |
| `rendered-markup-invalid` | 2026-09-23 | `chaos-stream-swaps.wb` |
| `rendered-markup-invalid` | 2026-08-31 | `docs/guide/firewall.md` tables |
| `rendered-markup-invalid` | 2026-08-18 | website stylesheet build |
| `rendered-markup-invalid` | 2026-08-30 | command-equivalents mirror |
| `secret-echoed-to-the-client` | 2026-09-06 | `ze config set` |
| `secret-echoed-to-the-client` | 2026-10-10 | web terminal `set` feedback |
| `silent-fall-through` | 2026-10-21 | goyang `ToEntry` uses augment |
| `silent-fall-through` | 2026-08-19 | `foldFilters` |
| `silent-fall-through` | 2026-10-10 | web `show <list>` |
| `silent-fall-through` | 2026-10-10 | web `compare` baseline |
| `silent-fall-through` | 2026-10-10 | empty OSPF area, `keyedList` |
| `silent-fall-through` | 2026-10-21 | goyang `ApplyDeviate` |
| `stream-buffer-grows-for-its-whole-life` | 2026-09-27 | CLI `outputBuf` |
| `bound-too-small-for-its-own-burst` | 2026-09-04 | `?` box `renderExplanationBox` cannot scroll past 22 rows |
| `comment-describes-superseded-behaviour` | 2026-09-03 | ssh `idle-timeout` applied as `MaxTimeout` |
| `declared-format-contradicts-payload` | 2026-09-28 | REST `api.CommandMeta` has no json tags |
| `declared-format-contradicts-payload` | 2026-10-10 | SSH exec JSON answer followed by plugin frame trailer |
| `declared-format-contradicts-payload` | 2026-10-14 | workbench `entryHasPendingChange` reads `/`, paths joined by spaces |
| `declared-format-contradicts-payload` | 2026-10-26 | command YAML formatter writes strings and keys without quoting |
| `gate-excludes-part-of-its-population` | 2026-09-05 | `cmdBgpChildren` alias barrier is a hand list missing `reject-asn` |
| `gate-excludes-part-of-its-population` | 2026-09-08 | `TestDeclaredShapesReachTheRegistry` red: `show bgp irr`, `show bgp rib` |
| `gate-fires-outside-its-population` | 2026-10-10 | CLI grammar R1: five root commands open with no verb |
| `gate-red-where-nothing-blocks-on-it` | 2026-10-09 | `TestTheRealCheckoutPassesAndWasRead` R1 refusals |
| `published-value-drifts-from-the-behavior-it-describes` | 2026-09-25 | `ze init` `--web-cert` flags registered `FlagValueNone` |
| `zero-value-as-valid-answer` | 2026-09-13 | `newEditor` and `(*Editor).SetValue` |
| `test-against-broken-path` | 2026-08-17 | `TestValidateSyntaxMissingSemicolon` |
| `test-against-broken-path` | 2026-09-04 | web goldens `get-fragment-detail*.txt`, `get-show-bgp-finder.txt` |
| `test-against-broken-path` | 2026-10-09 | `test/plugin/ze-reference-mcp.ci` stale oracle |
| `key-omits-a-fact-the-builder-uses` | 2026-10-11 | AC-30: cli `computeDiff` compares path-less lines as sets |
| `error-names-no-fix-command` | 2026-11-17 | AC-31: `MissingArgumentError` (`command/argbind.go`) says what is missing, with no why and no corrected command |

## Rows found already fixed

| Class file | Date | Surface | Evidence |
|------------|------|---------|----------|
| none in this group at the 2026-10-23 sweep | - | - | - |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (red recorded per AC)
- [ ] Tests PASS
- [ ] Scoped runs: `./le job run go test ./internal/component/web/... ./internal/component/cli/... ./internal/component/config/...`, `./le test functional ui`, `./le test editor`
