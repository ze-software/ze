# Spec: pipe operators derived from answer shape, published per command

| Field | Value |
|-------|-------|
| Status | verification |
| Scope | cli |
| Depends | `plan/audit-pipe-operator-coverage.md`, `plan/audit-presentation-pipes.md`, `plan/audit-command-pipe-vs-subcommand.md` |
| Phase | 6/6 |
| Deferral shard | `plan/deferrals/cli-pipe-operator-coverage.md` |
| Handoff | `plan/handoff-cli-remaining.md` |
| Updated | 2026-08-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make every command answer every pipe operator its answer's SHAPE supports,
refuse by name the ones it does not, and publish that per-command answer from a
single derivation that the wiki, the website and `ze help command --json` all
read.

Three things are owed, and the owner stated each.

1. **Global operations on every command.** Formatting and saving act on the
   answer whatever it holds, so every command owes them. Saving does not exist
   at all today.
2. **Data-dependent operations where the data supports them.** Selections and
   summaries are owed only where there are rows to select from, and where there
   are not, the operator MUST be refused rather than answered wrongly.
3. **The website documentation improved with what each command supports.** That
   is the deliverable this spec exists for. It MUST be generated, and a gate
   MUST fail when the page and the product disagree.

The owner also ruled that nesting MUST work: `match X | match Y` composes, and
so does every other repeatable operator, on every path that runs a chain.

`plan/audit-pipe-operator-coverage.md` measured the current state and this spec
implements its section 9. The audit is the evidence; this file is the contract.

## Required Reading

### Architecture Docs

- `docs/architecture/api/commands.md` -- aliases, command-owned filters, and the
  client/daemon split. The most accurate pipe document in the tree, and the one
  the website manifest omits.
- `docs/architecture/api/wire-format.md` -- the answer head, its item type
  (`doc` / `map` / `tab`), and the column names a `tab` head carries.
- `docs/features/formatting.md` -- the only page publishing all 16 operator
  names, and the page whose universal claim this spec makes true.
- `ai/rules/cli.md` -- the rule this spec replaces. Section 6 of the audit says
  why the current one cannot be met.
- `ai/rules/evidence.md` -- derive, do not hardcode. This spec removes five
  hand-copied operator lists.

### RFC Summaries (Scope: protocol)

Not applicable. No wire protocol changes; the answer head already carries the
item type this spec reads, and the one head change (field types) is Ze's own
format, specified in `docs/architecture/api/wire-format.md`.

## Current Behavior (MANDATORY)

Measured in `plan/audit-pipe-operator-coverage.md` against a daemon built from
this tree. The numbers below are its measurements, not estimates.

| Fact | Value | Where |
|------|-------|-------|
| Operators the parser knows | 16 | `knownPipeOps`, now derived in `internal/component/command/pipe_catalog.go` |
| Hand-copied operator lists elsewhere | 5, no two agreeing | audit 1.1 |
| Operators absent from EVERY user-reachable list | 2 (`display`, `fill`) | audit 1.1 |
| Commands reaching no pipe layer on any surface | 38 | audit 3.1 |
| Of those, published as `"global-pipes": true` while the daemon answers `unknown command` | 20 | audit 3.1 |
| Commands losing pipes only on the `ze <verb>` form | 6 measured, 2 unmeasured | audit 3.2 |
| Wiki entries carrying a hand-typed ten-operator line | 381 | audit 8.2 |
| Behaviors for a repeated operator | 4 | audit 5 |
| A `save` / `write` / `tee` operator | none exists | audit 1 |

Source files read for the table above:

- [x] `internal/component/command/pipe.go` -- the original `ValidatePipes`, `countItems`, `truncateItems`, `collectPipeMeta`, `foldFilters`, and `ApplyPipes`
- [x] `internal/component/command/pipe_columns.go` -- `columnsInChain`, which originally assigned rather than intersected
- [x] `internal/component/command/pipe_records.go` and `render_records.go` -- the record path now owned by `RenderRecords`
- [x] `internal/component/command/column_order.go` -- `ColumnsForCommand`, the registry pattern the shape declaration reuses
- [x] `internal/component/command/pipe_filter.go` -- `PipeFiltersForCommand` and `unknownFilterError`
- [x] `internal/component/command/alias.go` -- `AliasesForCommand`, originally absent from `ze help command --json`
- [x] `pkg/plugin/rpc/message.go` -- `AppendAnswerHead`, `checkAnswerType`
- [x] `cmd/ze/help_command.go` -- `operatorsFor`, which replaced the verbose help copy
- [x] `cmd/ze/ze_core_pipe.go` -- `pipeUsage`, now catalog-derived
- [x] `internal/component/ssh/ssh.go` -- `ProcessPipesDefaultFormatChecked` on the exec channel
- [x] `internal/le/wikicatalog/render.go` -- the native generator
- [x] `internal/le/docvalid/command_render.go` -- `RenderCommandSurfaces`, which replaced the Python-only website route

Three paths run a chain and the same operator means different things on each
(audit 2): the DOCUMENT path (`ApplyPipes`), the RECORD path
(`ApplyPipesRecords`), and NONE, where a local handler answered first and there
is no pipe layer at all.

The specific wrong answers this spec must end, each measured:

| Chain | Answers today | Why |
|-------|---------------|-----|
| `show bgp \| count` | 6 | `countItems` counts top-level KEYS when a map holds two or more |
| `show bgp \| first 1` | the whole payload | `truncateItems` leaves a map of six alone |
| `show version \| first 1` | the version string | a scalar answer has no rows and nothing refuses the operator |
| `match idle \| match 192` | all three rows | the document metadata path overwrites instead of composing |
| `display state \| display address` | `{address}` | the second request WIDENS, recovering a dropped field |
| `show bgp \| origin` | decorates any value that parses as an address | no field type is declared anywhere |
| `show env list \| json` | `error: unknown command` | YANG declares the wire method, no handler serves it |

`ValidatePipes` (`pipe.go:488`) checks syntax only: argument presence, a
number's sign, one format operator. It never reads the item type, so no
operator is ever refused for shape.

### Behavior to preserve

| Behavior | Why it must not change |
|----------|------------------------|
| A bare command's default rendering, on every surface | 465 commands' output is what every existing `.ci` asserts and what every operator reads. This spec adds refusals and a save operator; it does not restyle an answer |
| `\| json`, `\| ndjson`, `\| yaml` keeping alphabetical keys | owner directive, 2026-08-19: column order is for a person, and a program reads these three |
| Command-owned filters winning over a generic operator of the same name | `foldFilters` runs first, and `show bgp rib \| match X` reaching the handler is what makes the RIB filter fast. The composition fix must keep the fold, not replace it |
| `multipleFormatsError` refusing two format operators | already correct, and it is the model this spec generalises |
| `unknownFilterError` enumerating the valid set | already correct, and AC-3's refusal copies its shape |
| An old client reading a new head | R-4. An added head field must be ignorable |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

Four, and they differ, which is why the audit found a client/daemon asymmetry:

| Entry | Expands the chain | Reaches |
|-------|-------------------|---------|
| `ze cli -c "<cmd>"` | the DAEMON, `ProcessPipesDefaultFormatChecked` | document or record path |
| an SSH exec channel | the DAEMON, same call (`internal/component/ssh/ssh.go`) | document or record path |
| bare `ze cli` (interactive) | the CLIENT (`model_mode.go`) | document path over a collapsed answer |
| `ze <verb> ...` | `matchLocalHandler` in `RunCommand`, BEFORE daemon dispatch | no pipe layer |

### Transformation Path

1. `ParsePipe` splits the input into a command and a chain.
2. `foldFilters` removes each command-owned filter from the chain and appends it
   to the command's arguments.
3. `ValidatePipes` checks the chain's syntax.
4. **(new)** the command's declared SHAPE is resolved, and every operator that
   shape does not support is refused by name, before dispatch.
5. Dispatch: the daemon handler, or **(changed)** a local handler now returning a
   payload instead of printing.
6. `ApplyPipes` (document) or `ApplyPipesRecords` (records) runs the chain.
7. `ProcessPipesDefaultFormatChecked` appends the session's default format when
   the chain named none.

This spec inserts one step and changes one:

- **inserted**, after `foldFilters` and before dispatch: resolve the command's
  declared SHAPE and refuse every operator that shape does not support.
- **changed**: the local-handler branch of `RunCommand` gains a payload-returning
  form, so a local answer reaches the same `ApplyPipes` the daemon answers reach.

### Boundaries Crossed

| Boundary | What crosses | Direction |
|----------|--------------|-----------|
| command registry -> pipe layer | the declared shape and field types | read |
| producer -> wire | the answer head's item type and, new here, its field types | write |
| daemon -> client | the rendered answer, unchanged | write |
| in-process registry -> `ze help command --json` | the per-command operator list, replacing the `global-pipes` boolean | read |
| JSON -> wiki and website generators | that same list | read |

### Integration Points

- `internal/component/command/`: `pipe.go`, `pipe_records.go`, `pipe_columns.go`,
  `pipe_filter.go`, `column_order.go`, and the shared `newCommandRegistry[T]`
  that `RegisterColumns` and `RegisterPipeFilters` already use. The shape
  registry is a third user of that same mechanism, not a new one.
- `pkg/plugin/rpc/message.go`: the head, `checkAnswerType`, and the field types
  a `tab` head gains.
- `cmd/ze/help_command.go`, `cmd/ze/ze_core_pipe.go`, `ze_core_dispatch.go`,
  `completer.go`: the five hand-copied lists, all deleted and read from the
  catalog.
- `internal/le/wikicatalog/render.go`, `website/tools/render-cli-catalog.py`,
  `render-command-equivalents.py`, `render-llms-txt.py`: the generators.
- `internal/le/docvalid/drift.go` under `./le doc check verify`: the gate.

### Architectural Verification

| Question | Answer |
|----------|--------|
| Does a new command need core changes to declare its shape? | No. It calls `RegisterShape` from its own `register.go`, like `RegisterColumns` today |
| Registration over hardcoding | Yes. The shape and the field types are declared by the command that owns them, resolved by the existing longest-prefix registry. Nothing in a core package enumerates commands |
| Does the core hold a list of which commands support what? | No. The list is derived per command at publish time from the catalog and the declared shape |
| New package? | No |
| Does a plugin get the same surface as a built-in? | Yes. `RegisterShape` is on the same registry a plugin already uses for filters, columns and aliases |

## Risks & Assumptions

### Assumptions

- **A-1**: The answer head's item type is already written by every producer, so
  the shape a command declares can be CHECKED against the answer in hand rather
  than trusted. `AppendAnswerHead` and `checkAnswerType` are the evidence.
- **A-2**: A command's shape is a property of the command, not of the data
  volume. The head's item type is not (audit 2.1: the same command answers `doc`
  at 200 rows and `tab` at 300), which is exactly why the declaration is needed
  beside it.
- **A-3**: Converting a local handler to return a payload does not change what
  `ze <verb>` prints today, because the same text renderer runs over that payload
  by default.
- **A-4**: The wiki and website generators consume `ze help command --json` and
  nothing else, so enriching that JSON reaches all three without a second
  channel.

### Risks

- **R-1**: Refusing an operator that today answers something is USER-VISIBLE and
  will break a script that relies on the wrong answer. `show bgp | count`
  answering 6 is the case: a caller may parse that 6. Mitigation: the refusal
  names the shape and the alternative (`show bgp | peers | count`), and the
  change is called out in the release notes. The alternative -- keeping a wrong
  answer -- is what the owner asked to end.
- **R-2**: The `pipe` metadata key becomes an ordered list instead of a map, so a
  tool reading `.pipe.match` reads `.pipe[].arg` after this. It is the only
  honest representation of a composing chain, and the current map cannot record
  `match X | match Y` at all.
- **R-3**: 46 local paths converting to payload answers is the largest phase and
  the one most likely to change output by accident. Mitigation: phase 4 converts
  them family by family, each with a `.ci` asserting the bare command's output is
  byte-identical before and after.
- **R-4**: Field types on the head are a wire addition. Old readers MUST ignore
  an unknown head field rather than refuse it; the parser's treatment of unknown
  head fields is verified in phase 2 before anything is written.

## Blast Radius

| Area | Effect |
|------|--------|
| Every CLI answer | a refused operator where one was silently accepted |
| `ze help command --json` | `global-pipes` boolean replaced by an operator list, plus aliases |
| the wiki catalog | 381 hand-typed lines become derived per-command lines |
| the website | three generators gain real pipe information |
| plugins | a new optional declaration; a plugin that declares nothing keeps a `doc` default |
| the wire | one added head field, ignorable by an old reader |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | Feature Code | Test |
|-------------|--------------|------|
| `ze help command --json` | `ShapeForCommand` -> catalog derivation | `TestWiringShapeReachesPublishedJSON` |
| `ze cli -c "<testcmd> \| fill"` | `ValidatePipes` -> shape refusal | `test/plugin/pipe-shape-refusal.ci` <!-- doc-links: ignore (this spec creates this file; the phase that does is named in the same row) --> |
| `./le wiki-catalog update` | JSON -> `gen_wiki_commands.py` | `test/plugin/pipe-catalog-published.ci` <!-- doc-links: ignore (this spec creates this file; the phase that does is named in the same row) --> |

