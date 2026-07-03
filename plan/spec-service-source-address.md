# Spec: service-source-address

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 7/7 |
| Updated | 2026-07-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/core/network/network.go` - RealDialer abstraction
4. `internal/component/tacacs/client.go:488-494` - TACACS+ source-address reference pattern
5. `internal/component/bgp/plugins/bmp/sender.go:88` - BMP bare dialer (primary target)
6. `internal/component/bgp/plugins/rpki/rtr_session.go:118` - RPKI bare dialer
7. `internal/plugins/flowexport/sender.go:69` - FlowExport bare UDP dialer
8. `internal/plugins/ldp/register.go:431` - LDP bare dialer (transport-address not bound)
9. `internal/component/resolve/irr/client.go:236` - IRR bare dialer
10. `internal/component/managed/client.go:101` - Managed bare TLS dialer

## Task

Ze's outbound services inconsistently support source-address binding. Operators who
deploy Ze with loopback or management VRF interfaces need every service's outbound
connections to originate from a specified source address. Today, only 3 of 10+
operator-relevant outbound services support this.

Additionally, `network.RealDialer` (`internal/core/network/network.go:54-70`) provides
a shared dialer abstraction with `LocalAddr` support but only BGP uses it. Other services
create bare `net.Dialer` instances directly. This spec generalizes `RealDialer` as the
standard outbound TCP dialer for all Ze services.

**Origin:** VyOS vyos-1x T9013 (2026-06, BMP source-interface) exposed a gap that
extends far beyond BMP. This spec replaces `spec-bmp-source-interface.md` and covers
all outbound services.

## Required Reading

### Architecture Docs
- [ ] `internal/core/network/network.go` - RealDialer abstraction with LocalAddr
  -> Constraint: `RealDialer` implements `network.Dialer` interface. When MD5Key="" and OutTTL=0, no Control callback is set, so it behaves identically to `net.Dialer{LocalAddr}`. Lacks `Timeout` field (inner `net.Dialer{}` has zero timeout). Adding `Timeout` is backwards-compatible.
- [ ] `ai/rules/config-surface.md` - YANG vs env var decision
  -> Decision: source-address is operator-facing config (capacity planning, traffic engineering). YANG config, not env var.
- [ ] `ai/rules/config-naming.md` - naming conventions
  -> Constraint: kebab-case, no abbreviations, PascalCase Go field. `source-address` -> `SourceAddress`.
- [ ] `ai/patterns/config-option.md` - structural template for config options
  -> Constraint: YANG leaf with `type zt:ip-address`, description, no default (optional leaf). Go struct field + JSON tag. No env var (not under environment/).
- [ ] `ai/rules/plugin-self-containment.md` - plugin owns its config leaves
  -> Constraint: each plugin adds its own `source-address` leaf in its own YANG module.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7854.md` - BMP (BGP Monitoring Protocol)
  -> Constraint: BMP sender connects to collector. No RFC requirement on source-address, but operator practice for loopback binding.
- [ ] `rfc/short/rfc8210.md` - RPKI-to-Router Protocol (RTR)
  -> Constraint: RTR client connects to cache server. Same source-binding need.
- [ ] `rfc/short/rfc5036.md` - LDP (Label Distribution Protocol)
  -> Constraint: Transport address advertised in Hello determines TCP session. Device should bind to its own transport-address when initiating. Not doing so is arguably a bug.

**Key insights:**
- `network.RealDialer` (`network.go:54-70`) already supports `LocalAddr *net.TCPAddr` but only BGP uses it. Adding `Timeout time.Duration` makes it a complete `net.Dialer` replacement.
- TACACS+ (`client.go:488-494`) is the reference pattern: `source-address` leaf with type `zt:ip-address`, parsed to `net.Dialer.LocalAddr`.
- Each service's YANG model has a server/collector list where `source-address` can be added as an optional leaf.
- LDP's `transport-address` already exists in YANG but is only used in Hello messages, not bound to the TCP dialer. This is a wiring fix, not a new leaf.
- IRR queries have two paths: CLI (`ze resolve irr --server`) and BGP filter plugin (`bgp > policy > irr > server` in `ze-filter-irr.yang`). Source-address goes in the YANG-configured path (filter plugin).
- Managed client config is under `plugin > hub > client` in `ze-plugin-conf.yang`.

## Service Inventory (RESEARCH)

### Services WITH source-address binding today

| Service | YANG leaf | Go mechanism | File |
|---------|-----------|-------------|------|
| BGP peers | `local-address` (mandatory) | `network.RealDialer.LocalAddr` | `bgp/reactor/session.go:369` |
| TACACS+ | `source-address` (`zt:ip-address`) | Inline `net.Dialer.LocalAddr` | `tacacs/client.go:490` |
| RADIUS | `source-address` (`zt:ip-address`) | `net.UDPAddr{IP: cfg.SourceAddress}` | `radius/client.go:71` |
| L2TP RADIUS | `source-address` (`zt:ip-address`) | Via RADIUS client | `l2tp/plugins/authradius/config.go:24` |
| IPsec/IKE | `local-address` | Tunnel local endpoint | `ipsec/yang/ze-ipsec-conf.yang:320` |

