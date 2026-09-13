# Spec: plugin-query-mode

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 5/5 |
| Handoff | - |
| Updated | 2026-09-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The owner's requirement, 2026-09-07, verbatim.**

> "we may need to have way to ask plugin for their details without starting them"

and, correcting a misreading of that first sentence:

> "no, it must be started but we should have a way to ask it for information in
> a way where it is aware that it is not being started but queries, for example
> not inialising any data or starting connection/binding/etc."

**What that asks for.** The plugin process IS started. A mode tells the plugin
that it is being interrogated rather than run. The plugin answers with its
declarations and performs none of a live start's work: no data initialisation,
no connection, no socket bind, no listener, no timer.

**Which plugins still need it, measured 2026-09-08.** An in-tree plugin's
command and pipe declarations are ALREADY readable with no process: the closed
`spec-daemon-backed-command-catalog` put the same `commandDecls()` slice on
`registry.Registration`, and `Collect` (`internal/le/command/list/commandlist.go`)
reads it today. The population still trapped behind a live start is the
EXTERNAL plugin, whose code lives in another binary and therefore has no
compiled-in twin. That is what this spec serves.

**One fact, one declaration.** This spec adds no second copy of anything. There
is one declaration, each plugin's own `commandDecls()` in its own package, and
query mode emits it as the SAME Stage 1 `declare-registration` message a running
daemon receives, written to stdout instead of to the hub connection. No manifest,
no build-time emission, no generated list, nothing that can disagree with the
plugin (`ai/rules/principles.md`).

**Prior art, not a dependency.** `spec-daemon-backed-command-catalog` rejected a
collector that would start engines to introspect them, on the evidence of an
audit of all registered runners. It closed on 2026-09-08 and its text lives in
`plan/learned/007-declaration-on-the-registration.md`. This spec transcribes the
part of it that bears on query mode.

## Required Reading

<!-- NEVER tick [ ] to [x]. -->

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - the five startup stages and what each carries
  -> Constraint: Stage 1 `declare-registration` is the FIRST engine call, carries the whole `DeclareRegistrationInput`, and is sent unconditionally even for an empty registration. Query mode emits that same message and nothing else
  -> Constraint: the wire format is newline-framed `#<id> <method> <json>`, so an answer written to stdout in that framing is the protocol's own format rather than a second one
  -> Constraint: each stage has a 5-second budget (`defaultStageTimeout`, `internal/component/plugin/server/server.go`, overridden by `ze.plugin.stage.timeout`). The query budget is set from the same number
- [ ] `docs/architecture/api/commands.md` - where a command is served, and the local-data command the reader is one of
  -> Constraint: a local-data handler returns DATA rather than finished text, "which is what lets `| json`, `| yaml` and `| table` be three renderings of one payload". AC-8 asserts that property over this command's own answer
- [ ] `docs/architecture/cli/plugin-modes.md` - internal and external plugin modes
  -> Decision: the page names three modes (CLI, engine decode, engine). Query mode is a FOURTH and the page gains a row; `--features` and `--yang` are the existing precedent for a mode answered before any connection
- [ ] `docs/architecture/plugin/plugin-system.md` - registration, discovery, process boundary
  -> Constraint: removing a plugin must remove every one of its features, so the reader spells no plugin name and walks the registry and the config instead
- [ ] `docs/plugin-development/protocol.md` - what a plugin author is told the protocol is
  -> Constraint: this is where a third-party author learns query mode exists and which SDK entry point makes their plugin answer it
- [ ] `ai/rules/plugins.md` - what a plugin owns and what a generic package must not spell
  -> Constraint: no plugin spelling in a generic package; the reader is registered by the plugin component, which already owns `show plugin list`
- [ ] `ai/rules/cli.md` - command grammar, answer shape, pipes
  -> Constraint: the reader answers with structured rows through `MustRegisterLocalData`, so `json`, `yaml` and `table` are three renderings of one payload; a row's state is a FIELD, never a character glued to the name
- [ ] `ai/patterns/plugin.md` - the structural template for a plugin and its runner

### RFC Summaries (Scope: protocol)
N-A. No wire protocol and no RFC obligation: the protocol here is Ze's own
plugin RPC.

**Key insights:** (minimal context to resume after compaction)
- The declarations of an IN-TREE plugin are already readable with no process. Only the EXTERNAL plugin needs interrogating.
- The dirty work is in the runner body BEFORE `p.Run`, which no protocol stage gates. Query mode never enters a runner body, so that work is out of scope and named as a follow-up spec.
- Five states must be distinguishable, and "declared none" is not "sent nothing".

## Current Behavior (MANDATORY)

**Source files read:** (2026-09-07 at the producers, re-measured 2026-09-08)
- [ ] `pkg/plugin/rpc/types.go` - `CommandDecl` carries 11 wire fields: name, description, long-help, help (retired), args, completable, hidden, deprecated-names, shape, columns, address-fields. `PipeDecl` carries command, name, description, expansion. Every one is a static property of the plugin's source
  -> Constraint: the answer is this value, encoded once by `encoding/json` through the tags these types already carry
- [ ] `pkg/plugin/sdk/sdk.go` - `(*Plugin).Run` runs the five stages in a straight line: Stage 1 `declare-registration` is the first engine call, `OnConfigure` fires at Stage 2, `OnStarted` after Stage 5. `Registration` is `Run`'s SECOND ARGUMENT: there is no field, setter or getter for it on `Plugin`, so the value is built at the `p.Run` call site inside the runner body. `Run` mutates one field of the caller's copy, `WantsValidateOpen`, derived from the callbacks registered before it
  -> Constraint: the SDK cannot fetch a plugin's declaration without the runner body having run. A query-mode entry point must therefore take the declaration and the activation function as SEPARATE arguments, which is what makes activation unreachable rather than discouraged
  -> Constraint: `NewWithConn` decides `IsInternal` by type-asserting `rpc.Bridger`; `NewWithIO` and `NewFromTLSEnv` can never be internal
- [ ] `internal/component/plugin/cli/main.go` - `cli.Run` looks the plugin up with `registry.Lookup(args[0])` and holds the whole `*registry.Registration` BEFORE it calls `reg.CLIHandler(args[1:])`. `test` and `help` are the only reserved words
  -> Decision: this is where query mode is answered for the ze binary. Nothing of the plugin's own code has run at that point, so inertness is a property of unreachability rather than of a promise
- [ ] `internal/component/plugin/cli/cli.go` - `RunPlugin` answers `--features` and `--yang` and returns 0 before any connection; `connFromEnv()` and `cfg.RunEngine(conn)` come after. `PluginConfig` carries `Name`, `Features`, `RunEngine`, and NOT the registration
  -> Constraint: the pre-connection answer already exists as a shape; the registration it would need is held one level up, in `cli.Run`
- [ ] `internal/component/plugin/process/process.go` - `(*Process).startExternal` runs the config's `run` string under a shell, so no argv can be appended by the forker; its env is `os.Environ()` plus one `append` of the five `ZE_PLUGIN_*` variables, so a sixth variable is a one-line addition. `(*Process).monitorCmd` discards the child's exit status
  -> Decision: the carrier is an environment variable, because argv cannot reach a shell-quoted run string. The daemon's own start path is NOT changed: the reader forks the run string itself
  -> Constraint: a query reader cannot learn "I could not answer" from a `Process` exit code, because that path discards it. The reader forks and waits itself
- [ ] `internal/component/plugin/registry/registry.go` - `Registration` carries `Commands []rpc.CommandDecl` and `Pipes []rpc.PipeDecl` for an in-tree plugin, plus `RunEngine func(conn net.Conn) int` and `CLIHandler func(args []string) int`. There is no `Internal` field: internal against external is `p.config.Internal`, read at `(*Process).Start`
- [ ] `internal/component/plugin/register.go` - `show plugin list` writes one row per plugin and NEVER drops a row it could not read: a plugin that recorded a setup outcome and never completed `Register` keeps its row with `descriptionUnregistered`, and a registered plugin that recorded nothing keeps its row with the unknown outcome. It is registered with `MustRegisterLocalData`, `Mode: modeOffline`, and declares its shape and column order
  -> Decision: this is the structural template for the reader. Same package, same registration call, same never-omit rule
- [ ] `internal/le/command/list/commandlist.go` - `Collect` walks `pluginregistry.All()` and reads `registration.Commands`, with no engine started. `internal/le/wikicatalog/catalog.go` is the second reader of the same field
  -> Constraint: in-tree commands and pipes are already answerable with no process, so query mode adds nothing for that population and must not grow a second path for it
- [ ] `internal/component/plugin/server/startup.go`, `startup_driver.go` - a plugin that sends Stage 1 and then closes is a FAILURE to the engine: `deliverConfig` errors, `PluginFailed` fires, `rollbackStartupProcess` runs, and a `FailureFatal` plugin stops the daemon
  -> Constraint: query mode must NOT be implemented by driving the real handshake and hanging up after Stage 1. The child writes its declaration to stdout and exits 0

