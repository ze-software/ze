# Spec: fixit-irr-empty-answer-clears-set

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 6/6 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-22 |

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
| A-1 | An emptied entry bound to an interface blackholes that interface | `buildIfaceTables` gates accept terms on a non-empty list and emits the drop term unconditionally | The interface consumer is safe and only the table consumer needs fixing | A test that builds interface tables from an emptied entry and asserts no drop-everything table is produced | confirmed -- `TestBuildIfaceTablesNeverBlackholes` failed against the unfixed builder with `term "iface_eth1_drop" drops every packet on eth1 with no accept term to precede it` |
| A-2 | The IRR reply markers for not-found, ok-empty and server error are all reachable from real servers, not only from the fakes | the fakes send the not-found marker; the parser cannot distinguish any of them | Only one marker matters and the guard can be narrower | Capture the three reply shapes in the fake server and assert the client distinguishes them | confirmed for the PROTOCOL: RPSL answers a `!` query with one status line, `C`, `D`, `E` or `F <message>`, and `parseReply` now reads it. `TestLookupPrefixesDistinguishesEmptyFromData` drives all four plus a truncated reply. NOT exercised against a live IRR server: no test in this repository reaches one |
| A-3 | Keeping the previous data on an empty answer cannot strand a genuinely deregistered AS-SET forever | the operator can still remove the binding or force a purge | A deregistered AS-SET keeps being enforced after it should stop | The chosen design must offer an explicit purge, proven by a test | confirmed -- `PrefixStore.Purge` plus `clear firewall irr asn|as-set`, proven by `TestPurgeRemovesEntry` and `TestClearFirewallIRRPurgesEntry` |
| A-4 | The BGP IRR filter consumer has the same root cause and can be fixed by the same store-level guard | it reads the same store and replaces its list with the empty one | The BGP side needs its own guard and this spec's scope is short by one file | Read the BGP filter's load path during Phase 1 and record the answer here | confirmed -- `refreshASNCtx` (`internal/component/bgp/plugins/filter_irr/filter_irr.go`) reads `entry, err := ps.Refresh(...)` and returns on `err != nil` without touching `st.list`, so `ErrNoPrefixes` puts it on the branch that keeps the previous prefix list. `loadFromStore` (`internal/component/bgp/plugins/filter_irr/cache.go`) seeds that list from the persisted last-known-good before the first refresh runs, so a cold start is covered too. No edit to `filter_irr` was needed, and `TestRefreshEmptyAnswerKeepsFilterList` now pins it: red with the store guard reverted, on `the empty answer replaced the prefix list: &{entries:[]}` |

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
| AC-8 | An operator runs `update firewall irr as-set X` within an hour of the last fetch | The command queries the server. Fixed: `IRR.RefreshPrefixes` (`internal/component/resolve/irr/client.go`) never reads the `cacheTTL = time.Hour` cache, and `PrefixStore.resolve` (`internal/component/resolve/irr/store/store.go`) calls it rather than `LookupPrefixes`. Before that split, the one command an operator had to force a refresh answered from memory and never reached the server, which kept both new `.ci` files red on `a refresh that learned nothing reported success`. Both now pass on Linux (see Functional Tests). Routed here by the review of `plan/spec-fixit-firewall-irr-term-fails-validation.md` |

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
| `TestLookupPrefixesDistinguishesEmptyFromData` | `internal/component/resolve/irr/client_test.go` | replaces `TestLookupPrefixesEmpty`: not-found, ok-empty and error replies are distinguishable from a real empty result | pass, red before |
| `TestParseReply` | `internal/component/resolve/irr/client_test.go` | the four RPSL states at the producer | pass |
| `TestResolveASSetServerError` | `internal/component/resolve/irr/client_test.go` | the same defect on the AS-SET expansion path | pass, red before |
| `TestLookupPrefixesDoesNotCacheEmptyAnswer` | `internal/component/resolve/irr/client_test.go` | AC-2 | pass, red before |
| `TestRefreshKeepsLastGoodOnEmptyAnswer` | `internal/component/resolve/irr/store/store_test.go` | AC-1, over all three reply shapes | pass, red with the guard reverted |
| `TestRefreshStoresNothingOnFirstEmptyAnswer` | `internal/component/resolve/irr/store/store_test.go` | AC-1: an empty answer creates no entry | pass, red with the guard reverted |
| `TestRefreshClearsStaleness` | `internal/component/resolve/irr/store/store_test.go` | AC-1: a good refresh clears the stale marker | pass, red with the guard reverted |
| `TestRefreshFailureKeepsLastGood` | `internal/component/firewall/plugins/irr/irr_test.go` | rewrite of the existing vacuous test so it actually refreshes and asserts retention | pass |
| `TestBuildIfaceTablesNeverBlackholes` | `internal/component/firewall/plugins/irr/sets_test.go` | AC-3, validates A-1 | pass, red before |
| `TestBuildIfaceTablesKeepsDropWhenPopulated` | `internal/component/firewall/plugins/irr/sets_test.go` | AC-3 control: the whitelist still closes | pass |
| `TestBuildSetsEmptyBoth` | `internal/component/firewall/plugins/irr/sets_test.go` | AC-3 through the table-term consumer: an entry with no prefixes yields no set AND no table, so nothing empty is programmed. Strengthened rather than deleted; the original asserted only the zero-set half | pass, red with `buildSets` emitting a set per family |
| `TestRefreshEmptyAnswerKeepsFilterList` | `internal/component/bgp/plugins/filter_irr/filter_irr_test.go` | AC-1 at the third consumer: an empty answer leaves the BGP filter's prefix list intact and its peer's UPDATEs accepted | pass, red with the store guard reverted |
| `TestShowIRRReportsStaleEntry` | `internal/component/firewall/plugins/irr/command_test.go` | AC-5 | pass |
| `TestRefreshOutcomeCountsEmptyDistinctly` | `internal/component/firewall/plugins/irr/irr_test.go` | AC-4 | pass |
| `TestRefreshOutcomeCountsSuccess` | `internal/component/firewall/plugins/irr/irr_test.go` | AC-4 control | pass |
| `TestVerifyRefsRefusesEmptyEntry` | `internal/component/firewall/plugins/irr/irr_test.go` | AC-3 at commit time | pass |
| `TestPurgeRemovesEntry` | `internal/component/resolve/irr/store/store_test.go` | AC-6 | pass |
| `TestClearFirewallIRRPurgesEntry` | `internal/component/firewall/plugins/irr/command_test.go` | AC-6 through the command surface | pass |
| `TestDoctorReportsStaleIRRData`, `TestDoctorSilentWithoutIRRReferences`, `TestDoctorCodesAreRegistered` | `internal/component/firewall/plugins/irr/doctor_test.go` | AC-5 through `ze doctor` | pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| prefixes in a refreshed entry | 0 to the server's answer size | 0 is a legal answer but not a legal overwrite | N/A | N/A |
| refresh interval | 60 to 86400 seconds, 0 meaning off | 86400 | 59 | 86401 |
| prefix cache lifetime | one hour today | unchanged for non-empty answers | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `firewall-irr-empty-answer-keeps-last-good` | `test/plugin/firewall-irr-empty-answer-keeps-last-good.ci` | the mock IRR server answers not-found after a good refresh, and the interface keeps accepting the previously learned prefixes | pass on Linux 2026-08-22; red with the Phase 3 guard reverted, on `ZE-OBSERVER-FAIL: a refresh that learned nothing reported success` |
| `firewall-irr-iface-no-blackhole` | `test/plugin/firewall-irr-iface-no-blackhole.ci` | an interface bound to an AS-SET with no data does not drop everything | pass on Linux 2026-08-22; red with the Phase 3 guard reverted, with the same observer failure |

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
- The BGP IRR filter consumer is fixed only if A-4 holds; if it needs its own guard, that lands in this spec's Phase 3 or gets its own spec, decided in Phase 1 and recorded here. A-4 HOLDS: `filter_irr` takes its existing error branch on `ErrNoPrefixes` and leaves `st.list` untouched, so no edit to that plugin was needed.
- Every firewall IRR `.ci` declares `option=needs-linux:caps=net-admin`, so all of them SKIP on darwin. The two new files were executed on Linux on 2026-08-22 in a privileged container, and all 12 firewall IRR `.ci` files pass there. The command is in the handoff. Two conditions the runner does not enforce: the daemon binary MUST sit outside the checkout, because `paths.ConfigDirFromBinary` derives the config dir from the binary path and a binary in `<repo>/bin` reads and writes the developer's `<repo>/etc/ze/database.zefs`, which warms the IRR cache that `firewall-irr-cold-cache-recovers` needs cold; and the suite MUST run serially, because every test programs the one node-wide nftables ruleset the container shares.

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
- [ ] Lesson written as a row in `plan/journal/<class>.md` (`plan/learned/NNN-<name>.md` no longer exists as a destination)
- [ ] **Commit A:** code + tests + spec + journal rows
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- Phases 1 to 6 landed in five earlier commits (`f67afd0e1`, `20ad3ebb6`, `a96912e24`, `7f7b74995`, `15821cf9e`). The audit phase re-read every producer against the tree, closed two test gaps, and obtained the functional proof. No production Go was edited in the audit phase or in this closure.
- The audit phase added the consumer half of `TestBuildSetsEmptyBoth` (`internal/component/firewall/plugins/irr/sets_test.go`) and `TestRefreshEmptyAnswerKeepsFilterList` (`internal/component/bgp/plugins/filter_irr/filter_irr_test.go`), the third consumer's regression pin.
- This closure added a fifth cluster to `TestContendingFunctionalTestsDeclareExclusiveGroup` (`internal/test/runner/exclusive_group_test.go`) and `option=exclusive:group=firewall-irr-nft` to all 12 `test/plugin/firewall-irr-*.ci` files, so the runner enforces the isolation the suite previously got from an operator remembering `-p 1`.
- This closure recorded the `ConfigDirFromBinary` condition as a HARNESS NOTE in `test/plugin/firewall-irr-cold-cache-recovers.ci`, the test whose first assertion it breaks.

