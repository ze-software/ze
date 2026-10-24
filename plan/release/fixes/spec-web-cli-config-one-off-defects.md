# Spec: Web UI, CLI and config one-off defects from the journal

| Field | Value |
|-------|-------|
| Status | design |
| Scope | cli |
| Priority | P1 |
| Release reason | Each row is incorrect operator-facing behaviour: a secret echoed, a commit not audited, a list shown as empty, a YANG statement ignored, markup that does not render |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-10-23 |

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
| `command-waits-for-input-it-was-not-given` | 2026-09-07 | `cmdDecode` |
| `dark-theme-cards-hide-their-text` | 2026-10-04 | generated BFD plugin page |
| `declared-summary-without-its-explanation` | 2026-10-24 | `request`, `show policy` summary in a stripped build |
| `declared-summary-without-its-explanation` | 2026-09-10 | `ze pipe json compact` |
| `declared-summary-without-its-explanation` | 2026-09-07 | `TestEveryCommandNodeHasASummary` |
| `field-carries-two-meanings` | 2026-08-19 | cli column selection |
| `fixture-encodes-an-impossible-state` | 2026-08-29 | looking glass golden capture |
| `format-cannot-hold-a-state-the-tree-holds` | 2026-10-11 | `emitValueOrArrayNop` |
| `format-cannot-hold-a-state-the-tree-holds` | 2026-10-21 | `tokenize` |
| `guard-added-to-one-half-of-a-pair` | 2026-10-13 | leaf-list builders `yang_schema.go` |
| `guard-added-to-one-half-of-a-pair` | 2026-09-03 | web editor rollback |
| `help-answers-unknown-for-its-own-command` | 2026-09-07 | `ze plugin command help` |
| `help-lists-a-root-verb-twice` | 2026-10-08 | `zeUsage` |
| `inert-registration-offered-as-a-choice` | 2026-10-12 | `InternalPluginNameValidator` |
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
| `gate-excludes-part-of-its-population` | 2026-09-05 | `cmdBgpChildren` alias barrier is a hand list missing `reject-asn` |
| `gate-excludes-part-of-its-population` | 2026-09-08 | `TestDeclaredShapesReachTheRegistry` red: `show bgp irr`, `show bgp rib` |
| `gate-fires-outside-its-population` | 2026-10-10 | CLI grammar R1: five root commands open with no verb |
| `gate-red-where-nothing-blocks-on-it` | 2026-10-09 | `TestTheRealCheckoutPassesAndWasRead` R1 refusals |
| `published-value-drifts-from-the-behavior-it-describes` | 2026-09-25 | `ze init` `--web-cert` flags registered `FlagValueNone` |
| `zero-value-as-valid-answer` | 2026-09-13 | `newEditor` and `(*Editor).SetValue` |
| `test-against-broken-path` | 2026-08-17 | `TestValidateSyntaxMissingSemicolon` |
| `test-against-broken-path` | 2026-09-04 | web goldens `get-fragment-detail*.txt`, `get-show-bgp-finder.txt` |
| `test-against-broken-path` | 2026-10-09 | `test/plugin/ze-reference-mcp.ci` stale oracle |

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
