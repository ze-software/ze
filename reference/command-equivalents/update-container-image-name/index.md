# `update container image name`

Fetch the named container\'s image again and move it to a new digest\.

## Ze command

- Registry path: `update container image name`
- Usage: `update container image name <value>`
- Mode: Daemon
- Wire method: `ze-container:update-image`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
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

The reference the container\'s configuration names is fetched again\. When it resolves to another digest\, the container restarts on it with an empty writable layer\, and the image it ran stays as its previous generation\. When the digest is the same\, nothing changes\.

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
