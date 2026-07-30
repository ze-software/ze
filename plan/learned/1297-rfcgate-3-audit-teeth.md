# 1297 -- rfcgate-3 -- giving the RFC audit record teeth

## Context

`make ze-rfc-check` proves a LINK: this RFC requirement id has a positive and a negative test
tagged against it. It cannot read either test. The judgement of whether a test would actually fail
if the implementation stopped complying belongs to the `/ze-rfc-audit` skill. That judgement is
recorded per requirement in `rfc/audit/<rfc>.json`. That record had no teeth.

`verdict_is_fresh` was the freshness rule as it stood before this work. It compared
`requirement_sha` and the `tests` map, and never read `verdict["verdict"]`. That name is no longer
in the tree, and the last Gotcha says why. A verdict recorded `weak` or `wrong` was therefore
treated exactly like `enforced`. The skill itself calls those two "the valuable outputs".

The vocabulary had already drifted to a fifth word (`implemented`), and nothing noticed, because
nothing looked. `load_audit` was a bare `json.load` returning `data.get("requirements", {})`: no
field check, no enum check, no check that a recorded id existed. Three verdicts carried an empty
`tests` map, so their freshness test reduced to `{} == {}` and they can never go stale. And the
semantic half covered ONE enrolled RFC out of 166, published nowhere.

Meanwhile the file-level fingerprint made a verdict stale on any edit anywhere in a tagged file.
Six of a pending sixteen commits to the one audit file were mechanical re-stamps. Nothing about
what a test asserts had changed, and not one of them changed a verdict. At fleet scale that trains
the reflex -- re-stamp when the gate goes red -- and the reflex is what fails, not the reading.

The goal was machinery only:

- Make the verdict value load-bearing, and do not make honesty the expensive path.
- Validate the record fail-closed.
- Make an unfalsifiable verdict impossible.
- Remove the human from every no-judgement re-stamp.
- Publish the coverage figure derived rather than maintained.

## Decisions

- `weak` and `wrong` are PUBLISHED at recording time, never gated, over failing the build on any
  non-`enforced` verdict: failing immediately is safe only because zero such verdicts exist today,
  and the first person it would bite is the honest auditor. Four independent tests assert that
  recording a finding exits 0.
- Findings are nonetheless PERMANENT -- deleting one, or upgrading it to `enforced` with every unit
  fingerprint unchanged, is red -- over leaving them advisory. The cost falls on erasure, not on
  reporting. This is `check_coverage_ratchet`'s shape applied to judgement instead of tags.
- The verdict ratchet keys on PRESENCE, and is deliberately stricter than the spec. The spec
  protected only a verdict that was FRESH at HEAD. Staleness is exactly the state in which
  deletion is most tempting and least honest. Exempting it would aim the ratchet away from its
  own case.
- A `wrong` or `unimplemented` verdict must be disclosed in `docs/features/rfc-status.md`, over
  treating disclosure as the annotation's job. `check_status_agreement` already refuses to let a
  `{gap}` hide under a clean "Supported" row. A verdict saying the same thing must not be weaker.
  `weak` is deliberately excluded -- it is a claim about the TEST, not the code.
- Freshness gains a `shifted` state whose remedy is mechanical. This is over F18's own proposal
  (keep the boolean, improve the message), and over dropping the file hash. A better message
  leaves the human step in place. Dropping the file hash loses the one real class the unit hash
  misses.
- The tagged unit is defined ONCE, in `scripts/dev/rfc_tagged_scope.py`, imported by both the gate
  and the edit-time hook. This is over copying the span logic. `reseal_rfc_audits`'s own docstring
  names the hazard: a second copy that drifted would re-seal against a hash the gate does not
  compute.
