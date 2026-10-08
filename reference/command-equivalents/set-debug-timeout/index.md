# `set debug timeout`

Set how long debug output stays enabled\.

## Ze command

- Registry path: `set debug timeout`
- Usage: `set debug timeout <duration>`
- Mode: Daemon
- Wire method: `ze-debug:set-timeout`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: none
- Pipes, when the answer has rows: none
- Pipes, while streaming: none
- Pipes, local process only: none
- Command pipes: none
- Pipe aliases: none

The duration is written as 30m\, 1h or 90s\. Seconds are rounded up to minutes\, the longest accepted value is 24h\, and zero disables the timer\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `duration` | string | yes | any value of this type | How long debug stays enabled\. | A duration such as 30m\, 1h\, 90s or 0\. |

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
