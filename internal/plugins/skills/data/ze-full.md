---
name: ze
description: Ze network OS overview and agent entry points (expanded).
---

# Ze

Ze is a network OS in Go with its own BGP implementation, YANG-modeled config,
SSH-accessible CLI, HTMX web UI, and agent-facing tooling.

## Agent Entry Points

```sh
ze cli -c "validate config <file> | json"
ze explain [--json] <diagnostic-code>
ze config fix --plan <file>
ze help ai --json
ze skills list [--json]
ze skills get <name> [--full] [--json]
```

## Diagnostic Loop

1. Run `ze cli -c "validate config <file> | json"` to get structured diagnostics.
2. For each diagnostic, check `code` and `help` fields.
3. Run `ze explain <code>` for codes you do not recognize.
4. Run `ze config fix --plan <file>` for repair candidates.
5. Apply the smallest fix, then re-validate.

## Skills

Use `ze skills list` to see all bundled skills.
Use `ze skills get ze-diagnostics` for the diagnostic workflow.
Use `ze skills get ze-config` for config syntax and common errors.
Use `ze skills get ze-commands` for CLI and dispatch key reference.
Use `ze skills get ze-agent` for the agent edit loop.
