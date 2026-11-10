# Spec: firewall existing family and owner correctness

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P0 |
| Release reason | Existing ARP IP-header misreads and cross-producer table collisions can defeat accepted firewall and VRRP protection. |
| Scope | config |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-10 |

## Task

Correct accepted family-arp matches that read IP offsets and prevent operator/producer table-name collisions. These obligations exist without the new ARP/ND leaves. AC-16 must be demonstrated using the current operator match surface; adding the six new leaves is not a prerequisite. Preserve the source decisions D-3 and its journal evidence, without assuming that dated observations are a fresh reproduction.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

The owner is away; a fix spec's design needs no approval (`ai/META-DECISIONS.md`, "How agents work").

## Required Reading

### Architecture Docs

- `ai/doors/system.md`, `ai/doors/forwarding.md`, `ai/doors/system-plugins.md`, `ai/doors/access-security.md` (the IKE source guard is a producer), `ai/doors/le-tooling.md` (the `.ci` files).
  -> Constraint: the firewall table registry is process-local; every producer runs in process and calls `RegisterTables` (forwarding door, Boundaries).
  -> Constraint: no central list of producer names in the firewall component; each producer declares its own names from its own `register*.go` `init()` (system door, "everything registers").
- `docs/architecture/firewall/table-ownership-and-shutdown-flush.md`, Rule 2.
  -> Decision: Rule 2's producer table is wrong (8 rows, 12 producers exist); it is corrected in the same change, and a Rule 2b states the reservation.
