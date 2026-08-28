# `show bgp rib rpf`

## Ze command

- Syntax: `show bgp rib rpf`
- Registry path: `show bgp rib rpf`
- Mode: Read-only
- Wire method: `ze-rib-api:rpf`
- Answer shape: doc
- Address fields: source, next-hop
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Reverse-path forwarding lookup in the Loc-RIB. Performs a longest-prefix-match and returns the best-path entry. Use this to verify RPF checks would pass for a given source.

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
