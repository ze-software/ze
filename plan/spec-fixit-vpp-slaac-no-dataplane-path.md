# Spec: fixit-vpp-slaac-no-dataplane-path

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 2/3 |
| Deferral shard | - |
| Updated | 2026-08-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze accepts IPv6 stateless autoconfiguration on a VPP-owned interface and then
does nothing that makes it work.

The neighbouring clients are gated. The `dhcp`, `dhcpv6` and
`router-advertisement` containers each carry `ze:backend "netlink"` in
`internal/component/iface/yang/ze-iface-conf.yang`, so `ValidateBackendFeatures`
(`internal/component/config/backend_gate.go`) refuses them when the active
backend is `vpp`. Two tests pin that refusal:
`test/parse/iface-vpp-rejects-dhcp.ci` and
`test/parse/iface-vpp-rejects-router-advertisement.ci`.

The `autoconf` leaf and the `accept-ra` leaf sit in the same `ipv6` container and
carry no `ze:backend` annotation. The gate accepts them under `backend vpp`.
`applySysctl` (`internal/component/iface/config_sysctl.go`) then writes
`net.ipv6.conf.<os-name>.autoconf` and `net.ipv6.conf.<os-name>.accept_ra`, and
under the VPP backend that OS name is the Linux Control Plane tap.

Kernel autoconfiguration on that tap learns an address only if the dataplane
delivers Router Advertisements to it. Ze never asks for that delivery. No
feature-arc request exists anywhere in the tree, and the govpp `feature` binapi
module is not vendored under `vendor/go.fd.io/govpp/binapi/`.

Symptom: an operator enables IPv6 autoconfiguration on a VPP-owned NIC, the
commit succeeds, and no address is ever learned. The configuration and the
dataplane disagree, and nothing tells the operator.

VLAN sub-interfaces carry the same question one level down. `CreateVLAN`
(`internal/plugins/iface/vpp/ifacevpp.go`) sends `CreateVlanSubif` and the QoS
messages only. It creates no Linux Control Plane pair, so a sub-interface tap
exists only through `lcp-auto-subint`, which `GenerateStartupConf`
(`internal/component/vpp/startupconf.go`) writes into `startup.conf`.

**The premise must be settled before the fix is chosen.** The vendored Linux
Control Plane subset (`third_party/vpp-linux-cp/src/lcp_interface.c`) programs a
cross-connect input feature on the host interface, IPv4 and IPv6 punt redirect on
the physical interface, and the ARP nodes. It touches no multicast arc. That is
suggestive and not conclusive, because the vendored subset is management-only.
The evidence that settles it is a QEMU test: enable `autoconf` on a VPP-owned NIC
with an advertisement sender on the link, and record whether an address appears
on the tap.

Goal: make the configuration surface and the dataplane agree. Two answers are
possible, and the QEMU result decides which. Gate the two leaves to the netlink
backend, the way the DHCP clients and the advertisement sender are already gated,
or deliver the packets to the tap and prove it with the same test. A third
outcome is legitimate: if the test shows the tap already receives the
advertisements, close this spec and record the evidence.

**Decision (Thomas, 2026-08-24): gate the two leaves now, and measure later.**
The premise test needs `test/qemu/`, which does not exist, and R-1 says this spec
stalls while that harness is built. The gate lands on the reasoning that already
governs the three sibling containers, so the operator stops getting a silent
accept today. The measurement moves to `plan/spec-vpp-slaac-tap-premise.md`,
which carries AC-1 and can REVERSE this gate: if the tap turns out to receive the
advertisements, the annotation comes off and the two leaves are restored on the
vpp backend. The delivery branch of the goal above is not taken here.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `ai/digests/vpp-dataplane.md` - the current shape of the VPP subsystem, and
  which backend owns which surface
  → Constraint: `iface-vpp` implements the generic `iface.Backend`, so a
  backend-specific behavior belongs behind that interface, not in the shared
  `internal/component/iface` apply path
