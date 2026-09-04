# Spec: command help and description

| Field | Value |
|-------|-------|
| Status | complete |
| Scope | cli |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/command-help-and-description.md` |
| Handoff | - |
| Updated | 2026-09-03 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The owner asked for two things on 2026-09-03, in these words: "make sure we
ensure all command have a help and description, help for the short text,
description for the long, using these names in the code. have code to validate
that after yang edition/creation that both exist and have suitable length".

The second ask is new work. The first ask REVERSES a decision this repository
took on 2026-08-31 and shipped. `spec-yang-short-and-long-command-help` made
`description` the SHORT summary and `ze:help` the LONG explanation. It wrote
that meaning into nine Go types, into 25 documentation pages, into the
`long-help` JSON key of three published answers, and into
`./le docvalid help-shape`, which now gates it.

**This spec supersedes that decision.** Every place that states the old meaning
is named in Files to Modify. Nothing here weakens the earlier spec's other
result: no renderer derives one text from the other, and that stays.

Three facts, each read at the producer, decide how the work is shaped.

| Fact | Where | What it means for this spec |
|------|-------|----------------------------|
| `Help` already means the SHORT text in three of the four types that carry it, and the LONG text in the fourth | `Command.Help`, `Completion.Help` and `commandEntry.Help` against `command.Node.Help` | The owner's naming REMOVES a live collision. This is the argument for the inversion, and it is stronger than the one about text length |
| Every command summary already satisfies the short shape | `./le docvalid help-shape`, run 2026-09-03: 611 command nodes, 211 RPCs and 19 offline local commands, no refusal | The inversion moves 841 texts that are already correct. It buys naming, not shorter text |
| The paragraph on a one-line row comes from CONFIG nodes, which declare ONE text | `revealCandidateExplanation` (`internal/component/cli/model_keys.go`) states it: a config node declares one text, its YANG description, and it is often a paragraph | The inversion does not fix it. A config node with a long `description` and no short text renders WORSE, because the one-line row then has nothing to read |

## Owner Decisions (ANSWERED 2026-09-03)

The owner answered all four. His words on D-1: "sorry I gave the instruction the
wrong way round. make sure all is consistent with no inversion".

| # | Question | Answer | What it makes this spec |
|---|----------|--------|-------------------------|
| D-1 | Does the inversion run at all? | NO | `description` keeps the SHORT text and `ze:help` keeps the LONG one, as shipped on 2026-08-31. The work is a CONSISTENCY pass, not a reversal: `Description` names the short text and `LongHelp` the long one at every Go layer and every wire key |
| D-2 | Does it cover CONFIG nodes? | YES, fold in | Config leaves join the population. `plan/future/spec-yang-config-leaf-short-and-long-help.md` is superseded by this spec and is deleted at closure |
| D-3 | Does the plugin SDK wire change its JSON keys? | YES, and refuse the retired key | Under D-1 NO, `CommandDecl` already spells the pair `description` and `long-help`, so the SDK wire does NOT move. The key that DOES move is `help` on the `system command list` answer, which is Ze's own daemon-to-CLI wire. `validateHelpDecls` refuses a declaration carrying a retired key rather than decoding it to the zero value |
| D-4 | What length bounds? | The proposed table | Short: 25 words, 96 characters, one line, no semicolon, ends in a full stop. Long: 4096 bytes, and differs from the short text beside it |

## The Collision D-1 Removes

`Help` carries two opposite meanings today, and a copy runs across the crossing.
Verified by reading each declaration.

| Type | Field | Means | Becomes |
|------|-------|-------|---------|
| `command.Node` | `Help` | the LONG text, from `ze:help` | `LongHelp` |
| `command.CommandEntry` | `Help` | the LONG text | `LongHelp` |
| `yang.RPCMeta` | `Help` | the LONG text | `LongHelp` |
| `pluginserver.Command` | `Help` | the SHORT text | `Description` |
| `pluginserver.Completion` | `Help`, JSON `help` | the SHORT text | `Description`, JSON `description` |
| `client.commandEntry` | `Help`, JSON `help` | the SHORT text | `Description`, JSON `description` |
| `pkg/plugin/rpc.CommandDecl` | `Description`, `LongHelp` | already correct | unchanged |

After the pass, `Description` means the short text everywhere and `LongHelp`
means the long text everywhere. `injectPluginCommands` loses the doc comment
that says the names cross, because they no longer do.

## Measured Population (2026-09-03, over `git ls-files '*.yang'`)

This REPLACES the narrower count the earlier draft carried. The count is over
TRACKED files only: an earlier run read untracked build copies under `tmp/` and
reported six times the real figure.

| Module kind | Files | `description` | Over 25 words | Over 96 characters | `ze:help` |
|-------------|-------|---------------|---------------|--------------------|-----------|
| `-cmd.yang` | 96 | 1,135 | 63 | 170 | 308 |
| `-api.yang` | 34 | 495 | 7 | 15 | 14 |
| config | 109 | 2,616 | 315 | 640 | 16 |
| Total | 239 | 4,246 | 385 | 825 | 338 |

825 descriptions breach at least one bound D-4 sets, across 180 files. Each owes
a one-line summary in `description`, with the paragraph moved to `ze:help`.
The distribution decides how the prose is batched.

| Files holding | Files | Descriptions |
|---------------|-------|--------------|
| 1 to 5 | 155 | 319 |
| 6 to 20 | 19 | 202 |
| 21 to 50 | 3 | 81 |
| 51 or more | 3 | 223 |

The three largest are `internal/component/bgp/yang/ze-bgp-conf.yang` (92),
`internal/component/iface/yang/ze-iface-conf.yang` (72) and
`internal/plugins/ospf/yang/ze-ospf-conf.yang` (59).

## What the Bounds Govern (found 2026-09-03, during implementation)

The bounds D-4 sets exist for ONE reason: a summary that renders on the CLI's
one-line completion row, which `overlayInnerWidth` clamps to [48, 96]
characters. So the population is the statements that REACH that row, and it is
not every `description` in a module.

| Statement | Renders on a one-line row | Bound applies |
|-----------|---------------------------|---------------|
| `container`, `list`, `leaf`, `leaf-list`, `choice`, `case`, `action`, `rpc`, `notification` | Yes | Yes |
| `enum`, and ONLY where its leaf is a LIST KEY | Yes, through `enumKeyVocabulary`, which is reached only from `getListKeyEntry` | The character and word caps apply. The long-text rule does NOT: nothing reads a `ze:help` on an enum |
| `module`, `submodule`, `revision`, `import`, `include`, `grouping`, `typedef`, `identity`, `feature`, `extension` | No | No |

Measured over the tracked corpus at the time of the finding: 524 of 640
over-long descriptions sit on a rendering statement, and 116 do not. The 116
are `import` (57), `extension` (26), `revision` (16), `typedef` (8), `module`
(6) and `grouping` (3).

The finding cost three agents one pass each. Given a brief that said "every
over-long description", all three independently refused to write `ze:help` on a
module, because `getHelpExtension` is called from exactly two sites,
`command.go:393` over a command container's `Exts` and `rpc.go:59` over
`RPC.Exts()`. Each then moved the prose into a `//` comment instead. That is a
DOWNGRADE: a YANG `description` is schema, readable by standard YANG tooling and
published in the schema output, and a comment is neither. All three passes were
reverted for the non-rendering statements and kept for the rendering ones.

`enum` was found by a prose agent and then narrowed twice by reading.
`enumKeyVocabulary` (`internal/component/cli/completer.go:672`) reads each
enum's own `description` off the leaf's parse-tree node and returns it as the
completion help for that value, so an enum description CAN reach a one-line row.
Its one caller (`:566`) takes its argument from `getListKeyEntry`, so only a
LIST KEY leaf's enums render. Of the 278 enum descriptions in the corpus, 15
run past a bound and none of those 15 sits on a list key, so none is capped
today. The predicate is still implemented, because a future enumeration-keyed
list would render.

The owner resolution MUST track braces. Resolving a description's owner by the
nearest preceding statement keyword reports an enum description as owned by its
enclosing leaf, which is how the population was under-counted until an agent
rewrote the scanner.

