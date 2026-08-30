# Spec: flowspec-states-its-own-grammar

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | spec-generated-command-usage.md |
| Phase | - |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`announce flowspec` is the last command in the tree whose description prescribes
a CLI spelling. `./le docvalid usage-contract` reports it, and the sentence
cannot be deleted because the model renders `announce flowspec` and nothing
else.

`spec-generated-command-usage.md` called this the permitted residue, on the
reading that the model cannot express the grammar. That reading is wrong, and
the owner said so on 2026-08-30. Two things were treated as inexpressible and
neither is:

1. **The action.** `rate-limit <bps>` and `discard` looked like a mandatory
   alternation where one branch carries a value, which no `ze:modifier` states.
   They are not an alternation. RFC 8955 makes a FlowSpec action an extended
   COMMUNITY, and `handleAnnounceFlowspec` already agrees: it synthesises
   `traffic-rate` arguments and hands them to `route.ParseExtendedCommunities`.
   Modelled as a community, the action reuses the group `announce unicast`
   already declares and the alternation disappears.
2. **The match components.** They looked like another plugin's vocabulary that
   the announce module must not restate. True, and the wrong conclusion was
   drawn: `augment` lets the flowspec plugin declare them into announce's node
   from its OWN module. 48 modules already use it, `ze-mpls-cmd.yang` among
   them, so no new mechanism is needed.

The goal is that `announce flowspec` renders its whole grammar from the model,
its authored sentence is deleted, and the component keyword set stays declared
in exactly one place.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/bgp/on-demand-origination.md` - the announce command shape
  → Decision: put the command where the grammar divides, and the model states
    the division rather than the handler. `announce` was split into three
    commands on 2026-08-30 for that reason; this spec finishes the third.
- [ ] `docs/architecture/api/commands.md` - the generated invocation form
  → Constraint: `Usage` reads the MODEL and nothing else, so no description
    text can influence a generated line.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc8955.md` - FlowSpec NLRI and traffic actions
  → Constraint: Section 4.2 allows each component type exactly once, which is
    why a repeated keyword joins into one component rather than adding a
    second. The model must say `repeat`, not `once`.

