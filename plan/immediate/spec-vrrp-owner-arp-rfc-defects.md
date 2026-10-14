# Spec: vrrp-owner-arp-rfc-defects

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Commit `850eb41b66` stopped the VRRP address owner's parent from answering ARP
and ND for the virtual address with its physical MAC. Two halves were recorded
open in `plan/journal/guard-added-to-one-half-of-a-pair.md` on 2026-09-27 (the
`ownerfilter.go` row and the vpp `verifyMatch` row). Both are re-confirmed open
in HEAD `3e2770062b` on 2026-10-13; no commit after `850eb41b66` touches
`ownerfilter.go` or the vpp `verify.go`.

Research on 2026-10-13 changed both rows' shape. Neither repair the journal
names for D1 (drop or rewrite) keeps the owner able to resolve its own LAN
neighbours, and D2 as written ("refused with a generic error") describes a path
the VRRP tables never take: plugin-registered tables skip the vpp verifier, and
the vpp translator silently ignores a match it does not know. Design review r1
(2026-10-14) added the boot and restart window of RFC 9568 Section 8.1.2, which
the 2026-10-13 design proved only in the steady state, and a false
`{not-applicable}` pair in `rfc/short/rfc9568.md`, which commit `bd9fbb5989`
has since corrected to `{gap}` (RFC9568-8.2.2-2 and -3; RFC5798-8.2.2-3
already read `{gap}`). Design review r2 (2026-10-14) settled the boot order at
the producers: iface installs the owned address before VRRP builds, which Q7
puts to the owner. The design below answers all of it, and Owner Questions
carry the decisions it cannot make.

## Defects

| ID | Source | Verbatim quote | Producer | What Ze does wrong | Audit verdict / record | Status in HEAD |
|----|--------|----------------|----------|--------------------|------------------------|----------------|
| D1 | RFC 9568 Section 8.1.2 (RFC9568-8.1.2-3, SHOULD NOT); restates RFC 5798 Section 8.1.2 (RFC5798-8.1.2-2) and RFC 3768 Section 8.2 (RFC3768-8.2-2), both `superseded` onto 8.1.2-3 | "When a VRRP Router restarts or boots, it SHOULD NOT send any ARP messages using its physical MAC address for an IPv4 address for which it is the IPv4 address owner (as defined in Section 1.7), and it should only send ARP messages that include Virtual Router MAC addresses." | `internal/plugins/vrrp/ownerfilter.go` `ownerFilterTables` emits `ownerARPReplyTerm` only (one IPv4 term per owned address, ARP operation reply), published at promotion by `doInstallVIPs` (`instance.go`); `internal/plugins/vrrp/dataplane_linux.go` `parentSysctls` sets `arp_ignore=1`, `arp_filter=1`, `rp_filter=1`, applied by `engine.build` before the worker starts, and leaves `arp_announce` at the host value | Two windows. Steady state: an ARP request the parent sends while resolving a LAN neighbour, whose sender protocol address is the owned address, leaves with the parent's physical MAC as sender hardware address. Boot and restart: the iface component installs the owned address on the parent before VRRP builds the instance (the instance waits for a usable parent address, `parentReady`), and the owner filter is published only at promotion, so until then the parent answers who-has for the owned address with its physical MAC and its requests carry it too. A host that receives either (the target always; any host already holding an entry for the address, by the RFC 826 merge rule, A-5) records the owned address against the physical MAC | No tag, no audit entry: `rfc/audit/rfc9568.json` has no `RFC9568-8.1.2-3` key; `./le rfc check` on 2026-10-13 prints no VRRP line | open |
| D2 | journal row of 2026-09-27 on `verifyMatch` (no RFC sentence: backend parity), widened by research to `ai/rules/principles.md` "silently wrong value" | (none) | `internal/plugins/firewall/vpp/verify.go` `verifyMatch` (no case for `MatchARPOperation`, `MatchARPSenderAddress`, `MatchNDTargetAddress`; `verifyTable` never reads `Table.Family`); `internal/plugins/firewall/vpp/translate.go` `applyMatch` (no default arm) and `translateTerm`; `internal/component/firewall/registry.go` `ApplyAll` (merges every owner's tables and hands them to `Backend.Apply` with no `RunVerifier`) | Commit-time: the three matches fall to "match type ... not recognized by backend vpp", which no operator config can reach today, because no YANG leaf produces them (`plan/spec-firewall-arp-nd-matches.md`, status ready, not implemented). Runtime: the VRRP owner tables reach `firewall.RegisterTables` then `ApplyAll` then the vpp `Apply`, which never runs `Verify`; `applyMatch` returns nil for every unknown match, and the arp family is translated as an IP ACL. An owner term (output interface, ARP operation, sender address, drop) becomes an ACL rule with no constraint and a deny verdict, on the output hook | journal row, not fixed | open; runtime reachability is A-2 |
| D3 (proposed, Q3) | RFC 9568 Section 8.2.2 (RFC9568-8.2.2-5, SHOULD NOT; siblings RFC9568-8.2.2-2 MUST and RFC9568-8.2.2-3 MUST NOT) | "When a VRRP Router restarts or boots, it SHOULD NOT send any ND messages with its physical MAC address for the IPv6 address it owns and it should only send ND messages that include Virtual Router MAC addresses." and "When an Active Router sends an ND Neighbor Solicitation message for a host's IPv6 address, the Active Router MUST include the Virtual Router MAC address for the Virtual Router if it sends a source link-layer address option in the Neighbor Solicitation message.  It MUST NOT use its physical MAC address in the source link-layer address option." | `ownerfilter.go` `ownerNDAdvertTerm` (Neighbor Advertisements only); no Ze code steers the Neighbor Solicitations the Linux IPv6 neighbour discovery sends from the parent on Ze's behalf | The IPv6 sibling of D1: a Neighbor Solicitation the parent sends carries the physical MAC in its Source Link-Layer Address option, and the solicited host records it. For -2 and -3 this holds for every Active Router, owner or not | No tag. `rfc/short/rfc9568.md` marks -2 and -3 `{gap: the Linux kernel sends the Active Router's Neighbor Solicitations on ze's behalf ...}` since commit `bd9fbb5989`; `rfc/short/rfc5798.md` marks the superseded twins RFC5798-8.2.2-2 and -3 `{gap: ...}` | open (-5 not in the skeleton's scope; the -2/-3 ledger already states the gap whatever Q3 answers) |

## What a fix must prove

| Obligation | Detail |
|------------|--------|
| Failing test first | D1 steady state: a QEMU guest test capturing the parent's own ARP request while it resolves a LAN host. D1 from build: an in-package Go integration test in the QEMU guest (the shape of `owner_answer_integration_linux_test.go`) capturing every ARP message from before the build-time publication, while a LAN peer sends who-has for the owned address, until the owned address sits on the macvlan. D1 boot and restart, as goal evidence: the `vrrp-keepalived` lab capturing from before ze starts, across a ze restart. D2: a vpp verifier and translator unit test per match and per family, and a registry test per owner. D3: the IPv6 capture, if Q3 is yes |
| Both polarities | D1: the owner still resolves the LAN host (the request is answered and the neighbour entry reaches REACHABLE); no request or reply carries the owned address with the physical MAC. D2: each match refuses with its own reason; a known match still translates; another owner's tables still apply |
| Discrimination | each QEMU test red with its fix reverted. Under Q7 (b) (owner, 2026-10-14) no test carries an RFC9568-8.1.2-3 tag: the row stays `{gap}` because the pre-build window is unmet, and an ordinary `{gap}` forbids polarity tags (`docs/contributing/rfc-conformance-gates.md`, "Clause-scoped evidence"), so the in-package from-build test is untagged goal evidence whose red (the `syncOwnerFilter` call removed from `engine.build`) and green output is pasted. Under Q7 (a) the row can be met, the from-build test carries the tag, and its red is recorded with `./le rfc discriminate-record` the way `850eb41b66` recorded the reply term. The steady-state test and the lab boot and restart capture are untagged under either answer (no lab file carries an RFC tag), and their red and green output is pasted |
| Real entry point | the `vrrp-keepalived` QEMU lab (`internal/le/test/qemu/guestlabs.go`, path `v2-owner-keeps-the-address`, a new owner-request path and a new owner-boot path) |
| Interop | `vrrp-v2-owner-keepalived` and `vrrp-mastership-keepalived` stay green, and the owner scenario gains the request assertion with its revert-red record (Interop Tests) |

## Owner Questions

