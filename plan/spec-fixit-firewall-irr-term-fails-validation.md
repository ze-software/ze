# Spec: fixit-firewall-irr-term-fails-validation

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | - |
| Phase | 5/5 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A firewall table term that matches by ASN or AS-SET cannot be committed at all.

Reproduced against a binary built from the working tree on 2026-08-18. A config
holding one inet table, one input chain, one term whose from-block carries
`source-asn 13335`, run through `ze config validate`, exits 1 with:

  firewall: table "ze_wan" chain "input" term "t": match references unknown set "irr_v4_AS13335"

The `source-as-set` spelling fails the same way, naming `irr_v4_AS-CLOUDFLARE`.

The cause is an ownership split. `irrSetMatch` in the config parser emits a
`MatchInSet` naming the IRR set, and emits no `Set`, because the sets belong to
the `firewall-irr` plugin, which registers them as a separate owner and has them
merged in at `ApplyAll`. `ValidateTables` runs long before that merge, from
`OnConfigVerify` and `OnConfigure`, and `collectSetNames` reads only the table's
own `Sets`. Every IRR match therefore looks like a reference to a set that does
not exist.

Cached data cannot help: the check reads the table, never the store, so the
failure is unconditional and independent of whether the prefixes were fetched.

Two consequences follow beyond the rejection itself. The documented operator
workflow instructs exactly this config as step 3, so the guide teaches a config
the parser refuses. And the documented promise that a commit rejects with an
actionable error naming the missing IRR entry and the command to run is
unreachable for table terms: the set-reference error fires first and says
nothing about IRR.

The interface-binding form of the feature works, because the plugin builds both
the sets and the terms in one owner. Only the table-term form is dead.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/firewall/firewall-irr.md` - how IRR sets reach the ruleset and who owns them
  → Constraint: the sets are owned by the plugin and merged at apply, so validation of a config referencing them cannot be a per-table check alone
- [ ] `docs/guide/firewall.md` - the documented operator workflow and the config-leaf table
  → Constraint: the guide's step 3 is the config this spec makes work; the doc is the acceptance statement, not an afterthought
- [ ] `ai/rules/plugins.md` - no plugin spelling in generic or central packages
  → Decision: a carve-out keyed on the IRR set-name prefix inside the firewall component would hardcode plugin spelling in a core package, so the fix must be a registration, not a prefix test
- [ ] `ai/rules/evidence.md` - guards fail closed, and a lookup that gates behavior must not answer from a partial view
  → Constraint: the fix must keep refusing a genuinely unknown set name; widening the check into an exemption would remove a real guard

**Key insights:**
- The parser already admits the coupling: the IRR match builder carries a comment recording that the set-name spelling is shared with the plugin by design decision. The fix must not deepen that coupling into a second place.
- Validation runs twice, at verify and at configure, and both see the same partial view.
- The plugin owns a verify hook of its own that rejects an uncached reference. That hook is where IRR knowledge already lives.
- No test covers parse plus validate for a table term: the parser test stops at the parse, and neither IRR functional test uses the table form.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/config.go` - `irrSetMatch` builds the `MatchInSet` for an IRR reference and adds no `Set`; `expandIRRTermV6` clones the term for IPv6 with the v6 set name; the prefix constants live here
- [ ] `internal/component/firewall/validate.go` - `ValidateTables` collects per-table names and validates every term; `collectSetNames` reads only the table's own sets; `validateMatch` refuses a `MatchInSet` naming anything absent from that map
- [ ] `internal/component/firewall/engine.go` - `ValidateTables` is called from the verify path and again from the configure path, both on the parsed config's tables alone
- [ ] `internal/component/firewall/registry.go` - `ApplyAll` merges every owner's tables and sets; this is the first point at which the IRR sets and the config's terms coexist
- [ ] `internal/component/firewall/plugins/irr/sets.go` - `buildSets` names the sets, and `buildIfaceTables` builds sets and terms together, which is why the interface form works
- [ ] `internal/component/firewall/plugins/irr/verify_test.go` - the plugin's own verify rejects an uncached reference, which is the IRR-aware check that already exists
- [ ] `internal/component/firewall/config_test.go` - the source-asn parser test asserts the emitted matches and never calls `ValidateTables`
- [ ] `docs/guide/firewall.md` - documents the workflow, the leaves, and a commit-rejection promise that the current code cannot deliver for table terms

