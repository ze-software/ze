# Ze Features

Ze is a BGP daemon written in Go.

| Feature | Description |
|---------|-------------|
| [BGP Protocol](features/bgp-protocol.md) | 21 address families, 13 capabilities, 17 path attributes |
| [Configuration](features/configuration.md) | YANG-modeled config with prefix limits, update groups, session resilience |
| [Interfaces](features/interfaces.md) | Linux interface management via netlink: 6 types, DHCP, monitoring, migration, mirroring |
| [Plugins](features/plugins.md) | RIB, route server, graceful restart, RPKI, healthcheck, community filters, interface monitoring |
| Route Installation | FIB pipeline: protocol RIB best-path tracking, system RIB selection by admin distance, kernel route programming via netlink (RTPROT_ZE=250), crash recovery via stale-mark-sweep, external change monitoring |
<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- bestChangeEntry, publishBestChanges -->
<!-- source: internal/plugins/sysrib/sysrib.go -- sysribTopic, admin distance selection -->
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
| [Interoperability Testing](features/interoperability-testing.md) | 32 Docker-based scenarios against FRR, BIRD, GoBGP |
| [MCP Integration](features/mcp-integration.md) | AI-assisted BGP operations via Model Context Protocol |
| [DNS Resolver](features/dns-resolver.md) | Built-in cached DNS resolver for all components |
| Resolution CLI | Offline `ze resolve` tool for DNS, Team Cymru ASN names, PeeringDB prefix counts, and IRR AS-SET expansion |
| [ExaBGP Compatibility](features/exabgp-compatibility.md) | Automatic config migration and plugin bridge |
