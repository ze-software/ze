# Spec: rfc4301-architecture-gaps

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | `plan/spec-ipsec-lifetime-volume.md` (owns the Section 4.4.2.1 byte-count SAD lifetime); `plan/spec-rfcgate-6-supported-extraction-signoff.md` (owns the `rfc4301` extraction sign-off and the ledger scope set) |
| Phase | - |
| Deferral shard | `plan/deferrals/rfc4301-architecture-gaps.md` |
| Handoff | - |
| Updated | 2026-08-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`docs/features/rfc-status.md` claimed RFC 4301 was `Supported` until 2026-08-30. An
extraction walk of `rfc/full/rfc4301.txt` found MUST-level architecture obligations Ze
does not meet, and none of them was on the checklist in `rfc/short/rfc4301.md`. The owner
decided on 2026-08-30 to correct the public row to `Partial` with the gaps disclosed and
to open this spec for the implementation work, one phase per block, security-bearing
blocks first.

The row now reads `Partial` and names this spec. This spec implements the obligations and
returns the row to `Supported` behind a landed extraction sign-off.

**Why the fragment block is first.** `ikeBypassPolicies`
(`internal/component/ike/engine/bypass.go`) installs BYPASS policies scoped to UDP port
500 and 4500 with any-address selectors. Ze holds no fragment state, so a forged
non-initial fragment whose addresses and protocol match one of those policies is passed in
the clear. That is the attack RFC 4301 Section 7.4 and Appendix D.4 describe in their own
words, reachable on a running daemon, and it is not disclosed by any `{gap}` annotation.

**Why the block list is not the whole RFC.** Two blocks the walk touched are homed
elsewhere and are NOT re-implemented here. The Section 4.4.2.1 byte-count SAD lifetime is
designed in `plan/spec-ipsec-lifetime-volume.md`, which this spec depends on. The
extraction sign-off artifact `rfc/extraction/rfc4301.json` belongs to
`plan/spec-rfcgate-6-supported-extraction-signoff.md`, and this spec's last phase lands
under that spec's rules rather than replacing them.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - the Child SA to XFRM projection, declared by `dataplane.go`, `bypass.go`, `child.go`
  → Decision: [what the page states about how an SPD entry reaches the kernel]
  → Constraint: [what the page says a backend that cannot express a policy must do]
- [ ] `docs/architecture/ike/ipsec-dataplane-inspection.md` - how installed policy and state are read back
  → Decision: [what the page states about drift detection between Ze and the kernel]
- [ ] `docs/guide/ipsec.md` - the operator-facing IPsec surface every new config leaf lands in
  → Constraint: [the config grammar a new SPD surface must match]
