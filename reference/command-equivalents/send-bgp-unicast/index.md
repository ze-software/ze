# `send bgp unicast`

Originate a unicast route on demand\.

## Ze command

- Registry path: `send bgp unicast`
- Usage: `send bgp <selector> unicast <prefix> [next-hop <address>] [community <value> ...] [tag <key> <value>] [for <duration>]`
- Mode: Daemon
- Wire method: `ze-bgp:announce-unicast`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `community`, `for`, `next-hop`, `tag`
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

The address family follows the prefix\, so it is ipv4 unicast or ipv6 unicast\. The ORIGIN is IGP\, and the next hop is this router unless next\-hop names one\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `selector` | string | yes | any value of this type | Peer selector | The value is an IP address\, a peer name\, an AS pattern such as as65001\, a glob\, a comma\-separated list of those\, or \* for every peer\. The message goes to each session the selector matches\. |
| `prefix` | string | yes | any value of this type | Prefix to originate\, in CIDR form | The value is an IPv4 or IPv6 prefix with its length\, such as 192\.0\.2\.0\/24\. The address family of the route follows from the prefix\. A value that is not a prefix is refused\. |

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
