# Spec: ze-config-fmt

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/templates/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | ready |
| Priority | P3 |
| Release reason | Repository-wide canonicalization and corpus gate are engineering work; operator-information preservation remains with its separate release-fix owner. |
| Scope | tooling |
| Depends | `plan/release/fixes/spec-config-list-entries-operator-order.md` (list-entry operator order), `plan/release/fixes/spec-config-yang-definition-order.md` (YANG definition order for leaves and containers). The gate must not lock in the old sorted form, so neither the gate phase nor the migration phase starts before both land |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-28 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later session reviews that commit and closes, on Opus when the model is an Anthropic one. `-` closes in the same session. -->

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     The file's DIRECTORY carries the release bucket: plan/immediate/ for a defect
     an operator meets, plan/pre-release/ for work the release cannot go out
     without, plan/ for everything else (plan/README.md). -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze needs one canonical text form for its configuration, and a gate that holds
every config in this repository to it. The original examples below record the
August 2026 motivation; they require current reproduction before being counted
as defects.

**A formatter already exists. This spec does not create one.** `ze config fmt`
is implemented in `internal/component/config/cli/cmd_fmt.go` (`cmdFmt`,
`configFmtBytes`), with `-w`, `--check`, `--diff`, and stdin support. It parses
with `config.NewParser` and prints with `config.Serialize`. This parent owns
canonical-form and repository-gate policy. Operator information preservation
was delivered by spec-config-fmt-preserves-operator-information, closed
2026-10-28: fmt keeps every comment and the meaning of every value, as
`docs/architecture/config/syntax.md` ("Comments") states, proven over the
corpus by `TestFormatCorpusRoundTrip`.
`ai/rules/no-layering.md` applies: a second formatter must not be added beside
this one.

G-2 and AC-4 remain required outcomes for this parent, discharged through that
immediate owner. The corpus gate must require its preservation evidence before
recommending writeback. The split changes ownership, not the no-information-loss
requirement, and Q-5 still needs a preservation design.

### Why this spec exists

`spec-fixit-peer-process-event-filter` renamed a config keyword across about
500 files. Its review surfaced two facts.

**1. Historical parser failures (2026-08-15).** The session recorded both forms
below as invalid. These observations predate the current tokenizer and keyword
spelling and do not establish that inline blocks are invalid today:

    attach process bgp-rr { receive [ update ] }      -> configuration invalid
    attach process bgp-rr { receive [ update ]; }     -> configuration invalid

Those forms appeared in `docs/guide/bgp-resilience.md`,
`docs/guide/plugins.md`, `docs/guide/flowspec-route-reflector.md` and
`docs/guide/flowspec-protected-router.md`. The review used them as evidence of
accepted syntax before checking the parser.

`docs/contributing/writing-style.md` now records both inline forms with
`update-received` as accepted. `tokenizer.scan` inserts a terminator before `}`,
as well as at newline and EOF. The earlier journal rows remain historical
evidence; each cited example must be rechecked before this spec claims it is
still refused.

**2. The corpus gate remains a design obligation.** The original review found no
repository-wide parser/formatter check over every carrier. The gate inventory
must be checked again during design. One config concept has many spellings:
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

## Research refresh (2026-10-28)

Thomas approved resuming research. This section supersedes the historical
current-behavior claims below. Design choices remain unapproved except for the
statement-order decision already recorded in Q-2.

Design approved 2026-10-28: the owner decision table under Open Questions
answers every Q-row, and the sections from Data Flow down are the approved
design. Where this research section and that table differ (generated configs),
the table wins.

### Current producers and constraints

| Surface | Source read | Finding and design constraint |
|---------|-------------|-------------------------------|
| Formatter entry | `internal/component/config/cli/cmd_fmt.go`, `configFmtBytes`, `fmtTree`, `cmdFmt` | Detects the text form, refuses set/set-meta with a conversion command, parses hierarchical input, hashes plaintext-password leaves, then calls `config.Serialize`. Preserve exits 0/1/2 and the current action flags. A corpus checker must not mistake credential normalization for information loss. |
| Hierarchical printer | `internal/component/config/serialize.go`, `Serialize`, `listBlockKeys` | Encodes sensitive values before printing. Only `ordered-by user` lists retain recorded entry order; other lists sort by key. Q-2 requires changing that latter policy, not inventing another formatter. |
| Comment association | `internal/component/config/parser_comments.go`, `commentScope.begin`, `headCount` | Comments survive. A blank line after opening comments makes them the block's head, not the following statement's comments. Q-2 requires changing this association as well as list order. The first-line schema stamp is a distinct existing constraint. |
| Existing preservation proof | `internal/component/config/cli/fmt_corpus_test.go`, `TestFormatCorpusRoundTrip` | Checks accepted hierarchical bodies for equal lowered maps, equal multisets of set lines, user-ordered list order, idempotence, and comment-text multisets. This is preservation evidence, not proof that every source parses or is canonical. Comment-text equality alone does not prove statement association. |
| Existing collector | Same file, `collectCorpusCarriers` | Walks `test/`, skips directories named `draft`, reads `.conf`, `.ci` tmpfs names ending `.conf`, and only stdin named `config`. Carrier parse errors and config parse errors are skipped. Docs, other stdin names, other suffixes, editor/web files, and runtime builders are outside this collector. Do not turn its successful-parse filter into the new gate's admission policy. |
| Migration printer | `internal/exabgp/migration/migrate_serialize.go`, `SerializeTree`, `serializeTreeIndent` | This is not only a printer: it wraps root groups in `bgp`, rewrites root plugin entries into internal/external blocks, and emits an empty `bgp` root when needed. `MigrateFromExaBGP`, `injectRIBPlugin`, and `migrateNeighbors` in `migrate.go` build that intermediate shape. A direct call substitution is unsafe; the migration must first produce a native schema-shaped tree. |
| Other migration output | `internal/exabgp/migration/env.go`, `MapEnvToZe`, `mapEnvKnownKey`; `internal/plugins/exabgp/main.go`, `cmdMigrateEnv` | INI environment migration also prints native config directly, including explanatory comments. AC-5 cannot claim every producer agrees if it covers only `SerializeTree`. |
| Carrier reader | `internal/test/tmpfs/tmpfs.go`, `Parse`, `parseWithLimits`, `parseTmpfsBlock`, `parseStdinBlock` | The existing reader extracts arbitrary named stdin and tmpfs bodies, including encoded forms. It does not declare them to be Ze config. Reuse this grammar rather than write another extractor from header patterns. |
| Dynamic carrier | `internal/test/fixture/misc_fixture_managed.go`, `managedConfig` | Builds text with runtime router IDs, ports, and a conditional rejection peer. Scanning source strings cannot establish the emitted config or cover both branches. Design must name the generated-output evidence instead of claiming static coverage. |
| Documentation gate | `internal/le/doc/check/actions.go`, `actions`; `internal/le/doc/wiring/docverify.go`, `docVerifyStages` | The inspected aggregate checks registries, source anchors, rules, and indexes; its stage list contains no config-body parse/canonical check. |
| Gate integration | `internal/le/verify/engine/stages.go`, `fullStages`, `fastStages`, `changedStages` | Full verification includes doc checks; fast derives from full. Changed verification selects lint, unit/static/race checks and functional suites instead. Q-7 must name the scoped route as well as the full route; adding a command alone does not arm a gate. |

### Observed CLI behavior

Built the current working tree through `./le job run label config-fmt-research-build
quiet command env CGO_ENABLED=0 go build`, targeting `./cmd/ze` with
`ze_core,ze_bgp,ze_exabgp`. The output binary was session-private. These probes
exercise the real CLI, not a test helper; they are not a full-corpus verdict.

| Input / command | Observed result |
|-----------------|-----------------|
| `ze config fmt -`, attach process with `receive [ update-received ]`, with and without an explicit semicolon | Both exit 0. Their formatted outputs are byte-idempotent and each passes `--check`. The August inline-block refusal is not current evidence. |
| `ze config fmt -`, peers entered as z then a, with a comment on z | Exit 0; a prints before z, and the comment stays on z. Output is byte-idempotent and passes `--check`. |
| `ze config fmt -`, blank-separated opening comment before router-id, followed by a peer | Exit 0; the comment remains at the block head, followed by a blank line and the peer. It does not follow router-id through schema ordering. Output is byte-idempotent and passes `--check`. |
| `ze config fmt -`, set-form input | Exit 2, names the set form and `ze config migrate format hierarchical -`. |
| `ze config fmt -`, unknown field under bgp | Exit 2 with a parse error. A discovery rule that only includes successful parses would hide this input. |
| `ze exabgp migrate -`, one neighbor with local-as and peer-as | Migration exits 0; its output parses through fmt but `fmt --check -` exits 1. |
| `ze exabgp migrate --env -`, log level and destination | Migration exits 0; `fmt --check -` exits 1. |

### Population and unresolved classification

`git ls-files -z` counted 1,067 tracked `.conf` files, including 1,015 under
`test/`, 38 under `demos/`, and two vendored files. It also counted 2,611
`.ci`, 179 `.et`, and 103 `.wb` files under `test/`, and 472 Markdown files
under `docs/`. These are file counts, not counts of Ze configs or valid examples.

The `.conf` population includes foreign languages: the inspected
`test/interop/scenarios/bgp-panic-cease/frr.conf` is FRR configuration.
`test/parse/config-fmt-set-refused.ci` deliberately feeds a set-form `.conf` to
fmt and expects exit 2. A bulk rewrite would destroy that test's input.

Q-1/Q-8 must distinguish native complete config, contextual fragments, deliberate
negative/noncanonical tests, foreign syntax, and generated output. The parser
must decide syntax; successful parsing must not decide which attempts are checked.
The current Markdown helper `markdownFence` in
`internal/le/doc/yangcontract/command_surfaces.go` handles fence delimiters but
is not a config-example extractor. Inline and table examples still need an
explicit carrier contract.

