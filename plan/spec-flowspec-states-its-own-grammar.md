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
