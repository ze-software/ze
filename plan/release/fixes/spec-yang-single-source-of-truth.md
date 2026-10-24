# Spec: yang-single-source-of-truth

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | Shipped config and command surfaces accept input Ze then ignores, refuses, or reads differently from what its YANG declares; an operator cannot trust `ze config validate`, `show`, or the generated help. These are defects in implemented capabilities, so they belong before the first release |
| Depends | Coordinates with `plan/release/fixes/spec-config-yang-when-unique-choice.md` (choice and `when` semantics) and `plan/release/fixes/spec-config-yang-rpc-mandatory-defaults.md` (rpc input mandatory and defaults). Those specs own the RFC 7950 rule; this one owns the derivation that makes every consumer apply it |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Root cause.** Ze's YANG modules declare the config, command and notification surface, but nothing makes a consumer derive from them. Each consumer restates the fact it needs by hand: the Go value type of a leaf, the set of sections the validator walks, which leaf a plugin reads, where a default is applied, which positional token a handler takes, which enum values a backend handles. Nothing compares the restatement with the declaration, so the two drift, and nothing refuses a declared node that no code reads, handles or produces.

**Symptoms (51 open journal rows, nine classes).** A leaf validates, commits and shows, and changes nothing (`auto-commit-delay`, `max-batch-size`, `auto-flush`, thirty-plus IPsec, OSPF, iface, fib and l2tp leaves). A value the leaf's own range accepts is refused (every `uint64` above 2^32, every ranged `decimal64`). A section's `ze:validate` never runs because the section is absent from a hand list. A declared rpc has no handler and a declared notification has no producer. A handler reads `args[0..2]` the schema never declares, so the web form, MCP and the CLI each send a different shape. A YANG enum omits a value the backend handles, or offers one it does not.

**Why it matters for shipping Ze.** The operator's contract is the YANG model: it drives the CLI, completion, web forms, MCP tools, `ze config validate` and the docs. When the model and the code disagree, every one of those surfaces lies. A NOS cannot ship with config that silently does nothing.

**Goal.** One fix that removes the family at its source: every consumer of a schema node registers against that node and derives its type, default, vocabulary and argument shape from it, and a check refuses a node with no registered consumer and a consumer that names no node.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/config-yang.md`, `ai/doors/commands-cli.md` - design boundaries for config, YANG and command dispatch
- [ ] `docs/architecture/config/yang-config-design.md` - how the schema is built from YANG and consumed
  → Constraint: YANG is required for every RPC; there is no command-module category outside YANG
- [ ] `docs/architecture/api/commands.md`, `docs/architecture/cli/command-verbs.md` - how command YANG becomes `command.ArgDef` and grammar
- [ ] `docs/architecture/config/environment.md` - env leaves and legacy key aliases
- [ ] `ai/rules/principles.md` - "Every fact MUST be declared once"; registration over central enumeration
- [ ] `ai/rules/config.md` - YANG leaf, env alias and listener rules

**Key insights:**
- The schema already exists as one typed tree (`config.YANGSchema()`); the defect is that consumers do not go through it.
- Existing gates (`internal/le/doc/yangcontract`, the enumeration agreement gate, `CheckAllValidatorsRegistered`) each check one slice by hand list or text match, with population holes recorded in the same journal.

## Current Behavior (MANDATORY)

**Source files read (producers verified 2026-10-23):**
- [ ] `internal/component/config/yang_schema.go` `yangTypeToValueType` (line 1209): `Yuint32` and `Yuint64` both map to `TypeUint32`; `Ydecimal64`, `Yunion`, `Yleafref`, `Ybits` and `Yidentityref` all map to `TypeString`. A `uint64` leaf is parsed with a 32-bit limit and a ranged `decimal64` reaches the range check's `default:` arm and is refused. Still open.
- [ ] `internal/component/config/validate_sections.go` `validatedSections` (line 100): a hand-written slice of 21 section names, each widening justified by a paragraph of prose above it. `ValidateCustomSections` walks only these; a `ze:validate` under any other section loads, passes `CheckAllValidatorsRegistered`, and never runs. Still open. The 2026-08-11 row's `yangSectionsToValidate` in `cmd_validate.go` is gone: `runValidation` now calls the shared `config.ValidateCustomSections`, so validate and start share one walk, but the walk's population is still the hand list.
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` declares `auto-commit-delay` and `max-batch-size`; no `.go` file under `internal/`, `pkg/` or `cmd/` names either leaf or its Go spelling. Still open.
- [ ] `internal/component/bgp/yang/ze-bgp-api.yang` lines 956 to 1068 declare seven notifications; `peer-state-change` has no Go producer (grep finds none outside tests), and three carry a description admitting "no notification today". Still open.
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` `injectRoute` (lines 270 to 289): reads `args[0]` as a peer IP, `args[1]` as a family, `args[2]` as a prefix and `args[3:]` as attributes, none declared in the command YANG. Still open.
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` lines 460, 502, 675, plus `admin.go`, `create.go`, `raw.go`, `announce.go`, `update_text.go`, `commit.go`, `monitor.go`, `cache.go`: handlers read `validated.Tokens()` by position. Still open.
- [ ] `internal/component/config/listener_defaults.go` `RegisterBuiltinListenerDefaults`: no non-test caller today (the 2026-08-23 row named `ze doctor` as its one caller; that call is gone too), so the listener conflict check sees only endpoints an operator spelled out. Still open, and wider than the row says.
- [ ] `internal/component/hub/yang/ze-hub-conf.yang` lines 66 to 69: the `backend` enum declares `stderr`, `stdout`, `syslog`; `slogutil` `backendNames` also accepts `kmsg`. Still open.
- [ ] `internal/component/config/yang/command.go` `argDefFor`: builds `command.ArgDef` from the leaf type alone; the leaf `description` is dropped.

