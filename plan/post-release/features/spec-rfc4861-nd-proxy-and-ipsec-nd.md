# Spec: rfc4861-nd-proxy-and-ipsec-nd

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | Owner decision of 2026-11-24 (split from `plan/release/fixes/spec-rfc4861-ze-owned-must-gaps.md`): Neighbor Discovery proxy and AH/ESP-secured Neighbor Discovery are unstarted RFC 4861 features, moved to `plan/post-release/features/` as honest post-release gaps (not declined) under `ai/META-DECISIONS.md` release timing |
| RFC gaps | RFC4861-7.2.8-1, RFC4861-7.2.8-2, RFC4861-11.2-1, RFC4861-11.2-2 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Origin and Owner Ruling

Split from `plan/release/fixes/spec-rfc4861-ze-owned-must-gaps.md` per Thomas's owner decision on 2026-11-24:

- Decision: Move ND Proxy (`RFC4861-7.2.8-1`, `RFC4861-7.2.8-2`) and AH/ESP-secured ND (`RFC4861-11.2-1`, `RFC4861-11.2-2`) to `plan/post-release/features/` as honest post-release gaps (not declined).
- Reciprocal release spec: `plan/release/fixes/spec-rfc4861-ze-owned-must-gaps.md` closes the nine shipped-capability RFC 4861 MUST gaps.

## Task

Implement the two unstarted RFC 4861 capabilities whose MUST rows remain `{gap}` in `rfc/short/rfc4861.md`:

| Row | Capability | What is missing |
|-----|-----------|-----------------|
| RFC4861-7.2.8-1, RFC4861-7.2.8-2 | Neighbor Discovery Proxy | No Ze configuration or dataplane code enables Neighbor Discovery proxying: nothing joins the solicited-node multicast address(es) corresponding to proxied IPv6 addresses, nothing sets `net.ipv6.conf.<dev>.proxy_ndp` or installs kernel proxy neighbour entries, and nothing ensures solicited proxy Neighbor Advertisements carry Override flag 0 |
| RFC4861-11.2-1, RFC4861-11.2-2 | AH/ESP-secured Neighbor Discovery | No Ze configuration secures IPv6 Neighbor Discovery with IPsec AH or ESP, so nothing installs the per-link XFRM policies and states to authenticate ND messages or verifies that ND packets failing authentication checks are silently discarded |

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` - interface YANG model, kernel backend, and sysctl integration
  → Constraint: kernel tunables go through `sysctl` over the EventBus; features that program only the Linux kernel carry `ze:backend "kernel"`
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - XFRM SA/SP installation contract and Linux kernel AH/ESP requirements
  → Constraint: XFRM transport-mode states are keyed by destination address (`plan/journal/unwired-feature.md` 2026-09-14 row)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4861.md` and `rfc/full/rfc4861.txt`, Sections 7.2.8 and 11.2
  → Constraint: solicited-node multicast join for every proxied address; Override flag 0 on solicited proxy NAs; AH/ESP verification and silent discard of packets failing authentication

## Current Behavior (MANDATORY)

**Source files read:** (skeleton; design phase re-reads each target before any decision)
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - carries `ipv4 proxy-arp` but no `ipv6` Neighbor Discovery proxy leaf/list and no IPsec-for-ND configuration
- [ ] `internal/component/iface/config_sysctl.go` - emits `net.ipv4.conf.<dev>.proxy_arp`, no `net.ipv6.conf.<dev>.proxy_ndp`
- [ ] `internal/component/ike/dataplane/` - installs XFRM SAs/SPs for IKEv2 and OSPFv3 (RFC 4552), none for IPv6 Neighbor Discovery

**Behavior to preserve:**
- All existing non-proxy IPv6 Neighbor Discovery and Router Advertisement behavior when ND proxy and IPsec-secured ND are not configured

**Behavior to change:**
- Add opt-in Neighbor Discovery proxy configuration and enforcement (solicited-node multicast join and Override=0 solicited proxy NAs)
- Add opt-in AH/ESP Neighbor Discovery authentication configuration and enforcement (verification and silent discard on authentication failure)

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Interface sysctl emission | `internal/component/iface/config_sysctl.go` `applySysctl` | Emits per-interface IPv4/IPv6 sysctl defaults including `proxy_arp` via EventBus | extends (design phase): emit `net.ipv6.conf.<dev>.proxy_ndp` when ND proxy is configured |
| Kernel interface backend | `internal/plugins/iface/netlink/` | Manages links, addresses, routes, and neighbor reads via netlink | extends (design phase): manage kernel proxy neighbour entries (`NTF_PROXY`) and solicited-node group joins |
| XFRM dataplane seam | `internal/component/ike/dataplane/` | Installs Linux XFRM SAs and SPs for IKEv2 and OSPFv3 | extends (design phase): reuse the ungated XFRM dataplane seam for AH/ESP ND policies and states |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Skeleton: operator YANG configuration under `interface <kind> <name> unit <u> ipv6` for ND proxy and ND IPsec authentication (exact schema to be decided at design).

