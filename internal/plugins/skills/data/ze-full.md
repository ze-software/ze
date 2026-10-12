---
name: ze
description: Ze network OS overview and agent entry points (expanded).
---

# Ze

Ze is a network OS in Go with its own BGP implementation, YANG-modeled config,
SSH-accessible CLI, HTMX web UI, and agent-facing tooling.

## Agent Entry Points

```sh
ze cli -c "show config validation <file> | json"
ze show diagnostic [--json] <diagnostic-code>
ze config fix --plan <file>
ze help ai --json
ze show skills [--json]
ze show skill <name> [--full] [--json]
```

## Diagnostic Loop

1. Run `ze cli -c "show config validation <file> | json"` to get structured diagnostics.
2. For each diagnostic, check `code` and `help` fields.
3. Run `ze show diagnostic <code>` for codes you do not recognize.
4. Run `ze config fix --plan <file>` for repair candidates.
5. Apply the smallest fix, then re-validate.

## Skills

Use `ze show skills` to see all bundled skills.
Use `ze show skill ze-diagnostics` for the diagnostic workflow.
Use `ze show skill ze-config` for config syntax and common errors.
Use `ze show skill ze-commands` for CLI and dispatch key reference.
Use `ze show skill ze-agent` for the agent edit loop.
