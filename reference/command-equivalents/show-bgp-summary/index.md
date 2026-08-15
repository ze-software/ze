# `show bgp summary`

## Ze command

- Syntax: `show bgp summary`
- Registry path: `show bgp summary`
- Mode: Read-only
- Wire method: `ze-bgp:summary`
- Global pipes: yes

Show a one-line-per-peer BGP summary. Lists every peer with state, ASN, prefixes received, and uptime. Optionally scope by address family: ipv4, ipv6, or l2vpn.

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
