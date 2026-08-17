# Week of 2026-08-03

Ze is being checked against the RFCs it implements, one MUST at a time, and that is where most of this week's work came from. BGP took the largest share. IS-IS learned to answer a purge of its own LSP, gNMI stopped serving unauthenticated writes, and web and DNS listeners can now carry an operator's own certificate.

## 🛰️ BGP on the wire

New:

- Prefix limits are per address family. `teardown`, `idle-timeout` and `updated` now sit per family, joined by a new `reconnect` leaf. `idle-timeout 0`, the default, holds the peer down rather than reconnecting, and that peer reads `idle-hold`. Set `prefix { reconnect backoff; }` for the previous behaviour.
- `show bgp peer detail` reports ConnectRetryCounter, mandatory session attribute 2 in RFC 4271, which Ze did not have at all. It is also a gauge.
- `ze_bgp_announce_dropped_oversize_total` counts an announce whose route does not fit its build buffer, labelled by rail and stage. A log line was the only trace before.
- A peer can record every message it receives to a bounded file, and `ze-test replay` feeds that file back through the session with an injected clock. Off by default.

Fixed:

- A message that only withdraws routes carries no path attributes, per RFC 4271 Section 4.3. Five egress rules added one anyway, FRR rejected each as malformed, and the route stayed live at the peer. The End-of-RIB marker was stamped the same way.
- LOCAL_PREF crossed the AS boundary on the two forward rails, which RFC 4271 Section 5.1.5 forbids.
- A hold expiry told the peer nothing. RFC 4271 Section 8.2.2 Event 10 lists seven actions and Ze performed one. Worst-case dead-peer detection falls from two hold times to one.
- MCAST-VPN and BGP-MUP relayed route types Ze cannot read, against RFC 7606 Section 5.4. Both are now parsed and installed as opaque RIB entries.
- An MP_UNREACH-only UPDATE whose withdrawals were all unrecognized was rebuilt into a forged End-of-RIB.
- A malformed COMMUNITY was rounded down, which put an attribute on the wire the peer never sent.
- Community names lived in five tables and two had drifted, so five names parsed through one entry point and 31 through the other.
- One route could stall a route server. The community strip cost grew with the square of the community count, and a peer can fill that attribute to the maximum the format allows.
- A plugin's cache-consumer declaration never reached the reactor, so an announce could be dropped from the cache before it flushed and reach no peer.
- A crashed egress filter was reported as a policy decision, with no counter and no log.
- A config reload wiped the learned-role map, so a peer configured with `role { export customer }` stopped advertising to its customers until the session bounced.

## 🛰️ IS-IS

`show isis database` carries an `own` field, so an operator can tell which LSPs this node originated. A falling lifetime on one means the refresh timer is broken; a purge of one means a neighbour withdrew this node's advertisement.

Fixed:

- Ze relayed a purge of its own LSP and could not get back into the database, because own-LSP sequence numbers ignored what the network already held. ISO 10589 clause 7.3.16.4 c) is implemented, and FRR 10.3.1 confirms the recovery.
- A signed own LSP advertised a checksum no receiver can reproduce.
- Ze could not open an IS-IS circuit at all when the configuration named its interfaces only under `isis{}`. All six FRR IS-IS interop scenarios were red for that reason and nothing reported it.

## 🔒 IPsec and IKEv2

`show ipsec dataplane` reads the SAs and policies the kernel actually holds, rather than what the engine believes it installed. A doctor check covers the XFRM state, and a health detector reports the drift between the two. Every other IPsec surface reports the engine's own record, so a state the kernel silently refused was invisible from inside Ze.

Fixed:

- Every EAP-Success and EAP-Failure carried the wrong Identifier, which a conforming peer discards under RFC 3748 Section 4.2.
- A Delete was never retransmitted, and it is the peer's only signal that an SA is gone.
- The initiator keyed its Child SAs from its own first proposal rather than the one the responder accepted, so ESP ran under algorithms the peer never agreed to.
- Ze's CLI had never worked in the IPsec interop lab, so one scenario had been failing at its first command since it was written.

## 🛡️ DDoS mitigation

