# Spec: config-enum-refusal-proposes-valid-values

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P1 |
| Release reason | Honor the approved consistent schema-derived enum refusal across operator entry points. |
| Scope | config |
| Depends | - |
| Phase | 2/6 |
| Handoff | - |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

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
| `internal/component/cli/model_commands_edit_secret_test.go` `TestSSHCLISetNeverEchoesASecret`; `internal/component/config/cli/cmd_set_secret_test.go` `TestConfigSetRefusalNeverEchoesASecret` | `invalid value` (needle moves to `is not in the accepted format`; placeholder assertions unchanged) |
| `test/parse/asn4-refuses-boolean-spelling.ci`, `test/parse/capability-leaf-refuses-presence-spelling.ci`, `test/parse/ipsec-application-override.ci` (`expected one of`, `deny, allow`: the comma-joined list becomes one value per line, so the needle becomes each value line), `test/isis/isis-auth.ci`, `test/ospf/ospf-config.ci`, `test/parse/image-server-invalid-rescue-auth.ci`, `test/parse/redistribute-tag-filter.ci`, `test/web/web-error-fragment.wb`, `test/editor/pipe/set-cli-format.et`, `test/ui/cli-argument-length-refused.ci`, `test/ui/cli-argument-refused-plugin-route.ci`, `test/ipsec/ipsec-dataplane-show.ci`, `test/plugin/session-editor-commit-verify.ci` | the old words |
| `test/plugin/policy-test-errors.ci` | NEGATIVE `reject=... invalid value` |
| `test/parse/bmp-route-action-refuses-redistribute.ci`, `test/reload/bmp-route-action-redistribute-refused.ci` | `route-action`, `redistribute`, `monitor` (and `reload error`): the new body keeps all three; strengthened with the marker `is not an accepted value` and the `fix:` line |

