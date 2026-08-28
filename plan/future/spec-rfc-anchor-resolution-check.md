# Spec: rfc-anchor-resolution-check

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `-` |
| Updated | 2026-08-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Refuse an RFC summary that names a section its RFC does not have, and give a
wrong requirement anchor a way to be retired instead of annotated forever.

**The failure is not that `§24.10` looks silly. The failure is that an anchor is
unverified at birth and permanent thereafter.** Nothing reads a section number
when a summary is written, so a wrong one enters. Nothing reads it afterwards
either, so it stays, and every author who cites the summary copies it into
source. The class grows on its own. Only its visible third, the anchors that
number no section at all, is ever noticed; an anchor that IS a real section but
the WRONG one looks correct to every reader and to every count.

`spec-fixit-l2tp-sccrq-tunnel-id-zero` measured both halves of that class in one
package.

| Half | Count in `internal/component/l2tp/` | How it was found |
|------|-------------------------------------|------------------|
| Anchor numbers no section of RFC 2661 | 49 of 226 attributed anchors, in 16 files | a script, in one run |
| Anchor is a real section but the wrong one | 36 sites in round 4, 31 more in round 5, 3 more in round 6 | five review rounds by hand, and each round still found a class the round before it had swept for |
| Citation names a guide TRAP number and no section at all | 3 sites in round 6 | reading the file while fixing its header; no count can see one |

The second row is the point. A dedicated sweep for exactly this defect missed
nine sites the next reviewer found, because every one of them was numerically in
range. Reading is not a control for this. A machine has to hold the anchor
against the text.

The supplier was traced: `rfc/short/rfc2661.md` took its headings and eight
checklist anchors from `docs/research/l2tpv2-implementation-guide.md`, whose
numbering runs to Section 26 against RFC 2661's 13. Every later author copied
them onward in good faith.

## Two parts, and why they are one spec

### (a) Enrolment-time anchor resolution

Every `(§N)` anchor in `rfc/short/<stem>.md` must resolve to a real section of
`rfc/full/<stem>.txt`.

This is mechanical and it is the part that scales. It would have refused §24.10,
§24.12 and §15 on the day they were written, before any of the nine RFC 2661 ids
became frozen and before a single source comment copied them.

Two facts the design must carry, both measured on RFC 2661:

- **A top-level section is numbered `N.0` in the text and cited as `N` in prose.**
  RFC 2661 heads its sections `1.0` through `13.0`. A resolver that compares
  literal strings reports every correct `Section 8` as unresolvable.
- **The check owes a report before it owes a block.** The enrolled corpus is 170
  RFCs. Running the resolver over the corpus at rest, and publishing the
  per-RFC count, is what tells the owner whether arming it is a one-commit
  change or a project.

The natural extension, once the resolver exists, is the `// RFC:` file header in
source: every number in one should resolve to a section AND to a site in that
file, and every section a site cites should appear in the header.
`spec-fixit-l2tp-sccrq-tunnel-id-zero` applied that two-sided property by hand to
seven files over three rounds, and the round that WROTE the rule broke both
halves of it in the same pass: it added an unsupported number to one header and
added sites to three files without touching theirs. A header is a two-sided
invariant that an edit to either side can break, which is the case for checking
it by machine rather than by care.

### (b) An audited id-rename map

A requirement id is frozen twice over. `_validate_id` in
`internal/le/rfc/rfc.go` locks the id to its trailing `(§N)` anchor, and
`check_retired_requirements` refuses an id that leaves an enrolled summary. So a
wrong anchor cannot be corrected. RFC 2661 carries nine ids in that state, each
one now annotated with a sentence saying the anchor is frozen and naming the real
section in the requirement TEXT.

That annotation is a workaround and it is honest about being one. It costs a
sentence per requirement, it is invisible to any machine, and it grows with the
class.

The fix is a rename map the ratchet consults: an audited, append-only record that
`RFC2661-24.10-1` became `RFC2661-4.4.3-N`, so `check_retired_requirements` sees
a rename rather than a deletion.

**This is a separate part, not a separate spec, because (b) is what makes any
wrong anchor repairable at all.** The annotation route does not repair (a), and
it is worth being exact about why: (a) reads the ANCHOR and the annotation edits
the TEXT. `extract_section` takes the trailing parenthetical first, and
`_validate_id` locks the id's head to whatever it returns, so a sentence added to
the requirement text leaves `(§24.10)` exactly where it was. That route is the
one `check_retired_requirements` blesses in as many words, and it is not a
workaround for a resolver over anchors: it is invisible to one. Armed over a
frozen id, (a) is a red gate with NO repair, the annotation included. The id
cannot change, because `_validate_id` refuses a head that disagrees with its
anchor. The line cannot go, because `check_retired_requirements` refuses an id
that leaves an enrolled summary.

**The coupling is narrower than "ship them together", and R-3 is why.**
Grandfathering (a) by SCOPE, new-since-HEAD in the `check_new_summaries` shape,
puts the nine frozen RFC 2661 ids outside (a)'s population: none of them is new,
so nothing goes red and the two parts separate cleanly. What needs (b) first is
arming (a) BEYOND new-since-HEAD scope.

It touches a ratchet's escape hatch and every `RFC requirement:` tag in the tree,
which is why it is scoped and reviewed here rather than folded into the commit
that found the class.

## Non-goals

- Judging whether a requirement is correctly CLASSIFIED. The resolver reads the
  anchor, never the disposition.
- Reading section numbers out of prose in `docs/`. The supplier was a research
  document, and a resolver over free prose is a different problem with a
  different false-positive profile.

## Risks

| Id | Risk | Mitigation |
|----|------|-----------|
| R-1 | The resolver reports a real anchor as unresolvable because the RFC's own heading format differs (`N.0` vs `N`, appendices, RFCs with no numbered sections) | Report first, over the whole enrolled corpus, and read the false positives before arming anything |
| R-2 | (b) lets a rename launder a requirement out of existence | The map is append-only and audited: a rename records both ids, and the ratchet checks the NEW id is present |
| R-3 | Arming (a) makes a large number of enrolled RFCs red at once | The corpus report from (a) sizes this before the decision. Grandfathering is SCOPE (new-since-HEAD), the shape `check_new_summaries` already uses, never an allowlist file |