- `docs/guide/firewall.md`, Table Families.
- `docs/architecture/core-design.md`, the firewall table registry and verify-time validation (the `// Design:` of `registry.go` and `validate.go`).
  -> Constraint: verify refuses what the backend cannot apply exactly; the refusal belongs at verify, not at apply.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/validate.go` - `ValidateTables`, `validateMatch`, `validateSetFieldMatch`, `validateSetFamilyCompat`
- [ ] `internal/component/firewall/registry.go` - `RegisterTables`, `ApplyAll`, `mergeSameNameTables`
- [ ] `internal/component/firewall/engine.go` - `parseAndVerifyFirewallSections`, the configure closure, `registerConfigTables`
- [ ] `internal/component/firewall/register.go` - the firewall plugin `init()`
- [ ] `internal/component/firewall/config.go` - `parseFromBlock`, `irrSetMatch`, `domainSetMatch`, `tableNamePrefix`
- [ ] `internal/plugins/vrrp/ownerfilter.go` - `ownerFilterTable`, `ownerARPReplyTerm`
- [ ] `internal/plugins/vrrp/owner_answer_integration_linux_test.go` - the QEMU owner-answer proof

**Behavior to preserve:** IRR and domain set contributions into operator tables; every existing refusal text; producer registrations under their own owner.

**Behavior to change:**

- `internal/component/firewall/validate.go` `validateMatch`: no family check for `MatchSourceAddress`, `MatchDestinationAddress`, `MatchSourcePort`, `MatchDestinationPort`, `MatchProtocol`, `MatchConnState`, `MatchConnMark`, `MatchTCPFlags`; a port set match in family arp passes `validateSetFieldMatch`. An arp table therefore accepts matches the nft lowering reads at IPv4 offsets of the ARP header (journal `guard-added-to-one-half-of-a-pair.md` 2026-09-28). `icmp-type`, `icmpv6-type`, `dscp` already refuse.
- `internal/component/firewall/registry.go` `RegisterTables`: keyed by owner, checks only the `ze_` prefix. `ApplyAll` -> `mergeSameNameTables` merges same-name, same-family tables of different owners. An operator table `vrrp_owner_nd` becomes `ze_vrrp_owner_nd` and is merged into the VRRP owner table (journal `two-owners-share-one-name.md` 2026-09-28).
- `internal/component/firewall/engine.go` `parseAndVerifyFirewallSections` (verify, `ze config validate`) and the configure path call `ValidateTables(cfg.Tables)`; nothing compares operator names with producer names. `registerConfigTables` registers owner `firewall` and `conntrackHelperOwner`.
- `internal/component/firewall/register.go` `init()` registers the firewall plugin.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where | How this spec uses it |
|-----------|-------|------------------------|
| `RegisterTables(owner, tables)` prefix refusal | `registry.go` | Extended: also refuses a name another owner reserved, before any mutation |
| `mergeSameNameTables` | `registry.go` | Kept: IRR and domain contribute sets into an operator table's name, which is unreserved, so the merge they rely on is unchanged |
| `ValidateTables` / `validateMatch` family guards (`MatchDSCP`, `MatchICMPType`) | `validate.go` | Extended with one family-arp guard ahead of the type switch; `ValidateTables` also judges helper tables, so the reserved-name refusal goes in the engine's operator path, not here |
| Producer `init()` registration in `register*.go` | each producer | Each producer calls `firewall.MustReserveTableNames(owner, names...)` with its own constants; no list in the firewall component |
| `StripZeTablePrefix` | firewall | Names the operator's table in the refusal |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

Operator config `firewall { table <name> { family arp; ... } }` -> `ze config validate` / reload verify -> `parseAndVerifyFirewallSections`; configure -> the `OnConfigure` closure in `engine.go`. Producers: `RegisterTables` from each owner.

### Transformation Path

Config tree -> `ParseFirewallConfig` (`ze_` + name) -> `refuseReservedTableNames` -> `ValidateTables` (`validateMatch` arp guard) -> backend verifier -> `registerConfigTables` -> `RegisterTables("firewall")` (reservation check) -> `ApplyAll`.

### Boundaries Crossed

None new. Reservations are package state filled at `init()`; all producers run in process (forwarding door).

### Changed-Wire Census

No wire change. Kernel state changes only by refusing configs that previously merged into a producer table.

### Integration Points

`internal/component/firewall/reservation.go` (new); `register*.go` of vrrp, flowspec-firewall, copp, policyroute, ddos/local, anomaly/shape, gtsm, ike/engine, firewall/plugins/irr, firewall (conntrack helpers).

### Architectural Verification

No producer name appears in `internal/component/firewall` except its own conntrack-helper tables. Removing a producer package removes its reservation.

## Risks & Assumptions

### Assumptions

- A compiled-out producer reserves nothing; no table of its name can then exist, so no collision is possible.

### Risks

- A test in a producer package registering a reserved name under another owner would now fail; checked by the scoped package runs.

## Blast Radius

firewall component, its engine verify/configure, the 10 producer `register*.go` files, two docs.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` with an arp table using `source-address` | → | `validateMatch` arp guard | `test/parse/firewall-arp-ip-header-match-rejected.ci` |
| `ze config validate` with a table named `vrrp_owner_arp` | → | `refuseReservedTableNames` in `parseAndVerifyFirewallSections` | `test/parse/firewall-reserved-table-name-rejected.ci` |
| Kernel, VRRP owner table beside an operator arp accept table | → | `ownerFilterTables` through `RegisterTables` and the nft backend | `TestVRRPOwnerSurvivesOperatorARPTable` (QEMU) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|---|---|---|---|
| AC-12 | In a table of family arp, any of source-address, destination-address, source-port, destination-port, protocol, connection-state, connection-mark, tcp-flags, a set match, an IRR or domain-group match | Refused at verify with an error naming the leaf and "family arp carries no IP header"; icmp-type, icmpv6-type and dscp keep their existing refusals | Matches that read no IP header stay accepted in arp: input/output interface, mark, arp-operation, arp-sender-address, because they read meta or ARP fields |
| AC-15 | An operator table whose kernel name equals a name another producer reserved (`vrrp_owner_arp`, `vrrp_owner_nd`, `vrrp` among them), in any family, whether or not that producer is active | Refused at verify with an error naming the table, its kernel name and the owner that holds it; nothing is registered, so the VRRP owner table in the kernel keeps its terms | Every unreserved name stays accepted, so operator tables keep working |
| AC-16 | An operator arp table with an accept term for ARP replies from a VRRP-owned address, beside the VRRP owner table | The capture on the LAN sees only the virtual MAC: the VRRP owner drop still removes the parent's physical-MAC reply (drop in any base chain is final) | The operator's table is installed and its accept runs; the control phase without the owner filter shows the physical MAC |
| AC-22 | Any owner calls `RegisterTables` with a table name another owner reserved | Returns an error naming both owners; the registry keeps the reserving owner's tables unchanged. IRR and domain registering sets into an operator table's name stay accepted | The reserving owner's own registrations and every unreserved name stay accepted |

