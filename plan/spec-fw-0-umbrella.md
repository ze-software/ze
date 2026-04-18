# Spec: fw-0-umbrella — Firewall and Traffic Control Architecture

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-04-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/iface/backend.go` — Backend interface pattern
4. `plan/spec-vpp-0-umbrella.md` — VPP integration (firewall backends are spec-vpp-5)

## Task

Replace VyOS firewall with native ze-managed nftables firewall and tc-based traffic control.
Ze owns its tables (prefixed `ze_`, boot-time, YANG-configured). Lachesis and other software
own their own tables. Neither touches the other's.

Two new components, four new plugins (two Linux backends, two VPP backends):

| Component | Linux plugin | VPP plugin |
|-----------|-------------|------------|
| `internal/component/firewall/` | `internal/plugins/firewall/nft/` | `internal/plugins/firewallvpp/` |
| `internal/component/traffic/` | `internal/plugins/traffic/netlink/` | `internal/plugins/trafficvpp/` |

## Child Specs

| Spec | Scope | Depends |
|------|-------|---------|
| `spec-fw-1-data-model.md` | Expression types, Table/Chain/Set/Flowtable/Rule, InterfaceQoS | This umbrella |
| `spec-fw-2-firewall-nft.md` | firewallnft plugin, google/nftables, reconciler | fw-1 |
| `spec-fw-3-traffic-netlink.md` | trafficnetlink plugin, vishvananda/netlink tc | fw-1 |
| `spec-fw-4-yang-config.md` | YANG modules, config parsing, readable keywords | fw-1 |
| `spec-fw-5-cli.md` | show, monitor, counters CLI commands | fw-2, fw-3, fw-4 |
| `spec-fw-6-firewall-vpp.md` | firewallvpp plugin, GoVPP ACL/classifier | VPP Phase 0 |
| `spec-fw-7-traffic-vpp.md` | trafficvpp plugin, GoVPP policer/scheduler | VPP Phase 0 |
| `spec-fw-8-lns-gaps.md` | ICMP type / iface wildcard / NAT exclude matches + firewall component reactor (Gap 4) | fw-1, fw-2, fw-4 |
| `spec-fw-9-traffic-lifecycle.md` | traffic component reactor (register.go, OnConfigure/Apply, backend-gate call wiring; annotation-driven rejection tests deferred to fw-7) | - |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component/plugin architecture
  → Constraint: components under `internal/component/`, plugins under `internal/plugins/`
  → Decision: backend interface pattern (component defines interface, plugin implements)
- [ ] `internal/component/iface/backend.go` - existing backend interface pattern
  → Constraint: Backend registered via RegisterBackend in init(), selected by config leaf
  → Decision: single-method Apply pattern chosen over granular methods (design decision 3)
- [ ] `plan/spec-vpp-0-umbrella.md` - VPP integration spec set
  → Constraint: VPP backends depend on vpp-1 (lifecycle management)
  → Decision: ACL and Policer/QoS are owned by firewallvpp (spec-fw-6) and trafficvpp (spec-fw-7), not by a separate VPP-native YANG surface. Replaces the original spec-vpp-5-features plan, retired 2026-04-17.
- [ ] `.claude/patterns/registration.md` - registration pattern
  → Constraint: init() + registry.Register() pattern for plugins
- [ ] `.claude/patterns/config-option.md` - config option pattern
  → Constraint: YANG leaf + env var registration for every config option
- [ ] `rules/config-design.md` - config design rules
  → Constraint: fail on unknown keys, no version numbers, listener grouping pattern

### RFC Summaries (MUST for protocol work)

Not protocol work. No RFCs apply.

**Key insights:**
- Backend interface: Apply (write) + ListTables/GetCounters/ListQdiscs (read). Plugin owns reconciliation.
- Table ownership: ze tables prefixed `ze_*`, plugin never touches non-`ze_*` tables
- Abstract data model: Term = Name + []Match + []Action. 42 types. nft backend lowers to register operations.
- Config syntax: hybrid Junos/ze. Named terms, from/then split, readable names, nftables concepts.
- Two separate components (firewall + traffic) because different kernel subsystems
- Apply on startup + reload, same code path, box boots with firewall active
- Components register as plugins via registry.Register() in init(), same as iface

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/backend.go` — Backend interface with RegisterBackend/LoadBackend/GetBackend
  → Constraint: same pattern for firewall and traffic backends