The old “four in five” fragment estimate is not a current measurement.
Annotation cost remains unmeasured until candidate discovery and fragment
classification are defined. Each justified opt-out costs a source annotation
and review; no silent skip may stand in for that decision.

### Research gate

The strongest risk is a falsely complete corpus gate: suffix-only discovery
misclassifies foreign files, successful-parse discovery hides broken examples,
and static text scanning misses generated configs. The research gate must
approve this baseline before choosing coverage and exception policy.

Research gate approved: Thomas selected “Proceed to design”.

### Approved documentation layout boundary

On 2026-10-28, Thomas answered “Yes—keep them inline” to “May valid config
examples stay compact inside sentences and table cells?” This confirms the
clarified question, not an implicit approval of the rest of the design.

Inline examples in prose and table cells must pass syntax checking, in their
declared context when they are fragments. Their compact layout is exempt from
canonical byte comparison. Separate config code blocks must match
`ze config fmt`. This qualifies G-3's layout requirement, not its validity
requirement. Incomplete examples still need the reasoned opt-out Q-8 requires.

### Approved carrier coverage

Thomas selected “Include generated configs too”: the scope includes saved
config files, embedded test fixtures, documentation examples, and configuration
text generated by code. Static scanning alone does not discharge G-4. The
design must identify how generated outputs are exercised and checked, without
claiming that reading a Go string literal proves its runtime substitutions or
conditional branches.


## Open Questions (decide at DESIGN)

All nine questions are answered: Thomas approved the design on 2026-10-28, and
each row below carries his decision after what was known. G-2 already requires
information preservation; Q-5 chose how to meet it and cannot make comments
optional.

| # | Owner decision (Thomas, 2026-10-28) |
|---|-------------------------------------|
| Q-1 | **Carriers:** `.conf` files; the config bodies inside `.ci` files (`tmpfs=` bodies and `stdin=config`); and config blocks in `docs/` that Ze's parser recognizes as a config attempt. **Not** configs that Go code assembles at runtime: there is no file to rewrite, and the tests that run that code already check them. This supersedes the earlier "Include generated configs too" selection in "Approved carrier coverage" above |
| Q-2 | **Normalize.** fmt rewrites to the canonical form (the `gofmt` answer). Statement order follows the Junos model recorded in the Q-2 cell. List-entry operator order is implemented by `plan/release/fixes/spec-config-list-entries-operator-order.md`; YANG definition order for leaves and containers by `spec-config-yang-definition-order.md`. This spec edits neither |
| Q-3 | **Reuse `config.Serialize`.** It stays the one printer |
| Q-4 | **Delete `internal/exabgp/migration/migrate_serialize.go`** (`SerializeTree`, `serializeTreeIndent`). `ze exabgp migrate` and `internal/plugins/exabgp/main.go` then print through `config.Serialize`. ExaBGP 6.0/5.0/4.2 compatibility is agreed: the migration output's meaning must not change, only its text. Every migration golden or expectation refresh goes to `ze-test-judge`. Delete first (`ai/rules/no-layering.md`) |
| Q-5 | **Delivered** by the closed spec-config-fmt-preserves-operator-information: `TestFormatCorpusRoundTrip` (`internal/component/config/cli/fmt_corpus_test.go`) and `docs/architecture/config/syntax.md`, "Comments" |
| Q-6 | **Yes.** Idempotence and round trip are the acceptance properties, already asserted by `TestFormatCorpusRoundTrip` |
| Q-7 | **A changed-file ratchet:** the gate checks only the carriers a commit touches, with no mass reformat and no flag day, through the existing changed-file-aware gate surface. Surface chosen: see Key Design Decisions |
| Q-8 | **A one-line opt-out marker above a doc block, stating its reason.** The gate reads every block the parser recognizes as a config attempt. Marker form and annotation cost: see Key Design Decisions |
| Q-9 | **Option-free.** No style flag is added; `-w`, `--check`, `--diff` select an action only |

| # | Question | What is already known |
|---|----------|-----------------------|
| Q-1 | **Scope of carriers.** `.conf` files only, or also `.ci` fixtures, `tmpfs=` bodies, fenced blocks in `docs/`, and config built by Go string builders? | 940 `.conf` files exist, 760 of them under `test/`. 395 `.ci` files carry a `tmpfs=<name>.conf` body and 346 carry a `stdin=config` body. 31 files under `docs/` carry a top-level `bgp {` line. The Go builders are the carrier class that defeated this spec's own motivating 500-file sweep |
| Q-2 | **Which spelling is canonical.** The parser accepts several spellings of one tree. Does the formatter normalize the others to the canonical one, or refuse them? | `config.Serialize` already picks one for every construct. Normalizing is the `gofmt` answer and it makes the gate a formatter run. Refusing is a stricter language and it turns every non-canonical file into an error rather than a rewrite. **Owner decision (2026-10-28), statement order:** the Junos model. Leaves and containers inside a block print in YANG schema order; the entries of every list print in the order the tree recorded (operator order), never sorted by key; blank lines are not kept, and a comment above a group stays with its first statement. `ordered-by user` lists already keep their order (`listBlockKeys`); the change is to stop sorting the other lists. The statement-order part (list entry order, and YANG child order) is implemented by `plan/release/fixes/spec-config-list-entries-operator-order.md`; blank lines and comment grouping stay here |
| Q-3 | **Agreement with the serializer.** `internal/component/config/flatten.go` and the `ze:flatten` extension already decide what ze prints for one construct. Does the formatter reuse `config.Serialize`, or replace it? | Reusing it is the only answer that keeps one canonical form. Replacing it means every other printer (`ze config show`, the editor, the `zefs` diff path) moves at the same time |
| Q-4 | **A second serializer already disagrees.** `internal/exabgp/migration/migrate_serialize.go` (`SerializeTree`, `serializeTreeIndent`) prints ze config text with its own rules: alphabetical key order, and quoting only when the value holds a space. `config.Serialize` uses YANG schema order and `quoteIfNeeded` quotes on space, tab, quote, apostrophe, brace, `;`, and `#`. Which one survives? | Two producers of one language is the layering this spec exists to remove. `ai/rules/no-layering.md`: delete the loser, do not wrap it |
| Q-5 | **Comment preservation mechanism.** How will the existing parser/formatter preserve comments and their association with configuration statements? | The immediate preservation spec owns this design and implementation. G-2/AC-4 remain required here; corpus policy may not waive them |
| Q-6 | **Idempotence and round-trip.** Format twice equals format once. Parse, format, parse again yields the same tree. Are these the acceptance properties? | They are the two properties that make a formatter testable without a golden file per construct. Neither is asserted anywhere today |
| Q-7 | **The surface.** `ze config fmt` and its `--check` mode exist. Does the gate run them, over which population, and does it live in `./le doc check all`, in `./le doc wiring`, or in a new target? | `ai/rules/repo-maintenance.md` owns whether a docs gate is worth building, and its Current Discovery Surfaces table lists the changed-file-aware gates that already exist. `ai/rules/cli.md` owns the grammar, and `config fmt` already satisfies action before identifier |
| Q-8 | **Elided and partial examples.** `docs/guide/plugins.md` shows `attach process rib { ... }` with a literal ellipsis, and `docs/guide/flowspec-route-reflector.md` shows a full block inside a markdown table cell rather than a fenced block. How does the gate see a fragment that was never meant to be a whole config? | A gate that reads only fenced blocks misses both. A gate that reads everything must be told how an intentional fragment declares itself. **Two constraints are already decided. They were retired into this cell from `ai/rules/points/writing/documentation/` on 2026-08-16.** First, the gate MUST read what ze's own parser recognizes as a config attempt, and it MUST carry an opt-OUT that states its reason on the block. A gate over every fenced block in `docs/` fires mostly on deliberate excerpts, an estimated four in five, because they start mid-tree or carry a placeholder. An opt-IN marker inverts the failure: every example that is already refused stays unmarked and uncaught, which is the `rpki.md` case exactly. Second, whoever proposes the gate MUST state that annotation cost, and MUST NOT sell the gate. Somebody annotates the excerpts one time, and each new excerpt pays one line |
| Q-9 | **Options.** `gofmt` normalizes with no configuration, and that is the reason it won. Does `ze config fmt` stay option-free? | Every style option is a second canonical form, so it is a second thing the gate must accept. The existing flags select an action, not a style: none of `-w`, `--check`, `--diff` changes the output text |

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - the published grammar, and the `// Design:` anchor of every file in `internal/component/config/`
  → Constraint: this doc is the operator-facing description of the language, so any canonical-form decision changes it. Its "Process Section" is the surface the motivating migration rewrote
  → Decision: the page states one canonical form, "what `ze config fmt` prints", in one paragraph (Phase 5); the order sentences belong to the two Depends specs, and the grammar sections keep listing every accepted spelling
  → Constraint (research refresh): Q-2 changes the documented system-list sorting and blank-separated head-comment behavior. Update both sections with the code change; preserve schema stamps and credential handling.
- [ ] `ai/rules/no-layering.md` - two serializers already print ze config text
  → Constraint: when replacing X with Y, delete X first. A formatter that leaves `internal/exabgp/migration/migrate_serialize.go` in place has added a layer, not removed one
- [ ] `ai/rules/repo-maintenance.md` - owns the gate question and the discovery surfaces
  → Constraint: the Mechanical Checklist must be answered in this spec: where an agent looks first, what rule prevents regression, what source of truth prevents drift, what verification proves it
- [ ] `ai/rules/cli.md` - command grammar
  → Constraint: a closed keyword precedes any user-supplied value. `ze config fmt <file>` already satisfies it, so no grammar change is expected
- [ ] `ai/rules/simplicity.md` - the option-free formatter is the simplest answer only if one canonical form serves every carrier
  → Constraint: Q-1 decides that, so Q-1 is answered before Q-9
