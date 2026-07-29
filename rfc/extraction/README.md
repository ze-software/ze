# Extraction sign-offs

One file per RFC, `rfc/extraction/<stem>.json`. Written as an unclassified skeleton by
`make ze-rfc-extract STEM=<stem>`, classified by hand, and re-checked by
`make ze-rfc-check` on every run.

Spec: `plan/spec-rfcgate-1-extraction.md`. Rule: `ai/rules/rfc-compliance.md`,
"Extraction Completeness".

## What this artifact is

Every other check in `scripts/dev/rfc_requirements.py` judges the requirements a summary
**lists**. None of them can see an obligation nobody wrote down, so a green gate is
bounded by what was extracted, and until this artifact existed nothing anywhere bounded
what a summary **missed**.

A sign-off closes that by re-reading the RFC's own text. Every normative site the
extractor can see is either `mapped` to a requirement id in `rfc/short/<stem>.md` or
`excluded` with a kind from a closed set and a reason (the **forward** arithmetic, which
catches a missed obligation), and every gated requirement the summary declares is the
target of some site or is declared `unsourced-ids` on the section it was read from (the
**reverse** arithmetic, which catches an invented one).

## What this artifact is NOT

**It is not a bound over obligations.** The forward arithmetic proves every site the
extractor can *see* is accounted for. Those differ by the extractor's recall, and recall
can be near zero for a section written in indicative prose: RFC 4271 Section 8.2.2 is
35168 characters of the most load-bearing state machine in the product and contains
exactly one capitalised MUST-level keyword. `unsourced-ids` is how a reviewer records an
obligation the extractor cannot see. The residual is published rather than claimed away.

**A `manual-walk` sign-off is an assertion the gate cannot verify.** It checks that the
section walk is complete against the derived section list and that the source sha is
pinned. It cannot check that the reviewer read anything. Such a sign-off still earns drain
credit -- an RFC whose own authors wrote no RFC 2119 keywords must have *some* route out
of the backlog -- and the honesty is carried by the published register column, not by
withholding the credit.

**It does not judge whether a requirement's text renders its source sentence faithfully.**
The artifact makes the source-sentence-to-requirement pair explicit, which is what a
reader needs in order to judge it. The gate checks that the link's endpoints exist; it
never claims to judge the rendering.

## Derived versus authored

Only **dispositions and reasons** are authored. `sites`, `sections`, `quote`, `register`,
`source-path`, `source-sha` and every published count are DERIVED from the source text at
check time and compared against what the artifact records
(`ai/rules/derive-not-hardcode.md`). A hand-typed "sites seen" number would be a claim,
and claims are what this programme exists to remove; editing a derived field to make a red
go green fails the check naming the field and the locator.

## Why a generated skeleton can never pass

The writer emits `"disposition": null` for every site and every section, and an
unclassified site FAILS the check. There is no `--sign-off` mode, no default disposition
and no bulk classifier, so generating artifacts en masse makes the gate **redder**, never
greener. That is a structural answer to a social failure mode, which is what the
2026-07-20 owner ruling in `plan/deferrals/rfc-gate-regression-ratchets.md` asked for.

## The register

Derived from the source text, never authored. An artifact may declare the derived register
or a **weaker** one; a stronger claim is refused.

| Register | Derived when | What the sign-off rests on |
|---|---|---|
| `rfc2119` | the source has capitalised MUST-level keywords outside the RFC 2119 boilerplate, and at least as many sites as the summary declares gated rows | the full forward and reverse arithmetic over a keyword inventory |
| `prose` | no capitalised keyword, or fewer sites than gated rows declared | the same arithmetic over a noisier case-insensitive modal inventory |
| `manual-walk` | the inventory is empty under both scans while the summary declares a gated requirement | the reviewer's declared section walk, plus a stated reason why no inventory exists |

Measured over the 166 enrolled RFCs on 2026-07-29: 101 derive `rfc2119`, 64 derive
`prose`, and 1 (`rfc1877`) derives `manual-walk`. 23 have no capitalised MUST-level
keyword SITE at all while declaring 172 gated MUSTs between them, which is why a
keyword-only design would have been vacuously green for a large minority of the corpus.

