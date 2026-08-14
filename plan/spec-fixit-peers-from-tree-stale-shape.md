# Spec: fixit-peers-from-tree-stale-shape

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-implement at stage 10.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | - |
| Phase | 1/1 |
| Deferral shard | plan/deferrals/ad-hoc-2026-07-27-423eaa77.md |
| Updated | 2026-08-14 |

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     Deferral shard: every deferred item lands there (ai/rules/planning.md)
     and closure must resolve its rows, so name the file from the start. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Symptom.** `parsePeersFromTree` reads a peer shape the YANG no longer produces.
On a real YANG tree it dies at `missing required remote container`
(`internal/component/bgp/reactor/reactor_api.go`) before reaching any other leaf.

**Producer.** `parsePeersFromTree` (`internal/component/bgp/reactor/reactor_api.go`)
expects `remote`, `local`, `receive-hold-time` and `router-id` DIRECTLY under the
peer. `grouping peer-fields` (`internal/component/bgp/yang/ze-bgp-conf.yang`)
nests them as `connection > remote`, `session > router-id`,
`timer > receive-hold-time` -- which is what the real pipeline reads
(`internal/component/bgp/config/config.go` and `:144`).

**Reachability (corrected 2026-08-14).** The 2026-07-27 text said a stdin config
(`ze -`) reaches the fallback. It reaches the fallback's SELECTION CONDITION, and
nothing calls the enclosing function on a stdin daemon. Both halves are verified:

- The condition holds. `CreateReactor` (`internal/component/bgp/config/loader.go`)
  guards `SetConfigPath` and `SetReloadFunc` together on
  `configPath != "" && configPath != "-"`, so a stdin daemon has `reloadFn == nil`
  and `configPath == ""`, which is what `loadPeersFullOrTree` (`reactor_api.go`)
  branches on.
- Nothing gets there. `runReload` (`cmd/ze/hub/main_reload.go`) short-circuits on
  `load == nil` and returns `Server.ReloadFromDisk`, whose `reloadFull`
  (`internal/component/plugin/server/reload.go`) answers
  `errNoConfigLoaderConfigured`. The two production callers of
  `loadPeersFullOrTree` are `PeerDiffCount` and `ReconcilePeersWithJournal`, both
  inside the plugin config transaction (`OnConfigVerify` and `OnConfigApply`,
  `internal/component/bgp/plugin/register.go`), which a file-configured daemon
  enters and where both guards hold.

So the flat parser was unreachable in production, and the failure it would have
produced was never user-visible. It was still a second parser of one shape.

**Why it was not fixed on discovery.** Realigning the shape rewrites a function whose
ENTIRE existing test surface encodes the flat form. Changing the production shape
without changing those tests would be a silent behaviour change under a green bar;
changing the tests alongside is a test rewrite that needs its own review
(`ai/rules/testing.md` "Test Rewrite as Replacement").

**Goal.** One peer parser in the package. `parsePeersFromTree` is deleted and
`loadPeersFullOrTree`'s fallback calls `PeersFromTree`
(`internal/component/bgp/reactor/config.go`), which already reads the nested YANG
shape and is what the file-config route reaches too (`peersAndDynamicGroups`,
`internal/component/bgp/config/peers.go`). Its tests migrate to that shape.

Neither option the 2026-07-27 text named was taken. Porting the flat parser leaves
two parsers of one shape, and that duplication is what produced the router-id
divergence its own comment records. Deleting the branch makes `loadPeersFullOrTree`
return `reloadFn(configPath)` on a nil `reloadFn`, and contradicts
`plan/spec-review-typed-config-decode.md`, which lists the function with "keep test
fallback".

**Provenance.** Found 2026-07-27 while correcting a closure record's false claim
that the fallback is unreachable in production. It is reachable; it just fails early.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `ai/rules/no-layering.md` - the change replaces one parser with another
  → Constraint: delete the old parser first, do not leave both standing.
- [ ] `ai/rules/testing.md` - the whole existing test surface encodes the old shape
  → Constraint: "Test Rewrite as Replacement". A removed test's proof must still exist.

### RFC Summaries (Scope: protocol)
Not applicable. Scope is config parsing. One RFC-tagged behavior is carried
through unchanged and is named in the Acceptance Criteria: RFC 6286 Section 2.1
(non-zero BGP Identifier), enforced by `parseRouterID`
(`internal/component/bgp/reactor/config.go`).

**Key insights:** (minimal context to resume after compaction)
- Two parsers read one config shape. The nested one is correct and already
  production-wired; the flat one is a duplicate that no longer matches the YANG.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `loadPeersFullOrTree`
  picks `reloadFn(configPath)` or the flat `parsePeersFromTree`.
- [ ] `internal/component/bgp/reactor/config.go` - `PeersFromTree` and
  `parsePeerFromTree` read the nested YANG shape; `ErrIncompleteConfig` makes a
  peer skipped with a warning rather than fatal.
- [ ] `internal/component/bgp/config/peers.go` - `peersAndDynamicGroups` calls
  `reactor.PeersFromTree`, so the file-config route ends in the same parser.
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - `grouping peer-fields`
  nests `connection`, `session > asn`, `timer`.

**Behavior to preserve:** (unless the user explicitly said to change it)
- `loadPeersFullOrTree` keeps both branches and its signature
  `([]*PeerSettings, error)`.
