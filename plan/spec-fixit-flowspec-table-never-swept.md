# Spec: fixit-flowspec-table-never-swept

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 5/5 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**A FlowSpec rule a peer withdraws is enforced by the kernel forever, and the
rules of a rule that stays accumulate on every reconcile.**

Two producers were read on 2026-08-22 and both hold today.

`shouldDeleteTable` (`internal/plugins/firewall/nft/backend_linux.go`) returns
false for any kernel table whose name does not start with `ze_`. That prefix is
how the nft backend decides which tables an `Apply` owns, and the comment above
the sweep says why: an unknown table may belong to another producer and must not
be swept by prefix alone.

The FlowSpec plugin names its table `flowspec` (`internal/plugins/flowspec-firewall/state.go`,
the `firewall.Table` literal returned for the built chains). It carries no `ze_`
prefix, so the sweep never considers it.

Two consequences follow, both reachable from the wire.

1. **A withdraw leaves the table installed.** When a peer withdraws its last
   FlowSpec route, `applyRules` registers no table, `ApplyAll`
   (`internal/component/firewall/registry.go`) hands the backend a desired set
   without `flowspec`, and the sweep declines to delete it. The kernel keeps
   dropping the traffic the withdrawn rule named.
2. **Rules accumulate on every reconcile.** `Apply` deletes a desired table
   before `applyTable` recreates it, so a table that survives the sweep is added
   to rather than replaced. Measured on 2026-08-22: two identical rules after two
   reconciles.

**Every other firewall owner carries the prefix**, so this is one producer out of
step rather than a convention nobody follows: `ze_copp`
(`internal/plugins/copp/translate.go`), `ze_pr`
(`internal/plugins/policyroute/translate.go`), `ze_irr_iface`
(`internal/component/firewall/plugins/irr/sets.go`), `ze_ddos-local`
(`internal/plugins/ddos/local/responder.go`), and the config-derived tables the
firewall component prefixes with `tableNamePrefix` (`internal/component/firewall/config.go`).

**A rename alone is not the whole fix.** A router running today holds a kernel
table named `flowspec`. After an upgrade the plugin creates `ze_flowspec`, and
the sweep still cannot see the unprefixed one, so the stale table keeps enforcing
its rules for the life of the box.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - "Firewall reconcile concurrency" and table ownership
  → Constraint: `ApplyAll` merges every owner's tables and commits them in one flush. Ownership is a property of the registry, and the backend's prefix test is a proxy for it.
- [ ] `docs/guide/flowspec-protected-router.md` - what an operator is told a FlowSpec route does
  → Constraint: the page states that a withdrawn route stops being enforced. Today it does not.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc8955.md` - FlowSpec route withdrawal
  → Constraint: a FlowSpec NLRI is a route. Withdrawing it is an ordinary BGP withdraw, so the traffic action it carried must stop.

**Key insights:**
- The defect is one unprefixed name, and the blast radius is a rule that outlives the route that asked for it.
- The prefix is a spelling standing in for ownership the registry already knows. Fixing the spelling fixes this instance; deriving ownership from the registry would remove the class.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/firewall/nft/backend_linux.go` - `zeTablePrefix`, `Apply`'s sweep loop, and `shouldDeleteTable`, which returns false without the prefix
- [ ] `internal/plugins/flowspec-firewall/state.go` - the `firewall.Table` literal naming the table `flowspec`
- [ ] `internal/component/firewall/registry.go` - `ApplyAll` merges every owner's tables and hands them to the backend as one set
- [ ] `internal/component/firewall/config.go` - `tableNamePrefix`, the convention every other owner follows
- [ ] `internal/component/firewall/accessor.go` - `StripZeTablePrefix`, which the CLI uses to render a table name back to an operator

**Behavior to preserve:**
- The sweep keeps refusing to delete an unknown `ze_*` table it did not apply. That guard exists so two Ze producers cannot destroy each other's tables.
- `show firewall ruleset` keeps rendering the operator-facing name. `StripZeTablePrefix` already removes the prefix, so the rename must not change what an operator types or reads.
- A FlowSpec component naming no value keeps producing no match.

**Behavior to change:**
- The FlowSpec table is swept like every other owner's, so a withdraw removes it and a reconcile replaces rather than appends.
- A kernel table left by an older build under the unprefixed name is removed once, so an upgrade does not strand it.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A peer withdraws a FlowSpec route, or announces one that triggers a second reconcile.
- Format at entry: a BGP UPDATE carrying withdrawn FlowSpec NLRI, delivered to the plugin as JSON.

### Transformation Path
1. `handleUpdate` (`internal/plugins/flowspec-firewall/engine.go`) receives the withdraw.
2. `applyRules` registers the remaining tables with the firewall registry under the owner name `flowspec`.
3. `ApplyAll` (`internal/component/firewall/registry.go`) merges every owner's tables into one desired set.
4. `Apply` (`internal/plugins/firewall/nft/backend_linux.go`) lists the kernel's tables and calls `shouldDeleteTable` on each -- the defect is here.
5. `applyTable` recreates the desired tables and one `Flush` commits.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Peer ↔ flowspec plugin | withdrawn NLRI in an UPDATE | No |
| Plugin ↔ firewall registry | `RegisterTables` with owner `flowspec` | No |
| Backend ↔ kernel | one netlink `Flush` per reconcile | No |

