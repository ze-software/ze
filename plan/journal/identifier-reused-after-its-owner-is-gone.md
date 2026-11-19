# Identifier reused after its owner is gone

A cache or a map is invalidated correctly when the object is deleted, and then
refilled by a reader that resolves the same identifier again. The identifier is
one the kernel or a foreign system is free to hand to a different object, so the
refill answers for the NEW owner while the caller is still asking about the old
one. The invalidation is right; what is missing is any way for the refill to
know which owner the caller meant.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-19 | fixit-vpp-loopback-recreated-per-apply | iface vpp backend | tunnel creates (`tunnel.go`, `vxlan.go`) send their add message on every apply: a second interface, a deleter aimed at the first, stale `nameMap` reverse entries | open: WireGuard fixed (`ensureWireguardInterface` checks `lookupIndex`). `createGRETunnel`, `createIPIPTunnel`, `createVxlanTunnel` still create with no existence check |
| 2026-08-18 | - | iface netlink monitor | `ifName` (`monitor_netlink_linux.go`) resolves a route or address event's index through `netlink.LinkByIndex`; a reused index gives a late event the new link's name | open: 58ce054f7e stopped `ifName` caching it. A late event for a deleted link still carries the new link's name once; the name must be bound to the event's link |
| 2026-10-26 | spec-ike-unbound-peer-inner-address | VPP SAD id lookup (`liveSADID`, `ike/dataplane/vpp_policy.go`) | A new SA installed with a removed SA's SPI, destination and protocol takes its identity in `sadIDs`, so the stranded entry's retry resolves the new SA's id: it misses the stranded entry, or deletes the new SA's live one. Latent: `vppPolicyInterface` refuses IfIndex 0 | open: key the retry on the SAD id the recorded entry names, never on an identity a new SA can reuse |
| 2026-11-11 | single-owner-lifecycle-5-session-end-locals | L2TP PPP event handlers (`handlePPPEvent`, `handleSessionIPAssigned`, `internal/component/l2tp/reactor_kernel.go`) | `ppp.StartSession` and every `ppp.Event` name a session by tunnel id and session id only, and each handler looks it up by them. `allocateSessionID` can redraw a freed id at once, so an event queued for the ended session (a 64-slot channel) lands on its successor: an address written, or a teardown | fixed: 98b534756d, 1ed4cebeab; pppSessionLocked checks control and birth (TestLatePPPEventIsRefusedOnAReusedSessionID) |
