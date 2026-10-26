# Spec: prose-cites-symbols-not-facts

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P2 |
| Release reason | Engineering: the defect family lives in docs, comments, RFC summaries, specs and the `le` gates that read them. It reaches a user only through a wrong page, so it does not block the first release, and its 125 journal rows are the largest single tooling family |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Root cause.** Prose in this repository restates facts that code or the tree
already declares: a path, a symbol name, a line number, a count, a list, a config
or command spelling, the behaviour of a function in another file, the text of an
RFC section, a verdict measured on one day. Each restatement is a second
declaration of the fact (`ai/rules/principles.md`, "declared once"). The gates
that read prose check that a reference RESOLVES (the path exists, the name occurs
as a word), never what the prose SAYS, and nothing re-reads a citer when the
commit moves what it cites. So every restatement drifts the moment its producer
changes, and the drift is found by the next reader who happens to trust it.

**Why it matters.** A wrong page teaches an operator a config the parser refuses
(`origin set igp` in `docs/architecture/api/update-syntax.md`). A wrong RFC
annotation removes an obligation from the conformance count (`rfc/short/rfc7752.md`
excludes eight rows because four constructors "have no caller", and
`ls_export/export_encode.go` calls them). A dangling citation costs every later
session the time to find it is empty. 125 open journal rows across twelve classes
are this one mechanism.

**The fix, in one sentence.** Prose never carries a copy of a fact a producer
holds: it either cites the producer by symbol and lets a generator render the
fact, or it carries a claim that a gate judges against the producer, and the
commit that moves a producer re-judges every claim bound to it.

**Prefer deletion.** Where a prose surface exists only to restate a producer, the
work deletes it or replaces it with a generated block, rather than adding a check
that keeps the copy in step. Named deletions are in Key Design Decisions.

## Required Reading

### Architecture Docs
- [ ] the doc-gates page `ai/CODE-TO-DOCS.md` names for `internal/le/doc/` (read first; the gap this spec names is what the gates do not judge)
  → Constraint: every new check runs inside an existing `./le doc ...`, `./le rfc check` or `./le commit create` action; no new top-level area
- [ ] `ai/rules/documentation.md`, `ai/rules/principles.md`, `ai/rules/stale-comments.md`
  → Decision: a page wrong after a change is repaired in that change; this spec makes the commit enforce what those rules ask of the author
- [ ] `docs/contributing/rfc-conformance-gates.md`, "The row quote"
  → Constraint: the verbatim matcher `./le rfc check` already applies to `rfc/short/` rows is the one reused for RFC quotes in Go comments and summary prose
- [ ] `docs/contributing/committing.md`
  → Constraint: commit-time checks run inside `./le commit create` over the index it commits, never over the working tree
- [ ] `docs/features/ai-first.md` (declared as the design page by code this spec modifies)
  → Constraint: the page's description of the doc gates changes with F2 to F4 in the same commits

