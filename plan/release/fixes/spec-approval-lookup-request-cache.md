# Spec: reuse approval lookup work within one request

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P1 |
| Release reason | Repeated approval scans delay native commit preparation and required evidence publication by tens of minutes. |
| Scope | tooling |
| Depends | None |
| Phase | closure ready — native implementation and removal commits pending |
| Handoff | `tmp/session/2026-11-14-210034/state/session-state-approval-lookup-request-cache-01a1db03-fbc1-7b32-8cbf-72b6284c110b.md` |
| Updated | 2026-11-16 |

## Task

Make native approval lookup reuse its inventory, source reads, declaration
summaries and landed-history walk within one request. Preserve every approval
decision, identity, ambiguity check, digest, refusal and publication guard.
This is a performance repair to the existing native path. It adds no CLI,
approval escape, persistent cache, executor or alternative commit route.

Main accepted this correctness-preserving repair and released the coordinated
source hold. Independent design review approved request-local reuse with the
preservation controls below. Main observed the existing-loader regression red
in run `bae24a0f`: only operation counts failed (three inventories, six base
reads and four history walks/streams). Its row/result assertions and fresh-load
control passed. Main then released implementation. The implementation and
additional semantic controls are written. Main observed both loader roots
PASS in `job-atomic-approval-cache-green-bae24a0f.log`. Independent code review
found no concrete defect in the seven Go paths. The weakening and commit
package suites passed. The RFC package timed out in the corpus traversal
after the named approval controls passed. The focused RFC approval run then
passed in `661d17b3` (14.062s). The original native Create dry-run then
completed successfully, as recorded below. Final owned-scope lint analysis is
complete with no cache findings; native closure publication remains pending. The final post-guard scoped populations also passed
in `315e613f`: RFC 25.608s, weakening 45.425s and commit 140.861s.
The parent checked for an existing spec with this scope and found none.

### Observed blocker and provenance

The parent observed a native `commit create` for 54 selected paths and dozens
of approvals running for approximately 39 minutes before it refused. During
the wait, repetitive Git children and parser work continued without output.
That duration is an observation from this one campaign, not a benchmark or
a claim about every invocation. The process has ended. This spec authorizes
no cancellation, restart, approval pruning or bypass.

The parent originally captured a one-second native Apple sample at
`tmp/session/2026-11-14-210034/scratch/atomic-proof/commit-create-sample.txt`.
Main has removed that obsolete sample after AC-8 and the retained operation
counts superseded the latency diagnosis; its absence was checked. Its observed
stack included `commit.checkSourceGates` → `rfcChangeProblems` →
`weakened.LoadApprovals` → `staleApproval` →
`rfc.ResolveApprovalUnitContentIn` → `resolveApprovalSides` →
`approvalUnitTexts` → `readFunctionUnits` → `DeclaredUnits`, plus Git process
launch/wait frames. Source inspection explains the repeated work below.
The sample did not assign a measured fraction of elapsed time to each cause.
The record above preserves that limit; no sample or duplicate copy is retained.

## Required Reading

| Read | Actionable constraint |
|------|-----------------------|
| `ai/doors/system.md`, `ai/doors/le-tooling.md` | → Constraint: extend the existing native tool path. Product code must not depend on this tooling cache |
| `ai/rules/commands.md` | → Constraint: main runs scoped tests through named native actions. No agent test/build/formatter or whole-tree verification |
| `ai/rules/git-safety.md`, `docs/contributing/committing.md`, selected-hunk section | → Constraint: preserve selected blobs, prepared-parent and publication freshness checks; never replace them with working-tree fallback |
| `docs/contributing/rfc-implementation-guide.md`, judge approval section | → Constraint: preserve canonical directory identity, exact Go boundaries, Content binding, same-carrier target bindings and raw length-framed history validation |
| `internal/le/rfc/approvalunits.go` and `goscope.go` | → Constraint: cache existing resolution inputs/results without changing receiver grouping, file scope, parse failures, moves or duplicate handling |
| `internal/le/rfc/approve.go` and `approvalhistory.go` | → Constraint: one shared history stream must return each row's result and validate the complete stream before any result authorizes approval |
| `internal/le/test/weakened/ledger.go` and `internal/le/commit/rfcchange.go` | → Constraint: keep one loader authoritative for hooks, weakening, RFC changes and publication callers; used/stale/unselected rows retain their existing meaning |

## Current Behavior

| Producer | Repeated work |
|----------|---------------|
| `weakened/ledger.go::LoadApprovals` | For every valid ordinary row not already landed, calls `staleApproval` independently |
| `approvalunits.go::resolveApprovalSides` | For every unit, lists old/current paths again, rebuilds path sets and scans candidate carriers with the matching directory basename |
| `approvalunits.go::approvalSide` | Launches `git show revision:path` for every matching base carrier, then reads its current side |
| `approvalunits.go::approvalUnitTexts` | Recomputes file-scope classification and declaration grouping for both sides of every candidate, although sibling approvals ask about the same files |
| `goscope.go::DeclaredUnits` | Performs the full Go declaration parse behind those repeated lookups. Tag-boundary classification can perform further existing scope reads |
| `approve.go::approvalLanded` | Each row independently lists history IDs and opens a `cat-file --batch` stream |
| `approvalhistory.go::readApprovalHistory` | Reads every retained commit for that row, including after finding its match, to validate framing and completion |

Before this repair, R approval rows in one package and P candidate carriers
repeated roughly R × P side reads and inventory construction. History I/O
similarly repeated its commit stream for each row. The profile and source
motivated the real-reader count regression; its observed RED/GREEN results
below now establish the repeated-operation bound, without a speedup benchmark.

### Behavior to preserve

- Directory-basename keys remain authoritative. Go package-clause aliases
  remain refused, with existing diagnostic behavior.
- Include every current eligible carrier, including tracked/untracked files
  outside the selected commit paths. An unselected carrier can cause ambiguity.
