# Spec: firewall-fib-type-match

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | New firewall match VyOS offers and Ze lacks; nothing shipped is wrong without it, so it is post-release feature work. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An operator wants a firewall term to match on what the routing table says about
a packet's address: "destination is a local address of this box", "source
would route to a blackhole", "destination is unreachable or prohibited". This is
the nftables `fib` expression with the address-type result (`fib daddr type
local`, `fib saddr type != unicast`). It lets an operator write an input policy
that accepts only traffic to the router's own addresses without listing them,
and drop traffic whose source has no usable route, without depending on the
`rp_filter` sysctl.

Ze gains a `fib` container in the firewall `from` block: a lookup side
(`source-address` or `destination-address`) and a route type, optionally
negated.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/forwarding.md` - firewall backend split, exact-or-refuse at verify
  → Constraint: a node a backend cannot implement carries `ze:backend` and the commit refuses it
- [ ] `docs/architecture/core-design.md` sections 14a, 14b - commit-time capability gate, firewall backends
- [ ] `docs/guide/firewall.md` - the operator surface the new match joins

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/model.go` - the `Match` types. There is no FIB match. `MatchUnicast` is a link-layer packet-type match (`meta pkttype`), daemon-only, and is not a route lookup.
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` `grouping from-block` - no `fib` node.
- [ ] `internal/component/firewall/config.go` `parseFromBlock` - one branch per `from` leaf; no FIB branch.
- [ ] `internal/component/firewall/validate.go` `validateMatch` - per-match family checks; the FIB match needs its own hook and family rules.
- [ ] `internal/plugins/iface/netlink/rpf_linux.go` `rpfDropExprs` - already lowers an `expr.Fib` (saddr lookup, result OIF, `FlagPRESENT`, `FlagIIF` for strict mode) for interface reverse-path filtering. That is the only `expr.Fib` producer today.
- [ ] `internal/plugins/firewall/nft/lower_linux.go` `lowerMatch` - one case per match type; no FIB case.
- [ ] `internal/plugins/firewall/vpp/verify.go` `Verify` - lists the matches VPP accepts; a FIB type match is not among them.

**Behavior to preserve:** every existing match and its lowering; the iface RPF table.

**Behavior to change:** add the `fib` match.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| iface RPF lowering | `internal/plugins/iface/netlink/rpf_linux.go` `rpfDropExprs` | Builds an `expr.Fib` saddr/OIF lookup for strict and loose RPF | Reference only: same expression type, different result (`ResultADDRTYPE`). The firewall lowering is a separate producer in a separate plugin, with no import between them |
| Firewall match model and lowering | `internal/component/firewall/model.go` `Match`; `internal/plugins/firewall/nft/lower_linux.go` `lowerMatch` | Typed matches lowered to nft expressions | Extends: one new `Match` type and one lowering case |
| Backend capability gate | `ze:backend` in `ze-firewall-conf.yang`, `config.validateBackendFeatures` | Refuses at commit a node the selected backend cannot program | Extends: the `fib` node carries `ze:backend "nft"`, so a VPP commit is refused with the YANG path |
| `global-options source-validation` | `ze-firewall-conf.yang` `global-options` | Sets the kernel `rp_filter` sysctl | Does not apply: a sysctl, not a per-term match; both remain |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: a `fib` container under a term's `from` block, naming the lookup side and the route type (final leaf names decided at design).

### Transformation Path
1. YANG parse and `parseFromBlock` build a new FIB match value.
2. `validateMatch` refuses illegal combinations (hook, family, empty container).
3. nft lowering emits `expr.Fib` with the lookup flag and `ResultADDRTYPE`, then a compare against the kernel `RTN_*` value, `!=` for negation.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → firewall component | YANG tree, `ParseFirewallConfig` | No |
| Firewall component → nft backend | `Backend.Apply` with the typed model | No |

### Integration Points
- `lowerMatch` in `internal/plugins/firewall/nft/lower_linux.go` - one new case
- `validateMatch` in `internal/component/firewall/validate.go` - one new case

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `from fib` in config | → | nft lowering of the FIB match | `test/firewall/fib-type-local.ci` (proposed) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Input-hook term with `fib` lookup destination-address, type local, then accept | The rule reads back with a FIB expression (daddr flag, address-type result) compared to the local route type; a packet to a local address is accepted and counted, a forwarded packet is not | Other terms keep their behavior |
| AC-2 | Each of `local`, `unicast`, `broadcast`, `multicast`, `anycast`, `blackhole`, `unreachable`, `prohibit` | Each lowers to its own kernel route-type value, proven per type by a unit test that reads the compare data | None: an unknown type is refused by the YANG enumeration |
| AC-3 | Negated type (final spelling at design) | The compare is `!=`; a packet to a non-local address matches | The non-negated form stays available |
| AC-4 | Lookup source-address | The FIB flag is saddr, not daddr; a source covered by a blackhole route matches type blackhole in a functional test that installs that route | - |
| AC-5 | Both source-address and destination-address requested in one term | Ze emits two separate FIB lookups ANDed in one rule, never one concatenated `saddr . daddr` lookup (the form VyOS renders). If the kernel refuses saddr and daddr in one expression (A-1), the two-expression form is what avoids it | - |
| AC-6 | `fib` container present with no lookup or no type | Commit refused naming the missing leaf; VyOS added `fib` and `fib match` to its empty-node check for the same reason | - |
| AC-7 | `fib` in a chain whose hook the kernel's FIB validation refuses, or in a `bridge`, `arp` or `netdev` table | Refused at verify with the hook or family named, never at apply | - |
| AC-8 | `firewall backend vpp` with a `fib` term | Commit refused by `ze:backend "nft"` with the YANG path | - |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The kernel refuses `NFTA_FIB_F_SADDR` and `NFTA_FIB_F_DADDR` together | Recollection of `net/netfilter/nft_fib.c` `nft_fib_init`; not yet read | AC-5 still holds; only its reason changes | Read `nft_fib_init` and `nft_fib_validate` at the Ze kernel version | unvalidated |
| A-2 | Address-type results are allowed on prerouting, input, forward and output | Same file, `nft_fib_validate`; not yet read | AC-7's hook list changes | Same read, plus a refusal test per hook | unvalidated |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLowerFIBTypeMatch` | `internal/plugins/firewall/nft/lower_linux_test.go` | AC-2, AC-3, AC-5 expression shape per type and side | |
| `TestValidateFIBMatch` | `internal/component/firewall/validate_test.go` | AC-6, AC-7 refusals | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fib-type-local` | `test/firewall/fib-type-local.ci` | AC-1, AC-4: traffic to a local address matches, a blackholed source matches | |

## Implementation Steps

1. **Phase: Wiring** -- YANG node, parse branch, failing `.ci`
2. **Phase: Lowering and verify** -- AC-2 to AC-8

## Files to Modify
- `internal/component/firewall/model.go` - FIB match type
- `internal/component/firewall/config.go` - parse
- `internal/component/firewall/validate.go` - verify rules of AC-6 and AC-7
- `internal/component/firewall/yang/ze-firewall-conf.yang` - `fib` container in `from-block`, `ze:backend "nft"`
- `internal/plugins/firewall/nft/lower_linux.go` - lowering
- `docs/guide/firewall.md` - the new match

## Files to Create
- `test/firewall/fib-type-local.ci` - end-to-end match

## VyOS reference

VyOS added this in PR #5372, task T9157 (merge `8fec254b2`, 2026-09-11):
`interface-definitions/include/firewall/fib.xml.i`, renderer in
`python/vyos/firewall.py`. Its lookup accepts both addresses and joins them as
`saddr . daddr` in one expression (AC-5 avoids that), and its types are the eight
in AC-2 with a `!` prefix for negation.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] Assumptions A-1 and A-2 confirmed against the kernel source
- [ ] Targeted tests pass: `./le test unit plugins`, `./le test integration firewall`, `./le test firewall`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
