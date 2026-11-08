# Spec: openvpn

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | OpenVPN is a VPN Ze does not offer; IPsec and WireGuard ship without it |
| RFC gaps | - |
| Depends | - |
| Phase | 0/0 (the owner approves the design route before work starts) |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

OpenVPN is the VPN many operators already run for remote access and for
site-to-site links to sites that cannot run IPsec or WireGuard. VyOS offers it
in site-to-site, client and server modes. Ze offers IKEv2/IPsec and WireGuard,
and no OpenVPN.

The goal: Ze terminates and originates OpenVPN tunnels that interoperate with
OpenVPN 2.6 and 2.7, in site-to-site, client and server modes, with the data
path in the Linux kernel through the `ovpn` data channel offload (DCO) module.

OpenVPN has no RFC. The authority is the OpenVPN 2.7 source and its protocol
documentation, read at design.

## Design route (owner decision before design)

Ze's appliance image carries no external binaries (`internal/appliance/kernelreq.go`
header: "The image ships two ze binaries and gokrazy randomd and heartbeat"),
and an external command needs an entry in `ai/allowed-system-commands.md`
(`ai/rules/go-standards.md`, "External Commands"). That rules out the route
VyOS takes, rendering a config file for the `openvpn` daemon, unless the owner
decides otherwise.

| Route | Data path | Control channel | Cost |
|-------|-----------|-----------------|------|
| A: native | kernel `ovpn` DCO, configured over its generic netlink family | Go implementation of the OpenVPN control channel (reliability layer, TLS, key method 2, peer-id, data channel negotiation) | large; no third-party Go module known, so each new module is put to the owner first (`ai/META-DECISIONS.md`) |
| B: supervise the `openvpn` binary | DCO through the daemon | the daemon | breaks the no-external-binary image and needs the binary in the image |

Route A is the one consistent with the rules above; the owner chooses.

## VyOS reference

VyOS checkout `~/Code/github.com/vyos/vyos-1x`, `interfaces openvpn`. Each
recent fix is a pitfall Ze's design owns from the start:

| Ticket and commit | VyOS defect or change | Ze acceptance criterion |
|-------------------|-----------------------|-------------------------|
| T8264 (`2c92731f1`, `d63485ac8`, `043992126`, `585c491b3`, `06dee60d8`, `963e4cc5b`) | DCO data path: create the interface type DCO needs, keep the kernel module loaded, refuse the raw options DCO cannot serve, migrate away configs DCO cannot serve, disturb the interface only when the daemon restarts | AC-1, AC-8, AC-9 |
| T9370 (`8d99a2f6f`) | add CHACHA20-POLY1305 as a data cipher, allowed with DCO | AC-2 |
| T9363 (`b6abe23ba`) | site-to-site passed the failure count as seconds: the keepalive timeout is interval x failure-count | AC-4 |
| T9364 (`12d3aa212`) | the `ovpn` module does no MSS clamping, so a 1500 MTU fragments on the underlay: default 1420 with DCO, refuse below 1280 with IPv6 inside | AC-5 |
| T9334 (`f931a8703`) | OpenVPN 2.7.6 refuses `keepalive 0 0`; an interval of 0 must mean keepalive off | AC-3 |
| T9337 (`aaeb1dae0`) | `ping 0` with the restart timer still armed tore down an idle tunnel | AC-3 |
| T9386 (`0d49d1ba8`) | static-key mode supports only CBC, so an AEAD fallback cipher with a shared secret key cannot start | AC-6 |
| T9560 (`432fa1807`) | `tls-auth` was accepted in site-to-site mode and never used; key direction follows the role | AC-7 |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - how Ze installs a kernel data path it negotiated, the model the DCO install follows
- [ ] `docs/features/interfaces.md` - the interface kinds and their backends
- [ ] `ai/rules/platform-linux.md` - the kernel requirement list and its proof