- A DEDICATED `make ze-rfc-reseal` is the only writer of `rfc/audit/`, over folding the re-seal into
  `ze-rfc-index` (this spec's own earlier position, superseded by owner ruling): `ze-rfc-index` runs
  routinely for reasons unrelated to an audit, so re-sealing there would AUTOMATE the blind
  re-stamp reflex the work exists to remove.
- Owner ruling OR-1: the enum gains a FIFTH value, `not-applicable`, over re-judging the one record
  that had no legal state and over re-deriving its annotation. It requires three facts in two
  files, where `enforced` needed one word:
  - an absent-or-empty `tests` map
  - a mandatory `no_code_path` prose reason
  - an independently committed `{not-applicable}` annotation

  The reason lives in a DEDICATED field rather than the generic `note`, because AC-17 constrains
  only `enforced` notes. A `not-applicable` note would otherwise have been unconstrained prose.
  `{gap}` does NOT satisfy it: `{gap}` says Ze can comply and does not, and `{not-applicable}`
  says nothing can.
- AC-24's subtraction lives in a NEW audit section, never in the polarity rollup, over subtracting
  from the rollup's **Both** column. **Both** answers which polarities EXIST, and a `weak`-verdict
  requirement genuinely has both. Removing it would make the column false. It would also break the
  partition `scripts/dev/testing_health.py` asserts rather than degrades.
- `.py` tag carriers are file-scoped BY DECLARATION (`scope_reader`), over writing a Python span
  parser for two tags. File scope is strictly MORE sensitive than function scope. Declaring it can
  only over-trigger a re-read, and never under-trigger a false fresh. Previously `.py` fell to
  whole-file scope only because the Go span finder finds no `func` in Python. That was the right
  answer for the wrong reason, and it would have changed silently the day anyone taught the finder
  about `def`.
- The one re-judgement (`RFC7606-5.1-2`, `implemented` -> `enforced`) was READ, not mapped, over
  rewriting the illegal value mechanically: a mechanical rewrite fabricates a verdict nobody
  reached, which is the exact failure this machinery exists to prevent. Its would-it-fail is
  recorded from MUTATION, not inference.

## Consequences

- The verdict value is now read by six checks.
- The `code` map makes the previously unfalsifiable class stale when a cited producer moves.
- The record's schema is validated on read, so an agent-authored file is no longer trusted input to
  a build gate.
- `make ze-rfc-reseal` is a new habit with a cost: clearing a `shifted` verdict is TWO commands
  (`ze-rfc-reseal`, then `ze-rfc-index` for the ledger). Each red names the one command that clears
  it, which is the deliberate price of keeping every write to an evidence file intentional.
- Published audit coverage is 49 of 1344 auditable requirements (3.65%). A fleet performing the rest
  writes into a schema that refuses the cheap fakes, but nothing here proves a human read an RFC.
  Every check is a proxy, and saying so is part of the design.
- `ai/RFC-REQUIREMENTS.md` now depends on `rfc/audit/*.json`, so an audit edit without
  `make ze-rfc-index` reds `check_ledger_fresh`. This is precedent, not novelty: child 1 already
  made the ledger depend on the hand-authored `rfc/extraction/`.
- The edit-time guard now fires for ANY tag carrier holding a tag, with the carrier list derived
  from the shared leaf. Until this work the predicate was a literal covering `_test.go` and a
  `/test/` `.ci`. The two interop `check.py` files child 2 admitted as evidence therefore carried
  RFC obligations the guard did not see AT ALL.
- `RFC7606-8-1` remains open on the merits and says so on its own record: `ai/rules/rfc-compliance.md`
  voids `{not-applicable}` as AUTHORITY, so the annotation this verdict agrees with is itself a
  classification the owner has voided. OR-1 makes the VERDICT honest about what the code does. It
  does not re-affirm the annotation, and re-deriving that is fleet-drain work under the rfcgate
  umbrella's D4.

## Gotchas

- **The documented way to use a brand-new state produced a permanently red gate with a lying
  remedy.** OR-1's `not-applicable` verdict cites no test, and `ai/skills/ze-rfc-audit.md` tells the
  author to OMIT the `tests` field. An absent key compared unequal to the computed `{}` (`None == {}`
  is False). A record authored the documented way therefore read `STALE_UNIT` forever, and its
  error text was false in all three of its clauses:
  - No tagged test was gone, since the state FORBIDS citing one.
  - It was not a line shift.
  - Re-running `/ze-rfc-audit` reproduces the identical record.

  A `--reseal` refused it as well, so nothing cleared it. Every test wrote the `{}` spelling and
  passed. The normaliser that fixes it, `_sha_map`, already existed for the LOAD path. Two
  comparison sites bypassed it, because each was written before the state that needed it. **When you
  add a state, drive the DOCUMENTED authoring path end to end: the schema accepting both spellings
  does not mean every consumer does.**
- **An honest number can be lower than the flattering one.** Audit coverage was reported as 4.52%
  (44 of 974). The denominator counted polarity from TAGS alone and never read `req.annotation`.
  The schema meanwhile happily judged a `{single-polarity}` requirement. So 370 requirements the
  auditor can legally judge fell outside `Auditable`.

  Counting properly moved the denominator to 1344 and the figure to **3.65%**. The reported number
  was higher because it was wrong. Five recorded verdicts (`RFC7606-3.h-1`, `-3.h-2`, `-5.1-3`,
  `-7.14-2`, `-7.15-2`) were being counted in NO column at all, and being fresh `enforced`, in no
  worklist row either.
- **The gate's own green line said every verdict it held was proven.** It printed
  `0 audited-but-not-proven` on a tree holding two `unimplemented` gaps and one `not-applicable`,
  while the LEDGER named three. The CLI summed `r.findings` and discarded the worklist it had just
  computed. This is the failure the whole spec set exists to remove: a mechanical count read as a
  semantic assurance. It was reproduced inside the check meant to remove it. The correct principle
  was already stated in a comment directly above the offending line ("a gate that reports OK while its
  judgement half covers one RFC in 166 is telling a reader something it has not measured").
- **A guard whose docstring names exactly what it prevents can still be untested.** `_unit_identity`
  compares a MULTISET of `(file, unit-sha)` so that deleting one of two tags inside the same function
  changes the count. Collapsing it to a set survived all 488 tests at the time. Four requirement ids
  carrying verdicts are reachable through it today (`RFC7606-2-5`, `-3.d-1`, `-3.f-1`, `-5.2-1`).
  Each has two tags in one file resolving to one unit, so the false FRESH was live, not
  hypothetical.
- **A fix that is handed one site MUST look for the second.** Every one of the three defects found
  by review had a wider reach than the report that surfaced it:
  - The absent-versus-empty bug had TWO bypassing comparison sites, not one.
  - The coverage-denominator bug had a second hole. The RECORD walk was gated on `req.gated`,
    though a verdict is schema-legal on any requirement of the RFC.
  - The multiset guard's reachability was four requirement ids rather than the one example given.

  Ask what ELSE reads this value before you call the fix complete.
- **`_write_audit`'s comment promised a byte-level re-read its code did not perform**: it validated
  the in-memory dict, not the staged bytes. That is not the same guarantee when `--check` reads the
  file, and a JSON round trip is exactly where a writer defect hides. Both are checked now, the dict
  first, since it still refuses things the round trip would launder.
- **A test suite over dead code reads exactly like coverage.** `verdict_is_fresh` was meant to
  survive as the single spelling of the pre-`units` file-level rule. The new four-state
  `verdict_freshness` was to delegate to it, so that "the pre-`units` behaviour cannot drift". Two
  docstrings said so. No delegation was ever written: the function had ZERO non-def call sites, and
  `verdict_freshness` re-implements the rule inline in its `if not units_recorded` branch.

  The two spellings had already drifted in exactly the way that docstring ruled out. Only the live
  branch consults the `code` map. So a verdict with `code` set, no `units` and unchanged `tests`,
  whose producer moved, yields two different answers. The live path returns `('stale-unit', [...])`,
  and the dead one returns `True`.

  Ten assertions exercised the dead spelling and passed, so the module looked covered where it was
  not executed. The transitional rule the closure audit first cited as AC-20's evidence named a call
  that did not exist. Found by a SECOND review pass, after the first reported clean.

  The fix deletes the helper (`ai/rules/no-layering.md`: delete the old spelling, do not keep it
  beside the new one). It RE-POINTS the assertions at the live branch rather than deleting them
  (`ai/rules/no-test-deletion.md`). That revealed a fifth case the old spelling cannot express:
  the boundary at which a recorded `units` map leaves the transitional branch. Net effect on the
  suite: 520 tests -> 521, and one confirmation that the helper was dead -- `--check` prints
  byte-identical figures with it removed.

  Two transferable rules. **A docstring's account of which function calls which is a belief, not a
  wiring -- grep for call sites.** `ai/rules/no-fabrication.md` already bans citing a comment as
  design intent, and this is the same ban one level down, about the call graph. **When you keep an
  old function "as the one spelling", the delegation IS the deliverable.** If it is not written,
  you have two spellings and a comment claiming otherwise, which is strictly worse than one.
- **`make ze-verify` was not green at closure, for reasons measured to be outside this change.** A
  concurrent session held uncommitted edits to four `rfc/short/*.md` summaries, adding gated MUSTs
  that are neither proven nor annotated. The generators that MUST be regenerated
  (`ai/RFC-REQUIREMENTS.md`, `ai/CODE-TO-DOCS.md`) read the whole working tree, so they absorb
  another session's rows. Teaching a correct gate to accommodate a transient foreign tree is the
  wrong fix. The umbrella's Sequencing Constraint (never two children in flight) exists precisely to
  prevent this, and it was not honoured.

## Files

| File | What |
|------|------|
| `scripts/dev/rfc_requirements.py` | The whole gate: `_validate_verdict`, `load_audit`, `check_audit_files`, `check_audit_schema`, `_verdict_claims`, `recorded_map`, `unit_shas`, `_unit_identity`, `verdict_freshness`, `audit_freshness`, `check_audit_freshness`, `reseal_audits`, `_write_audit`, `run_reseal`, `check_audit_disclosure`, `check_audit_findings`, `check_audit_verdict_ratchet`, `check_audit_note`, `polarity_covered`, `audit_coverage`, `_render_audit_coverage` |
| `scripts/dev/rfc_tagged_scope.py` | NEW. The single definition of "the tagged unit": `unit_at`, `scope_reader`, `go_func_scopes`, `is_tag_carrier`, `tag_scope` |
| `scripts/dev/rfc_requirements_test.py` | 34 audit test classes, 156 cases, plus `TestTransitionalFileLevelRule` holding the re-pointed transitional cases. The whole selftest is 521 |
| `scripts/dev/rename_module_path.py` | `reseal_rfc_audits` delegates to the shared re-seal, keeping `rename_only_since_head` as its extra per-file proof |
| `scripts/dev/rename_module_path_test.py` | `ResealDelegates` (4 cases), including "the predicate must be the real proof, not a lambda that says yes" |
| `.claude/hooks/pretool-writeedit.py` | Loads the shared leaf. The `rfc-tagged-test` carrier predicate is now derived, not a literal |
| `Makefile` | `ze-rfc-reseal`, the only writer of `rfc/audit/` |
| `rfc/audit/rfc7606.json` | 4 record corrections plus a 49-verdict `units` backfill |
| `ai/RFC-REQUIREMENTS.md` | Generated `## Audit coverage` section and its `### Audited but not proven` worklist |
| `ai/skills/ze-rfc-audit.md` | The five-value vocabulary and the new fields, held to the code by two gate tests |
| `ai/rules/hook-mapping.md`, `ai/INDEX.md`, `docs/contributing/rfc-implementation-guide.md`, `docs/functional-tests.md` | Discovery: the new target, the shared leaf, and what the audit record now guarantees |
| `plan/deferrals/rfc-gate-regression-ratchets.md` | The 2026-07-20 "skip it" row recorded VOID as authority and superseded |
