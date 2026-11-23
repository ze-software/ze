# Spec: yang-single-source-of-truth

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | Shipped config and command surfaces accept input Ze then ignores, or read it differently from what the YANG declares: a leaf commits and changes nothing, a declared rpc or notification has no handler or producer, a handler reads tokens the model never declares, a Go vocabulary and its YANG enum disagree. These are defects in implemented capabilities, so they belong before the first release |
| RFC gaps | - |
| Depends | Split on 2026-11-14 (owner, "Split. Pieces A, B, C become release fixes now ... D, E and F become one design spec, with the startup refusal replaced by a build gate plus `ze doctor`"). Siblings: A `plan/release/fixes/spec-review-typed-config-decode.md`, B `plan/release/fixes/spec-validation-walks-every-schema-section.md`, C `plan/release/fixes/spec-yang-defaults-applied-once.md`. Argument modelling of commands with no leaf: `plan/release/fixes/spec-declared-commands-without-leaves.md`. External plugin YANG: `spec-plugin-external-yang-at-startup`. Peer leaves the peer parser never reads: `plan/release/fixes/spec-peer-leaves-the-peer-parser-never-reads.md` |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

This spec now holds three pieces of the original root cause. Pieces A, B and C
moved to their own specs (Depends). Each piece below needs Thomas's design pass
before `ready`; the decisions that are his are in "Decisions that are Thomas's".

| Piece | Problem | Goal |
|-------|---------|------|
| D, command arguments | Handlers pick arguments out of `ValidatedArgs.Tokens()` by index, so the argument is not a declared leaf: usage, the web form and MCP render nothing to type, and the validator judges nothing. One leaf is also declared twice, on the `-cmd` container and on the rpc input | Every owned handler reads its arguments by leaf name through the `ValidatedArgs` accessors that already exist; each argument is declared once |
| E, inert surface | A config leaf validates, commits and shows, and nothing reads it; a declared rpc has no handler; a declared notification has no producer | A build-time gate refuses a declared node with no consumer, extending the root claim gate to leaf granularity; `ze doctor` reports at run time what the build cannot see; each owned node is wired or deleted |
| F, enum vocabulary | A YANG enum omits a value its Go consumer handles, or offers one it does not | The enumeration agreement gate judges every Go vocabulary that parses a YANG enum leaf, in both directions, whatever the enum's size |

**What changed from the 2026-10-23 design.** The load-time startup refusal of an
unconsumed node is removed: plugins load on demand, compile out by tag, or run
external and declare after config load, so load cannot decide it. The design
page already assigns the check to a build-time gate plus `ze doctor`
(`docs/architecture/config/yang-config-design.md`, "Claim Completeness Gate").
The per-leaf typed accessor is no longer assumed: it would change the plugin
SDK's Stage 2 contract (every value a JSON string, `pkg/` public API), so it is
an open decision (T-3). The value-type, validation-population and defaults
work moved out: `yang_schema.go` `yangTypeToValueType`, the
`validate_sections.go` walk, `config/yang/validator.go` and the
`config/yang/command.go` `argDefFor` description (already fixed) are no longer
this spec's.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md`, "Claim Completeness Gate"
  → Decision: delivery is judged at build time by `TestConfigSchemaRootsClaimed` and `TestConfigRootsPhantomClaims`, and at run time by `ze doctor` (`doctor-config-root-unclaimed`) for a plugin compiled out or that failed to load
  → Constraint: `./le config unread-leaves report` is a heuristic and exits 0; promoting it to a gate needs a signal that is not a string-literal match
- [ ] `docs/architecture/api/commands.md`, "Every route validates" and "YANG-Typed Command Arguments"
  → Constraint: a handler reads `Tokens()`, `Positional(leaf)` or `Value(leaf)`; `Keyed`, `Flag` and `PositionalTokens` also exist; a handler that passes tokens on (the `update` encoding parser) passes a slice to a helper that is not a handler type
- [ ] `ai/rules/cli.md`, "CLI Grammar: Keywords Before Values"
  → Constraint: peer commands address a peer positionally, the one exception; every other free-form value follows a keyword