### Services to add source-address binding (IN SCOPE)

| Service | Current dialer | File | Protocol | YANG location |
|---------|---------------|------|----------|---------------|
| BMP | `net.Dialer{Timeout: 10s}` | `bmp/sender.go:88` | TCP | `bgp > bmp > sender > collector` list |
| RPKI/RTR | `net.Dialer{Timeout: 30s}` | `rpki/rtr_session.go:118` | TCP | `bgp > rpki > cache-server` list |
| FlowExport | `net.DialUDP(net, nil, addr)` | `flowexport/sender.go:69` | UDP | `flow-export > collector` list |
| LDP | `net.Dialer{Timeout: 5s}` | `ldp/register.go:431` | TCP | existing `transport-address` leaf |
| IRR | `net.Dialer{Timeout: c.timeout}` | `resolve/irr/client.go:236` | TCP | `bgp > policy > irr` container |
| Managed | `tls.Dialer{Config: tlsConf}` | `managed/client.go:101` | TLS | `plugin > hub > client` list |

### Excluded

| Service | Why |
|---------|-----|
| Signal | CLI tool, dials localhost SSH. Not a service. Source-address irrelevant. |
| Appliance (crypto, config push, init) | Internal connections to local endpoints. |
| Analyse tools (replay, inject, export_bmp) | Dev/debug tools, not operator-configured. |

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/network/network.go` - RealDialer: LocalAddr, PeerAddr, MD5Key, OutTTL. No Timeout field. DialContext creates `net.Dialer{}`, sets LocalAddr if non-nil, adds Control callback only if MD5Key/OutTTL set.
  -> Constraint: Adding Timeout must preserve zero-value behavior (no timeout = context-controlled).
- [ ] `internal/component/bgp/plugins/bmp/sender.go:80-99` - reconnect loop: `(&net.Dialer{Timeout: 10s}).DialContext(ss.stopCtx, "tcp", addr)`. No source binding. Config comes from `collectorConfig{Address, Port}` in bmp.go:86-89.
  -> Constraint: Must preserve reconnect loop, backoff logic, stopCtx cancellation.
- [ ] `internal/component/bgp/plugins/bmp/bmp.go:86-89` - `collectorConfig{Address string, Port string}`. JSON tags match YANG leaf names.
  -> Constraint: Add `SourceAddress string` with `json:"source-address"` tag.
- [ ] `internal/component/bgp/plugins/rpki/rtr_session.go:116-119` - `connectAndSync()`: `&net.Dialer{Timeout: 30s}` then `dialer.Dial("tcp", addr)`. No context used (uses `Dial` not `DialContext`).
  -> Constraint: Switch to DialContext with context from stopCh for proper cancellation.
- [ ] `internal/component/bgp/plugins/rpki/rtr_session.go:29-61` - `RTRSession` struct has address, port, preference. Constructor `NewRTRSession(address, port, pref, cache, aspaCache, stopCh)`.
  -> Constraint: Add sourceAddress parameter to constructor.
- [ ] `internal/plugins/ldp/register.go:427-432` - `tcpAddr` built from `adj.TransportAddr` (peer address). Bare `net.Dialer{Timeout: 5s}`.
  -> Constraint: `cfg.TransportAddr` (local device's transport address, used in Hello messages) should be bound as LocalAddr. This is the same address advertised in LDP Hello, so binding it to the dialer is RFC-correct.
- [ ] `internal/plugins/flowexport/sender.go:59-72` - `NewSender(address, port)`: resolves address, creates `&net.UDPAddr{IP, Port}`, calls `net.DialUDP(network, nil, udpAddr)`. Second arg nil = no source binding.
  -> Constraint: Change NewSender signature to accept optional source address. Pass as first UDPAddr arg to DialUDP.
- [ ] `internal/component/resolve/irr/client.go:233-237` - `query(ctx, command)`: `net.Dialer{Timeout: c.timeout}`, `dialer.DialContext(ctx, "tcp", c.server)`.
  -> Constraint: `IRR` struct has `server` and `timeout` fields. Add `sourceAddress` field. IRR created by `NewIRR(server)` in client.go and by filter-irr plugin at `filter_irr.go:183`.
- [ ] `internal/component/managed/client.go:90-104` - `runConnection()`: creates `tls.Config`, then `(&tls.Dialer{Config: tlsConf}).DialContext(connectCtx, "tcp", cfg.Server)`.
  -> Constraint: Split into RealDialer TCP connect + tls.Client() + HandshakeContext(). Preserves TLS 1.3 minimum, InsecureSkipVerify opt-in, connect timeout.
- [ ] `internal/component/tacacs/client.go:488-494` - Reference pattern: `dialer := &net.Dialer{Timeout: c.config.Timeout}; if c.config.SourceAddress != "" { dialer.LocalAddr = &net.TCPAddr{IP: net.ParseIP(c.config.SourceAddress)} }`. YANG leaf `source-address` type `zt:ip-address`.

**Behavior to preserve:**
- All services connect without source-address when unconfigured (default OS routing)
- Existing source-address behavior in TACACS+, RADIUS, BGP unchanged
- Reconnect/retry logic in each service unchanged
- BMP reconnect backoff (min/max exponential)
- RPKI retry interval and version negotiation
- LDP adjacency discovery and session establishment
- FlowExport multi-collector support
- Managed heartbeat/notification loop
- IRR cache and per-query timeout

**Behavior to change:**
- `network.RealDialer` gains `Timeout time.Duration` field
- BMP, RPKI, LDP, IRR: replace `net.Dialer` with `network.RealDialer`
- FlowExport: pass source `*net.UDPAddr` to `net.DialUDP`
- Managed: TCP via `network.RealDialer`, then TLS upgrade via `tls.Client()`
- Each service's config struct gains `SourceAddress` field (except LDP which reuses `TransportAddr`)

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- YANG config: `<service> ... source-address <addr>` per service (LDP uses existing `transport-address`)

### Transformation Path
1. YANG config parsed by each service's config loader into struct field `SourceAddress string`
2. Service reads `SourceAddress` during connection setup (dial function or constructor)
3. For TCP: `network.RealDialer{LocalAddr: &net.TCPAddr{IP: net.ParseIP(sourceAddr)}, Timeout: ...}`
4. For UDP: `net.DialUDP(network, &net.UDPAddr{IP: net.ParseIP(sourceAddr)}, dstAddr)`
5. For TLS: `network.RealDialer` for TCP, then `tls.Client(conn, tlsConf)` + `HandshakeContext(ctx)`
6. Connection established with specified source

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> each service | YANG tree -> JSON -> Go struct field | [ ] |
| Each service -> network package | `network.RealDialer` (TCP) or `net.DialUDP` (UDP) | [ ] |
| Network -> OS | `net.Dialer.LocalAddr` or `net.UDPAddr` source binding | [ ] |

### Integration Points
- `internal/core/network/network.go` - shared RealDialer abstraction (add Timeout)
- Per-service YANG modules - add source-address leaf
- Per-service config structs - add SourceAddress field
- Per-service dial functions - switch to RealDialer or bind source

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing RealDialer, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding (each plugin owns its YANG leaf)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `source-address` (IP) is sufficient; `source-interface` is out of scope | User confirmed scope | Follow-up spec for interface resolution | User approval in design gate | confirmed |
| A-2 | Each service's YANG model can accept a `source-address` leaf without schema conflicts | Read all 6 YANG models; all have list/container structures that accept new leaves | Restructure YANG | Direct read of each YANG file | confirmed |
| A-3 | No VRF-aware dialing needed (standard `LocalAddr` binding sufficient) | Ze runs in default namespace; VRF support is not implemented | Would need SO_BINDTODEVICE | Check Ze VRF support | confirmed (out of scope, documented in Known Limitations) |
| A-4 | Adding `Timeout` to `RealDialer` is backwards-compatible | Zero value means "no timeout" which matches current behavior (`net.Dialer{}.Timeout` is zero) | BGP dialer behavior changes if Timeout accidentally set | Unit test: RealDialer with Timeout=0 behaves identically to current | confirmed (`TestRealDialerTimeoutZero`; BGP suite green) |
| A-5 | Splitting Managed's `tls.Dialer.DialContext()` into TCP+TLS upgrade preserves identical behavior | Go stdlib `tls.Dialer.DialContext()` does TCP dial then `tls.Client()` + Handshake internally | Subtle TLS behavior differences | Compare Go source; write test against real hub | **REFUTED** -- `tls.Dialer` infers ServerName (tls.go:160-166), `tls.Client` does not; split dropped hostname verification. Fixed via `serverNameFromAddr`; `TestManagedTLSServerName` |
| A-6 | LDP should bind `transport-address` as TCP source | RFC 5036: transport address is used for TCP session. Not binding it means outgoing TCP may use a different source than advertised in Hello. | If some deployments rely on OS source selection | Review RFC 5036 transport address semantics | confirmed (conditional bind: only when `IsValid()`; `TestLDPTransportAddressBinding`) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Source address unavailable after interface change | Dial fails with EADDRNOTAVAIL | Clear error log; service retries on reconnect cycle |
| R-2 | UDP source binding (FlowExport) may cause unexpected ICMP errors | FlowExport packets not received by collector | Same as TCP source binding; log and continue |
| R-3 | Managed TLS upgrade change introduces subtle connection issues | Managed client fails to connect to hub | Test with real hub; fallback: revert to inline tls.Dialer with net.Dialer.LocalAddr |
| R-4 | LDP transport-address binding breaks existing deployments where OS source selection worked | LDP sessions fail to establish | Make binding conditional: only bind when transport-address is configured |

## Wiring Test (MANDATORY)
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| YANG config `bgp { bmp { sender { collector X { source-address Y } } } }` | -> | `network.RealDialer.LocalAddr` set in BMP `sender.go` | `TestBMPCollectorSourceAddress` |
| YANG config `bgp { rpki { cache-server X { source-address Y } } }` | -> | `network.RealDialer.LocalAddr` set in RPKI `rtr_session.go` | `TestRPKISourceAddress` |
| YANG config `flow-export { collector X { source-address Y } }` | -> | `net.UDPAddr` source set in FlowExport `sender.go` | `TestFlowExportSourceAddress` |
| YANG config `ldp { transport-address Y }` | -> | `network.RealDialer.LocalAddr` set in LDP `register.go` | `TestLDPTransportAddressBinding` |
| YANG config `bgp { policy { irr { source-address Y } } }` | -> | `network.RealDialer.LocalAddr` set in IRR `client.go` | `TestIRRSourceAddress` |
| YANG config `plugin { hub { client X { source-address Y } } }` | -> | `network.RealDialer.LocalAddr` set in Managed `client.go` | `TestManagedSourceAddress` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `network.RealDialer` with `Timeout` set | Dial respects timeout (same as `net.Dialer.Timeout`) |
| AC-2 | `network.RealDialer` with `Timeout` zero | Dial uses context deadline only (unchanged from current) |
| AC-3 | BMP collector with `source-address` configured | TCP connection to collector uses specified source |
| AC-4 | BMP collector without `source-address` | TCP connection uses OS-selected source (unchanged) |
| AC-5 | RPKI cache-server with `source-address` configured | TCP connection to cache uses specified source |
| AC-6 | RPKI cache-server without `source-address` | TCP connection uses OS-selected source (unchanged) |
| AC-7 | FlowExport collector with `source-address` configured | UDP datagrams use specified source |
| AC-8 | FlowExport collector without `source-address` | UDP datagrams use OS-selected source (unchanged) |
| AC-9 | LDP with `transport-address` configured | TCP session binds to transport-address |
| AC-10 | LDP without `transport-address` configured | TCP session uses OS-selected source (unchanged) |
| AC-11 | IRR filter with `source-address` configured | WHOIS queries use specified source |
| AC-12 | IRR filter without `source-address` | WHOIS queries use OS-selected source (unchanged) |
| AC-13 | Managed hub client with `source-address` configured | TLS connection to hub uses specified source |
| AC-14 | Managed hub client without `source-address` | TLS connection uses OS-selected source (unchanged) |
| AC-15 | Invalid `source-address` value in any service | Config validation rejects (YANG `zt:ip-address` type check) |
| AC-16 | Existing TACACS+/RADIUS/BGP source-address configs | No regression |

## End-to-End User Stories (MANDATORY for new features)
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures BMP collector with source-address on loopback | YANG -> collectorConfig.SourceAddress -> RealDialer.LocalAddr -> TCP SYN from loopback IP | `TestBMPCollectorSourceAddress` |
| 2 | Configures RPKI cache-server with source-address | YANG -> RTRSession.sourceAddress -> RealDialer.LocalAddr -> TCP connect from specified IP | `TestRPKISourceAddress` |
| 3 | Configures FlowExport collector with source-address | YANG -> CollectorConfig.SourceAddress -> net.DialUDP source bind -> UDP from specified IP | `TestFlowExportSourceAddress` |
| 4 | Configures LDP transport-address | YANG -> ldpConfig.TransportAddr -> RealDialer.LocalAddr -> TCP from transport-address | `TestLDPTransportAddressBinding` |
| 5 | Configures IRR source-address for BGP filter | YANG -> IRR.sourceAddress -> RealDialer.LocalAddr -> WHOIS from specified IP | `TestIRRSourceAddress` |
| 6 | Configures managed hub client with source-address | YANG -> ClientConfig.SourceAddress -> RealDialer TCP + tls.Client -> TLS from specified IP | `TestManagedSourceAddress` |
| 7 | Service without source-address | Default dial, OS picks source | Existing tests (no regression) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| YANG leaf name `source-address` | `local-address` (BGP/IPsec convention) | TACACS+/RADIUS established `source-address` for infrastructure services. BGP `local-address` is mandatory per-peer and has different semantics (listener binding). `source-address` is the right name for optional outbound source binding. |
| Generalize `network.RealDialer` with `Timeout` | Keep inline `net.Dialer` per service (TACACS+ pattern) | User directive. RealDialer with zero MD5/TTL has no overhead vs bare `net.Dialer`. Adding Timeout makes it a complete drop-in. Centralizes future socket options (e.g., SO_BINDTODEVICE). |
| Per-service YANG leaf (no shared grouping) | Shared `grouping source-binding` | Each service has different config structure (some use lists, some containers). A shared grouping would require `uses` in each YANG module anyway, and the leaf is one line. No abstraction needed. |
| FlowExport inline UDP binding | Add UDP support to network package | FlowExport is the only UDP outbound service. An abstraction for one user is premature. |
| LDP: wire existing `transport-address` | Add separate `source-address` leaf | LDP RFC 5036 says transport-address IS the TCP session binding address. Adding a separate leaf would create two conflicting source concepts. |
| Managed: TCP via RealDialer + tls.Client() upgrade | net.Dialer with LocalAddr passed to tls.Dialer.NetDialer | `tls.Dialer.NetDialer` is `*net.Dialer`, not an interface. Cannot substitute RealDialer. TCP+TLS split is the standard Go pattern and is what `tls.Dialer` does internally. |
| IRR: source-address on filter-irr YANG, not CLI | CLI `--source` flag on `ze resolve irr` | Production IRR usage is through the filter-irr BGP plugin (YANG-configured). CLI is ad-hoc diagnostics. Source-address matters for persistent automated queries. |

## Known Limitations
- Covers `source-address` (IP address) only; `source-interface` (resolve interface name to address) is a potential follow-up
- Does not change services that already have source-address (TACACS+, RADIUS, BGP); those are reference patterns only
- Signal plugin excluded (CLI tool, always dials localhost)
- Appliance-internal and dev/debug tool connections excluded
- No VRF-aware dialing (SO_BINDTODEVICE); standard LocalAddr binding only

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRealDialerTimeout` | `internal/core/network/network_test.go` | RealDialer with Timeout passes it to inner net.Dialer | |
| `TestRealDialerTimeoutZero` | `internal/core/network/network_test.go` | RealDialer with Timeout=0 behaves identically to current (no timeout) | |
| `TestBMPCollectorSourceAddress` | `internal/component/bgp/plugins/bmp/bmp_test.go` | collectorConfig parses source-address from JSON | |
| `TestRPKISourceAddress` | `internal/component/bgp/plugins/rpki/rpki_test.go` | RTRSession created with source address, used in dialer | |
| `TestFlowExportSourceAddress` | `internal/plugins/flowexport/sender_test.go` | NewSender binds UDP source when address provided | |
| `TestLDPTransportAddressBinding` | `internal/plugins/ldp/ldp_test.go` | Transport address passed to dialer as LocalAddr | |
| `TestIRRSourceAddress` | `internal/component/resolve/irr/client_test.go` | IRR client uses source address in dialer | |
| `TestManagedSourceAddress` | `internal/component/managed/client_test.go` | Managed client binds source address on TCP before TLS upgrade | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| source-address | Valid IP (`zt:ip-address`) | Any valid IPv4/IPv6 | Empty string = not configured | N/A (YANG type validates) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `source-address-bmp` | `test/parse/source-address-bmp.ci` | BMP collector config with source-address parses correctly | |
| `source-address-rpki` | `test/parse/source-address-rpki.ci` | RPKI cache-server config with source-address parses correctly | |
| `source-address-flowexport` | `test/parse/source-address-flowexport.ci` | FlowExport collector config with source-address parses correctly | |
| `source-address-irr` | `test/parse/source-address-irr.ci` | IRR filter config with source-address parses correctly | |
| `source-address-managed` | `test/parse/source-address-managed.ci` | Managed hub client config with source-address parses correctly | |