**Key insights:**
- The component keyword set has ONE producer today: `isComponentKeyword`
  (`internal/component/bgp/plugins/nlri/flowspec/plugin_encode_text.go`), over
  19 `kw*` constants. Declaring them in YANG creates a second copy, so the copy
  needs a check that compares it against that function.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/cmd/announce/announce.go` - `handleAnnounceFlowspecCmd` takes the whole tail, `splitFlowspecArgs` cuts it at the action keyword, and everything before that cut is passed to `registry.EncodeNLRIByFamily` unread
- [ ] `internal/component/bgp/plugins/nlri/flowspec/plugin_encode_text.go` - `parseComponentText` dispatches on 19 keywords; `parseNumericComponentText` consumes value tokens until the next component keyword; `parseOperatorValue` accepts `>=`, `<=`, `!=`, `>`, `<`, `=` or none, then a decimal or hex number
- [ ] `internal/component/bgp/plugins/cmd/announce/yang/ze-cli-announce-cmd.yang` - the `flowspec` container declares a `ze:command` and no arguments
- [ ] `internal/component/plugin/server/usage_model_test.go` - `TestModifierGroupsLeaveDispatchUntouched` is the harness proving a modifier group adds no argument definition to the command node

**Behavior to preserve:**
- Every string an operator types today. `announce flowspec destination 192.0.2.0/24 protocol =6 destination-port =80 rate-limit 9600` must work unchanged, and so must `discard`.
- `handleAnnounceFlowspec` continues to receive the raw tail. A modifier group adds no argument definition to the command node, so `validateCommandArgs` neither consumes nor refuses a token it passes through today.

**Behavior to change:**
- `announce flowspec` gains `community <value>` as a way to state the action directly, alongside the `rate-limit` and `discard` spellings.
- The generated usage line states the components, the action and the trailing options.

## Data Flow (MANDATORY)

### Entry Point
`announce flowspec ...` typed at the CLI, dispatched to `ze-bgp:announce-flowspec`.

### Transformation Path
The dispatcher strips `announce flowspec` and passes the tail. `splitFlowspecArgs` cuts at the action. Components go to `registry.EncodeNLRIByFamily`, which routes to the flowspec plugin's encoder. The action becomes an extended community on the attribute builder.

### Boundaries Crossed
The announce command plugin to the flowspec NLRI plugin, through the family registration seam. This spec adds a second crossing in the MODEL: an `augment` from the flowspec module into the announce module's tree.

### Integration Points
`BuildCommandTree` must see augmented children as modifier groups of the flowspec command node, with a defined order.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by |
|----|-----------|-------|----------|--------------|
| A-1 | `BuildCommandTree` sees children an `augment` from another module contributes | `ze-mpls-cmd.yang` augments `/clishowcmd:show` and its commands work | the components cannot live in the flowspec module and the spec needs a different home for them | a rendered usage line carrying an augmented component |
| A-2 | A modifier group adds no argument definition to the command node, so dispatch is untouched | `TestModifierGroupsLeaveDispatchUntouched`, and the same property held when policy's `filter` and `source-asn4` became groups on 2026-08-30 | every flowspec invocation breaks | that test, extended with a flowspec case |
| A-3 | `ModifierOrder` is defined for an augmented child | `declaredContainerOrder` reads the declaring module's statement order | the usage line's component order varies between runs | a repeated render asserting a stable line |

### Risks
- The line grows long: 19 components plus community, tag and for. It is honest rather than short, and no shorter line is truthful.
- A second declaration of the keyword set. Mitigated by the check in AC-4, not by trusting the two to stay in step.

## Blast Radius

`announce flowspec` only. No other command declares components, and the augment names one target node.

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `announce flowspec destination 192.0.2.0/24 protocol =6 rate-limit 9600` typed at the CLI | → | `handleAnnounceFlowspecCmd` (`internal/component/bgp/plugins/cmd/announce/announce.go`) receives the identical argument slice, and the command node carries zero own argument definitions | `TestModifierGroupsLeaveDispatchUntouched` |
| `announce flowspec help` typed at the CLI | → | `Usage` (`internal/component/command/usage.go`) renders the components from the augmented model | `TestAnnounceFlowspecUsageStatesTheComponents` |

## Acceptance Criteria

| ID | Criterion |
|----|-----------|
| AC-1 | The flowspec NLRI plugin declares its own YANG module augmenting the announce module's `flowspec` container with one `ze:modifier "repeat"` group per component keyword |
| AC-2 | `announce flowspec` renders a usage line stating every component, the action and the trailing options |
| AC-3 | The authored `Usage:` sentence is deleted from `ze-cli-announce-cmd.yang` and `./le docvalid usage-contract` reports no difference for it |
| AC-4 | A test asserts the YANG-declared component set equals `isComponentKeyword`'s set exactly, in both directions, so the copy cannot drift from its producer |
| AC-5 | `community <value>` states a FlowSpec action, and `rate-limit` and `discard` keep working as they do today |
| AC-6 | Every flowspec invocation in the existing test corpus passes unchanged |

## End-to-End User Stories

An operator types `announce flowspec help` and reads a form naming the components they may match on, rather than a sentence someone wrote by hand that no gate compares against the code.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestModelDeclaresEveryComponentKeyword` | `internal/component/bgp/plugins/nlri/flowspec/yang/component_parity_test.go` | the YANG-declared component set equals `isComponentKeyword`'s set in both directions (AC-4) | |
| `TestAnnounceFlowspecUsageStatesTheComponents` | `internal/component/command/usage_test.go` | the rendered line names the components, the action and the trailing options, and is stable across repeated renders (AC-2, A-3) | |
| `TestFlowspecActionAcceptsACommunity` | `internal/component/bgp/plugins/cmd/announce/announce_test.go` | a community reaches `ParseExtendedCommunities`, and `rate-limit` and `discard` still produce the traffic-rate they do today (AC-5) | |
| `TestModifierGroupsLeaveDispatchUntouched` | `internal/component/plugin/server/usage_model_test.go` | the flowspec case dispatches the identical argument slice, and the command node carries zero own argument definitions (A-2) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| component value | per `componentMaxValue` by type | the type's max | N/A | max + 1 refused by `parseNumericComponentText` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing flowspec cases | `internal/component/bgp/plugins/cmd/announce/announce_test.go` | every invocation an operator types today keeps working (AC-6) | |
| `announce flowspec help` | `test/plugin/*.ci` | an operator reads the generated form instead of an authored sentence | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | No wire-visible change. The same NLRI and the same extended community are produced from the same operator input; this spec moves a grammar from prose into the model | N-A |

## Files to Modify
- `internal/component/bgp/plugins/cmd/announce/yang/ze-cli-announce-cmd.yang` - delete the authored sentence
- `internal/component/bgp/plugins/cmd/announce/announce.go` - accept a community as the action
- `internal/component/plugin/server/usage_model_test.go` - the wiring case

