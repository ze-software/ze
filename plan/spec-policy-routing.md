# Spec: policy-routing -- Policy-based routing (Surfprotect)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-fw-8-lns-gaps (interface wildcard, component reactor pattern), spec-static-routes (table 100 populated) |
| Phase | - |
| Updated | 2026-04-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/firewall/model.go` -- match/action types (reused here)
4. `internal/component/firewall/config.go` -- from-block parsing pattern (reused here)
5. `internal/plugins/firewall/nft/lower_linux.go` -- nftables expression lowering (extended here)
6. `vendor/github.com/vishvananda/netlink/rule.go` -- ip rule management API
7. `ze/lns.conf` lines 179-201 -- VyOS Surfprotect policy routing config

## Task

Add policy-based routing to ze, enabling the Surfprotect content filtering setup used on the
Exa LNS. Policy routing directs subscriber HTTP/HTTPS traffic through a GRE tunnel to a
content filter, while bypassing certain destinations and sources.

The VyOS LNS config implements this with two mechanisms working together:

1. **nftables rules** in a `route` type chain (prerouting hook) that match traffic on
   l2tp* interfaces and set packet marks for matching flows
2. **ip rules** that map packet marks to routing table 100 (which has a default route via
   tun100 to the Surfprotect GRE tunnel)

Ze needs both: nftables policy chains for traffic classification and ip rule management
for mark-to-table mapping.

### VyOS config being replaced

```
set policy route surfprotect interface 'l2tp*'

# Rule 1: bypass DstBypass addresses on ports 80,443
set policy route surfprotect rule 1 action 'accept'
set policy route surfprotect rule 1 destination group address-group 'DstBypass'
set policy route surfprotect rule 1 destination port '80,443'
set policy route surfprotect rule 1 protocol 'tcp_udp'

# Rule 2: bypass SrcBypass sources on ports 80,443
set policy route surfprotect rule 2 action 'accept'
set policy route surfprotect rule 2 destination port '80,443'
set policy route surfprotect rule 2 protocol 'tcp_udp'
set policy route surfprotect rule 2 source group address-group 'SrcBypass'

# Rule 3: drop QUIC (UDP 80,443) to force TCP
set policy route surfprotect rule 3 action 'drop'
set policy route surfprotect rule 3 destination address '0.0.0.0/0'
set policy route surfprotect rule 3 destination port '80,443'
set policy route surfprotect rule 3 protocol 'udp'

# Rule 5: TCP SYN to 80,443 -> table 100, clamp MSS to 1436
set policy route surfprotect rule 5 destination address '0.0.0.0/0'
set policy route surfprotect rule 5 destination port '80,443'
set policy route surfprotect rule 5 protocol 'tcp'
set policy route surfprotect rule 5 set table '100'
set policy route surfprotect rule 5 set tcp-mss '1436'
set policy route surfprotect rule 5 tcp flags syn

# Rule 10: all other TCP to 80,443 -> table 100
set policy route surfprotect rule 10 destination address '0.0.0.0/0'
set policy route surfprotect rule 10 destination port '80,443'
set policy route surfprotect rule 10 protocol 'tcp'
set policy route surfprotect rule 10 set table '100'

