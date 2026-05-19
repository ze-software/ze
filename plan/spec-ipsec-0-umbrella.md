# Spec: ipsec-0 -- IPsec VPN (Umbrella)

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-05-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `docs/features/interfaces.md` -- interface capability table and patterns
4. `internal/component/iface/backend.go` -- Backend interface (33 methods)
5. `internal/component/iface/tunnel.go` -- TunnelKind/TunnelSpec pattern
6. `internal/component/iface/wireguard.go` -- WireguardSpec pattern
7. `internal/component/iface/pppoe_client.go` -- managed lifecycle pattern
8. Child specs: `spec-ipsec-1-*` through `spec-ipsec-5-*`

## Task

Add IPsec VPN support to Ze. The target deployment is a site-to-site IKEv2 tunnel
with X.509 certificate authentication, VTI mode (route-based), and managed connection
lifecycle (initiate, DPD restart, rekey). This matches the production config at
`../home.conf` where a CPE behind PPPoE establishes a management VPN to a central
gateway using strongSwan/charon.

Ze currently has no IPsec, no PKI certificate store, and no VTI interface type.
WireGuard is the only tunnel VPN, implemented natively via wgctrl. IPsec is
fundamentally different: it requires an IKE daemon for key negotiation (IKEv2 is
a complex state machine with >20 message types) and the kernel XFRM subsystem
for dataplane encryption. The pragmatic approach is to manage strongSwan/charon
as a supervised subprocess (like PPPoE manages pppd via the dialer abstraction),
not to implement IKEv2 natively.

### Reference Configuration (from ../home.conf)

The production config uses five cooperating sections:

| Section | Purpose |
|---------|---------|
| `pki` | CA cert (`exa-cpe-ca`), device cert + private key (`EXAFO000000400`) |
| `interfaces vti vti0` | Route-based tunnel: IPv6 address FC00::0:0000:0400:2/112 |
| `vpn ipsec esp-group ESP-RW` | AES-128-GCM, SHA-256, PFS disabled, 86400s lifetime |
| `vpn ipsec ike-group IKE-RW` | IKEv2, DH-14, AES-128-GCM, SHA-256, DPD restart 10/30, close-action start |
| `vpn ipsec site-to-site peer management-bridge` | X.509 auth, DNS remote, VTI bind, connection-type initiate |
| `firewall ipv4 input rule 8` | Accept ESP (protocol 50) |

Deployment pattern: CPE with dynamic IP (PPPoE) initiates IKEv2 to
`cpe-gateway.exa.net.uk`. ECDSA certificates for authentication.
Traffic routed through `vti0` with IPv6 overlay on IPv4 underlay.
Used for SSH management and software updates.

On Linux this maps to: **strongSwan/charon** for IKEv2 negotiation,
**XFRM** for kernel SA/SP database, **VTI** netdev bound by XFRM mark.

### Existing Foundation

| Capability | Package | Status |
|------------|---------|--------|
| Interface backend abstraction (33 methods) | `internal/component/iface/backend.go` | Implemented |
| Tunnel interfaces (8 GRE/IPIP/SIT kinds) | `internal/component/iface/tunnel.go` | Implemented |
| WireGuard (declarative peers, $9$ keys) | `internal/component/iface/wireguard.go` | Implemented |
| PPPoE client lifecycle (reconnect, dialer) | `internal/component/iface/pppoe_client.go` | Implemented |
| $9$ sensitive leaf encoding | `internal/component/config/secret/` | Implemented |
| YANG choice/case in schema walker | `internal/component/config/yang_schema.go` | Implemented |
| Firewall ESP protocol rule | `internal/component/firewall/` | Implemented (protocol match) |
| Bus event system | `internal/core/events/` | Implemented |
| Health registry | `internal/core/health/` | Implemented |
| Config reload with rollback | `internal/component/config/transaction/` | Implemented |

### Design Principles

