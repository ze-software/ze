# Spec: Web UI, CLI and config one-off defects from the journal

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Priority | P1 |
| Release reason | Each row is incorrect operator-facing behaviour: a secret echoed, a commit not audited, a list shown as empty, a YANG statement ignored, markup that does not render |
| Depends | - |
| Phase | web amendment implemented; original journal obligations open |
| Handoff | verify |
| Updated | 2026-11-16 |

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
| AC-37 | The graphical editor provides the advanced configuration actions available through its shared editor: structural copy/rename/order, activation, review/validation and confirmed-commit controls, with the same candidate, authorization and publication semantics. | Workbench remains experimental; an action refused by the shared editor remains refused with its reason rather than being approximated by the graphical adapter. |
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
