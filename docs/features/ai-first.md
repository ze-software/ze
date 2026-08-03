# AI-First Design

<!-- source: internal/component/mcp/tools.go -- MCP tool dispatch primitives -->
<!-- source: cmd/ze/help_ai.go -- ze help ai machine-readable reference -->
<!-- source: internal/test/cli/cmd_mcp.go -- MCP test client -->
<!-- source: ai/rules/repo-maintenance.md -- Current Discovery Surfaces -->
<!-- source: mk/inventory.mk -- ze-inventory and documentation targets -->

Ze is built around a single command and discovery surface. Commands,
configuration nodes, RPCs, events, and plugin metadata are registered once, then
exposed through MCP and the operator interfaces that need them. AI tools can
discover the same command catalog, run the same actions, and read structured
output while helping an operator use or debug the system.

## Register Once, Expose Everywhere

The core design point is broader than AI. A feature added to Ze should avoid
separate hand-written glue for every surface. The same registration path can
feed the CLI, SSH sessions, the web workbench, REST/gRPC, MCP, generated
references, completion, authorization, audit, and diagnostics.

## Principle: The CLI Is the API

Every command available through `ze cli` (interactive or `ze cli -c` for
one-shot) is exposed programmatically through MCP, and command output is shared
with the API engine where those commands are surfaced. This means:

- Features do not become CLI-only by accident
- Human and machine interfaces use the same command grammar
- New commands become automation surfaces when they register with the command catalog

## Self-Describing Command Reference

```
ze help command              # full command catalog, filterable
ze help command bgp          # filter to BGP-related commands
ze help command --json       # machine-readable JSON (for tooling, wiki generation)
ze help ai                   # AI-oriented summary with recipes and context
ze help ai --json            # machine-readable JSON reference
ze help ai api               # daemon API endpoints (ze-show:*, ze-set:*, ...)
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

## Development-Time Discovery

Feature, tooling, self-check, verification, and test-infrastructure changes must
update their discovery path in the same work. The standard path is
`ai/rules/repo-maintenance.md` for policy, `ai/INDEX.md` for keyword lookup,
`ai/NAVIGATION.md` for task routing, and the relevant make target or docs page
for verification and usage.

<!-- source: ai/rules/repo-maintenance.md -- Required Discovery Artifacts -->

Agents should use the existing inventory and verification surfaces before
inventing new ones: `make ze-inventory`, `make ze-command-list`,
`make ze-doc-test`, `make ze-doc-index`, and `make ze-verify-wiring-docs`.

Commit preparation uses `scripts/dev/commit_helper.py`: agents pass the vetted
subject, body, and explicit file list, and the helper creates the session ID,
message file, executable user-run script, ignored-path checks, `git commit -F`
flow, and learned-summary gate for workflow/tooling/rule changes.
<!-- source: scripts/dev/commit_helper.py -- commit helper CLI and lesson gate -->

<!-- source: mk/inventory.mk -- quick reference and targets -->

## MCP Transport

The MCP (Model Context Protocol) server wraps the CLI command surface for AI consumption:

| Tool | Description |
|------|-------------|
| `ze_execute` | Run **any** CLI command -- full daemon control |
| `ze_reference` | Machine-readable reference for this daemon (commands, endpoints, dispatch keys, plugins, families); same JSON as `ze help ai --json` |
| `ze_announce` | Announce routes with typed parameters (origin, next-hop, communities, prefixes) |
| `ze_withdraw` | Withdraw routes |
| `ze_show_bgp` | BGP peer state, ASN, uptime, and summary views (auto-generated from `show bgp ...`) |
| `ze_request_peer` | Peer lifecycle: teardown, pause, resume, flush (auto-generated from `request peer ...`) |

Additional tools are auto-generated from the command registry. Every YANG command
and plugin command becomes a typed MCP tool automatically, so an AI agent can
discover features, execute the same commands as an operator, and inspect
structured outputs while debugging.

The `ze_execute` tool is the bridge: anything a human can type, an AI can
execute. Route management, RIB queries, peer lifecycle, configuration changes,
event subscription, schema discovery, doctor output, warnings, and health checks
all go through one interface.

Start with `ze start --mcp <port>` or configure via YANG (`environment/mcp`).

## What Makes This Different

Other software adds an API endpoint and expects operators to build separate
wrappers, UI, documentation, and automation around it. Ze exposes its command
surface through a self-describing interface that tools can discover at runtime.
`ze help command` lists every command with its description. `ze help ai` adds
context (recipes, families, update syntax). MCP tools have typed parameters. The
command list is queryable at runtime (`show command list`,
`show command help <name>`).

The useful property is the shared surface: when a plugin or subsystem registers
commands, YANG, RPCs, or events, the same metadata can feed CLI, web, generated
references, automation, authorization, audit, and diagnostics.

See [MCP Guide](../guide/mcp/overview.md) for configuration and
[MCP Remote Access](../guide/mcp/remote-access.md) for tunneling.
