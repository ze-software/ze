# `show ospf route fast-reroute`

## Ze command

- Syntax: `show ospf route fast-reroute`
- Registry path: `show ospf route fast-reroute`
- Mode: Read-only
- Wire method: `ze-show:ospf-route-fast-reroute`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Show OSPF fast-reroute (LFA / TI-LFA) backups (RFC 5286). Lists each prefix's primary next-hops with their pre-computed loop-free backup, protection class (node/link/downstream), and TI-LFA repair label stack. Unprotected primaries are shown as unprotected.

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
