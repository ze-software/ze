---
spec: ppp-ncp-independent-lifetime
topic: BNG
found-by: review
---

## Before

On an L2TP or PPPoE subscriber session, Ze treated IPCP and IPv6CP as if
their life was tied to the session's.

- A CPE that answered Ze's IPCP or IPv6CP with an LCP Protocol-Reject, as
  pppd does for a protocol it is told not to run, kept receiving that NCP's
  Configure-Requests until its retry count ran out.
- When a subscriber closed one family after the session came up, for example
  an IPCP Terminate-Request, Ze removed the address from the PPP interface but
  told nobody else. The subscriber `/32` and its framed routes stayed in the
  redistribution feed, so BGP kept announcing them. `show subscriber` and
  `show l2tp sessions` kept showing the address, and RADIUS Interim-Updates
  kept reporting it as Framed-IP-Address.
- Framed-IPv6-Route prefixes were announced when IPv4 came up rather than when
  IPv6 did, and an IPv6-only session never announced them.
- A session whose NCPs had all gone could stay up for ever, with LCP open and
  no network protocol, and could even be reported up with no NCP open.
- A CPE that Code-Rejected an LCP Echo-Request kept being sent them.

## After

Each NCP now opens and closes on its own for the whole life of the session.

- A Protocol-Reject naming IPCP or IPv6CP stops that NCP until LCP
  renegotiates. The other family carries on, so an IPv4-only CPE comes up as
  soon as IPCP opens. A Protocol-Reject received while LCP is not open is
  ignored, as RFC 1661 asks.
- When one family closes, its routes leave the redistribution feed (for IPv4
  the `/32` and the Framed-Route prefixes, for IPv6 the Framed-IPv6-Route
  prefixes), and its address leaves the subscriber registry, `show l2tp
  sessions` and RADIUS accounting. The other family is untouched. When the
  subscriber reopens the family, everything comes back with the same address.
- `show subscriber id <id> detail` now prints `ipv6-interface-id`.
- A session with no NCP open is held for one bound: `ncp timeout` on L2TP
  (30 seconds by default), a fixed 30 seconds on PPPoE. An NCP opening cancels
  it. A session that came up and then lost every NCP is closed with an LCP
  Terminate-Request and an Acct-Terminate-Cause of Idle Timeout. A session
  that never got an NCP open ends with NAS Error, as before.
- A Code-Rejected Echo-Request stops the echo probes for the rest of the
  session.

With the defaults, a session that keeps at least one family open behaves
exactly as before.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
