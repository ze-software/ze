---
kind: table
level:
stage:
---
| Endpoint kind | Grouping | Fields | Port type |
|---------------|----------|--------|-----------|
| Inbound bind (the service listens) | `uses zt:listener` + `ze:listener` extension | `ip` (local literal), `port` | `zt:listener-port` (0 = OS-assigned) |
| Outbound target (the service connects out) | `uses zt:endpoint` (add to `ze-types` if absent) | `address` (IP or hostname), `port` | `zt:port` (1..65535) |
