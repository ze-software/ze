# `clear firewall irr as-set`

Remove the cached IRR prefix-list for an AS-SET.

## Ze command

- Registry path: `clear firewall irr as-set`
- Usage: `clear firewall irr as-set <as-set>`
- Mode: Daemon
- Wire method: `ze-clear:firewall-irr-as-set`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Drops the entry from memory and from the persisted cache, then re-applies the firewall tables.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `as-set` | string | yes | any value of this type |

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
