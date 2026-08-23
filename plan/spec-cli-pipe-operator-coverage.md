# Spec: pipe operators derived from answer shape, published per command

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | `plan/audit-pipe-operator-coverage.md`, `plan/audit-presentation-pipes.md`, `plan/audit-command-pipe-vs-subcommand.md` |
| Phase | 4 of 6 (phases 1, 2, 3, 5, 6 done; 4 mechanism done, four families outstanding) |
| Deferral shard | `plan/deferrals/cli-pipe-operator-coverage.md` |
| Handoff | - |
| Updated | 2026-08-23 |

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
| Operators the parser knows | 16 | `knownPipeOps`, `internal/component/command/pipe.go:64` |
| Hand-copied operator lists elsewhere | 5, no two agreeing | audit 1.1 |
| Operators absent from EVERY user-reachable list | 2 (`display`, `fill`) | audit 1.1 |
| Commands reaching no pipe layer on any surface | 38 | audit 3.1 |
| Of those, published as `"global-pipes": true` while the daemon answers `unknown command` | 20 | audit 3.1 |
| Commands losing pipes only on the `ze <verb>` form | 6 measured, 2 unmeasured | audit 3.2 |
| Wiki entries carrying a hand-typed ten-operator line | 381 | audit 8.2 |
| Behaviors for a repeated operator | 4 | audit 5 |
| A `save` / `write` / `tee` operator | none exists | audit 1 |

Source files read for the table above:

- [ ] `internal/component/command/pipe.go` -- `knownPipeOps:64`, `ValidatePipes:488`, `countItems:586`, `truncateItems:674`, `collectPipeMeta:276`, `foldFilters:197`, `ApplyPipes:382`
- [ ] `internal/component/command/pipe_columns.go:119` -- `columnsInChain`, which assigns rather than intersects
- [ ] `internal/component/command/pipe_records.go:46` -- `ApplyPipesRecords`, the one path that composes correctly
- [ ] `internal/component/command/column_order.go:64` -- `ColumnsForCommand`, the registry pattern the shape declaration reuses
- [ ] `internal/component/command/pipe_filter.go:66` -- `PipeFiltersForCommand`, and `unknownFilterError`, the model for a refusal that enumerates
- [ ] `internal/component/command/alias.go:545` -- `AliasesForCommand`, absent from `ze help command --json`
- [ ] `pkg/plugin/rpc/message.go` -- `AppendAnswerHead`, `checkAnswerType`
- [ ] `cmd/ze/help_command.go` -- `printCommandVerbose`, hand-copied list of 10
- [ ] `cmd/ze/ze_core_pipe.go` -- `pipeUsage`, a different hand-copied list of 10
- [ ] `internal/component/ssh/ssh.go` -- `ProcessPipesDefaultFormatChecked` on the exec channel
- [ ] `scripts/dev/gen_wiki_commands.py` -- the generator holding its own copy
- [ ] `website/tools/page_registry.py` -- `DOCS_MANIFEST`, which omits the accurate page

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
- `scripts/dev/gen_wiki_commands.py`, `website/tools/render-cli-catalog.py`,
  `render-command-equivalents.py`, `render-llms-txt.py`: the generators.
- `scripts/docvalid/doc_drift.go` under `ze-doc-verify`: the gate.

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
| `make ze-wiki-commands-update` | JSON -> `gen_wiki_commands.py` | `test/plugin/pipe-catalog-published.ci` <!-- doc-links: ignore (this spec creates this file; the phase that does is named in the same row) --> |

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
| **AC-14** | `make ze-wiki-commands-update`; the website build | The wiki catalog and the website CLI catalog are generated from that JSON and hold no operator literal. `docs/architecture/api/commands.md` is in the website manifest. |
| **AC-15** | an operator name in docs absent from the catalog | `ze-doc-verify` fails when any operator name in `docs/` or the wiki is absent from the catalog, or when a command's published list disagrees with what it declares. |
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
| `scripts/dev/gen_wiki_commands.py` | delete the operator literal |
| `website/tools/render-cli-catalog.py`, `render-command-equivalents.py`, `render-llms-txt.py` | render the real list |
| `website/tools/page_registry.py` | add `docs/architecture/api/commands.md` to `DOCS_MANIFEST` |
| `scripts/docvalid/doc_drift.go` | the new gate check |
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

- [ ] Registered, not hardcoded: shape and field types declared by the owning command
- [ ] The catalog has exactly one copy and every surface derives from it
- [ ] A plugin can declare a shape without a core edit
- [ ] The generated page and the product cannot disagree without a red gate

### Documentation Update Checklist (BLOCKING)

- [ ] `docs/features/formatting.md` prose corrected: the universal claim, and `match` as line grep
- [ ] `docs/guide/cli.md` stops presenting 5 operators as the set, and stops spelling `match <regex>` for a `strings.Contains` match
- [ ] `docs/architecture/api/commands.md` published to the website
- [ ] `docs/guide/config-editor.md` says its `|` vocabulary is a different language from the operational one
- [ ] `docs/architecture/api/wire-format.md` documents the field types
- [ ] Release note for R-1 and R-2, the two user-visible breaks

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

- [ ] Registration over hardcoding: no core package enumerates commands or shapes
- [ ] No sixth copy of the operator list introduced anywhere
- [ ] Every refusal names the operator, the shape, and what to type instead
- [ ] The derivation is read from the declaration, never from the head alone (A-2)
- [ ] `ai/rules/simplicity.md`: the local-handler conversion is ONE mechanism, not 20 new daemon handlers

### Deliverables Checklist

- [ ] Every AC has a named test
- [ ] Each user-visible break has a release note
- [ ] The generated page exists and the gate fails when it drifts

### Security Review Checklist

- [ ] `| save` writes only where the invoking user may write, and refuses a path
      it cannot write with a clear message rather than a partial file
- [ ] `| save` inside an SSH session does not let a remote caller write outside
      the paths that session is already entitled to
- [ ] No operator list is built from user input

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
`ze-doc-drift-check` by name.

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

**Still owed for AC-10:** the yang and config CLI families (11 of the 20
commands that declare a wire method no daemon handler implements). Each is the
same conversion: a data handler beside the existing printer, a
`RegisterLocalData` call, and a declared shape.

The env family (3), the schema family (5) and two of the three storage commands
are converted.

**One command is deliberately NOT converted, and that is an answer rather than
a gap.** `show data cat` returns the BYTES of one stored file, which may be
YAML, JSON, a certificate or a binary blob. Those bytes are the answer; wrapping
them in a record would corrupt the one use the command has, and no pipe operator
has anything to do with them. It keeps its plain handler and the published page
says it reaches no pipe layer, which is true rather than an omission. A command
whose answer is a byte stream is the boundary of this spec, not a case it
missed. Schema was cheap
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

- [ ] Every AC has a test that FAILS without the change
- [ ] The wiring test passes with no core edit
- [ ] `ze help command --json` and the website agree with the product, enforced
- [ ] No hand-copied operator list remains in the tree

### TDD

- [ ] Tests written
- [ ] Tests FAIL (red) before implementation
- [ ] Tests PASS (green) after implementation
- [ ] Discrimination proven: each new gate reddens when its subject is reverted

### Closure

- [ ] `make ze-lint-changed` clean
- [ ] `make ze-precommit-verify` green over the commits (worktree, on cadence)
- [ ] Review Gate 0 BLOCKER / 0 ISSUE
- [ ] `plan/deferrals/cli-pipe-operator-coverage.md` rows resolved
