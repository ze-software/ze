# Spec: fixit-irr-empty-answer-clears-set

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

An IRR server that answers "I have nothing" is treated as an IRR server that
answers "the answer is nothing", and the difference costs an interface.

`(*IRR).query` returns the response bytes with a nil error for any reply it can
read. `lookupFamilyPrefixes` skips every word it cannot parse as a prefix, so the
key-not-found marker, the query-ok-zero-results marker and a server-side error
reply all reduce to an empty list with no error. `LookupPrefixes` then caches
that empty answer for an hour. `(*PrefixStore).Refresh` keeps the previous data
only when it gets an error, so an empty success overwrites a populated entry and
persists the emptied version.

The consequence differs by consumer, and both are severe:

- Interface bindings: `buildIfaceTables` gates the accept terms on a non-empty
  prefix list, and emits the drop term unconditionally. An emptied entry
  therefore produces a table that drops everything arriving on that interface,
  and the apply SUCCEEDS. A transient upstream answer blackholes a port.
- Table terms: the set disappears from the merged tables, so `lowerMatchInSet`
  fails with an unknown set, and `Apply` returns before `Flush`. The kernel keeps
  the previous ruleset, and every later apply by any owner fails the same way
  until the set comes back.

A third consumer is outside this spec's files but shares the root cause: the BGP
IRR filter replaces its prefix list with the empty one and then rejects every
UPDATE from that peer.

Nothing reports it. The show command prints status ok with a count of zero, the
refresh outcome counter records success, and the last-refresh timestamp is
stamped whether or not anything was learned. The documentation claims
last-known-good persistence, which is the property this defect breaks.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/firewall/firewall-irr.md` - the IRR-to-firewall design, its fail-closed claim, and why auto-refresh ships off
  → Constraint: the page already names "invalid IRR data causing a firewall outage" as the risk this feature carries, so an empty answer accepted as data is the named risk realized
- [ ] `docs/guide/irr-filtering.md` - the operator-facing promise
  → Constraint: the page claims atomic refresh and last-known-good persistence; the fix must make that claim true rather than the claim be softened
- [ ] `docs/architecture/resolve.md` - where IRR resolution sits and what it may assume about upstreams
  → Decision: resolution is a cache in front of a third-party service, so an unreachable or unhelpful upstream is an expected state, not an exception

**Key insights:**
- The repository has already made this exact fix once, for RPKI: a reachable cache holding no usable data was being treated as authoritative, and the fix separated "configured" from "synced" and surfaced the difference. The same separation applies here.
- A `PrefixList.Empty` helper already exists and no producer calls it.
- The one-hour prefix cache means an operator who notices the outage and forces a refresh gets the same empty answer with no network query.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/resolve/irr/client.go` - `query` returns bytes and nil error for any readable reply; `lookupFamilyPrefixes` skips unparseable words including the not-found, ok-empty and error markers; `LookupPrefixes` aggregates, caches the result for an hour, and returns nil error; `PrefixList.Empty` exists and is unused
- [ ] `internal/component/resolve/irr/store/store.go` - `resolve` returns the previous data only on error; `Refresh` overwrites the cache entry and persists whenever the error is nil; `persist` writes to zefs
- [ ] `internal/component/firewall/plugins/irr/irr.go` - `refreshLoop` ticks on the configured interval; `refreshAllNow` logs and counts a failure and continues; the refresh outcome counter records success for an empty answer; the last-refresh gauge is stamped unconditionally
- [ ] `internal/component/firewall/plugins/irr/sets.go` - `buildSets` emits nothing for an empty family; `buildIRRTables` skips a table with no sets; `buildIfaceTables` gates the accept terms on a non-empty list and emits the drop term unconditionally
- [ ] `internal/component/firewall/plugins/irr/command.go` - the show command reports status ok and the counts, with no staleness notion
- [ ] `internal/plugins/firewall/nft/lower_linux.go` - `lowerMatchInSet` fails when the named set is absent from the merged tables
- [ ] `internal/plugins/firewall/nft/backend_linux.go` - `Apply` returns the lowering error before the single `Flush`, so the kernel keeps the previous ruleset
- [ ] `internal/component/resolve/irr/client_test.go` - `TestLookupPrefixesEmpty` asserts that empty plus nil error is correct, pinning the defect
- [ ] `internal/component/firewall/plugins/irr/sets_test.go` - `TestBuildSetsEmptyBoth` pins "zero sets" as correct for an empty entry
- [ ] `internal/component/firewall/plugins/irr/irr_test.go` - `TestRefreshFailureKeepsLastGood` is vacuous: it asserts a nil lookup before any refresh runs and discards the values it builds

