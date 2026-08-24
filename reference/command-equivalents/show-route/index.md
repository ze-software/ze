# `show route [<limit>] [<prefix>]`

## Ze command

- Syntax: `show route [<limit>] [<prefix>]`
- Registry path: `show route`
- Mode: Read-only
- Wire method: `ze-show:route`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Show the kernel routing table. Lists installed routes with next-hop, interface, protocol, and metric. Pass a CIDR prefix or 'default' to filter, or a route limit to cap the output.

## Mapping intents

### Routing table

Category: Routing

## Vendor equivalents

### Junos MX
- `show route` (verified, junos-route)
  - Intent: Routing table

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show ip route` (verified, vyos-cli)
  - Intent: Routing table
