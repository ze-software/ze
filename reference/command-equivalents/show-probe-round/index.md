# `show probe-round`

Run a parallel traceroute probe round to a target\.

## Ze command

- Registry path: `show probe-round`
- Usage: `show probe-round [dest <dest>] [probes <probes>] [max-hops <max-hops>] [timeout <timeout>]`
- Mode: Read-only
- Wire method: `ze-show:probe-round`
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

Sends all probes concurrently for faster results than sequential traceroute\. Returns per\-hop RTT and IP\. Use probes and max\-hops to tune accuracy vs speed\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `dest` | string | no | any value of this type | Target host or IP address\. | The value is the first bare word of the command\. A host name is resolved in either address family\, and the family of the answer selects ICMPv4 or ICMPv6\. |
| `probes` | uint | no | any value of this type | Number of probes per hop\. | The value is accepted in the range 1 to 10 and then not read\: a probe round sends one probe at each time\-to\-live\. |
| `max-hops` | uint | no | any value of this type | Maximum number of hops\. | The value is the largest time\-to\-live Ze probes\, 1 to 64\. Absent\, or set to 30\, the round probes 16 hops\. |
| `timeout` | string | no | any value of this type | Timeout duration\. | The value is accepted in Go duration syntax\, 1s to 30s\, and then not read\: a probe round waits one second for every answer\. |

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
