---
kind: table
level:
stage:
---
| What changed | Also update |
|---|---|
| New plugin | `./le repository generate` (updates `all.go`), `TestAllPluginsRegistered` count |
| New family | `family.MustRegister()`, NLRI decoder/encoder registration |
| New capability | Capability codec registration |
| New event type | `Registration.EventTypes` field |
| Renamed name | See `ai/rules/plugins.md` "Renaming a Registered Name" for full grep |
