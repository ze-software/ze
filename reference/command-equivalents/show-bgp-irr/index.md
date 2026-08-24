# `show bgp irr`

## Ze command

- Syntax: `show bgp irr`
- Registry path: `show bgp irr`
- Mode: Read-only
- Wire method: `ze-show:irr-status`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill
- Pipes, on rows: none

Show IRR filter status per ASN. Lists each enrolled ASN with its resolved AS-SET, prefix counts, last refresh time, and error status. Use this to confirm that IRR prefix-lists are loaded and current.

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