- [ ] `internal/plugins/iface/netlink/register.go` — init() calls iface.RegisterBackend("netlink", factory)
  → Constraint: firewallnft registers as "nft", trafficnetlink registers as "tc"
- [ ] `internal/plugins/fib/kernel/backend.go` — simpler backend without RegisterBackend
  → Constraint: fibkernel uses direct interface, not registration. Firewall uses registration like iface.
- [ ] `vendor/github.com/vishvananda/netlink/qdisc.go` — tc types already vendored
  → Constraint: HTB, HFSC, FQ, FQ_CoDel, SFQ, TBF, Netem, Prio, Clsact, Ingress available
- [ ] `vendor/github.com/mdlayher/netlink/` — already vendored, used by google/nftables

**Behavior to preserve:**
- No existing firewall or traffic control code in ze. This is greenfield.
- Existing iface and fibkernel components must not be affected.
- Lachesis's nftables tables must not be touched (only ze_* tables managed).

**Behavior to change:**
- Add new firewall component and traffic component
- Add google/nftables as new dependency (approved by user)

## Data Flow (MANDATORY)

### Entry Point
- YANG config file parsed at startup or reload
- Config tree contains `firewall { ... }` and `traffic-control { ... }` sections

### Transformation Path
1. Config file parsed into YANG tree by config component
2. Firewall component extracts `firewall` subtree, builds `[]Table` with expression pipelines
3. Traffic component extracts `traffic-control` subtree, builds `map[string]InterfaceQoS`
4. Each component calls its backend's `Apply(desired)` method
5. firewallnft plugin translates `[]Table` to google/nftables API calls (tables, chains, sets, rules)
6. trafficnetlink plugin translates `InterfaceQoS` to vishvananda/netlink tc calls (qdiscs, classes, filters)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → Component | YANG tree parsing, OnConfigure/OnConfigReload callbacks | [ ] |
| Component → Plugin | Backend Apply method call with data model structs | [ ] |
| Plugin → Kernel | google/nftables netlink (firewallnft), vishvananda/netlink RTNETLINK (trafficnetlink) | [ ] |

### Integration Points
- `internal/component/config/` — YANG tree parsing provides firewall and traffic-control subtrees
- `internal/component/iface/` — traffic-control references interface names managed by iface
- `internal/core/env/` — env var registration for config leaves
- `pkg/ze/` — EventBus for lifecycle events (OnStarted, OnConfigReload)

### Architectural Verification
- [ ] No bypassed layers (config → component → backend → kernel)
- [ ] No unintended coupling (firewall and traffic are independent components)
- [ ] No duplicated functionality (new capability, not recreating existing)
- [ ] Zero-copy preserved where applicable (config structs passed by reference)

## Backend Interfaces

Firewall backend:

| Method | Signature | Semantics |
|--------|-----------|-----------|
| Apply | `Apply(desired []Table) error` | Receive full desired state. Reconcile against kernel. Create/replace ze_* tables. Delete orphan ze_* tables. Never touch non-ze_* tables. |
| ListTables | `ListTables() ([]Table, error)` | Read current ze_* tables with chains, terms, sets from kernel. For CLI show. |
| GetCounters | `GetCounters(tableName string) ([]ChainCounters, error)` | Read packet/byte counter values per term. For CLI show counters. |

Traffic control backend:

| Method | Signature | Semantics |
|--------|-----------|-----------|
| Apply | `Apply(desired map[string]InterfaceQoS) error` | Receive full desired state keyed by interface name. Reconcile qdiscs, classes, filters. |
| ListQdiscs | `ListQdiscs(ifaceName string) (InterfaceQoS, error)` | Read current tc state for an interface. For CLI show. |

Both backends: component calls Apply on startup (OnStarted) and on config reload (OnConfigReload).
Same code path. Component registers as a plugin via `registry.Register()` in `init()` (same pattern
as iface). Backend registers via `RegisterBackend()` in the plugin's `init()`.

## Wiring Test (MANDATORY — NOT deferrable)

Umbrella spec delegates wiring tests to child specs. Each child has its own wiring table.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| firewall config in YANG | → | firewallnft Apply | spec-fw-2 wiring tests |
| traffic-control config in YANG | → | trafficnetlink Apply | spec-fw-3 wiring tests |
| `ze firewall show` CLI | → | firewall show handler | spec-fw-5 wiring tests |
| `ze traffic-control show` CLI | → | traffic show handler | spec-fw-5 wiring tests |

## Design Decisions

All decisions from `/ze-design` session 2026-04-13:

