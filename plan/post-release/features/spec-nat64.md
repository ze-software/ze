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
translation prefix, the IPv4 pool, and the interfaces it applies to.

## Open Owner Questions

| Question | Evidence | Options |
|----------|----------|---------|
| Q1: which layer translates | Linux nftables has no NAT64 expression. Jool is an out-of-tree kernel module (would ship in Ze's kernel, `plan/post-release/kernel/`). VPP has a nat64 plugin, but the vendored govpp tree carries only `nat44_ed` and `nat_types`; using another package of an approved module needs Thomas's approval for that use (`ai/META-DECISIONS.md`, "Code and dependencies"). A userspace TUN translator is a third route | (a) Jool in Ze's kernel for the nft backend, VPP nat64 for the VPP backend; (b) VPP only, refused on nft; (c) a Go TUN translator |
| Q2: RFC enrolment | RFC 6146 (stateful NAT64), RFC 6052 (address format) and RFC 7915 (SIIT) are not in `rfc/full/` and no profile lists them; RFC validation scope does not grow without a profile listing them (`ai/META-DECISIONS.md`) | Thomas lists them on a profile, or the feature ships with no conformance figure |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/forwarding.md` - backend split, exact-or-refuse, nft table ownership
- [ ] `docs/architecture/firewall/fw-6-firewall-vpp.md` - the VPP NAT backend
- [ ] `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - what Ze owns in the kernel

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/model.go` `SNAT`, `DNAT`, `Masquerade` - same-family NAT only; no cross-family translation.
- [ ] `internal/component/firewall/config.go` `parseNATSpec` - one address family per target.
- [ ] `internal/plugins/firewall/vpp/backend_linux.go` - imports `nat44_ed` and `nat_types` only.
- [ ] `internal/plugins/firewall/vpp/nat_linux.go` `applyNATChains` - NAT44-ED static mappings and address ranges.
- [ ] No NAT64, Jool or Tayga code exists under `internal/` (grep).

**Behavior to preserve:** NAT44 and IPv6 NAT.

**Behavior to change:** add NAT64.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Firewall backend registry | `internal/component/firewall/backend.go` `Backend`, `RegisterBackend` | nft and VPP backends program NAT44/NAT66 | Extends: NAT64 config is verified per backend; a backend that cannot translate refuses at verify |
| VPP NAT44-ED integration | `internal/plugins/firewall/vpp/nat_linux.go`, `ops.go` `vppOps` | Address ranges and static mappings behind a test seam | Extends under Q1 (a) or (b): new ops for the nat64 plugin behind the same seam |
| Ze's kernel | `plan/post-release/kernel/` | Ze builds and boots its own kernel | Extends under Q1 (a): Jool module carried in Ze's kernel |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `nat64` block with prefix, pool and interfaces (final placement at design).

### Transformation Path
1. Parse and verify prefix length (RFC 6052 lengths), pool and interfaces.
2. Program the selected translator; withdraw it on removal.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → firewall component | YANG tree | No |
| Firewall → translator | kernel module netlink, or VPP binary API | No |

### Integration Points
- Firewall backend verify and apply

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `nat64` config | → | translator programming | `test/firewall/nat64-basic.ci` (proposed) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Prefix 64:ff9b::/96, pool 192.0.2.0/28 | An IPv6 client reaches an IPv4 server at 198.51.100.1 by 64:ff9b::c633:6401; the server sees a pool source address | - |
| AC-2 | Second NAT64 source rule or instance in one network namespace | Refused at verify when the translator allows only one instance per namespace, naming both (VyOS `d5e67f48a`: its check looked for a node that does not exist, so a second Jool instance was created and which one translated was undefined) | - |
| AC-3 | Prefix length not one RFC 6052 allows | Refused at verify | - |
| AC-4 | Removal on reload | Translator state withdrawn, nothing left in the kernel or VPP | - |
| AC-5 | Backend that cannot translate | Refused at verify with the fix | - |
| AC-6 | `ze doctor` with a missing translator module | Reports it with the command that fixes it | - |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNAT64Verify` | firewall component | AC-2, AC-3, AC-5 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `nat64-basic` | `test/firewall/nat64-basic.ci` | AC-1 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `nat64-ipv4-server` | `test/interop/scenarios/` | an IPv4-only host and an IPv6-only client | AC-1 over real stacks | |

## Implementation Steps

1. **Phase: Q1 and Q2 answered**
2. **Phase: Wiring** -- YANG, verify, failing `.ci`
3. **Phase: Translator** -- AC-1, AC-4
4. **Phase: Doctor** -- AC-6

## Files to Modify
- `internal/component/firewall/yang/ze-firewall-conf.yang`, `config.go`, `validate.go`
- `internal/plugins/firewall/vpp/ops.go` (under Q1 a or b)
- `docs/guide/firewall.md`

## Files to Create
- `test/firewall/nat64-basic.ci`

## VyOS reference

T9536 (merge `d658b1297`, PR #5609): `d5e67f48a` "Fix check for only one source
rule". AC-2 carries it.

## Checklist

### Goal Gates (MUST pass)
- [ ] Q1 and Q2 answered
- [ ] AC-1..AC-6 all demonstrated
- [ ] Targeted tests pass: `./le test firewall`, `./le test integration firewall`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
