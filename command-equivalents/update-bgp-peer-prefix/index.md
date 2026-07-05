# `update bgp peer <selector> prefix`

## Ze command

- Syntax: `update bgp peer <selector> prefix`
- Registry path: `update bgp peer prefix`
- Mode: Daemon
- Wire method: `ze-update:bgp-peer-prefix`
- Global pipes: yes

Refresh max-prefix limits from PeeringDB. Usage: update bgp peer <selector> prefix. Queries PeeringDB for each matched peer's ASN, applies the configured margin, and writes the result to the config draft. Run 'config commit' to apply.

## Mapping intents

No vendor equivalent has been curated yet for this Ze command.
## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
