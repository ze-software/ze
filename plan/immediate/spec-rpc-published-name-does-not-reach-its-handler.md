# Spec: rpc-published-name-does-not-reach-its-handler

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | - |
| Phase | 2/7 |
| Handoff | - |
| Updated | 2026-10-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the schema promises.** A YANG `rpc` statement declares a callable method,
and two surfaces publish the name Ze derives for it. `cmdMethods`
(`internal/component/config/schema/cli/main.go`) prints that name, its module and
its summary for `ze schema methods`. `Build`
(`internal/component/aihelp/aihelp.go`) puts the same name in the `RPCs` array of
`ze help ai --json`, and the MCP tool `ze_reference`
(`internal/component/mcp/tools.go`) hands that array to a client. An operator or
a plugin that reads the schema and calls the published name gets nothing.
Measured on 2026-09-06 over the 34 `internal/**/*-api.yang` modules against every
`"<prefix>:<name>"` string literal in `internal/`, `cmd/` and `pkg/`: **198
declared rpcs, 129 publishing a method no Go literal carries, across 28 of the 33
modules.** The heaviest are `ze-l2tp-api` (19 of 19), `ze-bgp-cmd-peer-api` (14 of
14), `ze-cli-show-api` (14 of 14), `ze-rib-api` (10 of 13), and
`ze-bgp-cmd-meta-api` and `ze-command-meta-api` (8 of 8 each). Five modules are
clean: `ze-config-archive-api`, `ze-iface-api`, `ze-plugin-api`, `ze-resolve-api`
and `ze-system-api`.

**Which way the count errs, and by how much.** It errs in BOTH directions, and
each direction was measured. It over-reports where a handler registers a method
through a name the scan cannot read as a literal, and where an external plugin
process serves the method: the five `ze-fakeredist-api` methods are declared by a
`ze:command` node in `ze-fakeredist-cmd.yang` and served through the plugin SDK,
with no Go literal anywhere. It under-reports where a literal in that spelling is
not a registration at all. A tighter scan that reads only the `WireMethod:` field
of an `RPCRegistration` answers **132**, and the three names the loose scan
counted as served are `ze-rib:help`, `ze-rib:event-list` and `ze-rib:show`, none
of which any handler answers. The over-report channel is empty for this
population: of those 132 published names, **zero** are declared by a `ze:command`
node either, so none of them is reachable through the plugin route. Neither scan
is exact, which is why the gate below is specified as a runtime comparison.

**What Ze does instead, read at the producer.** `WireModule`
(`internal/component/config/yang/rpc.go`) strips the `-api` suffix from the module
name, and `RegisterRPCs` (`internal/component/plugin/server/schema.go`) joins the
result to the rpc name through `ipc.FormatMethod`. The handler names its own
method independently, as a literal in an `RPCRegistration` value
(`internal/component/plugin/server/rpc_register.go`). Nothing compares the two.
`ze-bgp-cmd-peer-api.yang` declares 14 rpcs and publishes the prefix
`ze-bgp-cmd-peer`, which no handler in the tree uses, while `peer.go`
(`internal/component/bgp/plugins/cmd/peer/`) registers `ze-bgp:peer-pause`,
`ze-bgp:peer-resume` and `ze-bgp:peer-list`. Those three commands work. A third
declaration is why: `ze-peer-cmd.yang` carries `ze:command "ze-bgp:peer-pause"`,
and `LoadBuiltins` (`internal/component/plugin/server/command.go`) keys the
dispatcher on `wireToPath[reg.WireMethod]`, which comes from the `ze:command`
nodes. The command tree routes on the handler's spelling and the schema publishes
a different one. BFD shows the same defect from the other side.
`ze-bfd-cmd.yang` and `bfd.go` (`internal/component/bfd/cmd/`) both name
`ze-bfd-api:show-sessions`, keeping the `-api`, while the schema publishes
`ze-bfd:show-sessions`.

**Why 129 accumulated.** The published name reaches no dispatch path.
`SchemaRegistry.rpcs` is written by `RegisterRPCs` and read by `ListRPCs`, which
serves the two help surfaces. `findRPC`, `findRPCByCommand` and
`registerCLICommand` in the same file have no caller outside
`internal/component/plugin/server/schema_test.go`, so `SchemaRegistry.commands`
is empty in a running daemon and `RegisteredRPC.CLICommand` is always empty. A
wrong published name therefore breaks documentation and never breaks a command,
and no gate reads it. `Validate` (`internal/le/doc/yangcontract/contract.go`) keeps only
modules whose name ends in `-cmd`, so it never opens an `-api` module, and it
computes `orphanLocalHandlers` and then leaves that set out of
`contractSatisfied`. A run on 2026-09-06 printed 394 YANG commands, 365 handlers,
15 local handlers with no YANG command, and the verdict "All commands validated."

**What a reader gets today.** `ze schema methods` prints the published spelling
alone. `printAPICommands` (`cmd/ze/help_ai.go`) prints the published spelling with
an empty dispatch-key column, then prints the handler spellings in a second list
with no description, and nothing marks which of the two answers. `Build` appends
both sets to one `RPCs` array, so an MCP client reads 129 method names that
resolve to nothing beside the names that work. `AllRPCDocs`
(`internal/component/config/yang/cli/tree.go`) starts from the handler's method
and joins the parameter metadata through `loadRPCParams`, whose map is keyed by
the PUBLISHED method, so `ze yang doc` prints no input or output parameters for
any command whose two names disagree. `docs/architecture/api/wire-format.md`
copied the derivation into its "Method Naming" table, and its `ze-rib:show` row
names a method that no handler answers and no `ze:command` node declares.
One measurement is masked in the tree today, and it is a separate defect that
this spec does not fix: on the `ze_core,ze_distro` build of 2026-09-06,
`SchemaRegistry` returns an empty registry after a loader error it discards, so
`ze help ai` reports "0 YANG RPCs" and `ze schema methods` fails with "register
ze-ddos-fake.yang: module ze-fakeddos-conf imports ze-ddos-detect-conf but it is
not available".

**The design, decided by the owner on 2026-09-06.** The `ze:command` node is the
single declaration of a wire method. The `rpc` statement stops naming a method,
and the derivation from the module file name goes away with `WireModule`. Three
questions stay open, and the design has to answer each one.

The FIRST is an rpc with no command node. `ze-plugin-engine.yang` and
`ze-plugin-callback.yang` (`internal/core/ipc/yang/`) declare 13 and 9 rpcs, which
is the plugin IPC protocol. Neither module ends in `-api`, so `APIModuleNames`
(`internal/component/config/yang/loader.go`) never lists them and `SchemaRegistry`
publishes none of them. Their methods carry the FULL module name on the wire:
`startup_driver.go` (`internal/component/plugin/server/`) sends
`ze-plugin-engine:declare-registration`, and `sdk_dispatch.go`
(`pkg/plugin/sdk/`) matches an inbound one by the same spelling. Those 22 names
are the external plugin contract, no node declares them, and no derivation
reaches them, so the design owes them an explicit statement on the rpc itself. A
further 19 rpcs sit in `-api` modules with no `ze:command` node naming them;
they were the sibling spec's, and since the owner decision of 2026-10-08 they are
settled here, among the 117 the gate names.

