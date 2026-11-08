# Spec: nat-prefix-translation

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | New NAT capability VyOS offers (NAT44 and NAT66 prefix translation) and Ze lacks; nothing shipped is wrong without it, so it is post-release feature work. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An operator wants to map one prefix onto another of the same length, host bits
kept (1:1 prefix NAT, the kernel's netmap): 10.1.0.0/24 to 192.0.2.0/24, or an
IPv6 site prefix onto a provider prefix (VyOS calls the IPv6 form NAT66). Today
Ze translates to one address or a contiguous range, which picks addresses from a
pool and does not keep host bits. Ze gains a prefix target for `snat` and `dnat`,
for IPv4 and IPv6, with an optional port range.

Stateful IPv6 source NAT to a single address already works (`parseNATSpec`
accepts a bracketed IPv6 address and `lowerNAT` sets AF_INET6), so "NAT66" here
means the prefix form plus IPv6 address ranges, which are refused today.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/forwarding.md` - backend split, exact-or-refuse
- [ ] `docs/guide/firewall.md` - NAT syntax

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/model.go` `SNAT`, `DNAT` - address, address end, port, port end, flags; no prefix.
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` `snat`, `dnat` - `to` leaf of type `nat-spec`.
- [ ] `internal/component/firewall/config.go` `parseNATSpec` - accepts an address, an IPv4 range, a port or port range; refuses IPv6 ranges and anything with a slash.
- [ ] `internal/plugins/firewall/nft/lower_linux.go` `lowerNAT` - refuses any non-zero NAT flag, so the netmap flag cannot reach the kernel today.
- [ ] `internal/plugins/firewall/vpp/verify.go` `verifyNATTerm` - checks address family of single targets.

**Behavior to preserve:** every existing NAT form.

**Behavior to change:** add the prefix target and IPv6 ranges.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| NAT target parse | `config.go` `parseNATSpec` | Address, IPv4 range, port range | Extends: a prefix form and IPv6 ranges |
| NAT lowering | `lower_linux.go` `lowerNAT` | Registers 1 to 4, no flags | Extends: the prefix becomes the min and max address registers with the kernel's netmap range flag; the flag is set by the lowering, not passed as an operator flag |
| VPP NAT44-ED | `vpp/verify.go` `verifyNATChain` | Single static mappings | Refuses a prefix target unless VPP maps it exactly |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `snat to <prefix>` or `dnat to <prefix>`, optionally with a port range (final syntax at design).

### Transformation Path
1. Parse the prefix, refuse host bits set.
2. Verify the match side: the matched source (for snat) or destination (for dnat) prefix has the same length as the target.
3. Lower to the NAT expression with netmap semantics, keeping any port range.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → firewall component | YANG tree | No |
| Firewall component → nft backend | `Backend.Apply` | No |

### Integration Points
- `parseNATSpec`, `lowerNAT`, `validateAction`

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `snat to <prefix>` | → | netmap lowering | `test/firewall/nat-prefix.ci` (proposed) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `snat to 192.0.2.0/24` for traffic from 10.1.0.0/24 | 10.1.0.37 leaves as 192.0.2.37, proven by a traffic test reading the translated source | - |
| AC-2 | Same for IPv6 (a /48 to a /48) | Host bits kept, IPv6 conntrack entry shows the mapped address | - |
| AC-3 | Prefix target with a port range | The port range is kept in the programmed rule and observed on the wire; if the kernel cannot combine netmap with a port range, verify refuses the term naming both, never a silent drop of the port range (VyOS `385cafdcb`: the port range was lost with no warning when the rule had no address match) | - |
| AC-4 | Prefix target in a term with no source or destination address match | Same mapping as AC-1, keyed on the whole address space of the family, port range kept (the case VyOS got wrong) | - |
| AC-5 | Target prefix with host bits set, or a family different from the table or match | Refused at verify naming the value | - |
| AC-6 | IPv6 address range target (not a prefix) | Accepted and lowered as the IPv4 range form is, both ends one family | - |
| AC-7 | VPP backend | Exact mapping or refusal at verify | - |
| AC-8 | Documentation | The guide calls this prefix NAT; it never claims RFC 6296 NPTv6, which is stateless and checksum-neutral while netmap is stateful | - |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseNATPrefix` | `internal/component/firewall/config_test.go` | AC-5, AC-6 | |
| `TestLowerNATPrefix` | `internal/plugins/firewall/nft/lower_linux_test.go` | AC-1 to AC-4 registers and flag | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `nat-prefix` | `test/firewall/nat-prefix.ci` | AC-1, AC-3 | |

## Implementation Steps

1. **Phase: Wiring** -- parse, failing `.ci`
2. **Phase: Lowering** -- AC-1 to AC-6
3. **Phase: VPP and docs** -- AC-7, AC-8

## Files to Modify
- `internal/component/firewall/model.go`, `config.go`, `validate.go`
- `internal/component/firewall/yang/ze-firewall-conf.yang`
- `internal/plugins/firewall/nft/lower_linux.go`
- `internal/plugins/firewall/vpp/verify.go`
- `docs/guide/firewall.md`

## Files to Create
- `test/firewall/nat-prefix.ci`

## Known Limitations
- Stateless NPTv6 (RFC 6296) is a different mechanism and is not delivered here. No release profile lists RFC 6296 and no summary is enrolled, so it is not proposed as a spec (RFC scope does not grow, `ai/META-DECISIONS.md`).

## VyOS reference

T9475, `385cafdcb` "keep translation port when translating to a prefix": nft has
no syntax for a prefix plus a port range, so VyOS dropped the port range without
a warning; its fix uses a map keyed on 0.0.0.0/0. VyOS NAT66 arrived in PR #5236
(`e238afbd6`) and its table rebuild fix is `c01cf1021`.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Targeted tests pass: `./le test unit plugins`, `./le test integration firewall`, `./le test firewall`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
