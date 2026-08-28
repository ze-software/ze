# `show bgp irr prefix <peer>`

## Ze command

- Syntax: `show bgp irr prefix <peer>`
- Registry path: `show bgp irr prefix`
- Mode: Read-only
- Wire method: `ze-show:irr-prefix`
- Answer shape: map
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display
- Pipes, on rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show IRR-resolved prefixes for a peer. Usage: show bgp irr prefix <peer>. Lists all IPv4 and IPv6 prefixes in the IRR-resolved prefix-list for the given peer address.

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
