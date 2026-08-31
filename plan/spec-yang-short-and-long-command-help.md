# Spec: yang-short-and-long-command-help

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | `plan/spec-generated-command-usage.md` |
| Phase | 12/12 (closure) |
| Deferral shard | `plan/deferrals/yang-short-and-long-command-help.md` |
| Handoff | - |
| Updated | 2026-08-31 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A YANG command node declares ONE `description`. Every surface that shows a
command needs TWO things from it: a one-line summary for a list row, a
completion candidate or a table cell, and a longer explanation for the help
page of that one command. Nothing in the model states which half is which, so
each renderer guesses, and the guesses disagree.

FIVE different guesses ship today over the same authored string. Four were known when this spec was written; the fifth was found during implementation:

| Guess | Producer | Rule |
|-------|----------|------|
| First sentence | `internal/core/helpfmt/helpfmt.go` `Summary` | cut at `.`/`!`/`?` followed by a space |
| First line | `cmd/ze/help_command.go` `printCommandTable`; `internal/le/wikicatalog/render.go` `firstLine`; `internal/plugins/completion/words.go`; `internal/le/docvalid/command_surfaces.go` | cut at the first newline |
| Whole string | `internal/component/command/completer.go` `matchChildren`; `internal/component/web/cli.go`; `internal/component/mcp/tools.go` | no cut at all |
| Character count | `internal/le/site/llmsdata.go` `trimInline` (170); `writeLLMSConfigRoots` (180) | cut mid-prose, add an ellipsis |
| Fixed 50 characters (FOUND 2026-08-31, during implementation) | `printSchemaTable` (`internal/component/config/schema/cli/main.go`) | `desc[:47]` plus an ellipsis, with no terminal width read anywhere in the file, so it is a hardcoded guess rather than the width-derived clamp this spec preserves. It also sliced BYTES rather than runes, so a multi-byte character on the boundary left invalid UTF-8 on an operator's terminal. Deleted, which removes the guess and the defect together |
| Terminal width (NOT a guess, deliberately kept) | `internal/component/cli/model_render.go` (`descWidth-3`) | Answers "this terminal is 80 columns", which is a display constraint. The rows above answer "which half of this string did the author mean", which is the question the model now answers |

The corpus proves no heuristic can be repaired. MEASURED through the built
binary with every feature tag on (2026-08-31): 601 command-tree nodes, 390 of
which run a command, 601 carrying a summary and 0 carrying a long form. 419 of
those nodes break at least one shape rule, 874 refusals in total: 300 are not
one sentence, 299 carry a newline, 155 exceed the word cap, 102 lack a full
stop, 18 carry a semicolon. A punctuation rule cannot separate two halves in
text where half the nodes have no sentence break to cut at.

Three consequences reach an operator today. A parent node's own description is
unreachable, because `writeHelp` prints a node's description only when the node
has no children. A multi-line description corrupts the shell-completion format,
which is `name`, tab, `description`, newline. And the web admin command form
shows no description at all, because its one producer never sets the field.

The goal is to replace four guesses with one declaration: a short form every
surface can put on one line, and a long form the per-command help page prints.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - the command model, the five per-command declaration registries, and the Discovery table
  → Decision: a per-command declaration is carried as a registry field with an empty-is-a-floor rule, never as a parse of prose; the second help field follows that shape
  → Constraint: the page is SILENT on every truncation rule in the table above, so this work owes it a "Description contract" section naming the short form, the long form and which surface renders which (`ai/rules/documentation.md`)
  → Constraint: line 1272 ("leaf descriptions provide help text") is WRONG for the ArgDef path. `argDefFor` reads `Type` and `Mandatory` only, and `command.ArgDef` has no description field, so a leaf description inside a `ze:command` container reaches no surface. Repair the sentence in this work
- [ ] `docs/architecture/config/yang-config-design.md` - goyang as the parser, extensions as the declared mechanism, §6 "CLI Help from YANG"
  → Decision: an extension is how Ze declares behavior standard YANG has no statement for; 30 are declared in one module, so the long form is an extension rather than a new convention
  → Constraint: §2.2's extension table lists 11 of the 30 declared extensions and is not marked partial; §6 never names a truncation. Both are repaired in this work
- [ ] `docs/contributing/writing-style.md` - the YANG description entry and the STE habits
  → Constraint: "A YANG description: one sentence on what the leaf MEANS. Never prescribe a CLI spelling." That sentence describes the SHORT form, which settles which half keeps the `description` statement
  → Constraint: the rules are SILENT on whether a description ends in a full stop, and both spellings ship. This spec decides it (AC-3) because the choice is visible in ~400 table cells
- [ ] `docs/architecture/api/architecture.md` - declared by a file this spec changes; named so the spec-validation hook can see it was considered
  → Constraint: the command surface is reached through the dispatcher, so a help field added to the model must not become a second dispatch input
- [ ] `docs/architecture/api/ipc_protocol.md` - the plugin IPC envelope carrying `CommandDecl`
  → Constraint: an added declaration key is additive and `omitempty`; a plugin that omits it MUST still register, which is AC-7
- [ ] `docs/architecture/config/syntax.md` - the config file surface
  → Constraint: no config syntax changes here; the split is confined to command nodes, which is why config leaves are deferred rather than edited
- [ ] `docs/architecture/core-design.md` - registration over central enumeration
  → Constraint: the long form is read through an extension reader in the owning package, never through a central switch that lists command paths
- [ ] `docs/guide/mcp/overview.md` - the MCP tool surface built from the command tree
  → Constraint: a tool description and its action enum are two different lengths, so they take the long form and the summary respectively
- [ ] `ai/rules/cli.md` - the CLI grammar and the agent-facing contract
  → Constraint: a description must not spell an invocation form; `./le docvalid usage-contract` refuses `Usage:`, `Syntax:` and `Filters:`, and that ban extends to the long form
- [ ] `ai/rules/no-layering.md` - replace, never keep both
  → Decision: `helpfmt.Summary` and the first-newline cuts are DELETED, not left as a fallback for an unconverted node. Both copies of `Summary` go
- [ ] `plan/journal/field-carries-two-meanings.md` - the defect class this spec closes
  → Constraint: 2026-08-24 row: "The fix is a SECOND field, never a normalisation of the first. On a cross-process contract the second field's ZERO VALUE MUST be the refusal." `rpc.CommandDecl` is such a contract, which settles the field assignment (see Key Design Decisions)

### RFC Summaries (Scope: protocol)

N/A. This spec changes no wire protocol and no protocol-implementing code.

**Key insights:** (minimal context to resume after compaction)
- One `description`, four incompatible truncations, none declared by the author.
- 601 command-tree nodes (measured through the binary; a static parse of the .yang text counts 653 because it sees nodes no feature tag builds). 390 run a command. 218 RPC descriptions sit beside them in `-api.yang`.
- goyang v1.6.3 vendored. `Entry` carries `Description` and `Exts`, and has NO `Reference` field, so the `reference` statement cannot be repurposed without a type switch on `Entry.Node`.
- `ze-extensions.yang` declares 30 extensions; `ze:related` is the precedent for a long multi-line string argument.
- `internal/le/ste/extract.go` `yangDescriptionRe` matches the literal keywords `description|error-message`, so text moved to a new statement leaves STE scope unless the pattern is extended in the same change.
- Baselines, and a caveat that matters more than the numbers. `./le wiki-catalog check` says `../wiki/command-catalog.md is stale`, from a third session's uncommitted regeneration. At 13:49 on 2026-08-31, with HEAD at 79d199a31, `./le cli-grammar` answered `cli-grammar: OK` (exit 0) and `./le docvalid usage-contract` reported 390 command nodes with 0 authored usage sentences. NONE of these is a property of HEAD. `cligrammar` walks the tree on disk (`filepath.WalkDir` in `cligrammar.go` and `flags.go`), so it judges whatever every session has uncommitted at that instant: a peer measured exit 1 with twelve R9 flowspec findings an hour later, and the whole difference was another session holding two `-cmd.yang` modules uncommitted. Read every number here as a timestamped observation of a shared working tree, never as a baseline to compare against. `plan/journal/gate-verdict-depends-on-the-machine.md` carries the class.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/config/yang/command.go` - `BuildCommandTree` merges `-cmd.yang` modules in sorted name order; `mergeYANGEntry` copies `Entry.Description` to `command.Node.Description`; `validateNode` warns on a missing description; `PathToDescription`/`collectDescriptions` build the path map; `GetCommandExtension` is the shape every extension reader follows; `argDefFor` never reads a description
- [ ] `internal/core/helpfmt/helpfmt.go` - `Summary` is the first-sentence guess; `(*Page).WriteTo` renders `Page.Summary` in the header and `Summary(e.Desc)` per entry
- [ ] `cmd/ze/internal/helpfmt/helpfmt.go` - a SECOND `Summary`, the same guess declared twice
- [ ] `internal/component/command/help.go` - `writeHelp` prints a node's own description only when it has no listed children; `writeHelpEntry` applies `Summary`; `HelpEntries` returns the raw string
- [ ] `internal/component/command/node.go` - `Node.Description`, `CommandEntry.Description`, and `MergeCommandPaths`, whose empty-description test is one of six sentinel sites
- [ ] `internal/component/command/completer.go` - `matchChildren` and `choiceSuggestions` put the WHOLE description on a completion candidate; `Summary` is never called on a completion path
- [ ] `cmd/ze/help_command.go` - `commandEntry` is the JSON catalog shape; `printCommandTable` cuts at the first newline; `printCommandVerbose` prints every line; `collectCommands` emits the whole string
- [ ] `cmd/ze/command_help_page.go` - `commandHelpPage` assigns the whole description to `Page.Summary`, so a multi-line description breaks the one-line header
- [ ] `pkg/plugin/rpc/types.go` - `CommandDecl.Description` and `PipeDecl.Description` cross the plugin process boundary as JSON
- [ ] `internal/component/web/handler_admin.go` - `buildAdminFragmentData` builds `CommandFormData` and never sets `Description`, so the web admin command form shows nothing
- [ ] `internal/le/site/commands.go`, `equivalentdetail.go`, `derived.go` - the published CLI reference row, the per-command detail card, and the 170-character `llms.txt` cut
- [ ] `internal/le/wikicatalog/render.go` - `firstLine` for the summary column, `renderDetail` for the body, driven by `./le wiki-catalog update`
- [ ] `internal/le/ste/extract.go` - `yangDescriptionRe` and `unitsYANG` decide what STE reviews
- [ ] `internal/le/docvalid/usage.go` - `authoredUsage` and `usageMarkers`; the types are package-private and the action is absent from `docVerifyStages`

