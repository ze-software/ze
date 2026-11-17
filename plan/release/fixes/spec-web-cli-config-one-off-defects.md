# Spec: Web UI, CLI and config one-off defects from the journal

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Priority | P1 |
| Release reason | Each row is incorrect operator-facing behaviour: a secret echoed, a commit not audited, a list shown as empty, a YANG statement ignored, markup that does not render |
| Depends | - |
| Phase | feature parity: scoped acceptance |
| Handoff | verify |
| Updated | 2026-11-17 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The 2026-10-23 journal sweep left about 265 open rows that fit none of the
thirteen root-cause specs. This spec owns the 34 in the web UI, the SSH CLI and
editor, the command and pipe layer, YANG loading, and rendered markup (web pages,
website CSS, generated docs). Each row is a claim until a red test reproduces it
at HEAD.

Left to root-cause specs: `yangTypeToValueType` and `validateYangType` (S1);
the `website/AI.md` checklist and the stranded-documentation rows (T2, T4).

## Required Reading

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
| P6e | Conformance of committed configured plugins through the daemon | Model `request plugin conformance name <configured-name>` as an executing action; resolve its target from the accepted daemon generation, reuse the existing conformance report, and prove allowed, denied, unconfigured, candidate-only, removed, canceled and stopped-generation cases before claiming this capability |

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
conformance 2.007 seconds). Successful reloads without
an authoritative tree must withdraw executable authority; rejected reloads must
retain the prior accepted snapshot. Neither that implementation nor full
conformance execution parity is claimed complete.

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
