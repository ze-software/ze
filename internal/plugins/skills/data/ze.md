---
name: ze
description: Ze network OS overview and agent entry points.
---

# Ze

Ze is a network OS with its own BGP implementation, YANG-modeled config, and agent-facing tooling.

## Agent Entry Points

```sh
ze cli -c "show config validation <file> | json"
ze show diagnostic [--json] <diagnostic-code>
ze config fix --plan <file>
ze help ai --json
ze show skills
ze show skill <name> [--full]
```

Driving ze over MCP instead of the CLI? Call the `ze_reference` tool (it appears
in `tools/list` on connect) for the same `ze help ai --json` reference. <!-- doc-links: ignore (JSON-RPC method name, not a path) -->

## Version-Matched Skills

Use `ze show skills` to discover skills bundled with this binary.
Use `ze show skill <name>` to load the one relevant to your task.