# Table 100 has a default route via GRE tunnel
set protocols static table 100 route 0.0.0.0/0 interface tun100
```

### How it works under the hood

1. nftables table `ze_policy` (type route, hook prerouting) processes l2tp* ingress traffic
2. Bypass rules (accept) skip marking for whitelisted destinations/sources
3. Drop rules block QUIC to force TCP-only Surfprotect
4. Marking rules set fwmark on matching HTTP/HTTPS TCP flows
5. ip rule: `fwmark 0x<mark> lookup table 100` maps marked packets to table 100
6. Table 100 has `0.0.0.0/0 dev tun100` (from spec-static-routes)
7. tun100 is the GRE tunnel to the Surfprotect content filter

## Required Reading

### Architecture Docs
- [ ] `internal/component/firewall/model.go` -- existing match/action types
  --> Constraint: MatchSourceAddress, MatchDestinationAddress, MatchSourcePort, MatchDestinationPort, MatchProtocol, MatchInputInterface, MatchConnState, Accept, Drop, Return, SetMark
  --> Decision: policy routing reuses firewall match types for traffic classification
- [ ] `internal/component/firewall/config.go` -- from-block/then-block parsing
  --> Constraint: parseFromBlock and parseThenBlock patterns
  --> Decision: policy routing config uses same from-block syntax for consistency
- [ ] `internal/plugins/firewall/nft/lower_linux.go` -- nftables expression lowering
  --> Constraint: lowerMatch and lowerAction type switches
  --> Decision: new action types (SetTable, SetTCPMSS) need lowering cases
- [ ] `internal/component/firewall/backend.go` -- Backend interface
  --> Constraint: Backend registered via RegisterBackend, Apply([]Table)
  --> Decision: policy routing uses the SAME firewall backend for nftables operations
- [ ] `vendor/github.com/vishvananda/netlink/rule.go` -- ip rule management
  --> Constraint: Rule struct: Priority, Table, Mark, Mask, IifName, OifName, Family
  --> Constraint: RuleAdd, RuleDel, RuleList functions available
- [ ] `plan/spec-fw-0-umbrella.md` -- firewall design decisions
  --> Decision 4: abstract types, not nftables-native
  --> Decision 8: ze_ prefix on all tables
- [ ] `plan/spec-fw-8-lns-gaps.md` -- interface wildcard matching
  --> Constraint: MatchInputInterface.Wildcard for l2tp* matching

### RFC Summaries (MUST for protocol work)

Not protocol work. Policy routing is a Linux kernel feature.

**Key insights:**
- Policy routing uses two kernel subsystems: nftables (packet marking) and ip rule (mark-to-table routing)
- nftables chain type "route" in hook "output" or "prerouting" can modify routing decisions via marks
- ip rules are ordered by priority (lower = earlier evaluation)
- ze should own its ip rules via a priority range or mark range to avoid collisions
- VyOS uses fwmark-based policy routing (not source/destination-based ip rules) because matching criteria are complex (port, protocol, TCP flags)
- TCP MSS clamping in nftables: set the MSS option in TCP SYN packets via the `tcp option maxseg size set` expression

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/model.go` -- 18 match types, 24 action types. No MatchTCPFlags, no SetTable action, no SetTCPMSS action
- [ ] `internal/component/firewall/config.go` -- parseThenBlock has no "table" or "tcp-mss" key handling
- [ ] `internal/plugins/firewall/nft/lower_linux.go` -- no TCPFlags, SetTable, or SetTCPMSS lowering
- [ ] `internal/component/firewall/schema/ze-firewall-conf.yang` -- no tcp-flags in from-block, no table/tcp-mss in then-block
- [ ] No ip rule management anywhere in ze
- [ ] No policy routing component exists

**Behavior to preserve:**
- All existing firewall match/action types and their lowering
- Firewall ze_ table prefix and ownership
- Firewall Apply reconciliation semantics
- Existing firewall component unchanged (policy routing is a separate component that produces its own firewall tables)

**Behavior to change:**
- Add new match type: MatchTCPFlags (SYN, ACK, FIN, RST, etc.)
- Add new action types: SetTable (internally: SetMark + managed ip rule), SetTCPMSS
- Add lowering for new types in firewallnft
- Add new policy routing component that:
  - Produces firewall Table(s) for nftables (type route, hook prerouting)
  - Manages ip rules via netlink (fwmark -> table mapping)
  - Owns a mark range to avoid collisions with user SetMark

## Data Flow (MANDATORY)

### Entry Point
- YANG config file parsed at startup or reload by config component
- Config tree contains `policy { route { ... } }` section

