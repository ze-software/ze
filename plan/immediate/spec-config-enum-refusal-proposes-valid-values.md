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
`ze config set`, editor `set`, `load` and `commit`, the editor `set cli format`
command, web UI form and web CLI terminal `set`, REST API and gNMI `Set`, reload,
and operational command arguments, which are YANG enums too). It:

| Element | Source |
|---------|--------|
| names the setting by its config path, list keys included | the path the caller already holds |
| quotes the refused value (a placeholder on a sensitive leaf) | the input; `config.LeafHoldsSecret` decides the placeholder |
| lists every accepted value, in YANG declaration order | the YANG enumeration, never a hand list; order source per producer in Key Design Decisions |
| gives each accepted value its one-line `ze:help` where the YANG declares one | `yang.EnumValueSummaries` (the one reader of an enum value's `ze:help`) |
| offers "did you mean" when one accepted value is close, never on a sensitive leaf | `suggest.Command` (`internal/core/suggest`), the distance the CLI and the unknown-key refusal already use, in the one hint form `(did you mean X?)` that `UnknownKeywordHint` (`retired.go`) writes |

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
  → Constraint: `rfc/discrimination/rfc7950.json` holds six records whose producer this spec edits, each fingerprinted by `producer-sha`; a record whose `producer-sha` no longer matches committed code is a violation at the commit that carries the edit (`docs/contributing/rfc-conformance-gates.md`, refusal table). The six: `RFC7950-9.6-1` positive and negative (`TestRadiusAuthMethodEnum`, producer `validateEnumeration`); `RFC7950-7.3.4-1` (`TestRFC7950DerivedTypeInheritsOrReplacesTheDefault`, producer `yangToLeaf`); `RFC7950-9.12-1` three records (`TestRFC7950UnionAcceptsEveryMemberType`, `TestValidateTree_UnionViolation`, `TestValidateTree_ValidConfig`, producer `config/yang/validator.go` `validateUnion`, edited only if Q-2 is yes). Each is re-recorded with `./le rfc discriminate-record` in the same commit (Implementation Steps, Phase 8).

**Key insights:**
- Eleven producers today, one fact (the enumeration): the six texts in the table below plus the leaf-list pair (`storeValueOrArray`, `validateValueOrArrayItems`), the `cli format` status line, the command union arm, and the parser's missing-value refusal. Two of them (`ValidateLeafValue` in `schema.go`, `validateInlineListChildValue` in `parser_list.go`) are the ones the operator meets on every file path.
- Two paths do not check an enum value at `set` at all: the web CLI terminal `set` (`executeTerminalSet` in `web/cli_terminal.go` calls `EditorManager.SetValue`, which checks only the secret placeholder) and the REST/gNMI session set.
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
- [ ] `internal/component/config/retired.go` - `UnknownKeywordHint` (retired hint, else `suggest.Command`, rendered ` (did you mean X?)`), `retiredKeywords` hand map for retired KEYWORDS.
- [ ] `internal/component/config/parser_list.go` (`storeValueOrArray`) - bracket leaf-list member outside `ValueOrArrayNode.ValidValues`: `line N: invalid value for <leaf>: "x" (valid: a, b)`.
- [ ] `internal/component/config/setparser.go` (`validateValueOrArrayItems`) - the same check for the plain and meta set parsers, same `(valid: ...)` text.
- [ ] `internal/component/config/parser.go` (`ParseAt`) - editor `load` parses from a context path; the parser starts with no path of its own, so a path stack must be seeded from `contextPath`. `parseLeaf` also has a missing-value arm for an enum leaf: `<leaf> needs a value, one of a, b, and is not a block or a bare word` plus `write:`.
- [ ] `internal/component/config/cli/cmd_set.go` - offline `ze config set` checks the value with `cli.Completer.ValidateValueAtPath`, not the set parser, then masks with `DisplayMessageAtPath`.
- [ ] `internal/component/cli/model_keys.go` (`set cli format`, `cliFormatNames`) - the values are the YANG enum of `cliFormatLeafPath`, re-sorted with `slices.Sorted`; refusal `invalid format: x (valid: json, ndjson, table, text, yaml)`.
- [ ] `internal/component/command/argvalidate.go` (`validateUnion`) - command argument union with an enum member: `invalid value "x", expected unsigned integer or one of: a, b`.
- [ ] `internal/component/web/cli_terminal.go` (`executeTerminalSet`) and `web/editor.go` (`EditorManager.SetValue`) - the web CLI terminal `set` performs no value check; an out-of-enum value is stored and refused only at commit.
- [ ] `internal/component/web/handler_config_leaf.go`, `fragment.go` - an enum leaf renders as a `<select>` of `leaf.Enums`, so the form refuses an enum value only for a crafted POST.
- [ ] `cmd/ze/hub/api.go` (`configValidationHook`) and `cmd/ze/hub/service_gnmi.go` - REST and gNMI commit hook; `TestConfigValidationHookRunsFullValidation` (`cmd/ze/hub/rfc8907_api_test.go`) installs it in a Go test.
- [ ] `vendor/github.com/openconfig/goyang/pkg/yang/types_builtin.go` (`EnumType.Names`) - answers the names sorted with `sort.Strings`, not declared.

**The texts today:**

| # | Path | Producer | Text |
|---|------|----------|------|
| 1 | file load, `ze config validate`, reload, REST/gNMI commit | `ValidateLeafValue` via parser / set parser | `line N: invalid value for route-action: invalid enum: "redistribute" (expected one of: monitor)` plus `write:` line |
| 2 | same, one member of an inline leaf-list | `validateInlineListChildValue` | `invalid enum: "x" (expected one of: a, b)` |
| 2b | same, a bracket leaf-list member; set-format files | `storeValueOrArray`, `validateValueOrArrayItems` | `line N: invalid value for leaf: "x" (valid: a, b)`, names in goyang's sorted order |
| 2c | file, enum leaf written with no value | `parseLeaf` missing-value arm | `leaf needs a value, one of a, b, and is not a block or a bare word` plus `write:` |
| 3 | tree walk (`ValidateCustomSections`, commit) when the parser did not refuse first | `validateEnumeration` | `enum error at a.b.c: value "x" is not a valid enumeration value` |
| 4 | editor/web commit, peer leaf | `formatPeerError` | `peer X: "leaf" must be one of: a, b (got "x")` |
| 5 | editor `set`, offline `ze config set` | `ValidateValueAtPath` | `invalid value "x" for leaf (expected enum)` |
| 5b | editor `set cli format` | `model_keys.go` | `invalid format: x (valid: json, ndjson, table, text, yaml)` |
| 5c | editor `load` | `ParseAt` → text 1, 2, 2b | as text 1, path relative to the edit context |
| 5d | web CLI terminal `set` | none | not refused at set; text 4 or 3 at commit |
| 6 | operational command argument | `validateEnum` | `invalid value "x", expected one of: a, b` |
| 6b | operational command union argument | `validateUnion` | `invalid value "x", expected unsigned integer or one of: a, b` |

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
| `test/editor/pipe/set-cli-format.et` ("Invalid value" block) | `status:contains=invalid format`, `status:contains=valid: json, ndjson, table, text, yaml` (text 5b, sorted order) | No |

A grep of `test/` for `valid: `, `expected one of`, `invalid enum`, `must be one of` and `expected enum` (2026-10-13) found no other functional test pinning a refusal text; every other hit was prose in a comment.

RFC-tagged tests that touch enum refusals but NOT the text (must stay green, untouched): `rfc7950_radius_auth_method_enum_test.go` (`RFC7950-9.6-1`, asserts `ErrTypeEnum` and `Expected` contains `pap`, `chap`), `rfc7950_isis_auth_algorithm_enum_test.go` (`RFC7950-9.6-1`, asserts `ErrTypeEnum`), `radius_auth_method_eap_enum_test.go` (untagged sibling, asserts `Expected` contains each value), `rfc7950_validator_yang_test.go` `TestValidateTree_EnumViolation` (asserts `ErrTypeType` and the path), `TestValidateTree_FamilyModeEnum` and `TestValidateTree_AddPathDirectionEnum` (same file, `RFC7950-9.6-1` positive and negative, assert `ErrTypeEnum` on the refused value and no error on each accepted one). None reads `Message` or the order of `Expected`.

**Behavior to preserve:**
- Every value outside an enumeration stays refused on every path that refuses it today; the verdict never changes, only the words.
- `ValidationError.Type == ErrTypeEnum`, `Expected` = accepted names joined with `, `, `Got` = the value (read by the RFC-tagged tests and by `cmd_validate` JSON `expected`/`actual`).
- The parser's `line N:` prefix and its `write:` fix line, including the `enable`/`disable` choice for a boolean spelling.
- Validation order type, enum, range, length, pattern.
- Secret masking of every refusal (`DisplayMessageAtPath`, `maskSecretInMessage`).
- `UnknownKeywordHint` and `retiredKeywords` (keywords, not values) unchanged.

**Behavior to change:**
- Every text in the table becomes one generic refusal (Design).
- Editor `set` and offline `ze config set` name the accepted values (today: the word `enum`).
- Web CLI terminal `set` refuses an enum value at set time (today: no check at set); the web form handler refuses a crafted POST at set time (today: only at commit).
- `ze config validate` reports a parser enum refusal with code `config-yang-enum` and fills `expected`/`actual` (today: `config-parse` with neither); `actual` stays omitted on a sensitive leaf, as the tree walk already does.
- `Expected` and every rendered list follow YANG declaration order (today: goyang's alphabetical `Names()` in the validator and the leaf-list builder, `slices.Sorted` in `cli format`).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A config value as text: a config file line, a `set` line (editor, offline `ze config set`, REST/gNMI session set), a web form post, or an operational command argument.

### Transformation Path
1. Schema build: `yangToLeaf` reads the enumeration names (declared order) and, new, the per-value `ze:help` (`yang.EnumValueSummaries`) onto `LeafNode`; the leaf-list builder does the same for `ValueOrArrayNode`; the command ArgDef builder does the same for an enum argument.
2. Value check: `ValidateLeafValue` (parser, set parser, meta parser, freeform parser, list keys, `ParseAt` for editor `load`), `validateInlineListChildValue`, `storeValueOrArray`, `validateValueOrArrayItems`, the parser's missing-value arm, `validateEnumeration` (tree walk), `ValidateValueAtPath` (editor `set`, offline `ze config set`, and new in the web CLI terminal `set`), `set cli format`, the web form sites, `validateEnum` and `validateUnion` (command args) each build ONE typed refusal value (path, value, accepted values with help, suggestion, secret flag) when the value is not accepted.
3. Rendering: the typed refusal's `Error()` gives the shared text; each surface adds only its own frame (`line N:` and `write:` for a file, a `set <path> <value>` fix for the editor, offline set and web terminal, the diagnostic code and `expected`/`actual` for `ze config validate` JSON, the path and a set-form fix with no `line N:` and no `write:` for REST and gNMI). Every frame wraps the refusal with `%w` (or a typed line error with `Unwrap`), so `errors.As` recovers it through the frame.
4. Masking: a refusal on a leaf where `config.LeafHoldsSecret` holds is built with its secret flag set: the value slot carries `config.SecretDataPlaceholder` and no suggestion is computed, so nothing derived from the secret is rendered. `DisplayMessageAtPath` / `MaskSecretInMessage` stay as the surface's second line of defence, unchanged.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG module → config schema | `yangToLeaf` copies enum names and per-value `ze:help` | No |
| config → cli editor / web / api | typed error crosses as `error`; `errors.As` recovers the fields | No |
| daemon → REST/gNMI client | commit error text from `ValidateContent`; frame is the path plus a set-form fix, no `line N:`, no `write:` | No |
| web CLI terminal → editor manager | `executeTerminalSet` gains the `ValidateValueAtPath` check before `SetValue` | No |

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
| Registration over hardcoding, inbound | Yes | searched `invalid enum`, `expected one of`, `must be one of`, `not a valid enumeration value`, `(expected enum)`, `(valid: `, `or one of`, `needs a value, one of`: every list of values in the new text comes from the YANG enumeration on the node at hand; no leaf name appears in any producer |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `suggest.Command` with its `len(input)/2` bound is a good enough closest match for enum values | it is the CLI's and the unknown-key refusal's helper (`docs/architecture/config/syntax.md`, "Unknown keys") | a far value gets a misleading hint, or a near one none | `TestEnumRefusalSuggestion` table (`requre` gives `require`, `redistribute` gives none) | unvalidated |
| A-2 | Most enum values declare `ze:help` | 607 `enum x {` blocks and no bare `enum x;` in `internal/**/*.yang` (grep, 2026-10-13); the help-shape gate counts values without a summary but does not refuse them | values print with no help text; the list still names them | `./le doc yang-contract help-shape` counts `SchemaEnumValuesWithSummary` against `SchemaEnumValues` | unvalidated |
| A-3 | REST and gNMI receive the refusal only at commit, through `ValidateContent` | `ConfigSessionManager` set calls `SetValue` with no check; `configValidationHook` | a set-time refusal elsewhere would bypass the renderer | wiring tests for AC-8, AC-9 | unvalidated |
| A-4 | The parser can carry a path stack without changing its output for valid configs | `Parser` holds no path today; `parseBody`/`parseContainer` recurse per block and list entry | path wrong or missing list keys | `TestParserEnumRefusalNamesFullPath` with a list key in the path | unvalidated |
| A-5 | A multi-line refusal renders readably in the web terminal output, the web error fragment, the toast and `ValidateContent`'s join | `executeTerminalSet` and `WriteOOBError` return a string; `ValidateContent` indents only the first line of each diagnostic | lines run together or lose indentation | AC-7 `.wb` asserts each accepted value is visible in the terminal; AC-8 asserts every line | unvalidated |
| A-6 | No `.ci` drives a gNMI `Set` or a REST enum commit today, so this spec builds the clients | settled 2026-10-13 at the producer: `gnmi-show.ci` runs a dispatch command, `mgmt-guard-gnmi-*.ci` test the listener guard, and no fixture under `internal/test/fixture/` sends a `SetRequest`; the vendored `github.com/openconfig/gnmi` proto is the client API; the REST fixture `plugin/api-config-commit-reject` (registered in `plugin_fixture_01.go`) is the shape | the clients do not reach the daemon's session manager | two `le test fixture` clients in Files to Create (`plugin/api-config-enum-refusal` REST, `plugin/gnmi-set-enum-refusal` gNMI `Set`) drive the two `.ci`; `TestConfigValidationHookEnumRefusal` (`cmd/ze/hub/rfc8907_api_test.go`, the file whose `TestConfigValidationHookRunsFullValidation` already installs `configValidationHook`) proves the body and frame at the hook | design settled; proof owed at implementation |
| A-7 | "Friendly errors everywhere" covers operational command enum arguments | owner answer 2026-10-13; those arguments are YANG enums (`config/yang/command.go` builds `NewEnumArg` from YANG) | AC-12 and Phase 6 move to their own spec if the owner says otherwise (Q-3) | owner confirmation of Q-3 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A leaf with dozens of values (address families, log levels) prints a very long list | refusal over about 20 lines in a `.ci` | Q-4; recommended: print every value, no cap, since `?` shows the same list |
| R-2 | Changing `ValidateLeafValue`'s signature to take the path touches every caller (eight in `config`, one in `cli`) | compile errors | intended: the compiler finds every caller, none can forget the path |
| R-3 | A sensitive enum leaf leaks the value, directly or through the suggestion (a close accepted value reveals most of the secret) | masking test | the refusal carries a secret flag set from `config.LeafHoldsSecret`: the value slot holds `config.SecretDataPlaceholder`, no suggestion is computed, JSON omits `actual`; surface masking stays as a second defence; `TestEnumRefusalMasksSensitiveValue` asserts no suggestion line (AC-16) |
| R-4 | Union leaves with enum members are not enum-checked by `ValidateLeafValue` today (`yangToLeaf` sets `Enums` only for a `Yenum` type) | union case refused with a type text | Q-2; recommended yes, AC-13 |
| R-5 | `ze config validate` JSON code for a parser enum refusal changes from `config-parse` to `config-yang-enum` | `test/ui/*` `.ci` | only `invalid syntax here` cases pin `config-parse`, and those stay `config-parse`; a consumer keyed on `config-parse` for an enum value is the case the code exists to separate |
| R-6 | The RFC-tagged enum tests break through a change to `Expected` | those tests red | `Expected` keeps the comma-joined names; help text goes in a separate field |
| R-7 | The BMP dependant and this spec land in either order, and the BMP `.ci` must stay green under both texts | `test/parse/bmp-route-action-refuses-redistribute.ci` red after this spec lands | re-read 2026-10-13: the BMP spec's AC-1, AC-2 and Functional row pin exactly three words, `route-action`, `redistribute` and `monitor`, with no `:` in a needle and no other message text, on `ze config validate -` and on `ze -`. This spec's body carries all three on both commands: the path ends in the leaf name `route-action`, the quoted value is `redistribute` (not a sensitive leaf, so no placeholder), and the accepted-value list holds `monitor`; `redistribute` against `monitor` is AC-3's no-suggestion case, so the list still prints. AC-1 and AC-10 assert path, value and every accepted value on those same two commands, so a change that drops any of the three words is red here first. The BMP spec also cites by name `TestValidateValueAtPathEnumRefusal`, `enum-refusal-set.et`, `TestWebFormRefusesEnumAtSet`, the REST and gNMI wiring tests, `enum-refusal-daemon-start.ci` and `enum-refusal-reload.ci` for AC-6 to AC-10: those names and AC numbers are kept. Its Required Reading line quoting the old text ("invalid value for <leaf> ... expected one of <values>") is that spec's prose, not a pin; Phase 7 re-greps its `.ci` |
| R-8 | A rewritten pinned test asserts less than before | review | each rewrite asserts the value, the path and every accepted value (`ai/rules/testing.md`) |
| R-9 | `Expected` from the tree walk changes from alphabetical to declared order | a test asserting `Expected` by equality | every reader found asserts `Contains` (`rfc7950_radius_auth_method_enum_test.go`, `radius_auth_method_eap_enum_test.go`); AC-11 asserts the declared order by equality from here on |
| R-10 | The six RFC 7950 discrimination records go stale when their producer changes, and the commit is refused | `./le rfc check` reports a `discrimination:` line naming the record | Phase 8 re-records each with `./le rfc discriminate-record` in the commit that edits the producer; never hand-edited |

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
| `ze config validate -` on a bracket leaf-list member | → | `storeValueOrArray` | `test/parse/enum-refusal-inline-leaf-list.ci` (second case) |
| `ze config validate -` on a set-format file with a bad leaf-list member | → | `validateValueOrArrayItems` | `test/parse/enum-refusal-set-format.ci` |
| `ze config validate -` on an enum leaf with no value | → | `parseLeaf` missing-value arm | `test/parse/enum-refusal-proposes-valid-values.ci` (missing-value case) |
| daemon start on a file with a bad enum | → | loader → parser | `test/plugin/enum-refusal-daemon-start.ci` |
| reload to a file with a bad enum | → | reload → parser | `test/reload/enum-refusal-reload.ci` |
| offline `ze config set` with a bad enum | → | `cmd_set.go` → `ValidateValueAtPath` | `test/parse/enum-refusal-config-set.ci` |
| SSH editor `set` with a bad enum | → | `ValidateValueAtPath` | `test/editor/validation/enum-refusal-set.et` |
| SSH editor `load` of a snippet with a bad enum, inside an edit context | → | `ParseAt` → parser path stack seeded from the context | `test/editor/validation/enum-refusal-load.et` |
| SSH editor `set cli format bogus` | → | `model_keys.go` `cli format` arm | `test/editor/pipe/set-cli-format.et` (rewritten) |
| web CLI terminal `set` with a bad enum | → | `executeTerminalSet` → `ValidateValueAtPath` | `test/web/config-enum-refusal.wb` |
| crafted web form POST with a bad enum | → | `handler_config_form.go` value check | `TestWebFormRefusesEnumAtSet` (handler test; the form renders a `<select>`, so no browser path sends one) |
| REST config session commit | → | `configValidationHook` → `ValidateContent` | `test/plugin/api-config-enum-refusal.ci` driven by `le test fixture plugin/api-config-enum-refusal` |
| gNMI `Set` | → | `configValidationHook` → `ValidateContent` | `test/plugin/gnmi-set-enum-refusal.ci` driven by `le test fixture plugin/gnmi-set-enum-refusal` (A-6) |
| operational command enum argument | → | `validateEnum` | `test/plugin/command-arg-enum-refusal.ci` |
| operational command union argument with an enum member | → | `validateUnion` | `test/plugin/command-arg-enum-refusal.ci` (union case) |

## Acceptance Criteria

The example leaf is `bgp peer peer1 session capability route-refresh` (values
`enable`, `disable`, `require`, `refuse`, each with a `ze:help`).

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | file config with `route-refresh maybe;` under peer `peer1`, `ze config validate` | exit 1; stdout carries the line number, the path `bgp peer peer1 session capability route-refresh`, the quoted value `"maybe"`, a heading for the accepted values, and one line per value holding the value name and its `ze:help` text, in YANG declaration order; the `write: route-refresh <enable\|disable\|require\|refuse>;` line stays |
| AC-2 | same with `route-refresh requre;` | AC-1, and the problem line ends with ` (did you mean require?)`, the exact hint form `UnknownKeywordHint` writes, rendered by the one helper both call |
| AC-3 | value far from every accepted value (`redistribute` against `monitor`) | no `did you mean` text; the value list is still printed; the fix line lists every value as an alternation (AC-6) |
| AC-4 | enum value with no `ze:help` | its line carries the name alone; no placeholder, no empty separator |
| AC-5 | inline leaf-list member, bracket leaf-list member (`storeValueOrArray`), and set-format leaf-list member (`validateValueOrArrayItems`) outside the enumeration | the same refusal for that member, path named, values in declared order; `(valid: ` no longer appears |
| AC-6 | SSH editor `set ... route-refresh requre`, and offline `ze config set` of the same | the same body as AC-2 (path, value, suggestion, values with help), and a fix line on its own line: `set <path> require` when a suggestion exists, `set <path> <enable\|disable\|require\|refuse>` when none (AC-3); `(expected enum)` no longer appears |
| AC-7 | web CLI terminal `set ... route-refresh requre` | refused at set, before commit, with the AC-6 body and fix line in the terminal output; nothing is stored in the working tree (a following `show` holds no `requre`). A crafted form POST of `requre` to the form handler is refused at set with the same body (`TestWebFormRefusesEnumAtSet`) |
| AC-8 | REST config session sets `requre` then commits | commit refused; the error text holds the AC-2 body framed by the path and a set-form fix line (`set <path> require`); it carries no `line N:` and no `write:`; active config unchanged |
| AC-9 | gNMI `Set` of `requre` | `Aborted`; status message holds the AC-2 body framed as in AC-8, with no `line N:` and no `write:` |
| AC-10 | daemon start and reload with AC-2's file | start refuses with the AC-2 body; reload refuses with it and the running config is unchanged |
| AC-11 | `ze config validate` JSON output on AC-2's file | the diagnostic has code `config-yang-enum`, `expected` = `enable, disable, require, refuse` (declared order, by equality), `actual` = `requre`, `path` in the dotted form the tree walk already writes (`bgp.peer.peer1.session.capability.route-refresh`), and the message equal to the text body |
| AC-12 | operational command with a bad enum argument, for example `send bgp 192.0.2.1 raw gzip DEADBEEF`; and a union argument with an enum member | refusal names the command and argument, quotes `gzip`, lists `hex` and `b64` with their `ze:help`, and suggests when close; the union refusal lists the enum values and names the other member types in words; `or one of` no longer appears |
| AC-13 | (Q-2 yes) union leaf whose members include an enumeration, value matching no member | refusal lists the enum members with help, in union member order then each member's declared order, and names the other member types in words |
| AC-14 | every value accepted before this change, on every path | still accepted (verdict unchanged); every value refused before is still refused |
| AC-16 | sensitive enum leaf (a real one if it exists, else a test schema) refused, value close to an accepted value | the rendering shows `config.SecretDataPlaceholder` in the value slot; neither the refused value nor any suggestion appears (there is no `did you mean` text at all); JSON omits `actual`; the accepted values still appear |
| AC-17 | YANG validator tree walk (`validateEnumeration`) refusal | `Type` stays `ErrTypeEnum`, `Expected` is the comma-joined names in declared order, `Message` is the shared body; `ValidationError.Error()` names the path once (today it prefixes `enum error at <path>:`); RFC-tagged tests unchanged and green |
| AC-18 | file config with `route-refresh;` (enum leaf, no value) | the shared body with no quoted value (the problem line says the leaf needs a value), every accepted value with help, and the `write:` line as today; `needs a value, one of` no longer appears |
| AC-19 | SSH editor `edit bgp peer peer1`, then `load` a snippet holding `session { capability { route-refresh requre; } }` | refused with the AC-2 body naming the full path `bgp peer peer1 session capability route-refresh`, the context included |
| AC-20 | SSH editor `set cli format bogus` | the shared body for the `cli format` leaf; values in declared order with help; `(valid: ` no longer appears |
| AC-21 | `errors.As` on a parser error, an editor error and a `ValidationError` | recovers the typed refusal through every frame (`TestRunValidationParserEnumDiagnostic` proves the parser chain) |

AC-15 is unused: it was a removed-value reason, which the owner's direction (c) leaves out.

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | typos an enum in a config file and validates it | file → parser → `ValidateLeafValue` → refusal | `test/parse/enum-refusal-proposes-valid-values.ci` |
| 2 | types a bad enum in the SSH editor | editor `set` → `ValidateValueAtPath` | `test/editor/validation/enum-refusal-set.et` |
| 3 | types a bad value in the web UI's CLI terminal | `executeTerminalSet` → `ValidateValueAtPath` | `test/web/config-enum-refusal.wb` |
| 7 | loads a snippet with a bad enum in the SSH editor | `load` → `ParseAt` | `test/editor/validation/enum-refusal-load.et` |
| 4 | pushes config through REST or gNMI | session set → commit → `ValidateContent` | `test/plugin/api-config-enum-refusal.ci`, `test/plugin/gnmi-set-enum-refusal.ci` |
| 5 | reloads a file with a bad enum | reload → parser | `test/reload/enum-refusal-reload.ci` |
| 6 | mistypes an enum argument of an operational command | dispatcher → `validateEnum` | `test/plugin/command-arg-enum-refusal.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestValueRefusalRender` | `internal/core/suggest/refusal_test.go` | body layout: path, quoted value, suggestion line, value lines with and without help (AC-1 to AC-4) | |
| `TestEnumRefusalSuggestion` | `internal/core/suggest/refusal_test.go` | `requre` gives `require`, `monitr` gives `monitor`, `redistribute` gives none (A-1) | |
| `TestUnknownKeywordHintSharesTheHintForm` | `internal/component/config/retired_test.go` | the unknown-key hint and the enum refusal hint come from the one helper and read ` (did you mean X?)` | |
| `TestYangToLeafCarriesEnumHelp` | `internal/component/config/yang_schema_test.go` | `LeafNode` holds per-value `ze:help` in declared order, from `EnumValueSummaries` | |
| `TestValueOrArrayEnumDeclaredOrder` | `internal/component/config/yang_schema_test.go` | the leaf-list builder fills `ValidValues` from `EnumNamesDeclared`, not goyang's sorted `Names()`, using a YANG enum declared out of alphabetical order | |
| `TestValidateLeafValueEnumRefusal` | `internal/component/config/enum_test.go` | typed refusal recovered with `errors.As`, fields filled, path as passed | |
| `TestParserEnumRefusalNamesFullPath` | `internal/component/config/parser_test.go` | path includes list key `peer1` (A-4) | |
| `TestParseAtEnumRefusalNamesContextPath` | `internal/component/config/parser_test.go` | `ParseAt` seeds the path stack from `contextPath` (AC-19) | |
| `TestParserMissingEnumValueRefusal` | `internal/component/config/parser_leaf_fix_test.go` | AC-18 | |
| `TestInlineLeafListEnumRefusal` | `internal/component/config/parser_list_test.go` | AC-5: inline and bracket members (`validateInlineListChildValue`, `storeValueOrArray`) | |
| `TestSetParserLeafListEnumRefusal` | `internal/component/config/setparser_test.go` | AC-5: `validateValueOrArrayItems` | |
| `TestValidateEnumerationSharedBody` | `internal/component/config/yang/validator_test.go` | AC-17, including the path named once in `ValidationError.Error()` and `Expected` in declared order | |
| `TestValidateValueAtPathEnumRefusal` | `internal/component/cli/completer_validate_test.go` | AC-6 body, and both fix lines (with and without a suggestion) | |
| `TestCliFormatRefusalUsesSharedBody` | `internal/component/cli/model_keys_test.go` | AC-20 | |
| `TestFormatPeerErrorEnumUsesSharedBody` | `internal/component/cli/validator_test.go` | text 4 replaced | |
| `TestWebFormRefusesEnumAtSet` | `internal/component/web/handler_config_form_test.go` | AC-7 crafted POST at handler level | |
| `TestWebTerminalSetRefusesEnum` | `internal/component/web/cli_terminal_test.go` | AC-7 at `executeTerminalSet`: refused, nothing stored | |
| `TestConfigValidationHookEnumRefusal` | `cmd/ze/hub/rfc8907_api_test.go` | AC-8/AC-9 frame at the hook REST and gNMI share: path, set-form fix, no `line N:`, no `write:` (A-6) | |
| `TestRunValidationParserEnumDiagnostic` | `internal/component/config/cli/cmd_validate_test.go` | AC-11 and AC-21: `errors.As` recovers the refusal through the parser's `line N:` frame | |
| `TestEnumRefusalMasksSensitiveValue` | `internal/component/config/mask_test.go` | AC-16, R-3: placeholder in the value slot, no suggestion line, no `actual` | |
| `TestValidateEnumArgRefusal` | `internal/component/command/argvalidate_test.go` | AC-12 enum and union arms | |
| Rewritten: the pinning Go tests and `set-cli-format.et` in Current Behavior | as listed | assert the new body (value, path, every accepted value) instead of the old words; each assertion stays at least as strong (R-8) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| suggestion distance | 0 to `len(input)/2` | distance `len(input)/2` suggests | N/A | `len(input)/2 + 1` gives no suggestion |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `enum-refusal-proposes-valid-values` | `test/parse/enum-refusal-proposes-valid-values.ci` | AC-1, AC-2, AC-3, AC-11 via `ze config validate` | |
| `enum-refusal-inline-leaf-list` | `test/parse/enum-refusal-inline-leaf-list.ci` | AC-5, inline and bracket members | |
| `enum-refusal-set-format` | `test/parse/enum-refusal-set-format.ci` | AC-5, set-format leaf-list member | |
| `enum-refusal-config-set` | `test/parse/enum-refusal-config-set.ci` | AC-6 through offline `ze config set` | |
| `enum-refusal-daemon-start` | `test/plugin/enum-refusal-daemon-start.ci` | AC-10 start | |
| `enum-refusal-reload` | `test/reload/enum-refusal-reload.ci` (shape of `config-reload-invalid-validator.ci`) | AC-10 reload | |
| `enum-refusal-set` | `test/editor/validation/enum-refusal-set.et` | AC-6 | |
| `enum-refusal-load` | `test/editor/validation/enum-refusal-load.et` | AC-19 | |
| `config-enum-refusal` | `test/web/config-enum-refusal.wb` (shape of `test/web/cli-set-commit-discard.wb`) | AC-7 through the web CLI terminal `set` | |
| `api-config-enum-refusal` | `test/plugin/api-config-enum-refusal.ci` (shape of `api-config-commit-reject.ci`) | AC-8 | |
| `gnmi-set-enum-refusal` | `test/plugin/gnmi-set-enum-refusal.ci` | AC-9 | |
| `command-arg-enum-refusal` | `test/plugin/command-arg-enum-refusal.ci` | AC-12, enum and union arguments | |
| updated | `test/parse/asn4-refuses-boolean-spelling.ci`, `test/parse/capability-leaf-refuses-presence-spelling.ci`, `test/isis/isis-auth.ci`, `test/editor/pipe/set-cli-format.et` | expectations follow the new body; each still proves its refusal | |

### Interop Tests (Scope: protocol)
N-A: config refusal text; no peer sees it and no wire byte changes.

## Files to Modify
- `internal/component/config/schema.go` - `LeafNode` and `ValueOrArrayNode` gain per-value help; `ValidateLeafValue` and `ValidateListKey` take the path and return the typed refusal.
- `internal/component/config/yang_schema.go` - `yangToLeaf` fills per-value help from `yang.EnumValueSummaries`; the leaf-list builder switches from goyang's sorted `entry.Type.Enum.Names()` to `yang.EnumNamesDeclared` and fills help the same way.
- `internal/component/config/yang/enum.go` - a declared-order reader for a union's enum members (union member order, then each member's declared order), beside `EnumNamesDeclared`; `EnumValueNames` stays the sorted completion order.
- `internal/component/config/parser.go` - path stack in the parser, seeded from `contextPath` in `ParseAt`; `errorf` gains a typed line error (or `%w`) so the refusal survives the `line N:` frame; enum refusal framed with `line N:` and the `write:` line (`leafValueFix` kept unchanged); the missing-value arm uses the shared body.
- `internal/component/config/parser_list.go` (`validateInlineListChildValue`, `storeValueOrArray`), `parser_freeform.go`, `setparser.go` (`validateValueOrArrayItems`), `setparser_meta.go` - pass the path and build the shared refusal.
- `internal/component/config/retired.go` - `UnknownKeywordHint` renders its hint through the one helper the refusal uses.
- `internal/component/config/yang/validator.go` - `validateEnumeration` fills `Message` with the shared body and `Expected` in declared order; `Type` and `Got` unchanged; `ValidationError` gains `Unwrap` to the refusal, and `Error()` names the path once when a refusal is attached; `validateUnion` too if Q-2 is yes. Its `// Design:` doc `yang-config-design.md` is updated.
- `internal/component/config/cli/cmd_validate.go` - `errors.As` on a parser error: code `config-yang-enum`, expected, actual (omitted when secret), dotted path, repair.
- `internal/component/config/cli/cmd_set.go` - offline `ze config set` prints the set-form fix line from the refusal.
- `cmd/ze/hub/api.go` - `configValidationHook` frames the refusal for REST and gNMI: path and set-form fix, no `line N:`, no `write:`.
- `internal/component/cli/completer.go`, `completer_validate.go` - editor refusal for an enum (and a union, Q-2) through the shared type; fix line `set <path> <value>`, or the alternation when there is no suggestion.
- `internal/component/cli/model_keys.go` - `set cli format` refusal through the shared type; `cliFormatNames` stops re-sorting.
- `internal/component/cli/editor_commands.go` - `ValidateListKey` call passes the path.
- `internal/component/cli/validator.go` - `formatPeerError` enum arm and the generic arm use the shared body.
- `internal/component/web/cli_terminal.go` - `executeTerminalSet` checks the value with `ValidateValueAtPath` before `SetValue`, masked as today.
- `internal/component/web/handler_config_form.go`, `handler_config_entry.go` - validate with `ValidateLeafValue` (type and YANG restrictions), path passed, message masked as today.
- `internal/component/command/argvalidate.go` (`validateEnum`, `validateUnion`), `argdef.go` and `internal/component/config/yang/command.go` - enum ArgDef carries per-value help in declared order; both arms use the shared type.
- `rfc/discrimination/rfc7950.json` - the six records named under RFC Summaries, re-recorded by `./le rfc discriminate-record`, never edited by hand.
- The pinning tests listed in Current Behavior, `test/editor/pipe/set-cli-format.et` included.
- Docs: `docs/architecture/config/syntax.md`, `docs/architecture/config/yang-config-design.md`, `docs/guide/config-editor.md`, `docs/guide/web-interface.md`, `docs/architecture/web-interface.md` (declared by the web handlers and `cli_terminal.go`: the form and the terminal now refuse an enum at set), `docs/architecture/api/commands.md`, `docs/architecture/api/architecture.md` (declared by `cmd/ze/hub/api.go`; it says nothing of the commit validation hook today, so the edit is one sentence on the REST/gNMI refusal frame where the page covers config sessions, or none if the page covers no config session at implementation).

## Files to Create
- `internal/core/suggest/refusal.go` - the refusal type (path, value, accepted values with help, suggestion) and its text renderer.
- `internal/core/suggest/refusal_test.go`
- `internal/component/web/cli_terminal_test.go`, `internal/component/web/handler_config_form_test.go`, `internal/component/cli/completer_validate_test.go` (none exists today).
- `internal/test/fixture/plugin_fixture_enum_refusal.go` - two `le test fixture` clients, registered in the plugin fixture table beside `plugin/api-config-commit-reject`: `plugin/api-config-enum-refusal` (REST session set then commit) and `plugin/gnmi-set-enum-refusal` (a gNMI `SetRequest` over the vendored `github.com/openconfig/gnmi` proto, asserting `Aborted` and the status message).
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
| 10 | Test infrastructure changed? | Yes | two new `le test fixture` clients (REST, gNMI `Set`); check the page that lists fixtures through `ai/CODE-TO-DOCS.md` for `internal/test/fixture/` at implementation |
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
2. **Phase: schema carries help and declared order** -- `yangToLeaf` and the leaf-list builder store per-value `ze:help`; the leaf-list builder moves to `EnumNamesDeclared`.
   - Tests: `TestYangToLeafCarriesEnumHelp`, `TestValueOrArrayEnumDeclaredOrder`
3. **Phase: file paths** -- parser path stack (seeded by `ParseAt`), typed line error, `validateInlineListChildValue`, `storeValueOrArray`, `validateValueOrArrayItems`, the missing-value arm, set/meta/freeform parsers, `cmd_validate` diagnostic mapping; rewrite the pinned parser tests and three `.ci`.
   - Tests: `TestParserEnumRefusalNamesFullPath`, `TestParseAtEnumRefusalNamesContextPath`, `TestParserMissingEnumValueRefusal`, `TestInlineLeafListEnumRefusal`, `TestSetParserLeafListEnumRefusal`, `TestRunValidationParserEnumDiagnostic`, the parse, set-format, start and reload `.ci`
4. **Phase: tree walk** -- `validateEnumeration` message from the shared body, `Expected` in declared order, `ValidationError.Unwrap` and the single path in `Error()`; `Type`/`Got` unchanged; run the RFC-tagged enum tests untouched.
   - Tests: `TestValidateEnumerationSharedBody`
5. **Phase: interactive and remote** -- editor `ValidateValueAtPath` (editor `set` and offline `ze config set`), `set cli format`, `formatPeerError`, web CLI terminal `set`, web form sites, the `configValidationHook` frame, the two `le test fixture` clients, then the REST and gNMI `.ci`.
   - Tests: the editor, load, cli-format, web, hook, API and gNMI rows
6. **Phase: command arguments** -- ArgDef help from YANG in declared order, `validateEnum` and `validateUnion`; rewrite `TestSendArgumentsAreDeclared` and `TestValidateArgStringEnum`.
7. **Phase: unions (Q-2) and docs** -- unions as answered; the pages in the Documentation checklist; re-grep `test/parse/bmp-route-action-refuses-redistribute.ci` if it has landed and confirm it pins only `route-action`, `redistribute` and `monitor` (R-7).
8. **Phase: discrimination records** -- for each of the six `rfc/discrimination/rfc7950.json` records named under RFC Summaries whose producer this spec changed (`validateEnumeration` ×2, `yangToLeaf`, `validateUnion` ×3 if Q-2), run `./le rfc discriminate-record` so the record observes the red over the new producer; the records ride in the commit that carries the producer edit (R-10).
   - Verify: `./le rfc check` reports no `discrimination:` line for RFC 7950.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | grep for `invalid enum`, `expected one of`, `must be one of`, `(expected enum)`, `not a valid enumeration value`, `(valid: `, `or one of`, `needs a value, one of` in non-test Go under `internal/component/config`, `internal/component/cli`, `internal/component/command`, `internal/component/web`, `internal/component/api` and `cmd/ze/hub`: zero producers remain for a YANG enum (the unscoped tree holds unrelated hits such as `internal/chaos/scenario/target.go`) |
| Naming | JSON diagnostic keys unchanged (`expected`, `actual`, `path`, `code`) |
| Data flow | accepted values and help read only through `EnumNamesDeclared` / `EnumValueSummaries` (or the `LeafNode` fields they fill) |
| Rule: principles (one declaration) | no list of enum values anywhere in Go for this purpose |
| Rule: testing | each rewritten assertion is as strong as before; none deleted |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| one refusal type | `gopls symbols internal/core/suggest/refusal.go` |
| old texts gone | the grep in the Critical Review "Correctness" row |
| every path proven | every Wiring Test row's test exists and passes |
| discrimination records fresh | `./le rfc check` carries no RFC 7950 `discrimination:` line |

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
- Three paths do not refuse at `set` (web form: type check only; web CLI terminal: no check; REST/gNMI: no check), so "the same message on every path" also means the same moment for the web terminal, where the operator is looking at the line they typed. REST/gNMI keep the commit-time refusal (A-3).
- The enum value order was not one fact: goyang's `EnumType.Names()` sorts, so the tree walk, the leaf-list builder and `set cli format` listed values alphabetically while the parser listed them as declared.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Owner direction (c), 2026-10-13: "M2 go with (c), friendly errors everywhere" | (d) a YANG-declared reason for a removed value | the owner chose (c); (d) is not designed here (Known Limitations) |
| A typed refusal value (path, value, accepted values with help, suggestion) built where each check refuses, rendered once by its `Error()`; surfaces add only their frame | (B) keep `fmt.Errorf` everywhere and add a shared string helper; (C) refuse only in the YANG tree walk and remove the parser and editor enum checks | B keeps the text but loses structure: `ze config validate` JSON and the editor fix line need the fields, and every call site still formats. C changes the moment of the verdict (editor `set` and file load would refuse later, without a line number) and is a layering change no one asked for. A is the one declaration of the text and the only shape the JSON can read |
| The type lives in `internal/core/suggest` | `internal/component/config`; `internal/component/config/yang`; a new core package | `config/yang` imports `command` and `config` imports `config/yang`, so only a leaf package is importable by all producers; `suggest` already owns "did you mean" and imports nothing from ze, so a new package would hold one type |
| `ValidateLeafValue` and `ValidateListKey` take the path as a parameter | a setter on the returned error filled by callers | the compiler then finds every caller; a setter can be forgotten silently (`ai/rules/principles.md`, silent wrong value) |
| Per-value text is `ze:help` only | also read the enum `description` | `ai/rules/config.md`: an enum value carries its text as `ze:help` alone; no surface reads an enum description |
| Accepted values in YANG declaration order, everywhere, `Expected` included | sorted (`EnumValueNames`, the completion order); keep `Expected` alphabetical | the parser and `leafValueFix` already use declared order and two `.ci` pin it; declared order is the author's intent; one order means the JSON `expected` and the text list agree. `Expected` readers assert `Contains` (R-9) |
| Order source per producer: `ValidateLeafValue`, the parser arms and `ValidateValueAtPath` read `LeafNode.Enums` (`EnumNamesDeclared`, already); `storeValueOrArray` and `validateValueOrArrayItems` read `ValidValues`, which the leaf-list builder now fills from `EnumNamesDeclared` instead of goyang's `Names()` (which `sort.Strings`); `validateEnumeration` replaces `yangType.Enum.Names()` with the declared reader; `set cli format` drops `slices.Sorted`; command args use `namesByValue` (`EnumNamesDeclared`'s own order) already. Unions: union member order, then each enum member's declared order, through the new union reader in `yang/enum.go` | leave each producer's order as found | goyang's `Names()` is alphabetical, so three of the producers list values in an order the YANG author never wrote. `EnumNamesDeclared` answers value order, which is declaration order for every enum with implicit values |
| The body is identical everywhere; only the frame differs: `line N:` and `write:` for a file; a `set <path> <value>` fix line for editor `set`, offline `ze config set` and the web terminal; the path and the same set-form fix, with no `line N:` and no `write:`, for REST and gNMI (the client sent a set, not a file); code and fields for JSON | one identical string including the fix line | the fix differs by surface (`ai/rules/user-facing-errors.md`: "the exact next step"); a file is edited, the editor and the API take a `set` |
| The fix line when no value is close: the file keeps `leafValueFix` unchanged (`write: <leaf> <a\|b\|c>;`, `enable`/`disable` for a boolean spelling); a set-form surface writes `set <path> <a\|b\|c>` in declared order. With a suggestion, the set-form fix names it (`set <path> require`) and `write:` stays as `leafValueFix` writes it | change `write:` to the suggestion | `write:` and its boolean choice are pinned (`TestEnumLeafRefusalNamesTheReplacementLine`, two `.ci`); the suggestion is already in the body |
| Closest match through `suggest.Command`, in the hint form ` (did you mean X?)` produced by one helper in `internal/core/suggest` that `UnknownKeywordHint` also calls | a new helper with a different bound; a separate quoted line `did you mean "X"?` | an enum value is a short kebab-case word the operator types in the same position and the same way as a keyword, so its typical error is the same one-or-two-letter typo the keyword helper is bounded for; the `len(input)/2` bound refuses to propose a different word (`redistribute` is never "corrected" to `monitor`), which is what an operator choosing between named modes needs. One distance and one wording across CLI, unknown key and enum value |
| A sensitive leaf's refusal is built with a secret flag from `config.LeafHoldsSecret`: `config.SecretDataPlaceholder` in the value slot, no suggestion computed, JSON `actual` omitted | rely on `MaskSecretInMessage` alone | masking replaces the value string, but a suggestion close to the secret leaks most of it in a different string; the flag stops the leak at the builder, masking stays as the second defence |
| Every frame keeps the refusal reachable by `errors.As`: the parser's `errorf` gains a typed line error (or `%w`), `ValidationError` gains `Unwrap` | string matching in `cmd_validate` | `errorf` formats with `%s` today and drops the chain, so the JSON mapping could not see the refusal |
| `ValidationError.Error()` returns the refusal text (prefixed `line N:` when set) when a refusal is attached, instead of `<type> error at <path>: <message>` | keep the prefix | the shared body already names the path, so the prefix would print it twice |
| JSON `path` stays the dotted form the tree walk writes (`section.leaf.path`); the human text writes the path space-separated, as an operator types it | one form for both | dotted is what `ze config validate` JSON already emits for every YANG error, and a consumer must not see two formats for one field |
| The parser's missing-value refusal (`needs a value, one of`) is in scope | leave it | it lists an enum's values to an operator, so "every YANG enum, every path" covers it; the body has no quoted value, the `write:` line stays |

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
| Q-6 | Web: refuse an out-of-enum value at set time (new) rather than only at commit, on the web CLI terminal `set` (no check at set today) and on a form POST? The form renders an enum leaf as a `<select>`, so only the terminal or a crafted request can send such a value | Yes: the terminal operator is looking at the line they typed, and it is the same check the SSH editor `set` runs; the form handler refuses a crafted POST the same way | |

