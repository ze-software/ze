# `show route lookup`

Look up which route the kernel would use for a given IP.

## Ze command

- Registry path: `show route lookup`
- Usage: `show route lookup <ip>`
- Mode: Read-only
- Wire method: `ze-show:route-lookup`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Performs a longest-prefix-match and returns the matching route with gateway, interface, protocol, and metric.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `ip` | string | yes | any value of this type |

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
