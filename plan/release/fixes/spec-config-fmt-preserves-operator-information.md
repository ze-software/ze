# Spec: config-fmt preserves operator information

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/templates/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P0 |
| Release reason | Formatter writeback must not destroy operator comments or configuration meaning. |
| Scope | cli, config |
| Depends | - |
| Phase | 3/3 |
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

Make `ze config fmt -w` safe for an operator's file. This task inherits G-2
and AC-4 from `plan/post-release/engineering/spec-ze-config-fmt.md`: formatting must never
lose information the file carried. Preserve all comments and their association
with configuration statements, preserve configuration meaning, and make a
second formatting pass byte-identical to the first.

The current `tokenizer.skipWhitespaceAndComments`
(`internal/component/config/tokenizer.go`) discards comments before the tree is
built. `cmdFmt` (`internal/component/config/cli/cmd_fmt.go`) serialises that tree
and writes it over the operator's file. The design must address that source of
information loss and investigate the other information the current tree does
not represent. It must reuse the existing formatter rather than add a second one.

Retain the existing command actions, exit-code contract and supported input
forms. Prove preservation through operator-visible writeback, including comments
attached to statements and parse/format/parse equivalence. The design below
carries comments on the parsed tree, fixes the quoting defect that changes a
value's meaning, and proves both over the repository's config corpus. The
evidence is in Goal Validation and Pre-Commit Verification below.

**Information this spec preserves:** every comment, byte for byte after its
`#`, attached to the statement it was written beside; the schema stamp on the
first line; the configuration meaning (every value, every `inactive:` flag, the
order of every ordered leaf-list and `ordered-by user` list).

**Presentation the canonical form owns, not preserved here:** blank lines
(except the one that detaches a head comment group), the order of unordered
sibling statements, indentation, quote style, block versus inline spelling,
explicit semicolons, and number spelling (`065001` prints `65001`). The parent
spec decides these under its Q-2, and this design works under either answer
because a comment travels with its statement, not with its line number.

The parent keeps canonical spelling, serializer consolidation and corpus-carrier
gate policy. Its gate must not recommend writeback until this defect's
preservation evidence is available. Those policy choices cannot waive G-2.
This split records immediate-defect ownership without authorising new scope.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - the published grammar; the `// Design:` anchor of `cmd_fmt.go` and of the config package. Its Schema Stamp section says the stamp "is a comment (ignored by all parsers)" and that `ScanStampRelease` reads it from the first line
  → Constraint: the stamp line must stay the first line of a formatted file, or fmt silently turns a stamped file into an unstamped one
  → Decision: a new "Comments" subsection states the attachment rules below, and the Schema Stamp section says fmt keeps the stamp
- [ ] `docs/architecture/config/tokenizer.md` - token table lists `#` as "Comment (rest of line ignored)"
  → Constraint: the only comment form is `#` to end of line. There is no block comment, so none is designed
  → Decision: the row changes to say the comment is kept as trivia for the parser to attach
- [ ] `docs/config-migration.md` - the operator page for `ze config fmt` (its `// Design:` source anchor names `cmd_fmt.go`)
  → Decision: the fmt section states that fmt keeps comments and meaning and normalizes presentation, and names the set-format refusal and its fix command
- [ ] `docs/guide/configuration.md` - the syntax table row "Comments: `#` to end of line"
  → Decision: the row says comments survive `ze config fmt`; the editor commit still drops them (owned journal row in `spec-web-cli-config-one-off-defects`)
- [ ] `docs/architecture/testing/ci-format.md` - `expect=file:path=<rel>:contains=`, `expect=stdout:regex` exist, so a `.ci` can assert the written file and adjacency
  → Constraint: stream assertions are file-level over all commands, so each needle is unique to its command
- [ ] `ai/rules/config.md` - "Config content MUST be manipulated by ... a parsed YANG tree ... Raw text surgery ... and any manipulation that infers structure from text patterns MUST NOT be used"
  → Decision: comments are carried on the parsed tree, never spliced back into the output by line number
- [ ] `ai/rules/no-layering.md` and `ai/rules/simplicity.md`
  → Constraint: one formatter, one printer. `cmdFmt` stops duplicating `configFmtBytes`; no second serializer is added, `config.Serialize` prints comments itself
- [ ] `ai/rules/cli.md` - exit codes and flags are a contract
  → Constraint: `-w`, `--check`, `--diff`, stdin `-`, exit 0/1/2 are unchanged
