# `show cache`

List the cached BGP UPDATE message IDs.

## Ze command

- Registry path: `show cache`
- Usage: `show cache`
- Mode: Read-only
- Wire method: `ze-bgp:cache-list`
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

The answer carries two keys, 'ids' and 'count'. An ID is listed while its entry is retained, or while that entry has not expired. The IDs come alone, with no retain flag and no consumer state beside them.

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
