# `show bgp reject-asn known transit-free`

Print the curated transit-free ASNs as a config block.

## Ze command

- Registry path: `show bgp reject-asn known transit-free`
- Usage: `show bgp reject-asn known transit-free`
- Mode: Read-only
- Wire method: `ze-show:reject-asn-known-transit-free`
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

Prints the well-known transit-free ASNs as one `indirect [ ... ];` line to paste directly under `reject-asn NAME { }`, with the sources and the curated date as comments. Ze acts on no ASN it was not configured with, so this command is how the set reaches a config. After the paste the config holds the numbers, and a later change to the curated table cannot alter it.

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
