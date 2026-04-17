# Spec: fw-8-lns-gaps -- Firewall gaps for VyOS LNS replacement

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-fw-0-umbrella |
| Phase | - |
| Updated | 2026-04-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/firewall/model.go` -- match/action types and interface markers
4. `internal/component/firewall/config.go` -- from-block and then-block parsing
5. `internal/plugins/firewallnft/lower_linux.go` -- nftables expression lowering
6. `internal/component/firewall/schema/ze-firewall-conf.yang` -- YANG schema
7. `internal/component/firewall/cmd/show.go` -- CLI formatting
8. `internal/component/iface/register.go` -- component registration pattern (registry.Register)

## Task

Close four gaps in the firewall component that block replacing VyOS on the Exa LNS
(sp-lns2.tcw.man). The LNS VyOS config requires ICMP type matching, interface wildcard
matching, NAT exclude rules, and the component reactor that wires the firewall into ze's
engine lifecycle. Without these, the firewall data model exists but cannot express the LNS
ruleset and does not run at boot or on config reload.

### Gap summary

| # | Gap | Why needed | Scope |
|---|-----|-----------|-------|
| 1 | ICMP type matching | VyOS `icmp type-name echo-request` (IPv4 rule 40) and `protocol icmpv6` type filtering (IPv6 rule 15) | New match types, config, YANG, lowering, CLI |
| 2 | Interface wildcard matching | VyOS `inbound-interface name 'l2tp*'` in NAT rules | Flag on existing match types, adjusted lowering |
| 3 | NAT exclude rules | VyOS `nat destination rule N exclude` skips NAT for matched traffic | Config parsing recognises `exclude` flag, emits Return action |
| 4 | Component reactor | Firewall has model/config/backend/CLI but no engine registration; rules never apply | New register.go following iface pattern |

## Required Reading

### Architecture Docs
- [ ] `internal/component/iface/register.go` -- component registration via registry.Register
  --> Constraint: init() creates registry.Registration with Name, YANG, ConfigRoots, RunEngine
  --> Decision: RunEngine receives net.Conn, uses SDK 5-stage protocol (verify, apply, rollback)
- [ ] `internal/component/firewall/model.go` -- 18 match types, 24 action types
  --> Constraint: Match interface with matchMarker(), Action interface with actionMarker()
  --> Decision: new match types must implement Match interface
- [ ] `internal/component/firewall/config.go` -- parseFromBlock and parseThenBlock
  --> Constraint: from-block builds []Match, then-block builds []Action
  --> Decision: new config keys parsed in same pattern as existing ones
- [ ] `internal/plugins/firewallnft/lower_linux.go` -- lowerMatch and lowerAction type switches
  --> Constraint: every match/action type needs a case in the type switch or returns error
  --> Decision: lowering produces []expr.Any for nftables kernel programming
- [ ] `internal/component/firewall/schema/ze-firewall-conf.yang` -- from-block and then-block groupings
  --> Constraint: new match types need leaves in from-block grouping
- [ ] `internal/component/firewall/cmd/show.go` -- formatMatch and formatAction type switches
  --> Constraint: new types need cases in format functions or display as `<T>`
- [ ] `plan/spec-fw-0-umbrella.md` -- design decisions 3, 4, 8, 9, 14
  --> Decision 3: Backend Apply([]Table) is the write path
  --> Decision 4: abstract types, not nftables-native
  --> Decision 8: ze_ prefix on all tables
  --> Decision 9: Apply on startup + reload, same code path
  --> Decision 14: registry.Register() in init()
- [ ] `pkg/plugin/sdk/` -- SDK 5-stage protocol (NewWithConn, OnConfigure, OnConfigReload)
  --> Constraint: plugin receives config via OnConfigure, reloads via OnConfigReload

### RFC Summaries (MUST for protocol work)

Not protocol work per se, but ICMP type numbers come from IANA assignments:
- IPv4 ICMP types: RFC 792 (echo-request = type 8, echo-reply = type 0)
- ICMPv6 types: RFC 4443 (echo-request = type 128, echo-reply = type 129, etc.)

No rfc/short/ entries needed; the type numbers are well-known constants.

**Key insights:**
- Component registration follows iface pattern: registry.Registration in init() with RunEngine callback
- RunEngine uses SDK 5-stage protocol: conn -> sdk.NewWithConn -> OnConfigure -> OnConfigReload
- Match types implement the Match interface via matchMarker()
- Config parsing builds []Match in parseFromBlock, new keys slot in alongside existing ones
- nftables lowering is a type switch in lowerMatch; new cases produce []expr.Any
- Interface wildcard in nftables: compare only prefix bytes of iifname, not full 16-byte padded name
- NAT exclude in nftables: rule in nat chain that matches traffic and returns (verdict RETURN)
- ICMP type matching in nftables: Payload(TransportHeader, offset=0, len=1) + Cmp for type byte

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/model.go` -- 18 match types defined, no MatchICMPType or MatchICMPv6Type
  --> Constraint: Match interface requires matchMarker() method
