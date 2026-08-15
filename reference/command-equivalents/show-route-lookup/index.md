# `show route lookup`

## Ze command

- Syntax: `show route lookup`
- Registry path: `show route lookup`
- Mode: Read-only
- Wire method: `ze-show:route-lookup`
- Global pipes: yes

Look up which route the kernel would use for a given IP. Performs a longest-prefix-match and returns the matching route with gateway, interface, protocol, and metric. Usage: show route lookup <ip>.

## Mapping intents

### Route lookup for a prefix or address

Category: Routing

## Vendor equivalents

### Junos MX
- `show route <prefix>` (verified, junos-route)
  - Intent: Route lookup for a prefix or address

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show ip route <prefix>` (verified, vyos-cli)
  - Intent: Route lookup for a prefix or address
