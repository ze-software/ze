# Spec: iface-tunnel-identity-collision

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P1 |
| Release reason | Two tunnels the kernel cannot tell apart pass verify and fail the commit at apply with EEXIST: a config check that lives only on the apply path |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The Linux kernel keeps one table of tunnels per encapsulation and network
namespace, and refuses a second tunnel whose lookup identity is already in that
table with EEXIST. Ze's verify never compares tunnels with each other, so two
`interface tunnel` entries with the same identity commit through `ze config
validate` and verify, and the second create fails at apply.

The goal: verify refuses two tunnels the kernel would see as one, naming both
entries, the shared identity, and the fix. It accepts every pair the kernel keeps
apart, so no valid config is refused.

Found by comparing recent VyOS fixes against Ze. VyOS fixed the same class in
T9193, in `~/Code/github.com/vyos/vyos-1x`:

| Commit | What it encodes |
|--------|-----------------|
| 1a9181807 | Two GRE tunnels collide only when local address, remote, source interface (link) and key all match, as `ip_tunnel_find()` in `net/ipv4/ip_tunnel.c` decides. An any address counts as unset. Keys compare as the value the kernel stores |
| b3c4828f5 | gre and gretap live in tables of their own, so a gre and a gretap tunnel never collide |
| 4bdf9c3a6 | A zero key is no key only where neither endpoint is set |

VyOS checks the GRE family only. Ze has the same exposure for every kind it
creates, so the design reads the kernel lookup of each one.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md`, section 14a - exact or refuse, at verify
  → Constraint: verify reads no kernel. The identity is computed from config alone
- [ ] The kernel source at the version Ze's kernel runs: `ip_tunnel_find` (`net/ipv4/ip_tunnel.c`, gre, gretap, ipip, sit), `ip6gre_tunnel_find` (`net/ipv6/ip6_gre.c`), `ip6_tnl_locate` (`net/ipv6/ip6_tunnel.c`), and the VNI check of `vxlan` (`drivers/net/vxlan/vxlan_core.c`)
  → Constraint: the identity tuple and the table of each kind come from that source, quoted with the function name, never from VyOS alone

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/register.go` `parseAndVerifyIfaceSections` - runs `validateUniqueMatchMAC`, delegated-prefix and VPP QoS checks. No check compares two tunnels
- [ ] `internal/component/iface/tunnel.go` `TunnelSpec`, `kernelLinkTypes` - the identity fields (`LocalAddress`, `LocalInterface`, `RemoteAddress`, `Key`/`KeySet`, `VNI`, `Port`) and the kernel table of each kind. `ip6tnl` and `ipip6` share the kernel type `ip6tnl`
- [ ] `internal/plugins/iface/netlink/tunnel_linux.go` `buildGretun` - the kernel receives local, remote, the link index from `LocalInterface`, and the key as both IKey and OKey

**Behavior to preserve:**
- Every pair of tunnels the kernel accepts still commits
- A tunnel's own checks (remote mandatory and not unspecified, address family) stay

**Behavior to change:**
- Two tunnels with one kernel identity are refused at verify

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Pairwise uniqueness at verify | `internal/component/iface/config.go` `validateUniqueMatchMAC`, called from `parseAndVerifyIfaceSections` | Refuses two ethernet entries that select one MAC | extends: the same shape, a pure pass over `cfg.Tunnel` keyed by the kernel identity |
| Kernel table per kind | `internal/component/iface/tunnel.go` `kernelLinkTypes` | Maps each kind to the kernel link type | extends: the table key of the identity, so `ip6tnl` and `ipip6` share one table, and gre and gretap do not |
| Exact or refuse at verify | `docs/architecture/core-design.md`, section 14a | A backend that cannot apply a config refuses it at verify | extends: this is one such refusal |

## Data Flow (MANDATORY)

### Entry Point
- `ze config validate` or a commit carrying two or more `interface tunnel` entries

### Transformation Path
1. `parseTunnelEntry` builds each `TunnelSpec`
2. Verify keys each spec by kernel table and identity tuple, and refuses a duplicate key

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| config to iface plugin | verify stream event | No |

### Integration Points
- `parseAndVerifyIfaceSections`, `kernelLinkTypes`

## Risks & Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Ze always sets a remote, so the zero-key special case of 4bdf9c3a6 never arises | the YANG `container remote` is mandatory, and the netlink backend's `parseTunnelRemote` refuses an unspecified remote at apply | the identity must model the any-remote case | design reads the parser and the YANG | unvalidated |
| A-2 | The VPP backend refuses the same pairs | not read | VPP may need its own rule | design reads the VPP tunnel create | unvalidated |
| A-3 | A tunnel with `LocalInterface` has the link index as part of its identity, and no local address | `buildGretun` sets `Link` | identity wrong for a local-interface tunnel | kernel source read at design | unvalidated |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` with two gre tunnels of one identity | → | verify tunnel identity check | `test/parse/iface-tunnel-identity-duplicate.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Two gre tunnels with the same local, remote, link and key | Verify refuses, naming both tunnels and the shared tuple | none |
| AC-2 | A gre and a gretap tunnel with the same local, remote and key | Both commit and both devices exist | both: the kernel keeps them in separate tables |
| AC-3 | Two gre tunnels that differ only in key | Both commit | both: the kernel keeps them apart |
| AC-4 | An `ip6tnl` and an `ipip6` tunnel with the same local, remote and link | Verify refuses (one kernel table) | none |
| AC-5 | Each other kind (ipip, sit, ip6gre, ip6gretap, vxlan) with a duplicate identity as its kernel lookup defines it | Verify refuses | none |
| AC-6 | A config that passed AC-1's verify on today's code | Today it fails at apply with EEXIST; after the fix it never reaches apply | none |

### Tests that prove each AC

| AC | Test | Entry point |
|----|------|-------------|
| AC-1, AC-4, AC-5 | new `test/parse/iface-tunnel-identity-duplicate.ci`, one refusal per kind | `ze config validate` |
| AC-2, AC-3 | new `test/plugin/iface-tunnel-identity-distinct.ci`, `option=needs-linux`: commit and read both devices back | config commit through the daemon |
| AC-6 | `TestVerifyTunnelIdentity` in `internal/component/iface/register_test.go`, a table over every kind | `parseAndVerifyIfaceSections` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVerifyTunnelIdentity` | `internal/component/iface/register_test.go` | AC-1 to AC-5 | |

## Files to Modify
- `internal/component/iface/register.go` - the verify step
- `internal/component/iface/tunnel.go` - the identity key per kind

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | No | a cross-entry check, not a leaf constraint |
| Doctor check | No | no new runtime dependency |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 6 | Has a user guide page? | Yes | the tunnel guide names the refusal |

## Implementation Steps
1. **Phase: Wiring** -- the failing `.ci`
2. **Phase: Kernel read** -- quote each kind's lookup into the Design Insights
3. **Phase: Verify check** -- AC-1 to AC-5

## Checklist

### Goal Gates (MUST pass)
- [ ] `./le test functional parse` and `./le test functional plugin` for the new `.ci` files; the iface unit package through `./le job run`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `validate-accepts-what-start-refuses.md` | 2026-11-08 | duplicate kernel tunnel identity fails at apply with EEXIST |
