# Spec: nat-load-balance

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | New NAT capability VyOS offers and Ze lacks; nothing shipped is wrong without it, so it is post-release feature work. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An operator wants one public address and port spread over several back-end
servers (destination NAT load balancing), or outbound traffic spread over
several source addresses, with a weight per back end. nftables does it with a
map keyed by `numgen random` or by a `jhash` of chosen header fields. Conntrack
keeps every later packet of a flow on the back end its first packet chose.

Ze gains a `load-balance` form of `dnat` and `snat`: a list of back ends, each
an address with an optional port and a weight, and a choice of `random` or a
hash over named fields (source address, destination address, source port,
destination port) with an optional seed.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/forwarding.md` - backend split, exact-or-refuse
- [ ] `docs/guide/firewall.md` - `snat`, `dnat` syntax
- [ ] `docs/architecture/firewall/fw-6-firewall-vpp.md` - what the VPP NAT backend programs

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/model.go` `SNAT`, `DNAT` - one address or an address range, one port or a port range, and `Flags`.
- [ ] `internal/component/firewall/config.go` `parseNATSpec` - parses one target; a comma list of ports is refused; IPv6 ranges are refused.
- [ ] `internal/plugins/firewall/nft/lower_linux.go` `lowerNAT` - immediates into registers 1 to 4 and one `expr.NAT`; any non-zero flag is refused. No `expr.Numgen`, `expr.Hash` or map lookup is lowered anywhere in the firewall backend, though the vendored nftables library carries both expressions.
- [ ] `internal/plugins/firewall/vpp/verify.go` `verifyNATTerm` - VPP accepts SNAT, DNAT and masquerade with single targets.

**Behavior to preserve:** single-target `snat`, `dnat`, `masquerade` and their verify rules.

**Behavior to change:** add the load-balanced target list.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| NAT actions | `model.go` `SNAT`, `DNAT`; `lower_linux.go` `lowerNAT` | Single target or range into NAT registers | Extends: the NAT expression reads its address and port registers from a map lookup instead of immediates |
| Anonymous sets and maps | `lower_linux.go` `lowerPortMatch` (anonymous interval sets), `lowerSet` | Builds anonymous and named sets | Extends: an anonymous verdict-free data map from hash bucket to address and port |
| VPP NAT44-ED | `internal/plugins/firewall/vpp/verify.go` `verifyNATChain`; vendored `nat44_ed` binapi carries an LB static mapping call | Single static mappings | Design choice: program VPP's LB static mapping if it matches the weight semantics exactly, otherwise refuse at verify |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `dnat load-balance` (or `snat load-balance`) with back ends, weights and the distribution method.

### Transformation Path
1. Parse back ends; compute the modulus as the sum of the weights.
2. Build an anonymous map whose key ranges partition 0 to sum-1 by weight, each value an address concatenated with its port.
3. Lower: numgen random mod sum (or jhash of the named fields mod sum, with seed), map lookup into the NAT registers, then the NAT expression with its flags after it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → firewall component | YANG tree | No |
| Firewall component → nft backend | `Backend.Apply` | No |

### Integration Points
- `lowerNAT` and the action parse in `parseThenBlock`

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `dnat load-balance` in config | → | map-based NAT lowering | `test/firewall/nat-load-balance.ci` (proposed) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Three back ends, weights 1, 1, 2, random | Modulus is 4 (the sum), the map covers 0..3 with no gap, and a traffic test sees roughly 25/25/50 | - |
| AC-2 | Weights summing to less than or more than 100 | Weights are shares of their sum: every hash value maps to a back end, no packet falls through the map (VyOS `938140bfe`: a fixed modulus of 100 left buckets with no back end, and commit only warned) | - |
| AC-3 | Back ends with a translation port, or a port range | The port is part of every map element (address and port), and any NAT flags follow the map; the commit applies and port-forwarding works end to end (VyOS `a0ea6ec90`: the port placed before the hash expression made nft reject the rule) | - |
| AC-4 | Hash over source address only | The same client always reaches the same back end across connections; the lowered hash names exactly the configured fields and seed | - |
| AC-5 | A weight of 0, an empty back-end list, mixed address families, or a sum that overflows 32 bits | Refused at verify, naming the back end | - |
| AC-6 | Back end set changed on reload | Existing flows keep their translation (conntrack), new flows use the new map; the reload replaces the rule atomically | - |
| AC-7 | VPP backend | Programs an exact equivalent, or is refused at verify naming the term | - |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLowerNATLoadBalanceMap` | `internal/plugins/firewall/nft/lower_linux_test.go` | AC-1 to AC-4 map and modulus | |
| `TestParseNATLoadBalance` | `internal/component/firewall/config_test.go` | AC-5 refusals | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| weight | 1 to design maximum | 1 | 0 | sum overflow |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `nat-load-balance` | `test/firewall/nat-load-balance.ci` | AC-1, AC-3 | |

## Implementation Steps

1. **Phase: Wiring** -- YANG, parse, failing `.ci`
2. **Phase: nft lowering** -- AC-1 to AC-6
3. **Phase: VPP** -- AC-7

## Files to Modify
- `internal/component/firewall/model.go`, `config.go`, `validate.go`
- `internal/component/firewall/yang/ze-firewall-conf.yang`
- `internal/plugins/firewall/nft/lower_linux.go`
- `internal/plugins/firewall/vpp/verify.go`
- `docs/guide/firewall.md`

## Files to Create
- `test/firewall/nat-load-balance.ci`

## VyOS reference

T9475 (merge `cd6a243f6`, PR #5574): `938140bfe` "load-balance weights are
relative to their sum" and `a0ea6ec90` "load-balance rules with a translation
port failed to commit". AC-2 and AC-3 carry both.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Targeted tests pass: `./le test unit plugins`, `./le test integration firewall`, `./le test firewall`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
