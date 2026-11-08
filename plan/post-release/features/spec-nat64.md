# Spec: nat64

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | Stateful NAT64 is a capability VyOS offers and Ze lacks; no release profile lists RFC 6146, so it is post-release feature work. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An operator running an IPv6-only access network wants its clients to reach
IPv4-only servers: stateful NAT64 translates IPv6 packets addressed to a NAT64
prefix (64:ff9b::/96 or an operator prefix) into IPv4 packets from a pool, and
back. VyOS offers it through Jool. Ze gains a `nat64` configuration: the
translation prefix, the IPv4 pool, and the interfaces it applies to. Both
data planes translate: the kernel backend through Jool, the VPP backend through
VPP's nat64 plugin.

## Owner Decisions

| Decision | What Thomas decided | What it fixes in this spec |
|----------|---------------------|----------------------------|
| D1: which layer translates | Both data planes translate: Jool, carried in Ze's kernel, on the kernel (nft) backend, and VPP's nat64 plugin on the VPP backend. A userspace TUN translator is not built | Two backend design sections below, each with its own ACs. Neither backend refuses NAT64 for want of a translator |
| D2: govpp `binapi/nat64` | No approval needed: every module already in `go.mod` may be used in full, except experimental crypto modules such as Cloudflare's `circl`, and `go.fd.io/govpp` is already required | The vendored govpp tree gains `binapi/nat64` (today it carries `nat44_ed` and `nat_types` only). It is a new package of an already-required module, so `go mod vendor` adds it in the same commit as the code that first imports it |
| D3: RFC enrolment | RFC 6146 (stateful NAT64), RFC 6052 (address format) and RFC 7915 (SIIT) are not enrolled in any profile and get no `rfc/short/` summary: RFC validation scope does not grow, and RFCs no profile lists wait until after the first release | The feature ships with no conformance figure, ledger rows or discrimination records; code still quotes the RFC sentences it implements |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/forwarding.md` - backend split, exact-or-refuse, nft table ownership
- [ ] `ai/doors/appliance.md` - the kernel build and the runtime kernel the QEMU proofs boot
- [ ] `docs/architecture/firewall/fw-6-firewall-vpp.md` - the VPP NAT backend
- [ ] `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - what Ze owns in the kernel
- [ ] `docs/architecture/appliance/kernel-profiles.md` - config fragments, the patch series, and how gokrazy loads modules; Jool reaches Ze's kernel by this route

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/model.go` `SNAT`, `DNAT`, `Masquerade` - same-family NAT only; no cross-family translation.
- [ ] `internal/component/firewall/config.go` `parseNATSpec` - one address family per target.
- [ ] `internal/plugins/firewall/vpp/backend_linux.go` - imports `nat44_ed` and `nat_types` only.
- [ ] `internal/plugins/firewall/vpp/nat_linux.go` `applyNATChains` - NAT44-ED static mappings and address ranges.
- [ ] `internal/plugins/firewall/vpp/ops.go` `vppOps` - the nat44 calls behind the test seam.
- [ ] `internal/plugins/firewall/vpp/verify.go` `verifyNATChain`, `verifyNATTerm` - the VPP verifier that refuses what NAT44-ED cannot apply exactly.
- [ ] `gokrazy/kernel/kernel.config`, `runtime.config`, `patches/` - no Jool source or config today.
- [ ] No NAT64, Jool or Tayga code exists under `internal/` (grep).

**Behavior to preserve:** NAT44 and IPv6 NAT.

**Behavior to change:** add NAT64 on both backends.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Firewall backend registry | `internal/component/firewall/backend.go` `Backend`, `RegisterBackend` | nft and VPP backends program NAT44/NAT66 | Extends: the NAT64 model is handed to the selected backend, which verifies and applies it |
| nft firewall backend | `internal/plugins/firewall/nft/` | Lowers NAT44 and NAT66 into `ze_` nft tables | Does not translate: nftables has no NAT64 expression. Its NAT64 apply drives Jool instead (kernel design below) |
| VPP NAT44-ED integration | `internal/plugins/firewall/vpp/nat_linux.go` `applyNATChains`, `ops.go` `vppOps` | Address ranges and static mappings behind a test seam | Extends (D1, D2): new ops for VPP's nat64 plugin behind the same `vppOps` seam, so the unit tests keep a fake |
| VPP firewall verifier | `internal/plugins/firewall/vpp/verify.go` `verifyNATChain` | Refuses what VPP cannot apply exactly | Extends: refuses a NAT64 shape VPP's nat64 plugin cannot express, never approximates |
| Ze's kernel build | `gokrazy/kernel/` (`kernel.config`, `runtime.config`, `patches/`), `docs/architecture/appliance/kernel-profiles.md` | Ze builds and boots its own kernel from config fragments and a patch series | Extends (D1): Jool's kernel module is carried in Ze's kernel by the same route, in both the hardware and the runtime profile |

## Design: kernel backend (Jool)

1. Jool's NAT64 module is built into Ze's kernel through `gokrazy/kernel/`. How (an in-tree patch carrying Jool's source, or a module build beside the kernel) is the first design step, judged against `kernel-profiles.md` and the gokrazy module loader it describes.
2. Ze programs Jool directly over its Generic Netlink family and never runs Jool's userspace tool (`ai/rules/go-standards.md`, external commands). The family and its message layout are read from Jool's source at the version carried, and the kernel function that handles each message is named at design.
3. Jool runs in netfilter mode, one instance per network namespace. The instance, pool4 and the pool6 prefix are created on apply, replaced on change, and removed on withdrawal.
4. A config naming a second instance in one namespace is refused at verify (AC-2).
5. `ze doctor` checks that the running kernel carries the Jool module, and names the fix when it does not.

## Design: VPP backend (nat64 plugin)

1. The vendored govpp tree gains `binapi/nat64` with the code that first imports it (D2); `vppOps` gains the nat64 calls: plugin enable, pool address range, prefix, inside and outside interface flags.
2. The VPP verifier refuses any NAT64 shape the plugin cannot apply exactly, with the YANG path and the fix.
3. Apply and withdrawal follow the same reconcile as NAT44-ED (`applyNATChains`, `cleanupNATOrphans`), so a reload leaves no stale pool, prefix or interface flag in VPP.
4. Which VPP calls exist, and their semantics (prefix lengths accepted, per-VRF prefixes), are read from VPP's nat64 plugin source at the version Ze runs, with the deciding function named at design.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `nat64` block with prefix, pool and interfaces (final placement at design).

### Transformation Path
1. Parse and verify prefix length (RFC 6052 lengths), pool and interfaces.
2. The selected backend verifies the model it can apply exactly.
3. The kernel backend programs Jool over Generic Netlink; the VPP backend programs the nat64 plugin over the binary API. Each withdraws its state on removal.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → firewall component | YANG tree | No |
| nft backend → Jool | Generic Netlink to the Jool module in Ze's kernel | No |
| VPP backend → nat64 plugin | govpp `binapi/nat64` | No |

### Integration Points
- Firewall backend verify and apply, on both backends
- `ze doctor`

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `nat64` config, kernel backend | → | Jool programming | `test/firewall/nat64-basic.ci` (proposed) |
| `nat64` config, VPP backend | → | nat64 plugin programming | `test/vpp/nat64-basic.ci` (proposed) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Kernel backend, prefix 64:ff9b::/96, pool 192.0.2.0/28 | An IPv6 client reaches an IPv4 server at 198.51.100.1 by 64:ff9b::c633:6401 through Jool; the server sees a pool source address | - |
| AC-2 | Second NAT64 source rule or instance in one network namespace | Refused at verify when the translator allows only one instance per namespace, naming both (VyOS `d5e67f48a`: its check looked for a node that does not exist, so a second Jool instance was created and which one translated was undefined) | - |
| AC-3 | Prefix length not one RFC 6052 allows | Refused at verify, on both backends | - |
| AC-4 | Removal on reload | Translator state withdrawn: no Jool instance, pool4 or pool6 left in the kernel, and no nat64 pool, prefix or interface flag left in VPP | - |
| AC-5 | A NAT64 shape one backend cannot apply exactly | Refused at verify by that backend with the YANG path and the fix | - |
| AC-6 | `ze doctor` on a kernel without the Jool module | Reports it with the command that fixes it | - |
| AC-7 | VPP backend, same config as AC-1 | The same client-to-server reachability through VPP's nat64 plugin; the server sees a pool source address | - |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNAT64Verify` | firewall component | AC-2, AC-3 | |
| `TestNAT64JoolMessages` | nft backend | Generic Netlink messages for the instance, pool4 and pool6, and their removal (AC-4) | |
| `TestNAT64VPPOps` | `internal/plugins/firewall/vpp/` | nat64 ops through the `vppOps` fake, and removal (AC-4) | |
| `TestNAT64VPPVerify` | `internal/plugins/firewall/vpp/verify_test.go` | AC-5 on VPP | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `nat64-basic` | `test/firewall/nat64-basic.ci` | AC-1 | |
| `nat64-basic` | `test/vpp/nat64-basic.ci` | AC-7 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `nat64-ipv4-server` | `test/interop/scenarios/` | an IPv4-only host and an IPv6-only client | AC-1 over real stacks on Ze's kernel | |

