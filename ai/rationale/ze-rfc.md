# Rationale: the RFC summary skill

`ai/skills/ze-rfc.md` holds the steps an agent runs to write or change an RFC
summary. This page holds the reasons and the history behind some of them. Each
section is named by the step that needs it, so the reason is one read away and
the step does not carry it.

## Why enrolment requires the source text

A summary written without its source is validated only against itself, which
permits both a requirement the RFC does not contain and, invisibly, one it does
contain that was never extracted.

## Why a new summary is enrolled in the same change

Writing obligations down and gating none of them is how a compliance claim
rots.

## Why the extraction walk is recorded

The old two-grep, eyeball-the-ratio coverage check was an honour-system count
that nothing recorded and nothing re-checked. It had caught real failures and
would have caught more: `rfc5303`, `rfc5304` and `rfc5310` shipped summaries
with 23, 13 and 12 normative keywords in the source and zero captured. A ratio
you eyeball says "roughly enough"; a classified site list says which sentence,
by name.

## Why the skeleton lands in the scratch

An unclassified artifact under `rfc/extraction/` reds the gate for the whole
corpus, and generating a batch of them would red it for everybody at once.

## Why the ledger is not committed

Until 2026-09-11 both outputs were owed in the same commit, and the cost was paid nine times in
`plan/journal/concurrent-rfc-gate-stale.md`: this checkout is shared, a
regeneration renders every session's summaries at once, and the author either
published another session's requirement rows under their own message or charged
a verification-debt row for not doing so.

## Why the id carries the section, not a counter

RFCs are immutable: section numbers are frozen the day the RFC publishes, which
makes them the most stable name available. A bare counter encodes allocation
order, a fact about our editing history, not about the RFC. It teaches a reader
nothing, its neighbours are unrelated requirements, and the moment you add a
requirement (which must go at the end, since renumbering is forbidden) the
numbering stops tracking the document at all.

## Why lower-layer names a producer

The producer demand is the whole difference from `{not-applicable}`: a
judgement nobody can check is what grew that kind to 915 sites the owner ruling
presumes are mostly wrong.

## Why the successor label is checked by one word

The refusal of any other field name containing `obsolet` exists because the
reader used to know one spelling: the corpus wrote four, and 93 requirements of
three enrolled summaries were gated as current documents until the label was
widened.

No summary writes `Superseded by`, `Replaced by` or `Successor` today, and
widening the word waits on a survey of what the corpus writes, because the
field-name pattern reads the first cell of every table row and a looser word
would collide with the requirement tables.

## Why only the test side of a link is authored

A hand-written back-link survives deletion of the test it names; a tag dies
with the test.

## Why a requirement needs both polarities

A negative-only test passes if the code rejects everything; a positive-only
test passes if it accepts everything. Only the pair pins behavior to the
requirement.

## Why a table case carries its own tag

A function-level tag on a function that covers many requirements stays green
after the one enforcing case is deleted, which is exactly the rot this system
exists to catch.

## Why a tagged test needs a judge

`ai/rules/testing.md` already says fix the code, not the test. For RFC-tagged
tests this is enforced, because a test that encodes a standards obligation is
the only thing standing between a regression and a shipped protocol violation.