| Principle | Detail |
|-----------|--------|
| Supervised subprocess, not native IKEv2 | IKEv2 (RFC 7296) is ~130 pages of state machine. strongSwan is battle-tested, audited, and handles edge cases (NAT-T, MOBIKE, EAP) that would take months to implement natively. Ze generates swanctl.conf and supervises charon, similar to how PPPoE uses a dialer abstraction |
| Config-driven, no shell scripts | All IPsec config lives in Ze's YANG tree. No `/etc/ipsec.conf`, no `ipsec up/down` shell commands. Ze generates the strongSwan config from its own tree and uses VICI protocol for runtime control |
| VTI for route-based tunnels | VTI (Virtual Tunnel Interface) over policy-based IPsec because route-based is the only model that composes with Ze's routing table, BGP, and firewall. XFRM interfaces (Linux 4.19+) are the modern successor to VTI and should be supported alongside |
| PKI as shared infrastructure | The `pki {}` config section is not IPsec-specific. TLS for web, SSH host certificates, managed device TLS, and future mutual TLS all need certificates. Build a proper PKI store first |
| VICI over CLI | strongSwan's VICI protocol (Unix socket, key-value sections) is the programmatic interface. Parsing `ipsec statusall` text output would be fragile. VICI provides structured SA state, connection control, and event subscription |

### Scope

**In Scope:**

| Area | Description |
|------|-------------|
| PKI certificate store | YANG `pki {}`, parse CA/cert/key, $9$ encoding for private keys, `show pki` CLI |
| VTI interfaces | New interface type in iface, Backend.CreateVTI, XFRM mark, YANG schema |
| XFRM interfaces | Modern alternative to VTI (Linux 4.19+), if_id based |
| IPsec data model | YANG `vpn ipsec {}`, IKE/ESP groups with proposals, DPD, lifetimes |
| Site-to-site peers | Authentication (X.509, PSK), connection lifecycle, VTI binding |
| strongSwan integration | Generate swanctl.conf, supervise charon, VICI protocol for status/control |
| IPsec CLI | `show vpn ipsec sa`, `show vpn ipsec status`, `clear vpn ipsec sa` |
| Bus events | Tunnel up/down/rekeyed events on the event bus |
| Diagnostics | Health checks, SA expiry monitoring, connection state |

**Out of Scope:**

| Area | Reason |
|------|--------|
| Native IKEv2 implementation | Complexity vs. benefit. strongSwan is the right tool |
| Remote access / road warrior | EAP, virtual IP pools, split tunneling. Future spec if needed |
| Transport mode IPsec | Ze is a router; tunnel mode only |
| Policy-based IPsec | Route-based (VTI/XFRM) only. Policy-based doesn't compose with routing |
| IPsec with VPP backend | VPP has its own IPsec plugin. Separate concern, separate spec |
| L2TP/IPsec | L2TP is a separate component. IPsec transport mode for L2TP is out of scope |
| DMVPN / FlexVPN | Cisco-specific overlays. Not applicable |
| IKEv1 | Deprecated. IKEv2 only |

### strongSwan Deployment Model

Ze targets gokrazy appliances (no package manager, no systemd). strongSwan must be
available as a statically-linked binary or built into the appliance image. Two options:

1. **Appliance includes charon-systemd** (or charon standalone). Ze generates
   `/tmp/ze-swanctl.conf` and manages the process. This is the VyOS model.
2. **Embedded charon via libcharon** (CGo). Higher integration but complex build
   and licensing (GPLv2). Not recommended for v1.

For v1: assume charon is available in PATH (appliance image includes it). Ze
writes config to a predictable path and manages the process lifecycle. The
charon binary location is configurable via YANG (`environment { ipsec { charon-path } }`).

### Child Specs

| Phase | Spec | Scope | Depends |
|-------|------|-------|---------|
| 1 | `spec-ipsec-1-pki-store.md` | YANG `pki {}` config, certificate parser, in-memory store, `$9$` for private keys, `show pki certificates` CLI, certificate validation (chain, expiry). Shared infrastructure for IPsec, TLS, and future mutual-auth features | - |
| 2 | `spec-ipsec-2-vti-xfrm.md` | VTI and XFRM interface types in iface component. Backend methods (CreateVTI, CreateXFRM). YANG schema for `vti` and `xfrm` lists. Netlink wiring via ip_tunnel (VTI) and xfrm_interface (XFRM). Reconciliation | ipsec-1 (soft) |
| 3 | `spec-ipsec-3-data-model.md` | YANG `vpn ipsec {}`. IKE groups (proposals, DPD, key-exchange, lifetime, close-action). ESP groups (proposals, PFS, lifetime). Interface binding. Config parser producing typed Go structs. Validation | - |
| 4 | `spec-ipsec-4-strongswan.md` | strongSwan integration. swanctl.conf generation from Ze config. Charon process supervision. VICI protocol client (Go, Unix socket). Connection lifecycle (initiate, terminate, rekey). Site-to-site peer config (X.509, PSK). VTI/XFRM binding. Bus events. Config reload (diff peers, restart changed) | ipsec-1, ipsec-2, ipsec-3 |
| 5 | `spec-ipsec-5-cli-diag.md` | `show vpn ipsec sa`, `show vpn ipsec status`, `show vpn ipsec peer <name>`, `clear vpn ipsec sa`. Pipe support. Web status page. Health checks. SA expiry monitoring. Metrics (tunnel up/down counter, bytes, rekey count) | ipsec-4 |