The SECOND is an rpc reached by more than one node, which happens today. Five
wire methods are declared by more than one `ze:command` statement, and the widest
are `ze-iface:interface-unit-add` and `ze-iface:interface-addr-add`, each declared
three times in `ze-iface-cmd.yang`. `WireMethodToPaths`
(`internal/component/config/yang/command.go`) already models one method with
several paths and calls the extras aliases. So the declaration is the method
STRING, a repeated value is an alias rather than a second declaration, and the
check compares sets of strings and MUST NOT refuse a repeat.

The THIRD is the join from a node to the rpc statement that carries its
description and its parameters, and it cannot be by name. Of the 198 `-api` rpcs,
141 join to exactly one wire method by local name, 38 are ambiguous, and 19 have
no node. The ambiguous set is `help`, `command-list`, `command-help` and
`command-complete`, each declared by six modules and each matching
`ze-bgp:`, `ze-plugin:` and `ze-system:` nodes, plus `summary`, `sessions`,
`session` and `statistics` shared by `ze-l2tp-api`, `ze-pppoe-api` and
`ze-subscriber-api`. The node has to name its rpc, or the rpc statements have to
move into the command modules.

**The isolation requirement, added by the owner on 2026-09-06.** Ze needs a name
clash to be impossible, and one owner MUST NOT be able to take another owner's
command. The guarantee is asymmetric today, and the asymmetry falls the wrong
way. An EXTERNAL plugin is already refused. `CommandRegistry.Register`
(`internal/component/plugin/server/command_registry.go`) checks three grounds
before it stores a name, and returns one `RegisterResult` for each command
naming the ground: `validateCommandName` failed, the name is a builtin
("conflicts with builtin: X"), or another process holds it ("already registered
by process: <name>"). That guarantee is met, and this spec asks only for a test
that keeps it. An INTERNAL component is refused by nothing. `Dispatcher.Register`
and `RegisterWithOptions` (`internal/component/plugin/server/command.go`) both
write `d.commands[key]` with a bare map assignment, no duplicate check and no
return value, so the last `init()` to run wins in silence and the other handler
never executes. Nobody controls the order of `init()` across packages, and
internal components register the several hundred command nodes this spec is
about. The same shape sits one level down: `loadBuiltinsWithAliases` builds
`wireToHandler[reg.WireMethod]` with a bare assignment, so a duplicate wire
method is lost there too.

**What already catches part of this, and what it misses.**
`TestYANGPathsAreUnique` (`internal/component/plugin/server/all_import_test.go`)
imports the composition root and refuses two builtins that map to one CLI path,
and `TestEveryRPCHasYANGPath` beside it refuses a handler with no node. Both are
tests rather than runtime refusals, and neither checks that a wire method is
unique across every linked builtin. `TestRPCRegistrationTable`
(`internal/component/plugin/server/rpc_registration_test.go`) does check wire
method uniqueness, over the `server` package's own registrations alone, which its
own comment states. No collision exists in the tree today. The five `WireMethod`
literals declared twice are `_linux.go` and `_other.go` build-tag pairs, so one
of each is compiled, which is why the runtime check has to read the LINKED set
rather than the source text.

**The convention, and it inverts the earlier advice.** A wire method carries a
prefix, and 397 distinct methods across 24 prefixes fall into three families:
verb-based 249 (`ze-show:`, `ze-set:`, `ze-clear:`, `ze-update:`), owner-based
106 (`ze-bgp:`, `ze-iface:`), and 49 that copied the module name verbatim and
still carry `-api` (`ze-l2tp-api` 20, `ze-rib-api` 9, `ze-pppoe-api` 5,
`ze-fakeredist-api` 5, `ze-bfd-api` 4, and three more). The third family is a
mistake that exists because nothing refuses it. A VERB prefix is a shared pool
and gives no protection: `ze-show:` alone is declared from **43 owner
directories** and carries 211 of the 397 methods, so 43 owners sit in one
namespace and a clash between them is possible by construction. Eight prefixes
are shared by more than one owner today. An OWNER-derived prefix makes a clash
impossible instead of detectable, because the prefix follows from the registering
package and one component cannot spell another's. Enforcement is then
derivational, over where the code lives, rather than a list somebody maintains.
It does not fight the verb-first CLI grammar, because the grammar governs the
PATH and not the method: `ze-bgp:peer-list` already sits under `show bgp peer
list`, and `ze-iface:interface-unit-add` under three `create interface` paths.
The cost is real and belongs beside the benefit: 249 of 397 declarations move,
each with the handler literal beside it, plus the `.ci` expectations and the doc
tables. "Owner" also needs a definition, because `ze-bgp:` is declared today from
four directories (`internal/component/bgp`, `internal/component/cmd`,
`internal/plugins/log`, `internal/plugins/meta`), so the unit is the subsystem
rather than the directory. Two facts settle it. The declaration set is already
collision-free by accident, with zero of 397 methods declared by more than one
owner directory and zero by more than one file, and nothing holds that property
in place. And the 38 name-ambiguous rpcs named above stop being ambiguous under
an owner prefix, because each owner's `help`, `command-list`, `command-help` and
`command-complete` carries its own, while under a verb prefix they stay ambiguous
for good. **This spec concludes for the owner-derived prefix.**

**The boundary with `plan/immediate/spec-yang-rpc-declarations-with-no-handler.md`,
in both directions.** This spec owns the naming contract and the gate. It decides
which declaration carries a method name, it renames nothing an operator types, and
it adds no capability. That spec owns the missing peer capability, which is
peer-add, peer-delete and peer-save, and it decides whether each dead declaration
should exist at all. Where a declaration has a node and a handler that disagree in
spelling, this spec repairs it. Where a declaration has neither, that spec decides
its fate and this spec's gate reports it without deciding. The ordering knot is
resolved (owner, 2026-10-08): the gate no longer waits for that spec to close.
The strengthened gate already landed red in 949091e15a, and it turns green as
this spec repairs the published methods it names. The one exception to "neither
spec edits the other's files" is `ze-bgp-cmd-peer-api.yang`, which this spec
removes after moving its two unique facts.

-> Decision (owner, 2026-10-08): this spec runs next. It absorbs the 117 published methods with no handler found by the command-contract gate (949091e15a), and removes ze-bgp-cmd-peer-api.yang after moving its two unique facts (peer-save's rpc declaration under ze-bgp, and the session input leaf of session-peer-ready) into their final form under this spec's design.
-> Decision (owner, 2026-10-08), Q3: the `ze:command` node carries an explicit pointer to its rpc in the `-api` module, and the rpc names no method. `WireModule` is deleted, and an `-api` module whose rpcs are all dead is deleted.
-> Decision (owner, 2026-10-08), Q4: the owner of a wire prefix is derived from where the registering handler's package lives. There is no central table.
-> Decision (owner, 2026-10-08), Q5: each of the 15 unmatched published methods is repointed to the method that already answers it, and deleted only if none does.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/api/wire-format.md` - declares the "Method Naming" rule this spec retires
  → Decision: the page states that `WireModule` removes the `-api` suffix and that a `ze:command` declaration can set a different wire method. Both sentences change.
  → Constraint: the page's `ze-rib:show` row is wrong today and the edit lands in the same work as the code.
- [ ] `docs/architecture/api/commands.md` - declares that the dispatch key is the YANG path
  → Decision: the path stays the dispatch key. This spec changes the method NAME only.
  → Constraint: the page already warns that a path move is a wire break for a plugin sender.
- [ ] `docs/architecture/api/architecture.md` - repeats the `-api` stripping rule
  → Constraint: the sentence naming `WireModule()` is a second copy of the rule and goes with it.
- [ ] `ai/rules/cli.md` - forbids a compatibility alias
  → Constraint: "Ze is unreleased, so a second spelling MUST be renamed outright rather than aliased".
  → Constraint: "a rename of a programmatic command path breaks the wire, so every programmatic sender MUST be found first".

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfcNNNN.md` - [why relevant]
  → Constraint: [specific RFC rule that applies here]

**Key insights:** (minimal context to resume after compaction)
- Three files can name one method, and the two that agree decide whether the command works.
- The published name reaches documentation surfaces only, which is why the defect stayed invisible.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/config/yang/rpc.go` - `WireModule` strips `-api` and `-conf` from a module name
- [ ] `internal/component/plugin/server/schema.go` - `RegisterRPCs` keys the registry on the derived method; `findRPC`, `findRPCByCommand` and `registerCLICommand` have no non-test caller
- [ ] `internal/component/plugin/server/rpc_register.go` - `RegisterRPCs` collects the handler-declared method from `init()`
- [ ] `internal/component/plugin/server/command.go` - `LoadBuiltins` keys the dispatcher on the CLI path the `ze:command` node gives
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` - registers `ze-bgp:peer-pause` and 13 siblings
- [ ] `internal/component/bgp/plugins/cmd/peer/yang/ze-bgp-cmd-peer-api.yang` - declares 14 rpcs under a prefix no handler uses
- [ ] `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` - carries the `ze:command` value that routes
- [ ] `internal/component/bfd/cmd/bfd.go` - registers `ze-bfd-api:show-sessions`, keeping the suffix
- [ ] `internal/component/aihelp/aihelp.go` - `SchemaRegistry` and `Build` publish the derived name
- [ ] `cmd/ze/help_ai.go` - `printAPICommands` prints the derived name with an empty dispatch column
- [ ] `internal/component/config/schema/cli/main.go` - `cmdMethods` prints the derived name
- [ ] `internal/component/config/yang/cli/tree.go` - `AllRPCDocs` joins parameters through a map keyed by the derived name
- [ ] `internal/le/doc/yangcontract/contract.go` - `Validate` reads `-cmd` modules only; `contractSatisfied` ignores orphan local handlers
- [ ] `internal/le/cli/dispatch/resolver.go` - `newSurface` registers command PATHS, so the gate checks no wire method
- [ ] `internal/core/ipc/yang/ze-plugin-engine.yang` - 13 rpcs whose wire prefix is the full module name
- [ ] `internal/core/ipc/yang/ze-plugin-callback.yang` - 9 rpcs of the same shape
- [ ] `internal/core/ipc/method.go` - `ParseMethod` and `FormatMethod` define the `module:rpc` form
- [ ] `internal/component/plugin/server/command_registry.go` - `Register` refuses an external plugin on three grounds and names each one
- [ ] `internal/component/plugin/server/all_import_test.go` - `TestYANGPathsAreUnique` and `TestEveryRPCHasYANGPath` over the linked set
- [ ] `internal/component/plugin/server/rpc_registration_test.go` - `TestRPCRegistrationTable` checks uniqueness over the server package alone

**Behavior to preserve:** (unless the user explicitly said to change it)
- Every command an operator types keeps its words. The dispatch key stays the YANG path.
- Every plugin IPC method keeps its current spelling, because an external plugin holds it.
- `ze schema methods`, `ze help ai` and `ze yang doc` keep their output shape.

**Behavior to change:** (only what the user asked for)
- A published method name comes from the `ze:command` node instead of the module file name.
- `WireModule` and every derivation from a module file name are deleted.
- The command contract gate compares published methods against registered handlers.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A YANG `ze:command` statement in a `-cmd` module, read by `BuildCommandTree`.
- A `WireMethod` field in an `RPCRegistration` value, read by `AllBuiltinRPCs`.

### Transformation Path
1. `BuildCommandTree` and `WireMethodToPaths` (`internal/component/config/yang/command.go`) read the `ze:command` values into method-to-path pairs.
2. `LoadBuiltins` (`internal/component/plugin/server/command.go`) registers each handler under the path its method maps to.
3. `RegisterRPCs` (`internal/component/plugin/server/schema.go`) indexes the rpc metadata under a method name, derived today and declared after this change.
4. `ListRPCs` serves `ze schema methods`, `ze help ai` and `ze_reference` from that index.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | JSON-RPC line with the method word first (`pkg/plugin/rpc`, `AppendRequest`) | No |
| Daemon ↔ MCP client | `ze_reference` returns the `Build` reference as JSON | No |
| Daemon ↔ operator | `ze schema methods` and `ze help ai` text output | No |

### Integration Points
- `internal/le/doc/yangcontract/contract.go` - the gate that gains the second comparison.
- `internal/le/cli/dispatch/resolver.go` - the neighboring gate that checks command paths and not methods.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No operator-facing wire method travels over a socket as a method word | `LoadBuiltins` keys the dispatcher on the CLI path, and `Hub.RouteCommand` routes on the handler path | A rename breaks a live sender | Grep every `Method:` assignment in `pkg/plugin` and `internal/component/plugin` | confirmed 2026-10-08 with two exceptions, S-4 and S-5 in "Phase 2 Result" |
| A-2 | The 22 plugin IPC methods keep their spelling under the new design | `startup_driver.go` and `sdk_dispatch.go` both hold the literal | An external plugin stops registering | The plugin functional suite under `test/plugin/` | validated 2026-10-08: none of this spec's code commits (c8574d4911, 03f75f251a, e62c16d72e, 949091e15a, 55e1d3bd4c, 756e85f100, 89497c80e4, 07431bb80e, a699ee08a1) touches `pkg/plugin`; the only `"ze-plugin` literal changes in `pkg/plugin` over that range are additions by other work (35a332e5c3, 5ede86fd04), none renamed; `TestPluginIPCMethodsKeepTheirSpelling` green at 55e1d3bd4c (not re-run at closure: `internal/component/command` holds another session's uncommitted edits that do not build) |
| A-3 | The runtime comparison sees every handler the daemon registers | `AllBuiltinRPCs` returns the process registry, so a package nobody imports is invisible | The gate passes on a partial population | An emitter floor in the gate, as `cidispatch` uses | validated 2026-10-08: `internal/le/doc/yangcontract/contract.go` blank-imports `internal/component/plugin/all`; `./le doc yang-contract command-contract` reports 372 registered handlers plus 2 skipped editor-internal ones, 374, which is every line of `internal/component/plugin/all/testdata/wire-methods.snapshot` |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The gate is red on the 117 published methods with no handler | The gate run of 949091e15a lists them | Resolved (owner, 2026-10-08): this spec absorbs the 117, and the gate goes green as this spec lands, without waiting for the sibling spec |
| R-2 | Moving the rpc metadata into the command modules churns 34 YANG files at once | A review that cannot be read | Land the join first, then the file moves, one module group per commit |
| R-3 | A test pins the old published spelling | `test/parse/cli-schema-methods.ci` asserts `ze schema methods` prints `ze-system:help` | Correct the expectation in the same commit as the rename |
| R-4 | The owner-derived prefix moves 249 of 397 declarations, which is a large diff to review | A commit that no reviewer can hold | One subsystem for each commit, with the gate green after each |
| R-5 | "Owner" is defined too narrowly and `ze-bgp:` splits across its four declaring directories | The check refuses a declaration that is correct | Define the unit as the subsystem, and name the subsystem for each directory in one table the check reads |
| R-6 | `Dispatcher.Register` gains a refusal and a startup check finds a collision nobody knew about | The daemon refuses to start | Run the startup check as a test first, in the phase before the refusal lands |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An external plugin fails stage 1 and never registers, or a documented method name stops matching the daemon |
| How is it reverted? | A single commit revert, until the YANG module moves land |
| Who else touches this path? | `plan/immediate/spec-yang-rpc-declarations-with-no-handler.md`, on the same `-api` modules |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le doc yang-contract command-contract` | → | `Validate` in `internal/le/doc/yangcontract/contract.go` | `TestPublishedMethodHasAHandler` |
| `ze schema methods <module>` | → | `cmdMethods` in `internal/component/config/schema/cli/main.go` | `test/parse/cli-schema-methods.ci` |
| `ze yang doc "<command>"` | → | `AllRPCDocs` in `internal/component/config/yang/cli/tree.go` | `TestAllRPCDocsHaveParams` (`internal/component/config/yang/cli/tree_test.go`, needs `-tags ze_bgp`) |
| Daemon startup with every component linked | → | the collision check over the registered set | `TestNoOwnerHoldsAnotherOwnersName` |
| A second builtin registering one name | → | `Dispatcher.Register` in `internal/component/plugin/server/command.go` | `TestDispatcherRefusesADuplicateName` |
| An external plugin declaring a held name | → | `CommandRegistry.Register` in `internal/component/plugin/server/command_registry.go` | `TestCommandRegistryRefusesOnEachGround` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A published wire method that no registered handler answers | `./le doc yang-contract command-contract` names it and the verdict is FAIL |
| AC-2 | A registered handler whose method no declaration publishes | The same run names it and the verdict is FAIL |
| AC-3 | A local handler path that no YANG command node declares | The same run names it and the verdict is FAIL, which 15 rows do not do today |
| AC-4 | The gate reads its two sets | Both come from the live process, one from the command tree and one from `AllBuiltinRPCs`, and neither from a text scan |
| AC-5 | `ze schema methods` and `ze help ai --json` on any module | Each command appears under one method name, and that name is the one the daemon answers |
| AC-6 | `ze yang doc "show bgp peer list"` | The output carries the parameters the rpc declares: `selector` (`peer-selector`) under "Parameters (input)", and output parameters wherever an rpc declares them. Restated 2026-10-08: the AC first named `show bgp peer pause`, now `request peer pause`, whose rpc declared no leaves and was deleted in 03f75f251a, so it can no longer demonstrate parameters. `show bgp peer list` is the command `TestAllRPCDocsHaveParams` asserts, and a ze_core,ze_distro,ze_bgp build prints its `selector` input |
| AC-7 | An external plugin completing stage 1 | It sends `ze-plugin-engine:declare-registration` and the engine accepts it, unchanged |
| AC-8 | A grep for a method derived from a module file name | `WireModule` does not exist, and no surface builds a method from a file name |
| AC-9 | A new `rpc` statement added with no `ze:command` node and no explicit declaration | The gate refuses it, so the count cannot grow again |
| AC-10 | Two owners try to produce one wire method | The second is impossible rather than refused, because the prefix is derived from the registering subsystem, and a check proves each subsystem declares under its own prefix alone |
| AC-11 | Two builtins register one command name | `Dispatcher.Register` refuses the second, returns the refusal, and the message names the owner that holds the name |
| AC-12 | The daemon starts with every component linked | A startup check reads the whole registered set, builtins included, and refuses to serve while any two owners hold one name or one wire method |
| AC-13 | An external plugin declares a name that is a builtin, is already held, or is malformed | `CommandRegistry.Register` refuses it with the ground it failed on, and a test covers each of the three grounds |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An AI agent reads `ze_reference` and calls a method it names | MCP `ze_reference` -> `Build` -> the daemon dispatcher | `test/ui/help-ai-json-methods-answer.ci` (no `test/mcp/` suite exists; `ze help ai --json` runs the same `Build`) |
| 2 | An operator reads `ze schema methods ze-l2tp-api` and calls the name printed | `cmdMethods` -> `ListRPCs` -> the daemon dispatcher | `test/parse/cli-schema-methods.ci` |
| 3 | An operator runs `ze yang doc` for a command and reads its parameters | `AllRPCDocs` -> `loadRPCParams` -> the rpc metadata | `TestAllRPCDocsHaveParams` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPublishedMethodHasAHandler` | `internal/le/doc/yangcontract/contract_test.go` | Every published method answers | |
| `TestHandlerMethodIsPublished` | `internal/le/doc/yangcontract/contract_test.go` | Every registered handler is published | |
| `TestOrphanLocalHandlerFailsTheGate` | `internal/le/doc/yangcontract/contract_test.go` | `contractSatisfied` reads all three sets | |
| `TestAllRPCDocsHaveParams` | `internal/component/config/yang/cli/tree_test.go` | The parameter join finds the metadata | green 2026-10-08 with `-tags ze_core,ze_distro,ze_bgp` (without `ze_bgp` the peer handlers are not linked and it fails on "not found"); planned as `TestRPCDocsCarryParameters`, the existing test stands in |
| `TestBuildPublishesEachRPCOnce` | `internal/component/aihelp/aihelp_test.go` | `ze help ai --json` publishes each method once | red on 21 duplicated methods before a47daeb8d0, green after |
| `TestPluginIPCMethodsKeepTheirSpelling` | `internal/core/ipc/yang/method_test.go` | The 22 IPC methods are unchanged | |
| `TestDispatcherRefusesADuplicateName` | `internal/component/plugin/server/command_test.go` | A second builtin is refused rather than overwriting | green (phase 5); red with the refusal removed |
| `TestCommandRegistryRefusesOnEachGround` | `internal/component/plugin/server/command_registry_test.go` | The three existing refusals cannot regress | green (phase 5); builtin and held subtests red with those two checks removed |
| `TestNoOwnerHoldsAnotherOwnersName` | `internal/component/plugin/server/all_import_test.go` | No collision across the whole linked set | green (phase 5): zero collisions in the linked set |
| `TestEverySubsystemDeclaresUnderItsOwnPrefix` | `internal/le/doc/yangcontract/published_test.go` | A prefix cannot be spelled by a subsystem that does not own it | red with 302 violations before the renames, green after (phase 6) |
| `TestForeignPrefixesNamesEachGround`, `TestAForeignPrefixFailsTheVerdict` | `internal/le/doc/yangcontract/published_test.go` | The gate names a foreign prefix and a package with no subsystem root, and fails on either | phase 6 |
| `TestOwnerPrefixFollowsThePackage`, `TestRegisterRPCsStampsTheRegistrar` | `internal/component/plugin/server/owner_prefix_test.go` | The derivation, and a registrar stamped from the caller rather than the struct | red (undefined) at write, green (phase 6) |
| (status of the rows above) | | `TestPublishedMethodHasAHandler` landed as `TestEveryPublishedMethodHasAHandler` (phase 4); `TestOrphanLocalHandlerFailsTheGate` as `TestAnOrphanLocalHandlerFailsTheVerdict` (949091e15a); `TestPluginIPCMethodsKeepTheirSpelling` green (55e1d3bd4c); `TestRPCDocsCarryParameters` stood in for by `TestAllRPCDocsHaveParams` (see its row); `TestHandlerMethodIsPublished` not confirmed by this agent | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Method length | 1-256 octets | 256 | 0 | 257 |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cli-schema-methods` | `test/parse/cli-schema-methods.ci` | The printed method is the one the daemon answers | phase 7: the stale `ze-rib:show`/`ze-rib:best` expectations (the defect this spec names) replaced by `ze-bgp:rib-routes`/`ze-bgp:rib-best`, plus a `ze-l2tp-api` case rejecting the module spelling |
| `help-ai-json-methods-answer` | `test/ui/help-ai-json-methods-answer.ci` | The `ze help ai --json` RPCs array, which `ze_reference` hands an MCP client, carries handler methods only. No `test/mcp/` suite exists, so it sits beside the other `help ai --json` tests | phase 7; a47daeb8d0 adds a reject on `ze-plugin:session-ready` published twice, the duplicate the closure review found |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | No wire protocol changes. The plugin IPC spelling is preserved by AC-7 | |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/component/config/yang/rpc.go` - delete `WireModule` and its derivation
- `internal/component/plugin/server/schema.go` - `RegisterRPCs` takes the declared method
- `internal/component/config/yang/command.go` - the node carries the join to its rpc
- `internal/component/config/yang/cli/tree.go` - `loadRPCParams` keys on the declared method
- `internal/component/aihelp/aihelp.go` - one method name for each command
- `cmd/ze/help_ai.go` - one list instead of two
- `internal/component/config/schema/cli/main.go` - print the declared method
- `internal/le/doc/yangcontract/contract.go` - the two new comparisons and the verdict
- `internal/component/plugin/server/command.go` - `Dispatcher.Register` and `RegisterWithOptions` refuse a duplicate and name the holder; `loadBuiltinsWithAliases` stops overwriting `wireToHandler`
- `internal/component/plugin/server/command_registry.go` - unchanged behavior, gains the test that keeps its three refusals
- `internal/core/ipc/yang/ze-plugin-engine.yang` - explicit method declaration
- `internal/core/ipc/yang/ze-plugin-callback.yang` - explicit method declaration
- `docs/architecture/api/wire-format.md` - the "Method Naming" section and its table
- `docs/architecture/api/architecture.md` - the sentence naming `WireModule()`
- `docs/architecture/api/commands.md` - the declaration half of "Dispatch keys are YANG paths"

## Files to Create
- `internal/le/doc/yangcontract/published_test.go` - the gate's own tests

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | Every `-api` module loses its derived prefix; the two IPC modules gain an explicit one |
| YANG validation constraints | N-A | No leaf is added |
| YANG custom validators | N-A | No leaf is added |
| CLI commands/flags | No | Every command keeps its words |
| CLI grammar (keyword before value) | No | No grammar changes |
| Editor autocomplete | No | Completion reads the command tree, which is unchanged |
| Functional test for new RPC/API | Yes | `test/parse/cli-schema-methods.ci` |
| Pipe completeness | No | No new command |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Prometheus counters/metrics | N-A | No observable state added |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family surface |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | No capability is added |
| 2 | Config syntax changed? | No | No config leaf changes |
| 3 | CLI command added/changed? | No | Every command keeps its words |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/wire-format.md`, `docs/architecture/api/architecture.md`, `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | No | No plugin is added |
| 6 | Has a user guide page? | | |
| 7 | Wire format changed? | Yes | `docs/architecture/api/wire-format.md` |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/plugin-development/protocol.md`, if the IPC declaration form changes |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC surface |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/documentation-testing.md`, which the gate names on failure |
| 11 | Affects daemon comparison? | No | No feature comparison changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/architecture.md` |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | N-A | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared by this spec's files (`./le spec citation anchors`, 2026-10-08), each read and corrected in the phase that changes its declaring file: `docs/architecture/api/process-protocol.md` (`schema.go`, `command.go`, `command_registry.go`; phases 4 and 5), `docs/architecture/config/yang-config-design.md` (`rpc.go`, `command.go`, `tree.go`, `schema/cli/main.go`; phases 4 and 7), `docs/features/ai-first.md` (`aihelp.go`; phase 7), `docs/guide/mcp/overview.md` (`aihelp.go`; phase 7). Also declared once the stale `docvalid`/`cidispatch` paths were corrected (2026-10-08): `docs/architecture/core-design.md` (`internal/le/doc/yangcontract/contract.go`, read only: the gate's behavior section lives in `docs/contributing/documentation-testing.md`, row 10) and `docs/architecture/cli/command-namespacing.md` (`internal/le/cli/dispatch/resolver.go`, read only: this spec does not change that gate) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The `ze-rib:show` row in `docs/architecture/api/wire-format.md` is wrong today |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- write the gate against the tree as it stands, and let it be red
   - Tests: `TestPublishedMethodHasAHandler`, `TestHandlerMethodIsPublished`, `TestOrphanLocalHandlerFailsTheGate`
   - Files: `internal/le/doc/yangcontract/contract.go`, `internal/le/doc/yangcontract/published_test.go`
   - Verify: the run names the mismatched methods, and the count it prints is the measurement this spec could only bracket at 129 to 132
2. **Phase: Find every programmatic sender** -- before any rename
   - Tests: none; this phase produces the sender list the next phase works from
   - Files: `pkg/plugin/rpc/message.go` (`AppendRequest` puts the method word on the line), `pkg/plugin/sdk/sdk_dispatch.go`, `internal/component/plugin/server/startup_driver.go`, `test/parse/cli-schema-methods.ci`
   - Verify: `./le cli dispatch check` does NOT cover this. `newSurface` (`internal/le/cli/dispatch/resolver.go`) builds its surface from `WireMethodToPaths` and registers the PATHS, so the gate resolves command strings and never a wire method. The sender list is produced by hand and recorded here
3. **Phase: Declare the plugin IPC methods explicitly** -- the 22 rpcs with no node
   - Tests: `TestPluginIPCMethodsKeepTheirSpelling`
   - Files: `internal/core/ipc/yang/ze-plugin-engine.yang`, `internal/core/ipc/yang/ze-plugin-callback.yang`
   - Verify: the spelling on the wire is byte-identical, and the plugin suite under `test/plugin/` stays green
4. **Phase: Move the declaration to the node** -- retire the derivation
   - Tests: the unit tests above, plus `TestAllRPCDocsHaveParams`
   - Files: `internal/component/config/yang/rpc.go`, `internal/component/plugin/server/schema.go`, `internal/component/config/yang/command.go`
   - Verify: `WireModule` is deleted, and the gate from phase 1 turns green for every command that has a node and a handler
   - Status (2026-10-08): the declarations that are dead under the Q3 design are deleted, because each duplicates an rpc a node will point at and holds no leaf the survivor lacks. Modules deleted: `ze-bgp-cmd-peer-api` (after moving peer-save into `ze-bgp-api` and the session input leaf into `ze-plugin-api` session-peer-ready), `ze-bgp-cmd-commit-api`, `ze-bgp-cmd-meta-api`, `ze-bgp-cmd-raw-api`, `ze-bgp-cmd-subscribe-api`, `ze-bgp-cmd-update-api`, `ze-cli-update-api`, `ze-command-meta-api`. Rpcs deleted: `ze-cli-show-api` bgp-peer and system-update-history, `ze-rib-api` help, command-list and event-list, `ze-route-refresh-api` peer-borr, peer-eorr and peer-clear-soft. Gate under the le feature tags: 117 to 70 published methods with no handler.
   - Status (2026-10-08, later): done, and phase 3 with it. `ze:rpc` and `ze:method` declared in `ze-extensions.yang`; 155 `ze:rpc` pointers over 149 methods in 34 `-cmd` modules (76 by exact name, 57 from the pointer table, 16 Q5 repoints). The 28 IPC rpcs (19 engine, 9 callback; "22" was stale) and the 8 notifications declare `ze:method` with their unchanged spelling; `TestPluginIPCMethodsKeepTheirSpelling` (`internal/core/ipc/yang/method_test.go`) also ties seven `pkg/plugin/rpc` Method constants to them. `yang.PublishedRPCs` (`rpc_publish.go`) answers Commands, Protocol, Unnamed and Unlinked (a pointer at a module this binary did not link, which the gate refuses); a pointer at a missing rpc of a loaded module, a malformed target, a pointer with no `ze:command` and one method pointing at two rpcs are `ErrRPCPointer`. `WireModule` deleted; `SchemaRegistry.RegisterRPCs`/`RegisterNotifications` take the rows and refuse one with no method (`ErrRPCUnnamed`); `aihelp`, `ze schema methods`/`events`, `ze yang doc` (`loadRPCParams`) and `cmd/ze/hub` `buildParamMeta` read `PublishedRPCs`. `ze-bgp-api` peer-remove deleted: its address-only contract has no handler, and `ze-delete:bgp-peer` (`RequiresSelector`) already points at `ze-cli-delete-api` bgp-peer. Gate: `./le doc yang-contract command-contract` passes with 0 rpc declarations with no handler. Not run: `test/plugin/` suite, `test/parse/cli-schema-methods.ci`.
   - Superseded: the `ze:rpc` pointer needed `ze-extensions.yang`, and `WireModule` lives in `internal/component/config/yang/rpc.go`; both sit in a directory where another session holds uncommitted work (`loader.go`, `ze-types.yang`), as does `internal/le/doc/yangcontract` (`usage.go`). The node-to-rpc pointer table is in the per-spec state file.
   - Q5 targets, applied with the pointer: log-show to `ze-bgp:log-levels`; cache to `ze-bgp:cache-list` (the `ze-bgp-api` copy survives, `ze-bgp-cmd-cache-api` dies); peer-show to `ze-bgp:peer-detail`; peer-show-capabilities to `ze-bgp:peer-capabilities`; peer-show-statistics to `ze-bgp:peer-statistics`; `ze-bgp-api` peer-remove to `ze-delete:bgp-peer` (then `ze-cli-delete-api` dies); `ze-rib-api` show to `ze-rib-api:routes`; announce to the three `ze-bgp:announce-*` nodes; withdraw to the three `ze-bgp:withdraw-*` nodes; metrics-show to `ze-bgp:metrics-values`; bgp-warnings to `ze-show:warnings`; pki certificates and certificate to `ze-show:pki-certificates` and `ze-show:pki-certificate`.
5. **Phase: Make a clash impossible** -- the refusals and the derivation
   - Tests: `TestDispatcherRefusesADuplicateName`, `TestCommandRegistryRefusesOnEachGround`, `TestNoOwnerHoldsAnotherOwnersName`, `TestEverySubsystemDeclaresUnderItsOwnPrefix`
   - Files: `internal/component/plugin/server/command.go`, `internal/component/plugin/server/command_registry.go`, `internal/le/doc/yangcontract/contract.go`
   - Verify: the collision check runs first as a test and reports zero, which is what the tree holds today; then `Dispatcher.Register` gains its refusal, and the startup check refuses to serve on a collision
   - Status (2026-10-08): the refusals landed. `Dispatcher.Register` and `RegisterWithOptions` return `ErrCommandHeld` naming the holder (a builtin's holder is its wire method). `loadBuiltinsWithAliases` refuses a wire method two builtins carry (`ErrWireMethodHeld`) and a name two owners reach. `NewServer` returns either refusal, and a refused wire-method registration, instead of logging. `newSurface` in `internal/le/cli/dispatch/resolver.go` returns the first refusal. Tests added: `TestDispatcherRefusesADuplicateName`, `TestLoadBuiltinsRefusesADuplicateWireMethod`, `TestLoadBuiltinsRefusesTwoOwnersOnOnePath`, `TestNewServerRefusesABuiltinCollision` (`command_test.go`), `TestCommandRegistryRefusesOnEachGround`, `TestNoOwnerHoldsAnotherOwnersName`. Open: the prefix derivation in `internal/le/doc/yangcontract/contract.go` and `TestEverySubsystemDeclaresUnderItsOwnPrefix`, which follow the Q4 decision
6. **Phase: Move each subsystem to its own prefix** -- one subsystem for each commit
   - Tests: the gate from phase 1 stays green after each commit
   - Files: the `-cmd` modules of one subsystem, and the handler literals beside them
   - Verify: 249 verb-prefixed declarations reach an owner prefix, the 49 that still carry `-api` are corrected, and no prefix is left with two owners
   - Status (2026-10-08): Q4 derivation done. `RegisterRPCs` stamps `RPCRegistration.Registrar` with the caller's package, and `OwnerPrefix` (`internal/component/plugin/server/rpc_register.go`) answers `ze-` plus the directory directly under `internal/component/` or `internal/plugins/`, `-cmd` dropped. The gate's `foreignPrefixes` (`contract.go`) fails the run on any other prefix. 302 of 374 registered methods moved in one mechanical pass (map: session scratch `p6/map.tsv`). A verb prefix becomes the first word of the name with the owner word dropped (`ze-show:ospf-neighbors` to `ze-ospf:show-neighbors`); a foreign owner prefix becomes a qualifier (`ze-rib-api:routes` to `ze-bgp:rib-routes`, `ze-system:help` to `ze-plugin:system-help`, `ze-bgp:log-levels` to `ze-log:bgp-log-levels`). One commit rather than one per subsystem, because the check is all-or-nothing over the linked set.
   - Open: 52 `ze:command` methods with no builtin handler keep their old prefix: local CLI handlers in `internal/plugins/{env,config-cli,config-schema,config-storage,config-yang,debug,diag,explain,skills,support}` and `internal/component/{plugin,bgp/cli}`, and the external fake plugins under `internal/test/plugins/`. No registrar exists for them, so their owner needs a second derivation source (the package that declares the `-cmd` module), and the gate does not judge them yet.
-> Decision: the verb or foreign-owner qualifier keeps every renamed method unique; the prefix is derived and checked, the name after it is a convention.
7. **Phase: Correct the surfaces and the pages** -- one name for each command
   - Tests: `test/parse/cli-schema-methods.ci`, `test/ui/help-ai-json-methods-answer.ci`
   - Files: `internal/component/aihelp/aihelp.go`, `cmd/ze/help_ai.go`, `internal/component/config/schema/cli/main.go`, `internal/component/config/yang/cli/tree.go`, the three `docs/architecture/api/` pages
   - Verify: `ze help ai` prints one list, and every method it names resolves

### Phase 2 Result: Programmatic Sender Inventory (2026-10-08)

Produced by hand, as step 2 requires, from every `"<prefix>:<name>"` string literal in non-test Go under
`internal/`, `cmd/` and `pkg/` that is not a `WireMethod:` field, every `CallRPC`, `AppendRequest`,
`callEngine` and `rpc.Request` method argument, every consumer of `AllBuiltinRPCs` or `ListRPCs`, and
every live `.ci` and `docs/` mention. The scratch lists are under the session scratch directory
(`p2-go-lines.txt`, `p2-ci.txt`).

| # | Class | Sender (file, symbol) | Method spelling it sends or prints | Receiver | What a rename of an operator method owes here |
|---|-------|----------------------|------------------------------------|----------|-----------------------------------------------|
| S-1 | Plugin IPC, engine side | `startup_driver.go` (`methodDeclareCapabilities`, `methodReady`), `codec.go` (`internal/component/bgp/server/`, 5 codec RPCs), `internal/component/plugin/ipc/rpc.go` (24 literals), `dispatch_registry.go` (`engineOps`, through the `rpc.Method*` constants) | `ze-plugin-engine:*`, `ze-plugin-callback:*` | the external plugin and the engine | Nothing. AC-7 keeps all 22 IPC names byte-identical |
| S-2 | Plugin IPC, SDK side | `pkg/plugin/sdk/sdk.go`, `sdk_engine.go`, `sdk_dispatch.go` (21), `pkg/plugin/rpc/types.go` (16) and `state.go` (5) | `ze-plugin-engine:*`, `ze-plugin-callback:*` | the engine | Nothing, same reason. 89 of the 103 non-registration literals are IPC |
| S-3 | Plugin IPC, test fixture | `internal/test/fixture/plugin_fixture_11.go` | `ze-plugin-callback:configure`, `share-registry`; reads `ze-plugin-engine:declare-capabilities`, `ready` | the engine | Nothing |
| S-4 | Plugin to engine, operator method | `formatFlushRequest` (`internal/exabgp/bridge/bridge_muxconn.go`) | `ze-bgp:peer-flush` | `dispatchPluginRPC` holds no such method (journal `unwired-feature.md`, 2026-10-08) | The only live sender of an operator method word. Rename it with `peer.go`, and settle the receiver first |
| S-5 | API socket client, operator method | `newSocketReloadNotifier` (`internal/component/cli/reload.go`) | `ze-system:daemon-reload` | no reader: no non-test caller, and `server.go` says nothing dispatches through `rpcDispatcher` (same journal row) | Rename with `system.go`; it reaches no daemon today |
| S-6 | Plugin bridge callback match | `internal/exabgp/bridge/bridge.go` (case arms) | `ze-plugin-callback:deliver-batch`, `deliver-event` | the bridge itself | Nothing (IPC) |
| S-7 | Handler-side constants (registrations, not senders) | `internal/plugins/vrrp/cmd_show.go` (4), `internal/component/mtu/cmd/register.go`, `internal/component/bgp/plugins/cmd/monitor/monitor.go` (`WireMethod` const) | `ze-show:vrrp*`, `ze-clear:vrrp-statistics`, `ze-show:mtu`, `ze-bgp:monitor` | the dispatcher, through `WireMethodToPaths` | Rename beside the YANG node, like every `WireMethod:` literal |
| S-8 | Name lists that hold a method | `internal/component/command/grammar/checker.go` (`ze-bgp:help`, the `ze-bgp:plugin-` prefix), `internal/le/doc/yangcontract/contract.go` (`ze-editor:mode-command`, `ze-editor:mode-edit`) | as listed | the grammar checker and the contract gate | Each is a second declaration of a method; renaming the method owes the same edit here |
| S-9 | Published surfaces (print, never send) | `cmdMethods` and `schema_data.go` (`internal/component/config/schema/cli/`), `Build` (`internal/component/aihelp/aihelp.go`) feeding MCP `ze_reference`, `printAPICommands` (`cmd/ze/help_ai.go`), `cmd/ze/help_command.go`, `cmd/ze/ze_core_dispatch.go`, `AllRPCDocs` and `doc.go` (`internal/component/config/yang/cli/`), `internal/component/cli/client/main.go` and `verb_tree.go`, `internal/component/command/usage.go`, `ensure.go` (`internal/component/plugin/server/`), `internal/le/cli/catalog`, `internal/le/cli/list`, `internal/le/cli/grammar/cligrammar.go`, `internal/le/doc/yangcontract` (`command_render.go`, `command_surfaces.go`, `helpshape.go`, `report.go`, `usage.go`), `internal/le/site` (`derived.go`, `equivalents.go`, `equivalentdetail.go`) | whatever `WireMethod` or `ListRPCs` holds | an operator, an MCP client, a generated page | Nothing by hand: each reads the registry, so a rename reaches it, and phase 7 makes each print one name |
| S-10 | Functional tests | 920 live `.ci` files carry a `prefix:name` string, of which the `ze-bgp:`, `ze-conf:`, `ze-fw:`, `ze-bfd:`, `ze-sysctl:`, `ze-image:`, `ze-dns:`, `ze-tftp:` `terminator` and `timeout` keys are runner options, not methods. The method mentions left are 92 in 66 files, mostly `test/plugin/*-show.ci` naming the `ze-show:` handler, plus `test/parse/cli-schema-methods.ci` and `cli-schema-events.ci` (published spellings: `ze-system:help`, `ze-rib:show`, `ze-plugin:session-ping`, `ze-bgp:peer-list`, `ze-system-api:exit`), `test/ui/help-ai-json-rpc-leaf-texts.ci` and `test/plugin/mcp-tools-list-argument-texts.ci` (`ze-system:command-help`) | as listed | the runner's expectations | Each is test data: a rename that changes it needs the owner's approval per `ai/rules/testing.md` |
| S-11 | Go unit tests | 157 `_test.go` files carry a method literal | as listed | assertions | Moves with the rename in the same commit; any that pins a published spelling is R-3 |
| S-12 | Documentation | 24 `docs/` pages carry an operator method: `docs/architecture/api/architecture.md`, `docs/architecture/hub-api-commands.md`, `docs/architecture/command-ownership.md`, `docs/architecture/cli/command-verbs.md`, `docs/architecture/exabgp-bridge.md`, `docs/architecture/resolve.md`, `docs/architecture/testing/ci-format.md`, `docs/functional-tests.md`, `docs/contributing/documentation-testing.md`, `docs/guide/command-reference.md`, `docs/guide/command-catalogue.md`, `docs/guide/config-reload.md`, `docs/guide/configuration.md`, and the ospf, isis, traffic, anomaly, storage, dns, peeringdb and flow-export pages; 17 more carry only IPC names | as listed | a reader | Each page edit lands with the rename that makes it wrong (`ai/rules/documentation.md`) |

-> Decision: A-1 is CONFIRMED for this tree with two exceptions, S-4 and S-5. No other code path puts an operator
method word on a socket: `ze cli`, the web UI, MCP and SSH dispatch by command PATH (through
`ze-plugin-engine:dispatch-command` for a plugin), so renaming a `ze-show:`/`ze-bgp:` method moves no operator's
command. The only external contract is S-1 to S-3, which AC-7 freezes.

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Feature completeness | The gate reads three sets and the verdict reads all three |
| Correctness | No alias and no fallback spelling survives anywhere (`ai/rules/no-layering.md`) |
| Naming | Each command carries exactly one method name in every surface |
| Data flow | The method comes from the command tree, and no code derives it from a file name |
| Rule: `ai/rules/cli.md` | Every programmatic sender was found before the rename, and none was left on the old spelling |
| Rule: `ai/rules/evidence.md` | The gate fails closed: an empty registry refuses rather than passes |
| Isolation | No registration path writes a name with a bare map assignment. Each one refuses and names the holder |
| Isolation | The prefix is derived from the registering subsystem, so no owner can spell another's prefix |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| The gate reports and fails | `./le doc yang-contract command-contract` |
| `WireModule` is gone | `gopls references` on the symbol returns nothing, and a grep finds no caller |
| One method name for each command | `ze help ai --json` piped through a check that finds no duplicate command |
| The IPC spelling is unchanged | `git diff` over `pkg/plugin/` shows no method literal changed |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | `ParseMethod` (`internal/core/ipc/method.go`) bounds the method at 256 octets and refuses whitespace, and the declared method goes through it |
| Authorization | `WireMethodToPath` feeds the authz context, so a method that maps to two paths MUST stay deterministic |

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
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->
- Three files can name one method, and the command works when any two of them agree. That is why a wrong published name is invisible from the CLI.
- A published name that reaches no dispatch path is documentation, and documentation with no consumer inside the process drifts without a gate.
- 2026-10-08, implementation start. Phase 1's gate already exists: the sibling spec moved it to
  `internal/le/doc/yangcontract/contract.go` (949091e15a), so the `internal/le/docvalid/` paths in
  this spec are stale. `./le doc yang-contract command-contract` after `./le --update`: 418 YANG
  commands, 372 handlers, 40 local handlers, verdict FAIL on 117 rpc declarations with no handler,
  0 local orphans.
  -> Constraint: a join by method LOCAL name fails even inside one `-cmd` module. 19 `-cmd` modules
  declare two or three methods that share a local name under different prefixes (`ze-ping-cmd`:
  `ze-monitor:ping`, `ze-resolve:ping`, `ze-show:ping`; `ze-cli-show-cmd`: `summary` under
  `ze-l2tp-api`, `ze-pppoe-api`, `ze-subscriber-api`; `ze-debug-cmd`, `ze-resolve-cmd`,
  `ze-firewall-irr-cmd`, ...). So the third open question cannot be answered by "move the rpc
  statements into the command modules" alone: the node must name its rpc explicitly either way.
  -> OPEN (blocks phase 4, which is what turns the gate green; no owner answer recorded): how the
  node names its rpc. Proposed pick: the node carries an explicit pointer to the rpc statement it
  documents, the rpc names no method, and `WireModule` goes.
  -> OPEN (blocks phase 6 and AC-10; no owner answer recorded): how "owner" is derived for the
  owner prefix. R-5's "one table the check reads" is a central enumeration (`ai/rules/principles.md`).

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The `ze:command` node is the single declaration of the wire method, and the rpc statement stops naming one (owner, 2026-09-06) | Rename the modules so the derivation matches the handlers. Blast radius: 28 modules, and the shape does not fit. One module publishes one prefix while its handlers use several, so `ze-bgp-cmd-peer-api` alone would have to split across `ze-bgp`, `ze-plugin`, `ze-delete`, `ze-update` and `ze-show`. Each rename also touches the file name, the `module` statement, the namespace, the prefix, every importer and every Go embed registration | The node is already the spelling that routes, so this makes the working declaration the only one. No file name carries meaning after it |
| | Keep the handler prefix authoritative and let the rpc statement carry it explicitly. Blast radius: 198 rpc statements gain a prefix statement, plus the three `WireModule` call sites in `schema.go` and `tree.go`. Wire effect: none, because the names would be set to what the handlers already answer | Rejected: it keeps two declarations of one fact and only makes them agree, so the next edit can separate them again |
| | Option 3 as chosen. Blast radius: 34 `-api` modules lose their method identity, 22 IPC rpcs need an explicit declaration, 19 nodeless rpcs must be settled first, and the join from node to rpc must be built because a name join is ambiguous for 38 of 198. Wire effect: none for an operator command, because the dispatch key is the path; the IPC methods are preserved by AC-7 | Chosen |
| The prefix is derived from the owning subsystem, so `ze-bgp:`, not `ze-show:` | A verb-based prefix, which is 249 of 397 declarations today and matches the verb-first CLI grammar. Blast radius of keeping it: none. Cost: `ze-show:` is one pool of 43 owner directories, so a clash stays possible by construction and can only be detected | Rejected. The owner requirement is that a clash be IMPOSSIBLE, and a shared pool cannot give that. It also leaves the 38 ambiguous rpc names ambiguous for good |
| | An owner-derived prefix. Blast radius: 249 declarations move with the handler literal beside each, plus the `.ci` expectations and the doc tables. "Owner" is the subsystem, because `ze-bgp:` is declared from four directories today | Chosen. The prefix follows from the registering package, so one owner cannot spell another's, and the check is derivational rather than a maintained list. The verb-first grammar is untouched, because it governs the path |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- `plan/immediate/spec-yang-rpc-declarations-with-no-handler.md` keeps the peer-add, peer-delete and peer-save capability (its RIB-effect proofs and the peer-create/peer-delete rename). The 117 published methods with no handler, the nodeless `-api` rpcs among them, are this spec's since the owner decision of 2026-10-08.
- `SchemaRegistry` returning an empty registry after a discarded loader error is a separate defect and is not fixed here.
- `findRPC`, `findRPCByCommand` and `registerCLICommand` have no non-test caller. Deleting them is not in this spec.

## RFC Documentation (Scope: protocol)

N-A. No RFC governs the method naming of Ze's own plugin IPC.

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
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
