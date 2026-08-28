# `show dns cache stats`

## Ze command

- Syntax: `show dns cache stats`
- Registry path: `show dns cache stats`
- Mode: Read-only
- Wire method: `ze-show:dns-cache-stats`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show DNS cache hit, miss, eviction, expiry, and hit-rate counters without changing cache contents.

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