- A bad `router-id` still fails the whole parse and returns no peers.
- An unparseable remote IP still fails with an error quoting the value.

**Behavior to change:** (only what the user asked for)
- The fallback branch reads the nested YANG shape instead of the flat one.
- A peer missing a required leaf is now SKIPPED with a warning on that branch
  (`ErrIncompleteConfig`) instead of failing the parse. This matches what the
  file-config route has always done.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator config commit. The plugin config transaction hands the BGP subtree to
  `OnConfigVerify` / `OnConfigApply` (`internal/component/bgp/plugin/register.go`)
  as JSON, unmarshalled to `map[string]any` in the nested YANG shape.

### Transformation Path
1. `Reactor.PeerDiffCount` / `Reactor.ReconcilePeersWithJournal`
   (`internal/component/bgp/reactor/reactor.go`) take the tree.
2. `loadPeersFullOrTree` (`reactor_api.go`) picks the route: a file-configured
   daemon re-reads the file through `reloadFn`, everything else parses the tree.
3. Both routes end at `PeersFromTree` (`config.go`): `reloadFn` reaches it via
   `peersAndDynamicGroups` (`../config/peers.go`), the fallback calls it directly.
4. `parsePeerFromTree` then `parsePeerSettings` produce `[]*PeerSettings`.
5. `reconcilePeersJournaled` (`reactor_api.go`) diffs and applies them.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | BGP subtree as JSON in `sdk.ConfigSection.Data`, unchanged by this spec | Yes |

### Integration Points
- `PeersFromTree` (`internal/component/bgp/reactor/config.go`) - the fallback now
  calls it, so the package holds one peer parser instead of two.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Both branches of `loadPeersFullOrTree` end at `PeersFromTree` |
| No unintended coupling (components stay isolated) | Yes | Both functions are in package `reactor`; no new import |
| No duplicated functionality (extends existing, does not recreate) | Yes | The duplicate `parsePeersFromTree` is deleted, not ported |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Config parse, not a wire path |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them; no per-feature field, switch case, or factory added to a core/shared package (`ai/rules/plugins.md`) | Yes | Nothing registered, nothing added; 123 lines removed |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `PeersFromTree` returns the same type `loadPeersFullOrTree` returns | `config.go` signature | the swap does not compile | read both signatures: both `([]*PeerSettings, error)` | confirmed |
| A-2 | The flat parser is unreachable in production | `runReload` short-circuit, plugin transaction callers | a stdin daemon would start failing reconcile differently | read `runReload` (`cmd/ze/hub/main_reload.go`), `reloadFull` (`internal/component/plugin/server/reload.go`), the transaction hooks (`internal/component/bgp/plugin/register.go`) | confirmed |
| A-3 | No production caller depends on the fallback's hard error for an incomplete peer | only `VerifyConfig`, `ApplyConfigDiff`, `peerDiffCount` and `ReconcilePeersWithJournal` call it, and all four take the `reloadFn` branch in production | an incomplete peer would be skipped where it used to be refused | `gopls references` on `loadPeersFullOrTree`, then read each caller | confirmed |
| A-4 | The difference is visible only to the reactor's own tests | same reference set | a `.ci` would go red | `gopls references`; the whole reactor, bgp/config and bgp/plugin suites pass | confirmed |
| A-5 | `parseUint32FromString` survives the deletion | `applyPeerOperation` (`operation.go`) also calls it | the build breaks | `gopls references` names `operation.go` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A migrated test passes vacuously, because a wrongly shaped tree now yields zero peers and no error instead of an error | the test still passes when the tree shape is reverted | Proved by reverting `makeBGPTree` to the flat shape: five of six go red. The sixth carries no peer. Two tests that asserted only "no error" gained a peer-count assertion |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible. The deleted branch was unreachable in production (Reachability, above); the reachable route is untouched |
| How is it reverted? | Single commit revert. No config migration, no wire change |
| Who else touches this path? | `plan/spec-review-typed-config-decode.md` lists `loadPeersFullOrTree` under Files to Modify with "keep test fallback". That fallback is kept, and now shares the typed parser that spec is about |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by .claude/hooks/validate-spec.sh, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A reactor with no config path verifies a BGP tree (`reactorAPIAdapter.VerifyConfig`) | → | `loadPeersFullOrTree` → `PeersFromTree` | `TestReactorVerifyConfigValid` |
| A reactor with no config path applies a BGP tree (`reactorAPIAdapter.ApplyConfigDiff`) | → | `loadPeersFullOrTree` → `PeersFromTree` | `TestReactorApplyConfigDiffAddPeer` |
| The plugin transaction's verify budget (`Reactor.PeerDiffCount`) | → | `peerDiffCount` → `loadPeersFullOrTree` → `PeersFromTree` | `TestBGPVerifyEstimate` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A BGP tree in the shape `grouping peer-fields` produces, given to a reactor with no config path | Its peers are parsed, with their remote address, both AS numbers and their timers |
| AC-2 | The same tree, verified | Verification succeeds AND the peers really parse, so the pass is not an empty result |
| AC-3 | A peer whose remote IP does not parse | The whole call fails with an error quoting the value |
| AC-4 | A peer whose `router-id` is `0.0.0.0` or malformed (RFC 6286 Section 2.1) | The whole call fails, names the peer, quotes the value, and returns no peers, rather than skipping that peer |
| AC-5 | The package after the change | It holds one peer parser. `parsePeersFromTree` no longer exists |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Commits a config that adds a peer | plugin transaction -> `ReconcilePeersWithJournal` -> `loadPeersFullOrTree` -> `reloadFn` -> `PeersFromTree` | `TestReactorApplyConfigDiffAddPeer` covers the tree route; the file route is unchanged and covered by `TestReactorReloadBackwardCompat` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReactorVerifyConfigValid` | `internal/component/bgp/reactor/reload_test.go` | AC-1, AC-2: a nested tree verifies and really produces both peers | pass |
| `TestReactorVerifyConfigInvalidAddress` | `internal/component/bgp/reactor/reload_test.go` | AC-3: an unparseable remote IP fails and the error quotes it | pass |
| `TestReactorVerifyConfigNoMutation` | `internal/component/bgp/reactor/reload_test.go` | AC-2: verify is read-only, over a tree proven to parse | pass |
| `TestReactorApplyConfigDiffAddPeer` | `internal/component/bgp/reactor/reload_test.go` | AC-1: the peer reaches the reactor at its configured address | pass |
| `TestReactorApplyConfigDiffRemovePeer` | `internal/component/bgp/reactor/reload_test.go` | AC-1: an empty peer list removes the peer | pass |
| `TestReactorApplyConfigDiffChangedPeer` | `internal/component/bgp/reactor/reload_test.go` | AC-1: `timer > receive-hold-time` reaches `PeerSettings.ReceiveHoldTime` | pass |
| `TestBGPVerifyEstimate` | `internal/component/bgp/reactor/reactor_api_test.go` | AC-1: the diff count is proportional, over peers proven to parse | pass |
| `TestPeersFromTreeRejectsBadRouterID` | `internal/component/bgp/reactor/reactor_api_test.go` | AC-4: RFC 6286 Section 2.1 rejection at the tree entry point, not the helper | pass |
| `TestPeersFromTreeRejectsWrongTypedPeerSection` | `internal/component/bgp/reactor/reactor_api_test.go` | AC-3 extended by the Review Gate: an unreadable `peer` node is refused, and an absent one is still no peers and no error | pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `router-id` | non-zero uint32 (RFC 6286 Section 2.1) | `10.0.0.1` | `0.0.0.0` | N/A (uint32 is the field width) |

The timer boundaries this spec's parser inherits (`receive-hold-time` 0 or >= 3,
`send-hold-time` 0 or >= 480) are owned and tested by `parsePeerSettings`
(`internal/component/bgp/reactor/config.go`), not re-tested here.

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
Not applicable, and the reason is the point of the spec: the deleted branch was
unreachable from any user-facing entry point (Reachability). The route a user
does reach, the file-config transaction, is unchanged by this spec and keeps its
existing `.ci` coverage.

### Interop Tests (Scope: protocol)
Not applicable. Scope is config, and no wire-visible behavior changes.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/component/bgp/reactor/reactor_api.go` - delete `parsePeersFromTree`;
  `loadPeersFullOrTree` calls `PeersFromTree`; three stale doc comments corrected.
