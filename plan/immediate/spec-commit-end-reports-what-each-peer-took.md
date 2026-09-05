# Spec: commit end reports what each peer took

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | cli |
| Depends | - |
| Phase | DESIGN |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`request commit end <name>` and `request commit eor <name>` answer an operator
with a count of the work the commit carried, and that count is computed before
any of the work is attempted.

`(*reactorAPIAdapter).SendRoutes` (`internal/component/bgp/reactor/reactor_api_batch.go`)
sets `totalResult.RoutesAnnounced = len(routes)` and
`totalResult.RoutesWithdrawn = len(withdrawals)` BEFORE the peer loop. Inside the
loop, three outcomes leave those numbers untouched:

1. A peer with no send context (the session is not established) is skipped with a
   bare `continue`.
2. `(*CommitService).Commit` returns an error and the error is discarded with a
   bare `continue`. The partial `stats` it returns beside the error is discarded
   with it, so the UPDATEs that DID leave for that peer are not counted either.
3. `sendWithdrawals` refuses a family whose NLRIs do not fit the build buffer and
   continues to the next family.

The refusals themselves are correct and fail closed. Nothing malformed reaches
the wire and no peer receives a partial UPDATE. What is wrong is the report: the
operator is told that every queued route and withdrawal was carried, for every
matched peer, whatever happened.

`eor_sent` in the same payload has the same shape: it echoes the operator's
REQUEST rather than what left, so a failed End-of-RIB send is reported as sent.

The goal is an answer that states, per peer, what that peer took, and a status
that is not `done` when a matched peer took nothing.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - the page the commit plugin declares as its design document, and the page that publishes the batching workflow
  → Decision: the `Group Commands (Batching)` block publishes `group start` and `group end`, and no handler answers either. The commit answer shape is not documented at all, so this spec adds it.
  → Constraint: the page is the declared owner of `internal/component/bgp/plugins/cmd/commit/commit.go`, so an edit to that file names this page.
- [ ] `docs/architecture/core-design.md` - the page declared by `reactor_api_batch.go` and by `internal/component/bgp/types/types.go`
  → Constraint: it describes the announce and withdraw rails and the wire abstractions. This change alters no rail and no wire form, so the page is named and unedited.
- [ ] `ai/rules/cli.md` - the command answer contract
  → Constraint: the payload is structured data that `| json`, `| yaml` and `| table` each render, so per-peer state belongs in a field on a row rather than in a rendered sentence.
  → Constraint: every JSON key is lowercase kebab-case. `routes_announced`, `routes_withdrawn`, `updates_sent` and `eor_sent` in the current payload are snake_case, so the keys this spec touches are respelled.
- [ ] `ai/rules/principles.md` - the guard directive
  → Decision: a caller cannot tell a real count from one nobody checked, so the count is DERIVED from what each peer took rather than assigned from the queue length.

### RFC Summaries (Scope: protocol)
Not applicable. This spec changes no wire behavior, no message, and no state
machine. The UPDATEs Ze sends and the conditions under which it refuses to send
one are unchanged.