**Behavior to preserve:**
- Every published artifact keeps its current file shape: the wiki catalog's summary table plus detail blocks, the site's CLI reference table, `llms.txt` line format, and the `ze help command --json` envelope. Only the text inside a cell changes.
- `ze help command --verbose` keeps printing the full explanation for one command.
- A plugin compiled before this change keeps working and keeps its description shown as the summary.
- The offline `ze` root subcommand listing keeps its `%-20s` two-column shape (`cmd/ze/internal/cmdutil/cmdutil.go` `DescribeCommand`).
- The terminal-width clamp in the TUI completion pane stays. It answers a display constraint, not a content question.

**Behavior to change:**
- A command's summary is DECLARED, never derived. Every guess listed in the Task table is deleted.
- A parent node with children shows its own summary and its own long help, which today are unreachable.
- The web admin command form shows the command's help.
- The `llms.txt` command line carries the whole short form rather than 170 characters of the long one.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A `.yang` module under `internal/component/**/yang/` or `internal/plugins/**/yang/`, parsed at startup.
- A plugin's `CommandDecl`, arriving over the plugin IPC channel as JSON.

### Transformation Path
1. goyang parses the module; `Loader.Resolve` produces `gyang.Entry`, whose `Description` holds the short form and whose `Exts` holds the `ze:help` statement.
2. `mergeYANGEntry` reads both and writes `command.Node.Description` and `command.Node.Help`.
3. `MergeCommandPaths` folds plugin-declared commands into the same tree.
4. Renderers read the field their surface needs: summary for a row, help for a page.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG file ↔ command tree | goyang `Entry.Exts`, read by an extension reader shaped like `GetCommandExtension` | No |
| Engine ↔ Plugin | `rpc.CommandDecl` JSON, new `help` key, `omitempty` | No |
| Daemon ↔ CLI client | `ze help command --json` `commandEntry`, new kebab-case key | No |
| Daemon ↔ Web | `/cli/complete` JSON suggestion; admin fragment data | No |
| Daemon ↔ MCP | `tools/list` tool description and action enum | No |
| Daemon ↔ API | `api.CommandMeta`, rendered into OpenAPI `summary` and `description` | No |
| Repository ↔ published wiki | `./le wiki-catalog update` writing `../wiki/command-catalog.md` | No |
| Repository ↔ published site | `./le site build` reading `ze help command --json` | No |

