# Spec: ipsec-spd-final-discard-vpp-ospf

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P3 |
| Release reason | RFC 4301 Section 4.4.1 asks every SPD for a final DISCARD entry. Ze installs one in each XFRM SPD the IKE engine owns, and in no other SPD: the VPP SPDs and the kernel SPD that holds the OSPFv3 RFC 4552 manual-key entries carry none. The requirement is a SHOULD of an RFC Ze implements, and the VPP IPsec backend is itself post-release (`plan/post-release/features/spec-ipsec-vpp-policy-interface.md`), so the work is owed after the first release. |
| RFC gaps | RFC4301-4.4.1-3, RFC4301-5.2-1, RFC4301-5.2-9 |
| Depends | spec-ipsec-spd-selection, closed (it built the XFRM node-wide and per-interface final entries this spec extends; their design is `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`); `plan/post-release/features/spec-ipsec-vpp-policy-interface.md` (owns the VPP interface index an SPD binds to) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 4301 Section 4.4.1: "Every SPD SHOULD have a nominal, final entry that
matches anything that is otherwise unmatched, and discards it." Section 5.2
restates it: "Every SPD SHOULD have a nominal, final entry that catches
anything that is otherwise unmatched, and discards it. This ensures that
non-IPsec-protected traffic that arrives and does not match any SPD-I entry
will be discarded."

