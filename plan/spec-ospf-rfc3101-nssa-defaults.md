# Spec: ospf-rfc3101-nssa-defaults

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | learned 972 (OSPF AF seam), learned 975 (OSPFv3 NSSA redistribution) |
| Phase | RESEARCH and DESIGN complete. Implementation runs on Opus 4.8 |
| Deferral shard | `-` |
| Updated | 2026-08-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 3101 requires an NSSA border router to originate a default destination into
every directly attached NSSA, and requires an NSSA border router to reject a
received Type-7 default when its P-bit is clear or when the router suppresses
Type-3 summary import. Ze implements neither obligation for OSPFv3, and until
2026-08-02 implemented neither for OSPFv2.

An uncommitted change in the working tree implements the OSPFv2 half. It is
proven: OSPF unit tests pass, `make ze-lint-changed` reports 0 issues, and
`make ze-interop-test INTEROP_SCENARIO=ospf-stub-nssa-frr` passes against FRR
and discriminates against HEAD. It also removed two `{gap}` annotations from
`rfc/short/rfc3101.md` and rewrote the `docs/features/rfc-status.md` RFC 3101
row to claim both requirements are implemented and tested.

Those claims are false for OSPFv3, and the same change made the OSPFv3 defect
worse rather than merely leaving it in place:

- `applyNSSADefaults` has no address-family dispatch, so an OSPFv3 NSSA border
  router originates an OSPFv2-format Type-7 LSA into the shared LSDB. The
  branch existed before but fired only under `no-summary` or
  `default-originate`; it now fires on every dual-area OSPFv3 NSSA ABR.
- `v6ApplyAreaTypePolicy` injects the `::/0` inter-area default for stub areas
  only, so an OSPFv3 no-summary NSSA now receives no default at all.

The goal is full RFC 3101 conformance for the NSSA default route across BOTH
address families, with tagged tests in both polarities and non-unit evidence
for the wire-visible behaviour, so the `{gap}` deletions and the public
conformance row become true as written.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - OSPF engine placement and the address-family seam
  → Decision: ONE `ospf` engine carries both families through Transport, Codec and
    AFPrefixStrategy seams. Never create a second OSPFv3 engine.
  → Constraint: every family-sensitive engine method dispatches explicitly; there is no
    implicit family routing to fall back on.
- [ ] `docs/architecture/wire/ospfv3.md` - OSPFv3 LSA scope and prefix encoding
  → Constraint: the document carries NO NSSA material at all. The v3 NSSA default route is a
    documentation gap this spec must fill, not merely a code gap.
- [ ] `docs/architecture/testing/ci-format.md` - `.ci` directive vocabulary
  → Constraint: `expect=output:json=<n>.<field>=<value>` reads JSON command output; an
    `expect=output:absent=` assertion is vacuous unless a preceding step makes the needle
    present, so every negative `.ci` assertion needs a positive control step before it.

### Prior Art (learned summaries)
- [ ] `docs/architecture/ospf/ospf-af-unify.md` - the address-family seam decision
  → Constraint: scope-typed OSPFv3 LS Types MUST be classified through the helpers
    (`types.LSType.NSSA()`, `.ASExternal()`, `.ASWide()`), never through OSPFv2 numeric
    constants. A past defect put OSPFv3 AS-External LSAs in per-area storage because code
    compared `key.Type == 5`.
- [ ] `docs/architecture/ospf/ospfv3-5-nssa-redist.md` - OSPFv3 Type-7 origination
  → Decision: reuse the v4 NSSA policy (translator election, P-bit boundary rule, source
    preference) and vary ONLY the wire encode. This spec follows the same split.
  → Constraint: the OSPFv3 P-bit rides in the prefix's `PrefixOptions` (`OptPrefixP`), NOT
    in the LSA header Options as OSPFv2 does.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc3101.md` - NSSA option, Type-7 origination and installation
  → Constraint: RFC3101-2.4-5 (MUST) requires a default-destination LSA into EVERY directly
    attached NSSA, with no operator gate.
  → Constraint: RFC3101-2.4-4 (MUST) requires the P-bit clear on an ABR-originated Type-7
    default, and permits installing a received Type-7 default only when its P-bit is set.
  → Constraint: RFC3101-2.5-1 (MUST) requires an NSSA border router that suppresses Type-3
    summary import to ignore Type-7 defaults entirely.
  → Constraint: RFC3101-2.7-2 (SHOULD) makes the no-summary NSSA default a Type-3
    summary-LSA rather than a Type-7.
  → Constraint: Section 2.7 also carries an unextracted MUST NOT: when summary routes ARE
    imported, the border router's default LSA must not be a Type-3 summary-LSA. The code
    satisfies it; no checklist id exists for it.
- [ ] `rfc/short/rfc5340.md` - OSPFv3 LSA scope and prefix options
  → Constraint: the OSPFv3 NSSA-LSA is LS Type 0x2007 and its body is byte-identical to the
    AS-External body (Appendix A.4.8); only LS Type and flooding scope differ.

**Key insights:** (minimal context to resume after compaction)
- The OSPFv2 half of this work is ALREADY WRITTEN and uncommitted in the working tree, and
  is proven: OSPF unit tests pass, `make ze-lint-changed` reports 0 issues, and
  `make ze-interop-test INTEROP_SCENARIO=ospf-stub-nssa-frr` passes against FRR and fails
  when the production change is reverted.
- The receive-side install gates ALREADY cover OSPFv3 with no new code: `ExternalInput` is
  built once in the shared computer and `v6Strategy.ComputeExternal` delegates to the shared
  `ospfspf.ComputeExternalWith`. Those requirements need v6 PROOF, not v6 code.
- The summary policy map already carries `Type: nssa` and `NoSummary` for a v6 engine, so
  the v6 no-summary default is a single branch in `v6ApplyAreaTypePolicy`.
- The v6 origination path has a keep-set hazard that the v4 bookkeeping does not: a v6 NSSA
  default would be swept by any unrelated redistribution withdrawal unless its `SelfLSARef`
  joins the `v6WithdrawExternal` keep-set.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/ospf/nssa.go` - `applyNSSADefaults` reconciles the per-area NSSA
  default. It has NO address-family dispatch, unlike `translateNSSA` in the same file which
  branches on `e.dispatch != nil && e.dispatch.codec.IsV6()` and delegates to
  `translateNSSAV6`. It calls the OSPFv2 producer unconditionally.
