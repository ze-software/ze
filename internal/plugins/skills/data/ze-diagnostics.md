---
name: ze-diagnostics
description: Read Ze diagnostics, explanations, and fix plans.
---

# Ze Diagnostics

Use this when Ze config fails to parse or validate.

## Commands

```sh
ze cli -c "validate config <file> | json"
ze explain [--json] <diagnostic-code>
ze config fix --plan <file>
```

## Diagnostic Shape

Fields from `validate config | json`:

- `code`: stable lower-kebab diagnostic code
- `severity`: `error` or `warning`
- `message`: short human summary
- `path`, `line`, `column`, `length`: source span
- `expected` and `actual`: mismatch facts when available
- `help`: concise next action
- `fix-safety`: safety label for a repair
- `repair`: optional repair id and summary
- `related`: extra spans or facts

## Fix Safety

- `format-only`: formatting only
- `section-local`: confined to one config section
- `behavior-preserving`: no runtime behavior change
- `api-changing`: config API shape may change
- `target-changing`: runtime target may change
- `requires-human-review`: cannot prove the edit is safe

## Agent Triage

1. Run the failing command with `--json`.
2. Use the span to inspect the relevant config.
3. Run `ze explain <code>` before broad changes.
4. Fix the earliest root cause first.
5. Re-validate after each fix.