- [ ] `docs/architecture/testing/interop.md` - the strongSwan suite and the vacuity traps
  → Constraint: [the red-phase proof each new scenario owes]

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4301.md` - the current checklist, 18 requirement ids, one `{gap}`
  → Constraint: the summary's prose is wrong about the implementation and this spec repairs it (see Current Behavior)
- [ ] `rfc/short/rfc7296.md` - traffic-selector negotiation and narrowing, which feeds every SPD entry IKE creates
  → Constraint: a selector Ze puts on the wire is the selector Ze programs, so a new selector field is negotiated or refused, never rounded outward
- [ ] `rfc/short/rfc4303.md` - ESP, the transform an SPD PROTECT entry names

**Key insights:** (minimal context to resume after compaction)
- The RFC's own text is the authority. `rfc/short/rfc4301.md` is a derived artifact and disagrees with the code today.
- Every obligation below was read in `rfc/full/rfc4301.txt` and every producer was read in the tree on 2026-08-30.

## Current Behavior (MANDATORY)

**Source files read:** (read on 2026-08-30, before this spec was written)
- [ ] `internal/component/ike/dataplane/dataplane.go` - `SPAction` defines `SPActionProtect` (0) and `SPActionBypass` (1) and nothing else. `SPParams` carries `Src`, `Dst`, `Dir`, `Proto`, `Mode`, `IfID`, `ReqID`, `Action`, `Owner`, `Priority`, `SrcPort`, `DstPort`, `UpperProto`, `IfIndex`, `TunnelSrc`, `TunnelDst`, `SAID`. `PortMatch` holds a port and a mask, and `AnyPortMatch` and `ExactPortMatch` are its two constructors. `PriorityIKEBypass` is 100 and `PriorityChildSA` is 2000, both fixed constants.
- [ ] `internal/component/ike/engine/bypass.go` - `ikeBypassPolicies` builds four BYPASS policies per address family: an outbound pair matching `SrcPort` exactly on UDP 500 and 4500 with any destination port, and an inbound pair with the ports reversed. Source and destination prefixes are the any-address net.
- [ ] `internal/component/ike/engine/child.go` - `createFirstChildSA` sets the Child SA mode to tunnel, and to transport when `sa.UseTransportMode` is set. `childPolicyParams` builds the `SPParams` for a negotiated Child SA.
- [ ] `internal/component/ike/engine/remote_id.go` - `remoteIDMatches` compares an asserted identity with the configured `remote-id` after a class gate. ID_FQDN and ID_RFC822_ADDR compare with `asciiEqualFold`, ID_DER_ASN1_DN renders to RFC 4514 and compares with `asciiEqualFold`, ID_KEY_ID compares byte for byte, and an address parses through `netip.ParseAddr` and compares as one value.
- [ ] `internal/component/ike/engine/rekey.go` - `lifetimeState` holds `softTime`, `hardTime`, `softBytes` and `byteCount`. `newLifetimeState` assigns `softTime` and `hardTime` only. `softExpired` reads `softBytes` in its second arm, and that arm is unreachable in a running daemon because nothing assigns the field outside tests.
- [ ] `internal/component/ike/engine/initiator.go` - `tsToIPNet` is deleted; a comment at the deletion site records that `wireToSelectors` in `ts_narrow.go` replaced it, converts every selector, and keeps the port and the protocol.
- [ ] `internal/plugins/ospf/ipsec_install.go` - the RFC 4552 manual path installs transport-mode ESP or AH through the same dataplane seam.
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - the operator config surface. `vpn ipsec` carries `esp-group`, `ike-group`, `remote-access` and `site-to-site peer`, and a peer carries a `traffic-selector` list with `local` and `remote` prefixes and ports. There is no operator-authored SPD entry, no policy action leaf and no policy order leaf.
- [ ] `docs/features/rfc-status.md` - the RFC 4301 row, now `Partial`, naming this spec.
- [ ] `rfc/short/rfc4301.md` - 18 requirement ids, one `{gap}` on `RFC4301-4.4.1.1-1`, seven `{not-applicable}` annotations, and prose that says Ze delegates IKEv2 negotiation to strongSwan and charon.

**Behavior to preserve:**
- The four IKE bypass policies keep exempting Ze's own IKE sockets. A Child SA policy that captured IKE would prevent its own rekey and teardown.
- `SPActionProtect` stays the zero value of `SPAction`, so a caller that forgets the field gets a policy that protects rather than one that passes traffic in the clear.
- A backend that cannot express a policy refuses the install rather than widening or downgrading it.
- Every existing `test/interop-ipsec/scenarios/` scenario keeps passing against strongSwan.

**Behavior to change:**
- An SPD entry can carry a DISCARD action, an ICMP type and code selector, and a port range.
- A BYPASS entry with a non-trivial port range is served by stateful fragment checking, or its install is refused.
- An administrator can accept or reject unauthenticated ICMP error messages at ICMP-type granularity, on both sides of the IPsec boundary.
- A protected transit ICMP error message can be checked for consistency between its payload header and the traffic selectors of the SA that carried it.
- An administrator can order SPD entries totally through the management interface.
- The DF treatment of a tunnel-mode SA is configurable as set, clear, or copy from the inner header.
- A per-SA PMTU is held and aged.
- The outer tunnel header's DSCP value is mapped for the domain the packet enters.
- PAD identity matching accepts a sub-tree for a DN, a DNS name and an RFC 822 address, and an address range.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator config under `vpn ipsec` in `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`, parsed into the ipsec config types.
- IKEv2 traffic selectors arriving on the wire, narrowed by `wireToSelectors` and `ts_narrow.go`.
- ICMP error messages arriving on the unprotected side, on the protected side, and inside an SA.
- IP fragments arriving at an interface a BYPASS policy covers.

### Transformation Path
1. Config or negotiation produces a policy description in the ipsec config types.
2. `childPolicyParams` and the OSPFv3 installer build `dataplane.SPParams` and `dataplane.SAParams`.
3. The backend selected at runtime writes them: `xfrm_linux.go` through netlink, `vpp_policy.go` through the VPP API.
4. The kernel or VPP classifies each packet against the installed SPD and SAD.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to engine | YANG tree to ipsec config value types | No |
| Engine to dataplane | `dataplane.SPParams` and `dataplane.SAParams` value types | No |
| Dataplane to kernel | netlink XFRM messages | No |
| Dataplane to VPP | VPP binary API | No |
| Ze to peer | IKEv2 traffic selector payloads | No |

### Integration Points
- `dataplane.SPAction` - gains a DISCARD member, and every backend must express it or refuse.
- `dataplane.SPParams` - gains ICMP type and code selector fields and a port range form.
- `dataplane.SPParams.Priority` - stops being two fixed constants and carries an operator order.
- `remoteIDMatches` - gains sub-tree and range matching.
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - gains the new leaves.
- `internal/core/diagnostic/codes.go` - gains a code for each new runtime dependency a doctor check reads.

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
| A-1 | `SPAction` has exactly two members today, so a DISCARD entry cannot be expressed at all | read at `internal/component/ike/dataplane/dataplane.go`, `SPAction` and its two constants, 2026-08-30 | the block is already partly implemented and phase 2 shrinks | re-read the type at the start of phase 2 | confirmed |
| A-2 | Ze holds no IP fragment state anywhere in the IPsec path | a case-insensitive search for `fragment` over `internal/component/ike/` returned only EAP-TLS message fragmentation, 2026-08-30 | phase 2 has an existing mechanism to extend rather than to add | re-run the search at the start of phase 2 and read every hit | confirmed |
| A-3 | The Linux kernel does not discharge the Section 7.4 stateful fragment obligation for a template-free XFRM policy on its own | XFRM classifies a non-initial fragment on addresses and protocol alone, because the transport header it would need is not present | the obligation is kernel-delegated and the phase becomes a test plus a recorded evidence trail | write the QEMU test that forges a non-initial fragment against a port-scoped BYPASS policy and read what the kernel does, before designing the fix | unvalidated |
| A-4 | The Section 8.2.2 PMTU aging obligation is met by the kernel's own PMTU expiry for an XFRM route | the kernel ages a dst entry's PMTU, and Ze holds no PMTU value of its own | the phase must add a per-SA PMTU field, its aging timer and its reset period | read the producing kernel path and prove it with a QEMU test that observes the PMTU changing back after the period | unvalidated |
| A-5 | The seven `{not-applicable}` annotations on `rfc/short/rfc4301.md` are void under the owner directive of 2026-07-27 and must be re-answered rather than cited | `ai/rules/rfc-compliance.md`, "Every earlier answer that pointed away from full compliance or full proof is VOID" | the annotations stand and phase 9 shrinks to a prose repair | read each annotation's producing function and record the fresh answer | unvalidated |
| A-6 | The VPP backend can express a DISCARD policy | `vpp_policy.go` builds an SPD entry, and VPP's SPD model carries a discard action | the VPP backend refuses the install, which is the correct fail-closed behavior, and the refusal needs its own test | read `spdEntry` in `vpp_policy.go` and the VPP API binding at the start of phase 2 | unvalidated |
| A-7 | No existing interop scenario would break when `SPAction` gains a member and `SPParams` gains fields | the zero value of every new field means "as before", and `SPActionProtect` stays 0 | a landed scenario reds and the change is wider than one enum member | run the whole `test/interop-ipsec` suite at the end of phase 2 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A phase classifies an obligation `{not-applicable}` because the kernel performs the packet-level half, and the gate goes green over an unmet MUST | a new `{not-applicable}` annotation appears in `rfc/short/rfc4301.md` | `ai/rules/rfc-compliance.md` reserves that call to the owner. The kernel performing a step is evidence to put to him, never a classification a phase makes. Ask which way to fix it |
| R-2 | The fragment work grows into a general connection-tracking subsystem | phase 2 starts naming a new package outside `internal/component/ike/` | the obligation is scoped to BYPASS and DISCARD entries with a non-trivial port range. Reassembly for exactly those entries is the deliverable (`ai/rules/simplicity.md`) |
| R-3 | A new SPD config surface duplicates the `traffic-selector` list a peer already carries | two config paths describe one selector | one declaration, and the peer selector list derives from it or names it. `ai/rules/principles.md` forbids the second copy |
| R-4 | An operator order leaf collides with `PriorityIKEBypass`, and a Child SA policy captures IKE | an interop scenario stops rekeying | the IKE bypass keeps a reserved band no operator order can reach, and a config validator refuses an order inside it |
| R-5 | A phase lands the code and leaves the ledger row at `Partial`, so the public page stays wrong in the other direction | phase 9 never runs | phase 9 is the closing phase and AC-12 is a Goal Gate. The row moves when the sign-off lands, not before |
| R-6 | The ICMP block is large enough to exceed one agent, and gets trimmed to fit | an agent reports partial coverage of Section 6 | the package boundary is the BLOCK. An agent whose block is too big reports the size to the main thread, which re-cuts by section (`ai/rules/planning.md`) |
| R-7 | A new selector field is programmed but never negotiated, so Ze installs a policy the peer never proposed | an interop scenario shows a selector mismatch | RFC 7296 Section 2.9 narrowing is the existing rule: a selector Ze programs is a selector Ze negotiated, or the install is refused |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Live IPsec sessions. A wrong SPD action black-holes traffic an operator asked to pass, and a wrong bypass passes traffic an operator asked to protect. A wrong DF or PMTU setting silently drops large packets. A wrong PAD match authorizes a peer the operator did not name |
| How is it reverted? | Per phase, by one commit. The enum member and the new selector fields are additive and their zero values are the previous behavior, so a revert of a later phase does not disturb an earlier one. The ledger row in phase 9 is one line |
| Who else touches this path? | `plan/spec-ipsec-lifetime-volume.md` owns `lifetimeState`. `plan/spec-rfcgate-6-supported-extraction-signoff.md` owns `rfc/extraction/rfc4301.json` and the `supportClaimingScope` list in `internal/le/rfc/check_test.go`. `plan/spec-ipsec-0-umbrella.md` and the other `plan/spec-ipsec-*` specs work the same engine |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `set vpn ipsec policy <name> action discard` committed | → | the SPD config to `dataplane.SPParams` path, carrying `SPActionDiscard` | `TestDiscardPolicyReachesTheBackend` |
| A forged non-initial fragment matching a port-scoped BYPASS policy | → | the fragment check on the BYPASS path | `TestNonInitialFragmentIsNotBypassed` |
| `set vpn ipsec icmp unauthenticated <type> reject` committed | → | the ICMP acceptance control | `TestUnauthenticatedICMPTypeRejected` |
| A protected transit ICMP error whose payload header contradicts the SA selectors | → | the transit ICMP consistency check | `TestTransitICMPPayloadHeaderChecked` |
| `set vpn ipsec policy <name> selector icmp-type <n> icmp-code <n>` committed | → | the ICMP selector fields on `SPParams` | `TestICMPSelectorReachesTheBackend` |
| A peer asserting `host.example.com` against `remote-id .example.com` | → | `remoteIDMatches` sub-tree arm | `TestRemoteIDSubTreeMatch` |
| `set vpn ipsec policy <name> order <n>` committed | → | `SPParams.Priority` derived from the operator order | `TestPolicyOrderReachesTheBackend` |
| `set vpn ipsec site-to-site peer <name> df-bit copy` committed | → | the DF treatment on the installed SA | `TestDFBitTreatmentReachesTheBackend` |
| `set vpn ipsec site-to-site peer <name> dscp-map <value>` committed | → | the outer-header DSCP mapping | `TestOuterHeaderDSCPReachesTheBackend` |
| `./le rfc check` over the closing tree | → | `checkSupportedSignoff` and `checkStatusAgreement` (`internal/le/rfc/check_status.go`) | `TestSupportedRowsHaveDerivableScope` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An operator writes an SPD entry whose action is discard | matching traffic is dropped by the installed policy, and a backend that cannot express the action refuses the install with an error naming the backend and the action |
| AC-2 | A forged non-initial fragment arrives whose addresses and protocol match a BYPASS entry that names a non-trivial port range | the fragment is not passed in the clear, and the event is counted and logged |
| AC-3 | An administrator configures unauthenticated ICMP error messages of a given ICMP type to be rejected | an ICMP error of that type arriving on the unprotected side is dropped, and one of a permitted type is processed. The same control applies to the protected side |
| AC-4 | A protected transit ICMP error message arrives whose payload header, with source and destination addresses and ports reversed, does not match the traffic selectors of the SA it arrived on | the message is dropped when the check is configured, and forwarded when an SPD entry explicitly allows carriage of such traffic |
| AC-5 | An SPD entry names an ICMP type and code, or a port range | the selector is programmed exactly, and an entry the backend cannot program exactly is refused rather than widened |
| AC-6 | A peer asserts an identity that falls under a configured sub-tree, and another peer asserts one that does not | the first authenticates and the second is refused. The same holds for a DN sub-tree, a partially qualified RFC 822 address, and an address inside and outside a configured range |
| AC-7 | An administrator gives two overlapping SPD entries an explicit order | the first-ordered entry decides the disposition, the order survives a restart, and an order that would outrank the IKE bypass is refused at commit |
| AC-8 | An SA is configured with each of set, clear and copy for the DF bit | the outer header of an emitted tunnel-mode packet carries the configured DF value, observed on the wire |
| AC-9 | A per-SA PMTU is learned and the aging period elapses | the PMTU returns to the first-hop data-link MTU and is relearned, and the period is configurable |
| AC-10 | A packet enters a domain for which the inner DSCP value is not appropriate and a mapping is configured | the outer tunnel header carries the mapped value, observed on the wire |
| AC-11 | `rfc/short/rfc4301.md` is read at closure | its prose describes the native IKEv2 engine rather than strongSwan and charon; the seven `{not-applicable}` annotations and the `{gap}` carry a fresh answer recorded from the owner; the `{single-polarity}` reason on `RFC4301-4.1-3` no longer says the child-SA path hardcodes tunnel mode |
| AC-12 | `docs/features/rfc-status.md` and `./le rfc check` are read at closure | the RFC 4301 row reads `Supported` with a Remaining cell that claims no gap, `rfc4301` is back in `supportClaimingScope` with its pinning assertion removed, `rfc/extraction/rfc4301.json` is a sign-off the check accepts, and `./le rfc check` names `rfc4301` in no violation |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An operator writes an SPD entry that discards traffic to a subnet | config to ipsec types to `SPParams` with `SPActionDiscard` to the backend to the kernel SPD | `TestDiscardPolicyReachesTheBackend` plus the `spd-discard-entry` interop scenario |
| 2 | An attacker forges a non-initial fragment aimed at a bypassed port range | interface to the fragment check to a drop | `TestNonInitialFragmentIsNotBypassed` plus the `bypass-fragment-forgery` interop scenario |
| 3 | An operator rejects unauthenticated ICMP Destination Unreachable | config to the ICMP acceptance control to a drop | `TestUnauthenticatedICMPTypeRejected` plus a `.ci` over the operator path |
| 4 | An operator authorizes every peer under one DNS sub-tree with one PAD entry | config to `remoteIDMatches` to the authentication verdict | `TestRemoteIDSubTreeMatch` plus the `pad-subtree-identity` interop scenario |
| 5 | An operator orders two overlapping SPD entries and reads them back | config to `SPParams.Priority` to the kernel, and back through the inspection path | `TestPolicyOrderReachesTheBackend` plus a `.ci` reading the order back |
| 6 | An operator pins the DF bit of a tunnel to clear so a legacy path stops dropping large packets | config to the installed SA to the emitted outer header | `TestDFBitTreatmentReachesTheBackend` plus the `tunnel-df-treatment` interop scenario |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDiscardPolicyReachesTheBackend` | `internal/component/ike/engine/bypass_test.go` | the discard action survives the config to `SPParams` path | |
| `TestBackendRefusesUnexpressibleAction` | `internal/component/ike/dataplane/vpp_policy_test.go` | a backend that cannot express DISCARD refuses rather than downgrading | |
| `TestNonInitialFragmentIsNotBypassed` | `internal/component/ike/engine/bypass_test.go` | a non-initial fragment matching a port-scoped BYPASS entry is not passed | |
| `TestUnauthenticatedICMPTypeRejected` | `internal/component/ike/engine/icmp_test.go` | the per-type acceptance control decides an unauthenticated ICMP error | |
| `TestTransitICMPPayloadHeaderChecked` | `internal/component/ike/engine/icmp_test.go` | the payload header with addresses and ports reversed is matched against the SA selectors | |
| `TestICMPSelectorReachesTheBackend` | `internal/component/ike/dataplane/dataplane_test.go` | an ICMP type and code selector is programmed exactly | |
| `TestPortRangeSelectorRefusedWhenNotExpressible` | `internal/component/ike/dataplane/dataplane_test.go` | a range the backend cannot program is refused, never widened |  |
| `TestRemoteIDSubTreeMatch` | `internal/component/ike/engine/remote_id_test.go` | a partial DNS name, a partial RFC 822 address and a DN sub-tree each match under and only under their sub-tree | |
| `TestRemoteIDAddressRangeMatch` | `internal/component/ike/engine/remote_id_test.go` | an address identity inside a configured range matches and one outside it does not | |
| `TestPolicyOrderReachesTheBackend` | `internal/component/ike/engine/child_test.go` | an operator order becomes the policy priority and keeps the IKE bypass band reserved | |
| `TestPolicyOrderInsideBypassBandRefused` | `internal/component/ike/ipsec/validate_test.go` | an order that would outrank the IKE bypass is refused at commit | |
| `TestDFBitTreatmentReachesTheBackend` | `internal/component/ike/dataplane/dataplane_test.go` | each of set, clear and copy reaches the installed SA | |
| `TestPMTUAgingPeriodConfigurable` | `internal/component/ike/engine/pmtu_test.go` | the aging period is read from config and bounded | |
| `TestOuterHeaderDSCPReachesTheBackend` | `internal/component/ike/dataplane/dataplane_test.go` | the mapped DSCP value reaches the installed SA | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| SPD entry order | 1 to 65535 | 65535 | 0 | 65536 |
| ICMP type selector | 0 to 255 | 255 | N/A | 256 |
| ICMP code selector | 0 to 255 | 255 | N/A | 256 |
| Port range endpoints | 0 to 65535 | 65535 | N/A | 65536 |
| DSCP value | 0 to 63 | 63 | N/A | 64 |
| PMTU aging period, seconds | 60 to 86400 | 86400 | 59 | 86401 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-spd-discard-entry` | `test/ipsec/ipsec-spd-discard-entry.ci` | an operator writes a discard entry and sees matching traffic dropped | |
| `ipsec-bypass-fragment-check` | `test/ipsec/ipsec-bypass-fragment-check.ci` | a forged non-initial fragment aimed at a bypassed port range is dropped and counted | |
| `ipsec-icmp-unauthenticated-control` | `test/ipsec/ipsec-icmp-unauthenticated-control.ci` | an operator rejects one ICMP type and permits another, and reads the result back | |
| `ipsec-spd-order-readback` | `test/ipsec/ipsec-spd-order-readback.ci` | an operator orders two overlapping entries and reads the order back through the inspection path | |
| `ipsec-df-bit-treatment` | `test/ipsec/ipsec-df-bit-treatment.ci` | an operator sets each DF treatment and the emitted outer header carries it | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `spd-discard-entry` | `test/interop-ipsec/scenarios/` | strongSwan | a discard entry drops traffic strongSwan would otherwise carry, and the tunnel beside it stays up | |
| `bypass-fragment-forgery` | `test/interop-ipsec/scenarios/` | strongSwan | a forged non-initial fragment aimed at the bypassed IKE port range does not reach the protected side | |
| `icmp-error-payload-check` | `test/interop-ipsec/scenarios/` | strongSwan | a transit ICMP error whose payload header contradicts the SA selectors is dropped, and a consistent one is delivered | |
| `pad-subtree-identity` | `test/interop-ipsec/scenarios/` | strongSwan | a strongSwan peer whose ID_FQDN falls under a configured sub-tree authenticates, and one outside it is refused | |
| `tunnel-df-treatment` | `test/interop-ipsec/scenarios/` | strongSwan | the outer header DF bit carries each configured treatment, read on the wire | |

## Files to Modify
- `internal/component/ike/dataplane/dataplane.go` - `SPAction` gains DISCARD; `SPParams` gains ICMP type and code selectors, a port range form, a DF treatment and a DSCP mapping; the two priority constants become a reserved band plus an operator order
- `internal/component/ike/dataplane/xfrm_linux.go` - the Linux projection of every new field, and the refusal for a field it cannot express
- `internal/component/ike/dataplane/vpp_policy.go` - the VPP projection, and the refusal for a field it cannot express
- `internal/component/ike/engine/bypass.go` - the fragment check on the BYPASS path
- `internal/component/ike/engine/child.go` - `childPolicyParams` carries the new selector and priority fields
- `internal/component/ike/engine/remote_id.go` - `remoteIDMatches` gains sub-tree and range matching
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - the SPD entry list, the ICMP controls, the DF treatment, the DSCP mapping and the PMTU aging period
- `internal/component/ike/ipsec/validate.go` - the commit-time validators for the new leaves
- `internal/core/diagnostic/codes.go` - a code per new runtime dependency a doctor check reads
- `internal/component/ike/engine/doctor.go` - the doctor checks for the new dependencies
- `rfc/short/rfc4301.md` - the prose repair, the fresh answers on the seven `{not-applicable}` annotations and the `{gap}`, the corrected `{single-polarity}` reason on `RFC4301-4.1-3`, and one new requirement id per obligation this spec implements
- `docs/features/rfc-status.md` - the RFC 4301 row, raised to `Supported` in the closing phase
- `internal/le/rfc/check_test.go` - `rfc4301` returns to `supportClaimingScope` and its pinning assertion is removed
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - declared by the `// Design:` header of `dataplane.go`, `xfrm_linux.go`, `vpp_policy.go`, `bypass.go` and `child.go`. The SPD action set, the selector set and the policy ordering it describes all change
- `docs/architecture/ike/ipsec-dataplane-inspection.md` - the policy fields the inspection path reads back
- `docs/architecture/ike/ipsec-7-ikev2-engine.md` - declared by the `// Design:` header of `remote_id.go` and `doctor.go`. The PAD matching rules it describes change in phase 5, and it gains the new doctor checks in phase 3
- `docs/architecture/ike/ipsec-3-data-model.md` - declared by the `// Design:` header of `internal/component/ike/ipsec/validate.go`. The cross-reference validation it describes gains the SPD order, ICMP type and code, port range, DSCP and PMTU period rules
- `docs/features/ai-first.md` - declared by the `// Design:` header of `internal/core/diagnostic/codes.go`. Named here as affected only by the new diagnostic codes: the page describes how a code reaches an agent, and adding codes changes the list rather than the mechanism
- `docs/guide/ipsec.md` - the new operator config
- `docs/features.md` - the IPsec feature rows
- `docs/functional-tests.md` - the five new `.ci` names
- `docs/architecture/testing/interop.md` - the five new scenario names

