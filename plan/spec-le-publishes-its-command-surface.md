# Spec: le-publishes-its-command-surface

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`le` declares its whole command surface in code and publishes only part of it.
`leaction.Action` carries the verb, the purpose, the write flag and the keyword
grammar. `leaction.Row`, the type every listing and every `| json` rendering is
built from, carries the first three and drops the grammar. The root help is not
a payload at all: `Usage` writes text to stderr, so `./le | json` answers
`unknown command: |`.

Three costs follow from that one omission.

A reader cannot see an action's grammar without invoking the action. The only
gesture that shows it is a trailing `--help`, and `leroot.Dispatch` passes a
trailing help word through to the handler, so in an area that does not check for
one the word is consumed as data and the action RUNS. `./le stress-repro run
suite --help` sets `suite=--help` and starts the burn: measured at about 943% CPU
for twenty minutes on a development machine on 2026-09-11.

An agent cannot enumerate the surface at all. There is no call that answers what
areas exist, what actions each holds, and what each action takes. The closest
artifact is the hand-written 87-row table in `ai/INDEX.md`, which no generator
writes and no gate compares against the registry.

The grammar that IS reachable is wrong about itself. `actionUsage` wraps every
parameter in brackets, so a required keyword and an optional one render
identically.

The goal is that `le` publishes what it already declares: the manifest is one
call, the grammar is in it, and no invocation carrying a help word can run work.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - the design `leaction.go` and `leroot.go` both name in their `// Design:` headers
  → Decision: an action answers structured data and `leroot` renders it, so `| json`, `| yaml` and `| table` reach every action with no per-tool code. A payload this spec adds MUST therefore be a struct with `json:` tags, never rendered text.
  → Constraint: `leroot.Answer` is `func(args []string) (payload any, code int)`. The dispatcher cannot see inside a handler, so anything the dispatcher must know about an area is declared at registration or is not knowable.
- [ ] `ai/rules/cli.md` - governs every CLI surface change
  → Constraint: a response payload MUST satisfy structured data and MUST NOT be text a renderer already formatted. The root help is text today, which is the violation this spec removes.
  → Constraint: the first token after the noun MUST be a keyword from a closed set, and a free-form value MUST NOT sit in an untyped positional slot. The published grammar must therefore describe keywords, never positions.
  → Decision: `--version`, `-V`, `--help` and `-h` are the one exception to the no-flag rule, so the three help spellings stay.
- [ ] `ai/rules/principles.md` - always-on
  → Constraint: every fact is declared once and every other surface derives from it. `ai/INDEX.md`'s 87-row inventory is a second declaration of the registry, which is why it can drift.
  → Constraint: a value that is silently wrong MUST NOT be reachable. A manifest that omits requiredness is a silently wrong answer to "what does this action take".
- [ ] `docs/contributing/ze-go-style.md` - mandatory before any Go
  → Constraint: derive help from the registry or schema; hardcoded help text is named as the anti-pattern.
  → Constraint: a zero value is never an answer. `Parameter.Required` defaulting to false must be read as a real declaration, not as "nobody said", so the migration in spec 2 sets it deliberately on every row.

