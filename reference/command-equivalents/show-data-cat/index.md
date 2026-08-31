# `show data cat`

Print the raw content of a blob store entry.

## Ze command

- Registry path: `show data cat`
- Usage: `show data cat <key>`
- Mode: Read-only
- Wire method: `ze-show:data-cat`
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

Outputs the value for the given key, like 'cat' for ZeFS.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `key` | string | yes | any value of this type |

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