**Runtime evidence:**
- [ ] `ze config validate` on a table term using `source-asn`, and again using `source-as-set`, both exit 1 with the unknown-set error naming the IRR set. Built from the working tree, 2026-08-18

**Behavior to preserve:**
- A term naming a set that genuinely does not exist must still be refused at verify time, with the same message.
- The set-type agreement check must keep working: a match field and a set type that disagree must still be refused.
- The interface-binding form keeps working exactly as it does now.
- The plugin's uncached-reference rejection keeps its actionable message.

**Behavior to change:**
- A table term matching by ASN or AS-SET validates, commits, and reaches the kernel.
- The error an operator sees for an uncached IRR reference names the IRR entry and the command to fetch it, as the guide already claims.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A configuration commit carrying a firewall table whose term has an IRR match in its from-block.
- Format at entry: the resolved config tree section for the firewall root.

### Transformation Path
1. `ParseFirewallConfig` in `internal/component/firewall/config.go` builds the tables; `irrSetMatch` emits the IRR match and no set
2. `expandIRRTermV6` adds the IPv6 twin of the term
3. The verify path in `internal/component/firewall/engine.go` calls `ValidateTables` on those tables alone
4. `collectSetNames` builds the per-table set map; `validateMatch` refuses the IRR match, and the commit stops here today
5. Had it passed, the configure path would call `ValidateTables` again, then register the tables under the firewall owner
6. The `firewall-irr` plugin registers its own tables and sets under its owner
7. `ApplyAll` merges every owner and hands one set of tables to the backend, which is where the IRR sets and the config's terms finally meet

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ firewall component | the parsed firewall section | Yes, by the runtime reproduction |
| Firewall component ↔ IRR plugin | separate registry owners, merged at apply | Yes |
| Registry ↔ backend | merged tables handed to the backend | No |
| Component ↔ operator | the verify error text | Yes, by the runtime reproduction |

