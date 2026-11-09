# Spec: home-cpe-20-xfrm-cross-family-tunnel-state

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P1 |
| Release reason | A shipped capability does the wrong thing: a tunnel-mode Child SA whose negotiated selectors carry IPv6 over IPv4 endpoints (or IPv4 over IPv6) installs, reports up, and carries no inner traffic of the other family. RFC 4301 Section 5.1.2.1 allows the versions to differ. |
| RFC gaps | - |
| Depends | - |
| Phase | 2/2 |
| Handoff | - |
| Updated | 2026-11-09 |

Parent: plan/release/fixes/spec-home-cpe-0-umbrella.md

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

### Dependency and closure order (shared with child 6)

Child 6 needs this fix to carry its scenario, and this spec needs child 6's
scenario as its interop proof. The cycle is broken by separating three things
that one "depends" line used to carry:

| Item | Owner | Rule |
|------|-------|------|
| The `XFRM_STATE_AF_UNSPEC` fix and its red-first namespace proof (`TestXFRMTunnelCarriesTheOtherInnerFamily`) | this spec | Implemented and committed first. Needs nothing from child 6 |
| Interop scenario `vti-ipv6-inner-ipv4-outer` | child 6 (`spec-home-cpe-6-ipsec-ipv6-in-ipv4-vti.md`), sole owner | Child 6 creates it, runs it, and records its red walk with this spec's fix reverted. This spec creates and edits no file of it |
| Implementation prerequisite | child 6 | Child 6 needs this spec's fix COMMITTED in HEAD, not this spec closed |
| Closure order | both | 1. this spec implemented and committed; 2. child 6 implemented, its scenario green, its red walk recorded, child 6 closed; 3. this spec closed, citing child 6's recorded run as its interop evidence |