### Dependency Graph

```
ipsec-1 (PKI)  ipsec-3 (Data Model)
    |                |
    v                v
ipsec-2 (VTI) ---> ipsec-4 (strongSwan) ---> ipsec-5 (CLI/Diag)
```

Specs 1 and 3 can be implemented in parallel. Spec 2 has a soft dependency on 1
(VTI itself doesn't need PKI, but the implementation order follows the config
hierarchy). Spec 4 is the integration point. Spec 5 is presentation.

### RFC Coverage

| RFC | Topic | Summary needed |
|-----|-------|---------------|
| RFC 4301 | Security Architecture for IP | Done: `rfc/short/rfc4301.md` |
| RFC 4303 | ESP (Encapsulating Security Payload) | Done: `rfc/short/rfc4303.md` |
| RFC 7296 | IKEv2 | Done: `rfc/short/rfc7296.md` |
| RFC 6071 | IPsec/IKE Roadmap | Done: `rfc/short/rfc6071.md` |
| RFC 3948 | UDP Encapsulation of ESP (NAT-T) | Done: `rfc/short/rfc3948.md` |
| RFC 4555 | MOBIKE | Optional (future) |

### Key Design Questions (Resolved)

| Question | Decision | Rationale |
|----------|----------|-----------|
| Native IKEv2 vs. strongSwan? | strongSwan | IKEv2 is ~130 pages of RFC. strongSwan handles NAT-T, MOBIKE, EAP, fragmentation, retransmission. Battle-tested in millions of deployments |
| VTI vs. XFRM interfaces? | Both | VTI for compatibility (existing configs), XFRM for new deployments. XFRM interfaces are the Linux maintainer-recommended path forward |
| VICI vs. swanctl CLI? | VICI | Structured protocol over Unix socket. No text parsing. Supports event subscription for SA state changes |
| PKI in IPsec component vs. shared? | Shared `pki` component | Certificates are used by web TLS, SSH host keys, managed device auth. PKI is infrastructure |
| Config generation vs. passthrough? | Generation | Ze owns the config tree. Generating swanctl.conf from YANG ensures validation, diffing, and reload work consistently |

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` -- interface capability table, tunnel/wireguard patterns
  -> Decision: VTI/XFRM follow the same Backend method + Spec struct pattern as tunnel/wireguard
  -> Constraint: new interface kinds must be registered via YANG schema + backend extension
- [ ] `docs/architecture/core-design.md` -- component lifecycle, event bus, registration pattern
  -> Constraint: IPsec component follows registration pattern; bus events for SA state
- [ ] `internal/component/iface/backend.go` -- Backend interface (33 methods, CreateTunnel/CreateWireguardDevice precedent)
  -> Decision: add CreateVTI and CreateXFRM methods to Backend
- [ ] `internal/component/iface/pppoe_client.go` -- PPPoEClient lifecycle (supervised subprocess precedent)
  -> Decision: charon supervision follows PPPoE dialer pattern: config-driven start/stop, reconnect with backoff
- [ ] `internal/component/config/secret/secret.go` -- $9$ sensitive leaf encoding
  -> Constraint: PKI private keys use $9$ encoding, same as wireguard keys and PPPoE passwords

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4301.md` -- IPsec Security Architecture (SPD, SAD, tunnel mode processing)
- [ ] `rfc/short/rfc4303.md` -- ESP wire format, AEAD, anti-replay
- [ ] `rfc/short/rfc7296.md` -- IKEv2 exchanges, auth methods, proposals, DPD, NAT-T, rekeying
- [ ] `rfc/short/rfc6071.md` -- IPsec/IKE document roadmap, algorithm requirements
- [ ] `rfc/short/rfc3948.md` -- NAT-T UDP encapsulation, port 4500, keepalives

