# `update serve start`

Start the update server and answer its listen address\.

## Ze command

- Registry path: `update serve start`
- Usage: `update serve start [listen <listen>]`
- Mode: Daemon
- Wire method: `ze-update:update-serve-start`
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

The answer names the address the server listens on\, the bound port included when the listen leaf asked for port 0\. A second start while one server runs is refused\: stop it first with \'update serve stop\'\. Meant for build infrastructure rather than for a router in production\.

## Arguments

| Name | Type | Required | Values | Summary | Description |
| --- | --- | --- | --- | --- | --- |
| `listen` | string | no | any value of this type | Listen on this host\:port \(default \:8080\)\. | The address the server listens on\, as host\:port\, an IPv6 host in brackets\. An empty host listens on every address\. Without it\, the server listens on \:8080\. |

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
