# Spec: plugin-registers-pipe-operations

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 6/6 |
| Deferral shard | `plan/deferrals/plugin-registers-pipe-operations.md` |
| Handoff | - |
| Updated | 2026-08-22 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An in-tree command can name a pipe alias for itself. `show bgp | summary` renders
the aggregate half of the `show bgp` answer, because the BGP peer command plugin
calls `command.RegisterAliases` from its `init()`.

An external plugin cannot. It declares its commands over the 5-stage startup
protocol with `sdk.Registration` and `sdk.CommandDecl`, and that message carries
no field for a pipe alias. The alias registry is reachable from Go code compiled
into the daemon and from nowhere else.

The result is that a plugin has one way to offer a second view of the same data,
which is to declare a second command. The RPKI plugin did exactly that:
`show bgp rpki status` and `show bgp rpki summary` are two commands over one set
of counters. The second should be `show bgp rpki | summary`.

The goal is a declaration channel that lets a plugin name a pipe alias for its
own commands, with the collision, scope and wire-form decisions the owner has
already made, and one converted consumer that proves the channel works end to
end.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/api/commands.md` - the pipe operator framework, the three
      command-path registries, and the four properties that make one-pass alias
      expansion sound
  → Decision: an alias resolves by the longest registered command path that is a
    prefix of the command. A registration on a longer path shadows one on a
    shorter path for its whole subtree, and that shadowing is the intended
    mechanism rather than a conflict.
  → Constraint: an alias MUST NOT name another alias, MUST NOT carry the name of
    a built-in pipe operator, MUST NOT carry the name of a pipe filter on an
    overlapping command path, and MUST take no argument.
  → Constraint: selection reaches every output format. Sequence reaches
    `| table` and `| text` alone.

- [ ] `docs/architecture/api/process-protocol.md` - the 5-stage startup protocol
      and what Stage 1 carries
  → Decision: Stage 1 is where a plugin declares every static fact about itself.
    Families, commands, filters, doctor checks, enrichers and the YANG schema all
    arrive in one `declare-registration` message.
  → Constraint: a Stage 1 handler returns an error whose text the driver relays
    to the plugin verbatim, and the failed process is torn down after the tier
    handshake completes.

- [ ] `docs/architecture/api/ipc_protocol.md` - the RPC type definitions the SDK
      re-exports
  → Constraint: a new declaration field is a wire change. It needs the Go type,
    the YANG description of the RPC input, and the Python test client.

- [ ] `ai/rules/cli.md` - pipe completeness and payload structure
  → Constraint: a command's response payload MUST be structured data, never text
    a renderer already formatted. `| json`, `| yaml` and `| table` are three
    renderings of one payload.
  → Constraint: every command that produces output MUST support all pipe
    operators, and MUST route its output through `ApplyPipes` or a
    `ProcessPipes*` wrapper.

- [ ] `ai/rules/plugins.md` - registration over hardcoding, and the 5-stage
      protocol summary
  → Constraint: remove the plugin and all its features vanish. A plugin's pipe
    alias MUST leave the registry when the plugin leaves.

**Key insights:** (minimal context to resume after compaction)

- The pipe chain runs in the DAEMON for every command path that matters. The SSH
  exec handler splits the chain off the input and applies it to the dispatcher's
  answer, and the interactive CLI is a Bubble Tea model the SSH server hosts. The
  registry a plugin writes to is therefore in the process that reads it.
- The one exception is `cliClient.StreamMonitor`, which resolves pipes in the
  client process for streaming monitor commands. A plugin's alias is unknown
  there.
- A pipe alias is pure reshaping. It cannot rename a key, add two numbers, or
  count the rows that match a predicate.
- `show bgp rpki` is already listed in `cmdBgpChildren` and already carries an
  empty alias declaration, put there by the in-tree BGP peer command plugin to
  stop `| summary` and `| peers` leaking down from `show bgp`.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)

- [ ] `internal/component/command/alias.go` - `Alias`, `RegisterAliases`,
      `checkedAlias`, `lookupAlias`, `AliasesForCommand`, `aliasSuggestions`,
      `filterShadowing`, `aliasShadowing`, `pathsOverlap`. Registration parses the
      expansion once and stores the resulting operators. Four bad registrations
      are refused with `panic("BUG:")`, on the stated premise that only code in
      this repository reaches the function and no operator input does.
- [ ] `internal/component/command/column_order.go` - `ColumnOrder`,
      `RegisterColumns`, `ColumnsForCommand`, and the generic `commandRegistry`
      the alias, column and filter registries all share. `register` stores one
      value per normalized path and REPLACES what that path held.
      `lookup` returns the value on the longest registered prefix, with a word
      boundary so `show bgp rib` does not match `show bgp ribbon`. There is no
      unregister, only `reset` for tests.
- [ ] `internal/component/command/pipe.go` - `knownPipeOps`, `ParsePipe`,
      `parsePipeOps`, `parsePipeChain`, `expandAliases`, `foldFilters`,
      `ApplyPipes`, and the `ProcessPipes*` wrappers. Sixteen built-in operator
      names. `expandAliases` runs between parsing and filter folding, in one pass,
      and refuses a word after an alias name.
- [ ] `internal/component/command/pipe_filter.go` - `PipeFilter`,
      `RegisterPipeFilters`, `lookupPipeFilters`, `PipeFiltersForCommand`. A
      filter is folded into a command ARGUMENT and runs in the handler, at the
      source of the data.
- [ ] `internal/component/command/pipe_table.go` - `tableStyle.orderKeys`,
      `declaredKeys`, `bestColumnOrder`, `splitByOrder`. Ordering never adds a
      column and never drops one.
- [ ] `internal/component/command/completer.go` - `completePipeForCommand`,
      `pipeExtras`, `completePipe`, `completeDisplayFields`. Alias names and
      filter names are offered together in the operator slot.
      `| display` argument completion reads the column registry.
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` - `cmdBgpChildren`,
      `registerColumns`, `registerAliases`. The only caller of `RegisterAliases`
      in the tree. It declares `summary` and `peers` on `show bgp`, then declares
      every direct child of `show bgp` as carrying no alias and no column order,
      to stop the whole subtree inheriting them. `show bgp rpki` is one of those
      children.
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` - the Stage 1 registration
      with six `sdk.CommandDecl` entries and no bare `show bgp rpki`, the
      `handleCommand` switch, `statusCommand`, `summaryCommand`, `roaCommand`,
      `aspaCommand`.
- [ ] `internal/component/plugin/server/startup.go` - `engineStartupSink.onRegistration`,
      `registrationFromRPC`, `registerPluginFamilies`. Validation runs before
      conversion, family registration is rolled back with the registry row when it
      fails, and both run under `startupRegistrationMu`.
- [ ] `pkg/plugin/rpc/types.go` - `DeclareRegistrationInput`, `CommandDecl`,
      `EnricherDecl`, `FilterDecl`, `DoctorCheckDecl`. Each optional declaration
      is its own list on the registration input.
- [ ] `pkg/plugin/sdk/sdk_types.go` - the SDK aliases that re-export those types
      so an external author imports one package.
- [ ] `internal/plugins/meta/cmd/help.go` - `handleBgpCommandHelp`,
      `pipeFilterHelp`. Help reports a command's pipe FILTERS. It reports no
      aliases at all.
- [ ] `internal/le/command/list/commandlist.go` - the source of `./le command list`. It
      is a `go run` program with a blank import of `internal/component/plugin/all`,
      so it sees the in-process registrations of the compiled tree and starts no
      external plugin.
- [ ] `cmd/ze/help_command.go` - `collectCommands`, `extractPipes`. The source of
      `ze help command --json`, which `./le wiki-catalog update` pipes into the
      wiki. It reads the YANG command tree and the local command registry in its
      own process, and reports a command's pipe FILTERS from the in-process
      registry. It reaches no running daemon.
- [ ] `test/scripts/ze_api.py` (retired, no successor) <!-- doc-links: ignore (deleted 2026-08-28 by eae282592 with no replacement) --> - the Python plugin client the `.ci` suite drives,
      with `declare_enricher` as the model for a new Stage 1 declaration.

**Behavior to preserve:**

- `show bgp | summary` and `show bgp | peers` keep their current meaning and their
  current expansions.
- Every path below `show bgp` keeps carrying no inherited alias, which is what
  `registerAliases(cmdBgpChildren)` buys today.
- `show bgp rpki status`, `show bgp rpki cache`, `show bgp rpki roa`,
  `show bgp rpki summary`, `show bgp rpki aspa` and `request bgp rpki validate`
  keep working and keep their current payloads. Removing a working command is not
  part of this change.
- `RegisterAliases` keeps refusing an in-tree registration with `panic("BUG:")`.
  That premise is true for in-tree callers and the panic is the right answer for a
  fault the compiler cannot see.
- A built-in operator name always outranks an alias, and a pipe filter on an
  overlapping path always outranks an alias. Both are resolution rules the parser
  and `foldFilters` implement, and neither changes.
- The longest registered command path wins. A registration on a longer path
  shadows one on a shorter path for its subtree.

**Behavior to change:**

- Stage 1 accepts a list of pipe alias declarations from a plugin, validates them,
  and registers the valid set into the alias registry.
- A bad declaration fails the plugin's whole Stage 1 registration with a relayed
  error, instead of reaching a panic.
- The alias registry gains removal by plugin name, so a plugin's aliases leave
  with the plugin.
- `command help "<name>"` reports the pipe aliases a command answers to, beside
  the pipe filters it already reports.
- The RPKI plugin declares a bare `show bgp rpki` command whose payload carries
  the aggregate fields and the detail as siblings, and declares `summary` as a
  pipe alias over it. CORRECTED in phase 5: this line said `summary` and `cache`.
  `cache` cannot be an alias. `cacheCommand` reports `preference`, `session-id`,
  `serial` and three intervals that the bare payload does not carry, so a `cache`
  alias would answer a DIFFERENT record from the `cache` subcommand under one
  word. `show bgp rpki cache` stays a subcommand.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

- A plugin process sends `ze-plugin-engine:declare-registration` over Socket A
  during Stage 1 of the startup handshake. The message is JSON. The new
  declaration is a `pipes` list on that message, and each entry carries four
  strings: the command path, the alias name, the description, and the expansion.
- An operator later types a command with a pipe chain into the SSH exec channel
  or the interactive CLI. That text is the second entry point, and it reaches the
  daemon with its pipe characters intact.

### Transformation Path

1. `startup_driver.go` decodes the Stage 1 frame into `rpc.DeclareRegistrationInput`
   and calls `engineStartupSink.onRegistration`.
2. `onRegistration` validates the pipe declarations before it converts anything,
   in the same position where it validates doctor checks and enrichers today. A
   refusal returns an error and nothing is registered.
3. Under `startupRegistrationMu`, beside the registry row and the family
   registration, the engine writes the accepted aliases into `aliasRegistry`,
   keyed by the normalized command path, and records the plugin name that owns
   them. It also writes a derived empty declaration on every other command the
   plugin declared that sits strictly below one of those paths.
4. A failure at any later point of the same Stage 1 rolls the pipe registrations
   back with the registry row and the families.
5. At command time, the SSH exec handler calls
   `command.ProcessPipesDefaultFormatChecked`, which calls `parsePipeChain`.
   `ParsePipe` splits the command from the chain, and `expandAliases` replaces
   the plugin's alias name with the operator chain it stands for.
6. `foldFilters` classifies what is left. The plugin's expansion has already
   become built-in operators, so it takes the default arm and stays in the chain.
7. The dispatcher runs the command, the plugin answers with its payload, and
   `ApplyPipes` runs the chain over that payload with the command's declared
   column orders.
8. When the plugin stops or its startup is rolled back, the engine removes every
   alias it owns from the registry.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Plugin process to engine | The `pipes` list on the Stage 1 `declare-registration` JSON message | No |
| Engine to alias registry | The engine calls the registry's plugin-facing registration entry point, which returns an error rather than panicking | No |
| Operator to daemon | Command text with its pipe characters intact, over the SSH exec channel or the interactive CLI | No |
| Alias registry to renderer | `expandAliases` splices the stored operators into the chain `ApplyPipes` runs | No |

### Integration Points

- `rpc.DeclareRegistrationInput` gains one list. Every other Stage 1 declaration
  is shaped the same way, so this follows the pattern rather than adding one.
- `internal/core/ipc/yang/ze-plugin-engine.yang` describes the `declare-registration`
  input, so the new list is described there too.
- `pkg/plugin/sdk/sdk_types.go` re-exports the new type so an external author
  imports only the SDK.
- `test/scripts/ze_api.py` (retired, no successor) <!-- doc-links: ignore (deleted 2026-08-28 by eae282592 with no replacement) --> gains the matching declaration method, which is how the
  `.ci` suite drives an external plugin.
- `internal/component/command/alias.go` gains a plugin-facing registration entry
  point that returns an error, and removal by owner.
- `internal/plugins/meta/cmd/help.go` gains the alias listing beside the filter
  listing it already builds.

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | The declaration travels Stage 1 like every other plugin declaration, and the engine is the only writer to the registry |
| No unintended coupling (components stay isolated) | No | `internal/component/command` learns nothing about plugins. It gains an entry point that reports an error instead of panicking, and removal by an opaque owner string |
| No duplicated functionality (extends existing, does not recreate) | No | The expansion is parsed by `parsePipeOps`, the one reader of operator names. No second grammar is written |
| Zero-copy preserved where applicable (refs, not copies) | No | Not applicable. This is a startup-time declaration and a registry lookup, on no hot path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | The whole change is a registration channel. No name of any plugin appears in `internal/component/command` |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The pipe chain for a plugin command is resolved in the daemon process, so a registry the engine writes is the registry the resolver reads | `internal/component/ssh/ssh.go` `execMiddleware` calls `command.ProcessPipesDefaultFormatChecked` before dispatch, and the interactive CLI is a Bubble Tea model the SSH server hosts | The alias would have to be shipped to the client process as well, which is a much larger change | `test/plugin/plugin-pipe-alias.ci` typing the alias over the exec channel | confirmed. The `.ci` passes, and `ze cli -c` reaches `execMiddleware`. CORRECTED in phase 4: the daemon-hosted TUI holds too, and `ze cli` interactive does not |
| A-2 | `aliasRegistry.lookup` returns only the longest matching prefix and never falls back to a shorter one, so any registration on a path stops inheritance for that path and everything below it | `commandRegistry.lookup` and `lookupAlias` in `internal/component/command/column_order.go` and `internal/component/command/alias.go` | The derived barrier would be unnecessary, or a different barrier would be needed | `TestPluginAliasDoesNotLeakToSiblingLeaf` | confirmed. `commandRegistry.lookup` takes the longest matching prefix and returns it alone |
| A-3 | In-tree `init()` registration always completes before any external plugin reaches Stage 1 | Go initializes every imported package before `main` runs, and plugin startup is driven from `main` | An in-tree alias could lose to a plugin, which the collision rule assumes cannot happen | `TestRegisterPluginAliasesRefusesSameNameOnSamePath`, which is the test that was written for it, plus the startup order: in-tree `RegisterAliases` runs in `init()`, `loadBuiltinsWithAliases` runs in `NewServer`, and both complete before `runPluginPhase` | confirmed |
| A-4 | A pipe alias cannot reproduce the current `show bgp rpki summary` payload from the current `show bgp rpki status` payload | `summaryCommand` computes `vrp-count` as the sum of two counts, `sessions-established` as a count of rows matching a state, and spells `sessions-total` where `status` spells `sessions` | The RPKI conversion would be a pure alias with no payload work, and the payload obligation stated below would be unnecessary | Reading both handlers in `internal/component/bgp/plugins/rpki/rpki.go`, then `test/plugin/rpki-pipe-summary.ci` comparing both answers | confirmed. Four of the seven fields are computed, which is why phase 5 built the payload rather than writing a pure alias |
| A-5 | Stage 1 refusal tears the plugin process down and leaves no partial registration behind | `onRegistration` returns an error the driver relays, and the comment on it names `rollbackStartupProcess` | A refused plugin could leave aliases in the registry and block its own restart | `TestOnRegistrationRollsBackPipesOnLaterFailure`, which drives the unwind the `.ci` cannot reach, beside `test/plugin/plugin-pipe-alias-collision.ci` | confirmed | <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->
| A-6 | No pipe filter in the tree carries a name a converted consumer wants | `filterShadowing` refuses the pair, and `registerAliases` in `peer.go` records that no filter carries `summary` or `peers` today | A converted consumer would be refused and would need a different alias name | A grep of every `RegisterPipeFilters` call: the three in `internal/component/bgp/plugins/cmd/rib/rib.go` carry advertised, community, count, family, first, graph, histogram, last, match, path, peer, prefix, reason and received, and none of them is `summary` | confirmed |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | "Fail on collision" read as "any overlapping path" refuses the motivating consumer, because `show bgp` already carries `summary` and `pathsOverlap` makes every `show bgp *` path overlap it | `show bgp rpki | summary` is refused at registration during the first implementation phase | The collision rule below is written against the EXACT path for alias versus alias, and against overlapping paths for alias versus filter, because those are the two different resolution rules. The unit tests pin both readings |
| R-2 | A plugin registers an alias on a parent path and it leaks to every leaf below it that declares nothing, so `show bgp rpki roa \| summary` is offered over an answer with no aggregate fields | Completion offers the name on a leaf whose payload cannot answer it | The engine derives the empty barrier for the plugin's own leaves, so an author does not have to know the inheritance rule |
| R-3 | A plugin restart collides with its own previous registration, because nothing removes an alias when a plugin stops | The second start of a plugin fails where the first succeeded | Removal by owner is part of this spec, not a follow-up. The rollback path and the stop path both call it |
| R-4 | Two plugins want the same alias name on the same path, and which one fails depends on tier and startup order | An operator sees a plugin fail to start with a message naming another plugin | The refusal names both plugins, both paths and the name, so the operator can change one. Startup order dependence is accepted, because the alternative is silent replacement, which the owner refused |
| R-5 | An alias is registered for a command whose payload cannot answer it, so the operator gets an empty or whole record instead of the half they asked for | `| display` on a name the payload does not carry leaves the record whole, which reads as the alias doing nothing | The payload obligation below is an acceptance criterion of any consumer conversion, and the RPKI conversion proves it |
| R-6 | A plugin declares a pipe alias whose name a pipe filter later takes on an overlapping path, and the alias goes dark with nothing reporting it | No signal at all, which is why `aliasShadowing` exists | A plugin cannot declare a pipe filter today. If that channel is ever added, it runs the same overlap check from its side. CORRECTED by the Review Gate: no row was owed and none was written. Nothing is deferred here, because the channel does not exist and this spec does not add one. Known Limitations carries the obligation for whoever adds it |
| R-7 | `cliClient.StreamMonitor` resolves pipes in the client process, where a plugin's alias is unknown, so an alias typed on a streaming monitor command is reported as an unknown operator | An operator sees "unknown pipe operator" for a name completion offered | Scoped out below, with the reason. No plugin declares a streaming monitor command that wants an alias today |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A plugin fails to start, which takes its commands and its runtime behavior with it. For the RPKI plugin that means no origin validation. Nothing on the BGP wire changes, and no session is affected |
| How is it reverted? | Single commit revert. The declaration is additive, and a plugin that sends no `pipes` list behaves exactly as it does today |
| Who else touches this path? | The parallel audit that produces `plan/audit-command-pipe-vs-subcommand.md` names the other consumers. Any spec touching `internal/component/command` pipe files, and any spec touching Stage 1 of the plugin protocol |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A plugin sends a `pipes` list in `declare-registration` | → | `engineStartupSink.onRegistration` validates it and writes it to `aliasRegistry` | `TestOnRegistrationRegistersPluginPipes` |
| An operator types `show bgp rpki \| summary` over the SSH exec channel | → | `expandAliases` splices the plugin's chain, `ApplyPipes` runs it over the plugin's payload | `test/plugin/plugin-pipe-alias.ci` |
| A plugin declares an alias name a built-in operator already carries | → | The validator refuses it and `onRegistration` returns the relayed error | `test/plugin/plugin-pipe-alias-collision.ci` | <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->
| A plugin stops after registering an alias | → | Removal by owner clears its entries from `aliasRegistry` | `TestPluginPipesRemovedOnPluginStop` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A plugin declares a command and a pipe alias on that command in one Stage 1 message | The plugin starts, and an operator typing that name after a pipe character on that command gets the expansion's answer |
| AC-2 | A plugin declares an alias whose name is a built-in pipe operator | Stage 1 fails, the plugin does not start, and the error text names the plugin, the alias name and the reason |
| AC-3 | A plugin declares an alias whose name a pipe filter of an overlapping command path already carries | Stage 1 fails, and the error text names the alias name, the plugin, and the command path of the filter |
| AC-4 | A plugin declares an alias on the exact command path where an alias of that name is already registered | Stage 1 fails, and the plugin already serving that name keeps serving it unchanged |
| AC-5 | A plugin declares an alias whose expansion names a word that is not a built-in pipe operator | Stage 1 fails, and the error text names the offending word |
| AC-6 | A plugin declares an alias whose expansion names another alias | Stage 1 fails, and the error text says an alias may not name another alias |
| AC-7 | A plugin declares an alias on a command path it did not declare in the same message | Stage 1 fails, and the error text names the path and says the plugin does not own it |
| AC-8 | A plugin declares an alias on a parent path and also declares leaf commands below it | The alias is offered on the parent and on no leaf |
| AC-9 | A plugin that registered an alias stops | The name stops resolving, stops being offered by completion, and the same plugin can start again and register it |
| AC-10 | An operator types a partial pipe name after a plugin command | Completion offers the plugin's alias names beside the built-in operators |
| AC-11 | An operator runs `command help` for a plugin command that carries an alias | The answer lists the alias name and its description |
| AC-12 | An operator runs `show bgp rpki \| summary` | The answer carries the RPKI aggregate fields and no cache server rows |
| AC-13 | An operator runs `show bgp rpki summary` | The answer is unchanged from today |
| AC-14 | An operator types an argument after a plugin alias name | The chain is refused with a message saying the alias accepts no argument |
| AC-15 | A plugin declares two aliases of the same name on the same path in one message | Stage 1 fails, and neither is registered |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `show bgp rpki \| summary` and reads the aggregate counters | SSH exec, ProcessPipes, expandAliases, dispatcher, RPKI plugin answer, ApplyPipes | `test/plugin/rpki-pipe-summary.ci` |
| 2 | Types `show bgp rpki \| ` and reads the names on offer | Interactive CLI completer, `pipeExtras`, `AliasesForCommand` | `test/ui/plugin-pipe-alias-completion.ci` | <!-- doc-links: ignore (fixture never written: no client in the tree drives the daemon-hosted TUI, and no daemon-side completion surface offers a pipe operator) -->
| 3 | Runs `command help "show bgp rpki"` and reads which pipe names the command answers to | Meta command plugin, `AliasesForCommand` | `test/plugin/plugin-pipe-alias-help.ci` | <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->
| 4 | Starts a plugin whose alias name is already taken and reads why it refused | Stage 1 validation, relayed error, daemon log | `test/plugin/plugin-pipe-alias-collision.ci` | <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->
| 5 | Runs `show bgp rpki roa 192.0.2.0/24` and gets the covering VRPs | Dispatcher argument folding, RPKI plugin lookup | Existing RPKI coverage, unchanged by this spec |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegisterPluginAliasesReturnsErrorNotPanic` | `internal/component/command/alias_test.go` | The plugin-facing entry point reports every bad registration as an error, for each of the refusal cases the in-tree path panics on | done, phase 1 |
| `TestRegisterPluginAliasesRefusesBuiltinOperatorName` | `internal/component/command/alias_test.go` | AC-2 | done, phase 2 |
| `TestRegisterPluginAliasesRefusesFilterNameOnOverlappingPath` | `internal/component/command/alias_test.go` | AC-3, and that the population for this check is overlapping paths | done, phase 2. It drives a filter on a SHORTER path and one on a LONGER path, so the overlap reading is pinned in both directions |
| `TestRegisterPluginAliasesAllowsSameNameOnLongerPath` | `internal/component/command/alias_test.go` | R-1. `summary` on `show bgp rpki` is accepted while `show bgp` carries `summary` | done, phase 2 |
| `TestRegisterPluginAliasesRefusesSameNameOnSamePath` | `internal/component/command/alias_test.go` | AC-4, and that the population for this check is the exact path | done, phase 2 |
| `TestRegisterPluginAliasesRefusesExpansionNamingAnAlias` | `internal/component/command/alias_test.go` | AC-6 | done, phase 2 |
| `TestRegisterPluginAliasesRefusesUnknownOperatorInExpansion` | `internal/component/command/alias_test.go` | AC-5 | done, phase 2 |
| `TestRegisterPluginAliasesRefusesDuplicateNameInOneBatch` | `internal/component/command/alias_test.go` | AC-15 | done, phase 2 |
| `TestRegisterPluginAliasesIsAllOrNothing` | `internal/component/command/alias_test.go` | A batch with one bad entry registers none of its entries | done, phase 2 |
| `TestUnregisterPluginAliasesRemovesOnlyThatOwner` | `internal/component/command/alias_test.go` | AC-9, and that an in-tree registration on the same path survives | done, phase 3. A second owner's entry on the same path survives too |
| `TestPluginAliasDoesNotLeakToSiblingLeaf` | `internal/component/command/alias_test.go` | AC-8 and A-2 | done, phase 3 |
| `TestOnRegistrationRegistersPluginPipes` | `internal/component/plugin/server/startup_test.go` | The Stage 1 wiring, and that validation runs before any conversion | done, phase 1. The refusal half is `TestOnRegistrationRefusesMalformedPluginPipe`, beside it |
| `TestOnRegistrationRefusesPipeOnUndeclaredCommand` | `internal/component/plugin/server/startup_test.go` | AC-7 | done, phase 2 |
| `TestOnRegistrationRollsBackPipesOnLaterFailure` | `internal/component/plugin/server/startup_test.go` | A-5. A family conflict after the pipe registration leaves no alias behind | done, phase 3 |
| `TestPluginPipesRemovedOnPluginStop` | `internal/component/plugin/server/startup_test.go` | AC-9 | done, phase 3. It stops the plugin through `rollbackStartupProcess` and starts it again |
| `TestAliasesForCommandListsPluginAliases` | `internal/component/command/alias_test.go` | The completion and help sources see a plugin's aliases | done, phase 4. It asserts `completePipeForCommand` too, which is the completer contract AC-10 rests on |
| `TestPipeAliasArgumentRefused` | `internal/component/command/pipe_test.go` | AC-14 for a plugin-registered alias | done, phase 4 |
| `TestPipeAliasHelp` | `internal/plugins/meta/cmd/help_test.go` | The help renderer, beside `TestPipeFilterHelp` | done, phase 4 |

