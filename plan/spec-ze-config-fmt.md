# Spec: ze-config-fmt

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-15 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later Opus 5 session reviews that commit and closes. `-` closes in the same session. -->

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     Deferral shard: every deferred item lands there (ai/rules/planning.md)
     and closure must resolve its rows, so name the file from the start. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze needs one canonical text form for its configuration, and a gate that holds
every config in this repository to it. Today the repository publishes config
that ze refuses, and nothing catches it.

**A formatter already exists. This spec does not create one.** `ze config fmt`
is implemented in `internal/component/config/cli/cmd_fmt.go` (`cmdFmt`,
`configFmtBytes`), with `-w`, `--check`, `--diff`, and stdin support. It parses
with `config.NewParser` and prints with `config.Serialize`. The work here is to
decide what canonical means, close the gaps that make the existing command
unsafe to run on an operator's file, and put the result behind a gate.
`ai/rules/no-layering.md` applies: a second formatter must not be added beside
this one.

### Why this spec exists

`spec-fixit-peer-process-event-filter` renamed a config keyword across about
500 files. Its review surfaced two facts.

**1. Operator guides show config ze refuses.** The main thread verified with a
freshly built binary that a one-line attach block is invalid, both without and
with an explicit semicolon:

    attach process bgp-rr { receive [ update ] }      -> configuration invalid
    attach process bgp-rr { receive [ update ]; }     -> configuration invalid

`docs/guide/bgp-resilience.md`, `docs/guide/plugins.md`,
`docs/guide/flowspec-route-reflector.md` and
`docs/guide/flowspec-protected-router.md` each show that form in examples an
operator would copy. A review lens then cited those lines as evidence that a
config shape was live in the tree, so invalid config was read as proof about the
language.

`plan/journal/documentation-shows-config-the-parser-refuses.md` records four
rows, all dated 2026-08-15. Two of them say "recorded, not fixed", so the class
is live and already recurring: `docs/guide/ospf.md`,
`docs/architecture/api/process-protocol.md`, `docs/guide/rpki.md`,
`docs/guide/route-reflection.md` and `docs/guide/graceful-restart.md` each carry
an example the parser refuses.

**2. Nothing checks that a config example parses.** No make target, CI job, or
script runs `ze config fmt --check` or `ze config validate` over the repository.
That is why the examples survived. And one config concept has many spellings:
flat and nested, quoted and unquoted keys and values, bracket list and bare
value, the `set` command form, and the inline single-leaf form. A guard that
tried to read them all by pattern was patched in four consecutive review rounds
and found a new unread shape every time. A guard built on the parser reads every
shape by construction, which is the argument for routing this through the
formatter rather than through a text matcher.

### The goal

| # | Goal |
|---|------|
| G-1 | One canonical text form for a ze config, defined by the parser and one serializer |
| G-2 | `ze config fmt -w` is safe to run on an operator's file, so it never loses information the file carried |
| G-3 | A gate refuses a config example, fixture, or file in this repository that does not parse, and refuses one that is not canonically formatted |
| G-4 | The carrier classes are covered, not only `.conf` files |

## Open Questions (decide at DESIGN)

Each row is a decision for Thomas. None is settled here.

