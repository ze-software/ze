# `clear container layer name`

Empty the named container\'s writable layer\.

## Ze command

- Registry path: `clear container layer name`
- Usage: `clear container layer name <value>`
- Mode: Daemon
- Wire method: `ze-container:clear-layer`
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

The container\'s writable layer is emptied\. A running container is stopped first and started again on the empty layer\. The name of a container the configuration no longer holds is accepted\, and its stored layer is removed\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `value` | string | yes | any value of this type | Name of the container\. | The name of a container\, configured or not\. The word after the name keyword is the value\. |

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