**Behavior to preserve:**
- A transport failure keeps the previously known prefixes; that half already works and must stay.
- Auto-refresh stays off by default.
- The persisted zefs cache stays the source of last-known-good across restarts.
- An AS-SET that genuinely holds no prefixes must remain expressible; the fix must not make an empty result permanently unreachable.

**Behavior to change:**
- An empty answer stops overwriting a populated entry.
- An empty answer stops being cached as if it were data.
- An emptied or stale entry stops producing a blackhole table.
- A refresh that learned nothing is reported as such: in the show command, in the counters, and in a log line.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A whois query to an IRR server, driven by the refresh ticker or by an operator command.
- Format at entry: a text reply, one word per token, with markers for not-found, ok and error.

### Transformation Path
1. `query` in `internal/component/resolve/irr/client.go` reads the reply and returns the bytes
2. `lookupFamilyPrefixes` parses prefixes and silently skips every other word, markers included
3. `LookupPrefixes` aggregates, writes the result into the one-hour prefix cache, and returns nil error
4. `resolve` in `internal/component/resolve/irr/store/store.go` builds an entry; `Refresh` overwrites the in-memory cache and persists it to zefs
5. `buildSets`, `buildIRRTables` and `buildIfaceTables` in `internal/component/firewall/plugins/irr/sets.go` turn the entry into firewall sets and terms
6. `RegisterTables` and `ApplyAll` merge every owner's tables
7. The nft backend lowers and flushes, or fails at `lowerMatchInSet` and flushes nothing

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze ↔ IRR server | whois over TCP, text reply | No |
| Client ↔ store | `PrefixList` plus an error, where the error is the only signal that distinguishes failure | No |
| Store ↔ zefs | persisted last-known-good entry | No |
| Plugin ↔ firewall registry | `RegisterTables` under the IRR owner | No |
| Registry ↔ kernel | one flush per reconcile, all owners together | No |

