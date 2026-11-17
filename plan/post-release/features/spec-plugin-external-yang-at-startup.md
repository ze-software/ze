# Spec: plugin-external-yang-at-startup

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P3 |
| Release reason | Compile external plugins own YANG at startup: a new supported plugin-config surface, not a current compiled-in schema defect. |
| Scope | plugin |
| Depends | - |
| Phase | 8/10 |
| Handoff | - |
| Updated | 2026-11-17 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner goal (Thomas, 2026-10-16): "I want people to be able to write their own
plugins in Go or any other language and load them at startup".

Owner statement of the design (Thomas, 2026-11-15): "we could register any piece
of software via the API. It will register his Yang; he would be informed of the
state of the Yang on commit and on start, and would be able to do what he needs
with it." And, of this spec: "if right now third-party developers cannot
correctly develop this specific plugin, then that's the first step".

Owner statement of the load order (Thomas, 2026-11-15): "He learn about it by
reading the config. It is in the design." And: "We add a plugin block into the
configuration to declare the plugin to load, which was optional for our plugin,
and then the plugin could read its section. It was also possible to put the
plugin via the command line." "That is done before the protocol is used because
to start the protocol you must run the plugin." "You can also declare a plugin
by adding it to the command line instead of the configuration. Which would
allow for example a plugin which has no configuration to still run with Ze."
Every plugin compiled into `ze` counts as declared with no entry.

Owner ruling (Thomas, 2026-11-15): "the plugin can only be changed on
restarting ze".

Owner ruling (Thomas, 2026-11-16, given after phase 6 was committed as
`d8caeb5c2d`): "plugin command requiring external plugins which can not be
started from the code when no daemon is running to pass it to are not
available, for command built-in then yes you should run them as we can look
them up in the yang hardcoded in the binary". The question put to him: with no
daemon running, `ze config validate` and the other read-only commands started
every program the config's `plugin` block names, in declare mode, to read its
YANG, so inspecting a config someone else wrote ran their command. What it
decides: with no daemon running Ze starts no external plugin's program, and a
config section only such a plugin's schema defines is not available to a
command then; a compiled-in plugin is unchanged, because its YANG is in the
binary. It reverses the design decision "Offline commands read the same
declaration through declare mode", whose rejected alternative "refuse to
validate a config with an external root" is now the design, and it narrows the
constraint "what `ze config validate` accepts is what boots" to one direction:
validate never accepts what boot refuses, and boot accepts a section validate
calls not available. Every row below that said otherwise is corrected in
place.

An external plugin (any language, a separate process speaking the plugin
protocol) declares a YANG module in its registration. Today Ze stores that text
and never joins it to the config schema, so the operator cannot write config
under the plugin's own root: the config is refused at load, before the plugin
has started. This spec builds the load order the design page already draws
(`docs/architecture/hub-architecture.md`, "Startup Flow Diagram"): Ze reads the
plugin declarations of the config and the command line, starts those plugins,
takes each one's module at its Stage 1 declaration, joins the modules to the
schema every reader uses, parses the full config against the combined schema,
and delivers the plugin's section to it at start and on commit.

Owner ruling (Thomas, 2026-10-16): "if a user use an external plugin the
security is on him". There is no trust sandbox. Two refusals still apply:

| Refusal | Why |
|---------|-----|
| Invalid YANG, naming the plugin and the module | A module that does not compile has no schema to parse against |
| A module whose name or namespace clashes with a compiled-in module | The schema would be ambiguous |

Trust design (signing, allow-lists, sandboxing) is out of scope.

In scope: the schema an external plugin brings at daemon start, for config.
By the ruling above, the set of plugins and their modules is fixed while the
daemon runs: a reload that would change it is refused, naming the restart.
A plugin's command argument model is decision D-3.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/plugin-infra.md`, `ai/doors/config-yang.md`, `ai/doors/hub-engine.md`, `ai/doors/le-tooling.md`
  → Constraint: `pkg/plugin/rpc` field names are wire. This spec changes no wire field: the declaration and its `schema` object stay as they are.
  → Constraint: what `ze config validate` accepts is what boots (`test/plugin/config-validate-agrees-with-boot.ci`, whose header forbids "a second config runtime with its own parser, in any form"). The first pass of this spec MUST be the same `Parser` over a sub-schema. Narrowed by the owner ruling of 2026-11-16: validate takes the first pass and the same parser, but starts no plugin, so it never accepts what boot refuses and it refuses, as not available, a section only an external plugin's schema defines.
  → Constraint: a declaration is a pure value; a plugin queried in declare mode starts nothing (`ai/rules/plugins.md`).
  → Constraint: a backend that cannot apply a config exactly refuses it. A reload that would need a schema the daemon does not hold is refused, never accepted and left unapplied.
- [ ] `docs/architecture/api/process-protocol.md`, "Query Mode"
  → Decision (owner ruling, 2026-11-16): query mode serves no command that parses a config. With no daemon Ze starts no external plugin, in query mode or any other. Query mode stays for `show plugin declarations` and the conformance check, whose purpose is to run the program the operator names.
- [ ] `docs/architecture/hub-architecture.md`, sections 2 and 3, "Extended 5-Stage Protocol" and "Startup Flow Diagram"
  → Decision: this is the design, and the spec follows it. Step 1 "minimal parse - just plugin blocks"; step 2 fork the plugins; step 3 Stage 1 with the schema, then "BARRIER: All plugins declared"; step 4 "Hub parses full config against combined YANG", then Stage 2. A plugin is started once, and its live Stage 1 declaration is the source of its module.
  → Constraint: the page's status header says the standalone orchestrator was deleted because its second parser disagreed with validate. The minimal parse here is the one `Parser` over a sub-schema.
- [ ] `docs/plugin-development/schema.md`, `docs/plugin-development/protocol.md`
  → Constraint: the author-facing contract this spec makes true. Its example modules (`acme-monitor`, `my-plugin`) do not end in `-conf`, so they would add no config even once joined; the examples are corrected.
- [ ] `docs/architecture/config/yang-config-design.md`
  → Constraint: `schemaFromResolved` takes top-level config nodes only from modules whose name ends in `-conf`; an augment attaches to its target's tree whatever the augmenting module is named.
  → Constraint: every reader of a checked module set takes a `Resolved`; no reader is handed an unchecked set. The join answers a `Resolved` or an error.
- [ ] `plan/post-release/engineering/spec-config-yang-compiled-at-build-time.md`
  → Decision: not a dependency. Nothing of it exists in the tree and it is at `design`. Today's loader already joins module text. This spec defines one join function; that spec's last commit re-implements the body of that function and changes none of its callers.
  → Constraint: that spec's AC-7 (a start with no external plugin compiles nothing extra) is kept here as AC-9.
- [ ] `plan/learned/DESIGN-HISTORY.md`, Configuration and Plugin system
  → Constraint: row 293, "Startup fails if any `ze:validate` reference has no registered validator". An external module that names a validator, a reference source or any other daemon-side registration the daemon does not hold is refused at the join.
  → Constraint: row 334, extend a container with `augment`, never with a second definition. An external module that redefines an existing top-level node is refused; it reaches a shared container such as `service` by augmenting it.
  → Constraint: row 846, "Plugin YANG opt-in loading — chicken-and-egg" was abandoned for internal plugins, whose YANG is always loaded. This spec is the external case, where the module is unknown until asked for.

### RFC Summaries (Scope: protocol)
- N-A: no wire protocol; YANG 1.1 (RFC 7950) checks are the loader's, unchanged.

**Key insights:** (minimal context to resume after compaction)
- The config is parsed at the top of `runYANGConfig`, long before any external plugin is forked (startup phase 2). The schema must be known before that parse.
- The design starts the plugins named by the `plugin` block and the command line before the full parse, and takes their modules at Stage 1. Today the plugin server is built from the fully parsed config, so the boot order changes: the plugin server needs only the first pass to accept a Stage 1.
- Offline `ze config validate`, and every other offline command that parses a config, has no hub. There no plugin is started (owner ruling, 2026-11-16): the command parses against the compiled-in schema and says that a section of an external plugin is not available. Only boot feeds the join function.
- The schema is a process-global: about 40 production files call `config.YANGSchema()`. The external modules must enter the set those calls answer from, or the editor, web and gNMI never see them.
- `loadYANGModules` already takes module text, but it drops a duplicate module silently, checks no namespace, and its result reaches only one parse.
- `PluginOnlySchema` cannot parse a full config: the parser refuses every unknown top-level keyword.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/hub/main.go` - `run`, `runYANGConfig`: parse first, then hub config, provider, plugin server, then the five startup phases
- [ ] `internal/component/config/loader.go` - `LoadConfig`, `loadConfig`, `parseTreeWithYANG`, `finalizeConfigLoad`
- [ ] `internal/component/config/yang_schema.go` - `PluginOnlySchema`, `YANGSchema`, `YANGSchemaWithPlugins`, `loadYANGModules`, `schemaFromResolved`, `validatorFromResolved`
- [ ] `internal/component/config/parser.go` - `parseRoot`, `parseBody`
- [ ] `internal/component/config/yang/register.go`, `loader.go` - `RegisterModule`, `modulesGeneration`, `DefaultLoader`, `Loader.AddModuleFromText`, `Loader.Resolve`
- [ ] `internal/component/plugin/inprocess.go` - `CollectPluginYANG`, `GetInternalPluginYANG`
- [ ] `internal/component/plugin/declarations.go` - `runQuery`, `queryChildEnv`, `declarationFromStdout`, `queriedDeclarationRow`
- [ ] `internal/component/plugin/server/startup.go` - `engineStartupSink.onRegistration`, `registrationFromRPC`, `deliverConfigRPC`
- [ ] `internal/component/plugin/server/reload.go`, `startup_autoload.go` - `groupRootFor`, `selectAutoLoad`
- [ ] `internal/component/config/plugin_verify.go` - `BuildPluginConfigSections`, `ExtractConfigSubtree`
- [ ] `internal/component/config/cli/cmd_validate.go` - the offline validate entry

Verified at the producer on 2026-10-16 and again on 2026-11-15:

| Fact | Evidence |
|------|----------|
| External plugin YANG is stored and never joined | `registrationFromRPC` (`plugin/server/startup.go`) sets `reg.PluginSchema` from `input.Schema.YANGText`; no non-test code reads it. An earlier version of this spec named a function `handleRegistration`, which does not exist |
| The config is parsed before any external plugin exists | `runYANGConfig` calls `LoadConfig` first; an external is forked in startup phase 2, after every phase 1 plugin finished its five stages |
| The boot parse schema is compiled-in modules only | `loadConfig` calls `plugin.CollectPluginYANG(cliPlugins)`, which reads `registry.Lookup(name).YANG` of INTERNAL registrations |
| A config block under an external plugin's own root is refused | `parseRoot` refuses with `unknown top-level keyword: <root>`; observed 2026-10-16 with `ze config validate` (exit 1) |
| A road for module text exists | `parseTreeWithYANG` → `YANGSchemaWithPlugins` → `loadYANGModules` → `Loader.AddModuleFromText` → `Loader.Resolve`, which runs every check |
| That road is not usable as it stands | `loadYANGModules` swallows any error containing `duplicate module`, so a module named like a compiled-in one is dropped silently. Nothing checks a duplicate namespace. `Schema.Define` merges a redefined top-level container. The schema it builds serves the one parse inside `loadConfig` and no other reader |
| The schema is a process-global | `config.YANGSchema()` and `YANGValidator()` sit on `yang.DefaultLoader()`, a one-entry cache keyed by `modulesGeneration`. `RegisterModule` is documented as init-only and not safe for concurrent use |
| Three caches would not follow a later change | gNMI `configSchema` (`schemaOnce`), `yang.commandArgDefs`, and the completer's `valueSchema` are each built once |
| `PluginOnlySchema` has no caller and cannot parse a full config | It builds a schema holding the `plugin` module alone; `parseBody` then refuses every other root (`TestPluginOnlySchemaRejectsUnknown`) |
| Query mode reads the schema and drops it | `declarationFromStdout` unmarshals the whole `rpc.DeclareRegistrationInput`; `queriedDeclarationRow` keeps commands and pipes only. Its one caller is `show plugin declarations` |
| Offline validate does not take the boot road | `config/cli/cmd_validate.go` parses with `YANGSchema()` directly |
| Stage 2 delivery needs no change | `BuildPluginConfigSections` and `ExtractConfigSubtree` walk the tree by path; `groupRootFor` matches a declared root on whole path segments, so a sub-path root such as `service/chrony` works |
| An unregistered `ze:validate` is accepted silently | `validatorFromResolved` logs the result of `CheckAllValidatorsRegistered`, and `applyCustomValidators` skips a nil validator |
| Doctor would call an external root unclaimed | `checkConfigClaims` builds its population from `pluginregistry.ConfigSectionRootsMap()`, internal registrations only |
| A reload does not start an added external plugin | `Server.config.Plugins` is fixed at `NewServer`; `selectAutoLoad` starts only owners found in the compiled-in registry. Read at code, not proven by a run |
| A third-party Go plugin runs in process only when compiled in | `plugin/all/all.go` is generated by `./le repo update`; there is no runtime Go plugin loading |

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|-------------------------------------------|--------------------|--------------------------------------------------------------------|
| Stage 1 declaration with a `schema` object | `pkg/plugin/rpc/types.go` `DeclareRegistrationInput`, `SchemaDecl` | Carries the module name, namespace and text | Extends: unchanged on the wire; it becomes the one source of an external module |
| Plugin declaration in the config and on the command line | `ze-plugin-conf.yang` `plugin { external <name> }`; `config/loader.go` `ExtractPluginsFromTree`, `MergeCliPlugins`; `plugin/resolve.go` `ResolvePlugin` | Names the plugins to load, as a block or as a command-line path or command; a compiled-in plugin needs no entry. Read only after the full parse | Extends: the same two sources are read in the first pass, before the full parse and before any protocol |
| Five-stage startup and its barrier | `plugin/server/startup_driver.go` `runStartupHandshake`; `process-protocol.md` | Stage 1 declaration, a barrier, Stage 2 config, per tier, all after the full parse | Extends: the Stage 1 of a declared external plugin runs before the full parse; its Stage 2 and later stages stay in its tier |
| Stored declared schema | `plugin/server/startup.go` `registrationFromRPC`, `reg.PluginSchema` | Stored, read by nothing | Extends: it becomes the source of the plugin's module at boot |
| Query mode | `internal/component/plugin/declarations.go` `runQuery`, `declarationFromStdout`; `process-protocol.md` "Query Mode" | Starts a configured plugin with `ZE_PLUGIN_MODE=declare`, reads its Stage 1 line, kills the group on a budget | Does not apply (owner ruling, 2026-11-16): no command that parses a config starts a plugin. Query mode stays for `show plugin declarations` and the conformance check |
| Module text road | `config/yang_schema.go` `loadYANGModules`, `YANGSchemaWithPlugins`; `config/loader.go` `parseTreeWithYANG` | Adds module text to a private loader for one parse, dropping duplicates | Replaces: one strict join function takes its place; the silent duplicate drop is deleted |
| Internal plugin YANG for the parse | `plugin/inprocess.go` `CollectPluginYANG` | Hands the parse the YANG of internal plugins named on the command line, which `init()` has already registered | Replaces, if A-3 holds: it adds nothing the registry lacks, so it is deleted with the road above |
| Plugin-only schema | `config/yang_schema.go` `PluginOnlySchema` | A schema for the `plugin` block alone, no caller | Extends: it gets its caller, the first pass, with a parser mode that skips the roots it does not know |
| Module registry and its generation | `config/yang/register.go` `RegisterModule`, `modulesGeneration`; `loader.go` `DefaultLoader` | Init-time registration; every schema reader follows the generation | Extends: one more layer, the external modules, set once per process before the first full parse, advancing the generation |
| Startup integrity check of validators | `config/yang_schema.go` `validatorFromResolved`, `yang.CheckAllValidatorsRegistered` | Logs a `ze:validate` with no registered function | Extends: over an external module the same check refuses the join |
| Config delivery by declared root | `config/plugin_verify.go` `BuildPluginConfigSections`; `plugin/server/reload.go` `groupRootFor` | Cuts the subtree of each `wants-config` root, at start and on reload | Does not apply a change: it works once the tree holds the root |
| Config transaction callbacks | `docs/architecture/config/transaction-protocol.md` | Verify, apply, rollback and committed, per plugin root | Does not apply a change: this is "informed on commit" |
| Plugin failure reporting | `plugin/server` `PluginFailed`, `RecordStopped`; doctor `doctor-plugin-stopped` | Stops a plugin that fails a stage and raises a doctor finding | Extends: a plugin that restarts with a module different from the published one fails through it |
| Doctor config-claims check | `checkConfigClaims`, `pluginregistry.ConfigSectionRootsMap` | Reports a configured root no internal plugin claims | Extends: the roots running external plugins declared join its population |
| `ze schema` exec of a plugin with `--yang` | `config/schema/cli/main.go` `getExternalPluginYANG` | Runs a plugin binary with a flag only Ze's own plugin CLI handles | Replaces: the exec is deleted. The schema CLI is an offline command, so by the owner ruling of 2026-11-16 it starts no plugin and shows compiled-in modules only with no daemon (phase 8) |
| `ConfigProvider.RegisterSchema` | `pkg/ze` `ConfigProvider`, `config.Provider.RegisterSchema` | Stores text, no caller | Does not apply: it is on the public interface and stores nothing a reader uses; left as is, recorded in Known Limitations |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze start <config>`, `ze config validate <config>`, and every other entry that parses a config file, where the config holds a `plugin { external <name> { run ... } }` entry whose plugin declares a module in `schema.yang-text`.