### Integration Points
- `ze-extensions.yang` - the one module that declares an extension; `ze:help` joins the existing 30.
- `GetCommandExtension` - the shape the new reader copies.
- `docvalid usage-contract` - the existing walk over every command description; the new shape gate extends it rather than adding a second walk.

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
| A-1 | goyang exposes an unknown extension on `Entry.Exts` with its argument intact, including a multi-line argument | `internal/component/config/yang/command.go` `GetCommandExtension` reads `Exts` this way for 30 existing extensions; `ze:related` already carries a multi-line argument | The long form needs a different carrier and the whole mechanism changes | Phase 1 wiring test reads a `ze:help` back off a parsed module | confirmed -- `TestMergeYANGEntryReadsHelpExtension` parses a three-line `ze:help` and reads it back with its newlines |
| A-2 | An external plugin compiled against today's `CommandDecl` keeps working when a `help` key is added with `omitempty` | `pkg/plugin/rpc/types.go`; every optional field there uses the same shape | The plugin SDK needs a version negotiation this spec does not plan | A test that decodes a `CommandDecl` JSON payload with no `help` key and asserts the summary still renders | unvalidated |
| A-3 | The 653 command-tree nodes can each be given a correct summary without a subject-matter decision the author cannot make | Corpus: 45% already carry a splittable short+long; 23% are already one short line | Some nodes need an owner ruling and the content phase stalls | Phase 8 reports every node where the existing text yields no honest summary | unvalidated |
| A-4 | STE's per-file ratchet does not go red when prose moves out of `description` into `ze:help` within the same file | `internal/le/ste/ratchet.go` `Ratchet` compares a habit's count against the same file's HEAD text; the extended regex keeps both statements in scope | A green STE run needs a ratchet reseed this spec does not plan | Run `./le ste check` over a converted module in Phase 1 and read the delta | unvalidated |
| A-5 | `./le wiki-catalog check` being stale at HEAD is another session's uncommitted regeneration, not a defect this spec causes | `git -C ../wiki status` shows `M command-catalog.md`, and the `wiki` session confirmed it belongs to a third session | This spec cannot tell its own breakage from the pre-existing red | Record the RED output before Phase 1 and compare after Phase 6 | unvalidated |
| A-6 | No command node's description is load-bearing for a non-help consumer | `PathToDescription` and `collectDescriptions` feed help surfaces only; `argDefFor` reads no description | Shortening a description silently changes behavior elsewhere | Grep every reader of `Node.Description` and name its surface before Phase 3 edits it | broken, then repaired -- Phase 3 walked every reader. All are help surfaces: `writeHelp`, `HelpEntries`, `matchChildren`, `choiceSuggestions`, `mergeChildren`, `mergeHelpText`, `writeCompletionRecord`, `commandHelpPage`, `collectCommands`, `DescribeCommand`, the TUI hint and pane. ONE non-help reader existed and it read the description as DATA rather than as prose: `placeholderValueCommands` (`cmd/ze/internal/cmdutil/cmdutil_test.go`) selected the commands to exercise by matching a value placeholder in the description. Shortening the descriptions took that sample to 0, which is the one way this assumption was BROKEN. The reader is gone: `declaredValueCommands` now samples `command.Usage` (`internal/component/command/usage.go`), the model producer of the invocation form, and keeps a command with a `UsageValue` token. 95 commands, 104 verb forms, measured 2026-08-31 with every feature tag on |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The content phase (653 nodes) becomes the whole spec and the plumbing lands unfinished | Phase 8 starts before Phase 7's gate exists | The gate lands FIRST: no node is converted until a check can tell a good summary from a bad one |
| R-2 | Another session edits a `-cmd.yang` module mid-conversion and the merge loses one side | `git status` shows a `-cmd.yang` this spec has not touched | Announce on the session bus before each module family. `ze-flowspec-cmd.yang` and `ze-cli-announce-cmd.yang` are held by the `grammar` session and are carved out of Phase 8 entirely (see Implementation Steps) |
| R-3 | The per-field merge changes which description wins for a colliding path, silently changing rendered help | The `YANG command description mismatch` warning count changes | Capture the warning set before and after; assert the count and the paths in a test |
| R-4 | Deleting `helpfmt.Summary` breaks a caller outside the two known ones | Compile failure, or an empty help line at runtime | Both call sites are named in Files to Modify; the compiler finds any third |
| R-5 | The published wiki and site artifacts diverge because one is regenerated and the other is not | `./le doc check verify` red after Phase 6 | Both regenerate in the same phase and the same commit |
| R-6 | An empty `ze:help` reads as "this command has no detail" when it means "nobody wrote one yet" | A help page with a summary and a blank body | `validateNode` warns per field; the gate reports coverage rather than failing, so an unconverted node is visible instead of silent |
| R-7 | The `help` key on `CommandDecl` collides with `Completion.Help`, which already means the summary on the same boundary | Two fields named `help` meaning different halves | RESOLVED 2026-08-31: the wire key is `long-help` and the Go field `LongHelp`, chosen independently by three phase agents. `pkg/plugin/rpc/types.go` carries the reason in a comment |
| R-8 | A content agent writes a summary that is fluent and WRONG, asserting a relationship the protocol does not have | A summary that reads well but names a kinship the RFC does not give the two nodes | A node needing domain knowledge for its summary to be true is written by the holder of that knowledge; the flowspec and announce modules are carved out of Phase 8 for exactly this |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Help text only. No wire encoding, no config resolution, no protocol behavior. The worst outcome is an operator reading a wrong or empty explanation, plus two published artifacts (wiki catalog, website command pages) rendering stale text |
| How is it reverted? | Single commit revert for the plumbing. The published wiki and gh-pages artifacts need one regeneration each after a revert |
| Who else touches this path? | `plan/spec-generated-command-usage.md` (same descriptions, in-progress); the `wiki` session owns `../wiki` and has committed and stood down; a third session holds the uncommitted `command-catalog.md`; the `ls-to-list` session's landed rename touched two `-cmd.yang` modules and two `helpfmt.HelpEntry` usage blocks |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A `-cmd.yang` node declaring `ze:help` | → | `mergeYANGEntry` writing `Node.Help` | `TestMergeYANGEntryReadsHelpExtension` |
| `ze <command> --help` on a node WITH children | → | `commandHelpPage` printing the node's own summary and help | `TestHelpPageCarriesBothDeclaredHelpTexts` |
| Tab completion on a partial command | → | `matchChildren` emitting the summary | `TestCompleterSuggestsSummaryNotWholeDescription` |
| `ze help command --json` | → | `commandEntry` carrying both keys | `TestCommandCatalogCarriesSummaryAndHelp` |
| A plugin sending `CommandDecl` with a help field | → | `MergeCommandPaths` folding both halves | `TestPluginCommandDeclCarriesHelp` |
| A plugin sending `CommandDecl` with NO help field | → | the same path, summary preserved | `TestPluginCommandDeclWithoutHelpKeepsSummary` |
| Web admin command form request | → | `buildAdminFragmentData` setting the description | `TestAdminCommandFormShowsHelp` |
| `./le site build` | → | `writeCommandRow` and `equivalentZeCard` | `TestPublishedCommandRowUsesSummary` |
| `./le wiki-catalog update` | → | `wikicatalog.Render` | `TestWikiCatalogRendersDeclaredSummary` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A `-cmd.yang` node declares `description` and `ze:help` | Both reach `command.Node` as separate fields, neither derived from the other |
| AC-2 | Any surface that renders one line for a command | Renders the declared summary verbatim, with no sentence, newline or character truncation applied |
| AC-3 | A summary is authored | It is one sentence, at most 25 words, contains no newline and no semicolon, ends with a full stop, and contains none of `Usage:`, `Syntax:`, `Filters:` |
| AC-4 | `ze <command> --help` on a node that has children | Prints the node's own summary and its long help, then its children's summaries |
| AC-5 | Tab completion, shell completion, and the web completion endpoint | Each receives the summary; no newline can reach the `name`-tab-`description` shell format |
| AC-6 | `ze help command --json` | Carries the summary and the long help under two distinct lowercase kebab-case keys |
| AC-7 | A plugin sends a `CommandDecl` with no help field | The command renders with its summary and an empty long help; nothing renders blank where a summary was expected |
| AC-8 | The web admin command form is requested for any command | Shows the summary as the heading and the long help as the body |
| AC-9 | `./le site build` runs | The CLI reference row shows the summary; the per-command detail page shows the summary as its lede and the long help as its body; the `llms.txt` command line carries the whole summary with no ellipsis |
| AC-10 | `./le wiki-catalog update` runs | The summary table column holds the declared summary; the detail block holds the long help |
| AC-11 | Two `-cmd.yang` modules contribute the same command path with different text | The merge decides each field independently and warns per field, naming the field in the message |
| AC-12 | A command node declares no `description`, or a summary that breaks AC-3 | The shape gate names the path and the rule it broke |
| AC-13 | `./le ste check` runs over a converted module | Both the summary and the long help are still extracted as review units |
| AC-14 | Every one of the 601 command-tree nodes | Carries a summary satisfying AC-3; the long help is present wherever the node has detail worth stating |
| AC-15 | `helpfmt.Summary` is searched for in the tree | It does not exist, in either package, and neither does the dead `writeHelp` renderer that duplicated `HelpEntries` |
| AC-16 | Every one of the 218 RPC descriptions in `-api.yang` | Carries a summary satisfying AC-3 and, where it has detail, a long form declared by the same `ze:help` extension the command tree uses |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Types a partial command and presses tab | terminal → completer → `matchChildren` → summary on the candidate line | `TestCompleterSuggestsSummaryNotWholeDescription` |
| 2 | Runs `ze bgp --help` on a node that has children | dispatcher → `commandHelpPage` → `helpfmt.(*Page).WriteTo` → node summary, node help, children summaries | `TestHelpPageCarriesBothDeclaredHelpTexts` |
| 3 | Runs `ze help command --json` and reads one command | catalog → `collectCommands` → two keys | `TestCommandCatalogCarriesSummaryAndHelp` |
| 4 | Opens the web admin page for a command | HTTP → `buildAdminFragmentData` → heading plus body | `TestAdminCommandFormShowsHelp` |
| 5 | Reads a command page on the published website | `./le site build` → `equivalentZeCard` → lede plus body | `TestPublishedCommandDetailUsesBothForms` |
| 6 | Reads the command catalog on the wiki | `./le wiki-catalog update` → table plus detail | `TestWikiCatalogRendersDeclaredSummary` |
| 7 | Asks an MCP client to list tools | `buildToolDef` → summary in the action enum, long help in the tool description | `TestMCPToolDefUsesSummaryAndHelp` |
| 8 | Reads the OpenAPI document | `api.CommandMeta` → `summary` and `description` | `TestOpenAPICarriesSummaryAndDescription` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMergeYANGEntryReadsHelpExtension` | `internal/component/config/yang/command_test.go` | AC-1, the extension reader | |
| `TestMergeYANGEntryWarnsPerFieldOnMismatch` | `internal/component/config/yang/command_test.go` | AC-11; replaces the single-field assertion at the existing mismatch test | |
| `TestMergeYANGEntryWireMethodOverwriteIsPerField` | `internal/component/config/yang/command_test.go` | the undocumented second precedence rule, now stated per field | |
| `TestHelpPageCarriesBothDeclaredHelpTexts` | `cmd/ze/command_help_page_test.go` | AC-4, on the shipped page. It carries the presence, the one-line header and the summary/help/children ORDER. The `writeHelp` test that first held AC-4 died with that unshipped renderer (Phase 8e) | |
| `TestCompleterSuggestsSummaryNotWholeDescription` | `internal/component/command/completer_test.go` | AC-5 | |
| `TestShellCompletionRejectsNewlineInSummary` | `internal/plugins/completion/flags_test.go` | AC-5, the tab-separated format | |
| `TestCommandCatalogCarriesSummaryAndHelp` | `cmd/ze/help_command_test.go` | AC-6 | |
| `TestPluginCommandDeclCarriesHelp` | `pkg/plugin/rpc/types_test.go` | AC-7 | |
| `TestPluginCommandDeclWithoutHelpKeepsSummary` | `pkg/plugin/rpc/types_test.go` | AC-7, the zero-value refusal | |
| `TestAdminCommandFormShowsHelp` | `internal/component/web/handler_admin_test.go` | AC-8 | |
| `TestMCPToolDefUsesSummaryAndHelp` | `internal/component/mcp/tools_test.go` | story 7 | |
| `TestOpenAPICarriesSummaryAndDescription` | `internal/component/api/schema_test.go` | story 8 | |
| `TestPublishedCommandRowUsesSummary` | `internal/le/site/commands_test.go` | AC-9 | |
| `TestPublishedCommandDetailUsesBothForms` | `internal/le/site/equivalentdetail_test.go` | AC-9 | |
| `TestLLMSCommandLineCarriesWholeSummary` | `internal/le/site/derived_test.go` | AC-9, the deleted 170-character cut | |
| `TestWikiCatalogRendersDeclaredSummary` | `internal/le/wikicatalog/render_test.go` | AC-10 | |
| `TestHelpShapeGateNamesTheBrokenRule` | `internal/le/docvalid/helpshape_test.go` | AC-12 | green; one subtest per rule, each proven by mutating its check |
| `TestSTEExtractsHelpExtension` | `internal/le/ste/ste_test.go` | AC-13 | green; red before the pattern was extended |
| `TestEveryCommandNodeHasASummary` | `internal/le/docvalid/helpshape_test.go` | AC-14, coverage over the whole tree | RED by design until Phase 8: 419 of 601 nodes break a rule |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Summary word count | 1-25 | 25 words | 0 words (empty description) | 26 words |
| Summary sentence count | 1 | 1 | 0 | 2 |
| Summary newline count | 0 | 0 | N/A | 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `help-parent-node` | `test/ui/help-parent-node.ci` | An operator asks for help on a node that has children and reads its explanation | green; RED forced by guarding `page.Summary`/`page.Help` on a childless node |
| `completion-words-summary` | `test/ui/completion-words-summary.ci` | An operator tabs a partial command and the candidate line carries the declared summary, whole, on one line | green; RED forced twice -- once by piping the long form onto the candidate (`mergeChildren` copying `Help` onto an existing node, `matchChildren` emitting `Description + "\n" + Help`), once by putting a newline in a description with the fold in `writeCompletionRecord` removed |
| `help-command-json-two-fields` | `test/ui/help-command-json-two-fields.ci`, `test/ui/help-command-json-summary-only.ci` | An agent reads the catalog and gets a summary and a help text, and gets no long form for a command that declares none | green; RED forced by binding `long-help` to the summary in `collectCommands` |

### Interop Tests (Scope: protocol)

N/A. No wire-visible behavior changes; no protocol peer is involved.

## Files to Modify

- `internal/component/config/yang/modules/ze-extensions.yang` - declare `ze:help`
- `internal/component/config/yang/command.go` - the extension reader, `mergeYANGEntry` per-field merge and per-field warning, `validateNode` per-field warning, `PathToDescription` beside a help map, and the `argDefFor` doc correction
- `internal/component/command/node.go` - `Node.Help`, `CommandEntry.Help`, and the `MergeCommandPaths` sentinel
- `internal/component/command/help.go` - DELETE the unshipped renderer `writeHelp`, `writeHelpLine` and `writeHelpEntry` (owner approved 2026-08-31); keep `HelpEntries`, `listedChildNames` and `describeChildren`, which `commandHelpPage` calls
- `internal/component/command/completer.go` - `matchChildren`, `choiceSuggestions`
- `internal/core/helpfmt/helpfmt.go` - DELETE `Summary`; add the long-help block to `Page`/`WriteTo`
- `cmd/ze/internal/helpfmt/helpfmt.go` - DELETE the second `Summary`
- `cmd/ze/help_command.go` - `commandEntry`, `printCommandTable`, `printCommandVerbose`, `collectCommands`
- `cmd/ze/command_help_page.go` - `commandHelpPage`
- `cmd/ze/help_ai.go` - delete the `". "` first-sentence cut
- `cmd/ze/internal/cmdutil/cmdutil.go` - `DescribeCommand`
- `cmd/ze/internal/cmdutil/cmdutil_test.go` - `declaredValueCommands` replaces `placeholderValueCommands`: the sample moves from a regex over the description to a `UsageValue` token from `command.Usage`, because this spec took the grammar spellings out of the prose
- `cmd/ze/hub/command_meta.go`, `cmd/ze/hub/api.go` - carry both halves
- `internal/component/api/types.go`, `internal/component/api/schema.go` - `CommandMeta`, OpenAPI `summary` and `description`
- `internal/component/mcp/tools.go` - `CommandInfo`, `buildToolDef`
- `internal/component/plugin/server/command.go`, `command_registry.go` - the dispatcher side
- `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_types.go` - `CommandDecl`, `PipeDecl`
- `internal/plugins/meta/cmd/help.go` - `commandHelp`, `handleBgpCommandList`
- `internal/plugins/completion/flags.go`, `words.go`, `root_commands.go` - summary only
- `internal/component/cli/model_render.go`, `model_keys.go`, `model.go` - summary on the hint and the pane
- `internal/component/cli/client/main.go`, `inject.go` - the merge sentinels
- `internal/component/web/handler_admin.go` - set the command form description
- `internal/component/web/cli.go` - the completion payload
- `internal/component/config/cli/main.go`, `internal/component/config/storage/cli/main.go` - the two `helpfmt.HelpEntry` usage blocks the `ls`-to-`list` rename built
- `internal/le/site/commands.go`, `equivalentdetail.go`, `derived.go`, `catalog.go` - published rows, detail cards, `llms.txt`
- `internal/le/wikicatalog/render.go`, `catalog.go` - summary column and detail body
- `internal/le/docvalid/usage.go`, `command_surfaces.go`, `command_render.go` - the shape gate and the drift comparison
- `internal/le/doc/wiring/docverify.go` - wire the shape gate into `docVerifyStages`
- `internal/le/ste/extract.go` - extend `yangDescriptionRe` to `ze:help`
- Every `-cmd.yang` and `-api.yang` module under `internal/component/**/yang/`, `internal/component/bgp/plugins/**/yang/` and `internal/plugins/**/yang/` - the 601-node command conversion and the 218 RPC descriptions
- `docs/architecture/api/commands.md` - the Description contract section, and the line 1272 correction
- `docs/architecture/config/yang-config-design.md` - §6, and the §2.2 extension table
- `docs/architecture/web-interface.md` - the leaf description renderings the page does not name
- `docs/contributing/gh-pages.md` - the command surfaces the page omits
- `docs/contributing/writing-style.md` - the summary shape rule, including the full stop
- `website/AI.md` - the stale `RenderCommandSurfaces` claim
- `docs/guide/command-reference.md`, `docs/guide/cli.md` - rows whose paraphrase drifts from the new summary
- `../wiki/command-catalog.md`, `command-reference.md`, `cli.md`, `show-commands.md` - regenerated and re-synced

## Files to Create

- `internal/le/docvalid/helpshape.go` - the summary shape gate (AC-3, AC-12, AC-14)
- `plan/deferrals/yang-short-and-long-command-help.md` - the deferral shard
- `test/ui/help-parent-node.ci` - functional test for AC-4. The `ui` suite, not `editor`: the help page an operator reads for one command is produced by `commandHelpPage` (`cmd/ze/command_help_page.go`) on the `ze <path> help` CLI path, and the `.et` editor harness reaches no such page
- `test/ui/help-command-json-two-fields.ci` - functional test for AC-6, the two declared keys
- `test/ui/help-command-json-summary-only.ci` - functional test for AC-6, the zero-value half. It is a second file because `reject=stdout` matches the ACCUMULATED stdout of every command in a file, so the negative assertion is only meaningful when the file runs one command (`docs/architecture/testing/ci-format.md`)
- `test/ui/completion-words-summary.ci` - functional test for AC-5. `ze completion words run show bgp rib` is the surface: the `.et` editor harness completes CONFIG paths, not command paths, so it reaches no command summary. One command in the file, because the format reject below matches the ACCUMULATED stdout of every command in it

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/config/yang/modules/ze-extensions.yang` declares `ze:help`; every `-cmd.yang` uses it |
| YANG validation constraints | N-A | `ze:help` takes a free-text argument; its shape is checked by the docvalid gate, which native YANG statements cannot express |
| YANG custom validators | N-A | The constraint is on prose shape, not on a config value, so it belongs in the gate rather than in `ze:validate` |
| CLI commands/flags | No | No command is added, removed or renamed. Help text only |
| CLI grammar (keyword before value) | No | No token changes. `./le cli-grammar` is green at HEAD (measured, exit 0) and is in no blocking set, so it is run by hand before and after |
| Editor autocomplete | Yes | `internal/component/command/completer.go`, `internal/component/cli/model_render.go`; proven end to end by `test/ui/completion-words-summary.ci` |
| Functional test for new RPC/API | Yes | `test/ui/help-command-json-two-fields.ci` and `test/ui/help-command-json-summary-only.ci`, both green in `./le functional ui` |
| Pipe completeness | N-A | No new command and no new answer shape |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module, binary or certificate |
| Prometheus counters/metrics | N-A | No observable runtime state added |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP protocol surface touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - command help now has a declared summary and a declared explanation |
| 2 | Config syntax changed? | No | No config leaf, container or value changes |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, `docs/guide/cli.md` - the rows paraphrase descriptions that change |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` - the Description contract section, the catalog key set, and the line 1272 correction |
| 5 | Plugin added/changed? | Yes | `docs/architecture/api/process-protocol.md` - the `CommandDecl` help key |
| 6 | Has a user guide page? | Yes | `docs/guide/cli.md` |
| 7 | Wire format changed? | No | No BGP or other protocol wire format touched |
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No protocol behavior |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` - the two new functional tests |
| 11 | Affects daemon comparison? | No | No feature parity claim changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` §6 and §2.2 |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | N-A | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md` - the command declaration gains a field |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/spec-yang-short-and-long-command-help.md` before Phase 9 and name every result |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/web-interface.md`, `docs/contributing/gh-pages.md`, `website/AI.md` each describe a renderer this work changes or already misdescribe one |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - declare the extension, read it, carry it
   - Tests: `TestMergeYANGEntryReadsHelpExtension`, `TestHelpPageCarriesBothDeclaredHelpTexts`
   - Files: `ze-extensions.yang`, `internal/component/config/yang/command.go`, `internal/component/command/node.go`
   - Verify: one converted module round-trips both fields; the wiring tests fail before the reader exists. The pre-change baseline is RECORDED (2026-08-31, HEAD 79d199a31): `wiki-catalog check` stale, `cli-grammar` OK, `usage-contract` 390 nodes / 0 authored sentences. Compare against it rather than re-measuring