- `max-mitigation-duration` is enforced. Both plugins parsed it, defaulted it to 3600 and range-checked it, and neither ever read it, so the one-hour cap it promises did not exist.
- Once a FlowSpec rule went upstream, nothing withdrew it: the probe that decides when to lift mitigation had no driver.
- A target the traffic policy exempts stayed blackholed upstream, and a victim that narrows to a local address kept its upstream rule standing.
- Four read-only show commands each held a mutex across a netlink reconcile, an event fan-out or an RPC to the BGP engine.

## 📡 Subscriber edge

- RADIUS accounting carries Framed-IP-Address (8) and NAS-Port-Id (87) on Start, Interim and Stop. RFC 2866 Section 4.1 makes the first mandatory, and Ze emitted nine attributes without it, so a server could not join a record to a subscriber. Proven with a real xl2tpd LAC in front of Ze.
- A LAN interface can send Router Advertisements, through a new `router-advertisement` container carrying every bound RFC 4861 Section 6.2.1 states. The sender was welded to the BNG with a fixed layout before.
- `show interface errors` returned every interface as if each had errors, `show interface type` returned usage text, and `show interface brief` returned full detail.

## 🔐 Management plane and operations

- Web, AS112 and GeoDNS listeners take a certificate from the PKI store, with the chain assembled and verified. A broken reference exits 1 and names what is available rather than degrading to self-signed, and a reload rotates the material without dropping the listener.
- The looking glass gets TLS on by default and an optional bearer token.
- The birdwatcher-compatible looking glass API has a specification now: endpoints, envelope, protocol object, route object, and every divergence, written with RFC 2119 keywords. Upstream publishes none.

Fixed:

- gNMI served unauthenticated read and write on 0.0.0.0:9339 once enabled, and Set is a full config mutation. MCP had a fail-closed check that startup never called. `ze.web.insecure=true` bypassed the loopback clamp. The looking glass threw away its TLS and token settings unless its block said `enabled true`.
- Config that Ze accepts and delivers nowhere is refused, and a doctor check reports the same condition on a running daemon.
- The config editor runs a session's commands one at a time. Each newline of a pasted block used to dispatch before the previous command answered, racing the editor's own document fields.
- An operator whose `ze start` aborted saw an exit code and nothing else.
- The appliance build could produce an image whose credential database was never written. It built green and failed at boot.
- A store flush could report the file it had just written as missing.

## 📚 Standards ledger

Ze is being checked against every RFC it implements, one MUST at a time. A MUST counts as met only when a test names it, and only when that test fails if the behaviour is taken out. The work has started rather than finished, and it produced most of the list above. Claude Opus 5 does the checking.

Where it stands: 2950 MUSTs are checked across 168 documents, and 85 of them still owe a test. Three documents have been read end to end against their own source text, to confirm the list of MUSTs missed none. The other 165 have not. So a green run today proves everything on the list, and does not yet prove the list is complete.

What that turned up this week:

- RFC 4271 Section 8.2.2 Event 10 lists seven actions on a hold expiry. Ze performed one of them.
- RFC 3748 Section 4.2's Identifier rule had no row on any checklist, so nothing was watching while every EAP-Success and EAP-Failure Ze sent was one a conforming peer discards.
- RFC 4271 Section 9.1.2's AS-loop rule has worked in Ze for a long time, and nobody had ever written it down as a requirement. Four tests that already existed were counted as proof of nothing. The code beside them carried a quote attributed to RFC 4271 that appears nowhere in it, asserting a MUST the text does not state.
- Two notes saying an obligation does not apply to Ze were re-read against the RFC instead of being taken at their word. Both were wrong. Both are now being implemented: VRRP accept-mode filtering, and BGP AS Confederation.
- One malformed tag stopped the checker before it reached any other file, so that run judged no coverage anywhere. Three MUSTs had their tests filed as proof of the opposite case, which made one rule look four times better proven than it is.

## 🔭 Coming up

MUST level comes first. The current order is: fix the defects this work has already turned up, clear the 85 MUSTs that still owe a test, then read the remaining 165 documents end to end so every list of MUSTs is known to be complete.

The SHOULD level is designed and waits behind all of that. 1053 SHOULDs are already written down and checked by nothing, and the lowercase `should` an older RFC uses has to be read as well. It is real work, and it is not the work that comes next.