**Key insights:** (minimal context to resume after compaction)
- The count is assigned before the loop; three branches inside the loop leave it wrong.
- `(*Transaction).QueueAnnounce` has NO non-test caller, so the announce half of a named commit is dead today and the reachable manifestation is the withdrawal half.
- `answerValue` (`pkg/plugin/sdk/sdk_engine.go`) drops `Data` when a command answers with an error status, so the refusal must also be stated in the error sentence.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` - `SendRoutes` assigns both counts before the peer loop and discards the commit error, the partial stats, and the skip of a non-established peer. `sendWithdrawals` returns only an UPDATE count and reports a refused family to the log alone.
- [ ] `internal/component/bgp/rib/commit.go` - `(*CommitService).Commit` returns stats beside its error, and those stats are PARTIAL: grouped sends increment `UpdatesSent` and `RoutesAnnounced` per group, so a refusal in the third group leaves two groups already on the wire. `buildMPReachNLRI` calls `ValidateNextHops` and writes a Warn record whose comment says the caller above discards the error. `enforcePathsLimit` drops routes over a negotiated per-prefix limit, so even a fully successful commit can carry fewer routes than were queued.
- [ ] `internal/component/bgp/plugins/cmd/commit/commit.go` - `handleNamedCommitEnd` calls `SendRoutes` and renders `routes_announced`, `routes_withdrawn`, `updates_sent`, `families` and `eor_sent`. It answers `StatusDone` whenever `SendRoutes` returns no error.
- [ ] `internal/component/bgp/types/types.go` - `TransactionResult` holds five scalars and a family list, and carries no per-peer shape.
- [ ] `internal/component/bgp/types/reactor.go` - the `SendRoutes` signature on the reactor interface, which the commit plugin holds.
- [ ] `internal/component/bgp/transaction/commit_manager.go` - `(*Transaction).Routes` returns what `QueueAnnounce` stored, and `QueueAnnounce` has no non-test caller. `QueueWithdraw` is called by `handleNamedCommitWithdraw`, so a named commit can hold withdrawals and cannot hold announcements.
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `getMatchingPeersSel` matches CONFIGURED peers, whatever their session state, so a peer that never established is in the returned set.
- [ ] `internal/component/bgp/reactor/peer.go` - `sendContext` reads the pointer that `setEncodingContexts` stores at Established, so it is nil for every other state.
- [ ] `internal/component/bgp/plugins/cmd/peer/fields.go` - the field spellings every multi-peer answer already uses: `peers` for the rows, `peer` and `address` for identity, `name`, `state`, `updates-sent`, `eor-sent`.
- [ ] `internal/component/plugin/server/quiesce.go` - `quiesceAll` answers `StatusError` naming the failed subsystems and still fills `Data` with the full participant list. This is the precedent for an error envelope that keeps its payload.
- [ ] `pkg/plugin/sdk/sdk_engine.go` - `answerValue` collapses the answer into a document and then answers `StatusError` with a nil document when the terminator carries a message, so the document is dropped for a plugin caller on the error path.
- [ ] `internal/component/bgp/plugins/cmd/commit/yang/ze-bgp-cmd-commit-api.yang` - the `ze:help` on the `commit` RPC states what `end` and `eor` do and says nothing about what the answer reports.
- [ ] `test/plugin/cli-grammar-action-first.ci` and `internal/test/fixture/plugin_fixture_04_cli.go` - the shape a functional test uses to drive `request commit ...` through the real dispatcher and read the answer as a map.

**Behavior to preserve:** (unless the user explicitly said to change it)
- The wire is unchanged. A refused commit still sends nothing for that peer, a refused family still sends nothing for that family, and no partial UPDATE is emitted.
- `no peers match selector` stays an error, and an empty commit still answers `commit empty, nothing sent`.
- The canonical and deprecated commit grammars both keep working, and `test/plugin/cli-grammar-action-first.ci` keeps passing.
- Peer identity in the answer keeps the spellings the other multi-peer answers use.

**Behavior to change:** (only what the user asked for)
- Every count in the answer is derived from what each peer took, and no count is assigned from the queue length.
- The answer carries one row per matched peer.
- The response status is `done` only when every matched peer took every queued item.
- `eor-sent` reports the End-of-RIB markers that left, not the operator's request.
- The four snake_case keys are respelled kebab-case.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator types `request commit end <name>` or `request commit eor <name>` in the CLI over SSH, or an attached plugin dispatches the same command text over the plugin hub.
- Format at entry: command text, tokenized by the dispatcher and delivered to the RPC `ze-bgp:commit` with the action and the name as arguments.

### Transformation Path
1. Dispatch: `handleCommit` reads the action keyword and routes to `handleNamedCommitEnd` (`internal/component/bgp/plugins/cmd/commit/commit.go`).
2. Transaction: `(*CommitManager).End` removes the named transaction and answers it; `Routes()` and `Withdrawals()` give what it held.
3. Reactor: `(*reactorAPIAdapter).SendRoutes` matches peers with `getMatchingPeersSel` and loops over them.
4. Per peer: `sendContext` decides whether the peer can be encoded for; `rib.NewCommitService(...).Commit` sends the announcements; `sendWithdrawals` sends the withdrawals; `message.BuildEOR` plus `peer.SendUpdate` sends the End-of-RIB.
5. Result: `bgptypes.TransactionResult` returns to the handler, which builds the response payload.
6. Render: the CLI runs the payload through `ApplyPipes`, so `| json`, `| yaml` and `| table` each render the same fields.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin ↔ Engine | RPC `ze-bgp:commit`, JSON response envelope with a `data` payload | No |
| Command plugin ↔ Reactor | `bgptypes.Reactor.SendRoutes`, Go call over the interface in `internal/component/bgp/types/reactor.go` | No |
| Reactor ↔ RIB | `rib.NewCommitService(peer, ctx, true).Commit`, which returns partial stats beside its error | No |
| Reactor ↔ Peer | `peer.SendUpdate`, one UPDATE at a time | No |

### Integration Points
- `bgptypes.TransactionResult` - the one type crossing the reactor boundary for this command. Per-peer rows are added to it, so the reactor states the outcome and the plugin renders it.
- `internal/component/bgp/plugins/cmd/peer/fields.go` - the peer row spellings this answer reuses rather than inventing.
- `ApplyPipes` (`internal/component/command/pipe.go`) - already applied to this command's answer; a list of maps under `peers` is what `| table` renders as rows.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | The outcome is produced where it is known (the reactor loop) and rendered where rendering belongs (the command handler). Nothing reads the reactor's internals from the plugin. |
| No unintended coupling (components stay isolated) | No | `TransactionResult` already crosses this boundary; the change adds fields to it and introduces no new import in either direction. |
| No duplicated functionality (extends existing, does not recreate) | No | The per-peer row reuses the field spellings in `cmd/peer/fields.go` and the error-with-payload shape in `quiesce.go`. |
| Zero-copy preserved where applicable (refs, not copies) | No | The rows carry counts and short strings only, and are built once per command, off any wire path. |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | No new command and no new registry entry. The change is inside one existing handler, one existing reactor method, and the result type they already share. |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `(*Transaction).QueueAnnounce` has no non-test caller, so a named commit carries withdrawals only and the announced count is 0 on every shipped path today | `gopls references` on the `QueueAnnounce` declaration in `internal/component/bgp/transaction/commit_manager.go`, which answers nine sites, all in `commit_manager_test.go` | The announced half is reachable too, and the functional test must drive it as well as the withdrawal half | The same query at implementation time, plus the functional test which drives the withdrawal half | unvalidated |
| A-2 | A `plugin.Response` can carry `Data` beside `Status` error | `quiesceAll` (`internal/component/plugin/server/quiesce.go`) does exactly this, and `Response` (`internal/component/plugin/types.go`) declares both fields | The refusal detail must live entirely in the error sentence | `TestCommitEndAnswersErrorAndKeepsThePeerRows` reads both fields off the returned response | unvalidated |
| A-3 | A peer matched by the selector but not established has a nil send context | `sendContext` reads `p.sendCtx`, which `setEncodingContexts` fills at Established (`internal/component/bgp/reactor/peer.go`) | The not-established row cannot be distinguished from a refusal, and the state field needs another source | `TestSendRoutesNamesTheNotEstablishedPeer` builds a peer with no send context | unvalidated |
| A-4 | No shipped reader outside `handleNamedCommitEnd` consumes `TransactionResult.RoutesAnnounced` | `grep RoutesAnnounced` over the tree: one non-test caller in `internal/component/bgp/plugins/cmd/commit/commit.go`, four mock reactors in plugin tests, and the unrelated `rib.CommitServiceStats` field of the same name | A rename breaks a reader nobody enumerated | The same grep at implementation time, and the compiler | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The error status makes `request commit end` fail whenever any matched peer is down, and an operator learns to ignore it | A `.ci` or an operator report where a routine down peer fails a commit | The error sentence names the peer and the reason, so the failure is actionable rather than generic. The remedy is a narrower selector, which the answer's peer rows make obvious |
| R-2 | `answerValue` drops `Data` on the error path, so a plugin caller sees the sentence alone | A fixture reading the payload after a refusal gets nil | The error sentence names every refused peer and its reason class, so the sentence alone carries the facts. AC-3 asserts the sentence, not the payload |
| R-3 | `reactor_api_batch.go` is being edited concurrently for the RFC 8669 Prefix-SID egress boundary, and `SendRoutes` may gain a parameter | A merge conflict in `SendRoutes` | Read the landed signature before implementing; this change touches the body's accounting, not its parameters |
| R-4 | A per-peer row for a `peer *` selector on a large router makes the answer long | A table with hundreds of rows | The rows are the answer, and `| table`, `| json` and the pipe filters are how an operator narrows them. No truncation, which would reintroduce a silently incomplete answer |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing on the wire. The failure mode is a wrong or missing report for `request commit end` and `request commit eor`, and a status that fails a script that used to pass. |
| How is it reverted? | Single commit revert. No config migration, no persisted state, no peer-visible change. |
| Who else touches this path? | A concurrent session is editing `reactor_api_batch.go` for the RFC 8669 Prefix-SID egress boundary. `plan/spec-rib-package-dead-surface.md` names `SendRoutes` in its wiring table. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Operator types `request commit end <name>` with a selector matching an established peer and a peer that never established | → | `handleNamedCommitEnd` then `SendRoutes` then the per-peer rows | `test/plugin/commit-end-per-peer-report.ci` driving fixture `plugin/commit-end-per-peer-report` |
| Plugin dispatches `request commit eor <name>` over the hub | → | the End-of-RIB branch of `SendRoutes` and the `eor-sent` field of each row | `TestCommitEORReportsTheMarkersThatLeft` |
| Reactor accounting called directly with one established and one non-established peer | → | `(*reactorAPIAdapter).SendRoutes` | `TestSendRoutesNamesTheNotEstablishedPeer` |
| Commit refused mid-way by the next-hop guard | → | the `(*CommitService).Commit` error path read by `SendRoutes` | `TestSendRoutesKeepsThePartialUpdatesOfARefusedCommit` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `request commit end <name>` where the selector matches one established peer and one peer whose session never established, with one prefix queued for withdrawal | The answer carries one row per matched peer. The established peer's row states it took the withdrawal; the other peer's row states `not-established` and a withdrawn count of zero. |
| AC-2 | The same command | The queue size is reported under its own name and is never presented as the delivered count. The delivered totals are the sum of the per-peer rows. |
| AC-3 | Any matched peer takes none of the queued work | The response status is `error`, and the error sentence names each such peer and its reason. The response payload still carries every peer row. |
| AC-4 | Every matched peer takes every queued route and withdrawal | The response status is `done`, and no peer row carries a reason. |
| AC-5 | A peer whose commit is refused after some UPDATEs have already left, because the next hop of a later attribute group has no wire form | That peer's row reports the UPDATEs that left and the routes they carried, states the refusal reason, and the top-level updates count includes them. |
| AC-6 | `request commit eor <name>` where one matched peer accepts the End-of-RIB and one peer's send fails | Each row reports the End-of-RIB markers that left for that peer. A peer whose send failed reports none. The top level states that an End-of-RIB was requested, not that one was sent. |
| AC-7 | The answer of `request commit end`, `request commit eor` and `request commit rollback` | Every key is lowercase kebab-case. `| json`, `| yaml` and `| table` each render the answer, and `| table` prints one row per peer. |
| AC-8 | An operator reads `docs/guide/route-injection.md` and `docs/architecture/api/commands.md` | Neither page shows a route announcement joining a named commit, because no producer queues one. Both state what the commit workflow carries today and what its answer reports. |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Queues a withdrawal into a named commit and flushes it while one of two matched peers is down | CLI, `ze-bgp:commit`, `handleNamedCommitEnd`, `SendRoutes`, `sendWithdrawals`, the wire, and the per-peer rows back | `test/plugin/commit-end-per-peer-report.ci` |
| 2 | Flushes a named commit whose matched peers are all down | the same path, with no peer holding a send context | `TestCommitEndAnswersErrorAndKeepsThePeerRows` |
| 3 | Renders the answer as a table to see which peers took the batch | answer payload through `ApplyPipes` and the `table` operator | `TestCommitEndAnswerRendersAsRows` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSendRoutesNamesTheNotEstablishedPeer` | `internal/component/bgp/reactor/reactor_api_batch_test.go` | AC-1, AC-2: a peer with no send context appears as its own row with zero counts, and the totals do not include it | |
| `TestSendRoutesKeepsThePartialUpdatesOfARefusedCommit` | `internal/component/bgp/reactor/reactor_api_batch_test.go` | AC-5: the partial stats returned beside the commit error are counted, and the row carries the reason | |
| `TestSendRoutesCountsOnlyTheWithdrawalsThatLeft` | `internal/component/bgp/reactor/reactor_api_batch_test.go` | AC-2: a family refused by `sendWithdrawals` is not counted as withdrawn, and its reason reaches the row | |
| `TestSendRoutesReportsTheEORThatLeft` | `internal/component/bgp/reactor/reactor_api_batch_test.go` | AC-6: a failed End-of-RIB send is not reported as sent | |
| `TestCommitEndAnswersErrorAndKeepsThePeerRows` | `internal/component/bgp/plugins/cmd/commit/commit_test.go` | AC-3: status, error sentence and payload on a refusal | |
| `TestCommitEndAnswersDoneWhenEveryPeerTookEverything` | `internal/component/bgp/plugins/cmd/commit/commit_test.go` | AC-4 | |
| `TestCommitEORReportsTheMarkersThatLeft` | `internal/component/bgp/plugins/cmd/commit/commit_test.go` | AC-6 through the handler | |
| `TestCommitAnswerKeysAreKebabCase` | `internal/component/bgp/plugins/cmd/commit/schema_test.go` | AC-7: every key of every commit answer | |
| `TestCommitEndAnswerRendersAsRows` | `internal/component/bgp/plugins/cmd/commit/commit_test.go` | AC-7: the payload satisfies `ResponseData` and the peer rows render as table rows | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| matched peers | 0 to the configured peer count | the configured peer count | 0 answers `no peers match selector`, which is existing behavior | N/A |
| queued withdrawals | 0 to unbounded | the queued count | 0 answers `commit empty, nothing sent`, which is existing behavior | N/A |
| withdrawals delivered per peer | 0 to the queued count | the queued count | N/A | a value above the queued count is a defect the unit test asserts against |
| End-of-RIB markers per peer | 0 to the family count of the commit | the family count | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `commit-end-per-peer-report` | `test/plugin/commit-end-per-peer-report.ci` | Two configured peers, one establishing against `ze-peer` and one pointing at a port with no listener. The fixture starts a named commit, queues one withdrawal, ends it, and reads the answer: one row per peer, the established peer carrying the withdrawal, the other stating `not-established`, and the status an error naming it. `ze-peer` asserts the withdrawal UPDATE arrives on the established session, so the wire stays correct while the report changes. | |

