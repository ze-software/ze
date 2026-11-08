# Spec: iface-vxlan-gbp-gpe

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Adds two VXLAN extensions absent from the supported release: Group Based Policy (GBP) and Generic Protocol Extension (GPE). Plain VXLAN stays shipped and required |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

VXLAN-GBP carries a 16-bit group policy id in the VXLAN header, so a fabric
can enforce segmentation by group (the Linux kernel maps it to and from the
packet mark). VXLAN-GPE carries a next-protocol field, so the tunnel can carry
IPv4, IPv6 or NSH without an inner Ethernet header. VyOS added both in recent
work (T9319 GBP, T9318 GPE validation). An operator must be able to enable
either on a Ze VXLAN tunnel, with the conflicts VyOS found refused at verify.

Neither extension is an RFC: GBP is `draft-smith-vxlan-group-policy`, GPE is
`draft-ietf-nvo3-vxlan-gpe`. No `rfc/short/` summary is written for them
(`ai/META-DECISIONS.md`: RFC validation scope does not grow).

| # | Operator need |
|---|---------------|
| 1 | `gbp` on a VXLAN tunnel; the packet mark carries the group id on receive and sets it on send |
| 2 | `gpe` on a VXLAN tunnel, in the flow-based (external) mode the kernel requires for it (A-1) |
| 3 | Turning either on or off re-creates the link, because the kernel cannot change them on a live device |
| 4 | Conflicts refused: GBP with GPE; two tunnels on one UDP port and address family that disagree on GBP |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` section 14a - `ze:backend`
  → Constraint: the VPP backend implements plain VXLAN through VxlanAddDelTunnelV3; a flag it cannot honor is refused at commit

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `container vxlan` (`ze:backend "netlink vpp"`): local and remote ip, `vni` 1..16777215, `port` default 4789; no gbp, gpe or external leaf; a unit with a vlan-id on it is refused
- [ ] `internal/component/iface/tunnel.go` - `TunnelKindVxlan` "landed in both backends (netlink Vxlan link + VPP VxlanAddDelTunnelV3)"
- [ ] `internal/plugins/iface/netlink/tunnel_linux.go` - `buildVxlan` sets `VxlanId`, `SrcAddr`, `Group`, `Port`; refuses a local-interface source; validates the VNI
- [ ] `internal/component/iface/backend.go` - `CreateTunnel(TunnelSpec)`

**Behavior to preserve:**
- Every plain VXLAN tunnel on both backends.

**Behavior to change:**
- `gbp` and `gpe` (with the flow-based mode GPE needs) on the netlink backend; refused on VPP unless A-3 finds an exact mechanism.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| VXLAN tunnel kind | `tunnel.go` `TunnelKindVxlan`, `tunnel_linux.go` `buildVxlan`, YANG `container vxlan` | Builds a unicast VXLAN link | Extends with two flags; no second VXLAN kind |
| Tunnel re-create on change | the iface apply's tunnel path | Re-creates a tunnel whose immutable parameters changed | Extends: GBP and GPE are immutable on a live link (VyOS re-creates too) |
| Backend capability gate | `ze:backend`, `validateBackendGate` | Refuses unsupported nodes at commit | Extends: the new leaves carry `ze:backend "netlink"` unless A-3 holds |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `gbp` and `gpe` leaves under the vxlan case (VyOS spells `gbp` as a valueless node and `gpe` under `parameters`).

### Transformation Path
1. Verify: GBP and GPE together refused; GPE without the flow-based mode refused or implied (A-1); GBP conflicts on a shared port and family refused.
2. Apply: `buildVxlan` sets the flags; a change re-creates the link.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface to kernel | rtnetlink IFLA_VXLAN_GBP, IFLA_VXLAN_GPE, IFLA_VXLAN_COLLECT_METADATA | No |

### Changed-Wire Census

**Search evidence:** owed at design (the VXLAN header bits on the wire change when enabled; no Ze JSON wire changes).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [finding] | [disposition] |

### Integration Points
- `buildVxlan` and the tunnel spec parse.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The Linux kernel accepts VXLAN-GPE only in flow-based (collect-metadata, "external") mode, which Ze does not model today | VyOS T9318 `ec84c2e48` validates GPE against `external` | GPE could be a plain flag | Read `drivers/net/vxlan/vxlan_core.c` `vxlan_config_validate` and quote it | unvalidated |
| A-2 | The vendored `vishvananda/netlink` `Vxlan` has a `GBP` field and no `GPE` field | `vendor/github.com/vishvananda/netlink/link.go` | GPE needs Ze's own attribute or a module change kept as a diff against upstream (a change Ze needs in a third-party Go module is kept as a diff applied at build time, never a fork) | Read the vendored type. Using any feature of `vishvananda/netlink` needs no approval: the per-use approval applies only to experimental crypto modules such as Cloudflare's `circl` | unvalidated |
| A-3 | VPP's VXLAN plugin has no GBP flag, and GPE lives in a separate `vxlan-gpe` plugin with its own API | VPP source | VPP could support one of them | Read VPP source, quote the functions. The govpp `vxlan_gpe` binapi package needs no approval: every module already in `go.mod` may be used in full, except experimental crypto modules such as Cloudflare's `circl`, and `go.fd.io/govpp` is already required; `go mod vendor` adds the package with the code that first imports it | unvalidated |
| A-4 | Flow-based mode is useful only with routes that carry tunnel metadata (lwtunnel encap), so GPE without them carries nothing | Linux lwtunnel design | GPE scope is larger than one flag | Design phase decides whether this spec also owns the encap route, or names a separate spec | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A rejected commit leaves the running tunnel changed | Tunnel recreated before refusal | Verify refuses before any apply; test asserts the running tunnel is untouched (VyOS T9318 and T9319 both test this) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The overlay on that tunnel |
| How is it reverted? | Remove the leaf; the link is re-created plain |
| Who else touches this path? | Firewall marks (GBP maps to the packet mark), the tunnel apply |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| VXLAN tunnel with `gbp` committed | → | `buildVxlan` sets IFLA_VXLAN_GBP | `test/plugin/iface-vxlan-gbp-apply.ci` (`option=needs-linux`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | VXLAN with `gbp` | The kernel link reports GBP; a packet sent with mark 0x64 leaves with group policy id 100 in the header (capture on the underlay) | None |
| AC-2 | `gbp` removed, then re-enabled | The link is re-created each time and its other settings (addresses, MTU, units) are kept | None |
| AC-3 | `gbp` and `gpe` on one tunnel | Verify refuses naming both | None |
| AC-4 | Two VXLAN tunnels on the same UDP port and address family, one with `gbp` and one without | Verify refuses naming both tunnels and the port; the running tunnels are unchanged | Different address families, or different underlay VRFs, may differ (VyOS T9319) |
| AC-5 | `gpe` with the flow-based mode | The kernel link reports GPE and external | None |
| AC-6 | `gpe` without the mode the kernel requires | Verify refuses naming the leaf and the fix, and the running tunnel is unchanged (VyOS T9318) | None |
| AC-7 | `backend vpp` with `gbp` or `gpe` | Commit refused with the YANG path, unless A-3 finds an exact mechanism | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables GBP on an overlay | config, verify, tunnel re-create | `test/plugin/iface-vxlan-gbp-apply.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVxlanGBPGPEConflicts` | `internal/component/iface/config_vxlan_test.go` | AC-3, AC-4, AC-6 | |

### Boundary Tests (numeric inputs)
N-A: the new leaves take no number.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-vxlan-gbp-apply` | `test/plugin/` | AC-1, AC-2 | |
| `iface-vxlan-gpe-refused` | `test/plugin/` | AC-6 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `vxlan-gbp-linux-peer` | `test/interop/scenarios/` | a plain Linux VXLAN-GBP endpoint | The group id survives both directions | |

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - the leaves
- `internal/component/iface/tunnel.go` - `TunnelSpec` flags
- `internal/plugins/iface/netlink/tunnel_linux.go` - `buildVxlan`

