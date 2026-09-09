# Self-Documenting System

<!-- source: internal/component/config/yang/cli/main.go -- ze schema subcommands -->
<!-- source: internal/plugins/env/env.go -- ze env subcommands -->
<!-- source: cmd/ze/help_ai.go -- ze help ai output -->
<!-- source: internal/le/inventory/inventory.go -- Answer -->
<!-- source: internal/le/command/list/commandlist.go -- Answer -->
<!-- source: internal/le/docvalid/actions.go -- Answer -->
<!-- source: internal/le/docvalid/actions.go -- Answer -->

Ze is self-documenting: every plugin, environment variable, RPC, event type, and CLI command
is registered at startup and discoverable at runtime. Nothing exists unregistered -- the
system enforces this with compile-time registration (`init()`) and runtime abort on
unregistered access (`env.MustRegister()`).

## Runtime Introspection

| Command | What it shows |
|---------|---------------|
| `ze schema list` | All registered YANG modules (65 modules) |
| `ze schema show <module>` | Full YANG content for a module |
| `ze schema methods [module]` | All RPCs with parameters from YANG |
| `ze schema events` | All notification/event types from YANG |
| `ze schema handlers` | Which handler serves which YANG module |
| `ze schema protocol` | Protocol version and wire format info |
| `ze env list` | All registered environment variables with types and defaults |
| `ze env list -v` | Same, plus current values |
| `ze env get <key>` | Details for a single environment variable |
| `ze show plugin list` | All registered plugins with families, RFCs, and capability codes |
| `ze show plugin declarations` | One row for each plugin, with the state that says whether its declaration was read |
| `ze show plugin declarations config <path>` | The same, plus a row for each plugin that config file names |
| `ze help command [filter]` | Full command catalog, filterable, with descriptions |
| `ze help command --json` | Command catalog as JSON (for wiki generation, tooling) |
| `ze help ai` | Machine-readable command reference generated from live binary |
| `ze help ai api` | Daemon API endpoints (`ze-show:*`, `ze-set:*`, ...) with parameters |

An in-tree plugin's declarations are in reach of both catalogs. `ze help command
--json` and `./le command list` read the compiled command tree in their own
process and start no plugin, but a registered plugin puts the same declarations
on its `registry.Registration`: `Commands` for what each answer holds and
`Pipes` for the aliases it puts on its commands. So both catalogs name every
plugin command and publish its `answer-shape`, `column-orders`,
`address-fields` and `pipe-aliases`. A command the plugin marks hidden is left
out of both, the way the daemon leaves it out of completion.

One registration is still out of their reach: an EXTERNAL plugin's. It registers
nothing in the composition root, so its declaration reaches a running daemon
alone, through `show command help "<name>"` and Tab completion in the
interactive session. That answer carries a command's two help texts under
`description` (the one-line summary) and `long-help` (the explanation), for a
builtin and for a plugin command alike.

`ze show plugin declarations` closes that last gap. One row for each plugin
this binary carries, one for each plugin the named config file declares, and no
plugin omitted. The plugins this binary carries are the set `ze show plugin
list` answers for, so a plugin whose own `init()` recorded a setup outcome and
never completed its registration keeps a row in both commands.

The config block decides where a row's declaration comes from, because the
block says which binary holds the plugin's code. An `internal` block names code
this binary carries, so the row is answered from the compiled-in
`registry.Registration` and no process is started. An `external` block names
another program, whatever name the block gives it, so the row is answered by
starting that program in query mode (`ZE_PLUGIN_MODE=declare`), with every
`ze.plugin.` variable removed from the child's environment, and reading the
declaration it writes to stdout. A block reading `external mrt { run
"/opt/vendor/my-mrt" }` is a vendor's program that took a name Ze also uses,
and it answers for itself. Anything else the child writes is ignored, so a
banner or a log line cannot corrupt the answer.

The state field carries one of five values, so absence never stands in for an
answer:

| State | What it means |
|-------|---------------|
| `declared` | an answer arrived carrying declarations |
| `declared-none` | an answer arrived carrying an empty declaration |
| `no-answer` | the process ran and wrote no declaration, which is what a plugin with no query mode in it does |
| `unstartable` | the process could not be started |
| `timeout` | the process started, wrote no declaration inside the budget, and was stopped |

The budget is 5 seconds, the same wait a startup stage gets, and
`ze.plugin.query.timeout` sets it. A value of zero or less is refused rather
than read as "wait no time". A plugin that runs out of the budget is stopped
with everything it started: the run string is given to a shell, and a shell
that runs more than one command holds the plugin as its own child.
<!-- source: internal/component/plugin/declarations.go -- dataDeclarations, declarationRows, queriedDeclarationRow -->
<!-- source: internal/component/config/register_plugin_declarations.go -- readConfiguredPlugins -->
<!-- source: cmd/ze/help_command.go -- collectCommands, extractPipes -->
<!-- source: internal/component/command/declared.go -- DeclaredForCommand -->
<!-- source: internal/plugins/meta/cmd/help.go -- commandHelp, pipeAliasHelp -->

## Build-Time Verification

| Native action | What it does |
|---------------|--------------|
| `./le inventory` | Reports plugins, YANG modules, RPCs, families, tests, and packages |
| `./le command list` | Reads every CLI command from the compiled registries |
| `./le docvalid command-contract` | Cross-checks YANG commands and handlers |
| `./le docvalid doc-drift` | Detects documentation drift |

Each plugin the inventory reports also carries the package directory it
registers from and every YANG file beside it. Both are DERIVED, so no plugin
declares either: the directory is the package the plugin's engine function was
compiled in, and the file list is the directory holding the module the
registration carries. The public plugin catalog publishes both.
<!-- source: internal/le/inventory/plugins.go -- pluginPackageDir, pluginYANGFiles -->

## Design Principle

The self-documenting property emerges from the registration architecture:

- **Plugins** register via `registry.Register()` in `init()` -- name, families, capabilities,
  YANG schema, dependencies, event types, send types, features
- **Environment variables** register via `env.MustRegister()` -- calling `env.Get()` with an
  unregistered key aborts the process
- **RPCs** are defined in YANG schemas -- no command handler exists without a schema definition
- **CLI dispatch** is auto-generated from registrations -- no hand-wired dispatch tables
- **Tab completion** is driven by YANG schemas -- new config leaves appear automatically
- **Web UI** is generated from YANG schemas -- no hardcoded forms

The result: adding a plugin with a YANG schema automatically updates the CLI, web UI, tab
completion, schema discovery, inventory, and environment variable listing. No manual wiring.

No other open-source BGP daemon provides runtime introspection of its own capabilities.
