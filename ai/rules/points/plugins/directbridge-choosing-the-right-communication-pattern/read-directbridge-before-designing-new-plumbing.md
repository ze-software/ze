---
kind: note
level:
stage:
---
DirectBridge (`pkg/plugin/rpc/bridge.go`) provides typed direct function calls
between the engine and internal plugins, bypassing JSON serialization and socket
I/O entirely. It supports multiple communication patterns. Before designing any
new core-to-plugin communication, read DirectBridge and check whether it already
covers your use case.
