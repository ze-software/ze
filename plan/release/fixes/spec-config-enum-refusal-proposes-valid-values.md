# Spec: config-enum-refusal-proposes-valid-values

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P1 |
| Release reason | Honor the approved consistent schema-derived enum refusal across operator entry points. |
| Scope | config |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

This spec was reworked on 2026-11-12 (Disposition: REWORK). The design of
2026-10-14, reviewed in 17 rounds, is in git history at `09be99b7db`. It
predates the operator error (`opererr`), the parser's union check, the parser's
line error and refusal path, YANG constraint messages, the argument-fault
refusal and multi-leaf-key list addressing. This version rebuilds the design
on those mechanisms. It keeps every owner answer and every acceptance
criterion's intent. Review rounds 18 to 21 judged the rework; their findings
are applied here and one short round 22 over the round-21 points is owed
(Review Gate).

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

Inside `edit bgp peer peer1`, the SSH editor's fix is the command typed there:
`fix: set session capability route-refresh <enable|disable|require|refuse>`.

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
- [ ] `ai/rules/cli.md` - validation errors surfaced to an agent
  → Constraint: the JSON diagnostic keeps its codes (`config-yang-enum`, `config-yang-range`, `config-yang-length`, `config-yang-pattern`) and its keys (`expected`, `actual`, `path`, `code`).
- [ ] `ai/rules/config.md` - enum help text
  → Constraint: an `enum` value carries its one-line text as `ze:help` alone; an enum `description` is read by no surface.
- [ ] `docs/architecture/config/syntax.md` - "Unknown keys", "multi-leaf list keys"
  → Decision: the unknown-key refusal writes ` (did you mean X?)` with `suggest.Command`; the enum refusal reuses the helper and the wording.
  → Constraint: a list address is the first key value, then `name value` for each other key leaf (`list_key.go`); every list path walker reads it through `ListPathStep`, and the one creator is `EnsureListEntry`.
- [ ] `docs/architecture/config/yang-config-design.md` - validation layers, "Constraint error messages"
  → Constraint: a declared `error-message` speaks in place of the generic text on every path (`ConstraintMessages`).
- [ ] `docs/architecture/api/commands.md` - YANG-typed argument validation
  → Constraint: a command line's faults are all named by `ArgumentFaults`, and every route restates them through `RestatedForLine`.
- [ ] `docs/guide/config-editor.md` - secrets in command answers
  → Constraint: `DisplayMessageAtPath` / `maskSecretInMessage` stay as a second defence; the refusal is built masked.

### RFC Summaries (Scope: protocol)
- N-A for a summary edit: no wire behaviour changes and no row of `rfc/short/rfc7950.md` is added or changed. RFC 7950 Section 9.6 (enumeration) is what the RFC-tagged tests prove; they assert `ErrTypeEnum`, `Expected` and `Got`, never the message text, and this spec keeps all three.
  → Constraint: every `rfc/discrimination/rfc7950.json` record whose producer this spec edits goes stale and is re-recorded with `./le rfc record` in the commit that edits it. The set counted 2026-10-13 was 25 (`validateEnumeration` 2, `yangToLeaf` 1, `walkTree` 5, `refuseInvalidCustomSections` 5, `validateString` 4, `checkYangRange` 3, `validateUnsigned` 1, `validateDecimal64` 4). 3e94cb7c40 has edited `validator.go` since, so each phase recounts its producers with `./le rfc check` before it commits.
  → Decision: the ranged decimal64 fix is RFC 7950 Section 9.3 behaviour now enforced by the parser; `TestValidateLeafValueDecimal64Range` is untagged (the one Section 9.3.4 row is the presence obligation, which it does not prove).

**Key insights:**
- The operator error exists and is the only refusal shape allowed; this spec adds no error type beside it.
- The typed facts the surfaces need (path segments, accepted values, kind, value, secret flag) live on the existing `configyang.ValidationError`, which holds the operator error and unwraps to it.
- The parser already checks unions (`validateUnionValue`), already builds a refusal path with keys on unwind (`prependSyntaxPath`, for a refusal wrapped in `syntaxRefusal`), and returns a typed line error (`LineError`); reload already prints one `parse config:`.
- The creator's key rule, stated once: an address with `ListAddress.Tokens == 0` is not judged; every other address is. Only `ListPathStep`'s anonymous arm and `materializeMetaPaths` (stored change-file keys read back) build an address with `Tokens == 0`; every address built from typed tokens has `Tokens >= 1` (`ParseListAddress`, `editor_walk.go`, `editor_commands.go`). So a typed `default` or `x#3` is judged, and stored `KeyDefault`, `#N` and change-file keys are not.

## Current Behavior (MANDATORY)

