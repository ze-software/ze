# `delete debug profile name`

Delete a named debug profile\.

## Ze command

- Registry path: `delete debug profile name`
- Usage: `delete debug profile name <profile-name>`
- Mode: daemon, offline
- Wire method: `ze-debug:delete-profile-name`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

It removes the stored slot and nothing else\. The running daemon keeps whatever it is writing\, so deleting the profile that is live does not turn its output off\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `profile-name` | string | yes | any value of this type | Profile name\. | A stored profile\, as show debug profile lists it\. |

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
