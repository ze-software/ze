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
4. `internal/component/iface/backend.go` -- Backend interface (~~33 methods~~ 42 methods as of 2026-07-10, see Post-wave corrections)
5. `internal/component/iface/tunnel.go` -- TunnelKind/TunnelSpec pattern
6. `internal/component/iface/wireguard.go` -- WireguardSpec pattern
7. `internal/component/iface/pppoe_client.go` -- managed lifecycle pattern
8. Child specs: `spec-ipsec-1-*` through `spec-ipsec-10-*`

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
| Interface backend abstraction (~~33 methods~~ 42 methods as of 2026-07-10, see Post-wave corrections) | `internal/component/iface/backend.go` | Implemented |
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
| Native IKEv2 in Go | Ze implements the IKEv2 state machine natively, consistent with how BGP is implemented natively rather than wrapping BIRD. strongSwan source serves as a reference implementation to learn from. No external daemon, no subprocess management, no IPC protocol. Ze owns the protocol end-to-end |
| Config-driven, direct kernel integration | All IPsec config lives in Ze's YANG tree. The IKE engine consumes typed Go structs directly from the config parser. XFRM SAs and policies are installed in the kernel via netlink. No intermediate config files, no shell scripts |
| Route-based tunnels only | XFRM interfaces (Linux 4.19+) over policy-based IPsec because route-based is the only model that composes with Ze's routing table, BGP, and firewall. VTI (the older XFRM mark mechanism) is not supported; XFRM interfaces are the kernel-maintainer-recommended path |
| PKI as shared infrastructure | The `pki {}` config section is not IPsec-specific. TLS for web, SSH host certificates, managed device TLS, and future mutual TLS all need certificates. Build a proper PKI store first |
| EAP for Windows client compatibility | Windows built-in IKEv2 client requires EAP authentication (EAP-TLS or EAP-MSCHAPv2). Supporting EAP enables road warrior VPN from Windows, macOS, iOS, and Android without third-party software |

### Scope

**In Scope:**

| Area | Description |
|------|-------------|
| PKI certificate store | YANG `pki {}`, parse CA/cert/key, $9$ encoding for private keys, `show pki` CLI |
| XFRM interfaces | Modern route-based IPsec interfaces (Linux 4.19+), if_id based |
| IPsec data model | YANG `vpn ipsec {}`, IKE/ESP groups with proposals, DPD, lifetimes |
| Native IKEv2 engine | Full IKEv2 state machine in Go: IKE_SA_INIT, IKE_AUTH, CREATE_CHILD_SA, INFORMATIONAL exchanges |
| IKEv2 wire format | Packet codec for all IKEv2 payload types (SA, KE, Nonce, ID, AUTH, CERT, TSi/TSr, Notify, Delete, EAP, CP) |
| Crypto engine | DH groups, PRF/integrity/encryption algorithms, SKEYSEED derivation, proposal negotiation |
| X.509 certificate authentication | RSA-PSS and ECDSA signatures, certificate chain validation via PKI store |
| PSK authentication | Pre-shared key authentication for simple site-to-site deployments |
| EAP authentication | EAP-TLS (certificate) and EAP-MSCHAPv2 (password) for Windows/macOS/iOS/Android client compatibility |
| Remote access (road warrior) | Virtual IP assignment via IKEv2 Configuration Payload (RFC 7296 Section 2.19), traffic selector narrowing |
| Site-to-site peers | Connection lifecycle, XFRM interface binding, DPD, rekeying |
| XFRM SA/SP installation | Install/remove Security Associations and Security Policies in kernel via netlink |
| NAT-T | NAT detection (RFC 7296 Section 2.23), UDP encapsulation of ESP (RFC 3948), keepalives |
| DPD and rekeying | Dead Peer Detection via INFORMATIONAL, IKE SA and Child SA rekeying |
| IPsec CLI | `show vpn ipsec sa`, `show vpn ipsec status`, `clear vpn ipsec sa`, `monitor vpn ipsec` |
| Bus events | Tunnel up/down/rekeyed events on the event bus |
| Diagnostics | Health checks, SA expiry monitoring, connection state, Prometheus metrics |
| VPP dataplane | SA/SP installer abstraction with two backends: kernel XFRM (netlink) and VPP (VPP API). IKEv2 engine is backend-agnostic |

**Out of Scope:**

