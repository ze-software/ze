# `show dns-resolver`

LAN DNS resolver status\, cache and counters\.

## Ze command

- Registry path: `show dns-resolver`
- Usage: `show dns-resolver`
- Mode: Read-only
- Wire method: `ze-dnsresolver:show-dns-resolver`
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

Enabled\, mode\, listeners\, allow\-from count\, upstreams\, limits\, the answer and negative cache entry counts\, cache hits\, misses and hit rate\, the queries refused \(by reason\)\, rate\-limited and dropped\, and the upstream exchanges by result\.

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
