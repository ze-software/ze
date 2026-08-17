# Week of 2026-03-23

A full web interface, fleet management, redistribution filtering, and a stack of routing-protocol correctness work all landed together.

## 🖥️ Web interface

A browser-based config editor shipped alongside the existing CLI and SSH editor:

- YANG-driven rendering with per-user drafts, inline diffs, and live SSE updates
- Admin commands, finder-style navigation, and a light/dark theme toggle
- Started with `ze start --web`
- Hardened against DoS, SSE injection, and XSS (capped request bodies, tightened CSP, sanitized SSE event types)
- The web server now refuses to start without blob storage backing it, so TLS keys can't leak to the filesystem

## 🧩 Fleet management

A managed/fleet-config system landed: named hub blocks with per-client authentication, a managed client with first-boot TLS bootstrap, duplicate-client rejection, and CLI flags for managed mode. A client stops itself automatically if `managed` is turned off in its config.

## 🔌 Redistribution filtering

External plugins can now filter and modify routes on ingress and egress: a YANG-configured filter chain with piped transforms and reject short-circuit, wired into the reactor's forwarding path, shipped with a working community filter plugin.

## 🔒 Plugin and session security

- Plugin TLS hardened with per-plugin tokens, certificate pinning, and secret clearing from the environment
- The ExaBGP compatibility bridge gained a TLS connect-back mode for engine-launched plugins
- A new plugin debug shell inspects a running plugin over an SSH channel
- Two new subsystems: a cached DNS resolver component (used by decorators such as Team Cymru lookups), and an MCP server exposing six tools for AI-assisted BGP operations (announce, withdraw, peers, peer control, execute, commands)

## 🛰️ Routing correctness

- Route loop detection: AS-path, ORIGINATOR_ID, and CLUSTER_LIST loops are caught and silently withdrawn before reaching prefix limits or plugins (RFC 4271 §9, RFC 4456 §8)
- Hold timer split in two per RFC 9687: `receive-hold-time` (the classic RFC 4271 timer) and `send-hold-time` (auto by default)
- Fixed peers configured with only a per-peer local-as sending AS_PATH with ASN 0, which other implementations correctly reject under RFC 7607
- Graceful TCP close now drains before closing, so a pending NOTIFICATION can't be lost to an RST
- Junos-style `inactive`/`deactivate` config: mark any config block inactive without deleting it, with matching `show | active` / `show | inactive` filters
- Update groups now build one UPDATE per group of peers sharing identical attributes instead of one per peer

## 📊 Observability and prefix hygiene

- Prometheus instrumentation expanded with histograms, session lifecycle, wire metrics, and plugin health (status, restarts, delivery)
- New prefix data infrastructure pulls maximum-prefix guidance from PeeringDB and IRR/RIR delegation data, tracks staleness, and warns on stale data at login and in `show`
- Forward-pool congestion handling gained weight-based auto-sizing and two-threshold backpressure: soft buffer denial for the worst-offending peer, then forced GR-aware session teardown if a peer keeps hogging the pool

## ⚡ Performance and benchmarking

TCP_NODELAY is now enabled on production BGP sessions, and outgoing packets get DSCP CS6 marking. The `ze-perf` benchmarking tool grew report generation and gained more implementations to compare against: RustyBGP, freeRtr, and GoBGP alongside BIRD.
