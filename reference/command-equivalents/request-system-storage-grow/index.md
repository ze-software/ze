# `request system storage grow`

Grow the persistent storage to the size of its disk\.

## Ze command

- Registry path: `request system storage grow`
- Usage: `request system storage grow`
- Mode: Daemon
- Wire method: `ze-host:request-system-storage-grow`
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

Extends the persistent partition to the end of its disk and grows its filesystem to the partition\, with no reboot\. The answer carries the sizes before and after\. A system whose storage already uses the whole disk gets that answer and nothing is written\. Read the current sizes with \'show host storage\'\.

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