**Behavior to preserve:**
- Every config that loads today and is valid under its YANG still loads, with the same effect.
- The command text operators type today keeps working where the YANG already declares it.

**Behavior to change:**
- A declared node with no consumer, a consumer naming no node, and a consumer whose vocabulary, type or default disagrees with the node are refused at build or load, with a message naming the node and the fix.
- Value types, validation population, defaults and handler arguments come from the schema.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config file, SSH editor `set`, web form, MCP tool, CLI command text, plugin YANG emitted with `--yang`.

### Transformation Path
1. YANG modules load into one schema (`config.YANGSchema()`), including external plugin YANG.
2. Each consumer registers the schema paths it reads (config leaf), answers (rpc), emits (notification) or parses (enum vocabulary), and receives a typed accessor bound to the node.
3. The parser and validator take type, range, length, pattern, mandatory and default from the node; the validation walk covers every section the schema derives.
4. The command dispatcher hands a handler named, typed arguments decoded from the declared rpc input; the handler never sees raw positional tokens.
5. At load, the registry is compared with the schema; an unconsumed node or an orphan consumer refuses startup and `ze config validate` alike.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine and plugin | Plugin declares read paths, rpc handlers and notifications at registration; external plugins send the same declaration over the plugin protocol | No |
| Config to consumer | Typed accessor bound to a schema node | No |
| Command text to handler | Decoded rpc input, not token slice | No |