Read at HEAD `e660eff82e`, 2026-11-12.

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
- [ ] `internal/component/cli/completer_validate.go` (`validateTokenPath`) - skips a key with `ListAddressSpan`, judging none; the store after it creates through `EnsureListEntry`.
- [ ] `internal/component/config/yang/validator_error.go` (`ValidationError`, `ErrorType`) and `validator.go` (`validateEntry`, `validateEnumeration`, `walkTree`) - `ValidationError` is a plain struct rendering `<type> error at <path>: <message>`; `validateEnumeration` answers `ErrTypeEnum`, `value %q is not a valid enumeration value`, `Expected` = goyang's sorted names joined `, `.
- [ ] `internal/component/config/yang/constraint_message.go` (`ConstraintMessages`, `LeafConstraintMessages`, `WithConstraintMessage`) - a declared `error-message` on a range, length or pattern, joined after the refusal with `: `.
- [ ] `internal/component/config/yang/enum.go` (`EnumNamesDeclared`, `EnumValueSummaries`) - declared order and per-value help.
- [ ] `internal/component/config/validate_sections.go` (`refuseInvalidCustomSections`, `SectionValidationError.Message`) - refusals joined by `; `; `Message()` renders `<section>: <path>: <reason>` and hides the whole reason on a sensitive leaf.
- [ ] `internal/component/config/cli/cmd_validate.go` (`runValidation`, `ValidateContent`, `yangErrorCode`) - a parser error becomes `config-parse` with the `SplitLine` message and no `expected`/`actual`; `ValidateContent` writes `config validation failed:`, then per error `\n  ` and the message (continuation lines unindented), and flattens to `errors.New`.
- [ ] `internal/component/cli/completer.go` (`ValidateValueAtPath`), `completer_validate.go` (`validateLeafValue`, `validateYangType`, `validateStringPatterns`, `validateUintRange`) - `invalid value %q for <leaf> (expected <hint>)`, the hint `enum` for an enum; the heuristic refuses every valid prefix (journal `helper-bypassed-by-an-open-coded-copy.md`, 2026-10-13).
- [ ] `internal/component/cli/model_keys.go` (`handleSetCLIFormat`) - `invalid format: x (valid: json, ndjson, table, text, yaml)` written to `statusMessage`.
- [ ] `internal/component/cli/model_commands_commit.go` (`CommitRefusal`, `commitValidationRefusal`, `appendIssueSummary`, `formatIssueList`) and `model_load.go` (`cmdCommitConfirmed`, `formatValidationErrors`) - issues joined by `; ` on one status row.
- [ ] `internal/component/cli/model_render.go` (`View`, `feedbackLine`) - `bottomRows = 3`; row 1 is `Error: <m.err>`; a multi-line error pushes the prompt off the frame. The editor has `top` (`cmdTop`, no arguments); `cmdSet` always prepends `m.contextPath`; `set` has no root-path form.
- [ ] `internal/component/cli/validator.go` (`validateCore`, `parseError`, `validatePeer`, `formatPeerError`, the missing-mandatory warnings) - `validateCore` parses first and returns on a parse error, since both parsers answer no tree then; `formatPeerError` has an `ErrTypeEnum` arm (`must be one of`), reachable when the walk refuses a value the parser accepted (a `MultiLeafNode` or `BracketLeafListNode` member, which carries no enum list). The missing-mandatory warnings print a root-form `(set bgp peer ...)` hint whatever the context, which inside an `edit` context is not the command the operator types.
- [ ] `internal/component/web/cli_terminal.go` (`executeTerminalSet`, `execListEntryOp`), `internal/component/web/cli.go` (`handleCLISet`, `splitSetArgs`), `handler_config_form.go`, `handler_config_entry.go`, `error_fragment.go` (`WriteOOBError(path, message string)`) - no value check at `set`; the form checks the type only. These files import `internal/component/cli`.
- [ ] `internal/component/api/config_session.go` (`SetSegments`), `internal/component/api/rest/server.go` (`writeSessionError`: 400, 403 when forbidden, for set and commit), `internal/component/api/grpc/server.go` (`sessionStatusError`: `InvalidArgument` unless forbidden, for set and commit), `internal/component/gnmi/set.go` (a path refused before `SetSegments` answers `InvalidArgument "update path: ..."`; `applyUpdate` returns the `SetSegments` error raw and `Set` passes it unwrapped, so the client receives `Unknown` while the metric records `InvalidArgument`) - REST, gRPC and gNMI sets reach `cli.Editor.SetValue`, then `walkOrCreate`, then `EnsureListEntry`.
- [ ] `internal/component/command/argvalidate.go` (`validateEnum`, `validateUnion`, `validateUint`, `validateString`), `argfaults.go` (`ArgumentFaults`, `newArgumentFaults`, `Restated`, `RestatedForLine`, `wholeCommand`, `faultList.verdict`), `argbind.go` (`ValidateArgs`), `local_data.go` (`ValidateModelArgs`), `internal/component/plugin/server/arg_faults.go` (`restateArgumentFaults`), `internal/component/plugin/server/command.go` (the `several` read, `adoptablePositional`, `argFaultOutsideSelector`) - a lone fault is returned bare (`verdict`), so no route restates it; `newArgumentFaults` panics under two faults; `RestatedForLine` writes the whole command only when every fault is missing (`onlyMissing`), else the general correction (`argumentFaultsFix`); `Restated` lists a replacement fault by its `Error()`.
- [ ] `internal/component/bgp/plugins/cmd/update/update_text.go` (`parseCommonAttributeText`) - `invalid origin: %w` over the walk's `enum error at bgp/peer/update/attribute/origin: ...`.

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
| `internal/component/cli/model_commit_refusal_test.go` `TestCommitRefusalListsIssuesAndNamesForce`; `model_commit_mandatory_test.go` `TestCommitRefusesMissingMandatoryInSection`; `internal/component/web/editor_commit_window_test.go` | issues on the status row |
| `internal/component/cli/model_commands_edit_secret_test.go` `TestSSHCLISetNeverEchoesASecret`; `internal/component/config/cli/cmd_set_secret_test.go` `TestConfigSetRefusalNeverEchoesASecret` | `invalid value` (needle moves to `is not in the accepted format`; placeholder assertions unchanged) |
| `test/parse/asn4-refuses-boolean-spelling.ci`, `test/parse/capability-leaf-refuses-presence-spelling.ci`, `test/parse/ipsec-application-override.ci` (`expected one of`, `deny, allow`: the comma-joined list becomes one value per line, so the needle becomes each value line), `test/isis/isis-auth.ci`, `test/ospf/ospf-config.ci`, `test/parse/image-server-invalid-rescue-auth.ci`, `test/parse/redistribute-tag-filter.ci`, `test/web/web-error-fragment.wb`, `test/editor/pipe/set-cli-format.et`, `test/ui/cli-argument-length-refused.ci`, `test/ui/cli-argument-refused-plugin-route.ci`, `test/ipsec/ipsec-dataplane-show.ci`, `test/plugin/session-editor-commit-verify.ci` | the old words |
| `test/plugin/policy-test-errors.ci` | NEGATIVE `reject=... invalid value` |
| `test/parse/bmp-route-action-refuses-redistribute.ci`, `test/reload/bmp-route-action-redistribute-refused.ci` | `route-action`, `redistribute`, `monitor` (and `reload error`): the new body keeps all three; strengthened with the marker `is not an accepted value` and the `fix:` line |

Stay green unchanged: the RFC-tagged enum tests (`rfc7950_radius_auth_method_enum_test.go`, `rfc7950_isis_auth_algorithm_enum_test.go`, `rfc7950_validator_yang_test.go` `TestValidateTree_EnumViolation`, `TestValidateTree_FamilyModeEnum`, `TestValidateTree_AddPathDirectionEnum`), `radius_auth_method_eap_enum_test.go`; the pins of the kinds that keep today's text, `internal/component/config/yang/validator_test.go` `TestValidationErrorTypeContract` (`unknown error at test/value: unspecified`) and `internal/component/config/yang/rfc7950_reference_walk_test.go` (`HasPrefix "reference error at sys/mgmt: "`); `TestSectionValidationErrorRedactsASensitiveLeaf`, `TestCLIFormatNamesMatchTheSchemaLeaf`, `TestUpdateText_OriginValidation_YANG`, `TestSetCommandRejectsInvalidValue` and `TestInsertValidatesValueType` (type refusals keep their text), `TestEditorCommitConfirmedBlocksOnPeerPipelineRefusal` (renamed from `...OnMissingLeakFilter` by 74c830e0d2), `test/plugin/session-editor-commit-force.ci`, `test/parse/firewall-priority-out-of-range.ci`.