- [ ] `docs/research/vpp-deployment-reference.md` - Linux Control Plane pairing
  → Decision: the tap shadows a VPP interface so kernel networking can bind on a
  VPP-owned NIC

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4862.md` - what a host must receive before it can configure
  an address without a stateful server

**Key insights:** (minimal context to resume after compaction)
- The gate is a YANG annotation read by one walker, so adding a backend
  restriction is a schema edit, not a code path.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `dhcp`, `dhcpv6` and
  `router-advertisement` carry `ze:backend "netlink"`; `autoconf` and
  `accept-ra` carry no annotation
- [ ] `internal/component/config/backend_gate.go` - `ValidateBackendFeatures`
  walks the tree and refuses a node whose annotation excludes the active backend
- [ ] `internal/component/iface/config_sysctl.go` - `applySysctl` emits
  `net.ipv6.conf.<os-name>.autoconf` and `.accept_ra` for any unit
  → Constraint: the symbol is `applySysctl`, and its `emit` closure writes both
  keys. An earlier draft of this spec named `applyUnitSysctls`, which does not
  exist anywhere in the tree
- [ ] `internal/component/iface/config_apply.go` - the ethernet unit loop calls
  the sysctl step for every unit, with no backend test
- [ ] `internal/plugins/iface/vpp/ifacevpp.go` - `CreateVLAN` sends
  `CreateVlanSubif` and no pairing message
- [ ] `internal/component/vpp/startupconf.go` - `GenerateStartupConf` writes
  `lcp-auto-subint`

**Behavior to preserve:**
- The refusal of `dhcp`, `dhcpv6` and `router-advertisement` under the vpp
  backend, and the two `.ci` tests that pin it.
- Autoconfiguration under the netlink backend, which is unaffected.
- The sysctl emission path for every other leaf it carries.

**Behavior to change:**
- Only the outcome of enabling `autoconf` or `accept-ra` while the active backend
  is `vpp`. Whether that outcome is a refusal or a working address depends on the
  premise test named in the Task.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config leaf `interface ethernet <name> unit <n> ipv6 autoconf true`, with
  `interface backend vpp` set, read at commit time.
- The same path carries `accept-ra`, a `uint8` in the range 0 to 2.

### Transformation Path
1. The config tree reaches `ValidateBackendFeatures`
   (`internal/component/config/backend_gate.go`), which reads the `ze:backend`
   annotation of each node and refuses the ones the active backend excludes.
   Neither leaf carries an annotation today, so both pass.
2. `applyConfig` (`internal/component/iface/config_apply.go`) walks the units and
   calls the sysctl step with the resolved OS device name.
3. `applySysctl` (`internal/component/iface/config_sysctl.go`) emits the two
   `net.ipv6.conf.<os-name>` keys on the EventBus.
4. The sysctl plugin writes them on the tap, and the kernel waits for Router
   Advertisements that no dataplane path delivers.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ backend gate | YANG `ze:backend` annotation read by one walker | Read, and run: the leaf annotation reaches the refusal |
| iface component ↔ sysctl plugin | EventBus key/value emission | Read. Unchanged by this spec |
| Kernel tap ↔ VPP dataplane | Linux Control Plane pair, punt paths programmed by the plugin | Unverified. This is the premise `plan/spec-vpp-slaac-tap-premise.md` owns |

### Integration Points
- `ValidateBackendFeatures` - the only place a leaf is restricted to a backend.
- `iface.Backend` (`internal/component/iface/backend.go`) - where a dataplane
  delivery step would belong if the fix is delivery rather than refusal.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The verdict sits in `ValidateBackendFeatures`, the one place a node is restricted to a backend. `applySysctl` is untouched, so the shared emission step learns nothing about backends |
| No unintended coupling (components stay isolated) | Yes | The change is a YANG annotation. `internal/component/config` names no backend and no iface leaf; it reads whatever the schema carries |
| No duplicated functionality (extends existing, does not recreate) | Yes | The same annotation and the same walker the `dhcp`, `dhcpv6` and `router-advertisement` containers already use |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No wire encoding and no buffer path is touched |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No new field, switch case, or factory. `backendAnnotation` already had the `*LeafNode` case; this is its first user |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The Linux Control Plane pair alone does not deliver Router Advertisements to the tap | `third_party/vpp-linux-cp/src/lcp_interface.c` programs cross-connect, punt redirect and ARP only | The gate is too wide and comes off again | QEMU test with an advertisement sender on the link | unvalidated, owned by `plan/spec-vpp-slaac-tap-premise.md` |
| A-2 | No dataplane request for packet delivery exists in ze today | tree search for a feature-arc message; the govpp `feature` module is not vendored | The fix is smaller than stated | grep plus `vendor/go.fd.io/govpp/binapi/` listing | confirmed: no `vendor/go.fd.io/govpp/binapi/feature/`, and no feature-arc call anywhere in the tree |
| A-3 | Gating the two leaves to netlink breaks no working deployment | the DHCP clients and the advertisement sender are already gated the same way | An operator loses a working configuration on upgrade | search deployments and release notes | confirmed: no config in the tree sets either leaf with `backend vpp` |
| A-4 | The gate reads a `ze:backend` annotation on a LEAF, not only on a container | `backendAnnotation` (`internal/component/config/backend_gate.go`) carries a `*LeafNode` case, and `yangToLeaf` (`internal/component/config/yang_schema.go`) sets `LeafNode.Backend` | The whole fix is vacuous: an annotation nobody reads | `ze config validate` against a vpp config that sets `autoconf`, before and after the annotation | confirmed: refused with `/interface/ethernet/eth0/unit/0/ipv6/autoconf: feature not supported by backend "vpp" (supported: netlink)`, accepted without the annotation |
| A-5 | The `accept-ra` YANG default of 0 does not materialize into the tree the gate walks | `ApplyDefaults` (`internal/component/config/schema_defaults.go`) has one non-test caller, `applyPeerSchemaDefaults` (`internal/component/bgp/config/peers.go`) | Every VPP unit with an `ipv6` container is refused for a leaf nobody set | `iface-vpp-accepts-ipv6-address.ci` and `TestBackendGateIPv6AcceptRADefaultIsNotMaterialized` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The premise test is expensive to build and the spec stalls on it | no QEMU harness reaches a VPP-owned NIC with a link partner | reuse the existing QEMU appliance harness rather than a new one |
| R-2 | Gating removes a leaf an operator already sets, and the upgrade fails the commit | a config in the field carries `autoconf` under `backend vpp` | name the interface and the backend in the refusal, and say which backend supports it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A gate that is too wide refuses a working configuration at commit. A gate that is too narrow leaves the silent failure in place. |
| How is it reverted? | Single commit revert. No config migration, unless the fix gates a leaf that a stored config already carries. |
| Who else touches this path? | `plan/future/spec-fixit-vpp-vlan-promiscuous.md` works the same VLAN surface; `plan/spec-dataplane-seams-4-control-packet-rx.md` owns the shared receive-path question. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ipv6 autoconf true` with `backend vpp` | → | `ValidateBackendFeatures` (`internal/component/config/backend_gate.go`) | `test/parse/iface-vpp-rejects-ipv6-autoconf.ci` |
| `ipv6 accept-ra 2` with `backend vpp` | → | the same decision point | `TestBackendGateIPv6Autoconf` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A QEMU run with `autoconf true` on a VPP-owned NIC and an advertisement sender on the link | The test records whether an address is learned, and that record is the spec's premise evidence. **Moved to `plan/spec-vpp-slaac-tap-premise.md` by the 2026-08-24 decision: `test/qemu/` does not exist, and R-1 says this spec stalls behind it** |
| AC-2 | `ipv6 autoconf true` under `backend vpp` | The commit is refused with a message naming the unit and the supporting backend. Silent acceptance with no address is gone |
| AC-3 | `ipv6 accept-ra` under `backend vpp` | The same outcome as AC-2, because the two leaves describe one behavior |
| AC-4 | The same configuration on a VLAN sub-interface of a VPP-owned NIC | The outcome matches AC-2, so the parent and the sub-interface never disagree |
| AC-5 | `ipv6 autoconf true` under `backend netlink` | Unchanged: accepted, and the address is learned as before |
| AC-6 | An `ipv6` container under `backend vpp` that names neither leaf | Accepted. The gate covers the two leaves and does not widen to their container or to the `accept-ra` default |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | enables IPv6 autoconfiguration on a VPP-owned NIC | config tree → backend gate → refusal naming the unit and the netlink backend | `iface-vpp-rejects-ipv6-autoconf.ci` |
| 2 | enables it on a VLAN sub-interface of that NIC | same path with the sub-interface unit key | `iface-vpp-rejects-ipv6-autoconf-vlan.ci` |
| 3 | enables it on a netlink-owned NIC | config tree → backend gate → accepted, address learned as before | `iface-netlink-accepts-ipv6-autoconf.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBackendGateIPv6Autoconf` | `internal/component/config/backend_gate_test.go` | the gate's verdict for `autoconf` and `accept-ra` under each backend | pass |
| `TestBackendGateIPv6AcceptRADefaultIsNotMaterialized` | `internal/component/config/backend_gate_test.go` | the `accept-ra` YANG default of 0 does not reach the gate, so an `ipv6` container that names neither leaf is accepted under `backend vpp` | pass |

