# Week of 2026-07-01

Work landed across DNS, traffic security, routing, and the CLI this week.

## 🌐 AS112 anycast DNS

The AS112 node flagged last week as design work has shipped (RFC 7534 / 7535):

- An authoritative sink for misdirected RFC 1918 and link-local reverse-DNS queries, plus the EMPTY.AS112.ARPA DNAME redirection zone.
- While the node is serving, Ze originates the four fixed covering prefixes into BGP under a watchdog, with an operator-chosen origin AS (default 112) and optional community. It withdraws them the moment the node stops answering.
- Doctor checks catch misconfiguration: watchdog-withdraw wiring and a global-origin sanity check. Interop verified against FRR and BIRD.

## 🛡️ Behavioral anomaly detection

A new security domain, separate from volumetric DDoS, that watches how traffic behaves rather than how much of it there is:

- A report-only detector builds a per-source baseline (fan-out, out/in ratio, destination-port entropy, new-peer and rare-port activity, coarse beaconing), scores deviation and cohort rarity, and correlates weak signals into a single incident with a confirm/clear lifecycle. Visible under `show anomaly`.
- Freeze-learn keeps a sustained attack from quietly training the baseline to accept it, and a never-seen source cannot trip a first-sight false positive.
- A shadow-first responder installs surgical per-source firewall rate-limits with per-entity arming, timed auto-revert, a blast-radius cap, a kill switch, and an allowlist. Visible under `show anomaly-shape`.
- Underneath, traffic analysis was split into a neutral facts layer that both the DDoS and anomaly families read, surfaced on its own via `show traffic-feature`.

## 🛰️ Routing

- OSPF gained a full extension family across both OSPFv2 and OSPFv3: Segment Routing, traffic engineering, TI-LFA and LFA fast reroute, graceful restart, BFD, LDP-IGP sync, virtual links, NBMA and point-to-multipoint, multi-instance, OSPFv3 multi-AF, and OSPFv3 IPsec.
- BGP now emits AS4_PATH toward two-octet peers when a four-octet origin AS cannot be mapped, so the peer recovers the real origin instead of seeing AS_TRANS (RFC 6793).
- Redistributed routes now replay to peers that establish after injection, so a dynamic or inbound peer no longer misses routes it joined too late to hear.
- Static is now a first-class redistribute source, and import sources are validated at config-check time.
- SR-Policy: RFC 9830 S-bit compliance on Type-A segments and the binding SID, plus ExaBGP bridge parity.

## ⌨️ Verb-first CLI

Command syntax is now uniformly verb-first, enforced so it stays that way:

- Reads consistently: `show host`, `show crashes`, `show event list`, `show metrics pool`, `request config archive`.
- `show host` and `show crashes` work whether or not the daemon is running.
- Turning debug output on or off now uses the same set / delete / show / clear verbs as any other setting, instead of a one-off command.
- DNS cache inspect and clear split into separate per-action commands.

## 🔒 Security & hardening

- Six outbound services (BMP, RPKI/RTR, flow-export, IRR whois, the managed hub TLS client, and LDP) can now bind to a chosen source address; unset means OS-selected, unchanged.
- Config archive now redacts credentials from both error and success messages.
- Updated golang.org/x/net to v0.56.0 for a CVE advisory.

## 🛠️ Under the hood

- The VPP backend reloads on crash or reconnect to clear stale state instead of running on.
- DNS serving moved to a shared in-core harness (now behind both GeoDNS and AS112) with the recursion guard turned into an enforced invariant.

## 🔭 Looking ahead

The near-term focus is proving the capabilities already shipped hold up in real deployments. Further out, with no fixed order, some design threads are being sketched, including IPsec / IKEv2 hardening and a DHCPv6 server.
