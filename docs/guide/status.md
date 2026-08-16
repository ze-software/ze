# Project Status

Ze is in **early active development**. It is not yet released and should not be used in production. APIs, configuration syntax, and plugin interfaces may change without notice.

That said, brave souls who understand the risks may find Ze already useful for specific use cases, particularly if they are coming from ExaBGP and want a more capable architecture.

## What Works Today

### BGP Protocol

Ze establishes BGP sessions, exchanges routes, and handles the full FSM lifecycle. The wire layer supports encoding and decoding across all listed address families.

| Area | Status |
|------|--------|
| Session establishment (FSM) | Working -- tested with real peers and built-in test peers |
| OPEN capability negotiation | Working -- ASN4, ADD-PATH, Extended Messages, Extended Next-Hop, GR, Route Refresh, Role, Hostname, Software Version |
| UPDATE encode/decode | Working -- all address families have decode support; most have encode |
| NOTIFICATION handling | Working -- all standard codes including RFC 9003 shutdown communication |
| Keepalive / hold timer | Working |
| TCP MD5 authentication | Working on Linux and FreeBSD. Not available on macOS (kernel limitation). |
<!-- source: internal/component/bgp/reactor/ -- reactor event loop, FSM, wire layer -->

### Address Families

All families decode. Most encode. Use `ze --plugins` to see the current state.

| Family | Decode | Encode | Config Routes |
|--------|--------|--------|---------------|
| IPv4/IPv6 Unicast | Yes | Yes | Yes |
| IPv4/IPv6 Multicast | Yes | Yes | Yes |
| VPNv4/VPNv6 | Yes | Yes | Yes |
| EVPN | Yes | Yes | Yes |
| FlowSpec (IPv4/IPv6/VPN) | Yes | Yes | Yes |
| Labeled Unicast | Yes | Yes | Yes |
| BGP-LS | Yes | No | No |
| VPLS | Yes | Yes | Yes |
| MVPN | Yes | No | No |
| Route Target Constraint | Yes | No | No |
| Mobile User Plane | Yes | Yes | Yes |
<!-- source: internal/component/bgp/plugins/nlri/ -- NLRI plugin registrations with Families -->

### Plugins

The current binary reports 97 registered plugins and schemas covering protocol features, all address families, BFD, BMP, route filters, L2TP/PPP helpers, OSPF config wiring, firewall, traffic control, VPP, NTP, sysctl, FIB backends, route redistribution, and TACACS+/RADIUS admin AAA. The plugin lifecycle uses a 5-stage handshake, newline-framed YANG RPC IPC, and DirectBridge for internal hot paths. Plugins can be written in any language.
<!-- source: internal/component/plugin/all/all.go -- generated plugin imports -->

| Plugin | Status |
|--------|--------|
| bgp-rib (route storage) | Working -- stores received/sent routes, best-path selection |
| bgp-rs (route server) | Working -- forward-all model with zero-copy optimization |
| bgp-gr (graceful restart) | Working -- stale route retention, restart marker, timer expiry |
| bgp-rpki (RPKI validation) | Working -- RTR client, origin validation, fail-open safety |
| bgp-adj-rib-in | Working -- raw hex replay for convergent replay |
| bgp-route-refresh | Working -- RFC 2918 and RFC 7313 enhanced route refresh |
| role (RFC 9234) | Working -- OTC filtering, role mismatch detection |
| bgp-hostname, bgp-softver | Working -- capability advertisement |
| bgp-llnh | Working -- link-local next-hop handling |
| bgp-persist | Working -- route persistence across restarts |
| bgp-watchdog | Working -- deferred route announcement |
| All NLRI plugins (9) | Working -- decode and encode for their families |
| ospf | Partial -- config schema, validators, event namespace, raw-socket enrolment skeleton |
<!-- source: internal/plugins/ospf/register.go -- OSPF registry entry and lifecycle -->
<!-- source: internal/plugins/ospf/yang/ze-ospf-conf.yang -- OSPF config schema -->
<!-- source: internal/component/bgp/plugins/ -- all plugin register.go files -->

### Infrastructure

