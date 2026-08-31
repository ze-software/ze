# `show policy chain peer`

Show the import/export filter chain applied to a peer.

## Ze command

- Registry path: `show policy chain peer`
- Usage: `show policy chain peer <selector> [import\|export]`
- Mode: Read-only
- Wire method: `ze-show:policy-chain`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `direction`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

The selector (IP, name, as<N>) and the optional direction are parsed by the handler. Shows the effective chain after group inheritance is resolved. Without a direction keyword, shows both import and export.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `selector` | string | yes | any value of this type |

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
