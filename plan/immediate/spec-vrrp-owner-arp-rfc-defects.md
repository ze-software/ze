# Spec: vrrp-owner-arp-rfc-defects

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Commit `850eb41b66` stopped the VRRP address owner's parent from answering ARP
and ND for the virtual address with its physical MAC. Two halves were recorded
open in `plan/journal/guard-added-to-one-half-of-a-pair.md` on 2026-09-27
(rows at lines 94 and 95). Both are re-confirmed open in HEAD `3e2770062b` on
2026-10-13; no commit after `850eb41b66` touches `ownerfilter.go` or the vpp
`verify.go`.

Research on 2026-10-13 changed both rows' shape. Neither repair the journal
names for D1 (drop or rewrite) keeps the owner able to resolve its own LAN
neighbours, and D2 as written ("refused with a generic error") describes a path
the VRRP tables never take: plugin-registered tables skip the vpp verifier, and
the vpp translator silently ignores a match it does not know. The design below
answers both, and Owner Questions carry the decisions it cannot make.

## Defects

| ID | Source | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record | Status in HEAD |
|----|--------|----------------|----------|--------------------|------------------------|----------------|
| D1 | RFC 9568 Section 8.1.2 (RFC9568-8.1.2-3, SHOULD NOT); restates RFC 5798 Section 8.1.2 (RFC5798-8.1.2-2) and RFC 3768 Section 8.2 (RFC3768-8.2-2), both `superseded` onto 8.1.2-3 | "When a VRRP Router restarts or boots, it SHOULD NOT send any ARP messages using its physical MAC address for an IPv4 address for which it is the IPv4 address owner (as defined in Section 1.7), and it should only send ARP messages that include Virtual Router MAC addresses." | `internal/plugins/vrrp/ownerfilter.go` `ownerFilterTables` emits `ownerARPReplyTerm` only (one IPv4 term per owned address, ARP operation reply); `internal/plugins/vrrp/dataplane_linux.go` `parentSysctls` sets `arp_ignore=1`, `arp_filter=1`, `rp_filter=1` and leaves `arp_announce` at the host value | An ARP request the parent sends while resolving a LAN neighbour, whose sender protocol address is the owned address, leaves with the parent's physical MAC as sender hardware address. A host that receives it (the target always; any host already holding an entry for the address, by the RFC 826 merge rule) records the owned address against the physical MAC | No tag, no audit entry: `rfc/audit/rfc9568.json` has no `RFC9568-8.1.2-3` key; `./le rfc check` on 2026-10-13 prints no VRRP line | open |
| D2 | journal row 95 (no RFC sentence: backend parity), widened by research to `ai/rules/principles.md` "silently wrong value" | (none) | `internal/plugins/firewall/vpp/verify.go` `verifyMatch` (no case for `MatchARPOperation`, `MatchARPSenderAddress`, `MatchNDTargetAddress`; `verifyTable` never reads `Table.Family`); `internal/plugins/firewall/vpp/translate.go` `applyMatch` (no default arm) and `translateTerm`; `internal/component/firewall/registry.go` `ApplyAll` (hands registry tables to `Backend.Apply` with no `RunVerifier`) | Commit-time: the three matches fall to "match type ... not recognized by backend vpp", which no operator config can reach today, because no YANG leaf produces them (`plan/spec-firewall-arp-nd-matches.md`, status ready, not implemented). Runtime: the VRRP owner tables reach `firewall.RegisterTables` then `ApplyAll` then the vpp `Apply`, which never runs `Verify`; `applyMatch` returns nil for every unknown match, and the arp family is translated as an IP ACL. An owner term (output interface, ARP operation, sender address, drop) becomes an ACL rule with no constraint and a deny verdict, on the output hook | journal row, not fixed | open; runtime reachability is A-2 |
| D3 (proposed, Q3) | RFC 9568 Section 8.2.2 (RFC9568-8.2.2-5, SHOULD NOT) | "When a VRRP Router restarts or boots, it SHOULD NOT send any ND messages with its physical MAC address for the IPv6 address it owns and it should only send ND messages that include Virtual Router MAC addresses." | `ownerfilter.go` `ownerNDAdvertTerm` (Neighbor Advertisements only) | The IPv6 sibling of D1: a Neighbor Solicitation the parent sends with the owned address as source carries the physical MAC in its Source Link-Layer Address option, and the solicited host records it | No tag, no audit entry | open (not in the skeleton's scope) |

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | D1: a QEMU guest test capturing the parent's own ARP request while it resolves a LAN host. D2: a vpp verifier and translator unit test per match and per family. D3: the IPv6 capture, if Q3 is yes |
| Both polarities | D1: the owner still resolves the LAN host (the request is answered and the neighbour entry reaches REACHABLE); no request carries the owned address with the physical MAC. D2: each match refuses with its own reason; a known match still translates |
| Discrimination | the QEMU test red with the parent recipe reverted, recorded with `./le rfc discriminate-record` the way `850eb41b66` recorded the reply term |
| Real entry point | the `vrrp-keepalived` QEMU lab (`internal/le/test/qemu/guestlabs.go`, path `v2-owner-keeps-the-address` and a new owner-request path) |
| Interop | `vrrp-v2-owner-keepalived` and `vrrp-mastership-keepalived` stay green, and the owner scenario gains the request assertion |

## Owner Questions

| Q | Question | Options | Recommendation |
|---|----------|---------|----------------|
| Q1 | D1 shape. The journal's two repairs both break the owner's own neighbour resolution (R-1, R-2). Which shape? | (a) parent `arp_announce=2` on every VRRP parent, so the kernel picks the parent's primary address in the target's subnet as sender; no new firewall term. (b) (a) plus a drop term for requests still carrying the owned address. (c) move the owned address off the parent while Active, the macvlan becoming the sole holder at the subnet prefix, which removes the owner filter altogether. (d) record a deviation from the SHOULD NOT | (a). It is a sysctl in the recipe that already carries `arp_ignore` and `arp_filter`, it adds no firewall match, and it cannot cut resolution. (b) cuts resolution whenever (a) cannot steer. (c) is the only shape that covers Q2's case and is a redesign of the owner dataplane |
| Q2 | Under (a), an owner whose owned address is the parent's only address in that subnet, or its primary one, still sends the physical MAC with the owned address: no other sender exists. What does Ze do for that configuration? | (i) refuse at commit: an owner group needs a non-virtual address listed first in the same subnet. (ii) accept, and report it as a `ze doctor` warning naming the SHOULD NOT. (iii) take Q1 (c) for that case only | (ii). The RFC sentence is a SHOULD NOT scoped to restart and boot, and its own list asks for "an IPv4 address known to belong to that router" (RFC 9568 Section 8.1.2, third bullet). Refusing (i) rejects a configuration RFC 9568 Section 1.7 defines as the normal owner |
| Q3 | Take D3, the IPv6 sibling, in this spec? | yes / no (journal row) | yes. Same mechanism and the same test lab; IPv6 has no `arp_announce`, so its answer is the IPv6 source-address preference and must be researched in this spec's next pass |
| Q4 | D2 shape | (a) the vpp verifier refuses each of the three matches and every non-ip family with its own reason, and `ApplyAll` runs the active backend's verifier over registry tables before `Apply`, and `applyMatch` gains a default arm that refuses. (b) lower the matches on vpp | (a). VPP's ACL plugin matches IP headers; VRRP is refused on the vpp interface backend already (`internal/plugins/vrrp/groups.go` `validateGroups`) |
| Q5 | Under Q1 (a) the recipe writes `arp_announce`, which the operator can also set (`internal/component/iface/config_sysctl.go`). Which wins? | the VRRP recipe, as it does for `arp_ignore` today / the operator's value, with a doctor warning | the VRRP recipe, same as `arp_ignore` (`reassertDataplaneSysctls`), so one rule governs the three knobs |
| Q6 | RFC-tagged tests change under the recommended design (list in the TDD plan). Approve the edits? | yes / no | yes |

-> Constraint (owner, 2026-10-13, on Q1 and Q5): "we have to be careful about playing with linux arp changes as to accept some loopback locally we may need to change arp too and it may conflict." The `arp_*` sysctls are shared by every feature that answers for an address it holds on another interface (a loopback or anycast address answered locally, the LVS direct-routing pattern that sets `arp_ignore`/`arp_announce` for a VIP on `lo`). The Q1 (a) recommendation and the Q5 "VRRP wins" recommendation are therefore NOT settled: the next design pass must inventory every Ze writer and reader of `arp_ignore`, `arp_announce` and `arp_filter` (the VRRP recipe, `internal/component/iface/config_sysctl.go`, any loopback or anycast feature), state per interface which owner each knob has, and prefer a shape that does not widen VRRP's sysctl footprint (Q1 (c), or a per-interface scope that cannot touch `all`/`default` or a loopback). How keepalived, FRR vrrpd and VyOS treat the same conflict is being researched.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md` - owner dataplane, "The address owner needs a filter, not a sysctl"
  → Constraint: the owned address sits on the parent at the subnet prefix and on the private macvlan as /32 or /128 (`vipMaskBits`), so the parent holds the subnet's connected route and every LAN-bound request of the owner leaves the parent
  → Decision: the doc says requests and Neighbor Solicitations from the parent "are left alone: they are its own resolution traffic"; that sentence is D1 and changes with the fix
  → Constraint: the macvlan is private mode (`internal/plugins/vrrp/register.go` line 393), so a unicast frame to the virtual MAC is delivered to the macvlan, never to the parent
- [ ] `docs/architecture/firewall/fw-6-firewall-vpp.md` - vpp verifier and translation contract
  → Constraint: the verifier runs at commit through `RunVerifier` (`internal/component/firewall/engine.go` line 219) over config tables only; the translator's comment says the verifier guarantees unsupported expressions were rejected, which is false for registry tables
- [ ] `docs/architecture/testing/interop.md` - VRRP scenarios, lab sysctl grants
  → Constraint: the VRRP scenarios' ze writes only lab-granted sysctls (`TestVRRPWritesOnlyLabGrantedSysctls`, `internal/plugins/vrrp/lab_profile_linux_test.go`); a new `arp_announce` write needs its grant
- [ ] `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` - declared by `groups.go`, `doctor.go`, `instance.go`, `register.go`; component map and owner auto-detection
  → Constraint: owner detection forces priority 255 and `accept-mode true` (section "Owner auto-detection forces the effective values"); a Q2 rule reads the same `realAddresses` detection and changes the page's owner section
  → Constraint: the dataplane row names `dataplane_linux.go` as "the macvlan ARP-ownership recipe"; the `arp_announce` knob is added to that recipe, so the row stays true and the recipe list in the dataplane doc changes
- [ ] `docs/architecture/core-design.md` - declared by `internal/component/firewall/registry.go`, `backend.go`, `engine.go`
  → Constraint: components reach backends through registries; the `ApplyAll` verifier call looks up the active backend's registered verifier by name and spells no backend
- [ ] `docs/architecture/testing/qemu-integration.md` - the `vrrp-keepalived` guest lab
  → Constraint: kernel ARP behaviour is proven in the QEMU guest, never in a host namespace (root in agent sessions is limited to the stress harness, `.claude/rules/sudo.md`)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9568.md` - rows 8.1.2-1 to -6 and 8.2.2-1 to -8
  → Constraint: 8.1.2-3 and 8.2.2-5 are SHOULD NOT rows in the Compliance Checklist, untagged; adding a tag owes a discrimination record (`ai/rules/rfc-compliance.md`)
- [ ] `rfc/short/rfc5798.md`, `rfc/short/rfc3768.md` - superseded rows 5798-8.1.2-2, 3768-8.2-2
  → Constraint: both are `superseded` onto RFC9568-8.1.2-3, so the tag goes on the 9568 row only
- [ ] `rfc/full/rfc9568.txt` Section 8.1.2, read 2026-10-13
  → Constraint: Section 8.1.2: "Note that the source address of the Ethernet frame of this ARP response is the physical MAC address of the physical router." The obligation is on the ARP payload's sender hardware address, not the Ethernet source
  → Constraint: Section 8.1.2, third bullet: "When, for example, Secure Shell (SSH) access to a particular VRRP Router is required, an IPv4 address known to belong to that router SHOULD be used." The RFC expects a router address other than the virtual one for the router's own traffic, which is what Q1 (a) steers to

**Key insights:**
- D1 cannot be fixed by a firewall term alone: drop starves the owner's own resolution, rewrite sends the reply to the macvlan (R-1, R-2)
- The sender protocol address the kernel puts in a request is chosen by `arp_announce`; level 2 picks the primary address of the outgoing interface in the target's subnet
- D2's real defect is the runtime path: registry tables reach `vpp.Apply` unverified and unknown matches vanish in `applyMatch`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/vrrp/ownerfilter.go` (265L) - `ownedVIPs`, `setOwnerFilter`, `clearOwnerFilter`, `ownerFilterTables`, `ownerARPReplyTerm`, `ownerNDAdvertTerm`, `ownerFilterTable`; one arp table `ze_vrrp_owner_arp` and one ip6 table `ze_vrrp_owner_nd`, output hook, policy accept, owner `vrrp-owner`
  → Constraint: published through `firewall.RegisterTables` then `firewall.ApplyAll`; the backend is whichever the firewall component loaded or the OS default
- [ ] `internal/plugins/vrrp/register.go` - `vipCIDRs`, `vipMaskBits` (owned address installed as a host route), macvlan mode private
  → Constraint: the parent keeps the subnet's connected route for an owner
- [ ] `internal/plugins/vrrp/instance.go` - `doInstallVIPs` line 595 calls `setOwnerFilter` before install; `doRemoveVIPs` line 656 calls `clearOwnerFilter` after removal
- [ ] `internal/plugins/vrrp/dataplane_linux.go` - `parentSysctls` (arp_ignore=1, arp_filter=1, rp_filter=1), `vmacSysctls`, `applyDataplaneSysctls`, `reassertDataplaneSysctls`, `restoreParentSysctls`
  → Constraint: shared parent knobs are saved on the first group and restored on the last (`applySharedSysctls`, `restoreDataplaneSysctls`); a new knob joins that save and restore
- [ ] `internal/plugins/vrrp/groups.go` - `validateGroups` refuses VRRP when `/interface/backend` is vpp
- [ ] `internal/plugins/firewall/vpp/verify.go` (400L) - `Verify`, `verifyTable` (sets, flowtables, chains; no family check), `verifyMatch` (default "not recognized by backend vpp")
- [ ] `internal/plugins/firewall/vpp/translate.go` - `translateTerm`, `applyMatch` (switch with no default arm), `chainToACLRules`
- [ ] `internal/plugins/firewall/vpp/backend_linux.go` - `Apply`, `applyAll` (translates every base non-NAT chain, no family read), `bindAllACLs` (hook to input or output)
- [ ] `internal/component/firewall/registry.go` - `RegisterTables`, `ApplyAll` (no verifier call)
- [ ] `internal/component/firewall/engine.go` - `RunVerifier` at commit over `cfg.Tables` only
- [ ] `internal/plugins/vrrp/owner_answer_integration_linux_test.go` - `TestVRRPOwnerAnswersWithVirtualMACOnly`, QEMU guest, the capture harness this spec extends

**Behavior to change:** D1 (and D3 if Q3 is yes): no ARP request (Neighbor Solicitation) leaves the owner's parent with the owned address as sender and the physical MAC. D2: registry tables on the vpp backend are verified, and an unknown match or family is refused, never translated to an unconstrained rule.

**Behavior to preserve:** the reply and advertisement drops and their tests; the owner resolves its LAN neighbours; non-owner groups' recipe; the save-and-restore of parent knobs on the last group.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- an interface unit config holding a VRRP group whose virtual address is also a configured address of the unit (owner)

### Transformation Path
1. `groups.go` extracts the group spec with `realAddresses`; `validateGroups` refuses the vpp interface backend
2. on promotion `doInstallVIPs` applies the parent recipe (`applyDataplaneSysctls`), publishes the owner filter (`setOwnerFilter`), installs the address on the macvlan
3. the firewall registry hands every owner's tables to the loaded backend's `Apply` (nft lowers the arp family; vpp translates to ACLs with no verification)
4. the kernel builds the parent's ARP requests: sender protocol address chosen by `arp_announce`, sender hardware address the parent's MAC

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| VRRP plugin ↔ firewall component | `firewall.RegisterTables` / `ApplyAll` with `firewall.Table` values | Yes, `ownerfilter.go` lines 128-150 |
| firewall component ↔ vpp backend | `Backend.Apply`, no `Verifier` on this path | Yes, `registry.go` `ApplyAll` |
| VRRP plugin ↔ kernel | procfs sysctl writes in `dataplane_linux.go` | Yes |

### Integration Points
- `parentSysctls` recipe (Q1 a), the firewall backend verifier registry (`firewall.RegisterVerifier`, Q4 a)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | registry tables bypass the backend verifier today (D2); the fix routes them through it |
| No unintended coupling (components stay isolated) | Yes | VRRP touches the firewall component only through `RegisterTables`; the verifier is looked up by backend name |
| No duplicated functionality (extends existing, does not recreate) | Yes | reuses `RunVerifier` and the parent sysctl recipe |
| Zero-copy preserved where applicable (refs, not copies) | N-A | control plane only |
| Registration over hardcoding, outbound | Yes | the verifier is already registered per backend |
| Registration over hardcoding, inbound | Yes | `ApplyAll` asks the registry for the active backend's verifier; no backend name is spelled |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|------------|---------------------------------|----------|--------------|--------|
| A-1 | A host's ARP reply to a request whose sender hardware address is the virtual MAC is sent to the virtual MAC and so is delivered to the private macvlan, leaving the parent's neighbour entry incomplete | macvlan private mode (`register.go` line 393); Linux `arp_send` replies to the request's sender hardware address | Q1 rewrite would work and becomes a cheaper candidate | QEMU guest probe in the implementation's first phase: rewrite rule, `ip neigh` on the parent | unvalidated |
| A-2 | The vpp firewall backend can be the loaded firewall backend while VRRP runs on the netlink interface backend | no cross-component gate found: `validateBackendGate` reads only firewall YANG; `validateGroups` reads only `/interface/backend` | D2's runtime half is unreachable today but stays a latent silent-deny for any future registry table with an unknown match; the fix is still owed under `principles.md` | unit test with both backends configured, or a grep of every backend gate | unvalidated |
| A-3 | With `arp_announce=2` the kernel picks the parent's primary address in the target's subnet even when the triggering packet's source is the owned address | Linux `ip-sysctl` documentation for `arp_announce` level 2 | Q1 (a) does not fix D1 | the new QEMU test, both polarities | unvalidated |
| A-4 | The interop and QEMU owner configs list a non-virtual address first (`172.30.0.2` in `test/interop/scenarios/vrrp-v2-owner-keepalived/ze.conf`, `vrrpZeAddress` in `internal/le/test/qemu/vrrp_owner_linux.go` line 64), so it is the primary | the configs and their comments | the test cannot observe the fix | read at test-writing time | validated (configs read 2026-10-13) |
| A-5 | Hosts record the sender of an ARP request: the target always, others when they hold an entry | RFC 826 packet reception ("merge flag"); Linux `arp_process` | D1 is cosmetic | the QEMU peer's `ip neigh` after a parent request, before the fix | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | A drop term for requests (journal option 1, Q1 b) starves the owner's own neighbour resolution when no other sender exists | owner cannot ping a LAN host; neighbour entry FAILED | do not ship (b) without Q2 (i) |
| R-2 | A sender-MAC rewrite (journal option 2) sends the reply to the macvlan (A-1) | parent neighbour stays INCOMPLETE | rejected unless A-1 is broken |
| R-3 | `arp_announce=2` overrides an operator's iface `arp-announce` value | doctor or log line on reassert | Q5; restored on the last group like `arp_ignore` |
| R-4 | `arp_announce=2` changes the sender of the router's ARP for non-VRRP traffic on the parent | other ARP-dependent tests change | it only narrows the sender to the interface's own primary address, the documented VRRP host recipe; run the VRRP QEMU and interop sets |
| R-5 | `ApplyAll` running the verifier refuses a registry table that applies today on vpp | a plugin's apply error on a vpp box | correct by `principles.md`: today that table becomes a different ACL than asked; list every `RegisterTables` caller in the implementation audit |
| R-6 | The lab sysctl grant check refuses the new write | `TestVRRPWritesOnlyLabGrantedSysctls` red | add the grant in the same change |
| R-7 | Owners whose owned address is primary or sole stay non-conformant to the SHOULD NOT (Q2) | doctor warning, if Q2 (ii) | owner decision |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | an owner Master that cannot resolve LAN hosts (Q1 b) or a vpp box applying a deny ACL; both caught by the tests below |
| How is it reverted? | single commit revert per defect |
| Who else touches this path? | `plan/spec-firewall-arp-nd-matches.md` (operator ARP/ND leaves, `ze:backend nft`, defers the vpp daemon-side refusal to this spec's D2); `plan/spec-dataplane-seams-5-copp-non-tcp.md` (non-inet families) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| owner group config, promotion | → | `parentSysctls` with `arp_announce=2` | `TestDataplaneApplyIPv4SetsRecipe` (extended) |
| owner group in the guest, parent resolves a LAN host | → | kernel request sender on the parent | `TestVRRPOwnerRequestsCarryNoPhysicalMACForOwnedAddress` (new, QEMU) |
| registry table on the vpp backend | → | `ApplyAll` runs the backend verifier | `TestApplyAllVerifiesRegistryTables` (new, `internal/component/firewall`) |
| VRRP owner tables handed to vpp | → | `verifyTable` family check, `verifyMatch` cases | `TestVerifyRejectsARPAndNDMatchesWithReason` (new, vpp) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | owner Master whose parent holds a non-virtual primary address in the subnet; the parent resolves a LAN host by pinging it from the owned address | every ARP request captured on the LAN from the parent's physical MAC carries the primary address as sender, none the owned address; the peer's neighbour entry for the owned address stays on the virtual MAC |
| AC-2 | same setup, AC-1 traffic | the parent's neighbour entry for the LAN host reaches REACHABLE and the ping succeeds |
| AC-3 | same group with the recipe's `arp_announce` reverted | the capture sees a request with the owned address and the physical MAC (the red control) |
| AC-4 | last VRRP group on the parent removed | `arp_announce` is restored to its pre-VRRP value |
| AC-5 | Q2 configuration (owned address primary or sole), per the answer | (i) commit refused with a message naming the group and the missing address; or (ii) `ze doctor` warning naming the group and RFC 9568 Section 8.1.2 |
| AC-6 | vpp `Verify` over a table holding each of `MatchARPOperation`, `MatchARPSenderAddress`, `MatchNDTargetAddress` | one error per match naming it and the reason VPP ACLs cannot match it; never "not recognized" |
| AC-7 | vpp `Verify` over a table of family arp, bridge or netdev | refused naming the family |
| AC-8 | `firewall.ApplyAll` with the vpp backend loaded and a registry table carrying an unsupported match | returns the verifier's error and calls no `Apply` |
| AC-9 | `applyMatch` given a match type it has no case for | returns an error naming the type; no rule is produced |
| AC-10 (if Q3 yes) | IPv6 owner Master resolves a LAN host | no Neighbor Solicitation leaves the parent with the owned address as source and the physical MAC in its Source Link-Layer Address option |

## End-to-End User Stories

| Story | Path | Test |
|-------|------|------|
| An operator runs Ze as the VRRP address owner; LAN hosts keep the gateway on the virtual MAC while the owner talks to them | config → promotion → recipe → kernel ARP | `TestVRRPOwnerRequestsCarryNoPhysicalMACForOwnedAddress`, `vrrp-v2-owner-keepalived` |
| An operator on the vpp firewall backend gets a refusal, not a silent deny, when a plugin registers a table VPP cannot express | registry → `ApplyAll` → verifier | `TestApplyAllVerifiesRegistryTables` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDataplaneApplyIPv4SetsRecipe` (RFC-tagged, body changes: asserts `arp_announce=2` on the parent) | `internal/plugins/vrrp/dataplane_linux_test.go` | AC-1 recipe | [ ] |
| `TestDataplaneRestoreOnLastGroup` (RFC-tagged, body changes: asserts the restore of `arp_announce`) | same | AC-4 | [ ] |
| `TestVRRPWritesOnlyLabGrantedSysctls` (untagged, grant list changes) | `internal/plugins/vrrp/lab_profile_linux_test.go` | R-6 | [ ] |
| `TestOwnerGroupPrimaryAddressRule` (new; shape follows Q2) | `internal/plugins/vrrp/groups_test.go` or `doctor_test.go` | AC-5 | [ ] |
| `TestVerifyRejectsARPAndNDMatchesWithReason` (new) | `internal/plugins/firewall/vpp/verify_test.go` | AC-6 | [ ] |
| `TestVerifyRejectsNonIPFamilies` (new) | same | AC-7 | [ ] |
| `TestApplyMatchRejectsUnknownMatch` (new) | `internal/plugins/firewall/vpp/translate_test.go` | AC-9 | [ ] |
| `TestApplyAllVerifiesRegistryTables` (new) | `internal/component/firewall/registry_test.go` | AC-8 | [ ] |

RFC-tagged tests that change and need owner approval (Q6): `TestDataplaneApplyIPv4SetsRecipe` (tags RFC3768-8.2-1, RFC9568-8.1.2-1, RFC5798-8.1.2-1, and the 6.4.3-1 rows) and `TestDataplaneRestoreOnLastGroup` (the same rows, negative). Under Q1 (a) no other tagged unit changes: `TestVRRPOwnerAnswersWithVirtualMACOnly`, `TestOwnerFilterWiredOnPromotionForEveryFamily` and `TestOwnerFilterTablesDropParentAnswersForOwnedAddresses` keep their bodies. Under Q1 (b) `TestOwnerFilterTablesDropParentAnswersForOwnedAddresses` (untagged) changes its term count; under Q1 (c) every owner-filter test above changes or is deleted, and that list returns to the owner.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no numeric input added | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestVRRPOwnerRequestsCarryNoPhysicalMACForOwnedAddress` (new, tag RFC9568-8.1.2-3 positive and negative, discrimination record on `parentSysctls`) | `internal/plugins/vrrp/owner_request_integration_linux_test.go`, QEMU guest | AC-1, AC-2, AC-3 | [ ] |
| IPv6 sibling (if Q3 yes, tag RFC9568-8.2.2-5) | same file | AC-10 | [ ] |
| owner-request path in the `vrrp-keepalived` lab | `internal/le/test/qemu/vrrp_owner_linux.go` | the owner pings the keepalived peer from the owned address and the peer's neighbour table keeps the virtual MAC | [ ] |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `vrrp-v2-owner-keepalived` (extended) | `test/interop/scenarios/vrrp-v2-owner-keepalived/` | keepalived | Ze stays owner; after Ze sends traffic from the owned address to keepalived's address, keepalived's host neighbour entry for `172.30.0.60` holds the virtual MAC `00:00:5e:00:01:14` | [ ] |
| `vrrp-mastership-keepalived` | `test/interop/scenarios/vrrp-mastership-keepalived/` | keepalived | unchanged mastership with the new recipe | [ ] |

### Goal Validation
| Goal | Evidence owed |
|------|---------------|
| LAN hosts never learn the owner's physical MAC for the owned address | the QEMU capture (AC-1) with its red control (AC-3), and the keepalived peer's neighbour entry in the interop scenario |
| The owner keeps reaching its LAN | AC-2 in the guest, the interop scenario staying green |
| vpp never applies a table it cannot express | AC-8 negative test with its unsupported-match error |

## Files to Modify

- `internal/plugins/vrrp/dataplane_linux.go` - `parentSysctls` gains `arp_announce` (Q1 a)
- `internal/plugins/vrrp/doctor.go` or `groups.go` - Q2 rule
- `internal/plugins/vrrp/ownerfilter.go` - package comment sentence on requests (stale-comment rule)
- `internal/plugins/firewall/vpp/verify.go` - family check, three match cases
- `internal/plugins/firewall/vpp/translate.go` - `applyMatch` default arm, `translateTerm` comment
- `internal/component/firewall/registry.go` - `ApplyAll` runs the active backend's verifier
- `internal/le/test/qemu/vrrp_owner_linux.go` and the interop scenario check for the owner-request path
- tests named in the TDD plan; `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md`; `docs/architecture/firewall/fw-6-firewall-vpp.md`; journal rows 94 and 95 Fix cells

## Files to Create

- `internal/plugins/vrrp/owner_request_integration_linux_test.go`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new leaf |
| YANG validation constraints | No | no new leaf |
| YANG custom validators | Q2 (i) only | `internal/plugins/vrrp/groups.go` `validateGroups` |
| CLI commands/flags | No | none |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | N-A | no leaf |
| Functional test for new RPC/API | N-A | no RPC; the QEMU test covers the path |
| Pipe completeness | N-A | no output |
| Env var registration | No | none |
| Doctor check for runtime dependencies | Yes | `arp_announce` procfs write: `internal/plugins/vrrp/doctor.go`, and the Q2 (ii) warning |
| Prometheus counters/metrics | No | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the VRRP feature page under `features/` that cites RFC 9568 owner behaviour |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | behaviour only |
| 6 | Has a user guide page? | Yes | `docs/guide/vrrp.md` (owner address guidance, Q2) |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9568.md` rows 8.1.2-3 (and 8.2.2-5); `docs/features/rfc-status.md` is generated |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/qemu-integration.md` (new owner-request path), `docs/architecture/testing/interop.md` (sysctl grant) |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md`, `docs/architecture/firewall/fw-6-firewall-vpp.md`, `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` (owner section, if Q2 adds a rule); `docs/architecture/core-design.md` unaffected (registration shape unchanged) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `ownerfilter.go` declares `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md`; `verify.go` and `translate.go` declare `docs/architecture/firewall/fw-6-firewall-vpp.md`; advisory mentions (2026-10-13 run): `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` (describes `ApplyAll`; affected, update with the verifier step), `docs/guide/firewall.md` (affected only if it states registry tables apply unverified on vpp; check), `docs/architecture/firewall/firewall-irr.md` (unaffected: names `RegisterTables` as its route, behaviour unchanged on nft), `docs/functional-tests.md` (affected if the new QEMU path is listed there); rerun `./le spec citation anchors` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/vrrp.md` owner example: check address order against Q2 |

## Implementation Steps

1. **Phase: Wiring** - QEMU probe for A-1 and A-3 (one guest run), then the failing recipe test and the failing owner-request QEMU test
2. **Phase: D1** - `parentSysctls` gains `arp_announce=2`, saved and restored; lab grant; Q2 rule; doc edits
3. **Phase: D2** - verifier family and match cases, `applyMatch` default arm, `ApplyAll` verifies; doc edit
4. **Phase: D3** (if Q3 yes) - research the IPv6 source preference, then the same TDD loop
5. **Phase: Evidence** - `./le rfc discriminate-record` for the new tags, interop scenarios green, journal Fix cells

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | the owner still resolves its LAN (AC-2) in the same run that proves AC-1 |
| Data flow | registry tables and config tables reach the same verifier |
| Rule: rfc-compliance | the new tag claims only what the capture checks; discrimination recorded |
| Rule: stale-comments | `ownerfilter.go` package comment and the dataplane doc sentence on requests |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| D1 fix | QEMU test output pasted, red and green |
| D2 fix | vpp and firewall unit tests output pasted |
| Interop | `./le test integration` owner scenario output |

### Security Review Checklist
| Check | What to look for |
|-------|------------------|
| Input validation | no new input; the verifier now rejects what it silently accepted |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A-1 broken (rewrite works) | DESIGN: Q1 reopens with the rewrite as a candidate |
| A-3 broken | DESIGN: Q1 (c) |
| Test fails on behavior mismatch | re-read Current Behavior |

## Design Insights

- The journal row framed D1 as a firewall term; the obligation is on the sender the kernel chooses, which a sysctl governs.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Steer the sender with `arp_announce=2` (Q1 a, pending) | drop term, sender-MAC rewrite, address move, deviation | the only shape that keeps the owner's resolution and adds no firewall match |
| Verify registry tables in `ApplyAll` (Q4 a, pending) | per-plugin verification; vpp lowering | one gate for every table a backend applies |

## Known Limitations
- Owners with no non-virtual address in the subnet: Q2.

## RFC Documentation (Scope: protocol)

Add `// RFC 9568 Section 8.1.2: "..."` above the `arp_announce` recipe entry, quoting the SHOULD NOT sentence in the Defects table.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete
- [ ] `./le verify worktree` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + journal Fix cells
- [ ] **Commit B:** remove the spec
