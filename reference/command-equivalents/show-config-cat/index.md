# `show config cat`

Print the full text of a stored configuration snapshot.

## Ze command

- Registry path: `show config cat`
- Usage: `show config cat <id>`
- Mode: Read-only
- Wire method: `ze-show:config-cat`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: none
- Pipes, when the answer has rows: none
- Pipes, while streaming: none
- Pipes, local process only: none
- Command pipes: none
- Pipe aliases: none

Outputs the config as-is.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `id` | string | yes | any value of this type |

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
