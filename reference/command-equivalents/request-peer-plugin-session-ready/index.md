# `request peer plugin session ready`

Signal that per-peer plugin setup is complete.

## Ze command

- Registry path: `request peer plugin session ready`
- Usage: `request peer <selector> plugin session ready`
- Mode: Daemon
- Wire method: `ze-plugin:session-peer-ready`
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

The daemon closes this process's share of the peer's End-of-RIB barrier, so the peer stops waiting for routes from this process. The signal is keyed on the sending process, so one process does not release another. A peer of '*', and an empty peer, are both ignored.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `selector` | string | yes | any value of this type |

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
