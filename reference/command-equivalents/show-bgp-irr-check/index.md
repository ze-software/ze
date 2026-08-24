# `show bgp irr check <peer> <prefix>`

## Ze command

- Syntax: `show bgp irr check <peer> <prefix>`
- Registry path: `show bgp irr check`
- Mode: Read-only
- Wire method: `ze-show:irr-check`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill, resolve, origin

Check if a prefix is accepted by the IRR filter. Usage: show bgp irr check <peer> <prefix>. Reports whether the prefix would be accepted or rejected, and which entry matches.

## Mapping intents

### RPKI cache and validation session state

Category: BGP

Current Ze commands are IRR-focused. RPKI-specific show paths can be added to this entry when present in the live command catalog.

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
