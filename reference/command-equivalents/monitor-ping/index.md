# `monitor ping`

Continuous ping with live loss and RTT statistics\.

## Ze command

- Registry path: `monitor ping`
- Usage: `monitor ping <destination> [interval <interval>] [timeout <timeout>] [count <count>] [size <size>]`
- Mode: read-only, offline
- Wire method: `ze-ping:monitor-ping`
- Backends: any backend
- Task support: required: the MCP server always answers with a task handle
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: target
- Column order: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill, resolve, origin
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Pings \<target\> until you stop it\. Adjust interval and timeout as needed\. Shows running min\/avg\/max RTT and packet loss\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `destination` | string | yes | any value of this type | Host or IP address to ping\. | The value is the first bare word of the command\, or follows the destination keyword\. A host name is resolved in either address family\, and the family of the answer selects ICMPv4 or ICMPv6\. |
| `interval` | string | no | any value of this type | Time between two echo requests\. | The value is the time between two echo requests\, in Go duration syntax\, 100ms to 30s\. Absent\, the value is 1s\. |
| `timeout` | string | no | any value of this type | Time to wait for each reply\. | The value is the time Ze waits for each reply\, in Go duration syntax\, 1s to 30s\. Absent\, the value is 5s\. |
| `count` | uint | no | any value of this type | Number of echo requests before the stream stops\. | Ze sends this many echo requests and stops\. Absent\, the stream runs until you stop it\, and the one\-shot answer of the API surfaces sends 5\. |
| `size` | uint | no | any value of this type | ICMP echo payload size in bytes\. | Ze fills the payload to exactly this many bytes\, with its marker copied into the front\. Absent\, Ze sends the small default payload\. |

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