Stay green unchanged: the RFC-tagged enum tests (`rfc7950_radius_auth_method_enum_test.go`, `rfc7950_isis_auth_algorithm_enum_test.go`, `rfc7950_validator_yang_test.go` `TestValidateTree_EnumViolation`, `TestValidateTree_FamilyModeEnum`, `TestValidateTree_AddPathDirectionEnum`), `radius_auth_method_eap_enum_test.go`; the pins of the kinds that keep today's text, `internal/component/config/yang/validator_test.go` `TestValidationErrorTypeContract` (`unknown error at test/value: unspecified`) and `internal/component/config/yang/rfc7950_reference_walk_test.go` (`HasPrefix "reference error at sys/mgmt: "`); `TestSectionValidationErrorRedactsASensitiveLeaf`, `TestCLIFormatNamesMatchTheSchemaLeaf`, `TestUpdateText_OriginValidation_YANG`, `TestSetCommandRejectsInvalidValue` and `TestInsertValidatesValueType` (type refusals keep their text), `TestEditorCommitConfirmedBlocksOnPeerPipelineRefusal` (renamed from `...OnMissingLeakFilter` by 74c830e0d2), `test/plugin/session-editor-commit-force.ci`, `test/parse/firewall-priority-out-of-range.ci`; every existing test of `top` alone, which keeps its meaning.

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
| The parser's refusal path | `internal/component/config/parser_diagnostics.go` `syntaxRefusal`, `prependSyntaxPath`; `parser_list.go` key prepends; `ParseAt` context | On unwind each caller prepends the name or list key it owns, keys and context included, at no cost on success, for a refusal wrapped in `syntaxRefusal` | extends: every value and key arm of the hierarchical parser wraps its refusal in `syntaxRefusal` (not only `parseLeaf`); `syntaxRefusal` keeps the segments as a list beside its string, and at the parse root (`Parse`, `ParseAt`) a refusal whose chain holds a `ValidationError` is rebuilt once: `newLineError(line, ve.WithPath(segments))` replaces the `syntaxRefusal`, so the problem carries the space-separated path and no slash prefix is printed; the 2026-10-14 path stack and its `Parser.path` extension are dropped |
| The union check | `internal/component/config/schema.go` `validateUnionValue`, `LeafNode.Union` | Every parser refuses a union value no member accepts | extends: its refusal becomes the friendly one, the enum members listed with help and the other members named in words |
| YANG constraint messages | `internal/component/config/yang/constraint_message.go` `ConstraintMessages`, `WithConstraintMessage` | A declared `error-message` speaks on range, length, pattern, joined after the refusal | extends: a declared message is the why line; `WithConstraintMessage`'s inline join is replaced by that placement |
| The list entry creator | `internal/component/config/list_key.go` `EnsureListEntry`, `ListEntryDestination` | The one creator every addressing surface calls; the rename and copy destination | extends: on creating an entry it judges the first key value with `ValidateListKey` unless the address has `Tokens == 0`, and strips no `#N` suffix; `ListEntryDestination` judges the destination key. This replaces the 2026-10-14 `Schema.ValidateTokenPathKeys`, `TokenPathSegments` and `CheckListKey`. `ParseListAddress` stays a reader and judges no first key |
| The argument-fault refusal | `internal/component/command/argfaults.go` `ArgumentFaults`, `RestatedForLine`, `wholeCommand`; `argbind.go` `ValidateArgs` | All faults of a line; whole-command fix when every fault is missing; one restating function on every route | extends: a typed value fault joins the missing-argument fault as a fault with a known correction; a line with one value fault is an `ArgumentFaults` of one, so every route restates it; this replaces the 2026-10-14 command-words parameter and `EnsureStep` field |
| The closest-match helper | `internal/core/suggest/suggest.go` `Command`; `internal/component/config/retired.go` `UnknownKeywordHint` | Edit distance, ` (did you mean X?)` | extends: the same helper and wording |
| The per-value help reader | `internal/component/config/yang/enum.go` `EnumValueSummaries`, `EnumNamesDeclared` | The one reader of a value's `ze:help` and of declared order | extends: `yangToLeaf`, the leaf-list builders and the `ArgDef` builder store the help |
| The SSH viewport refusal route | `internal/component/cli/model_load.go` (`Commit blocked by conflicts:`) | A long refusal shown in the viewport, the row keeps one line | extends: every multi-line `m.err` takes it |
| The editor's per-command copy and position result | `internal/component/cli/model_commands.go` `executeCommand` (value receiver), `dispatchCommand`; `model.go` `handleCommandResult`, `applyResult` (`clearContext`, `newContext`, `refreshConfig`) | Each dispatched command runs on a copy of the model; only the result's `clearContext` and `newContext` move the real position; `refreshConfig` is resolved at the real position | extends: `top <command>` dispatches the rest of the line on that copy with the position emptied; the real position moves only when the inner result sets `clearContext` or `newContext`; nothing is restored by hand. The 23 results that redraw the current position as a side effect switch from a copy-built `configView` to `refreshConfig`, so the real position draws them (Data Flow 7) |
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
   - The views a command answers. 24 results write `configView: m.configViewAtPath(m.contextPath)`. One is a view the verb was asked for and keeps `configView`; the other 23 redraw the current position as a side effect of a change or a message and answer `refreshConfig: true` instead, so `handleCommandResult` and `applyResult` draw them at the real position:

     | Set | Count | Sites (function) |
     |-----|-------|------------------|
     | asked-for view, keeps `configView` | 1 | `model_commands_show.go` `cmdShow` (default working view, no columns) |
     | incidental, becomes `refreshConfig` | 2 | `model_commands_option.go` `cmdOptionErrors` (`hints`, `hide`) |
     | incidental | 6 | `model_commands_edit.go` `cmdSet`, `createListEntry`, `cmdDelete`, `runActivation` (`activate`, `deactivate`), `cmdInsert`, `listEntryMove` (`rename`, `copy`) |
     | incidental | 8 | `model_commands_commit.go` `cmdRollback`, `cmdCommitVerify`, `commitValidationRefusal`, the `commit failed:` reload arms of `commitCandidateAndReload` and `runCommitSession`, `cmdDiscardSession`, `cmdDiscardPath`, `cmdDiscard` |
     | incidental | 4 | `model_commands_session.go` `cmdShowChanges` and `cmdShowChangesAll` (`No pending changes`), `cmdShowChangesAll` (the count), `cmdDisconnectSession` |
     | incidental | 2 | `model_load.go` `rollbackConfirmed`, `applyLoad` |
     | incidental | 1 | `model_commit_window.go` `windowReverted` |

     The two `show changes` sites are `show` verbs whose answer is the status line; the view under it is the current position's config, not what was asked for, so they are incidental. The other views `cmdShow` and `cmdShowPipe` build (annotated, alternate source, filtered) are asked-for views and keep `configView`; `cmdEdit` and `cmdUp` build the view at the new position with `newContext` and are unchanged.
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
| A-2 | Every enum value declares `ze:help` | `./le doc yang-contract help-shape`, 2026-10-13: 857 of 857 | a value prints with no help | AC-4 test schema; the gate's counts | unvalidated |
| A-3 | REST, gRPC and gNMI meet a value refusal only at commit; a key refusal at set | the session set calls `SetValue` with no value check; `SetValue` creates through `EnsureListEntry` | a set-time value refusal elsewhere bypasses the frame | AC-8, AC-9, AC-43 | unvalidated |
| A-5 | A multi-line operator error renders on every surface that prints one | `ArgumentFaults` and the `}` refusal already reach these surfaces multi-line | lines run together, or the SSH prompt is lost | AC-1, AC-6, AC-20, AC-23, `TestEditorMultiLineErrorKeepsThePromptRow` | unvalidated |
| A-6 | No `.ci` drives a gNMI `Set`, a gRPC config session or a REST enum commit, so this spec builds the clients | 2026-10-13 grep; the REST fixture `plugin/api-config-commit-reject` is the shape | the clients do not reach the session manager | the three fixture clients | unvalidated |
| A-7 | "Friendly errors everywhere" covers command arguments | owner answer Q-3 "yes" | - | owner | confirmed |
| A-9 | The creators of list entries are the ones the table below lists, each with its verdict | `grep -rn '\.AddListEntry('` over non-test Go at `e660eff82e` | a creator keeps an unchecked key | the same grep at implementation is the gate: a creator not in the table refuses the phase until it has a verdict | unvalidated |
| A-10 | Every route that refuses a command line restates through `RestatedForLine` | `restateArgumentFaults` (dispatcher), `ValidateModelArgs` (`local_data.go`) | a route prints a value fault with no whole-command fix | AC-12 one-fault case on the dispatcher and a model route | unvalidated |
| A-11 | Only `edit`, `up` and `top` set `clearContext` or `newContext` | a grep of non-test Go in `internal/component/cli` at `1d565709a2` for both fields finds setters only in `cmdTop`, `cmdUp` and `cmdEdit`; the other writes of `m.contextPath` are the two appliers and the restoring `cmdShow`/`cmdShowPipe` | another command that moves the position keeps its move under `top`, which is the rule's intent | the grep; `TestTopKeepsPositionUnlessInnerMoves` over every `dispatchCommand` verb | confirmed (grep), test owed |
| A-12 | On the CLI bar only the navigation handlers change the page's position | `writeCLIResponse` is called only by `handleCLIEdit`, `handleCLITop` and `handleCLIUp` (`internal/component/web/cli.go`, `internal/component/web/cli_terminal.go`) | - | read at HEAD | confirmed |
| A-13 | The `handleEnter` interceptions are `run`, bare `run`, `exit`, `quit`, `stop`, `restart` and `clear`, and none acts on the position | `handleEnter` read at `1d565709a2`: no arm reads `m.contextPath`; `clear` reads it only through `showConfigContent`, an incidental redraw at the real position | a verb behind `top` misbehaves | `TestTopInterceptedVerbsBehaveAsWithout` | confirmed (read), test owed |
| A-14 | The 23 incidental views render the same without `top` once they answer `refreshConfig` | `configViewAtPath` reads only `m.editor` and its path; both appliers resolve `refreshConfig` at the real position after the same steps; marks, hints and gutter come from the real model in both forms (Data Flow 7) | an operator sees a different view after `set`, `delete` or `commit` with no `top` | `TestIncidentalViewUnchangedWithoutTop` | unvalidated |
| A-15 | Every editor-command fix line is written through `FrameFix` | the framers named in Data Flow 5 are every writer of an editor-command fix at `1d565709a2` | a fix printed behind `top` is wrong as typed | a grep for `fix:` writers and `WithFix` callers in `internal/component/cli` and `internal/component/web` at implementation; `TestFrameFix`, `TestTopCommitFixUsesTopSetInsideAContext` | unvalidated |

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
| R-18 | `top <command>` shows a view at a position other than the one the operator asked for or the prompt names | `TestTopShowShowsTheRoot`, `TestTopSetRefreshesAtThePosition` or `TestIncidentalViewUnchangedWithoutTop` red | only `cmdShow`'s asked-for views keep `configView`; the 23 incidental sites answer `refreshConfig`, resolved at the real position, so `top set`, `top delete`, `top commit` and the rest redraw the context the prompt names; the switch is proven invisible without `top` (A-14); the view records its own path, so `top show` gets the root's annotator |
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
| same, non-first key pair of a multi-leaf key | → | `ParseListAddress` pair arm | `test/parse/enum-refusal-multi-leaf-key.ci` |
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
| SSH editor `set`, `delete`, `commit`, `discard` and every other incidental-view verb, no `top` | → | `refreshConfig` in `handleCommandResult`/`applyResult` | `TestIncidentalViewUnchangedWithoutTop` |
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
| AC-14 | every value accepted before this change, on every path | still accepted, except the named moves: the SSH editor, `insert` and offline set take the parser's verdict (`TestValidateValueAtPathRangePatternRefusal` asserts each move: valid prefixes and hex keys accepted, declared range, length and pattern misses refused); a ranged decimal64 in-range value accepted (AC-41); an enum value on a `MultiLeafNode` or `BracketLeafListNode` refused (AC-5). Timing moves: the web refuses at set; a refused new key is refused where the entry is created. Every value refused before stays refused, a typed `default` and `x#3` on a typed key included (`TestTypedDefaultKeyStaysRefused`). Every stored entry stays reachable (`TestLookupWalkFindsEveryStoredKey`) and every stored change file reads back (`TestChangeFileReadsBackUnjudged`). Every other verdict is unchanged: the scoped package tests of `config`, `config/yang`, `config/cli`, `cli`, `web`, `api`, `gnmi`, `command`, `plugin/server`, `bgp/config` and the named functional tests stay green except the pins this spec rewrites; `TestParserRefusesUnionNonMember` asserts the parser keeps refusing a union non-member |
| AC-16 | sensitive enum leaf (test schema), value close to an accepted one, on the parser and the walk | `config.SecretDataPlaceholder` in the value slot; no value, no `did you mean`; JSON omits `actual`; the accepted values still listed, on the walk too (`SectionValidationError.Message` stops writing `sensitive value redacted` for a refusal it can render masked) |
| AC-17 | tree walk refusal through `validateEntry` (Go test: `ValidateCustomSections` over a walked section, and `Validator.Validate`) | `Type` `ErrTypeEnum`, `Got` the value, `Expected` in declared order; `Error()` is the operator error, the path named once; `errors.As` recovers the `ValidationError` and the `*opererr.Error`; a missing, type, reference, cardinality or unknown `ValidationError` keeps `<type> error at <path>: <message>`; the RFC-tagged tests green unchanged |
| AC-18 | `route-refresh;` | problem `bgp peer peer1 session capability route-refresh: needs a value`, the value list, `fix: route-refresh <enable\|disable\|require\|refuse>;`; no `needs a value, one of` |
| AC-19 | SSH editor `edit bgp peer peer1`, `load` a snippet with `route-refresh requre` | the AC-2 body naming the full path, context and key included; the fix is the leaf line `fix: route-refresh require;`, the line the operator corrects in the snippet |
| AC-20 | SSH editor `set cli format bogus` | the Set frame in the viewport, path `cli format`, values in declared order (`text, table, json, yaml, ndjson`) with help, `fix: set cli format <text\|table\|json\|yaml\|ndjson>`; the feedback row in `errorStyle`; no `(valid: `; completion keeps sorted order |
| AC-21 | `errors.As` on a hierarchical and a set parser error, on `ValidateValueAtPath`'s error, on a walk error, on `ValidateContent`'s error, on a `WithFix` and a `WithPath` copy | each recovers the `ValidationError` and the `*opererr.Error`; a copy's other facts equal the original's |
| AC-22 | `update text origin bogus nlri ipv4/unicast add 10.0.0.0/24` from a plugin; the same through `update cursor` | the problem `update text origin: "bogus" is not an accepted value` (`update cursor origin` for cursor), origin values with help in declared order, a `fix:` line with the command retyped; no `bgp/peer/update/attribute/origin` |
| AC-23 | SSH editor commit, `commit confirmed 60`, `commit verify` and `show \| errors` of a candidate holding `route-refresh requre`, in file mode and in session mode, at the root and inside `edit bgp peer peer1` and inside `edit ospf`, also on an inactive (`nop`) leaf; a web terminal commit and a CLI bar commit of the same, at the root and inside a context | `CommitRefusal` keeps its header, ends it with no trailing `: `, and writes each issue on its own line; the SSH viewport shows the issues, the status row the header alone; at the root the fix is `fix: set bgp peer peer1 session capability route-refresh require`; inside any context it is `fix: top set bgp peer peer1 session capability route-refresh require`; never a file-form line or `nop`; after typing it the leaf holds `require`, the position is unchanged, and an inactive leaf stays inactive; `commit confirmed 60` refuses through `commitValidationRefusal` and writes no `cannot commit:` (`formatValidationErrors` deleted); no forced-command hint, since an error blocks. A refused member of a plain leaf-list in the candidate gets `fix: delete <path> <refused-member>`, behind `top` inside a context. The web terminal and the CLI bar (now given the request's position) answer the same lines |
| AC-24 | set-format file with `set bgp peer peer1 session capability route-refresh requre`, the `set-meta` form, and the same leaf on a `nop` line; `ze config validate` and daemon start | `line N: ` and the AC-2 body; `fix: set bgp peer peer1 session capability route-refresh require` (`fix: nop ...` on the `nop` line: only a file holds `nop`); no `invalid value for`; JSON as AC-11 |
| AC-25 | SSH editor `set <ospf-path> redistribute bogus metric 30` (OSPF `list redistribute`, key `source`), file and session mode; and a hierarchical file holding `redistribute bogus { metric 30; }` | the editor refuses where the entry would be created, no entry created: path ending `redistribute bogus`, every `source` value with help, `fix: set <ospf-path> redistribute <connected\|static\|kernel\|bgp\|isis> metric 30`; `show` holds no `redistribute bogus`. The file refuses at the list-key arm with the same body, the full path named, and the file-form fix; no `invalid key for` |
| AC-26 | SSH editor `rename` and `copy <ospf-path> redistribute connected to bogus` | refused through `ListEntryDestination`: `fix: rename <ospf-path> redistribute connected to <connected\|...>` (and `copy`); `show` unchanged |
| AC-27 | web terminal `set <ospf-path> redistribute bogus metric 30` | as AC-25, after `error: `; nothing stored |
| AC-28 | offline `ze config set <file> <ospf-path> redistribute bogus metric 30` | exit 2; stderr the AC-27 text; file unchanged |
| AC-29 | SSH editor `insert <path> <member> last` under a refused key (test YANG module in `TestKeyCheckOnInsertNestedActivation`) | refused; fix the `insert` command with the key replaced; no entry created |
| AC-30 | SSH editor `set <list-path> <bad-key> <child-list> <key-leaf> <key>` (same test module) | refused on the ancestor key; nothing created, file and session mode |
| AC-31 | SSH editor `deactivate` and `activate <list-path> <bad-key> <leaf-list> <member>` (same test module) | refused, since no entry exists under the refused key; nothing marked |
| AC-32 | every key the parser accepts, among them `10.0.0.0/8` and `2001:db8::/32` on `bgp policy prefix-list <n> entry <prefix>`, through SSH `set` mid-path and `rename`, offline set and the web terminal; and lookups of stored anonymous and `#N` entries | accepted; each case carries a control proving the creator ran (`EnsureListEntry` created the `entry` list's entry), and the OSPF `connected`/`bogus` pair proves the creator judges the key; `TestLookupWalkFindsEveryStoredKey` finds an anonymous and a `#N` entry through `ListPathStep`, `deleteFromList` and the inactive marker |
| AC-33 | web terminal `set <ospf-path> redistribute source bogus` (key-leaf form); the same with `connected`; the in-entry `set <ospf-path> redistribute connected source static` | `bogus` refused with `fix: set <ospf-path> redistribute source <connected\|...>`, nothing stored; `connected` stores keyed entry `redistribute connected`, no anonymous entry; the in-entry line refused with the in-entry key-leaf refusal (Text samples), nothing stored, the change file still parses (`ParseChangeFile`) |
| AC-34 | SSH editor in session mode: create `redistribute connected`, then `set <ospf-path> redistribute connected source static` | the in-entry key-leaf refusal in the viewport; nothing stored; the change file still parses |
| AC-35 | range: `ospf interfaces interface eth0 cost 0` on every config surface; `cost 70000` (past the width); a declared `error-message` on a range (test schema) | the problem `<path>: "<value>" is outside the accepted range`, `cost accepts a value in 1..65535` (declared bounds for `70000` too), `fix: cost <1..65535>;` or `fix: set <path> <1..65535>`; with an `error-message`, that message is the why line; JSON `config-yang-range`, `expected` `1..65535`; no `outside range`, `outside allowed` |
| AC-36 | length: `exabgp bridge process p1 run ""` on the same surfaces; a sensitive length miss | `"" is outside the accepted length (0 characters)`, `run accepts 1..4096 characters`, fix `<1..4096 characters>`; sensitive: placeholder, no count; JSON `config-yang-length` |
| AC-37 | pattern: `l2tp shaper default-rate fast`, `service image-server rescue-auth tooshort` (inline), the OSPFv3 IPsec hex key (sensitive), and the control `ospf router-address 10.0.0.256` | `is not in the accepted format`, the format text (typedef description, or the leaf's `ze:help` inline), fix `<rate>` or `<value>`; no regular expression in the output; the address control keeps `invalid IPv4: "10.0.0.256"`; JSON `config-yang-pattern` with `expected` the format text |
| AC-38 | command argument length miss (`cli-argument-length-refused.ci`), range miss (`show vpn ipsec dataplane sa spi 0`), pattern miss (`resolve dns ptr zz`), each alone and beside a second fault | one fault: the kind's problem naming the command words and argument, the why from bounds, format reader or `error-message`, fix the whole command with the kind's placeholder in place of the value; two faults: one `ArgumentFaults` refusal as AC-12; no `out of range`, `out of allowed ranges`, `does not match expected pattern` |
| AC-39 | pattern miss on a leaf-list member, a bracket leaf-list member, a `MultiLeafNode` value; range, length, pattern miss on a new list key, at the command and in a hierarchical file (the list-key arms) | the kind's refusal for that member or key, the full path named as in AC-1; a key at creation gets the Set form with the placeholder in the key's place, a key in a file the file form |
| AC-40 | a pattern typedef with no `ze:help` and no description; an inline pattern on a leaf with no `ze:help` (test schema) | `<leaf> accepts a value of type <typedef-name>`; the inline case's why is `<leaf> does not accept this value`; no regular expression |
| AC-41 | the five ranged `zt:decimal-2` leaves given an in-range value (`5.00`, `5.00`, `2.50`, `5.00`, `0.25`) and an out-of-range one (`500.00`, `500.00`, `16.01`, `0.99`, `1.01`), through `ze config validate` (text, JSON), SSH `set`, offline set; `5.001` and `abc` | in range accepted; out of range the range refusal with the decimal bounds (`1.00..100.00`, `0.00..16.00`, `0.00..1.00`); `5.001` and `abc` keep the type refusals `expected decimal64 with at most 2 fraction digits` and `expected decimal64` |
| AC-42 | multi-leaf-key list (BFD `single-hop-session`, key `peer vrf interface`): a non-first key pair whose value its leaf refuses, in a file and at SSH `set` | the kind's refusal for that key leaf, path naming the list and the first key value; no `invalid <leaf> for <list>:` prefix |
| AC-43 | REST, gRPC and gNMI session set of `<ospf-path> redistribute bogus metric 30` | refused at set, nothing stored: REST HTTP 400 (`writeSessionError`), gRPC `InvalidArgument` (`sessionStatusError`), gNMI `InvalidArgument` (today `Unknown`: `applyUpdate` returns the `SetSegments` error raw; this spec wraps it as `InvalidArgument`, a defect fix); each message holds the problem, the value lines and `fix: set <ospf-path> redistribute <connected\|...> metric 30` |
| AC-45 | SSH editor inside `edit bgp peer peer1`: `top set ospf router-id 192.0.2.9` | the leaf `ospf router-id` holds `192.0.2.9`; the prompt and the position are still `bgp peer peer1`; a refreshed view shows the config at `bgp peer peer1`; history holds the line `top set ospf router-id 192.0.2.9` as typed |
| AC-46 | SSH editor inside `edit bgp peer peer1`: `top show` | the viewport shows the whole configuration from the root; afterwards the prompt and the position are still `bgp peer peer1` |
| AC-47 | SSH editor inside `edit bgp peer peer1`: `top edit ospf`; then `top` alone; then `top top edit bgp`; then `top run show version`; then, with no pending change, `top exit` | `top edit ospf` moves the position to `ospf`; `top` alone moves to the root as today; `top top edit bgp` moves to `bgp`; `top run show version` answers what `run show version` answers and leaves the position; `top exit` leaves config mode exactly as `exit` does |
| AC-48 | SSH editor inside `edit bgp peer peer1`: `top set nosuchleaf 1`, `top set bgp peer peer1 session capability route-refresh requre`, `top nosuchverb`; and `top exit` with a pending change | each refused with the inner command's own refusal (the AC-2 body for the second, with its fix framed from the operator's position as typed: `fix: top set bgp peer peer1 session capability route-refresh require`; `unknown command` for the third); the position stays `bgp peer peer1`; nothing stored; `top exit` with a pending change asks the same confirmation `exit` asks |
| AC-49 | SSH editor inside `edit bgp peer peer1`: completion and `?` after `top ` and after `top set ` | the rest of the line is completed with `m.completer.Complete(tree, rest, nil)` at the root context and the editor's own verbs: the candidates are what the root offers at the same point (`set`, `edit`, `show`, `run` and the rest after `top `; the root's top-level nodes after `top set `), never the context's children; `?` after `top` shows the `top <command>` form from the help table |
| AC-50 | web terminal and CLI bar, position `bgp peer peer1`: `top set ospf router-id 192.0.2.9`, `top show`, `top edit ospf`, `top` alone, `top set nosuchleaf 1` | each answers as on the SSH editor (AC-45 to AC-48): the answered position stays `bgp peer peer1` except after `top edit ospf` (`ospf`) and `top` alone (root); the failing line answers the inner refusal; the CLI bar no longer drops the rest of a `top ...` line |
| AC-51 | SSH editor commit inside `edit ospf` of a candidate whose `bgp peer peer1` lacks a mandatory leaf; the same at the root; the same on the web terminal and the CLI bar | inside the context the missing-mandatory hint reads `(top set bgp peer peer1 <leaf> <value>)`; at the root `(set bgp peer peer1 <leaf> <value>)` as today; the hint is framed when it is rendered, so a hint computed before an `edit` reads correctly after it; the `discard ...` conflict fixes are unchanged |
| AC-52 | SSH editor inside `edit bgp peer peer1`: `top load terminal relative`, paste `ospf { router-id 192.0.2.9; }`, Ctrl-D | the paste is applied at the root: `ospf router-id` holds `192.0.2.9` and nothing is written under `bgp peer peer1`; the position stays `bgp peer peer1` |
| AC-53 | SSH editor inside `edit ospf`, a candidate holding `bgp peer peer1 session capability route-refresh requre`: `top commit`; `top show \| errors`; the web terminal and the CLI bar at position `ospf`: `top commit` | `top commit` is refused with `fix: top set bgp peer peer1 session capability route-refresh require` (the operator's position, not the copy's root); `top show \| errors` lists the same issue with the same fix and is not refused as unpipeable; the web answers the same line; the position stays `ospf` |
| AC-54 | no `top`: at the root and inside `edit bgp peer peer1`, each of the 23 incidental-view verbs of Data Flow 7 (`set`, `delete`, `activate`, `insert`, `rename`, `commit verify`, a refused `commit`, `discard`, `rollback`, `show changes` with none, `option errors hints` and the rest) | the viewport content, its diff gutter, its validation marks, the scroll position and the status row equal what the copy-built `configView` drew before the switch; history holds every line exactly as typed, and a line typed behind `top` is recorded with `top` (AC-45) |

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
| `TestValidateEnumerationSharedBody`, `TestSectionValidationErrorRendersRefusal` | `internal/component/config/yang/validator_test.go`, `internal/component/config/validate_sections_test.go` | AC-16, AC-17 | |
| `TestRefuseInvalidCustomSectionsOneRefusalPerLine`, `TestSectionRefusalsJoinOnePerBlock` | `internal/component/config/value_refusal_test.go` | AC-10: walk refusals one per line, never `; `; the join puts each refusal on its own block ending with its own `fix:` line | committed |
| `TestLegacyHintPrecedesTheRefusal` | `internal/component/config/value_refusal_test.go` (the spec named `loader_test.go`) | AC-10 hint placement | committed |
| `TestRunValidationParserEnumDiagnostic`, `TestValidateContentFramesEnumRefusal` | `internal/component/config/cli/cmd_validate_test.go` | AC-8, AC-11, AC-21 | |
| `TestValidateValueAtPathEnumRefusal`, `TestValidateValueAtPathRangePatternRefusal` | `internal/component/cli/completer_test.go` | AC-6, AC-14 | |
| `TestEditorMultiLineErrorKeepsThePromptRow`, `TestCommitRefusalWritesOneIssuePerLine`, `TestEditorFixIsTheCommandTypedHere`, `TestCommitFixUsesTopInsideAContext`, `TestMissingMandatoryHintUsesTopInsideAContext`, `TestCliFormatRefusalUsesSharedBody`, `TestInEntryKeyLeafRefusal`, `TestFormatPeerErrorPassesTheRefusalThrough`, `TestKeyCheckOnInsertNestedActivation` | `internal/component/cli/*_test.go` | AC-6, AC-20, AC-23, AC-29 to AC-31, AC-34, AC-51 | |
| `TestTopKeepsPositionUnlessInnerMoves`, `TestTopShowShowsTheRoot`, `TestTopSetRefreshesAtThePosition`, `TestTopFailingCommandKeepsPosition`, `TestTopInterceptedVerbsBehaveAsWithout`, `TestTopLoadTerminalPastesAtTheRoot`, `TestTopCompletesFromTheRoot`, `TestTopCommitFixUsesTopSetInsideAContext`, `TestTopRefusedSetFixKeepsTop`, `TestTopPipedShowRunsAtTheRoot`, `TestTopKeepsHistoryAsTyped` | `internal/component/cli/model_top_test.go` | AC-45 to AC-49, AC-52, AC-53, A-11, A-13, R-18 to R-21; `TestTopShowShowsTheRoot` also asserts the tree-diff annotator; `TestTopKeepsHistoryAsTyped` covers `top set`, `top run`, `top clear` and an unrecorded `top exit` | |
| `TestIncidentalViewUnchangedWithoutTop` | `internal/component/cli/model_view_refresh_test.go` | AC-54, A-14: at the root and in a context, the applied viewport equals `configViewAtPath(m.contextPath)` rendered through `setViewportData` at the same position, with marks and status row equal. As committed it covers 9 typed lines (its VALIDATES list): `set`, `delete`, `deactivate`, `activate`, `commit verify`, `discard all`, `option errors hints`, `option errors hide`, `load file` (`applyLoad`). Not reached by it, of the 23 sites of Data Flow 7: `createListEntry`, `cmdInsert`, `listEntryMove` (`rename`, `copy`), `cmdRollback`, `commitValidationRefusal` (a refused `commit`), the `commit failed:` reload arms of `commitCandidateAndReload` and `runCommitSession`, `cmdDiscardSession`, `cmdDiscardPath`, `cmdShowChanges` and `cmdShowChangesAll` (none, and the count), `cmdDisconnectSession`, `rollbackConfirmed` | partial |
| `TestShowPathAnnotatesAtItsOwnPath` | `internal/component/cli/model_view_refresh_test.go` | `show bgp` at the root is annotated with the line gutter of its own path, `show` at the root with the tree diff, `top show` inside a context with the tree diff (the recorded `viewportData` path) | |
| `TestFrameFix` | `internal/component/cli/fix_frame_test.go` | the three arms of `FrameFix` (equal position, root operator, other), A-15 | |
| `TestWebFormRefusesEnumAtSet`, `TestWebTerminalCommitShowsEnumRefusal`, `TestWebTerminalKeyLeafFormStoresKeyedEntry`, `TestWebTopCommandKeepsPosition`, `TestCLIBarCommitFramesTheFixWithThePosition` | `internal/component/web/*_test.go` | AC-7, AC-23, AC-33, AC-50, AC-51 | |
| `TestSetUpdateRefusalIsInvalidArgument` | `internal/component/gnmi/set_test.go` | AC-43 gNMI code | |
| `TestValidateArgEnumRefusal`, `TestValidateArgRangeLengthPatternRefusal`, `TestOneValueFaultIsRestated`, `TestFaultEntryHoldsNoFixLine` | `internal/component/command/*_test.go` | AC-12, AC-38 | |
| `TestDispatcherRestatesOneValueFault` | `internal/component/plugin/server/route_arg_faults_test.go` | AC-12 dispatcher route | |
| `TestUpdateTextOriginEnumRefusal` | `internal/component/bgp/plugins/cmd/update/update_text_test.go` | AC-22 | |
| `TestConfigValidationHookEnumRefusal` | `cmd/ze/hub/rfc8907_api_test.go` | AC-8 at the hook | |

### Boundary Tests (numeric inputs)
| Field | Range | Last valid | First invalid below | First invalid above |
|-------|-------|------------|---------------------|---------------------|
| `cost` | 1..65535 | 65535 | 0 | 65536 |
| `threshold-multiplier` | 1.00..100.00 | 100.00 | 0.99 | 100.01 |
| `run` length | 1..4096 | 4096 characters | 0 | 4097 |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `enum-refusal-proposes-valid-values` | `test/parse/` | AC-1, AC-2, AC-3, AC-18, AC-25 file case | |
| `enum-refusal-inline-leaf-list`, `enum-refusal-set-format`, `enum-refusal-union`, `enum-refusal-multi-leaf-key`, `enum-refusal-config-set`, `range-length-pattern-refusal`, `decimal64-range` | `test/parse/` | AC-5, AC-13, AC-24, AC-28, AC-35 to AC-37, AC-39, AC-41, AC-42 | |
| `enum-refusal-daemon-start` | `test/plugin/` | AC-10, AC-24 | |
| `enum-refusal-reload` | `test/reload/` | AC-10 | |
| `enum-refusal-set`, `enum-refusal-load`, `enum-refusal-commit`, `range-length-pattern-set` | `test/editor/validation/` | AC-6, AC-19, AC-23 (with the top case: from inside `edit bgp peer peer1`, type the printed `top set ...` fix line, `commit`, and see the commit succeed and the position unchanged), AC-25, AC-26, AC-34 | |
| `top-command` | `test/editor/navigation/` | AC-45 to AC-49, AC-52 | |
| `config-enum-refusal`, `config-range-length-pattern-refusal` | `test/web/` | AC-7, AC-27, AC-33, AC-50, Q-8a, Q-8b | |
| `api-config-enum-refusal`, `grpc-config-enum-refusal`, `gnmi-set-enum-refusal`, `command-arg-enum-refusal`, `update-text-origin-enum-refusal` | `test/plugin/` | AC-8, AC-9, AC-12, AC-22, AC-43 | |
| `cli-argument-pattern-refused` | `test/ui/` | AC-38 | |
| Rewritten pins (Current Behavior table), each asserting path, value, every accepted value and the `fix:` line, each change judged by `ze-test-judge` | as listed | | |

### Interop Tests (Scope: protocol)
- N-A: no wire behaviour changes.

## Files to Modify

| File | Change |
|------|--------|
| `internal/core/suggest/suggest.go` | the value-list renderer beside `Command` |
| `internal/component/config/yang/validator_error.go`, `validator.go` | `ValidationError` holds, for the enum, range, length and pattern kinds, segments, accepted values, suggestion, secret flag, format text and its operator error; `Error()` delegates for those kinds and keeps today's text for the others; `Unwrap` answers the operator error; `WithFix` and `WithPath` copy; `validateEntry` attaches the facts; `walkTree` threads segments; range, length, pattern arms build the kind |
| `internal/component/config/yang/enum.go`, `constraint_message.go`, `command.go` | declared order and help for the walk and `ArgDef`; a declared message as the why; the format reader's callers |
| `internal/component/config/yang_schema.go` | `yangToLeaf` and the three leaf-list builders store enum facts, help and format text; the sensitive reader moves to `config/yang`; decimal64 maps to `TypeDecimal64` with `FractionDigits` |
| `internal/component/config/schema.go` | `ValidateLeafValue` and `ValidateListKey` take segments and build the refusal, union arm included; the decimal64 range arm; enum checks on `MultiLeafNode` and `BracketLeafListNode` |
| `internal/component/config/parser.go`, `parser_diagnostics.go`, `line_error.go` | `parseLeaf` and `parseInactiveBracketMember` return the refusal inside `syntaxRefusal`; `syntaxRefusal` keeps the unwind segments as a list; `Parse` and `ParseAt` rebuild a refusal whose chain holds a `ValidationError` as `newLineError(line, ve.WithPath(segments))`, so no slash prefix is printed for it; `SplitLine` needs no new arm; the file-form fix replaces `write:` |
| `internal/component/config/parser_list.go`, `parser_freeform.go` | every value and key arm wraps its refusal in `syntaxRefusal` so it gets the path on unwind: the two list-key arms (`ValidateListKey`), whose `invalidListKeyError` is replaced by the friendly key refusal with its `peer` special case deleted; the two inline-list child arms, the multi-leaf arm, the bracket arm, the two value-or-array arms (the `(valid: ` text among them), the value arm after them, and the freeform attribute arm. The `parser_freeform.go` inline-list `KeyType` check stays as it is (exempt, `exabgp.yang` only) |
| `internal/component/config/setparser.go`, `setparser_meta.go`, `setparser_entries.go` | segments from the consumed tokens; the file-form fix with the line's verb; the key-only arm routed through `EnsureListEntry`; the raw-token first-key `ValidateListKey` calls removed, the creator judging the same token, its error wrapped in the line error |
| `internal/component/config/list_key.go` | `EnsureListEntry` returns an error and judges a new entry's first key value unless the address has `Tokens == 0`, with no suffix stripped; `ListEntryDestination` judges the destination key; the `ParseListAddress` pair refusal becomes the friendly one |
| `internal/component/config/change_file.go` | `materializeMetaPaths` keeps building its addresses with `Tokens == 0`, so stored keys are read back unjudged; the creator cannot refuse that form, so the call keeps its shape |
| `internal/component/config/loader.go` | `parseConfigError` writes its legacy-syntax hint before the refusal |
| `internal/component/config/validate_sections.go` | `Message()` renders the refusal; refusals one per line |
| `internal/component/config/cli/cmd_validate.go`, `cmd_set.go` | JSON codes and fields from `errors.As`; `ValidateContent` joins the operator errors (`errors.Join`), each block indented; offline set prints the refusal |
| `internal/component/cli/completer.go`, `completer_validate.go` | `ValidateValueAtPath` judges with `ValidateLeafValue` and the member check; the heuristic deleted; completion judges a typed new key with `ValidateListKey` |
| `internal/component/cli/editor_walk.go`, `editor_commands.go` | callers of `EnsureListEntry` handle its error |
| `internal/component/cli/model_commands.go` | `dispatchCommand` takes `top` with arguments before its pipe split; `cmdTop(args)`: no arguments as today; with arguments, `m.topOrigin` and `m.underTop` saved on the copy (outermost `top` only), the copy's `m.contextPath` emptied, the rest dispatched through `dispatchCommand`, the inner result answered with its views as built |
| `internal/component/cli/model_commands_option.go`, `model_commands_edit.go`, `model_commands_session.go`, `model_commit_window.go` | the incidental sites of Data Flow 7 answer `refreshConfig: true` in place of `configView: m.configViewAtPath(m.contextPath)`; the refused `set`, `insert`, `rename` and `copy` fixes built as `FixCommand` and framed with `m.operatorPosition()` (`FrameFix`, a file this spec creates) |
| `internal/component/cli/model_keys.go` | `handleEnter` drops a leading `top ` before its interceptions when the rest is `run ...`, `run`, `exit`, `quit`, `stop`, `restart` or `clear`; `finishPasteMode` applies the paste at the position the paste state records |
| `internal/component/cli/model.go`, `model_load.go` | `Model` gains `topOrigin` and `underTop` (set only on a copy by `cmdTop`); `rollbackConfirmed` and `applyLoad` answer `refreshConfig`; `dispatchWithPipe` unchanged; the paste state records the position `load terminal` ran at (`commandResult` and the model beside `pasteModeLocation`); `commandCompleterInput` and `updateCompletions` gain a `top ` branch that strips `top ` and completes the rest with `m.completer.Complete(tree, rest, nil)` and the editor's own verbs |
| `internal/component/cli/editor_help.go` | `top` gains the form `top <command>`: "Run one command as if at the root context" (shipped in 95445948c5; the earlier wording "then come back" was false for `top edit`, which moves the position) |
| `internal/component/cli/model_render.go`, `model_commands_commit.go`, `model_commands_edit.go`, `validator.go` | the multi-line `m.err` route to the viewport; `set cli format` through `ValidateLeafValue`; one issue per line; `commit confirmed` through `commitValidationRefusal`; a refused `set` keeps the relative form typed; `ConfigValidationError` carries a typed fix (`FixCommand`) for commit refusals and the missing-mandatory hint, kept beside `Message`; `CommitRefusal`, `appendIssueSummary` and `formatIssueList` take `operator []string` and frame it with `FrameFix`, called with `m.operatorPosition()` (`set` at the root, `top set` inside a context, behind `top` too); the eight commit-file incidental sites answer `refreshConfig`; `configViewAtPath` records its path in `viewportData` and `setViewportData` chooses the annotator from that path, not `m.contextPath` (fixing `show <path>` too); `parseError` keeps the operator error; the in-entry check; `formatPeerError`'s `ErrTypeEnum` arm passes the `ValidationError` through with the peer context |
| `internal/component/web/cli_terminal.go`, `cli.go` | the terminal's `executeTerminalConfigMode` runs `top <command>` with an empty position and keeps the request's position unless `executeTerminalNav` answers a new one; the CLI bar's `dispatchCLICommand` calls itself with the rest of the line and an empty position (replacing `handleCLITop`'s silent drop of its arguments); `cliCommand` gains `Operator` (the request's `path`), kept by both `top` recursions; `handleCLICommit` and `runWebCommit` pass `cmd.Operator` to `EditorManager.runCommit` (`editor_commit_window.go`), which passes it to `cli.CommitRefusal`; `handleCommitPost` and `CommitNow` (`handler_config_commit.go`, `editor_commit_window.go`) pass nil; `executeTerminalSet` and `handleCLISet` frame their fixes with `cmd.Operator`; value checks at set (terminal, CLI bar); the key-leaf form through `CreateEntry`; the in-entry check |
| `internal/component/web/handler_config_form.go`, `handler_config_entry.go`, `error_fragment.go`, `editor.go` | value checks at set (form); key checks through the creator (ADD, rename, URL keys); the notice reads `Problem`, `Why`, `Fix` through `errors.As`; the decimal64 field type stays string |
| `internal/component/gnmi/set.go` | `applyUpdate` wraps the `SetSegments` error as `InvalidArgument` (AC-43) |
| `internal/component/api/config_session.go`, `internal/component/api/grpc/server.go`, `internal/component/api/rest/server.go` | the set refusal passes through unchanged (the status mapping stays); the typed fix is written in the plain root form, since these surfaces hold no position |
| `internal/component/command/argvalidate.go`, `argfaults.go`, `argbind.go`, `argdef.go` | `ValueArgumentError` from the renderer; `verdict` answers an `ArgumentFaults` of one for a lone value fault; `newArgumentFaults` accepts one value fault and then writes its problem and why without the count header; `wholeCommand` writes a value fault's placeholder in place of its typed value and runs when every fault is missing or a value fault; a value fault's list entry is its problem and why, never its fix; the `ArgumentFaults` and `newArgumentFaults` docs that say "two or more" are rewritten |
| `internal/component/plugin/server/arg_faults.go`, `internal/component/plugin/server/command.go`, `internal/component/command/local_data.go` | `restateArgumentFaults` and `ValidateModelArgs` restate a one-fault refusal through `RestatedForLine`; the `several` variable and comments at the dispatcher's restate, in `adoptablePositional`, `argFaultOutsideSelector` and `validateModelArgsAs` are renamed and reworded, their verdicts unchanged |
| `internal/component/bgp/plugins/cmd/update/update_text.go`, `cursor.go` | re-root to the command words |
| `internal/component/bgp/config/loader_test.go`, `internal/component/plugin/server/send_test.go`, `rfc8907_command_test.go`, and the other pins | rewritten (Current Behavior) |
| `rfc/discrimination/rfc7950.json` | the stale records re-recorded with `./le rfc record` |

## Files to Create

- `internal/core/suggest/valuelist_test.go`, `internal/component/config/yang/refusal_test.go`, `internal/component/config/yang/format_test.go`, `internal/component/cli/model_top_test.go`, `internal/component/cli/model_view_refresh_test.go`, `internal/component/cli/fix_frame_test.go`
- `internal/component/cli/fix_frame.go` - `FixCommand` and `FrameFix(fix FixCommand, operator []string) string`, the one framer of editor-command fix lines (Data Flow 5), and `Model.operatorPosition()`
- `internal/test/fixture/plugin_fixture_enum_refusal.go` - three `le test fixture` clients registered beside `plugin/api-config-commit-reject`: `plugin/api-config-enum-refusal` (REST session set then commit), `plugin/grpc-config-enum-refusal` (gRPC `ZeConfigService` session set then commit) and `plugin/gnmi-set-enum-refusal` (a gNMI `SetRequest` over the vendored `github.com/openconfig/gnmi` proto, asserting the code and the message); each takes the path and the value, so one client serves every kind and the key cases
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
| 6 | Has a user guide page? | Yes | `docs/guide/config-editor.md` (Phase 3), `docs/guide/web-interface.md` (Phase 4, `top <command>` and set-time refusals) |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior changed or newly proven? | Yes | RFC 7950 Section 9.3 decimal64 range now enforced by the parser; `docs/architecture/config/yang-config-design.md`; Phase 2 |
| 10 | Test infrastructure changed? | Yes | three fixture clients; the page `ai/CODE-TO-DOCS.md` names for `internal/test/fixture/`; Phase 4 |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` (the refusal, its error chain, the format reader) and `docs/architecture/cli/error-surface.md` (multi-line errors in the SSH viewport, the `top` fix rule, the typed fix framed by each surface); Phases 1 and 3 |
| 13 | Route metadata keys? | No | none |
| 14 | Prometheus counters? | No | none |
| 15 | Registered plugin, event, command or inventory changed? | No | none |
| 16 | Changed source referenced by doc anchors? | derived | `./le spec citation anchors spec plan/release/fixes/spec-config-enum-refusal-proposes-valid-values.md` per phase |
| 17 | Docs show examples for this area? | Yes | grep `docs/` for `invalid enum`, `expected one of`, `invalid peer name`, `write:`, `(set bgp` per phase |

## Implementation Steps

Each phase lands as its own commit once its scoped tests pass; a phase that
edits an RFC 7950 producer re-records its records in the same commit. Evidence
is the named scoped runs (`./le job run label <area> command go test -race
<packages>` and the named functional tests), never `./le verify`.

Order: Phase 1, then Phase 2, then Phase 3, then Phase 4 (the web imports
`internal/component/cli`, which Phase 3 changes). Phase 5 may run beside
Phase 3 once Phase 2 lands, provided it changes no exported signature of
`internal/component/command` that `cli` reads; otherwise it runs after Phase 3.
Phase 6 runs last. Before Phase 2 commits, the `AddListEntry` grep is re-run
against the creator table (A-9). Within Phase 3, `top <command>` lands first,
as its own commit, because the commit-refusal fix lines print it.

| Phase | Packages (files) | Tests | Red walk owed |
|-------|------------------|-------|---------------|
| 1 Refusal and schema facts | `internal/core/suggest`; `internal/component/config/yang` (`validator_error.go`, `validator.go`, `enum.go`, `constraint_message.go`, format reader); `internal/component/config` (`yang_schema.go` only) | `TestValueListRender`, `TestValidationErrorOperatorError`, `TestFrameKeepsProblemAndWhy`, `TestTypeFormatReader`, `TestYangToLeafCarriesEnumHelp`, `TestValueOrArrayEnumDeclaredOrder`, `TestMultiLeafAndBracketCarryEnums`, `TestValidateEnumerationSharedBody`; RFC-tagged enum tests, `TestValidationErrorTypeContract` and the reference walk test unchanged | `./le rfc record` for `validateEnumeration`, `walkTree`, `yangToLeaf` and the range, length, pattern producers; break the renderer's alignment and see `TestValueListRender` red |
| 2 Parsers and keys | `internal/component/config` (`schema.go`, `parser*.go`, `line_error.go`, `setparser*.go`, `list_key.go`, `change_file.go`, `validate_sections.go`, `loader.go`); `internal/component/bgp/config` (the peer-name pin) | the parser, set-parser, list-key, change-file, schema, loader and section tests; the `bgp/config` loader test; `test/parse/*`, `test/plugin/enum-refusal-daemon-start.ci`, `test/reload/enum-refusal-reload.ci`, the BMP pins | `./le rfc record` for `refuseInvalidCustomSections`; break the creator's key check and see `TestEnsureListEntryJudgesNewKey` red; judge `Tokens == 0` and see `TestChangeFileReadsBackUnjudged` and `TestLookupWalkFindsEveryStoredKey` red; widen the exemption to a typed `default` and see `TestTypedDefaultKeyStaysRefused` red; drop one value or key arm's `syntaxRefusal` and see `TestEveryValueArmNamesTheFullPath` red; drop `WithPath` and see `TestParserEnumRefusalNamesFullPath` red; break the decimal64 arm and see `TestValidateLeafValueDecimal64Range` red |
| 3 Offline and SSH editor, `top <command>` | `internal/component/config/cli`; `internal/component/cli` (adds `model_commands.go` `cmdTop`, `model_keys.go` `handleEnter` and `finishPasteMode`, `model.go` and `model_load.go` paste state and completion, `editor_help.go`, `fix_frame.go`, the incidental-view files of Data Flow 7, `model_top_test.go`, `model_view_refresh_test.go`, `fix_frame_test.go` to the phase's files; the package set is unchanged) | `cmd_validate`, `completer`, model, commit and `top` tests; `test/editor/validation/*`, `test/editor/navigation/top-command.et`, `test/editor/pipe/set-cli-format.et`, `test/ui/config-*` | revert the viewport route and see `TestEditorMultiLineErrorKeepsThePromptRow` red; let `cmdTop` keep the emptied position and see `TestTopKeepsPositionUnlessInnerMoves` and `top-command.et` red; remove the `handleEnter` strip and see `TestTopInterceptedVerbsBehaveAsWithout` red; drop the paste position and see `TestTopLoadTerminalPastesAtTheRoot` red; print the root form inside a context and see `TestCommitFixUsesTopInsideAContext` red; frame from the copy's `m.contextPath` instead of `m.operatorPosition()` and see `TestTopCommitFixUsesTopSetInsideAContext` and `TestTopRefusedSetFixKeepsTop` red; put one incidental site back to a copy-built `configView` and see `TestTopSetRefreshesAtThePosition` red; resolve `refreshConfig` at another path and see `TestIncidentalViewUnchangedWithoutTop` red; split the pipe before `top` and see `TestTopPipedShowRunsAtTheRoot` red |
| 4 Web, REST, gRPC, gNMI | `internal/component/web`; `internal/component/api`; `internal/component/gnmi`; `internal/test/fixture`; `cmd/ze/hub` (tests) | web, api and gnmi tests, `test/web/*`, the three `test/plugin/*-enum-refusal.ci` | remove the terminal `set` check and see `config-enum-refusal.wb` red, with the binary rebuilt; let the web `top` keep the emptied position and see `TestWebTopCommandKeepsPosition` red; drop the bar's commit position and see `TestCLIBarCommitFramesTheFixWithThePosition` red; let the web `top` recursion reset `cliCommand.Operator` and see `TestWebTopCommandKeepsPosition` (its `top commit` case) red; remove the `applyUpdate` wrap and see `TestSetUpdateRefusalIsInvalidArgument` red |
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
| Only an asked-for view (`cmdShow`, `cmdShowPipe`, and `edit`/`up` at their new position) answers `configView`; the 23 incidental redraws answer `refreshConfig`, which the appliers resolve at the real position; every view records the path it was built at, and the annotator reads that path | `top show` must show the whole configuration (the owner's purpose), as `show <path>` keeps its own view; passing an incidental view through would show the root after `top set`, a position the prompt does not name, and the pass-through rule cannot tell the two apart, so the verb says which by the field it answers; the switch is invisible without `top` (A-14) |
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

## Review Gate

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

### Run 1

| # | Severity | Finding | Location | Status |
|---|----------|---------|----------|--------|

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
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
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
