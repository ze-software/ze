# `show mtu`

Measure the path MTU and size the IPsec tunnels from it\.

## Ze command

- Registry path: `show mtu`
- Usage: `show mtu [host <host>] [search <exhaustive>] [view <detail>]`
- Mode: Read-only
- Wire method: `ze-show:mtu`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: doc
- Address fields: none
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Measures the path MTU to every configured IPsec peer and to the reference address\, derives the ESP ceiling of each tunnel from its negotiated transform\, and reports whether each tunnel interface is oversized\, tight\, under\-utilized or correct\, with the configuration commands that fix it\. The run changes nothing on the router\. With host \<address\>\, one address is measured instead of the peers\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `host` | union | no | any value of this type | Measure one address instead of the IPsec peers\. | The value follows the host keyword and is one IPv4 or IPv6 address\. Ze measures the path MTU to that address alone and skips the peers and the reference address\. A name is refused\: the run sends probes to the address as typed\. |
| `search` | enum | no | `exhaustive` | exhaustive\: a slower run that probes every size and discards the cached and reported path MTU\. | The bare word exhaustive makes the run slower and its figure comes from probing every size on the wire\: the path MTU the kernel remembered and the value a router reported are both discarded\, so a stale or poisoned cache value cannot reach the answer\. |
| `view` | enum | no | `detail` | detail\: report every probe sent and every reply received\. | The bare word detail adds\, per target\, the size of every probe sent and the answer each one received\, so an operator can follow how the value was found\. |

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