A command registered in a test package, declaring shape `tab` with one address
field, MUST:

1. appear in `ze help command --json` with the exact operator list its shape
   derives, without any core file naming it;
2. be refused `| fill` when it declares shape `map` instead, by name, with the
   shape in the message;
3. reach the generated wiki page with that same list, produced by the generator
   holding no operator literal.

If any of the three needs an edit to a core package to pass, the registration is
not wired and the phase is not done.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| **AC-1** | any surface naming an operator | `knownPipeOps` is the single source, exported with each operator's class, supported shapes, argument kind, repetition semantics and description. The five hand-copied lists are DELETED and read from it. A test fails if any surface names an operator the catalog does not hold, or omits one it does. |
| **AC-2** | a command with no shape declaration | A command declares its answer shape through the existing command registry. Absent a declaration the shape is `doc`, which is the conservative answer: it owes the globals and refuses the row operators. |
| **AC-3** | `show version \| first 1`; `show bgp \| count` | An operator the shape does not support is REFUSED by name, naming the shape and the reason, before the command runs. `show version \| first 1` is refused; `show bgp \| count` answers rows or is refused, never 6. |
| **AC-4** | an envelope carrying rows beside aggregates | `countItems` and `truncateItems` answer over ROWS on an envelope that carries rows beside aggregates, or the operator is refused. The measured 6, 8 and 5 become the row counts or refusals. |
| **AC-5** | `match X \| match Y`; `fill a \| fill b` | Every repeatable operator COMPOSES on all four paths -- document data, document metadata, folded command filters, and records. `match X \| match Y` narrows twice on each. An operator whose repetition is meaningless (`fill`, `count`, a second format) is REFUSED, not silently replaced. |
| **AC-6** | a chain repeating an operator | The `pipe` metadata records the whole chain in order, so a repeated operator is visible to a tool. |
| **AC-7** | `<any command> \| save <path>` | A `save` operator exists, writes the answer to a named path, and is in the global class, so every command owes it. It works in an interactive session, which is the surface a shell redirect cannot reach. |
| **AC-8** | `<any command> \| log` over `ze cli -c` and an exec channel | `\| log` is honored on the exec and `ze cli -c` surfaces, or it leaves the global class. It is not left inert on the two surfaces a tool author uses. |
| **AC-9** | `<rows command> \| match X` with no format operator | `\| match` with no format operator filters ROWS, not the dispatcher's JSON. The documented "grep on output lines" becomes true for the spelling an operator actually types. |
| **AC-10** | `show env list \| json`, and the other 37 | All 46 local-registry paths reach the pipe layer, so the 38 commands that reach none today reach one. `show env list \| json` answers. |
| **AC-11** | `ze show interface` versus `ze cli -c "show interface"` | A dual-registered command answers identically through `ze <verb>` and `ze cli -c`, including its pipe handling. The `show interface` asymmetry is gone. |
| **AC-12** | `\| resolve` / `\| origin` on a command declaring no address field | `resolve` and `origin` are refused unless a field is DECLARED to hold an address. A `tab` head carries field types; `doc` and `map` refuse until the command declares otherwise. |
| **AC-13** | `ze help command --json` | `ze help command --json` publishes, per command, the operator list the command's shape derives, plus its filters and its aliases. The `global-pipes` boolean is gone. |
| **AC-14** | `./le wiki-catalog update`; the website build | The wiki catalog and the website CLI catalog are generated from that JSON and hold no operator literal. `docs/architecture/api/commands.md` is in the website manifest. |
| **AC-15** | an operator name in docs absent from the catalog | `./le doc check verify` fails when any operator name in `docs/` or the wiki is absent from the catalog, or when a command's published list disagrees with what it declares. |
| **AC-16** | `ai/rules/cli.md` read against the product | `ai/rules/cli.md` states the two rules of audit section 6, replacing the unmeetable universal one, and its point file follows. |

## End-to-End User Stories

- **US-1**: A tool author reads the website page for `show bgp peer list`, sees
  the exact operators it supports, builds against them, and is right.
- **US-2**: An operator types `show version | first 1` and is told a single-value
  answer has no rows, instead of receiving a truncated answer.
- **US-3**: An operator types `show bgp rib | match 10.0 | match /24` and gets the
  intersection, in an interactive session and over `ze cli -c` alike.
- **US-4**: An operator inside an interactive session types
  `show bgp rib | save /tmp/rib.json` and the answer lands in the file, which no
  shell redirect can do from that session.
- **US-5**: A plugin author declares a shape in `register.go` and their command's
  supported operators appear on the website with no core change.

## 🧪 TDD Test Plan

### Unit Tests

| Test | Asserts | Phase |
|------|---------|-------|
| catalog is the only source | every published list equals the catalog; a name added to the catalog appears in all five surfaces with no other edit | 1 |
| completion matches the parser | `PipeOperators` and `knownPipeOps` hold the same names -- the self-comparison in `completer_test.go` is replaced | 1 |
| shape resolution | longest-prefix, and an undeclared command resolves to `doc` | 2 |
| refusal by name | each shape refuses exactly the operators of the derivation table, message naming operator and shape | 2 |
| head field types | a `tab` head round-trips field types; an old reader ignores an unknown head field | 2 |
| `countItems` over an envelope | rows counted, not keys, for a payload carrying rows beside aggregates | 3 |
| `truncateItems` over an envelope | truncates rows, leaves aggregates | 3 |
| composition, four paths | `match X \| match Y` narrows on document data, document metadata, folded filters and records | 3 |
| repetition refusal | `fill \| fill`, `count \| count`, two formats each refused by name | 3 |
| chain metadata | the `pipe` key records both occurrences in order | 3 |
| `save` | writes the answer, refuses an unwritable path by name, is offered on every shape | 5 |
| local handler payload | a converted local handler answers the same text bare, and honors a chain | 4 |

### Boundary Tests (numeric inputs)

| Input | Boundary | Expected |
|-------|----------|----------|
| `first 0` | zero | refused, as today |
| `first N` where N = row count | exact | all rows |
| `first N+1` | one over | all rows, no error |
| rows = 0 with `first 1` | empty | empty answer, not an error |
| a `doc` answer of exactly 256 records | `rpc.AnswerBufferThreshold` | the DECLARED shape governs the refusal, not the head's item type, which flips here |

### Functional Tests

| File | Proves | Phase |
|------|--------|-------|
| `test/plugin/pipe-shape-refusal.ci` | `show version \| first 1` refused by name; `show bgp \| count` no longer answers 6 | 2 <!-- doc-links: ignore (this spec creates this file; the phase that does is named in the same row) --> |
| `test/plugin/pipe-chain-composes.ci` | `match X \| match Y` narrows over `ze cli -c`, and `display a \| display b` no longer widens | 3 <!-- doc-links: ignore (this spec creates this file; the phase that does is named in the same row) --> |
| `test/plugin/pipe-save.ci` | `\| save` writes the file and its content matches the piped answer | 5 <!-- doc-links: ignore (this spec creates this file; the phase that does is named in the same row) --> |
| `test/plugin/pipe-local-command.ci` | `show env list \| json` answers instead of `unknown command` | 4 <!-- doc-links: ignore (this spec creates this file; the phase that does is named in the same row) --> |
| `test/plugin/pipe-dual-registration.ci` | `ze show interface` and `ze cli -c "show interface"` answer identically | 4 <!-- doc-links: ignore (this spec creates this file; the phase that does is named in the same row) --> |
| `test/ui/pipe-interactive-save.et` | `\| save` inside an interactive session | 5 <!-- doc-links: ignore (this spec creates this file; the phase that does is named in the same row) --> |
| `test/plugin/pipe-catalog-published.ci` | `ze help command --json` per-command operator list matches the declared shape | 6 <!-- doc-links: ignore (this spec creates this file; the phase that does is named in the same row) --> |

### Interop Tests (Scope: protocol)

