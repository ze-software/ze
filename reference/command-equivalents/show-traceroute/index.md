# `show traceroute`

Trace the network path from this router to a target.

## Ze command

- Registry path: `show traceroute`
- Usage: `show traceroute [dest <dest>] [max-hops <max-hops>] [timeout <timeout>] [probes <probes>]`
- Mode: Read-only
- Wire method: `ze-show:traceroute`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Shows each hop with its IP and round-trip time. Dest can be an IP or hostname. Defaults: 30 max hops, 3 probes per hop. Increase probes for more reliable RTT measurements.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `dest` | string | no | any value of this type |
| `max-hops` | uint | no | any value of this type |
| `timeout` | string | no | any value of this type |
| `probes` | uint | no | any value of this type |

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
