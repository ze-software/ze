# Spec: a plugin declares the shape of its commands' answers

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | `plan/spec-cli-show-bgp-answer-shapes.md` |
| Phase | 4/5 |
| Deferral shard | `plan/deferrals/plugin-declares-answer-shape.md` |
| Handoff | - |
| Updated | 2026-08-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A plugin declares a command in its Stage 1 registration message with
`CommandDecl`. That message carries a name, a description, an argument list and
three flags. It carries no answer shape, no column order and no address-field
list, so a command served by a plugin process cannot declare what its answer
holds.

The consequence is that `validateDeclaredShape` returns early for every one of
them. The operators a plugin command supports are never published, an operator
it cannot support is never refused before dispatch, and `| display <partial>`
does not complete because `completeDisplayFields` reads the column registry.

`plan/deferrals/plugin-registers-pipe-operations.md` deferred exactly this on
2026-08-21, when the alias channel shipped without it: "Ordering is a second
declaration channel with its own collision and inheritance rules, and folding it
into a spec whose subject is aliases would give both halves one set of tests."
Its destination was "a spec of its own, not yet written". This is that spec.

Eleven `show bgp` commands are the population that motivates it, and every one
of them is a plugin command with no in-core shim to declare on its behalf.

| Plugin | Commands |
|--------|----------|
| rpki | `show bgp rpki`, `... status`, `... cache`, `... roa`, `... summary`, `... aspa` |
| rs | `show bgp rs status`, `show bgp rs peers` |
| adj-rib-in | `show bgp adj-rib-in`, `show bgp adj-rib-in status` |
| healthcheck | `show bgp healthcheck` |

The channel is general: every plugin command in the tree gains the ability, and
this spec declares for the eleven above because they are the set already
measured. One of them, `show bgp healthcheck`, also answers a different SHAPE
for a different argument, which is the defect class `spec-cli-show-bgp-answer-shapes`
fixes for its own two instances. `show bgp rpki aspa` is the other.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md`, "A plugin declares a pipe alias in its
      Stage 1 message" and the two sections after it
  → Decision: `validatePipeDecls` reads the shape and the ownership, in the
    position where Stage 1 already validates doctor checks and enrichers, BEFORE
    it converts anything. `registerPluginPipes` then writes the accepted set
    under `startupRegistrationMu`, and each later failure unwinds what the steps
    above it wrote. A shape declaration joins that sequence rather than
    inventing one.
  → Decision: a plugin names only a path it declared itself in the same message.
    A path another PLUGIN declared is refused a step earlier, by
    `PluginRegistry.Register`.
  → Decision: a refusal fails the WHOLE Stage 1 registration and the plugin does
    not start, so a plugin never has to undo a partial registration.
  → Constraint: "A declaration ADDS to a path. It never replaces what the path
    holds." The alias registry merges. A shape and a column order cannot merge,
    so the collision rule from the dependency spec is what makes a plugin
    declaration land on a path the BGP command plugin has already blanked.

- [ ] `ai/rules/plugins.md` - the plugin boundary
  → Constraint: no plugin spelling in a generic or central package. The engine
    reads a declaration; it never learns which plugin sent it.

- [ ] `docs/contributing/ze-style.md` - the working standard for every line of Go
  → Constraint: a limit on everything. A declaration arriving from another
    process is external input, so the list lengths and the name lengths are
    bounded before they are stored.
  → Constraint: `panic("BUG:")` is for a state only a Ze defect can reach. A bad
    plugin message is not that state, so it is REFUSED, never panicked on.

**Key insights:**
- The dependency spec makes `commandRegistry.register` treat an empty
  declaration as a floor. Without it, a plugin declaring a shape on
  `show bgp rpki` would either be overwritten by, or overwrite, the empty
  declaration the BGP command plugin writes for every child of `show bgp`,
  depending on which ran last.
- The published catalog cannot see any of this. `make ze-command-list` and
  `ze help command --json` read the compiled tree in their own process and start
  no plugin, so a declaration that reaches only a running daemon does not reach
  the page. That is recorded in
  `plan/deferrals/plugin-registers-pipe-operations.md`, row 2, and this spec
  does not close it.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/plugin/rpc/types.go` - `CommandDecl` carries `Name`, `Description`,
      `Args`, `Completable`, `Hidden`, `DeprecatedNames`. `PipeDecl` carries
      four strings. Both are named in `DeclareRegistrationInput`.