### Transformation Path
1. Config file: `policy { route surfprotect { interface "l2tp*"; rule ... } }`
2. YANG validation
3. Config parser builds PolicyRoute struct (name, interfaces, rules with match/action)
4. Component translates PolicyRoute into firewall.Table:
   - Table name: `ze_pr_surfprotect` (type route, hook prerouting, priority -150)
   - Interface match: each rule prepended with MatchInputInterface{Name:"l2tp", Wildcard:true}
   - "set table 100" becomes SetMark{Value: allocated_mark} + register ip rule
   - "accept" becomes Accept (skip this policy, packet routes normally)
   - "drop" becomes Drop
   - "set tcp-mss 1436" becomes SetTCPMSS{Size: 1436}
5. Component calls firewall.GetBackend().Apply() for the nftables table
6. Component calls netlink RuleAdd for fwmark-to-table ip rules
7. On reload: reconcile both nftables tables and ip rules

### Mark Allocation

Each "set table N" action in a policy route needs a unique fwmark value. The component
allocates marks from a reserved range:

| Range | Owner | Purpose |
|-------|-------|---------|
| 0x50000-0x5FFFF | policy routing | fwmark values for table routing |
| Other | user | user-controlled marks in firewall config |

Allocation: deterministic hash of policy-name + table-id to a mark value in the range.
Or sequential allocation tracked in the component state. The mark value is internal;
users never see it.

### ip Rule Management

For each unique "set table N" action, the component creates one ip rule:

```
ip rule add fwmark 0x50001 lookup table 100 priority 100
```

Rules are created at Apply time and removed on cleanup. The component tracks which rules
it owns (by priority range or mark range) for reconciliation.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config --> Component | YANG tree JSON via SDK OnConfigure | [ ] |
| Component --> Firewall backend | firewall.GetBackend().Apply([]Table) | [ ] |
| Component --> Kernel ip rules | netlink.RuleAdd / netlink.RuleDel | [ ] |
| Firewall backend --> Kernel nftables | google/nftables (existing) | [ ] |

### Integration Points
- `internal/component/firewall/` -- reuses match types, action types, and Backend for nftables
- `internal/component/firewall/model.go` -- new types added here (MatchTCPFlags, SetTable, SetTCPMSS)
- `internal/plugins/firewall/nft/lower_linux.go` -- new lowering cases added here
- `vendor/github.com/vishvananda/netlink` -- RuleAdd/RuleDel for ip rules
- `internal/component/staticroute/` -- populates tables referenced by SetTable actions

### Architectural Verification
- [ ] No bypassed layers (config --> component --> firewall backend + netlink rules)
- [ ] No unintended coupling (policy routing component is separate; shares firewall types/backend)
- [ ] No duplicated functionality (extends firewall model, does not recreate it)
- [ ] Zero-copy not applicable

## New Types (added to firewall model)

### MatchTCPFlags (new match type, added to model.go)

| Field | Type | Description |
|-------|------|-------------|
| Flags | TCPFlags | Bitmask of flags to match |
| Mask | TCPFlags | Which flags to check (optional, defaults to Flags) |

TCPFlags enum (bitmask):

| Flag | Value | Description |
|------|-------|-------------|
| TCPFlagFIN | 0x01 | FIN |
| TCPFlagSYN | 0x02 | SYN |
| TCPFlagRST | 0x04 | RST |
| TCPFlagPSH | 0x08 | PSH |
| TCPFlagACK | 0x10 | ACK |
| TCPFlagURG | 0x20 | URG |

nftables lowering: `Payload(TransportHeader, offset=13, len=1) + Bitwise(mask) + Cmp(flags)`
(TCP flags are in byte 13 of the TCP header)

### SetTCPMSS (new action type, added to model.go)

| Field | Type | Description |
|-------|------|-------------|
| Size | uint16 | MSS value in bytes (e.g., 1436) |

nftables lowering: uses `expr.Exthdr` to set TCP option MSS. In nftables syntax this is
`tcp option maxseg size set 1436`. The google/nftables library expresses this as:
- `Exthdr{Op: ExthdrOpTcpopt, Type: 2 (MSS), Offset: 2 (after kind+length), Len: 2, SourceRegister: true, Register: 1}`
- `Immediate{Register: 1, Data: bigEndian(1436)}`

