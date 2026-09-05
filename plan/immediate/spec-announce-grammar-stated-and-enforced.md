# Spec: announce-grammar-stated-and-enforced

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | - |
| Phase | 5/6 |
| Handoff | - |
| Updated | 2026-08-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## RESUME HERE (2026-08-30, session stopped on token budget)

The implementing session was stopped mid-Phase-5, not because anything failed.
Its last reported state: "Both wire tests pass. Now Phase 5's forced red for the
handler change."

| Phase | State |
|-------|-------|
| 1 probe | DONE. A-6 is answered: `internal/test/fixture/ui_fixture_cli_announce.go` exists, and one fixture starts a daemon over ephemeral SSH plus a `ze-peer`. AC-6 and AC-7 do NOT have to split |
| 2 renderer | edits present in `usage.go`, `ze-extensions.yang`, `ze-cli-announce-cmd.yang` |
| 3 completion and help | edits present in `completer.go`, `help.go`, and both test files |
| 4 parser | edits present in `announce.go` and `announce_test.go` |
| 5 functional coverage | the two wire tests PASS. The FORCED RED is NOT done, so their discrimination is unproven |
| 6 documentation | NOT STARTED. This is a live debt against `ai/rules/documentation.md`, which requires the page edit in the same work as the code |

The next session does three things, in this order. First, the Phase 5 forced
red: revert the handler change, rebuild the artifact the test drives so the
revert takes effect, confirm RED, restore, confirm GREEN, and record the red
output. A test written against already-working code has unproven discrimination
until a red is forced. Second, AC-2's corpus diff, which is order-sensitive and
was NOT captured before the renderer edits landed: the before-capture must come
from a tree WITHOUT the Phase 2 changes, so take it from the commit that
precedes them rather than from the working tree. Third, Phase 6.

Nothing here was verified against source by the supervising session. Every claim
in this block came from the implementing agent or from `git status`, so treat it
as a starting point and read the producer before relying on it
(`ai/rules/evidence.md`).

## Task

The grammar the command model STATES and the grammar the handler ENFORCES
disagree about `announce flowspec`, in both directions, and nothing proves
either one.

The model understates the obligation. `announce flowspec` requires an action,
and the generated usage line brackets all three spellings of it, so the line
says the action is optional. `splitFlowspecArgs`
(`internal/component/bgp/plugins/cmd/announce/announce.go`) answers
`errFlowspecRequiresAction` for a tail that names none, and
`handleAnnounceFlowspec` answers `errMissingFlowspecComponents` for a tail with
no match component. The operator reads the obligation from the error rather
than from the line.

The handler under-enforces. `parseTrailingOpts` ends its keyword loop with a
default arm that returns the options it has and no error, so every remaining
token is discarded in silence. `announce flowspec destination 1.1.1.1/32
discard rate-limit 500` announces a plain discard and loses the rate limit
without a word to the operator. The guard above it, the `consumed` comparison
in `handleAnnounceFlowspec`, reads the action tokens only and never the tail.

Nothing proves either half. `init()` in that file registers seven handlers and
no `.ci` exercises one of them. The whole CLI exercise in the corpus is four
`ze withdraw help` lines in
`test/ui/test-withdraw-forms-are-separate-commands.ci`, which assert usage text
with no daemon behind them.