Not applicable: no wire protocol behavior changes. The one head addition is
Ze's own answer format and is covered by the round-trip unit test above.

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/command/pipe.go` | catalog with per-operator contract; shape-aware validation; `countItems`, `truncateItems`, `collectPipeMeta` |
| `internal/component/command/pipe_columns.go` | `columnsInChain` intersects instead of assigning |
| `internal/component/command/pipe_records.go` | the record path reads the same catalog and the same shape |
| `internal/component/command/column_order.go` | neighbour of the new shape registry; unchanged mechanism |
| `pkg/plugin/rpc/message.go` | field types on a `tab` head; `checkAnswerType` |
| `cmd/ze/help_command.go`, `cmd/ze/ze_core_pipe.go`, `cmd/ze/ze_core_dispatch.go` | delete three hand-copied lists |
| `internal/component/cli/completer.go` | delete `PipeOperators`, read the catalog |
| `internal/component/cli/*` (`RunCommand`, `matchLocalHandler`) | route a local answer through the pipe layer |
| `internal/plugins/env/register.go`, `internal/component/config/{schema,storage,yang}/cli/register.go`, `internal/component/config/cli/register.go` | the five families that reach no pipe layer |
| `internal/le/wikicatalog/render.go` | delete the operator literal |
| `website/tools/render-cli-catalog.py`, `render-command-equivalents.py`, `render-llms-txt.py` | render the real list |
| `website/tools/page_registry.py` | add `docs/architecture/api/commands.md` to `DOCS_MANIFEST` |
| `internal/le/docvalid/drift.go` | the new gate check |
| `docs/features/formatting.md`, `docs/guide/command-reference.md`, `docs/guide/cli.md`, `docs/guide/isis.md` | take the operator table from a generated include; stop re-listing |
| `ai/rules/cli.md` and its point file | the two rules replacing the universal one |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/component/command/pipe_catalog.go` | the operator contract table and its exported reader |
| `internal/component/command/answer_shape.go` | `RegisterShape` / `ShapeForCommand` over the existing registry |
| `docs/features/pipe-operators.generated.md` | the generated include every doc page links to <!-- doc-links: ignore (this spec creates this file; the phase that does is named in the same row) --> |
| the `.ci` / `.et` files named above | |

### Integration Checklist

- [x] Registered, not hardcoded: shape and address fields declared by the owning command
- [x] The catalog has exactly one copy and every surface derives from it
- [x] A plugin can declare a shape without a core edit
- [x] The generated page and the product cannot disagree without a red gate

### Documentation Update Checklist (BLOCKING)

- [ ] `docs/features/formatting.md` prose corrected after the foreign migration lands: the universal claim, and `match` as line grep
- [x] `docs/guide/cli.md` stops presenting 5 operators as the set, and stops spelling `match <regex>` for a `strings.Contains` match
- [ ] `docs/architecture/api/commands.md` address-field claim corrected after the foreign migration lands; the page is published to the website
- [x] `docs/guide/config-editor.md` says its `|` vocabulary is a different language from the operational one
- [x] `docs/architecture/api/wire-format.md` reviewed; D-3 records why the unchanged answer head does not carry address declarations
- [ ] Release note for R-1 and R-2 lands after the foreign migration owner commits the existing file

## Implementation Steps

Six phases. Each ends with its own commit and its own focused tests green; the
worktree gate runs on a cadence over the resulting commits, not per phase.

| Phase | Delivers | Closes |
|-------|----------|--------|
| 1 | the operator catalog; five hand-copied lists deleted | AC-1 |
| 2 | shape declaration, shape-aware refusal, head field types | AC-2, AC-3, AC-12 |
| 3 | the row-operator defects and composition on all four paths | AC-4, AC-5, AC-6, AC-9 |
| 4 | local handlers through the pipe layer; the dual-registration asymmetry | AC-10, AC-11 |
| 5 | `save`, and `log` on the exec surfaces | AC-7, AC-8 |
| 6 | the published surface, the generators, the gate, the rule | AC-13, AC-14, AC-15, AC-16 |

Phase order is forced: 6 cannot publish a derivation that 1 and 2 have not
built, and 3's composition fixes must land before 4 multiplies the paths that
run a chain.

### Critical Review Checklist

- [x] Registration over hardcoding: no core package enumerates commands or shapes
- [x] No sixth copy of the operator list introduced anywhere
- [x] Every refusal names the operator, the shape, and what to type instead
- [x] The derivation is read from the declaration, never from the head alone (A-2)
- [x] `ai/rules/simplicity.md`: the local-handler conversion is ONE mechanism, not 20 new daemon handlers

### Deliverables Checklist

- [x] Every AC has a named test
- [ ] Each user-visible break has a tracked release note; the existing file is foreign-owned and untracked
- [x] The generated page exists and the gate fails when it drifts

### Security Review Checklist

- [x] `| save` writes only where the invoking user may write, and refuses a path
      it cannot write with a clear message rather than a partial file
- [x] `| save` inside an SSH session cannot write on the daemon filesystem;
      remote expansion refuses it before dispatch
- [x] No operator list is built from user input

### Failure Routing

A defect found in a phase that BLOCKS that phase is fixed there. One that does
not gets a spec and an ask, per `ai/rules/rule-precedence.md`, and its row goes
to `plan/deferrals/cli-pipe-operator-coverage.md`.

## Implementation Notes

### Phase 2, as built

The spec proposed that an undeclared command defaults to `doc` and is refused on
that basis. Building it showed a better answer, and the difference matters:

**The refusal is derived from the ANSWER, not from the declaration.** Every
answer has a shape whether the command declared one or not, so the refusal is
universal on the day it lands, rather than a property of the commands somebody
remembered to annotate. The declaration is still built and still required, for
two things the answer cannot do: publishing what a command supports before it
runs (phase 6), and refusing before dispatch instead of after.

Two shapes carry rows, not one. An ARRAY of objects is the obvious one. A MAP
KEYED BY IDENTITY is the other, and `show bgp peer list` answers it, keyed by
peer address. The old `countItems` happened to answer correctly for it by
unwrapping a single-key map, which the audit recorded as right by accident. It
is a real shape and it is handled deliberately now, including on the way out:
`| first 2` over an identity-keyed answer keeps the keys, because writing an
array back would answer two peers with their addresses gone.

An answer holding SEVERAL row sets is refused, naming them, rather than picking
one.

A refusal that arrives after the command has run reaches the caller as the
formatted string, so `command.IsPipeError` marks it and `ze pipe` exits
non-zero. Known Limitations records the surfaces that still exit 0.

### Phase 6, as built, and why it ran before 4 and 5

Phase order was stated as forced only in one direction: 6 cannot publish a
derivation that 1 and 2 have not built, and 3 must land before 4 multiplies the
paths that run a chain. Nothing forces 4 or 5 ahead of 6, and 6 is the
deliverable the owner named, so it ran third.

What it publishes, per command:

- the operators that ALWAYS apply, and separately the ones that apply only when
  the answer carries rows. That split is what a caller acts on.
- the shape the command declared, absent when it declared none.
- the named chains (aliases), which `ze help command --json` had never read, so
  they published on `show command help` alone.

`global-pipes` is gone. It was a boolean that named no operator and was set
unconditionally for every command carrying a wire method.

**Two honesty rules the building forced, both narrowing what is published:**

`resolve` and `origin` are published for a command only when it DECLARES a field
holding an IP address. `show bgp` declares `address` and publishes them; nothing
else does, so nothing else publishes them. Publishing them everywhere would have
asserted support nothing could honor, which is the failure this surface exists
to end.

An undeclared command publishes its row operators as `with-rows` rather than as
supported. That is the truth: the operator is applied to the answer in hand and
refused by name when it has none. 232 of 252 commands are undeclared today, and
the published page says so per command rather than averaging it away.

**The gate is exact.** `docs/features/pipe-operators.generated.md` is written by
the same call the checker compares against, so a description or an argument kind
that changes in the catalog and not on the page reddens as surely as a missing
operator. Mutation-proven: renaming `fill` to `filll` on the page reddens
`./le docvalid doc-drift` by name.

### Phase 5, as built

`save` did not exist. Nothing in the CLI wrote an answer to a file: a caller
redirected stdout from the shell, which works for `ze cli -c` and an exec
channel and is unavailable inside an interactive session, where the answer is
drawn to a terminal and never reaches a pipe. That session is where it was
needed.

**It is refused where the DAEMON expands the chain, by name.** The daemon
expands for whoever connected, so a save honored on the SSH exec channel or any
web surface would write on the daemon's filesystem, with the daemon's
privileges, at a path the remote caller chose. That is a write primitive handed
to anyone who can reach the CLI. The split is an explicit pair of entry points
rather than a flag, and the REMOTE one keeps the existing name, so the safe
behaviour is what an unexamined call gets.

It writes the answer after the whole chain, wherever `| save` sits in it,
because the configured default format is appended to the END of a chain naming
none: a save applied in place would write the dispatcher's JSON while the
terminal showed something else. The write is atomic and the file is `0600`.

**AC-8 took its second branch: `log` left the global class.** It was published
as owed by every command and did nothing on either surface a tool author uses.
It acts on a SEQUENCE of answers, so for a command that answers once there is no
second update to append, and no implementation could change that. It is now its
own class, published per command as "while the command keeps answering". The
owner's taxonomy said the same thing: the global class is formatting and saving,
and `log` is neither.

### Phase 4, as built so far, and what is still owed

The mechanism is built and one family uses it. Four families are not converted
yet, and that is the honest state: AC-10 is NOT met.

`ze <verb> ... | json` was never the gap: the shell eats the `|` before ze sees
it. The gap is `ze cli -c "show env list | json"`, which answered
`unknown command`, and the client is where it is fixed. The client has the same
registry compiled in, so a command served in that process needs no daemon at
all: it is answered BEFORE credentials are loaded, which is why the chain works
with nothing listening.

- `registry.LocalDataHandler` answers with DATA rather than printing.
  `RegisterLocalData` also registers the plain handler, built from the same data
  handler, so `ze <verb>` and `ze cli -c` render through one path and cannot
  drift apart. That is AC-11's mechanism.
- `command.ServeLocal` runs the operator's chain over the payload.
- `show env list`, `show env get` and `show env registered` are converted, with
  a declared shape and column order.

**AC-10 is met.** The config family landed on 2026-08-23 and it was the last.
Three of its six converted and three deliberately did not; the reading that
decided which is below, kept because it is the answer rather than a plan.

| Command | Answer today | Disposition |
|---------|--------------|-------------|
| `show config dump` | the fully resolved config tree, already JSON | CONVERT. It is one nested document, so it declares `doc`; check that against what it actually emits, the way `show yang tree` had to be |
| `show config history` | snapshots with timestamps and commit messages | CONVERT, rows. Needs a file argument and an editor handle (`cli.NewEditorWithStorage`), so its data function owns that lifecycle |
| `show config list` | `[data] <key>` and `[fs] <path>` lines from two sources | CONVERT, rows of `{source, path}`. The bracket prefix becomes a field |
| `show config cat` | the configuration TEXT of one snapshot | LEAVE. The text is the answer, as with `show data cat` |
| `show config fmt` | the config pretty-printed | LEAVE. The formatting IS the answer; a record of it would be a record of nothing |
| `show config diff` | a rendered diff | LEAVE for now, and it is the one worth revisiting. A structured diff (per-change records) would genuinely serve a tool, but it is a feature rather than a conversion: nothing in the tree emits one, so it would be designed here rather than lifted, and it deserves its own spec |

So three convert, three do not, and one of the three that do not is a real
feature request in disguise.

**How the config family went, since it was the one flagged as riskiest.**
`show config dump` reuses the map `--json` already emitted: `resolveDump` is
that path lifted into a function both spellings call, so they cannot disagree.
`history` and `list` needed payloads written, and both turned a printed prefix
into a FIELD: `[data] <key>` became `{source, path}`, and the `draft` line
printed above the revisions became a row like any other, so a row operator can
reach it.

**It also exposed the last gap between the published surface and the runtime,
which this spec then closed.** `show config dump` answers a nested
configuration tree, and a tree whose one top-level key holds a map of maps is
indistinguishable from rows keyed by identity. So the ANSWER said rows,
`| first 1` was accepted, and it returned a fragment of the config, while
`ze help command --json` published nine operators for it from the DECLARED
shape. `validateDeclaredShape` refuses an operator the declared shape cannot
support before the command runs, which is what AC-3 asked for and what makes the
catalog true rather than aspirational. An undeclared command is still left to
its answer, so the refusal stays universal rather than becoming a property of
the commands somebody annotated.

Converted: env (3), schema (5), storage `list` and `registered` (2), yang `tree`
and `completion` (2). Twelve of twenty.

**The published surface no longer claims what it cannot honor.** A command the
client serves in its own process reaches the pipe layer only if it answers with
DATA; one that keeps a plain printing handler reaches none, whatever YANG
declares, because the local handler wins over the daemon dispatch. `ze help
command --json` now publishes zero operators for those, and 34 commands
publish zero.

**Two commands are deliberately NOT converted, and that is an answer rather
than a gap.** `show data cat` returns the BYTES of one stored file, which may be
YAML, JSON, a certificate or a binary blob. Those bytes are the answer; wrapping
them in a record would corrupt the one use the command has, and no pipe operator
has anything to do with them. It keeps its plain handler and the published page
says it reaches no pipe layer, which is true rather than an omission. A command
whose answer is a byte stream is the boundary of this spec, not a case it
missed.

`show yang doc` is the second. It renders documentation PROSE for a reader, it
has no `--json` path to lift, and the same facts already reach a machine as
structured data through `ze help command --json`. A second record for them would
be a second surface to keep true.

**One shape was declared wrong and running it caught that.** `show yang tree`
was declared `doc`, on the reading that a tree is one document. `formatTreeJSON`
emits a top-level ARRAY, so the answer has rows: its top-level nodes, each
carrying its own children. `| first 1` answers one subtree whole. A declaration
that disagrees with the answer would publish a refusal the product does not
make, which is the same falsehood in a new place. Schema was cheap
because four of its five commands already built a payload for a `--json` flag,
so the data function was lifted out of the printer rather than invented, and the
two spellings of each answer cannot disagree. Storage and yang have no `--json`
path at all, so their payloads have to be designed from what their printers
walk; that is the reason they are separable rather than mechanical.

`show schema protocol` is the first command to declare `doc`. It answers one
document, so every row operator is refused over it BEFORE it runs, which is the
half of the derivation that needs a declaration rather than an answer.

**Two defects this phase found in the earlier phases, both fixed here:**

An EMPTY answer was being refused. `rowsIn` treated `{}`, `[]` and `null` as
"no rows to act on" rather than "zero rows", so a filter that removed every row
turned an empty result into an error. `show bgp peer list | match nothing` must
answer nothing and exit 0, and it does.

Two functional tests were RED across four commits and I did not see them,
because I ran unit tests and lint and not the UI suite that covers the surfaces
I changed. `pipe-operators.ci` asserted the two help headings phase 1 renamed.
`cli-format-default.ci` emptied an answer with `show version | match`, which
phase 3 correctly began refusing. Both are fixed, and the second is strengthened
rather than relaxed: its machine-format arm now parses the answer and asserts it
carries no rows, where it used to assert an empty string.

## Review Gate

The independent gate completed after the earlier self-review recorded below.
Twenty-six passes followed each product fix until the last two passes returned
zero blockers and zero issues.

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/cli-pipe-operator-coverage-55d210d4-6348-48ab-9fd4-30966ec229f4.md` |
| `./le spec session review check` | `OK (14 code files, clean, hashes match)` on 2026-08-28 |
| Rounds | 26. Rounds 15-24 found native renderer and structural validator defects; round 25 verified their fixes and round 26 verified post-clean lint corrections |
| Owner authorisation | `Thomas said ok finish on 2026-08-27` |
| Reviewer lenses | `PipeRound14Coverage`, `PipeRound25Docs`, `PipeRound26Regression` |

The self-review and its two findings remain below as historical evidence. The
independent review starts at “Independent review, round 1”.

### Findings

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| I-1 | ISSUE | `show bgp rib` had TWO row orderings for one command. `jsonTerminal.drain` re-sorted by peer, direction, family and prefix, while the streaming path yields the sources' own order. Which one a caller got depended only on whether `\| json` was typed, and the streaming path cannot match a global sort because sorting a stream means holding every row | FIXED. The terminal no longer re-sorts; both paths yield the sources' order, which is deterministic because each source sorts its PEER LIST at construction. `TestShowPipelineOrdersTheSameWithAndWithoutATerminal` compares the two answers row for row and is mutation-proven: re-adding a sort to the terminal reddens it by name |
| N-1 | NOTE | `rowSet` treats a map whose every value is an object as identity-keyed rows. That is a heuristic, and `show config dump` is the case where it is wrong: a configuration tree looks exactly like that | Answered by `validateDeclaredShape` rather than by a better heuristic. A command that knows it holds one document says so, and the declaration wins over the guess |
| N-2 | NOTE | `sortRouteRows` was added, then removed within the same spec | Recorded because the reason is worth keeping: it was correct while the terminal was the only path, and became a divergence the moment a second path existed. The peer-list sort that replaced it is what makes both paths deterministic |
| N-3 | NOTE | The `save` operator writes a file, and its refusal on daemon-expanded chains is enforced at ONE call site pair | The safe form keeps the existing name, so an unexamined caller gets the refusal. A new remote surface that copies the local entry point by name would defeat it, and nothing gates that |
| N-4 | NOTE | 232 of 252 commands declare no shape | Deliberate. An undeclared command is refused from its answer, so the refusal is universal on day one rather than a property of the annotated set. The published page says `with-rows` for those rather than claiming support |

**Round 1: 1 ISSUE found, 1 fixed, 0 outstanding.**

### Round 2, over Pre-Commit Verification's fix

Round 1 read the diff. Pre-Commit Verification then ran the product, and AC-11
turned out to be false in a way no reading had caught.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| I-2 | ISSUE | `ze show env list` and `ze cli -c "show env list"` did NOT answer the same bytes. The verb path renders through `command.RenderLocalAnswer`, whose `writeAnswer` ends an answer with exactly one newline; the CLI client called `fmt.Println` over the same rendered string, which added a SECOND newline whenever the rendering already ended in one, and every table rendering does. AC-11 asserts the two surfaces answer identically | FIXED. `writeAnswer` is exported as `command.WriteAnswer` and both `ServeLocal` call sites in `internal/component/cli/client/main.go` use it, so the newline policy has one owner. `test/ui/pipe-local-command.ci` section 9 compares the two surfaces byte for byte |

**Round 2: 1 ISSUE found, 1 fixed, 0 outstanding.** Round 2 covered only the
fix and the AC it falsified; it opened nothing new, so the loop ends here.

The finding is worth more than its size. The defect was one call to the wrong
print function, it was invisible to every test in the spec, and it made the one
claim AC-11 exists to make false on the surface a tool author reads. It was
found by running two spellings of one command and comparing the bytes, which is
the cheapest check in this whole spec and was not written until closure.

### The self-review's limit, measured

Round 1 was a self-review and said so. I-2 is the evidence for what that costs:
a defect in the diff Round 1 read, in a file the spec created, falsifying an AC
the spec wrote, and Round 1 did not see it. An independent reviewer might not
have either. What found it was not a better reader, it was EXECUTION.

### What this review did NOT cover

- The `.ci` and `.et` corpus beyond the files this spec changed. The
  shape-change blast radius was found by the pre-push gate, not by reading.
- Any surface that consumes `ze help command --json` outside this repository.
- The website generators' rendered output, which another session holds
  uncommitted.
- Argument handling. `ze show env list --nosuchflag` answers all 96 rows and
  exits 0, on both surfaces. That is outside the operator language, so it was
  recorded in `plan/audit-command-pipe-vs-subcommand.md` rather than fixed
  here.

### Independent review, round 1 (2026-08-26)

Three independent contexts reviewed the 12 implementation and fix commits
through logic, security, and public-contract lenses. The scoped patch contains
61 files and 5,450 changed lines. The main session verified each finding
against the current producer source.

Artifact:
`tmp/review/cli-pipe-operator-coverage-55d210d4-6348-48ab-9fd4-30966ec229f4.md`.
The recorder could not determine the running model, so the review-model
boundary is unchecked.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR-1 | BLOCKER | `recordsLast` retains one copied record per result up to an unbounded user value. An SSH user can make daemon memory grow with the full RIB (`internal/component/command/pipe_records.go` `recordsLast`; `pkg/plugin/rpc/types.go` `HeldRecords`) | FIXED |
| IR-2 | ISSUE | `parsePipeOps` discards surplus arguments for no-argument and single-argument operators, so malformed chains execute instead of failing (`internal/component/command/pipe.go` `parsePipeOps`) | FIXED |
| IR-3 | ISSUE | `log` is accepted as a no-op on one-shot commands, although its contract says that it applies only to streams (`internal/component/command/pipe.go` `ApplyPipes`) | FIXED |
| IR-4 | ISSUE | Pipe metadata overwrites a legitimate answer field named `pipe` (`internal/component/command/pipe.go` `injectPipeMeta`) | FIXED |
| IR-5 | ISSUE | An unknown `display` field returns the complete record. A typo fails open and can expose fields that the operator meant to remove (`internal/component/command/pipe_columns.go` `selectRecord`) | FIXED |
| IR-6 | ISSUE | The YANG local-data handlers bypass the existing argument parser. `dataTree` loses conflict and unknown-option errors, and `dataCompletion` ignores `--min-prefix` (`internal/component/config/yang/cli/yang_data.go` `dataTree`, `dataCompletion`) | FIXED |
| IR-7 | BLOCKER | `TestShowPipelineOrdersTheSameWithAndWithoutATerminal` cannot detect the removed global sort. Its fixture is already ordered by the same keys (`internal/component/bgp/plugins/rib/rib_pipeline_show_stream_test.go` `TestShowPipelineOrdersTheSameWithAndWithoutATerminal`) | FIXED |
| IR-8 | ISSUE | An empty keyed row set such as `{"peers":{}}` becomes one outer row, so a zero-peer answer counts as one (`internal/component/command/answer_shape.go` `rowSet`, `rowsInKeyed`; `internal/component/bgp/plugins/cmd/peer/peer.go` `handleBgpPeerList`) | FIXED |
| IR-9 | BLOCKER | `operatorsFor` suppresses operators for a plain local handler even when the CLI reaches a daemon data handler. `show version` is published with no operators although its daemon path pipes (`cmd/ze/help_command.go` `operatorsFor`; `internal/component/cmd/show/show.go` `handleShowVersion`) | FIXED |
| IR-10 | BLOCKER | The documentation drift gate compares only the global generated operator table. It does not verify per-command wiki or website output against `operatorsFor`, so AC-15 is not enforced (`internal/le/docvalid/drift.go` `checkPipeOperatorReference`) | FIXED |
| IR-11 | ISSUE | `ClassStream` is dropped or mislabeled by `pipeUsage`, the wiki generator, the command-equivalents generator, and the CLI rule. The LLM catalog removes its qualifier (`cmd/ze/ze_core_pipe.go` `pipeUsage`; `internal/le/wikicatalog/render.go` `render_detail`; `website/tools/render-command-equivalents.py` `split_operators`; `website/tools/render-llms-txt.py` `command_meta`) | FIXED |
| IR-12 | ISSUE | `ze cli --help` keeps a second six-operator list, omits catalog entries, and still describes row operators as line operations (`internal/component/cli/client/main.go` `usage`) | FIXED |
| IR-13 | BLOCKER | Address declarations neither gate undeclared commands nor constrain enrichment to declared fields. `show version \| origin` is accepted, while declared `show bgp \| resolve` also enriches `router-id` (`internal/component/command/pipe.go` `validateDeclaredShape`; `internal/component/command/pipe_resolve.go` `resolveJSON`; `internal/component/command/pipe_origin.go` `originJSON`) | FIXED |
| IR-14 | ISSUE | Pipe metadata is injected before the first format operator. A later row operator treats the metadata slice as a second row set and refuses or truncates the wrong set (`internal/component/command/pipe.go` `ApplyPipes`; `internal/component/command/answer_shape.go` `rowsInKeyed`) | FIXED |
| IR-15 | ISSUE | `outboundSource.Next` ranges nested family and route maps directly. Advertised RIB output and `first 1` are nondeterministic (`internal/component/bgp/plugins/rib/rib_pipeline.go` `outboundSource.Next`) | FIXED |
| IR-16 | ISSUE | Streaming `save` writes each event before the default renderer and atomically overwrites the same path. A multi-event monitor leaves only the last raw event (`internal/component/cli/client/main.go` `cliClient.StreamMonitor`; `internal/component/command/pipe_save.go` `applySaves`) | FIXED |
| IR-17 | ISSUE | `operatorsFor` publishes `save` as `always`, but remote daemon-expanded chains intentionally refuse it. The machine contract omits this surface restriction (`cmd/ze/help_command.go` `operatorsFor`; `internal/component/command/pipe_save.go` `validateSaveOps`) | FIXED |
| IR-18 | BLOCKER | No functional test drives allowed interactive `save` or refused SSH/web `save`. The security guard is tested only below those entry points (`internal/component/command/pipe_save.go` `validateSaveOps`, `saveAnswer`) | FIXED |
| IR-19 | BLOCKER | The tree has 15 `MustRegisterLocalData` commands, but functional pipe coverage drives only four. Eleven converted handlers have no entry-to-pipe proof, so AC-10 is not covered (`internal/component/command/local_data.go` `ServeLocal`; `test/ui/pipe-local-command.ci`) | FIXED |

**Independent round 1: 7 BLOCKER, 12 ISSUE, 0 outstanding.** All 19
findings were fixed before round 2.

### Independent review, round 2 (2026-08-27)

Three fresh contexts reviewed fix commit `e25b12b49` through logic, security,
and public-contract lenses. Repository wiring and test-relaxation checks were
clean before the pass. The main session verified each finding against its
producer.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR2-1 | BLOCKER | Interactive SSH runs `cli.Model` inside the daemon, but `Model.executeOperationalCommand` uses the local save validator. An SSH PTY user can write any daemon-writable path (`internal/component/cli/model_mode.go` `Model.executeOperationalCommand`; `internal/component/ssh/ssh.go` `Server.teaHandler`) | FIXED |
| IR2-2 | BLOCKER | `functionalLocalDataInvocations` prefers an ignored draft test when present, so the ratchet can report coverage that no normal suite runs (`internal/component/command/registry/local_data_functional_coverage_test.go` `functionalLocalDataInvocations`) | FIXED |
| IR2-3 | BLOCKER | Committed functional tests do not drive the new bounds, one-shot stream refusal, display typo refusal, remote save, traceroute save, `show version` publication, or derived help through their user entry points | FIXED |
| IR2-4 | BLOCKER | `checkPublishedCommandSurfaces` returns clean when no sibling website or wiki catalog exists, so CI can execute no per-command comparison (`internal/le/docvalid/drift.go` `checkPublishedCommandSurfaces`) | FIXED |
| IR2-5 | BLOCKER | The drift gate compares only website command JSON. Stale or incorrect CLI HTML, Markdown, equivalents, and `llms.txt` can remain green (`internal/le/docvalid/drift.go` `compareWebsiteCommandCatalog`) | FIXED |
| IR2-6 | ISSUE | Public renderers treat `always` and `local-only` as alternatives. `save` loses its independent `always` answer qualifier (`cmd/ze/help_command.go` `splitOperators`; website and wiki renderer equivalents) | FIXED |
| IR2-7 | ISSUE | The primary website CLI page does not render `answer-shape` or `address-fields` (`website/tools/render-cli-catalog.py` `render_pipe_details`) | FIXED |
| IR2-8 | ISSUE | `ze cli --help` still copies five format values and omits valid `raw` (`internal/component/cli/client/main.go` `usage`) | FIXED |
| IR2-9 | ISSUE | Verbose command help builds the `pipes:` heading and resets the buffer without writing it (`cmd/ze/help_command.go` `printCommandVerbose`) | FIXED |
| IR2-10 | ISSUE | Command-equivalent detail pages omit command-specific filters and pipe aliases (`website/tools/render-command-equivalents.py` `render_ze_detail`, `render_detail_markdown`) | FIXED |
| IR2-11 | BLOCKER | `foldFilters` removes RIB `last` and `count` before catalog argument validation. Oversized counts and surplus arguments bypass the new guards (`internal/component/command/pipe.go` `foldFilters`, `processPipesDefaultFormat`) | FIXED |
| IR2-12 | BLOCKER | The undeclared-address guard rejects standalone `ze pipe resolve` over stdin although the command still publishes and tests that operator (`cmd/ze/ze_core_pipe.go` `runPipe`; `internal/component/command/pipe.go` `validateDeclaredShape`) | FIXED |
| IR2-13 | ISSUE | Address enrichment accepts positional table rows but transforms only map keys, so positional arrays remain unchanged (`internal/component/command/pipe_records.go` `applyRecordOp`; `pipe_resolve.go` `resolveJSON`; `pipe_origin.go` `originJSON`) | FIXED |
| IR2-14 | BLOCKER | Positional display narrows row values but SSH writes the original field schema in the streamed answer head (`internal/component/command/render_records.go` `RenderRecords`; `internal/component/ssh/answer.go` `writeExecRecords`) | FIXED |
| IR2-15 | ISSUE | The record path applies row operators before formats regardless of chain order. `text \| first 1` changes behavior at the streaming threshold (`internal/component/command/pipe_records.go` `applyPipesRecords`; `internal/component/command/render_records.go` `RenderRecords`) | FIXED |
| IR2-16 | ISSUE | Command-equivalent detail pages carry generic operators but omit command-owned `pipes` and `pipe-aliases` (`website/tools/render-command-equivalents.py`) | FIXED |

**Independent round 2: 8 BLOCKER, 8 ISSUE, 0 outstanding.** All 16
findings were fixed before round 3.

### Independent review, round 3 (2026-08-27)

Three new contexts reviewed fix commit `e112c1f67` through logic, security,
and public-contract lenses. The main session verified each report against the
named producer. One reported runtime defect was already fixed in the exact
reviewed commit and is recorded as dismissed.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR3-1 | BLOCKER | SSH PTY monitor ping and traceroute start registered views before the model's filesystem-authority guard. Their local stream API can stage and commit `save` with daemon authority (`internal/component/cli/model_keys.go` `Model.handleEnter`; `model_ping.go` `Model.startPingMonitorPiped`; `model_traceroute.go` `Model.startTraceroutePiped`) | FIXED |
| IR3-2 | BLOCKER | Raw SSH streaming exec dispatches the full pipe-bearing input before remote validation. `log` is not applied and `save` is not refused before handler dispatch (`internal/component/ssh/ssh.go` `Server.execMiddleware`) | FIXED |
| IR3-3 | BLOCKER | The hub authority test reaches only the command-model fallback. No test discriminates the editor-capable production `NewModel` branch (`cmd/ze/hub/session_factory.go` `buildSessionModelFactory`) | FIXED |
| IR3-4 | BLOCKER | Local-data `ze cli -c --format raw` passes `raw` as a default and falls back to text. Invalid explicit formats fail open to text on the same path (`internal/component/cli/client/main.go` `runBGP`) | FIXED |
| IR3-5 | BLOCKER | Without a sibling wiki checkout, docvalid neither runs nor structurally validates `gen_wiki_commands.py` (`internal/le/docvalid/drift.go` `checkPublishedCommandSurfaces`) | FIXED |
| IR3-6 | BLOCKER | The local-data functional ratchet scans comments and dead Python text, so a non-executed call can satisfy AC-10 (`internal/component/command/registry/local_data_functional_coverage_test.go` `functionalLocalDataInvocations`) | FIXED |
| IR3-7 | BLOCKER | The live remote scenario does not drive an authenticated production SSH PTY, so the authority boundary remains functionally unproven (`test/plugin/pipe-review-remote-contracts.ci`) | FIXED |
| IR3-8 | BLOCKER | Primary CLI HTML and Markdown drift validators omit command-owned filters and aliases (`internal/le/docvalid/drift.go` `validatePrimaryCommandContract`, `validatePrimaryMarkdownContract`) | FIXED |
| IR3-9 | BLOCKER | Generated-surface validators require catalog operators but never reject extra names absent from the catalog (`internal/le/docvalid/drift.go` `validateGeneratedCommandSurfaces`) | FIXED |
| IR3-10 | BLOCKER | Generated operator class and description are not compared with the catalog (`internal/le/docvalid/drift.go` `validatePrimaryCommandContract`) | FIXED |
| IR3-11 | BLOCKER | A format-before-transform record chain always materializes every source record. `ndjson` followed by bounded line transforms loses record-path cancellation and bounded retention (`internal/component/command/render_records.go` `RenderRecords`) | FIXED |
| IR3-12 | BLOCKER | Structured transforms after incompatible rendered formats are accepted as inert successes instead of refused by operator and format (`internal/component/command/pipe.go` `ApplyPipes`; `render_records.go` `formatBeforeDataTransform`) | FIXED |
| IR3-13 | ISSUE | Address enrichment overwrites existing derived sibling fields and iterates unsorted source keys, so it can erase data or produce order-dependent derived fields (`internal/component/command/pipe_resolve.go` `resolveJSON`; `pipe_origin.go` `originJSON`) | FIXED |
| IR3-D1 | DISMISSED | The report said standalone post-input refusals reached stdout with exit 0. Exact commit `e112c1f67` already checks `command.IsPipeError`, writes stderr, and exits 1 (`cmd/ze/ze_core_pipe.go` `runPipe`) | DISMISSED |

**Independent round 3: 12 BLOCKER, 1 ISSUE, 0 outstanding; 1 reported
finding dismissed against the producer.** All 13 verified findings are fixed.
The next independent round reviews their exact committed population.

### Independent review, round 4 (2026-08-27)

Three new contexts reviewed fix commit `c577d13b9` through logic, security,
and public-contract lenses. The main session verified each finding against its
producer.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR4-1 | BLOCKER | The AC-10 ratchet treats triple-quoted Python string content as executable top-level calls and completion markers (`internal/component/command/registry/local_data_functional_coverage_test.go` `parseFunctionalLocalDataInvocations`) | FIXED |
| IR4-2 | BLOCKER | Generated-surface validators inspect only the first operator group and accept a duplicate group that names catalog-absent operators (`internal/le/docvalid/drift.go`) | FIXED |
| IR4-3 | BLOCKER | `ServeLocal` invokes the local producer before it validates an explicit pipe refusal, so source work and producer errors can precede AC-3 (`internal/component/command/local_data.go` `ServeLocal`) | FIXED |
| IR4-4 | BLOCKER | Positional record enrichment still appends duplicate derived columns and can overwrite producer values (`internal/component/command/pipe_records.go` `recordsPositionalAddressTransformed`) | FIXED |
| IR4-5 | BLOCKER | NDJSON line transforms collapse short answers with faults but stream long answers line by line, making output threshold-dependent (`internal/component/command/render_records.go` `RenderRecords`) | FIXED |
| IR4-6 | BLOCKER | NDJSON `last` measures positional array bytes before expansion to field-named objects, so the retained-byte bound applies to the wrong representation (`internal/component/command/pipe_records.go`) | FIXED |
| IR4-7 | BLOCKER | NDJSON `match` can filter the fault from a malformed positional row before the schema violation reaches the caller (`internal/component/command/pipe_records.go` `recordsMatchingRenderedJSON`) | FIXED |
| IR4-8 | BLOCKER | Raw SSH stream formatting writes `pipe error:` sentinels to stdout and returns success instead of stderr and nonzero (`internal/component/ssh/ssh.go` `streamPipeWriter.writeEvent`) | FIXED |
| IR4-9 | BLOCKER | The production PTY ping and traceroute refusal cases also pass through the ordinary fallback, so they do not prove either registered piped view was reached (`test/plugin/pipe-review-remote-contracts.ci`) | FIXED |
| IR4-10 | ISSUE | The raw stream formatter test uses idempotent pretty JSON and cannot detect applying the formatter twice (`internal/component/ssh/save_entry_test.go`) | FIXED |

**Independent round 4: 9 BLOCKER, 1 ISSUE, 0 outstanding.** All 10
findings are fixed. The next independent round reviews their exact commit.

### Independent review, round 5 (2026-08-27)

Three new contexts reviewed fix commit `ffa2697d5` through logic, security,
and public-contract lenses. The security lens returned clean. The main session
verified the five remaining findings against their producers.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR5-1 | BLOCKER | Positional `count \| ndjson` leaves the source schema attached to the count document, so canonicalization emits a positional-row fault (`internal/component/command/pipe_records.go`) | FIXED |
| IR5-2 | BLOCKER | The AC-10 ratchet validates `run.py` but does not prove the `.ci` command executes it instead of a second payload (`internal/component/command/registry/local_data_functional_coverage_test.go`) | FIXED |
| IR5-3 | BLOCKER | Rendered validators select the first per-command row or section and accept a duplicate container for the same command with catalog-absent operators (`internal/le/docvalid/drift.go`) | FIXED |
| IR5-4 | BLOCKER | HTML group scanners return prior valid groups when a later duplicate group is unterminated, accepting malformed catalog-absent output (`internal/le/docvalid/drift.go`) | FIXED |
| IR5-5 | ISSUE | The AST fixture changed the post-completion fake call from a literal to a dynamic value, losing discrimination for late literal evidence (`internal/component/command/registry/local_data_functional_coverage_test.go`) | FIXED |

**Independent round 5: 4 BLOCKER, 1 ISSUE, 0 outstanding.** All five
findings are fixed. The next independent round reviews their exact commit.

### Independent review, round 6 (2026-08-27)

Two new contexts reviewed fix commit `2df4a4141` through logic and
public-contract lenses. The main session verified all findings against their
producers. The security lens did not rerun because round 5 was clean and this
commit changed no security surface.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR6-1 | BLOCKER | Record-path row transforms after `count` succeed on the count document while the document path refuses them (`internal/component/command/pipe_records.go`) | FIXED |
| IR6-2 | ISSUE | The count/NDJSON test compares two record paths and does not prove equivalence with the document runner (`internal/component/command/render_records_test.go`) | FIXED |
| IR6-3 | BLOCKER | `ndjson \| count` over one valid JSON line is parsed as a document and refused on the document path, but counted as one line on the record path (`internal/component/command/pipe.go`) | FIXED |
| IR6-4 | BLOCKER | The AC-10 ratchet accepts malformed `stop` and `api` directives that the real runner rejects | FIXED |
| IR6-5 | BLOCKER | The AC-10 ratchet accepts invalid executable fields that the real runner rejects | FIXED |
| IR6-6 | BLOCKER | The AC-10 ratchet accepts unresolved stdin bindings | FIXED |
| IR6-7 | BLOCKER | The AC-10 ratchet ignores stop steps that can preempt the `run.py` launch | FIXED |
| IR6-8 | BLOCKER | Wiki validation ignores malformed same-command detail headings | FIXED |
| IR6-9 | BLOCKER | Primary HTML row scanning can pair a missing close with the next command row | FIXED |
| IR6-10 | BLOCKER | Equivalent HTML scanning can pair a missing Ze close with the next article | FIXED |
| IR6-11 | BLOCKER | Primary Markdown ignores malformed same-command rows | FIXED |
| IR6-12 | BLOCKER | `llms.txt` validation ignores malformed same-command rows | FIXED |
| IR6-13 | BLOCKER | Equivalent Markdown ignores malformed duplicate Ze headings | FIXED |
| IR6-14 | BLOCKER | Primary HTML ignores partial same-command row openers | FIXED |
| IR6-15 | BLOCKER | Equivalent HTML ignores partial Ze-article openers | FIXED |
| IR6-16 | ISSUE | Markdown heading counting treats fenced-code examples as command containers | FIXED |
| IR6-17 | BLOCKER | The AC-10 ratchet accepts skip directives that prevent `run.py` execution | FIXED |
| IR6-18 | BLOCKER | The AC-10 ratchet ignores malformed non-command directives that make the runner refuse the scenario | FIXED |
| IR6-19 | BLOCKER | Equivalent HTML does not bind the selected Ze article to the expected command path | FIXED |

**Independent round 6: 17 BLOCKER, 2 ISSUE, 0 outstanding.** All 19
findings are fixed. `review_gate.py` still requires owner authorization to
record round 6 or run the final clean pass.

### Independent review, round 7 (2026-08-27)

Three owner-authorized contexts reviewed fix commit `44a167a09` through
semantics, coverage, and rendered-contract lenses. The main session verified
all findings against their producers.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR7-1 | BLOCKER | Folded command filters can remove `count` before count-follower validation, allowing source work before AC-3 refusal (`internal/component/command/pipe.go` `foldFilters`) | FIXED |
| IR7-2 | BLOCKER | AC-10 does not require the top-level `OK:` marker to be observed, so exit-only or assertion-free scenarios can pass without executing AST-counted calls | FIXED |
| IR7-3 | BLOCKER | AC-10 accepts malformed expectation fields validated only during runner execution | FIXED |
| IR7-4 | BLOCKER | The isolated candidate path can satisfy `needs-path` when the real repository would skip the scenario | FIXED |
| IR7-5 | BLOCKER | Equivalent HTML ignores malformed same-path Ze articles whose Registry-path markup is noncanonical (`internal/le/docvalid/drift.go`) | FIXED |
| IR7-6 | BLOCKER | Markdown command identity parsers assume one-backtick code-span delimiters and ignore valid matching multi-backtick spans | FIXED |
| IR7-7 | BLOCKER | Invalid backtick fence openers can hide active command containers | FIXED |
| IR7-8 | BLOCKER | Equivalent Markdown ignores duplicate Ze headings with inline comments or other non-space suffixes | FIXED |
| IR7-9 | ISSUE | HTML capture treats valid nested rows or articles as peer-container starts and rejects valid postprocessed markup | FIXED |

**Independent round 7: 8 BLOCKER, 1 ISSUE, 0 outstanding.** All nine
findings are fixed. The next authorized review covers their exact commits.

### Independent review, round 8 (2026-08-27)

Three owner-authorized contexts reviewed the Round-7 semantics, coverage, and
migrated rendered-contract fixes. The semantics lens returned clean. The main
session verified the five remaining findings against their producers.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR8-1 | BLOCKER | AC-10 still equates AST call syntax with execution, so helper rebinding or a spoofed marker can pass without real `ze cli -c` calls | FIXED |
| IR8-2 | BLOCKER | Migrated equivalent HTML ignores malformed same-path Ze articles when noncanonical Registry-path markup prevents identity recovery | FIXED |
| IR8-3 | BLOCKER | Migrated equivalent Markdown ignores duplicate Ze headings with text-free inline HTML that renders the same identity | FIXED |
| IR8-4 | ISSUE | HTML capture treats valid direct child articles as peer containers and rejects valid nested postprocessing | FIXED |
| IR8-5 | ISSUE | Markdown code-span identity uses `strings.Fields` instead of CommonMark whitespace normalization | FIXED |

**Independent round 8: 3 BLOCKER, 2 ISSUE, 0 outstanding.** All five
findings are fixed. The next authorized review covers their exact files.

### Independent review, round 9 (2026-08-27)

Two owner-authorized contexts reviewed the compiled runtime-evidence path and
the final migrated rendered-contract parser. The main session verified all
findings against their producers.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR9-1 | BLOCKER | Compiled coverage fixtures use the inherited working directory and can overwrite or race checkout files | FIXED |
| IR9-2 | BLOCKER | The compiled helper accepts extra schema protocol fields | FIXED |
| IR9-3 | BLOCKER | The compiled helper drops malformed YANG tree nodes and children | FIXED |
| IR9-4 | ISSUE | The compiled helper accepts completion collisions without array-valued siblings | FIXED |
| IR9-5 | BLOCKER | The compiled helper accepts extra count-result and pipe-metadata fields | FIXED |
| IR9-6 | BLOCKER | Runtime evidence markers are prefix-spoofable for a new registration path | FIXED |
| IR9-7 | BLOCKER | The migrated gate bypasses its native renderer seam, leaving renderer failure and mutation tests vacuous | FIXED |
| IR9-8 | BLOCKER | Registry identity recovery misses visibly matching labels or later code descendants | FIXED |
| IR9-9 | BLOCKER | Markdown container identity does not parse code spans across physical lines | FIXED |
| IR9-10 | BLOCKER | Native rendering silently overwrites command detail files on slug collisions | FIXED |
| IR9-11 | BLOCKER | Aggregate rendered surfaces do not reject command containers absent from the live catalog | FIXED |
| IR9-12 | ISSUE | Fence closing accepts non-ASCII whitespace forbidden by CommonMark | FIXED |
| IR9-13 | BLOCKER | CRLF in a code span normalizes to two spaces instead of one line ending | FIXED |
| IR9-14 | ISSUE | HTML shape and address checks reject benign postprocessing because they compare serialized fragments | FIXED |
| IR9-15 | BLOCKER | Truncated Ze article classes are not treated as malformed candidates | FIXED |
| IR9-16 | ISSUE | Nested command articles contaminate their parent Registry-path scan | FIXED |

**Independent round 9: 12 BLOCKER, 4 ISSUE, 0 outstanding.** All 16
findings are fixed. The next authorized review covers their exact files.

### Independent review, round 10 (2026-08-27)

Two owner-authorized contexts reviewed the committed self-contained runtime
proof and migrated rendered gate. The main session verified all findings
against their producers.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR10-1 | BLOCKER | The runtime ratchet reads the draft path instead of the committed scenario | FIXED |
| IR10-2 | BLOCKER | The interpreted proof does not reject rebinding of `run` or `subprocess.run` | FIXED |
| IR10-3 | BLOCKER | Indirect stdout aliases and dynamically assembled markers can spoof interpreted evidence | FIXED |
| IR10-4 | BLOCKER | Interpreted tree traversal accepts non-array children | FIXED |
| IR10-5 | ISSUE | Interpreted completion collisions accept non-array siblings | FIXED |
| IR10-6 | ISSUE | Interpreted row helpers accept non-object entries | FIXED |
| IR10-7 | BLOCKER | The interpreted scenario can substitute a fake `ze` through `PATH` | FIXED |
| IR10-8 | BLOCKER | Aggregate rendered surfaces do not reject command containers absent from live | FIXED |
| IR10-9 | BLOCKER | Markdown command identities do not span physical lines and CRLF normalizes incorrectly | FIXED |
| IR10-10 | ISSUE | Fence closers accept non-ASCII whitespace | FIXED |
| IR10-11 | BLOCKER | Markdown heading identity ignores visible inline emphasis and links | FIXED |
| IR10-12 | BLOCKER | Registry identity markup is accepted without explicit node closure | FIXED |
| IR10-13 | BLOCKER | Truncated Ze article classes are not malformed candidates | FIXED |
| IR10-14 | ISSUE | Nested articles contaminate parent Registry scans | FIXED |
| IR10-15 | BLOCKER | HTML values use serialized fragments instead of exact parsed visible values with closure | FIXED |
| IR10-16 | BLOCKER | Metadata fields lack exact zero-or-one cardinality and duplicate-value rejection | FIXED |
| IR10-17 | BLOCKER | Primary and LLM surfaces do not compare every visible path, mode, and description value | FIXED |

**Independent round 10: 13 BLOCKER, 4 ISSUE, 0 outstanding.** All 17
findings are fixed. The next authorized review covers their exact files.

### Independent review, round 11 (2026-08-27)

Two owner-authorized contexts reviewed the final compiled coverage and native
rendered-contract fixes. The main session verified all findings against their
producers.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR11-1 | ISSUE | Compiled coverage row conversion silently accepts non-object members when another row matches | FIXED |
| IR11-2 | ISSUE | Compiled coverage accepts non-object completion sibling members | FIXED |
| IR11-3 | BLOCKER | Equivalent indexes do not compare complete exact command identity multisets | FIXED |
| IR11-4 | ISSUE | Equivalent detail Markdown does not validate the visible top-level command heading | FIXED |
| IR11-5 | ISSUE | Visible HTML values do not recursively reject unclosed descendants | FIXED |
| IR11-6 | ISSUE | Equivalent filter and alias details compare serialized fragments instead of structural visible values | FIXED |
| IR11-7 | ISSUE | Native rendering accepts an empty normalized command slug and can overwrite the aggregate index | FIXED |
| IR11-8 | ISSUE | The CommonMark whitespace test does not distinguish correct attribution from broad collapsing | FIXED |
| IR11-9 | ISSUE | The pseudo-fence mutation accepts both zero and two-container failures and is non-discriminating | FIXED |

**Independent round 11: 1 BLOCKER, 8 ISSUE, 0 outstanding.** All nine
findings are fixed. The next authorized review covers their exact files.

### Independent review, round 12 (2026-08-27)

Two owner-authorized contexts reviewed the Round-11 coverage and rendered
surface fixes. Coverage returned clean. The main session verified two
remaining rendered-surface issues.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR12-1 | ISSUE | Wrapped command code spans in equivalent Markdown indexes can evade the identity multiset | FIXED |
| IR12-2 | ISSUE | Semicolons in valid filter or alias descriptions are parsed as extra Markdown detail entries | FIXED |

**Independent round 12: 0 BLOCKER, 2 ISSUE, 0 outstanding.** Both issues are fixed.

### Independent review, round 13 (2026-08-27)

Two owner-authorized contexts reviewed the Round-12 fixes. Coverage returned
clean. The main session verified six rendered-surface findings.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR13-1 | BLOCKER | Postprocessed operator guide rows can hide catalog-absent operators from the raw HTML regex | FIXED |
| IR13-2 | BLOCKER | Wrapped identities on primary Markdown and LLM aggregate surfaces can evade exact population checks | FIXED |
| IR13-3 | BLOCKER | Unmatched CommonMark emphasis markers are discarded and visible drift can pass | FIXED |
| IR13-4 | ISSUE | Wrapped-identity tests do not prove valid wrappers decode successfully | FIXED |
| IR13-5 | BLOCKER | Detail expectation derivation does not apply renderer escaping for literal backslashes and semicolons | FIXED |
| IR13-6 | BLOCKER | Multiline descriptions break one-line primary Markdown and LLM containers | FIXED |

**Independent round 13: 5 BLOCKER, 1 ISSUE, 0 outstanding.** All six findings are fixed.

### Independent review, round 14 (2026-08-27)

Two owner-authorized contexts reviewed the Round-13 fixes. Coverage returned
clean. The main session verified ten CommonMark and native-renderer issues.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR14-1 | ISSUE | CommonMark rule-of-three logic uses the opener's `canOpen` state where `canClose` is required | FIXED |
| IR14-2 | ISSUE | CommonMark punctuation classification incorrectly treats every Unicode symbol as punctuation | FIXED |
| IR14-3 | ISSUE | Alias expansions are backslash-escaped inside code spans and render visibly wrong | FIXED |
| IR14-4 | ISSUE | Existing backslashes are not protected before escaping Markdown table pipes | FIXED |
| IR14-5 | ISSUE | Multiline detail descriptions escape delimiters but still leave physical line breaks | FIXED |
| IR14-6 | ISSUE | LLM alias metadata does not encode semicolon or comma delimiters in expansions | FIXED |
| IR14-7 | ISSUE | Primary Markdown metadata splits commas inside alias code spans | FIXED |
| IR14-8 | ISSUE | Primary Markdown scans contract labels in every table cell instead of the contract cell | FIXED |
| IR14-9 | ISSUE | Aggregate descriptions render active Markdown instead of literal catalog text | FIXED |
| IR14-10 | ISSUE | Detail descriptions leave emphasis, links, and inline HTML active | FIXED |

**Independent round 14: 0 BLOCKER, 10 ISSUE, 0 outstanding.** All ten findings are fixed.

### Independent review, round 15 (2026-08-27)

An owner-authorized independent context reviewed the native renderers and
structural drift validators after Round 14.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR15-1 | BLOCKER | Wiki descriptions render active Markdown instead of literal catalog text | FIXED |
| IR15-2 | BLOCKER | Wiki code values use fixed delimiters that break on embedded backticks | FIXED |
| IR15-3 | BLOCKER | Alias-only and metadata-only wiki entries omit their detail sections | FIXED |
| IR15-4 | BLOCKER | Wiki validation ignores duplicate optional metadata lines | FIXED |
| IR15-5 | BLOCKER | Wiki validation ignores duplicate alias and filter groups | FIXED |
| IR15-6 | BLOCKER | Wiki operator parsing accepts malformed unmatched code spans | FIXED |
| IR15-7 | BLOCKER | Nested HTML command containers evade identity accounting | FIXED |
| IR15-8 | BLOCKER | Primary HTML alias details can be satisfied by an unrelated serialized fragment | FIXED |
| IR15-9 | BLOCKER | Greedy emphasis pairing miscomputes nested CommonMark visible text | FIXED |
| IR15-10 | ISSUE | Unicode-only command paths produce empty renderer slugs | FIXED |
| IR15-11 | ISSUE | Markdown command paths use fixed code-span delimiters | FIXED |

**Independent round 15: 9 BLOCKER, 2 ISSUE, 0 outstanding.** All eleven findings are fixed.

### Independent review, round 16 (2026-08-27)

An owner-authorized independent context reviewed the committed Round-15 fixes.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR16-1 | BLOCKER | CommonMark emphasis parsing permits crossing delimiter pairs removed by the delimiter stack | FIXED |
| IR16-2 | BLOCKER | Primary Markdown leaves table pipes active inside metadata code spans | FIXED |
| IR16-3 | BLOCKER | Live command catalog validation accepts duplicate per-command identities | FIXED |
| IR16-4 | ISSUE | Wiki validation ignores contents entries, group counts, headings, and the final total | FIXED |
| IR16-5 | BLOCKER | ATX headings with tab-delimited closing hashes evade duplicate-section detection | FIXED |
| IR16-6 | ISSUE | Wiki verb labels and anchors render active punctuation without a stable matching fragment | FIXED |

**Independent round 16: 4 BLOCKER, 2 ISSUE, 0 outstanding.** All six findings are fixed.

### Independent review, round 17 (2026-08-27)

An owner-authorized independent context reviewed the committed Round-16 fixes.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR17-1 | BLOCKER | Primary HTML rows without canonical IDs evade command and operator accounting | FIXED |
| IR17-2 | BLOCKER | Primary HTML operator groups ignore trailing sibling values | FIXED |
| IR17-3 | ISSUE | Punctuation-only wiki verbs produce empty contents anchors | FIXED |
| IR17-4 | ISSUE | Equivalent HTML emits `dt` and `dd` groups without a containing `dl` | FIXED |
| IR17-5 | ISSUE | Wiki descriptions do not normalize CRLF or bare CR before row rendering | FIXED |

**Independent round 17: 2 BLOCKER, 3 ISSUE, 0 outstanding.** All five findings are fixed.

### Independent review, round 18 (2026-08-27)

An owner-authorized independent context reviewed the committed Round-17 fixes.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR18-1 | BLOCKER | No-ID command rows with a noncanonical cell count evade candidate accounting | FIXED |
| IR18-2 | BLOCKER | Equivalent command details accept unknown `Pipes, ...` groups | FIXED |
| IR18-3 | BLOCKER | The drift gate bypasses the shipping wiki catalog collector | FIXED |

**Independent round 18: 3 BLOCKER, 0 ISSUE, 0 outstanding.** All three findings are fixed.

### Independent review, round 19 (2026-08-27)

An owner-authorized independent context reviewed the committed Round-18 fixes.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR19-1 | BLOCKER | Equivalent Markdown accepts unknown `Pipes, ...` groups | FIXED |
| IR19-2 | BLOCKER | Equivalent HTML ignores extra top-level Ze command articles | FIXED |
| IR19-3 | BLOCKER | Primary HTML and Markdown accept unknown pipe-group labels | FIXED |

**Independent round 19: 3 BLOCKER, 0 ISSUE, 0 outstanding.** All three findings are fixed.

### Independent review, round 20 (2026-08-27)

An owner-authorized independent context reviewed the committed Round-19 fixes.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR20-1 | BLOCKER | Wiki entries with empty support omit an explicit per-command pipe verdict | FIXED |
| IR20-2 | BLOCKER | Wiki validation accepts unknown pipe availability groups | FIXED |
| IR20-3 | BLOCKER | Primary label classification ignores unknown variants such as `Always, legacy` | FIXED |

**Independent round 20: 3 BLOCKER, 0 ISSUE, 0 outstanding.** All three findings are fixed.

### Independent review, round 21 (2026-08-27)

An owner-authorized independent context reviewed the committed Round-20 fixes.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR21-1 | BLOCKER | Wiki pipe label classification uses raw Markdown instead of visible text | FIXED |

**Independent round 21: 1 BLOCKER, 0 ISSUE, 0 outstanding.** The finding is fixed.

### Independent review, round 22 (2026-08-27)

An owner-authorized independent context reviewed the committed Round-21 fix.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR22-1 | BLOCKER | Wiki label splitting treats delimiters inside inline HTML attributes as structural | FIXED |

**Independent round 22: 1 BLOCKER, 0 ISSUE, 0 outstanding.** The finding is fixed.

### Independent review, round 23 (2026-08-27)

An owner-authorized independent context reviewed the committed Round-22 fix.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR23-1 | BLOCKER | Wiki label splitting does not skip every CommonMark inline HTML form | FIXED |

**Independent round 23: 1 BLOCKER, 0 ISSUE, 0 outstanding.** The finding is fixed.

### Independent review, round 24 (2026-08-27)

An owner-authorized independent context reviewed the committed Round-23 fix.

| # | Sev | Finding | Disposition |
|---|-----|---------|-------------|
| IR24-1 | BLOCKER | Inline HTML scanning gives backslashes escape semantics inside quoted attributes | FIXED |

**Independent round 24: 1 BLOCKER, 0 ISSUE, 0 outstanding.** The finding is fixed.

### Independent review, round 25 (2026-08-27)

The owner-authorized independent documentation context returned clean after
reviewing the committed Round-24 fix. The independent compiled-coverage context
from round 14 remains clean because that path did not change.

**Independent round 25: 0 BLOCKER, 0 ISSUE. CLEAN.**

### Independent review, round 26 (2026-08-27)

An owner-authorized independent context reviewed the post-clean lint
corrections. The compiled-coverage review remains clean.

**Independent round 26: 0 BLOCKER, 0 ISSUE. CLEAN.**

## Design Insights

The audit's own evidence made the design: the wiki page is ALREADY generated and
still wrong, because the generator holds its own copy of the answer. Every
proposal that adds a channel repeats that mistake. So the whole spec is one
move -- put the answer in one place and let every surface read it -- applied to
an operator set, a per-command shape, and a published page.

## Key Design Decisions

| Decision | Why |
|----------|-----|
| The command declares the shape; the head reports the answer's | The head alone flips with walk length (audit 2.1), so it cannot be a contract. Both, checked against each other, can |
| Undeclared defaults to `doc` | Conservative: the globals are owed, the row operators are refused. A wrong refusal is visible and reported; a wrong answer is not |
| Field types on the `tab` head, not a key-name convention | The head is written by the producer on every answer, so it cannot drift. `nexthop` versus `next-hop` across four families proves a convention would be wrong on arrival (audit 9.4) |
| Local handlers return a payload rather than 20 new daemon handlers | One mechanism covers all 46 local paths, closes both halves of the 38, and removes the dual-registration asymmetry as a side effect |
| Composition is the contract; meaningless repetition is refused | The record path already composes correctly and is the behaviour the owner asked for. Refusing `fill \| fill` is honest where picking a winner is not |
| `pipe` metadata becomes an ordered list | A map cannot record `match X \| match Y`. R-2 |

## Known Limitations

- A command whose answer genuinely changes shape with its arguments declares the
  widest shape it can answer, so the refusal is conservative for its narrow form.
- `| resolve` and `| origin` stay refused on `doc` and `map` until a command
  declares an address field. That is a deliberate narrowing from today's
  guess-by-parsing, and it is reversible per command by declaring.
- A refusal that arrives AFTER the command has run reaches the caller as the
  formatted answer string, because that is the only channel left at that point.
  `ze pipe` reads `command.IsPipeError` and exits non-zero; the `ze cli -c` and
  SSH exec surfaces still print it and exit 0, which phase 5 closes when it
  settles exit codes across surfaces. A tool author reading those two surfaces
  today must check the prefix.
- `| log` is a TUI concept; if phase 5 finds it cannot be honored on an exec
  channel, AC-8's second branch applies and it leaves the global class rather
  than being published as global and inert.

## RFC Documentation (Scope: protocol)

Not applicable.

## Checklist

### Goal Gates (MUST pass)

- [x] Every AC has a test that FAILS without the change
- [x] The wiring test passes with no core edit
- [x] Goal Validation table is full, with evidence per goal
- [x] No hand-copied operator list remains in the tree

### TDD

- [x] Tests written
- [x] Tests FAIL (red) before implementation
- [ ] Tests PASS (paste output)
- [x] Implementation passes
- [x] Discrimination proven: catalog, rendered-surface, refusal, and local-data coverage mutations redden their named tests

### Closure

- [x] `./le changed scope` was reviewed; the selected all-package result came from foreign migration status
- [x] `./le verify current mode full` evidence is recorded at committed tree `08cba7efd`, under the name that gate carried then
- [ ] `./le verify worktree` is re-run over the closing commit, because the gate now runs against a commit in a throwaway worktree rather than in place (`ai/rules/git-safety.md`)
- [x] Review Gate 0 BLOCKER / 0 ISSUE
- [x] `plan/deferrals/cli-pipe-operator-coverage.md` has no rows and is ready for removal
- [ ] Citation and foreign-owned documentation edits landed

## External Closure Blocker

The review gate and every product check are complete. The remaining blocker is
one ownership constraint: concurrent `le` migration changes already modify the
files that carry two citations and part of the documentation review. This
closure must not stage those foreign edits. Commit A and Commit B therefore
remain unprepared, and the spec stays at `verification`.

The migration owner must land the current changes first. The closure context
then has six exact edits: restate the historical citation in
`plan/handoff-cli-remaining.md` as `spec-cli-pipe-operator-coverage`; change the
`Depends` cell in `plan/spec-cli-show-bgp-answer-shapes.md` to the same bare
stem; correct the pipe availability and address-field claims in
`docs/features/formatting.md`, `docs/features.md`, and
`docs/architecture/api/commands.md`; and include the existing release note
`website/changes/discord/2026-08-17-weekly.md`. The final report carries the
native closure commands.

## Implementation Summary

Five surfaces held different operator lists, `ze help command --json` published
the `global-pipes` boolean, and local commands could bypass the pipe layer. The
implementation gives the operator language one catalog, derives a per-command
contract from answer declarations, refuses unsupported operators by name, and
renders the wiki and website from the same contract.

| Piece | Producing symbol |
|-------|------------------|
| Canonical operator contract | `internal/component/command/pipe_catalog.go` `PipeOperatorCatalog`, `knownPipeOps` |
| Shape and address declarations | `internal/component/command/answer_shape.go` `RegisterShape`, `RegisterAddressFields`, `ShapeForCommand` |
| Refusal and document transforms | `internal/component/command/pipe.go` `validateDeclaredShape`, `ApplyPipes` |
| Record transforms | `internal/component/command/render_records.go` `RenderRecords` |
| Ordered chain metadata | `internal/component/command/pipe.go` `collectPipeMeta`, `injectPipeMeta` |
| Local command path | `internal/component/command/local_data.go` `ServeLocal`, `WriteAnswer` |
| Save and stream lifecycle | `internal/component/command/pipe_save.go` `validateSaveOps`, `saveAnswer`, `StreamSaves` |
| Published command contract | `cmd/ze/help_command.go` `operatorsFor`, `collectCommands` |
| Wiki output | `internal/le/wikicatalog/catalog.go` `Collect`; `render.go` `Render` |
| Website and drift gate | `internal/le/docvalid/command_render.go` `RenderCommandSurfaces`; `command_surfaces.go` `validateGeneratedCommandSurfaces` |

The review fixed every blocker and issue it found. The durable detail is in the
26 Review Gate rounds above. The final focused correction commit is
`08cba7efd`, and `03be88802` records the clean review.

## Deviations

| # | Spec said | Built | Why |
|---|-----------|-------|-----|
| D-1 | An undeclared command defaults to `doc` | It publishes row operators as `with-rows` and lets the answer in hand decide | Most undeclared commands do carry rows. A false `doc` default would refuse useful operators |
| D-2 | `countItems` and `truncateItems` repair envelopes in place | `rowSet`, `rowsInKeyed`, and `selectRows` make rows the unit for every row operator | Composition needs one shared row model |
| D-3 | A `tab` answer head carries field types | The command declares `address-fields` beside its shape and columns; the answer head remains type, key, and columns | Admission must happen before dispatch, and the Stage 1 declaration already crosses the plugin boundary |
| D-4 | Local payload rendering was byte-identical from the start | `WriteAnswer` had to become the shared newline policy | Product execution found the two spellings differed by one newline |
| D-5 | Wiki and website generators consume only `ze help command --json` | The website consumes the JSON contract; native `wikicatalog.Collect` reads the same registries directly | The migration removed the Python hop while preserving one producer contract |
| D-6 | Seven separate `.ci` and `.et` files carry the functional plan | `pipe-local-command.ci`, `pipe-interactive-save.ci`, `pipe-review-entry-contracts.ci`, and `pipe-review-remote-contracts.ci` consolidate the entry paths | The consolidated fixtures exercise the real local, interactive, web, SSH exec, and SSH PTY boundaries |
| D-7 | Closure docs and citations land with Commit A | Six required paths already carry foreign migration work | The migration owner must land those edits first; this closure will then apply only its own lines |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | A substring assertion passed for `raw` because `ndjson` contained it | Operator lists require token equality | Mutation review | Replaced substring checks with token checks |
| approach | The RIB answer shape changed before its consumers were swept | Six consumers depended on the grouped shape | Functional gate | Migrated each producer-bound consumer |
| assumption | A nested config document looked like keyed rows | Payload shape cannot distinguish every document from rows | `show config dump \| first 1` | Declared shape wins before dispatch |
| approach | A streaming conversion nested a read lock behind a queued writer | Go `RWMutex` blocks that later reader | Lint and deadlock trace | Construct under the lock and drain after release |
| assumption | Two local spellings shared the renderer and therefore the bytes | Their newline writers differed | Byte comparison in AC-11 | Both call `WriteAnswer` |
| assumption | The wiki would remain a JSON consumer | Native migration reads the registries through `wikicatalog.Collect` | Producer audit at closure | Recorded D-5 and verified both consumers derive the same contract |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Global formatting and saving on every reachable command | Done | `pipe_catalog.go` `pipeCatalog`; `pipe_save.go` `validateSaveOps` | `save` is local-process-only; remote expansion refuses the filesystem effect |
| Row operations only where rows exist, with named refusal elsewhere | Done | `answer_shape.go` `rowsInKeyed`; `pipe.go` `validateDeclaredShape`, `ApplyPipes` | Declared shape resolves ambiguity; answer shape covers undeclared commands |
| Per-command website and wiki contract from one derivation | Done | `help_command.go` `operatorsFor`; `wikicatalog.Render`; `docvalid.RenderCommandSurfaces` | Structural drift tests reject missing, extra, reordered, or malformed groups |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Producing symbol |
|-------|--------|-----------------|------------------|
| AC-1 | Done | `TestCatalogIsTheParser`, tagged CLI catalog tests | `PipeOperatorCatalog`, `knownPipeOps` |
| AC-2 | Changed | D-1 and `TestDeclaredShapeResolvesByLongestPrefix` | `RegisterShape`, `ShapeForCommand` |
| AC-3 | Done | declared-doc refusal in `pipe-local-command.ci` | `validateDeclaredShape` |
| AC-4 | Changed | count cases in `pipe-local-command.ci` | `rowSet`, `selectRows`, `applyCount` |
| AC-5 | Done | nested match and repetition cases | `foldFilters`, `ApplyPipes`, `RenderRecords` |
| AC-6 | Done | ordered metadata assertion | `collectPipeMeta`, `injectPipeMeta` |
| AC-7 | Done | `pipe-interactive-save.ci`, entry-contract fixtures | `validateSaveOps`, `saveAnswer`, `StreamSaves` |
| AC-8 | Done | stream-class tests | `ProcessStreamPipes`, `validateStreamOps` |
| AC-9 | Done | no-format nested match case | `applyMatch`, `recordsMatching` |
| AC-10 | Done | 16/16 executable local-data cases | `RegisterLocalData`, `ServeLocal` |
| AC-11 | Done | byte parity case in `pipe-local-command.ci` | `WriteAnswer` |
| AC-12 | Changed | D-3 and declared-field address tests | `RegisterAddressFields`, `bindAddressFields` |
| AC-13 | Done | tagged CLI JSON contract tests | `operatorsFor`, `collectCommands` |
| AC-14 | Done | wikicatalog golden and current-catalog tests | `wikicatalog.Collect`, `Render` |
| AC-15 | Done | docvalid mutation tests | `validateGeneratedCommandSurfaces`, `validateWikiPipeSupport` |
| AC-16 | Done | rendered CLI rule and point | `ai/rules/cli.md`; catalog producer above |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Catalog, completion, and published contract | Done | `pipe_catalog_test.go`, tagged `cmd/ze` tests | One token population |
| Shape resolution and named refusal | Done | `answer_shape_test.go`, `pipe-local-command.ci` | Includes ambiguous document |
| Row extraction, composition, repetition, metadata | Done | `pipe_test.go`, `pipe_columns_test.go`, `render_records_test.go` | Document and record paths |
| Save and stream lifecycle | Changed | `pipe_save_test.go`, `pipe-interactive-save.ci`, entry fixtures | Consolidated local and remote boundaries |
| Local handler payload and parity | Changed | `pipe-local-command.ci`, `local_data_functional_coverage_test.go` | Executable population, no copied count |
| Published wiki and website | Changed | `wikicatalog/render_test.go`, `docvalid/command_surfaces_test.go` | Native renderers replaced planned Python fixtures |
| Numeric and empty boundaries | Done | `answer_shape_test.go`, `pipe_test.go`, `render_records_test.go` | Zero, exact, one over, and threshold |
| Security entry paths | Done | `pipe-review-entry-contracts.ci`, `pipe-review-remote-contracts.ci` | Interactive, web, SSH exec, SSH PTY |

### Files from Plan

| File group | Status | Notes |
|------------|--------|-------|
| `internal/component/command/pipe*.go` | Done | Catalog, row model, composition, save, stream, metadata |
| `answer_shape.go`, `pipe_catalog.go` | Done | Created as planned |
| `pkg/plugin/rpc/message.go` head field types | Changed | D-3 keeps the head unchanged and declares address fields in `CommandDecl` |
| CLI help, completion, and local dispatch | Done | All hand-copied lists removed; local payload path shared |
| Local-data command registrations | Done | Executable coverage derives all 16 production registrations |
| Wiki, website, and drift generators | Changed | Native `wikicatalog` and `docvalid` replace the planned Python-only route |
| Generated operator reference and rules | Done | Catalog-generated table and CLI rule point |
| Planned functional files | Changed | D-6 records consolidation into four fixtures |
| Documentation pages | Changed | D-7 and External Closure Blocker name the pending foreign-owned corrections |
| Release note | Changed | Existing content is correct but remains untracked under foreign ownership |

- **Total rows:** 37
- **Done:** 26
- **Changed:** 11, each recorded in Deviations or the External Closure Blocker
- **Partial:** 0
- **Skipped:** 0

## Goal Validation

| Goal from Task | Evidence Type | Concrete Evidence |
|----------------|---------------|-------------------|
| Every reachable command answers global formatting and local saving | Functional | `bin/ze-test ui 188` passed; the entry fixtures cover local save and remote refusal |
| Row operators act on rows and refuse documents | Functional and mutation | `pipe-local-command.ci` counts rows, composes two matches, and refuses a declared document; `answer_shape_test.go` covers ambiguous and empty answers |
| Website, wiki, and JSON publish one per-command contract | Structural and mutation | Tagged docvalid and wikicatalog packages passed; mutations of operator, qualifier, alias, identity, group order, and empty support redden named tests |
| Repeated operators compose on every chain path | Unit and functional | Document, metadata, folded-filter, and record tests pass; the UI fixture narrows twice |
| The feature is complete rather than a partial publication layer | Population gate | `TestEveryLocalDataRegistrationHasAFunctionalCase` and `TestLocalDataCoverageEvidenceIsNonVacuousAndComplete` cover all 16 production registrations |

## Security Review

| Concern | Result | Producer evidence |
|---------|--------|-------------------|
| Local file write | Clean | `saveAnswer` creates a same-directory temporary file, sets mode 0600, and renames only after a complete write |
| Remote filesystem authority | Clean | `validateSaveOps` refuses daemon-expanded save; web, SSH exec, and production SSH PTY entry fixtures prove the guard |
| Injection | Clean | Pipe operators execute no shell or SQL; catalog names and classes are compiled declarations |
| Untrusted input and bounds | Clean | `validatePipeArgument` rejects malformed tails; record `last` retention is bounded by `rpc.AnswerBufferThreshold` and `rpc.MaxMessageSize` |
| Path traversal and symlinks | Clean for the declared local effect | The local user chooses the path with that user's process authority; atomic replacement does not grant daemon authority |
| Races and partial files | Clean | `StreamSaves` stages every destination and commits after stream success, aborting all temporary files on failure |
| Resource exhaustion | Clean | Row walks stream where possible; retained tail and declaration sizes are bounded |
| Cryptography, secrets, privilege escalation | Not applicable | No cryptographic primitive, secret, privilege transition, or new listener was added |
| Information leakage | Clean | Refusals name operator and shape without payload or path internals beyond the path the local user supplied |
| Runtime dependencies and doctor checks | Not applicable | No new external binary, socket, port, certificate, kernel feature, or persistent runtime path |

No security blocker or issue remains.

## Deferrals Resolved

| Row from shard | Final Status | Destination or evidence |
|----------------|--------------|-------------------------|
| none | done | `plan/deferrals/cli-pipe-operator-coverage.md` contains no data row and is removed with the spec in Commit B |

The unknown-argument finding belongs to command grammar and is recorded in
`plan/audit-command-pipe-vs-subcommand.md`; it was never a deferral from this
spec.

## Documentation Updates

| Category or page | Yes/No | Source-aware result |
|------------------|--------|---------------------|
| Feature list | Yes | `docs/features.md` must replace its universal address and row-operator claims after the foreign migration lands |
| User guide | Yes | `docs/guide/cli.md` now says `match <text>` and points readers to the per-command contract |
| Config syntax | No | No YANG leaf or config parser changed |
| CLI reference | Yes | `docs/features/formatting.md` must state per-command availability; `docs/features/pipe-operators.generated.md` is catalog-generated |
| API/RPC docs | Yes | `docs/architecture/api/commands.md` and `process-protocol.md` describe declarations and refusal; address fields now admit and limit transforms |
| Plugin SDK | Yes | `pkg/plugin/rpc/types.go` `CommandDecl` documents shape, columns, and address fields |
| Wire format | No | `AppendAnswerHead` and `parseAnswerHead` still carry item type, key, and columns; D-3 moved address declarations to Stage 1 |
| RFC compliance | No | No RFC or routing wire behavior changed |
| Comparison table | No | Support level did not change |
| Test infrastructure | Yes | Native docvalid, wikicatalog, local-data population, and functional entry fixtures are documented in this audit |
| Architecture design | Yes | `docs/architecture/api/commands.md` is the durable design destination |
| Config editor | Yes | `docs/guide/config-editor.md` distinguishes editor filters from operational pipe operators |
| Release note | Yes | `website/changes/discord/2026-08-17-weekly.md` names `save`, per-command publication, and RIB shape changes |

`./le docvalid doc-drift` and the tagged docvalid package passed before closure.
The aggregate `./le doc check verify` remains red only through foreign migration
state already recorded in the Gates table below. No RFC status or doctor update
applies.

## Pre-Commit Verification

### Files Exist

| File | Exists | Fresh evidence |
|------|--------|----------------|
| `internal/component/command/pipe_catalog.go` | yes | `stat`: 14565 bytes |
| `internal/component/command/answer_shape.go` | yes | `stat`: 15792 bytes |
| `docs/features/pipe-operators.generated.md` | yes | `stat`: 2146 bytes |
| `test/ui/pipe-local-command.ci` | yes | `stat`: 1626 bytes |
| `test/ui/pipe-interactive-save.ci` | yes | `stat`: 1415 bytes |
| `test/ui/pipe-review-entry-contracts.ci` | yes | `stat`: 762 bytes |
| `test/plugin/pipe-review-remote-contracts.ci` | yes | `stat`: 748 bytes |

### AC Verified

| AC ID | Producer | Recorded focused evidence |
|-------|----------|---------------------------|
| AC-1 | `PipeOperatorCatalog`, `knownPipeOps` | Tagged CLI catalog tests passed |
| AC-2 | `ShapeForCommand`, `shapeOfAnswer` | Shape tests and declared/undeclared UI cases passed; D-1 |
| AC-3 | `validateDeclaredShape` | Declared document refusal passed in UI |
| AC-4 | `rowSet`, `selectRows`, `applyCount` | Local row count and empty/keyed boundary tests passed |
| AC-5 | `foldFilters`, `ApplyPipes`, `RenderRecords` | Two-match and repetition tests passed |
| AC-6 | `collectPipeMeta`, `injectPipeMeta` | Ordered chain assertion passed |
| AC-7 | `validateSaveOps`, `saveAnswer`, `StreamSaves` | Interactive bytes/mode and remote-refusal fixtures passed |
| AC-8 | `ProcessStreamPipes`, `validateStreamOps` | One-shot refusal and stream acceptance tests passed |
| AC-9 | `applyMatch`, `recordsMatching` | Match then count without a format passed |
| AC-10 | `RegisterLocalData`, `ServeLocal` | Local-data coverage package passed for 16/16 registrations |
| AC-11 | `WriteAnswer` | Two spellings compared byte for byte in UI |
| AC-12 | `RegisterAddressFields`, `bindAddressFields` | Undeclared refusal and declared-field-only transforms passed; D-3 |
| AC-13 | `operatorsFor`, `collectCommands` | Tagged CLI JSON tests passed; no `global-pipes` field |
| AC-14 | `wikicatalog.Collect`, `Render` | Tagged wikicatalog golden/current tests passed |
| AC-15 | `validateGeneratedCommandSurfaces`, `validateWikiPipeSupport` | Tagged docvalid mutation and structural tests passed |
| AC-16 | `PipeOperatorCatalog` rendered into `ai/rules/cli.md` | Rule point and rendered aggregate checked |

### Wiring Verified

| Entry point | Producer path | End-to-end evidence |
|-------------|---------------|---------------------|
| `ze help command --json` | `operatorsFor` to `collectCommands` | Tagged `cmd/ze` contract tests |
| `ze cli -c` local command | `ServeLocal` to `ApplyPipes` | `pipe-local-command.ci`, 16/16 population check |
| Interactive save | model command to `StreamSaves` | `pipe-interactive-save.ci` |
| Web and SSH remote save | entry handler to `validateSaveOps` | entry and remote contract fixtures |
| Wiki and website | registry/JSON contract to native renderers | tagged wikicatalog and docvalid packages |

`bin/ze-test ui 188` passed. The local-data coverage test reads executable
fixture registrations rather than comments or copied command names.

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `AppendAnswerHead` and `parseAnswerHead` carry the item type on every answer; `TestShapeSpellingMatchesTheWire` binds declaration vocabulary to it |
| A-2 | confirmed | A declaration stays constant across the 256-record buffering threshold; answer representation may change without changing `ShapeForCommand` |
| A-3 | broken and corrected | The shared renderer had two newline writers; `WriteAnswer` now owns both spellings and the UI parity case passes. See D-4 and the Mistake Log |
| A-4 | broken by migration and corrected | Website rendering consumes the JSON contract; native wiki rendering reads the same registries through `wikicatalog.Collect`. See D-5 and the Mistake Log |

### Documentation Verified

| Claim or category | Source evidence | Result |
|-------------------|-----------------|--------|
| Per-command availability | `help_command.go` `operatorsFor` | Checked against CLI guide and pending formatting/features edits |
| Match semantics | `pipe.go` `applyMatch`, `applyMatchLines` | CLI guide corrected |
| Address-field scope | `pipe.go` `bindAddressFields`; `pipe_resolve.go` `resolveJSON` | RPC comment and process protocol corrected; commands doc edit pending foreign migration |
| Editor pipe language | `model_load.go` `dispatchWithPipe`, `ClassifyShowPipes` | Config editor guide corrected |
| Generated operator reference | `pipe_catalog.go` `RenderOperatorReference` | Tagged docvalid tests passed |
| Runtime inventory | `operatorsFor`, `wikicatalog.Collect` | Derived from registries, never from memory |

### Gates

| Gate | Result |
|------|--------|
| Review artifact hash check | `OK (14 code files, clean, hashes match)` after closure edits |
| Focused tagged packages | PASS: docvalid, wikicatalog, localdatacoverage, CLI, registry |
| Functional UI | `bin/ze-test ui 188`: PASS |
| Tracked build | PASS at `08cba7efd` |
| `./le docvalid doc-drift` | PASS before foreign migration widened the tree |
| `./le changed scope` | Selected all packages because isolated verification had no status |
| Lint/aggregate docs | Remaining red findings are foreign migration changes, including the already deleted obsolete runtime-evidence test |

## Journal Decision

No journal row is added at closure. The implementation already recorded the
test-vacuity, consumer-sweep, lock-order, and self-review lessons when they were
found. The closure followed the existing rule that a clean review artifact does
not replace the documentation checklist, so it taught no new mechanism or
constraint.
