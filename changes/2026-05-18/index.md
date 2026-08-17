# Week of 2026-05-18

A full native IPsec/IKEv2 VPN stack, a route server for IXPs, and a big allocation-hunting pass across the BGP hot path.

## 🔒 IPsec / IKEv2 VPN

Ze gained a native IKEv2 implementation built from the wire format up, interop-tested against strongSwan:

- IKEv2 crypto primitives and wire codec, with an FSM-driven engine handling auth, transport, and reconciliation
- Child SA negotiation, dead-peer detection, rekeying, and a dataplane abstraction
- EAP authentication (MSCHAPv2 and TLS), NAT-T, and a virtual IP address pool for remote-access clients
- Route-based VPN via a new XFRM interface type
- A PKI certificate store shared with TLS, with health checks and Prometheus metrics for certificate expiry
- CLI, web UI, health checks, and metrics for managing tunnels end to end

## 🛰️ Routing, BGP & subscriber access

- SRv6 Prefix-SID support (RFC 9252, RFC 8669)
- Recursive next-hop resolution, ECMP path grouping, and IGP-cost-aware best-path selection, feeding richer route attributes (type, metric, table, MPLS) into both the kernel and VPP FIB backends
- A route server mode for IXPs: dynamic peer groups that accept sessions from any address in a configured range, RS-client transparent AS-PATH forwarding, and community-based selective forwarding (announce-only, do-not-announce, RFC 7999 blackhole)
- AIGP attribute support (RFC 7311)
- PPPoE and L2TP subscriber sessions now share one session model. PPPoE is visible to the same auth, address-pool, shaping, and telemetry plugins L2TP already had.
- BGP FlowSpec routes (RFC 8955/8956) can now drive local firewall rules directly
- RPKI ASPA verification, shipped opt-in last week, can now enforce a reject policy on invalid AS_PATHs (log-only by default; a route that is ROA-valid but ASPA-invalid is still rejected)

## 🖥️ CLI & diagnostics

- A pure-Go packet capture command (`show capture interface`), an AF_PACKET-based tcpdump replacement for gokrazy appliances
- An mtr-style `monitor traceroute` with parallel per-hop probing, ECMP path awareness, and pipe support
- `monitor ping`, `monitor system` (live netlink event streaming), and nine other diagnostic commands built in for gokrazy boxes
- New pipe operators: `| origin` for ASN lookups and `| resolve` for reverse DNS, usable from traceroute and ping output
- CLI modes renamed to match standard NOS terminology: `configure` enters config mode, `exit` returns to operational mode instead of quitting
- `ze start --cli` launches an interactive CLI attached to the daemon in one step
- Machine-readable diagnostics for scripted or AI-assisted operations: `ze explain <code>`, `ze config validate --json`, `ze help --ai --json`

## 💿 Appliance, provisioning & readiness

- `ze install` restructured into `local` and `remote` subcommands, plus a new `ze uninstall`
- Zero-touch provisioning: PXE boot extensions on the DHCP server, a TFTP server, and an HTTP image server, all shipped as ordinary Ze plugins
- Self-update for the `ze` binary
- `ze doctor` gained config-driven readiness checks across the board (disk space, DNS resolver, interfaces, SSH/API listeners, web TLS, kernel modules) and a `--json` mode for scripted prerequisite checks before starting the daemon
- Config reload no longer drops connections: graceful, bind-before-close listener migration now covers the looking glass, REST, gRPC, and MCP servers, not just the web UI
- Config files now carry a schema stamp; if a config becomes incompatible after a downgrade, ze recovers automatically from the rollback history

## ⚡ Performance

A focused pass on BGP's hot path: attributes are parsed once per UPDATE instead of once per NLRI, AS-PATH prepend and hold-timer resets are allocation-free on the fast path, JSON formatting for monitors and RIB output skips work when nothing is watching, and GC pressure across the reactor and event pipeline dropped.

## 🛠️ Under the hood

ZeFS, Ze's internal storage layer, gained per-record CRC32c checksums, in-place writes, and a check/repair CLI. Remote fleet management got easier with per-service SSH credential storage and a `ze remote` CLI for targeting other instances. DNS resolver cache management (list, selective delete, flush, stats reset), firewall masquerade port mapping, additional conntrack helpers (NFS, SQL*Net), and per-interface rate tracking (CLI, Prometheus, web) also landed.
