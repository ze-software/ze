# Spec: cli-order-pipe

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | spec-cli-column-order |
| Phase | 1/5 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`spec-cli-column-order` gave each command a built-in column order that an
operator cannot change. The operator needs to change it: to put the field they
are working on first, and to cut a 19-column table down to the four columns the
question is about.

Two operators are that answer. Owner-specified syntax, 2026-08-19. This
REPLACES the earlier single `| order` with a trailing `*`; Key Design Decisions
records why both `*` and a positional mode word were dropped.

```
| display <field> [<field> ...]    field names only
| fill [<way>] [reverse]           keywords only: alpha or overall
```

Each operator takes ONE homogeneous kind of argument, so no token is a field
name in one position and a keyword in another.

```
show bgp peer list | display state name           -> ONLY those two columns, in that order
show bgp peer list | display state | fill         -> state, then the rest in the declared order
show bgp peer list | display state | fill alpha   -> state, then the rest by field name
show bgp peer list | fill overall                 -> every column, narrowest rendered column first
show bgp peer list | fill alpha reverse           -> every column, reverse by field name
```

| Written | The remaining fields are ordered by |
|---------|------------------------------------|
| no `\| fill` | nothing: they do not appear |
| `\| fill` | the command's built-in declared order, and by name when it declared none |
| `\| fill alpha` | field name, FORCED, even where a built-in order exists |
| `\| fill overall` | the rendered column width, narrowest first |
| `\| fill ... reverse` | the same way, flipped |

`| display` selects and sequences. `| fill` decides whether the remainder
appears at all and in what order. They are additive and neither changes what the
other means. With no `| display`, every field is a remaining field.

Selection is the half that does not exist anywhere today. Ze can render every
column or the built-in order; it cannot render four of nineteen.

**Field names after `| display` MUST autocomplete in the SSH CLI** (owner
requirement, 2026-08-19). This makes the built-in declarations from
`spec-cli-column-order` load-bearing: the registry is the only in-process list
of a command's field names, so a command that declared no order can offer no
completions. `| fill` completes its own keywords and never a field name.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - CLI pipe operators, command-owned pipe filters
  → Decision: generic operators are client-side over the dispatcher's JSON; command-owned filters fold into server-side arguments
  → Constraint: an operator that is neither classified in `foldFilters` nor generic is dropped without an error

### Rules
- [ ] `ai/rules/cli.md` - pipe completeness, JSON format contract, the `*` selector wildcard
  → Constraint: every command that produces output supports all pipe operators
  → Decision: `*` stays the peer-selector wildcard and enters no pipe grammar. A shell expands an unquoted one before ze sees it, which is why `| fill` says the same thing in a word
  → Decision: keyword before value; an operator's argument is positional after the operator name, and every argument of one operator is the same kind
- [ ] `ai/rules/stale-comments.md` - changing behavior changes the comments that described it
  → Constraint: `orderKeys`' doc comment states that a key is never dropped. `| display` makes that true only of the BUILT-IN path, so the comment changes in the same edit

