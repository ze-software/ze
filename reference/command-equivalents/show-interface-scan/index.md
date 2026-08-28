# `show interface scan`

## Ze command

- Syntax: `show interface scan`
- Registry path: `show interface scan`
- Mode: Read-only
- Wire method: `ze-show:interface-scan`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Discover and classify all OS interfaces. Returns name, Ze type (ethernet, bridge, vxlan, etc.), and MAC for each interface found. Pipe to table, yaml, or json for different views. Useful during initial setup to see what the box has.

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
