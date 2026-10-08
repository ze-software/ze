# `show traceroute`

Trace the network path from this router to a target\.

## Ze command

- Registry path: `show traceroute`
- Usage: `show traceroute [dest <dest>] [max-hops <max-hops>] [timeout <timeout>] [probes <probes>] [do-not-fragment <honor-cache\|bypass-cache>]`
- Mode: Read-only
- Wire method: `ze-traceroute:show-traceroute`
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

Shows each hop with its IP and round\-trip time\. Dest can be an IP or hostname\. Defaults\: 30 max hops\, 3 probes per hop\. Increase probes for more reliable RTT measurements\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `dest` | string | no | any value of this type | Target host or IP address\. | The value is the first bare word of the command\. A host name is resolved in either address family\, and the family of the answer selects ICMPv4 or ICMPv6\. |
| `max-hops` | uint | no | any value of this type | Maximum number of hops\. | The value is the largest time\-to\-live Ze probes\, 1 to 64\. The trace stops early when the target answers\. Absent\, the value is 30\. |
| `timeout` | string | no | any value of this type | Timeout duration\. | The value is the time Ze waits for one probe\, in Go duration syntax\, 1s to 30s\. Absent\, the value is 3s\. |
| `probes` | uint | no | any value of this type | Number of probes per hop\. | The value is the number of probes Ze sends at each time\-to\-live\, 1 to 10\, and each probe reports its own round\-trip time\. Absent\, the value is 3\. |
| `do-not-fragment` | enum | no | `honor-cache`, `bypass-cache` | Set the Don\'t Fragment bit\; honor\-cache obeys the cached path MTU\, bypass\-cache ignores it\. | The keyword takes one of two values\. honor\-cache sets the Don\'t Fragment bit and honors the kernel\'s cached path MTU\: a probe larger than the cached value is refused at send time\, and a router\'s Fragmentation Needed answer updates the cache\. bypass\-cache sets the bit and ignores the cached value\, so the probe is put on the wire at its full size\. Absent\, the kernel fragments a probe larger than the path\, which is the behavior of before the keyword existed\. |

## Mapping intents

### Ping and traceroute diagnostics

Category: Diagnostics

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS
- `ping <target>` (verified, nokia-mdcli)
  - Intent: Ping and traceroute diagnostics

### VyOS
- `ping <target>` (verified, vyos-cli)
  - Intent: Ping and traceroute diagnostics
- `traceroute <target>` (verified, vyos-cli)
  - Intent: Ping and traceroute diagnostics