### Boundary Tests (numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Number of pipe declarations in one Stage 1 message | 0 to the message size limit | the largest batch that fits the frame | N/A | a batch that exceeds the frame, refused by the existing frame limit |
| Number of operator segments in one expansion | 1 upward | no declared ceiling | 0 segments, refused as "expands to nothing" | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `plugin-pipe-alias` | `test/plugin/plugin-pipe-alias.ci` | An external Python plugin declares a command and an alias over it, and the operator gets the expansion's answer | done, phase 1 |
| `plugin-pipe-alias-collision` | `test/plugin/plugin-pipe-alias-collision.ci` | A second plugin declares a name that is already taken, fails to start, and the first plugin keeps answering | done, phase 2. CORRECTED: two plugins cannot reach one path, so the refused declaration is a plugin against the IN-TREE aliases on `show bgp` | <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->
| `plugin-pipe-alias-namespaced` | `test/plugin/plugin-pipe-alias-namespaced.ci` | The alias is offered on the declaring plugin's command and refused on an unrelated command | done, phase 3. It asserts the plugin's own leaf below the alias too | <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->
| `plugin-pipe-alias-help` | `test/plugin/plugin-pipe-alias-help.ci` | `command help` lists the alias with its description | done, phase 4. It asserts the in-tree half beside the declared one |
| `plugin-pipe-alias-completion` | `test/ui/plugin-pipe-alias-completion.ci` | The interactive CLI offers the plugin's alias name in the operator slot | BLOCKED, phase 4. `ze cli` runs its model in the CLIENT process, so no declared alias is offered or resolvable there. See the Phase 4 Record | <!-- doc-links: ignore (fixture never written: no client in the tree drives the daemon-hosted TUI, and no daemon-side completion surface offers a pipe operator) -->
| `rpki-pipe-summary` | `test/plugin/rpki-pipe-summary.ci` | `show bgp rpki \| summary` answers the aggregate half, and `show bgp rpki summary` is unchanged | done, phase 5 |