- `internal/component/bgp/reactor/reload_test.go` - `makeBGPTree` builds the
  nested shape; two tests gain a peer-count assertion.
- `internal/component/bgp/reactor/reactor_api_test.go` - `TestBGPVerifyEstimate`
  trees reshaped; the router-id test retargeted at `PeersFromTree`; the
  wrong-typed peer section test added (Review Gate round 1).
- `internal/component/bgp/reactor/config.go` - added during the Review Gate:
  `PeersFromTree` reads the `peer` node's raw value, so an unreadable node is an
  error rather than an empty peer set. Not in the original file list, because the
  defect it fixes only became reachable when this spec routed the fallback here.

## Files to Create
None. This spec only deletes and redirects.

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No leaf added or moved. The change makes Go read the shape the YANG already declares |
| YANG validation constraints | No | No new leaf |
| YANG custom validators | No | No new leaf. The existing `ze:validate "nonzero-ipv4"` on `router-id` is untouched |
| CLI commands/flags | No | No CLI surface |
| CLI grammar (keyword before value) | N-A | No CLI surface |
| Editor autocomplete | No | No new leaf |
| Functional test for new RPC/API | No | No new RPC or API. See Functional Tests above for why none is added |
| Pipe completeness | N-A | No command output |
| Env var registration | No | No env var |
| Doctor check for runtime dependencies | No | No new file path, socket, port, module, binary, or certificate |
| Prometheus counters/metrics | No | No new observable state. `configReloadErrors` and `configReloads` are unchanged |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability, or attribute touched |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Nothing added; a duplicate parser removed |
| 2 | Config syntax changed? | No | The YANG shape is unchanged. Go now reads the shape it already declared |
| 3 | CLI command added/changed? | No | No CLI surface |
| 4 | API/RPC added/changed? | No | `loadPeersFullOrTree` is unexported; no exported signature changed |
| 5 | Plugin added/changed? | No | `internal/component/bgp/plugin/register.go` is unchanged |
| 6 | Has a user guide page? | No | Internal parser, no user-facing page |
| 7 | Wire format changed? | No | Config parsing only |
| 8 | Plugin SDK/protocol changed? | No | The transaction's `ConfigSection` payload is unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 6286 Section 2.1 rejection is preserved, not newly proven: `parseRouterID` already enforced it and `TestParsePeerFromTreeInvalid` already carried the tagged row |
| 10 | Test infrastructure changed? | No | No runner, harness, or fixture format changed |
| 11 | Affects daemon comparison? | No | No behavior an operator compares |
| 12 | Internal architecture changed? | No | `docs/architecture/core-design.md` describes `peerSettingsEqual`, which is untouched. No doc describes the fallback branch |
| 13 | Route metadata keys added/changed? | No | None touched |
| 14 | Prometheus counters added/changed? | No | None touched |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registered |
| 16 | Any changed source file referenced by existing doc source anchors? | No | Six anchors name `reactor_api.go`: `DrainPeerSync`, `getMatchingPeersSel`, command dispatch, `apiStateObserver`, `peersSynced`, `OnPeerEstablished`/`OnPeerClosed`. None names `parsePeersFromTree` or `loadPeersFullOrTree`, and none is stale |
| 17 | Existing docs show config/CLI/API examples for this area? | No | `ai/digests/config-pipeline.md` describes the reload path through `createReloadFunc`, which is unchanged. It never described the fallback |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

