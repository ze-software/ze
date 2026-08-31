# `update firewall irr asn`

Fetch or refresh IRR prefix-list for an ASN.

## Ze command

- Registry path: `update firewall irr asn`
- Usage: `update firewall irr asn <asn>`
- Mode: Daemon
- Wire method: `ze-update:firewall-irr-asn`
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

Queries the IRR server and saves resolved prefixes to the zefs cache. Creates the cache entry if it does not exist.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `asn` | string | yes | any value of this type |

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
