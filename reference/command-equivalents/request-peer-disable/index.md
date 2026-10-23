# `request peer disable`

Hold a peer administratively down\.

## Ze command

- Registry path: `request peer disable`
- Usage: `request peer <selector> disable [message <message>]`
- Mode: Daemon
- Wire method: `ze-bgp:peer-disable`
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

Ze closes a running session with Cease subcode 2\, Administrative Shutdown\, and opens none until \'request peer \<selector\> enable\'\. The words after disable are the RFC 9003 Shutdown Communication\, and the answer echoes the text that fits on the wire\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `selector` | string | yes | any value of this type | \* for every peer\, or an address\, a peer name or an AS pattern | The value is an IP address\, a peer name\, an AS pattern such as as65001\, a glob\, or \* for every peer\. \'pause\' and \'resume\' refuse a selector that matches more than one peer\, and the other commands act on each peer it matches\. |
| `message` | string | no | any value of this type | Shutdown communication | Every word after disable\, joined by one space\. Absent\, the Cease carries no communication\. |

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
