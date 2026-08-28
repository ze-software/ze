# `show policy chain peer <selector> [import|export]`

## Ze command

- Syntax: `show policy chain peer <selector> [import\|export]`
- Registry path: `show policy chain peer`
- Mode: Read-only
- Wire method: `ze-show:policy-chain`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show the import/export filter chain applied to a peer. Usage: show policy chain peer <selector> [import|export]. The selector (IP, name, as<N>) and the optional direction are parsed by the handler. Shows the effective chain after group inheritance is resolved. Without a direction keyword, shows both import and export.

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