- [ ] `internal/plugins/ospf/lsdb/nssa.go` - `OriginateNSSA` installs an OSPFv2 LSA keyed
  `types.LSTypeNSSA` (0x0007) with a `packet.ExternalLSA` body and a `[4]byte` forwarding
  address. It carries the P-bit in the header Options (`types.OptionNP`) and re-clears it
  when the forwarding address is zero or a self Type-5 exists for the same network. No
  family guard.
- [ ] `internal/plugins/ospf/origination_v6_nssa.go` - `v6OriginateNSSALSA` is the OSPFv3
  producer: key `v6NSSAKey` (LS Type 0x2007), body carrying the P-bit in the prefix's
  `OptPrefixP`, a `[16]byte` forwarding address plus a `HasForwardingAddr` flag, and an
  arbitrary caller-supplied LSID. `externalScopeV6` determines NSSA attachment and the v6
  forwarding address through `interfaceIPv6ForwardingAddress`, which filters out link-local,
  loopback, unspecified, multicast and 4-in-6 addresses.
- [ ] `internal/plugins/ospf/origination_v6_stub.go` - `v6ApplyAreaTypePolicy` drops Type-4
  equivalents and, when `NoSummary`, every inter-area prefix, then injects the `::/0` default
  for `AreaTypeStub` ONLY. Its OSPFv2 twin `spf/area_type.go applyAreaTypePolicy` now also
  injects for a no-summary NSSA.
- [ ] `internal/plugins/ospf/origination_v6_external.go` - `v6WithdrawExternal` builds its
  keep-set from `redistV6` LSIDs and `translations` only, then calls `FlushStaleSelfLSAs`
  with `v6ExternalSelfTypes`, which INCLUDES `LSTypeNSSA`.
- [ ] `internal/plugins/ospf/spf/external.go` - `ComputeExternalWith` holds the new
  receive-side gates, keyed on `NSSABorderRouter` and `NSSAPolicies[area].NoSummary`.
- [ ] `internal/plugins/ospf/afstrategy_v6.go` - `v6Strategy.ComputeExternal` delegates to
  the shared `ospfspf.ComputeExternalWith`, so the new gates already apply to OSPFv3.
- [ ] `internal/plugins/ospf/spf/computer.go` - builds `ExternalInput` once for both
  families, setting `NSSAAreas`, `NSSAPolicies` and `NSSABorderRouter`.
- [ ] `test/ospf/ospf-nssa.ci` - config validation only: three `ze config validate` runs. It
  drives no daemon and asserts nothing about originated LSAs. It also accepts an area with
  BOTH `no-summary true` and `nssa { default-originate true }`.
- [ ] `test/interop/scenarios/ospf-stub-nssa-frr/` - Ze is a dual-area OSPFv2 NSSA ABR; FRR
  is an NSSA-internal router with no backbone, so FRR never originates a Type-7 default and
  Ze's border-router install gates are never reached by a peer.
- [ ] `test/interop/scenarios/ospf-v6-nssa-redist-frr/` - Ze is SINGLE-area, an NSSA-internal
  ASBR, so `IsABR` is false and no OSPFv3 ABR default behaviour is exercised.

**Behavior to preserve:**
- The OSPFv2 behaviour the uncommitted change already delivers and proved against FRR: an
  NSSA ABR originates a Type-7 default into a regular NSSA with no operator gate, a
  no-summary NSSA receives a Type-3 default instead, and the receive-side gates discard a
  P-clear Type-7 default and every Type-7 default under suppressed summary import.
- The v4 NSSA policy split recorded in learned 975: policy is shared, only the wire encode
  varies by family.
- Existing OSPFv3 redistribution and Type-7 to Type-5 translation, including the `nssaMu`
  then `e.mu` lock order.
- `nssa { default-originate }` continues to mean an internal-router P-set Type-7 default,
  which is the meaning the uncommitted change gave it.

**Behavior to change:**
- `applyNSSADefaults` gains an address-family branch so an OSPFv3 engine originates through
  `v6OriginateNSSALSA` instead of the OSPFv2 producer.
- `v6ApplyAreaTypePolicy` injects the `::/0` default for a no-summary NSSA as well as a stub.
- The OSPFv3 default's self-LSA reference joins the `v6WithdrawExternal` keep-set so an
  unrelated redistribution withdrawal cannot purge it.
- RFC 3101 Section 2.4 mutual exclusivity is enforced: an internal router does not originate
  a Type-7 default into a no-summary NSSA.
- `externalScopeV6` gains the deterministic ordering, zero-address upgrade and active-
  interface filter its OSPFv2 counterpart `externalScopeFor` already has.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
Two entry points, one per direction.

- **Origination:** operator config (`ospf { areas { area X { area-type nssa ... } } }`)
  arrives as a YANG-modelled tree, is parsed by `parseOSPFConfig` and applied through
  `engine.reconcile`. A second trigger is the one-second engine timer, which re-runs the
  same reconciliation so interface state changes take effect.
- **Installation:** a peer-originated Type-7 NSSA-LSA arrives on the wire, is flooded into
  the area LSDB, and is read during the SPF external calculation.

### Transformation Path
1. `engine.reconcile` calls `applyNSSADefaults` (also called from the 1 Hz timer).
2. `applyNSSADefaults` snapshots the running interface set, derives per-area activity from
   `lsdbTopology` filtered by `lsdb.AreaHasAdvertisedLinks`, and derives ABR status from
   `ospfspf.IsABR` over the active areas.
3. It decides per area: an ABR wants a P-clear Type-7 default for a regular NSSA; a
   no-summary NSSA gets nothing here because its default is a Type-3 from the summary path;
   a non-ABR with `default-originate` and a usable forwarding address wants a P-set Type-7.
4. **The family branch this spec adds:** OSPFv2 originates through `lsdb.OriginateNSSA`,
   OSPFv3 through `v6OriginateNSSALSA` with an area-scoped LSID.
5. Separately, `Computer.Run` builds `SummaryInput.Policies` from `areaConfigMaps` and calls
   `OriginateSummaries`, which applies `applyAreaTypePolicy` (v2) or `v6ApplyAreaTypePolicy`
   (v3) per destination area. This is where the no-summary NSSA's Type-3 / `::/0` default
   is injected.