| # | Decision | Resolved | Rationale |
|---|----------|----------|-----------|
| 1 | Scope | Reconciler, boot-time tables. Ze owns `ze_*`, lachesis owns its own. | User requirement |
| 2 | Firewall placement | `internal/component/firewall/` + `internal/plugins/firewall/nft/` | Matches iface/ifacenetlink. Single responsibility. |
| 3 | Firewall backend | `Apply(desired []Table) error` + `ListTables() ([]Table, error)` + `GetCounters(table) ([]ChainCounters, error)` | Plugin owns reconciliation. Read methods for CLI. Matches iface Backend pattern. |
| 4 | Data model | Abstract firewall concepts. Term = Name + `[]Match` + `[]Action`. 42 types (18 match, 16 action, 8 modifier). | VPP backend shouldn't reverse-engineer nftables register chains. Complexity in nft backend (lowering) not every future backend. |
| 5a | Scope includes tc | Firewall (nftables) + traffic control (tc). | VoIP needs both sides. |
| 5b | Firewall config syntax | Hybrid: Junos structure (named terms, from/then) + ze naming + nftables concepts (table/chain/hook/set/flowtable). | Structural safety (from/then prevents mis-ordering), named rules for logging/counters, full nftables power. |
| 5c | tc config syntax | `traffic-control { interface X { qdisc htb { class name { rate, ceil, priority, match mark } } } }` | User approved. |
| 6 | Traffic control placement | `internal/component/traffic/` + `internal/plugins/traffic/netlink/` | Separate kernel subsystem, separate library. |
| 7 | Traffic backend | `Apply(desired map[string]InterfaceQoS) error` + `ListQdiscs(iface) (InterfaceQoS, error)` | Consistent with firewall. Read method for CLI. |
| 8 | Table ownership | All ze tables prefixed `ze_`. Plugin manages only `ze_*`, deletes orphans, never touches others. | Config truth + prefix safety. |
| 9 | Config reload | Apply on startup (OnStarted) + reload (OnConfigReload). Same code path. | Box must boot with firewall. |
| 10 | Expression coverage | 42 abstract types (19 match, 16 action, 7 modifier). No deferral. Hash/Numgen deferred. | Production workload. nft backend lowers to nftables expressions internally. |
| 11 | CLI commands | Full read-only CLI (show tables, counters, tc). Monitor. All mutations through config. | All required for production. |
| 12 | VPP compatibility | Same data model, same component, different plugin. Backend selected by config. Both can coexist. | VPP plan Phase 4 becomes firewallvpp/trafficvpp. |
| 13 | Backend read path | Expand interface: Apply + ListTables + GetCounters (firewall), Apply + ListQdiscs (traffic). | Matches iface Backend pattern (mixed read/write). |
| 14 | Component lifecycle | Plugin via registry.Register() in init(), same as iface. | Codebase dictates: no alternative pattern exists. |
| 15 | Expression model | Abstract firewall concepts, not nftables-native. | VPP backend maps concepts directly. nft backend lowers to register operations. |
| 16 | Expression inventory | 18 match + 16 action + 8 modifier = 42 types. Hash/Numgen deferred. | Covers full production workload. |
| 17 | Config syntax | Hybrid Junos/ze: named terms, from/then split, ze readable names, nftables concepts (table/chain/hook/set/flowtable). | Structural safety, named rules for logging/counters, prevents mis-ordering errors. |

## Acceptance Criteria

Umbrella AC — child specs have detailed per-feature AC.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | ze boots with firewall config | nftables tables named ze_* created in kernel |
| AC-2 | ze boots with traffic-control config | tc qdiscs/classes applied to named interfaces |
| AC-3 | ze config reload changes firewall | ze_* tables reconciled (create new, delete orphan, replace changed) |
| AC-4 | ze config reload changes traffic-control | tc qdiscs/classes reconciled on affected interfaces |
| AC-5 | lachesis creates its own nftables table | ze does not touch it during apply or reload |
| AC-6 | `ze firewall show` | displays current ze_* tables, chains, rules, sets, counters |
| AC-7 | `ze traffic-control show` | displays current qdiscs, classes, filters per configured interface |
| AC-8 | firewall config with all expression types | all nftables expressions programmed correctly in kernel |
| AC-9 | VPP backend configured | same YANG config, VPP ACLs/policers applied instead of nftables/tc |

## 🧪 TDD Test Plan