This spec cannot close before child 6: the fix puts inner traffic of the other
family on the wire as ESP, so it is a protocol change that owes an interop test
(`ai/rules/interop-and-goal-validation.md`), and the one scenario that carries it
is child 6's. The metadata `Depends` stays `-` because nothing in this spec's
implementation waits on another spec.

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
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` `xfrmStateFromParams` - refuses a mixed outer pair (RFC 4301 Section 4.4.2.1), builds `netlink.XfrmState` with Src, Dst, Proto, Mode, SPI, Reqid, Ifid; sets `state.Selector` only when `p.Sel` is non-nil. Its comment says so: "A tunnel-mode IKE Child SA leaves p.Sel nil, so msg.Sel stays the zero value."
- [ ] `vendor/github.com/vishvananda/netlink/xfrm_state_linux.go` - `xfrmUsersaInfoFromXfrmState` sets `msg.Family` from `state.Dst` and calls `selFromPolicy` only for a non-nil `Selector`; the only flag it ever sets is `XFRM_STATE_ESN`. `nl/xfrm_state_linux.go` defines `XFRM_STATE_AF_UNSPEC = 32` and nothing uses it.
- [ ] `vendor/github.com/vishvananda/netlink/xfrm_policy_linux.go` - `selFromPolicy` defaults the selector family to IPv4, so passing an empty selector does not produce AF_UNSPEC either; in `xfrmPolicyAddOrUpdate` the template family follows the template address length, which already gives an IPv6 policy an IPv4 template correctly.
- [ ] Linux 7.2.9 source as Ze builds it (`ze-kernel-build` docker volume, `linux-7.2.9-yes`), read through a read-only container on 2026-10-31:
  - `net/xfrm/xfrm_user.c` `copy_from_user_state`: "if (!x->sel.family && !(p->flags & XFRM_STATE_AF_UNSPEC)) x->sel.family = p->family;". With Ze's zero selector and no flag, the selector family becomes the outer family.
  - `net/xfrm/xfrm_state.c` `__xfrm_init_state`: when `x->sel.family != AF_UNSPEC` only `inner_mode` of that family is set; only the AF_UNSPEC branch also fills `inner_mode_iaf` with the other family's tunnel mode.
  - `net/xfrm/xfrm_state.c` `xfrm_state_look_at`: a VALID state is skipped when "x->sel.family && (x->sel.family != family || !xfrm_selector_match(...))". An IPv6 flow resolving a template to an IPv4-selector state finds none: the outbound half drops (expected counter `XfrmOutNoStates`).
  - `include/net/xfrm.h` `xfrm_ip2inner_mode`: returns `inner_mode_iaf` only when the selector family is AF_UNSPEC; otherwise the state's single inner mode handles every decapsulated packet. The inbound effect on an IPv6 inner packet is not yet measured.
- [ ] `internal/component/ike/engine/child.go` (`childPolicies`, `childPairPolicy`) - one policy per negotiated pair and direction, carrying the Child SA's if_id; an IPv6 pair over IPv4 endpoints already produces an IPv6 policy with an IPv4 template.

**Behavior to preserve:**
- Transport-mode states and OSPF RFC 4552 states keep their explicit selectors and get no AF_UNSPEC flag.
- The mixed-outer-pair refusal stays.
- Readback (`show vpn ipsec sa`, dataplane inspection) reports the outer family from the message family.
- A same-family tunnel (IPv4 in IPv4) keeps working; with AF_UNSPEC the kernel picks the inner mode by protocol (`xfrm_ip2inner_mode`), which serves it.

**Behavior to change:**
- A tunnel-mode state installed with no selector carries `XFRM_STATE_AF_UNSPEC`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| XFRM state builder | `internal/component/ike/dataplane/xfrm_linux.go` `xfrmStateFromParams` | Builds `netlink.XfrmState` from `SAParams`; sets `Selector` only when `p.Sel` is non-nil; refuses a mixed outer pair | extends: sets the AF_UNSPEC field for tunnel mode with nil `Sel`, leaves every other branch as it is |
| Vendored-module patch set | `internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch`, checked by `internal/le/vendorpatch/netlink_xfrm_patch_test.go` | Carries Ze's diff against the upstream netlink release, applied at build time | extends: the new `XfrmState` flag field goes into this patch, never a fork or a hand edit of `vendor/` |
| Child SA policy set | `internal/component/ike/engine/child.go` `childPolicies`, `childPairPolicy` | One policy per negotiated pair and direction; an IPv6 pair over IPv4 endpoints already gets an IPv6 policy with an IPv4 template | does not apply: the policy side is already correct, only the state is wrong |
| Shared XFRM seam for OSPF RFC 4552 | `internal/plugins/ospf/ipsec_install.go`, `ai/doors/access-security.md` "Where it sits" | OSPFv3 installs transport-mode states with an explicit selector through the same dataplane | does not apply: explicit-selector states get no flag (Behavior to preserve) |

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
| A-2 | Inbound IPv6 inner over an IPv4 SA is also mishandled without the flag | `xfrm_ip2inner_mode` returns the single inner mode | Only the outbound half is broken; the fix is the same | Linux v7.3 source (stable tree, read 2026-11-07): decapsulation succeeds for either inner family, because `xfrm_inner_mode_encap_remove` (`net/xfrm/xfrm_input.c`) picks `xfrm4_remove_tunnel_encap` or `xfrm6_remove_tunnel_encap` by the ESP next header, not by the state. The drop comes after: `__xfrm_policy_check` (`net/xfrm/xfrm_policy.c`) matches every secpath state with `xfrm_selector_match(&x->sel, &fl, family)`, which opens "if (family != sel->family && sel->family != AF_UNSPEC) return false;", so an IPv6 inner packet through an AF_INET-selector state counts `XfrmInStateMismatch`, not `XfrmInStateModeError`. On a cross-netns xfrm interface `xfrmi_rcv_cb` calls `xfrm_ip2inner_mode` only when the selector family is AF_UNSPEC, so the same mismatch follows. The red walk cannot observe it: the outbound half drops first | confirmed by source read (counter corrected to `XfrmInStateMismatch`); unmeasured |
| A-3 | strongSwan sets AF_UNSPEC on tunnel SAs | strongSwan 6.1.0 (the tag the lab image builds) `src/libcharon/plugins/kernel_netlink/kernel_netlink_ipsec.c` `add_sa`: for `MODE_TUNNEL` and `MODE_IPTFS` it ORs `XFRM_STATE_AF_UNSPEC` into the SA flags (read 2026-11-02) | Interop still proves Ze's half | Source read; child 6's scenario run shows the peer half carrying IPv6 | source-read |
| A-4 | Upstream `vishvananda/netlink` has no field for the flag at the vendored version | `XfrmState` struct in the vendored file | If a newer release has one, bump instead of patching | Read 2026-11-07 through the Go module proxy: the latest release is v1.3.1 (`@latest`, the version `go.mod` requires), and upstream `main` at `v1.3.2-0.20261102090517-2cd1a50c5846` still defines `XFRM_STATE_AF_UNSPEC = 32` in `nl/xfrm_state_linux.go` and sets no flag but `XFRM_STATE_ESN` in `xfrmUsersaInfoFromXfrmState` (`xfrm_state_linux.go`) | confirmed: no release or upstream commit carries it, so the vendor patch stays |

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
| `ipsec-show-dataplane-kernel` (existing) | `test/ipsec/ipsec-show-dataplane-kernel.ci` | The daemon path: a Child SA established through two `ze` daemons, whose responder installs a tunnel-mode state through the real XFRM backend (now flagged) and `show vpn ipsec dataplane sa` reads it back by its outer addresses (AC-7 through the user entry point, and no regression in install). The cross-family datapath itself needs two namespaces and is proven by the integration test above and child 6's interop scenario | owed: run in the QEMU guest on Ze's kernel (`option=needs-linux:caps=net-admin`) |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `vti-ipv6-inner-ipv4-outer` (owned and created by child 6; this spec's closure cites its recorded run, see "Dependency and closure order") | `test/interop-ipsec/scenarios/` | strongSwan 6.1.0 | IPv6 ping across an IPv4 SA; reverting this fix turns it red (child 6 AC-6) | |

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
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md`, as two separate statements. Dataplane: a tunnel-mode SA Ze installs carries either inner family over an outer pair of either family (RFC 4301 Section 5.1.2.1). Operator-facing IKE support is narrower: every IKE socket is `udp4` (`plan/journal/unwired-feature.md`, 2026-09-04 row, second finding), so a tunnel an operator can configure today has an IPv4 outer pair carrying IPv4 or IPv6 inner traffic (child 6). IPv4 inside an IPv6 outer pair is proven only by this spec's namespace test and no configuration reaches it until IKE runs over IPv6 |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4301.md` row for Section 5.1.2.1 if one exists |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` |
| 13 | Route metadata keys | No | |
| 14 | Prometheus counters | No | |
| 15 | Registered plugin/event changed? | No | |
| 16 | Changed source referenced by doc anchors? | Yes | derived by `./le spec citation anchors spec plan/release/fixes/spec-home-cpe-20-xfrm-cross-family-tunnel-state.md` |
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
| `kernel-refuses-what-the-installer-sends.md` | 2026-11-07 | `xfrmStateFromParams` installed tunnel states with no selector and no `XFRM_STATE_AF_UNSPEC` (row `fixed:`, citing 0db8414cfd) |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Set `XFRM_STATE_AF_UNSPEC` on tunnel states with no selector | Install the negotiated selector on the state | One Child SA may carry both families (RFC 7296 Section 2.9 narrowing to a v4 and a v6 pair); a state selector holds one family. The kernel's AF_UNSPEC branch is built for this |
| Carry the flag through the existing netlink vendor patch | Raw netlink message built in Ze | META: third-party changes are diffs against upstream; one encoder stays the only encoder |
| Prove red with a namespace datapath test | Trust the kernel source read | The source read names a mechanism; only a run shows the drop and the counter |