### Interop Tests (Scope: protocol)

Not applicable. Nothing here reaches the BGP wire, and no peer daemon observes a
CLI pipe alias.

## Files to Modify

- `pkg/plugin/rpc/types.go` - the new declaration type and the list on
  `DeclareRegistrationInput`. Its declared design document is
  `docs/architecture/api/ipc_protocol.md`
- `pkg/plugin/sdk/sdk_types.go` - the SDK re-export. Its declared design document
  is `docs/architecture/api/process-protocol.md`
- `pkg/plugin/sdk/sdk.go` - the SDK-side declaration helper, if the SDK carries
  one for the other declarations
- `internal/core/ipc/yang/ze-plugin-engine.yang` - the description of the new
  Stage 1 input list
- `internal/component/plugin/server/startup.go` - validation, registration,
  rollback, and removal on stop. Its declared design document is
  `docs/architecture/api/process-protocol.md`
- `internal/component/command/alias.go` - the plugin-facing registration entry
  point that returns an error, ownership, and removal by owner. Its declared
  design document is `docs/architecture/api/commands.md`
- `internal/plugins/meta/cmd/help.go` - list a command's pipe aliases beside its
  pipe filters. Its declared design document is
  `docs/architecture/api/commands.md`
- `internal/component/bgp/plugins/rpki/rpki.go` - declare the bare
  `show bgp rpki` command, build its payload, and declare the pipe aliases over
  it. Its declared design document is
  `docs/architecture/plugin/rib-storage-design.md`
- `test/scripts/ze_api.py` (retired, no successor) <!-- doc-links: ignore (deleted 2026-08-28 by eae282592 with no replacement) --> - the matching declaration method for the Python
  plugin client
- `docs/architecture/api/commands.md` - the pipe alias section gains the plugin
  declaration channel, the collision rule and the payload obligation
- `docs/architecture/api/process-protocol.md` - the Stage 1 declaration list
- `docs/architecture/api/ipc_protocol.md` - the new RPC type
- `docs/architecture/plugin/rib-storage-design.md` - the RPKI command surface
  change
- `docs/guide/rpki.md` - the operator-facing command list
- `docs/guide/command-reference.md` - the new bare `show bgp rpki` command
- `docs/guide/plugins.md` - what a plugin can declare
- `ai/rules/plugins.md` - the Stage 1 declaration list

## Files to Create

- `test/plugin/plugin-pipe-alias.ci` - the declaration and the operator's answer
- `test/plugin/plugin-pipe-alias-collision.ci` - the refusal and its message <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->
- `test/plugin/plugin-pipe-alias-namespaced.ci` - the scope boundary <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->
- `test/plugin/plugin-pipe-alias-help.ci` - the help listing <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->
- `test/plugin/rpki-pipe-summary.ci` - the converted consumer
- `test/ui/plugin-pipe-alias-completion.ci` - the completion offer <!-- doc-links: ignore (fixture never written: no client in the tree drives the daemon-hosted TUI, and no daemon-side completion surface offers a pipe operator) -->

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/core/ipc/yang/ze-plugin-engine.yang` describes the `declare-registration` input, and the new list is described there. This is the IPC schema, not operator config, so `ai/rules/config.md` does not apply |
| YANG validation constraints | N-A | The IPC schema describes the RPC input. Validation of a plugin's declaration is engine-side, because the refusal text has to name the other holder of a colliding name |
| YANG custom validators | N-A | Same reason |
| CLI commands/flags | Yes | `internal/component/bgp/plugins/rpki/rpki.go` declares the bare `show bgp rpki` command. No `cmd/ze/` change: the command is a plugin declaration, not a binary subcommand |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md`. `show bgp rpki` is a bare path with no value, and the aliases take no argument |
| Editor autocomplete | N-A | No YANG config leaf is added |
| Functional test for new RPC/API | Yes | `test/plugin/plugin-pipe-alias.ci` and its siblings |
| Pipe completeness | Yes | The bare `show bgp rpki` answer routes through `ApplyPipes` like every other plugin command answer, and the aliases are built-in operators after expansion |
| Env var registration | N-A | No YANG leaf under `environment/` |
| Doctor check for runtime dependencies | N-A | No new file path, socket, service, port or binary |
| Prometheus counters/metrics | No | A refused registration is a startup failure the existing plugin startup path already logs and counts |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute changes |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`, for the bare `show bgp rpki` command |
| 2 | Config syntax changed? | No | No YANG config leaf and no config file syntax changes |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/rpki.md` |
| 7 | Wire format changed? | No | Nothing on the BGP wire changes. The plugin IPC message changes, which is row 8 |
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugins.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/api/ipc_protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RPKI validation behavior is untouched. Only the shape of the command that reports it changes |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, for the new declaration method on the Python plugin client |
| 11 | Affects daemon comparison? | No | No feature parity claim changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/commands.md` carries the pipe alias design and gains the plugin channel |
| 13 | Route metadata keys added/changed? | No | No route metadata is read or written |
| 14 | Prometheus counters added/changed? | No | No counter is added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md`, for the new command and the new declaration |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/spec-plugin-registers-pipe-operations.md` at implementation time. The declared owners of the files listed above are `docs/architecture/api/commands.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/api/ipc_protocol.md` and `docs/architecture/plugin/rib-storage-design.md`, and all four are named in Files to Modify |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/rpki.md` shows the RPKI command examples. Verify each against the handler after the change |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - the declaration reaches the registry
   - Tests: `TestOnRegistrationRegistersPluginPipes`, `test/plugin/plugin-pipe-alias.ci`
   - Files: `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_types.go`,
     `internal/core/ipc/yang/ze-plugin-engine.yang`,
     `internal/component/plugin/server/startup.go`,
     `internal/component/command/alias.go`, `test/scripts/ze_api.py` (retired, no successor) <!-- doc-links: ignore (deleted 2026-08-28 by eae282592 with no replacement) -->
   - Verify: the entry point exists and is reachable. The functional test fails
     because the validator refuses everything and the registration is a stub

