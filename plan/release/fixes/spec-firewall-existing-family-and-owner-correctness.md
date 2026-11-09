# Spec: firewall existing family and owner correctness

| Field | Value |
|-------|-------|
| Status | ready |
| Priority | P0 |
| Release reason | Existing ARP IP-header misreads and cross-producer table collisions can defeat accepted firewall and VRRP protection. |
| Scope | config |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-09 |

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

| Scope | Current owner |
|---|---|
| New leaves: AC-1..11, AC-13..14, AC-17..21; all feature design outside the release correction. | `plan/post-release/features/spec-firewall-arp-nd-matches.md` |

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

## Review Gate

Pending.

## Provenance and retained evidence

Split from `plan/post-release/features/spec-firewall-arp-nd-matches.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/post-release/features/spec-firewall-arp-nd-matches.md`. This child claims no new test result or completion.
