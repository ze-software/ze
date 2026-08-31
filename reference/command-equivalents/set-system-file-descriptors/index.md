# `set system file-descriptors`

Raise the file descriptor limit for the daemon process.

## Ze command

- Registry path: `set system file-descriptors`
- Usage: `set system file-descriptors [limit <limit\|max>]`
- Mode: Daemon
- Wire method: `ze-set:system-file-descriptors`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Pass a number or 'max' to go to the hard limit. Takes effect immediately. Check current limits with 'show system file-descriptors'.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `limit` | union | no | `max` |

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