2. **Phase: Validation and refusal** - every collision case answers correctly
   - Tests: the `TestRegisterPluginAliases*` set, `TestOnRegistrationRefusesPipeOnUndeclaredCommand`,
     `test/plugin/plugin-pipe-alias-collision.ci` <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->
   - Files: `internal/component/command/alias.go`,
     `internal/component/plugin/server/startup.go`
   - Verify: AC-2 through AC-7 and AC-15 each have a failing test that passes
     after the check lands, and the same-path and overlapping-path readings are
     each pinned by their own test

3. **Phase: Scope and lifecycle** - the alias reaches the declaring commands and
   leaves with the plugin
   - Tests: `TestPluginAliasDoesNotLeakToSiblingLeaf`,
     `TestUnregisterPluginAliasesRemovesOnlyThatOwner`,
     `TestPluginPipesRemovedOnPluginStop`,
     `TestOnRegistrationRollsBackPipesOnLaterFailure`,
     `test/plugin/plugin-pipe-alias-namespaced.ci` <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->
   - Files: `internal/component/command/alias.go`,
     `internal/component/plugin/server/startup.go`
   - Verify: AC-8 and AC-9 pass, and a plugin can start, stop and start again

4. **Phase: Discovery** - completion and help report the name
   - Tests: `TestAliasesForCommandListsPluginAliases`,
     `test/ui/plugin-pipe-alias-completion.ci`, <!-- doc-links: ignore (fixture never written: no client in the tree drives the daemon-hosted TUI, and no daemon-side completion surface offers a pipe operator) -->
     `test/plugin/plugin-pipe-alias-help.ci` <!-- doc-links: ignore (fixture this spec will create; it is not implemented yet) -->
   - Files: `internal/plugins/meta/cmd/help.go`
   - Verify: AC-10 and AC-11 pass. Completion already reads the registry, so the
     completion test is expected to pass with no completer change, and it is
     written anyway because that is the assertion nothing else makes

5. **Phase: The converted consumer** - RPKI
   - Tests: `test/plugin/rpki-pipe-summary.ci`, plus the existing RPKI coverage
   - Files: `internal/component/bgp/plugins/rpki/rpki.go`
   - Verify: AC-12 and AC-13 pass. The bare `show bgp rpki` payload carries the
     aggregate fields and the cache server rows as siblings, and the alias is
     pure selection over them

6. **Phase: Documentation** - the channel is discoverable by the next author
   - Tests: `./le doc check verify`
   - Files: every row of the Documentation Update Checklist answered Yes
   - Verify: `./le spec citation anchors spec plan/<this-spec>.md` reports no unnamed owner

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The collision population is the EXACT path for alias against alias, and OVERLAPPING paths for alias against filter. A reviewer must confirm both readings have their own test, because collapsing them either refuses the motivating consumer or lets a dark alias through |
| Correctness | The derived barrier covers every declared command of the plugin that sits strictly below a path carrying one of its aliases, and covers nothing else |
| Correctness | No path from a plugin's declaration reaches a `panic`. Grep the plugin-facing entry point for every call it makes into the in-tree registration path |
| Correctness | Removal by owner runs on the Stage 1 rollback path AND on the plugin stop path. One without the other leaves a plugin unable to restart |
| Naming | The declared thing is a pipe ALIAS, which is the name the registry already uses. No second name for it appears in code, docs or error text |
| Naming | Wire keys are lowercase kebab-case, matching every other Stage 1 declaration |
| Data flow | Nothing in `internal/component/command` learns what a plugin is. The owner is an opaque string |
| Rule: `ai/rules/cli.md` | The bare `show bgp rpki` answer is structured data, never finished text, and it routes through `ApplyPipes` |
| Rule: `ai/rules/plugins.md` | Registration over hardcoding. No plugin name appears in a core or shared package |
| Rule: `ai/rules/evidence.md` | The refusal names the plugin, the alias name, the command path and the other holder. A refusal that says only "collision" tells an operator nothing they can act on |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The Stage 1 declaration type exists and is re-exported by the SDK | `grep -rn "PipeDecl" pkg/plugin/rpc/types.go pkg/plugin/sdk/sdk_types.go` |
| The engine validates before it registers | Read `onRegistration` and confirm the validation call sits beside the doctor check and enricher validation, before `registrationFromRPC` |
| No plugin input reaches a panic | `grep -n "panic" internal/component/command/alias.go` and confirm every remaining one is on the in-tree path only |
| Removal by owner exists and is called twice | `grep -rn "UnregisterPluginAliases" internal/` returns the rollback call site and the stop call site |
| The Python plugin client can declare one | `grep -n "declare_pipe" test/scripts/ze_api.py` |
| Every functional test exists | `ls test/plugin/plugin-pipe-alias*.ci test/plugin/rpki-pipe-summary.ci`. `test/ui/plugin-pipe-alias-completion.ci` is NOT among them, for the reason the Phase 4 Record gives | <!-- doc-links: ignore (fixture never written: no client in the tree drives the daemon-hosted TUI, and no daemon-side completion surface offers a pipe operator) -->
| The RPKI conversion works | `./le functional plugin` covering `rpki-pipe-summary.ci` |
| The gate is green | `./le verify current mode full` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | Every string in the declaration comes from a plugin process. The name, the command path and the expansion are all validated before use. A name is normalized to lowercase and trimmed, and an empty result is refused rather than stored |
| Input validation | The expansion is parsed by the one operator parser. No second grammar, and no path where an unparsed string reaches the renderer |
| Resource exhaustion | An expansion is parsed once at registration, and expansion at command time is one pass with a length fixed at registration. A plugin cannot make the resolver loop, because an alias may not name another alias |
| Authorization that could fail open | A plugin may name only a command path it declared in the same message. A check that answers "allowed" when it cannot resolve the path would let a plugin claim another owner's subtree, so the check refuses whatever it cannot confirm |
| Error leakage | The refusal text names plugin names and command paths, which are already visible to an operator through the plugin inventory and the command list. It carries no config value and no credential |
| Denial of service | A refused declaration fails one plugin's startup. Confirm it cannot fail the daemon's startup, and that a plugin failing after a partial registration leaves the registry as it was |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood, go back to RESEARCH |
| Lint failure | Fix inline. If architectural, go back to DESIGN |
| Functional test fails | Check the AC. Wrong AC goes to DESIGN, correct AC goes to IMPLEMENT |
| The motivating consumer is refused by the collision rule | The rule collapsed the exact-path and overlapping-path readings. Read R-1 and the two tests that pin them |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

### The declaration is a separate list, not a field on `CommandDecl`

`CommandDecl` is keyed by an exact command name. A pipe alias sits on a command
PATH, and the path is what the longest-prefix lookup resolves. A plugin that
declares six commands and wants one alias over the parent of all six would have to
say so in one of the six entries, and the entry it picked would carry a fact about
a different command.

The registration input already carries one list per optional declaration:
families, filters, doctor checks, enrichers, config operations. A `pipes` list is
the same shape, so the wire message stays uniform and a plugin that declares no
alias sends nothing.

Each entry carries four values.

| Field | Type | Description |
|-------|------|-------------|
| command | string | The command path the alias sits on. MUST be one of the plugin's own declared command names in the same message |
| name | string | The word an operator types after the pipe character. Lowercase, one word |
| description | string | The line completion and help show beside the name |
| expansion | string | The operator chain the name stands for, written the way an operator would type it |

### The grammar is the one that already exists

The expansion is read by `parsePipeOps`, which is what `RegisterAliases` already
uses to read an in-tree expansion. That keeps one reader of operator names, which
is why an in-tree alias and a plugin alias can never disagree about what `display`
means.

| Rule | Form |
|------|------|
| An expansion | one or more segments, separated by the pipe character |
| A segment | an operator name, then the argument words that operator takes |
| An operator name | a word in the built-in operator set |

The refusals, and what an author reads.

| Declaration | Answer |
|-------------|--------|
| expansion `display router-id local-as` | accepted, one segment |
| expansion `display state \| count` | accepted, two segments |
| expansion empty, or whitespace only | refused: the alias expands to nothing |
| expansion names a word that is no operator | refused, and the message names the word |
| expansion names another alias | refused: an alias may not name another alias |
| name is a built-in operator name | refused, and the message names the operator |
| name is empty after trimming | refused |
| a word follows the alias name when the operator types it | refused at command time, with a message saying the alias accepts no argument |

The one-pass property is what the "may not name another alias" rule protects.
`expandAliases` splices an alias's stored operators into the chain and never looks
at the result again, so an alias naming an alias would leave an unresolved name in
a chain nothing re-reads.

### Collision has two populations, because there are two resolution rules

The naive reading of "fail on collision" refuses the motivating consumer.
`show bgp` carries an alias named `summary`, and `pathsOverlap` says
`show bgp rpki` overlaps `show bgp`. Reading collision as overlap therefore
refuses RPKI's `summary` before it is written.

The two resolution rules are different, and each one decides its own population.

| Pair | How a command resolves them | Collides when |
|------|----------------------------|---------------|
| Alias against alias | `lookupAlias` reads the set on the LONGEST registered prefix. It never falls back to a shorter path. A per-path alias also shadows a global one of the same name | The two sit on the SAME normalized path. A longer path deliberately shadows a shorter one, and that shadowing is the mechanism `show bgp peer list` already uses for column order |
| Alias against pipe filter | `foldFilters` resolves a command's own filter before the chain reaches anything generic, for the whole subtree the filter covers | Their command paths OVERLAP, which is the rule `filterShadowing` and `aliasShadowing` already implement |
| Alias against built-in operator | `ParsePipe` reads the built-in name first, so the alias would never be reached | Always |

So a plugin's declaration is refused when any of the following is true.

1. The name is a built-in operator name.
2. A pipe filter on an overlapping command path carries the name.
3. An alias on the same command path carries the name, whoever registered it.
4. Two entries in the plugin's own message carry the same name on the same path.
5. The command path is not one the plugin declared in the same message.

And it is NOT refused when an alias of the same name sits on a shorter path, or in
the global table. Both of those are shadowed for this path by the existing
resolution rule, and shadowing is not a conflict.

### The moment of the check, and why order settles it

The check runs inside `onRegistration`, before any conversion, in the position
where doctor checks and enrichers are already validated. The write runs under
`startupRegistrationMu`, beside the registry row and the family registration, so
two plugins in one startup tier cannot both pass a check the other invalidates.