## End-to-End User Stories

- An operator writes `source-address` in an arp table: `ze config validate` refuses it naming the leaf and the reason, and tells them to move the term to an ip, ip6 or inet table.
- An operator names a table `vrrp_owner_arp`: verify refuses it naming the VRRP owner, and the VRRP filter stays intact.

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| `TestValidateARPFamilyRefusesIPHeaderMatches` | `internal/component/firewall/validate_test.go` | AC-12 |
| `TestRegisterTablesRefusesAReservedName` | `internal/component/firewall/reservation_test.go` | AC-22 |
| `TestRegisterTablesAcceptsSetsIntoAnUnreservedName` | same | AC-22 |
| `TestOperatorTableRefusedOnAReservedName` | same | AC-15 |
| `TestMustReserveTableNamesPanicsOnASecondOwner` | same | AC-22 |
| `TestOwnerTablesAreReserved` | `internal/plugins/vrrp/reservation_test.go` | AC-15 |

### Boundary Tests (numeric inputs)

None: no numeric input.

### Functional Tests

| Test | AC |
|------|----|
| `test/parse/firewall-arp-ip-header-match-rejected.ci` | AC-12 |
| `test/parse/firewall-reserved-table-name-rejected.ci` | AC-15 |

### Interop Tests (Scope: protocol)

Not owed: config-only change, no protocol peer. AC-16 runs against the real kernel in QEMU: `TestVRRPOwnerSurvivesOperatorARPTable` (`internal/plugins/vrrp/owner_operator_arp_integration_linux_test.go`).

## Files to Modify

- `internal/component/firewall/validate.go` - family-arp refusal (AC-12); stale comment on `validateSetFamilyCompat`.
- `internal/component/firewall/registry.go` - cross-owner `RegisterTables` refusal (AC-22).
- `internal/component/firewall/engine.go` - operator reserved-name refusal at verify and configure (AC-15).
- `internal/component/firewall/register.go` - reserves the conntrack-helper tables.
- `register*.go` of `internal/plugins/vrrp`, `internal/plugins/flowspec-firewall`, `internal/plugins/copp`, `internal/plugins/policyroute`, `internal/plugins/ddos/local`, `internal/plugins/anomaly/shape`, `internal/component/gtsm`, `internal/component/ike/engine`, `internal/component/firewall/plugins/irr` - reserve their own names; IRR reserves only `ze_irr_iface`, domain reserves nothing.
- `docs/architecture/firewall/table-ownership-and-shutdown-flush.md`, `docs/guide/firewall.md`.
- `plan/journal/two-owners-share-one-name.md`, `plan/journal/guard-added-to-one-half-of-a-pair.md` - close the owned rows.

## Files to Create

- `internal/component/firewall/reservation.go`, `internal/component/firewall/reservation_test.go`
- `internal/plugins/vrrp/owner_operator_arp_integration_linux_test.go`
- `test/parse/firewall-arp-ip-header-match-rejected.ci`, `test/parse/firewall-reserved-table-name-rejected.ci`

### Integration Checklist

- [ ] every `RegisterTables` producer reserves its names
- [ ] operator refusal runs on verify and configure

### Documentation Update Checklist (BLOCKING)

- [ ] ownership page Rule 2 table and Rule 2b
- [ ] guide Table Families

## Implementation Steps