- [ ] `docs/architecture/testing/ci-format.md` - stdin and tmpfs carrier grammar
  → Constraint: bodies are arbitrary bytes; neither `.conf` nor `stdin=config` establishes language or expected acceptance.
- [ ] `docs/contributing/documentation-testing.md` - existing doc and changed-file gates
  → Constraint: reuse native action registration and record how the new check reaches scoped verification, not only the complete doc action.
- [ ] `docs/contributing/writing-style.md`, “Config examples in docs”
  → Constraint: standalone examples use one statement per line; inline mentions can stay inline. Q-8 must reconcile that display contract with canonical comparison.
- [ ] `docs/exabgp/exabgp-migration.md` - migration command and compatibility contract
  → Constraint: preserve migrated process arguments, default values and warnings while replacing serialization; INI environment output is a producer too.
  → Decision: carry the existing converter semantics into a native tree, then print through `config.Serialize` (approved 2026-10-28, Q-4). The page shows no migrated ze config text (its fenced blocks are JSON, Python and shell), so it gains one sentence: the output is canonical ze config, and its text, never its meaning, changed.

- [ ] `docs/architecture/core-design.md` - the `// Design:` anchor of `internal/exabgp/migration/migrate_serialize.go`, `internal/exabgp/migration/env.go`, `internal/plugins/exabgp/main.go` and `internal/le/doc/wiring/sources.go`
  → Constraint: its ExaBGP bridge and migration text, and its changed-file gate routing text, describe the files Phase 3, 4 and 1 change. Deleting the migration printer and adding a selector row must leave no sentence there naming `SerializeTree` or the old routing; grep the page for both in the phase that changes them
  → Decision: the page keeps one statement of the migration contract (ExaBGP config in, native ze config printed by `config.Serialize` out); nothing else of it changes
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - the `// Design:` anchor of `internal/le/verify/engine/stages.go`
  → Constraint: adding a stage to `changedStages` changes the documented population of `./le verify changed`; the page names that population, so the stage and the page move in one commit
- [ ] `docs/contributing/documentation-testing.md` - "Adding a new documentation check" and the `./le doc wiring` router
  → Decision: the gate is a native action of its own registered through `leroot.Register`, selected by `./le doc wiring` for changed carriers (step 4 of that list: `selectedActions` in `internal/le/doc/wiring/sources.go`, `goActions` in `delegate.go`), and run by `./le verify changed` as its own stage
- [ ] `internal/le/doc/check/links.go` - the `doc-links: ignore (reason)` marker
  → Decision: the Q-8 opt-out reuses that convention's shape, `<tool>: ignore (reason)`, with the reason required and an empty reason refused, as `checkIgnored` and the "states no reason" refusal there do

### RFC Summaries (Scope: protocol)
- N/A. Configuration syntax is a ze surface, governed by no RFC.

**Key insights:** (minimal context to resume after compaction)
- `ze config fmt` exists and is parse-then-`config.Serialize`. This spec decides its contract, it does not build it.
- The August measurement lost comments. That defect is closed: spec-config-fmt-preserves-operator-information carries comments on the tree, proven by `TestFormatCorpusRoundTrip` (Q-5).
- The gate is `./le config canonical check`, a changed-file ratchet selected by `./le doc wiring` and staged in `./le verify changed`. It judges only carriers the change touches.
- `ze exabgp migrate` runs inside the `ze` binary (`cmd/ze/dispatch_exabgp.go`, `ze_core && ze_exabgp`), so `config.YANGSchema()` is callable at its print point, although the migration package itself holds only the ExaBGP input schema (`exaBGPSchema`, `internal/exabgp/migration/schema.go`).
- `tokenizer.scan` now inserts a semicolon at newline, EOF and before `}` after a value-ending token. The earlier inline-block diagnosis is obsolete.
- Two independent serializers print ze config text, and they disagree on ordering and on quoting.

## Current Behavior (MANDATORY)

**Historical baseline:** the entries and measurements below predate the research
refresh above. Do not use their comment-loss or “only functional test” claims
as current evidence.

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/config/tokenizer.go` - `tokenizer.next` sets `insertSemi` after a WORD, STRING, `]`, or `)`. `tokenizer.scan` emits the synthetic semicolon at newline, EOF or before `}`. `skipWhitespaceAndComments` consumes a `#` comment to end of line and produces no token, so no comment reaches the parser
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
- The current result of each original attach example. The old `receive [ update ]` spelling and the old binary cannot establish a present grammar defect. Design must reproduce examples against current YANG and the current tokenizer before naming a cause.
- Whether `config.Serialize` is idempotent and round-trip stable over the whole corpus. One sample held. Nothing measures the rest.
- Whether every `.conf` file in the tree parses today.

**Behavior to preserve:** (unless the user explicitly said to change it)
- `ze config fmt` exit codes: 0 success, 1 for `--check` with changes pending, 2 for a parse error. `test/parse/cli-config-fmt.ci` and `test/ui/dash-stdio.ci` read them.
- `config.Serialize` output is read back by `ze config show`, the editor, and the `zefs` diff path. A change to the printed form moves all of them.
- The parser keeps accepting both the flat and the nested spelling. `Parser.parseContainer` automatic brace insertion is what lets a config that reads one way print another way without a diff on every commit.

**Behavior to change:** (only what the user asked for)
- Canonical spelling, serializer consolidation and corpus policy remain this parent's design choices under Q-2/Q-4. G-2/AC-4 preservation and the Q-5 mechanism are implemented and proven by the immediate preservation dependency before this gate recommends writeback.

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

### Gate path (designed 2026-10-28)

Entry: `./le config canonical check` with no argument (the change set read
from git), `changed-file <path>` repeated (named files), or the same action
selected by `./le doc wiring` and staged by `./le verify changed`.

1. The change set and its changed lines come from the one anchor every
   changed-file gate uses: `repochanged.LinesSinceUpstream` and `PathsSince`
   (`internal/le/repo/changed/lines.go`), base resolved by `ResolveAnchor`.
   No base resolves: exit 2, the reason named, nothing judged.
2. Carrier discovery over the changed paths, by file kind:

| File kind | Carriers it holds | Judged when |
|-----------|-------------------|-------------|
| `*.conf` anywhere in the tree | the whole file is one carrier | the file is in the change set |
| `*.ci` under `test/` | each `tmpfs=` body whose path ends `.conf`, and each `stdin=config` body, read with `tmpfs.Parse` (`internal/test/tmpfs/tmpfs.go`), so encoded bodies decode the way the runner decodes them | a changed line falls inside the body or on its header line |
| `*.md` under `docs/` | each fenced block whose info string is empty or `text`, and each inline code span holding a `{` | a changed line falls inside the block, or on the span's line |

3. Recognition, decided by the schema, never by a hand list: a carrier is a
   config attempt when its first statement word is a child of the schema root
   (`config.YANGSchema()`), or is `set` or `delete` (set form, `DetectFormat`),
   or names a schema node at some context path where `Parser.ParseAt` accepts
   it (a fragment). A carrier that is none of these is reported by path as
   "not ze config" and not judged: this is how FRR and BIRD `.conf` files
   (`test/interop/scenarios/*/frr.conf`) and shell or JSON blocks stay out.
4. Opt-out: a carrier preceded by the marker (Key Design Decisions, Q-8) is
   not parsed; it is counted in the report with its reason. A marker whose
   parenthesised reason is empty is itself a refusal.
5. Verdict per attempt:

| Attempt | Syntax verdict | Canonical verdict |
|---------|----------------|-------------------|
| Hierarchical, root-starting (whole `.conf`, `.ci` body, fenced doc block) | `config.NewParser(schema).Parse` | output of the exported fmt core (today `configFmtBytes` in `cmd_fmt.go`) byte-equal to the input |
| Hierarchical fragment, fenced doc block | `Parser.ParseAt` at the first context path that accepts it | byte-equal to the canonical print of the smallest complete config holding it, cut back to the fragment's lines and its indentation |
| Set or set-meta form | `config.NewSetParser(schema).Parse` | none: set form is the editor's storage form, printed by `SerializeSet`, and `ze config fmt` refuses it with exit 2 |
| Inline span in prose or a table cell | parse as above | none, by the approved documentation layout boundary |

6. Report: counts per file kind (judged, accepted, refused, opted out with
   reason, not ze config), then one refusal per line, naming the carrier
   (`<path>:<line>`, the `.ci` body name), the parser's error or "not
   canonical", and the fix command (`ze config fmt -w <path>` for a `.conf`;
   for a body or a block, the command that prints its canonical form). Exit 0
   when nothing is refused, 1 when something is, 2 when a carrier could not be
   read or no base resolved.

### Migration path (designed 2026-10-28)

1. `cmdMigrate` (`internal/plugins/exabgp/main.go`) reads, parses with
   `ParseExaBGPConfig`, migrates with `MigrateFromExaBGP`, as today.
2. `MigrateFromExaBGP` returns a tree already in native schema shape: the root
   wrap into `bgp`, the plugin internal/external split and the empty-`bgp`
   root that `serializeTreeIndent` performs while printing move into tree
   construction (`migrate.go`, beside `injectRIBPlugin` and `migrateNeighbors`).
3. `cmdMigrate` loads `config.YANGSchema()`, applies the plaintext-password
   hashing every whole-tree CLI writer owes (`hashPlaintextPasswords`, whose
   comment already names migrate), and prints with `config.Serialize`.
4. `--env` (`cmdMigrateEnv`): `MapEnvToZe` builds a native tree carrying its
   explanatory comments as tree comments (the closed preservation spec's
   mechanism) instead of text, and the same print step runs.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator file ↔ parser | Text in, `*config.Tree` out. Comments ride the tree since the closed preservation spec | Yes, `TestFormatCorpusRoundTrip` |
