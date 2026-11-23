# `show bgp rpki roa`

Show ROA table entries or lookup covering VRPs for a prefix

## Ze command

- Registry path: `show bgp rpki roa`
- Usage: `show bgp rpki roa`
- Mode: Read\-only
- Wire method: `not listed`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: prefix
- Column order: prefix\, max\-length\, asn
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save\, match\, count\, first\, last\, display\, fill\, resolve\, origin
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