## Implementation Steps

1. **Phase: Wiring** -- YANG, verify, failing `.ci` for each backend
2. **Phase: Kernel translator** -- Jool in Ze's kernel, Generic Netlink programming, AC-1, AC-2, AC-4
3. **Phase: VPP translator** -- `binapi/nat64` vendored with the ops and verifier, AC-4, AC-5, AC-7
4. **Phase: Doctor** -- AC-6

## Files to Modify
- `internal/component/firewall/yang/ze-firewall-conf.yang`, `config.go`, `validate.go`
- `internal/plugins/firewall/nft/` (NAT64 apply through Jool)
- `internal/plugins/firewall/vpp/ops.go`, `backend_linux.go`, `nat_linux.go`, `verify.go`
- `gokrazy/kernel/kernel.config`, `gokrazy/kernel/runtime.config`, `gokrazy/kernel/patches/`
- `docs/guide/firewall.md`, `docs/architecture/appliance/kernel-profiles.md`

## Files to Create
- `vendor/go.fd.io/govpp/binapi/nat64/` (added by `go mod vendor` with its first user, D2)
- `test/firewall/nat64-basic.ci`
- `test/vpp/nat64-basic.ci`

## VyOS reference

T9536 (merge `d658b1297`, PR #5609): `d5e67f48a` "Fix check for only one source
rule". AC-2 carries it.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Targeted tests pass: `./le test firewall`, `./le test vpp`, `./le test integration firewall`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
