# Spec: config-enum-refusal-proposes-valid-values

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner decision, 2026-10-13: error messages must be user friendly and "propose what
is known is valid from the yang". Direction chosen by the owner, 2026-10-13,
verbatim: "M2 go with (c), friendly errors everywhere". Option (c) is one generic
friendly refusal for every YANG enum, on every path: the setting path, the refused
value, the accepted values with their YANG help, and the closest match where a
helper exists.

Today a value outside a YANG enumeration is refused with
`invalid enum: "redistribute" (expected one of: monitor)`: no setting path, no
meaning for each accepted value, no closest-match hint. Worse, the same refusal has
six different texts depending on the path the operator took (table in Current
Behavior), and two paths (the SSH editor `set`, the web form) do not list the
accepted values at all, or do not refuse at `set` time.

Goal: ONE generic refusal for every YANG enumeration, built from the schema alone,
reached through every path (config file load, `ze config validate`, offline
`ze config set`, editor `set` and `commit`, web UI, REST API and gNMI `Set`, reload,
and operational command arguments, which are YANG enums too). It:

| Element | Source |
|---------|--------|
| names the setting by its config path, list keys included | the path the caller already holds |
| quotes the refused value | the input |
| lists every accepted value, in YANG declaration order | the YANG enumeration, never a hand list |
| gives each accepted value its one-line `ze:help` where the YANG declares one | `yang.EnumValueSummaries` (the one reader of an enum value's `ze:help`) |
| offers "did you mean" when one accepted value is close | `suggest.Command` (`internal/core/suggest`), the distance the CLI and the unknown-key refusal already use |

No per-leaf special case. Known dependant:
`plan/immediate/spec-bmp-route-action-redistribute-acts-as-monitor.md` (owner answer
Q-2 there) removes `enum redistribute` from `route-action` and relies on this
message to refuse it.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/user-facing-errors.md` - binds every message in this spec
  → Constraint: problem first, then why it matters, then the fix on a line of its own; plain Simplified Technical English; no color carries meaning alone; JSON output carries no color.
  → Constraint: the message names what the operator typed (the value) and where (the path), never only the library call; the binding covers CLI, web UI, logs, API and gNMI answers alike.
- [ ] `ai/rules/cli.md` - validation errors surfaced to an agent
  → Constraint: every validation error surfaced to an agent carries a stable diagnostic code; the enum refusal's code is `config-yang-enum` (`diagnostic.CodeConfigYANGEnum` in `internal/core/diagnostic/codes.go`). JSON keys stay kebab-case.
- [ ] `ai/rules/config.md` - enum help text
  → Constraint: "An `enum` value carries its one-line text as `ze:help` alone." So the per-value text is the enum's `ze:help`; an enum `description` is read by no surface (`schemaEnums` comment in `internal/le/doc/yangcontract/helpshape_schema.go`) and this spec does not start reading it.
  → Constraint: "An unknown key MUST fail at every level with the closest valid key suggested" is already met by `UnknownKeywordHint`; the enum refusal follows the same shape (same distance, same "did you mean" wording).
- [ ] `docs/architecture/config/syntax.md` - "Retired keywords" and "Unknown keys" sections
  → Decision: the unknown-key refusal already says `(did you mean router-id?)` with `suggest.Command`; the enum refusal reuses that helper and wording, so the two refusals read alike.
  → Constraint: this page MUST gain an "Enumeration values" subsection in the same phase that changes the message (`ai/rules/documentation.md`).
- [ ] `docs/architecture/config/yang-config-design.md` - YANG validator
  → Constraint: `validateEnumeration` is documented there; its `ValidationError` keeps `Type == ErrTypeEnum` and `Expected` (comma-joined names), which the RFC-tagged tests read.
- [ ] `docs/guide/config-editor.md` - masking of refusals (section on secrets in command answers)
  → Constraint: a refusal on a sensitive leaf passes through `DisplayMessageAtPath` / `maskSecretInMessage`; the new message must still be masked by them (they replace the value and its `%q` form).

### RFC Summaries (Scope: protocol)
- N-A: no wire behaviour changes. RFC 7950 Section 9.6 (enumeration) is the obligation the RFC-tagged tests prove (`RFC7950-9.6-1`); it says what is accepted, not how the refusal reads. The tagged tests assert `ErrTypeEnum` and `Expected`, never the message text, and this spec keeps both.

**Key insights:**
- Six producers, six texts, one fact (the enumeration). Two of them (`ValidateLeafValue` in `schema.go`, `validateInlineListChildValue` in `parser_list.go`) are the ones the operator meets on every file path.
- `suggest.Command` exists; no new edit-distance helper is needed.
- `yang.EnumValueSummaries(entry)` is the one reader of per-value `ze:help`; `LeafNode` carries `ShortHelp` for the leaf but NO per-value help today, so `yangToLeaf` must store it.
- `config/yang` imports `command`, and `config` imports `config/yang`, so the shared renderer must live in a leaf package all producers can import: `internal/core/suggest` (imports nothing from ze).
- The parser holds no path stack (`Parser` fields: `schema`, `tok`, `warnings`), so the refusal there names only the leaf today.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/schema.go` (`ValidateLeafValue`) - checks type, then `node.Enums`, then ranges, lengths, patterns. Enum arm returns `invalid enum: %q (expected one of: %s)`; knows no path. `LeafNode` has `Enums []string` (declared order), `ShortHelp`, `Description`, no per-value help.
  → Constraint: ordering type, enum, range, length, pattern is kept.
- [ ] `internal/component/config/parser_list.go` (`validateInlineListChildValue`) - the `ValueOrArrayNode.ValidValues` arm returns the same text for one member of an inline leaf-list.
- [ ] `internal/component/config/parser.go` (`parseLeaf`, `leafValueFix`, `errorf`) - wraps as `line N: invalid value for <leaf>: <err>` and, for an enum leaf, appends a `write: <leaf> <enable|...>;` line (`enable`/`disable` picked for a boolean spelling).
  → Constraint: the `write:` fix line and its boolean-spelling choice are preserved (pinned by `TestEnumLeafRefusalNamesTheReplacementLine` and two `.ci`).
- [ ] `internal/component/config/yang_schema.go` (`yangToLeaf`) - fills `Enums` via `yang.EnumNamesDeclared`; `ShortHelp` via `yang.GetHelpExtension`.
- [ ] `internal/component/config/yang/enum.go` - `EnumNamesDeclared`, `EnumValueNames` (sorted, union-aware, the completion order), `EnumValueSummaries` (the one per-value `ze:help` reader, union and typedef aware).
- [ ] `internal/component/config/yang/validator.go` (`validateEnumeration`, `ValidationError.Error`) - answers `ValidationError{Type: ErrTypeEnum, Message: "value %q is not a valid enumeration value", Expected: names joined ", ", Got}`, rendered `enum error at <dotted path>: <message>`.
- [ ] `internal/component/config/cli/cmd_validate.go` (`runValidation`, `ValidateContent`, `yangErrorCode`, `yangRepair`) - parses first (a parser error gets code `config-parse`, no expected/actual), then `ValidateCustomSections` (YANG errors get `config-yang-enum`, expected, actual, repair "Replace value with one of the allowed enum values"). `ValidateContent` joins error diagnostics with a newline and two spaces, and is the commit hook of REST and gNMI (`configValidationHook` in `cmd/ze/hub/api.go`, also wired in `cmd/ze/hub/service_gnmi.go`).
- [ ] `internal/component/cli/completer.go` (`ValidateValueAtPath`, `TypeHint`) and `completer_validate.go` (`validateLeafValue`) - editor `set` refusal: `invalid value %q for <leaf> (expected enum)`. `TypeHint` answers the word `enum`, so the editor names NO accepted value.
- [ ] `internal/component/cli/model_commands_edit.go` - editor `set` calls `ValidateValueAtPath`, then masks with `DisplayMessageAtPath`.
- [ ] `internal/component/cli/validator.go` (`formatPeerError`) - editor/web commit refusal for a peer leaf: `peer X: "<leaf>" must be one of: <names> (got "<v>")`; other YANG errors render `<field>: <Message>`.
- [ ] `internal/component/web/handler_config_form.go` and `handler_config_entry.go` - web set validates with `config.ValidateValue(type)` only: an enum value outside the set is NOT refused at set, only at commit.
- [ ] `internal/component/api/config_session.go` - REST/gNMI session set calls `Editor.SetValue` with no value check; the refusal arrives at commit through `ValidateContent`.
- [ ] `internal/component/command/argvalidate.go` (`validateEnum`) - operational command enum argument: `invalid value %q, expected one of: %s`. ArgDef enum values are built from YANG (`internal/component/config/yang/command.go`, `NewEnumArg` in `argdef.go`).
- [ ] `internal/core/suggest/suggest.go` (`Command`) - answers the closest candidate within distance `len(input)/2`, else "".
- [ ] `internal/component/config/retired.go` - `UnknownKeywordHint` (retired hint, else `suggest.Command`), `retiredKeywords` hand map for retired KEYWORDS.

**The six texts today:**

| # | Path | Producer | Text |
|---|------|----------|------|
| 1 | file load, `ze config validate`, reload, offline `ze config set`, REST/gNMI commit | `ValidateLeafValue` via parser / set parser | `line N: invalid value for route-action: invalid enum: "redistribute" (expected one of: monitor)` plus `write:` line |
| 2 | same, one member of an inline leaf-list | `validateInlineListChildValue` | `invalid enum: "x" (expected one of: a, b)` |
| 3 | tree walk (`ValidateCustomSections`, commit) when the parser did not refuse first | `validateEnumeration` | `enum error at a.b.c: value "x" is not a valid enumeration value` |
| 4 | editor/web commit, peer leaf | `formatPeerError` | `peer X: "leaf" must be one of: a, b (got "x")` |
| 5 | editor `set` | `ValidateValueAtPath` | `invalid value "x" for leaf (expected enum)` |
| 6 | operational command argument | `validateEnum` | `invalid value "x", expected one of: a, b` |

**Tests that pin the current text (none RFC-tagged on the text):**

| Test | Pins | RFC-tagged? |
|------|------|-------------|
| `internal/component/config/parser_leaf_fix_test.go` `TestEnumLeafRefusalNamesTheReplacementLine` | `invalid value for route-refresh: invalid enum: "true"`, `invalid enum: "false"`, `invalid enum: "maybe"`, `invalid enum: "require"` (link-local-nexthop), and the `write:` lines | No |
| `internal/component/config/schema_bool_mode_test.go` `TestASN4IsACapabilityMode`, `TestBooleanLeafRefusesCapabilityMode`, `TestCapabilityModeLeavesShareOneTypedef` | contains `invalid enum` | No |
| `internal/component/config/enum_test.go` `TestYANGLeafRestrictionsValidateParse` | contains `invalid enum` | No |
| `internal/component/plugin/server/send_test.go` `TestSendArgumentsAreDeclared` | `expected one of: hex, b64`, `expected one of: open, update, notification, keepalive, route-refresh`, `expected one of: text, hex, b64, cursor` (command args, text 6) | No |
| `internal/component/command/argvalidate_test.go` `TestValidateArgStringEnum` | refusal contains the valid value `summary` | No |
| `internal/component/plugin/server/rfc8907_command_test.go` `TestMissingMandatoryOutranksAnUnplaceableToken` | comment only (`PREVENTS: invalid value "update", expected one of: import, export`); no assertion on the text. The file's two `RFC8907-8.3-4` tags sit on accounting tests that do not read the text | tags in file, not on this text |
| `test/parse/asn4-refuses-boolean-spelling.ci` | `invalid value for asn4`, `expected one of`, `enable, disable, require, refuse` | No |
| `test/parse/capability-leaf-refuses-presence-spelling.ci` | `invalid value for route-refresh`, `enable, disable, require, refuse`, `route-refresh enable;` | No |
| `test/isis/isis-auth.ci` | `invalid value for algorithm` | No |

RFC-tagged tests that touch enum refusals but NOT the text (must stay green, untouched): `rfc7950_radius_auth_method_enum_test.go` (`RFC7950-9.6-1`, asserts `ErrTypeEnum` and `Expected` contains `pap`, `chap`), `rfc7950_isis_auth_algorithm_enum_test.go` (`RFC7950-9.6-1`, asserts `ErrTypeEnum`), `radius_auth_method_eap_enum_test.go` (untagged sibling, asserts `Expected` contains each value), `rfc7950_validator_yang_test.go` `TestValidateTree_EnumViolation` (asserts `ErrTypeType` and the path).

**Behavior to preserve:**
- Every value outside an enumeration stays refused on every path that refuses it today; the verdict never changes, only the words.
- `ValidationError.Type == ErrTypeEnum`, `Expected` = accepted names joined with `, `, `Got` = the value (read by the RFC-tagged tests and by `cmd_validate` JSON `expected`/`actual`).
- The parser's `line N:` prefix and its `write:` fix line, including the `enable`/`disable` choice for a boolean spelling.
- Validation order type, enum, range, length, pattern.
- Secret masking of every refusal (`DisplayMessageAtPath`, `maskSecretInMessage`).
- `UnknownKeywordHint` and `retiredKeywords` (keywords, not values) unchanged.

**Behavior to change:**
- All six texts become one generic refusal (Design).
- Editor `set` names the accepted values (today: the word `enum`).
- Web form `set` refuses an enum value at set time with the generic refusal (today: only at commit).
- `ze config validate` reports a parser enum refusal with code `config-yang-enum` and fills `expected`/`actual` (today: `config-parse` with neither).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A config value as text: a config file line, a `set` line (editor, offline `ze config set`, REST/gNMI session set), a web form post, or an operational command argument.

### Transformation Path
1. Schema build: `yangToLeaf` reads the enumeration names (declared order) and, new, the per-value `ze:help` (`yang.EnumValueSummaries`) onto `LeafNode`; the leaf-list builder does the same for `ValueOrArrayNode`; the command ArgDef builder does the same for an enum argument.
2. Value check: `ValidateLeafValue` (parser, set parser, meta parser, freeform parser, list keys), `validateInlineListChildValue`, `validateEnumeration` (tree walk), `ValidateValueAtPath` (editor), the web form sites, `validateEnum` (command args) each build ONE typed refusal value (path, value, accepted values with help, suggestion) when the value is not accepted.
3. Rendering: the typed refusal's `Error()` gives the shared text; each surface adds only its own frame (`line N:` and `write:` for a file, `set <path> <value>` for the editor, the diagnostic code and `expected`/`actual` for `ze config validate` JSON).
4. Masking: unchanged, applied by the surface on the rendered text.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG module → config schema | `yangToLeaf` copies enum names and per-value `ze:help` | No |
| config → cli editor / web / api | typed error crosses as `error`; `errors.As` recovers the fields | No |
| daemon → REST/gNMI client | commit error text from `ValidateContent` | No |

### Integration Points
- `internal/core/suggest` - gains the shared refusal type and its renderer, next to `Command`.
- `yang.EnumValueSummaries`, `yang.EnumNamesDeclared` - the only sources of names and help.
- `diagnostic.CodeConfigYANGEnum` - the code `ze config validate` attaches.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | every producer that refuses today builds the refusal where it refuses; no new validation layer |
| No unintended coupling (components stay isolated) | Yes | the type lives in a core leaf package; `config/yang`, `config`, `command`, `cli`, `web` import it, none imports another for it |
| No duplicated functionality (extends existing, does not recreate) | Yes | reuses `suggest.Command` and `yang.EnumValueSummaries`; deletes five of six formatters |
| Zero-copy preserved where applicable (refs, not copies) | Yes | error path only; nothing on a hot path |
| Registration over hardcoding, outbound | Yes | no per-feature field, switch or factory is added |
| Registration over hardcoding, inbound | Yes | searched `invalid enum`, `expected one of`, `must be one of`, `not a valid enumeration value`, `(expected enum)`: every list of values in the new text comes from the YANG enumeration on the node at hand; no leaf name appears in any producer |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `suggest.Command` with its `len(input)/2` bound is a good enough closest match for enum values | it is the CLI's and the unknown-key refusal's helper (`docs/architecture/config/syntax.md`, "Unknown keys") | a far value gets a misleading hint, or a near one none | `TestEnumRefusalSuggestion` table (`requre` gives `require`, `redistribute` gives none) | unvalidated |
| A-2 | Most enum values declare `ze:help` | 607 `enum x {` blocks and no bare `enum x;` in `internal/**/*.yang` (grep, 2026-10-13); the help-shape gate counts values without a summary but does not refuse them | values print with no help text; the list still names them | `./le doc yang-contract help-shape` counts `SchemaEnumValuesWithSummary` against `SchemaEnumValues` | unvalidated |
| A-3 | REST and gNMI receive the refusal only at commit, through `ValidateContent` | `ConfigSessionManager` set calls `SetValue` with no check; `configValidationHook` | a set-time refusal elsewhere would bypass the renderer | wiring tests for AC-8, AC-9 | unvalidated |
| A-4 | The parser can carry a path stack without changing its output for valid configs | `Parser` holds no path today; `parseBody`/`parseContainer` recurse per block and list entry | path wrong or missing list keys | `TestParserEnumRefusalNamesFullPath` with a list key in the path | unvalidated |
| A-5 | A multi-line refusal renders readably in the web error fragment, the toast and `ValidateContent`'s join | `WriteOOBError` and the toast take a string; `ValidateContent` indents only the first line of each diagnostic | lines run together or lose indentation | AC-7 `.wb` asserts each accepted value is visible; AC-8 asserts every line | unvalidated |
| A-6 | A gNMI `Set` can be driven from a `.ci` test | `test/plugin/` holds `gnmi-show.ci` and `mgmt-guard-gnmi-*.ci` | no `.ci` reaches gNMI Set | first step of Phase 5: find the harness; if none exists, the proof is `internal/component/gnmi/integration_test.go` and the gap goes to the owner, never dropped | unvalidated |
| A-7 | "Friendly errors everywhere" covers operational command enum arguments | owner answer 2026-10-13; those arguments are YANG enums (`config/yang/command.go` builds `NewEnumArg` from YANG) | AC-12 and Phase 6 move to their own spec if the owner says otherwise (Q-3) | owner confirmation of Q-3 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A leaf with dozens of values (address families, log levels) prints a very long list | refusal over about 20 lines in a `.ci` | Q-4; recommended: print every value, no cap, since `?` shows the same list |
| R-2 | Changing `ValidateLeafValue`'s signature to take the path touches every caller (eight in `config`, one in `cli`) | compile errors | intended: the compiler finds every caller, none can forget the path |
| R-3 | A sensitive enum leaf leaks the value through the suggestion line | masking test | the value appears only in the quoted slot `maskSecretInMessage` already replaces; `TestEnumRefusalMasksSensitiveValue` |
| R-4 | Union leaves with enum members are not enum-checked by `ValidateLeafValue` today (`yangToLeaf` sets `Enums` only for a `Yenum` type) | union case refused with a type text | Q-2; recommended yes, AC-13 |
| R-5 | `ze config validate` JSON code for a parser enum refusal changes from `config-parse` to `config-yang-enum` | `test/ui/*` `.ci` | only `invalid syntax here` cases pin `config-parse`, and those stay `config-parse`; a consumer keyed on `config-parse` for an enum value is the case the code exists to separate |
| R-6 | The RFC-tagged enum tests break through a change to `Expected` | those tests red | `Expected` keeps the comma-joined names; help text goes in a separate field |
| R-7 | The BMP dependant lands before this spec and its `.ci` pins the old text | its AC-1 `.ci` contains `invalid enum` | the BMP spec pins only `route-action`, `redistribute` and `monitor`, words both texts carry; Phase 7 re-checks it |
| R-8 | A rewritten pinned test asserts less than before | review | each rewrite asserts the value, the path and every accepted value (`ai/rules/testing.md`) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a refusal text reads wrong, or a path accepts a value it refused before (the verdict regression AC-14 guards) |
| How is it reverted? | single commit revert; no config, wire or storage format changes |
| Who else touches this path? | `plan/immediate/spec-bmp-route-action-redistribute-acts-as-monitor.md` (relies on the message); any spec adding an enum (benefits automatically) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate -` on a file with a bad enum | → | `parseLeaf` → `ValidateLeafValue` → shared refusal | `test/parse/enum-refusal-proposes-valid-values.ci` |
| `ze config validate -` on an inline leaf-list member | → | `validateInlineListChildValue` | `test/parse/enum-refusal-inline-leaf-list.ci` |
| daemon start on a file with a bad enum | → | loader → parser | `test/plugin/enum-refusal-daemon-start.ci` |
| reload to a file with a bad enum | → | reload → parser | `test/reload/enum-refusal-reload.ci` |
| offline `ze config set` with a bad enum | → | set parser → `ValidateLeafValue` | `test/parse/enum-refusal-config-set.ci` |
| SSH editor `set` with a bad enum | → | `ValidateValueAtPath` | `test/editor/validation/enum-refusal-set.et` |
| web form post with a bad enum | → | web form value check | `test/web/config-enum-refusal.wb` |
| REST config session commit | → | `ValidateContent` | `test/plugin/api-config-enum-refusal.ci` |
| gNMI `Set` | → | `ValidateContent` | `test/plugin/gnmi-set-enum-refusal.ci` (A-6) |
| operational command enum argument | → | `validateEnum` | `test/plugin/command-arg-enum-refusal.ci` |

## Acceptance Criteria

The example leaf is `bgp peer peer1 session capability route-refresh` (values
`enable`, `disable`, `require`, `refuse`, each with a `ze:help`).

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | file config with `route-refresh maybe;` under peer `peer1`, `ze config validate` | exit 1; stdout carries the line number, the path `bgp peer peer1 session capability route-refresh`, the quoted value `"maybe"`, a heading for the accepted values, and one line per value holding the value name and its `ze:help` text, in YANG declaration order; the `write: route-refresh <enable\|disable\|require\|refuse>;` line stays |
| AC-2 | same with `route-refresh requre;` | AC-1 plus a line `did you mean "require"?` placed before the value list |
| AC-3 | value far from every accepted value (`redistribute` against `monitor`) | no "did you mean" line; the value list is still printed |
| AC-4 | enum value with no `ze:help` | its line carries the name alone; no placeholder, no empty separator |
| AC-5 | inline leaf-list member outside its enumeration | the same refusal for that member, path named |
| AC-6 | SSH editor `set ... route-refresh requre` | the same body as AC-2 (path, value, suggestion, values with help), and a fix line `set <path> require`; `(expected enum)` no longer appears |
| AC-7 | web form posts `requre` for that leaf | refused at set, before commit, with the AC-2 body in the field error; nothing is stored in the working tree |
| AC-8 | REST config session sets `requre` then commits | commit refused; the error text holds the AC-2 body; active config unchanged |
| AC-9 | gNMI `Set` of `requre` | `Aborted`; status message holds the AC-2 body |
| AC-10 | daemon start and reload with AC-2's file | start refuses with the AC-2 body; reload refuses with it and the running config is unchanged |
| AC-11 | `ze config validate` JSON output on AC-2's file | the diagnostic has code `config-yang-enum`, `expected` = `enable, disable, require, refuse`, `actual` = `requre`, a path naming the leaf, and the message equal to the text body |
| AC-12 | operational command with a bad enum argument, for example `send bgp 192.0.2.1 raw gzip DEADBEEF` | refusal names the command and argument, quotes `gzip`, lists `hex` and `b64` with their `ze:help`, and suggests when close |
| AC-13 | (Q-2 yes) union leaf whose members include an enumeration, value matching no member | refusal lists the enum members with help and names the other member types in words |
| AC-14 | every value accepted before this change, on every path | still accepted (verdict unchanged); every value refused before is still refused |
| AC-16 | sensitive enum leaf (a real one if it exists, else a test schema) refused | the refused value does not appear in any rendering; the accepted values still do |
| AC-17 | YANG validator tree walk (`validateEnumeration`) refusal | `Type` stays `ErrTypeEnum`, `Expected` stays the comma-joined names, `Message` is the shared body; RFC-tagged tests unchanged and green |

AC-15 is unused: it was a removed-value reason, which the owner's direction (c) leaves out.

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | typos an enum in a config file and validates it | file → parser → `ValidateLeafValue` → refusal | `test/parse/enum-refusal-proposes-valid-values.ci` |
| 2 | types a bad enum in the SSH editor | editor `set` → `ValidateValueAtPath` | `test/editor/validation/enum-refusal-set.et` |
| 3 | picks a bad value through the web UI | form → `ValidateLeafValue` | `test/web/config-enum-refusal.wb` |
| 4 | pushes config through REST or gNMI | session set → commit → `ValidateContent` | `test/plugin/api-config-enum-refusal.ci`, `test/plugin/gnmi-set-enum-refusal.ci` |
| 5 | reloads a file with a bad enum | reload → parser | `test/reload/enum-refusal-reload.ci` |
| 6 | mistypes an enum argument of an operational command | dispatcher → `validateEnum` | `test/plugin/command-arg-enum-refusal.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestValueRefusalRender` | `internal/core/suggest/refusal_test.go` | body layout: path, quoted value, suggestion line, value lines with and without help (AC-1 to AC-4) | |
| `TestEnumRefusalSuggestion` | `internal/core/suggest/refusal_test.go` | `requre` gives `require`, `monitr` gives `monitor`, `redistribute` gives none (A-1) | |
| `TestYangToLeafCarriesEnumHelp` | `internal/component/config/yang_schema_test.go` | `LeafNode` holds per-value `ze:help` in declared order, from `EnumValueSummaries` | |
| `TestValidateLeafValueEnumRefusal` | `internal/component/config/enum_test.go` | typed refusal recovered with `errors.As`, fields filled, path as passed | |
| `TestParserEnumRefusalNamesFullPath` | `internal/component/config/parser_test.go` | path includes list key `peer1` (A-4) | |
| `TestInlineLeafListEnumRefusal` | `internal/component/config/parser_list_test.go` | AC-5 | |
| `TestValidateEnumerationSharedBody` | `internal/component/config/yang/validator_test.go` | AC-17 | |
| `TestValidateValueAtPathEnumRefusal` | `internal/component/cli/completer_validate_test.go` | AC-6 body and fix line | |
| `TestFormatPeerErrorEnumUsesSharedBody` | `internal/component/cli/validator_test.go` | text 4 replaced | |
| `TestWebFormRefusesEnumAtSet` | `internal/component/web/handler_config_form_test.go` | AC-7 at handler level | |
| `TestRunValidationParserEnumDiagnostic` | `internal/component/config/cli/cmd_validate_test.go` | AC-11 | |
| `TestEnumRefusalMasksSensitiveValue` | `internal/component/config/mask_test.go` | AC-16, R-3 | |
| `TestValidateEnumArgRefusal` | `internal/component/command/argvalidate_test.go` | AC-12 | |
| Rewritten: the six pinning Go tests in Current Behavior | as listed | assert the new body (value, path, every accepted value) instead of the old words; each assertion stays at least as strong (R-8) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| suggestion distance | 0 to `len(input)/2` | distance `len(input)/2` suggests | N/A | `len(input)/2 + 1` gives no suggestion |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `enum-refusal-proposes-valid-values` | `test/parse/enum-refusal-proposes-valid-values.ci` | AC-1, AC-2, AC-3, AC-11 via `ze config validate` | |
| `enum-refusal-inline-leaf-list` | `test/parse/enum-refusal-inline-leaf-list.ci` | AC-5 | |
| `enum-refusal-config-set` | `test/parse/enum-refusal-config-set.ci` | offline `ze config set` gets the AC-2 body | |
| `enum-refusal-daemon-start` | `test/plugin/enum-refusal-daemon-start.ci` | AC-10 start | |
| `enum-refusal-reload` | `test/reload/enum-refusal-reload.ci` (shape of `config-reload-invalid-validator.ci`) | AC-10 reload | |
| `enum-refusal-set` | `test/editor/validation/enum-refusal-set.et` | AC-6 | |
| `config-enum-refusal` | `test/web/config-enum-refusal.wb` | AC-7 | |
| `api-config-enum-refusal` | `test/plugin/api-config-enum-refusal.ci` (shape of `api-config-commit-reject.ci`) | AC-8 | |
| `gnmi-set-enum-refusal` | `test/plugin/gnmi-set-enum-refusal.ci` | AC-9 | |
| `command-arg-enum-refusal` | `test/plugin/command-arg-enum-refusal.ci` | AC-12 | |
| updated | `test/parse/asn4-refuses-boolean-spelling.ci`, `test/parse/capability-leaf-refuses-presence-spelling.ci`, `test/isis/isis-auth.ci` | expectations follow the new body; each still proves its refusal | |

### Interop Tests (Scope: protocol)
N-A: config refusal text; no peer sees it and no wire byte changes.

## Files to Modify
- `internal/component/config/schema.go` - `LeafNode` and `ValueOrArrayNode` gain per-value help; `ValidateLeafValue` and `ValidateListKey` take the path and return the typed refusal.
- `internal/component/config/yang_schema.go` - `yangToLeaf` fills per-value help from `yang.EnumValueSummaries`; the leaf-list builder does the same.
- `internal/component/config/parser.go` - path stack in the parser; enum refusal framed with `line N:` and the `write:` line (`leafValueFix` kept).
- `internal/component/config/parser_list.go`, `parser_freeform.go`, `setparser.go`, `setparser_meta.go` - pass the path.
- `internal/component/config/yang/validator.go` - `validateEnumeration` fills `Message` with the shared body; `Type`, `Expected`, `Got` unchanged. Its `// Design:` doc `yang-config-design.md` is updated.
- `internal/component/config/cli/cmd_validate.go` - `errors.As` on a parser error: code `config-yang-enum`, expected, actual, path, repair.
- `internal/component/cli/completer.go`, `completer_validate.go` - editor refusal for an enum (and a union, Q-2) through the shared type; fix line `set <path> <value>`.
- `internal/component/cli/editor_commands.go` - `ValidateListKey` call passes the path.
- `internal/component/cli/validator.go` - `formatPeerError` enum arm and the generic arm use the shared body.
- `internal/component/web/handler_config_form.go`, `handler_config_entry.go` - validate with `ValidateLeafValue` (type and YANG restrictions), path passed, message masked as today.
- `internal/component/command/argvalidate.go`, `argdef.go` and `internal/component/config/yang/command.go` - enum ArgDef carries per-value help; `validateEnum` uses the shared type.
- The pinning tests listed in Current Behavior.
- Docs: `docs/architecture/config/syntax.md`, `docs/architecture/config/yang-config-design.md`, `docs/guide/config-editor.md`, `docs/guide/web-interface.md`, `docs/architecture/web-interface.md` (declared by the web handlers: the form now refuses an enum at set), `docs/architecture/api/commands.md`.

## Files to Create
- `internal/core/suggest/refusal.go` - the refusal type (path, value, accepted values with help, suggestion) and its text renderer.
- `internal/core/suggest/refusal_test.go`
- The `.ci`, `.et` and `.wb` files in the Functional Tests table.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no leaf, RPC or extension added |
| YANG validation constraints | No | enumerations exist; only the refusal text changes |
| YANG custom validators | No | no `ze:validate` added; the refusal is native |
| CLI commands/flags | No | no command added |
| CLI grammar (keyword before value) | N-A | no grammar change |
| Editor autocomplete | No | completion already lists values with `ze:help` through `EnumValueSummaries`; the refusal now matches it |
| Functional test for new RPC/API | Yes | `test/plugin/api-config-enum-refusal.ci`, `test/plugin/gnmi-set-enum-refusal.ci` |
| Pipe completeness | N-A | no command output added; `ze config validate` JSON already routes its diagnostics |
| Env var registration | N-A | no env var |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Prometheus counters/metrics | N-A | no observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | refusal wording only; check `./le feature report` at implementation |
| 2 | Config syntax changed? | Yes | `docs/architecture/config/syntax.md`: new "Enumeration values" subsection beside "Unknown keys" |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`: enum argument refusal |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/config-editor.md` (set refusal), `docs/guide/web-interface.md` (form refusal at set) |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 7950 Section 9.6 verdicts unchanged |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md`: one refusal type, where it is built |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | derived | run `./le spec citation anchors spec plan/immediate/spec-config-enum-refusal-proposes-valid-values.md` at implementation; `ai/CODE-TO-DOCS.md` already names `syntax.md`, `yang-config-design.md`, `config-editor.md`, both `web-interface.md` pages, `aaa-tacacs.md`, `fixit-bcrypt-hash-credential.md`, `debugging-tools.md`, `redistribution.md` and `api/commands.md` for the edited files |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | grep of `docs/` for `invalid enum` and `expected one of` found none on 2026-10-13; re-grep for quoted refusal examples in `config-editor.md` and update any |

## Implementation Steps

1. **Phase: Wiring** -- the shared type in `internal/core/suggest`, `ValidateLeafValue` / `ValidateListKey` signature with the path, every caller passes it; write the parse `.ci` and the `.et` failing.
   - Tests: `TestValueRefusalRender`, `test/parse/enum-refusal-proposes-valid-values.ci`
   - Verify: compiler finds every caller; the `.ci` is red on the old text.
2. **Phase: schema carries help** -- `yangToLeaf` and the leaf-list builder store per-value `ze:help`.
   - Tests: `TestYangToLeafCarriesEnumHelp`
3. **Phase: file paths** -- parser path stack, `validateInlineListChildValue`, set/meta/freeform parsers, `cmd_validate` diagnostic mapping; rewrite the pinned parser tests and three `.ci`.
   - Tests: `TestParserEnumRefusalNamesFullPath`, `TestInlineLeafListEnumRefusal`, `TestRunValidationParserEnumDiagnostic`, the parse, start, reload and config-set `.ci`
4. **Phase: tree walk** -- `validateEnumeration` message from the shared body; `Type`/`Expected`/`Got` unchanged; run the RFC-tagged enum tests untouched.
   - Tests: `TestValidateEnumerationSharedBody`
5. **Phase: interactive and remote** -- editor `ValidateValueAtPath` and `formatPeerError`, web form sites, REST and gNMI `.ci` (find the gNMI Set harness first, A-6).
   - Tests: the editor, web, API and gNMI rows
6. **Phase: command arguments** -- ArgDef help from YANG, `validateEnum`; rewrite `TestSendArgumentsAreDeclared` and `TestValidateArgStringEnum`.
7. **Phase: unions (Q-2) and docs** -- unions as answered; the pages in the Documentation checklist; re-read the BMP dependant's `.ci` expectations (R-7).

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | grep for `invalid enum`, `expected one of`, `must be one of`, `(expected enum)`, `not a valid enumeration value` in non-test Go: zero producers remain for a YANG enum |
| Naming | JSON diagnostic keys unchanged (`expected`, `actual`, `path`, `code`) |
| Data flow | accepted values and help read only through `EnumNamesDeclared` / `EnumValueSummaries` (or the `LeafNode` fields they fill) |
| Rule: principles (one declaration) | no list of enum values anywhere in Go for this purpose |
| Rule: testing | each rewritten assertion is as strong as before; none deleted |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| one refusal type | `gopls symbols internal/core/suggest/refusal.go` |
| old texts gone | the grep in the Critical Review "Correctness" row |
| every path proven | the ten Wiring Test files exist and pass |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the refused value is quoted (`%q` form), so control characters cannot forge extra lines in a log or the web fragment |
| Error leakage | sensitive leaves: the value never appears (AC-16); the web fragment escapes the text as today |
| Resource exhaustion | suggestion costs values times length on an error path only; values are schema-bounded |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The parser's enum refusal is reported by `ze config validate` as `config-parse` today, so the diagnostic code that exists for enums (`config-yang-enum`) is reached only when the parser did not refuse first. The typed refusal lets the parser's error carry the same code.
- Two paths do not refuse at `set` (web form: type check only; REST/gNMI: no check), so "the same message on every path" also means the same moment for the web form, where the operator is looking at the field.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Owner direction (c), 2026-10-13: "M2 go with (c), friendly errors everywhere" | (d) a YANG-declared reason for a removed value | the owner chose (c); (d) is not designed here (Known Limitations) |
| A typed refusal value (path, value, accepted values with help, suggestion) built where each check refuses, rendered once by its `Error()`; surfaces add only their frame | (B) keep `fmt.Errorf` everywhere and add a shared string helper; (C) refuse only in the YANG tree walk and remove the parser and editor enum checks | B keeps the text but loses structure: `ze config validate` JSON and the editor fix line need the fields, and every call site still formats. C changes the moment of the verdict (editor `set` and file load would refuse later, without a line number) and is a layering change no one asked for. A is the one declaration of the text and the only shape the JSON can read |
| The type lives in `internal/core/suggest` | `internal/component/config`; `internal/component/config/yang`; a new core package | `config/yang` imports `command` and `config` imports `config/yang`, so only a leaf package is importable by all producers; `suggest` already owns "did you mean" and imports nothing from ze, so a new package would hold one type |
| `ValidateLeafValue` and `ValidateListKey` take the path as a parameter | a setter on the returned error filled by callers | the compiler then finds every caller; a setter can be forgotten silently (`ai/rules/principles.md`, silent wrong value) |
| Per-value text is `ze:help` only | also read the enum `description` | `ai/rules/config.md`: an enum value carries its text as `ze:help` alone; no surface reads an enum description |
| Accepted values in YANG declaration order | sorted (`EnumValueNames`, the completion order) | the parser and `leafValueFix` already use declared order and two `.ci` pin it; declared order is the author's intent |
| The body is identical everywhere; only the frame differs (`line N:` and `write:` for a file, `set <path> <value>` for the editor, code and fields for JSON) | one identical string including the fix line | the fix differs by surface (`ai/rules/user-facing-errors.md`: "the exact next step"); a file is edited, the editor takes a `set` |
| Closest match through `suggest.Command` | a new helper with a different bound | one distance across CLI, unknown key and enum value; no new helper |

## Known Limitations

- Option (d), a YANG-declared reason for a removed enum value, is not designed here: the owner chose (c). Possible future work only.
- Pattern refusals (`value "x" does not match pattern "<regex>"`) and range/length refusals are not changed here unless the owner answers Q-5; if Q-5 places them in another spec, that spec is written and named here.
- `retiredKeywords` (keywords, not values) stays a Go map.

## Owner Questions

| ID | Question | Recommendation | Answer |
|----|----------|----------------|--------|
| Q-0 | Direction: (c) one generic friendly refusal for every YANG enum on every path, or (d) a YANG-declared removed-value reason | - | **Answered by the owner, 2026-10-13: "M2 go with (c), friendly errors everywhere".** |
| Q-2 | A union leaf whose members include an enumeration: does it get the generic refusal (enum values with help, plus the other member types in words)? | Yes: "everywhere" and "every YANG enum" include union members; `EnumValueNames` and `EnumValueSummaries` already read unions | |
| Q-3 | Operational command enum arguments (`send bgp ... raw gzip`, built from YANG RPC input) use the same refusal? Read as yes under "everywhere"; this row asks only for confirmation | Yes: they are YANG enums, their text is the sixth variant, and the same type renders them | |
| Q-4 | Long value lists (dozens of values): print all, or cap with "N more, press ? to see them"? | Print all: the operator needs the full list and `?` shows the same list | |
| Q-5 | Pattern refusals show a raw regular expression today; range and length refusals state the range but not the path. In this spec, in a separate spec, or left? | Separate spec for pattern (its friendly form needs a decision on what to show instead of the regex, perhaps the type's `ze:help`); range and length gain the path here, since the path parameter from Phase 1 reaches them at no extra cost | |
| Q-6 | Web form: refuse an enum at set time (new) rather than only at commit? | Yes: the operator is looking at the field, and it is the same check the file parser runs | |

No design approval is recorded. Status stays `design` until the owner answers Q-2 to Q-6 and approves the design.

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
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
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
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/immediate/spec-config-enum-refusal-proposes-valid-values.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