`applySysctl` is unchanged, so the sysctl emission needs no new test: the gate
refuses the config before the vpp path can reach that call.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `accept-ra` | 0-2 | 2 | N/A | 3 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-vpp-rejects-ipv6-autoconf.ci` | `test/parse/` | the operator learns at commit time that autoconfiguration needs the netlink backend | pass |
| `iface-vpp-rejects-ipv6-accept-ra.ci` | `test/parse/` | the receiving half answers the same way | pass |
| `iface-vpp-rejects-ipv6-autoconf-vlan.ci` | `test/parse/` | a VLAN sub-interface answers the same way as its parent | pass |
| `iface-vpp-accepts-ipv6-address.ci` | `test/parse/` | a static IPv6 address on a VPP-owned NIC still commits | pass |
| `iface-netlink-accepts-ipv6-autoconf.ci` | `test/parse/` | the netlink operator sees no change | pass |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `vpp-slaac-tap` | `test/qemu/` | an advertisement sender on the link | whether the dataplane delivers advertisements to the tap | moved to `plan/spec-vpp-slaac-tap-premise.md` |

The scenario keeps its name and loses the numeric prefix an earlier draft gave it
(`ai/rules/interop-and-goal-validation.md`, and the de-numbering in `9aaa03e3d`).
`test/qemu/` does not exist, so the scenario is the whole subject of the
follow-up spec rather than a row this one can fill.

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - `ze:backend "netlink"` on
  the `autoconf` and `accept-ra` leaves
- `internal/component/config/backend_gate_test.go` - `TestBackendGateIPv6Autoconf`
  and `TestBackendGateIPv6AcceptRADefaultIsNotMaterialized`
- `docs/features/interfaces.md` - the feature rows for the two leaves, and the
  section that states why they need the netlink backend

The delivery branch is not taken, so `internal/plugins/iface/vpp/ifacevpp.go` and
`internal/component/iface/config_sysctl.go` are unchanged. `applySysctl` keeps
emitting both keys for every unit it is called for; the gate now refuses the
config before that call can happen on the vpp backend.

## Files to Create
- `test/parse/iface-vpp-rejects-ipv6-autoconf.ci` - the commit-time refusal on
  the vpp backend
- `test/parse/iface-vpp-rejects-ipv6-accept-ra.ci` - the same refusal for the
  receiving half of the behavior
- `test/parse/iface-vpp-rejects-ipv6-autoconf-vlan.ci` - a VLAN sub-interface
  answers the way its parent does
- `test/parse/iface-vpp-accepts-ipv6-address.ci` - the gate stays on the two
  leaves and does not widen to the `ipv6` container
- `test/parse/iface-netlink-accepts-ipv6-autoconf.ci` - the netlink backend is
  unchanged

The names follow the sibling convention (`iface-vpp-rejects-dhcp.ci`,
`iface-vpp-rejects-router-advertisement.ci`) rather than the placeholder names an
earlier draft carried.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/iface/yang/ze-iface-conf.yang`, annotation only |
| YANG validation constraints | No | the leaves already carry their types and ranges |
| YANG custom validators | No | the backend gate is the validator |
| CLI commands/flags | No | no new command |
| CLI grammar (keyword before value) | N-A | no new grammar |
| Editor autocomplete | No | no new leaf |
| Functional test for new RPC/API | No | no new RPC |
| Pipe completeness | N-A | no new output |
| Env var registration | No | no environment leaf |
| Doctor check for runtime dependencies | No | the gate refuses the config at commit, so no running system can reach the disagreeing state. A doctor check would report a state the commit path no longer produces |
| Prometheus counters/metrics | No | no new observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | a restriction on two existing leaves, not a feature |
| 2 | Config syntax changed? | No | the syntax is unchanged. `docs/guide/configuration.md` documents no `autoconf` leaf table and shows no `autoconf` example; its two hits are RA-sender prose |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | no plugin is touched |
| 6 | Has a user guide page? | Yes | `docs/features/interfaces.md`, new section "Receiving advertisements needs the netlink backend", and the two feature rows |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 4862 governs the host that receives the advertisements. Ze changes which backend offers that host function, and implements no new protocol behavior |
| 10 | Test infrastructure changed? | No | the five new tests are `.ci` fixtures in the existing `test/parse/` suite. The QEMU scenario is `plan/spec-vpp-slaac-tap-premise.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | `ai/digests/vpp-dataplane.md` states nothing about `autoconf`, `accept-ra` or SLAAC, and the data flow it describes is unchanged |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `internal/component/iface/yang/ze-iface-conf.yang` and `internal/component/config/backend_gate.go` are both anchored from `docs/features/interfaces.md`, which this spec updates |
| 17 | Existing docs show config/CLI/API examples for this area? | No | every `autoconf` and `accept-ra` mention across `docs/` was read. All of them describe the RA sender or the kernel installer, and none shows either leaf with `backend vpp` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - prove the gate reads a LEAF
   - Tests: `TestBackendGateIPv6Autoconf`, and `ze config validate` against a
     vpp tree that sets `autoconf`
   - Files: `internal/component/config/backend_gate_test.go`
   - Verify: the annotation on a leaf reaches the refusal. The three siblings
     already gated are containers, so a walker that read containers only would
     make the whole change vacuous. Verified: `backendAnnotation` carries a
     `*LeafNode` case, `yangToLeaf` populates `LeafNode.Backend`, and the
     refusal names the leaf path
2. **Phase: Decision** - gate
   - Tests: `TestBackendGateIPv6Autoconf`
   - Files: `internal/component/iface/yang/ze-iface-conf.yang`
   - Verify: the unit test and the `.ci` fixtures fail first, then pass
3. **Phase: Parent and sub-interface agree**
   - Tests: `iface-vpp-rejects-ipv6-autoconf-vlan.ci`
   - Files: none. A VLAN sub-interface is a `unit` entry that carries a
     `vlan-id`, in the same `unit` list, so it reaches the same annotated leaf
   - Verify: the sub-interface verdict matches the parent's

The premise phase this spec first planned is now
`plan/spec-vpp-slaac-tap-premise.md`. Its result can reverse phase 2.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Correctness | The refusal message names the interface, the leaf, and the backend that supports it |
| Naming | The annotation spelling matches the neighbouring `dhcp` and `dhcpv6` containers exactly |
| Data flow | The decision sits in the gate or the backend, never in the shared sysctl step |
| Rule: `ai/rules/evidence.md` | The premise is settled by a run, not by reading the vendored subset |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The premise answer | not delivered here. `plan/spec-vpp-slaac-tap-premise.md` carries it, and its outcome can reverse this gate |
| Config and dataplane agree | the five `.ci` fixtures pass with the annotation and three of them fail without it, run on the same tree |
| The gate reads a leaf annotation | `TestBackendGateIPv6Autoconf` fails when the annotation is removed |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `accept-ra` stays bounded to 0..2 whichever answer lands |
| Fail closed | An unknown backend must not silently accept a leaf no dataplane serves |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Gate the two leaves to netlink now | deliver the advertisements to the tap; close with no change | Owner ruling, 2026-08-24. The delivery branch and the close branch both need `test/qemu/`, which does not exist. The gate lands on the reasoning that already governs the three sibling containers, and it removes the silent accept today |
| Annotate the two LEAVES, not their `ipv6` container | annotate `ipv6` | The container also holds `address`, `forwarding` and `rpf-check`, which the vpp backend serves. Annotating it would refuse every VPP unit that configures an IPv6 address |
| Do not make `applySysctl` backend-aware | pass the backend into the emission step | The gate refuses the config before the commit reaches the emission step, so a second verdict there would have no input the first did not already reject. It would also teach a shared step about backends |
| Measure the premise in a separate spec | keep this spec open until `test/qemu/` exists | R-1 named the stall. A spec that waits for a harness nobody has started is a spec that never lands its own fix |

## RFC Documentation (Scope: protocol)

The change implements no new protocol behavior: it decides whether a host
function is offered on a backend that can carry it. Where the answer is delivery,
the code that arranges it carries a comment naming the RFC 4862 requirement the
host cannot meet without the packets.

## Known Limitations
- The spec does not touch the DHCP clients or the advertisement sender. Their
  refusal under the vpp backend is settled and tested.
- **The premise behind the gate is not measured.** The vendored Linux Control
  Plane subset programs cross-connect, punt redirect and the ARP nodes and
  touches no multicast arc, which is suggestive and not conclusive: that subset
  is management-only. `plan/spec-vpp-slaac-tap-premise.md` carries the
  measurement, and a result showing the tap does receive the advertisements
  reverses this gate.
- An operator who set `autoconf` or `accept-ra` with `backend vpp` sees the
  commit refused after upgrade. That configuration learned no address before the
  upgrade, so nothing that worked stops working, and the message names the unit
  and the backend that supports the leaf.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