**Key insights:**
- `leaction.Area` already holds everything the manifest needs. Nothing has to be re-derived; it has to be published.
- `leroot.Dispatch` is the one funnel every area's handler is reached through, so the help-word guard has exactly one correct home.
- The seven hand-rolled areas have no action table, so the dispatcher can guard them but cannot render their grammar. That half arrives with spec 2.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/leaction/leaction.go` - declares `Parameter{Keyword, Value}`, `Action{Verb, Why, Writes, Parameters, Answer, AnswerArgs}`, `Row{Verb, Writes, Why}`, `List{Area, Actions}`. `Area.Answer` dispatches; `actionUsage` renders one action's grammar to stderr; `parseArguments` validates a closed keyword grammar and REFUSES a keyword given twice.
- [ ] `internal/le/leroot/leroot.go` - `Register` wires an area at `le <name>`; `Run` splits the pipe chain, calls the handler, renders the payload. `RefuseArgument` answers 1 for a value after a zero-argument command.
- [ ] `internal/le/leroot/dispatch.go` - `Usage` builds a `helpfmt.Page` and calls `WriteErr`. `Dispatch` treats a help word as the FIRST argument, and passes a help word after a verb through to the handler. `helpNode` renders a node's page from `meta.ResolveSubs()`, which is verb names with no descriptions.
- [ ] `internal/le/leroot/group.go` - the five groups and their render order.
- [ ] `internal/core/helpfmt/helpfmt.go` - pads the name column and appends the description with no width limit.
- [ ] `internal/le/stressrepro/actions.go` - `Answer` has no help check; `parseOptions` consumes the word after `suite` as its value, so a trailing help word reaches `runAt`.

**Behavior to preserve:**
- Every existing `./le <area>` listing keeps its current text rendering, because `List.Text` is what a person reads today and 2,358 citations describe that surface.
- Every existing exit code stays what it is. Exit-code discipline is spec 3.
- `leaction.Area.Answer` keeps answering the `List` for a bare area and for a leading help word.
- `./le <area> <verb> --help` keeps answering 0. A reader who typed the help word asked a question rather than making a mistake.

**Behavior to change:**
- `Row` publishes the action's parameters.
- `Parameter` states whether a keyword is required and whether it may repeat, and `parseArguments` honours `Repeat` instead of refusing every second occurrence.
- `actionUsage` renders a required keyword differently from an optional one.
- The root answers a payload, so `./le | json` renders the manifest.
- A TRAILING help word never reaches a handler.

## Data Flow (MANDATORY)

### Entry Point
- `argv` reaching `leroot.Dispatch`, from the `le` launcher or from `ze le` in a `ze_le` build.

### Transformation Path
1. `Dispatch` resolves the registered command among the leading words (`resolve`, bounded at `commandWordsMax`).
2. NEW: when the last tool argument is a help word, `Dispatch` renders usage and returns. The handler is not called.
3. Otherwise `Run` splits the operator's pipe chain, calls the handler, and renders the payload through the chain.
4. NEW: with no command word at all, `Dispatch` answers a `Manifest` payload through `Run` rather than writing text, so the pipe chain reaches it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Launcher ↔ dispatcher | `argv`, unchanged | No |
| Dispatcher ↔ area | `leroot.Answer`, plus the new actions provider registered beside it | No |
| Dispatcher ↔ renderer | a JSON-encodable payload through `command.ProcessPipesDefaultFormatLocal` | No |

### Integration Points
- `leroot.Register` - gains a sibling `RegisterActions`, called by each `leaction` area's `register.go`, which already exposes `Actions() leaction.List`.
- `helpfmt.Page` - stays the renderer for a node page; the manifest's `Text` is the existing root help, moved behind a payload.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No le action takes a positional value, so a published keyword grammar describes the whole surface | conversion inventory read across all seven hand-rolled areas and the `leaction` parser; `goextract/answer.go` and `weekly/answer.go` state the keyword-only choice in their doc comments | The manifest would be incomplete for the positional actions and spec 2 could not migrate them | grep for a parser arm that consumes `args[0]` as a value without a preceding keyword | CONFIRMED for the seven, by reading each parser. What looks positional in `spec session state latest` and `spec status closure check` is a fixed literal sub-verb, not an operator-chosen value. Re-check in spec 2 when those sub-verbs flatten |
| A-2 | Every le payload already satisfies `json.Marshal`, so adding a root payload introduces no new encoding failure | `leroot.Run` already marshals every answer and reports a payload that does not encode | The root would answer an encoding error instead of the manifest | the round-trip test in the TDD plan | unvalidated |
| A-3 | A trailing help word is the gesture that causes the damage, so guarding the trailing position closes the hole | the measured `stress-repro run suite --help` burn; `leaction`'s own comment says a help word further up the line can be a keyword's legitimate value | A probe with the help word in a non-trailing position still runs work | a test asserting the guard for the trailing position, and a Known Limitation naming the rest | unvalidated |
| A-4 | The 63 `leaction` areas all expose `Actions() leaction.List` under that name, so `RegisterActions` needs no per-area authoring | every area read so far follows the pattern, and `leaction.New` is called in 63 directories | Some areas need a hand-written provider and the change is wider than planned | compile after wiring all 63 | BROKEN, bounded. Measured 2026-09-12: 62 of 63 expose `Actions()`. `internal/le/doc/wiring` declares `zeroArgumentActions` and exposes `Subs()` only, so it needs the accessor written. One area, one function, no change to the plan |
| A-5 | Publishing requiredness does not change what any action accepts today, because the accepting code is each action's own body | `parseArguments` validates only keyword spelling and value presence; requiredness is enforced inside `AnswerArgs` | Marking a parameter required would start refusing an invocation that works today | a test that an action with a required parameter still refuses at the same point and with the same code | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Six peer sessions are editing this checkout, so a change to `leaction` or `leroot` collides with work in flight | a commit script refusing a file carrying another session's hunks | The change is confined to four files under `internal/le/leaction/` and `internal/le/leroot/` plus 63 one-line `register.go` edits; land it as one commit and name any carried hunks |
| R-2 | `RegisterActions` is optional while the seven hand-rolled areas exist, which is the "some declare, some do not" drift this work exists to remove | an area added after this spec that registers no actions and is invisible to the manifest | Spec 2 migrates the last seven and makes the provider mandatory; until then a test asserts that every registered area either provides actions or is on a named, shrinking list of seven |
| R-3 | Making `Repeat` honoured in `parseArguments` loosens a parser that refuses a duplicate today, so an invocation that was refused starts being accepted | no signal: acceptance is silent | `Repeat` defaults to false, so every existing parameter keeps today's refusal; only a parameter that opts in changes |
| R-4 | The manifest becomes a second thing to keep in step with the root help text | the two disagreeing about an area's description | `Text` on the manifest payload IS the root help, so the text is derived from the manifest rather than written beside it |
| R-5 | A trailing-help guard in the dispatcher changes what `./le <area> <verb> --help` prints for a `leaction` area, where the area rendered it before | a changed usage line in an existing test | The dispatcher renders from the same `Parameters` the area declares, so the line is the same fact; the test asserts the text |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing an operator meets. `le` is a development tool and is in no shipped binary. A wrong landing breaks development commands for every session sharing the checkout, which is why R-1 exists |
| How is it reverted? | A single commit revert. No persisted state, no wire format, no config |
| Who else touches this path? | `plan/spec-le-command-namespaces.md` owns le's root NAMING grammar and is `ready`. Spec 2 of this series rewrites the seven hand-rolled areas. Six peer sessions share the checkout |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le` with no arguments | → | `leroot.Dispatch` answering a `Manifest` payload through `Run` | `TestBareRootAnswersTheManifestAsAPayload` |
| `./le \| json` | → | `leroot.Run` rendering the manifest through the pipe chain | `TestRootManifestRendersThroughTheJSONOperator` |
| `./le <area> <verb> --help` where the area runs work | → | `leroot.Dispatch` returning before the handler | `TestATrailingHelpWordNeverReachesTheHandler` |
| `./le <area> \| json` | → | `leaction.Row` carrying the action's parameters | `TestAreaListingPublishesEveryParameter` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le` with no arguments | Answers the same help text a reader sees today, on stdout, with exit 0 |
| AC-2 | `./le \| json` | Answers one JSON document naming every registered area, its group, its description, and for each area its actions with verb, why, writes and parameters |
| AC-3 | `./le <area> \| json` for a `leaction` area | Each action row carries its parameters, each with keyword, value name, required and repeat |
| AC-4 | `./le <area> <verb> --help` for an action with a required keyword and an optional one | The required keyword renders without brackets and the optional one renders inside them, and the two are distinguishable in the text |
| AC-5 | A trailing help word after any verb of any registered area, including the ones that hand-roll dispatch | The handler is not called, usage is rendered, and the exit code is 0 -- with ONE exception, decided by whether the word carries a DASH. The bare word `help` carries none, so where a declared keyword's value slot holds it, it is that keyword's value and the action runs (`le source-rewrite replace file <path> old beta new help`). A dash-leading word is an OPTION: le follows GNU option syntax, `ai/rules/cli.md` declares le's whole option set (`--help`, `-h`, `--version`, `-V`) and bans every other flag from its grammar, so no action takes a value that begins with a dash. FOUR spellings ask for help, and the set is CLOSED: `help`, `--help`, `-h` and `-help`. A trailing one of the four asks the question in every area (`le stress-repro run suite -help` renders a page and starts no burn). Every other option in a value slot is REFUSED with code 2 by `parseArguments`, wherever it stands on the line, so `le verify status check path --help path internal` checks no path and `le verify status check path -xh` answers 2. An area that publishes no action table has no value slot the dispatcher can read, so the four spellings ask the question there and every other option reaches that area's own parser, which is what `command <argv...>` means for `le job run` |
| AC-6 | `./le stress-repro run suite --help` | Renders usage and starts no burn, proven by the run function not being reached |
| AC-7 | An action declaring a parameter with `Repeat` true, given that keyword twice | Both values are parsed, and a parameter without `Repeat` given twice is still refused with the existing message and code |
| AC-8 | An action declaring a parameter with `Required` true, invoked without it | Refused exactly where and how it is refused today, because requiredness is published rather than newly enforced |
| AC-9 | Every registered le area | Either registers an actions provider, or is named in the migration list, and no other area may register none. Measured 2026-09-12: 89 areas, 63 wired, 26 on the list; after review round 1 migrated `verify status`, 64 wired and 25 on the list. The list ratchets both ways, so an unlisted area with no table fails and a listed area that starts publishing fails until its row is deleted |
| AC-10 | An action body reading a keyword's value | Reads it through a named accessor. A direct index of `Arguments` does not compile, so a `Repeat` keyword cannot be read as a single string by accident |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRowCarriesTheParametersItsActionDeclares` | `internal/le/leaction/leaction_test.go` | AC-3 | |
| `TestUsageDistinguishesARequiredKeywordFromAnOptionalOne` | `internal/le/leaction/leaction_test.go` | AC-4 | |
| `TestARepeatableKeywordIsParsedTwiceAndANonRepeatableIsStillRefused` | `internal/le/leaction/leaction_test.go` | AC-7 | |
| `TestARepeatKeywordCannotBeReadAsOneValue` | `internal/le/leaction/leaction_test.go` | AC-10 | |
| `TestManifestNamesEveryRegisteredAreaAndItsGroup` | `internal/le/leroot/dispatch_test.go` | AC-2 | |
| `TestManifestTextIsTheRootHelpAReaderSeesToday` | `internal/le/leroot/dispatch_test.go` | AC-1 | |
| `TestATrailingHelpWordNeverReachesTheHandler` | `internal/le/leroot/dispatch_test.go` | AC-5, AC-6 | |
| `TestAnOptionIsRefusedInAValueSlotAnywhereOnTheLine` | `internal/le/leaction/leaction_test.go`, `internal/le/leroot/dispatch_test.go` | AC-5. Review round 3: an option in a value slot is refused with code 2 wherever it stands, and the bare word `help` in the same slot is still data | |
| `TestATrailingOptionThatIsNotTheQuestionReachesATableLessArea` | `internal/le/leroot/dispatch_test.go` | AC-5. Review round 3: the guard fires for every help spelling in an area with no table, and every other option reaches that area as argv | |
| `TestEveryRegisteredAreaProvidesActionsOrIsOnTheMigrationList` | `internal/le/actions_test.go` | AC-9. It sits in `internal/le` rather than in `leroot`, because `leroot`'s own test binary registers no area and the same assertion there would pass over an empty set | |
| `TestBareRootAnswersTheManifestAsAPayload` | `internal/le/leroot/dispatch_test.go` | wiring | |
| `TestRootManifestRendersThroughTheJSONOperator` | `internal/le/leroot/dispatch_test.go` | wiring, AC-2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | this spec adds no numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | `le` registers no daemon command, so `.ci` has no surface to drive. The dispatcher tests run the real `Dispatch` from argv, which is le's entry point | N-A | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | Scope is tooling; no wire-visible behavior changes | N-A | N-A | |

