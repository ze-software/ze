# Spec: cli-tab-reveals-command-help

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | - |
| Phase | 6/6 |
| Deferral shard | `plan/deferrals/cli-tab-reveals-command-help.md` (or `-` if nothing deferred) |
| Handoff | - |
| Updated | 2026-09-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`spec-yang-short-and-long-command-help` closed on 2026-08-31. It gave every
command node a declared one-line summary and a declared long explanation, and
deleted the five truncation guesses that stood in for them. Six surfaces render
both halves today: the terminal binary's `--help`, the web admin command form,
the MCP tool schema, the OpenAPI document, the published website and the wiki
catalog.

The interactive CLI renders neither correctly, and it is the surface an operator
lives in over SSH.

Two defects, and the second is the reason the first is not a one-line fix.

**The long explanation is unreachable.** `MergeCommandPaths`
(`internal/component/command/node.go`) carries `Node.Help` into the CLI client's
tree, and the daemon answers `command help "<path>"` with a `long-help` key
(`internal/plugins/meta/cmd/help.go`, `commandHelp`). The value arrives and
nothing renders it: the `Model` reaches commands only through
`CommandModeCompleter` (`internal/component/cli/model.go`), whose methods are
`Complete` and `GhostText`, and whose `Completion` type
(`internal/component/cli/contract/contract.go`) carries Text, Description and
Type with no help field at all. The long form cannot cross that interface.

**The completion menu spends its width on prose.** `renderDropdownBox`
(`internal/component/cli/model_render.go`) draws each candidate as a name column
and a description column, and cuts the description to the width that remains.
That truncation is the last one left in Ze after the closed spec removed five
others, and it survived on the argument that a width clamp answers a display
constraint rather than guessing at an author's intent. The owner has removed the
argument: the menu shows the name alone, and the summary belongs on the message
line, following the selection.

The goal is that an operator reaches both halves without leaving the CLI, using
the key already under their finger.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/cli/error-surface.md` - the two-line message area
  → Constraint: the message area is TWO lines with different owners. `feedbackLine` carries `m.err` and `m.statusMessage`; `warningLine` carries `m.completionHint`, and `validationHintLine` and `idleInfoLine` compete for the same row in a declared losing order. The summary goes on the `warningLine` row, and this spec MUST NOT let it displace an error
  → Decision: verified correct against `messageLines` in `internal/component/cli/model_render.go`; it is the one page in this area that matches its code
- [ ] `docs/architecture/cli/command-completion.md` - the completion path
  → Constraint: its claim that `MergeCommandPaths` never overwrites a builtin's summary or long help is UNVERIFIED. The prior session read the doc comment on the function and not its body, and labelled the claim so. Read the body before relying on it
- [ ] `docs/guide/cli.md` - the operator-facing CLI page
  → Constraint: it carries one line saying Tab completion exists and says nothing about the `?` hint, the `help` overlay, or Tab's state machine. This spec adds a keys-and-what-they-reveal section, because a feature nobody can discover is a feature nobody uses
- [ ] `docs/architecture/testing/ci-format.md` - the `.et` and `.ci` expectation vocabulary
  → Constraint: `expect=` has no `hint` or `overlay` kind, so the behavior this spec adds cannot be asserted by any existing expectation. The vocabulary is part of the feature, not a follow-up

**Key insights:** (minimal context to resume after compaction)
- `handleTab` (`internal/component/cli/model_keys.go`) is STATELESS across presses. There is no last-key memory and no cycling counter beyond `m.selected`. "Completion already ran and had nothing to add" is not an observable state today: it is the bare fall-through when the completion list is empty. **This is the load-bearing unknown and it is now answered: the state must be built.**
- `m.completionHint` is cleared at NINE sites, including every text key and every other key. Any new level state must be reset at the same nine, or it will survive a keystroke that should have dismissed it.
- `renderHelpOverlay` (`model_render.go`) takes NO content argument. It holds two hardcoded keybinding strings, with no wrapping, no width clamp and no scroll. It cannot render an arbitrary explanation.
- The `m.showHelp` key arm (`model_keys.go`) SWALLOWS every key except Escape and Ctrl-C, so typing and Backspace never reach the input. The owner's dismissal semantics are the opposite of what exists.
- `newHeadlessCommandModel` (`internal/component/cli/testing/headless.go`) never sets a command completer, so operational-mode `.et` tests have no command tree to complete against.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/cli/model_keys.go` - `handleKeyMsg` dispatches; Tab arms in two places, one for an open dropdown and one general; `handleTab` reads `m.ghostText`, `m.showDropdown`, `m.completions`, `m.selected` and nothing else; the `?` arm writes `"<text>: <description>"` to `m.completionHint` for the selected candidate; the `m.showHelp` arm swallows all keys but Escape and Ctrl-C
- [ ] `internal/component/cli/model.go` - `updateCompletions` recomputes `completions`, `selected`, `ghostText`, `showDropdown`, `completionHint` on every keystroke; `CommandModeCompleter` is the Model's only route to command data
- [ ] `internal/component/cli/model_render.go` - `View` composes viewport, login warnings, message line 1, message line 2, prompt; then ONE overlay, dropdown winning over help. `renderDropdownBox` clamps `innerWidth` from the terminal and derives `cmdWidth` and `descWidth`; `placeOverlay` positions above the prompt
- [ ] `internal/component/command/completer.go` - `matchChildren` returns nil for a leaf with no children, dynamic children, value hints or arg defs. That nil is what "nothing left to add" means
- [ ] `internal/component/cli/contract/contract.go` - `Completion` is the type crossing into the Model, and it has no help field
- [ ] `internal/component/cli/completer_command.go` and `cmd/ze/hub/web_completer.go` - the two implementations of `CommandModeCompleter`; both change together or the interface lies
- [ ] `internal/component/cli/testing/expect.go`, `parser.go`, `input.go`, `headless.go` - the `.et` harness: Tab is drivable, nothing about a hint or an overlay is assertable, and the headless command model has no completer

**Behavior to preserve:**
- Tab with something left to complete completes, exactly as today. This spec adds a branch to the empty case and MUST NOT change the completing case.
- `feedbackLine` keeps priority for a real error. A summary MUST NOT hide an error the operator needs.
- Enter runs the command as typed, from every level.
- The config-editor mode of the CLI is untouched. Its completion is over config paths, and this feature is about command help.