### Unit Tests
Delegated to child specs. Each child has its own test plan.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| Data model tests | spec-fw-1 | Expression types, Table/Chain/Set construction | |
| firewallnft reconciler tests | spec-fw-2 | Apply reconciliation logic | |
| trafficnetlink reconciler tests | spec-fw-3 | Apply reconciliation logic | |
| Config parsing tests | spec-fw-4 | YANG to data model conversion | |
| CLI output tests | spec-fw-5 | show/monitor command formatting | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Chain priority | -400 to 300 | 300 | N/A (nftables allows negative) | N/A (kernel clamps) |
| Table family | enum (inet, ip, ip6, arp, bridge, netdev) | netdev | invalid string | invalid string |
| Port number | 1-65535 | 65535 | 0 | 65536 |
| Rate limit value | 1+ | 1 | 0 | N/A (uint64 max) |
| HTB rate | 1+ bps | 1 | 0 | N/A |
| HTB ceil | >= rate | rate value | rate-1 | N/A |
| Mark value | 0x0-0xFFFFFFFF | 0xFFFFFFFF | N/A | N/A (uint32) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Firewall boot apply | `test/firewall/001-boot-apply.ci` | Config with firewall, ze starts, nft list shows ze_* tables | |
| Traffic boot apply | `test/traffic/001-boot-apply.ci` | Config with traffic-control, ze starts, tc show shows qdiscs | |
| Config reload | `test/firewall/002-reload.ci` | Change firewall config, reload, verify kernel state changed | |
| Lachesis coexistence | `test/firewall/003-coexistence.ci` | Non-ze_* table exists, ze reload does not touch it | |
| CLI firewall show | `test/firewall/004-cli-show.ci` | `ze firewall show` outputs table/chain/rule info | |
| CLI traffic show | `test/traffic/002-cli-show.ci` | `ze traffic-control show` outputs qdisc/class info | |

### Future (if deferring any tests)
- VPP backend tests deferred to spec-fw-6 and spec-fw-7 (depend on VPP Phase 0)

## Files to Modify

Umbrella creates no files directly. All files are in child specs.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | Yes | `internal/component/firewall/schema/ze-firewall-conf.yang` (spec-fw-4) |
| YANG schema (new RPCs) | Yes | `internal/component/traffic/schema/ze-traffic-control-conf.yang` (spec-fw-4) |
| CLI commands/flags | Yes | `internal/component/firewall/cmd/` (spec-fw-5) |
| CLI commands/flags | Yes | `internal/component/traffic/cmd/` (spec-fw-5) |
| Editor autocomplete | Yes | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | Yes | `test/plugin/` (each child spec) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — add firewall and traffic control |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` — add firewall and traffic-control sections |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` — add firewall/traffic-control commands |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` — add firewallnft, trafficnetlink |
| 6 | Has a user guide page? | Yes | `docs/guide/firewall.md`, `docs/guide/traffic-control.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` — add firewall/tc comparison vs VyOS/FRR |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` — add firewall/traffic components |

## Files to Create

All files delegated to child specs. Summary:

| Child spec | Files |
|------------|-------|
| fw-1 | `internal/component/firewall/model.go`, `internal/component/firewall/backend.go`, `internal/component/traffic/model.go`, `internal/component/traffic/backend.go` |
| fw-2 | `internal/plugins/firewall/nft/` (5-6 files) |
| fw-3 | `internal/plugins/traffic/netlink/` (4-5 files) |
| fw-4 | `internal/component/firewall/schema/`, `internal/component/traffic/schema/`, config parsers |
| fw-5 | `internal/component/firewall/cmd/`, `internal/component/traffic/cmd/` |
| fw-6 | `internal/plugins/firewallvpp/` (deferred) |
| fw-7 | `internal/plugins/trafficvpp/` (deferred) |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + relevant child spec |
| 2. Audit | Child spec Files to Modify/Create |
| 3. Implement (TDD) | Child spec Implementation phases |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Child spec Critical Review Checklist |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Child spec Deliverables Checklist |
| 10. Security review | Child spec Security Review Checklist |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Data model (fw-1)** — define all types
   - Tests: model_test.go for Table/Chain/Rule/Set construction and validation
   - Files: firewall/model.go, firewall/backend.go, traffic/model.go, traffic/backend.go
   - Verify: tests fail → implement → tests pass

