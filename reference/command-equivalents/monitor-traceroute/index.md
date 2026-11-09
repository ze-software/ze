# `monitor traceroute`

Live mtr\-style traceroute that updates continuously\.

## Ze command

- Registry path: `monitor traceroute`
- Usage: `monitor traceroute <destination> [max-hops <max-hops>] [timeout <timeout>] [probes <probes>] [do-not-fragment <honor-cache\|bypass-cache>]`
- Mode: read-only, offline
- Wire method: `ze-traceroute:monitor-traceroute`
- Backends: any backend
- Task support: required: the MCP server always answers with a task handle
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: addr
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill, resolve, origin
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Shows each hop with running RTT statistics\. Keeps probing so you can watch path changes and latency shifts over time\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `destination` | string | yes | any value of this type | Host or IP address to trace\. | The value is the first bare word of the command\, or follows the destination keyword\. A host name is resolved in either address family\, and the family of the answer selects ICMPv4 or ICMPv6\. |
| `max-hops` | uint | no | any value of this type | Maximum number of hops\. | The value is the largest time\-to\-live the stream probes\. Absent\, the stream probes 30 hops\, and the one\-shot round of the API surfaces probes 16\. |
| `timeout` | string | no | any value of this type | Timeout duration\, accepted and not read\. | The value is accepted in Go duration syntax\, 1s to 30s\, and then not read\: each probe round waits one second for every answer\. |
| `probes` | uint | no | any value of this type | Number of probes per hop\, accepted and not read\. | The value is accepted in the range 1 to 10 and then not read\: a probe round sends one probe at each time\-to\-live\. |
| `do-not-fragment` | enum | no | `honor-cache`, `bypass-cache` | Don\'t Fragment mode\, accepted and not read\. | The keyword takes one of two values\, honor\-cache or bypass\-cache\, as on show traceroute\, and then neither is read\: a probe round sends every probe with the Don\'t Fragment bit off\. |

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
