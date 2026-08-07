---
kind: note
level:
stage:
---
The anchor still owns NO command. The central guard test bans each carved token so
a command cannot drift back into the central schema (for `clear`:
`internal/component/cmd/clear/yang/self_containment_test.go`,
`TestClearSchemaHasNoMigratedOwnerCommands`; each owner `yang/` holds the matching
presence test, e.g. `TestResolveCmdSchemaOwnsClearDNSCache`).