No design approval is recorded. Status stays `design` until the owner answers Q-2 to Q-6 and approves the design.

## Review Gate

### Design review history

| Round | Date | Result | Disposition |
|-------|------|--------|-------------|
| 1 | 2026-10-13 | 1 BLOCKER, 8 ISSUE, 6 NIT | Each finding verified at its producer; all fixed in this edit, none disputed. B1 producer list completed (`storeValueOrArray`, `validateValueOrArrayItems`, `set cli format` pinned by `set-cli-format.et`, command `validateUnion`, offline `ze config set` corrected to `cmd_set.go` → `ValidateValueAtPath`, web CLI terminal `set`, editor `load` via `ParseAt` with a seeded path) in the texts table, Wiring Test, Files to Modify, pinned tests and the Correctness grep (now scoped, plus `(valid: `, `or one of`, `needs a value, one of`); I2 six `rfc7950.json` records named, Phase 8 re-records them, R-10; I3 order source per producer stated (goyang `Names()` sorts), leaf-list builder and `Expected` move to declared order, union order named, R-9; I4 A-6 settled: two `le test fixture` clients (REST, gNMI `Set`) and `TestConfigValidationHookEnumRefusal`; I5 AC-7 moved to the web CLI terminal `set`, crafted POST kept as `TestWebFormRefusesEnumAtSet`, Q-6 reworded; I6 secret flag from `config.LeafHoldsSecret`, placeholder, no suggestion, no `actual` (AC-16, R-3); I7 typed line error or `%w`, AC-21, `TestRunValidationParserEnumDiagnostic`; I8 REST/gNMI frame defined in AC-8, AC-9 and Key Design Decisions; I9 BMP spec re-read, R-7 rewritten against its three pinned words and cited test names; N10 domain reason for `suggest.Command`; N11 `TestValidateTree_FamilyModeEnum` and `TestValidateTree_AddPathDirectionEnum` listed; N12 one hint form ` (did you mean X?)` shared with `UnknownKeywordHint`; N13 `ValidationError.Error()` names the path once (AC-17); N14 dotted JSON path (AC-11); N15 missing-value text in scope (AC-18) and the no-suggestion fix line defined (AC-6, Key Design Decisions) |
| 2 | owed | not run | Round 2 of the design review is owed over the round-1 edit, by a reader who did not author it, before the design is presented for approval |

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