**Behavior to change:**
- The completion menu row becomes the command name alone.
- The message line carries the SELECTED candidate's summary, following the selection.
- Tab on an exhausted completion reveals the long explanation in its own region.
- Escape descends one display level per press rather than dismissing everything.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A keypress in the interactive CLI: Tab, Escape, Enter, an arrow, or a text rune.

### Transformation Path
1. `handleKeyMsg` dispatches the key.
2. `handleTab` reads the completion state and decides: complete, or climb a level.
3. The level state selects what `View` composes.
4. A climb to the explanation asks `CommandModeCompleter` for the node's two texts, resolved from the typed path.
5. `View` renders the menu, the message line, and at most one overlay region.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Model ↔ command tree | `Explain` on `CommandModeCompleter`, implemented by `*CommandCompleter` and `*pluginCompleter` | Yes: `TestBothCompleterImplementationsAnswerHelp` runs one table over both |
| CLI client ↔ daemon | the client reads the tree it already merged. No round trip is added | Yes: `(*TreeCompleter).Explain` reads `Node.Help` from the in-process tree |
| Model ↔ screen | the summary is on message line 2 and the explanation is an overlay. The menu and the explanation are never on screen together | Yes: `TestSummaryDoesNotDisplaceAnError`, `TestExplanationOverlayKeepsTheFrame` |

