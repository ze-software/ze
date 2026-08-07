---
kind: note
level:
stage:
---
If your plugin calls a same-process-effect function directly, check `sdk.Plugin.IsInternal()` (`pkg/plugin/sdk/sdk.go`) right after `sdk.NewWithConn(...)` and choose severity by how much of the plugin's value survives running external:
