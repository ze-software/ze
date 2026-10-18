# Spec: cli-question-mark-cannot-be-typed

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-18 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later session reviews that commit and closes, on Opus when the model is an Anthropic one. `-` closes in the same session. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The CLI editor never lets an operator type a literal `?`. Any value that carries
one, a URL with a query string being the common case, cannot be entered at the
prompt, even inside double quotes.

Found during an external defect-class review on 2026-10-03. CONFIRMED by reading the producer; not reproduced in a terminal.

`(Model).handleKeyMsg` in `internal/component/cli/model_keys.go` dispatches on
`key.Text == "?"` before the generic `key.Text != ""` arm that forwards a rune to
the text input. The `?` arm reveals the explanation of the selected completion
candidate when there is one, and otherwise calls `handleTab`. Neither path
inserts the character, and the arm reads neither the input value nor the cursor
position, so an open quote makes no difference: inside a quoted value `?` still
opens help or completes. Whether a bracketed paste carrying `?` reaches the
input by another route has not been checked.

The goal is that `?` inserts itself when the cursor is inside an open quoted
value, and keeps its help meaning everywhere else. The design phase settles what
"inside a quote" means for the CLI tokenizer (an escaped quote, an unterminated
quote at the end of the line) and whether an escape for an unquoted `?` is also
owed. The proof owed is a model test that types a quoted URL with a query
string and reads it back from the input value, and a test that `?` outside a
quote still reveals help.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/cli/command-completion.md` - what Tab and `?` reveal, and the statement that `?` never reaches the input
  → Decision: `?` stays bound to help everywhere outside an open quote; the reveal state machine is unchanged
  → Constraint: the page says "`?` ... does not reach the input"; that sentence becomes wrong and is corrected in the same change
- [ ] `docs/guide/cli.md` "Keys that reveal help" - the operator key table
  → Constraint: a new row names what `?` does inside a quoted value

**Key insights:**
- The CLI tokenizer (`tokenizeCommand`) has one quote rule: every `"` toggles quoted state, a backslash is not an escape, and a single quote is an ordinary character.
- Bracketed paste arrives as `tea.PasteMsg`, which `Model.Update` forwards to the text input without passing through `handleKeyMsg`, so a pasted `?` already reaches the input.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cli/model_keys.go` - `handleKeyMsg` dispatches `key.Text == "?"` before the generic text arm; that arm reveals the selected candidate's explanation or calls `handleTab`, and never inserts
- [ ] `internal/component/cli/model_commands.go` - `tokenizeCommand` / `handleQuoteChar`: a `"` toggles quoted state, no escape sequences
- [ ] `internal/component/cli/model.go` - `Update` forwards unhandled messages, `tea.PasteMsg` included, to `textInput.Update`

**Behavior to preserve:**
- `?` outside a quoted value: reveals the highlighted candidate's explanation, otherwise completes as Tab does
- Paste delivers its content whole

**Behavior to change:**
- `?` typed while the cursor sits inside an open double-quoted value is inserted as a character

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A key press from the terminal, `tea.KeyPressMsg` with `Text == "?"`

### Transformation Path
1. `Model.Update` hands the key to `handleKeyMsg`
2. The `?` arm reads the input up to the cursor; when a quote is open there, the key falls through to the text arm and is inserted as any rune is
3. Otherwise the existing reveal / Tab path runs

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None: the change stays inside the CLI model | - | Yes |

