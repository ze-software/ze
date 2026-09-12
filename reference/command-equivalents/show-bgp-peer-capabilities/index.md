# `show bgp peer capabilities`

Show what capabilities were negotiated with a peer.

## Ze command

- Registry path: `show bgp peer capabilities`
- Usage: `show bgp peer <selector> capabilities`
- Mode: Read-only
- Wire method: `ze-bgp:peer-capabilities`
- Backends: any backend
- Task support: optional: the MCP call is synchronous, which is the default
- Subcommands: none: this command takes no subcommand
- Answer shape: tab
- Address fields: peer
- Pipes, always: json, ndjson, table, text, yaml, raw, no-more, save, match, count, first, last, display, fill, resolve, origin
- Pipes, on its rows: none
- Pipes, while streaming: log
- Pipes, local process only: save
- Command pipes: none
- Pipe aliases: none

The row carries the peer address and the FSM state. Once the OPEN exchange completed, a negotiated map follows with the address families, the extended message capability, enhanced route refresh, and 4-byte ASN support. negotiated.paths-limit carries nonzero limits keyed by family: send is the peer's maximum paths per prefix enforced on our outbound updates; receive is our advertised receive request, not a locally enforced inbound limit. Zero or absent limits are omitted. A selector that matches no peer is refused with 'no matching peers'.

## Arguments

| Name | Type | Required | Values |
| --- | --- | --- | --- |
| `selector` | string | yes | any value of this type |

## Mapping intents

### Negotiated BGP capabilities for a peer

Category: BGP

## Vendor equivalents

### Junos MX

No equivalent listed.

### IOS XR

No equivalent listed.

### SR OS

No equivalent listed.

### VyOS

No equivalent listed.
