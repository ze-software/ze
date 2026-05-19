# Spec: ipsec-4 -- strongSwan Integration

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | ipsec-1, ipsec-2, ipsec-3 |
| Phase | - |
| Updated | 2026-05-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `spec-ipsec-0-umbrella.md` -- umbrella design decisions
4. `spec-ipsec-1-pki-store.md` -- PKI store API (certificate export paths)
5. `spec-ipsec-3-data-model.md` -- IKEGroup, ESPGroup, SiteToSitePeer structs
6. `internal/component/iface/pppoe_client.go` -- PPPoEClient lifecycle pattern (supervisor model)
7. `internal/component/ipsec/config.go` -- parsed IPsec config types (from ipsec-3)

## Task

Wire the PKI store (ipsec-1), VTI/XFRM interfaces (ipsec-2), and IPsec data model
(ipsec-3) into a working IPsec VPN by integrating strongSwan. Ze generates a
`swanctl.conf` from its own config tree, supervises the charon IKE daemon as a
subprocess, and uses the VICI protocol (Unix socket) for runtime control and SA
state monitoring.

The PPPoE client lifecycle in `internal/component/iface/pppoe_client.go` is the
closest existing pattern: `PPPoEDialer` abstracts platform work, `PPPoEClient`
manages start/stop/reconnect with exponential backoff, and
`reconcilePPPoEClients` diffs config against running clients on reload. The IPsec
component follows the same shape: `CharonSupervisor` manages the charon process,
a VICI client provides runtime control, and an `IPsecReconciler` handles config
reload by diffing peers.

### strongSwan Architecture

strongSwan's charon daemon handles all IKEv2 negotiation and installs XFRM
security associations (SAs) in the kernel. Ze's role is:

1. **Config generation:** translate Ze's typed IPsec structs into `swanctl.conf`
2. **Process supervision:** start charon, monitor for crashes, restart with backoff
3. **Runtime control via VICI:** initiate/terminate connections, query SA state, subscribe to events
4. **Certificate provisioning:** call PKI store's ExportPEM to write certs/keys as PEM files (the store handles base64-DER-to-PEM conversion; see ipsec-1 AC-9)
5. **Event bridging:** translate VICI SA events into Ze bus events

### VICI Protocol

VICI (Versatile IKE Configuration Interface) is strongSwan's programmatic control
protocol. It runs over a Unix socket (`/var/run/charon.vici` by default) and uses
a binary section-encoding format:

- **Request/Response:** named commands (`list-sas`, `initiate`, `terminate`, `load-conn`, `get-conns`)
- **Event subscription:** register for events (`ike-updown`, `child-updown`, `ike-rekey`, `child-rekey`)
- **Section encoding:** type-length-value with nested sections, key-value pairs, and lists

Ze implements a minimal VICI client covering: `list-sas`, `initiate`, `terminate`,
`load-conn`, `unload-conn`, `load-authority`, `load-key`, `load-cert`, and event
subscription for `ike-updown`, `child-updown`, `ike-rekey`, `child-rekey`.

### swanctl.conf Generation

Ze generates the complete swanctl.conf from its own config tree. The file is
written to a predictable path (`/tmp/ze-ipsec/swanctl.conf`) and charon is
configured to read from there. Certificate and key PEM files are exported by the
PKI store to `/tmp/ze-ipsec/certs/`, `/tmp/ze-ipsec/cacerts/`, and
`/tmp/ze-ipsec/private/`.

The config maps from Ze's YANG model to swanctl.conf sections:

| Ze Config | swanctl.conf Section |
|-----------|---------------------|
| `vpn ipsec site-to-site peer <name>` | `connections.<name> { ... }` |
| `vpn ipsec ike-group <name>` | Inlined into connection's `proposals`, `dpd_*`, `rekey_time` |
| `vpn ipsec esp-group <name>` | Inlined into connection's `children.<child>.esp_proposals`, `rekey_time` |
| `pki ca <name>` | `authorities.<name> { cacert = <path> }` |
| `pki certificate <name>` (private key) | `secrets.ecdsa-<name> { file = <path> }` or `secrets.rsa-<name>` |
| Peer authentication x509 | `connections.<name>.local.certs`, `connections.<name>.remote.id` |
| Peer authentication psk | `secrets.ike-<name> { secret = <value> }` |
| Peer vti bind | `connections.<name>.children.<child>.if_id_in`, `if_id_out` (XFRM) or `updown` script (VTI) |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- component lifecycle, registration, event bus
  -> Constraint: IPsec component registers via init(), follows OnConfigure/OnShutdown lifecycle
- [ ] `docs/features/interfaces.md` -- Backend interface, tunnel/wireguard patterns
  -> Decision: charon supervision is NOT part of iface; it is a separate ipsec component
- [ ] `internal/component/iface/pppoe_client.go` -- PPPoEClient lifecycle: Start/Stop, reconnect backoff, reconcilePPPoEClients
  -> Decision: CharonSupervisor follows PPPoEClient pattern (Start/Stop/status, exponential backoff)
- [ ] `spec-ipsec-0-umbrella.md` -- design principles, scope, deployment model
  -> Constraint: VICI over CLI, config generation not passthrough, charon in PATH
- [ ] `spec-ipsec-1-pki-store.md` -- PKI store API, certificate export to PEM files
  -> Constraint: swanctl.conf references file paths from PKI store ExportPEM method
- [ ] `spec-ipsec-3-data-model.md` -- IKEGroup, ESPGroup, SiteToSitePeer Go structs
  -> Constraint: swanctl.conf generation takes these structs as input

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7296.md` -- IKEv2: SA negotiation, DPD, rekeying, AUTH methods
  -> Constraint: connection-type maps to IKEv2 original_initiator flag
- [ ] `rfc/short/rfc4301.md` -- IPsec architecture: SA, SP, tunnel mode
  -> Constraint: route-based (VTI/XFRM) only, no policy-based
- [ ] `rfc/short/rfc3948.md` -- NAT-T: UDP encapsulation of ESP
  -> Constraint: charon handles NAT-T automatically; Ze config does not expose it

**Key insights:**
- PPPoEClient.run() is a goroutine with reconnect loop; CharonSupervisor.run() follows the same shape
- reconcilePPPoEClients compares config entries to running clients by name; IPsec reconciler compares peer names
- PPPoEDialer abstracts platform work; CharonProcess abstracts charon binary execution
- VICI is a binary protocol, not text; Ze needs its own codec (no Go VICI library in vendor tree)
- swanctl.conf is regenerated on every reload; charon reloads via VICI `load-conn`/`unload-conn`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/pppoe_client.go` -- PPPoEClient: Start/Stop/status, PPPoEDialer interface, reconcilePPPoEClients diffs config vs running, exponential backoff via ReconnectDelay
  -> Decision: CharonSupervisor mirrors PPPoEClient lifecycle; IPsecReconciler mirrors reconcilePPPoEClients
- [ ] `internal/component/iface/pppoe_client.go:305` -- reconcilePPPoEClients: desired map from config, stop removed/changed, start new/restarted
  -> Decision: reconcileIPsecPeers follows identical pattern with VICI load-conn/unload-conn instead of client Start/Stop
- [ ] `internal/core/events/events.go` -- EventBus, Publish, Subscribe. JSON payloads
  -> Constraint: IPsec bus events use same EventBus; topic prefix `vpn/ipsec/`
- [ ] `internal/core/health/registry.go` -- RegisterCheck, Check interface with Name/Status/Details
  -> Constraint: IPsec tunnel health check returns healthy when all initiated tunnels have active SAs

**Behavior to preserve:**
- PPPoE client lifecycle unchanged
- EventBus API unchanged
- Health registry API unchanged
- Config transaction and rollback mechanics unchanged

**Behavior to change:**
- New `internal/component/ipsec/` component with registration, config parsing, and lifecycle
- Charon process managed by Ze (was: not managed, IPsec not supported)
- VICI protocol client for runtime SA control
- Bus events published for IPsec SA state changes
- Health check registered for IPsec tunnel state

