---
kind: directive
level: MUST
stage:
---
**A change to a `register.go` or an `init()` MUST also update everything its row names.**

| What changed | Also update |
|---|---|
| New plugin | `./le repository generate` (updates `all.go`), and the `TestAllPluginsRegistered` count |
| New family | `family.MustRegister()`, plus NLRI decoder and encoder registration |
| New capability | Capability codec registration |
| New event type | The `Registration.EventTypes` field |
| Renamed registered name | `ai/rules/plugins.md`, "Renaming a Registered Name", for the full grep |