**Key insights:**
- `completePipeForCommand(command, partial)` (`internal/component/command/completer.go`) already carries the command into completion, so field completion needs no new plumbing to reach the registry. What is missing is that `pipeSubArgs` is a static map keyed by operator name only.
- `orderKeys` (`internal/component/command/pipe_table.go`) already separates declared keys from the remainder and returns both concatenated. Selection is returning the first slice alone.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/command/pipe.go` - `knownPipeOps` holds 14 operators and none of `display`, `fill` or `sort` is among them. `ParsePipe` splits on `|` then applies `strings.Fields` per segment; only `pipeMatch` joins the tail into `op.arg`, which is the precedent a multi-token argument follows. `foldFilters` classifies ops into client-side and server-side and **has no `default:` arm**, so an unlisted kind reaches neither. `ApplyPipes` builds `tableStyle` and takes the registered orders as its fourth parameter.
- [ ] `internal/component/command/pipe_table.go` - `orderKeys` splits keys into `declared` (stable-sorted by rank) and `rest`, returning `append(declared, rest...)`; its doc comment states a key is never dropped. `bestColumnOrder` picks the registered order matching the most keys and ties to the first declared, so an operator order appended to that slice would lose to a longer registered one. `renderRecord` renders a key/value table where dropping a key drops a ROW; `renderMapOfMaps` applies `orderKeys` to child keys only and prepends the parent-key column afterwards.
- [ ] `internal/component/command/column_order.go` - `ColumnOrder` is a bare `[]string`; `RegisterColumns` lowercases names and drops empty ones, so a `"*"` sentinel would survive normalization and must not be used to carry the flag; `ColumnsForCommand` resolves by longest command prefix.
- [ ] `internal/component/command/completer.go` - `PipeOperators` is a hand-maintained list of 14 suggestions; `pipeSubArgs` maps only `json`; `completePipe(partial, commandFilters)` takes the whole tail after the operator name as one sub-partial, which prefix-matching cannot use once more than one field is typed; `completePipeForCommand` supplies the command.
- [ ] `internal/core/selector/selector.go` - `*` means "all" and per-octet globs, reached only from command text, never from a pipe segment.
- [ ] `cmd/ze/ze_core_pipe.go` - `runPipe` joins `os.Args` with a space, so an unquoted `*` is glob-expanded by the user's shell before ze sees it. `peer *` already carries the identical exposure.

**Behavior to preserve:**
- A command with neither operator renders exactly as `spec-cli-column-order` left it: built-in order first, alphabetical remainder, nothing hidden.
- The BUILT-IN order never selects. A command's own declaration must never hide a field, because a reader who did not ask for it reads an absent column as an absent value.
- `| json`, `| ndjson` and `| yaml` keep alphabetical key order.
- Every existing pipe operator keeps working, including on commands with registered pipe filters.

**Behavior to change:**
- Two new operators exist. `| display` takes one or more field names and overrides the built-in order for the fields it names. `| fill` takes a way word and an optional `reverse`, and orders the fields `| display` did not name.
- Without a `| fill`, the fields `| display` did not name are dropped from every format.
- `orderKeys`' doc comment stops claiming that a key is never dropped, and states which path that now holds for.
- Field names complete in the SSH CLI after `| display`, and way words after `| fill`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator types `show bgp peer list | display state name | fill alpha` into the SSH CLI, the web CLI, or `ze cli -c`. The text reaches `ParsePipe` (`internal/component/command/pipe.go`) as one string.

### Transformation Path
1. `ParsePipe` splits the command from the operator chain and parses each operator into a `pipeDisplay` or `pipeFill` op whose `arg` is the joined tail, following the `pipeMatch` precedent.
2. `foldFilters` classifies BOTH kinds as CLIENT-side ops. Without this, each is silently dropped for every command that registers pipe filters.
3. `ValidatePipes` rejects a `display` that names no field, and a `fill` whose word is neither `alpha` nor `overall` nor `reverse`. A bare `fill` is valid.
4. The dispatcher runs the command; `ResponseJSON` marshals the payload.
5. `columnsInChain` reads both ops into one `columnRequest` before the apply loop runs, so the answer does not depend on which side of the format operator either one was written.
6. `applyDisplaySelect` cuts the payload when the request selects, which is `display` present and `fill` absent. That reaches every format.
7. `ApplyPipes` builds `tableStyle` with that request, which the renderer consults INSTEAD of `bestColumnOrder` for the fields `display` named, never appended beside it.
8. `orderKeys` sequences the displayed fields by the request, and `fillKeys` sequences the rest by the way `fill` named, which for a bare `fill` is the command's own declaration.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator input ↔ renderer | A parsed `columnRequest` carried on `tableStyle`, never into the payload | Yes: `TestColumnOpsAbsentLeavesOutputUnchanged` |
| Operator input ↔ payload | `applyDisplaySelect` rewrites the JSON in the apply loop, at the operator's own position | Yes: `TestDisplaySelectionReachesJSON` |
| Completion ↔ column registry | `completePipeForCommand` reads `ColumnsForCommand` for the field list | Yes: `TestCompleteDisplayFieldsFromRegistry` and `test/ui/display-fill-completion.ci` |

### Integration Points
- `internal/component/command/column_order.go` - `ColumnsForCommand` is the default fill order and the completion source
- `internal/component/command/pipe_table.go` - `orderKeys` gains the request, and `fillKeys` the remainder decision
- `internal/component/command/completer.go` - `pipeSubArgs` gains fill's keywords, and a dynamic per-command case answers `display`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Selection applies everywhere; order applies to the human formats

This is the one subtle rule, and it follows from what each half MEANS.

| Half | Question it answers | Applies to |
|------|--------------------|-----------|
| Order | in what sequence | `\| table`, `\| text` only. Sequence carries no meaning for a program, per the owner directive of 2026-08-19 |
| Selection | which fields | every format, including `\| json`, `\| ndjson`, `\| yaml` |

Selection is a DATA question the operator asked explicitly, so honoring it in
JSON is what they asked for. The alternative readings were both worse: ignoring
`| display` under a programmatic format drops a request silently, and refusing
the combination would error on `| display a b` whenever `environment cli format
default json` is committed, which the operator never typed.

The built-in order still never reaches JSON, because the built-in never selects.
That is what keeps `spec-cli-column-order`'s AC-5 true.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No argument of either operator can be confused with the selector wildcard | The `*` token is gone from the grammar, and `ParsePipe` cuts the command at the first `\|` before any operator is parsed; `selector.go` is reached only from command text | An argument misroutes to selector code | `TestParsePipeDisplayJoinsFields` and `TestParsePipeFillWayAndReverse`, plus four `.ci` driving the real CLI | confirmed |
| A-2 | Multi-token completion needs only last-token matching, not a new completer | `completePipe` took the whole tail as one sub-partial; `strings.Fields` already collapsed whitespace | Field completion works for the first field only | `TestCompleteDisplayFieldsAfterFirst` | confirmed |
| A-3 | Classifying both kinds in `foldFilters` is sufficient to make them survive on filtered commands | The switch has no `default:`, so an unlisted kind is dropped | `show bgp rib \| display ...` silently does nothing | `TestColumnOpsSurviveFoldFiltersOnFilteredCommand` (AC-9) and `test/ui/display-fill-filtered-command.ci` | confirmed |
| A-4 | Selection in JSON does not break an in-tree consumer | The six structured-data callers reclassified in `spec-fixit-cli-format-default-everywhere` all use `\| raw`, which is an identity arm | A dashboard or completion helper loses fields | Grep the six callers; none passes `\| display` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An operator selects away the column that identifies the row, leaving an unreadable table | `\| display state` on a peer table renders a column of states with no addresses | `renderMapOfMaps` prepends the parent key outside `orderKeys`, so keyed shapes keep identity; for `renderList` this is the operator's explicit request and is left as typed |
| R-2 | A post-format `\| match` stops matching because its column was selected away | `\| display a b \| match <value-in-dropped-column>` returns nothing | Correct behavior, since the text genuinely no longer contains it; pinned by a test so it is a decision rather than a surprise |
| R-3 | An unquoted `*` is glob-expanded by the user's shell before ze sees it | RETIRED. The `*` token was dropped from the grammar for exactly this reason, and no argument of either operator carries a shell metacharacter | Closed by the design change of 2026-08-19; `peer *` keeps its own pre-existing exposure |
| R-4 | Field completion offers nothing for the many commands that declared no built-in order | Tab after `\| display` on most commands is silent | Known and stated: completion quality tracks the column-order rollout. The operator can still type names by hand |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Rendered CLI output and, for an explicit `\| display` only, the field set of a JSON answer. No protocol behavior, no config, no wire format |
| How is it reverted? | Single commit revert. The operator must type `\| display` or `\| fill` for any of it to apply |
| Who else touches this path? | `spec-cli-column-order` (implemented, unclosed) owns the registry and `orderKeys`; `spec-fixit-cli-format-default-everywhere` owns the format-default resolution on the same wrappers |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Operator types `show bgp peer list \| display state \| fill alpha` in the SSH CLI | → | both kinds parsed, classified client-side, applied in `orderKeys` | `test/ui/display-fill-remainder.ci` |
| Operator types `show bgp peer list \| display state name` | → | `applyDisplaySelect` cuts the payload and `orderKeys` sequences what is left | `test/ui/display-fill-select.ci` |
| Operator presses Tab after `show bgp peer list \| display ` | → | `completePipeForCommand` reads `ColumnsForCommand` | `test/ui/display-fill-completion.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show bgp peer list \| display state address \| fill alpha` | Those columns first in that order, then every remaining column by field name |
| AC-2 | `show bgp peer list \| display state address` | Only those two columns, in that order. Every other column is absent |
| AC-3 | `\| display` names a field the payload does not carry | The name is inert; no empty column. With a `\| fill` the remaining fields still render; without one only the fields that matched render |
| AC-4 | `\| display` on a command that declared a built-in order | The operator's order wins completely for the fields it names. The built-in does not merge into it |
| AC-5 | `\| display a b \| json` | The JSON carries only fields `a` and `b`. Key order is unchanged (alphabetical). A `\| fill` in the chain brings every field back to the JSON |
| AC-6 | Neither operator typed | Output is byte-identical to what `spec-cli-column-order` produces, in every format |
| AC-7 | `\| display` with no field, or `\| fill` with a word it does not know | A clear pipe error naming the operator and the valid words. Not silence, not a crash. A bare `\| fill` is VALID |
| AC-8 | Tab after `\| display `, again after `\| display address `, and after `\| fill ` | The command's field names are offered for `display` both times and a name already typed is not offered again; `fill` offers `alpha`, `overall` and `reverse`, and never a field name |
| AC-9 | `show bgp rib \| display prefix \| fill alpha` (a command WITH registered pipe filters) | Both operators apply. Neither is silently dropped by `foldFilters` |
| AC-10 | `\| display state` on a map-of-maps payload | The parent key column still renders, so rows remain identifiable |
| AC-11 | `\| display a b \| fill` on a command that declared a built-in order | The named fields lead in the order given, and the REMAINDER follows the command's declared order rather than the alphabet. Two sequences over two disjoint key sets |
| AC-12 | `\| fill overall`, and `\| fill overall reverse` | The remaining columns are ordered by the width they RENDER at, narrowest first, and reverse flips it. A tie goes to the field name |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Cuts a wide peer table to two columns to answer one question | SSH input → `ParsePipe` → `foldFilters` → `applyDisplaySelect` → `orderKeys` | `test/ui/display-fill-select.ci` |
| 2 | Puts the field being investigated first and keeps the rest | same path, plus `fillKeys` | `test/ui/display-fill-remainder.ci` |
| 3 | Discovers the field names by pressing Tab rather than reading the docs | completer → `completePipeForCommand` → `ColumnsForCommand` | `test/ui/display-fill-completion.ci` |

## 🧪 TDD Test Plan

### Unit Tests
Every test below is in `internal/component/command/pipe_columns_test.go`, beside
the operators' own file, unless the File column says otherwise.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParsePipeDisplayJoinsFields` | `pipe_columns_test.go` | Multi-token argument follows the `match` precedent | |
| `TestParsePipeFillWayAndReverse` | `pipe_columns_test.go` | Every row of the `fill` table, and `reverse` as a flag rather than a way | |
| `TestValidatePipesRejectsBadColumnArguments` | `pipe_columns_test.go` | AC-7, both directions: what is refused and what stays valid | |
| `TestColumnOpsSurviveFoldFiltersOnFilteredCommand` | `pipe_columns_test.go` | AC-9, the silent-drop trap (A-3), for BOTH kinds | |
| `TestDisplayAloneSelectsAndSequences` | `pipe_columns_test.go` | AC-2 | |
| `TestDisplayThenFillAlpha` | `pipe_columns_test.go` | AC-1, and `reverse` over it | |
| `TestFillOverallOrdersByRenderedWidth` | `pipe_columns_test.go` | AC-12 | |
| `TestFillAloneOrdersEveryField` | `pipe_columns_test.go` | `fill` with no `display` orders the whole table | |
| `TestFillDefaultUsesTheDeclaredOrderForTheRemainder` | `pipe_columns_test.go` | AC-11, the two-sequences bug | |
| `TestFillDefaultAloneMatchesTheBuiltInOrder` | `pipe_columns_test.go` | A bare `fill` changes nothing it was not asked to change | |
| `TestDisplayOverridesRegisteredOrder` | `pipe_columns_test.go` | AC-4; the request is not appended to `tableStyle.orders` | |
| `TestDisplayUnknownFieldIsInert` | `pipe_columns_test.go` | AC-3 | |
| `TestDisplayKeepsParentKeyColumn` | `pipe_columns_test.go` | AC-10, R-1 | |
| `TestDisplaySelectionReachesJSON` | `pipe_columns_test.go` | AC-5: selection applies, sequence does not | |
| `TestDisplaySelectsANestedRecordTheSameWay` | `pipe_columns_test.go` | `\| text` and `\| json` agree at every depth | |
| `TestColumnOpsAbsentLeavesOutputUnchanged` | `pipe_columns_test.go` | AC-6 | |
| `TestColumnOpsAfterTheFormatOperator` | `pipe_columns_test.go` | The answer does not depend on which side of the format operator the request is written | |
| `TestDisplayThenMatchOnDroppedColumn` | `pipe_columns_test.go` | R-2 pinned as a decision | |
| `TestCompleteDisplayFieldsFromRegistry` | `internal/component/command/completer_test.go` | AC-8 first field | |
| `TestCompleteDisplayFieldsAfterFirst` | `internal/component/command/completer_test.go` | AC-8 later fields, last-token matching (A-2) | |
| `TestCompleteDisplaySkipsTypedFields` | `internal/component/command/completer_test.go` | AC-8 no repeats | |
| `TestCompleteFillOffersKeywordsOnly` | `internal/component/command/completer_test.go` | AC-8 keywords, and never a field name | |
| `TestCommandModelTakesACompleterWithNoEditor` | `internal/component/cli/model_test.go` | The interactive CLI starts at all; row in `plan/journal/guard-added-to-one-half-of-a-pair.md` | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Field-name count after `\| display` | 1 to the payload key count | equal to the key count | 0 names, rejected by AC-7 | more names than keys, inert per AC-3 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `display-fill-select` | `test/ui/display-fill-select.ci` | Operator cuts the peer table to two columns, and the row identity survives | |
| `display-fill-remainder` | `test/ui/display-fill-remainder.ci` | Operator brings the rest back, in the declared order, by name, and by width | |
| `display-fill-completion` | `test/ui/display-fill-completion.ci` | Operator tabs for field names and for way words in a real interactive SSH CLI, driven over a pseudo-terminal | |
| `display-fill-filtered-command` | `test/ui/display-fill-filtered-command.ci` | `show bgp rib \| display ... \| fill ...` on injected routes is not silently dropped (AC-9) | |