In-tree registration happens in `init()`, which completes before `main` runs, so
an in-tree name always exists before any plugin declares one. Between two plugins,
the one that reaches Stage 1 second is the one refused. A plugin already serving a
name is never displaced, which is what the owner's answer requires.

The residual is that two plugins wanting the same name on the same path get an
outcome that depends on tier and startup order. That is accepted, and the refusal
names both sides so the operator can change one. The alternative is silent
replacement, which the owner refused.

### Namespaced means per command path, with the plugin's own commands as the limit

"Namespaced to its own commands" is per COMMAND PATH in effect, and per PLUGIN in
authority. A plugin may name a path only when that path is one of the commands it
declared in the same message. It cannot reach the global alias table at all.
CORRECTED by the Review Gate: a path another PLUGIN declared is refused by
`PluginRegistry.Register` a step earlier, so that half holds. A path the DAEMON
serves itself is not refused, because the check reads the plugin's own command
list and the builtins are in neither registry it consults. Finding R-8 records
what that leaves, and it is additive only.

That leaves the inheritance question. An alias on `show bgp rpki` is inherited by
`show bgp rpki roa`, because `roa` registers nothing of its own and the lookup
resolves the longest registered prefix. The in-tree answer to this is
`RegisterAliases(cmdBgpChildren)`, an explicit empty declaration on every child.

Asking a plugin author to know that rule is asking them to know the resolution
algorithm. The engine derives it instead: for every command the plugin declared
that sits strictly below a path carrying one of its aliases, and that carries no
alias of its own, the engine registers an empty declaration. The barrier is a
consequence of the plugin's own command list, not a thing an author writes.

### A pipe alias reshapes an answer. It cannot produce one

This is the boundary, and it is the reason `roa` and `aspa` stay subcommands.

What the operators an expansion may name actually do.

| Operator | What it does to the payload |
|----------|-----------------------------|
| `display` | keeps the named keys, drops the rest, and puts the named ones first |
| `fill` | re-sequences the keys `display` did not name |
| `count` | replaces the answer with the number of items |
| `first`, `last` | keeps the first or last N items |
| `match` | keeps the rendered lines that match a pattern |
| `json`, `ndjson`, `yaml`, `table`, `text`, `raw` | renders the same payload a different way |
| `resolve`, `origin` | adds a reverse name or an origin AS beside an address already in the payload |

What none of them can do: rename a key, add two numbers, count the rows whose
field holds a given value, or ask the command handler for anything.

A pipe alias also takes no argument. `expandAliases` refuses a word after the
name, deliberately, rather than dropping it.

So the test for whether something is an alias or a subcommand has two questions,
and either one sends it back to being a subcommand.

| Question | If yes |
|----------|--------|
| Would the operator have to supply a value? | Subcommand. An alias takes no argument |
| Does the answer need data the parent's payload does not already carry? | Subcommand. An alias reshapes what was returned |

`show bgp rpki roa 192.0.2.0/24` takes a prefix and looks it up against the ROA
cache. `show bgp rpki aspa 65001` takes a customer AS and looks it up against the
ASPA cache. Both fail the first question, and the lookup version fails the second
as well. They stay subcommands.

### The payload has to be built for it, and RPKI's is not

`show bgp | summary` works because the `show bgp` answer carries its aggregate
fields and its `peers` array as siblings at the same level. The alias is a
selection among sibling keys, and nothing is computed.

RPKI's two answers are not related that way.

| Field in `show bgp rpki summary` | Where it comes from |
|----------------------------------|---------------------|
| `vrp-count` | the SUM of `vrp-count-ipv4` and `vrp-count-ipv6`, which `status` reports separately |
| `sessions-total` | the field `status` spells `sessions` |
| `sessions-established` | a COUNT of the `cache-servers` rows whose `state` is established |
| `validation-enabled` | a constant |
| `sessions-synced`, `aspa-enabled`, `aspa-records` | present verbatim in both answers |

Four of the seven cannot be produced by any operator an expansion may name. So
`show bgp rpki | summary` reproduces today's summary payload only if the bare
`show bgp rpki` payload is DESIGNED to carry those aggregate fields as siblings of
the cache server rows, exactly as `show bgp` does.

That is a payload obligation on the command, not a feature of the pipe layer. Any
command that wants a pipe alias owes it.

A document payload is fine. `display` cuts the keys of a single record just as it
cuts the keys of a row, and the renderers handle both shapes. What is not fine is
a payload whose aggregate half is absent, because there is nothing to select.

### Discovery has three surfaces and one of them cannot answer

Every discovery surface is one of two kinds. A surface that reads the running
daemon's registry can report a plugin's alias. A surface that reads the compiled
tree in its own process cannot, because no plugin's Stage 1 message ever reaches
it.

| Surface | Kind | Sees a plugin's alias |
|---------|------|----------------------|
| Completion in the interactive CLI a plain ssh client with a pty reaches | Running daemon. `bubbletea.Middleware` in `internal/component/ssh/ssh.go` hosts the model, built by `buildSessionModelFactory` in `cmd/ze/hub/session_factory.go`, and `pipeExtras` reads `AliasesForCommand` in that process | Yes, with no change |
| Completion in `ze cli` with no command argument | The CLIENT process. `runInteractiveSession` (`internal/component/cli/client/main.go`) runs its own `tea.NewProgram`, and `executeOperationalCommand` (`internal/component/cli/model_mode.go`) resolves the chain there before it sends anything | No. CORRECTED in phase 4: the plan recorded this surface as needing no change, and it needs a channel that does not exist. The compiled-in aliases work there, which is what hid it |
| `command help "<name>"` | Running daemon. The meta command plugin answers it from the dispatcher and the registries | Only after a change. The handler reports a command's pipe FILTERS and no aliases. That gap exists for in-tree aliases today, and this spec closes it for both |
| `./le command list` | Own process. `internal/le/command/list/commandlist.go` is a `go run` program that blank-imports the compiled plugin set | No, and it cannot |
| `ze help command --json`, and the wiki catalog `./le wiki-catalog update` builds from it | Own process. `collectCommands` reads the YANG command tree and the local command registry, and `extractPipes` reads the in-process pipe filter registry | No, and it cannot |

So the running daemon is the ONLY discovery surface for a plugin's alias, and
`command help` is the only one that answers a question about a named command
from any client. That is why the help change is in scope rather than deferred:
without it a plugin's alias is discoverable by pressing Tab in one client and
not at all in the other.

The published wiki catalog therefore lists a plugin's commands without its pipe
aliases. Giving the catalog a daemon-backed source is a change to the inventory
tooling, not to this mechanism, and it is recorded in Known Limitations.

### Where the chain is resolved, and the one place it is not

The daemon resolves the chain for every path that matters. The SSH exec handler
splits the chain off the input and applies it to the dispatcher's answer, and the
web and interactive CLI surfaces call the same wrappers in the same process.

The exception is `cliClient.StreamMonitor`, which resolves pipes in the CLI client
process for streaming monitor commands. A plugin's alias is not registered there.
No plugin declares a streaming monitor command that wants an alias today, and
carrying the registry to the client is a much larger change than this one, so that
surface is scoped out and recorded in Known Limitations.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A colliding declaration fails the plugin's whole registration | Last wins, first wins, silent skip | OWNER DECISION. A silent skip leaves an operator with a name that completes and does nothing, and nothing says why. Last wins lets a plugin loading later take a name from one already serving. First wins looks safe and still leaves the losing plugin running with a feature it thinks it has. A refusal is the only outcome where the operator learns which two things want the same word |
| A plugin's alias is namespaced to its own commands and never becomes global | A global registration, or an opt-in global flag | OWNER DECISION. A global name reaches every command in the daemon, including commands whose payload cannot answer it. The blast radius of one plugin's naming choice would be the whole CLI |
| The expansion travels as a string the engine parses | A structured or typed declaration of operators and arguments | OWNER DECISION. The string is what an operator would type, so an author writes what they read in the CLI. It also reuses `parsePipeOps`, which keeps one reader of operator names in the tree. A typed declaration would be a second spelling of the same grammar, and the two would drift |
| Collision is the EXACT path for alias against alias, and OVERLAPPING paths for alias against filter | One rule for both, using overlapping paths | Two different resolution rules produce two different populations. A filter wins for its whole subtree, so an overlapping filter really does make an alias unreachable. An alias on a longer path shadows one on a shorter path, which is the documented inheritance mechanism, and treating that as a collision refuses `show bgp rpki \| summary` because `show bgp` carries `summary` |
| The declaration is its own list on the registration input | A field on `CommandDecl` | An alias sits on a command PATH and `CommandDecl` is keyed by an exact name. Every other optional declaration on the registration input is already its own list |
| The engine derives the inheritance barrier from the plugin's own command list | Ask the plugin to declare an empty entry per leaf, as `peer.go` does | The in-tree form makes an author responsible for knowing the longest-prefix rule. An author who forgets gets an alias offered on a leaf whose payload cannot answer it, and nothing reports it |
| A plugin's declaration reaches an entry point that returns an error | Reuse `RegisterAliases`, which panics on a bad registration | `RegisterAliases` documents that only a registration in this repository can produce its panic and that no operator input reaches it. A plugin's string breaks that premise, and a panic on a plugin's declaration takes the daemon down over one plugin's typo |
| Removal by owner is part of this change | Leave removal to a follow-up | Without it, a plugin that restarts collides with its own previous registration, and "fail on collision" makes restart impossible. The failure is created by this change, so it is fixed by it |
| `show bgp rpki summary` stays as a command | Remove it, or map it to the pipe form as a deprecated name | Removing a working command is not what this spec was asked to do, and `DeprecatedNames` maps an old name to a command rather than to a pipe chain. Whether the subcommand is retired is a separate question for the audit's consumer list |
| One consumer is converted in this spec | Ship the mechanism alone, or convert every consumer the audit finds | A mechanism with no consumer is unwired, and unwired code is what the wiring test rule exists to refuse. Converting every consumer makes this spec depend on an audit that is still running |

## Known Limitations

- Neither `./le command list` nor `ze help command --json` can report a
  plugin's pipe alias. Both read the compiled tree in their own process and start
  no plugin, so the published wiki catalog `./le wiki-catalog update` builds
  lists a plugin's commands without its aliases. The running daemon is the only
  discovery surface, through completion and `command help`. Giving the catalog a
  daemon-backed source is a change to the inventory tooling and is recorded in the
  deferral shard.
- `cliClient.StreamMonitor` resolves pipes in the CLI client process, where a
  plugin's alias is unknown. An alias on a streaming monitor command would be
  reported as an unknown operator. No plugin declares such a command today.
- CORRECTED in phase 4, and wider than the row above: the whole interactive
  model of `ze cli` runs in the CLIENT process, so a declared alias is neither
  offered by completion nor resolvable there. The surfaces that answer are the
  SSH exec channel and the daemon-hosted TUI a plain ssh client reaches. The
  repair is a channel that carries the daemon's alias table to the client, and
  it is recorded in `plan/journal/unwired-feature.md` rather than designed here.
- A plugin cannot declare a per-command COLUMN ORDER. Without one,
  `| display <partial>` over a plugin command offers no field names, because
  `completeDisplayFields` reads the column registry. The alias itself works, and
  its NAME completes, because that comes from the alias registry. This is a
  separate declaration channel and a separate spec. Recorded in the deferral
  shard.