- [ ] `ai/META-DECISIONS.md`, "Code and dependencies"
  → Constraint: a command path names the subject and stops; scope, presentation and format are pipe operations, never arguments
- [ ] `ai/doors/system.md`, "`pkg/` is the public API"
  → Constraint: a change to the Stage 2 config body or to `sdk.Registration` is a published contract change, made on purpose with its page

**Key insights:**
- Every gate this spec needs exists at a coarser grain: root claims, the yangcontract orphan sets, the enumeration corpus of YANG enums, the per-package enum agreement tests. The work is to extend them, not to add a consumer registry beside them.

## Current Behavior (MANDATORY)

**Source files read (producers verified 2026-11-14):**
- [ ] `internal/component/command/argbind.go` - `ValidatedArgs` offers `Tokens`, `Positional`, `Keyed`, `Flag`, `PositionalTokens` and `Value`
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` `injectRoute` - reads `args[0]` as the peer, `args[1]` as the family, `args[2]` as the prefix
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` `handleBgpPeerList` and three more handlers call `validated.Tokens()`; across `internal/` 117 non-test files call `.Tokens()`, at least 20 of them under `internal/component/bgp/` (a population census, not each a defect)
- [ ] `internal/component/config/claims/claims.go` `Audit` and `internal/component/plugin/all/config_claims_test.go` `TestConfigSchemaRootsClaimed` - judge delivery per config root against `ConfigRoots`, `ConfigReads`, hub handler paths and `allowlist.json`
- [ ] `internal/le/config/unreadleaves/yangleafmentions.go` - reports a leaf whose kebab name appears in no string literal of the owning package; always exits 0
- [ ] `internal/component/doctor/checks_config_claims.go` `checkConfigClaims` - reports a configured root this binary delivers to nobody
- [ ] `internal/le/doc/yangcontract/contract.go` `contractSatisfied` - false on orphan YANG nodes, orphan handlers, orphan local handlers, orphan rpcs and foreign prefixes; it holds no notification check
- [ ] `internal/component/bgp/yang/ze-bgp-api.yang` - seven notifications (`peer-state-change`, `route-received`, `route-update-sent`, `eor-received`, `graceful-restart-state`, `session-established`, `session-closed`), each description saying no producer emits it
- [ ] `internal/le/arch/enumeration/corpus.go` `yangEnumerations` - a closed corpus of YANG enum value sets: a Go literal counts as a copy only when it holds the whole set, so a Go table holding a value the enum lacks is not judged
- [ ] `internal/component/bgp/reactor/yang_enum_agreement_test.go` - per-package agreement tests between named Go tables and the loaded model (`TestAddPathDirectionsMatchTheYANGModel`)
- [ ] `internal/component/bgp/reactor/config_capabilities.go` `parseAddPathDirection` - accepts `send/receive` and `receive/send`; the `ze-bgp-conf.yang` `direction` enum declares `send`, `receive`, `send/receive`, so the `receive/send` arm is dead
- [ ] `internal/component/hub/yang/ze-hub-conf.yang` leaf `backend` - `stderr`, `stdout`, `syslog`; `internal/core/slogutil/slogutil.go` also handles `kmsg`

**Behavior to preserve:**
- Every config valid under its YANG loads with the same effect, except an owned inert leaf that is deleted, which is refused with a fix.
- The peer selector stays positional (`request peer <selector> teardown`).
- Command text operators type today keeps working where the YANG already declares it.

