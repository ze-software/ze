# Spec: YANG-Typed Arguments for Operational Commands

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/10 |
| Updated | 2026-05-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/command/node.go` - Node struct (ValueHints, DynamicChildren)
4. `internal/component/config/yang/command.go` - BuildCommandTree, mergeYANGEntry
5. `internal/component/command/completer.go` - TreeCompleter.Complete, matchChildren
6. `internal/component/plugin/server/command.go` - Dispatcher.Dispatch (line 399)
7. `internal/component/config/yang/validator.go` - validateYangType (line 220)

## Task

Add YANG leaf type constraints inside ze:command containers so the schema
enforces argument types (uint, enum, pattern, union). The completer generates
suggestions automatically from YANG enum/union types, eliminating manual
ValueHints wiring. The dispatcher validates arguments against the schema
before calling handlers. Migrate all existing commands (23 commands, 54
typed arguments) to use YANG-declared argument types.

## Required Reading

### Architecture Docs
- [ ] `patterns/cli-command.md` - command registration, YANG tree definition, handler dispatch
  -> Constraint: Container nesting mirrors CLI path. ze:command marks executable nodes.
  -> Constraint: Handler signature is func(ctx *CommandContext, args []string) (*plugin.Response, error). Must not change.
- [ ] `docs/architecture/api/commands.md` - command tree types, ValueHints vs DynamicChildren
  -> Decision: ValueHints are terminal argument values, DynamicChildren are navigable path segments. Both CLI interactive and shell completion use shared TreeCompleter.
  -> Constraint: Both CLI interactive and shell completion use the same TreeCompleter from the command package.
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` - all ze: extensions
  -> Constraint: ze:command takes a handler argument string (WireMethod). ze:validate references custom validator functions for runtime-determined sets (e.g., plugin families).
  -> Decision: ze:validate exists for runtime-dynamic sets. YANG native types (enum, pattern, range) handle static sets. No new extension needed.

### Source Files
- [ ] `internal/component/command/node.go` - Node struct definition
  -> Constraint: ValueHints func() []Suggestion on Node with type "value" for terminal args. No argument type information exists on Node today.
- [ ] `internal/component/config/yang/command.go` - BuildCommandTree and mergeYANGEntry
  -> Constraint: mergeYANGEntry filters child.Config != TSFalse. Leaves have Config == TSUnset (inherited, not explicit). goyang does not propagate config false to leaf descendants; it provides ReadOnly() which walks the parent chain instead.
  -> Decision: Fix approach is a second pass on leaf children (entry.Type != nil) of ze:command nodes, not changing the main filter.
- [ ] `internal/component/command/completer.go` - TreeCompleter.Complete and matchChildren
  -> Constraint: matchChildren returns static children (type "command"), DynamicChildren, and ValueHints (type "value"). All prefix-filtered.
- [ ] `internal/component/plugin/server/command.go` - Dispatcher.Dispatch
  -> Constraint: At line 482-489, extracts remaining text as args []string via tokenize(). Handler called at line 492+. No validation between extraction and handler call.
  -> Decision: Validation insertion point is between tokenize(remaining) and handler invocation.
- [ ] `internal/component/config/yang/validator.go` - validateYangType
  -> Decision: Already handles Ystring (length, pattern), Yuint8-64 (range), Yint8-64 (range), Yenum (name match), Ybool, Yunion (try each member type). Reusable patterns for command args.
  -> Constraint: Config validator receives `any` (JSON-parsed values). Command args arrive as raw strings. A string-to-type conversion layer is needed (ValidateArgString).
- [ ] `internal/component/command/valuehints.go` - WireValueHints
  -> Constraint: Currently manual wiring per command: rib families (navigatePath show/bgp/rib), log levels (navigatePath log/set), FD limit max (navigatePath set/system/file-descriptors). Each requires explicit Go code in wireFoo() functions.
  -> Decision: Runtime-dynamic hints (families from plugin registry) must remain as Go callbacks. Only static hints (enum values known at schema time) should move to YANG.
- [ ] `vendor/github.com/openconfig/goyang/pkg/yang/yangtype.go` - YangType struct
  -> Constraint: YangType has Kind (TypeKind), Enum (*EnumType with name-to-value map), Pattern ([]string for XSD regex), Range (YangRange for integer constraints), Type ([]*YangType for union members). All available at YANG parse time.

