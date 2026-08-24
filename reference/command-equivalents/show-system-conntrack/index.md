# `show system conntrack`

## Ze command

- Syntax: `show system conntrack`
- Registry path: `show system conntrack`
- Mode: Read-only
- Wire method: `ze-show:system-conntrack`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Show the kernel connection tracking table. Returns conntrack entry count, table size, timeouts, and loaded modules. Requires the nft backend. Check this when you suspect conntrack table exhaustion is dropping traffic.

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
