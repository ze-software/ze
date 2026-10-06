# `request peer pause`

Pause reading from a peer\'s TCP socket\.

## Ze command

- Registry path: `request peer pause`
- Usage: `request peer <selector> pause`
- Mode: Daemon
- Wire method: `ze-bgp:peer-pause`
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

Ze stops reading the peer\'s socket and the session stays up\, so nothing is withdrawn\. The selector resolves to one peer and a wildcard is refused\, because flow control acts on one read loop\. \'request peer \<selector\> resume\' restarts it\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `selector` | string | yes | any value of this type | Peer selector | The value is an IP address\, a peer name\, an AS pattern such as as65001\, a glob\, or \* for every peer\. \'pause\' and \'resume\' refuse a selector that matches more than one peer\, and the other commands act on each peer it matches\. |

## Mapping intents

### Pause or resume a BGP peer without deleting configuration

Category: BGP

Ze exposes explicit runtime flow control; most vendors use config disable or administrative tools.

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
