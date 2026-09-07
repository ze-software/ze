# `show dns cache stats`

Show the DNS cache hit, miss, eviction, expiry, and hit-rate counters.

## Ze command

- Registry path: `show dns cache stats`
- Usage: `show dns cache stats`
- Mode: Read-only
- Wire method: `ze-show:dns-cache-stats`
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

It reads the counters and does not change the cache contents.

## Arguments

No command-specific arguments listed.

## Mapping intents

### DNS lookup and cache inspection

Category: Diagnostics

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show dns` (verified, vyos-cli)
  - Intent: DNS lookup and cache inspection
