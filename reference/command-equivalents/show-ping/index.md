# `show ping [<count>] [<dest>] [<size>] [<timeout>]`

## Ze command

- Syntax: `show ping [<count>] [<dest>] [<size>] [<timeout>]`
- Registry path: `show ping`
- Mode: Read-only
- Wire method: `ze-show:ping`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Ping a target from the router itself. Sends ICMP echo requests to <dest> (IP or hostname). Default count is 5. Timeout uses Go duration syntax (e.g. 3s, 500ms). Confirms reachability from this box, not from your workstation.

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
