# Spec: OSPF, IS-IS, BFD and VRRP one-off defects from the journal

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Each row is incorrect shipped IGP, BFD or VRRP behaviour: an RFC 8362 extended-LSA read, an ISO 10589 hello timing, a BFD state or session, a VRRP owner decision |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The 2026-10-23 journal sweep left about 265 open rows that fit none of the
thirteen root-cause specs. This spec owns the 10 in OSPF (mostly OSPFv3 RFC 8362
extended LSAs), IS-IS, BFD and VRRP. Each row is a claim until a red test
reproduces it at HEAD.

Existing specs checked first: `spec-vrrp-owner-arp-rfc-defects` already owns the
`ownerfilter.go` and vpp `verifyMatch` rows (`guard-added-to-one-half-of-a-pair`,
2026-09-27), so they are not repeated here.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` and the OSPF, IS-IS, BFD and VRRP doors in `ai/doors/README.md`
- [ ] `docs/guide/ospf.md`

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc8362.txt` Sections 3.4, 3.6, 3.7, 4.2, 4.3, 4.5
- [ ] `rfc/full/rfc8665.txt` Section 5, `rfc/full/rfc8666.txt` Section 6
- [ ] `rfc/full/rfc5880.txt` Section 6.8.6
- [ ] `rfc/full/rfc9568.txt` Section 1.7
- [ ] ISO/IEC 10589 Section 10.4 (not in `rfc/full/`; quote from the standard at design)

## Current Behavior (MANDATORY)

Spot-checked on 2026-10-23: `internal/plugins/vrrp/groups.go` still sets
`spec.IsOwner = ownsAny(spec.VIPs, real)`, so the 2026-10-14 row is present.

- [ ] `internal/plugins/ospf/default.go` - `applyDefaultInformation`
- [ ] `internal/plugins/ospf/sr_install.go` - `srAttachedAdvertisers`
- [ ] `internal/plugins/ospf/bgpls_export.go` - `bgplsArea.extendedV3`
- [ ] `internal/plugins/ospf/state.go` - `incrementBootCount`
- [ ] `internal/plugins/ospf/` - `v6PrefixSIDsFromTLV`, `engine.v6ReceivedPrefixSIDs`
- [ ] `internal/plugins/isis/circuit/ish.go` - `receiveISH`
- [ ] `internal/component/bfd/api/events.go` - `StateChange.State`
- [ ] `internal/component/bfd/engine/snapshot.go` - session profile
- [ ] `internal/component/bfd/bfd.go` - `EnsureSession`, `bindV6`
- [ ] `internal/plugins/vrrp/groups.go` - `IsOwner`, `ownedVIPs`

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
An OSPFv3 E-LSA in the LSDB; an IS-IS ISH on a P2P circuit; a BFD Control
packet; VRRP group config with VIPs.

### Transformation Path
1. LSA parse, TLV walk, SPF and SR install; BGP-LS export.
2. IS-IS adjacency formation on hello.
3. BFD session state machine and its subscribers (BGP, OSPF).
4. VRRP group spec build and owner decision.

### Boundaries Crossed
Wire to plugin engine; engine to FIB and BGP-LS; BFD to subscribing protocols.

### Integration Points
BFD `api.Service`, BGP-LS export, the durable store (boot count).

### Architectural Verification
- [ ] Every TLV reader receives the LS type it is applicable to

## Risks & Assumptions

### Assumptions
- `spec-ospf-ext-0-umbrella` owns absent RFC 8362 features; this spec only fixes reads Ze already does.

### Risks
- Splitting BFD AdminDown from path failure changes subscriber behaviour; BGP and OSPF consumers need review.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| OSPFv3 Router-LSA with two Attached-Router TLVs | → | `extendedV3` | unit asserting the second is ignored |
| ISH received on a P2P circuit | → | `receiveISH` | unit asserting an IIH is sent at once |

## Acceptance Criteria

