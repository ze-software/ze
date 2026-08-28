# `show policy test peer <selector> import|export [filter <name>]`

## Ze command

- Syntax: `show policy test peer <selector> import\|export [filter <name>]`
- Registry path: `show policy test peer`
- Mode: Read-only
- Wire method: `ze-show:policy-test`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, on rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Test what your policy does to a specific UPDATE. Feed a hex-encoded BGP UPDATE through a peer's filter chain and see the accept/reject result plus attribute modifications at each stage. Read-only: no routes are actually forwarded. Great for validating policy changes before you commit. Usage: show policy test peer <selector> import|export [filter <name>] update <hex> [source-asn4 true|false]. The selector and the import/export/filter/update/source-asn4 tokens are parsed by the handler so the peer selector can be a free-form name or address.

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