- A plugin cannot declare a pipe FILTER, which is the mechanism that folds a
  filter into a command argument and runs it in the handler. Nothing in this spec
  adds that channel. If it is ever added, it must run the same overlapping-path
  check from its side, exactly as `RegisterPipeFilters` does today.
- Two plugins wanting the same alias name on the same path get an outcome that
  depends on tier and startup order. The refusal names both, and the operator
  resolves it by renaming one. Measured in phase 2 and unreachable in practice:
  `PluginRegistry.Register` refuses a command another plugin declared, and it
  runs before the alias write.
- The ownership check confirms that a plugin DECLARED the command path, not that
  the daemon routes that path to it. A plugin that declares a name the daemon
  serves itself passes Stage 1, and the dispatcher's own registry rejects that
  command entry later as a builtin conflict while the plugin keeps running. Its
  alias then sits on a command path the daemon answers. It can only ADD a name
  there, never take one, and the name leaves with the plugin. Found by the
  Review Gate as R-8 and reproduced from `onRegistration`; tightening the check
  would remove the only reachable exact-path collision, which is what the merge
  protection and `test/plugin/plugin-pipe-alias-collision.ci` both rest on, so
  it is the owner's call rather than the reviewer's.
- Only the RPKI consumer is converted here. `plan/audit-command-pipe-vs-subcommand.md`
  is the source of the full consumer list, and each remaining conversion is its
  own change against this mechanism.

## RFC Documentation (Scope: protocol)

Not applicable. Nothing in this spec implements or changes protocol behavior. The
RPKI plugin's validation behavior, its RTR sessions and its wire handling are
untouched, and only the shape of the command that reports them changes.

## Checklist

### Goal Gates (MUST pass)

- [ ] AC-1..AC-15 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `pkg/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD

- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: no wire-visible behavior changes)

### Closure

- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-plugin-registers-pipe-operations.md` only (commit A preserves the spec in history)

## Review Gate

Independent pass over `91203b8aa..de0f8c040` plus `2d244b186`, run by a context
that wrote none of it. Every lens was run by this one context, and no reader was
spawned (`ai/rules/planning.md`, "Independence is a property of the CONTEXT").

### Round scope, written before each round ran

| Round | Scope |
|-------|-------|
| 1 | The whole diff: six phase commits, the audit row, every changed Go file, every `.ci`, every doc, the deferral shard and the two journal rows |
| 2 | Only the prose round 1 corrected: `docs/architecture/api/commands.md`, this spec, the deferral shard. Re-run `./le doc check verify` |

Round 2 found nothing further, and every round-1 finding below is either fixed
or recorded with its owner. The loop ends there: a round whose findings are all
record defects is the last round.

### What was run, not narrated

| Check | Result |
|-------|--------|
| `./le functional plugin` | 686/686 PASS. The five tests this spec owns pass: `plugin-pipe-alias`, `plugin-pipe-alias-collision`, `plugin-pipe-alias-help`, `plugin-pipe-alias-namespaced`, `rpki-pipe-summary` |
| `go test -race ./...` over `command`, `plugin/server`, `bgp/plugins/rpki`, `plugins/meta/cmd` | green |
| `./le repository check` | all checks passed |
| `./le doc check verify` | passed, before and after the round-1 doc correction |
| `./le commit audit base 91203b8aa^` | clean, 13 test files examined |
| `./le spec citation anchors spec plan/<this-spec>.md` | exit 0 |

### Mutations run by this review, not trusted from a Phase Record

Each one was applied to the tree, the named test was run, the tree was restored
from a saved copy.

| Mutation | Test that went red |
|----------|--------------------|
| `mergedAliases` returns the declared set instead of merging | `TestRegisterPluginAliasesKeepsWhatThePathAlreadyCarries`, and `TestUnregisterPluginAliasesRemovesOnlyThatOwner` lost `show bgp` its `summary` |
| `UnregisterPluginAliases` removes the PATH instead of the owner's entries | the in-tree aliases and the second owner's alias both stopped answering |
| `aliasOnPath` reads `lookup` (longest prefix) instead of `get` (exact path) | `TestRegisterPluginAliasesAllowsSameNameOnLongerPath`: `show bgp rpki` was refused `summary` |
| `filterShadowing` narrowed from overlapping paths to the exact path | `TestRegisterPluginAliasesRefusesFilterNameOnOverlappingPath` in BOTH directions, and `TestAliasRegistrationRefusesFilterCollision` |
| `rollbackStartupProcess` stops removing the owner's aliases | `TestPluginPipesRemovedOnPluginStop`: the second start failed with `pipe alias totals is already registered on that command path`, which is exactly the hole phase 3 exists to close |
| a counter added to `appendSummaryFields` and not to `summaryFieldNames` | `TestSummaryFieldNamesMatchTheWrittenPayload` |

Both collision populations are therefore pinned independently: EXACT path for
alias against alias, OVERLAPPING paths for alias against filter.

### Findings

| # | Severity | Finding | Disposition |
|---|----------|---------|-------------|
| R-8 | ISSUE | `docs/architecture/api/commands.md` stated "One plugin's naming choice therefore never reaches a command it does not own". It does. `validatePipeDecls` (`internal/component/plugin/server/startup.go`) confirms the plugin DECLARED the path, and `PluginRegistry.Register` (`internal/component/plugin/registration.go`) never consults the builtin set, so a plugin that declares `show bgp` gets its alias on `show bgp`. Reproduced from `onRegistration` in a throwaway test: the call returned nil and `show bgp` then answered `[peers summary wide]`. The dispatcher's own `CommandRegistry.Register` rejects the plugin's command entry later as a builtin conflict, and logs a warning while the plugin keeps running | FIXED as a false safety claim. The paragraph now states what the check confirms and what it leaves. The BEHAVIOR is unchanged and is the owner's to rule on: a tightening was written and reverted, because refusing a builtin path removes the only reachable exact-path alias collision, which is the scenario `mergedAliases` was built for and the one `test/plugin/plugin-pipe-alias-collision.ci` drives. Named in Known Limitations |
| N-1 | NOTE | Every A-N still read `unvalidated` after all six phases, while the Goal Gates ask for none | FIXED. Each row carries its evidence. A-3's named test does not exist under that name and the row now names the test that was written |
| N-2 | NOTE | Sixteen Status cells in the TDD Test Plan and the Functional Tests table were blank for tests that exist and pass | FIXED |
| N-3 | NOTE | R-6's mitigation said the missing pipe-FILTER channel is "Recorded in the deferral shard". No row existed, and none was owed: nothing is deferred, the channel does not exist | FIXED, the claim is corrected |
| N-4 | NOTE | Known Limitations and the Phase 6 Record both name the deferral shard as the home for a daemon-backed command catalog. The shard carried one row, for the column order | FIXED, the row is written |
| N-5 | NOTE | `check_cross_package_wiring` reports `ColumnsForCommand` (`internal/component/command/column_order.go`) has no cross-package non-test caller, because this spec touched that file. It has two in-package callers, `render_records.go` and `completer.go`, so it is not dead. Pre-existing and outside `./le verify current mode full`, the same class the Phase 1 Record recorded for `AliasesForCommand` | not fixed. Unexporting it is a separate edit with its own callers to check |
| N-6 | NOTE | `audit-test-relaxation.py origin/main` reports one WEAKENED file, `internal/component/bgp/reactor/peer_initial_sync_test.go`, carrying RFC 2545 and RFC 4724 tags. `git log` charges it to `478dd21a5`, which is not in this spec's range. Scoped to `91203b8aa^` the audit is clean | not fixed. It belongs to the session that wrote `478dd21a5`, and reported to the owner |
| N-7 | NOTE | `handleBgpCommandComplete` (`internal/plugins/meta/cmd/help.go`) still reads `Dispatcher().Commands()`, the builtin table alone, so `show command complete` offers no command any plugin declares. Phase 4 fixed exactly this for `handleBgpCommandHelp` beside it and left the completer | not fixed. Pre-existing, and the phase that owns discovery recorded the surface gap it did close |
| N-8 | NOTE | `overviewCommand` always writes `"cache-servers":[]`, while `statusCommand` omits the key when no session exists. The two answers disagree about how "no cache server" is spelled | not fixed. `\| summary` drops the key either way, and no test pins either spelling |

0 BLOCKER, 0 ISSUE open. R-8's reviewable half is fixed; its residual is a
behavior the owner decided and it is now written down rather than denied.

### What was verified against source and a run, item by item

| Asked | Answer |
|-------|--------|
| Every AC-1..AC-15 has an implementation at file and symbol, and a discriminating test | Yes, with one exception. AC-10 is met at `completePipeForCommand` (`TestAliasesForCommandListsPluginAliases`) and has no end-to-end test. The Phase 4 Record says so and the reason holds: `test/ui/display-fill-completion.ci` is the one pty harness in the tree and it drives `ze cli`, whose model runs in the CLIENT process, and `handleBgpCommandComplete` offers command names alone |
| The two collision rules | Both pinned, and both mutations went red. See the mutation table |
| Merge, not replace | `mergedAliases` verified, and a plugin declaration cannot destroy `show bgp`'s `summary` or `peers`. The `.ci` asserts both still answer after the refusal |
| Removal is entry-scoped | `UnregisterPluginAliases` filters by NAME and removes a path only when the owner created it and its last entry is gone. In-tree names and a second owner's names both survive |
| Restart | `TestPluginPipesRemovedOnPluginStop` stops the plugin through `rollbackStartupProcess` and starts it again. The mutation proves the hole is real. A CRASH still reaches neither call site, which the Phase 3 Record states and which `PluginRegistry.Register` already blocks a respawn on, for the command row before the alias |
| AC-13 | `show bgp rpki summary` writes the same seven keys in the same order through `appendSummaryFields`, `TestSummaryCommand` still pins it, and `rpki-pipe-summary.ci` asserts the subcommand and the pipe form answer the identical record |
| The payload obligation | `summaryAliasExpansion` is built from `summaryFieldNames`, and `TestSummaryFieldNamesMatchTheWrittenPayload` holds that list against the bytes `summaryCommand` writes. Adding a counter to the writer alone turns it red |

### The three known gaps, judged

| Gap | Judgement |
|-----|-----------|
| A plugin's alias does not resolve in `ze cli` interactive | Honestly recorded, and documented well enough. Five documents name the exact string an operator sees, `pipe error: unknown pipe operator: <name>`, say why, and name the two surfaces that work: `docs/guide/rpki.md`, `docs/guide/command-reference.md`, `docs/features/formatting.md`, `docs/guide/plugins.md`, `docs/plugin-development/commands.md`, and `docs/architecture/api/commands.md` carries the table. The producing function is `executeOperationalCommand`, read and confirmed |
| A plugin cannot declare a column order | Honestly recorded in the deferral shard with a destination, and the cost is stated correctly: `\| display <partial>` offers no field names, the alias name still completes |
| `validation-enabled` is a literal | The call is right. Both literals predate this spec, the journal row names the producing functions and the conditional state, and AC-13 pins the payload. What this spec did change is the SURFACE: the literal now also appears in the bare `show bgp rpki` answer, and it could not be left out without breaking AC-13. Fixing it is a payload change that belongs to whoever retires `show bgp rpki summary` |

## Phase 1 Record: Wiring (2026-08-21)

The declaration reaches the registry. Both named tests exist, pass, and were
proven to discriminate by disabling `registerPluginPipes`.