Every figure above is derived by driving this module's own parser over `rfc/enrolled.txt`
-- `derive_inventory(stem, gated)` per stem, register taken from the returned inventory,
and the 23 selected by `keyword_sites == 0` with their gated totals summed from
`gated_counts` -- never retyped from a previous run.
<!-- source: scripts/dev/rfc_requirements.py — derive_inventory, derive_register, gated_counts, source_keyword_count -->

**Two denominators, both correct; do not reconcile them into one number.** The figure
above counts SITES: normative *sentences* after the RFC 2119 boilerplate is excluded. A
second measurement counts raw uppercase keyword *occurrences* anywhere in the source
(`source_keyword_count`), and on the same day it gives **22 stems declaring 164 gated
MUSTs** -- the figure `plan/spec-rfcgate-0-umbrella.md` records as one of its "three
pre-2119 measurements". The whole difference is `rfc5443` (8 gated MUSTs): its only four
uppercase occurrences sit inside its own "Conventions Used in This Document" paragraph,
which the site scan correctly refuses to count as an obligation. 164 + 8 = 172. Quote the
site figure when talking about sign-off arithmetic, because sites are what a sign-off
classifies; quote the occurrence figure only when talking about raw keyword presence.
<!-- source: scripts/dev/rfc_requirements.py — _sites_for, _BOILERPLATE_RE, source_keyword_count -->

*(Corrected 2026-07-29: this paragraph previously read "168 gated MUSTs", which is neither
denominator's answer. Re-measured against the producing code, `ai/rules/documentation.md`
Source Anchors.)*

## Fields

| Field | Authored or derived | Meaning |
|---|---|---|
| `schema-version` | authored | `1` |
| `stem` | authored, cross-checked | must equal the filename stem |
| `register` | authored, cross-checked | never stronger than the derived register |
| `register-reason` | authored | required for `manual-walk`: why no mechanical inventory exists |
| `source-path` | derived | `rfc/full/<stem>.txt` or `rfc/drafts/<stem>.txt` |
| `source-sha` | derived | sha256[:16] of the normalized source; a change stales the sign-off |
| `signed-off` | authored | the date the walk was performed |
| `reviewer` | authored | who performed it |
| `resign-reason` | authored | required only when the exclusion count rises above HEAD |
| `sections[]` | mixed | `id` and `sites` derived; `disposition` (`walked`/`skipped`), `skip-kind`, `reason` and `unsourced-ids` authored |
| `sites[]` | mixed | `id` (`<section>:<n>`) and `quote` derived; `disposition` (`mapped`/`excluded`), `mapped-to`, `excluded-kind` and `reason` authored |

`signed-off`, `reviewer` and `register-reason` are required to sign off, not to parse: an
unsigned skeleton is a legal intermediate state, so the check can be run mid-walk to see
which sites are left.

### Exclusion kinds (closed set)

| Kind | Means | Extra obligation |
|---|---|---|
| `not-a-requirement` | the keyword is in non-normative use: a quotation, a description of another system, boilerplate the extractor did not strip | the reason names which |
| `binds-another-role` | the obligation binds a role Ze does not implement (a CA, a registry, an IANA action, the peer) | the reason names the role |
| `duplicate-of` | restates an obligation already captured | `mapped-to` must name an id that some OTHER site maps |
| `cross-document` | the obligation belongs to another RFC the sentence cites | -- |
| `advisory-in-context` | the capitalised keyword sits inside a SHOULD/MAY construction the splitter mis-cut | the reason quotes the enclosing construction |

### Section skip kinds (closed set)

`front-matter`, `references`, `iana`, `acknowledgements`, `appendix-non-normative`.

## Ratchets

The set of stems with a valid sign-off may not shrink, and a signed stem's exclusion count
may not rise without a `resign-reason` and a bumped `signed-off` date. Both compare
against HEAD. Exclusions are shrink-only rather than capped at a ratio, because a cap is
gamed by rewording and picks a number nobody can defend; the per-RFC exclusion ratio is
published in `ai/RFC-REQUIREMENTS.md` instead, so the pressure is directional and the state
is visible.

## Grandfathering

Implemented as SCOPE, never as an allowlist file. `check_extraction_signoff` judges a stem
when the stem has an artifact, and `check_enrolment` demands one only for a stem enrolled
since HEAD. Nothing had to be added to a list of exceptions when the first RFC stopped
being an exception, and nothing has to be removed from one when the last does.
