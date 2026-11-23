# `update container image rollback name`

Return the named container to its previous image and its writes\.

## Ze command

- Registry path: `update container image rollback name`
- Usage: `update container image rollback name <value>`
- Mode: Daemon
- Wire method: `ze-container:update-image-rollback`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: doc
- Address fields: none
- Column order: none
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

The container\'s current and previous generations swap\, and the container restarts on the image it ran before\, with the writes it had\. Ze never rolls back on its own\. A container with no previous generation is refused\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `value` | string | yes | any value of this type | Name of the container\. | The name of a configured container\, as its instance entry spells it\. The word after the name keyword is the value\. |

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