| Area | Reason |
|------|--------|
| VTI interfaces | Legacy mechanism using XFRM marks. XFRM interfaces are the kernel-maintainer-recommended replacement |
| Transport mode IPsec | Ze is a router; tunnel mode only |
| Policy-based IPsec | Route-based (XFRM) only. Policy-based doesn't compose with routing |
| ~~IPsec with VPP backend~~ | ~~Moved to In Scope~~ |
| L2TP/IPsec | L2TP is a separate component. IPsec transport mode for L2TP is out of scope |
| DMVPN / FlexVPN | Cisco-specific overlays. Not applicable |
| IKEv1 | Deprecated. IKEv2 only |
| MOBIKE | ~~Out of scope~~ Promoted to `spec-ipsec-11-mobike.md` |
| EAP-RADIUS backend | EAP credentials are local to Ze config. External RADIUS backend is future work |

### Native IKEv2 Architecture

Ze implements IKEv2 entirely in Go, with no external daemon dependency. This
eliminates the gokrazy appliance constraint (no need to bundle charon) and
aligns with Ze's philosophy of native protocol implementations (BGP, DNS).

The IKEv2 engine is a new component at `internal/component/ike/` with these layers:

| Layer | Responsibility |
|-------|---------------|
| Wire codec | Encode/decode IKEv2 messages (header + payload chain) |
| Crypto | DH groups, PRF, encryption, integrity, SKEYSEED/SK_* derivation |
| Proposal negotiation | Match local policy against remote proposals, select algorithms |
| State machine | Per-IKE-SA FSM: exchanges, retransmission, timers, error handling |
| Authentication | X.509 (RSA-PSS, ECDSA), PSK, EAP (EAP-TLS, EAP-MSCHAPv2) |
| XFRM installer | Install/remove kernel SAs and policies via netlink |
| Transport | UDP socket management (port 500/4500), NAT-T detection and encapsulation |

strongSwan source (`src/libcharon/`, `src/libstrongswan/`) serves as a reference
for edge cases, state transitions, and protocol details. Key reference areas:

| Area | strongSwan location | What to learn |
|------|---------------------|---------------|
| Message encoding | `src/libcharon/encoding/` | Payload type IDs, encoding order, padding |
| IKEv2 FSM | `src/libcharon/sa/ike_sa.c` | State transitions, error handling |
| Proposal negotiation | `src/libstrongswan/crypto/proposal/` | Algorithm ID mapping, selection logic |
| DH groups | `src/libstrongswan/plugins/` | Group parameters, key derivation |
| XFRM installer | `src/libcharon/plugins/kernel_netlink/` | SA/SP netlink attributes, if_id binding |
| Rekeying | `src/libcharon/sa/ikev2/tasks/` | Rekey collision handling, overlapping SA window |
| NAT-T | `src/libcharon/sa/ikev2/tasks/ike_natd.c` | Detection hash, port floating |
| EAP framework | `src/libcharon/sa/ikev2/authenticators/eap_authenticator.c` | EAP exchange flow in IKE_AUTH |
| EAP-TLS | `src/libcharon/plugins/eap_tls/` | TLS-in-EAP framing |
| EAP-MSCHAPv2 | `src/libcharon/plugins/eap_mschapv2/` | Challenge/response flow |
| Configuration Payload | `src/libcharon/sa/ikev2/tasks/ike_config.c` | Virtual IP assignment |

### Child Specs

