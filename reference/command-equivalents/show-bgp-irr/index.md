# `show bgp irr`

Show IRR filter status per ASN.

## Ze command

- Registry path: `show bgp irr`
- Usage: `show bgp irr`
- Mode: Read-only
- Wire method: `ze-show:irr-status`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `check`, `prefix`
- Answer shape: tab
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Lists each enrolled ASN with its resolved AS-SET, prefix counts, last refresh time, and error status. Use this to confirm that IRR prefix-lists are loaded and current.

## Arguments

No command-specific arguments listed.

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