### Integration Points
- `tokenizeCommand` quote rule - the cursor test applies the same toggle rule, so what the editor treats as inside a quote is what the tokenizer later reads as one token

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the rune is inserted by the same `textInput.Update` call the generic text arm makes |
| No unintended coupling (components stay isolated) | Yes | only `internal/component/cli` changes |
| No duplicated functionality (extends existing, does not recreate) | Yes | the quote rule is stated once, in `quoteOpen` beside `handleQuoteChar` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | interactive key path, not a wire path |
| Registration over hardcoding, outbound | N-A | no command, view or handler is added |
| Registration over hardcoding, inbound | N-A | no name is introduced that a list must learn |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Double quote is the only quoting character and has no escape | `tokenizeCommand` comment and body | a `\"` or `'` case would need its own rule | reading `tokenizeCommand`, `handleQuoteChar` | confirmed |
| A-2 | Bracketed paste does not pass through `handleKeyMsg` | `Model.Update` has no `tea.PasteMsg` arm and forwards to `textInput.Update` | a pasted `?` would be swallowed | `TestPastedQuestionMarkReachesTheInput` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An operator who opens a quote loses `?` help until they close it | operator report | same trade VyOS made (T9077); Tab still completes inside the quote |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | `?` help inside a quoted value, or a `?` typed in one; nothing outside the interactive editor |
| How is it reverted? | single commit revert |
| Who else touches this path? | nobody in flight; `internal/component/cli` was clean at claim time |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| typed `?` key press | → | `handleKeyMsg` `?` arm, `quoteOpen` | `TestQuestionMarkInsideQuotesIsTyped`, `test/editor/completion/question-mark-inside-quotes.et` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | operator types `set bgp description "https://example.net/a?b=1&c=?"` key by key | the input holds exactly that line, and no help is revealed |
| AC-2 | input ends in an unterminated quote, `?` pressed | `?` is appended |
| AC-3 | cursor moved back between a closed pair of quotes, `?` pressed | `?` is inserted at the cursor |
| AC-4 | cursor before an opening quote, or after a closing one, `?` pressed | nothing is inserted; help or completion runs as before |
| AC-5 | a value carrying `?` is pasted | the paste arrives whole |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | types a quoted URL with a query string at the editor prompt | KeyPressMsg -> handleKeyMsg -> textInput | `TestQuestionMarkInsideQuotesIsTyped` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestQuestionMarkInsideQuotesIsTyped` | `internal/component/cli/model_keys_quote_test.go` | AC-1 | |
| `TestQuestionMarkInsideAnUnterminatedQuoteIsTyped` | same | AC-2 | |
| `TestQuestionMarkFollowsTheCursorNotTheLine` | same | AC-3, AC-4 | |
| `TestQuestionMarkOutsideQuotesStillRevealsHelp` | same | AC-4 control | |
| `TestPastedQuestionMarkReachesTheInput` | same | AC-5 | |
| `TestQuestionMarkCursorIsCountedInRunes` | same | Critical Review "cursor is a rune index" | added at closure |

### Boundary Tests (numeric inputs)
N-A: the feature takes no numeric input.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| model tests above | `internal/component/cli/` | the model tests drive `handleKeyMsg`, the function a terminal key press reaches | |
| `question-mark-inside-quotes.et` | `test/editor/completion/` | the `.ci` harness sends whole command lines, but the `.et` editor harness sends one `tea.KeyPressMsg` per rune (`Input.toTypeMessages` in `internal/component/cli/testing/input.go`) through `Model.Update`, so it reaches the `?` arm: the operator types the quoted URL key by key, commits it with Enter, and reads it back from the working config; the control types `?` outside a quote | added at closure |

### Interop Tests (Scope: protocol)
N-A: Scope is cli, no wire behavior.

## Files to Modify
- `internal/component/cli/model_keys.go` - the `?` arm falls through to text insertion inside an open quote
- `internal/component/cli/model_commands.go` - `quoteOpen` beside the tokenizer quote rule
- `docs/guide/cli.md`, `docs/architecture/cli/command-completion.md` - key table, and the "does not reach the input" sentence

## Files to Create
- `internal/component/cli/model_keys_quote_test.go` - model tests
- `test/editor/completion/question-mark-inside-quotes.et` - key-press functional test (added at closure)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no schema change |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | N-A | no command |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | N-A | completion unchanged |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | none |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | defect fix |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/cli.md` key table |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/cli/command-completion.md` `?` paragraph |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/guide/config-editor.md` describes `?` on a highlighted key, still true outside a quote; `docs/features/formatting.md`, `docs/guide/command-reference.md` and `docs/architecture/config/yang-config-design.md` do not describe the `?` key |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring** -- model tests drive `handleKeyMsg`; red at HEAD
2. **Phase: Fix** -- `quoteOpen`, the `?` arm guard, docs

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | the cursor position is a rune index, so the prefix is cut in runes, not bytes |
| Rule: one declaration | the quote rule is stated once, beside the tokenizer |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `?` inserted inside quotes | `./le test unit go -run QuestionMark ./internal/component/cli/` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | the inserted rune follows the path every other rune takes; the schema validates the value at set time as before |

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
- VyOS fixed the same defect (T9077, vyos-1x commits 1ec6e664a and 412dfb9e9) the same way: `?` inside a quoted value is literal, outside it is help.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Read the quote state of the input up to the cursor | parity over the whole line | the character lands at the cursor, so the cursor's context decides |
| No escape for an unquoted `?` | backslash escape, a literal-next key | the tokenizer has no escapes; quoting the value is the escape |
| Derive the state from the input with the tokenizer's rule (each `"` toggles) | track quote state per keystroke | state derived from the input cannot drift from it after Backspace, history recall or paste |

