# `show dns lookup <hostname> [<type>]`

## Ze command

- Syntax: `show dns lookup <hostname> [<type>]`
- Registry path: `show dns lookup`
- Mode: Read-only
- Wire method: `ze-show:dns-lookup`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Look up a DNS name from the router. Resolves <hostname> using the daemon's DNS resolver (falls back to the system resolver if no DNS component is configured). Default type is A. Returns records, TTL, and query time. Supports A, AAAA, MX, NS, TXT, CNAME, and PTR.

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