| Q | Question | Options | Recommendation |
|---|----------|---------|----------------|
| Q1 | D1 shape. The journal's two repairs both break the owner's own neighbour resolution (R-1, R-2). Which shape? | (a) parent `arp_announce=2` per VRRP parent device (never `all`, `default` or a loopback), so the kernel picks the parent's primary address in the target's subnet as sender; the owner filter and the parent recipe are applied when the instance is built rather than at promotion (boot-window design, Key Design Decisions). (b) (a) plus a drop term for requests still carrying the owned address. (c) move the owned address off the parent while Active, the macvlan becoming its holder at the subnet prefix, which removes the owner filter. (d) record a deviation from the SHOULD NOT. The consequence of each option is the delta table below | **Owner, 2026-10-14: accept (recommendation): (a).** (Recommendation revised 2026-10-14 by design review r1.) (c) does not replace (a): where the parent keeps another address in the subnet, the parent's connected route still carries the owner's LAN traffic, and the kernel's sender choice still names the owned address unless `arp_announce=2` is set (A-6), so (c) is (a) plus an address move. Its only extra coverage is the Q2 configuration, and for it (c) costs a new iface capability, the competing connected route `vipMaskBits` exists to avoid, a boot window it does not close, and 38 RFC tag lines. The owner's ARP sysctl constraint is answered by the inventory below: (a) writes one per-device knob at the value the `dsr` profile already writes, and its one conflict (`proxy_arp=1`) is one VRRP's `arp_ignore=1` already carries |
| Q2 | Under (a), an owner whose owned address is the parent's only address in that subnet, or its primary one, still sends the physical MAC with the owned address: no other sender exists. What does Ze do for that configuration? The reading is per address (`ownedVIPs`), not per group (`IsOwner`, which `ownsAny` sets when any VIP is real); the two readings disagree for a group holding a real and a non-real VIP, recorded in `plan/journal/field-carries-two-meanings.md` (2026-10-14) and not decided here | (i) refuse at commit: each owned address needs a non-virtual parent address listed first in the same subnet. (ii) accept, and report it as a `ze doctor` warning naming the SHOULD NOT; RFC9568-8.1.2-3 then carries a deviation annotation that keeps its section and reason, and that configuration is never counted conformant. (iii) take Q1 (c) for that case only: two owner dataplane shapes, and every (c) cost in the delta table for the case that needs it | **Owner, 2026-10-14: accept (recommendation): (ii).** The RFC sentence is a SHOULD NOT scoped to restart and boot, and its own list asks for "an IPv4 address known to belong to that router SHOULD be used" (RFC 9568 Section 8.1.2, third bullet). Refusing (i) rejects a configuration RFC 9568 Section 1.7 defines as the normal owner. (iii) is the only conformant answer for that configuration, at the cost stated |
| Q3 | Take D3 (RFC9568-8.2.2-5) and the Neighbor Solicitation defect of RFC9568-8.2.2-2/-3 in this spec? The -2/-3 rows already read `{gap}` (commit `bd9fbb5989`) whatever the answer | yes / no | **Owner, 2026-10-14: accept (recommendation): yes.** Same mechanism and the same test lab; IPv6 has no `arp_announce`, so its answer is the IPv6 source-address and SLLAO behaviour and must be researched in this spec's next pass. Under "no", the two `{gap}` rows stay published as gaps on `docs/features/rfc-status.md`, -5 stays untagged, and one journal row records the open NS defect for every Active Router |
| Q4 | D2 shape. `ApplyAll` builds one merged set over every registered owner (the firewall config, copp, flowspec, policy routes, firewall-irr, domain, the VRRP accept and owner tables). A verifier that refuses the merged set and calls no `Apply` lets one VRRP owner table on the vpp firewall backend stop the operator's whole firewall from reconciling | (a) verify the merged set in `ApplyAll`, refuse it whole. (b) verify per owner: the active backend's verifier judges each owner's tables as that owner registered them, before `mergeSameNameTables` (`registry.go`) builds the merged set, so a conflict that only the merge creates is not judged here; a refused owner's tables are left out of the merged set and its error names the owner, and every other owner still applies, following the precedent of `dropTablesMissingAProvidedSet`, which already leaves one table out rather than failing the whole reconcile; plus `applyMatch` refuses an unknown match and the vpp verifier gives each ARP/ND match and each non-ip family its own reason. (c) refuse at commit a VRRP address-owner group while the firewall backend is vpp, in `validateGroups`, beside its existing refusal of the vpp interface backend. (d) lower the matches on vpp | **Owner, 2026-10-14: accept (recommendation): (b) and (c) together, (c) only if A-2 holds.** (b) repairs the class for every registry owner without letting one plugin stop another's firewall, which (a) does. (c) gives the operator a commit-time refusal: under (b) alone the owner filter, which carries the RFC9568-8.1.2-1 MUST NOT for the owner, would only fail at promotion with a log line. (d) is out: VPP's ACL plugin matches IP headers. (c) holds only if A-2 is validated (VRRP on netlink with the vpp firewall backend is reachable); if A-2 is broken, (c) drops and (b) stands |
| Q5 | Under Q1 (a) the recipe writes `arp_announce` on the parent, which the operator can also set per unit (`internal/component/iface/config_sysctl.go`, the `arp-announce` leaf). Which wins? | the VRRP recipe, as it does for `arp_ignore` today / the operator's value, with a doctor warning | **Owner, 2026-10-14: accept (recommendation): the VRRP recipe**, same as `arp_ignore` (`reassertDataplaneSysctls`), so one rule governs the knobs, restored on the last group. The kernel uses the higher of `conf/all` and `conf/<device>`, so an operator `all` value of 2 is unaffected and a lower per-device value is the only thing overridden |
| Q6 (re-issued 2026-10-13, list corrected 2026-10-14) | RFC-tagged tests change, and the list depends on Q1 (full list in the TDD plan). RFC 9568 Section 8.1.2: "The Active Router MUST NOT respond with its physical MAC address in the ARP response." (RFC9568-8.1.2-1) and "When a VRRP Router restarts or boots, it SHOULD NOT send any ARP messages using its physical MAC address for an IPv4 address for which it is the IPv4 address owner ..." (RFC9568-8.1.2-3). Under every answer: `TestDataplaneApplyIPv4SetsRecipe` (6 tag lines) has its prose corrected, and `TestDataplaneRestoreOnLastGroup` (6 negative tag lines) has its tags withdrawn as wrong polarity. Under Q1 (a): `TestOwnerFilterWiredOnPromotionForEveryFamily` (`ownerfilter_test.go`, 10 lines) changes its body, because the filter is published at build rather than at promotion. Under Q1 (c): 38 more tag lines change or go with the owner filter: `TestVRRPOwnerAnswersWithVirtualMACOnly` (`owner_answer_integration_linux_test.go`, 20 lines: RFC3768-8.2-1, RFC3768-8.2-4, RFC5798-8.1.2-1, RFC5798-8.1.2-6, RFC5798-8.2.2-1, RFC5798-8.2.2-8, RFC9568-8.1.2-1, RFC9568-8.1.2-6, RFC9568-8.2.2-1, RFC9568-8.2.2-8, each positive and negative), `TestOwnerFilterWiredOnPromotionForEveryFamily` (10 lines: RFC3768-8.2-1, RFC5798-8.1.2-1, RFC5798-8.2.2-1, RFC9568-8.1.2-1, RFC9568-8.2.2-1, each positive and negative) and `TestInstanceOwnerStartupGoesMaster` (`instance_test.go`, 8 lines, among them RFC3768-6.4.3-4 positive "the owner Master installs the VIP (a real parent address)", which (c) makes false). Their negative control ("without the filter the parent answers ... with its physical MAC") needs a new form, and the owner proof of 8.1.2-1 and 8.2.2-1 MUST survive on the address-move path. Approve the list for the Q1 answer given? | yes / no | **Answered by the owner, 2026-10-13: "approve all RFC-tagged test changes".** Q1 was answered (a) on 2026-10-14; the approval covers the Q1 (a) list. The commands are in the TDD plan under "Owner approval commands" |
| Q7 (new, design review r2, 2026-10-14) | At boot the iface component installs the owned address on the parent before VRRP builds the instance, and nothing VRRP owns runs earlier. Read at the producers: `vrrp` declares `Dependencies: []string{"interface"}` (`internal/plugins/vrrp/register.go`); `runPluginPhase` (`internal/component/plugin/server/startup.go`) runs the whole five-stage handshake of one dependency tier before the next tier begins; iface's `OnConfigure` (`internal/component/iface/register.go`) runs `applyAndPublish`, whose `applyConfig` calls `AddAddress` for every configured address; only then does VRRP's `OnConfigure` call `engine.apply`, then `build`, which itself waits on an iface reconcile for the macvlan (`waitDevicePresent`). An orderly stop leaves the address on the parent (iface calls `RemoveAddress` only in reconcile and migration, never at shutdown) while the shutdown flush and `revertDataplane` withdraw the protection (Key Design Decisions row 3), so a restart is a boot and opens the same window. From the iface install to `build`, the parent answers who-has for the owned address with its physical MAC, and its requests may name it as sender. How is that window handled? | (a) Close it. VRRP registers an address claim with iface in `init()` (a new iface capability, registered, so iface spells no VRRP name): iface withholds from install a configured address that a VRRP group of the same unit lists as a virtual address. VRRP's `build` publishes the owner filter and the recipe, then hands the address back through `iface.RegisterOwnedAddresses` on the parent under the instance owner, and `teardown` withdraws it, so the address leaves the parent at an orderly stop. Cost: a new iface capability with its tests and doc; the owner's own real address now depends on the VRRP build succeeding, so a failed build leaves the router without the address it owns (and RFC 9568 Section 1.7 then makes it no owner); every orderly stop takes the address and the router's own traffic from it down with ze; whether iface reconciles once more during shutdown, so the address actually leaves, is unread; the Q1 (c) delta needs the same withhold. (b) Record the residual window. RFC9568-8.1.2-3 is annotated `{gap: ...}`, its reason stating that Ze meets the sentence from `engine.build` on (`syncOwnerFilter`) and not in the interval from the iface address install to `build`, at every boot and restart. The record shape is decided here (design review r3, read in `docs/contributing/rfc-conformance-gates.md`): an ordinary `{gap}` forbids polarity tags, so a narrowed tag beside a gap note is refused; the one clause-scoped form, `{partial: tested "<span>"; gap "<span>"; ...}`, needs two non-overlapping verbatim spans of the quote, and the pre-build interval is a time window inside "When a VRRP Router restarts or boots", not a clause of the sentence, so no selector pair states it honestly. The from-build Go test (AC-7) and the lab capture (AC-8) are therefore both untagged goal evidence, their red and green output pasted, and the lab capture measures the interval | **Owner, 2026-10-14: accept (recommendation): (b).** The obligation is a SHOULD NOT, and (b) closes everything VRRP controls: today the promotion-time filter leaves the whole of Initialize and the macvlan wait open, and under (b) the parent is silent for the owned address from `build`. (a) changes when every owner holds its own address, adds a failure mode in which a VRRP build error removes an operator's configured address, and buys only the interval from the iface tier to the VRRP tier's configure. Consequence under (b): RFC9568-8.1.2-3 stays a `{gap}` row and is never counted met, no test carries its tag, and Known Limitations, the dataplane doc and the row's gap reason state the pre-build window. Consequence under (a): the row can be met, the from-build test carries the RFC9568-8.1.2-3 positive and negative tags with a discrimination record, the iface claim and its withhold tests join this spec (the from-build test then drives the withhold too), and AC-8 asserts no physical-MAC message at all |

### Q1 option deltas

The body of this spec (wiring, ACs, Files, steps, Key Design Decisions) is
written for (a). Each other option changes it as follows.