### Interop Tests (Scope: protocol)
Not applicable. Scope is `cli`; no wire-visible behavior changes.

## Files to Modify
- `internal/component/command/pipe.go` - `pipeDisplay` and `pipeFill` kinds, `knownPipeOps` entries, `ParsePipe` multi-token argument, `ValidatePipes` rejections, `foldFilters` client-side classification of BOTH, `ApplyPipes` request plumbing
- `internal/component/command/pipe_table.go` - `tableStyle` gains the operator's `columnRequest`; `orderKeys` honors it, splits into `declaredKeys`, `splitByOrder` and `fillKeys`, and its doc comment stops claiming a key is never dropped (`ai/rules/stale-comments.md`). Three width helpers measure a column only when `fill overall` asks
- `internal/component/command/completer.go` - `PipeOperators` gains both; sub-argument completion gains a dynamic per-command case reading `ColumnsForCommand`, matching on the last token, and `pipeSubArgs` gains fill's keywords
- `internal/component/cli/model.go` - `SetCommandCompleter` and `refreshCompleter` stop dereferencing a config completer a command-only model does not have. Row in `plan/journal/guard-added-to-one-half-of-a-pair.md`, 2026-08-19
- `docs/features/formatting.md` - documents both operators, the fill table, and that selection reaches JSON while sequence does not
- `docs/guide/command-reference.md` - the operator list and the `ze pipe` list
- `docs/architecture/api/commands.md` - the operator table and where each half of the request travels

