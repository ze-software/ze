# `explain`

Explain one diagnostic code Ze printed\.

## Ze command

- Registry path: `explain`
- Usage: `explain <code>`
- Mode: Daemon
- Wire method: `ze-explain:explain`
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

The answer gives the meaning of the code\, its likely cause and the recommended fix\. It is read from the diagnostic catalog this binary carries and asks no daemon\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `code` | string | yes | any value of this type | Diagnostic code\. | A diagnostic code as a log line or an error message printed it\. An unknown code is refused\. |

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
