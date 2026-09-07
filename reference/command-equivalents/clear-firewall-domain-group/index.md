# `clear firewall domain-group`

Remove the addresses Ze cached for a domain group.

## Ze command

- Registry path: `clear firewall domain-group`
- Usage: `clear firewall domain-group <name>`
- Mode: Daemon
- Wire method: `ze-clear:firewall-domain-group`
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

Drops the group's addresses from memory and from the persisted cache, then re-applies the firewall tables. Config that still names the group fails to verify until it is resolved again with 'update firewall domain-group <name>'.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `name` | string | yes | any value of this type |

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