| Area | Status |
|------|--------|
| YANG configuration + validation | Working -- unknown keys rejected with suggestions |
| Interactive config editor | Working -- tab completion, rollback, live validation |
| Config reload (SIGHUP) | Working -- add/remove/update peers without restart |
| SSH-based CLI | Working -- interactive and single-command modes |
| Hierarchical logging | Working -- per-subsystem levels, runtime changes |
| ExaBGP config migration | Working -- auto-detect and convert |
| ExaBGP plugin bridge | Partial -- compatibility bridge exists, but not all ExaBGP behavior is equivalent |
| Chaos testing (ze-chaos) | Working -- deterministic replay, property validation |
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- YANG config; internal/component/config/cli/ -- config CLI; internal/component/cli/ -- interactive CLI; internal/core/slogutil/ -- hierarchical logging; internal/exabgp/ -- ExaBGP migration -->

### Test Suite

| Type | Count |
|------|-------|
| Unit test functions | 10,400+ |
| Functional test files (.ci) | 790+ |
| Fuzz targets | 57 |
| Linters | 26 |

Unit tests run with the race detector enabled (`-race`). Functional, browser, and compatibility suites do not currently use the race detector.
<!-- source: Makefile -- ze-unit-test, ze-functional-test, ze-lint, ze-fuzz-test targets -->

## What Does NOT Work Yet

### Not implemented

| Feature | Notes |
|---------|-------|
| MRT dump (RFC 6396) writing | Reading works via `ze-analyse`. Writing is not implemented. |
| Full BGP confederation behavior | AS path segment parsing exists, but full confederation deployment behavior is not listed as supported. |
| Flowspec redirect to VRF | 4-byte ASN with IP redirect (Type 0x82) extended community not yet supported. |
| External plugin filesystem discovery | External plugins must be configured. Built-in plugins auto-load by config roots, families, event types, and send types. |
| Multi-instance | Single daemon per config file. |

### Known rough edges

| Area | Issue |
|------|-------|
| ExaBGP compatibility | 17/37 ExaBGP compat tests still failing -- missing wire encoding for some attribute combinations |
| TTL security | Parsed from config but enforcement not yet wired to socket options |
| macOS MD5 | TCP MD5 auth returns "not supported" on Darwin (kernel limitation, not a bug) |
| Error messages | Some parse errors could be more specific about what went wrong |
| Packaging | Docker build support and install commands exist; binary release and package-manager workflows are still pre-release. |
<!-- source: internal/component/bgp/ -- FSM, wire layer, reactor implementation -->

### API stability

Nothing is stable yet. Expect changes in:

- Configuration syntax (YANG schemas evolving)
- JSON event format (field names and structure may change)
- Plugin IPC protocol (RPC names and payloads)
- CLI command names and output format
- Go package APIs (not intended for library use yet)

## Ze vs ExaBGP: When to Choose Which

### Choose Ze if you want

- **Plugin architecture** -- plugins in Go, Python, or any language, with typed IPC instead of stdin/stdout line parsing
- **RPKI validation** -- built-in RTR client with fail-open safety
- **YANG configuration** -- schema-validated config with tab completion and an interactive editor
- **Route server** -- zero-copy forwarding with ADD-PATH support
- **Graceful restart** -- proper RFC 4724 implementation with restart markers
- **Chaos testing** -- deterministic fault injection for validating your setup
- **Modern Go codebase** -- if you want to contribute or extend

### Choose ExaBGP if you need

- **Production stability** -- 15+ years of production use in networks worldwide
- **Community knowledge** -- large user base, extensive troubleshooting resources
- **Packaging** -- available via pip, Docker, and OS package managers
- **Complete wire encoding** -- handles more edge cases in attribute encoding
- **Proven integrations** -- well-documented integration patterns with monitoring and SDN systems

### Choose another routing suite if you need

- **Production-hardened OSPF/IS-IS** -- Ze ships experimental OSPFv2/OSPFv3 and IS-IS engines (SPF, LSDB, areas, redistribution), but they are pre-release and not yet proven across vendors in production.
- **Production-proven router functionality** -- Ze has FIB, static, connected, kernel, MPLS, LDP, RSVP-TE, firewall, interface, OSPF, IS-IS, and VPP code in tree, but it remains pre-release

## For the Brave

If you decide to use Ze today:

1. **Pin to a commit** -- there are no releases or version tags yet
2. **Run the test suite** -- `make ze-precommit-verify` before deploying any build
3. **Start with monitoring** -- use Ze to observe BGP sessions before relying on it for route injection
4. **Keep ExaBGP as fallback** -- Ze can migrate ExaBGP configs, so you can switch back easily
5. **Report issues** -- [github.com/ze-software/ze/issues](https://github.com/ze-software/ze/issues)
6. **Join early** -- feedback from real-world usage shapes the project more than any test suite
