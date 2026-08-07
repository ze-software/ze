---
kind: table
level:
stage:
---
| Directory | Contains | Depends on |
|-----------|----------|------------|
| `core/` | Shared primitives (logging, crashlog, env, events, diagnostics) | Nothing internal (or only other `core/`) |
| `component/` | Subsystem implementations + config YANG (data model) | `core/`, other `component/` |
| `plugins/` | User-facing command surfaces: command YANG, RPC handlers, CLI registration | `core/`, `component/` |