### Integration Points
- `shouldDeleteTable` - the single decision about which kernel tables an `Apply` owns
- `StripZeTablePrefix` (`internal/component/firewall/accessor.go`) - what keeps the rename invisible to an operator

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The withdraw still travels plugin to `RegisterTables` to `ApplyAll` to `Backend.Apply`. The fix changes what the sweep in `Apply` (`internal/plugins/firewall/nft/backend_linux.go`) recognizes; it adds no path around the registry |
| No unintended coupling (components stay isolated) | Yes | The ledger lives in the shared component (`internal/component/firewall/legacy_tables.go`) and names its producers in comments only. No plugin imports another, and `internal/plugins/ddos/local/register.go` reaches the component through the same `firewall` import `responder.go` already had |
| No duplicated functionality (extends existing, does not recreate) | Yes | One decision (`IsLegacyTable`), one pre-filter (`IsLegacyTableName`), one gate (`LegacySweepPending`). No second ownership mechanism: the prefix test in `shouldDeleteTable` is untouched |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Not a wire path. The one added allocation is the chain-name slice in `isLegacyTable`, built with `make([]string, 0, len(chains))` and only for a name the pre-filter already matched |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | `legacyTables` is a data map, not a switch, and it is written to be deleted. `RegisterTables` gained a name check, not a per-producer branch: it names no plugin. `TestEveryProducerThatShippedABareNameIsInTheLedger` pins the map's population so an entry cannot be added or dropped silently |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `flowspec` is the only firewall table name in the tree without the `ze_` prefix | every `firewall.Table` producer, enumerated 2026-08-22: `ze_copp`, `ze_pr`, `ze_irr_iface`, `ze_ddos-local`, and the config-derived tables | the fix is one name and the class stays open for the others | the enumeration, re-run at implementation time | **broken 2026-08-23**. The re-run over every `firewall.Table` literal found a SECOND bare producer in the tree: `tableNameV4 = "anomaly-shape"` and `tableNameV6 = "anomaly-shape6"` (`internal/plugins/anomaly/shape/match.go`), registered through `registerTables` (`responder.go`, the `firewall.RegisterTables` indirection). AC-4's registry guard would have refused them at runtime and broken the shape responder, so both were renamed in this spec. **Correction, on review 2026-08-24:** the enumeration read the tree at HEAD, and what an upgraded box holds is what SHIPPED. Two more producers shipped bare and were renamed before this spec: `coppTableName = "copp"` (`internal/plugins/copp/translate.go`, commit `0ee364e29`, renamed in `c5273da42`) and `tableName = "ddos-local"` (`internal/plugins/ddos/local/responder.go`, renamed in `1423ad437`). Both are now in `legacyTables`. `git log -S` over the other three names shows one commit each, the one that introduced the prefixed spelling, so four producers is the whole population |
| A-2 | Nothing outside the plugin depends on the literal table name `flowspec` | `StripZeTablePrefix` renders the operator-facing name; the CLI and web page read the stripped form | the rename breaks a CLI argument, a web page, or a `.ci` assertion | `gopls references` plus a grep over `test/`, `docs/` and `internal/component/web/` | **broken 2026-08-23** for tests and docs, confirmed for the CLI and the web. No CLI argument or web page carries the literal: `page_firewall.go` and `nft/cmd_show.go` both render through `StripZeTablePrefix`. **Correction, on review:** the reason first written here was wrong. `handleShowFirewallRuleset` and `collectTables` read `LastApplied()`, the merged set the registry stored, NOT the prefix-filtered kernel readback, so both already rendered `flowspec` and the rename ADDS nothing to `show firewall ruleset`. What the rename changes is the `ListTables()` readers, which DO filter on the prefix: `ze firewall show table flowspec` now finds a table where it used to find none. Three `.ci` assertions and two doc commands did carry it (`flowspec-fw-protocol-sctp.ci`, `flowspec-fw-untranslatable-keeps-others.ci`, `test/interop/scenarios/bgp-flowspec-sctp-gobgp/check.py`, `docs/guide/flowspec-protected-router.md`) and each is updated here |
| A-3 | Removing a legacy unprefixed `flowspec` table cannot destroy another producer's table | the sweep's own comment says an unknown `ze_*` table may belong to another producer; an unprefixed `flowspec` was written by this plugin and by nothing else | a one-time removal deletes a table Ze does not own | a grep for any other writer of a table named `flowspec`, plus the nft table family | **confirmed 2026-08-23**. No other producer registers a table named `flowspec`, `anomaly-shape` or `anomaly-shape6`: the enumeration of `firewall.Table` literals is complete and each name has one writer. **Correction, on review:** the enumeration answers "no OTHER ze producer writes these names", which is a smaller claim than the assumption makes, and it says nothing about software that is not ze. The first fix keyed the removal on name AND family, and that was NOT enough: `flowspec` is an ordinary word, and third-party FlowSpec-to-nftables tooling using it in the inet family would have lost its table to us. `IsLegacyTable` (`internal/component/firewall/legacy_tables.go`) now requires the name, the family AND every chain the kernel table holds to be one the ze producer wrote, and refuses a table carrying no chain at all. `TestApplyLeavesAnotherToolsTableAlone` (`internal/plugins/firewall/nft/integration_linux_test.go`) seeds exactly that collision against a real kernel and requires the table to survive. **Second correction, review 2026-08-24:** the chain test reads NAMES only, and `copp`/`input` and `ddos-local`/`ingress` are more collidable than `flowspec`/`flowspec-fwd`. The removal is therefore gated on `LegacySweepPending` in `Apply` (`internal/plugins/firewall/nft/backend_linux.go`), so only a table already in the kernel when ze started is ever a candidate. The residual is in Known Limitations |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The one-time removal of the legacy table runs on every start and deletes a table an operator created by hand | an operator reports a disappearing nft table | the removal is scoped to the inet family and to a table whose chains match what this plugin writes, and it is logged when it fires |
| R-2 | The rename lands and the accumulation persists because the sweep still misses it for another reason | a second reconcile still shows duplicate rules | the functional test counts rules across two reconciles rather than asserting one is present |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Either a withdrawn FlowSpec rule keeps dropping traffic, or a table an operator owns is deleted |
| How is it reverted? | Single commit revert. The legacy removal is idempotent and leaves nothing behind |
| Who else touches this path? | `spec-fixit-flowspec-protocol-name-drift` owned the translation feeding these tables and CLOSED on 2026-08-22. Its stem is bare because a `plan/` path would cite a file the tree no longer holds |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A peer withdraws its last FlowSpec route | → | `shouldDeleteTable` | `test/plugin/flowspec-fw-withdraw-removes-table.ci` <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) --> |
| A peer announces a FlowSpec route that reconciles twice | → | `Apply`, `applyTable` | `TestApplySweepsEveryOwnersTable` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A peer withdraws its last FlowSpec route | The kernel holds no FlowSpec table and drops none of the traffic the route named |
| AC-2 | A FlowSpec route survives two reconciles | The kernel holds exactly the rules the current route set names, with no duplicates |
| AC-3 | A kernel holds a table named `flowspec` written by an older build, and the daemon starts | The legacy table is removed once, and the removal is logged |
| AC-4 | Any firewall owner registers a table | Its name carries the ownership prefix, and no producer can register one without it |
| AC-5 | An operator runs `show firewall ruleset` and the CLI table commands | The name they type and read is unchanged by this fix |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Receives a FlowSpec rule from a scrubbing provider, then receives its withdrawal | wire → plugin → registry → nft → kernel | `test/plugin/flowspec-fw-withdraw-removes-table.ci` <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) --> |
| 2 | Upgrades a router that was enforcing a FlowSpec rule under the old table name | daemon start → backend → kernel | `test/plugin/flowspec-fw-legacy-table-removed.ci` <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) --> |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestApplySweepsEveryOwnersTable` | `internal/plugins/firewall/nft/integration_linux_test.go` | a desired table from every owner is swept before it is recreated, so two reconciles leave one rule. It drives `Apply` against a real kernel in a netns, which is why it lives beside the other `Apply` tests rather than in a `backend_linux_test.go` the spec first named | written |
| `TestApplyRemovesTheLegacyUnprefixedTableOnce` | `internal/plugins/firewall/nft/integration_linux_test.go` | validates AC-3 at the backend: a seeded unprefixed table is deleted, an operator table beside it is not | written |
| `TestApplyLeavesAnotherToolsTableAlone` | `internal/plugins/firewall/nft/integration_linux_test.go` | a table another tool wrote under a name ze used to use, same inet family, foreign chain, SURVIVES the removal | written |
| `TestApplyWithNoDesiredTablesStillRemovesTheLegacyTable` | `internal/plugins/firewall/nft/integration_linux_test.go` | an EMPTY desired set still removes the stale table, which is the shape the other two proofs step around | written |
| `TestApplyRemovesEveryLegacyTableInItsOwnFamily` | `internal/plugins/firewall/nft/integration_linux_test.go` | every ledger entry is removed against a real kernel in the family its producer wrote: `copp` and `flowspec` in inet, `ddos-local` and `anomaly-shape` in ip, `anomaly-shape6` in ip6. The inet-only proofs never walked the family-raising path | written |
| `TestTheLegacyRemovalRunsOnceInTheProcess` | `internal/plugins/firewall/nft/integration_linux_test.go` | validates AC-3's word "once": the removal runs on the first reconcile and a table written under a legacy name afterwards survives. Drives `firewall.ApplyAll`, which is what clears the gate | written |
| `TestFlowSpecTableCarriesTheOwnershipPrefix` | `internal/plugins/flowspec-firewall/state_test.go` | the table this plugin builds is named so the backend owns it | written |
| `TestEveryRegisteredTableCarriesThePrefix` | `internal/component/firewall/registry_test.go` | validates AC-4: no producer can register an unprefixed table | written |
| `TestIsLegacyTableRequiresNameFamilyAndChainShape` | `internal/component/firewall/legacy_tables_test.go` | the removal is decided on name, family AND every chain the table holds, so a table another tool wrote under the same name is left alone | written |
| `TestIsLegacyTableNameIsOnlyThePreFilter` | `internal/component/firewall/legacy_tables_test.go` | the cheap name/family test is a pre-filter that keeps a netlink round trip off every table, never the decision | written |
| `TestAnEmptyReconcileReachesABackendWhileTheSweepIsPending` | `internal/component/firewall/legacy_tables_test.go` | an empty desired set loads a backend and reaches Apply while the removal is pending, and stops doing so once it has run | written |
| `TestLegacyTableNamesAreNoLongerRegistrable` | `internal/component/firewall/legacy_tables_test.go` | every legacy name is one no current producer can register, so an entry cannot outlive the rename that made it legacy | written |
| `TestEveryProducerThatShippedABareNameIsInTheLedger` | `internal/component/firewall/legacy_tables_test.go` | the ledger holds an entry for each of the four producers that shipped a bare name and for no other, so an upgraded box strands nothing | written |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Reconciles before a rule count is asserted | 1..N | N | N/A | N/A |
| Rules per FlowSpec route after N reconciles | 1 | 1 | N/A | 2 is the defect |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `flowspec-fw-withdraw-removes-table` | `test/plugin/flowspec-fw-withdraw-removes-table.ci` | a withdrawn route stops being enforced | written <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) --> |
| `flowspec-fw-legacy-table-removed` | `test/plugin/flowspec-fw-legacy-table-removed.ci` | a table written by an older build is removed once, with a route announced | written |
| `firewall-legacy-table-removed-with-no-config` | `test/firewall/firewall-legacy-table-removed-with-no-config.ci` | the case the other two proofs step around: no `firewall {}` section, a bridge that never announces, and the stale table must still go while another tool's table under a legacy name survives | written |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-flowspec-sctp-gobgp` | `test/interop/scenarios/` | GoBGP | extend the existing scenario: after the peer withdraws, ze's kernel holds no FlowSpec table | written. `check.py` announces the SCTP route, asserts `table inet ze_flowspec` with the protocol match, then withdraws through GoBGP and asserts no table whose name carries `flowspec` survives |

