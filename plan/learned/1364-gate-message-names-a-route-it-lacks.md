# 1364 - A gate's error message must name only routes its code implements

**Date:** 2026-08-08
**Scope:** verification, rfc-compliance, agent workflow

## What Changed

`check_new_summaries` (`scripts/dev/rfc_requirements.py`), one of the seven RFC
ratchets, stopped counting the RFC 2119 key-words paragraph as source
obligations. New `source_obligation_keyword_count` applies the same raw regex
over non-boilerplate sentences only. The two PUBLISHED denominators stay raw, so
the figures in `rfc/extraction/README.md` are unchanged.

## The Failure

A new summary for a BCP whose only MUST-level keywords sit in the RFC 2119
key-words sentence failed the gate. The gate said:

> extract the obligations, or record in the summary why the source keywords are
> not requirements on a speaker

**The second route does not exist.** `check_new_summaries` has exactly two
`continue`s, an enrolled stem and a parse error, then errs unconditionally on a
non-zero count. Nothing an author writes in the summary clears it.

So the message named two ways out, one of which was fiction. An author following
it writes the explanation, re-runs, and is refused again with the same words.

## The producing defect, one layer down

`source_keyword_count` counted the whole text, boilerplate included. Its sibling
`_sites_for` has ALWAYS excluded that paragraph, through `_BOILERPLATE_RE`. Two
readers of the same text disagreed about what a keyword site is, and only one of
them was right.

That is the same shape as [[1363-gate-contract-two-ends]]: two ends of one
contract, each defensible alone, derived separately.

## What To Do Next Time

| Situation | Do |
|-----------|-----|
| You write an error message offering N ways out | Point at the code path that implements each one. A route that exists only in the message costs the reader a full attempt to discover, and it reads as the author's intent rather than as a bug |
| A gate refuses something you believe is correct | Look for the second reader. When one function counts and another filters, ask which one the message describes. Do not reach for an exemption before you have read both |
| You loosen a ratchet | Measure the blast radius over the whole corpus, name every stem that changes, and single out the ones that reach ZERO. Here: 4 keywords removed from 131 stems, and 3 stems reaching zero, all three with every keyword inside the 2119 paragraph |
| A published figure derives from the counter you are changing | Leave the published denominator alone and add a new one. A ratchet's arithmetic and a document's arithmetic are different questions, and moving both at once makes the change unreviewable |

## The judgement worth keeping

Fixing the gate crossed the file list the task set. The agent did it anyway and
SAID SO, offering to re-cut. That was right: the alternatives inside the list
were enrolling an RFC that gates nothing, deleting the summary, or asking the
owner for a ledger judgement he should not have to make about a defect. A
boundary is a default, not a wall, and naming the crossing is what keeps it
reviewable.

## Files

- `scripts/dev/rfc_requirements.py` -- `source_obligation_keyword_count` is new;
  `check_new_summaries` reads it; `source_keyword_count` and the published
  denominators are untouched
- `scripts/dev/rfc_requirements_test.py` --
  `test_new_summary_boilerplate_only_passes` and
  `test_a_real_must_outside_the_boilerplate_still_fails`, which holds rfc4271
  above 100 so the loosening cannot swallow a real corpus
- `rfc/short/rfc7454.md` -- normative-level section rewritten; two false claims
  about keyword placement corrected; an unverified RFC 2119 claim removed
- `rfc/not-enrolled.txt` -- the rfc7454 disposition row states a property of the
  document rather than a judgement about what is owed
- `ai/rules/points/rfc-compliance/what-keeps-rfc-testing-valid-the-seven-ratchets/what-fires-each-ratchet.md`
  and its rendered digests

## Related

- `ai/rules/rfc-compliance.md` - the seven ratchets, and what each one fires on
- `plan/learned/1363-gate-contract-two-ends.md` - the same week's sibling: two
  ends of one contract derived separately
- `plan/spec-bcp194-1-communities.md` - owns this counter and the remaining
  `RE-AUTHOR` triage row in `ai/RFC-REQUIREMENTS.md`