## Data Flow (MANDATORY)

### Entry Point
- Config load/reload: YANG `vpn ipsec {}` tree parsed (ipsec-3), PKI certs exported to PEM (ipsec-1), swanctl.conf generated, charon started/reloaded
- CLI `show vpn ipsec sa`: queries VICI `list-sas`
- Bus subscription: VICI event stream bridged to Ze bus events

### Transformation Path
1. Config parser produces IKEGroup, ESPGroup, SiteToSitePeer structs (ipsec-3, already done)
2. PKI store exports CA certs, device certs, and private keys to PEM files under `/tmp/ze-ipsec/`
3. swanctl generator takes parsed structs + PEM paths, produces swanctl.conf text
4. CharonSupervisor writes swanctl.conf to disk, starts charon with `--conf` pointing to it
5. VICI client connects to charon socket, loads connections via `load-conn` commands
6. VICI client initiates connections for peers with connection-type=initiate
7. VICI event subscriber receives `ike-updown`, `child-updown` events, publishes on Ze bus
8. On config reload: reconciler diffs peers, unloads removed, loads new, re-initiates changed

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze config to swanctl.conf | Text generation from typed Go structs | [ ] |
| Ze to charon process | Subprocess exec with stdout/stderr capture | [ ] |
| Ze to charon VICI | Unix socket, binary section-encoded protocol | [ ] |
| Charon to kernel XFRM | Netlink (managed entirely by charon, Ze does not touch) | [ ] |
| VICI events to Ze bus | Event subscription, translation to JSON bus payloads | [ ] |
| PKI store to filesystem | PEM file export to /tmp/ze-ipsec/ | [ ] |

### Integration Points
- `internal/component/pki/` (ipsec-1) -- ExportPEM for certificate/key files
- `internal/component/ipsec/config.go` (ipsec-3) -- IKEGroup, ESPGroup, SiteToSitePeer structs
- `internal/component/iface/` (ipsec-2) -- VTI/XFRM interfaces already created before charon starts
- `internal/core/events/` -- EventBus for SA state change events
- `internal/core/health/` -- Health check registration for tunnel state
- `cmd/ze/hub/main.go` -- Component wiring at startup