**Behavior to change:**
- A declared config leaf, rpc or notification with no consumer is refused at build time; at run time `ze doctor` names a configured node no running consumer reads.
- A Go vocabulary that disagrees with its YANG enum is refused at build time.
- Owned handlers read arguments by leaf name.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Config claims gate | `config/claims/claims.go` `Audit`; `plugin/all/config_claims_test.go` `TestConfigSchemaRootsClaimed`, `TestConfigRootsPhantomClaims`; `claims/allowlist.json` | Every config root reaches a consumer; every declared root names a schema node | Extends (E): the same audit at leaf granularity, over the declaration T-2 chooses |
| Unread-leaves report | `internal/le/config/unreadleaves` (`./le config unread-leaves report`) | Advisory string-literal heuristic | Extends or replaces (E), by T-2: it becomes the gate only if its signal stops being a text match |
| Doctor root claim check | `doctor/checks_config_claims.go` `checkConfigClaims` (`doctor-config-root-unclaimed`) | Run-time report of a configured root delivered to nobody | Extends (E): run-time report at leaf granularity, covering a plugin compiled out, failed or external |
| Plugin read declarations | `plugin/registry/registry.go` `Registration.ConfigRoots`, `ConfigReads`; `sdk.Registration` `WantsConfig` | Root-level declaration of what a plugin owns and reads | Extends (E), by T-2: the leaf-level declaration, if chosen, lives beside these |
| YANG command contract | `le/doc/yangcontract/contract.go` `contractSatisfied` (`./le doc yang-contract command-contract`) | Refuses orphan nodes, handlers, local handlers and rpcs | Extends (E): adds the notification producer side; the rpc side already exists |
| Argument binding | `command/argbind.go` `ValidateArgs`, `ValidatedArgs.Value`, `Keyed`, `Flag`, `PositionalTokens` | Judges the line against the model and answers each leaf by name | Extends (D): the owned handlers move onto it; no new accessor |
| Argument extraction | `config/yang/command.go` `extractArgDefs`, `argDefFor`, `inheritArgDefs` | Builds `ArgDefs` from declared leaves, with `ze:help` and `description` | Extends (D): refuses a leaf declared on both the `-cmd` container and the rpc input |
| Parameter schema | `buildParamMeta` (named by the 2026-09-15 `argument-leaf-declared-on-container-and-rpc-input.md` row) | Builds API parameters from the rpc input | Extends or replaces (D): one of the two producers derives from the other, chosen at design |
| Enumeration gate | `le/arch/enumeration/corpus.go` `yangEnumerations` (`./le arch enumeration check`) | Refuses a Go literal that copies a whole YANG enum value set | Extends (F): a table that maps a YANG enum leaf is judged for agreement in both directions |
| Enum agreement tests | `bgp/reactor/yang_enum_agreement_test.go` | Per-package tests of named Go tables against the model | Replaces (F): one generic agreement check over every declared mapping, so a new table needs no new test |
| Filter insertion paths | `config.CheckFilterInsertionPaths` | Refuses a declared insertion condition that names no compiled schema node | Does not apply: already judges its own declarations; a model for the phantom check (E) |
| Validator section coverage | `config/validate_sections.go` `ValidatorSectionCoverage`, `CheckAllValidatorsRegistered` | Validator reach and existence | Does not apply: piece B (`spec-validation-walks-every-schema-section`) |
| Listener port table gate | `./le config ports check` | Listener defaults against YANG refines | Does not apply: piece C (`spec-yang-defaults-applied-once`) |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Build: the compiled schema and the registries a test binary links.
- Run time: `ze doctor` over the running config.
- Commands: CLI text, web form, MCP tool, REST.

### Transformation Path
1. E: a gate loads the schema, collects every config leaf, rpc and notification, and the consumers the registries declare; it refuses a node with no consumer and a declaration naming no node.
2. E: `ze doctor` compares the configured nodes with what the running consumers declare.
3. F: for each Go vocabulary declared as mapping a YANG enum leaf, the gate compares both value sets.
4. D: the dispatcher judges the line (`ValidateArgs`); the handler reads each leaf by name.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine and plugin | Leaf-level read declaration, if T-2 chooses a Stage 1 field | No |
| Command text to handler | `ValidatedArgs` by leaf name | No |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `pkg/`, `cmd/`, `test/`; terms `ValidatedArgs`, `Tokens()`, `ConfigReads`, `DeclareRegistrationInput`, `ze:method`; positive control: `injectRoute` found reading `args[0]`. D changes no wire: the command text keeps its declared form. E changes the Stage 1 declaration only under T-2 option 1.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Stage 1 `declare-registration` (`rpc.DeclareRegistrationInput`) | writer | every plugin's `sdk.Registration` passed to `p.Run`, in-tree and third-party | `ai/doors/plugin-infra.md`: `sdk.Registration` is an alias of `rpc.DeclareRegistrationInput` | Changes only under T-2 option 1: a new optional field; in-tree plugins declare it; documented in `docs/plugin-development/protocol.md` |
| same | reader | `plugin/server` Stage 1 handling of the declaration | `ai/doors/plugin-infra.md`, "Who depends on it" | Reads the new field under T-2 option 1; otherwise preserved |
| Command line to handler | writer | CLI, web form (`command.WriteInvocation`), MCP tool call | `docs/architecture/api/commands.md`, "YANG-Typed Command Arguments" | Preserved: declared forms keep working; a newly declared leaf takes its keyword |
| same | reader | `ValidateArgs` then each owned handler | `argbind.go` | Handlers read by leaf name |

