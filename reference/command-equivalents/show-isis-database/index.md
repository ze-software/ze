# `show isis database`

## Ze command

- Syntax: `show isis database`
- Registry path: `show isis database`
- Mode: Read-only
- Wire method: `ze-show:isis-database`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show the IS-IS link-state database. Lists each LSP with its LSP ID, sequence number, remaining lifetime, checksum, and overload bit, across Level-1 and Level-2. The own field is true on the LSPs this node originated and false on the LSPs it learned from a neighbor.

## Mapping intents

### IS-IS neighbors, database, routes, and interfaces

Category: Routing protocols

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
