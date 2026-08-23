# Spec: fixit-firewall-irr-term-fails-validation

| Field | Value |
|-------|-------|
| Status | done |
| Scope | config |
| Depends | - |
| Phase | 5/5 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-23 |

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
| Who else touches this path? | `spec-fixit-irr-empty-answer-clears-set` changed what the store returns and what the sets contain; this spec changes whether a config referencing them can be committed. They meet at the same functional test and were sequenced, not merged. That spec CLOSED on 2026-08-22, so its stem is written bare here: a `plan/` path would cite a file the tree no longer holds |

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
| `TestReconfigureKeepsFetchedPrefixes` | `internal/component/firewall/plugins/irr/irr_test.go` | AC-1, AC-7: the prefixes a fetch cached survive the reload that commits the term naming them, and the set reaches the backend with elements | passes; red before the fix (`the reload dropped the prefixes the fetch cached: <nil>`) |
| `TestReconfigureToAnotherServerKeepsFetchedPrefixes` | `internal/component/firewall/plugins/irr/irr_test.go` | AC-1 on the one path a reuse-only-when-the-server-is-unchanged fix would leave open, and that the next refresh queries the server the reload named | passes; red before the fix |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `source-asn` / `destination-asn` | 1 to 4294967294 | 4294967294 | 0 | 4294967295 |
| IRR sets referenced by one table | 0 upward | no fixed limit | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `firewall-irr-table-term-commit` | `test/plugin/firewall-irr-table-term-commit.ci` | the documented workflow: fetch the AS-SET, commit a table term matching it, and see the rule in the kernel ruleset | passes under `unshare -Urn` against a binary whose config directory holds no `database.zefs`, which is what the functional suite builds; red with the validation fix reverted (`config verify failed: ... match references unknown set`), and red with the store-lifetime fix reverted (`the committed IRR term is not in the kernel ruleset`, then `firewallnft: table "ze_wan" not found`) |
| `firewall-irr-table-term-uncached-reject` | `test/plugin/firewall-irr-table-term-uncached-reject.ci` | committing a table term for an unfetched AS-SET is refused with the IRR message | passes; its `reject=stderr:pattern=references unknown set` fires with the fix reverted | 

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | No wire-visible protocol behavior; this is config validation and local filtering | |

## Files to Modify
- `internal/component/resolve/irr/store/store.go` - the store keeps its cache when a reload moves the IRR server or the PeeringDB URL (`UseClients`), instead of the consumer building a second store
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
| 12 | Internal architecture changed? | Yes | `docs/architecture/firewall/firewall-irr.md`. Two further design docs are declared by the `// Design:` headers of files this spec changes, and both are unaffected. `docs/architecture/core-design.md` is named by `validate.go` and `registry.go`; it describes verify-time validation as a stage, and this spec changes what one guard can SEE rather than when the stage runs. `docs/architecture/resolve.md` is named by `store/store.go`; it describes the resolver's cache, and the change keeps that cache across a client swap rather than altering its model. Named here so the anchor check reads a decision rather than an omission |
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
| The prefix store lives as long as the plugin, and a reload points it at the new servers (`PrefixStore.UseClients`) rather than building a second one | Keep the store only when the server is unchanged; copy the entries into the new store; seed `database.zefs` in the test harness | `configure` built a store on every apply, so a commit verify had accepted against the populated store applied against an empty one: no set was registered, `ApplyAll` held back the operator's table, and the commit reported success with nothing in the kernel. Keeping the store only when the server is unchanged leaves the same fail-open on a server edit, and it is correctness, not machinery, so simplicity does not buy it. Copying entries keeps two stores alive, and a refresh that started against the old one then lands in the discarded one. Seeding the file in the harness would turn the test green and leave the daemon losing its prefixes on reload |
| `dropTablesMissingAProvidedSet` keeps its WARN; it does not fail the apply | Fail `ApplyAll` when a provided set is unresolved | The fail-closed answer to "the data is not there" is `verifyRefs` at verify time, which refuses the commit and names the entry and the fetch command, and it is now trustworthy because the store the apply uses is the store verify read. Failing the apply cannot replace it: within one transaction the firewall engine applies BEFORE the plugin registers its sets (measured, `table held back` fires at the engine's apply and the table is programmed at the plugin's), so failing there would refuse every commit that adds an IRR term. At startup no verify runs at all, and an error from `OnConfigure` would take copp, the DDoS tables and the policy routes out with it. The startup path says why instead: `warnUncachedRefs` logs the same entry name and fetch command that a commit would have been refused with |
| Keep the verify-time check and add to it | Replace it | The check catches typos before anything reaches the kernel, which is worth keeping; it is not wrong, it is under-informed |
| Fix the documentation in the same spec | Leave the guide alone | The guide currently teaches a config that cannot be committed, which is its own defect class in this repository, and it is the acceptance statement for AC-7 |

## Known Limitations
- Validation learns set NAMES, not contents. A name that is registered but whose data is empty is a different problem, owned by the IRR store spec.
- This spec makes the table-term path reachable. Anything that path meets for the first time at lowering or apply is discovered by the Phase 4 functional test, and R-4 records that finding it is in scope rather than a surprise.
- An IRR term works in a table whose family is `inet`, and is refused in `ip` or `ip6`. One leaf emits a v4 match and its IPv6 twin, and an address set of the wrong family in a single-family table would lower against the wrong header bytes. The refusal is `validateSetFamilyCompat`'s existing message, which names the family to use. `buildIRRTables` also registers its tables as `inet` only, so an `ip` table would never receive the set. The guide now states the constraint.
- `firewall-irr-empty-answer-keeps-last-good.ci` and `firewall-irr-iface-no-blackhole.ci` stay red on `a refresh that learned nothing reported success`. Both were red before this spec, for the command-argument defect this spec fixed; the fix moved them onto an assertion that `spec-fixit-irr-empty-answer-clears-set` owned, and that spec CLOSED on 2026-08-22, so both are green now. Its stem is bare here because a `plan/` path would cite a file the tree no longer holds. Journalled under `plan/journal/zero-value-as-valid-answer.md`. Their producer is now measured: both log one `configured` and two `firewall-irr: refreshed ... ipv4=3 ipv6=1`, so the second refresh never reached the server that answers nothing. `IRR.LookupPrefixes` (`internal/component/resolve/irr/client.go`) serves it from the `cacheTTL = time.Hour` cache, which is AC-8 of that spec.

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

---

## Implementation Summary

### What Was Implemented
- `MatchInSet.ProvidedType` (`internal/component/firewall/model.go`) states the element type of a set a DIFFERENT registry owner supplies. `irrSetMatch` and `expandIRRTermV6` (`internal/component/firewall/config.go`) are its only producers, for the four IRR leaves alone.
- `validateMatch` (`internal/component/firewall/validate.go`) accepts a match carrying it and checks the field and family against that type. A match without it still has to name a set the table declares, so the unknown-set guard is unchanged for everything an operator can type.
- `dropTablesMissingAProvidedSet` and `unresolvedProvidedSet` (`internal/component/firewall/registry.go`) hold back the one table whose term names a provided set nobody registered, program the rest, and log `table held back` at WARN.
- `PrefixStore.UseClients` (`internal/component/resolve/irr/store/store.go`) points the store at new resolvers on a reload instead of `configureStore` building a second one, so the store verify read is the store the apply uses.
- `verifyRefs` and `warnUncachedRefs` (`internal/component/firewall/plugins/irr/irr.go`) cover table terms, so the actionable IRR message reaches the table form; `extractRefsFromConfig` (`config.go`) reads all four leaves out of every table term.
- `buildTermSets` and `familySet` (`internal/component/firewall/plugins/irr/sets.go`) declare BOTH family sets for any entry the plugin holds data for. Found by the closure review; see Bugs Found/Fixed.
- Docs: `docs/guide/firewall.md` states the workflow, the errors an operator can see and the `inet` constraint; `docs/architecture/firewall/firewall-irr.md` documents the provided-type register, the per-table wait, and the family pairing.