**Key insights:**
- mergeYANGEntry skips leaves because it checks Config (explicit annotation) not ReadOnly() (inherited from parent). Leaves inside config false containers have Config == TSUnset. Fix: after identifying a ze:command node, do a second pass reading leaf children by checking entry.Type != nil.
- The config Validator validates `any` (from JSON). Command args are raw strings. Need a ValidateArgString function that attempts strconv parse then validates against the YANG type.
- Enum values from YangType.Enum directly become Suggestion entries, replacing manual ValueHints.
- Union types (e.g., uint64 | enum "max"): extract enum members for completion, use full union for validation (try each member type in order).
- Dispatcher has a clean insertion point for validation: after tokenize(remaining) at line 489 and before handler invocation at line 492.
- ze:validate extension coexists with YANG native types. ze:validate handles runtime-dynamic sets (plugin families); native types handle static sets declared in the schema.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cmd/set/system_linux.go` - validates args[0] == "max" or strconv.ParseUint; rejects invalid with usage error
- [ ] `internal/component/cmd/show/fd_linux.go` - checks args for "detail" or "summary" enum; defaults to "summary"
- [ ] `internal/component/cmd/show/ping.go` - parses keyword-value pairs: count (uint via Atoi), timeout (duration string)
- [ ] `internal/component/cmd/show/traceroute.go` - parses max-hops (uint), timeout (duration), probes (uint) as keyword-value pairs
- [ ] `internal/component/cmd/show/audit.go` - parses keyword-value pairs: action, actor, surface (string), since, until (RFC3339), count (uint)
- [ ] `internal/component/cmd/show/kernel_log_linux.go` - parses level (enum) and count (uint) as keyword-value pairs
- [ ] `internal/component/cmd/show/capture_interface.go` - parses count (uint 1-10000), snap-len (uint 64-65535), duration (string), format (enum: pcap, text)
- [ ] `internal/component/cmd/show/sockets_linux.go` - first arg enum (tcp, udp), keyword-value: state (string), port (uint)
- [ ] `internal/component/cmd/show/goroutines.go` - first arg positional enum: summary, blocked, full
- [ ] `internal/component/cmd/show/neighbors.go` - positional enum: ipv4, ipv6, any, all
- [ ] `internal/component/cmd/show/ip.go` - --limit N keyword-value (uint), --family (enum: ipv4, ipv6, any, all)
- [ ] `internal/component/cmd/show/profile.go` - positional enum: cpu, heap, goroutine, allocs; keyword: duration
- [ ] `internal/component/cmd/show/capture_raw.go` - positional enums: start/stop/dump, l2tp/bgp, pcap/json; keyword: count (uint)
- [ ] `internal/component/cmd/show/dns.go` - positional string (hostname); keyword: type (enum: A, AAAA, MX, NS, TXT, CNAME, PTR); cache subcommand: positional enum stats/list/record
- [ ] `internal/component/cmd/show/tcp_check.go` - positional: host (string), port (uint); keyword: source (string), timeout (duration)
- [ ] `internal/component/cmd/show/crashes.go` - positional: "latest" or filename (union of enum + string)
- [ ] `internal/component/cmd/log/log.go` - log set: positional logger (string) + level (enum); log recent: keyword level (enum), component (string), count (uint)

**Behavior to preserve:**
- Handler signature `func(ctx *CommandContext, args []string) (*plugin.Response, error)` unchanged
- Handlers still receive all args and can do additional validation beyond YANG types
- Shell completion (`ze completion words`) continues working via TreeCompleter
- CLI interactive completion continues working via TreeCompleter
- All existing commands continue to work identically with valid inputs
- Runtime-dynamic ValueHints (plugin families for rib) remain as Go callbacks

**Behavior to change:**
- YANG command containers gain leaf children declaring argument types
- BuildCommandTree reads leaf children and populates ArgDefs on command.Node
- Completer auto-generates suggestions from ArgDefs enum values (replaces manual ValueHints for static sets)
- Dispatcher validates args against ArgDefs before calling handler (rejects early with clear error)
- Manual ValueHints wiring in valuehints.go reduced to runtime-dynamic cases only (wireRibHints stays; wireFDSetHints and wireLogSetHints removed)

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- YANG -cmd modules loaded at startup by Loader (registered via init() in schema/register.go for each verb)
- Format at entry: YANG source text, parsed by goyang into *yang.Entry tree

### Transformation Path
1. goyang parses YANG source into *yang.Entry tree with all type metadata (Kind, Enum, Pattern, Range)
2. mergeYANGEntry walks entry.Dir children of -cmd modules, building command.Node tree from config false containers
3. NEW: For nodes with ze:command extension, second pass reads leaf children (entry.Type != nil) and extracts YangType into ArgDefs on the Node
4. TreeCompleter.matchChildren reads ArgDefs enum values to generate type "value" suggestions and leaf names to generate keyword suggestions
5. Dispatcher.Dispatch validates args against ArgDefs using a two-phase algorithm: Phase 1 scans for keyword-value pairs (args matching leaf names consume the next arg as a typed value); Phase 2 validates remaining positional args against enum types. Runs between tokenize at line 489 and handler call at line 492.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG schema -> command.Node | mergeYANGEntry reads leaf types into ArgDefs field | [ ] |
| ArgDefs -> completer | matchChildren reads ArgDefs enum values as Suggestion entries | [ ] |
| ArgDefs -> dispatcher | Dispatch runs two-phase validation (keyword extraction then positional matching) before handler | [ ] |

### Integration Points
- `BuildCommandTree` in `yang/command.go` - extends mergeYANGEntry to read leaf children of ze:command nodes
- `Node` in `command/node.go` - new ArgDefs field ([]ArgDef)
- `matchChildren` in `command/completer.go` - reads ArgDefs for auto-generated suggestions
- `Dispatch` in `server/command.go` - validates args against ArgDefs before handler call
- `WireValueHints` in `command/valuehints.go` - static entries removed, runtime-dynamic entries kept

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze completion words run show system goroutines` | -> | matchChildren reads ArgDefs enum values from YANG leaf | `completion-words-goroutine-modes.ci` |
| `ze completion words run set system file-descriptors` | -> | matchChildren reads ArgDefs union enum values from YANG leaf | `completion-words-fd-limit.ci` (exists) |
| `ze completion words run show audit` | -> | matchChildren shows keyword arg names from YANG leaves | `completion-words-audit-keywords.ci` |
| Invalid positional enum arg via dispatcher | -> | Dispatcher validates positional enum against ArgDef | `TestDispatcherArgValidationEnum` |
| Invalid union arg via dispatcher | -> | Dispatcher validates union (uint64 or max) against ArgDef | `TestDispatcherArgValidationUnion` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | YANG leaf with enum type inside ze:command container | BuildCommandTree creates ArgDef with EnumValues populated from YangType.Enum |
| AC-2 | YANG leaf with union(uint64, enum max) type | ArgDef has Kind=ArgUnion, EnumValues=["max"] extracted from enum member |
| AC-3 | YANG leaf with uint32 and range constraint | ArgDef has Kind=ArgUint with range metadata from YangType.Range |
| AC-4 | YANG leaf with string pattern constraint | ArgDef has Kind=ArgString with Pattern from YangType.Pattern |
| AC-5 | Tab completion on command with enum ArgDef | Completer returns enum values as type "value" suggestions |
| AC-6 | Tab completion on command with keyword ArgDefs | Completer returns leaf names as suggestions |
| AC-7 | Command invoked with valid args matching ArgDefs | Dispatcher passes validation, handler receives args unchanged |
| AC-8 | Command invoked with invalid enum arg | Dispatcher rejects before handler call with clear error message |
| AC-9 | Command invoked with invalid uint arg (non-numeric) | Dispatcher rejects before handler call |
| AC-10 | Command invoked with uint arg out of declared range | Dispatcher rejects before handler call |
| AC-11 | Command invoked with union arg matching enum member ("max") | Dispatcher accepts the arg |
| AC-12 | Command invoked with union arg matching uint member ("1024") | Dispatcher accepts the arg |
| AC-13 | `show ping count 5` (keyword-value pair) | Dispatcher matches "count" to leaf name, validates "5" as uint32, accepts |
| AC-14 | `show ping count` (keyword with no value) | Dispatcher rejects: "count requires a value" |
| AC-15 | `show system goroutines invalid` (invalid positional) | Dispatcher rejects: "invalid" does not match any enum value |
| AC-16 | `show ping 192.168.1.1 count 5 timeout 3s` (mixed positional + keyword) | Dispatcher extracts keyword pairs (count=5, timeout=3s), validates types, passes positional "192.168.1.1" as string |
| AC-17 | Command with mandatory ArgDef and no matching arg | Dispatcher rejects: "required argument missing: dest" |
| AC-18 | All 23 commands have YANG leaves declaring argument types | All cmd YANG schemas have leaf children inside ze:command containers |
| AC-19 | Manual ValueHints for static enums removed | wireFDSetHints and wireLogSetHints removed from valuehints.go; wireRibHints stays (runtime-dynamic) |
| AC-20 | Runtime-dynamic ValueHints (plugin families) still work | completion-words-values.ci still passes |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestArgDefFromEnumYANG` | `internal/component/config/yang/command_test.go` | AC-1: enum leaf produces ArgDef with EnumValues | |
| `TestArgDefFromUnionYANG` | `internal/component/config/yang/command_test.go` | AC-2: union leaf produces ArgDef with extracted enum values | |
| `TestArgDefFromUintRangeYANG` | `internal/component/config/yang/command_test.go` | AC-3: uint leaf with range produces ArgDef with range metadata | |
| `TestArgDefFromPatternYANG` | `internal/component/config/yang/command_test.go` | AC-4: string leaf with pattern produces ArgDef with Pattern | |
| `TestValidateArgStringEnum` | `internal/component/command/argvalidate_test.go` | AC-8: invalid enum string rejected |  |
| `TestValidateArgStringUint` | `internal/component/command/argvalidate_test.go` | AC-9: non-numeric string rejected for uint arg | |
| `TestValidateArgStringUintRange` | `internal/component/command/argvalidate_test.go` | AC-10: out-of-range uint rejected | |
| `TestValidateArgStringUnion` | `internal/component/command/argvalidate_test.go` | AC-11, AC-12: union accepts enum member and uint member | |
| `TestCompleterArgDefsEnumSuggestions` | `internal/component/command/completer_test.go` | AC-5: enum values appear as suggestions | |
| `TestCompleterArgDefsKeywordSuggestions` | `internal/component/command/completer_test.go` | AC-6: leaf names appear as keyword suggestions | |
| `TestDispatcherArgValidation` | `internal/component/plugin/server/command_test.go` | AC-7, AC-8: valid args pass, invalid rejected | |
| `TestDispatcherKeywordExtraction` | `internal/component/plugin/server/command_test.go` | AC-13, AC-14: keyword-value pairs matched by leaf name | |
| `TestDispatcherPositionalMatching` | `internal/component/plugin/server/command_test.go` | AC-15: unmatched args validated against enum types | |
| `TestDispatcherMixedArgs` | `internal/component/plugin/server/command_test.go` | AC-16: mixed positional + keyword args parsed correctly | |
| `TestDispatcherMandatoryMissing` | `internal/component/plugin/server/command_test.go` | AC-17: missing mandatory arg rejected | |
| `TestDispatcherNoArgDefsPassthrough` | `internal/component/plugin/server/command_test.go` | Commands without ArgDefs skip validation entirely | |
| `TestArgDefsPopulated` | `internal/component/config/yang/command_test.go` | AC-18: all 23 commands have ArgDefs after BuildCommandTree | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| uint32 (general) | 0-4294967295 | 4294967295 | N/A (unsigned) | 4294967296 |
| uint64 (file-descriptors) | 0-18446744073709551615 | 18446744073709551615 | N/A | overflow string |
| capture count | 1-10000 | 10000 | 0 | 10001 |
| snap-len | 64-65535 | 65535 | 63 | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `completion-words-goroutine-modes` | `test/ui/completion-words-goroutine-modes.ci` | Tab completion for show system goroutines offers summary, blocked, full | |
| `completion-words-fd-limit` | `test/ui/completion-words-fd-limit.ci` | Tab completion for set system file-descriptors offers max (exists) | |
| `completion-words-audit-keywords` | `test/ui/completion-words-audit-keywords.ci` | Tab completion for show audit offers keyword names (action, actor, count, etc.) | |
| `completion-words-levels` | `test/ui/completion-words-levels.ci` | Tab completion for log set still works after ValueHints migration (exists) | |
| `completion-words-values` | `test/ui/completion-words-values.ci` | Tab completion for rib still shows families via runtime ValueHints (exists) | |

### Interop Tests (MANDATORY for protocol features)
N/A -- this is internal CLI/completion infrastructure, not protocol code.

### Future (if deferring any tests)
- None deferred

## Files to Modify
- `internal/component/command/node.go` - add ArgDef type, ArgKind constants, ArgDefs field on Node
- `internal/component/config/yang/command.go` - mergeYANGEntry reads leaf children of ze:command nodes, extracts YangType into ArgDefs
- `internal/component/command/completer.go` - matchChildren uses ArgDefs for auto-generated enum and keyword suggestions
- `internal/component/plugin/server/command.go` - Dispatch validates args against matched command's ArgDefs before handler call
- `internal/component/command/valuehints.go` - remove wireFDSetHints and wireLogSetHints (keep wireRibHints for runtime-dynamic families)
- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` - add leaf children to all show command containers with typed args
- `internal/component/cmd/set/schema/ze-cli-set-cmd.yang` - add leaf to set system file-descriptors container
- `internal/component/cmd/log/schema/ze-cli-log-cmd.yang` - add leaf children to log set and log recent containers
- `docs/architecture/api/commands.md` - document ArgDefs, YANG leaf convention for command arguments

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | No new RPCs, only leaf additions to existing command containers |
| CLI commands/flags | No | No new CLI commands |
| CLI grammar (action before identifier) | No | No new commands |
| Editor autocomplete | Yes (automatic) | YANG-driven; automatic when YANG updated and ArgDefs populated |
| Functional test for new RPC/API | Yes | `test/ui/completion-words-goroutine-modes.ci`, `test/ui/completion-words-audit-keywords.ci` |
| Doctor check for runtime dependencies | No | No new runtime dependencies |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | N/A (internal improvement; existing commands unchanged) |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A (existing commands gain completion hints, not new commands) |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | No | N/A |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/commands.md` - document ArgDefs field on Node, YANG leaf convention for command arguments, auto-generated completion from YANG types |

## Files to Create
- `internal/component/command/argvalidate.go` - ValidateArgString function for validating arg strings against ArgDef types
- `internal/component/command/argvalidate_test.go` - unit tests for arg string validation
- `test/ui/completion-words-goroutine-modes.ci` - functional test for enum completion
- `test/ui/completion-words-audit-keywords.ci` - functional test for keyword completion

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- define ArgDef types, add ArgDefs to Node, write failing wiring tests
   - Tests: `TestArgDefFromEnumYANG`, `TestCompleterArgDefsEnumSuggestions`, `completion-words-goroutine-modes.ci` (failing)
   - Files: `command/node.go`, `command/completer_test.go`, `config/yang/command_test.go`
   - Verify: ArgDef type compiles; tests fail because no YANG extraction or completer integration yet

2. **Phase: YANG extraction** -- mergeYANGEntry reads leaf children of ze:command nodes
   - Tests: `TestArgDefFromEnumYANG`, `TestArgDefFromUnionYANG`, `TestArgDefFromUintRangeYANG`, `TestArgDefFromPatternYANG`
   - Files: `config/yang/command.go`
   - Verify: tests fail -> implement extraction -> tests pass. ArgDefs populated on Nodes.

3. **Phase: Arg validation** -- ValidateArgString for each ArgKind
   - Tests: `TestValidateArgStringEnum`, `TestValidateArgStringUint`, `TestValidateArgStringUintRange`, `TestValidateArgStringUnion`
   - Files: `command/argvalidate.go`, `command/argvalidate_test.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Completer integration** -- matchChildren reads ArgDefs for suggestions
   - Tests: `TestCompleterArgDefsEnumSuggestions`, `TestCompleterArgDefsKeywordSuggestions`
   - Files: `command/completer.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Dispatcher integration** -- two-phase validation (keyword extraction + positional matching) before handler call
   - Tests: `TestDispatcherArgValidation`, `TestDispatcherKeywordExtraction`, `TestDispatcherPositionalMatching`, `TestDispatcherMixedArgs`, `TestDispatcherMandatoryMissing`, `TestDispatcherNoArgDefsPassthrough`
   - Files: `server/command.go`
   - Verify: tests fail -> implement -> tests pass

6. **Phase: YANG migration** -- add leaves to all 23 commands in YANG schemas
   - Tests: `TestArgDefsPopulated`
   - Files: `ze-cli-show-cmd.yang`, `ze-cli-set-cmd.yang`, `ze-cli-log-cmd.yang`
   - Verify: all 23 commands have ArgDefs after BuildCommandTree

7. **Phase: ValueHints cleanup** -- remove static manual wiring, keep runtime-dynamic
   - Tests: `completion-words-levels.ci` (existing), `completion-words-values.ci` (existing), `completion-words-fd-limit.ci` (existing)
   - Files: `command/valuehints.go`
   - Verify: wireFDSetHints and wireLogSetHints removed; wireRibHints stays; all existing completion tests still pass

8. **Phase: Functional tests** -- create and verify end-to-end tests
   - Tests: `completion-words-goroutine-modes.ci`, `completion-words-audit-keywords.ci`
   - Files: `test/ui/`
   - Verify: all functional tests pass

9. **Full verification** -- `make ze-verify` (lint + all ze tests except fuzz)

10. **Complete spec** -- fill audit tables, write learned summary to `plan/learned/NNN-cmd-typed-args.md`, delete spec from `plan/`. BLOCKING: summary is part of the commit, not a follow-up.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Enum extraction | Union types correctly extract enum members from nested YangType.Type members |
| Range validation | uint range checks handle both single values (range "0 \| 3 \| 7") and continuous ranges (range "1..10000") |
| Completer regression | Existing completion tests still pass: completion-words-values.ci (families), completion-words-levels.ci (log levels) |
| Dispatcher passthrough | Commands without ArgDefs skip validation entirely (no false rejections) |
| Handler compatibility | Handlers still receive args []string unchanged; no signature or arg ordering change |
| YANG parsing | goyang correctly parses leaf children inside config false containers (verify Config == TSUnset, entry.Type != nil) |
| Naming | YANG leaf names use kebab-case matching the keyword names handlers already parse |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| ArgDef type in node.go | `grep 'type ArgDef struct' internal/component/command/node.go` |
| ArgDefs field on Node | `grep 'ArgDefs' internal/component/command/node.go` |
| Leaf extraction in mergeYANGEntry | `grep 'ArgDef' internal/component/config/yang/command.go` |
| ValidateArgString function | `grep 'func ValidateArgString' internal/component/command/argvalidate.go` |
| Completer uses ArgDefs | `grep 'ArgDefs' internal/component/command/completer.go` |
| Dispatcher validates args | `grep 'ArgDefs\|ValidateArg' internal/component/plugin/server/command.go` |
| YANG leaves in show commands | `grep -c 'leaf ' internal/component/cmd/show/schema/ze-cli-show-cmd.yang` shows nonzero |
| YANG leaves in set commands | `grep -c 'leaf ' internal/component/cmd/set/schema/ze-cli-set-cmd.yang` shows nonzero |
| YANG leaves in log commands | `grep -c 'leaf ' internal/component/cmd/log/schema/ze-cli-log-cmd.yang` shows nonzero |
| wireFDSetHints removed | `grep wireFDSetHints internal/component/command/valuehints.go` returns no match |
| wireLogSetHints removed | `grep wireLogSetHints internal/component/command/valuehints.go` returns no match |
| wireRibHints still present | `grep wireRibHints internal/component/command/valuehints.go` returns match |
| Functional test goroutine modes | `ls test/ui/completion-words-goroutine-modes.ci` |
| Functional test audit keywords | `ls test/ui/completion-words-audit-keywords.ci` |
| ArgDefs populated for all commands | `go test -run TestArgDefsPopulated ./internal/component/config/yang/` passes |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | ArgDef validation must not accept overly long strings; add a max arg length check (e.g., 1024 bytes) before type validation |
| Pattern safety | YANG pattern regexes must be anchored; compile patterns at startup (BuildCommandTree time), not per-request |
| Resource exhaustion | EnumValues slices are bounded by YANG schema (no user input controls their size); Pattern compilation is O(1) at startup |
| Error leakage | Validation error messages must not leak internal type details; report "invalid argument" with the expected format |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior; if misunderstood return to RESEARCH |
| Lint failure | Fix inline; if architectural return to DESIGN phase |
| Functional test fails | Check AC; if AC wrong return to DESIGN; if AC correct return to IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| goyang does not parse leaves inside config false containers as expected | Verify with a minimal YANG module test; may need to add explicit config false to each leaf |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Typed Argument Catalog

| Command | Arg | Type | Style |
|---------|-----|------|-------|
| show audit | action, actor, surface, since, until | string | keyword |
| show audit | count | uint | keyword |
| show capture | protocol | enum: l2tp, bgp | positional |
| show capture | tunnel-id | uint16 | keyword |
| show capture | peer | string | keyword |
| show capture | count | uint | keyword |
| show capture raw | action | enum: start, stop, dump | positional |
| show capture raw | protocol | enum: l2tp, bgp | positional |
| show capture raw | format | enum: pcap, json | positional |
| show capture raw | count | uint | keyword |
| show capture interface | iface | string | positional |
| show capture interface | count | uint (1-10000) | keyword |
| show capture interface | duration | string (duration pattern) | keyword |
| show capture interface | snap-len | uint (64-65535) | keyword |
| show capture interface | format | enum: pcap, text | keyword |
| show capture interface | protocol | string | keyword |
| show dns lookup | hostname | string | positional |
| show dns lookup | type | enum: A, AAAA, MX, NS, TXT, CNAME, PTR | keyword |
| show dns cache | action | enum: stats, list, record | positional |
| show dns cache | name | string | positional |
| show interface | mode | enum: brief, type, errors, rate, detail, counters | positional |
| show ip arp | family | enum: ipv4, ipv6, any, all | keyword |
| show ip route | prefix | string (CIDR) | positional |
| show ip route | limit | uint | keyword |
| show neighbors | family | enum: ipv4, ipv6, any, all | positional |
| show system file-descriptors | mode | enum: summary, detail | positional |
| show system goroutines | mode | enum: summary, blocked, full | positional |
| show system kernel-log | level | enum (syslog level) | keyword |
| show system kernel-log | count | uint | keyword |
| show system profile | type | enum: cpu, heap, goroutine, allocs | positional |
| show system profile | duration | string (duration pattern) | keyword |
| show system sockets | protocol | enum: tcp, udp | positional |
| show system sockets | state | string | keyword |
| show system sockets | port | uint | keyword |
| show ping | dest | string | positional |
| show ping | count | uint | keyword |
| show ping | timeout | string (duration pattern) | keyword |
| show traceroute | dest | string | positional |
| show traceroute | max-hops | uint | keyword |
| show traceroute | timeout | string (duration pattern) | keyword |
| show traceroute | probes | uint | keyword |
| show probe-round | dest | string | positional |
| show probe-round | max-hops | uint | keyword |
| show probe-round | timeout | string (duration pattern) | keyword |
| show probe-round | probes | uint | keyword |
| show tcp-check | host | string | positional |
| show tcp-check | port | uint | positional |
| show tcp-check | source | string | keyword |
| show tcp-check | timeout | string (duration pattern) | keyword |
| show crashes | name | string (latest or filename) | positional |
| set system file-descriptors | limit | union: uint64, enum max | positional |
| log set | logger | string | positional |
| log set | level | enum (slog level) | positional |
| log recent | level | enum (slog level) | keyword |
| log recent | component | string | keyword |
| log recent | count | uint | keyword |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- mergeYANGEntry filter (child.Config != TSFalse) was deliberately written to walk only explicitly-marked config false containers. Leaves inherit config false via ReadOnly() but have Config == TSUnset. The fix is not to change the filter but to add a targeted second pass on leaf children of ze:command nodes only.
- Union types are the most interesting pattern: the completer extracts enum members for suggestions while the validator tries each union member in order. This naturally handles "uint64 or max" with no special-casing.
- Keyword-value args and positional args both benefit from the same YANG leaf approach. The dispatcher uses leaf names as natural keyword detectors: when an arg matches a leaf name, it is a keyword and the next arg is its value. No YANG annotation needed because leaf names (count, timeout, level) never collide with positional values (summary, detail, 192.168.1.1).
- Duration arguments (timeout, duration) cannot use a native YANG duration type (none exists). They are modeled as string with a pattern constraint like `[0-9]+[smh]?`.

## Core Insight

YANG leaves inside ze:command containers serve as a declarative argument schema. The same leaf metadata drives three independent consumers: the completer (enum values become suggestions), the validator (types become constraints), and the documentation (leaf descriptions become help text). All three stay in sync because they read from the same source.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| No new YANG extension for positional vs keyword | ze:syntax "positional", ze:arg-style extension | Leaf names naturally distinguish keywords from positional values. The dispatcher matches arg tokens against leaf names to detect keyword-value pairs. No annotation needed. |
| Second pass on leaf children, not changing the main filter | Using child.ReadOnly() instead of child.Config != TSFalse | ReadOnly() would pick up config leaves too. Targeted second pass only reads leaves of ze:command nodes. |
| ArgDefs on Node, not extending ValueHints | Auto-populating ValueHints from YANG | ArgDefs carry type metadata for validation, not just completion. ValueHints is completion-only. |
| Two-phase validation in dispatcher | Single-pass validation, validate-on-complete | Phase 1 (keyword extraction by leaf name match) + Phase 2 (positional enum matching) catches both keyword-value type errors and invalid positional values. Clean insertion point between tokenize and handler call. |
| Keep handlers receiving args []string unchanged | Parsed typed args passed to handler | Handlers already parse args. Changing the signature would require modifying all 23 handlers. The dispatcher validates; the handler still receives raw strings. |

## Dispatcher Validation Algorithm

Two-phase validation runs after tokenize(remaining) and before handler invocation:

**Phase 1 -- Keyword extraction:** Scan args left-to-right. When args[i] matches an ArgDef leaf name and args[i+1] exists, treat as a keyword-value pair: validate args[i+1] against that leaf's type, mark both as consumed. If args[i] matches a leaf name but no value follows, reject with "keyword requires a value" error. Each leaf name can only be consumed once.

**Phase 2 -- Positional matching:** Remaining unconsumed args are positional values. For each, try matching against all ArgDefs that have enum values. If the positional value matches an enum value in any ArgDef, it is valid. String-typed ArgDefs always match (accept any value). If no ArgDef matches, the arg is passed through to the handler (unknown args are not rejected, to preserve backward compatibility with handlers that accept unmodeled args).

**Phase 3 -- Mandatory check:** If any ArgDef has mandatory=true and was not matched by either a keyword or positional arg, reject with a "required argument missing" error.

**Why leaf names naturally distinguish keyword from positional:** Leaf names are semantic identifiers (count, timeout, level, mode, format) that never collide with positional values (summary, detail, 192.168.1.1, max, tcp). The algorithm detects keywords by name match, not by YANG annotation.

## Known Limitations
- Duration arguments are modeled as patterned strings, not as a semantic duration type. YANG has no native duration type. Handlers must still parse the duration string.
- String-typed args (hostname, interface name, peer address) get no validation beyond optional pattern constraints. These remain handler-validated.
- Unknown args (not matching any leaf name or enum value) are passed through rather than rejected, to preserve backward compatibility. Handlers may accept args not modeled in YANG.

## RFC Documentation

N/A -- no protocol code.

## Implementation Summary

### What Was Implemented
- [To be filled during implementation]

### Bugs Found/Fixed
- [To be filled during implementation]

### Documentation Updates
- [To be filled during implementation]

### Deviations from Plan
- [To be filled during implementation]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| YANG leaves declare argument types for all 23 commands | Functional test | `TestArgDefsPopulated` passes |
| Completer auto-generates enum suggestions from YANG | Functional test | `completion-words-goroutine-modes.ci` passes |
| Dispatcher validates args before handler call | Unit test | `TestDispatcherArgValidation` passes |
| Manual ValueHints replaced by YANG-driven suggestions | Grep | wireFDSetHints and wireLogSetHints absent from valuehints.go |
| Runtime-dynamic ValueHints still work | Functional test | `completion-words-values.ci` still passes |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-cmd-typed-args.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