### Interop Tests (Scope: protocol)
Not applicable, with the reason `ai/rules/interop-and-goal-validation.md` admits:
this change is wire-invisible. It alters no message, no attribute, no timer and
no state transition, and the functional test above asserts that the same
withdrawal UPDATE still reaches the peer. The existing interop scenarios cover
the UPDATEs this path emits and are unaffected.

## Files to Modify
- `internal/component/bgp/types/types.go` - `TransactionResult` gains the per-peer rows and the queued-versus-delivered split; the snake-case-rendered scalars are renamed to what they mean.
- `internal/component/bgp/types/reactor.go` - the `SendRoutes` doc comment states what the result reports, so the next caller does not have to read the loop.
- `internal/component/bgp/reactor/reactor_api_batch.go` - `SendRoutes` derives every count from the loop, keeps the partial stats of a refused commit, and records a reason per peer. `sendWithdrawals` answers what it withdrew and what it refused, instead of an UPDATE count alone.
- `internal/component/bgp/plugins/cmd/commit/commit.go` - the answer payload of `end` and `eor`: kebab-case keys, per-peer rows, and the status rule.
- `internal/component/bgp/plugins/cmd/commit/yang/ze-bgp-cmd-commit-api.yang` - the `ze:help` states that the answer names each matched peer and that a peer which took nothing fails the command.
- `internal/component/bgp/plugins/cmd/commit/mock_reactor_test.go`, `internal/component/bgp/plugins/cmd/raw/mock_reactor_test.go`, `internal/component/bgp/plugins/cmd/update/mock_reactor_test.go`, `internal/component/bgp/plugins/cmd/peer/mock_reactor_test.go` - the four mocks that build a `TransactionResult` from the queue length, which is the defect being removed.
- `internal/test/fixture/plugin_fixture_04_cli.go` - registers and implements the functional-test driver.
- `docs/architecture/api/commands.md` - the answer shape of `request commit end` and `request commit eor`, and the `Group Commands (Batching)` block, which publishes two commands no handler answers.
- `docs/guide/route-injection.md` - the `Commit Workflow` block, which shows `send bgp ... update` between `commit start` and `commit end` and says the routes are sent together. No producer queues an announcement into a transaction, so the block describes a workflow that does not run.

