# `show interface name <name> detail`

## Ze command

- Syntax: `show interface name <name> detail`
- Registry path: `show interface name detail`
- Mode: Read-only
- Wire method: `ze-show:interface-detail`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show full detail for one interface. Usage: show interface name <name> detail.

## Mapping intents

### Detailed interface state

Category: Interfaces

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show interfaces <name>` (verified, vyos-cli)
  - Intent: Detailed interface state