| What | Where |
|------|-------|
| The wire type and the list on the registration input | `pkg/plugin/rpc/types.go` `PipeDecl`, `DeclareRegistrationInput.Pipes` |
| The SDK re-export | `pkg/plugin/sdk/sdk_types.go` `PipeDecl` |
| The Stage 1 input description | `internal/core/ipc/yang/ze-plugin-engine.yang` `list pipe` |
| The shape validator, run beside the doctor check and enricher validators | `internal/component/plugin/server/startup.go` `validatePipeDecls` |
| The write into the registry, under `startupRegistrationMu` | `internal/component/plugin/server/startup.go` `registerPluginPipes`, called from `engineStartupSink.onRegistration` |
| The plugin-facing entry point that returns an error | `internal/component/command/alias.go` `RegisterPluginAliases`, over `PluginAlias` |
| The one reading of the four refusals, shared by both entry points | `internal/component/command/alias.go` `checkAlias`. `checkedAlias` wraps it and panics, so the in-tree premise is unchanged |
| The Python plugin client declaration | `test/scripts/ze_api.py` (retired, no successor) <!-- doc-links: ignore (deleted 2026-08-28 by eae282592 with no replacement) --> `declare_pipe` |

### What phase 1 deliberately does not do

| Left | Consequence today | Owner |
|------|-------------------|-------|
| The exact-path alias-versus-alias refusal | A plugin declaring on a path that already carries an alias set REPLACES it, silently. Nothing in the tree declares a plugin alias, so no in-tree alias is reachable this way yet | Phase 2 |
| The undeclared-command refusal | A plugin can name a command path it did not declare | Phase 2 |
| The duplicate-name-in-one-message refusal | The later entry of a duplicated pair wins, because the batch is built into a map | Phase 2 |
| Removal by owner, and the rollback that calls it | A Stage 1 that fails AFTER the pipes are written leaves them in the registry, and the plugin cannot restart once phase 2 refuses a name already taken. `onRegistration` unwinds its own pipe failure, so the hole is a failure in a LATER stage rather than in the write itself | Phase 3 |
| The derived empty barrier | An alias on a parent path is offered on every leaf below it that declares none. Measured while writing `TestOnRegistrationRefusesMalformedPluginPipe`, whose first command path sat below the first test's and inherited its alias | Phase 3 |

### Corrections to the plan

| Statement | What happened |
|-----------|---------------|
| Phase 1 Verify: "The functional test fails because the validator refuses everything and the registration is a stub" | Not taken. A red `.ci` in `test/plugin/` reddens `./le verify current mode full` for every session in the checkout (`ai/rules/testing.md`, "Draft a Functional Test Before It Is Live"). Phase 1 wires the happy path far enough for `test/plugin/plugin-pipe-alias.ci` to pass, and the refusals stay with phase 2 |
| "`test/scripts/ze_api.py` (retired, no successor) <!-- doc-links: ignore (deleted 2026-08-28 by eae282592 with no replacement) --> gains the matching declaration method" | The declaration method was not enough. A Python plugin could declare a command and could not ANSWER one with data, so there was no payload for a selection to cut. `on_execute_command` was added beside `declare_pipe`. Recorded in `plan/journal/unwired-feature.md` |
| Files to Modify names `pkg/plugin/sdk/sdk.go` | No change was needed. The SDK passes `Registration` straight to the wire, so the type alias is the whole SDK surface |
| Discovery is phase 4 | `./le verify lint rundocwiring/checks.go` reports that `AliasesForCommand` (`internal/component/command/alias.go`) has no cross-package non-test caller. It predates this spec and phase 1 only exposed it by touching the file. Phase 4 is the fix: `internal/plugins/meta/cmd/help.go` is the caller it is missing. The finding reds no gate, because `check_cross_package_wiring` is deliberately outside `./le verify current mode full` (`internal/le/` native action tables, `./le repository tree-check`) |

## Phase 2 Record: Validation and refusal (2026-08-21)

Every collision case answers. AC-2, AC-3, AC-5 and AC-6 were already implemented
in phase 1, through the checks `checkAlias` shares with the in-tree path, and
they gained the tests that prove it. AC-4, AC-7 and AC-15 gained the checks that
were missing.

| What | Where |
|------|-------|
| The exact-path alias-versus-alias refusal (AC-4) | `internal/component/command/alias.go` `aliasOnPath`, read by `RegisterPluginAliases` |
| What one path declares, read without inheritance | `internal/component/command/column_order.go` `commandRegistry.get` |
| A declaration adds to a path and never replaces what it holds | `internal/component/command/alias.go` `mergedAliases` |
| The duplicate-name-in-one-message refusal (AC-15) | `internal/component/command/alias.go` `RegisterPluginAliases` |
| The undeclared-command refusal (AC-7) | `internal/component/plugin/server/startup.go` `validatePipeDecls`, over `commandPathKey` |
| The refusal an operator reads | `test/plugin/plugin-pipe-alias-collision.ci` |

### What phase 2 measured that the plan did not say

| Finding | Consequence |
|---------|-------------|
| The exact-path NAME check does not close the replacement hazard on its own. `commandRegistry.register` stores one value for each path, so a declaration carrying a name nobody holds still dropped every alias that path answered to. Measured before the fix: `show bgp` carrying `summary` and `peers` kept the declared `wide` alone | `mergedAliases` adds the declared set to what the path holds. Phase 3 removal by owner therefore removes ENTRIES, never paths: one path can hold the aliases of two owners |
| Two plugins cannot collide on one exact path at all. `PluginRegistry.Register` refuses a command another plugin declared, and it runs before the alias registration, so the second plugin fails on the COMMAND and never reaches its alias | R-4's startup-order residual is unreachable, and so is the "second plugin" of the `plugin-pipe-alias-collision` row. The reachable exact-path collision is a plugin against an IN-TREE alias, which is what the functional test drives |
| A refused plugin is torn down before it can act on the error the driver relays. The Python client recorded that it had started and never recorded the refusal | The operator's surface for a refusal is the daemon log, and the functional test reads it there |
| AC-14 and its test `TestPipeAliasArgumentRefused` sit in no phase's test list | Nothing proves that an argument after a PLUGIN-registered alias is refused. `TestAliasTakesNoArgument` proves it for an in-tree one |
| `internal/le/stressrepro/run.go` reads `bin/ze` and `bin/ze-test`, which a session-scoped build does not write | The load proof for the new functional test was six concurrent runs of the suite selector instead |

### What phase 2 deliberately does not do

Removal by owner, the rollback that calls it, and the derived empty barrier stay
with phase 3, exactly as the Phase 1 Record records them. The exact-path refusal
makes the missing removal reachable: a plugin that registers an alias, stops and
starts again is now refused its own name.

## Phase 3 Record: Scope and lifecycle (2026-08-21)

An alias reaches the command it was declared for and no command of the plugin
below it, and it leaves the registry when the plugin leaves. A plugin that
registers an alias, stops and starts again now starts.

| What | Where |
|------|-------|
| Ownership, and the removal that takes back what one owner registered | `internal/component/command/alias.go` `RegisterPluginAliases`, `UnregisterPluginAliases` |
| What one owner put on one command path, which removal reverses | `internal/component/command/alias.go` `pluginAliasPath`, over `pluginAliases` |
| The derived empty declaration that stops an alias below the command it sits on | `internal/component/command/alias.go` `aliasBarriers` |
| A command path stops being declared, so a command under it inherits again | `internal/component/command/column_order.go` `commandRegistry.remove` |
| The owner and the command list travelling from Stage 1 | `internal/component/plugin/server/startup.go` `registerPluginPipes` |
| The unwind of a Stage 1 that fails after the pipe write | `internal/component/plugin/server/startup.go` `engineStartupSink.onRegistration` |
| The removal a failed startup and a stopped plugin both run | `internal/component/plugin/server/startup.go` `rollbackStartupProcess` |
| The scope boundary an operator meets | `test/plugin/plugin-pipe-alias-namespaced.ci` |

### What phase 3 measured that the plan did not say

| Finding | Consequence |
|---------|-------------|
| The rollback call site and the stop call site are ONE function. `rollbackStartupProcess` is what the tier loop calls for a plugin that failed any stage, and what a config reload calls to stop a running plugin whose config section the operator removed (`stopCollectedProcesses`, `autoStopPluginNames`, `stopOrphanedDependencies`) | The Deliverables row expecting two call sites is answered by that function and by the family-conflict unwind inside `onRegistration` |
| With the pipe aliases written LAST, no failure inside `onRegistration` follows the write, so the rollback Data Flow step 4 asks for would have been unreachable code | The pipe registration now sits between the registry row and the families. Each failure branch unwinds what the branches above it wrote, in the reverse order |
| `TestOnRegistrationRollsBackPipesOnLaterFailure` cannot be driven through `runPluginPhase`. The driver's own rollback removes the aliases a moment later, so the test passes with the in-function unwind deleted | It calls `onRegistration` directly. The two unwinds answer different questions: the driver's runs once the whole tier has finished its handshake, and the plugins of one tier register concurrently, so a name a failed plugin no longer wants must be free for its neighbour before the tier ends |
| Removing the owner's NAMES is not enough on its own. A derived barrier carries no name at all, so a removal keyed by name can never take one back | The record carries the paths the owner registered from nothing beside the names it added. Such a path is removed once its last entry is gone, and a path that already carried a declaration keeps it |
| A crash and a respawn reach neither call site. `cleanupProcess` is the runtime exit path, and it unregisters the dispatcher's commands while leaving the `PluginRegistry` row and the runtime families in place | The alias is removed where the registry row and the families are removed, and nowhere else. Removing it on the runtime exit path alone would make the alias the one registration that left |
| `ResetAliasesForTest` had to clear the ownership record beside the registry. A record that survived the registry it describes removes another test's entries | The reset clears both, under the registry's own mutex |

### What phase 3 deliberately does not do

| Left | Consequence today | Owner |
|------|-------------------|-------|
| Discovery. `command help` reports a command's pipe filters and no alias, for an in-tree alias and a declared one alike | A declared alias is discoverable by tab completion alone | Phase 4 |
| AC-14 for a plugin-registered name, and its test `TestPipeAliasArgumentRefused` | `expandAliases` refuses a word after any alias name and reads no owner, so `TestAliasTakesNoArgument` proves the mechanism for both. What no test makes is the assertion over a DECLARED name | Phase 4, which is the phase whose subject is what an operator types at a plugin alias |

## Phase 4 Record: Discovery (2026-08-21)

`command help` reports the pipe aliases a command answers to, for a command a
plugin declared and for one the daemon carries itself. AC-11 and AC-14 are met.
AC-10 is met at the completer and NOT met end to end, because one of the two
interactive clients cannot see the registry the alias lives in.

| What | Where |
|------|-------|
| The alias listing, beside the filter listing | `internal/plugins/meta/cmd/help.go` `pipeAliasHelp`, read by `commandHelp` |
| The one answer both kinds of command are described by | `internal/plugins/meta/cmd/help.go` `commandHelp` |
| A plugin's command being describable at all | `internal/plugins/meta/cmd/help.go` `handleBgpCommandHelp`, over `Dispatcher().Registry().Lookup` |
| The cross-package caller `AliasesForCommand` was missing | `internal/plugins/meta/cmd/help.go` `commandHelp` |
| AC-14 over a declared name | `internal/component/command/pipe_test.go` `TestPipeAliasArgumentRefused` |
| The completer contract AC-10 rests on | `internal/component/command/alias_test.go` `TestAliasesForCommandListsPluginAliases`, over `completePipeForCommand` |
| The operator's answer | `test/plugin/plugin-pipe-alias-help.ci` |

