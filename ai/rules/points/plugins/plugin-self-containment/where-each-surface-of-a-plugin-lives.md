---
kind: table
level:
stage:
---
| Surface | Owner location |
|---------|----------------|
| RPC handler + `pluginserver.RegisterRPCs` | owner package (e.g. `internal/component/<owner>/cmd/` or the plugin's own package) |
| YANG command schema (full path from root, e.g. `show bgp peer ...`) | `<owner>/yang/ze-<x>-cmd.yang`, NEVER `<owner>/cmd/yang/` |
| Offline/root command + handler | owner package via the offline command registry |
| Help / usage / completion | derived from the owner's registry + schema (see `ai/rules/evidence.md`) |
| Doctor check + its unit test | owner package (Proximity Principle) |