## Files to Create
- `internal/component/command/pipe_columns.go` - the two operators: parse, validate, and the payload selection
- `internal/component/command/pipe_columns_test.go`
- `test/ui/display-fill-select.ci`
- `test/ui/display-fill-remainder.ci`
- `test/ui/display-fill-completion.ci`
- `test/ui/display-fill-filtered-command.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | A pipe operator is presentation, and `ai/rules/cli.md` keeps presentation out of YANG. The saved per-command default is a separate spec and owns that config surface |
| YANG validation constraints | N-A | No YANG leaf is added |
| YANG custom validators | N-A | No YANG leaf is added |
| CLI commands/flags | No | No command or flag is added; a generic pipe operator is added |
| CLI grammar (keyword before value) | Yes | `display` and `fill` are the keywords and their arguments are the values, matching `\| match <pattern>` and `\| first N` |
| Editor autocomplete | Yes | `internal/component/command/completer.go`, the AC-8 requirement |
| Functional test for new RPC/API | No | No new RPC; four `.ci` tests cover the operator |
| Pipe completeness | Yes | `pipeDisplay` and `pipeFill` are both classified in `foldFilters`, which AC-9 pins; every other operator keeps working |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new file path, socket, service, port or binary |
| Prometheus counters/metrics | N-A | No observable runtime state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family surface change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features/formatting.md` |
| 2 | Config syntax changed? | No | No config surface in this spec |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` gains the operator |
| 4 | API/RPC added/changed? | No | No RPC changes |
| 5 | Plugin added/changed? | No | No plugin boundary changes |
| 6 | Has a user guide page? | Yes | `docs/features/formatting.md` |
| 7 | Wire format changed? | N-A | Scope is cli |
| 8 | Plugin SDK/protocol changed? | No | Nothing enters the payload contract except the operator's own field selection |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Scope is cli |
| 10 | Test infrastructure changed? | No | New `.ci` tests use the existing runner |
| 11 | Affects daemon comparison? | No | No feature-parity claim changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/commands.md` operator table |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing enters an inventory |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `pipe.go`, `pipe_table.go`, `completer.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/features/formatting.md` lists the operators; both were added and the rendered examples re-checked |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- both operators parse, survive classification, and reach the renderer
   - Tests: `TestParsePipeDisplayJoinsFields`, `TestParsePipeFillWayAndReverse`, `TestValidatePipesRejectsBadColumnArguments`, `TestColumnOpsSurviveFoldFiltersOnFilteredCommand`
   - Files: `internal/component/command/pipe.go`, `internal/component/command/pipe_columns.go`
   - Verify: `pipeDisplay` and `pipeFill` appear in `knownPipeOps` AND in the `foldFilters` switch. A run of `show bgp rib | display prefix` must reach `ApplyPipes`; the wiring test fails while the renderer ignores it