### What phase 4 measured that the plan did not say

| Finding | Consequence |
|---------|-------------|
| `handleBgpCommandHelp` read `Dispatcher().Lookup`, the BUILTIN table alone. `show command help "<any plugin command>"` answered `unknown command`, so the surface that owes the alias listing could not describe the commands the listing is for. `lookupCommandHelp` in the `system` namespace already read the plugin registry, so the two help surfaces disagreed about which commands exist | The handler reads the plugin command registry after the builtins. Recorded in `plan/journal/unwired-feature.md` |
| `ze cli` with no command argument runs its Bubble Tea model in the CLIENT process, and that model resolves the pipe chain before it sends anything. A declared alias is therefore neither offered by Tab nor resolvable there, and the operator reads `pipe error: unknown pipe operator`. The compiled-in aliases work in the same client, which is what hid it: Tab after the pipe character on `show bgp` offers `summary` and `peers` | AC-10 has no end-to-end test and `test/ui/plugin-pipe-alias-completion.ci` is not written. The two surfaces that answer are the SSH exec channel and the daemon-hosted TUI. The repair is a new channel and a design decision, recorded in `plan/journal/unwired-feature.md` | <!-- doc-links: ignore (fixture never written: no client in the tree drives the daemon-hosted TUI, and no daemon-side completion surface offers a pipe operator) -->
| `system command complete` and `show command complete` complete command NAMES only. Neither offers a pipe operator, so no daemon-side surface answers a completion question about a pipe segment | There is no client-independent way to assert AC-10 end to end today |
| `AliasesForCommand` resolves by longest prefix, so a command inherits what its nearest declared ancestor holds. The help listing inherits with it, which is the same answer completion gives and the same answer the parser gives | The listing needs no rule of its own |

### What phase 4 deliberately does not do

| Left | Consequence today | Owner |
|------|-------------------|-------|
| The channel that carries the daemon's alias table to a CLI client | A declared alias is offered and resolves over the SSH exec channel and in the daemon-hosted TUI, and neither works in `ze cli` interactive | Owner decision. It is a wire surface, a client-side registration whose collision rule differs from the plugin-facing one, and an answer for a plugin that stops mid-session |
| `system command help` reports neither filters nor aliases | It reported neither before this phase, so the two help surfaces are exactly as far apart as they were | Whoever decides whether the two surfaces merge |

## Phase 5 Record: The converted consumer, RPKI (2026-08-22)

`show bgp rpki` is a command, its payload carries the aggregate counters and the
cache server rows as siblings, and `show bgp rpki | summary` answers the same
record `show bgp rpki summary` answers. AC-12 and AC-13 are met.

| What | Where |
|------|-------|
| The bare command an operator types | `internal/component/bgp/plugins/rpki/rpki.go` `overviewCommand`, declared in the Stage 1 `Commands` list and reached from `handleCommand` |
| The aggregate half, written once and read by both answers | `internal/component/bgp/plugins/rpki/rpki.go` `appendSummaryFields` |
| The rows, written once and read by the bare command and by `show bgp rpki status` | `internal/component/bgp/plugins/rpki/rpki.go` `appendCacheServers` |
| The one authored list of aggregate field names | `internal/component/bgp/plugins/rpki/rpki.go` `summaryFieldNames` |
| The expansion, built from that list rather than repeating it | `internal/component/bgp/plugins/rpki/rpki.go` `summaryAliasExpansion`, declared in the Stage 1 `Pipes` list |
| The payload obligation, held by a test | `internal/component/bgp/plugins/rpki/rpki_overview_test.go` `TestOverviewAggregatesMatchSummaryCommand` |
| The expansion and the payload held together | `internal/component/bgp/plugins/rpki/rpki_overview_test.go` `TestSummaryAliasExpansionNamesEverySummaryField`, `TestSummaryFieldNamesMatchTheWrittenPayload` |
| The operator's three answers | `test/plugin/rpki-pipe-summary.ci` |

### What phase 5 measured that the plan did not say

| Finding | Consequence |
|---------|-------------|
| Behavior to change names `cache` as a second alias over the bare command. It cannot be one. `cacheCommand` reports `preference`, `session-id`, `serial`, `refresh-interval`, `retry-interval` and `expire-interval`, and a bare payload carrying those six turns the at-a-glance command into the detail command. A `cache` alias over the shorter row would answer a DIFFERENT record from the `cache` subcommand, under one word | only `summary` is declared. `show bgp rpki cache` stays the one answer to that word, and `show bgp rpki cache \| summary` is refused by the derived barrier |
| The expansion is a second copy of the field list, and nothing holds the two together. A counter added to `summaryCommand` and not to the expansion is dropped by `\| summary` in silence, because `display` names keys and reports no miss | `summaryAliasExpansion` is built from `summaryFieldNames`, and two tests hold the authored list against the bytes the writer produces. This is the payload obligation's other half: R-5 covers a payload that cannot answer the alias, and this covers a payload that grows past it |
| The dispatcher needed no change. `longerCommandPath` (`internal/component/plugin/server/command.go`) refuses the `show bgp` builtin match once `show bgp rpki` is a registered plugin path, and `matchPluginCommand` takes the longest plugin key, so `show bgp rpki status` keeps reaching its own handler | the four-branch guard `test/plugin/show-bgp-child-not-swallowed.ci` covers was written for exactly this, and the new command is the first one to sit ON a branch root rather than below it |
| The three answers do not render in the same ORDER. `registerColumns(cmdBgpChildren)` puts an empty column order on `show bgp rpki`, and a plugin cannot declare one, so `\| table` sorts the keys alphabetically for the bare command and for the subcommand. `\| summary` renders in the expansion's order, because `display` puts the named keys first in the order named | the pipe form is the most readable of the three, and the JSON records are identical. The column-order channel is already a Known Limitation of this spec |
| `validation-enabled` and `running` are literals in a payload whose subject is conditional. `startSessions` clears `active` when no cache server is configured, and the `OnAllPluginsReady` handler then skips the validation gate, while both fields keep reporting true | recorded in `plan/journal/constant-reported-as-measured-state.md`. NOT fixed here: reading them from `active` changes what `show bgp rpki summary` answers, which AC-13 holds unchanged |
| `docs/guide/command-reference.md` used `show bgp rpki` as its example of a branch root that declares nothing, and stated that no command under `show bgp` offers `\| summary` | corrected in this phase rather than left for phase 6, because this phase is what makes the sentence false. The rest of the documentation checklist stays with phase 6 |

### What phase 5 deliberately does not do

| Left | Consequence today | Owner |
|------|-------------------|-------|
| Retiring `show bgp rpki summary` | Two commands answer the same record, and a third spelling, `show bgp rpki \| summary`, answers it too | `plan/audit-command-pipe-vs-subcommand.md`, which owns the consumer list. Behavior to preserve holds the subcommand working |
| A column order for `show bgp rpki` | The bare command and the subcommand render alphabetically | The column-order declaration channel, a separate spec |
| The documentation checklist | The channel and the new command are described in the spec and in one corrected sentence of the command reference | Phase 6 |

## Phase 6 Record: Documentation (2026-08-22)

The channel, its two collision rules, the payload obligation and both known gaps
are written where the next author looks. Every Documentation Update Checklist row
answered Yes is done.

| # | Row | Where it landed |
|---|-----|-----------------|
| 1 | New user-facing feature | `docs/features.md` "Plugin-Declared Pipe Aliases" |
| 3 | CLI command added/changed | `docs/guide/command-reference.md` `### show bgp` and `## Interactive CLI Features`, `docs/guide/cli.md` `## RPKI Commands` |
| 4 | API/RPC added/changed | `docs/architecture/api/commands.md` `### A plugin declares a pipe alias in its Stage 1 message` and its six subsections |
| 5 | Plugin added/changed | `docs/guide/plugins.md` `### Naming a pipe alias for your own command`, `docs/plugin-development/commands.md` `## Naming a Pipe Alias for Your Command` |
| 6 | Has a user guide page | `docs/guide/rpki.md` `## CLI Commands` |
| 8 | Plugin SDK/protocol changed | `ai/rules/plugins.md` `## Runtime Pipe Alias Declaration`, rendered from five new point files. `docs/architecture/api/process-protocol.md` "Pipe Alias Declaration (Stage 1)". `docs/architecture/api/ipc_protocol.md` `### Plugin-Provided Commands`. `docs/plugin-development/protocol.md` Stage 1 field table plus the `PipeDecl` sub-table |
| 10 | Test infrastructure changed | `docs/functional-tests.md` `#### Declaring a pipe alias` |
| 12 | Internal architecture changed | Same as row 4 |
| 15 | Registered plugin or command changed | `docs/plugin-overview.md` and `docs/features/plugins.md`, both `bgp-rpki` rows |
| 16 | Source anchors on changed files | `./le spec citation anchors spec plan/spec-plugin-registers-pipe-operations.md` exits 0. The four declared owners were already named in Files to Modify |
| 17 | Existing examples for this area | `docs/guide/rpki.md` command table gained the three commands it never listed, and the seven counter names were read from `summaryFieldNames` |

### What phase 6 measured that the plan did not say

| Finding | Consequence |
|---------|-------------|
| Two documents stated that an alias is expanded in the CLIENT. `docs/architecture/api/commands.md` said "It is expanded in the client, so a command handler cannot tell an alias from the chain it stands for", and `docs/features/formatting.md` said "It is expanded in the CLI, before the command runs" | Both are corrected. The chain is expanded in the process that PARSES it, and for a plugin's alias that process MUST be the daemon. The false sentence is also the reason the client gap went unseen until phase 4 |
| `docs/architecture/plugin/rib-storage-design.md` is the declared design document of `rpki.go` and carries no RPKI content at all | Nothing was added there. Filing the RPKI command surface under a RIB storage design would put it where no reader looks. The spec names the file, which is what `spec_doc_anchors.py` asks for, and the RPKI command surface is documented in `docs/guide/rpki.md` and `docs/architecture/api/commands.md` |
| `docs/guide/cli.md` carries a second RPKI command table, a duplicate of the one in `docs/guide/rpki.md`. Neither listed the bare command, `show bgp rpki aspa` or `request bgp rpki validate` | Both tables now list the seven commands. The alias is described once, in `docs/guide/rpki.md`, and `docs/guide/cli.md` points at it |
| `docs/architecture/cli/command-completion.md` says nothing about pipe operators at all. It covers command-path completion alone | Left as it is. The alias completion story is one cut, and it is made in `docs/architecture/api/commands.md` "Discovery" beside the surfaces it compares |
| `plan/journal/gate-verdict-depends-on-the-machine.md:35` carries a dead path reference that `./le doc check links` reports. It belongs to another session and is not part of this spec | Not carried. Reported to the owner instead |

### What phase 6 deliberately does not do

| Left | Consequence today | Owner |
|------|-------------------|-------|
| Closure: the review gate, the learned summary, and the two closing commits | The spec stays `in-progress` with every phase implemented | `/ze-close` |
| A daemon-backed source for the wiki command catalog | The published catalog lists a plugin's commands without its aliases, and the two catalogs now say so | The deferral shard |