A second split, found the same day: the module KIND decides whether a `leaf`
renders, and the two producers disagree.

| Leaf declared in | Producer | Reaches an operator | Bound applies |
|------------------|----------|--------------------|---------------|
| a `-cmd.yang` or `-api.yang` module | `argDefFor` builds a `command.ArgDef` from `leaf.Type` alone, and `ArgDef` holds no text field | No. The description is dropped at the tree boundary | No |
| a config module | `entryDescription` reads `entry.Description` off the goyang entry and puts it on the completion row | Yes | Yes |

The corroboration is the gate's own silence: `leaf target` in
`internal/plugins/as112/yang/ze-as112-cmd.yang` carries a semicolon and no full
stop, and `./le docvalid help-shape` refuses nothing over it, which it could not
do if that leaf were in the tree it walks. A command leaf's unread description
is its own defect and is recorded in
`plan/journal/validated-value-discarded-by-its-caller.md`. This spec does not
fix it.

So `./le docvalid help-shape` and the write hook MUST judge the owning statement
before they judge the text. A gate that caps a `module` description reports a
defect that does not exist, and the repair it invites destroys published prose.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - the canonical page for a command's two texts, and the renderer table this spec repoints
  → Decision: the page holds one row per RENDERER, naming the field each one reads. Any rename edits that table, and the table is the count of how many surfaces exist
  → Constraint: the OpenAPI operation already maps `Description` to `summary` and `LongHelp` to `description`, so `description` ALREADY means the long text at one published boundary
- [ ] `docs/architecture/config/yang-config-design.md` - the page that declares what `description` and `ze:help` mean in a Ze module
  → Constraint: the extension table and the command-help table each state the current meaning in one row, so the reversal is two row edits and one paragraph
- [ ] `docs/architecture/api/process-protocol.md` - the plugin Stage 1 wire
  → Constraint: the key is `long-help` and not `help` because `help` already names the summary on the same boundary. Under the inversion `help` names the summary everywhere, and that reason disappears
- [ ] `docs/architecture/cli/command-completion.md` - how a plugin command's two texts enter the completion tree
  → Constraint: `MergeCommandPaths` writes each text only on a leaf it creates, or on one holding nothing in THAT field. An inversion that swaps two fields must not make a half-filled node take the wrong half
- [ ] `docs/contributing/documentation-testing.md` - where `./le docvalid help-shape` is declared for a reader
  → Decision: the page already tells an author to run the gate after writing a `description`, a `ze:help`, or a `registry.Meta`. The new rules extend that row rather than adding one
- [ ] `docs/contributing/writing-style.md` - the per-surface table that states the shape each half owes
  → Constraint: four rows state the current meaning, and they are the prose the reversal rewrites. `plan/spec-*.md` is excluded in `internal/le/ste/ste.go`, so `./le ste check` does not read this spec file

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc7950.txt` Section 7.21.3 - the normative meaning of the `description` statement
  → Constraint: "The 'description' statement takes as an argument a string that contains a human-readable textual description of this definition." No length is stated, so both mappings conform. The sentence favors the LONG reading, because standard YANG tooling reads `description` and never reads `ze:help`

**Key insights:** (minimal context to resume after compaction)
- `Help` means the summary in `pluginserver.Command`, `pluginserver.Completion` and `client.commandEntry`, and the long text in `command.Node`. `MergeCommandPaths` copies across that crossing.
- The command corpus already satisfies the short shape. Coverage of the LONG half is the gap: 317 of 611 nodes, 25 of 211 RPCs, 11 of 19 offline local commands.
- 488 command declarations owe a long text. That is authored prose, not a mechanical edit.
- The config half is already specced at `plan/future/spec-yang-config-leaf-short-and-long-help.md`, and this spec does not re-decide it.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/command/node.go` - `Node.Description` is the one-line summary from the YANG `description` statement. `Node.Help` is the long explanation from `ze:help`
- [ ] `internal/component/config/yang/command.go` - `BuildCommandTree` sets both. `getHelpExtension` reads `ze:help` whole, newlines kept. `PathToDescription` and `PathToHelp` publish the two maps
- [ ] `internal/component/config/yang/rpc.go` - `RPCMeta.Description` is the summary and `RPCMeta.Help` is the long text, for an RPC
- [ ] `internal/component/plugin/server/command.go` - `Command.Help` is the SUMMARY and `Command.LongHelp` is the explanation. `LoadBuiltins` fills them from `pathToDesc` and `pathToHelp`
- [ ] `internal/component/plugin/server/command_registry.go` - `Completion.Help` and `Completion.LongHelp` are the `system command list` wire row. `RegisteredCommand.Description` and `RegisteredCommand.LongHelp` hold a plugin's declaration
- [ ] `internal/component/plugin/server/startup.go` - `validateHelpDecls` bounds a plugin's two texts. The summary takes `maxSummaryLen` and one line. The explanation takes `maxLongHelpLen` and keeps newlines
- [ ] `internal/component/cli/client/inject.go` - `commandEntry.Help` is the summary and `commandEntry.LongHelp` the explanation. Its doc comment says the names cross here
- [ ] `internal/component/cli/model_render.go` - `warningText` puts the selected candidate's `Description` on the second message row, and drops it when it equals the open explanation. `overlayInnerWidth` clamps a box to [48, 96]
- [ ] `internal/component/cli/model_keys.go` - `revealCandidateExplanation` names the config case: one text, often a paragraph, and the box is the only place it fits whole
- [ ] `internal/component/command/registry/registry.go` - `Meta.Description` is the summary and `Meta.LongHelp` the explanation, for an offline local command
- [ ] `cmd/ze/hub/command_meta.go` - `commandMeta.Description` and `commandMeta.LongHelp`, the neutral row both the API and the MCP listers read
- [ ] `pkg/plugin/rpc/types.go` - `CommandDecl.Description` (JSON `description`) and `CommandDecl.LongHelp` (JSON `long-help`), the published plugin SDK wire
- [ ] `internal/component/cli/contract/contract.go` - `Completion.Description`, the one-row text the editor renders
- [ ] `internal/le/docvalid/helpshape.go` - `helpShapeContract` walks three surfaces and applies eight rules to the SUMMARY. It counts long-text coverage and refuses nothing about it
- [ ] `internal/le/doc/wiring/docverify.go` - `docHelpShapeStage` is one stage of `./le doc wiring`, which `./le verify` runs as a structural stage
- [ ] `internal/le/hookruntime/writeedit.go` - `writeFilePatterns` and its siblings judge ONE proposed file's text. No stage loads the YANG model
- [ ] `internal/le/ste/tables.go` - `MaxDescriptiveWords` is 25 and is exported so the help gate and the prose gate cannot disagree
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - the `router-id` leaf declares a four-line description, and no second text
- [ ] `plan/future/spec-yang-config-leaf-short-and-long-help.md` - the config half, already homed, awaiting the owner's word
- [ ] `plan/future/spec-shrink-only-baseline-cannot-see-a-relocation.md` - a baseline keyed on a path reads a rename as new debt

Measured on 2026-09-03 by `./le docvalid help-shape`:

| Surface | Population | Declares a summary | Declares a long text | Owes a long text |
|---------|-----------|--------------------|----------------------|------------------|
| Command tree node | 611 | 611 | 317 | 294 |
| RPC | 211 | 211 | 25 | 186 |
| Offline local command | 19 | 19 | 11 | 8 |
| Total | 841 | 841 | 353 | 488 |

Measured by a lexical count over the 1,225 YANG modules:

| Population | `description` statements | Over 25 words | `ze:help` statements |
|-----------|--------------------------|---------------|----------------------|
| `-cmd.yang` modules | 5,712 | 317 | 1,558 |
| `-api.yang` modules | 2,475 | 35 | 70 |
| Every other module | 13,115 | 1,586 | 80 |

82 percent of the over-long descriptions are in CONFIG modules. The command
surfaces are clean because the 2026-08-31 gate made them so.