## Files to Modify
- `internal/le/leaction/leaction.go` - `Parameter` gains `Required` and `Repeat`; `Row` gains `Parameters`; `actionUsage` renders the two forms; `parseArguments` honours `Repeat`; `New` validates the new fields.
- `internal/le/leroot/leroot.go` - `RegisterActions` records an area's actions provider beside its group.
- `internal/le/leroot/dispatch.go` - `Dispatch` answers the manifest for a bare invocation and guards a trailing help word; `Usage` renders from the manifest payload.
- `internal/le/*/register.go` - 63 areas call `RegisterActions` with the `Actions` function each already exposes.
- `ai/INDEX.md` - the hand-written native command inventory states that `./le | json` is the authority, so the table stops being a second declaration.
- `docs/contributing/running-commands.md` - names the manifest as the way to discover a command.

## Files to Create
- `internal/le/leroot/manifest.go` - the `Manifest` payload and its `Text` rendering.
- `internal/le/leroot/manifest_test.go` - the manifest's own tests.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | `le` is a Go tool personality; it declares no YANG |
| YANG validation constraints | N-A | no YANG leaf added |
| YANG custom validators | N-A | no YANG leaf added |
| CLI commands/flags | Yes | `internal/le/leroot/dispatch.go`; no new flag, the three help spellings already exist |
| CLI grammar (keyword before value) | Yes | `Parameter.Required` and `Repeat` describe a keyword grammar; `./le cli-grammar` is the gate |
| Editor autocomplete | N-A | `le` has no YANG-fed completion surface |
| Functional test for new RPC/API | N-A | no RPC; the dispatcher tests drive the real entry point from argv |
| Pipe completeness | Yes | the manifest is a payload, so `\| json`, `\| yaml` and `\| table` reach it through `leroot.Run` unchanged |
| Env var registration | N-A | no env var added |
| Doctor check for runtime dependencies | N-A | no new file path, socket, port, binary or kernel surface |
| Prometheus counters/metrics | N-A | development tooling emits no metrics |
| BGP family surface | N-A | not a protocol change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | N-A | `le` is not in a shipped binary, so `docs/features.md` does not describe it |
| 2 | Config syntax changed? | N-A | no config surface |
| 3 | CLI command added/changed? | Yes | `docs/contributing/running-commands.md`; `docs/guide/command-reference.md` covers the `ze` operator CLI, not `le` |
| 4 | API/RPC added/changed? | N-A | no RPC |
| 5 | Plugin added/changed? | N-A | no plugin |
| 6 | Has a user guide page? | Yes | `docs/contributing/running-commands.md` |
| 7 | Wire format changed? | N-A | no wire surface |
| 8 | Plugin SDK/protocol changed? | N-A | no SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | not a protocol change |
| 10 | Test infrastructure changed? | N-A | no test runner change |
| 11 | Affects daemon comparison? | N-A | `le` is not a daemon feature |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, which both changed files name in their `// Design:` header |
| 13 | Route metadata keys added/changed? | N-A | no route metadata |
| 14 | Prometheus counters added/changed? | N-A | no metrics |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `ai/INDEX.md`'s native command inventory becomes derived rather than authoritative |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED, run 2026-09-12: `./le spec citation anchors spec plan/spec-le-publishes-its-command-surface.md` names one page, `docs/architecture/system-architecture.md`, mentioned by `internal/le/leroot/dispatch.go`. It is advisory rather than blocking: the mention is a `<!-- source: -->` reference, not a `// Design:` declaration. Judge it against the changed dispatcher and update it if it describes the bare-invocation or help path |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/contributing/running-commands.md` and `ai/INDEX.md` both show `./le` invocations; check each against the registry once the manifest exists |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the manifest exists and is reachable, and the guard exists
   - Tests: `TestBareRootAnswersTheManifestAsAPayload`, `TestATrailingHelpWordNeverReachesTheHandler`
   - Files: `internal/le/leroot/manifest.go`, `internal/le/leroot/dispatch.go`, `internal/le/leroot/leroot.go`
   - Verify: the entry point exists and is reachable; the wiring tests fail because the manifest is empty and the guard renders nothing
