# Spec: ipsec-vpp-policy-interface

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | An operator SPD entry (`vpn ipsec policy`) committed on the VPP dataplane is accepted and never installed: `vppPolicyInterface` refuses it for its interface index 0 and `reconcileSPDPolicies` reads that refusal as a platform with no dataplane, so the commit succeeds and the entry does nothing. That is accepted but ineffective config (`plan/README.md`, "Split fixes from features"), so the work is a release fix. Thomas chose on 2026-11-20 to fix it by building the interface model (decision 7 of `plan/release/fixes/spec-rfc4301-architecture-gaps.md`, "Pull the post-release spec forward") rather than refusing operator entries on VPP |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-20 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Moved on 2026-11-20 from `plan/post-release/features/spec-ipsec-vpp-policy-interface.md`
into release scope. Status stays `design`: Thomas approved the interface source
only (Thomas's rulings, below); the rest of the design still goes to him whole.

`spec-fixit-vpp-ipsec-inoperable` closed on 2026-08-10 with real-VPP SA
install/read-back evidence. That prerequisite is fulfilled. This spec owns the
interface an SPD binds to, the installation of IKE Child SA policies and operator
SPD entries on VPP, and the policy and rekey correctness that installation makes
reachable.

**Decided by Thomas on 2026-11-20 (C-2, his decision 2):** the real-VPP ESP proof
(`plan/release/validation/spec-ipsec-vpp-dual-form-proof.md`) and the operator
dataplane selector (`plan/release/fixes/spec-ipsec-dataplane-selector.md`) come
into the first release, so VPP IPsec is a first-release feature. It follows from
his ruling 3a on the first task's finding (1): Ze meets RFC 4301 Section 5.2 for
inbound cleartext on VPP itself, through an inbound ACL it derives on the IPsec
WAN interface (Thomas's rulings, below). Both specs still depend on this one.

## Task

**Nothing tells the VPP IPsec backend which VPP interface its SPD applies to, so
no IKE-produced policy and no operator SPD entry is installed on that backend.
Give the interface a source: the existing `vpn ipsec interface` leaf.**

VPP has no node-wide security policy database. A policy lives in an SPD, and an
SPD takes effect only on the interfaces it is bound to (`ipsec_interface_add_del_spd`).
`vppPolicyInterface` (`internal/component/ike/dataplane/vpp_policy.go`) therefore
refuses `SPParams.IfIndex` 0, which is correct: sw_if_index 0 is a real VPP
interface, so reading an unset field as "any" would program the wrong one.

`childPolicyParams` (`internal/component/ike/engine/child.go`) leaves the field at
its zero value, and so does every operator SPD entry `reconcileSPDPolicies`
(`engine/spd_policy.go`) installs. So every IKE Child SA policy is refused by that
backend, and every operator entry is refused and silently dropped
(`docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, "SPD dispositions").

The refusal is right. The gap is that nothing supplies the interface.

## Thomas's rulings

| Date | Ruling, his words in quotes | What it fixes here |
|------|-----------------------------|--------------------|
| 2026-11-20 | (5a) "An operator SPD entry on VPP binds to every interface Ze's SPD is attached to" | An operator entry shares the SPD Child SA policies use, with the node-wide meaning it has on XFRM |
| 2026-11-20 | Decision 7: "Pull the post-release spec forward" | This spec is release scope; operator entries on VPP are installed, not refused |
| 2026-11-20 | "reuse the existing IPsec WAN interface leaf" | On VPP the one Ze SPD is bound to the interface `vpn ipsec interface` names, resolved with `iface.ResolveVPPIndex`; Child SA policies and operator entries both land in it. XFRM is unchanged: `SPParams.IfIndex` stays 0 there |
| 2026-11-20 | Decision 3, answer "3a" | On VPP, RFC 4301 Section 5.2 inbound processing (cleartext matching an inbound PROTECT or DISCARD entry is discarded) is met by Ze itself: Ze derives an inbound ACL on the IPsec WAN interface (the `vpn ipsec interface` leaf, the interface Ze's one VPP SPD binds to) from the inbound PROTECT and DISCARD entries, because real VPP v26.06 forwards such cleartext (first task outcome). The ACL needs one owner shared with the VPP firewall backend, a composition still to be presented. **Found unworkable as stated on 2026-11-20; alternatives with Thomas (C-4)**: in SPD mode the decrypted inner packet re-enters `ip4-unicast` on the WAN interface with its RX interface unchanged and no decrypted flag (`esp_decrypt.c`, v26.06), so an input ACL denying the PROTECT selectors also drops decrypted tunnel traffic |
| 2026-11-20 | C-4, answer "B" to the option: "Patch VPP, one more next to the 14 Ze already carries. Decryption marks the packet; IPsec input then judges unmarked cleartext against the SPD: drop on protect or discard, pass on bypass. This is RFC 4301's own model, done where VPP actually knows what was decrypted." | Supersedes 3a's ACL (kept above as history). AC-8's mechanism is a VPP patch Ze carries: `third_party/vpp-patches-v26.06/0106-ipsec-judge-unprotected-inbound-traffic.patch` (Apache-2.0, VPP v26.06). The policy-mode decrypt nodes set `VNET_BUFFER_F_IPSEC_DECRYPTED`; `ipsec4/6-input-feature` read and clear it, pass a marked packet, and judge an unmarked non-ESP/AH/IKE packet over BYPASS, DISCARD and the PROTECT selectors by priority: PROTECT or DISCARD drop (counted), BYPASS passes, no match passes (Ze's catch-all is its own entry). Earlier rulings kept: one Ze SPD per VPP backend on the `vpn ipsec interface` WAN; the VPP firewall stays the only owner of interface ACLs, so no IPsec ACL is written |
| 2026-11-20 | Decision 2, following from 3a | `spec-ipsec-vpp-dual-form-proof` (now `plan/release/validation/`) and `spec-ipsec-dataplane-selector` (now `plan/release/fixes/`) come into the first release; VPP IPsec is a first-release feature (C-2) |

Alternatives Thomas rejected for the interface source:

| Alternative | Why he set it aside |
|-------------|---------------------|
| A per-Child-SA route lookup toward the peer (`iface.RouteLookup`) | It handles several WANs, but the SPD moves whenever routing changes |
| strongSwan's VPP plugin approach: bind to the interface holding the local address, exact match | It breaks when the local address is on a loopback |

Alternatives Thomas rejected for inbound cleartext on VPP (decision 3):

| Alternative | Why he set it aside |
|-------------|---------------------|
| (b) Keep VPP IPsec unselectable until such enforcement exists | He chose to build the enforcement (3a) and bring VPP IPsec into the first release |
| (c) VPP route-based mode (an ipip tunnel with `tunnel protect`) | It brings no RFC 4301 inbound policy either |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - "Decisions" (5a and the WAN-interface ruling), "SPD dispositions" (the VPP column and the silent drop), "Policy ownership"
  → Decision: an operator SPD entry on VPP binds to every interface Ze's SPD is attached to; on VPP that SPD is bound to the WAN leaf's interface
  → Constraint: XFRM keeps a node-wide SPD with `SPParams.IfIndex` 0 for IKE policies
- [ ] `ai/doors/access-security.md` - `ike/dataplane` is an ungated seam OSPF also uses, so a change to `SPParams` reaches OSPF's RFC 4552 installer
  → Constraint: OSPF hands a KERNEL ifindex to `SPParams.IfIndex`; VPP must keep refusing it
- [ ] `ai/doors/forwarding.md` - interface names go through the shared resolver; exact or refuse
  → Constraint: the leaf's name is resolved by `iface`, never read as a device or an index
- [ ] `ai/doors/system.md` - a backend that cannot apply a config exactly refuses it at verify; a plugin can run in another process
  → Constraint: an unresolvable interface is refused, never guessed; the IKE engine is a plugin (`runEngine`, `engine/register.go`)
- [ ] `ai/rules/protocol.md` - exact-or-reject for a backend that applies operator config
- [ ] `ai/rules/config.md` - the interface comes from an existing YANG leaf; no new leaf

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4301.md` - Sections 4.4.1 (SPD), 5.1 and 5.2 (outbound and inbound processing)
  → Constraint: RFC 4301 Section 5.2 inbound processing applies to every packet arriving on a protected interface, cleartext included, which is the first task's risk

**Key insights:**
- The leaf exists, the resolver exists, the bind exists (`ensureSPD`); what is missing is the path from the leaf to the bind, and a VPP reading of index 0 that is not "refuse".
- `SPParams.IfIndex` is read by XFRM as `sel.ifindex` and copied into the SA state scope (`childStateScope`, `engine/child_scope.go`), so the VPP interface must not travel in it.

## Current Behavior (MANDATORY)

**Source files read:** (2026-11-20)
- [ ] `internal/component/ike/dataplane/vpp_policy.go` - `InstallPolicy` calls `vppPolicyInterface(p)`, which refuses `IfIndex` 0 or out of u32 range with `ErrNotSupported`; `ensureSPD` creates one SPD per backend and binds it to each `swIfIndex` once (`b.spdBound`); `undoSPD` and `unbindSPD` undo a bind
- [ ] `internal/component/ike/dataplane/vpp.go` - `vppBackend` holds `spdID`, `spdBound`, `spdEntries`, `sadIDs`; `RemoveSA` and `removeInstalled` (the AC-5 leak); the `RemoveSA` comment names this spec at a path that no longer exists
- [ ] `internal/component/ike/dataplane/dataplane.go` - the `Dataplane` interface; `SPParams.IfIndex` comment: the index of the forwarding plane the backend programs; IKE leaves 0; "Three refusals keep that inert today" (an under-count, see below); `SASelector.IfIndex` is the selector's kernel index
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - `InstallPolicy` upserts every policy
- [ ] `internal/component/ike/engine/child_scope.go` - `childStateScope` copies each policy's `IfIndex` into the SA's `SASelector`
- [ ] `internal/component/ike/engine/unmatched_xfrm_interface.go` - `xfrmInterfaceExemptions` sets `IfIndex: facts.ifIndex`, the kernel index `readXFRMInterface` reads with `iface.ResolveIndex`
- [ ] `internal/component/ike/engine/register.go` - `runEngine` loads the dataplane (`dataplane.Load(ikeDataplaneName())`); `resolveInterfaceAddr` resolves the leaf's first IPv4 address through `iface.Addresses`, after `iface.EnsureBackend`
- [ ] `internal/component/ike/engine/apply.go` - `applyConfig` resolves `cfg.Interface` first; a reload whose interface holds no IPv4 address is refused
- [ ] `internal/component/ike/engine/dynamic_wan.go` - the address worker re-reads the interface on address and create events; `readInterfaceAddr` answers an absent interface as "no address, wait"
- [ ] `internal/component/ike/engine/testport.go` - `ikeDataplaneName`: production loads `xfrm`; VPP is reachable only through `ze.test.ike.dataplane`
- [ ] `internal/component/iface/resolve.go` - `ResolveVPPIndex(name)` refuses unless the active iface backend is `vpp`, then returns the binding's sw_if_index, refusing 0 or below
- [ ] `internal/plugins/fib/vpp/backend.go` (`toFibPath`), `internal/plugins/static/backend_vpp_linux.go` (`resolveVPPSwIfIndex`) - both resolve an interface name with `iface.ResolveVPPIndex`
- [ ] `internal/component/ike/engine/spd_policy.go`, `internal/component/ike/engine/child.go` - `reconcileSPDPolicies` and `installSPDPoliciesAtStart` return with nothing installed and no error when `isXFRMUnsupported` answers true, which it does for any `ErrNotSupported`
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - the `interface` leaf (WAN interface; commit accepts a name the host does not carry yet); the `policy` list description says the VPP dataplane installs no operator SPD entry
- [ ] `internal/le/test/deployment/vppevidencerun.go` - `runIPsec` passes a hand-set index to the probe as `ZE_VPP_IPSEC_SW_IF_INDEX`

**Behavior to preserve:**
- XFRM: every IKE policy and SA state carries interface index 0 and the policies stay byte-identical; the OSPF RFC 4552 installer keeps its kernel ifindex scoping; `xfrmInterfaceExemptions` keeps its kernel index
- VPP keeps refusing a non-zero `SPParams.IfIndex`, which today only a kernel index fills
- The `interface` leaf keeps its current meaning on both dataplanes (local address, IKE socket bind, dynamic WAN)

**Behavior to change:**
- On VPP, the SPD is bound to the leaf's interface, and Child SA policies and operator entries are installed in it
- An operator entry VPP cannot install fails the commit instead of being dropped silently

## Corrected facts

The post-release text of this spec carried two facts the tree contradicts.

| Stale statement | What the tree says |
|-----------------|--------------------|
| `buildIPsecPolicies` is "the one producer of a non-zero `IfIndex` in the tree" (the same claim stands in the `vppPolicyInterface` comment, `vpp_policy.go`) | `xfrmInterfaceExemptions` (`engine/unmatched_xfrm_interface.go`) also sets one, the kernel index of a bound xfrm interface. Both are kernel indexes. The `vppPolicyInterface` comment is corrected on the first edit of that file |
| The evidence run is under `internal/le/deployment/` | It is `internal/le/test/deployment/vppevidencerun.go` (`runIPsec`) |

The `SPParams.IfIndex` comment (`dataplane.go`) and the `RemoveSA` comment
(`vpp.go`) also name this spec at `plan/spec-ipsec-vpp-policy-interface.md`, a
path that no longer exists; the first edit of each points it at
`plan/release/fixes/spec-ipsec-vpp-policy-interface.md`.

## First task: measure cleartext inbound on a bound interface

**This is the first task, before any code, because its answer can invalidate the
design.**

VPP's `ipsec4-input-feature` (`src/vnet/ipsec/ipsec_input.c`, VPP master, read
on 2026-11-20) examines only ESP, AH and UDP 500 and 4500. Any other packet
increments `unprocessed` and passes. So plain cleartext arriving on an interface
Ze's SPD is bound to may never be tested against an inbound BYPASS or DISCARD
entry, and an operator `direction in` DISCARD would then drop nothing.

RFC 4301 Section 5.2 makes inbound processing apply to every packet arriving on a
protected interface, cleartext included. This must be measured on a real VPP at
the version the lab runs: bind an SPD carrying an inbound DISCARD for a cleartext
flow to an interface, send that flow, and read the interface counters and the
SPD entry's match counter.

| Result | What follows |
|--------|--------------|
| The cleartext packet is dropped by the DISCARD entry | The design stands; the measurement becomes AC-8's test |
| The packet passes, counted as `unprocessed` | The design cannot meet 5a for inbound cleartext. Stop and take it back to Thomas with the measurement: it is a design problem, not an implementation detail |

### Outcome of the first task (measured 2026-11-20)

Measured on real VPP v26.06 (x86 server, docker-lab on Ze's kernel), `./le test
deployment vpp` at commit `aba3cec65b`, by `TestVPPRealCleartextProbe`
(`internal/component/ike/dataplane/vpp_real_cleartext_integration_test.go`) and
`internal/le/test/deployment/vppevidencecleartext.go`. The counters are those of
`ipsec4-input-feature`, verbatim:

| # | Frame | Matching entry | VPP's verdict | Counters |
|---|-------|----------------|---------------|----------|
| 1 | cleartext ICMP `100.64.46.10` -> `100.64.45.1` | inbound PROTECT `100.64.46.0/24` | FORWARDED | 'policy match' 0, 'bypass' 0, 'discard' 0, 'not matched' 1 |
| 2 | cleartext ICMP from `100.64.47.10` | inbound DISCARD `100.64.47.0/24` | FORWARDED | same counters as 1 |
| 3 | cleartext ICMP from `100.64.48.10` | inbound BYPASS `100.64.48.0/24` | FORWARDED | same counters as 1 |
| 4 | ESP, unknown SPI, `100.64.47.10` -> `100.64.45.1` (the orientation the engine writes an inbound selector: Src is the remote range) | inbound DISCARD `100.64.47.0/24` | dropped | 'discard' 0, 'not matched' 1 |
| 5 | ESP `100.64.45.10` -> `100.64.47.1` (the pair exchanged) | inbound DISCARD, only with the pair exchanged | dropped | 'discard' 1, 'not matched' 0 |

Two findings follow.

1. **Confirmed: VPP's inbound SPD never judges cleartext.** Frames 1 to 3 were
   forwarded unjudged whatever entry they matched, so RFC 4301 Section 5.2 step 3b
   ("If there is a match and the packet is to be discarded or bypassed, do so." and
   "If there is no match, discard the traffic.") is unmet on VPP in policy mode. This
   is the second row of the table above: the design cannot meet 5a for inbound
   cleartext, and AC-8 is not met by design. **Pending Thomas's ruling**; nothing in
   this spec decides it, and no work proceeds on AC-6's inbound half or AC-8 until he
   rules. **Ruled 2026-11-20, no longer pending:** Thomas answered decision 3 with
   "3a": Ze meets Section 5.2 for inbound cleartext itself, through an inbound ACL it
   derives on the IPsec WAN interface from the inbound PROTECT and DISCARD entries
   (Thomas's rulings). The same day that ACL was found unworkable as stated (it would
   also drop decrypted tunnel traffic, see the rulings row), and the alternatives are
   with Thomas (C-4, "Open design item" below). AC-8 is restated as the outcome only;
   its mechanism is pending.
2. **Defect found in passing: the VPP backend installed every inbound SPD entry
   address-reversed.** Frames 4 and 5 show it: VPP's inbound lookup compares the
   packet's destination with the entry's local range, and `vppSPDSelector`
   (`internal/component/ike/dataplane/vpp_policy.go`) wrote `SPParams.Src` and
   `SrcPort` into the local side for both directions, while the engine writes an
   inbound policy with Src on the remote side (`childPairPolicy`,
   `internal/component/ike/engine/child.go`). Small, and related to the work in
   hand, so it is fixed here (AC-9, "Found in passing" below).

The same run also showed an unrelated red: the scenario `fib` reported "real VPP
FIB route not observed" (a BGP-learned route installed through `fib-vpp` is not in
`show ip fib`). It was not investigated. This work did not cause it: that scenario
does not touch `pg1` or IPsec.

## Found in passing: inbound SPD entries address-reversed on VPP

| Item | Detail |
|------|--------|
| Defect | `vppSPDSelector` mapped `SPParams.Src`/`SrcPort` to VPP's local side for an inbound policy too, so an inbound BYPASS or DISCARD matched the traffic flowing the other way and missed the traffic it was written for (outcome, frames 4 and 5) |
| Fix | An inbound (`SADirIn`) policy maps `Dst`/`DstPort` to the local range and port and `Src`/`SrcPort` to the remote ones; outbound is unchanged. `recordedEntry` builds its probe through `vppSPDSelector`, and `PolicyDeleteCommand` and `Close` send back the recorded entry, so install, removal, the vppctl delete command and teardown agree |
| Unit test | `TestVPPInstallPolicyInboundSides` (`internal/component/ike/dataplane/rfc7296_vpp_message_test.go`): the inbound entry's local side is Dst and port 53, its remote side Src and port 5000, the delete command and the removal name the same entry, and the outbound control keeps Src local |
| Real-VPP evidence | the next `./le test deployment vpp` run: `probe-esp-discard` counts 'discard' 1, `probe-esp-discard-reversed` counts 'discard' 0 |

## Decided design item: the inbound cleartext mechanism on VPP (C-4)

**Decided 2026-11-20: Thomas answered "B", a VPP patch (Thomas's rulings).** The
paragraphs below record why 3a's ACL was set aside and stay as history; the ACL
constraint in them no longer binds, because no IPsec ACL is written.

| Item | State |
|------|-------|
| Patch | `third_party/vpp-patches-v26.06/0106-ipsec-judge-unprotected-inbound-traffic.patch`, against VPP v26.06 `c3200b88dc`; applies with zero fuzz to the pristine release (checked 2026-11-20) and reproduces the edited sources. Compiled in the image build of 2026-11-20 |
| Image | `ze-vpp-deployment`, built only by `./le build vpp deployment` (`internal/le/build/vpp/deployment.go`), kept in le's image cache; `./le test deployment vpp` loads it (`readyImage`, `internal/le/test/deployment/vppiface.go`, through `interoplab.(*Docker).LoadCached`) and refuses on a cold cache naming the build command |
| AC-8 proof | The cleartext probe is now a proof (`judgeCleartextFrame`, `internal/le/test/deployment/vppevidencecleartext.go`): PROTECT and DISCARD frames owed dropped, BYPASS forwarded, each under its own counter, and `probe-esp-decrypted`, ESP the probe's SA decrypts (`decryptedProbePacket`), owed delivery with `IPSec decrypted packet` 1 |
| Evidence (2026-11-20, x86 server, docker-lab on Ze's kernel docker-53c57c37, commit 584dd88aa0) | `./le build vpp deployment` compiled the patched v26.06 and cached `ze-vpp-deployment` (lab-exit 0). `./le test deployment vpp` on it: PROTECT cleartext dropped, DISCARD cleartext dropped ('discard' 1), BYPASS cleartext forwarded ('bypass' 1), both ESP discard checks as AC-9 requires, and `probe-esp-decrypted` forwarded with 'policy match' 1: green for every IPsec check. Discrimination, the same run with `ZE_VPP_DOCKER_IMAGE=ligato/vpp-base:26.06-release`: red at `probe-cleartext-protect` ("real VPP forwarded cleartext ICMP echo ... matches the inbound PROTECT entry"). The `fib` scenario stayed red in both runs, unrelated ("real VPP FIB route not observed") |
| Still owed | an RFC 4301 Section 5.2 tag on the proof, recorded by `./le rfc record` |
| Not covered by the patch | fast-path SPDs, IPv6 extension headers, tunnel-protect SAs, a decrypted packet that leaves the unicast arc, RFC 4301 Section 5.2 step 4's inner-selector check (`third_party/vpp-patches-v26.06/README.md`) |

Ruling 3a
puts an inbound ACL, derived from the inbound PROTECT and DISCARD entries, on the
interface `vpn ipsec interface` names. On 2026-11-20 that was found unworkable as
stated: in VPP's SPD mode the packet ESP decryption yields re-enters `ip4-unicast`
on the WAN interface with its RX interface unchanged and no decrypted flag
(`esp_decrypt.c`, v26.06), so an input ACL denying a PROTECT selector drops the
decrypted tunnel traffic too, which AC-8 forbids. The alternatives are with Thomas,
and the mechanism stays open until he rules. Whatever mechanism he chooses, one
constraint found for the ACL form stands if any ACL is part of it. The VPP firewall backend
(`internal/plugins/firewall/vpp`) already writes ACLs to interfaces, and
`ACLInterfaceSetACLList` replaces an interface's whole ACL vector, input and output
in one write (`docs/architecture/firewall/fw-6-firewall-vpp.md`, the merge
paragraphs). That backend preserves ACLs it does not own and treats every ze-tagged
index as its own on each reconcile (`discoverACLs`, `reconcileRemovals`, per
`plan/release/fixes/spec-vpp-nat44-pool-ownership.md`). Two writers on one vector
therefore need one owner both share; which one, and how an IPsec ACL and the
firewall's ACLs are ordered and tagged in that vector, would be part of the design.

Where it lives, and why: this spec, not a new child. `plan/` holds no spec that
owns VPP ACL composition (searched 2026-11-20 for `ACLInterfaceSetACLList`,
`firewall` with `vpp` and `acl`: `spec-finish-vpp-stub` only lists the ACL messages
its stub misses, `spec-vpp-nat44-pool-ownership` uses the ACL reconcile as a pattern,
`spec-firewall-vpp-apply-before-connect` and `spec-vpp-interface-in-use` touch
start order and interface references). The ACL exists only to meet AC-8, which
this spec owns, so splitting the mechanism off would separate an acceptance
criterion from it; `ai/META-DECISIONS.md` asks for the existing spec to be amended first.
If the C-4 design grows its own firewall-side scope, the main session may split it
then.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The IPsec WAN interface leaf | `ze-ipsec-conf.yang` `vpn ipsec interface`; `resolveInterfaceAddr` (`engine/register.go`); `dynamic_wan.go` | Names the WAN; its first IPv4 address is the local address of peers that name none; Ze follows the address | Extends: by Thomas's ruling the same leaf names the interface Ze's VPP SPD is bound to. No new leaf |
| The VPP interface resolver | `iface.ResolveVPPIndex` (`internal/component/iface/resolve.go`) | Turns a logical interface name into a VPP sw_if_index, refusing when the active iface backend is not `vpp` | Extends (reused as is): the VPP IPsec backend resolves the leaf's name with it, as `fib/vpp` and `static` do |
| The VPP SPD bind | `ensureSPD`, `undoSPD`, `unbindSPD`, `b.spdBound` (`dataplane/vpp_policy.go`) | Creates the backend's one SPD and binds it to each interface a policy names | Extends: the bound interface comes from the leaf, once per backend, instead of from each policy; a leaf change unbinds the old interface and binds the new one |
| The index-0 refusal | `vppPolicyInterface` (`dataplane/vpp_policy.go`) | Refuses every policy with `IfIndex` 0 | Replaces its reading of 0: on VPP, 0 means "Ze's SPD", installed on the leaf's interface, and is refused only when no interface resolves. A non-zero index (a kernel index today) stays refused |
| The shared selector interface | `SPParams.IfIndex`, `SASelector.IfIndex` (`dataplane/dataplane.go`); `childStateScope` (`engine/child_scope.go`) | XFRM's `sel.ifindex`, copied into the SA state scope | Does not apply: the VPP interface never travels in it, so XFRM policies and states stay byte-identical (AC-2) |
| The operator SPD reconcile | `reconcileSPDPolicies`, `installSPDPoliciesAtStart` (`engine/spd_policy.go`), `isXFRMUnsupported` (`engine/child.go`) | Reads any `ErrNotSupported` install answer as "no dataplane" and accepts the commit | Extends: once VPP installs operator entries, a VPP refusal of an entry fails the apply as AC-14 of `spec-rfc4301-architecture-gaps` already does for XFRM, and at start follows that spec's start-up rule |
| The commit-time refusal for a VPP limitation | the `icmp-error-check` refusal on VPP (`spec-rfc4301-architecture-gaps`, the "VPP and `icmp-error-check`" row) | VPP, reachable through the test override, refuses at commit and names the fix | Extends: a VPP dataplane with no `vpn ipsec interface` is refused at commit the same way (AC-3) |
| The start-up rule for SPD install failures | `installUnmatchedAtStart` (`engine/unmatched.go`), `assertUnmatched` (`engine/apply.go`); doctor `ipsec-spd-install` | At start, install what it can, log at Error, report through doctor; a reload refuses | Extends: an SPD bind that fails at start (interface absent, already holding another SPD) follows the same rule |
| The dynamic WAN worker | `startAddressWorker`, `readInterfaceAddr` (`engine/dynamic_wan.go`) | Re-reads the leaf's interface on address and create events; an absent interface means "wait" | Extends: the same event retries a bind that failed because the interface was absent |
| The real-VPP evidence run | `runIPsec` (`internal/le/test/deployment/vppevidencerun.go`) | Programs a real VPP from a hand-set sw_if_index | Extends: the run names the interface by the leaf and lets the backend resolve it (AC-4) |
| The VPP firewall ACL writer | `internal/plugins/firewall/vpp` (`docs/architecture/firewall/fw-6-firewall-vpp.md`) | Merges ze-owned ACLs into an interface's vector and writes it whole with `ACLInterfaceSetACLList`, preserving foreign ACLs | Open (C-4): ruling 3a put an inbound ACL in the same vector on the WAN interface under one shared owner; that form was found unworkable as stated, and whether any ACL remains part of the mechanism is with Thomas |
| The cleartext probe | `TestVPPRealCleartextProbe` (`dataplane/vpp_real_cleartext_integration_test.go`), `internal/le/test/deployment/vppevidencecleartext.go` | Measured that VPP's SPD forwards cleartext unjudged | Extends: the same frames become AC-8's real-VPP proof once the mechanism exists |

## Design

### The proposed carrier: the VPP backend resolves the leaf itself

Thomas's ruling leaves open how the interface reaches VPP. The choice proposed
here: **the engine hands the backend the leaf's interface NAME once per apply,
through a backend-level call every backend answers, and the VPP backend resolves
it with `iface.ResolveVPPIndex` and binds its one SPD there.** Every policy keeps
`SPParams.IfIndex` 0, and the VPP backend reads 0 as "the SPD bound to the WAN
interface".

| Reason | Grounded in |
|--------|-------------|
| The interface is one fact per backend, not per policy: Ze has one SPD (`b.spdID`) and the ruling binds it to one interface. A per-policy field would copy the same value onto every policy, a second declaration of one fact (`ai/rules/principles.md`) | `ensureSPD` creates one SPD per backend |
| `SPParams.IfIndex` cannot carry it: XFRM reads it as `sel.ifindex`, and `childStateScope` copies it into the SA state scope | `dataplane.go`, `child_scope.go` |
| A VPP-only field in `SPParams` would make the engine name the VPP backend to fill it, and the engine cannot resolve a VPP index on XFRM anyway: `ResolveVPPIndex` refuses unless the iface backend is `vpp`. Only the VPP backend knows it programs a VPP index space | `iface/resolve.go`; system door, "The core knows nothing, and everything registers" |
| The name, not an index, crosses the seam, so the XFRM backend answers "node-wide, nothing to bind" and OSPF, which never makes the call, is unaffected | `ai/doors/access-security.md`, `ike/dataplane` is shared with OSPF |
| Resolution at install time follows a WAN that appears after Ze, as the leaf already promises: the bind is attempted when a policy is installed, refused naming the interface while it is absent, and retried on the dynamic WAN worker's events | YANG `interface` description; `dynamic_wan.go` |
| Operator entries, the unmatched catch-all and Child SA policies all carry `IfIndex` 0, so all land in the one SPD with no per-producer change | `childPolicyParams`, `reconcileSPDPolicies`, `unmatchedPolicies` |

The call is a method on the `Dataplane` interface rather than an optional
interface found by type assertion: a failed assertion would disable the binding
with no line in the log (`ai/rules/principles.md`), while a method makes every
backend answer. The XFRM and noop answers are a documented no-op.

The `iface` resolver is called in the IKE engine's process, as
`resolveInterfaceAddr` and `readXFRMInterface` already do for the same kind of
lookup. The IKE engine is a plugin (`runEngine`), so when it runs forked the
resolver it reaches is the subprocess's own; `ResolveVPPIndex` then refuses
loudly unless a `vpp` iface backend is active there (R-5). `./le plugin boundary
check` judges the call like the existing ones.

### VPP SPD facts the design relies on

Read from VPP source on 2026-11-20; re-checked at the lab's VPP version during
the first task.

| Fact | Consequence |
|------|-------------|
| The outbound SPD lookup uses the egress interface of the routed cleartext | Outbound policies apply to traffic routed out of the WAN interface, which is where tunnel traffic leaves |
| The inbound lookup uses the receiving interface | Inbound ESP and cleartext are judged on the WAN interface they arrive on |
| One SPD may be bound to several interfaces | A later multi-WAN extension can bind more interfaces without a second SPD |
| An interface already holding an SPD refuses a second bind (`VNET_API_ERROR_SYSCALL_ERROR_2`) | A WAN carrying another SPD (configured outside Ze) is refused with the interface named (AC-7) |
| `ipsec4-input-feature` examines only ESP, AH and UDP 500 and 4500 | The first task's risk |

## Design documents

Rows written on 2026-11-20 for the pages this spec touches. The full derivation,
`./le spec design documents spec plan/release/fixes/spec-ipsec-vpp-policy-interface.md`,
is owed before the design is presented, with every row it lists.

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the system door | "A backend that cannot apply a config exactly refuses it at verify." | agrees | An unset leaf on VPP is refused at commit, an unresolvable interface is refused at install, and an entry VPP cannot express is refused rather than dropped |
| `ai/doors/access-security.md` | Where it sits | door covering `internal/component/ike` | "`ike/dataplane` is the XFRM seam OSPF also uses for RFC 4552" | agrees | The interface crosses the seam as a backend-level name, not in `SPParams`, so OSPF's policies are unchanged and its kernel index stays refused on VPP |
| `ai/doors/forwarding.md` | Boundaries | door covering `internal/component/iface` | "Interface names go through the shared resolver" | agrees | The VPP backend resolves the leaf with `iface.ResolveVPPIndex` |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` | Decisions | `// Design:` header of `engine/child_scope.go`; Thomas's 5a ruling | "An operator SPD entry on VPP binds to every interface Ze's SPD is attached to." | agrees | Operator entries are installed in Ze's one VPP SPD, bound to the WAN leaf's interface |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` | Decisions | Thomas's interface ruling, 2026-11-20 | "On VPP, Ze's one SPD is bound to the interface `vpn ipsec interface` names." | agrees | This spec builds it |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` | SPD dispositions | page this spec modifies | "That is an open defect, not a design." | agrees | This spec fixes that defect; the paragraph is rewritten when it lands |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Following a dynamic WAN address | `// Design:` header of `engine/apply.go`, `engine/dynamic_wan.go`, `engine/register.go` | "A signal is a trigger and `resolveInterfaceAddrFn` is the truth, so a burst costs one read." | agrees | A bind refused while the WAN is absent is retried on the same worker's trigger, and the resolver stays the truth; the worker gains no second source. Its `## Decisions` rows are listed by the derivation owed above |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `set vpn ipsec interface <name>`, and `vpn ipsec policy` entries, on a daemon whose IKE dataplane is VPP (today only through `ze.test.ike.dataplane=vpp`)

### Transformation Path
1. `applyConfig` (`engine/apply.go`) reads `cfg.Interface`, refuses a VPP dataplane without one at commit, and hands the name to the loaded backend
2. The VPP backend records the name; on the next install it resolves it with `iface.ResolveVPPIndex` and binds its SPD (`ensureSPD`), unbinding a previously bound interface the leaf no longer names
3. `childPolicyParams`, `reconcileSPDPolicies` and `unmatchedPolicies` produce policies with `IfIndex` 0; `InstallPolicy` places each in the bound SPD
4. VPP classifies outbound traffic on the egress interface and inbound traffic on the receiving interface

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine to dataplane backend | The new backend-level call carrying the interface name; `InstallPolicy` with `IfIndex` 0 | No |
| Dataplane to iface | `iface.ResolveVPPIndex` | No |
| Dataplane to VPP | `ipsec_interface_add_del_spd`, `ipsec_spd_entry_add_del_v2` (GoVPP binary API) | No |

### Changed-Wire Census

**Search evidence:** scope `internal/component/ike/`, `internal/plugins/ospf/`, `internal/le/test/deployment/`; terms `IpsecInterfaceAddDelSpd`, `IpsecSpdEntryAddDelV2`, `IfIndex`, `ZE_VPP_IPSEC_SW_IF_INDEX`; positive control `ensureSPD` (found, `vpp_policy.go`). No config syntax, plugin protocol or peer-visible wire changes: the leaf exists, and the IKE and ESP bytes are unchanged. The one changed wire is the VPP binary API traffic Ze sends.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| VPP `ipsec_interface_add_del_spd` | writer | `ensureSPD`, `unbindSPD` (`dataplane/vpp_policy.go`) | sends `IpsecInterfaceAddDelSpd` with the bound sw_if_index | change: the index comes from the leaf, and a leaf change sends an unbind then a bind |
| VPP `ipsec_interface_add_del_spd` | reader | VPP's SPD interface bind (VPP source) | the bind refuses an interface already holding an SPD | preserve; the refusal is reported (AC-7) |
| VPP `ipsec_spd_entry_add_del_v2` | writer | `InstallPolicy`, `spdEntry` (`dataplane/vpp_policy.go`) | now reached by IKE and operator policies | change: entries reach VPP; rekey replacement per AC-5 |
| VPP `ipsec_spd_entry_add_del_v2` | reader | VPP's SPD (VPP source) | matches an entry by its whole content | preserve |
| `ZE_VPP_IPSEC_SW_IF_INDEX` | writer | `runIPsec` (`internal/le/test/deployment/vppevidencerun.go`) | the probe's hand-set index | replace: the run names the interface instead (AC-4) |
| `ZE_VPP_IPSEC_SW_IF_INDEX` | reader | `TestVPPRealDataplaneInstalls` (the probe the run executes) | reads the index | replace with the interface name |

### Integration Points
- `applyConfig` and the dynamic WAN worker - hand the leaf's name to the backend, retry a bind the interface's absence refused
- `reconcileSPDPolicies`, `installSPDPoliciesAtStart` - a VPP refusal of an operator entry fails the apply, or at start is logged and reported
- Doctor `ipsec-spd-install` and `ipsec-interface` - report a bind that failed at start

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The name flows config to engine to backend to VPP; no engine call reaches VPP directly |
| No unintended coupling (components stay isolated) | Yes | The engine names no backend; the VPP backend calls `iface`, as the engine's `resolveInterfaceAddr` already does |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses the leaf, `ResolveVPPIndex`, `ensureSPD` and the dynamic WAN worker (Existing Mechanisms) |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Control-plane path only; no per-packet work |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The interface is a method every registered backend answers, not a VPP field in `SPParams` |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | No new name is introduced; searched `dataplane.Register` callers and `ikeDataplaneName` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | VPP's inbound SPD judges cleartext arriving on a bound interface | RFC 4301 Section 5.2; contradicted by a read of `ipsec_input.c` | 5a cannot be met for inbound cleartext; back to Thomas | First task, a real-VPP measurement | broken (measured 2026-11-20; Thomas ruled 3a, found unworkable as stated, C-4 open) |
| A-2 | Ze is the only writer of an SPD on the WAN interface | the appliance runs Ze's VPP | a bind is refused | AC-7's test against a pre-bound interface | unvalidated |
| A-3 | `iface.ResolveVPPIndex` answers in the IKE engine's process | `fib/vpp` and `static` call it; `resolveInterfaceAddr` calls `iface.Addresses` there | the backend must get the index another way | a unit test with the vpp iface backend loaded, and the real-VPP run | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Cleartext inbound is never tested against the SPD (first task) | `unprocessed` counter rises, the DISCARD entry's counter does not | Stop; report to Thomas with the measurement |
| R-2 | A WAN created after Ze has no sw_if_index at first install | the bind is refused with the interface named | Retry on the dynamic WAN worker's events, as peers already wait |
| R-3 | A leaf change on reload leaves the SPD bound to the old interface | VPP's interface dump shows both | Unbind the old interface in the same apply (AC-7) |
| R-4 | Making operator entries effective on VPP exposes the rekey and upsert defects this spec owns | duplicate SPD entries after a rekey | AC-5 and the owned journal rows |
| R-5 | The IKE engine forked: the subprocess's iface resolver has no `vpp` backend | `ResolveVPPIndex` refuses, naming the active backend | The refusal is loud by construction; the forked case is tested, and if it refuses, the design returns to C-1 before implementation proceeds |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | VPP only, reachable through the test override: policies installed on the wrong interface, or operator entries that do not decide traffic. XFRM is untouched |
| How is it reverted? | A single commit revert; no config migration, because the leaf already exists |
| Who else touches this path? | `spec-rfc4301-architecture-gaps` (decision 5a, operator entries), `spec-ipsec-vpp-dual-form-proof`, `spec-ipsec-dataplane-selector`, `spec-ipsec-vpp-dataplane-readback`, `spec-ipsec-spd-final-discard-vpp-ospf`, the OSPF RFC 4552 installer |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `vpn ipsec interface` committed on a VPP dataplane | → | the backend-level interface call, `ensureSPD` | `TestVPPSPDBindsToTheIPsecWANInterface` |
| A `vpn ipsec policy` entry committed on a VPP dataplane | → | `reconcileSPDPolicies`, `InstallPolicy` | `TestVPPOperatorEntryLandsInTheWANSPD` |
| A Child SA established on a VPP dataplane | → | `childPolicyParams`, `InstallPolicy` | `TestVPPChildSAPolicyLandsInTheWANSPD` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `vpn ipsec interface <wan>` is set, the VPP dataplane is loaded, and an IKE Child SA is established | every policy of the Child SA is installed in Ze's one VPP SPD, and that SPD is bound to the sw_if_index `iface.ResolveVPPIndex` returns for `<wan>`, read back from VPP's SPD interface dump | VPP lets one SPD bind to several interfaces; Ze binds only the leaf's interface, by Thomas's ruling |
| AC-2 | The same configuration on the XFRM dataplane | the installed XFRM policies and SA states carry interface index 0 and are byte-identical to today's, read back from the kernel | OSPF's RFC 4552 kernel ifindex and `xfrmInterfaceExemptions`' kernel index keep their non-zero values, which they carry today |
| AC-3 | The VPP dataplane with no `vpn ipsec interface`, or with an interface `iface.ResolveVPPIndex` cannot resolve | with no leaf, the commit is refused, naming the VPP dataplane's need and the fix `set vpn ipsec interface <name>`; with a leaf naming an interface that is absent or not a VPP interface, the install is refused naming the interface and what would have been programmed (the SPD and its entries), and the message names `show interface` to find a VPP interface name. At start both follow the start-up rule: logged at Error and reported by `ze doctor` with the same fix | A leaf naming an interface the host does not carry yet is accepted at commit, as the leaf already promises for a late WAN; its SPD is bound when the interface appears |
| AC-4 | The real-VPP evidence run (`runIPsec`, `internal/le/test/deployment/vppevidencerun.go`) | the run drives an IKE-shaped policy with `IfIndex` 0 and a named WAN interface, and VPP's dump shows the SPD bound to that interface | The hand-set `ZE_VPP_IPSEC_SW_IF_INDEX` path is removed, not kept beside the new one (`ai/rules/no-layering.md`) |
| AC-5 | A Child SA rekey over the VPP backend | ONE SPD entry per direction names the live SA, and the retired entry AND the retired SA are gone: from VPP, from `b.spdEntries` and from `b.sadIDs`. A delete VPP reports retval 0 for while keeping the object is not read as success. Proven against a real VPP | None: a retired entry or SA that survives is the leak this criterion exists for |
| AC-6 | A `vpn ipsec policy` entry (bypass or discard) committed on the VPP dataplane with the leaf set | the entry is installed in the same SPD as the Child SA policies, at its order, and decides matching traffic on the WAN interface; an entry VPP cannot express exactly (an ICMP type or code, which VPP does not compare) is refused at commit with the entry named and its fix; an install VPP refuses fails the apply as AC-14 of `spec-rfc4301-architecture-gaps` requires | Inbound cleartext is not decided by the SPD entry on VPP (measured, first task outcome); Ze owes it as AC-8 states, mechanism pending (C-4) |
| AC-7 | The leaf changes on reload from `<a>` to `<b>`, or `<b>` already holds an SPD Ze did not bind | on a change the SPD is unbound from `<a>` and bound to `<b>` in the same apply, read back from VPP; a bind VPP refuses because `<b>` holds another SPD fails the apply naming `<b>` and the cause, and at start is logged and reported by doctor | None |
| AC-8 | On a real VPP with `vpn ipsec interface <wan>` set, cleartext arrives on `<wan>` matching an inbound PROTECT entry, an inbound DISCARD entry, and an inbound BYPASS entry; ESP and IKE (UDP 500 and 4500) for a live Child SA arrive on the same interface | cleartext matching the inbound PROTECT or DISCARD entry is dropped, read from VPP's counters; cleartext matching the inbound BYPASS is delivered; ESP and IKE keep reaching the SPD and the IKE sockets, unaffected; traffic the Child SA decrypts is delivered, unaffected | None. Mechanism (C-4, decided "B"): Ze's VPP patch `0106-ipsec-judge-unprotected-inbound-traffic.patch` in the image the run boots. Proven by the cleartext proof of `./le test deployment vpp` on `ze-vpp-deployment` (green 2026-11-20, C-4 table); upstream VPP's own verdict (forwarded unjudged, measured 2026-11-20) does not meet it and is the proof's red |
| AC-9 | An inbound (`SADirIn`) policy installed on the VPP backend (found in passing, 2026-11-20) | the SPD entry's local range and port are the policy's `Dst` and `DstPort`, its remote range and port `Src` and `SrcPort`; an outbound policy keeps `Src` local; removal, the vppctl delete command and `Close` send back that same entry; on a real VPP an inbound DISCARD counts 'discard' for ESP from its remote range (`probe-esp-discard`) and not for the exchanged pair (`probe-esp-discard-reversed`) | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets `vpn ipsec interface` and brings up a site-to-site tunnel on VPP | config, `applyConfig`, backend bind, `InstallPolicy`, VPP SPD | the real-VPP evidence run (AC-4) |
| 2 | Adds a `vpn ipsec policy` discard on VPP | config, `reconcileSPDPolicies`, `InstallPolicy`, VPP SPD | `TestVPPOperatorEntryLandsInTheWANSPD`, and the first task's measurement |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVPPSPDBindsToTheIPsecWANInterface` | `internal/component/ike/dataplane/vpp_policy_test.go` | AC-1, AC-7: bind, rebind | not written |
| `TestVPPOperatorEntryLandsInTheWANSPD` | `internal/component/ike/dataplane/vpp_policy_test.go` | AC-6 | not written |
| `TestVPPChildSAPolicyLandsInTheWANSPD` | `internal/component/ike/engine/` | AC-1 from the engine | not written |
| `TestVPPWithoutIPsecInterfaceRefusedAtCommit` | `internal/component/ike/engine/` | AC-3 | not written |
| `TestXFRMPoliciesKeepInterfaceIndexZero` | `internal/component/ike/dataplane/` | AC-2 | not written |
| `TestVPPInstallPolicyInboundSides` | `internal/component/ike/dataplane/rfc7296_vpp_message_test.go` | AC-9 | written 2026-11-20, not yet run |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| resolved sw_if_index | 1 to 4294967295 | 4294967295 | 0 (refused by `ResolveVPPIndex`) | N/A (u32) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-vpp-without-interface-refused` | `test/parse/` | commit on a VPP dataplane with no `vpn ipsec interface` is refused with the fix | not written |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| the real-VPP IPsec evidence run | `internal/le/test/deployment/vppevidencerun.go` | real VPP | AC-1, AC-4, AC-5, AC-6, AC-8 on VPP itself | not written |

## Files to Modify
- `internal/component/ike/dataplane/dataplane.go` - the backend-level interface call on `Dataplane`; the `SPParams.IfIndex` comment (count of refusals, the spec path)
- `internal/component/ike/dataplane/vpp_policy.go` - `vppPolicyInterface` reads 0 as the bound SPD; `ensureSPD` binds from the resolved leaf; rebind; the stale "one producer" comment; `vppSPDSelector` maps an inbound policy's Dst to the local side (AC-9, done 2026-11-20)
- `internal/component/ike/dataplane/vpp.go` - record the interface name; rekey replacement (AC-5); the `RemoveSA` comment's spec path
- `internal/component/ike/dataplane/xfrm_linux.go` and the noop backend - the documented no-op answer
- `internal/component/ike/engine/apply.go`, `internal/component/ike/engine/dynamic_wan.go` - hand the leaf to the backend; refuse a VPP dataplane with no leaf; retry a refused bind
- `internal/component/ike/engine/spd_policy.go` - a VPP refusal of an operator entry fails the apply
- `internal/component/ike/engine/child.go` - rekey policy replacement (AC-5)
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - the `interface` and `policy` descriptions, once VPP installs operator entries
- `internal/le/test/deployment/vppevidencerun.go` - AC-4
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - "SPD dispositions" once the defect is fixed
- `docs/guide/ipsec.md` - the VPP paragraph that names this spec

## Files to Create
- `test/parse/ipsec-vpp-without-interface-refused.ci` - AC-3 at commit

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | the leaf exists; only descriptions change |
| YANG validation constraints | No | no new leaf |
| YANG custom validators | No | the refusal is a commit verify in the engine |
| CLI commands/flags | No | none |
| CLI grammar (keyword before value) | N-A | no command added |
| Editor autocomplete | N-A | existing leaf |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no output |
| Env var registration | No | `ze.test.ike.dataplane` exists |
| Doctor check for runtime dependencies | Yes | the VPP SPD bind: `ipsec-spd-install` reports a bind refused at start |
| Doctor code per problem class | Yes | unresolvable WAN interface on VPP; interface already holding an SPD |
| Prometheus counters/metrics | No | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | VPP stays behind the test override |
| 2 | Config syntax changed? | No | descriptions only |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` |
| 7 | Wire format changed? | No | IKE and ESP unchanged |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4301.md` rows for SPD processing on VPP, once measured |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md`, the VPP evidence run |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived by `./le spec citation anchors spec plan/release/fixes/spec-ipsec-vpp-policy-interface.md` before implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ipsec.md` |

## Owed on the first touch of `dataplane.go`: the `SPParams.IfIndex` comment under-counts

The comment on `SPParams.IfIndex` (`internal/component/ike/dataplane/dataplane.go`) says
"Three refusals keep that inert today" and names them: the `SADirFwd` direction refusal
in `spdEntry`, the transport-mode refusal in `vppProtectMode` (both `vpp_policy.go`), and
the unset-`Dir` refusal in `vppUnsupportedSA` (`vpp.go`).

**There is a fourth on the same path.** `buildIPsecPolicies`
(`internal/plugins/ospf/ipsec_install.go`) sets no `SAID`, and `SPActionProtect` is the
zero value of `SPParams.Action`, so every OSPF policy reaches `spdEntry` as a PROTECT
policy with `SAID` 0, which `spdEntry` refuses. It is reached only after
`vppProtectMode`, so widening the mode refusal alone still leaves it closed.

The comment's point survives: the count is an under-count, not a false claim. Correct the
count, and the path of this spec it names, on the first edit of that file.

## What this spec makes reachable: the rekey policy leak, and the SA under it

**Building this turns a latent defect live, so it is in scope here and it is not a
separate find.** No IKE policy reaches the VPP backend today, so no IKE rekey does
either. Supplying the interface removes exactly that block.

`xfrmBackend.InstallPolicy` (`internal/component/ike/dataplane/xfrm_linux.go`) UPSERTS
with `XfrmPolicyUpdate`, and `removeChildSAExcept`
(`internal/component/ike/engine/child.go`) is built on that behaviour: a make-before-break
rekey holds ONE shared policy per direction, so it passes `dropPolicy=false` and never
sends the retired child's policy back to the dataplane.

The VPP backend does not upsert. `InstallPolicy`
(`internal/component/ike/dataplane/vpp_policy.go`) sends `ipsec_spd_entry_add_del_v2` with
`IsAdd` true and appends the entry to `b.spdEntries`. VPP matches an SPD entry by its
whole content, and a replacement child's entry differs in `SaID`: a rekey carries a new
SPI (RFC 7296 Section 2.8), which is a new `saIdentity` and a new SAD id (`allocSadID`,
`vpp.go`). So one rekey would leave VPP holding two entries per direction, and the retired
one would stay in `b.spdEntries` and in VPP until `Close`, naming a retired SA whose
reference VPP is still counting.

**The retired SA is leaked past `Close`, which is worse than the entry.** Found on
2026-08-10 by reading the whole chain. `removeChildSAOutgoing` and
`removeChildSAIncoming` (`internal/component/ike/engine/child.go`) call
`RemovePolicyParams` only when `dropPolicy` is true, and call `RemoveSA` unconditionally.
On a rekey the retired SA's PROTECT entry therefore still holds VPP's reference when
`RemoveSA` runs, `deleteSAD` returns retval 0 with the SA still installed (the behavior
`removeInstalled` measured on VPP v26.06), and `RemoveSA` drops the identity from
`b.sadIDs` on that success. `removeInstalled` iterates `b.sadIDs` for its SA deletes, so
the retired identity is no longer in the population it walks and no delete is ever sent.
The SA lives until VPP restarts. Recorded in
`plan/journal/false-synchronization-claim.md`.

Two shapes answer AC-5, and choosing between them is part of this spec's design work
(C-3): give the VPP backend a replace step keyed on the selector, or give the engine a
way to say "this policy now names a different SA" that both backends honor.

## Design presentation

### What changes

On the VPP dataplane, Ze binds its one SPD to the interface `vpn ipsec interface`
names, and installs every Child SA policy and every operator SPD entry there. A VPP
dataplane with no such leaf is refused at commit with the command that fixes it, and
an operator entry VPP cannot install fails the commit instead of being dropped. XFRM
behaves exactly as today.

### Before and after

1. An operator commits a `vpn ipsec policy` discard entry on VPP. Before: the commit succeeds and nothing is installed. After: the entry is in the SPD bound to the WAN interface (its readback belongs to `plan/post-release/features/spec-ipsec-vpp-dataplane-readback.md`).
2. An operator brings up a tunnel on VPP with `vpn ipsec interface` unset. Before: the Child SA's policies are refused at install with "VPP has no node-wide SPD". After: the commit is refused and names `set vpn ipsec interface <name>`.
3. An operator moves `vpn ipsec interface` from one VPP interface to another. Before: not reachable. After: the SPD moves with the leaf in the same apply.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Per-Child-SA route lookup toward the peer (`iface.RouteLookup`) | Rejected by Thomas: the SPD moves with routing |
| Bind to the interface holding the local address (strongSwan's VPP plugin) | Rejected by Thomas: breaks with the address on a loopback |
| Carry the VPP index in `SPParams.IfIndex` | XFRM reads it as `sel.ifindex` and `childStateScope` copies it into the SA state scope, so XFRM would change |
| A VPP-only field in `SPParams`, filled by the engine | One fact copied onto every policy; the engine would have to name the VPP backend, and `ResolveVPPIndex` refuses on a non-VPP iface backend |
| An optional backend interface found by type assertion | A failed assertion disables the binding with no log line |
| Refuse operator entries on VPP (decision 7 option b) | Thomas chose to pull this spec forward instead |

### Documents departed from

none

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | How the interface reaches the VPP backend | (a) recommended: a backend-level call carrying the leaf's name, resolved by the VPP backend; (b) a VPP-only field in `SPParams` filled by the engine | open |
| C-2 | Whether `spec-ipsec-vpp-dual-form-proof` and `spec-ipsec-dataplane-selector` also come into release scope | (a) both stay post-release; (b) one or both come forward | DECIDED 2026-11-20 (his decision 2): both come into the first release; moved to `plan/release/validation/` and `plan/release/fixes/` |
| C-4 | The mechanism that drops inbound cleartext matching an inbound PROTECT or DISCARD entry on VPP (AC-8) | Ruled 3a (an inbound ACL derived on the WAN interface, one owner shared with the VPP firewall backend); found unworkable as stated on 2026-11-20 (it also drops decrypted tunnel traffic, `esp_decrypt.c` v26.06); B, a VPP patch: decryption marks the packet, IPsec input judges unmarked cleartext | DECIDED 2026-11-20: B (Thomas's rulings) |
| C-3 | The two AC-5 shapes | (a) a VPP replace step keyed on the selector; (b) an engine "same policy, new SA" call both backends honor | open |

## Implementation Steps

1. **Phase: First task, the cleartext inbound measurement** -- on a real VPP, before any code
   - Tests: the measurement of AC-8
   - Verify: the result decides whether the design stands (First task table)
2. **Phase: Wiring** -- the backend-level interface call, the engine handing it the leaf, the commit refusal
   - Tests: `TestVPPSPDBindsToTheIPsecWANInterface`, `TestVPPWithoutIPsecInterfaceRefusedAtCommit`
   - Files: `dataplane.go`, `vpp_policy.go`, `apply.go`
   - Verify: tests fail, then pass
3. **Phase: Policies and operator entries** -- index 0 lands in the bound SPD; an operator refusal fails the apply
   - Tests: `TestVPPChildSAPolicyLandsInTheWANSPD`, `TestVPPOperatorEntryLandsInTheWANSPD`, `TestXFRMPoliciesKeepInterfaceIndexZero`
   - Files: `vpp_policy.go`, `spd_policy.go`
   - Verify: tests fail, then pass
4. **Phase: Rekey and the owned journal rows** -- AC-5
   - Files: `vpp.go`, `child.go`
   - Verify: the rekey unit tests fail, then pass
5. **Phase: Real-VPP evidence** -- AC-4, AC-5, AC-6 on VPP
   - Files: `vppevidencerun.go`
   - Verify: the run's SPD and interface dumps
6. **Phase: AC-8 on the patched VPP** (C-4, decided "B") -- patch, image and proof written 2026-11-20
   - Files: `third_party/vpp-patches-v26.06/`, `internal/le/build/vpp/deployment.go`, `internal/le/interoplab/docker_image_load.go`, `internal/le/test/deployment/vppiface.go`, `vppevidencecleartext.go`
   - Verify: `./le build vpp deployment` compiles the patched VPP; `./le test deployment vpp` passes the cleartext proof on it and fails it on `ligato/vpp-base:26.06-release`
   - Follow-up: an RFC 4301 Section 5.2 tag on the proof, through `./le rfc record`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | XFRM policies and states byte-identical; OSPF's kernel index still refused on VPP |
| Naming | The backend-level call names an interface, not a VPP index |
| Data flow | The engine never names the VPP backend; only the VPP backend calls `ResolveVPPIndex` |
| Operator errors and doctor codes | The unset-leaf refusal and the bind refusals name their fix and doctor code |
| Rule: `ai/rules/no-layering.md` | The hand-set evidence index is removed, not kept beside the leaf path |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| SPD bound to the WAN leaf on VPP | the real-VPP run's SPD interface dump |
| Operator entries installed on VPP | the real-VPP run's SPD dump |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail open | An unresolvable interface or a refused bind must not leave traffic unjudged while the commit reports success |
| Input validation | The leaf's name is resolved, never parsed as an index |

### Failure Routing
| Failure | Route To |
|---------|----------|
| The first task shows cleartext is not judged | Happened 2026-11-20: Thomas ruled 3a, found unworkable as stated; AC-8 waits on C-4 |
| Compilation error | Fix in the phase that introduced it |
| Test fails on behavior mismatch | Re-read the source in Current Behavior |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The silent drop of operator entries on VPP is the release reason, and it comes from `isXFRMUnsupported` reading any `ErrNotSupported` as "no dataplane": a refusal that exists for a real platform limit also swallows a refusal that means "this backend lacks an input".

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `upsert-assumed-across-backends.md` | 2026-10-26 | `adoptStrandedLocked` assumes XFRM upsert; VPP keeps the old SPD entry |
| `upsert-assumed-across-backends.md` | 2026-10-26 | `recordedEntry` refuses identical duplicate or orphaned SPD entries forever |
| `upsert-assumed-across-backends.md` | 2026-10-26 | VPP fast-path delete of an absent entry fails, so the retry stays stranded |
| `identifier-reused-after-its-owner-is-gone.md` | 2026-10-26 | `liveSADID` resolves a new SA that reused a removed SA's identity |
| `false-synchronization-claim.md` | 2026-08-10 | VPP `RemoveSA` on a rekey leaks the retired SA past `Close` (AC-5) |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The SPD binds to the `vpn ipsec interface` leaf (Thomas, 2026-11-20) | route lookup per Child SA; interface holding the local address | His ruling; see "Thomas's rulings" |
| Proposed: the name crosses the seam once per backend, the VPP backend resolves it | VPP-only `SPParams` field; the shared `IfIndex` | Design, "The proposed carrier"; awaiting C-1 |

## Known Limitations

- One WAN interface. Several WANs would bind the one SPD to several interfaces, which VPP allows; nothing in this design prevents it, and no request for it exists.
- VPP stays reachable only through `ze.test.ike.dataplane` until `plan/release/fixes/spec-ipsec-dataplane-selector.md` lands, which also needs the ESP proof of `plan/release/validation/spec-ipsec-vpp-dual-form-proof.md` (C-2, both first-release since 2026-11-20).

## RFC Documentation (Scope: protocol)

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 4301 Sections 4.4.1, 5.1, 5.2 | VPP dataplane with `vpn ipsec interface` set: outbound and inbound traffic on the WAN interface are judged against Ze's SPD, operator entries included | XFRM node-wide SPD; OSPF interface-scoped policies on XFRM | AC-1, AC-6, AC-8 tests | AC-2's XFRM byte-identity test |

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
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: `./le test unit pkg path ./internal/component/ike/dataplane/` (with the `ze_vpp` tag the toolchain supplies), `./le test unit pkg path ./internal/component/ike/engine/`, `./le test functional parse` for the new `.ci`, and the real-VPP IPsec evidence run
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-ipsec-vpp-policy-interface.md` only, in the same `./le commit create` script