**Key insights:**
- Backend interface already has the CreateTunnel/CreateWireguardDevice pattern; VTI/XFRM follow the same shape
- PPPoE client lifecycle (dialer abstraction, reconnect backoff, config reconciliation) is the model for charon supervision
- $9$ encoding handles all sensitive leaves uniformly; PKI private keys get the same treatment
- YANG choice/case walker already works (tunnel spec added it); IPsec proposal groups can use it
- Ze does NOT shell out to ip/iproute2; VTI/XFRM must be created via netlink

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/backend.go` -- Backend interface: 33 methods, CreateTunnel(TunnelSpec), CreateWireguardDevice(name), ConfigureWireguardDevice(WireguardSpec). No VTI/XFRM/IPsec methods
  -> Constraint: new Backend methods must follow existing signatures (return error, take spec struct)
- [ ] `internal/component/iface/tunnel.go` -- TunnelKind enum + TunnelSpec struct. 8 tunnel kinds via YANG choice/case. *Set booleans for optional numeric fields
  -> Decision: VTI does NOT belong in TunnelKind. VTI is semantically different (XFRM-bound, no explicit remote endpoint). Separate type, separate Backend method
- [ ] `internal/component/iface/wireguard.go` -- WireguardSpec/WireguardPeerSpec. wgctrl dependency for key types
  -> Decision: PKI store similarly wraps crypto/x509 types behind Ze-specific spec structs
- [ ] `internal/component/iface/config.go` -- parseTunnelEntry, parseWireguardEntry, applyTunnels, applyWireguards, reconciliation with recreate-on-change
  -> Constraint: VTI/XFRM parsing and reconciliation follow the same pattern
- [ ] `internal/component/iface/pppoe_client.go` -- PPPoEClient: Start/Stop/status, PPPoEDialer interface, reconnect backoff, config reconciliation via reconcilePPPoEClients
  -> Decision: charon management uses similar Supervisor interface with Start/Stop/status
- [ ] `internal/component/config/secret/` -- $9$ JunOS-compatible obfuscation. Encode/Decode/IsEncoded. ze:sensitive YANG extension
  -> Constraint: PKI private-key leaf uses ze:sensitive, auto-decoded on load, re-encoded on show/dump

**Behavior to preserve:**
- All existing Backend interface methods unchanged
- Tunnel and wireguard parsing, reconciliation, and lifecycle unchanged
- $9$ encoding for sensitive leaves unchanged
- Firewall ESP protocol matching unchanged
- Config transaction and rollback mechanics unchanged

**Behavior to change:**
- Backend interface extended with CreateVTI, CreateXFRM methods
- New `pki {}` top-level YANG container for certificate store
- New `vpn ipsec {}` top-level YANG container for IPsec config
- New `vti` and `xfrm` interface lists in iface YANG schema
- New `internal/component/ipsec/` component for strongSwan lifecycle
- New `internal/component/pki/` component for certificate management
- New CLI commands under `show vpn ipsec` and `show pki`

## Data Flow (MANDATORY)

### Entry Point
- Config load/reload: YANG tree parsed, PKI certs loaded, IPsec config parsed, VTI/XFRM interfaces created, swanctl.conf generated, charon started/reloaded
- CLI: `show vpn ipsec sa` queries charon via VICI protocol
- Bus events: charon SA state changes (via VICI event subscription) published on event bus

### Transformation Path
1. Config parser reads `pki {}` tree, creates in-memory certificate store (cert chains, private keys)
2. Config parser reads `interface { vti ... }` / `interface { xfrm ... }`, creates VTI/XFRM netdevs via Backend
3. Config parser reads `vpn ipsec {}` tree, produces typed Go structs (IKEGroup, ESPGroup, SiteToSitePeer)
4. IPsec component generates swanctl.conf from Go structs + PKI store (cert paths)
5. IPsec component starts/reloads charon subprocess, initiates connections via VICI
6. Charon negotiates IKEv2 with remote peer, installs XFRM SAs in kernel
7. Kernel routes traffic through VTI/XFRM interface, encrypted by XFRM SA
8. VICI event subscription detects SA up/down/rekey, publishes bus events

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree to PKI store | Parse PEM certificates from YANG leaves | [ ] |
| Config tree to IPsec structs | Parse IKE/ESP groups and peers from YANG | [ ] |
| Ze to strongSwan | Generated swanctl.conf file + VICI Unix socket | [ ] |
| strongSwan to kernel | XFRM netlink (SA/SP installation, managed by charon) | [ ] |
| Ze to kernel | VTI/XFRM netdev creation via netlink (managed by iface backend) | [ ] |
| strongSwan to Ze | VICI event subscription for SA state changes | [ ] |

### Integration Points
- `iface.Backend` -- extended with CreateVTI, CreateXFRM methods
- `iface.config.go` -- extended with parseVTIEntry, parseXFRMEntry, applyVTIs, applyXFRMs
- `config/secret` -- PKI private keys use existing $9$ encoding
- `events.EventBus` -- IPsec SA up/down/rekeyed events
- `health.Registry` -- IPsec tunnel health checks
- `config/transaction` -- IPsec config participates in transactional reload

### Architectural Verification
- [ ] No bypassed layers (VTI/XFRM created via Backend, not raw netlink)
- [ ] No unintended coupling (IPsec component talks to charon via VICI, not to kernel XFRM directly)
- [ ] No duplicated functionality (extends iface Backend, does not create parallel interface management)
- [ ] Zero-copy preserved where applicable (config parsing uses existing tree walker)

## Acceptance Criteria (Umbrella-Level)

These are the top-level outcomes. Each child spec has its own detailed ACs.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Ze config has `pki { ca ... certificate ... }` | Certificates parsed, validated, stored in memory. `show pki certificates` lists them with subject, issuer, expiry |
| AC-2 | Ze config has `interface { vti vti0 { ... } }` | VTI netdev created via netlink, address assigned, admin up |
| AC-3 | Ze config has `vpn ipsec { esp-group ... ike-group ... }` | Config parsed into typed Go structs, validated (algorithm support, DH groups) |
| AC-4 | Ze config has `vpn ipsec { site-to-site { peer ... } }` with X.509 auth | swanctl.conf generated, charon started, IKEv2 tunnel established |
| AC-5 | IKEv2 tunnel established | `show vpn ipsec sa` shows SA with peer, algorithm, bytes, rekey time |
| AC-6 | Remote peer unreachable | DPD detects failure, connection restarted per close-action |
| AC-7 | Config reload changes peer remote-address | Changed connection restarted, unchanged connections preserved |
| AC-8 | charon crashes | Ze detects exit, restarts charon with backoff |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config load with `pki {}` block | -> | PKI store parses and holds certificates | `test/parse/pki-certificate.ci` |
| Config load with `interface { vti ... }` | -> | VTI netdev created via Backend.CreateVTI | `test/reload/test-tx-iface-vti-create.ci` |
| Config load with `vpn ipsec { esp-group ... }` | -> | IPsec config parsed into Go structs | `test/parse/ipsec-esp-group.ci` |
| Config load with site-to-site peer | -> | swanctl.conf generated, charon started | `test/ipsec/ipsec-site-to-site-initiate.ci` |
| `show vpn ipsec sa` CLI command | -> | VICI query returns SA state | `test/ipsec/ipsec-show-sa.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParsePKICertificate` | `internal/component/pki/config_test.go` | PEM certificate parsing from YANG tree | |
| `TestPKICertValidation` | `internal/component/pki/store_test.go` | Chain validation, expiry check | |
| `TestParseVTIEntry` | `internal/component/iface/config_test.go` | VTI config parsing from YANG | |
| `TestParseXFRMEntry` | `internal/component/iface/config_test.go` | XFRM config parsing from YANG | |
| `TestParseIKEGroup` | `internal/component/ipsec/config_test.go` | IKE group with proposals, DPD, lifetime | |
| `TestParseESPGroup` | `internal/component/ipsec/config_test.go` | ESP group with proposals, PFS, lifetime | |
| `TestParseSiteToSitePeer` | `internal/component/ipsec/config_test.go` | Peer with X.509 auth, VTI bind | |
| `TestGenerateSwanctlConf` | `internal/component/ipsec/swanctl_test.go` | swanctl.conf output matches expected | |
| `TestVICIClient` | `internal/component/ipsec/vici_test.go` | VICI protocol encode/decode/command | |
| `TestCharonSupervisor` | `internal/component/ipsec/supervisor_test.go` | Start/stop/restart lifecycle | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pki-certificate` | `test/parse/pki-certificate.ci` | PKI config accepted, cert parsed | |
| `ipsec-esp-group` | `test/parse/ipsec-esp-group.ci` | ESP group config accepted | |
| `ipsec-site-to-site-initiate` | `test/ipsec/ipsec-site-to-site-initiate.ci` | Tunnel initiates to peer | |
| `ipsec-show-sa` | `test/ipsec/ipsec-show-sa.ci` | SA displayed with correct fields | |
| `iface-vti-create` | `test/reload/test-tx-iface-vti-create.ci` | VTI netdev created on config load | |