2. **Phase: the grammar becomes publishable** -- `Parameter` states requiredness and repetition, `Row` carries the parameters, `New` validates them
   - Tests: `TestRowCarriesTheParametersItsActionDeclares`, `TestARepeatableKeywordIsParsedTwiceAndANonRepeatableIsStillRefused`
   - Files: `internal/le/leaction/leaction.go`
   - Verify: tests fail → implement → tests pass
3. **Phase: usage tells the reader which keyword is required** -- `actionUsage` renders the two forms
   - Tests: `TestUsageDistinguishesARequiredKeywordFromAnOptionalOne`
   - Files: `internal/le/leaction/leaction.go`
   - Verify: the existing `session seed-store` and `session scratch` usage lines stop being identical in shape
4. **Phase: every area provides its actions** -- 63 `register.go` files call `RegisterActions`
   - Tests: `TestEveryRegisteredAreaProvidesActionsOrIsOnTheMigrationList`
   - Files: `internal/le/*/register.go`
   - Verify: the manifest names every area's actions except the seven on the migration list
5. **Phase: the root help is the manifest's rendering** -- `Usage` renders the payload's `Text`, and the manifest reaches the pipe operators
   - Tests: `TestManifestTextIsTheRootHelpAReaderSeesToday`, `TestRootManifestRendersThroughTheJSONOperator`, `TestManifestNamesEveryRegisteredAreaAndItsGroup`
   - Files: `internal/le/leroot/dispatch.go`, `internal/le/leroot/manifest.go`
   - Verify: the root help text is unchanged for a person, and `./le | json` answers the manifest
