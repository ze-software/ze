# Spec: config-enum-refusal-proposes-valid-values

| Field | Value |
|-------|-------|
| Status | verification |
| Priority | P1 |
| Release reason | Honor the approved consistent schema-derived enum refusal across operator entry points. |
| Scope | config |
| Depends | - |
| Phase | 6/6 |
| Handoff | verify |
| Updated | 2026-11-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Closure continuation

On 2026-11-13 the owner answered the request to continue source reviews beyond
five through clean closure: "Autorized and continued." This one-off decision
clears the previous authorization blocker. The existing five-round findings
remain historical evidence; round 5 did not pass. The fresh independent
`closure_round6` context reviewed the committed implementation at
`b9c22dfb7d3e81eaae7d103d677bdf89d86dc52c` as a verification handoff.

Round 6 covers the early non-first-key sensitive-suffix repair and sibling
fix/framing paths; the later carriage-return codec repair in `configtext`
quoting and the config token readers, including actual parser/model and
operational entry-point tests; and `Browser.Close` preserving context budget
values while replacing cancellation with a fresh finite cleanup deadline.
The review also applies every always-in-scope class from `ai/rules/planning.md`.
Its separate producer lenses cover correctness and boundary symmetry, security
and test preservation, plus the explicit `docs/contributing/ze-go-style.md`
pass and the spec audit. The source verdict is recorded below. Main owns the
native record/check and commits A and B, followed by the dependent BMP closure. No push is authorized.

WIP-cap exception, 2026-11-12: the owner granted a one-off cap exception for
this spec, naming it in answer to "grant a cap exception so I take that spec
next". It is the dependency that lets
`plan/release/fixes/spec-bmp-route-action-redistribute-acts-as-monitor.md`
close. The exception covers this spec's claim only.

This spec was reworked on 2026-11-12 (Disposition: REWORK). The design of
2026-10-14, reviewed in 17 rounds, is in git history at `09be99b7db`. It
predates the operator error (`opererr`), the parser's union check, the parser's
line error and refusal path, YANG constraint messages, the argument-fault
refusal and multi-leaf-key list addressing. This version rebuilds the design
on those mechanisms. It keeps every owner answer and every acceptance
criterion's intent. Review rounds 18 to 25 judged the rework and the
`top <command>` form the owner's Q-9 answer asks for; their findings are applied
here, and round 25 found no further round owed (Review Gate). The owner
approved the design on 2026-11-12 ("approve").

## Task

Owner decision, 2026-10-13: error messages must be user friendly and "propose what
is known is valid from the yang". Direction chosen by the owner, verbatim:
"M2 go with (c), friendly errors everywhere". Option (c) is one generic
friendly refusal for every YANG enum, on every path. Owner answer Q-5, verbatim:
"b": range, length and pattern refusals become friendly in this spec too, on
every surface the enum refusal reaches.

Owner rule (`ai/rules/user-facing-errors.md`): every message a user sees says
what is wrong in plain words, why, and the exact command that fixes it. A
refusal an operator reads is an `opererr.Error`, and the fix is the exact
command alone on the last line, rendered `fix: <command>`.