## Files to Modify
- `internal/plugins/flowspec-firewall/state.go` - the table carries the ownership prefix
- `internal/plugins/firewall/nft/backend_linux.go` - the one-time removal of a legacy unprefixed table, gated on `firewall.LegacySweepPending` so it runs on the first reconcile of the process only
- `internal/component/firewall/legacy_tables.go` - the ledger of what the four bare-name builds wrote, and the gate that ends the migration
- `internal/component/firewall/registry.go` - refuse a table registered without the prefix, so this cannot recur
- `internal/plugins/ddos/local/register.go` - one empty reconcile at startup while the removal is pending. This responder registers nothing until an attack arrives, so a box that is never attacked would get no reconcile that could reach its stale table
- `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` - Rule 2a: the ledger's population, and that "once" means the first reconcile of the process
- `docs/guide/flowspec-protected-router.md` - the kernel table name an operator sees in `nft list ruleset`
- `plan/journal/gate-excludes-part-of-its-population.md` - the 2026-08-18 row names `copp`, `policy-routes` and `firewall-irr` as unprefixed. Those are OWNER names; their TABLE names carry the prefix. Correct the row in place rather than deleting it

## Files to Create
- `test/plugin/flowspec-fw-withdraw-removes-table.ci` - the AC-1 and AC-2 proof
- `test/plugin/flowspec-fw-legacy-table-removed.ci` - the AC-3 proof
- `test/firewall/firewall-legacy-table-removed-with-no-config.ci` - the AC-3 proof on the box with no `firewall {}` section, which the other two step around
### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no operator-visible setting changes |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | N-A | no new leaf |
| CLI commands/flags | No | `StripZeTablePrefix` keeps the operator-facing name unchanged |
| CLI grammar (keyword before value) | N-A | no grammar change |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | N-A | no new output |
| Env var registration | N-A | no new env var |
| Doctor check for runtime dependencies | No | no new runtime dependency |
| Prometheus counters/metrics | No | the legacy removal is logged once; a counter for a one-time event is noise |
| BGP family surface | N-A | FlowSpec 133 and 134 already registered |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | a defect is removed |
| 2 | Config syntax changed? | No | no leaf changes |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | Yes | `docs/guide/flowspec-protected-router.md`: the kernel table name, and that a withdraw removes it |
| 6 | Has a user guide page? | Yes | the same page |
| 7 | Wire format changed? | No | decoding is unchanged |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | RFC 8955 route withdrawal gains a proof that the traffic action stops |
| 10 | Test infrastructure changed? | No | existing runners |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, the firewall table ownership paragraph |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | the owner name is unchanged; only the table name moves |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` for anchors on `nft/backend_linux.go` and `flowspec-firewall/state.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | verify any `nft list ruleset` output quoted in the FlowSpec guide |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the defect before changing it
   - Tests: `TestApplySweepsEveryOwnersTable`, `flowspec-fw-withdraw-removes-table`
   - Files: `internal/plugins/firewall/nft/integration_linux_test.go`, `test/plugin/flowspec-fw-withdraw-removes-table.ci`. The spec first named a `backend_linux_test.go` that does not exist: `Apply` needs a real kernel, so its tests live beside the other `Apply` tests in the integration file.
   - Verify: both red today, the `.ci` showing the table still installed after the withdraw and the duplicate rule after two reconciles
