# 026 - A related ICMP error belongs to the session it quotes

**Spec:** spec-gtsm-related-icmp-quoted-destination, closed 2026-09-15
**Class:** `plan/journal/guard-keyed-on-a-field-its-consumer-ignores.md`

## What the work built

The RFC 5082 related-message filter (`peerTerms`,
`internal/component/gtsm/gtsm.go`) claimed an ICMPv4 error by its outer source
address being the peer. The kernel delivers the error to a socket by the header
the error quotes (`tcp_v4_err`), so a forged error from any other address,
quoting the session and carrying a low outer TTL, walked past the filter.

Now each `ze_gtsm` term matches the destination QUOTED inside the error
(`MatchICMPErrorQuotedDestination`, `internal/component/firewall/model.go`)
beside the quoted TCP port, and reads no outer field but the TTL. The nft
lowering (`lowerICMPErrorQuotedDestinationMatch`,
`internal/plugins/firewall/nft/lower_linux.go`) is a 4-octet compare at
transport offset 24 behind the shared `quotedIPv4HeaderGuard` (nfproto IPv4,
`meta l4proto icmp`, quoted version and IHL byte 0x45). nft renders it as
`@th,192,32 0xc0000202` for 192.0.2.2.

## Decisions

- Match the quoted DESTINATION, not the quoted source: an error ze receives is
  about a packet ze sent, whose destination is the peer, and the peer address
  is always configured where ze's local address is not.
- Keep the quoted-port match beside it: the port ties the error to the BGP
  session rather than to any other flow toward the peer (RFC5082-3-4).
- No new interop scenario: a dropped forged error is invisible to the peer
  daemon, so the kernel counters (`IcmpMsg InType3`, `TCPMinTTLDrop`) are the
  only observer, and the three receive proofs read them.
- The 0x45 guard is one helper shared by both quoted-header lowerings, so the
  two cannot drift on which guard precedes a fixed-offset read.

## What the review found

- The functional fixture rendered the compared value with `%08x`; nft prints a
  raw payload compare with no zero padding (`0x6`, `0x6fe`). Identical for
  192.0.2.2, wrong for any first octet below 0x10. Fixed to `%x`.

## Traps for the next session

- A `.ci` asserting a dotted address in `nft list table` goes red the moment
  the match moves into a quoted header: nft has no name for a field inside a
  quote and prints `@th,<bit offset>,<bit length> 0x<unpadded hex>`.
- A quoted header carrying IP options moves the port offsets. The proof for it
  chooses option octets that spell the BGP port at the 20-octet offsets, so
  only the version and IHL guard keeps the message Unknown.
- A tagged unit whose claim widens owes a re-observed discrimination record and
  the owner's `./le rfc approve unit <package>.<TestName> reason "<words>"`
  before `./le commit create` accepts the change.