Owner answer Q-9, 2026-11-12: the editor's `top` takes an optional command,
run once with the position at the root ("a transient change of directory for
the command behind, whatever the command is"). So a fix line that names a path
outside the operator's `edit` context is `top set <full path> <value>`, the
exact command from any position, and every command the editor accepts works
behind `top`.

Today a value outside a YANG enumeration is refused with
`invalid enum: "banana" (expected one of: enable, disable, require, refuse)`.
The refusal has no setting path, no meaning for each accepted value, no
closest-match hint and no `fix:` line. Its text also differs by path (Current
Behavior). The SSH editor `set` names no accepted value (`(expected enum)`), and
the web terminal and the web form do not refuse at `set` at all.

Goal: one refusal for every YANG enumeration, range, length and pattern miss,
built from the schema alone, reached through every path. The paths are config
file load, `ze config validate` (text and JSON), offline `ze config set`, the
editor `set`, `insert`, `load` and `commit`, the editor `set cli format`
command, the web UI form and web CLI terminal `set`, REST, gRPC and gNMI config
sessions, reload, and operational command arguments. The refusal is an
operator error:

| Part | Content | Source |
|------|---------|--------|
| problem (line 1) | the setting path as an operator types it, words separated by spaces, list keys included; the quoted value (a placeholder on a sensitive leaf); the kind's marker; ` (did you mean X?)` when one accepted value is close and the leaf is not sensitive | the path the producer holds; `suggest.Command` in the `UnknownKeywordHint` hint form |
| why (line 2 and after) | for an enum, `<leaf> accepts only these values:` then one line per accepted value, in YANG declaration order, with its one-line `ze:help`; for a range, length or pattern, the accepted bounds or format, or the YANG `error-message` the constraint declares | `yang.EnumNamesDeclared`, `yang.EnumValueSummaries`, `LeafNode.Ranges`/`Lengths`, the one format reader, `ConstraintMessages` |
| fix (last line) | `fix: ` and the exact text the operator types on that surface, the refused word replaced by the suggestion, or by the alternation `<a\|b\|c>` (or the bound placeholder) when there is none | the surface frame (Key Design Decisions, fix table) |

The kinds:

| Kind | Problem marker | Why line, no `error-message` declared | Fix placeholder |
|------|----------------|---------------------------------------|-----------------|
| enum | `is not an accepted value` | `<leaf> accepts only these values:` and the value lines | the suggestion, else `<a\|b\|c>` in declaration order |
| range | `is outside the accepted range` | `<leaf> accepts a value in <bounds>` (declared range, else the base type's width) | `<bounds>` |
| length | `is outside the accepted length (N characters)`; no count on a sensitive leaf | `<leaf> accepts <bounds> characters` | `<bounds> characters` |
| pattern | `is not in the accepted format` | `<leaf> accepts <format text>`: the nearest typedef's `ze:help`, else the first sentence of its `description`, else the leaf's own `ze:help`, never the regular expression; `a value of type <typedef-name>` when none answers | the typedef name `<rate>`, or `<value>` for an inline pattern |

When the constraint declares a YANG `error-message`, that message is the why
line (RFC 7950 Section 7.5.4.1). No suggestion is offered for range, length or
pattern. A missing value (`route-refresh;`) is the enum refusal with the problem
`<path>: needs a value`. The other `ValidationError` kinds (missing, type,
reference, cardinality, unknown) hold no operator error and keep today's text
(`<type> error at <path>: <message>`).

No per-leaf special case. Known dependant:
`plan/release/fixes/spec-bmp-route-action-redistribute-acts-as-monitor.md` closes
only after this spec, because its A-2b is validated by this spec's tests.

### Text samples

`<help of X>` is the one-line `ze:help` of enum value X, read from the YANG. A
value line is two spaces, the name, then, when the value declares a help, enough
spaces to align every help two columns after the longest name. A value with no
help is its name alone, with no trailing space.

`ze config validate` of a hierarchical file (File frame). The line prefix and
the continuation layout are the ones `ze config validate` already gives an
operator error inside a `config.LineError` (`SplitLine`); no slash path is
printed before the problem:

```text
line 12: bgp peer peer1 session capability route-refresh: "requre" is not an accepted value (did you mean require?)
route-refresh accepts only these values:
  enable   <help of enable>
  disable  <help of disable>
  require  <help of require>
  refuse   <help of refuse>
fix: route-refresh require;
```

The same value with no close match (`banana`): no hint, and the fix line is
`fix: route-refresh <enable|disable|require|refuse>;`.

SSH editor at the root context, offline `ze config set`, web terminal (Set frame):

```text
bgp peer peer1 session capability route-refresh: "maybe" is not an accepted value
route-refresh accepts only these values:
  enable   <help of enable>
  disable  <help of disable>
  require  <help of require>
  refuse   <help of refuse>
fix: set bgp peer peer1 session capability route-refresh <enable|disable|require|refuse>
```

Inside `edit bgp peer peer1`, a refused `set` the operator just typed keeps the
relative form typed: `fix: set session capability route-refresh <enable|disable|require|refuse>`.
A commit refusal inside any context names the leaf from the root behind `top`:
`fix: top set bgp peer peer1 session capability route-refresh require`.

Range, File frame (`cost` is `range "1..65535"` in `ze-ospf-conf.yang`):

```text
line 7: ospf interfaces interface eth0 cost: "0" is outside the accepted range
cost accepts a value in 1..65535
fix: cost <1..65535>;
```

Pattern, Set frame (`l2tp shaper default-rate`, typedef `zt:rate`, whose
description holds no sentence end, so it is taken whole):

```text
l2tp shaper default-rate: "fast" is not in the accepted format
default-rate accepts Traffic rate string (for example 10mbit, 100kbit, 1gbps)
fix: set l2tp shaper default-rate <rate>
```

Command argument, one fault (the route restates it with the whole command):

```text
send bgp raw encoding: "gzip" is not an accepted value
encoding accepts only these values:
  hex  <help of hex>
  b64  <help of b64>
fix: send bgp 192.0.2.1 raw <hex|b64> DEADBEEF
```

Command argument, two faults: the `ArgumentFaults` refusal, whose why holds one
`- ` line per fault. A value fault's entry is its problem line, then its why
lines indented under it; it never carries its own `fix:` line. The refusal's one
fix line is the whole command with every faulted value replaced by its
placeholder and every missing argument by its usage.

In-entry key-leaf refusal (structural, not a value refusal; no value list and
no marker):

```text
<ospf-path> redistribute connected source: source is the key of list redistribute and cannot be set inside an entry
rename changes the key of an existing entry, and set would store source as a second value
fix: rename <ospf-path> redistribute connected to <new-key>
```

`ValidateContent` with several refusals (REST, gRPC and gNMI commit):

```text
config validation failed:
  bgp peer peer1 session capability route-refresh: "requre" is not an accepted value (did you mean require?)
  route-refresh accepts only these values:
    enable   <help of enable>
    ...
  fix: set bgp peer peer1 session capability route-refresh require
  ospf interfaces interface eth0 cost: "0" is outside the accepted range
  cost accepts a value in 1..65535
  fix: set ospf interfaces interface eth0 cost <1..65535>
```

## Required Reading

### Architecture Docs
- [ ] `ai/rules/user-facing-errors.md` - binds every message in this spec
  → Constraint: a refusal an operator sees is an `opererr.Error`; problem, why, then `fix: <exact command>` alone on the last line; a layer adds context to the first line only and never appends after the fix.
- [ ] `docs/architecture/cli/error-surface.md` - "The operator error", "The message area is two rows", "How an error reads"
  → Constraint: `Error()` renders problem, why (`: <cause>` when a cause is set), `fix: <fix>`; the web reads `Problem`, `Why`, `Fix`; the SSH feedback row is one row, so a multi-line refusal goes to the viewport.
  → Constraint: `./le arch operator-error check` (`internal/le/arch/operatorerror`, `classifyError`) refuses only an `errors.New`, an `fmt.Errorf` or a zero `opererr.Error` literal at a registered handler's return, and passes a callee's error unjudged; `./le arch error-inventory check` refuses a plain message a change adds at a surface writer. Neither proves a refusal reaches its writer typed, so this spec's proof is the `errors.As` tests (AC-21).
- [ ] `docs/contributing/ze-go-style.md` - "Every error is handled"
  → Constraint: `opererr.New` and `opererr.Wrap` panic on an empty part; its fields are private and immutable; a producer builds the error once, a layer wraps only to add a fact it holds.
- [ ] `ai/rules/cli.md` - validation errors surfaced to an agent, editor grammar
  → Constraint: the JSON diagnostic keeps its codes (`config-yang-enum`, `config-yang-range`, `config-yang-length`, `config-yang-pattern`) and its keys (`expected`, `actual`, `path`, `code`).
  → Constraint: an editor verb's forms are declared once (`editorHelp` table, `internal/component/cli/editor_help.go`), and help and completion derive from it.
- [ ] `ai/rules/config.md` - enum help text
  → Constraint: an `enum` value carries its one-line text as `ze:help` alone; an enum `description` is read by no surface.
- [ ] `docs/architecture/config/syntax.md` - "Unknown keys", "multi-leaf list keys"
  → Decision: the unknown-key refusal writes ` (did you mean X?)` with `suggest.Command`; the enum refusal reuses the helper and the wording.
  → Constraint: a list address is the first key value, then `name value` for each other key leaf (`list_key.go`); every list path walker reads it through `ListPathStep`, and the one creator is `EnsureListEntry`.
- [ ] `docs/architecture/config/yang-config-design.md` - validation layers, "Constraint error messages"
  → Constraint: a declared `error-message` speaks in place of the generic text on every path (`ConstraintMessages`).
- [ ] `docs/architecture/api/commands.md` - YANG-typed argument validation
  → Constraint: a command line's faults are all named by `ArgumentFaults`, and every route restates them through `RestatedForLine`.
- [ ] `docs/guide/config-editor.md` - navigation (`edit`, `up`, `top`), secrets in command answers
  → Constraint: `DisplayMessageAtPath` / `maskSecretInMessage` stay as a second defence; the refusal is built masked.
  → Constraint: this page documents `top`, so it gains the `top <command>` form in the phase that adds it.

### RFC Summaries (Scope: protocol)
- N-A for a summary edit: no wire behaviour changes and no row of `rfc/short/rfc7950.md` is added or changed. RFC 7950 Section 9.6 (enumeration) is what the RFC-tagged tests prove; they assert `ErrTypeEnum`, `Expected` and `Got`, never the message text, and this spec keeps all three.
  → Constraint: every `rfc/discrimination/rfc7950.json` record whose producer this spec edits goes stale and is re-recorded with `./le rfc record` in the commit that edits it. The set counted 2026-10-13 was 25 (`validateEnumeration` 2, `yangToLeaf` 1, `walkTree` 5, `refuseInvalidCustomSections` 5, `validateString` 4, `checkYangRange` 3, `validateUnsigned` 1, `validateDecimal64` 4). 3e94cb7c40 has edited `validator.go` since, so each phase recounts its producers with `./le rfc check` before it commits.
  → Decision: the ranged decimal64 fix is RFC 7950 Section 9.3 behaviour now enforced by the parser; `TestValidateLeafValueDecimal64Range` is untagged (the one Section 9.3.4 row is the presence obligation, which it does not prove).

**Key insights:**
- The operator error exists and is the only refusal shape allowed; this spec adds no error type beside it.
- The typed facts the surfaces need (path segments, accepted values, kind, value, secret flag) live on the existing `configyang.ValidationError`, which holds the operator error and unwraps to it.
- The parser already checks unions (`validateUnionValue`), already builds a refusal path with keys on unwind (`prependSyntaxPath`, for a refusal wrapped in `syntaxRefusal`), and returns a typed line error (`LineError`); reload already prints one `parse config:`.
- The creator's key rule, stated once: an address with `ListAddress.Tokens == 0` is not judged; every other address is. Only `ListPathStep`'s anonymous arm and `materializeMetaPaths` (stored change-file keys read back) build an address with `Tokens == 0`; every address built from typed tokens has `Tokens >= 1` (`ParseListAddress`, `editor_walk.go`, `editor_commands.go`). So a typed `default` or `x#3` is judged, and stored `KeyDefault`, `#N` and change-file keys are not.
- `top <command>` needs no new state machine: the SSH editor already runs each dispatched command on a copy of the model (`executeCommand` has a value receiver) and applies the position only from the result's `clearContext` and `newContext` (`handleCommandResult`); `show <path>` and `load <path>` already change the position for one command and put it back. The verbs `handleEnter` takes before dispatch (`run`, `exit`, `quit`, `stop`, `restart`, `clear`) act on nothing the position decides (`clear` reads it only to redraw the view at the real position), so `top` in front of them is dropped.
- A command answers either a view it was asked for (`show`) or an incidental redraw of the current position (`set`, `delete`, `commit` and the rest). Only the first may be built on the copy; the second is `refreshConfig`, resolved at the real position, so behind `top` nothing draws a position the prompt does not name.
- A fix line names a command the operator types where they are, so it is framed by one function from the operator's real position, which `top` saves on the copy beside the emptied resolution position.

## Current Behavior (MANDATORY)

Read at HEAD `e660eff82e`, 2026-11-12; the `internal/component/cli` and `internal/component/web` rows for `top <command>` re-read at `1d565709a2` in round 24.

**Source files read:**
- [ ] `internal/core/opererr/opererr.go` (`New`, `Wrap`, `Error`, `Problem`, `Why`, `Fix`) - three required parts, private and immutable; `Error()` writes problem, why (plus `: <cause>`), `fix: <fix>`.
- [ ] `internal/component/config/schema.go` (`ValidateLeafValue`, `FlexNode`) - type, then the union arm (`validateUnionValue`: `invalid value %q: it matches no member type of the union (...)`), then enum (`invalid enum: %q (expected one of: %s)`, declared order from `LeafNode.Enums`), range (`outsideRange`, `validateNumericRanges`: `value %q outside range %s: set a value in %s`, or the constraint message), length (`validateLengths`), pattern (`validateLeafPatterns`: `value %q does not match pattern %q`, or the constraint message). A whole number past the type width already reports the range (`allDigits`). `parseNumericRangeValue` answers `range validation not supported for type string` for a ranged decimal64 leaf, so every parser refuses every value of the five ranged `zt:decimal-2` leaves. `ValueOrArrayNode` holds `ValidValues` (enum); `MultiLeafNode` and `BracketLeafListNode` hold `Patterns` only. `FlexNode` holds children, order and backends, and no key type.
  → Constraint: validation order type, union, enum, range, length, pattern is kept.
- [ ] `internal/component/config/parser.go` (`Parse`, `parseLeaf`, `leafValueFix`, `ParseAt`, `parseInactiveBracketMember`) and `parser_diagnostics.go` (`syntaxRefusal`, `prependSyntaxPath`) - `parseLeaf` wraps a value refusal as `syntaxRefusal{cause: p.errorf(...)}`, the `LineError` holding `invalid value for <leaf>: <err>\n  write: <fix>`; the refusal is built before its path is known. On unwind each caller prepends the node name or list key it owns (`prependSyntaxPath`, `parser_list.go` per entry, `ParseAt` with the context) into a slash-joined string, at no cost on success, but only when a `syntaxRefusal` is already in the chain; `syntaxRefusal.Error()` writes `<slash path>: <cause>`, and `SplitLine`'s `syntaxRefusal` arm does the same. `Parse` returns no tree on a parse error (`parseRoot`, `refuseUnterminated`).
- [ ] The other value and key arms of the hierarchical parser return a plain `p.errorf` `LineError` with no `syntaxRefusal`, so their refusal gets no path on unwind: `internal/component/config/parser.go` `parseInactiveBracketMember` (`invalid value for %s:` over `validateValuePatterns`); `internal/component/config/parser_list.go` the two list-key arms (`ValidateListKey` into `invalidListKeyError`, which answers `invalid peer name: %v` for a list named `peer` and `invalid key for %s: %v` otherwise), the inline-list child arms (`invalid value for %s.%s:`, two), the multi-leaf arm, the bracket arm, the value-or-array arms (`invalid value for %s: %q (valid: %s)` and its type arm), the value arm after them; `internal/component/config/parser_freeform.go` the freeform attribute arm (`invalid value for %s.%s:`). The inline-list key check in `parser_freeform.go` (`ValidateValue(node.KeyType, ...)`, `invalid key for %s:`) serves only inline lists, which `internal/exabgp/migration/exabgp.yang` alone declares.
- [ ] `internal/component/config/line_error.go` (`LineError`, `SplitLine`, `parseConfigError`) - both parsers return a `LineError`; `SplitLine` answers the line and the message apart, and renders an `opererr.Error` around a line refusal again without the line. `internal/component/config/loader.go` `parseConfigError` appends the legacy-syntax hint after the refusal (`\n\n<hint>`).
- [ ] `internal/component/config/parser_list.go` (`parseList`, `addParsedListEntry`, `validateInlineListChildValue`, `storeValueOrArray`), `parser_freeform.go` (inline lists judge `KeyType` with `ValidateValue`) and `setparser.go` (`validateValueOrArrayItems`, `walkAndSet`, `deleteFromList`, the key-only arm), `setparser_entries.go` (`walkAndSetListEntry`, `ensureFlexListEntry`), `setparser_meta.go` (`walkAndSetWithMeta`), `setparser_inline.go` - `parseList` stores the anonymous block under `KeyDefault` (`"default"`) with no key check; leaf-list texts `invalid enum: ...` and `invalid value for <leaf>: "x" (valid: a, b)` in goyang's sorted order; the set parsers judge the raw `tokens[0]` with `ValidateListKey` (so a typed `default` or `x#3` on a typed key is refused today) and wrap `line %d: invalid key for %s: %w`; the key-only arm creates through `AddListEntry` directly.
- [ ] `internal/component/config/tree.go` (`AddListEntry`) - a duplicate key is stored as `<key>#N` (ADD-PATH); `EnsureListEntry` creates only when the key is absent, so it never meets an automatic `#N`.
- [ ] `internal/component/config/list_key.go` (`ParseListAddress`, `ListPathStep`, `WalkListEntryPath`, `ResolveListEntryTarget`, `EnsureListEntry`, `ListEntryDestination`, `keyLeafMoveRefusal`) - the one address reader judges each non-first key pair with `ValidateLeafValue` (`invalid <leaf> for <list>: ...`) and never the first key value. `ParseListAddress` builds `Tokens >= 1`; `ListPathStep`'s anonymous arm builds `Tokens == 0`. `EnsureListEntry` is the creator of `internal/component/cli/editor_walk.go`, `editor_commands.go`, `internal/component/config/setparser.go`, `setparser_meta.go`, `setparser_entries.go`, `change_file.go` (`materializeMetaPaths`, which rebuilds entries from the stored meta keys with `Tokens == 0`, `"default"` for an edit under an anonymous entry among them). Lookup callers read an address of an entry that exists and treat a refused address as absence: `ListPathStep` in `internal/component/cli/editor_walk.go` (four), `editor.go`, `editor_annotated.go`, `editor_commands.go`, `internal/component/web/handler_config_walk.go` (two), `internal/component/gnmi/path.go`, `internal/component/gnmi/get.go`, `internal/component/config/edit_path.go`; the set parser's inactive marking (marks nothing) and `deleteFromList` (refuses the delete); `internal/component/web/workbench_enrich.go`.
- [ ] `internal/component/cli/editor_commit.go` (`DiscardSessionPath`, `memberOpFix`), `editor_commands.go` (write-through `set`, `openChangeFile`) - every discard, write-through `set` and commit reads the change file through `readChangeFile` and `materializeMetaPaths`; only `discard all` gets past an unparsable change file. The conflict fixes print `discard <root path> ...`.
- [ ] `internal/component/cli/model_commands_commit.go` (`cmdDiscardSession`) and `cmdDiscardPath` - hand the typed path to `DiscardSessionPath`/`DiscardPath` without `m.contextPath`, so `discard` takes a root path in every context and the `memberOpFix` lines are exact today.
- [ ] `internal/component/cli/model_commands.go` (`executeCommand`, `dispatchCommand`, `cmdTop`, `cmdUp`, `cmdEdit`) and `model.go` (`handleCommandResult`) - `executeCommand` has a value receiver, so `dispatchCommand` runs every dispatched command on a copy of the model; the position changes only when `handleCommandResult` applies the result's `clearContext` (set by `cmdTop`) or `newContext` (set by `cmdEdit` and `cmdUp`); no other verb sets either. `handleCommandResult` resolves `refreshConfig` at the real `m.contextPath`. `cmdTop` takes no arguments today: it answers `clearContext` and the root `configView`. An unknown verb answers `unknown command`, and `exit`/`quit` reaching `dispatchCommand` answer nothing (they are handled before). `dispatchCommand` splits a pipe (`FindPipeIndex`) before its verb switch, and `dispatchWithPipe` (`model_load.go`) accepts only `show` before the pipe, so `top show | errors` answers `command 'top' does not support piping` today. The only setters of `clearContext` and `newContext` are `cmdTop`, `cmdUp` and `cmdEdit`; the only other writes of `m.contextPath` are the two appliers (`handleCommandResult`, `applyResult`) and the save-and-restore of `cmdShow` and `cmdShowPipe`.
- [ ] `internal/component/cli/model.go` (`handleCommandResult`, `applyResult`) and `model_render.go` (`configViewAtPath`, `setViewportData`) - both appliers apply the position, run validation when `revalidate` asks, then take the first of `refreshConfig && m.hasEditor()` (`m.configViewAtPath(m.contextPath)` on the real model), `configView`, `output`; an `err` returns before any of it, and `statusMessage` is set after it in both. `configViewAtPath` reads only `m.editor` (a pointer, shared by the copy) and the path it is given; validation marks are applied later, in `setViewportData`, from the real model's `validationErrors`, `validationWarnings` and `showHints`. `setViewportData` chooses the tree-diff annotator when the real `m.contextPath` is empty and the line gutter otherwise, so a view built at any path other than the real position gets the wrong annotator: a root view applied at a non-root position is annotated as a sub-view, and today `show <path>` at the root (`cmdShow` extends the copy's position, so the view is a sub-view) is annotated with the root's tree diff. `finishPasteMode` (`model_keys.go`) and the `commit confirmed` timeout (`handleConfirmCountdown`) apply their results through `applyResult`; `pollDaemonWindow` keeps only the `statusMessage` of `windowReverted`.
- [ ] The views commands answer at `1d565709a2`: 24 results write `configView: m.configViewAtPath(m.contextPath)`, classified in Data Flow 7 ("The views a command answers").
- [ ] `internal/component/cli/model_keys.go` (`handleEnter`) - takes `run <cmd>` (operational dispatch), bare `run` (usage), `exit`/`quit` (leave config mode, or quit, with the pending-changes guard), `stop`, `restart` (daemon lifecycle with confirmation) and `clear` (screen) before dispatch. None acts on the position: the only read is `clear`'s `showConfigContent` (`model_commands.go`), which redraws `configViewAtPath(m.contextPath)` at the real position after emptying the scrollback. History: the `run <cmd>` arm appends `input` before it runs the operational command; bare `run`, `exit`, `quit`, `stop` and `restart` return before the history append and are not recorded; `clear` and every dispatched line are appended at the shared append before dispatch.
- [ ] `internal/component/cli/model_commands_show.go` (`show <path>`, `cmdShow`) - `show` answers through `configView` (or `output`) built at `m.contextPath`; with a path argument it extends the position for the call and puts the saved one back. `internal/component/cli/model_load.go` (`load <path>`) does the same with `defer`; `load terminal` answers `enterPasteMode` with `pasteModeLocation` and `pasteModeAction`, and `finishPasteMode` (`model_keys.go`) later calls `applyLoad` on the real model, which reads the real `m.contextPath`. The `commit confirmed` countdown (`handleConfirmCountdown`) reverts or confirms the whole configuration and reads the position only to draw the view at the real position.
- [ ] `internal/component/cli/model.go` (`commandCompleterInput`, `updateCompletions`, `cmdRun`) and `editor_help.go` (`editorHelp` table) - completion after `run ` strips the prefix and hands the rest to the operational completer; the help table declares `run` with its form, and `top` with none.
- [ ] `internal/component/cli/completer_validate.go` (`validateTokenPath`) - skips a key with `ListAddressSpan`, judging none; the store after it creates through `EnsureListEntry`.
- [ ] `internal/component/config/yang/validator_error.go` (`ValidationError`, `ErrorType`) and `validator.go` (`validateEntry`, `validateEnumeration`, `walkTree`) - `ValidationError` is a plain struct rendering `<type> error at <path>: <message>`; `validateEnumeration` answers `ErrTypeEnum`, `value %q is not a valid enumeration value`, `Expected` = goyang's sorted names joined `, `.
- [ ] `internal/component/config/yang/constraint_message.go` (`ConstraintMessages`, `LeafConstraintMessages`, `WithConstraintMessage`) - a declared `error-message` on a range, length or pattern, joined after the refusal with `: `.
- [ ] `internal/component/config/yang/enum.go` (`EnumNamesDeclared`, `EnumValueSummaries`) - declared order and per-value help.
- [ ] `internal/component/config/validate_sections.go` (`refuseInvalidCustomSections`, `SectionValidationError.Message`) - refusals joined by `; `; `Message()` renders `<section>: <path>: <reason>` and hides the whole reason on a sensitive leaf.
- [ ] `internal/component/config/cli/cmd_validate.go` (`runValidation`, `ValidateContent`, `yangErrorCode`) - a parser error becomes `config-parse` with the `SplitLine` message and no `expected`/`actual`; `ValidateContent` writes `config validation failed:`, then per error `\n  ` and the message (continuation lines unindented), and flattens to `errors.New`.
- [ ] `internal/component/cli/completer.go` (`ValidateValueAtPath`), `completer_validate.go` (`validateLeafValue`, `validateYangType`, `validateStringPatterns`, `validateUintRange`) - `invalid value %q for <leaf> (expected <hint>)`, the hint `enum` for an enum; the heuristic refuses every valid prefix (journal `helper-bypassed-by-an-open-coded-copy.md`, 2026-10-13).
- [ ] `internal/component/cli/model_keys.go` (`handleSetCLIFormat`) - `invalid format: x (valid: json, ndjson, table, text, yaml)` written to `statusMessage`.
- [ ] `internal/component/cli/model_commands_commit.go` (`CommitRefusal`, `commitValidationRefusal`, `appendIssueSummary`, `formatIssueList`) and `model_load.go` (`cmdCommitConfirmed`, `formatValidationErrors`) - issues joined by `; ` on one status row. None takes a position: each writes `ConfigValidationError.Message` as built. The web calls `cli.CommitRefusal` inside `EditorManager.runCommit` (`internal/component/web/editor_commit_window.go`), whose callers are `runWebCommit` (terminal, `cli_terminal.go`), `handleCLICommit` (`cli.go`), `CommitNow` and `handleCommitPost` (`handler_config_commit.go`, the commit modal, which reads no `path`); no framer holds a position today.
- [ ] `internal/component/cli/model_render.go` (`View`, `feedbackLine`) - `bottomRows = 3`; row 1 is `Error: <m.err>`; a multi-line error pushes the prompt off the frame. `cmdSet` always prepends `m.contextPath`.
- [ ] `internal/component/cli/validator.go` (`validateCore`, `parseError`, `validatePeer`, `formatPeerError`, the missing-mandatory warnings) - `validateCore(content)` parses first and returns on a parse error, since both parsers answer no tree then; it holds no position, and validation ticks independently of navigation. `formatPeerError` has an `ErrTypeEnum` arm (`must be one of`), reachable when the walk refuses a value the parser accepted (a `MultiLeafNode` or `BracketLeafListNode` member, which carries no enum list). The missing-mandatory warnings build a plain string with a root-form `(set bgp peer ...)` hint, which inside an `edit` context is not the command the operator types.
- [ ] `internal/component/web/cli_terminal.go` (`executeTerminalCommand`, `executeTerminalConfigMode`, `executeTerminalNav`, `executeTerminalSet`, `execListEntryOp`) and `internal/component/web/cli.go` (`dispatchCLICommand`, `handleCLITop`, `handleCLIEdit`, `handleCLIUp`, `handleCLICommit`, `handleCLISet`, `writeCLIResponse`, `splitSetArgs`) - both web editors keep a position: the page sends it as the `path` form value on every request. The terminal returns it in `terminalCommandResult.path`, replaced only when `executeTerminalNav` answers a non-nil new path. On the CLI bar only `writeCLIResponse` changes the page's position, and only `handleCLIEdit`, `handleCLITop` and `handleCLIUp` call it; `set`, `delete`, `show` and `commit` leave it alone. `handleCLITop` ignores its arguments, so `top set ...` on the bar today moves to the root and drops the rest of the line. `handleCLICommit` receives no position. No value check at `set`; the form checks the type only. These files import `internal/component/cli`, and share no dispatcher with the SSH editor.
- [ ] `internal/component/web/handler_config_form.go`, `handler_config_entry.go`, `error_fragment.go` (`WriteOOBError(path, message string)`) - the form checks the type only.
- [ ] `internal/component/api/config_session.go` (`SetSegments`), `internal/component/api/rest/server.go` (`writeSessionError`: 400, 403 when forbidden, for set and commit), `internal/component/api/grpc/server.go` (`sessionStatusError`: `InvalidArgument` unless forbidden, for set and commit), `internal/component/gnmi/set.go` (a path refused before `SetSegments` answers `InvalidArgument "update path: ..."`; `applyUpdate` returns the `SetSegments` error raw and `Set` passes it unwrapped, so the client receives `Unknown` while the metric records `InvalidArgument`) - REST, gRPC and gNMI sets reach `cli.Editor.SetValue`, then `walkOrCreate`, then `EnsureListEntry`. None of the three holds a position.
- [ ] `internal/component/command/argvalidate.go` (`validateEnum`, `validateUnion`, `validateUint`, `validateString`), `argfaults.go` (`ArgumentFaults`, `newArgumentFaults`, `Restated`, `RestatedForLine`, `wholeCommand`, `faultList.verdict`), `argbind.go` (`ValidateArgs`), `local_data.go` (`ValidateModelArgs`), `internal/component/plugin/server/arg_faults.go` (`restateArgumentFaults`), `internal/component/plugin/server/command.go` (the `several` read, `adoptablePositional`, `argFaultOutsideSelector`) - a lone fault is returned bare (`verdict`), so no route restates it; `newArgumentFaults` panics under two faults; `RestatedForLine` writes the whole command only when every fault is missing (`onlyMissing`), else the general correction (`argumentFaultsFix`); `Restated` lists a replacement fault by its `Error()`.
- [ ] `internal/component/bgp/plugins/cmd/update/update_text.go` (`parseCommonAttributeText`) - `invalid origin: %w` over the walk's `enum error at bgp/peer/update/attribute/origin: ...`.
- [ ] YANG names: no schema node is named `top` (the nearest is `leaf top-n-sources`, `internal/plugins/ddos/detect/yang/ze-ddos-detect-conf.yang`), so `top <command>` cannot be read as a path.

**The texts today:**

| # | Path | Producer | Text |
|---|------|----------|------|
| 1 | file load, `ze config validate`, reload, editor `load`, every commit | `ValidateLeafValue` via the parsers | `line N: <slash path>: invalid value for route-refresh: invalid enum: "x" (expected one of: ...)` plus `write:` (hierarchical) or no fix (set parsers) |
| 1b | a list key in a hierarchical file | `ValidateListKey` → `invalidListKeyError` | `line N: invalid key for <list>: ...`, or `invalid peer name: ...` for `peer`, no path |
| 2 | leaf-list members | `validateInlineListChildValue`, `storeValueOrArray`, `validateValueOrArrayItems`, `parseInactiveBracketMember` | `invalid enum: ...` or `(valid: a, b)` sorted, no path |
| 2c | enum leaf with no value | `parseLeaf` | `needs a value, one of a, b, ...` plus `write:` |
| 3 | tree walk | `validateEnumeration` | `value "x" is not a valid enumeration value` |
| 4 | editor `set`, offline `ze config set` | `ValidateValueAtPath` | `invalid value "x" for leaf (expected enum)` |
| 5 | editor `set cli format` | `handleSetCLIFormat` | `invalid format: x (valid: ...)`, success colour |
| 6 | web terminal `set`, web form | none | not refused until commit |
| 7 | command argument | `validateEnum`, `validateUnion` | `invalid value "x", expected one of: a, b` |
| 8 | `update text ... origin x` | `parseCommonAttributeText` | `invalid origin: enum error at bgp/peer/update/attribute/origin: ...` |
| 9 | union non-member, every parser | `validateUnionValue` | `invalid value "x": it matches no member type of the union (...)` |
| 10 | non-first key pair of a multi-leaf key | `ParseListAddress` | `invalid <leaf> for <list>: <ValidateLeafValue text>` |
| 11 | editor and web commit, a walk refusal of a value the parser accepted | `formatPeerError` | `peer X: "leaf" must be one of: a, b (got "x")` |
| 12 | editor missing-mandatory warning inside a context | `validator.go` | `(set bgp peer ...)`, a root path the context-relative `set` cannot take |
| 13 | SSH editor `top <anything>` | `cmdTop` | arguments ignored; moves to the root |
| 14 | CLI bar `top <anything>` | `handleCLITop` | arguments ignored; moves to the root, the rest of the line dropped |

**Pins of today's text (rewritten, never weakened):**

| Pin | Needle today |
|-----|--------------|
| `internal/component/config/parser_leaf_fix_test.go` `TestEnumLeafRefusalNamesTheReplacementLine` | `invalid enum: "true"` and the `write:` lines |
| `internal/component/config/schema_bool_mode_test.go` (`TestASN4IsACapabilityMode`, `TestBooleanLeafRefusesCapabilityMode`, `TestCapabilityModeLeavesShareOneTypedef`) | `invalid enum` |
| `internal/component/config/enum_test.go` `TestYANGLeafRestrictionsValidateParse` | `invalid enum`, `outside range` |
| `internal/component/config/pattern_test.go` (`TestYANGPatternRestrictionsValidateParse`, `...SetParse`) | `does not match pattern` |
| `internal/component/bgp/config/loader_test.go` (the peer-name case) | `invalid peer name`, under `if err != nil`, so it passes when no error comes back. Its comment names the resolve layer, and `internal/component/bgp/config/resolve.go` also writes `invalid peer name %q: must not be a valid IP address` for the key `10.0.0.1`, so the pin may not reach the parser arm. The implementation first establishes which producer fires for this input; it rewrites the needle to the kind's marker and the path `bgp peer <key>` only if the parser arm fires, leaves it otherwise, and puts the conditional assert to the `ze-test-judge` either way |
| `internal/component/plugin/server/send_test.go` `TestSendArgumentsAreDeclared` | `expected one of: hex, b64` and two more |
| `internal/component/command/argvalidate_test.go` `TestValidateArgStringEnum` | contains `summary` |
| `internal/component/plugin/server/rfc8907_command_test.go` (`TestMissingMandatoryOutranksAnUnplaceableToken` NEGATIVE `invalid value`; `TestPositionalErrorNamesTheOneOpenDefinition`; `TestDispatchNamesTheBoundOfAKeywordBoundValue`) | negative and positive argument texts |
| `internal/component/iface/cmd/manage_test.go` `TestUnitVIDBoundIsTheModels` | `value N out of range 1..4094` |
| `internal/component/cli/model_cli_format_test.go` `TestCLIFormatRefusesAnUndeclaredValue`; `model_commands_test.go` `TestSetCLIFormatInvalid` | `invalid format`, `valid: json, ...` |
| `internal/component/cli/model_commit_refusal_test.go` `TestCommitRefusalListsIssuesAndNamesForce`; `model_commit_mandatory_test.go` `TestCommitRefusesMissingMandatoryInSection`; `internal/component/web/editor_commit_window_test.go` | issues on the status row; the missing-mandatory `set ...` hint |
| `internal/component/cli/model_commands_edit_secret_test.go` `TestSSHCLISetNeverEchoesASecret`; `internal/component/config/cli/cmd_set_secret_test.go` `TestConfigSetRefusalNeverEchoesASecret` | `invalid value` becomes `is outside the accepted length` for these length-constrained secret fixtures; placeholder and no-secret assertions remain |
| `internal/component/cli/model_test.go` `TestTabOnListKeyShowsChildrenImmediately` | the old address-shaped peer key becomes the valid key `peer2`; Tab still must accept it, add the trailing space and immediately show peer children |
| `internal/component/cli/model_commands_test.go` `TestSetCommandRejectsInvalidValue`, `TestInsertValidatesValueType` | primitive refusal bodies remain `invalid uint16: "abc"` and `invalid IP: "not-an-ip"`, now prefixed by the full setting path; both keep their positive control and assert nothing invalid was stored |
| `internal/component/cli/model_test.go` `TestModelCommitBlockedOnErrors` | Q-7 (a) moves the full issue list from the status row to the viewport; the header remains on the status row and the candidate and active-config assertions remain |
| `test/parse/asn4-refuses-boolean-spelling.ci`, `test/parse/capability-leaf-refuses-presence-spelling.ci`, `test/parse/ipsec-application-override.ci` (`expected one of`, `deny, allow`: the comma-joined list becomes one value per line, so the needle becomes each value line), `test/isis/isis-auth.ci`, `test/ospf/ospf-config.ci`, `test/parse/image-server-invalid-rescue-auth.ci`, `test/parse/redistribute-tag-filter.ci`, `test/web/web-error-fragment.wb`, `test/editor/pipe/set-cli-format.et`, `test/ui/cli-argument-length-refused.ci`, `test/ui/cli-argument-refused-plugin-route.ci`, `test/ipsec/ipsec-dataplane-show.ci`, `test/plugin/session-editor-commit-verify.ci` | the old words |
| `test/plugin/policy-test-errors.ci` | NEGATIVE `reject=... invalid value` |
| `test/parse/bmp-route-action-refuses-redistribute.ci`, `test/reload/bmp-route-action-redistribute-refused.ci` | `route-action`, `redistribute`, `monitor` (and `reload error`): the new body keeps all three; strengthened with the marker `is not an accepted value` and the `fix:` line |

Stay green unchanged: the RFC-tagged enum tests (`rfc7950_radius_auth_method_enum_test.go`, `rfc7950_isis_auth_algorithm_enum_test.go`, `rfc7950_validator_yang_test.go` `TestValidateTree_EnumViolation`, `TestValidateTree_FamilyModeEnum`, `TestValidateTree_AddPathDirectionEnum`), `radius_auth_method_eap_enum_test.go`; the pins of the kinds that keep today's text, `internal/component/config/yang/validator_test.go` `TestValidationErrorTypeContract` (`unknown error at test/value: unspecified`) and `internal/component/config/yang/rfc7950_reference_walk_test.go` (`HasPrefix "reference error at sys/mgmt: "`); `TestSectionValidationErrorRedactsASensitiveLeaf`, `TestCLIFormatNamesMatchTheSchemaLeaf`, `TestUpdateText_OriginValidation_YANG`, `TestEditorCommitConfirmedBlocksOnPeerPipelineRefusal` (renamed from `...OnMissingLeakFilter` by 74c830e0d2), `test/plugin/session-editor-commit-force.ci`, `test/parse/firewall-priority-out-of-range.ci`; every existing test of `top` alone, which keeps its meaning. The two primitive-type pin rewrites above retain their rejection/acceptance verdicts while naming the full setting path.

The inventory is re-run at implementation over every `_test.go` under `internal/`, `cmd/`, `pkg/` and over `test/` for `invalid enum`, `expected one of`, `must be one of`, `not a valid enumeration`, `(valid: `, `invalid format`, `needs a value`, `or one of`, `expected enum`, `does not match pattern`, `outside range`, `outside allowed`, `out of range`, `matches no member type`, `invalid key for`, `invalid peer name`, `write:`, `error at `, `(set bgp`, `top `, `NotContains`, `reject=`.

**Behavior to preserve:**
- Every value refused today stays refused on every path that refuses it, a typed `default` or `<key>#N` on a typed key included; every value accepted stays accepted, except the named changes below.
- Every entry stored today (anonymous `KeyDefault`, duplicate `<key>#N`, keyed) stays reachable by every lookup, and every stored session change file reads back as today.
- `ValidationError.Type`, `Got`, and `Expected` as comma-joined names (now in declared order); the text of the kinds that hold no operator error.
- `LineError`'s line and the JSON diagnostic keys.
- A type refusal (not a number, not a boolean, decimal64 fraction digits) keeps its text.
- Secret masking.
- `UnknownKeywordHint` and `retiredKeywords` unchanged.
- `top` alone moves to the root, as today, on the SSH editor, the web terminal and the CLI bar.

**Behavior to change:**
- Every text in the table becomes the operator error of its kind.
- SSH editor `set`, `insert` and offline `ze config set` judge with `ValidateLeafValue` (union arm included) and the leaf-list member check, not the completer heuristic, which is deleted. Verdicts move toward the parsers' own: valid prefixes and hex keys now accepted at set; range, length and non-address pattern misses refused at set instead of at commit.
- A ranged decimal64 leaf accepts an in-range value on every parser (AC-41).
- An enum-typed `MultiLeafNode` or `BracketLeafListNode` value is refused by the parser, as the walk already refuses it (AC-5).
- The web terminal `set` and the form refuse at set (Q-6).
- A new list key outside its key leaf's restrictions is refused where the entry is created, on every surface (Q-8a), and the web terminal's key-leaf form stores a keyed entry.
- The in-entry key-leaf form is refused at set and no longer corrupts the session change file.
- `ze config validate` reports a parser refusal with its kind's code and `expected`/`actual`.
- A gNMI `Set` refused at the session answers `InvalidArgument`, not `Unknown` (AC-43).
- `top <command ...>` runs that one command at the root, whatever the command is, and keeps the position unless the command moves it, on the SSH editor, the web terminal and the CLI bar (Q-9); texts 13 and 14 go. The missing-mandatory hint names its fix with `top` inside a context (AC-51).
- The diff annotator follows the path a view was built at, so `show <path>` at the root is no longer annotated with the root's tree diff (`TestShowPathAnnotatesAtItsOwnPath`).

## Existing Mechanisms (MANDATORY)

Each row exists at HEAD `e660eff82e` and already does part of this job.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The operator error | `internal/core/opererr/opererr.go` `Error`, `New`, `Wrap`; `docs/architecture/cli/error-surface.md`, "The operator error" | Problem, why, `fix: <command>`; the only refusal shape the rule allows | extends: every refusal here is an `opererr.Error`; the accepted-value list sits in the why, as `ArgumentFaults` puts its fault list there, so the type needs no new part |
| The typed validation error | `internal/component/config/yang/validator_error.go` `ValidationError` | `Type`, `Path`, `Expected`, `Got`, `Message`, rendered `<type> error at <path>: <message>` | extends: for the enum, range, length and pattern kinds it gains path segments, accepted values with help, suggestion, secret flag, format text and the operator error built from them; `Error()` delegates to that operator error and `Unwrap` answers it; `WithFix` and `WithPath` answer copies with a rebuilt operator error. The other kinds keep their text. The parsers build it too. It replaces the separate refusal type the 2026-10-14 design put in `internal/core/suggest` |
| The parser's line error | `internal/component/config/line_error.go` `LineError`, `SplitLine` | Typed `line N:` refusal of both parsers; `SplitLine` gives line and message apart | extends unchanged: it carries the refusal; the 2026-10-14 typed line error and its indent table are dropped |
| Config token codec | `internal/component/config/list_key.go` `QuoteIfNeeded`, `splitQuotedTokens`, `readQuotedToken`; `tokenizer.go` `readWord` | Quoting and decoding retain complete words, and the tokenizer defines bare-word boundaries | extends the same codec: its scanner and quoter move to `internal/core/configtext` for YANG and command fixes; command parsing retains quote provenance to distinguish a literal pipe from an operator |
| The parser's refusal path | `internal/component/config/parser_diagnostics.go` `syntaxRefusal`, `prependSyntaxPath`; `parser_list.go` key prepends; `ParseAt` context | On unwind each caller prepends the name or list key it owns, keys and context included, at no cost on success, for a refusal wrapped in `syntaxRefusal` | extends: every value and key arm of the hierarchical parser wraps its refusal in `syntaxRefusal` (not only `parseLeaf`); `syntaxRefusal` keeps the segments as a list beside its string, and at the parse root (`Parse`, `ParseAt`) a refusal whose chain holds a `ValidationError` is rebuilt once: `newLineError(line, ve.WithPath(segments))` replaces the `syntaxRefusal`, so the problem carries the space-separated path and no slash prefix is printed; the 2026-10-14 path stack and its `Parser.path` extension are dropped |
| The union check | `internal/component/config/schema.go` `validateUnionValue`, `LeafNode.Union` | Every parser refuses a union value no member accepts | extends: its refusal becomes the friendly one, the enum members listed with help and the other members named in words |
| YANG constraint messages | `internal/component/config/yang/constraint_message.go` `ConstraintMessages`, `LeafConstraintMessages`, `WithConstraintMessage` | Reads the effective range, length and pattern `error-message`; the legacy string helper joins a message after a refusal | reuses the reader unchanged: the new kind-specific refusal builders use its message as the why line; the helper and this file are unchanged |
| Config session transport adapters | `internal/component/api/config_session.go` `Set`, `SetSegments`; `internal/component/api/grpc/server.go` `SetConfig`; `internal/component/api/rest/server.go` `handleConfigSet` | Sessions delegate mutations to `Editor.SetValue`; the transports pass its error to their existing status writers | reuses unchanged: the editor supplies the typed root-form refusal and these three files need no source edit |
| The list entry creator | `internal/component/config/list_key.go` `EnsureListEntry`, `ListEntryDestination` | The one creator every addressing surface calls; the rename and copy destination | extends: on creating an entry it judges the first key value with `ValidateListKey` unless the address has `Tokens == 0`, and strips no `#N` suffix; `ListEntryDestination` judges the destination key. This replaces the 2026-10-14 `Schema.ValidateTokenPathKeys`, `TokenPathSegments` and `CheckListKey`. `ParseListAddress` stays a reader and judges no first key |
| The argument-fault refusal | `internal/component/command/argfaults.go` `ArgumentFaults`, `RestatedForLine`, `wholeCommand`; `argbind.go` `ValidateArgs` | All faults of a line; whole-command fix when every fault is missing; one restating function on every route | extends: a typed value fault joins the missing-argument fault as a fault with a known correction; a line with one value fault is an `ArgumentFaults` of one, so every route restates it; this replaces the 2026-10-14 command-words parameter and `EnsureStep` field |
| The closest-match helper | `internal/core/suggest/suggest.go` `Command`; `internal/component/config/retired.go` `UnknownKeywordHint` | Edit distance, ` (did you mean X?)` | extends: the same helper and wording |
| The per-value help reader | `internal/component/config/yang/enum.go` `EnumValueSummaries`, `EnumNamesDeclared` | The one reader of a value's `ze:help` and of declared order | extends: `yangToLeaf`, the leaf-list builders and the `ArgDef` builder store the help |
| The SSH viewport refusal route | `internal/component/cli/model_load.go` (`Commit blocked by conflicts:`) | A long refusal shown in the viewport, the row keeps one line | extends: every multi-line `m.err` takes it |
| The editor's per-command copy and position result | `internal/component/cli/model_commands.go` `executeCommand` (value receiver), `dispatchCommand`; `model.go` `handleCommandResult`, `applyResult` (`clearContext`, `newContext`, `refreshConfig`) | Each dispatched command runs on a copy of the model; only the result's `clearContext` and `newContext` move the real position; `refreshConfig` is resolved at the real position | extends: `top <command>` dispatches the rest of the line on that copy with the position emptied; the real position moves only when the inner result sets `clearContext` or `newContext`; nothing is restored by hand. The current inventory has 26 incidental `refreshConfig` sites, resolved at the real position (Data Flow 7) |
| The one-command position change | `internal/component/cli/model_commands_show.go` (`show <path>`), `internal/component/cli/model_load.go` (`load <path>`) | Extend the position for one command and put the saved one back; the view the command answers is the one it built | extends: `top <command>` is the same pattern with the root in place of a deeper path, and it keeps the inner `configView` as `show <path>` does |
| The `handleEnter` interceptions | `internal/component/cli/model_keys.go` `handleEnter` (`run`, `exit`, `quit`, `stop`, `restart`, `clear`) | Verbs taken before dispatch; none reads the position | extends: `handleEnter` drops a leading `top ` before its interceptions, so these verbs behave behind `top` exactly as without it |
| The `run` completion prefix | `internal/component/cli/model.go` `commandCompleterInput` (the `run ` strip), `updateCompletions` | Completion after `run ` strips the prefix and completes the rest | extends as the model: completion after `top ` strips `top ` and completes the rest at the root context |
| The editor help table | `internal/component/cli/editor_help.go` `editorHelp` | One declaration of each verb, its help and its forms; help and the command reference derive from it | extends: `top` gains the form `top <command>` |
| The paste state | `internal/component/cli/model.go` (`pasteModeLocation`, `pasteModeAction`), `model_keys.go` `finishPasteMode` → `applyLoad` | `load terminal` enters paste mode; the later Ctrl-D applies the paste at the real position | extends: the paste state also records the position the `load` ran at, so `top load terminal ...` pastes at the root |
| The web editors' position | `internal/component/web/cli_terminal.go` (`terminalCommandResult.path`, `executeTerminalNav`), `internal/component/web/cli.go` (`dispatchCLICommand`, `writeCLIResponse` from the three navigation handlers) | The page sends the position with each request; only navigation answers a new one | extends: the terminal runs `top <command>` with an empty position and keeps the request's position unless `executeTerminalNav` answered a new one; the CLI bar calls `dispatchCLICommand` with the rest of the line and an empty position, which needs no signal because only the navigation handlers change the page's position |
| The editor's root-path commands | `internal/component/cli/model_commands_commit.go` `cmdDiscardSession`, `cmdDiscardPath`; `internal/component/cli/editor_commit.go` `memberOpFix` | `discard` takes a root path in every context, so its fixes are exact today | does not apply: `discard` fixes stay as they are |

New pieces, each with the reason no mechanism does it:

| New piece | Why no existing mechanism does it |
|-----------|-----------------------------------|
| A value-list renderer in `internal/core/suggest` (name column, aligned help, the alternation) | Both `config/yang` and `command` write the list, and `config/yang` imports `command`, so the shared writer must sit in a leaf package; no error path renders a value list today (completion renders its own menu) |
| A typed value fault in `command` (`ValueArgumentError`, beside `MissingArgumentError`): argument, typed value, problem, why lines, placeholder | `command` cannot import `config/yang`, so it cannot use `ValidationError`; `ArgumentFaults` reads its faults by type (`MissingArgumentError`) and needs the placeholder to write the whole command |
| The pattern format reader in `config/yang` (nearest typedef's `ze:help`, else its description's first sentence, else the leaf's `ze:help`) | No reader answers a typedef's text today; `GetHelpExtension` reads one statement |
| A decimal64 value type and range arm (`TypeDecimal64`, `LeafNode.FractionDigits`, an exported `ParseDecimal64` from the walk's `validateDecimal64`) | The parser has no decimal64 type (a defect, journaled in `declared-format-contradicts-payload.md`); the walk's own parse is reused, not copied |
| The in-entry key-leaf check | No walker refuses the first key leaf named inside an entry (`set <list> <key> <key-leaf> <v>`); `ParseListAddress` reads only non-first key leaves as address pairs |
| `top <command>` (Q-9) | No command runs another at the root: `top` ignores its arguments, `cmdSet` always prepends the context; the owner chose a `top` form so that it serves every command |
| A typed fix on `ConfigValidationError` (the leaf path and value of a missing-mandatory or commit refusal) | `validateCore` builds plain strings with no position, and validation runs apart from navigation, so the frame that knows the position must write the fix line; a string cannot be reframed |
| One fix framer, `cli.FrameFix` (Data Flow 5), and the operator's saved position under `top` (`Model.topOrigin`, `cliCommand.Operator`) | No framer holds a position today (`CommitRefusal`, `appendIssueSummary`, `formatIssueList`, the warning hint, `runCommit`); `memberOpFix` writes root-path `discard` lines, which need no frame because `discard` takes a root path everywhere. Under `top` the copy's `m.contextPath` is the root, so a framer reading it would print the plain form while the operator is inside a context; the operator's position must travel apart from the resolution position |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A config value as text: a file line, a `set` line (editor, offline, web terminal, REST/gRPC/gNMI session), a web form post, or a command argument.
- An editor line `top <command ...>` on the SSH editor, the web terminal or the CLI bar.

### Transformation Path
1. Schema build: `yangToLeaf`, the leaf-list builders (`ValueOrArrayNode`, `MultiLeafNode`, `BracketLeafListNode`) and the `ArgDef` builder store the enum names in declared order, the per-value help and the format text.
2. Value check: `ValidateLeafValue` (every parser, `ParseListAddress` pairs, `ValidateListKey`, the set surfaces) and the tree walk build a `ValidationError` holding the path segments it knows, the value, kind, accepted values with help, suggestion, secret flag and constraint message. `ValidateArgs` builds a `ValueArgumentError` from the same renderer.
3. Path: in the hierarchical parser every value and key arm builds the refusal with the leaf or list name alone and wraps it in `syntaxRefusal`; `prependSyntaxPath` collects the segments on unwind (keys and `ParseAt` context included), and at the parse root `Parse` and `ParseAt` rebuild it once with `WithPath`, which answers a new `ValidationError` and a new operator error whose problem names the space-separated path. The set parsers pass the tokens their walk consumed; the set surfaces pass the token path they hold; `EnsureListEntry` passes the list path. The walk threads segments from `ValidateTree`. `parseCommonAttributeText` re-roots to the command words.
4. Error chain: both parsers `LineError` → `ValidationError` → `*opererr.Error` (the hierarchical parser's `syntaxRefusal` is replaced at the parse root for this refusal and stays for the others); the set surfaces `ValidationError` → `*opererr.Error`. `ValidationError.Error()` renders the held operator error, whose fix is the set form; `Unwrap` answers it, so `errors.As` finds the `ValidationError` (JSON, frames) and the `*opererr.Error` (the web notice, `SplitLine`). A frame that holds another fix calls `WithFix`, which answers a copy with the same problem and why. The cause is not set on the operator error, so nothing is printed twice. A `ValidationError` of another kind holds no operator error and renders as today.
5. Rendering: `LineError` and `SplitLine` carry and print it on the file surfaces; the SSH editor moves a multi-line `m.err` to the viewport; `CommitRefusal` and `formatIssueList` write one issue per line; the web form reads `Problem`, `Why`, `Fix`; REST, gRPC and gNMI read `ValidateContent`'s joined error; JSON takes `code`, `expected`, `actual`, `path` from the `ValidationError`. A commit refusal and a missing-mandatory warning carry a typed fix (`ConfigValidationError`, leaf path and value).
   - Every fix line that is an editor command is written by one function in `internal/component/cli`, `FrameFix(fix FixCommand, operator []string) string`. `FixCommand` holds `Verb` (`set`, `delete`, `rename`, `copy`, `insert`), `At` (the position the arguments are relative to; nil for a root path) and `Args`. It answers `<verb> <args>` when `operator` equals `At` (the command typed where the operator is: a refused `set` inside a context keeps its relative form), `<verb> <At> <args>` when `operator` is the root, and `top <verb> <At> <args>` otherwise, which is exact from any position. A framer builds the string with it and hands it to `WithFix`, or writes it as the hint's `(...)`. File-form fixes (`route-refresh require;`), the `discard` fixes (`memberOpFix`, already root paths) and command-argument fixes (operational commands, no position) are not editor commands at a position and are not framed.
   - The framers and the operator position each passes: `cmdSet`, `cmdInsert`, `cmdRename`/`cmdCopy` refusals and `CommitRefusal`, `appendIssueSummary`, `formatIssueList`, the missing-mandatory hint, called from the SSH model with `m.operatorPosition()`; `executeTerminalSet`, `handleCLISet` and `runCommit` (through `runWebCommit` and `handleCLICommit`) with `cliCommand.Operator`; `handleCommitPost`, `CommitNow`, the offline `ze config set`, REST, gRPC and gNMI with nil (they hold no position: the commit modal reads no `path`), so they write the plain root form. The commit modal and `CommitNow` hold no CLI position at all, so an operator who types their fix into the CLI bar while it sits inside a context puts `top ` in front of it themselves; the modal's line is exact from the root. `CommitRefusal`, `appendIssueSummary` and `formatIssueList` gain the `operator []string` parameter; `EditorManager.runCommit` and `runWebCommit` gain it and pass it through.
   - The operator position on the SSH editor is `m.operatorPosition()`: `m.topOrigin` when `m.underTop` is set, else `m.contextPath`. Both fields live on the copy only: `cmdTop` sets them before it empties the copy's position, and the copy dies with the command, so the real model never holds them.
   - Inline validation marks are not framers: `highlightValidationIssues` writes `shortDiagnostic(e.Message)`, which drops the missing-mandatory hint, and the typed fix is kept beside `Message`, never written into it, so no mark carries a fix line.
6. Masking: the secret flag is set where the refusal is built (`config.LeafHoldsSecret` over a `LeafNode`; the entry's `ze:sensitive`/`ze:bcrypt` through one reader on the walk), so the value slot holds `config.SecretDataPlaceholder` and no suggestion is computed.
7. `top <command>` on the SSH editor:
   - `handleEnter` first drops a leading `top ` when the rest begins with a verb it intercepts (`run`, `exit`, `quit`, `stop`, `restart`, `clear`); none acts on the position, so they behave exactly as without `top` (`top exit` leaves config mode as `exit` does; `top run show version` runs the operational command; `top clear` empties the scrollback and redraws the real position, as `clear` does, the incidental-view rule below).
   - History keeps the line as typed, `top` included, on every arm: the strip changes only the verb the interception tests, never the string appended. The `run` arm appends `input`, which still holds `top run ...`; the shared append before dispatch appends the typed line. The arms that record nothing today (bare `run`, `exit`, `quit`, `stop`, `restart`) record nothing behind `top` either.
   - Every other line reaches `dispatchCommand`, which takes `top` with arguments before its pipe split, so `top show | errors` strips `top`, empties the copy's position and then reaches `dispatchWithPipe` with `show` (today the split comes first and answers `command 'top' does not support piping`). `cmdTop(args)` saves the operator position on the copy (`m.topOrigin = m.contextPath`, `m.underTop = true`, only when `m.underTop` is not already set, so `top top ...` keeps the outermost), sets the copy's `m.contextPath` to nil and dispatches the rest of the line, then answers the inner result with its `configView` and `output` as built. So `top show` displays the whole configuration.
   - The real position moves only when the inner result sets `clearContext` or `newContext`. The verbs that set them are exactly `edit`, `up` and `top` (`cmdEdit`, `cmdUp`, `cmdTop` are their only setters; `exit` and `quit` leave the mode or the session through `handleEnter` and set neither). Every other verb leaves the position alone.
   - The views a command answers. The current implementation has 26 incidental `refreshConfig: true` return sites. `handleCommandResult` and `applyResult` draw each at the real position. The table records the current producers, including successful commit and confirmation answers that the earlier 23-site inventory omitted. A refused commit and a verification with issues answer an issues viewport under Q-7 (a), and a valid `commit verify` answers a status alone, leaving the viewport unchanged; none is an incidental refresh:

     | Set | Count | Sites (function) |
     |-----|-------|------------------|
     | asked-for view, keeps `configView` | 1 | `model_commands_show.go` `cmdShow` (default working view, no columns) |
     | incidental, becomes `refreshConfig` | 2 | `model_commands_option.go` `cmdOptionErrors` (`hints`, `hide`) |
     | incidental | 6 | `model_commands_edit.go` `cmdSet`, `createListEntry`, `cmdDelete`, `runActivation` (`activate`, `deactivate`), `cmdInsert`, `listEntryMove` (`rename`, `copy`) |
     | incidental | 9 | `model_commands_commit.go` `cmdRollback`, `commitSaveAndReload`, `commitCandidateAndReload` (reload refused, reload accepted), `runCommitSession` (reload refused, committed), `cmdDiscardSession`, `cmdDiscardPath`, `cmdDiscard` |
     | incidental | 4 | `model_commands_session.go` `cmdShowChanges` and `cmdShowChangesAll` (`No pending changes`), `cmdShowChangesAll` (the count), `cmdDisconnectSession` |
     | incidental | 4 | `model_load.go` `cmdCommitConfirmed`, `cmdConfirm`, `rollbackConfirmed`, `applyLoad` |
     | incidental | 1 | `model_commit_window.go` `windowReverted` |

     The `show | changes` sites are `show` operations whose answer is the status line; the view under it is the current position's config, not what was asked for, so they are incidental. `cmdShowPipe` reaches `cmdShowChanges`, which reaches `cmdShowChangesAll` for `show | changes all`; these forms require a session. The other views `cmdShow` and `cmdShowPipe` build (annotated, alternate source, filtered) are asked-for views and keep `configView`; `cmdEdit` and `cmdUp` build the view at the new position with `newContext` and are unchanged.
   - Without `top`, the switch changes nothing visible. `configViewAtPath` reads only `m.editor`, a pointer the copy shares, and its path argument; the copy's `m.contextPath` at the return equals the real one, because no incidental verb moves the position and `cmdShow`/`cmdShowPipe` restore theirs. Validation marks, `showHints` and the diff-gutter switch are applied in `setViewportData` from the real model in both forms, after validation in both. The appliers' order is the same for both (position, validation, view, then `statusMessage`; an error returns before any of it), and no site sets both fields. `finishPasteMode` and `handleConfirmCountdown` apply through `applyResult`, which resolves `refreshConfig` the same way; `pollDaemonWindow` keeps only `windowReverted`'s status line. The one case where the two forms differ is a queued line whose copy was taken before an earlier queued result moved the position (`dispatchQueue` applies results in order): there `refreshConfig` draws the position the prompt names, which is the intent.
   - The annotator follows the view's own path. `setViewportData` takes the tree-diff annotator only when the real `m.contextPath` is empty, so any view built at another path is annotated for the wrong position: `top show` inside a context (a root view drawn with the line gutter), and today already `show <path>` at the root (a sub-view drawn with the root's tree diff). `configViewAtPath(path)` records `path` in `viewportData`, and `setViewportData` reads that recorded path instead of `m.contextPath`. This covers `top`, `show <path>` and every other view, so `cmdTop` marks nothing. Where the view's path equals the real position, which is every incidental view and `show` with no path, the choice is unchanged; the existing `show <path>` mismatch is fixed (`TestShowPathAnnotatesAtItsOwnPath`).
   - The operator position is kept as two fields, `topOrigin []string` and `underTop bool`, rather than one pointer-typed field: a nil slice is a valid origin (the root), so the bool says "set" without a pointer to a slice that every reader must dereference.
   - An inner error passes through as built, so `m.err` is the inner command's own refusal and the position is untouched; its fix line is framed with `m.operatorPosition()` (Data Flow 5), so `top set bgp peer peer1 session capability route-refresh requre` typed inside `edit bgp peer peer1` answers `fix: top set bgp peer peer1 session capability route-refresh require`, and `top commit` there answers `top set ...` fixes, never the plain root form.
   - Validation marks are recomputed over the whole tree (`runValidation`), which does not depend on the position.
   - Multi-step commands: `load terminal` returns `enterPasteMode`; the result also carries the position the `load` ran at (nil under `top`), stored beside `pasteModeLocation`, and `finishPasteMode` applies the paste at that stored position, so `top load terminal relative` pastes at the root. The `commit confirmed` countdown and its prompt revert or confirm the whole configuration and read the position only to draw the view at the real position, so they need nothing. The `exit` and `quit` confirmations read no position.
8. `top <command>` on the web. Pipes: the web terminal splits a pipe inside `executeTerminalNav` (`cli_terminal.go`, the `cli.FindPipeIndex` arm), which `executeTerminalConfigMode` calls after its own verbs, so `top` taken in `executeTerminalConfigMode` comes before the split and `top show | ...` reaches it with `show`; the CLI bar has no pipe handling (`parseCLICommand` in `cli.go` only tokenizes), so the question does not arise there. `cliCommand` gains `Operator []string`, the request's `path`, set where each editor parses the line and kept by the `top` recursion; every web framer reads it (Data Flow 5), never the resolution position. The terminal's `executeTerminalConfigMode` runs the rest of the line as a `cliCommand` with an empty position and the request's `Operator`, and answers the request's position unless `executeTerminalNav` answered a new one. The CLI bar's `dispatchCLICommand` calls itself with the rest of the line, an empty position and the same `Operator`; since only `handleCLIEdit`, `handleCLITop` and `handleCLIUp` write a new position (`writeCLIResponse`), every other verb leaves the page's position alone with no signal needed. Both replace today's `top` that ignores its arguments.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG module → config schema | `yangToLeaf` copies names, help, format text | No |
| config → cli / web / api | the error crosses as `error`; `errors.As` recovers `ValidationError` and `*opererr.Error` | No |
| daemon → REST/gRPC/gNMI client | `ValidateContent` joins the refusals with `errors.Join`; `errors.As` answers the first; set refusals keep each surface's status mapping, gNMI's now wrapped as `InvalidArgument` | No |
| plugin or API command → `update text` | `errors.As` on the validator's answer, re-rooted | No |
| browser → web editor | the position travels as the `path` form value and returns in the answer; `top <command>` answers the request's position unless the inner verb navigated | No |

### Integration Points
- `internal/core/opererr`, `internal/core/suggest`, `config.LineError`, `prependSyntaxPath`, `ConstraintMessages`, `ArgumentFaults`, `EnsureListEntry`, `dispatchCommand`/`handleCommandResult`, `handleEnter`, `editorHelp`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | each producer that refuses today builds the refusal where it refuses; `top <command>` dispatches through each editor's own dispatcher and interceptions |
| No unintended coupling | Yes | `config`, `config/yang`, `command`, `cli`, `web` import only `opererr` and `suggest` for it, both core leaves |
| No duplicated functionality | Yes | no second error type; one refusal path; one creator; one fault list; one `top <command>` rule per editor dispatcher, no command list |
| Zero-copy preserved | Yes | error path and editor dispatch only |
| Registration over hardcoding, outbound | Yes | no per-feature field or switch; `invalidListKeyError`'s `peer` special case is deleted; `top <command>` names no inner command |
| Registration over hardcoding, inbound | Yes | every value list comes from the YANG node at hand; no leaf name in any producer |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The closest-match bound suits enum values. Implemented (833c08af3f) as `suggest.Correction` (`internal/core/suggest/suggest.go`): a value is proposed only within one edit for an input of up to four characters, and within a third of its length (rounded down) beyond that. `suggest.Command` keeps `len(input)/2` for command suggestions, which never enter a fix line. `suggest.Command`'s bound proposed an unrelated value for a boolean spelling, so `color true;` now gets no suggestion and the alternation fix | the CLI and the unknown-key refusal used `suggest.Command`; its bound was too wide for a fix line | a misleading or missing hint | `TestCorrection` (`internal/core/suggest/suggest_test.go`), `TestEnumRefusalSuggestion` (`internal/component/config/yang/refusal_test.go`: `requre` gives `require`, `banana` none) | confirmed |
| A-2 | Every shipped enum value declares `ze:help` | `./le --name session doc yang-contract help-shape`, 2026-11-13: 1033 enum values rendered, 1033 with a summary | a value prints with no help | Current native census; AC-4 separately checks graceful rendering of a test value with no help | confirmed by the current census |
| A-3 | REST, gRPC and gNMI meet a value refusal only at commit; a key refusal at set | the session set calls `SetValue` with no value check; `SetValue` creates through `EnsureListEntry` | a set-time value refusal elsewhere bypasses the frame | AC-8, AC-9, AC-43: the three real-transport tests, each included in the 80-invocation API load selection | confirmed; promoted gRPC/gNMI clean phases passed and both native hook overlays produced the expected observer refusals |
| A-5 | A multi-line operator error renders on every surface that prints one | the parser, offline writer, SSH viewport, browser notice/terminal and API transports have explicit full-body controls | lines run together, or the SSH prompt is lost | AC-1, AC-6, AC-20, AC-23, `TestEditorMultiLineErrorKeepsThePromptRow`; full config/CLI runs, editor/API workflows and all three browser workflows | confirmed by the surface controls, native viewport overlay, completed browser 80-run campaign and all three promoted browser live cases |
| A-6 | The three functional fixtures exercise real REST, gRPC and gNMI config mutations | `register_config_refusal.go` explicitly registers the three clients with `StopsDaemon`; `plugin_fixture_01_api.go` issues REST set/commit requests, and `plugin_fixture_config_refusal.go` calls gRPC `SetConfig`/`CommitSession` and gNMI `Set` over pinned TLS with a token | the clients do not reach the session manager | the three promoted fixture tests; all three selected in every API load invocation | confirmed by current client/registration source and the 80-invocation API proof; the historical absence claim is superseded |
| A-7 | "Friendly errors everywhere" covers command arguments | owner answer Q-3 "yes" | - | owner | confirmed |
| A-9 | The creators of list entries are the ones the table below lists, each with its verdict | current non-test `AddListEntry` census over `internal`, `cmd` and `pkg`: 37 calls, comprising 14 config/editor stores and 23 ExaBGP migration stores, all covered by the table's mechanisms or exemptions | a creator keeps an unchecked key | current store-site census and producing creator/parser/restore source; the original key and stored-entry controls remain in the scoped config/CLI runs | confirmed source census; no unmatched store site found |
| A-10 | Dispatcher `ArgumentFaults` and model routes that can reconstruct their invocation restate through `RestatedForLine` | `restateArgumentFaults` delegates directly; `validateModelArgsAs` delegates after `modelLineHead` supplies the head. When that head is unavailable, the model retains the generic refusal; non-argument errors do not enter this contract | a route prints a value fault with no whole-command fix when it knows the line | AC-12 one-fault dispatcher/model controls; full command and plugin-server package runs passed | confirmed at the producing routes; the earlier claim about every command-line refusal was too broad |
| A-11 | Only `edit`, `up` and `top` set `clearContext` or `newContext` | the current non-test setter census finds only `cmdTop`, `cmdUp` and `cmdEdit`; `handleCommandResult` and `applyResult` consume those fields | another command that moves the position keeps its move under `top`, which is the rule's intent | current setter source; `TestTopKeepsPositionUnlessInnerMoves` covers navigation and representative nonmoving commands, including nested `top`; the final full CLI race run passed | confirmed by source, the named controls and the final full CLI race run |
| A-12 | On the CLI bar only the navigation handlers change the page's position | `writeCLIResponse` is called only by `handleCLIEdit`, `handleCLITop` and `handleCLIUp` (`internal/component/web/cli.go`, `internal/component/web/cli_terminal.go`) | - | read at HEAD | confirmed |
| A-13 | The `handleEnter` interceptions are `run`, bare `run`, `exit`, `quit`, `stop`, `restart` and `clear`, and none acts on the position | current `handleEnter` uses `lineBehindTop` for the verb tests and retains the original input for history; `clear` redraws the real position through `showConfigContent` | a verb behind `top` misbehaves | `TestTopInterceptedVerbsBehaveAsWithout`, retained in the passing final full CLI race run | confirmed by source, the named control and the final full CLI race run |
| A-14 | The 26 current incidental refresh sites render the same without `top` once they answer `refreshConfig` | `configViewAtPath` reads only `m.editor` and its path; both appliers resolve `refreshConfig` at the real position after the same steps; marks, hints and gutter come from the real model in both forms (Data Flow 7) | an operator sees a different view after `set`, `delete` or `commit` with no `top` | `TestIncidentalViewUnchangedWithoutTop`, `TestRemainingIncidentalViewsWithoutTop`, `TestWindowRevertedRefreshesThePosition`; the valid-verification control keeps its status-only answer | confirmed by focused and full CLI runs; both native wrong-position overlay walks passed clean and failed their named assertions under the break |
| A-15 | Schema value/key refusals and required-field editor fixes use `FrameFix`; existing root-path conflict `discard` fixes keep their contract | current CLI/web writer census: `FramedSetRefusal`, `FramedKeyRefusal`, `FramedWordRefusal`, `InEntryKeyLeafRefusal`, `refusalIssue` and `issueWithFix` reach `FrameFix`; issue renderers call `ConfigValidationError.Text`. The remaining direct `fix:` writers render conflict fixes | a fix printed behind `top` is wrong as typed | producing framer/issue/writer source, `TestFrameFix`, contextual replay controls and browser workflows | confirmed within the specified refusal scope, including completed browser load and promoted live correction replay; round 6 source review is CLEAN |

Creators that store a list entry, and the key verdict each takes:

| Creator | Verdict |
|---------|---------|
| `EnsureListEntry` (`internal/component/config/list_key.go`) and its callers that pass typed tokens (`Tokens >= 1`) | in scope: the new first-key check |
| `ListEntryDestination` (rename, copy) | in scope: the destination key is judged |
| `addParsedListEntry` and the two list-key arms (`internal/component/config/parser_list.go`) | in scope: the hierarchical parser's own `ValidateListKey` calls stay and now build the friendly refusal inside `syntaxRefusal`; `invalidListKeyError` is replaced, its `peer` special case deleted; its `KeyDefault` anonymous block is no typed key |
| set parser key-only arm (`internal/component/config/setparser.go`), `walkAndSetListEntry` (`setparser_entries.go`) and `walkAndSetWithMeta` (`setparser_meta.go`) | in scope, as implemented in 833c08af3f: the set parser keeps its own raw-token key check, `SetParser.lineKeyRefusal` (`setparser_refusal.go`), called at the three sites that judged at HEAD, and the creator `EnsureListEntry` judges new entries too. The creator returns a stored entry unjudged, so without the set parser's check a typed `default` or `x#2` naming a stored entry would be accepted (`TestSetLineKeyOnStoredEntryStaysRefused`). Both checks build the same refusal (`keyLineRefusal`). An `inactive` line on a stored `#N` entry reaches it as before (`TestInactiveLineReachesStoredDuplicateEntry`); an `inactive` line that creates a new entry is judged where the entry is created, its fix an `inactive` line |
| `change_file.go` `materializeMetaPaths` | exempt: it reads back keys that were judged when written, `"default"` for an edit under an anonymous entry among them, as addresses with `Tokens == 0`, so nothing is judged and every stored change file reads back as today. A refusal here would lock the session, since `discard <path>`, every write-through `set` and every commit read the change file through it |
| inline-list arms: `internal/component/cli/editor_walk.go`, `internal/component/cli/editor_commands.go`, `setparser.go` inline arm, `setparser_entries.go` inline arm, `setparser_meta.go` (two), `setparser_inline.go` | exempt: inline lists are declared only in `internal/exabgp/migration/exabgp.yang`, the ExaBGP input schema, whose file the hierarchical parser reads and judges (`parser_freeform.go`, `KeyType`); no Ze config module, editor or session reaches them |
| `parser_freeform.go` inline-list key check (`ValidateValue(node.KeyType, ...)`) and its stores | exempt from the new refusal: it judges only `exabgp.yang` inline-list keys, which no Ze operator surface writes; its check and text stay |
| `ensureFlexListEntry` (`setparser_entries.go`) | exempt: a `FlexNode` (`internal/component/config/schema.go`) holds children, order and backends and declares no key type to judge; the grep gate re-judges if one ever does |
| `editor_discard_path.go` restores | exempt: they copy entries back from the committed tree, whose keys were judged when stored |
| ExaBGP migration (`internal/exabgp/migration/migrate.go` and its siblings) | exempt: the migration writes a Ze config file, and every key in it is judged when that file is parsed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A long value list | refusal over 20 lines | owner answer Q-4 "print all": no cap |
| R-2 | Adding segments to `ValidateLeafValue` and `ValidateListKey` touches every caller | compile errors | intended: the compiler finds every caller |
| R-3 | A sensitive leaf leaks through the value or the suggestion | masking tests | flag set at build time; no suggestion; JSON omits `actual`; surface masking stays |
| R-5 | `ze config validate` JSON codes move from `config-parse` to the kind codes | `test/ui/*` | only `invalid syntax here` cases pin `config-parse`, and those stay |
| R-6 | `Expected` order changes to declared order | an equality assertion | every reader found asserts `Contains`; AC-11 asserts declared order by equality |
| R-8 | A rewritten pin asserts less | review | each rewrite asserts the path, the value, every accepted value and the `fix:` line; a test-weakening judge (`ze-test-judge`) rules on each |
| R-10 | Discrimination records go stale | `./le rfc check` | each phase re-records its producers in its own commit |
| R-11 | A negative pin on an old word goes vacuous (`TestMissingMandatoryOutranksAnUnplaceableToken`, `policy-test-errors.ci`, `cli-argument-*-refused.ci`) | none: it stays green | each keeps its old needle and gains one on each new marker, proven red by making the dispatcher answer with the new refusal |
| R-12 | The creator's key check turns a test that created a key the parser refuses red at the command | package and functional runs | such a key could never commit; the test is corrected to an accepted key |
| R-13 | A frame rebuilds an operator error and drops a fact | review | `WithFix` and `WithPath` copy every fact and replace only the fix or the path; `TestFrameKeepsProblemAndWhy` |
| R-15 | The tree and the session change file hold keys no key check produced: the anonymous block (`KeyDefault`, `parseList`), duplicates (`<key>#N`, `AddListEntry`) and stored meta keys. A check where an address is read, or where stored keys are read back, would hide, refuse or lock them (Current Behavior, `list_key.go` and `editor_commit.go` rows) | a lookup, delete, discard or resume test red | the one rule (`Tokens == 0` is not judged) keeps the check to addresses built from typed tokens; a lookup never runs it; `TestLookupWalkFindsEveryStoredKey` covers an anonymous entry and a `#N` entry through `ListPathStep`, `deleteFromList` and the inactive marker; `TestChangeFileReadsBackUnjudged` covers a stored `"default"` key and a session discard; `TestTypedDefaultKeyStaysRefused` keeps a typed `default` and `x#3` refused on a typed key |
| R-16 | The parser starts refusing an enum value on a `MultiLeafNode` or `BracketLeafListNode` that a config in the repository holds | `test/parse` and the repository config corpus test red | the walk and the commit already refuse such a value in walked sections; a config that holds one is converted in the same phase (`ai/META-DECISIONS.md`) |
| R-18 | `top <command>` shows a view at a position other than the one the operator asked for or the prompt names | `TestTopShowShowsTheRoot`, `TestTopSetRefreshesAtThePosition` or the AC-54 incidental-view tests red | only `cmdShow`'s asked-for views keep `configView`; the 26 current incidental sites answer `refreshConfig`, resolved at the real position, so `top set`, `top delete`, `top commit` and the rest redraw the context the prompt names; the switch is proven invisible without `top` (A-14); the view records its own path, so `top show` gets the root's annotator |
| R-20 | A fix line framed under `top` reads the emptied copy's position and prints the plain form while the operator is inside a context | `TestTopCommitFixUsesTopSetInsideAContext`, `TestTopRefusedSetFixKeepsTop` red | one framer, `FrameFix`, takes the operator position, which `cmdTop` saves apart from the resolution position (`m.operatorPosition()`, `cliCommand.Operator`) |
| R-21 | A piped line behind `top` is refused before `cmdTop` runs | `TestTopPipedShowRunsAtTheRoot` red | `dispatchCommand` takes `top` with arguments before its pipe split |
| R-19 | A verb behind `top` misbehaves: an intercepted verb (`run`, `exit`, `quit`, `stop`, `restart`, `clear`) answering `unknown command` or nothing, a nested `top top ...`, or a later step of a multi-step command (paste) landing at the real position | `TestTopInterceptedVerbsBehaveAsWithout`, `TestTopLoadTerminalPastesAtTheRoot` red | `handleEnter` drops `top ` before its interceptions; `top top set` runs `top set` at the root, which runs `set` there; the paste state records its position |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a refusal reads wrong, a path accepts or refuses a value it did not before (AC-14), or `top <command>` leaves the editor at a wrong position |
| How is it reverted? | per-phase commits, each revertable; no config, wire or storage format changes |
| Who else touches this path? | `spec-bmp-route-action-redistribute-acts-as-monitor.md`; any spec adding an enum benefits; editor navigation tests |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate -`, bad enum, hierarchical | → | `parseLeaf` → `ValidateLeafValue` → `syntaxRefusal` → `prependSyntaxPath` → `WithPath` at the parse root → `LineError` | `test/parse/enum-refusal-proposes-valid-values.ci` |
| same, enum leaf with no value | → | `parseLeaf` missing-value arm | `test/parse/enum-refusal-proposes-valid-values.ci` (missing case) |
| same, inline-list child, bracket (`ValueOrArrayNode`), inactive bracket member, `MultiLeafNode`, `BracketLeafListNode` and freeform attribute values | → | the `parser.go`, `parser_list.go` and `parser_freeform.go` value arms, each wrapped in `syntaxRefusal` | `test/parse/enum-refusal-inline-leaf-list.ci` |
| same, a refused list key in a hierarchical file | → | the `parser_list.go` list-key arms, `ValidateListKey` inside `syntaxRefusal` | `test/parse/enum-refusal-proposes-valid-values.ci` (key case) |
| same, set-format leaf, leaf-list member, `nop` line, list key, key-only line | → | `walkAndSet`, `validateValueOrArrayItems`, `walkAndSetWithMeta`, `EnsureListEntry` | `test/parse/enum-refusal-set-format.ci` |
| same, union non-member | → | `validateUnionValue` | `test/parse/enum-refusal-union.ci` |
| same, non-first key pair of a multi-leaf key | → | hierarchical `parseListFieldBlock` and leaf validation; set/editor `ParseListAddress` and `WalkListEntryPath` | `TestParserNonFirstKeyRefusalAtEntryPoint`, `TestModelNonFirstKeyRefusalReplaysPrintedFix`, `TestModelThirdKeyRefusalReplaysPrintedFix`; locally resolved constrained YANG, because shipped BFD later keys are unrestricted strings |
| daemon start, hierarchical and set format | → | loader → parser | `test/plugin/enum-refusal-daemon-start.ci` |
| reload | → | reload → parser | `test/reload/enum-refusal-reload.ci` |
| offline `ze config set`, value and key | → | `cmd_set.go` → `ValidateValueAtPath`; `SetValue` → `EnsureListEntry` | `test/parse/enum-refusal-config-set.ci` |
| SSH editor `set`, key branch, mid-path key, `rename`, `copy`, in-entry form | → | `ValidateValueAtPath`; `EnsureListEntry`; `ListEntryDestination`; the in-entry check | `test/editor/validation/enum-refusal-set.et` |
| SSH editor `insert`, nested key-leaf form, `activate`, `deactivate` under a refused key | → | `cmdInsert`, `ensureListEntry`, `runActivation` → `EnsureListEntry` | `TestKeyCheckOnInsertNestedActivation` (Go, test YANG module) |
| SSH editor session over a change file holding a stored `"default"` key: write-through `set`, `discard <path>`, commit | → | `readChangeFile` → `materializeMetaPaths` → `EnsureListEntry` with `Tokens == 0` | `TestChangeFileReadsBackUnjudged` |
| SSH editor `load` in an edit context; `load terminal` paste | → | `ParseAt`, `finishPasteMode` | `test/editor/validation/enum-refusal-load.et`, `TestEditorMultiLineErrorKeepsThePromptRow` |
| SSH editor `set cli format bogus` | → | `handleSetCLIFormat` → `ValidateLeafValue` | `test/editor/pipe/set-cli-format.et` (rewritten) |
| SSH editor `commit`, `commit confirmed 60`, `commit verify`, `show \| errors`, file and session mode, root and `edit` context | → | `validateCore` → `parseError` → `CommitRefusal`, `appendIssueSummary`, `formatIssueList`, the typed fix framed by the model | `test/editor/validation/enum-refusal-commit.et` |
| SSH editor inside a context: the printed `top set ...` fix typed, then `commit` | → | `dispatchCommand` → `cmdTop(args)` → `cmdSet` at the root | `test/editor/validation/enum-refusal-commit.et` (top case) |
| SSH editor `top set`, `top show`, `top edit`, `top` alone, `top <failing command>`, `top run <cmd>`, `top exit`, `top load terminal`, completion after `top ` | → | `handleEnter` `top ` strip; `cmdTop(args)` → `dispatchCommand`; the paste state; `commandCompleterInput` `top ` branch | `test/editor/navigation/top-command.et`, `TestTopKeepsPositionUnlessInnerMoves`, `TestTopInterceptedVerbsBehaveAsWithout`, `TestTopLoadTerminalPastesAtTheRoot`, `TestTopCompletesFromTheRoot` |
| SSH editor missing-mandatory warning inside a context | → | the typed fix on `ConfigValidationError`, framed by the model | `TestMissingMandatoryHintUsesTopInsideAContext` |
| SSH editor inside a context: `top commit`, `top set ... requre`, `top show \| errors` | → | `cmdTop` saves `m.topOrigin` → `CommitRefusal`/`cmdSet` → `FrameFix` with `m.operatorPosition()`; `top` taken before the pipe split | `TestTopCommitFixUsesTopSetInsideAContext`, `TestTopRefusedSetFixKeepsTop`, `TestTopPipedShowRunsAtTheRoot`, `test/editor/navigation/top-command.et` |
| SSH editor `set`, `delete`, `commit`, `discard` and every other incidental-view verb, no `top` | → | `refreshConfig` in `handleCommandResult`/`applyResult` | `TestIncidentalViewUnchangedWithoutTop`, `TestRemainingIncidentalViewsWithoutTop`, `TestWindowRevertedRefreshesThePosition` |
| SSH editor commit of a multi-leaf or bracket value the walk refuses | → | `validatePeer` → `formatPeerError` `ErrTypeEnum` arm | `TestFormatPeerErrorPassesTheRefusalThrough` |
| web terminal `set` (value, key, key-leaf form, in-entry form) and `commit` | → | `executeTerminalSet`; `runCommit` → `parseError` | `test/web/config-enum-refusal.wb`, `TestWebTerminalCommitShowsEnumRefusal` |
| web terminal and CLI bar `top <command>`; CLI bar commit inside a context | → | `executeTerminalConfigMode`; `dispatchCLICommand` with an empty position; `handleCLICommit` with the request's position | `TestWebTopCommandKeepsPosition`, `test/web/config-enum-refusal.wb` (top case) |
| web CLI bar `set` (Q-8b) | → | `handleCLISet` → the same checks | `test/web/config-enum-refusal.wb` (CLI bar case) |
| crafted web form POST, form ADD key, form rename, URL ancestor keys | → | form handlers → `ValidateLeafValue`; `EnsureListEntry`; `ListEntryDestination` | `TestWebFormRefusesEnumAtSet`, `test/web/config-enum-refusal.wb` |
| REST session commit; gRPC config session commit; gNMI `Set` | → | `configValidationHook` → `ValidateContent` | `test/plugin/api-config-enum-refusal.ci`, `test/plugin/grpc-config-enum-refusal.ci`, `test/plugin/gnmi-set-enum-refusal.ci` |
| REST, gRPC and gNMI set of a refused key | → | `SetValue` → `EnsureListEntry`; gNMI `applyUpdate` wraps `InvalidArgument` | the same three `.ci` (key cases, AC-43) |
| command enum, union, range, length, pattern argument, one fault and several | → | `ValidateArgs` → `ValueArgumentError` → `ArgumentFaults` → `RestatedForLine` | `test/plugin/command-arg-enum-refusal.ci`, `test/ui/cli-argument-length-refused.ci`, `test/ipsec/ipsec-dataplane-show.ci`, `test/ui/cli-argument-pattern-refused.ci` |
| `update text` and `update cursor` origin | → | `parseCommonAttributeText` | `test/plugin/update-text-origin-enum-refusal.ci` |
| range, length, pattern on every config surface | → | `ValidateLeafValue` arms | `test/parse/range-length-pattern-refusal.ci`, `test/editor/validation/range-length-pattern-set.et`, `test/web/config-range-length-pattern-refusal.wb` |
| ranged decimal64 | → | `ValidateLeafValue` decimal64 arm | `test/parse/decimal64-range.ci` |

## Acceptance Criteria

The example leaf is `bgp peer peer1 session capability route-refresh` (values
`enable`, `disable`, `require`, `refuse`, each with a `ze:help`). "The body" is
the problem line and the why lines of the Text samples. "Each surface" prints
the operator error as the surface prints one today.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | file with `route-refresh maybe;` under `peer1`, `ze config validate` | exit 1; `line N: ` then the problem `bgp peer peer1 session capability route-refresh: "maybe" is not an accepted value`, the path printed once, with spaces; the line `route-refresh accepts only these values:`; one line per value with its help, in declared order; the last line `fix: route-refresh <enable\|disable\|require\|refuse>;`; no `write:`, no `invalid enum`, no slash path |
| AC-2 | same with `requre` | AC-1 with ` (did you mean require?)` at the end of the problem line and `fix: route-refresh require;` |
| AC-3 | a value far from every accepted value (`route-refresh banana`), and BMP `route-action redistribute` (the one accepted value is `monitor` since fa74dc4c00) | no `did you mean`; every value listed; the fix line holds the alternation; for BMP, whose leaf accepts one value, the correction is the value itself: `fix: route-action monitor;`; the BMP `.ci` pins pass with the marker and the fix line added |
| AC-4 | an enum value with no `ze:help` (test schema) | its line is two spaces and the name, no trailing space; the other helps stay aligned |
| AC-5 | a member outside the enumeration on an inline-list child, a bracket-form `ValueOrArrayNode` (`storeValueOrArray`), an inactive bracket member (`parseInactiveBracketMember`), a set-format leaf-list (`validateValueOrArrayItems`), an enum-typed `MultiLeafNode` and `BracketLeafListNode`, and a freeform attribute (test schema when no shipped leaf is one) | the same refusal for that member, the full path named as in AC-1, declared order; no `(valid: `; the `MultiLeafNode` and `BracketLeafListNode` cases are refused by the parser, which accepted them before |
| AC-6 | SSH editor `set ... route-refresh requre` at the root and inside `edit bgp peer peer1`, and offline `ze config set` of the same | the AC-2 body; the fix is the command the operator typed, corrected: `fix: set bgp peer peer1 session capability route-refresh require` at the root and offline, `fix: set session capability route-refresh require` in the edit context; no `(expected enum)`. SSH: the whole refusal in the viewport (`.et` `viewport:contains`), the feedback row holds the problem line alone and the prompt stays on the last row (`TestEditorMultiLineErrorKeepsThePromptRow`). Offline: on stderr after `error: `, exit 2 |
| AC-7 | web terminal `set ... route-refresh requre`; a crafted form POST of `requre` | refused at set with the AC-6 body and the root fix; a following `show` holds no `requre`; the form's notice shows problem, why and fix (`TestWebFormRefusesEnumAtSet`) |
| AC-8 | REST session sets `requre` and commits; a second session with `requre` and `cost 0` | commit refused with HTTP 400 (`writeSessionError`); the error holds the AC-2 body and `fix: set bgp peer peer1 session capability route-refresh require`; no `line N:`; with two refusals, `config validation failed:` then each refusal as a block whose every line is indented two spaces (value lines four), each block ending with its own `fix:` line (Text samples); `errors.As` on `ValidateContent`'s error answers the first refusal's `ValidationError`; active config unchanged |
| AC-9 | gNMI `Set` of `requre`; gRPC config session set of `requre` then commit | gNMI `Aborted` (the commit hook); gRPC `InvalidArgument` (`sessionStatusError`); each message holds the AC-8 text |
| AC-10 | daemon start and reload with AC-2's file | start: `error: load config: parse config: line N: ` and the AC-2 operator error; reload: `reload: parse config: line N: ` (one `parse config:`, as `test/reload/reload-parse-error-single-prefix.ci` pins) and the same; the running config unchanged. Several walk refusals at start (any kind) are separated by a newline, never `; ` (`refuseInvalidCustomSections`, `TestRefuseInvalidCustomSectionsOneRefusalPerLine`). When the loader adds its legacy-syntax hint, the hint is written before the refusal, so `fix:` stays the last line |
| AC-11 | `ze config validate --json` on AC-2's file | code `config-yang-enum`; `expected` = `enable, disable, require, refuse` by equality; `actual` = `requre`; `line` set; `path` = `bgp.bgp/peer[peer1]/session/capability/route-refresh` by equality; `message` = the operator error without the line (`SplitLine`) |
| AC-12 | `send bgp 192.0.2.1 raw gzip DEADBEEF` on the dispatcher and on a model route; a union argument with an enum member; a line holding the bad enum and a missing argument | one fault: the problem `send bgp raw encoding: "gzip" is not an accepted value`, the why lists `hex` and `b64` with help, `fix: send bgp 192.0.2.1 raw <hex\|b64> DEADBEEF` (the suggestion in place when close), the same on both routes; union: the enum members with help, the other members named in words, no `or one of`; two faults: one `ArgumentFaults` refusal whose `- ` entry for the enum fault is its problem then its value lines indented, with no `fix:` inside the why, and whose one fix is the whole command with `<hex\|b64>` in place of `gzip` and the missing argument's usage written |
| AC-13 | a union leaf with an enum member, value no member accepts, on every parser (`ze config validate` of a `vpn ipsec` `port` and of `bgp peer P connection local ip autoo`), SSH editor `set`, offline `ze config set`, web terminal `set` | the parser refuses it (`validateUnionValue`); the why lists the enum members with help in member order, then names the other member types in words; no `matches no member type` text |
| AC-14 | every value accepted before this change, on every path | still accepted, except the named moves: the SSH editor, `insert` and offline set take the parser's verdict (valid prefixes and hex keys accepted, declared range, length and pattern misses refused); a ranged decimal64 in-range value accepted (AC-41); an enum value on a `MultiLeafNode` or `BracketLeafListNode` refused (AC-5). Timing moves: the web refuses at set; a refused new key is refused where the entry is created. Every value refused before stays refused, a typed `default` and `x#3` on a typed key included (`TestTypedDefaultKeyStaysRefused`). Every stored entry stays reachable (`TestLookupWalkFindsEveryStoredKey`) and every stored change file reads back (`TestChangeFileReadsBackUnjudged`). Every other verdict is unchanged: the scoped package tests of `config`, `config/yang`, `config/cli`, `cli`, `web`, `api`, `gnmi`, `command`, `plugin/server`, `bgp/config` and the named functional tests stay green except the pins this spec rewrites; `TestParserRefusesUnionNonMember` asserts the parser keeps refusing a union non-member |
| AC-16 | sensitive enum leaf (test schema), value close to an accepted one, on the parser and the walk | `config.SecretDataPlaceholder` in the value slot; no value, no `did you mean`; JSON omits `actual`; the accepted values still listed, on the walk too (`SectionValidationError.Message` stops writing `sensitive value redacted` for a refusal it can render masked) |
| AC-17 | tree walk refusal through `validateEntry` (Go test: `ValidateCustomSections` over a walked section, and `Validator.Validate`) | `Type` `ErrTypeEnum`, `Got` the value, `Expected` in declared order; `Error()` is the operator error, the path named once; `errors.As` recovers the `ValidationError` and the `*opererr.Error`; a missing, type, reference, cardinality or unknown `ValidationError` keeps `<type> error at <path>: <message>`; the RFC-tagged tests green unchanged |
| AC-18 | `route-refresh;` | problem `bgp peer peer1 session capability route-refresh: needs a value`, the value list, `fix: route-refresh <enable\|disable\|require\|refuse>;`; no `needs a value, one of` |
| AC-19 | SSH editor `edit bgp peer peer1`, `load` a snippet with `route-refresh requre` | the AC-2 body naming the full path, context and key included; the fix is the leaf line `fix: route-refresh require;`, the line the operator corrects in the snippet |
| AC-20 | SSH editor `set cli format bogus` | the Set frame in the viewport, path `cli format`, values in declared order (`text, table, json, yaml, ndjson`) with help, `fix: set cli format <text\|table\|json\|yaml\|ndjson>`; the feedback row in `errorStyle`; no `(valid: `; completion keeps sorted order |
| AC-21 | `errors.As` on a hierarchical and a set parser error, on `ValidateValueAtPath`'s error, on a walk error, on `ValidateContent`'s error, on a `WithFix` and a `WithPath` copy | each recovers the `ValidationError` and the `*opererr.Error`; a copy's other facts equal the original's |
| AC-22 | `update text origin bogus nlri ipv4/unicast add 10.0.0.0/24` from a plugin; the same through `update cursor` | the problem `update text origin: "bogus" is not an accepted value` (`update cursor origin` for cursor), origin values with help in declared order, a `fix:` line with the command retyped; no `bgp/peer/update/attribute/origin` |
| AC-23 | SSH editor commit, `commit confirmed 60`, `commit verify` and `show \| errors` of a candidate holding `route-refresh requre`, in file mode and in session mode, at the root and inside `edit bgp peer peer1` and inside `edit ospf`, also on an inactive (`nop`) leaf; a web terminal commit and a CLI bar commit of the same, at the root and inside a context | `CommitRefusal` keeps its header, ends it with no trailing `: `, and writes each issue on its own line; the SSH viewport shows the issues, the status row the header alone; at the root the fix is `fix: set bgp peer peer1 session capability route-refresh require`; inside any context it is `fix: top set bgp peer peer1 session capability route-refresh require`; never a file-form line or `nop`; after typing it the leaf holds `require`, the position is unchanged, and an inactive leaf stays inactive; `commit confirmed 60` refuses through `commitValidationRefusal` and writes no `cannot commit:` (`formatValidationErrors` deleted); no forced-command hint, since an error blocks. A refused member of a plain leaf-list in the candidate gets `fix: delete <path> <refused-member>`, behind `top` inside a context. The web terminal and the CLI bar (now given the request's position) answer the same lines |
| AC-24 | set-format file with `set bgp peer peer1 session capability route-refresh requre`, the `set-meta` form, and the same leaf on a `nop` line; `ze config validate` and daemon start | `line N: ` and the AC-2 body; `fix: set bgp peer peer1 session capability route-refresh require` (`fix: nop ...` on the `nop` line: only a file holds `nop`); no `invalid value for`; JSON as AC-11 |
| AC-25 | SSH editor `set <ospf-path> redistribute bogus nssa-propagate true` (OSPF `list redistribute`, key `source`), file and session mode; and a hierarchical file holding `redistribute bogus { nssa-propagate true; }` | the editor refuses where the entry would be created, no entry created: path ending `redistribute bogus`, every `source` value with help, `fix: set <ospf-path> redistribute <connected\|static\|kernel\|bgp\|isis> nssa-propagate true`; `show` holds no `redistribute bogus`. The file refuses at the list-key arm with the same body, the full path named, and the file-form fix; no `invalid key for` |
| AC-26 | SSH editor `rename` and `copy <ospf-path> redistribute connected to bogus` | refused through `ListEntryDestination`: `fix: rename <ospf-path> redistribute connected to <connected\|...>` (and `copy`); `show` unchanged |
| AC-27 | web terminal `set <ospf-path> redistribute bogus nssa-propagate true` | as AC-25, after `error: `; nothing stored |
| AC-28 | offline `ze config set <file> <ospf-path> redistribute bogus nssa-propagate true` | exit 2; stderr the AC-27 text; file unchanged |
| AC-29 | SSH editor `insert <path> <member> last` under a refused key (test YANG module in `TestKeyCheckOnInsertNestedActivation`) | refused; fix the `insert` command with the key replaced; no entry created |
| AC-30 | SSH editor `set <list-path> <bad-key> <child-list> <key-leaf> <key>` (same test module) | refused on the ancestor key; nothing created, file and session mode |
| AC-31 | SSH editor `deactivate` and `activate <list-path> <bad-key> <leaf-list> <member>` (same test module) | refused, since no entry exists under the refused key; nothing marked |
| AC-32 | every key the parser accepts, among them `10.0.0.0/8` and `2001:db8::/32` on `policy prefix-list <n> entry <prefix>`, through SSH `set` mid-path and `rename`, offline set and the web terminal; and lookups of stored anonymous and `#N` entries | accepted; each case carries a control proving the creator ran (`EnsureListEntry` created the `entry` list's entry), and the OSPF `connected`/`bogus` pair proves the creator judges the key; `TestLookupWalkFindsEveryStoredKey` finds an anonymous and a `#N` entry through `ListPathStep`, `deleteFromList` and the inactive marker |
| AC-33 | web terminal `set <ospf-path> redistribute source bogus` (key-leaf form); the same with `connected`; the in-entry `set <ospf-path> redistribute connected source static` | `bogus` refused with `fix: set <ospf-path> redistribute source <connected\|...>`, nothing stored; `connected` stores keyed entry `redistribute connected`, no anonymous entry; the in-entry line refused with the in-entry key-leaf refusal (Text samples), nothing stored, the change file still parses (`ParseChangeFile`) |
| AC-34 | SSH editor in session mode: create `redistribute connected`, then `set <ospf-path> redistribute connected source static` | the in-entry key-leaf refusal in the viewport; nothing stored; the change file still parses |
| AC-35 | range: `ospf interfaces interface eth0 cost 0` on every config surface; `cost 70000` (past the width); a declared `error-message` on a range (test schema) | the problem `<path>: "<value>" is outside the accepted range`, `cost accepts a value in 1..65535` (declared bounds for `70000` too), `fix: cost <1..65535>;` or `fix: set <path> <1..65535>`; with an `error-message`, that message is the why line; JSON `config-yang-range`, `expected` `1..65535`; no `outside range`, `outside allowed` |
| AC-36 | length: `exabgp bridge process p1 run ""` on the same surfaces; a sensitive length miss | `"" is outside the accepted length (0 characters)`, `run accepts 1..4096 characters`, fix `<1..4096 characters>`; sensitive: placeholder, no count; JSON `config-yang-length` |
| AC-37 | pattern: `l2tp shaper default-rate fast`, `service image-server rescue-auth tooshort` (inline), the OSPFv3 IPsec hex key (sensitive), and the control `ospf router-address 10.0.0.256` | `is not in the accepted format`, the format text (typedef description, or the leaf's `ze:help` inline), fix `<rate>` or `<value>`; no regular expression in the output; the address control keeps `invalid IPv4: "10.0.0.256"`; JSON `config-yang-pattern` with `expected` the format text |
| AC-38 | command argument length miss (`cli-argument-length-refused.ci`), range miss (`show vpn ipsec dataplane sa spi 0`), pattern miss (`resolve dns ptr zz`), each alone and beside a second fault | one fault: the kind's problem naming the command words and argument, the why from bounds, format reader or `error-message`, fix the whole command with the kind's placeholder in place of the value; two faults: one `ArgumentFaults` refusal as AC-12; no `out of range`, `out of allowed ranges`, `does not match expected pattern` |
| AC-39 | pattern miss on a leaf-list member, a bracket leaf-list member, a `MultiLeafNode` value; range, length, pattern miss on a new list key, at the command and in a hierarchical file (the list-key arms) | the kind's refusal for that member or key, the full path named as in AC-1; a key at creation gets the Set form with the placeholder in the key's place, a key in a file the file form |
| AC-40 | a pattern typedef with no `ze:help` and no description; an inline pattern on a leaf with no `ze:help` (test schema) | `<leaf> accepts a value of type <typedef-name>`; the inline case's why is `<leaf> does not accept this value`; no regular expression |
| AC-41 | the five ranged `zt:decimal-2` leaves given an in-range value (`5.00`, `5.00`, `2.50`, `5.00`, `0.25`) and an out-of-range one (`500.00`, `500.00`, `16.01`, `0.99`, `1.01`), through `ze config validate` (text, JSON), SSH `set`, offline set; `5.001` and `abc` | in range accepted; out of range the range refusal with the decimal bounds (`1.00..100.00`, `0.00..16.00`, `0.00..1.00`); `5.001` and `abc` keep the type refusals `expected decimal64 with at most 2 fraction digits` and `expected decimal64` |
| AC-42 | a multi-leaf-key list whose non-first key carries a constraint, through hierarchical parsing and SSH editor `set`; the instance-local YANG test uses BFD's `peer vrf interface` shape with constrained later keys | the kind's refusal for that key leaf, the complete consumed address retained in the path; no `invalid <leaf> for <list>:` prefix; the printed correction preserves later address and payload words and replays to the exact composite identity. Shipped BFD later keys are unrestricted strings, so the typed refusal is proved with locally resolved YANG at the actual entry points |
| AC-43 | REST, gRPC and gNMI session set of `<ospf-path> redistribute bogus nssa-propagate true` | refused at set, nothing stored: REST HTTP 400 (`writeSessionError`), gRPC `InvalidArgument` (`sessionStatusError`), gNMI `InvalidArgument` (today `Unknown`: `applyUpdate` returns the `SetSegments` error raw; this spec wraps it as `InvalidArgument`, a defect fix); each message holds the problem, the value lines and `fix: set <ospf-path> redistribute <connected\|...> nssa-propagate true` |
| AC-45 | SSH editor inside `edit bgp peer peer1`: `top set ospf router-id 192.0.2.9` | the leaf `ospf router-id` holds `192.0.2.9`; the prompt and the position are still `bgp peer peer1`; a refreshed view shows the config at `bgp peer peer1`; history holds the line `top set ospf router-id 192.0.2.9` as typed |
| AC-46 | SSH editor inside `edit bgp peer peer1`: `top show` | the viewport shows the whole configuration from the root; afterwards the prompt and the position are still `bgp peer peer1` |
| AC-47 | SSH editor inside `edit bgp peer peer1`: `top edit ospf`; then `top` alone; then `top top edit bgp`; then `top run show version`; then, with no pending change, `top exit` | `top edit ospf` moves the position to `ospf`; `top` alone moves to the root as today; `top top edit bgp` moves to `bgp`; `top run show version` answers what `run show version` answers and leaves the position; `top exit` leaves config mode exactly as `exit` does |
| AC-48 | SSH editor inside `edit bgp peer peer1`: `top set nosuchleaf 1`, `top set bgp peer peer1 session capability route-refresh requre`, `top nosuchverb`; and `top exit` with a pending change | each refused with the inner command's own refusal (the AC-2 body for the second, with its fix framed from the operator's position as typed: `fix: top set bgp peer peer1 session capability route-refresh require`; `unknown command` for the third); the position stays `bgp peer peer1`; nothing stored; `top exit` with a pending change asks the same confirmation `exit` asks |
| AC-49 | SSH editor inside `edit bgp peer peer1`: completion and `?` after `top ` and after `top set ` | the rest of the line is completed with `m.completer.Complete(tree, rest, nil)` at the root context and the editor's own verbs: the candidates are what the root offers at the same point (`set`, `edit`, `show`, `run` and the rest after `top `; the root's top-level nodes after `top set `), never the context's children; `?` after `top` shows the `top <command>` form from the help table |
| AC-50 | web terminal and CLI bar, position `bgp peer peer1`: `top set ospf router-id 192.0.2.9`, `top show`, `top edit ospf`, `top` alone, `top set nosuchleaf 1` | each answers as on the SSH editor (AC-45 to AC-48): the answered position stays `bgp peer peer1` except after `top edit ospf` (`ospf`) and `top` alone (root); the failing line answers the inner refusal; the CLI bar no longer drops the rest of a `top ...` line |
| AC-51 | SSH editor commit inside `edit ospf` of a candidate whose `bgp peer peer1` lacks a mandatory leaf; the same at the root; the same on the web terminal and the CLI bar | inside the context the missing-mandatory hint reads `(top set bgp peer peer1 <leaf> <value>)`; at the root `(set bgp peer peer1 <leaf> <value>)` as today; the hint is framed when it is rendered, so a hint computed before an `edit` reads correctly after it; the `discard ...` conflict fixes are unchanged |
| AC-52 | SSH editor inside `edit bgp peer peer1`: `top load terminal relative`, paste `ospf { router-id 192.0.2.9; }`, Ctrl-D | the paste is applied at the root: `ospf router-id` holds `192.0.2.9` and nothing is written under `bgp peer peer1`; the position stays `bgp peer peer1` |
| AC-53 | SSH editor inside `edit ospf`, a candidate holding `bgp peer peer1 session capability route-refresh requre`: `top commit`; `top show \| errors`; the web terminal and the CLI bar at position `ospf`: `top commit` | `top commit` is refused with `fix: top set bgp peer peer1 session capability route-refresh require` (the operator's position, not the copy's root); `top show \| errors` lists the same issue with the same fix and is not refused as unpipeable; the web answers the same line; the position stays `ospf` |
| AC-54 | no `top`: at the root and inside `edit bgp peer peer1`, each of the 26 incidental refresh sites in Data Flow 7, including successful commits, reload refusals, structural edits, discard, rollback, session visibility, confirmation and restored-window events; valid `commit verify` is checked separately as a status-only answer, and issue reports keep Q-7 (a)'s viewport | the viewport content, its diff gutter, its validation marks, the scroll position and the status row equal what the copy-built `configView` drew before the switch; history holds every typed line exactly, and a line typed behind `top` is recorded with `top` (AC-45) |

AC-15 and AC-44 are unused: AC-15 was a removed-value reason, which direction
(c) leaves out; AC-44 was a resume-time key refusal, withdrawn in round 20
because stored keys are read back unjudged.

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | typos an enum in a file and validates it | file → parser → `ValidateLeafValue` | `test/parse/enum-refusal-proposes-valid-values.ci` |
| 2 | types a bad enum in the SSH editor | `set` → `ValidateValueAtPath` | `test/editor/validation/enum-refusal-set.et` |
| 3 | types a bad value in the web terminal | `executeTerminalSet` → `ValidateLeafValue` | `test/web/config-enum-refusal.wb` |
| 4 | pushes config through REST, gRPC or gNMI | session → commit → `ValidateContent` | `test/plugin/api-config-enum-refusal.ci`, `test/plugin/grpc-config-enum-refusal.ci`, `test/plugin/gnmi-set-enum-refusal.ci` |
| 5 | reloads a file with a bad enum | reload → parser | `test/reload/enum-refusal-reload.ci` |
| 6 | mistypes a command argument | dispatcher → `ValidateArgs` → `RestatedForLine` | `test/plugin/command-arg-enum-refusal.ci` |
| 7 | loads a snippet with a bad enum | `load` → `ParseAt` | `test/editor/validation/enum-refusal-load.et` |
| 8 | writes an out-of-range, wrong-length or malformed value | parsers, `set` surfaces | `test/parse/range-length-pattern-refusal.ci`, `test/editor/validation/range-length-pattern-set.et`, `test/web/config-range-length-pattern-refusal.wb` |
| 9 | inside a context, copies a commit refusal's `top set ...` fix and commits | `cmdTop(args)` → `cmdSet` at the root → commit | `test/editor/validation/enum-refusal-commit.et` (top case) |
| 10 | changes, shows or pastes config elsewhere without leaving the current context | `top <command>` | `test/editor/navigation/top-command.et` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestValueListRender` | `internal/core/suggest/valuelist_test.go` | alignment, no-help line, alternation (AC-4) | |
| `TestEnumRefusalSuggestion` | `internal/component/config/yang/refusal_test.go` | A-1 cases | |
| `TestValidationErrorOperatorError` | same | problem, why, fix of each kind; secret flag; constraint message as why; the other kinds keep today's text (AC-16, AC-17, AC-35 to AC-37) | |
| `TestFrameKeepsProblemAndWhy` | same | `WithFix` replaces the fix only, `WithPath` the path only; `errors.As` finds the `ValidationError` and the `*opererr.Error` through `LineError` and through each copy; `Error()` prints problem and why once (R-13, AC-21) | |
| `TestTypeFormatReader` | `internal/component/config/yang/format_test.go` | typedef help, description first sentence, leaf help, type name (AC-37, AC-40) | |
| `TestYangToLeafCarriesEnumHelp`, `TestValueOrArrayEnumDeclaredOrder`, `TestMultiLeafAndBracketCarryEnums` | `internal/component/config/yang_schema_test.go` | help, declared order, enum facts on every leaf-list node | |
| `TestParserEnumRefusalNamesFullPath`, `TestParserRefusalIsTyped`, `TestParseAtEnumRefusalNamesContextPath`, `TestParserMissingEnumValueRefusal`, `TestEveryValueArmNamesTheFullPath` | `internal/component/config/value_refusal_test.go` (833c08af3f; the spec named `parser_test.go`) | AC-1, AC-2, AC-5, AC-18, AC-19, AC-21, AC-25, AC-39; the path once, space-separated, keys included, no slash prefix, on `parseLeaf`, on `parseInactiveBracketMember`, on the two list-key arms and on each `parser_list.go` and `parser_freeform.go` value arm | committed |
| `TestParserRefusesUnionNonMember` | same | AC-13, AC-14 | committed |
| `TestSetParserEnumRefusalFrame`, `TestSetParserLeafListEnumRefusal`, `TestSetLineKeyOnStoredEntryStaysRefused`, `TestInactiveLineReachesStoredDuplicateEntry` | same (the spec named `setparser_test.go`) | AC-24 incl. `nop`, set-meta and the key-only line; AC-5 set-format leaf-list; AC-14 the set parser's own key check on a stored entry (`lineKeyRefusal`) and the `inactive` line on a stored duplicate entry | committed |
| `TestInlineLeafListEnumRefusal` | same (the spec named `parser_list_test.go`) | AC-5 | committed |
| `TestEnsureListEntryJudgesNewKey`, `TestTypedDefaultKeyStaysRefused`, `TestParseListAddressPairRefusal`, `TestLookupWalkFindsEveryStoredKey`, `TestChangeFileReadsBackUnjudged` | same (the spec named `list_key_test.go`, `change_file_test.go`) | AC-14, AC-25 to AC-32, AC-42, R-15 | committed |
| `TestValidateLeafValueRangeLengthPatternRefusal`, `TestValidateLeafValueDecimal64Range` | same (the spec named `schema_test.go`) | AC-35 to AC-37, AC-41 | committed |
| `TestValidateEnumerationSharedBody` | `internal/component/config/yang/validator_test.go` | AC-17's direct `Validator.Validate` comparison; `TestValidationErrorOperatorError` above covers its sensitive walk and typed operator facts | committed; earlier full YANG package run passed |
| `TestSectionValidationErrorRendersRefusal`, `TestValidateCustomSectionsEnumRefusal` | `internal/component/config/section_refusal_walk_test.go` | AC-16/AC-17: actual `ValidateCustomSections` enum refusal retains typed facts/chain; a local sensitive YANG walk's `Message` retains the masked value list without the secret, suggestion or legacy redaction summary | both named tests passed the focused race run; the full config race run including them also passed |
| `TestRefuseInvalidCustomSectionsOneRefusalPerLine`, `TestSectionRefusalsJoinOnePerBlock` | `internal/component/config/value_refusal_test.go` | AC-10: walk refusals one per line, never `; `; the join puts each refusal on its own block ending with its own `fix:` line | committed |
| `TestLegacyHintPrecedesTheRefusal` | `internal/component/config/value_refusal_test.go` (the spec named `loader_test.go`) | AC-10 hint placement | committed |
| `TestRunValidationParserEnumDiagnostic`, `TestValidateContentFramesEnumRefusal` | `internal/component/config/cli/cmd_validate_refusal_test.go` | AC-8, AC-11, AC-21 | committed; earlier phase proofs retained |
| `TestCmdSetValueRefusal`, `TestCmdSetKeyRefusalKeepsTheWholeCommand`, `TestCmdSetAcceptsPrefixKeys`, `TestCmdSetKeyRefusalMasksSensitiveSuffix` | `internal/component/config/cli/cmd_set_refusal_test.go` | AC-6, AC-13, AC-28, AC-32: exact enum/union/key refusal, retained suffix, accepted IPv4/IPv6 entry controls, masked sensitive suffix and unchanged file bytes | focused pass; committed in 1ce3fb78c2 |
| `TestParseDiagnosticsCollectsScalarStatements`, `TestParseDiagnosticsStopsAtStructuralErrors`, `TestParseDiagnosticsCollectsMissingValue`, `TestParseDiagnosticsCollectsInactiveScalars`, `TestParseSetDiagnosticsCollectsLines`, `TestParseSetDiagnosticsStopsAtStructuralErrors` | `internal/component/config/parser_validation_test.go` | AC-8, AC-21: collect complete typed value failures, preserve their paths, return no usable invalid tree, retain strict normal parsing and stop on unsynchronized or structural input | focused selector and full config package passed |
| `TestValidateContentTwoValueRefusals` | `internal/component/config/cli/cmd_validate_refusals_test.go` | AC-8, AC-21: both hierarchical and set candidates return two fully indented blocks, each ending in its own root set fix; typed chain recovers the first refusal | focused pass and REST load proof |
| `TestCandidateChangeFilePreservesInvalidValues`, `TestCandidateChangeFileRefusesCorruption`, `TestCandidateChangeFileRejectsBrokenBrackets`, `TestCandidateChangeFileKeepsQuotedBracketMembers`, `TestCandidateChangeFileKeepsLegacyBracketMembers` | `internal/component/config/change_file_candidate_test.go` | AC-23 repair prerequisite: complete stored leaf payloads survive with ownership while ordinary parsing stays strict; malformed path, key, arity, secret, quote, verb and raw delimiter controls still refuse | focused candidate selector and full config package passed |
| `TestSessionCandidateValueCanBeRepairedAfterRefusal`, `TestSessionCandidateMemberCanBeDeletedAfterRefusal`, `TestBareSessionCommitKeepsRefusedCandidateEditable` | `internal/component/cli/editor_candidate_repair_test.go` | AC-23: refused API-created candidates remain editable in the same or reopened session, printed member deletion works, unrelated edits survive and bare commit routes neither publish nor consume invalid state | focused session/commit/corruption selector passed; functional repair workflow below |
| `TestAPISetFixtureReachesCommit`, `TestAPISetFixtureRefusesBadDeclarations`, `TestAPISetFixtureKeepsTokenAndValueBoundaries`, `TestAPISetFixtureNeedsAnEditor` | `internal/component/cli/testing/api_set_fixture_test.go` | the ordered fixture invokes the real API writer, keeps path/value boundaries and refuses invalid declarations or operational-only use | focused fixture and promoted editor selection passed. The broader harness run's unchanged backup timeout and its isolated/live passes remain disclosed below |
| `TestRequiredViolationExpandsCompositeEntry`, `TestRequiredViolationExpandsCompositeAncestor`, `TestRequiredViolationQuotesSingleKey`; `TestPrintedRequiredFixPreservesCompositeEntry`, `TestPrintedRequiredFixPreservesCompositeAncestor`, `TestPrintedRequiredFixPreservesSingleKey` | `internal/component/config/required_path_words_test.go`, `internal/component/cli/required_fix_replay_test.go` | AC-51: required paths expand each composite address into its canonical words, retain spaced single keys, and the actual printed correction executes at root and in a context without changing the entry identity | independent review exposed both composite boundaries; actual RED then focused config/model GREEN |
| `TestRequiredViolationQuotesLiteralPipe`, `TestPrintedRequiredSetHintPreservesLiteralPipe` | same | standalone required hints preserve a literal pipe key through the command grammar, as editor fixes do | actual RED then focused config/model GREEN after command-word quoting |
| `TestRequiredViolationPreservesNumericSuffix`, `TestPrintedRequiredSetHintPreservesNumericSuffix` | same | AC-51: required entry and ancestor paths retain literal `edge#3` and composite final-key `beta#3`; standalone hints update the original identity without creating another entry | eight config/raw-hint cases observed RED before the private identity-reader repair, then focused config/model GREEN; independent round 3 found no BLOCKER or ISSUE |
| `TestModelPipeLiteralArgument`, `TestWebTerminalPipeLiteralArgument`, `TestCommandWordsOwnership`, `TestCommandWordsZeroContract` | `internal/component/cli/model_pipe_literal_test.go`, `command_words_ownership_test.go`, `internal/component/web/cli_pipe_literal_test.go` | AC-14, AC-45 to AC-50: quoted pipe arguments remain data after every operator, including chained filters and nested `top`; immutable parsed words retain bounded provenance and safe empty behavior | actual CLI/web RED then focused GREEN; three existing test API conversions independently approved and recorded |
| `TestModelUnfinishedQuoteRefusesWithoutChangingCandidate`, `TestWebTerminalUnfinishedQuoteRefusesWithoutChangingCandidate` | the new model/web pipe test files | malformed quoted secret assignments refuse before mutation, return the operator error without echoing the input, and permit a completed correction | focused model/web controls passed for both quote forms |
| `TestConfirmedSessionValidatesBeforeDaemonRequirement`, `TestConfirmedDaemonSessionFramesEnumRefusal` | `internal/component/cli/model_confirmed_validation_order_test.go` | AC-23: offline session validation precedes the no-daemon guard; real daemon-window sessions retain the full contextual refusal, pending candidate and unchanged active config, then accept a corrected candidate | focused confirmed/session selector passed; existing history/backend guards retained |
| `TestEditorSetValueFramesAncestorKeyRefusal`, `TestEditorKeyRefusalMasksTheFollowingSecret` | `internal/component/cli/editor_set_key_refusal_test.go` | AC-43: direct API callers receive the whole root set correction, typed refusal and unchanged candidate; a sensitive suffix is masked | focused pass; all three transport load cases passed |
| `TestPrintedEnumFixPreservesListKey`, `TestFrameFixPreservesLiteralTokens`, `TestLengthFixLeavesTheCorrectionPlaceholderRaw`, `TestPrintedKeyFixPreservesFollowingValue`, `TestWebPrintedEnumFixPreservesListKey` | `internal/component/cli/fix_tokens_refusal_test.go`, `internal/component/web/cli_fix_tokens_test.go` | exact fixes preserve quoted list keys, following values, literal angle-bracket keys, literal pipes, numeric suffixes and explicit correction placeholders; actual printed commands replay through model/web dispatch | actual failures observed before repair; focused CLI/config/web replay controls passed, including `edge#3` and `edge#3#4`; earlier full CLI passed; final browser baseline, completed 80-run campaign and all three promoted live cases passed |
| `TestHierarchicalRefusalReplaysLiteralEnumCorrection`, `TestSetRefusalReplaysLiteralEnumCorrection`, `TestDefaultRefusalReplaysLiteralEnumCorrection` | `internal/component/config/refusal_literal_fix_test.go` | hierarchical, set-file and default YANG fixes preserve literal enum corrections through the same shared quoter | actual red before repair; focused controls and full config package passed |
| `TestModelKeyRefusalMasksFollowingSecret`, `TestWebTypedKeyRefusalMasksTheFollowingSecret` | `internal/component/cli/model_key_secret_refusal_test.go`, `internal/component/web/cli_typed_key_refusal_test.go` | root/context/top fixes mask spaced, quoted and backslash secrets before rendering; web keeps both typed error chains | actual secret/type failures observed before repair; focused controls passed; full CLI passed |
| `TestParserNonFirstKeyRefusalAtEntryPoint`, `TestModelNonFirstKeyRefusalReplaysPrintedFix`, `TestModelThirdKeyRefusalReplaysPrintedFix` | `internal/component/config/parser_multikey_refusal_entry_test.go`, `internal/component/cli/model_multikey_refusal_entry_test.go` | AC-42: actual hierarchical/context/set parsing and file/session root/context editor dispatch over locally resolved constrained YANG; full typed paths, retained candidate/pending state and printed-fix replay to exact composite identities | parser controls passed; actual editor failures were observed before repair, then the focused boundary race batch passed. The nonexistent functional-file claim is replaced transparently under the independent test judgment |
| `TestParserRefusalQuotesAcceptedPathWords`, `TestModelRefusalQuotesAcceptedPathWords`, `TestModelKeyLeafRefusalQuotesAcceptedPathWords`, `TestModelRequiredWarningQuotesAcceptedPathWords` | the two entry-point files above and `internal/component/cli/model_problem_path_entry_test.go` | accepted newline, carriage return, tab and quote bytes remain identity data while problems stay on one line; actual enum, structural key-leaf and required-warning corrections replay against the original entry | actual display failures preceded the source repairs; current focused race controls passed. The local required case calls public `ValidateWithYANG` and replays its actual warning, without claiming a custom-module commit-verify run |
| `TestModelNonFirstKeyRefusalMasksFollowingSecret` | `internal/component/cli/model_multikey_secret_refusal_test.go` | AC-16/AC-42: the earliest refused non-first-key correction retains the whole line while masking its sensitive suffix, in both modes and positions | all eight cases failed before masking and passed after it; the sixth independent source review is CLEAN |
| `TestOperationalEnumFixReplaysLiteralValue`, `TestOperationalEnumFixKeepsMissingArgument` | `internal/component/plugin/server/enum_fix_replay_test.go` | AC-12: literal concrete enum corrections traverse the normal checked pipeline and actual dispatcher without changing argument bytes or turning data into a pipe; missing placeholders remain syntax | actual failures preceded repair; current command/server/CLI/MCP boundary race controls passed. Existing quote-refusal and tokenizer expectations were independently judged against the deliberate quoted-escape contract |
| `TestBrowserClosePreservesParallelBudget`, `TestBrowserCloseParallelHangStaysBounded` | `internal/component/web/testing/close_budget_policy_test.go` | cleanup retains the existing parallel command bound when it clears cancellation, and a hung owned browser remains bounded | both policy controls failed before repair; full web/testing race, real native observer controls, the fresh completed browser 80-run campaign and all three promoted live cases passed. Round 6 source review is CLEAN |
| `TestWebPrintedDeleteFixKeepsAcceptedMembers` | `internal/component/web/cli_delete_fix_test.go` | replay the commit's actual plain/bracket leaf-list deletion through terminal/bar at root/context, preserving accepted members and unrelated settings, then commit and read the stored format | actual subtree deletion and terminal feedback failures observed; all eight replay/state/commit controls passed after the existing deletion seam received all tokens |
| `TestReportDistinguishesOperatorTextFromMarkup` | `internal/test/markupcheck/markupcheck_test.go` | canonical direct operator-text arguments remain text; adjacent, nested, foreign, shadowed and malformed-source markup remains detected, with literal counts retained | actual red before helper repair; full helper and web/LG markup guards passed; independent judge approved the exact exception |
| `TestPublishDaemonPIDReplacesIdentity`, `TestPublishDaemonPIDModeOwnership`, `TestPublishDaemonPIDRenameFailure`, `TestPublishDaemonPIDOwnershipFailure`, `TestPublishDaemonPIDCreateFailure`, `TestPublishDaemonPIDRunnerFailure` | `internal/test/runner/daemon_pid_test.go` | publish a complete owned identity atomically, preserve old readers on replacement/failure, remove staging files, and make both daemon launch arms fail and reap their child when publication fails | publisher and both caller failures observed red; focused controls, full runner package and fresh 80-invocation reload proof passed |
| `TestWebHandlerGoldenOutput/get-l2tp-session-json` | `internal/component/web/testdata/handler/get-l2tp-session-json.txt` | existing exact snapshot retains the committed empty `ProxyLCP` field | independently approved additive fixture correction passed; the two operational-root navigation goldens remain separately owned failures |
| `TestValidateValueAtPathEnumRefusal`, `TestValidateValueAtPathRangePatternRefusal` | `internal/component/cli/model_refusal_frame_test.go` | AC-6; AC-14 numeric range/type controls | final full CLI race run passed |
| `TestValidateValueAtPathKeepsParserVerdicts`, `TestModelPrefixMembersKeepParserVerdicts`, `TestModelHexKeyKeepsParserVerdicts` | `internal/component/cli/acceptance_verdict_test.go` | AC-14: direct IPv4/IPv6 prefix and hex acceptance, declared range, length and pattern boundaries; actual set/insert preserve refused candidates and retain accepted prefix order and hex spelling in file/session modes | focused race run passed every case (`enum-acceptance-verdict-green-20261113.log`). Direct test loads actual shipped bridge and L2TP shaper modules into its local loader; no shared registration is changed |
| `TestEditorMultiLineErrorKeepsThePromptRow`, `TestCommitRefusalWritesOneIssuePerLine`, `TestEditorFixIsTheCommandTypedHere`, `TestCommitFixUsesTopInsideAContext`, `TestMissingMandatoryHintUsesTopInsideAContext`, `TestCliFormatRefusalUsesSharedBody`, `TestInEntryKeyLeafRefusal`, `TestFormatPeerErrorPassesTheRefusalThrough`, `TestKeyCheckOnInsertNestedActivation` | `internal/component/cli/*_test.go` | AC-6, AC-20, AC-23, AC-29 to AC-31, AC-34, AC-51 | |
| `TestTopKeepsPositionUnlessInnerMoves`, `TestTopShowShowsTheRoot`, `TestTopSetRefreshesAtThePosition`, `TestTopFailingCommandKeepsPosition`, `TestTopInterceptedVerbsBehaveAsWithout`, `TestTopLoadTerminalPastesAtTheRoot`, `TestTopCompletesFromTheRoot`, `TestTopCommitFixUsesTopSetInsideAContext`, `TestTopRefusedSetFixKeepsTop`, `TestTopPipedShowRunsAtTheRoot`, `TestTopKeepsHistoryAsTyped` | `internal/component/cli/model_top_test.go` | AC-45 to AC-49, AC-52, AC-53, A-11, A-13, R-18 to R-21; `TestTopShowShowsTheRoot` also asserts the tree-diff annotator; `TestTopKeepsHistoryAsTyped` covers `top set`, `top run`, `top clear` and an unrecorded `top exit` | |
| `TestIncidentalViewUnchangedWithoutTop` | `internal/component/cli/model_view_refresh_test.go` | AC-54, A-14: the existing eight typed cases at root and peer context cover `set`, `delete`, `deactivate`, `activate`, `discard all`, `option errors hints`, `option errors hide`, `load file` (`applyLoad`). They reach seven distinct refresh return sites because activation shares one producer; additive assertions cover exact history and feedback too | focused pass; original assertions retained |
| `TestRemainingIncidentalViewsWithoutTop`, `TestWindowRevertedRefreshesThePosition`, `TestValidCommitVerifyKeepsItsViewport` | `internal/component/cli/model_view_refresh_remaining_test.go` | AC-54, A-14: twenty typed scenarios at root and peer context cover eighteen further return sites; rename/copy and session commit modes deliberately share producers. Window restoration covers the final site through both appliers over restored store content. Checks viewport with marks and gutter, scroll, feedback, exact history and actual stored entries. Valid verification preserves its viewport and applies nothing | focused and full CLI runs passed; both native wrong-position walks discriminated their overlay with the intended view assertions |
| `TestShowPathAnnotatesAtItsOwnPath` | `internal/component/cli/model_view_refresh_test.go` | `show bgp` at the root is annotated with the line gutter of its own path, `show` at the root with the tree diff, `top show` inside a context with the tree diff (the recorded `viewportData` path) | |
| `TestFrameFix` | `internal/component/cli/fix_frame_test.go` | the three arms of `FrameFix` (equal position, root operator, other), A-15 | |
| `TestWebFormRefusesEnumAtSet`, `TestWebTerminalCommitShowsEnumRefusal`, `TestWebTerminalKeyLeafFormStoresKeyedEntry`, `TestWebTopCommandKeepsPosition`, `TestCLIBarCommitFramesTheFixWithThePosition` | `internal/component/web/*_test.go` | AC-7, AC-23, AC-33, AC-50, AC-51 | |
| `TestSetUpdateRefusalIsInvalidArgument` | `internal/component/gnmi/set_test.go` | AC-43 gNMI code | |
| `TestValidateArgEnumRefusal`, `TestValidateArgRangeLengthPatternRefusal`, `TestOneValueFaultIsRestated`, `TestFaultEntryHoldsNoFixLine` | `internal/component/command/*_test.go` | AC-12, AC-38 | |
| `TestDispatcherRestatesOneValueFault` | `internal/component/plugin/server/route_arg_faults_test.go` | AC-12 dispatcher route | |
| `TestUpdateTextOriginEnumRefusal` | `internal/component/bgp/plugins/cmd/update/update_text_origin_refusal_test.go` | AC-22 | full update package passed |
| `TestConfigValidationHookEnumRefusal` | `cmd/ze/hub/config_validation_refusal_test.go` | AC-8 at the hook | full hub package passed; native hook walk passed clean and failed on the missing refusal and typed chain under the break |

### Boundary Tests (numeric inputs)
| Field | Range | Last valid | First invalid below | First invalid above |
|-------|-------|------------|---------------------|---------------------|
| `cost` | 1..65535 | 65535 | 0 | 65536 |
| `threshold-multiplier` | 1.00..100.00 | 100.00 | 0.99 | 100.01 |
| `run` length | 1..4096 | 4096 characters | 0 | 4097 |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `enum-refusal-proposes-valid-values` | `test/parse/` | AC-1, AC-2, AC-3, AC-18, AC-25 file case | committed; earlier phase proofs retained |
| `enum-refusal-inline-leaf-list`, `enum-refusal-set-format`, `enum-refusal-union`, `range-length-pattern-refusal`, `decimal64-range` | `test/parse/` | AC-5, AC-13, AC-24, AC-35 to AC-37, AC-39, AC-41 | committed; earlier phase proofs retained. The previously listed `enum-refusal-multi-leaf-key.ci` never existed; AC-42's independently judged actual entry-point replacement is recorded above |
| `enum-refusal-config-set` | `test/parse/` | AC-6, AC-13, AC-28, AC-32: offline refusal, unchanged state and both prefix-key controls | standalone, all 80 load invocations and ordinary live projection passed; cost report read and promoted |
| `enum-refusal-daemon-start` | `test/plugin/` | AC-10, AC-24 | committed; earlier phase proofs retained |
| `enum-refusal-reload` | `test/reload/` | AC-10; unchanged invalid input declares the actual `config-yang-enum` diagnostic | fresh baseline and all 80 load invocations passed after atomic PID publication; cost report read, promoted and native independent approval recorded; ordinary live reload and the single-prefix regression passed |
| `enum-refusal-set`, `set-cli-format` | `test/editor/validation/`, `test/editor/pipe/` | AC-6, AC-20, AC-25, AC-26, AC-34 | committed; continuation ran the combined selection for 80 invocations with no failure |
| `enum-refusal-load`, `enum-refusal-commit`, `range-length-pattern-set` | `test/editor/validation/` | AC-19, AC-23, AC-35 to AC-37: root/context and file/session refusal, printed `top set` correction, successful commit and inactive preservation | standalone cases, all 80 load invocations and ordinary promoted live selection passed; cost report read |
| `commit-blocked-missing-leak-filter` | `test/editor/lifecycle/` | retains both original peer-pipeline reason needles in the viewport and checks the full commit-count header in status, with dirty-candidate assertions intact | independently judged correction passed the same 80-invocation proof and ordinary promoted live selection; native approval recorded |
| `top-command` | `test/editor/navigation/` | AC-45 to AC-49, AC-52 | committed; earlier phase proof retained |
| `config-enum-refusal`, `config-range-length-pattern-refusal` | `test/web/` (strengthened drafts promoted) | AC-7, AC-27, AC-33, AC-50, Q-8a, Q-8b; exact quoted correction replay and retained-value readback, including empty length refusal | original assertions retained; final baseline, every invocation of the fresh 80-run campaign, cost review and ordinary promoted live cases passed. Fresh form readback uses the documented canonical `/show/` route after independent judgment |
| `literal-pipe-filter` | `test/editor/pipe/` | actual keystroke quoted-pipe filtering, later bare operators and root/context/nested-top positions | baseline, all 80 load invocations and ordinary promoted live selection passed; cost report read |
| `config-literal-pipe-filter` | `test/web/` (promoted from draft) | actual browser quoted-pipe filtering, later bare operators and root/context/nested-top positions | final native baseline, all 80 load invocations, cost review, promotion and ordinary live projection passed, with authored assertions and budgets unchanged |
| `api-config-enum-refusal`, `grpc-config-enum-refusal`, `gnmi-set-enum-refusal` | `test/plugin/` (promoted from drafts) | AC-8, AC-9, AC-43: leased REST endpoint; real TLS/token gRPC and gNMI; full key fixes, commit refusals and unchanged running config; REST also returns both invalid values | all three passed each of 80 load invocations and their promoted clean runs; native gRPC/gNMI hook walks passed clean and failed with the expected missing-refusal observer message under the break |
| `command-arg-enum-refusal`, `update-text-origin-enum-refusal` | `test/plugin/` | AC-12, AC-22 | committed; earlier Phase 5 proofs retained |
| `cli-argument-pattern-refused` | `test/ui/` | AC-38 | committed; earlier Phase 5 proof retained |
| `ipsec-dataplane-show` | `test/ipsec/` | AC-38 command-range pin, retaining the dataplane reachability controls | committed pin; all 80 load invocations passed |
| Rewritten pins in Current Behavior, each retaining its original positive/negative question and the independent test-judge record required for the changed unit | as listed | text/viewport changes only where this spec changes the producer | earlier phase judgments retained; final changed-pin inventory and all new changes still checked before closure |

### Current implementation evidence

This is implementation bookkeeping, not a closure report. Phase is `6/6`;
status is `blocked` on the explicit decision permitting a sixth independent
source review. Runtime proofs and final documentation/RFC checks are recorded.

| Piece | Observed evidence | Still owed |
|-------|-------------------|------------|
| Offline writer | 1ce3fb78c2 carries `cmdSetImpl`, its exact refusal/state/secret tests and guide paragraph; focused `TestCmdSet...` plus existing secret tests passed; `repo compiles check` passed all five flavors of that HEAD. Later masking and literal replay controls passed. The real parse workflow passed its baseline, all 80 load invocations and ordinary promoted live projection | final config/CLI corpus retains the foreign invalid LDP fixture failure disclosed below |
| API refusals | `./le --name session test stress-repro run suite 'plugin --draft --pattern enum-refusal' iterations 80 parallel 2 burners 2 timeout 120 any-failure` completed 80 zero-exit invocations selecting the three named API drafts; all are promoted and passed their live clean projections. Full API, REST, gRPC, gNMI, CLI and hub package runs passed. Both native transport/hook overlays failed on the intended missing-refusal assertions | no remaining API proof |
| AC-54 | `TestRemainingIncidentalViewsWithoutTop` passed after actual-entry assertions replaced incorrect serializer assumptions; the original eight cases, window restoration through both appliers and valid verification passed their focused controls. Full CLI package passed. Both native wrong-position overlays passed clean and failed their intended view assertions under the break | no remaining refresh proof |
| Candidate repair | `TestCandidateChangeFile...` and the combined session repair, member deletion, bare commit, confirmed-session and corrupt-file safeguards passed. Full `config` and editor `cli` packages passed. All four editor drafts passed each of 80 invocations under two parallel runs and two burners, cost report was read, and all four are promoted. The independent lifecycle approval is recorded. The six-case promoted editor selection, including unchanged backup, passed | the broader harness run's original backup timeout remains disclosed below; no assertion or deadline changed |
| RFC evidence | 024ba1d203 re-observes all thirteen records in the seven RFC 7950 proof files; 0f8c7ccf04 stamps independent rejudgment of both owned audit rows. Six additional producer observations and the final RFC 8907 audit reseal are recorded. The lexical row is honestly `weak` | final owned RFC 7950/8907 freshness observed in `job-enum-final-closure-rfc-freshness-4fea72ad.log`; its global 99 violations concern other stems. No full RFC support claim |
| Exact fix tokens | CLI, config and web replay tests failed before the shared codec/framer repair and passed afterward. `configtext` supplies the word-boundary scanner and quoter; explicit placeholder indexes and immutable lexical pipe positions preserve literal data through `top` and chained filters. Typed web refusals, masking and numeric-suffix replay passed. The literal-pipe editor and all three browser workflows completed baseline, 80-run, cost, promotion and live proof | final independent source review and closure metadata |
| Required-field identity | `walkRequired` and `checkListRequired` use private `storedListKeyTokens`/`splitStoredListKey`, preserving every entry and ancestor word without display suffix normalization. Actual composite, quoting and numeric-suffix regressions were observed red, then green under focused config/model selectors. Public display helpers keep their existing normalization; final full CLI/config race runs passed | no extra overlay is claimed: the actual pre-fix RED and post-fix GREEN establish this regression. Separate serializer round trips belong to the owner below |
| Printed member deletion | all eight terminal/bar root/context plain/bracket deletion replay cases retain accepted members and unrelated state, commit successfully and read back the saved format; complete terminal feedback passed | independent final review and the remaining functional workflows |
| Markup and golden maintenance | full markup helper and web/LG guards passed after the independently judged canonical operator-text exception. The independently judged additive `ProxyLCP` snapshot correction passed | two unrelated `/show/request/` navigation goldens remain red and are owned by `plan/release/fixes/spec-config-yang-accepted-input-correctness.md` AC-13; their snapshots were not refreshed |
| PID publication | producer and both daemon launch callers were observed red before repair; focused publication controls and the full runner package passed after atomic publication and error propagation. Fresh reload baseline and all 80 load invocations passed; annotation promoted, cost report read and native approval recorded. Ordinary live reload, the single-prefix regression and three runner primitives passed | no remaining PID proof |
| Measured execution | four `go test -covermode=count` measurements passed: the initial nine-package set, the final typed-pipeline/four-package boundary set, the final config required-identity set, and the new section tests. Reports sum duplicate blocks across package test binaries. Final added-block counts are `command_words.go` 21/21, `model_load.go` 6/6, the confirmed-session guard 2/2, `required.go` 7/7 and `list_key.go` 8/10. The focused section profile records both operator-renderer blocks in `SectionValidationError.Message` twice. Reports retain zero spans; fixture clients are not instrumented by these unit runs and have separate transport load proofs | no claim that every added block or the complete section renderer ran; scoped health exceptions are recorded below |
| Scoped package health | the 18-package race run passed config, CLI, API/REST/gRPC/gNMI, command, plugin-server, BGP config/update, hub and suggest; configtext had no test files. Full YANG retry, markup helper, web/LG guards, L2TP golden and fixture PATH retries passed. The runner package separately passed in full. The final five-package race run passed full CLI and config; a subsequent full config race run including the new section tests passed | full web still reports the two separately owned operational-root goldens; config/CLI still reports the foreign invalid LDP corpus fixture. CLI/testing reported a 30-second `session/edit-backup.et` timeout; its isolated race subtest passed unchanged. No all-package green claim |

The final browser campaign selected exactly the three `config-` drafts with
`iterations 80 minutes 60 parallel 2 burners 2 timeout 600 any-failure`.
Main audited 80 unique numbered successful invocations and 240 successful
browser closures. The controller's raw exit 1 was its `notReproduced` result;
the complete numbered output establishes the positive proof. The three source
hashes are recorded in `enum-isolated-browser-proof.json`. Browser child
temporary profiles used an owned tmpfs directory; Ze data and socket placement
remained unchanged. Earlier failures have no attributed causal fix.

The native cost report passed without listing these cases as hot or unmeasured.
All three drafts were promoted by plain move and their ordinary live selection
passed. That selection also passed the incidental recorded-config-save case and
reported an unrelated read-only download skip; none of these three owned
workflows was skipped.

The passing count profiles are `enum-branch-count.cover`,
`enum-final-boundary-count.cover`, `enum-required-identity-count.cover` and `enum-section-count.cover`,
with their JSON reports in the session scratch directory. The latter two
supersede only the source blocks their repairs changed.

Separable findings remain open with their existing owners: emitted edit-URL
aliases in `spec-web-cli-config-one-off-defects.md`; operational `config false`
projection and literal numeric-suffix set/positional/inline serialization in
`spec-config-yang-accepted-input-correctness.md` AC-13/AC-14. Ordinary named
brace output retains those suffixes. These records claim no implementation or
runtime serializer proof. The required-path and actual printed enum replay
regressions above are repaired independently of those findings.

The proof selections and remaining closure actions are explicit:

| Proof | Next command or action |
|-------|------------------------|
| Four promoted editor workflows | complete: all four passed their ordinary live selection, alongside the promoted pipe case and unchanged backup case; native lifecycle approval recorded |
| Three browser workflows | complete: final baseline, all 80 successful invocations of the fresh campaign, cost report, plain promotion and ordinary live selection passed for all three owned cases |
| Literal-pipe editor workflow | complete: baseline, all 80 load invocations, cost review, promotion and live run passed. The earlier invocation omitting `--draft` selected nothing and is not evidence |
| Offline set workflow | complete: baseline, all 80 invocations, cost review, promotion and ordinary live `test bgp parse --pattern enum-refusal-config-set` passed |
| Reload annotation | complete: baseline, all 80 invocations, cost review, promotion, native approval, ordinary live reload and `reload-parse-error-single-prefix` passed |
| Discrimination | complete: all eight native overlay walks passed clean and failed on the intended assertions under their breaks. Evidence is recorded in `enum-native-redwalk-evidence-20261113.md`; both transport observers specifically reported the missing enum refusal |
| Final scoped health | scoped package, runner primitive and promoted functional regressions, including ordinary gNMI, are recorded. The current eight-package boundary race run passed; the full-config obsolete display expectation was independently judged and its targeted rerun passed. Preserve the disclosed root golden/corpus failures and unchanged backup timeout with its isolated pass; no all-package green or `./le verify` claim |
| Assumptions | A-2's current 1033/1033 census and the named A-5/A-11/A-13 surface/source controls are recorded above; do not replace remaining workflow or overlay proof with those narrower confirmations |
| Section walk proof | both new direct section tests passed their focused race run and the subsequent full config race run. The focused count profile records both operator-renderer blocks in `SectionValidationError.Message` twice; it does not claim every legacy branch ran |
| Documentation and RFC freshness | complete: final owned RFC 7950/8907 freshness observed; doc-index `ae12aa2a` passed with no owned tracked map diff. Fresh `job-enum-round6-doc-check-f119ecea.log` reports seven unrelated anchors: sysrib `ApplyConfiguration` and `recomputeBest`, plus five old report `max*Len` symbols. No owned anchor failed. The round 6 source review is CLEAN; native record/check passed |

### Interop Tests (Scope: protocol)
- N-A: no wire behaviour changes.

## Files to Modify

| File | Change |
|------|--------|
| `internal/core/suggest/suggest.go` | `Correction` supplies the bounded value suggestion beside the existing command suggestion; the value-list renderer is in the new `valuelist.go` |
| `internal/component/config/yang/validator_error.go`, `validator.go` | `ValidationError` holds, for the enum, range, length and pattern kinds, segments, accepted values, suggestion, secret flag, format text and its operator error; `Error()` delegates for those kinds and keeps today's text for the others; `Unwrap` answers the operator error; `WithFix` and `WithPath` copy; `validateEntry` attaches the facts; `walkTree` threads segments; range, length, pattern arms build the kind |
| `internal/component/config/yang/enum.go`, `command.go` | declared order and help for the walk and `ArgDef`; the format reader's callers. The existing constraint-message reader is reused without changing `constraint_message.go` |
| `internal/component/config/yang/refusal.go` | corrections carry an explicit literal-or-placeholder distinction; `CorrectionText` uses the shared token quoter for concrete corrections and retains declared replacement syntax for placeholders |
| `internal/component/config/yang_schema.go` | `yangToLeaf` and the three leaf-list builders store enum facts, help and format text; the sensitive reader moves to `config/yang`; decimal64 maps to `TypeDecimal64` with `FractionDigits` |
| `internal/component/config/schema.go` | node facts and decimal64 type metadata; leaf and key restriction checks live in the new `leaf_value.go`, with enum/union/range/length/pattern facts and leaf-list member checks |
| `internal/component/config/parser.go`, `parser_diagnostics.go`, `line_error.go` | `parseLeaf` and `parseInactiveBracketMember` return the refusal inside `syntaxRefusal`; unwind segments become a typed full path through `namedRefusal`; the file-form fix replaces `write:`. Private diagnostic collection marks whether a scalar terminator was consumed; the new `parser_validation.go` collects synchronized failures without yielding an invalid tree, while `Parse`/`ParseAt` retain strict verdicts |
| `internal/component/config/parser_list.go`, `parser_freeform.go` | every value and key arm wraps its refusal in `syntaxRefusal` so it gets the path on unwind: the two list-key arms (`ValidateListKey`), whose `invalidListKeyError` is replaced by the friendly key refusal with its `peer` special case deleted; the two inline-list child arms, the multi-leaf arm, the bracket arm, the two value-or-array arms (the `(valid: ` text among them), the value arm after them, and the freeform attribute arm. The `parser_freeform.go` inline-list `KeyType` check stays as it is (exempt, `exabgp.yang` only) |
| `internal/component/config/setparser.go`, `setparser_meta.go`, `setparser_entries.go`, `setparser_refusal.go` | segments and file-form fix from the consumed tokens; creation through `EnsureListEntry`; the existing typed-line key check stays for stored entries (`lineKeyRefusal`), as the implemented creator table explains. Complete lines support diagnostic collection. A private stored-candidate path defers complete leaf payload constraints while retaining grammar/path/key/secret checks and quote provenance; ordinary parsers stay strict |
| `internal/component/config/list_key.go` | `EnsureListEntry` judges new typed keys unless `Tokens == 0`, without stripping a suffix; `ListEntryDestination` judges the destination; `ParseListAddress` pairs use the shared refusal. `splitQuotedTokensMarked` adds optional provenance to the same token codec, distinguishing raw stored-candidate brackets from quoted literal members. Private `splitStoredListKey`/`storedListKeyTokens` retain raw required-field identities; public display helpers retain their existing suffix normalization |
| `internal/component/config/tokenizer.go`, `edit_path.go` | the tokenizer and `QuoteIfNeeded` use the same lower `configtext` word-boundary scanner and quoter. `ParseEditWords` exposes quote provenance from the existing decoder for command parsing; there is no second lexer or character list |
| `internal/component/config/change_file.go` | `materializeMetaPaths` reads stored keys as `Tokens == 0`; `ParseCandidateChangeFile` privately enables stored-payload decoding through the existing change-file grammar, so a refused candidate remains available for correction without admitting corrupt structure |
| `internal/component/config/loader.go` | `parseConfigError` writes its legacy-syntax hint before the refusal |
| `internal/component/config/validate_sections.go` | `Message()` renders the refusal; refusals one per line |
| `internal/component/config/cli/cmd_validate.go`, `cmd_set.go` | JSON codes/fields from typed facts; `runValidation` consumes parser diagnostics before any semantic use of a tree; `ValidateContent` writes each indented block and `contentRefusal.Unwrap` preserves its ordered typed errors. Offline key fixes retain the full command, mask its secret operand and reach the writer as a declared `*opererr.Error` |
| `internal/component/cli/completer.go`, `completer_validate.go` | `ValidateValueAtPath` judges with `ValidateLeafValue` and the member check; the heuristic deleted; completion judges a typed new key with `ValidateListKey` |
| `internal/component/cli/editor_walk.go`, `editor_commands.go` | callers handle `EnsureListEntry` errors; ancestor probes prevent partial mutation before a refused key, including member operations; direct `SetValue` callers get a full root-form key fix with a masked suffix |
| `internal/component/cli/editor.go`, `editor_draft.go` | pending/session discovery and `readChangeFile` use candidate decoding; typed create refusals lose no schema facts to an internal prefix. A bare `SaveDraft` strict-validates before writing or consuming candidate changes when no semantic callback is installed; callback callers retain their existing validator |
| `internal/component/config/required.go` | `RequiredViolation.Path` keeps every instance and ancestor key through private `storedListKeyTokens`, separately from the schema anchor. `checkListRequired` quotes each concrete command word with `QuoteCommandWord` before the explicit value placeholder; literal pipes and numeric suffixes survive in standalone hints and contextual editor fixes |
| `internal/component/cli/testing/parser.go`, `runner.go` | the existing ordered step parser/dispatcher gains `fixture=api-set`; the new fixture implementation invokes the real editor API so a functional test can prepare a candidate that typed set correctly refuses |
| `internal/component/cli/model_commands.go` | `dispatchCommand` parses once, then `dispatchCommandWords` takes `top` before the pipe split. `cmdTop(CommandWords)` keeps bare-top navigation; with arguments it saves the outermost operator position on the copy, empties the resolution position and dispatches the remaining immutable words directly. `CommandWords.Tail` and `SplitPipe` preserve actual pipe positions without reprinting and parsing the line |
| `internal/component/cli/model_commands_option.go`, `model_commands_edit.go`, `model_commands_session.go`, `model_commit_window.go` | the incidental sites of Data Flow 7 answer `refreshConfig: true` in place of `configView: m.configViewAtPath(m.contextPath)`; the refused `set`, `insert`, `rename` and `copy` fixes built as `FixCommand` and framed with `m.operatorPosition()` (`FrameFix`, a file this spec creates) |
| `internal/component/cli/model_keys.go` | `handleEnter` drops a leading `top ` before its interceptions when the rest is `run ...`, `run`, `exit`, `quit`, `stop`, `restart` or `clear`; `finishPasteMode` applies the paste at the position the paste state records |
| `internal/component/cli/model.go`, `model_load.go` | `Model` gains `topOrigin` and `underTop` (set only on a copy by `cmdTop`); `rollbackConfirmed` and `applyLoad` answer `refreshConfig`; `dispatchWithPipe` and `ParsePipeFilters` retain immutable lexical pipe positions, including literal pipe filter arguments; the paste state records the position `load terminal` ran at (`commandResult` and the model beside `pasteModeLocation`); completion resolves a command behind `top` from the root |
| `internal/component/cli/editor_help.go` | `top` gains the form `top <command>`: "Run one command as if at the root context" (shipped in 95445948c5; the earlier wording "then come back" was false for `top edit`, which moves the position) |
| `internal/component/cli/model_render.go`, `model_commands_commit.go`, `model_commands_edit.go`, `validator.go` | multi-line refusals route to the viewport, one issue per line and the header on the status row. `cmdCommitRequest` validates an offline session before its no-daemon confirmed-commit guard; file-mode confirmed refusals retain their existing error contract. Typed `FixCommand` values frame commit/required/section issues at `operatorPosition`, and set/insert/rename/copy fixes keep the typed context. The current nine commit-file refresh sites are listed in Data Flow 7. The view's own path chooses its annotator. Type refusals keep their primitive body; enum peer errors preserve the typed refusal |
| `internal/component/web/cli_terminal.go`, `cli.go` | the terminal's `executeTerminalConfigMode` runs `top <command>` with an empty position and keeps the request's position unless `executeTerminalNav` answers a new one; the CLI bar's `dispatchCLICommand` calls itself with the rest of the line and an empty position (replacing `handleCLITop`'s silent drop of its arguments); `cliCommand` gains `Operator` (the request's `path`), kept by both `top` recursions; `handleCLICommit` and `runWebCommit` pass `cmd.Operator` to `EditorManager.runCommit` (`editor_commit_window.go`), which passes it to `cli.CommitRefusal`; `handleCommitPost` and `CommitNow` (`handler_config_commit.go`, `editor_commit_window.go`) pass nil; set-time value and key-leaf checks share the creator. Strict command decoding preserves quotes and bare-pipe provenance. Typed set refusals mask semantic secret operands before framing and retain their typed error chain. Terminal/bar deletion passes every path/member token to existing `DeleteByPath`, and feedback names the complete deletion |
| `internal/component/web/handler_config_form.go`, `handler_config_entry.go`, `error_fragment.go`, `editor.go` | value checks at set (form); key checks through the creator (ADD, rename, URL keys); the notice reads `Problem`, `Why`, `Fix` through `errors.As`; the decimal64 field type stays string |
| `internal/component/gnmi/set.go` | `applyUpdate` wraps the `SetSegments` error as `InvalidArgument` (AC-43) |
| `internal/component/command/argvalidate.go`, `argfaults.go`, `argbind.go`, `argdef.go` | `ValueArgumentError` from the renderer; `verdict` answers an `ArgumentFaults` of one for a lone value fault; `newArgumentFaults` accepts one value fault and then writes its problem and why without the count header; `wholeCommand` writes a value fault's placeholder in place of its typed value and runs when every fault is missing or a value fault; a value fault's list entry is its problem and why, never its fix; the `ArgumentFaults` and `newArgumentFaults` docs that say "two or more" are rewritten |
| `internal/component/plugin/server/arg_faults.go`, `internal/component/plugin/server/command.go`, `internal/component/command/local_data.go` | `restateArgumentFaults` and `ValidateModelArgs` restate a one-fault refusal through `RestatedForLine`; the `several` variable and comments at the dispatcher's restate, in `adoptablePositional`, `argFaultOutsideSelector` and `validateModelArgsAs` are renamed and reworded, their verdicts unchanged |
| `internal/component/bgp/plugins/cmd/update/update_text.go`, `cursor.go` | re-root to the command words |
| `internal/component/bgp/config/loader_test.go`, `internal/component/plugin/server/send_test.go`, `rfc8907_command_test.go`, and the other pins | rewritten (Current Behavior) |
| `internal/test/markupcheck/markupcheck.go`, `markupcheck_test.go` | the canonical `opererr.New`/`Wrap` direct text arguments follow their plain-text contract. The AST-bound exception preserves ordinary/nested markup, import shadowing, malformed-source scanning and the literal census; the independent judge approved this repair |
| `internal/test/runner/runner_exec_util.go`, `runner_exec.go` | `publishDaemonPID` writes, closes and assigns ownership before a sibling rename publishes the identity; both launch arms report publication failure and leave cleanup with their existing process owner |
| `internal/component/web/testdata/handler/get-l2tp-session-json.txt` | independently approved additive empty `ProxyLCP` field in the exact handler snapshot; no changes to the unrelated operational-root navigation fixtures |
| `rfc/discrimination/rfc7950/` requirement files | thirteen producer-break observations re-recorded through the native recorder and landed in 024ba1d203; the old monolithic JSON path no longer exists |
| `rfc/audit/rfc7950/RFC7950-8.3.3-2.json`, `rfc/audit/rfc7950/RFC7950-9.1-1.json` | independent rejudgment stamped in 0f8c7ccf04: the general constraint row stays unimplemented and the broad lexical row is honestly weak; the separately owned post-release lead is not implemented by this correction |
| `plan/journal/parser-refuses-a-form-its-producer-emits.md`, `plan/release/fixes/spec-web-cli-config-one-off-defects.md` | metadata only: the 2026-11-13 emitted edit-URL alias finding and its existing owner; no route repair claimed |
| `plan/journal/operational-schema-enters-configuration.md`, `plan/journal/format-cannot-hold-a-state-the-tree-holds.md`, `plan/release/fixes/spec-config-yang-accepted-input-correctness.md` | metadata only: operational-root projection (AC-13) and source-traced literal numeric-suffix serialization (AC-14); preserve their separate ownership and unexecuted serializer status |
| `plan/journal/guard-blocks-its-own-authors-repair.md`, `plan/post-release/engineering/spec-gates-share-one-semantic-population.md` | metadata only: the recorded disposition-time partial-edit refusal and its existing gate owner; no global gate repair claimed |

## Files to Create

- `internal/core/suggest/valuelist.go` and `valuelist_test.go`; `internal/component/config/yang/refusal.go`, `refusal_test.go`, `format.go`, `format_test.go`; `internal/component/config/leaf_value.go` and `value_refusal_test.go`
- `internal/component/config/parser_validation.go`, `parser_validation_test.go`, `setparser_candidate.go`, `change_file_candidate_test.go`; `internal/component/config/cli/cmd_set_refusal_test.go`, `cmd_validate_refusal_test.go`, `cmd_validate_refusals_test.go`
- `internal/component/cli/model_top_test.go`, `model_view_refresh_test.go`, `model_view_refresh_remaining_test.go`, `fix_frame_test.go`; `editor_key_activation_refusal_test.go`, `editor_set_key_refusal_test.go`, `editor_candidate_repair_test.go`, `validator_refusal_remaining_test.go`, `completer_union_verdict_test.go`, `model_confirmed_validation_order_test.go`; `internal/component/config/required_path_words_test.go`
- `internal/component/cli/fix_frame.go` - `FixCommand` with explicit placeholder indexes and `FrameFix(fix FixCommand, operator []string) string`, the one framer of editor-command fix lines (Data Flow 5), and `Model.operatorPosition()`
- `internal/core/configtext/token.go` - the existing word-boundary/address scanner and byte-preserving quoter moved below config/YANG/CLI; `QuoteCommandWord` protects a literal pipe
- `internal/component/cli/command_words.go` - immutable `CommandWords`, strict parsing through the existing config decoder, copied token access and bounded transformations retaining lexical pipe positions through `top` and every filter
- `internal/component/cli/required_fix_replay_test.go`, `model_pipe_literal_test.go`, `command_words_ownership_test.go`, `internal/component/web/cli_pipe_literal_test.go` - review regressions for canonical required addresses, literal numeric-suffix identity, standalone hint replay, literal pipe arguments, parser ownership and malformed quoted secret assignments; the three old direct parser/top test inputs are converted without removing assertions
- `internal/component/cli/key_refusal.go`, `model_key_refusal_test.go`, `model_refusal_frame_test.go`, `model_activation_refusal_test.go`, `validator_peer_refusal_test.go` - shared key framing and the model's root/context refusal controls
- `internal/component/cli/testing/fixture.go`, `api_set_fixture_test.go` - the ordered API-candidate fixture, extending the existing `parser.go` and `runner.go` rather than adding a second runner
- `internal/test/fixture/register_config_refusal.go`, `plugin_fixture_config_refusal.go` - explicit REST/gRPC/gNMI registrations with `StopsDaemon`, plus a TLS certificate fixture and pinned TLS/token clients. REST takes its leased endpoint and extends `plugin_fixture_01_api.go`; TLS clients take listener address and certificate path. Each scenario tests key refusal and value commit, and REST also tests two refusal blocks
- `internal/component/web/cli_activation_refusal_test.go` - enum-keyed activation framing with real member-state controls
- `internal/component/cli/fix_tokens_refusal_test.go`, `internal/component/web/cli_fix_tokens_test.go`, `internal/component/config/refusal_literal_fix_test.go` - actual printed-fix replay and literal-versus-placeholder controls for command, hierarchical, set-file and default corrections
- `internal/component/cli/model_key_secret_refusal_test.go`, `internal/component/web/cli_typed_key_refusal_test.go` - quoted/backslash/spaced sensitive suffixes, positional framing and typed error preservation
- `internal/component/web/cli_delete_fix_test.go` - actual commit correction replay through both web command surfaces, with retained-member/unrelated-state/commit/readback assertions
- `internal/test/runner/daemon_pid_test.go` - atomic publisher, ownership/failure cleanup and both daemon caller failure controls
- `internal/component/config/parser_multikey_refusal_entry_test.go`, `internal/component/cli/model_multikey_refusal_entry_test.go`, `model_problem_path_entry_test.go`, `model_multikey_secret_refusal_test.go` - actual locally resolved multi-key parser/editor proofs, escaped diagnostic path words, printed-fix identity and masked sensitive suffixes
- `internal/component/command/invocation_scan.go`, `pipe_invocation_test.go`, `internal/component/plugin/server/enum_fix_replay_test.go`, `internal/component/cli/operational_invocation_words_test.go`, `model_keys_quote_escape_test.go` - one operational quote-aware scanner, checked-pipeline and actual dispatcher correction replay, and editor input-boundary controls
- `internal/component/web/testing/close_budget_policy_test.go` - actual cleanup policy and bounded-hang controls for the existing parallel browser command budget
- `test/draft/editor/pipe/literal-pipe-filter.et`, `test/draft/web/config-literal-pipe-filter.wb` - original draft locations; promoted to `test/editor/pipe/` and `test/web/` after baseline, 80-run and cost proof. Ordinary live projections passed
- `test/draft/web/config-enum-refusal.wb`, `config-range-length-pattern-refusal.wb` - original stronger draft locations; all old assertions retained, complete correction/readback, 80-run and cost proofs passed, then promoted to `test/web/` and passed their ordinary live projections
- `test/editor/navigation/top-command.et` and the other functional tests in the TDD plan

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no leaf, RPC or extension added |
| YANG validation constraints | No | enforcement exists; the refusal changes, plus the decimal64 range and leaf-list enum defect fixes |
| YANG custom validators | No | none added |
| CLI commands/flags | Yes | the editor verb `top` gains the form `top <command>` (`internal/component/cli/editor_help.go`); no operational command added |
| CLI grammar (keyword before value) | Yes | `top <command>` is a keyword followed by a whole command line, the shape `run <cmd>` already has; no YANG node named `top` exists, so it cannot be misread as a path; `./le cli grammar` judges the editor help table it reads |
| Editor autocomplete | Yes | the `top ` branch in `commandCompleterInput` and `updateCompletions`; values keep their sorted completion order |
| Functional test for new RPC/API | Yes | `test/plugin/api-config-enum-refusal.ci`, `test/plugin/grpc-config-enum-refusal.ci`, `test/plugin/gnmi-set-enum-refusal.ci` |
| Pipe completeness | Yes | no command output added; `dispatchCommand` takes `top` before its pipe split, so `top show ... \| <pipe>` reaches the inner command's pipe handling (`dispatchWithPipe`), which refused it before |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no runtime dependency; the refusal is the operator's own input, answered at the command |
| Prometheus counters/metrics | N-A | none; the gNMI error metric already records `InvalidArgument` |
| BGP family surface | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `top <command>` in the editor; check `./le feature report` in Phase 3 |
| 2 | Config syntax changed? | Yes | `docs/architecture/config/syntax.md`: "Enumeration values" beside "Unknown keys", with range, length, pattern and the key check at creation; Phase 2 |
| 3 | CLI command added/changed? | Yes | `docs/guide/config-editor.md` (navigation: `top <command>`, every command behind it) and the editor command reference the help table feeds; Phases 3 and 4 |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`: argument refusals, one fault restated; `docs/architecture/api/architecture.md`: the gNMI `Set` code; Phases 4 and 5 |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/config-editor.md` (candidate repair, command quoting and confirmed-session validation order), `docs/guide/web-interface.md` (quoted correction replay, masked suffix and member deletion), `docs/guide/api.md` (whole key correction and unchanged running state) |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior changed or newly proven? | Yes | RFC 7950 Section 9.3 decimal64 range now enforced by the parser; `docs/architecture/config/yang-config-design.md`; Phase 2 |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/ci-format.md` and `docs/functional-tests.md`: ordered API candidate fixture; `docs/architecture/testing/runner-architecture.md` and the readiness section of `ci-format.md`: atomic PID publication; `docs/architecture/web-components.md`: the operator-text markup contract |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` (the refusal, its error chain, the format reader) and `docs/architecture/cli/error-surface.md` (multi-line errors in the SSH viewport, the `top` fix rule, the typed fix framed by each surface); Phases 1 and 3 |
| 13 | Route metadata keys? | No | none |
| 14 | Prometheus counters? | No | none |
| 15 | Registered plugin, event, command or inventory changed? | No | none |
| 16 | Changed source referenced by doc anchors? | derived | `./le spec citation anchors spec plan/release/fixes/spec-config-enum-refusal-proposes-valid-values.md` per phase |
| 17 | Docs show examples for this area? | Yes | grep `docs/` for `invalid enum`, `expected one of`, `invalid peer name`, `write:`, `(set bgp` per phase |

## Implementation Steps

The owner directed on 2026-11-13: "Complete the work fully and commit it all at the end."
The earlier three implementation commits remain landed; the remaining work
lands in the final closure sequence. A phase that edits an RFC 7950 producer
re-records its records in the same commit. Evidence
is the named scoped runs (`./le job run label <area> command go test -race
<packages>` and the named functional tests), never `./le verify`.

Order: Phase 1, then Phase 2, then Phase 3, then Phase 4 (the web imports
`internal/component/cli`, which Phase 3 changes). Phase 5 may run beside
Phase 3 once Phase 2 lands, provided it changes no exported signature of
`internal/component/command` that `cli` reads; otherwise it runs after Phase 3.
Phase 6 runs last. Before Phase 2 commits, the `AddListEntry` grep is re-run
against the creator table (A-9). Within Phase 3, `top <command>` lands first,
as its own commit, because the commit-refusal fix lines print it.

The Phase 3 continuation also reaches `internal/component/config` for safe
multi-refusal diagnostics and stored-candidate repair, and
`internal/component/cli/testing` for the ordered API fixture. These are
prerequisites of AC-8 and AC-23 exposed by the actual workflows, beyond the
original offline/editor package set. The current Files to Modify/Create and
Unit Tests tables name their sources and proof. Shared fix-token rendering,
typed secret masking and web member deletion are repaired and their focused
controls passed. The same continuation includes the markup guard and atomic
PID publication blockers. Offline/reload/editor/API load proofs and their
promotions are recorded above. The literal-pipe editor, three browser load/live
proofs, eight native overlays and current scoped package evidence are now
recorded. Final native documentation and owned RFC freshness checks are also
recorded. The authorized round 6 source review is CLEAN; native record/check passed; both closure commits remain owed.

| Phase | Packages (files) | Tests | Red walk owed |
|-------|------------------|-------|---------------|
| 1 Refusal and schema facts | `internal/core/suggest`; `internal/component/config/yang` (`validator_error.go`, `validator.go`, `enum.go`, format reader; unchanged constraint-message reader reused); `internal/component/config` (`yang_schema.go` only) | `TestValueListRender`, `TestValidationErrorOperatorError`, `TestFrameKeepsProblemAndWhy`, `TestTypeFormatReader`, `TestYangToLeafCarriesEnumHelp`, `TestValueOrArrayEnumDeclaredOrder`, `TestMultiLeafAndBracketCarryEnums`, `TestValidateEnumerationSharedBody`; RFC-tagged enum tests, `TestValidationErrorTypeContract` and the reference walk test unchanged | `./le rfc record` for `validateEnumeration`, `walkTree`, `yangToLeaf` and the range, length, pattern producers; break the renderer's alignment and see `TestValueListRender` red |
| 2 Parsers and keys | `internal/component/config` (`schema.go`, `parser*.go`, `line_error.go`, `setparser*.go`, `list_key.go`, `change_file.go`, `validate_sections.go`, `loader.go`); `internal/component/bgp/config` (the peer-name pin) | the parser, set-parser, list-key, change-file, schema, loader and section tests; the `bgp/config` loader test; `test/parse/*`, `test/plugin/enum-refusal-daemon-start.ci`, `test/reload/enum-refusal-reload.ci`, the BMP pins | `./le rfc record` for `refuseInvalidCustomSections`; break the creator's key check and see `TestEnsureListEntryJudgesNewKey` red; judge `Tokens == 0` and see `TestChangeFileReadsBackUnjudged` and `TestLookupWalkFindsEveryStoredKey` red; widen the exemption to a typed `default` and see `TestTypedDefaultKeyStaysRefused` red; drop one value or key arm's `syntaxRefusal` and see `TestEveryValueArmNamesTheFullPath` red; drop `WithPath` and see `TestParserEnumRefusalNamesFullPath` red; break the decimal64 arm and see `TestValidateLeafValueDecimal64Range` red |
| 3 Offline and SSH editor, `top <command>` | `internal/component/config/cli`; `internal/component/cli` (position, paste, framing, command words and incidental-view files); continuation prerequisites in `internal/component/config`, `internal/core/configtext` and `internal/component/cli/testing` | `cmd_validate`, `completer`, model, commit, literal replay, candidate and `top` tests; `test/editor/validation/*`, `test/editor/navigation/top-command.et`, `test/editor/pipe/set-cli-format.et`, `test/ui/config-*` | revert the viewport route and see `TestEditorMultiLineErrorKeepsThePromptRow` red; let `cmdTop` keep the emptied position and see `TestTopKeepsPositionUnlessInnerMoves` and `top-command.et` red; remove the `handleEnter` strip and see `TestTopInterceptedVerbsBehaveAsWithout` red; drop the paste position and see `TestTopLoadTerminalPastesAtTheRoot` red; print the root form inside a context and see `TestCommitFixUsesTopInsideAContext` red; frame from the copy's `m.contextPath` instead of `m.operatorPosition()` and see `TestTopCommitFixUsesTopSetInsideAContext` and `TestTopRefusedSetFixKeepsTop` red; put one incidental site back to a copy-built `configView` and see `TestTopSetRefreshesAtThePosition` red; resolve `refreshConfig` at another path and see `TestIncidentalViewUnchangedWithoutTop` red; split the pipe before `top` and see `TestTopPipedShowRunsAtTheRoot` red |
| 4 Web, REST, gRPC, gNMI | `internal/component/web`; unchanged `internal/component/api` adapters reused and tested; `internal/component/gnmi`; `internal/test/fixture`; `cmd/ze/hub` (tests); continuation blockers in `internal/test/markupcheck` and `internal/test/runner` | web, api, gnmi, helper and PID tests, `test/web/*`, the three `test/plugin/*-enum-refusal.ci` | remove the terminal `set` check and see `config-enum-refusal.wb` red, with the binary rebuilt; let the web `top` keep the emptied position and see `TestWebTopCommandKeepsPosition` red; drop the bar's commit position and see `TestCLIBarCommitFramesTheFixWithThePosition` red; let the web `top` recursion reset `cliCommand.Operator` and see `TestWebTopCommandKeepsPosition` (its `top commit` case) red; remove the `applyUpdate` wrap and see `TestSetUpdateRefusalIsInvalidArgument` red |
| 5 Command arguments and update text | `internal/component/command`; `internal/component/config/yang/command.go`; `internal/component/plugin/server`; `internal/component/iface/cmd` (test); `internal/component/bgp/plugins/cmd/update` | command and dispatcher tests, `test/plugin/command-arg-enum-refusal.ci`, `test/plugin/update-text-origin-enum-refusal.ci`, `test/ui/cli-argument-*.ci`, `test/ipsec/ipsec-dataplane-show.ci`, `test/plugin/policy-test-errors.ci` | make `verdict` return a lone fault bare again and see `TestOneValueFaultIsRestated` red; each negative pin proven red by making the dispatcher answer with the new refusal (R-11) |
| 6 Docs | `docs/` pages of the Documentation checklist not yet edited by their phase | `./le doc check all` | - |

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N has an implementation at file:line |
| Correctness | a grep for the old texts (Current Behavior inventory list) in non-test Go under `internal/` and `cmd/` finds none, except the kept `<type> error at` text of the kinds that hold no operator error and the exempt `exabgp.yang` inline-list key text |
| One error type | no error type beside `opererr.Error`; the `errors.As` tests (AC-21) prove each refusal reaches its writer typed; `./le arch operator-error check` and `./le arch error-inventory check` clean on the changed lines |
| Creators | the `AddListEntry` grep matches the creator table; `Tokens == 0` is the only form left unjudged |
| Parser paths | every hierarchical value and key arm that refuses wraps the refusal in `syntaxRefusal` |
| `top <command>` | one rule, no list of inner commands, on the SSH dispatcher, the `handleEnter` interceptions and each web editor; the real position moves only through the inner result or a web navigation answer; the paste lands where `load` ran; a grep for `configView: m.configViewAtPath(m.contextPath)` finds only `cmdShow`; every editor-command fix line goes through `FrameFix` |
| Data flow | values and help read only through `EnumNamesDeclared`/`EnumValueSummaries` or the node fields they fill |
| Rule: testing | every rewritten assertion as strong as before, judged by `ze-test-judge`; negative pins keep their old needle |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| one refusal, no new error type | `gopls symbols internal/component/config/yang/validator_error.go`; the diff adds `ValueArgumentError` as a fault type only |
| old texts gone | the Critical Review grep |
| `top <command>` on every editor | `top-command.et`, `TestWebTopCommandKeepsPosition` |
| every path proven | every Wiring Test row's test exists and passes |
| records fresh | `./le rfc check` has no RFC 7950 `discrimination:` line |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the refused value is quoted (`%q`), so it cannot forge a line in a log or the web fragment; the web `top` validates the position it is given as today (`ValidatePathSegments`) |
| Error leakage | a sensitive value and its suggestion never appear (AC-16) |
| Authorization | `top <command>` runs the inner command through the same dispatcher, interceptions and authorization as typing it at the root; `top stop` and `top restart` ask the same confirmation; it grants nothing |
| Resource exhaustion | the suggestion costs values times length, on an error path only; nested `top top ...` is bounded by the line length |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | fix in the phase that introduced it |
| Test fails for the wrong reason | fix the assertion or setup |
| Test fails on behavior mismatch | re-read the producer in Current Behavior; if misunderstood, back to design |
| Lint failure | fix inline; if architectural, back to design |
| Functional test fails | check the AC: wrong AC to design, right AC to implementation |
| 3 fix attempts failed | stop, report the three approaches |

## Design Insights

- The operator error already carries a multi-line list in its why (`ArgumentFaults`), so the accepted values need no new part of the type.
- Both parsers answer no tree on a parse error, so the commit walks never meet a value the parser refused; the walk still refuses what the parser could not judge, which is why the leaf-list nodes gain enum facts.
- A key check belongs where an entry is created from typed tokens, and one field says which: `Tokens == 0` marks an address no operator typed (the anonymous step, a stored key read back).
- The parser's refusal is built before its path is known, so every value and key arm joins the unwind and the path is added once, at the parse root, by rebuilding the immutable error rather than by prefixing its text.
- The SSH editor already runs each dispatched command on a copy and moves the real position only through the result, so `top <command>` restores nothing by hand; the only places a position is read later or earlier than the dispatch (the `handleEnter` interceptions, which read none, and the paste, which reads it at Ctrl-D) are handled where they read it.
- A fix line that depends on the position cannot be a string built where the position is unknown; it is a typed fix the rendering frame writes.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `helper-bypassed-by-an-open-coded-copy` | 2026-10-13 | `validateStringPatterns` beside `config.ValidateLeafValue` (completer refuses valid prefixes) |
| `declared-format-contradicts-payload` | 2026-10-13 | ranged decimal64 refused by every parser (`parseNumericRangeValue`) |
| `parser-refuses-a-form-its-producer-emits` | 2026-10-13 | in-entry key-leaf `set` corrupts the session change file |
| `guard-added-to-one-half-of-a-pair` | 2026-10-13 | web terminal and CLI bar `set <list> <key-leaf> <v>` stored in an anonymous entry |

## Key Design Decisions

| Decision | Reason |
|----------|--------|
| The refusal is an `opererr.Error` held by the existing `configyang.ValidationError`, whose `Error()` delegates to it and whose `Unwrap` answers it; the kinds that hold none keep today's text | the rule allows one refusal shape; `ValidationError` already carries `Type`, `Path`, `Expected`, `Got` and is read by the RFC tests and the JSON; delegation prints the text once, and both `errors.As` targets resolve |
| The accepted values are the why lines | they say why the value is refused; `ArgumentFaults` is the precedent for a list in the why |
| The fix is per surface: the frame that holds the surface calls `WithFix`, or writes the typed fix of a `ConfigValidationError`; the problem and why are built once | a frame adds only the fact it holds (`docs/contributing/ze-go-style.md`), and only the frame knows the position |
| Fix table: a hierarchical file and an editor `load` snippet `fix: <leaf> <value>;`; a set-format file `fix: <verb> <path> <value>`; a refused editor `set` the command as typed, corrected (relative inside a context); an editor commit refusal and the missing-mandatory hint `set <full path> <value>` at the root and `top set <full path> <value>` inside a context, on the SSH editor, the web terminal and the CLI bar; offline, web form, REST, gRPC, gNMI `fix: set <path> <value>` (no position); a key `fix: rename <list-path> <old> to <key>`; a refused plain leaf-list member in a commit candidate `fix: delete <path> <member>` (behind `top` inside a context); `discard` fixes unchanged; a command the whole command with the placeholder | the fix is the exact text the operator types where they are (Q-9) |
| `top <command>` is an argument of `cmdTop` inside `dispatchCommand`, and `handleEnter` drops `top ` before its own interceptions | dispatched verbs already run on a copy; the intercepted verbs (`run`, `exit`, `quit`, `stop`, `restart`, `clear`) read no position, so they behave behind `top` exactly as without it; the owner asked for every command |
| The real position moves only when the inner result sets `clearContext` or `newContext`, which exactly `edit`, `up` and `top` do | those two fields are the editor's one way to move (`handleCommandResult`); a failure leaves both unset, so the position stays on failure too |
| Only an asked-for view (`cmdShow`, `cmdShowPipe`, and `edit`/`up` at their new position) answers `configView`; the 26 current incidental redraw sites answer `refreshConfig`, which the appliers resolve at the real position; every view records the path it was built at, and the annotator reads that path | `top show` must show the whole configuration (the owner's purpose), as `show <path>` keeps its own view; passing an incidental view through would show the root after `top set`, a position the prompt does not name, and the pass-through rule cannot tell the two apart, so the verb says which by the field it answers; the switch is invisible without `top` (A-14) |
| One framer, `FrameFix`, from the operator's real position, which `cmdTop` saves on the copy (`topOrigin`) apart from the emptied resolution position; the web carries it as `cliCommand.Operator` | under `top` the copy's position is the root, so a framer reading it prints `set <full path>` while the operator is inside a context, which is wrong as typed; one function keeps every surface's rule the same; the web already imports `internal/component/cli` |
| `top` with arguments is taken before the pipe split | the split runs first today and refuses `top show \| ...`; the owner asked for every command behind `top` |
| History records the typed line, `top` included | the strip only changes which verb the interceptions test; the operator recalls what they typed |
| The paste state records the position `load terminal` ran at | the paste is applied at Ctrl-D, after the dispatch; without it `top load terminal` would paste at the context |
| Web: the terminal keys on `executeTerminalNav`'s new path; the CLI bar needs no signal | the bar's position changes only through `writeCLIResponse`, which only its navigation handlers call |
| The parser's path comes from `prependSyntaxPath` on unwind, every value and key arm joins it, and the path is written into the refusal once, at the parse root, by `WithPath` | it already names keys and the `ParseAt` context and costs nothing on success; the error is immutable, so it is rebuilt rather than prefixed, and the slash prefix is dropped |
| `invalidListKeyError` is replaced and its `peer` special case deleted | the friendly key refusal names the path (`bgp peer <key>`), so a word reserved for one list name is a per-leaf special case the Task forbids |
| No continuation-indent table | each surface prints an operator error as it prints one today; this spec changes only the `; ` joins, `ValidateContent`'s block indent and the SSH row |
| Declaration order everywhere a refusal lists values; completion keeps sorted order | the YANG order is the author's; completion order is a separate, tested fact |
| The set parser keeps its own raw-token key check, `SetParser.lineKeyRefusal`, at the three sites that judged at HEAD (`setparser.go`, `setparser_entries.go`, `setparser_meta.go`), and the creator judges new entries too; both frame the same refusal | the creator returns a stored entry unjudged, so dropping the set parser's check would accept a typed `default` or `x#2` naming a stored entry, which was refused before (AC-14, `TestSetLineKeyOnStoredEntryStaysRefused`); an `inactive` line on a stored `#N` entry reaches it as before, and one creating a new entry is judged where it is created |
| A value is proposed by `suggest.Correction`: within one edit up to four characters, within a third of the length (rounded down) beyond; `suggest.Command` keeps `len/2` for command suggestions | a fix line is typed as printed, so the bound is tighter than a command hint's; `len/2` steered a boolean spelling (`color true;`) to an unrelated value (A-1) |
| A leaf with one accepted value takes that value as its correction | the fix line is then the exact text to type, not a choice of one (`refusalFacts.correction`); BMP prints `fix: route-action monitor;` (AC-3) |
| `ListAddress.Tokens == 0` is not judged; every other address is, with no suffix stripped | only the anonymous step and a stored key read back build `Tokens == 0`; every typed address has `Tokens >= 1`, so a typed `default` or `x#3` stays refused and stored `KeyDefault`, `#N` and change-file entries stay readable |
| A lone value fault is an `ArgumentFaults` of one | every route already restates an `ArgumentFaults` through the one function, so one fault gets the whole-command fix with no route edited beyond accepting it |
| `formatPeerError`'s `ErrTypeEnum` arm is kept and passes the `ValidationError` through with the peer context | the walk still refuses values the parser could not judge before this spec, and a candidate loaded from an older file can hold one |
| Range, length, pattern share the type through `ValidationError.Type` | `ErrTypeRange`, `ErrTypeLength`, `ErrTypePattern` exist; no kind field is added |
| The legacy-syntax hint is written before the refusal | nothing is appended after `fix:` |
| gNMI `applyUpdate` answers a session set refusal as `InvalidArgument` | a refused argument answered as `Unknown` is a defect; the metric already counts it as `InvalidArgument` |

Changed from the 2026-10-14 design:

| 2026-10-14 | Now | Why |
|------------|-----|-----|
| a new refusal type in `internal/core/suggest` with its own renderer | `opererr.Error` held by `ValidationError`; `suggest` gains only the value-list renderer | 3e94cb7c40 and `ai/rules/user-facing-errors.md` |
| fix lines with no label (`write:`, `set ...`, `Retry the command ...`, `Choose one of ...`) and a separate why line per frame (`The value was not set.`) | `fix: <command>` last; one why built by the producer | the operator error layout |
| a new typed line error and a continuation-indent table | `config.LineError`, `SplitLine`, today's printers | they exist |
| a new parser path stack | `prependSyntaxPath` on unwind, every value and key arm joining it, applied at the parse root | it exists and already names keys |
| the parser does not check unions; union cases proven on the walk | the parser refuses union non-members (`validateUnionValue`) | 238bdb62e9 |
| the doubled reload prefix | `reload: parse config:` | fixed since |
| `Schema.ValidateTokenPathKeys`, `TokenPathSegments`, `CheckListKey` | the check in the creator, keyed on `Tokens` | multi-leaf-key addressing gave every surface one creator |
| `ValidateArgs` gains command words; `EnsureStep` gains a field | `ArgumentFaults` and `RestatedForLine` carry the line | 3af4462e82, 37c3a2abb1 |
| the walk's group re-root and inherited-refusal dedup (AC-23) | dropped; the walk's enum arm passes the refusal through | the parser refuses those values first |
| Q-5 kinds with fixed why text | a declared `error-message` is the why | `ConstraintMessages` (3e94cb7c40) |
| the BMP worked example with `redistribute` accepted | `redistribute` refused, `monitor` the one value | fa74dc4c00 |
| a fix outside the context with no exact command | `top set <full path> ...` | owner answer Q-9 |
| one closing commit, `./le verify worktree` | per-phase commits, scoped runs | `ai/META-DECISIONS.md` |

## Known Limitations

- Option (d), a YANG-declared reason for a removed value, is not designed here: the owner chose (c).
- Pattern format text: of the 24 pattern typedefs, 0 declare `ze:help` (2026-10-13), so a typedef'd refusal shows its description's first sentence; the 94 inline patterns show the leaf's `ze:help`, which names the setting more than the format. Closing it is a YANG authoring pass.
- No parser enforces a leaf-list's YANG `range` or `length` (journal `guard-added-to-one-half-of-a-pair.md`, 2026-10-13 r15 row); AC-39 holds no such case.
- A YANG `uint64` leaf maps to `TypeUint32` (journal `declared-format-contradicts-payload.md`, r15 row), so the width case keeps the type text.
- `retiredKeywords` stays a Go map.
- The `exabgp.yang` inline-list key check (`parser_freeform.go`, `KeyType`) keeps its text: no Ze operator surface writes such a key.

## Owner Questions

| ID | Question | Recommendation | Answer |
|----|----------|----------------|--------|
| Q-0 | Direction: (c) one generic friendly refusal on every path, or (d) a YANG-declared removed-value reason | - | **Answered by the owner, 2026-10-13: "M2 go with (c), friendly errors everywhere".** |
| Q-2 | Union leaves with enum members get the generic refusal? | Yes | **Answered by the owner, 2026-10-13: "yes".** Since 238bdb62e9 the parser itself refuses a union non-member, so the refusal is reached on every parser path as well as the ones the question named; the answer has one reading under that change |
| Q-3 | Command enum arguments use the same refusal? | Yes | **Answered by the owner, 2026-10-13: "yes".** |
| Q-4 | Long value lists: print all, or cap? | Print all | **Answered by the owner, 2026-10-13: "print all".** |
| Q-5 | Range, length and pattern in this spec? | - | **Answered by the owner, 2026-10-13: "b".** In this spec, records re-recorded |
| Q-6 | Web refuses at set (terminal `set`, form POST)? | Yes | **Answered by the owner, 2026-10-13: "yes".** Since 238bdb62e9 `ValidateLeafValue` also judges unions, so the web refuses a union non-member at set too, as the SSH editor does: one reading, the same check as SSH |
| Q-7 | SSH commit refusal: (a) full refusal, one issue per line, in the viewport; (b) first line only | (a) | **Answered by the owner, 2026-10-13: "a".** |
| Q-8a | Web key checks at the command (terminal `rename`, `copy`, `insert`, `activate`, `deactivate`, form rename, form ADD key, URL ancestor keys)? | Yes | **Answered by the owner, 2026-10-13: "yes".** Delivered through `EnsureListEntry` and `ListEntryDestination`, which those paths reach |
| Q-8b | Web CLI bar `set` gets the value and key checks? | Yes | **Answered by the owner, 2026-10-13: "yes".** |
| Q-9 | How should an SSH editor fix line name a config path outside the operator's current `edit` context? `discard` already takes a root path in every context, so its fixes are exact today; `set` always prepends the context and `top` took no arguments, so no single command reached a path outside the context. Options were (a) the root-form `set` typed after `top`, (b) moving the editor to the root on such refusals, (c) a root-path form of `set` | (c) | **Answered by the owner, 2026-11-12: "I think the top command should have a set sub option, which effectively make it transient. So it will be a top set bgp instead of the proposed set top bgp."** and, on making it general: **"Yes, you can make it top effectively. A transient change of directory for the command behind, whatever the command is."** Designed as `top <command ...>`: `top` alone keeps its meaning; with a command, that command runs once at the root, whatever it is, and the position is kept unless the command itself moves it (Data Flow 7 and 8, AC-45 to AC-52) |

Q-1 is unused. Every owner question is answered. The owner approved the design
on 2026-11-12 ("approve"), after review round 25, and the status moved to
`ready`.

## Design and Source Review History

### Design review history

| Round | Date | Result | Disposition |
|-------|------|--------|-------------|
| 1-17 | 2026-10-13 to 2026-10-14 | rounds 1 to 17 over the 2026-10-14 design; full table in git history at `09be99b7db` | every finding applied then; the design they reviewed is replaced by the 2026-11-12 rework |
| rework | 2026-11-12 | not reviewed | Disposition REWORK at `e660eff82e`: the refusal rebuilt on `opererr.Error` and `ValidationError`; existing mechanisms extended instead of new pieces; union cases moved to the parser; fix lines in the `fix:` layout; AC-42 added; BMP pins added; stale `./le verify` and single-commit instructions replaced |
| 18 | 2026-11-12 | 2 BLOCKER, 7 ISSUE, 3 NIT (full review of the rework) | All 12 verified at the producer and applied (I-6 with correction). B-1: the first-key check moved from `ParseListAddress` to the creator, with `TestLookupWalkFindsEveryStoredKey`. B-2: `ValueArgumentError`, an `ArgumentFaults` of one, the placeholder in `wholeCommand`, fault entries without `fix:`. I-1: the error chain with delegating `Error()`, `Unwrap`, `WithFix`. I-2: `prependSyntaxPath` reused, A-4 and the `Parser.path` extension deleted. I-3: A-8 and R-14 deleted, the `formatPeerError` arm kept, leaf-list nodes gain enum facts (AC-5, R-16). I-4: AC-43, the gRPC cases and client. I-5: Phase 4 after Phase 3. I-6: fixes typed in the current context, the leaf-list `delete` fix, Q-9 for a leaf outside the context. I-7: `ipsec-application-override.ci` pinned, AC-29 to AC-31 in a Go test. N-1 to N-3: the proof rests on `errors.As`, the hint before the refusal, `ValidateContent` joined with `errors.Join` |
| 19 | 2026-11-12 | 0 BLOCKER, 5 ISSUE, 2 NIT (narrow review of the round-18 edits) | All 7 verified at the producer and applied. I-1: exemption narrowed to `ListPathStep`'s anonymous arm, no suffix stripped, the key-only arm through the creator, `TestTypedDefaultKeyStaysRefused`. I-2: the creator table under A-9, the grep the gate. I-3: the other `ValidationError` kinds keep today's text, two pins added. I-4: `WithPath` at the parse root, space-separated, the slash prefix dropped. I-5: gNMI `applyUpdate` wrapped as `InvalidArgument`; REST 400 and gRPC `InvalidArgument` in the ACs. N-1: stale `several` comments listed. N-2: a resume-time refusal (AC-44, R-17), withdrawn in round 20. Q-9 rewritten for one editor convention |
| 20 | 2026-11-12 | 1 BLOCKER, 2 ISSUE, 1 NIT (narrow review of the round-19 edits) | All 4 verified at the producer and applied. B-1: `DiscardSessionPath`, every write-through `set` and every commit read the change file through `materializeMetaPaths`, and a change file holds `"default"` for an edit under an anonymous entry; stored keys are now read back unjudged, AC-44 and R-17 deleted, `TestChangeFileReadsBackUnjudged` added. I-1: the `parser_list.go` and `parser_freeform.go` value arms return a plain `p.errorf`; each now wraps its refusal in `syntaxRefusal`, with `TestEveryValueArmNamesTheFullPath`. I-2: `discard` takes a root path in every context; Q-9 rewritten around `set` alone. N-1: the `ensureFlexListEntry` row cites the `FlexNode` declaration |
| 21 | 2026-11-12 | 0 BLOCKER, 1 ISSUE, 1 NIT (narrow review of the round-20 edits) | Both verified at the producer and applied. I-1: `parseInactiveBracketMember` and the two `parser_list.go` list-key arms wrapped in `syntaxRefusal`, covered by `TestEveryValueArmNamesTheFullPath`; `invalidListKeyError` replaced, its `invalid peer name:` arm deleted, its pin added; AC-25 gains the file case and AC-39 the hierarchical key case; the `parser_freeform.go` `KeyType` check named exempt. N-1: the creator rule stated once, `ListAddress.Tokens == 0` is not judged |
| 22 | 2026-11-12 | 0 BLOCKER, 0 ISSUE, 1 NIT (short review of the round-21 points) | The NIT verified and applied: the `invalid peer name` pin in `internal/component/bgp/config/loader_test.go` asserts only under `if err != nil`, and `validatePeerName` (`internal/component/bgp/config/resolve.go`) writes the same words for a key that is an IP address, so the pin inventory now has the implementation first establish which producer fires, rewrite the pin only if it is the parser arm, and put the conditional assert to the `ze-test-judge`. The reviewer stated the design ready for owner approval with Q-9 open |
| amendment | 2026-11-12 | reviewed in round 23 | Q-9 answered by the owner. `top <command>` designed on the editor's existing per-command copy and position result and the one-command position change of `show <path>` and `load <path>`; one web helper; fix lines `top set <full path> ...` inside a context; AC-23 updated, AC-45 to AC-51 added |
| 23 | 2026-11-12 | 1 BLOCKER, 4 ISSUE, 2 NIT (review of the `top <command>` addition) | All 7 verified at the producer and applied. B-1: `cmdShow` answers through `configView` (`model_commands_show.go`), so rebuilding it would have shown the context under `top show`; the inner `configView` is now kept as built, and only `refreshConfig` resolves at the real position (`handleCommandResult`); R-18 rewritten. I-1: `handleEnter` takes `run`, `exit`, `quit`, `stop`, `restart` and `clear` before dispatch, and only `edit`, `up` and `top` set `clearContext`/`newContext` (`exit` does not); `handleEnter` now drops `top ` before its interceptions, since none reads the position, so every verb works behind `top` as without it; A-11 corrected, A-13 added, AC-47 and AC-48 gain `top run`, `top exit` and `top nosuchverb`, R-19 rewritten. I-2: `finishPasteMode` applies the paste at the real position later; the paste state records the position `load terminal` ran at, AC-52 added; the `commit confirmed` countdown and the exit confirmations read no position. I-3: `validateCore` holds no position and the CLI bar's `handleCLICommit` receives none; `ConfigValidationError` carries a typed fix framed by the SSH model, the web terminal and the CLI bar (now given the request's position); REST, gRPC and gNMI write the plain form. I-4: A-12 confirmed (only the three navigation handlers call `writeCLIResponse`); the bar's `top <command>` calls `dispatchCLICommand` with an empty position and no signal; today's `handleCLITop` drop of its arguments recorded as text 14. N-1: completion after `top ` uses `m.completer.Complete(tree, rest, nil)` (AC-49). N-2: no YANG node is named `top` (nearest `leaf top-n-sources`). Narrow round 24 is owed over these edits |
| 24 | 2026-11-12 | 0 BLOCKER, 2 ISSUE, 2 NIT (narrow review of the round-23 edits) | All 4 verified at the producer at `1d565709a2` and applied. I-1: 24 results write `configView: m.configViewAtPath(m.contextPath)`; only `cmdShow`'s is an asked-for view, the other 23 (option 2, edit 6, commit 8, session 4, load 2, commit window 1, the two `show changes` "No pending changes" arms among them) answer `refreshConfig`; proven invisible without `top` from `configViewAtPath`, `setViewportData` and both appliers (A-14, AC-54, `TestIncidentalViewUnchangedWithoutTop`); while verifying, `setViewportData` was found to pick the annotator from the real position, so `cmdTop` marks the inner view `atRoot`. I-2: one framer `cli.FrameFix(FixCommand, operator)`, the operator position saved by `cmdTop` as `m.topOrigin` and carried on the web as `cliCommand.Operator`; `CommitRefusal`, `appendIssueSummary`, `formatIssueList` and `EditorManager.runCommit` gain the parameter; the commit modal and `CommitNow` pass nil; AC-48 corrected, AC-53, `TestTopCommitFixUsesTopSetInsideAContext`, `TestTopRefusedSetFixKeepsTop`, `TestFrameFix` added. Found while verifying: `dispatchCommand` splits a pipe before its verb switch and `dispatchWithPipe` refuses any verb but `show`, so `top show \| errors` was refused, and the Integration Checklist said otherwise; `top` is now taken before the split (R-21, `TestTopPipedShowRunsAtTheRoot`). N-1: history keeps the typed line on every arm; the arms that record nothing today (bare `run`, `exit`, `quit`, `stop`, `restart`) stay unrecorded (`TestTopKeepsHistoryAsTyped`). N-2: recorded with a correction: no interception acts on the position, but `clear` reads it through `showConfigContent` to redraw the real position; the position movers are exactly `cmdEdit`, `cmdUp`, `cmdTop` by grep (A-11, A-13 confirmed). Narrow round 25 is owed over these edits |
| 25 | 2026-11-12 | 0 BLOCKER, 1 ISSUE, 2 NIT (narrow review of the round-24 edits) | All 3 verified at the producer and applied. I-1: `setViewportData` takes the tree diff only when the real `m.contextPath` is empty, so `show <path>` already gets the wrong annotator; the `atRoot` mark is replaced by `configViewAtPath` recording its path in `viewportData`, read by `setViewportData`; `cmdTop` marks nothing; the `show <path>` mismatch is fixed (`TestShowPathAnnotatesAtItsOwnPath`). N-1: `topOrigin` plus `underTop` kept, the reason stated; the commit modal and `CommitNow` hold no CLI position, so a fix typed into the CLI bar inside a context needs `top `. N-2: the web terminal splits a pipe inside `executeTerminalNav`, after `executeTerminalConfigMode` takes `top`; the CLI bar has no pipe handling (`parseCLICommand` only tokenizes). No further round owed; the design is ready for owner approval |
| owner approval | 2026-11-12 | approved | The owner answered "approve" after round 25; Status set to `ready` |

### Implementation source review evidence

The `independent_review` context performed the first three source rounds.
The independent closure context performed rounds 4 and 5; `closure_round6`
performed the separately authorized sixth pass. The table preserves
each historical verdict and the current disposition of its findings; it is not
the final closure artifact or permission to start another source pass.

| Round | Scope | Result | Disposition |
|-------|-------|--------|-------------|
| 1 | complete scoped implementation diff and manifest, including candidate parsing/publication, refusal framing, pipeline parsing, web guards, markup and PID ownership | 4 ISSUEs | composite required addresses, literal filter-pipe provenance, immutable command ownership and stale formatter references repaired |
| 2 | round-1 repairs, sibling entry points, new unit replay/ownership controls and editor/browser drafts | 1 ISSUE | required paths reused display normalization and lost literal numeric suffixes; replaced with private raw-identity helpers at both required boundaries |
| 3 | sole round-2 suffix repair, direct Path/SetHint and raw-hint replay controls, CLI/web enum vectors and related documentation/ownership metadata | 0 BLOCKER, 0 ISSUE | the scoped source repair was clean at this checkpoint. Subsequent package, editor/API/offline/reload, eight native-overlay and completed browser load/live results are recorded above; round 6 later closed the source gate; native record/check passed |
| 4 | independent closure deliverables, generic operational correction replay, actual non-first-key entry points, path rendering, security, style, test preservation and documentation | 3 ISSUEs | operational invocation quoting, actual AC-42 entry-point framing/proof and diagnostic path-word quoting were repaired, with actual regression failures followed by passing current boundary tests. Round 5 then found a new sensitive-suffix defect in the early key-refusal repair |
| 5 | only the frozen round-4 operational scanner/writer and sibling consumers, non-first-key path/framing repair, three diagnostic path renderers, their entry-point regression tests and matching documentation | 1 ISSUE; review stopped at the cap | the new earliest `Model.cmdSet` refusal framed raw arguments. The repair now uses semantic masking and explicit placeholder metadata; all eight secret cases failed before it and passed afterward. The owner authorized round 6 on 2026-11-13; its source verdict below is CLEAN. The covering machine artifact and hash check now passed |

| 6 | early sensitive-suffix masking and sibling framing; CR quoter/readers and actual parser/model/operational boundary tests; browser cleanup context policy and bounded-close tests; always-in-scope findings | 0 BLOCKER, 0 ISSUE | Separate correctness/boundary and security/test-preservation producer lenses, explicit Go style pass and spec audit found no further source defect. Native record/check passed; closure commits remain owed |

Eleven independent native test approvals are recorded. The original five cover reload, lifecycle,
`TestParsePipeFiltersRollback`, `TestModelCmdTop` and
`TestTopSavesTheOperatorPositionOnTheCopy`. Six later approvals cover operational
quote writing, command placement, MCP placement, web admin quote handling,
defined tokenizer escapes and the stored-key problem display. All original
inputs and assertions are retained except the independently justified obsolete
expectations described in the judgments. The independently judged browser,
markup and golden corrections retain their required final commit trailers.

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
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
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
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it: all audit, goal, assumption, documentation, ownership and Review Gate evidence is populated; commits A/B remain pending
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`: observed CLEAN at round 6; the artifact and 110-code-file hash check are recorded below
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** the closing edits (spec, any remaining docs, journal rows owed); the implementation has already landed phase by phase
- [ ] **Commit B:** `remove plan/release/fixes/spec-config-enum-refusal-proposes-valid-values.md` only, in the same `./le commit create` script (commit A preserves the spec in history)

## Disposition

| Field | Value |
|-------|-------|
| Date | 2026-11-12 |
| Checked at | e660eff82e |
| Verdict | REWORK (design reworked; review rounds 18 to 25 applied; Q-9 answered and designed; no further review round owed; owner approval given 2026-11-12) |
| Evidence 1 | 3e94cb7c40 (spec-operator-errors-name-the-fix, closed dc5b80f1b6) made every operator refusal an `opererr.Error` with a `fix: <command>` last line; `ai/rules/user-facing-errors.md` requires it |
| Evidence 2 | 238bdb62e9 added the union arm to `ValidateLeafValue` (`validateUnionValue`), contradicting the old AC-10, AC-13, AC-14, AC-23 and R-4 |
| Evidence 3 | 3e94cb7c40 added `config.LineError`/`SplitLine` (deleting `withoutLinePrefix`), `ConstraintMessages`, and `ArgumentFaults` (with 3af4462e82, 37c3a2abb1); `syntaxRefusal` and `prependSyntaxPath` exist |
| Evidence 4 | the reload prefix is `reload: parse config: line N:` (`test/reload/reload-parse-error-single-prefix.ci`) |
| Evidence 5 | 163d9a07c8, 98394e78ce, d2f0e2cb08: multi-leaf-key addressing, one address reader `ParseListAddress`, one creator `EnsureListEntry` |
| Evidence 6 | still open at HEAD: texts 1 to 14, the `; ` joins, no web set check, the completer heuristic, the ranged decimal64 defect |
| Evidence 7 | fa74dc4c00 removed `enum redistribute` from BMP `route-action`; the two BMP `.ci` pin only `route-action`, `redistribute`, `monitor` |

The goal still holds: every refusal in the Current Behavior table reads at HEAD
as listed, so the defect stands at its producers. The 2026-10-14 design no
longer fit main. It built a refusal type and frames beside the operator error
that now governs every refusal, it rebuilt a line error and a path stack that
exist, and it rested several acceptance criteria on the parser accepting union
non-members, which it no longer does. The rework rebuilds the design on those
mechanisms, and the findings of rounds 18 to 25 are applied. The owner answered
Q-9 and `top <command>` is designed in; round 25 closed the review with no
further round owed, and the owner approved the design on 2026-11-12.

## Implementation Summary

### What Was Implemented

The shared YANG refusal carries its accepted values and help, bounds or format,
typed path, masking and final correction through file parsing, offline writes,
editor validation, web rendering and the three config transports. Session
candidate decoding preserves refused leaf values for repair while ordinary
loading and publication keep their validation boundaries.

The editor runs `top <command>` at the root without moving the prompt unless
the inner command navigates. Typed corrections retain their operator position,
literal word boundaries and placeholder metadata. The incidental-view controls
cover all 26 recorded refresh sites. Operational arguments use the same
quote-aware scan at their pipeline and dispatcher boundaries.

The test prerequisites extend existing mechanisms: ordered API candidate setup,
leased REST and pinned TLS/token clients, atomic daemon PID publication, and
browser cleanup that retains the existing parallel command budget. These have
observed scoped tests and completed browser load/live proof. Final owned RFC
freshness and final native documentation checks are observed with their
separately owned global failures disclosed. Round 6 is source CLEAN. The covering native Review Gate record/check
passed over 152 historical paths, including 110 code files. Both closure
commits remain owed; this summary claims no completed closure.

### Bugs Found/Fixed

| Defect | Producing boundary | Observed proof |
|--------|--------------------|----------------|
| A refused candidate could become unavailable for repair | `ParseCandidateChangeFile`, `Editor.readChangeFile`, `Editor.SaveDraft` | candidate corruption, repair, member deletion and bare-publication controls; promoted editor and API workflows |
| Printed fixes lost composite keys, literal suffixes or quoted operands | `checkListRequired`, `FrameFix`, `ParseCommandWords`, operational invocation writer/scan | actual pre-fix failures, exact printed-fix replay controls and current eight-package boundary race run |
| A later key refusal lost its full path and remaining command | `ParseListAddress`, `WalkListEntryPath`, `Model.cmdSet` | actual second/third-key file/session root/context failures, then passing entry-point controls |
| The new early key-refusal arm exposed a sensitive suffix | `Model.cmdSet` before value splitting | all eight `TestModelNonFirstKeyRefusalMasksFollowingSecret` cases failed before masking and passed after it; round 6 source review is CLEAN |
| Accepted path words could add diagnostic lines | `refusalFacts.problem`, `InEntryKeyLeafRefusal`, `validateWithYANG` | actual parser/editor enum, structural-key and required-warning controls, including control characters and printed-fix replay |
| Browser cleanup discarded the parallel budget policy with cancellation | `Browser.Close` | both new policy tests failed before repair; full web/testing race and real observer SDK controls passed |

### Documentation Updates

The existing schema, syntax, editor, API, web, error-surface and test-runner
pages describe the implemented boundaries. The operational grammar and
completion pages describe the deliberate quoted escapes. Source anchors were
corrected for the moved word scanner and union validator. Final native
doc-index `ae12aa2a` passed with no owned tracked map diff. Fresh `job-enum-round6-doc-check-f119ecea.log`
reports seven unrelated anchors: sysrib `ApplyConfiguration` and
`recomputeBest`, plus five old report `max*Len` symbols. Owned
syntax/web anchors are fixed. Final owned RFC freshness is recorded separately.
Configuration stays `experimental` with `partial` scope; no maturity upgrade
is claimed.

### Risks and observations

| Recorded risk | Current evidence and limit |
|---------------|----------------------------|
| R-1, complete value lists | Values remain in declaration order without a display cap, as the owner required |
| R-3, sensitive data | Parser, walk, JSON, editor and web controls retain masking; the new early-key leak has actual RED/GREEN evidence but still owes independent post-fix review |
| R-10, proof freshness | All nineteen owned producer observations were recorded; final RFC 7950/8907 freshness after the audit reseal is observed. The global check's 99 violations concern other stems |
| R-15, stored identities | Typed keys are judged; stored anonymous/duplicate/change-file identities retain their readback controls. The separate serializer findings are not claimed fixed |
| R-18 to R-21, position and views | Top navigation, interceptions, paste, saved operator position, requested views and the 26 incidental refresh sites have direct/functional controls and discriminating overlays |
| Scoped health and browser load | Separate root goldens and foreign corpus failures remain disclosed. The original backup timeout has unchanged isolated/live passes. A partial or failed browser campaign never counts as the required 80-run proof |

### Deviations from Plan

| Change from the plan | Reason and preserved obligation |
|----------------------|---------------------------------|
| Actual refusal tests live in dedicated source files instead of the initially named general test files | The TDD table names each actual file and test; the required entry points and assertions remain |
| The claimed `enum-refusal-multi-leaf-key.ci` is replaced by independently judged real entry-point tests | That file never existed. Shipped BFD later keys are unrestricted strings and IPC is not config; locally resolved constrained YANG drives actual hierarchical/context/set parsers and editor dispatch without a fake product module or global registration |
| The local required-warning probe uses public `ValidateWithYANG` and actual printed-fix dispatch | Commit verification's separate global content parser cannot resolve a module deliberately local to this test. The probe states its real boundary; existing BGP commit/verification controls remain |
| Shared word quoting, operational scanning and explicit placeholder metadata replace reconstruction from decoded strings | Literal data must remain data through exact correction replay and pipe handling |
| Candidate repair, PID publication and cleanup policy received source repairs | Their defects blocked the agreed end-to-end proofs; no assertion, authored timeout or coverage was removed to clear them |
| The browser load campaign explicitly requests `minutes 60` | The controller otherwise stops at its 20-minute default before 80 invocations. This is campaign allowance, not a case or per-invocation timeout change; only a completed fresh 80-run campaign counts |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The original handover listed thirteen stale producer records | Final `rfc check` found six additional records for `validateString` and `parseTreeWithYANG` | Native RFC freshness check | All six clean/producer-halt pairs were observed and recorded; final owned freshness after the closure fixes and audit reseal is observed |
| approach | Operational concrete enum corrections were written as unquoted syntax | Definitions admit spaces, literal pipes and quoted data; the printed correction must preserve one admitted value through the normal pipeline and dispatcher | Independent closure read of `NewEnumArg`, `enumCorrection`, `wholeCommand` and `scanCommand` | The repair directly replays the printed correction; current scoped tests passed. Round 5 found a separate sensitive-suffix issue, round 6 then verified that repair as CLEAN; native record/check passed |
| approach | The plan claimed a committed multi-key functional file that does not exist | BFD's non-first configuration keys are unrestricted strings, and the other multi-key module is IPC; the existing helper/set-parser test did not drive hierarchical parsing or the editor | File existence check, shipped key census and `TestParseListAddressPairRefusal` | Add constrained instance-local YANG entry-point proofs; independently judge the replacement before changing the fictional functional claim |
| approach | Refusal problems joined accepted path words as raw display text | An accepted string key can contain a newline, tab or quote; display must escape those bytes while typed paths and corrections retain the identity | Closure security read of `refusalFacts.problem`, `InEntryKeyLeafRefusal` and the required-warning loop in `validateWithYANG` | Actual parser and editor controls cover all three producers before the shared quoting repair; no URL or position-carrier rewrite is claimed |
| approach | The non-first-key repair framed raw arguments before the usual sensitive-value defense | An early refusal still holds a following semantic operand and must mask it before quoting the full correction | Round-5 source finding, then eight actual failing secret cases | Source masking and focused tests passed; round 5 stopped at its cap; the owner then authorized round 6, whose source verdict is CLEAN |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| One schema-derived enum/union/range/length/pattern refusal | Observed | `yang/refusal.go`, `leaf_value.go`, `suggest/valuelist.go` | declaration order/help, kind markers, bounds/format and constraint messages have parser/walk/argument controls |
| Every named config and command entry point | Observed; round 6 source review is CLEAN; native record/check passed | parser/offline/editor/web/API handlers and operational argument routes | real transport, editor/offline/reload and all three completed browser load/live workflows passed |
| Exact correction, full identity and secret masking | Observed; round 6 source review is CLEAN | `FrameFix`, typed key framing, invocation writer/scan, required paths | actual literal/composite/suffix/control-character replay and secret regressions passed |
| Contextual fixes and transient `top <command>` | Observed | `cmdTop`, result appliers and web dispatchers | root/context, navigation/interceptions, paste, history and incidental views have direct and editor-workflow controls |
| Complete spec closure and final commits | Pending | Review Gate and Pre-Commit Verification below | no CLEAN artifact; no closure commits yet |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Observed | `enum-refusal-proposes-valid-values.ci`; parser/full-path tests | retained earlier phase proof |
| AC-2 | Observed | same file plus `TestEnumRefusalSuggestion` | closest accepted value and final file correction |
| AC-3 | Observed | both BMP refusal pins and no-suggestion controls | all accepted values retained; single-value correction is concrete |
| AC-4 | Observed | `TestValueListRender` | missing help keeps its bare line and alignment |
| AC-5 | Observed | inline/leaf-list parser tests and functional file | each refusing member retains its full path |
| AC-6 | Observed | promoted set/offline workflows and viewport controls | full-body, context and exit/state checks |
| AC-7 | Observed | handler controls and final browser workflow | baseline, fresh completed 80-run campaign and promoted live case passed |
| AC-8 | Observed | real leased REST workflow, hook tests and native hook overlay | single/multiple refusals; unchanged running config |
| AC-9 | Observed | real TLS/token gRPC and gNMI workflows | expected transport statuses and complete correction |
| AC-10 | Observed | start/reload workflows and single-prefix regression | promoted reload passed 80 and ordinary live checks |
| AC-11 | Observed | parser diagnostic JSON controls | codes, actual/expected/path/line and masked actual omission |
| AC-12 | Observed; round 6 source review is CLEAN | argument route controls and `TestOperationalEnumFixReplaysLiteralValue` | current checked-pipeline/dispatcher literal replay passed; round 6 source review is CLEAN; native record/check passed |
| AC-13 | Observed | union parser, completer, offline and web controls | parser verdict preserved; enum help and other member types retained |
| AC-14 | Observed with health disclosures | direct prefix/hex acceptance and retained parser/creator controls | scoped race proofs passed; separate root goldens, foreign LDP fixture and original backup timeout remain disclosed |
| AC-16 | Observed; round 6 source review is CLEAN | sensitive parser/walk/JSON controls and eight non-first-key secret cases | actual sensitive-suffix RED then GREEN; round 6 source review is CLEAN |
| AC-17 | Observed | actual section walk, `Validator.Validate` and typed-chain controls | full config/YANG evidence and direct secret rendering controls |
| AC-18 | Observed | missing enum parser/functional cases | needs-value body and final correction |
| AC-19 | Observed | promoted relative-load editor workflow | context/full path, snippet correction and unchanged candidate |
| AC-20 | Observed | `set-cli-format.et`, CLI format and prompt-row controls | complete viewport refusal; sorted completion retained |
| AC-21 | Observed | parser/set/completer/section/content copy-chain tests | both typed refusal and operator error remain recoverable |
| AC-22 | Observed | update-origin unit/functional controls | text/cursor invocation path and correction retained |
| AC-23 | Observed | promoted commit workflow, candidate repair and confirmed-session controls | both modes, inactive retention, same-context correction, status/viewport contract |
| AC-24 | Observed | set/set-meta/nop functional and diagnostic cases | own verb and complete set-file correction retained |
| AC-25 | Observed | real key/set workflow and creator controls | refused key creates nothing; suffix retained |
| AC-26 | Observed | `TestRenameCopyRefuseTheDestinationKey` | unchanged state and exact verb retained |
| AC-27 | Observed | terminal workflow and typed key controls | final baseline, all 80 invocations and promoted live case passed |
| AC-28 | Observed | promoted offline-set workflow | exit 2, stderr placement and unchanged file |
| AC-29 | Observed | `TestKeyCheckOnInsertNestedActivation` | insert key refusal and unchanged state |
| AC-30 | Observed | same entry-point test and nested key controls | ancestor refusal leaves no partial entry |
| AC-31 | Observed | same test and activation controls | both activation verbs retain state and correction |
| AC-32 | Observed | real offline prefix keys and editor/web/lookup controls | IPv4/IPv6 accepted; stored anonymous/duplicate entries remain reachable |
| AC-33 | Observed | key-leaf/in-entry handler controls and terminal workflow | keyed-entry behavior, completed 80-run campaign and promoted live case passed |
| AC-34 | Observed | session in-entry refusal and change-file controls | no second key leaf or corrupt pending file |
| AC-35 | Observed | range parser/editor/JSON/walk controls and browser workflow | declared bounds, completed 80-run campaign and promoted live case passed |
| AC-36 | Observed | empty/oversized length and secret controls; browser workflow | accepted value retained; completed 80-run campaign and promoted live case passed |
| AC-37 | Observed | pattern/format parser/editor/JSON controls; browser workflow | format and primitive-address controls; completed 80-run campaign and promoted live case passed |
| AC-38 | Observed | command length/range/pattern and multiple-fault controls | original reason assertions retained; scoped current command proofs passed |
| AC-39 | Observed | member-pattern/key restriction and typed path tests | member pattern and new-key bounds retain the kind's refusal; the separately owned leaf-list range/length propagation defect remains open |
| AC-40 | Observed | `TestTypeFormatReader` and fallback tests | typedef/inline fallback contains no regular expression |
| AC-41 | Observed | decimal64 boundary controls and functional file | in-range values accepted; range/type distinctions retained |
| AC-42 | Observed; round 6 source review is CLEAN | actual local-YANG parser and second/third-key editor replay tests | nonexistent functional-file claim corrected; no fake shipped constraint |
| AC-43 | Observed | all three real config transports and native hook walks | key refusal at set, complete suffix and no stored refused entry |
| AC-45 | Observed | `top-command.et`, model top/history controls | root execution retains actual prompt and history |
| AC-46 | Observed | `TestTopShowShowsTheRoot` | asked-for root view and its annotator retained |
| AC-47 | Observed | navigation/interception top controls and functional file | inner navigation and nonmoving interceptions preserve their contracts |
| AC-48 | Observed | refused top/model commands and pending-exit controls | inner refusal and actual operator position retained |
| AC-49 | Observed | `TestTopCompletesFromTheRoot` and help-table controls | root completion and optional command form |
| AC-50 | Observed | web top controls and terminal workflow | final baseline, all 80 invocations and promoted live case passed |
| AC-51 | Observed | BGP commit/required hints, generic required public validator and actual replay | local-module test states its true boundary; conflict discard fixes unchanged |
| AC-52 | Observed | top terminal-paste controls and navigation workflow | paste uses the recorded root position without moving the prompt |
| AC-53 | Observed | contextual top commit/errors and web commit controls | final fixes use the saved operator position |
| AC-54 | Observed | original/remaining refresh, restored-window and valid-verification controls | all 26 recorded sites; both native wrong-position overlays discriminated |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Shared refusal, format and declaration facts | Observed | `internal/core/suggest`, `internal/component/config/yang`, config schema/value tests | exact names and purpose remain in the TDD table; current full YANG run passed |
| Parser, stored-key, candidate and required identities | Observed | config parser/candidate/required tests | full config's old display expectation was independently corrected and its targeted rerun passed; no broad all-green claim replaces the recorded run |
| Editor key/value, commit, top, prompt and incidental views | Observed | CLI TDD files above | earlier full CLI and current reachable boundary selectors passed |
| Web forms, terminal/bar corrections and transport hooks | Observed | web, gNMI and hub TDD files | typed errors, masked suffixes, state and real transport evidence retained |
| Operational literal correction and invocation consumers | Observed; round 6 source review is CLEAN | command/server/CLI/MCP/admin tests | actual pre-fix RED; current focused eight-package race run passed |
| Native editor/API/offline/reload workflows | Observed | promoted `.et` and `.ci` files above | completed 80-run selections and ordinary promoted projections recorded |
| Three browser workflows | Observed | `test/web/config-*.wb` | final baseline, fresh completed 80-run campaign, cost review, promotion and all three owned live projections passed |
| PID and browser cleanup prerequisites | Observed; round 6 source review is CLEAN | runner PID tests and `web/testing/close_budget_policy_test.go` | actual negative controls, full scoped packages and real observer SDK checks passed |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Existing schema/parser/editor/web/argument producers | Observed in source and prior phase commits | exact paths and changes are listed under Files to Modify; no generated composition root or product YANG probe added |
| New refusal, codec, candidate and replay files | Observed | Files to Create now includes actual local-YANG entry-point proofs and operational scanner/consumer tests |
| API candidate and transport fixture files | Observed | existing runners/registrations extended; actual leased or pinned TLS endpoints exercised |
| Docs and news fragment | Observed | owned pages, news and source anchors updated; final native doc-index passed and fresh round-6 doc-check reports seven unrelated anchors and no owned error. Native Review Gate recording and hash check passed |
| Browser target files | Observed, promoted | two strengthened workflows retain all old assertions; the new literal-pipe workflow and both strengthened cases completed proof and ordinary live projection |

### Audit Summary

There are 52 active AC rows; AC-15 and AC-44 were explicitly withdrawn during
design. All have observed behavior evidence, including completed browser load/live proof.
They do not replace the independent source gate. Round 5 was not clean. The owner authorized round 6 on 2026-11-13, and its
source verdict is CLEAN. The covering native record/check passed with matching hashes for 110 code
files across 152 historical paths. Both closure commits remain owed.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| One refusal built from schema facts on every config entry point | parser/walk and real transport/editor/browser workflows | declared facts, typed chains, safe collection, repair and active-state controls passed; three API and three browser cases each completed their 80-run campaigns and promoted live projections |
| A correction an operator can type without losing context, identity or secrecy | actual printed-fix replay and negative controls | file/session/root/context corrections, composite and literal suffix keys, operational checked-pipeline/dispatcher arguments and the eight sensitive-suffix cases passed after observed RED. Round 6 source review is CLEAN |
| Transient root commands without unrelated editor changes | editor workflow and producer-discriminating controls | `top-command.et`, paste/history/navigation controls and all 26 incidental refresh sites; both wrong-position native overlays failed their intended assertions |
| Friendly declared range, length and pattern refusals retain acceptance boundaries | parser/editor/argument and browser controls | prefix/hex acceptance, numeric/decimal64 boundaries, empty length, format and primitive-type controls passed; the browser workflow completed baseline, 80-run and promoted live proof. The separately owned leaf-list range/length propagation gap is disclosed above |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Carrying declared range and length onto every leaf-list node | Existing enforcement gap recorded by the 2026-10-13 leaf-list-builder occurrence in `guard-added-to-one-half-of-a-pair`; it stays open. AC-5 adds enum facts, while AC-14 preserves accepted verdicts except its named changes; no leaf-list range/length repair is claimed | `plan/release/fixes/spec-web-cli-config-one-off-defects.md` AC-8, whose owned-journal table already names that occurrence |
| Operational `config false` projection and literal numeric-suffix set/positional/inline serializer defects | Separable accepted-input defects found during proof; the required-path and printed-refusal regressions repaired here do not claim those serializer repairs | `plan/release/fixes/spec-config-yang-accepted-input-correctness.md` |
| Emitted web edit-URL aliases | Canonical `/show/` readback proves this refusal's retained state; the independently recorded alias defect remains separate | `plan/release/fixes/spec-web-cli-config-one-off-defects.md` |
| Whole-file partial-edit gate population mismatch | Separate tooling defect encountered while repairing records; no gate bypass or repair is claimed here | `plan/post-release/engineering/spec-gates-share-one-semantic-population.md` |
| Full RFC 7950 lexical/reference-type coverage | The owned audit correction records the lexical claim as weak instead of claiming full support; the owner's post-release YANG scope remains in force | `plan/post-release/features/spec-config-yang-reference-types.md` |

These rows preserve separately owned findings. This spec's browser work has
completed its proof and live projection. Round 6 is source CLEAN and the covering native Review Gate record/check
passed. Commits A and B remain obligations of this spec.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/config-enum-refusal-proposes-valid-values-config-enum-refusal-20261112-codex-continue.md`; recorded CLEAN, rounds 6 |
| `./le spec review check` | `review_gate: OK (110 code files, clean, hashes match)` in `job-enum-parent-covering-review-check-valid-f1fca422.log` |
| Covering population | 152 historical paths across `b9c22dfb7d`, `0f8c7ccf04`, `024ba1d203` and `1ce3fb78c2`, plus final closure files; owned shared-file content read from committed blobs, excluding foreign WIP |
| Citation clearance | Main observed zero full parent-path citations outside this spec across the tracked tree; no citation edit is owed |
| Rounds | six: final round 6 reports 0 BLOCKER and 0 ISSUE over the scope recorded before the pass. The product reason beyond five is the round-5 early `Model.cmdSet` sensitive-suffix exposure; the owner authorization is "Autorized and continued." (2026-11-13) |
| Source verdict | CLEAN: 0 BLOCKER, 0 ISSUE; no product source changed in this review |
| Automated prechecks | Observed: `job-enum-round6-repo-check-89d513b0.log` reports 31 foreign findings; `job-enum-round6-weakened-check-36862edf.log` reports 13 unexplained IKE/sysrib weakenings outside this change. No owned source/test finding; these are disclosed global reds, not a global clean claim |
| Reviewer lenses used | deliverables and wiring; security and edge cases; style, documentation and test preservation |

### Run 6

The fresh `closure_round6` context read the early `Model.cmdSet` mask through
`DisplayValueAtPath`, `secretAtPath` and `Schema.LookupTokenPath`, then the
`FramedWordRefusal`, `correctionFix` and `FrameFix` producers and sibling paths.
The CR pass read `configtext.QuoteToken`, both config token readers, the shared
operational scanner/writer and actual parser/model/dispatcher replay tests.
The cleanup pass read `Browser.Close`, `runAgentCore`, `Bound`, ownership and
the actual successful-close and bounded-hang controls.

Correctness/boundary symmetry, security/test preservation and the explicit
`docs/contributing/ze-go-style.md` pass report 0 BLOCKER and 0 ISSUE. Loops are
input-bounded, cleanup has a fresh finite command deadline, and no new panic
is reachable. The spec audit retains all 52 active ACs, goals, 13 assumptions
and separately owned Work Not Done rows; no scope moved. Historical round 5
remains not clean. The two record NOTEs were fixed in one prose edit: the
editor guide now lists escaped CR, and the RFC evidence below names the full
final log rather than the ambiguous suffix alone.

### Record NOTES

The closure news gate found a fragment identity mismatch. `newsFragmentPath`
and `newsFragmentClosureProblem` require the canonical stem, without `spec-`,
in both filename and front matter. Commit A adds the correctly named fragment
with the same operator text, removes the old tracked copy through the generated
script, and corrects the fragments README at those producers. This is a record
correction; the 110-code-file CLEAN review and round count remain unchanged.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Composite required fixes, pipe provenance, command ownership and formatter references | round-1 producing boundaries listed above | source repairs and actual replay/ownership controls; round 3 later closed its scoped source review |
| 2 | ISSUE | Required identities lost literal numeric suffixes through display normalization | `storedListKeyTokens`, `checkListRequired` | private identity-preserving helpers; actual RED/GREEN and raw hint replay |
| 3 | ISSUE | Concrete operational enum corrections were emitted as syntax and cut by a quote-blind pipeline | invocation writer/scan and argument correction producers | actual checked-pipeline/dispatcher replay; scoped race tests passed; round 6 source review is CLEAN |
| 4 | ISSUE | AC-42 cited a nonexistent test and missed actual non-first-key editor framing | `ParseListAddress`, `WalkListEntryPath`, `Model.cmdSet` | independently judged local-YANG actual entry-point tests and full-address corrections; scoped race tests passed; round 6 source review is CLEAN |
| 5 | ISSUE | Accepted path words could add diagnostic lines | typed refusal, structural key-leaf and required-warning renderers | explicit display quoting with typed identity and actual correction replay retained; round 6 source review is CLEAN |
| 6 | ISSUE | The new earliest non-first-key refusal printed its sensitive suffix | `Model.cmdSet` | actual eight-case RED then masking/GREEN; round 5 remains historically not clean; the separately authorized round 6 is CLEAN |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| Shared refusal/format/value-list and typed fix files | Yes | prior phase commits plus the current owned file inventory; TDD table names actual canonical files |
| Candidate, key, identity, literal and local-YANG entry-point tests | Yes | current owned inventory and their named actual test runs; no nonexistent multi-key `.ci` is counted |
| API `.ci` files and registered transport clients | Yes | all three promoted real-transport workflows and their clean native overlay projections ran |
| Editor/offline/reload workflows | Yes | promoted files ran through their ordinary native projections |
| Three final browser workflows | Yes, promoted | all three canonical `test/web/config-*.wb` cases completed fresh load proof and ordinary live projection |
| News fragment | Yes | `website/changes/fragments/config-enum-refusal-proposes-valid-values.md`, with `spec: config-enum-refusal-proposes-valid-values`; commit A adds it and removes the old prefixed fragment |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 to AC-6, AC-10, AC-11, AC-18 to AC-21, AC-24 to AC-26, AC-28 to AC-32, AC-34, AC-39 to AC-41 | file/editor/offline/creator verdicts and correction boundaries | actual parser/walk and current boundary race controls; promoted editor/offline/reload projections and their completed load campaigns |
| AC-8, AC-9, AC-43 | real config transports retain refusal and active state | three leased or pinned TLS/token client workflows, each in every API load invocation; hook/transport overlays discriminated |
| AC-12 to AC-14, AC-16, AC-17, AC-22, AC-23, AC-38, AC-42, AC-51 | typed acceptance, operational replay, secret/candidate/required boundaries | current eight-package boundary race run passed; actual source RED/GREEN controls recorded above; round 6 source review is CLEAN; native record/check passed |
| AC-45 to AC-49, AC-52 to AC-54 | top dispatch, completion, paste, history and views | full earlier CLI result, current reachable controls and both wrong-position overlays; later codec changes have selected boundary coverage |
| AC-7, AC-27, AC-33, AC-35 to AC-37, AC-50 | browser delivery and retained state | final baseline, all 80 unique successful invocations, cost review, plain promotion and all three owned ordinary live cases passed |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Config validate/file start and reload | parser/start/reload files in Wiring Test | observed earlier native proofs; promoted reload and single-prefix regression also passed live |
| Offline config set | `test/parse/enum-refusal-config-set.ci` | process exit/stderr/state and prefix-key controls; 80-run and promoted live projection passed |
| SSH editor | promoted `.et` files in Wiring Test | actual keystrokes/viewport/candidate and correction replay; completed load and live selections |
| Non-first typed key absent from shipped configuration schema | actual local-YANG parser/model tests above | actual supported entry points observed, not a fictitious product `.ci`; independent substitution judgment recorded |
| REST/gRPC/gNMI config sessions | three promoted `test/plugin/*enum-refusal.ci` files | actual listeners, tokens/TLS where configured, candidate hook and unchanged active state |
| Operational arguments and update origin | existing command-argument/update-origin functional files | retained daemon-path proofs plus current literal correction checked-pipeline/dispatcher tests |
| Browser | three promoted `test/web/config-*.wb` files | final baseline, completed 80-run campaign and all three owned live projections passed |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | bounded `suggest.Correction` controls retain close suggestions and reject unrelated candidates |
| A-2 | confirmed | native help-shape census observed 1033 values and 1033 summaries; no-help rendering separately tested |
| A-3 | confirmed | actual REST/gRPC/gNMI key-set and value-commit workflows, including status and retained state |
| A-5 | confirmed | viewport/prompt, parser, offline, API and completed browser load/live controls; native viewport overlay |
| A-6 | confirmed | explicitly registered real transport fixtures, leased/pinned endpoints and complete API load campaign |
| A-7 | confirmed | owner Q-3 includes operational arguments |
| A-9 | confirmed | recorded creator census and typed/stored/anonymous-entry positive and negative controls |
| A-10 | confirmed at the documented routes | argument-fault route controls and current exact invocation replay; routes without a known head retain their generic contract |
| A-11 | confirmed | setter census and top navigation/nonmoving-command controls |
| A-12 | confirmed | recorded web position-writer census; only navigation changes the page's position |
| A-13 | confirmed | interception source and corresponding top controls; original input history retained |
| A-14 | confirmed | 26-site inventory, original/remaining/restored-window comparisons and both discriminating native overlays |
| A-15 | confirmed at observed boundaries; round 6 source review is CLEAN | shared typed framer, actual root/context and required/member/key replay, completed browser campaign and promoted live cases |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Feature declaration | `features/configuration.md`; native feature report retained `experimental` / `configuration` / `daemon` / `experimental`, with scope `partial` | observed; no maturity or scope upgrade |
| Schema, syntax and error surface | `yang/refusal.go`, `leaf_value.go`, tokenizer/quoter and typed editor issue producers | relevant pages updated and owned source anchors corrected; final native docs results recorded below. Round 6 source review is CLEAN; native record/check passed |
| Editor and web usage | model/web dispatch, typed framing, candidate and required/member corrections | guide examples, `top` behavior, masking and actual canonical readback documented |
| API and RPC usage | actual session hook, transport status mapping and registered real clients | API pages describe retained candidate and unchanged running-state behavior |
| Test infrastructure | API candidate fixture, PID publication and browser cleanup policy | existing runner pages updated; scoped runtime proof includes the completed browser campaign and all three owned promoted live cases; round 6 source review is CLEAN; native record/check passed |
| Operational grammar and completion | shared invocation scan/writer and normal checked pipeline | deliberate quoted escapes documented; obsolete quote-refusal tests independently judged |
| Plugin SDK, wire, metadata, counters and comparison | no product registration, SDK/protocol, wire encoding, route metadata or counter change in the owned scope | no new product dependency or comparison/maturity claim |
| RFC records | thirteen original observations plus six additional clean/producer-halt pairs; RFC 8907 audit resealed for unchanged accounting units | final `job-enum-final-closure-rfc-freshness-4fea72ad.log` reports no owned RFC 7950/8907 stale/shifted record errors. Global other-stem violations and honest RFC 7950 partial/weak disclosures remain |
| Native checks | final doc-index `ae12aa2a` passed with no owned tracked map diff; fresh `job-enum-round6-doc-check-f119ecea.log` reports two foreign sysrib and five old report `max*Len` anchors, with no owned guide/source error | observed with separately owned global failures disclosed; help census remains 1033 enum values / 1033 summaries. No blanket green claim |