### Bugs Found/Fixed
- AC-8, fixed in an earlier phase: `update firewall irr as-set X` answered from the one-hour client cache and never reached the server, so the empty-answer branch was unreachable from the surface an operator drives. `IRR.RefreshPrefixes` (`internal/component/resolve/irr/client.go`) now queries the server and never reads that cache. Covered by both new `.ci` files, which were red on exactly this.
- The exclusive-group ratchet's population excluded the firewall-irr cluster. Fixed in this closure; the ratchet is red without the declarations and names all 12 files.
- The functional runner gives every daemon the same binary-derived config dir, so all 690 tests share one `database.zefs` and it is the developer's own. Recorded, not fixed: the repair is a per-test config dir in the runner's process model.

### Documentation Updates
- `docs/guide/irr-filtering.md` and `docs/architecture/firewall/firewall-irr.md` carry the staleness surface, the per-family retention, the purge path, and the troubleshooting rows. Landed in the earlier phase commits, re-verified here by grep against the producing functions.
- `docs/guide/firewall.md` documents the `error` and `empty` labels of `ze_firewall_irr_refresh_outcomes_total`.
- `docs/functional-tests.md` gained the fifth exclusive-group cluster with three source anchors, and its stale plugin-suite count was corrected from 530 to 690. WRITTEN BUT NOT COMMITTED: see NOTE-4 in the Review Gate.
- `make ze-doc-verify` exit 0, `make ze-doc-links-check` exit 0, `make ze-repository-check` exit 0.