## Known Limitations
- The inbound half (A-2) is asserted by the same test but its exact kernel path is not traced here.
- IPv4 inside an IPv6 outer pair (AC-6) is a dataplane capability only: IKE has no IPv6 socket, so no configuration reaches it until IKE over IPv6 exists.

## RFC Documentation (Scope: protocol)

The builder line carries `// RFC 4301 Section 5.1.2.1: "IPsec allows the IP
version of the encapsulating header to be different from that of the inner
header."` above the flag.

## Recorded evidence

| Date | What ran | Where | Result |
|------|----------|-------|--------|
| 2026-11-07 | `TestXFRMTunnelCarriesTheOtherInnerFamily` (subtests `ipv6_inside_ipv4`, `ipv4_inside_ipv6`, `ipv4_inside_ipv4`), `TestXFRMTunnelStateIsInstalledAFUnspec`, `TestXFRMReadbackOfAFUnspecState`, with the fix of 0db8414cfd | `./le test qemu run kernel tmp/kernel/build/vmlinuz`, guest reporting Ze's runtime kernel 7.3 | PASS, all three (AC-2, AC-3, AC-5, AC-6, AC-7 at the dataplane) |
| 2026-11-07 | Interop scenario `vti-ipv6-inner-ipv4-outer` (child 6), after d0c2738a48 "ike: traffic into an xfrm interface passes the discard catch-all" and 3c23153b48 "interop-ipsec: vti IPv6 scenario offers MODP_2048 for its PFS rekey" | Ze's Docker host, `./le test integration interop-ipsec` | PASS, reported by the main thread. Its red walk with this fix reverted belongs to child 6 (AC-6 there) |
| 2026-11-07 | AC-1 red walk: `state.AFUnspec = true` in `xfrmStateFromParams` replaced by `_ = state // red walk` (`lx8.sh`, section `==RED20`), then `TestXFRMTunnelCarriesTheOtherInnerFamily -v`; file restored from its copy, `git diff --stat` empty, test rerun | QEMU guest on Ze's kernel, `./le job run label lx`; `tmp/session/2026-11-07-104251/scratch/qemu-lx8.log` lines 96-121 | RED: `ipv6_inside_ipv4` "side b received no request from fd00:2a::1 through the IPv4-outer SA: read udp [fd00:2a::2]:47020: i/o timeout; xfrm_stat moved: side a: XfrmOutNoStates +1; side b: no counter moved"; `ipv4_inside_ipv6` the same for 10.42.0.1 with `XfrmOutNoStates +1`; `ipv4_inside_ipv4` PASS (the control). Restored: `ok internal/component/ike/dataplane 1.030s` |
| 2026-11-08 | Child 6's red walk of `vti-ipv6-inner-ipv4-outer` with this fix reverted (overlay `rwinv/home-cpe-20-vti-af-unspec-interop/`, `state.AFUnspec = false`) | Ze's Docker host, `./le test integration interop-ipsec` | RED: "ze: the state 172.28.0.2 -> 172.28.0.3 on if_id 0x7 lacks af-unspec, so the kernel gives its selector the outer family and no IPv6 flow finds it" (`rwinv/home-cpe-20-vti-af-unspec-interop/vti-ipv6-inner-ipv4-outer.log`) |
| 2026-11-07 | `test/ipsec/ipsec-show-dataplane-kernel.ci` | QEMU guest, Ze 7.3; `tmp/session/2026-11-07-104251/scratch/rwinv/out-gb6/ipsec-clean.log` | PASS ("1/1 PASS 28 ipsec-show-dataplane-kernel") |
| 2026-11-09 | `go test ./internal/component/ike/dataplane/ ./internal/le/vendorpatch/` after the closure's comment fix; `GOOS=linux go vet ./internal/component/ike/dataplane/` | darwin host, `./le job run label c20-unit` / `c20-vet` | ok, ok; vet clean |

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
- [ ] The targeted tests for what changed pass: the scoped package tests for `internal/component/ike/dataplane` and `internal/le/vendorpatch` through `./le job run label unit-pkg command go test -tags ...`; the kernel tests through `./le job run label xfrm-cross-family command go test -tags integration -run 'TestXFRMTunnelCarriesTheOtherInnerFamily|TestXFRMTunnelStateIsInstalledAFUnspec|TestXFRMReadbackOfAFUnspecState' ./internal/component/ike/dataplane/` on Ze's kernel (each test enters its own user and network namespace through `userns.Enter`, as `xfrm_integration_linux_test.go` does); and `./le verify changed` over the closing commits
- [ ] Interop evidence cited, not produced here: child 6's recorded run of `IPSEC_INTEROP_SCENARIO=vti-ipv6-inner-ipv4-outer ./le test integration interop-ipsec`, green with this fix and red with it reverted (closure order step 3)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features: child 6's scenario, cited at closure

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A** and **Commit B** per `ai/rules/planning.md`

