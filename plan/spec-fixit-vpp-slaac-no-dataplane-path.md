# Spec: fixit-vpp-slaac-no-dataplane-path

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Updated | 2026-08-18 |

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
`applyUnitSysctls` (`internal/component/iface/config_sysctl.go`) then writes
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
- [ ] `internal/component/iface/config_sysctl.go` - `applyUnitSysctls` emits
  `net.ipv6.conf.<os-name>.autoconf` and `.accept_ra` for any unit
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
3. `applyUnitSysctls` (`internal/component/iface/config_sysctl.go`) emits the two
   `net.ipv6.conf.<os-name>` keys on the EventBus.
4. The sysctl plugin writes them on the tap, and the kernel waits for Router
   Advertisements that no dataplane path delivers.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ backend gate | YANG `ze:backend` annotation read by one walker | Read |
| iface component ↔ sysctl plugin | EventBus key/value emission | Read |
| Kernel tap ↔ VPP dataplane | Linux Control Plane pair, punt paths programmed by the plugin | Unverified |

### Integration Points
- `ValidateBackendFeatures` - the only place a leaf is restricted to a backend.
- `iface.Backend` (`internal/component/iface/backend.go`) - where a dataplane
  delivery step would belong if the fix is delivery rather than refusal.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The Linux Control Plane pair alone does not deliver Router Advertisements to the tap | `third_party/vpp-linux-cp/src/lcp_interface.c` programs cross-connect, punt redirect and ARP only | The spec closes with no code change and records the evidence | QEMU test with an advertisement sender on the link | unvalidated |
| A-2 | No dataplane request for packet delivery exists in ze today | tree search for a feature-arc message; the govpp `feature` module is not vendored | The fix is smaller than stated | grep plus `vendor/go.fd.io/govpp/binapi/` listing | unvalidated |
| A-3 | Gating the two leaves to netlink breaks no working deployment | the DHCP clients and the advertisement sender are already gated the same way | An operator loses a working configuration on upgrade | search deployments and release notes | unvalidated |

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
| `ipv6 autoconf true` with `backend vpp` | → | `ValidateBackendFeatures` or the chosen delivery step | `iface-vpp-autoconf-agrees-with-dataplane.ci` |
| `ipv6 accept-ra 2` with `backend vpp` | → | the same decision point | `TestBackendGateIPv6Autoconf` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A QEMU run with `autoconf true` on a VPP-owned NIC and an advertisement sender on the link | The test records whether an address is learned, and that record is the spec's premise evidence |
| AC-2 | `ipv6 autoconf true` under `backend vpp` | Either the commit is refused with a message naming the interface and the supporting backend, or an address is learned and the QEMU test proves it. Silent acceptance with no address is gone |
| AC-3 | `ipv6 accept-ra` under `backend vpp` | The same outcome as AC-2, because the two leaves describe one behavior |
| AC-4 | The same configuration on a VLAN sub-interface of a VPP-owned NIC | The outcome matches AC-2, so the parent and the sub-interface never disagree |
| AC-5 | `ipv6 autoconf true` under `backend netlink` | Unchanged: accepted, and the address is learned as before |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | enables IPv6 autoconfiguration on a VPP-owned NIC | config tree → backend gate → sysctl or dataplane delivery | `iface-vpp-autoconf-agrees-with-dataplane.ci` |
| 2 | enables it on a VLAN sub-interface of that NIC | same path with the sub-interface OS name | `iface-vpp-autoconf-vlan.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBackendGateIPv6Autoconf` | `internal/component/config/backend_gate_test.go` | the gate's verdict for `autoconf` and `accept-ra` under each backend | |
| `TestApplyUnitSysctlsIPv6Autoconf` | `internal/component/iface/config_sysctl_test.go` | which keys the sysctl step emits, and for which OS name | | <!-- doc-links: ignore (artifact this spec will create; it forbids code until a test proves its premise) -->

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `accept-ra` | 0-2 | 2 | N/A | 3 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-vpp-autoconf-agrees-with-dataplane.ci` | `test/parse/` or `test/plugin/` | the operator learns at commit time whether autoconfiguration works on this backend | |
| `iface-vpp-autoconf-vlan.ci` | `test/parse/` | a VLAN sub-interface answers the same way as its parent | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-vpp-slaac-tap` | `test/qemu/` | an advertisement sender on the link | whether the dataplane delivers advertisements to the tap | | <!-- doc-links: ignore (artifact this spec will create; it forbids code until a test proves its premise) -->

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - the annotation on
  `autoconf` and `accept-ra`, if the answer is a gate
- `internal/plugins/iface/vpp/ifacevpp.go` - the delivery step, if the answer is
  delivery
- `internal/component/iface/config_sysctl.go` - only if the emission must become
  backend-aware

## Files to Create
- `test/parse/iface-vpp-autoconf-agrees-with-dataplane.ci` - the commit-time <!-- doc-links: ignore (artifact this spec will create; it forbids code until a test proves its premise) -->
  behavior
- `test/parse/iface-vpp-autoconf-vlan.ci` - the sub-interface behavior <!-- doc-links: ignore (artifact this spec will create; it forbids code until a test proves its premise) -->

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
| Doctor check for runtime dependencies | | decide during design: a check could report the disagreement on a running system |
| Prometheus counters/metrics | No | no new observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` if a leaf becomes backend-restricted |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | | |
| 6 | Has a user guide page? | | `docs/guide/interfaces.md` | <!-- doc-links: ignore (artifact this spec will create; it forbids code until a test proves its premise) -->
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | | |
| 10 | Test infrastructure changed? | | `docs/functional-tests.md` if the QEMU scenario is new |
| 11 | Affects daemon comparison? | | |
| 12 | Internal architecture changed? | | `ai/digests/vpp-dataplane.md` |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | | |
| 16 | Any changed source file referenced by existing doc source anchors? | | grep `docs/` for the changed files |
| 17 | Existing docs show config/CLI/API examples for this area? | | verify every `autoconf` example against the new verdict |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - settle the premise
   - Tests: the QEMU scenario named in the Interop table
   - Files: the QEMU harness
   - Verify: the run records whether an address is learned. This answer selects
     the fix, and no code is written before it
2. **Phase: Decision** - gate or deliver
   - Tests: `TestBackendGateIPv6Autoconf`
   - Files: the YANG annotation or the VPP backend
   - Verify: the unit test fails first, then passes
3. **Phase: Parent and sub-interface agree**
   - Tests: `iface-vpp-autoconf-vlan.ci`
   - Files: whichever path Phase 2 chose
   - Verify: the sub-interface verdict matches the parent's

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
| The premise answer, recorded in this spec | the QEMU log quoted in the spec |
| Config and dataplane agree | the `.ci` test passes and fails when the change is reverted |

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

## RFC Documentation (Scope: protocol)

The change implements no new protocol behavior: it decides whether a host
function is offered on a backend that can carry it. Where the answer is delivery,
the code that arranges it carries a comment naming the RFC 4862 requirement the
host cannot meet without the packets.

## Known Limitations
- The spec does not touch the DHCP clients or the advertisement sender. Their
  refusal under the vpp backend is settled and tested.

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
