---
kind: table
level:
stage:
---
| Category | Meaning | Allowed home |
|----------|---------|--------------|
| `framework` | Wiring substrate or setup feature that exists to register, configure, command, audit, or orchestrate other packages. | `internal/component/` or setup packages under `internal/plugins/` |
| `host-service` | Listener, appliance, host API, or platform service pinned to composition by startup or doctor/platform registration. | `internal/component/` |
| `domain-library` | Non-engine package that belongs to a real domain cluster. In this spec that means BNG and VPN only. | `internal/component/` |
| `planned-violation` | Existing known placement that is scheduled to move or disappear. New rows need a spec reference in the rationale. | `internal/component/` or `internal/plugins/` |