### Deviations from Plan
- The Closure checklist asked for `plan/learned/NNN-<name>.md`. That destination no longer exists; the lesson is three rows under `plan/journal/`.
- Phases 1 to 6 were implemented by earlier sessions rather than in one pass. The audit phase verified each producer instead of re-implementing.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The closure considered adding `option=env:var=ze.config.dir:value=.` to the 12 `.ci` files, to give each test a private prefix store | That patches 12 files of 690 and leaves the same exposure everywhere else, and it changes test semantics: `PrefixStore.persist` returns early when the zefs file does not exist, so an empty tmpfs makes the store in-memory only | Reading `DefaultConfigDir` (`internal/core/paths/paths.go`) and `PrefixStore.persist` (`internal/component/resolve/irr/store/store.go`) | Abandoned. The root cause is the runner's process model, recorded in `plan/journal/suite-shares-one-persistent-store.md` |
| approach | The first discrimination mutant deleted the `Empty()` branch from `PrefixStore.Refresh` outright | That leaves `fmt` unused and the package fails to BUILD, so every test reads as red for the wrong reason and the measurement says nothing | the run answered `"fmt" imported and not used` and `[build failed]` rather than an assertion | Re-cut the mutant as a condition that compiles and never fires, so the reds are assertion failures rather than build failures |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| An empty answer stops overwriting a populated entry | Done | `PrefixStore.Refresh`, `commit`, `markStale` (`internal/component/resolve/irr/store/store.go`) | `Refresh` returns `ErrNoPrefixes` and calls `markStale`; `commit` keeps the cached prefixes of any family that answered nothing |
| An empty answer stops being cached as if it were data | Done | `IRR.queryPrefixes` (`internal/component/resolve/irr/client.go`) | caches only when `!result.Empty()` |
| An emptied or stale entry stops producing a blackhole table | Done | `buildIfaceTables` (`internal/component/firewall/plugins/irr/sets.go`) | a binding with no accept term emits no term and logs a WARN |
| A refresh that learned nothing is reported as such | Done | `refreshAllNow`, `refreshName` (`internal/component/firewall/plugins/irr/irr.go`) | `incRefreshOutcome("empty")`, `logEmptyRefresh`, and `markRefreshLearned` only on a learn |
| The three reply shapes are distinguishable | Done | `parseReply` (`internal/component/resolve/irr/client.go`) | exact status-letter match; a reply with no status line reports `replyFailed` |
| The third consumer shares the fix | Done | `refreshASNCtx` (`internal/component/bgp/plugins/filter_irr/filter_irr.go`), `loadFromStore` (`internal/component/bgp/plugins/filter_irr/cache.go`) | the error branch returns without touching `st.list`; `loadFromStore` skips an entry that yields no entries |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `PrefixStore.Refresh` | `TestRefreshKeepsLastGoodOnEmptyAnswer`, `TestRefreshStoresNothingOnFirstEmptyAnswer`, `TestRefreshFailureKeepsLastGood`, `TestRefreshEmptyAnswerKeepsFilterList`, `firewall-irr-empty-answer-keeps-last-good.ci` |
| AC-2 | Done | `IRR.queryPrefixes` | `TestLookupPrefixesDoesNotCacheEmptyAnswer` |
| AC-3 | Done | `buildIfaceTables`, `buildIRRTables` | `TestBuildIfaceTablesNeverBlackholes`, `TestBuildSetsEmptyBoth`, `firewall-irr-iface-no-blackhole.ci` |
| AC-4 | Done | `refreshAllNow`, `refreshName` | `TestRefreshOutcomeCountsEmptyDistinctly`, `TestRefreshOutcomeCountsSuccess` |
| AC-5 | Done | the show handler in `internal/component/firewall/plugins/irr/command.go`, and `CachedEntry.Stale` | `TestShowIRRReportsStaleEntry`, `TestDoctorReportsStaleIRRData` |
| AC-6 | Done | `PrefixStore.Purge` | `TestPurgeRemovesEntry`, `TestClearFirewallIRRPurgesEntry` |
| AC-7 | Done | `dropTablesMissingAProvidedSet` (`internal/component/firewall/registry.go`) | `firewall-irr-cold-cache-recovers.ci`, `firewall-irr-table-term-uncached-reject.ci` |
| AC-8 | Done | `IRR.RefreshPrefixes`, called by `PrefixStore.resolve` | both new `.ci` files, red on this exact symptom before the split |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Every unit test in the TDD table | Done | the four IRR packages | `./internal/component/resolve/irr`, `./internal/component/resolve/irr/store`, `./internal/component/firewall/plugins/irr`, `./internal/component/bgp/plugins/filter_irr` all exit 0 |
| `TestLookupPrefixesEmpty` replaced | Done | `internal/component/resolve/irr/client_test.go` | the name resolves nowhere in the tree; `TestLookupPrefixesDistinguishesEmptyFromData` drives all four reply states |
| `TestRefreshFailureKeepsLastGood` rewritten | Done | `internal/component/firewall/plugins/irr/irr_test.go` | refreshes twice and asserts retention plus staleness; red against the neutered guard on `a refresh that learned nothing must report it, not report success` |
| `TestBuildSetsEmptyBoth` strengthened | Done | `internal/component/firewall/plugins/irr/sets_test.go` | both halves red against the `buildSets` mutant, the builder on `expected 0 sets for empty prefix lists, got 1` and the new consumer half on `an entry with no prefixes produced 1 table(s)` |
| The two `.ci` files | Done | `test/plugin/` | 12 of 12 firewall IRR `.ci` pass on Linux |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| The six production files in Files to Modify | Done | all six carry the change; verified by reading each producer in this closure |
| The three test files in Files to Modify | Done | one replaced, one rewritten, one strengthened |
| `docs/guide/irr-filtering.md`, `docs/architecture/firewall/firewall-irr.md` | Done | staleness, per-family retention, purge, troubleshooting |
| The two files in Files to Create | Done | both exist and both pass on Linux |
| `internal/test/runner/exclusive_group_test.go`, the 12 `.ci` | Changed | added by this closure, outside the original plan; recorded in `plan/journal/gate-excludes-part-of-its-population.md` |

