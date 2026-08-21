---
kind: note
level: MUST NOT
stage:
---
A command handler MAY answer with a `plugin.Records` rather than a built value. `Records.Rows` is walked once, before the handler's call returns, and MUST NOT be stored.
<!-- source: pkg/plugin/sdk/sdk.go -- Plugin.Run, Stage 3 declare-capabilities -->
<!-- source: pkg/plugin/records.go -- Records, Records.WriteAnswer -->
<!-- source: internal/component/plugin/ipc/rpc.go -- PluginConn.SendExecuteCommandAnswer -->
