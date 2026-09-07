# `show firewall domain-group`

Show what each configured domain group's DNS names resolve to.

## Ze command

- Registry path: `show firewall domain-group`
- Usage: `show firewall domain-group [name <name>]`
- Mode: Read-only
- Wire method: `ze-show:firewall-domain-group-status`
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

Lists each group with its names, the addresses Ze is enforcing for them, when each name last resolved, and what the last answer said. Read this before a commit to confirm a group has data.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `name` | string | no | any value of this type |

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