2. **Phase: Merge semantics** - decide each field independently
   - Tests: `TestMergeYANGEntryWarnsPerFieldOnMismatch`, `TestMergeYANGEntryWireMethodOverwriteIsPerField`
   - Files: `internal/component/config/yang/command.go`
   - Verify: the existing `peer` node collision warns naming which field collided; the warning set before and after is captured and compared
3. **Phase: Delete the guesses** - one declaration, no derivation
   - Tests: `TestCompleterSuggestsSummaryNotWholeDescription`, `TestShellCompletionRejectsNewlineInSummary`, `TestCommandCatalogCarriesSummaryAndHelp`
   - Files: both `helpfmt` packages, `command/help.go`, `completer.go`, `help_command.go`, `command_help_page.go`, `help_ai.go`, `cmdutil.go`, the completion plugin, the TUI files
   - Verify: `Summary` exists nowhere; every one-line surface reads the declared field
4. **Phase: The plugin boundary** - the second field with a refusing zero value
   - Tests: `TestPluginCommandDeclCarriesHelp`, `TestPluginCommandDeclWithoutHelpKeepsSummary`
   - Files: `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_types.go`, `plugin/server/command.go`, `command_registry.go`, `meta/cmd/help.go`
   - Verify: a payload with no help key renders a summary and an empty body, never a blank summary
5. **Phase: Web, TUI and machine surfaces** - close the unwired form
   - Tests: `TestAdminCommandFormShowsHelp`, `TestMCPToolDefUsesSummaryAndHelp`, `TestOpenAPICarriesSummaryAndDescription`
   - Files: `web/handler_admin.go`, `web/cli.go`, `mcp/tools.go`, `api/types.go`, `api/schema.go`, `hub/command_meta.go`, `hub/api.go`
   - Verify: the admin command form renders help where it rendered nothing
6. **Phase: Published artifacts** - site and wiki in one commit
   - Tests: `TestPublishedCommandRowUsesSummary`, `TestPublishedCommandDetailUsesBothForms`, `TestLLMSCommandLineCarriesWholeSummary`, `TestWikiCatalogRendersDeclaredSummary`
   - Files: `internal/le/site/*`, `internal/le/wikicatalog/*`, `../wiki/*`
   - Verify: `./le --name main-19 site build` and `./le --name main-19 wiki-catalog update` both run; `./le doc check verify` returns to the Phase 1 baseline
   - **The catalog hunk is CLEARED for regeneration, and here is the evidence rather than the conclusion.** `../wiki/command-catalog.md` carries 2640 insertions and 479 deletions, uncommitted. Its mtime is 2026-08-27 03:54, four days before this work, so it belongs to no running session; every commit-tooling file in `../wiki/tmp/` is from 2026-08-31 13:30-13:43 and belongs to the `wiki` session. The `wiki` session then generated a fresh catalog to a SCRATCH path and diffed it against the working-tree file: every hunk is command-model content, and no prose-shaped line a generator would not emit was found. It is a four-day-old generation of a model that has since moved. The full patch is preserved at `backups/wiki-command-catalog-20260831-161626.patch` regardless
   - **EXPECT A LARGE COSMETIC DIFF AND DO NOT READ IT AS CONTENT MOVEMENT.** The generator's markdown escaping changed since 2026-08-27: it now escapes punctuation, so a sentence ends `demand\.` and a citation reads `\(RFC 8955\)\.`. Measured by the `wiki` session: the raw regeneration diff is 4514 lines, and normalising the escaping away drops it to 2380. So roughly 2100 lines of the diff have nothing to do with this spec. A reviewer who reads the line count as the size of the change will be wrong by about half
   - Sequencing is the OWNER'S call, not this spec's: Thomas has told the `wiki` session to hold its wiki commits while this change is in flight, and that session has an uncommitted `rfc-implementation.md` refresh waiting on the same instruction
