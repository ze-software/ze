# `show policy test peer`

Test what your policy does to a specific UPDATE.

## Ze command

- Registry path: `show policy test peer`
- Usage: `show policy test peer <selector> <import\|export> [filter <name>] update <hex> [source-asn4 <true\|false>]`
- Mode: Read-only
- Wire method: `ze-show:policy-test`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: `filter`, `source-asn4`, `update`
- Answer shape: not declared
- Address fields: none
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save
- Pipes, when the answer has rows: match, count, first, last, display, fill
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

Feed a hex-encoded BGP UPDATE through a peer's filter chain and see the accept/reject result plus attribute modifications at each stage. Read-only: no routes are actually forwarded. Great for validating policy changes before you commit. The selector and the direction, filter, update and source-asn4 tokens are parsed by the handler, so the peer selector can be a free-form name or address and the tokens can arrive in any order.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `selector` | string | yes | any value of this type |
| `direction` | enum | yes | `import`, `export` |

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
