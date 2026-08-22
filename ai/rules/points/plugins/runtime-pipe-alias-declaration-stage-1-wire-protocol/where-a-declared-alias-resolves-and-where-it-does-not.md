---
kind: note
level:
stage:
---
**A declared alias resolves over the SSH exec channel and in the daemon-hosted
interactive session. It does NOT resolve in `ze cli` with no command argument.**
That process runs its own copy of the interactive model and expands the chain
before it sends anything, and no plugin alias is registered there. An operator
reads `pipe error: unknown pipe operator: <name>`, and Tab offers the name
nowhere. The repair is a channel that carries the daemon's alias table to the
client, and it is not built. `cliClient.StreamMonitor` has the same gap.
<!-- source: internal/component/cli/model_mode.go -- executeOperationalCommand -->