2. **Phase: YANG config (fw-4)** — config parsing
   - Tests: config_test.go for YANG-to-model conversion
   - Files: schema/*.yang, config.go parsers
   - Verify: tests fail → implement → tests pass

3. **Phase: firewallnft backend (fw-2)** — kernel programming
   - Tests: reconciler unit tests with mock, functional .ci tests
   - Files: firewallnft/*.go
   - Verify: tests fail → implement → tests pass

4. **Phase: trafficnetlink backend (fw-3)** — tc programming
   - Tests: reconciler unit tests with mock, functional .ci tests
   - Files: trafficnetlink/*.go
   - Verify: tests fail → implement → tests pass

5. **Phase: CLI (fw-5)** — show, monitor commands
   - Tests: CLI output tests, functional .ci tests
   - Files: firewall/cmd/*.go, traffic/cmd/*.go
   - Verify: tests fail → implement → tests pass

6. **Functional tests** → Cover all AC from umbrella and children
7. **Full verification** → `make ze-verify`
8. **Complete spec** → Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N from umbrella and active children demonstrated |
| Correctness | ze_* prefix applied correctly, non-ze_* tables untouched |
| Naming | Config keywords use readable names (destination port, not dport) |
| Data flow | Config → component → backend → kernel, no shortcuts |
| Rule: no-layering | No VyOS compatibility shims, no translation layers |
| Rule: single-responsibility | Firewall and traffic components fully independent |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| firewall component exists | `ls internal/component/firewall/` |
| traffic component exists | `ls internal/component/traffic/` |
| firewallnft plugin exists | `ls internal/plugins/firewall/nft/` |
| trafficnetlink plugin exists | `ls internal/plugins/traffic/netlink/` |
| YANG modules register | `grep -r "yang.RegisterModule" internal/component/firewall/ internal/component/traffic/` |
| Backend registration works | `grep -r "RegisterBackend" internal/plugins/firewall/nft/ internal/plugins/traffic/netlink/` |
| ze_* tables in kernel after boot | functional .ci test |
| Non-ze_* tables untouched | functional .ci test |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | YANG config values validated: port ranges, IP addresses, interface names |
| Privilege | nftables and tc require CAP_NET_ADMIN; verify ze drops other capabilities |
| Table ownership | Plugin MUST only touch ze_* tables; verify prefix check cannot be bypassed |
| Set element injection | Named set elements from config must be validated before Apply |
| Mark values | Mark values are uint32; verify no truncation or overflow |
| Interface names | Traffic-control interface names must match existing interfaces |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Package Layout

| Package | Purpose | Library |
|---------|---------|---------|
| `internal/component/firewall/` | Data model, YANG config, parsing, dispatch | None (pure Go) |
| `internal/component/firewall/schema/` | `ze-firewall-conf.yang` | None |
| `internal/component/firewall/cmd/` | CLI commands | None |
| `internal/plugins/firewall/nft/` | nftables backend, reconciler | `github.com/google/nftables` |
| `internal/plugins/firewallvpp/` | VPP ACL/classifier backend | `go.fd.io/govpp` |
| `internal/component/traffic/` | Data model, YANG config, parsing, dispatch | None (pure Go) |
| `internal/component/traffic/schema/` | `ze-traffic-control-conf.yang` | None |
| `internal/component/traffic/cmd/` | CLI commands | None |
| `internal/plugins/traffic/netlink/` | tc backend, reconciler | `github.com/vishvananda/netlink` (vendored) |
| `internal/plugins/trafficvpp/` | VPP policer/scheduler backend | `go.fd.io/govpp` |

## Config Syntax Overview

Firewall (hybrid Junos structure + ze naming + nftables concepts):

```
firewall {
    table wan {
        family inet;
        chain input {
            type filter;
            hook input;
            priority 0;
            policy drop;
            term allow-established {
                from {
                    connection state established,related;
                }
                then {
                    counter allow-established;
                    accept;
                }
            }
            term allow-ssh {
                from {
                    destination port 22;
                }
                then {
                    accept;
                }
            }
            term voip-mark {
                from {
                    protocol udp;
                    destination port 5060-5061,16384-32767;
                }
                then {
                    mark set 0x10;
                    accept;
                }
            }
            term default-drop {
                then {
                    limit rate 10/second;
                    log prefix "INPUT-DROP: ";
                    counter default-drop;
                    reject with icmp admin-prohibited;
                }
            }
        }
        set blocked-hosts {
            type ipv4;
            flags interval;
        }
    }
}
```

Traffic control:

```
traffic-control {
    interface eth0 {
        qdisc htb {
            default-class bulk;
            class voip {
                rate 10mbit;
                ceil 100mbit;
                priority 0;
                match mark 0x10;
            }
            class interactive {
                rate 5mbit;
                ceil 100mbit;
                priority 1;
                match mark 0x20;
            }
            class bulk {
                rate 85mbit;
                ceil 100mbit;
                priority 2;
            }
        }
    }
}
```

Note: table name `wan` in config becomes `ze_wan` in the kernel. The `ze_` prefix is
added by the component, transparent to the user.

## VPP Compatibility

The abstract data model is the abstraction boundary. Components build `[]Table` or
`map[string]InterfaceQoS` from YANG config. Backends translate to their dataplane.

| Ze abstract type | nftables backend (lowering) | VPP backend |
|-----------------|---------------------------|-------------|
| MatchSourceAddress | Payload(NetworkHeader,12) + Bitwise + Cmp | AclRule.SrcPrefix |
| MatchDestinationAddress | Payload(NetworkHeader,16) + Bitwise + Cmp | AclRule.DstPrefix |
| MatchSourcePort | Meta(L4Proto) + Payload(TransportHeader,0) + Cmp | AclRule.SrcportFirst/Last |
| MatchDestinationPort | Meta(L4Proto) + Payload(TransportHeader,2) + Cmp | AclRule.DstportFirst/Last |
| MatchProtocol | Meta(L4Proto) + Cmp | AclRule.Proto |
| MatchConnState | Ct(state) + Bitwise + Cmp | VPP reflexive ACL |
| Accept | Verdict(NF_ACCEPT) | AclRule.IsPermit=1 |
| Drop | Verdict(NF_DROP) | AclRule.IsPermit=0 |
| Reject | Reject expression | AclRule.IsPermit=0 + punt |
| SNAT/DNAT | NAT expression | Nat44 API |
| SetMark | Immediate + Meta(MARK) | ClassifyAddDelSession |
| Limit | Limit expression | PolicerAddDel |
| Log | Log expression | TraceFilterApply |
| Counter | Counter expression | Per-ACL counters |
| FlowOffload | FlowOffload expression | Native (VPP IS the offload) |
| MatchInSet | Lookup expression | Classifier tables |
| MatchDSCP | Payload(NetworkHeader,TOS) + Bitwise + Cmp | PolicerAddDel conform/violate |

| Ze tc concept | Kernel backend | VPP backend |
|---------------|---------------|-------------|
| HTB qdisc | netlink.QdiscAdd(htb) | PolicerAddDel (token bucket) |
| Classes with rate/ceil | netlink.ClassAdd(htbClass) | Policer CIR/EIR |
| Priority scheduling | netlink.QdiscAdd(prio) | QosEgressMapUpdate |
| Mark-based classification | tc filter fw | ClassifyAddDelSession |
| DSCP rewriting | tc action skbedit | QosMarkEnableDisable |

## Lachesis Integration

Ze and lachesis coexist on the same box. Ze's firewall creates `ze_*` tables on boot.
Lachesis creates its own tables (e.g., `surfprotect`) via its orchestrate/cube-config
daemons. Neither touches the other's tables.

Lachesis nftables operations (from `orchestrate/core/nftables.go` and `cube-config/core/nftables.go`):
- Manipulates named set elements within its own tables (bypass rules)
- Creates/replaces its own tables on cube devices
- Reads current firewall state by enumerating chains and sets

All of this continues working unchanged because ze only touches `ze_*` tables.

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

- Lachesis's nftables library is hand-rolled raw netlink. google/nftables provides a superset
  with proper transaction semantics (Flush), base chain support, and all expression types.
- vishvananda/netlink already vendored with full tc support (HTB, HFSC, FQ, FQ_CoDel, etc.).
  No new dependency needed for traffic control.
- Abstract data model chosen over nftables-native: 42 types (18 match, 16 action, 8 modifier)
  model firewall concepts, not nftables register operations. The nft backend lowers abstract
  types to nftables expressions internally. The VPP backend maps them directly to ACL fields.
- Hybrid Junos/ze config syntax: named terms with from/then split provide structural safety
  (prevents mis-ordering) and named rules for logging/counters. Full nftables power via
  table/chain/hook/priority/policy/set/flowtable concepts.
- VPP compatibility falls out naturally from the abstract model. Both nft and VPP backends
  translate from the same concept types, no reverse-engineering of register chains needed.
- Limit moved from match to modifier (then block). Matches Junos policer pattern. Common
  use case is `then { limit rate ...; drop; }`, not `from { limit rate ...; }`.

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
- [ ] AC-1..AC-9 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-fw-0-umbrella.md`
- [ ] Summary included in commit