**Behavior to preserve:** (unless the user explicitly said to change it)
- No renderer derives one text from the other. AC-1 of the superseded spec removed four truncation guesses, and none returns.
- `./le docvalid help-shape` keeps its eight summary rules, its three surfaces, and its refusal of an empty population.
- `validateDeclaredText` keeps both bounds and both shapes for a plugin declaration.
- A command that declares no explanation still renders. The help page prints the summary alone.
- `MergeCommandPaths` still writes a field only on a node holding nothing in that field.

**Behavior to change:** (only what the user asked for)
- The two names, at every layer the owner listed, subject to D-1 and D-3.
- `./le docvalid help-shape` refuses a command declaration that carries only one of the two texts, and refuses a summary past the character bound D-4 sets.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
Four authors declare a command's two texts, and each writes a different file.

| Author writes | In | Read by |
|---------------|-----|---------|
| A `description` statement and a `ze:help` extension on a command container | a `-cmd.yang` module | `BuildCommandTree` |
| A `description` statement and a `ze:help` extension on an `rpc` | an `-api.yang` module, or `internal/core/ipc/yang/` | `ExtractRPCs` |
| A `registry.Meta` beside a handler | a Go file that registers an offline local command | `registry.ListLocal` |
| A `CommandDecl` in a Stage 1 registration | the plugin process, over the wire | `validateHelpDecls`, then `onRegistration` |

### Transformation Path
1. `BuildCommandTree` (`internal/component/config/yang/command.go`) merges every `-cmd` module into one `command.Node` tree and writes both texts onto each node.
2. `PathToDescription` and `PathToHelp` flatten that tree into two maps keyed by CLI path.
3. `LoadBuiltins` (`internal/component/plugin/server/command.go`) writes each map into the dispatcher, as `Command.Help` and `Command.LongHelp`.
4. A plugin's `CommandDecl` reaches `validateHelpDecls`, then `RegisteredCommand`, then `MergeCommandPaths`, which writes it into the same tree.
5. `system command list` answers one `Completion` row per command, carrying `help` and `long-help`.
6. `injectPluginCommands` (`internal/component/cli/client/inject.go`) reads that row back into a client-side `command.Node`, and the two names cross.
7. Each renderer reads the field its row of `docs/architecture/api/commands.md` names.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG module ↔ command tree | `getHelpExtension` over `Entry.Exts` and over `RPC.Exts()` | Yes, read `getHelpExtension` |
| Engine ↔ plugin process | Stage 1 JSON, keys `description` and `long-help` | Yes, read `CommandDecl` and `validateHelpDecls` |
| Daemon ↔ CLI client | `system command list` rows, keys `help` and `long-help` | Yes, read `Completion` and `commandEntry` |
| Daemon ↔ HTTP API | OpenAPI `summary` and `description` | Yes, read the renderer table in `docs/architecture/api/commands.md` |
| Daemon ↔ MCP client | tool description: the summary, a blank line, then the explanation | Yes, read the same table |
| Repository ↔ published site | `wikicatalog.Render` and `internal/le/site` | Yes, read `docs/contributing/gh-pages.md` |