1. AC-12 guard, unit test, `.ci`; commit.
2. Reservation API, registry refusal, operator refusal, producer reservations, tests, `.ci`, docs; commit.
3. AC-16 QEMU test; commit.
4. Review, close journal rows, closure.

### Critical Review Checklist

- [ ] every refusal names what, why and the fix (`ai/rules/user-facing-errors.md`)

### Deliverables Checklist

- [ ] AC-12, AC-15, AC-16, AC-22 each have a test observed red without the fix

### Security Review Checklist

- [ ] no operator config can merge into a daemon-owned table

### Failure Routing

Red outside this change: name it in the commit body.

## Design Insights

The collision is a name-ownership question, so it is answered where names are declared: each producer reserves its names, and both the registry and the operator verify path consult the reservation (paired check).

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `two-owners-share-one-name.md` | 2026-09-28 | operator `vrrp_owner_nd` merged into the VRRP owner table |
| `guard-added-to-one-half-of-a-pair.md` | 2026-09-28 | literal address match accepted in family arp |

## Key Design Decisions

- D-1: The arp guard runs ahead of the type switch, on a predicate naming each IP-header match's leaf, so every IP-header match gets the same refusal text.
- D-2: Reservation is by name, not by owner: an unreserved name stays shareable, which keeps IRR and domain set contributions into operator tables.
- D-3 (from the source spec): a producer table name is owned by its producer; an operator cannot reuse it in any family.
- D-4: The operator refusal is in the engine's operator path, not `ValidateTables`, because `ValidateTables` also judges the conntrack-helper tables, which hold reserved names legitimately.

## Known Limitations

None.

## Implementation Ownership

| Scope | Current owner |
|---|---|
| New leaves: AC-1..11, AC-13..14, AC-17..21; all feature design outside the release correction. | `plan/post-release/features/spec-firewall-arp-nd-matches.md` |
| Existing match-family correctness and reserved-table ownership: AC-12, AC-15, AC-16, AC-22. | `plan/release/fixes/spec-firewall-existing-family-and-owner-correctness.md` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| New leaves: AC-1..11, AC-13..14, AC-17..21; all feature design outside the release correction | Split out as post-release feature work before this spec started | `plan/post-release/features/spec-firewall-arp-nd-matches.md` |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-12, AC-15, AC-16, AC-22 all demonstrated
- [ ] Wiring Test table complete
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests of the changed packages, `./le test functional parse` for the two `.ci` files, `./le test qemu` for `TestVRRPOwnerSurvivesOperatorARPTable`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + journal rows
- [ ] **Commit B:** remove the spec

## Implementation Summary

### What Was Implemented
- AC-12 (6da2c99b2c): `validateMatch` (`internal/component/firewall/validate.go`) refuses, in family arp and ahead of the type switch, every match `ipHeaderMatchLeaf` names, with "<leaf> match reads the IP header, and family arp carries no IP header; move the term to a table of family ip, ip6 or inet".
- AC-15 and AC-22 (29c4f94f8e): `internal/component/firewall/reservation.go` (`MustReserveTableNames`, `refuseForeignReservations`, `refuseReservedTableNames`); `RegisterTables` refuses a name another owner reserved before it changes the registry (`registry.go`); `parseAndVerifyFirewallSections` and the configure closure refuse an operator table on a reserved kernel name (`engine.go`); every `RegisterTables` producer reserves its own names from its own `register_tables.go` `init()` (vrrp, flowspec-firewall, copp, policyroute, ddos/local, anomaly/shape, gtsm, ike/engine, firewall/plugins/irr), and the firewall engine reserves its conntrack-helper tables from `register.go`.
- AC-16 (29c4f94f8e): `TestVRRPOwnerSurvivesOperatorARPTable` (QEMU) proves an operator arp accept beside the VRRP owner table does not override the owner drop.
- Review round-2 fix (the wording commit after 3853a00b92, see Review Gate): the two reservation refusals and the firewall guide say two owners in one kernel table "would overwrite each other's chains", true both for registry owners merged by `ApplyAll` and for `ze_iface_rpf`, whose two writers delete and recreate the table; `docs/features/interfaces.md` names the reserved `iface_rpf`.
- Review round-1 fix (3853a00b92, see Review Gate): the IPv6 reverse-path table `ze_iface_rpf`, written directly over netlink by `ReplaceIPv6RPF` (`internal/plugins/iface/netlink/rpf_linux.go`) and never registered, is reserved for owner `iface-rpf` from `internal/plugins/iface/netlink/register_tables.go`, which now holds `rpfTableName` untagged.