2. **Phase: Validate A-2 and A-3 before the rename** -- enumerate every reader of the literal name and every writer of an unprefixed `flowspec` table
   - Files: the spec's Assumptions table
   - Verify: each A-N flips to `confirmed` or `broken`. A broken A-2 stops the phase and goes to the user
3. **Phase: The prefix** -- the FlowSpec table is named like every other owner's
   - Tests: `TestFlowSpecTableCarriesTheOwnershipPrefix`, the two above
   - Files: `internal/plugins/flowspec-firewall/state.go`
   - Verify: red before, green after, and the operator-facing name is unchanged
4. **Phase: The legacy table** -- an upgrade strands nothing
   - Tests: `flowspec-fw-legacy-table-removed`
   - Files: `internal/plugins/firewall/nft/backend_linux.go`
   - Verify: a kernel seeded with an unprefixed `flowspec` table loses it once, and a second start logs nothing
5. **Phase: Close the class** -- the registry refuses an unprefixed table
   - Tests: `TestEveryRegisteredTableCarriesThePrefix`
   - Files: `internal/component/firewall/registry.go`
   - Verify: registering an unprefixed table is an error, so the next producer cannot repeat this

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Correctness | The sweep's existing refusal to delete an unknown `ze_*` table is untouched |
| Data flow | One prefix decision, made where the table is built, read where the sweep runs |
| Rule: `ai/rules/simplicity.md` | The fix is the name plus the legacy removal plus one guard. No new ownership layer |
| Rule: `ai/rules/interop-and-goal-validation.md` | Reverting the rename reddens the `.ci`, not only the unit test |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| A withdraw removes the table | `test/plugin/flowspec-fw-withdraw-removes-table.ci` <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) --> |
| Two reconciles leave one rule | the same test |
| A legacy table is removed once | `test/plugin/flowspec-fw-legacy-table-removed.ci` <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) --> |
| No producer can register an unprefixed table | `make ze-unit-pkg-test PKG=./internal/component/firewall` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The route set is peer-controlled. A rule that outlives its route is a peer holding a drop in place after withdrawing it |
| Resource exhaustion | Rules accumulating per reconcile is unbounded growth driven from the wire; AC-2 bounds it |

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

- A name used as a proxy for ownership is a convention until something enforces it. Four producers followed it, one did not, and nothing could tell.
- A sweep that skips what it does not recognise fails safe against another producer and fails open against its own.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Name the FlowSpec table with the ownership prefix, remove the legacy table once, and refuse an unprefixed registration | **B. Derive ownership from the registry instead of the name.** Deferred, not rejected: it removes the class rather than this instance, and it is a larger change than the first release needs. Recorded in Known Limitations. **C. Rename only.** REJECTED: it strands the old table on every upgraded router, which is the defect with a new name | The prefix is already the convention four other owners follow, so this restores an invariant rather than inventing one. The registry guard stops the next producer repeating it |

## Known Limitations

- Ownership is still carried by a spelling. The Key Design Decisions table above records why, and names deriving it from the registry as the change that would remove the class rather than this instance.
- The legacy removal can be dropped once no supported upgrade path starts from a build that wrote the unprefixed name.
- **The legacy removal's chain test reads chain NAMES and nothing else.** It does not read the hook, the type or the priority, so a foreign kernel table matches whenever its name, its family and every one of its chain names are ones a ze producer wrote. Three of the five entries are collidable words: `copp` with the chain `input`, `ddos-local` with `ingress` or `forward`, and `anomaly-shape` with `ingress`. Two things bound the residual, and neither closes it. `IsLegacyTable` refuses a table holding any chain ze did not write and refuses one holding no chain at all, and `Apply` runs the removal on the FIRST reconcile of the process only, so a candidate has to have been in the kernel before ze started. Pinning the hook, the type and the priority would close it, and it was not done here for one reason: the values to pin are the ones the OLD build wrote, they differ per producer, and the only test that could prove a pin right is the QEMU integration run. A wrong pin fails silently in the safe direction, leaving the stale table AC-3 exists to remove, and nothing in the merge gate would say so.
- **The whole withdraw path is gated behind the Linux tier, and NOTHING in the merge gate executes it.** Every proof of it is either `//go:build integration && linux` or `option=needs-linux:caps=net-admin`, and `make ze-precommit-verify` runs unprivileged on darwin, so all of it SKIPS there. The unit layer proves the decisions (`IsLegacyTable`, `RegisterTables`, the empty-set autoload gate) and proves nothing about the kernel. Both were re-run at closure on 2026-08-24, over the tree this spec commits, and both are green. `make ze-qemu-integration-test` reports `ok github.com/ze-software/ze/internal/plugins/firewall/nft`, and a verbose re-run of the same package in the VM names every test: `TestApplySweepsEveryOwnersTable`, `TestApplyRemovesTheLegacyUnprefixedTableOnce`, `TestApplyLeavesAnotherToolsTableAlone`, `TestApplyWithNoDesiredTablesStillRemovesTheLegacyTable`, `TestApplyRemovesEveryLegacyTableInItsOwnFamily`, `TestTheLegacyRemovalRunsOnceInTheProcess`, `TestIsLegacyTableRefusesBeforeItReadsTheKernel`, all PASS. `make ze-qemu-needs-linux-test` passes the whole `firewall` suite 22 of 22, `firewall-legacy-table-removed-with-no-config` included, and the four kernel-touching `flowspec-fw-*.ci`: `flowspec-fw-legacy-table-removed`, `flowspec-fw-protocol-sctp`, `flowspec-fw-untranslatable-keeps-others`, `flowspec-fw-withdraw-removes-table`. Four, not six: `flowspec-fw-add` and `flowspec-fw-withdraw` assert wire bytes, declare no `option=needs-linux`, and `ZE_QEMU_LINUX_ONLY=1` skips them. `INTEROP_SCENARIO=bgp-flowspec-sctp-gobgp make ze-interop-test` passes the withdraw assertion against GoBGP. The standing CI home is `.github/workflows/qemu-nightly.yml`, which is advisory and reports rather than blocks.
- **The startup empty reconcile has three call sites and one proof.** The FlowSpec bridge (`engine.go`, `runEngine`), the anomaly-shape responder (`register.go`) and ddos-local (`register.go`) each run one empty reconcile while the removal is pending, because each registers nothing until an event arrives. `test/firewall/firewall-legacy-table-removed-with-no-config.ci` exercises the FlowSpec one end to end, and `TestAnEmptyReconcileReachesABackendWhileTheSweepIsPending` proves the registry side that all three depend on. The other two call sites are read, not run: each sits inside a plugin `Run` closure that no unit test can reach without restructuring the plugin's startup.
- **One shape of the stranded table stays out of reach: the upgrade that also DELETES the owner from the config.** The one-time removal runs inside `Backend.Apply`, and a backend is loaded only when a producer registers a table or a plugin that owns a legacy name starts. If an operator upgrades and removes the `flowspec-firewall` plugin block in the same step, nothing loads the nft backend and the old `flowspec` table survives. Closing that needs a firewall path that runs with no config at all, and no such path exists: `getConfigPathPlugins` (`internal/component/plugin/server/startup_autoload.go`) starts a plugin only when a config root it declares is present, and there is no always-load phase (`Server.StartWithContext`, `server.go`). Giving the firewall component one is a change to plugin startup for every box and belongs in its own spec, not in this fix.