6. **Phase: the documentation stops being a second declaration** -- `ai/INDEX.md` and `docs/contributing/running-commands.md` name the manifest
   - Files: `ai/INDEX.md`, `docs/contributing/running-commands.md`, `docs/architecture/core-design.md`
   - Verify: `./le doc check verify` and `./le ste check`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:symbol |
| Feature completeness | The manifest names every area a bare `./le` prints, with no area reachable but unpublished |
| Correctness | The guard returns BEFORE the handler on every path through `Dispatch`, including the namespace-token path and the folded-pipe path |
| Naming | Every new JSON key is lowercase kebab-case (`ai/rules/cli.md`) |
| Data flow | `Usage` renders the manifest rather than building a second `helpfmt.Page` from the registry |
| Rule: `ai/rules/no-layering.md` | `RegisterActions` is transitional and spec 2 removes its optionality; no second help renderer survives this change |
| Rule: `ai/rules/principles.md` | `Parameter.Required` false means "declared optional", not "nobody said"; `New` refuses a table that leaves it ambiguous |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `./le \| json` answers the manifest | `./le '\|' json` and read the document |
| A probe cannot start a job | `./le stress-repro run suite --help` returns promptly with usage |
| Required and optional keywords read differently | `./le session seed-store --help` against `./le session scratch --help` |
| Every area publishes its actions | `TestEveryRegisteredAreaProvidesActionsOrIsOnTheMigrationList` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The guard must not be bypassable by a help word supplied as a legitimate keyword value; the trailing-position rule is what keeps the two apart |
| Resource exhaustion | The guard exists because a probe could start an unbounded CPU burn; the test must prove the run function is not reached, never merely that output looks like usage |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| An area's `Actions` function is named differently | A-4 is broken: record it, write the provider by hand, and name the area |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- `Arguments` carries one value for each keyword, and a `Repeat` keyword carries
  every value it was given. The NUL-joined `map[string]string` written first is
  REVERSED: it reached the right answer through `Values(keyword)` and a silently
  wrong one through `a[keyword]`, which still compiles and hands back
  `"first\x00second"`. `ai/rules/principles.md` forbids exactly that, because a
  caller cannot tell the joined string from a value an operator typed.
  Remeasured 2026-09-12: the conversion is 44 direct index reads across 14
  files, not the 87 the first estimate gave, and `internal/le/commit` already
  holds the honest shape as `keywordValues map[string][]string` with `one()` and
  `has()`. So the map shape was the cheaper answer only on a number that was
  wrong, and keeping it would have made spec 2's `commit` migration a DOWNGRADE
  from a correct multi-value map to a joined string.
  → Decision: `Arguments` stops being a bare `map[string]string`, so a direct
  index stops compiling and every one of the 44 reads fails loudly at build time
  rather than silently at run time.