- Preserve qualifiers, both old/current membership, deleted units, file-scoped
  carriers, semantic-only edits and grouped same-name receiver methods.
- Preserve exact declaration text, doc comments, one-line boundaries and parse
  failures. A following declaration cannot enter another unit's digest.
- Preserve name-dependent parse refusals. A missing name remains absent, an
  inexact matched declaration remains refused, and existing file-stem or
  file-scope fallback can still bind full content in a malformed file.
- Preserve changed-carrier filtering for moves and simultaneous duplicates.
  Do not reduce a name to the first path found.
- Preserve full canonical targets, first-parent selection, target-change checks,
  Content admission, reason bytes, landed/spent state and selected-row behavior.
- Preserve exact ordinary trailer and bound-pair semantics in one commit,
  including duplicate verdict/binding checks and reason-renewal refusals.
- Preserve malformed/unreadable history refusal and complete raw framing.
  Even a found match cannot authorize a partial or failed stream.
- Preserve caller-selected trees/blobs and current publication-parent checks.
  A cache hit supplies data to those checks and never replaces a check.
- An empty set of validated rows performs no history or resolver work.

## Existing Mechanisms

| Mechanism | Producer | Disposition |
|-----------|----------|-------------|
| Shared ledger loader | `weakened/ledger.go::LoadApprovals` | Extend with one request-scoped lookup context, retaining its row validation and result shape |
| Canonical unit resolver | `approvalunits.go::ResolveApprovalUnitContentIn` and `resolveApprovalSides` | Extract an internal reusable resolver. Keep current public entry points as fresh-request wrappers |
| Declaration authority | `goscope.go::readFunctionUnits`, `DeclaredUnits`; `changedunits.go::tagFallsOutsideFunction` | Reuse their semantics. Construct and retain per-content summaries rather than replacing them with a regex or approximate boundary |
| Move/ambiguity judgment | `approvalunits.go::changedApprovalCarriers` | Apply unchanged to each unit's cached old/current carriers |
| Raw history reader | `approvalhistory.go::readApprovalHistory`, `approvalMessageLanded` | Extend to one result per requested row while retaining every frame and per-message predicate |
| Git read policy | `approvalunits.go::approvalGit`, `gitenv.ReadOnlyQuery` | Preserve discovery, byte handling and Git environment policy. Do not silently change replacement-object or membership behavior |
| Immutable blob batching | `internal/le/repo/objects/objects.go` | Existing option for future batching. Do not change its policy or add a second object protocol for this bounded cache repair |
| Selected-content reader | `commit/rfcchange.go::rfcChangeProblems`, `hunks.Selection.Read` | Preserve its authority. No whole working-tree substitution for selected hunk content |

## Proposed Design

### Alternatives

| Approach | Trade-off | Recommendation |
|----------|-----------|----------------|
| Request-scoped resolver and multi-row history lookup | Reuses inventory, content and declaration results for all rows, then releases memory. A fresh request observes later edits/history | Recommended. It removes the repeated work without persistent invalidation policy |
| Persistent package/global cache around the same resolver | Reuses work across commands but must invalidate on HEAD, index, untracked files, source bytes, qualifiers and selected-tree changes | Reject for this repair. It adds stale-authority risk and a wider lifecycle than the blocker requires |

### Source lookup

`LoadApprovals` creates one resolver for its existing source view and lifetime.
Retain the base/target identities and old/current membership inventories once.
Index eligible paths by the same directory-basename rule used today.
Load candidates lazily for each package requested, so unrelated packages
do not require source parsing. A qualified lookup can narrow candidates by
the existing exact-path rule, but cannot invent membership for an ignored file.

For each candidate side, retain the actual content and a summary containing
the existing file-scope classification and declarations grouped by name in
file order. Multiple requested names reuse that summary. Existing scope
helpers can retain their internal algorithms; this repair removes repeated
summary construction per row rather than changing their boundary semantics.
Cache absent/error results within the request with their fail-closed meaning.
Do not represent a read or parse error as an empty successful carrier set.

Per-unit resolution still collects old/current carriers, applies
`changedApprovalCarriers`, rejects ambiguity, validates target changes and
computes/adjudicates the same Content binding. Do not reuse only the path or
the final answer across unrelated content/view identities.

Retain immutable revisions already selected by callers. Each new load or
publication attempt creates a new resolver. No request context survives to
admit a later edit, new untracked duplicate, changed HEAD or publication retry.
Resolve ambient HEAD to its canonical commit once within that resolver, so
all base reads in one request refer to the same revision.
The existing selected-blob readers and actual-parent publication guards remain
in place. This proposal adds no new snapshot/export of source files.

### Landed history

Collect all ordinary and exact-target history questions from the validated
rows before resolving their spent state. Read the canonical history-ID list
once and stream its raw commit objects once for that load.
Return one result for each query in input order, retaining the same newest
matching carrier that independent lookups would choose.

The current multi-query helper returns only the first matched query, so its
result cannot be reused unchanged as an all-row answer. Extend the internal
reader to retain a match for each unresolved query while it validates every
subsequent frame and the end of the stream. A late framing error, failed Git
exit or trailing payload refuses the entire load, even if all rows matched.
Do not narrow history through message separators, lossy grep output or an
early-return optimization. Carrier display names remain opaque display data.

### Bounds and challenge

The request retains only the inventories and candidate content/summaries it
actually uses. History retains one raw commit payload at a time plus one
result per row. There is no worker, persistent file, server or global cache.
Retained strings/slices must own data that otherwise belongs to a reused
stream buffer. The strongest concern is an apparent cache hit accepting the
wrong source view or a partially validated history. Tests must decide that
boundary before measuring speed.