| Option | Consequence | Delta against the (a) body |
|--------|-------------|----------------------------|
| (a) | Fixes requests wherever the parent holds a non-virtual primary address in the target's subnet (A-3). Leaves the Q2 configuration non-conformant to the SHOULD NOT, decided by Q2. Adds one per-device knob to VRRP's sysctl footprint. Closes the window from `build` to Active by publishing the protection before the transport opens; the interval from the iface address install to `build`, at every boot and restart, stays open unless Q7 (a) is taken (A-7, A-8) | none |
| (b) | Starves the owner's resolution wherever (a) cannot steer the sender (R-1), so it is safe only with Q2 (i) | adds a request-drop term to `ownerFilterTables`, an AC that the owner's resolution in the Q2 configuration is refused at commit, and `TestOwnerFilterTablesDropParentAnswersForOwnedAddresses` (untagged) changes its term count |
| (c) | Covers the Q2 configuration (sole holder on the macvlan: its requests leave with the virtual MAC as sender hardware address). Still needs `arp_announce=2` where the parent keeps another address in the subnet (A-6), so it is (a) plus a move. Brings back the competing connected route `vipMaskBits` installs /32 to avoid: in the tested owner configs (A-4: `.2/24` and `.60/24` on the parent) the parent keeps the subnet through `.2`, so the macvlan's `.60/24` is a second connected route for it (the shape every non-owner group already runs). Takes the configured parent address away from the iface component: `iface.RegisterOwnedAddresses` only adds addresses to `desiredState`, so iface needs a new capability to withhold a configured address while a VRRP owner is Active, and iface reconciles would otherwise put it back. Does not close the boot window, because iface installs the address at boot before VRRP builds, unless iface also withholds it from the start | replaces the owner filter with the address move; adds the iface withhold capability, its wiring row and AC; `vipMaskBits` and its comment change; the 38 tag lines in Q6 change, with their approval rows; AC-4 to AC-6 stay (the recipe still carries `arp_announce`) |
| (d) | No change to Ze's behaviour. An approved deviation keeps its RFC section and reason in the scope record and is never counted conformant (`ai/rules/rfc-compliance.md`) | removes AC-1 to AC-6 and the D1 tests; adds the deviation annotation on RFC9568-8.1.2-3 and the doc pages' statement of it; D2 stands, and Q7 drops |

-> Constraint (owner, 2026-10-13, on Q1 and Q5): "we have to be careful about playing with linux arp changes as to accept some loopback locally we may need to change arp too and it may conflict." The `arp_*` sysctls are shared by every feature that answers for an address it holds on another interface (a loopback or anycast address answered locally, the LVS direct-routing pattern that sets `arp_ignore`/`arp_announce` for a VIP on `lo`). How keepalived, FRR vrrpd and VyOS treat the same conflict is being researched.

-> Decision (design review r1, 2026-10-14): the inventory the constraint asked for, read at the producers.

| Writer | Knobs and scope | Owner of the knob on a VRRP parent |
|--------|-----------------|------------------------------------|
| VRRP `parentSysctls` (`dataplane_linux.go`) | `arp_ignore=1`, `arp_filter=1`, `rp_filter=1` on the parent device; under (a) also `arp_announce=2` | VRRP while a group runs, restored on the last group |
| VRRP `vmacSysctls` | `arp_ignore=1` (8 for an IPv6 group) and `rp_filter=0` on the macvlan only | VRRP |
| iface `config_sysctl.go` | per unit device: `arp_filter`, `proxy_arp`, `arp_announce`, `arp_ignore` from operator leaves, emitted on the EventBus to the sysctl plugin | the operator, overridden by the VRRP recipe (Q5) |
| sysctl profile `dsr` (`internal/core/sysctl/profiles.go`) | per device: `arp_announce=2`, `arp_ignore=1` ("Direct Server Return: ARP tuning for loopback VIP") | the operator; same values as the VRRP recipe, so no conflict |
| sysctl profiles `hardened`, `multihomed` | per device: `arp_filter=1` | same value as VRRP |
| sysctl profile `proxy` | per device: `proxy_arp=1`, `arp_accept=1` | conflicts with VRRP: `internal/core/sysctl/register_profiles.go` declares `arp_ignore=1` vs `proxy_arp=1` and `arp_announce=2` vs `proxy_arp=1` conflicts; the first already holds at HEAD |

No loopback or anycast feature in Ze writes an `arp_*` knob (a grep of every
Go and YANG file under `internal/` and `cmd/` for `arp_announce` and
`arp-announce` returns only the files above). The LVS direct-routing pattern the
constraint names sets `arp_announce=2` on the LAN interface, the same direction
as (a). Whether the sysctl plugin's conflict check sees VRRP's direct procfs
writes is unverified and read in the implementation audit.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md` - owner dataplane, "The address owner needs a filter, not a sysctl"
  → Constraint: the owned address sits on the parent at the subnet prefix and on the private macvlan as /32 or /128 (`vipMaskBits`), so the parent holds the subnet's connected route and every LAN-bound request of the owner leaves the parent
  → Decision: the doc says requests and Neighbor Solicitations from the parent "are left alone: they are its own resolution traffic"; that sentence is D1 and changes with the fix
  → Constraint: the macvlan is private mode (`internal/plugins/vrrp/register.go`, `createMacvlan`), so a unicast frame to the virtual MAC is delivered to the macvlan, never to the parent
- [ ] `docs/architecture/firewall/fw-6-firewall-vpp.md` - vpp verifier and translation contract
  → Constraint: the verifier runs at commit through `RunVerifier` (`internal/component/firewall/engine.go`) over config tables only; the translator's comment says the verifier guarantees unsupported expressions were rejected, which is false for registry tables
- [ ] `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - `ApplyAll` and what happens to registry tables at stop
  → Constraint: Rule 3, read 2026-10-14 (r2): `flush-on-shutdown` defaults true and `FlushAllTables` removes every ze-owned nft table at an orderly stop, `vrrp-owner` among them; the design keeps that (Key Design Decisions row 3), so A-8 is settled and a restart is a boot
- [ ] `docs/architecture/testing/interop.md` - VRRP scenarios, lab sysctl grants
  → Constraint: the VRRP scenarios' ze writes only lab-granted sysctls (`TestVRRPWritesOnlyLabGrantedSysctls`, `internal/plugins/vrrp/lab_profile_linux_test.go`), and the grant is every key under `net/` (`NetSysctlGranted`, `internal/le/interoplab/apparmor_netsysctl.go`), so `arp_announce` needs no new grant
- [ ] `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` - declared by `groups.go`, `doctor.go`, `instance.go`, `register.go`; component map and owner auto-detection
  → Constraint: owner detection forces priority 255 and `accept-mode true` (section "Owner auto-detection forces the effective values"); a Q2 rule reads the same `realAddresses` detection and changes the page's owner section
  → Constraint: the dataplane row names `dataplane_linux.go` as "the macvlan ARP-ownership recipe"; the `arp_announce` knob is added to that recipe, so the row stays true and the recipe list in the dataplane doc changes
- [ ] `docs/architecture/core-design.md` - declared by `internal/component/firewall/registry.go`, `backend.go`, `engine.go`
  → Constraint: components reach backends through registries; the per-owner verification looks up the active backend's registered verifier by name and spells no backend
- [ ] `docs/architecture/testing/qemu-integration.md` - the `vrrp-keepalived` guest lab
  → Constraint: kernel ARP behaviour is proven in the QEMU guest, never in a host namespace (root in agent sessions is limited to the stress harness, `.claude/rules/sudo.md`)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9568.md` - rows 8.1.2-1 to -6 and 8.2.2-1 to -8
  → Constraint: 8.1.2-3 and 8.2.2-5 are SHOULD NOT rows in the Compliance Checklist, untagged; adding a tag owes a discrimination record (`ai/rules/rfc-compliance.md`)
  → Constraint: 8.2.2-2 and 8.2.2-3 read `{gap: the Linux kernel sends the Active Router's Neighbor Solicitations on ze's behalf ...}` since commit `bd9fbb5989` (they carried a false `{not-applicable}` before it); nothing in this spec edits them unless Q3 is yes and the fix steers the SLLAO
- [ ] `rfc/short/rfc5798.md`, `rfc/short/rfc3768.md` - superseded rows 5798-8.1.2-2, 3768-8.2-2, 5798-8.2.2-2/-3
  → Constraint: both are `superseded` onto RFC9568-8.1.2-3, so the tag goes on the 9568 row only
  → Constraint: RFC5798-8.2.2-2 and RFC5798-8.2.2-3 both read `{gap: ...}` (8.2.2-3: "the same unbuilt path as RFC5798-8.2.2-2"), the same reading the 9568 rows now give
- [ ] `rfc/full/rfc9568.txt` Sections 1.7, 8.1.2, 8.2.2, read 2026-10-14
  → Constraint: Section 8.1.2: "Note that the source address of the Ethernet frame of this ARP response is the physical MAC address of the physical router." The obligation is on the ARP payload's sender hardware address, not the Ethernet source
  → Constraint: Section 8.1.2, third bullet: "When, for example, Secure Shell (SSH) access to a particular VRRP Router is required, an IPv4 address known to belong to that router SHOULD be used." The RFC expects a router address other than the virtual one for the router's own traffic, which is what Q1 (a) steers to
  → Constraint: Section 8.1.2 scopes RFC9568-8.1.2-3 to "When a VRRP Router restarts or boots"; a test that checks the steady state alone does not prove it
  → Constraint: Section 1.7: the IP Address Owner is "The VRRP Router that has the Virtual Router's IPvX address(es) as real interface address(es)."
- [ ] `rfc/full/rfc826.txt` - fetched in the Wiring phase, step 1 (absent today; fetching a text is allowed under the 2026-10-08 RFC scope freeze, enrolment is not)
  → Constraint: A-5 cites RFC 826's packet reception ("merge flag"), which must be quoted from the text, not from memory

**Key insights:**
- D1 cannot be fixed by a firewall term alone: drop starves the owner's own resolution, rewrite sends the reply to the macvlan (R-1, R-2)
- The sender protocol address the kernel puts in a request is chosen by `arp_announce`; level 2 picks the primary address of the outgoing interface in the target's subnet
- RFC9568-8.1.2-3 is a boot and restart obligation: the parent holds the owned address before VRRP runs (iface's tier configures first, Q7), so the protection is published in `build`, the earliest point VRRP controls, and a restart is a boot because an orderly stop withdraws it
- D2's real defect is the runtime path: registry tables reach `vpp.Apply` unverified and unknown matches vanish in `applyMatch`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/vrrp/ownerfilter.go` (265L) - `ownedVIPs`, `setOwnerFilter`, `clearOwnerFilter`, `ownerFilterTables`, `ownerARPReplyTerm`, `ownerNDAdvertTerm`, `ownerFilterTable`; one arp table `ze_vrrp_owner_arp` and one ip6 table `ze_vrrp_owner_nd`, output hook, policy accept, owner `vrrp-owner`
  → Constraint: published through `firewall.RegisterTables` then `firewall.ApplyAll`; the backend is whichever the firewall component loaded or the OS default