- [ ] `internal/component/plugin/server/startup.go` - `validatePipeDecls` runs
      before any conversion; `registerPluginPipes` converts and writes; the
      caller unwinds each earlier step on a later failure.
- [ ] `internal/component/command/alias.go` - `RegisterPluginAliases` and
      `UnregisterPluginAliases`: a plugin's declaration is taken back under the
      plugin name when it stops.
- [ ] `internal/component/command/answer_shape.go` - `RegisterShape`,
      `RegisterAddressFields`, and the `AnswerShape` values.
- [ ] `internal/component/command/pipe_catalog.go` - `AnswerShape.String()`
      answers `doc`, `map` or `tab`. Nothing parses that spelling back.
- [ ] `internal/component/command/column_order.go` - `RegisterColumns` and the
      shared `commandRegistry[T]`.

**The eleven answers, as their producers write them.** Every row was read from
the producing function.

| Command | Producer | Shape | Rows under | Row fields in producer order | Address or prefix fields |
|---------|----------|-------|------------|------------------------------|--------------------------|
| `show bgp rpki` | `overviewCommand` | tab | `cache-servers` | `address`, `port`, `state`, `synced`, `version` | `address` |
| `show bgp rpki status` | `statusCommand` | doc | two candidate row sets, `cache-servers` and `peer-actions`, which is the ambiguous case `rowsInKeyed` refuses | — | — |
| `show bgp rpki cache` | `cacheCommand` | tab | `cache-servers` | `address`, `port`, `preference`, `state`, `synced`, `version`, `session-id`, `serial`, `refresh-interval`, `retry-interval`, `expire-interval` | `address` |
| `show bgp rpki roa` | `roaCommand`, `roaLookupCommand` | tab | `entries` | `prefix`, `max-length`, `asn` | `prefix` |
| `show bgp rpki summary` | `summaryCommand` | doc | none | — | — |
| `show bgp rpki aspa` | `aspaCommand` | tab | `entries` | `customer-asn`, `providers` | none |
| `show bgp rs status` | `handleCommand` | doc | none | — | — |
| `show bgp rs peers` | `peerStatus` | tab | `peers` | `address`, `remote`, `up` | `address` |
| `show bgp adj-rib-in` | `AdjRIBInManager.show` | tab | `adj-rib-in`, a map keyed by peer address whose values are ARRAYS | `family`, `key`, `nhop-hex`, `attr-hex`, `nlri-hex`, `seq-index`, `validation-state` | the map key is a peer address; `key` embeds the prefix in a compound string |
| `show bgp adj-rib-in status` | `AdjRIBInManager.status` | doc | `peers` is a map of address to a COUNT, not to an object, so it is no row set | — | the map keys are addresses |
| `show bgp healthcheck` | `probeManager.handleShow` | tab | the answer itself, a bare array with no envelope key | `name`, `group`, `state` | none |

**Behavior to preserve:**
- The answer payload of every command except `show bgp healthcheck` and
  `show bgp rpki aspa`, whose argument-selected branches change.
- Stage 1 registration refusing the WHOLE message when any one declaration is
  bad, so a plugin never starts half-registered.
- A plugin's declarations leaving with the plugin.

**Behavior to change:**
- `CommandDecl` carries three optional fields.
- `show bgp healthcheck` with a probe name answers a one-row set rather than one
  object.
- `show bgp rpki aspa` with a customer ASN answers a one-row set rather than one
  object.
- The eleven commands declare.

## Data Flow (MANDATORY)

### Entry Point
- A plugin process sends its Stage 1 `DeclareRegistrationInput` over the plugin
  RPC transport, as JSON.

### Transformation Path
1. The engine validates the declared commands and registers them.
2. `validateShapeDecls` reads the three new fields on each `CommandDecl`,
   refusing an unknown shape spelling, a field declaration with no shape, and a
   list past its bound. It runs where `validatePipeDecls` runs, before any
   conversion.
3. `registerPluginShapes` converts and writes into the shape, column and
   address-field registries under `startupRegistrationMu`.
4. A later failure in the sequence unwinds it, as it unwinds the alias write.
5. When the plugin stops, `UnregisterPluginShapes` removes what it wrote, so the
   paths return to what they held before.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | three optional JSON fields on `CommandDecl`. Absent means undeclared, which is today's behavior | No |
| Plugin process ↔ command registries | the engine writes; the plugin never reaches a registry | No |

### Integration Points
- `validatePipeDecls` / `registerPluginPipes` - the new pair sits beside them
  and joins the same unwind.
- `UnregisterPluginAliases` - the new unregister runs beside it on plugin stop.
- `commandRegistry.register` - the floor rule from the dependency spec is what
  lets a plugin declaration land on a blanked path.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | |
| No unintended coupling | No | |
| No duplicated functionality | No | |
| Zero-copy preserved where applicable | No | |
| Registration over hardcoding | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The dependency spec's floor rule has landed, so a plugin declaring onto a path the BGP command plugin blanked wins | `plan/spec-cli-show-bgp-answer-shapes.md` Phase 1 | The declaration is silently dropped, or drops the empty declaration and lets the child inherit `show bgp`'s peer columns | `TestPluginShapeOverridesEmptyDeclaration` | unvalidated |
| A-2 | No caller depends on `show bgp healthcheck` answering one object for a named probe, nor on `show bgp rpki aspa` answering one object for a customer ASN | The commands are reached only through the dispatcher | A caller breaks | `gopls references` on `handleShow` and `aspaCommand`, and a grep of `test/` for both command paths | confirmed 2026-08-24. `handleShow` is called only by `handleCommand` (`healthcheck.go`) and `aspaCommand` only by `handleCommand` (`rpki.go`); every other reference is a test in the same package. One `.ci` reads the named-probe answer, `test/plugin/as112-probe-anycast-not-loopback.ci`, and it matches the SUBSTRINGS `state: UP` and `state: DOWN` in the `\| yaml` render, which survive the two-space sequence indent `writeMapItem` (`internal/component/command/format.go`) adds. No `.ci` reads the aspa lookup answer |
| A-3 | A plugin that stops and restarts re-declares, so removal on stop loses nothing | `UnregisterPluginAliases` already works this way | A restarted plugin's commands lose their declarations | `TestUnregisterPluginShapes` and a plugin restart in a `.ci` |ered unvalidated |
| A-4 | `show bgp rpki status` and `show bgp adj-rib-in status` genuinely hold no single row set | Read of `rowsInKeyed` against both producers: one has two candidate keys, the other maps an address to a scalar | Declaring `doc` refuses a row operator that used to answer | A `.ci` asserting the refusal names the operator | confirmed 2026-08-24. `statusCommand` (`rpki.go`) writes two candidate keys, pinned by `TestDocCommandsHoldNoSingleRowSet`; `AdjRIBInManager.status` (`rib_commands.go`) maps an address to an `int`, pinned by `TestStatusHoldsNoRowSet`. `test/ui/show-bgp-plugin-shapes.ci` asserts both refusals by operator name and on `cannot apply here` |
| A-5 | `show bgp adj-rib-in` holds a row set keyed by peer address, so the `first 1` operator answers one peer's routes. This is the premise of AC-16 and of the Current Behavior row that calls the command `tab` | The Current Behavior table read the payload as "a map keyed by peer address whose values are ARRAYS" and treated that as rows | AC-16 cannot be satisfied, and the command must declare `doc` rather than `tab` | Read of `rowSet` (`internal/component/command/answer_shape.go`) against `AdjRIBInManager.show` (`rib_commands.go`) | **broken 2026-08-24**. `rowSet` reads a map as rows only when EVERY value is an object, and the peer map's values are arrays, so it is no row set. The one candidate left is the envelope itself: one row named `adj-rib-in` carrying every peer, over which the `first 1` operator answers the whole table and `count` answers 1. Making AC-16 true needs the peer map to hold objects, which changes a payload "Behavior to preserve" protects and which `test/interop/scenarios/show-rib-under-frr-load/check.py`, `test/interop/scenarios/rpki-frr/rpki-check.py` and `test/scripts/ze_api.py` navigate. Phase 4 therefore declares `doc`, which refuses the operator by name, and AC-16 is put to the owner |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Adding fields to `CommandDecl` breaks a plugin built against the old shape | A plugin fails Stage 1 | All three are optional and absent means undeclared. The change is additive, and JSON ignores an unknown field in the other direction |
| R-2 | A plugin declares a column name its handler does not write, so the order is inert and the published field does not exist | Nothing fails. This is the risk with no signal | The engine cannot check a name against a payload it has not seen. The `.ci` asserts each declared name appears in the rendered answer, which catches it for the eleven; for a third-party plugin it stays the author's responsibility and the doc says so |
| R-3 | An unbounded list in a plugin message is stored | Memory growth at Stage 1 | Bound the list length and each name's length, and refuse past it, naming the plugin and the command |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A plugin fails to start, which takes its commands and its filters with it. Nothing on the wire and nothing in the RIB |
| How is it reverted? | A single commit revert. The fields are additive and nothing persists them |
| Who else touches this path? | `spec-plugin-registers-pipe-operations` (in-progress, phase 6 of 6) owns `startup.go` and `alias.go`. Coordinate before Phase 2 |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A plugin sends a Stage 1 message carrying a shape | → | `registerPluginShapes` writes it into `shapeRegistry` | `TestRegisterPluginShapes` |
| A plugin sends an unknown shape spelling | → | `validateShapeDecls` refuses the message | `TestValidateShapeDecls` |
| A plugin stops | → | `UnregisterPluginShapes` removes what it wrote | `TestUnregisterPluginShapes` |
| `show bgp rpki cache \| display address state` typed at the CLI | → | the rpki plugin's declaration reaches `ColumnsForCommand` | `test/ui/show-bgp-plugin-shapes.ci` |
| `show bgp rs peers \| resolve` typed at the CLI | → | the rs plugin's declared address field reaches `applyResolve` | `test/ui/show-bgp-plugin-shapes.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A plugin declares a command with a shape of `tab`, a column order and an address field | All three reach the registries, and the command's operators are published from them |
| AC-2 | A plugin declares a shape that is not `doc`, `map` or `tab` | Stage 1 is refused, the message names the plugin, the command and the value, and the plugin does not start |
| AC-3 | A plugin declares a column order or an address field with no shape | Stage 1 is refused, saying a field declaration needs a shape |
| AC-4 | A plugin declares a shape for a command path it did not declare in the same message | Stage 1 is refused, as it is for an alias on an undeclared path |
| AC-5 | A plugin declares a list past its bound, or a name past its bound | Stage 1 is refused, naming the plugin, the command and the bound |
| AC-6 | A plugin declares onto a path the BGP command plugin declared empty | The plugin's declaration is what resolves |
| AC-7 | A plugin stops | Its shape, column and address-field declarations are removed, and each path returns to what it held before |
| AC-8 | A plugin fails a later Stage 1 step after its shapes were written | The shape write is unwound with the rest, and no declaration survives |
| AC-9 | `show bgp rpki cache \| display address state` | Answers those two fields, in that order |
| AC-10 | `show bgp rpki cache \| resolve` | Decorates the `address` field |
| AC-10b | `show bgp rpki \| resolve` over the OVERVIEW answer, and `show bgp rpki summary \| resolve` | Each is judged on its own declared address-field list, and neither inherits the `address` field `show bgp` declares. **Added 2026-08-24**, displaced from `spec-cli-show-bgp-answer-shapes` AC-7: that spec cannot satisfy it, because `validateDeclaredShape` (`internal/component/command/pipe.go`) returns at `if !declared` before it reads the address-field list, so an address operator is refused only once a SHAPE is declared, and `show bgp rpki`'s shape is declared here. The empty address-field declaration the BGP peer command plugin writes for every child of `show bgp` is what makes the refusal correct rather than accidental, and it landed in that spec's Phase 2 |
| AC-11 | `show bgp rpki summary \| first 2` | Refused by name: the answer is one document |
| AC-12 | `show bgp rpki status \| count` | Refused by name: the answer holds two candidate row sets and no single one |
| AC-13 | `show bgp rs peers \| count` | Answers the peer count |
| AC-14 | `show bgp healthcheck` with a probe name | Answers a one-row set, in the same spelling it uses with no argument |
| AC-15 | `show bgp rpki aspa` with a customer ASN | Answers a one-row set, in the same spelling it uses with no argument |
| AC-16 | ~~`show bgp adj-rib-in \| first 1`~~ | ~~Answers one peer's routes~~ **FALSE AS WORDED, 2026-08-24, Phase 4, and the Current Behavior table was wrong with it.** `AdjRIBInManager.show` (`internal/component/bgp/plugins/adj_rib_in/rib_commands.go`) writes an envelope whose `adj-rib-in` key holds a map of peer address to an ARRAY of routes. `rowSet` (`internal/component/command/answer_shape.go`) reads a map as rows only when EVERY value is an object, so that map is not a row set; the only candidate left is the envelope itself, read as ONE row named `adj-rib-in` holding every peer. `first 1` would answer the whole table and `count` would answer 1. Declaring `tab` with the route field names would be wrong twice over, because those names are keys two levels below any row. The command therefore declares `doc`, which refuses the row operators by name. Making AC-16 true means the peer map must hold objects rather than arrays, which changes a payload "Behavior to preserve" protects and which three consumers navigate as it stands (`test/interop/scenarios/show-rib-under-frr-load/check.py`, `.../rpki-frr/rpki-check.py`, `test/scripts/ze_api.py`). That is a payload question rather than a declaration question, so it is Thomas's call and not this spec's. Recorded as A-5 broken |
| AC-17 | Every one of the eleven commands | Declares a shape, and declares a column order and an address-field list where its answer has rows and addresses |
| AC-18 | `ze help command --json` for a plugin `show bgp` path, from a RUNNING daemon | Lists the operators that path supports |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Resolves the RTR cache servers: `show bgp rpki cache \| resolve` | CLI → declared address field from Stage 1 → dispatch to the rpki process → `applyResolve` | `test/ui/show-bgp-plugin-shapes.ci` |
| 2 | Reads probe states as a table: `show bgp healthcheck \| display name state` | CLI → declared order → healthcheck process | `test/ui/show-bgp-plugin-shapes.ci` |
| 3 | Types an operator a plugin command cannot support: `show bgp rpki summary \| count` | CLI → `validateShapeDecls` wrote `doc` → refused by name before dispatch | `test/ui/show-bgp-plugin-shapes.ci` |
| 4 | Writes a plugin and declares its answer shape | plugin SDK → Stage 1 → the three registries | `TestRegisterPluginShapes` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseAnswerShape` | `internal/component/command/pipe_catalog_test.go` | the wire spelling round-trips against `String()`, and a fourth spelling is refused | |
| `TestValidateShapeDecls` | `internal/component/plugin/server/startup_test.go` | AC-2, AC-3, AC-4, AC-5 | |
| `TestRegisterPluginShapes` | `internal/component/plugin/server/startup_test.go` | AC-1 | |
| `TestPluginShapeOverridesEmptyDeclaration` | `internal/component/plugin/server/startup_test.go` | AC-6, and A-1 | |
| `TestUnregisterPluginShapes` | `internal/component/plugin/server/startup_test.go` | AC-7 | |
| `TestShapeWriteUnwindsWithStageOne` | `internal/component/plugin/server/startup_test.go` | AC-8 | |
| `TestHealthcheckNamedProbeAnswersRows` | `internal/component/bgp/plugins/healthcheck/healthcheck_test.go` | AC-14 | |
| `TestAspaLookupAnswersRows` | `internal/component/bgp/plugins/rpki/rpki_test.go` | AC-15 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| declared column count per command | 0-64 | 64 | N/A | 65, refused |
| declared address-field count per command | 0-16 | 16 | N/A | 17, refused |
| declared field-name length | 1-64 | 64 | 0, refused | 65, refused |
| declared shape spelling | doc, map, tab | tab | N/A | any fourth spelling, refused |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-bgp-plugin-shapes` | `test/ui/show-bgp-plugin-shapes.ci` | An operator uses `\| display`, `\| resolve` and `\| count` over plugin-served `show bgp` commands, and is refused BY NAME where the answer holds no rows | passing 2026-08-24 |
| `plugin-shape-declaration-refused` | `test/plugin/plugin-shape-declaration-refused.ci` | A plugin declaring a bad shape does not start, and the daemon log names the plugin, the command and the value | |