The goal is that the line an operator reads, the grammar the handler accepts,
and a functional test that drives the real entry point all say the same thing.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/bgp/on-demand-origination.md` - the design page for the announce and withdraw verbs
  → Decision: the action is an extended community and not an alternation, per RFC 8955 Section 7. `rate-limit` and `discard` are sugar synthesized into `traffic-rate` arguments, so the model states `community <value>` as the general case and keeps the other two as spellings of it. A required one-of therefore groups three spellings of ONE thing, and must not be designed as a choice between three mechanisms.
  → Constraint: the page carries a paragraph that is IMPRECISE and must be sharpened in this work, not deleted. It says closing this gap "needs a modifier that states 'one of these', which no command declares today". `ModifierChoice` does state a one-of, but an OPTIONAL one over single words drawn from ONE leaf's enum (`usageChoiceToken`), and three modules declare it: `ze-pki-cmd.yang`, `ze-peer-cmd.yang` and `ze-policy-cmd.yang`. What is absent is a REQUIRED one-of over sibling GROUPS that each carry their own value. The page edit names that distinction, because a reader who takes the sentence at face value will reach for `choice` and find it cannot carry `rate-limit <bytes-per-second>`.
  → Constraint: an augmented container states `config false` itself, because `mergeYANGEntry` (`internal/component/config/yang/command.go`) drops a child whose own `Config` is not false, and goyang never propagates it from a parent. Any new container added under an augment carries the same obligation.
  → Constraint: `declaredContainerOrder` counts the PARENT's own container statements, so an augmented container's `ModifierOrder` is 0 and the augmented set sorts by name ahead of every locally declared group. This is why the nineteen match components print before the action.

- [ ] `docs/architecture/api/commands.md` - cited in the header of `internal/component/command/usage.go` as the page for the generated invocation form
  → Constraint: the page documents neither `ze:modifier` nor `UsageKind`. The modifier vocabulary lives only in a comment in `ze-extensions.yang` and in a table in `usage.go`, so this spec's new word has no documented home until the page gains one.

- [ ] `ai/patterns/cli-command.md` - the structural template for a command surface change
  → Constraint: the action keyword comes before any user-supplied identifier, and the YANG tree defines the dispatch path. The wrapper container this spec adds is NOT a dispatch node and carries no `ze:command`, so it changes no path: it groups three keywords that already precede their own values. This is what keeps the change a rendering change rather than a command-surface change.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc8955.md` - the FlowSpec traffic action is an extended community
  → Constraint: `traffic-rate-bytes` is extended community 0x8006 carrying a 2-octet AS and a 4-octet float, and a negative rate must be treated as zero. `discard` is that community with a rate of zero, which is why the design page calls the two spellings sugar over one mechanism rather than two branches of a choice.
  → Decision: this spec changes which operator input is ACCEPTED and never how an accepted input encodes. The bytes for a given accepted command are identical before and after, which is what makes the interop row N-A rather than skipped.

