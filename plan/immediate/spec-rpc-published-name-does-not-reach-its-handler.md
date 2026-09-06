# Spec: rpc-published-name-does-not-reach-its-handler

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-06 |

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
and no gate reads it. `Validate` (`internal/le/docvalid/contract.go`) keeps only
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
further 19 rpcs sit in `-api` modules with no `ze:command` node naming them, and
those belong to the sibling spec named below.

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
its fate and this spec's gate reports it without deciding. The gate lands after
that spec closes, or it starts red on the 19 declarations that spec owns. Neither
spec edits the other's files.

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
- [ ] `internal/le/docvalid/contract.go` - `Validate` reads `-cmd` modules only; `contractSatisfied` ignores orphan local handlers
- [ ] `internal/le/cidispatch/resolver.go` - `newSurface` registers command PATHS, so the gate checks no wire method
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
- `internal/le/docvalid/contract.go` - the gate that gains the second comparison.
- `internal/le/cidispatch/resolver.go` - the neighboring gate that checks command paths and not methods.

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
| A-1 | No operator-facing wire method travels over a socket as a method word | `LoadBuiltins` keys the dispatcher on the CLI path, and `Hub.RouteCommand` routes on the handler path | A rename breaks a live sender | Grep every `Method:` assignment in `pkg/plugin` and `internal/component/plugin` | unvalidated |
| A-2 | The 22 plugin IPC methods keep their spelling under the new design | `startup_driver.go` and `sdk_dispatch.go` both hold the literal | An external plugin stops registering | The plugin functional suite under `test/plugin/` | unvalidated |
| A-3 | The runtime comparison sees every handler the daemon registers | `AllBuiltinRPCs` returns the process registry, so a package nobody imports is invisible | The gate passes on a partial population | An emitter floor in the gate, as `cidispatch` uses | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The gate goes red on the 19 declarations the sibling spec owns | The first gate run lists them | Land the gate after that spec closes |
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
| `./le docvalid command-contract` | → | `Validate` in `internal/le/docvalid/contract.go` | `TestPublishedMethodHasAHandler` |
| `ze schema methods <module>` | → | `cmdMethods` in `internal/component/config/schema/cli/main.go` | `test/parse/cli-schema-methods.ci` |
| `ze yang doc "<command>"` | → | `AllRPCDocs` in `internal/component/config/yang/cli/tree.go` | `TestRPCDocsCarryParameters` |
| Daemon startup with every component linked | → | the collision check over the registered set | `TestNoOwnerHoldsAnotherOwnersName` |
| A second builtin registering one name | → | `Dispatcher.Register` in `internal/component/plugin/server/command.go` | `TestDispatcherRefusesADuplicateName` |
| An external plugin declaring a held name | → | `CommandRegistry.Register` in `internal/component/plugin/server/command_registry.go` | `TestCommandRegistryRefusesOnEachGround` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A published wire method that no registered handler answers | `./le docvalid command-contract` names it and the verdict is FAIL |
| AC-2 | A registered handler whose method no declaration publishes | The same run names it and the verdict is FAIL |
| AC-3 | A local handler path that no YANG command node declares | The same run names it and the verdict is FAIL, which 15 rows do not do today |
| AC-4 | The gate reads its two sets | Both come from the live process, one from the command tree and one from `AllBuiltinRPCs`, and neither from a text scan |
| AC-5 | `ze schema methods` and `ze help ai --json` on any module | Each command appears under one method name, and that name is the one the daemon answers |
| AC-6 | `ze yang doc "show bgp peer pause"` | The output carries the input and output parameters the rpc declares |
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
| 1 | An AI agent reads `ze_reference` and calls a method it names | MCP `ze_reference` -> `Build` -> the daemon dispatcher | `test/mcp/reference-methods-answer.ci` |
| 2 | An operator reads `ze schema methods ze-l2tp-api` and calls the name printed | `cmdMethods` -> `ListRPCs` -> the daemon dispatcher | `test/parse/cli-schema-methods.ci` |
| 3 | An operator runs `ze yang doc` for a command and reads its parameters | `AllRPCDocs` -> `loadRPCParams` -> the rpc metadata | `TestRPCDocsCarryParameters` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPublishedMethodHasAHandler` | `internal/le/docvalid/contract_test.go` | Every published method answers | |
| `TestHandlerMethodIsPublished` | `internal/le/docvalid/contract_test.go` | Every registered handler is published | |
| `TestOrphanLocalHandlerFailsTheGate` | `internal/le/docvalid/contract_test.go` | `contractSatisfied` reads all three sets | |
| `TestRPCDocsCarryParameters` | `internal/component/config/yang/cli/tree_test.go` | The parameter join finds the metadata | |
| `TestPluginIPCMethodsKeepTheirSpelling` | `internal/core/ipc/yang/method_test.go` | The 22 IPC methods are unchanged | |
| `TestDispatcherRefusesADuplicateName` | `internal/component/plugin/server/command_test.go` | A second builtin is refused rather than overwriting | |
| `TestCommandRegistryRefusesOnEachGround` | `internal/component/plugin/server/command_registry_test.go` | The three existing refusals cannot regress | |
| `TestNoOwnerHoldsAnotherOwnersName` | `internal/component/plugin/server/all_import_test.go` | No collision across the whole linked set | |
| `TestEverySubsystemDeclaresUnderItsOwnPrefix` | `internal/le/docvalid/published_test.go` | A prefix cannot be spelled by a subsystem that does not own it | |

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
| `cli-schema-methods` | `test/parse/cli-schema-methods.ci` | The printed method is the one the daemon answers | |
| `reference-methods-answer` | `test/mcp/` | Every method `ze_reference` names resolves | |

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
- `internal/le/docvalid/contract.go` - the two new comparisons and the verdict
- `internal/component/plugin/server/command.go` - `Dispatcher.Register` and `RegisterWithOptions` refuse a duplicate and name the holder; `loadBuiltinsWithAliases` stops overwriting `wireToHandler`
- `internal/component/plugin/server/command_registry.go` - unchanged behavior, gains the test that keeps its three refusals
- `internal/core/ipc/yang/ze-plugin-engine.yang` - explicit method declaration
- `internal/core/ipc/yang/ze-plugin-callback.yang` - explicit method declaration
- `docs/architecture/api/wire-format.md` - the "Method Naming" section and its table
- `docs/architecture/api/architecture.md` - the sentence naming `WireModule()`
- `docs/architecture/api/commands.md` - the declaration half of "Dispatch keys are YANG paths"

## Files to Create
- `internal/le/docvalid/published_test.go` - the gate's own tests

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
| 16 | Any changed source file referenced by existing doc source anchors? | | DERIVED, do not answer from memory: `./le spec citation anchors spec plan/<this-spec>.md` lists them |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The `ze-rib:show` row in `docs/architecture/api/wire-format.md` is wrong today |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- write the gate against the tree as it stands, and let it be red
   - Tests: `TestPublishedMethodHasAHandler`, `TestHandlerMethodIsPublished`, `TestOrphanLocalHandlerFailsTheGate`
   - Files: `internal/le/docvalid/contract.go`, `internal/le/docvalid/published_test.go`
   - Verify: the run names the mismatched methods, and the count it prints is the measurement this spec could only bracket at 129 to 132
2. **Phase: Find every programmatic sender** -- before any rename
   - Tests: none; this phase produces the sender list the next phase works from
   - Files: `pkg/plugin/rpc/message.go` (`AppendRequest` puts the method word on the line), `pkg/plugin/sdk/sdk_dispatch.go`, `internal/component/plugin/server/startup_driver.go`, `test/parse/cli-schema-methods.ci`
   - Verify: `./le ci-dispatch check` does NOT cover this. `newSurface` (`internal/le/cidispatch/resolver.go`) builds its surface from `WireMethodToPaths` and registers the PATHS, so the gate resolves command strings and never a wire method. The sender list is produced by hand and recorded here
3. **Phase: Declare the plugin IPC methods explicitly** -- the 22 rpcs with no node
   - Tests: `TestPluginIPCMethodsKeepTheirSpelling`
   - Files: `internal/core/ipc/yang/ze-plugin-engine.yang`, `internal/core/ipc/yang/ze-plugin-callback.yang`
   - Verify: the spelling on the wire is byte-identical, and the plugin suite under `test/plugin/` stays green
4. **Phase: Move the declaration to the node** -- retire the derivation
   - Tests: the unit tests above, plus `TestRPCDocsCarryParameters`
   - Files: `internal/component/config/yang/rpc.go`, `internal/component/plugin/server/schema.go`, `internal/component/config/yang/command.go`
   - Verify: `WireModule` is deleted, and the gate from phase 1 turns green for every command that has a node and a handler
5. **Phase: Make a clash impossible** -- the refusals and the derivation
   - Tests: `TestDispatcherRefusesADuplicateName`, `TestCommandRegistryRefusesOnEachGround`, `TestNoOwnerHoldsAnotherOwnersName`, `TestEverySubsystemDeclaresUnderItsOwnPrefix`
   - Files: `internal/component/plugin/server/command.go`, `internal/component/plugin/server/command_registry.go`, `internal/le/docvalid/contract.go`
   - Verify: the collision check runs first as a test and reports zero, which is what the tree holds today; then `Dispatcher.Register` gains its refusal, and the startup check refuses to serve on a collision
6. **Phase: Move each subsystem to its own prefix** -- one subsystem for each commit
   - Tests: the gate from phase 1 stays green after each commit
   - Files: the `-cmd` modules of one subsystem, and the handler literals beside them
   - Verify: 249 verb-prefixed declarations reach an owner prefix, the 49 that still carry `-api` are corrected, and no prefix is left with two owners
7. **Phase: Correct the surfaces and the pages** -- one name for each command
   - Tests: `test/parse/cli-schema-methods.ci`, `test/mcp/reference-methods-answer.ci`
   - Files: `internal/component/aihelp/aihelp.go`, `cmd/ze/help_ai.go`, `internal/component/config/schema/cli/main.go`, `internal/component/config/yang/cli/tree.go`, the three `docs/architecture/api/` pages
   - Verify: `ze help ai` prints one list, and every method it names resolves

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
| The gate reports and fails | `./le docvalid command-contract` |
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
- The 19 `-api` rpcs that no `ze:command` node names are not settled here. `plan/immediate/spec-yang-rpc-declarations-with-no-handler.md` owns them, and it owns the peer-add, peer-delete and peer-save capability.
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