| # | Question | What is already known |
|---|----------|-----------------------|
| Q-1 | **Scope of carriers.** `.conf` files only, or also `.ci` fixtures, `tmpfs=` bodies, fenced blocks in `docs/`, and config built by Go string builders? | 940 `.conf` files exist, 760 of them under `test/`. 395 `.ci` files carry a `tmpfs=<name>.conf` body and 346 carry a `stdin=config` body. 31 files under `docs/` carry a top-level `bgp {` line. The Go builders are the carrier class that defeated this spec's own motivating 500-file sweep |
| Q-2 | **Which spelling is canonical.** The parser accepts several spellings of one tree. Does the formatter normalize the others to the canonical one, or refuse them? | `config.Serialize` already picks one for every construct. Normalizing is the `gofmt` answer and it makes the gate a formatter run. Refusing is a stricter language and it turns every non-canonical file into an error rather than a rewrite |
| Q-3 | **Agreement with the serializer.** `internal/component/config/flatten.go` and the `ze:flatten` extension already decide what ze prints for one construct. Does the formatter reuse `config.Serialize`, or replace it? | Reusing it is the only answer that keeps one canonical form. Replacing it means every other printer (`ze config show`, the editor, the `zefs` diff path) moves at the same time |
| Q-4 | **A second serializer already disagrees.** `internal/exabgp/migration/migrate_serialize.go` (`SerializeTree`, `serializeTreeIndent`) prints ze config text with its own rules: alphabetical key order, and quoting only when the value holds a space. `config.Serialize` uses YANG schema order and `quoteIfNeeded` quotes on space, tab, quote, apostrophe, brace, `;`, and `#`. Which one survives? | Two producers of one language is the layering this spec exists to remove. `ai/rules/no-layering.md`: delete the loser, do not wrap it |
| Q-5 | **Comment preservation.** The tokenizer discards comments, so the tree carries none and the formatter deletes every comment in the file. Is comment preservation in scope? | This is where most formatters get hard. It decides whether `ze config fmt -w` is a tool an operator may run on a live file. Measured, see Current Behavior |
| Q-6 | **Idempotence and round-trip.** Format twice equals format once. Parse, format, parse again yields the same tree. Are these the acceptance properties? | They are the two properties that make a formatter testable without a golden file per construct. Neither is asserted anywhere today |
| Q-7 | **The surface.** `ze config fmt` and its `--check` mode exist. Does the gate run them, over which population, and does it live in `./le doc-check verify`, in `./le doc-wiring`, or in a new target? | `ai/rules/repo-maintenance.md` owns whether a docs gate is worth building, and its Current Discovery Surfaces table lists the changed-file-aware gates that already exist. `ai/rules/cli.md` owns the grammar, and `config fmt` already satisfies action before identifier |
| Q-8 | **Elided and partial examples.** `docs/guide/plugins.md` shows `attach process rib { ... }` with a literal ellipsis, and `docs/guide/flowspec-route-reflector.md` shows a full block inside a markdown table cell rather than a fenced block. How does the gate see a fragment that was never meant to be a whole config? | A gate that reads only fenced blocks misses both. A gate that reads everything must be told how an intentional fragment declares itself. **Two constraints are already decided. They were retired into this cell from `ai/rules/points/writing/documentation/` on 2026-08-16.** First, the gate MUST read what ze's own parser recognizes as a config attempt, and it MUST carry an opt-OUT that states its reason on the block. A gate over every fenced block in `docs/` fires mostly on deliberate excerpts, an estimated four in five, because they start mid-tree or carry a placeholder. An opt-IN marker inverts the failure: every example that is already refused stays unmarked and uncaught, which is the `rpki.md` case exactly. Second, whoever proposes the gate MUST state that annotation cost, and MUST NOT sell the gate. Somebody annotates the excerpts one time, and each new excerpt pays one line |
| Q-9 | **Options.** `gofmt` normalizes with no configuration, and that is the reason it won. Does `ze config fmt` stay option-free? | Every style option is a second canonical form, so it is a second thing the gate must accept. The existing flags select an action, not a style: none of `-w`, `--check`, `--diff` changes the output text |

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - the published grammar, and the `// Design:` anchor of every file in `internal/component/config/`
  → Constraint: this doc is the operator-facing description of the language, so any canonical-form decision changes it. Its "Process Section" is the surface the motivating migration rewrote
  → Decision: (fill during design) which sections state a canonical spelling and which state only an accepted one
- [ ] `ai/rules/no-layering.md` - two serializers already print ze config text
  → Constraint: when replacing X with Y, delete X first. A formatter that leaves `internal/exabgp/migration/migrate_serialize.go` in place has added a layer, not removed one
- [ ] `ai/rules/repo-maintenance.md` - owns the gate question and the discovery surfaces
  → Constraint: the Mechanical Checklist must be answered in this spec: where an agent looks first, what rule prevents regression, what source of truth prevents drift, what verification proves it
- [ ] `ai/rules/cli.md` - command grammar
  → Constraint: a closed keyword precedes any user-supplied value. `ze config fmt <file>` already satisfies it, so no grammar change is expected
- [ ] `ai/rules/simplicity.md` - the option-free formatter is the simplest answer only if one canonical form serves every carrier
  → Constraint: Q-1 decides that, so Q-1 is answered before Q-9

### RFC Summaries (Scope: protocol)
- N/A. Configuration syntax is a ze surface, governed by no RFC.