## Files to Create
- `test/plugin/iface-vxlan-gbp-apply.ci`, `test/plugin/iface-vxlan-gpe-refused.ci`

## Implementation Steps

1. **Phase: Wiring** - leaves, parse, failing `.ci`.
2. **Phase: GBP** - flag, re-create, conflicts.
3. **Phase: GPE** - per A-1 and A-4.
4. **Phase: VPP** - refusal or mechanism.

Proof: `./le test unit plugins`, `./le test functional plugin` for the named `.ci` files, `./le test qemu all`.

## VyOS reference

VyOS checkout `~/Code/github.com/vyos/vyos-1x`, read 2026-11-08 at `611550d23`.

| Ticket / commit | Lesson for Ze |
|-----------------|---------------|
| T9319, `4fd9ad0ac` "vxlan: add VXLAN-GBP support" | Recreate the link when GBP changes; reject GBP with GPE; reject conflicting GBP on a shared UDP port, by address family and underlay VRF; external mode carries GBP in tunnel metadata, not the mark. AC-2, AC-3, AC-4 |
| T9318, `ec84c2e48` "vxlan: fix GPE external-mode validation" | Validation looked up `external` in the wrong place, so valid GPE configs were refused; tests assert the running tunnel survives a rejected commit. AC-5, AC-6 |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- VXLAN with a local-interface source and multicast groups are unchanged by this spec.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)