| Challenge | Assessment for main review |
|-----------|----------------------------|
| Simplicity | Four production files extend existing mechanisms. Keep batching/parallelism and persistent caching out of the first repair |
| Uniformity | One ledger loader and existing identity/history predicates remain authoritative for all callers |
| Performance | Package inventories/summaries and history bytes are processed once per load rather than per row. The real-reader count oracle passes; one native dry-run elapsed is recorded below without a speedup benchmark claim |

## Data Flow

1. Existing caller passes its normal approval source and source view to the loader.
2. Parse and validate rows exactly as today, retaining row order and reason bytes.
3. Create one request-scoped inventory/resolver and one batch of history queries.
4. Validate the entire history stream and retain each row's spent result.
5. Resolve each ordinary unspent row through shared candidate summaries.
6. Apply existing Content admission and return the existing row/problem shapes.
7. Existing callers select approvals and perform their normal selected-tree and publication-parent checks.
8. Release all request state. A later load starts from fresh inputs.

No external wire, CLI grammar, ledger schema, trailer or digest format changes.
Consumers are existing Go callers of `LoadApprovals` and resolver wrappers in
RFC approval, weakening, hooks and commit paths. The batch API remains internal
to tooling. Single-unit issuance still performs a complete fresh lookup.

## Acceptance Criteria

| ID | Input | Required outcome |
|----|-------|------------------|
| AC-1 | Many ordinary approvals naming different units in one package | Every valid unit retains the same Path/Base/Current and admission result. Package membership, candidate reads and summary construction do not repeat per row |
| AC-2 | Multiple ordinary/targeted rows with landed, unspent and stale controls | Each gets the same spent/stale result and carrier as independent lookups. One history-ID inventory and one commit-object stream serve the load |
| AC-3 | Unknown/aliased name, qualifier, move, deleted unit, same-directory duplicate, cross-directory collision and grouped receiver methods | Existing valid moves/deletions/method groups remain usable. Every existing ambiguity, alias and invalid qualifier refusal remains enforced |
| AC-4 | Exact one-line declarations, a changed following declaration and malformed source | Only exact unchanged semantics enter the digest. Following text cannot authorize a unit, and unresolved boundaries remain refused |
| AC-5 | A second request after editing content, adding an untracked duplicate, moving HEAD or changing selected hunk content | The fresh request observes the change. No previous cache authority survives to admit it |
| AC-6 | History matches followed by malformed/truncated/trailing data or a failed Git exit | The load fails closed. Ordinary and exact same-carrier pair rules, duplicate checks and unchanged reason data remain intact |
| AC-7 | Native preparation with multiple selected tagged units and foreign working-tree hunks | Existing selected-blob, unselected-row, actual-parent and spent-approval guards remain authoritative; no gate is bypassed or weakened |
| AC-8 | The original native workflow after scoped tests pass | Main records the actual native result, elapsed time and operation counts. The repeated scan multiplier is gone. A legitimate remaining refusal remains a refusal |

## Risks & Assumptions

| ID | Assumption / risk | Basis or early signal | Validation / mitigation |
|----|-------------------|-----------------------|-------------------------|
| A-1 | Per-row lookup repeats source and history work that a request can reuse | Sample stack and source loops motivated the hypothesis; real-reader RED/GREEN counts now demonstrate the repetition | Count oracle confirmed; native preparation completed. The stronger original dominance inference has no causal timing breakdown and is not claimed |
| R-1 | Cache survives across mutable views | A new edit/duplicate still passes with old Content | Request ownership only and AC-5 |
| R-2 | Candidate index loses an ambiguity | One selected path hides an unselected duplicate | Inventory all existing eligible carriers and preserve move/qualifier tests |
| R-3 | Cached summaries change exact boundaries | A one-line/following-declaration test changes its verdict | Reuse existing scope authority and AC-4 |
| R-4 | History batching returns only the first row or exits early | Later row is missed or malformed tail passes | Per-query ordered results plus full-stream validation and AC-2/AC-6 |
| R-5 | Borrowed blob/history storage is retained after reuse | One query receives another file/message's text | Own retained content and test multiple different payloads |
| R-6 | New reader silently changes Git policy | Fixture with a discovery/replacement/absent-history case changes result | Preserve current read policy; keep object-batch refactoring outside this repair |
| R-7 | A performance test accepts refusal or skips useful approvals | Fast run reports no useful rows | Require valid approvals and explicit stale/refused controls together; assert operations, not a fragile wall-clock threshold |

## Files to Modify or Create

| File | Minimal responsibility |
|------|------------------------|
| `internal/le/rfc/approvalunits.go` | Reusable request resolver, candidate inventory and content/declaration summaries; fresh-request compatibility wrappers |
| `internal/le/rfc/approve.go` | Batch history entry point that reuses canonical history collection and preserves single-query callers |
| `internal/le/rfc/approvalhistory.go` | Per-query results from one fully validated raw stream |
| `internal/le/test/weakened/ledger.go` | One resolver/history batch per `LoadApprovals`, preserving ledger validation and result behavior |
| `internal/le/rfc/approval_request_cache_test.go` (new) | Multiple real unit resolutions, shared read counts and fresh-request invalidation |
| `internal/le/rfc/approval_history_batch_test.go` (new) | Multiple independent landed questions and matched-then-malformed stream controls |
| `internal/le/test/weakened/approval_request_test.go` (new) | One real multi-row load proving useful/stale/spent outcomes and bounded repeated I/O |
| `internal/le/test/weakened/approve_test.go` | Update only the mutation comment to name the active `staleApprovalWith` producer |
| Existing approval and selected-hunk tests | Preserve the contracts below. Add only missing cases; do not weaken or duplicate their assertions |
| `docs/contributing/rfc-implementation-guide.md` | Explain request lifetime and unchanged approval/history semantics with implementation |
| `docs/architecture/testing/test-health.md` | Repoint the source anchor from removed `staleApproval` to active `staleApprovalWith` |
| `docs/contributing/committing.md`, selected-hunk/publication sections | Check existing guarantees against the implementation; amend only if internal lifetime needs explanation |

