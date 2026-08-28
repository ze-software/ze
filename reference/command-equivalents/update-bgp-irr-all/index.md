# `update bgp irr all`

## Ze command

- Syntax: `update bgp irr all`
- Registry path: `update bgp irr all`
- Mode: Daemon
- Wire method: `ze-update:irr-all`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Refresh all IRR prefix-lists immediately. Re-queries the IRR server for every enrolled ASN and atomically swaps prefix-lists on success. Failed refreshes preserve the existing prefix-list and report an error.

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