7. **Phase: The gate** - before any content is written
   - Tests: `TestHelpShapeGateNamesTheBrokenRule`, `TestSTEExtractsHelpExtension`, `TestEveryCommandNodeHasASummary`
   - Files: `internal/le/docvalid/helpshape.go`, `usage.go`, `internal/le/doc/wiring/docverify.go`, `internal/le/ste/extract.go`
   - Verify: the gate names a bad summary by path and by broken rule, and reports coverage over the whole tree
8. **Phase: Content conversion** - 601 command nodes and 218 RPC descriptions
   - Tests: the Phase 7 gate, run over each module family as it converts
   - Files: every `-cmd.yang` and `-api.yang` module EXCEPT the two carved out below
   - Verify: the gate reports full coverage. Announce each module family on the session bus before starting it
   - **CARVED OUT, do not convert (owner of the domain writes these):** `internal/component/bgp/plugins/nlri/flowspec/yang/ze-flowspec-cmd.yang` and `internal/component/bgp/plugins/cmd/announce/yang/ze-cli-announce-cmd.yang`, about 27 nodes. The `grammar` session rewrote both on 2026-08-31 and will write their summaries itself. A summary inferred from the node NAME would state a relationship the protocol does not have: `destination-ipv4` and `destination-ipv6` are not two forms of one match, because RFC 8955 Type 1 and RFC 8956 Type 1 share a type code and carry different wire encodings, one with an offset. The announce and withdraw forms are likewise separate commands, because the arguments after the form word differ, so a summary treating them as one verb with a mode word undoes that split
9. **Phase: Documentation** - every page named in the Documentation checklist
   - Tests: `./le doc check verify`
   - Files: the docs rows above, plus the anchors `./le spec citation anchors` returns
   - Verify: no page describes a truncation this spec deleted

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and AC-14 has a coverage number rather than a sample |
| Feature completeness | Each of the eight user stories runs end to end; the web admin form in particular, which rendered nothing before |
| Correctness | No renderer derives a summary from the long form; the per-field merge preserves the pre-change winner for every path that did not collide |
| Naming | The new JSON key is lowercase kebab-case on every boundary, and does not collide with the existing `help` key on `Completion` |
| Data flow | The long form reaches only per-command help pages; no list row, completion candidate or tab-separated format can receive it |
| Rule: `ai/rules/no-layering.md` | Both `Summary` functions and all four truncations are DELETED, not left as a fallback |
| Rule: `ai/rules/evidence.md` | The summary shape gate fails closed: a node with no summary is named, never skipped |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `ze:help` declared | `grep 'extension help' internal/component/config/yang/modules/ze-extensions.yang` |
| No `Summary` guess remains | `grep -rn 'func Summary' internal/core/helpfmt cmd/ze/internal/helpfmt` returns nothing |
| No character truncation of a command description | `grep -rn 'trimInline' internal/le/site/derived.go` shows no command-description caller |
| Every command node carries a summary | `./le --name main-19 docvalid help-shape` reports zero uncovered paths |
| Published artifacts agree with the model | `./le --name main-19 doc check verify` at or better than the Phase 1 baseline |
| Both halves stay in STE scope | `./le --name main-19 ste check` extracts units from a `ze:help` argument |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A plugin-supplied help string reaches a terminal, an HTML page and a JSON document. Confirm the web path escapes it (the templ layer) and that no newline or control character can break the tab-separated shell-completion format or the one-line terminal header |
| Resource exhaustion, per declaration | A plugin can declare an arbitrarily long help string. ANSWERED by Phase 4: `validateHelpDecls` and `validateDeclaredText` (`internal/component/plugin/server/startup.go`) bound the summary at 256 bytes and the long help at 4096, refuse every C0 control and DEL in the summary because it reaches a tab-separated completion format, and allow only newline in the long form. Bounds justified against the measured widest declared summary in the tree, 64 bytes |
| Resource exhaustion, per REPLY — NOT YET ANSWERED, and it is a different question | Validating what a peer may DECLARE tells you nothing about what ze WRITES from those declarations. Does anything downstream build a string proportional to the COUNT of declarations, which a plugin chooses, rather than to the size of any one of them? Named candidates to check at the producer: `buildToolDef` (`internal/component/mcp/tools.go`) joins every action's text into ONE tool description; `commandText` in the same file; the shell-completion writer, which emits one record per command; the admin form and the published catalog, each a sum over declarations. The closure review MUST answer this and MUST NOT treat the per-declaration bound above as the answer. Evidence that the two are unrelated, stated as it was actually measured rather than as it was first reported to me: a sibling session found an UNBOUNDED WRITE in `WriteLCPOptions`, latent at HEAD with no reachable trigger, which its own in-flight change would have made reachable had the bound not landed in the same tree. No peer was ever exposed. What the case proves is undiminished and is the reason it is cited here: the PPP option parser bounded every read and refused every malformed input, and the overflow was a reply built ENTIRELY FROM VALID INPUT, one entry per received option. A per-item bound and a per-reply bound are independent, and passing the first is not evidence about the second. The sharper half is why this belongs in a review rather than in an author's head: the HEAD contract said "caller MUST ensure capacity", every caller honoured it, and one unrelated ruling about which reply to send silently made that false. The person changing the arithmetic had no reason to look at the writer |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| `./le doc check verify` red | Compare against the Phase 1 baseline before treating it as this spec's breakage |
| A node whose text yields no honest summary | Report it under A-3; do not invent one |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The split is already half-built, three incompatible ways, and none of them is documented anywhere. This spec does not introduce a distinction; it replaces three guesses with one declaration.
- The corpus, not taste, decides that a convention cannot work: 178 of 653 nodes carry no terminal punctuation and 23 exceed 120 characters with no sentence break, so a quarter of the tree has no derivable split point.
- **General practice for "is this generated file safe to overwrite", forced by a prohibition rather than chosen.** `git stash`, `git restore` and `git reset` are all banned here, so once a generator has written over an uncommitted hunk there is no sanctioned way back. That leaves exactly one safe move: generate to a scratch destination nobody owns, diff it against the working-tree file, and let the file answer whether its content is derivable. If every hunk is generator output, overwriting loses staleness rather than work. If any line is prose no generator would emit, it is somebody's work and the answer is to find the owner. This is what settled `../wiki/command-catalog.md`, and it applies to every generated artifact in a shared checkout. `ai/rules/never-destroy-work.md` names the save-a-patch-and-stop route but not this one, which is a gap worth filling.
- **A total is not a stable comparison in a shared checkout; a section delta is.** Two sessions counting the same generated artifact minutes apart get different totals because commits keep landing, and the difference says nothing about either count being wrong. Compare the deltas a known change PREDICTS instead: `announce` 1 to 3, `withdraw` 1 to 3, `peer` 2 to 5 and `show` 254 to 259 are the shape the announce/withdraw split and the `ls` to `list` rename produce, and they hold while a total moves under both observers. Same failure as the `cli-grammar` baseline recorded above, in a different costume.
- **A root container's summary is declared dozens of times and read once.** YANG augmentation makes every module that hangs a subtree under `show` declare `container show` itself, so 49 modules carry the same summary string. `mergeHelpText` keeps the first loaded and warns on the rest, which means 48 of those 49 authored strings are dead text: never rendered, never read, and free to drift from the one that wins. The gate cannot be cleared for such a node by any single family agent, because the node's twins live in other families, so it needs one cross-cutting sweep rather than N local edits. Measured on `show`, `monitor`, `update`, `show system` and `set`. This is the `ai/rules/principles.md` one-declaration rule broken by the schema language's own augmentation model, and it is worth its own look after this spec: nothing today tells an author that the description they are writing will never be shown.
- The failing-node count is not the edit count. goyang expands a `uses` statement by duplicating the grouping's entry per instantiation (`vendor/github.com/openconfig/goyang/pkg/yang/entry.go:654`, `case *Uses:` returning `ToEntry(g).dup()`), so one `description` statement inside a grouping appears as N nodes in the command tree and as N refusals in the gate. `ze-cli-announce-cmd.yang` instantiates `announce-forms` and `withdraw-forms` twice each, so every announce and withdraw form is reachable bare and under `peer <selector>` from one authored string. Phase 8 must be sized and REPORTED by authored statements, not by failing nodes, or it will look several times larger than it is and its progress will jump unevenly.
- `Node.Description == ""` is a sentinel in six places. Two fields means each site must say which field emptiness tests, and that is where a silent wrong answer would enter.
- A summary is not a shortened description, and the difference is a correctness question rather than a length one. Compressing a node's text forces a claim about what the node IS, and the compressor can assert a relationship the protocol does not have. The measured case: `destination-ipv4` and `destination-ipv6` read as two forms of one match, but RFC 8955 Type 1 and RFC 8956 Type 1 share a type code with different wire encodings. So a node whose summary needs domain knowledge to be TRUE is written by whoever holds that knowledge, never inferred from the node name by the agent doing the sweep.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `description` keeps the SHORT form; a new `ze:help` extension carries the LONG form | `description` keeps the long form and a `ze:summary` extension carries the short one | `rpc.CommandDecl` crosses the plugin process boundary. A plugin compiled today sends `description` alone. If that is the summary, an old plugin degrades to "summary, no detail". If it were the long form, every list row for that plugin's commands would render blank. `plan/journal/field-carries-two-meanings.md` (2026-08-24) states the rule: on a cross-process contract the second field's zero value must be the refusal. `docs/contributing/writing-style.md` independently describes a YANG description as one sentence on what the node means, and STE's 25-word descriptive cap already treats it as one |
| A declared extension | A blank-line convention inside the single `description` | Zero of 653 command nodes use a blank line today, so the separator would be unambiguous and it is the cheaper change. It was rejected because it is declaration-by-punctuation, which is the defect being removed, and because a convention has to be re-implemented as string parsing in the plugin SDK, where a field is simply a field |
| Repurposing `reference` was rejected | RFC 7950 `reference` statement | goyang's `Entry` has no `Reference` field. The statement is parsed onto the AST node but never copied to the Entry, so every reader would need a type switch on `Entry.Node`. Zero `reference` statements exist in the corpus, and RFC 7950 §7.21.5 defines it as an external-document citation, not help text |
| The gate lands before the content | Convert the nodes first, add a check afterwards | 601 hand-written summaries with no check is 601 chances to write a bad one, and a check written afterwards has to be argued against text already in the tree. Sequencing it first makes `./le verify worktree` red until Phase 8 lands, which is the intended order rather than a defect |
| The TUI terminal-width clamp stays | Delete every truncation uniformly | A width clamp answers "this terminal is 80 columns", which is a display constraint. The other four answered "which half of this string did the author mean", which is the question the model now answers |