6. On the receive side, `Computer.Run` builds one `ExternalInput` carrying `NSSAAreas`,
   `NSSAPolicies` and `NSSABorderRouter`, and both strategies call the shared
   `ComputeExternalWith`, whose gates discard the disallowed Type-7 defaults before the
   route reaches `selectBestRoutes` and the Loc-RIB installer.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree → engine | `parseOSPFConfig` → `ospfConfig.Areas` (`AreaType`, `NoSummary`, `DefaultCost`, `NSSADefaultOriginate`) | No |
| Engine → LSDB (v2) | `lsdb.OriginateNSSA`, key `types.LSTypeNSSA` (0x0007), `packet.ExternalLSA` body, `[4]byte` FA | No |
| Engine → LSDB (v3) | `v6OriginateNSSALSA`, key `v6NSSAKey` (0x2007), P-bit in `OptPrefixP`, `[16]byte` FA | No |
| SPF → summary origination | `SummaryInput.Policies` (shared by both families) → `applyAreaTypePolicy` / `v6ApplyAreaTypePolicy` | No |
| LSDB → SPF external calc | `ExternalInput` (`NSSABorderRouter`, `NSSAPolicies`) → `ComputeExternalWith` | No |
| Engine → peer daemon | Type-7 LSA on the wire, verified against FRR in the interop scenarios | No |

### Integration Points
- `engine.reconcile` and the 1 Hz timer both call `applyNSSADefaults`; the family branch
  must hold for both.
- `v6WithdrawExternal`'s keep-set and `v6ExternalSelfTypes` manage-set decide whether a
  self-originated v6 NSSA LSA survives an unrelated redistribution withdrawal.
- `nssaMu` serialises NSSA reconciliation against translation; the documented lock order is
  `nssaMu` then `e.mu` and the new code must not invert it.
