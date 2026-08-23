# Spec: fixit-flowspec-table-never-swept

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-22 |

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
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `flowspec` is the only firewall table name in the tree without the `ze_` prefix | every `firewall.Table` producer, enumerated 2026-08-22: `ze_copp`, `ze_pr`, `ze_irr_iface`, `ze_ddos-local`, and the config-derived tables | the fix is one name and the class stays open for the others | the enumeration, re-run at implementation time | confirmed 2026-08-22, to be re-run |
| A-2 | Nothing outside the plugin depends on the literal table name `flowspec` | `StripZeTablePrefix` renders the operator-facing name; the CLI and web page read the stripped form | the rename breaks a CLI argument, a web page, or a `.ci` assertion | `gopls references` plus a grep over `test/`, `docs/` and `internal/component/web/` | unvalidated |
| A-3 | Removing a legacy unprefixed `flowspec` table cannot destroy another producer's table | the sweep's own comment says an unknown `ze_*` table may belong to another producer; an unprefixed `flowspec` was written by this plugin and by nothing else | a one-time removal deletes a table Ze does not own | a grep for any other writer of a table named `flowspec`, plus the nft table family | unvalidated |

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
| `TestApplySweepsEveryOwnersTable` | `internal/plugins/firewall/nft/backend_linux_test.go` | a desired table from every owner is swept before it is recreated | <!-- doc-links: ignore (this spec's own acceptance criteria create this artifact; the spec is ready and not yet authorised to run) --> |
| `TestFlowSpecTableCarriesTheOwnershipPrefix` | `internal/plugins/flowspec-firewall/state_test.go` | the table this plugin builds is named so the backend owns it | |
| `TestEveryRegisteredTableCarriesThePrefix` | `internal/component/firewall/registry_test.go` | validates AC-4: no producer can register an unprefixed table | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Reconciles before a rule count is asserted | 1..N | N | N/A | N/A |
| Rules per FlowSpec route after N reconciles | 1 | 1 | N/A | 2 is the defect |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `flowspec-fw-withdraw-removes-table` | `test/plugin/flowspec-fw-withdraw-removes-table.ci` | a withdrawn route stops being enforced | <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) --> |
| `flowspec-fw-legacy-table-removed` | `test/plugin/flowspec-fw-legacy-table-removed.ci` | a table written by an older build is removed once | <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) --> |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-flowspec-sctp-gobgp` | `test/interop/scenarios/` | GoBGP | extend the existing scenario: after the peer withdraws, ze's kernel holds no FlowSpec table | |

## Files to Modify
- `internal/plugins/flowspec-firewall/state.go` - the table carries the ownership prefix
- `internal/plugins/firewall/nft/backend_linux.go` - the one-time removal of a legacy unprefixed table
- `internal/component/firewall/registry.go` - refuse a table registered without the prefix, so this cannot recur
- `docs/guide/flowspec-protected-router.md` - the kernel table name an operator sees in `nft list ruleset`
- `plan/journal/gate-excludes-part-of-its-population.md` - the 2026-08-18 row names `copp`, `policy-routes` and `firewall-irr` as unprefixed. Those are OWNER names; their TABLE names carry the prefix. Correct the row in place rather than deleting it

## Files to Create
- `test/plugin/flowspec-fw-withdraw-removes-table.ci` - the AC-1 and AC-2 proof <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) -->
- `test/plugin/flowspec-fw-legacy-table-removed.ci` - the AC-3 proof <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) -->

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
   - Files: `internal/plugins/firewall/nft/backend_linux_test.go`, `test/plugin/flowspec-fw-withdraw-removes-table.ci` <!-- doc-links: ignore (this spec's own acceptance criteria create this file; the spec is ready and not yet authorised to run) -->
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

## RFC Documentation (Scope: protocol)

Add `// RFC 8955: a withdrawn FlowSpec NLRI stops the traffic action it carried`
above the sweep path that removes the table, and tag the functional test with the
RFC 8955 withdrawal requirement id once `rfc/short/rfc8955.md` carries one.

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