### SetTable (component-internal, NOT added to model.go)

SetTable is NOT a firewall action type. It is a policy routing concept that the component
translates into: SetMark{Value: allocated_mark} in the nftables rule, plus an ip rule
`fwmark <mark> lookup table <N>` in the kernel. The user writes `table 100` in config;
the component handles the mark allocation and ip rule creation internally.

## Config Syntax

```
policy {
    route surfprotect {
        interface "l2tp*"

        rule bypass-dst {
            from {
                destination-address @DstBypass
                destination-port 80,443
                protocol tcp
            }
            then {
                accept
            }
        }
        rule bypass-src {
            from {
                destination-port 80,443
                protocol tcp
                source-address @SrcBypass
            }
            then {
                accept
            }
        }
        rule block-quic {
            from {
                destination-address 0.0.0.0/0
                destination-port 80,443
                protocol udp
            }
            then {
                drop
            }
        }
        rule surfprotect-syn {
            from {
                destination-address 0.0.0.0/0
                destination-port 80,443
                protocol tcp
                tcp-flags syn
            }
            then {
                table 100
                tcp-mss 1436
            }
        }
        rule surfprotect-tcp {
            from {
                destination-address 0.0.0.0/0
                destination-port 80,443
                protocol tcp
            }
            then {
                table 100
            }
        }
    }
}
```

### Key differences from firewall config