| Phase | Spec | Scope | Depends |
|-------|------|-------|---------|
| 1 | `spec-ipsec-1-pki-store.md` | YANG `pki {}` config, certificate parser, in-memory store, `$9$` for private keys, `show pki certificates` CLI, certificate validation (chain, expiry). Shared infrastructure for IPsec, TLS, and future mutual-auth features | - |
| 2 | `spec-ipsec-2-xfrm.md` | XFRM interface type in iface component. Backend methods (CreateXFRM, GetXFRMInfo). YANG schema for `xfrm` list. Netlink wiring via xfrm_interface. Reconciliation. Discovery of unmanaged XFRM interfaces | ipsec-1 (soft) |
| 3 | `spec-ipsec-3-data-model.md` | ~~DONE~~ YANG `vpn ipsec {}`. IKE groups, ESP groups, interface binding. Config parser producing typed Go structs. Design record: `docs/architecture/ike/ipsec-3-data-model.md` | - |
| 4 | `spec-ipsec-4-data-model-eap.md` | Extend ipsec-3 data model with EAP authentication config (eap-tls, eap-mschapv2), remote-access peer type, virtual IP pool config, EAP credentials | ipsec-3 |
| 5 | `spec-ipsec-5-ikev2-wire.md` | IKEv2 packet codec: header, all payload types (SA, KE, Nonce, ID, AUTH, CERT, CERTREQ, TSi, TSr, Notify, Delete, Vendor, EAP, CP). Encode/decode roundtrip. No state machine, no network I/O | - |
| 6 | `spec-ipsec-6-ikev2-crypto.md` | Crypto primitives: DH groups (14, 19, 20), PRF (HMAC-SHA256/384/512), encryption (AES-GCM, AES-CBC), integrity, SKEYSEED/SK_* key hierarchy derivation, proposal negotiation | - |
| 7 | `spec-ipsec-7-ikev2-engine.md` | IKEv2 state machine: per-IKE-SA FSM, IKE_SA_INIT + IKE_AUTH exchanges, X.509 and PSK authentication, retransmission with exponential backoff, UDP transport (port 500/4500) | ipsec-1, ipsec-5, ipsec-6 |
| 8 | `spec-ipsec-8-ikev2-child-xfrm.md` | CREATE_CHILD_SA exchange, XFRM SA/SP installation via netlink (vishvananda/netlink), traffic selectors, DPD (INFORMATIONAL exchange), IKE SA and Child SA rekeying, lifetime management | ipsec-2, ipsec-7 |
| 9 | `spec-ipsec-9-ikev2-eap-nat.md` | EAP authentication framework (EAP-TLS, EAP-MSCHAPv2), virtual IP via Configuration Payload, NAT-T detection (NAT_DETECTION_*_IP), UDP encapsulation switching (port 4500), keepalives | ipsec-4, ipsec-7 |
| 10 | `spec-ipsec-10-cli-diag.md` | `show vpn ipsec sa`, `show vpn ipsec status`, `show vpn ipsec peer <name>`, `clear vpn ipsec sa`, `monitor vpn ipsec`. Pipe support. Web status page. Health checks. SA expiry monitoring. Metrics. Queries internal Go state directly (no VICI) | ipsec-8 |

### Dependency Graph

```
ipsec-1 (PKI)   ipsec-3 (Data Model) [DONE]
    |                  |
    |            ipsec-4 (EAP config)
    |                  |
    |   ipsec-5 (Wire)  ipsec-6 (Crypto)     [parallelizable]
    |        \           /
    |         v         v
    +-----> ipsec-7 (IKE Engine) <--- ipsec-1 (PKI, for X.509 auth)
                  |
    +-----> ipsec-8 (Child SA + Dataplane) <--- ipsec-2 (XFRM interfaces)
    |             |
    |       ipsec-9 (EAP + NAT-T) <--- ipsec-4
    |             |
    +-----> ipsec-10 (CLI/Diag)
```

Specs 5 and 6 are parallelizable (no interdependency). Spec 7 is the core
engine that depends on both. Spec 8 adds child SA management and the dataplane
abstraction (XFRM netlink + VPP API). Spec 9 extends the engine with EAP
authentication and NAT traversal. Spec 10 is presentation.

Spec 2 (XFRM interfaces) is in progress and independent of the IKEv2 engine.
Spec 3 is done. Spec 3b extends the data model for EAP/remote-access config.

### RFC Coverage

| RFC | Topic | Summary needed |
|-----|-------|---------------|
| RFC 4301 | Security Architecture for IP | Done: `rfc/short/rfc4301.md` |
| RFC 4303 | ESP (Encapsulating Security Payload) | Done: `rfc/short/rfc4303.md` |
| RFC 7296 | IKEv2 | Done: `rfc/short/rfc7296.md` |
| RFC 6071 | IPsec/IKE Roadmap | Done: `rfc/short/rfc6071.md` |
| RFC 3948 | UDP Encapsulation of ESP (NAT-T) | Done: `rfc/short/rfc3948.md` |
| RFC 3748 | EAP Framework | MUST CREATE: `rfc/short/rfc3748.md` |
| RFC 5216 | EAP-TLS | MUST CREATE: `rfc/short/rfc5216.md` |
| RFC 2759 | MS-CHAPv2 | MUST CREATE: `rfc/short/rfc2759.md` |
| RFC 7427 | Signature Authentication in IKEv2 | MUST CREATE: `rfc/short/rfc7427.md` (digital signature method, replaces legacy RSA/ECDSA AUTH) |
| RFC 4555 | MOBIKE | Optional (future) |

