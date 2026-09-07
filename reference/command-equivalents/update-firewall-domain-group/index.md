# `update firewall domain-group`

Resolve a domain group's DNS names now and program its set.

## Ze command

- Registry path: `update firewall domain-group`
- Usage: `update firewall domain-group <name>`
- Mode: Daemon
- Wire method: `ze-update:firewall-domain-group`
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

Asks for each name in the group at once rather than waiting for its TTL. A name that fails to resolve keeps the addresses Ze already had for it, and only a name that answers NXDOMAIN empties its own contribution.

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