- [ ] `internal/plugins/vrrp/register.go` - `vipCIDRs`, `vipMaskBits` (owned address installed as a host route, because a subnet-prefix copy "would add a second, competing connected route for the subnet"), macvlan mode private, `parentReady` (link up and a usable address of the family)
  → Constraint: the parent keeps the subnet's connected route for an owner
- [ ] `internal/plugins/vrrp/engine.go` - `build` creates the macvlan, applies the parent recipe (`applyDataplane`), opens the transport, all before the worker starts; an `openInstance` failure unwinds with `revertDataplane` and `deleteMacvlan` ("a failed build leaves no kernel state"); `apply` reconfigures a running instance on the same netdev in place (`dev == in.dev`, and the unresolved-ifindex branch) through `in.reconfigure`, and rebuilds only on a genuine move; `teardown` runs `in.shutdown`, `closeInstance`, `revertDataplane`, `deleteMacvlan`; `stopAll` tears every instance down at plugin stop
  → Decision: the boot-window design publishes the owner filter in `build`, after the recipe and before `openInstance`, through `syncOwnerFilter` (Key Design Decisions row 2)
- [ ] `internal/plugins/vrrp/fsm/fsm.go` - `masterConfigUpdated` emits `InstallVIPs` when a Master's VIPs change (and when accept-mode flips), so today `doInstallVIPs` republishes `ownedVIPs` on a reconfigure
  → Constraint: with the promotion call removed, `reconfigure` is the only place a changed owned set reaches the filter, in every state
- [ ] `internal/component/plugin/server/startup.go` `runPluginPhase`, `internal/component/iface/register.go` `OnConfigure` - read for A-7 (r2): the iface tier's configure, which installs every configured address, completes before the VRRP tier's configure begins
- [ ] `internal/plugins/vrrp/instance.go` - `doInstallVIPs` calls `setOwnerFilter` with `ownedVIPs` before install; `doRemoveVIPs` calls `clearOwnerFilter` after removal; `evaluateReadiness` waits for `parentReady`; `reconfigure` swaps the spec and dispatches `ConfigUpdated` under `mu`
- [ ] `internal/plugins/vrrp/ownerfilter.go` `setOwnerFilter` - an empty set withdraws the entry, and an unchanged parent and set returns without a reconcile, so republishing an unchanged owned set costs nothing
- [ ] `internal/plugins/vrrp/groups.go` - `validateGroups` refuses VRRP when `/interface/backend` is vpp; `IsOwner` is `ownsAny` (any VIP real), read by `EffectivePriority`, `EffectiveAcceptMode`, the track refusal and the v2 owner discard
- [ ] `internal/plugins/vrrp/dataplane_linux.go` - `parentSysctls` (arp_ignore=1, arp_filter=1, rp_filter=1), `vmacSysctls`, `applyDataplaneSysctls`, `reassertDataplaneSysctls`, `restoreParentSysctls`
  → Constraint: shared parent knobs are saved on the first group and restored on the last (`applySharedSysctls`, `restoreDataplaneSysctls`); a new knob joins that save and restore
- [ ] `internal/component/iface/address_owner.go` - `RegisterOwnedAddresses` adds an owner's addresses to `desiredState`; it cannot withhold a configured one (Q1 (c) delta)
- [ ] `internal/plugins/firewall/vpp/verify.go` (400L) - `Verify`, `verifyTable` (sets, flowtables, chains; no family check), `verifyMatch` (default "not recognized by backend vpp")
- [ ] `internal/plugins/firewall/vpp/translate.go` - `translateTerm`, `applyMatch` (switch with no default arm), `chainToACLRules`
- [ ] `internal/plugins/firewall/vpp/backend_linux.go` - `Apply`, `applyAll` (translates every base non-NAT chain, no family read), `bindAllACLs` (hook to input or output)
- [ ] `internal/component/firewall/registry.go` - `RegisterTables` (already refuses a table without the ownership prefix, per owner), `ApplyAll` (merges every owner, no verifier call)
- [ ] `internal/component/firewall/engine.go` - `RunVerifier` at commit over `cfg.Tables` only
- [ ] `internal/plugins/vrrp/owner_answer_integration_linux_test.go` - `TestVRRPOwnerAnswersWithVirtualMACOnly`, QEMU guest, the capture harness this spec extends

**Behavior to change:** D1 and D3 (Q3 yes): no ARP request or reply (Neighbor Solicitation or Advertisement) leaves the owner's parent with the owned address and the physical MAC, in the steady state and from instance build through Active, at boot and at restart; the interval before build is Q7. D2: registry tables on the vpp backend are verified per owner, and an unknown match or family is refused, never translated to an unconstrained rule.

**Behavior to preserve:** the reply and advertisement drops and their tests; the owner resolves its LAN neighbours; non-owner groups' recipe; the save-and-restore of parent knobs on the last group; every other registry owner's tables apply when one owner is refused.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- an interface unit config holding a VRRP group whose virtual address is also a configured address of the unit (owner)

### Transformation Path
1. `groups.go` extracts the group spec with `realAddresses`; `validateGroups` refuses the vpp interface backend (and, under Q4 (c), an owner group with the vpp firewall backend)
2. the iface component installs the unit's configured addresses on the parent, the owned one among them, in its own tier's configure, before the VRRP tier's configure runs (A-7; Q7 (b), owner 2026-10-14: iface does not withhold it, and that interval stays open)
3. `engine.build` creates the macvlan and applies the parent recipe (`applyDataplaneSysctls`, with `arp_announce=2` under (a)); under the fix it then calls `syncOwnerFilter`, which hands `setOwnerFilter` the `ownedVIPs` of the spec's own VIPs, and only then opens the transport, all before the worker starts. `reconfigure` calls `syncOwnerFilter` again after each spec swap; `teardown` and `build`'s `openInstance` unwind call `clearOwnerFilter`
4. the worker waits for `parentReady`, promotes (an owner goes Active at once), and `doInstallVIPs` installs the address on the macvlan
5. the firewall registry verifies each owner's tables against the loaded backend's verifier and hands the accepted set to `Apply` (nft lowers the arp family; vpp refuses what it cannot express)
6. the kernel builds the parent's ARP requests: sender protocol address chosen by `arp_announce`, sender hardware address the parent's MAC

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| VRRP plugin ↔ firewall component | `firewall.RegisterTables` / `ApplyAll` with `firewall.Table` values | Yes, `ownerfilter.go` `setOwnerFilter` |
| firewall component ↔ vpp backend | `Backend.Apply`, no `Verifier` on this path today | Yes, `registry.go` `ApplyAll` |
| VRRP plugin ↔ kernel | procfs sysctl writes in `dataplane_linux.go` | Yes |
| iface ↔ VRRP plugin | iface installs the owned address; VRRP waits on `parentReady` | Yes, `register.go` `parentReady` |

