# A listener refuses the peer its config names

The daemon runs and completes its work, but the listener the config declares
for a peer refuses that peer's connection, so the session never forms.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-17 | spec-boot-record-after-listen | `test/reload/bgp-rib-maximum-paths-reload.ci`, the DUT's BGP listener | Red alone, on the memory store and with `option=storage:value=disk`: the test peer's 30 s dial (`dialTarget`, `internal/test/peer/peer.go`) gets `connection_refused` every try, while the staged fixture's SIGHUP reload completes. Not the store. The build carried other sessions' uncommitted `bgp/reactor` and `plugins/rib` hunks | open: cause not traced; rerun on a HEAD build, then read when the reactor binds the peer's listener |
| 2026-10-19 | - | `test/interop/scenarios/bgp-vpn-frr` | Ze refused the IPv4 session with Bad BGP Identifier: one FRR (AS 65002, id 172.30.0.3) holds both sessions | fixed: fixture, not Ze (RFC 4271 6.8 collision is per address pair; RFC 6286 2.1 uniqueness is a "should"); ze.conf sets allow-shared-router-id. Still red: checkVPNFRR reads FRR's RD-keyed JSON as 0 paths, and FRR 10.3.1 never prints a 48-octet VPN-IPv6 link-local; the tagged checker needs owner approval |
