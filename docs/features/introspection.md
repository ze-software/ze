# Self-Documenting System

<!-- source: internal/component/config/yang/cli/main.go -- ze schema subcommands -->
<!-- source: internal/plugins/env/env.go -- ze env subcommands -->
<!-- source: cmd/ze/help_ai.go -- ze help ai output -->
<!-- source: internal/le/inventory/inventory.go -- Answer -->
<!-- source: internal/le/commandlist/commandlist.go -- Answer -->
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
| `ze --plugins` | All registered plugins with families, capabilities, dependencies |
| `ze help command [filter]` | Full command catalog, filterable, with descriptions |
| `ze help command --json` | Command catalog as JSON (for wiki generation, tooling) |
| `ze help ai` | Machine-readable command reference generated from live binary |
| `ze help ai api` | Daemon API endpoints (`ze-show:*`, `ze-set:*`, ...) with parameters |

One registration is out of reach of both catalogs. `ze help command --json` and
`./le command-list` read the compiled command tree in their own process, and
they start no plugin. Neither reports a pipe alias a plugin declared in its
Stage 1 message. The running daemon answers that question, through
`command help "<name>"` and through Tab completion in the interactive session.
The wiki catalog built from the JSON therefore lists a plugin's commands without
its aliases.
<!-- source: cmd/ze/help_command.go -- collectCommands, extractPipes -->
<!-- source: internal/plugins/meta/cmd/help.go -- commandHelp, pipeAliasHelp -->

## Build-Time Verification

| Native action | What it does |
|---------------|--------------|
| `./le inventory` | Reports plugins, YANG modules, RPCs, families, tests, and packages |
| `./le command-list` | Reads every CLI command from the compiled registries |
| `./le docvalid command-contract-check` | Cross-checks YANG commands and handlers |
| `./le docvalid doc-drift-check` | Detects documentation drift |

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
