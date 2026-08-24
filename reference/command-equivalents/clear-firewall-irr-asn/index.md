# `clear firewall irr asn <asn>`

## Ze command

- Syntax: `clear firewall irr asn <asn>`
- Registry path: `clear firewall irr asn`
- Mode: Daemon
- Wire method: `ze-clear:firewall-irr-asn`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill

Remove the cached IRR prefix-list for an ASN. Usage: clear firewall irr asn <asn>. Drops the entry from memory and from the persisted cache, then re-applies the firewall tables. Config that still references the ASN fails to verify until it is fetched again with 'update firewall irr asn <asn>'.

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
