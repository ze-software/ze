# `resolve peeringdb as-set <asn>`

## Ze command

- Syntax: `resolve peeringdb as-set <asn>`
- Registry path: `resolve peeringdb as-set`
- Mode: Read-only
- Wire method: `ze-resolve:peeringdb-as-set`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Find the IRR AS-SET registered for an ASN in PeeringDB. Usage: resolve peeringdb as-set <asn>. Feed the result into 'resolve irr expand' to get the full member list.

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
