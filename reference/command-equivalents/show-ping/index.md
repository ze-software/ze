# `show ping`

Ping a target from the router itself.

## Ze command

- Registry path: `show ping`
- Usage: `show ping [dest <dest>] [count <count>] [size <size>] [timeout <timeout>]`
- Mode: Read-only
- Wire method: `ze-show:ping`
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

Sends ICMP echo requests to <dest> (IP or hostname). Default count is 5. Timeout uses Go duration syntax (e.g. 3s, 500ms). Confirms reachability from this box, not from your workstation.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `dest` | string | no | any value of this type |
| `count` | uint | no | any value of this type |
| `size` | uint | no | any value of this type |
| `timeout` | string | no | any value of this type |

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
