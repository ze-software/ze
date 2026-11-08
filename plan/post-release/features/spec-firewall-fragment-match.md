# Spec: firewall-fragment-match

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

An operator wants to match IP fragments (or only non-fragments): drop fragments
arriving on a WAN interface, rate-limit them, or count them. With connection
tracking loaded, the kernel reassembles fragments in prerouting before any chain
at normal priority runs, so a plain header read in a filter chain never sees a
fragment. Ze gains a `fragment` leaf in the firewall `from` block
(`fragment`, `non-fragment`, final spelling at design) whose result is right
whatever chain the term sits in, and which never disturbs packet marks.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/forwarding.md` - backend split, exact-or-refuse
- [ ] `docs/architecture/policyroute/policy-routing.md` - policy routing owns marks 0x50000 to 0x5FFFF and ip rules on them
  → Constraint: any mark bit this feature reserves must be masked by every Ze mark writer and every Ze ip rule
- [ ] `docs/guide/firewall.md` - `mark`, `mark-set`, `connection-mark-set` semantics

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/model.go` - no fragment match; `MatchMark` and `SetMark` carry a value and a mask.
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` `grouping from-block` - no fragment leaf; `mark` defaults to mask 0xffffffff.
- [ ] `internal/component/firewall/config.go` `parseFromBlock` - no fragment branch.
- [ ] `internal/plugins/firewall/nft/lower_linux.go` `lowerMatch` - no fragment case. The IPv6 hop-limit lowering notes that offset 7 is the IPv4 fragment-offset low byte, the only fragment reference.
- [ ] `internal/plugins/policyroute/marks.go` `allocator.allocateMark` - allocates packet marks from 0x50000 to 0x5FFFF.
- [ ] `internal/plugins/policyroute/rules_linux.go` `applyIPRules` - installs one fwmark ip rule per mark through `iprule.Mark.Rule`.
- [ ] `internal/plugins/policyroute/translate.go` `translate` - marks at prerouting priority -150.

**Behavior to preserve:** every existing mark writer and mark match; policy-routing table selection.

**Behavior to change:** add the fragment match.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Firewall match model and lowering | `model.go` `Match`; `lower_linux.go` `lowerMatch` | Typed matches lowered to nft expressions | Extends: one fragment match |
| Chain priority | `ze-firewall-conf.yang` `chain priority`, `model.go` `ChainPriorityMin` | Operator sets the chain priority | Extends: a term in a chain at or before conntrack defrag priority reads the header directly (IPv4 fragment offset and MF flag, IPv6 fragment extension header) with no mark |
| Packet marks | `SetMark`, `MatchMark`, `policyroute` `allocateMark`, `iprule.Mark` | Firewall and policy routing write and match marks | Extends only if design picks the mark route: one reserved bit, masked by every writer, matcher and ip rule |
| Backend capability gate | `ze:backend` | Refuses a node a backend cannot program | Extends: `ze:backend "nft"` unless VPP can match fragments exactly |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `fragment` leaf in a firewall term's `from` block.

### Transformation Path
1. Parse and verify the leaf against the chain's family and priority.
2. Lower to a header read when the chain runs before defrag; otherwise to the design's carrier for "this packet was a fragment" (a reserved mark bit set in a pre-defrag chain Ze owns, or a refusal).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → firewall component | YANG tree | No |
| Firewall component → nft backend | `Backend.Apply` | No |

### Integration Points
- `lowerMatch`, `validateMatch`; the mark writers and `iprule.Mark` if a mark bit is reserved

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `from fragment` in config | → | fragment match lowering | `test/firewall/fragment-match.ci` (proposed) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | IPv4 and IPv6 fragmented traffic through a term matching fragments, conntrack loaded | Every fragment of the flow is counted, in an integration test that sends real fragments | Non-fragments are not counted |
| AC-2 | Same with non-fragment | Only unfragmented packets match | - |
| AC-3 | A fragment that already carries a packet mark, then a fragment match | The existing mark is unchanged after the match: the feature never overwrites the whole mark (VyOS `dbde15e2d`: fragments were tagged by replacing the mark, losing marks already set) | - |
| AC-4 | Fragment match plus a later `mark-set` or a policy-routing table rule | The fragment match still matches and the policy route still selects its table: no mark writer clears the fragment carrier, and no ip rule or `mark` match sees the carrier bit (VyOS `dbde15e2d`: a later `set mark` or `set table` broke fragment matching) | - |
| AC-5 | If a mark bit is reserved | It is cleared before consumers outside the firewall (traffic control, IPVS) read the mark, and `mark`, `mark-set`, `connection-mark-set` and `iprule.Mark` all mask it; config refuses a mark value or mask that uses the bit, naming it | - |
| AC-6 | Fragment match in an `arp`, `bridge` or `netdev` table | Refused at verify with the family named | - |
| AC-7 | VPP backend | Refused at commit unless VPP programs it exactly | - |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Conntrack defrag runs at prerouting priority -400, so chains after it see reassembled packets | Kernel `nf_defrag_ipv4`, not yet read | AC-1 design changes | Read `net/ipv4/netfilter/nf_defrag_ipv4.c` and the IPv6 one | unvalidated |
| A-2 | Bit 31 of the mark is free in Ze | Policy routing uses 0x50000 to 0x5FFFF; operator marks are 32-bit and unrestricted | AC-5 must reserve another bit or refuse operator values | Grep every mark producer at design | unvalidated |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLowerFragmentMatch` | `internal/plugins/firewall/nft/lower_linux_test.go` | expression shape per family | |
| `TestFragmentMarkBitMasked` | `internal/component/firewall/` and `internal/plugins/policyroute/` | AC-4, AC-5 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fragment-match` | `test/firewall/fragment-match.ci` | AC-1, AC-3, AC-4 | |

## Implementation Steps

1. **Phase: Wiring** -- YANG, parse, failing `.ci`
2. **Phase: Lowering** -- AC-1, AC-2, AC-6
3. **Phase: Mark safety** -- AC-3 to AC-5

## Files to Modify
- `internal/component/firewall/model.go`, `config.go`, `validate.go`
- `internal/component/firewall/yang/ze-firewall-conf.yang`
- `internal/plugins/firewall/nft/lower_linux.go`
- `internal/plugins/policyroute/marks.go`, `internal/plugins/policyroute/rules_linux.go` (only on the mark route)
- `docs/guide/firewall.md`

## Files to Create
- `test/firewall/fragment-match.ci`

## VyOS reference

T9474, `dbde15e2d` "keep fragment matching working alongside packet marks": VyOS
tagged fragments by overwriting the whole packet mark before reassembly, so a
later `set mark` or policy route `set table` erased the tag and marks already on
fragments were lost. Its fix uses bit 31 only, masks it in every mark match,
writer and ip rule, and clears it after the forward and input filters. AC-3 to
AC-5 carry that lesson.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Targeted tests pass: `./le test unit plugins`, `./le test integration firewall`, `./le test firewall`, `./le test policy`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
