# `request interface migrate`

## Ze command

- Syntax: `request interface migrate`
- Registry path: `request interface migrate`
- Mode: Daemon
- Wire method: `ze-iface:interface-migrate`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Move IP addresses between interfaces with minimal downtime. Takes a source interface, a target interface, and the address to move. Adds addresses to the target before removing them from the source (make-before-break).

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
