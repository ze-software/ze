# Self-Documenting System

<!-- source: cmd/ze/yang/main.go -- ze schema subcommands -->
<!-- source: cmd/ze/environ/main.go -- ze env subcommands -->
<!-- source: cmd/ze/help_ai.go -- ze help --ai output -->
<!-- source: scripts/inventory/inventory.go -- make ze-inventory -->
<!-- source: scripts/inventory/commands.go -- make ze-command-list -->
<!-- source: scripts/docvalid/commands.go -- make ze-validate-commands -->
<!-- source: scripts/docvalid/doc_drift.go -- make ze-doc-drift -->

Ze is self-documenting: every plugin, environment variable, RPC, event type, and CLI command
is registered at startup and discoverable at runtime. Nothing exists unregistered -- the
system enforces this with compile-time registration (`init()`) and runtime abort on
unregistered access (`env.MustRegister()`).

## Runtime Introspection

| Command | What it shows |
|---------|---------------|
| `ze schema list` | All registered YANG modules (52 modules) |
| `ze schema show <module>` | Full YANG content for a module |
| `ze schema methods [module]` | All RPCs with parameters from YANG |
| `ze schema events` | All notification/event types from YANG |
| `ze schema handlers` | Which handler serves which YANG module |
| `ze schema protocol` | Protocol version and wire format info |
| `ze env list` | All registered environment variables with types and defaults |
| `ze env list -v` | Same, plus current values |
| `ze env get <key>` | Details for a single environment variable |
| `ze --plugins` | All registered plugins with families, capabilities, dependencies |
| `ze help --ai` | Machine-readable command reference generated from live binary |

## Build-Time Verification

| Make target | What it does |
|-------------|--------------|
| `make ze-inventory` | Full project inventory: plugins, YANG modules, RPCs, families, tests, packages |
| `make ze-inventory-json` | Same as above, machine-readable JSON |
| `make ze-command-list` | Every CLI command with wire method, help text, read-only flag, source |
| `make ze-validate-commands` | Cross-check YANG command tree against registered handlers |
| `make ze-doc-drift` | Detect documentation that no longer matches code |

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
