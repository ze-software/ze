# `show bgp rpki`

Show RPKI validation counters with one row for each cache server

## Ze command

- Registry path: `show bgp rpki`
- Usage: `show bgp rpki`
- Mode: Read-only
- Wire method: `not listed`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: address
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill, resolve, origin
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: `summary`: The validation counters, without the cache server rows (`display vrp-count validation-enabled sessions-total sessions-established sessions-synced aspa-enabled aspa-records`)

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