### Key Design Questions (Resolved)

| Question | Decision | Rationale |
|----------|----------|-----------|
| Native IKEv2 vs. strongSwan? | Native Go | Consistent with BGP (native, not wrapping BIRD). Eliminates external daemon dependency for gokrazy appliances. Ze owns the protocol end-to-end. strongSwan source is a reference for edge cases |
| VTI vs. XFRM interfaces? | XFRM only | VTI is legacy (XFRM marks). XFRM interfaces (Linux 4.19+, if_id) are the kernel-maintainer-recommended replacement. No value carrying both |
| XFRM netlink vs. VPP API? | Both, behind abstraction | SA/SP installer interface with two backends. IKEv2 engine is dataplane-agnostic. Kernel XFRM via netlink for Linux, VPP API for VPP deployments |
| PKI in IPsec component vs. shared? | Shared `pki` component | Certificates are used by web TLS, SSH host keys, managed device auth. PKI is infrastructure |
| EAP methods? | EAP-TLS + EAP-MSCHAPv2 | EAP-TLS is certificate-based (strongest). EAP-MSCHAPv2 is password-based (Windows built-in VPN default). Together they cover all major OS built-in clients |
| EAP credential storage? | Local config | EAP credentials stored in Ze's YANG tree (password uses $9$ encoding). No external RADIUS backend for v1 |
| Virtual IP assignment? | IKEv2 Configuration Payload | RFC 7296 Section 2.19. Ze acts as the IP address pool manager for road warrior clients. Pool defined in YANG config |

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` -- interface capability table, tunnel/wireguard patterns
  -> Decision: VTI/XFRM follow the same Backend method + Spec struct pattern as tunnel/wireguard
  -> Constraint: new interface kinds must be registered via YANG schema + backend extension
- [ ] `docs/architecture/core-design.md` -- component lifecycle, event bus, registration pattern
  -> Constraint: IPsec component follows registration pattern; bus events for SA state
- [ ] `internal/component/iface/backend.go` -- Backend interface (~~33 methods~~ 42 methods as of 2026-07-10, CreateTunnel/CreateWireguardDevice precedent)
  -> Decision: add CreateVTI and CreateXFRM methods to Backend (CreateXFRM already landed via ipsec-2, backend.go -- see Post-wave corrections 2026-07-10)
- [ ] `internal/component/iface/pppoe_client.go` -- PPPoEClient lifecycle (supervised subprocess precedent)
  -> Decision: IKEv2 engine follows similar lifecycle pattern: config-driven start/stop, per-peer goroutine with reconnect
- [ ] `internal/component/config/secret/secret.go` -- $9$ sensitive leaf encoding
  -> Constraint: PKI private keys and EAP passwords use $9$ encoding, same as wireguard keys and PPPoE passwords

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4301.md` -- IPsec Security Architecture (SPD, SAD, tunnel mode processing)
- [ ] `rfc/short/rfc4303.md` -- ESP wire format, AEAD, anti-replay
- [ ] `rfc/short/rfc7296.md` -- IKEv2 exchanges, auth methods, proposals, DPD, NAT-T, rekeying
- [ ] `rfc/short/rfc6071.md` -- IPsec/IKE document roadmap, algorithm requirements
- [ ] `rfc/short/rfc3948.md` -- NAT-T UDP encapsulation, port 4500, keepalives