## Implementation Summary

### What Was Implemented
- 0db8414cfd: `netlink-xfrm-fixes.patch` adds `XfrmState.AFUnspec`; the vendored `xfrmUsersaInfoFromXfrmState` sets `XFRM_STATE_AF_UNSPEC` from it and the state decoder reports it back. `xfrmStateFromParams` (`internal/component/ike/dataplane/xfrm_linux.go`) sets it for a tunnel-mode state with no selector, under the RFC 4301 Section 5.1.2 quote.
- Tests: `TestXFRMStateFlagsForTunnelWithoutSelector` (unit), `TestXFRMTunnelStateIsInstalledAFUnspec`, `TestXFRMReadbackOfAFUnspecState`, `TestXFRMTunnelCarriesTheOtherInnerFamily` (kernel, own netns each).
- Closure: the `xfrmStateFromParams` doc comment said "Nothing below sets a flag", false since `AFUnspec`; corrected. That comment edit staled three audit verdicts that cite the function (RFC4301-4.2-1, RFC4301-4.4.2.1-3, RFC4303-3.4.4.1-2); each was re-judged (`./le rfc audit-stamp ... mode rejudge`) with its verdict unchanged, because no statement changed.

### Bugs Found/Fixed
- The defect this spec exists for, covered by `TestXFRMTunnelCarriesTheOtherInnerFamily` (red walk recorded above).
- Stale doc comment on `xfrmStateFromParams` (closure review, ISSUE 1).

