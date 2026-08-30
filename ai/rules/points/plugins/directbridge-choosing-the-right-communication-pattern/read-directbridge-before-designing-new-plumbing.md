---
kind: directive
level: MUST
stage:
---
- **Before designing any new core-to-plugin communication, `pkg/plugin/rpc/bridge.go` MUST be read to check whether DirectBridge already covers the case.** It gives typed direct function calls between the engine and an internal plugin, bypassing JSON serialization and socket I/O. Which pattern fits which problem is `docs/architecture/plugin/plugin-system.md`, "Communication patterns".