### Interop Tests (Scope: protocol)
Not applicable. Nothing wire-visible changes. The plugin RPC contract changes
additively, and it is not a protocol Ze speaks to another implementation.

## Files to Modify
- `pkg/plugin/rpc/types.go` - three optional fields on `CommandDecl`
- `pkg/plugin/sdk/sdk_types.go` - the SDK re-export
- `internal/component/plugin/server/startup.go` - `validateShapeDecls`,
  `registerPluginShapes`, and the unwind
- `internal/component/command/answer_shape.go` - `RegisterPluginShapes` and
  `UnregisterPluginShapes`, taken back under the plugin name
- `internal/component/command/pipe_catalog.go` - a parser from the wire spelling
  to `AnswerShape`
- `internal/component/bgp/plugins/rpki/rpki.go` - six declarations, and the aspa
  lookup answering rows
- `internal/component/bgp/plugins/rs/server.go` - two declarations
- `internal/component/bgp/plugins/adj_rib_in/rib.go` - two declarations
- `internal/component/bgp/plugins/healthcheck/healthcheck.go` - one declaration,
  and the named-probe branch answering rows
- `docs/architecture/api/commands.md` - the shape channel, beside the alias
  channel
- `docs/architecture/api/ipc_protocol.md` - declared by `pkg/plugin/rpc/types.go`:
  the three new fields on the Stage 1 command declaration
