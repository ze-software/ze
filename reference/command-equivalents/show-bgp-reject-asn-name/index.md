# `show bgp reject-asn name`

Show one reject-asn list.

## Ze command

- Registry path: `show bgp reject-asn name`
- Usage: `show bgp reject-asn name <name>`
- Mode: Read-only
- Wire method: `ze-show:reject-asn-name`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: doc
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: `peers`: The peer rows, without the aggregate fields (`display peers`); `summary`: The aggregate fields, without the peer rows (`display router-id local-as uptime peers-configured peers-established`)

The same answer for a single list, for a config that holds many.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `name` | string | yes | any value of this type |

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
