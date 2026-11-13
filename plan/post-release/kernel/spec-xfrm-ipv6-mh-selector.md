# Spec: xfrm-ipv6-mh-selector

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | Missing exact IPv6 Mobility Header selector capability requires Linux receive-flow extraction work. The release repair in `plan/release/fixes/spec-rfc4301-architecture-gaps.md` must refuse ineffective constrained MH configuration and negotiation; this future feature remains owed under the owner's release-timing rule. |
| RFC gaps | RFC4301-4.4.1.1-1 |
| Depends | `plan/release/fixes/spec-rfc4301-architecture-gaps.md` -- owns exact-or-refuse validation and the observed shipped-kernel baseline |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make exact IPv6 Mobility Header Type selectors work for received, forwarded
and locally generated traffic on the Linux kernel Ze runs. Lift Ze's
constrained-MH refusal only when the running dataplane can enforce the
negotiated value. This skeleton records wanted work; its design is not approved
and no kernel patch is selected or implemented by the release repair.

RFC 4301 Section 4.4.1.1: "The following selector parameters MUST be supported
by all IPsec implementations to facilitate control of SA granularity."
The section includes an IPv6 Mobility Header message Type selector and says:
"For IKE, the IPv6 Mobility Header message type (MH type) is placed in the most
significant eight bits of the 16-bit local \"port\" selector."
RFC 7296 Section 3.13.1 specifies the zero low octet. Valid encoding alone does
not prove that the packet classifier reads the Type.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/access-security.md`, `ai/doors/appliance.md`, `ai/doors/le-tooling.md` -- subsystem boundaries and kernel ownership
  → Constraint: extend the existing XFRM backend and kernel build/proof route; create no second policy implementation
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` -- Child SA selectors and exact-or-refuse installation
  → Constraint: selectors accepted in negotiation must be enforced on both receive directions before the SA is reported installed
- [ ] `docs/architecture/testing/qemu-integration.md` -- privileged tests on the actual runtime kernel
  → Constraint: prove incoming and forwarded packets; a local raw6 send alone is insufficient
- [ ] `docs/architecture/mpls/mpls-kernel.md` -- existing kernel patch and runtime capability proof pattern
  → Constraint: the future capability must be detected on the running kernel, including an unpatched host
