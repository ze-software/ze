# Spec: a plugin declares the shape of its commands' answers

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | `spec-cli-show-bgp-answer-shapes` |
| Phase | 5/5 |
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

the retired deferral shard "plugin-registers-pipe-operations" deferred exactly this on
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

- [ ] `docs/contributing/ze-go-style.md` - the working standard for every line of Go
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
- The published catalog cannot see any of this. `./le command list` and
  `ze help command --json` read the compiled tree in their own process and start
  no plugin, so a declaration that reaches only a running daemon does not reach
  the page. That is recorded in
  the retired deferral shard "plugin-registers-pipe-operations", row 2, and this spec
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
| `show bgp adj-rib-in` | `AdjRIBInManager.show` | map (this row read `tab` when it was written, which was wrong: see A-5) | `adj-rib-in`, rows keyed by peer address, each row that peer's routes as a LIST | none. A column name orders the keys of a ROW, and these sit one level below it | the map key is a peer address; `key` embeds the prefix in a compound string |
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
| A-1 | The dependency spec's floor rule has landed, so a plugin declaring onto a path the BGP command plugin blanked wins | `spec-cli-show-bgp-answer-shapes` Phase 1 | The declaration is silently dropped, or drops the empty declaration and lets the child inherit `show bgp`'s peer columns | `TestPluginShapeOverridesEmptyDeclaration` | unvalidated |
| A-2 | No caller depends on `show bgp healthcheck` answering one object for a named probe, nor on `show bgp rpki aspa` answering one object for a customer ASN | The commands are reached only through the dispatcher | A caller breaks | `gopls references` on `handleShow` and `aspaCommand`, and a grep of `test/` for both command paths | confirmed 2026-08-24. `handleShow` is called only by `handleCommand` (`healthcheck.go`) and `aspaCommand` only by `handleCommand` (`rpki.go`); every other reference is a test in the same package. One `.ci` reads the named-probe answer, `test/plugin/as112-probe-anycast-not-loopback.ci`, and it matches the SUBSTRINGS `state: UP` and `state: DOWN` in the `\| yaml` render, which survive the two-space sequence indent `writeMapItem` (`internal/component/command/format.go`) adds. No `.ci` reads the aspa lookup answer |
| A-3 | A plugin that stops and restarts re-declares, so removal on stop loses nothing | `UnregisterPluginAliases` already works this way | A restarted plugin's commands lose their declarations | `TestUnregisterPluginShapes` | confirmed 2026-09-08. That test drives the real Stage 1 path: it starts the plugin, asserts `ShapeForCommand` answers `tab`, calls `rollbackStartupProcess`, asserts the path returns to the EMPTY declaration rather than to nothing, and then runs `runPluginPhase` a second time and asserts the shape is declared again |
| A-4 | `show bgp rpki status` and `show bgp adj-rib-in status` genuinely hold no single row set | Read of `rowsInKeyed` against both producers: one has two candidate keys, the other maps an address to a scalar | Declaring `doc` refuses a row operator that used to answer | A `.ci` asserting the refusal names the operator | confirmed 2026-08-24. `statusCommand` (`rpki.go`) writes two candidate keys, pinned by `TestDocCommandsHoldNoSingleRowSet`; `AdjRIBInManager.status` (`rib_commands.go`) maps an address to an `int`, pinned by `TestStatusHoldsNoRowSet`. `test/ui/show-bgp-plugin-shapes.ci` asserts both refusals by operator name and on `cannot apply here` |
| A-5 | `show bgp adj-rib-in` holds a row set keyed by peer address, so the `first 1` operator answers one peer's routes. This is the premise of AC-16 and of the Current Behavior row that calls the command `tab` | The Current Behavior table read the payload as "a map keyed by peer address whose values are ARRAYS" and treated that as rows | AC-16 cannot be satisfied, and the command must declare `doc` rather than `tab` | Read of `rowSet` (`internal/component/command/answer_shape.go`) against `AdjRIBInManager.show` (`rib_commands.go`) | **broken 2026-08-24, repaired 2026-09-06**. `rowSet` reads a map as rows only when EVERY value is an object, and the peer map's values are arrays, so it is no row set. The one candidate left is the envelope itself: one row named `adj-rib-in` carrying every peer, over which the `first 1` operator answers the whole table and `count` answers 1. Making AC-16 true needs the peer map to hold objects, which changes a payload "Behavior to preserve" protects and which `test/interop/scenarios/show-rib-under-frr-load/check.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) -->, `test/interop/scenarios/rpki-frr/rpki-check.py` (retired; now `internal/le/interoplab/bgp/`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> and `test/scripts/ze_api.py` (retired, no successor) <!-- doc-links: ignore (deleted 2026-08-28 by eae282592 with no replacement) --> navigate. Phase 4 therefore declares `doc`, which refuses the operator by name, and AC-16 is put to the owner | RULED 2026-09-06: the reader was taught the shape rather than the payload reshaped, so the assumption now HOLDS. `rowSet` reads an identity map whose values share one shape, the command declares `map`, and `| first 1` answers one peer's routes. The Current Behavior row calling the command `tab` stays wrong, because a column name orders the keys of a ROW and a row here is a list.

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
| AC-16 | `show bgp adj-rib-in \| first 1` | Answers one peer's routes. **RULED 2026-09-06 by the owner, and no longer struck: the ROW READER was wrong, and the payload is unchanged.** The peer address is the row's identity and it is already in the answer, as the map key, so nothing is invented to name a row. `rowSet` (`internal/component/command/answer_shape.go`) reads an identity map whose values share ONE shape, so a row is a list under this command and an object under `show bgp peer list`; `identityValuesShareOneShape` keeps a mixed map one document. `commandDecls` (`internal/component/bgp/plugins/adj_rib_in/rib.go`) declares `map` for the command, which is what admits the operator before dispatch. `| first 1` answers one peer's routes under that peer's address, and `| count` answers the peer count. Proved from the real chain by `TestShowAnswersRowsKeyedByPeer`, `TestShowCountsThePeers` and `TestShowRendersInEveryFormat` (`rib_shape_test.go`), each red before the fix, and asserted end to end by `test/ui/show-bgp-plugin-shapes.ci`. The three consumers the earlier reading protected are retired, and no payload change was needed for any of them |
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
- `docs/plugin-development/protocol.md` - declared by `pkg/plugin/rpc/types.go`:
  the `CommandDecl` field table the Stage 1 page was missing
- `docs/features/cli-commands.md` - the eleven plugin paths now declare
- `docs/guide/command-reference.md` - the healthcheck rows
- `docs/guide/healthcheck.md` - declared by `healthcheck.go`: both spellings
  answer a row set
- `docs/guide/rpki.md` - declared by `rpki.go`: the aspa lookup answers rows
- `ai/rules/points/plugins/answer-shape-declaration-stage-1-wire-protocol/` -
  six points, plus the manifest line. `ai/rules/plugins.md` is RENDERED from it

## Files to Create
- `test/ui/show-bgp-plugin-shapes.ci`
- `test/plugin/plugin-shape-declaration-refused.ci`
- the retired deferral shard "plugin-declares-answer-shape"

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
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`: DONE, a "Declaring what your command's answer holds" section beside the pipe alias one, pointing at `docs/plugin-development/commands.md` for the field list. `docs/plugin-development/commands.md`: DONE, the three fields in the `CommandDecl` table plus the four refusals |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | No BGP wire change |
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior? | N-A | Nothing here implements an RFC obligation |
| 10 | Test infrastructure changed? | No | Both `.ci` files use the existing runner |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/commands.md` |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | No | |
| 15 | Registered command or capability changed? | Yes | DONE in `docs/architecture/api/process-protocol.md` and `docs/plugin-development/protocol.md`, which are the two pages that enumerate a Stage 1 declaration. `docs/features/plugins.md` and `docs/plugin-overview.md` are UNAFFECTED: the first lists plugins by name and the second names Stage 1 by its RPC, and neither says what a `CommandDecl` carries |
| 16 | Changed source file referenced by doc source anchors? | DERIVED | Run `./le spec citation anchors spec plan/spec-plugin-declares-answer-shape.md` at the start of each phase. Two declared docs are UNAFFECTED. `docs/architecture/core-design.md`, declared by `rs/server.go`, describes the route server's place in the engine; that file gains two declarations stating what its existing answers already hold, and no behavior the doc records changes. `docs/architecture/plugin/rib-storage-design.md`, declared by `adj_rib_in/rib.go`, describes Adj-RIB-In raw hex storage; that file gains two declarations and stores nothing differently |
| 17 | Existing docs show examples for this area? | Yes | The two Stage 1 wire examples in `docs/architecture/api/process-protocol.md` show a `commands` list with names alone. They stay CORRECT, because the three fields are optional and a plugin that sends none is the case they show. The Go example a plugin author copies is in `docs/plugin-development/commands.md` and in `docs/guide/plugins.md`, and both now carry a declared shape |
| 18 | Anchored docs the checklist omitted | DERIVED | `spec_doc_anchors.py` names sixteen. Four were judged this phase. `docs/plugin-development/protocol.md`: AFFECTED, it enumerates the Stage 1 field set, so it gains a `CommandDecl` field table. `docs/guide/healthcheck.md` and `docs/guide/rpki.md`: AFFECTED, each shows the command whose answer changed shape. `docs/architecture/api/architecture.md`, `docs/architecture/api/wire-format.md` and `docs/plugin-development/handlers.md`: UNAFFECTED. The first anchors the 5-stage list and the transport table, the second anchors the kebab-case JSON key convention that `address-fields` obeys, and the third anchors the handler input and output types |

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
| A bad declaration REFUSES the plugin; it never panics | Reuse the in-tree panic | A plugin message is external input. `docs/contributing/ze-go-style.md` reserves `panic("BUG:")` for a state only a Ze defect can reach, and a plugin taking the daemon down is the failure mode this avoids |
| The eleven declarations live in each plugin's own registration | A table in the engine mapping command paths to shapes | A table in a central package is plugin spelling in a generic package, which `ai/rules/plugins.md` bans. Remove the plugin and its declaration MUST vanish with it |
| `show bgp rpki status` and `show bgp adj-rib-in status` declare `doc` | Reshape both so they carry one row set | Both are genuinely documents: one holds two candidate row sets, the other maps an address to a scalar. Reshaping them to please an operator would change an answer this spec has no reason to change |

## Known Limitations

- The published catalog SHOWS an in-tree plugin's declaration. This paragraph
  used to say it could not, and that was true until 0f991285e (2026-09-07,
  `spec-daemon-backed-command-catalog`) taught `ze help command --json` and
  `./le command list` to read `registry.Registration.Commands` and `.Pipes`,
  which `init()` sets from the same function the runner sends at Stage 1
  (`command.DeclaredForCommand`, `internal/component/command/declared.go`). The
  catalogs still start no plugin, and that is why the route is the registration
  rather than a running daemon. One case is genuinely out of reach: an EXTERNAL
  plugin registers nothing in the composition root, so its declaration reaches a
  running daemon alone, through `show command help "<name>"` and Tab completion
  (`docs/features/introspection.md`).
- The engine cannot check a declared column name against a payload it has not
  seen. For the eleven commands the `.ci` checks it; for a third-party plugin it
  stays the author's responsibility.
- `show bgp adj-rib-in` declares `map` and its rows are its peers, each row
  being that peer's routes (ruled 2026-09-06; see AC-16). It declares NO column
  order, because a column name orders the keys of a row and this row is a list
  whose keys sit one level below it. `| resolve` stays refused: the peer address
  is the map KEY rather than a field, so no address field is declared, which is
  the identity-keyed row set limitation carried by the retired deferral shard
  "cli-show-bgp-answer-shapes".

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-18 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests N-A: nothing wire-visible changes

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-plugin-declares-answer-shape.md` only

