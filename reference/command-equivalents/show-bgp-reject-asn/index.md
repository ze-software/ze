# `show bgp reject-asn`

Show every reject-asn list.

## Ze command

- Registry path: `show bgp reject-asn`
- Usage: `show bgp reject-asn`
- Mode: Read-only
- Wire method: `ze-show:reject-asn`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `known`, `name`
- Answer shape: tab
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: `peers`: The peer rows, without the aggregate fields (`display peers`); `summary`: The aggregate fields, without the peer rows (`display router-id local-as uptime peers-configured peers-established`)

Lists each configured reject-asn list with every ASN it holds, the effective position set for that ASN, the network the curated table names for it, and how many peers name the list on import and on export. An ASN the curated table does not know prints with no annotation.

## Arguments

No command-specific arguments listed.

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