### Documentation Updates
- `docs/guide/ipsec.md` "The inner family of a tunnel does not have to match its outer family" (0db8414cfd), with the dataplane and operator statements kept separate.
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` "A tunnel-mode state with no selector carries `XFRM_STATE_AF_UNSPEC`" and the vendor-patch list (0db8414cfd).
- `docs/architecture/linux-limitations.md` (0db8414cfd).
- `./le doc check all`: anchors resolve (3043 across 23 digests); the run is red on the pipe-operator availability check for `clear debug`, which this spec does not touch.

### Deviations from Plan
- Goal Gates named `./le verify changed`; no agent runs it (`ai/META-DECISIONS.md`). The scoped runs in Recorded evidence are the evidence.
- The red walk ran only `TestXFRMTunnelCarriesTheOtherInnerFamily`; `TestXFRMTunnelStateIsInstalledAFUnspec` was not observed red. It is not RFC-tagged and AC-1 names the datapath test, which the walk proves.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The implementation left the ECN doc comment of `xfrmStateFromParams` saying nothing sets a flag | `AFUnspec` sets `XFRM_STATE_AF_UNSPEC` | Closure review, comment check | Comment corrected; audit verdicts re-judged |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Prove the drop red before the fix | Done | `qemu-lx8.log` lines 96-121 | `XfrmOutNoStates +1` |
| Set `XFRM_STATE_AF_UNSPEC` on tunnel states with no selector | Done | `xfrm_linux.go` `xfrmStateFromParams`; vendored `xfrmUsersaInfoFromXfrmState` | via `netlink-xfrm-fixes.patch` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | red walk, Recorded evidence row 2026-11-07 | counter `XfrmOutNoStates`; red output in commit A body |
| AC-2 | Done | `TestXFRMTunnelCarriesTheOtherInnerFamily/ipv6_inside_ipv4` PASS (`qemu-lx6.log`, `qemu-kern2.log`) | |
| AC-3 | Done | `TestXFRMTunnelStateIsInstalledAFUnspec` PASS (`qemu-kern2.log` line 109) | |
| AC-4 | Done | `TestXFRMStateFlagsForTunnelWithoutSelector` (unit, ok 2026-11-09); explicit-selector rows of `TestXFRMTunnelStateIsInstalledAFUnspec` | |
| AC-5 | Done | `ipv4_inside_ipv4` PASS in both red and green runs | |
| AC-6 | Done | `ipv4_inside_ipv6` PASS (`qemu-lx6.log` line 61) | |
| AC-7 | Done | `TestXFRMReadbackOfAFUnspecState` PASS; `ipsec-show-dataplane-kernel.ci` PASS | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestXFRMStateFlagsForTunnelWithoutSelector` | Done | `xfrm_linux_test.go` | |
| `TestXFRMTunnelStateIsInstalledAFUnspec` | Done | `xfrm_integration_linux_test.go` | |
| `TestXFRMTunnelCarriesTheOtherInnerFamily` | Done | `xfrm_cross_family_integration_linux_test.go` | red-first |
| `TestXFRMReadbackOfAFUnspecState` | Done | `xfrm_readback_integration_linux_test.go` | |
| `ipsec-show-dataplane-kernel` | Done | `test/ipsec/ipsec-show-dataplane-kernel.ci` | PASS on Ze 7.3 |
| `vti-ipv6-inner-ipv4-outer` | Done (child 6) | `test/interop-ipsec/scenarios/` | green, and red with this fix reverted |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch` | Done | 0db8414cfd |
| `vendor/github.com/vishvananda/netlink/xfrm_state_linux.go` | Done | regenerated, 0db8414cfd |
| `internal/component/ike/dataplane/xfrm_linux.go` | Done | 0db8414cfd; comment fixed at closure |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` | Done | 0db8414cfd |
| `internal/component/ike/dataplane/xfrm_cross_family_integration_linux_test.go` | Done | created, 0db8414cfd |