**Key insights:**
- Backend interface already has the CreateTunnel/CreateWireguardDevice pattern; XFRM follows the same shape
- PPPoE client lifecycle (reconnect backoff, config reconciliation) informs the IKE SA manager pattern
- $9$ encoding handles all sensitive leaves uniformly; PKI private keys and EAP passwords get the same treatment
- YANG choice/case walker already works (tunnel spec added it); IPsec proposal groups can use it
- Ze does NOT shell out to ip/iproute2; XFRM interfaces and XFRM SAs/policies are managed via netlink
- vishvananda/netlink already in vendor tree; provides XfrmPolicyAdd/XfrmStateAdd for SA/SP installation
- Go standard library provides all needed crypto: `crypto/ecdh`, `crypto/ecdsa`, `crypto/rsa`, `crypto/aes`, `crypto/hmac`, `crypto/sha256`, `crypto/tls`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/backend.go` -- Backend interface: ~~33 methods~~ 42 methods as of 2026-07-10, CreateTunnel(TunnelSpec), CreateWireguardDevice(name), ConfigureWireguardDevice(WireguardSpec). ~~No VTI/XFRM/IPsec methods~~ CreateXFRM/GetXFRMInfo now exist (backend.go/:108, see Post-wave corrections)
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
- Backend interface extended with CreateXFRM, GetXFRMInfo methods
- New `pki {}` top-level YANG container for certificate store
- New `vpn ipsec {}` top-level YANG container for IPsec config (done: ipsec-3)
- New `xfrm` interface list in iface YANG schema
- New `internal/component/ike/` component for native IKEv2 engine
- New `internal/component/pki/` component for certificate management
- New CLI commands under `show vpn ipsec` and `show pki`
- UDP listeners on port 500 and 4500 for IKEv2 protocol
- XFRM SA/SP installed directly by Ze via netlink (or VPP API)
- EAP authentication support for road warrior clients
- Virtual IP assignment for remote access clients

## Data Flow (MANDATORY)

### Entry Point
- Config load/reload: YANG tree parsed, PKI certs loaded, IPsec config parsed, XFRM interfaces created, IKE engine starts peers
- IKEv2 wire: UDP packets on port 500/4500 received by IKE engine
- CLI: `show vpn ipsec sa` queries IKE engine's in-memory SA table
- Bus events: IKE engine publishes SA state changes on event bus

### Transformation Path
1. Config parser reads `pki {}` tree, creates in-memory certificate store (cert chains, private keys)
2. Config parser reads `interface { xfrm ... }`, creates XFRM netdevs via Backend
3. Config parser reads `vpn ipsec {}` tree, produces typed Go structs (IKEGroup, ESPGroup, SiteToSitePeer)
4. IKE engine receives config, opens UDP sockets, starts per-peer goroutines
5. For initiator peers: sends IKE_SA_INIT, negotiates crypto, authenticates (X.509/PSK/EAP)
6. On successful IKE_AUTH: installs XFRM SA/SP in kernel via dataplane backend (netlink or VPP API)
7. Kernel routes traffic through XFRM interface, encrypted/decrypted by XFRM SA
8. IKE engine handles DPD, rekeying, and error recovery; publishes bus events on state changes
9. For road warrior clients: EAP exchange inside IKE_AUTH, virtual IP assigned via Configuration Payload

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree to PKI store | Parse PEM certificates from YANG leaves | [ ] |
| Config tree to IPsec structs | Parse IKE/ESP groups and peers from YANG | [ ] |
| IKE engine to network | UDP sockets on port 500/4500 | [ ] |
| IKE engine to kernel (dataplane) | XFRM SA/SP via netlink (XfrmStateAdd/XfrmPolicyAdd) or VPP API | [ ] |
| iface Backend to kernel | XFRM netdev creation via netlink (managed by iface) | [ ] |
| IKE engine to bus | SA state change events published to EventBus | [ ] |

### Integration Points
- `iface.Backend` -- CreateXFRM, GetXFRMInfo methods
- `iface.config.go` -- parseXFRMEntry, applyXFRMs
- `config/secret` -- PKI private keys and EAP passwords use existing $9$ encoding
- `events.EventBus` -- IPsec SA up/down/rekeyed events
- `health.Registry` -- IPsec tunnel health checks
- `config/transaction` -- IPsec config participates in transactional reload
- Dataplane interface -- SA/SP installer abstraction (XFRM netlink backend, VPP API backend)

### Architectural Verification
- [ ] No bypassed layers (XFRM interfaces created via Backend, XFRM SAs via dataplane abstraction)
- [ ] No unintended coupling (IKE engine uses dataplane interface, not raw netlink directly)
- [ ] No duplicated functionality (extends iface Backend, does not create parallel interface management)
- [ ] Zero-copy preserved where applicable (config parsing uses existing tree walker, wire codec uses buffer-first pattern)

## Acceptance Criteria (Umbrella-Level)

These are the top-level outcomes. Each child spec has its own detailed ACs.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Ze config has `pki { ca ... certificate ... }` | Certificates parsed, validated, stored in memory. `show pki certificates` lists them with subject, issuer, expiry |
| AC-2 | Ze config has `interface { xfrm xfrm0 { ... } }` | XFRM netdev created via netlink, address assigned, admin up |
| AC-3 | Ze config has `vpn ipsec { esp-group ... ike-group ... }` | Config parsed into typed Go structs, validated (algorithm support, DH groups) |
| AC-4 | Ze config has site-to-site peer with X.509 auth | IKEv2 negotiation completes natively, IKE SA and Child SA established |
| AC-5 | IKEv2 tunnel established | `show vpn ipsec sa` shows SA with peer, algorithm, bytes, rekey time |
| AC-6 | Remote peer unreachable | DPD detects failure, connection restarted per close-action |
| AC-7 | Config reload changes peer remote-address | Changed connection torn down and re-initiated, unchanged connections preserved |
| AC-8 | Child SA lifetime expires | Rekeying completes transparently, traffic uninterrupted |
| AC-9 | Windows client connects with EAP-MSCHAPv2 | IKE_AUTH exchange includes EAP, client authenticates with username/password, virtual IP assigned |
| AC-10 | Client connects with EAP-TLS | IKE_AUTH exchange includes EAP-TLS, client authenticates with certificate, tunnel established |
| AC-11 | Peer behind NAT | NAT-T detected in IKE_SA_INIT, ESP switched to UDP encapsulation on port 4500 |
| AC-12 | VPP backend active | XFRM SAs installed via VPP API instead of kernel netlink; traffic encrypted/decrypted by VPP |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config load with `pki {}` block | -> | PKI store parses and holds certificates | `test/parse/pki-certificate.ci` |
| Config load with `interface { xfrm ... }` | -> | XFRM netdev created via Backend.CreateXFRM | `test/reload/test-tx-iface-xfrm-create.ci` |
| Config load with `vpn ipsec { esp-group ... }` | -> | IPsec config parsed into Go structs | `test/parse/ipsec-esp-group.ci` |
| Config load with site-to-site peer | -> | IKE engine initiates IKEv2 negotiation | `test/ipsec/ipsec-site-to-site-initiate.ci` |
| IKEv2 negotiation completes | -> | XFRM SA/SP installed in kernel via dataplane backend | `test/ipsec/ipsec-sa-installed.ci` |
| `show vpn ipsec sa` CLI command | -> | IKE engine SA table queried, JSON returned | `test/ipsec/ipsec-show-sa.ci` |
| Windows client connects with EAP | -> | EAP exchange completes, virtual IP assigned | `test/ipsec/ipsec-eap-auth.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParsePKICertificate` | `internal/component/pki/config_test.go` | PEM certificate parsing from YANG tree | |
| `TestPKICertValidation` | `internal/component/pki/store_test.go` | Chain validation, expiry check | |
| `TestParseXFRMEntry` | `internal/component/iface/config_test.go` | XFRM config parsing from YANG | |
| `TestParseIKEGroup` | `internal/component/ike/ipsec/config_test.go` | IKE group with proposals, DPD, lifetime | |
| `TestParseESPGroup` | `internal/component/ike/ipsec/config_test.go` | ESP group with proposals, PFS, lifetime | |
| `TestParseSiteToSitePeer` | `internal/component/ike/ipsec/config_test.go` | Peer with X.509 auth, XFRM bind | |
| `TestIKEv2EncodeHeader` | `internal/component/ike/wire/header_test.go` | IKEv2 header encode/decode roundtrip | |
| `TestIKEv2EncodePayloads` | `internal/component/ike/wire/payload_test.go` | All payload types encode/decode | |
| `TestIKEv2ProposalNegotiation` | `internal/component/ike/crypto/proposal_test.go` | Proposal selection from remote offers | |
| `TestIKEv2SKEYSEED` | `internal/component/ike/crypto/keys_test.go` | SKEYSEED and SK_* key hierarchy derivation | |
| `TestIKEv2FSMInitiator` | `internal/component/ike/engine/fsm_test.go` | State transitions for initiator IKE_SA_INIT + IKE_AUTH | |
| `TestIKEv2EAPExchange` | `internal/component/ike/engine/eap_test.go` | EAP exchange within IKE_AUTH | |
| `TestXFRMSAInstall` | `internal/component/ike/dataplane/xfrm_test.go` | XFRM SA/SP installation via netlink | |
| `TestDataplaneVPP` | `internal/component/ike/dataplane/vpp_test.go` | SA/SP installation via VPP API | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pki-certificate` | `test/parse/pki-certificate.ci` | PKI config accepted, cert parsed | |
| `ipsec-esp-group` | `test/parse/ipsec-esp-group.ci` | ESP group config accepted | |
| `ipsec-site-to-site-initiate` | `test/ipsec/ipsec-site-to-site-initiate.ci` | Tunnel initiates to peer | |
| `ipsec-sa-installed` | `test/ipsec/ipsec-sa-installed.ci` | XFRM SA/SP visible in kernel after negotiation | |
| `ipsec-show-sa` | `test/ipsec/ipsec-show-sa.ci` | SA displayed with correct fields | |
| `ipsec-eap-auth` | `test/ipsec/ipsec-eap-auth.ci` | EAP authentication completes, virtual IP assigned | |
| `iface-xfrm-create` | `test/reload/test-tx-iface-xfrm-create.ci` | XFRM netdev created on config load | |