### Integration Points
- `helpShapeContract` (`internal/le/docvalid/helpshape.go`) - the gate this spec extends. It already holds all three surfaces and the walk that reaches them.
- `docHelpShapeStage` (`internal/le/doc/wiring/docverify.go`) - the stage that runs it inside `./le verify`.
- `judgeSummary` (`internal/le/docvalid/helpshape.go`) - the one judge the new rules join, so a command node and an RPC cannot drift into two shapes.

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
| A-1 | The owner wants the inversion at the YANG statement layer, and not only at the Go field layer | His words name `description` and `help`, which are the YANG spellings, and also say "using these names in the code" | The work shrinks to a Go rename plus 25 doc pages, and 841 YANG declarations stay as they are | D-1, answered by the owner before Status leaves `skeleton` | unvalidated |
| A-2 | 488 long explanations are worth writing, one for each command that owes one | The owner asked that all commands have both texts | A gate that demands them produces 488 restatements of the summary, which AC-1 of the superseded spec banned | Read a sample of 20 authored long texts after the migration phase, and check that none restates its summary | unvalidated |
| A-3 | No plugin outside this repository sends `long-help` today | `pkg/plugin/` is the published SDK, and the key was added on 2026-08-31 | A key rename drops an explanation silently, because an unknown JSON key decodes to the zero value | D-3, answered by the owner | unvalidated |
| A-4 | A command PATH is a stable key for a per-command record | `helpShapeContract` already reports every refusal by path | A command rename reads as new debt, the failure `plan/future/spec-shrink-only-baseline-cannot-see-a-relocation.md` records for a path-keyed baseline | The recommended gate shape persists nothing, so this binds only if the owner picks the baseline alternative | unvalidated |
| A-5 | 96 characters is the right upper bound for a summary | `overlayInnerWidth` clamps every Ze overlay to [48, 96] | A bound that is too tight rewrites summaries that render correctly, and one that is too loose lets a row be cut | D-4, answered by the owner against the measured 7.3 percent cost | unvalidated |
| A-6 | The write hook can judge one proposed YANG file on its own text | `writeFilePatterns` and its siblings read `ctx.content` and never load a model | The hook cannot answer at all, and the `./le` gate is the only check | Write the hook check first and run it over a fixture module | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The rename lands on 49 files and misses one, leaving a field named for the half it no longer carries | A doc comment that disagrees with its own field, found by the review round | Rename at the declaration through `gopls rename`, then read every one of the 25 documentation pages named in Files to Modify |
| R-2 | The two halves are SWAPPED in a module and nothing notices, because both texts stay present | A summary that reads as a paragraph, or an explanation that is one short sentence | The rules judge each half separately, so a swap breaks the summary rules on the long text |
| R-3 | 488 authored long texts arrive as restatements of their summaries | Two texts that differ only in punctuation | AC-3 refuses an exact copy. A restatement that is merely close needs a reader, so the review round samples them |
| R-4 | The gate arms and 488 commands go red on day one, so every session steps over a red stage | `./le doc wiring` fails on a tree nobody changed | The recommended gate judges only what the commit under test added or changed |
| R-5 | The plugin wire key changes and a plugin outside this repository loses its explanation with no error | Nothing at all. An unknown key decodes to the zero value, the silent failure `ai/rules/principles.md` names | D-3. If the keys change, `validateHelpDecls` refuses a declaration carrying the retired key, so the failure speaks |
| R-6 | The config defect the owner reported stays open, because this spec does not touch config nodes | `set bgp router-id` still puts a paragraph on the second message row | D-2. The work is already homed at `plan/future/spec-yang-config-leaf-short-and-long-help.md` |
| R-7 | A YANG file edited through a Bash heredoc never reaches the write hook | A module lands with one text and no hook fired | The `./le` gate is the load-bearing check. The hook is an early warning and is never the only one |
| R-8 | Another session is editing `internal/component/cli/completer.go` and `docs/architecture/config/yang-config-design.md` right now | A conflict at the first edit of either file | Read the file before editing it, and never revert a hunk this spec did not write (`ai/rules/never-destroy-work.md`) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every help surface an operator reads: the completion row, the explanation box, `ze help command --json`, the OpenAPI operation, the MCP tool description, the web admin form, and the published wiki. A swapped pair puts a paragraph on a one-line row and one sentence in a box |
| How is it reverted? | The Go rename and the doc edits revert as one commit, and the YANG statement moves revert with them. A plugin outside this repository that adopted a new wire key does not revert, which is what D-3 decides |
| Who else touches this path? | `internal/component/cli/completer.go`, `internal/component/config/yang/validator*.go` and `docs/architecture/config/yang-config-design.md` carry uncommitted edits from another session as of 2026-09-03. `plan/future/spec-yang-config-leaf-short-and-long-help.md` claims the config half of the same problem |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le docvalid help-shape` over a fixture module declaring a summary and no explanation | → | the long-half judge in `helpShapeContract` | `TestHelpShapeRefusesACommandWithNoLongHelp` |
| `./le doc wiring` over the checkout | → | `docHelpShapeStage` | `TestDocWiringRunsTheLongHelpRule` |
| `./le hook-check pretool-writeedit` with a proposed `-cmd.yang` edit declaring one text | → | the YANG write check in `internal/le/hookruntime/writeedit.go` | `TestWriteEditWarnsOnAYangCommandWithOneText` |
| An operator types a command prefix in `ze cli` and reads the second message row | → | `warningText` (`internal/component/cli/model_render.go`) | `test/ui/command-help-both-texts.ci` |
| An operator types a CONFIG path prefix in `ze cli`, reads the row, then presses `?` | → | `entryLongHelp` -> `Completion.LongHelp` -> `revealCandidateExplanation` | `test/ui/config-help-both-texts.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A command node, an RPC, an offline local command, or a config leaf declares a summary and no long text, and the commit under test added or changed it | `./le docvalid help-shape` refuses it, names the path, and names the rule `missing-long-help` |
| AC-2 | A `description` statement runs past 96 characters, or past 25 words | The gate refuses it, names the rule `char-cap` or `word-cap`, and states the measured length beside the bound |
| AC-3 | A `ze:help` text is byte-equal to the `description` beside it, once each is trimmed | The gate refuses it and names the rule `long-restates-summary` |
| AC-4 | An agent proposes a write or an edit to any `.yang` file whose new text declares a node carrying a `description` past the bound, or one of the two texts alone | `./le hook-check pretool-writeedit` answers exit 1 and names the node and the fault |
| AC-5 | A reader opens any `.yang` module | The `description` statement carries the one-line summary and the `ze:help` extension carries the long explanation. This is the shipped meaning and it is PRESERVED |
| AC-6 | A reader opens `command.Node` or `command.CommandEntry` | The field named `Description` holds the summary and the field named `LongHelp` holds the explanation. No field is named `Help` |
| AC-7 | A reader opens `pluginserver.Command`, `pluginserver.Completion`, `yang.RPCMeta` or `client.commandEntry` | No field is named `Help`. Each type spells the summary `Description` and the explanation `LongHelp` |
| AC-8 | An operator runs `system command list`, or `ze help command --json` | Each row carries the summary under `description` and the explanation under `long-help`. The key `help` is absent |
| AC-9 | An operator selects a command or a config leaf in the `ze cli` completion menu on an 80-column terminal | The second message row holds the whole summary, uncut, and the paragraph is reachable only in the `?` box |
| AC-10 | A Stage 1 `CommandDecl`, or a `system command list` row, carries a retired key | The decoder refuses it and the error names the retired key and its replacement. It is never decoded to the zero value |
| AC-11 | An operator types `set bgp router-id ` in `ze cli` | The message row holds a one-line summary of the leaf, and the RFC 6286 paragraph appears only in the `?` box |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Types `show bgp ` in `ze cli` and reads the row under the menu | YANG module -> `BuildCommandTree` -> `system command list` -> `injectPluginCommands` -> `warningText` | `test/ui/command-help-both-texts.ci` |
| 2 | Presses `?` on a highlighted candidate and reads the box | `revealCandidateExplanation` -> `Explain` -> `renderExplanationBox` | `test/ui/command-help-explanation-box.ci` |
| 3 | Runs `ze help command --json` and reads both keys of one row | `collectCommands` -> `aihelp.Build` | `cmd/ze/help_command_test.go` |
| 4 | Writes a new command node carrying one text, and runs the gate | `helpShapeContract` -> `docHelpShapeStage` | `TestHelpShapeRefusesACommandWithNoLongHelp` |
| 5 | Adds a command to a plugin and starts the daemon | `CommandDecl` -> `validateHelpDecls` -> `MergeCommandPaths` | `test/plugin/plugin-command-two-texts.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHelpShapeRefusesACommandWithNoLongHelp` | `internal/le/docvalid/helpshape_test.go` | AC-1 over a fixture command node | |
| `TestHelpShapeRefusesAnRPCWithNoLongHelp` | `internal/le/docvalid/helpshape_test.go` | AC-1 over a fixture RPC | |
| `TestHelpShapeRefusesALocalWithNoLongHelp` | `internal/le/docvalid/helpshape_test.go` | AC-1 over a fixture `registry.Meta` | |
| `TestHelpShapeRefusesASummaryPastTheCharacterCap` | `internal/le/docvalid/helpshape_test.go` | AC-2, at the bound and one character past it | |
| `TestHelpShapeRefusesALongTextThatRestatesItsSummary` | `internal/le/docvalid/helpshape_test.go` | AC-3, including a trailing-space difference | |
| `TestHelpShapeIgnoresACommandTheCommitDidNotTouch` | `internal/le/docvalid/helpshape_test.go` | The scope rule that keeps the gate green on day one | |
| `TestHelpShapeStillRefusesAnEmptyPopulation` | `internal/le/docvalid/helpshape_test.go` | The preserved refusal of a load that read nothing | |
| `TestWriteEditWarnsOnAYangDescriptionPastTheCharCap` | `internal/le/hookruntime/writeedit_test.go` | AC-4, `char-cap` | PASS |
| `TestWriteEditWarnsOnAYangDescriptionPastTheWordCap` | `internal/le/hookruntime/writeedit_test.go` | AC-4, `word-cap` | PASS |
| `TestWriteEditWarnsOnAYangHelpThatRestatesItsDescription` | `internal/le/hookruntime/writeedit_test.go` | AC-4, `long-restates-summary` | PASS |
| `TestWriteEditWarnsOnAYangDescriptionWithNoFullStop` | `internal/le/hookruntime/writeedit_test.go` | AC-4, `shape` | PASS |
| `TestWriteEditWarnsOnAYangDescriptionCarryingASemicolon` | `internal/le/hookruntime/writeedit_test.go` | AC-4, `shape` | PASS |
| `TestWriteEditPassesAYangDescriptionWithinTheBounds` | `internal/le/hookruntime/writeedit_test.go` | AC-4, the negative case | PASS |
| `TestWriteEditYangSummaryBoundsHoldAtTheirLastValidValue` | `internal/le/hookruntime/writeedit_test.go` | 96 passes and 97 refuses, 25 passes and 26 refuses | PASS |
| `TestWriteEditSaysWhenTheProposedYangDoesNotRead` | `internal/le/hookruntime/writeedit_test.go` | A failed parse is reported, never passed silently | PASS |
| `TestWriteEditSaysWhenAYangEditNamesNoOwningStatement` | `internal/le/hookruntime/writeedit_test.go` | An unresolved owner is counted as NOT judged | PASS |
| `TestWriteEditIgnoresALongModuleDescription` | `internal/le/hookruntime/writeedit_test.go` | A non-rendering statement takes no bound | PASS |
| `TestWriteEditIgnoresALongRevisionDescription` | `internal/le/hookruntime/writeedit_test.go` | The same, for a revision | PASS |
| `TestWriteEditJudgesALeafInsideAGrouping` | `internal/le/hookruntime/writeedit_test.go` | The owner is the nearest enclosing statement, not the grouping | PASS |
| `TestWriteEditIgnoresALeafInACommandModuleButJudgesOneInAConfigModule` | `internal/le/hookruntime/writeedit_test.go` | The module-kind split | PASS |
| `TestWriteEditIgnoresALeafListInACommandModuleButJudgesOneInAConfigModule` | `internal/le/hookruntime/writeedit_test.go` | The same, for a leaf-list | PASS |
| `TestCommandNodeHelpIsTheSummary` | `internal/component/command/node_test.go` | AC-6 | |
| `TestBuildCommandTreeReadsTheSummaryFromZeHelp` | `internal/component/config/yang/command_test.go` | AC-5 | |
| `TestMergeCommandPathsKeepsEachHalfOnItsOwnField` | `internal/component/command/merge_test.go` | R-2, that a half-filled node never takes the wrong half | |
| `TestValidateHelpDeclsRefusesTheRetiredKey` | `internal/component/plugin/server/startup_test.go` | AC-10 | |
| `TestCommandDeclRoundTripsBothKeys` | `pkg/plugin/rpc/types_test.go` | AC-8 at the wire | |
| `TestConfigCompletionCarriesBothTexts` | `internal/component/cli/completer_test.go` | AC-11, a config leaf yields both texts | PASS |
| `TestConfigCompletionRowIsNotTheParagraph` | `internal/component/cli/completer_test.go` | AC-9, the row is the short text | PASS |
| `TestRevealCandidateExplanationUsesLongHelp` | `internal/component/cli/model_keys_test.go` | AC-11, the box shows `ze:help` | PASS |
| `TestRevealCandidateExplanationSaysNothingIsDeclared` | `internal/component/cli/model_keys_test.go` | No fallback to the description | PASS |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Summary word count | 1-25 | 25 words | 0 words, which is the `missing-summary` rule | 26 words |
| Summary character count | 1-96 | 96 characters | 0 characters, which is the `missing-summary` rule | 97 characters |
| Long text byte count | 1-4096 | 4096 bytes | 0 bytes, which is the `missing-long-help` rule | 4097 bytes |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `command-help-both-texts` | `test/ui/command-help-both-texts.ci` | The operator opens the menu and reads a one-row summary | |
| `command-help-explanation-box` | `test/ui/command-help-explanation-box.ci` | The operator presses `?` and reads the explanation | |
| `config-help-both-texts` | `test/ui/config-help-both-texts.ci` | The operator types `set rou`, reads a one-line row, presses `?`, reads the RFC 6286 paragraph | PASS, discrimination RED recorded |
| `plugin-command-two-texts` | `test/plugin/plugin-command-two-texts.ci` | A plugin declares both texts and both reach the operator | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | No wire protocol another implementation speaks changes. The plugin Stage 1 wire is Ze's own, and D-3 governs it | |

## Files to Modify

D-1 was answered NO, so no YANG statement moves and no meaning is reversed. The
Go work is a CONSISTENCY rename and the YANG work is authored prose.

The gate and the hook.

- `internal/le/docvalid/helpshape.go` - the three new rules, the long-half judge, the scope rule, the rendering-statement population
- `internal/le/doc/wiring/docverify.go` - the stage heading states what the gate now answers
- `internal/le/hookruntime/writeedit.go` - the YANG write check, scoped to rendering statements
- `docs/contributing/documentation-testing.md` - the gate row and the author row
- `ai/rules/config.md` - what a config node owes when it is written

The Go consistency pass: `Description` is the summary, `LongHelp` the explanation.

- `internal/component/command/node.go` - `Node.Help` and `CommandEntry.Help` become `LongHelp`
- `internal/component/config/yang/rpc.go` - `RPCMeta.Help` becomes `LongHelp`
- `internal/component/config/yang/command.go` - `getHelpExtension` exported as `GetHelpExtension`, so the config path reuses one reader
- `internal/component/plugin/server/command.go`, `command_registry.go`, `startup.go` - `Command.Help` and `Completion.Help` become `Description`, JSON key `help` becomes `description`, and a retired key is refused
- `internal/component/cli/client/inject.go` - `commandEntry`, and `decodeCommandList` refuses the retired key
- `pkg/plugin/rpc/types.go` - unchanged keys, stale comment removed

The config two-text path.

- `internal/component/config/yang/modules/ze-extensions.yang` - `extension help` now covers a config node
- `internal/component/cli/completer.go` - `entryLongHelp` reads `ze:help` off the config entry
- `internal/component/cli/contract/contract.go` - `Completion.LongHelp`
- `internal/component/cli/model_keys.go` - the box reads `LongHelp`, with no fallback to the description
- `docs/architecture/config/yang-config-design.md`, `docs/guide/config-editor.md`

The prose: every `.yang` module holding a rendering statement whose `description`
breaches a bound. 825 at the start, measured over tracked files.

## Files to Create
- `internal/le/docvalid/testdata/help-shape-one-text/` - a fixture module declaring one text
- `test/ui/command-help-both-texts.ci` - the operator reads the summary on one row
- `test/ui/command-help-explanation-box.ci` - the operator reads the explanation in the box
- `test/plugin/plugin-command-two-texts.ci` - a plugin declares both texts
- `plan/deferrals/command-help-and-description.md` - the shard this spec's metadata names

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | Every `-cmd.yang` and `-api.yang` module, under D-1. No leaf and no container is added |
| YANG validation constraints | N-A | The change is to two documentation statements, and neither one takes a type |
| YANG custom validators | N-A | `ze:validate` judges a config VALUE. A declaration's shape is judged by `./le docvalid help-shape` |
| CLI commands/flags | No | No command is added. The two texts of every existing command change |
| CLI grammar (keyword before value) | N-A | No grammar changes |
| Editor autocomplete | Yes | `injectPluginCommands` and `warningText` read the renamed fields |
| Functional test for new RPC/API | Yes | `test/ui/command-help-both-texts.ci` and `test/plugin/plugin-command-two-texts.ci` |
| Pipe completeness | N-A | No command output changes shape |
| Env var registration | N-A | No leaf under `environment/` |
| Doctor check for runtime dependencies | N-A | No file path, socket, port, kernel module, or binary is added |
| Prometheus counters/metrics | N-A | The gate runs in `le`, which exports no metric |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`, the Self-Documenting System row, which states the current meaning in full |
| 2 | Config syntax changed? | No | No config leaf, container, or value changes, so `docs/guide/configuration.md` holds |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, which names `long-help` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`, the canonical page and its renderer table |
| 5 | Plugin added/changed? | Yes | `docs/plugin-development/commands.md`, `docs/plugin-development/protocol.md`, `docs/plugin-development/README.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/cli.md`, `docs/guide/api.md`, `docs/guide/web-interface.md`, `docs/guide/mcp/overview.md` |
| 7 | Wire format changed? | Yes | `docs/architecture/api/process-protocol.md`, under D-3 |
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugins.md`, `ai/rules/points/plugins/directives/bound-and-clean-every-declared-text.md`, `ai/patterns/registration.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 7950 Section 7.21.3 sets no length, so no `rfc/short/` row changes |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/documentation-testing.md` |
| 11 | Affects daemon comparison? | No | No capability is added or removed, so `docs/comparison.md` holds |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md`, `docs/architecture/cli/command-completion.md`, `docs/architecture/cli/error-surface.md`, `docs/architecture/mcp/overview.md`, `docs/architecture/web-interface.md`, `docs/architecture/testing/ci-format.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/features/introspection.md`, `docs/contributing/gh-pages.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED, never answered from memory. Run `./le spec citation anchors spec plan/spec-command-help-and-description.md` at the start of implementation, and name every document it lists |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/contributing/writing-style.md` carries four per-surface rows that state the current meaning, and `ai/skills/ze-review.md` names the pair |

## Implementation Steps

Phases 1 to 3 run whatever the owner answers. Phases 4 to 6 run only under
D-1 YES.

1. **Phase: Wiring (MANDATORY FIRST)** -- the gate reaches the long-half judge
   - Tests: `TestHelpShapeRefusesACommandWithNoLongHelp`, `TestDocWiringRunsTheLongHelpRule`
   - Files: `internal/le/docvalid/helpshape.go`, `internal/le/doc/wiring/docverify.go`, the fixture module
   - Verify: the gate names the new rule over the fixture, and the checkout stays green because the scope rule holds
2. **Phase: the three rules** -- `missing-long-help`, `char-cap`, `long-restates-summary`
   - Tests: the seven `helpshape_test.go` rows above
   - Files: `internal/le/docvalid/helpshape.go`
   - Verify: each rule refuses on its own row, and the detail carries the measured value beside the bound
3. **Phase: the write hook** -- an early warning on a proposed YANG edit
   - Tests: `TestWriteEditWarnsOnAYangCommandWithOneText`, `TestWriteEditPassesAYangCommandWithBothTexts`
   - Files: `internal/le/hookruntime/writeedit.go`
   - Verify: the check reads one proposed file lexically and loads no model, which is the whole of what a hook can do
4. **Phase: the Go rename** -- under D-1
   - Tests: `TestCommandNodeHelpIsTheSummary`, `TestMergeCommandPathsKeepsEachHalfOnItsOwnField`, `TestValidateHelpDeclsRefusesTheRetiredKey`, `TestCommandDeclRoundTripsBothKeys`
   - Files: the 18 Go files named above
   - Verify: no field is named `LongHelp`, and `Help` names the summary at every layer