### Interop Tests (MANDATORY for protocol features)
Not applicable. Source-address binding is a local socket option that does not change wire protocol behavior. No new interop scenarios needed.

### Future (if deferring any tests)
- Integration tests with actual service connections (BMP to collector, RPKI to cache) require running daemons. Unit tests with listener verify the source address binding. Full integration deferred to ze-test scenarios.

## Files to Modify
- `internal/core/network/network.go` - Add Timeout field to RealDialer
- `internal/core/network/network_test.go` - Test Timeout behavior
- `internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang` - Add source-address to collector list
- `internal/component/bgp/plugins/bmp/bmp.go` - Add SourceAddress to collectorConfig
- `internal/component/bgp/plugins/bmp/sender.go` - Use RealDialer with LocalAddr
- `internal/component/bgp/plugins/rpki/yang/ze-rpki.yang` - Add source-address to cache-server list
- `internal/component/bgp/plugins/rpki/rtr_session.go` - Add sourceAddress field, use RealDialer
- `internal/plugins/flowexport/yang/ze-flowexport-conf.yang` - Add source-address to collector list
- `internal/plugins/flowexport/config.go` - Add SourceAddress to CollectorConfig
- `internal/plugins/flowexport/sender.go` - Accept and bind source address
- `internal/plugins/flowexport/exporter.go` - Pass source address to NewSender
- `internal/plugins/ldp/register.go` - Bind cfg.TransportAddr to dialer LocalAddr
- `internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr.yang` - Add source-address to irr container
- `internal/component/bgp/plugins/filter_irr/filter_irr.go` - Pass source-address to IRR constructor
- `internal/component/resolve/irr/client.go` - Add sourceAddress field, use RealDialer
- `internal/component/plugin/yang/ze-plugin-conf.yang` - Add source-address to hub client list
- `internal/component/plugin/types.go` - Add SourceAddress to HubClientConfig
- `internal/component/config/loader_extract.go` - Parse source-address for hub client
- `internal/component/managed/client.go` - Add SourceAddress to ClientConfig, split TCP+TLS
- `cmd/ze/ze_core_start.go` - Pass SourceAddress from hub client config to managed ClientConfig

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | Yes | 5 YANG files listed above |
| YANG validation constraints | Yes | `type zt:ip-address` provides format validation |
| YANG custom validators | No | `zt:ip-address` is sufficient |
| CLI commands/flags | No | No new CLI commands |
| CLI grammar (action before identifier) | No | No new CLI commands |
| Editor autocomplete | No | IP address input, no completion needed |
| Functional test for new config | Yes | 5 parse tests in `test/parse/` |
| Pipe completeness | No | No new command output |
| Env var registration | No | Not under environment/ |
| Doctor check for runtime dependencies | No | No new runtime dependencies |
| Prometheus counters/metrics | No | No new observable state |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - add source-address to service features |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` - add source-address examples |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | Existing plugins extended, not new |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | Source-address is operational, not protocol |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - if source-address is a comparison point |
| 12 | Internal architecture changed? | No | RealDialer extended, not architecturally changed |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin inventory changed? | No | |
| 16 | Any changed source file referenced by doc anchors? | [ ] | Check with grep |
| 17 | Existing docs show config examples for this area? | [ ] | Check with grep |

## Files to Create
- `test/parse/source-address-bmp.ci` - BMP source-address parse test
- `test/parse/source-address-rpki.ci` - RPKI source-address parse test
- `test/parse/source-address-flowexport.ci` - FlowExport source-address parse test
- `test/parse/source-address-irr.ci` - IRR source-address parse test
- `test/parse/source-address-managed.ci` - Managed source-address parse test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-10. Fix/re-verify loop | |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Core (RealDialer Timeout)** - Add Timeout field to RealDialer
   - Tests: `TestRealDialerTimeout`, `TestRealDialerTimeoutZero`
   - Files: `network.go`, `network_test.go`
   - Verify: existing BGP tests still pass (zero Timeout = unchanged behavior)

2. **Phase: BMP** - Add source-address to BMP collector
   - Tests: `TestBMPCollectorSourceAddress`, `source-address-bmp.ci`
   - Files: `ze-bmp-conf.yang`, `bmp.go`, `sender.go`
   - Verify: BMP with and without source-address both work

3. **Phase: RPKI** - Add source-address to RPKI cache-server
   - Tests: `TestRPKISourceAddress`, `source-address-rpki.ci`
   - Files: `ze-rpki.yang`, `rtr_session.go`
   - Verify: RPKI with and without source-address both work

4. **Phase: FlowExport** - Add source-address to FlowExport collector
   - Tests: `TestFlowExportSourceAddress`, `source-address-flowexport.ci`
   - Files: `ze-flowexport-conf.yang`, `config.go`, `sender.go`, `exporter.go`
   - Verify: FlowExport with and without source-address both work

5. **Phase: LDP** - Wire transport-address to dialer
   - Tests: `TestLDPTransportAddressBinding`
   - Files: `register.go`
   - Verify: LDP with transport-address binds correctly; without, unchanged

6. **Phase: IRR** - Add source-address to IRR filter config
   - Tests: `TestIRRSourceAddress`, `source-address-irr.ci`
   - Files: `ze-filter-irr.yang`, `filter_irr.go`, `client.go`
   - Verify: IRR filter with and without source-address both work

7. **Phase: Managed** - Add source-address to managed hub client
   - Tests: `TestManagedSourceAddress`, `source-address-managed.ci`
   - Files: `ze-plugin-conf.yang`, `types.go`, `loader_extract.go`, `client.go`, `ze_core_start.go`
   - Verify: Managed client with and without source-address both work; TLS upgrade preserves behavior

8. **Functional tests** -> Create parse tests for all services
9. **Documentation** -> Update features.md, configuration guide
10. **Full verification** -> `make ze-verify`
11. **Complete spec** -> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | All 6 services have source-address binding |
| Correctness | Source-address binding only when configured; nil/empty = OS default |
| Naming | YANG: `source-address`, kebab-case. Go: `SourceAddress`, PascalCase |
| Data flow | Config -> struct -> dialer. No shortcuts or bypassed layers |
| Registration over hardcoding | Each plugin adds its own YANG leaf in its own module |
| YANG validation | `type zt:ip-address` on every new leaf |
| Rule: no-layering | RealDialer extended, not wrapped. No new abstraction layer |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| RealDialer has Timeout field | `grep 'Timeout.*time.Duration' internal/core/network/network.go` |
| BMP source-address YANG leaf | `grep 'source-address' internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang` |
| RPKI source-address YANG leaf | `grep 'source-address' internal/component/bgp/plugins/rpki/yang/ze-rpki.yang` |
| FlowExport source-address YANG leaf | `grep 'source-address' internal/plugins/flowexport/yang/ze-flowexport-conf.yang` |
| IRR source-address YANG leaf | `grep 'source-address' internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr.yang` |
| Managed source-address YANG leaf | `grep 'source-address' internal/component/plugin/yang/ze-plugin-conf.yang` |
| LDP transport-address bound to dialer | `grep 'LocalAddr.*TransportAddr\|RealDialer' internal/plugins/ldp/register.go` |
| All 6 unit tests pass | `make ze-unit-test` |
| All 5 parse tests exist | `ls test/parse/source-address-*.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | `zt:ip-address` YANG type validates format. No additional validation needed at Go layer (ParseIP handles it). |
| Source address spoofing | OS enforces source address ownership. Binding to an address not on the device fails with EADDRNOTAVAIL. No security concern. |
| TLS upgrade (Managed) | Verify TLS 1.3 minimum preserved. Verify InsecureSkipVerify only when explicitly configured. |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| YANG parse error | Check YANG syntax; verify zt:ip-address type exists |
| Existing service tests break | Check that zero-value SourceAddress preserves existing behavior |
| Managed TLS connection fails | Verify TCP+TLS split matches tls.Dialer behavior |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## Core Insight
Generalizing `network.RealDialer` from BGP-only to all TCP services required only adding one field (`Timeout`). The MD5/TTL fields are zero-valued no-ops for non-BGP services, so the struct was already general; only the usage was specific.

