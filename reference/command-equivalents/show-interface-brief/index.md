# `show interface brief`

## Ze command

- Syntax: `show interface brief`
- Registry path: `show interface brief`
- Mode: Read-only
- Wire method: `ze-show:interface-brief`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

One-line summary per interface: name, state, IP, and MTU. Quick way to see what is up and what addresses are assigned.

## Mapping intents

### Interface list and brief status

Category: Interfaces

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS
- `show interfaces` (verified, vyos-cli)
  - Intent: Interface list and brief status
