# `create container image reference`

Fetch the named image into the store\.

## Ze command

- Registry path: `create container image reference`
- Usage: `create container image reference <value>`
- Mode: Daemon
- Wire method: `ze-container:create-image`
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

The image is fetched from its registry\, each byte checked against its digest and size\, and stored in one folder named by its digest\. A name with no registry host is fetched from the default registry\, and a name with no tag means the tag latest\. The command answers once\, at the end\; the image listing shows the fetch meanwhile\. Every container that waited for the image then starts\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `value` | string | yes | any value of this type | Image reference\. | The image reference\: a name\, optionally with a registry host first and a tag or a digest last\. The word after the reference keyword is the value\. |

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