No atomic/BGP source files, launcher, generated script format or product runtime
files belong in this scope. Recheck file ownership before main releases Go edits.

## TDD and Wiring Tests

| Test | Behavior proved |
|------|------------------|
| `TestLoadApprovalsSharesOneRequest` | The actual loader, not a resolver called manually by a test, shares the index/history and retains useful, spent and stale controls. Real Git read counts expose repeated inventory and candidate reads |
| `TestLoadApprovalsStartsFresh` | A later load observes edited content and an untracked duplicate |
| `TestApprovalRequestPinsBaseAndStartsFresh` | One request retains its initial immutable base for an unread package; a later request observes the new HEAD |
| `TestApprovalRequestPreservesMalformedNameBoundaries` | Missing names stay absent, found inexact names refuse, and existing file-stem fallback retains full content |
| `TestApprovalHistoryBatchMatchesIndependentQueries` | Ordinary and exact-target queries all receive the same result as independent lookups, including distinct carriers and absent controls |
| `TestApprovalHistoryBatchRefusesMalformedTail` | Matches for every query before truncated/trailing data authorize no row; original first-match ordering is preserved |
| `TestLoadApprovalsRefusesFailedHistoryAfterMatches` | Real frames and a matched verdict followed by a failed Git exit refuse the whole load |
| `TestLoadApprovalsEmptyLedgerDoesNotReadGit` | Empty validated rows perform no history/source work |
| Existing `TestApprovalResolvesUniqueCarrierMovesAndRefusesSimultaneousDuplicates`, `TestApprovalRefusesUnknownAliasAndAmbiguousCarriers`, `TestApprovalCanonicalCarrierIncludesSemanticAndDeletedUnits` | Canonical carrier semantics survive |
| Existing `TestTargetApprovalExactOneLineBoundary`, `TestApproveGroupsMethodsOfOneNameInOneFile`, `TestOrdinaryApprovalRefusesChangedUnitContent` | Exact boundaries, receiver grouping and Content binding survive |
| Existing `TestTrailerLandedAnswersOnlyWhatGitCanAnswer`, ordinary/bound landed tests | Existing history/refusal semantics survive |
| Existing `TestRFCChangeHunksJudgesSelectedBlob`, `TestCreateSemanticRFCApprovalSelection`, `TestCreateSemanticRFCApprovalCollision` | Native selected-tree and ambiguity checks remain enforced |

Operation counters must observe/delegate to real reads in owned fixture
repositories. They cannot supply fabricated approval results. A bounded
operation-count regression is the performance acceptance test. Wall-clock
measurements supplement it under main's admitted native run.

## Observed verification checkpoint

All results below are retained in
`tmp/session/2026-11-14-210034/scratch/job-atomic-approval-cache-packages-091c3b9c.log`.

| Population | Observed result | Limit |
|------------|-----------------|-------|
| `internal/le/test/weakened` | Entire package PASS, 57.488s | Includes four new loader roots and existing Content/receiver/qualifier controls |
| `internal/le/commit` | Entire package PASS, 508.192s | Includes selected-hunk, semantic/exact selection, bound-pair and appended-publication controls |
| `internal/le/rfc` | Package FAIL after the 10-minute timeout, 601.176s | All four new RFC roots and named approval identity/boundary controls passed before the timeout; the package is not green |

The timeout stack is in `TestRFCTreeOverRealCorpus` at `corpus_test.go`
`TestRFCTreeOverRealCorpus`, through `nextAnswer`, corpus extraction and
`Deriver.InventoryUnder`, with the active regexp walk in `splitOffBoilerplate`.
Those producers are outside the four changed approval files. This result does
not establish that the corpus traversal is correct or completed. The timeout
remains a known red to disclose in the commit body. No assertion or timeout is
changed. The focused approval run in
`tmp/session/2026-11-14-210034/scratch/job-atomic-rfc-approval-regressions-661d17b3.log`
passed (14.062s). It supplies scoped completion evidence and does not turn
the full RFC package result green.

The two earlier loader roots also passed in
`job-atomic-approval-cache-green-bae24a0f.log`, after their original real TDD
count failure. The successful native dry-run result and its limits follow below.

After the three ordered guard corrections, the final scoped run in
`tmp/session/2026-11-14-210034/scratch/job-atomic-cache-final-guards-315e613f.log`
passed in all three packages: RFC 25.608s, weakening 45.425s and commit
140.861s. These are scoped populations, not replacement whole-package claims.
Main reports the cache formatter `c484a7ea` succeeded; its retained log has no
output. No further package rerun is owed solely for evidence freshness.

After removal of the two unused private wrappers, all eight new roots passed
in `tmp/session/2026-11-14-210034/scratch/job-atomic-cache-wrapper-cleanup-green-d7097296.log`:
the four RFC roots completed in 2.797s and four actual-loader roots in 4.009s.
Main reports formatter `809412ff` succeeded; its retained log is empty.
The count oracle preserves its valid/live, stale and spent row checks; it
measures bounded operations rather than a native elapsed-time speedup.

### Native preparation result — AC-8

Main observed the fresh named CLI from `57d55a56` finish the exact 64-path
native Create dry-run in direct session `61629` (PID `8024`) with exit 0:
session `ebf09f54`, all 64 selected paths, debt `nothing owed`, and normal
message/ScriptText output. `/usr/bin/time` reported real 181.12s, user 63.43s
and sys 77.34s. The operation-count oracle remains the evidence that repeated
per-row reads were removed; this single native run is not a speedup benchmark.

`internal/le/commit/prepare.go` `Create` reaches its dry-run return only after
the normal preparation, approval, selected-blob and source gates and script
composition. This dry-run did not publish a commit or execute the script's
per-attempt actual-parent/publication guards. Existing commit tests cover those
guards separately. The printed names `tmp/commit-msg-ebf09f54-a-3bf349.txt`
and `tmp/commit-ebf09f54-a-cd133a.sh` are prospective dry-run output, not written
artifacts and not commands to execute.