## Files to Modify
- `internal/component/iface/backend.go` -- add CreateXFRM, GetXFRMInfo methods to Backend interface
- `internal/component/iface/config.go` -- add parseXFRMEntry, applyXFRMs
- `internal/component/iface/discover.go` -- add zeTypeXFRM classification
- `internal/component/iface/register.go` -- register XFRM parsing
- `internal/component/iface/yang/ze-iface-conf.yang` -- add xfrm interface list
- `internal/plugins/ifacenetlink/backend_linux.go` -- implement CreateXFRM, GetXFRMInfo (removed; ifacenetlink was a dead stub)
- `internal/plugins/ifacenetlink/backend_other.go` -- stub CreateXFRM, GetXFRMInfo (removed; ifacenetlink was a dead stub)
- `docs/features/interfaces.md` -- update capability table (XFRM: have)
- `cmd/ze/hub/main.go` -- wire PKI store and IKE engine component at startup

## Files to Create
- `internal/component/pki/` -- PKI certificate store component
- `internal/component/ike/` -- IKEv2 engine (new component, replaces the strongSwan integration)
- `internal/component/ike/wire/` -- IKEv2 packet codec (header, all payload types)
- `internal/component/ike/crypto/` -- DH, PRF, encryption, integrity, SKEYSEED, proposal negotiation
- `internal/component/ike/engine/` -- FSM, exchanges, authentication, retransmission, DPD, rekeying
- `internal/component/ike/eap/` -- EAP framework, EAP-TLS, EAP-MSCHAPv2
- `internal/component/ike/dataplane/` -- SA/SP installer interface + XFRM netlink backend + VPP backend
- `internal/component/ike/transport/` -- UDP socket management, NAT-T, port 500/4500
- `internal/component/iface/xfrm.go` -- XFRMSpec, XFRMInfo types
- `internal/plugins/ifacenetlink/xfrm_linux.go` -- XFRM interface netlink creation (removed; ifacenetlink was a dead stub)
- `rfc/short/rfc3748.md`, `rfc5216.md`, `rfc2759.md`, `rfc7427.md` -- RFC summaries (EAP, signature auth)