## Known Limitations
- Inside an open quote `?` no longer opens help; Tab still does.

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
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- `quoteOpen` (`internal/component/cli/model_commands.go`) reports an open double quote at the end of a prefix under the tokenizer's rule (every `"` toggles, no escape)
- `Model.cursorInQuote` (`internal/component/cli/model_keys.go`) cuts the input at the cursor, in runes, and asks `quoteOpen`
- The `?` arm of `Model.handleKeyMsg` (`internal/component/cli/model_keys.go`) is guarded by `!m.cursorInQuote()`, so inside a quote the key reaches the generic text arm and `textInput.Update` inserts it
- Implementation commit `b5d6305b93`; closure added `TestQuestionMarkCursorIsCountedInRunes` and `test/editor/completion/question-mark-inside-quotes.et`

### Bugs Found/Fixed
- The defect itself: `?` could not be typed inside a quoted value. Covered by `TestQuestionMarkInsideQuotesIsTyped` and `question-mark-inside-quotes.et`

### Documentation Updates
- `docs/guide/cli.md` "Keys that reveal help": new row for `?` inside a double-quoted value; source anchor gains `cursorInQuote` (commit `b5d6305b93`)
- `docs/architecture/cli/command-completion.md`: the "does not reach the input" sentence corrected, anchors `handleKeyMsg, cursorInQuote` and `quoteOpen, tokenizeCommand` (commit `b5d6305b93`)
- `./le doc check verify` is red on this tree for reasons outside these pages: `clear bgp rib in/out` command-surface arguments and missing `../gh-pages/reference/command-equivalents/*` pages. No finding names a file this spec changed

### Deviations from Plan
- The spec said no functional test could reach the key arm because the `.ci` harness sends whole lines. The `.et` editor harness sends one key press per rune, so closure added `question-mark-inside-quotes.et`
- Added `TestQuestionMarkCursorIsCountedInRunes` for the Critical Review check "the prefix is cut in runes, not bytes", which no planned test discriminated

## Mistake Log
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The spec stated no functional test could reach the `?` arm | `.et` tests under `test/editor/` deliver typed text as one `tea.KeyPressMsg` per rune (`Input.toTypeMessages`) | closure review read `internal/component/cli/testing/input.go` | `.et` test added; Functional Tests row corrected |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `?` inserts itself inside an open quoted value | Done | `Model.handleKeyMsg`, `Model.cursorInQuote`, `quoteOpen` | |
| `?` keeps its help meaning elsewhere | Done | `Model.handleKeyMsg`, `?` arm body unchanged | |
| Inside-a-quote follows the tokenizer rule | Done | `quoteOpen` applies `tokenizeCommand`'s `isQuote` toggle | |
| No escape for an unquoted `?` | Done | Key Design Decisions | quoting is the escape |
| Model test with a quoted URL; help test outside a quote | Done | `model_keys_quote_test.go` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestQuestionMarkInsideQuotesIsTyped`, `question-mark-inside-quotes.et` steps 1-6 | |
| AC-2 | Done | `TestQuestionMarkInsideAnUnterminatedQuoteIsTyped` | |
| AC-3 | Done | `TestQuestionMarkFollowsTheCursorNotTheLine/between_the_quotes` | |
| AC-4 | Done | `.../before_an_opening_quote`, `TestQuestionMarkOutsideQuotesStillRevealsHelp`, `.et` steps 7-9 | |
| AC-5 | Done | `TestPastedQuestionMarkReachesTheInput` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestQuestionMarkInsideQuotesIsTyped` | Done | `internal/component/cli/model_keys_quote_test.go` | |
| `TestQuestionMarkInsideAnUnterminatedQuoteIsTyped` | Done | same | |
| `TestQuestionMarkFollowsTheCursorNotTheLine` | Done | same | |
| `TestQuestionMarkOutsideQuotesStillRevealsHelp` | Done | same | |
| `TestPastedQuestionMarkReachesTheInput` | Done | same | |
| `TestQuestionMarkCursorIsCountedInRunes` | Done | same | added at closure |
| `question-mark-inside-quotes.et` | Done | `test/editor/completion/` | added at closure |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/cli/model_keys.go` | Done | |
| `internal/component/cli/model_commands.go` | Done | |
| `docs/guide/cli.md` | Done | |
| `docs/architecture/cli/command-completion.md` | Done | |
| `internal/component/cli/model_keys_quote_test.go` | Done | |
| `test/editor/completion/question-mark-inside-quotes.et` | Done | added at closure |

### Audit Summary
- **Total items:** 23
- **Done:** 23
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2, the closure tests (recorded in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An operator can type a quoted value carrying `?` | functional `.et` over the real key path | `question-mark-inside-quotes.et`: typed key by key, Enter, `content:contains=https://example.net/a?b=1&c=?` passes. With the guard defeated (the `?` arm always taken) step 2 went red, input `"https://example.net/ab=1&c="`; restored, green. Full editor suite 179/179 |
| `?` keeps its help meaning outside a quote | functional `.et` + model tests | `.et` steps 7-9 (`?` not typed, completion offers `description`); `TestQuestionMarkOutsideQuotesStillRevealsHelp` |
| The cursor's context decides, counted in runes | unit | `TestQuestionMarkCursorIsCountedInRunes` red under a byte cut (`"set bgp description \"é\"? "`), green restored |

