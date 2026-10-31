# Spec: home-cpe-20-xfrm-cross-family-tunnel-state

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | A shipped capability does the wrong thing: a tunnel-mode Child SA whose negotiated selectors carry IPv6 over IPv4 endpoints (or IPv4 over IPv6) installs, reports up, and carries no inner traffic of the other family. RFC 4301 Section 5.1.2.1 allows the versions to differ. |
| RFC gaps | - |
| Depends | - |
| Phase | 0/2 |
| Handoff | - |
| Updated | 2026-10-31 |

Parent: plan/home-cpe/spec-home-cpe-0-umbrella.md

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze installs every tunnel-mode IKE Child SA state with no selector and without
the `XFRM_STATE_AF_UNSPEC` flag. The Linux kernel then sets the state's selector
family to the OUTER family, and an SA so installed serves only inner packets of
that family. A route-based tunnel carrying IPv6 inside an IPv4 SA (the home CPE
management tunnel, child 6) therefore establishes, installs its IPv6 policies on
the VTI, and drops every IPv6 packet.

The umbrella recorded this as suspected. This spec settles the mechanism from the
kernel source Ze builds (read 2026-10-31, see Current Behavior), and still
requires the defect to be proven red by a test before the fix: a datapath test
that sends IPv6 through an IPv4-outer tunnel SA installed by Ze's dataplane, and
observes the drop. Then Ze sets `XFRM_STATE_AF_UNSPEC` on tunnel-mode states it
installs with no selector, and the same test goes green.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/access-security.md` - the IKE door
  → Constraint: `ike/dataplane` stays ungated because OSPF's RFC 4552 installer
    (`internal/plugins/ospf/ipsec_install.go`) uses the same seam. A flag rule here
    must not change a transport-mode OSPF state, which carries an explicit selector.
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - Child SA creation and the dataplane abstraction
  → Decision: the engine hands `dataplane.SAParams` to the backend; the netlink
    message shape is the backend's concern. The fix lives in the backend, not the engine.
- [ ] `docs/architecture/ike/ipsec-dataplane-inspection.md` - readback of SAD and SPD
  → Constraint: readback decodes addresses with the message family
    (`plan/journal/key-omits-a-fact-the-builder-uses.md`, 2026-09-10). A state with
    selector family 0 must still be reported with its outer family, never inferred.
- [ ] `docs/contributing/patching-the-standard-library.md` and `internal/le/vendorpatch/` - how a third-party diff is applied
  → Constraint: a change Ze needs in a third-party Go module is a diff against the
    upstream release, applied at build time (`ai/META-DECISIONS.md`). The vendored
    `vishvananda/netlink` already carries `internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch`.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4301.md` - present
  → Constraint: RFC 4301 Section 5.1.2.1: "IPsec allows the IP version of the
    encapsulating header to be different from that of the inner header." Note (1)
    of the same section: "The IP version in the encapsulating header can be
    different from the value in the inner header."
  → Constraint: RFC 4301 Section 4.4.2.1: "Tunnel header IP source and destination
    address -- both addresses must be either IPv4 or IPv6 addresses." The outer
    pair stays single-family (`xfrm_linux.go` already refuses a mixed pair); only the
    inner family may differ.
- [ ] `rfc/short/rfc7296.md` - present
  → Constraint: RFC 7296 Section 2.9: "When the responder chooses a subset of the
    traffic proposed by the initiator, it narrows the Traffic Selectors to some
    subset of the initiator's proposal (provided the set does not become the null
    set)." A single Child SA can legitimately carry selectors of both families, so
    a per-family state selector cannot describe it.

