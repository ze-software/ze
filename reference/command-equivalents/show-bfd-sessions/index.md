# `show bfd sessions`

## Ze command

- Syntax: `show bfd sessions`
- Registry path: `show bfd sessions`
- Mode: Read-only
- Wire method: `ze-bfd-api:show-sessions`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

List all active BFD sessions. One line per session: peer address, state, negotiated tx/rx intervals, and detect multiplier.

## Mapping intents

### BFD session state

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