### Transformation Path
1. To be designed: YANG parse and cross-leaf verification in `internal/component/iface/`.
2. To be designed: ND proxy sysctl + netlink `NTF_PROXY` / solicited-node multicast join + Override=0 proof against Linux kernel ND.
3. To be designed: XFRM AH/ESP state and policy installation for ICMPv6 ND messages (types 133-137) and silent-discard proof for unauthenticated packets.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to iface component | config transaction verify/apply | No |
| iface component to kernel / XFRM | netlink proxy neighbour entries, sysctl EventBus, `ike/dataplane` XFRM seam | No |

### Changed-Wire Census

**Search evidence:** skeleton — to be completed during design against the chosen ND proxy and AH/ESP ND wire paths.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| ICMPv6 Neighbor Solicitation / Advertisement (proxy) | writer | to be designed (Linux kernel `net/ipv6/ndisc.c` driven by Ze `proxy_ndp` + `NTF_PROXY`, or Ze handler) | to be verified at design | preserve standard RFC 4861 NA format with Override=0 on solicited proxy NA |
| ICMPv6 Neighbor Solicitation / Advertisement (proxy) | reader | remote IPv6 neighbor on the link (outside repository) and Linux kernel `net/ipv6/ndisc.c` | to be verified at design | preserve standard RFC 4861 NS/NA format |

### Integration Points
- To be designed: `internal/component/iface/`, `internal/plugins/iface/netlink/`, `internal/component/ike/dataplane/`, `internal/core/sysctl/`.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Operator configures ND proxy for a target IPv6 address on a unit | → | ND proxy netlink/sysctl setup | `TestRFC4861NDProxyJoinsSolicitedNodeAndClearsOverride` (planned, QEMU Linux integration) |
| Operator configures AH/ESP authentication for ND on a unit | → | XFRM ND SA/SP installer | `TestRFC4861IPsecNDVerifiesAndDiscardsUnauthenticated` (planned, QEMU Linux integration) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Unit configured with ND proxy for a proxied IPv6 address | The interface joins the corresponding solicited-node multicast group and answers Neighbor Solicitations with a Neighbor Advertisement whose Override flag is 0 (`RFC4861-7.2.8-1`, `RFC4861-7.2.8-2`) | none |
| AC-2 | Unit configured with AH or ESP protection for ND receives authenticated and unauthenticated ND packets | Authenticated ND packets are accepted and processed; ND packets failing authentication checks are silently discarded (`RFC4861-11.2-1`, `RFC4861-11.2-2`) | none |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC4861NDProxyConfigValidation` | `internal/component/iface/rfc4861_nd_proxy_test.go` | Config parse and validation of ND proxy targets | |
| `TestRFC4861IPsecNDConfigValidation` | `internal/component/iface/rfc4861_ipsec_nd_test.go` | Config parse and validation of AH/ESP ND protection | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-nd-proxy` | `test/plugin/iface-nd-proxy.ci` | Operator configures ND proxy and verifies committed state | |
| `iface-ipsec-nd` | `test/plugin/iface-ipsec-nd.ci` | Operator configures AH/ESP ND protection and verifies committed state | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `nd-proxy-and-ipsec-nd` | `test/interop/scenarios/` | Linux host / strongSwan | Solicited-node proxy NA with Override=0 and AH/ESP ND authentication/discard over the wire | |

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `rfc-extension-not-implemented.md` | 2026-10-26 | IPv6 Neighbor Discovery (iface RA, kernel ND) |

## Files to Modify

Skeleton: to be confirmed at design.

- `internal/component/iface/yang/ze-iface-conf.yang` - ND proxy and AH/ESP ND configuration nodes
- `internal/component/iface/config_sysctl.go` - `proxy_ndp` sysctl default emission
- `internal/plugins/iface/netlink/` - proxy neighbour entry and solicited-node join management
- `rfc/short/rfc4861.md` - remove `{gap}` from `RFC4861-7.2.8-1`, `RFC4861-7.2.8-2`, `RFC4861-11.2-1`, `RFC4861-11.2-2`
- `docs/features/interfaces.md` - document ND proxy and AH/ESP ND

## Files to Create

- `internal/component/iface/rfc4861_nd_proxy_test.go` - ND proxy config and integration tests
- `internal/component/iface/rfc4861_ipsec_nd_test.go` - AH/ESP ND config and integration tests

## Implementation Steps

Skeleton: to be ordered and confirmed at design.

1. **Phase: Wiring (MANDATORY FIRST)** -- YANG schema, config parsing, and backend/XFRM entry points for ND proxy and AH/ESP ND.
2. **Phase: ND Proxy** -- solicited-node multicast join, `proxy_ndp` / `NTF_PROXY` programming, Override=0 proof (`RFC4861-7.2.8-1`, `RFC4861-7.2.8-2`).
3. **Phase: AH/ESP ND** -- XFRM policy and state installation for ND messages and silent-discard proof (`RFC4861-11.2-1`, `RFC4861-11.2-2`).
4. **Phase: Records** -- `./le rfc record` for every new tagged test and `./le rfc check`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Current Behavior, Existing Mechanisms, Design documents, Design presentation, and Data Flow sections completed
- [ ] AC-N rows carry testable positive assertions and Permitted Survivors

### Goal Gates (MUST pass)
- [ ] AC-1..AC-2 demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted scoped tests pass in the main checkout
- [ ] Every owned journal row resolved

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + journal rows
- [ ] **Commit B:** `remove plan/post-release/features/spec-rfc4861-nd-proxy-and-ipsec-nd.md`