| Aspect | Firewall | Policy routing |
|--------|----------|---------------|
| Table creation | Explicit (user defines table/chain/hook) | Implicit (component creates ze_pr_* table, type route, hook prerouting) |
| Interface binding | Per-rule match | Per-policy (all rules in a policy share the interface match) |
| "table N" action | Not supported | Translated to fwmark + ip rule |
| "tcp-mss N" action | Not supported | Translates to nftables TCP option write |
| "accept" meaning | Accept packet | Skip this policy (packet routes normally, no mark applied) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | --> | Feature Code | Test |
|-------------|-----|--------------|------|
| Config with policy route at boot | --> | ParsePolicyConfig, build Table, Apply, RuleAdd | `test/policy/001-boot-apply.ci` |
| Config with "set table 100" action | --> | Mark allocation, SetMark in nftables, ip rule created | `test/policy/002-set-table.ci` |
| Config with tcp-flags syn match | --> | MatchTCPFlags lowered to Payload+Bitwise+Cmp | `test/policy/003-tcp-flags.ci` |
| Config with tcp-mss action | --> | SetTCPMSS lowered to Exthdr write | `test/policy/004-tcp-mss.ci` |
| Config reload changes policy | --> | Reconcile nftables + ip rules | `test/policy/005-reload.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with policy route "surfprotect" on interface "l2tp*" | nftables table ze_pr_surfprotect created (type route, hook prerouting) with rules prepended by iifname "l2tp*" wildcard match |
| AC-2 | Rule with `then { accept; }` | nftables rule produces Verdict(ACCEPT), packet bypasses policy |
| AC-3 | Rule with `then { drop; }` | nftables rule produces Verdict(DROP) |
| AC-4 | Rule with `then { table 100; }` | nftables rule sets fwmark; ip rule `fwmark X lookup 100` created |
| AC-5 | Rule with `then { tcp-mss 1436; }` | nftables rule writes TCP MSS option to 1436 |
| AC-6 | Rule with `from { tcp-flags syn; }` | nftables rule matches TCP SYN flag in transport header byte 13 |
| AC-7 | Multiple policy routes with different tables | Each gets unique fwmark, each has separate ip rule |
| AC-8 | Config reload removes a policy route | nftables table deleted, ip rules removed |
| AC-9 | Config reload changes rules within a policy | nftables table reconciled, ip rules updated |
| AC-10 | `ze policy show` | Displays policy routes with interface binding, rules, actions |
| AC-11 | Rule with `from { destination-address @DstBypass; }` | Set reference resolved via existing firewall set mechanism |
| AC-12 | ze boots with policy + static route config | Traffic matching policy rules is forwarded via table 100 through tun100 |
| AC-13 | Rule with `from { tcp-flags syn,ack; }` | Matches packets with both SYN and ACK set |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMatchTCPFlags` | `internal/component/firewall/model_test.go` | MatchTCPFlags implements Match, flag bitmask | |
| `TestSetTCPMSS` | `internal/component/firewall/model_test.go` | SetTCPMSS implements Action | |
| `TestParseTCPFlags` | `internal/component/firewall/config_test.go` | "tcp-flags" key parsed from from-block, symbolic names | |
| `TestParsePolicyConfig` | `internal/component/policyroute/config_test.go` | Policy route JSON to PolicyRoute struct | |
| `TestParsePolicyConfigTable` | `internal/component/policyroute/config_test.go` | "table 100" in then-block parsed | |
| `TestParsePolicyConfigTCPMSS` | `internal/component/policyroute/config_test.go` | "tcp-mss 1436" in then-block parsed | |
| `TestPolicyToFirewallTable` | `internal/component/policyroute/translate_test.go` | PolicyRoute translated to firewall.Table | |
| `TestMarkAllocation` | `internal/component/policyroute/marks_test.go` | Unique marks allocated for different table IDs | |
| `TestLowerTCPFlags` | `internal/plugins/firewall/nft/lower_linux_test.go` | MatchTCPFlags produces Payload(13,1)+Bitwise+Cmp | |
| `TestLowerSetTCPMSS` | `internal/plugins/firewall/nft/lower_linux_test.go` | SetTCPMSS produces Exthdr MSS write | |
| `TestFormatTCPFlags` | `internal/component/firewall/cmd/show_test.go` | formatMatch displays "tcp flags syn" | |
| `TestFormatSetTCPMSS` | `internal/component/firewall/cmd/show_test.go` | formatAction displays "tcp-mss 1436" | |
| `TestPolicyRegistration` | `internal/component/policyroute/register_test.go` | registry.Register succeeds | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| TCP MSS | 1-65535 | 65535 | 0 (invalid) | 65536 (parse error, uint16) |
| Table ID (in then-block) | 1-4294967295 | 4294967295 | 0 (invalid, 0 = main table, use direct routing instead) | N/A (uint32) |
| TCP flags | 0x01-0x3F | 0x3F (all flags) | 0 (no flags, rejected) | 0x40+ (invalid bits) |
| fwmark range | 0x50000-0x5FFFF | 0x5FFFF | N/A (internal) | N/A (internal) |
| ip rule priority | internal | internal | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Boot apply | `test/policy/001-boot-apply.ci` | Policy route at boot, nftables table + ip rule created | |
| Set table | `test/policy/002-set-table.ci` | "table 100" action creates fwmark rule + ip rule | |
| TCP flags | `test/policy/003-tcp-flags.ci` | tcp-flags syn matched in nftables | |
| TCP MSS | `test/policy/004-tcp-mss.ci` | tcp-mss 1436 clamped in nftables | |
| Reload | `test/policy/005-reload.ci` | Policy change reconciled | |

### Future (if deferring any tests)
- None. All tests required for LNS replacement.

## Files to Modify

