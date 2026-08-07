# 1359 - A number typed into a rule file is an unverified claim

**Date:** 2026-08-07
**Scope:** rules, tooling, verification

## What Changed

Nothing in the product. This records a measurement. It was taken from a check of
`CRITICAL-REVIEW.md`, a self-audit of the repository, against its own citations.

## The Measurement

The review carried about 40 factual claims and declared itself
"producer-verified". Re-checking every one against the file that implements it
split the errors along one clean line.

**The reviewed document was deleted at the owner's instruction and was never
committed, so the two totals below cannot be re-derived by anyone.** They are
recorded as the reason this summary exists, not as evidence. The six errors in
the next section are each independently checkable against a producer that is
still here, and they are what this summary rests on.

| Where the claim came from | Claims checked | Wrong |
|---|---|---|
| A `.go` file the review opened | ~20 | 0 |
| A rule file under `ai/rules/` the review summarized | ~15 | 6 |

Zero errors on the Go side. The `WireUpdate` freeze comment, `ContextID uint16`,
`shardIDBits = 4`, and the `AllocCeilings` entries all matched, several word for
word.

Six errors on the rules side:

- `stagesForMode` has 24 full and 21 changed stages. The review said 22 and 20,
  in five places, while its own transcription of the list held 24 entries.
- It proposed building `make ze-token-economy`. That target exists
  (`mk/inventory.mk`), and the context figures the review quotes are its output.
- It attributed a 240s duration to `Makefile`, which contains no such number.
- It proposed deleting `.claude/skills/` and `.codex/skills/`, which are
  generated from `ai/skills/` and gitignored.
- It proposed per-agent `GOCACHE` to remove the verify lock. `verify-lock.sh`
  takes an unconditional `flock` on `tmp/.ze-verify.lock`, which a cache split
  does not touch at all. (The ports and `bin/ze` rationale for that lock lives
  in `ai/rules/git-safety.md`, not in the script. An earlier draft of THIS
  summary attributed it to the script: the same defect, one hop further in.)
- It read a scoped commit as blocked without `ze-verify-changed`, missing
  `--unverified`, which `git-safety.md` prescribes for exactly that case.

## The Cause

Every wrong claim was a number or a mechanism the review restated from a rule
file instead of deriving from the thing that produces it. `stagesForMode` is the
producer of "24 stages". `git-safety.md` typed that number by hand, and typed
"240s" beside it while another paragraph said 25 to 30 minutes. The review then
re-typed both, one of them wrong.

`ai/rules/evidence.md` already says to read the producer. It is read as a rule
about Go. It applies to a rule file with equal force, because a rule file is
also a document that describes something else.

The corpus makes this likely rather than merely possible. Its size is not stated
here, for the reason this whole summary gives: run `wc -l ai/rules/*.md` and read
the number off the corpus rather than off a document describing it. An agent that
would open a short Go file will summarize a thousand-line rule, and a summary is
where a number changes.

## The Rule Already Exists Elsewhere

`rfc/extraction/` solved this problem. Its README states the split: "Only
**dispositions, reasons and the two relocation fields** are authored. `sites`,
`sections`, `quote`, `register`, `source-path`, `source-sha` and every published
count are DERIVED from the source text at check time [...]", because "A hand-typed
"sites seen" number would be a claim, and claims are what this programme exists
to remove" (`rfc/extraction/README.md`, "Derived versus authored").
`check_gap_count_agreement` (`scripts/dev/rfc_requirements.py`) refuses a spelled
number in `docs/features/rfc-status.md` that disagrees with the real count.

Both quotations above are verbatim from the README, and that is deliberate. An
earlier draft of this summary quoted `ai/rules/rfc-compliance.md`'s CONDENSED
paraphrase of that sentence inside quotation marks, having never opened the
README. A summary about paraphrase drift, quoting a paraphrase as source text.

The rules corpus has no equivalent. It authors its counts.

## What To Do About It