The inventory is re-run at implementation over every `_test.go` under `internal/`, `cmd/`, `pkg/` and over `test/` for `invalid enum`, `expected one of`, `must be one of`, `not a valid enumeration`, `(valid: `, `invalid format`, `needs a value`, `or one of`, `expected enum`, `does not match pattern`, `outside range`, `outside allowed`, `out of range`, `matches no member type`, `invalid key for`, `invalid peer name`, `write:`, `error at `, `NotContains`, `reject=`.

**Behavior to preserve:**
- Every value refused today stays refused on every path that refuses it, a typed `default` or `<key>#N` on a typed key included; every value accepted stays accepted, except the named changes below.
- Every entry stored today (anonymous `KeyDefault`, duplicate `<key>#N`, keyed) stays reachable by every lookup, and every stored session change file reads back as today.
- `ValidationError.Type`, `Got`, and `Expected` as comma-joined names (now in declared order); the text of the kinds that hold no operator error.
- `LineError`'s line and the JSON diagnostic keys.
- A type refusal (not a number, not a boolean, decimal64 fraction digits) keeps its text.
- Secret masking.
- `UnknownKeywordHint` and `retiredKeywords` unchanged.

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
| The editor's root-path commands | `internal/component/cli/model_commands_commit.go` `cmdDiscardSession`, `cmdDiscardPath`; `internal/component/cli/editor_commit.go` `memberOpFix` | `discard` takes a root path in every context, so its fixes are exact today | does not apply until Q-9: the owner's answer decides whether `set` gains the same root form |

New pieces, each with the reason no mechanism does it:

| New piece | Why no existing mechanism does it |
|-----------|-----------------------------------|
| A value-list renderer in `internal/core/suggest` (name column, aligned help, the alternation) | Both `config/yang` and `command` write the list, and `config/yang` imports `command`, so the shared writer must sit in a leaf package; no error path renders a value list today (completion renders its own menu) |
| A typed value fault in `command` (`ValueArgumentError`, beside `MissingArgumentError`): argument, typed value, problem, why lines, placeholder | `command` cannot import `config/yang`, so it cannot use `ValidationError`; `ArgumentFaults` reads its faults by type (`MissingArgumentError`) and needs the placeholder to write the whole command |
| The pattern format reader in `config/yang` (nearest typedef's `ze:help`, else its description's first sentence, else the leaf's `ze:help`) | No reader answers a typedef's text today; `GetHelpExtension` reads one statement |
| A decimal64 value type and range arm (`TypeDecimal64`, `LeafNode.FractionDigits`, an exported `ParseDecimal64` from the walk's `validateDecimal64`) | The parser has no decimal64 type (a defect, journaled in `declared-format-contradicts-payload.md`); the walk's own parse is reused, not copied |
| The in-entry key-leaf check | No walker refuses the first key leaf named inside an entry (`set <list> <key> <key-leaf> <v>`); `ParseListAddress` reads only non-first key leaves as address pairs |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A config value as text: a file line, a `set` line (editor, offline, web terminal, REST/gRPC/gNMI session), a web form post, or a command argument.

### Transformation Path
1. Schema build: `yangToLeaf`, the leaf-list builders (`ValueOrArrayNode`, `MultiLeafNode`, `BracketLeafListNode`) and the `ArgDef` builder store the enum names in declared order, the per-value help and the format text.
2. Value check: `ValidateLeafValue` (every parser, `ParseListAddress` pairs, `ValidateListKey`, the set surfaces) and the tree walk build a `ValidationError` holding the path segments it knows, the value, kind, accepted values with help, suggestion, secret flag and constraint message. `ValidateArgs` builds a `ValueArgumentError` from the same renderer.
3. Path: in the hierarchical parser every value and key arm builds the refusal with the leaf or list name alone and wraps it in `syntaxRefusal`; `prependSyntaxPath` collects the segments on unwind (keys and `ParseAt` context included), and at the parse root `Parse` and `ParseAt` rebuild it once with `WithPath`, which answers a new `ValidationError` and a new operator error whose problem names the space-separated path. The set parsers pass the tokens their walk consumed; the set surfaces pass the token path they hold; `EnsureListEntry` passes the list path. The walk threads segments from `ValidateTree`. `parseCommonAttributeText` re-roots to the command words.
4. Error chain: both parsers `LineError` → `ValidationError` → `*opererr.Error` (the hierarchical parser's `syntaxRefusal` is replaced at the parse root for this refusal and stays for the others); the set surfaces `ValidationError` → `*opererr.Error`. `ValidationError.Error()` renders the held operator error, whose fix is the set form; `Unwrap` answers it, so `errors.As` finds the `ValidationError` (JSON, frames) and the `*opererr.Error` (the web notice, `SplitLine`). A frame that holds another fix calls `WithFix`, which answers a copy with the same problem and why. The cause is not set on the operator error, so nothing is printed twice. A `ValidationError` of another kind holds no operator error and renders as today.
5. Rendering: `LineError` and `SplitLine` carry and print it on the file surfaces; the SSH editor moves a multi-line `m.err` to the viewport; `CommitRefusal` and `formatIssueList` write one issue per line; the web form reads `Problem`, `Why`, `Fix`; REST, gRPC and gNMI read `ValidateContent`'s joined error; JSON takes `code`, `expected`, `actual`, `path` from the `ValidationError`.
6. Masking: the secret flag is set where the refusal is built (`config.LeafHoldsSecret` over a `LeafNode`; the entry's `ze:sensitive`/`ze:bcrypt` through one reader on the walk), so the value slot holds `config.SecretDataPlaceholder` and no suggestion is computed.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG module → config schema | `yangToLeaf` copies names, help, format text | No |
| config → cli / web / api | the error crosses as `error`; `errors.As` recovers `ValidationError` and `*opererr.Error` | No |
| daemon → REST/gRPC/gNMI client | `ValidateContent` joins the refusals with `errors.Join`; `errors.As` answers the first; set refusals keep each surface's status mapping, gNMI's now wrapped as `InvalidArgument` | No |
| plugin or API command → `update text` | `errors.As` on the validator's answer, re-rooted | No |

### Integration Points
- `internal/core/opererr`, `internal/core/suggest`, `config.LineError`, `prependSyntaxPath`, `ConstraintMessages`, `ArgumentFaults`, `EnsureListEntry`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | each producer that refuses today builds the refusal where it refuses |
| No unintended coupling | Yes | `config`, `config/yang`, `command`, `cli`, `web` import only `opererr` and `suggest` for it, both core leaves |
| No duplicated functionality | Yes | no second error type; one refusal path; one creator; one fault list |
| Zero-copy preserved | Yes | error path only |
| Registration over hardcoding, outbound | Yes | no per-feature field or switch; `invalidListKeyError`'s `peer` special case is deleted |
| Registration over hardcoding, inbound | Yes | every value list comes from the YANG node at hand; no leaf name in any producer |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `suggest.Command`'s `len(input)/2` bound suits enum values | the CLI and the unknown-key refusal use it | a misleading or missing hint | `TestEnumRefusalSuggestion` (`requre` gives `require`, `banana` none) | unvalidated |
| A-2 | Every enum value declares `ze:help` | `./le doc yang-contract help-shape`, 2026-10-13: 857 of 857 | a value prints with no help | AC-4 test schema; the gate's counts | unvalidated |
| A-3 | REST, gRPC and gNMI meet a value refusal only at commit; a key refusal at set | the session set calls `SetValue` with no value check; `SetValue` creates through `EnsureListEntry` | a set-time value refusal elsewhere bypasses the frame | AC-8, AC-9, AC-43 | unvalidated |
| A-5 | A multi-line operator error renders on every surface that prints one | `ArgumentFaults` and the `}` refusal already reach these surfaces multi-line | lines run together, or the SSH prompt is lost | AC-1, AC-6, AC-20, AC-23, `TestEditorMultiLineErrorKeepsThePromptRow` | unvalidated |
| A-6 | No `.ci` drives a gNMI `Set`, a gRPC config session or a REST enum commit, so this spec builds the clients | 2026-10-13 grep; the REST fixture `plugin/api-config-commit-reject` is the shape | the clients do not reach the session manager | the three fixture clients | unvalidated |
| A-7 | "Friendly errors everywhere" covers command arguments | owner answer Q-3 "yes" | - | owner | confirmed |
| A-9 | The creators of list entries are the ones the table below lists, each with its verdict | `grep -rn '\.AddListEntry('` over non-test Go at `e660eff82e` | a creator keeps an unchecked key | the same grep at implementation is the gate: a creator not in the table refuses the phase until it has a verdict | unvalidated |
| A-10 | Every route that refuses a command line restates through `RestatedForLine` | `restateArgumentFaults` (dispatcher), `ValidateModelArgs` (`local_data.go`) | a route prints a value fault with no whole-command fix | AC-12 one-fault case on the dispatcher and a model route | unvalidated |

Creators that store a list entry, and the key verdict each takes:

| Creator | Verdict |
|---------|---------|
| `EnsureListEntry` (`internal/component/config/list_key.go`) and its callers that pass typed tokens (`Tokens >= 1`) | in scope: the new first-key check |
| `ListEntryDestination` (rename, copy) | in scope: the destination key is judged |
| `addParsedListEntry` and the two list-key arms (`internal/component/config/parser_list.go`) | in scope: the hierarchical parser's own `ValidateListKey` calls stay and now build the friendly refusal inside `syntaxRefusal`; `invalidListKeyError` is replaced, its `peer` special case deleted; its `KeyDefault` anonymous block is no typed key |
| set parser key-only arm (`internal/component/config/setparser.go`, `AddListEntry` after its own check) | in scope: routed through `EnsureListEntry`, its own check removed, since the creator judges the same raw token |
| `walkAndSetListEntry` (`setparser_entries.go`) and `walkAndSetWithMeta` (`setparser_meta.go`) | in scope: their raw-token `ValidateListKey` calls are removed; the creator judges every address with `Tokens >= 1`, a typed `default` or `x#3` included |
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

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a refusal reads wrong, or a path accepts or refuses a value it did not before (AC-14) |
| How is it reverted? | per-phase commits, each revertable; no config, wire or storage format changes |
| Who else touches this path? | `spec-bmp-route-action-redistribute-acts-as-monitor.md`; any spec adding an enum benefits |

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
| SSH editor `commit`, `commit confirmed 60`, `commit verify`, `show \| errors`, file and session mode, root and `edit` context | → | `validateCore` → `parseError` → `CommitRefusal`, `appendIssueSummary`, `formatIssueList` | `test/editor/validation/enum-refusal-commit.et` |
| SSH editor commit of a multi-leaf or bracket value the walk refuses | → | `validatePeer` → `formatPeerError` `ErrTypeEnum` arm | `TestFormatPeerErrorPassesTheRefusalThrough` |
| web terminal `set` (value, key, key-leaf form, in-entry form) and `commit` | → | `executeTerminalSet`; `runCommit` → `parseError` | `test/web/config-enum-refusal.wb`, `TestWebTerminalCommitShowsEnumRefusal` |
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
| AC-3 | a value far from every accepted value (`route-refresh banana`), and BMP `route-action redistribute` (the one accepted value is `monitor` since fa74dc4c00) | no `did you mean`; every value listed; the fix line holds the alternation, for BMP `fix: route-action monitor;` as the one-value alternation; the BMP `.ci` pins pass with the marker and the fix line added |
| AC-4 | an enum value with no `ze:help` (test schema) | its line is two spaces and the name, no trailing space; the other helps stay aligned |
| AC-5 | a member outside the enumeration on an inline-list child, a bracket-form `ValueOrArrayNode` (`storeValueOrArray`), an inactive bracket member (`parseInactiveBracketMember`), a set-format leaf-list (`validateValueOrArrayItems`), an enum-typed `MultiLeafNode` and `BracketLeafListNode`, and a freeform attribute (test schema when no shipped leaf is one) | the same refusal for that member, the full path named as in AC-1, declared order; no `(valid: `; the `MultiLeafNode` and `BracketLeafListNode` cases are refused by the parser, which accepted them before |
| AC-6 | SSH editor `set ... route-refresh requre` at the root and inside `edit bgp peer peer1`, and offline `ze config set` of the same | the AC-2 body; the fix is the command the operator types where they are: `fix: set bgp peer peer1 session capability route-refresh require` at the root and offline, `fix: set session capability route-refresh require` in the edit context; no `(expected enum)`. SSH: the whole refusal in the viewport (`.et` `viewport:contains`), the feedback row holds the problem line alone and the prompt stays on the last row (`TestEditorMultiLineErrorKeepsThePromptRow`). Offline: on stderr after `error: `, exit 2 |
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
| AC-23 | SSH editor commit, `commit confirmed 60`, `commit verify` and `show \| errors` of a candidate holding `route-refresh requre`, in file mode and in session mode, at the root and inside `edit bgp peer peer1`, also on an inactive (`nop`) leaf; a web terminal commit of the same | `CommitRefusal` keeps its header, ends it with no trailing `: `, and writes each issue on its own line; the SSH viewport shows the issues, the status row the header alone; in every mode the fix is the editor command typed where the operator is (`fix: set bgp peer peer1 session capability route-refresh require` at the root, `fix: set session capability route-refresh require` inside `edit bgp peer peer1`; a leaf outside the context waits on Q-9), never a file-form line or `nop`, and after typing it an inactive leaf stays inactive; `commit confirmed 60` refuses through `commitValidationRefusal` and writes no `cannot commit:` (`formatValidationErrors` deleted); no forced-command hint, since an error blocks. A refused member of a plain leaf-list in the candidate gets `fix: delete <path> <refused-member>` |
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
| `TestParserEnumRefusalNamesFullPath`, `TestParseAtEnumRefusalNamesContextPath`, `TestParserMissingEnumValueRefusal`, `TestEveryValueArmNamesTheFullPath` | `internal/component/config/parser_test.go` | AC-1, AC-2, AC-5, AC-18, AC-19, AC-25, AC-39; the path once, space-separated, keys included, no slash prefix, on `parseLeaf`, on `parseInactiveBracketMember`, on the two list-key arms and on each `parser_list.go` and `parser_freeform.go` value arm | |
| `TestParserRefusesUnionNonMember` | same | AC-13, AC-14 | |
| `TestSetParserEnumRefusalFrame` | `internal/component/config/setparser_test.go` | AC-24 incl. `nop` and the key-only line | |
| `TestInlineLeafListEnumRefusal`, `TestSetParserLeafListEnumRefusal` | `internal/component/config/parser_list_test.go`, `setparser_test.go` | AC-5 | |
| `TestEnsureListEntryJudgesNewKey`, `TestTypedDefaultKeyStaysRefused`, `TestParseListAddressPairRefusal`, `TestLookupWalkFindsEveryStoredKey`, `TestChangeFileReadsBackUnjudged` | `internal/component/config/list_key_test.go`, `internal/component/config/change_file_test.go` | AC-14, AC-25 to AC-32, AC-42, R-15 | |
| `TestValidateLeafValueRangeLengthPatternRefusal`, `TestValidateLeafValueDecimal64Range` | `internal/component/config/schema_test.go` | AC-35 to AC-37, AC-41 | |
| `TestValidateEnumerationSharedBody`, `TestSectionValidationErrorRendersRefusal`, `TestRefuseInvalidCustomSectionsOneRefusalPerLine` | `internal/component/config/yang/validator_test.go`, `internal/component/config/validate_sections_test.go` | AC-10, AC-16, AC-17 | |
| `TestLegacyHintPrecedesTheRefusal` | `internal/component/config/loader_test.go` | AC-10 hint placement | |
| `TestRunValidationParserEnumDiagnostic`, `TestValidateContentFramesEnumRefusal` | `internal/component/config/cli/cmd_validate_test.go` | AC-8, AC-11, AC-21 | |
| `TestValidateValueAtPathEnumRefusal`, `TestValidateValueAtPathRangePatternRefusal` | `internal/component/cli/completer_test.go` | AC-6, AC-14 | |
| `TestEditorMultiLineErrorKeepsThePromptRow`, `TestCommitRefusalWritesOneIssuePerLine`, `TestEditorFixIsTheCommandTypedHere`, `TestCliFormatRefusalUsesSharedBody`, `TestInEntryKeyLeafRefusal`, `TestFormatPeerErrorPassesTheRefusalThrough`, `TestKeyCheckOnInsertNestedActivation` | `internal/component/cli/*_test.go` | AC-6, AC-20, AC-23, AC-29 to AC-31, AC-34 | |
| `TestWebFormRefusesEnumAtSet`, `TestWebTerminalCommitShowsEnumRefusal`, `TestWebTerminalKeyLeafFormStoresKeyedEntry` | `internal/component/web/*_test.go` | AC-7, AC-23, AC-33 | |
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
| `enum-refusal-set`, `enum-refusal-load`, `enum-refusal-commit`, `range-length-pattern-set` | `test/editor/validation/` | AC-6, AC-19, AC-23, AC-25, AC-26, AC-34 | |
| `config-enum-refusal`, `config-range-length-pattern-refusal` | `test/web/` | AC-7, AC-27, AC-33, Q-8a, Q-8b | |
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
| `internal/component/cli/model.go`, `model_render.go`, `model_keys.go`, `model_load.go`, `model_commands_commit.go`, `model_commands_edit.go`, `validator.go` | the multi-line `m.err` route to the viewport; `set cli format` through `ValidateLeafValue`; one issue per line; `commit confirmed` through `commitValidationRefusal`; every editor fix written as the command typed in the operator's context (`WithFix`); `parseError` keeps the operator error; the in-entry check; `formatPeerError`'s `ErrTypeEnum` arm passes the `ValidationError` through with the peer context |
| `internal/component/web/cli_terminal.go`, `cli.go`, `handler_config_form.go`, `handler_config_entry.go`, `error_fragment.go`, `editor.go` | value checks at set (terminal, CLI bar, form); key checks through the creator (ADD, rename, URL keys); the key-leaf form through `CreateEntry`; the in-entry check; the notice reads `Problem`, `Why`, `Fix` through `errors.As`; the decimal64 field type stays string |
| `internal/component/gnmi/set.go` | `applyUpdate` wraps the `SetSegments` error as `InvalidArgument` (AC-43) |
| `internal/component/api/config_session.go`, `internal/component/api/grpc/server.go`, `internal/component/api/rest/server.go` | the set refusal passes through unchanged (the status mapping stays) |
| `internal/component/command/argvalidate.go`, `argfaults.go`, `argbind.go`, `argdef.go` | `ValueArgumentError` from the renderer; `verdict` answers an `ArgumentFaults` of one for a lone value fault; `newArgumentFaults` accepts one value fault and then writes its problem and why without the count header; `wholeCommand` writes a value fault's placeholder in place of its typed value and runs when every fault is missing or a value fault; a value fault's list entry is its problem and why, never its fix; the `ArgumentFaults` and `newArgumentFaults` docs that say "two or more" are rewritten |
| `internal/component/plugin/server/arg_faults.go`, `internal/component/plugin/server/command.go`, `internal/component/command/local_data.go` | `restateArgumentFaults` and `ValidateModelArgs` restate a one-fault refusal through `RestatedForLine`; the `several` variable and comments at the dispatcher's restate, in `adoptablePositional`, `argFaultOutsideSelector` and `validateModelArgsAs` are renamed and reworded, their verdicts unchanged |
| `internal/component/bgp/plugins/cmd/update/update_text.go`, `cursor.go` | re-root to the command words |
| `internal/component/bgp/config/loader_test.go`, `internal/component/plugin/server/send_test.go`, `rfc8907_command_test.go`, and the other pins | rewritten (Current Behavior) |
| `rfc/discrimination/rfc7950.json` | the stale records re-recorded with `./le rfc record` |

## Files to Create

- `internal/core/suggest/valuelist_test.go`, `internal/component/config/yang/refusal_test.go`, `internal/component/config/yang/format_test.go`
- `internal/test/fixture/plugin_fixture_enum_refusal.go` - three `le test fixture` clients registered beside `plugin/api-config-commit-reject`: `plugin/api-config-enum-refusal` (REST session set then commit), `plugin/grpc-config-enum-refusal` (gRPC `ZeConfigService` session set then commit) and `plugin/gnmi-set-enum-refusal` (a gNMI `SetRequest` over the vendored `github.com/openconfig/gnmi` proto, asserting the code and the message); each takes the path and the value, so one client serves every kind and the key cases
- the functional tests in the TDD plan

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no leaf, RPC or extension added |
| YANG validation constraints | No | enforcement exists; the refusal changes, plus the decimal64 range and leaf-list enum defect fixes |
| YANG custom validators | No | none added |
| CLI commands/flags | No | none added |
| CLI grammar (keyword before value) | N-A | no grammar change (Q-9 option (c) would add one, if chosen) |
| Editor autocomplete | No | completion already lists values with help, sorted |
| Functional test for new RPC/API | Yes | `test/plugin/api-config-enum-refusal.ci`, `test/plugin/grpc-config-enum-refusal.ci`, `test/plugin/gnmi-set-enum-refusal.ci` |
| Pipe completeness | N-A | no command output added |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no runtime dependency; the refusal is the operator's own input, answered at the command |
| Prometheus counters/metrics | N-A | none; the gNMI error metric already records `InvalidArgument` |
| BGP family surface | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | refusal wording; check `./le feature report` in Phase 3 |
| 2 | Config syntax changed? | Yes | `docs/architecture/config/syntax.md`: "Enumeration values" beside "Unknown keys", with range, length, pattern and the key check at creation; Phase 2 |
| 3 | CLI command added/changed? | No | none, unless Q-9 (c) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`: argument refusals, one fault restated; `docs/architecture/api/architecture.md`: the gNMI `Set` code; Phases 4 and 5 |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/config-editor.md` (Phase 3), `docs/guide/web-interface.md` (Phase 4) |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior changed or newly proven? | Yes | RFC 7950 Section 9.3 decimal64 range now enforced by the parser; `docs/architecture/config/yang-config-design.md`; Phase 2 |
| 10 | Test infrastructure changed? | Yes | three fixture clients; the page `ai/CODE-TO-DOCS.md` names for `internal/test/fixture/`; Phase 4 |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` (the refusal, its error chain, the format reader) and `docs/architecture/cli/error-surface.md` (multi-line errors in the SSH viewport); Phases 1 and 3 |
| 13 | Route metadata keys? | No | none |
| 14 | Prometheus counters? | No | none |
| 15 | Registered plugin, event, command or inventory changed? | No | none |
| 16 | Changed source referenced by doc anchors? | derived | `./le spec citation anchors spec plan/release/fixes/spec-config-enum-refusal-proposes-valid-values.md` per phase |
| 17 | Docs show examples for this area? | Yes | grep `docs/` for `invalid enum`, `expected one of`, `invalid peer name`, `write:` per phase |

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
against the creator table (A-9).

| Phase | Packages (files) | Tests | Red walk owed |
|-------|------------------|-------|---------------|
| 1 Refusal and schema facts | `internal/core/suggest`; `internal/component/config/yang` (`validator_error.go`, `validator.go`, `enum.go`, `constraint_message.go`, format reader); `internal/component/config` (`yang_schema.go` only) | `TestValueListRender`, `TestValidationErrorOperatorError`, `TestFrameKeepsProblemAndWhy`, `TestTypeFormatReader`, `TestYangToLeafCarriesEnumHelp`, `TestValueOrArrayEnumDeclaredOrder`, `TestMultiLeafAndBracketCarryEnums`, `TestValidateEnumerationSharedBody`; RFC-tagged enum tests, `TestValidationErrorTypeContract` and the reference walk test unchanged | `./le rfc record` for `validateEnumeration`, `walkTree`, `yangToLeaf` and the range, length, pattern producers; break the renderer's alignment and see `TestValueListRender` red |
| 2 Parsers and keys | `internal/component/config` (`schema.go`, `parser*.go`, `line_error.go`, `setparser*.go`, `list_key.go`, `change_file.go`, `validate_sections.go`, `loader.go`); `internal/component/bgp/config` (the peer-name pin) | the parser, set-parser, list-key, change-file, schema, loader and section tests; the `bgp/config` loader test; `test/parse/*`, `test/plugin/enum-refusal-daemon-start.ci`, `test/reload/enum-refusal-reload.ci`, the BMP pins | `./le rfc record` for `refuseInvalidCustomSections`; break the creator's key check and see `TestEnsureListEntryJudgesNewKey` red; judge `Tokens == 0` and see `TestChangeFileReadsBackUnjudged` and `TestLookupWalkFindsEveryStoredKey` red; widen the exemption to a typed `default` and see `TestTypedDefaultKeyStaysRefused` red; drop one value or key arm's `syntaxRefusal` and see `TestEveryValueArmNamesTheFullPath` red; drop `WithPath` and see `TestParserEnumRefusalNamesFullPath` red; break the decimal64 arm and see `TestValidateLeafValueDecimal64Range` red |
| 3 Offline and SSH editor | `internal/component/config/cli`; `internal/component/cli` | `cmd_validate`, `completer`, model and commit tests; `test/editor/validation/*`, `test/editor/pipe/set-cli-format.et`, `test/ui/config-*` | revert the viewport route and see `TestEditorMultiLineErrorKeepsThePromptRow` red; write the root-form fix inside a context and see `TestEditorFixIsTheCommandTypedHere` red |
| 4 Web, REST, gRPC, gNMI | `internal/component/web`; `internal/component/api`; `internal/component/gnmi`; `internal/test/fixture`; `cmd/ze/hub` (tests) | web, api and gnmi tests, `test/web/*`, the three `test/plugin/*-enum-refusal.ci` | remove the terminal `set` check and see `config-enum-refusal.wb` red, with the binary rebuilt; remove the `applyUpdate` wrap and see `TestSetUpdateRefusalIsInvalidArgument` red |
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
| Data flow | values and help read only through `EnumNamesDeclared`/`EnumValueSummaries` or the node fields they fill |
| Rule: testing | every rewritten assertion as strong as before, judged by `ze-test-judge`; negative pins keep their old needle |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| one refusal, no new error type | `gopls symbols internal/component/config/yang/validator_error.go`; the diff adds `ValueArgumentError` as a fault type only |
| old texts gone | the Critical Review grep |
| every path proven | every Wiring Test row's test exists and passes |
| records fresh | `./le rfc check` has no RFC 7950 `discrimination:` line |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the refused value is quoted (`%q`), so it cannot forge a line in a log or the web fragment |
| Error leakage | a sensitive value and its suggestion never appear (AC-16) |
| Resource exhaustion | the suggestion costs values times length, on an error path only |

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
| The fix is per surface: the frame that holds the surface calls `WithFix`; the problem and why are built once | a frame adds only the fact it holds (`docs/contributing/ze-go-style.md`) |
| Fix table: a hierarchical file and an editor `load` snippet `fix: <leaf> <value>;`; a set-format file `fix: <verb> <path> <value>`; the SSH editor (`set`, commit, every mode) the command typed in the operator's current context; offline, web terminal, web form, REST, gRPC, gNMI `fix: set <path> <value>`; a key `fix: rename <list-path> <old> to <key>`; a refused plain leaf-list member in a commit candidate `fix: delete <path> <member>`; a command the whole command with the placeholder | the fix is the exact text the operator types where they are; the web form's fix is the `set` line the web terminal on the same page accepts |
| The parser's path comes from `prependSyntaxPath` on unwind, every value and key arm joins it, and the path is written into the refusal once, at the parse root, by `WithPath` | it already names keys and the `ParseAt` context and costs nothing on success; the error is immutable, so it is rebuilt rather than prefixed, and the slash prefix is dropped |
| `invalidListKeyError` is replaced and its `peer` special case deleted | the friendly key refusal names the path (`bgp peer <key>`), so a word reserved for one list name is a per-leaf special case the Task forbids |
| No continuation-indent table | each surface prints an operator error as it prints one today; this spec changes only the `; ` joins, `ValidateContent`'s block indent and the SSH row |
| Declaration order everywhere a refusal lists values; completion keeps sorted order | the YANG order is the author's; completion order is a separate, tested fact |
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
| Q-9 | How should an SSH editor fix line name a config path outside the operator's current `edit` context? `discard` already takes a root path in every context (`cmdDiscardSession` and `cmdDiscardPath` hand the typed path to `DiscardSessionPath`/`DiscardPath` without the context), so its conflict fixes (`memberOpFix`, `discard <root path> ...`) are exact today. `set` does not: `cmdSet` always prepends `m.contextPath`, and `top` (`cmdTop`) takes no arguments, so no single `set` command reaches a path outside the context. The one existing refusal that prints a root-form `set` whatever the context is the missing-mandatory hint `(set bgp peer ...)` (`internal/component/cli/validator.go`), which inside a context is not the command typed; this spec would add commit refusals of a leaf outside the context. Which way? (a) keep the root-form `set` and say in the why that it is typed after `top`: no code change, but those fixes are not a command alone; (b) move the editor to the root context on every refusal that names a root path, so the root-form `set` is exact: changes navigation in the middle of an edit; (c) give `set` the root form `discard` already has, for example `set top <path> <value>`, and write the missing-mandatory hint and this spec's out-of-context fixes in it: a grammar addition that makes `set` match `discard` | (c): it makes `set` take a root path the way `discard` does, keeps every fix the exact command from any context, and corrects the one existing hint; (a) is smallest but leaves those fixes short of the rule; (b) surprises an operator in the middle of an edit | **Open: put to the owner 2026-11-12.** Until answered, AC-23 asserts the relative fix for a leaf under the context and the root form at the root; the missing-mandatory hint is unchanged |

Q-1 is unused. Two things are owed from the owner: approval of this design
(review round 22 found it ready) and the answer to Q-9. Until both arrive the
status stays `design`.

## Review Gate

### Design review history

| Round | Date | Result | Disposition |
|-------|------|--------|-------------|
| 1-17 | 2026-10-13 to 2026-10-14 | rounds 1 to 17 over the 2026-10-14 design; full table in git history at `09be99b7db` | every finding applied then; the design they reviewed is replaced by the 2026-11-12 rework |
| rework | 2026-11-12 | not reviewed | Disposition REWORK at `e660eff82e`: the refusal rebuilt on `opererr.Error` and `ValidationError`; existing mechanisms extended instead of new pieces; union cases moved to the parser; fix lines in the `fix:` layout; AC-42 added; BMP pins added; stale `./le verify` and single-commit instructions replaced |
| 18 | 2026-11-12 | 2 BLOCKER, 7 ISSUE, 3 NIT (full review of the rework) | All 12 verified at the producer and applied (I-6 with correction). B-1: the first-key check moved from `ParseListAddress` to the creator, with `TestLookupWalkFindsEveryStoredKey`. B-2: `ValueArgumentError`, an `ArgumentFaults` of one, the placeholder in `wholeCommand`, fault entries without `fix:`. I-1: the error chain with delegating `Error()`, `Unwrap`, `WithFix`. I-2: `prependSyntaxPath` reused, A-4 and the `Parser.path` extension deleted. I-3: A-8 and R-14 deleted, the `formatPeerError` arm kept, leaf-list nodes gain enum facts (AC-5, R-16). I-4: AC-43, the gRPC cases and client. I-5: Phase 4 after Phase 3. I-6: fixes typed in the current context, the leaf-list `delete` fix, Q-9 for a leaf outside the context. I-7: `ipsec-application-override.ci` pinned, AC-29 to AC-31 in a Go test. N-1 to N-3: the proof rests on `errors.As`, the hint before the refusal, `ValidateContent` joined with `errors.Join` |
| 19 | 2026-11-12 | 0 BLOCKER, 5 ISSUE, 2 NIT (narrow review of the round-18 edits) | All 7 verified at the producer and applied. I-1: exemption narrowed to `ListPathStep`'s anonymous arm, no suffix stripped, the key-only arm through the creator, `TestTypedDefaultKeyStaysRefused`. I-2: the creator table under A-9, the grep the gate. I-3: the other `ValidationError` kinds keep today's text, two pins added. I-4: `WithPath` at the parse root, space-separated, the slash prefix dropped. I-5: gNMI `applyUpdate` wrapped as `InvalidArgument`; REST 400 and gRPC `InvalidArgument` in the ACs. N-1: stale `several` comments listed. N-2: a resume-time refusal (AC-44, R-17), withdrawn in round 20. Q-9 rewritten for one editor convention |
| 20 | 2026-11-12 | 1 BLOCKER, 2 ISSUE, 1 NIT (narrow review of the round-19 edits) | All 4 verified at the producer and applied. B-1: `DiscardSessionPath`, every write-through `set` and every commit read the change file through `materializeMetaPaths`, and a change file holds `"default"` for an edit under an anonymous entry; stored keys are now read back unjudged, AC-44 and R-17 deleted, `TestChangeFileReadsBackUnjudged` added. I-1: the `parser_list.go` and `parser_freeform.go` value arms return a plain `p.errorf`; each now wraps its refusal in `syntaxRefusal`, with `TestEveryValueArmNamesTheFullPath`. I-2: `discard` takes a root path in every context; Q-9 rewritten around `set` alone. N-1: the `ensureFlexListEntry` row cites the `FlexNode` declaration |
| 21 | 2026-11-12 | 0 BLOCKER, 1 ISSUE, 1 NIT (narrow review of the round-20 edits) | Both verified at the producer and applied. I-1: three more arms returned a plain `p.errorf`: `parseInactiveBracketMember` (`parser.go`, over `validateValuePatterns`) and the two `parser_list.go` list-key arms (`ValidateListKey` into `invalidListKeyError`); all three are added to Current Behavior, the creator table and Files to Modify, each wrapped in `syntaxRefusal`, and covered by `TestEveryValueArmNamesTheFullPath`; `invalidListKeyError` is replaced and its `invalid peer name:` arm deleted as a per-leaf special case, its pin (`internal/component/bgp/config/loader_test.go`) added; AC-25 gains the file case and AC-39 the hierarchical key case; the `parser_freeform.go` `KeyType` check is named exempt (`exabgp.yang` inline lists only). N-1: the creator rule is stated once, `ListAddress.Tokens == 0` is not judged, with its two builders (`ListPathStep`'s anonymous arm, `materializeMetaPaths`) and the typed builders that set `Tokens >= 1`. One short round 22 over these two points is owed |
| 22 | 2026-11-12 | 0 BLOCKER, 0 ISSUE, 1 NIT (short review of the round-21 points) | The NIT verified and applied: the `invalid peer name` pin in `internal/component/bgp/config/loader_test.go` asserts only under `if err != nil`, and `validatePeerName` (`internal/component/bgp/config/resolve.go`) writes the same words for a key that is an IP address, so the pin inventory now has the implementation first establish which producer fires, rewrite the pin only if it is the parser arm, and put the conditional assert to the `ze-test-judge`. The reviewer states the design is ready for owner approval with Q-9 open; no further round is owed |

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
| Verdict | REWORK (design reworked; review rounds 18 to 22 applied; ready for owner approval, Q-9 open) |
| Evidence 1 | 3e94cb7c40 (spec-operator-errors-name-the-fix, closed dc5b80f1b6) made every operator refusal an `opererr.Error` with a `fix: <command>` last line; `ai/rules/user-facing-errors.md` requires it |
| Evidence 2 | 238bdb62e9 added the union arm to `ValidateLeafValue` (`validateUnionValue`), contradicting the old AC-10, AC-13, AC-14, AC-23 and R-4 |
| Evidence 3 | 3e94cb7c40 added `config.LineError`/`SplitLine` (deleting `withoutLinePrefix`), `ConstraintMessages`, and `ArgumentFaults` (with 3af4462e82, 37c3a2abb1); `syntaxRefusal` and `prependSyntaxPath` exist |
| Evidence 4 | the reload prefix is `reload: parse config: line N:` (`test/reload/reload-parse-error-single-prefix.ci`) |
| Evidence 5 | 163d9a07c8, 98394e78ce, d2f0e2cb08: multi-leaf-key addressing, one address reader `ParseListAddress`, one creator `EnsureListEntry` |
| Evidence 6 | still open at HEAD: texts 1 to 11, the `; ` joins, no web set check, the completer heuristic, the ranged decimal64 defect |
| Evidence 7 | fa74dc4c00 removed `enum redistribute` from BMP `route-action`; the two BMP `.ci` pin only `route-action`, `redistribute`, `monitor` |

The goal still holds: every refusal in the Current Behavior table reads at HEAD
as listed, so the defect stands at its producers. The 2026-10-14 design no
longer fit main. It built a refusal type and frames beside the operator error
that now governs every refusal, it rebuilt a line error and a path stack that
exist, and it rested several acceptance criteria on the parser accepting union
non-members, which it no longer does. The rework rebuilds the design on those
mechanisms, and the findings of rounds 18 to 22 are applied. Round 22 found the
design ready; owner approval and the Q-9 answer are owed.
