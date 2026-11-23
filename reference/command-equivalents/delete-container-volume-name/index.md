# `delete container volume name`

Delete a stored volume the configuration no longer declares\.

## Ze command

- Registry path: `delete container volume name`
- Usage: `delete container volume name <value> <instance>`
- Mode: Daemon
- Wire method: `ze-container:delete-volume`
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

The volume\'s folder and everything the container wrote in it are removed\. A volume the configuration still declares is refused\, and the refusal names the statement to remove\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `value` | string | yes | any value of this type | Name of the volume\. | The name of the volume\, as the container\'s volume entry spelled it\. The word after the name keyword is the value\. |
| `instance` | string | yes | any value of this type | Name of the container\. | The container the volume belongs to\: two containers may each hold a volume of the same name\. |

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
