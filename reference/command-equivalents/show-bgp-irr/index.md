# `show bgp irr`

Show IRR filter status per ASN\.

## Ze command

- Registry path: `show bgp irr`
- Usage: `show bgp irr`
- Mode: Read-only
- Wire method: `ze-bgp:show-irr-status`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `check`, `prefix`
- Answer shape: tab
- Address fields: none
- Column order: asn, as-set, status, error, ipv4-count, ipv6-count, server, generation, checked-at, refreshed-at, peers
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Lists each enrolled ASN with the AS\-SET it is filtered by\, the prefix counts of the stored copy the filter enforces\, which copy that is \(server\, generation\, checked\-at\, refreshed\-at\)\, and whether it could be read\. The irr plugin fetches and stores the copies\; show irr answers when each is due\.

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
