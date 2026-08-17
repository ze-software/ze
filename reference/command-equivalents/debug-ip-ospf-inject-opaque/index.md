# `debug ip ospf inject opaque scope <link|area|as> id <opaque-id>`

## Ze command

- Syntax: `debug ip ospf inject opaque scope <link\|area\|as> id <opaque-id>`
- Registry path: `debug ip ospf inject opaque`
- Mode: Daemon
- Wire method: `ze-debug:ospf-inject`
- Global pipes: yes

Inject a crafted IPv4 opaque LSA into the local LSDB (RFC 5250). Usage: debug ip ospf inject opaque scope <link|area|as> id <opaque-id> [type <128-255>] [hex <body> | tlv <type> <value-hex> ...] [withdraw]. The default Opaque Type is Private-Use so a test LSA never collides with a standards-track consumer. Requires `debug ospf inject enable`.

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
