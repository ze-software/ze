# `send bgp flowspec`

Originate a FlowSpec rule on demand \(RFC 8955\)\.

## Ze command

- Registry path: `send bgp flowspec`
- Usage: `send bgp <selector> flowspec [destination-ipv4 <prefix> ...] [destination-ipv6 <prefix> ...] [destination-port <value> ...] [dscp <value> ...] [flow-label <value> ...] [fragment <value> ...] [icmp-code <value> ...] [icmp-type <value> ...] [next-header <value> ...] [packet-length <value> ...] [port <value> ...] [protocol <value> ...] [rd <value>] [source-ipv4 <prefix> ...] [source-ipv6 <prefix> ...] [source-port <value> ...] [tcp-flags <value> ...] [traffic-class <value> ...] (community <value>\|rate-limit <bytes-per-second>\|discard) [tag <key> <value>] [for <duration>]`
- Mode: Daemon
- Wire method: `ze-bgp:announce-flowspec`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `action`, `destination-ipv4`, `destination-ipv6`, `destination-port`, `dscp`, `flow-label`, `for`, `fragment`, `icmp-code`, `icmp-type`, `next-header`, `packet-length`, `port`, `protocol`, `rd`, `source-ipv4`, `source-ipv6`, `source-port`, `tag`, `tcp-flags`, `traffic-class`
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

The match components come from ze\-flowspec\-cmd\, which augments this container\. They are the FlowSpec codec vocabulary\, and the plugin that owns it declares them\, so this module never restates another plugin\'s words\. An augmented container carries no declaration order\, so the components sort by name and land in front of the action and the options declared here\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `selector` | string | yes | any value of this type | Peer selector | The value is an IP address\, a peer name\, an AS pattern such as as65001\, a glob\, a comma\-separated list of those\, or \* for every peer\. The message goes to each session the selector matches\. |

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