The earlier wrapped attempt `af4ff709` refused a malformed private protocol
patch after 3.18s, before those gates. Main repaired only its missing final
context line. That refusal is not the AC-8 success or a timing comparison.

### Final owned-scope lint — 2945761c

`tmp/session/2026-11-14-210034/scratch/job-atomic-cache-product-owned-lint-checkpoint-2945761c.log`
contains completed normal, Linux-integration, capability and compile-out
headers, with no No Plan refusal. Main's admitted selector covered five
packages; the capability and compile-out headers each identify their one
applicable package. The overall run exited 1. Its final related list names
exactly twenty files, all under `internal/component/bgp/reactor` or
`internal/le/interoplab/bgp`. Both the finding-path census and final related
list contain no file from either frozen cache package or the fixture package.
This completes owned-scope lint analysis without asserting an overall PASS.

## Deliverables Checklist

| Deliverable | Producing source / evidence | Readiness |
|-------------|-----------------------------|-----------|
| One resolver and history stream per ledger load | `ledger.go` `LoadApprovals`; `approvalunits.go` `ApprovalResolver`; `approve.go` `ApprovalsLanded` | Source reviewed; loader count regression PASS |
| Approval identity and history semantics preserved | `approvalSource.unitTexts`, `changedApprovalCarriers`, `readApprovalHistoryAll`, `approvalMessageLanded` | Independent source review clean; named approval controls PASS, focused RFC approval PASS in 661d17b3 |
| Scoped native commit guarantees preserved | `rfcchange.go` `rfcChangeProblems`, `renderApprovalPublicationCheck`; `prepare.go` `Create` | Commit package PASS; exact 64-path native Create dry-run exit 0. Publication guards were not executed by the dry-run |
| Durable documentation and closure evidence | `docs/contributing/rfc-implementation-guide.md`, request-lifetime paragraph; `docs/architecture/testing/test-health.md`, active producer anchor | Source comparison and main-observed direct file STE pass; owned-scope lint analyzed without cache findings; publication pending |
| Companion mutation reference remains accurate | `internal/le/test/weakened/approve_test.go`, `TestOrdinaryApprovalRefusesChangedUnitContent` comment | Symbol-only replacement reviewed; inputs and assertions unchanged |

## Security Review Checklist

| Concern | Source assessment | Required remaining evidence |
|---------|-------------------|-----------------------------|
| Stale authority across requests or revisions | Resolver state is allocated per load; `loadInventory` pins canonical HEAD | Pinned-HEAD and fresh-request controls PASS in 091c3b9c |
| Lost ambiguity or approximate declaration boundaries | Complete eligible membership and existing grouping/scope predicates remain | Named identity, malformed-source and selected-hunk controls PASS in 091c3b9c; focused RFC approval PASS in 661d17b3 |
| Partial history authorizes approval | Full frame, EOF and `command.Wait` checks precede returned matches | Malformed-tail and failed-Git-exit controls PASS in 091c3b9c |
| Command/path injection or authority bypass | Existing argument-vector Git reads and `gitenv.ReadOnlyQuery` remain; no writer, global cache, new CLI or publication bypass | Weakening and commit package suites PASS; focused RFC approval population PASS in 661d17b3 |

## Integration Checklist

| Surface | Applies? | Disposition |
|---------|----------|-------------|
| YANG schema/validation/custom validators | N-A | No command/config field changes |
| CLI commands, flags and grammar | N-A | Existing actions remain unchanged |
| Editor autocomplete | N-A | No new editable value |
| Functional/API carrier | Yes | Existing native approval/commit fixture tests exercise the real loader and selected views |
| Pipes | N-A | Existing answer shapes/renderers remain unchanged |
| Env variables | N-A | No cache toggle or new variable |
| Doctor/runtime dependency checks | N-A | No new external dependency, daemon runtime failure class or diagnostic code |
| Metrics | N-A | Test-owned operation counts only; no production telemetry |
| BGP family surface | N-A | Tooling only, no protocol change |

## Documentation Update Checklist

| # | Surface | Applies? | Disposition |
|---|---------|----------|-------------|
| 1 | Product feature/evidence level | No | Internal performance repair only |
| 2 | Config syntax | No | Unchanged |
| 3 | CLI reference | No | Grammar unchanged |
| 4 | API/RPC reference | No | No external interface change |
| 5 | Plugins | No | No plugin lifecycle/contract change |
| 6 | User guide | No | Approval semantics remain in the contributing guide |
| 7 | Wire format | No | No ledger/trailer/digest change |
| 8 | SDK/protocol | No | Internal Go tooling API only |
| 9 | RFC support | No | No product conformance credit |
| 10 | Test infrastructure | No | Existing fixture mechanisms, no new harness |
| 11 | Comparison | No | No product claim |
| 12 | Architecture | Yes | `rfc-implementation-guide.md` request lifetime; inspect committing selected-view guarantees |
| 13 | Route metadata | No | No routing changes |
| 14 | Metrics | No | No published metric |
| 15 | Registration/inventory | No | Reuse existing actions and carrier inventory |
| 16 | Source anchors | Yes | Listed `// Design:` guide travels with implementation; main checks scoped citations |
| 17 | Examples | No syntax change | Keep approval/commit examples unchanged; verify their stated guarantees |

Discovery remains the existing approval guide and the native approval/commit
actions. The shared loader is the single authority. New tests prevent the
repeated scan regression without a second inventory or cache service.

## Proposed Implementation Order

1. Main reviews preservation rules, view lifetime, minimal scope and test plan.
   Do not edit Go until main explicitly releases its coordinated hold.
2. Add the multi-unit real-reader regression and fresh-request controls first.
3. Extract the request resolver and reuse it in `LoadApprovals`. Keep identity
   and publication predicates unchanged.
