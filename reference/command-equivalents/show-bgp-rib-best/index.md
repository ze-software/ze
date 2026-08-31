# `show bgp rib best`

Show the winning route for each prefix.

## Ze command

- Registry path: `show bgp rib best`
- Usage: `show bgp rib best`
- Mode: Read-only
- Wire method: `ze-rib-api:best`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `status`
- Answer shape: tab
- Address fields: best-peer, next-hop
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill, resolve, origin
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: `community <value>`: Filter by standard community; `count`: Count matching best paths without serializing rows; `family <value>`: Filter by AFI/SAFI; `first <value>`: Take first N best paths; `graph`: Render AS-path topology graph; `histogram`: Count routes by family and prefix length; `last <value>`: Take last N best paths; `match <value>`: Cross-field structured match; `path <value>`: Filter by AS path; `peer <value>`: Filter by peer; `prefix <value>`: Filter by prefix; `reason`: Explain best-path selection
- Pipe aliases: none

Same filters as 'show bgp rib'. Use '\| reason' to see why each path was selected (local-pref, AS path length, MED, etc.).

## Arguments

No command-specific arguments listed.

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