## Files to Modify
- `internal/component/iface/backend.go` -- add CreateVTI, CreateXFRM methods to Backend interface
- `internal/component/iface/config.go` -- add parseVTIEntry, parseXFRMEntry, applyVTIs, applyXFRMs
- `internal/component/iface/discover.go` -- add zeTypeVTI, zeTypeXFRM classification
- `internal/component/iface/register.go` -- register VTI/XFRM parsing
- `internal/component/iface/schema/ze-iface-conf.yang` -- add vti and xfrm interface lists
- `internal/plugins/ifacenetlink/backend_linux.go` -- implement CreateVTI, CreateXFRM
- `internal/plugins/ifacenetlink/backend_other.go` -- stub CreateVTI, CreateXFRM
- `docs/features/interfaces.md` -- update capability table (VTI, XFRM: have)
- `cmd/ze/hub/main.go` -- wire PKI store and IPsec component at startup

## Files to Create
- `internal/component/pki/` -- PKI certificate store component
- `internal/component/ipsec/` -- IPsec component (config, swanctl, supervisor, VICI)
- `internal/component/iface/vti.go` -- VTISpec type
- `internal/component/iface/xfrm.go` -- XFRMSpec type
- `internal/plugins/ifacenetlink/vti_linux.go` -- VTI netlink creation
- `internal/plugins/ifacenetlink/xfrm_linux.go` -- XFRM interface netlink creation
- `rfc/short/rfc4301.md`, `rfc4303.md`, `rfc7296.md`, `rfc6071.md`, `rfc3948.md` -- RFC summaries