One phase. The entry points in the Wiring Test table already exist and are
already reached by tests, so there is no registration to add: the wiring step is
proving those tests still bind the entry point to the parser after the swap.

1. **Phase: one parser** -- delete `parsePeersFromTree`, point
   `loadPeersFullOrTree` at `PeersFromTree`, migrate the tests that encoded the
   flat shape.
   - Tests: every row of the TDD Test Plan.
   - Files: the three in Files to Modify.
   - Verify: `make ze-test-pkg PKG=./internal/component/bgp/reactor` green, and
     the migrated tests go RED when `makeBGPTree` is reverted to the flat shape
     (R-1). Then `make ze-lint-changed`.

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has a named test in the TDD Test Plan |
| Feature completeness | Both branches of `loadPeersFullOrTree` reach `PeersFromTree`, and the `reloadFn` branch is unchanged |
| Correctness | No caller of `loadPeersFullOrTree` depends on the deleted parser's hard error for an incomplete peer (A-3) |
| Test discrimination | Every migrated test goes RED on the flat shape, or its assertion is proven not to carry a peer shape (R-1, `ai/rules/interop-and-goal-validation.md`) |
| Coverage | Every deleted or renamed test has its proof named somewhere that still runs |
| Naming | The retargeted test's name says what it drives: `PeersFromTree`, not `parsePeersFromTree` |
| Data flow | The package holds one peer parser. `gopls symbols` on `reactor_api.go` shows no `parsePeersFromTree` |
| Rule: `ai/rules/no-layering.md` | The old parser is DELETED, not left beside the new one |
| Rule: `ai/rules/stale-comments.md` | The three doc comments naming `parsePeersFromTree` are corrected, not left describing a function that is gone |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| `parsePeersFromTree` no longer exists | `gopls symbols internal/component/bgp/reactor/reactor_api.go` names it nowhere |
| The fallback calls `PeersFromTree` | `grep -n "PeersFromTree(bgpTree)" internal/component/bgp/reactor/reactor_api.go` |
| The reactor suite is green | `make ze-test-pkg PKG=./internal/component/bgp/reactor` |
| The migrated tests discriminate | revert `makeBGPTree` to the flat shape and re-run: five of six go red |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | The config tree is operator input. The new parser validates MORE of it than the deleted one: it requires `connection > local > ip`, rejects `ip dynamic` on a static peer, and validates the local address as a global next-hop (RFC 2545 Section 3, `applyLocalAddress`, `config_nexthop_form.go`) |
| Fail closed | A bad `router-id` refuses the whole tree and returns no peers, so no peer can reach the reactor with a zero BGP Identifier. Pinned by `TestPeersFromTreeRejectsBadRouterID` |
| Silent acceptance | `PeersFromTree` SKIPS a peer whose error wraps `ErrIncompleteConfig`, and logs a warning. The two tests whose only assertion was "no error" now assert the peer count, so a config that parses to nothing cannot read as a pass |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior; if misunderstood → RESEARCH |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->
- **A "test-only" fallback drifts because nothing outside the tests reads it.**
  The flat parser stayed correct for its own tests for as long as the tests were
  its only caller. The YANG moved, the tests did not, and the green bar said
  nothing. A fallback that shares the production parser cannot drift this way:
  it fails the same tests the production route fails.
- **The vacuity moved with the parser.** The old parser refused an unrecognised
  shape; the new one skips the peer and returns success with an empty list. So
  a test asserting only "no error" survived the migration while losing its
  meaning. Migrating a test to a more forgiving parser needs the assertion
  re-checked, not just the fixture reshaped.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Point the fallback at `PeersFromTree` | Port `parsePeersFromTree` to the nested shape | Porting leaves two parsers of one shape in one package. That duplication already produced the router-id divergence the deleted function's own comment records |