### Integration Points
- `config.YANGSchema()`, `yangTypeToValueType`, `ValidateCustomSections`, `config.ApplyDefaults`, `internal/component/config/yang/command.go` (`argDefFor`), `internal/le/doc/yangcontract`, plugin registration in `internal/component/plugin/registry`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | Design routes every read through the schema-bound accessor |
| No unintended coupling | No | Plugins declare against schema paths, not against each other |
| No duplicated functionality | No | Hand lists (`validatedSections`, positional readers, private enum tables) are deleted, not wrapped |
| Zero-copy preserved where applicable | No | Accessor returns the stored value; no new copy on hot paths (config is not a hot path) |
| Registration over hardcoding, outbound | No | Consumers register; the core discovers them |
| Registration over hardcoding, inbound | No | `validatedSections`, `kernelTunnelKinds`-style tables and per-handler token readers are replaced by derivation from the schema |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every shipped config leaf that is meant to do something has one identifiable consumer package | journal rows name the package for each inert leaf | Some leaves are read through generic map walks; those walks must register the paths they read | first run of the reachability check lists every unconsumed node | unvalidated |
| A-2 | Widening the validation walk to every section refuses no shipped config | the `service`, `system`, `rib` widenings measured zero new refusals | A shipped `.ci` config goes red | run parse, install and ui suites after widening | unvalidated |
| A-3 | Named rpc input can carry every operator form handlers accept today | `request peer <sel> teardown` already has a keyword form in MCP and web | A handler accepts a form no YANG shape expresses | per-handler YANG review in implementation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The reachability check finds hundreds of nodes at first run | count printed by the check | Each node is wired or deleted (no-layering); no allowlist, no `ze:unread` escape |
| R-2 | Changing a positional command to named input breaks existing operator text | `.ci` tests that type the old form | Keep the declared form where the YANG already states it; add the keyword where it did not exist |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Config rejected at load, or a command form refused. No wire impact |
| How is it reverted? | Single commit revert per step |
| Who else touches this path? | Specs `spec-config-yang-when-unique-choice`, `spec-config-yang-rpc-mandatory-defaults`, `spec-config-yang-list-key-leaf-list`, `spec-config-yang-loader-structural-checks` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` on a config setting a `uint64` leaf above 2^32 | → | `yangTypeToValueType`, `ValidateValue` | `test/parse/yang-uint64-full-range.ci` |
| `ze start` with a test YANG module declaring a leaf no plugin reads | → | schema consumer reachability check at load | `test/parse/yang-unconsumed-leaf-refused.ci` |
| `ze config validate` with a `ze:validate` failure under a section absent from today's hand list | → | `ValidateCustomSections` over every derived section | `TestValidateCustomSectionsCoversEverySchemaSection` |
| `request bgp rib inject` from the web admin form and from CLI text | → | dispatcher decoding rpc input to named arguments | `test/plugin/rib-inject-named-args.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A YANG `uint64` leaf set to 18446744073709551615 within its range; a ranged `decimal64` leaf set to `5.00` | Both accepted; a value outside the declared range refused with the range in the message |
| AC-2 | A `leaf-list` or multi-value leaf carrying a `range` or `length`, set outside it (`copp protected-port 0`) | Refused, same as a single leaf |
| AC-3 | A `ze:validate` under any top-level schema section, failing | `ze config validate` and `ze start` both refuse; `validatedSections` no longer exists |
| AC-4 | A mandatory leaf inside an absent non-presence container, and inside a `peer` or `group` list entry | Refused by validate and start alike; a satisfied mandatory choice raises no warning |
| AC-5 | A leaf with a YANG `default` the operator omits | The plugin receives the default without calling `ApplyDefaults` itself; listener defaults are applied before the conflict check, so two endpoints colliding on a default port are refused by `ze config validate` |
| AC-6 | A YANG config leaf, rpc or notification with no registered consumer, handler or producer (reproduced with a test module, and with `auto-commit-delay` before it is wired or deleted) | Load refuses, naming the node path and the fix; the check runs in the build, so the tree cannot carry one |
| AC-7 | A consumer registering a path absent from the schema | Load refuses, naming the path |
| AC-8 | A YANG enum and the Go vocabulary its consumer parses differ in either direction (`log backend kmsg`, ADD-PATH `receive/send`, `diag protocol bfd`, softver `mode`) | The check refuses the disagreement whatever the enum size; the shipped enums match their consumers |
| AC-9 | A command argument declared on both the `-cmd` container and the rpc input, or declared on neither and read by the handler | Refused at schema build; every handler receives named typed arguments, and `request bgp rib inject`, `peer teardown`, `peer disable message`, `announce`, `peer raw` work identically from CLI text, web form and MCP |
| AC-10 | A command argument leaf with a `description` | The description appears in CLI help, web form and MCP tool schema |
| AC-11 | External plugin emitting YANG with `--yang` | Its nodes join the daemon schema and are subject to AC-6 to AC-8 |
| AC-12 | Each owned row's representative reproduction (Owned journal rows below) | No longer reproduces; the row is re-pointed to this spec and closed with its test name |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestYangTypeToValueTypeCoversEveryBuiltin` | `internal/component/config/yang_schema_test.go` | AC-1: every goyang kind maps to a type holding its full value space |
| `TestLeafListCarriesRangeAndLength` | `internal/component/config/yang_schema_test.go` | AC-2 |
| `TestValidateCustomSectionsCoversEverySchemaSection` | `internal/component/config/validate_sections_test.go` | AC-3 |
| `TestMandatoryWalkDescendsListsAndAbsentContainers` | `internal/component/config/yang/validator_test.go` | AC-4 |
| `TestSchemaConsumerReachability` | `internal/component/config/consumer_test.go` | AC-6, AC-7 with a synthetic module |
| `TestEnumVocabularyMatchesConsumer` | `internal/component/config/consumer_test.go` | AC-8 |
| `TestArgDeclaredOnceAndDescriptionCarried` | `internal/component/config/yang/command_test.go` | AC-9, AC-10 |

