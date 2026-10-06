# `show ping`

Ping a target from the router itself\.

## Ze command

- Registry path: `show ping`
- Usage: `show ping [dest <dest>] [count <count>] [size <size>] [timeout <timeout>] [do-not-fragment <honor-cache\|bypass-cache>]`
- Mode: Read-only
- Wire method: `ze-show:ping`
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

Sends ICMP echo requests to \<dest\> \(IP or hostname\)\. Default count is 5\. Timeout uses Go duration syntax \(e\.g\. 3s\, 500ms\)\. Confirms reachability from this box\, not from your workstation\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `dest` | string | no | any value of this type | Target host or IP address\. | The value is the first bare word of the command\. A host name is resolved in either address family\, and the family of the answer selects ICMPv4 or ICMPv6\. |
| `count` | uint | no | any value of this type | Number of ping packets\. | The value is the number of echo requests Ze sends\, 1 to 100\, paced 10 milliseconds apart\. Absent\, the value is 5\. |
| `size` | uint | no | any value of this type | ICMP echo payload size in bytes \(1\-65507\)\. Omit for the engine default\. | Ze fills the payload to exactly this many bytes\, with its marker copied into the front so a capture still identifies the sender\. |
| `timeout` | string | no | any value of this type | Timeout duration\. | The value is the time Ze waits for each reply\, in Go duration syntax\, 1s to 30s\. Absent\, the value is 5s\. |
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
