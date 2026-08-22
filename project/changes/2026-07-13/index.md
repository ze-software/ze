# Week of 2026-07-13

A busy routing week brought VRRP, BGP multipath in the FIB, BMP Loc-RIB monitoring, tighter RPKI policy, and several management-plane security fixes.

## Routing and first-hop redundancy

VRRP is now available for IPv4 and IPv6. VRRPv3 follows RFC 9568 by default, with VRRPv2 available for IPv4 groups. Each group owns a virtual MAC through a macvlan device, installs and removes virtual addresses with state changes, sends gratuitous ARP or unsolicited Neighbor Advertisements on takeover, and exposes state through the CLI, web UI, metrics, and events. IPv6 Duplicate Address Detection is disabled on the VRRP macvlan so failover is immediate. VRRP also has its own `ze_vrrp` compile-time feature gate.

BGP multipath selection now reaches the forwarding plane. Equal candidates are carried from the BGP RIB into the shared Loc-RIB, then installed as an ECMP group by the active FIB backend. A change to the sibling set updates the installed group even when the winning peer stays the same.

Runtime route injection now preserves IPv6 next hops. Native IPv6 routes and IPv4 routes using an RFC 5549 or RFC 8950 extended next hop are stored with MP_REACH_NLRI and forwarded with the negotiated capability. The RIB also gained `show bgp rib rpf`, a longest-prefix reverse-path lookup for IPv4 and IPv6 families.

Long-Lived Graceful Restart stale routes now pass through their destination-specific egress decision during RIB readvertisement. LLGR peers receive the stale route, non-LLGR eBGP peers receive a withdrawal, and non-LLGR iBGP peers receive a de-preferred route with `NO_EXPORT` and `LOCAL_PREF=0`.

## BMP and RPKI

BMP collectors can subscribe to the local RIB with `loc-rib true`. Ze emits RFC 9069 Peer Type 3 messages, sends one Loc-RIB Peer Up for the RIB instance, replays the current table when monitoring starts, streams subsequent best-path changes, and sends Peer Down on shutdown.

RPKI origin validation became policy-driven. Invalid and NotFound actions can be selected globally, per group, or per peer, with per-leaf inheritance in the order peer, group, global. ASPA Invalid and Unknown actions use the same model. Installed routes are revalidated when the VRP set changes, so a cache update can change the selected route without waiting for another BGP UPDATE. The RTR v2 path also gained the current ASPA Provider Authorization format from RFC 9582.

## CLI and operational work

Plugin commands now appear in interactive tab completion after the daemon starts. Command names were normalized into namespace form, including the remaining hyphenated roots, and commands that take filenames consistently accept `-` for stdin or stdout. This makes config and MRT pipelines work without temporary files.

`show | compare` now displays only the modified subtree. `show reload status` exposes a generation counter and the last reload outcome, giving automation a reliable fence. Ping gained payload sizing and bounded counts, while both `show ping` and `monitor ping` keep sending on schedule when replies are lost instead of serializing every probe behind its timeout.

The Birdwatcher-compatible Looking Glass response now includes `state_changed`, `last_error`, and live per-peer received, imported, and exported route counts. The fields feed Alice-LG directly rather than remaining empty placeholders.

## Management-plane security

Ze now refuses to start an unauthenticated web, MCP, gNMI, REST, or gRPC management listener on a non-loopback address. Wildcard binds and unparseable hosts fail closed. Reload also refuses to migrate an unauthenticated web, MCP, REST, or gRPC service onto a remote address and leaves the previous listener running.

The zefs super-admin bcrypt hash can only be used directly as a credential over loopback or a Unix socket. Remote SSH, web, REST, and gRPC clients must supply the plaintext password. Config displays mask bcrypt leaves, while the edit-authorized raw download retains the real hash for a byte-exact edit and upload cycle.

AAA paths now reject logins that resolve to no authorization profile. This includes RADIUS accepts without a usable profile, TACACS+ mappings to undefined profiles, and SSH-only configurations that previously skipped profile enforcement.

## IPsec, L2TP, and firewall reliability

EAP-TLS no longer loses a wakeup and deadlocks during the handshake. When a CA is configured, the EAP peer validates the authenticator certificate chain without relying on a DNS hostname, which EAP-TLS does not carry.

`clear vpn ipsec sa` sends an encrypted IKE Delete before local cleanup, then allows an immediate replacement exchange. Responders accept a new IKE_SA_INIT while the old SA is still being maintained, and INITIAL_CONTACT removes stale state after authentication.

The L2TP RADIUS plugin gained an optional `coa-port`. It remains disabled when unset, accepts requests only from configured RADIUS server addresses, and allows deployments to choose the local RFC 5176 listener port. Firewall reconciliation is now serialized so overlapping apply operations cannot deadlock or race the active ruleset.

## Quality and compliance

The RFC compliance gate now maps individual requirements to the tests that enforce them. A large enrollment pass added permanent requirement IDs and coverage for BGP, OSPF, IS-IS, RPKI, BMP, IPsec, L2TP, PPP, DHCP, DNS, flow export, and other implemented protocols. The RFC status page records partial support and known gaps rather than promoting a protocol from test coverage alone.

Fuzz targets are discovered automatically, reactor benchmarks have allocation ceilings in CI, and functional suites use isolated per-run binaries and scratch directories. Reproducible terminal demonstrations now exercise the real CLI and publish recordings, posters, and plain-text transcripts alongside the guides.