### Transformation Path
The daemon, at start:

1. **First pass, before any protocol.** The config text is parsed by the same `Parser` over a sub-schema (`PluginOnlySchema`, widened to what a plugin needs to be run: the `plugin` block, its hub listener, and the `environment` block), in a mode that skips a top-level block it does not know (balanced braces, or the lines of that root in `set` form). The command-line plugins are merged in. The result is the list of declared external plugins, each with its run string. A compiled-in plugin is not in the list: its module is already registered.
2. **Run the declared plugins.** The plugin server accepts connections with what the first pass gave it, and each declared external plugin is forked. Running the plugin is what starts the protocol.
3. **Stage 1 and its barrier.** Each plugin sends its declaration, which carries its module. The daemon waits until every declared external plugin has declared or failed. A plugin that fails or stalls is kept as an outcome with its reason, under the failure handling that exists.
4. **Join.** One function takes the compiled-in resolved set and the declared modules and answers a combined resolved set, or an error that names the plugin and the module. It refuses the cases in the Refusals table.
5. **Publish.** The joined modules become the external layer of the process's module set, once, before the full parse. The generation advances, so every reader that asks for the schema after this point sees them.
6. **Full parse.** The config is parsed against the combined schema by the unchanged road. An unknown top-level keyword that remains is reported with each plugin that did not declare, so the operator sees why a root is not known.
7. **Stage 2 and commit.** The plugins held at the barrier continue in their startup tier with the unchanged stages: each receives the subtree of each root it wants, at start and on each commit.

A command that parses a config with no daemon (`ze config validate` and the other offline config commands) takes step 1, keeps the names of the declared external plugins, and parses the full config against the compiled-in schema. It takes none of steps 2 to 5 and starts no program (owner ruling, 2026-11-16). A name the compiled-in schema does not define is refused, and when the config declares an external plugin the refusal is one operator error: what was refused, the plugins the config declares, that Ze reads such a plugin's schema when `ze start` runs it and that this command starts no plugin, and the daemon form to type (`show config validation <file>`), with `ze cli` as the fix. It never says a plugin owns the refused name.

While the daemon runs, the published set never changes (owner ruling). A reload whose plugin declarations differ from the ones the daemon started with is refused, naming the restart. A plugin that is restarted and declares a module different from the published one is failed, naming the restart.

### What the audit showed about the order of `runYANGConfig`

Read at code on 2026-11-16 (`cmd/ze/hub/main.go` `runYANGConfig`, from the config parse to `apiServer.StartWithContext`).

What moves before the full parse, because it needs the first pass alone:

1. The hub listener: `config.ExtractHubConfig`, which reads `plugin/hub` and nothing else.
2. The daemon's certificate authority: `zepki.LoadOrGenerateRoot`, which reads the store. Whether it needs `zepki.Load` of the tree's PKI block first was not settled by reading.
3. The process manager and its acceptor: `pluginmgr.NewManager`, `SetHubAuthority`, `SetHubConfig`, then `Manager.ensureAcceptor` and `plugin.NewHubAcceptor`.
4. The list of declared external plugins with what a fork reads: name, run string, encoder, respawn, stage timeout and the work directory, from the `plugin` block and the command line.
5. The fork and the connect-back of each declared external plugin (`process.Process.startExternal`), then the read of its Stage 1 declaration.
6. The join and the publish.

What stays after the full parse, because it reads the whole tree or the published schema:

1. `config.LoadConfig` itself, with its validators, the publication of the `environment` block and the full plugin list: `ExtractPluginsFromTree` adds the plugins a section declares inline, and `ExpandDependencies` reads the tree.
2. `kernelcap.Refuse`, `publishNameServers`, `CollectContainerPaths`, and the web, looking glass and MCP listener resolution.
3. The config provider (`lowerForPlugins`), the PKI block, the boot users, the API listeners and the authorization store.
4. `zePlugin.NewCoordinator` and its bootstrap. The coordinator has `SetConfigTree`, so it could be built earlier, but nothing needs it earlier.
5. `pluginserver.NewServer`: it takes `yang.DefaultLoader` at construction, so it is built after the publish. Its `ServerConfig` fields `ConfiguredPaths`, `DataPlane`, `ConfigNeedHolds` and `Plugins` come from the full tree.
6. The infrastructure hook, the config loader for reload, `engine.NewEngine`, the BNG subsystems, telemetry, `eng.Start` and `apiServer.StartWithContext`.

What blocks the order as the code stands, with the smallest change that unblocks each:

1. `Manager.SpawnMore` refuses until `Manager.StartAll` ran, and `StartAll` is called by `Engine.Start`, which needs the config provider and the subsystems of the full tree. The manager needs an entry that starts the declared external plugins under a context before `StartAll`.
2. The handshake is one function, `runStartupHandshake`, that reads Stage 1 and runs the four later stages inside the tier's coordinator, and its Stage 1 sink needs a built `Server`. The early step needs no `Server` if it only reads and keeps each declaration: the tier's handshake then consumes the kept request and runs every check unchanged (R-11, A-7).
3. `runPluginPhaseWithin` spawns every plugin of its phase. It must take a process the early step already started (R-12).
4. The start path in `cmd/ze` parses the whole config before `hub.Run` (A-10, R-16).
5. With no external plugin declared, steps 5 and 6 of the first list do nothing and the order is today's, but the first pass still runs: it must not resolve the module set a second time (A-11).

### Refusals at the join

| Case | Refusal names |
|------|---------------|
| The module does not parse or resolve | The plugin, the module, and the loader's error with its location |
| The declared `module` or `namespace` field differs from the text | The plugin and both values |
| The module name equals a module already in the set, at any revision | The plugin, the module, and the holder of the name |
| The namespace equals a namespace already in the set | The plugin and both modules |
| A top-level node of the module already exists and is not reached by `augment` | The plugin, the node, and the module that defines it, with the `augment` form to use |
| The module names a validator, reference source, decorator, backend or other registration the daemon does not hold | The plugin, the module, the extension and the name it uses |
| The module would contribute nothing: its name does not end in `-conf` and it augments nothing | The plugin, the module, and the naming rule |

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin process to hub, at boot | The Stage 1 declaration over the plugin connection carries the module | No |
| Offline command to plugin binary | Not crossed (owner ruling, 2026-11-16): a command with no daemon starts no plugin | N-A |
| Plugin declaration to module set | The join function, text in, checked `Resolved` out | No |
| Module set to every schema reader | The registry generation, which the schema caches key on | No |
| Hub to plugin | Stage 2 `configure` and the config transaction carry the section under the plugin's root | No |

### Changed-Wire Census
The declaration, its `schema` object, the query-mode line and the Stage 2 section keep their fields and encoding. One carrier changes: the schema CLI stops running a plugin binary with `--yang`.