2. **Phase: Sequence and selection** -- `orderKeys` honors the request and `fillKeys` the remainder
   - Tests: `TestDisplayAloneSelectsAndSequences`, `TestDisplayThenFillAlpha`, `TestFillOverallOrdersByRenderedWidth`, `TestFillAloneOrdersEveryField`, `TestFillDefaultUsesTheDeclaredOrderForTheRemainder`, `TestFillDefaultAloneMatchesTheBuiltInOrder`, `TestDisplayOverridesRegisteredOrder`, `TestDisplayUnknownFieldIsInert`, `TestDisplayKeepsParentKeyColumn`
   - Files: `internal/component/command/pipe_table.go`, `internal/component/command/pipe.go`
   - Verify: AC-1 to AC-4, AC-10 to AC-12. The request is a distinct field, never appended to `tableStyle.orders`, and it does not suppress the declared order for the fields it does not name
3. **Phase: Selection reaches the programmatic formats, sequence does not**
   - Tests: `TestDisplaySelectionReachesJSON`, `TestDisplaySelectsANestedRecordTheSameWay`, `TestColumnOpsAbsentLeavesOutputUnchanged`, `TestColumnOpsAfterTheFormatOperator`, `TestDisplayThenMatchOnDroppedColumn`
   - Files: `internal/component/command/pipe_columns.go`
   - Verify: AC-5 and AC-6. `spec-cli-column-order`'s AC-5 must still pass unchanged