| Keep the fallback branch | Delete it, so `loadPeersFullOrTree` only ever calls `reloadFn` | A reactor with a nil `reloadFn` reaches the function from three unexported callers, so deleting the branch panics rather than parses. `plan/spec-review-typed-config-decode.md` also lists the function with "keep test fallback" |
| Keep the router-id test, retargeted | Delete it as duplicated by `TestParsePeerFromTreeInvalid` | That table drives `parsePeerFromTree`, the helper. This test drives the tree-level entry point, and what it proves there is unique: a bad router-id is NOT swallowed by the skip-and-continue path `ErrIncompleteConfig` opens |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     needs a row in the deferral shard named in the metadata table. -->
- The fallback branch is still reachable only from tests. This spec makes it read
  the same shape and use the same parser as production; it does not give it a
  production caller, and no spec claims it needs one.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes (the pre-commit gate; `ai/rules/git-safety.md`)
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
- `parsePeersFromTree` is deleted from `internal/component/bgp/reactor/reactor_api.go`
  (123 lines). `gopls symbols` on that file names `loadPeersFullOrTree` and no
  second peer parser.
- `loadPeersFullOrTree` returns `PeersFromTree(bgpTree)` (`reactor_api.go`,466).
  Its `reloadFn` branch is untouched, so both routes now end at one parser.
- Three doc comments that described the deleted parser are corrected:
  `VerifyConfig`, `ApplyConfigDiff` and `loadPeersFullOrTree` itself
  (`ai/rules/stale-comments.md`).
- `makeBGPTree` (`reload_test.go`) builds the nested shape `grouping peer-fields`
  declares: transport under `connection`, AS numbers under `session > asn`,
  timers under `timer`.
- `TestBGPVerifyEstimate` trees reshaped, and its global local AS added under
  `bgp > session > asn > local`, which `PeersFromTree` requires.
- `TestParsePeersFromTreeRejectsBadRouterID` is renamed
  `TestPeersFromTreeRejectsBadRouterID` and retargeted at `PeersFromTree`. No
  test was deleted.
- `PeersFromTree` (`config.go`) reads the `peer` node's raw value and refuses a
  node that is not a map, naming the type it found. This restores a guard the
  deletion had taken away. `TestPeersFromTreeRejectsWrongTypedPeerSection` covers
  it, including the absent case that must stay a success.

### Bugs Found/Fixed
- The migration carried its own vacuity. `PeersFromTree` skips a peer whose error
  wraps `ErrIncompleteConfig` (`config.go`,573) and returns success, so
  `TestReactorVerifyConfigValid` and `TestReactorVerifyConfigNoMutation`, whose
  only assertion was "no error", would have passed on a tree nobody can read.
  Both now assert the peer count.
- A guard lost with the deletion, found by the Review Gate. The deleted parser
  answered a `peer` node of the wrong type with `invalid peer section type: %T`.
  `PeersFromTree` reached it through `mapMap` (`config.go`,946), which answers
  "key absent" and "key present, wrong type" with the same `false`, and turned
  that into `nil, nil`. So a `peer` node arriving as a string, a list or a number
  read as "no peers configured", and `ApplyConfigDiff` applies that by
  reconciling the reactor to zero peers: every session torn down, no error
  anywhere. The guard is restored in `PeersFromTree`, which gives it to the
  production route as well as to the fallback.

### Documentation Updates
- None. Six `<!-- source: ... reactor_api.go -->` anchors exist
  (`docs/functional-tests.md`, `docs/guide/command-reference.md`,
  `docs/architecture/hub-api-commands.md`, `docs/architecture/behavior/fsm.md`,
  `docs/architecture/api/text-format.md`, `docs/architecture/api/commands.md`).
  Each names `DrainPeerSync`, `getMatchingPeersSel`, command dispatch,
  `apiStateObserver`, `OnPeerEstablished`/`OnPeerClosed`, or `peersSynced`. None
  names the deleted parser or `loadPeersFullOrTree`.
- `grep -rn "loadPeersFullOrTree" docs ai` returns nothing, so no document
  described the fallback branch. `ai/digests/config-pipeline.md` describes the
  production route through `PeersFromTree`, which this change does not alter.
- `make ze-doc-test` runs every stage green except one, and that one is another
  session's: `rfc/requirements/rfc7911.md is stale vs its sources`. That file is
  derived entirely from `forward_path_id*.go`, `forward_body*.go` and
  `session_validation_nlritype_bypass_test.go`, and a live one-line probe in
  `forward_path_id.go` shifts every tagged line below it. No file in this commit
  carries an `RFC requirement:` tag. Regenerating the index here would commit
  that probe's line numbers, so it is left to the session that owns it.

### Deviations from Plan
- The spec's Reachability paragraph, written 2026-07-27, said a stdin daemon
  (`ze -`) reaches the fallback. It reaches the fallback's SELECTION condition
  only. It was corrected in place during implementation, and the same false claim
  was corrected in `plan/deferrals/ad-hoc-2026-07-27-423eaa77.md`.
- The surviving parser is stricter than the "Behavior to change" list states, in
  three places the deleted parser only warned about: an out-of-range
  `receive-hold-time`, `send-hold-time` or `keepalive` is refused rather than
  ignored, and `connect false` with `accept false` is refused rather than
  silently reset to `ConnectionBoth` (`parsePeerSettings`, `config.go`). This is
  the production parser's existing behaviour, reached now by one more caller, not
  a behaviour this diff wrote. `connection > local > ip` is required on the same
  terms.
- `internal/component/bgp/reactor/config.go` joined the file list during the
  Review Gate. The deletion had taken a guard away with the parser, and the fix
  belongs in `PeersFromTree` rather than at the call site, so both routes get it.
  Fixing it here rather than deferring it follows `ai/rules/completion.md`: the
  goal of this spec is that the fallback reads config through the production
  parser, so a hole in that parser is a defect this work is the entry point for.