- `New` refuses a boolean switch declared `Required` or `Repeat`. Presence is a
  switch's whole meaning, so neither field can say anything about one, and
  refusing the two shapes is what keeps `Required: false` reading as "declared
  optional" rather than "nobody said".
- The dispatcher's help guard renders an action's grammar from the listing the
  area registered, so `le <area> <verb> --help` answers the area page rather
  than the action's keywords until step 4 wires the 63 `register.go` files. The
  two steps belong in one landing.
- The trailing POSITION is a proxy for the question "is this word data", and the
  proxy is wrong wherever a keyword's value is the last thing typed. Review
  round 1 measured it: `le source-rewrite replace file <path> old beta new help`
  answered 0, printed usage and ran nothing, where the same line ran the
  replacement before this spec. A guard that swallows an invocation and answers
  0 is the silent-wrong-value failure `ai/rules/principles.md` names, inside the
  commit that exists to remove one.
  → Decision: the guard asks the area's registered table whether the last word
  lands in a declared keyword's VALUE slot (`leaction.List.TrailingWordIsValue`,
  over `trailingIsValue`). A word in a value slot is data and travels on; a word
  anywhere else is the question and is answered without the handler.
  → Constraint: `leroot.Dispatch` is not the only guard site.
  `leaction.Area.Answer` holds the same trailing-help check for the area's own
  callers, so both read one predicate. Two copies of the walk would be two
  answers to one question about one grammar.
- `Arguments.Values` and `Parameter.Repeat` shipped with a test as their only
  user, which `ai/rules/completion.md` calls dead code. `verify status check`
  was already accumulating repeated `path <value>` pairs in a hand-rolled loop,
  so the repeat machinery had a production caller waiting for it.
  → Decision: `verify status` migrates onto `leaction.New`, declares `path` with
  `Repeat`, and reads it with `Values("path")`. One area leaves the exemption
  list (26 rows, now 25), the machinery gets its caller, and the area publishes
  the grammar its four verbs always had.