### Architectural Verification
- [ ] No bypassed layers (Ze talks to charon via VICI, not to kernel XFRM directly)
- [ ] No unintended coupling (IPsec component depends on PKI store interface, not internals)
- [ ] No duplicated functionality (process supervision follows PPPoE pattern, does not reinvent)
- [ ] Zero-copy preserved where applicable (swanctl.conf generated via bytes.Buffer, not string concat)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config load with `vpn ipsec { site-to-site { peer ... } }` | -> | swanctl.conf generated to /tmp/ze-ipsec/swanctl.conf | `test/ipsec/ipsec-swanctl-generated.ci` |
| IPsec component OnConfigure | -> | CharonSupervisor.Start() launches charon process | `test/ipsec/ipsec-charon-start.ci` |
| `show vpn ipsec sa` CLI command | -> | VICI list-sas query returns SA state | `test/ipsec/ipsec-show-sa.ci` |
| Config reload changes peer remote-address | -> | Reconciler unloads old, loads new connection via VICI | `test/ipsec/ipsec-reload-peer-change.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Ze config has site-to-site peer with IKE group, ESP group, and X.509 auth | swanctl.conf generated with correct connections, authorities, and secrets sections |
| AC-2 | PKI store has CA cert and device cert with private key | Certificate PEM files written to /tmp/ze-ipsec/cacerts/ and /tmp/ze-ipsec/private/; swanctl.conf references these paths |
| AC-3 | IPsec component starts | Charon process started and supervised; restart on crash with exponential backoff (1s base, 60s max) |
| AC-4 | Charon process running | VICI client connects to Unix socket and list-sas returns successfully |
| AC-5 | Peer has connection-type=initiate | VICI `initiate` command sent after charon starts; connection attempt begins |
| AC-6 | IKEv2 SA established or torn down | Bus event published on `vpn/ipsec/sa-up` or `vpn/ipsec/sa-down` with peer name, SA details |
| AC-7 | Config reload changes peer remote-address | Changed connection terminated via VICI, new connection loaded and initiated; other connections untouched |
| AC-8 | Config reload adds new peer / removes existing peer | New peer loaded and initiated via VICI; removed peer terminated and unloaded |
| AC-9 | Remote peer unreachable, DPD timeout expires | Charon detects via DPD; close-action=start triggers reconnection attempt |
| AC-10 | Ze shuts down (SIGTERM) | Charon process terminated cleanly via SIGTERM, VICI client closed, PEM files cleaned up |
| AC-11 | IPsec component active | Health check registered under name "ipsec"; reports healthy when all initiated peers have active child SAs |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGenerateSwanctlConf` | `internal/component/ipsec/swanctl_test.go` | Full swanctl.conf output from IKEGroup + ESPGroup + SiteToSitePeer + cert paths | |
| `TestSwanctlConfX509Auth` | `internal/component/ipsec/swanctl_test.go` | X.509 authentication: local cert, remote id, CA authority section | |
| `TestSwanctlConfPSKAuth` | `internal/component/ipsec/swanctl_test.go` | PSK authentication: ike secret section with correct id selectors | |
| `TestSwanctlConfVTIBind` | `internal/component/ipsec/swanctl_test.go` | VTI binding: if_id_in and if_id_out set on child SA | |
| `TestSwanctlConfDPD` | `internal/component/ipsec/swanctl_test.go` | DPD parameters: dpd_delay, dpd_timeout mapped from IKE group | |
| `TestSwanctlConfProposals` | `internal/component/ipsec/swanctl_test.go` | IKE and ESP proposals rendered in strongSwan format (cipher-hash-dhgroup) | |
| `TestVICICodecEncodeSection` | `internal/component/ipsec/vici_codec_test.go` | VICI section encoding: type byte, length, nested sections, key-value, lists | |
| `TestVICICodecDecodeSection` | `internal/component/ipsec/vici_codec_test.go` | VICI section decoding: roundtrip with nested structures | |
| `TestVICIClientListSAs` | `internal/component/ipsec/vici_test.go` | VICI list-sas command over mock Unix socket | |
| `TestVICIClientInitiate` | `internal/component/ipsec/vici_test.go` | VICI initiate command with connection name | |
| `TestVICIClientTerminate` | `internal/component/ipsec/vici_test.go` | VICI terminate command with connection name | |
| `TestVICIEventSubscription` | `internal/component/ipsec/vici_test.go` | VICI event subscription: register, receive ike-updown event, unregister | |
| `TestCharonSupervisorStart` | `internal/component/ipsec/supervisor_test.go` | Supervisor starts charon binary, captures pid | |
| `TestCharonSupervisorRestart` | `internal/component/ipsec/supervisor_test.go` | Supervisor detects charon exit, restarts with backoff | |
| `TestCharonSupervisorStop` | `internal/component/ipsec/supervisor_test.go` | Supervisor sends SIGTERM, waits for exit, cleans up | |
| `TestReconcilePeersAdded` | `internal/component/ipsec/reconcile_test.go` | New peer in config triggers VICI load-conn + initiate | |
| `TestReconcilePeersRemoved` | `internal/component/ipsec/reconcile_test.go` | Removed peer triggers VICI terminate + unload-conn | |
| `TestReconcilePeersChanged` | `internal/component/ipsec/reconcile_test.go` | Changed peer triggers unload + load + initiate | |
| `TestReconcilePeersUnchanged` | `internal/component/ipsec/reconcile_test.go` | Unchanged peer not touched | |
| `TestIPsecHealthCheck` | `internal/component/ipsec/health_test.go` | Health check reports healthy/degraded based on SA state | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| DPD interval | 1-3600 seconds | 3600 | 0 | 3601 |
| DPD timeout | 1-86400 seconds | 86400 | 0 | 86401 |
| IKE lifetime | 0 (unlimited) or 60-86400 | 86400 | 59 (non-zero) | 86401 |
| ESP lifetime | 60-86400 seconds | 86400 | 59 | 86401 |
| Reconnect backoff | 1s base, 60s max | 60s | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-swanctl-generated` | `test/ipsec/ipsec-swanctl-generated.ci` | Config with site-to-site peer produces valid swanctl.conf | |
| `ipsec-charon-start` | `test/ipsec/ipsec-charon-start.ci` | Charon process started on config load | |
| `ipsec-show-sa` | `test/ipsec/ipsec-show-sa.ci` | `show vpn ipsec sa` returns SA state | |
| `ipsec-reload-peer-change` | `test/ipsec/ipsec-reload-peer-change.ci` | Config reload restarts changed peer connection | |

## Files to Modify
- `internal/component/ipsec/register.go` -- wire CharonSupervisor and reconciler into component lifecycle (OnConfigure, OnShutdown)
- `cmd/ze/hub/main.go` -- wire IPsec component at startup (import for init() registration)
- `internal/component/ipsec/config.go` -- add IPsecConfig.Interface binding field consumed by swanctl generator

## Files to Create
- `internal/component/ipsec/swanctl.go` -- swanctl.conf generation from typed IPsec structs + PKI export paths
- `internal/component/ipsec/swanctl_test.go` -- unit tests for swanctl.conf generation
- `internal/component/ipsec/supervisor.go` -- CharonSupervisor: start/stop/restart charon process with backoff
- `internal/component/ipsec/supervisor_test.go` -- unit tests for supervisor lifecycle
- `internal/component/ipsec/vici.go` -- VICI protocol client: connect, command, event subscription
- `internal/component/ipsec/vici_test.go` -- unit tests for VICI client (mock socket)
- `internal/component/ipsec/vici_codec.go` -- VICI binary section encoding and decoding
- `internal/component/ipsec/vici_codec_test.go` -- codec roundtrip tests
- `internal/component/ipsec/reconcile.go` -- config reload reconciliation: diff peers, load/unload/initiate via VICI
- `internal/component/ipsec/reconcile_test.go` -- reconciler unit tests
- `internal/component/ipsec/events.go` -- bus event types for vpn/ipsec/sa-up, sa-down, sa-rekeyed
- `internal/component/ipsec/health.go` -- health check registration for IPsec tunnel state
- `internal/component/ipsec/health_test.go` -- health check unit tests
- `test/ipsec/ipsec-swanctl-generated.ci` -- functional test for swanctl.conf generation
- `test/ipsec/ipsec-charon-start.ci` -- functional test for charon process start
- `test/ipsec/ipsec-show-sa.ci` -- functional test for show vpn ipsec sa
- `test/ipsec/ipsec-reload-peer-change.ci` -- functional test for config reload with peer change

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | `internal/component/ipsec/schema/ze-ipsec-conf.yang` (extends from ipsec-3) |
| CLI commands/flags | [ ] | Deferred to ipsec-5 (CLI/Diag spec) |
| Editor autocomplete | [ ] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [ ] | `test/ipsec/*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` (IPsec VPN: have) |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md` (vpn ipsec site-to-site examples) |
| 3 | CLI command added/changed? | [ ] | Deferred to ipsec-5 |
| 4 | API/RPC added/changed? | [ ] | N/A (VICI is internal) |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | `docs/guide/ipsec.md` (new) |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/rfc7296.md` (IKEv2 via strongSwan) |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` (IPsec support) |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` (new ipsec component) |

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
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- register IPsec component, create entry points
   - Tests: wiring tests from Wiring Test table (failing stubs)
   - Files: `register.go` (extend), `cmd/ze/hub/main.go` (import)
   - Verify: component registered, OnConfigure called, stubs fail gracefully

2. **Phase: VICI Codec** -- binary section encoding/decoding for VICI protocol
   - Tests: `TestVICICodecEncodeSection`, `TestVICICodecDecodeSection`
   - Files: `vici_codec.go`, `vici_codec_test.go`
   - Verify: roundtrip encode/decode of nested sections, key-values, lists

3. **Phase: VICI Client** -- connect, command, event subscription over Unix socket
   - Tests: `TestVICIClientListSAs`, `TestVICIClientInitiate`, `TestVICIClientTerminate`, `TestVICIEventSubscription`
   - Files: `vici.go`, `vici_test.go`
   - Verify: commands sent and responses parsed; event stream decoded

4. **Phase: swanctl.conf Generation** -- translate Ze config to strongSwan config file
   - Tests: `TestGenerateSwanctlConf`, `TestSwanctlConfX509Auth`, `TestSwanctlConfPSKAuth`, `TestSwanctlConfVTIBind`, `TestSwanctlConfDPD`, `TestSwanctlConfProposals`
   - Files: `swanctl.go`, `swanctl_test.go`
   - Verify: generated config matches expected output for all auth modes and binding types

5. **Phase: Charon Supervisor** -- process lifecycle management
   - Tests: `TestCharonSupervisorStart`, `TestCharonSupervisorRestart`, `TestCharonSupervisorStop`
   - Files: `supervisor.go`, `supervisor_test.go`
   - Verify: charon started, crash detected and restarted with backoff, clean shutdown on Stop

6. **Phase: Reconciliation** -- config reload diffs peers, loads/unloads via VICI
   - Tests: `TestReconcilePeersAdded`, `TestReconcilePeersRemoved`, `TestReconcilePeersChanged`, `TestReconcilePeersUnchanged`
   - Files: `reconcile.go`, `reconcile_test.go`
   - Verify: peer diffs correct, VICI commands issued in right order

7. **Phase: Events and Health** -- bus events for SA state, health check registration
   - Tests: `TestIPsecHealthCheck`
   - Files: `events.go`, `health.go`, `health_test.go`
   - Verify: bus events published on SA changes, health check reports correct state

8. **Functional tests** -- create after feature works
9. **Full verification** -- `make ze-verify`
10. **Complete spec** -- audit tables, learned summary, delete spec

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | swanctl.conf output matches strongSwan expected format; VICI codec handles all type bytes |
| Naming | Bus event topics use `vpn/ipsec/` prefix; VICI command names match strongSwan docs |
| Data flow | Config structs flow through swanctl generator only; VICI client is the sole charon interface |
| Rule: no-layering | No direct XFRM netlink calls from ipsec component (charon manages XFRM) |
| Rule: exact-or-reject | Unknown IKE/ESP algorithms rejected at config parse, not silently dropped |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| swanctl.conf generator produces valid output | `go test -run TestGenerateSwanctlConf` passes |
| VICI codec roundtrips all section types | `go test -run TestVICICodec` passes |
| VICI client connects and commands work | `go test -run TestVICIClient` passes |
| Charon supervisor manages process lifecycle | `go test -run TestCharonSupervisor` passes |
| Config reconciliation diffs peers correctly | `go test -run TestReconcilePeers` passes |
| Health check registered | `grep -rn 'RegisterCheck.*ipsec' internal/component/ipsec/` |
| Bus events published | `grep -rn 'Publish.*vpn/ipsec' internal/component/ipsec/` |
| Functional tests exist | `ls test/ipsec/ipsec-*.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | VICI responses from charon socket are untrusted; validate section structure before accessing fields |
| Private key handling | PEM files written with mode 0600; cleaned up on shutdown; never logged |
| Process execution | Charon binary path validated (no path traversal); arguments are fixed, not user-injectable |
| Socket permissions | VICI socket path validated; connection timeout prevents hanging on stale socket |
| Temporary files | /tmp/ze-ipsec/ directory created with restricted permissions; cleaned up on shutdown |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior, RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural, DESIGN phase |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

Add `// RFC 7296 Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: DPD behavior, rekeying triggers, AUTH method selection, SA lifetime constraints.

## Implementation Summary

### What Was Implemented
- (to be filled)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- (to be filled)

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ipsec-4-strongswan.md`
- [ ] Summary included in commit