### Bugs Found/Fixed
- **An ASN or AS-SET announcing one family cost the operator the WHOLE table.** Found by this closure's review, reproduced, and fixed here. `expandIRRTermV6` emits an IPv6 twin of every IRR term because the parser cannot see the prefix data, while `buildSets` emitted a set only for a family that announced prefixes. So for an IPv4-only entry the twin named `irr_v6_<name>`, no owner declared it, and `dropTablesMissingAProvidedSet` removed the operator's entire table from the applied snapshot while the commit reported success. An AS-SET with no IPv6 space is the ordinary case, and the AS-TEST fixture answers both families, so nothing reached it. `buildTermSets` now declares both sets, the family with no prefixes carrying no elements, which is what its term must read. Covered by `TestBuildTermSetsPairsTheFamilies`, `TestConfigureAcceptsIRRTableTermWithOneFamily`, and the AS-V4ONLY half of `test/plugin/firewall-irr-table-term-commit.ci`.

### Documentation Updates
- `docs/architecture/firewall/firewall-irr.md`: new section "A table term declares both families or neither", anchored on `sets.go -- buildTermSets` and `config.go -- expandIRRTermV6`.
- `docs/guide/firewall.md`: the workflow, the `table held back` line and how to end it, and the `inet` family constraint. Landed in `e049941d6`.
- `make ze-doc-verify`: PASSED, 3025 digest anchors resolve.

