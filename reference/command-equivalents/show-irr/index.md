# `show irr`

Show the stored IRR datasets and their refresh schedule\.

## Ze command

- Registry path: `show irr`
- Usage: `show irr`
- Mode: Read\-only
- Wire method: `ze-irr:show-irr`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: none
- Column order: as\-set\, server\, status\, ipv4\-count\, ipv6\-count\, refreshed\-at\, checked\-at\, stale\-since\, next\-refresh\, generation\, consumers
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save\, match\, count\, first\, last\, display\, fill
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

One row per stored AS\-SET\: the server it came from\, the prefix counts\, when the server last answered\, when the next refresh is due\, and the consumers that enforce it\. With refresh\-interval 0 the schedule reads manual\.

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
