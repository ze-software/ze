# `request as112 healthcheck`

One-shot authoritative query against an anycast service address (or the given target), exit 0 iff the expected AS112 answer comes back.

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

Finding M4: the tool a healthcheck probe calls, since dig is not on the gokrazy appliance and 'ze resolve dns' cannot target a specific server.

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
