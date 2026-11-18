# `delete container image digest`

Delete the stored image a digest names\.

## Ze command

- Registry path: `delete container image digest`
- Usage: `delete container image digest <value>`
- Mode: Daemon
- Wire method: `ze-container:delete-image`
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

The one stored image the digest names is removed\. An image a configured container holds as its current or its previous generation is kept\, and the refusal names the container and the generation\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `value` | string | yes | any value of this type | Digest of the image\. | A digest\, or a prefix of at least 12 hexadecimal digits that names one image\. The word after the digest keyword is the value\. |

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
