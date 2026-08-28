# `show traceroute [<dest>] [<max-hops>] [<probes>] [<timeout>]`

## Ze command

- Syntax: `show traceroute [<dest>] [<max-hops>] [<probes>] [<timeout>]`
- Registry path: `show traceroute`
- Mode: Read-only
- Wire method: `ze-show:traceroute`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Trace the network path from this router to a target. Shows each hop with its IP and round-trip time. Dest can be an IP or hostname. Defaults: 30 max hops, 3 probes per hop. Increase probes for more reliable RTT measurements.

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