### Bugs Found/Fixed
- Family-arp IP-header misreads (journal `guard-added-to-one-half-of-a-pair.md`): covered by `TestValidateARPFamilyRefusesIPHeaderMatches` and `test/parse/firewall-arp-ip-header-match-rejected.ci`.
- Operator table merged into a producer table (journal `two-owners-share-one-name.md`): covered by the reservation tests and `test/parse/firewall-reserved-table-name-rejected.ci`.
- `ze_iface_rpf` unreserved (review round 1 ISSUE): covered by `TestRPFTableIsReserved` and `test/parse/firewall-iface-rpf-table-name-rejected.ci`.

### Documentation Updates
- `docs/architecture/firewall/table-ownership-and-shutdown-flush.md`: the Rule 2 producer table lists every producer, with the `iface-rpf` row and a paragraph on the writer outside the registry; Rule 2b states the reservation. Anchors: `reservation.go -- MustReserveTableNames, refuseForeignReservations, refuseReservedTableNames`, `engine.go -- parseAndVerifyFirewallSections`, `iface/netlink/register_tables.go -- rpfTableName, rpfTableOwner`, `iface/netlink/rpf_linux.go -- ReplaceIPv6RPF`.
- `docs/guide/firewall.md`, Table Families: the arp refusal (anchor `validate.go -- validateMatch, ipHeaderMatchLeaf`) and the reserved-name refusal, which names `iface_rpf` and says the reserved set depends on the build's features (anchor `reservation.go -- refuseReservedTableNames`).
- `./le doc check all`: owed to the main thread; the closure agent runs no checks.

