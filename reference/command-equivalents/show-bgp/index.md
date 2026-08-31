# `show bgp`

BGP peers, sessions, RIB, and protocol tools.

## Ze command

- Registry path: `show bgp`
- Usage: `show bgp`
- Mode: Read-only
- Wire method: `ze-bgp:overview`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `decode`, `encode`, `health`, `irr`, `peer`, `rib`
- Answer shape: tab
- Address fields: address
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill, resolve, origin
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: `peers`: The peer rows, without the aggregate fields (`display peers`); `summary`: The aggregate fields, without the peer rows (`display router-id local-as uptime peers-configured peers-established`)

Typed with no subcommand, lists every peer with state, ASN, prefixes received, and uptime. Optionally scope by address family: ipv4, ipv6, or l2vpn.

## Arguments

No command-specific arguments listed.

## Mapping intents

### BGP session summary

Category: BGP

## Vendor equivalents

### Junos MX
- `show bgp summary` (verified, junos-bgp-summary)
  - Intent: BGP session summary

### IOS XR
- `show bgp ipv4 unicast summary` (verified, iosxr-bgp-commands)
  - Intent: BGP session summary
- `show bgp summary` (verified, iosxr-bgp-commands)
  - Intent: BGP session summary

### SR OS
- `show router bgp summary` (verified, nokia-bgp-show)
  - Intent: BGP session summary

### VyOS
- `show bgp ipv4 summary` (verified, vyos-bgp)
  - Intent: BGP session summary
- `show ip bgp summary` (legacy, vyos-bgp)
  - Intent: BGP session summary