- The first version of the new `loadPeersFullOrTree` comment said the two routes
  "cannot disagree about what a peer's config means". Both Review Gate lenses
  refused it: the file route runs `PruneInactive`, `ResolveBGPTree`,
  `CheckRequiredFields` and `applyPeerSchemaDefaults` before the parse and
  `patchRoutes` after it, so the routes share the last stage only. The comment now
  says that, and it restores the `peerSettingsEqual` rationale the deleted comment
  carried.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The 2026-07-27 record claimed the flat parser was reachable in production through a stdin config | `runReload` (`cmd/ze/hub/main_reload.go`) short-circuits on a nil loader into `Server.ReloadFromDisk`, whose `reloadFull` (`internal/component/plugin/server/reload.go`) answers `errNoConfigLoaderConfigured`, so nothing calls the enclosing function on a stdin daemon | Reading the producer while validating A-2, rather than the caller that suggested it | Corrected in the spec's Reachability paragraph and in the deferral row that carried the same claim |
| approach | Two migrated tests were reshaped and left asserting only "no error" | A more forgiving parser makes that assertion vacuous: an unreadable tree yields zero peers and no error | Reading `PeersFromTree`'s skip branch after the fixture change | Both tests gained a peer-count assertion |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| One peer parser in the package | Done | `internal/component/bgp/reactor/reactor_api.go` | `parsePeersFromTree` deleted; `gopls symbols` names no second parser |
| The fallback calls `PeersFromTree` | Done | `reactor_api.go`,466 `loadPeersFullOrTree` | `return PeersFromTree(bgpTree)` |
| Its tests migrate to the nested shape | Done | `reload_test.go` `makeBGPTree`, `reactor_api_test.go` `TestBGPVerifyEstimate`, `TestPeersFromTreeRejectsBadRouterID` | No test deleted |
| The stale doc comments are corrected | Done | `reactor_api.go` `VerifyConfig`, `ApplyConfigDiff`, `loadPeersFullOrTree` | None names the deleted function |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestReactorApplyConfigDiffAddPeer`, `TestReactorApplyConfigDiffChangedPeer`, `TestBGPVerifyEstimate` | `timer > receive-hold-time` reaches `PeerSettings.ReceiveHoldTime`; both AS numbers reach the peer |
| AC-2 | Done | `TestReactorVerifyConfigValid`, `TestReactorVerifyConfigNoMutation` | Each now asserts the peer count, so a pass cannot be an empty result |
| AC-3 | Done | `TestReactorVerifyConfigInvalidAddress` | `parsePeerFromTree` (`config.go`,86-89) quotes the value; the error does not wrap `ErrIncompleteConfig`, so the call fails |
| AC-4 | Done | `TestPeersFromTreeRejectsBadRouterID` | `parseRouterID` (`config.go`,995) rejects `0.0.0.0` and a malformed value; its error is unwrapped, so `PeersFromTree` (`config.go`,573) refuses the whole tree |
| AC-5 | Done | `gopls symbols internal/component/bgp/reactor/reactor_api.go` | Only `loadPeersFullOrTree` appears; `parsePeersFromTree` is absent |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestReactorVerifyConfigValid` | Done | `reload_test.go` | Peer-count assertion added |
| `TestReactorVerifyConfigInvalidAddress` | Done | `reload_test.go` | Unchanged assertion, reshaped fixture |
| `TestReactorVerifyConfigNoMutation` | Done | `reload_test.go` | Peer-count assertion added |
| `TestReactorApplyConfigDiffAddPeer` | Done | `reload_test.go` | |
| `TestReactorApplyConfigDiffRemovePeer` | Done | `reload_test.go` | |
| `TestReactorApplyConfigDiffChangedPeer` | Done | `reload_test.go` | |
| `TestBGPVerifyEstimate` | Done | `reactor_api_test.go` | `require.Len(existingPeers, 2)` added |
| `TestPeersFromTreeRejectsBadRouterID` | Done | `reactor_api_test.go` | Renamed and retargeted, not deleted |
| `TestPeersFromTreeRejectsWrongTypedPeerSection` | Done | `reactor_api_test.go` | Added by the Review Gate with the guard it covers |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/reactor/reactor_api.go` | Done | `git diff --numstat`: 14 added, 135 removed |
| `internal/component/bgp/reactor/reload_test.go` | Done | 27 added, 5 removed. Fixture reshaped, two assertions added |
| `internal/component/bgp/reactor/reactor_api_test.go` | Done | Trees reshaped, one test renamed, one test added |
| `internal/component/bgp/reactor/config.go` | Changed | Not in the original list. `PeersFromTree` gained the raw-value read for the `peer` node (Review Gate round 1), recorded in Deviations |

### Audit Summary
- **Total items:** 22 (4 requirements, 5 AC, 9 tests, 4 files)
- **Done:** 21
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (`config.go` was added to the file list during the Review Gate,
  recorded in Deviations, along with the Reachability correction)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| One peer parser in the package, so the two routes cannot disagree about what a peer's config means | structural, machine-checked | `gopls symbols internal/component/bgp/reactor/reactor_api.go` names `(*reactorAPIAdapter).loadPeersFullOrTree` and no `parsePeersFromTree`. `grep -n "PeersFromTree(bgpTree)" reactor_api.go` returns line 466 |
| The fallback reads the shape the YANG produces | functional (unit, whole package) | `make ze-test-pkg PKG=./internal/component/bgp/reactor` ok in 21.061s, plus `./internal/component/bgp/config` ok 66.903s and `./internal/component/bgp/plugin` ok 1.439s. A later run of the reactor package is RED with 8 failures, every one a `TestForwardPathID*` or `TestForwardSplit*`: another session added `return received // DISCRIMINATION PROBE, REMOVE` to `forward_path_id.go` `(*fwdPathIDTable).generate`, a file this commit does not carry. No test in the four files under change fails |
| The migrated tests are not vacuous | discrimination measurement | A probe built the FLAT shape and called `PeersFromTree`: zero peers, no error. So every peer-count assertion (`Len(peers,2)`, `Len(parsed,1)`, `Len(existingPeers,2)`) fails on the flat fixture, which is the shape the tests carried before this change |
| RFC 6286 Section 2.1 rejection survives at the tree entry point | tagged unit test | `TestPeersFromTreeRejectsBadRouterID`. Verified the test reaches the check: `parsePeerSettings` (`config.go`,167) refuses a missing local AS before parsing router-id at ,171, and the test's tree states `session > asn > local` 65000, so the peer is not skipped first |
| Routing the fallback through the production parser leaves that parser no weaker than the one it replaced | removed-behaviour audit plus regression test | The Review Gate walked every field the deleted parser read and found each re-established in `parsePeerFromTree` / `parsePeerSettings`, except the wrong-typed `peer` node. That one is restored in `PeersFromTree` and covered by `TestPeersFromTreeRejectsWrongTypedPeerSection`. Discrimination measured: a probe proved `mapMap` answers `false` for a string, a list and a number, so the pre-fix code returned no peers and no error for all three and the new test reds on it |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| `plan/deferrals/ad-hoc-2026-07-27-423eaa77.md`, row 2026-07-27 "rfc6286 review NIT-1 follow-through": `parsePeersFromTree` reads a peer shape the YANG no longer produces | done | The function is deleted and the fallback calls `PeersFromTree` (`reactor_api.go`,466). The row's Status cell moves from `deferred` to `done` and its Destination names this spec as closed. The shard is NOT removed: three of its seven rows are still live (`zefs-diff-structural-ops` deferred, `netns-test-dut-tags` deferred, and the `spec-release-evidence-gate` re-audit half marked half-done), so it outlives this spec |
| Same shard, the 2026-07-27 `validateCommandArgs` row marked done | not this spec's to resolve | Its closing update withdraws one reproduction and records that the `show interface rate` alias-contract defect "remains OPEN and unhomed". The row's Status is terminal, so `deferral_unassigned_problems` does not see the fragment, and no spec on disk owns it. This closure does not invent one: a destination naming a spec that does not exist is a dangling citation, and the open-spec count is at its cap. The question goes to the owner instead |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-peers-from-tree-stale-shape-ca112cd4-8337-4992-b4e1-e0d7bbff5820.md` |
| `review_gate.py check` | clean. `review_gate: OK (4 code files, clean, hashes match ...)`, exit 0 |
| Rounds | 2. Round 1 covered the whole diff on two independent lenses and found two ISSUEs. Round 2 covered only round 1's fixes and found 0 BLOCKER, 0 ISSUE |
| Reviewer lenses used | Round 1 lens A: logic, wiring, removed-behaviour audit, test discrimination. Round 1 lens B: security, guard behaviour, RFC conformance, edge cases. Round 2: the three round-1 fixes and what they touched |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | A wrong-typed `peer` node lost its error and read as "no peers". `mapMap` answers "key absent" and "key present, wrong type" with the same `false`, and `PeersFromTree` turned that into `nil, nil`, which `ApplyConfigDiff` applies by reconciling to zero peers. The deleted parser had answered `invalid peer section type: %T` | `internal/component/bgp/reactor/config.go` `PeersFromTree`, via `mapMap` (,946) | `PeersFromTree` reads the raw value: `nil, nil` only when the key is absent, an error naming the type otherwise. Regression test `TestPeersFromTreeRejectsWrongTypedPeerSection`. Both lenses reached it, one as the removed-behaviour audit and one as the fail-open guard audit |
| 2 | ISSUE | The new `loadPeersFullOrTree` comment claimed the two routes "cannot disagree about what a peer's config means", and dropped the `peerSettingsEqual` rationale the deleted comment carried. The file route runs `PruneInactive`, `ResolveBGPTree`, `CheckRequiredFields` and `applyPeerSchemaDefaults` before the parse and `patchRoutes` after it | `internal/component/bgp/reactor/reactor_api.go` `loadPeersFullOrTree` | Rewritten to name that pipeline, to say the routes share one stage, and to restore the reason a half-populated `PeerSettings` reads as a change |

### Findings recorded, not blocking (NOTE)
| # | Finding | Location | Why it is a NOTE |
|---|---------|----------|------------------|
| 1 | A skipped peer is indistinguishable downstream from a deleted peer, and removal is silent | `reactor_api.go` `reconcilePeersJournaled`, `peerDiffCount` | Not introduced here. The production route reaches the same parser and is guarded by `CheckRequiredFields` |
| 2 | The `// RFC 9687` comment on the send-hold-time check carries no Section and no quoted requirement | `config.go` `parsePeerSettings` | Outside the diff, and RFC 9687 has no `rfc/short/` summary, so it is outside the RFC gate's population |
| 3 | No test drives a wrong-typed `peer` node from a transaction entry point | `reactor.go` `ReconcilePeersWithJournal`, `reactor_api.go` `peerDiffCount` | Propagation is one unconditional `if err != nil` in each, and round 2 read both. Coverage, not a defect |
| 4 | A `peer` key present with a JSON `null` now errors where it previously read as no peers | `config.go` `PeersFromTree` | The one input whose treatment changed. No producer emits it: `Tree.ToMap` omits an empty list and `ResolveBGPTree` omits an empty peer map. It fails in the safe direction |
| 5 | The comment said the routes "share the last stage only", but the file route continues past the parser | `reactor_api.go` `loadPeersFullOrTree` | Round 2's only wording finding. Corrected in one edit to "the only stage the two routes share", which is what the source supports. A record defect earns no further round |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/reactor/reactor_api.go` | Yes | `ls -l` 42K, 14 Aug 17:05 |
| `internal/component/bgp/reactor/reactor_api_test.go` | Yes | `ls -l` 16K, 14 Aug 17:07 |
| `internal/component/bgp/reactor/reload_test.go` | Yes | `ls -l` 24K, 14 Aug 17:10 |
| `internal/component/bgp/reactor/config.go` | Yes | `git status --porcelain internal/component/bgp/reactor/` lists it modified |