## Implementation Summary

### What Was Implemented
- `network.RealDialer` gained a `Timeout time.Duration` field (`network.go:82`),
  passed to the inner `net.Dialer`. Zero preserves existing BGP behavior.
- BMP: `collectorConfig.SourceAddress` (`json:"source-address"`) -> senderSession ->
  `RealDialer.LocalAddr` (`bmp/sender.go:88-102`).
- RPKI: `cacheServerConfig.SourceAddress` (parsed at `rpki_config.go:131`) ->
  `NewRTRSession(... sourceAddress ...)` -> `RealDialer.LocalAddr`. Dial now uses a
  stopCh-cancellable context (`rtr_session.go`).
- FlowExport: `parseCollectorMap` reads `source-address` (`config.go`); `NewSender`
  binds the UDP local source.
- LDP: `ldpSessionDialer(localTransport)` binds `cfg.TransportAddr` as LocalAddr
  when valid (`register.go`).
- IRR: `IRR.SetSourceAddress` + `RealDialer.LocalAddr` in `query` (`irr/client.go`).
- Managed: TCP via `RealDialer` then `tls.Client()` + `HandshakeContext()`;
  ServerName derived via `serverNameFromAddr` (`managed/client.go`).

### Bugs Found/Fixed (in critical review, then fixed)
- **FlowExport source-address was inert.** `parseCollectorMap` never read
  `m["source-address"]`, so the `json` tag was dead and the field always empty.
  Fixed by adding the explicit map read. Would have been caught by the (missing)
  unit test.