- [ ] `internal/component/firewall/config.go` -- parseFromBlock handles 11 keys (source-address through dscp), no icmp-type or wildcard flag; parseThenBlock handles verdicts and NAT but no exclude concept
  --> Constraint: config JSON comes from YANG tree, keys are lowercase hyphenated
- [ ] `internal/plugins/firewallnft/lower_linux.go` -- lowerMatch has 10 cases (MatchSourceAddress through MatchDSCP), MatchInputInterface does exact 16-byte comparison; lowerAction has 15 cases
  --> Constraint: ifnameBytes pads to 16 bytes; Cmp uses CmpOpEq (exact match only)
- [ ] `internal/component/firewall/schema/ze-firewall-conf.yang` -- from-block grouping has 11 leaves, no icmp-type
- [ ] `internal/component/firewall/cmd/show.go` -- formatMatch has 11 cases, formatAction has 15 cases
- [ ] `internal/component/firewall/backend.go` -- Backend interface defined, RegisterBackend/LoadBackend/GetBackend exist, but no register.go in the firewall package
- [ ] `internal/component/iface/register.go` -- template for component reactor (registry.Register in init(), RunEngine callback with SDK 5-stage protocol)

**Behavior to preserve:**
- All 18 existing match types and their lowering
- All 24 existing action types and their lowering
- Exact interface matching (MatchInputInterface without wildcard flag continues to work as before)
- Config parser backwards compatibility (existing configs still parse)
- ze_ table prefix and ownership rules
- Backend interface (Apply, ListTables, GetCounters) unchanged

**Behavior to change:**
- Add MatchICMPType and MatchICMPv6Type match types
- Add Wildcard flag to MatchInputInterface and MatchOutputInterface
- Add `exclude` recognition in NAT config parsing (emits Return action)
- Add component reactor (register.go) that wires firewall into engine lifecycle

## Data Flow (MANDATORY)

### Entry Point
- YANG config file parsed at startup or reload by config component
- Config tree contains `firewall { ... }` section
- For gap 4 (reactor): engine starts firewall plugin, plugin receives config via SDK

### Transformation Path

**Gap 1 (ICMP type):**
1. Config file: `from { icmp-type echo-request; }` or `from { icmpv6-type echo-request; }`
2. YANG validation: icmp-type leaf in from-block grouping
3. Config parser: parseFromBlock reads "icmp-type" key, maps name to number, creates MatchICMPType
4. Backend Apply: lowerMatch produces Payload(TransportHeader, 0, 1) + Cmp(type number)
5. Kernel: nftables rule matches ICMP type byte

**Gap 2 (interface wildcard):**
1. Config file: `from { input-interface "l2tp*"; }`
2. Config parser: parseFromBlock detects trailing `*`, strips it, sets Wildcard=true on MatchInputInterface
3. Backend Apply: lowerMatch uses ifnameBytes with prefix length (not padded to 16), and `CmpOpEq` on just the prefix bytes
4. Kernel: nftables meta iifname prefix match

**Gap 3 (NAT exclude):**
1. Config file: `then { exclude; }` in a NAT chain term
2. Config parser: parseThenBlock detects "exclude" key, emits Return action
3. Backend Apply: lowerAction produces Verdict(RETURN) (already implemented)
4. Kernel: rule returns from NAT chain without applying translation

