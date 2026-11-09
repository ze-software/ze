# 2026-10-25 – 2026-11-09 (16 days)

This update covers two weeks, and a number of the changes can break a working configuration. Following RFC 8212, an eBGP peer with no policy configured no longer receives or announces any route, and the routing policy which lived under BGP is now shared by every protocol. Please read the changes below before upgrading.

The work left is kept as a list of items, each either required before the first release or planned for after it. 298 items are now required, up from 240, and 348 are planned for later, up from 273. Most of the 71 new required items came from searching the code for errors which were ignored rather than reported, and 12 left the list. The numbers compare both ends of the period, so an item added and completed in between is not counted, and they count work items, not how close the release is: https://ze-software.net/project/roadmap/

## ⚠️ Changes to check

- BGP: an eBGP peer without an import or export policy no longer takes or sends routes (RFC 8212 Section 3), iBGP is not affected. Should you want the old behaviour, add `filter { import { final accept; } export { final accept; } }`. Both `ze config validate` and the session warn, giving the command to run.
- Policy: `bgp { policy { ... } }` moved to a top-level `policy { ... }`, used by BGP, IS-IS, OSPF, static, connected and the RIB. The old form is refused and should be moved by hand.
- Policy: a chain can no longer name a `prefix-list` (or any other list) directly. The list goes in a `match` instance, `policy { match customers { prefix-list CUSTOMERS; } }`, which the chain then uses with `filters [ customers ]`.
- Redistribution: the `family` and `tag` leaves of a redistribute import, as well as OSPF's redistribute `metric`, `metric-type` and `tag`, are replaced by filters, and the old leaves are refused.
- ADD-PATH `limit N;` is now `limit { receive N; }`.

## ⚠️ More changes to check

- RADIUS: every request is now signed with a Message-Authenticator, and any answer without one is dropped (CVE-2024-3596, "BlastRADIUS"). A server which can not sign its answers should be upgraded, `require-message-authenticator false` is there for when it can not be.
- IPsec: only Ze is now allowed to install per-socket IPsec policies (`vpn ipsec application-override deny`, the default). It requires Linux 6.10 and CAP_BPF, and the commit fails without them, so a container needs CAP_BPF or `application-override allow`.
- Connection tracking helpers are only attached when listed in `system conntrack module [ ... ]`, which is empty by default (RFC 4787).
- L2TP and PPPoE: `auth-method chap-md5` now means CHAP only, where a subscriber could previously negotiate down to PAP. The allowed methods are listed with `authentication { allow [ ... ] }` or `pppoe authentication-allow`.
- DHCP, router advertisements and L2TP pools now give out the DNS servers learned on the WAN when none are configured. `dns-source { source [ configured ]; }` keeps the previous behaviour.

## 🔒 Security

- RADIUS: without the signature, anyone between Ze and the RADIUS server could forge an Access-Accept, for a system login or for a subscriber. Configurations without RADIUS were not exposed.
- gNMI returned BGP passwords, RADIUS and TACACS+ keys in clear, and a Set echoed them to every subscriber. They are now sent in `$9$` form. Configurations without gNMI were not exposed.
- GTSM: for a peer using it, SYNs below the TTL floor are now dropped by the firewall before reaching BGP (RFC 5082 Section 3), and `ze doctor` checks the rule is installed.

## 🏠 Home router

Ze can now replace the router of a home FTTP line, using PPPoE carried to the LNS over L2TP. The PPPoE client learns its DNS servers (RFC 1877), runs IPv6 with DHCPv6 prefix delegation, and has new `keepalive` and `reconnect` options. A LAN interface takes a /64 of the delegated prefix with `ipv6 delegated-prefix <client>`. It was tested against accel-ppp, xl2tpd and strongSwan; reports from other access networks are welcome.

- `service dns-resolver` is a caching DNS forwarder for the LAN, with `show dns resolver`. It does not validate DNSSEC yet.
- `show system dns` shows which DNS servers each service uses and where they were learned.

## 🛰️ Routing

New:

- `timer { delay-open N; }` delays sending the OPEN, for peers which need it with BFD.
- PATHS-LIMIT is enforced in both directions, counted by `ze_bgp_paths_limit_refused_total`. Tested against FRR and BIRD.
- `show rib candidates` lists the paths hidden by a policy, and which chain hid them.
- IS-IS `metric-style narrow|transition|wide`, wide by default (RFC 3787 Section 5).
- IKEv2 supports Ed25519 and Ed448 signatures (RFC 8420), and a second authentication round (RFC 4739).

Fixed:

- A policy prepend created a new AS_PATH segment rather than extending the first one (RFC 4271 Section 5.1.2).
- A peer with `asn4 disable` and remote AS 23456 was refused with Bad Peer AS (RFC 6793 Section 4.1).
- The 14 September update said `preference` ordered RPKI cache failover. It did not correctly until now (RFC 8210 Section 8.3).
- The 18 May update announced IKE virtual IP pools, but no address was handed out until now.
- OSPFv3 installed no IPv4 next hop (RFC 5838), and IS-IS point-to-point links to FRR only worked on FRR's first circuit.

## 📚 Closing the RFC gap

A requirement counts as proven when its test fails once the code behind it is broken on purpose. There were 1,844 on 11 October, 2,108 on 25 October and 2,388 today.

Most of the work went into running these proofs on Ze's own system in a virtual machine, which let test suites run that could not start before. A requirement Linux meets for Ze now has a test checking the state Ze installs, and a requirement only the kernel can decide is reported on its own rather than counted as proven.

2,239 of the 5,367 MUST requirements written down are now covered, against 1,994 of 4,680 last time: https://ze-software.net/quality/rfc-compliance/
