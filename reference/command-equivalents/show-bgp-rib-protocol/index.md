# `show bgp rib protocol`

Show routes for a specific protocol: <protocol> [peer-selector] [pipeline-args...]

## Ze command

- Registry path: `show bgp rib protocol`
- Usage: `show bgp rib protocol`
- Mode: Read-only
- Wire method: `not listed`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: peer, next-hop
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill, resolve, origin
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: `advertised`: Select advertised routes; `community <value>`: Filter by standard community; `count`: Count matching routes without serializing rows; `family <value>`: Filter by AFI/SAFI; `first <value>`: Take first N routes; `graph`: Render AS-path topology graph; `histogram`: Count routes by family and prefix length; `last <value>`: Take last N routes; `match <value>`: Cross-field structured match; `path <value>`: Filter by AS path; `peer <value>`: Filter by peer; `prefix <value>`: Filter by prefix; `received`: Select received routes
- Pipe aliases: none

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