**Gap 4 (component reactor):**
1. ze boots, engine loads plugins via registry
2. Firewall register.go: init() calls registry.Register with Name="firewall", ConfigRoots=["firewall"], RunEngine=runEngine
3. runEngine: sdk.NewWithConn, receives config via OnConfigure
4. OnConfigure: calls ParseFirewallConfig, then LoadBackend + backend.Apply
5. OnConfigReload: same path (verify new config, apply, rollback on failure)
6. Shutdown: calls CloseBackend

Related: `plan/learned/621-backend-feature-gate.md` describes the commit-time
`ze:backend` walker that iface already calls from its `OnConfigure` /
`OnConfigVerify`. The firewall reactor is the natural call site for the same
helper; whether to bundle that wiring with Gap 4 or land it in a follow-up
commit is an implementation choice for this spec's author.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config --> Component | YANG tree JSON via SDK OnConfigure callback | [ ] |
| Component --> Backend | ParseFirewallConfig --> backend.Apply([]Table) | [ ] |
| Backend --> Kernel | google/nftables netlink (existing) | [ ] |

### Integration Points
- `internal/component/plugin/registry/` -- Register() for component lifecycle
- `pkg/plugin/sdk/` -- SDK 5-stage protocol for config delivery
- `internal/component/firewall/config.go` -- ParseFirewallConfig (existing)
- `internal/component/firewall/backend.go` -- LoadBackend, GetBackend, CloseBackend (existing)