## RFC Documentation (Scope: protocol)

Add `// RFC 8955: a withdrawn FlowSpec NLRI stops the traffic action it carried`
above the sweep path that removes the table, and tag the functional test with the
RFC 8955 withdrawal requirement id once `rfc/short/rfc8955.md` carries one.

## Goal Validation (BLOCKING)

The Linux tier is where four of these five rows are proven, and it does not run
on darwin. Every row below names the command that produces its evidence. All
three of those commands ran at closure on 2026-08-24 and all three are green;
none is owed. The Known Limitations entry above carries the outputs.

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A FlowSpec rule a peer withdraws stops being enforced | interop + functional | `test/interop/scenarios/bgp-flowspec-sctp-gobgp/check.py` announces an SCTP FlowSpec route from GoBGP, asserts `table inet ze_flowspec` carries the protocol match, then withdraws it and asserts no table whose name carries `flowspec` survives. `test/plugin/flowspec-fw-withdraw-removes-table.ci` drives the same withdraw over the wire and reads the kernel ruleset. Command: `make ze-interop-test`, `make ze-qemu-needs-linux-test` |
| Rules stop accumulating on every reconcile | functional + integration | `test/plugin/flowspec-fw-withdraw-removes-table.ci` counts the rules the daemon removed across two reconciles, asserting `count=1\b` rather than "a rule is present". `TestApplySweepsEveryOwnersTable` drives two `Apply` calls against a real kernel and requires one rule. Command: `make ze-qemu-needs-linux-test`, `make ze-qemu-integration-test` |
| An upgraded box strands no table an older build wrote | integration + functional | `TestApplyRemovesEveryLegacyTableInItsOwnFamily` seeds one kernel table per ledger entry, in that entry's own family, and requires all five gone after one `Apply` while an operator's table survives. `test/plugin/flowspec-fw-legacy-table-removed.ci` and `test/firewall/firewall-legacy-table-removed-with-no-config.ci` prove it from the daemon, the second with no `firewall {}` section at all. `TestEveryProducerThatShippedABareNameIsInTheLedger` pins the ledger's population against the four producers `git log -S` shows shipped bare. Command: `make ze-qemu-integration-test`, `make ze-qemu-needs-linux-test`, `make ze-unit-pkg-test PKG=./internal/component/firewall` |
| The removal is a migration with an end, not a standing deletion policy | integration | `TestTheLegacyRemovalRunsOnceInTheProcess` drives `firewall.ApplyAll` twice against a real kernel, re-seeding the same bare table between the two, and requires the second one to survive. Reverting the `sweepLegacy` gate in `Apply` reddens it. Command: `make ze-qemu-integration-test` |
| No producer can register an unprefixed table again | unit | `TestEveryRegisteredTableCarriesThePrefix` and `TestLegacyTableNamesAreNoLongerRegistrable` (`internal/component/firewall`). Command: `make ze-unit-pkg-test PKG=./internal/component/firewall`, run 2026-08-24, `ok github.com/ze-software/ze/internal/component/firewall` |
| The name an operator types and reads is unchanged | unit | `TestFlowSpecTableCarriesTheOwnershipPrefix` (`internal/plugins/flowspec-firewall/state_test.go`) asserts the built name is `ze_flowspec` and that `firewall.StripZeTablePrefix` renders it back as `flowspec`. Command: `make ze-unit-pkg-test PKG=./internal/plugins/flowspec-firewall` |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
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

---

## Implementation Summary

### What Was Implemented
- **The prefix.** `tableName = "ze_flowspec"` (`internal/plugins/flowspec-firewall/state.go`) and
  `tableNameV4`/`tableNameV6` (`internal/plugins/anomaly/shape/match.go`). Those were the two
  producers still bare in the tree.
- **The guard.** `RegisterTables` (`internal/component/firewall/registry.go`) returns an error for a
  name without `tableNamePrefix` and stores nothing. Every caller was changed to carry the error:
  `flowspec-firewall/engine.go`, `anomaly/shape/responder.go`, `ddos/local/responder.go`,
  `copp/register.go`, `policyroute/register.go`, `firewall/plugins/irr/irr.go`,
  `firewall/engine.go`. A withdraw (`nil`) can never be refused and reads `_ =`.