### Integration Points
- `parentSysctls` recipe, `syncOwnerFilter` (called from `engine.build` and `instance.reconfigure`; `clearOwnerFilter` from `engine.teardown` and the `build` unwind), the firewall backend verifier registry (`firewall.RegisterVerifier`, Q4 (b))

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | registry tables bypass the backend verifier today (D2); the fix routes them through it |
| No unintended coupling (components stay isolated) | Yes | VRRP touches the firewall component only through `RegisterTables`; the verifier is looked up by backend name |
| No duplicated functionality (extends existing, does not recreate) | Yes | reuses `RunVerifier` and the parent sysctl recipe; the owner filter is published by one function, `syncOwnerFilter`, wherever the owned set can change (build, reconfigure) and cleared by `clearOwnerFilter` wherever the instance ends (teardown, build unwind) |
| Zero-copy preserved where applicable (refs, not copies) | N-A | control plane only |
| Registration over hardcoding, outbound | Yes | the verifier is already registered per backend |
| Registration over hardcoding, inbound | Yes | `ApplyAll` asks the registry for the active backend's verifier; no backend name is spelled |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|------------|---------------------------------|----------|--------------|--------|
| A-1 | A host's ARP reply to a request whose sender hardware address is the virtual MAC is sent to the virtual MAC and so is delivered to the private macvlan, leaving the parent's neighbour entry incomplete | macvlan private mode (`register.go` `createMacvlan`); Linux `arp_send` replies to the request's sender hardware address | Q1 rewrite would work and becomes a cheaper candidate | QEMU guest probe in the implementation's first phase: rewrite rule, `ip neigh` on the parent | unvalidated |
| A-2 | The vpp firewall backend can be the loaded firewall backend while VRRP runs on the netlink interface backend | no cross-component gate found: `validateBackendGate` reads only firewall YANG; `validateGroups` reads only `/interface/backend` | D2's runtime half is unreachable today and Q4 (c) drops; (b) is still owed under `principles.md` for any registry table | unit test with both backends configured, or a grep of every backend gate | unvalidated |
| A-3 | With `arp_announce=2` the kernel picks the parent's primary address in the target's subnet even when the triggering packet's source is the owned address | the Linux kernel's `Documentation/networking/ip-sysctl.rst` (upstream `https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/Documentation/networking/ip-sysctl.rst`, rendered at `https://docs.kernel.org/networking/ip-sysctl.html`; the repo has no location for kernel documentation, `reference/` holding IETF documents only), `arp_announce` level 2, verbatim: "Such local address is selected by looking for primary IP addresses on all our subnets on the outgoing interface that include the target IP address." (matched by design review r3 against a copy of the upstream file, lines 2154-2163); the kernel source (`arp_solicit`) is unread | Q1 (a) does not fix D1: Q1 reopens | the new QEMU test, both polarities | unvalidated |
| A-4 | The interop and QEMU owner configs list a non-virtual address first (`172.30.0.2` in `test/interop/scenarios/vrrp-v2-owner-keepalived/ze.conf`, `vrrpZeAddress` in `internal/le/test/qemu/vrrp_owner_linux.go`), so it is the primary | the configs and their comments | the test cannot observe the fix | read at test-writing time | validated (configs read 2026-10-13) |
| A-5 | Hosts record the sender of an ARP request: the target always, others when they hold an entry | RFC 826 packet reception ("merge flag", quoted from `rfc/full/rfc826.txt` once the Wiring phase fetches it); Linux `arp_process` | D1 is cosmetic | the QEMU peer's `ip neigh` after a parent request, before the fix | unvalidated |
| A-6 | Under Q1 (c) without `arp_announce`, a packet sourced from the owned address (now on the macvlan only) that the parent's connected route carries still leaves in an ARP request naming the owned address as sender with the physical MAC: level 0 takes the packet's source when it is local on any interface, and level 1 keeps it because it lies in a subnet of the parent | the Linux kernel's `Documentation/networking/ip-sysctl.rst` (upstream path and URL as in A-3), `arp_announce`, verbatim: level 0 "Use any local address, configured on any interface", level 1 "will preserve the source address if it is from such subnet" (matched by design review r3 against a copy of the upstream file, lines 2144 and 2151-2152); the kernel source (`arp_solicit`) is unread, so the reading stays a documentation reading | (c) alone fixes D1 and Q1 reopens with (c) as a candidate that adds no knob | the A-1/A-3 guest probe, run once with the owned address moved | unvalidated |
| A-7 | At cold boot the iface component installs the owned address on the parent BEFORE VRRP's `engine.build` runs, so the interval from the install to `build` stays open after the fix unless Q7 (a) is taken | read at the producers 2026-10-14 (r2): `Dependencies: []string{"interface"}` (`internal/plugins/vrrp/register.go`); `runPluginPhase` completes every stage of one dependency tier before the next (`internal/component/plugin/server/startup.go`); iface `OnConfigure` runs `applyAndPublish`, whose `applyConfig` calls `AddAddress` (`internal/component/iface/register.go`, `config_apply.go`); VRRP's `OnConfigure` then calls `engine.apply`, and `build` waits on an iface reconcile for the macvlan (`waitDevicePresent`) | settled; the residual is Q7 | the lab capture (AC-8) measures the interval | validated (settled the open way: Q7) |
| A-8 | An orderly stop withdraws the protection and leaves the owned address on the parent, so a restart is a boot | read 2026-10-14 (r2): `flush-on-shutdown` defaults true and `FlushAllTables` removes every ze-owned nft table, `vrrp-owner` among them (`table-ownership-and-shutdown-flush.md` Rule 3); `stopAll` then `teardown` then `revertDataplane` restores the parent knobs; iface calls `RemoveAddress` only in reconcile and migration, never at shutdown (`internal/component/iface`) | settled; the design keeps today's orderly stop (Key Design Decisions row 3) | the restart leg of the lab capture (AC-8) | validated |
| A-9 | `arp_announce` needs no new lab grant | `NetSysctlGranted` grants every key under `net/`; `TestVRRPWritesOnlyLabGrantedSysctls` seeds from `parentSysctls`, so it covers the new key unchanged | the lab refuses the write | read 2026-10-14 | validated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | A drop term for requests (journal option 1, Q1 b) starves the owner's own neighbour resolution when no other sender exists | owner cannot ping a LAN host; neighbour entry FAILED | do not ship (b) without Q2 (i) |
| R-2 | A sender-MAC rewrite (journal option 2) sends the reply to the macvlan (A-1) | parent neighbour stays INCOMPLETE | rejected unless A-1 is broken |
| R-3 | `arp_announce=2` overrides an operator's iface `arp-announce` value | doctor or log line on reassert | Q5; restored on the last group like `arp_ignore` |
| R-4 | `arp_announce=2` changes the sender of the router's ARP for non-VRRP traffic on the parent | other ARP-dependent tests change | it only narrows the sender to the interface's own primary address, the documented VRRP host recipe and the `dsr` profile's value; run the VRRP QEMU and interop sets |
| R-5 | Per-owner verification refuses a registry table that applies today on vpp | a plugin's apply error naming its owner on a vpp box | correct by `principles.md`: today that table becomes a different ACL than asked; the other owners keep applying (Q4 (b)); list every `RegisterTables` caller in the implementation audit |
| R-6 | Publishing the owner filter at build silences the parent for the owned address while the owner sits in Initialize | the owned address unanswered before promotion | intended: until the owner is Active the Backup answers for the address with the virtual MAC, and a physical-MAC answer from the parent is what RFC9568-8.1.2-3 forbids at boot; an owner leaves Initialize as soon as `parentReady` holds. The same holds in Backup: the filter now lives as long as the instance, not as long as Active, so an owner that is Backup stays silent for the owned address while the Active router answers with the virtual MAC, and the `doRemoveVIPs` comment that says the parent "has to answer for it again" is rewritten |
| R-7 | Owners whose owned address is primary or sole stay non-conformant to the SHOULD NOT (Q2) | doctor warning (Q2 (ii), owner 2026-10-14) | owner decision; the deviation keeps its section and reason and is not counted conformant |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | an owner Master that cannot resolve LAN hosts, an owner address silent too long at boot, or a vpp box applying a deny ACL; each caught by the tests below |
| How is it reverted? | single commit revert per phase |
| Who else touches this path? | `plan/spec-firewall-arp-nd-matches.md` (operator ARP/ND leaves, `ze:backend nft`, defers the vpp daemon-side refusal to this spec's D2); `plan/spec-dataplane-seams-5-copp-non-tcp.md` (non-inet families) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| owner group config, instance build | → | `parentSysctls` with `arp_announce=2` | `TestDataplaneApplyIPv4SetsRecipe` (extended) |
| owner group config, instance build | → | `engine.build` calls `syncOwnerFilter` after the recipe and before `openInstance` | `TestOwnerFilterPublishedAtBuild` (new) and `TestOwnerFilterWiredOnPromotionForEveryFamily` (body changed) |
| config commit changing a running owner's VIPs or real addresses on the same netdev | → | `engine.apply` then `instance.reconfigure` calls `syncOwnerFilter` | `TestOwnerFilterRepublishedOnReconfigure` (new) |
| owner group config, `openInstance` fails | → | `build` unwind calls `clearOwnerFilter` before `revertDataplane` | `TestBuildUnwindClearsOwnerFilter` (new) |
| owner group in the guest, parent resolves a LAN host | → | kernel request sender on the parent | `TestVRRPOwnerRequestsCarryNoPhysicalMACForOwnedAddress` (new, QEMU) |
| owner address on the parent in the guest, LAN peer sending who-has, `newEngine(...).build` then a probe before `start`, then promotion | → | `engine.build` itself: the recipe, `syncOwnerFilter`, `openInstance`, in its order | `TestVRRPOwnerSendsNoPhysicalMACARPFromBuild` (new, in-package, QEMU guest; untagged under Q7 (b), tagged RFC9568-8.1.2-3 under Q7 (a)) |
| `vrrp-keepalived` lab, ze cold start and restart | → | the whole daemon path, boot and restart | owner-boot path in `internal/le/test/qemu/vrrp_owner_linux.go` with the new `(*vrrpLab).restartZe` (untagged goal validation) |
| registry tables of two owners on the vpp backend | → | per-owner verification in `ApplyAll` | `TestApplyAllVerifiesEachOwner` (new, `internal/component/firewall`) |
| VRRP owner tables handed to vpp | → | `verifyTable` family check, `verifyMatch` cases | `TestVerifyRejectsARPAndNDMatchesWithReason` (new, vpp) |
| owner group config with the vpp firewall backend (Q4 (c), if A-2 holds) | → | `validateGroups` | `TestValidateGroupsRefusesOwnerOnVPPFirewall` (new) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | owner Active whose parent holds a non-virtual primary address in the subnet; the parent resolves a LAN host by pinging it from the owned address | every ARP request captured on the LAN from the parent's physical MAC carries the primary address as sender, none the owned address; the peer's neighbour entry for the owned address stays on the virtual MAC |
| AC-2 | same setup, AC-1 traffic | the parent's neighbour entry for the LAN host reaches REACHABLE and the ping succeeds |
| AC-3 | same group with the recipe's `arp_announce` reverted | the capture sees a request with the owned address and the physical MAC (the red control) |
| AC-4 | last VRRP group on the parent removed | `arp_announce` is restored to its pre-VRRP value |
| AC-5 | owner group built (unit) | the dataplane calls show the parent recipe, then the owner filter (exactly the spec's `ownedVIPs`), then `openInstance`, all before the worker starts, and no publication at promotion or at a Master-to-Backup transition; teardown withdraws the filter |
| AC-6 | Q2 configuration (owned address primary or sole); Q2 (ii), owner 2026-10-14 | the commit is accepted; `ze doctor` warns naming the group and RFC 9568 Section 8.1.2; RFC9568-8.1.2-3 carries the deviation annotation with its section and reason and is not counted conformant |
| AC-7 | in-package guest test driving `newEngine(...).build` itself, never a hand replay of its order: the owned address on the parent, capture running and the LAN peer sending who-has for it; the engine built with an `enginePlatform` whose `applyDataplane` and `revertDataplane` are the real `applyDataplaneSysctls` and `revertDataplaneSysctls`, whose `createMacvlan` creates the device through the guest test backend (`CreateMacvlanDevice`, the real one waits on an iface reconcile) and whose `openInstance` is the fake platform's; `engineDeps` carrying the real `setOwnerFilter` and `clearOwnerFilter` and a fake `installVIPs` that adds the CIDRs to the macvlan through the test backend (the real one is `iface.RegisterOwnedAddresses`, which lands only through an iface reconcile); a probe between `build` and `start` (build returns the worker unstarted), then `start` and promotion | no ARP message (request or reply) carries the owned address with the physical MAC from `build` returning on, through the owned address reaching the macvlan, and the replies for it carry the virtual MAC; red with the `syncOwnerFilter` call removed from `engine.build` (the probe between `build` and `start` sees a physical-MAC reply). Under Q7 (b) the test is untagged and its red and green output is pasted; under Q7 (a) it carries the RFC9568-8.1.2-3 tags and the red is recorded with `./le rfc discriminate-record` |
| AC-8 | `vrrp-keepalived` lab: capture running before ze starts, the peer sending who-has for the owned address throughout, ze cold start, then a stop and start through `(*vrrpLab).restartZe`, each until the owner is Active | from each instance build on, no ARP message carries the owned address with the physical MAC; every such message seen before a build (from the iface install, and in the stopped interval) is reported with its timestamps, the measured Q7 window. Under Q7 (a) there is none. Untagged goal evidence |
| AC-15 | running owner Master, commit changing its VIPs (same netdev), and a running owner in Initialize or Backup with the same change | the owner filter names exactly the new `ownedVIPs` after `reconfigure` in every state; an unchanged owned set causes no reconcile |
| AC-16 | owner group build whose `openInstance` fails (unit) | the owner filter entry is withdrawn, then the recipe reverted and the macvlan deleted; no `vrrp-owner` entry remains |
| AC-9 | vpp `Verify` over a table holding each of `MatchARPOperation`, `MatchARPSenderAddress`, `MatchNDTargetAddress` | one error per match naming it and the reason VPP ACLs cannot match it; never "not recognized" |
| AC-10 | vpp `Verify` over a table of family arp, bridge or netdev | refused naming the family |
| AC-11 | `firewall.ApplyAll` with the vpp backend loaded, one owner's table carrying an unsupported match and another owner's table supported | the refused owner's error names that owner and the verifier's reason; the other owner's table reaches `Apply` |
| AC-12 | `applyMatch` given a match type it has no case for | returns an error naming the type; no rule is produced |
| AC-13 (Q4 (c), if A-2 holds) | commit of an owner group while the firewall backend is vpp | refused naming the group, the backend and RFC 9568 Section 8.1.2 |
| AC-14 (Q3 yes, owner 2026-10-14; mechanism owed by the next design round) | IPv6 owner Active resolves a LAN host, steady state and at boot | no Neighbor Solicitation leaves the parent with the owned address as source and the physical MAC in its Source Link-Layer Address option |

## End-to-End User Stories

| Story | Path | Test |
|-------|------|------|
| An operator runs Ze as the VRRP address owner; LAN hosts keep the gateway on the virtual MAC while the owner talks to them, and across a reboot or restart of the owner | config → build → recipe and filter → kernel ARP | `TestVRRPOwnerRequestsCarryNoPhysicalMACForOwnedAddress`, `TestVRRPOwnerSendsNoPhysicalMACARPFromBuild`, the lab owner-boot path (AC-8), `vrrp-v2-owner-keepalived` |
| An operator on the vpp firewall backend gets a refusal, not a silent deny, when a plugin registers a table VPP cannot express, and the rest of the firewall keeps applying | registry → `ApplyAll` → per-owner verifier | `TestApplyAllVerifiesEachOwner` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDataplaneApplyIPv4SetsRecipe` (RFC-tagged; body asserts `arp_announce=2` on the parent). The `arp_announce` assertion carries NO new tag: RFC9568-8.1.2-3 is a boot and restart obligation, and a recipe assertion does not observe it (under Q7 (b) no test carries that tag; under Q7 (a) it sits on `TestVRRPOwnerSendsNoPhysicalMACARPFromBuild`). The existing 8.1.2-1 / 8.2-1 prose "the parent's physical MAC never answers ARP for the virtual address" is false at HEAD for the owner, whose owned address sits on the parent, so `arp_ignore=1` does not stop the parent answering (that is why the owner filter exists): the prose is scoped to the non-owner | `internal/plugins/vrrp/dataplane_linux_test.go` | AC-1 recipe | [ ] |
| `TestDataplaneRestoreOnLastGroup` (RFC-tagged negative: RFC3768-6.4.3-1, RFC3768-8.2-1, RFC9568-6.4.3-1, RFC5798-6.4.3-1, RFC9568-8.1.2-1, RFC5798-8.1.2-1). Its six tag lines are WITHDRAWN: restoring knobs on teardown violates neither 8.1.2-1 nor 6.4.3-1, so it is a scoping property no RFC sentence states (wrong polarity at HEAD, journal row in `plan/journal/reference-checked-claim-unchecked.md`). The body stays as an untagged guard and also asserts the restore of `arp_announce`, untagged | same | AC-4 | [ ] |
| `TestVRRPWritesOnlyLabGrantedSysctls` (untagged, unchanged: it seeds from `parentSysctls`, and the grant covers every `net/` key, A-9) | `internal/plugins/vrrp/lab_profile_linux_test.go` | A-9 | [ ] |
| `TestOwnerFilterPublishedAtBuild` (new, untagged: ordering only) | `internal/plugins/vrrp/engine_test.go` | AC-5 | [ ] |
| `TestOwnerFilterRepublishedOnReconfigure` (new, untagged: a Master, an Initialize and a Backup instance each reconfigured with a changed and an unchanged owned set) | `internal/plugins/vrrp/engine_test.go` | AC-15 | [ ] |
| `TestBuildUnwindClearsOwnerFilter` (new, untagged: `openInstance` failing on the fake platform) | `internal/plugins/vrrp/engine_test.go` | AC-16 | [ ] |
| `TestOwnerFilterWiredOnPromotionForEveryFamily` (RFC-tagged, 10 lines; body changes: the filter is asserted present before the worker starts and before `install-addresses`, rather than handed at promotion; the tags' claims, that the owner Active hands the dataplane a filter for exactly its owned address, stay true) | `internal/plugins/vrrp/ownerfilter_test.go` | AC-5 | [ ] |
| `TestOwnerGroupPrimaryAddressRule` (new; Q2 (ii): the doctor warning; per owned address, `ownedVIPs`) | `doctor_test.go` | AC-6 | [ ] |
| `TestVerifyRejectsARPAndNDMatchesWithReason` (new) | `internal/plugins/firewall/vpp/verify_test.go` | AC-9 | [ ] |
| `TestVerifyRejectsNonIPFamilies` (new) | same | AC-10 | [ ] |
| `TestApplyMatchRejectsUnknownMatch` (new) | `internal/plugins/firewall/vpp/translate_test.go` | AC-12 | [ ] |
| `TestApplyAllVerifiesEachOwner` (new) | `internal/component/firewall/registry_test.go` | AC-11 | [ ] |
| `TestValidateGroupsRefusesOwnerOnVPPFirewall` (new, Q4 (c) if A-2 holds) | `internal/plugins/vrrp/groups_test.go` | AC-13 | [ ] |

RFC-tagged tests that change and need owner approval (Q6), under the
recommended Q1 (a): `TestDataplaneApplyIPv4SetsRecipe` (prose corrected),
`TestDataplaneRestoreOnLastGroup` (six negative tag lines withdrawn) and
`TestOwnerFilterWiredOnPromotionForEveryFamily` (body changed, tags kept).
Under the recommended Q7 (b) no new tag is added: RFC9568-8.1.2-3 stays
`{gap}`, and `TestVRRPOwnerSendsNoPhysicalMACARPFromBuild` is untagged goal
evidence. Under Q7 (a) the new tag is RFC9568-8.1.2-3 on that test, a new Go
unit that owes its discrimination record, not an approval. No lab file carries
a tag. `TestVRRPOwnerAnswersWithVirtualMACOnly`
and `TestOwnerFilterTablesDropParentAnswersForOwnedAddresses` keep their bodies.
The other options' lists are in Q6 and the delta table.

#### Owner approval commands

Approved by the owner on 2026-10-13 (Q6). The gate reads approvals from the committing session's own file (`tmp/commit-rfc-approved-<session>.md`), and a row is used up by the commit that carries its `RFC-approved:` trailer. So the owner runs these at the prompt of the session that implements and commits the edits, and runs a unit's command again if a later commit changes that unit a second time. A unit is `<package directory>.<top-level test function>`; a deleted unit is asked for too, by its old path. Every name was checked to exist at HEAD `051876c3f4`. The first two apply under every Q1 answer; the third applies under Q1 (a) and Q1 (c); the last two apply only under Q1 (c). `TestOwnerFilterTablesDropParentAnswersForOwnedAddresses` is untagged and needs none.

5 commands:

```
./le rfc approve unit vrrp.TestDataplaneApplyIPv4SetsRecipe reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-vrrp-owner-arp-rfc-defects, tag prose corrected, every Q1 answer)'
./le rfc approve unit vrrp.TestDataplaneRestoreOnLastGroup reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-vrrp-owner-arp-rfc-defects, six negative tag lines withdrawn, every Q1 answer)'
./le rfc approve unit vrrp.TestOwnerFilterWiredOnPromotionForEveryFamily reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-vrrp-owner-arp-rfc-defects, Q1 (a): body asserts the filter at build; Q1 (c): 10 tag lines change or go with the owner filter)'
./le rfc approve unit vrrp.TestVRRPOwnerAnswersWithVirtualMACOnly reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-vrrp-owner-arp-rfc-defects, Q1 (c) only, 20 tag lines change or go, negative control rebuilt on the address-move path)'
./le rfc approve unit vrrp.TestInstanceOwnerStartupGoesMaster reason 'Thomas, 2026-10-13: "approve all RFC-tagged test changes" (spec-vrrp-owner-arp-rfc-defects, Q1 (c) only, 8 tag lines; RFC3768-6.4.3-4 "the owner Master installs the VIP (a real parent address)" re-judged on the address-move path)'
```

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no numeric input added | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestVRRPOwnerRequestsCarryNoPhysicalMACForOwnedAddress` (new, untagged: the steady state is not the scope of RFC9568-8.1.2-3; red with `arp_announce` reverted, output pasted) | `internal/plugins/vrrp/owner_request_integration_linux_test.go`, QEMU guest | AC-1, AC-2, AC-3 | [ ] |
| `TestVRRPOwnerSendsNoPhysicalMACARPFromBuild` (new, in-package Go test under the `integration && linux` build tag, QEMU guest; it reuses the capture of `owner_answer_integration_linux_test.go` (`newGatewayWire`, the AF_PACKET peer socket, the nft backend) but NOT its order, which calls `applyDataplaneSysctls` and `setOwnerFilter` by hand and so could not go red with `engine.build` changed. It drives `newEngine(...).build` as AC-7 states, with a GroupSpec whose `ParentDevice` is the guest parent (`build` uses it as given; `apply` is what fills it in production), so the macvlan name follows `fakePlatform.parentIfindex`'s made-up index, which is harmless because the owner string and the filter key derive from that same name: the `fakePlatform` of `engine_test.go` with `applyDataplane`/`revertDataplane` replaced by the real sysctl functions and `createMacvlan` by the guest test backend, the `fakeDeps` of `instance_test.go` with the real `setOwnerFilter`/`clearOwnerFilter` and an `installVIPs` that adds the address through the test backend; it probes between `build` and `start`, then starts the worker. It cannot start the daemon, so it claims only what it drives: from `build` on, through the owned address reaching the macvlan, no ARP message the parent sends carries the owned address with its physical MAC; control: with the `syncOwnerFilter` call removed from `engine.build`, the who-has probed between `build` and `start` is answered with the physical MAC. Under Q7 (b) (owner, 2026-10-14) it is UNTAGGED goal evidence, red and green output pasted, because the row stays `{gap}`. Under Q7 (a) it carries RFC9568-8.1.2-3 positive and negative, drives the iface withhold too, and its red is recorded with `./le rfc discriminate-record` on that `syncOwnerFilter` call) | `internal/plugins/vrrp/owner_boot_integration_linux_test.go`, QEMU guest | AC-7 | [ ] |
| IPv6 sibling (if Q3 yes, tags RFC9568-8.2.2-5 and, if the fix steers the SLLAO, RFC9568-8.2.2-2/-3) | same files | AC-14 | [ ] |
| owner-request and owner-boot paths in the `vrrp-keepalived` lab (untagged goal validation: the lab starts the daemon with `startCapture` and `startZe`, and restarts it with `(*vrrpLab).restartZe`, new work in `internal/le/test/qemu/vrrp_keepalived_linux.go`: stop the guest's ze in order, then start it again with the capture kept running) | `internal/le/test/qemu/vrrp_owner_linux.go` | the owner pings the keepalived peer from the owned address, and is cold-started and restarted, while the peer's neighbour table keeps the virtual MAC; AC-8 timestamps reported | [ ] |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `vrrp-v2-owner-keepalived` (extended) | `test/interop/scenarios/vrrp-v2-owner-keepalived/` | keepalived | Ze stays owner; after Ze sends traffic from the owned address to keepalived's address, keepalived's host neighbour entry for `172.30.0.60` holds the virtual MAC `00:00:5e:00:01:14`. Owed with its revert-red record: revert the recipe's `arp_announce`, rebuild the ze image the scenario drives, confirm the new assertion red, restore, confirm green, and record the red result (`ai/rules/interop-and-goal-validation.md`) | [ ] |
| `vrrp-mastership-keepalived` | `test/interop/scenarios/vrrp-mastership-keepalived/` | keepalived | unchanged mastership with the new recipe and the build-time owner filter | [ ] |

### Goal Validation
| Goal | Evidence owed |
|------|---------------|
| LAN hosts never learn the owner's physical MAC for the owned address | the QEMU captures (AC-1, AC-7) with their red controls (AC-3, and AC-7's pasted red under Q7 (b) or its recorded discrimination under Q7 (a)), the `{gap}` reason on RFC9568-8.1.2-3 naming the pre-build window under Q7 (b), the lab boot and restart capture (AC-8) with its measured pre-build window, and the keepalived peer's neighbour entry in the interop scenario with its revert-red record |
| The owner keeps reaching its LAN | AC-2 in the guest, the interop scenario staying green |
| vpp never applies a table it cannot express, and one plugin cannot stop another's firewall | AC-11 negative test with its per-owner error and the other owner's table applied |

## Files to Modify

- `rfc/short/rfc9568.md` - RFC9568-8.1.2-3 row only: under Q7 (b) it gains `{gap: ...}` whose reason states the sentence is met from `engine.build` on (`internal/plugins/vrrp/engine.go::build`) and unmet from the iface address install to `build` at every boot and restart; under Q7 (a) it stays unannotated and is met by the tagged from-build test (the 8.2.2-2/-3 rows were corrected by `bd9fbb5989`); `docs/features/rfc-status.md` regenerated by `./le rfc index-update`
- test seams for `TestVRRPOwnerSendsNoPhysicalMACARPFromBuild`: none new. Read 2026-10-14 (r3): `enginePlatform` (`engine.go`) is a struct of function fields, not an interface, carrying `createMacvlan`, `applyDataplane`, `revertDataplane`, `openInstance` and `parentIfindex`; `newEngine(clk, platform, deps)` takes it; `engineDeps` (`instance.go`) carries `installVIPs`, `setOwnerFilter` and `clearOwnerFilter`; `build` returns the worker unstarted and `start` runs it; the untagged `fakePlatform` (`engine_test.go`) and `fakeDeps` (`instance_test.go`) compile into the `integration && linux` build of the same package, so the test overrides their fields
- `internal/plugins/vrrp/dataplane_linux.go` - `parentSysctls` gains `arp_announce=2`, joins the save and restore
- `internal/plugins/vrrp/engine.go` - `build` calls `syncOwnerFilter` after `applyDataplane` and before `openInstance`, and its `openInstance` unwind calls `clearOwnerFilter` before `revertDataplane`; `teardown` calls `clearOwnerFilter` after `closeInstance`; the `build` comment ("the macvlan, the dataplane recipe, the transport") names the filter
- `internal/plugins/vrrp/instance.go` - `reconfigure` calls `syncOwnerFilter` after the spec swap; `doInstallVIPs` and `doRemoveVIPs` drop their owner-filter calls, and their comments (the "installed first" ordering, the "has to answer for it again" sentence) are rewritten
- `internal/plugins/vrrp/doctor.go` or `groups.go` - Q2 rule; `groups.go` `validateGroups` Q4 (c)
- `internal/plugins/vrrp/ownerfilter.go` - new `syncOwnerFilter` (the instance owner string and the group spec in; it hands `setOwnerFilter` the spec's `ownedVIPs` of the spec's own VIPs, an empty set withdrawing the entry, and returns its error for the caller to log); the `setOwnerFilter` and `clearOwnerFilter` comments and the package comment sentence on requests and on when the filter is published (stale-comment rule)
- `internal/le/test/qemu/vrrp_keepalived_linux.go` - new `(*vrrpLab).restartZe` beside `startZe`
- `internal/plugins/firewall/vpp/verify.go` - family check, three match cases
- `internal/plugins/firewall/vpp/translate.go` - `applyMatch` default arm, `translateTerm` comment
- `internal/component/firewall/registry.go` - `ApplyAll` verifies each owner with the active backend's verifier and leaves a refused owner out
- `internal/le/test/qemu/vrrp_owner_linux.go` and the interop scenario check for the owner-request and owner-boot paths
- tests named in the TDD plan; `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md`; `docs/architecture/firewall/fw-6-firewall-vpp.md`; `docs/architecture/firewall/table-ownership-and-shutdown-flush.md`; the Fix cells of the two 2026-09-27 rows in `plan/journal/guard-added-to-one-half-of-a-pair.md`

## Files to Create

- `rfc/full/rfc826.txt` (fetched from `https://www.rfc-editor.org/rfc/rfc826.txt`, Wiring phase step 1)
- `internal/plugins/vrrp/owner_request_integration_linux_test.go`
- `internal/plugins/vrrp/owner_boot_integration_linux_test.go`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new leaf |
| YANG validation constraints | No | no new leaf |
| YANG custom validators | Yes | `internal/plugins/vrrp/groups.go` `validateGroups`: Q4 (c), and Q2 (i) if chosen |
| CLI commands/flags | No | none |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | N-A | no leaf |
| Functional test for new RPC/API | N-A | no RPC; the QEMU tests cover the path |
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
| 6 | Has a user guide page? | Yes | `docs/guide/vrrp.md` (owner address guidance, Q2; owner group refused with the vpp firewall backend, Q4 (c)) |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9568.md` rows 8.1.2-3 (per Q7) and, if Q3 is yes, 8.2.2-5 and 8.2.2-2/-3; `docs/features/rfc-status.md` is generated |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/qemu-integration.md` (new owner-request and owner-boot paths) |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md` (recipe, filter published at build), `docs/architecture/firewall/fw-6-firewall-vpp.md`, `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` (owner section, if Q2 adds a rule); `docs/architecture/core-design.md` unaffected (registration shape unchanged) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `ownerfilter.go` declares `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md`; `verify.go` and `translate.go` declare `docs/architecture/firewall/fw-6-firewall-vpp.md`; advisory mentions (2026-10-13 run): `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` (describes `ApplyAll`; affected, update with the per-owner verifier step), `docs/guide/firewall.md` (affected only if it states registry tables apply unverified on vpp; check), `docs/architecture/firewall/firewall-irr.md` (unaffected: names `RegisterTables` as its route, behaviour unchanged on nft), `docs/functional-tests.md` (affected if the new QEMU paths are listed there); rerun `./le spec citation anchors` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/vrrp.md` owner example: check address order against Q2 |

## Implementation Steps

1. **Phase: Wiring** - fetch `rfc/full/rfc826.txt` and quote A-5's merge rule from it (the ledger correction once planned here as Phase 0 landed in `bd9fbb5989`); QEMU probe for A-1, A-3 and A-6 (one guest run); then the failing recipe test, the failing build-order, reconfigure and unwind unit tests, the failing owner-request and from-build in-package QEMU tests, and the lab owner-boot path with `(*vrrpLab).restartZe`
2. **Phase: D1** - `parentSysctls` gains `arp_announce=2`, saved and restored; `syncOwnerFilter` added and called from `engine.build` and `instance.reconfigure`, `clearOwnerFilter` from `teardown` and the `build` unwind, the promotion calls removed; Q2 rule; Q7 as answered; doc edits
3. **Phase: D2** - vpp verifier family and match cases, `applyMatch` default arm, per-owner verification in `ApplyAll`; A-2 settled, then Q4 (c) in `validateGroups`; doc edits
4. **Phase: D3** (if Q3 yes) - research the IPv6 source preference and SLLAO, then the same TDD loop
5. **Phase: Evidence** - under Q7 (b) the pasted red and green of the from-build test and the `{gap}` row checked by `./le rfc check`; under Q7 (a) `./le rfc discriminate-record` for the new tag; the interop revert-red record, interop scenarios green, journal Fix cells

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Correctness | the owner still resolves its LAN (AC-2) in the same run that proves AC-1 |
| Data flow | registry tables and config tables reach the same verifier |
| Rule: rfc-compliance | under Q7 (b) RFC9568-8.1.2-3 is a `{gap}` row and no test carries its tag (an ordinary `{gap}` forbids polarity tags, and no `{partial}` selector pair states a time window); the residual pre-build window is stated in the gap reason and on the pages, never counted met. Under Q7 (a) the tag sits only on `TestVRRPOwnerSendsNoPhysicalMACARPFromBuild`, whose claim states no more than it drives, with its discrimination recorded. No lab file carries a tag under either answer |
| Rule: stale-comments | `ownerfilter.go` package comment, the `engine.build` comment on the recipe, the `vipMaskBits` comment and the dataplane doc sentence on requests |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| D1 fix | QEMU test output pasted, red and green, steady state and from build; the lab boot and restart capture with its timestamps; `./le rfc check` output for the 8.1.2-3 row |
| D2 fix | vpp and firewall unit tests output pasted |
| Interop | `./le test integration` owner scenario output, with the revert-red record |

### Security Review Checklist
| Check | What to look for |
|-------|------------------|
| Input validation | no new input; the verifier now rejects what it silently accepted, per owner |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A-1 broken (rewrite works) | DESIGN: Q1 reopens with the rewrite as a candidate |
| A-3 broken | DESIGN: Q1 reopens; (c) is then the only steering shape |
| A-6 broken ((c) alone steers) | DESIGN: Q1 reopens with (c) as a candidate that adds no sysctl |
| A-7 settled (r2): iface installs the owned address before `engine.build` | owner decision Q7 |
| The lab capture shows a physical-MAC message after a build | DESIGN: the build-time publication does not hold; re-read `build` and `reconfigure` |
| Q7 (a): iface reconciles no more at shutdown, so the withheld address does not leave at stop | DESIGN: the restart window reopens; route back to Q7 |
| Test fails on behavior mismatch | re-read Current Behavior |

## Design Insights

- The journal row framed D1 as a firewall term; the obligation is on the sender the kernel chooses, which a sysctl governs.
- RFC9568-8.1.2-3 is scoped to restart and boot, and the owned address is on the parent before VRRP exists; a steady-state proof does not reach it.
- The owner filter belongs to the instance's life, not to Active: published wherever the owned set can change (build, reconfigure), withdrawn wherever the instance ends (teardown, build unwind), through one function.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Steer the sender with a per-parent `arp_announce=2` (Q1 (a), recommended, pending the owner) | drop term, sender-MAC rewrite, address move, deviation (Q1 option deltas) | the shape that keeps the owner's resolution, adds no firewall match, and that the address move also needs (A-6); one per-device knob at the `dsr` profile's value. The simpler design that fails: no knob at all, which leaves the request sender at the owned address |
| Publish the owner filter through one function, `syncOwnerFilter`, called in `engine.build` after the parent recipe and before `openInstance`, and in `instance.reconfigure` after every spec swap; withdraw it with `clearOwnerFilter` in `teardown` and in the `build` unwind after an `openInstance` failure; promotion and demotion no longer touch it | keep the promotion call; publish at both | closes the window between build and Active in which the parent answers for the owned address with its physical MAC. `apply` reconfigures a same-netdev instance in place, and a Master whose VIPs change gets `InstallVIPs` (`fsm.masterConfigUpdated`), which today is what republishes the set: with the promotion call gone, `reconfigure` takes that job in every state. `setOwnerFilter` returns early on an unchanged set, so the reconfigure call is free when nothing changed. The simpler design that fails: a build-only publish, which keeps the build-time set after a commit changes the VIPs, and a build with no unwind clear, which leaves a `vrrp-owner` entry silencing an address no instance holds |
| An orderly stop keeps today's behaviour: `flush-on-shutdown` (default true) removes the `vrrp-owner` tables with every other ze table, and `stopAll`, `teardown` and `revertDataplane` restore the parent knobs. A restart is therefore a boot, and its window is Q7's | survive stop and adopt at the next start (r1 proposal) | settled at the producers (A-8). Surviving stop needs a `vrrp-owner` exception to Rule 3 of `table-ownership-and-shutdown-flush.md` and knobs left set by a process that is gone, while iface leaves the address on the parent either way. One window, handled once, by Q7, is the simpler answer |
| Verify each registry owner in `ApplyAll` (Q4 (b), recommended) and refuse an owner group with the vpp firewall backend at commit (Q4 (c), if A-2 holds) | verify the merged set; lower on vpp | one plugin's table cannot stop another's firewall from reconciling, and the operator meets the refusal at commit rather than in a log |

## Known Limitations
- Owners with no non-virtual address in the subnet, or whose owned address is primary: Q2.
- The interval from the iface address install to `engine.build`, at every boot and restart (A-7 settled that iface installs first): Q7. Under Q7 (b) it stays open and is stated in the `{gap}` reason of the RFC9568-8.1.2-3 row and in the dataplane doc.

## RFC Documentation (Scope: protocol)

Quote RFC 9568 Section 8.1.2, the SHOULD NOT sentence in the Defects table, above the `arp_announce` recipe entry and above `syncOwnerFilter` (the implementing function); each caller (`engine.build`, `instance.reconfigure`) carries the section reference only.

Ledger record, per Q7: under (b) (recommended) the RFC9568-8.1.2-3 row carries `{gap: ...}` naming the pre-build window and the producer `internal/plugins/vrrp/engine.go::build`, and no test carries its tag; under (a) the row carries no annotation and `TestVRRPOwnerSendsNoPhysicalMACARPFromBuild` carries its positive and negative tags with a discrimination record.

## Design Review Log

| Round | Date | Report | Findings | Disposition |
|-------|------|--------|----------|-------------|
| 1 | 2026-10-14 | `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-vrrp-design-r1.md` | 0 BLOCKER, 7 ISSUE, 6 NIT | applied: Q1 re-evaluated to (a) with the option delta table and the body written for (a); (c)'s competing route and iface ownership and (d)'s deviation consequence stated; boot and restart window designed (build-time filter, A-7, A-8, AC-7, AC-8) and the 8.1.2-3 tag moved to the boot test; Phase 0 for the false 8.2.2-2/-3 exclusions; Q4 per-owner verification plus commit refusal; R-6 and the sysctl grant corrected (A-9); `TestInstanceOwnerStartupGoesMaster` added to Q6; NITs: journal rows cited by date, ARP sysctl inventory, RFC 826 fetch planned, owner reading stated with a journal row in `plan/journal/field-carries-two-meanings.md`, interop revert-red record. Also corrected: the recipe is applied in `engine.build`, not at promotion. Round 2 owed |
| 2 | 2026-10-14 | `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-vrrp-design-r2.md` | 0 BLOCKER, 4 ISSUE, 2 NIT | applied: Phase 0 removed as future work (landed in `bd9fbb5989`; RFC5798-8.2.2-3 already `{gap}`), only the `rfc826.txt` fetch kept, in step 1; owner filter published by one function, `syncOwnerFilter`, from `build` and `reconfigure`, cleared in `teardown` and the `build` unwind (AC-15, AC-16, two unit tests); orderly stop kept as today, A-8 and Key Design Decisions row 3 settled; A-7 settled at the producers (iface's tier configures and installs first), the residual window put to the owner as Q7, the RFC9568-8.1.2-3 tag moved to an in-package Go test claiming "from build", the lab boot and restart capture made untagged goal evidence with the new `(*vrrpLab).restartZe`; NITs: A-3/A-6 cite `ip-sysctl.rst` by URL (no repo location for kernel docs), Q4 (b) verifies each owner before `mergeSameNameTables`. Round 3 owed (narrow: the r2 edits and Q7) |
| 3 | 2026-10-14 | `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-vrrp-design-r3.md` | 0 BLOCKER, 2 ISSUE, 1 NIT | applied: AC-7, its TDD and Wiring rows make the from-build test drive `newEngine(...).build` (real `applyDataplane`, `createMacvlan` through the guest test backend, fake `openInstance` and `installVIPs`, real owner-filter deps, a probe between `build` and `start`), red with the `syncOwnerFilter` call removed from `build`; the seams were read and all exist (`enginePlatform` is a struct of function fields; Files to Modify records that none is new); the record shape is decided conditional on Q7: under (b) RFC9568-8.1.2-3 is `{gap}` and the from-build test is untagged goal evidence, under (a) the row is met and the test tagged, aligned in What a fix must prove, Q7, the Wiring row, AC-7, the TDD rows and paragraph, Goal Validation, Files to Modify, step 5, the Critical Review row, Known Limitations and RFC Documentation; NIT: A-3 and A-6 cite `Documentation/networking/ip-sysctl.rst` by upstream path and URL with verbatim quotes, the hedge dropped. Round 4 owed (narrow: the r3 edits) |
| 4 | 2026-10-14 | `tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/review-vrrp-design-r4.md` | 0 BLOCKER, 0 ISSUE, 1 NIT | NIT applied by the main thread: the from-build test's GroupSpec sets `ParentDevice` to the guest parent, and the macvlan name follows the fake ifindex. Design converged; Q1..Q5 and Q7 stay with the owner |
| - | 2026-10-14 | owner answers | Owner, 2026-10-14: "Do what you think is right", accepting every recommendation | Recorded: Q1 (a), Q2 (ii), Q3 yes, Q4 (b) and (c) together with (c) only if A-2 holds, Q5 the VRRP recipe, Q7 (b); Q6 note now names the Q1 (a) list. AC-6, AC-14, R-7, the AC-6 test row, Data Flow step 2, Behavior to change and the Q7 (b) mentions lose their conditionals; AC-13 and its test keep "if A-2 holds" because the owner's answer carries it. Owed by the next design round: (1) Q3 yes: the IPv6 mechanism for D3 and RFC9568-8.2.2-2/-3 (source-address and SLLAO choice for Neighbor Solicitations, Q3 row: "must be researched in this spec's next pass"), with AC-14's test, its TDD row and Files rows, and whether the -2/-3 `{gap}` rows change; (2) Q2 (ii): the exact deviation annotation text on RFC9568-8.1.2-3 and the doc pages that state it; (3) Q4 (c): the A-2 validation step and what the spec carries if A-2 fails (AC-13 and `TestValidateGroupsRefusesOwnerOnVPPFirewall` drop; (b) stays); (4) a sweep of the remaining Q1 option-delta, Q2 (i) and Q7 (a) wording (Q1 option deltas table, R-1, A-6/A-7, Required Reading constraints, the TDD tag notes) for text that still reads as open, which this edit pass did not walk line by line |

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