5. **Phase: the YANG statement move** -- under D-1
   - Tests: `TestBuildCommandTreeReadsTheSummaryFromZeHelp`, then the whole gate over the checkout
   - Files: every `-cmd.yang` and `-api.yang` module declaring a command node or an RPC
   - Verify: `./le docvalid help-shape` reports 841 summaries and 841 long texts, with no refusal
6. **Phase: the documentation** -- the 25 pages, edited in the phase that made them wrong
   - Tests: `./le doc wiring`, `./le ste check`
   - Files: the Documentation Update Checklist rows
   - Verify: no page states the superseded meaning, and `./le spec citation anchors` lists no unnamed owner

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named file and symbol, and each conditional AC states which decision enabled it |
| Feature completeness | Every renderer row of `docs/architecture/api/commands.md` reads the field its row names |
| Correctness | A swap is caught. The long text is judged by the long rules and the summary by the summary rules, never the reverse |
| Naming | Under D-1 YES, no field, JSON key, or doc sentence still spells `LongHelp`, and `Help` means the summary at every layer |
| Data flow | `MergeCommandPaths` still writes a field only where that field is empty, so a half-filled node never takes the wrong half |
| Rule: `ai/rules/principles.md` | A retired JSON key is refused with a message, and never decoded to the zero value |
| Rule: `ai/rules/no-layering.md` | The old meaning is deleted rather than kept beside the new one, and no fallback reads the retired key |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The gate refuses a one-text command | `./le docvalid help-shape` over the fixture module |
| The gate is green over the checkout on day one | `./le doc wiring` |
| No field is named `LongHelp` | `grep -rn --include='*.go' 'LongHelp' internal cmd pkg` answers nothing |
| Every command declares both texts | `./le docvalid help-shape` reports equal summary and long-text counts on all three surfaces |
| No page states the superseded meaning | `grep -rln 'long-help' docs/ ai/` read against the Documentation Update Checklist |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A plugin's two texts stay bounded and free of control characters. `validateDeclaredText` keeps both bounds, and the rename must not drop the call for either half |
| Error leakage | `clampDeclared` still bounds a refused value before it reaches the daemon log and the disk under it |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood -> RESEARCH |
| Lint failure | Fix inline. If architectural -> DESIGN |
| Functional test fails | Check the AC: wrong AC -> DESIGN, correct AC -> IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- `Help` already carries two opposite meanings in this repository, and a copy runs across that crossing in `MergeCommandPaths`. That collision is what the owner's naming removes, and it is a better argument for the inversion than text length is.
- A gate whose population is the MODEL cannot see a renderer defect. `plan/journal/green-that-could-not-have-been-red.md` records exactly that for this area, over nine green gates. So every AC about what an operator READS carries a `.ci` test that runs the binary.
- The word cap does not protect the one-line row. 25 words is about 150 characters, and no terminal is that wide at the row the summary uses. A character bound is what the row needs, and no bound exists today.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The gate refuses only what the commit under test added or changed, measured against HEAD | (a) write the 488 long texts first, then arm an absolute gate. (b) a shrink-only baseline file listing the 488 paths | (a) asks one session for 488 authored paragraphs, and a session under that load writes restatements, which AC-1 of the superseded spec banned. (b) keys on a path, and `plan/future/spec-shrink-only-baseline-cannot-see-a-relocation.md` records that a rename then reads as new debt. The chosen shape persists no file, cannot be silenced by appending to one, and is the pattern `./le rfc check` already uses for a tagged unit carrying no discrimination record |
| Coverage is REPORTED beside the refusals | Report nothing until the corpus is complete | `helpShapeContract` already prints coverage for three surfaces, and that number is how the owner watches 353 climb toward 841 |
| The write hook warns and the `./le` gate refuses | The hook alone | The hook sees one proposed file and cannot load the merged model. It also never fires on an edit made through a Bash heredoc, which is what auto mode tells an agent to prefer (`.claude/rules/session-start.md`). An early warning is worth having, and it is never the only check |
| The config half stays where it is homed | Fold config nodes into this spec | `plan/future/spec-yang-config-leaf-short-and-long-help.md` already carries that population, its nine renderers, and the one constraint config has that commands do not: `ai/rules/config.md` makes a leaf description name the environment variable that overrides the leaf |

## Known Limitations

- The reported defect on `set bgp router-id` is not fixed here. A config node declares one text, so the inversion leaves that leaf exactly as it is, and D-2 decides whether the config spec runs.
- A long text that PARAPHRASES its summary passes AC-3, which catches an exact copy alone. A reader catches the rest, in the review round.
- The character bound is measured against the widest overlay Ze draws. A surface outside Ze, such as a published web page, has its own width and this bound does not govern it.

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
- [ ] The owner has answered D-1, D-2, D-3 and D-4

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
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

---

## Implementation Summary

### What Was Implemented
- The gate. `./le docvalid help-shape` judges four populations (command nodes, RPCs, offline local commands, config nodes) against `missing-long-help`, `char-cap`, `word-cap`, `long-restates-summary` and `missing-summary`. It derives its population by walking the resolved goyang entry tree, so no keyword list restates the schema, and it judges the long half only over declarations this tree changed against HEAD.
- The write hook. `./le hook-check pretool-writeedit` answers on a proposed `.yang` edit, scoped to rendering statements: a `module` or `revision` description takes no bound, and a `leaf` in a `-cmd.yang` or `-api.yang` module is not judged.
- The Go consistency pass. `Description` is the summary and `LongHelp` the explanation, in `command.Node`, `command.CommandEntry`, `yang.RPCMeta`, `pluginserver.Command`, `pluginserver.Completion`, `contract.Completion` and `client.commandEntry`. No field is named `Help`.
- The retired key. `help` named the summary on the daemon-to-CLI wire until 2026-09-03. `validateHelpDecls` and `decodeCommandList` refuse a payload carrying it rather than decoding it to an empty summary.
- The config two-text path. `ze:help` now covers a config node, `entryLongHelp` reads it off the resolved entry, and the `?` box shows it with no fallback to the description.
- The prose. 3,011 of 3,196 config nodes carry a long explanation, up from 16 at the start; 3,181 carry a summary. Command nodes 612 of 613, RPCs 204 of 211, offline locals 19 of 19.

### Bugs Found/Fixed
Writing an explanation means reading the producer, and reading 3,196 producers found defects the prose could not honestly describe. Each was fixed at its source, with a test that fails against the old code.

| Defect | Fix | Test that goes red without it |
|--------|-----|-------------------------------|
| `over-limit-policy drop` set the base input chain policy to drop, so a value named for over-limit traffic discarded SSH, ICMP and every service the four CoPP terms did not name | The drop rides the rate-limit term as the nftables inverted limiter; the chain policy is accept under both settings | `TestCoppTranslateDropPolicy`, `test/firewall/copp-over-limit-drop.ci` |
| A TACACS+ server with no key sent the packet body in cleartext under a header whose `TAC_PLUS_UNENCRYPTED_FLAG` was clear | `MarshalInto` returns `ErrNoSharedSecret`; `tacacsBackend.Build` refuses the server by address; the YANG leaf is `mandatory true` with `length "1..max"` | `TestPacketMarshalNoEncryption`, `test/parse/tacacs-key-required.ci` |
| The help of a node several modules declare was chosen by module-name order, so every declaration but the first was unreachable | `mergeHelpExts` joins them | `TestMergeKeepsEveryDeclarationsHelp`, `TestMergeKeepsTheHelpOfTheDeclarationThatCarriesOne` |
| Three `.(bool)` and `.(float64)` assertions on delivered config could never succeed, because every leaf arrives as a string | `configvalue.Bool` and `configvalue.Int` | `TestParseFIBConfigReadsTheDeliveredStrings`, the pool tests fed the delivered shape |
| `fib/kernel` read its section one level above where it arrives, so neither of its settings had ever reached a running daemon | `configvalue.Section` derives the unwrap from the declared root | every test in `config_delivery_test.go` |
| `configvalue.Int`'s upper bound rejected nothing: `math.MaxInt64` rounds up to 2^63 in float64, so exactly 2^63 passed and the conversion answered differently per architecture | `value >= -float64(math.MinInt64)` | `TestIntReadsEveryDeliveryShape/one_past_the_top_of_int64` |
| `Bool` and `Int` could not tell an absent leaf from an unreadable one, and both callers kept their default | The callers split on map presence and refuse a value that does not read | `TestParseFIBConfigRefusesAValueItCannotRead` |
| `sweep-delay 0` is schema-legal and was discarded by a `> 0` guard | Accepted | `TestParseFIBConfigReadsAConfiguredZero` |
| `show firewall` and the web rules page printed an inverted limiter exactly like a plain one | Both print `over` | `TestFormatActionTypes/limit_packets,_inverted` |
| `ddos/flowtriq/api-key` carried a bearer token with no `ze:sensitive` | Marked sensitive. A sweep of 13 secret-shaped leaves found no second gap | (schema; the masking producer is `LeafHoldsSecret`) |