- **The ledger and the migration.** `internal/component/firewall/legacy_tables.go` holds
  `legacyTables` (five names from four producers), `IsLegacyTableName` (the pre-filter),
  `IsLegacyTable` (name AND family AND every chain), and `legacySweepPending` with
  `LegacySweepPending`/`legacySweepReached`. `(*backend).Apply` and `(*backend).isLegacyTable`
  (`internal/plugins/firewall/nft/backend_linux.go`) delete each entry once and log it.
  `firewall.Logger()` (`backend.go`) is what gives the backend the component's logger.
- **The reachable trigger.** `ApplyAll` autoloads a backend for an EMPTY desired set while the sweep
  is pending, and the three event-driven owners (`flowspec-firewall/engine.go` `runEngine`,
  `anomaly/shape/register.go`, `ddos/local/register.go`) each run one empty reconcile at startup.
- **The contention ratchet.** Two cluster rows in `TestContendingFunctionalTestsDeclareExclusiveGroup`
  (`internal/test/runner/exclusive_group_test.go`) put every kernel-touching FlowSpec test and the
  legacy seeds into one `flowspec-nft` exclusive group.

### Bugs Found/Fixed
- **The ledger was derived from the TREE and the tree is not what shipped.** Found on the 2026-08-24
  review. `copp` and `ddos-local` had already been renamed in `c5273da42` and `1423ad437`, so reading
  every `firewall.Table` literal at HEAD saw two bare producers where four had shipped bare. Both
  entries were added, and `legacyTable` gained `families []TableFamily` because `ddos-local` is one
  name in two families. Covered by `TestEveryProducerThatShippedABareNameIsInTheLedger` and
  `TestApplyRemovesEveryLegacyTableInItsOwnFamily`.
- **The removal read as a standing deletion policy, not a migration.** `Apply` asked
  `IsLegacyTable` on every reconcile, so a bare table written after ze started was ze's to delete
  forever. `Apply` now reads `firewall.LegacySweepPending()` once into `sweepLegacy` before the loop.
  Covered by `TestTheLegacyRemovalRunsOnceInTheProcess`.
- **The migration could not reach the box it exists for.** `ApplyAll` returned before it loaded a
  backend when the desired set was empty, and every owner of a bare name registers only on an event.
  Covered by `TestAnEmptyReconcileReachesABackendWhileTheSweepIsPending` and
  `test/firewall/firewall-legacy-table-removed-with-no-config.ci`. Journal row:
  `plan/journal/invariant-enforced-by-an-absent-call-site.md`, 2026-08-24.
- **`expect=stderr:pattern=count=1` matched `count=10` through `count=19`.** Anchored to `count=1\b`
  in `test/plugin/flowspec-fw-withdraw-removes-table.ci`.
- **The cluster enumeration in `docs/functional-tests.md` was short by one** (found at closure). The
  diff added a sixth and seventh ratchet row and the doc still said "four clusters" inside the plugin
  suite and named five clusters. Fixed with a FlowSpec-cluster paragraph and two source anchors.

### Documentation Updates
- `docs/architecture/firewall/table-ownership-and-shutdown-flush.md`: seven producers, Rule 2 is now
  enforced rather than conventional, and a new Rule 2a for the one-time removal. Anchors:
  `registry.go -- RegisterTables`, `legacy_tables.go -- legacyTables, IsLegacyTable, legacySweepPending`,
  `nft/backend_linux.go -- Apply, isLegacyTable`, `flowspec-firewall/state.go -- tableName`,
  `anomaly/shape/match.go -- tableNameV4`, `ddos/local/register.go`.
- `docs/architecture/core-design.md`: the table-ownership paragraph now states the registry guard and
  the one-time removal.
- `docs/guide/flowspec-protected-router.md`: `ze_flowspec` in every `nft` command, what a withdraw
  does, the log line the removal writes, and the hand cleanup for the one upgrade shape out of reach.
- `docs/functional-tests.md`: the FlowSpec exclusive-group cluster, plus the corrected cluster count.
- No RFC status row moved: RFC 8955 gains a proof of an obligation RFC 4271 Section 9 states, and
  `rfc/short/rfc8955.md` carries no withdrawal requirement id to tag (recorded in RFC Documentation).

### Deviations from Plan
- The spec's step 1 named a `backend_linux_test.go`. `Apply` needs a real kernel, so its tests live in
  `integration_linux_test.go` beside the other `Apply` tests.
- A-1 and A-2 both broke; both corrections are recorded in the Assumptions table with their evidence.
- The chain test reads chain NAMES only. Pinning hook, type and priority was considered and recorded
  in Known Limitations rather than implemented: the values to pin are the OLD builds' and only the
  QEMU run could prove a pin right, while a wrong pin fails silently in the safe direction.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1: `flowspec` was the only bare table name | Four producers shipped bare and two of them were already renamed, so the tree could not show them | The 2026-08-23 re-run found `anomaly-shape`; the 2026-08-24 review asked what an UPGRADED box holds and `git log -S` found `copp` and `ddos-local` | Ledger carries all five names; `TestEveryProducerThatShippedABareNameIsInTheLedger` pins the population |
