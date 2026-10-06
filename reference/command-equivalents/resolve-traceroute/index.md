# `resolve traceroute`

Traceroute from the router with optional source binding\.

## Ze command

- Registry path: `resolve traceroute`
- Usage: `resolve traceroute <target> [source <source>] [max-hops <max-hops>] [timeout <timeout>] [probes <probes>] [do-not-fragment <honor-cache\|bypass-cache>]`
- Mode: Read-only
- Wire method: `ze-resolve:traceroute`
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

The options are source\, max\-hops\, timeout and probes\, each one a keyword and a value\, and do\-not\-fragment\, a keyword with one of two values\. max\-hops is 1 to 64 and defaults to 30\, probes is 1 to 10 and defaults to 3\, and timeout is 1s to 30s and defaults to 3s\. The source also selects the address family\: a target name is resolved in the family of the source\, and a target with no address in that family is refused\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `target` | string | yes | any value of this type | Host or IP address to trace to\. | The value follows the target keyword\. A host name is resolved in the family of the source\, or in either family when no source is given\, and a name with no address in that family is refused\. |
| `source` | string | no | any value of this type | Source IP address to bind\, whose family also selects the family the target is resolved in\. | The value is a local IPv4 or IPv6 address\, with an optional \%zone\, that Ze binds the probe socket to\. Absent\, the kernel picks the source\. |
| `max-hops` | uint | no | any value of this type | Largest time\-to\-live to probe\. | The trace stops early when the target answers\. Absent\, the value is 30\. |
| `timeout` | string | no | any value of this type | Per\-probe timeout\, 1s to 30s\, in Go duration syntax\. | The value is the time Ze waits for one probe before it records a missing answer\. Absent\, the value is 3s\. |
| `probes` | uint | no | any value of this type | Number of probes per hop\. | Each probe at one time\-to\-live reports its own round\-trip time\. Absent\, the value is 3\. |
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