### Integration Points
- `PrefixList.Empty` - the existing helper the guard should use
- the RPKI synced-versus-configured split - the precedent this fix follows
- `internal/component/firewall/plugins/irr/command.go` - where staleness becomes visible to an operator

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
| A-1 | An emptied entry bound to an interface blackholes that interface | `buildIfaceTables` gates accept terms on a non-empty list and emits the drop term unconditionally | The interface consumer is safe and only the table consumer needs fixing | A test that builds interface tables from an emptied entry and asserts no drop-everything table is produced | unvalidated |
| A-2 | The IRR reply markers for not-found, ok-empty and server error are all reachable from real servers, not only from the fakes | the fakes send the not-found marker; the parser cannot distinguish any of them | Only one marker matters and the guard can be narrower | Capture the three reply shapes in the fake server and assert the client distinguishes them | unvalidated |
| A-3 | Keeping the previous data on an empty answer cannot strand a genuinely deregistered AS-SET forever | the operator can still remove the binding or force a purge | A deregistered AS-SET keeps being enforced after it should stop | The chosen design must offer an explicit purge, proven by a test | unvalidated |
| A-4 | The BGP IRR filter consumer has the same root cause and can be fixed by the same store-level guard | it reads the same store and replaces its list with the empty one | The BGP side needs its own guard and this spec's scope is short by one file | Read the BGP filter's load path during Phase 1 and record the answer here | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Treating empty as failure hides a legitimate empty answer and leaves stale prefixes enforced | An operator reports that a removed prefix is still accepted | Expose staleness in the show command and the counters, so enforcement on stale data is visible rather than silent |
| R-2 | The one-hour cache change increases query load on public IRR servers | Refresh timing changes in the logs | Cache successful non-empty answers as today; only the empty answer stops being cached |
| R-3 | Changing `buildIfaceTables` alters the table shape for an entry that is legitimately empty | Interface table term counts change | Decide the correct behavior for a legitimately empty binding explicitly: refuse the binding, or drop the whole interface table rather than emit a drop-only one |
| R-4 | The vacuous existing test gives false confidence during the fix | `TestRefreshFailureKeepsLastGood` stays green while the behavior changes | Rewrite that test first, in Phase 1, so the suite starts telling the truth |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Ingress filtering on customer-facing ports. Too strict and an interface is blackholed; too loose and prefixes that should be filtered are accepted |
| How is it reverted? | Single commit revert. The persisted zefs cache format must stay compatible, or the revert loses the cached prefixes |
| Who else touches this path? | The BGP IRR filter reads the same store; `plan/spec-firewall-dynamic-address-group.md` and `plan/spec-firewall-remote-group.md` are adjacent set-population designs |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An IRR server answers with the not-found marker | → | `LookupPrefixes` in `internal/component/resolve/irr/client.go` | `TestLookupPrefixesDistinguishesEmptyFromData` |
| A refresh returns nothing while a populated entry exists | → | `Refresh` in `internal/component/resolve/irr/store/store.go` | `TestRefreshKeepsLastGoodOnEmptyAnswer` |
| An emptied entry reaches the interface table builder | → | `buildIfaceTables` in `internal/component/firewall/plugins/irr/sets.go` | `TestBuildIfaceTablesNeverBlackholes` |
| An operator asks for IRR status after an empty refresh | → | the show command in `internal/component/firewall/plugins/irr/command.go` | `TestShowIRRReportsStaleEntry` |
| A running daemon gets an empty answer from the IRR server | → | the whole chain to the kernel ruleset | `test/plugin/firewall-irr-empty-answer-keeps-last-good.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The IRR server answers not-found, ok-empty, or a server error, while a populated entry exists | The previous prefixes are kept, in memory and in the persisted cache |
| AC-2 | The same answer arrives | It is not written into the prefix cache, so the next refresh queries the server again |
| AC-3 | An entry has no prefixes for a family bound to an interface | No table is produced that drops everything on that interface |
| AC-4 | A refresh learns nothing | The refresh outcome counter records it distinctly from a success, the last-refresh gauge is not stamped as a successful learn, and a log line names the AS-SET |
| AC-5 | An operator runs the IRR show command after an empty refresh | The entry reports its staleness and the age of the data being enforced |
| AC-6 | An AS-SET is genuinely deregistered and the operator wants the prefixes gone | An explicit path removes them, and it is documented |
| AC-7 | A table term references an IRR set whose data is stale | The reconcile still applies; the term enforces the last-known-good set rather than failing the whole apply |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs an interface bound to an AS-SET while the IRR server has a bad minute | whois → client → store → sets → nft | `test/plugin/firewall-irr-empty-answer-keeps-last-good.ci` |
| 2 | Checks whether the filtering data is fresh | show command → store entry state | `TestShowIRRReportsStaleEntry` |
| 3 | Removes an AS-SET that no longer exists upstream | explicit purge path | `TestPurgeRemovesEntry` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLookupPrefixesDistinguishesEmptyFromData` | `internal/component/resolve/irr/client_test.go` | replaces `TestLookupPrefixesEmpty`: not-found, ok-empty and error replies are distinguishable from a real empty result | |
| `TestLookupPrefixesDoesNotCacheEmptyAnswer` | `internal/component/resolve/irr/client_test.go` | AC-2 | |
| `TestRefreshKeepsLastGoodOnEmptyAnswer` | `internal/component/resolve/irr/store/store_test.go` | AC-1 | |
| `TestRefreshFailureKeepsLastGood` | `internal/component/resolve/irr/store/store_test.go` | rewrite of the existing vacuous test so it actually refreshes and asserts retention | |
| `TestBuildIfaceTablesNeverBlackholes` | `internal/component/firewall/plugins/irr/sets_test.go` | AC-3, validates A-1 | |
| `TestShowIRRReportsStaleEntry` | `internal/component/firewall/plugins/irr/command_test.go` | AC-5 | |
| `TestRefreshOutcomeCountsEmptyDistinctly` | `internal/component/firewall/plugins/irr/irr_test.go` | AC-4 | |
| `TestPurgeRemovesEntry` | `internal/component/resolve/irr/store/store_test.go` | AC-6 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| prefixes in a refreshed entry | 0 to the server's answer size | 0 is a legal answer but not a legal overwrite | N/A | N/A |
| refresh interval | 60 to 86400 seconds, 0 meaning off | 86400 | 59 | 86401 |
| prefix cache lifetime | one hour today | unchanged for non-empty answers | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `firewall-irr-empty-answer-keeps-last-good` | `test/plugin/firewall-irr-empty-answer-keeps-last-good.ci` | the mock IRR server answers not-found after a good refresh, and the interface keeps accepting the previously learned prefixes | |
| `firewall-irr-iface-no-blackhole` | `test/plugin/firewall-irr-iface-no-blackhole.ci` | an interface bound to an AS-SET with no data does not drop everything | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | No BGP wire behavior changes here; the BGP IRR filter consumer is recorded as A-4 and scoped in Phase 1 | |