Files to Create was None, and neither the Wiring Test nor Functional Tests names
a `.ci`, so no other path is owed a row here.

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | A nested tree parses into peers with address, both AS numbers and timers | `make ze-test-pkg PKG=./internal/component/bgp/reactor` ok 21.061s, covering `TestReactorApplyConfigDiffAddPeer`, `...ChangedPeer` and `TestBGPVerifyEstimate` |
| AC-2 | Verification succeeds and the peers really parse | `reload_test.go` carries `assert.Len(peers, 2, ...)` and `require.Len(parsed, 1, ...)` after each `VerifyConfig` |
| AC-3 | An unparseable remote IP fails with the value quoted | `config.go`,86-89 `parsePeerFromTree` returns `invalid remote ip %q`, unwrapped, so `PeersFromTree` returns it |
| AC-4 | A bad `router-id` refuses the tree and returns no peers | `config.go`,995-1005 `parseRouterID`; `config.go`,571-578 returns `nil, err` because the error does not wrap `ErrIncompleteConfig` |
| AC-5 | The package holds one peer parser | `gopls symbols internal/component/bgp/reactor/reactor_api.go`: no `parsePeersFromTree` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `reactorAPIAdapter.VerifyConfig` over a BGP tree | none; unit `TestReactorVerifyConfigValid` (`reload_test.go`) | Yes. Read the test: it builds the tree, calls `VerifyConfig`, then `loadPeersFullOrTree` and asserts two peers |
| `reactorAPIAdapter.ApplyConfigDiff` over a BGP tree | none; unit `TestReactorApplyConfigDiffAddPeer` (`reload_test.go`) | Yes. Read the test: the peer reaches the reactor at its configured address |
| `Reactor.PeerDiffCount` verify budget | none; unit `TestBGPVerifyEstimate` (`reactor_api_test.go`) | Yes. Read the test: it pre-adds peers through `PeersFromTree` and asserts the count is 2 before diffing |

