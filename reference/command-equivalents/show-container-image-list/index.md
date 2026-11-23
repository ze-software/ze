# `show container image list`

Show the images in the store and the fetches in flight\.

## Ze command

- Registry path: `show container image list`
- Usage: `show container image list`
- Mode: Read\-only
- Wire method: `ze-container:show-image-list`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: none
- Column order: reference\, digest\, size\, fetched\, state\, received\, used\-by\, error\, code
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save\, match\, count\, first\, last\, display\, fill
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

One row for each image in the store\, each fetch in flight and each fetch that failed\: the whole reference\, the digest\, the size and the fetch time\, the state \(present\, fetching\, failed or previous\)\, the bytes a fetch received so far\, and the containers that hold the image\. A failed row carries the error and its doctor code\.

## Arguments

No command-specific arguments listed.

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
