# `debug ipv6 ospf inject lsa scope <link|area|as> type <ls-type>`

## Ze command

- Syntax: `debug ipv6 ospf inject lsa scope <link\|area\|as> type <ls-type>`
- Registry path: `debug ipv6 ospf inject lsa`
- Mode: Daemon
- Wire method: `ze-debug:ospfv3-inject`
- Global pipes: yes

Inject a crafted OSPFv3 LSA into the local LSDB (RFC 5340). Usage: debug ipv6 ospf inject lsa scope <link|area|as> type <ls-type> id <link-state-id> [hex <body>] [withdraw]. The flooding scope is derived from the LS Type S2/S1 bits (a reserved scope is rejected). Requires `debug ospf inject enable`.

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