## Files to Create
- a YANG module under `internal/component/bgp/plugins/nlri/flowspec/yang/` carrying the augment, with its embed and register glue
- a test asserting the model and `isComponentKeyword` agree

### Documentation Update Checklist (BLOCKING)
- `docs/architecture/bgp/on-demand-origination.md` states that flowspec keeps its own sentence and why. That paragraph becomes wrong and is rewritten in the same work.

## Implementation Steps
1. Confirm A-1 by rendering one augmented component before writing the rest.
2. Declare the component groups in the flowspec module.
3. Add the drift check against `isComponentKeyword`.
4. Accept a community as the action.
5. Delete the sentence and re-read the gate.
6. Rewrite the design page paragraph.

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The model states the grammar | `./le docvalid usage-contract` reports no difference for `announce flowspec` |
| The copy cannot drift | the AC-4 test fails when a keyword is added to either side alone |
| Nothing an operator types changed | the announce test corpus passes unchanged |

## Key Design Decisions
- **The action is a community, not an alternation.** RFC 8955 says so and the handler already builds one. Modelling the sugar rather than the thing is what made this look inexpressible.
- **The components are declared by their owner.** `augment` puts them in the flowspec module, so the announce module never names another plugin's vocabulary.
- **The second copy gets a check, not a promise.** `isComponentKeyword` stays the producer and the model is compared against it.

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
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
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
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Known Limitations
- The rendered line is long. That is the grammar, and a shorter line would understate it.
- `parseNumericComponentText` accepts several values under one keyword (`port >80 <100`). A repeat group renders the keyword and its value with a repetition marker, which states the repetition without stating that it may occur within one keyword as well as across keywords.
- The line brackets the action and every component, so it reads as though both were optional. The handler says otherwise: `splitFlowspecArgs` answers `errFlowspecRequiresAction` for a tail naming none, and `handleAnnounceFlowspec` answers `errMissingFlowspecComponents` for a tail with no component. A group states `once` or `repeat`, and `required` is the wrong word for one member of a set where any single member satisfies the rule, so closing this needs a modifier that states "one of these" and no command declares one today. The deleted authored sentence stated the obligation and the generated line does not, which is the one thing this change made a reader read for the worse. Recorded on the design page rather than here, because this spec is removed at closure.
- The augmented components sort by name, ahead of the action and the options, and no module can choose otherwise. `declaredContainerOrder` counts the PARENT's own container statements, so every augmented container carries `ModifierOrder` 0 and `modifierChildren` breaks the tie on the name. The order is stable and it is not the module's to state.

---

## Implementation Summary

### What Was Implemented
- `internal/component/bgp/plugins/nlri/flowspec/yang/ze-flowspec-cmd.yang` declares the nineteen match components as `ze:modifier` groups and `augment`s them onto `/cliannouncecmd:announce/cliannouncecmd:flowspec`. Each container states `config false` itself, which `mergeYANGEntry` requires and goyang does not inherit.
- `ze-cli-announce-cmd.yang` lost its authored `Usage:` sentence and gained the action and the trailing options as declared groups: `community`, `rate-limit`, `discard`, `tag`, `for`.
- `splitFlowspecArgs` gained a `kwCommunity` branch and `trailingOptsAt`, so an action `route.ParseExtendedCommunities` defines and the two keywords cannot spell, `redirect` among them, is reachable.
- `handleAnnounceFlowspec` now reads the parser's `consumed` count and refuses a leftover token.
- `TestModelDeclaresEveryComponentKeyword` compares the model's component set against the encoder's, in both directions, deriving each side from its own source.
- `TestAnnounceFlowspecUsageStatesTheComponents` pins the generated line, twice over two independently built trees.
- `TestFlowspecActionAcceptsACommunity` proves the community form and the two sugar spellings encode the same bytes, and that a leftover token announces nothing.