An independent RFC audit found the row `RFC4301-4.4.1-3` met only for the XFRM
SPDs the IKE engine installs: the node-wide SPD at if_id 0 and the SPD of each
XFRM interface a peer binds with `vti bind` (commit 78f09955aa, "every xfrm
interface SPD gets its own DISCARD catch-all"). The row now carries
`{partial: tested "a nominal, final entry ..."; gap "Every SPD"; ...}` in
`rfc/short/rfc4301.md`. Two kinds of SPD remain without a final entry:

1. **The VPP SPDs.** The VPP backend creates one SPD and binds it to each
   interface a policy names. No final entry is ever installed in it, and
   `unmatched discard` is refused at verify on VPP, so a VPP deployment runs
   only under `unmatched bypass`.
2. **The kernel SPD holding the OSPFv3 RFC 4552 manual-key entries.** The OSPF
   plugin installs its per-interface protocol 89 transport entries through
   whichever dataplane is loaded, with no final entry beneath them. They are in
   force with no `vpn ipsec` block and after the IKE engine releases its SPD.
   On VPP the OSPF entries are refused today (the `SPParams.IfIndex` comment in
   `dataplane.go` names the three refusals), so this SPD is the kernel's.

The goal: every SPD Ze installs entries into carries a final entry that matches
anything otherwise unmatched and discards it, through the mechanism that
already does this for the XFRM SPDs, and the row becomes whole (both polarities
on the whole sentence, the `{partial}` removed). The same work closes the two
paths the `{gap}` on `RFC4301-5.2-1` and `RFC4301-5.2-9` names.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - the node-wide catch-all, the per-interface catch-all and their install and release order
- [ ] `docs/architecture/ike/ipsec-3-data-model.md` - the SPD entry roles (`dataplane.PolicyRole`)
- [ ] `docs/guide/ipsec.md` - the `unmatched` leaf the operator writes, and what it says about VPP
- [ ] `docs/guide/vpp.md` - what the VPP backend supports
- [ ] `ai/digests/ipsec-ike.md`, `ai/digests/vpp-dataplane.md`, `ai/digests/ospf.md` - current code flow
- [ ] `ai/doors/access-security.md`, `ai/doors/forwarding.md`, `ai/doors/routing-protocols.md` - `ike/dataplane` is the ungated seam OSPF also uses; VPP exact-or-refuse

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4301.md` - rows RFC4301-4.4.1-3 (`{partial}`), RFC4301-5.2-1 and RFC4301-5.2-9 (`{gap}`, the same two paths)
- [ ] `rfc/full/rfc4301.txt` Sections 4.4.1 and 5.2 - the sentences quoted in Task
- [ ] `rfc/short/rfc4552.md` - what the OSPFv3 manual-key SPD entries owe

**Key insights:**
- Each XFRM if_id is one SPD; the VPP backend holds one SPD per backend, bound per interface; the OSPF entries sit at if_id 0 with an interface selector.
- A node-wide DISCARD installed on behalf of OSPF alone would drop every cleartext packet of the box. What "the SPD" covers for OSPFv3 manual keying is the first design question.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/dataplane/vpp_policy.go` - `vppPolicyInterface` refuses `SPParams.IfIndex` 0 ("VPP has no node-wide SPD"); `vppBackend.CatchAllSupported` answers `dataplane.ErrNoNodeWideSPD`; `vppBackend.ensureSPD` creates the backend's one SPD (lowest id VPP does not hold, confirmed by dump) and binds it to each `sw_if_index` once, installing no entry of its own; `vppBackend.spdEntry` and `vppSPDSelector` build each entry
- [ ] `internal/component/ike/engine/unmatched.go` - `installUnmatched` installs the node-wide catch-all; on a backend that declares `ErrNoNodeWideSPD` it logs a warning and installs nothing under `unmatched bypass`; `verifyUnmatchedEnforceable` refuses `unmatched discard` there
- [ ] `internal/component/ike/engine/unmatched_xfrm_interface.go` - `interfaceSPDCatchAll(ifID)` builds in, out and fwd for both families at the interface's if_id, ranked at `dataplane.PriorityUnmatched`, DISCARD under both values of `unmatched`
- [ ] `internal/plugins/ospf/ipsec_install.go` - `defaultDataplaneSource` returns the loaded backend, loading `xfrm` only when none is; `buildIPsecPolicies(ifindex, c)` builds out, in and fwd transport PROTECT entries over `::/0`, upper protocol 89, `IfIndex` the kernel ifindex, `Role: dataplane.PolicyRoleOSPF`; no final entry is installed beneath them
- [ ] `internal/component/ike/dataplane/dataplane.go` - `SPParams.IfIndex` comment: the OSPF installer's kernel ifindex reaches the VPP backend as a `sw_if_index` when VPP is loaded, kept inert today by three refusals (`spdEntry` on fwd, `vppProtectMode` on transport, `vppUnsupportedSA` on the SA)
- [ ] `internal/component/ike/dataplane/policy_role.go` - `PolicyRoleCatchAll`, `PolicyRoleInterfaceCatchAll`, `PolicyRoleOSPF`

**Behavior to preserve:**
- The XFRM node-wide and per-interface final entries, their install-before and release-after order, and every test tagged RFC4301-4.4.1-3 today.
- An OSPFv3 interface without IPsec still exchanges OSPF in the clear (the interface-scoped selector, RFC 4552 Section 6).
- Traffic that no Ze IPsec configuration names is not dropped by an entry installed for OSPFv3 alone, unless the design decides otherwise and says why.

**Behavior to change:**
- The VPP SPD gets a final DISCARD entry, and `unmatched discard` stops being refused on VPP once it does.
- The SPD holding the OSPFv3 manual-key entries gets a final entry whose scope the design settles.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|---------------------------------------------------------------------|
| Node-wide catch-all | `internal/component/ike/engine/unmatched.go` `installUnmatched`, `unmatchedPolicies` | Installs the if_id 0 final entry for both families, ranked last, before every other Ze entry | Extends: the same builder shape, ranked last, for the VPP SPD |
| Per-interface catch-all (commit 78f09955aa) | `internal/component/ike/engine/unmatched_xfrm_interface.go` `interfaceSPDCatchAll`, `spdCatchAllEntries` | One DISCARD final entry per bound XFRM interface SPD, in, out and fwd, both families, installed before the if_id 0 exemptions and removed after them | Extends: the model of one final entry per SPD, installed first and released last; the VPP SPD and the OSPF SPD each get theirs the same way |
| Catch-all capability declaration | `internal/component/ike/dataplane/dataplane.go` `CatchAllInstaller`; `vpp_policy.go` `vppBackend.CatchAllSupported` | A backend declares whether it can hold an entry bound to no interface; VPP answers `ErrNoNodeWideSPD` | Extends: VPP answers per bound SPD instead of refusing outright; the declaration stays the backend's own |
| VPP SPD creation | `vpp_policy.go` `vppBackend.ensureSPD`, `undoSPD`, `deleteSPD` | Creates and binds the one SPD, undoes on failure, deletes on Close | Extends: the final entry is installed when the SPD is created and removed before it is deleted |
| Policy roles | `internal/component/ike/dataplane/policy_role.go` `PolicyRole` | Labels each SPD entry's owner | Extends: the new final entries carry a catch-all role, so readback and `show` name them |
| OSPFv3 manual-key installer | `internal/plugins/ospf/ipsec_install.go` `ipsecInstaller`, `buildIPsecPolicies` | Installs and removes the per-interface protocol 89 entries | Extends: the installer installs and releases the final entry its SPD owes, through the `ipsecDataplane` seam, never through the IKE engine |
| Exact or refuse at verify | `docs/architecture/core-design.md` section 14a; `unmatched.go` `verifyUnmatchedEnforceable` | Refuses a disposition the backend cannot enforce | Extends: the VPP refusal is lifted only by the backend's own declaration once the final entry exists |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `vpn ipsec` config apply (IKE engine) and OSPFv3 `ipsec` interface config (OSPF plugin).

### Transformation Path
1. Config apply builds `dataplane.SPParams`.
2. The loaded backend writes them: `xfrm_linux.go` through netlink, `vpp_policy.go` through the VPP binary API.
3. The kernel or VPP classifies each packet against the SPD.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| OSPF plugin to `ike/dataplane` | Go call into the ungated seam (`ai/doors/access-security.md`); in-process only today | No |
| Dataplane to VPP | GoVPP binary API (`ipsec_spd_entry_add_del_v2`) | No |
| Dataplane to kernel | XFRM netlink | No |

### Integration Points
- `vppBackend.ensureSPD` (VPP final entry); `ipsecInstaller` (OSPF final entry); `verifyUnmatchedEnforceable` (VPP refusal).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | OSPF must not import `ike/engine` |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|------------|---------------------------------|----------|--------------|--------|
| A-1 | VPP passes a packet that matches no entry of a bound SPD, so a final entry changes what VPP forwards | Unread: no VPP source checkout is local. Read `src/vnet/ipsec/ipsec_output.c` and `ipsec_input.c` at the lab's VPP version and quote the no-match branch | If VPP already drops unmatched traffic, the final entry is a declaration and the row's test asserts the entry Ze installs | VPP source read | unvalidated |
| A-2 | VPP orders SPD entries by priority, and Ze can rank an entry below every other it installs | Unread: VPP SPD priority semantics (higher or lower first) | The rank constant differs from XFRM's | VPP source read; `vppSPDSelector` | unvalidated |
| A-3 | When the IKE engine has installed the if_id 0 catch-all, it is also the final entry of the OSPF entries, because both sit in the if_id 0 SPD | Inference from `buildIPsecPolicies` (no if_id) and `installUnmatched`; the RFC4301-5.2-1 `{gap}` text claims the OSPF entries have no final entry "whether or not the IKE engine has applied a configuration", which this contradicts | The design differs for the IKE-present case | Integration test reading XFRM state with both installed | unvalidated |
| A-4 | `PriorityUnmatched` ranks below the OSPF entries' priority | `buildIPsecPolicies` sets no priority | A catch-all could shadow the OSPF entries | XFRM readback | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | A node-wide DISCARD installed for OSPFv3 alone drops all cleartext traffic on the box | Any non-OSPF test traffic fails with OSPFv3 IPsec configured | The design scopes the OSPF final entry to what the OSPF SPD covers (interface plus protocol 89, or the `unmatched` disposition), and states the RFC reading |
| R-2 | The OSPF installer's kernel ifindex reaches VPP as a `sw_if_index` (`SPParams.IfIndex` comment) | A VPP final entry on the wrong interface | Depends on `spec-ipsec-vpp-policy-interface.md`; until then the OSPF final entry is XFRM-only and VPP refuses as today |
| R-3 | A final entry left behind after Close or release | XFRM or VPP readback lists a Ze catch-all with no other Ze entry | Install first, release last, as `interfaceSPDCatchAll` does; a release test |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Cleartext traffic dropped on a VPP box or an OSPFv3-IPsec box, or traffic left unprotected |
| How is it reverted? | Single commit revert |
| Who else touches this path? | spec-ipsec-spd-selection, closed (XFRM SPDs), `spec-ipsec-vpp-policy-interface.md` (VPP interface index), the OSPF plugin |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `vpn ipsec` apply on the VPP backend | → | `vppBackend.ensureSPD` installs the final entry | VPP readback test (name in design) |
| OSPFv3 `ipsec` interface config | → | `ipsecInstaller` installs the final entry | XFRM integration test reading the kernel SPD (name in design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|------------------------------|
| AC-1 | VPP backend, a Child SA policy installed | The VPP SPD Ze created holds a DISCARD entry, both families and both directions, ranked below every other Ze entry, installed before them and removed after them; readback lists it with its catch-all role | None |
| AC-2 | VPP backend, `unmatched discard` | Accepted at verify, because the backend declares it can hold the final entry of each SPD it binds | `unmatched bypass` still accepted |
| AC-3 | VPP backend, a packet matching no entry on a bound interface | Discarded (VPP counter or test traffic), per A-1 | If A-1 shows VPP drops already, the AC asserts the installed entry only, and the design says so |
| AC-4 | OSPFv3 RFC 4552 manual keys configured, no `vpn ipsec` block | The SPD holding the OSPF entries carries a final DISCARD entry of the scope the design settles, installed before the OSPF entries and removed after them | Cleartext traffic outside that scope, with the RFC reading recorded |
| AC-5 | OSPFv3 manual keys plus a `vpn ipsec` block, then the IKE engine releases its SPD | The OSPF entries keep a final entry throughout | None |
| AC-6 | Close or reload removing all Ze IPsec config | No Ze final entry remains in VPP or the kernel | None |
| AC-7 | `rfc/short/rfc4301.md` | RFC4301-4.4.1-3 carries no `{partial}` and has positive and negative tags with discrimination records over the VPP and OSPF producers; the two paths are removed from the RFC4301-5.2-1 and RFC4301-5.2-9 `{gap}` text | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|---------------------|-----------------------|
| 1 | Configures IPsec on a VPP box with `unmatched discard` | config verify, apply, `ensureSPD`, VPP | VPP functional or integration test (design names it) |
| 2 | Configures OSPFv3 authentication with manual keys only | OSPF config, `ipsecInstaller`, XFRM | Linux integration test reading XFRM policies |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| VPP final entry built and ordered (name in design) | `internal/component/ike/dataplane/` (vpp test file) | AC-1, AC-6 | |
| VPP `unmatched discard` accepted (name in design) | `internal/component/ike/engine/` | AC-2 | |
| OSPF installer final entry order (name in design) | `internal/plugins/ospf/ipsec_install_test.go` | AC-4, AC-5 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| XFRM readback with OSPFv3 manual keys (name in design) | Linux integration (`*_integration_linux_test.go`) | AC-4, AC-5, AC-6 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| An existing OSPFv3 IPsec scenario against FRR (name in design) | `test/interop/scenarios/` | FRR | OSPFv3 manual-key adjacency still forms with the final entry installed | |

Tests owed on the row: RFC4301-4.4.1-3 positive and negative over the VPP SPD and over the OSPF SPD, each with a discrimination record (`./le rfc record`); the existing XFRM tags stay.

## Files to Modify
- `internal/component/ike/dataplane/vpp_policy.go` - final entry in `ensureSPD`, removal before `deleteSPD`, `CatchAllSupported`
- `internal/component/ike/engine/unmatched.go` - `installUnmatched` and `verifyUnmatchedEnforceable` follow the new declaration
- `internal/plugins/ospf/ipsec_install.go` - the OSPF final entry
- `internal/component/ike/dataplane/policy_role.go` - a role if a new one is needed
- `rfc/short/rfc4301.md` - remove `{partial}` from RFC4301-4.4.1-3; narrow RFC4301-5.2-1 and RFC4301-5.2-9
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, `docs/guide/ipsec.md`, `docs/guide/vpp.md` - what each SPD holds
- `internal/component/ike/engine/unmatched_xfrm_interface.go`, `apply.go` - hold `xfrmInterfaceEntry` by pointer in its maps, clearing the four gocritic `rangeValCopy` findings (lines 257, 352, 405, 429) spec-ipsec-spd-selection left. Inherited from that spec's Work Not Done: the change moves the fingerprints of `planXFRMInterfaceEntries` and `installXFRMInterfaceEntries`, which the RFC4301-4.4.1-3 mutant records and the RFC4301-5.2-1, 4.4.1-3 and 5.2-9 audit verdicts pin, so it lands here, where those proofs are re-recorded anyway

## Files to Create
- Tests named in the design.

## Implementation Steps
1. Research: read VPP's no-match branches and priority order (A-1, A-2); settle the OSPF SPD scope (R-1) from RFC 4301 and RFC 4552, reading FRR's and BIRD's handling where the RFC is silent.
2. Design: write the decisions into this spec, move to `design`.
3. Tests first, then the VPP final entry, then the OSPF final entry, then the RFC rows and docs.

## Design Insights

## Owned journal rows

| Journal row | File |
|-------------|------|
| - | - |

## Key Design Decisions
- Open: the scope of the OSPF SPD's final entry (R-1).
- Open: whether the VPP final entry honours `unmatched bypass` or always discards.

## Known Limitations

## RFC Documentation (Scope: protocol)
- Quote RFC 4301 Section 4.4.1 above each new final-entry builder, and name the section at each caller (`ai/rules/rfc-compliance.md`).

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] A-1 to A-4 validated
- [ ] Existing Mechanisms re-checked against HEAD

### Goal Gates (MUST pass)
- [ ] Every AC has its test
- [ ] RFC4301-4.4.1-3 whole, both polarities, records verified

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N-A: the final entry takes no numeric operator input)
- [ ] Linux integration test for the OSPF SPD: `./le test integration` over `internal/component/ike/dataplane` and `internal/plugins/ospf`; unit packages through `./le test unit plugins`; `./le test ospfv3` for the OSPFv3 functional suite; `./le rfc check`
- [ ] Interop tests for protocol features: an existing OSPFv3 IPsec scenario against FRR through `./le test integration interop` (name in design)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/post-release/features/spec-ipsec-spd-final-discard-vpp-ospf.md` only
