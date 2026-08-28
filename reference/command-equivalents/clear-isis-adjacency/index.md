# `clear isis adjacency`

## Ze command

- Syntax: `clear isis adjacency`
- Registry path: `clear isis adjacency`
- Mode: Daemon
- Wire method: `ze-clear:isis-adjacency`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Tear down every IS-IS adjacency so neighbors re-form. Usage: clear isis adjacency. Adjacencies re-learn from the next Hello; the circuit is not closed and the configuration is unchanged.

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