## Implementation Summary

### What Was Implemented
- `CommandDecl` (`pkg/plugin/rpc/types.go`) carries `Shape`, `Columns` and
  `AddressFields`, all optional, all kebab-case on the wire.
- `ParseAnswerShape` (`internal/component/command/pipe_catalog.go`) reads the
  wire spelling back, off the one `allShapes` list `String`, `anyShape` and
  `Shapes` also read.
- `validateShapeDecls` and `validateDeclaredFieldName`
  (`internal/component/plugin/server/startup.go`) refuse a bad declaration
  before anything is stored, and `clampDeclared` bounds every plugin string that
  reaches the daemon log.
- `registerPluginShapes` writes under `startupRegistrationMu`, between the pipe
  aliases and the runtime families, and joins the same unwind.
  `releasePluginRegistrations` (`restart.go`) takes the declaration back when
  the plugin stops.
- `declarationRegistry.declareFor` and `.withdraw`
  (`internal/component/command/column_order.go`) are the socket-facing write:
  the conflict `declare` panics on is an error here, and what a path held before
  an owner wrote is recorded so `withdraw` can put it back.
- `RegisterPluginShapes` and `UnregisterPluginShapes`
  (`internal/component/command/answer_shape.go`) write all three registries
  together and unwind on the first error.
