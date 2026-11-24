---
spec: rfc-proof-debt-outside-filter-wire
topic: BGP
found-by: rfc-check
---

## Before

Explicit raw BGP UPDATE commands (`send bgp <peer> raw hex ...`, in both body
and full-packet forms) bypassed negotiated multiprotocol family admission. A
parseable raw UPDATE carrying labeled IPv6 VPN (`ipv6/mpls-vpn`, AFI 2 / SAFI
128) or another unnegotiated address family could be transmitted onto a BGP
session that had negotiated only `ipv4/unicast`.

<!-- source: internal/component/bgp/reactor/session_ownership.go -- adjOutWrite.section -->
<!-- source: internal/component/bgp/reactor/session_write.go -- Session.writeUpdateBody -->

## After

Parseable locally originated raw BGP UPDATE commands now obey RFC 4760 Section
8 and RFC 4659 Section 3.4 negotiated-family admission on the session wire.
Pure raw UPDATEs carrying only an unnegotiated address family are withheld
without resetting the peer session, mixed raw UPDATEs have unnegotiated family
sections stripped while negotiated families in the same UPDATE still transmit,
and unparseable or unknown-family diagnostic payloads remain opaque.

<!-- source: internal/component/bgp/reactor/session_ownership.go -- adjOutWrite.section -->
<!-- source: internal/component/bgp/reactor/session_write.go -- Session.writeUpdateBody -->
<!-- source: docs/architecture/api/update-syntax.md -- Raw Passthrough Commands -->

## Corrects

none

## Exposure

Only peers configured with `send [ raw ]` where an operator or diagnostic
script injected parseable raw UPDATEs for an address family the session had not
negotiated were affected. Ordinary BGP route origination, RIB forwarding, and
sessions without raw passthrough already enforced negotiated-family admission.

## Upstream

none

## Asked by

none
