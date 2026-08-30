---
name: ze-agent
description: Agent workflow for config validation and repair.
---

# Ze Agent Workflow

Use this when editing Ze config as an agent.

## Edit Loop

1. Read the config file before editing.
2. Make the smallest change that satisfies the request.
3. Validate:

```sh
ze cli -c "validate config <file> | json"
```

4. When validation reports a diagnostic:

```sh
ze explain <diagnostic-code>
ze config fix --plan <file>
```

5. Apply the fix, then re-validate.

## Rules

- Prefer structured JSON output over parsing text.
- Do not invent config syntax. Load `ze-config` when unsure.
- Do not invent CLI fields. Run the command with `--json` and read the data.
- Treat `requires-human-review` as a planning hint, not an automatic patch.
