# `resolve dns a <hostname>`

## Ze command

- Syntax: `resolve dns a <hostname>`
- Registry path: `resolve dns a`
- Mode: Read-only
- Wire method: `ze-resolve:dns-a`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Look up IPv4 addresses (A records) for a hostname. Usage: resolve dns a <hostname>.

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
