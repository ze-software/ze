# Ze Features

Ze is a BGP daemon written in Go.

| Feature | Description |
|---------|-------------|
| [BGP Protocol](features/bgp-protocol.md) | 21 address families, 13 capabilities, 17 path attributes |
| [Configuration](features/configuration.md) | YANG-modeled config with prefix limits, update groups, session resilience |
| [Interfaces](features/interfaces.md) | Linux interface management via netlink: ethernet, dummy, veth, bridge, loopback, VLAN, 8 tunnel kinds (GRE, GRETAP, IP6GRE, IP6GRETAP, IPIP, SIT, IP6TNL, IPIP6), and WireGuard (declarative peers with `$9$`-encoded keys); DHCP, monitoring, migration, mirroring |
| [Plugins](features/plugins.md) | RIB, route server, graceful restart, RPKI, healthcheck, community filters, prefix-list filters, BMP (RFC 7854), interface monitoring |
<!-- source: internal/component/bgp/plugins/filter_prefix/filter_prefix.go -- bgp-filter-prefix, per-prefix partition modify path (cmd-4 phase 2) -->
| BFD Liveness Detection | RFC 5880 Bidirectional Forwarding Detection plugin: pinned single-hop (UDP 3784) and multi-hop (UDP 4784) sessions, profile-driven timer bundles, GTSM enforcement (IP_TTL=255 outbound / IP_RECVTTL ingress gate), multi-hop min-TTL floor, RFC 5880 §6.8.7 TX jitter (0-25%, clamped to [10%, 25%) when detect-multiplier=1), SO_BINDTODEVICE for single-hop interface and multi-VRF binding, BGP peer opt-in with RFC 9384 Cease subcode 10 teardown, `show bfd sessions/session/profile` commands, `ze_bfd_*` Prometheus metrics, RFC 5880 §6.7 Keyed SHA1/MD5 (meticulous variants included) authentication with file-backed sequence-number persistence, and RFC 5880 §6.4 Echo mode config/wire advertisement (transport half tracked as spec-bfd-6b-echo-transport). |
<!-- source: internal/plugins/bfd/engine/loop.go -- passesTTLGate, applyJitter -->
<!-- source: internal/plugins/bfd/transport/udp_linux.go -- applySocketOptions, parseReceivedTTL -->
<!-- source: internal/plugins/bfd/bfd.go -- runtimeState, resolveLoopDevices, newUDPTransport -->
<!-- source: internal/plugins/bfd/engine/snapshot.go -- Loop.Snapshot, SessionDetail -->
<!-- source: internal/plugins/bfd/metrics.go -- bindMetricsRegistry, metricsHook -->
| Modular Deployment | Config-driven plugin loading: BGP, interfaces, and FIB load only when their config section is present. Add or remove subsystems at runtime via config reload (SIGHUP). |
<!-- source: internal/component/plugin/server/startup_autoload.go -- autoLoadForNewConfigPaths, autoStopForRemovedConfigPaths -->
| Route Installation | FIB pipeline: protocol RIB best-path tracking, system RIB selection by admin distance, kernel route programming via netlink (RTPROT_ZE=250), crash recovery via stale-mark-sweep, external change monitoring |
<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- bestChangeEntry, publishBestChanges -->
<!-- source: internal/plugins/sysrib/sysrib.go -- system-rib topic, admin distance selection -->
<!-- source: internal/plugins/fibkernel/fibkernel.go -- fibKernel, netlink backend, stale sweep -->
| [CLI Commands](features/cli-commands.md) | Protocol tools, config management, schema discovery, daemon control, AS topology graph |
| [API Commands](features/api-commands.md) | Peer management, route updates, RIB operations, event subscription |
| [Configuration Reload](features/config-reload.md) | Live reload via SIGHUP with automatic reconciliation |
| [Fleet Management](features/fleet-management.md) | Centralized config distribution over TLS |
| [Performance Benchmarking](features/benchmarking.md) | Cross-implementation latency benchmarking with ze-perf |
| [Web Interface](features/web-interface.md) | HTTPS config editor with YANG-driven UI and CLI bar |
| [Looking Glass](features/looking-glass.md) | Public BGP looking glass with birdwatcher API and AS path graphs |
| [AI-First Design](features/ai-first.md) | Self-describing CLI-as-API with MCP transport for AI assistants |
| [Self-Documenting System](features/introspection.md) | Runtime introspection of plugins, env vars, RPCs, schemas, commands |
| Operational Report Bus | Cross-subsystem `ze show warnings` and `ze show errors` commands: single place to surface prefix-threshold crossings, stale route data, BGP NOTIFICATIONs sent/received, unexpected session drops. State-based warnings + event-based error ring, login banner reads the same source. |
<!-- source: internal/core/report/report.go -- Issue, RaiseWarning, RaiseError, Warnings, Errors -->
<!-- source: internal/component/cmd/show/show.go -- handleShowWarnings, handleShowErrors -->
<!-- source: internal/component/bgp/reactor/session_prefix.go -- raisePrefixThreshold, RaisePrefixStale, raiseNotificationError, raiseSessionDropped -->
<!-- source: internal/component/bgp/config/loader.go -- collectPrefixWarnings reads from report bus for login banner -->
| [Interoperability Testing](features/interoperability-testing.md) | 32 Docker-based scenarios against FRR, BIRD, GoBGP |
| REST/gRPC API | Programmatic API with OpenAPI 3.1 spec, SSE streaming, config sessions. Both transports accept multiple named listen endpoints via `environment.api-server.rest.server <name>` / `.grpc.server <name>`. Bearer token auth, CORS support. Both transports share one engine -- identical command output. |
<!-- source: internal/component/api/engine.go -- APIEngine, Execute, Stream, ListCommands -->
<!-- source: internal/component/api/rest/server.go -- RESTServer, all HTTP handlers, multi-listener Serve -->
<!-- source: internal/component/api/grpc/server.go -- GRPCServer, ZeService, ZeConfigService, multi-listener Serve -->
| Named Service Listeners | Every service that accepts inbound connections (web, ssh, mcp, looking-glass, telemetry, REST, gRPC, plugin hub) models its listen endpoints as a named YANG list. Each entry binds its own listener on the same subsystem; bind is all-or-nothing with rollback on failure. `CollectListeners` detects overlapping `ip:port` pairs at config parse time across every service. |
<!-- source: internal/component/config/yang/modules/ze-types.yang -- grouping listener -->
<!-- source: internal/component/config/yang/modules/ze-extensions.yang -- extension listener -->
<!-- source: internal/component/config/listener.go -- CollectListeners, ValidateListenerConflicts -->
| [MCP Integration](features/mcp-integration.md) | AI-assisted BGP operations via Model Context Protocol |
| [DNS Resolver](features/dns-resolver.md) | Built-in cached DNS resolver for all components |
| Resolution CLI | Offline `ze resolve` tool for DNS, Team Cymru ASN names, PeeringDB prefix counts, and IRR AS-SET expansion |
| [ExaBGP Compatibility](features/exabgp-compatibility.md) | Automatic config migration and plugin bridge |