## Known Limitations

- **AC-5's web half has no FUNCTIONAL test.** `ze completion words run` is covered by `test/ui/completion-words-summary.ci`, and that is the command every shell integration calls at tab time (`bash.go`, `zsh.go`, `fish.go`, `nushell.go`), and the TUI pane reads the same `matchChildren` suggestions. `HandleCLIComplete` (`internal/component/web/cli.go`) returns that same completer's `Description` over HTTP and is covered by a unit test (`TestCLICompleteSendsSummaryNotExplanation`), so the PRODUCER is pinned twice. What is missing is a functional test driving the HTTP endpoint itself. Recorded rather than claimed: the agent that wrote the completion test declined to substitute a unit test for the missing half, which is the correct call.
- **The interactive CLI renders no long form.** `ze <command> --help` prints both halves, and the daemon's `command help "<path>"` answers with `long-help` beside `description` (`internal/plugins/meta/cmd/help.go`, `keyLongHelp`), and `MergeCommandPaths` carries `Node.Help` into the CLI client's tree. But nothing in `internal/component/cli/` RENDERS it: `model_render.go` and `model_keys.go` read `Suggestion.Description` only. So an operator on the SSH CLI, which is Ze's primary surface, sees the summary and never the explanation, while the terminal binary, the web admin form, MCP, OpenAPI, the website and the wiki all show both. The `help` verb exists in the CLI (`model.go`, `cmdHelp`) and the data is already in the client tree, so the fix is a rendering path rather than a model change. FOUND BY THE OWNER asking to see the CLI, not by any gate. Left for his decision on scope rather than taken unilaterally.

- CONFIG leaf descriptions (1241 leaves, plus 443 containers and 277 enums) keep their single `description`. The user scoped this work to commands. The web config editor therefore keeps showing one string in its tooltip, its input title and its placeholder. Extending the split to config leaves is a separate decision with its own corpus, and it is recorded in the deferral shard rather than assumed.
- `docs/guide/command-reference.md`, `docs/guide/cli.md` and the hand-written wiki pages paraphrase rather than copy the YANG text, so they drift rather than break, and no gate reports it. This spec re-syncs them once; keeping them in sync is not solved here.
- `./le wiki-catalog check` reports the published catalog stale at HEAD, from a third session's uncommitted regeneration. This spec records that baseline and regenerates the catalog in Phase 6; it does not adopt the other session's hunk.

## RFC Documentation (Scope: protocol)

N/A. This spec implements no protocol requirement.

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
- [ ] AC-1..AC-16 all demonstrated
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
- `ze:help` declared in `internal/component/config/yang/modules/ze-extensions.yang` (extension 31). `getHelpExtension` (`internal/component/config/yang/command.go`) reads it off `Entry.Exts` for a command node and off `gyang.RPC.Exts()` for an rpc, so one reader serves both carriers.
- `command.Node.Help`, `command.CommandEntry.Help`, `yang.RPCMeta.Help`, `yang.PathToHelp`. `mergeHelpText` decides each of the two fields on its own and warns naming the field.
- Five guesses DELETED: `helpfmt.Summary` (both copies), the first-newline cuts in `printCommandTable`, `wikicatalog.firstLine`, `completion/words.go` and `docvalid/command_surfaces.go`, the `". "` cut in `help_ai.go`, the 170-character `trimInline` on the `llms.txt` command line, and the fixed 50-character byte slice in `printSchemaTable` (`internal/component/config/schema/cli/main.go`).
- The plugin IPC boundary carries `CommandDecl.LongHelp` (`long-help`, `omitempty`). `validateHelpDecls` / `validateDeclaredText` (`internal/component/plugin/server/startup.go`) bound the summary at 256 bytes with no control character and the long help at 4096 bytes with newlines kept.
- Every machine surface carries both halves: `api.CommandMeta` into OpenAPI `summary`/`description`, `zemcp.CommandInfo` into the tool description and the action enum, `aihelp.RPC`, `hub.commandMeta`, the web admin form, the wiki catalog and the published site.
- `internal/le/docvalid/helpshape.go`, the shape gate, wired into `docVerifyStages`. It judges three corpora against seven rules and reports coverage beside its refusals.
- Content: 601 command nodes, 211 RPCs and 19 offline local commands converted across 111 `.yang` modules and 9 `register.go` files.

### Bugs Found/Fixed
- `commandHelpPage` (`cmd/ze/command_help_page.go`) had ACQUIRED the guard this spec exists to remove: it set `page.Summary` and `page.Help` only for a childless node. Fixed at the producer; `test/ui/help-parent-node.ci` forces the regression red.
- The `peer` verb root read `Raw byte injection (testing and conformance only)`, false for its `update`, `announce` and `withdraw` children. Now `Act on selected BGP peer sessions.` (`ze-raw-cmd.yang`).
- The `request` verb root read `Cache operational actions` while owning twelve children from a dozen owners. Now `Commands that act on the running system.` (`ze-cli-cache-cmd.yang`).
- `Loader.ModuleNames` walked goyang's module map raw, which keys a revisioned module under both its bare name and `<name>@<revision>`. `gnmi.buildModels` was emitting most supported models TWICE in its Capabilities reply. Fixed at the loader; `TestLoaderNamesEachModuleOnce` guards it.
- `buildAdminFragmentData` (`internal/component/web/handler_admin.go`) never set the command form's description, so the web admin form rendered no help at all. It now takes the command tree and sets both halves.
- `printSchemaTable` sliced BYTES at 47, so a multi-byte character on the boundary left invalid UTF-8 on the terminal. The cut is gone.
- The help-shape gate itself excluded the offline local command registry, a third corpus `ze help command --json` publishes. `generate wireguard keypair` published a two-sentence 41-word summary nothing refused. Closed in Phase 12.