### Existing specs this one extends (no duplication)
| Spec | Relation |
|------|----------|
| `plan/post-release/engineering/spec-stale-spec-frame-check.md` (skeleton) | Owns the spec-frame half (a spec's Status or "unimplemented" claim older than the code). This spec's F5 binds recorded verdicts to input digests and leaves spec Status to that spec; its four `stale-spec-claims-done` rows are listed here and move there if it runs first |
| `plan/post-release/engineering/spec-rfc-anchor-resolution-check.md` (skeleton) | Owns `(§N)` anchors on `rfc/short/` requirement rows. This spec's F4 covers RFC section quotes in Go comments and in summary prose outside the requirement rows; the two share the section-text index |
| `plan/post-release/engineering/spec-publish-the-regenerated-command-catalog.md` (skeleton) | Owns publishing the sibling `gh-pages` and `wiki` catalogs. This spec removes the in-repo gate's dependency on those sibling copies (F5) |
| `plan/post-release/engineering/spec-docs-read-before-authoring.md` (design) | Forces the author to read the page. This spec makes the commit check what the page says. Complementary, no shared AC |
| `plan/post-release/engineering/spec-code-can-land-with-no-design-doc.md` (skeleton) | Missing pages. Not covered here |
| `plan/post-release/engineering/spec-gates-share-one-semantic-population.md` (design) | Supplies the Go unit library (symbol identity and body fingerprint) that F2 and F4 consume. The doc-drift unit-size rows (`./le doc wiring` symbol-to-page binding) are owned there |

## Current Behavior (MANDATORY)

**Source files read (2026-10-23):**
- [ ] `internal/le/doc/index/codetodocs.go` - `claimIsDeclared` resolves a dotted anchor claim by its member name alone; `claimAppearsAsWord` accepts an anchor when the name occurs as a whole word anywhere in the anchored file, a comment included. The anchor's REFERENCE is checked, its CLAIM is not
- [ ] `internal/le/doc/index/codetodocs_report.go` - renders `anchor symbol(s) not declared where named` (the analysis placed the check here; the logic is in `codetodocs.go`)
- [ ] `internal/le/doc/yangcontract/command_surfaces.go` - `scanMarkdownLines` marks every line inside a fenced block inactive, so command spans inside fences are never resolved
- [ ] `internal/le/doc/ste/extract.go` - the prose gate skips fenced blocks as data, correctly; no other gate reads them either
- [ ] `internal/component/bgp/plugins/cmd/update/update_text.go` - `kwSet` is "Rejected with migration hint"; `docs/architecture/api/update-syntax.md` still teaches `origin set igp` and `nhop set 10.0.0.1` in its grammar line and both attribute tables
- [ ] `rfc/short/rfc7752.md` - the RFC7752-3.2-2, 3.2-3 and 3.2.1-1 rows exclude obligations because `NewBGPLSNode` and siblings "have no caller outside _test.go", citing `types_nlri.go` by line number
- [ ] `internal/component/bgp/plugins/ls_export/export_encode.go` - calls `bgpls.NewBGPLSNode` and `bgpls.NewBGPLSLink` in production
- [ ] `rfc/README.md` - five hand-written status tables, a copy of `docs/features/rfc-status.md`
- [ ] `website/data/repo-facts.json` - committed counts refreshed by hand commits (`f1d5b2a901`, `059321905b`, `0696cb63b2` in five days)
- [ ] `internal/le/commit/prepare.go` - no file in `internal/le/commit/` names a citation or link check: `./le commit create` does not re-judge citers of a path or symbol the commit removes
- [ ] `docs/contributing/rfc-implementation-guide.md` - still opens a bullet "Five native RFC actions"
- [ ] `test/plugin/rfc7606-withdraw.ci` - one of 25 `.ci` files that still cite the deleted `ai/rules/qemu-testing.md` or another removed page
- [ ] `docs/guide/configuration.md` - 10 config lines still annotate with `//`, which the tokenizer does not accept

**Behavior to preserve:**
- `./le doc check links`, `./le doc check all`, `./le spec citation`, `./le doc wiring` keep their current verdicts for every claim they already judge correctly
- The verbatim row-quote rule in `./le rfc check` and its refusals
- `<!-- source: -->` anchor syntax and `// Design:` headers stay valid citation forms

**Behavior to change:** see the five shapes below.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le commit create` over the index of the commit under construction (paths and Go symbols removed or renamed, producer inputs changed)
- `./le doc check all` over the tracked tree (the full claim corpus)

### Transformation Path
1. Collect claims: anchors, `// Design:` headers, fenced blocks with an info string, inline command spans, RFC quotes in comments, count and list sentences in sections bound to a registry, recorded verdicts with input digests
2. Resolve each claim against its producer: symbol table (Go units from the T3 library), the real config parser with the full feature tag set, the `ze` and `le` command registries, `rfc/full/` section text, generator output
3. At commit time, select the claims bound to what the commit changes and re-judge only those
4. Report each failing claim with file, the claim, what the producer says, and the command that repairs it

### Boundaries Crossed
| Boundary | How |
|----------|-----|
| `le` to the `ze` binary | config parsing and the command registry are read from a `ze` built with the full feature set (the same build `./le doc check all` already uses for published surfaces) |
| Commit index to working tree | none: the commit-time half reads the index only |

### Integration Points
- `internal/le/doc/index` (anchors), `internal/le/doc/check` (links), `internal/le/spec/citation`, `internal/le/rfc` (row-quote matcher), `internal/le/commit` (commit-time selection)

### Architectural Verification
| Check | Answer |
|-------|--------|
| Registration over hardcoding | Claim kinds register with the claim collector; no central switch of kinds |
| No layering | The anchor-resolution path in `codetodocs.go` is replaced, not wrapped |
| Declared once | Every fact this spec touches ends with one producer and zero hand copies |

## Five shapes and the fix for each

| Shape | Rows | Mechanism today | Fix at the source | Check that stops it coming back |
|-------|------|-----------------|-------------------|---------------------------------|
| F1 Copied lists and counts | counts in prose and pinned in tests, hand tables (`rfc/README.md`, carrier table, `repo-facts.json`, site goldens pinned to gh-pages) | a human retypes what a registry or generator holds | delete the copy; where the reader needs it, render a generated block from the producer; a test asserts the derivation, never a literal count | `./le doc check all` refuses a numeral-plus-noun sentence inside a page section bound to a registry, and a test literal compared to a registry length |
| F2 Citations | dangling paths, renamed symbols, `// Design:` headers, discrimination records naming deleted units, line numbers, journal row line citations, rebased SHAs | citers are checked by a tree-wide sweep nobody runs at the moment of deletion | citation grammar is file plus symbol, never a line number; a journal row is cited by class and date; a SHA must be reachable from HEAD | `./le commit create` resolves every citer of each path or symbol the commit removes or renames, over the index, and refuses until each is repaired in the same commit |
| F3 Parseable examples | config blocks and command spans in `docs/`, skills, `ai/`, generated `CLAUDE.md`, interop and deployment scenario configs | fenced blocks are skipped; scenario configs are parsed only when the scenario runs | every fence in a tracked page carries an info string; `ze-config` fences and scenario configs parse with the real config parser; `ze-cli` and `le` command spans resolve against the registries | `./le doc check all` refuses an untagged fence, a `ze-config` block the parser refuses (with the parser's own error), and a command span no registry holds |
| F4 Behaviour claims | pages and cross-file comments describing a function, `rfc/short/` annotations judging code, RFC section quotes in comments | the reference is checked, the content never | each such claim carries an anchor; the anchor stores the cited unit's body fingerprint (from the T3 unit library); an RFC quote in a comment must be a verbatim span of the cited section | the commit that changes a cited unit's fingerprint must touch the citing paragraph or re-seal the anchor with a reviewed reason (a `plan/doc-reviewed/` row, which exists today); `./le rfc check` applies the row-quote matcher to RFC quote comments added from today |
| F5 Derived artifacts and recorded verdicts | `rfc/requirements/*`, `ai/RFC-REQUIREMENTS.md`, discrimination and audit records, web captures, templ output, published catalogs, spec evidence | a generated file is committed and regenerated by whoever next notices it red | a derived file is either untracked and generated on demand, or regenerated by `./le commit create` from the index whenever its producer input is in the commit; a recorded verdict stores the digest of its inputs | `./le commit create` refuses a commit whose producer inputs changed while a tracked derived file did not; a verdict whose input digest moved reads STALE, never current |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | Validation |
|----|------------|-------|------------|
| A-1 | Every fenced block in `docs/` can be given an info string without loss | fences today are bare or `text`; the info string is invisible in rendered Markdown | the migration counts untagged fences before and after |
| A-2 | The full-feature `ze` build parses every config the docs show as valid | the daemon is that build | F3 check run over the tree; each refusal is either a doc defect (owned row) or a parser defect (journal row to the S1 family) |
| A-3 | Body fingerprints from the T3 library are stable under comment and whitespace edits | go/ast printing drops comments | unit test over a comment-only edit |

### Risks
| Risk | Mitigation |
|------|-----------|
| The first run lights up hundreds of claims across the tree | land each check with its owned rows repaired in the same phase; no baseline that hides a finding |
| A large function (`runYANGConfig`) binds many pages, so every edit re-seals them | the unit-size problem is owned by the T3 spec; until it lands, a re-seal names the paragraph, not the page |
| A commit-time citer search is slow | the citer index is built once per `./le doc check all` run and read by commit create |

## Blast Radius

`internal/le/doc/**`, `internal/le/spec/citation`, `internal/le/rfc` (quote matcher reuse), `internal/le/commit` (commit-time selection), every tracked page with a fence, `rfc/short/*.md` annotations with line numbers, `rfc/README.md`, `website/data/repo-facts.json`, `internal/le/site/testdata/` goldens.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le commit create` removing a cited spec | → | commit-time citer resolution in `internal/le/commit` | `test/ui/le-commit-refuses-dangling-citer.ci` |
| `./le doc check all` over a page with a refused `ze-config` fence | → | F3 fence parser | `test/ui/le-doc-config-fence-refused.ci` |
| `./le commit create` changing a cited function body | → | F4 fingerprint re-judge | `test/ui/le-commit-refuses-unsealed-behaviour-claim.ci` |
| `./le commit create` changing an RFC producer without its shard | → | F5 derived regeneration | `test/ui/le-commit-regenerates-rfc-shards.ci` |

## Acceptance Criteria

| AC | Given | Then |
|----|-------|------|
| AC-1 | a commit that removes a file cited by any tracked file (`// Design:`, anchor, spec link, record) | `./le commit create` refuses, naming each citer and the removed path, and passes once the citers are repaired in the same commit |
| AC-2 | a commit that renames or deletes a Go symbol an anchor names | refused the same way, naming the anchor and the symbol |
| AC-3 | an anchor `X.Y` where the anchored file declares `Y` on another receiver, or only mentions `X.Y` in a comment | `./le doc check all` refuses it; `claimAppearsAsWord` no longer exists |
| AC-4 | a tracked page with a fence that has no info string | refused with the file, the fence, and the info strings accepted |
| AC-5 | a `ze-config` fence or a scenario config the parser refuses | refused with the parser's own error; `docs/architecture/api/update-syntax.md` passes after its repair |
| AC-6 | a `ze` or `./le` command span in a page, skill, rule or generated instruction that no registry holds | refused, naming the nearest registered command |
| AC-7 | a commit that changes the body of a unit an anchor cites, without touching the citing paragraph or a `plan/doc-reviewed/` row for it | refused, naming the paragraph |
| AC-8 | a new RFC quote comment whose quote is not in the cited section of `rfc/full/` | `./le rfc check` refuses it with the section it searched |
| AC-9 | a commit changing an RFC requirement producer input without the regenerated `rfc/requirements/` shard and `ai/RFC-REQUIREMENTS.md` | `./le commit create` writes the regenerated files into the commit, or refuses naming the generator |
| AC-10 | a recorded verdict (discrimination record, spec evidence row, review artifact) whose input digest no longer matches | every reader prints STALE with the digest pair, never the verdict as current |
| AC-11 | `rfc/README.md` | holds no status table; it points to `docs/features/rfc-status.md` |
| AC-12 | `website/data/repo-facts.json` | is generated by `./le site build` and untracked, or regenerated by the commit that moves a counted input; no hand refresh commit is possible |
| AC-13 | `rfc/short/*.md` annotations | carry no line numbers; `./le rfc check` refuses a path-plus-line citation inside an annotation |
| AC-14 | every row in "Owned journal rows" | is repaired or re-judged at its current producer (rows naming retired `scripts/`, `mk/` or `.claude/hooks/*.py` producers are re-judged against the Go successor) and closed in its journal file in the commit that repairs it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Proves |
|------|------|--------|
| `TestDottedAnchorNeedsReceiver` | `internal/le/doc/index/codetodocs_test.go` | AC-3 |
| `TestCommentMentionDoesNotSatisfyAnchor` | same | AC-3 |
| `TestUntaggedFenceRefused` | `internal/le/doc/check/fence_test.go` | AC-4 |
| `TestConfigFenceParsedWithRealParser` | same | AC-5 |
| `TestCommandSpanResolvesAgainstRegistry` | `internal/le/doc/yangcontract/command_surfaces_test.go` | AC-6 |
| `TestRemovedPathCitersSelectedFromIndex` | `internal/le/commit/citers_test.go` | AC-1, AC-2 |
| `TestFingerprintChangeNeedsSeal` | `internal/le/doc/index/seal_test.go` | AC-7 |
| `TestCommentRFCQuoteMustBeVerbatim` | `internal/le/rfc/commentquote_test.go` | AC-8 |
| `TestAnnotationLineNumberRefused` | `internal/le/rfc/check_test.go` | AC-13 |
| `TestStaleVerdictReadsStale` | `internal/le/rfc/discriminate_test.go` | AC-10 |

### Functional Tests
| Test | Location | Proves |
|------|----------|--------|
| the four Wiring Test `.ci` files | `test/ui/` | AC-1, AC-5, AC-7, AC-9 through the real `./le` entry point |

## Files to Modify
- `internal/le/doc/index/codetodocs.go` - replace word-match resolution with symbol resolution and fingerprint seals
- `internal/le/doc/yangcontract/command_surfaces.go` - resolve command spans inside tagged fences
- `internal/le/doc/check/links.go` - expose the citer index to commit create
- `internal/le/commit/prepare.go` - commit-time citer and derived-artifact selection
- `internal/le/rfc/check_core.go` - annotation line-number refusal, comment quote matcher
- `rfc/README.md`, `website/data/repo-facts.json`, `docs/contributing/rfc-implementation-guide.md`, `internal/le/site/testdata/` - deletions named in Key Design Decisions

## Files to Create
- `internal/le/doc/check/fence.go` - fence info-string and config parsing check
- `internal/le/commit/citers.go` - citers of removed paths and symbols, read from the index

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | tooling only |
| CLI commands/flags | No | checks join existing `./le doc check all`, `./le rfc check`, `./le commit create` |
| Functional test | Yes | `test/ui/*.ci` named in Wiring Test |
| Env var, doctor, metrics, BGP family | N-A | tooling only |

### Documentation Update Checklist (BLOCKING)
| Page | Change |
|------|--------|
| `docs/contributing/committing.md` | commit create refuses dangling citers and regenerates derived files |
| the doc-gates page `ai/CODE-TO-DOCS.md` names for `internal/le/doc/` | claim kinds, fence info strings, seals |
| `ai/rules/writing.md` | cite by symbol, no line numbers, no counts a registry holds |

## Implementation Steps
1. Land the T3 Go unit library (symbol identity and body fingerprint), or the minimal part F2 and F4 need
2. F2: citer index plus commit-time refusal; repair every owned `closure-deletes-a-cited-document` and dangling-anchor row in the same phase
3. F1: delete the `rfc/README.md` tables, generate the carrier table and the action list, untrack `repo-facts.json`, replace pinned counts and gh-pages goldens with derivation tests; close the owned `hardcoded-count-in-test` and `published-value-drifts` rows
4. F3: fence info strings over `docs/`, then the parser and registry check; repair every owned `documentation-shows-config-the-parser-refuses` and `failure-report-names-a-command` row
5. F4: anchor seals and the comment RFC quote matcher; repair owned `comment-describes-superseded-behaviour`, `reference-checked-claim-unchecked` and RFC-annotation rows
6. F5: commit-time regeneration of RFC derived files and digest-bound verdicts; close owned `concurrent-rfc-gate-stale`, `stale-artifact-reused` and `stale-spec-claims-done` rows (coordinate with `spec-stale-spec-frame-check`)
7. Run `./le test unit` over the touched `internal/le` packages and `./le test ui` for the four `.ci` files

## Key Design Decisions
| Decision | Why |
|----------|-----|
| Delete the status tables in `rfc/README.md` | `docs/features/rfc-status.md` is the ledger the gates read; the README copy has no reader that needs it |
| Untrack `website/data/repo-facts.json` and generate it at site build | it is a counted copy of the tree; hand refresh commits are its only maintenance |
| Replace site goldens pinned to the published gh-pages head with renderer tests on fixtures the package owns | a test of this repository must not depend on what a sibling checkout last published |
| No line numbers in any citation | a line number is the most volatile fact in the tree and no reader needs it once a symbol is named |
| Fence info strings rather than content sniffing | sniffing whether a block "looks like config" is the text proxy the T3 spec removes |
| The commit, not a periodic sweep, re-judges citers | the sweep exists today and drifted to 36 reds because nothing runs it at the moment of deletion |

## Owned journal rows

125 rows. A row belongs here when the defect is prose (page, comment, summary annotation, spec, test header, skill, generated instruction) stating a fact a producer holds, or a citation or derived copy that no longer matches its producer.

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| claim-outlives-the-evidence-it-cites | 2026-10-16 | `rfc/short/rfc7752.md` rows near 587-601, eight `{not-applicable}` exclusions |
| claim-outlives-the-evidence-it-cites | 2026-10-16 | `rfc/short/rfc7752.md` not-applicable note on `RFC7752-3.3.2.2-1` |
| claim-outlives-the-evidence-it-cites | 2026-10-16 | header comment of `internal/plugins/isis/rfc9514_bgpls_export_sentence_test.go` |
| claim-outlives-the-evidence-it-cites | 2026-10-04 | `./le spec citation` |
| claim-outlives-the-evidence-it-cites | 2026-09-07 | `./le docs-to-code index-check` |
| claim-outlives-the-evidence-it-cites | 2026-09-04 | the redistribution section of `docs/guide/configuration.md`, and the same sentence in `docs/comparison.md` |
| claim-outlives-the-evidence-it-cites | 2026-09-03 | `plan/spec-radius-subscriber-attributes.md`, closed at `64464097c0` |
| claim-outlives-the-evidence-it-cites | 2026-09-03 | nine RFC 2918 "Section 3" comments in `route_refresh/handler/refresh.go`, `reactor/reactor_api_forward.go`, ... |
| claim-outlives-the-evidence-it-cites | 2026-09-03 | the unit row of the carrier table in `docs/contributing/rfc-implementation-guide.md` |
| claim-outlives-the-evidence-it-cites | 2026-09-01 | `rfc/README.md`, against `docs/features/rfc-status.md` and the `rfc/full/` tree |
| claim-outlives-the-evidence-it-cites | 2026-08-31 | `rfc/short/rfc9582.md`, its row in `rfc/enrolled.txt`, the `RFC 9582` row of `docs/features/rfc-status.md` |
| claim-outlives-the-evidence-it-cites | 2026-08-31 | `docs/contributing/rfc-implementation-guide.md`, the "Five native RFC actions" bullet against `internal/le/rfc/actions.go` |
| claim-outlives-the-evidence-it-cites | 2026-08-22 | spec Review Gate section against `tmp/review/` |
| claim-outlives-the-evidence-it-cites | 2026-08-24 | commit SHAs cited in prose, after a rebase |
| claim-outlives-the-evidence-it-cites | 2026-08-31 | `rfc/short/rfc7313.md`, the line numbers inside six checklist annotations |
| claim-outlives-the-evidence-it-cites | 2026-08-31 | `rfc/short/rfc4301.md`, `docs/features/rfc-status.md` RFC 4301 row |
| claim-outlives-the-evidence-it-cites | 2026-09-02 | `docs/guide/plugins.md`, the AS-Path Length Filter section, against `countASPathHops` |
| claim-outlives-the-evidence-it-cites | 2026-09-02 | `docs/features/configuration.md`, a source anchor naming `internal/component/config/loader.go` |
| claim-outlives-the-evidence-it-cites | 2026-09-04 | The Measurement Record's `ipsec-bgp-redistribute-frr` Routing cell, and the two journal rows it cites |
| claim-outlives-the-evidence-it-cites | 2026-09-09 | `plan/TEMPLATE-CLOSURE.md` and `.claude/skills/ze-close/SKILL.md`, against `internal/le/commit/` |
| claim-outlives-the-evidence-it-cites | 2026-09-11 | The spec's Goal Validation interop rows, citing four scenario runs (`bgp-bfd-strict-frr`, ...) |
| claim-outlives-the-evidence-it-cites | 2026-09-13 | `rfc/discrimination/rfc6793.json`, the RFC6793-4.2.3-2 positive record |
| claim-outlives-the-evidence-it-cites | 2026-09-17 | `internal/le/site/changes_test.go`: `TestTheChangesIndexReadsAsThePublishedIndex`, ... |
| claim-outlives-the-evidence-it-cites | 2026-10-14 | `Defect review` rows of `features/*.md`, which cite journal rows by class file and line number |
| claim-outlives-the-evidence-it-cites | 2026-10-10 | doc source anchors: `compilePatterns`, `positionalDef`, `checkInlineNextHop`, `confirmImportReplace`, ... |
| reference-checked-claim-unchecked | 2026-10-05 | `passesTTLGate` comments (`internal/component/bfd/engine/loop.go`) |
| reference-checked-claim-unchecked | 2026-09-19 | `./le spec citation`, `danglingInSpec` (`internal/le/spec/citation/speccitation.go`) |
| reference-checked-claim-unchecked | 2026-08-14 | doc gates (`path_resolves` measures the checkout, not the tracked tree) |
| reference-checked-claim-unchecked | 2026-08-10 | doc gates (`claim_is_declared` resolves a dotted claim by its member alone) |
| reference-checked-claim-unchecked | 2026-08-21 | rfc summary annotations |
| reference-checked-claim-unchecked | 2026-08-23 | bgp VPN next-hop comments |
| reference-checked-claim-unchecked | 2026-08-31 | `rfc/short/rfc1877.md` |
| reference-checked-claim-unchecked | 2026-09-04 | the spec's own Wiring Test table, and three `// Design:` headers under `internal/component/bgp/plugins/bmp/` |
| reference-checked-claim-unchecked | 2026-09-08 | `internal/component/bgp/plugins/cmd/peer/create.go`, `internal/component/bgp/plugins/adj_rib_in/relay_chunk.go`, ... |
| closure-deletes-a-cited-document | 2026-10-07 | `TestEveryLinkedPathExistsInTheTree` (`internal/le/site/rfcdetail_test.go`) |
| closure-deletes-a-cited-document | 2026-09-13 | RFC ledger prose links on the published site, `rfcProseLinksHTML` (`internal/le/site/rfcprose.go`) |
| closure-deletes-a-cited-document | 2026-09-06 | source anchors in `docs/`, against `checkAnchors` |
| closure-deletes-a-cited-document | 2026-09-03 | YANG module comments |
| closure-deletes-a-cited-document | 2026-08-23 | `./le doc check links`, and `./le doc check verify` through it |
| closure-deletes-a-cited-document | 2026-08-11 | functional test comments |
| closure-deletes-a-cited-document | 2026-08-10 | workflow |
| closure-deletes-a-cited-document | 2026-08-12 | Go source header |
| closure-deletes-a-cited-document | 2026-08-11 | workflow |
| closure-deletes-a-cited-document | 2026-08-13 | spec cross-reference |
| closure-deletes-a-cited-document | 2026-08-14 | Go source header |
| closure-deletes-a-cited-document | 2026-10-23 | native post-closure citation checks |
| closure-deletes-a-cited-document | 2026-10-23 | `TestEveryLinkedPathExistsInTheTree` (`internal/le/site/rfcdetail_test.go`) |
| documentation-shows-config-the-parser-refuses | 2026-09-05 | `docs/architecture/api/update-syntax.md`, the whole "Text Mode" half |
| documentation-shows-config-the-parser-refuses | 2026-08-24 | plugin IPC protocol documentation |
| documentation-shows-config-the-parser-refuses | 2026-08-15 | docs/guide/ospf.md |
| documentation-shows-config-the-parser-refuses | 2026-08-15 | docs/architecture/api/process-protocol.md |
| documentation-shows-config-the-parser-refuses | 2026-08-15 | docs/guide/flowspec-protected-router.md |
| documentation-shows-config-the-parser-refuses | 2026-08-15 | 15 changed files under docs/ |
| documentation-shows-config-the-parser-refuses | 2026-08-23 | `test/interop/scenarios/ospfv3-sr-frr/ze.conf` |
| documentation-shows-config-the-parser-refuses | 2026-08-30 | `docs/contributing/running-commands.md`, `ai/rules/testing.md` |
| documentation-shows-config-the-parser-refuses | 2026-09-03 | config examples across `docs/` annotated with `//` |
| documentation-shows-config-the-parser-refuses | 2026-09-05 | `docs/architecture/system-architecture.md`, "Section 2: Plugin Declarations" and "Third-Party Plugins" |
| documentation-shows-config-the-parser-refuses | 2026-09-09 | `docs/contributing/running-commands.md`, "The changed-set selector" |
| documentation-shows-config-the-parser-refuses | 2026-09-11 | `.claude/README.md` |
| documentation-shows-config-the-parser-refuses | 2026-09-11 | `ze-doc-update` skill |
| documentation-shows-config-the-parser-refuses | 2026-09-11 | `docs/architecture/textbuf-string-building.md`, "Reusing a buffer with Reset()" |
| documentation-shows-config-the-parser-refuses | 2026-09-14 | `docs/config-reference.md`, the "IPsec VPN" block |
| documentation-shows-config-the-parser-refuses | 2026-10-21 | the `vpp-iface-test` uRPF and LCP scenario configs (`internal/le/test/deployment/vppifacescenarios.go`) |
| comment-describes-superseded-behaviour | 2026-09-17 | Unrelated source-anchor checks |
| comment-describes-superseded-behaviour | 2026-08-17 | test-weakening audit tooling, spec text |
| comment-describes-superseded-behaviour | 2026-08-17 | bgp/reactor forward path, two `.ci` headers |
| comment-describes-superseded-behaviour | 2026-08-17 | review gate rules and skills |
| comment-describes-superseded-behaviour | 2026-08-19 | `rfc/short/rfc2181.md`, `docs/features/rfc-status.md` |
| comment-describes-superseded-behaviour | 2026-08-31 | `docs/guide/bmp.md`, "Known limitations" |
| comment-describes-superseded-behaviour | 2026-09-01 | `internal/le/rfc/sections.go`, generated `ai/RFC-REQUIREMENTS.md` |
| comment-describes-superseded-behaviour | 2026-09-06 | `docs/architecture/behavior/peer-lifecycle.md`, the `cleanup()` section, step 3 |
| comment-describes-superseded-behaviour | 2026-09-08 | `applyRouteInstall` doc comment, `internal/component/plugin/server/dispatch_route.go` |
| comment-describes-superseded-behaviour | 2026-09-11 | `test/plugin/rfc7606-withdraw.ci`, unit-proof comment |
| comment-describes-superseded-behaviour | 2026-10-03 | `internal/mrt/bgp.go` and `internal/mrt/mrt_test.go`, the RFC 8050 subtype comment |
| comment-describes-superseded-behaviour | 2026-10-09 | Header comments of `test/appliance/no-install-appliance.ci`, `appliance-help-not-deprecated.ci`, ... |
| comment-describes-superseded-behaviour | 2026-10-14 | `internal/component/bgp/reactor/forward_local_pref.go`, `applyFactsLocalPref` header |
| comment-describes-superseded-behaviour | 2026-10-17 | `internal/plugins/isis/packet/tlv_neighbours.go`, `narrowMetricValueMask` comment |
| comment-describes-superseded-behaviour | 2026-10-20 | `ai/rules/architecture.md` "Design Principles", `docs/architecture/traffic/fw-7-traffic-vpp.md` |
| comment-describes-superseded-behaviour | 2026-10-20 | `docs/architecture/core-design.md` section 19 |
| comment-describes-superseded-behaviour | 2026-10-20 | `docs/architecture/core-design.md` sections 19, 20 |
| comment-describes-superseded-behaviour | 2026-10-20 | `ai/digests/fib-programming.md`, `ai/digests/hub-engine.md`, `ai/digests/aaa-auth.md` |
| comment-describes-superseded-behaviour | 2026-10-20 | `docs/architecture/web-interface.md` "LG Data Access" |
| comment-describes-superseded-behaviour | 2026-10-20 | `docs/architecture/appliance/command-provider.md`, `feature-gates.txt` header |
| comment-describes-superseded-behaviour | 2026-10-20 | `plan/learned/RECURRING-PATTERNS.md`, `plan/learned/DESIGN-HISTORY.md` |
| comment-describes-superseded-behaviour | 2026-10-21 | `sa.peerEndpoint` comments in ike `responder.go`, `ts_nat_substitute.go`, `sa.go` |
| failure-report-names-a-command-that-does-not-exist | 2026-09-23 | Native functional help |
| failure-report-names-a-command-that-does-not-exist | 2026-08-19 | internal/test/runner/report.go, internal/component/bgp/cli/main.go |
| failure-report-names-a-command-that-does-not-exist | 2026-08-31 | `CLAUDE.md` "Before You..." row for finishing Go edits, generated from `ai/INSTRUCTIONS.md` |
| failure-report-names-a-command-that-does-not-exist | 2026-09-11 | `docs/architecture/testing/test-health.md`, `internal/le/tier/testdata/core_import_baseline.txt`, ... |
| hardcoded-count-in-test | 2026-10-19 | `TestAnnouncementPlanOrderAndTimingFixture` (`internal/le/interoplab/bgp/speaker_test.go`) |
| hardcoded-count-in-test | 2026-09-24 | the published-site goldens under `internal/le/site/testdata/` against `website/blog/posts/` |
| hardcoded-count-in-test | 2026-09-14 | `TestTheChangesIndexReadsAsThePublishedIndex`, `TestTheChangesIndexFileIsNewestFirst`, ... |
| hardcoded-count-in-test | 2026-09-12 | `TestSupportedRowsHaveDerivableScope` (`internal/le/rfc/check_test.go`) and `TestGapCount...` |
| hardcoded-count-in-test | 2026-09-05 | `ui/le-spec-status-answers` (`internal/test/fixture/ui_fixture_le_spec_status_answers.go`) |
| hardcoded-count-in-test | 2026-09-01 | `TestSupportedRowsHaveDerivableScope` (`internal/le/rfc/check_test.go`) |
| hardcoded-count-in-test | 2026-09-02 | `TestRootLaunchersArePOSIXShellWithoutPython/le` in `cmd/ze/root_launcher_test.go` |
| hardcoded-count-in-test | 2026-09-03 | `TestAnnouncementPlanOrderAndTimingFixture` (`internal/le/interoplab/bgp/speaker_test.go`) |
| hardcoded-count-in-test | 2026-09-08 | the hand-maintained site goldens under `internal/le/site/testdata/` |
| gate-red-where-nothing-blocks-on-it | 2026-10-10 | `TestEveryStoredDigestMatchesTheTreeItRatchets`, `hookSourcesDigest` (`internal/le/ai/hooks`) |
| gate-red-where-nothing-blocks-on-it | 2026-09-15 | `./le doc check links`, red at HEAD (`9e3298e3d6`) on two references |
| gate-red-where-nothing-blocks-on-it | 2026-08-31 | `./le doc check links` |
| gate-red-where-nothing-blocks-on-it | 2026-09-17 | `internal/plugins/ospf`, `TestOSPFNetworkTypeV4V6Isolation` (`config_nbma_test.go`) |
| published-value-drifts-from-the-behavior-it-describes | 2026-09-25 | `./le doc check verify`, published per-command surface |
| published-value-drifts-from-the-behavior-it-describes | 2026-09-28 | `./le site facts check`, `website/data/repo-facts.json` |
| published-value-drifts-from-the-behavior-it-describes | 2026-09-28 | `./le doc wiring`, doc-drift |
| concurrent-rfc-gate-stale | 2026-08-15 | `rfc/requirements/rfc9568.md` |
| concurrent-rfc-gate-stale | 2026-08-16 | rfc ledger and audit records |
| concurrent-rfc-gate-stale | 2026-08-17 | RFC 4271 requirements |
| concurrent-rfc-gate-stale | 2026-08-19 | RFC requirements ledger, the regeneration itself |
| concurrent-rfc-gate-stale | 2026-09-03 | `rfc/audit/rfc7606.json`, `ai/RFC-REQUIREMENTS.md`, `rfc/requirements/rfc7606.md` |
| concurrent-rfc-gate-stale | 2026-09-17 | `test/ui/le-rfc-answers.ci`, `./le rfc check` |
| concurrent-rfc-gate-stale | 2026-09-28 | `./le rfc check`; `rfc/discrimination/` rfc4271, rfc7611, rfc7705, rfc7947, rfc8907 |
| concurrent-rfc-gate-stale | 2026-10-03 | `rfc/discrimination/rfc6396.json` |
| concurrent-rfc-gate-stale | 2026-10-23 | `./le rfc check`; `rfc/discrimination/rfc7432/`, `rfc9830/` |
| stale-spec-claims-done | 2026-08-14 | bgp replay ownership, plugin startup |
| stale-spec-claims-done | 2026-08-14 | ipsec interop lab |
| stale-spec-claims-done | 2026-08-22 | spec metadata Status field against the tree |
| stale-spec-claims-done | 2026-10-21 | `plan/spec-ospf-ext-0-umbrella.md` against `internal/plugins/ospf/auth_wiring.go` |
| stale-artifact-reused | 2026-10-09 | `../wiki/command-catalog.md` (`./le cli catalog update`) |
| stale-artifact-reused | 2026-10-04 | Published wiki and website command catalogs |
| stale-artifact-reused | 2026-09-16 | `../gh-pages/data/cli-commands.json`, `reference/cli/`, `reference/command-equivalents/` |
| stale-artifact-reused | 2026-09-06 | `internal/component/web/testdata/handler/*.txt`, four captures |
| stale-artifact-reused | 2026-08-31 | `./le doc check templ-output` over `internal/component/web/page_snapshot_templ.go` |
| stale-artifact-reused | 2026-08-30 | `rfc/short/rfc4301.md` checklist annotations |
| hardcoded-count-in-test | 2026-08-15 | assert-nothing ratchet floor |
| hardcoded-count-in-test | 2026-08-23 | `docs/guide/ipsec.md` "Sixteen scenarios" |
| plugin-list-hardcoded | 2026-08-31 | `internal/le/docvalid/register.go` `Description` lists three gates |
| plugin-list-hardcoded | 2026-09-08 | `vrrp-keepalived-test` `Why` and placeholder list three of four scenarios |
| reference-checked-claim-unchecked | 2026-08-09 | doc gates: 82 source anchors name an absent symbol |
| reference-checked-claim-unchecked | 2026-08-16 | hook mapping prose still describes the removed `updatedInput` check |
| reference-checked-claim-unchecked | 2026-08-23 | a peer's correction adopted before anybody opened the file |
| stale-artifact-reused | 2026-09-30 | RFC2473-4.1.1 and RFC4213-3.6 `{not-applicable}` markers |
| stale-artifact-reused | 2026-10-11 | `normalizeRequest` comment in `kernelbuilder/recipe.go` |
| stale-spec-claims-done | 2026-08-29 | `plan/` spec `in-progress` after its deliverables landed |
| tree-state-claim-published-unverified | 2026-08-31 | a YANG description collision reported across sessions |
| unwired-feature | 2026-08-28 | `functional` area Description advertises Docker-exec analysis |
| zero-value-as-valid-answer | 2026-09-03 | a grep offered to a peer as an all-clear on a rename |
| checklist-names-a-page-that-changed-role | 2026-09-07 | `website/AI.md` Phase 4 checklist and its skill copy name the dispatch `comparison.md` as the BGP mirror |
| claim-outlives-the-evidence-it-cites | 2026-08-23 | a gate verdict quoted against HEAD in a shared checkout, stale when sent |
| claim-outlives-the-evidence-it-cites | 2026-08-25 | a peer session's credit claim accepted and published unverified |
| claim-outlives-the-evidence-it-cites | 2026-08-29 | `rfc/short/rfc2205.md` and `rfc4090.md` restate their RFC wrongly |
| claim-outlives-the-evidence-it-cites | 2026-08-29 | a capability claim resting on a plugin registration, not a call path |
| claim-outlives-the-evidence-it-cites | 2026-09-01 | `RFC5443-4-1` tag in `ospf/ldp_sync_test.go` rests on "ze originates no TE LSA" |
| claim-outlives-the-evidence-it-cites | 2026-09-03 | commit body `1243b5974`, a negative grep passed on as an all-clear |
| claim-outlives-the-evidence-it-cites | 2026-09-04 | uncommitted prose in `docs/architecture/core-design.md` drifting under three sessions |
| claim-outlives-the-evidence-it-cites | 2026-09-09 | doctor inventory row `listener/ipsec` names a listener no extractor produces |
| claim-outlives-the-evidence-it-cites | 2026-09-12 | commit message of `d5a7a8a84` misdescribes `PruneLanded` and the gate |
| claim-outlives-the-evidence-it-cites | 2026-09-30 | `RFC4301-4.1-8` retired as "no multicast SAs" while OSPF installs them |
| claim-outlives-the-evidence-it-cites | 2026-10-08 | `RFC9012-13-7` retired on absent consumption |
| claim-outlives-the-evidence-it-cites | 2026-10-20 | `draft-ietf-sidrops-8210bis` rows `7-1` and `7-4` quote one sentence and disagree |
| comment-describes-superseded-behaviour | 2026-09-01 | `checkSupportedSignoff` comment "Ten such rows sit on the page today" |
| document-prose-breaks-ste-ratchet | 2026-09-05 | `bgp/defaults/attribute` summary is a paragraph; anchors name `getHelpExtension` |
| documentation-shows-config-the-parser-refuses | 2026-08-15 | `docs/guide/logging.md` prints `bgp log set` and `bgp log levels`, not commands |
| green-that-could-not-have-been-red | 2026-08-23 | a grep whose pattern was never proven able to find, reported as absence |
| green-that-could-not-have-been-red | 2026-08-24 | `.git/index.lock` diagnosed orphaned on true signals answering another question |

## Rows found already fixed

Verified at the producer on 2026-10-23 unless the Evidence cell says otherwise. Another agent closes these in the journal.

| Class file | Date | Surface (short) | Evidence |
|------------|------|-----------------|----------|
| claim-outlives-the-evidence-it-cites | 2026-08-29 | `rfc/short/rfc2205.md`, `rfc/short/rfc4090.md` restated their RFC wrongly | `RFC2205-3.10-1` now quotes "Class-Num = 0bbbbbbb" verbatim at §3.10; `./le rfc check` refuses any non-verbatim row corpus-wide, which covers the RFC 4090 half |
| claim-outlives-the-evidence-it-cites | 2026-08-25 | `(§11)` on `RFC8210-11-1` | the row now cites §13 with the RFC's own sentence (`rfc/short/rfc8210.md`) |
| reference-checked-claim-unchecked | 2026-08-09 | 82 anchors naming a symbol absent from their file, all green | `claimIsDeclared` (`internal/le/doc/index/codetodocs.go`) and the `not declared where named` arm of `codetodocs_report.go` now refuse them |
| reference-checked-claim-unchecked | 2026-08-16 | hook mapping named `updatedInput` | the row's own Fix cell records the non-check row removed; not re-read at the producer |
| hardcoded-count-in-test | 2026-08-23 | `docs/guide/ipsec.md` "Sixteen scenarios run today" | no scenario count remains on the page |
| comment-describes-superseded-behaviour | 2026-09-01 | comment above `checkSupportedSignoff` ("Ten such rows") | the sentence is gone from `internal/le/rfc/check_status.go` |
| documentation-shows-config-the-parser-refuses | 2026-08-15 | `docs/guide/logging.md` `bgp log set` and `bgp log levels` | neither string remains on the page |
| stale-spec-claims-done | 2026-08-24 | RFC gate ratchets spec | the row's Fix cell records the spec closed as already satisfied; not re-read |
| stale-spec-claims-done | 2026-08-29 | EBGPWire removal spec | the row's Fix cell records the spec closed as already satisfied; not re-read |

## Rows read and not owned

Every open row of these twelve classes that is in neither table above was read and routed elsewhere:

| Routed to | Rows |
|-----------|------|
| T5 (RFC tag proves something else) | `reference-checked-claim-unchecked` 2026-10-13 x4, 2026-10-06 x3, 2026-08-14; `claim-outlives` 2026-09-01, 2026-09-30 x2, 2026-10-08, 2026-10-20 |
| product family (prose states the intended behaviour, the code is wrong) | `comment-describes` 2026-10-15 VPP SIGKILL, 2026-09-03 SSH idle-timeout, 2026-10-21 MUP forms; `claim-outlives` 2026-10-13 `ze signal restart`, 2026-10-19 DHCPv6 test, 2026-09-09 doctor listener inventory; `reference-checked` 2026-08-10 l2tp `AddRoute`; `published-value` 2026-09-21, 2026-09-25; `failure-report` 2026-10-10, 2026-10-13 |
| T4 (shared tree, stale binary, cache keys) | every other `stale-artifact-reused` row; `closure-deletes` 2026-08-15 web package build; `concurrent-rfc-gate-stale` 2026-08-15 x2 |
| T6 (failure prints the wrong cause) | `failure-report` 2026-09-18 |
| a count gate doing its job | `hardcoded-count-in-test` 2026-08-12, 2026-08-15 x2, 2026-08-25 (ratchet floors) |
| T1, T3, T6 or product | `gate-red-where-nothing-blocks-on-it` rows other than the four pins owned here |
| one-off practice rows with no artifact a gate can read | `claim-outlives` 2026-08-23, 2026-08-25, 2026-08-29, 2026-09-03 commit body, 2026-09-04 uncommitted prose, 2026-09-12 commit message; `concurrent-rfc-gate-stale` 2026-08-25 unproven MUSTs; `reference-checked` 2026-08-23 peer correction |
| `spec-gates-share-one-semantic-population.md` | `claim-outlives` 2026-09-02 `interopCarriers`, `gate-red-where-nothing-blocks-on-it` 2026-10-09 untagged test |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] No code snippets
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] Wiring Test table complete
- [ ] Targeted tests pass: `./le test unit` over the touched `internal/le` packages, `./le test ui` for the four `.ci` files, then `./le verify current fast`
- [ ] Every owned row closed in its journal file by the commit that repairs it

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests: N-A, tooling with no protocol peer

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + journal row closures
- [ ] **Commit B:** `remove` the spec only
