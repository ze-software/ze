# `show container list`

Show every configured container\, its state and the image it runs\.

## Ze command

- Registry path: `show container list`
- Usage: `show container list`
- Mode: Read\-only
- Wire method: `ze-container:show-list`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: none
- Column order: name\, state\, reason\, image\, digest\, since
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save\, match\, count\, first\, last\, display\, fill
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

One row for each configured container\: its name\, its state \(running\, waiting\-image\, restarting\, stopped\, disabled or failed\)\, a reason sentence\, the whole reference its image leaf completes to\, and\, while it runs\, the digest of the image it runs and when its entrypoint was released\. A container waiting for its image names the reference and why the store does not hold it\: never fetched\, a fetch running\, a fetch that failed\, or an image the store lost\. A failed container names the step that failed\.

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
