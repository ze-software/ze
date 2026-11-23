# `show bgp rpki aspa`

Show ASPA cache or lookup providers for a customer AS

## Ze command

- Registry path: `show bgp rpki aspa`
- Usage: `show bgp rpki aspa`
- Mode: Read\-only
- Wire method: `not listed`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: none
- Column order: customer\-asn\, providers
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save\, match\, count\, first\, last\, display\, fill
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

## Arguments

No command-specific arguments listed.

## Mapping intents

### RPKI cache and validation session state

Category: BGP

## Vendor equivalents

### Junos MX
- `show validation session` (unknown, no source ref)
  - Intent: RPKI cache and validation session state

### IOS XR
- `show bgp rpki server summary` (unknown, no source ref)
  - Intent: RPKI cache and validation session state

### SR OS

No equivalent listed.

### VyOS
- `show rpki cache-connection` (verified, vyos\-cli)
  - Intent: RPKI cache and validation session state
