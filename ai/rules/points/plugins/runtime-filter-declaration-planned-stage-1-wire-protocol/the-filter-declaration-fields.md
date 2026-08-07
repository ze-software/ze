---
kind: table
level:
stage:
---
| Field | Type | Purpose |
|-------|------|---------|
| `filters[].name` | string | Filter name (config references as `<plugin>:<name>`) |
| `filters[].direction` | enum | import, export, both |
| `filters[].attributes` | []string | Attribute names to receive |
| `filters[].raw` | bool | Include raw wire bytes; REQUIRED for non-CIDR families |
| `filters[].on-error` | enum | reject (fail-closed) or accept (fail-open) |
| `filters[].overrides` | []string | Default filters this filter replaces |