### Architectural Verification
- [ ] No bypassed layers (config --> component --> backend --> kernel)
- [ ] No unintended coupling (firewall reactor is self-contained like iface)
- [ ] No duplicated functionality (extends existing model/config/lowering)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | --> | Feature Code | Test |
|-------------|-----|--------------|------|
| Config with `icmp-type echo-request` | --> | parseFromBlock creates MatchICMPType, lowerMatch produces Payload+Cmp | `test/firewall/010-icmp-type.ci` |
| Config with `input-interface "l2tp*"` | --> | parseFromBlock creates MatchInputInterface{Wildcard:true}, lowerMatch prefix match | `test/firewall/011-iface-wildcard.ci` |
| Config with NAT `exclude` | --> | parseThenBlock creates Return, lowerAction produces Verdict(RETURN) | `test/firewall/012-nat-exclude.ci` |
| ze boots with firewall config | --> | registry.Register, RunEngine, OnConfigure, Apply | `test/firewall/001-boot-apply.ci` (update existing) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config with `from { icmp-type echo-request; }` in inet table | MatchICMPType{Type: 8} created, nftables rule matches ICMP type 8 |
| AC-2 | Config with `from { icmp-type echo-reply; }` | MatchICMPType{Type: 0} created |
| AC-3 | Config with `from { icmpv6-type echo-request; }` | MatchICMPv6Type{Type: 128} created |
| AC-4 | Config with `from { icmpv6-type neighbor-solicitation; }` | MatchICMPv6Type{Type: 135} created |
| AC-5 | Config with `from { input-interface "l2tp*"; }` | MatchInputInterface{Name:"l2tp", Wildcard:true} created |
| AC-6 | Config with `from { input-interface "eth0"; }` (no wildcard) | MatchInputInterface{Name:"eth0", Wildcard:false} (backwards compatible) |
| AC-7 | Config with NAT chain term containing `then { exclude; }` | Return action emitted, nftables produces Verdict(RETURN) |
| AC-8 | ze boots with firewall config section | Firewall plugin starts, calls Apply, ze_* tables created in kernel |
| AC-9 | ze config reload changes firewall section | OnConfigReload triggers re-parse and Apply |
| AC-10 | ze boots with no firewall config section | Firewall plugin starts but Apply called with empty slice (no tables, no error) |
| AC-11 | `ze firewall show` after ICMP type term | Displays "icmp type echo-request" in from block |
| AC-12 | `ze firewall show` after wildcard interface term | Displays "input interface l2tp*" (with asterisk) |
| AC-13 | Config with `from { icmp-type 8; }` (numeric) | MatchICMPType{Type: 8} created (numeric fallback) |
| AC-14 | Config with `from { output-interface "veth*"; }` | MatchOutputInterface{Name:"veth", Wildcard:true} created |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMatchICMPType` | `internal/component/firewall/model_test.go` | MatchICMPType implements Match, fields correct | |
| `TestMatchICMPv6Type` | `internal/component/firewall/model_test.go` | MatchICMPv6Type implements Match, fields correct | |
| `TestParseICMPType` | `internal/component/firewall/config_test.go` | "icmp-type" key parsed from from-block, symbolic + numeric | |
| `TestParseICMPv6Type` | `internal/component/firewall/config_test.go` | "icmpv6-type" key parsed from from-block | |
| `TestParseInterfaceWildcard` | `internal/component/firewall/config_test.go` | Trailing `*` sets Wildcard=true, strips `*` from Name | |
| `TestParseInterfaceExact` | `internal/component/firewall/config_test.go` | No `*` keeps Wildcard=false (regression) | |
| `TestParseNATExclude` | `internal/component/firewall/config_test.go` | "exclude" key in then-block produces Return action | |
| `TestLowerICMPType` | `internal/plugins/firewallnft/lower_linux_test.go` | MatchICMPType produces Payload(Transport,0,1)+Cmp(type) | |
| `TestLowerICMPv6Type` | `internal/plugins/firewallnft/lower_linux_test.go` | MatchICMPv6Type produces Payload(Transport,0,1)+Cmp(type) | |
| `TestLowerInterfaceWildcard` | `internal/plugins/firewallnft/lower_linux_test.go` | Wildcard produces prefix-length comparison, not 16-byte | |
| `TestLowerInterfaceExact` | `internal/plugins/firewallnft/lower_linux_test.go` | Non-wildcard produces 16-byte exact match (regression) | |
| `TestFormatICMPType` | `internal/component/firewall/cmd/show_test.go` | formatMatch displays "icmp type echo-request" | |
| `TestFormatICMPv6Type` | `internal/component/firewall/cmd/show_test.go` | formatMatch displays "icmpv6 type echo-request" | |
| `TestFormatInterfaceWildcard` | `internal/component/firewall/cmd/show_test.go` | formatMatch displays "input interface l2tp*" | |
| `TestFirewallRegistration` | `internal/component/firewall/register_test.go` | registry.Register succeeds, Name="firewall", ConfigRoots=["firewall"] | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ICMP type | 0-255 | 255 | N/A (uint8, 0 is valid echo-reply) | 256 (parse error) |
| ICMPv6 type | 0-255 | 255 | N/A (uint8) | 256 (parse error) |
| Interface name length | 1-15 | 15 chars (IFNAMSIZ-1) | 0 (empty, rejected) | 16+ (kernel rejects) |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| ICMP type match | `test/firewall/010-icmp-type.ci` | Config with icmp-type echo-request, verify nftables rule created | |
| Interface wildcard | `test/firewall/011-iface-wildcard.ci` | Config with l2tp* wildcard, verify prefix match in nftables | |
| NAT exclude | `test/firewall/012-nat-exclude.ci` | NAT chain with exclude term, verify RETURN verdict in nftables | |
| Boot apply | `test/firewall/001-boot-apply.ci` | Firewall config at boot, ze_* tables in kernel (update existing if present) | |

### Future (if deferring any tests)
- None. All tests are required for LNS replacement.

## Files to Modify

- `internal/component/firewall/model.go` -- add MatchICMPType, MatchICMPv6Type types; add Wildcard field to MatchInputInterface, MatchOutputInterface
- `internal/component/firewall/config.go` -- add icmp-type, icmpv6-type parsing in parseFromBlock; wildcard detection on interface names; exclude key in parseThenBlock
- `internal/component/firewall/schema/ze-firewall-conf.yang` -- add icmp-type and icmpv6-type leaves to from-block grouping; add exclude leaf to then-block grouping
- `internal/plugins/firewallnft/lower_linux.go` -- add MatchICMPType and MatchICMPv6Type cases in lowerMatch; modify MatchInputInterface/MatchOutputInterface cases for wildcard
- `internal/component/firewall/cmd/show.go` -- add MatchICMPType, MatchICMPv6Type, wildcard cases in formatMatch
- `internal/component/firewall/model_test.go` -- add tests for new match types
- `internal/component/firewall/config_test.go` -- add tests for new config parsing
- `internal/component/firewall/cmd/show_test.go` -- add tests for new format cases

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | Yes | `internal/component/firewall/schema/ze-firewall-conf.yang` |
| CLI commands/flags | No | Existing show/counters commands handle new types via format functions |
| Editor autocomplete | Yes | YANG-driven (automatic if YANG updated) |
| Functional test | Yes | `test/firewall/010-icmp-type.ci`, `011-iface-wildcard.ci`, `012-nat-exclude.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- add ICMP type matching, wildcard interfaces, NAT exclude |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` -- add icmp-type, icmpv6-type, wildcard, exclude examples |
| 3 | CLI command added/changed? | No | Existing commands, new output cases |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | Existing firewallnft extended, not new plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/firewall.md` -- add ICMP and NAT exclude sections |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | ICMP types are IANA constants, not RFC behavior |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- firewall now covers ICMP, wildcards, NAT exclude |
| 12 | Internal architecture changed? | No | Follows existing patterns |

