# 1311 -- RFC compliance page, and the copies that rotted

## Context

The README claimed "2,700+ MUST-level requirements across 166 enrolled RFCs and drafts"
and linked no explanation of how any of that is proven. The wiki had a page on the
subject, `rfc-implementation`, that nothing in the README pointed at and that had drifted
badly: 166 enrolled documents, 2,720 requirements, 539 gap annotations, four ratchets,
"none outstanding". The tree said 168, 2,909, 534, seven ratchets, and 34 requirements
still owing work in the declared remainder. Nobody was careless. Every figure was right
when it was typed, and no check reads either surface.

The goal was a public page explaining how compliance is proven, every claim checked
against the code that produces it, linked from the README where quality is discussed.

## Decisions

- **Rewrote the existing wiki page rather than adding an "RFC compliance" page.** The
  sidebar and five other pages link the `rfc-implementation` slug, and a second page
  would drift against the first.
- **Stated the weaknesses on the page.** Four of them. 3,060 of the 3,081 requirement
  tags are unit tests. 32 enrolled RFCs have no row on the public status page. The
  letter-and-spirit audit covers 49 of 1,534 auditable requirements, all in RFC 7606. And
  no check verifies the page itself. A page arguing that Ze's compliance claim is honest,
  which hid its own weak points, would refute itself.
- **Kept dated figures rather than removing every number.** A page with no numbers loses
  the reader. The date, the reproduce-it commands, and an explicit "it does not check
  this page" are the honest form.
- **Dropped a proposed statistics generator** (`--stats`, a generated file, a freshness
  check) after `--check-fresh` showed `ai/RFC-REQUIREMENTS.md` was already generated and
  already refuses to go stale. A new file plus a new check would have duplicated
  machinery that exists. Review then caught that this answer overcorrected: the ledger
  publishes the headline counts and the per-RFC table, but no totals row and neither
  aggregate split, so those figures are still hand-typed. The residue is a much smaller
  job than the spec proposed, and it is recorded below rather than built.
- **Recorded the vocabulary rule in `ai/rules/`, not as a memory or a one-file fix**
  (`ai/rules/rule-placement.md`): a shared behavior rule belongs where every agent reads
  it, which here means `ai/rules/simplified-technical-english.md` plus the digest.

## Consequences

- `ai/rules/simplified-technical-english.md` now carries the general rule: use the plain
  word unless the technical one earns its place, writing for a capable reader who knows
  computing but not this repository. It reaches every session through
  `ai/rules/CONDENSED.md`. `gated` is named as the standing example rather than as the
  rule itself, which is where two drafts went wrong: the first banned that one word, the
  second banned three more beside it.
- The rule is a directive, not a checker entry. `scripts/dev/ste_check.py` carries six
  habits and no jargon list, so nothing catches a new `gated`, and the 867 existing uses
  in `docs/`, `ai/` and `plan/` stay green. Making it mechanical means adding the word to
  that checker, which is an implementation-phase change and was not done here.
- The README's RFC row is the one remaining hand-typed figure, deliberately rounded
  ("2,900+ across 168") so ordinary prose edits do not need a regeneration pass.
- **One item stays open.** The aggregate figures the page states, the annotation split
  (534 / 841 / 370) and the evidence split (3,060 / 19 / 0 / 2), appear in no generated
  artifact. `ai/RFC-REQUIREMENTS.md` has the headline counts, the per-RFC table and the
  audit line, but no totals row. Both this session and the reviewer recomputed those
  splits with ad-hoc scripts. A totals line plus the two splits inside `render_ledger` is
  roughly thirty lines and inherits `check_ledger_fresh`: no new file, no new target, no
  new check. It needs an implementation session.
- A future edit to the wiki page MUST recompute figures from
  `scripts/dev/rfc_requirements.py`, and never copy them from `ai/RFC-REQUIREMENTS.md`,
  which can itself be stale in a dirty working tree.

## Gotchas

- **A regex over `rfc/short/*.md` overcounts annotations.** Counting `{gap` by pattern
  gave 538. The scanner's own collector gives 534, because the regex also matches
  annotations on lines below MUST level. A statistic about a checker has to be produced
  by that checker. The same mistake inflated `{not-applicable}` (855 vs 841) and
  `{single-polarity}` (373 vs 370).
- **The generated ledger can be stale while every number in it is defensible.**
  `ai/RFC-REQUIREMENTS.md` records each test's `file:line`, so an unrelated session's
  uncommitted edits shift line numbers and red `make ze-rfc-check` without any compliance
  change. Read that failure as "regenerate", not as "compliance broke".
- **My own work drew thirteen findings across two independent review rounds.** Nine on
  the page, the worst being the test-change approval escape stated as owner-enforced when
  it is a comment plus a greppable trail. Four more on the repository diff, including a
  README claim that every annotation carries a "published" reason when only the 534 gaps
  do. Writing a page about verification does not exempt the page from verification.
- **A wording rule written as a word ban is the wrong shape.** The owner objected to
  `gated`, so I banned `gated`, then widened it to `carrier`, `polarity` and
  `disposition`, which are exact names of real things and would have cut the
  prose-to-code link the rules deliberately make. The owner's actual point was broader
  and simpler: write the way a teacher speaks to a capable student, plain words unless
  the technical one carries meaning the plain one cannot. One example illustrates that
  rule. A list of forbidden words replaces it with something narrower and wrong at both
  edges.
- **Ask what already exists before you offer the user a choice between things to build.**
  I put a publishing decision to the owner for a generator, got an answer, and wrote a
  spec against it, when one command (`--check-fresh`) would have shown the need was
  already met. The owner spotted it by asking whether the spec needed implementing.

## Files

- `plan/spec-rfc-compliance-docs.md` (this spec, removed at closure)
- `README.md` -- corrected figures, links the page from Testing and Documentation
- `ai/rules/simplified-technical-english.md`, `ai/rules/CONDENSED.md` -- the vocabulary
  directive
- `docs/contributing/writing-style.md` -- the build-vocabulary replacement table
- `../wiki/rfc-implementation.md`, `../wiki/_Sidebar.md` -- separate repository, separate
  commit