**Key insights:** (minimal context to resume after compaction)
- Three faces of one problem: the model understates, the handler under-enforces, nothing proves either.
- Owner decision, 2026-08-30: the modifier is a NEW parent container carrying `ze:modifier "one-of"` wrapping the three action containers, with `modifierChildren` recursing one level. The rejected alternative was a set key on the three siblings.
- Owner decision, 2026-08-30: the `.ci` coverage gate blind spot gets its own spec AFTER this one. Its row is in `plan/journal/gate-excludes-part-of-its-population.md`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/command/usage.go` - declares the `Modifier` vocabulary, renders every generated usage line
  → Constraint: `modifierNames` is the single table `ParseModifier` reads, so a module and this package cannot disagree about a modifier's name. A new word is added there or it does not exist.
  → Constraint: `appendGroupTokens` is the sole brackets or no-brackets decision. `ModifierRequired` emits a bare keyword and its values, `ModifierChoice` goes to `usageChoiceToken`, and everything else goes to `usageGroupToken`.
  → Constraint: `modifierChildren` selects DIRECT children only and does not recurse, so a nested member set is invisible to it today. The chosen shape requires exactly this recursion.
  → Constraint: `UsageToken.Group` holds a group's VALUES, not alternatives. There is no representation for "one of these groups, each with its own values", so a new `UsageKind` is owed alongside the new `Modifier`.
  → Constraint: `UsageToken` states in its own doc comment that it is published in the command catalog and that its JSON keys are part of that contract. `UnmarshalJSON` errors on an unknown kind name, so a new kind is a contract change and not an internal detail.
  → Constraint: brackets are emitted in exactly two arms of `writeUsageToken`, the `UsageGroup` with `UsageGroupRepeat` case and the `UsageChoice` case. Alternation with a pipe is emitted only by the joins in `writeChoiceMembers` and `writeUsageValue`.
  → Constraint: no modifier is ENFORCED anywhere. `internal/component/plugin/server` holds zero non-test references to `Modifier`, `matchCommandTokens` hands the whole tail to the handler, and `validateCommandArgs` walks the command node's own `ArgDefs` only. A modifier is a render, completion and help contract. Stating the obligation in the model does NOT make the handler enforce it, which is why this spec carries the parser fix as well.

- [ ] `internal/component/bgp/plugins/cmd/announce/announce.go` - the seven handlers, the argument splitters, the trailing option parser
  → Constraint: `splitFlowspecArgs` scans left to right and returns at the FIRST action keyword. Everything before it is components and everything after it is the trailing options, so a second action keyword is not an error at the splitter: it lands in the option tail.
  → Constraint: `parseTrailingOpts` ends with a default arm returning the options and no error. This is the silent discard, and the same function serves the unicast and blackhole forms, so the fix reaches all three.
  → Constraint: the `consumed` comparison in `handleAnnounceFlowspec` guards the ACTION tokens only. Its own comment says a dropped token "advertises something they did not describe", which is exactly what the option tail does today.
  → Decision: the components rule is "at least one", checked as a token count. Which keywords are valid is decided later by the codec, so the count and the vocabulary are enforced in two different packages.

- [ ] `internal/component/bgp/plugins/cmd/announce/yang/ze-cli-announce-cmd.yang` - the announce command model
  → Constraint: `community`, `rate-limit`, `discard`, `tag` and `for` all carry `ze:modifier "once"` today. The first three are the set this spec regroups.

- [ ] `internal/component/bgp/plugins/nlri/flowspec/yang/ze-flowspec-cmd.yang` - the augment contributing the match components
  → Constraint: eighteen containers carry `ze:modifier "repeat"` and `rd` carries `once`. This spec does NOT regroup them: see Known Limitations.

- [ ] `internal/component/bgp/plugins/nlri/flowspec/component_parity_test.go` - the parity guard over the augmented component set
  → Constraint: `augmentedContainerNames` parses container names with a literal eight-space prefix and calls `t.Fatal` on zero names. Nesting anything inside that augment silently changes what it reads. The chosen shape nests only the ACTION containers, which live in the announce module, so this guard is untouched. Any later work that nests the components repairs this parser first.

**Behavior to preserve:**
- The rendered order of the augmented match components ahead of the action and the options. `modifierChildren` sorts by `ModifierOrder` then name, and nothing is hoisted for being required.
- Every existing accepted spelling of `announce flowspec`, `announce unicast`, `announce blackhole` and the three withdraw forms. This spec refuses MORE input, never less.
- The four usage lines pinned by `test/ui/test-withdraw-forms-are-separate-commands.ci`.

**Behavior to change:**
- The generated `announce flowspec` line states the action as required and alternated, rather than as three optional groups.
- A trailing token that no option keyword claims becomes an error rather than a silent discard.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator types `ze announce flowspec ...` as argv, or `announce flowspec ...` in the interactive editor.

### Transformation Path
1. `goyang` parses the announce module and its flowspec augment. `mergeYANGEntry` and `getModifierExtension` (`internal/component/config/yang/command.go`) turn each `ze:modifier` container into a `command.Node` carrying `Modifier` and `ModifierOrder`.
2. `Usage` walks that node, `modifierChildren` selects its trailing groups, and `appendGroupTokens` turns each into a `UsageToken`. `UsageLine` and `writeUsageToken` render the tokens an operator reads.
3. Independently of steps 1 and 2, argv reaches `matchCommandTokens` (`internal/component/plugin/server/command.go`), which strips the registered command path and hands the whole tail to the handler.
4. `splitFlowspecArgs` divides that tail into components, action and trailing options. `parseTrailingOpts` reads the options. Neither reads the command tree.
5. `handleAnnounceFlowspec` encodes the NLRI, builds the extended community, and `announceAndTrack` records the announcement and hands the batch to the reactor.

Steps 2 and 4 are the two halves that disagree. They share no code, which is why this spec changes both and proves them together.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG model to command tree | `mergeYANGEntry` and `getModifierExtension` (`internal/component/config/yang/command.go`) | No |
| Command tree to rendered line | `Usage` and `UsageLine` (`internal/component/command/usage.go`) | No |
| CLI argv to handler | `matchCommandTokens` (`internal/component/plugin/server/command.go`) | No |
| Handler to wire | `announceAndTrack`, then the reactor's NLRI batch announce | No |

### Integration Points
- `modifierNames` and `usageKindNames` (`internal/component/command/usage.go`): the two single declarations a new word joins.
- `matchChildren` (`internal/component/command/completer.go`) and `listedChildNames` (`internal/component/command/help.go`): both special-case `ModifierChoice` today and both owe a decision for the new value.
- `errFlowspecActionExtraTokens` (`internal/component/bgp/plugins/cmd/announce/announce.go`): the existing precedent for the error the default arm should answer, so the parser fix reuses a shape rather than inventing one.
- `startFixtureProcess` and `Poll` (`internal/test/fixture/`): the helpers that start `ze-test peer` and a daemon, already used separately and combined for the first time here.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The functional tests drive argv through `matchCommandTokens` to the handler, not the handler directly. The offline usage test reads the rendered line, not the token list |
| No unintended coupling (components stay isolated) | Yes | The announce plugin declares the wrapper in its own module. `internal/component/command` gains a vocabulary word and learns nothing about announce or FlowSpec |
| No duplicated functionality (extends existing, does not recreate) | Yes | The new word joins `modifierNames` and `usageKindNames`, the two existing single declarations. The parser fix reuses the shape of `errFlowspecActionExtraTokens` rather than inventing an error |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Usage rendering runs on help and catalog build, never on a wire path. No buffer, pool, or encoder is touched |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The renderer gains one arm keyed on a modifier VALUE, not on a command name. No core package learns the word `announce`, `flowspec`, `discard`, or `rate-limit` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `one-of` modifier has exactly one user in the repository today | A grep for a second handler enforcing a choose-one rule found none. Everything else found was "keyword requires a value" or "requires a selector" | The vocabulary word is justified by one command, which `ai/rules/simplicity.md` treats as a warning sign. Raised with the owner at the scope gate and confirmed | Re-run the grep at review and record the result | unvalidated |
| A-2 | Nesting the three action containers does not disturb `component_parity_test.go` | `augmentedContainerNames` parses the FLOWSPEC augment file, and the action containers live in the ANNOUNCE module | The parity guard goes red or, worse, silently reads a smaller set | Run that test before and after the YANG change | unvalidated |
| A-3 | No stored artifact holds a serialized usage grammar that a new `UsageKind` word would fail to round-trip | Traced every hop. `internal/le/wikicatalog` is typed as `[]command.UsageToken`, so `UsageKind.UnmarshalJSON` runs on the live bytes, but producer and consumer share `usageKindNames`, so one table gains one row. `internal/le/site/catalog.go` and `internal/le/docvalid/command_surfaces.go` both carry `Kind` as a plain string. No golden file and no `testdata/` holds a grammar | Nothing. The rework risk is one table row and two republished sibling artifacts | Re-run `./le docvalid` after the renderer change | confirmed |
| A-4 | A `.ci` can drive `ze announce` as argv against a live daemon | BROKEN, and known so before implementation. The daemon publishes its ephemeral SSH address at start into the file named by `ZE_SSH_EPHEMERAL`, so only a Go fixture can read it and set `ZE_SSH_HOST` and `ZE_SSH_PORT` on the client. `option=env` is static, and no `.ci` in `test/ui/` or `test/plugin/` sets `ZE_SSH_*` | The argv-level coverage needs a Go fixture, which is now in Files to Create rather than discovered mid-implementation | `internal/test/fixture/ui_fixture_cli_verb_daemon_dispatch.go` is the working precedent | broken |
| A-6 | A single fixture can start a daemon over SSH AND a `ze-peer`, so one test proves argv reaches the wire | No precedent found. `runCLIVerbDaemonDispatch` writes its own config and starts the daemon itself, so a `.ci`-launched `ze-peer --port $PORT` is not in that config | The argv proof and the wire proof split into two tests: argv reaches the handler, and the handler reaches the wire. That is weaker than one end-to-end chain and must be stated as such rather than papered over | Write the fixture as a draft under `test/draft/` first and see whether the peer can be started from it | unvalidated |
| A-5 | Refusing an unclaimed trailing token breaks no existing caller | This spec refuses more input than before, and no `.ci` exercises any announce form | A caller somewhere passes a trailing token that works by accident today | Grep every `.ci`, `.et` and unit test for announce invocations, then run the unit suite | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The new `UsageKind` word reaches the published catalog and a consumer pinned to today's set refuses it | The catalog build or `docvalid` fails after the renderer change | Land the renderer and the catalog word in one commit, and check the website data files for a stored grammar before starting |
| R-2 | The `modifierChildren` recursion changes an unrelated command's rendered line | A usage assertion elsewhere goes red | Recurse only into a node carrying the new modifier, never into every group. Re-derive all 384 rendered lines and compare before and after |
| R-3 | Refusing the unclaimed trailing token turns a silent success into a red test elsewhere | A previously passing test fails on an announce invocation | A-5's grep runs before the parser change |
| R-4 | The completion arm offers the new parent container's name, which the operator never types | Completion suggests a word the handler rejects | `matchChildren` special-cases `ModifierChoice` today. The new value needs its own arm, and a completion test pins that the parent name is never offered |
| R-5 | Test and gate repair consumes the session instead of the product | A third repair to scaffolding begins | `ai/rules/pre-release.md`: stop, return to the product code, and report what is left red |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A wrong renderer change alters the published usage line of any command carrying a modifier, which is most of the 384. A wrong parser change refuses an announce an operator relies on. Neither touches wire encoding or a live session |
| How is it reverted? | Single commit revert. Nothing here is observed by a peer and no config migration is involved |
| Who else touches this path? | `spec-generated-command-usage` is in-progress over the same renderer and is the parent of this work. Its closure note names this spec's subject as its one remaining item, so coordinate before landing renderer changes |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze announce flowspec help` as argv | → | `Usage`, `appendGroupTokens`, `writeUsageToken` | `test-announce-forms-state-their-action` (`test/ui/*.ci`) |
| `ze announce flowspec destination 1.1.1.1/32 discard rate-limit 500` as argv against a live daemon | → | `parseTrailingOpts` | `TestAnnounceRefusesAnUnclaimedTrailingToken` and the fixture below |
| `ze announce unicast 10.0.0.0/24` as argv against a live daemon with a peer | → | `handleAnnounceUnicast`, `announceAndTrack`, the reactor | `cli-announce-reaches-the-wire` (Go fixture plus `.ci`) |
| `ze withdraw tag <key>` as argv after an announce | → | `handleWithdrawTag`, `Registry` | `cli-announce-tag-round-trip` |
| The operator presses tab after `announce flowspec destination 1.1.1.1/32 ` | → | `matchChildren` | `TestCompletionOffersTheActionsNotTheWrapper` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze announce flowspec help` | The usage line states the action unbracketed and alternated, so an operator reads the obligation from the line. The three match spellings appear once each, and no wrapper name appears |
| AC-2 | Every command's usage line is rendered before and after the change | Only the `announce flowspec` line differs. No other command's published line changes |
| AC-3 | `announce flowspec destination 1.1.1.1/32 discard rate-limit 500` | Refused with an error naming the unclaimed token. Nothing is announced |
| AC-4 | `announce unicast 10.0.0.0/24 bogus`, and the same trailing garbage on `announce blackhole` | Refused, naming the unclaimed token. The fix reaches all three forms, not flowspec alone |
| AC-5 | Tab completion after a complete component list | Offers `discard`, `rate-limit` and `community`, and never the wrapper container's name |
| AC-6 | An operator runs `ze announce unicast <prefix>` as argv against a daemon with an established peer | The peer receives an UPDATE carrying that prefix, asserted as bytes or as decoded fields |
| AC-7 | An operator announces with `tag k v`, runs `show announcements`, then `withdraw tag k` | The announcement is listed while live, the withdraw reports one removed, and the peer receives the withdrawal |
| AC-8 | `ParseModifier` is given the new word, and a `UsageToken` carrying the new kind is marshalled and unmarshalled | Both round-trip. An undeclared word still answers false rather than a valid-looking group |
| AC-9 | The `ze-extensions.yang` prose is read | Both places that enumerate the occurrence set name the new one. Neither still says four |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Asks how to announce a flowspec rule and reads the usage line | model to `Usage` to `UsageLine` to stderr | `test-announce-forms-state-their-action` |
| 2 | Announces a flowspec discard rule to a peer | argv to `matchCommandTokens` to `handleAnnounceFlowspec` to reactor to wire | `cli-announce-reaches-the-wire` |
| 3 | Mistypes a second action and is told so | argv to `splitFlowspecArgs` to `parseTrailingOpts` to error | AC-3's functional row |
| 4 | Tracks an announcement by tag, lists it, and withdraws it | argv to the three handlers to `Registry` to wire | `cli-announce-tag-round-trip` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseModifierReadsTheOneOfWord` | `internal/component/command/usage_test.go` | AC-8: the new word round-trips and an undeclared word still answers false | |
| `TestUsageKindReadsBackFromItsWord` (extend) | `internal/component/command/usage_test.go` | AC-8: the existing kind enumeration covers the new kind | |
| `TestUsageRendersARequiredOneOfGroup` | `internal/component/command/usage_test.go` | AC-1: a wrapper with three value-carrying children renders unbracketed and alternated | |
| `TestModifierChildrenRecursesOnlyIntoTheOneOf` | `internal/component/command/usage_test.go` | AC-2 and R-2: no other modifier gains recursion | |
| `TestCompletionOffersTheActionsNotTheWrapper` | `internal/component/command/completer_test.go` | AC-5 and R-4 | |
| `TestAnnounceRefusesAnUnclaimedTrailingToken` | `internal/component/bgp/plugins/cmd/announce/announce_test.go` | AC-3 and AC-4, driven through all three announce forms | |
| `TestAnnounceFlowspecUsageStatesTheComponents` (update) | `internal/component/plugin/server/usage_model_test.go` | AC-1: the pinned line is updated to the new rendering, deliberately, not incidentally | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| No new numeric input | N-A | N-A | N-A | N-A |

