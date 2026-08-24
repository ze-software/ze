# `show dns cache list`

## Ze command

- Syntax: `show dns cache list`
- Registry path: `show dns cache list`
- Mode: Read-only
- Wire method: `ze-show:dns-cache-list`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

List all non-expired DNS cache entries, sorted by shortest TTL first.

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