## Implementation Steps

### Implementation Phases

Each phase corresponds to a child spec. Phases are ordered by dependency.

1. **Phase: PKI Store (ipsec-1)** -- certificate parsing, validation, YANG schema, CLI
   - Tests: `TestParsePKICertificate`, `TestPKICertValidation`, `pki-certificate.ci`
   - Files: `internal/component/pki/`, `show pki` CLI
   - Verify: certificates parsed from config, chain validated, shown via CLI

2. **Phase: VTI/XFRM Interfaces (ipsec-2)** -- new interface types, Backend extension, netlink
   - Tests: `TestParseVTIEntry`, `TestParseXFRMEntry`, `iface-vti-create.ci`
   - Files: `internal/component/iface/vti.go`, `xfrm.go`, backend, netlink
   - Verify: VTI/XFRM netdevs created, addresses assigned, reconciled on reload

3. **Phase: IPsec Data Model (ipsec-3)** -- YANG vpn ipsec {}, config parser, validation
   - Tests: `TestParseIKEGroup`, `TestParseESPGroup`, `TestParseSiteToSitePeer`, `ipsec-esp-group.ci`
   - Files: `internal/component/ipsec/config.go`, YANG schema
   - Verify: config parsed into typed structs, validation rejects invalid algorithms

4. **Phase: strongSwan Integration (ipsec-4)** -- swanctl.conf, charon, VICI, lifecycle
   - Tests: `TestGenerateSwanctlConf`, `TestVICIClient`, `TestCharonSupervisor`, `ipsec-site-to-site-initiate.ci`
   - Files: `internal/component/ipsec/swanctl.go`, `vici.go`, `supervisor.go`
   - Verify: swanctl.conf generated, charon started, connection initiated, SA established

5. **Phase: CLI and Diagnostics (ipsec-5)** -- show/clear commands, web page, health, metrics
   - Tests: `ipsec-show-sa.ci`
   - Files: CLI command handlers, web page, health checks
   - Verify: SA state visible via CLI and web, health checks registered

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling
