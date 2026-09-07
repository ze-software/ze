# `resolve traceroute`

Traceroute from the router with optional source binding.

## Ze command

- Registry path: `resolve traceroute`
- Usage: `resolve traceroute <target> [source <source>] [max-hops <max-hops>] [timeout <timeout>] [probes <probes>]`
- Mode: Read-only
- Wire method: `ze-resolve:traceroute`
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

The options are source, max-hops, timeout and probes, each one a keyword and a value. max-hops is 1 to 64 and defaults to 30, probes is 1 to 10 and defaults to 3, and timeout is 1s to 30s and defaults to 3s. The source also selects the address family: a target name is resolved in the family of the source, and a target with no address in that family is refused.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `target` | string | yes | any value of this type |
| `source` | string | no | any value of this type |
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