**Key insights:**
- The kernel decision is in `copy_from_user_state`, the inner-mode choice in `__xfrm_init_state`, and the outbound refusal in `xfrm_state_look_at` (all quoted below).
- `XfrmState` in the vendored netlink has no field that reaches `xfrm_usersa_info.flags` except ESN; the flag needs the vendor patch.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` (state build, about lines 150-215) - refuses a mixed outer pair (RFC 4301 Section 4.4.2.1), builds `netlink.XfrmState` with Src, Dst, Proto, Mode, SPI, Reqid, Ifid; sets `state.Selector` only when `p.Sel` is non-nil. Its comment says so: "A tunnel-mode IKE Child SA leaves p.Sel nil, so msg.Sel stays the zero value."
- [ ] `vendor/github.com/vishvananda/netlink/xfrm_state_linux.go` - `xfrmUsersaInfoFromXfrmState` sets `msg.Family` from `state.Dst` and calls `selFromPolicy` only for a non-nil `Selector`; the only flag it ever sets is `XFRM_STATE_ESN` (about line 282). `nl/xfrm_state_linux.go` defines `XFRM_STATE_AF_UNSPEC = 32` and nothing uses it.
- [ ] `vendor/github.com/vishvananda/netlink/xfrm_policy_linux.go` - `selFromPolicy` defaults the selector family to IPv4 (about line 108), so passing an empty selector does not produce AF_UNSPEC either; the template family follows the template address length (about line 181), which already gives an IPv6 policy an IPv4 template correctly.
- [ ] Linux 7.2.9 source as Ze builds it (`ze-kernel-build` docker volume, `linux-7.2.9-yes`), read through a read-only container on 2026-10-31:
  - `net/xfrm/xfrm_user.c` `copy_from_user_state` (line 829): "if (!x->sel.family && !(p->flags & XFRM_STATE_AF_UNSPEC)) x->sel.family = p->family;" (line 842). With Ze's zero selector and no flag, the selector family becomes the outer family.
  - `net/xfrm/xfrm_state.c` `__xfrm_init_state` (line 3215): when `x->sel.family != AF_UNSPEC` only `inner_mode` of that family is set; only the AF_UNSPEC branch also fills `inner_mode_iaf` with the other family's tunnel mode.
  - `net/xfrm/xfrm_state.c` `xfrm_state_look_at` (line 1313): a VALID state is skipped when "x->sel.family && (x->sel.family != family || !xfrm_selector_match(...))". An IPv6 flow resolving a template to an IPv4-selector state finds none: the outbound half drops (expected counter `XfrmOutNoStates`).
  - `include/net/xfrm.h` `xfrm_ip2inner_mode`: returns `inner_mode_iaf` only when the selector family is AF_UNSPEC; otherwise the state's single inner mode handles every decapsulated packet. The inbound effect on an IPv6 inner packet is not yet measured.
- [ ] `internal/component/ike/engine/child.go` (`childPolicies`, `childPairPolicy`) - one policy per negotiated pair and direction, carrying the Child SA's if_id; an IPv6 pair over IPv4 endpoints already produces an IPv6 policy with an IPv4 template.

**Behavior to preserve:**
- Transport-mode states and OSPF RFC 4552 states keep their explicit selectors and get no AF_UNSPEC flag.
- The mixed-outer-pair refusal stays.
- Readback (`show vpn ipsec sa`, dataplane inspection) reports the outer family from the message family.
- A same-family tunnel (IPv4 in IPv4) keeps working; with AF_UNSPEC the kernel picks the inner mode by protocol (`xfrm_ip2inner_mode`), which serves it.

**Behavior to change:**
- A tunnel-mode state installed with no selector carries `XFRM_STATE_AF_UNSPEC`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A Child SA established by the engine (`createFirstChildSA`, child rekey) calls the dataplane's state install with `SAParams` (Mode tunnel, Sel nil).

### Transformation Path
1. Engine builds `dataplane.SAParams` (`create_child.go`, `child.go`).
2. `xfrm_linux.go` maps it to `netlink.XfrmState`.
3. Vendored netlink serialises `xfrm_usersa_info`, now with the AF_UNSPEC flag for this case.
4. Kernel `copy_from_user_state` leaves `sel.family` 0; `__xfrm_init_state` fills both inner modes.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze ↔ kernel | netlink XFRM_MSG_NEWSA, `xfrm_usersa_info.flags` | No |
| Ze build ↔ upstream module | vendor patch over `vishvananda/netlink` | No |

### Integration Points
- `netlink-xfrm-fixes.patch` - gains a field on `XfrmState` that sets the flag.
- `xfrm_linux.go` state builder - sets that field for tunnel mode with nil selector.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Change confined to the dataplane backend and the vendor patch |
| No unintended coupling (components stay isolated) | Yes | Engine unchanged |
| No duplicated functionality (extends existing, does not recreate) | Yes | Extends the existing netlink patch |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Control path |
| Registration over hardcoding, outbound | N-A | No new feature registered |
| Registration over hardcoding, inbound | N-A | No list learns a name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Without the flag, outbound IPv6 through an IPv4-outer SA finds no state | `xfrm_state_look_at` and `copy_from_user_state` in Linux 7.2.9, read 2026-10-31 | The defect lies elsewhere (policy, template, if_id); the red test shows where | `TestXFRMTunnelCarriesTheOtherInnerFamily` red before the fix, with the counter that moved | source-read, unmeasured |
| A-2 | Inbound IPv6 inner over an IPv4 SA is also mishandled without the flag | `xfrm_ip2inner_mode` returns the single inner mode | Only the outbound half is broken; the fix is the same | Same test, inbound direction, counter `XfrmInStateModeError` or delivery | unvalidated |
| A-3 | strongSwan sets AF_UNSPEC on tunnel SAs | Umbrella survey; strongSwan source not present locally | Interop still proves Ze's half | Read `kernel_netlink_ipsec.c` `add_sa` in the strongSwan 6.1.0 tarball the lab image builds from | unvalidated |
| A-4 | Upstream `vishvananda/netlink` has no field for the flag at the vendored version | `XfrmState` struct in the vendored file | If a newer release has one, bump instead of patching | Read upstream's latest `xfrm_state_linux.go` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | AF_UNSPEC changes behaviour for a same-family tunnel | Existing xfrm integration tests and IPsec interop scenarios go red | The kernel selects the inner mode by protocol; run the existing integration tests under the same guest |
| R-2 | Readback starts inferring a family from selector bytes | Dataplane inspection test reports IPv4 for an IPv6 outer state | Keep decoding on `msg.Family`; add a readback assertion for a flagged state |
| R-3 | The red test is vacuous (sends nothing, or the counter never moves) | Counter delta zero in both runs | Assert delivery at the far namespace AND the counter, and record the red run |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every tunnel-mode IKE Child SA: inner traffic dropped |
| How is it reverted? | Single commit revert (patch hunk plus builder line) |
| Who else touches this path? | OSPF RFC 4552 installer (transport mode, unaffected by rule), `spec-ipsec-vti-bind-installs-if-id-zero.md` (same builder, if_id) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `XFRMDataplane` state install for a tunnel-mode SA with nil selector | → | `xfrm_linux.go` state builder, vendored `xfrmUsersaInfoFromXfrmState` | `TestXFRMTunnelStateIsInstalledAFUnspec` (readback of the installed state's flags) |
| An IPv6 packet routed into an IPv4-outer tunnel SA pair installed through the dataplane | → | kernel datapath | `TestXFRMTunnelCarriesTheOtherInnerFamily` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Before the fix: two network namespaces, IPv4 outer endpoints, a tunnel-mode ESP SA pair and ::/0 policies installed through Ze's dataplane; ping6 across | Test fails: no echo reply, and the named xfrm counter advances. The red run is recorded in the commit body |
| AC-2 | After the fix: same setup | IPv6 echo request and reply both cross; the counter does not advance |
| AC-3 | Tunnel-mode state with nil selector | Readback shows `XFRM_STATE_AF_UNSPEC` set and selector family 0 |
| AC-4 | Transport-mode state, or any state with explicit `Sel` | No AF_UNSPEC flag; selector as given |
| AC-5 | IPv4 in IPv4 tunnel with the flag | IPv4 ping crosses (no regression) |
| AC-6 | IPv4 in IPv6 outer (mirror case) | IPv4 ping crosses |
| AC-7 | `show vpn ipsec sa` / dataplane readback of a flagged state | Outer family reported from the message family |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures a VTI peer whose selectors include ::/0 over an IPv4 WAN, pings an IPv6 address across | IKE Child SA → dataplane state install → kernel → vti | child 6 interop scenario `vti-ipv6-inner-ipv4-outer` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXFRMStateFlagsForTunnelWithoutSelector` | `internal/component/ike/dataplane/xfrm_linux_test.go` | builder sets the flag field for tunnel mode with nil Sel, not for transport or explicit Sel | |
| `TestXFRMTunnelStateIsInstalledAFUnspec` | `internal/component/ike/dataplane/xfrm_integration_linux_test.go` | kernel readback of flags and selector family | |
| `TestXFRMTunnelCarriesTheOtherInnerFamily` | `internal/component/ike/dataplane/xfrm_cross_family_integration_linux_test.go` | IPv6 in IPv4 and IPv4 in IPv6 cross; red before the fix with the counter named | |
| `TestXFRMReadbackOfAFUnspecState` | `internal/component/ike/dataplane/xfrm_readback_integration_linux_test.go` | outer family from message family | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no numeric input | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | the datapath needs a kernel; proven by the integration test above and by child 6's interop scenario | | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `vti-ipv6-inner-ipv4-outer` (owned by child 6) | `test/interop-ipsec/scenarios/` | strongSwan 6.1.0 | IPv6 ping across an IPv4 SA; reverting this fix turns it red | |