- `internal/component/firewall/model.go` -- add MatchTCPFlags, SetTCPMSS types
- `internal/component/firewall/config.go` -- add tcp-flags in parseFromBlock, tcp-mss in parseThenBlock
- `internal/component/firewall/schema/ze-firewall-conf.yang` -- add tcp-flags to from-block, tcp-mss to then-block
- `internal/plugins/firewall/nft/lower_linux.go` -- add MatchTCPFlags and SetTCPMSS lowering
- `internal/component/firewall/cmd/show.go` -- add MatchTCPFlags and SetTCPMSS formatting

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (firewall extensions) | Yes | `internal/component/firewall/schema/ze-firewall-conf.yang` |
| YANG schema (policy routing) | Yes | `internal/component/policyroute/schema/ze-policy-conf.yang` |
| CLI commands | Yes | `internal/component/policyroute/cmd/show.go` |
| Editor autocomplete | Yes | YANG-driven |
| Functional test | Yes | `test/policy/*.ci` |
| Plugin import generation | Yes | `scripts/gen-plugin-imports.go` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- policy routing |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` -- policy route config, tcp-flags, tcp-mss |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` -- `ze policy show` |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` -- policyroute component |
| 6 | Has a user guide page? | Yes | `docs/guide/policy-routing.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- policy routing |
| 12 | Internal architecture changed? | No | Follows existing patterns |

## Files to Create

- `internal/component/policyroute/model.go` -- PolicyRoute, PolicyRule structs
- `internal/component/policyroute/config.go` -- config JSON to PolicyRoute parsing
- `internal/component/policyroute/translate.go` -- PolicyRoute to firewall.Table translation
- `internal/component/policyroute/marks.go` -- fwmark allocation for table routing
- `internal/component/policyroute/rules_linux.go` -- ip rule management via netlink
- `internal/component/policyroute/rules_other.go` -- noop for non-Linux
- `internal/component/policyroute/register.go` -- registry.Register, RunEngine
- `internal/component/policyroute/schema/ze-policy-conf.yang` -- YANG schema
- `internal/component/policyroute/schema/register.go` -- YANG module registration
- `internal/component/policyroute/schema/embed.go` -- embed YANG file
- `internal/component/policyroute/cmd/show.go` -- CLI show formatting
- `internal/component/policyroute/model_test.go` -- model tests
- `internal/component/policyroute/config_test.go` -- config parsing tests
- `internal/component/policyroute/translate_test.go` -- translation tests
- `internal/component/policyroute/marks_test.go` -- mark allocation tests
- `internal/component/policyroute/register_test.go` -- registration tests
- `internal/component/policyroute/cmd/show_test.go` -- CLI formatting tests
- `test/policy/001-boot-apply.ci` -- functional test
- `test/policy/002-set-table.ci` -- functional test
- `test/policy/003-tcp-flags.ci` -- functional test
- `test/policy/004-tcp-mss.ci` -- functional test
- `test/policy/005-reload.ci` -- functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + spec-fw-8-lns-gaps + spec-static-routes |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5-12 | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Firewall model extensions** -- MatchTCPFlags, SetTCPMSS types + lowering + CLI
   - Tests: TestMatchTCPFlags, TestSetTCPMSS, TestLowerTCPFlags, TestLowerSetTCPMSS, TestFormatTCPFlags, TestFormatSetTCPMSS, TestParseTCPFlags
   - Files: firewall/model.go, firewall/config.go, firewallnft/lower_linux.go, firewall/cmd/show.go, ze-firewall-conf.yang
   - Verify: tests fail --> implement --> tests pass

2. **Phase: Policy route data model** -- PolicyRoute, PolicyRule structs
   - Tests: TestParsePolicyConfig, TestParsePolicyConfigTable, TestParsePolicyConfigTCPMSS
   - Files: policyroute/model.go, policyroute/config.go, ze-policy-conf.yang
   - Verify: tests fail --> implement --> tests pass

3. **Phase: Translation layer** -- PolicyRoute to firewall.Table
   - Tests: TestPolicyToFirewallTable
   - Files: policyroute/translate.go
   - Verify: tests fail --> implement --> tests pass

4. **Phase: Mark allocation + ip rules** -- fwmark management and netlink rules
   - Tests: TestMarkAllocation
   - Files: policyroute/marks.go, policyroute/rules_linux.go, policyroute/rules_other.go
   - Verify: tests fail --> implement --> tests pass

5. **Phase: Component reactor** -- registration, lifecycle, Apply orchestration
   - Tests: TestPolicyRegistration
   - Files: policyroute/register.go
   - Verify: tests fail --> implement --> tests pass