## Files to Create
- `test/plugin/commit-end-per-peer-report.ci` - the functional test above.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new RPC and no new config leaf. The `commit` RPC already exists. |
| YANG validation constraints | N-A | No new leaf takes a value. |
| YANG custom validators | N-A | No new leaf takes a value. |
| CLI commands/flags | No | The grammar of `request commit` is unchanged; only its answer changes. |
| CLI grammar (keyword before value) | N-A | No token is added to the grammar. |
| Editor autocomplete | N-A | No new leaf and no new completion candidate. |
| Functional test for new RPC/API | Yes | `test/plugin/commit-end-per-peer-report.ci` |
| Pipe completeness | Yes | The answer already routes through `ApplyPipes`; the peer rows are what `| table` renders. |
| Env var registration | N-A | No environment leaf. |
| Doctor check for runtime dependencies | N-A | No new file path, socket, service, module, port or binary. |
| Prometheus counters/metrics | No | The counts are per command invocation, not cumulative state. A commit counter is a separate question and is not created here. |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute is added. |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The commands already exist; their answer becomes truthful. |
| 2 | Config syntax changed? | No | No config leaf is touched. |
| 3 | CLI command added/changed? | No | `docs/guide/command-reference.md` lists the commands and not their answers; the rows stay correct. |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` gains the answer shape and loses the `group start` and `group end` block, which no handler serves. |
| 5 | Plugin added/changed? | No | No plugin is added or removed. |
| 6 | Has a user guide page? | Yes | `docs/guide/route-injection.md`, `Commit Workflow`. |
| 7 | Wire format changed? | No | No message, attribute or encoding changes. |
| 8 | Plugin SDK/protocol changed? | No | The envelope is unchanged; only this command's payload fields change. |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC requirement is implemented, changed or newly proven. |
| 10 | Test infrastructure changed? | No | One `.ci` and one fixture are added through the existing mechanisms. |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` makes no claim about this answer. |
| 12 | Internal architecture changed? | No | `docs/architecture/core-design.md` is the design document declared by `reactor_api_batch.go` and `internal/component/bgp/types/types.go`. It is NAMED here and unedited: it describes the announce and withdraw rails and the wire abstractions, and this change alters neither. |
| 13 | Route metadata keys added/changed? | No | No metadata key. |
| 14 | Prometheus counters added/changed? | No | No counter. |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registers or unregisters. |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation time by `./le spec citation anchors spec plan/immediate/spec-commit-end-reports-what-each-peer-took.md`. The two declared owners are named in rows 4 and 12. `docs/guide/route-injection.md` carries a source anchor naming the commit command directory and is edited in row 6. |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The `Commit Workflow` example in `docs/guide/route-injection.md` and the `Group Commands (Batching)` example in `docs/architecture/api/commands.md` both show a batching route no code serves. |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the answer reaches an operator before changing what it says
   - Tests: `test/plugin/commit-end-per-peer-report.ci`, fixture `plugin/commit-end-per-peer-report`
   - Files: `test/plugin/commit-end-per-peer-report.ci`, `internal/test/fixture/plugin_fixture_04_cli.go`
   - Verify: the fixture drives `request commit start`, `request commit withdraw` and `request commit end` through the real dispatcher and reads the answer. The test fails because the answer carries no peer rows and reports a withdrawal for a peer that never established