### Integration Points
- `ValidateTables` - the guard that must learn about sets another owner provides
- `ApplyAll` - the existing merge point, and the only place the full picture exists today
- the IRR plugin's verify hook - where IRR-aware rejection already lives and where the actionable message belongs

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
| A-1 | The rejection is unconditional and cached data does not change it | `collectSetNames` reads only the table's own sets, and the runtime reproduction ran with no cached data | With data cached the config might commit, and the defect would be a race rather than a wall | Repeat the runtime reproduction after fetching the AS-SET, and record both outcomes | confirmed. `test/plugin/firewall-irr-table-term-commit.ci` fetches AS-TEST first and the commit was still refused with `match references unknown set "irr_v4_AS-TEST"`, measured on a build with the fix reverted |
| A-2 | The interface-binding form is unaffected | its sets and terms are built together by one owner | The whole feature is dead, not half, and the spec's scope grows | The existing interface functional tests must stay green, and one must be read to confirm it exercises the table-free path | confirmed. `firewall-irr-iface-commit.ci` and `firewall-irr-iface-reject.ci` carry no firewall table and both pass |
| A-3 | Validation can consult a registered provider of externally-owned set names without weakening the unknown-set guard | the registry already knows every owner; the check needs the names, not the contents | The guard becomes an exemption and a typo in an AS-SET name reaches the lowering layer | A test that a misspelled non-IRR set name is still refused, and one that a misspelled AS-SET is refused by the IRR-aware check with its own message | broken as stated, in the way the row predicted. A provider registered by NAMESPACE is the only order-independent shape, and it accepts `source-address "@irr_v4_typo"` typed by hand, which then aborts the whole reconcile at the backend. See the Key Design Decisions row |
| A-4 | No third owner will need the same treatment soon | copp, policy routes and ddos-local build their own sets and terms together | The fix is shaped for one plugin and the next one repeats it | Read each owner's table builder in Phase 1 and record whether any emits a match naming another owner's set | confirmed. `MatchInSet` has one other producer, `parseAddressMatch` in `internal/plugins/policyroute/translate.go`, and it names a set the operator wrote on that owner's own table. copp and ddos-local emit none |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A fix that exempts the IRR name prefix hardcodes plugin spelling into a core package | The change adds a string literal naming the plugin's prefix to the firewall component | Reject that shape: the names come from a registration, not a literal. The parser's existing coupling comment is a warning, not a precedent to extend |
| R-2 | Moving validation to the merged view delays the error from commit time to apply time | An operator's typo is reported after the commit succeeds | Keep the verify-time check and give it the extra names; do not replace it with an apply-time check |
| R-3 | The IRR-aware message and the set-reference message could both fire, in an order that hides the useful one | The operator sees the unknown-set error again after the fix | Order the checks so the IRR-aware rejection runs first for an IRR-owned name, and cover it with a test asserting the message |
| R-4 | Making the config commit reveals a second defect at lowering or apply, since nothing has ever reached that path | The functional test fails after validation is fixed | That is the point of the functional test in Phase 4; a second defect found there is in scope, because the goal is a working table term |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The verify-time set-reference guard, which today catches a genuine typo before it reaches the kernel. A careless widening turns a commit-time error into an apply-time failure of the whole reconcile |
| How is it reverted? | Single commit revert; no persisted state, no config format change. The feature returns to being unusable, which is the current state |
| Who else touches this path? | `plan/spec-fixit-irr-empty-answer-clears-set.md` changes what the store returns and what the sets contain; this spec changes whether a config referencing them can be committed. They meet at the same functional test and should be sequenced, not merged |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A config with a table term using `source-asn` | → | `ValidateTables` in `internal/component/firewall/validate.go` | `TestValidateTablesAcceptsIRRTermMatch` |
| A config naming a set that no owner provides | → | the same guard | `TestValidateTablesStillRefusesUnknownSet` |
| A config naming an AS-SET with no cached data | → | the IRR plugin's verify hook | `TestVerifyRejectsUncachedTableTerm` |
| A commit of a table term with cached data | → | the configure path and the registry merge | `TestConfigureAcceptsIRRTableTerm` |
| An operator commits the documented workflow on a running daemon | → | the whole chain to the kernel ruleset | `test/plugin/firewall-irr-table-term-commit.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A table term with `source-asn` and cached prefix data | The config validates, commits, and the kernel ruleset carries the rule |
| AC-2 | The same with `source-as-set`, `destination-asn` and `destination-as-set` | Same outcome for each of the four leaves |
| AC-3 | A table term naming an IRR entry with no cached data | The commit is refused by the IRR-aware check, with a message naming the entry and the command to fetch it, exactly as the guide promises |
| AC-4 | A term naming a set that no owner provides | The commit is refused with the existing unknown-set message |
| AC-5 | A term whose match field disagrees with the set type | Refused as today |
| AC-6 | An interface binding, unchanged | Behaves exactly as before |
| AC-7 | The documented workflow in the firewall guide is followed end to end | Every step succeeds as written |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Fetches an AS-SET, then commits a table term filtering on it | update command → store → config commit → validate → apply → kernel | `test/plugin/firewall-irr-table-term-commit.ci` |
| 2 | Commits a term for an AS-SET they forgot to fetch | config commit → IRR-aware verify → actionable refusal | `TestVerifyRejectsUncachedTableTerm` |
| 3 | Mistypes a named set in an ordinary term | config commit → unknown-set refusal | `TestValidateTablesStillRefusesUnknownSet` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestValidateTablesAcceptsIRRTermMatch` | `internal/component/firewall/validate_test.go` | AC-1: the parsed IRR term passes validation | passes; red before the fix |
| `TestValidateTablesStillRefusesUnknownSet` | `internal/component/firewall/validate_test.go` | AC-4, validates A-3 | passes; red under a name-prefix exemption, measured |
| `TestValidateTablesStillRefusesSetTypeMismatch` | `internal/component/firewall/validate_test.go` | AC-5 | passes; red before the fix |
| `TestParseAndValidateSourceASN` | `internal/component/firewall/config_test.go` | the gap that let this survive: parse plus validate in one test, for all four leaves | passes; red before the fix, all four leaves |
| `TestVerifyRejectsUncachedTableTerm` | `internal/component/firewall/plugins/irr/verify_test.go` | AC-3 | passes |
| `TestConfigureAcceptsIRRTableTerm` | `internal/component/firewall/engine_test.go` | AC-1 through the configure path, not only the verify one | passes; red before the fix |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `source-asn` / `destination-asn` | 1 to 4294967294 | 4294967294 | 0 | 4294967295 |
| IRR sets referenced by one table | 0 upward | no fixed limit | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `firewall-irr-table-term-commit` | `test/plugin/firewall-irr-table-term-commit.ci` | the documented workflow: fetch the AS-SET, commit a table term matching it, and see the rule in the kernel ruleset | passes under `unshare -Urn`; red with the fix reverted (`config verify failed: ... match references unknown set`) |
| `firewall-irr-table-term-uncached-reject` | `test/plugin/firewall-irr-table-term-uncached-reject.ci` | committing a table term for an unfetched AS-SET is refused with the IRR message | passes; its `reject=stderr:pattern=references unknown set` fires with the fix reverted | 

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | No wire-visible protocol behavior; this is config validation and local filtering | |