- `docs/features/rfc-status.md` carries the public RFC 3101 row, and `check_status_completeness`
  plus the Remaining-count agreement check read it.

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
| A-1 | The receive-side install gates already cover OSPFv3 with no new production code | `afstrategy_v6.go` `v6Strategy.ComputeExternal` delegates to `ospfspf.ComputeExternalWith`; `spf/computer.go` builds one `ExternalInput` for both families. Both read in the main thread | v6 needs its own install-gate implementation, roughly doubling the code scope | A v6 unit test asserting the gate discards a v6 Type-7 default on a border router | confirmed |
| A-2 | The shared summary policy map carries `Type: nssa` and `NoSummary` for a v6 engine at runtime | `origination_v6_summary.go` `v6OriginateSummaries` passes `in.Policies[dst]`; `spf_wiring.go` fills `AreaConfig` address-family-agnostically. Read in the main thread | The v6 no-summary default needs its own policy plumbing rather than one branch | A v6 unit test driving `v6OriginateSummaries` for a no-summary NSSA | confirmed |
| A-3 | The v6 NSSA default can take LSID 0 without colliding with a redistribution LSID | `v6InjectExternal` pre-increments (`e.redistV6Next++` then `v6SummaryLSID(e.redistV6Next)`), so redistribution allocations start at 1 and 0 is never taken. Read in the main thread | An LSID collision would overwrite or purge a redistributed external LSA | A unit test originating the default alongside a redistributed external and asserting both survive | confirmed |
| A-4 | FRR 10.3.1 originates a Type-7 default only when it is an ABR, in both families | `ospfd` grammar read from the pinned image plus one-hop upstream evidence; `ospf6d` carries `ospf6_abr_nssa_type_7_default_create` / `_delete` / `ospf6_abr_nssa_type_7_defaults`, all ABR-named. Read in the main thread | The peer would originate without a backbone area and the scenario topology could be simpler | Both interop scenarios give the FRR side its own backbone area; the LSA appearing in Ze's LSDB validates it | confirmed |
| A-5 | FRR `ospf6d` can originate an OSPFv3 Type-7 default | Confirmed in the main thread from the pinned image: `area <A.B.C.D\|(0-4294967295)> nssa [{default-information-originate [{metric (0-16777214)\|metric-type (1-2)}]\|no-summary ...}]`, help string "Originate Type 7 default into NSSA area" | The OSPFv3 receive-side gates would need crafted-LSA injection through `inject_v3.go` instead of a real peer | The OSPFv3 two-ABR interop scenario originating the default | confirmed |
| A-7 | An FRR ABR's Type-7 default is P-clear, so it is exactly the LSA RFC3101-2.4-4 requires Ze to refuse | RFC 3101 Section 2.4 requires the P-bit clear on an ABR-originated Type-7 default, and FRR is presumed conformant | The negative direction of RFC3101-2.4-4 needs an injected P-clear default rather than a peer-originated one | Assert the received LSA's P-bit in the scenario before asserting Ze refuses it | unvalidated |
| A-6 | A `.ci` under `test/ospf/` earns verify-tier evidence | `mk/test-functional.mk` `all_suites` contains `ospf` and `ospfv3`. Read in the main thread | The new functional tests would resolve `unrun` and their RFC tags would be refused | `make ze-rfc-check` accepting the new tags | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A v6 NSSA default is purged by an unrelated redistribution withdrawal, because `v6ExternalSelfTypes` includes `LSTypeNSSA` and `v6WithdrawExternal` builds its keep-set only from `redistV6` and `translations` | A redistributed external is withdrawn and the NSSA default vanishes with it | Add the default's `SelfLSARef` to that keep-set, and add a unit test that withdraws an unrelated external then asserts the default survives |
| R-2 | THREE sites compute ABR status independently and on different clocks: `isAreaBorderRouter` inside `lsdb.OriginateFromTopology` (which sets the advertised Router-LSA B-bit), `ospfspf.IsABR` in `applyNSSADefaults` (live interface state, 1 Hz tick), and `IsABR` in `spf/computer.go` (SPF-result presence). A no-summary NSSA can briefly hold neither default across a backbone transition, and Ze can advertise B=1 while originating no default | Transient absence of any default in a backbone flap test; or a Router-LSA with the B-bit set while no default LSA exists | Owner decision 2026-08-02: UNIFY. Make the Router-LSA B-bit determination the single producer and have both default-route consumers read it, so what Ze advertises and what Ze originates cannot disagree |
| R-3 | The meaning of `nssa { default-originate }` changed: it is now inert on an ABR. An operator upgrading gets a default they did not configure, and a leaf that silently stops doing what its old description said | An operator reports an unexpected `0.0.0.0/0` in an NSSA | Document in `docs/guide/ospf.md` and the YANG description, and call it out at closure as an operator-visible change |
| R-4 | `ai/RFC-REQUIREMENTS.md` regeneration is entangled with a concurrent session's uncommitted rfc9190 work, so `make ze-rfc-index-update` would sweep foreign changes into this commit | `make ze-rfc-check` stays red on the staleness violation | Owner action: sequence the regeneration against the other session rather than running it blind |
| R-5 | The evidence ratchet keys on `kind/tier`, so substituting a verify-tier `.ci` binding for a nightly-tier interop one fires it even at unchanged tag count | `check_evidence_ratchet` fails on a requirement whose evidence kind changed | Every new binding ADDS; no existing tag is moved or retargeted |
| R-6 | `option=netns-link` tests skip outside `ZE_TEST_NETNS`, so a daemon-driving OSPF `.ci` runs under `make ze-netns-test` but not under `ze-qemu-needs-linux-test` | The new `.ci` passes locally but contributes no evidence in the tier that was expected | Confirm in DESIGN which suite the functional evidence must land in, and pick the option set accordingly |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Wire-visible and routing-visible. A wrong default origination sends an NSSA's internal routers a route out that does not work, or removes the only one they have, so traffic to AS-external destinations blackholes. A malformed OSPFv2-format LSA flooded into an OSPFv3 area can be rejected by conforming peers, which is an interop failure. A wrong install gate installs or drops a default route. |
| How is it reverted? | A single commit revert restores the pre-change behaviour, but peers will have seen the LSAs. A withdrawn default takes a MaxAge flood to clear from every neighbour, so revert is not instant from the peers' point of view. |
| Who else touches this path? | A concurrent session owns the uncommitted IKE/IPsec and rfc9190 work, including `rfc/not-enrolled.txt` and the staleness of `ai/RFC-REQUIREMENTS.md`. `plan/spec-ospf-ext-13-l3vpn-dn-bit.md` is the only other open OSPF spec and does not touch NSSA defaults. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ospf { areas { area X { area-type nssa } } }` on an OSPFv3 ABR | → | `applyNSSADefaults` family branch → `v6OriginateNSSALSA` | `TestOSPFv3NSSABorderRouterOriginatesDefault` |
| `ospf { areas { area X { area-type nssa no-summary true } } }` on an OSPFv3 ABR | → | `v6ApplyAreaTypePolicy` → `v6OriginateInterAreaPrefix` | `TestOSPFv3NSSANoSummaryDefaultInjection` |
| A peer-originated Type-7 default arriving on an OSPFv3 NSSA interface | → | `v6Strategy.ComputeExternal` → `ComputeExternalWith` gates | `TestOSPFv3NSSABorderRouterDefaultPBit` |
| An operator running `ze config` for an NSSA ABR, then `show ospf database nssa-external` | → | `reconcile` → `applyNSSADefaults` → LSDB → `show ospf` | `test/ospf/ospf-nssa-abr-default.ci` |
| A redistributed external being withdrawn while an NSSA default exists | → | `v6WithdrawExternal` keep-set → `FlushStaleSelfLSAs` | `TestOSPFv3NSSADefaultSurvivesUnrelatedWithdrawal` |
| A backbone interface going down on an ABR attached to a no-summary NSSA | → | `isAreaBorderRouter` (single producer) → `applyNSSADefaults` and `OriginateSummaries` | `TestOSPFNSSANoSummaryDefaultSurvivesBackboneFlap` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An OSPFv3 router with an active backbone area and an active regular NSSA area | Originates an OSPFv3 NSSA-LSA (LS Type 0x2007) for the default destination into that NSSA, with the P-bit clear in the prefix options, with no operator gate (RFC3101-2.4-5) |
| AC-2 | An OSPFv3 ABR attached to a no-summary NSSA | Originates an Inter-Area-Prefix-LSA for `::/0` at the area default-cost into that area, and originates no Type-7 default there (RFC3101-2.7-2) |
| AC-3 | Any OSPFv3 engine originating an NSSA default | No LSA keyed with an OSPFv2 LS Type (0x0007) is installed in any area store |
| AC-4 | An OSPFv3 ABR holding an NSSA default when an unrelated redistributed external is withdrawn | The NSSA default LSA remains installed and non-purged |
| AC-5 | A non-ABR NSSA router with `default-originate` enabled in a no-summary NSSA, in either family | Originates no Type-7 default (RFC 3101 Section 2.4 mutual exclusivity) |
| AC-6 | An NSSA border router receiving a Type-7 default whose P-bit is clear, in either family | Does not install a default route from it (RFC3101-2.4-4 install clause) |
| AC-7 | An NSSA border router that suppresses Type-3 summary import, receiving any Type-7 default | Does not install a default route from it (RFC3101-2.5-1) |
| AC-8 | A router that is NOT an NSSA border router receiving a P-clear Type-7 default | Does install a default route from it, proving the gate is scoped to border routers |
| AC-9 | An operator configuring an NSSA ABR with no `nssa { default-originate }` leaf | The running daemon's `show ospf database nssa-external` reports the self-originated default |
| AC-10 | An operator setting `nssa { default-originate true }` on an internal NSSA router with no usable forwarding address | The daemon originates no Type-7 default (RFC3101-2.4-2) |
| AC-11 | `make ze-rfc-check` over the tree | RFC3101-2.4-4, 2.4-5, 2.5-1 and 2.7-2 each carry positive and negative tagged evidence, and the Section 2.7 MUST NOT carries a checklist id with both polarities |
| AC-12 | A reader of `docs/features/rfc-status.md` and `docs/guide/ospf.md` | Both state the NSSA default behaviour for BOTH address families, carry source anchors, and the Remaining count agrees with the real `{gap}` count |
| AC-13 | Any reachable router state, including mid-transition, with at least one attached NSSA | What Ze ADVERTISES and what Ze ORIGINATES agree: whenever the self Router-LSA carries the B-bit, every attached NSSA holds its required default (Type-7 for a regular NSSA, Type-3 or `::/0` for a no-summary one), and whenever the B-bit is clear, Ze originates no border-router default in any area. The single-producer refactor is the means of achieving this, not the assertion |
| AC-14 | A backbone interface transitioning down then up on a router attached to a no-summary NSSA | The area never holds zero defaults as a result of the two consumers disagreeing. Any remaining absence is bounded by the single producer's own update, not by a race between producers |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures an OSPFv2 NSSA ABR and expects its internal routers to reach external destinations | config → `reconcile` → `applyNSSADefaults` → `lsdb.OriginateNSSA` → flood → FRR route table | `test/interop/scenarios/ospf-stub-nssa-frr` (already passing) |
| 2 | Configures an OSPFv3 NSSA ABR and expects the same | config → `reconcile` → family branch → `v6OriginateNSSALSA` → flood → FRR `::/0` | `test/interop/scenarios/ospf-v6-nssa-abr-frr` |
| 3 | Runs two NSSA ABRs and expects neither to install the other's P-clear default | peer LSA → LSDB → `ComputeExternalWith` gates → no route | `test/interop/scenarios/ospf-nssa-two-abr-frr` |
| 4 | Configures a totally-NSSA area and expects one way out | config → `Computer.Run` → `OriginateSummaries` → `applyAreaTypePolicy` / `v6ApplyAreaTypePolicy` | `test/ospf/ospf-nssa-no-summary-default.ci` |
| 5 | Inspects what the daemon originated | `show ospf database nssa-external` JSON | `test/ospf/ospf-nssa-abr-default.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFv3NSSABorderRouterOriginatesDefault` | `internal/plugins/ospf/origination_v6_nssa_test.go` | AC-1: LS Type 0x2007, P-bit clear in prefix options, default destination | |
| `TestOSPFv3NSSADefaultUsesV6Producer` | `internal/plugins/ospf/origination_v6_nssa_test.go` | AC-3: no 0x0007-keyed LSA exists in the area store after a v6 default origination | |
| `TestOSPFv3NSSANoSummaryDefaultInjection` | `internal/plugins/ospf/origination_v6_stub_test.go` | AC-2: `::/0` at default-cost for a no-summary NSSA, absent for a regular NSSA | |
| `TestOSPFv3NSSADefaultSurvivesUnrelatedWithdrawal` | `internal/plugins/ospf/origination_v6_external_test.go` | AC-4 and R-1: the keep-set retains the default across `v6WithdrawExternal` | |
| `TestOSPFv3NSSADefaultLSIDDoesNotCollide` | `internal/plugins/ospf/origination_v6_external_test.go` | A-3: the default's LSID never equals a redistribution LSID | |
| `TestOSPFNSSAInternalDefaultExcludedByNoSummary` | `internal/plugins/ospf/nssa_ac14_16_test.go` | AC-5 both families: Section 2.4 mutual exclusivity | |
| `TestOSPFv3NSSABorderRouterDefaultPBit` | `internal/plugins/ospf/spf/external_nssa_test.go` | AC-6 and AC-7 on the v6 reader, proving the shared gate fires for 0x2007 | |
| `TestOSPFNSSANonBorderRouterInstallsPClearDefault` | `internal/plugins/ospf/spf/external_nssa_test.go` | AC-8: the permissive direction, currently unproven | |
| `TestOSPFv3NSSADefaultForwardingAddressDeterministic` | `internal/plugins/ospf/origination_v6_nssa_test.go` | `externalScopeV6` ordering and zero-address upgrade parity with the v4 path | |
| `TestOSPFNSSADefaultAgreesWithRouterLSABBit` | `internal/plugins/ospf/nssa_ac14_16_test.go` | AC-13: table-driven over reachable states. For each, assert the self Router-LSA B-bit and the per-area default set agree in both directions, both families | |
| `TestOSPFNSSANoSummaryDefaultSurvivesBackboneFlap` | `internal/plugins/ospf/nssa_ac14_16_test.go` | AC-14: a backbone down/up transition never leaves a no-summary NSSA holding zero defaults through producer disagreement | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `default-cost` (area) | 0..16777215 | 16777215 | N/A (uint32, 0 is valid) | 16777216 |
| Type-7 default metric on the wire | 0..16777215 (24-bit external metric) | 16777215 | N/A | value is masked, assert the mask |
| NSSA default LSID (v6) | 0 reserved for the default | 0 | N/A | redistribution starts at 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-nssa-abr-default` | `test/ospf/ospf-nssa-abr-default.ci` | An operator configures an NSSA ABR with no `default-originate` leaf and the daemon originates the Type-7 default anyway | |
| `ospf-nssa-no-summary-default` | `test/ospf/ospf-nssa-no-summary-default.ci` | An operator configures a no-summary NSSA and the daemon originates a Type-3 default rather than a Type-7 | |
| `ospf-nssa-internal-default` | `test/ospf/ospf-nssa-internal-default.ci` | An operator sets `default-originate` on an internal NSSA router and the daemon originates a P-set Type-7 only when a forwarding address is usable | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-stub-nssa-frr` | `test/interop/scenarios/` (existing) | FRR 10.3.1 `ospfd` | AC-1 for OSPFv2: the ABR default reaches FRR with no `default-originate` leaf. Already passing and already discriminating | passing |
| `ospf-v6-nssa-abr-frr` | `test/interop/scenarios/` (new) | FRR 10.3.1 `ospf6d` | AC-1 and AC-3 for OSPFv3: Ze as a dual-area v6 NSSA ABR originates a 0x2007 default and FRR installs `::/0` as an NSSA route | |
| `ospf-nssa-two-abr-frr` | `test/interop/scenarios/` (new) | FRR 10.3.1 `ospfd` | AC-6 and AC-7 for OSPFv2: FRR is a second NSSA ABR configured with `area X nssa default-information-originate`, so Ze receives a P-clear Type-7 default and must not install it. A no-summary variant proves AC-7 | |
| `ospf-v6-nssa-two-abr-frr` | `test/interop/scenarios/` (new) | FRR 10.3.1 `ospf6d` | AC-6 and AC-7 for OSPFv3 through the same two-ABR topology | |

Each new scenario asserts on BOTH sides: FRR's route table via `FRROSPF6.wait_ospf6_route` or
`FRROSPF.wait_ospf_route`, and Ze's own state via `docker_exec_quiet(ZE_CONTAINER, ["ze",
"show", "ospf", "database", "nssa-external"])` and `["ze", "show", "ospf", "route"]`. The
receive-side scenarios must show the LSA PRESENT in Ze's LSDB while `0.0.0.0/0` (or `::/0`)
is ABSENT from Ze's route table: that pair is what distinguishes "gate worked" from "LSA
never arrived", which is the vacuity trap `ai/rules/interop-and-goal-validation.md` names.

## Files to Modify
- `internal/plugins/ospf/nssa.go` - extract the address-family-neutral default-route policy
  decision, add the family branch, enforce Section 2.4 mutual exclusivity, and add the RFC
  citation comments at the enforcing sites
- `internal/plugins/ospf/origination_v6_nssa.go` - the OSPFv3 default originator and the
  `externalScopeV6` determinism, zero-address upgrade and active-interface parity
- `internal/plugins/ospf/origination_v6_stub.go` - inject `::/0` for a no-summary NSSA
- `internal/plugins/ospf/origination_v6_external.go` - add the default's `SelfLSARef` to the
  `v6WithdrawExternal` keep-set so an unrelated withdrawal cannot purge it
- `internal/plugins/ospf/spf/area_type.go` - RFC citation comment on the Type-3 default
- `internal/plugins/ospf/spf/external.go` - RFC citation comments on both gates
- `internal/plugins/ospf/lsdb/origination.go` - expose the `isAreaBorderRouter` result that
  sets the Router-LSA B-bit, so it becomes the single ABR producer (R-2, AC-13)
- `internal/plugins/ospf/spf/computer.go` - read ABR status from that producer instead of
  recomputing it from SPF-result presence (R-2, AC-13)
- `internal/plugins/ospf/instance.go` - remove the `ipv4Address` production test seam in
  favour of the existing package-helper pattern, or justify it in Design Insights
- `rfc/short/rfc3101.md` - add the Section 2.7 MUST NOT checklist id
- `docs/features/rfc-status.md` - state both families, correct the Remaining count
- `docs/guide/ospf.md` - state both families, restore the still-true OSPFv3 P-bit prose, fix
  the orphaned `v6OriginateNSSALSA` source anchor
- `docs/architecture/wire/ospfv3.md` - add the NSSA-LSA and default-route material it lacks
- `test/interop/scenarios/ospf-v6-nssa-redist-frr/` - unchanged, but referenced as the
  single-area contrast case

## Files to Create
- `test/ospf/ospf-nssa-abr-default.ci` - AC-9 through the running daemon
- `test/ospf/ospf-nssa-no-summary-default.ci` - AC-2 for OSPFv2 through the running daemon
- `test/ospf/ospf-nssa-internal-default.ci` - AC-10 through the running daemon
- `test/ospfv3/ospfv3-nssa-abr-default.ci` - AC-1 and AC-3 for OSPFv3 through the daemon
- `test/interop/scenarios/ospf-v6-nssa-abr-frr/{ze.conf,frr.conf,check.py}`
- `test/interop/scenarios/ospf-nssa-two-abr-frr/{ze.conf,frr.conf,check.py}`
- `test/interop/scenarios/ospf-v6-nssa-two-abr-frr/{ze.conf,frr.conf,check.py}`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new leaf. `nssa { default-originate }` already exists; only its description changes, and that edit is already in the tree |
| YANG validation constraints | No | No new leaf to constrain |
| YANG custom validators | Yes | `internal/plugins/ospf/yang/ze-ospf-conf.yang`: consider rejecting or warning on `no-summary true` together with `nssa { default-originate true }`, which RFC 3101 Section 2.4 calls mutually exclusive and which `test/ospf/ospf-nssa.ci` currently accepts as valid |
| CLI commands/flags | No | `show ospf database nssa-external` already exists and is the read surface |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | No | No new leaf; the existing enum and boolean leaves already complete |
| Functional test for new RPC/API | Yes | `test/ospf/*.ci` and `test/ospfv3/*.ci` listed in Files to Create |
| Pipe completeness | No | Output flows through the existing `show ospf` handlers, unchanged |
| Env var registration | N-A | No environment leaf involved |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module, binary or certificate. The change adds no runtime dependency |
| Prometheus counters/metrics | No | `ze_ospf_nssa_translations_total` already covers translation; the default route adds no new observable requiring a counter. Revisit if DESIGN adds one |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`: the NSSA default route now works in both families without an operator gate |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`: `nssa { default-originate }` changed meaning and is now inert on an ABR |
| 3 | CLI command added/changed? | No | `show ospf database nssa-external` is unchanged |
| 4 | API/RPC added/changed? | No | No new command or RPC |
| 5 | Plugin added/changed? | No | The `ospf` plugin's registration is unchanged |
| 6 | Has a user guide page? | Yes | `docs/guide/ospf.md`: state both families, restore the OSPFv3 P-bit prose, fix the orphaned source anchor |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/ospfv3.md` has no NSSA material at all and must gain the NSSA-LSA and default-route description |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface touched |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc3101.md` (the Section 2.7 MUST NOT id) and the `docs/features/rfc-status.md` RFC 3101 row including its Remaining count |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`: three new interop scenarios and four new `.ci` tests |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: NSSA default-route conformance is a feature-parity claim against FRR and BIRD |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`: the address-family dispatch obligation for NSSA default origination |
| 13 | Route metadata keys added/changed? | No | No metadata key involved |
| 14 | Prometheus counters added/changed? | No | No counter added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registered changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `nssa.go`, `origination_v6_nssa.go`, `origination_v6_stub.go`, `spf/area_type.go`, `spf/external.go`; `docs/guide/ospf.md` already carries a `v6OriginateNSSALSA` anchor that currently anchors nothing |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ospf.md` shows `nssa { default-originate true }`; the example must be re-framed as an internal-router setting |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove an OSPFv3 engine reaches a v6 default originator at all
   - Tests: `TestOSPFv3NSSABorderRouterOriginatesDefault`, `TestOSPFv3NSSADefaultUsesV6Producer`
   - Files: `internal/plugins/ospf/nssa.go` (family branch), `origination_v6_nssa.go` (stub originator)
   - Verify: both tests FAIL first because an OSPFv3 ABR currently installs a 0x0007-keyed LSA. `TestOSPFv3NSSADefaultUsesV6Producer` is the wiring test and must go red before it goes green
2. **Phase: Unify the ABR producer** -- one source of truth for "am I an ABR"
   - Tests: `TestOSPFNSSADefaultAgreesWithRouterLSABBit`, `TestOSPFNSSANoSummaryDefaultSurvivesBackboneFlap`
   - Files: `internal/plugins/ospf/lsdb/origination.go` (expose the `isAreaBorderRouter` result that sets the Router-LSA B-bit), `internal/plugins/ospf/nssa.go`, `internal/plugins/ospf/spf/computer.go`
   - Verify: AC-13 and AC-14. The B-bit determination becomes the producer; `applyNSSADefaults` and the summary path both READ it. Grep proves no third site recomputes ABR status from its own snapshot
3. **Phase: Policy extraction** -- lift the address-family-neutral default decision out of `applyNSSADefaults`
   - Tests: the whole existing OSPFv2 suite must stay green, unchanged, including the discriminating FRR interop
   - Files: `internal/plugins/ospf/nssa.go`
   - Verify: `make ze-interop-test INTEROP_SCENARIO=ospf-stub-nssa-frr` still passes. No OSPFv2 assertion is edited in this phase; if one needs editing, the extraction changed behaviour and is wrong. The extracted decision takes ABR status as an INPUT from phase 2's single producer, never recomputing it
4. **Phase: OSPFv3 origination** -- the v6 default, its LSID and its keep-set
   - Tests: `TestOSPFv3NSSADefaultSurvivesUnrelatedWithdrawal`, `TestOSPFv3NSSADefaultLSIDDoesNotCollide`, `TestOSPFv3NSSADefaultForwardingAddressDeterministic`
   - Files: `origination_v6_nssa.go`, `origination_v6_external.go`
   - Verify: R-1 is closed by a test that withdraws an unrelated external and asserts the default survives
5. **Phase: OSPFv3 no-summary default** -- `::/0` through the summary path
   - Tests: `TestOSPFv3NSSANoSummaryDefaultInjection`
   - Files: `origination_v6_stub.go`
   - Verify: a regular v6 NSSA gets no `::/0`, a no-summary one does, at the configured default-cost
6. **Phase: Section 2.4 mutual exclusivity and the permissive gate direction**
   - Tests: `TestOSPFNSSAInternalDefaultExcludedByNoSummary`, `TestOSPFNSSANonBorderRouterInstallsPClearDefault`, `TestOSPFv3NSSABorderRouterDefaultPBit`
   - Files: `nssa.go`, `spf/external_nssa_test.go`
   - Verify: AC-5, AC-6, AC-7 and AC-8 all have both polarities
7. **Phase: Functional `.ci` coverage** -- drive the daemon
   - Tests: the four `.ci` files in Files to Create
   - Files: `test/ospf/`, `test/ospfv3/`
   - Verify: each `.ci` asserts on `show ospf database nssa-external` JSON, and every `absent=` assertion is preceded by a step that makes the needle present
8. **Phase: Interop** -- prove it against FRR in both families and both directions
   - Tests: the three new scenarios
   - Files: `test/interop/scenarios/`
   - Verify: each scenario FAILS when the corresponding production change is reverted. Record which revert was used for each, per `ai/rules/interop-and-goal-validation.md`
9. **Phase: RFC ledger and docs**
   - Tests: `make ze-rfc-check`, `make ze-doc-verify`, `make ze-repository-check`
   - Files: `rfc/short/rfc3101.md`, `docs/features/rfc-status.md`, `docs/guide/ospf.md`, `docs/architecture/wire/ospfv3.md`, `docs/features.md`, `docs/guide/configuration.md`, `docs/comparison.md`, `docs/functional-tests.md`, `docs/architecture/core-design.md`
   - Verify: every new binding ADDS rather than substitutes, so `check_evidence_ratchet` stays green (R-5)

### Owner Actions (NOT agent work)

| # | Action | Why it is the owner's |
|---|--------|----------------------|
| O-1 | Supply the `rfc-test-change-approved:` token for `internal/plugins/ospf/nssa_ac14_16_test.go` and `internal/plugins/ospf/spf/area_type_test.go`, which `scripts/dev/audit-test-relaxation.py` reports as `[WEAKENED]` | The gate reserves RFC-tagged test edits to the owner. Coverage was verified intact by review: no RFC3101-3.2-2 or 2.7-1 assertion was lost |
| O-2 | Sequence the `ai/RFC-REQUIREMENTS.md` regeneration against the concurrent session | `make ze-rfc-index-update` would sweep that session's uncommitted rfc9190 work into this commit (R-4) |

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Feature completeness | Every user story has a working path, no broken links |
| Address-family parity | Every NSSA path that originates or purges an LSA either dispatches on family or is provably family-neutral. Grep for `types.LSTypeNSSA` and `types.LSTypeASExternal` used as bare constants on a path a v6 engine can reach, per learned 972 |
| Scope-typed LS types | New comparisons use `types.LSType.NSSA()` / `.ASExternal()` helpers, never OSPFv2 numeric constants |
| Test discrimination | Each new interop scenario and each new `.ci` was run with the corresponding production change reverted, and the revert used is recorded |
| Lost invariant | The old `applyNSSADefaults` comment required a non-zero forwarding address for the ABR default. Either restate why a P-clear default may carry a zero FA, or assert the FA |
| Rule: `ai/rules/rfc-compliance.md` | Every MUST enforced carries `// RFC NNNN Section X.Y: "<quote>"` directly above the enforcing code, and each tag sits above the code that ACTUALLY enforces it |
| Rule: `ai/rules/evidence.md` | The `ComputeExternalWith` gates fail closed: confirm a zero-value `ExternalInput` (nil `NSSAPolicies`, false `NSSABorderRouter`) cannot silently disable both MUSTs on any real caller |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| OSPFv3 NSSA ABR originates a 0x2007 default | `go test -tags "ze_core $(features)" ./internal/plugins/ospf/...` and the new interop scenario |
| No 0x0007 LSA from a v6 engine | `TestOSPFv3NSSADefaultUsesV6Producer` |
| OSPFv3 no-summary NSSA gets `::/0` | `TestOSPFv3NSSANoSummaryDefaultInjection` |
| Default survives unrelated withdrawal | `TestOSPFv3NSSADefaultSurvivesUnrelatedWithdrawal` |
| Both gate polarities proven against FRR | `make ze-interop-test INTEROP_SCENARIO=ospf-nssa-two-abr-frr` and the v6 twin |
| Functional coverage of the daemon | `make ze-functional-test` covering the `ospf` and `ospfv3` suites |
| RFC ledger consistent | `make ze-rfc-check` exits 0 for rfc3101 (the rfc9190 and staleness violations are O-2's, not this spec's) |
| Docs consistent | `make ze-doc-verify`, `make ze-repository-check` |
| No test weakened | `python3 scripts/dev/audit-test-relaxation.py` clean for OSPF paths, or O-1 token supplied |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The Type-7 default arrives from a peer. Its metric is masked to the 24-bit external range, its prefix must be the default destination, and its forwarding address must be resolvable. Confirm a malformed or oversized field cannot reach the route table |
| Resource | An unbounded number of peer-originated Type-7 defaults must not grow per-area state without limit. The install gates discard rather than accumulate |
| Guard fails closed | Both new gates DENY on match. Verify the miss path (zero-value input) cannot be reached with a real `ExternalInput`, since a permissive miss silently disables two MUSTs |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

**The RFC requirement checklist has no address-family dimension, and that is the
structural reason this defect was invisible.** A requirement id such as RFC3101-2.4-5
is per-RFC, not per-family. `make ze-rfc-check` treats it as satisfied once a tagged
test exists in both polarities, so a test exercising only the OSPFv2 path marks the
obligation proven for OSPFv3 as well. Attempting to record the truth with a
family-scoped `{gap}` is actively refused: the checker reports "annotated {gap} but IS
tested -- the annotation is stale", because tested and gap are modelled as mutually
exclusive. The disclosure therefore has to live in the `docs/features/rfc-status.md`
Remaining prose, which no machine checks for family coverage.

This is worth routing at closure, per `ai/rules/repo-maintenance.md`. Ze implements
several protocols in one engine across two address families (OSPF here, and the same
shape exists wherever a codec seam serves two families), so any RFC whose obligations
are family-sensitive can be green on a gate while unimplemented for one family. Two
candidate fixes, both larger than this spec: allow a requirement id to carry a family
qualifier, or require that a tagged test declare which family it exercises so the
checker can demand both.

**The install-side gate is genuinely address-family-neutral, by construction.**
`ComputeExternalWith` filters on `h.Type.NSSA()`, the scope-aware helper that matches
both 0x0007 and 0x2007, rather than on an OSPFv2 numeric constant. That is exactly the
discipline learned 972 records, and it is why the receive side needed no v6 code while
the origination side needed a whole branch.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Extract the address-family-neutral default-route policy decision from `applyNSSADefaults`, then let a thin per-family originator apply it | (A) Mirror the existing pattern exactly: an early `IsV6()` return delegating to a sibling `applyNSSADefaultsV6`, duplicating the ABR determination and the want/not decision. (C) Treat the v6 default as a pseudo-external routed through the redistribution keep-set machinery | Learned 975 already recorded the intended split: "reuse the v4 NSSA policy and vary ONLY the wire encode". This defect exists precisely because policy and encode were fused in one function, so (A) would duplicate the fusion and guarantee the next RFC change drifts again. (C) conflates a policy-originated default with a redistributed external, which breaks withdrawal semantics. The cost of extraction is touching an already-proven OSPFv2 path, mitigated by the existing discriminating tests: phase 2 edits no OSPFv2 assertion, so any behaviour change shows up as a red test |
| Prove the receive-side gates with a two-ABR topology where FRR originates the P-clear default | Crafted-LSA injection through `inject_v3.go`; accepting unit-only proof | An FRR ABR's Type-7 default is P-clear because RFC 3101 Section 2.4 requires it to be, so the peer naturally produces exactly the LSA Ze must refuse. That is stronger evidence than Ze injecting an LSA into itself. Injection stays available as a fallback if A-7 proves false |
| Give the OSPFv3 default LSID 0 | Allocate it from `redistV6Next`; key it by prefix like a redistributed external | `v6InjectExternal` pre-increments before allocating, so redistribution never uses 0. Reserving 0 for the default destination is both collision-free and semantically apt |
| Reuse the shared `ComputeExternalWith` gates for OSPFv3 rather than writing a v6 install path | A v6-specific install gate | `v6Strategy.ComputeExternal` already delegates to the shared function and `ExternalInput` is built once for both families, so the gates already apply. The v6 work here is proof, not code |
| Make the Router-LSA B-bit determination (`isAreaBorderRouter` in `lsdb.OriginateFromTopology`) the single ABR producer, read by both default-route consumers | Documenting the transient window as a Known Limitation; adding an AC and a flap test while keeping three producers | Owner decision 2026-08-02. The B-bit is what Ze ADVERTISES, so it is the only determination peers can observe. Three independent computations on different clocks let Ze claim B=1 while originating no default, which is an inconsistency visible on the wire rather than merely an internal race. Unifying on the advertised value makes the claim and the behaviour the same fact |

## Known Limitations
- RFC3101-2.2-1 (Type-7 address-range aggregation into one Type-5 with a 0.0.0.0 forwarding
  address) stays unimplemented. It is a MAY, it is unchanged by this spec, and Appendix E's
  DoNotAdvertise technique depends on it.
- The `nssa { default-originate }` leaf becomes inert on an ABR rather than being rejected
  there. Making it an error would be a config-breaking change for existing deployments; this
  spec documents the new meaning instead and proposes a validator warning under the
  Integration Checklist.
- R-2 is CLOSED by unification, not documented: the owner chose on 2026-08-02 to make the
  Router-LSA B-bit determination the single ABR producer. Any residual delay is that one
  producer's own update latency, which is the same latency peers already observe in the
  advertised B-bit, so Ze cannot advertise one thing and originate another.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
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