### Deviations from Plan
- Round 1 found a twelfth table writer outside the registry (`ze_iface_rpf`). It is reserved the same way, from its owning package, with no list in the firewall component. Files to Modify gains `internal/plugins/iface/netlink/rpf_linux.go`; Files to Create gains `internal/plugins/iface/netlink/register_tables.go`, `internal/plugins/iface/netlink/reservation_test.go` and `test/parse/firewall-iface-rpf-table-name-rejected.ci`.
- `internal/plugins/flowspec-firewall/state_test.go` registered owner `flowspec-prefix-test` on the reserved `ze_flowspec`; it now uses the real owner `flowspec`. An independent `ze-test-judge` approved the change.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The producer census for Rule 2b was taken from `RegisterTables` callers only | `ReplaceIPv6RPF` writes a `ze_` table over netlink without the registry, so it had no reservation | Independent review round 1 | Reserved from `iface/netlink/register_tables.go`. A grep of every `google/nftables` importer under `internal/` finds no other direct writer of a `ze_` name: flowexport's `ze-conntrack` uses a hyphen, `support/collect_linux.go` only reads, the rest are tests and fixtures |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Correct accepted family-arp matches that read IP offsets | Done | `internal/component/firewall/validate.go` `validateMatch` | AC-12 |
| Prevent operator/producer table-name collisions | Done | `internal/component/firewall/reservation.go`; `registry.go` `RegisterTables`; `engine.go` `parseAndVerifyFirewallSections` and the configure closure | AC-15, AC-22, and `ze_iface_rpf` |
| AC-16 on the current operator match surface | Done | `internal/plugins/vrrp/owner_operator_arp_integration_linux_test.go` `TestVRRPOwnerSurvivesOperatorARPTable` | no new leaf used |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-12 | Done | `TestValidateARPFamilyRefusesIPHeaderMatches`, `TestValidateARPFamilyAcceptsMetadataMatches`, `test/parse/firewall-arp-ip-header-match-rejected.ci` | survivors input-interface and mark proven accepted |
| AC-15 | Done | `TestOperatorTableRefusedOnAReservedName`, `TestOwnerTablesAreReserved`, `TestRPFTableIsReserved`, `test/parse/firewall-reserved-table-name-rejected.ci`, `test/parse/firewall-iface-rpf-table-name-rejected.ci` | |
| AC-16 | Done | `TestVRRPOwnerSurvivesOperatorARPTable` | its control phase is the discrimination |
| AC-22 | Done | `TestRegisterTablesRefusesAReservedName`, `TestRegisterTablesAcceptsSetsIntoAnUnreservedName`, `TestMustReserveTableNamesPanicsOnASecondOwner`, `TestConntrackHelperTablesAreReserved` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestValidateARPFamilyRefusesIPHeaderMatches` | Done | `internal/component/firewall/validate_test.go` | |
| `TestValidateARPFamilyAcceptsMetadataMatches` | Done | `internal/component/firewall/validate_test.go` | added by round 1, NOTE 1 |
| `TestRegisterTablesRefusesAReservedName` | Done | `internal/component/firewall/reservation_test.go` | |
| `TestRegisterTablesAcceptsSetsIntoAnUnreservedName` | Done | `internal/component/firewall/reservation_test.go` | |
| `TestOperatorTableRefusedOnAReservedName` | Done | `internal/component/firewall/reservation_test.go` | |
| `TestMustReserveTableNamesPanicsOnASecondOwner` | Done | `internal/component/firewall/reservation_test.go` | |
| `TestOwnerTablesAreReserved` | Done | `internal/plugins/vrrp/reservation_test.go` | |
| `TestRPFTableIsReserved` | Done | `internal/plugins/iface/netlink/reservation_test.go` | added by round 1 |
| `test/parse/firewall-arp-ip-header-match-rejected.ci` | Done | `test/parse/` | |
| `test/parse/firewall-reserved-table-name-rejected.ci` | Done | `test/parse/` | |
| `test/parse/firewall-iface-rpf-table-name-rejected.ci` | Done | `test/parse/` | added by round 1 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/firewall/validate.go` | Done | 6da2c99b2c |
| `internal/component/firewall/registry.go`, `engine.go`, `register.go`, `reservation.go`, `reservation_test.go` | Done | 29c4f94f8e; the `reservation.go` comment is reworded in 3853a00b92, and its refusal text and `MustReserveTableNames` comment in the round-2 wording commit |
| producer `register_tables.go` files | Done | 29c4f94f8e, plus `internal/plugins/iface/netlink/register_tables.go` in 3853a00b92 |
| `internal/plugins/vrrp/owner_operator_arp_integration_linux_test.go` | Done | 29c4f94f8e |
| both docs | Done | 6da2c99b2c, 29c4f94f8e, 3853a00b92, and the round-2 wording commit (which also edits `docs/features/interfaces.md`) |
| both journal rows | Done | closed with `fixed:` cells in 6da2c99b2c and 29c4f94f8e |

