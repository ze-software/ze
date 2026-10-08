# `resolve ping`

Ping from the router with optional source binding\.

## Ze command

- Registry path: `resolve ping`
- Usage: `resolve ping <target> [source <source>] [count <count>] [size <size>] [do-not-fragment <honor-cache\|bypass-cache>]`
- Mode: Read-only
- Wire method: `ze-ping:resolve-ping`
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

The target takes a hostname or an address\, and it accepts a zone suffix such as fe80\:\:1\%eth0\. The source binds a local address and takes the same zone suffix\. The count runs from 1 to 100 and the payload size from 1 to 65507 bytes\. \'show ping\' declares no source leaf and puts no upper bound on its count\. The do\-not\-fragment keyword is the same word\, with the same two values\, on both\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `target` | string | yes | any value of this type | Host or IP address to ping\. | The value follows the target keyword\. A host name is resolved in either address family\, and the family of the answer selects ICMPv4 or ICMPv6\. |
| `source` | string | no | any value of this type | Source IP address to bind\. | The value is a local IPv4 or IPv6 address\, with an optional \%zone\, that Ze binds the echo socket to\. Absent\, the kernel picks the source\. |
| `count` | uint | no | any value of this type | Number of echo requests\. | Ze sends this many echo requests and waits 5 seconds for each reply\. Absent\, the value is 4\. |
| `size` | uint | no | any value of this type | ICMP echo payload size in bytes\. | Ze fills the payload to exactly this many bytes\, with its marker copied into the front\. Absent\, Ze sends the small default payload\. |
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