| AC | Rows | Observable |
|----|------|------------|
| AC-1 | BFD `StateDown` | Subscribers can tell a neighbour AdminDown from a path failure (RFC 5880 Section 6.8.6, quote owed at design); BGP does not tear down on neighbour AdminDown where RFC 5882 says so |
| AC-2 | `show bfd sessions` profile | The displayed profile is the one whose auth the session runs after a shared revive |
| AC-3 | VRRP `IsOwner` | Owner status is decided per VIP; a group with one real and one non-real VIP does not force priority 255 for both (RFC 9568 Section 1.7, quote owed at design) |
| AC-4 | OSPFv3 `default-information originate` | The OSPFv3 engine originates an E-AS-External LSA in OSPFv3 format; a unit asserts LS type and body |
| AC-5 | `srAttachedAdvertisers` OSPFv3 | Mapping Server SID PHP reads the LA-bit for OSPFv3. RFC 8666 Section 6: "The Prefix is inter-area type and the downstream neighbor is an ABR, which is advertising prefix reachability and is setting the LA-bit in the Prefix Options as described in [RFC8362]." |
| AC-6 | `receiveISH` | An IIH is sent upon receipt of an ISH, not at the next hello tick (ISO/IEC 10589 Section 10.4, quoted in the row) |
| AC-7 | `extendedV3` repeated TLVs | RFC 8362 Section 4.2: "Instances of the Attached-Router TLV subsequent to the first MUST be ignored." Sections 4.3 and 4.5 likewise for their TLVs |
| AC-8 | `v6PrefixSIDsFromTLV` | RFC 8362 Sections 3.4, 3.6, 3.7: "Inclusion in other Extended LSAs MUST be ignored." A Prefix-SID in a TLV carried by the wrong LS type is ignored |
| AC-9 | BFD IPv6 without `bind-v6` | `EnsureSession` refuses an IPv6 peer when no IPv6 socket is bound, with a message naming `bfd { bind-v6 }`; `ospfv3-bfd-frr` sees the refusal or a working session |
| AC-10 | `incrementBootCount` | A store lock held past the deadline is retried, not fatal to the engine; a unit holds the lock across the first deadline |
| AC-11 | VRRP x4 (owner ARP filter requests, shared-sysctl restore retry, unbound selector in `ze doctor`, negative tags on `TestDataplaneRestoreOnLastGroup`) | The owner's parent sends no ARP request with the owned address as sender; a failed restore keeps the original values; a group whose selector names no device shows as not running in `ze doctor` and metrics; the six negative tags are withdrawn or backed |
| AC-12 | OSPF x2 (NSSA translator set, OSPFv3 AH kernel) | The RFC 3101 Section 3.2 translator set is read from translator-equivalent LSAs; the runtime kernel carries the AH transform and an OSPFv3 AH adjacency forms on it |
| AC-13 | RSVP-TE state lifetime | RFC 2205 Section 3.7: the PSB/RSB lifetime is at least (K + 0.5)*1.5*R; a test refreshing at 1.5R keeps the state |
| AC-14 | OSPF AuType 2 sequence | The cryptographic sequence never regresses across a restart after any number of packets; a test restarts after N > 2 sends |
| AC-15 | OSPFv3 extended LSAs x2 | RFC 8362 Sections 4.8 and 4.3: E-Intra-Area-Prefix references 0xA021 or 0xA022 and each E-Inter-Area-Prefix-LSA carries one Inter-Area-Prefix TLV (hex tests) |
| AC-16 | OSPF reload race | `TestOSPFReloadCreatedInstanceInitializesState` and its AF sibling pass under `-race -count=20` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| one red-first unit per AC | package of the producer | AC-1 to AC-10 |

### Interop Tests (Scope: protocol)

| Scenario | AC |
|----------|----|
| `ospfv3-bfd-frr` | AC-9 |

## Files to Modify
- The producer files under Current Behavior.

### Integration Checklist
- [ ] Each fix reached through a received packet or config

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/guide/ospf.md` (AC-5 removes the stated SHOULD gap)

## Implementation Steps
1. Read each producer at HEAD; move a fixed row to "Rows found already fixed".
2. Red test, fix, green, page edit, journal cell closed with the commit.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `session-never-establishes-with-the-peer` | 2026-10-27 | bgp-bfd-shared-session-widens |
| `field-carries-two-meanings` | 2026-09-08 | `api.StateChange.State` |
| `field-carries-two-meanings` | 2026-10-11 | `show bfd sessions` profile |
| `field-carries-two-meanings` | 2026-10-14 | `GroupSpec.IsOwner` |
| `guard-added-to-one-half-of-a-pair` | 2026-08-29 | OSPFv3 `default-information originate` |
| `guard-added-to-one-half-of-a-pair` | 2026-10-02 | `srAttachedAdvertisers` |
| `queued-request-waits-for-an-unrelated-trigger` | 2026-09-30 | IS-IS `receiveISH` |
| `repeated-instance-read-past-the-first` | 2026-10-20 | `bgplsArea.extendedV3` |
| `requirement-applied-outside-the-case-it-names` | 2026-10-20 | `v6PrefixSIDsFromTLV` |
| `silent-fall-through` | 2026-09-30 | BFD `EnsureSession` IPv6 |
| `transient-failure-treated-as-fatal` | 2026-09-30 | `engine.incrementBootCount` |
| `guard-added-to-one-half-of-a-pair` | 2026-09-27 | vrrp `ownerfilter.go` ARP request |
| `rollback-forgets-partial-apply` | 2026-09-23 | vrrp shared-sysctl restore |
| `unwired-feature` | 2026-08-23 | vrrp unbound selector, `ze doctor`, metrics |
| `reference-checked-claim-unchecked` | 2026-10-13 | VRRP negatives on `TestDataplaneRestoreOnLastGroup` |
| `key-omits-a-fact-the-builder-uses` | 2026-09-27 | ospf NSSA translator set |
| `unwired-feature` | 2026-09-14 | runtime kernel without AH for OSPFv3 |
| `bound-too-small-for-its-own-burst` | 2026-08-29 | RSVP-TE `expiredPSBs` lifetime K*R below RFC 2205 floor |
| `claim-outlives-the-evidence-it-cites` | 2026-09-30 | OSPF `authStore.signKey` AuType 2 sequence regresses across restart |
| `declared-format-contradicts-payload` | 2026-10-20 | `v6EIntraAreaPrefixBody` references LS Type 0x2001 |
| `declared-format-contradicts-payload` | 2026-10-20 | `v6EInterAreaPrefixBody` has no Inter-Area-Prefix TLV |
| `gate-red-where-nothing-blocks-on-it` | 2026-09-17 | OSPF reload-created instance tests race in `reconcile` |

## Rows found already fixed

| Class file | Date | Surface | Evidence |
|------------|------|---------|----------|
| none in this group at the 2026-10-23 sweep | - | - | - |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (red recorded per AC)
- [ ] Tests PASS
- [ ] Scoped runs: `./le job run go test ./internal/plugins/ospf/... ./internal/plugins/isis/... ./internal/component/bfd/... ./internal/plugins/vrrp/...`, `./le test ospf`