### Audit Summary
- **Total items:** 4 AC, 3 task requirements, 11 tests, 6 file groups
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** the `ze_iface_rpf` reservation and the flowspec test owner (Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An arp table no longer accepts a match the kernel reads at IPv4 offsets of the ARP header | functional (`ze config validate`) + unit | `test/parse/firewall-arp-ip-header-match-rejected.ci` passes, and with the guard removed it went red: "expected exit code 1, got 0 ... configuration valid"; `TestValidateARPFamilyRefusesIPHeaderMatches` red without the guard |
| An operator table cannot share a kernel table with a producer | functional (`ze config validate`) + unit | `test/parse/firewall-reserved-table-name-rejected.ci` passes, red without `refuseReservedTableNames` ("expected exit code 1, got 0"); `TestOperatorTableRefusedOnAReservedName` red without it ("expected a refusal for a reserved operator table name, got nil"); for the netlink-written table, `test/parse/firewall-iface-rpf-table-name-rejected.ci` and `TestRPFTableIsReserved` pass after 3853a00b92 (main-thread run: full-tag tests of `internal/plugins/iface/netlink`, `internal/component/firewall` and `internal/plugins/vrrp`, and all ten firewall parse `.ci` files), and both go red with the `ze_iface_rpf` reservation removed (main-thread red proof) |
| A producer cannot register another owner's table | unit | `TestRegisterTablesRefusesAReservedName` red without `refuseForeignReservations` ("expected a refusal for a name another owner reserved, got nil") |
| The VRRP owner filter survives an operator arp accept, on the current match surface | kernel integration (QEMU, real nft) | `TestVRRPOwnerSurvivesOperatorARPTable` PASS (2.20s); its control phase, operator table alone, sees the physical MAC, so the owner-drop assertion discriminates |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/firewall-existing-family-and-owner-correctness-f90e7822-ecbf-462e-ac37-c494b4c2c988.md` |
| `./le spec review check` | owed to the main thread over the closure commit's files, after the round-3 artifact is recorded |
| Rounds | 3, all by independent reviewers |
| Reviewer lenses used | round 1: independent reviewer, covering logic, wiring, the producer census and docs; round 2: independent reviewer over 3853a00b92, confirming each round-1 fix and re-reading the refusal text, the guide and the ownership page; round 3: independent reviewer over 7d2ecf5c70, confirming the wording fix and the interfaces page |

### Run 1 (independent reviewer): 0 BLOCKER, 1 ISSUE, 4 NOTE
- NOTE 1: no accept rows proved input-interface and mark stay accepted in family arp. Fixed: `TestValidateARPFamilyAcceptsMetadataMatches`.
- NOTE 2: the guide did not say the reserved set depends on the build's features. Fixed: `docs/guide/firewall.md`, Table Families.
- NOTE 3: the `tableReservations` comment said "its own register.go" and "no table of its name can then exist". Fixed: it names `register_tables.go` and says no producer table remains to merge with; Rule 2b carries the same correction.
- NOTE 4: flowexport's `ze-conntrack` is a direct nft writer but safe (hyphen, no `ze_` prefix). Confirmed by grep; no change.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `ze_iface_rpf` is written directly over netlink and never reserved: an operator ip6 table `iface_rpf` shares it, the nft backend's apply deletes and recreates it (wiping the RPF chain and the strict and loose sets), and `ReplaceIPv6RPF` then wipes the operator's rules | `internal/plugins/iface/netlink/rpf_linux.go` `ReplaceIPv6RPF`, `rpfTableName` | `internal/plugins/iface/netlink/register_tables.go` reserves it for `iface-rpf`; `TestRPFTableIsReserved`; `test/parse/firewall-iface-rpf-table-name-rejected.ci`; the Rule 2 row |

### Run 2 (independent reviewer, over 3853a00b92): 0 BLOCKER, 1 ISSUE, 2 NOTE
All four round-1 findings confirmed fixed. An earlier round 2 was a re-read by the agent that wrote the round-1 fixes; it is not independent and is superseded by this run.
- ISSUE: `refuseForeignReservations`, `refuseReservedTableNames` (`internal/component/firewall/reservation.go`) and `docs/guide/firewall.md`, Table Families, said "two owners in one kernel table would merge their chains", false for `ze_iface_rpf`: `queueIPv6RPF` (`internal/plugins/iface/netlink/rpf_linux.go`) adds and deletes the table and the nft backend's apply deletes and recreates it, so each writer wipes the other. Fixed in the wording commit after 3853a00b92: both refusals and the guide say "would overwrite each other's chains", and the `MustReserveTableNames` comment names both outcomes. No test asserts the clause.
- NOTE 1: an operator table cannot be named `iface_rpf`, and the interface pages did not say so. `docs/features/interfaces.md`, Reverse-path check, now says so in the wording commit. `docs/guide/configuration.md` is left as is: it carries another session's uncommitted hunk.
- NOTE 2: the ddos-local owner string in `docs/architecture/firewall/table-ownership-and-shutdown-flush.md`, Rule 2 table. Checked: the owner is `ze_ddos-local` (`internal/plugins/ddos/local/register_tables.go` reserves under `tableName`, `responder.go` sets it to `ze_ddos-local`), as the page says. No change.

### Run 3 (independent reviewer, over 7d2ecf5c70): 0 BLOCKER, 0 ISSUE, 0 NOTE
The round-2 ISSUE is confirmed fixed. `queueIPv6RPF` adds, deletes and re-adds `ze_iface_rpf`, and the nft backend's `Apply` deletes every desired-named `ze_` table and recreates it, so "would overwrite each other's chains" holds for both the registry case and the out-of-registry case. No reservation refusal or guide still says "merge their chains". The `docs/features/interfaces.md` paragraph matches `register_tables.go` and `refuseReservedTableNames`.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/firewall/reservation.go`, `reservation_test.go` | yes | `git show --stat 29c4f94f8e` |
| `internal/plugins/vrrp/owner_operator_arp_integration_linux_test.go` | yes | `git show --stat 29c4f94f8e` |
| `test/parse/firewall-arp-ip-header-match-rejected.ci` | yes | `git show --stat 6da2c99b2c` |
| `test/parse/firewall-reserved-table-name-rejected.ci` | yes | `git show --stat 29c4f94f8e` |
| `internal/plugins/iface/netlink/register_tables.go`, `reservation_test.go`, `test/parse/firewall-iface-rpf-table-name-rejected.ci` | yes | `git show --stat 3853a00b92` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-12 | arp guard ahead of the type switch | `validateMatch`: `if leaf, readsIP := ipHeaderMatchLeaf(m); readsIP {` before `switch v := m.(type)` |
| AC-15 | operator refusal on verify and configure | `parseAndVerifyFirewallSections` and the configure closure in `engine.go` each call `refuseReservedTableNames(cfg.Tables)` before `ValidateTables` |
| AC-16 | owner drop survives operator accept | `--- PASS: TestVRRPOwnerSurvivesOperatorARPTable (2.20s)` |
| AC-22 | registry refusal before the registry changes | `RegisterTables` calls `refuseForeignReservations(owner, tables)` ahead of `tableRegistry.mu.Lock()` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze config validate` with an arp table using `source-address` | `test/parse/firewall-arp-ip-header-match-rejected.ci` | read: `exec=ze config validate -`, expects exit 1 and the arp refusal text |
| `ze config validate` with a table named `vrrp_owner_arp` | `test/parse/firewall-reserved-table-name-rejected.ci` | read: expects exit 1 and `kernel name "ze_vrrp_owner_arp" is reserved by owner "vrrp-owner"` |
| `ze config validate` with a table named `iface_rpf` | `test/parse/firewall-iface-rpf-table-name-rejected.ci` | read: expects exit 1 and `kernel name "ze_iface_rpf" is reserved by owner "iface-rpf"` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 (a compiled-out producer reserves nothing) | confirmed | every reservation is an `init()` in the producer's own package (grep of `MustReserveTableNames` callers); a package the build does not link runs no `init()` and installs no table |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Rule 2 table lists every producer, `iface-rpf` included | grep of `MustReserveTableNames` callers matches the rows; firewall-domain reserves nothing, as the page says | yes |
| Guide: the arp refusal text and the reserved-name refusal | `validateMatch` and `refuseReservedTableNames` message text | yes |
| Feature list, CLI, API, YANG, wire format, RFC | no YANG, command or wire change | No update needed |

## Provenance and retained evidence

Split from `plan/post-release/features/spec-firewall-arp-nd-matches.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/post-release/features/spec-firewall-arp-nd-matches.md`. This child claims no new test result or completion.
