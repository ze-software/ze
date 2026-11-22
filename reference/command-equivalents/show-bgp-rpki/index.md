# `show bgp rpki`

Show RPKI validation counters with one row for each cache server

## Ze command

- Registry path: `show bgp rpki`
- Usage: `show bgp rpki`
- Mode: Read\-only
- Wire method: `not listed`
- Backends: any backend
- Task support: optional\: the MCP call is synchronous\, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: address
- Column order: address\, port\, state\, synced\, version
- Pipes, always: json\, ndjson\, table\, text\, yaml\, raw\, no\-more\, save\, match\, count\, first\, last\, display\, fill\, resolve\, origin
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: `summary`: The validation counters\, without the cache server rows (`display vrp-count validation-enabled sessions-total sessions-established sessions-synced aspa-enabled aspa-records`)

## Arguments

No command-specific arguments listed.

## Mapping intents

### RPKI cache and validation session state

Category: BGP

## Vendor equivalents

### Junos MX
- `show validation session` (verified, junos\-bgp)
  - Intent: RPKI cache and validation session state

### IOS XR
- `show bgp rpki server summary` (verified, iosxr\-bgp)
  - Intent: RPKI cache and validation session state

### SR OS

No equivalent listed.

### VyOS
- `show rpki cache-connection` (verified, vyos\-cli)
  - Intent: RPKI cache and validation session state