| assumption | A-2: nothing outside the plugin depends on the literal name | Three `.ci` assertions, one interop `check.py` and two doc commands carried it. The first reason written for A-2 was also wrong: the CLI reads `LastApplied()`, not the prefix-filtered kernel readback | `gopls references` plus a grep over `test/`, `docs/`, `internal/component/web/` | All six updated; the corrected reason is in the Assumptions table |
| approach | The removal was written as a decision `Apply` takes on every reconcile | "Once" has to be enforced, or the migration is a standing policy that deletes another tool's table in every release | Review, 2026-08-24 | `sweepLegacy` read once before the loop, gated on `LegacySweepPending` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A withdrawn FlowSpec rule stops being enforced | Done | `(*backend).Apply` sweep loop, `internal/plugins/firewall/nft/backend_linux.go`; `tableName`, `internal/plugins/flowspec-firewall/state.go` | The table now carries the prefix the sweep tests |
| Rules stop accumulating per reconcile | Done | same sweep loop, ahead of `applyTable` | `TestApplySweepsEveryOwnersTable`, two reconciles, one rule |
| An upgraded box strands no table | Done | `legacyTables`, `IsLegacyTable`, `internal/component/firewall/legacy_tables.go`; `(*backend).isLegacyTable` | Five names, four producers, each proven against a real kernel in its own family |
| The class cannot recur | Done | `RegisterTables`, `internal/component/firewall/registry.go` | Refuses a bare name and returns the error to the owner |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `test/plugin/flowspec-fw-withdraw-removes-table.ci`; `test/interop/scenarios/bgp-flowspec-sctp-gobgp/check.py` | Interop PASS 2026-08-24: "the withdrawn route is no longer enforced" |
| AC-2 | Done | `TestApplySweepsEveryOwnersTable`; the same `.ci`, `count=1\b` | QEMU integration PASS 2026-08-24 |
| AC-3 | Done | `TestApplyRemovesTheLegacyUnprefixedTableOnce`, `TestApplyWithNoDesiredTablesStillRemovesTheLegacyTable`, `TestApplyRemovesEveryLegacyTableInItsOwnFamily`, `TestTheLegacyRemovalRunsOnceInTheProcess`; `test/plugin/flowspec-fw-legacy-table-removed.ci`; `test/firewall/firewall-legacy-table-removed-with-no-config.ci` | All four integration tests PASS by name in QEMU 2026-08-24 |
| AC-4 | Done | `TestEveryRegisteredTableCarriesThePrefix`, `TestLegacyTableNamesAreNoLongerRegistrable` | `ok github.com/ze-software/ze/internal/component/firewall 1.710s`, 2026-08-24 |
| AC-5 | Done | `TestFlowSpecTableCarriesTheOwnershipPrefix` | Asserts `StripZeTablePrefix("ze_flowspec")` renders `flowspec` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestApplySweepsEveryOwnersTable` | Done | `internal/plugins/firewall/nft/integration_linux_test.go` | PASS 0.22s in QEMU |
| `TestApplyRemovesTheLegacyUnprefixedTableOnce` | Done | same | PASS 0.11s |
| `TestApplyLeavesAnotherToolsTableAlone` | Done | same | PASS 0.10s |
| `TestApplyWithNoDesiredTablesStillRemovesTheLegacyTable` | Done | same | PASS 0.04s |
| `TestApplyRemovesEveryLegacyTableInItsOwnFamily` | Done | same | PASS 0.09s |
| `TestTheLegacyRemovalRunsOnceInTheProcess` | Done | same, last in file order | PASS 0.15s |
| `TestIsLegacyTableRefusesBeforeItReadsTheKernel` | Done | `internal/plugins/firewall/nft/lower_linux_test.go` | PASS 0.00s; covers the three returns ahead of any netlink call |
| `TestFlowSpecTableCarriesTheOwnershipPrefix` | Done | `internal/plugins/flowspec-firewall/state_test.go` | |
| `TestEveryRegisteredTableCarriesThePrefix` | Done | `internal/component/firewall/registry_test.go` | |
| `TestIsLegacyTableRequiresNameFamilyAndChainShape` | Done | `internal/component/firewall/legacy_tables_test.go` | 21 cases |
| `TestIsLegacyTableNameIsOnlyThePreFilter` | Done | same | |
| `TestAnEmptyReconcileReachesABackendWhileTheSweepIsPending` | Done | same | |
| `TestLegacyTableNamesAreNoLongerRegistrable` | Done | same | |
| `TestEveryProducerThatShippedABareNameIsInTheLedger` | Done | same | |
| `flowspec-fw-withdraw-removes-table` | Done | `test/plugin/` | |
| `flowspec-fw-legacy-table-removed` | Done | `test/plugin/` | |
| `firewall-legacy-table-removed-with-no-config` | Done | `test/firewall/` | |
| `bgp-flowspec-sctp-gobgp` | Done | `test/interop/scenarios/` | PASS 2026-08-24 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/flowspec-firewall/state.go` | Done | |
| `internal/plugins/firewall/nft/backend_linux.go` | Done | |
| `internal/component/firewall/legacy_tables.go` | Done | new |
| `internal/component/firewall/registry.go` | Done | |
| `internal/plugins/ddos/local/register.go` | Done | |
| `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` | Done | |
| `docs/guide/flowspec-protected-router.md` | Done | |
| `plan/journal/gate-excludes-part-of-its-population.md` | Done | 2026-08-18 row corrected in place, twice |
| `test/plugin/flowspec-fw-withdraw-removes-table.ci` | Done | new |
| `test/plugin/flowspec-fw-legacy-table-removed.ci` | Done | new |
| `test/firewall/firewall-legacy-table-removed-with-no-config.ci` | Done | new |
| `internal/plugins/anomaly/shape/{match,register,responder,testsupport}.go` | Changed | not in the plan: A-1 broke and the second bare producer is here |
| `internal/plugins/copp/register.go`, `internal/plugins/policyroute/register.go`, `internal/component/firewall/plugins/irr/irr.go`, `internal/component/firewall/engine.go` | Changed | every `RegisterTables` caller had to carry the new error |
| `internal/component/firewall/backend.go` | Changed | `Logger()`, so the backend logs the removal under `ze.log.firewall` |
| `internal/test/runner/exclusive_group_test.go` | Changed | the `flowspec-nft` exclusive group |
| `docs/architecture/core-design.md`, `docs/functional-tests.md` | Changed | doc drift found at closure |
| `test/weakened.md` | Changed | one row: the moved helper body, no coverage lost |

