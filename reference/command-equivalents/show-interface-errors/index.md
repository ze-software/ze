# `show interface errors`

## Ze command

- Syntax: `show interface errors`
- Registry path: `show interface errors`
- Mode: Read-only
- Wire method: `ze-show:interface-errors`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Show interfaces that have errors or drops. Filters to only interfaces with non-zero Rx/Tx error or drop counters. Quick way to find troubled links.

## Mapping intents

### Interface counters and error counters

Category: Interfaces

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
