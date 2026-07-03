# Week of 2026-04-20

Security hardening, a new diagnostics subsystem, and a redesigned operator web UI headline this week.

## 🔒 Security & hardening

- SSH client connections now reject unknown remote hosts by default (insecure mode is opt-in), and managed-mode TLS verification is on by default
- Security headers (CSP, HSTS, clickjacking protection) now also apply to failed-login and unauthenticated responses, not just logged-in pages
- Per-user SSH public key authentication
- BGP correctness fixes: the OTC treat-as-withdraw path is now bounds-checked, best-path selection falls back to router ID when ORIGINATOR_ID is absent, and export filters no longer share mutable state across peers
- BFD detection timing now correctly follows RFC 5880's RemoteDesiredMinTx

## 📊 Observability & diagnostics

A new diagnostics subsystem shipped: structured log queries, a component health registry with a `/health` endpoint, packet capture rings for L2TP and BGP, active ping/route-lookup probes, and FSM history for BGP peers and L2TP tunnels/sessions. Netdata-compatible telemetry collectors now cover memory, disk, CPU, network, IPv6, and ZFS/btrfs/mdstat, each independently configurable.

## 🖥️ Web UI

A new RouterOS-style "workbench" operator UI landed behind a feature flag, with two-level navigation and per-domain dashboards. Embedded services (gokrazy console, L2TP, health) now render inside a portal iframe instead of separate tabs.

## 🛰️ Routing

- New policy-based routing plugin with next-hop actions
- New static route plugin: ECMP, weighted next-hops, BFD-triggered failover, and a VPP backend with redistribution

## ⚡ Route-server performance

Route-server forwarding now bypasses the plugin dispatch chain for a reactor-native fast path, with batched withdrawals and deferred flush. The event hot path switched from string keys to typed integer IDs, and the Loc-RIB is now sharded by prefix hash to cut lock contention.

## 🛠️ Under the hood

MCP gained elicitation (interactive prompts mid tool-call) and OAuth 2.1 remote binding. A new `tcp-mss-set` firewall action clamps TCP MSS. Static DNS name servers are now configurable, with resolver config unified.