### Integration Points
- `command.FindNode` - resolves a path to a node; exists, and the Model has no root to give it. That is the gap.
- `placeOverlay` and `renderDropdownBox`'s width clamp - the sizing and positioning to reuse rather than reinvent.
- The `?` arm's `"<text>: <description>"` hint - the nearest existing behavior to the summary-follows-selection half.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The Model reaches a command only through `CommandModeCompleter`. It holds no tree root and calls `command.FindNode` nowhere |
| No unintended coupling (components stay isolated) | Yes | `internal/component/command` gained `Explain` and learned nothing about the CLI. `cmd/ze/hub/web_completer.go` implements a different interface and is untouched |
| No duplicated functionality (extends existing, does not recreate) | Yes | `Complete` and `Explain` share `(*TreeCompleter).resolve`. `renderDropdownBox` and `renderExplanationBox` share `boxTopBorder`, `boxContentRow`, `boxBottomBorder`, `overlayInnerWidth` and `placeAbovePrompt`. The `run ` strip is declared once in `commandCompleterInput` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No wire path. The explanation is a declared string read from the tree, not copied per keystroke |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No command is added, removed or renamed. The two new Model fields are reveal state, added to `knownModelFields` in the same change, and the reveal level is derived from them rather than stored |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A level counter reset at the nine `completionHint` clear sites is enough to make a second Tab observable | `handleTab` and `updateCompletions` are the only writers of completion state | The state survives a keystroke that should dismiss it, and help appears over the wrong command | A test driving text-then-Tab and asserting no help | broken as written, and replaced by a stronger mechanism: the level is DERIVED from `explanation` and `showDropdown`, and all thirteen clear sites call one `dismissReveal`. `TestLevelResetsOnEveryCompletionClearSite` drives every key that reaches a site and goes red when the clear is dropped |
| A-2 | The Model can obtain a node's two texts without a daemon round trip, because `MergeCommandPaths` already merged them into the client tree | `internal/component/command/node.go`; the prior spec's Phase 3 handoff | Tab-for-help needs an RPC and becomes latency-visible | Read `MergeCommandPaths`'s BODY, which the completion doc's claim about it is not evidence for | confirmed: `internal/component/command/node.go:228` writes each help field only on the last path element and only when that field is empty, so the merged client tree carries both texts |
| A-3 | One overlay region is enough: the menu and the explanation never need to be on screen at once | `View` composites exactly one overlay today | Level 2 cannot show the menu and the explanation together, and the level model needs redesign | Render both at level 2 in a test and read the output | confirmed: `updateCompletions` (`model.go:767`) clears `showDropdown` whenever the completion list holds one entry or none, and the explanation is revealed only when it is empty |
| A-4 | Removing the description column frees enough width that no candidate name needs truncating | `renderDropdownBox` clamps `innerWidth` between 48 and 96 | A long command name is cut, replacing the truncation this spec removes with another | Measure the longest candidate name in the tree against the clamp | confirmed: the longest YANG node name in the repository is `advertise-interval-milliseconds` at 31 characters, against an inner width clamped at 48 minimum |
| A-5 | The `.et` harness can assert on a rendered region once `State` exposes one | in-package Go tests already read `m.View().Content` | The feature is only testable in Go tests, and no `.et` proves an operator reaches it | Write the expectation kind first, before the feature | confirmed: `checkHint` and `checkExplanation` (`internal/component/cli/testing/expect.go`) read `MessageHint()` and `Explanation()`, and `TestETFileAssertsHintAndExplanation` fails when either kind is unregistered |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A Tab pressed from habit on an already-unambiguous command flashes help mid-typing | The message line changes during ordinary fast typing | Require the second Tab to be CONSECUTIVE, with no other key between. Proposed to the owner as the default; not yet ruled on |
| R-2 | The summary displaces an error on the shared message row | An error disappears when the menu opens | `feedbackLine` and `warningLine` are different rows; assert an error survives a menu |
| R-3 | The explanation is taller than the terminal | A long `ze:help` renders off-screen or corrupts the frame | Decide scroll-or-clamp in design, not at render time. `renderDropdownBox` already solves the same problem for candidates |
| R-4 | Dismissal that lets keys through re-introduces the swallowing bug in reverse: a key both dismisses AND acts | Backspace deletes a character the operator wanted kept | State the contract per key in an AC, and test the exact sequence |
| R-5 | The two `CommandModeCompleter` implementations diverge | The web completer compiles but answers no help | The interface change breaks both at compile time; add a test over each |
| R-6 | This lands on the same files another session is editing | `git status` shows `internal/component/cli/` dirty | Announce on the session bus before starting; the CLI is a high-traffic tree |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The interactive CLI's input handling: a swallowed key, a stuck overlay, or a menu that hides an error. No wire encoding, no config resolution, no protocol behavior. An operator can always press Ctrl-C |
| How is it reverted? | Single commit revert. No schema change, no published artifact, no migration |
| Who else touches this path? | `internal/component/cli/` is shared ground. `cmd/ze/hub/web_completer.go` belongs to the web surface. Announce before starting |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Tab with completions remaining | → | `handleTab`'s completing branch | `TestTabStillCompletesWhenThereIsMoreToAdd` |
| Tab with the completion list empty | → | the new level climb in `handleTab` | `TestTabOnExhaustedCompletionRevealsTheExplanation` |
| Arrow through the menu | → | the summary write to the message row | `TestSelectionMovesTheSummaryOnTheMessageLine` |
| Escape at the explanation level | → | the level descent | `TestEscapeDescendsOneLevelPerPress` |
| A text rune while help shows | → | dismissal that still reaches the input | `TestTypingDismissesHelpAndReachesTheInput` |
| Enter while help shows | → | command execution unchanged | `TestEnterRunsTheCommandFromEveryLevel` |
| An `.et` file naming a hint expectation | → | the new expectation kind | `TestExpectHintKindIsAccepted` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Tab pressed when completion has something to add | Completes exactly as before this spec; no help appears |
| AC-2 | The completion menu is open | Each row shows the command name ALONE, with no description column and no truncation |
| AC-3 | The selection moves from one candidate to the next | The message line shows the newly selected command's declared summary, whole, and changes with each move |
| AC-4 | Tab pressed when the completion list is empty for the typed path | The command's declared long explanation appears in its own screen region |
| AC-5 | The explanation is showing and Escape is pressed | The explanation region goes; the menu and the selected summary remain |
| AC-6 | The menu is showing and Escape is pressed | The menu and summary go; plain typing resumes |
| AC-7 | Any text rune is typed at any level | The level is dismissed AND the rune reaches the input; no keystroke is swallowed |
| AC-8 | Enter is pressed at any level | The command as typed runs, unchanged by the level |
| AC-9 | An error is on the message area when the menu opens | The error stays visible; the summary does not displace it |
| AC-10 | A command declares no long explanation and Tab is pressed on it | Nothing is invented: the level does not climb, or it states plainly that none is declared |
| AC-11 | The explanation is longer than the terminal height | It is bounded the way the candidate list is bounded; no frame corruption |
| AC-12 | An `.et` test names the message line or the explanation region | The harness accepts the expectation and can fail on it |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Types a partial command, presses Tab, reads what each candidate does | keypress → `handleTab` → menu of names → summary on the message row | `TestSelectionMovesTheSummaryOnTheMessageLine` |
| 2 | Presses Tab again on the command they chose, reads the full explanation | keypress → level climb → node's `Help` → explanation region | `TestTabOnExhaustedCompletionRevealsTheExplanation` |
| 3 | Presses Escape twice and carries on typing | keypress → level descent → level descent → plain input | `TestEscapeDescendsOneLevelPerPress` |
| 4 | Reads the explanation, then just keeps typing the arguments | keypress → dismissal → the rune lands in the input | `TestTypingDismissesHelpAndReachesTheInput` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTabStillCompletesWhenThereIsMoreToAdd` | `internal/component/cli/model_keys_test.go` | AC-1 | |
| `TestTabOnExhaustedCompletionRevealsTheExplanation` | `internal/component/cli/model_keys_test.go` | AC-4 | |
| `TestDropdownRowIsTheCommandNameAlone` | `internal/component/cli/model_render_test.go` | AC-2 | |
| `TestSelectionMovesTheSummaryOnTheMessageLine` | `internal/component/cli/model_keys_test.go` | AC-3 | |
| `TestEscapeDescendsOneLevelPerPress` | `internal/component/cli/model_keys_test.go` | AC-5, AC-6 | |
| `TestTypingDismissesHelpAndReachesTheInput` | `internal/component/cli/model_keys_test.go` | AC-7 | |
| `TestEnterRunsTheCommandFromEveryLevel` | `internal/component/cli/model_keys_test.go` | AC-8 | |
| `TestSummaryDoesNotDisplaceAnError` | `internal/component/cli/model_render_test.go` | AC-9 | |
| `TestTabOnACommandWithNoExplanationInventsNothing` | `internal/component/cli/model_keys_test.go` | AC-10 | |
| `TestExplanationIsBoundedByTerminalHeight` | `internal/component/cli/model_render_test.go` | AC-11 | |
| `TestLevelResetsOnEveryCompletionClearSite` | `internal/component/cli/model_test.go` | A-1, all nine sites | |
| `TestBothCompleterImplementationsAnswerHelp` | `internal/component/cli/completer_command_test.go`, `cmd/ze/hub/web_completer_test.go` | R-5 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Display level | 0-2 | 2 | -1 (never below plain typing) | 3 (a third Tab must not climb further) |
| Explanation region height | 1..terminal height minus prompt | the available rows | 0 rows available | one row more than available |
| Candidate name width | 1..`innerWidth` clamp | the clamp | N/A | a name longer than the clamp |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `tab-reveals-summary-on-selection` | `test/editor/*.et` | An operator tabs and reads what each candidate does | |
| `tab-again-reveals-explanation` | `test/editor/*.et` | An operator asks for the full help without leaving the CLI | |
| `escape-descends-one-level` | `test/editor/*.et` | An operator gets back to typing | |

### Interop Tests (Scope: protocol)

N/A. No wire protocol and no protocol peer.

## Files to Modify
- `internal/component/cli/model.go` - the level field beside the completion state; the third method on `CommandModeCompleter`; the nine reset sites
- `internal/component/cli/model_keys.go` - `handleTab`'s empty-completion branch; the Escape descent; the dismissal arm that must stop swallowing keys
- `internal/component/cli/model_render.go` - `renderDropdownBox` loses its description column; the message row carries the selected summary; an explanation region that wraps, clamps and scrolls
- `internal/component/cli/contract/contract.go` - `Completion` gains the help the Model cannot otherwise see
- `internal/component/cli/completer_command.go` - the first implementation
- `cmd/ze/hub/web_completer.go` - the second, which must not diverge
- `internal/component/cli/testing/expect.go`, `parser.go`, `headless.go` - the expectation kinds, and a command tree in the headless model
- `docs/architecture/cli/error-surface.md` - the message row's new occupant
- `docs/architecture/cli/command-completion.md` - Tab's state machine, and the unverified `MergeCommandPaths` claim resolved either way
- `docs/guide/cli.md` - the keys and what each reveals
- `docs/architecture/testing/ci-format.md` - the new expectation vocabulary

## Files to Create
- `internal/component/cli/help_level.go` - the level state and its transitions, if it does not belong in `model.go`
- `test/editor/tab-reveals-summary-on-selection.et`
- `test/editor/tab-again-reveals-explanation.et`
- `test/editor/escape-descends-one-level.et`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | The data already exists; this spec only renders it |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | No | No command added, removed or renamed. Key handling only |
| CLI grammar (keyword before value) | N-A | No token changes |
| Editor autocomplete | Yes | `internal/component/cli/model_keys.go`, `model_render.go` - this IS the feature |
| Functional test for new RPC/API | Yes | Three `.et` files, and the expectation kinds that make them possible |
| Pipe completeness | N-A | No new command answer |
| Env var registration | N-A | None |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Prometheus counters/metrics | N-A | No observable runtime state |
| BGP family surface | N-A | No protocol surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | No | No config surface touched |
| 3 | CLI command added/changed? | Yes | `docs/guide/cli.md` - the keys and what each reveals |
| 4 | API/RPC added/changed? | No | `command help` already answers `long-help` |
| 5 | Plugin added/changed? | No | No plugin surface |
| 6 | Has a user guide page? | Yes | `docs/guide/cli.md` |
| 7 | Wire format changed? | No | None |
| 8 | Plugin SDK/protocol changed? | No | None |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No protocol behavior |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/ci-format.md` - the new expectation kinds |
| 11 | Affects daemon comparison? | No, on evidence | `docs/comparison.md` carries eleven daemon columns. A Ze `Yes` beside ten `Unclear` cells is a claim about the other daemons that this spec did not inspect, and the page's own honesty rule refuses it. Several CLI-driven daemons there publish inline `?` help, so a Ze-only row would be false as well |
| 12 | Internal architecture changed? | Yes | `docs/architecture/cli/error-surface.md`, `docs/architecture/cli/command-completion.md` |
| 13 | Route metadata keys added/changed? | N-A | None |
| 14 | Prometheus counters added/changed? | N-A | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None registered |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/spec-cli-tab-reveals-command-help.md` before the documentation phase and name every result |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/cli.md` and `docs/guide/config-editor.md` each carry one line on Tab completion; verify both against the new behavior |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - make the help text reachable and the level observable
   - Tests: `TestBothCompleterImplementationsAnswerHelp`, `TestLevelResetsOnEveryCompletionClearSite`
   - Files: `contract.go`, `model.go`, `completer_command.go`, `cmd/ze/hub/web_completer.go`
   - Verify: the Model can name a node's two texts for the typed path, and a level survives exactly as long as the completion state does. Both fail before the interface changes
2. **Phase: The test surface** - before any rendering, so the rendering is provable
   - Tests: `TestExpectHintKindIsAccepted`
   - Files: `internal/component/cli/testing/expect.go`, `parser.go`, `headless.go`, `docs/architecture/testing/ci-format.md`
   - Verify: an `.et` file can name the message row and the explanation region and can FAIL on them. The headless command model has a command tree
3. **Phase: The menu and the message line** - the smallest behavior change, and the one with an existing path to follow
   - Tests: `TestDropdownRowIsTheCommandNameAlone`, `TestSelectionMovesTheSummaryOnTheMessageLine`, `TestSummaryDoesNotDisplaceAnError`
   - Files: `model_render.go`, `model_keys.go`
   - Verify: the description column is gone, the last truncation in Ze with it, and the summary follows the selection
4. **Phase: The explanation region** - the genuinely new renderer
   - Tests: `TestTabOnExhaustedCompletionRevealsTheExplanation`, `TestExplanationIsBoundedByTerminalHeight`, `TestTabOnACommandWithNoExplanationInventsNothing`
   - Files: `model_render.go`, `model_keys.go`
   - Verify: an explanation wraps, is bounded by the terminal, and a command with none declared invents nothing
5. **Phase: The key contract** - the descent and the dismissal
   - Tests: `TestEscapeDescendsOneLevelPerPress`, `TestTypingDismissesHelpAndReachesTheInput`, `TestEnterRunsTheCommandFromEveryLevel`, `TestTabStillCompletesWhenThereIsMoreToAdd`
   - Files: `model_keys.go`
   - Verify: every key's contract holds at every level, and no keystroke is swallowed
6. **Phase: Functional tests and documentation**
   - Tests: the three `.et` files
   - Files: the documentation rows above, plus what `./le spec citation anchors` returns
   - Verify: an operator's key sequence is proven end to end, and no page describes the removed description column

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has an implementation at file:symbol, and AC-7's "reaches the input" half is asserted, not assumed |
| Feature completeness | All four user stories run as key sequences in an `.et`, not only in Go tests |
| Correctness | The level resets at all nine completion-clear sites, not the two that were convenient |
| Naming | The level's states are named for what is on screen, never `mode1`/`mode2` |
| Data flow | The explanation is read from the merged tree, not fetched per keystroke |
| Rule: `ai/rules/no-layering.md` | The description column is DELETED, not left behind a width check |
| Rule: `ai/rules/evidence.md` | `MergeCommandPaths`'s body is read, since the doc claim about it is a claim |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The description column is gone | `grep -n descWidth internal/component/cli/model_render.go` returns nothing |
| The Model can reach a node's help | the third `CommandModeCompleter` method has two implementations and a test over each |
| An `.et` can assert a hint | `expect=hint:` in a passing test file |
| No key is swallowed | `TestTypingDismissesHelpAndReachesTheInput` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Untrusted text on a terminal | A plugin declares a command's summary and explanation, and both now reach a TTY. Phase 4 of the closed spec bounds them at 256 and 4096 bytes and refuses C0 controls in the summary; confirm the explanation's newline allowance cannot emit an escape sequence that moves the cursor or clears the screen |
| Resource exhaustion per reply | The explanation region's size derives from the declared text, whose length a plugin chooses. The closed spec left the per-reply question open where the per-declaration bound was the answer; do not repeat that. Bound what is RENDERED, not only what is declared |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| A key contract cannot hold at some level | STOP: the level model is wrong, not the key. Back to DESIGN |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The feature's cost is not the rendering, it is that `handleTab` remembers nothing between presses and the Model cannot reach a command node at all. Both are prerequisites that produce no visible behavior, which is why they are Phase 1 and why an estimate based on "show some text" would be wrong.
- The test surface is a prerequisite too, not a follow-up. The `.et` harness can DRIVE Tab and cannot ASSERT on anything this feature renders, so building the feature first would mean proving it only in Go tests, which do not prove an operator reaches it.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Tab is the trigger | `?` and `??`, the owner's first proposal | Tab removes the question of whether `?` is a valid token for the command being typed, and it is the key already under the operator's finger. The owner replaced his own proposal with it |
| Two levels, not three | A separate Tab step for the summary | Once the menu shows the selected candidate's summary on the message line, a Tab that climbs to "show the summary" renders what is already on screen. The owner's before-and-after collapsed the level: the menu shows the name alone, the summary follows the selection |
| Escape descends ONE level | Escape dismisses everything | The owner's explicit correction. Returning to the level you were at means two presses to reach a bare prompt, which is the cost of not losing the menu when you only wanted the explanation gone |
| The menu row is the name alone | Keep a narrower description column | The column is what forces a width truncation, and it duplicates what the message line now shows. Removing it deletes the last truncation left in Ze after the closed spec removed five |
| `contract.Completion` gains no help field | Carry the long explanation on every candidate | The explanation is needed exactly when the completion list is EMPTY, so no candidate exists to carry it. The field would have no reader, and the summary a candidate DOES need is already `Description`. The spec's Files to Modify named the field; its Deliverables row named the method, and the method is what the ACs require |
| Enter is unchanged at every level | Make Enter run the command from the menu too | AC-8 reads two ways. Literally, Enter at the menu would run the typed command; but Enter is the menu's only accept key, so that reading deletes the ability to choose a candidate, which no AC asks for. The reading taken is "unchanged by the level": the level this spec adds changes Enter nowhere. `TestEnterRunsTheCommandFromEveryLevel` asserts the accept and the run |

## Known Limitations

- The config-editor mode is out of scope. Its completion is over config paths, and a config leaf has no long form until `plan/future/spec-yang-config-leaf-short-and-long-help.md` runs.
- `registry.ListRoot()`'s 21 root-command summaries are read by no gate, so a root command reached by Tab may show a summary nothing holds to the one-line contract. Recorded in `plan/journal/gate-excludes-part-of-its-population.md` by the closed spec; not this spec's to fix.

## RFC Documentation (Scope: protocol)

N/A. This spec implements no protocol requirement.

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
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

## Implementation Summary

### What Was Implemented
- `Explain(input string) (string, bool)` on `CommandModeCompleter` (`internal/component/cli/model.go`). Two implementations answer it: `(*CommandCompleter).Explain` (`completer_command.go`), which delegates to the new `(*TreeCompleter).Explain` (`internal/component/command/completer.go`), and `(*pluginCompleter).Explain` (`completer_plugin.go`). `Complete` and `Explain` share one walk, `(*TreeCompleter).resolve`.
- The reveal state, derived and never stored. `revealLevel` (`internal/component/cli/model_help_level.go`) reads `explanation` and `showDropdown`. `dismissReveal` is the one clear, and it replaced thirteen hand-written clear pairs.
- The menu row is the command name alone. `renderDropdownBox` (`model_render.go`) lost `cmdWidth`, `descWidth` and the description column.
- The selected candidate's summary is DERIVED on message line 2 by `warningText` (`model_render.go`). No key handler writes it.
- `revealExplanation` (`model_keys.go`) fires on the exhausted completion list. `renderExplanationBox` and `wrapForBox` (`model_render.go`) draw the text, sanitized and bounded by the rows the terminal has.
- Escape descends one level. The new arm in `handleKeyMsg` sits above the arm that clears the whole input.
- The `.et` harness gained `hint` and `explanation`. `checkHint` and `checkExplanation` (`internal/component/cli/testing/expect.go`) read `MessageHint()` and `Explanation()`. `headlessCommandTree` (`testing/headless.go`) builds the real YANG command tree and refuses an empty one.

### Bugs Found/Fixed
- `MessageHint` sanitized the row it READ and `warningLine` did not sanitize the row it RENDERED. Fixed in phase 3. `TestSummaryStripsATerminalEscape` holds it. Row in `plan/journal/guard-added-to-one-half-of-a-pair.md`.
- Message line 2 drew a SECOND screen row for a declared summary carrying a newline. Found at this review gate and fixed here by `oneRow` (`model_render.go`). `TestSummaryWithANewlineStaysOnOneRow` and `TestHintWithANewlineStaysOnOneRow` hold it, and both were observed RED. Row in `plan/journal/gate-excludes-part-of-its-population.md`.
- Two unrelated finds carry their own rows and no fix here: `plan/journal/bulk-rename-corruption.md` (the `command list` rename) and `plan/journal/pointer-shared-across-the-names-it-indexes.md`.

### Documentation Updates
- `docs/architecture/cli/command-completion.md` -- new "What Tab reveals": the three levels, the key table per level, the one input rule, the render bounds. Five source anchors.
- `docs/architecture/cli/error-surface.md` -- new "The message area is two rows": the owner of each row, line 2's four occupants in order, and the one-row bound. The fault row said line 2 and `feedbackLine` puts `m.err` on line 1. Repaired. Anchor names `messageLines, feedbackLine, warningText, oneRow`.
- `docs/architecture/testing/ci-format.md` -- four expectation rows, what each kind reads, and the command tree an `option=mode:value=command` test completes against.
- `docs/guide/cli.md` -- new "Keys that reveal help", an eleven-row key table and the Escape descent. Three anchors.
- `docs/guide/config-editor.md` -- the config menu row, and Tab on a complete config path. The section carried no anchor and gained one.
- `docs/guide/command-reference.md` -- one pointer row to the keys table.
- `docs/features.md` -- the Self-Documenting System row names `ze cli` as a surface that renders both halves.
- `docs/architecture/cli/color-system.md` -- the migration list dropped the description column it named.
- `docs/architecture/api/commands.md` -- three claims repaired: the completion pane no longer reads `Description`, no surface cuts a summary, and `Model.warningText` owns the interactive message line.
- `docs/architecture/config/yang-config-design.md` -- the "Read by" cells for `description` and `ze:help`.
- `./le doc check verify` fails on the pre-existing command-metadata drift and the published `gh-pages` surfaces. It names no file this spec touched.
- `./le docs-to-code check`: up to date, 284 design docs.

### Deviations from Plan
- `contract.Completion` gains NO help field. The spec's Files to Modify named one. The explanation is reached when the completion list is EMPTY, so no candidate exists to carry it, and the summary a candidate needs is already `Description`. The spec's Deliverables row asked for the METHOD, which is what was built.
- R-5 named `cmd/ze/hub/web_completer.go` as the second implementation of `CommandModeCompleter`. It implements `zeweb.CommandCompleter`, which is `Complete` alone. The second implementation is `*pluginCompleter`. `web_completer.go` is unchanged.
- The new file is `internal/component/cli/model_help_level.go`, not `help_level.go`. `.golangci.yml` excludes `hugeParam` for `internal/component/cli/model[^/]*\.go`, and `(Model).revealLevel()` needs the value receiver `tea.Model` forces.
- The spec counted nine `completionHint` clear sites. There are thirteen.
- The three `.et` files went to `test/editor/completion/`, beside the other completion tests. The spec named `test/editor/*.et`.
- Enter is unchanged at the menu. AC-8 reads two ways, and the reading taken is "unchanged by the level". See Key Design Decisions.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1 proposed a level counter reset at nine clear sites | There are thirteen sites, and a counter beside two flags is a second declaration of one fact | The design pass counted the sites and read `updateCompletions` | The level is DERIVED by `revealLevel`, and `dismissReveal` is the one clear. `TestLevelResetsOnEveryCompletionClearSite` drives every site |
| approach | R-5 named `cmd/ze/hub/web_completer.go` as the second implementation to change | It implements a different interface, `zeweb.CommandCompleter` | Phase 1 read `internal/component/web/cli.go` before editing | `*pluginCompleter` answers `Explain`. `web_completer.go` is untouched |
| escalation | A code comment cited `./le docvalid help-shape` as the bound that keeps message line 2 to one row | The gate reads declarations from SOURCE. A plugin declares its summary over the wire, and that text reaches the same row | This review gate traced the claim to `helpShapeContract` | `oneRow` applies the bound where the text is drawn. Row in `plan/journal/gate-excludes-part-of-its-population.md` |
| approach | Commit a44e5f631 carried a foreign hunk in `docs/architecture/testing/ci-format.md` | The "How an `exec=` value becomes argv" section documents `03f568f969`, committed 30 minutes later | This closure read the doc diff and dated both commits | Nothing to repair. The text is true at HEAD and its referenced files are tracked. Named here so the next reader does not attribute it to this spec |
| approach | Two of this spec's documentation edits are not in commit a44e5f631 | `docs/architecture/api/commands.md` and `docs/architecture/config/yang-config-design.md` landed in `dc09235f36`, the next commit, which the owner ordered separately | This closure compared the phase handoffs with `git show --stat a44e5f631` | Nothing to repair. Both pages carry the correct claims at HEAD, verified by `grep`. The two rows here and above are one shape: concurrent sessions selected files from one shared working tree, and each commit took a hunk the other owned |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The long explanation is reachable from the Model | Done | `internal/component/command/completer.go` -- `(*TreeCompleter).Explain` | Read from the merged client tree. No daemon round trip |
| The completion menu spends no width on prose | Done | `internal/component/cli/model_render.go` -- `renderDropdownBox` | `grep -n descWidth` returns nothing |
| An operator reaches both halves with the key under their finger | Done | `internal/component/cli/model_keys.go` -- `handleTab`, `revealExplanation` | Three `.et` files drive the key sequences |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestTabStillCompletesWhenThereIsMoreToAdd` | `revealExplanation` is the last fall-through in `handleTab`, after every completing branch |
| AC-2 | Done | `TestDropdownRowIsTheCommandNameAlone` | The row is `prefix + Text`, clamped by `boxContentRow` so the frame closes |
| AC-3 | Done | `TestSelectionMovesTheSummaryOnTheMessageLine` | `warningText` READS `m.completions[m.selected].Description` |
| AC-4 | Done | `TestTabOnExhaustedCompletionRevealsTheExplanation` | `revealExplanation` writes `explanation` and `explanationSubject` |
| AC-5 | Done | `TestEscapeDescendsOneLevelPerPress/from the explanation` | The new arm in `handleKeyMsg` runs before the arm that clears the input |
| AC-6 | Done | `TestEscapeDescendsOneLevelPerPress/from the menu` | The dropdown block's `tea.KeyEscape` case |
| AC-7 | Done | `TestTypingDismissesHelpAndReachesTheInput` | The text arm and the fall-through arm both call `dismissReveal` and then reach the input |
| AC-8 | Changed | `TestEnterRunsTheCommandFromEveryLevel` | At the menu, Enter accepts the candidate and the next press runs it. See Key Design Decisions |
| AC-9 | Done | `TestSummaryDoesNotDisplaceAnError` | `feedbackLine` owns line 1 and `warningText` owns line 2 |
| AC-10 | Done | `TestTabOnACommandWithNoExplanationInventsNothing` | `Explain` answers false, and line 2 reads `<command>: no explanation is declared` |
| AC-11 | Done | `TestExplanationIsBoundedByTerminalHeight`, `TestWrapForBoxReadsNoMoreRowsThanAsked` | `renderExplanationBox` bounds the frame and `wrapForBox` bounds the work |
| AC-12 | Done | `TestETFileAssertsHintAndExplanation` | Both kinds parse from an `.et` file and fail on a mismatch |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestTabStillCompletesWhenThereIsMoreToAdd` | Done | `model_keys_test.go:97` | |
| `TestTabOnExhaustedCompletionRevealsTheExplanation` | Done | `model_keys_test.go:56` | |
| `TestDropdownRowIsTheCommandNameAlone` | Done | `model_render_test.go:912` | |
| `TestSelectionMovesTheSummaryOnTheMessageLine` | Done | `model_render_test.go:953` | The spec named `model_keys_test.go`. The subject is a render function |
| `TestEscapeDescendsOneLevelPerPress` | Done | `model_keys_test.go:217` | |
| `TestTypingDismissesHelpAndReachesTheInput` | Done | `model_keys_test.go:275` | |
| `TestEnterRunsTheCommandFromEveryLevel` | Done | `model_keys_test.go:352` | |
| `TestSummaryDoesNotDisplaceAnError` | Done | `model_render_test.go:994` | |
| `TestTabOnACommandWithNoExplanationInventsNothing` | Done | `model_keys_test.go:121` | |
| `TestExplanationIsBoundedByTerminalHeight` | Done | `model_render_test.go:1077` | |
| `TestLevelResetsOnEveryCompletionClearSite` | Done | `model_test.go:1533` | Thirteen sites, not the nine the spec counted |
| `TestBothCompleterImplementationsAnswerHelp` | Done | `completer_command_test.go:147` | One table over both implementations. `cmd/ze/hub/web_completer_test.go` is not one of them |
| `TestExpectHintKindIsAccepted` | Done | `testing/expect_test.go:389` | |
| `tab-reveals-summary-on-selection` | Done | `test/editor/completion/` | |
| `tab-again-reveals-explanation` | Done | `test/editor/completion/` | |
| `escape-descends-one-level` | Done | `test/editor/completion/` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/cli/model.go` | Done | `Explain` on the interface, two new fields, `commandCompleterInput` |
| `internal/component/cli/model_keys.go` | Done | The reveal, the Escape descent, thirteen sites behind `dismissReveal` |
| `internal/component/cli/model_render.go` | Done | The column removed, the summary row, the explanation region, `oneRow` |
| `internal/component/cli/contract/contract.go` | Changed | No field added. See Deviations |
| `internal/component/cli/completer_command.go` | Done | |
| `cmd/ze/hub/web_completer.go` | Changed | Untouched. It implements a different interface. See Deviations |
| `internal/component/cli/testing/expect.go`, `parser.go`, `headless.go` | Changed | `parser.go` needed no edit. `parseExpect` reads the type without a list |
| `internal/component/cli/model_help_level.go` | Done | Named `model_help_level.go`. See Deviations |
| The three `.et` files | Done | Under `test/editor/completion/` |
| The five documentation pages | Done | Ten pages in the end. See Documentation Updates |

### Audit Summary
- **Total items:** 41 (3 requirements, 12 ACs, 16 tests, 10 file groups)
- **Done:** 37
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4 (AC-8, `contract.go`, `web_completer.go`, the harness file set), each recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An operator reads what each candidate does without leaving the CLI | functional | `test/editor/completion/tab-reveals-summary-on-selection.et`. Three `hint:contains` rows quote the text `ze-diag-cmd.yang`, `ze-traceroute-cmd.yang` and the traffic `-cmd` modules declare. RED when the selection branch is removed from `warningText` |
| An operator reads the long explanation without leaving the CLI | functional | `test/editor/completion/tab-again-reveals-explanation.et`. Two `explanation:contains` rows quote the `ze:help` of `show uptime`. RED when `revealExplanation` is removed from `handleTab` |
| An operator returns to typing without losing the words they typed | functional | `test/editor/completion/escape-descends-one-level.et`. The final `input:value` row is the AC-5 claim, and it names the whole typed command. RED when the Escape arm is removed, where an explanation-only assertion stays green |
| The last width truncation in Ze is gone | deliverable | `grep -n descWidth internal/component/cli/model_render.go` returns nothing. `TestDropdownRowIsTheCommandNameAlone` asserts no description fragment reaches the box |
| Declared text reaching a TTY is bounded at the render boundary | unit | `TestSummaryStripsATerminalEscape`, `TestExplanationStripsATerminalEscape`, `TestSummaryWithANewlineStaysOnOneRow`, `TestHintWithANewlineStaysOnOneRow`, `TestWrapForBoxReadsNoMoreRowsThanAsked`. Each was observed RED against the unbounded code |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none | done | No shard exists. `ls plan/deferrals/cli-tab-reveals-command-help.md` reports no such file, and nothing was deferred. No `remove` is prepared for a shard that was never created |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/cli-tab-reveals-command-help-45c02a83-014d-4cda-97b1-0f71acc09e91.md` |
| `./le spec session review check` | OK, clean, hashes match, exit 0 |
| Rounds | 2 |
| Reviewer lenses used | wiring and dead code, removed-behavior audit over the `Complete` refactor, logic and guard audit, security over plugin-declared text on a TTY, documentation drift, simplicity, the ze-go-style pass |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The diff asserted a safety property it did not prove. A comment named `./le docvalid help-shape` as the bound that keeps message line 2 to one row. That gate reads declarations from source. A plugin declares its summary over the wire, `MergeCommandPaths` writes it into the command tree, and `sanitizeForDisplay` keeps the newline by design. The row then drew a second screen line, and `View` put the cursor one row above the prompt | `internal/component/cli/model_render.go` -- `warningText` | `oneRow` strips the escape bytes and folds every whitespace run into a single space. Both text rows on line 2 read it. `TestSummaryWithANewlineStaysOnOneRow` and `TestHintWithANewlineStaysOnOneRow` were observed RED against `sanitizeForDisplay` alone. `docs/architecture/cli/error-surface.md` states the bound, and `plan/journal/gate-excludes-part-of-its-population.md` carries the class |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/cli/model_help_level.go` | Yes | `ls -la` reports 2.2K, 2026-09-02 |
| `test/editor/completion/tab-reveals-summary-on-selection.et` | Yes | `ls -la` reports 1.2K |
| `test/editor/completion/tab-again-reveals-explanation.et` | Yes | `ls -la` reports 1.2K |
| `test/editor/completion/escape-descends-one-level.et` | Yes | `ls -la` reports 852 bytes |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Tab still completes | `sed -n '265,315p' model_keys.go`: `revealExplanation` is the last statement of `handleTab`, under every completing branch |
| AC-2 | The row is the name alone | `grep -n descWidth internal/component/cli/model_render.go` returns nothing, exit 1 |
| AC-3 | The summary follows the selection | `warningText` returns `oneRow(m.completions[m.selected].Description)`, read from source |
| AC-4 | Tab reveals the explanation | `handleTab` calls `m.revealExplanation()`, which writes `m.explanation` from `commandCompleter.Explain` |
| AC-5 | Escape clears the reveal alone | `handleKeyMsg` holds `if keyStr == keyEsc && m.revealLevel() == revealExplanation` above the dropdown block |
| AC-6 | Escape closes the menu | The dropdown block's `tea.KeyEscape` case sets `showDropdown = false` and calls `dismissReveal` |
| AC-7 | No key is swallowed | The `key.Text != ""` arm and the fall-through arm both call `dismissReveal` and then `m.textInput.Update(msg)` |
| AC-8 | Enter is unchanged by the level | `handleEnter`'s dropdown block is unchanged, and the Enter arm calls `dismissReveal` before it |
| AC-9 | An error survives | `feedbackLine` renders `m.err`. `warningText` reads neither `m.err` nor `m.statusMessage` |
| AC-10 | Nothing is invented | `revealExplanation` writes `completionHint` on a false answer and leaves `explanation` empty |
| AC-11 | The region is bounded | `renderExplanationBox` derives `rows` from `availableHeight` and `wrapForBox` reads `rows+1` lines at most |
| AC-12 | The harness can fail on it | `validExpectationTypes` (`testing/expect.go`) holds `"hint"` and `"explanation"` |
| all | The suite is green | `./le test-unit cli` exit 0: cli 114.6s, client 5.9s, cli/testing 304.2s |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Tab with completions remaining | `test/editor/completion/tab-again-reveals-explanation.et` | Yes. The first Tab completes `show upt` to `show uptime ` and asserts `explanation:empty` |
| Tab with the completion list empty | `test/editor/completion/tab-again-reveals-explanation.et` | Yes. The second Tab asserts two `explanation:contains` rows |
| Arrow through the menu | `test/editor/completion/tab-reveals-summary-on-selection.et` | Yes. Two `down` presses, three `hint:contains` rows |
| Escape at the explanation level | `test/editor/completion/escape-descends-one-level.et` | Yes. The `input:value` row proves the typed words survived |
| A text rune while help shows | `test/editor/completion/tab-again-reveals-explanation.et` | Yes. A typed pipe operator dismisses the reveal and reaches the input |
| Enter while help shows | `TestEnterRunsTheCommandFromEveryLevel` | Go test. The `.et` harness runs no command in a headless command model |
| An `.et` file naming a hint expectation | `TestETFileAssertsHintAndExplanation` | Yes. It parses `.et` text and runs it, in both directions |
| `Explain` reaches a user | The three `.et` files | Yes. `./le repository check` reports no unwired export in `internal/component/cli` or `internal/component/command` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | A counter is not needed. The level is derived by `revealLevel`, and thirteen sites call one `dismissReveal`. `TestLevelResetsOnEveryCompletionClearSite` drives every site |
| A-2 | confirmed | `MergeCommandPaths` (`internal/component/command/node.go`) writes each help field on the last path element alone, and only when it is empty. The merged client tree carries both texts |
| A-3 | confirmed | `updateCompletions` (`model.go`) calls `dismissReveal` and hides the menu whenever the list holds one entry or none. That is the state the reveal fires in |
| A-4 | confirmed | The longest YANG node name in the repository is `advertise-interval-milliseconds`, 31 runes, against an inner width clamped at 48 or more by `overlayInnerWidth` |
| A-5 | confirmed | `checkHint` and `checkExplanation` read `MessageHint()` and `Explanation()`. `TestETFileAssertsHintAndExplanation` fails when either kind is unregistered |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/cli.md` key table | Every row read against `handleKeyMsg`, `handleTab` and `handleEnter`. The `?` row matches the `key.Text == "?"` arm, which writes `<name>: <summary>` for the selected candidate | Yes |
| `docs/architecture/cli/error-surface.md` row owners | `messageLines` returns `feedbackLine` then `warningLine`. `m.err` renders in `feedbackLine` alone | Yes |
| `docs/architecture/cli/error-surface.md` one-row bound | `oneRow` (`model_render.go`), added at this review gate | Yes |
| `docs/architecture/cli/command-completion.md` level table | `revealLevel` reads `explanation` then `showDropdown`. `View` composes dropdown, then explanation, then `showHelp` | Yes |
| `docs/architecture/testing/ci-format.md` expectation rows | `validExpectationTypes` holds both kinds. `checkHint` and `checkExplanation` accept `empty` and `contains` and refuse an expectation naming neither | Yes |
| `docs/features.md` Self-Documenting System row | The anchor names `handleTab, revealExplanation`, and both exist in `model_keys.go` | Yes |
| Row 11, `docs/comparison.md` | No row written. The API and Programmability table carries eleven daemon columns, and this spec inspected none of them. A Ze `Yes` beside ten `Unclear` cells is a claim about the other daemons | Yes, No on evidence |
| `./le docs-to-code check` | up to date, 284 design docs, exit 0 | Yes |
| `./le doc check verify` | Fails on the pre-existing command-metadata drift and the published `gh-pages` surfaces. `grep` over its output names no file this spec touched | Pre-existing |
| `./le spec citation` | One dangling reference, in `plan/spec-rfcgate-6-supported-extraction-signoff.md`. No citer names `plan/spec-cli-tab-reveals-command-help.md` outside the spec itself | Yes |

## Core Insight

A gate's NAME says what it checks. It never says which population it reads. `./le docvalid help-shape` refuses a newline in a command summary, and a reader takes that as the bound on every summary. It reads the declarations this tree holds. A plugin declares its summary over the wire and reaches the same screen row. The bound belongs where the text is DRAWN, because that is the one place every population passes through.