## Work Not Done
| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every AC and Task goal is done | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/cli-question-mark-cannot-be-typed-20123ea5-a7dd-4d5e-aa9c-d2846361ee33.md` |
| `./le spec review check` | clean: `review_gate: OK (4 code files, clean, hashes match ...)` |
| Rounds | 2: round 1 found the two ISSUEs below, round 2 over the added tests found 0 BLOCKER, 0 ISSUE |
| Reviewer lenses used | wiring + functional coverage, logic and edge cases (quote rule, cursor runes, paste route), security, style pass (`ze-go-style.md`: no panic, no discarded error, comments are sentences), docs drift |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | No functional test over the key path, though the `.et` harness reaches it | `test/editor/` | `question-mark-inside-quotes.et` (red/green recorded) |
| 2 | ISSUE | No test discriminated the rune cut the Critical Review requires | `model_keys_quote_test.go` | `TestQuestionMarkCursorIsCountedInRunes` (red/green recorded) |

### Notes
- NOTE: `quoteOpen` restates the toggle `tokenizeCommand` applies rather than sharing code with it; its comment names the source, and the `.et` Enter step proves the tokenizer reads the typed line with the quoted value as one token.
- NOTE: an unbracketed paste (a terminal without bracketed paste) arrives as key presses, so an unquoted pasted `?` is help there; the architecture page says "bracketed paste", which is accurate.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/cli/model_keys_quote_test.go` | yes | `git show --stat b5d6305b93` lists it |
| `test/editor/completion/question-mark-inside-quotes.et` | yes | editor run id 48 PASS |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-5 | model tests green | `go test -race -run 'QuestionMark|Paste' ./internal/component/cli/`: ok 2.705s |
| AC-1, AC-4 | key path end to end | `./le --name qmclose test editor -v -p question-mark-inside-quotes`: pass 1/1, steps 1-9 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| typed `?` key press | `test/editor/completion/question-mark-inside-quotes.et` | read: `input=type:text=` sends one `KeyPressMsg` per rune through `Model.Update` to `handleKeyMsg` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `tokenizeCommand` toggles only on `c == '"'`; no escape branch |
| A-2 | confirmed | `Model.Update` (`model.go`) has no `tea.PasteMsg` arm and ends in `m.textInput.Update(msg)`; vendored `textinput.Model.Update` handles `tea.PasteMsg` with `insertRunesFromUserInput`; no `WithoutBracketedPaste` in the tree; `TestPastedQuestionMarkReachesTheInput` green |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/cli.md` `?` inside a quoted value row | `handleKeyMsg` arm guard, `cursorInQuote` | yes |
| `docs/architecture/cli/command-completion.md` `?` paragraph | `cursorInQuote`, `quoteOpen`, `Model.Update` paste route | yes |
| `docs/guide/config-editor.md` `?` on a highlighted key | still true outside a quote | yes |