### Documentation Updates
- `docs/architecture/api/commands.md` (the "A command's two help texts" section, the "Which surface renders which field" table, the line-1272 ArgDef correction, the TUI clamp rows), `docs/architecture/config/yang-config-design.md` (§2.2 completed to 31 extensions, §6 rewritten, the rpc half, the module-identity note), `docs/architecture/cli/command-completion.md`, `docs/architecture/web-interface.md`, `docs/architecture/mcp/overview.md`, `docs/architecture/api/process-protocol.md` (landed in `ccaf018a7`, another session's commit).
- `docs/guide/cli.md`, `docs/guide/command-reference.md`, `docs/guide/command-catalogue.md`, `docs/guide/api.md`, `docs/guide/mcp/overview.md`, `docs/guide/web-interface.md`, `docs/features.md`, `docs/features/introspection.md`, `docs/plugin-development/README.md`, `commands.md`, `protocol.md`, `docs/contributing/documentation-testing.md`, `docs/contributing/writing-style.md`, `docs/contributing/gh-pages.md`, `website/AI.md`.
- `ai/rules/plugins.md` gained the point `bound-and-clean-every-declared-text`. `ai/PACKAGE-MAP.md` and `ai/skills/ze-review.md` regenerated.
- `./le doc check verify`: the `help-shape` and `usage-contract` stages are GREEN. The drift stage reports 3294 issues and every one names `../gh-pages/**` or `../wiki/**`, which are sibling checkouts this closure was told to leave alone.

### Deviations from Plan
- The three functional tests landed in `test/ui/`, not the `test/editor/` and `test/plugin/` paths the spec named. The `.et` editor harness completes CONFIG paths and reaches no command help; `ze help command --json` needs no daemon. A fourth file was added because `reject=stdout` matches the accumulated stdout of a whole `.ci`.
- `writeHelp`, `writeHelpLine` and `writeHelpEntry` were DELETED rather than fixed (owner approved 2026-08-31). They had no non-test caller; `commandHelpPage` is the page an operator reaches.
- AC-16's RPC count is 211, not the spec's 218. Seven of the 218 `rpc` statements are in `internal/test/plugins/fake*`, which no production build registers.
- `plan/learned/NNN-<name>.md` was NOT written. That directory holds three durable documents and no numbered file; `/ze-close` step 6a makes the closure artifact a journal row, which is what this closure wrote.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-6 assumed no command description was load-bearing for a non-help consumer | `placeholderValueCommands` (`cmd/ze/internal/cmdutil/cmdutil_test.go`) selected its sample by matching a value placeholder in the DESCRIPTION. Shortening the descriptions took the sample to 0 | `TestDescribedValueCommandsAcceptTheirValue` went red | The sample moved to `command.Usage`, the model producer of the invocation form. `declaredValueCommands`, floor 90, measured 95 commands / 104 verb forms |
| approach | The renderer defect was assumed fixed once Phase 3 landed, because nine gates were green | `commandHelpPage` had re-acquired the childless guard, and no gate runs the binary an operator types into | Read at the producer during the final phases, with the AC-4 unit test red and unread | Fixed at the producer; `test/ui/help-parent-node.ci` was written and forced RED by restoring the guard. Journal row in `plan/journal/green-that-could-not-have-been-red.md` |
| escalation | The help-shape gate was declared complete over the YANG tree and the RPCs | `ze help command --json` merges a THIRD corpus, the offline local registry, which the gate never walked | An unrefused two-sentence summary on `generate wireguard keypair` | Third surface added; row in `plan/journal/gate-excludes-part-of-its-population.md`, which also names `registry.ListRoot`'s 21 still-ungated root summaries |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Replace the guesses with one declaration: a short form and a long form | Done | `ze-extensions.yang` `extension help`; `command.go` `getHelpExtension`, `mergeHelpText` | Both fields reach `command.Node` and `yang.RPCMeta` |
| Every deleted guess is gone, not left as a fallback | Done | `grep -rn 'func Summary(' --include=*.go .` outside vendor returns nothing | Five guesses deleted |
| A parent node's own description reaches an operator | Done | `cmd/ze/command_help_page.go` `commandHelpPage` | Both fields set unconditionally |
| A multi-line description cannot corrupt the shell-completion format | Done | `internal/plugins/completion/words.go` `writeCompletionRecord` | Folds tab/newline/CR to single spaces |
| The web admin command form shows the command's help | Done | `internal/component/web/handler_admin.go` `buildAdminFragmentData` | `TestAdminCommandFormShowsHelp` |
| `llms.txt` carries the whole short form | Done | `internal/le/site/derived.go` `writeLLMSCommands` | `cleanInline`, no cut |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestMergeYANGEntryReadsHelpExtension`, `TestRPCDescriptionCarriesSummaryAndHelp` | Two fields, neither derived |
| AC-2 | Done | `TestHelpEntriesKeepTheWholeSummary`, `TestPageRendersEachDeclaredHelpTextWhole`, `TestCommandCatalogCarriesSummaryAndHelp` | |
| AC-3 | Done | `TestHelpShapeGateNamesTheBrokenRule` (5 subtests), `TestHelpShapeGateCapsTheSummaryAtItsBound` | Seven rules |
| AC-4 | Done | `TestHelpPageCarriesBothDeclaredHelpTexts`, `test/ui/help-parent-node.ci` | The `.ci` pins the order and was forced RED |
| AC-5 | Done (CLI half); web half unit-tested only | `TestCompleterSuggestsSummaryNotWholeDescription`, `TestShellCompletionRejectsNewlineInSummary`, `test/ui/completion-words-summary.ci`; `TestCLICompleteSendsSummaryNotExplanation` | Known Limitation: no functional test drives the HTTP endpoint |
| AC-6 | Done | `TestCommandCatalogCarriesSummaryAndHelp`, `test/ui/help-command-json-two-fields.ci`, `test/ui/help-command-json-summary-only.ci` | |
| AC-7 | Done | `TestPluginCommandDeclWithoutHelpKeepsSummary`, `TestPluginWithNoHelpDeclarationKeepsItsSummary` | Zero value is the refusal |
| AC-8 | Done | `TestAdminCommandFormShowsHelp`, `TestAdminCommandFormEscapesHelp` | |
| AC-9 | Done | `TestPublishedCommandRowUsesSummary`, `TestPublishedCommandDetailUsesBothForms`, `TestLLMSCommandLineCarriesWholeSummary` | Producers only; `../gh-pages` not written |
| AC-10 | Done | `TestWikiCatalogRendersDeclaredSummary` | Producers only; `../wiki` not written |
| AC-11 | Done | `TestMergeYANGEntryWarnsPerFieldOnMismatch` (3 subtests), `TestMergeCommandPathsDecidesEachHelpFieldOnItsOwn` | Warning names the field |
| AC-12 | Done | `TestHelpShapeGateNamesANodeWithNoSummary`, `TestHelpShapeGateRefusesAnEmptyTree` | Fails closed |
| AC-13 | Done | `TestSTEExtractsHelpExtension` | `yangDescriptionRe` matches `ze:help` |
| AC-14 | Done | `./le --name close docvalid help-shape` exit 0, 601 nodes / 211 RPCs / 19 locals, zero refusals | Long form on 310 nodes, 25 RPCs, 11 locals |
| AC-15 | Done | `grep -rn 'func Summary(' --include=*.go .` outside vendor: no hits. `writeHelp` also gone | |
| AC-16 | Done | `TestRPCDescriptionCarriesSummaryAndHelp`, `TestHelpShapeGateWalksRPCDescriptions` (7 subtests), `TestRegisterRPCsCarriesBothHelpTexts`, `TestReferenceRPCCarriesLongHelp` | 211 RPCs, not 218 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Every unit test in the TDD table | Done | as listed in the table | `TestHelpShapeGateNamesTheBrokenRule` and `TestEveryCommandNodeHasASummary` were RED by design until Phase 8; both green |
| `TestWriteHelpShowsParentNodeHelp` | Changed | deleted with `writeHelp` (Phase 8e) | Its order assertion moved to `TestHelpPageCarriesBothDeclaredHelpTexts` |
| `help-parent-node`, `help-command-json-two-fields`, `help-command-json-summary-only`, `completion-words-summary` | Done | `test/ui/*.ci` | All four green in `./le functional ui`; each forced RED before being claimed |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Every file in Files to Modify | Done | 95 Go/templ files, 111 `.yang` modules, 9 `register.go`, 24 docs pages |
| `internal/le/docvalid/helpshape.go` | Done | New, ~520 lines, three corpora |
| `plan/deferrals/yang-short-and-long-command-help.md` | Done | One live row, homed at `plan/future/spec-yang-config-leaf-short-and-long-help.md` |
| `test/editor/help-parent-node.et`, `test/plugin/help-command-json-two-fields.ci` | Changed | Landed as four `test/ui/*.ci`; neither planned harness reaches the producer |
| `../wiki/*`, `../gh-pages/*` | Skipped | Sibling checkouts. Producers changed and unit-tested; publication is the owner's to sequence |

### Audit Summary
- **Total items:** 16 AC + 6 requirements + 5 file groups = 27
- **Done:** 25
- **Partial:** 0
- **Skipped:** 1 (the sibling publications, deliberately out of scope for this closure)
- **Changed:** 3 (the functional test paths, `writeHelp` deleted rather than fixed, 211 RPCs not 218)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A short form every surface can put on one line | gate over the whole corpus | `./le --name close docvalid help-shape` exit 0: 601 command nodes, 211 RPCs, 19 offline local commands, every one a one-sentence summary, zero refusals. The gate's own rules are proven by twelve producer mutations, each turning its own test red |
| A long form the per-command help page prints | functional test through the binary | `test/ui/help-parent-node.ci`: `ze show bgp help` renders the summary on the header line and all three long-help lines in the body block. Forced RED by restoring the `len(node.Children) == 0` guard |
| Each surface reads the half it renders, and no renderer derives one from the other | functional test through the binary | `test/ui/help-command-json-two-fields.ci` (both keys, whole, `\n` preserved) and `test/ui/help-command-json-summary-only.ci` (the key is ABSENT, not empty). Forced RED by binding `long-help` to the summary in `collectCommands` |
| No newline can reach the shell-completion format | functional test through the binary | `test/ui/completion-words-summary.ci`, `reject=stdout:pattern=(?m)^[^\t\n]+$`. Forced RED by emitting a newline in a family hint with the fold removed from `writeCompletionRecord`; a CONTROL run with the fold restored passed |
| The four guesses are deleted, not left as fallbacks | grep over the tree | `grep -rn 'func Summary(' --include=*.go .` outside vendor returns nothing; `trimInline` survives with three callers, none of them a command description (`derived.go:266,317,498`) |
| A plugin compiled before this change keeps working | unit test over the real handshake | `TestPluginWithNoHelpDeclarationKeepsItsSummary` runs a real Stage 1 through `runPluginPhase` and reads `MergeCommandPaths` output. Forced RED by `VisibleCommandEntries` setting `Help: cmd.Description` |
| A defect in the model reaches nobody in silence | negative test | `TestHelpShapeGateRefusesAnEmptyTree`, `TestHelpShapeGateRefusesAModuleSetWithNoRPC`, and the zero-local refusal: a load that returned nothing is an error, never a green report |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| The short/long split for CONFIG nodes: 1241 leaf and leaf-list, 443 container and list, 277 enum descriptions, and the nine config renderers that guess a length | deferred | `plan/future/spec-yang-config-leaf-short-and-long-help.md`, written and in the tree at `skeleton`. The shard is NOT removed: it still holds this live row |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/yang-short-and-long-command-help-2af3e0ff-181e-4d65-af83-f641cac0551d.md` (217 files, verdict=clean) |
| `./le spec session review check` | clean |
| Rounds | 2 |
| Reviewer lenses used | wiring + functional-test coverage; documentation drift; removed-behavior and test-rewrite audit; logic and guard audit; security (declared-text bounds, per-declaration AND per-reply); allocation; simplicity and altitude; ze-go-style pass |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `MainPackageLocalCommands` exported with no cross-package non-test caller. `./le repository check` named it; both callers are in package `docvalid` | `internal/le/docvalid/helpshape.go:363` | Unexported to `mainPackageLocalCommands`; the doc comment's "it is exported because" paragraph rewritten. `./le repository check` no longer names it |
| 2 | ISSUE | The guide page describing the admin command form was silent on the help it now renders, while `./le doc wiring` named `HandleAdminView` under doc-drift. AC-8 is user-visible and its guide page did not say so | `docs/guide/web-interface.md:327` | Two sentences added under "Admin Commands", with a `buildAdminFragmentData` and a `commandForm` source anchor. `./le ste check` clean after one run-on was split |
| 3 | ISSUE | Twelve `[WEAKENED]` findings from `./le commit audit`, none of them explained | `cmd/ze/internal/cmdutil/cmdutil_test.go`, `internal/component/command/help_test.go`, `internal/component/web/handler_admin_test.go`, `handler_admin_yang_test.go`, `internal/core/helpfmt/helpfmt_test.go`, `internal/le/docvalid/actions_test.go` | Twelve rows written to `test/weakened.md`, each naming what the old assertion proved and where that proof lives now. Two deletions, one assertion reduction, nine renames |

### Run 2
Re-read the two fixes and everything they touched. `go vet -tags ze_core ./internal/le/docvalid/` exit 0, `gofmt -l` clean, `./le repository check` down to 9 findings and every one pre-existing at HEAD (verified with `git grep` at HEAD for `WriteOut`, the five `subsystem.go` exports and `CommandSchema`; the two `debug/register.go` `.ci` rows name commands this diff does not add). `go test ./internal/le/docvalid/` green except `TestEveryYANGCommandHasAHandler`, which is a `ze_core`-only build with the BGP handlers compiled out over modules another session holds mid-edit. `./le ste check file docs/guide/web-interface.md` exit 0. `./le spec citation` OK. 0 BLOCKER, 0 ISSUE.

### Notes recorded, not blocking
- `ze-plugin:session-ping` still declares `leaf pong { type boolean; }` while `handlePluginSessionPing` answers a pid. The rpc description was corrected to name the pid, so the module still disagrees with itself in two statements. NOT fixed: the repair is a wire change on the plugin IPC boundary. Row in `plan/journal/declared-format-contradicts-payload.md`.
- `getHelpExtension` matches `ze:help` or any `:help` suffix, so a module importing the extensions under a different prefix is read. `GetCommandExtension` has the same shape and is the precedent.
- `registry.ListRoot()`'s 21 root-command summaries reach `ze --help` and no gate reads one. Row in `plan/journal/gate-excludes-part-of-its-population.md`.
- `./le spec citation` WARNs that `vendor/.../entry.go:654` no longer shows the token `uses`. Line 654 IS `case *Uses:`; the extractor matched the wrong word.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/docvalid/helpshape.go` | Yes | `-rw-rw-r-- 24458 Aug 31 21:50 internal/le/docvalid/helpshape.go` |
| `plan/deferrals/yang-short-and-long-command-help.md` | Yes | `-rw-rw-r-- 1019 Aug 31 21:55` |
| `test/ui/help-parent-node.ci` | Yes | `-rw-rw-r-- 1462 Aug 31 19:01` |
| `test/ui/help-command-json-two-fields.ci` | Yes | `-rw-rw-r-- 1260 Aug 31 19:05` |
| `test/ui/help-command-json-summary-only.ci` | Yes | `-rw-rw-r-- 1146 Aug 31 19:05` |
| `test/ui/completion-words-summary.ci` | Yes | `-rw-rw-r-- 1944 Aug 31 19:42` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | `ze:help` is declared and read | `grep -n -A6 'extension help' internal/component/config/yang/modules/ze-extensions.yang` shows `extension help { argument text; ... }` at line 151 |
| AC-14 | Every node, rpc and local command carries a conformant summary | `./le --name close docvalid help-shape` exit 0: 601/601 nodes, 211/211 RPCs, 19/19 locals, long form on 310/25/11 |
| AC-15 | No `Summary` guess remains | `grep -rn 'func Summary(' --include=*.go .` outside vendor returns nothing |
| AC-9 | No character truncation of a command description | `grep -n trimInline internal/le/site/*.go`: three callers, at `derived.go:266` (config node), `:317` (plugin description), `:498` (module why). None is a command summary |
| AC-4 / AC-5 / AC-6 | Through the binary | `./le functional ui` ran all four `.ci` green in Phases 10 and 11; each was forced RED first, and the RED output is recorded in the per-spec session state |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze <path> help` on a node with children | `test/ui/help-parent-node.ci` | Yes -- read the file: `exec=ze show bgp help`, asserts the header line, all three body lines, a child row and one `(?sm)` order pattern |
| `ze help command <filter> --json` | `test/ui/help-command-json-two-fields.ci` | Yes -- asserts `description` and `long-help` as two whole values with `\n` preserved |
| The zero-value half of the same catalog | `test/ui/help-command-json-summary-only.ci` | Yes -- `reject=stdout:contains="long-help"`, one command in the file so the reject is meaningful |
| Tab and shell completion | `test/ui/completion-words-summary.ci` | Yes -- three `(?m)^name\tsummary$` patterns, two content rejects, one format reject refusing any line with no tab |
| A `-cmd.yang` node declaring `ze:help` | unit | `TestMergeYANGEntryReadsHelpExtension` parses a three-line argument and reads it back with its newlines |
| A plugin `CommandDecl` with and without the field | unit over the real handshake | `TestPluginHelpDeclarationReachesTheCommandTree`, `TestPluginWithNoHelpDeclarationKeepsItsSummary` |
| Web admin command form | unit over the HTTP entry point | `TestAdminCommandFormShowsHelp`, `TestAdminCommandFormEscapesHelp` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestMergeYANGEntryReadsHelpExtension` reads a three-line `ze:help` back off a parsed module, newlines intact |
| A-2 | confirmed | `TestPluginCommandDeclWithoutHelpKeepsSummary` decodes a `CommandDecl` payload with no `long-help` key; `TestPluginWithNoHelpDeclarationKeepsItsSummary` runs the whole Stage 1 handshake and reads the summary back off the merged tree |
| A-3 | confirmed | Every one of the 601 nodes, 211 RPCs and 19 locals now carries a summary and the gate exits 0. Two roots needed a subject-matter decision and got one from the module that owns them (`peer`, `request`); one, `show health`, was WRITTEN rather than split and its replacement is determined by the node's own next sentence |
| A-4 | confirmed | `./le ste check` reports no habit growth in any converted module. `yangDescriptionRe` matches `ze:help`, so prose that left `description` stayed in STE scope. One growth WAS introduced (two 26-word sentences in `ze-rib-api.yang`) and the ratchet caught it |
| A-5 | confirmed | `./le doc wiring` at closure: 3294 drift issues, and every issue line names `../gh-pages/**`. Zero name an in-repo producer, measured by filtering the run's own output |
| A-6 | broken, then repaired | `placeholderValueCommands` read the description as DATA. `declaredValueCommands` now samples `command.Usage`; 95 commands, 104 verb forms, floor 90. Mistake Log row above |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| 1. New user-facing feature | `docs/features.md` "Self-Documenting System" row states the two-field contract with three source anchors | Yes |
| 3. CLI command changed | `docs/guide/cli.md` and `docs/guide/command-reference.md` carry the two catalog keys and the shell record shape; all three of Phase 3's doc-drift rows cleared | Yes |
| 4. API/RPC changed | `docs/architecture/api/commands.md` "A command's two help texts" and "Which surface renders which field", read against `collectCommands` and `commandHelpPage` | Yes |
| 5. / 8. Plugin SDK and protocol | `docs/architecture/api/process-protocol.md:331-352` states both keys, both bounds (256 / 4096) and the zero-value reading, read against `validateHelpDecls`. `docs/plugin-development/protocol.md:191,192,227` and `commands.md:289,290` agree | Yes |
| 6. User guide | `docs/guide/cli.md`; `docs/guide/web-interface.md` gained the admin-form help paragraph in this closure | Yes |
| 10. Test infrastructure | `docs/contributing/documentation-testing.md` carries the `help-shape` row, its when-to-run row and its output block | Yes |
| 12. Internal architecture | `docs/architecture/config/yang-config-design.md` §2.2 lists all 31 extensions, §6 states the two-field contract and the rpc carrier | Yes |
| 15. Registered inventory | `docs/plugin-development/README.md` SDK type table names `LongHelp` on `sdk.CommandDecl`. `docs/plugin-overview.md` is N-A in fact: it carries no `commands[]` field list to make wrong | Yes |
| 2, 7, 11, 13, 14 | No config leaf, container or value changed; no wire format; no parity claim; no route metadata; no counters. `grep -rn 'source: internal/component/config/yang/command.go' docs/` names only the pages already edited | Yes (No applies) |
| 16. Source anchors | `./le --name close spec citation anchors spec plan/spec-yang-short-and-long-command-help.md`: 34 pages, 8 edited by earlier phases, 3 repaired (`plugin-development/README.md`, the `command-reference.md` anchor, `guide/web-interface.md`), the rest read and unaffected | Yes |

## Core Insight

A gate whose population is the MODEL proves nothing about the RENDERING. Nine gates read this spec's YANG declarations, its published artifacts and its prose, and all nine were green while the one shipped renderer an operator types into had the defect the spec exists to remove. The declaration and the rendering are two populations, and a gate over one is not evidence about the other. Every surface whose gate reads a model owes a second test that runs the binary.