### Integration Points
- `internal/component/plugin/all` test binary (links every plugin), `ze doctor` registry, `./le arch enumeration check`, the dispatcher.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | Arguments through `ValidateArgs`; delivery judged by the claims audit |
| No unintended coupling (components stay isolated) | No | Consumers declare; gates read registries |
| No duplicated functionality (extends existing, does not recreate) | No | Every gate is an extension (Existing Mechanisms) |
| Zero-copy preserved where applicable (refs, not copies) | No | No hot path |
| Registration over hardcoding, outbound | No | A consumer declares what it reads and maps |
| Registration over hardcoding, inbound | No | Searched: per-handler index reads, per-package enum agreement tests, `allowlist.json`; each is derived or removed |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every shipped leaf meant to do something has one identifiable consumer package | the owned rows name a package for each inert leaf | Some leaves are read through generic map walks, which must declare what they read | first gate run lists every unconsumed node | unvalidated |
| A-2 | Every operator form an owned handler accepts today can be declared as leaves | `ze:rest-of-line`, `ze:modifier` and anchored leaves exist (`commands.md`) | A form no YANG shape expresses | per-handler YANG review | unvalidated |
| A-3 | Each Go enum vocabulary can name the leaf it maps | the agreement tests already name them per package | A vocabulary shared by several leaves | the generic check lists each mapping | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The leaf gate reports hundreds of nodes at its first run | the count it prints | Each node is wired or deleted under T-4; an exception list exists only if T-5 allows it |
| R-2 | Moving a handler off positions breaks operator text | `.ci` tests typing the old form | Keep the declared form; the keyword is added where none existed |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A build gate refuses the tree; a config setting a deleted leaf is refused; a command form is refused |
| How is it reverted? | Single commit revert per piece |
| Who else touches this path? | `spec-declared-commands-without-leaves`, `spec-peer-leaves-the-peer-parser-never-reads`, `spec-plugin-external-yang-at-startup`, `spec-gates-share-one-semantic-population`, `spec-config-yang-when-unique-choice` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A test module declaring a config leaf no consumer reads, in the composition-root test binary | → | the claims audit at leaf granularity | `TestConfigSchemaLeavesConsumed` |
| `ze doctor` on a config setting a leaf whose plugin is compiled out | → | the doctor leaf claim check | `test/plugin/doctor-config-leaf-unread.ci` |
| `request bgp rib inject` from the web admin form and from CLI text | → | `ValidateArgs` then `injectRoute` reading by leaf name | `test/plugin/rib-inject-named-args.ci` |
| A Go vocabulary holding a value its YANG enum lacks | → | the extended enumeration gate | `TestEveryYANGEnumMappingAgrees` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A config leaf no compiled-in consumer declares reading, reproduced with a test module and with each owned inert leaf before it is wired or deleted (old AC-6) | The build gate fails, naming the leaf path and the fix | Only what T-5 allows, each with a reason and an owner, as `allowlist.json` holds for roots |
| AC-2 | A consumer declaration naming a path absent from the schema (old AC-7) | The gate fails naming the path | None |
| AC-3 | `ze doctor` on a configured leaf no running consumer declares reading: plugin compiled out, failed to load, or external (old AC-11, run-time half) | Reports the leaf, the cause and the fix under a doctor code of its own | An external plugin that declares nothing is reported, never assumed to read |
| AC-4 | A declared rpc with no handler, or a notification no producer emits | `./le doc yang-contract command-contract` refuses both, naming the node | None |
| AC-5 | Each owned inert leaf, rpc and notification (Owned journal rows) | Wired, with a test through its entry point, or deleted and refused with a fix, as T-4 rules per row | A row T-4 leaves as a feature for after the first release stays a gap in its own spec, never silently accepted |
| AC-6 | A YANG enum and the Go vocabulary its consumer parses differ in either direction (`log backend kmsg`, ADD-PATH `receive/send`, `diag` capture `protocol bfd`, softver `mode`) (old AC-8) | The gate refuses the disagreement whatever the enum size; the shipped enums and vocabularies match | A Go table carrying data the model lacks (an RFC number, a handler) keeps it; only the value set is compared |
| AC-7 | `request bgp rib inject`, `peer teardown`, `peer disable message`, `announce`, `peer raw`, `peer update` and the five `show bgp` commands of the 2026-08-24 row (old AC-9, second half) | Each handler reads its arguments by leaf name; each works identically from CLI text, web form and MCP | The peer selector stays positional (`ai/rules/cli.md`); a handler passing tokens to an encoding parser passes them as data, not as arguments |
| AC-8 | An argument leaf declared on both the `-cmd` container and the rpc input (old AC-9, first half) | Refused at schema build, naming both declarations; each owned command declares each argument once | None |
| AC-9 | Each owned row's reproduction (old AC-12) | No longer reproduces; the row's Fix cell turns `fixed:` with its test | None |