- The value-slot exemption is drawn at the DASH, not at the position and not at
  the two help spellings. Position answers "could a keyword have introduced this
  word", which is the wrong question for an option: le follows GNU option
  syntax, `ai/rules/cli.md` declares le's whole option set and bans every other
  flag from its grammar, so no le action takes a value that begins with a dash,
  while `help` carries no dash and an operator can type it as text. Review round
  2 measured the cost of the position-only rule (`le verify status check path
  --help` ran the check over a path named `--help`), and the owner then measured
  the cost of the two-spelling rule: `le stress-repro run suite -help` set
  `suite=-help` and started the burn, because `-help` is a CLUSTER of short
  options (`-h -e -l -p`) rather than a third spelling of the word.
  → Decision: `leaction.IsOption` reports a dash-leading word, `trailingIsValue`
  answers false for one before it walks the grammar, and `parseArguments`
  refuses one in every value slot with code 2.
  → Decision: the HELP question is a CLOSED set of four spellings, `help`,
  `--help`, `-h` and `-help`, and `IsHelpArg` tests nothing else. Review round 4
  measured what a rule of shape costs: reading any cluster carrying `h` as the
  question made `le job run label x command echo -html=cover.out` answer 0 with
  a help page and never run the child, which is the silent no-op this spec
  exists to remove. The shape was too narrow as well, because `-xh` carries the
  help option and still reached the area as data. A table-less area publishes no
  grammar, so le cannot tell its OWN option from a forwarded one, and checking
  each letter against le's declared options would refuse `command prog -xyz`.
  Four words cost a wrong help page at worst; a shape costs a silent no-op.
  → Constraint: the refusal lives in `parseArguments`, so it reaches the actions
  that dispatch through the table. An area that hand-rolls its parser, and an
  area that forwards raw argv to a child, are unchanged by it.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `RegisterActions` beside `Register` rather than a fifth argument to `Register` | Changing `Register`'s signature at 89 call sites in one commit | The owner chose the two-spec cut on 2026-09-12 to keep the first diff small against a checkout six sessions are editing. The optionality is transitional and spec 2 removes it |
| The guard fires on a TRAILING help word only | Refusing a help word anywhere in the invocation | `leaction`'s own comment records that a help word further up the line can be the value a keyword introduced. Refusing it everywhere would make a legitimate value unreachable |
| `Parameter` gains `Required` and `Repeat` in this spec, not in the migration | Adding them in spec 2 with the seven areas that need them | This spec owns the published shape. A manifest that omits requiredness is the same lie the brackets tell today, and `parseArguments` refusing every repeat would block `commit` from migrating at all |
| The manifest's `Text` IS the root help | Keeping `Usage` as it is and adding a payload beside it | Two renderings of one fact drift. Deriving the text from the payload is the same rule this spec applies to `ai/INDEX.md` |
| One `helpfmt.Page` builder on the manifest, rendered by `Text` for stdout and by `Usage` for stderr | `Usage` printing `Text` to stderr | The page content is declared once either way. Color is decided per stream, and deciding stderr's color from stdout is the one thing the single-call shape would get wrong |