### Boundary Tests (numeric inputs)
| Field | Range | Last valid | First invalid |
|-------|-------|------------|---------------|
| `uint64` leaf | full | 18446744073709551615 | 18446744073709551616 |
| `uint32` leaf | full | 4294967295 | 4294967296 |
| `decimal64` fraction-digits 2, range 1.00..100.00 | declared | 100.00 | 100.01 |

### Functional Tests
| Test | Validates |
|------|-----------|
| `test/parse/yang-uint64-full-range.ci` | AC-1 through the real parser |
| `test/parse/yang-unconsumed-leaf-refused.ci` | AC-6 at load |
| `test/parse/listener-default-conflict-refused.ci` | AC-5 |
| `test/plugin/rib-inject-named-args.ci` | AC-9 |
| `test/plugin/external-plugin-yang-joins-schema.ci` | AC-11 |

## Files to Modify
- `internal/component/config/yang_schema.go` - type derivation, leaf-list constraints
- `internal/component/config/validate_sections.go` - delete `validatedSections`, derive sections
- `internal/component/config/yang/validator.go` - mandatory walk population
- `internal/component/config/listener_defaults.go`, `internal/component/config/cli/cmd_validate.go` - defaults before the conflict check
- `internal/component/config/yang/command.go` - argument declaration and description
- command handlers under `internal/component/bgp/plugins/cmd/` and `internal/component/bgp/plugins/rib/rib_commands.go` - named arguments
- every consumer of an owned inert leaf - wire or delete the leaf
- `internal/le/doc/yangcontract` - fold into the load-time check or delete (no-layering)