No `.ci` is owed. The branch under change is reachable from tests only, which is
the spec's own finding (Reachability), and the user-reachable route through the
plugin config transaction is unchanged and keeps its existing coverage.

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Both `PeersFromTree` (`config.go`,527) and `loadPeersFullOrTree` (`reactor_api.go`,454) are `([]*PeerSettings, error)`; the package compiles and its tests pass |
| A-2 | confirmed | `runReload` (`cmd/ze/hub/main_reload.go`) into `reloadFull` (`internal/component/plugin/server/reload.go`) answers `errNoConfigLoaderConfigured` |
| A-3 | confirmed | Every caller read; the four production entries take the `reloadFn` branch |
| A-4 | confirmed | `make ze-test-pkg` green for reactor, `bgp/config` and `bgp/plugin` |
| A-5 | confirmed | `operation.go`,248 still calls `parseUint32FromString`, which the diff keeps |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No source anchor names the changed symbols | `grep -rn "source: internal/component/bgp/reactor/reactor_api.go" docs ai`: six hits, each naming a different symbol, none stale | Yes |
| No document describes the fallback branch | `grep -rn "loadPeersFullOrTree" docs ai`: no hits | Yes |
| The config-pipeline digest still matches | `ai/digests/config-pipeline.md` describes `PeersFromTree` on the production route, which this change does not alter | Yes |
| `make ze-doc-test` | every stage green except `rfc/requirements/rfc7911.md is stale`, attributed above to another session's in-flight work. The stages this change could touch, discovery indexes and digest anchors, both report up to date | Yes |

## Core Insight

A fallback that no production caller reaches drifts silently, because its own
tests are the only thing that reads it, and they drift with it. The fix is not to
correct the fallback: it is to delete its parser and let the branch call the
production one. A shared parser cannot drift, because the same tests that would
catch the production route catch this one.
