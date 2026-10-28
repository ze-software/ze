# `clear dns-resolver cache all`

Empty every LAN DNS resolver cache\.

## Ze command

- Registry path: `clear dns-resolver cache all`
- Usage: `clear dns-resolver cache all`
- Mode: Daemon
- Wire method: `ze-dnsresolver:clear-dns-resolver-cache`
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

Empties the answer and the negative cache\, so the next query for any name goes upstream\. The counters are kept\.

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
