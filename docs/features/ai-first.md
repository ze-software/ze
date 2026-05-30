# AI-First Design

<!-- source: internal/component/mcp/handler.go -- MCP HTTP handler -->
<!-- source: cmd/ze/help_ai.go -- ze help --ai machine-readable reference -->
<!-- source: cmd/ze-test/mcp.go -- MCP test client -->

Ze is designed AI-first: the entire command surface is programmatically accessible and
self-describing. AI assistants do not need documentation -- the binary describes itself.

## Principle: The CLI Is the API

Every command available through `ze cli` (interactive or `ze cli -c` for one-shot) is exposed
programmatically via the MCP `ze_execute` tool. There is no separate "API surface" -- the CLI
and the API are the same thing. This means:

- No commands are CLI-only or API-only
- No translation layer between human and machine interfaces
- Any new CLI command is automatically available to AI assistants

## Self-Describing Command Reference

```
ze help command              # full command catalog, filterable
ze help command bgp          # filter to BGP-related commands
ze help command --json       # machine-readable JSON (for tooling, wiki generation)
ze help --ai                 # AI-oriented summary with recipes and context
ze help --ai --json          # machine-readable JSON reference
```

Generates a command reference from the live binary. The output is assembled
from the plugin registry, YANG schemas, and RPC registrations -- it cannot go stale because
it is generated from code, not written by hand.

`ze help command` is the human-facing catalog: every command with its description,
filterable by keyword. The `--json` form is consumed by `make ze-wiki-update` to
regenerate the wiki command catalog.

## Structured Diagnostics

```
ze config validate --json <file>
ze explain [--json] <diagnostic-code>
ze config fix --plan --json <file>
```

Config validation emits structured diagnostic records with stable codes, source spans,
expected/actual facts, and repair metadata. Agents parse JSON diagnostics instead of
scraping terminal prose.

Each diagnostic carries a stable code (e.g., `config-parse`, `config-yang-type`,
`config-listener-conflict`). Use `ze explain <code>` to get an explanation.

`ze config fix --plan --json` reports candidate repairs without editing files.
Repair plans carry safety labels (`format-only`, `section-local`, `behavior-preserving`,
`requires-human-review`) and stable repair IDs.

## Version-Matched Skills

```
ze skills list
ze skills get ze-diagnostics
ze skills get ze --full
```

The installed binary serves agent workflow guides matched to its exact version.
Skills cover diagnostics, config, commands, and agent edit loops. Agents load
only the skill relevant to the current task.

## MCP Transport

The MCP (Model Context Protocol) server wraps the CLI command surface for AI consumption:

| Tool | Description |
|------|-------------|
| `ze_announce` | Announce routes with typed parameters (origin, next-hop, communities, prefixes) |
| `ze_withdraw` | Withdraw routes |
| `ze_peers` | Monitor peer state, ASN, uptime |
| `ze_peer_control` | Teardown, pause, resume, flush peers |
| `ze_execute` | Run **any** CLI command -- full daemon control |
| `ze_commands` | List all available daemon commands |

Additional tools are auto-generated from the command registry. Every YANG command
and plugin command becomes a typed MCP tool automatically.

The `ze_execute` tool is the key: anything a human can type, an AI can execute. Route
management, RIB queries, peer lifecycle, configuration changes, event subscription,
schema discovery -- all through one interface.

Start with `ze start --mcp <port>` or configure via YANG (`environment/mcp`).

## What Makes This Different

Other software adds an API endpoint and hopes someone wraps it for AI. Ze exposes its
entire command surface through a self-describing interface that AI can discover and use
without external documentation. `ze help command` lists every command with its description.
`ze help --ai` adds context (recipes, families, update syntax). The MCP tools have typed
parameters. The command list is queryable at runtime (`command-list`, `command-help <name>`).

No other network daemon -- BGP or otherwise -- is designed this way.

See [MCP Guide](../guide/mcp/overview.md) for configuration and
[MCP Remote Access](../guide/mcp/remote-access.md) for tunneling.