## Files to Create
- `internal/component/ike/engine/icmp.go` - the ICMP acceptance controls and the transit consistency check
- `internal/component/ike/engine/pmtu.go` - the per-SA PMTU value and its aging
- `internal/component/ike/engine/fragment.go` - the fragment state for BYPASS and DISCARD entries with a non-trivial port range
- `test/ipsec/*.ci` - the five functional tests named above
- `test/interop-ipsec/scenarios/<name>/` - the five scenarios named above
- `plan/deferrals/rfc4301-architecture-gaps.md` - the shard named in the metadata table

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` for the SPD entry list, the ICMP controls, the DF treatment, the DSCP mapping and the PMTU aging period |
| YANG validation constraints | Yes | every new leaf takes `range`, `enumeration` or `pattern`: the order, the ICMP type and code, the port range endpoints, the DSCP value and the aging period all have the bounds in the Boundary Tests table |
| YANG custom validators | Yes | the order leaf needs a validator that refuses the reserved IKE bypass band, which no native constraint can express |
| CLI commands/flags | Yes | the config editor reaches the new leaves; `show vpn ipsec` gains the SPD order and action in its output |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md`: every new path is keyword before value, `policy <name> action discard` rather than `<name> discard` |
| Editor autocomplete | Yes | automatic for the enumerations; the policy name leaf needs a `CompleteFn` over the configured entries |
| Functional test for new RPC/API | Yes | the five `.ci` tests named above |
| Pipe completeness | Yes | the `show vpn ipsec` additions route through the existing pipe path |
| Env var registration | N-A | no new environment leaf |
| Doctor check for runtime dependencies | Yes | the fragment path and the ICMP path each read a kernel facility. Each gets an owning-package check plus a code in `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | Yes | a counter per dropped forged fragment, per rejected unauthenticated ICMP error, and per transit ICMP error refused by the payload check |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` and `docs/guide/ipsec.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` for the `show vpn ipsec` additions |
| 4 | API/RPC added/changed? | No | no new RPC handler beyond the config surface |
| 5 | Plugin added/changed? | No | the IKE component is not a plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` |
| 7 | Wire format changed? | No | no new IKEv2 payload or ESP framing. The selector fields travel in existing traffic selector payloads |
| 8 | Plugin SDK/protocol changed? | No | no SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4301.md` and the `docs/features/rfc-status.md` row |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` and `docs/architecture/testing/interop.md` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` compares IPsec features against strongSwan |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` and `docs/architecture/ike/ipsec-dataplane-inspection.md` |
| 13 | Route metadata keys added/changed? | No | no route metadata |
| 14 | Prometheus counters added/changed? | Yes | the subsystem telemetry doc for the three new counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | no registry entry changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/spec-rfc4301-architecture-gaps.md` at the start of each phase and name every doc it lists. `docs/features/rfc-status.md` already carries a `<!-- source: internal/component/ike/dataplane/dataplane.go -->` anchor over a file every phase edits |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ipsec.md` shows the `vpn ipsec` grammar; verify every example against the YANG after each phase |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register every new entry point and prove each wiring test red
   - Tests: the ten rows of the Wiring Test table
   - Files: `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`, `internal/component/ike/dataplane/dataplane.go`, the skeleton handlers
   - Verify: every config path commits, every new field reaches a stub that returns "not implemented", and every wiring test fails on the stub rather than on a missing path
2. **Phase: BYPASS and DISCARD fragments (Sections 7.4 and D.4)** -- the live attack, first
   - RFC: Section 7.4, "All implementations MUST support DISCARDing of fragments using the normal SPD packet classification mechanisms." and "All implementations MUST support stateful fragment checking to accommodate BYPASS traffic for which a non-trivial port range is specified." Appendix D.4, "implementations MUST support fragment reassembly for BYPASS/DISCARD traffic when port fields are specified" and "An implementation also MUST permit a user or administrator to accept such traffic or reject such traffic using the SPD conventions described in Section 4.4.1."
   - Tests: `TestDiscardPolicyReachesTheBackend`, `TestBackendRefusesUnexpressibleAction`, `TestNonInitialFragmentIsNotBypassed`, `ipsec-spd-discard-entry`, `ipsec-bypass-fragment-check`, the `spd-discard-entry` and `bypass-fragment-forgery` scenarios
   - Files: `dataplane.go`, `xfrm_linux.go`, `vpp_policy.go`, `bypass.go`, `fragment.go`
   - Verify: validate A-3 with the QEMU test BEFORE designing the fix. Run the whole `test/interop-ipsec` suite at the end (A-7)
3. **Phase: ICMP processing (Section 6)** -- the whole section, unimplemented and unextracted
   - RFC: Section 6, "Disposition of non-error, ICMP messages (that are not addressed to the IPsec implementation itself) MUST be explicitly accounted for using SPD entries." Section 6.1.1, "a compliant IPsec implementation MUST permit a local administrator to configure an IPsec implementation to accept or reject unauthenticated ICMP traffic. This control MUST be at the granularity of ICMP type and MAY be at the granularity of ICMP type and code." Section 6.1.2, "implementers MUST provide controls to allow local administrators to constrain the processing of ICMP error messages received on the protected side of the boundary, and directed to the IPsec implementation." Section 6.2, "an IPsec implementation MUST be configurable to check that this payload header information is consistent with the SA via which it arrives" and "IPsec senders and receivers MUST support the following processing for ICMP error messages that are sent and received via SAs."
   - Tests: `TestUnauthenticatedICMPTypeRejected`, `TestTransitICMPPayloadHeaderChecked`, `ipsec-icmp-unauthenticated-control`, the `icmp-error-payload-check` scenario
   - Files: `icmp.go`, `ze-ipsec-conf.yang`, `validate.go`, the doctor check and its diagnostic code
   - Verify: the ICMP type granularity is a MUST and the type-and-code granularity is a MAY, so the MAY goes to the owner as a question (`ai/rules/rfc-compliance.md`)
4. **Phase: SPD selectors (Section 4.4.1.1)** -- clears the one gated requirement
   - RFC: Section 4.4.1.1, "The following selector parameters MUST be supported by all IPsec implementations to facilitate control of SA granularity."
   - Tests: `TestICMPSelectorReachesTheBackend`, `TestPortRangeSelectorRefusedWhenNotExpressible`
   - Files: `dataplane.go`, `xfrm_linux.go`, `vpp_policy.go`, `child.go`, `ze-ipsec-conf.yang`
   - Verify: this phase clears `RFC4301-4.4.1.1-1`. Its `{gap}` annotation is removed only when the selectors are programmed AND a tagged test proves it, and the ledger's gap disclosure is corrected in the same commit
5. **Phase: PAD matching (Section 4.4.3.1)** -- the authorization surface
   - RFC: Section 4.4.3.1, "But, at a minimum, sub-tree matching of the sort described above MUST be supported." and "For IPv4 and IPv6 addresses, the same address range syntax used for SPD entries MUST be supported."
   - Tests: `TestRemoteIDSubTreeMatch`, `TestRemoteIDAddressRangeMatch`, the `pad-subtree-identity` scenario
   - Files: `remote_id.go`, `ze-ipsec-conf.yang`, `validate.go`
   - Verify: the class gate in `remoteIDMatches` stays. A sub-tree pattern that would also match an exact identity of another class is refused at commit, not resolved at runtime
6. **Phase: SPD entry ordering (Section 4.4.1)** -- depends on the SPD entry list phase 2 introduces
   - RFC: Section 4.4.1, "The management interface for the SPD MUST allow creation of entries consistent with the selectors defined in Section 4.4.1.1, and MUST support (total) ordering of these entries, as seen via this interface." and "the system administrator MUST be able to specify whether or not a user or application can override (default) system policies."
   - Tests: `TestPolicyOrderReachesTheBackend`, `TestPolicyOrderInsideBypassBandRefused`, `ipsec-spd-order-readback`
   - Files: `dataplane.go`, `child.go`, `ze-ipsec-conf.yang`, `validate.go`
   - Verify: R-4. The IKE bypass keeps a reserved band, and an interop scenario that rekeys proves the bypass still outranks every Child SA policy
7. **Phase: DF bit and PMTU (Section 8)**
   - RFC: Section 8.1, "All IPsec implementations MUST support the option of copying the DF bit from an outbound packet to the tunnel mode header that it emits, when traffic is carried via a tunnel mode SA. This means that it MUST be possible to configure the implementation's treatment of the DF bit (set, clear, copy from inner header) for each SA." Section 8.2.2, "In all IPsec implementations, the PMTU associated with an SA MUST be 'aged' and some mechanism is required to update the PMTU in a timely manner".
   - Tests: `TestDFBitTreatmentReachesTheBackend`, `TestPMTUAgingPeriodConfigurable`, `ipsec-df-bit-treatment`, the `tunnel-df-treatment` scenario
   - Files: `dataplane.go`, `xfrm_linux.go`, `pmtu.go`, `ze-ipsec-conf.yang`
   - Verify: validate A-4 by reading the producing kernel path and observing a PMTU return to the first-hop MTU in QEMU. Kernel behavior is evidence to put to the owner, never a classification this phase makes (R-1)
8. **Phase: Outer-header DSCP (Section 5.1.2.1)**
   - RFC: Section 5.1.2.1, note 5, "If the packet will immediately enter a domain for which the DSCP value in the outer header is not appropriate, that value MUST be mapped to an appropriate value for the domain".
   - Tests: `TestOuterHeaderDSCPReachesTheBackend`
   - Files: `dataplane.go`, `xfrm_linux.go`, `ze-ipsec-conf.yang`
   - Verify: the mapping is configurable per SA, and the emitted outer header carries the mapped value on the wire rather than only in the installed state
9. **Phase: summary, annotations and the public row** -- the closing phase
   - Tests: `TestSupportedRowsHaveDerivableScope`, `./le rfc check`
   - Files: `rfc/short/rfc4301.md`, `docs/features/rfc-status.md`, `internal/le/rfc/check_test.go`, `rfc/extraction/rfc4301.json`
   - Verify: every void annotation carries a fresh answer, every new obligation carries a new requirement id and a tagged test, the row reads `Supported`, and `./le rfc check` names `rfc4301` in no violation. The sign-off artifact lands under the rules of `plan/spec-rfcgate-6-supported-extraction-signoff.md`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every block in the Task has a phase, and every phase has a landed test naming its RFC section |
| Feature completeness | Every new config leaf is reachable from the editor, appears in `show vpn ipsec`, and is proven by a `.ci`, not only by a unit test |
| Correctness | `SPActionProtect` is still the zero value of `SPAction`, so a caller that forgets the field still gets a protecting policy |
| Correctness | Every backend either programs a new selector field exactly or refuses the install. No field is widened, rounded or silently dropped |
| Correctness | The fragment check covers DISCARD entries as well as BYPASS entries. Section 7.4 names both |
| Naming | Every new field says what the value is rather than its Go type, and every new YANG leaf matches the operator vocabulary already in `docs/guide/ipsec.md` |
| Data flow | An SPD entry is declared once. The peer `traffic-selector` list and the new SPD entry list do not both declare the same selector (R-3) |
| Rule: `ai/rules/rfc-compliance.md` | No `{gap}`, `{not-applicable}` or `partial` is written or kept for an unmet obligation without a recorded answer from the owner. Every MUST implemented here carries a comment above the enforcing code quoting its RFC section |
| Rule: `ai/rules/interop-and-goal-validation.md` | Each new scenario was proven RED by reverting the change and rebuilding the artifact, and the RED result is recorded |
| Rule: `ai/rules/evidence.md` | Every claim about what the kernel does names the producing path and is proven by a QEMU test, never inferred from the caller |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| DISCARD is expressible | `grep -n SPActionDiscard internal/component/ike/dataplane/dataplane.go` returns the constant and every backend returns a call site |
| Fragments are checked | `test/ipsec/ipsec-bypass-fragment-check.ci` passes and the `bypass-fragment-forgery` scenario passes |
| ICMP controls exist | `./le config schema` shows the ICMP leaves and `ipsec-icmp-unauthenticated-control.ci` passes |
| Selectors are complete | `RFC4301-4.4.1.1-1` carries no `{gap}` annotation and `rfc/requirements/rfc4301.md` lists a test for it |
| PAD sub-tree matching exists | the `pad-subtree-identity` scenario passes against strongSwan |
| SPD ordering exists | `ipsec-spd-order-readback.ci` passes |
| DF and PMTU are configurable | the `tunnel-df-treatment` scenario passes and `TestPMTUAgingPeriodConfigurable` passes |
| DSCP is mapped | `TestOuterHeaderDSCPReachesTheBackend` passes |
| The summary is repaired | `grep -in strongswan rfc/short/rfc4301.md` returns nothing that describes Ze's own negotiation |
| The public row is raised | the RFC 4301 row reads `Supported` and `./le rfc check` names `rfc4301` in no violation |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Guard fails open | A fragment the check cannot classify must be dropped, never passed. A selector a backend cannot program must refuse the install, never widen it. A PAD sub-tree that fails to parse must refuse the peer, never match everything |
| Input validation | Every field this spec reads off the wire or out of a fragment header is attacker-controlled. The fragment path parses a forged header by definition, so it takes the boundary tests and a fuzz target |
| Resource exhaustion | Fragment state is attacker-driven. It carries a bounded entry count and a bounded lifetime, and the bound is stated in the code (`docs/contributing/ze-go-style.md`, "A limit on everything") |
| Downgrade | A peer must not be able to negotiate a selector wider than the configured SPD entry. Narrowing goes inward only |
| Authorization | The PAD sub-tree match widens who can authenticate. A pattern that matches more than the operator intended is the failure mode, so every sub-tree test carries a negative case |
| Error leakage | A refusal message names the offending value and the configured expectation. It does not print a key, a certificate private field or an operator credential |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A backend cannot express a field | The install is refused and the refusal is tested. Widening the selector is banned |
| The kernel appears to discharge an obligation | Evidence to put to the owner with the producing path and a QEMU observation. Never a `{not-applicable}` this spec writes (R-1) |
| A phase is too large for one agent | R-6: report the size to the main thread and let it re-cut by section. Do not trim the block |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The walk that found these blocks read the RFC's own text rather than the summary. The summary declared 18 obligations against a source carrying 98 uppercase MUST-level keyword occurrences, and every block above sat outside the 18. An unbounded checklist is green for what nobody wrote down.
- `rfc/short/rfc4301.md` contradicts itself today. Its prose says Ze delegates IKEv2 negotiation to strongSwan and charon, and its own annotations cite `internal/component/ike/engine`, the native engine. A page that disagrees with the code is a defect the work that meets it repairs (`ai/rules/documentation.md`).
- Two annotations on that page are false at the producer as of 2026-08-30, and both were read rather than inferred. The `{gap}` on `RFC4301-4.4.1.1-1` says `SPParams` has no port or protocol field; `SrcPort`, `DstPort` and `UpperProto` all exist, and only the ICMP type and code are absent. The `{single-polarity}` reason on `RFC4301-4.1-3` says the child-SA path hardcodes tunnel mode so a peer SA can never be transport; `createFirstChildSA` sets transport mode on `sa.UseTransportMode`.
- Seven annotations are void under the owner directive of 2026-07-27 and must be re-answered rather than cited: the `{gap}` on `RFC4301-4.4.1.1-1` and the `{not-applicable}` on `RFC4301-4.1-5`, `RFC4301-4.1-6`, `RFC4301-4.4.1-2`, `RFC4301-5.2-1`, `RFC4301-5.2-2` and `RFC4301-7-1`. Six of the seven rest on "the kernel does the per-packet half", which is an observation about where the work happens and not an answer about whether Ze meets the obligation.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Correct the public row first and implement second | Implement inside `plan/spec-rfcgate-6-supported-extraction-signoff.md`; leave the row at `Supported` | Owner decision, 2026-08-30. The row is a public promise that is false today, and correcting it costs one line. Leaving it was not on the table, and folding eight blocks of protocol work into a bookkeeping spec would have made that spec unclosable |
| One phase per block | One phase for the whole RFC; one phase per requirement id | A block shares one subject matter and one surface, which is the unit an agent can hold. A requirement id is too small to justify a phase report and the whole RFC is too large for one context (`ai/rules/planning.md`) |
| Fragments before ICMP | Follow the RFC's own section order | The fragment gap is reachable on a running daemon by a forged packet, and the ICMP gap is a missing control. Blast radius orders the phases, not the document |
| The byte-count SAD lifetime stays in its own spec | Implement Section 4.4.2.1 here | `plan/spec-ipsec-lifetime-volume.md` already designs it. A second design of one fact is a future disagreement with nothing to arbitrate it (`ai/rules/principles.md`) |
| A new operator-authored SPD entry list | Extend the peer `traffic-selector` list with an action and an order | A DISCARD entry and a BYPASS entry are not attached to a peer, and Section 4.4.1 asks for a management interface over the SPD itself. Overloading the peer selector list would put two meanings on one leaf |
| The IKE bypass keeps a reserved priority band | Let an operator order every entry freely | A Child SA policy that outranks the IKE bypass prevents its own rekey and teardown. The band is refused at commit, where the operator can read the reason |

## Known Limitations

- **It does not implement the Section 4.4.2.1 byte-count SAD lifetime.** `plan/spec-ipsec-lifetime-volume.md` designs it and this spec depends on it. Closure of this spec needs that work landed, because the ledger row cannot read `Supported` while a MUST of Section 4.4.2.1 is unmet.
- **It does not write `rfc/extraction/rfc4301.json`.** That artifact belongs to `plan/spec-rfcgate-6-supported-extraction-signoff.md`. Phase 9 lands it under that spec's rules and its checks.
- **It does not answer the Section 6.1.1 MAY.** ICMP type-and-code granularity is a MAY, and `ai/rules/rfc-compliance.md` reserves a MAY to the owner. Phase 3 puts it to him with the cost of each option.
- **It does not widen the class gate in `remoteIDMatches`.** Sub-tree matching is added inside each existing class. An asserted ID_FQDN still cannot satisfy an address-valued `remote-id`.

## RFC Documentation (Scope: protocol)

Every MUST this spec implements carries a comment directly above the enforcing code, in
the form `// RFC 4301 Section X.Y: "<quoted requirement>"`, with the sentence quoted from
`rfc/full/rfc4301.txt` and not from `rfc/short/rfc4301.md`. The fragment path, the ICMP
path and the DF path each carry an ASCII diagram of the header fields they read, with byte
offsets, because each one parses a header an attacker controls.

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
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
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
