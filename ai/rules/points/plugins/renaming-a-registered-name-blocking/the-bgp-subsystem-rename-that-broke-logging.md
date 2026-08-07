---
kind: note
level:
stage:
---
The `938df51d` fix exists because BGP-as-plugin Phase 2 registered subsystems
as `bgp-gr` / `bgp-rib` etc., but config and `ze.log.*` env vars expected
`bgp.gr` / `bgp.rib`. Log levels were silently never applied. Six days passed
before review caught it.
