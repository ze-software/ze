---
kind: directive
level: MUST
stage:
---
- **Every RPC MUST carry a YANG registration for the CLI, whether it is registered through `registry.Register()` or through `pluginserver.RegisterRPCs()`.** A command handler with no YANG schema is a structural defect to fix, not a different category. There is no "command module": everything with RPCs is a plugin and lives under `plugins/<name>/`.