### Audit Summary
- **Total items:** 20
- **Done:** 20
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Prove the defect red by a datapath test before the fix | functional (kernel netns) | `qemu-lx8.log`: `ipv6_inside_ipv4` and `ipv4_inside_ipv6` FAIL, `XfrmOutNoStates +1`, control PASS |
| IPv6 crosses an IPv4-outer tunnel SA Ze installs | interop | child 6 `vti-ipv6-inner-ipv4-outer` PASS against strongSwan 6.1.0; RED with the fix reverted ("lacks af-unspec") |
| The same test goes green with the fix | functional (kernel netns) | `qemu-lx6.log` / `qemu-kern2.log`: all three subtests PASS |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every AC is demonstrated | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/home-cpe-20-xfrm-cross-family-tunnel-state-7e292f27-ced0-427f-a7b6-3a931db211f9.md` (4 files, verdict clean) |
| `./le spec review check` | OK (1 code file, clean, hashes match) |
| Rounds | 2 |
| Reviewer lenses used | wiring (builder reached by `InstallSA`), logic of the flag rule (tunnel and nil selector only), vacuity of the red walk (control subtest green, counter named), comment drift, security (policy scope), style pass over the changed Go (no `panic()`, no new loop, no allocation) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Doc comment said "Nothing below sets a flag" after `AFUnspec` was added | `xfrm_linux.go` `xfrmStateFromParams` | Comment states that NOECN is never set and that ESN and AFUnspec are the struct's only flag fields |

Run 2: 0 BLOCKER, 0 ISSUE. NOTEs: the spec's prose cites RFC 4301 Section 5.1.2.1 for "IPsec allows the IP version...", which `rfc/full/rfc4301.txt` places in Section 5.1.2 (the code quotes 5.1.2 correctly); `TestXFRMTunnelStateIsInstalledAFUnspec` has no recorded red. Security: a state is reached only through an SPD template, and each policy is one negotiated pair bound to the Child SA's if_id (`childPolicies`), so AF_UNSPEC widens no path into the state.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/ike/dataplane/xfrm_cross_family_integration_linux_test.go` | Yes | 332 lines in 0db8414cfd |
| `test/ipsec/ipsec-show-dataplane-kernel.ci` | Yes | ran as test 28 in `ipsec-clean.log` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | red with the flag removed | `qemu-lx8.log` line 106: "XfrmOutNoStates +1" |
| AC-3, AC-4 | flag only for tunnel with nil selector | `xfrm_linux.go` line 222 `if p.Mode == ModeTunnel && p.Sel == nil`; unit test ok 2026-11-09 |
| AC-2, AC-5..AC-7 | kernel tests green | `qemu-kern2.log` lines 100-117 PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Child SA install through two `ze` daemons, readback by `show vpn ipsec dataplane sa` | `test/ipsec/ipsec-show-dataplane-kernel.ci` | PASS on Ze 7.3 |
| IPv6 through an IPv4 SA to strongSwan | interop `vti-ipv6-inner-ipv4-outer` | PASS, red with the fix reverted |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | red walk: `XfrmOutNoStates +1` on side a, nothing on side b |
| A-2 | confirmed (source read) | Linux v7.3 `__xfrm_policy_check`; the walk cannot observe the inbound half because the outbound drops first |
| A-3 | confirmed | child 6's scenario carries IPv6 both ways against strongSwan 6.1.0 |
| A-4 | confirmed | v1.3.1 and upstream main carry no setter; vendor patch stays |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/ipsec.md` inner family paragraph | `xfrmStateFromParams` sets `AFUnspec` for tunnel mode with nil `Sel` | Yes |
| `ipsec-8-ikev2-child-xfrm.md` AF_UNSPEC section | vendored `xfrmUsersaInfoFromXfrmState` line 721 | Yes |
| RFC status | `grep -i 'encapsulating header' rfc/short/rfc4301.md`: no row (a MAY-level "allows"); `./le rfc update` regenerated nothing new | Yes |