## Files to Modify
- `internal/component/resolve/irr/client.go` - distinguish the reply markers; stop caching an answer that learned nothing
- `internal/component/resolve/irr/store/store.go` - do not overwrite a populated entry with an empty one; record staleness on the entry; provide the explicit purge
- `internal/component/firewall/plugins/irr/sets.go` - never emit a drop-only interface table
- `internal/component/firewall/plugins/irr/irr.go` - count an empty refresh distinctly, log it, and stop stamping the last-refresh gauge as a learn
- `internal/component/firewall/plugins/irr/command.go` - report staleness and data age
- `internal/component/resolve/irr/client_test.go` - `TestLookupPrefixesEmpty` asserts the defect and must be replaced
- `internal/component/firewall/plugins/irr/sets_test.go` - `TestBuildSetsEmptyBoth` pins the empty-is-fine assumption and must be revisited
- `internal/component/firewall/plugins/irr/irr_test.go` - `TestRefreshFailureKeepsLastGood` is vacuous and must be rewritten
- `docs/guide/irr-filtering.md` - the last-known-good claim becomes true and gains the staleness surface
- `docs/architecture/firewall/firewall-irr.md` - document what an empty answer now does

## Files to Create
- `test/plugin/firewall-irr-empty-answer-keeps-last-good.ci` - functional proof for AC-1
- `test/plugin/firewall-irr-iface-no-blackhole.ci` - functional proof for AC-3

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | if the purge is a command it needs its node in `internal/component/firewall/plugins/irr/yang/`; decided in Phase 4 |
| YANG validation constraints | Yes | any new leaf takes native constraints; the refresh interval range is unchanged |
| YANG custom validators | N-A | no value needs a custom validator |
| CLI commands/flags | Yes | the show command gains staleness output; the purge path may add a verb |
| CLI grammar (keyword before value) | Yes | any new verb follows the keyword-before-value rule |
| Editor autocomplete | Yes | automatic for a YANG-declared node |
| Functional test for new RPC/API | Yes | the two new `.ci` files |
| Pipe completeness | Yes | the show command output must stay pipeable |
| Env var registration | N-A | no `environment/` leaf |
| Doctor check for runtime dependencies | Yes | no IRR doctor check exists; a check that reports an entry enforcing stale data, with a diagnostic code |
| Prometheus counters/metrics | Yes | an empty-answer outcome label and a data-age gauge beside the existing IRR metrics |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP family surface changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | staleness reporting and the purge path belong in `docs/features.md` if they add a verb |
| 2 | Config syntax changed? | No | unless the purge becomes config, which Phase 4 decides |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` for the show output and any purge verb |
| 4 | API/RPC added/changed? | No | no API surface |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` if the IRR plugin's command surface changes |
| 6 | Has a user guide page? | Yes | `docs/guide/irr-filtering.md` |
| 7 | Wire format changed? | No | no wire format |
| 8 | Plugin SDK/protocol changed? | No | no SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | No | IRR is not RFC-gated in this repository's ledger |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if the mock IRR server gains the reply-marker modes |
| 11 | Affects daemon comparison? | No | no comparison claim |
| 12 | Internal architecture changed? | Yes | `docs/architecture/firewall/firewall-irr.md` and `docs/architecture/resolve.md` |
| 13 | Route metadata keys added/changed? | No | no metadata key |
| 14 | Prometheus counters added/changed? | Yes | the new outcome label and age gauge |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | if a purge verb registers, `docs/features/plugins.md` and `docs/guide/status.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/guide/firewall.md` anchors `irr.go` and `sets.go`; `docs/guide/irr-filtering.md` anchors `store.go`; `ai/digests/firewall.md` carries rows for both |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | verify the IRR examples and the show output samples against the new fields |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the suite tell the truth, and settle A-1 and A-4
   - Tests: rewritten `TestRefreshFailureKeepsLastGood`, new `TestBuildIfaceTablesNeverBlackholes`
   - Files: `internal/component/firewall/plugins/irr/irr_test.go`, `internal/component/firewall/plugins/irr/sets_test.go`
   - Verify: the blackhole test fails today, proving A-1; the BGP filter's load path is read and A-4 is answered in this spec
