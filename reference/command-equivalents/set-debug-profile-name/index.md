# `set debug profile name`

Save the current debug state as a named profile\.

## Ze command

- Registry path: `set debug profile name`
- Usage: `set debug profile name <profile-name>`
- Mode: Daemon
- Wire method: `ze-debug:set-profile-name`
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

It copies what the daemon is writing now into a named slot\. The default slot is left as it is\, so saving a profile does not change what the running daemon logs\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `profile-name` | string | yes | any value of this type | Profile name\. | The slot the current state is saved under\. |

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