4. **Phase: Completion**
   - Tests: `TestCompleteDisplayFieldsFromRegistry`, `TestCompleteDisplayFieldsAfterFirst`, `TestCompleteDisplaySkipsTypedFields`, `TestCompleteFillOffersKeywordsOnly`, `test/ui/display-fill-completion.ci`
   - Files: `internal/component/command/completer.go`
   - Verify: AC-8 on the SSH CLI, matching on the LAST token rather than the whole tail, and never a mixed set
5. **Phase: Docs**
   - Files: every doc row answered Yes above
   - Verify: the rendered examples match real output

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-12 has an implementation at file:symbol |
| Feature completeness | Tab completion works on a real SSH session, not only in a unit test |
| Correctness | The request replaces rather than merges for the fields it names, and does NOT suppress the declared order for the fields it does not; the parent-key column survives selection |
| Naming | `display` and `fill` do not collide in `knownPipeOps` or with any registered `PipeFilter` |
| Data flow | The operator's request never becomes a per-command registration; it lives for one command invocation |
| Rule: `ai/rules/cli.md` | Every pipe operator still works on every command, including filtered ones (AC-9) |
| Rule: `ai/rules/stale-comments.md` | `orderKeys`' doc comment matches what it now does |
| Rule: `ai/rules/simplicity.md` | Two operators, one argument kind each. No positional variants and no mode words beyond the two ways, per Known Limitations |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Both are known operators | `grep -n '"display"\|"fill"' internal/component/command/pipe.go` |
| They are classified, not silently dropped | `TestColumnOpsSurviveFoldFiltersOnFilteredCommand` and `test/ui/display-fill-filtered-command.ci` pass |
| Selection works | `TestDisplayAloneSelectsAndSequences` passes |
| Completion works on the real CLI | `test/ui/display-fill-completion.ci` passes |
| The built-in path is untouched | `spec-cli-column-order`'s unit and `.ci` tests still pass |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Field names come from operator input. They index a map and are never executed, interpolated into a query, or used to build a path. Confirm an arbitrary name can only ever be inert |
| Resource exhaustion | A very long field list is bounded by the payload's key count for effect, but the parse is not. Confirm the rank map build is linear and the list length is bounded or harmless |
| Information leakage | Selection HIDES fields, which is the operator's explicit request and applies only to their own output. Confirm it can never hide a field from an audit log or a recorded transcript |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| `\| display` or `\| fill` silently does nothing on one command | `foldFilters` classification is missing for that path. Fix the classification, never the test |
| A `spec-cli-column-order` test goes red | The override leaked into the built-in path. Fix the leak |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- Splitting the request into "sequence" and "selection" is what resolves its interaction with the programmatic formats. Sequence is presentation and stops at `table`/`text`; selection is a data question the operator asked out loud, so it travels.
- The `foldFilters` switch having no `default:` arm means adding a pipe kind is a two-place change, and the second place fails silently rather than at compile time. That is worth a test rather than a comment.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Two operators, each taking ONE kind of argument | One `\| order` taking field names and a mode word together | Owner decision, 2026-08-19. A token cannot be a field name in one position and a keyword in another: nothing can complete it, and a command whose answer carries a field called `alpha` makes the grammar ambiguous rather than merely awkward. Splitting `display` from `fill` removes the class |
| The `*` remainder token is dropped | `\| order a b *`, with `*` as the trailing "and the rest" | Owner decision, 2026-08-19. An unquoted `*` is expanded by the user's shell before ze sees it, so the one syntax the operator would type most often is the one their shell eats. `\| fill` says the same thing in a word no shell touches |
| No positional variants of the remainder token | `>*` and `<*` | `>*` is a synonym for `*`. `<*` would put the remainder before the fields the operator named, which defeats the reason they named them. An option with no case is machinery (`ai/rules/simplicity.md`) |
| `\| fill` with no way means the command's OWN declared order | Make a bare `\| fill` a synonym for `\| fill alpha`; or refuse it | Owner decision, 2026-08-19. The built-in declaration is an operator judgment about what leads, so the cheapest thing to type asks for it. Making it a synonym for `alpha` would leave no way to ask for the declaration at all |
| `reverse` is a bool beside the way, not a way of its own | `alpha-reverse` and `overall-reverse` constants | It flips whichever way is in force, including the default one, so doubling the constants would double again with the next way |
| Selection reaches JSON; sequence does not | Refuse `\| display` under a programmatic format; or ignore it silently | Refusing errors on `\| display a b` whenever `environment cli format default json` is committed, which the operator never typed. Ignoring drops a request silently, the worst of the three |
| The request is a separate field on `tableStyle` | Append the displayed names to `tableStyle.orders` | `bestColumnOrder` ranks by hit count, so a two-field request would lose to a nineteen-field registered one. The request must not compete with what it overrides. It also must not SUPPRESS the registered order for the fields it does not name, which is what a bare `\| fill` needs and what AC-11 pins |
| Selection is applied ONCE, to the payload | Select in `orderKeys` for the renderers and again over the JSON | One implementation cannot disagree with itself. `selectFields` walks the shapes `renderValue` walks and applies the rule `orderKeys` applies, so a nested sub-table and the JSON behind it carry the same fields |
| Both kinds are classified in `foldFilters` | Rely on them being generic | The switch has no `default:`, so an unclassified kind is dropped for every command with registered filters, with no error |

## Known Limitations
- No exclusion syntax. "Everything except these two fields" still means naming the other seventeen. This is the one modifier judged worth adding next, and it is left out here to keep the first version to the two operators the owner specified.
- Two ways only, `alpha` and `overall`. `title`, `length` and `width` were rejected as words.
- `| fill` orders the remainder, never the fields `| display` named. There is no way to say "these fields, sorted among themselves".
- Field completion offers nothing for a command that declared no built-in column order. Completion quality tracks the `spec-cli-column-order` rollout rather than this spec.
- Row ordering (sorting ROWS by a field's value) is neither operator and is not in scope.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
