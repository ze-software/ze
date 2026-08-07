---
kind: note
level:
stage:
---
Non-BGP owners share one general central guard,
`TestShowSchemaHasNoMigratedOwnerCommands` (same file), whose banned-token map
grows by one entry per carved owner (flow-export, rsvp-te, ldp, policy-routes,
static, vpn-ipsec, vpp, the iface kernel reads, ...); each owner's `yang/`
package holds the matching presence test (e.g. `TestRSVPTECmdSchemaOwnsShowRSVPTE`).
When you carve a new command, add both halves: the banned token here and the
presence assertion in the owner. Extend the same pattern to the other central
verb schemas (`internal/component/cmd/delete`, `internal/component/cmd/set`, ...) as they are made compliant.