## Implementation Steps

### Implementation Phases

Each phase corresponds to a child spec. Phases are ordered by dependency.

1. **Phase: PKI Store (ipsec-1)** -- certificate parsing, validation, YANG schema, CLI
   - Tests: `TestParsePKICertificate`, `TestPKICertValidation`, `pki-certificate.ci`
   - Files: `internal/component/pki/`, `show pki` CLI
   - Verify: certificates parsed from config, chain validated, shown via CLI

2. **Phase: XFRM Interfaces (ipsec-2)** -- new interface type, Backend extension, netlink
   - Tests: `TestParseXFRMEntry`, `iface-xfrm-create.ci`
   - Files: `internal/component/iface/xfrm.go`, backend, netlink
   - Verify: XFRM netdevs created, addresses assigned, reconciled on reload

3. **Phase: IPsec Data Model (ipsec-3)** -- ~~DONE~~ YANG vpn ipsec {}, config parser, validation
   - Design record: `docs/architecture/ike/ipsec-3-data-model.md`

4. **Phase: EAP Data Model (ipsec-4)** -- extend data model for EAP config
   - Tests: EAP authentication config parsing, virtual IP pool config
   - Files: `internal/component/ike/ipsec/config.go` (extend), YANG schema (extend)
   - Verify: EAP peer config parsed, virtual IP pool validated

