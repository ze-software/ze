# `show bgp irr check`

Check if a prefix is accepted by the IRR filter.

## Ze command

- Registry path: `show bgp irr check`
- Usage: `show bgp irr check <peer> <prefix>`
- Mode: Read-only
- Wire method: `ze-show:irr-check`
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
- Pipe aliases: none

Reports whether the prefix would be accepted or rejected, and which entry matches.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `peer` | string | yes | any value of this type |
| `prefix` | string | yes | any value of this type |

## Mapping intents

### RPKI cache and validation session state

Category: BGP

Current Ze commands are IRR-focused. RPKI-specific show paths can be added to this entry when present in the live command catalog.

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