## Files to Create

- `internal/component/firewall/register.go` -- component reactor, registry.Register in init(), RunEngine with SDK 5-stage protocol
- `internal/component/firewall/register_test.go` -- registration tests
- `internal/plugins/firewallnft/lower_linux_test.go` -- lowering tests for new types (if not already present)
- `test/firewall/010-icmp-type.ci` -- functional test
- `test/firewall/011-iface-wildcard.ci` -- functional test
- `test/firewall/012-nat-exclude.ci` -- functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + fw-0-umbrella |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: ICMP type matching** -- model, config, YANG, lowering, CLI
   - Tests: TestMatchICMPType, TestMatchICMPv6Type, TestParseICMPType, TestParseICMPv6Type, TestLowerICMPType, TestLowerICMPv6Type, TestFormatICMPType, TestFormatICMPv6Type
   - Files: model.go, config.go, ze-firewall-conf.yang, lower_linux.go, show.go
   - Verify: tests fail --> implement --> tests pass

2. **Phase: Interface wildcard** -- model field, config detection, lowering, CLI
   - Tests: TestParseInterfaceWildcard, TestParseInterfaceExact, TestLowerInterfaceWildcard, TestLowerInterfaceExact, TestFormatInterfaceWildcard
   - Files: model.go, config.go, lower_linux.go, show.go
   - Verify: tests fail --> implement --> tests pass

3. **Phase: NAT exclude** -- config parsing
   - Tests: TestParseNATExclude
   - Files: config.go, ze-firewall-conf.yang
   - Verify: tests fail --> implement --> tests pass
   - Note: Return action and its lowering already exist; only config parsing is new

4. **Phase: Component reactor** -- engine registration and lifecycle
   - Tests: TestFirewallRegistration, functional boot-apply test
   - Files: register.go (new)
   - Verify: tests fail --> implement --> tests pass
   - Note: follows iface/register.go pattern exactly