## Files to Modify
- `internal/component/firewall/validate.go` - let the set-reference guard see set names provided by another registered owner, without accepting an unknown name
- `internal/component/firewall/registry.go` - expose the registered owners' set names to validation, since the registry is what already knows them
- `internal/component/firewall/plugins/irr/irr.go` - register the IRR set names, and make the uncached-reference rejection cover table terms with its actionable message
- `internal/component/firewall/engine.go` - order the checks so the IRR-aware message wins for an IRR-owned name
- `internal/component/firewall/config_test.go` - the parser test must validate as well as parse
- `docs/guide/firewall.md` - the workflow and the rejection promise become true; state which errors an operator can see
- `docs/architecture/firewall/firewall-irr.md` - document that set names are registered for validation and merged at apply

## Files to Create
- `internal/component/firewall/validate_test.go` - if the package has no dedicated validation test file, this is where the new guard tests live
- `test/plugin/firewall-irr-table-term-commit.ci` - functional proof for AC-1 and AC-7
- `test/plugin/firewall-irr-table-term-uncached-reject.ci` - functional proof for AC-3

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | the four leaves already exist and already parse |
| YANG validation constraints | N-A | the ASN range constraints are already native |
| YANG custom validators | No | the check belongs at the firewall component's verify stage, where the owner registry is reachable; a YANG validator cannot see another owner's sets |
| CLI commands/flags | N-A | no new command |
| CLI grammar (keyword before value) | N-A | no grammar change |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | Yes | the two new `.ci` files |
| Pipe completeness | N-A | no new command output |
| Env var registration | N-A | no `environment/` leaf |
| Doctor check for runtime dependencies | No | no new runtime dependency; the failure is a config-time guard, covered by the functional tests |
| Prometheus counters/metrics | No | a commit-time rejection is reported to the operator directly, not counted |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | the feature is documented already; this makes it work |
| 2 | Config syntax changed? | No | the syntax is unchanged; what changes is whether it commits |
| 3 | CLI command added/changed? | No | no command surface |
| 4 | API/RPC added/changed? | No | no API surface |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` if the IRR plugin gains a registration duty |
| 6 | Has a user guide page? | Yes | `docs/guide/firewall.md`, the IRR prefix-list section |
| 7 | Wire format changed? | No | no wire format |
| 8 | Plugin SDK/protocol changed? | No | no SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | No | BCP 38 is named in the guide as intent, and no RFC checklist row changes |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if the mock IRR fixture is extended for the table-term tests |
| 11 | Affects daemon comparison? | No | no comparison claim |
| 12 | Internal architecture changed? | Yes | `docs/architecture/firewall/firewall-irr.md` |
| 13 | Route metadata keys added/changed? | No | no metadata key |
| 14 | Prometheus counters added/changed? | No | none added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | the IRR plugin registers set names for validation; `docs/features/plugins.md` if the registration is operator-visible |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/features.md` anchors `config.go` and `irrSetMatch`, `engine.go`, and the IRR plugin; `docs/guide/firewall.md` anchors `irr.go` and `sets.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the guide's step 3 is exactly the config this spec makes work, and it must be shown as a complete example including the table |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- pin the defect and settle A-1, A-2 and A-4
   - Tests: `TestParseAndValidateSourceASN`, `TestValidateTablesAcceptsIRRTermMatch`
   - Files: `internal/component/firewall/config_test.go`, `internal/component/firewall/validate_test.go`
   - Verify: both fail today with the unknown-set message; the runtime reproduction is repeated with data cached, and the result is recorded against A-1
2. **Phase: let validation see registered set names** -- registration, not a prefix literal
   - Tests: `TestValidateTablesAcceptsIRRTermMatch`, `TestValidateTablesStillRefusesUnknownSet`, `TestValidateTablesStillRefusesSetTypeMismatch`
   - Files: `internal/component/firewall/registry.go`, `internal/component/firewall/validate.go`, `internal/component/firewall/plugins/irr/irr.go`
   - Verify: an IRR term validates, an unknown name is still refused, and the firewall component holds no string naming the plugin
3. **Phase: the actionable message** -- IRR-aware rejection for table terms
   - Tests: `TestVerifyRejectsUncachedTableTerm`, `TestConfigureAcceptsIRRTableTerm`
   - Files: `internal/component/firewall/plugins/irr/irr.go`, `internal/component/firewall/engine.go`
   - Verify: an uncached reference produces the documented message, not the set-reference one
4. **Phase: functional proof, and whatever it uncovers**
   - Tests: the two new `.ci` files
   - Files: `test/plugin/`
   - Verify: the documented workflow completes end to end and the rule is present in the kernel ruleset. Per R-4, a further defect found on this newly reachable path is in scope
5. **Phase: correct the documentation**
   - Tests: the doc gates
   - Files: `docs/guide/firewall.md`, `docs/architecture/firewall/firewall-irr.md`
   - Verify: the guide's workflow matches what the code does, including the errors an operator can see

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Feature completeness | All four IRR leaves work in the table form, not only `source-asn` |
| Correctness | The unknown-set guard still refuses a genuine typo, proven by a test that would pass under a naive exemption |
| Naming | Set names come from one place; the component never spells the plugin's prefix |
| Data flow | Validation learns names through the registry, not by importing the plugin |
| Rule: `ai/rules/plugins.md` | No plugin spelling enters a central package |
| Rule: `ai/rules/evidence.md` | The guard fails closed; an unregistered name is refused, not tolerated |
| Registration over hardcoding | The set-name source is a registration the core discovers |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The documented workflow commits | run `ze config validate` on the guide's example and get exit 0 |
| The guard still refuses a typo | `TestValidateTablesStillRefusesUnknownSet` passes |
| Actionable IRR error | `TestVerifyRejectsUncachedTableTerm` asserts the message text |
| No plugin spelling in the component | grep the firewall component for the IRR set-name prefix; only the parser's existing constant may remain, and Phase 2 decides whether even that moves |
| Functional proof | the plugin suite runs both new `.ci` files green |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail closed | A set name nobody provides must be refused. The failure mode to avoid is an exemption that lets an unresolvable match reach the backend, where it aborts the whole reconcile |
| Input validation | AS-SET names are operator input that become set names; the character set accepted must be the one the backend can carry |
| Availability | A filtering rule that silently does not exist is worse than a refused commit; the commit must either work or say why |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A-1 refuted (cached data changes the outcome) | Re-derive the defect from the new evidence before designing; the shape of the fix depends on whether the check is blind or merely early |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The MATCH states the type of the set another owner provides (`MatchInSet.ProvidedType`), set by the config parser for the four IRR leaves and by nothing else | The approved design, a registered provider of set names; exempt names carrying the IRR prefix; emit placeholder sets from the parser; move validation to the merged view at apply | Implementation deviation from the approved design, taken because a registration cannot be exact. A provider registered at init can only claim a NAMESPACE, since the names it will supply depend on a config it has not seen; it therefore accepts `source-address "@irr_v4_typo"` typed by hand, and that unresolvable match aborts the whole reconcile at the backend, which is the failure mode this spec's Security Review names. A provider registered from the config cannot be consulted safely, because the owners register in no fixed order. The parser already spells the IRR set names and is the only thing that knows a match names another owner's set, so it declares the type it expects. No new plugin spelling enters the component, the guard stays exact per term, and nothing depends on registration order |
| `ApplyAll` holds back the TABLE naming an unregistered provided set and programs every other table, rather than handing the backend a rule it must refuse | Order the plugin apply hooks; register the sets at verify; drop the unresolved term from the applied snapshot; hold back the whole merged snapshot | R-4 fired: with validation fixed, the firewall owner applied first and the backend answered `match-in-set: unknown set "irr_v4_AS-TEST" (not registered on table)`, failing the reconcile for every owner. The order cannot be relied on: at startup the firewall engine configures before the plugin that depends on it, and in a reload transaction the participants apply in whatever order the orchestrator emits. Registering at verify leaves desired state ahead of a transaction that may abort, and it does nothing for startup, where no verify runs. Dropping the term applies a policy the operator did not write. **Amended 2026-08-19.** This row read "`ApplyAll` waits" and scoped the wait to the whole merged snapshot. That wait had no end state on a cold cache. The owner whose registration releases it is the IRR plugin, and a fresh install, a wiped `database.zefs` or a store that fails to open leaves it with no set to register, so the operator's tables, copp, the DDoS tables and the policy routes all stayed out of the kernel behind one WARN. `dropTablesMissingAProvidedSet` (`internal/component/firewall/registry.go`) now removes only the table naming the missing set. One table is the smallest unit that can wait, and it needs no fixpoint: an nftables set is table-local, so a set another table declares can never resolve this table's term. Landed in `20ad3ebb6`. The independent review of this spec confirmed the narrower scope and that `test/plugin/firewall-irr-cold-cache-recovers.ci` discriminates. Recorded here because the code diverged from this row before the row was corrected: the process failure is the unamended record, not the decision |
| Keep the verify-time check and add to it | Replace it | The check catches typos before anything reaches the kernel, which is worth keeping; it is not wrong, it is under-informed |
| Fix the documentation in the same spec | Leave the guide alone | The guide currently teaches a config that cannot be committed, which is its own defect class in this repository, and it is the acceptance statement for AC-7 |

## Known Limitations
- Validation learns set NAMES, not contents. A name that is registered but whose data is empty is a different problem, owned by the IRR store spec.
- This spec makes the table-term path reachable. Anything that path meets for the first time at lowering or apply is discovered by the Phase 4 functional test, and R-4 records that finding it is in scope rather than a surprise.
- An IRR term works in a table whose family is `inet`, and is refused in `ip` or `ip6`. One leaf emits a v4 match and its IPv6 twin, and an address set of the wrong family in a single-family table would lower against the wrong header bytes. The refusal is `validateSetFamilyCompat`'s existing message, which names the family to use. `buildIRRTables` also registers its tables as `inet` only, so an `ip` table would never receive the set. The guide now states the constraint.
- `firewall-irr-empty-answer-keeps-last-good.ci` and `firewall-irr-iface-no-blackhole.ci` stay red on `a refresh that learned nothing reported success`. Both were red before this spec, for the command-argument defect this spec fixed; the fix moved them onto an assertion that `plan/spec-fixit-irr-empty-answer-clears-set.md` owns. Journalled under `plan/journal/zero-value-as-valid-answer.md`.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
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