6. **Phase: CLI** -- show command
   - Files: policyroute/cmd/show.go
   - Verify: tests fail --> implement --> tests pass

7. **Functional tests** --> All .ci tests
8. **Full verification** --> `make ze-verify`
9. **Complete spec** --> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N (AC-1 through AC-13) has implementation with file:line |
| Correctness | TCP flags byte offset is 13 in TCP header; MSS option type is 2; fwmark values unique |
| Naming | Config keys: tcp-flags, tcp-mss, table, policy, route (lowercase hyphenated) |
| Data flow | Config --> PolicyRoute --> translate to Table --> firewall backend Apply + netlink RuleAdd |
| Mark isolation | Policy routing marks in 0x50000-0x5FFFF range, no collision with user marks |
| ip rule cleanup | All ze-created ip rules removed on shutdown / config change |
| Interface wildcard | l2tp* prepended to every rule in the policy (not just first rule) |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| MatchTCPFlags in firewall model | `grep "MatchTCPFlags" internal/component/firewall/model.go` |
| SetTCPMSS in firewall model | `grep "SetTCPMSS" internal/component/firewall/model.go` |
| policyroute component exists | `ls internal/component/policyroute/` |
| Translation produces firewall.Table | `grep "firewall.Table" internal/component/policyroute/translate.go` |
| ip rule management | `grep "RuleAdd\|RuleDel" internal/component/policyroute/rules_linux.go` |
| Mark allocation | `grep "0x50000" internal/component/policyroute/marks.go` |
| Functional tests | `ls test/policy/*.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | TCP MSS validated (1-65535); table ID validated (1+); tcp-flags bitmask validated |
| Mark range | Policy marks cannot overlap with user marks; range enforced |
| ip rule priority | Priority chosen to not conflict with system rules (default, main, local) |
| ip rule cleanup | Stale ip rules cleaned up on shutdown to prevent route leaks |
| Table reference | Table ID in "set table N" should match a table populated by static routes (warning if not) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior --> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural --> DESIGN phase |
| Functional test fails | Check AC; if AC wrong --> DESIGN; if AC correct --> IMPLEMENT |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | Policy routing as separate component, not a firewall config extension | Different semantics: policy route has interface binding, implicit table/chain creation, mark-to-table mapping. Mixing into firewall config would be confusing (user doesn't think "create a route chain at prerouting priority -150"). |
| 2 | Reuse firewall match/action types, extend with MatchTCPFlags and SetTCPMSS | The match types are identical (address, port, protocol, interface). Adding TCP flags and MSS to the firewall model benefits both firewall and policy routing. No duplication. |
| 3 | SetTable is a component concept, not a firewall action type | "Set table 100" is policy routing sugar for "set fwmark X" + "ip rule fwmark X lookup 100". The firewall model should not know about ip rules. The policy routing component translates, keeping the firewall model clean. |
| 4 | fwmark-based table routing over direct nftables rt expression | fwmark + ip rule is the standard Linux approach, works with all kernels, well-understood. Direct nftables routing table expressions are newer and less portable. VyOS uses fwmark too. |
| 5 | Deterministic mark allocation from reserved range | Prevents mark collisions with user firewall rules. Reserved range 0x50000-0x5FFFF gives 65536 possible marks, more than enough. Deterministic (hash of policy+table) means marks are stable across restarts. |
| 6 | Policy route creates nftables table with ze_pr_ prefix | Distinct from ze_ firewall tables. Easy to identify policy routing tables vs firewall tables. Both managed by the same firewallnft backend. |
| 7 | TCP MSS clamping via nftables Exthdr, not iptables TCPMSS target | nftables is the ze standard. Exthdr can write TCP options. No need for legacy iptables/xtables. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Implementation Summary

### What Was Implemented
- (To be filled after implementation)

### Bugs Found/Fixed
- (To be filled)

### Documentation Updates
- (To be filled)

### Deviations from Plan
- (To be filled)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-policy-routing.md`
- [ ] Summary included in commit