### Audit Summary
- **Total items:** 8 AC, 6 task requirements, 5 test groups, 5 file groups
- **Done:** 23
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (the exclusive-group cluster, added beyond the plan)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A transient IRR answer never blackholes a physical port | functional | `test/plugin/firewall-irr-iface-no-blackhole.ci` passes on Linux and is red with the Phase 3 guard reverted, on `ZE-OBSERVER-FAIL: a refresh that learned nothing reported success`. Unit control: `TestBuildIfaceTablesNeverBlackholes` reds with the empty-accept guard neutered, on `term "iface_eth1_drop" drops every packet on eth1 with no accept term to precede it` |
| A populated entry survives an unhelpful upstream, in memory and on disk | functional | `test/plugin/firewall-irr-empty-answer-keeps-last-good.ci` passes on Linux, red with the guard reverted. Unit control: neutering `PrefixStore.Refresh`'s `Empty()` branch reds 8 tests across three packages, `TestRefreshKeepsLastGoodOnEmptyAnswer` and `TestRefreshFailureKeepsLastGood` among them |
| The table-term consumer programs nothing empty | unit, driven from the consumer | `TestBuildSetsEmptyBoth` asserts through `buildIRRTables`, not only `buildSets`. Both halves red against the mutant: `expected 0 sets for empty prefix lists, got 1` and `an entry with no prefixes produced 1 table(s)` |
| The BGP IRR filter keeps accepting the peer's UPDATEs | unit, driven from the filter's entry point | `TestRefreshEmptyAnswerKeepsFilterList` refreshes twice and then drives `handleFilterUpdate`, asserting `FilterAccept`. Red against the mutant on `the empty answer replaced the prefix list: &{entries:[]}` |
| An operator can see that enforcement runs on stale data | functional and unit | `TestShowIRRReportsStaleEntry` (red against the mutant on `expected the empty answer to be reported`), `TestDoctorReportsStaleIRRData`, and the show output documented in `docs/guide/irr-filtering.md` |
| A deregistered AS-SET can still be removed | unit through the command surface | `TestPurgeRemovesEntry`, `TestClearFirewallIRRPurgesEntry` |
| The suite that proves all of this is reproducible without an operator remembering a flag | ratchet | `TestContendingFunctionalTestsDeclareExclusiveGroup` red naming all 12 files before the declarations, green after |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | done | The spec's metadata names no deferral shard, and `plan/deferrals/` holds no `fixit-irr-empty-answer-clears-set.md`. Nothing to remove and no foreign shard emptied by this closure |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-irr-empty-answer-clears-set-ebf5d9b3-b158-40df-bba0-32a51591883e.md` |
| `review_gate.py check` | clean, 15 code files, hashes match |
| Rounds | 1. Round 1 reported 0 BLOCKER and 0 ISSUE, so the loop terminated there |
| Reviewer lenses used | logic+wiring, security+edge-cases, gate/ratchet risk. All three because the diff changes a ratchet and the tests that pin a guard |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | NOTE only | No BLOCKER and no ISSUE were found. Four NOTEs sit in the artifact: a cross-suite table-name overlap tested and refuted at `run_suite`; the deliberate fail-open of `buildIfaceTables`; the BGP consumer's coarse `error` counter label, which AC-4 does not cover and which `st.lastErr` already distinguishes for an operator; and the exclusion of `docs/functional-tests.md` from the commit | `mk/test-functional.mk`, `internal/component/firewall/plugins/irr/sets.go`, `internal/component/bgp/plugins/filter_irr/filter_irr.go`, `docs/functional-tests.md` | Not applicable: NOTEs do not block |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/firewall-irr-empty-answer-keeps-last-good.ci` | Yes | `ls -la` reports `5.0K Aug 22 15:20` |
| `test/plugin/firewall-irr-iface-no-blackhole.ci` | Yes | `ls -la` reports `4.5K Aug 22 15:20` |
| All 12 `test/plugin/firewall-irr-*.ci` | Yes | `ze-test bgp plugin -l` lists them as records 252 to 263 of 690 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | the previous prefixes are kept | read `Refresh`: `entry.PrefixList().Empty()` returns `markStale` plus `ErrNoPrefixes`. Neutering it reds `TestRefreshKeepsLastGoodOnEmptyAnswer`, `TestRefreshStoresNothingOnFirstEmptyAnswer`, `TestRefreshQueriesServerWithinCacheTTL` and `TestRefreshClearsStaleness` |
| AC-2 | nothing empty enters the prefix cache | read `queryPrefixes`: it sets the cache only inside `if !result.Empty()` |
| AC-3 | no blackhole table | read `buildIfaceTables`: a binding with no accept term logs and continues. Neutered, `TestBuildIfaceTablesNeverBlackholes` reds |
| AC-4 | the empty outcome is counted apart | read `refreshAllNow`: `errors.Is(err, store.ErrNoPrefixes)` takes `incRefreshOutcome("empty")`, and `markRefreshLearned()` runs only when something was learned |
| AC-5 | staleness is visible | read the show handler: `entry.Stale()` emits `"status":"stale"` and `"stale-since"`. Neutering the store guard reds `TestShowIRRReportsStaleEntry` |
| AC-6 | an explicit removal path exists | read `Purge`: it refuses an invalid name, deletes in memory and on disk, and reports whether anything was there |
| AC-7 | a stale set does not fail the whole apply | read `dropTablesMissingAProvidedSet`: the unit held back is one TABLE, and its supplier's own `ApplyAll` programs it later |
| AC-8 | the force refresh reaches the server | read `RefreshPrefixes`: it calls `queryPrefixes` directly and never reads `prefixCache`; `resolve` calls it rather than `LookupPrefixes` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| An IRR server answers with the not-found marker | unit at the client | `TestLookupPrefixesDistinguishesEmptyFromData`; `parseReply` matches the status letter exactly rather than by prefix |
| A refresh returns nothing while a populated entry exists | `test/plugin/firewall-irr-empty-answer-keeps-last-good.ci` | read the file: it drives `update firewall irr as-set AS-TEST` against the mock server's empty-after-first mode and asserts the prefixes survive |
| An emptied entry reaches the interface table builder | `test/plugin/firewall-irr-iface-no-blackhole.ci` | read the file: it configures `interface eth1 { source-as-set AS-TEST }` and asserts no drop-only table |
| An operator asks for IRR status after an empty refresh | unit at the command | `TestShowIRRReportsStaleEntry` |
| A running daemon gets an empty answer | both new `.ci` | 12 of 12 firewall IRR `.ci` pass on Linux, serially |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestBuildIfaceTablesNeverBlackholes` reds against the unfixed builder with `term "iface_eth1_drop" drops every packet on eth1 with no accept term to precede it`, re-measured in this closure |
| A-2 | confirmed for the protocol | `parseReply` reads the RPSL status line; `TestLookupPrefixesDistinguishesEmptyFromData` drives C, D, E, F and a truncated reply. Not exercised against a live IRR server: no test in this repository reaches one |
| A-3 | confirmed | `PrefixStore.Purge` plus the `clear firewall irr` verbs, proven by `TestPurgeRemovesEntry` and `TestClearFirewallIRRPurgesEntry` |
| A-4 | confirmed, re-derived at the producer in this closure | `refreshASNCtx` sets `st.lastErr`, signals, unlocks and RETURNS on a non-nil error, never reaching the `st.list` assignment; `loadFromStore` skips an entry whose `prefixListFromIRR` yields no entries, so a cold start installs no empty list. `TestRefreshEmptyAnswerKeepsFilterList` reds against the mutant |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/irr-filtering.md` last-known-good and staleness claims | grep shows the `stale`, `stale-since` and `data-age-seconds` rows; `CachedEntry.Stale` and the show handler produce them | Yes |
| `docs/guide/irr-filtering.md` purge instructions | both `clear firewall irr` forms are documented; `PrefixStore.Purge` and the command handler produce them | Yes |
| `docs/architecture/firewall/firewall-irr.md` empty-answer design | the page states the per-family retention and the operator-action removal, which `commit` and `Purge` implement | Yes |
| `docs/guide/firewall.md` metric labels | documents `error` and `empty` for `ze_firewall_irr_refresh_outcomes_total`; `incRefreshOutcome` emits both | Yes |
| `docs/functional-tests.md` exclusive-group clusters | the fifth cluster is written with three source anchors, and the stale suite count is corrected | Written, not committed (NOTE-4) |
| RFC status | No: IRR is not RFC-gated in this repository's ledger, and the diff touches no wire protocol | Yes |
| Doctor checks | No new runtime dependency in this closure; the IRR doctor check and its diagnostic codes landed with AC-5 and are covered by `TestDoctorCodesAreRegistered` | Yes |

## Core Insight

A guard is only as reachable as the surface an operator can drive to it. The
store guard was correct from Phase 3 and both functional tests stayed red,
because the one command an operator has to force a refresh answered from a
one-hour client cache and never reached the server that would have produced the
empty answer. Every unit test of the guard was green throughout. The proof that
mattered was the one driven from the operator's command, not from the function.