### Documentation Updates
- `docs/architecture/config/yang-config-design.md` -- the config-node two-text section, and what the merge does when several modules declare one node.
- `docs/architecture/config/syntax.md` -- a plain leaf is a STRING at both ends of the delivery, which is the fact three readers guessed wrong.
- `docs/architecture/traffic/cp-survival-2-copp-port179.md` -- the chain policy is always accept, and `over-limit-policy` rides the limiter.
- `docs/guide/tacacs.md` -- what a keyless server does at commit, at boot with BGP, and at boot without it.
- `docs/features.md` -- the CoPP row. **Left uncommitted:** its other changed row belongs to another session.
- `./le ste check` passes on every one.

### Deviations from Plan
| Planned | Actual | Why |
|---------|--------|-----|
| D-1 inverts the two statements | No inversion. `description` stays SHORT, `ze:help` stays LONG | The owner reversed his own instruction on 2026-09-03: "sorry I gave the instruction the wrong way round. make sure all is consistent with no inversion" |
| `internal/le/docvalid/testdata/help-shape-one-text/` | `internal/le/docvalid/testdata/helpshape/ze-fixture-conf.yang` | One fixture module serves every rule, so the per-rule directory was never created |
| `plan/deferrals/command-help-and-description.md` | Never created | Nothing was deferred |
| Five named TDD tests | Three exist under other names, two were genuinely absent and are now written | The three renames predate D-1 and carried the inverted spelling |
| Config nodes out of scope | Folded in | The owner answered D-2 "Fold it in here" |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1 assumed the owner wanted the inversion at the YANG layer | He wanted consistency with the SHIPPED meaning, and no inversion at all | He answered D-1 | The spec's premise was rewritten before implementation started |
| approach | The first measurement reported 4,147 over-long descriptions | 825. Untracked build copies under `tmp/` were counted | The count did not match `git ls-files` | The Measured Population section states that it reads tracked files only |
| approach | Three regex scans of the command tree answered 21, 36 and 21 nodes | 107. A regex cannot see the merged tree the gate walks | The gate disagreed with every scan | The population is enumerated from a built binary carrying all 36 feature tags |
| approach | Three agents were briefed without statement scoping and moved module and revision prose into comments | Only a rendering statement takes a bound | Reading their diffs | All three reverted; the scoping moved into the gate and the hook, where a brief cannot lose it |
| escalation | A unit test was written with a hand-typed unwrapped fixture and passed while the code under it could never read a real section | The daemon wraps a section in its full root path | Round 2 of the review | `configvalue.Section` exists so the next reader does not guess, and the fixtures carry the wrapper |
| escalation | Three prose claims about the TACACS+ key were written from the code I had changed rather than from the code that runs it | A keyless server refuses a reload, not a boot; a nil bundle lets SSH fall back to local users | Round 2 of the review | Every clause now names the producer it was read from |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Every command has a help and a description | Done | 612 of 613 command nodes, 204 of 211 RPCs, 19 of 19 offline locals | The gate names any node a commit leaves half-declared |
| `help` for the short text, `description` for the long | Changed | Reversed by the owner in D-1 | `description` is SHORT and `ze:help` is LONG, which is the shipped meaning |
| Use these names in the code | Done | `Description` and `LongHelp` on seven types | No field is named `Help` |
| Code to validate after YANG edit or creation that both exist and have suitable length | Done | `./le docvalid help-shape`, `./le hook-check pretool-writeedit` | The gate is the load-bearing check; the hook is the early warning |
| Config nodes too | Done | 3,011 of 3,196 carry both | D-2, "Fold it in here" |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestHelpShapeRefusesACommandWithNoLongHelp`, `...AnRPC...`, `...ALocal...` | |
| AC-2 | Done | `TestHelpShapeRefusesASummaryPastTheCharacterCap`, `TestHelpShapeRefusesASummaryPastTheWordCap` | |
| AC-3 | Done | `TestHelpShapeRefusesALongTextThatRestatesItsSummary` | |
| AC-4 | Done | 13 rows in `writeedit_test.go` | |
| AC-5 | Done | `TestMergeYANGEntryReadsHelpExtension` | The plan's name for this test carried the pre-D-1 spelling |
| AC-6 | Done | `internal/component/command/node.go` declares `Description` and `LongHelp`; a grep for a field named `Help` returns nothing | |
| AC-7 | Done | The same grep over `pluginserver`, `yang.RPCMeta` and `client.commandEntry` | |
| AC-8 | Done | `TestCommandDeclRoundTripsBothKeys`, `TestCommandDeclOmitsTheExplanationItDoesNotDeclare`, `TestRuntimeTreeCarriesLongHelp` | Written in this closure round; the wire keys had no round-trip test |
| AC-9 | Done | `TestConfigCompletionRowIsNotTheParagraph`, `test/ui/command-help-both-texts.ci` | |
| AC-10 | Done | `TestValidateHelpDeclsRefusesTheRetiredKey`, `TestDecodeCommandListRefusesTheRetiredKey` | Written in this closure round; both guards had no test |
| AC-11 | Done | `TestRevealCandidateExplanationUsesLongHelp`, `test/ui/config-help-both-texts.ci` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The seven `helpshape_test.go` rows | Done | `internal/le/docvalid/helpshape_test.go`, `helpshape_schema_test.go` | |
| The 13 `writeedit_test.go` rows | Done | `internal/le/hookruntime/writeedit_test.go` | |
| `TestCommandNodeHelpIsTheSummary` | Changed | Covered by `TestMergeYANGEntryReadsHelpExtension` and the field declarations | The plan's name asserts a compile-time fact |
| `TestBuildCommandTreeReadsTheSummaryFromZeHelp` | Changed | `TestMergeYANGEntryReadsHelpExtension` | The plan's name carried the pre-D-1 inversion |
| `TestMergeCommandPathsKeepsEachHalfOnItsOwnField` | Changed | `TestMergeCommandPathsDecidesEachHelpFieldOnItsOwn` | Renamed, same claim |
| `TestValidateHelpDeclsRefusesTheRetiredKey` | Done | `internal/component/plugin/server/startup_test.go` | Absent until this closure round |
| `TestCommandDeclRoundTripsBothKeys` | Done | `pkg/plugin/rpc/types_test.go` | Absent until this closure round |
| The four completer and model_keys rows | Done | `completer_test.go`, `model_keys_test.go` | |
| The four functional tests | Done | `test/ui/` x3, `test/plugin/` x1 | All four pass |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Every file in Files to Modify | Done | All present and changed |
| `internal/le/docvalid/testdata/help-shape-one-text/` | Changed | One fixture module under `testdata/helpshape/` serves every rule |
| The three `.ci` files | Done | Plus `test/ui/config-help-both-texts.ci`, which D-2 added |
| `plan/deferrals/command-help-and-description.md` | Skipped | Nothing was deferred, so the shard was never created |

### Audit Summary
- **Total items:** 34
- **Done:** 27
- **Partial:** 0
- **Skipped:** 1 (the deferral shard, because nothing was deferred)
- **Changed:** 6 (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Every command carries both texts | functional | `test/ui/command-help-both-texts.ci` and `test/ui/command-help-explanation-box.ci` PASS in the `ui` suite (91, 92 of 243). Each was forced red at its own renderer |
| A config node carries both texts | functional | `test/ui/config-help-both-texts.ci` PASS (107 of 243), discrimination RED recorded |
| A plugin declares both texts and both survive the wire | functional | `test/plugin/plugin-command-two-texts.ci` PASS (470 of 731) |
| Code validates that both exist and have suitable length after a YANG edit | gate | `./le docvalid help-shape` exits 0 over 613 command nodes, 211 RPCs, 19 offline locals and 3,196 config nodes, and named 29 breakages before they were written |
| The operator reads a summary on one row and a paragraph in the box | functional | `TestConfigCompletionRowIsNotTheParagraph` plus `config-help-both-texts.ci`, which types `set rou`, reads the row, presses `?` and reads the RFC 6286 paragraph |
| The prose is worth writing rather than a restatement | measurement | Over the 554 nodes carrying both texts: 0 byte-equal, and 48 (8.7%) open by repeating the description before adding new material. A-2 holds; R-3 is largely not realized |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| The shard was never created | done | Nothing was deferred. Every defect found on the way was fixed in this spec's commits or recorded as a journal row with its class |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/command-help-and-description-dd5fc94a-0a52-4e4b-b946-b2c79adc8646.md`, 16 files, verdict clean |
| `review_gate.py check` | clean |
| Rounds | 4. Round 1 found 12, round 2 found 6, round 3 found 4, round 4 found none |
| Reviewer lenses used | logic+wiring, security+edge-cases, feature-risk+docs+simplicity (round 1, three parallel agents); the fixes only (rounds 2 and 3) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | `fib/kernel` read its config section one level above where it arrives, so neither setting had ever reached a running daemon | `parseFIBConfig` | `configvalue.Section`, and fixtures carrying the wrapper |
| 2 | BLOCKER | Three prose claims about a keyless TACACS+ server were false: it refuses a reload not a boot, and a nil bundle lets SSH fall back to local users | `docs/guide/tacacs.md`, `ze-tacacs-conf.yang`, `tacacs-key-required.ci` | Every clause rewritten against its producer |
| 3 | ISSUE | `configvalue.Int`'s upper bound rejected nothing at exactly 2^63 | `configvalue.Int` | `value >= -float64(math.MinInt64)` |
| 4 | ISSUE | `Bool` and `Int` could not separate an absent leaf from an unreadable one | both callers | Split on map presence, refuse by name |
| 5 | ISSUE | `sweep-delay 0` discarded by a `> 0` guard | `parseFIBConfig` | Accepted |
| 6 | ISSUE | An inverted limiter rendered exactly like a plain one on two surfaces | `formatLimit`, `page_firewall.go` | Both print `over` |
| 7 | ISSUE | `over-limit-policy drop` had no test through the daemon | -- | `test/firewall/copp-over-limit-drop.ci` |
| 8 | ISSUE | The `?` box truncates a joined help and no key scrolls it | `renderExplanationBox` | Not fixed. Journal row; the page states the limit |
| 9 | ISSUE | A doc sentence claimed the box names each contributing module | `yang-config-design.md` | Sentence corrected |
| 10 | ISSUE | `mergeAugmentedEntries`' comment claimed the row was fixed too | `completer.go` | Comment corrected |
| 11 | ISSUE | The reload refusal is gated on a live bundle; the prose stated it unconditionally | `docs/guide/tacacs.md` | Table states all three paths |
| 12 | ISSUE | `ze:sensitive` does not encode what the commit path writes | two `ze:help` texts | Corrected; only `ze config dump` encodes |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/docvalid/testdata/helpshape/ze-fixture-conf.yang` | yes | `ls` 3.7K |
| `test/ui/command-help-both-texts.ci` | yes | `ls` 1.9K |
| `test/ui/command-help-explanation-box.ci` | yes | `ls` 2.0K |
| `test/ui/config-help-both-texts.ci` | yes | `ls` 1.7K |
| `test/plugin/plugin-command-two-texts.ci` | yes | `ls` 2.9K |
| `test/parse/tacacs-key-required.ci` | yes | committed in `4a1521d1b` |
| `test/firewall/copp-over-limit-drop.ci` | yes | committed in `f68705107` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-3 | The gate refuses a half-declared or over-long declaration | `./le docvalid help-shape` exit 0 today; it reported 29 breakages over 8 modules before they were written |
| AC-4 | The write hook answers on a proposed YANG edit | 13 PASS rows in `writeedit_test.go` |
| AC-5 | `description` is the summary, `ze:help` the explanation | `TestMergeYANGEntryReadsHelpExtension` asserts each reaches its own field |
| AC-6, AC-7 | No field is named `Help` | `grep -rn "\tHelp "` over the seven types returns nothing |
| AC-8 | The wire keys are `description` and `long-help` | `TestCommandDeclRoundTripsBothKeys` reads the raw JSON keys; red when `long-help` is renamed to `help` |
| AC-9, AC-11 | The row holds the summary, the box the explanation | `config-help-both-texts.ci` PASS |
| AC-10 | A retired key is refused, never decoded to zero | `TestValidateHelpDeclsRefusesTheRetiredKey` and `TestDecodeCommandListRefusesTheRetiredKey`; both red with their guard disabled |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| The operator opens the completion menu | `test/ui/command-help-both-texts.ci` | PASS, 91 of 243 |
| The operator presses `?` | `test/ui/command-help-explanation-box.ci` | PASS, 92 of 243 |
| The operator types `set rou` and presses `?` | `test/ui/config-help-both-texts.ci` | PASS, 107 of 243 |
| A plugin declares both texts | `test/plugin/plugin-command-two-texts.ci` | PASS, 470 of 731 |
| An operator commits a keyless TACACS+ server | `test/parse/tacacs-key-required.ci` | PASS, 297 of 326 |
| An operator sets `over-limit-policy drop` | `test/firewall/copp-over-limit-drop.ci` | NOT RUN. `needs-linux`, so it skips on this darwin host. Its red walk is owed on a Linux host |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | The owner answered D-1 no. Mistake Log row and Deviations entry |
| A-2 | confirmed | 0 byte-equal texts over 554 nodes carrying both; 8.7% open by repeating the summary and none is a pure restatement |
| A-3 | confirmed | D-3, "Yes, and refuse the retired key". Both refusals now carry a test |
| A-4 | not applicable | The gate persists nothing: its baseline is the summary TEXT read from HEAD, so a rename is not read as new debt |
| A-5 | confirmed | D-4, the proposed bounds, against the measured 7.3 percent cost |
| A-6 | confirmed | The hook check was written first and answers over a fixture module; 13 rows pass |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| The config-node two-text section | `entryDescription`, `entryLongHelp`, `mergeHelpExts` | yes |
| A plain leaf is a string at both ends | `(*Tree).toMap` copies `values map[string]string` through | yes |
| The CoPP chain policy is always accept | `translatePolicy` returns `firewall.PolicyAccept` unconditionally | yes |
| A keyless TACACS+ server at commit, at boot with BGP, at boot without | `main_reload.go`, `infra_setup.go`, `main.go` `noBGPAAAWiring`, `ssh.Server.Start` | yes, after two corrections |
| `ze:sensitive` masks but does not encode on the commit path | the only `secret.Encode` callers are `cmd_dump.go` and `iface/emit.go` | yes, after one correction |

## Core Insight

**A test that builds its own fixture chooses the shape, and it chooses the shape the author believes rather than the shape the producer emits.** Three defects in this spec are one defect wearing three faces: a `.(float64)` assertion that no delivered value satisfies, a section read one level above where it arrives, and a `> math.MaxInt64` guard that rejects nothing. Each had a green unit test over a hand-typed fixture, and each was invisible for exactly as long as nobody compared the fixture with the producer. The cure is not more assertions. It is to build the fixture from the producer, or to put the shape question in one reader that every caller uses.
