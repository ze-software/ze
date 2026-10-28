# Handover: RFC "Linux cannot do this" requirement state

## Goal

Thomas asked (2026-10-28): "Add a 'Linux can't do this' for requirements".
Today the RFC ledger cannot record a requirement Ze cannot meet because Linux
lacks the primitive, the one exception `ai/META-DECISIONS.md` allows to full
RFC coverage ("a basic primitive Linux cannot provide, named with evidence").
Every unmet requirement is `{gap}`. The goal is a requirement state that
carries that evidence and Thomas's approval, plus a computed per-RFC and
per-profile "attainable ceiling" on the public RFC pages.

## Where it stands

- Spec: `plan/release/validation/spec-rfc-linux-limit-annotation.md`, status
  `design`, committed in `0afee06b2f`, not claimed. Its "Open decisions for
  Thomas" table (D-1 to D-7) is unanswered. Do not implement before he answers
  or approves the recommendations; then `./le spec claim` it and continue with
  `/ze-spec` (approval) and `/ze-implement`.
- Nothing in `rfc/short/` is annotated. The spec's "Candidates found" section
  lists the rows that cite `docs/architecture/linux-limitations.md`; none
  qualifies today (each cited entry is `constrains design`, a library entry, or
  unverified).

## The decisions waiting on Thomas (recommendation in brackets)

| # | Decision | Recommendation |
|---|----------|----------------|
| D-1 | Syntax and evidence | `{linux-limit: LL-<ID>; <why no Ze code substitutes>}`; the cited `LL-` entry in `docs/architecture/linux-limitations.md` must be `Kind: kernel`, `Effect: blocks`, `Status: verified`, name the requirement id in `Blocks`, and carry Evidence |
| D-2 | Per-instance owner approval | `./le commit create` writes `Linux-limit-owed: <RID>: <LL-id>`; the commit lands, the push waits for Thomas's `Owner-attested:` |
| D-3 | What the ceiling subtracts | every row kind the figure can never prove: linux-limit, lower-layer, feature-declined, not-applicable, each as its own share |
| D-4 | Status row | counted as unmet: a `Supported` status is refused, Remaining counts it |
| D-5 | Register parser home | `internal/le/linuxlimit` |
| D-6 | Per-kind switches | add the kind to each (about a dozen); a trait-table refactor would be its own spec |
| D-7 | Card tone | neutral, with its own note |

## Context the spec does not carry

- The public legend sentence `rfcIndexCeiling` (`internal/le/site/rfcfilter.go`)
  used to claim "every RFC here can reach 100%". That was false: `figuresByStem`
  (`internal/le/rfc/profilecoverage.go`) keeps `{not-applicable}`,
  `{lower-layer}` and `{feature-declined}` rows in the denominator unproven
  (RFC 4302 has sixteen lower-layer rows). It was corrected in `b4a36b70be`
  before publication; the published wording now says such RFCs stay below 100%
  with nothing left to do. D-3 replaces that fixed sentence with a computed
  ceiling: update `rfcIndexCeiling`, the overview in `rfcFilterScript`, and the
  tests in `internal/le/site/rfccompliance_test.go` that assert it.
- The RFC compliance index was rewritten for readers on 2026-10-28
  (`d09a3cc477`, `2b02bd4125`): a glossary in `internal/le/site/rfcreader.go`
  and plain check-group lines from `internal/le/rfc/checkgroups.go`. The new
  annotation kind owes a glossary entry there, and its gate checks belong in an
  existing `CheckGroups` group (likely "Requirement annotations"); the group
  test in `internal/le/rfc/checkgroups_test.go` goes red on a check appended
  without a group.
- The per-RFC bars and the profile-link overview (`rfcStemRows`,
  `rfcFigureRowCells`, `rfcFilterScript` in `rfcfilter.go`) sum `data-mp`,
  `data-mt`, `data-op`, `data-ot` per row exactly as the profile rollup does;
  the ceiling needs its own per-row attribute so the overview can sum it too.
- Tests in `internal/le/site` must run with the feature tags or SSH-dependent
  ones fail:
  `tags="ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')"; ./le job run label unit-pkg command go test -tags "$tags" ./internal/le/site/ -skip TestBuildRendersEveryPublishedRoute -timeout 20m -count=1`.
  Some site tests regenerate the real `rfc/requirements/` shards and race each
  other in one run (already recorded in `plan/journal/check-mode-mutates-the-tree.md`);
  re-run a failure of that shape alone before believing it.

## Verification owed at implementation

Scoped package tests for `internal/le/rfc`, `internal/le/site`,
`internal/le/commit` and the appliance linux-limits test; a site rebuild
(`./le site build`, `./le site check`) and a look at the compliance index, a
profile page and a profile link (`../?profile=<name>&rfc=...#rfc-list`).
Pushing the site needs Thomas's word.