**Key insights:** (minimal context to resume after compaction)
- `ze config fmt` exists and is parse-then-`config.Serialize`. This spec decides its contract, it does not build it.
- The tokenizer discards comments, so today `ze config fmt -w` deletes every comment in the file. Measured, not inferred.
- Automatic semicolon insertion fires only at a newline or at end of input, so a block written on one line carries no statement terminator.
- Two independent serializers print ze config text, and they disagree on ordering and on quoting.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/config/tokenizer.go` - the lexer. `tokenizer.Next` sets `insertSemi` after a WORD, STRING, `]`, or `)`. `tokenizer.scan` emits the synthetic semicolon only when `skipWhitespaceAndComments` crossed a newline or the input ended. `skipWhitespaceAndComments` consumes a `#` comment to end of line and produces no token, so no comment reaches the parser
- [ ] `internal/component/config/parser.go` - `Parser.Parse`, `Parser.parseRoot`, `Parser.parseLeaf`. `Parser.parseContainer` implements automatic brace insertion: when the token after a container name is a word naming a child of that container, the child parses with no braces, so the flat spelling and the nested spelling both parse
- [ ] `internal/component/config/serialize.go` - `Serialize` walks `serializeTree` in YANG schema child order, indents with tabs, writes no explicit semicolons, and appends unknown keys sorted alphabetically through `serializeExtraValues`. `quoteIfNeeded` quotes an empty string and any value holding a space, tab, quote, apostrophe, brace, `;`, or `#`. `normalizeBool` prints `enable` and `disable`. `canInlineContainer` with `serializeContainerInline` collapses a container holding exactly one leaf onto one line, bounded by `maxInlineDepth`
- [ ] `internal/component/config/flatten.go` - `hasFlattenExtension`, `canFlattenContainer`, `serializeFlattenedContainer`. The `ze:flatten` extension chooses the flat spelling at print time. Exactly one YANG node carries it: `container attach` in `internal/component/bgp/yang/ze-bgp-conf.yang`
- [ ] `internal/component/config/serialize_set.go` - `DetectFormat` classifies a file as `FormatHierarchical`, `FormatSet`, or `FormatSetMeta` by scanning every line. `SerializeSet` prints the same tree as `set <path> <value>` lines in schema order, with `nop` for an inactive leaf
- [ ] `internal/component/config/cli/cmd_fmt.go` - `cmdFmt` and `configFmtBytes`. Exit 0 on success, exit 1 for `--check` when the file would change, exit 2 on a parse error
- [ ] `internal/exabgp/migration/migrate_serialize.go` - `SerializeTree` and `serializeTreeIndent`, a second and independent printer of ze config text. Sorts keys with `sort.Strings` and quotes only when the value holds a space
- [ ] `test/parse/cli-config-fmt.ci` - the only functional test of the command. It asserts three `contains` lines and nothing about idempotence, comments, or completeness

**Measured, with the `bin/ze` present in this checkout, built 2026-08-14:**

| Input | Output |
|-------|--------|
| A config carrying `# top comment` and `# inner comment` | Exit 0, and both comments absent from the output |
| `router-id` written before `peer p1` | `router-id` printed after `peer p1`, in YANG schema order |
| `local { ip 127.0.0.2 }` written as a block | Printed inline as `local ip 127.0.0.2` |
| That output fed back through `ze config fmt` | Byte identical to the first pass, on this one sample |

**Not established:**
- Why the one-line attach block is refused when it carries an explicit semicolon. Reading `tokenizer.scan` explains the semicolon-free form: after `]` the next scan crosses no newline, so no synthetic semicolon is produced and the parser meets `}` where it expects a terminator. That trace does not explain the explicit-semicolon form, where `scan` returns the real semicolon token. The `bin/ze` in this checkout predates the uncommitted edit to `internal/component/bgp/yang/ze-bgp-conf.yang`, so it answered `unknown field in peer: attach` and could not reach the question. Resolve this at DESIGN with a freshly built binary before any gate message quotes a cause.
- Whether `config.Serialize` is idempotent and round-trip stable over the whole corpus. One sample held. Nothing measures the rest.
- Whether every `.conf` file in the tree parses today.

