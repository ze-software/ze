# `show bgp rib best`

## Ze command

- Syntax: `show bgp rib best`
- Registry path: `show bgp rib best`
- Mode: Read-only
- Wire method: `ze-rib-api:best`
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill, resolve, origin

Show the winning route for each prefix. Same filters as 'show bgp rib'. Use '| reason' to see why each path was selected (local-pref, AS path length, MED, etc.).

## Mapping intents

### Best BGP routes

Category: BGP

### BGP route for a prefix

Category: BGP

Ze narrows the best RIB with command-specific pipe filters.

## Vendor equivalents

### Junos MX
- `show route <prefix> protocol bgp` (verified, junos-route)
  - Intent: BGP route for a prefix
- `show route protocol bgp` (verified, junos-route)
  - Intent: Best BGP routes

### IOS XR
- `show bgp ipv4 unicast` (verified, iosxr-bgp-commands)
  - Intent: Best BGP routes
- `show bgp ipv4 unicast <prefix>` (verified, iosxr-bgp-commands)
  - Intent: BGP route for a prefix

### SR OS

No equivalent listed.

### VyOS
- `show ip bgp <prefix>` (legacy, vyos-bgp)
  - Intent: BGP route for a prefix
