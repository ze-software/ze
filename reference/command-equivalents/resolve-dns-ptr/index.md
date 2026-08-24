# `resolve dns ptr <ip-address>`

## Ze command

- Syntax: `resolve dns ptr <ip-address>`
- Registry path: `resolve dns ptr`
- Mode: Read-only
- Wire method: `ze-resolve:dns-ptr`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Reverse-lookup an IP address to its hostname (PTR). Usage: resolve dns ptr <ip-address>.

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
