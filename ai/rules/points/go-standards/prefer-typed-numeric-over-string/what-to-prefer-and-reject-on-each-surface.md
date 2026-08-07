---
kind: table
level:
stage:
---
| Surface | Prefer | Reject |
|---------|--------|--------|
| Event/IPC payload crossing component seams | typed `uint8`/`uint16`, registered ID, `netip.Prefix`, `family.Family` | `string` for kinds (protocol, family, action, direction, state) |
| Hot-path dispatch key | integer const / typed enum | string switch |
| Hot-path map key | integer or struct | string |
| Internal state flags | typed enum, zero-invalid | magic strings |
| Hot-path comparison | `x == FooAdd` | `x == "add"` |
