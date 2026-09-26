# Spec: rfc-requirement-quote-hand-backfill

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Closed spec `spec-rfc-requirement-verbatim-quote` made every requirement
row in `rfc/short/<stem>.md` carry the RFC sentence it states, and
`./le rfc check` refuses a new or changed row whose quote is not in its section.
Its backfill (`./le rfc quote-backfill stem <stem> apply`) quoted 1821 rows
mechanically on 2026-09-26. This spec quotes every row the backfill could not.

The work ends when three things hold:

1. `./le rfc check` prints `unquoted: 0`, and no stem is named unjudged.
2. The row-quote rule applies to every row in the corpus, not only to a row a
   commit adds or changes. With the count at zero, the change scope has nothing
   left to protect.
3. Each row the backfill rewrote has had its tagged tests re-read against the
   new sentence (risk R-7 of the source spec). A test that proves less than the
   sentence now states is a finding: the test or the row is corrected, never
   the sentence.

### Rows left to quote

Figures from the backfill dry run over 201 stems (6354 rows), 2026-09-26. After
the apply, `./le rfc check` counted 3696 unquoted rows over 188 stems. The
difference from 3706 is the 10 `no-rfc-text` rows: rfc8326 (3) and rfc9129 (7)
have no text in `rfc/full/`, so the check names them unjudged and counts none
of their rows.

| Bucket | Kind | Rows | What a human does |
|--------|------|------|-------------------|
| review | partial | 256 | the sentence carries more obligations than the row; quote the whole sentence or split the row |
| review | number-absent | 217 | a number in the row is not in the sentence; find the sentence that carries it |
| review | qualified | 167 | the sentence qualifies the obligation; quote it with the qualifier |
| review | several-sites | 140 | the row maps to several sentences; pick one, or split the row |
| review | low-overlap | 108 | the paraphrase shares under half its content words with the sentence; check the mapping first |
| review | polarity-differs | 102 | NOT or MUST NOT differs between row and sentence; the row or the mapping is wrong |
| review | unresolved-anchor | 89 | the row's section does not resolve in the RFC text; correct the section |
| review | lead-in | 76 | the site is a lead-in to a list; quote the list item that states the obligation |
| review | outside-section | 69 | the sentence is not in the row's section; correct the section or the mapping |
| review | level-differs | 40 | the sentence's keyword differs from the row's level; correct the level or the mapping |
| review | short-sentence | 7 | the sentence is under the minimum length; quote the sentence with its context |
| human | unsourced | 1511 | the extraction declares the row has no source site; find the sentence, or retire the row |
| human | unmapped | 556 | no extraction site maps to the row; find the sentence |
| human | no-extraction | 358 | the stem has no extraction artifact; find each sentence by hand |
| human | no-rfc-text | 10 | fetch `rfc/full/rfc8326.txt` and `rfc/full/rfc9129.txt`, then quote |
| | **Total** | **3706** | review 1271, human 2435 |

### The review list

The per-stem list is not committed. It is derived: `./le rfc quote-backfill stem
<stem>` without `apply` writes nothing and lists every row it would not quote,
with its kind and reason. Run it for each stem in `ls rfc/short` to regenerate
the whole list over the tree in hand. A committed copy would be stale after the
first row quoted.

### Tagged tests to re-read (R-7)

Overlap cannot see a paraphrase whose words match and whose meaning differs.
The source spec measured about 1 in 15 applied rows as such. Three are known and
are read first:

| Row | What changed |
|-----|--------------|
| RFC7432-6.3-3 | the paraphrase named the normalized VID, the sentence names the originating VID |
| RFC9552-5.2-6 | the quoted sentence states a different obligation from the paraphrase |
| RFC5880-6.7.3-4 | the quoted sentence states a different obligation from the paraphrase |

Found by the rfc7606 re-audit (2026-09-26): four verdicts moved from enforced to
weak once the row states the full RFC sentence. Each note in
`rfc/audit/rfc7606.json` names the tag that moves it back; adding a tag owes a
discrimination record (`ai/rules/rfc-compliance.md`).

| Row | Clause no tagged unit drives |
|-----|------------------------------|
| RFC7606-3.g-2 | "whether recognized or unrecognized": both tagged units duplicate ORIGIN only |
| RFC7606-4-1 | the second §4 case, fewer than three octets remaining |
| RFC7606-7.10-2 | the zero-length half of "non-zero multiple of 4" |
| RFC7606-7.14-1 | the length-5 case; `TestRFC7606ExtendedCommunityLength` tests it but carries no tag |

The set to read is every row whose text changed in the backfill commit and whose
id a test tags (`RFC requirement:` tag). `git diff <backfill-commit>^ <backfill-commit> -- rfc/short`
names the rows.

## Required Reading

- `spec-rfc-requirement-verbatim-quote`, closed: its closure commit holds the
  spec (the rule, the backfill kinds, R-7, A-6); the rule and the kinds live on
  the page below
- `docs/contributing/rfc-conformance-gates.md`, "The row quote"
- `ai/rules/rfc-compliance.md`

## Current Behavior (MANDATORY)

Source files read so far. The design phase completes this list.

- [ ] `internal/le/rfc/quote_backfill.go` - `quoteBackfill` judges each row and names the kind that kept it unquoted
- [ ] `internal/le/rfc/check_quote.go` - `checkRowQuotes` judges only rows the tip commit adds or changes; `unquotedFigures` prints the tree figure

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

[Where data enters] Filled at design time.

### Transformation Path

Filled at design time.

### Boundaries Crossed

Filled at design time.

### Integration Points

Filled at design time.

## Wiring Test (MANDATORY -- NOT deferrable)

End condition 2 (the rule over every row) owes a test driven through
`./le rfc check`. The design phase names it.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` | → | `checkRowQuotes` over every row | Filled at design time |

## Acceptance Criteria

Filled at design time. The three end conditions under Task are the goals.

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| Filled at design time | - | - |

## Files to Modify

Filled at design time. The corpus rows are in `rfc/short/<stem>.md`.

## Implementation Steps

Filled at design time.

## Known Limitations

Filled at design time.

## Checklist

### TDD

- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure

- [ ] `./le verify worktree` passes