## Known Limitations
- In an area that hand-rolls its parser, any option that is not one of the four help spellings is read as DATA. `./le stress-repro run suite -xh` and `./le stress-repro run suite --help burners 4` both still start the burn: the second was measured here on 2026-09-12 (it printed `suite=--help` and began the run), and the first by review round 4. `stress-repro` publishes a table for its listing and parses the line itself (`internal/le/stressrepro/actions.go`, `parseOptions`), so `parseArguments` never sees the line. `plan/spec-le-every-area-dispatches-through-one-table.md` closes it, and only it can: the dispatcher tells le's own option from a forwarded child's only where a grammar was declared, so no rule over the option's SHAPE can stand in for that table. The bare word `help` stays data in any slot, in every area.
- In an area that declares no action table, a legitimate value spelled `help` is unreachable when it is typed LAST: the dispatcher cannot read a grammar nobody published, so it guards and renders the node page. That is the safe direction, because the alternative is the `stress-repro` burn, and it is a NEW limitation this spec introduces. It holds for the areas named in `areasWithoutAnActionTable` (`internal/le/actions_test.go`) and for no other, and `plan/spec-le-every-area-dispatches-through-one-table.md` removes it one area at a time as each declares its table. An area that HAS declared one takes the bare word as data whenever a declared keyword introduced it.
- A value that begins with a dash is unreachable in the line of an area that DECLARES a table. That is not a limitation of the guard, it is `ai/rules/cli.md`: le follows GNU option syntax, the four options it declares are the whole set, and no flag is grammar or a value. No migration changes it.
- An option that is not one of the four spellings reaches an area that declares no table, because the dispatcher has no grammar to read it against and the area's own parser owns the line. That is what `command <argv...>` means: `./le job run label encode-list command bin/ze-test bgp encode --list` hands `--list` to the child, which `docs/contributing/testing.md` prints as a recipe, and `./le job run label x command echo -html=cover.out` prints the option. Guarding every option there would refuse both.
- Twenty-five areas are guarded but publish no grammar, because they declare no `leaction` table. Six of them hand-roll a multi-verb dispatcher and are the subject of `plan/spec-le-every-area-dispatches-through-one-table.md`, which named seven until `verify status` migrated here (review round 1, to give `Repeat` and `Values` a production caller). The other nineteen are single-verb tools that refuse every argument by hand, declare their own `ActionList` type, or build a `leaction.List` inline from an unexported function. That spec empties the six remaining rows; deciding what the nineteen owe is its to settle, and the list ratchets so neither group can grow.
- Requiredness is PUBLISHED and not newly ENFORCED (AC-8). The table therefore states a fact each action's own body also enforces, which is a second declaration of one fact. It stays that way deliberately: `commit create` requires `subject` only sometimes and `commit debt-discharge` requires `owner` only when `kind` is `owner`, and a flat keyword table cannot express a cross-field rule. Central enforcement needs conditionality in the grammar, which is its own spec.
- Verb vocabulary, exit-code discipline and help-text wrapping are untouched here. They are spec 3 of this series.

## RFC Documentation (Scope: protocol)

N-A. Scope is tooling.

## Review Gate

Scope is declared BEFORE each round runs, so a round cannot shrink to whatever
produces a clean result (`ai/rules/planning.md`).

| Round | Scope declared before it ran | Commit under review | Result |
|-------|------------------------------|---------------------|--------|
| 1 | The whole diff, three lenses: correctness of the help guard on every dispatch path; zero values that read as answers (`Requirement`, `Arguments`, the 22 Required decisions sampled against their bodies); and whether each AC's named test would go red on a regression | `73861cc6c` | 0 BLOCKER, 2 ISSUE. A trailing help word in a declared value slot was swallowed, answering 0 and running nothing. `Arguments.Values` and `Parameter.Repeat` had no non-test caller |
| 2 | ONLY the two fixes and the call sites they touch: `trailingIsValue` and `List.TrailingWordIsValue` (`internal/le/leaction/leaction.go`), `asksForUsage` and `helpTrailing` (`internal/le/leroot/dispatch.go`), `Area.Answer`'s copy of the same check, and the whole of `internal/le/verify/status` as migrated. Plus the eight always-in-scope classes anywhere | `1b63b7354` | 0 BLOCKER, 2 ISSUE. `trailingIsValue` absorbs `-h` and `--help` as a keyword's value, where `ai/rules/cli.md` makes the flag spellings the one exception that is never data. AC-5 still claims the handler is never called, which `1b63b7354` deliberately changed. It confirmed the guard did not weaken for the 25 table-less areas, that the grammar check is ONE function rather than two copies, and that `Repeat` and `Values` now each have a non-test caller |
| 3 | ONLY the ISSUE-1 fix in `trailingIsValue` and `leaction.IsHelpArg`, the call sites that read them, and the AC-5 rewording. Plus the eight always-in-scope classes anywhere | `8036f6c2f` | 0 BLOCKER, 1 ISSUE, 1 NOTE. Six sentences claimed a flag is never data in any slot while `trailingIsValue` tested the trailing word only, so a flag in a non-trailing value slot was still data. NOTE: `-help` started the burn because it was not one of the three spellings |
| 4 | ONLY the generalisation the owner's GNU ruling produced: `IsOption`, `IsHelpArg` reading a short cluster, the dash-leading refusal in `parseArguments`, `trailingIsValue`'s argv-forwarding carve-out, and the nine corrected sentences. NOT `verify status`, which round 2 reviewed and this commit does not touch. Plus the eight always-in-scope classes anywhere | `468394d76d` | |

The eight always-in-scope classes apply to every round whatever its scope: an
unwired symbol, a vacuous test, an acceptance criterion with no test, a
user-facing behavior with no functional test, Linux-only code with no QEMU test,
a removed guard, a newly added guard that fails open, and any RFC or interop
non-conformance. Where a round's scope and that list disagree, the list wins.

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
- [ ] AC-1..AC-10 all demonstrated
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
