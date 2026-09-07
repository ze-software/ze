# `request as112 healthcheck`

Send one authoritative query and check the AS112 answer.

## Ze command

- Registry path: `request as112 healthcheck`
- Usage: `request as112 healthcheck [target <target>]`
- Mode: Daemon
- Wire method: `ze-as112:health`
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

The query goes to an anycast service address, or to the given target. The exit code is 0 only when the expected AS112 answer comes back. Finding M4: the tool a healthcheck probe calls, since dig is not on the gokrazy appliance and 'ze resolve dns' cannot target a specific server.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `target` | string | no | any value of this type |

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