**The audit of registered runners, re-measured 2026-09-08.** 92 registered
product runners were enumerated (99 `RunEngine:` sites, minus six test fakes and
one re-dispatch). 19 do work that reaches outside the runner's own locals before
`p.Run` sends Stage 1. Every row of the 2026-09-07 audit was confirmed at the
producer, with three corrections and seven additions:

| Finding | Runners | What runs before Stage 1 |
|---------|---------|--------------------------|
| Mutates the HOST | `flowspec-firewall`, `ike` | nftables syscalls through `ApplyAll` under `LegacySweepPending`; four node-wide XFRM policies through `installIKEBypass`, whose defer beside a live IKE engine removes that daemon's too |
| Mutates the calling PROCESS | `trafficusage` | `rlimit.RemoveMemlock()`, a `setrlimit`, before its own `!p.IsInternal()` gate |
| Opens a kernel handle | `fib/kernel`, `fib/p4`, `sysctl` | `newBackend()`, `netlink.NewHandle` inside it, closed after the abort's `return 1` |
| Allocates the process-wide default Loc-RIB | `connected`, `rib`, `static`, `sysrib` | `locrib.Default()` |
| Starts a goroutine | `iface` (four), `rpki`, `adj_rib_in` | workers started before the declaration; `iface`'s stops are straight-line code after `p.Run`, so its `return 1` skips all six |
| Leaves a process global pointing at a dead plugin | `adj_rib_in`, `rib`, `redistribute_egress`, `sysrib`, `isis`, `ospf`, `iface`, `kernel`, `connected` | exactly ONE is a defer (`rib`'s `activeManager.Store(nil)`, which does not unwind its route-injector registration); `ospf` also registers an opaque type whose second registration returns `ErrOpaqueTypeRegistered` and is only logged |
| Constructs a manager and subscribes | `as112` | `newAS112Producer`, `newServerManager`, `subscribeReplay` |

Corrections to the 2026-09-07 table: the netlink handle is
`internal/plugins/fib/kernel`, not `internal/plugins/kernel`, which is a
different plugin with its own early source registration; `sysrib` also allocates
the default Loc-RIB; and the unwind count was optimistic, one defer rather than
two.

**Seven runners never reach Stage 1 at all.** `capa`, `loop` (the reactor filter
plugin) and `srpolicy` return without calling `p.Run`. `as112`, `flowexport`,
`vrrp` and `trafficusage` return 1 at `if !p.IsInternal()`, which a bare
`net.Pipe` always fails because `NewWithConn` sets the bridge by type-asserting
`rpc.Bridger`. Since `Run` sends Stage 1 unconditionally even for an empty
`Registration`, "declared nothing" and "sent nothing" are different states.

**Nothing built early is passed into the declaration.** Across the 122 in-tree
`p.Run(ctx, sdk.Registration{...})` call sites, every field value is a package
constant or a pure declaration function; the one exception is `runSDKMode`
(`internal/plugins/exabgp/main_sdk.go`), which builds `Families` from a CLI
argument. `runRIBPlugin` looks like a counter-example and is not: its
`commandDecls()` calls `registerBuiltinCommands()` itself under a `sync.Once`.

**Behavior to preserve:**
- The five startup stages and their order. Query mode reads no new declaration and adds no stage.
- A live start is unchanged in every particular: same env block, same stages, same side effects. `startExternal` is not edited.
- `show plugin list` keeps answering from the compiled-in registry with no daemon.
- `--features` and `--yang` keep answering before any connection.

**Behavior to change:**
- A plugin process started in query mode knows it is being interrogated, writes its Stage 1 declaration to stdout, exits 0, and performs none of a live start's work.
- `show plugin declarations` answers, for every plugin the binary carries and every external plugin the config names, either the declaration or the state that says why there is none.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `show plugin declarations`, and `show plugin declarations config <path>` when the operator wants the external plugins a config file names. Offline mode: no daemon, no hub, no TLS listener.
- Format at entry: command tokens from the CLI or from the SSH command surface, answered as structured rows.

### Transformation Path
1. The reader builds the row set: one row per plugin in the compiled-in registry, plus one row per external plugin the named config file declares.
2. An IN-TREE row is answered from `registry.Registration` (`Commands`, `Pipes`). No process is started, because the answer is already linked into this binary.
3. An EXTERNAL row is answered by forking the plugin's own `run` string with the mode variable in its environment and no hub variables, then reading the child's stdout until it exits or the budget expires.
4. The child recognises the mode before it does anything else, writes one newline-framed `#1 ze-plugin-engine:declare-registration <json>` line to stdout, and exits 0.
5. The reader parses the framed line, ignoring any other stdout content, and records the declaration or the state that says why there is none.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reader -> external plugin process | fork of the config's `run` string with one added environment variable; answer read from the child's stdout | Yes: `startExternal` reads env from `os.Environ()` plus an append, and runs the string under a shell, so an environment variable is the only carrier a shell-quoted run string cannot swallow |
| Reader -> in-tree plugin | none: the declaration is a field of the compiled-in `registry.Registration` | Yes: `Collect` reads that field today with no process |
| ze binary -> its own plugin code | `cli.Run` answers the mode from the looked-up `Registration` and never calls `CLIHandler` | Yes: `cli.Run` holds the registration before the dispatch |
| Third-party binary -> its own plugin code | the SDK entry point sends the declaration and returns without calling the activation function | Yes for a plugin that adopts the entry point; a plugin that does not adopt it is reported as having sent nothing |

### Integration Points
- `cli.Run`, `internal/component/plugin/cli/main.go` - answers query mode for every `ze plugin <name>` process, in-tree or forked
- `internal/component/plugin/register.go` - the sibling command, the registration call, the never-omit rule and the column declaration
- `pkg/plugin/sdk` - the entry point that owns the order for a third-party plugin
- `internal/core/env` - the two new keys, registered like every other

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The answer is the Stage 1 `DeclareRegistrationInput` value in its own wire framing; no second format, no second declaration |
| No unintended coupling (components stay isolated) | Yes | The reader lives in the plugin component beside `show plugin list`, walks the registry and the config, and spells no plugin name |
| No duplicated functionality (extends existing, does not recreate) | Yes | In-tree rows reuse `registry.Registration.Commands`, the field the closed catalog spec created; query mode is added only for the population that has no such field |
| Zero-copy preserved where applicable (refs, not copies) | N-A | An offline introspection command on a cold path; no wire encoding, no pool buffer |
| Registration over hardcoding | Yes | The command registers through `cmdregistry.MustRegisterLocalData`; the rows come from `registry.All()` and the config tree; the two env keys register through `env.MustRegister` |

## Design

**D-1. The reader is `show plugin declarations`.** It is registered by the
plugin component beside `show plugin list`, with `MustRegisterLocalData` and
`Mode: offline`, so every pipe operator renders one payload. One row per plugin,
never an omission. The optional `config <path>` keyword adds the external
plugins a config file names; without it the answer covers the plugins this
binary carries.

**D-2. The carrier is an environment variable, `ze.plugin.mode`**, registered
through `env.MustRegister` and read as `ZE_PLUGIN_MODE`. An external plugin is
started as a shell-quoted run string, so a forker cannot append a flag to it,
and the environment block is already built by appending. The daemon's own start
path never sets the variable, so a live start is untouched.

**D-3. The answer is the Stage 1 message, written to stdout.** The child writes
one newline-framed `#1 ze-plugin-engine:declare-registration <json>` line and
exits 0. The framing is the protocol's own, so a line that is not the framed
message (a log line, a banner) is ignored rather than corrupting the answer, and
the JSON is the `DeclareRegistrationInput` value with the tags it already
carries. The child opens no connection, so it needs neither the hub token nor
the CA.

**D-4. Enforcement is by unreachability where Ze owns the binary, and by an SDK
entry point where it does not.** For every `ze plugin <name>` process, `cli.Run`
answers from the looked-up `Registration` and never calls `CLIHandler`, so no
plugin code runs at all. For a third-party binary, the SDK gains an entry point
that takes the declaration and the activation function as separate arguments and
calls activation only when the mode is absent; a plugin that adopts it cannot
side-effect in query mode. A plugin that does NOT adopt it is not silently
trusted: it is reported as having sent nothing. For third-party code this is
enforcement for adopters and CONVENTION for everyone else, and this spec says so
in those words.

**D-5. The five states are fields of the row.** The budget is 5 seconds, the
same number as `defaultStageTimeout`, overridable with `ze.plugin.query.timeout`.

| State | What it means |
|-------|---------------|
| `declared` | an answer arrived carrying declarations |
| `declared-none` | an answer arrived carrying an empty declaration |
| `no-answer` | the process ran and exited without writing a framed declaration. A plugin that does not implement query mode reads as this, never as `declared-none` |
| `unstartable` | the process could not be started |
| `timeout` | the process started and no answer arrived inside the budget |

**Alternatives considered.**

| Approach | Why it was not chosen |
|----------|----------------------|
| Drive the real 5-stage handshake against a query sink and hang up after Stage 1 | The engine treats a plugin that closes after Stage 1 as a startup failure (`deliverConfig` to `PluginFailed` to rollback, and a daemon stop for a `FailureFatal` plugin). It also needs a hub listener, a token and a CA for a read that needs none of them |
| A mode the runner reads inside its own body | Convention with no enforcement: the audit's 19 runners do their damage before the body reaches any check. This is the shape the closed catalog spec already rejected on that evidence |
| Put every Stage 1 field on `registry.Registration` and start nothing | Works only for in-tree plugins. A third-party binary is not linked into ze, so nothing compiled-in can carry its declaration, and the owner asked for the started-and-queried shape |
| A supervisor that denies the query process a socket | Stops a bind and does not stop `installIKEBypass`, which needs no socket |

**Answers to the skeleton's open questions.**

| Q | Answer |
|---|--------|
| Q-1 enforcement shape | D-4: unreachable for a ze-owned binary, an order-owning SDK entry point for a third-party one, labelled convention for a third-party plugin that adopts neither |
| Q-2 the five-state answer | D-5, with a stated 5-second budget and one row per plugin |
| Q-3 pre-`p.Run` split | Out of scope, because query mode never enters a runner body. The 19 runners are recorded in Current Behavior above and named as `spec-plugin-runner-inertness` |
| Q-4 in-tree plugins | In scope for the ANSWER and out of scope for the PROCESS: an in-tree row is answered from the compiled-in registration, which is strictly better than starting anything |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every field of a command declaration is static, so a query-mode answer equals a live daemon's answer | `CommandDecl` and `PipeDecl`, `pkg/plugin/rpc/types.go`; 122 `p.Run` call sites read 2026-09-08, all constant or pure except `runSDKMode` | A plugin whose declaration varies with configuration answers differently in the two modes | AC-7: the same plugin's query answer and its live Stage 1 declaration compared field by field | confirmed 2026-09-08, with one field named. `TestDeclarationAnswerMatchesStageOne` (`internal/component/plugin/declarations_test.go`) runs one declaration through the real SDK startup over a `net.Pipe` and through `sdk.RunOrDeclare`, and the two JSON values are equal. `Run` derives ONE field the declaration does not state, `WantsValidateOpen`, and the test's second case pins it: every other field stays equal, so a second derived field goes red. That derivation was broken until this spec fixed it (`plan/journal/field-carries-two-meanings.md`, 2026-09-08): a default handler in `initCallbackDefaults` made `Run` declare the field for EVERY plugin, so no author could make the two agree. In-tree plugins are covered separately by `./le plugin declarations check`, green on 2026-09-08 |
| A-2 | `cli.Run` holds the registration before any plugin code runs | `cli.Run`, `internal/component/plugin/cli/main.go`: `registry.Lookup` precedes `reg.CLIHandler` | Query mode for the ze binary would need the plugin's own cooperation, and inertness would drop to convention | AC-3: the fake plugin's `CLIHandler` is never called in query mode | confirmed at the producer 2026-09-08 |
| A-3 | The audit's runner findings still hold | re-measured at every producer 2026-09-08, three corrections recorded above | The follow-up spec is the wrong size | already re-measured | confirmed |
| A-4 | An external plugin's `run` string can be forked by the reader with the same semantics the daemon uses | `(*Process).startExternal`: the run string under a shell, env from `os.Environ()` plus an append | A plugin startable by the daemon is unstartable by the reader, so a row reads `unstartable` when the plugin is fine | AC-5: the functional test starts the same run string both ways | unvalidated |
| A-5 | A third-party plugin that ignores query mode cannot damage the host through the reader | the reader passes no hub variables, so an unaware plugin fails to connect and exits | An unaware plugin runs its live start under a query, which is the one thing this spec exists to prevent | AC-6: an unaware plugin is reported `no-answer`, and R-1 states the residual | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A third-party plugin that adopts neither the ze binary nor the SDK entry point still runs its live start under a query, because nothing but its own code can stop it | a query run changes host state for a plugin Ze does not compile | D-4 makes the SDK entry point the documented route and the reader passes no hub credentials; the residual is stated in Known Limitations rather than hidden |
| R-2 | The answer silently drops a plugin it could not read | a configured plugin absent from the rows | D-5: five states, one row per plugin, never an omission. AC-4 asserts a row for every state |
| R-3 | The child writes something else to stdout and the answer is misparsed | a plugin whose banner or log line lands on stdout | D-3: the answer is newline-framed with the protocol's own prefix, and every other line is ignored |
| R-4 | The budget cannot tell "did not answer" from "still starting" | a slow plugin reported `timeout` on a loaded machine | D-5 states the budget, ties it to `defaultStageTimeout`, and makes it configurable with `ze.plugin.query.timeout` |
| R-5 | A query run beside a LIVE daemon disturbs it | the live daemon loses state after a query | The reader starts no in-tree runner and passes no hub variables, so a query process joins no hub and claims no plugin name. The in-tree runners that would disturb a daemon are never entered |
| R-6 | The new command's name collides conceptually with `show plugin list` | an operator cannot guess which command answers what | One noun, two answers: `show plugin list` is what the binary carries, `show plugin declarations` is what each plugin declares. The owner chose both spellings on 2026-09-08 |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An operator-facing read returns a wrong or missing row. The forked child is the one thing that can reach the host, and only for a third-party plugin that implements neither route (R-1) |
| How is it reverted? | Single commit revert: the command, the SDK entry point and two env keys, with no caller inside the daemon |
| Who else touches this path? | `spec-daemon-backed-command-catalog` (closed 2026-09-08, same declarations read from `registry.Registration`), and the follow-up spec for the 19 runners |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `show plugin declarations` typed at the CLI | -> | the row builder in `internal/component/plugin/declarations.go` | `test/plugin/plugin-declarations-external.ci` |
| `ze plugin <name>` started with the mode variable | -> | the query answer in `cli.Run` | `TestQueryModeAnswersBeforeTheHandler`, `internal/component/plugin/cli/main_test.go` |
| a third-party plugin's `main` | -> | the SDK query entry point | `TestQueryModeSkipsActivation`, `pkg/plugin/sdk/sdk_query_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze plugin <name>` run with `ZE_PLUGIN_MODE=declare` in its environment | Writes one newline-framed `#1 ze-plugin-engine:declare-registration <json>` line to stdout carrying the plugin's `Commands` and `Pipes`, exits 0, opens no connection and reads no hub variable |
| AC-2 | The same invocation for a plugin whose registration declares no command and no pipe | Writes the framed line with an empty declaration and exits 0. The reader records `declared-none`, never `no-answer` |
| AC-3 | A plugin whose `CLIHandler` records that it ran, invoked in query mode | The handler is never called: the query is answered from the looked-up `Registration` in `cli.Run` |
| AC-4 | `show plugin declarations config <path>` over a config naming one plugin per state: a good external plugin, a plugin whose `run` string does not exist, a plugin that exits without answering, a plugin that sleeps past the budget | The five row states are distinguishable and every configured plugin has a row: `declared`, `declared-none`, `no-answer`, `unstartable`, `timeout`. No plugin is omitted |
| AC-5 | The same external `run` string started by the daemon and by the reader | Both start; the daemon's live start reaches Stage 5, the reader's query start exits 0 after one line and never appears in the hub |
| AC-6 | An external plugin binary that implements neither route, queried | The row reads `no-answer`, and the reader names it as a plugin that does not support query mode |
| AC-7 | The declaration a plugin sends at Stage 1 to a live daemon, and the declaration the same plugin writes in query mode through `sdk.RunOrDeclare` | The two JSON values are equal. The claim is about THAT route: the ze binary answers for a plugin it carries from `registry.Registration`, which holds commands and pipes and not the rest of the Stage 1 value, so its answer is the declared subset by the field-scope decision above and not a second declaration of the same fields |
| AC-8 | `show plugin declarations` rendered through `json`, `yaml` and `table`, and through a row operator such as `match` | Each is a rendering of the same payload, with the state carried as a field rather than glued to the name |
| AC-9 | `show plugin declarations` with no `config` keyword and no daemon running | Answers a row for every plugin the binary carries, from the compiled-in registration, starting no process |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Asks what commands an external plugin adds, before running it in a router | `show plugin declarations config <path>` -> fork with the mode variable -> framed Stage 1 line -> rows | `test/plugin/plugin-declarations-external.ci` |
| 2 | Asks the same of a plugin that is broken or does not support the mode | same path, the child fails or answers nothing -> the row carries `unstartable` or `no-answer` | `test/plugin/plugin-declarations-states.ci` |
| 3 | Writes a third-party plugin and makes it answer a query without activating | the plugin's `main` calls the SDK query entry point with its declaration and its activation function | `TestQueryModeSkipsActivation` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestQueryModeAnswersBeforeTheHandler` | `internal/component/plugin/cli/main_test.go` | AC-1, AC-3: the framed line is written and `CLIHandler` never runs | |
| `TestQueryModeEmptyDeclarationIsNotSilence` | `internal/component/plugin/cli/main_test.go` | AC-2: an empty declaration still writes the framed line | |
| `TestQueryModeSkipsActivation` | `pkg/plugin/sdk/sdk_query_test.go` | AC-1, AC-6: the SDK entry point writes the declaration and does not call the activation function | |
| `TestQueryModeActivatesWithoutTheMode` | `pkg/plugin/sdk/sdk_query_test.go` | the same entry point runs activation normally when the variable is absent, so a live start is unchanged | |
| `TestDeclarationRowsCarryEveryState` | `internal/component/plugin/declarations_test.go` | AC-4: the five states, one row each, none omitted | |
| `TestDeclarationRowsAnswerInTreeFromTheRegistration` | `internal/component/plugin/declarations_test.go` | an in-tree row starts no process: the config names the plugin INTERNAL and gives it a `run` string that cannot start, so a reader that started anything answers `unstartable` | |
| `TestDeclarationRowsCoverEveryRegisteredPlugin` | `internal/component/plugin/declarations_test.go` | AC-9: the form with no `config` keyword answers one row per plugin the binary carries, over the `registry.SetupResults` population rather than the narrower registered set | |
| `TestDeclarationAnswerMatchesStageOne` | `internal/component/plugin/declarations_test.go` | AC-7: the query answer equals the Stage 1 declaration for the same plugin | |
| `TestDeclarationIgnoresUnframedStdout` | `internal/component/plugin/declarations_test.go` | R-3: a banner line before the framed answer does not corrupt the row | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| query answer budget (`ze.plugin.query.timeout`) | positive duration, default 5s | an answer written just inside the budget is `declared` | zero or negative is refused, never treated as no wait | an answer written after the budget is `timeout`, and the child is stopped |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `plugin-declarations-external` | `test/plugin/plugin-declarations-external.ci` | An operator reads an external plugin's commands with no daemon running | |
| `plugin-declarations-states` | `test/plugin/plugin-declarations-states.ci` | The same read over a config whose plugins are broken, silent or slow: every one has a row | |
| `plugin-declarations-pipes` | `test/plugin/plugin-declarations-pipes.ci` | AC-8: the same answer rendered through each operator | |

### Interop Tests (Scope: protocol)
N-A. No wire-visible protocol change and no peer daemon: the plugin RPC is Ze's
own, and query mode emits an existing message on an existing framing.

## Files to Modify
- `internal/component/plugin/cli/main.go` - answer query mode in `cli.Run` from the looked-up registration, before `CLIHandler`
- `pkg/plugin/sdk/sdk.go` - the package the new query entry point joins, and the doc comment that points a plugin author at it
- `docs/architecture/cli/plugin-modes.md` - the fourth mode: its carrier, its answer and its inertness guarantee
- `docs/architecture/api/process-protocol.md` - query mode beside the five stages: the same Stage 1 message, on stdout, with no Stage 2
- `docs/plugin-development/protocol.md` - what a third-party plugin author calls to support the mode
- `docs/architecture/plugin/plugin-system.md` - the reader command in the introspection surface
- `ai/rules/plugins.md` - one directive: a plugin that declares must be answerable without activating

## Files to Create
- `internal/component/plugin/declarations.go` - `show plugin declarations`: the row builder, the five states, the fork and the framed-line parse
- `internal/component/plugin/declarations_test.go` - the unit tests above
- `pkg/plugin/sdk/sdk_query.go` - the entry point that owns the order for a third-party plugin
- `pkg/plugin/sdk/sdk_query_test.go` - the two SDK tests above
- `test/plugin/plugin-declarations-external.ci`, `test/plugin/plugin-declarations-states.ci`, `test/plugin/plugin-declarations-pipes.ci`

### Discovery (`ai/rules/repo-maintenance.md`)
| Question | Answer |
|----------|--------|
| Where does an agent look first? | `ai/INDEX.md`, keywords `plugin declarations` and `query mode`, pointing at `docs/architecture/cli/plugin-modes.md` |
| What rule prevents regression? | `ai/rules/plugins.md` gains the directive that a declaration must be answerable without activating |
| What registry prevents drift? | `registry.Registration` stays the single in-tree declaration, and `./le plugin declarations check` already holds it against the runner's literal |
| What verification proves it? | The three `.ci` tests, the unit tests above, and `./le verify worktree` |

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | An offline local-data command, registered exactly as `show plugin list` is; `./le docvalid command-contract` governs the pairing |
| YANG validation constraints | N-A | no YANG node added |
| YANG custom validators | N-A | no YANG node added |
| CLI commands/flags | Yes | `show plugin declarations`, `internal/component/plugin/declarations.go` |
| CLI grammar (keyword before value) | Yes | the only value is `config <path>`, typed by its keyword; `./le cli-grammar` is the check |
| Editor autocomplete | Yes | derived from the command declaration, as `show plugin list` is |
| Functional test for new RPC/API | Yes | the three `.ci` tests above |
| Pipe completeness | Yes | `MustRegisterLocalData` plus `RegisterShape` and `RegisterColumns`, as `show plugin list` does; AC-8 |
| Env var registration | Yes | `ze.plugin.mode` and `ze.plugin.query.timeout`, through `env.MustRegister` |
| Doctor check for runtime dependencies | Yes | the reader forks a command the config already names, and it forks it through `/bin/sh`, which an appliance image does not carry. That shell is a runtime dependency of every external plugin, live start and query alike, so it owes a check: `plugin-shell` (`internal/component/plugin/doctor/register.go`) raising `doctor-plugin-shell-missing` (owner decision, 2026-09-08) |
| Prometheus counters/metrics | N-A | an operator-invoked offline read |
| BGP family surface (new SAFI / capability / attribute) | N-A | no family, capability or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/architecture/cli/plugin-modes.md`, and the command reaches the published catalog through its registration |
| 2 | Config syntax changed? | N-A | no config node added |
| 3 | CLI command added/changed? | Yes | `docs/architecture/cli/plugin-modes.md`; the catalog page is generated from the registration |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/process-protocol.md`: the same Stage 1 message on a second carrier |
| 5 | Plugin added/changed? | N-A | no plugin added |
| 6 | Has a user guide page? | Yes | `docs/architecture/cli/plugin-modes.md` is the page an operator reads for plugin invocation |
| 7 | Wire format changed? | N-A | no wire format |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/plugin-development/protocol.md`, `docs/architecture/api/process-protocol.md`, `ai/rules/plugins.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | no RFC |
| 10 | Test infrastructure changed? | N-A | the `.ci` tests use the existing runner |
| 11 | Affects daemon comparison? | N-A | no daemon behavior changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/plugin/plugin-system.md` |
| 13 | Route metadata keys added/changed? | N-A | no route metadata |
| 14 | Prometheus counters added/changed? | N-A | none added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | one command registered; the generated catalog and completion follow from the registration |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/spec-plugin-query-mode.md` before the first code edit; `internal/component/plugin/cli/main.go` already carries a `Design:` anchor to `docs/architecture/api/process-protocol.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/cli/plugin-modes.md` carries the invocation examples the new mode joins |

## Implementation Steps

0. **Precondition, landed as its own commit before this spec starts** -- `show plugins` is renamed to `show plugin list` (owner directive, 2026-09-08), so the plugin namespace carries one noun before a second command joins it. Ze is unreleased, so the old spelling is replaced outright rather than aliased. This spec does not carry that rename.
1. **Phase: Wiring (MANDATORY FIRST)** -- register `show plugin declarations` as a local-data command answering a stub row set
   - Tests: `test/plugin/plugin-declarations-external.ci`, red because the row carries no declaration
   - Files: `internal/component/plugin/declarations.go`
   - Verify: the command is reachable from the CLI, and the test fails because the feature is a stub
2. **Phase: the query answer in the ze binary** -- `cli.Run` reads the mode, writes the framed Stage 1 line from the looked-up registration, and returns before `CLIHandler`
   - Tests: `TestQueryModeAnswersBeforeTheHandler`, `TestQueryModeEmptyDeclarationIsNotSilence`
   - Files: `internal/component/plugin/cli/main.go`, the `env` registration
   - Docs: `docs/architecture/cli/plugin-modes.md` in this phase, not later
3. **Phase: the reader** -- fork the config's `run` string with the mode, read stdout under the budget, parse the framed line, build the five states, and answer in-tree rows from the registration
   - Tests: `TestDeclarationRowsCarryEveryState`, `TestDeclarationRowsAnswerInTreeFromTheRegistration`, `TestDeclarationIgnoresUnframedStdout`, `test/plugin/plugin-declarations-states.ci`
   - Files: `internal/component/plugin/declarations.go`
4. **Phase: the SDK entry point** -- declaration and activation as separate arguments, activation unreachable under the mode
   - Tests: `TestQueryModeSkipsActivation`, `TestQueryModeActivatesWithoutTheMode`
   - Files: `pkg/plugin/sdk/sdk_query.go`, `pkg/plugin/sdk/sdk.go`
   - Docs: `docs/plugin-development/protocol.md`, `docs/architecture/api/process-protocol.md`, `ai/rules/plugins.md` in this phase
5. **Phase: equality and rendering** -- prove the query answer equals the live Stage 1 declaration, and that the payload renders through every operator
   - Tests: `TestDeclarationAnswerMatchesStageOne`, `test/plugin/plugin-declarations-pipes.ci`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Inertness | A query-mode start touches no nftables table, no XFRM policy, no rlimit, no socket, no timer, no process global. For a ze-owned binary this is unreachability, not a promise |
| No silent omission | Every plugin in the registry and every plugin the config names has a row, carrying one of the five states |
| State distinctness | `declared-none` and `no-answer` are separate answers, and a plugin that does not implement the mode reads as the second |
| One fact | The answer is the plugin's own declaration through the Stage 1 message; no second declaration, no second format |
| Enforcement, not convention | The spec states plainly where the guarantee is unreachability and where it is convention (D-4, R-1, Known Limitations) |
| No layering | In-tree rows and external rows are one mechanism stated once: the process that owns the plugin's code answers. No fallback chain, no hybrid |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `show plugin declarations` answers with no daemon | `ze show plugin declarations` with nothing running |
| An external plugin answers without activating | `test/plugin/plugin-declarations-external.ci` |
| Every state is reachable and none is omitted | `test/plugin/plugin-declarations-states.ci` |
| A third-party plugin can support the mode | `TestQueryModeSkipsActivation` |
| The answer equals the live declaration | `TestDeclarationAnswerMatchesStageOne` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Privilege at start | A query-mode start must not need, take, or keep the privilege a live start needs. The ze-owned route runs no plugin code at all |
| Credential exposure | The reader passes no hub token and no CA: a query process joins no hub and authenticates to nothing |
| Command execution | The reader forks a string the config already tells the daemon to run. It adds no new source of executable text, and it takes a run string from nowhere but the config file the operator named |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood -> RESEARCH |
| Lint failure | Fix inline. If architectural -> DESIGN |
| Functional test fails | Check the AC: wrong AC -> DESIGN, correct AC -> IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The declarations of an in-tree plugin are no longer trapped: the closed catalog spec freed them. What is still trapped is the external plugin's, and only because its code is in another binary.
- The protocol already splits declaring from activating for the common case. What is unguarded is the runner body before `p.Run`, and the way past it is not to enter the runner at all.
- `cli.Run` holding the registration before the dispatch is what turns inertness from a promise into a property.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `show plugin declarations`, beside the renamed `show plugin list` | `show plugin declaration list`; `show plugin declarations list` | Owner decision, 2026-09-08: the number follows what the token names, so the noun is singular and the plural names the many declarations the answer carries |
| Answer in `cli.Run`, before the plugin's own handler | a mode read inside the runner; a mode read inside `p.Run` | Both let the runner body run first, which is where the 19 measured side effects are |
| An environment variable as the carrier | a flag on `ze plugin <name>`; both | An external plugin runs under a shell as a config-supplied string, so a forker cannot append a flag. Two carriers for one mode can disagree |
| The Stage 1 message on stdout, in its own framing | a bespoke JSON document; a connect-back to a query listener | One declaration in one format, and a framed line survives a plugin that also writes to stdout. A connect-back needs a listener, a token and a CA for a read that needs none |
| Commands and pipes only | every Stage 1 field | The untwinned fields are a separate change with a separate blast radius, named as a follow-up spec |
| The 19 early-side-effecting runners stay as they are | fix them here | Query mode never enters a runner body, so it does not depend on them. Folding a 19-runner refactor into this commit costs it its single focus |

## Known Limitations
- A third-party plugin that implements neither route cannot be queried. It is reported `no-answer`, never guessed at. For such a plugin, query mode is convention, and this spec does not pretend otherwise (R-1).
- Query mode answers commands and pipes. The Stage 1 fields with no compiled-in twin (config operations, filters, doctor checks, enrichers, schema, budgets, failure policy) are not answered: `spec-plugin-declaration-fields-on-registration`.
- The 19 runners that mutate the host, the process or a global before they declare are unchanged: `spec-plugin-runner-inertness`.
- The two routes answer with different field sets, and no single test spans them. `sdk.RunOrDeclare` writes the whole declaration its caller passes, while the ze binary writes the commands and pipes `registry.Registration` carries. `rpc.WriteDeclaration` pins the line's id, method, framing and encoding for both, and `./le plugin declarations check` pins the compiled-in declaration against the runner's own literal, so what is unpinned is the field SET each route chooses, which the row above names as the follow-up spec.
- A plugin's Go package `init()` runs before either route answers, because the answer is written by a process that has already started. One of them mutates this process: `memlock` calls `lockexe.OnFault()` (`internal/plugins/memlock/memlock_linux.go`), an `mlock2` against `RLIMIT_MEMLOCK`. That is the state of every `ze plugin <name>` invocation, `--features` and `--yang` included, rather than something query mode introduced, and it is outside what an entry point can hold inert.

## RFC Documentation (Scope: protocol)

N-A. No RFC governs the plugin RPC.

## Review Gate

<!-- Filled by /ze-review at implementation time. Never delete this section. -->

| Run | Blockers | Issues | Notes |
|-----|----------|--------|-------|
| 1 (implementation, independent diff pass) | 2 | 4 | The merge answered Ze's declaration for an external block; the queried child outlived its budget. Both fixed at the producer with the red observed first |
| 2 (implementation, independent diff pass) | 0 | 1 | The live start carried the process-group attributes and not the group stop. Fixed with `KillGroupOnCancel`, plus four nits |
| 3 (closure, this context, every lens) | 0 | 2 | Discovery: `ai/INDEX.md` carried no keyword row for the command its own Discovery table names. Documentation: `docs/guide/command-reference.md` documents `show plugin list` by hand and carried no sibling section |
| 4 (closure, re-read after the round 3 edits) | 0 | 0 | Clean. The round 3 edits are prose and a YANG revision date; `./le docvalid command-contract` and `./le cli-grammar` re-run green after them |

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/plugin-query-mode-8e533ac8-8f4a-4ba7-ac45-ed497b31f2a8.md` (78 files) |
| `./le spec session review check` | `review_gate: OK (56 code files, clean, hashes match ...)` |
| Rounds | 4 |
| Reviewer lenses used | wiring, functional-test coverage, documentation drift, removed-behavior audit, data flow, edge cases, security, allocation bounds, logic correctness, altitude and simplicity, project rules, the six-question Go style pass |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The merge answered Ze's own declaration for a config block that names ANOTHER binary under a name Ze also uses | `declarationRows`, `internal/component/plugin/declarations.go` | The config block's kind decides: an `external` block is always queried. `TestDeclarationRowsQueryTheBinaryAnExternalBlockNames` |
| 2 | BLOCKER | The queried child outlived its budget: the stop reached the shell and left the plugin running | `runQuery`, `declarations.go` | `KillGroupOnCancel` sets `Setpgid` and a group-aimed `Cancel`. `TestDeclarationQueryStopsThePluginTheShellStarted` |
| 3 | ISSUE | The environment strip missed the dotted spelling of a plugin variable, so the hub token could reach the child | `queryChildEnv`, `declarations.go` | `env.InNamespace` (`internal/core/env/env.go`) drops the whole `ze.plugin.` namespace under either spelling. `TestQueryChildEnvDropsEveryPluginVariableSpelling` |
| 4 | ISSUE | The in-tree population was narrower than `show plugin list`, so a plugin that recorded a setup outcome and never registered had no row | `inTreeDeclarationRows`, `declarations.go` | The walk is `registry.SetupResults`. `TestDeclarationRowsCoverAPluginThatNeverRegistered` |
| 5 | ISSUE | Two of the three exits in the external `.ci` asserted nothing | `test/plugin/plugin-declarations-external.ci` | Per-command `cmd=...:exit=N`, and the external row's declaration proved through a `match`-scoped JSON rendering |
| 6 | ISSUE | `shellAvailable` accepted a directory and read the mode bits rather than asking the kernel | `shellAvailable`, `internal/component/plugin/shell.go` | `unix.Faccessat(AT_FDCWD, shell, X_OK, AT_EACCESS)`, the question `execve` asks. `TestShellAvailableAnswersForTheUserZeRunsAs` |
| 7 | ISSUE | The live start had the process-group attributes and not the group stop, so a daemon stop left the plugin running behind the shell | `(*Process).startExternal`, `internal/component/plugin/process/process.go` | `KillGroupOnCancel` (`sysproc.go`) is the one function that sets both; the connect-back failure path calls `KillProcessGroup`. `TestStopReachesThePluginTheShellStarted` |
| 8 | ISSUE | The internal-block-not-carried branch had no test | `queriedDeclarationRow`, `declarations.go` | `TestDeclarationRowsRefuseAnInternalBlockWithNoCodeHere`, which also proves no process started |
| 9 | ISSUE | `ai/INDEX.md` carried no keyword row, so an agent could not reach the feature from the discovery surface the spec's own Discovery table names | `ai/INDEX.md`, Keyword to Architecture Doc | One row: `plugin declarations, query mode, declare mode, ZE_PLUGIN_MODE, ...` pointing at the four pages |
| 10 | ISSUE | `docs/guide/command-reference.md` documents `show plugin list` in a hand-written section and carried no sibling section for the command this spec adds | `docs/guide/command-reference.md`, after `### show plugin list` | The `### show plugin declarations` section is WRITTEN and correct. It is NOT in the commit: the page carries another session's `show bgp update-delay` row whose source anchor names an untracked file, so carrying the page publishes a claim the committed tree cannot support (`ai/rules/git-safety.md`). Recorded in `plan/journal/documentation-stranded-by-a-siblings-hunk.md` |

### Notes (not blocking)
| Note | Disposition |
|------|-------------|
| The new YANG revision repeated the date of the revision below it, so the module had two revisions dated 2026-09-08 | Fixed: the new one reads 2026-09-09. `./le docvalid command-contract` exit 0, both wire methods paired |
| The published website catalog under `../gh-pages/` carries no page for either new command | Pre-existing and tree-wide: the same `./le doc check verify` run reports the identical failure for `request bgp rib mark-stale`, `show bgp rib` filters and others. The catalog is generated into another branch by `./le wiki-catalog update` and no commit on `main` can carry it |
| `ai/allowed-system-commands.md` carries an uncommitted row for the plugin fork, attributed "Thomas, 2026-09-08" | LEFT OUT of the commit. No phase handoff claims that edit, and that file's own rule says a row is added by Thomas and only by Thomas. It is the owner's to confirm |
| `sdk.RunOrDeclare` has no non-test caller in this repository | By construction: its audience is a binary ze does not compile. Stated in D-4 and in `docs/plugin-development/protocol.md` |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `-> Decision:` / `-> Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is
- [ ] Q-1 through Q-4 each answered in the spec, not in conversation

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### Goal Validation
| Goal | Evidence |
|------|----------|
| A plugin can be asked for its details while it is started and aware it is being queried | `test/plugin/plugin-declarations-external.ci`: with no daemon, `show plugin declarations config <path>` forks `ze plugin mrt` and reads `request mrt dump-rib` off its stdout. The answer is the SAME message a live daemon receives, and `TestDeclarationAnswerMatchesStageOne` (`internal/component/plugin/declarations_test.go`) proves it by running one declaration through the real SDK startup and through `sdk.RunOrDeclare` and comparing the two JSON values |
| A query-mode start performs none of a live start's work | The work is UNREACHABLE rather than skipped, and three tests hold the three routes. `TestQueryModeAnswersBeforeTheHandler` (`internal/component/plugin/cli/main_test.go`): the ze binary answers from the looked-up `Registration` and never calls `CLIHandler`. `TestQueryModeSkipsActivation` (`pkg/plugin/sdk/sdk_query_test.go`): `RunOrDeclare` never calls the activation function it was given. `TestDeclarationRowsAnswerInTreeFromTheRegistration` (`internal/component/plugin/declarations_test.go`): an in-tree row is answered in 0.00s from a config whose `run` string cannot start, so no process was started at all |
| No configured plugin is missing from the answer | `TestDeclarationRowsCarryEveryState` and `test/plugin/plugin-declarations-states.ci`: one config names a plugin per state, and each of `declared`, `declared-none`, `no-answer`, `unstartable` and `timeout` has its own row. The state is a FIELD of that row, which `test/plugin/plugin-declarations-pipes.ci` renders through `\| json`, `\| yaml` and `\| table` and selects on with `\| match` |

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
- [ ] **Commit B:** the spec removal only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- `show plugin declarations` and `show plugin declarations config <path>`, registered with `MustRegisterLocalData` beside `show plugin list` (`internal/component/plugin/register.go`), answering rows built in `internal/component/plugin/declarations.go`. One row per plugin, never an omission, with the five states as a FIELD.
- The query answer for the ze binary: `cli.Run` (`internal/component/plugin/cli/main.go`) answers from the looked-up `*registry.Registration` before `reg.CLIHandler`, so no plugin code runs.
- The query answer for a third-party binary: `sdk.RunOrDeclare` (`pkg/plugin/sdk/sdk_query.go`), taking the declaration and the activation function as separate arguments.
- One writer for both: `rpc.WriteDeclaration` (`pkg/plugin/rpc/declaration.go`) holds the id, the method, the framing and the encoding of the Stage 1 line, and `rpc.MethodDeclareRegistration` replaced the string literal at every Go site that spelled it.
- The reader's fork: `runQuery` starts the operator's `run` string under `plugin.Shell` with `KillGroupOnCancel`, `queryChildEnv` (every `ze.plugin.` variable dropped, the mode added), a bounded stdout buffer, and `declarationFromStdout` parsing only the framed line.
- Two env keys, `ze.plugin.mode` and `ze.plugin.query.timeout`, registered through `env.MustRegister`; a budget of zero or less is refused.
- The shell as a named runtime dependency: `plugin.Shell` and `ShellAvailable` (`shell.go`), read by both fork sites and by the new `plugin-shell` doctor check raising `doctor-plugin-shell-missing`.
- The local-data walk covers both new commands (`internal/test/localdatacoverage`), moving the completion marker to `20/20`.

### Bugs Found/Fixed
- The SDK declared `WantsValidateOpen` for EVERY plugin ever built: `Run` read `p.callbacks[callbackValidateOpen] != nil` and `initCallbackDefaults` fills that entry for every plugin, so the engine asked every plugin to validate every OPEN. Now `p.wantsValidateOpen`, set only by `OnValidateOpen`. Covered by `TestDeclarationAnswerMatchesStageOne` (`internal/component/plugin/declarations_test.go`), whose "nothing is derived from the plugin" case was RED on the unfixed tree.
- A row whose alphabetically first key held a list rendered YAML no parser accepts: `writeKeyValue` (`internal/component/command/format.go`) used one string for the key's own line and for every line under it. Covered by `TestRenderYAMLSequenceItemWithANestedListParses`, which decodes with `gopkg.in/yaml.v3`.
- The daemon's live start stopped the shell rather than the plugin behind it. Covered by `TestStopReachesThePluginTheShellStarted` (`internal/component/plugin/process/process_stop_test.go`).
- Both product defects were older than this spec, and both are daemon-visible.

### Documentation Updates
- `docs/architecture/cli/plugin-modes.md`: query mode as the fourth mode, its carrier, its answer, its inertness.
- `docs/architecture/api/process-protocol.md`: Stage 1 on a second carrier with no Stage 2; the group stop and Pdeathsig with its limit.
- `docs/plugin-development/protocol.md`: the `main` shape a third-party author writes, and what the SDK does and does not guarantee.
- `docs/architecture/plugin/plugin-system.md`: the two answer routes, the population, the shared stop.
- `docs/features/introspection.md`: the two commands in the table, the five states, the budget, the config-block rule.
- `docs/architecture/doctor-and-health-checks.md`, `docs/guide/health-checks.md`, `docs/guide/plugins.md`, `docs/guide/appliance.md`: the shell dependency and `doctor-plugin-shell-missing`.
- `ai/rules/plugins.md` gained one directive (`ai/rules/points/plugins/directives/answer-a-declaration-without-activating.md`); `ai/INDEX.md` gained the keyword row.
- `./le doc check verify`: the source-anchor stage names four stale anchors, all pre-existing and none in a file this spec touched (`docs/architecture/api/commands.md` at `getHelpExtension`, `docs/architecture/exabgp-bridge.md` at `script.line`, and two in `docs/architecture/firewall/firewall-irr.md` at `expandIRRTermV6` and `sdk.Registration.FailurePolicy`). Every anchor this spec wrote resolves.
- NOT updated, and recorded rather than forced: `docs/guide/command-reference.md`. See Review Gate finding 10.

### Deviations from Plan
- The spec named `internal/component/plugin/declarations.go` and `pkg/plugin/sdk/sdk_query.go` as the new files. Five more were created, each because a fact would otherwise have had two declarations: `pkg/plugin/rpc/declaration.go` (one writer for both binaries), `internal/component/plugin/childenv.go` (one PATH for both forks), `internal/component/plugin/sysproc*.go` (one stop for both forks), `internal/component/plugin/shell.go` (one shell for both forks and the doctor check), `internal/component/config/register_plugin_declarations.go` (the config seam the import direction requires).
- The doctor check was `N-A` in the Integration Checklist at design time and is now `Yes`. The reader forks through `/bin/sh`, which an appliance image does not carry, so the shell is a runtime dependency and owes a check (owner decision, 2026-09-08).
- The spec planned no change to `internal/component/command`. The YAML defect was found by AC-8's own rendering test and fixed at its producer.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1 assumed a query answer equals the live Stage 1 declaration for the same plugin | One field was derived rather than declared, and derived wrongly for every plugin: `WantsValidateOpen` | AC-7's test could not be made to pass | Product fixed at `(*Plugin).Run`; A-1 confirmed with the one derived field named |
| approach | Phase 2 and phase 4 each wrote the framed declaration line, each with its own `declarationRequestID = 1` | One line, two definitions, free to drift | Phase 4b read both | Consolidated into `rpc.WriteDeclaration`; `TestQueryAnswerIsTheSameLineFromBothWriters` pins the two answers byte for byte |
| approach | The query fork was given a group-aimed cancel while the live fork kept the attributes, so each fork held one half of one stop | Either half alone stops nothing extra | Review round 2 | `KillGroupOnCancel`, the only caller of `NewSysProcAttr`; journal row in `plan/journal/guard-added-to-one-half-of-a-pair.md` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The plugin process IS started, and is told it is being interrogated | Done | `runQuery` (`declarations.go`) starts the run string with `ZE_PLUGIN_MODE=declare` | The owner's correction of 2026-09-07 |
| It answers with its declarations | Done | `rpc.WriteDeclaration`, read by `declarationFromStdout` | The same Stage 1 message a live daemon receives |
| It performs none of a live start's work | Done | `cli.Run` for a plugin ze carries; `sdk.RunOrDeclare` for one it does not | Unreachability, not a promise. Convention only for a plugin that adopts neither, and D-4/R-1 say so |
| No second declaration of anything | Done | `registry.Registration` for in-tree, the plugin's own `commandDecls()` for external | No manifest, no build-time emission, no generated list |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestQueryModeAnswersBeforeTheHandler`, `TestQueryModeSkipsActivation` | Framed line on stdout, exit 0, no connection |
| AC-2 | Done | `TestQueryModeEmptyDeclarationIsNotSilence` | The empty declaration still writes the line |
| AC-3 | Done | `TestQueryModeAnswersBeforeTheHandler` | The fake plugin's `CLIHandler` records that it ran, and it never runs |
| AC-4 | Done | `TestDeclarationRowsCarryEveryState`, `test/plugin/plugin-declarations-states.ci` | Five states, one row each, none omitted |
| AC-5 | Done | `test/plugin/plugin-declarations-external.ci` against `test/plugin/llgr-egress-state-unloaded.ci` | The same `run "ze plugin <name>"` shape is started live by the daemon there and queried by the reader here. Both forks now call `plugin.Shell` and `plugin.ChildPathEnv` |
| AC-6 | Done | `plugin-declarations-states.ci`, the `plugin-silent` row | A plugin that ran and wrote nothing reads `no-answer`, with the reason naming the missing support |
| AC-7 | Done | `TestDeclarationAnswerMatchesStageOne`, `TestQueryAnswerIsTheSameLineFromBothWriters` | One declaration through the real SDK startup over a `net.Pipe` and through `RunOrDeclare`, compared whole |
| AC-8 | Done | `test/plugin/plugin-declarations-pipes.ci`, `test/ui/pipe-local-command.ci` | `json`, `yaml`, `table` and `match` over one payload; the walk reads the state as a FIELD |
| AC-9 | Done | `TestDeclarationRowsCoverEveryRegisteredPlugin`, `plugin-declarations-external.ci` seq=1 | The bare form answers from the registration with no daemon and no process |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestQueryModeAnswersBeforeTheHandler` | Done | `internal/component/plugin/cli/main_test.go` | |
| `TestQueryModeEmptyDeclarationIsNotSilence` | Done | `internal/component/plugin/cli/main_test.go` | |
| `TestQueryModeSkipsActivation` | Done | `pkg/plugin/sdk/sdk_query_test.go` | |
| `TestQueryModeActivatesWithoutTheMode` | Done | `pkg/plugin/sdk/sdk_query_test.go` | |
| `TestDeclarationRowsCarryEveryState` | Done | `internal/component/plugin/declarations_test.go` | |
| `TestDeclarationRowsAnswerInTreeFromTheRegistration` | Done | `internal/component/plugin/declarations_test.go` | Runs in 0.00s, which is the no-process evidence |
| `TestDeclarationRowsCoverEveryRegisteredPlugin` | Done | `internal/component/plugin/declarations_test.go` | |
| `TestDeclarationAnswerMatchesStageOne` | Done | `internal/component/plugin/declarations_test.go` | |
| `TestDeclarationIgnoresUnframedStdout` | Done | `internal/component/plugin/declarations_test.go` | |
| `plugin-declarations-external` | Done | `test/plugin/plugin-declarations-external.ci` | |
| `plugin-declarations-states` | Done | `test/plugin/plugin-declarations-states.ci` | |
| `plugin-declarations-pipes` | Done | `test/plugin/plugin-declarations-pipes.ci` | |
| Boundary: the budget | Done | `TestDeclarationBudgetRefusesNonPositive`, the `plugin-slow` row of the states `.ci` | Zero or less refused; an answer past the budget is `timeout` and the child is stopped |
| Added beyond the plan | Done | 11 further tests, named in the Review Gate findings and in the Files table | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/plugin/cli/main.go` | Done | `answerQuery` before `CLIHandler` |
| `pkg/plugin/sdk/sdk.go` | Done | `EnvPluginMode`, `ModeDeclare`, `QueryModeRequested`, and the `wantsValidateOpen` fix |
| `docs/architecture/cli/plugin-modes.md` | Done | |
| `docs/architecture/api/process-protocol.md` | Done | |
| `docs/plugin-development/protocol.md` | Done | |
| `docs/architecture/plugin/plugin-system.md` | Done | |
| `ai/rules/plugins.md` | Done | One directive, rendered from its point file |
| `internal/component/plugin/declarations.go` + `_test.go` | Done | |
| `pkg/plugin/sdk/sdk_query.go` + `_test.go` | Done | |
| The three `.ci` files | Done | |
| Seven files beyond the plan | Changed | Recorded in Deviations |

### Audit Summary
- **Total items:** 9 ACs, 13 planned tests, 15 planned files, 4 task requirements
- **Done:** all of them
- **Partial:** none
- **Skipped:** none
- **Changed:** the file list grew by seven, recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A plugin can be asked for its details while it is started and aware it is being queried | functional | `test/plugin/plugin-declarations-external.ci`: with no daemon, `show plugin declarations config <path>` forks `ze plugin mrt` and reads `"name": "request mrt dump-rib"` off its stdout through a `match`-scoped JSON rendering. The answer is the SAME message a live daemon receives, and `TestDeclarationAnswerMatchesStageOne` proves it by running one declaration through the real SDK startup and through `sdk.RunOrDeclare` and comparing the two JSON values whole |
| A query-mode start performs none of a live start's work | unit, three routes | `TestQueryModeAnswersBeforeTheHandler` (the ze binary never calls `CLIHandler`), `TestQueryModeSkipsActivation` (`RunOrDeclare` never calls the activation function), `TestDeclarationRowsAnswerInTreeFromTheRegistration` (an in-tree row answered in 0.00s from a config whose `run` string cannot start, so no process was started). The residual for a plugin that adopts neither route is R-1, stated in Known Limitations |
| No configured plugin is missing from the answer | functional + unit | `test/plugin/plugin-declarations-states.ci`: one config names a plugin per state and each of `declared`, `unstartable`, `no-answer` and `timeout` has its own row; `TestDeclarationRowsCarryEveryState` covers all five including `declared-none`. `TestDeclarationRowsCoverAPluginThatNeverRegistered` covers the plugin the registry knows and the registration walk would drop |
| The answer renders through every operator | functional | `test/plugin/plugin-declarations-pipes.ci` and `test/ui/pipe-local-command.ci` (test 214): `| json`, `| yaml`, `| table` and `| match` over one payload, with the walk reading `name`, `kind`, `state`, `commands`, `pipes` and `reason` as FIELDS. The `| yaml` rendering is the one that found the `writeKeyValue` defect |
| Interop | N-A | No wire-visible protocol change and no peer daemon: the plugin RPC is Ze's own, and query mode emits an existing message on an existing framing |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| The Stage 1 fields with no compiled-in twin (config operations, filters, doctor checks, enrichers, schema, budgets, failure policy) are not answered for a plugin ze carries | The field-scope decision in Key Design Decisions: the untwinned fields are a separate change with a separate blast radius | `plan/spec-plugin-declaration-fields-on-registration.md` |
| The 19 runners that mutate the host, the process or a process global before they declare are unchanged | Query mode never enters a runner body, so it does not depend on them | `plan/spec-plugin-runner-inertness.md` |
| The `### show plugin declarations` section of `docs/guide/command-reference.md` is written and not committed | The page carries another session's hunk whose source anchor names an untracked file, so committing the page publishes a claim the tree cannot support | No spec: one row in `plan/journal/documentation-stranded-by-a-siblings-hunk.md`, and the owner's call |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/plugin/declarations.go` | Yes | `ls -l` 27K |
| `internal/component/plugin/declarations_test.go` | Yes | `ls -l` 36K |
| `pkg/plugin/sdk/sdk_query.go` | Yes | `ls -l` 2.5K |
| `pkg/plugin/sdk/sdk_query_test.go` | Yes | `ls -l` 3.9K |
| `pkg/plugin/rpc/declaration.go` | Yes | `ls -l` 1.7K |
| `internal/component/plugin/shell.go` | Yes | `ls -l` 3.1K |
| `internal/component/plugin/sysproc.go` | Yes | `ls -l` 2.2K |
| `internal/component/plugin/childenv.go` | Yes | `ls -l` 2.4K |
| `internal/component/plugin/doctor/check_shell.go` | Yes | `ls -l` 2.1K |
| `internal/component/config/register_plugin_declarations.go` | Yes | `ls -l` 1.6K |
| `test/plugin/plugin-declarations-external.ci` | Yes | `ls -l` 2.9K |
| `test/plugin/plugin-declarations-states.ci` | Yes | `ls -l` 2.8K |
| `test/plugin/plugin-declarations-pipes.ci` | Yes | `ls -l` 2.6K |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2, AC-3 | The ze binary answers before the handler, and an empty declaration is not silence | `go test ./internal/component/plugin/cli`: `ok github.com/ze-software/ze/internal/component/plugin/cli 1.789s`, run 2026-09-09 in this closure context |
| AC-1, AC-6 (SDK half) | `RunOrDeclare` writes the declaration and does not activate | `go test ./pkg/plugin/sdk`: `ok ... 4.302s` |
| AC-4, AC-9 | Five states, one row each, the bare form starts nothing | `--- PASS: TestDeclarationRowsCarryEveryState (1.02s)`, `--- PASS: TestDeclarationRowsCoverEveryRegisteredPlugin (0.00s)`, `--- PASS: TestDeclarationRowsCoverAPluginThatNeverRegistered (0.00s)` |
| AC-7 | The query answer equals the live Stage 1 declaration | `--- PASS: TestDeclarationAnswerMatchesStageOne (0.00s)` |
| AC-8 | One payload, every rendering | `--- PASS: TestEvidenceAndMarkersAreTheCurrentPopulation`, and `ok github.com/ze-software/ze/internal/test/localdatacoverage 0.323s` |
| Budget boundary | Zero or less is refused | `--- PASS: TestDeclarationBudgetRefusesNonPositive (0.00s)` |
| The stop | Reaches the plugin, not the shell | `--- PASS: TestDeclarationQueryStopsThePluginTheShellStarted (1.00s)` |
| Lint | No finding in any file this spec wrote | `./le verify lint run scope "<the ten packages>"`: 2 issues, both the pre-existing `nilnil` in `internal/component/plugin/leaf_test.go` already recorded in `plan/journal/nil-nil-return.md` |
| Command contract | Both new wire methods are paired with a handler | `./le docvalid command-contract` exit 0: `ze-show:plugin-declarations` and `ze-show:plugin-declarations-config` both listed against `ze-plugin-show-cmd` |
| CLI grammar | Keyword before value | `./le cli-grammar`: 0 grammar findings; the 2 `flag-in-yang` are committed prose in `ze-hub-conf.yang`, already journaled |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `show plugin declarations` typed at the CLI | `test/plugin/plugin-declarations-external.ci` | Yes: seq=1 runs the bare form with no daemon and asserts `mrt +internal +declared`; seq=3 runs the config form through `ze cli -c` and reads the started plugin's own declaration out of the JSON rendering |
| `ze plugin <name>` started with the mode variable | `TestQueryModeAnswersBeforeTheHandler` (`internal/component/plugin/cli/main_test.go`) | Yes: the fake plugin's `CLIHandler` records that it ran, the answer is parsed with `rpc.ParseLine`, and the handler never ran |
| a third-party plugin's `main` | `TestQueryModeSkipsActivation` (`pkg/plugin/sdk/sdk_query_test.go`) | Yes: the activation function records that it ran and is never called; the sibling test proves it IS called with the variable absent |
| the pipe surface | `test/ui/pipe-local-command.ci` | Yes: the compiled walk executes both commands with `| json compact` and reads the six fields, and the `.ci` demands the `20/20` line the walk prints only after every assertion passes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed, with one field named | `TestDeclarationAnswerMatchesStageOne`. `Run` derives one field the declaration does not state, `WantsValidateOpen`, and that derivation was broken until this spec fixed it |
| A-2 | confirmed | `cli.Run` holds the `*registry.Registration` from `registry.Lookup` before `reg.CLIHandler`, read at the producer 2026-09-08 |
| A-3 | confirmed | Re-measured at every producer on 2026-09-08, with three corrections and seven additions recorded in Current Behavior |
| A-4 | confirmed | The two forks are now one composition: `plugin.Shell`, `plugin.ChildPathEnv(plugin.EngineBinDir(), ...)` and `KillGroupOnCancel` are called by `(*Process).startExternal` and by `runQuery` alike, so a run string the daemon starts is started the same way by the reader. `test/plugin/llgr-egress-state-unloaded.ci` starts `run "ze plugin bgp-gr"` live, and `plugin-declarations-external.ci` queries `run "ze plugin mrt"`, the same shape |
| A-5 | confirmed | `TestQueryChildEnvDropsEveryPluginVariableSpelling` proves no `ze.plugin.` variable reaches the child under either spelling, so an unaware plugin has no hub host, port, token, CA or name and cannot join a live daemon. `plugin-declarations-states.ci` shows the unaware plugin reported `no-answer`. The residual for a third-party binary that runs its live start anyway is R-1, in Known Limitations |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| 1, 3, 6 user-facing feature and CLI command | `docs/architecture/cli/plugin-modes.md`, `docs/features/introspection.md` | Yes: the state table matches the five constants in `declarations.go`; the budget sentence matches `declarationBudget`; the config-block rule matches `declarationRows`. The hand-written section owed by `docs/guide/command-reference.md` is Review Gate finding 10 |
| 4, 8 API/RPC and plugin SDK | `docs/architecture/api/process-protocol.md`, `docs/plugin-development/protocol.md`, `ai/rules/plugins.md` | Yes: both pages name `rpc.WriteDeclaration` as the one writer, and the SDK page's `main` shape matches `RunOrDeclare`'s signature |
| 12 internal architecture | `docs/architecture/plugin/plugin-system.md` | Yes: the population sentence matches `registry.SetupResults`, and the stop sentence matches `KillGroupOnCancel` |
| 15 registered command inventory | `./le docvalid command-contract` | Yes: both wire methods listed and paired |
| 16 changed files under existing anchors | `./le doc check verify`, source-anchor stage | Yes: every anchor this spec wrote resolves. Four pre-existing stale anchors remain, none in a file this spec touched |
| Runtime dependency owes a doctor check | `internal/component/plugin/doctor/check_shell.go`, `internal/core/diagnostic/codes.go` | Yes: `plugin-shell` registered at order 702, `doctor-plugin-shell-missing` declared with title, description and repairs; `TestDoctorDependencyInventory` carries the `binary/plugin-shell` row |
| 2, 5, 7, 9, 10, 11, 13, 14 | N-A | No config node, no plugin added, no wire format, no RFC, no test infrastructure change, no daemon behavior comparison, no route metadata, no metric. `grep -rn "show plugin declarations" docs/` names only the pages listed above |

## Core Insight

A mode a plugin READS is a promise. A mode it never REACHES is a guarantee. The
19 runners that program nftables, install XFRM policies or call `setrlimit`
before they declare are the measurement that decides the shape: the answer must
be written by a process that already holds the declaration, so the runner is
never entered at all. Where Ze does not own the binary that guarantee is not
available, and the honest move is to give that population an entry point and
call it convention in those words, rather than write one sentence that reads as
a guarantee for both. `plan/learned/013-inertness-is-unreachability.md` carries
it.
