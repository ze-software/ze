# `send bgp blackhole`

Originate a blackhole route on demand\.

## Ze command

- Registry path: `send bgp blackhole`
- Usage: `send bgp <selector> blackhole <prefix> [tag <key> <value>] [for <duration>]`
- Mode: Daemon
- Wire method: `ze-bgp:announce-blackhole`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `for`, `tag`
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

This command attaches the BLACKHOLE community itself\, so RFC 7999 Section 3\.1 narrows the fan\-out to the sessions that recorded the agreement\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `selector` | string | yes | any value of this type | Peer selector | The value is an IP address\, a peer name\, an AS pattern such as as65001\, a glob\, a comma\-separated list of those\, or \* for every peer\. The message goes to each session the selector matches\. |
| `prefix` | string | yes | any value of this type | Prefix to blackhole\, in CIDR form | The value is an IPv4 or IPv6 prefix with its length\, such as 192\.0\.2\.1\/32\. The address family of the route follows from the prefix\. A value that is not a prefix is refused\. |

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