**Behavior to preserve:** (unless the user explicitly said to change it)
- `ze config fmt` exit codes: 0 success, 1 for `--check` with changes pending, 2 for a parse error. `test/parse/cli-config-fmt.ci` and `test/ui/dash-stdio.ci` read them.
- `config.Serialize` output is read back by `ze config show`, the editor, and the `zefs` diff path. A change to the printed form moves all of them.
- The parser keeps accepting both the flat and the nested spelling. `Parser.parseContainer` automatic brace insertion is what lets a config that reads one way print another way without a diff on every commit.

**Behavior to change:** (only what the user asked for)
- (fill during design, gated on Q-2, Q-4, and Q-5)

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A config file path, or `-` for stdin, given to `ze config fmt` (`cmdFmt`, `internal/component/config/cli/cmd_fmt.go`).
- Format at entry: ze config text, in one of the three forms `DetectFormat` distinguishes.

### Transformation Path
1. `cliio.ReadFile` reads the bytes.
2. `config.YANGSchema` builds the schema from the registered YANG modules.
3. `config.NewParser(schema).Parse` tokenizes and builds a `*config.Tree`. Comments are dropped in `tokenizer.skipWhitespaceAndComments` and never reach the tree.
4. `config.Serialize` walks the schema children in order and writes canonical text.
5. `cmdFmt` compares input against output and acts on `-w`, `--check`, or `--diff`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator file ↔ parser | Text in, `*config.Tree` out. Anything the tree does not model is lost at this boundary | Yes, comment loss measured |
| Tree ↔ serializer | Schema order decides statement order, so printed order is a property of the YANG rather than of the file | Yes, measured |
| Repository ↔ gate | (fill during design) which population the gate reads, and which command it runs | No |