**Search evidence:** scope: every text file under the repository except `vendor/`, `gokrazy/`, `bin/`, `plan/` and `tmp/`, in Go, YANG, shell, Python, `.ci` and `.et`. Terms: `yang-text`, `YANGText`, `SchemaDecl`, `--yang`, `ZE_PLUGIN_MODE`. Positive control: the search finds the known writer type `pkg/plugin/rpc/types.go` `SchemaDecl` and the known reader `internal/component/plugin/server/startup.go` `registrationFromRPC`. Non-Go hits: `internal/core/ipc/yang/ze-plugin-engine.yang` (the declaration's model) and `test/plugin/filter-wire-batch.ci` (declare mode, no schema). No shell or Python carrier of the schema exists.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|---------------------------------------|
| Stage 1 declaration, `schema` object | writer | `pkg/plugin/rpc/types.go` `SchemaDecl`, re-exported by `pkg/plugin/sdk/sdk_types.go`; sent by `sdk.RunOrDeclare` and by any third-party plugin | Search hit; read at code | Preserved, no field change |
| Stage 1 declaration, `schema` object | writer (model) | `internal/core/ipc/yang/ze-plugin-engine.yang` | Search hit | Preserved |
| Stage 1 declaration, `schema` object | reader | `internal/component/plugin/declarations.go` `declarationFromStdout` | Read at code: it unmarshals the whole declaration | Preserved; its caller stops discarding `Schema` |
| Stage 1 declaration, `schema` object | reader | `internal/component/plugin/server/startup.go` `registrationFromRPC`, into `internal/component/plugin/registration.go` `PluginSchemaDecl` | Search hit; read at code | Preserved; the stored value gains a reader |
| Stage 1 declaration, `schema` object | reader | `internal/component/plugin/server/subsystem.go` `SubsystemHandler` | Search hit; its `Register` and `NewHandler` have no production caller | Preserved, untouched |
| `--yang` flag of a plugin binary | writer | `internal/component/plugin/cli/cli.go`, Ze's own plugin CLI | Search hit | Preserved: the flag stays for `ze plugin <name>` |
| `--yang` flag of a plugin binary | reader | `internal/component/config/schema/cli/main.go` `getExternalPluginYANG` | Search hit; a third-party binary has no such flag and would run its live start | Changed: the reader is deleted; with no daemon the schema CLI starts no plugin (owner ruling, 2026-11-16) |

### Integration Points
- `config.PluginOnlySchema` and `Parser`, for the first pass; `MergeCliPlugins` for the command line.
- `cmd/ze/hub` `runYANGConfig` and `pluginserver.NewServer`, whose order changes: the plugin server accepts a Stage 1 before the full parse.
- `runStartupHandshake` and `registrationFromRPC`, for the early Stage 1 and the stored module.
- `config.FirstPass` alone, for the offline commands: they read the names of the declared plugins and start nothing.
- `yang.Loader.AddModuleFromText` and `Resolve`, inside the join.
- `yang.RegisterModule`'s registry and `DefaultLoader`, for publishing.
- `checkConfigClaims`, for doctor.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | To prove at implementation: validate, boot and reload call one function for steps 1 to 4 |
| No unintended coupling (components stay isolated) | No | `config` already imports `plugin`; the query stays in `plugin`, the join in `config/yang` |
| No duplicated functionality (extends existing, does not recreate) | No | Existing Mechanisms: every row is extended or replaced, none rebuilt beside |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A: startup path, not a per-route path |
| Registration over hardcoding, outbound | No | The module arrives by declaration; no core package names a plugin |
| Registration over hardcoding, inbound | No | To prove at implementation: the refused-extension check derives from each extension's own registry, never from a list of extension names |

## Open Owner Questions

| # | Question | Recommendation |
|---|----------|----------------|
Answered on 2026-11-15, recorded in the Task section: the load order (the design page's, plugin declarations read before any protocol, from the config or the command line), and that a plugin changes only when `ze` restarts.

Design approved by the owner on 2026-11-16, with the order to implement it now ("Look is good to me, you can implement that"). That answers D-4 for the config half: it is built now. D-3 stays open and blocks no phase below.

| # | Question | Recommendation |
|---|----------|----------------|
| D-3 | Command argument model of an external plugin (journal row 2026-11-02, owned here): help, completion and checking of the arguments of a plugin's own commands, from a module the plugin brings | Keep it in this spec as its last phase: a plugin that wraps a program needs its config and its commands. It makes one declaration carry a config module and a command module, a wire change |
| D-4 | Priority and bucket: the spec is post-release P3 | The owner's to set. The design waits on no other spec |
| Q10 | Startup budget: the declared plugins reach Stage 1 before the full parse, plus one resolve of the module set | Measure at implementation on a known host |

## Audit findings for the owner

Found by the audit phase on 2026-11-16. The load order stands, and no broken assumption invalidates a step of the approved design: the two broken rows (A-10, A-11) and the orderings listed under Data Flow each name a change inside the design. One fact is recorded here because it changes what an author is told, not what is built.

| # | Finding | Evidence (file and symbol) | What it touches | What follows |
|---|---------|----------------------------|-----------------|--------------|
| F-1 | The compiled-in modules do not share `service` by `augment`. Six plugin modules each define a top-level `container service`, and the config schema merges same-named top-level containers. Only one of the six is always built, the DNS resolver's | `config.Schema.Define` and `mergeContainer` (`internal/component/config/schema.go`); `schemaFromResolved` (`yang_schema.go`), whose comment states the merge; `feature-gates.txt` for the five gated plugins; `plugin/all/all.go` for `dnsresolver` | AC-6, the Refusals row "a top-level node of the module already exists and is not reached by `augment`", and the author guide | AC-6 holds when the external module augments the `service` of `ze-dns-resolver-conf`. An author who wants a child under `service` therefore imports the DNS resolver's module, which the guide must say. The redefinition refusal is applied to external modules only, or it refuses the compiled-in set. Whether `service` should get an owner module of its own is the owner's to raise; this spec does not need it |

The door `ai/doors/config-yang.md` says a second definition of a container replaces it. For two `-conf` modules that each define the same top-level container, `Schema.Define` merges them. The page and the code disagree on that case.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The plugin server can accept a Stage 1 with only what the first pass yields (the `plugin` block, its hub listener, the `environment` block, the CA from the store) | `runYANGConfig` builds `ServerConfig` from the full tree today, but the fields a Stage 1 reads are the hub listener and the plugin list | The first pass widens to the roots the server needs, or those fields are set after the full parse and before Stage 2 | Read at code, 2026-11-16. Fork and connect-back need only first-pass values: `plugin/manager` `Manager.ensureAcceptor` builds `plugin.NewHubAcceptor` from the hub config (`config.ExtractHubConfig` reads `plugin/hub` alone) and the CA root (`zepki.LoadOrGenerateRoot`, the store alone); `process.Process.startExternal` reads the run string, the work directory, the acceptor and the daemon's environment. Of `ServerConfig`, a Stage 1 reads `Plugins` only (`Server.hasConfiguredPlugin`); `ConfiguredPaths`, `DataPlane` and `ConfigNeedHolds` are read by `runPluginStartup` before phase 1 and are plain fields that can be set after the full parse; `Hub` has no reader in the server. `engineStartupSink.onRegistration` reads `Server` state, not the tree: `registry`, `loadedPlugins`, `procManager`, `reactor`. Nothing a Stage 1 reads comes from the full tree, but two orderings block it today, listed under "What the audit showed about the order of `runYANGConfig`" | confirmed |
| A-2 | A plugin held at the Stage 1 barrier while the daemon parses the config is not failed by the stage timer | The timer measures a plugin that stalls; here the hub is the one working | The timer is not armed for the hub's own parse | Read at code, 2026-11-16; the `.ci` run is still owed. The only stage timer is armed in `Server.stageTransition`, which calls `StartupCoordinator.WaitForStageProgress` with the bound `startupStageStall` answers. It runs in a plugin's handshake goroutine after that plugin completed a stage, and it measures the silence of the other members of the tier: the time since any member last completed a stage (`lastProgress`, set by `SetStartTime` when the tier starts). With no coordinator `stageTransition` returns at once. So no timer measures the hub's own work, on one condition: the hold is not built as a wait inside a tier coordinator, where 5 s with no member progress fails the plugin. The hold is also longer than the parse: an explicit plugin's tier starts after every config-path plugin finished its five stages. Two other bounds exist: `startExternal` gives a forked plugin 30 s to connect back, and the Stage 1 read (`PluginConn.ReadRequest`) has no timer of its own (R-13). The Go SDK arms no timer between its Stage 1 and its Stage 2 (grep of `pkg/plugin/sdk/sdk.go`: only the two 10 s dial bounds) | confirmed |
| A-7 | The checks a Stage 1 runs against other plugins (`Dependencies` through `isPluginLoaded`, registry conflicts) can run when the plugin's tier starts, after the full parse | Those plugins are not loaded at the early Stage 1 | The early Stage 1 stores the declaration, and the checks move to the tier | Read at code, 2026-11-16, against the table `stage1Checks` (`plugin/server/stage1_checks.go`). Every check can run at the plugin's tier from a stored declaration: each takes the decoded declaration and the process, and none reads the connection. Three must run there and not earlier. `dependencies` reads `Server.isPluginLoaded`, filled by `runPluginPhaseWithin` when a phase starts, so at an early Stage 1 a dependency on a config-path plugin such as `bgp` is refused wrongly. `registration-conflicts` writes the daemon registries (`PluginRegistry.Register`, `registerPluginPipes`, `registerPluginShapes`, `registerPluginFamilies`): written early, a clash would fall on the compiled-in plugin that registers later. `filter-insertion-paths` is marked isolated but reads the schema (`config.SchemaNodeModule`), so before the publish it cannot find a path under the plugin's own module. The tier assumes Stage 1 happens inside it: `handleProcessStartupWithin` sets the stage and index, `stageTransition` records the configure allowance while the coordinator is at Registration, and `rollbackStartupProcess` unwinds what `onRegistration` wrote | confirmed |
| A-8 | Withdrawn by the owner ruling of 2026-11-16: no offline command starts a plugin, so none depends on a declare-mode answer, and the "If wrong" column is now the design. As first written: a plugin that brings a schema can answer declare mode, for the offline commands | `process-protocol.md` "Query Mode"; `sdk.RunOrDeclare` does it for Go plugins | Offline validate cannot know the plugin's root and says so; boot is not affected | `external-yang-declare-only.ci`, a shell script that answers only declare mode. Not settled by the audit: it needs that test run. What exists: `plugin.QueryDeclaration` answers a `DeclarationQuery` whose `Declaration` holds the whole Stage 1 input, `Schema` included. Phase 6 built on it and the ruling took it out | broken |
| A-3 | Every internal plugin's `registry.YANG` is also registered by `init()`, so `CollectPluginYANG` adds nothing | The build-time spec's A-6; `loadYANGModules` comment on duplicates | `CollectPluginYANG` stays and feeds the join as internal modules | Read at code, 2026-11-16. Every non-test `YANG:` value of a `registry.Registration` (75 of them) is a generated `Ze...YANG` variable of a `yang/` package, and each of those variables is also passed to `yang.RegisterModule` from that package's `init()`; the one computed value, `getYANG` of `bgp/plugins/adj_rib_in`, returns `ZeAdjRibInAPIYANG`, registered too. `plugin.CollectPluginYANG` therefore adds no module the registry lacks: it re-adds registered text under another file name, and `loadYANGModules` drops it as a duplicate. Compared by variable name, not by a run of `registry.YANGSchemas()`. `CollectPluginYANG` has four callers, not one (see Files to Modify) | confirmed |
| A-4 | A plugin that owns a shared container tolerates a child an external module augments into it | Several compiled-in modules already share `service` and `environment` | The augment AC narrows to roots no internal plugin parses strictly, and the refusal table gains the case | Read at code, 2026-11-16; the `.ci` run is still owed. Six compiled-in plugins declare the root `service` (`dhcpserver`, `tftpserver`, `geodns`, `imageserver`, `dnsresolver`, `as112`), each receives the whole section and takes its own child, so each already receives children it does not know. Read for `dhcpserver` `parseDHCPSections`; the other five by grep of their config files, which shows no unknown-key refusal. `ntp` reads `environment/ntp` the same way. How `service` is shared is row A-9 | confirmed |
| A-5 | No schema reader caches a schema before the external layer is published | Boot parses first; the three once-caches belong to services started later | That reader keeps a stale schema: it is re-keyed on the generation | Read at code, 2026-11-16. Inside `run` and `runYANGConfig` no reader takes the schema before `LoadConfig` (grep of `cmd/ze/hub`: the readers are `readConfigWithStorage`, `service_web.go`, `session_factory.go`, `command_meta.go`, `infra_setup.go` and the gNMI service, all built after the parse). `config.YANGSchema` and `YANGValidator` follow a publish by themselves: `resolvedCache.get` keys on the `*yang.Resolved` that `yang.DefaultLoader` answers for the current generation. The three once-caches are built after boot (gNMI `Server.configSchema` on the first path translation, `Completer.valueSchema` per completer) or hold nothing a config module changes (`yang.commandArgDefs`). Because the set is published once and never again, a once-cache first built after the publish is never stale: an ordering to keep, not a re-keying. Two readers do sit before the publish point: `pluginserver.NewServer` takes `yang.DefaultLoader` and seeds the argument definitions, so it must be built after the publish; and the start path in `cmd/ze` parses the whole config before `hub.Run` (row A-10). Read again when the publish was built (2026-11-16), the audit missed three things, so the row is broken as written. (1) `ze` routes its own command line through `cli.YANGCommandTree` (`cmd/ze/ze_core_dispatch.go` `yangVerbs`), which builds the `cli/client` YANG state (`loadYANGState`) before the daemon's first pass, in every start. (2) Any route that judges arguments (`command.ValidateModelArgs`) builds `yang.commandArgDefs` on first use, and nothing orders that after the publish. (3) More once-built values exist than the three named: the gNMI model list (`modelsOnce`), the backend gate schemas of `iface`, `firewall` and `traffic`, the reserved interface names (`iface.ValidateIfaceName`), the hub's command metadata (`commandMetaSource`) and the headless harness's command tree. Settled by the publish: the two values that can be built first are keyed on the `Resolved` (`loadYANGState`, `headlessCommandTree`) or built again by the publish (`rebuildCommandArgDefs`); the others stay once-built, record themselves with `yang.KeepsModuleSet`, and a publish that would change the set after one of them is refused, naming it. The page lists each | broken |
| A-6 | Skipping an unknown top-level block in the first pass needs only balanced braces and the `set` line prefix | `docs/architecture/config/syntax.md` | The first pass needs the tokenizer's quoting rules; it already uses the same `Parser` | Read at code, 2026-11-16; the unit tests are still owed. Brace form: the tokenizer emits a quoted string as one `tokenString` and each brace as its own token (`tokenLBrace`, `tokenRBrace`), with `tokenSemicolon` ending a leaf statement, so `Parser.parseBody` can skip an unknown top-level name by counting brace tokens to the statement's end, quoting already handled. No such mode exists today: `parseBody` refuses when its lookup answers nothing, and `parseUnknownField` takes one value only. `set` form: the mode exists, `SetParser.SetPreMigration` skips unknown fields and records them (`UnknownFields`), and `parseSetConfig` already uses it for its second parse. Built in phase 4 (2026-11-16): `Parser.SkipUnknownRoots` with `skipStatement` for the brace form, and `SetParser.SkipUnknownRoots` with `skipsLine` for the set form. The set form did not reuse `SetPreMigration`: that mode skips an unknown field at any depth, which would lose a mistyped leaf of a plugin declaration, so the first pass skips unknown roots only. Tests written in `first_pass_test.go`; their run is owed | confirmed |
| A-9 | An external module can reach the shared container `service` by `augment` | AC-6 and the Refusals table | The AC-6 example and the redefinition refusal need the owner's choice | Read at code, 2026-11-16. It can, through one module. `service` has no single owner: six plugin modules each define a top-level `container service` (`ze-dhcp-server-conf`, `ze-tftp-server-conf`, `ze-geodns-conf`, `ze-dns-resolver-conf`, `ze-image-server-conf`, `ze-as112-conf`), and `config.Schema.Define` merges them (`mergeContainer`). Five sit behind a build tag (`feature-gates.txt`: `ze_as112`, `ze_geodns`, `ze_dhcpserver`, `ze_pxe`); `ze-dns-resolver-conf` is always built (`plugin/all/all.go`), so an augment that imports it holds on every build, and `schemaFromResolved` carries the augmented child into the merged container. `environment` has an always-built definer too, `ze-hub-conf`, which `ze-bgp-conf` augments. Consequence for the join: the compiled-in modules share `service` by a second definition, so the redefinition refusal judges external modules only | confirmed |
| A-10 | Every parse of the config in the start path runs after the publish | The spec names `runYANGConfig` as the first parse | A config holding an external root is refused before the daemon's first pass | Read at code, 2026-11-16. `cmd/ze` parses the whole config with `config.LoadConfig` before `hub.Run`: `checkPushedConfig` (which removes a pushed config it judges invalid), `cmdStartManaged` on first boot, and `extractManagedClientConfig` (which then runs with no managed client, with a warning). The managed handler's `Validate` calls `ReadConfig` while the daemon runs, after the publish. These join the census in Files to Modify | broken |
| A-11 | The first pass costs no second resolve of the module set | AC-9 | A start with no external plugin pays a second resolve, about 200 ms by the comment on `YANGSchema` | Read at code, 2026-11-16. `config.PluginOnlySchema` builds its own loader and resolves every module, apart from `yang.DefaultLoader`. The first pass runs on every start, since it is what finds whether a plugin is declared, so it must take its sub-schema from the set `DefaultLoader` already resolved. Repaired in phase 4 (2026-11-16): `PluginOnlySchema` now cuts the roots `plugin` and `environment` out of `config.YANGSchema`, the cached schema of the set `DefaultLoader` resolved, and builds no loader. `TestPluginOnlySchemaSharesTheResolvedSet` is the proof; its run is owed. The assumption as first written stays broken | broken |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The boot order of `runYANGConfig` changes: the plugin server exists before the full parse | Existing `test/plugin` and `test/reload` tests change outcome | Only a declared external plugin takes the early Stage 1; with none declared the order is today's (AC-9). The audit phase lists what moves |
| R-2 | Offline validate refuses a config the router accepts, on every machine: with no daemon it starts no plugin (owner ruling, 2026-11-16) | `unknown top-level keyword` for a section under an external plugin's root | Accepted by the ruling. The error says which external plugins the config declares, why Ze holds no schema of them with no daemon, and the daemon form that checks the section; it never passes a root it could not check |
| R-3 | A slow or hanging plugin delays boot until its Stage 1 fails | Boot time grows by the stage timeout | The existing stage timeout and failure policy; Q10 measures |
| R-9 | Closed by the owner ruling of 2026-11-16. As built in phase 6, every offline command ran the plugin's command in declare mode on the machine that ran it, so inspecting a config someone else wrote ran their command | A marker file written by the declared program | No command that parses a config starts a declared program: `TestDeclaredSchemasStartsNoDeclaredProgram`, `TestValidateRefusesSectionOfExternalPluginWithoutStartingIt`, `external-yang-offline-not-available.ci`. `show plugin declarations` and the conformance check still run the program the operator names |
| R-10 | Closed by the same ruling: validate reads no module, so no second answer exists to disagree with the live Stage 1. What remains is by design: boot accepts a section validate calls not available | `external-yang-boot-accepts-what-offline-refuses.ci` | Validate never accepts what boot refuses, and its message says a running Ze can check the section |
| R-4 | Publishing after a reader already took the schema leaves that reader stale | A value accepted by one surface and refused by another | Built with the publish (2026-11-16). `config.YANGSchema` and `YANGValidator` are keyed on the `Resolved`; the `cli/client` YANG state and the headless command tree are keyed on it too; the publish builds the command argument definitions again; every value built once for the process records itself with `yang.KeepsModuleSet`, and `yang.PublishExternalModules` refuses a publish that would change the set after one of them, naming it (`refuseKeptModuleSet`). What stays: a reader that holds a `*yang.Resolved` it took before the publish keeps that set, by contract (a `cli.Completer`, an editor session). The phase that wires boot MUST publish before `pluginserver.NewServer` and before any full parse: a config with an interface parsed before the publish records the reserved interface names, and the publish is then refused |
| R-5 | An inline external plugin (`bgp peer ... attach run`) is not in the `plugin` block, so the first pass does not see it | It declares a module after the set is published | That plugin is failed, with the message to declare it under `plugin external` or on the command line |
| R-6 | Deleting the silent duplicate drop refuses a start that worked | A compiled-in plugin named on the command line fails with a duplicate module | A-3 decides it at audit, before the deletion. Phase 3 deleted the substring match on `duplicate module`, which dropped any duplicate whatever its content. On the internal text road (`loadYANGModules`) a text whose module the compiled-in set holds with byte-identical content is left out before the join (`Resolved.HoldsModuleText`); a held name with different content is refused as a name clash. `CollectPluginYANG` hands over registered texts unchanged, so its remaining callers (`config/loader.go`, `bgp/config/loader.go`) keep working and the risk is closed for them. The external join (`JoinExternalModules`) stays strict for an identical text too. `bgp/config/register.go` and `plugin/cli/test_cmd.go` were converted in phase 3 |
| R-20 | The join judges `ze:validate` and `ze:reference` against their registries, and cannot reach two others from `config/yang` | An external module's `ze:decorate` or `ze:backend` names something the daemon does not hold and is accepted | The decorators live in `internal/component/web` (`DecoratorRegistry`) and the backends in each component's `RegisterBackend` (`iface`, `firewall`, `traffic`); both import `config`. A later phase passes their names to the join from a package that can reach them, or AC-11 is narrowed by the owner |
| R-21 | Existing tests pass a registered module's text to `YANGSchemaWithPlugins` and relied on the drop | `config` tests through `serialize_test.go` and `extended_test.go` (`ze-graceful-restart.yang`) and `internal/plugins/ntp/command_active_config_test.go` (`ze-ntp-conf.yang`) go red with the name-clash refusal (seen: `TestFlagSyntax`, `TestFlagWithValue`, `TestFlagWithBlock`, `TestSerializeCapability`) | Settled without a test edit: each of those texts is byte-identical to the registered module, so the internal text road leaves it out (`TestModuleTextRoadLeavesOutIdenticalRegisteredModule`); a different content under a registered name is still refused (`TestModuleTextRoadRefusesRegisteredModule`) |
| R-7 | The build-time compile spec lands later and rebuilds the join | Two producers of the schema | One join function with fixture tests is the seam; that spec replaces its body only, and its Q1 is updated to say so |
| R-8 | Two external plugins declare modules that clash with each other | The second join fails | The refusal names both plugins; order is the config order. Built in phase 3 for the name, the namespace and a top-level node (`TestJoinRefusesClashBetweenTwoExternalModules`) |
| R-11 | The hub answers a Stage 1 before the plugin's tier, and a check that runs at the tier then refuses | The plugin has its OK and no request left to carry the refusal, so its connection closes with no reason it can read | Keep the answer with the checks: the early step reads and keeps the declaration, and the tier's handshake answers it. `PluginConn.ReadRequest` already replays a kept startup request (`pendingStartup`). The plugin then waits for its Stage 1 answer, which is the owner's "waits at the barrier" |
| R-12 | A plugin forked before the full parse is forked again when its phase starts | Two processes of one plugin; the second fails its connect-back or its registration | `runPluginPhaseWithin` calls the spawner for every plugin of the phase (`Manager.SpawnMore`, `ProcessManager.StartMore`); the phase must take the process the early step started |
| R-13 | A declared plugin connects and never sends its Stage 1 | Boot waits with no bound: the Stage 1 read has no timer, and today the bound comes only from a sibling of the tier waiting at the barrier | The early wait takes its own bound from the stage timeout, and keeps the outcome with its reason (AC-8). Read at code, not proven by a run |
| R-14 | The publish writes the module registry while other goroutines run | A data race on `modulesGeneration`, which `RegisterModule` documents as init-only and `DefaultLoader` reads under its own lock alone | The external layer and the generation are written under the lock `DefaultLoader` reads under, once, before the full parse. Built (2026-11-16): `yang.PublishExternalModules` writes the external layer and swaps the joined `Resolved` into `DefaultLoader`'s cached entry under `defaultSchema.mu` (`swapModuleSet`). It does not write `modulesGeneration` at all: the entry of the current generation is replaced in place, so the unlocked variable keeps its init-only contract |
| R-15 | The first pass applies the `environment` block before the validators judged the config | A setting of a config that is then refused has already reached the process, or the forked plugin misses a setting applied only after the full parse | `LoadConfig` applies the block only once refusal is impossible (`applyParsedEnvironment`). Decide at implementation which settings a forked plugin needs at fork, and whether `ApplyEnvConfig` reaches the environment `startExternal` hands the child: not settled by reading |
| R-16 | The start path refuses or drops a config before the daemon's first pass (A-10) | A pushed config holding an external root is removed as invalid; a managed edge runs with no managed client | The entries in `cmd/ze` take the same first pass, and those that need only the `plugin` block take the first pass alone. Read again in phase 4 (2026-11-16), nothing changed there yet. `extractManagedClientConfig` reads `plugin/hub` alone: it takes `config.FirstPass` and its `Hub.Clients`, with no full parse and no publish. `checkPushedConfig` and the first-boot branch of `cmdStartManaged` judge a whole config before `hub.Run`, and the first removes a pushed config it judges invalid: each needs the modules of the declared plugins before its `LoadConfig`. They run in the process that then boots, where the set is published once, so the phase that wires boot decides between moving both verdicts inside the daemon's boot after its publish, or publishing here from declare mode and having boot compare each live declaration with the published set (R-10) |
| R-17 | An external plugin's filter insertion names a condition path under its own module | Its Stage 1 is refused by `filter-insertion-paths` when the check runs before the publish | The check runs at the tier, after the publish (A-7) |
| R-18 | An external module augments a root the first pass parses strictly (`plugin` or `environment`) | The first pass refuses the augmented child as an unknown field, before the plugin that declares it has run, so the config can never load | Not settled by phase 4. Either the join refuses an external augment whose target sits under a first-pass root, naming the rule, or the first pass skips unknown children of those roots too. The first keeps a mistyped leaf of a plugin declaration refused, which is why phase 4 skips roots only |
| R-19 | With an external plugin declared, the config schema is built twice: once for the first pass over the compiled-in set, once after the publish | Start time grows by one schema build on top of the second resolve Q10 already counts | Inherent in the load order: the first pass needs the schema before the plugin's module exists. With no external plugin the first build is the one the full parse uses (`yangSchemaCache`), so AC-9 holds. Q10 measures both |
| R-22 | A value built once for the process is built before the publish, in a start that declares an external plugin | The publish is refused at boot with "it already built <value> without them", and the daemon does not start with that plugin | Intended: the refusal replaces a value that would answer without the plugin's module. The phase that wires boot orders the publish before `pluginserver.NewServer`, before any gNMI server, and before any full parse (a parsed interface name records the reserved names; A-10's three `cmd/ze` parses run before `hub.Run` today). `external-yang-own-root.ci` is the run that shows a wrong order |
| R-23 | `config.JoinExternalModules` joins to the set `DefaultLoader` answers, which after a publish is the published set | A caller that joins the process's own declarations after the publish gets a name clash that says the module is "compiled into ze" | Callers that hold the process's declarations publish and do not join. The wording of `refuseNameClash` for a held external module is not corrected: `yang.Join` cannot read the published layer while the publish holds its lock |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A config is refused at load, or a plugin is refused at start. No session or route is touched: the path runs before plugins start |
| How is it reverted? | A single revert of the phase commits; no stored format changes |
| Who else touches this path? | `spec-yang-single-source-of-truth`, `spec-validation-walks-every-schema-section`, `spec-yang-defaults-applied-once` (all `plan/release/fixes/`), and `spec-config-yang-compiled-at-build-time` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze start` with an external plugin declared in the `plugin` block, and a block under its root | → | first pass, early Stage 1, join, publish, parse, Stage 2 delivery | `test/plugin/external-yang-own-root.ci` |
| `ze start` with the same plugin declared on the command line, no `plugin` block | → | the command-line half of the first pass | `test/plugin/external-yang-command-line.ci` |
| `ze config validate` on the same config, no daemon | → | first pass, parse against the compiled-in schema, the not-available refusal, no program started | `test/plugin/external-yang-offline-not-available.ci`, `test/plugin/external-yang-boot-accepts-what-offline-refuses.ci` |
| `ze start`, module that does not compile | → | join refusal | `test/plugin/external-yang-invalid-refused.ci` |
| `ze start`, module named like a compiled-in module | → | join refusal | `test/plugin/external-yang-clash-refused.ci` |
| A running plugin is restarted and declares a different module | → | comparison with the published set | `test/plugin/external-yang-declaration-changed.ci` |
| SIGHUP with a config whose plugin declarations differ | → | reload refusal | `test/reload/external-yang-reload-refused.ci` |
| Editor session in the daemon, completion under the external root | → | published module set | `test/draft/ui/external-yang-session-editor.ci`, to promote as `test/ui/external-yang-session-editor.ci` (the `.et` runner has no daemon, so it never holds the module) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | An external plugin declares module `acme-monitor-conf` with root `acme-monitor`; the config holds an `acme-monitor { ... }` block | `ze start` accepts the config; the plugin receives the block in its Stage 2 section under root `acme-monitor`, with the values the operator wrote | A config with no such block still starts and the plugin receives an empty section: the root is optional |
| AC-2 | The same plugin and config, `ze config validate` with no daemon (restated by the owner ruling of 2026-11-16) | Validate starts no program, refuses the section and exits 1. The message holds the unknown keyword, the external plugins the config declares, that Ze reads their schema when `ze start` runs them and that this command starts no plugin, and the daemon form `show config validation <file>`, with `ze cli` alone on the fix line. `ze start` accepts the same config (AC-1). In a process that holds the plugin's module, as a running daemon does, validate judges the section with it: a value outside the module's range is refused naming the leaf | A config that declares the plugin and holds no section under its root is valid, and a compiled-in plugin is judged as before: only what needs the external schema is not available. The message never says a plugin owns the refused name: Ze holds no schema that could tell |
| AC-3 | After AC-1, a commit changes a leaf under `acme-monitor` | The plugin receives verify with the candidate, then apply with the changed path and its old and new value; a verify refusal aborts the commit with the plugin's text | A commit touching no path under the root does not call the plugin: only participants of a change take part |
| AC-4 | The declared module does not compile | Startup refuses, naming the plugin, the module and the loader's error with its location | None: a module with no schema cannot be parsed against |
| AC-5 | The module's name or namespace equals one already in the set, or it redefines an existing top-level node | Startup refuses, naming the plugin and both holders; for a redefined node the message gives the `augment` form | A module that reaches the same container by `augment` is accepted: that is the sanctioned way to share a container |
| AC-6 | An external module augments compiled-in container `service` with `chrony`, and the plugin wants root `service/chrony` | The config block `service { chrony { ... } }` is accepted and the plugin receives that subtree; the internal owner of `service` keeps working | The internal owner still receives its own children of `service`: delivery is by declared root |
| AC-7 | A running external plugin is restarted (operator restart or respawn) and declares a module whose text differs from the published one | That plugin is failed at Stage 1 with a message naming the restart of `ze`; doctor reports it; the config under its root stays loaded | A restart that declares the same module continues as today. The daemon and every other plugin keep running, as the plugin's failure policy already rules |
| AC-8 | A config holds a block under an external plugin's root and that plugin does not reach its Stage 1 (it cannot be run, it stalls, or it exits) | The load refuses with the unknown keyword, the plugin's name, why it did not declare and the command that fixes it | A config with no block under that root loads: a plugin that did not declare refuses nothing by itself, and doctor reports it |
| AC-9 | Neither the config nor the command line declares an external plugin | The boot order and the module set are today's: no early Stage 1, one resolve | None: the common case pays nothing |
| AC-16 | The plugin of AC-1 is declared on the command line and the config holds no `plugin` block | Same outcome as AC-1 | A command-line plugin with no module and no config section still runs, as today |
| AC-10 | A value out of the external module's declared range, a missing mandatory leaf, or a pattern mismatch | Refused at load with the same message a compiled-in leaf gives | Values the module allows are accepted as strings in the delivered section, as for every plugin |
| AC-11 | An external module names a `ze:validate` function, or another daemon-side registration, that the daemon does not hold | Startup refuses, naming the plugin, the extension and the name | Schema-only extensions (`ze:help`, `ze:sensitive`, `ze:hidden` and the rest) work: they need no registration |
| AC-12 | A reload whose config adds, removes or changes the declaration of an external plugin: an `external` block with a `run` line (restated in phase 7; as first written, "a plugin declaration") | The reload is refused whole, naming the plugin, what changed and the restart of `ze` that loads it; the running config is unchanged; doctor reports it as `doctor-config-reload-refused` | A reload that leaves those declarations equal proceeds as today, including changes under a plugin's own root. An `internal` block, or an `external` block with `use`, is not compared: it names a plugin compiled into `ze`, which brings no module, and a reload that adds one is applied today and tested (`test/reload/bgp-attach-no-route-refresh-restarts.ci`, `bgp-attach-sent-direction-restarts.ci` add `internal adj-rib-in` at reload). The command-line plugins are the same at boot and at reload |
| AC-13 | In the running daemon: the editor, `show config`, `show yang`, save and the JSON an API returns | Each shows the external root: completion offers its leaves, a saved config holds the block, and it parses again | Offline commands that load no config (`ze schema` with no config) show compiled-in modules only: they have no plugin to ask |
| AC-14 | Doctor on a daemon running AC-1 | No `unclaimed root` finding for `acme-monitor`; a plugin that did not declare, a refused module and a changed declaration each have a doctor code with its cause and fix | A root no running plugin wants is still reported: the check keeps its meaning. Found in phase 8: a refused module ends the start, so no daemon reports its code (`doctor-plugin-schema-refused`); the same holds for `doctor-plugin-schema-undeclared` when the config holds a section the missing schema would define. Both are read on the refusal and in the log, and then in `ze doctor` run with no daemon, from the last-stop record boot writes, until the next clean start deletes it. An undeclared plugin beside a config with no such section, and a changed declaration, are reported by doctor on the running daemon |
| AC-15 | A plugin that is a shell script, which leaves a marker when started and answers declare mode (restated by the owner ruling of 2026-11-16; as first written, offline validate accepted a config under its root) | Offline validate never starts it: the marker is absent after validate ran on a config that declares it, with and without a section under its root | `ze show plugin conformance run` and `show plugin declarations` start it: running the program the operator names is their purpose |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|---------------------|-----------------------|
| 1 | Writes a plugin in any language that declares a module, declares it in the `plugin` block or on the command line, adds its config block, starts Ze | first pass → run the plugin → Stage 1 → join → publish → parse → Stage 2 | `external-yang-own-root.ci`, `external-yang-command-line.ci` |
| 2 | Checks the config on the build machine before deploying | validate entry → first pass → parse against the compiled-in schema. Everything but a section of an external plugin is checked; that section is reported as not available with no daemon, with the daemon form that checks it | `external-yang-offline-not-available.ci`, `external-yang-boot-accepts-what-offline-refuses.ci` |
| 3 | Edits the plugin's config in the SSH editor and commits | editor schema → commit → verify → apply | `ui.external-yang-session-editor` (to be promoted as `test/ui/external-yang-session-editor.ci`), `external-yang-commit.ci` |
| 4 | Ships a broken module | join refusal with plugin, module, location | `external-yang-invalid-refused.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFirstPassSkipsUnknownRoots` | `internal/component/config/first_pass_test.go` | A full config with unknown roots yields the `plugin` block | |
| `TestFirstPassQuotedBraces` | `internal/component/config/first_pass_test.go` | A quoted brace inside a skipped block does not end it | |
| `TestFirstPassSetForm` | `internal/component/config/first_pass_test.go` | The `set` form yields the same plugin list | |
| `TestFirstPassSetMetaForm` | `internal/component/config/first_pass_test.go` | The set form with authorship metadata skips an unknown root too | |
| `TestFirstPassSkipsNestedBlock`, `TestFirstPassSkipsTopLevelLeaf` | `internal/component/config/first_pass_test.go` | A skipped root ends at the brace that closes it; a skipped statement with no block ends at its terminator | |
| `TestFirstPassRefusesUnclosedSkippedBlock`, `TestFirstPassRefusesUnfinishedSkippedStatement` | `internal/component/config/first_pass_test.go` | A skipped block or statement the file cuts is refused, naming its line | |
| `TestFirstPassKeepsKnownRootsStrict`, `TestStrictParseStillRefusesUnknownRoot`, `TestParseAtNeverSkips` | `internal/component/config/first_pass_test.go` | The skip covers unknown roots of a whole file, in an explicit mode, and nothing else | |
| `TestFirstPassLeavesOutCompiledInPlugins`, `TestFirstPassAnswersNothingToRun`, `TestFirstPassMergesCommandLineWithConfigPlugins` | `internal/component/config/first_pass_test.go` | Only a plugin that runs as another program is declared external, each once | |
| `TestFirstPassAnswersHubAndEnvironment`, `TestFirstPassReadsTheEnvironmentOfTheFullParse` | `internal/component/config/first_pass_test.go` | The hub listener, the `environment` block and the work directory are the full load's | |
| `TestPluginOnlySchemaSharesTheResolvedSet` | `internal/component/config/first_pass_test.go` | A-11: the first-pass schema is cut from the resolved set, with no resolve of its own | |
| `TestJoinAcceptsOwnRoot` | `internal/component/config/yang/join_test.go` | A `-conf` module adds its top-level node | |
| `TestJoinRefusesInvalidModule` | `internal/component/config/yang/join_test.go` | AC-4 text: plugin, module, location | |
| `TestJoinRefusesModuleNameClash` | `internal/component/config/yang/join_test.go` | AC-5, name, including a different revision | |
| `TestJoinRefusesNamespaceClash` | `internal/component/config/yang/join_test.go` | AC-5, namespace | |
| `TestJoinRefusesTopLevelRedefinition` | `internal/component/config/yang/join_test.go` | AC-5, node, with the `augment` hint | |
| `TestJoinAcceptsAugmentOfSharedContainer` | `internal/component/config/yang/join_test.go` | AC-6 at the schema | |
| `TestJoinRefusesUnregisteredValidator` | `internal/component/config/yang/join_test.go` | AC-11 | |
| `TestJoinRefusesModuleThatContributesNothing` | `internal/component/config/yang/join_test.go` | The `-conf` rule is stated, not silent | |
| `TestJoinRefusesDeclaredFieldMismatch` | `internal/component/config/yang/join_test.go` | `module` or `namespace` field against the text | |
| `TestFirstPassMergesCommandLinePlugins` | `internal/component/config/first_pass_test.go` | A command-line plugin joins the declared list with no `plugin` block | |
| `TestEarlyStage1HoldsAtBarrier` | `internal/component/plugin/server/early_stage1_test.go` | A declared external plugin's Stage 1 is read and kept unanswered: the plugin waits for its Stage 1 answer with no stage timer on it, and its startup tier reads the kept request and answers it (R-11) | |
| `TestEarlyStage1KeepsFailureOutcome` | `internal/component/plugin/server/early_stage1_test.go` | A plugin that does not declare is kept with its reason: not started, silent past its stage timeout (R-13), connection ended, first request not a declaration | |
| `TestEarlyStage1SecondPluginKeepsItsDeclaration`, `TestEarlyStage1NoPluginStartsNothing`, `TestEarlyStage1SpawnerRefusalIsEveryPluginsOutcome` | `internal/component/plugin/server/early_stage1_test.go` | One clock for the step does not cost a later plugin its declaration; no declared plugin starts nothing; a start that forks nothing is every plugin's outcome | |
| `TestSpawnMoreTakesADeclaredPluginOnce`, `TestSpawnMoreOfOnlyHeldPluginsForksNothing`, `TestSpawnMoreRefusesAPluginThatDidNotDeclare`, `TestStartDeclared*` | `internal/component/plugin/manager/declared_test.go` | R-12: a plugin started before the full parse is never forked again, its record is taken once, and a plugin that did not declare refuses its phase with the kept reason | |
| `TestDeclaredPluginThatCannotRunIsKeptAsAnOutcome`, `TestPrepareFailureRefusesTheBoot`, `TestDeclaredModulesTakesOnlyDeclaredModuleTexts`, `TestPluginServerConfigsCarriesTheWorkDirectory` | `cmd/ze/hub/declared_plugins_test.go` | The boot step between the first pass and the full parse: outcomes kept, the publish fed by declared module texts alone, one config builder for the held plugin and its phase | |
| `TestQueryKeepsSchema`, `TestQuerySilenceSaysWhatTheRunShowed` | `internal/component/plugin/declarations_test.go` | The declare-mode answer carries `schema`, for the conformance check; a run that gave none keeps the reason of its row | |
| `TestDeclaredSchemasStartsNoDeclaredProgram`, `TestParseRefusalSaysTheSectionIsNotAvailableOffline`, `TestParseRefusalNamesEveryDeclaredPluginAndNoFile`, `TestDeclaredSchemasAnswersFromThePublishedSet`, `TestDeclaredSchemasSecondCallAnswersTheSame` | `internal/component/config/declared_schemas_test.go` | Owner ruling of 2026-11-16 at the shared step: a declared program that leaves a marker is never started (with the control that the marker shows a start); the one operator error and its fix; a process that holds the plugin's module judges the section with it (range 1..10) | |
| `TestValidateRefusesSectionOfExternalPluginWithoutStartingIt`, `TestValidateAcceptsConfigWithNoSectionOfExternalPlugin`, `TestValidateJudgesSectionWithThePublishedModule` | `internal/component/config/cli/cmd_validate_declared_test.go` | The same at the validate entry | |
| `TestDeclaredSchemasNoExternalPluginRunsNothing`, `TestDeclaredSchemasRunsNothingOnceTheSetIsFixed`, `TestDeclaredSchemasTakesPluginsOfEverySource`, `TestDeclaredSchemasSkipsSourceTheFirstPassRefuses`, `TestDeclaredSchemasExplainsOnlyUnknownNames`, `TestParseRefusalNamesOneFix` | `internal/component/config/declared_schemas_test.go` | The shared step of the commands that parse a config with no daemon: AC-9, no program run and the restart named in a process whose module set is fixed, both sides of a diff, a plugin named only beside an unknown name. Removed with the execution they tested (owner ruling of 2026-11-16): `TestDeclaredSchemasJoinsModuleOfDeclareModeRun`, `TestDeclaredSchemasNamesPluginThatCannotRun`, `TestDeclaredSchemasAsksEachPluginOnce`, `TestDeclaredSchemasRefusesModuleTheJoinRefuses`, and at the validate entry `TestValidateJudgesSectionOfDeclaredExternalPlugin`, `TestValidateNamesPluginThatCannotRun`, `TestValidateAcceptsConfigWithNoSectionOfSilentPlugin` | |
| `TestNoExternalPluginKeepsBootOrder` | `cmd/ze/hub/declared_plugins_test.go` | AC-9: with no declared external plugin the boot step prepares no hub, starts no plugin and publishes nothing | |
| `TestPublishedSetReachesSchemaCaches` | `internal/component/config/publish_schema_test.go` | After a publish `YANGSchema` holds the external root and `YANGValidator` its range; before it, and after the test's restore, neither does | |
| `TestPublishWithNoExternalModuleKeepsTheSharedSet`, `TestPublishRefusedModuleKeepsTheCompiledInSchema`, `TestPublishTwiceThroughTheConfigEntry` | `internal/component/config/publish_schema_test.go` | AC-9 at the publish (same `Resolved`, schema and validator); a refused module publishes nothing; the once rule through the config entry | |
| `TestPublishMakesDefaultLoaderAnswerTheJoinedSet`, `TestPublishSameModulesTwiceChangesNothing`, `TestPublishOtherModulesIsRefusedNamingTheRestart`, `TestPublishRefusedModulePublishesNothing`, `TestPublishNoModuleResolvesNothing`, `TestPublishedModuleComparesADeclarationWithThePublishedSet`, `TestModuleRegisteredAfterAPublishIsJoinedAgain` | `internal/component/config/yang/publish_test.go` | The external layer of the module set: the swap, the once rule, the refusals, and what a later declaration is compared with | |
| `TestPublishRefusesAfterAKeeperReadTheSet`, `TestPublishRebuildsCommandArgDefs` | `internal/component/config/yang/publish_test.go` | A once-built value recorded before a publish refuses it; the argument definitions are built again | |
| `TestYANGStateFollowsThePublishedModuleSet`, `TestHeadlessCommandTreeFollowsThePublishedModuleSet`, `TestCompleterValueSchemaIsOfTheSetTheCompleterHolds` | `internal/component/cli/client/yang_state_publish_test.go`, `internal/component/cli/testing/headless_publish_test.go`, `internal/component/cli/completer_publish_test.go` | The values keyed on the `Resolved` follow a publish; a completer judges with the set it took | |
| `TestServerBuiltBeforeAPublishRefusesThePublish` (twice), `TestBackendGateUsedBeforeAPublishRefusesThePublish` (three times), `TestIfaceNameValidatedBeforeAPublishRefusesThePublish` | `module_set_keeper_test.go` in `plugin/server`, `gnmi`, `iface`, `firewall`, `traffic` | Each value built once for the process records itself, so a publish after it is refused, naming it | |
| `TestStage1RefusesChangedDeclaration`, `TestStage1AcceptsUnchangedDeclaration`, `TestStage1UnchangedDeclarationClearsTheWarning`, `TestStage1SchemaCheckNeedsAPublishedSet`, `TestOnRegistrationRefusesChangedSchema` | `internal/component/plugin/server/stage1_schema_test.go` | AC-7: each way a later declaration differs, the permitted survivors, the warning doctor reads, and the check reached through `onRegistration` | written, not run |
| `TestDeclarationChangesNamesEachDifference`, `TestRefuseChangedDeclarationsRefusesTheReload`, `TestRefuseChangedDeclarationsAcceptsTheSamePlugins` | `internal/component/plugin/server/reload_declarations_test.go` | AC-12 at the comparison and the raised event | written, not run |
| `TestReloadLoaderRefusesChangedPluginDeclarations`, `TestReloadLoaderLoadsWhenTheDeclarationsAreUnchanged` | `cmd/ze/hub/reload_declarations_test.go` | AC-12 through the loader every reload uses | written, not run |
| `TestDeclaredSchemasDoesNotNameAStartedPluginThatDeclaredNoModule`, `TestDeclaredSchemasNamesThePluginsThatCouldExplain`, `TestDeclaredSchemasWithoutARecordNamesEveryPlugin` | `internal/component/config/declared_schemas_test.go` | Boot's record of who declared, and the narrowed naming in a daemon | written, not run |
| `TestStartedPluginsRecordsWhatEachDeclared` | `cmd/ze/hub/declared_plugins_test.go` | The record boot builds from the early declarations | written, not run |
| `TestDoctorClaimsCountExternalRoots` | `internal/component/doctor/checks_config_claims_external_test.go` | AC-14: the root a running external plugin declared is not reported, a root nobody wants still is, and both are once the plugin is gone | written, not run |
| `TestReportUndeclaredSchemaRaisesTheWarning`, `TestReportUndeclaredSchemaOfASchemalessPlugin`, `TestReportRefusedSchemaRaisesTheError`, `TestExternalConfigRootsFollowTheRunningPlugin` | `internal/component/plugin/server/external_schema_report_test.go` | AC-14: the two codes on the report bus with their subject, message and fix; the roots kept while the plugin runs | written, not run |
| `TestSchemalessPluginsAreNamedBesideAnUnknownSection` | `cmd/ze/hub/schemaless_plugins_test.go` | The "declared none" case of `doctor-plugin-schema-undeclared` at boot's refusal | written, not run |
| `TestSchemaRegistryStartsNoExternalProgram`, `TestExternalNotShownNamesThePluginAndTheFix`, `TestSchemaRegistryHoldsPublishedExternalModule` | `internal/component/config/schema/cli/external_plugin_test.go` | AC-13 at the schema CLI: no program started (with the marker control), the not-available text and fix, the published module in the daemon's registry | written, not run |
| `TestEditorCompletesExternalRootFromThePublishedSet` | `internal/component/cli/editor_external_root_test.go` | AC-13, the editor: a section under an external root parses and its leaves are completed from the published set | written, not run |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| A ranged leaf of the fixture module | 1-10 | 10 | 0 | 11 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `external-yang-own-root` | `test/plugin/external-yang-own-root.ci` | AC-1 | |
| `external-yang-command-line` | `test/plugin/external-yang-command-line.ci` | AC-16 | |
| `external-yang-boot-accepts-what-offline-refuses` (was `external-yang-validate-agrees-with-boot`, restated by the owner ruling of 2026-11-16) | `test/plugin/external-yang-boot-accepts-what-offline-refuses.ci` | AC-2 beside AC-1: one config, refused as not available by validate with no daemon, accepted and delivered by boot | |
| `external-yang-commit` | `test/plugin/external-yang-commit.ci` | AC-3 | |
| `external-yang-invalid-refused` | `test/plugin/external-yang-invalid-refused.ci` | AC-4 | |
| `external-yang-clash-refused` | `test/plugin/external-yang-clash-refused.ci` | AC-5 | |
| `external-yang-augment-shared-root` | `test/plugin/external-yang-augment-shared-root.ci` | AC-6 | |
| `external-yang-declaration-changed` | `test/plugin/external-yang-declaration-changed.ci` | AC-7 | |
| `external-yang-query-failed` | `test/plugin/external-yang-query-failed.ci` | AC-8 | |
| `external-yang-offline-not-available` (replaces `external-yang-declare-only`, whose expectation the owner ruling of 2026-11-16 made wrong) | `test/plugin/external-yang-offline-not-available.ci` | AC-2, AC-15: the refusal and its message, the declared program not started, a config with no external section valid, a compiled-in plugin valid | |
| `external-yang-reload-refused` | `test/reload/external-yang-reload-refused.ci` | AC-12 | |
| `external-yang-session-editor` (replaces `external-yang-completion.et`, which the headless `.et` runner cannot run) | `test/draft/ui/external-yang-session-editor.ci`, to promote | AC-13, the editor: the root and its leaves offered, the leaf's description on `?`, a value outside the range refused with the range | written as a draft; the first run was red on an expectation the editor does not meet (a description in the completion box), corrected and not run again |
| `external-yang-daemon-schema` | `test/draft/ui/external-yang-daemon-schema.ci`, to promote | AC-13: `show schema list` and `show yang tree config` dispatched by the daemon hold the external module, over SSH exec and typed in the session | written as a draft; the first run was red on `show schema show`, a command the daemon does not have, corrected and not run again |
| `external-yang-augment-owner-keeps-section` | `test/draft/plugin/external-yang-augment-owner-keeps-section.ci`, to promote | AC-6: the owner of `service` receives its child beside the augment, read from `show dns resolver` | written as a draft, the resolver assertion not run |
| `external-yang-doctor` | `test/plugin/external-yang-doctor.ci` | AC-14 | |
| `external-yang-show-config` | `test/draft/plugin/external-yang-show-config.ci`, to promote | AC-13: `show config running` and its JSON hold the external section | written as a draft, not run |
| `external-yang-changed-keeps-config` | `test/draft/plugin/external-yang-changed-keeps-config.ci`, to promote | AC-7: the section stays in the running config after the refused restart | written as a draft, not run |

### Interop Tests (Scope: protocol)
N-A: no protocol peer. The outside party is a plugin author; AC-15 proves that a program not built on the Go SDK is never started with no daemon.

## Files to Modify
- `internal/component/config/parser.go` - a root-skip mode for the first pass
- `internal/component/config/yang_schema.go` - `PluginOnlySchema` gets its caller; `loadYANGModules` and `YANGSchemaWithPlugins` give way to the join; strict validator check over external modules
- `internal/component/config/loader.go` - the first pass (the `plugin` block and the command line), and the full parse taking the published set
- `cmd/ze/hub/main.go` - `runYANGConfig`: the plugin server accepts the declared external plugins' Stage 1 before the full parse
- `internal/component/plugin/server/startup.go`, `startup_driver.go` - the early Stage 1 of a declared external plugin, held at the barrier; `registrationFromRPC` feeds the join; a restart is compared with the published set
- `internal/component/config/yang/register.go`, `loader.go` - the external layer of the module set and its generation
- The entries that parse a config with no daemon, and the start path, listed in the census below - first pass; boot runs the plugins and joins, a command with no daemon runs none (owner ruling, 2026-11-16)
- `internal/component/plugin/declarations.go` - the declare-mode answer keeps `Schema`. Already there: `QueryDeclaration` answers the whole declaration in `DeclarationQuery.Declaration`; only `queriedDeclarationRow` drops the schema
- `internal/component/plugin/inprocess.go` - `CollectPluginYANG` removed: A-3 holds. Its four callers go with it: `config/loader.go` `loadConfig`, `bgp/config/loader.go` and `bgp/config/register.go` (each with `YANGValidatorWithPlugins`), and `plugin/cli/test_cmd.go` `cmdPluginTest` (with `YANGSchemaWithPlugins`). State after phase 3: `bgp/config/register.go` takes `YANGValidator()` and `plugin/cli/test_cmd.go` takes `YANGSchema()` (it still calls `CollectPluginYANG` to print the list). Still owed: `config/loader.go` (two sites pass `CollectPluginYANG(cliPlugins)` to `parseTreeWithYANG`), `bgp/config/loader.go` `loadReactorFile`, the test caller `internal/plugins/ospf/rfc5709_key_persistence_test.go`, and then the function itself
- `internal/component/config/yang_schema.go` - `YANGSchemaWithPlugins` and `YANGValidatorWithPlugins` stay, because tests in seven files call them with module texts; phase 3 put their texts through the join (`loadYANGModules`, `JoinExternalModules`). Removing them needs those tests moved to the join's entry first (R-21 names the three that pass a registered module)
- `internal/component/plugin/manager/manager.go` - an entry that starts the declared external plugins before `StartAll` (audit, Data Flow)
- `internal/component/plugin/ipc/rpc.go` - the kept Stage 1 request, replayed to the tier's handshake (`pendingStartup` exists)

Census of the entries that parse a config (audit, 2026-11-16). Every one takes its schema from `config.YANGSchema` or `yang.DefaultLoader`, so it sees the external modules once the process published them before the parse; none needs a schema argument of its own.

| File | Symbol | What it parses | Must see external modules |
|------|--------|----------------|---------------------------|
| `internal/component/config/cli/cmd_validate.go` | `runValidation` | The file given to `ze config validate` | Yes |
| `internal/component/config/cli/cmd_fmt.go` | `formatConfigBytes` | The file to format | Yes |
| `internal/component/config/cli/cmd_dump.go` | `resolveDump` | The file to dump | Yes |
| `internal/component/config/cli/cmd_diff.go` | `resolveDiff`, `parseAndResolve` | Both sides of a diff | Yes |
| `internal/component/config/cli/cmd_migrate.go` | `configMigrateWithWarnings` | The file to convert, in either form | Yes |
| `internal/component/config/cli/cmd_completion.go` | `completerFor` | The file a completion runs over | Yes |
| `internal/component/config/cli/cmd_graph.go` | `graphAnswer` | The file to graph | Yes |
| `internal/component/config/register_plugin_declarations.go` | `readConfiguredPlugins` | The file of `show plugin declarations config`, through `ReadConfig` | No: it needs the plugin list, so it takes the first pass alone. Today its full parse refuses a config that holds an external root |
| `internal/component/config/schema/cli/main.go` | `getExternalPluginYANG` | No config: it ran a plugin binary with `--yang` | Deleted in phase 8, with no replacement run: with no daemon the schema CLI starts no external program (owner ruling, 2026-11-16) and says the plugin's schema is not available |
| `internal/component/doctor/doctor.go` | `runChecks` | The config `ze doctor` judges, through `ParseTreeForValidation` and `ReadConfig` | Yes |
| `internal/component/support/support.go` | `collectConfig` | The config of a support report | Yes |
| `internal/component/config/system/crashdump.go` | `ParseCrashDumpIntent`, called by `internal/plugins/crashes/readiness.go` | The whole config, for the `system` settings | Yes, or its full parse refuses the file |
| `internal/component/tacacs/cli/main.go` | `probeConfig` | The whole config, for the TACACS+ servers | Yes, for the same reason |
| `internal/component/plugin/cli/test_cmd.go` | `cmdPluginTest` | A config for `ze plugin test`, over the old text road | Yes |
| `internal/component/cli/editor.go`, `validator.go` | The editor session and its commit check (symbols not traced by the audit) | The file under edit, with no daemon or inside it | Yes |
| `internal/component/config/plugin_verify.go` | `ParseTreeForValidation`, `VerifyPluginConfigContent`, `VerifyPluginConfigContentTransition` | The shared read behind validate, doctor, the editor commit and the web service | Yes, through the published set |
| `cmd/ze/pushed_config.go` | `checkPushedConfig` | A pushed config, before `hub.Run` | Yes (A-10) |
| `cmd/ze/ze_core_start.go` | `cmdStartManaged`, `extractManagedClientConfig` | The managed config, before `hub.Run` | The first-boot check: yes. The client extraction reads `plugin/hub`: first pass alone |
| `internal/component/bgp/config/loader.go`, `register.go` | The `LoadReactor` entries and the validator built with `CollectPluginYANG` (callers not traced by the audit) | A config, through `LoadConfig` | Yes; they are callers of the road this spec deletes |

The column "Must see external modules" was written before the owner ruling of 2026-11-16. It now holds for a command the daemon serves, which reads the set boot published. With no daemon, none of these entries sees an external module: each starts no plugin and reports a section of one as not available.

State after phase 6 and its rework under that ruling (2026-11-16, nothing run). The shared step is `config.SchemaWithDeclaredPlugins` (and `config.PublishDeclaredSchemas` before a `ReadConfig` or `ParseTreeForValidation`), in `internal/component/config/declared_schemas.go`. It is called by `runValidation`, `formatConfigBytes`, `resolveDump`, `resolveDiff` (both sides in one call), `configMigrateWithWarnings`, `completerFor`, `graphAnswer`, `readConfiguredPlugins`, `runChecks` of doctor, `collectConfig` of the support report, `LoadCrashDumpIntent`, `probeConfig`, `cmdPluginTest` and the editor's `newEditor`. Differences from the table above:

- `readConfiguredPlugins` takes the step and then the full `ReadConfig`, not the first pass alone: the first pass leaves out the compiled-in plugins, the inline ones and their dependencies, which that command lists.
- The step is in `LoadCrashDumpIntent`, not in `ParseCrashDumpIntent`: the daemon's boot effect calls the second.
- `ParseTreeForValidation` and the `VerifyPluginConfigContent` pair need no call: they read the published set, and their callers above publish first.
- Not covered here: `getExternalPluginYANG` of the schema CLI, which ran a binary with `--yang` until phase 8 deleted it ("the schema CLI"); the `LoadReactor` entries of `bgp/config/loader.go`, whose callers are the daemon and the in-process chaos run, neither of which is a command with no daemon. `checkPushedConfig`, `cmdStartManaged` and `extractManagedClientConfig` were settled in phase 5.
- Inside a running daemon the step runs no program (`yang.ModuleSetFixed`): a served command is answered from the set the daemon joined at start. A config that declares a plugin the daemon did not start with is refused there, naming the restart. The local command refuses a section of that plugin too, with another fix: it names the running Ze, where the daemon names the restart.
- With no daemon the step starts no program (owner ruling, 2026-11-16). It keeps the names of the declared external plugins and fixes the module set to the compiled-in one. Phase 6 as first built ran each declared program in declare mode; that run, its budget, its per-plugin outcomes and `DeclarationQuery.Silence` are deleted.
- The declared plugins are named only beside the refusal of a name the schema does not define, in one operator error (`DeclaredSchemas.ParseRefusal`, `Explain`, `Unchecked`). Beside an accepted config nothing is reported.
- Corrected by phase 10: the fix of that error is no longer `ze cli` with `show config validation <file>` typed there, which the daemon refuses for a file outside its config directories (`command.CallerPath`). `ze config validate` asks the running Ze itself, and every other command names `ze config validate <file>` as its fix. What phase 6 built, kept for the record: the fix of that error was `ze cli`, and its text named `show config validation <file>` to type there. `ze cli -c "show config validation <file>"` is not the fix: `ze cli -c` answers a local-data command in its own process (`command.ServeLocal`), which gives the same refusal. No local `ze` command asks a running daemon to parse a config file today; `ze config migrate`, `ze config completion`, `ze plugin test`, the TACACS+ probe, the support report and the crash-capture read have no daemon form of their own, so their refusal names the validation form as well.
- R-9 is closed by the ruling. `show plugin declarations` still runs each plugin of the config it is given in declare mode, with no daemon: that is its purpose, and the ruling's reading leaves it and the conformance check as they are.
- R-23 was not met: the step publishes an empty external layer and never joins.

Readers inside the running daemon need no edit, because they run after the publish: `cmd/ze/hub/main_reload.go` (the reload's `LoadConfig`), `main_system.go` `readConfigWithStorage`, `service_web.go`, `config/stamp.go` `RecoverConfig`, `config/archive` `ReadConfigFromPath` and `handleArchiveTrigger`, `config/cli/running_rpc.go` `handleShowConfigRunning`, and the reload function of `bgp/config/loader_create.go`.
- `internal/component/plugin/server/reload_declarations.go` (new) and `cmd/ze/hub/main_reload.go` `diskConfigLoaders` - the reload refusal of AC-12 (phase 7; `reload.go` itself is not edited)
- `internal/component/plugin/server/stage1_schema.go` (new) and the `schema-unchanged` entry of `stage1_checks.go` - the restarted plugin of AC-7
- `internal/component/config/schema/cli/main.go` - `getExternalPluginYANG` deleted: with no daemon the schema CLI starts no plugin and shows compiled-in modules only (owner ruling, 2026-11-16)
- `internal/component/gnmi/server.go`, `internal/component/config/yang/command_argdefs.go`, the completer's value schema - keyed on the generation. Audit (A-5): each is first built after the publish, and the set is published once, so none is stale as the code stands; what is owed is the test that proves it (`TestPublishedSetReachesSchemaCaches`) and `pluginserver.NewServer` staying after the publish, since it seeds the argument definitions
- Already in the tree (commit cb4d48a648 on this branch): the Stage 1 check table `stage1Checks` and `JudgeDeclaration` (`internal/component/plugin/server/stage1_checks.go`), which runs the checks that need no daemon; `plugin.QueryDeclaration`. Not in the tree at audit time: `config/yang/join.go`, every `external-yang-*` test, any caller of `PluginOnlySchema`, any reader of `reg.PluginSchema`
- the doctor config-claims check and `internal/core/diagnostic/codes.go` - external roots and the three new codes
- `docs/plugin-development/schema.md`, `protocol.md`, `README.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/plugin/plugin-system.md`, `docs/architecture/config/yang-config-design.md`, `docs/guide/plugins.md`, `ai/doors/plugin-infra.md`, `ai/doors/config-yang.md`, `ai/doors/hub-engine.md`, `ai/digests/hub-engine.md`, `features/plugins.md`

## Files to Create
- `internal/component/config/yang/join.go` and `join_test.go` - the join function and its fixtures
- `internal/component/config/join_schema_test.go` - the join's entry: AC-10 (range, pattern), AC-6 and AC-9 at the schema, and the deleted duplicate drop. AC-10's mandatory leaf is not the parser's check but the YANG validator's, run by the full load: `TestPublishedExternalModuleOwesItsMandatoryLeaf` (`internal/component/config/declared_schemas_test.go`) proves it through `ReadConfig` over a published module, and the `ze start mandatory.conf` step of `external-yang-boot-accepts-what-offline-refuses.ci` proves it at boot
- `internal/component/config/yang/publish.go` and `publish_test.go` - the external layer of the process's module set: `PublishExternalModules`, `PublishedExternalModules`, `PublishedModule`, `KeepsModuleSet`, the test reset
- `internal/component/config/publish_schema_test.go` - the config entry `PublishExternalModules` and the schema and validator caches behind it
- `module_set_keeper_test.go` in `plugin/server`, `gnmi`, `iface`, `firewall` and `traffic`; `yang_state_publish_test.go` in `cli/client`; `headless_publish_test.go` in `cli/testing`; `completer_publish_test.go` in `cli` - one test for each value kept from the module set
- the functional tests listed above, and the fixture plugin they share

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No Ze module changes; the plugin brings its module |
| YANG validation constraints | Yes | The loader's checks apply to an external module unchanged (AC-10) |
| YANG custom validators | Yes | An external module that names one the daemon lacks is refused (AC-11) |
| CLI commands/flags | No | No command added |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | Yes | Automatic once the module is in the published set (AC-13) |
| Functional test for new RPC/API | Yes | `test/plugin/external-yang-*.ci` |
| Pipe completeness | N-A | No new output |
| Env var registration | No | The query budget `ze.plugin.query.timeout` exists |
| Doctor check for runtime dependencies | Yes | A declared external plugin must be runnable and reach its Stage 1: `doctor-plugin-schema-undeclared`, raised at load and at the runtime phase |
| Doctor code per problem class | Yes | `doctor-plugin-schema-undeclared` (a declared plugin did not reach its Stage 1, with why), `doctor-plugin-schema-refused` (join refusal, at load), `doctor-plugin-schema-changed` (a restarted plugin declares a different module, runtime); the claims check counts external roots |
| Prometheus counters/metrics | No | Startup-only path |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not a family change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/plugins.md`: an external plugin's own config root |
| 2 | Config syntax changed? | No | The plugin's module defines its own syntax |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`: an external plugin with its own config, the restart rule |
| 6 | Has a user guide page? | Yes | `docs/plugin-development/schema.md`: "What works today", the `-conf` rule, `augment` for a shared container, the refused extensions, corrected examples |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/plugin-development/protocol.md`, `docs/architecture/api/process-protocol.md`: a declared external plugin's Stage 1 runs before the config is parsed, and it waits at the barrier; "Query Mode": no command that parses a config starts a plugin; query mode serves `show plugin declarations` and the conformance check |
| 9 | RFC behavior implemented, changed, or newly proven? | No | - |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`: the fixture plugin that declares a module |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/hub-architecture.md` sections 2 and 3 and the "Startup Flow Diagram", now the live order; `docs/architecture/plugin/plugin-system.md` "Startup phases and callbacks": the early Stage 1, and the owner ruling that a plugin changes only when `ze` restarts; `docs/architecture/config/yang-config-design.md` "Module Loading"; `ai/doors/plugin-infra.md`, `ai/doors/config-yang.md`, `ai/doors/hub-engine.md`, `ai/digests/hub-engine.md` |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived at implementation with `./le spec citation anchors` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/plugin-development/schema.md` acme-monitor example, `README.md` "Extend configuration" |

## Implementation Steps

Each phase edits the pages its row names as it codes. Tests are written first and seen red for the reason their AC names.

1. **Phase: Wiring (MANDATORY FIRST)** -- the fixture plugin and the functional tests, red
   - Tests: every `external-yang-*` test above
   - Files: `test/plugin/`, `test/reload/`, `test/editor/`, the fixture plugin
   - Verify: `external-yang-own-root` red with `unknown top-level keyword`
2. **Phase: Audit** -- validate A-1, A-2, A-3, A-5, A-7 and the census of offline entries that parse a config
   - Output: the assumptions table updated; the list of what `runYANGConfig` builds before and after the full parse; the list of offline entries written into Files to Modify
3. **Phase: Join** -- the join function and every refusal
   - Tests: the `TestJoin*` rows
   - Files: `config/yang/join.go`, `yang_schema.go`
   - Verify: the old text road and its duplicate drop are gone
4. **Phase: First pass and publish** -- the parser mode, the command-line merge, the external layer of the module set
   - Tests: `TestFirstPass*`, `TestPublishedSetReachesSchemaCaches`
   - Files: `parser.go`, `loader.go`, `yang/register.go`, the three caches
5. **Phase: Early Stage 1 at boot** -- the plugin server before the full parse, the barrier, the join fed by live declarations
   - Tests: `TestEarlyStage1*`, `TestNoExternalPluginKeepsBootOrder`
   - Files: `cmd/ze/hub/main.go`, `plugin/server/startup.go`, `startup_driver.go`
   - Verify: `external-yang-own-root`, `-command-line`, `-invalid-refused`, `-clash-refused`, `-augment-shared-root`, `-query-failed`, `-commit` green
6. **Phase: Offline commands** -- validate and the other offline entries take the first pass through one shared step, start no plugin (owner ruling, 2026-11-16), and say that a section of an external plugin is not available with no daemon, with the daemon form that checks it. As first built (commit `d8caeb5c2d`) the step ran each declared plugin in declare mode and joined its module; the rework removed that run
   - Tests: `TestDeclaredSchemasStartsNoDeclaredProgram`, `TestParseRefusalSaysTheSectionIsNotAvailableOffline`, `TestValidateRefusesSectionOfExternalPluginWithoutStartingIt`
   - Files: `config/declared_schemas.go`, `config/cli/cmd_validate.go`, the entries the audit lists
   - Verify: `external-yang-offline-not-available`, `external-yang-boot-accepts-what-offline-refuses` green
7. **Phase: Restart rule** -- a restarted plugin compared with the published set, the reload refusal
   - Tests: `TestStage1RefusesChangedDeclaration`
   - Files: `plugin/server/startup.go`, `reload.go`
   - Verify: `external-yang-declaration-changed`, `-reload-refused` green
   - Written 2026-11-17, nothing run (the main thread runs the owed commands). What was built, and where it differs from the two lines above:
     - Reload (AC-12): `pluginserver.RefuseChangedDeclarations` (`plugin/server/reload_declarations.go`) compares the `External` list of boot's first pass with the one of the reload's config. It is called by `diskConfigLoaders` (`cmd/ze/hub/main_reload.go`), the loader every reload reads its config through, before the full parse; `plugin/server/reload.go` is not edited, because the comparison needs the config text and that loader is its one reader. The refusal is logged and raised as `doctor-config-reload-refused`.
     - Restarted plugin (AC-7): `refuseChangedSchema` (`plugin/server/stage1_schema.go`), the `schema-unchanged` entry of the Stage 1 table, compares each Stage 1 of an external plugin with `yang.PublishedModule`. `startup.go` is not edited: the table is what `onRegistration` runs. The code `doctor-plugin-schema-changed` is added here with its event registration, because the refusal logs at Warn; the other two schema codes stay in phase 8.
     - Boot's record of who declared: `config.RecordStartedPlugins`, from `startedPlugins` in `cmd/ze/hub/declared_plugins.go`. A served command no longer names, beside a name no schema defines, a plugin the daemon started that declared no module.
     - AC-12 is restated to external plugins (see its row).
     - Lifecycle scenarios of `ai/rules/testing.md`. Reached: "a plugin fails a startup stage after it declared", because the Stage 1 refusal takes the existing failed-stage path; `external-yang-declaration-changed` proves it in external mode, the only mode compared (a plugin compiled into `ze` cannot change its module). Not reached: a process that ends under each failure policy, the restart bound, the `respawn` leaf, an exit or a signal during the stop, a reload after shutdown began, a crash or a missed deadline in verify or apply, a reload whose context ends during a handshake, and a command dispatched at startup. The reload refusal returns from the loader before any transaction opens, as a parse refusal of the new config does, and the Stage 1 check adds no state to those paths.
8. **Phase: Surfaces and doctor** -- the editor, the schema CLI, the claims check, the three codes
   - Tests: `TestDoctorClaimsCountExternalRoots`
   - Verify: `ui.external-yang-session-editor` (to be promoted as `test/ui/external-yang-session-editor.ci`), `external-yang-doctor` green
   - Written 2026-11-17, nothing run (the main thread runs the owed commands). What was built, and where it differs from the lines above:
     - Schema CLI: `getExternalPluginYANG` and its run of a program with `--yang` are deleted (`config/schema/cli/main.go`). `buildSchemaRegistry` skips a command-line spec that names an external program and registers each module of `yang.PublishedExternalModules`, with the roots the running plugin wants, so the schema commands the daemon serves answer an external module from the published set. With no daemon `ze schema` answers the compiled-in modules and writes one operator error on stderr (`externalNotShown`): the module is not available with no running ze, fix `ze cli`. No path-to-file option feeds a module to the schema CLI; `--yang` stays a flag of Ze's own plugin CLI, which starts no program. Of the three tracked tests of the deleted run in `main_test.go`, the test judge approved the removal of `TestGetExternalPluginYANG` and `TestExternalPluginNonexistent` and refused that of `TestExternalPluginEmptySpec`: an empty `--plugin` value was refused with exit 1 and must stay refused. `buildSchemaRegistry` refuses an empty or blank spec with an operator error (`emptyPluginSpecRefusal`, fix `ze show plugin list`), and the test is rewritten against it, through `Run` as well. `TestRunWarnsOfExternalPluginOnStderr` proves each `ze schema` command writes the not-available warning.
     - Claims check: the plugin server keeps the roots each running external plugin declared (`pluginserver.ExternalConfigRoots`, recorded by `onRegistration`, forgotten by a process cleanup hook), and `checkConfigClaims` counts them. `TestDoctorClaimsCountExternalRoots` is in `internal/component/doctor/checks_config_claims_external_test.go`.
     - Codes: `doctor-plugin-schema-undeclared` and `doctor-plugin-schema-refused` are declared in `codes_plugin_infra.go`, registered as server event codes, and raised by `pluginserver.ReportUndeclaredSchema` and `ReportRefusedSchema`, which boot calls (`cmd/ze/hub/main.go`). The undeclared code also covers a plugin that declared with no YANG module, named beside the refusal of a name no schema defines (`SchemalessRefusal`, `schemalessPlugins`). The lines boot adds beside a load refusal are now written only for such a refusal (`config.RefusesUnknownNode`).
     - Where each code is observable. Undeclared: on the log line at boot; in `ze show doctor` when the config holds no section of the plugin, so the daemon starts; on the terminal beside the unknown keyword when it does. Refused: on the terminal and the log line, then in the local `ze doctor`. The start ends and the report bus ends with the process, so boot writes the refusal to the last-stop record (`meta/plugin/last-stop`, `RecordRefusedSchemaStop`; the store is set before `runYANGConfig`), and the `plugin-fatal-stop` check reports the entry under `doctor-plugin-schema-refused` with the time, the refusal's text and its fix. The same record carries `doctor-plugin-schema-undeclared` when the load refuses a name no schema defines (`RecordUndeclaredSchemaStop`). `Server.settleLastStop` deletes the record at the next clean start. A daemon with no store writes none and logs that under the same code. "Doctor on a daemon" still cannot hold for either refused start, because no daemon runs: the local route is where they are read.
     - Editor: no product change was needed. `newEditor` reads `config.SchemaWithDeclaredPlugins` and `NewCompleter` reads `yang.DefaultLoader`, both the published set inside a daemon. `TestEditorCompletesExternalRootFromThePublishedSet` (`internal/component/cli/editor_external_root_test.go`) proves it. The draft `test/draft/editor/external-yang-completion.et` cannot run: the `.et` runner has no daemon and, by the ruling, starts no plugin, so its process never holds the module. It needs an `.et` option that publishes a fixture module, or a driver that types into the daemon's SSH session; neither exists. Still owed.
     - AC-13 in the daemon: `test/draft/plugin/external-yang-show-config.ci` covers `show config running` and its JSON. `show schema` and `show yang` have no functional test: `ze cli -c` answers those local-data commands in its own process, so a test needs a caller that dispatches through the daemon (a fixture using `fixture.Dispatch`, or REST). Save has no test. Still owed, with AC-6 "the internal owner of `service` keeps working" and AC-7 "the config under its root stays loaded".
     - Corrected 2026-11-17 after the first test run, which supersedes the two lines above where they differ:
       - The daemon serves `show schema list`, `methods`, `events`, `handlers` and `protocol`, and no `show schema show <module>`: it answered `unknown command`. What a running Ze shows of one module is its name and wanted roots in `show schema list` and its nodes, types and descriptions in `show yang tree config`. No daemon command prints a module's YANG text. The `ze schema` warning (`externalNotShown`) now names those two commands, typed in a `ze cli` session, and the pages that named `show schema show` are corrected.
       - `test/draft/ui/external-yang-daemon-schema.ci` sends `show schema list` and `show yang tree config` over SSH exec, types `show schema list` at the operational prompt of the daemon's session (what a `ze cli` session is), and reads `show config running`.
       - `test/draft/ui/external-yang-session-editor.ci` is the editor's functional proof, through a driver that types into the daemon's SSH session. It proves the root and its leaves are offered, `?` after Tab shows the leaf's YANG description, a value outside the range is refused with the range, and a value inside it is taken. The completion box holds names alone, so no description is asserted after Tab. `test/draft/editor/external-yang-completion.et` is superseded and is deleted when the session test is promoted.
       - AC-6's owner half: `test/draft/plugin/external-yang-augment-owner-keeps-section.ci` reads `show dns resolver`, which answers the listener and allow-from count only when the dns-resolver plugin received its child of `service`.
       - Still owed for AC-13: save. A commit typed in the daemon's session, the stored config holding the block, and a second start that parses it have no functional test.
       - `await=stderr` beside `expect=exit:code=` asserted nothing about the daemon's exit in `external-yang-own-root`, `-command-line`, `-augment-shared-root` and `-boot-accepts-what-offline-refuses` (journal row 2026-11-17, owned below). Drafts of the first three add `then=stop`; the fourth holds three commands in one file and is not converted.
9. **Phase: Measure** -- Q10 on a known host, the figure written into `docs/plugin-development/schema.md`
10. **Phase: Validate asks the running Ze** -- `ze config validate` sends the config to the running daemon first, through `show config supplied validation`; design, criteria AC-17 to AC-25 and tests in "Phase 10" under Key Design Decisions
   - Tests: the `TestValidateRoute*` rows
   - Files: `cli/sshclient/reach.go`, `doctor/local_route.go`, `config/declared_schemas.go`, `config/cli/validate_route.go`, `cmd_validate.go`, `ssh/answer.go`, `core/diagnostic/types.go`
   - Verify: `external-yang-validate-asks-running-ze` green as a draft, then promoted

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path |
| Correctness | One road for boot, validate and reload; no reader holds a schema older than the published set |
| Naming | Doctor codes start `doctor-plugin-schema-`; no core package names a plugin |
| Data flow | The query stays in `plugin`, the join in `config/yang`; no second parser exists |
| Operator errors and doctor codes | Every refusal names the plugin, the module, the cause and the fix on its last line |
| Rule: `no-layering` | The old text road, its duplicate drop and `--yang` are deleted, not kept beside the join |
| Rule: `principles` | A plugin that did not declare never reads as "no module": the outcome is kept and reported |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The join function is the only producer of a schema with external modules | grep for `AddModuleFromText` callers |
| `reg.PluginSchema` has a reader | grep |
| `PluginOnlySchema` has a caller | grep |
| The docs no longer say "Not yet used" | grep `docs/plugin-development/schema.md` |

### Security Review Checklist
| Check | What to look for |
|-------|------------------|
| Input validation | Module text is bounded by `rpc.MaxMessageSize`; the join runs every loader check |
| Execution | With no daemon, no command that parses a config starts a declared program (owner ruling, 2026-11-16). `show plugin declarations` and the conformance check run the run string the operator names; no caller input reaches it |
| Resource limits | The query budget and group kill; a bound on the number and size of declared modules |
| Trust | Owner ruling: no sandbox. The two refusals of the Task hold |

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

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `unwired-feature.md` | 2026-11-02 | A plugin's Stage 1 `Schema.YANGText` is stored in `reg.PluginSchema` and never read, so its command arguments are judged only against compiled-in YANG (D-3 decides whether its fix is this spec's last phase) |
| `green-that-could-not-have-been-red.md` | 2026-11-17 | `await=stderr` beside `expect=exit:code=` with no peer asserts nothing about the daemon's exit; four of this spec's tracked tests hold the pair, three have a corrected draft with `then=stop` |
| `unwired-feature.md` | 2026-08-13 | external plugin YANG never reaches the daemon schema (moved 2026-11-14 from `plan/release/fixes/spec-yang-single-source-of-truth.md`; AC-1 here holds its old AC-11 schema half, and that spec's AC-3 holds the run-time unread-leaf report) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| No trust sandbox (owner ruling 2026-10-16) | Signed modules, allow-lists | "if a user use an external plugin the security is on him" |
| The load order is the design page's: plugin declarations read first, from the config or the command line, the plugins run, their modules taken at Stage 1, then the full parse (owner, 2026-11-15) | A separate declaration query before the parse with the plugin started later (the first draft of this spec, set aside: it ran each plugin twice and departed from `hub-architecture.md`); a YANG file path in the `plugin external` block (declares the module twice) | "It is in the design." A plugin is run once, and the protocol starts when it runs |
| With no daemon, a command starts no external plugin and reports a section of one as not available (owner ruling, 2026-11-16) | Read the declaration through declare mode (the design approved on 2026-11-16 and built in phase 6, reversed by the ruling: inspecting a config someone else wrote ran their command); an opt-in keyword on the command; a schema cache written at boot or by the conformance check; start a hub for validate | "plugin command requiring external plugins which can not be started from the code when no daemon is running to pass it to are not available, for command built-in then yes you should run them as we can look them up in the yang hardcoded in the binary" |
| The set of plugins and modules is fixed while the daemon runs (owner ruling, 2026-11-15) | Start, stop or re-declare a plugin on reload | "the plugin can only be changed on restarting ze". A reload that would change it is refused, which is exact |
| No dependency on the build-time compile spec | Wait for it | Nothing of it exists; today's loader joins text. One join function is the seam it later re-implements |
| The wire is unchanged for config | A list of modules in `schema`; dropping the redundant `module` and `namespace` fields | One config module is all the config half needs. A second module kind comes only with command arguments (D-3) |
| An external module follows the `-conf` naming rule and uses `augment` for a shared container | Treat any declared module as config; allow a second definition | One rule for every module, compiled-in or not; a module that would add nothing is refused with the rule, never accepted in silence |
| A daemon-side extension the daemon does not hold is refused | Skip it, as an unregistered `ze:validate` is skipped today | An accepted annotation that checks nothing is a silent wrong answer; the plugin validates in its own verify step |

### Phase 10: `ze config validate` asks the running Ze first

This design was put to the owner on 2026-11-16. His answer was "Ignore the caps for that work." On 2026-11-17, told that two sessions had built the same daemon-side mechanism, he decided: "yes, keep theirs and point your client at it". The daemon side is therefore `show config supplied validation` (the `supplied` commands of `internal/component/config/cli/config_supplied.go`), and this phase builds the client alone. The `content` leaf this phase first added to `show config validation` is not built.

**Cap permission (one-off, for this piece of work only, not a rule).** The owner said the WIP caps are ignored for this work, should it need a spec of its own. It did not need one: it is a phase of this spec.

**Problem.** Phase 6 made `ze config validate <file>` with no daemon refuse a section only an external plugin's module defines, and told the operator to open `ze cli` and type `show config validation <file>`. Two defects:

| Defect | Evidence |
|--------|----------|
| With a daemon running, the local command still never asked it | The owner's sentence is "which can not be started from the code when no daemon is running to pass it to": with one running, the command is to be passed to it |
| The advice fails for the usual case | The daemon reads `<file>` on its own host through `command.CallerPath` (`internal/component/command/caller_path.go`), which admits the user's `ze` config directory and the store directory, never the operator's working directory |

**Choice.** `ze config validate <file>` asks the running daemon first, sending the file's content, and answers in its own process only when no daemon answers.

| Operator types `ze config validate ze.conf` | Before | After |
|---------------------------------------------|--------|-------|
| Ze running, the config has an external plugin's section | "not available", then advice that fails | Judged in full by the daemon, with the output and exit codes of the local command |
| Ze not running | "not available" | "not available", why no Ze answered, and the fix: start Ze, then the same command |
| Only compiled-in plugins | Validates locally | Validates locally, no daemon asked |

**Existing mechanisms, and how each is extended** (each verified at the code).

| Mechanism | Where | Extended how |
|-----------|-------|--------------|
| The local `ze doctor` asks the running daemon first and falls back; a failed ask is never an answer; one fix per reason | `internal/component/doctor/local_route.go` (`answerLocalWith`, `askDaemonOverSSH`) | The walk (address, probe, credential) is extracted to `sshclient.ReachDaemon`, which answers the step it stopped at. Doctor and validate both call it and each words its own fix. Doctor's texts and tests are unchanged |
| Reaching the daemon with no operator input | `sshclient.DaemonAddress`, `ProbeDaemon`, `LoadCredentialsNoPrompt` | Used through `ReachDaemon` |
| Judging bytes the caller holds: `show config supplied validation content <base64> [name <label>]`, bound 261,120 characters (195,840 bytes, one SSH packet) | `suppliedHandler` and `suppliedValidation` (`config_supplied.go`), the `supplied-content` and `supplied-source` groupings of `ze-config-cli-cmd.yang` | Used as it is. The client sends the file's bytes in `content` and the file's base name in `name`, and leaves `name` out when the daemon would refuse it. The line is judged by the model before it is sent |
| The verdict producer | `validationBytesAnswer`, shared by `show config validation` and the supplied form; it runs `runValidation`, which reads the schema through `config.SchemaWithDeclaredPlugins` | Unchanged. In a running daemon that schema holds the external modules published at start, so the section is judged in full |
| A config too large to send | `show config validation file <path>`, which reads a file inside the daemon's config directories (`command.CallerPath`) | Used for a config over the bound that lies there |
| Parity | `TestLocalCommandDaemonMatchesLocal`, `TestEveryCommandOnEverySurface` | Unchanged: they drive the local-data path through `command.ServeLocal`, which does not forward |

**Conflicts found with the design as presented, and what was done.**

| Finding | Evidence | Resolution |
|---------|----------|------------|
| The SSH exec channel dropped the payload of a refused local-data command, so the daemon's diagnostics of an invalid config never reached the caller | `internal/component/ssh/ssh.go`, the exec path wrote `writeExecFailure` alone when the executor answered an error; `localAnswerResponse` keeps the payload beside the refusal | Fixed in this phase: the channel writes that payload on stdout before the refusal line, and the terminator counts it. The design cannot draw a refusal without it |
| An account with no stored credential has no single command that supplies one | `readCredentials`: the store is readable by its owner only; another account names itself and gives a password | `ze config validate` gains `--user <name>`, as `ze cli` has, so the fix is one command |
| A stray positional word could fill a payload leaf | `update data key <key> <word>` bound the word to the mandatory `content` leaf | A new statement, `ze:keyword-only`, honoured by the binder, usage and completion, declared on the two `update data` content leaves. The `supplied-content` grouping belongs to the supplied commands' owner and does not declare it; its handler refuses every positional word itself |

**The compiled-in case.** A config that declares no external plugin is judged in the caller's process and no daemon is asked. Reasons: the owner's sentence splits the two cases ("for command built-in then yes you should run them as we can look them up in the yang hardcoded in the binary"); the table he saw says "Validates locally"; the local verdict is complete, so the result is identical by construction; and it keeps the verdict independent of which binary the daemon runs and of a foreign daemon on the default port. Doctor asks always because part of its answer lives only in the daemon for every config; for validate that holds exactly when an external plugin is declared. The standing rule that every operator command runs through the daemon is met by the daemon forms `show config validation` and `show config supplied validation`, which every surface serves.

**No loop.** `cmdValidate` is the only caller that asks. A process that holds a daemon's module set does not ask (`DeclaredSchemas.Offline` is false there), and the daemon serves both commands through `runValidation`, which asks nobody.

**Alternatives set aside.**

| Alternative | Why not |
|-------------|---------|
| Send only the path | Refused by `CallerPath` for a file in the working directory |
| A `content` leaf on `show config validation` | Built first on another branch; the supplied commands already carry the caller's bytes, and the owner kept those as the daemon side |
| Make every `ze cli -c` ask the daemon first | Overturns the documented rule that an answer about the caller's host stays local (`ai/doors/commands-cli.md`, `docs/architecture/api/commands.md`); too wide for this work |
| Ask the daemon for every config | Changes nothing an operator reads for a compiled-in config, and ties its verdict to the daemon's binary |

**Rulings checked against.** The owner ruling of 2026-11-16 (quoted in the Task section); the standing rule that every operator command runs through the daemon on every surface with the local command's output, the local route staying for when the daemon is unreachable; the decision of 2026-11-17 quoted above.

**Known limit, stated and not changed.** Through the daemon, as locally, validation is the parser, the YANG checks and the in-process verifiers. A plugin's live `OnConfigVerify` is not run.

**The other offline commands.** `fmt`, `dump`, `diff` and `graph` follow later on the same route, through their own supplied forms. Until then their refusal says the command asked no running Ze and names `ze config validate <file>` as the fix.

| AC | Input | Expected |
|----|-------|----------|
| AC-17 | Ze runs with the fixture plugin; `ze config validate` on a file in the working directory with a valid section under the plugin's root | Exit 0, `configuration valid: <file>`, the output of a local acceptance |
| AC-18 | The same, with a value outside the module's range | Exit 1, the range refusal, no unknown keyword |
| AC-19 | No Ze listens | Exit 1: the unknown keyword, the declared plugin, that a running Ze was asked and why none answered, `fix: ze start`, and the same command to run again. No `show config validation`, no `fix: ze cli` |
| AC-20 | The ask fails another way (no credential, login refused, an answer that is no validation result, a verdict that disagrees with its status) | The local verdict, with the reason and its own fix; never an acceptance |
| AC-21 | A config that declares no external plugin | No daemon asked; the verdict of `runValidation` unchanged |
| AC-22 | A process that holds a daemon's module set | No daemon asked |
| AC-23 | A config over 195,840 bytes outside the daemon's config directories | Not sent; the message names the bound and the directories, and the fix checks a copy placed there. Inside those directories it is sent by path to `show config validation` |
| AC-24 | The line the client sends | `show config supplied validation content <base64> name <label>`, the label being the file's base name; no label for standard input or a base name the daemon refuses |
| AC-25 | Every other caller of `ParseRefusal`, `Explain` and `Unchecked` | Its message says it asked no running Ze, and its fix is `ze config validate <file>` |

| Test | Location | Proves |
|------|----------|--------|
| `TestValidateRouteDrawsTheDaemonsAcceptance`, `...Refusal` | `internal/component/config/cli/validate_route_test.go` | AC-17, AC-18 over a stub ask; AC-24 with a label |
| `TestValidateRouteLeavesOutALabelTheDaemonRefuses` | same | AC-24 with no label |
| `TestValidateRouteFallsBackWhenNoDaemonListens`, `TestValidateRouteNamesOneFixPerReason` | same | AC-19, AC-20 |
| `TestValidateRouteDoesNotAskForCompiledInConfig`, `...FromInsideADaemon`, `...DoesNotSendOversizeContent`, `TestValidateRouteSendsOversizeConfigByPath` | same | AC-21, AC-22, AC-23 |
| `external-yang-validate-asks-running-ze` | `test/draft/plugin/`, to promote | AC-17, AC-18 through two processes |
| `external-yang-offline-not-available`, `external-yang-boot-accepts-what-offline-refuses` | `test/plugin/`, changed through drafts | AC-19 through the real entry |

Tests that asserted the phase 6 advice and were updated under the judge's approval (`ze-test-judge`): `TestParseRefusalSaysTheSectionIsNotAvailableOffline`, `TestParseRefusalNamesEveryDeclaredPluginAndNoFile`, `TestDeclaredSchemasSecondCallAnswersTheSame` and `TestParseRefusalNamesOneFix` (`internal/component/config/declared_schemas_test.go`), `TestValidateRefusesSectionOfExternalPluginWithoutStartingIt` (`internal/component/config/cli/cmd_validate_declared_test.go`), `test/plugin/external-yang-offline-not-available.ci` and `test/plugin/external-yang-boot-accepts-what-offline-refuses.ci`. The old expectation (`show config validation <file>`, `fix: ze cli`) is wrong against `command.CallerPath` and the owner's sentence, as the Problem table shows.

## Known Limitations
- In-process Go plugins still need a rebuild with the package in `internal/component/plugin/all`; runtime Go plugin loading is not in scope.
- A plugin, its declaration and its module change only when `ze` restarts (owner ruling). A reload that changes a plugin declaration is refused.
- With no daemon, no command checks a config section under an external plugin's root (owner ruling, 2026-11-16). The running Ze checks it: `ze config validate <file>` sends it the file's content (phase 10), and in a `ze cli` session, over SSH, the web, MCP, REST or gRPC, `show config validation <file>` reaches it for a file inside the daemon's config directories and `show config supplied validation content <base64>` for bytes the caller holds. `ze cli -c` answers those commands in its own process, so it does not. `fmt`, `dump`, `diff` and `graph` do not ask the running Ze yet.
- An external plugin attached inline to a BGP peer cannot bring a schema; it is declared under `plugin external` or on the command line.
- `ConfigProvider.RegisterSchema` stays unused.

## Work Not Done (homed elsewhere)

| Item | Home |
|------|------|
| Validating leafref values ("Ze validates that referenced values exist" was also removed from `docs/plugin-development/schema.md`) | `plan/post-release/features/spec-config-yang-type-leafref.md` |
| A plugin's command argument model from its own module (journal row 2026-11-02) | This spec, as a last phase with its own acceptance criteria, when D-3 is answered yes; a spec of its own when answered no |
| Defaults in the section delivered to an external plugin | `plan/release/fixes/spec-yang-defaults-applied-once.md` (D-1, AC-6 there) |
| Walking every schema section for custom validators | `plan/release/fixes/spec-validation-walks-every-schema-section.md` (AC-1 there) |

## Effort

Estimate 10 to 13 agent-days: first pass, the plugin server before the full parse and the early Stage 1, join with its refusals, publish and the three caches, the offline commands, the restart rule, doctor, docs, and the functional tests with a fixture plugin that declares a module.

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

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The scoped tests of the changed packages and the functional tests above pass; CI verifies the pushed commit
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/post-release/features/spec-plugin-external-yang-at-startup.md` only, in the same `./le commit create` script