### Deviations from Plan
- The approved design was a registered provider of set names. The implementation states the type on the MATCH instead. A provider registered at init can only claim a NAMESPACE, so it would accept `source-address "@irr_v4_typo"` typed by hand; a provider registered from config cannot be consulted safely because owners register in no fixed order. Recorded in Key Design Decisions.
- `ApplyAll` holds back one TABLE rather than the whole merged snapshot. Recorded and amended in Key Design Decisions.
- `engine.go` needed no check reordering. Validation stops refusing an IRR name, so the plugin's verify hook is what speaks for an uncached reference.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-3 assumed validation could consult a registered provider of set names without weakening the guard | A registration can only claim a namespace, which accepts a hand-typed name inside it | Phase 2 design | The match states the type instead; recorded in Key Design Decisions |
| approach | The first `ApplyAll` answer held back the WHOLE merged snapshot until every provided set resolved | On a cold cache no supplier ever arrives, so copp, the DDoS tables and the policy routes stayed out of the kernel too | `firewall-irr-cold-cache-recovers.ci` | Narrowed to one table in `20ad3ebb6` |
| escalation | The two producers of the IRR family pair disagreed: the parser emits both terms unconditionally, the plugin emitted sets per announced family | An IPv4-only entry is ordinary, so the common case lost its whole table | This closure's review, reproduced through `ApplyAll` | Fixed by `buildTermSets`; journalled under `guard-added-to-one-half-of-a-pair` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A table term matching by ASN or AS-SET can be committed | Done | `validateMatch` (`internal/component/firewall/validate.go`) | `ProvidedType` carries the type the supplying owner will register |
| The documented operator workflow is a config the parser accepts | Done | `docs/guide/firewall.md` | Proven end to end by `test/plugin/firewall-irr-table-term-commit.ci` |
| The actionable IRR error reaches the table form | Done | `verifyRefs`, `uncachedRefMessage` (`internal/component/firewall/plugins/irr/irr.go`) | `extractRefsFromConfig` reads the four leaves from table terms |
| The interface-binding form is unchanged | Done | `buildIfaceTables` (`internal/component/firewall/plugins/irr/sets.go`) | Still uses `buildSets`; it emits an accept term per family |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestValidateTablesAcceptsIRRTermMatch`, `TestConfigureAcceptsIRRTableTerm`, `TestConfigureAcceptsIRRTableTermWithOneFamily`, `firewall-irr-table-term-commit.ci` | Both the dual-stack and the single-family entry reach the kernel |
| AC-2 | Done | `TestParseAndValidateSourceASN` (four subtests), `TestVerifyRejectsUncachedTableTerm` | All four leaves parse, validate and produce a v4 match plus its v6 twin |
| AC-3 | Done | `TestVerifyRejectsUncachedTableTerm`, `firewall-irr-table-term-uncached-reject.ci` | Message names the entry and the fetch command |
| AC-4 | Done | `TestValidateTablesStillRefusesUnknownSet` | Also refuses a hand-written name inside the IRR namespace |
| AC-5 | Done | `TestValidateTablesStillRefusesSetTypeMismatch` | Field/type and set/table family disagreements both still refuse |
| AC-6 | Done | `firewall-irr-iface-commit.ci`, `firewall-irr-iface-reject.ci`, `firewall-irr-iface-no-blackhole.ci` | All three pass; `buildIfaceTables` is untouched |
| AC-7 | Done | `firewall-irr-table-term-commit.ci`, `docs/guide/firewall.md` | The guide's step 3 config is the one the test commits |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestValidateTablesAcceptsIRRTermMatch` | Done | `internal/component/firewall/validate_test.go` | |
| `TestValidateTablesStillRefusesUnknownSet` | Done | `internal/component/firewall/validate_test.go` | The negative test A-3 owes |
| `TestValidateTablesStillRefusesSetTypeMismatch` | Done | `internal/component/firewall/validate_test.go` | |
| `TestParseAndValidateSourceASN` | Done | `internal/component/firewall/config_test.go` | |
| `TestVerifyRejectsUncachedTableTerm` | Done | `internal/component/firewall/plugins/irr/verify_test.go` | |
| `TestConfigureAcceptsIRRTableTerm` | Done | `internal/component/firewall/engine_test.go` | |
| `TestConfigureAcceptsIRRTableTermWithOneFamily` | Done | `internal/component/firewall/engine_test.go` | Added by this closure |
| `TestBuildTermSetsPairsTheFamilies` | Done | `internal/component/firewall/plugins/irr/sets_test.go` | Added by this closure |
| `TestReconfigureKeepsFetchedPrefixes` | Done | `internal/component/firewall/plugins/irr/irr_test.go` | Assertion strengthened to the family pair |
| `TestReconfigureToAnotherServerKeepsFetchedPrefixes` | Done | `internal/component/firewall/plugins/irr/irr_test.go` | |
| `firewall-irr-table-term-commit` | Done | `test/plugin/firewall-irr-table-term-commit.ci` | Two AS-SETs: dual-stack and IPv4-only |
| `firewall-irr-table-term-uncached-reject` | Done | `test/plugin/firewall-irr-table-term-uncached-reject.ci` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/resolve/irr/store/store.go` | Done | `UseClients` |
| `internal/component/firewall/validate.go` | Done | `validateMatch` reads `ProvidedType` |
| `internal/component/firewall/registry.go` | Done | `dropTablesMissingAProvidedSet` replaced the registry-exposure plan |
| `internal/component/firewall/plugins/irr/irr.go` | Done | `verifyRefs`, `warnUncachedRefs`, `configureStore` |
| `internal/component/firewall/engine.go` | Changed | No ordering change was needed; the file lost its `StoreLastApplied` calls to `ApplyAll` instead |
| `internal/component/firewall/config_test.go` | Done | `TestParseAndValidateSourceASN` validates as well as parses |
| `docs/guide/firewall.md` | Done | |
| `docs/architecture/firewall/firewall-irr.md` | Done | |
| `internal/component/firewall/validate_test.go` | Done | Created |
| `test/plugin/firewall-irr-table-term-commit.ci` | Done | Created |
| `test/plugin/firewall-irr-table-term-uncached-reject.ci` | Done | Created |
| `internal/component/firewall/plugins/irr/sets.go` | Changed | Not in the plan: `buildTermSets`, added by the closure review |
| `internal/test/mock/irr/irr.go` | Changed | Not in the plan: the AS-V4ONLY fixture the single-family case needs |

### Audit Summary
- **Total items:** 36
- **Done:** 33
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3, recorded in Deviations and in Files from Plan

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A firewall table term matching by ASN or AS-SET can be committed at all | functional | `test/plugin/firewall-irr-table-term-commit.ci` PASS in a privileged Linux container, 2026-08-23. Reverting `buildTermSets` to `buildSets` reds it: `firewall: table "wan" not found; no firewall tables have been applied` |
| The kernel ruleset carries the committed rule | functional | The same test reads back `from-as-test`, `from-as-test_v6`, `from-v4only` and `from-v4only_v6` through `show firewall ruleset wan` |
| The documented operator workflow succeeds as written | functional | The `.ci` config is the guide's step 3; `make ze-doc-verify` PASSED |
| The commit-rejection promise the guide makes is reachable | functional | `test/plugin/firewall-irr-table-term-uncached-reject.ci` PASS; `TestVerifyRejectsUncachedTableTerm` asserts the exact message |
| The unknown-set guard still refuses a typo | unit | `TestValidateTablesStillRefusesUnknownSet`, which also refuses `source-address "@irr_v4_AS13335"` typed by hand |
| The interface-binding form is unaffected | functional | `firewall-irr-iface-commit`, `firewall-irr-iface-reject` and `firewall-irr-iface-no-blackhole` all PASS |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | done | The spec metadata declares no deferral shard, and `plan/deferrals/fixit-firewall-irr-term-fails-validation.md` does not exist |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-firewall-irr-term-fails-validation-ebf5d9b3-b158-40df-bba0-32a51591883e.md` |
| `review_gate.py check` | clean (`review_gate: OK ... hashes match`) |
| Rounds | 2 |
| Reviewer lenses used | wiring and functional coverage; guard audit and fail-closed; removed-behavior and test-rewrite audit; logic, allocation and style pass |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | An ASN or AS-SET announcing one family lost the operator's WHOLE table. The parser emits an IPv6 twin of every IRR term while the plugin declared a set only per announced family, so the twin named a set no owner declared and `dropTablesMissingAProvidedSet` removed the table while the commit reported success | `buildIRRTables`, `buildSets` (`internal/component/firewall/plugins/irr/sets.go`) | `buildTermSets` declares both family sets for any entry with data, the family with none declared empty. Tests: `TestBuildTermSetsPairsTheFamilies`, `TestConfigureAcceptsIRRTableTermWithOneFamily`, AS-V4ONLY in `firewall-irr-table-term-commit.ci` |
| 2 | ISSUE | `TestBuildSetsEmptyBoth` claimed to prevent "buildSets emitting an empty set for a family that answered nothing", which the fix makes deliberate for a table term | `internal/component/firewall/plugins/irr/sets_test.go` | Comment rewritten to the invariant that survives: an entry announcing NOTHING yields no set and no table |
| 3 | ISSUE | Two tests pinned the old one-set contract against an IPv4-only fixture, so each would have passed a table missing its IPv6 set | `TestRefreshNameProgramsWhatItLearned`, `TestReconfigureKeepsFetchedPrefixes` (`internal/component/firewall/plugins/irr/irr_test.go`) | `assertTermSets` asserts the stronger contract: both sets declared, the announced family carrying elements |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/firewall/validate_test.go` | Yes | `TestValidateTablesAcceptsIRRTermMatch` at :511, `TestValidateTablesStillRefusesUnknownSet` at :536, `TestValidateTablesStillRefusesSetTypeMismatch` at :566 |
| `test/plugin/firewall-irr-table-term-commit.ci` | Yes | `ls test/plugin/` lists it, 6.5K after the AS-V4ONLY addition |
| `test/plugin/firewall-irr-table-term-uncached-reject.ci` | Yes | `ls test/plugin/` lists it, 4.9K |
| `test/plugin/firewall-irr-cold-cache-recovers.ci` | Yes | `ls test/plugin/` lists it, 6.5K |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | A cached table term validates, commits and reaches the kernel | `make ze-unit-pkg-test PKG=./internal/component/firewall/...` ok 1.990s; `firewall-irr-table-term-commit` PASS 906ms in the container |
| AC-2 | All four leaves behave the same | `TestParseAndValidateSourceASN` runs `source-asn`, `source-as-set`, `destination-asn` and `destination-as-set`, and validates each |
| AC-3 | An uncached reference is refused with the IRR message | `TestVerifyRejectsUncachedTableTerm` asserts `firewall irr: no cached prefix data for AS99999; run 'update firewall irr asn 99999' first`; `firewall-irr-table-term-uncached-reject` PASS 6.0s |
| AC-4 | A set no owner provides is still refused | `TestValidateTablesStillRefusesUnknownSet` requires `match references unknown set "typo_set"`, and the same for a hand-written `@irr_v4_AS13335` |
| AC-5 | Field and family disagreements still refuse | `TestValidateTablesStillRefusesSetTypeMismatch`, three cases |
| AC-6 | The interface form is unchanged | `firewall-irr-iface-commit`, `firewall-irr-iface-reject` and `firewall-irr-iface-no-blackhole` PASS in the 12/12 suite run |
| AC-7 | The guide's workflow succeeds end to end | `firewall-irr-table-term-commit` PASS; `make ze-doc-verify` PASSED |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A config with a table term using `source-asn` | `test/plugin/firewall-irr-table-term-commit.ci` | Yes. Read: `config2.conf` carries `term from-as-test` and `term from-v4only`, and the observer reads the ruleset back through `show firewall ruleset wan` |
| A config naming a set no owner provides | unit, `TestValidateTablesStillRefusesUnknownSet` | Yes. Driven from `ValidateTables`, which is what `parseAndVerifyFirewallSections` calls |
| A config naming an AS-SET with no cached data | `test/plugin/firewall-irr-table-term-uncached-reject.ci` | Yes. `reject=stderr:pattern=references unknown set` proves the IRR message wins, not the set-reference one |
| A commit of a table term with cached data | unit, `TestConfigureAcceptsIRRTableTerm` and `TestConfigureAcceptsIRRTableTermWithOneFamily` | Yes. Both drive `parseAndVerifyFirewallSections`, then `RegisterTables` plus `ApplyAll` |
| The operator commits the documented workflow on a running daemon | `test/plugin/firewall-irr-table-term-commit.ci` | Yes. SIGHUP reload, then a kernel readback |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | The rejection was unconditional: `firewall-irr-table-term-commit.ci` fetches first and the commit was still refused with the fix reverted |
| A-2 | confirmed | `firewall-irr-iface-commit` and `firewall-irr-iface-reject` carry no firewall table and both pass |
| A-3 | broken as stated | A registration can only claim a namespace. `MatchInSet.ProvidedType` replaced it, and `TestValidateTablesStillRefusesUnknownSet` proves the guard did not become an exemption |
| A-4 | confirmed | `MatchInSet` has one other producer, `parseAddressMatch` in `internal/plugins/policyroute/translate.go`, and it sets no `ProvidedType`. copp and ddos-local emit none |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| "A table term declares both families or neither" | `buildTermSets` (`internal/component/firewall/plugins/irr/sets.go`), read after writing | Yes |
| "the table holding it is not programmed and the daemon logs `table held back`" | `dropTablesMissingAProvidedSet` (`internal/component/firewall/registry.go`) emits that exact WARN | Yes |
| "the table family must be `inet`" | `validateSetFamilyCompat` refuses an ipv6 set in `ip` and an ipv4 set in `ip6`; `buildIRRTables` registers `FamilyInet` | Yes |
| "the refusal names the entry and the command that fetches it" | `uncachedRefMessage` (`internal/component/firewall/plugins/irr/irr.go`) | Yes |
| Doc gates | `make ze-doc-verify` PASSED, 3025 digest anchors resolve | Yes |

## Core Insight

Two producers derived one paired artifact by different rules. The config parser
emits an IPv6 twin of every IRR term because it cannot see the prefix data, and
the plugin emitted a set only for a family that announced prefixes. Neither is
wrong alone, and the pair is broken for the ordinary input: an AS-SET with no
IPv6 space. The fixture answered both families, so every test agreed with both
producers and none of them tested the pair.

The rule that falls out: when one producer emits a pair unconditionally, its
counterpart must too, and the fixture must carry the asymmetric case.