2. **Phase: the result type** -- give the reactor somewhere to state the outcome
   - Tests: the four reactor unit tests, red against the current assignment
   - Files: `internal/component/bgp/types/types.go`, `internal/component/bgp/types/reactor.go`
   - Verify: the type compiles, the four mocks are updated, and every existing caller still builds
3. **Phase: reactor accounting** -- derive every count from the loop
   - Tests: `TestSendRoutesNamesTheNotEstablishedPeer`, `TestSendRoutesKeepsThePartialUpdatesOfARefusedCommit`, `TestSendRoutesCountsOnlyTheWithdrawalsThatLeft`, `TestSendRoutesReportsTheEORThatLeft`
   - Files: `internal/component/bgp/reactor/reactor_api_batch.go`
   - Verify: each test red before the edit and green after, with the wire assertions in the existing reactor tests unchanged
4. **Phase: the answer** -- render the outcome and decide the status
   - Tests: the five commit handler tests
   - Files: `internal/component/bgp/plugins/cmd/commit/commit.go`, `internal/component/bgp/plugins/cmd/commit/yang/ze-bgp-cmd-commit-api.yang`
   - Verify: the functional test from phase 1 turns green, and `test/plugin/cli-grammar-action-first.ci` still passes
5. **Phase: the pages** -- correct what the docs publish about this workflow
   - Tests: `./le docvalid`, and the citation anchor audit for this spec
   - Files: `docs/architecture/api/commands.md`, `docs/guide/route-injection.md`
   - Verify: no page shows an announcement joining a named commit, and both name what the answer reports

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named file and symbol, and no count in the answer is assigned before the work it counts |
| Feature completeness | The three user stories each have a passing test, and the functional test reads the answer through the real dispatcher |
| Correctness | A peer row's counts are the sum of what that peer's sends returned, never the queue length. The partial stats of a refused commit are added, not discarded |
| Naming | Every answer key is kebab-case, and each peer field reuses the spelling in `cmd/peer/fields.go` rather than a new synonym |
| Data flow | The outcome is decided in the reactor, where it is known, and rendered in the handler. The handler does not re-derive a count |
| Rule: `ai/rules/principles.md` | No branch answers a count nobody checked. Read the finished `SendRoutes` for an assignment from a queue length outside the queued-size fields |
| Rule: `ai/rules/cli.md` | The payload stays structured data, and the peer rows render under `| json`, `| yaml` and `| table` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `SendRoutes` assigns no delivered count before the loop | `grep -n "RoutesAnnounced = len\|RoutesWithdrawn = len" internal/component/bgp/reactor/reactor_api_batch.go` answers nothing |
| The commit error is read | `grep -n "cs.Commit" -A 4 internal/component/bgp/reactor/reactor_api_batch.go` shows the error reaching a peer row |
| Every answer key is kebab-case | `grep -n "\"[a-z]*_[a-z_]*\":" internal/component/bgp/plugins/cmd/commit/commit.go` answers nothing |
| The functional test exists and runs | `./le integration scenario commit-end-per-peer-report` |
| The docs no longer publish a batching route with no producer | `grep -n "commit start" docs/guide/route-injection.md docs/architecture/api/commands.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The commit name and the peer selector are already validated upstream; the new fields are counts and reason strings the daemon produces |
| Error leakage | A peer row's reason names the peer and the refusal class. It must not carry an attribute dump, a next hop the operator did not supply, or a raw internal error chain |
| Authorization | The `send [ update ]` permission gate in `getMatchingPeersSel` runs before any peer is looped over, and this change adds no path around it |

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
- The defect has one shape and four instances in one function: a count assigned before the work, a skipped peer, a discarded error, and a discarded partial. Fixing the assignment alone would leave three.
- The answer's identity problem is that one scalar cannot describe several peers. Every honest shape here is a list, and the list is what `| table` was built to render.
- A refusal that only reaches the log is invisible to the operator who caused it. `buildMPReachNLRI` already writes a Warn record and its comment says why: the caller discards the error. When the caller stops discarding it, the record duplicates an answer the operator now reads, so the comment states which surface owns the message.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One row per matched peer, and every total derived by summing the rows | A refused counter beside the announced one; a single failure naming the peers in prose | A counter says how many refused and never which, so the operator's next action, which is to narrow the selector or bring a peer up, is not in the answer. A prose sentence is finished text, which `ai/rules/cli.md` refuses as a payload |
| Status `done` only when every matched peer took every queued item; otherwise `error`, carrying both the sentence and the rows | Always `done` with the detail in the payload; `error` only when NO peer took anything | This rail does not queue for a peer that is not established, unlike `AnnounceNLRIBatch`, so undelivered means dropped. A `done` envelope over dropped work is the defect wearing a new shape, and a script reading the exit code learns nothing from it. The cost is accepted: a routine down peer fails the command, and the sentence says which peer and why |
| The error sentence names each refused peer and its reason, duplicating the rows | Rely on the payload alone | `answerValue` (`pkg/plugin/sdk/sdk_engine.go`) drops `Data` when a command answers with an error status, so a plugin caller over the socket receives the sentence and nothing else |
| Rename the four snake_case keys in the same change | Leave them and add kebab-case siblings | `ai/rules/cli.md` requires kebab-case, Ze is unreleased so a spelling is replaced rather than aliased, and two spellings of one count is the disagreement this whole spec exists to remove |
| Report the drop for a non-established peer; do not queue for it | Queue the routes as `AnnounceNLRIBatch` does | Queueing changes what the daemon sends after a session establishes, which is a behavior change well outside a reporting fix. The asymmetry is named in Known Limitations so the next reader meets it |

## Known Limitations
- The announce half of a named commit has no producer: `(*Transaction).QueueAnnounce` is called by nothing outside its own tests, so `request commit end` can carry withdrawals and cannot carry announcements. This spec makes the announced count honest by construction, so it stays correct when a producer is wired. The find is recorded in `plan/journal/unwired-feature.md` (2026-09-05).
- `SendRoutes` drops the work for a peer that is not established, where `AnnounceNLRIBatch` queues it for establishment. This spec reports the drop and does not change it.
- `answerValue` (`pkg/plugin/sdk/sdk_engine.go`) discards the collapsed document when a command answers with an error status. This spec works around it by stating the refusal in the sentence. The find is recorded in `plan/journal/error-path-discards-data-already-received.md` (2026-09-05).

## RFC Documentation (Scope: protocol)
Not applicable. No RFC requirement is implemented, changed or newly proven by
this spec, and no wire behavior changes.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