The parser fix adds no numeric field. `rate-limit`'s bytes-per-second and `for`'s duration keep their existing bounds, and `maxDurationSeconds` already caps the latter.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-announce-forms-state-their-action` | `test/ui/*.ci` | Reads the usage line for each announce form, no daemon. Cheapest proof of the modifier, written FIRST | |
| `cli-announce-reaches-the-wire` | Go fixture plus `test/ui/*.ci` | AC-6: argv reaches the handler and the peer receives the UPDATE | |
| `cli-announce-tag-round-trip` | Go fixture plus `test/ui/*.ci` | AC-7: announce with a tag, list it, withdraw it, peer sees the withdrawal | |
| `cli-announce-refuses-a-second-action` | same fixture | AC-3 through the real entry point, not the unit seam | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | No wire-visible change. The FlowSpec NLRI and the extended community this spec announces are byte-identical to what the same operator input produces today; only which inputs are ACCEPTED changes. `ai/rules/interop-and-goal-validation.md` permits the omission for a change with no wire-visible difference, and `ze-peer` asserts the bytes in the functional rows above | |

## Files to Modify

- `internal/component/command/usage.go` - the `Modifier` and `UsageKind` vocabularies, `appendGroupTokens`, `modifierChildren`, `writeUsageToken`
- `internal/component/command/completer.go` - `matchChildren` gains an arm for the new value
- `internal/component/command/help.go` - `listedChildNames` gains the same decision
- `internal/component/config/yang/modules/ze-extensions.yang` - both places that enumerate the occurrence set
- `internal/component/bgp/plugins/cmd/announce/yang/ze-cli-announce-cmd.yang` - the wrapper container and a revision
- `internal/component/bgp/plugins/cmd/announce/announce.go` - `parseTrailingOpts` refuses an unclaimed token
- `internal/component/plugin/server/usage_model_test.go` - the pinned announce line
- `docs/architecture/api/commands.md` - declared by four changed files' `// Design:` headers, and documents neither vocabulary
- `docs/architecture/bgp/on-demand-origination.md` - declared by two changed files, and carries the imprecise paragraph
- `docs/architecture/config/yang-config-design.md` - its extension table omits `ze:modifier`
- `docs/guide/command-reference.md` - carries no announce row today
- `docs/functional-tests.md` - the suite table, if a new fixture lands
- `ai/INDEX.md` - the command-grammar row gains `ze:modifier` and `one-of`

## Files to Create

- `internal/test/fixture/ui_fixture_cli_announce.go` - the joint fixture: a daemon over ephemeral SSH plus a `ze-peer`, driving `ze announce` as argv
- `test/ui/test-announce-forms-state-their-action.ci` - the offline usage-line proof
- `test/ui/cli-announce-reaches-the-wire.ci` - the fixture shim
- `test/ui/cli-announce-tag-round-trip.ci` - the fixture shim
- the retired deferral shard "announce-grammar-stated-and-enforced" - created only if something is deferred

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/plugins/cmd/announce/yang/ze-cli-announce-cmd.yang` gains the wrapper container and a revision. `internal/component/config/yang/modules/ze-extensions.yang` states the occurrence set TWICE, in the `extension modifier` description and in a comment table, and both say "four occurrences". Both gain the fifth |
| YANG validation constraints | N-A | The wrapper declares no leaf. The three action containers keep their existing `length` and `pattern` |
| YANG custom validators | N-A | No `ze:validate` is involved. The one-of is structural |
| CLI commands/flags | No | No new command word. The wrapper's name is never typed, the same contract `ModifierChoice` already has |
| CLI grammar (keyword before value) | Yes | `internal/component/command/usage.go`: `Modifier`, `modifierNames`, `UsageKind`, `usageKindNames`, `appendGroupTokens`, `modifierChildren`, `writeUsageToken`. Gate: `./le cli-grammar` |
| Editor autocomplete | Yes | `internal/component/command/completer.go`, `matchChildren`. It skips `ModifierChoice` today, and the new value must make it recurse into the wrapper's three children rather than offer the wrapper's name (R-4) |
| Functional test for new RPC/API | Yes | See the Functional Tests table |
| Pipe completeness | N-A | The seven handlers already register through the command registry and no new answer shape is added |
| Env var registration | N-A | No leaf under `environment/` |
| Doctor check for runtime dependencies | N-A | No new file path, socket, listen port, kernel module, or binary |
| Prometheus counters/metrics | N-A | No new observable state. The announce registry is already reported by `show announcements` |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new SAFI, capability, or attribute. FlowSpec NLRI encoding is unchanged |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The grammar is STATED, not added. No `docs/features.md` row changes |
| 2 | Config syntax changed? | N-A | The command tree is `config false` throughout |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` carries NO announce row today. Add the three announce forms and the three withdraw forms |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`. Four files declare it in their `// Design:` header (`usage.go`, `completer.go`, `help.go`, `node.go`) and it documents neither `ze:modifier` nor `UsageKind` |
| 5 | Plugin added/changed? | No | `docs/guide/plugins.md` is untouched |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` carries the only operator-facing announce prose. Its `announce unicast ... community` example stays valid; verify rather than assume |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | `docs/features/rfc-status.md` cites `announce blackhole`, and the blackhole form's grammar is unchanged. Named here as unaffected, which satisfies the check |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` suite table, if a new Go fixture lands |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` cites `announce blackhole` only |
| 12 | Internal architecture changed? | Yes | `docs/architecture/bgp/on-demand-origination.md` (the imprecise paragraph) and `docs/architecture/config/yang-config-design.md`, whose extension table lists `ze:validate` and `ze:command` and omits `ze:modifier` entirely |
| 13 | Route metadata keys added/changed? | N-A | |
| 14 | Prometheus counters added/changed? | N-A | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | The handler count does not change, so `docs/plugin-overview.md` and `docs/features/plugins.md` stand. The GENERATED inventories do change: `../wiki/command-catalog.md` and `../gh-pages/data/cli-commands.json` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes, DERIVED | Run `./le spec citation anchors spec plan/immediate/spec-announce-grammar-stated-and-enforced.md`. The `// Design:` declarations that BLOCK are listed in Files to Modify |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/configuration.md`, `docs/config-reference.md`, `docs/comparison.md` and `docs/features.md` each cite `announce unicast` or `announce blackhole`. None shows a flowspec action, so all four should survive. Verify each |

## Implementation Steps

1. **Phase: Wiring and probe (MANDATORY FIRST)** -- prove the joint fixture runs before any acceptance criterion depends on it
   - Tests: `cli-announce-reaches-the-wire` as a DRAFT under `test/draft/ui/`, asserting only that the daemon starts, the peer establishes, and `ze announce unicast` as argv reaches the handler
   - Files: `internal/test/fixture/ui_fixture_cli_announce.go`
   - Verify: A-6 is confirmed or broken. If the peer cannot be started from the same fixture, STOP and report: AC-6 and AC-7 then split into two weaker tests and the owner decides, per `ai/rules/completion.md`
2. **Phase: the line states the obligation** -- the renderer, offline and cheapest to prove
   - Tests: `TestParseModifierReadsTheOneOfWord`, `TestUsageRendersARequiredOneOfGroup`, `TestModifierChildrenRecursesOnlyIntoTheOneOf`, `test-announce-forms-state-their-action`
   - Files: `usage.go`, `ze-extensions.yang`, `ze-cli-announce-cmd.yang`, the pinned line in `usage_model_test.go`
   - Verify: render every command's line before and after and diff them. Only the announce flowspec line moves (AC-2)
3. **Phase: completion and help agree with the line**
   - Tests: `TestCompletionOffersTheActionsNotTheWrapper`
   - Files: `completer.go`, `help.go`
   - Verify: the wrapper name is never offered and never listed
4. **Phase: the handler enforces what the line states**
   - Tests: `TestAnnounceRefusesAnUnclaimedTrailingToken`, then `cli-announce-refuses-a-second-action` through the real entry point
   - Files: `announce.go`
   - Verify: run A-5's grep for existing callers BEFORE the change, then confirm the refusal names the offending token
5. **Phase: the seven handlers get their first functional coverage**
   - Tests: `cli-announce-reaches-the-wire`, `cli-announce-tag-round-trip`, promoted from `test/draft/` once green
   - Files: the fixture and its three `.ci` shims
   - Verify: force a RED phase per `ai/rules/interop-and-goal-validation.md` by reverting the handler and rebuilding, then restore and confirm green
6. **Phase: the pages move with the code**
   - Files: the six documentation files in Files to Modify, plus `./le site build` and `./le wiki-catalog update` to republish the two generated catalogs
   - Verify: `./le docvalid` and `./le doc check verify` are clean, including the pre-existing catalog staleness this republishing clears

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation named by file and symbol |
| Correctness | The rendered line and the handler's refusals agree token for token. An operator who obeys the line is never refused, and an operator the line does not permit is always refused |
| Data flow | The modifier stays a render, completion and help contract, and enforcement stays in the handler. No core package learns a FlowSpec word |
| Naming | The new occurrence word reads as an obligation an operator can act on, and matches the register of `once`, `repeat`, `required` and `choice` |
| Vacuity | AC-2's corpus diff was run and its before-file was generated BEFORE any renderer edit. A diff produced after the fact proves nothing |
| Rule: `ai/rules/evidence.md` | The refusal in `parseTrailingOpts` fails closed: an unrecognized token is an error, never a zero value that reads as an absent option |
| Rule: `ai/rules/interop-and-goal-validation.md` | The functional tests were forced RED by reverting the handler and rebuilding, and the red output is recorded |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The announce flowspec usage line states the action as required | `ze announce flowspec help` and read it |
| No other command's line changed | `ze help command --json` publishes a `usage` string and a `grammar` token list per command (`commandEntry`, `cmd/ze/help_command.go`). Capture it BEFORE any renderer edit, capture it after, and diff. Only the announce flowspec entry may differ. `./le docvalid usage-contract` walks the same tree and is the second reading |
| A second action is refused | `ze announce flowspec destination 1.1.1.1/32 discard rate-limit 500` exits nonzero naming `rate-limit` |
| Seven handlers have functional coverage | `grep -c 'exec=ze announce\|exec=ze withdraw' test/ui/*.ci` is nonzero, and the suite passes |
| The occurrence set has no stale copy | `grep -n 'four occurrences' internal/component/config/yang/modules/ze-extensions.yang` returns nothing |
| The published catalogs match the model | `./le docvalid` clean |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A refused trailing token names the offending value without echoing unbounded operator text into a log |

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

- The published command catalog is ALREADY stale, before this spec changes anything. `../gh-pages/data/cli-commands.json` holds one `announce` entry whose usage string is `announce <unicast|blackhole|flowspec> <args> [tag <key> <value>] [for <duration>]`, the authored sentence the 2026-08-30 split deleted, while the live model now publishes three announce commands. `checkSiblingPublications` is true in the default build and `compareWebsiteCommandCatalog` byte-compares the two, so this is a live disagreement another session left. This spec republishes with `./le site build` and `./le wiki-catalog update`, which clears it as a side effect rather than as its own errand.
- Discovery, per `ai/rules/repo-maintenance.md`: the `ai/INDEX.md` command-grammar row covers the modifier work and the announce row covers the handler work, but neither names `ze:modifier` or `one-of`, so those two keywords are added to the grammar row. The rule preventing regression already exists and needs no change: `ai/rules/cli.md` puts the grammar in the model, and `ai/rules/principles.md` forbids a silently wrong value, which is exactly the default arm this spec repairs. The registries preventing drift are `modifierNames` and `usageKindNames`, each read by exactly one parser.
- The occurrence set is DECLARED TWICE and no check compares the copies: `modifierNames` in Go, and prose in `ze-extensions.yang` that counts "four occurrences" in two separate places. Adding a fifth word makes the prose wrong in two spots. `component_parity_test.go` is the precedent in this repository for a check that compares a Go set against a YANG set, and the same shape would close this.
- `mergeYANGEntry` reads `ze:modifier` only when the node's `Modifier` is still `ModifierNone`, sets `ArgDefs` from the child's own leaves, and `inheritArgDefs` skips any node carrying a modifier. A wrapper container with no leaves therefore gets empty `ArgDefs` and keeps its container children intact, which is the property the one-level recursion depends on. Verified before the shape was chosen rather than after.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The action set is a NEW parent container carrying `ze:modifier "one-of"`, and `modifierChildren` recurses one level into it | A set key naming the group on each of the three sibling containers, with the renderer grouping by key and no recursion | Owner decision, 2026-08-30. The nested shape states the division in the tree where a reader meets it, and the parity guard that the alternative avoided reads a different module's file anyway |
| The three action spellings are grouped, the nineteen match components are not | Model the components' "at least one of nineteen" obligation with the same modifier | An alternation over nineteen names is not a line an operator can read, and the components already render as named repeatable groups. See Known Limitations |
| A NEW modifier word and a NEW `UsageKind`, each with one meaning | Overload `ModifierChoice` so `usageChoiceToken` reads child containers' `ArgDefs` when it has container children, expressing "required" separately. This adds no vocabulary word and keeps the `ze-extensions.yang` prose true | The saving is one YANG comment. The cost is a word whose meaning depends on the tree shape beneath it, which is the defect class `plan/journal/field-carries-two-meanings.md` collects. The published token would change shape either way, so the contract cost is not avoided |
| The proof of AC-1 comes from the 383 lines that must NOT move, not from the one that must | Update the pinned line in `usage_model_test.go` and treat a green suite as the proof | Implementing this means editing the very test that asserts the behaviour being changed, which is how a regression hides. AC-2's whole-corpus before-and-after diff is an independent check that no edited assertion can satisfy by accident |

## Known Limitations

- The "at least one match component" obligation stays unstated in the generated line. Nineteen alternated names would not be readable, and the honest rendering of that rule is an open question this spec does not answer.
- The `.ci` coverage gate stays blind to every CLI verb registered under `internal/component/bgp/plugins/`. Owner decision, 2026-08-30: its own spec, after this one. Row in `plan/journal/gate-excludes-part-of-its-population.md`.

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
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