### Integration Points
- `config.Serialize` (`internal/component/config/serialize.go`) - the single canonical printer this spec must adopt or replace.
- `config.SerializeSet` (`internal/component/config/serialize_set.go`) - the set-form printer, a second surface form of the same tree.
- `SerializeTree` (`internal/exabgp/migration/migrate_serialize.go`) - the competing printer named in Q-4.
- The `.ci` runner's `tmpfs=` and `stdin=config` bodies - the fixture carriers named in Q-1.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | Q-4 records a live duplication this spec must resolve rather than add to |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

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
| A-1 | `config.Serialize` is idempotent over every construct the parser accepts | One sample held, measured by running `ze config fmt` twice | The formatter cannot be a gate, because a green run today goes red tomorrow with no edit | A property test over every `.conf` file in the tree | unvalidated |
| A-2 | Parse, format, parse yields a tree equal to the first | Not measured | `-w` silently changes the running config | A round-trip property test comparing trees, not text | unvalidated |
| A-3 | Comment loss is the only information the tree drops | Measured for comments. Blank lines and statement order are also unmodeled | An operator loses more than comments on `-w` | Diff a formatted corpus against its source and classify every removal | unvalidated |
| A-4 | The whole `.conf` corpus parses today | Not measured. The docs corpus demonstrably does not | The gate lands red and cannot be armed in one pass | Run `ze config validate` over every `.conf` in the tree and count | unvalidated |
| A-5 | One canonical form serves `.conf` files, `.ci` fixtures, and doc examples alike | The assumption behind Q-1 | The gate needs a per-carrier mode, and each mode is a second canonical form | Design decision, recorded against Q-1 | unvalidated |
| A-6 | The explicit-semicolon one-line form is refused by the parser, not by the schema | The main thread's run with a freshly built binary | The Task's stated cause is wrong, and a gate message would teach the wrong rule | Re-run with a binary built from the current tree, and read the producing function | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `ze config fmt -w` deletes an operator's comments today, and the command is already shipped and documented | `docs/guide/command-reference.md` teaches `-w` | Decide Q-5 before arming any gate that tells a reader to run `-w`. A gate that recommends a lossy command is worse than no gate |
| R-2 | Formatting the whole `.conf` corpus is a very large diff that collides with every session working this checkout | The completed `spec-fixit-peer-process-event-filter` rename touched about 500 files | Sequence the sweep, and never run it while another config-touching spec is open |
| R-3 | Changing the canonical form moves `ze config show`, the editor, and the `zefs` diff path at once | Golden files and `.ci` expectations across `test/` | Treat the printed form as a published interface. Any change to it is its own phase with its own evidence |
| R-4 | A gate over doc examples fires on intentional fragments and gets weakened until it proves nothing | Q-8. The first exclusion added for an ellipsis is the signal | Decide up front how a fragment declares itself, and never let the gate learn shapes by pattern. The four-round patching described in the Task is what that failure looks like |
| R-5 | Deleting the migration serializer changes `ze exabgp migrate` output | Migration golden files | Compare both printers over the migration corpus before deleting either |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An operator runs `ze config fmt -w` and loses the comments in a production config. A corpus-wide reformat lands a diff over hundreds of files. A canonical-form change moves every surface that prints config |
| How is it reverted? | The gate and the command changes revert as one commit. A corpus-wide reformat does not revert cleanly once other sessions have edited the reformatted files |
| Who else touches this path? | `spec-fixit-peer-process-event-filter` completed the BGP YANG and guide rename. No active work on that closed spec remains |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config fmt --check <file>` | → | `cmdFmt` in `internal/component/config/cli/cmd_fmt.go` | `test-config-fmt-check-exit-one` |
| The repository gate over every carrier | → | the make target chosen at Q-7 | `TestConfigCorpusIsCanonical` |
| A config example published under `docs/` | → | the doc-example reader chosen at Q-8 | `TestDocConfigExamplesParse` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any config in the repository the formatter accepts | Formatting it twice gives the same bytes as formatting it once |
| AC-2 | Any config in the repository the formatter accepts | Parsing the formatted text gives a tree equal to the tree parsed from the source |
| AC-3 | A config example published under `docs/` | The gate reads it and fails when the parser refuses it |
| AC-4 | (fill during design, gated on Q-5) A config carrying comments, formatted with `-w` | (fill during design) |
| AC-5 | (fill during design, gated on Q-4) One tree, printed by every producer of ze config text | Every producer gives the same bytes |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Copies a config example from a guide and runs `ze config validate` | docs example -> parser -> validator | `TestDocConfigExamplesParse` |
| 2 | Runs `ze config fmt -w` on a config carrying comments | file -> parser -> serializer -> file | (fill during design, gated on Q-5) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSerializeIsIdempotent` | `internal/component/config/serialize_test.go` | AC-1, over a construct table covering every node type the schema defines | |
| `TestFormatRoundTripsTree` | `internal/component/config/serialize_test.go` | AC-2, comparing trees rather than text | |
| `TestFormatPreservesComments` | `internal/component/config/serialize_test.go` | AC-4, gated on Q-5 | |
| (fill during design) | | | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Container inline depth (`maxInlineDepth` in `internal/component/config/serialize.go`) | 0-1 | 1 | N/A | 2, which would cascade inlining beyond one level |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-config-fmt-check-exit-one` | `test/parse/*.ci` | An operator runs `--check` on an unformatted file and gets exit 1 with the file named | |
| `test-config-fmt-idempotent` | `test/parse/*.ci` | An operator formats a file twice and the second run reports no change | |
| `test-config-fmt-comments` | `test/parse/*.ci` | (fill during design, gated on Q-5) | |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | N/A | N/A | Configuration syntax is a ze surface with no wire peer | N/A |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/component/config/cli/cmd_fmt.go` - the command whose contract this spec fixes
- `internal/component/config/serialize.go` - the canonical printer, if Q-2 or Q-5 changes the output
- `internal/component/config/tokenizer.go` - only if Q-5 requires comments to reach the parser
- `internal/exabgp/migration/migrate_serialize.go` - the competing printer named in Q-4
- `docs/architecture/config/syntax.md` - the published grammar, and the `// Design:` anchor of the files above
- `test/parse/cli-config-fmt.ci` - the current functional test, which asserts too little

## Files to Create
- `internal/le/` or an existing gate script - the check chosen at Q-7 (confirm the file during design)
- `test/parse/*.ci` - the functional tests named in the plan above

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | The formatter adds no config leaf. It reads the schema that exists |
| YANG validation constraints | No | No new leaf |
| YANG custom validators | No | No new leaf |
| CLI commands/flags | (fill during design) | `internal/component/config/cli/cmd_fmt.go`. The command exists, and Q-9 decides whether any flag changes |
| CLI grammar (keyword before value) | Yes | `ze config fmt <file>` already places a closed keyword before the value (`ai/rules/cli.md`) |
| Editor autocomplete | No | No new leaf and no dynamic value |
| Functional test for new RPC/API | Yes | `test/parse/*.ci`, listed above |
| Pipe completeness | (fill during design) | `cmdFmt` writes through `cliio`, not through `ApplyPipes`. Confirm whether formatted output should be pipeable |
| Env var registration | No | No `environment/` leaf |
| Doctor check for runtime dependencies | No | The formatter opens no socket, path, or service beyond the file named on the command line |
| Prometheus counters/metrics | No | A one-shot CLI command exposes no runtime state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol surface |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | (fill during design) | `docs/features.md`, only if the gate changes what an operator can rely on |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md`. Any canonical-form decision is published here |
| 3 | CLI command added/changed? | (fill during design) | `docs/guide/command-reference.md`, if Q-9 changes a flag |
| 4 | API/RPC added/changed? | No | No RPC |
| 5 | Plugin added/changed? | No | No plugin |
| 6 | Has a user guide page? | Yes | Every guide carrying a config example is in the gate's population |
| 7 | Wire format changed? | No | No wire surface |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC governs ze config syntax |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, because `.ci` fixtures are a carrier in Q-1 |
| 11 | Affects daemon comparison? | No | No feature claim changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/syntax.md`, and any subsystem doc if Q-4 deletes a serializer |
| 13 | Route metadata keys added/changed? | No | No metadata key |
| 14 | Prometheus counters added/changed? | No | No counter |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | (fill during design) | `docs/features/cli-commands.md` already lists `config fmt`. Confirm the row still matches |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Every file in `internal/component/config/` carries a `// Design: docs/architecture/config/syntax.md` annotation. Grep `docs/` for anchors on each changed file |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | This is the motivation. The guides listed in the Task each carry an example the parser refuses |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- make the gate reachable and red
   - Tests: `TestConfigCorpusIsCanonical`, `TestDocConfigExamplesParse`
   - Files: the gate target chosen at Q-7
   - Verify: the gate runs, reads the population Q-1 named, and fails on the known-bad guides
2. **Phase: Measure the corpus** -- answer A-1 through A-4 with numbers before changing any printer
   - Tests: `TestSerializeIsIdempotent`, `TestFormatRoundTripsTree`
   - Files: `internal/component/config/serialize_test.go`
   - Verify: idempotence, round-trip, and information loss each carry a count over the real corpus
3. **Phase: (fill during design)** -- resolve Q-2, Q-4, and Q-5
4. **Phase: (fill during design)** -- sweep the corpus and arm the gate

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named file and symbol |
| Feature completeness | Every carrier class in Q-1 is either in the gate or excluded with a stated reason |
| Correctness | The formatter loses nothing the source carried, or the loss is stated and accepted at Q-5 |
| Naming | One name for the canonical form, used in the command help, the guide, and the gate message |
| Data flow | One printer. A second producer of ze config text is a defect, not a variant |
| Rule: `ai/rules/no-layering.md` | The losing serializer is deleted, not wrapped |
| Rule: `ai/rules/evidence.md` | Every claim about what the parser accepts cites the producing function, never a doc example |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| The gate exists and is reachable | The make target chosen at Q-7, run against a deliberately broken example |
| Every `.conf` file in the tree parses | the corpus test's count, printed |
| Every config example under `docs/` parses | the doc gate's count, printed |
| One serializer remains | `grep -rn "SerializeTree" internal/` names exactly one producer of config text |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | The formatter parses untrusted text. A malformed file must give exit 2 and a line number, never a panic. `internal/component/config/fuzz_test.go` already fuzzes the parser; confirm it covers the formatter path |
| Secret leakage | `Parser.parseLeaf` decodes `$9$` values on sensitive leaves to plaintext. Confirm what `-w` writes back for a sensitive leaf, because a formatter that decodes a secret and prints it in the clear has downgraded the file |
| Fail closed | `--check` must exit non-zero when it cannot read or parse a file, and never exit 0 on a file it did not read |

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
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->
- A formatter is the only guard shape that reads every spelling of the language, because it is the parser. Every text-matching guard over this surface has been patched round after round and has still missed a shape.
- The tokenizer's automatic semicolon insertion is why a one-line block fails. It is also why the serializer can print statements with no terminator at all, so formatted output carries no semicolons.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| (fill during design) | | |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     needs a row in the deferral shard named in the metadata table. -->
- (fill during design)

## RFC Documentation (Scope: protocol)

N/A. No RFC governs ze configuration syntax.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify current mode full` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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
- [ ] `/ze-review` gate clean, recorded via `internal/le/speclifecycle/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
