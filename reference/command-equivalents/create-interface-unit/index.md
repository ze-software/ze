# `create interface unit <parent> <vid>`

## Ze command

- Syntax: `create interface unit <parent> <vid>`
- Registry path: `create interface unit`
- Mode: Daemon
- Wire method: `ze-iface:interface-unit-add`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Add a VLAN sub-interface (802.1Q tagged). Usage: create interface unit <parent> <vid>. Parent must already exist.

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