- `docs/architecture/api/process-protocol.md` - the Stage 1 sequence a shape
  declaration joins
- `docs/architecture/bgp/healthcheck-plugin.md` - declared by
  `healthcheck.go`: the named-probe answer changes shape
- `docs/plugin-development/commands.md` - what a plugin author writes to declare
  a command's answer shape
- `ai/rules/plugins.md` - what a plugin declares at Stage 1

## Files to Create
- `test/ui/show-bgp-plugin-shapes.ci`
- `test/plugin/plugin-shape-declaration-refused.ci`
- `plan/deferrals/plugin-declares-answer-shape.md`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | No | No new command path and no new config leaf |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | No | No command is added, renamed or removed |
| CLI grammar (keyword before value) | No | No grammar change |
| Editor autocomplete | Yes | `completeDisplayFields` reads the column registry, so a plugin's declared order makes `\| display <partial>` complete on its commands |
| Functional test for new RPC/API | Yes | The two `.ci` files above |
| Pipe completeness | Yes | This spec is the pipe-completeness work for plugin-served commands |
| Env var registration | N-A | No new env var |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port or binary |
| Prometheus counters/metrics | No | A declaration is not observable state |
| BGP family surface | N-A | No SAFI, capability or attribute change |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features/cli-commands.md` |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`: the two answers that change shape |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | No BGP wire change |
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior? | N-A | Nothing here implements an RFC obligation |
| 10 | Test infrastructure changed? | No | Both `.ci` files use the existing runner |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/commands.md` |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | No | |
| 15 | Registered command or capability changed? | Yes | `docs/features/plugins.md`, `docs/plugin-overview.md`: what a plugin declares at Stage 1 |
| 16 | Changed source file referenced by doc source anchors? | DERIVED | Run `python3 scripts/dev/spec_doc_anchors.py plan/spec-plugin-declares-answer-shape.md` at the start of each phase. Two declared docs are UNAFFECTED. `docs/architecture/core-design.md`, declared by `rs/server.go`, describes the route server's place in the engine; that file gains two declarations stating what its existing answers already hold, and no behavior the doc records changes. `docs/architecture/plugin/rib-storage-design.md`, declared by `adj_rib_in/rib.go`, describes Adj-RIB-In raw hex storage; that file gains two declarations and stores nothing differently |
| 17 | Existing docs show examples for this area? | Yes | Check the Stage 1 registration examples in `docs/architecture/api/process-protocol.md` |

## Implementation Steps

1. **Phase 1: Wiring (MANDATORY FIRST)** - the contract and the parser
   - `CommandDecl` gains `Shape`, `Columns` and `AddressFields`, all optional.
     `AnswerShape` gains a parser from its wire spelling. Nothing reads them
     yet, so the wiring test fails.
   - Tests: `TestParseAnswerShape`
   - Files: `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_types.go`,
     `internal/component/command/pipe_catalog.go`
   - Verify: the fields survive a round trip through the Stage 1 message and
     reach no registry
2. **Phase 2: Validation and registration**
   - `validateShapeDecls` refuses an unknown spelling, a field declaration with
     no shape, a path the plugin did not declare, and a list or a name past its
     bound. `registerPluginShapes` writes under `startupRegistrationMu`, and
     `UnregisterPluginShapes` takes it back on stop. Both join the existing
     unwind. Coordinate with `spec-plugin-registers-pipe-operations`, which owns
     the same file.
   - Tests: `TestValidateShapeDecls`, `TestRegisterPluginShapes`,
     `TestPluginShapeOverridesEmptyDeclaration`, `TestUnregisterPluginShapes`,
     `TestShapeWriteUnwindsWithStageOne`,
     `test/plugin/plugin-shape-declaration-refused.ci`
   - Files: `startup.go`, `answer_shape.go`
   - Verify: AC-1 to AC-8, and A-1
3. **Phase 3: One shape whatever the argument**
   - `show bgp healthcheck` with a probe name and `show bgp rpki aspa` with a
     customer ASN each answer a one-row set. Confirm A-2 first.
   - Tests: `TestHealthcheckNamedProbeAnswersRows`, `TestAspaLookupAnswersRows`
   - Files: `healthcheck.go`, `rpki.go`
   - Verify: AC-14, AC-15
4. **Phase 4: The eleven commands declare**
   - Each of the four plugins declares its commands' shapes, column orders and
     address fields, from the table in Current Behavior.
   - Tests: `test/ui/show-bgp-plugin-shapes.ci`
   - Files: `rpki.go`, `rs/server.go`, `adj_rib_in/rib.go`, `healthcheck.go`
   - Verify: AC-9 to AC-13, AC-16, AC-17, AC-18, and A-4
5. **Phase 5: Documentation**
   - Files: `docs/architecture/api/commands.md`,
     `docs/architecture/api/process-protocol.md`, `ai/rules/plugins.md`, and
     every row the Documentation Update Checklist answers Yes

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All eleven commands declare, and each declaration matches the table read from its producer |
| Correctness | A bad plugin message is REFUSED and never panics. The panic in `commandRegistry.register` MUST NOT be reachable from a plugin declaration |
| Correctness | A refused declaration leaves NO partial write. The unwind covers the shape write as it covers the alias write |
| Correctness | A plugin's declaration replaces the empty declaration the BGP command plugin wrote, and is itself removed on stop, leaving the empty declaration behind rather than nothing |
| Naming | Every declared column name is a key the plugin handler actually writes, checked against the rendered answer in the `.ci` |
| Naming | Every declared address field holds an address in every branch the producer can take |
| Data flow | The engine never learns which plugin a declaration came from, beyond the owner name it needs to take it back |
| Rule: `ai/rules/plugins.md` | No plugin spelling reaches `internal/component/command` |
| Rule: `ai/rules/cli.md` | No plugin command accepts an operator and answers something plausible |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `CommandDecl` carries the three fields | `TestParseAnswerShape` and the Stage 1 round trip |
| A bad declaration stops the plugin | `test/plugin/plugin-shape-declaration-refused.ci` |
| A stopped plugin leaves no declaration | `TestUnregisterPluginShapes` |
| All eleven commands declare | `test/ui/show-bgp-plugin-shapes.ci` |
| No operator is accepted and ignored | The `.ci` asserts a refusal BY NAME for each unsupported operator |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A shape spelling, a column list and an address-field list arrive from another process. Each is validated against a closed set or a bound BEFORE it is stored, and refused by name otherwise |
| Resource exhaustion | List length and name length are bounded, per the Boundary Tests table. An unbounded list from a plugin message MUST NOT reach a registry |
| Error leakage | A refusal names the plugin, the command and the offending value, with the value CLAMPED before it reaches the log. An unbounded plugin string MUST NOT be mirrored into the daemon log |
| Authorization | None. A declaration changes rendering and refusal, never what a caller may run |

### Failure Routing
| Failure | Route To |
|---------|----------|
| The registry panics on a plugin declaration | The panic is reachable from a plugin message, which this spec forbids. Fix the path so the plugin is refused instead |
| A declared column name is absent from the rendered answer | Re-read the producing function. The declaration is wrong, never the payload |
| `startup.go` conflicts with `spec-plugin-registers-pipe-operations` | Stop and coordinate. Do not resolve a conflict in a file another in-progress spec owns |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The alias channel and the shape channel meet the same collision on the same
  paths and answer it differently, and the reason is the ARITY of the value. A
  set of aliases has a union, so a plugin's names merge with what the path
  holds. A shape and a column order have no union, so the answer is a floor rule
  in the registry rather than a merge at the call site.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Three optional fields on `CommandDecl` | A fourth list on `DeclareRegistrationInput`, beside `Pipes` | A shape belongs to a command, one for one. A separate list would need to name the command again and would let a plugin declare a shape for a command it did not declare, which is a refusal the pairing makes impossible to write |
| A bad declaration REFUSES the plugin; it never panics | Reuse the in-tree panic | A plugin message is external input. `docs/contributing/ze-style.md` reserves `panic("BUG:")` for a state only a Ze defect can reach, and a plugin taking the daemon down is the failure mode this avoids |
| The eleven declarations live in each plugin's own registration | A table in the engine mapping command paths to shapes | A table in a central package is plugin spelling in a generic package, which `ai/rules/plugins.md` bans. Remove the plugin and its declaration MUST vanish with it |
| `show bgp rpki status` and `show bgp adj-rib-in status` declare `doc` | Reshape both so they carry one row set | Both are genuinely documents: one holds two candidate row sets, the other maps an address to a scalar. Reshaping them to please an operator would change an answer this spec has no reason to change |

## Known Limitations

- The published catalog still cannot show a plugin's declaration.
  `make ze-command-list` and `ze help command --json` read the compiled tree and
  start no plugin, so the wiki page lists a plugin's commands without their
  operators. Recorded in `plan/deferrals/plugin-registers-pipe-operations.md`,
  row 2, and carried forward here.
- The engine cannot check a declared column name against a payload it has not
  seen. For the eleven commands the `.ci` checks it; for a third-party plugin it
  stays the author's responsibility.
- `show bgp adj-rib-in` answers ONE DOCUMENT, and every row operator over it is
  refused by name. `AdjRIBInManager.show` maps each peer address to an ARRAY of
  routes, and `rowSet` reads a map as rows only when every value is an object,
  so the payload holds no row set the engine can address. See A-5: the shape
  this spec's Current Behavior table predicted for the command was `tab`, and
  the producer does not support it. The peer address is also the map KEY rather
  than a field, so no address field is declared and `| resolve` is refused for
  that separate reason -- the identity-keyed row set limitation carried by
  `plan/deferrals/cli-show-bgp-answer-shapes.md`.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-18 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests N-A: nothing wire-visible changes

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-plugin-declares-answer-shape.md` only