4. Add multi-row history/tail-refusal tests, then share one validated history
   stream and apply existing per-message predicates for every query.
5. Main runs the three scoped package suites and reads named test results.
   Obtain independent review of both semantics and cost bounds.
6. Main repeats the authorized native workflow once and records the actual
   result. Do not hide a legitimate refusal behind the performance repair.

## Completion Gates

- [x] Main approves the design and releases the source hold.
- [x] AC-1 through AC-8 demonstrated without reduced approval checks; native evidence is preparation-only as stated above.
- [x] Scoped RFC approval, weakening and commit tests pass in the main checkout; the unrelated full RFC corpus timeout remains disclosed.
- [x] Actual loader count oracle confirms removal of repeated scan multipliers, and native preparation completes; no speedup benchmark claimed.
- [x] Independent generic/spec review is clean, both docs match their producers and main observed their direct file STE pass.
- [x] Final native lint analyzed the owned cache scope with no cache findings; overall reactor/interop red disclosed.
- [ ] Both native closure commits land and the spec claim is released.

## Owed Commands

The full weakening and commit package runs passed in 091c3b9c. The focused
RFC approval population passed in 661d17b3. Do not repeat those completed
populations to renew their timestamps.

Main subsequently released three changed-line guard corrections in
`approvalHistoryLanded`, `readApprovalHistory` and `approvalLanded`: errors
and missing-match conditions now have ordered single-fact guards. Independent
rereview found the same return tuples and refusal order. Main reports formatting succeeded in `c484a7ea`; the post-correction scoped
approval populations all passed in `315e613f`.

Main removed the two linter-confirmed unused private wrappers after these runs.
The existing mutation comment and test-health source anchor now name
`staleApprovalWith`. Independent reference census and narrow rereview are
clean. Main reports post-removal formatting succeeded in `809412ff`; the
eight new roots passed in `d7097296`. Earlier lint attempts under `47524449` produced no plan when appliance or
link-scope dependency inputs changed; they supplied no analysis or PASS.
Final `2945761c` completed the admitted five-package lint population and its
normal, Linux-integration and applicable capability/compile-out phases. Its
overall exit was 1; the final related list contains twenty reactor/interop
files and no cache-package or fixture path. Direct inspection of every
finding path likewise finds none under `internal/le/rfc` or
`internal/le/test/weakened`. This supplies the final owned-scope analysis
without claiming a green overall lint run. No identical narrower rerun is owed.

The final independent generic/spec review is clean, and native review record/check
succeeded over the final eight Go files plus both docs. Main observed the direct
file STE pass for both cache pages. Final native lint 2945761c analyzed the
cache scope with no cache findings; its overall foreign-finding red remains
explicitly disclosed.
The exact original reviewed native Create dry-run has passed with its actual
result and elapsed recorded above. No existing admission or publication check
may be bypassed.

The final native preparation/publication command uses the exact reviewed
selection owned by main. This spec supplies no replacement script or guessed
commit path. No whole-tree verification, export or source copy is required.

---

## Implementation Summary

### What Was Implemented

- `ApprovalResolver` reuses inventory, content and declaration summaries within one load.
- `ApprovalsLanded` returns independent results from one fully validated history stream.
- `LoadApprovals` shares those mechanisms without changing row admission or publication guards.

### Bugs Found/Fixed

- Repeated per-row reads are removed in source and the actual loader count regression passes. The exact native Create dry-run completes in 181.12s; this is preparation evidence, not a numerical speedup benchmark or publication result.
- Independent code review found no concrete implementation defect. The changed-line gate required three ordered single-fact guards. Later lint found two unused private wrappers, `approvalUnitTexts` and `staleApproval`; both were removed after confirming no callers. Active `approvalSource.unitTexts` and `staleApprovalWith` are unchanged. Independent narrow rereviews are clean; all eight post-removal roots pass in `d7097296`, and final lint 2945761c analyzed the owned scope with no cache findings, despite its overall reactor/interop red.

### Documentation Updates

- The request-lifetime paragraph in `docs/contributing/rfc-implementation-guide.md` anchors `ApprovalResolver`, `Resolve` and `readApprovalHistoryAll`.
- The `test-health.md` source anchor now names `staleApprovalWith`; the existing mutation comment in `approve_test.go` names that same active producer. Each is a symbol-only replacement.
- Existing identity, weakening and publication claims were compared with their unchanged producers. Main observed the direct file STE check on both cache pages pass within its combined three-page check (exit 0).

### Deviations from Plan

