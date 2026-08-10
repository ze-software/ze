---
name: ze-commands
description: CLI command reference, dispatch keys, and MCP tools.
---

# Ze Commands

Ze exposes commands through CLI, SSH, MCP, and HTTP API.

## Offline Commands

```sh
ze config validate|dump|fmt|diff|edit|set|migrate <args>
ze bgp decode|encode <args>
ze yang tree|completion|doc [module]
ze schema list|methods|events
ze explain [--json] <code>
ze skills list|get <name>
ze help ai [--json]
```

## Online Commands (Daemon)

Dispatch keys follow YANG tree paths:

```
show bgp peer <selector>
show version
delete bgp peer <selector>
```

Use `ze help ai --json` for the full generated command reference.

## MCP Tools

MCP tools are auto-generated from the YANG command registry.
Use `ze help ai --json` to see the same data in machine-readable form.

Over MCP, call the `ze_reference` tool (visible in `tools/list` on connect) to <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
get that same reference as JSON without leaving the protocol. `ze_execute` runs
any command; `ze_commands` lists them.