- **Managed TLS hostname verification regression.** The `tls.Dialer` -> `tls.Client`
  split dropped `ServerName` inference, skipping the x509 hostname check when
  `InsecureSkipVerify=false` (MITM risk). Fixed via `serverNameFromAddr`.
- **8 planned unit tests were absent.** All written; they pin both bugs above.

### Documentation Updates
- `docs/guide/configuration.md`: source-address in flow-export + hub client
  examples; new "Outbound Source Address" reference section.
- `docs/features.md`: new "Outbound source-address" capability row.
- `docs/comparison.md`: intentionally not updated (not a BGP-daemon comparison axis).

### Deviations from Plan
- RPKI dial context: implemented the stopCh-cancellable context the spec's
  Current Behavior constraint asked for (initial impl used `context.Background()`).
- LDP dialer + Managed ServerName extracted into named helpers for testability.

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| All operator-relevant outbound services support source-address | Done | 6 services below | BMP, RPKI, FlowExport, IRR, Managed, LDP |
| Generalize `network.RealDialer` as standard outbound dialer | Done | `network.go:82` | `Timeout` added; used by BMP/RPKI/IRR/LDP/Managed |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 Timeout honored | Done | `TestRealDialerTimeout` | |
| AC-2 Timeout zero unchanged | Done | `TestRealDialerTimeoutZero` | |
| AC-3 BMP with source | Done | `TestBMPCollectorSourceAddress` | |
| AC-4 BMP without source | Done | `TestBMPCollectorSourceAddress` (nosrc) | empty preserved |
| AC-5 RPKI with source | Done | `TestRPKISourceAddress` | |
| AC-6 RPKI without source | Done | `TestRPKISourceAddress` (empty) | |
| AC-7 FlowExport with source | Done | `TestFlowExportSourceAddress` | pins the wiring bug |
| AC-8 FlowExport without source | Done | `TestSenderUDP` (""), parse test | |
| AC-9 LDP with transport-address | Done | `TestLDPTransportAddressBinding` | |
| AC-10 LDP without transport-address | Done | `TestLDPTransportAddressBinding` (zero addr) | |
| AC-11 IRR with source | Done | `TestIRRSourceAddress` | non-local bind fails |
| AC-12 IRR without source | Done | existing IRR tests (no regression) | |
| AC-13 Managed with source | Done | `TestManagedSourceAddress` | non-local bind fails |
| AC-14 Managed without source | Done | existing managed tests | |
| AC-15 Invalid source rejected | Done | `zt:ip-address` YANG type + 5 parse tests | schema validation |
| AC-16 No regression TACACS/RADIUS/BGP | Done | full package suites green | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRealDialerTimeout` | Done | `internal/core/network/network_test.go` | |
| `TestRealDialerTimeoutZero` | Done | `internal/core/network/network_test.go` | |
| `TestBMPCollectorSourceAddress` | Done | `bmp/bmp_test.go` | |
| `TestRPKISourceAddress` | Done | `rpki/rpki_config_test.go` | |
| `TestFlowExportSourceAddress` | Done | `flowexport/sender_test.go` | |
| `TestLDPTransportAddressBinding` | Done | `ldp/register_test.go` | |
| `TestIRRSourceAddress` | Done | `resolve/irr/client_test.go` | |
| `TestManagedSourceAddress` | Done | `managed/client_test.go` | |
| `TestManagedTLSServerName` (added) | Done | `managed/client_test.go` | pins TLS hostname fix |
| 5 parse tests | Done | `test/parse/source-address-*.ci` | schema acceptance |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| All 19 "Files to Modify" | Done | plus `flowexport/config.go` wiring fix |
| 5 "Files to Create" (.ci) | Done | present |
| helpers added | Done | `ldpSessionDialer`, `serverNameFromAddr` |

### Audit Summary
- **Total items:** 16 ACs + 10 tests + 6 services
- **Done:** all
- **Partial:** none
- **Skipped:** comparison.md doc (deliberate, not a comparison axis)
- **Changed:** RPKI dial context improved; two helpers extracted for testability

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| All outbound services support source-address | Unit tests + parse tests | 6 services each with a passing unit test asserting config->dialer binding (AC-3..AC-14); 5 parse tests for schema acceptance |
| network.RealDialer generalized | Unit test | `TestRealDialerTimeout`/`TestRealDialerTimeoutZero` pass; BMP/RPKI/IRR/LDP/Managed all dial via `network.RealDialer` |
| No regression in existing services | Existing test suite | full package suites for network/bmp/rpki/filter_irr/flowexport/ldp/irr/managed/config/plugin all green (`go test -count=1`) |

## Review Gate
### Run 1 (initial critical review)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | FlowExport source-address inert: `parseCollectorMap` never read `m["source-address"]`; json tag dead | `flowexport/config.go` | Fixed: explicit map read |
| 2 | BLOCKER | Managed TLS hostname check dropped: empty ServerName + InsecureSkipVerify=false skips VerifyHostname (MITM) | `managed/client.go` | Fixed: `serverNameFromAddr` sets ServerName |
| 3 | ISSUE | All 8 planned unit tests missing; feature had zero behavioral coverage | multiple | Fixed: 8 + 1 tests written |
| 4 | ISSUE | Docs not updated (features.md, configuration.md) | docs/ | Fixed |
| 5 | NOTE | RPKI dial used `context.Background()`, spec asked for stopCh cancellation | `rtr_session.go` | Fixed: stopCh-cancellable dial context |

### Fixes applied
All five items above resolved. Two dialer/ServerName helpers extracted for unit testability.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | No new findings; all touched package suites green | - | - |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |

### Assumptions Resolved
| ID | Final Status | Evidence |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-16 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered
- [ ] Architecture docs updated where needed
- [ ] Critical Review passes
- [ ] Risks & Assumptions resolved

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests N/A (source-address is local socket option)
- [ ] Goal Validation table filled

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-service-source-address.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-service-source-address.md`