Old AC-10 (an argument's `description` reaches help, web and MCP) was met by
fbbbbc5cc0 before this rewrite; its row is marked `fixed:`. Old AC-1, AC-2,
AC-3, AC-4 and AC-5 live in the sibling specs named in Depends.

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Injects a route from the web admin form | form → `WriteInvocation` → dispatcher → `injectRoute` by leaf | `test/plugin/rib-inject-named-args.ci` |
| 2 | Runs `ze doctor` with a leaf set for a missing plugin | doctor → leaf claim check → report with fix | `test/plugin/doctor-config-leaf-unread.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConfigSchemaLeavesConsumed` | `internal/component/plugin/all/config_claims_test.go` | AC-1 | |
| `TestConfigLeafPhantomReads` | `internal/component/plugin/all/config_claims_test.go` | AC-2 | |
| `TestEveryYANGEnumMappingAgrees` | `internal/le/arch/enumeration/enumeration_test.go` | AC-6 | |
| `TestArgDeclaredOnce` | `internal/component/config/yang/command_test.go` | AC-8 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no numeric input added | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rib-inject-named-args` | `test/plugin/rib-inject-named-args.ci` | the same inject from CLI and web form | |
| `doctor-config-leaf-unread` | `test/plugin/doctor-config-leaf-unread.ci` | doctor names an unread leaf | |

### Interop Tests (Scope: protocol)
N-A: config and command surface only, no protocol wire change.

## Files to Modify
- `internal/component/config/claims/claims.go`, `internal/component/plugin/all/config_claims_test.go` - leaf-granularity audit
- `internal/le/config/unreadleaves/yangleafmentions.go` - per T-2
- `internal/component/doctor/checks_config_claims.go` - run-time leaf report
- `internal/le/doc/yangcontract/contract.go` - notification producer side
- `internal/le/arch/enumeration/corpus.go`, `internal/component/bgp/reactor/yang_enum_agreement_test.go` - mapping agreement
- `internal/component/bgp/reactor/config_capabilities.go`, `internal/component/hub/yang/ze-hub-conf.yang`, `internal/core/slogutil/slogutil.go` - the F rows
- `internal/component/config/yang/command.go` - one declaration per argument (AC-8)
- `internal/component/bgp/plugins/rib/rib_commands.go`, `internal/component/bgp/plugins/cmd/peer/peer.go` and the owned command handlers - D
- `internal/component/bgp/yang/ze-bgp-api.yang` - notifications and rpcs per T-4
- the owning module and consumer of each owned inert leaf - per T-4

## Files to Create
- `test/plugin/rib-inject-named-args.ci`, `test/plugin/doctor-config-leaf-unread.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | argument leaves (D), deleted leaves (E), enum values (F) |
| YANG validation constraints | Yes | typed argument leaves |
| YANG custom validators | No | None |
| CLI commands/flags | Yes | owned handlers |
| CLI grammar (keyword before value) | Yes | `./le cli grammar` over every changed command |
| Editor autocomplete | Yes | follows the declared leaves |
| Functional test for new RPC/API | Yes | `rib-inject-named-args` |
| Pipe completeness | No | answers unchanged |
| Env var registration | Yes | the 2026-08-30 env alias row |
| Doctor check for runtime dependencies | No | None |
| Doctor code per problem class | Yes | a code for the run-time unread-leaf report |
| Prometheus counters/metrics | No | None |
| BGP family surface | No | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`: deleted or newly wired leaves |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`: handlers read by leaf |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | Depends on T-2, T-3 | `docs/plugin-development/protocol.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior changed? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/config/yang-config-design.md`, "Claim Completeness Gate" |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md`; `docs/architecture/doctor-and-health-checks.md` |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | No | |
| 15 | Registered inventory changed? | No | |
| 16 | Changed source referenced by doc anchors? | Yes | derive with `./le spec citation anchors spec plan/release/fixes/spec-yang-single-source-of-truth.md`. Declared by changed files: `docs/architecture/core-design.md` (updated only if E adds a registration field, T-2 option 1) and `docs/architecture/plugin/rib-storage-design.md` (unaffected: `injectRoute` changes how it reads arguments, not what it stores) |
| 17 | Existing docs show examples for this area? | Yes | `ai/doors/config-yang.md` and `ai/doors/commands-cli.md` gate and trap rows |

## Implementation Steps

1. **Phase: Wiring** - the four wiring tests, each failing today.
2. **Phase: E gate** - leaf-granularity audit and doctor report over the declaration T-2 chooses (AC-1 to AC-3).
3. **Phase: E contract** - notification producers in yangcontract (AC-4).
4. **Phase: E rows** - wire or delete each owned node per T-4 (AC-5).
5. **Phase: F** - mapping agreement gate, then the four F rows (AC-6).
6. **Phase: D** - one declaration per argument (AC-8), then each owned handler onto the accessors (AC-7).
7. **Phase: rows** - each owned reproduction (AC-9).

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Correctness | No handler reads an argument by index; the peer selector stays positional |
| Data flow | Gates read registries and the schema, never a hand list |
| Rule: no-layering | Per-package agreement tests and hand lists removed once the generic gate covers them |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Leaf gate | `TestConfigSchemaLeavesConsumed` |
| Handlers by name | review of each owned handler against `argbind.go` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A newly declared argument leaf carries its type, range and pattern, so the validator refuses before the handler runs |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Gate reports a leaf with a real reader | Fix the declaration the reader is missing |
| Operator form lost | Re-read the handler; declare the form |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

Rows of the 2026-10-23 table that moved: A (uint64) to `spec-review-typed-config-decode`; B (three rows) to `spec-validation-walks-every-schema-section`; C (three rows) to `spec-yang-defaults-applied-once`; the 2026-10-12 `bgpMandatoryFailures` choice row to `spec-config-yang-when-unique-choice`; the 2026-09-15 thirteen-commands row to `spec-declared-commands-without-leaves`; the 2026-08-29 batching row and the 2026-09-06 `manual-eor`/`auto-flush` row to `spec-peer-leaves-the-peer-parser-never-reads`; the 2026-08-13 external plugin YANG row to `spec-plugin-external-yang-at-startup`; the two presence-container value-mode rows and the goyang `ToEntry` row to `spec-config-yang-accepted-input-correctness`; Ten rows are fixed at the producer and listed in no table. Three rows of another cause, marked "other cause:" in the table, are held here until the main thread moves them: they do not share this spec's root cause and no AC of this spec covers them. Their fitting homes refused the edit this session: `spec-config-yang-loader-structural-checks` (its AC-2 remainder is the tagging work) has no functional test in its plan, which the spec hook demands; `spec-web-cli-config-one-off-defects` (which owns the goyang `ToEntry` and `ApplyDeviate` rows) needs a producer read of thirteen files before an edit; `spec-prose-cites-symbols-not-facts` owns the `hardcoded-count-in-test` class. Every other row is prefixed with its piece, D, E or F.

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `zero-value-as-valid-answer.md` | 2026-10-10 | other cause: `goyang-enum-negative.patch` has no owner to drop it |
| `zero-value-as-valid-answer.md` | 2026-10-10 | other cause: `TestGoyangNumbersAnEnumAfterANegativeValue` follow-up |
| `hardcoded-count-in-test.md` | 2026-10-23 | other cause: `request bgp watchdog *` moved to YANG (`dcc0ab9dc5`): the wire-method snapshot half is fixed by 741d4382b7; the `published-cli-commands.json` site catalog half is still open |
| `command-takes-an-untyped-positional-value.md` | 2026-08-24 | D: five `show bgp` commands read an untyped positional |
| `command-takes-an-untyped-positional-value.md` | 2026-08-30 | `announce` handler reads args beyond the declared `selector` |
| `command-takes-an-untyped-positional-value.md` | 2026-09-04 | `peer raw`, `peer update` and siblings read undeclared positionals |
| `command-takes-an-untyped-positional-value.md` | 2026-09-04 | `request bgp rib inject` reads `args[0..2]` |
| `command-takes-an-untyped-positional-value.md` | 2026-10-21 | `peer teardown`/`disable` read `validated.Tokens()` by position |
| `argument-leaf-declared-on-container-and-rpc-input.md` | 2026-09-15 | D: 25 commands declare one leaf on the `-cmd` container and the rpc input |
| `unwired-feature.md` | 2026-10-23 | E: `label`/`labels` in a config `attribute` block never reach the route |
| `unwired-feature.md` | 2026-09-04 | `request peer <sel> borr` declared in `ze-refresh-cmd.yang`, dispatcher answers unknown command |
| `unwired-feature.md` | 2026-09-04 | fifteen leaves incl. `l2tp/shaper/upload-rate`, `bmp/route-action` with no reader |
| `unwired-feature.md` | 2026-09-04 | `ddos/flowtriq/api-key`, `bfd/enabled` and four more leaves with no reader |
| `unwired-feature.md` | 2026-09-04 | `fib/p4/*`, `fib/kernel/flush-on-stop`, `sweep-delay` inert through a type mismatch |
| `unwired-feature.md` | 2026-09-04 | `session/host-name`, `session/domain-name` legacy FQDN leaves inert |
| `unwired-feature.md` | 2026-09-04 | IPsec `remote-access` subtree and seventeen more leaves inert |
| `unwired-feature.md` | 2026-09-03 | `ospf/reference-bandwidth`, `max-metric` and two more inert |
| `unwired-feature.md` | 2026-09-03 | `unit/ipv6/dhcpv6/pd` and three more iface leaves inert |
| `unwired-feature.md` | 2026-09-03 | five `ze-bgp-api` RPCs (`peer-show`, `peer-add`, ...) with no handler |
| `unwired-feature.md` | 2026-08-30 | `environment/bgp/reactor` leaves lack the YANG-named env alias |
| `unwired-feature.md` | 2026-09-09 | `ze-bgp-api` notifications have no producer (the row names four; the model declares seven) |
| `declared-format-contradicts-payload.md` | 2026-09-05 | `rpki/cache-server/preference` declares an order Ze does not perform |
| `declared-format-contradicts-payload.md` | 2026-09-03 | `update/watchdog/withdraw` and sibling declare a meaning the producer lacks |
| `declared-format-contradicts-payload.md` | 2026-08-31 | E: `ze-plugin:session-ping` reply leaf contradicts its handler |
| `schema-omits-a-leaf-the-backend-supports.md` | 2026-09-14 | F: `environment/log/backend` enum omits `kmsg` |
| `schema-omits-a-leaf-the-backend-supports.md` | 2026-09-11 | ADD-PATH `direction` enum omits `receive/send` |
| `schema-omits-a-leaf-the-backend-supports.md` | 2026-09-14 | `diag` capture `protocol` enum omits `bfd` |
| `unwired-feature.md` | 2026-09-04 | `software-version/mode` enum offers four values, implements two (its `adj-rib` half is E) |
| `schema-omits-a-leaf-the-backend-supports.md` | 2026-09-05 | vxlan case has no `ttl` leaf (a missing leaf, not an enum: no agreement gate sees it, so it is fixed by adding the leaf) |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Build-time gate plus `ze doctor` for unconsumed nodes | Refuse at startup (the 2026-10-23 design) | Load cannot decide: plugins load on demand, compile out, or run external and declare after config load; the design page already assigns this to the build gate and doctor |
| D reuses `ValidatedArgs` accessors | A new typed accessor per argument | The accessors exist and every route already builds the value |
| F extends the enumeration gate | Keep adding per-package agreement tests | A test per table is a hand list of tables |
| Ownership with `spec-declared-commands-without-leaves`: that spec declares the leaves a command lacks (model side, the thirteen-command row); this one moves handlers that already have, or are given, leaves onto the accessors and removes double declarations | One spec for both | The model work is per-command design; the handler work is mechanical once the leaf exists. A handler whose leaf that spec has not declared waits for it |

### Decisions that are Thomas's

| Decision | Options | What each changes |
|----------|---------|-------------------|
| T-1: keep D, E and F in one spec, or split into three | One spec; three specs (D command arguments, E inert surface, F enum vocabulary) | One: one design pass and one review over three mechanisms that share no code. Three: each closes on its own; D and F are small and could land before E's gate design is settled |
| T-2: how a consumer declares the leaves it reads, for the E gate | 1. A leaf-path field on `registry.Registration` and on the Stage 1 declaration. 2. Promote the unread-leaves heuristic to a gate. 3. Consumers read only through a schema-bound accessor that records the path (needs T-3) | 1: exact, works for external plugins at run time through doctor; a public protocol field and an edit to every plugin's registration. 2: no contract change; a text match, with the false results its own comment lists. 3: exact without a declaration; changes the SDK config contract |
| T-3: a per-leaf typed config accessor in the SDK | Keep values as JSON strings (today's Stage 2 contract); add typed accessors bound to schema nodes | Keep: no `pkg/` change; parsers keep `configvalue` helpers. Add: a published SDK change, every plugin parser rewritten, and the basis for T-2 option 3 |
| T-4: for each owned row no RFC or agreed design answers, wire or delete: `remote-access` subtree, `session/host-name` and `domain-name`, `software-version/mode` values and `bgp/rib/adj-rib-*`, the seven `ze-bgp-api` notifications, the five `ze-bgp-api` rpcs, `rpki/cache-server/preference` ordering, `update/watchdog/withdraw`, `label`/`labels` on plugin families | Wire (build the behavior now); delete (refuse the config with a fix); keep as a gap owed after the first release, in a spec of its own | Wire: release scope grows. Delete: the operator surface shrinks and saved configs that set the leaf are refused. Later: the leaf stays accepted and inert until then, which is the defect, so it needs the gate's exception from T-5 |
| T-5: whether the leaf gate allows recorded exceptions | None; an allowlist like `claims/allowlist.json`, each entry with a reason and an owner | None: every row in T-4 is wired or deleted before the gate lands. Allowlist: the gate can land first and T-4's later rows wait in it, visible |

## Known Limitations
- External plugin YANG compiled at startup: `spec-plugin-external-yang-at-startup` (its AC-1 holds the old AC-11 schema half).

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" for every changed command

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests pass, run by the main thread: `./le test unit pkg path ./internal/component/plugin/all/`, `./le test unit pkg path ./internal/component/config/yang/`, `./le test unit pkg path ./internal/le/arch/enumeration/`, the owned command packages, `./le test functional plugin`, `./le test functional ui`, `./le cli grammar`, `./le doc yang-contract command-contract`, and the suites `./le repo changed scope print both` reaches. No `./le verify` command
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests N-A: no protocol wire change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + journal rows marked `fixed:`
- [ ] **Commit B:** `remove plan/release/fixes/spec-yang-single-source-of-truth.md` only
