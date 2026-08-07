---
kind: note
level:
stage:
---
To add a verb, edit `command.Verbs` (one place; the plugin gate and the static gate
both derive from it). Category exemptions (the text bridge, `ze-plugin:`/`ze-system:`
wire-protocol directives, and `ze-editor:` modes) live in `grammar.ExemptCategory`,
keyed on the handler wire-method namespace, never a per-command allowlist.