- Eleven commands declare, each in its own plugin's registration: six in
  `rpki/rpki.go`, two in `rs/server.go`, two in `adj_rib_in/rib.go`, one in
  `healthcheck/healthcheck.go`.
- `handleShow` (healthcheck) and `aspaCommand` (rpki) answer a row set whichever
  argument they take, and both list producers sort their keys so a row selector
  picks the same row on every call.
- `rowSet` (`answer_shape.go`) reads an identity map whose values are LISTS as
  rows, guarded by `identityValuesShareOneShape`. That is what makes
  `show bgp adj-rib-in | first 1` answer one peer's routes.

### Bugs Found/Fixed
- A declared column or address-field name reached the operator's terminal with
  no control-character refusal. `completeDisplayFields`
  (`internal/component/command/completer.go`) offers each declared column as a
  `| display` candidate and the table renderer writes it as a header, and
  `normalizeCommand` collapses only the whitespace control characters, so an ESC
  in a plugin's declared name wrote an ANSI sequence to the terminal.
  `validateDeclaredFieldName` now runs `validateDeclaredText`, the same check a
  command's `description` gets. Covered by two new cases in
  `TestValidateShapeDecls`.
- `show bgp rs peers` and `show bgp healthcheck` answered in Go map iteration
  order, so `| first 1` picked a different row on each call. Both producers sort
  now, and `comparePeerAddress` (`rs/server_handlers.go`) is total for whatever
  the engine sent.