**Key insights:**
- The `ovpn` module is in Linux since 6.16; `CONFIG_OVPN` joins
  `runtimeKernelRequirements` in `internal/appliance/kernelreq.go`, built in,
  because gokrazy ships no modprobe.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/appliance/kernelreq.go` - `runtimeKernelRequirements` carries
  `CONFIG_WIREGUARD`, XFRM and the crypto transforms; no `CONFIG_OVPN`.
- No OpenVPN code, YANG or interface kind exists in Ze.

**Behavior to preserve:**
- Every other interface kind and VPN.

**Behavior to change:**
- None; OpenVPN is new.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Interface backend and kinds | `internal/component/iface/backend.go` `Backend` | Creates and configures each interface kind, WireGuard included | extends: an OpenVPN interface kind, created and configured through the backend; VPP refuses it until it has a data path |
| Kernel requirement list | `internal/appliance/kernelreq.go` `runtimeKernelRequirements` | Names the kernel options Ze's image must carry | extends: `CONFIG_OVPN` |
| PKI store | `docs/architecture/pki/pki-store.md` | Certificates and CAs for TLS listeners and IKE | extends: OpenVPN TLS certificates come from it, not from files |
| IKE kernel install model | `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` | A control plane in Go installs keys into a kernel data path | extends as a pattern: the control channel installs DCO keys over netlink |
| RADIUS subscriber auth | `internal/component/l2tp/plugins/authradius` | RADIUS for PPP subscribers | does not apply until server mode user auth is designed |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: an OpenVPN interface with mode, peer, TLS, cipher, keepalive and MTU leaves.
- Wire: the OpenVPN protocol on UDP or TCP.

### Transformation Path
1. to be designed after the owner chooses the route

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| control channel to kernel | `ovpn` generic netlink (route A) | No |

### Changed-Wire Census

**Search evidence:** none; a new protocol.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| OpenVPN | writer and reader | to be designed | none | new |

### Integration Points
- to be designed

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to be designed |
| No unintended coupling (components stay isolated) | No | to be designed |
| No duplicated functionality (extends existing, does not recreate) | No | to be designed |
| Zero-copy preserved where applicable (refs, not copies) | No | to be designed |
| Registration over hardcoding, outbound | No | to be designed |
| Registration over hardcoding, inbound | No | to be designed |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Ze's kernel line carries the `ovpn` module | Linux 6.16 merged it; Ze runs a newer kernel (`kernelreq.go` names Linux 7.2) | route A needs a kernel change first | `./le test qemu all` with `CONFIG_OVPN` | unvalidated |
| A-2 | DCO serves only AEAD ciphers (AES-GCM, CHACHA20-POLY1305) | VyOS T8264 refuses the options DCO cannot serve | the cipher list differs | `ovpn` source read at design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The control channel is large and unspecified outside the OpenVPN source | design estimate | the owner chooses the route with the estimate in hand |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | OpenVPN tunnels only; nothing exists today |
| How is it reverted? | single commit revert per phase |
| Who else touches this path? | the iface component and the appliance kernel list |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| OpenVPN interface config, site-to-site | → | tunnel up, DCO keys installed | `openvpn-site-to-site` interop scenario |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | site-to-site and client/server against OpenVPN 2.7 | the tunnel comes up and traffic passes through the kernel DCO data path | none |
| AC-2 | data cipher CHACHA20-POLY1305, AES-256-GCM, AES-128-GCM | each negotiates with OpenVPN 2.7 and is accepted with DCO | none |
| AC-3 | keepalive interval 0 | keepalive is off: no ping is sent and no restart timer runs, so an idle tunnel stays up | none |
| AC-4 | keepalive interval I and failure count N | the peer is declared dead after I x N seconds, never N seconds; a product above the protocol's limit is refused at commit | none |
| AC-5 | DCO with no MTU set | MTU 1420; an MTU below 100, or below 1280 with IPv6 in the tunnel, is refused | none |
| AC-6 | static-key mode with an AEAD cipher or fallback cipher | refused at commit, naming the leaf | none |
| AC-7 | site-to-site with a tls-auth key | the key is used, its direction taken from the role | none |
| AC-8 | a config option DCO cannot serve | refused at commit with the option name and the reason | none |
| AC-9 | a commit that changes nothing the tunnel uses | the interface and its sessions are not disturbed | none |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| to be designed | placed at design | AC-3 to AC-6, AC-8 config refusals | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `openvpn-site-to-site` | `test/interop/scenarios/` | OpenVPN 2.7 | AC-1 to AC-4, AC-7 | |
| `openvpn-server-client` | `test/interop/scenarios/` | OpenVPN 2.7 client | AC-1, AC-2 | |

## Files to Modify
- `internal/appliance/kernelreq.go` - `CONFIG_OVPN`
- `internal/component/iface/backend.go` - the OpenVPN kind

## Files to Create
- placed at design

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| to be answered at design | | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| to be answered at design | | | |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Open: route A or route B | see "Design route" | the owner's choice; it decides whether the image carries an external binary |

## Implementation Steps

1. **Phase: Wiring** -- to be designed

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Targeted tests pass: the two scenarios through `./le test integration`, and `./le test qemu all` for the kernel module

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