5. **Phase: IKEv2 Wire Format (ipsec-5)** -- packet codec, all payload types
   - Tests: `TestIKEv2EncodeHeader`, `TestIKEv2EncodePayloads`
   - Files: `internal/component/ike/wire/`
   - Verify: encode/decode roundtrip for all payload types, fuzz-safe parsing
   - **Parallelizable with ipsec-6**

6. **Phase: IKEv2 Crypto (ipsec-6)** -- DH, PRF, AEAD, SKEYSEED, proposals
   - Tests: `TestIKEv2ProposalNegotiation`, `TestIKEv2SKEYSEED`
   - Files: `internal/component/ike/crypto/`
   - Verify: key derivation matches RFC 7296 test vectors, proposal selection correct
   - **Parallelizable with ipsec-5**

7. **Phase: IKEv2 Engine (ipsec-7)** -- FSM, IKE_SA_INIT, IKE_AUTH, X.509/PSK auth, retransmission
   - Tests: `TestIKEv2FSMInitiator`, `ipsec-site-to-site-initiate.ci`
   - Files: `internal/component/ike/engine/`, `internal/component/ike/transport/`
   - Verify: IKE SA established between two Ze instances, authenticated with X.509 and PSK

8. **Phase: Child SA + Dataplane (ipsec-8)** -- CREATE_CHILD_SA, XFRM SA/SP netlink, VPP API, rekeying, DPD
   - Tests: `TestXFRMSAInstall`, `TestDataplaneVPP`, `ipsec-sa-installed.ci`
   - Files: `internal/component/ike/dataplane/`, `internal/component/ike/engine/` (extend)
   - Verify: Child SA created, XFRM SA/SP in kernel, rekeying works, DPD detects failure

9. **Phase: EAP + NAT-T (ipsec-9)** -- EAP-TLS, EAP-MSCHAPv2, virtual IP, NAT detection, UDP encap
   - Tests: `TestIKEv2EAPExchange`, `ipsec-eap-auth.ci`
   - Files: `internal/component/ike/eap/`, `internal/component/ike/transport/` (extend)
   - Verify: Windows client authenticates via EAP, virtual IP assigned, NAT-T works

10. **Phase: CLI and Diagnostics (ipsec-10)** -- show/clear/monitor commands, web page, health, metrics
   - Tests: `ipsec-show-sa.ci`
   - Files: CLI command handlers, web page, health checks, Prometheus collector
   - Verify: SA state visible via CLI and web, health checks registered, metrics exposed

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-standard-test` passes
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

### TDD
<!-- Added 2026-07-10 to satisfy the spec validator; umbrella-level TDD is delegated to child specs. -->
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Post-wave corrections (2026-07-10)

Re-verified against current code after the followup-vpp-iface implementation wave (commits up to fe6aa242f):

- Backend interface size: the "33 methods" count is stale. `internal/component/iface/backend.go` `Backend` (backend.go) now declares exactly 42 methods. The wave added the traffic-mirroring surface `SetupMirror` (backend.go) and `RemoveMirror` (:195), and the Linux Control Plane surface `SetupLCPPair` (:203) and `RemoveLCPPair` (:204). Strikethroughs applied at the four places the old count appeared.
- ALREADY SATISFIED: the "Files to Modify" item "`internal/component/iface/backend.go` -- add CreateXFRM, GetXFRMInfo methods to Backend interface" and the matching "Behavior to change" bullet are done: `CreateXFRM(spec XFRMSpec)` exists at backend.go and `GetXFRMInfo(name string)` at backend.go (landed with ipsec-2). A future implementer must not re-add them.
- NEW MECHANISM for the VPP dataplane (AC-12): the wave vendored the govpp binapi under `vendor/go.fd.io/govpp/binapi/` (28 packages, including gre, ipip, vxlan, span, lcp, wireguard, sr). `binapi/ipsec` is NOT yet vendored (verified: absent from the vendor tree). A future VPP IPsec backend should vendor `binapi/ipsec` the same way instead of hand-rolling message structs; the existing hand-rolled types in `internal/component/ike/dataplane/vpp.go` carry a comment anticipating exactly this (vpp.go: when govpp/binapi/ipsec is vendored, replace these with the generated types).