None in implementation scope. The planned behavioral controls use the actual test names below.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|-----------------------|----------------|--------|
| Mechanical source gates | Changed-line guards and two unused private wrappers were refused | Ordered guards preserve the same refusal tuples; wrappers have no callers after request-local routing | Main reported guard/lint findings; reviewer checked active producers and complete internal reference census | Guards split; unused wrappers removed; two companion symbols updated; main owns final checks |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Reuse source lookup within one request | Implemented; count regression PASS | `ApprovalResolver.Resolve`, `LoadApprovals` | Named semantic controls PASS; focused RFC approval PASS in 661d17b3 |
| Reuse fully validated landed-history I/O | Implemented; basic loader oracle PASS | `ApprovalsLanded`, `readApprovalHistoryAll` | Multi-query and malformed-tail controls PASS in 091c3b9c |
| Preserve native authority and complete native preparation | PASS for preparation; publication guards tested separately | Exact 64-path native Create dry-run exit 0; selected-content and actual-parent producers unchanged | No script or per-attempt publication guard executed by the dry-run |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | PASS for the loader oracle | `TestLoadApprovalsSharesOneRequest` in `job-atomic-approval-cache-green-bae24a0f.log` | One ls-tree, one ls-files, two base shows, one history inventory and one cat-file are the passing assertions |
| AC-2 | PASS for scoped approval controls | Loader row outcomes and `TestApprovalHistoryBatchMatchesIndependentQueries` PASS in 091c3b9c | One stream and per-query newest carriers preserved |
| AC-3 | PASS for scoped approval controls | Move, alias, qualifier, deleted-unit and receiver controls PASS in 091c3b9c | No identity assertion changed |
| AC-4 | PASS for scoped approval controls | `TestApprovalRequestPreservesMalformedNameBoundaries` and exact one-line controls PASS in 091c3b9c | Existing full-file fallback retained |
| AC-5 | Named controls PASS | Fresh-load, pinned-HEAD and selected-hunk controls PASS in 091c3b9c | New requests observe later inputs |
| AC-6 | PASS for scoped approval controls | New malformed-tail/failed-exit controls and native bound-pair tests PASS in 091c3b9c | RFC package timeout is recorded separately |
| AC-7 | PASS for preparation and scoped guard tests | Whole commit package PASS in 091c3b9c and scoped PASS in 315e613f; exact 64-path dry-run exit 0 | Actual-parent/publication guards remain in the generated script and did not execute in this dry-run |
| AC-8 | PASS for native preparation | Main observed direct session61629/PID8024 exit 0 for exact 64-path dry-run; real181.12s/user63.43s/sys77.34s | Debt nothing owed; normal message/ScriptText. Operation bounds from passing loader oracle; no publication, bypass or speedup benchmark |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestLoadApprovalsSharesOneRequest` | PASS observed after cleanup in d7097296 | `internal/le/test/weakened/approval_request_test.go` | Actual loader; real Git operation counts and useful/stale/spent rows |
| `TestLoadApprovalsStartsFresh` | PASS observed after cleanup in d7097296 | Same file | Edited content and new untracked duplicate |
| `TestLoadApprovalsRefusesFailedHistoryAfterMatches` | PASS observed after cleanup in d7097296 | Same file | Real matched frames followed by failed Git exit |
| `TestLoadApprovalsEmptyLedgerDoesNotReadGit` | PASS observed after cleanup in d7097296 | Same file | Empty ledger makes no Git query |
| `TestApprovalRequestPinsBaseAndStartsFresh` | PASS observed after cleanup in d7097296 | `internal/le/rfc/approval_request_cache_test.go` | Unread package retains pinned base; next request sees new HEAD |
| `TestApprovalRequestPreservesMalformedNameBoundaries` | PASS observed after cleanup in d7097296 | Same file | Missing, inexact and full-file cases |
| `TestApprovalHistoryBatchMatchesIndependentQueries` | PASS observed after cleanup in d7097296 | `internal/le/rfc/approval_history_batch_test.go` | Independent ordinary/targeted answers |
| `TestApprovalHistoryBatchRefusesMalformedTail` | PASS observed after cleanup in d7097296 | Same file | Every match precedes a corrupt tail; no authority survives |

The existing receiver, Content and native selected-hunk controls passed with
the complete weakening and commit packages. Named RFC identity/history
controls passed before the corpus timeout; the focused RFC approval population
then passed in 661d17b3.

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `internal/le/rfc/approvalunits.go` | Present; source reviewed | Request resolver and summaries |
| `internal/le/rfc/approve.go` | Present; source reviewed | Batch history entry point |
| `internal/le/rfc/approvalhistory.go` | Present; source reviewed | Per-query matches and complete framing |
| `internal/le/test/weakened/ledger.go` | Present; source reviewed | Shared loader request |
| `internal/le/rfc/approval_request_cache_test.go` | Present; source reviewed | New resolver controls |
| `internal/le/rfc/approval_history_batch_test.go` | Present; source reviewed | New history controls |
| `internal/le/test/weakened/approval_request_test.go` | Present; source reviewed | Actual-loader controls |
| `docs/contributing/rfc-implementation-guide.md` | Present; source compared | Request-lifetime paragraph and source anchors |
| `internal/le/test/weakened/approve_test.go` | Present; comment-only diff reviewed | Mutation comment names active `staleApprovalWith`; no assertion changed |
| `docs/architecture/testing/test-health.md` | Present; source-anchor diff reviewed | Anchor names active `staleApprovalWith` |

### Audit Summary

All planned source and test files are present. All eight new roots have
observed PASS lines. Weakening and commit packages passed. The complete RFC
package timed out; focused RFC approval completion passed in 661d17b3.
Final post-guard scoped populations passed in 315e613f, and all eight new
roots passed after wrapper removal in d7097296. Native Create dry-run then
passed with the limits above. Owned-scope lint analysis is complete with no cache findings; overall lint remains red on reactor/interop findings. Only native closure publication remains pending, and the review pin/check is clean. No scope or assertion is removed.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Reuse lookup work within one request | Actual-loader operation regression | `TestLoadApprovalsSharesOneRequest` PASS; prior red failed only repeated-read counts |
| Preserve approval decisions, identity and history authority | Independent source review plus runtime controls | Source review clean; all new roots and named approval controls PASS in 091c3b9c; focused RFC approval PASS in 661d17b3 |
| Unblock native evidence preparation without bypass | Native workflow plus actual-loader oracle | Exact 64-path native Create dry-run exit 0 in real181.12s with debt nothing owed and normal generated text; no publication, per-attempt parent check or speedup benchmark claimed |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| No scope omitted or transferred | All in-scope implementation and verification evidence is recorded; native removal has not landed, so this spec remains open | Not applicable |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/approval-lookup-request-cache-01a1db03-fbc1-7b32-8cbf-72b6284c110b.md`, native CLEAN record over all ten carried source/test/doc paths |
| `./le spec review check` | Exit 0: 8 code files, clean, hashes match |
| Rounds | 1 final generic/spec round, 0 BLOCKER and 0 ISSUE; earlier implementation source review and narrow correction rereviews remain recorded above |
| Reviewer lenses used | Logic/security; performance/completeness; source identity/history, changed-Go style and spec alignment |