1. Treat a number in a rule file as a claim needing a producer, not as
   documentation. Prefer naming the producer ("the stage list in
   `stagesForMode`") over restating its cardinality.
2. Where a number must be stated, derive it. The stage count, the rule count and
   the skill count all have machine-readable producers and are candidates for a
   generated line, exactly as `TRIGGERS.md` already generates "Rules: 27".
3. When a rule states a duration, state what produces it. The 240s/2min/30min
   contradiction survived because three paragraphs each held a bare number with
   no source. `verify-lock.sh` was the producer the whole time.

## Files

Hand-edited sources only. The generated artifacts that follow from them
(`ai/rules/CORE.md`, `ai/rules/git-safety.md`, `quality.md`, `commands.md`,
`ai/LEARNED-FULL-INDEX.md`, `ai/DOCS-TO-CODE.md`) are regenerated and committed
alongside, as `ai/rules/git-safety.md` requires.

- `ai/rules/points/git-safety/before-any-commit/run-verify-in-the-foreground-and-wait.md` -- replaced the bare "240s timeout" with the two duration regimes and their producer
- `ai/rules/points/git-safety/before-any-commit/run-make-ze-verify-and-check-freshness-first.md` -- dropped its duplicate 240s, points at the section that owns the number
- `ai/rules/points/git-safety/before-any-commit/the-pre-commit-verify-checklist.md` -- same, in the checklist line
- `ai/rules/points/quality/proof/run-make-ze-verify-before-claiming-done.md` -- fourth copy of the same number, found by grep after the first three
- `internal/component/bgp/message/rfc7606_shape.go` -- `Design:` marker repointed from the closed spec to `plan/learned/1225-rfc7606-relay-shape.md`
- `ai/rules/points/commands/rationale/abandoned-poll-loops-made-the-suites-flaky.md` -- carried a live "22-stage" count, the same defect elsewhere in the corpus. It now says "a full `ze-verify`"
- `ai/skills/ze-verify.md` -- the operative instruction at the point of use: dropped its 240s timeout, its 240s Fallback trigger, and a citation to a rule section that does not exist
- `ai/skills/ze-check.md` -- same 240s timeout
- `ai/rationale/git-safety.md` -- "finishes well under the 240s Bash timeout"
- `.claude/hooks/pretool-bash.py` -- a comment asserting the rule prescribes `timeout 240s make ze-verify`, false once the rule stopped naming a duration

## What The Review Then Found In The Fix

Three independent reviewers ran over the change above, then two more over the
fixes. They confirmed the six errors, and found that the fix had reproduced the
defect it documents. **The faulty drafts described below were corrected in place
and never committed, so, like `CRITICAL-REVIEW.md` above, they cannot be
re-read. What can be checked is the producer each one got wrong.**

- It introduced `600s` as a directive. Nothing in this repository establishes
  that value for a verify call: it is a property of one agent harness, written
  into a shared rule that other tools read. (`mk/test-functional.mk` does set
  `ZE_SUITE_TIMEOUT ?= 600s`, a per-suite functional timeout, which is a
  different number for a different thing.)
- It cited `MAX_LOCK_AGE` (1800s) as proof that a 25-to-30-minute run is
  healthy. `verify-lock.sh` derives 1800 from the OPPOSITE premise: its header
  reads `ze-verify targets ~2 min (common case, two-pass strategy), so 30 min is
  well past "something is wrong"`.
- It said to give the call 600s and then re-enter at 600s, which is killing a
  verify on a self-chosen timeout, the thing the same paragraph forbids.
- It attached a true stage count to a false mechanism: `stagesForMode` returns
  the same 24 stages whatever the cache holds.
- It broke the always-on digest. `condense_body` (`scripts/dev/rules_condensed.py`)
  keeps continuation prose from only the first paragraph of a section, so the
  new multi-paragraph text rendered in `ai/rules/CORE.md` as sentences cut in
  half. The two-line text it replaced had survived condensation intact.

The lesson holds and is sharper than first written: **the pull toward authoring
a number is strong enough to catch the person actively writing the rule against
it.** It took three drafts to stop doing it.

Two things the corrected text does, both forced by a reviewer rather than
chosen:

- It quotes the corpus's competing durations ("25 to 30 minutes" in
  `git-safety`, "4-10 minutes" in `testing`) instead of claiming there is no
  number. An earlier draft asserted the absolute, which its own generated digest
  refuted twenty lines further down.
- It puts each directive on ONE physical line. `condense_body`
  (`scripts/dev/rules_condensed.py`) appends the bold-led LINE, not the
  paragraph and not the sentence, so a directive that wraps reaches
  `ai/rules/CORE.md` cut mid-clause. Two successive drafts were truncated there.
  The second one asserted, in this file, that truncation was no longer possible.

## Caveat

One review, one session. The split is clean enough to act on and too small to
call a rate.