- [ ] `docs/guide/ipsec.md` -- operator selector values and backend refusals

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4301.md` -- the selector requirement remains a gap, not an exclusion
- [ ] `rfc/short/rfc7296.md` -- MH high-octet encoding and negotiation

## Current Behavior (MANDATORY)

**Source and runtime evidence read on 2026-11-12:**
- [ ] Linux7.3 `include/net/xfrm.h::xfrm_flowi_sport` reads the bare `uli->mht.type`; its destination-port helper returns zero for MH.
- [ ] Linux7.3 `net/xfrm/xfrm_policy.c::decode_session6` clears the flow, copies ports and handles GRE and ICMPv6 specially, but never fills the MH Type.
- [ ] Linux `include/linux/in.h::proto_ports_offset` and `net/core/flow_dissector.c::skb_flow_get_ports` do not provide MH Type extraction through the ordinary port path.
- [ ] Linux `net/ipv6/raw.c::rawv6_probe_proto_opt` fills `fl6_mh_type` for a local raw send. That output-only path does not establish receive/forward support.
- [ ] `internal/component/ike/ipsec/traffic_selector.go::ValidForProtocol` retains the RFC's high-octet encoding independently of backend programmability.
- [ ] `internal/component/ike/engine/child.go::childPairPolicy` converts that IKE value to a bare Type at policy projection. Projection alone did not fix the receive-flow decoder.
- [ ] `internal/component/ike/engine/mh_selector_boundary_integration_linux_test.go::TestMHChildPoliciesSelectIncomingIPv6Packets` supplied the original failing real Child/backend Type probe; after the release repair its active contract retains all ANY packet controls and requires exact producer refusal before kernel mutation.

The lead ran the probe on Ze's cached
`7.3-runtime-amd64-runtime-docker-a9caebdd-e68264fe` kernel. Its config has
`CONFIG_IPV6_MIP6` unset. ANY Type selectors delivered Type5 and Type7 on both
local IN and routed FWD paths. The policy projected as exact Type5 dropped
matching Type5 on both paths, incrementing `XfrmInPolBlock`; Type7 also
dropped. A narrow operator OUT BYPASS to the protected LAN isolated this
from the independent unmatched-discard forwarding-default defect.

The kernel source copies used for that investigation are named in the
RFC4301 phase state, not treated as a second source tree to modify or test.
Re-read the kernel version Ze ships when designing this feature. Investigate
decoder/extraction changes; do not assume that enabling `CONFIG_IPV6_MIP6`
alone makes the missing receive-flow field appear.

**Behavior to preserve:** ANY MH traffic remains usable. Kernels and VPP
backends without exact Type classification keep the release refusal. Other
protocol selectors and fragment restrictions keep their enforcement.

**Behavior to change:** on a dataplane proven to classify MH Type exactly,
configured and negotiated exact types become usable for IN, FWD and OUT.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| One port/protocol meaning | `internal/component/ike/ipsec/traffic_selector.go`, `ValidForProtocol`, `ProgrammableForProtocol` | Separates valid IKE representation from exact classifier support | Extends capability-dependent acceptance; preserves high-octet wire meaning |
| One Child policy builder | `internal/component/ike/engine/child.go`, `childPairPolicy` | Projects negotiated values into the installed selector | Extends and proves the same builder, never an alternate installer |
| Runtime kernel patch series | `gokrazy/kernel/patches/series`, `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require` | Carries Ze's kernel changes and required built-in configuration | Extends if the approved design needs a kernel patch and capability marker |
| Native guest proof route | `docs/architecture/testing/qemu-integration.md` | Boots a named runtime kernel and runs root namespace tests | Uses this route on patched and unpatched kernels |

## Data Flow (MANDATORY)

### Entry Point
The existing peer traffic-selector config and IKE Traffic Selector payloads.

### Transformation Path
Typed port/protocol meaning, negotiation, Child policy/state projection,
netlink installation, Linux receive-flow extraction and exact Type match.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| IKE value to kernel selector | High-octet Type to bare Type once | Projection exists; useful receive behavior is not implemented |
| Kernel packet to selector flow | Linux decoder reads the received MH Type | Missing on the observed Ze7.3 baseline |

### Changed-Wire Census
Design must determine whether any kernel capability signal changes. The IKE
MH Type representation remains RFC7296's existing high-octet format. Enumerate
all relevant readers/writers before advancing this skeleton to design.

## Risks & Assumptions
| ID | Assumption | Basis | If wrong | Validation | Status |
|----|------------|-------|----------|------------|--------|
| A-1 | A future kernel extraction change can provide the exact Type on IN and FWD without widening non-initial fragments | Existing field/getter, missing observed decoder assignment | Choose another exact mechanism; keep refusal until proven | Patched/unpatched incoming and forward packet tests | unvalidated |

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | A raw-output test passes while incoming traffic remains broken | IN/FWD matching-type control drops | Require all directions on the same running kernel |
| R-2 | A feature probe accepts a kernel that still reads every MH packet as Type0 | Marker present but matching/different Type behavior is identical | Behavioral capability proof, including unpatched negative control |
| R-3 | Missing fragment Type becomes exact Type0 or ANY | A non-initial fragment passes a constrained selector without negotiation | Preserve RFC4301 Section7.3 fragment restrictions and prove them |

## Wiring Test
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Configured exact MH Type with a known capable kernel | → | Existing config/negotiation/Child backend chain | Name and design a functional carrier before implementation |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Exact Type5 on a proven capable kernel | Type5 delivers and Type7 is rejected on local IN and routed FWD, through the actual installed Child | ANY delivers both Types; no incoming type is inferred from local raw output |
| AC-2 | Unpatched or incapable backend | Exact Type config and negotiation are refused before any SA/policy mutation | ANY MH remains usable and no degraded exact-Type Child is published |
| AC-3 | Valid IKE Type5 encoding | Wire value1280 becomes bare kernel Type5 once; responses retain the original wire value | Defined transport and ICMP encodings remain unchanged |
| AC-4 | Constrained MH plus non-initial fragments without negotiation | The fragments are rejected with observed packet/counter evidence | ANY fragment behavior retains its RFC-defined capability |

## 🧪 TDD Test Plan
| Carrier | Required proof | State |
|---------|----------------|-------|
| Real kernel namespace unit | ANY, exact matching Type, mismatching Type, local and forward, patched versus unpatched kernel | Baseline packet evidence exists; future test contract/design owed |
| Config/negotiation units | Capability-gated acceptance, incapable-kernel refusal before mutation, wire value preserved | Design owed; retain release refusal controls |
| Functional and peer scenario | Actual operator config through negotiation to packet delivery with an independent implementation | Design and source research owed; no MH interop claim from current builder tests |

## Files to Modify
- `gokrazy/kernel/patches/series` and the approved kernel patch -- receive-flow extraction, if the design chooses a kernel patch
- `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require` -- any proven runtime capability requirements
- `internal/component/ike/ipsec/traffic_selector.go` -- capability-gated programmable selector acceptance
- `internal/component/ike/engine/child.go` -- preserve the one installation path and exact values
- `docs/guide/ipsec.md`, `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, `rfc/short/rfc4301.md` -- feature scope and evidence

## Integration and Documentation Checklists
This skeleton does not declare an approved capability probe, doctor code,
kernel marker, functional test name or interop peer. Complete the template's
full Integration and Documentation Update checklists during design, using the
existing kernel-capability enrollment and test-carrier mechanisms. No new
config grammar or alternate protocol registry is authorized here.

### Critical Review Checklist
| Check | What to verify |
|-------|----------------|
| Direction completeness | Actual incoming and forwarded packets match by Type; OUT-only raw6 extraction is insufficient |
| Backend honesty | An incapable running kernel/VPP backend refuses before mutation; enabling a config symbol is not a behavioral verdict |
| Scope preservation | RFC4301-4.4.1.1-1 stays a gap until every selector component it lists is proven; this feature alone cannot announce full RFC support |

## Known Limitations
This owns the absent exact MH capability. The release correctness repair stays
owned by `plan/release/fixes/spec-rfc4301-architecture-gaps.md`; recording this
skeleton does not complete that repair or close its remaining validation.
