---
name: ze
description: Ze network OS overview and agent entry points.
---

# Ze

Ze is a network OS with its own BGP implementation, YANG-modeled config, and agent-facing tooling.

## Agent Entry Points

```sh
ze cli -c "validate config <file> | json"
ze explain [--json] <diagnostic-code>
ze config fix --plan <file>
ze help ai --json
ze skills list
ze skills get <name> [--full]
```

Driving ze over MCP instead of the CLI? Call the `ze_reference` tool (it appears
in `tools/list` on connect) for the same `ze help ai --json` reference. <!-- doc-links: ignore (JSON-RPC method name, not a path) -->

## Version-Matched Skills

Use `ze skills list` to discover skills bundled with this binary.
Use `ze skills get <name>` to load the one relevant to your task.