## Files to Modify
- `internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch` - add an `XfrmState` field that sets `XFRM_STATE_AF_UNSPEC` in `xfrmUsersaInfoFromXfrmState`, and report it on readback
- `vendor/github.com/vishvananda/netlink/xfrm_state_linux.go` - regenerated by applying the patch, not hand-edited
- `internal/component/ike/dataplane/xfrm_linux.go` - set the field for tunnel mode with nil `Sel`; replace the comment that says the zero selector is enough
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - state the flag and why

## Files to Create
- `internal/component/ike/dataplane/xfrm_cross_family_integration_linux_test.go` - the red-first datapath test

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no config |
| YANG validation constraints | No | |
| YANG custom validators | No | |
| CLI commands/flags | No | |
| CLI grammar (keyword before value) | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | No | |
| Pipe completeness | No | |
| Env var registration | No | |
| Doctor check for runtime dependencies | No | no new dependency; xfrm already required |
| Prometheus counters/metrics | No | |
| BGP family surface | N-A | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md`: state that a tunnel carries either inner family over either outer family |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4301.md` row for Section 5.1.2.1 if one exists |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` |
| 13 | Route metadata keys | No | |
| 14 | Prometheus counters | No | |
| 15 | Registered plugin/event changed? | No | |
| 16 | Changed source referenced by doc anchors? | Yes | derived by `./le spec citation anchors spec plan/home-cpe/spec-home-cpe-20-xfrm-cross-family-tunnel-state.md` |
| 17 | Existing examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring and red proof (MANDATORY FIRST)**
   - Tests: `TestXFRMTunnelCarriesTheOtherInnerFamily`, `TestXFRMTunnelStateIsInstalledAFUnspec`
   - Files: the new integration test only
   - Verify: run in the Linux guest on Ze's kernel; the test is red, and the run names the counter that moved (`XfrmOutNoStates` expected). Record the output. If it is green, A-1 is broken: stop and report, the child 6 symptom has another cause
2. **Phase: Flag**
   - Tests: all of the TDD plan
   - Files: vendor patch, regenerated vendor file, `xfrm_linux.go`, docs
   - Verify: red tests turn green; existing `xfrm_*_integration_linux_test.go` and OSPF IPsec integration tests stay green

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | Flag only for tunnel mode with nil selector |
| Data flow | No engine change; backend only |
| Rule: vendor diff | Vendor file regenerated from the patch, not edited |
| Rule: red first | Red run recorded before the fix lands |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Patch hunk | grep `XFRM_STATE_AF_UNSPEC` in `netlink-xfrm-fixes.patch` |
| Red-then-green test | recorded output in the commit body |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Policy scope | AF_UNSPEC widens which inner family a state can carry; the SPD policies (one per negotiated pair, if_id bound) still decide what enters. Confirm no policy-less path reaches the state |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Red test is green before the fix | Stop; A-1 broken; report to child 6 |
| Test fails on behavior mismatch | Re-read the kernel source in Current Behavior |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

The zero selector looked like "any" and was in fact "any of the outer family".
The kernel's own name for "any family" is a flag, not a selector value.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| - | - | No row exists for this defect (grep of `plan/journal/` for AF_UNSPEC, cross-family and inner family on 2026-10-31 found none). Implementation writes one row in `kernel-refuses-what-the-installer-sends.md` when the red run is recorded, and lists it here |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Set `XFRM_STATE_AF_UNSPEC` on tunnel states with no selector | Install the negotiated selector on the state | One Child SA may carry both families (RFC 7296 Section 2.9 narrowing to a v4 and a v6 pair); a state selector holds one family. The kernel's AF_UNSPEC branch is built for this |
| Carry the flag through the existing netlink vendor patch | Raw netlink message built in Ze | META: third-party changes are diffs against upstream; one encoder stays the only encoder |
| Prove red with a namespace datapath test | Trust the kernel source read | The source read names a mechanism; only a run shows the drop and the counter |

## Known Limitations
- The inbound half (A-2) is asserted by the same test but its exact kernel path is not traced here.
- strongSwan's behaviour (A-3) is unverified locally.

## RFC Documentation (Scope: protocol)

The builder line carries `// RFC 4301 Section 5.1.2.1: "IPsec allows the IP
version of the encapsulating header to be different from that of the inner
header."` above the flag.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features (child 6 scenario)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A** and **Commit B** per `ai/rules/planning.md`