5. **Functional tests** --> Create after all phases work
6. **Full verification** --> `make ze-verify`
7. **Complete spec** --> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N (AC-1 through AC-14) has implementation with file:line |
| Correctness | ICMP type numbers match IANA assignments; wildcard produces correct nftables prefix match |
| Naming | Config keys use lowercase hyphenated (icmp-type, icmpv6-type); YANG leaves match |
| Data flow | Config --> parseFromBlock --> MatchICMPType --> lowerMatch --> Payload+Cmp |
| Backwards compat | Existing interface matches without wildcard still produce exact 16-byte comparison |
| Rule: no-layering | No compatibility shims; wildcard flag is a direct model extension |
| Rule: single-responsibility | Reactor in register.go only, not mixed into config.go or backend.go |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| MatchICMPType in model.go | `grep "MatchICMPType" internal/component/firewall/model.go` |
| MatchICMPv6Type in model.go | `grep "MatchICMPv6Type" internal/component/firewall/model.go` |
| Wildcard field on MatchInputInterface | `grep "Wildcard" internal/component/firewall/model.go` |
| icmp-type in config parser | `grep "icmp-type" internal/component/firewall/config.go` |
| exclude in config parser | `grep "exclude" internal/component/firewall/config.go` |
| ICMP lowering in firewallnft | `grep "MatchICMPType" internal/plugins/firewallnft/lower_linux.go` |
| Wildcard lowering in firewallnft | `grep "Wildcard" internal/plugins/firewallnft/lower_linux.go` |
| register.go exists | `ls internal/component/firewall/register.go` |
| Functional tests exist | `ls test/firewall/01*.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | ICMP type parsed as uint8 (0-255), reject values > 255 |
| Input validation | Interface wildcard name validated: non-empty prefix, reasonable length (< IFNAMSIZ) |
| Injection | NAT exclude cannot be combined with NAT actions in same term (would be contradictory; verify config rejects or warn) |
| Privilege | Component reactor inherits CAP_NET_ADMIN from parent ze process |
| Backwards compat | Existing configs without new keys must parse identically |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior --> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural --> DESIGN phase |
| Functional test fails | Check AC; if AC wrong --> DESIGN; if AC correct --> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | Separate MatchICMPType and MatchICMPv6Type rather than one type with a family field | ICMP and ICMPv6 have different type number spaces and different names; mixing them invites bugs. Matches the pattern of MatchSourceAddress / MatchDestinationAddress being separate types. |
| 2 | Wildcard as a bool field on existing MatchInputInterface/MatchOutputInterface, not a new type | The match concept is the same (interface name); wildcard is a modifier. Avoids doubling the type count for a flag. Backwards compatible: zero value (false) preserves exact match. |
| 3 | NAT exclude parsed as Return action, not a new action type | VyOS `exclude` is syntactic sugar for "match this traffic, return without NATting." Return action already exists and its lowering works. Only the config parser needs to recognise the keyword. |
| 4 | Symbolic ICMP type names as a map, numeric fallback | Same pattern as DSCP (dscpNames map + numeric parse). Covers the common names (echo-request, echo-reply, destination-unreachable, etc.) without encoding the full IANA registry. Numeric fallback covers edge cases. |
| 5 | Wildcard lowering uses prefix-length Cmp, not regex | nftables iifname match is a byte comparison. For "l2tp*", compare only 4 bytes "l2tp" instead of 16-byte padded "l2tp\0\0\0...". CmpOpEq on the 4-byte prefix is how nft internally handles `iifname "l2tp*"`. |
| 6 | Component reactor follows iface pattern exactly | registry.Register in init(), RunEngine with SDK 5-stage protocol. No reason to deviate from the established pattern. |

## ICMP Type Name Tables

### IPv4 ICMP types (from IANA / RFC 792)

| Name | Type | Code |
|------|------|------|
| echo-reply | 0 | - |
| destination-unreachable | 3 | - |
| source-quench | 4 | - |
| redirect | 5 | - |
| echo-request | 8 | - |
| router-advertisement | 9 | - |
| router-solicitation | 10 | - |
| time-exceeded | 11 | - |
| parameter-problem | 12 | - |
| timestamp-request | 13 | - |
| timestamp-reply | 14 | - |
| info-request | 15 | - |
| info-reply | 16 | - |
| address-mask-request | 17 | - |
| address-mask-reply | 18 | - |

### ICMPv6 types (from IANA / RFC 4443 / RFC 4861)

| Name | Type |
|------|------|
| destination-unreachable | 1 |
| packet-too-big | 2 |
| time-exceeded | 3 |
| parameter-problem | 4 |
| echo-request | 128 |
| echo-reply | 129 |
| router-solicitation | 133 |
| router-advertisement | 134 |
| neighbor-solicitation | 135 |
| neighbor-advertisement | 136 |
| redirect | 137 |

## Wildcard Interface Lowering Detail

Current exact match (MatchInputInterface with Name="eth0"):
1. Meta(IIFNAME) --> register 1 (16 bytes of interface name)
2. Cmp(EQ, register 1, ifnameBytes("eth0")) -- 16-byte padded comparison

Wildcard match (MatchInputInterface with Name="l2tp", Wildcard=true):
1. Meta(IIFNAME) --> register 1 (16 bytes of interface name)
2. Cmp(EQ, register 1, []byte("l2tp")) -- 4-byte prefix comparison only

The key is that Cmp.Data length controls how many bytes are compared. With 4 bytes,
nftables compares only the first 4 bytes of the interface name, effectively matching
any interface whose name starts with "l2tp".

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
- [ ] AC-1..AC-14 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
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
- [ ] Write learned summary to `plan/learned/NNN-fw-8-lns-gaps.md`
- [ ] Summary included in commit
