# `show bgp rib`

## Ze command

- Syntax: `show bgp rib`
- Registry path: `show bgp rib`
- Mode: Read-only
- Wire method: `ze-rib-api:routes`
- Answer shape: tab
- Address fields: peer, next-hop
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill, resolve, origin
- Pipes, on rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: `advertised`: Select advertised routes; `community <value>`: Filter by standard community; `count`: Count matching routes without serializing rows; `family <value>`: Filter by AFI/SAFI; `first <value>`: Take first N routes; `graph`: Render AS-path topology graph; `histogram`: Count routes by family and prefix length; `last <value>`: Take last N routes; `match <value>`: Cross-field structured match; `path <value>`: Filter by AS path; `peer <value>`: Filter by peer; `prefix <value>`: Filter by prefix; `received`: Select received routes
- Pipe aliases: none

Query routes in the BGP RIB. Look at received or advertised routes with flexible filters: peer, family, prefix, AS path regex, community, match expression. Pipe operators: | count, | histogram, | graph. This is the main route inspection command.

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