2. **Phase: distinguish the answers** -- the client stops collapsing failure into emptiness
   - Tests: `TestLookupPrefixesDistinguishesEmptyFromData`, `TestLookupPrefixesDoesNotCacheEmptyAnswer`
   - Files: `internal/component/resolve/irr/client.go`, `internal/component/resolve/irr/client_test.go`
   - Verify: the three reply shapes are distinguishable; nothing empty enters the one-hour cache
3. **Phase: keep last known good** -- the store refuses the destructive overwrite
   - Tests: `TestRefreshKeepsLastGoodOnEmptyAnswer`
   - Files: `internal/component/resolve/irr/store/store.go`
   - Verify: a populated entry survives an empty answer, in memory and in zefs
4. **Phase: no blackhole, and an exit** -- table shape plus the purge
   - Tests: `TestBuildIfaceTablesNeverBlackholes`, `TestPurgeRemovesEntry`
   - Files: `internal/component/firewall/plugins/irr/sets.go`, `internal/component/resolve/irr/store/store.go`
   - Verify: an empty binding never produces a drop-only table, and a deregistered AS-SET can still be cleared deliberately
5. **Phase: make it visible** -- counters, log line, show output, doctor check
   - Tests: `TestRefreshOutcomeCountsEmptyDistinctly`, `TestShowIRRReportsStaleEntry`
   - Files: `internal/component/firewall/plugins/irr/irr.go`, `internal/component/firewall/plugins/irr/command.go`, the doctor check and its diagnostic code
   - Verify: an operator can tell that enforcement is running on stale data
6. **Phase: functional proof**
   - Tests: the two new `.ci` files
   - Files: `test/plugin/`, the mock IRR server's reply modes
   - Verify: reverting Phase 3 makes the last-known-good test fail, so the test is not vacuous

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Failure, not-found, ok-empty and a genuine empty answer are four distinguishable states, and each has a defined effect |
| Naming | The staleness field is named the same way in the store, the show output and the metric |
| Data flow | The keep-last-known-good decision is made once, in the store, not repeated per consumer |
| Rule: `ai/rules/evidence.md` | A guard fails closed and a zero value never reads as a valid answer |
| Rule: `ai/rules/testing.md` | The three tests that pin the defect are corrected, and the vacuous one is rewritten rather than deleted |
| Registration over hardcoding | The doctor check and any new command register through the existing registries |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Empty answer never overwrites | `TestRefreshKeepsLastGoodOnEmptyAnswer` passes |
| No blackhole table | `TestBuildIfaceTablesNeverBlackholes` passes |
| Staleness visible | the IRR show command output carries the field, checked by the `.ci` |
| Functional proof | `make ze-functional-plugin-test` runs both new `.ci` files green |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The IRR reply is third-party text; every marker and every unparseable word must have a defined meaning, not be skipped |
| Fail closed | A guard must not treat "no data" as "the answer is nothing"; a zero value is never a valid-looking answer |
| Availability | The fix must not let a hostile or broken upstream take an interface down, which is the current behavior |
| Resource exhaustion | Keeping last-known-good must not grow the persisted cache without bound; entries are keyed by AS-SET and replaced, not appended |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A-1 refuted (no blackhole) | Narrow the spec to the table-term consumer and say so; the store fix stands either way |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Guard in the store, using the existing empty helper | Guard in every consumer; require N consecutive empty answers; refuse a shrink beyond a ratio | One guard fixes both firewall consumers and the BGP filter at once. Counting consecutive answers and ratio limits add tunables the store has no precedent for, and both still need the same distinction the client currently cannot make |
| Distinguish the reply markers at the client | Treat every empty answer as failure | Collapsing them loses the legitimate empty case, which A-3 says must stay expressible. The parser already sees the markers; it just discards them |
| Surface staleness rather than only fixing it | Fix silently | Enforcement on stale data is a real state with a real risk, and an operator must be able to see it. This follows the RPKI precedent where synced was separated from configured and surfaced in the show command |

## Known Limitations
- Enforcement continues on last-known-good data for as long as the upstream stays unhelpful. That is the intended trade against a blackhole, and the staleness surface is what makes it a choice rather than a surprise.
- The BGP IRR filter consumer is fixed only if A-4 holds; if it needs its own guard, that lands in this spec's Phase 3 or gets its own spec, decided in Phase 1 and recorded here.

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