### Bugs Found/Fixed
- `handleAnnounceFlowspec` discarded the token count `route.ParseExtendedCommunities` answers. Harmless while the action was synthesized; reachable the moment `community <action>` passed the operator's own text. `announce flowspec destination 192.0.2.0/24 community redirect 65001 100 junk` announced the redirect and dropped `junk` in silence. Covered by `TestFlowspecActionAcceptsACommunity`, journal row in `plan/journal/validated-value-discarded-by-its-caller.md`.
- The AC-4 guard failed open in the direction it existed to check. `encoderComponentKeywords` was a hand-written list beside `isComponentKeyword`, so a keyword added to the encoder alone appeared in neither loop and passed. It now reads the `// FlowSpec component keywords.` const block from `plugin_encode_text.go`. Proven by mutation in three directions: model-only, encoder-only, and a const the switch refuses.
- Three rows of `internal/component/plugin/server/usage_model_test.go` asserted lines the model had stopped producing after the announce split (`274e8e013`) and a policy-module leaf rename. Journal row in `plan/journal/claim-outlives-the-evidence-it-cites.md`.
- The `modifierChildren` doc comment said a name breaks "a tie no module should produce". An augment produces nineteen of them.

### Documentation Updates
- `docs/architecture/bgp/on-demand-origination.md`: the paragraph saying flowspec keeps its own sentence is replaced by the two mechanisms a reader would otherwise meet as silent behavior (the `config false` drop and the `ModifierOrder` 0 sort), by the RFC 8955 Section 7 reading of the action, and by the obligation the generated line does not state. Five `<!-- source: -->` anchors added, naming `splitFlowspecArgs`, `modifierChildren`, the augment, `declaredContainerOrder` and `mergeYANGEntry`.
- `ai/PACKAGE-MAP.md` carries the new package, written by `./le repository generate`.
- `./le repository check`: 123 issues, none in a file this spec touches, and no stale source anchor among them.

