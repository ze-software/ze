# `show system sockets`

Show open TCP and UDP sockets on this box\.

## Ze command

- Registry path: `show system sockets`
- Usage: `show system sockets [protocol <tcp\|udp>] [state <state>] [port <port>]`
- Mode: Read-only
- Wire method: `ze-cmd:show-system-sockets`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Every filter is optional and they combine\. States use kernel names \(ESTABLISHED\, LISTEN\, TIME\_WAIT\)\. Linux only\. Good for confirming listeners or spotting stuck connections\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `protocol` | enum | no | `tcp`, `udp` | Filter by protocol\. | The word is optional\. Absent\, the list carries both protocols\. Ze reads \/proc\/net\/tcp and \/proc\/net\/tcp6 for tcp\, and the udp pair for udp\. |
| `state` | string | no | any value of this type | Filter by socket state\. | The value is a kernel state name such as established\, listen or time\_wait\, in any letter case\. Only sockets in that state are listed\. A UDP socket has no state to match\. |
| `port` | uint | no | any value of this type | Filter by port number\. | The value is a port number from 1 to 65535\. A socket is listed when its local port or its remote port equals it\. A value outside that range is ignored and the list is not filtered\. |

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