- [ ] `plan/post-release/engineering/spec-ze-config-fmt.md` - the parent: G-2, AC-4, Q-2 (canonical spelling), Q-4 (second serializer), Q-5 (this spec's mechanism)
  → Constraint: canonical-form choices stay with the parent; its corpus gate waits for this spec's evidence

### RFC Summaries (Scope: protocol)
- N/A. Configuration syntax is a Ze surface governed by no RFC.

**Key insights:** (minimal context to resume after compaction)
- Comments die in `tokenizer.skipWhitespaceAndComments`; nothing downstream can recover them.
- `quoteIfNeeded` prints a value holding `]`, `[`, a newline or a carriage return bare, so fmt writes a file it then refuses: a meaning loss independent of comments.
- Every persistent writer except fmt and migrate prints set or set-meta text, which has no comment spelling. That is a separate defect, owned elsewhere.

## Current Behavior (MANDATORY)

Disposition check at `e39f85f43e` reproduced every loss below with the `bin/ze`
built 2026-10-27 from that tree.

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/config/tokenizer.go` - `skipWhitespaceAndComments` consumes `#` to end of line and returns only whether a newline was crossed (for automatic semicolon insertion). `readString` decodes `\n`, `\t`, `\\`, `\"`, `\'`. `endsWord` ends a bare word at space, tab, newline, carriage return, braces, brackets, `;`, quotes and `#`. `scan` reads a leading `(` or `)` as a paren token
- [ ] `internal/component/config/parser.go` - `Parser.Parse`, `parseBody`, `parseNode`, `parseLeaf`, `parseContainer`, the `inactive:` paths (`parseNodeInactive`, `parseInactiveValueOrArray`, `parseInactiveBracketMember`). Unknown keys are refused with a hint, so fmt never rewrites a file holding one. There is no include statement
- [ ] `internal/component/config/tree.go` - `Tree` holds values, `valuesOrder`, `multiValues`, inactive flags, containers, lists and `listOrder`. `Clone`, `MergeContainer`, `RemoveListEntry`, `RenameListEntry` move statements. No field holds text that is not configuration
- [ ] `internal/component/config/serialize.go` - `Serialize` walks schema child order (`serializeTree`, `serializeNode`), `serializeExtraValues` sorts unknown-allowed keys, `canInlineContainer` and `serializeContainerInline` collapse one-leaf containers onto one line
- [ ] `internal/component/config/flatten.go` - `canFlattenContainer`, the `ze:flatten` print-time spelling
- [ ] `internal/component/config/list_key.go` - `quoteIfNeeded` quotes only on space, tab, `"`, `'`, braces, `;`, `#` and the empty string; 103 non-test callers, the set printer among them
- [ ] `internal/component/config/prune.go` - `pruneUnchangedListEntries` drops an entry when two `SerializeSubtree` strings are equal
- [ ] `internal/component/config/meta.go` - `MetaTree`, the side tree of set-meta edit records keyed by path
- [ ] `internal/component/config/stamp.go` - `FormatSchemaStamp`, `ScanStampRelease` (first line only)
- [ ] `internal/component/config/cli/cmd_fmt.go` - `cmdFmt` repeats the body of `configFmtBytes` instead of calling it, so the unit tests in `main_test.go` never run the command's own path. `-w` writes through `cliio.WriteFile` only when the output differs
- [ ] `internal/component/config/cli/cmd_migrate.go` - reads set and set-meta through `DetectFormat` and `NewSetParser`, writes `Serialize` for `--format hierarchical`, else `SerializeSet`
- [ ] `test/parse/cli-config-fmt.ci` - three `contains` lines, nothing on comments or idempotence
- [ ] `test/ui/config-fmt-write-stdout.ci` - the `-w` in-place and `-w -` arms

**Measured losses, `ze config fmt <file>` at HEAD:**

| Input carried | Output | Class |
|---------------|--------|-------|
| `# ze-schema: 26.10.07` first line, header, leading, trailing, end-of-file comments | All absent | Information (this spec) |
| `description "x]y(z"` | `description x]y(z`; a second fmt fails `expected ';' after description value, got RBRACKET`, exit 2 | Meaning (this spec) |
| `description "l1\nl2"` | the value printed bare across two lines | Meaning (this spec) |
| `router-id` before `peer p1`; `connection` after `session` | schema order | Presentation (parent Q-2) |
| blank lines | removed | Presentation (parent Q-2) |
| `'single quoted'` | `"single quoted"` | Presentation |
| `remote { ip 192.0.2.1; }` | `remote ip 192.0.2.1` | Presentation |
| `remote 065001` | `remote 65001` | Presentation, same value |
| `inactive: peer p2 { ... }`, `filters [ zz-second aa-first ]` | kept, order kept | Preserved already |
| a set-format file, the form the editor stores | `unknown top-level keyword: set`, exit 2 | Refusal without a fix (owned row) |

**Behavior to preserve:** (unless the user explicitly said to change it)
- `ze config fmt` flags `-w`, `--check`, `--diff`, stdin `-`, and `-w -` emitting to stdout. Exit 0 success, 1 for `--check` with changes, 2 on any error. `test/parse/cli-config-fmt.ci`, `test/ui/config-fmt-write-stdout.ci` and `test/ui/dash-stdio.ci` read them.
- Hierarchical input only, as today; set input stays refused, now by name.
- `config.Serialize` output for a tree carrying no comments stays byte-identical, except values that today print unparseable text.

**Behavior to change:**
- Comments survive fmt, attached to their statements.
- `quoteIfNeeded` quotes every value the tokenizer would not read back as the same single token.
- `cmdFmt` calls `configFmtBytes`; a set-format input is refused with the converting command.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze config fmt [-w|--check|--diff] <file|->` (`cmdFmt`, `internal/component/config/cli/cmd_fmt.go`).
- Format at entry: hierarchical Ze config text, `#` line comments allowed anywhere whitespace is.

### Transformation Path
1. `cliio.ReadFile` reads the bytes; `config.DetectFormat` refuses set and set-meta input by name.
2. `configFmtBytes` builds the schema and calls `config.NewParser(schema).Parse`.
3. The tokenizer returns each `#` comment as trivia beside the next token, with whether a blank line or a newline preceded it; automatic semicolon insertion is unchanged.
4. The parser attaches each comment to its statement on the `Tree` under the rules in Key Design Decisions.
5. `config.Serialize` prints statements in canonical order and writes each statement's comments around it; `quoteIfNeeded` quotes any value that would not read back as itself.
6. `cmdFmt` compares and acts on `-w`, `--check`, `--diff` exactly as today.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator file to tokenizer | Text in, tokens plus comment trivia out | Measured: today comments are dropped here |
| Tokenizer to parser to `Tree` | Trivia attached to the statement node | No: designed here |
| `Tree` to `config.Serialize` | Schema-order walk prints comments; inline and flatten refuse a container whose inside carries one | No: designed here |
| `Tree` to set printers | Comments not printed (no spelling); owned journal row | Read |

### Integration Points
- `config.Tree` - gains the comment trivia; `Clone`, `MergeContainer`, `RemoveListEntry`, `RenameListEntry`, leaf removal keep it consistent.
- `config.Serialize`, `config.SerializeSubtree` - the one hierarchical printer; prints comments.
- `prune.go` `pruneUnchangedListEntries` - compares entries without comments, so a comment never makes an unchanged entry look changed.
- `quoteIfNeeded` - shared by every printer, so the meaning fix reaches the set writers too.
- `internal/test/tmpfs.Parse` - extracts `.ci` config bodies for the corpus test.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| One formatter, one printer | Yes | `config.Serialize` gains comments; `cmdFmt` calls `configFmtBytes`; no second walker |
| No text surgery (`ai/rules/config.md`) | Yes | Comments ride on the parsed `Tree`, never re-inserted by line number |
| No layering | Yes | The comment-dropping branch of `skipWhitespaceAndComments` is replaced, not wrapped |
| Registration over hardcoding | N-A | No new feature registry; the printer is schema-driven already |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|------------|---------------------------------|----------|--------------|--------|
| A-1 | `#` to end of line is the only comment form | `skipWhitespaceAndComments`; `tokenizer.md` token table | A block comment form is lost | `TestTokenizerCommentTrivia` covers every `#` position | confirmed by reading |
| A-2 | No include statement exists, so one file is one tree | `parser.go`, `loader.go` carry no include keyword | An included file's comments need their own carrier | grep at Phase 2 start | confirmed by grep (phase 2 start, 2026-10-28: no `include` keyword in `parser*.go`, `loader*.go` or `tokenizer.go`; the one hit is a comment in `parser_freeform.go`) |
| A-3 | Comments inside a bracket list can only follow `[` on its line: a newline inside the brackets after an item is refused by automatic semicolon insertion | Probe: `filters [ # c` newline `zz aa ]` parses, `[ zz` newline `aa ]` is refused | A comment between items needs a member-level anchor | `TestParserBracketComment` | confirmed by probe |
| A-4 | The set parser's `tokenizeLine` reads a value `quoteIfNeeded` now quotes (`]`, newline) back as one value | `setparser.go` `tokenizeLine` | The meaning fix reddens the set round trip | `TestQuoteRoundTripSet` | broken (phase 1): `tokenizeLine` decoded no escape, so `"l1\nl2"` and `"a\"b"` read back changed, and `strings.TrimSpace` on each line cut a trailing `\v`, `\f` or Unicode space from a bare value. Fixed at the reader: `tokenizeLine` deleted, both set parsers call `splitQuotedTokens` (the inverse of `quoteIfNeeded`), lines trimmed by `trimTokenSpace`. An unterminated quote in a set line is now refused with its line number |
| A-5 | Every `.conf` and `.ci` config body that parses today still parses after this change | No grammar change; comments were already accepted everywhere whitespace is | A carrier starts failing | `TestFormatCorpusRoundTrip` counts parses before and after | confirmed (phase 3, 2026-10-28): 1104 of 1911 hierarchical carriers parse, as at phase 1, and every one formats with its 3460 comments kept |

### Risks
| ID | Risk | Mitigation |
|----|------|------------|
| R-1 | A comment that changes printed text makes `prune` keep an unchanged list entry, so change files carry no-op entries | `pruneUnchangedListEntries` compares without comments; `TestPruneIgnoresComments` |
| R-2 | A leaf's comment survives the leaf's removal and reattaches to a later leaf of the same name | Leaf removal deletes the leaf's comments; `TestTreeLeafRemovalDropsComments` |
| R-3 | Inline or flattened printing has no line on which a comment inside the container can sit, so it would be lost or moved | `canInlineContainer` and `canFlattenContainer` refuse a container whose inside carries a comment; `TestSerializeCommentBlocksInline` |
| R-4 | The schema stamp moves off line 1 when its statement is reordered | The root's leading comment group before the first statement that starts at line 1 with the stamp prefix is always root head; `TestSerializeKeepsStampFirst` |
| R-5 | Display surfaces (editor `show`, web CLI `show`) begin printing comments the operator wrote | Intended: they print the operator's text. No file is written by them |
| R-6 | Corpus test silently passes on zero carriers | It fails when fewer carriers parse than a floor read from the tree at test start, and prints the counts |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An operator's file rewritten by `ze config fmt -w` loses or misplaces a comment, or a value changes meaning. Every hierarchical display shares the printer |
| How is it reverted? | Single commit revert per phase; no stored format changes, the set store is untouched |
| Who else touches this path? | `spec-ze-config-fmt` (parent, canonical form), `spec-web-cli-config-one-off-defects` (set writers, owns the comment row for them), the editor and web CLI display paths |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config fmt -w <file>` | → | `cmdFmt` → `configFmtBytes` → `Parser.Parse` → `Serialize` | `test/parse/config-fmt-comments.ci` |
| `ze config fmt --check <file>` on the written file | → | same path, idempotence | `test/parse/config-fmt-comments.ci` (second command exits 0) |
| `ze config fmt -w <file>` with a bracket or newline in a value | → | `quoteIfNeeded` | `test/parse/config-fmt-value-meaning.ci` |
| `ze config fmt <set-file>` | → | `cmdFmt` set-format refusal | `test/parse/config-fmt-set-refused.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A file carrying a comment on its own line above a leaf, a container, a list entry, and an `inactive:` statement, formatted with `-w` | Each comment's text appears once, on the line directly above its statement, at the statement's indent, after reordering |
| AC-2 | A comment after a statement on the same line, after `{`, and after `}` | Printed on the same line after the same statement, opening line or closing brace, separated by one space |
| AC-3 | Comments after the last statement of a block, and after the last statement of the file | Printed inside that block before `}`, and at the end of the file |
| AC-4 | A comment group at the head of a block or file separated from the next statement by a blank line | Printed at the head of that block, followed by one blank line |
| AC-5 | A file whose first line is `# ze-schema: <release>` followed directly by a statement that the canonical order moves | The stamp is still line 1 and `ScanStampRelease` reads the same release |
| AC-6 | A comment after `[` in a bracket leaf-list | Printed directly above that leaf statement; the members and their order unchanged |
| AC-7 | Two blocks of the same container, or a leaf set twice, each carrying comments | Every comment printed, in source order, on the merged statement; none dropped |
| AC-8 | A container whose inside carries a comment, which today prints inline or flattened | Printed as a block, the comment on its statement inside |
| AC-9 | Any value containing `[`, `]`, a newline, a carriage return, or starting with `(` or `)` | Printed quoted; parsing the output yields the same value; a second fmt exits 0 |
| AC-10 | Every `.conf` under `test/` and every `.ci` `tmpfs=` and `stdin=config` body that parses | Parse of the formatted text equals the parse of the source (values, inactive flags, ordered order); formatting twice changes no byte; the multiset of comment texts is equal |
| AC-11 | `ze config fmt -w` on the AC-1..AC-8 file, then `ze config fmt --check` on the result | `-w` exits 0 and prints `Formatted: <path>`; `--check` exits 0 |
| AC-12 | `ze config fmt` on a set or set-meta file | Exit 2; the message names the set form and `ze config migrate format hierarchical <file>` (`format` is a keyword, `parseOutputForm` in `cmd_migrate.go`; `--format` does not exist) |
| AC-13 | A file with no comments | Output byte-identical to today's, except AC-9 values |
| AC-14 | `prune` over a tree and a baseline differing only in comments | The list entry is pruned as unchanged |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `ze config fmt -w` on a commented production file | file → tokenizer trivia → parser attach → `Tree` → `Serialize` → file | `test/parse/config-fmt-comments.ci` |
| 2 | Runs `ze config fmt --check` in CI after formatting | same path, unchanged bytes, exit 0 | `test/parse/config-fmt-comments.ci` |
| 3 | Formats a file whose description holds `]` | parser → `quoteIfNeeded` → file → parser | `test/parse/config-fmt-value-meaning.ci` |
| 4 | Runs `ze config fmt` on the editor's stored set file | `DetectFormat` → refusal naming the fix | `test/parse/config-fmt-set-refused.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTokenizerCommentTrivia` | `internal/component/config/tokenizer_comments_test.go` (a new file, so `tokenizer_test.go` stays unedited) | Own-line, same-line, after `{`, after `}`, after `[`, end-of-file comments become trivia with their blank-line and same-line flags; semicolon insertion unchanged | phase 2 |
| `TestParserAttachesComments` | `internal/component/config/parser_comments_test.go` | AC-1, AC-2, AC-3, AC-4, AC-5 (stamp is root head) attachment on the `Tree` | phase 2 |
| `TestParserBracketComment` | `internal/component/config/parser_comments_test.go` | AC-6 | phase 2 |
| `TestParserMergedStatementComments` | `internal/component/config/parser_comments_test.go` | AC-7 | phase 2 |
| `TestTreeCloneKeepsComments` | `internal/component/config/tree_comments_test.go` | `Clone`, `MergeContainer`, `RenameListEntry` carry comments; a merge replacing a list entry keeps the entry's comments | phase 2 |
| `TestTreeLeafRemovalDropsComments` | `internal/component/config/tree_comments_test.go` | R-2 | phase 2 |
| `TestSerializeComments` | `internal/component/config/serialize_comments_test.go` | AC-1..AC-4 printed placement, golden strings written by hand | phase 3 |
| `TestSerializeKeepsStampFirst` | `internal/component/config/serialize_comments_test.go` | AC-5 | phase 3 |
| `TestSerializeCommentBlocksInline` | `internal/component/config/serialize_comments_test.go` | AC-8, both inline and `ze:flatten` | phase 3 |
| `TestParserRefusesUnterminatedQuote` | `internal/component/config/parser_quote_test.go` | AC-10 meaning: a quote with no closing quote is a syntax error naming its line (fuzz find `bgp peer A000"0000`) | phase 3 |
| `TestParserRefusesInlineValueOnContainer` | `internal/component/config/parser_quote_test.go` | AC-10 meaning: a one-line list value that falls on a container child is refused | phase 3 |
| `TestFormatOneLineListEntries` | `internal/component/config/serialize_list_key_test.go` | AC-10 meaning: a key named like a child, a leaf-list before the last child | phase 3 |
| `TestTreeDiffIgnoresComments` | `internal/component/cli/diff_tree_comments_test.go` | R-5: the editor diff reports no change for a comment-only difference, as AC-14 prune does | phase 3 |
| `TestSerializeStampedReplacesStamp` | `internal/component/config/serialize_comments_test.go` | AC-5 with a writer: the editor commit replaces the kept stamp, never stacks a second (`test/editor/lifecycle/history-dedup.et` went red without it) | phase 3 |
| `TestTreeDiffGutterHighlightsLineBelowComments` | `internal/component/cli/diff_tree_gutter_test.go` | R-5: with the diff gutter on, a validation error lands on its statement's line below comment lines | phase 3 |
| existing `serialize*_test.go`, `cli/main_test.go` fmt tests | `internal/component/config/` | AC-13: every existing expectation passes unedited; an edited one goes to an independent `ze-test-judge` agent | |
| `TestQuoteIfNeededRoundTrip` | `internal/component/config/list_key_test.go` | AC-9 through the hierarchical tokenizer | phase 1 |
| `TestQuoteRoundTripHierarchical` | `internal/component/config/list_key_test.go` | AC-9, AC-10 meaning and idempotence: leaf value and string list key through `Serialize` and `Parser` | phase 1 |
| `TestQuoteRoundTripSet` | `internal/component/config/list_key_test.go` (beside the shared value generator) | A-4: same values through `SerializeSet`, `SerializeSetWithMeta` and `SetParser` | phase 1 |
| `TestConfigFmtWriteKeepsValueMeaning` | `internal/component/config/cli/cmd_fmt_test.go` | AC-9 through `cmdFmt` -w then --check | phase 1 |
| `TestPruneIgnoresComments` | `internal/component/config/prune_test.go` | AC-14 | phase 3 |
| `TestFormatCorpusRoundTrip` | `internal/component/config/cli/fmt_corpus_test.go` | AC-10 over `test/**/*.conf` and `.ci` bodies via `tmpfs.Parse`; fails under a carrier floor | phase 3 |
| `FuzzFormatRoundTrip` | `internal/component/config/fuzz_test.go` | AC-10 properties for any input that parses, seeded from the corpus | phase 3 |
| `FuzzQuoteRoundTrip` | `internal/component/config/fuzz_test.go` | AC-9 for any value string | |
| `TestConfigFmtSetInputRefused` | `internal/component/config/cli/cmd_fmt_test.go` | AC-12 through `cmdFmt` | phase 1 |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | The feature takes no numeric input | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `config-fmt-comments` | `test/parse/config-fmt-comments.ci` | AC-1..AC-5, AC-7, AC-8, AC-11: `-w` then `--check`; `expect=file:path=` contains each comment and `expect=stdout:regex` proves each comment sits directly above or beside its statement | phase 3 |
| `config-fmt-value-meaning` | `test/parse/config-fmt-value-meaning.ci` | AC-9: `-w`, then `--check` exits 0, the file contains the quoted value | |
| `config-fmt-set-refused` | `test/parse/config-fmt-set-refused.ci` | AC-12: exit 2, stderr names the fix command | |

### Interop Tests (Scope: protocol)
- N-A: no wire protocol changes; config tooling only.

## Files to Modify

- `internal/component/config/tokenizer.go` - `skipWhitespaceAndComments` keeps each comment as trivia with its blank-line and same-line flags instead of discarding it
- `internal/component/config/parser.go` - attaches trivia to the statement it belongs to (rules in Key Design Decisions), including the `inactive:` and bracket paths
- `internal/component/config/tree.go` - comment trivia on `Tree`; `Clone`, `MergeContainer`, `RenameListEntry`, `RemoveListEntry` and leaf removal keep it consistent
- `internal/component/config/serialize.go` - `serializeNode` and the block walkers print leading, trailing, head and closing comments; `canInlineContainer` refuses a container whose inside carries one
- `internal/component/config/flatten.go` - `canFlattenContainer` refuses the same
- `internal/component/config/list_key.go` - `quoteIfNeeded` quotes `[`, `]`, newline, carriage return, and a leading `(` or `)`; a newline inside quotes is written `\n`
- `internal/component/config/prune.go` - `pruneUnchangedListEntries` compares entries without comments
- `internal/component/config/cli/cmd_fmt.go` - `cmdFmt` calls `configFmtBytes`; `DetectFormat` refusal of set and set-meta input naming `ze config migrate format hierarchical <file>`
- `internal/component/config/fuzz_test.go` - two fuzz targets
- `docs/architecture/config/syntax.md` - new Comments subsection; Schema Stamp says fmt keeps it
- `docs/architecture/config/tokenizer.md` - the `#` row
- `docs/config-migration.md` - the `ze config fmt` section
- `docs/guide/configuration.md` - the Comments row of the syntax table
- `plan/journal/error-names-no-fix-command.md` - close the 2026-10-27 `cmdFmt` row this spec owns

## Files to Create

- `internal/component/config/parser_comments_test.go` - attachment tests
- `internal/component/config/tree_comments_test.go` - tree operation tests
- `internal/component/config/serialize_comments_test.go` - placement tests
- `internal/component/config/cli/fmt_corpus_test.go` - corpus round trip
- `test/parse/config-fmt-comments.ci` - operator writeback with comments
- `test/parse/config-fmt-value-meaning.ci` - values that need quoting
- `test/parse/config-fmt-set-refused.ci` - the set-format refusal

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config node added |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | None |
| CLI commands/flags | No | `ze config fmt` keeps its flags; only its refusal text for set input changes |
| CLI grammar (keyword before value) | N-A | No grammar change |
| Editor autocomplete | N-A | None |
| Functional test for new RPC/API | N-A | No RPC; three `.ci` tests cover the command |
| Pipe completeness | N-A | Offline command, no pipe |
| Env var registration | N-A | None |
| Doctor check for runtime dependencies | N-A | No runtime dependency |
| Prometheus counters/metrics | N-A | None |
| BGP family surface | N-A | None |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Fix of an existing command |
| 2 | Config syntax changed? | Yes | `docs/architecture/config/syntax.md` (Comments subsection, stamp), `docs/guide/configuration.md` (Comments row) |
| 3 | CLI command added/changed? | Yes | `docs/config-migration.md` fmt section (behaviour, set refusal) |
| 4 | API/RPC added/changed? | N-A | None |
| 5 | Plugin added/changed? | N-A | None |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` |
| 7 | Wire format changed? | N-A | None |
| 8 | Plugin SDK/protocol changed? | N-A | None |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC |
| 10 | Test infrastructure changed? | No | Uses existing `.ci` matchers and `tmpfs.Parse` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/tokenizer.md` (trivia) |
| 13 | Route metadata keys added/changed? | N-A | None |
| 14 | Prometheus counters added/changed? | N-A | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derive with `./le spec citation anchors spec plan/release/fixes/spec-config-fmt-preserves-operator-information.md` at each phase end; `syntax.md` and `tokenizer.md` are known |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/config-migration.md` fmt examples re-run against the built binary |

## Implementation Steps

Each phase is sized at about 60 tool calls and ends with its scoped package run
and its own commit.

1. **Phase: Wiring and meaning (MANDATORY FIRST)** -- one fmt path, the set refusal, the quoting fix, the three `.ci` tests written
   - ACs: AC-9, AC-12, AC-13, and the meaning and idempotence halves of AC-10
   - Tests: `TestConfigFmtSetInputRefused`, `TestQuoteIfNeededRoundTrip`, `TestQuoteRoundTripSet`, `FuzzQuoteRoundTrip`, `TestFormatCorpusRoundTrip` (meaning and idempotence), `config-fmt-value-meaning.ci`, `config-fmt-set-refused.ci`, and `config-fmt-comments.ci` written and red
   - Files: `cmd_fmt.go`, `list_key.go`, `fmt_corpus_test.go`, the three `.ci`, `docs/config-migration.md`, the owned journal row
   - Verify: each test red before the fix, green after; the comments `.ci` red for the comment reason only
2. **Phase: Trivia and attachment** -- tokenizer keeps comments, parser attaches them, `Tree` carries them
   - ACs: AC-1..AC-7 at the `Tree` level
   - Tests: `TestTokenizerCommentTrivia`, `TestParserAttachesComments`, `TestParserBracketComment`, `TestParserMergedStatementComments`, `TestTreeCloneKeepsComments`, `TestTreeLeafRemovalDropsComments`, existing `FuzzTokenizer` and `FuzzConfigParser` green
   - Files: `tokenizer.go`, `parser.go`, `tree.go`, `docs/architecture/config/tokenizer.md`
   - Verify: semicolon insertion unchanged (`tokenizer_test.go` unedited)
3. **Phase: Printing and proof** -- the printer writes comments; prune ignores them; corpus and functional proof
   - ACs: AC-1..AC-8 printed, AC-10 comment multiset, AC-11, AC-14
   - Tests: `TestSerializeComments`, `TestSerializeKeepsStampFirst`, `TestSerializeCommentBlocksInline`, `TestPruneIgnoresComments`, `FuzzFormatRoundTrip`, `TestFormatCorpusRoundTrip` (all three properties), `config-fmt-comments.ci` green
   - Files: `serialize.go`, `flatten.go`, `prune.go`, `syntax.md`, `configuration.md`
   - Verify: revert the printer change, rebuild, confirm `config-fmt-comments.ci` goes red, restore, green; record it

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | No comment can be dropped: every trivia token reaches exactly one statement, head or closing slot, including on error-free merges |
| Correctness | A comment never changes meaning: `ToMap`, `DiffMaps` and every set printer ignore it |
| Data flow | Comments enter only in the tokenizer and leave only through the hierarchical printer |
| Rule: `ai/rules/config.md` | No text surgery: no comment is placed by line number or regex over output |
| Rule: `ai/rules/no-layering.md` | No second printer, no comment-only walker beside `serializeNode` |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Comments survive `ze config fmt -w` | `./le test functional parse` passes `config-fmt-comments` |
| Values keep meaning | `config-fmt-value-meaning` passes; `FuzzQuoteRoundTrip` |
| Corpus round trip | `TestFormatCorpusRoundTrip` prints carrier counts and passes |
| One fmt path | `cmdFmt` body calls `configFmtBytes` (`gopls references`) |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | A comment line of any length or byte content (binary, `\r`, very long) is carried verbatim and cannot break out of its line on output; `FuzzFormatRoundTrip` covers it |
| Resource exhaustion | Trivia storage grows linearly with input; no per-comment goroutine or unbounded copy |
| File writes | `-w` keeps `cliio.WriteFile` with mode 0600 |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood, back to design |
| Lint failure | Fix inline. If architectural, back to design |
| Functional test fails | Check the AC: wrong AC, back to design; correct AC, implement |
| A corpus carrier stops parsing | A-5 is wrong: fix the parser change, never exclude the carrier |
| 3 fix attempts failed | STOP. Report all 3 approaches |

## Design Insights

- The persistent store is set-meta text written by `SerializeSetWithMeta` with the stamp, so an operator's hierarchical comments already die at the first editor commit or startup evolution. fmt is the only writer this spec can make lossless now; the set writers need a comment spelling first.
- `cmdFmt` and `configFmtBytes` were two copies of one path, and only the copy no operator runs had unit tests.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `error-names-no-fix-command` | 2026-10-27 | `cmdFmt` on a set-format file |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Comments are trivia on the `Tree`, on the statement they belong to | A side table keyed by node path (the `MetaTree` shape); re-inserting comments into the output by line number | Line-number splicing is text surgery `ai/rules/config.md` bans, and breaks when the canonical order moves a statement. A path-keyed side table needs re-keying on every rename and merge and an encoding for multi-key list paths. On the `Tree` a comment moves with `Clone`, `MergeContainer` and `RenameListEntry` for free, and every printer reads one representation |
| Statement identity: a leaf (by name in its parent), an `inactive:` member line (name and member), a container or list entry (on its own `Tree`) | One comment slot per `Tree` | A slot per `Tree` cannot tell which leaf a comment described |
| Own-line comment: leading comment of the next statement in the same block | Attach to the previous statement | Operators write the comment above what it describes; gofmt and Junos read it the same way |
| Same-line comment after a statement, after `{`, after `}`: trailing on that statement, opening line or closing brace | Treat as leading of the next | It describes the line it sits on |
| Own-line comments after a block's last statement: closing comments, printed before `}` | Attach to the parent's next statement | They sit inside the block, so they stay inside it |
| A comment group before a block's first statement, separated from it by a blank line: head comments, printed first then one blank line. At the root, a first line carrying the schema stamp prefix is always head | Keep every blank line | Statement order is canonical (parent Q-2), so a blank line has no stable position; the only blank line with meaning is the one detaching a header. The stamp must stay on line 1 for `ScanStampRelease` |
| A comment after `[` in a bracket leaf-list: leading of that leaf | A member-level slot | A bracket list prints on one line; A-3 shows no other in-list position parses |
| Merged statements concatenate comments in source order | Keep only the last | fmt removes nothing, so no comment may vanish |
| A removed statement takes its comments with it | Re-home them | fmt removes nothing; only an editor delete removes a statement, and then its comment describes nothing |
| Inline and flattened spellings refuse a container whose inside carries a comment | Print the comment after the inline line | An inline line has room for one trailing comment at most, so association would be lost |
| Only the hierarchical printer prints comments; the set printers do not, owned row in `spec-web-cli-config-one-off-defects` | Invent a set-meta comment spelling here | `#` already opens set-meta's user field; choosing a spelling is a storage-format design for the spec that owns the set writers. The `Tree` representation chosen here is what that fix prints |
| `quoteIfNeeded` quotes every value the tokenizer would not read back as one equal token | Quote in fmt only | The defect is in the shared printer, so the set writers were changing meaning too (`ai/rules/simplicity.md`: fix at the root) |
| `cmdFmt` calls `configFmtBytes` | Keep both | Two copies of one path; the tested one was not the one operators run |
| An `ordered-by user` list prints in the order the tree recorded; every other list stays sorted by key (`ListNode.UserOrdered` from the YANG `ListAttr.OrderedByUser`, `listBlockKeys`) | Sort every list; keep operator order for every list | Phase 1 found `serializeListBlocks` sorting every list, which reorders a first-match list (firewall terms, prefix/aspath/community filter entries, policy-route rules, AAA and RPKI servers, RSVP-TE explicit routes). Order of the other lists is owner Q-2. IKE and ESP proposals are not user-ordered and their order carries no meaning: `ike/ipsec/config.go` sorts them by `number` after `GetListOrdered` |
| fmt and migrate hash a `plaintext-password` through one helper (`hashPlaintextPasswords`) before printing | Refuse the file | The daemon's load path and a commit apply the same hash; unhashed, the hierarchical form deleted the credential and the set form printed it in clear |
| OWNER DECISION (Thomas, 2026-10-28): fmt never prints a secret in clear (Junos model). A `ze:bcrypt` plaintext keeps its one-way hash; every other `ze:sensitive` leaf is written in `$9$` form through `secret.Encode` | Print the decoded plaintext (today) | Someone glancing at a screen must not read a secret, even though `$9$` is reversible. Blocker resolved by preserving the original encoding, the Junos model: `secret.Encode` draws a random salt, so every parser (hierarchical, inline list entry, set, set-meta, inline args) decodes through one helper, `decodeSecretLeaf` (`config/secret_text.go`), which keeps the `$9$` text it read (`Tree.SetStoredSecret`), and every printer (`Serialize`, `SerializeSet`, `SerializeSetWithMeta`, so fmt, both migrate forms and the editor) prints through `encodeSecrets`, which writes that text while it still decodes to the value held; only a value given in clear is encoded, once, and that encoding is recorded as the stored text, so the second print changes no byte. `secret.Encode` stays random, so equal keys never print equal strings. The set form was the gap the first pass left: the set parser read a `$9$` string as the literal key. Tests: `TestConfigFmtPrintsSecretEncoded`, `TestConfigFmtKeepsStoredSecretEncoding`, `TestConfigMigratePrintsSecretEncoded`, `TestConfigMigrateSetFormPrintsSecretEncoded`, `TestSetFormKeepsStoredSecretEncoding`, `TestEditorKeepsStoredSecretEncoding` |
| OWNER DECISION (same ruling): the `$9$` rule covers every surface that publishes a configured secret, not only the file printers. `SerializeSetWithMeta` writes each session entry of a `ze:sensitive` leaf, its `^previous` key included, in the `$9$` form (`encodeSecretMeta`) and the set-meta parser decodes both back; gNMI Get and Subscribe read the copy `EncodeSecrets` answers, and the Set echo to subscribers goes out through `SecretValueAtPath`; `Editor.SetValue` decodes a `$9$` value a client sends and keeps its text (`decodeSecretValue`, `keepStoredSecret`) | Leave gNMI and the meta entries in clear | Three leaks found in review of the first pass: the `^previous` key and a contested entry printed in clear in set-meta, gNMI Get/Subscribe answered keys in clear, and the Set echo carried one client's key to every subscriber. Tests: `TestSetMetaWritesContestedSecretEncoded`, `TestContestedSecretNeverWrittenInClear`, `TestSetMetaRefusesUndecodablePrevious`, `TestGetAnswersSecretsEncoded`, `TestSubscribeOnceAnswersSecretsEncoded`, `TestSetEchoesSecretsEncoded`, `TestSetValueDecodesStoredSecret`, `TestEncodeSecretsFollowsEveryNodeKind`, `TestEncodeSecretMetaFollowsEveryNodeKind` |
| `SerializeSubtree` prints a secret as the tree holds it, unencoded | Encode there too | `pruneUnchangedListEntries` compares two `SerializeSubtree` texts, and a fresh random salt on each side would make every entry look changed; every display caller masks first (`MaskSecrets`) |
| The editor's displays print comments (R-5). `show` prints them through `Serialize`; the change view compares comment-free trees (`config.WithoutComments` in `computeTreeAnnotatedDiff`), so a comment never marks a line changed, and maps each diff line to the n-th statement line of the working text (`renderDiffLines`, `workingStatementLines`), so a validation highlight lands below comment lines | Strip comments from every display | R-5 made concrete. Tests: `TestTreeDiffIgnoresComments`, `TestTreeDiffGutterHighlightsLineBelowComments` |
| A writer that persists the hierarchical form with a fresh stamp replaces the stamp the tree kept (`SerializeStamped`, `withoutSchemaStamp`) | Prepend the stamp as before | The parser keeps a first-line stamp as root head, so the editor commit stacked one more stamp per commit (`test/editor/lifecycle/history-dedup.et` went red). Test: `TestSerializeStampedReplacesStamp` |
| `Serialize` prints `ze:hidden` nodes, skipping only `ze:ephemeral` | Keep skipping hidden | fmt and migrate deleted the whole `redistribute` root and bgp `prefix maximum updated`, both hidden; `SerializeSet` never skipped them. Display views still skip hidden. Evidence: `TestFormatCorpusRoundTrip` map equality |
| The positional multi-block list form is used only when every entry's set children form a prefix of the child order and no key equals a child name (`entriesArePositional`) | Always positional | `nbma-neighbor X { priority 1 }` printed `X 1` and read back as another field; `plugin internal use 000` read back as an anonymous entry. Tests: `TestFormatOneLineListEntries`, corpus |
| The parser refuses an unterminated quote (`tokenizer.unterminated`, `refuseUnterminated`) and a one-line list value that falls on a container child (`refuseInlineSlot`) | Accept as before | Fuzz finds (`FuzzFormatRoundTrip` seeds a55eddd91b490dde, c8a6ea94bdee9abd, bd2ba0ef98c970e2): an unterminated quote silently swallowed the rest of the file, and the inline value was stored where no reader or printer looks, so fmt deleted it. Both are meaning losses at the parser. Tests: `TestParserRefusesUnterminatedQuote`, `TestParserRefusesInlineValueOnContainer` |

### Every writer of config text

| Writer | Printer | Keeps comments after this spec |
|--------|---------|--------------------------------|
| `ze config fmt` stdout, `-w`, `--diff`, `--check` | `Serialize` | Yes |
| `ze config migrate format hierarchical` | `Serialize` | Yes, for every node migration keeps (comments ride `Clone`) |
| `ze config migrate` default set output | `SerializeSet` | No: owned row |
| Editor commit within an editing session: the daemon's CLI editor, `ze config set`, `ze config deactivate`, `ze config rollback`, the web editor, gNMI Set | `FormatSchemaStamp` + `SerializeSetWithMeta` (`Editor.stampedWorkingContent`) | No: owned row |
| Editor commit with no editing session, over a hierarchical file | `SerializeStamped` (`Editor.stampedWorkingContent`): `Serialize` with the kept stamp replaced, never stacked | Yes |
| gNMI Get, Subscribe, and the Set echo to subscribers | values read from `EncodeSecrets(tree)`; the echo through `SecretValueAtPath` | N-A: no config text; every `ze:sensitive` key in the `$9$` form |
| Hub schema evolution at start (`cmd/ze/hub/main_evolve.go`), downgrade recovery (`stamp.go`) | `SerializeSetWithMeta` | No: owned row |
| Editor drafts and change files | `SerializeChangeFile`, `SerializeSetWithMeta` | No: editor-internal set text, same row |
| `ze config import` | raw bytes | Yes already: copies the file |
| `ze exabgp migrate` | `migration.SerializeTree` | N-A: ExaBGP input; the second printer is the parent's Q-4 |
| Editor `show`, web CLI `show`, annotated views | `Serialize`, annotated printers | Displays print comments where the tree has them; nothing written |

## Known Limitations

- The set and set-meta writers still drop comments. Owned by `plan/release/fixes/spec-web-cli-config-one-off-defects.md` through the 2026-10-27 row in `plan/journal/format-cannot-hold-a-state-the-tree-holds.md`.
- Blank lines and statement order are canonical presentation decided by `plan/post-release/engineering/spec-ze-config-fmt.md` Q-2.

## RFC Documentation (Scope: protocol)

N-A: no protocol code.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced (none referenced)
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Scoped package runs under `./le job run label <label> command go test -race <package>` per `docs/contributing/running-commands.md` for `internal/component/config/...`; `./le test functional parse` and `./le test functional ui` run by the main thread; `./le verify changed` over the closing commits
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes, `ai/rules/quality.md` satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N-A: none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests (N-A: no protocol)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + the closed journal row
- [ ] **Commit B:** `remove plan/release/fixes/spec-config-fmt-preserves-operator-information.md` only

## Disposition

| Field | Value |
|-------|-------|
| Date | 2026-10-27 |
| Checked at | `e39f85f43e` |
| Verdict | CURRENT |
| Evidence | `tokenizer.skipWhitespaceAndComments` still discards `#` comments; `cmdFmt` still writes `config.Serialize` output over the file |
| Evidence | Probe with `bin/ze` built 2026-10-27: every comment and the schema stamp absent from fmt output; a value holding `]` printed bare and refused on the second pass |
| Evidence | No commit or live spec preserves comments: the parent `spec-ze-config-fmt` delegates G-2, AC-4 and Q-5 here; no config-archive or change-file spec carries comments |

The defect occurs at the producer at HEAD, its targets all resolve, and nothing
landed that does the work, so the spec runs as designed above. The parent's
split stands: it owns canonical presentation, this spec owns preservation.

## Implementation Summary

### What Was Implemented
- Phase 1 (b4eb89e4c5): `configFmtBytes` is the one fmt path; set and set-meta input refused by name with `ze config migrate format hierarchical <file>`; `quoteIfNeeded` quotes every value `bareWordReadsBack` would not read back; both set parsers decode escapes through `splitQuotedTokens`; `Serialize` prints `ze:hidden` nodes; `entriesArePositional`; `ordered-by user` lists keep tree order (`listBlockKeys`); fmt and migrate hash `plaintext-password` (`hashPlaintextPasswords`).
- `$9$` secrets (cb8db32a25, 6a450e4d0d, 8b428a13ac, 3f236d38c6, 4ee25c4062): `config/secret_text.go` decodes on every parser and encodes on every printer, the meta entries and gNMI.
- Corpus test (1b44cefd82): `TestFormatCorpusRoundTrip`.
- Phase 2 (82e42f2c2d): tokenizer trivia, `commentScope` attachment, comments on `Tree`.
- Phase 3 (4d7174214d): the printer writes comments (`serialize_comments.go`), inline and flatten refuse commented containers, `PruneUnchanged` compares comment-free trees, the editor diff ignores comments and maps its gutter past comment lines, `SerializeStamped`, and three fuzz-found parser fixes.

### Bugs Found/Fixed
- `quoteIfNeeded` printed `]`, `[`, newline, CR and a leading paren bare: `TestQuoteRoundTripHierarchical`, `FuzzQuoteRoundTrip`.
- Set parsers decoded no escape and trimmed Unicode spaces: `TestQuoteRoundTripSet`.
- fmt and migrate deleted hidden roots and the `plaintext-password` credential: corpus map equality, `TestWritersKeepPlaintextPasswordAsHash`.
- `ordered-by user` lists were sorted: `TestConfigFmtKeepsUserOrderedEntries`.
- Secrets in clear in set-meta `^previous`, contested entries, gNMI Get, Subscribe and Set echo: the tests named in Key Design Decisions.
- Unterminated quote swallowed the file; inline value on a container child stored where nothing reads it; key named like a child printed positionally: `TestParserRefusesUnterminatedQuote`, `TestParserRefusesInlineValueOnContainer`, `TestFormatOneLineListEntries`.
- Editor commit stacked schema stamps: `TestSerializeStampedReplacesStamp`, `history-dedup.et`.

### Documentation Updates
- `docs/architecture/config/syntax.md`: Leaf quoting, Comments subsection, stamp, unterminated quote, one-line entries; closure added the editor commit and display paragraph with anchors to `Editor.stampedWorkingContent`, `Model.renderTreeAtPath`, `computeTreeAnnotatedDiff`.
- `docs/architecture/config/tokenizer.md`: `#` row and the Go tokenizer Comments section.
- `docs/config-migration.md`: migrate `format` keyword, fmt paragraphs; closure fixed the stale `cmd_fmt.go -- encodeSecrets` anchor.
- `docs/guide/configuration.md`: Comments row; closure corrected "an editor commit still drops them".
- `docs/guide/gnmi.md`, `docs/architecture/api/architecture.md`, `docs/architecture/config/yang-config-design.md`, `docs/architecture/ssh/fixit-bcrypt-hash-credential.md`: `$9$` surfaces.
- `./le doc check all`: no finding in these files; its reds are foreign (wiki command catalog drift; anchors in `core-design.md`, `ipsec-9-ikev2-eap-nat.md`, `ci-workflows.md`).

### Deviations from Plan
- Scope grew under owner rulings and found defects, each recorded in Key Design Decisions: `$9$` on every surface, user-ordered lists, password hashing, hidden nodes, positional lists, the parser refusals, the stamp writer, the display and diff paths.
- AC-12 names `ze config migrate format hierarchical`: `--format` does not exist.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-4: the set parser reads back any value `quoteIfNeeded` quotes | `tokenizeLine` decoded no escape | `TestQuoteRoundTripSet` red | Fixed at the reader |
| approach | An implementation agent edited `serialize.go`, `serialize_set.go`, `cli/cmd_fmt.go`, `cli/cmd_migrate.go` and an import in `parser.go` with python and sed, past the per-edit hooks | Repo files are edited with Edit and Write only | The agent's own handoff | Closure reviewed those hunks of cb8db32a25 line by line: call-site swaps to the encoder (later renamed `EncodeSecrets`), the import removal, two comments; no defect |
| approach | Phase docs said every editor commit drops comments | A commit with no editing session writes the hierarchical form and keeps them | Closure review | Docs corrected |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Preserve every comment and its association | Done | `config/parser_comments.go` `commentScope`, `config/serialize_comments.go` `writeBlock` | |
| Preserve configuration meaning | Done | `config/list_key.go` `quoteIfNeeded`, `config/serialize.go` `entriesArePositional`, `listBlockKeys` | |
| Second pass byte-identical | Done | `TestFormatCorpusRoundTrip`, `FuzzFormatRoundTrip` | |
| Reuse the existing formatter | Done | `config/cli/cmd_fmt.go` `cmdFmt` calls `configFmtBytes` | |
| Keep flags and exit codes | Done | `cmdFmt` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestSerializeComments`, `config-fmt-comments.ci` | |
| AC-2 | Done | `TestSerializeComments`, `config-fmt-comments.ci` | |
| AC-3 | Done | `TestSerializeComments`, `config-fmt-comments.ci` | |
| AC-4 | Done | `TestSerializeComments`, `config-fmt-comments.ci` | |
| AC-5 | Done | `TestSerializeKeepsStampFirst`, `TestSerializeStampedReplacesStamp` | |
| AC-6 | Done | `TestSerializeBracketComment`, `TestParserBracketComment` | |
| AC-7 | Done | `TestParserMergedStatementComments`, `config-fmt-comments.ci` | |
| AC-8 | Done | `TestSerializeCommentBlocksInline` | |
| AC-9 | Done | `TestQuoteRoundTripHierarchical`, `FuzzQuoteRoundTrip`, `config-fmt-value-meaning.ci` | |
| AC-10 | Done | `TestFormatCorpusRoundTrip`, `FuzzFormatRoundTrip` | |
| AC-11 | Done | `config-fmt-comments.ci` | |
| AC-12 | Done | `TestConfigFmtSetInputRefused`, `config-fmt-set-refused.ci` | |
| AC-13 | Done | existing `serialize*_test.go` and `cli/main_test.go` expectations unedited | |
| AC-14 | Done | `TestPruneIgnoresComments` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Every row of the TDD table | Done | files named in the table | Pass in the closure run below |
| `FuzzQuoteRoundTrip`, `FuzzFormatRoundTrip` | Done | `config/fuzz_test.go` | 40 s fuzz runs clean (phase handoffs); seeds pass in the closure run |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Every file in Files to Modify and Files to Create | Done | Plus the files the scope growth needed: `secret_text.go`, `tree_comments.go`, `serialize_comments.go`, `stamp.go`, `cli/diff_tree.go`, `cli/editor*.go`, `gnmi/{get,set,subscribe}.go` |

### Audit Summary
- **Total items:** 14 ACs, 5 task requirements
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** AC-12 wording; scope growth in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| fmt keeps every comment beside its statement | functional + corpus | `test/parse/config-fmt-comments.ci` passes (parse suite 380/380); red walk with `writeCommentLines`/`writeCommentAfter` emptied failed it at seq 3. `TestFormatCorpusRoundTrip`, closure run: "carriers 1926, hierarchical 1912, parsed and checked 1105, ... comments 3472", every comment multiset equal |
| fmt keeps meaning | functional + corpus + fuzz | `config-fmt-value-meaning.ci`; corpus map equality over 1105 configs and user order over 19 lists (red with the user-order branch disabled: 3 diffs); `FuzzFormatRoundTrip` and `FuzzQuoteRoundTrip` 40 s clean after three finds fixed |
| Second pass byte-identical | functional + corpus | `config-fmt-comments.ci` seq 2 `--check` exits 0; corpus idempotence check |
| Set input refused with its fix | functional | `config-fmt-set-refused.ci`, exit 2 and the migrate command |
| No secret printed in clear (owner ruling) | unit, every surface | `TestConfigFmtPrintsSecretEncoded`, `TestConfigMigrateSetFormPrintsSecretEncoded`, `TestContestedSecretNeverWrittenInClear`, `TestGetAnswersSecretsEncoded`, `TestSetEchoesSecretsEncoded`: each red before its fix |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| The set and set-meta writers print no comment | Needs a set-meta comment spelling, a storage-format design | `plan/release/fixes/spec-web-cli-config-one-off-defects.md` |
| Whether lists with no meaningful order keep operator order, and blank lines | Owner decision Q-2, open | `plan/post-release/engineering/spec-ze-config-fmt.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/config-fmt-preserves-operator-information-ed626c22-d6a4-4293-a5f7-287a2deb8cf2.md` |
| `./le spec review check` | clean |
| Rounds | 2 |
| Reviewer lenses used | wiring, docs drift, removed behaviour, secrets and security, data flow, style pass on every changed Go file, the hook-bypassed hunks of cb8db32a25 |

Round 1: 0 BLOCKER, 3 ISSUE (below), NOTEs: a key whose plaintext itself begins `$9$` prints unencoded and reads back decoded (Junos behaves the same; `secret encode` refuses such a key); `withoutSchemaStamp` clones before the memo of a fresh encoding when nothing printed the tree first, so that one commit draws a new salt; `EncodeSecrets` panics only when `crypto/rand` fails, which Go 1.27 never returns, so no peer reaches it; spec prose (`--format`, writer table, Key Design Decisions missing the scope growth) corrected in one edit. The style pass ran on every changed Go file: no other finding. The `$9$`, positional and parser-refusal changes were checked against the corpus and fuzz evidence rather than re-derived.
Round 2 (over the three doc fixes): 0 BLOCKER, 0 ISSUE; `./le doc check all` shows no finding in these files.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The guide said every editor commit drops comments; `Editor.stampedWorkingContent` writes the hierarchical form, comments kept, when there is no session | `docs/guide/configuration.md` Comments row | Row states both paths, anchor added |
| 2 | ISSUE | syntax.md did not say what the editor displays and change view do with comments, or which commit keeps them | `docs/architecture/config/syntax.md` Comments | Paragraph with anchors to the producers |
| 3 | ISSUE | Source anchor named `encodeSecrets` in `cmd_fmt.go`; it lives in `secret_text.go` | `docs/config-migration.md` | Anchors repointed |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `parser_comments_test.go`, `tree_comments_test.go`, `serialize_comments_test.go`, `cli/fmt_corpus_test.go` | Yes | `ls -la`: 6925, 4295, 8491, 9594 bytes |
| `test/parse/config-fmt-comments.ci`, `config-fmt-value-meaning.ci`, `config-fmt-set-refused.ci` | Yes | `ls -la`: 2396, 1903, 1481 bytes |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-14 | Tests named in the Audit pass | Closure run, `go test -race -tags <all> ./internal/component/config/ ./internal/component/config/cli/ ./internal/component/gnmi/ -run 'Comment|Stamp|Quote|Corpus|Prune|Secret|Fmt|Migrate|Refuses|OneLine|Inline|Fuzz'`: three `ok`, 1768 PASS, no FAIL |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze config fmt -w`, then `--check` | `test/parse/config-fmt-comments.ci` | Read: seq 1 `-w` exit 0 with `Formatted:`, seq 2 `--check` exit 0, seq 3 patterns for each comment placement |
| `ze config fmt` on a value needing quotes | `test/parse/config-fmt-value-meaning.ci` | Parse suite 380/380 (phase 3 handoff) |
| `ze config fmt` on set input | `test/parse/config-fmt-set-refused.ci` | Parse suite 380/380 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestTokenizerCommentTrivia` |
| A-2 | confirmed | grep at phase 2 start |
| A-3 | confirmed | `TestParserBracketComment` |
| A-4 | broken, fixed | `TestQuoteRoundTripSet`; Mistake Log row |
| A-5 | confirmed | corpus: 1105 parsed at closure, none lost |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Comments and editor paths (syntax.md, configuration.md) | `Editor.stampedWorkingContent`, `Model.renderTreeAtPath`, `computeTreeAnnotatedDiff` read | Yes |
| fmt secrets and quoting (config-migration.md) | `fmtTree`, `encodeSecretLeaf`, `quoteIfNeeded`, tokenizer `:[` case | Yes |
| gNMI `$9$` (gnmi.md, yang-config-design.md) | `gnmi/get.go`, `subscribe.go` `EncodeSecrets`; `set.go` `publishedUpdate` | Yes |