| Tree ↔ serializer | Schema order decides statement order, so printed order is a property of the YANG rather than of the file | Yes, measured; changed by the two Depends specs |
| Repository ↔ gate | Changed carriers from git, recognized by the schema, judged in-process by the parser and the fmt core; `le` is the `ze` binary built with every feature tag plus `ze_le` (`./le` script, `feature_tags`), so the full schema is linked | Designed; proven by the Phase 1 and 2 tests |
| Migration tree ↔ serializer | `MigrateFromExaBGP` hands a native-shaped tree to `config.Serialize` | Designed; proven by Phase 3 tests |

### Integration Points
- `config.Serialize` (`internal/component/config/serialize.go`) - the single canonical printer, kept (Q-3).
- `config.SerializeSet` (`internal/component/config/serialize_set.go`) - the set-form printer; set carriers are parse-checked only.
- `SerializeTree` (`internal/exabgp/migration/migrate_serialize.go`) - deleted (Q-4).
- `MapEnvToZe` (`internal/exabgp/migration/env.go`) - stops printing text and returns a tree.
- `tmpfs.Parse` (`internal/test/tmpfs/tmpfs.go`) - the one `.ci` body reader, reused.
- `Parser.ParseAt` (`internal/component/config/parser.go`) - the existing fragment parser the editor's relative load uses, reused for doc fragments.
- `selectedActions` and `goActions` (`internal/le/doc/wiring/sources.go`, `delegate.go`) - one selector row and one table row.
- `changedStages` (`internal/le/verify/engine/stages.go`) - one stage.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The gate calls the same fmt core `ze config fmt` runs; it never reimplements parse or print |
| No unintended coupling (components stay isolated) | Yes | `internal/le` already links `internal/component/config` (`internal/le/doc/yangcontract/contract.go`); the migration library gains a dependency on `config.Serialize` only through its CLI caller |
| No duplicated functionality (extends existing, does not recreate) | Yes | The second printer is deleted; the carrier reader is `tmpfs.Parse`; the marker reuses the `doc-links` shape |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Offline tooling and a one-shot CLI; no hot path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The action registers through `leroot.Register`; recognition derives from the YANG schema root, so a new module's root node is recognized with no edit to the gate |

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
| A-1 | `config.Serialize` is idempotent over every construct the parser accepts | `TestFormatCorpusRoundTrip` asserts idempotence over every accepted hierarchical corpus body (Q-6) | The formatter cannot be a gate, because a green run today goes red tomorrow with no edit | `TestFormatCorpusRoundTrip`, rerun after both Depends specs land and again after Phase 3 | validated (corpus test), recheck after Depends |
| A-2 | Parse, format, parse yields a tree equal to the first | `TestFormatCorpusRoundTrip` compares lowered maps and set-line multisets | `-w` silently changes the running config | Same test, same reruns | validated (corpus test), recheck after Depends |
| A-3 | Comment loss is the only information the tree drops | Comments are now carried; blank lines are dropped by the Junos decision (Q-2), so their loss is the design, not a defect | An operator loses more than comments on `-w` | Closed preservation spec's corpus classification; Q-2 records blank lines as not kept | validated by Q-5/Q-2 decisions |
| A-4 | The whole `.conf` corpus parses today | Not measured against the current tree | A ratchet never needs it: a carrier is judged only when touched. It still decides how often a touched file costs a repair | `./le config canonical report` (Phase 2) prints the whole-tree counts once, without failing | unvalidated, measured in Phase 2 |
| A-5 | One canonical form serves `.conf` files, `.ci` fixtures, and doc examples alike | Q-1/Q-2 decisions: normalize everything to `config.Serialize` output; the only exception is the approved inline layout boundary, which waives layout, not syntax | The gate needs a per-carrier mode, and each mode is a second canonical form | Design decision, recorded against Q-1 and in the Gate path verdict table | validated by owner decision |
| A-6 | The August inline attach failures still reproduce with current keywords and schema | Historical binary output only | The motivating examples cannot serve as live failures | Current-tree CLI probes in Research refresh: both explicit and implicit terminators exit 0 | broken |
| A-7 | Migration already returns a native schema-shaped tree | `MigrateResult.Tree` uses the native Tree type | Replacing only the printer loses configuration | `injectRIBPlugin`, `migrateNeighbors`, and `serializeTreeIndent`: root plugin/group layout is adapted while printing | broken |
| A-8 | Successful parse is a sufficient admission rule for the new corpus gate | Existing preservation test skips config parse errors | Broken examples vanish from the gate's population | `TestFormatCorpusRoundTrip` source plus the unknown-child CLI probe; admission and syntax verdict must be separate | broken |
| A-9 | Static carrier extraction covers runtime-generated configs | Original sweep's stated goal | Branches and substituted values remain unchecked | Q-1 owner decision: runtime-assembled configs are not carriers; the tests that run that code check them | retired by owner decision (Q-1) |
| A-10 | The historical fragment estimate predicts annotation cost | Q-8's August estimate | Scope approval hides the cost of exclusions | Measured 2026-10-28 with an awk pass over tracked `docs/*.md`: 328 fenced blocks in 83 files begin with a word that names a top-level YANG `container` or `list` of a `-conf` module, and 39 of them carry `...`, `…` or a `<placeholder>`. That is a lower bound on root-starting attempts and an upper bound on the placeholder share; fragment starts and refusals for other reasons are counted by `./le config canonical report` in Phase 2 | broken (the "four in five" estimate does not hold for root-starting blocks); Phase 2 count replaces it |
| A-11 | The migration has the native YANG schema at its print point | `cmdMigrate` runs inside the `ze` binary (`cmd/ze/dispatch_exabgp.go`, `//go:build ze_core && ze_exabgp`), and `ze config fmt` in that binary already calls `config.YANGSchema()`; the research-refresh probe built with `ze_core,ze_bgp,ze_exabgp` parsed migration output through fmt. The migration package itself never holds it: its only schema is `exaBGPSchema` (`internal/exabgp/migration/schema.go`), the ExaBGP input grammar | A build with `ze_exabgp` but without `ze_bgp` links no BGP YANG, so `config.Serialize` would print the migrated `bgp` subtree through its unknown-key path, sorted, and the output would not parse in that binary | Phase 3: read `feature-gates.txt` and `./le repo feature-tags check` for whether `ze_exabgp` implies `ze_bgp`; build the `ze_core,ze_exabgp` combination and run `ze exabgp migrate` on one neighbor. If the BGP YANG is absent, `cmdMigrate` refuses with a message naming the missing tag rather than printing | unvalidated |
| A-12 | Moving the printer's adaptations into tree construction keeps the migration's meaning | `serializeTreeIndent` adapts root groups, plugin kinds and the empty `bgp` root while printing (A-7) | ExaBGP 6.0/5.0/4.2 users get a config that means something else | AC-13: the set-line multiset of the old and new output over the migration corpus, recorded before the deletion and compared after | unvalidated |
| A-13 | `Parser.ParseAt` can parse a doc fragment at a context path that passes through a list without a real key | `ParseAt` takes a token path and checks it with `LookupTokenPath` (`parser.go`) | Fragments under a keyed list (`peer`, `group`) cannot be judged in context | Phase 2 first test: a fragment under `bgp peer` parsed at a context built from the schema; if a key token is required, the gate uses the list key's YANG type to supply a valid placeholder, decided in that phase and recorded here | unvalidated |
| A-14 | The fmt core is deterministic over every judged carrier | `fmtTree` hashes `plaintext-password` with bcrypt before printing | A carrier holding a plaintext password never compares equal, so a touched fixture is refused forever | Phase 2 test over a body with `plaintext-password`; the canonical form of such a body is its hashed print, so the gate's message names `ze config fmt -w` as the fix, and a fixture that must keep plaintext (5 `.ci` files hold the word) carries the opt-out marker with that reason | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `ze config fmt -w` deletes an operator's comments today | `cmdFmt` writes `config.Serialize` output after the tokenizer has discarded comments | Repaired by spec-config-fmt-preserves-operator-information (closed 2026-10-28): comments ride the parsed tree and `config.Serialize` prints them (`docs/architecture/config/syntax.md`, "Comments"); `TestFormatCorpusRoundTrip` is the G-2/AC-4 evidence this parent's corpus gate waited on |
| R-2 | Formatting the whole `.conf` corpus is a very large diff that collides with every session working this checkout | The completed `spec-fixit-peer-process-event-filter` rename touched about 500 files | Removed by Q-7: no mass reformat and no flag day. The ratchet asks a carrier to be canonical only when a change touches it |
| R-3 | Changing the canonical form moves `ze config show`, the editor, and the `zefs` diff path at once | Golden files and `.ci` expectations across `test/` | Treat the printed form as a published interface. Any change to it is its own phase with its own evidence |
| R-4 | A gate over doc examples fires on intentional fragments and gets weakened until it proves nothing | Q-8. The first exclusion added for an ellipsis is the signal | Decide up front how a fragment declares itself, and never let the gate learn shapes by pattern. The four-round patching described in the Task is what that failure looks like |
| R-5 | Deleting the migration serializer changes `ze exabgp migrate` output | Migration golden files | The text changes by design (Q-4); the meaning must not. Record the old output's set-line multiset over the migration corpus in session scratch before the deletion (AC-13), delete, then compare |
| R-6 | Corpus writeback changes negative, noncanonical, or foreign fixtures | `config-fmt-set-refused.ci` expects fmt refusal; FRR fixtures use `.conf` too | Classify carriers with explicit reasons; preserve each test's intended input and expected refusal. Do not format the extension population blindly. |
| R-7 | Consolidation removes migration semantics hidden in its private printer | Root wrapping, plugin-kind selection, empty BGP root and family output live in `serializeTreeIndent` | Move conversion semantics into native tree construction; compare behavior through migration entry points before removing the printer. |
| R-8 | The corpus gate passes while omitting malformed examples or dynamic builders | Current collector skips unreadable bodies and covers only selected static carriers | Separate discovery, classification and parse verdict; report every unreadable candidate and name the generated-output proof. |
| R-9 | Applying Q-2 moves block-heading comments to the wrong statement or moves the schema stamp | `headCount` distinguishes blank-separated heads and the first-line stamp | Define group-comment association and stamp treatment independently; test both through `fmt -w`. |
| R-10 | Tree equality reports legitimate credential hashing as corruption or misses a credential loss | `fmtTree` hashes plaintext-password before serialization | Compare credential meaning through the existing hashing contract, keep secret text protected, and retain the preservation owner's tests. |
| R-11 | The gate lands before the Depends specs and locks the sorted order into every touched carrier, so their landing turns those carriers red again | A carrier refused as "not canonical" whose only difference is list-entry or child order | `Depends` gates Phase 1 on both ordering specs; the Phase 1 implementer confirms both are closed (`./le spec status`) before starting |
| R-12 | A touched negative fixture (a body a `.ci` expects ze to refuse) is refused by the gate, and the author "fixes" it by making it valid, destroying the test | A refusal on a `.ci` whose `expect=` lines assert a nonzero exit or a parse error | The refusal text says: if the body is meant to be refused, mark it with `# config-fmt: ignore (reason)` above its header; never edit the body. The opt-out is the declared route, and the judge reviews any fixture body change |
| R-13 | A misspelled root word in a ze `.conf` (`bpg {`) is read as "not ze config" and escapes the gate | A changed `.conf` listed under "not ze config" in the report | The report lists every such path by name, so it is visible, never silent. A test `.conf` is also loaded by the `.ci` that names it, which fails on the typo. Recorded in Known Limitations |
| R-14 | The marker shifts line numbers in a negative fixture whose test pins `line N` in an error | A `.ci` expectation naming a line after the marker was added | The `.ci` marker sits above the carrier header, outside the body. A standalone `.conf` marker may be the tail of an existing leading comment line, so no line is added |
| R-15 | Migration expectation refreshes weaken what the migration tests prove | A refreshed `Contains` string that drops a value, a `.ci` `expect=` that loses its operand | Every refresh goes to `ze-test-judge` (Q-4), the commit carries one approval line per test, and AC-13's set-line comparison is the meaning proof the judge reads |
| R-16 | A build with `ze_exabgp` and no `ze_bgp` prints migrated BGP config through the unknown-key path | A-11's probe | `cmdMigrate` refuses with the missing tag named, per A-11 |
| R-17 | Running the in-process parser over hundreds of touched carriers makes `./le doc wiring` slow | Wall clock of the selected action on a 500-file change such as the motivating rename | One `config.YANGSchema()` per run (it is cached); carriers parse in milliseconds; the Phase 2 test records the time for the largest `.ci` directory |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A wrong gate refuses commits that touch valid carriers, or passes broken ones. A wrong migration consolidation hands an ExaBGP user a config that means something else. No corpus-wide reformat exists to go wrong (Q-7) |
| How is it reverted? | The gate is one action, one selector row, one table row and one stage: reverting its commit disarms it. The migration change is its own commit (Phase 3, Phase 4), revertable alone |
| Who else touches this path? | `spec-fixit-peer-process-event-filter` completed the BGP YANG and guide rename. No active work on that closed spec remains |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config fmt --check <file>` | → | `cmdFmt` in `internal/component/config/cli/cmd_fmt.go` | `test/parse/cli-config-fmt-check.ci` |
| `./le config canonical check changed-file <path>` | → | `Answer` in `internal/le/config/canonical/canonical.go` | `TestCanonicalCheckRefusesChangedCarrier` (`internal/le/config/canonical/canonical_test.go`) |
| `./le doc wiring` over a change touching a `.conf`, `.ci` or `docs/*.md` | → | `selectedActions` row and `goActions` row in `internal/le/doc/wiring/` | `TestSelectedActionsPicksConfigCanonical` (`internal/le/doc/wiring/docwiring_test.go`) |
| `./le verify changed` | → | `changedStages` in `internal/le/verify/engine/stages.go` | `TestChangedStagesRunConfigCanonical` (`internal/le/verify/engine/stages_structural_test.go`) |
| A config example published under `docs/` | → | the Markdown carrier reader in `internal/le/config/canonical/markdown.go` | `TestCanonicalCheckReadsDocBlocks` (`internal/le/config/canonical/markdown_test.go`) |
| `ze exabgp migrate <file>` | → | `cmdMigrate` → `config.Serialize` | `test/parse/cli-exabgp-migrate-canonical.ci` |
| `ze exabgp migrate --env <file>` | → | `cmdMigrateEnv` → `config.Serialize` | `test/parse/cli-exabgp-migrate-env-canonical.ci` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any hierarchical corpus body the formatter accepts | Formatting it twice gives the same bytes as formatting it once. Proven by the existing `TestFormatCorpusRoundTrip` (Q-6), rerun after both Depends specs land |
| AC-2 | Any hierarchical corpus body the formatter accepts | Parsing the formatted text gives a tree equal to the tree parsed from the source (`TestFormatCorpusRoundTrip`) |
| AC-3 | A changed `docs/*.md` fenced block (empty or `text` info string) whose first word is a schema root node and which the parser refuses | `./le config canonical check` exits 1; the refusal names `<path>:<line>` of the block's first line and the parser's error text |
| AC-4 | A config carrying comments, formatted with `-w` | All comments and their association with configuration statements survive, configuration meaning is unchanged, and a second format changes no bytes. Delivered by the closed preservation spec (Q-5); this spec reruns its tests only |
| AC-5 | `ze exabgp migrate <file>` and `ze exabgp migrate --env <file>` over every fixture their tests use | The output passed to `ze config fmt --check -` exits 0: the migration prints exactly what `config.Serialize` prints for that tree |
| AC-6 | `SerializeTree` and `serializeTreeIndent` | No declaration and no caller remains anywhere in the tree (`internal/exabgp/migration/migrate_serialize.go` deleted); `MapEnvToZe` returns a tree, not text |
| AC-7 | A changed `.conf` whose text is a valid hierarchical config not in canonical form | Exit 1; the refusal names the path and the fix command `ze config fmt -w <path>` |
| AC-8 | An unchanged `.conf` in non-canonical form, in a change that touches another file | Not judged; exit 0 when nothing else is refused (the ratchet, Q-7) |
| AC-9 | A changed line inside a `.ci` `tmpfs=<name>.conf` body or `stdin=config` body that the parser refuses | Exit 1; the refusal names the `.ci` path, the body's name and the body line; a body whose header and lines are all unchanged is not judged |
| AC-10 | A carrier directly preceded by `<!-- config-fmt: ignore (reason) -->` (Markdown) or `# config-fmt: ignore (reason)` (`.ci` header line above, `.conf` leading comment) | Not parsed; the report counts it as opted out and prints the reason |
| AC-11 | The same marker with an empty reason, `()` or none | Exit 1; the refusal names the marker's line and asks for the reason |
| AC-12 | A changed fenced doc block whose first word is not a root node but which `Parser.ParseAt` accepts at a schema context (for example `attach process rib { ... }` with a real body) | Syntax accepted; canonical verdict computed against the smallest complete config holding it. A fragment no context accepts is refused naming the line |
| AC-13 | Every `.conf` under `test/exabgp-compat/etc/` that `ze exabgp migrate` accepts today | The set-line multiset (`config.SerializeSet` of the parsed output) is identical before Phase 3 and after Phase 4; any difference is listed and is a defect, never a refresh |
| AC-14 | A changed inline code span holding `{` in prose or a table cell, first word a root node | Parsed; refused only when the parser refuses it; its layout is never compared |
| AC-15 | A changed `.conf` whose first word names no schema node (an FRR file) | Not judged; listed by path under "not ze config" in the report |
| AC-16 | A changed set-form or set-meta carrier | Parsed with `config.NewSetParser`; refused only when that parser refuses it; no canonical comparison |
| AC-17 | No base resolves, or a changed carrier cannot be read | Exit 2, naming the reason; no "PASSED" line is printed (fail closed) |
| AC-18 | `ze config fmt --help` | Lists `-w`, `--check`, `--diff` and stdin only; no flag alters the printed text (Q-9) |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | A contributor edits a guide's config block and commits | changed doc -> `./le doc wiring` selects `config canonical/check` -> parser -> refusal or pass | `TestCanonicalCheckReadsDocBlocks`, `TestSelectedActionsPicksConfigCanonical` |
| 2 | Runs `ze config fmt -w` on a config carrying comments | file -> parser -> serializer -> file, preserving comments and configuration meaning | closed preservation spec's functional test (Q-5) |
| 3 | An ExaBGP user runs `ze exabgp migrate` and then `ze config fmt --check` on the result | ExaBGP file -> migration tree -> `config.Serialize` -> stdout -> fmt | `test/parse/cli-exabgp-migrate-canonical.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFormatCorpusRoundTrip` (existing) | `internal/component/config/cli/fmt_corpus_test.go` | AC-1, AC-2, AC-4 (Q-5, Q-6); rerun, not rewritten | Existing |
| `TestCanonicalCheckRefusesChangedCarrier` | `internal/le/config/canonical/canonical_test.go` | Wiring: a temp repo with one changed invalid `.conf` exits 1 naming it | |
| `TestCanonicalCheckRefusesNonCanonicalConf` | same | AC-7 | |
| `TestCanonicalCheckIgnoresUnchangedCarrier` | same | AC-8 | |
| `TestCanonicalCheckForeignConfNotJudged` | same | AC-15 | |
| `TestCanonicalCheckSetFormParseOnly` | same | AC-16 | |
| `TestCanonicalCheckFailsClosed` | same | AC-17: no base, unreadable carrier | |
| `TestCanonicalCheckPlaintextPasswordNeedsMarker` | same | A-14 | |
| `TestCanonicalCheckReadsCIBodies` | `internal/le/config/canonical/ci_test.go` | AC-9: changed body judged, unchanged body not, encoded body decoded through `tmpfs.Parse` | |
| `TestCanonicalCheckCIMarkerAboveHeader` | same | AC-10 for `.ci`, R-14 | |
| `TestCanonicalCheckReadsDocBlocks` | `internal/le/config/canonical/markdown_test.go` | AC-3: fenced block refused with line; `bash`/`json` block not read | |
| `TestCanonicalCheckDocFragmentInContext` | same | AC-12, A-13 | |
| `TestCanonicalCheckInlineSpanSyntaxOnly` | same | AC-14 | |
| `TestCanonicalCheckMarkerReason` | `internal/le/config/canonical/marker_test.go` | AC-10, AC-11 for all three carrier syntaxes | |
| `TestSelectedActionsPicksConfigCanonical` | `internal/le/doc/wiring/docwiring_test.go` | a changed `.conf`, `.ci`, `docs/*.md` each select `config canonical/check`; a `.go` file does not | |
| `TestChangedStagesRunConfigCanonical` | `internal/le/verify/engine/stages_structural_test.go` | `StagesForMode("changed")` holds the stage; full mode reaches it only through `doc wiring` (no double run) | |
| `TestMigrateOutputIsCanonical` | `internal/plugins/exabgp/main_test.go` | AC-5: `cmdMigrate` output equals `config.Serialize` of its reparse | |
| `TestMigrateEnvOutputIsCanonical` | same | AC-5 for `--env`, comments kept | |
| `TestMigrateTreeIsNativeShape` | `internal/exabgp/migration/migrate_test.go` | Phase 3: root `bgp` wrap, plugin internal/external split, empty `bgp` root built into the tree | |
| `TestMigrateMeaningUnchanged` | `internal/exabgp/migration/roundtrip_property_test.go` | AC-13 regression guard after the one-time comparison: set lines of the migrated tree over the compat corpus equal the expected set lines the judge approved | |
| `TestFmtFlagsSelectActionOnly` | `internal/component/config/cli/cmd_fmt_test.go` | AC-18 | |

**Migration expectations to refresh (Q-4 count, 2026-10-28).** Every one that
changes goes to `ze-test-judge`, one approval line per test in the commit.

| Surface | Files | Count read from the tree |
|---------|-------|---------------------------|
| Unit tests calling `SerializeTree` | 13 files in `internal/exabgp/migration/` (`migrate_test.go` 30 calls, `migrate_addpath_test.go` 10, eleven others 1 or 2) | 53 call sites; about 165 text assertion lines (`Contains`, `HasPrefix`, `Equal`, raw-string wants) read that output |
| Unit tests of `MapEnvToZe` | `internal/exabgp/migration/env_test.go` | 10 calls, 25 `Contains` assertions |
| Functional `.ci` asserting migration stdout | `test/parse/cli-exabgp-migrate.ci` (4 `expect=`), `cli-exabgp-migrate-env.ci` (7), `cli-exabgp-migrate-env-api-version.ci` (3) | 14 `expect=` lines; `contains=version 4;` cannot survive, because `config.Serialize` writes no semicolon |
| Fixture carrying migrated text | `test/plugin/rfc9774-as-set-accept-migrated.ci` (RFC-tagged; any change goes to the judge as `RFC-approved:`) | 1 embedded config |
| Script reading migration output | `test/exabgp-compat/bin/test-migrate` | checks the output parses, asserts no text; no refresh expected |

Total candidates: 53 call sites and about 190 unit assertions, 14 `.ci`
expectation lines, 1 RFC-tagged fixture. Call sites that only build the
`t.Fatalf` dump text are not expectations and need no judge. The phase
records the exact list it refreshed in this spec before it commits.

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
| `cli-config-fmt-check` | `test/parse/cli-config-fmt-check.ci` | An operator runs `--check` on an unformatted file and gets exit 1 with the file named; on the `-w` result, exit 0 | |
| `cli-config-fmt-idempotent` | `test/parse/cli-config-fmt-idempotent.ci` | An operator formats a file twice and the second `--check` reports no change | |
| `cli-exabgp-migrate-canonical` | `test/parse/cli-exabgp-migrate-canonical.ci` | `ze exabgp migrate` output passes `ze config fmt --check -` | |
| `cli-exabgp-migrate-env-canonical` | `test/parse/cli-exabgp-migrate-env-canonical.ci` | `ze exabgp migrate --env` output passes `ze config fmt --check -` and keeps its comments | |
| Preservation functional test | Closed preservation spec's test | Operator writeback retains comments and their statement association | Existing (Q-5) |

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
| File | Change | Phase |
|------|--------|-------|
| `internal/component/config/cli/cmd_fmt.go` | export the fmt core (`configFmtBytes` renamed to an exported name, same body) so the gate and `cmdFmt` run one function; and `hashPlaintextPasswords` reachable by `cmdMigrate` (exported or moved beside `config.Serialize`) | 1, 3 |
| `internal/le/doc/wiring/sources.go` | one `actionOrder` entry and one selector rule for `.conf`, `test/**/*.ci`, `docs/**/*.md` | 1 |
| `internal/le/doc/wiring/delegate.go` | one `goActions` row calling the new `Answer` | 1 |
| `internal/le/verify/engine/stages.go` | one `stage("config canonical", "check")` in `changedStages` | 1 |
| `internal/exabgp/migration/migrate.go` | `MigrateFromExaBGP` builds the native shape the deleted printer produced | 3 |
| `internal/exabgp/migration/migrate_serialize.go` | **deleted** (Q-4) | 3 |
| the 13 `internal/exabgp/migration/*_test.go` files calling `SerializeTree` | call `config.Serialize` through one test helper; refreshed expectations via the judge | 3 |
| `internal/plugins/exabgp/main.go` | `cmdMigrate` and `cmdMigrateEnv` print through `config.Serialize` | 3, 4 |
| `internal/exabgp/migration/env.go`, `env_test.go` | `MapEnvToZe` returns a tree with comments | 4 |
| `test/parse/cli-exabgp-migrate.ci`, `cli-exabgp-migrate-env.ci`, `cli-exabgp-migrate-env-api-version.ci`, `test/plugin/rfc9774-as-set-accept-migrated.ci` | expectation refresh only where text changed, via the judge | 3, 4 |
| `docs/architecture/config/syntax.md` | one paragraph: canonical form is what `ze config fmt` prints, and the gate holds touched carriers to it | 5 |
| `docs/architecture/core-design.md` | migration prints through `config.Serialize`; no second printer | 3 |
| `docs/architecture/testing/verify-freshness-scope.md` | the changed population gains the stage | 1 |
| `docs/contributing/documentation-testing.md` | the native action table row, the "When to run" row, the marker | 1, 5 |
| `docs/contributing/writing-style.md`, "Config examples in docs" | the gate enforces the paragraph's last rule; the marker and its reason | 5 |
| `docs/architecture/testing/ci-format.md` | the `.ci` marker line above a carrier header | 5 |
| `docs/exabgp/exabgp-migration.md` | output is canonical ze config; text, not meaning, changed | 3 |
| `ai/INDEX.md`, `ai/CODE-TO-DOCS.md`, `ai/DOCS-TO-CODE.md` | keyword row "config canonical gate", file-to-page rows for the new package | 5 |
| `test/parse/cli-config-fmt.ci` | unchanged; its three `contains` lines stay, the new `.ci` files add the missing proof | - |

## Files to Create
| File | Purpose | Phase |
|------|---------|-------|
| `internal/le/config/canonical/register.go` | `leroot.Register` of area `config canonical`, verbs `check` (gate) and `report` (whole-tree counts, never fails on refusals) | 1 |
| `internal/le/config/canonical/canonical.go` | `Answer`, change-set read, verdicts, report | 1, 2 |
| `internal/le/config/canonical/ci.go`, `markdown.go`, `marker.go` | carrier readers and the marker | 2 |
| `internal/le/config/canonical/*_test.go` | the unit tests above | 1, 2 |
| `test/parse/cli-config-fmt-check.ci`, `cli-config-fmt-idempotent.ci` | functional proof of `--check` and idempotence | 5 |
| `test/parse/cli-exabgp-migrate-canonical.ci`, `cli-exabgp-migrate-env-canonical.ci` | functional proof of AC-5 | 3, 4 |

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | The formatter adds no config leaf. It reads the schema that exists |
| YANG validation constraints | No | No new leaf |
| YANG custom validators | No | No new leaf |
| CLI commands/flags | Yes | No `ze` flag changes (Q-9). New `le` action `./le config canonical check` / `report` in `internal/le/config/canonical/register.go`, with `changed-file <path>` repeatable, mirroring `./le doc wiring`; `le help` and completion cover it through `leaction` registration |
| CLI grammar (keyword before value) | Yes | `ze config fmt <file>` already places a closed keyword before the value; `changed-file <path>` is keyword then value (`ai/rules/cli.md`) |
| Editor autocomplete | No | No new leaf and no dynamic value |
| Functional test for new RPC/API | Yes | `test/parse/cli-config-fmt-check.ci`, `cli-config-fmt-idempotent.ci`, `cli-exabgp-migrate-canonical.ci`, `cli-exabgp-migrate-env-canonical.ci` |
| Pipe completeness | N-A | `ze config fmt` and `ze exabgp migrate` are offline commands printing a file's text to stdout through `cliio`/`fmt.Print`; they are not daemon `show` commands, and a shell pipe already composes them |
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
| 1 | New user-facing feature? | No | The operator-visible change is `ze exabgp migrate` text; its contract page is `docs/exabgp/exabgp-migration.md` (row 17). The gate is contributor tooling, so no `features/<id>.md` level moves |
| 2 | Config syntax changed? | Yes | `docs/architecture/config/syntax.md`: one paragraph naming the canonical form as `ze config fmt` output. Grammar is unchanged; `docs/guide/configuration.md` gains nothing, as order changes are the Depends specs' pages |
| 3 | CLI command added/changed? | No | No `ze` command or flag changes (Q-9); `docs/guide/command-reference.md` line 171 stays true. The `le` action is documented in `docs/contributing/documentation-testing.md` |
| 4 | API/RPC added/changed? | No | No RPC |
| 5 | Plugin added/changed? | No | No plugin |
| 6 | Has a user guide page? | Yes | `docs/exabgp/exabgp-migration.md` (migration output). Guides carrying config blocks join the gate's population when touched; none is edited by this spec |
| 7 | Wire format changed? | No | No wire surface |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC governs ze config syntax |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/ci-format.md` (the `.ci` marker line), `docs/architecture/testing/verify-freshness-scope.md` (changed population), `docs/contributing/documentation-testing.md` (the action). `docs/functional-tests.md` gains one line pointing at the gate for `.ci` bodies |
| 11 | Affects daemon comparison? | No | No feature claim changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (the migration printer is gone), `docs/architecture/config/syntax.md` |
| 13 | Route metadata keys added/changed? | No | No metadata key |
| 14 | Prometheus counters added/changed? | No | No counter |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | A new `le` area registers (`config canonical`); `le help` lists it from the registry. `docs/features/cli-commands.md` row `ze config fmt` (line 116) still matches and is unchanged |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared `// Design:` pages of changed files: `docs/architecture/config/syntax.md` (`cmd_fmt.go`), `docs/architecture/core-design.md` (`migrate_serialize.go`, `env.go`, `main.go`, `sources.go`, `delegate.go`), `docs/architecture/testing/verify-freshness-scope.md` (`stages.go`); each is edited in the phase named in Files to Modify. Re-derive with `./le spec citation anchors spec plan/post-release/engineering/spec-ze-config-fmt.md` at the start of each phase |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/exabgp/exabgp-migration.md` shows no migrated ze config text (its fenced blocks are JSON, Python and shell), so only its contract sentence changes in Phase 3. The historical attach examples parse today (A-6 broken), so none serves as a failing fixture |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

Five phases, each sized at about 60 tool calls and landed as its own commit.
Precondition for Phase 1: both Depends specs closed (`./le spec status`), and
`TestFormatCorpusRoundTrip` green after them (A-1, A-2 recheck).

1. **Phase: Wiring (MANDATORY FIRST)** -- the gate exists, is reachable from all three entry points, and judges `.conf` carriers
   - Tests: `TestCanonicalCheckRefusesChangedCarrier`, `TestCanonicalCheckRefusesNonCanonicalConf`, `TestCanonicalCheckIgnoresUnchangedCarrier`, `TestCanonicalCheckForeignConfNotJudged`, `TestCanonicalCheckSetFormParseOnly`, `TestCanonicalCheckFailsClosed`, `TestSelectedActionsPicksConfigCanonical`, `TestChangedStagesRunConfigCanonical`
   - Files: `internal/le/config/canonical/register.go`, `canonical.go`; `cmd_fmt.go` (export the fmt core, no behavior change); `sources.go`, `delegate.go`; `stages.go`; `verify-freshness-scope.md`; the action row in `documentation-testing.md`
   - Inputs: change set and lines from `repochanged`; schema from `config.YANGSchema()`. Output: the report and exit 0/1/2 of the Gate path
   - Verify: each test red before its code, green after; a scratch change adding one invalid `.conf` makes `./le doc wiring` exit 1 naming it
2. **Phase: Carrier readers and the marker** -- `.ci` bodies, Markdown blocks and inline spans, doc fragments in context, the marker, the `report` verb
   - Tests: `TestCanonicalCheckReadsCIBodies`, `TestCanonicalCheckCIMarkerAboveHeader`, `TestCanonicalCheckReadsDocBlocks`, `TestCanonicalCheckDocFragmentInContext`, `TestCanonicalCheckInlineSpanSyntaxOnly`, `TestCanonicalCheckMarkerReason`, `TestCanonicalCheckPlaintextPasswordNeedsMarker`
   - Files: `ci.go`, `markdown.go`, `marker.go` and their tests
   - Constraint: `.ci` bodies come from `tmpfs.Parse`, never a header regex; a `.ci` that `tmpfs.Parse` refuses is a refusal (exit 2 class), not a skip
   - Verify: A-13 and A-14 resolved and their rows updated; `./le config canonical report` run once over the tree and its counts (A-4, A-10) written into this spec's assumption rows
3. **Phase: Migration printer deletion** -- delete `migrate_serialize.go` first, then build the native shape in `MigrateFromExaBGP` and print through `config.Serialize`
   - Before the delete: record, in session scratch, the set-line multiset of `ze exabgp migrate` output for every `.conf` under `test/exabgp-compat/etc/` (AC-13 baseline). This is a measurement, not a second path
   - Tests: `TestMigrateTreeIsNativeShape`, `TestMigrateOutputIsCanonical`, `TestMigrateMeaningUnchanged`, `test/parse/cli-exabgp-migrate-canonical.ci`
   - Files: `migrate_serialize.go` (deleted), `migrate.go`, `main.go` (`cmdMigrate`), `cmd_fmt.go` (`hashPlaintextPasswords` reachable), the 13 test files, `core-design.md`, `exabgp-migration.md`
   - Judge: every refreshed expectation in the migration count table goes to `ze-test-judge`; refresh only what the new text forces, never an operand
   - Verify: AC-13 comparison shows zero differences (any difference is a defect fixed in `migrate.go`); A-11 probe run and recorded; `grep -rn SerializeTree` empty
4. **Phase: Env migration through the tree** -- `MapEnvToZe` returns a tree with its comments; `cmdMigrateEnv` prints through `config.Serialize`
   - Tests: `TestMigrateEnvOutputIsCanonical`, refreshed `env_test.go`, `test/parse/cli-exabgp-migrate-env-canonical.ci`
   - Files: `env.go`, `env_test.go`, `main.go` (`cmdMigrateEnv`), the two env `.ci` files
   - Judge: the 25 env assertions and the 10 env `.ci` `expect=` lines, as changed
   - Verify: AC-5 for `--env`; every explanatory comment the old output carried is in the new output (comment multiset compared once and recorded)
5. **Phase: Functional proof, documentation and discovery**
   - Tests: `cli-config-fmt-check.ci`, `cli-config-fmt-idempotent.ci`, `TestFmtFlagsSelectActionOnly`, rerun `TestFormatCorpusRoundTrip`
   - Files: `syntax.md`, `writing-style.md`, `ci-format.md`, `documentation-testing.md` ("When to run"), `functional-tests.md`, `ai/INDEX.md`, `ai/CODE-TO-DOCS.md`, `ai/DOCS-TO-CODE.md`
   - Verify: `./le doc check links` and `./le doc index check` over the touched pages; `./le doc wiring` on the phase's own change set selects and passes the new action

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-18 has an implementation at a named file and symbol and a test from the TDD plan |
| Feature completeness | The three Q-1 carrier classes (`.conf`, `.ci` `tmpfs=*.conf` and `stdin=config`, `docs/` attempts) each have a reader and a test; runtime-built configs are absent by Q-1, not by omission |
| Correctness | The gate calls the same fmt core `cmdFmt` calls: no second parse or print path. Recognition derives from `config.YANGSchema()` roots and `ParseAt`, never from a list of words in the gate |
| Correctness, ratchet | An unchanged carrier is never judged; a changed body or block is judged whole, even when only one of its lines changed |
| Correctness, migration | AC-13 shows zero set-line differences; every refreshed expectation carries a judge approval line; the RFC-tagged `rfc9774-as-set-accept-migrated.ci` carries `RFC-approved:` if it changed |
| Fail closed | No base, an unreadable carrier, a `.ci` `tmpfs.Parse` refuses, or a schema load failure exits 2 and prints no PASSED line |
| Naming | One name, "canonical form", in `ze config fmt` help, `syntax.md`, the gate's area (`config canonical`) and its refusal text |
| Data flow | One printer. `grep -rn "SerializeTree\|serializeTreeIndent" internal cmd` is empty |
| Rule: `ai/rules/no-layering.md` | `migrate_serialize.go` is deleted in the commit that rewires `cmdMigrate`, before any new code depends on the new shape |
| Rule: `ai/rules/user-facing-errors.md` | Every refusal says what is wrong, why, and the exact command or marker that fixes it |
| Rule: `ai/rules/evidence.md` | Every claim about what the parser accepts cites the producing function, never a doc example |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| The gate exists and is reachable | `./le config canonical check changed-file <scratch invalid .conf>` exits 1 naming it; `./le doc wiring dry-run` lists `config canonical/check` for a changed `.conf` |
| The gate is staged for scoped verification | `TestChangedStagesRunConfigCanonical` passes |
| Whole-tree counts are known | `./le config canonical report` output, its counts copied into A-4 and A-10 |
| One serializer remains | `grep -rn "SerializeTree\|serializeTreeIndent" internal cmd` prints nothing |
| Migration meaning unchanged | AC-13 comparison output, zero differences, recorded in this spec |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | The formatter parses untrusted text. A malformed file must give exit 2 and a line number, never a panic. `internal/component/config/fuzz_test.go` already fuzzes the parser; confirm it covers the formatter path |
| Secret leakage | `Parser.parseLeaf` decodes `$9$` values on sensitive leaves to plaintext. Confirm what `-w` writes back for a sensitive leaf, because a formatter that decodes a secret and prints it in the clear has downgraded the file |
| Fail closed | `--check` must exit non-zero when it cannot read or parse a file, and never exit 0 on a file it did not read. The gate likewise (AC-17) |
| Secret echo | A gate refusal prints the path, line and parser error, never the carrier's text, so a fixture's key is not echoed into a gate log |
| Resource exhaustion | A `.ci` body is bounded by `tmpfs.Parse` limits (`parseWithLimits`); a Markdown block by the file size; no carrier is read twice |

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

- The second Design Insight above predates the current tokenizer: a one-line block parses today (A-6 broken); only its second sentence, that formatted output carries no semicolons, still holds, and it is why `contains=version 4;` in the env `.ci` cannot survive Phase 4.
- `ze exabgp migrate` has the native schema in reach (same binary) but the migration library does not: the print point, `cmdMigrate`, is the only place that should load it.

## Owned journal rows
<!-- Every open journal row this spec will fix, one line each; `./le commit create`
     refuses an open row no spec lists (plan/journal/README.md, "Owned rows"). -->
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| - | - | None today. The 2026-08-15 attach-example rows are historical, and their failures do not reproduce (A-6) |

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Inline documentation examples keep their compact layout; separate code blocks are canonical | Requiring formatter layout everywhere, moving examples out of sentences and tables | Approved documentation layout boundary above: syntax remains checked without breaking prose and tables. |
| Q-1 (owner): carriers are `.conf`, `.ci` `tmpfs=` and `stdin=config` bodies, and `docs/` blocks the parser recognizes; runtime-built configs are not carriers | Scanning Go string builders; generated-output capture | No file to rewrite, and the tests that run the builder already check its output (owner, 2026-10-28) |
| Q-2 (owner): normalize to the canonical form | Refusing non-canonical spellings | The `gofmt` answer: a gate failure has a one-command fix. Order is owned by the two Depends specs |
| Q-3 (owner): reuse `config.Serialize`, through the one exported fmt core | A gate-private print, a new formatter | One canonical form; `ai/rules/no-layering.md` |
| Q-4 (owner): delete `migrate_serialize.go`; migration prints through `config.Serialize`; meaning unchanged, text may change, every refresh to the judge | Wrapping `SerializeTree` around `config.Serialize`; keeping both | Two producers of one language is the layering this spec removes. The printer's adaptations move into tree construction because a direct substitution loses them (A-7) |
| Q-4 research: the schema is loaded at `cmdMigrate`, not in the migration library | Passing a schema into `MigrateFromExaBGP` | The library builds its tree under `exaBGPSchema`; only the CLI writer prints, and `config.YANGSchema()` is in the same binary (A-11) |
| `--env` is in Q-4's scope | Leaving `MapEnvToZe` as a text printer | It is the same command (`ze exabgp migrate --env`), and the research probe showed its output fails `fmt --check`. An agreed design answers what follows from it |
| Q-5 (owner): delivered by the closed preservation spec | A second preservation mechanism here | `TestFormatCorpusRoundTrip`; `syntax.md`, "Comments" |
| Q-6 (owner): idempotence and round trip are the acceptance properties | Golden files per construct | Already asserted by `TestFormatCorpusRoundTrip`; this spec adds no duplicate |
| Q-7 (owner) ratchet; surface: a new native action `./le config canonical check`, selected by `./le doc wiring` and staged in `changedStages` | A sub-check inside `docwiring.checker.run` like `ci-sleep-ratchet`; a whole-tree stage in `fullStages`; a `TestConfigCorpusIsCanonical` unit test | `./le doc wiring` is the documented changed-file-aware gate and its own page names step 4, "add the callable action to `internal/le/doc/wiring`", as the route. It runs in `fullStages` and `fastStages`, so the release check covers it with no new full-mode stage. `./le verify changed` does not run `doc wiring`, so the action joins `changedStages` by name, which judges only the change set as that list requires. A sub-check inside `run()` could not be run alone and would not reach `verify changed`. A whole-tree stage is the flag day Q-7 ruled out |
| Area name `config canonical` | `config fmt` | `./le config fmt` next to `ze config fmt` would read as the same command; the area is named for what it judges |
| Q-8 (owner) marker: `<!-- config-fmt: ignore (reason) -->` alone on the line directly above a fenced block, or at the end of the line holding an inline example; in a `.ci`, `# config-fmt: ignore (reason)` on the line directly above the `tmpfs=`/`stdin=config` header; in a `.conf`, the same `#` text in its leading comment block (whole line, or the tail of an existing leading comment line) | A new `<!-- ze-config: skip -->` form; an opt-in `ze` info string; a sidecar allowlist | Reuses the shape of the existing `<!-- doc-links: ignore (reason) -->` marker (`internal/le/doc/check/links.go`, 85 uses in `docs/`), including the parenthesised, required reason and the refusal when it is empty. An opt-in would leave every refused unmarked example uncaught (the Q-8 constraint). A sidecar list is a second declaration of what the file already says |
| Q-8 annotation cost, stated as owed | Selling the gate as free | Measured lower bound: 328 root-starting fenced blocks in 83 doc files, 39 with `...` or a `<placeholder>` (A-10). Under the ratchet, nobody annotates them up front: the author who next touches such a block pays one marker line, or makes the block complete. Each new excerpt pays one line. Fragment starts and blocks refused for other reasons add to that and are counted by `./le config canonical report` in Phase 2 |
| Recognition from the schema, fragments through `Parser.ParseAt` | Fenced blocks only; any block regardless of first word; a word list in the gate | The parser decides what is an attempt (Q-8 constraint); `ParseAt` is the editor's existing relative-load parser, so no new grammar. A word list is the central enumeration `ai/rules/principles.md` bans |
| Set-form carriers: syntax only | Canonical comparison against `SerializeSet` | `ze config fmt` refuses set form with exit 2; a second canonical comparison would be a second canonical form (Q-9) |
| Q-9 (owner): option-free | Style flags | Every style option is a second canonical form the gate must accept |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     needs a row in the deferral shard named in the metadata table. -->
- Runtime-assembled configs (`internal/test/fixture/misc_fixture_managed.go`, `managedConfig`, and every Go builder) are not carriers, by owner decision Q-1; the tests that run them check their output.
- The ratchet leaves untouched non-canonical carriers in place by design (Q-7): no carrier is reformatted until a change touches it.
- A ze `.conf` whose root word is misspelled reads as "not ze config" and is listed, not refused (R-13); the `.ci` that loads it fails instead.
- Fenced blocks with an info string other than empty or `text` are not read; a ze config block written as `bash` escapes until its fence is corrected. The report counts skipped blocks per info string so the escape is visible.

## Mechanical Checklist (`ai/rules/repo-maintenance.md`, discovery)

`ai/rules/repo-maintenance.md` no longer carries a "Current Discovery Surfaces"
table or a "Mechanical Checklist" heading (read 2026-10-28; only
`docs/features/ai-first.md` cites the old heading in an anchor). The four
questions the `ze-spec` skill names are answered here:

| Question | Answer |
|----------|--------|
| Where does an agent look first? | `ai/INDEX.md` keyword row "config canonical gate" (Phase 5), pointing at `docs/contributing/documentation-testing.md`, whose native action table gains the `./le config canonical check` row (Phase 1) |
| What rule prevents regression? | The gate itself, run by `./le doc wiring` (full, fast) and `./le verify changed`; `docs/contributing/writing-style.md`, "Config examples in docs", already states that every docs example parses, and Phase 5 names the gate there |
| What source of truth prevents drift? | The YANG schema (`config.YANGSchema()`) decides recognition; `config.Serialize` decides the canonical bytes; `tmpfs.Parse` decides `.ci` body extraction. The gate holds no list of its own |
| What verification proves it? | `TestSelectedActionsPicksConfigCanonical`, `TestChangedStagesRunConfigCanonical` and the canonical package tests; `StagesForMode` registration per the first directive of `ai/rules/repo-maintenance.md` (the stage joins `changedStages`; `fullStages` and `fastStages` reach it through `doc wiring`) |
| Generated files to resync? | None: the action registers at `init()`; `ai/INDEX.md`, `ai/CODE-TO-DOCS.md`, `ai/DOCS-TO-CODE.md` are hand-edited maps |

## Spec Independence Test (2026-10-28)

| Question | Answer |
|----------|--------|
| Could another session implement this without more context? | Yes. The one external input is whether both Depends specs have closed, which the Phase 1 precondition checks with `./le spec status` |
| Can each AC be tested from its text alone? | Yes: each names the input, the command and the exit code or text. AC-12's "smallest complete config" depends on A-13, which Phase 2 resolves first |
| Are each step's inputs, outputs and constraints stated? | Yes: files, tests, judge duty and the verify line per phase |
| Anything discussed but not captured? | No: the brief's nine answers are the owner decision table and Key Design Decisions; the gate surface, the marker and the schema finding are recorded with their evidence |
| Are all concerns captured as A-N or R-N rows? | Yes: A-11 to A-14 and R-11 to R-17 were added for this design |

## RFC Documentation (Scope: protocol)

N/A. No RFC governs ze configuration syntax.

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

Pre-Spec Verification run 2026-10-28 (results; boxes stay `[ ]` by rule):

| Item | Result |
|------|--------|
| Metadata | Status `ready`, Depends names both ordering specs, Phase `-`, Updated 2026-10-28 |
| `ai/INDEX.md` | Checked: no row names `config fmt` or a config canonical gate today; Phase 5 adds one |
| RFC summaries | N-A: no RFC referenced |
| Format | Tables throughout; checkboxes all `[ ]` |
| No code snippets | Only the two historical indented sample lines in "Why this spec exists", which are input/output text samples, not code |
| Files to Modify | Names `cmd_fmt.go`, `migrate.go`, `main.go`, `env.go`, `sources.go`, `delegate.go`, `stages.go` and a new package |
| Current Behavior, Data Flow | Filled; Data Flow gains the Gate path and Migration path |
| ACs | AC-1..AC-18, each with input, command and observable result |
| Assumptions and risks | A-1..A-14 each with basis and validation; R-1..R-17 each with signal and mitigation |
| Required Reading | Every entry carries a `→ Decision:` or `→ Constraint:`; `core-design.md` named (validate-spec hook) |
| Integration Checklist | CLI grammar answered Yes; Doctor No (no runtime dependency) |

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits. The full verification is Thomas's release check: no agent runs it
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
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