### Audit Summary
- **Total items:** 5 AC, 18 tests, 17 file groups
- **Done:** all 5 AC, all 18 tests, all 11 planned files
- **Partial:** none
- **Skipped:** none
- **Changed:** 6 file groups the plan did not name, each recorded above and in Deviations

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none: the spec metadata declares no deferral shard | n/a | `ls plan/deferrals/` holds no `flowspec` or `table-never-swept` stem, so this closure removes no shard |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-flowspec-table-never-swept-9ad8358c-695f-41be-8019-5d92ba08f8e6.md`, 38 files, verdict clean |
| `review_gate.py check` | clean |
| Rounds | 2. Round 1 read the complete diff and found the ISSUE below. Round 2 was scoped to what round 1's fixes touched and found only a record defect, so it is the last round (`ai/rules/planning.md`, "A finding in the record is not a finding in the product"). The implementation phase's own rounds are in its report and are not counted here |
| Reviewer lenses used | wiring + guard audit; removed-behaviour + test-rewrite; documentation drift + simplicity; style pass over every changed Go file |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The diff adds the sixth and seventh rows to `TestContendingFunctionalTestsDeclareExclusiveGroup` and the doc that enumerates those clusters was not updated: it still said "four clusters" inside the plugin suite and named five | `docs/functional-tests.md` | A FlowSpec-cluster paragraph with two source anchors, and the count corrected to five |
| 2 | NOTE | The test-relaxation audit reports `addNftIntegrationTableWithChain` as weakened: its body moved into `addNftIntegrationTableInFamily` | `internal/plugins/firewall/nft/integration_linux_test.go` | One row in `test/weakened.md` naming what moved and where the assertions are; `make ze-test-weakened-check` green |
| 3 | NOTE | A duplicated word in a test header | `test/firewall/firewall-legacy-table-removed-with-no-config.ci` | Removed |
| 4 | NOTE | The Known Limitations paragraph said "the six `test/plugin/flowspec-fw-*.ci`" run under `ze-qemu-needs-linux-test`; only four declare `option=needs-linux` | this spec | Corrected in place |
| 5 | NOTE (round 2) | The `test/weakened.md` row round 1 wrote said the helper has six call sites; it has seven | `test/weakened.md` | Corrected. Round 2 found nothing else in its scope, so it is the last round |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/firewall/legacy_tables.go` | Yes | `wc -l` 142 |
| `internal/component/firewall/legacy_tables_test.go` | Yes | `wc -l` 175 |
| `test/plugin/flowspec-fw-withdraw-removes-table.ci` | Yes | `wc -l` 228 |
| `test/plugin/flowspec-fw-legacy-table-removed.ci` | Yes | `wc -l` 179 |
| `test/firewall/firewall-legacy-table-removed-with-no-config.ci` | Yes | `wc -l` 139 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | A withdraw takes the table out of the kernel | `INTEROP_SCENARIO=bgp-flowspec-sctp-gobgp make ze-interop-test`, 2026-08-24: `PASS 1 scenario(s)`, carrying the line `the withdrawn route is no longer enforced`. The run pinned its image by ID, `sha256:9b8390f8d3c5...` |
| AC-2 | Two reconciles leave one rule | `--- PASS: TestApplySweepsEveryOwnersTable (0.22s)` in QEMU, 2026-08-24 |
| AC-3 | A legacy table is removed once | `--- PASS: TestApplyRemovesTheLegacyUnprefixedTableOnce (0.11s)`, `TestApplyWithNoDesiredTablesStillRemovesTheLegacyTable (0.04s)`, `TestApplyRemovesEveryLegacyTableInItsOwnFamily (0.09s)`, `TestTheLegacyRemovalRunsOnceInTheProcess (0.15s)`, `TestApplyLeavesAnotherToolsTableAlone (0.10s)` |
| AC-4 | No producer can register a bare name | `make ze-unit-pkg-test PKG=./internal/component/firewall`: `ok github.com/ze-software/ze/internal/component/firewall 1.710s`, 2026-08-24 |
| AC-5 | The operator-facing name is unchanged | `make ze-unit-pkg-test PKG=./internal/plugins/flowspec-firewall`: `ok ... 1.097s`, which runs `TestFlowSpecTableCarriesTheOwnershipPrefix` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A peer withdraws its last FlowSpec route | `test/plugin/flowspec-fw-withdraw-removes-table.ci` | Read: the peer sends two announces and one MP_UNREACH, the driver waits for the pair of facts, then SIGTERMs the peer and requires no table carrying `flowspec` plus `rules removed ... count=1\b` |
| A peer announces a route that reconciles twice | `TestApplySweepsEveryOwnersTable` | Read: two identical `Apply` calls against a real kernel, then `Apply(nil)` and the table must be gone |
| An upgraded box with no firewall config at all | `test/firewall/firewall-legacy-table-removed-with-no-config.ci` | Read: seeds `inet flowspec`/`flowspec-fwd` and `ip anomaly-shape`/`forward`, the config carries only the bridge, and it asserts the first goes and the second stays |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | `git log --oneline -S'"ze_pr"'` and `-S'"ze_irr_iface"'` show each introduced already prefixed (`a6161c217` adds `const policyRoutingTable = "ze_pr"`; `f439d7066` adds `ifaceTableName = "ze_irr_iface"`), while `copp` and `ddos-local` shipped bare. Four producers, five names, pinned by `TestEveryProducerThatShippedABareNameIsInTheLedger` |
| A-2 | broken | `test/plugin/flowspec-fw-protocol-sctp.ci`, `flowspec-fw-untranslatable-keeps-others.ci`, `test/interop/scenarios/bgp-flowspec-sctp-gobgp/check.py` and `docs/guide/flowspec-protected-router.md` all carried the literal and all are updated. `grep -rn "table inet flowspec" docs/` now returns only the deliberate hand-cleanup commands |
| A-3 | confirmed, with two corrections | `IsLegacyTable` decides on name, family AND every chain, and `TestApplyLeavesAnotherToolsTableAlone` seeds that collision against a real kernel and requires the table to survive (PASS). The residual is bounded by `LegacySweepPending` and recorded in Known Limitations |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Rule 2 table, seven producers | `policyRoutingTable` (`internal/plugins/policyroute/translate.go`), `ifaceTableName` (`internal/component/firewall/plugins/irr/sets.go`), `coppTableName`, `tableName`, `tableNameV4` and `tableNameV6`, each read at its literal | Yes |
| Rule 2a, "once means once" | `sweepLegacy := firewall.LegacySweepPending()` read before the loop in `(*backend).Apply`; `legacySweepReached()` in `ApplyAll` | Yes |
| The guide's `nft` commands | `tableName` and `StripZeTablePrefix` | Yes |
| The log line the guide quotes | `firewall.Logger().Info("firewallnft: deleting a table an earlier ze build left without the ownership prefix", ...)` | Yes, byte-identical to the `.ci` assertion |
| Cluster enumeration | the `clusters` slice in `internal/test/runner/exclusive_group_test.go`, seven rows, five of them in the `plugin` suite | Yes, after the fix |
| No stale anchors elsewhere | `grep -rn "source:.*(nft/backend_linux|flowspec-firewall/state|firewall/registry|anomaly/shape/match|copp/translate|ddos/local/responder|policyroute/register|firewall/engine)" docs/` -- every hit re-read; the claims at `docs/architecture/anomaly/anomaly-2-shape.md`, `docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md` and `docs/guide/firewall.md` are about the owner key and the backend seam, neither of which moved | Yes |

## Core Insight

An ownership convention carried by a NAME needs two things a rename does not give it: something that
refuses the next producer that forgets, and a ledger of what the forgetful builds already put in the
kernel. The second is the one that gets missed, because the obvious way to build it is to read the
tree, and the tree holds who is bare NOW. What an upgraded box holds is what SHIPPED, and only
`git log -S` over the old spelling answers that.