## Files to Create
- `internal/component/config/consumer.go` - consumer registry and reachability check

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/architecture/config/yang-config-design.md` - consumer registration and derivation
- [ ] `docs/architecture/api/commands.md` - named arguments
- [ ] `docs/guide/configuration.md` - deleted or newly wired leaves
- [ ] `docs/architecture/config/syntax.md` - presence container value mode removed, set-path parsing
- [ ] `docs/architecture/doctor-and-health-checks.md` - listener defaults applied before the conflict check

## Implementation Steps
1. Type derivation and leaf-list constraints (AC-1, AC-2).
2. Validation population: delete `validatedSections`, walk every derived section, fix the mandatory walk (AC-3, AC-4).
3. Defaults applied once at delivery; listener defaults from the same source (AC-5).
4. Consumer registry and reachability check; run it, then wire or delete every node it reports (AC-6, AC-7, AC-11).
5. Enum agreement through the registry (AC-8).
6. Command arguments from rpc input; handlers move off `Tokens()` (AC-9, AC-10).
7. Re-run each owned row's reproduction (AC-12).

## Owned journal rows

The 52 open rows this root cause owns. Each is re-pointed to this spec when it lands.

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `unwired-feature.md` | 2026-10-23 | `label`/`labels` in a config `attribute` block never reach the route |
| `unwired-feature.md` | 2026-10-10 | `parseContainer` value mode: `graceful-restart disable` boots, later commit refused |
| `unwired-feature.md` | 2026-09-04 | `request peer <sel> borr` declared in `ze-refresh-cmd.yang`, dispatcher answers unknown command |
| `unwired-feature.md` | 2026-09-04 | fifteen leaves incl. `l2tp/shaper/upload-rate`, `bmp/route-action` with no reader |
| `unwired-feature.md` | 2026-09-04 | `ddos/flowtriq/api-key`, `bfd/enabled` and four more leaves with no reader |
| `unwired-feature.md` | 2026-09-04 | `fib/p4/*`, `fib/kernel/flush-on-stop`, `sweep-delay` inert through a type mismatch |
| `unwired-feature.md` | 2026-09-04 | `session/host-name`, `session/domain-name` legacy FQDN leaves inert |
| `unwired-feature.md` | 2026-09-04 | `software-version/mode` enum offers four values, implements two |
| `unwired-feature.md` | 2026-09-04 | IPsec `remote-address` accepts a hostname its consumers cannot use |
| `unwired-feature.md` | 2026-09-04 | IPsec `remote-access` subtree and seventeen more leaves inert |
| `unwired-feature.md` | 2026-09-03 | `ospf/reference-bandwidth`, `max-metric` and two more inert |
| `unwired-feature.md` | 2026-09-03 | `unit/ipv6/dhcpv6/pd` and three more iface leaves inert |
| `unwired-feature.md` | 2026-09-03 | five `ze-bgp-api` RPCs (`peer-show`, `peer-add`, ...) with no handler |
| `unwired-feature.md` | 2026-09-02 | `bgp policy loop-detection` names a filter type no plugin declares |
| `unwired-feature.md` | 2026-08-13 | external plugin YANG never reaches the daemon schema |
| `unwired-feature.md` | 2026-08-29 | `auto-commit-delay`, `max-batch-size` have no reader |
| `unwired-feature.md` | 2026-08-30 | `environment/bgp/reactor` leaves lack the YANG-named env alias |
| `unwired-feature.md` | 2026-09-09 | four `ze-bgp-api` notifications have no producer |
| `unwired-feature.md` | 2026-09-20 | bare word after any presence container stored as its value |
| `declared-format-contradicts-payload.md` | 2026-10-13 | `yangTypeToValueType`: `uint64` validated as `uint32` |
| `declared-format-contradicts-payload.md` | 2026-10-13 | ranged `decimal64` mapped to string, every value refused |
| `declared-format-contradicts-payload.md` | 2026-09-30 | `bgp/router-id`, `session/asn/local` declared mandatory against the design |
| `declared-format-contradicts-payload.md` | 2026-09-05 | `rpki/cache-server/preference` declares an order Ze does not perform |
| `declared-format-contradicts-payload.md` | 2026-09-03 | `update/watchdog/withdraw` and sibling declare a meaning the producer lacks |
| `declared-format-contradicts-payload.md` | 2026-08-31 | `ze-plugin:session-ping` reply leaf contradicts its handler |
| `schema-omits-a-leaf-the-backend-supports.md` | 2026-09-14 | `environment/log/backend` enum omits `kmsg` |
| `schema-omits-a-leaf-the-backend-supports.md` | 2026-09-11 | ADD-PATH `direction` enum omits `receive/send` |
| `schema-omits-a-leaf-the-backend-supports.md` | 2026-09-05 | vxlan case has no `ttl` leaf |
| `schema-omits-a-leaf-the-backend-supports.md` | 2026-09-14 | `diag` capture `protocol` enum omits `bfd` |
| `validated-value-discarded-by-its-caller.md` | 2026-09-03 | `argDefFor` drops a command argument's `description` |
| `declared-default-applied-by-each-consumer.md` | 2026-09-04 | YANG `default` applied only where a consumer calls `ApplyDefaults` |
| `command-takes-an-untyped-positional-value.md` | 2026-08-24 | five `show bgp` commands read an untyped positional |
| `command-takes-an-untyped-positional-value.md` | 2026-08-30 | `announce` handler reads args beyond the declared `selector` |
| `command-takes-an-untyped-positional-value.md` | 2026-09-04 | `peer raw`, `peer update` and siblings read undeclared positionals |
| `command-takes-an-untyped-positional-value.md` | 2026-09-04 | `request bgp rib inject` reads `args[0..2]` |
| `command-takes-an-untyped-positional-value.md` | 2026-09-15 | thirteen commands declare args on rpc input only; web form renders none |
| `command-takes-an-untyped-positional-value.md` | 2026-10-21 | `peer teardown`/`disable` read `validated.Tokens()` by position |
| `argument-leaf-declared-on-container-and-rpc-input.md` | 2026-09-15 | 25 commands declare one leaf on the `-cmd` container and the rpc input |
| `gate-excludes-part-of-its-population.md` | 2026-10-08 | `contractSatisfied`: 14 local handlers with no YANG node pass |
| `gate-excludes-part-of-its-population.md` | 2026-10-08 | yangcontract reads `-cmd` only: 117 rpc methods with no handler |
| `gate-excludes-part-of-its-population.md` | 2026-09-04 | `walkTree`: mandatory leaf inside an absent container never checked |
| `gate-excludes-part-of-its-population.md` | 2026-09-02 | `validatedSections` hand list: `ze:validate` outside it never runs |
| `gate-excludes-part-of-its-population.md` | 2026-08-23 | listener conflict check never sees listener defaults |
| `gate-excludes-part-of-its-population.md` | 2026-08-11 | satisfied mandatory `choice kind` reported missing per tunnel |
| `gate-excludes-part-of-its-population.md` | 2026-08-11 | section hand list beside the schema-derived sections (now `validatedSections`) |
| `gate-excludes-part-of-its-population.md` | 2026-08-11 | three validators answer from a registry not holding the asked value |
| `gate-excludes-part-of-its-population.md` | 2026-10-12 | `bgpMandatoryFailures` skips `peer`, `group` and choice cases |
| `helper-bypassed-by-an-open-coded-copy.md` | 2026-10-13 | completer `validateStringPatterns` guesses IP from pattern text |
| `parser-refuses-a-form-its-producer-emits.md` | 2026-10-21 | goyang `ToEntry` refuses an augment of an action's absent `input` |
| `unwired-feature.md` | 2026-09-06 | peer leaf `behavior > manual-eor` read by no reactor code |
| `zero-value-as-valid-answer.md` | 2026-10-10 | `goyang-enum-negative.patch` has no owner to drop it |
| `zero-value-as-valid-answer.md` | 2026-10-10 | `TestGoyangNumbersAnEnumAfterANegativeValue` follow-up |
| `hardcoded-count-in-test.md` | 2026-10-23 | `request bgp watchdog *` moved to YANG (`dcc0ab9dc5`): wire-method snapshot and site catalog not refreshed |

## Rows found already fixed

Another agent closes these in the journal; listed here so they are not re-owned.

| Class file | Date | Surface (short) | Evidence (2026-10-23) |
|------------|------|-----------------|-----------------------|
| `unwired-feature.md` | 2026-09-06 | peer `behavior > manual-eor` has no reader | `parsePeerSettings` reads it into `PeerSettings.ManualEOR` (`internal/component/bgp/reactor/config.go` lines 304 to 311) |
| `gate-excludes-part-of-its-population.md` | 2026-08-11 | `kernelTunnelKinds` omits vxlan | `kernelLinkVxlan` is in the map (`internal/component/iface/discover.go` line 71), pinned by `TestKernelTunnelKindsCoversEveryModeledKind` |
| `gate-excludes-part-of-its-population.md` | 2026-08-12 | hub `parsePluginBlock` parses a plugin-only config differently from validate | `internal/component/hub/config.go` was deleted in 8d92e9fab; `parsePluginBlock` no longer exists and `config.ProbeConfigType` has no non-test caller |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Producers read for at least three owned rows (`yangTypeToValueType`, `validatedSections`, `injectRoute`, `ze-bgp-conf.yang` batching leaves, `ze-bgp-api.yang` notifications, `RegisterBuiltinListenerDefaults`, hub log `backend` enum)
- [ ] Overlapping specs named in Depends

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated; AC-6 refuses a reproduced unconsumed node
- [ ] Every owned row's reproduction no longer reproduces
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted tests pass: `./le test unit` over `internal/component/config/...` and the command packages changed, `./le test functional parse`, `./le test functional plugin`, and `./le verify current fast`; the full gate runs once after commit A
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests: N-A, config and command surface only, no wire change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + journal rows re-pointed
- [ ] **Commit B:** `remove` of this spec only