### Documentation Updates
- `docs/architecture/api/commands.md`: the shape channel beside the alias
  channel, and the identity-map row rule.
- `docs/architecture/api/ipc_protocol.md`,
  `docs/architecture/api/process-protocol.md`,
  `docs/plugin-development/protocol.md`: the three new Stage 1 fields.
- `docs/plugin-development/commands.md`, `docs/guide/plugins.md`: what a plugin
  author writes, and the refusals.
- `docs/architecture/bgp/healthcheck-plugin.md`, `docs/guide/healthcheck.md`:
  both spellings answer a row set, the command declares `map`, `| fill` is
  refused by name.
- `docs/guide/rpki.md`: the aspa lookup answers rows under `entries`, with
  `found` beside them.
- `docs/features/cli-commands.md`, `docs/guide/command-reference.md`.
- `ai/rules/plugins.md`, rendered from six points under
  `ai/rules/points/plugins/answer-shape-declaration-stage-1-wire-protocol/`.
  Those point files were later deleted by 9ee958b7f (2026-08-30, "collapse 1110
  directives onto ten principles"), which removed 1153 point files corpus-wide.
  The mechanism survives at `docs/architecture/api/commands.md`, which
  `ai/rules/plugins.md` routes to by name for answer-shape declaration.
- `./le doc check verify` NOT RUN. It exits 1 with 3939 findings at HEAD, none
  on this spec's pages, and re-running it reconfirms a result already read
  (`ai/rules/pre-release.md`).

### Deviations from Plan
- `show bgp healthcheck` declares `map`, not the `tab` the Current Behavior
  table read, and declares no column order. The two branches carry DIFFERENT row
  fields on purpose, three for the probe list and ten for one named probe, and
  one column order cannot be read against both. `| display name state` still
  works, because `display` is admitted by `rowShapes`
  (`internal/component/command/pipe_catalog.go`), which holds `ShapeMap` and
  `ShapeTab`. So User Story 2's behavior holds and only its "declared order"
  path description is wrong. `| fill` is refused by name, which is the one
  operator the missing column order costs.
- `show bgp adj-rib-in` declares `map` and its rows are lists, ruled 2026-09-06
  under AC-16. The Current Behavior row calling it `tab` stays wrong, for the
  reason A-5 records.
- `TestAspaLookupAnswersRows` lives in
  `internal/component/bgp/plugins/rpki/rpki_commands_test.go`, not the
  `rpki_test.go` the TDD plan named.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-5 read `show bgp adj-rib-in` as a `tab` answer whose rows were peers | `rowSet` read a map as rows only when every value was an OBJECT, and the peer map's values are arrays, so the command held no row set at all | Reading `rowSet` against `AdjRIBInManager.show` | The READER was taught the shape rather than the payload reshaped (c87097bc6). `identityValuesShareOneShape` keeps a mixed map one document |
| approach | The Security Review row claimed every declared string was validated "against a closed set or a bound BEFORE it is stored", and the bound was the only half that existed | A declared field name reaches the operator's terminal, so it is a declared TEXT and owes a control-character refusal too | The review gate's guard audit, tracing a declared column to `completeDisplayFields` | `validateDeclaredFieldName` now calls `validateDeclaredText`, and `TestValidateShapeDecls` gained the escape and tab cases |
| escalation | The bound-and-clean rule for a plugin's declared text (`ai/rules/plugins.md`) was written for `description` and `long-help` in e691533a6, a week after this channel shipped, and nothing carried it back to the field names this channel had already added | A rule written for one declared string binds every declared string on the same surface | The same guard audit | Journal row in `plan/journal/rule-written-after-the-surface-it-binds.md` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A plugin command can declare what its answer holds | Done | `pkg/plugin/rpc/types.go`, `CommandDecl.Shape`, `.Columns`, `.AddressFields` | Three optional fields; absent means undeclared |
| `validateDeclaredShape` no longer returns early for a plugin command | Done | `internal/component/command/pipe.go`, reached through `shapeRegistry` | A declared path publishes its operators and refuses the rest before dispatch |
| The operators a plugin command supports are published | Done | `cmd/ze/help_command.go`, `collectCommands` | Satisfied by 0f991285e, which reads `registry.Registration.Commands` |
| An operator a plugin command cannot support is refused before dispatch | Done | `test/ui/show-bgp-plugin-shapes.ci`, on the string "cannot apply here" | Asserted for eight command-and-operator pairs |
| `\| display <partial>` completes on a plugin command | Done | `internal/component/command/completer.go`, `completeDisplayFields` | Reads `ColumnsForCommand`, which the plugin declaration now writes |
| The eleven measured commands declare | Done | four `commandDecls` functions | Six rpki, two rs, two adj-rib-in, one healthcheck |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRegisterPluginShapes` | All three registries written from one Stage 1 message |
| AC-2 | Done | `TestValidateShapeDecls` cases "a spelling no shape writes" and "a capitalized spelling"; `test/plugin/plugin-shape-declaration-refused.ci` | The `.ci` proves the plugin does not start and the daemon keeps running |
| AC-3 | Done | `TestValidateShapeDecls` cases "a column order with no shape" and "an address-field list with no shape" | |
| AC-4 | Done | `TestValidateShapeDecls` case "a shape on a path the message does not declare" | The shape is a FIELD on the `CommandDecl`, so declaring for a foreign path is unwritable; the nameless-path refusal is the reachable form |
| AC-5 | Done | `TestValidateShapeDecls`, six bound cases plus the two new control-character cases | 64 columns, 16 address fields, names of 1 to 64 bytes |
| AC-6 | Done | `TestPluginShapeOverridesEmptyDeclaration` | The floor rule in `declareFor` |
| AC-7 | Done | `TestUnregisterPluginShapes` | The path returns to the EMPTY declaration, and a restart declares again |
| AC-8 | Done | `TestShapeWriteUnwindsWithStageOne` | A family conflict after the shape write leaves nothing behind |
| AC-9 | Done | `ui_fixture_show_bgp_plugin_shapes.go`, the `\| display address state` block | Two rows, exactly the two fields |
| AC-10 | Done | same fixture, the `\| resolve` block | Decorates `address` with TEST-NET reverse lookups that never answer |
| AC-10b | Done | same fixture, the refusal rows for `show bgp rpki summary \| resolve` and `show bgp rs status \| resolve` | Each judged on its own declared list |
| AC-11 | Done | refusal row `show bgp rpki summary \| first 2` | |
| AC-12 | Done | refusal row `show bgp rpki status \| count` | `TestDocCommandsHoldNoSingleRowSet` pins the two candidate keys |
| AC-13 | Done | fixture `show bgp rs peers \| count` | Answers zero over an empty peer set, which is the count and not a refusal |
| AC-14 | Done | `TestHealthcheckNamedProbeAnswersRows` | Every field the list branch writes is spelled the same in the named branch |
| AC-15 | Done | `TestAspaLookupAnswersRows` | `jsonKeys(dumpRow)` equals `jsonKeys(lookupRow)` |
| AC-16 | Done | `TestApplyTakeKeepsIdentityKeysOverArrayRows`, `TestShowAnswersRowsKeyedByPeer`, `TestShowCountsThePeers`, and the fixture's `\| first 1` block | Ruled 2026-09-06: the row reader was taught, the payload is unchanged |
| AC-17 | Done, with one deviation | the four `commandDecls` functions; `TestDeclaredColumnsExistInPayload` in `rpki` and `rs` | `show bgp healthcheck` declares no column order: its two branches carry different row field sets, so no single order can be read against both. Recorded in Deviations |
| AC-18 | Done | 0f991285e, `cmd/ze/help_command.go` `collectCommands`; `cmd/ze/help_command_plugin_test.go` | `cmd/ze` built at `ze_core,ze_bgp` answers, for `show bgp rpki roa`, 17 operators plus answer-shape `tab` and column-orders `[[prefix, max-length, asn]]`; `show bgp rpki` answers 17 plus the plugin-declared alias `summary`; `show bgp adj-rib-in` answers 14 plus answer-shape `map`. Before that commit the catalog held 270 commands and none of the three; it now holds 313 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestParseAnswerShape` | Done | `internal/component/command/pipe_catalog_test.go` | |
| `TestValidateShapeDecls` | Done | `internal/component/plugin/server/startup_test.go` | 18 cases, two added by this closure |
| `TestRegisterPluginShapes` | Done | same file | |
| `TestPluginShapeOverridesEmptyDeclaration` | Done | same file | |
| `TestUnregisterPluginShapes` | Done | same file | |
| `TestShapeWriteUnwindsWithStageOne` | Done | same file | |
| `TestHealthcheckNamedProbeAnswersRows` | Done | `internal/component/bgp/plugins/healthcheck/healthcheck_test.go` | |
| `TestAspaLookupAnswersRows` | Changed | `internal/component/bgp/plugins/rpki/rpki_commands_test.go` | The plan named `rpki_test.go`; recorded in Deviations |
| `show-bgp-plugin-shapes` | Done | `test/ui/show-bgp-plugin-shapes.ci` | |
| `plugin-shape-declaration-refused` | Done | `test/plugin/plugin-shape-declaration-refused.ci` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `pkg/plugin/rpc/types.go` | Done | |
| `pkg/plugin/sdk/sdk_types.go` | Done | `sdk.CommandDecl` carries the three fields |
| `internal/component/plugin/server/startup.go` | Done | Plus `restart.go` for the plugin-stop path, which the plan did not name |
| `internal/component/command/answer_shape.go` | Done | |
| `internal/component/command/pipe_catalog.go` | Done | |
| `internal/component/command/column_order.go` | Changed | The plan did not name it; `declareFor` and `withdraw` had to live beside `declare` |
| `internal/component/bgp/plugins/rpki/rpki.go` | Done | |
| `internal/component/bgp/plugins/rs/server.go` | Done | Plus `server_handlers.go` for the sorted peer order |
| `internal/component/bgp/plugins/adj_rib_in/rib.go` | Done | |
| `internal/component/bgp/plugins/healthcheck/healthcheck.go` | Done | |
| every documentation row | Done | Listed under Documentation Updates |
| `ai/rules/points/plugins/answer-shape-.../` | Changed | Written at 77e42aeab, deleted corpus-wide at 9ee958b7f; the mechanism lives at `docs/architecture/api/commands.md` |
| `test/ui/show-bgp-plugin-shapes.ci` | Done | |
| `test/plugin/plugin-shape-declaration-refused.ci` | Done | |

### Audit Summary
- **Total items:** 48 (6 requirements, 18 ACs, 10 tests, 14 file rows)
- **Done:** 44
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4, each in Deviations: the healthcheck column order, the
  `TestAspaLookupAnswersRows` location, `column_order.go` being unplanned, and
  the rule points a later corpus refactor deleted

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A command served by a plugin process can declare what its answer holds | functional | `test/ui/show-bgp-plugin-shapes.ci` starts four plugins in-process and asserts eight refusals on the string "cannot apply here", which only `validateDeclaredShape` writes. The file's header states why asserting on the operator name alone would be vacuous: the post-dispatch refusal shares those substrings |
| The operators a plugin command supports are published | functional | `cmd/ze/help_command_plugin_test.go`, and the measured catalog: `show bgp rpki roa` answers 17 operators plus `answer-shape: tab` and `column-orders: [[prefix, max-length, asn]]`. The catalog went from 270 commands with none plugin-contributed to 313 |
| An operator a plugin command cannot support is refused BEFORE dispatch | functional | the refusal table in `ui_fixture_show_bgp_plugin_shapes.go`: `show bgp rpki summary \| first 2`, `show bgp rpki status \| count`, `show bgp rs status \| count`, `show bgp adj-rib-in status \| count`, and four `\| resolve` rows |
| `\| display <partial>` completes on a plugin command | functional and unit | `completeDisplayFields` reads `ColumnsForCommand`, which `registerPluginShapes` writes; `TestRegisterPluginShapes` proves the write through the real Stage 1 path, and the fixture's `\| display address state` block proves the selection end to end |
| A bad declaration refuses the plugin and never panics the daemon | functional | `test/plugin/plugin-shape-declaration-refused.ci`: the plugin spells its shape `table`, does not start, the daemon log names the plugin, the command and the value, and the daemon still answers |
| A plugin's declarations leave with the plugin | unit, driven from Stage 1 | `TestUnregisterPluginShapes`: start, assert `tab`, `rollbackStartupProcess`, assert the path returns to the EMPTY declaration and not to nothing, then start again and assert `tab` returns |
| Interop | N-A | Nothing wire-visible changes. The plugin RPC contract changes additively and is not a protocol Ze speaks to another implementation |

## Work Not Done

An EXTERNAL plugin's declaration is absent from this table on purpose. It was
never in this spec's scope, Required Reading says so, and it is recorded under
Known Limitations with the producer that decides it. `spec-daemon-backed-command-catalog`
did the in-tree half and closed on 2026-09-08; the external half is a property of
where an external plugin registers, not work this spec left.

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| `\| resolve` over an identity-keyed row set, where the address is the map KEY rather than a field | An address-field list names a field of a row, and `show bgp adj-rib-in`'s address is the row's identity | `plan/immediate/spec-show-bgp-operators-over-identity-keyed-rows.md` |
| One name for the peer address across the `show bgp` answers | Out of this spec's subject: it declares what the answers hold, it does not rename their fields | `plan/immediate/spec-show-bgp-one-name-for-the-peer-address.md` |
| A plugin declaring on a command path it does not serve | The pairing makes it unwritable today, so nothing enforces it if the pairing ever loosens | `plan/spec-plugin-declaration-names-a-path-it-serves.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/plugin-declares-answer-shape-d64e7b3f-bfdc-4614-8db4-f12043eb77cc.md`, 12 files, verdict clean |
| `./le spec session review check` | `review_gate: OK (2 code files, clean, hashes match ...)` |
| Rounds | 2. Round 1 found the control-character ISSUE; round 2 read the fix and found nothing |
| Reviewer lenses used | wiring and functional-test coverage; guard audit and security over the strings a plugin declares; removed-behavior audit over the `rowSet` widening; the style pass over every changed Go file |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | A declared column or address-field name was bounded but never checked for control characters, and it reaches the operator's terminal: `completeDisplayFields` offers each declared column as a `\| display` candidate and the table renderer writes it as a header. `normalizeCommand` collapses only the whitespace control characters, so an ESC survived into the terminal. `ai/rules/plugins.md` requires the refusal for every declared text that reaches an operator | `internal/component/plugin/server/startup.go`, `validateDeclaredFieldName` | It now calls `validateDeclaredText(name, maxNameLen, textOneLine)`, the check `description` gets, replacing its own length check so one bound is stated once. `TestValidateShapeDecls` gained "a column name carrying an escape" and "an address-field name carrying a tab" |

Two NOTEs were recorded and did not block: the healthcheck `map` deviation, and
`TestAspaLookupAnswersRows` living in a file the TDD plan did not name. Both are
in Deviations.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/ui/show-bgp-plugin-shapes.ci` | Yes | `-rw-rw-r-- 1 thomas thomas 2444 Sep 6 09:53 test/ui/show-bgp-plugin-shapes.ci` |
| `test/plugin/plugin-shape-declaration-refused.ci` | Yes | `-rw-rw-r-- 1 thomas thomas 1589 Aug 30 22:52 test/plugin/plugin-shape-declaration-refused.ci` |
| `internal/test/fixture/ui_fixture_show_bgp_plugin_shapes.go` | Yes | the native fixture both `.ci` files name; `git grep -ln 'show-bgp-plugin-shapes' internal/` finds it |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 to AC-8 | The Stage 1 channel validates, writes, unwinds and withdraws | `go test -mod=mod -tags <feature-gates> -run Shape ./internal/component/plugin/server/`: `TestValidateShapeDecls`, `TestValidateShapeDeclsClampsTheValueItReports`, `TestRegisterPluginShapes`, `TestPluginShapeDoesNotInheritItsParentsFields`, `TestPluginShapeOverridesEmptyDeclaration`, `TestUnregisterPluginShapes`, `TestShapeWriteUnwindsWithStageOne` and `TestOnRegistrationRefusesConflictingShapeDeclaration` all PASS |
| AC-5 | A control character in a declared name is refused | `TestValidateShapeDecls/a_column_name_carrying_an_escape` and `/an_address-field_name_carrying_a_tab` PASS after the fix. Before it, `validateDeclaredFieldName` held only an empty check and a length check, so both cases returned nil and `require.Error` failed |
| AC-14, AC-15 | Both argument branches answer a row set | `ok internal/component/bgp/plugins/healthcheck 3.677s`, `ok internal/component/bgp/plugins/rpki 1.254s` |
| AC-16 | `\| first 1` answers one peer's routes | `ok internal/component/bgp/plugins/adj_rib_in 0.237s`, `ok internal/component/command 2.517s` |
| AC-17 | Eleven commands declare | `grep -rn 'Shape:' internal/component/bgp/plugins/{rpki,rs,adj_rib_in,healthcheck}`: six in `rpki.go`, two in `rs/server.go`, two in `adj_rib_in/rib.go`, one in `healthcheck/healthcheck.go` |
| AC-18 | The catalog lists the operators | 0f991285e, verified at the producer: `command.DeclaredForCommand` (`internal/component/command/declared.go`) reads `registry.Registration.Commands` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A plugin sends a Stage 1 message carrying a shape | `test/plugin/plugin-shape-declaration-refused.ci` | Yes: the file drives `ze-test fixture plugin/shape-declaration-refused`, which starts a plugin whose only fault is the spelling `table` |
| A plugin sends an unknown shape spelling | same | Yes: the daemon log names the plugin, the command and the value, and the daemon still answers |
| A plugin stops | `TestUnregisterPluginShapes` | Yes: `rollbackStartupProcess` on the real process, then a second `runPluginPhase` |
| `show bgp rpki cache \| display address state` | `test/ui/show-bgp-plugin-shapes.ci` | Yes: the fixture asserts two rows and exactly the two fields |
| `show bgp rs peers \| resolve` | same | Yes: the refusal table judges `show bgp rs status \| resolve` on its own declared list, and the `\| resolve` block decorates `address` on the rpki cache rows |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestPluginShapeOverridesEmptyDeclaration` PASSES; the floor rule is the `case r.isEmpty(held)` arm of `declareFor` |
| A-2 | confirmed 2026-08-24 | `gopls references` on `handleShow` and `aspaCommand`; one `.ci` reads the named-probe answer and matches substrings the new sequence indent preserves |
| A-3 | confirmed 2026-09-08 | `TestUnregisterPluginShapes` restarts the plugin and asserts the shape is declared again |
| A-4 | confirmed 2026-08-24 | `TestDocCommandsHoldNoSingleRowSet`, `TestStatusHoldsNoRowSet`, and the fixture's refusal table |
| A-5 | broken 2026-08-24, repaired 2026-09-06, now holds | `rowSet` reads an identity map of lists as rows; `identityValuesShareOneShape` keeps a mixed map one document. Mistake Log row 1 |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/features/introspection.md` says the catalogs reach an in-tree plugin's declaration | `cmd/ze/help_command.go` `collectCommands`, `internal/component/command/declared.go` `DeclaredForCommand` | Yes, and it carries both source anchors |
| `docs/guide/healthcheck.md` says both spellings answer a row set and the command declares `map` | `healthcheck.go` `handleShow`, `commandDecls` | Yes, anchored on both symbols |
| `docs/guide/rpki.md` says the aspa lookup answers rows under `entries` with `found` beside them | `rpki.go` `aspaCommand`, and `TestAspaLookupAnswersRows` asserting `jsonKeys` equality | Yes, anchored on `aspaCommand` |
| `docs/architecture/api/commands.md` carries the shape channel | 29 occurrences of the shape vocabulary; `ai/rules/plugins.md` routes to it by name | Yes |
| The spec's own Known Limitations paragraph claiming the catalog "cannot show a plugin's declaration" | 0f991285e | Corrected in this closure. It had been FALSE since 2026-09-07 |

## Core Insight

The ARITY of the declared value decides how two declaration channels answer the
same collision. A set of aliases has a union, so a plugin's names merge with what
the path holds and the merge happens at the call site. A shape and a column order
have no union, so the answer has to be a floor rule inside the registry:
`declareFor` lets a real value replace an empty declaration, lets nothing replace
a real one, and records what the path held so `withdraw` can put it back. The
empty declaration is not an absence. It is a BARRIER an in-tree package wrote to
stop a child inheriting its parent's shape, and reading it as an absence puts a
plugin declaration on the wrong path in either direction.