Independent reviewer `atomic_docs_final_review` read the original seven Go paths and reported no concrete defect. After the three ordered guard corrections, the reviewer reread those producing functions and found unchanged refusal and indexing behavior. A later narrow rereview confirmed the two removed private wrappers have no internal callers and both companion reference changes name the active producer. The final population is eight Go files and two docs. The final review completed wiring, removed-behavior, allocation, boundary, authority and changed-Go style checks and mapped all eight ACs to the recorded source/tests/native preparation evidence. Native record and check now pin that final population. Final lint 2945761c separately analyzed the owned scope with no cache findings; its overall red names only reactor/interop files. This is native analyzer evidence, not a source-review substitute or a whole-tree pass.

The required light global prechecks were run: `repo check` exited 1 with 121 shared-tree inventory issues, and `test weakened check` exited 1 with 90 unexplained shared-tree findings (0 deleted, 90 weakened). The ze-close-required `doc check all` also exited 1, including 389 registry/catalog drift findings across shared documentation/wiki/generated surfaces. These are disclosed global reds, not claimed green. Cache-scope inspection found no new unwired symbol, removed assertion or unexplained weakening: its test changes are three new files and one mutation-comment symbol update. Existing shared-tree issues and foreign source/tests remain outside this selected closure.

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| none | None | Final scoped review found 0 BLOCKER and 0 ISSUE | Eight Go paths and two docs | No review-driven source fix required |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| Eight Go files and two docs listed above | Yes | Reviewer opened the changed source/test population and both doc diffs; native record pins all ten paths and check confirms all eight code hashes |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Loader read multiplier removed | Actual count-oracle root PASS in `job-atomic-approval-cache-green-bae24a0f.log` |
| AC-2 | Independent row outcomes retained | Live/stale/spent loader assertions and targeted batch control PASS in 091c3b9c |
| AC-3 | Identity and ambiguity preserved | Named identity/receiver controls PASS; focused RFC approval PASS in 661d17b3 |
| AC-4 | Exact declaration scope preserved | New malformed and existing exact-boundary controls PASS in 091c3b9c |
| AC-5 | New loads observe edits and duplicate carriers | Fresh-load, pinned-HEAD and selected-hunk controls PASS in 091c3b9c |
| AC-6 | Partial history cannot authorize a row | Malformed-tail/failed-exit controls PASS in 091c3b9c; focused RFC approval PASS in 661d17b3 |
| AC-7 | Selected content and actual parent stay authoritative | Commit package PASS and exact native preparation dry-run exit 0; per-attempt parent/publication guards tested separately, not executed by this dry-run |
| AC-8 | Original native preparation completes its checks | Direct session61629/PID8024 exit 0 for exact 64 paths, real181.12s; count regression PASS; publication and benchmark claims excluded |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Actual `LoadApprovals` entry point and normal `Create` preparation | N-A: tooling fixture uses real Git directly | Actual-loader/native commit controls PASS; exact 64-path native Create dry-run exit 0 through normal preparation gates |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | Confirmed for redundant work; no dominance or speedup claim | Actual before/after loader count oracle confirms repeated work and reuse; native dry-run completed in181.12s. No causal timing breakdown attributes the old delay or measures comparable speedup |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Request-local reuse, pinned base and full history validation | `ApprovalResolver`, `LoadApprovals`, `readApprovalHistoryAll` | Source comparison complete; main-observed direct file STE PASS for both cache pages |
| Existing approval identity, selected-content and publication semantics | Existing anchored paragraphs in contributing/testing and architecture test-health docs; underlying predicates retained | Source comparison complete; main-observed cache-page STE PASS. Full doc check exited 1 on shared-tree drift, including 389 registry/catalog findings; no full-doc green claimed |
| Product/runtime/CLI/RFC support surfaces do not change | Four production tooling files; no product, registry, CLI, ledger or trailer schema change | Source scope checked; no new conformance claim or generator owed |

Main owns both pending native closure commits; final owned lint analysis is complete with the qualifications above. No spec deletion, commit or push has occurred in this closure audit. The successful native workflow result is preparation-only; the final review artifact is a source/content pin, not a lint or publication result. Commit A must include all eight Go files, both docs and this spec; commit B removes only this spec with `no-news` because this repair changes development tooling only.


## Prepared native closure selection

Main-only preparation is at
`tmp/session/2026-11-14-210034/scratch/atomic-proof/cache-closure-create.sh`,
with the implementation and removal body files beside it. The recipe contains
the two native Create calls and all eleven implementation/spec paths. It takes
Create A's actual `script=` answer for Create B's `append script` argument,
so pre-existing scripts cannot make the append ambiguous. It never executes
the generated native script and carries no push or gate override.

Final admitted lint 2945761c has now produced the required owned-scope
analysis without cache findings. Its overall red on twenty reactor/interop
files stays disclosed; earlier No Plan attempts supply no such evidence.
No heavy verification action remains for this frozen cache selection. No
passed test, formatter, direct cache-page STE, source-review population or
identical lint scope needs repetition. Main may prepare the two native blocks;
Create and its publication script retain their own exact-content, approval
and review checks.

The native child claim was verified read-only under `ZE_AGENT_ID=atomic_record_queue`
and parent session `01a1db03-fbc1-7b32-8cbf-72b6284c110b`; it names this cache spec.
The recipe exports that child context locally so both messages receive the
cache `Spec-work` trailer and the primary atomic claim is untouched.
After both native commits land, main releases only that child context with
`ZE_AGENT_ID=atomic_record_queue CLAUDE_CODE_SESSION_ID=01a1db03-fbc1-7b32-8cbf-72b6284c110b ./le --name s-01a1db03 spec release`,
checks `./le spec citation`, and removes the three private
preparation files once the native commit messages and landed SHAs supersede
them. The spec remains in-progress until actual native removal. Its completed
evidence can now be preserved by commit A; closure itself is not claimed
until commit B lands and the claim is released.