### Deviations from Plan
| Planned | Delivered | Why |
|---------|-----------|-----|
| `TestAnnounceFlowspecUsageStatesTheComponents` in `internal/component/command/usage_test.go` | the same test in `internal/component/plugin/server/usage_model_test.go` | `internal/component/config/yang` imports `internal/component/command`, so `command` cannot load the model. The server package is the one that may import the composition root, which its own file header states |
| `TestModelDeclaresEveryComponentKeyword` in `.../flowspec/yang/` | `.../flowspec/component_parity_test.go` | it calls `isComponentKeyword`, which is unexported in package `flowspec` |
| `TestFlowspecActionAcceptsACommunity` covering AC-5 | delivered, plus cases in `TestSplitFlowspecArgs` | the split and the encode are separate producers and each owed a test |
| a `.ci` for `announce flowspec help` | NOT delivered | reported, not traded (`ai/rules/pre-release.md`). See Goal Validation |
| `plan/learned/NNN-<name>.md` (the spec's own Closure checklist) | two journal rows | the learned summary was replaced by the journal row (owner directive, 2026-08-10) |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-3's Basis said `declaredContainerOrder` "reads the declaring module's statement order" | It reads the PARENT's own statement substatements (`internal/component/config/yang/command.go`), so an augmented child is never found and takes 0 | reading the producer while filling the Assumptions table | The assumption's claim holds and its reason did not. `modifierChildren` breaks the tie on the name, which is a total order, so the line is stable. Recorded on the design page, in `modifierChildren`'s comment, and in Known Limitations |
| approach | the AC-4 guard was written as a list beside the producer it guards | a list is a third copy, and it fails open exactly where the guard is needed | the closure review, by asking what a keyword added to `isComponentKeyword` alone would do | the set is read from the const block; three mutations prove it discriminates |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `announce flowspec` renders its whole grammar from the model | Done | `internal/component/bgp/plugins/nlri/flowspec/yang/ze-flowspec-cmd.yang`, `.../cmd/announce/yang/ze-cli-announce-cmd.yang` | 24 groups on one line |
| its authored sentence is deleted | Done | `ze-cli-announce-cmd.yang`, `flowspec` description | `./le docvalid usage-contract` exits 0 |
| the component keyword set stays declared in exactly one place | Done | `isComponentKeyword` (`plugin_encode_text.go`) | the model is a copy and `TestModelDeclaresEveryComponentKeyword` derives both sides from their own source |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `ze-flowspec-cmd.yang`, one `container` per keyword under one `augment` | eighteen `repeat`, `rd` `once` because a Route Distinguisher is not an OR group |
| AC-2 | Done | `TestAnnounceFlowspecUsageStatesTheComponents` | goes red when one container loses `config false` |
| AC-3 | Done | `./le docvalid usage-contract`: 384 command nodes, 0 authored, 0 disagreements | |
| AC-4 | Done | `TestModelDeclaresEveryComponentKeyword` | mutation-proven in both directions |
| AC-5 | Done | `TestFlowspecActionAcceptsACommunity`, `TestSplitFlowspecArgs` | `rate-limit` and `discard` produce byte-identical attributes to their community spelling |
| AC-6 | Done | `TestHandleAnnounceFlowspec`, `TestSplitFlowspecArgs`, the announce package green under `-race` | no existing case edited |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestModelDeclaresEveryComponentKeyword` | Done | `internal/component/bgp/plugins/nlri/flowspec/component_parity_test.go` | file moved, see Deviations |
| `TestAnnounceFlowspecUsageStatesTheComponents` | Done | `internal/component/plugin/server/usage_model_test.go` | file moved, see Deviations |
| `TestFlowspecActionAcceptsACommunity` | Done | `internal/component/bgp/plugins/cmd/announce/announce_test.go` | |
| `TestModifierGroupsLeaveDispatchUntouched` | Done | `internal/component/plugin/server/usage_model_test.go` | flowspec case added: 0 own argument definitions, identical tail |
| `announce flowspec help` functional test | Skipped | - | no `.ci` covers any `announce` form today; see Goal Validation |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `.../cmd/announce/yang/ze-cli-announce-cmd.yang` | Done | sentence deleted, action and options declared |
| `.../cmd/announce/announce.go` | Done | `kwCommunity` branch, `trailingOptsAt`, the leftover-token guard |
| `internal/component/plugin/server/usage_model_test.go` | Done | wiring case, usage test, three stale rows repaired |
| a YANG module under `.../flowspec/yang/` with embed and register glue | Done | four files, glue written by `./le yang glue write` |
| a test asserting the model and `isComponentKeyword` agree | Done | reads both sides from source |

### Audit Summary
- **Total items:** 19
- **Done:** 18
- **Partial:** 0
- **Skipped:** 1 (the `announce flowspec help` functional test, reported rather than traded)
- **Changed:** 5 (Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| `announce flowspec` renders its whole grammar from the model | functional (model to rendered line) | `TestAnnounceFlowspecUsageStatesTheComponents` pins all 24 groups and re-renders from a second independently built tree. Removing `config false` from one container drops `[destination <prefix> ...]` and the test fails |
| the authored sentence is gone and nothing replaced it in prose | gate | `./le docvalid usage-contract` exits 0: "Every command states its grammar in the model", 384 command nodes, 0 authored |
| the keyword set has one producer | mutation | three mutations, each red: a container the encoder refuses, a keyword the model omits, a `kw` const the switch refuses |
| nothing an operator types changed | functional | `./internal/component/bgp/plugins/cmd/announce/...` green under `-race`; no existing case edited; `TestModifierGroupsLeaveDispatchUntouched` asserts the flowspec node carries zero own argument definitions and hands the handler the identical tail |
| an operator reaches the line at the CLI | NOT PROVEN | no `.ci` exercises `announce flowspec help`, and none exercises any `announce` form: `grep -rln 'announce unicast\|announce blackhole\|announce flowspec' test/` answers nothing. The gap predates this spec and covers the whole verb. `./le repository check` does not report it |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | done | The spec metadata carries `Deferral shard: -`. `ls plan/deferrals/flowspec-states-its-own-grammar.md` answers "No such file" |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/flowspec-states-its-own-grammar-0d49d3a4-3753-4eb2-86d9-cd63bdb9cafb.md` |
| `review check` | clean (11 code files, hashes match) |
| Rounds | 2 |
| Reviewer lenses used | wiring and registration, guard discrimination (mutation), removed-behavior audit over the deleted sentence, security and untrusted input over the new `community` tail, allocation, comment and documentation drift, Go style |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | the AC-4 guard could not go red when the ENCODER gained a keyword, which is the direction it exists to watch, and its comment claimed it could | `component_parity_test.go`, `encoderComponentKeywords` | the set is read from the `// FlowSpec component keywords.` const block; mutation-proven |
| 2 | ISSUE | the token count `route.ParseExtendedCommunities` answers was discarded, so the new `community` form silently dropped a leftover operator token | `handleAnnounceFlowspec` (`announce.go`) | `consumed < len(actionArgs)` answers `errFlowspecActionExtraTokens`; two subtests, both red without the guard |
| 3 | ISSUE | three rows asserted a generated line the model no longer produces, committed red | `usage_model_test.go` | re-derived from the model |
| 4 | ISSUE | `modifierChildren` said a name breaks "a tie no module should produce" while this change produces nineteen | `internal/component/command/usage.go` | comment states the augment case and names `declaredContainerOrder` |
| 5 | NOTE | UK spelling in the committed Go comment and design page | `announce.go`, `on-demand-origination.md` | `synthesised` to `synthesized`; `misspell` flags the Go one |
| 6 | NOTE | the generated line brackets a mandatory action | the model | no modifier states "one of these"; recorded on the design page and in Known Limitations |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/plugins/nlri/flowspec/yang/ze-flowspec-cmd.yang` | Yes | `git show a0aa4e029 --stat` lists it at 158 lines |
| `internal/component/bgp/plugins/nlri/flowspec/yang/embed.go`, `register.go`, `doc.go` | Yes | same diffstat; `embed.go` and `register.go` carry the generated header |
| `internal/component/bgp/plugins/nlri/flowspec/component_parity_test.go` | Yes | `go test -run TestModelDeclaresEveryComponentKeyword` compiles and passes |
| a `.ci` for `announce flowspec help` | No | `grep -rln 'announce flowspec' test/` answers nothing |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | nineteen groups under one augment | `TestModelDeclaresEveryComponentKeyword` PASS; `augmentedContainerNames` parses 19 and fails at zero |
| AC-2 | the line states components, action and options | `TestAnnounceFlowspecUsageStatesTheComponents` PASS; red with one `config false` removed |
| AC-3 | no authored sentence remains | `./le docvalid usage-contract` exit 0, "Authored usage sentences: 0" |
| AC-4 | the copy cannot drift | mutation add: "the model declares \"bogus-keyword\" and the encoder does not accept it"; mutation remove: "the encoder accepts \"dscp\" and the model does not declare it"; encoder-only mutation: "the encoder accepts \"probe-only\" and the model does not declare it" |
| AC-5 | community states an action, sugar unchanged | `TestFlowspecActionAcceptsACommunity` PASS, including byte equality between `rate-limit 9600` and `community traffic-rate 0 9600 bytes` |
| AC-6 | the corpus passes unchanged | `go test -race ./internal/component/bgp/plugins/cmd/announce/...` ok 3.359s |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `announce flowspec destination 192.0.2.0/24 protocol =6 destination-port =80 rate-limit 9600` | none | Yes, by `TestModifierGroupsLeaveDispatchUntouched`, which dispatches that exact string through the argument definitions `yang.PathToArgDefs` produces and asserts the handler reads the identical eight tokens |
| `announce flowspec help` | none | Partly. The model half is proven by `TestAnnounceFlowspecUsageStatesTheComponents` over the tree `yang.DefaultLoader` builds with the composition root imported. The CLI half is not covered by any test this spec added |
| the module reaches the daemon's tree at all | none | Yes, by `internal/component/plugin/all/all_ze_bgp.go` importing the package and `configyang.RegisterModule` in its `register.go`; the rendered line is the proof the augment landed |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | the rendered line carries all nineteen augmented components |
| A-2 | confirmed | `TestModifierGroupsLeaveDispatchUntouched` asserts `ownArgDefs` 0 for `announce flowspec` and an identical tail |
| A-3 | confirmed, basis wrong | `ModifierOrder` is defined and is 0, not the declaration order the Basis claimed: `declaredContainerOrder` counts the PARENT's containers. `modifierChildren` breaks the tie on the name, a total order, so the line is stable across independently built trees. Mistake Log carries the correction |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| architecture design: `docs/architecture/bgp/on-demand-origination.md` | `mergeYANGEntry` skips a child whose `Config` is not `TSFalse`; `declaredContainerOrder` returns 0 for a name absent from the parent's substatements | Yes, both read at the producer |
| CLI reference | `grep -rn 'announce' docs/guide/command-reference.md` finds one unrelated line about `resolve irr` | No update owed |
| feature list, config syntax, API/RPC, plugin SDK, wire format, comparison table | no wire-visible change, no config leaf, no RPC signature change: the wire method `ze-bgp:announce-flowspec` and its handler are untouched | No update owed |
| RFC status | RFC 8955 Section 7 is read, not newly implemented: `route.ParseExtendedCommunities` already encoded every action form | No update owed |
| doctor check | no new runtime dependency: one embedded YANG module | No update owed |

## Core Insight

A grammar looks inexpressible when the model is asked to state the SUGAR. `rate-limit <bps>|discard` is a mandatory alternation with one value-carrying branch, and no modifier says that. RFC 8955 says the thing underneath is one extended community, and the handler had agreed for a year by synthesizing `traffic-rate` arguments. Model the thing, keep the sugar beside it, and the alternation is gone. The second half is the same move: a command that borrows another plugin's vocabulary does not restate it, it lets the owner `augment` it in.
