# Spec: rfcgate-3 audit teeth

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | spec-rfcgate-1-extraction, spec-rfcgate-2-evidence (the umbrella's merge order 1, 2, 3, 4) |
| Phase | 9/9 |
| Deferral shard | `plan/deferrals/rfcgate-3-audit-teeth.md` |
| Updated | 2026-07-29 |

Part of the `rfcgate` spec set. Umbrella: `plan/spec-rfcgate-0-umbrella.md`.
Siblings are referenced by name only. This spec is independently implementable.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`make ze-rfc-check` proves a **link**: this requirement id has a positive and a
negative test tagged against it. It cannot read either test. The judgement of
whether a test would fail if the implementation stopped complying is the
`/ze-rfc-audit` skill's, and it is recorded per requirement in
`rfc/audit/<rfc>.json`.

That record currently has no teeth. Four defects, each verified against the
source rather than inferred:

**D1. The verdict value is inert.** `verdict_is_fresh`
(~~`scripts/dev/rfc_requirements.py:1227-1236`~~ -> `:1877-1886`, 2026-07-29) compares exactly two things:
`verdict.get("requirement_sha")` and `verdict.get("tests")`. It never reads
`verdict["verdict"]`. ~~`grep -rn '\["verdict"\]\|\.get("verdict"' scripts/ mk/
Makefile` returns a single hit, `scripts/dev/rename_module_path.py:386`, and that
is a WRITE of `verdict["tests"]`, not a read of the value.~~ **Corrected 2026-07-29: that
grep returns ZERO hits. The write at `rename_module_path.py:386` is `verdict["tests"] = fresh`,
which the quoted pattern does not match. The defect is unchanged and the evidence is stronger:
no code anywhere reads the value.** So a verdict recorded
`weak` or `wrong` is treated by the gate exactly as `enforced` is. The skill that
produces these records says the opposite of what the gate does:
~~`ai/skills/ze-rfc-audit.md:94`~~ -> `:96-97` (2026-07-29, the skill file is
unmodified since HEAD, so this citation was off by two when written) states that "`weak` and `wrong` are the valuable
outputs. A run that returns all `enforced` on first pass has probably not read
anything." The one mechanism designed to surface a bad test writes its findings
into a field no code reads.

**D2. No schema validation.** `load_audit`
(~~`scripts/dev/rfc_requirements.py:1239-1249`~~ -> `:1889-1899`, 2026-07-29, shape unchanged) does a bare `json.load` then
returns `data.get("requirements", {})`, discarding every other key and never
inspecting them. There is no field-presence check, no enum check on `verdict`, no
check that a recorded rid resolves to a real requirement of that RFC, and no
check that a requirement carrying tags has a non-empty `tests` map. The
vocabulary has already drifted: `rfc/audit/rfc7606.json` records `implemented`
for `RFC7606-5.1-2`, which is outside the four values defined at
`ai/skills/ze-rfc-audit.md:67-72` (`enforced`, `weak`, `wrong`, `unimplemented`).
Nothing noticed, because nothing looks.

**D3. Permanently-fresh verdicts.** Three of the 52 entries in
`rfc/audit/rfc7606.json` carry an empty `tests` map: `RFC7606-5.1-1`,
`RFC7606-5.4-1`, `RFC7606-8-1`. Their freshness test reduces to `{} == {}`, so
they can never go stale. ~~They are claims about CODE (the gap is real, the
divergence is deliberate) fingerprinted against TESTS that do not exist, which
makes them structurally unfalsifiable.~~ **Corrected 2026-07-29: that describes
only TWO of them. `RFC7606-5.1-1` and `RFC7606-5.4-1` are `unimplemented`
`{gap}` claims about code, as stated. `RFC7606-8-1` is `verdict: enforced` on a
`{not-applicable}` requirement whose note says "there is genuinely no Ze code
path that could satisfy or violate it ... there is no test to demand and no
finding" — a claim about the honesty of an ANNOTATION, with neither code nor
tests to fingerprint. All three are structurally unfalsifiable. Only two are
falsifiable by a `code` map. See falsified claim F-4.** Under `ai/rules/rfc-compliance.md`
("Every earlier answer that pointed away from full compliance or full proof is
VOID", and specifically the row naming "A code comment or `rfc/audit/*.json`
verdict calling the deviation deliberate"), a permanently-fresh verdict blessing
a void annotation is the worst combination the system can produce.

**D4. Coverage is 4.5%.** Measured on the tree at 2026-07-29: 974 gated,
enrolled, both-polarity requirements exist. 44 carry a verdict. All 44 are
rfc7606, the only file in `rfc/audit/`. The remaining 930 span 129 RFCs.
→ **Re-derived 2026-07-29 (freshness review), unchanged: 974 / 44 / 930 / 129 / 4.52%.**
The full population is 166 enrolled RFCs and 2720 gated MUST-level requirements
(`make ze-rfc-check` prints both, and is GREEN on the tree with child 2 staged). The
docstring of `check_audit_freshness`
(~~`scripts/dev/rfc_requirements.py:1285-1287`~~ -> `:1935-1937`, 2026-07-29) makes a missing verdict
deliberately non-fatal: "the audit is sampled, the gate is total." That is the
right call, and it also means the semantic half of the gate covers one RFC in
166.

**The false-stale problem this must design around.** `tagged_unit_shas`
(~~`scripts/dev/rfc_requirements.py:1252-1270`~~ -> `:1902-1920`, 2026-07-29) fingerprints the WHOLE ENCLOSING
FILE and keys on `file:line`, so a verdict goes stale on any edit anywhere in a
tagged file and on any line shift. Measured on the one existing audit: of 15
commits touching `rfc/audit/rfc7606.json`, five were mechanical re-stamps where
nothing about what a test asserts had changed (a nine-line header prepended,
shifting every key by +9. A module-path rename. Sibling subtests added to the
same file. A helper signature change at unrelated call sites). Zero of those five
changed a verdict.
→ **Updated 2026-07-29 (freshness review).** The count is now **six of a
pending sixteen**. `git log --follow -- rfc/audit/rfc7606.json` still shows 15
COMMITTED commits (`acbef856e`, `ba1040bcc`, `d5e940a3d` are the three
`re-stamp` subjects. `34cd33b87` is the module rename. `01761bd3e` is the
ledger-regen re-stamp). Child 2's edit to the file is STAGED, not yet
committed, and its own `reaudit_history` entry declares itself a re-stamp:
"2026-07-29 re-stamp, RFC7606-5.1-2 and RFC7606-5.1-3 only. Neither verdict was
re-judged". It is a `shifted`-class event by this spec's own definition (a
comment-only `.ci` edit plus an ADDED evidence key), and it cost a human a
mechanical proof and a 200-word note. **Child 2 supplied a sixth data point for
this spec's premise while this spec was being written.** The pattern is filed as F18 in
`plan/learned/HOOK-FRICTION.md:716-753` (unchanged). Exactly one class is automated today:
`reseal_rfc_audits` (`scripts/dev/rename_module_path.py:318-409`, unchanged) handles a
whole-repo string substitution, proving per file via `rename_only_since_head`
(`:286-299`, unchanged) that the normalized content is identical under the rename before
re-sealing, and refusing any verdict whose `requirement_sha` moved. Scaled from
44 verdicts to 930 or more, the predictable failure mode is BLIND RE-STAMPING:
the badge stays green while nobody re-reads.

**Goal.** Give the audit record teeth and make its coverage measurable and
monotonic, so that a fleet performing the remaining 930 audits produces evidence
rather than decoration. Specifically: make the verdict value load-bearing without
punishing the auditor who reports a bad test. Validate the record fail-closed.
make an unfalsifiable verdict impossible to record. Remove the human step from
every re-stamp that carries no judgement. And publish the coverage figure derived
from the data rather than maintained by hand.

**Explicitly out of scope: performing the 930 audits.** That is follow-on fleet
work. This spec is machinery only. The one exception is bringing the single
existing audit file into the schema this spec defines, which is 4 records
(one illegal verdict value, three empty-`tests` verdicts), not an audit campaign.

**Settled 2026-07-29: re-judging `RFC7606-5.1-2` IS in scope, and it is the only
audit work this spec permits.** That record carries the value `implemented`,
which is outside the four values the skill defines
(`ai/skills/ze-rfc-audit.md:67-72`). It is therefore a BROKEN RECORD, not an
unfavourable finding: no legal verdict was ever recorded for that requirement,
so there is nothing to preserve and nothing to erase. Mapping it to `enforced`
mechanically would fabricate a verdict nobody reached, which is the exact
failure this machinery exists to prevent, so the value is re-derived by reading
`rfc/full/rfc7606.txt` against the requirement's nine tagged tests. Any outcome
other than `enforced` routes to escalation under `ai/rules/rfc-compliance.md`
(see Failure Routing), never to a convenient value. **This single record is not a
licence to begin the 930-verdict drain here.** The boundary is the schema, not
the judgement: a record that cannot be made legal without reading is repaired.
a record that is merely absent is left absent.

**A void answer this spec supersedes.** `plan/deferrals/rfc-gate-regression-ratchets.md`
carries a 2026-07-20 row ruling "skip it" on arming `check_audit_freshness` for
the other 164 enrolled RFCs. Under `ai/rules/rfc-compliance.md` that answer is
VOID as authority and may not be cited as one. Its two stated reasons are
nonetheless partly correct as engineering, and this spec keeps the correct half:
mass-generating audit files would record verdicts for audits nobody performed,
which is the declare-instead-of-prove failure the programme exists to remove. So
this spec does not generate verdicts. It makes the ones that get written
expensive to fake and impossible to erase quietly. The deferral row's disposition
is updated by this spec (see Files to Modify), not cited by it.

## Required Reading

### Architecture Docs
- [ ] `ai/skills/ze-rfc-audit.md` - the producer of every record this spec validates
  → Decision: the four-value verdict vocabulary at `:67-72` is the schema enum. `implemented` in `rfc/audit/rfc7606.json` is drift, not a fifth value.
  → Constraint: ~~`:94`~~ -> `:96-97` (corrected 2026-07-29) says `weak` and `wrong` are the valuable outputs, so the gate must not make recording them the expensive path. `:67-72` (the four-value table) and `:100-104` (VOID as authority) are both re-verified correct.
  → Constraint: `:100-104` says every annotation is VOID as authority, so an `unimplemented` verdict may not be treated as a settled decision by anything this spec builds.
- [ ] `ai/rules/rfc-compliance.md` - the owner directive that voids prior narrowing answers
  → Constraint: an `rfc/audit/*.json` verdict calling a deviation deliberate is void by default. The machinery can record it, must disclose it, and must never let it read as authority.
  → Constraint: choosing anything narrower than full compliance plus full proof is Thomas's call, so the gate escalates rather than self-approves.
- [ ] `ai/rules/fail-closed-guards.md` - the guard discipline this spec implements
  → Constraint: a guard must fail closed or say something. `load_audit` today does neither for anything except unparseable JSON.
  → Constraint: the zero-value trap is the concrete defect in D3 (`{} == {}` reads as a legitimate "fresh") and in a unit extractor that returns empty on failure.
  → Constraint: drive the guard from the entry point (`run_check`), not only the helper, so a check that stops being called fails a test.
- [ ] `ai/rules/derive-not-hardcode.md` - the coverage figure must be derived
  → Decision: audit coverage is rendered into `ai/RFC-REQUIREMENTS.md` from the audit files and the tags, never maintained by hand, and its staleness is caught by the existing `check_ledger_fresh`.
- [ ] `ai/rules/testing.md` - the published-versus-gated distinction
  → Decision: the shape prescribed for `ze-test-health-check` ("STRUCTURAL facts only ... volume counters are published, not gated") is the model for `weak` and `wrong` counts.
- [ ] `plan/learned/HOOK-FRICTION.md:716-753` - F18, the false-stale friction
  → Constraint: the proposed fix there is a message change. This spec does the structural version (a unit-level fingerprint) because at fleet scale a better message still leaves a human step in every no-judgement re-stamp.
- [ ] `ai/rules/spec-no-code.md` - spec style
  → Constraint: tables and prose only. Field shapes below are described as tables, never as JSON or Python.

### RFC Summaries (Scope: protocol)
Not applicable. Scope is tooling: this spec changes the gate that measures RFC
evidence, and changes no protocol behavior and no tagged test. No RFC summary is
read, edited, or relied on for a protocol claim here.

**Key insights:** (minimal context to resume after compaction)
- The verdict field is written by a skill and read by nothing. Making it read is the whole point, but the reader must be designed so honesty is never the expensive path.
- The false-stale problem and the blind-re-stamp problem are the same problem: every no-judgement re-stamp trains the reflex that re-stamping is what you do. Removing the no-judgement class removes the training data.
- Re-measured 2026-07-29 with the hook's own scope rule (`_go_func_scopes`, `.claude/hooks/pretool-writeedit.py:1653`): **all 2571 scanned Go tags sit inside exactly one top-level function span.** Zero sit outside every span, zero sit inside more than one, and across the ~~368~~ -> **366** tagged `*_test.go` files no two spans overlap. A function-level fingerprint is therefore total over the Go corpus today, with the file-level fallback reserved for ~~the 4 `.ci` tags~~ -> **the 8 non-Go tags (6 `.ci` + 2 `.py`)** and for the day a Go tag lands outside a span.
  → Re-measurement 2026-07-29 (freshness review, after child 2 staged): the Go half is UNCHANGED and re-confirmed by driving `_go_func_scopes` over all 366 tagged `*_test.go` files — 2571 `_GO_TAG_RE` tags, 2571 inside exactly one span, 0 outside, 0 in two, 0 overlapping spans. The NON-Go half moved: child 2 added 2 `.ci` tags and 2 `.py` interop tags, so the corpus is 371 tagged files / 2579 tags. `.py` is a carrier kind this spec never contemplated — see collision C-4.
- **The measurement trap, stated because it produced a wrong number once:** `_go_func_scopes` returns CHARACTER OFFSETS, not line numbers, so a tag's line number cannot be compared against a span. The second trap is which regex counts as a tag. The gate credits only `_GO_TAG_RE` (~~`scripts/dev/rfc_requirements.py:148`~~ -> `:156`, a `//` comment at line start): 2571 tags, all in-span. The hook's broader `_RFC_TAG` (`:1615`, unchanged) also matches the phrase in ordinary prose and finds ~~2574~~ -> **2574 over the Go corpus (2573 in-span + 1 out)**, of which one (`internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go:441`, a backticked mention inside a `test-relax:` comment sitting between two functions) is outside every span. That prose mention is NOT a scanned tag. It is what makes the hook widen to whole-file scope for that one file, which is the fallback working as designed. An earlier draft of this spec reported "2570 of 2571 with 1 outside" by mixing the two populations.
  → Re-measured 2026-07-29: `_GO_TAG_RE` is line-anchored WITHOUT `re.MULTILINE`, so it must be matched per line, never with `finditer` over whole file content. Doing the latter yields ZERO matches and looks like a clean corpus. A third trap for the same measurement.
- **(added 2026-07-29) A third sibling constant now exists:** `_CI_TAG_RE` (`:157`) and `_PY_TAG_RE` (`:158`), plus the `TAG_MARKER` pre-filter (`:166`). Any shared tagged-scope module must state its answer for all three readers, not two.
- Measured 2026-07-29: all 49 non-empty-`tests` verdicts in `rfc/audit/rfc7606.json` already name at least one identifier that occurs literally in one of their tagged files. Requiring that is free for the existing corpus.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)

> **Citations re-verified 2026-07-29 (freshness review, after children 1 and 2).**
> `scripts/dev/rfc_requirements.py` grew 1769 -> 4192 lines, so every line number
> below moved. Struck numbers are the pre-children values; the number after each
> arrow is where the SYMBOL sits today. The symbol name is the anchor, not the
> number. Full table and the re-measured figures: see
> "Freshness Review 2026-07-29" at the end of this spec.

- [ ] ~~`scripts/dev/rfc_requirements.py:1227`~~ -> `:1877-1886` - `verdict_is_fresh`: compares `requirement_sha` and the `tests` map only. The verdict value is never read. **Shape unchanged** (re-read 2026-07-29).
- [ ] ~~`scripts/dev/rfc_requirements.py:1239`~~ -> `:1889-1899` - `load_audit`: bare `json.load`, returns `data.get("requirements", {})`, no validation of any kind. A missing file returns an empty map (legal), a malformed file raises `ParseError` (fail-closed for syntax only). **Shape unchanged** (re-read 2026-07-29).
- [ ] ~~`scripts/dev/rfc_requirements.py:1252`~~ -> `:1902-1920` - `tagged_unit_shas`: hashes the whole enclosing file, caches per file, keys `file:line`. Documented as coarse on purpose. **Shape unchanged** (re-read 2026-07-29).
- [ ] ~~`scripts/dev/rfc_requirements.py:1273`~~ -> `:1923` - `check_audit_freshness`: iterates REQUIREMENTS, skips any with no verdict, fails on a stale one. Never iterates the audit file, so a verdict for an unknown rid is silently discarded. **Shape unchanged**. The "sampled/total" docstring is ~~`:1285-1287`~~ -> `:1935-1937`. It is called from `run_check` at `:4052`.
- [ ] ~~`scripts/dev/rfc_requirements.py:955`~~ -> `:1547` - `check_coverage_ratchet`: the monotonic-evidence pattern this spec copies, including its baseline scoping to RFCs enrolled at HEAD.
- [ ] ~~`scripts/dev/rfc_requirements.py:1007`~~ -> `:1657` - `check_retired_requirements`: the deletion-is-the-cheapest-escape lesson, and the id-attribution care (longest prefix first, silent stems excluded).
- [ ] ~~`scripts/dev/rfc_requirements.py:1163`~~ -> `:1813` - `check_status_agreement`: the existing disclosure cross-check against `docs/features/rfc-status.md`, with the fail-open it already fixed (a blank Remaining is not a disclosure).
- [ ] ~~`scripts/dev/rfc_requirements.py:1408`~~ -> `:2115` - `_render_rollup`: the actionable per-RFC view. The audit columns are added here. **Changed by child 2**: it now renders a `Nightly-only` column and states a doctrine ("Both and One polarity are the polarity view ... an overlapping subset marker ... never a total to sum with the others") that AC-24 must be reconciled with. Its header is PINNED by `scripts/dev/testing_health.py` (`RFC_TABLE_HEADER` at `:90`, `RFC_ROW` at `:95`) -- see collision C-6.
- [ ] ~~`scripts/dev/rfc_requirements.py:1465`~~ -> `:2251` - `render_ledger`: composes the ledger. `check_ledger_fresh` (~~`:1578`~~ -> `:3906`) makes it monotonically regenerated. **Changed by child 1**: it now also calls `render_extraction_table`, so the ledger ALREADY depends on a hand-authored evidence directory (`rfc/extraction/`) -- see falsified claim F-3.
- [ ] ~~`scripts/dev/rfc_requirements.py:1629`~~ -> `:3957` - `run_check`: the entry point every new check must be wired into. `main` (~~`:1754`~~ -> `:4156`) dispatches ~~`--check`, `--check-fresh`, `--write`, `--selftest`~~ -> SIX modes today: `--selftest`, `--write`, `--check-fresh`, `--extraction-status`, `--extract-skeleton`, `--check` (children 1/2). See falsified claim F-1.
- [ ] ~~`scripts/dev/rfc_requirements.py:763`~~ -> `:1188-1222` - `_git_baseline_summary_stems`: the None-versus-empty polarity discipline every new baseline reader must copy.
- [ ] (added 2026-07-29) `scripts/dev/rfc_requirements.py:2786` `_str_field` and `:2797` `_reject_unknown_keys`, and `:2807` `parse_extraction_artifact` - child 1 already built the validating-parse helpers this spec's AC-2 needs (typed field, closed key set, `ParseError` -> clean exit 2). `_validated_stem` (`:3090`) is AC-4's stem check. `run_extract_skeleton` (`:3138`) is the atomic staged-write pattern that `--reseal` copies. REUSE these. Do not write a second copy.
- [ ] (added 2026-07-29) `scripts/dev/rfc_requirements.py:1599` `check_evidence_ratchet` and `:1415` `nonunit_evidence` - child 2's tier ratchet. Keyed on TAGS only, so a verdict change cannot fire it. The AC-24 interaction is at the REPORTING layer, not here (collision C-5).
- [ ] (added 2026-07-29) `scripts/dev/rfc_requirements.py:732` `CARRIERS` and `:876` `carrier_for` - child 2's carrier table. It introduced a THIRD tagged-file kind (`.py`), which AC-22 and A-3 do not cover (collision C-4).
- [ ] `scripts/dev/rename_module_path.py:286` - `rename_only_since_head`: proves a file differs from HEAD by nothing but the rename, under `rfc_requirements`'s own normalization.
- [ ] `scripts/dev/rename_module_path.py:318` - `reseal_rfc_audits`: the only automated re-stamp today. Refuses on a changed `requirement_sha` or on any file where more than the rename moved. Appends the previous `reaudit_note` to `reaudit_history`.
- [ ] `.claude/hooks/pretool-writeedit.py:1653` - `_go_func_scopes`: top-level func spans as CHARACTER OFFSETS (doc comment through closing brace, capped at the next func's doc comment), the two boundary bugs its docstring records, and the fact that the spans are not a partition of the file.
- [ ] `.claude/hooks/pretool-writeedit.py:1689` - `_enclosing_tagged_scope`: the proven definition of "the tagged unit", including the whole-file fallback when a tag sits outside every function span. Its docstring's "0 of 2515 Go tags" is a stale total (2571 today) but the polarity still holds. See Key insights.
- [ ] ~~`scripts/dev/rfc_requirements_test.py:34`~~ -> `:55` (`_patched`) and `:77` (`_run_capturing`) - the gate-level test harness that drives `run_check` end to end with substituted module constants. Re-verified 2026-07-29: `:34` is `R = _load()`. Builders `_req` `:815`, `_tag` `:828`. The file is now 5393 lines / 57 `Test*` classes / 364 tests. None of this spec's planned class names collide, and the cited shapes `TestStatusDisclosureFailsClosed` (`:2644`), `TestDegradedBaselineIsQuiet` (`:2429`) and `TestRealTree` (`:5377`) all still exist. Note an existing `TestAuditFreshness` at `:1084`, distinct from this spec's planned `TestAuditUnitFreshness`.
- [ ] `scripts/dev/rfc_requirements_gate_test.go:42` - `TestRFCRequirementsGate`, `TestRFCLedgerFresh`, `TestRFCRequirementsSelftest`, `TestRFCRequirementsFailsClosed`: the Go wrappers that put the Python gate into `go test`.
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_test.go:269` - `TestAdjRIBInRFC7606TreatAsWithdrawRemovesRoute`: the tagged unit whose verdict false-staled four times. Read to confirm the unit is self-contained (it calls `newTestManager` and `message.SynthesizeWithdraw`, and its assertions are inline), which is what makes a unit-level fingerprint meaningful here.

**Behavior to preserve:**
- A MISSING verdict remains legal and non-fatal. Coverage is sampled. The gate is total. Nothing in this spec turns "not yet audited" into a build failure, because that would force verdict generation, which is the one outcome the void deferral row correctly warned against.
- A stale verdict remains fatal, and the bias stays "a false stale costs a re-read, a false fresh ships an unenforced compliance claim".
- Every existing check in `run_check` keeps its current semantics and message. This spec adds checks and refines one freshness comparison. It does not relax any existing failure.
- `make ze-rfc-check` stays read-only. Nothing under `--check` writes to `rfc/audit/`.
- `make ze-rfc-index` also stays a non-writer of `rfc/audit/`. It renders the ledger and nothing else touches the evidence files, which the 2026-07-29 ruling makes a preserved property rather than an incidental one (A-7).
- ~~The four existing exit behaviors of `main`~~ → **SIX as of 2026-07-29** (`--selftest`, `--write`, `--check-fresh`, `--extraction-status`, `--extract-skeleton`, `--check`, `main` at `:4156`) and the wording of `check_audit_freshness`'s stale message for a genuine judgement change stay recognizable, so existing session knowledge and the F18 record still apply.
- `rfc/enrolled.txt`, the requirement id rules, and the tagged-test hook are untouched.

**Behavior to change:**
- `load_audit` gains a validating parse and a second consumer direction (iterating the audit file, not only the requirements).
- The freshness comparison gains a unit-level fingerprint and a third state.
- The verdict value becomes load-bearing, in the three distinct ways set out under Design.
- `ai/RFC-REQUIREMENTS.md` gains a derived audit-coverage section, which makes the ledger's byte content depend on `rfc/audit/*.json` ~~for the first time~~. → **Corrected 2026-07-29: NOT for the first time.** Child 1 already made the ledger depend on a hand-authored evidence directory: `render_ledger` (`:2251`) calls `render_extraction_table` (`:3647`), which reads `rfc/extraction/*.json`. This is precedent, not novelty — R-8's coupling concern is already lived with, and `make ze-rfc-extract` is already a dedicated evidence-writing target separate from check and index, which STRENGTHENS A-7 rather than weakening it.
- A new, dedicated `make ze-rfc-reseal` target performs the mechanical re-seal. It is the only writer of `rfc/audit/*.json` this spec adds, and it is deliberately NOT folded into `ze-rfc-index` (A-7).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
Three inputs converge in `run_check`, all already read there today except the
third's validation: `rfc/short/*.md` (requirement text and annotations, parsed to
`Requirement` records), the `RFC requirement:` tags scanned out of test sources
under `TEST_ROOTS` (parsed to `Tag` records, carrying file and line), and
`rfc/audit/<rfc>.json` (per-requirement verdicts written by the `/ze-rfc-audit`
skill). A fourth input is git HEAD, read through the existing
`_git_baseline_*` helpers, which supplies the "what did this look like before"
side of every ratchet. `docs/features/rfc-status.md` is read for the public
disclosure cross-check.

### Transformation Path
1. `_collect_for_check` parses every summary and scans the tree once, yielding enrolled set, requirements, parse errors, tags, and the per-stem parse map.
2. `load_audit` reads each enrolled RFC's audit file and, new here, validates it against the schema, raising `ParseError` on any structural defect so the gate fails closed with a clean exit rather than proceeding on a half-understood record.
3. The tagged units for each requirement are resolved: for each tag, the enclosing top-level function span (Go) or the whole file (`.ci`, or a Go tag outside every span), producing both a unit fingerprint and the existing file fingerprint.
4. Freshness is computed per verdict into one of four states (fresh, shifted, stale-unit, stale-requirement), replacing today's boolean.
5. The verdict value is consumed by three separate checks: disclosure agreement against the public ledger, the findings and verdict ratchets against HEAD, and the upgrade guard.
6. Errors accumulate into `run_check`'s `errs` list and are printed with the existing formatting. Exit 2 on any.
7. Independently, `render_ledger` folds the same audit data into a derived coverage section of `ai/RFC-REQUIREMENTS.md`, whose staleness is caught by the existing `check_ledger_fresh`.
8. Under `--reseal` only, reached from the dedicated `make ze-rfc-reseal` target and never from `--check` or `--write`, verdicts in the `shifted` state are re-stamped mechanically after proving unit identity, and the previous `reaudit_note` is preserved into `reaudit_history`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Skill (`/ze-rfc-audit`) → gate | A JSON file per RFC under `rfc/audit/`, hand-written by an agent. The schema is the contract and is validated on read | No |
| Gate → public compliance claim | `docs/features/rfc-status.md` rows, cross-checked by the existing `check_status_agreement` and by the new verdict-disclosure check | No |
| Gate → generated ledger | `ai/RFC-REQUIREMENTS.md`, written by `run_write`, guarded by `check_ledger_fresh` | No |
| Gate → edit-time hook | `.claude/hooks/pretool-writeedit.py` and the gate must agree on what "the tagged unit" is. A shared leaf module is the single definition | No |
| Gate → rename tool | `scripts/dev/rename_module_path.py` imports `rfc_requirements` as a library today and must keep exactly one copy of the fingerprint rule | No |
| Gate → git HEAD | `git show` / `git grep` / `git ls-tree` through the existing baseline helpers, with the None-versus-empty polarity discipline | No |

### Integration Points
- `run_check` (~~`scripts/dev/rfc_requirements.py:1629`~~ -> `:3957`, 2026-07-29) is where every new check is called. A check not called there is invisible to `make ze-rfc-check` and to `TestRFCRequirementsGate`.
- `render_ledger` and `_render_rollup` are where the coverage figure is derived. `check_ledger_fresh` is what makes the derivation mandatory. → **Added 2026-07-29: `_render_rollup`'s table header and row shape are PINNED by `scripts/dev/testing_health.py` (`RFC_TABLE_HEADER` `:90`, `RFC_ROW` `:95`), whose own comment says a column change "must fail loudly rather than silently yield zero". Any audit column added to the rollup must land with a `testing_health.py` update in the SAME change, or `make ze-test-health-check` goes red. That file is absent from Files to Modify.**
- `main` (~~`:1754`~~ -> `:4156`, 2026-07-29) gains the `--reseal` mode. ~~There are then THREE invocation sites: `Makefile:437` (`ze-rfc-check`, read-only) and `Makefile:442` (`ze-rfc-index`, ledger only) are unchanged in what they may write, and a new `ze-rfc-reseal` target is the sole caller of `--reseal` (A-7).~~ → **Corrected 2026-07-29: there are FIVE invocation sites today, SIX after this spec.** `Makefile:438` (`--selftest`) and `:439` (`--check`) under `ze-rfc-check`. `Makefile:443` (`--write`) under `ze-rfc-index`. `Makefile:454` (`--extract-skeleton`) under `ze-rfc-extract`. `Makefile:461` (`--extraction-status`) under `ze-rfc-extraction-status`. And `mk/inventory.mk:106` (`--check-fresh`). The A-7 claim each site preserves — that only `--reseal` writes under `rfc/audit/` — is unaffected and still correct. Only the arithmetic was wrong.
- `scripts/dev/rename_module_path.py` becomes a caller of the shared reseal rather than an owner of a second implementation.
- `.claude/hooks/pretool-writeedit.py` becomes a caller of the shared tagged-scope module, with `scripts/dev/hook-parity-check.py` proving its behavior is unchanged.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | Verify at implementation: every new check is reached from `run_check`, and each has a `_patched`-driven wiring test that fails if the call is removed |
| No unintended coupling (components stay isolated) | No | Verify at implementation: the shared tagged-scope module must not import the gate or the hook. It is a leaf both depend on |
| No duplicated functionality (extends existing, does not recreate) | No | Verify at implementation: `rename_only_since_head` keeps its rename-specific proof but the re-stamp loop is deleted in favor of the shared one. Exactly one definition of a tagged unit exists after this change |
| Zero-copy preserved where applicable (refs, not copies) | No | Not applicable: this is Python tooling with no wire path. The performance constraint that does apply is per-file hash caching, which `tagged_unit_shas` already does and the unit extractor must also do |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them. No per-feature field, switch case, or factory added to a core/shared package (`ai/rules/plugin-self-containment.md`) | No | Verify at implementation: no per-RFC special case may appear anywhere. The verdict vocabulary is one enum constant read by every consumer, and the audit files are discovered by enrolment, never enumerated |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Nothing in the repo reads `verdict["verdict"]`, so making it load-bearing breaks no existing consumer | ~~`grep -rn '\["verdict"\]\|\.get("verdict"' scripts/ mk/ Makefile` returns only `scripts/dev/rename_module_path.py:386`, a write of `verdict["tests"]`~~ **Re-run 2026-07-29: that grep returns ZERO hits** (over `scripts/ mk/ Makefile .claude/hooks/`, BRE and ERE both). `:386` is `verdict["tests"] = fresh`, which the pattern never matched. The CONCLUSION is unchanged and stronger: nothing reads the value anywhere. The rename tool's write still sits at `rename_module_path.py:386` | The change would alter an existing behavior silently | Re-run the grep at implementation (expect zero). Assert in the wiring test that the rename tool never mutates `verdict` | confirmed (re-measured 2026-07-29, the stated grep RESULT was wrong, the assumption holds) |
| A-2 | The only audit file is `rfc/audit/rfc7606.json`, holding 52 verdicts: 49 `enforced`, 2 `unimplemented`, 1 `implemented`, of which 3 have an empty `tests` map | Inventory run 2026-07-29 over `rfc/audit/` | The schema migration is larger than 4 records and the "machinery only" scope boundary is wrong | Re-run the inventory as the first implementation step | confirmed (re-inventoried 2026-07-29 after child 2's staged edit: still ONE file, still 52 entries, still 49/2/1, still the same 3 empty-`tests` rids `RFC7606-5.1-1` `RFC7606-5.4-1` `RFC7606-8-1`, and still zero dangling rids. Child 2's edit touched only `RFC7606-5.1-2` and `RFC7606-5.1-3` — 3 added `tests` keys, 2 notes appended, 1 `reaudit_history` entry — and changed no verdict value. **But see falsified claim F-4: one of those 3 empty-`tests` records is `enforced`, not a code claim**) |
| A-3 | A tagged Go test resolves to exactly one top-level function span, or to the documented whole-file fallback | Re-measured 2026-07-29 by driving `_go_func_scopes` over all ~~368~~ **366** tagged `*_test.go` files, comparing CHARACTER OFFSETS (not line numbers): all 2571 tags scanned by `_GO_TAG_RE` sit inside exactly one span, none outside, none in two, and no two spans overlap. ~~the 4 `.ci` tags are file-scoped~~ **the 8 non-Go tags are file-scoped: 6 `.ci` AND 2 `.py`** | The unit fingerprint would under-fingerprint some tests, which is a false-fresh, the worst failure available | Measured directly at design time (above). The standing guard is a corpus test that runs the extractor over every tag in the tree and asserts a resolved unit or an explicit file-scope marker for each, never an empty result | confirmed for the GO half (independently re-measured 2026-07-29: 2571/2571 in exactly one span, 0 out, 0 multi, 0 overlaps). **PARTLY BROKEN for the non-Go half**: child 2 added a `.py` carrier the spec never named, and a `check()` function is a natural unit span that whole-file scope throws away. Decide `.py` scope before Phase 4 (collision C-4) |
| A-4 | Every existing `enforced` verdict's note already names at least one identifier occurring in one of its tagged files, so the note-cites-a-symbol check costs the existing corpus nothing | Measured 2026-07-29: 0 of 49 non-empty-`tests` verdicts fail it | The check would go red on honest existing records and would have to be introduced as advisory first | Re-run the measurement at implementation, before the check is made blocking | confirmed (independently re-measured 2026-07-29 after child 2's note rewrites: 0 of 49 fail, using identifiers of 5+ characters against the concatenated tagged files) |
| A-5 | ~~The three empty-`tests` verdicts already name their producing code in prose, so requiring a machine-checkable `code` map is a transcription, not a re-judgement~~ **BROKEN 2026-07-29: true for TWO of the three, false for the third** | `RFC7606-5.1-1` cites `wireu/split.go:440-476`. `RFC7606-5.4-1` cites `FamilyRIB.insertOpaque` and `buildFwdBody` — **note that the Basis column never named the third, which is the tell.** `RFC7606-8-1` is `verdict: enforced` on a `{not-applicable}` requirement, and its note says the opposite of this assumption: "there is genuinely no Ze code path that could satisfy or violate it ... there is no test to demand and no finding" | Filling the `code` map would require re-auditing those requirements, which is out of scope and must then be escalated | Read the three notes and confirm each names a file or symbol that exists | **broken** (re-read 2026-07-29). Two are transcriptions as claimed. The third has no code to transcribe and no test to cite, so under this spec's own AC-5 and AC-6 it is illegal with no legal landing state defined. See falsified claim F-4. The disposition is Thomas's call, not the implementer's |
| A-6 | The coverage figures (974 both-polarity gated requirements, 44 with a verdict, 930 remaining across 129 RFCs) are reproducible from the tool's own parse | Reproduced exactly 2026-07-29 by driving `_collect_for_check` and `load_audit` | The published figure would disagree with the ledger and the spec's premise would be misstated | The ledger render itself, once it publishes the figure. A unit test pins the arithmetic on a fixture | confirmed (independently re-derived 2026-07-29 on the tree WITH child 2 staged: 166 enrolled, 2720 gated+enrolled, **974** both-polarity, **44** with a verdict, **930** remaining across **129** RFCs, **4.52%**. Cross-checked by annotation split: 52 verdicts = 44 unannotated + 5 `{single-polarity}` + 2 `{gap}` + 1 `{not-applicable}`. Every verdict is currently FRESH, so there is no pre-existing stale to migrate) |
| A-7 | ~~`make ze-rfc-index` is an acceptable writer of `rfc/audit/*.json`, by analogy with its existing ownership of the generated ledger~~ SUPERSEDED by the 2026-07-29 ruling: a DEDICATED `make ze-rfc-reseal` target is the only writer of `rfc/audit/*.json`, and neither `ze-rfc-check` nor `ze-rfc-index` writes there | Owner ruling 2026-07-29. `ze-rfc-index` is run routinely (it is required after any tag move, and its freshness variant `run_check_fresh` (`:1725`) is reached from `make ze-doc-test` through `mk/inventory.mk:106`), so letting it silently re-seal a hand-authored evidence file would AUTOMATE the blind re-stamp reflex this spec exists to remove. A dedicated target makes every write to an evidence file intentional and greppable, at a cost of one make target | Not applicable: the question is settled. The residual cost is the one new target and the habit of running it, which the `shifted` message names explicitly | The ruling itself. plus a test asserting that `--check` and `--write` write nothing under `rfc/audit/`, and that `--reseal` writes only `tests` and `reaudit_history` | confirmed (owner ruling 2026-07-29) |
| A-8 | The `/ze-rfc-audit` skill's four-value vocabulary is the intended enum, and `implemented` is drift rather than a fifth value someone meant | `ai/skills/ze-rfc-audit.md:67-72` defines four. One record uses a fifth. Settled by the 2026-07-29 ruling: `RFC7606-5.1-2` is a BROKEN RECORD, so re-judging it is in scope and is the sole audit work this spec permits (see Task) | The enum is wrong and the schema would reject valid records | The ruling settles the enum and the drift. It does NOT settle which legal value replaces `implemented`: that is a judgement made at implementation by reading `rfc/full/rfc7606.txt`, and anything other than `enforced` escalates per `ai/rules/rfc-compliance.md`. A blind rewrite of the value stays banned (see Known Limitations) | confirmed (owner ruling 2026-07-29, replacement value still to be judged) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | At fleet scale the dominant failure is a false `enforced` written without reading. No gate can prove a human read something | A batch of new verdicts whose notes are uniform in shape, name only the test function, and say nothing about what the assertion would do under non-compliance | Make the cheap fake expensive in the ways a machine CAN check (note must cite a symbol present in the tagged unit, `enforced` requires both polarities or a `{single-polarity}` annotation, upgrades require a changed unit or a stated reason) and make the corpus samplable (every note is in git, coverage is published per RFC) |
| R-2 | The unit extractor mis-resolves a span and under-fingerprints, shipping a false fresh | A unit sha equal to the file sha where a span was expected, or an empty extraction | Fail closed: an unresolvable span falls back to the FILE fingerprint and is recorded as file-scoped, never treated as "unchanged". An empty extraction is an error, never a hash input |
| R-3 | The note-cites-a-symbol check goes red when a tagged test is renamed, and people respond by pasting junk tokens into notes | Notes that consist of a bare identifier with no sentence | Accept the red: a renamed test IS a reason to re-read. Require only ONE matching token, and make the error name the tokens checked and the files searched so the fix is obvious |
| R-4 | Gating `wrong` on public disclosure creates a red the auditor cannot clear alone (it needs a `docs/features/rfc-status.md` edit) | An auditor reporting a finding and then reverting it to clear the build | The error names the exact row and the required change. And the finding ratchet means reverting the verdict is itself a failure, so the only exit is the docs edit |
| R-5 | The `code` map for `unimplemented` verdicts becomes a new false-stale class, since producer files churn far more than test files | Repeated re-seals of the same `unimplemented` verdict | Fingerprint the cited SYMBOL's span, not the producer file, using the same extractor. Fall back to the file only when the symbol cannot be resolved, and say so in the error |
| R-6 | A ratchet keyed on a percentage would fail the build for adding a tagged test (the denominator grows) | A red gate on a commit that only added coverage | Ratchet the SET of audited requirement ids, never the ratio. The percentage is published and never gated. This is written into the AC list as an explicit negative test |
| R-7 | Baseline reads from git degrade (shallow clone, detached state) and a ratchet accuses everything | A wall of ratchet violations on a fresh clone | Copy the polarity discipline of `_git_baseline_summary_stems` (`:763-794`): return None on "could not look", and treat None as "no opinion", never as "nothing was there" |
| R-8 | Making the ledger depend on `rfc/audit/*.json` couples two regen paths, so an audit edit without `make ze-rfc-index` reds the build for a docs reason. Since A-7 split the re-seal into its own target, clearing a `shifted` verdict is now TWO commands: `make ze-rfc-reseal` rewrites `tests`, which in turn stales the ledger | `check_ledger_fresh` failing on commits that only touched an audit file, or a developer running the re-seal and being met with a second, different red | Acceptable and consistent with today's behavior for tags. Each message names the ONE command that clears it (`shifted` says `make ze-rfc-reseal`, a stale ledger says `make ze-rfc-index`), so neither red is a guessing game. The two-command remedy is the deliberate price of keeping every write to an evidence file intentional |
| R-9 | The shared tagged-scope module changes hook behavior as a side effect | A `hook-parity-check.py` golden diff | The golden table is the gate: the hook's exit codes must be byte-identical after the extraction. Re-bless only if a case changed intentionally, which here it must not |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible and no daemon behavior. The failure modes are a build gate that is too loud (false stale, ratchet noise on a degraded git baseline) or too quiet (a false-fresh verdict from a mis-resolved unit span, which is the serious one: it would let an unenforced compliance claim keep its badge) |
| How is it reverted? | Single commit revert. The only persisted artifacts are `rfc/audit/*.json` field additions (`units`, `code`, `upgrade_reason`) and the ledger's new section. Both are additive and a revert leaves the old boolean freshness rule reading the fields it always read |
| Who else touches this path? | `scripts/dev/rename_module_path.py` (re-seal), `.claude/hooks/pretool-writeedit.py` (tagged scope), `ai/skills/ze-rfc-audit.md` (the record's author), and the sibling `rfcgate` specs in this set. A fleet performing audits will be writing these files concurrently, which is why the schema is validated on read rather than trusted |

## Wiring Test (MANDATORY -- NOT deferrable)

No `.ci` row: Scope is tooling and this spec changes no daemon Go, so the
user entry point is a make target, and the driving surfaces are
`scripts/dev/rfc_requirements_test.py` (gate-level, via `_patched` and
`_run_capturing`) and the existing Go wrappers that put the Python gate into
`go test`. N/A for `.ci`, per the same reasoning `validate-spec.sh` encodes for
tooling-only specs.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-rfc-check` | → | `run_check` calls the audit schema validation | `TestAuditSchemaWiring` (`scripts/dev/rfc_requirements_test.py`) |
| `make ze-rfc-check` | → | `run_check` calls the verdict-disclosure check | `TestAuditDisclosureWiring` |
| `make ze-rfc-check` | → | `run_check` calls the finding and verdict ratchets | `TestAuditRatchetWiring` |
| `make ze-rfc-check` | → | `run_check` calls the upgrade guard | `TestAuditUpgradeGuardWiring` |
| `make ze-rfc-check` | → | unit-level freshness replaces the file-level boolean | `TestAuditUnitFreshnessWiring` |
| `make ze-rfc-index` | → | `run_write` renders the derived audit-coverage section | `TestAuditLedgerSectionRendered` |
| `make ze-rfc-reseal` | → | `run_reseal` re-stamps only `shifted` verdicts | `TestResealOnlyTouchesShifted` |
| `make ze-rfc-index` | → | `run_write` writes nothing under `rfc/audit/` | `TestIndexNeverWritesAudit` |
| `go test ./scripts/dev` | → | the whole chain against the REAL tree | `TestRFCRequirementsGate` (`scripts/dev/rfc_requirements_gate_test.go`) |
| `go test ./scripts/dev` | → | the Python unit tests run at all | `TestPythonUnitTests` (`scripts/dev/python_tests_test.go`) |
| `python3 scripts/dev/rename_module_path.py` | → | `reseal_rfc_audits` delegates to the shared re-seal | `TestRenameResealDelegates` (`scripts/dev/rename_module_path_test.py`) |
| Write or Edit of a tagged test file | → | the hook uses the shared tagged-scope definition | `scripts/dev/hook-parity-check.py` golden (exit codes unchanged) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An audit file records a verdict value outside the four-value enum (the live `implemented` case) | `make ze-rfc-check` fails, naming the rid, the offending value, and the legal set |
| AC-2 | A verdict is missing `note`, `requirement_sha`, or `tests`, or carries a key outside the known set | The gate fails, naming the rid and the offending or missing field |
| AC-3 | An audit file records a verdict for a requirement id that does not exist in that RFC's summary | The gate fails, naming the dangling rid. A verdict may not describe a requirement that is not there |
| AC-4 | An audit file exists for a stem that is not enrolled, or has no summary | The gate fails, naming the file |
| AC-5 | A verdict of `enforced` carries an empty `tests` map | The gate fails. "Proven" with no cited test is a contradiction the schema rejects |
| AC-6 | A verdict of `enforced` sits on a requirement whose tags do not cover both polarities, and the requirement carries no `{single-polarity}` annotation | The gate fails, naming the missing polarity |
| AC-7 | A verdict of `unimplemented` carries an empty `code` map, or its requirement carries no `{gap}` or `{not-applicable}` annotation | The gate fails. A claim that the CODE does not comply must cite the producing code and must agree with the summary |
| AC-8 | A file or symbol cited in a verdict's `code` map is edited | That verdict goes stale. The empty-`tests` class is no longer permanently fresh |
| AC-9 | A verdict of `wrong` or `unimplemented` sits on a requirement whose `docs/features/rfc-status.md` row advertises clean "Supported" with no gap note | The gate fails with the same disclosure logic `{gap}` already gets, naming the row and what it must say |
| AC-10 | A freshly recorded, well-formed verdict of `weak` (or `wrong`, with its row disclosed) | `make ze-rfc-check` PASSES. Reporting a bad test never fails the build at recording time |
| AC-11 | A `weak` or `wrong` verdict present at HEAD is absent in the working tree | The gate fails. A finding cannot be deleted. It is resolved by a fix or by retiring the requirement |
| AC-12 | A `weak` or `wrong` verdict at HEAD becomes `enforced` while every unit fingerprint is unchanged and no `upgrade_reason` is recorded | The gate fails. A finding cannot become proof without something changing |
| AC-13 | A requirement carrying a fresh verdict at HEAD carries none in the working tree | The gate fails. Audit coverage is monotonic per requirement id |
| AC-14 | A sibling test in a tagged file is edited, leaving the tagged unit byte-identical (the F18 case) | The gate does NOT report a judgement change. It reports the `shifted` state and names `make ze-rfc-reseal` as the remedy, in those words, and no human re-read is asked for. The message must NOT name `make ze-rfc-index`, which does not clear this state (A-7) |
| AC-15 | A line inside the tagged unit is changed | The gate fails as stale, and the message names WHICH unit changed, distinguishing it from a requirement-text change and from a sibling edit |
| AC-16 | `make ze-rfc-reseal` runs over an audit file holding `shifted`, `stale`, and `fresh` verdicts | Only `shifted` verdicts are re-stamped. `verdict`, `note`, `units`, `code`, and `requirement_sha` are byte-identical afterward for every record. The previous `reaudit_note` is preserved into `reaudit_history`. Neither `make ze-rfc-check` nor `make ze-rfc-index` writes anything under `rfc/audit/`, so the re-seal target is the only path by which an evidence file changes without a human editing it |
| AC-17 | An `enforced` verdict whose note names no identifier occurring in any of its tagged units | The gate fails, listing the tokens checked and the files searched |
| AC-18 | `ai/RFC-REQUIREMENTS.md` is regenerated | It carries a derived audit-coverage figure per RFC and repo-wide, plus the named `weak` and `wrong` worklist. If you edit an audit file and do not regenerate, `check_ledger_fresh` fails |
| AC-19 | A new tagged requirement is added, lowering the audited PERCENTAGE while removing no verdict | The gate passes. The ratchet is on the set of audited ids, never on the ratio |
| AC-20 | A verdict recorded before this change, carrying no `units` field, is currently fresh at file level | The gate passes under the transitional rule and the backfill records its unit fingerprints without re-judging. A verdict that is NOT currently fresh is never backfilled |
| AC-21 | The whole tree after implementation | `rfc/audit/rfc7606.json` validates, `make ze-rfc-check` is green, and `make ze-verify` passes |
| AC-22 | The tagged-scope definition | Exactly one definition exists in the tree. The gate and `.claude/hooks/pretool-writeedit.py` both use it, and `scripts/dev/hook-parity-check.py` shows no golden change |
| AC-23 | The corpus of every `RFC requirement:` tag in the tree | The extractor resolves each to a unit span or an explicit file-scope marker. It never returns an empty result that would hash to a constant |
| AC-24 | A requirement carrying a fresh verdict of `weak`, `wrong` or `unimplemented` while both polarity tags are present | It is no longer REPORTED as proven: the derived audit-coverage section subtracts it from that RFC's proven count and names the verdict as the reason, so the ledger can never show one requirement as proven and weak at once. The gate still exits 0 (AC-10) -- the consequence is reporting, not a red, which is the counting half of "the verdict value is load-bearing" that AC-10, AC-11 and AC-18 leave to this row. Added 2026-07-29: `plan/spec-rfcgate-0-umbrella.md` gives this spec "make a `weak`, `wrong`, or `unimplemented` verdict stop counting the requirement as proven" in its Child Specs table and its AC-10, and no AC here carried it |

## 🧪 TDD Test Plan

### Unit Tests
All new Python tests extend `scripts/dev/rfc_requirements_test.py`, which is
auto-discovered by `TestPythonUnitTests` (`scripts/dev/python_tests_test.go`) and
needs no make target. Fixtures follow the file's existing idiom: `_req` / `_tag`
builders for records, `_patched` to substitute module constants (`AUDIT_DIR`,
`SUMMARY_DIR`, `STATUS_FILE`, `LEDGER_FILE`, and the `_git_baseline_*` readers),
and `_run_capturing` to drive `run_check` end to end so a check that stops being
called fails the test rather than passing silently.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAuditSchema.test_unknown_verdict_value_fails` | `scripts/dev/rfc_requirements_test.py` | AC-1, using the live `implemented` value as the fixture | |
| `TestAuditSchema.test_missing_required_field_fails` | same | AC-2, one case per required field | |
| `TestAuditSchema.test_unknown_key_in_verdict_fails` | same | AC-2, a typo'd field is not silently dropped | |
| `TestAuditSchema.test_verdict_for_unknown_rid_fails` | same | AC-3, the direction `check_audit_freshness` never walks | |
| `TestAuditSchema.test_audit_file_for_unenrolled_stem_fails` | same | AC-4 | |
| `TestAuditSchema.test_enforced_with_empty_tests_fails` | same | AC-5 | |
| `TestAuditSchema.test_enforced_needs_both_polarities` | same | AC-6, plus the `{single-polarity}` annotation exemption passing | |
| `TestAuditSchema.test_unimplemented_needs_code_and_gap_annotation` | same | AC-7, both halves as separate cases | |
| `TestAuditSchema.test_malformed_json_still_fails_closed` | same | preserved behavior: a syntax error remains a clean exit 2, not a traceback | |
| `TestAuditCodeFingerprint.test_editing_cited_producer_stales_verdict` | same | AC-8 | |
| `TestAuditCodeFingerprint.test_symbol_span_preferred_over_whole_file` | same | R-5, the producer-churn mitigation | |
| `TestAuditDisclosure.test_wrong_under_clean_supported_fails` | same | AC-9, mirroring `TestStatusDisclosureFailsClosed`'s existing shape | |
| `TestAuditDisclosure.test_wrong_with_disclosed_row_passes` | same | AC-9 negative half | |
| `TestAuditFindings.test_weak_verdict_does_not_fail_the_gate` | same | AC-10, the incentive decision made executable | |
| `TestAuditFindings.test_deleted_finding_fails` | same | AC-11 | |
| `TestAuditFindings.test_upgrade_without_unit_change_fails` | same | AC-12 | |
| `TestAuditFindings.test_upgrade_with_changed_unit_passes` | same | AC-12 negative half | |
| `TestAuditFindings.test_upgrade_with_recorded_reason_passes` | same | AC-12, the `upgrade_reason` escape, which is auditable rather than silent | |
| `TestAuditVerdictRatchet.test_removed_verdict_fails` | same | AC-13 | |
| `TestAuditVerdictRatchet.test_percentage_drop_from_new_tag_passes` | same | AC-19, the ratchet-on-a-ratio trap stated as a test | |
| `TestAuditVerdictRatchet.test_degraded_git_baseline_accuses_nobody` | same | R-7, copying `TestDegradedBaselineIsQuiet`'s existing shape | |
| `TestAuditUnitFreshness.test_sibling_edit_is_shifted_not_stale` | same | AC-14, the F18 case | |
| `TestAuditUnitFreshness.test_edit_inside_unit_is_stale` | same | AC-15 | |
| `TestAuditUnitFreshness.test_requirement_edit_is_distinguished` | same | AC-15, message discrimination | |
| `TestAuditUnitFreshness.test_missing_units_falls_back_to_file_rule` | same | AC-20 transitional rule | |
| `TestAuditUnitFreshness.test_unresolvable_span_falls_back_to_file` | same | R-2, fail-closed on extraction failure | |
| `TestAuditUnitFreshness.test_empty_extraction_is_an_error_not_a_hash` | same | R-2, the zero-value trap | |
| `TestAuditNote.test_note_must_cite_a_symbol_in_a_tagged_unit` | same | AC-17 | |
| `TestAuditNote.test_one_matching_token_is_enough` | same | AC-17, R-3, so a prose note is not punished | |
| `TestReseal.test_only_shifted_are_restamped` | same | AC-16 | |
| `TestReseal.test_reseal_never_mutates_judgement_fields` | same | AC-16, asserting byte identity of every other key | |
| `TestReseal.test_reseal_preserves_previous_note_into_history` | same | AC-16, the behavior `reseal_rfc_audits` already has | |
| `TestReseal.test_check_and_write_modes_never_touch_audit` | same | AC-16 and A-7: `--check` is read-only, and `--write` writes the ledger only, so `--reseal` is the sole automated writer of `rfc/audit/` | |
| `TestAuditLedger.test_coverage_section_is_derived` | same | AC-18, arithmetic pinned on a fixture | |
| `TestAuditLedger.test_weak_verdict_removes_proven_status` | same | AC-24: a `weak`, `wrong` or `unimplemented` verdict subtracts the requirement from the rendered proven count and names itself as the reason, while the gate stays green (AC-10). Its discriminating twin is `TestAuditFindings.test_weak_verdict_does_not_fail_the_gate`, which asserts the same fixture exits 0 | |
| `TestAuditLedger.test_findings_worklist_names_each_rid` | same | AC-18, a blur is not a worklist | |
| `TestAuditLedger.test_stale_ledger_after_audit_edit_fails` | same | AC-18, R-8 | |
| `TestTaggedScopeCorpus.test_every_tag_in_the_tree_resolves` | same | AC-23, A-3. Runs over the real tree like `TestRealTree` does today | |
| `TestRenameResealDelegates` | `scripts/dev/rename_module_path_test.py` | AC-22 and A-1: the rename tool calls the shared re-seal and never writes a judgement field | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `requirement_sha` / test sha / unit sha length | 16 hex characters, as produced by the existing sha helpers | 16 | 15 | 17 |
| `tests` map size for `enforced` | 1 or more entries | 1 | 0 (rejected, AC-5) | no upper bound |
| `code` map size for `unimplemented` | 1 or more entries | 1 | 0 (rejected, AC-7) | no upper bound |
| Polarities required for `enforced` | 2, unless `{single-polarity}` annotated | 2 | 1 without the annotation (rejected, AC-6) | not reachable: only two polarities exist |
| Note tokens matching a tagged unit | 1 or more | 1 | 0 (rejected, AC-17) | no upper bound |

### Functional Tests
Scope is tooling and this spec touches no daemon Go, so there is no `.ci`
surface. The equivalent end-to-end driving surfaces are named below and are the
substitute the validator's tooling-only branch expects.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `scripts/dev/rfc_requirements_test.py` gate-level classes | `scripts/dev/rfc_requirements_test.py` | A developer runs `make ze-rfc-check` over a tree holding a defective audit record and gets a specific, actionable failure | |
| `TestRFCRequirementsGate` | `scripts/dev/rfc_requirements_gate_test.go` | The real repository passes the armed gate, so the change is proven against 166 enrolled RFCs and not only against fixtures | |
| `TestRFCRequirementsFailsClosed` | `scripts/dev/rfc_requirements_gate_test.go` | An unreadable or malformed input yields exit 2 with a message, never a traceback or a green pass | |
| `scripts/dev/hook-parity-check.py` | `scripts/dev/hook-parity-check.py` | A developer editing a tagged test still gets exactly the block or pass they got before the scope definition moved | |

### Interop Tests (Scope: protocol)
Not applicable. This spec changes no wire behavior and no protocol code, so no
peer daemon is involved. The interop obligation in
`ai/rules/interop-and-goal-validation.md` is triggered by wire-visible change.
there is none here. The rule's vacuity guidance is nonetheless applied to this
spec's own tests: each new check has a negative case proving the gate goes red
when the defect is present and green when it is not, which is the same
discrimination requirement one layer up.

## Files to Modify
- `scripts/dev/rfc_requirements.py` - validating `load_audit`. The new checks (schema, disclosure, findings ratchet, verdict ratchet, upgrade guard, note citation). Unit-level freshness with four states. The `--reseal` mode. The derived audit-coverage section in `_render_rollup` and `render_ledger`
- `scripts/dev/rename_module_path.py` - `reseal_rfc_audits` delegates to the shared re-seal instead of owning a second loop. `rename_only_since_head` is kept as the rename-specific extra proof
- `.claude/hooks/pretool-writeedit.py` - `_enclosing_tagged_scope` sources its span logic from the shared module rather than a private copy
- `Makefile` - a NEW `ze-rfc-reseal` target calls `--reseal`. `ze-rfc-check` and `ze-rfc-index` are both unchanged, the first staying read-only and the second staying a writer of the ledger only (A-7)
- `rfc/audit/rfc7606.json` - schema migration of 4 records: the `implemented` value on `RFC7606-5.1-2` corrected by targeted re-judgement, and a `code` map transcribed for the three empty-`tests` verdicts. plus the mechanical `units` backfill for verdicts that are currently fresh
- `ai/RFC-REQUIREMENTS.md` - regenerated. Gains the audit-coverage section
- `ai/skills/ze-rfc-audit.md` - documents the enum as binding, the `units` and `code` fields, the `upgrade_reason` field, the escalation obligation on an `unimplemented` verdict, and the fact that `weak` and `wrong` are recorded without failing the build
- `ai/rules/hook-mapping.md` - the new gate rows, so the enforcement map stays complete
- `ai/INDEX.md` - the new `make ze-rfc-reseal` target in the Dev Tools table (beside `ze-rfc-check` at `:212` and `ze-rfc-index` at `:213`) and in the RFC keyword-map row (~~`:372`~~ -> `:374`, 2026-07-29), per `ai/rules/discovery-updates.md`. → **Re-verified 2026-07-29: `:212`/`:213` still correct, but the Dev Tools RFC block is now FOUR rows — `ze-rfc-extract` at `:214` and `ze-rfc-extraction-status` at `:215` (child 1). A new row lands at `:216`. The keyword map also gained an extraction row at `:375`.**
- `docs/contributing/rfc-implementation-guide.md` - the contributor-facing ~~pair~~ of RFC targets (`:516`) ~~becomes three~~, and the audit paragraph (`:523`, the bullet opens at `:521`) states which red each one clears. → **Corrected 2026-07-29: line `:516` still names a PAIR, but the guide as a whole already names four targets (`ze-rfc-extract` at `:581`, `ze-rfc-extraction-status` at `:596`), so `:516` is already stale against child 1 and adding reseal makes five. Fix `:516` as part of this, or leave the guide contradicting itself.**
- `plan/deferrals/rfc-gate-regression-ratchets.md` - the 2026-07-20 row's disposition is updated to record that its answer is void as authority and that this spec supersedes the machinery half of it (row confirmed present 2026-07-29 at `:8`, note its own inline citation `rfc_requirements.py:934` for `check_audit_freshness` is stale -> `:1923`)

> **Freshness review 2026-07-29 — a file this list is missing.**
> `scripts/dev/testing_health.py` pins `_render_rollup`'s table header and row
> shape (`RFC_TABLE_HEADER` `:90`, `RFC_ROW` `:95`, a 9-group regex) and raises
> `CollectError` rather than yielding zero when they move. AC-18 and AC-24 both
> change that table. Whether `testing_health.py` joins Files to Modify is a plan
> decision for the owner; this review only records that the coupling exists and
> that `make ze-test-health-check` (inside `ze-regen-check-readonly`) is the gate
> that will catch it.

## Files to Create
- `scripts/dev/rfc_tagged_scope.py` - the single definition of "the tagged unit": top-level Go function spans including their doc comments, the whole file for `.ci` and for a tag outside every span, imported by both the gate and the edit-time hook
- `scripts/dev/rename_module_path_test.py` - unit tests for the rename tool's delegation, if the file does not already exist at implementation time
- `plan/deferrals/rfcgate-3-audit-teeth.md` - this spec's deferral shard, created only if something is genuinely deferred

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No config surface. This is a build-time gate |
| YANG validation constraints | No | As above |
| YANG custom validators | No | As above |
| CLI commands/flags | No | `rfc_requirements.py` is a developer script, not a `ze` subcommand. Its `--reseal` flag follows the existing `--check` / `--write` mode style |
| CLI grammar (keyword before value) | N-A | Not a `ze` command. `ai/rules/cli-grammar.md` governs the operator CLI |
| Editor autocomplete | No | No YANG leaf added |
| Functional test for new RPC/API | N-A | No RPC. The driving surfaces are named under Functional Tests |
| Pipe completeness | N-A | No `ze` command output |
| Env var registration | No | No new env var |
| Doctor check for runtime dependencies | No | No new runtime dependency. git is already required by the existing baseline readers |
| Prometheus counters/metrics | No | The counters this adds are published in a generated document, not exported at runtime |
| BGP family surface (new SAFI / capability / attribute) | No | No protocol surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Developer tooling only. `docs/features.md` describes the product |
| 2 | Config syntax changed? | No | None |
| 3 | CLI command added/changed? | No | No `ze` command |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | None |
| 6 | Has a user guide page? | No | The audience is contributors. See rows 10 and 15 |
| 7 | Wire format changed? | No | None |
| 8 | Plugin SDK/protocol changed? | No | None |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No requirement gains or loses proof here. The one record correction (`RFC7606-5.1-2`) restates an existing judgement in legal vocabulary and must not change what is claimed. If the re-judgement finds the claim wrong, that IS an RFC-status change and this row flips to Yes with a `docs/features/rfc-status.md` edit |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/` (or `docs/functional-tests.md` if that is where the RFC gate is described): the audit record is now validated, its coverage published, and the new `make ze-rfc-reseal` is the only writer of `rfc/audit/`. Also `docs/contributing/rfc-implementation-guide.md:516-523`, which already names `ze-rfc-check` and `ze-rfc-index` as the pair a contributor runs and must now name the third |
| 10b | New make target added? | Yes | `ai/rules/discovery-updates.md` makes a new make target a discovery obligation: add `make ze-rfc-reseal` to the `ai/INDEX.md` Dev Tools table beside `ze-rfc-check` (`:212`) and `ze-rfc-index` (`:213`) — and, re-verified 2026-07-29, also beside `ze-rfc-extract` (`:214`) and `ze-rfc-extraction-status` (`:215`), so the new row lands at `:216` — and to the RFC keyword-map row (~~`:372`~~ -> `:374`), stating that it re-stamps `shifted` verdicts and writes nothing else. A target nobody can find is a target nobody runs, and the `shifted` red then has no remedy in practice |
| 11 | Affects daemon comparison? | No | None |
| 12 | Internal architecture changed? | No | No daemon architecture change |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `ai/RFC-REQUIREMENTS.md` is a generated inventory and gains a section. `ai/rules/hook-mapping.md` gains the new gate rows |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` and `ai/` for anchors naming `scripts/dev/rfc_requirements.py` and `.claude/hooks/pretool-writeedit.py`. Update any claim about the file-level fingerprint, which becomes one of two |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/skills/ze-rfc-audit.md` shows the verdict record shape. It must show the new fields or it teaches the fleet to write records that fail the schema |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - put every new check on the entry point before it does anything
   - Tests: `TestAuditSchemaWiring`, `TestAuditDisclosureWiring`, `TestAuditRatchetWiring`, `TestAuditUpgradeGuardWiring`, `TestAuditUnitFreshnessWiring`
   - Files: `scripts/dev/rfc_requirements.py` (stub check functions called from `run_check`), `scripts/dev/rfc_requirements_test.py`
   - Verify: each wiring test drives `run_check` through `_patched` with an input the finished check must reject, and FAILS because the stub returns no error. Removing any call from `run_check` must keep it failing
2. **Phase: Validate the record** - the schema, fail closed, both directions
   - Tests: the `TestAuditSchema` class, `TestAuditCodeFingerprint`
   - Files: `scripts/dev/rfc_requirements.py`
   - Verify: AC-1 through AC-7 red then green. Then run against the real tree and confirm the only failures are the 4 known rfc7606 records, which is A-2 validated by execution
3. **Phase: Migrate the one existing audit file** - 4 records, no campaign
   - Tests: `TestRFCRequirementsGate` (real tree) returns to green
   - Files: `rfc/audit/rfc7606.json`
   - Verify: the three `code` maps are transcribed from the notes' own citations (A-5). `RFC7606-5.1-2` is re-judged against `rfc/full/rfc7606.txt` rather than mapped mechanically, and if that re-judgement is anything other than `enforced`, STOP and escalate per `ai/rules/rfc-compliance.md` rather than recording a convenient value. Settled 2026-07-29: this ONE re-judgement is in scope precisely because the record is broken rather than unfavourable, and it is the only audit performed in this spec. No second requirement is judged here, whatever the reading of `RFC7606-5.1-2` turns up
4. **Phase: The tagged unit, extracted once** - shared module, hook parity held
   - Tests: `TestTaggedScopeCorpus`, `scripts/dev/hook-parity-check.py`
   - Files: `scripts/dev/rfc_tagged_scope.py`, `.claude/hooks/pretool-writeedit.py`
   - Verify: the corpus test resolves every tag in the tree (A-3). The hook golden is unchanged. The extractor never returns an empty result for a real file
5. **Phase: Four-state freshness** - the false-stale fix
   - Tests: the `TestAuditUnitFreshness` class
   - Files: `scripts/dev/rfc_requirements.py`
   - Verify: AC-14 and AC-15 red then green. The transitional no-`units` path keeps today's behavior exactly (AC-20). Reconstruct the F18 commit's shape as a fixture and confirm it now reports `shifted`
6. **Phase: Mechanical re-seal** - remove the human step from the no-judgement class
   - Tests: the `TestReseal` class, `TestRenameResealDelegates`
   - Files: `scripts/dev/rfc_requirements.py`, `scripts/dev/rename_module_path.py`, `Makefile` (the new `ze-rfc-reseal` target), `ai/INDEX.md`
   - Verify: only `shifted` records change. Every judgement field is byte-identical. `--check` and `--write` still write nothing under `rfc/audit/`, so `ze-rfc-reseal` is the only automated writer (A-7). The rename tool has no second copy of the rule
7. **Phase: The verdict value becomes load-bearing** - disclosure, ratchets, upgrade guard, note citation
   - Tests: `TestAuditDisclosure`, `TestAuditFindings`, `TestAuditVerdictRatchet`, `TestAuditNote`
   - Files: `scripts/dev/rfc_requirements.py`
   - Verify: AC-9 through AC-13 and AC-17 red then green. AC-10 explicitly proves a `weak` verdict passes. AC-19 explicitly proves the ratio is not ratcheted
8. **Phase: Publish the figure** - derived coverage in the ledger
   - Tests: the `TestAuditLedger` class
   - Files: `scripts/dev/rfc_requirements.py`, `ai/RFC-REQUIREMENTS.md`
   - Verify: the rendered figure reproduces the measured 44 of 974 (A-6) before any new audit is recorded, and `check_ledger_fresh` fails when an audit file is edited without regeneration
9. **Phase: Teach the producer** - the skill and the rule map
   - Tests: none mechanical. `make ze-doc-test` and `make ze-verify`
   - Files: `ai/skills/ze-rfc-audit.md`, `ai/rules/hook-mapping.md`, `docs/architecture/testing/`, `plan/deferrals/rfc-gate-regression-ratchets.md`
   - Verify: a reader following the skill produces a record that passes the schema on the first try, which is the only test that matters for a fleet

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at `file:line` and a named test |
| Feature completeness | Every user story path is a make target that actually runs the new code, not a helper reachable only from a test |
| Correctness | The four freshness states are mutually exclusive and total. No input falls through to a fifth, implicitly-fresh outcome |
| Fail-closed | Every new lookup uses an explicit presence check. No empty map, empty string, or unresolvable span reads as a legitimate answer (`ai/rules/fail-closed-guards.md`) |
| Incentive shape | Trace the cost of the honest path: recording `weak` must be green, and no new check may make reporting a finding more expensive than recording `enforced` |
| Naming | Verdict values match `ai/skills/ze-rfc-audit.md` exactly. Field names are lower-kebab or the existing snake style of the file, consistently, and the skill shows the same names |
| Data flow | `--check` performs no write anywhere. `--reseal` writes only `tests` and `reaudit_history` |
| Ratchet safety | Every baseline reader distinguishes "could not look" from "nothing was there", per `_git_baseline_summary_stems` |
| Rule: `ai/rules/derive-not-hardcode.md` | No hand-maintained list of audited RFCs, verdict counts, or per-RFC coverage anywhere. All derived |
| Rule: `ai/rules/rfc-compliance.md` | Nothing in the machinery lets an `unimplemented` verdict read as an approved deviation. The skill text says to escalate |
| Registration over hardcoding | No per-RFC branch, no enumerated audit-file list. Discovery is by enrolment and by directory |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The verdict value is read | `grep -rn 'verdict\["verdict"\]\|\.get("verdict"' scripts/` shows reads in the gate, not only the rename tool's write |
| The schema is validated | `make ze-rfc-check` on a tree with a deliberately corrupted audit record exits 2 naming the field |
| The empty-`tests` class is falsifiable | Touch a file cited in a `code` map, then `make ze-rfc-check` goes red |
| The false-stale class no longer costs a re-read | Reconstruct the F18 edit shape, run `make ze-rfc-check`, confirm the message says `shifted` and names `make ze-rfc-reseal` |
| Every write to an evidence file is intentional | `grep -rn 'reseal' Makefile mk/` names exactly one target, and `make ze-rfc-index` on a tree with a `shifted` verdict leaves `rfc/audit/` byte-identical |
| The new target is discoverable | `grep -n 'ze-rfc-reseal' ai/INDEX.md docs/contributing/rfc-implementation-guide.md` returns hits in the Dev Tools table, the keyword map, and the contributor guide |
| Coverage is published | `grep -n 'audited' ai/RFC-REQUIREMENTS.md` shows the derived per-RFC and repo-wide figures |
| A non-`enforced` verdict costs proven status | Record a `weak` verdict over a both-polarity requirement, `make ze-rfc-index`, then confirm that requirement is absent from its RFC's proven count and carries the verdict as the reason, and that `make ze-rfc-check` still exits 0 |
| Nothing generates verdicts | `git diff --stat rfc/audit/` across the whole change shows one file and 4 record corrections plus the mechanical `units` backfill |
| The gate still runs | `make ze-verify` green |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `rfc/audit/*.json` is agent-authored input parsed by a build gate. Every field must be type-checked before use. A string where a map is expected must produce a clean error, never an exception trace or a silently skipped record |
| Resource exhaustion | The unit extractor reads every tagged file. It must cache per file exactly as `tagged_unit_shas` does today, and must not re-read a file per tag (366 tagged files, 2575 tags) |
| Path handling | Keys in `tests` and `code` are `file:line` strings from a JSON file and become filesystem reads. They must be treated as repo-relative and rejected if they escape the tree, since a verdict is not a trusted path source |
| Error leakage | A failure message names the rid, the field, and the file. It must not dump the whole record, which for these notes runs to thousands of characters |
| Guard cannot fail open | The one catastrophic outcome is a false fresh. Every unresolvable case must degrade toward MORE checking (file scope), never toward less |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Not applicable (Python). A syntax or import error routes to the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline (`ruff` runs on write). If architectural → DESIGN |
| The real-tree gate goes red on records this spec did not plan for | A-2 is broken: stop, re-inventory, and report before widening the migration |
| The re-judgement of `RFC7606-5.1-2` is not `enforced` | STOP and escalate per `ai/rules/rfc-compliance.md`. This is a compliance decision, not a bookkeeping one |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The false-stale problem and the blind-re-stamp problem look like a tradeoff (make the fingerprint coarser and you get noise, finer and you get misses) but they are the same defect seen twice. Every no-judgement re-stamp is a training example teaching that re-stamping is what one does when the gate goes red. Five of fifteen commits to the one audit file were such examples, and none changed a verdict. Removing the human from that class is worth more than any message improvement, because at fleet scale the reflex is what fails, not the reading.
- The verdict field being inert is not merely a missing feature. It inverts the skill's own incentive: the skill says `weak` and `wrong` are the valuable outputs, and the gate treats them as identical to `enforced`, so recording a finding costs the auditor effort and buys the project nothing. That is why the design must make findings PUBLISHED and PERMANENT before it makes anything about them fail: the first thing a finding needs is to survive.
- The one thing no gate can check is whether a human read the RFC. Everything here is a proxy. Saying so plainly is part of the design: the proxies chosen (a note that cites a real symbol, both polarities before "proven", an upgrade that requires something to have changed) are the ones a lazy writer fails and an honest one passes without noticing.
- `check_retired_requirements` exists because deleting the checklist line was the cheapest route from red to green. The same shape applies here one level up: once verdicts are gated, deleting the verdict becomes the cheapest route, so the verdict ratchet is not optional decoration, it is the thing that keeps the rest honest.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `weak` and `wrong` are PUBLISHED, not gated, at recording time | Fail the gate immediately on any non-`enforced` verdict | Failing immediately is safe today only because zero such verdicts exist, and that is exactly the problem: the first person it would bite is the honest auditor who records one. At fleet scale that makes `enforced` the cheapest path to green, which is the declare-instead-of-prove failure the programme exists to remove. Publishing costs the reporter nothing and makes the finding visible |
| Findings are nonetheless PERMANENT: deleting or silently upgrading one fails | Leave findings purely advisory | A finding that can be deleted is a finding that will be. The cost falls on erasure, not on reporting: honest recording is free, quiet removal is red. This is `check_coverage_ratchet`'s shape applied to judgement rather than to tags |
| A `wrong` or `unimplemented` verdict must be disclosed in `docs/features/rfc-status.md` | Treat disclosure as the annotation's job only | These verdicts mean the requirement is not proven, or not met. `check_status_agreement` already refuses to let a `{gap}` hide under a clean "Supported" row. A verdict saying the same thing must not be weaker than an annotation saying it. The red falls on the public claim, which is the right place, not on the auditor's honesty |
| Freshness gains a `shifted` state whose remedy is mechanical | Keep the boolean and improve the message (F18's own proposal). Or drop the file-level fingerprint entirely | Improving the message leaves the human step in place, and the human step is what trains blind re-stamping. Dropping the file hash loses the one real class the unit hash misses (a helper in the same file changing under the tagged test). Three states keep both signals and route each to the cheapest correct response |
| The tagged unit is defined once, in a shared leaf module | Copy `_go_func_scopes` into the gate. Or have the gate import the hook | `reseal_rfc_audits`'s own docstring names the failure: a second copy of the fingerprint rule that drifted would re-seal against a hash the gate does not compute. The gate must not depend on `.claude/`, and the hook must stay import-light, so a leaf both import is the only shape that keeps one definition |
| An empty-`tests` verdict must carry a `code` map | Time-based expiry of unfalsifiable verdicts. Or ban empty-`tests` verdicts outright | Time-based staleness fires on a quiet repo and is evidence-free, which is against the grain of every other check here. Banning them would make `unimplemented` unrecordable, losing a real category. The notes already cite their producing code in prose, so requiring the citation in a machine-checkable field is transcription, and it makes the verdict falsifiable exactly when the gap might have closed |
| The ratchet is on the SET of audited requirement ids, never on the percentage | Ratchet the coverage ratio upward | A ratio ratchet fails the build for adding a tagged test, punishing coverage work. The set ratchet has no such perverse case, and the ratio remains useful as a published figure |
| Migration of the existing audit file is 4 records, and `RFC7606-5.1-2` is re-judged rather than mapped (settled 2026-07-29) | Mechanically rewrite `implemented` to `enforced`. Or declare the re-judgement out of scope as "audit work" and leave the record illegal | The value is illegal, so this is a BROKEN RECORD, not an unfavourable finding: no legal verdict exists to preserve, which is why repairing it is schema work and not the start of the 930-verdict drain. Which legal value replaces it is still a judgement about whether nine tests enforce the requirement, so rewriting it blind is precisely the fabrication this machinery exists to prevent, and `ai/rules/rfc-compliance.md` makes the void-answer discipline explicit. Leaving it illegal was rejected too: the schema this spec writes would reject the only audit file in the tree, so the gate could not be armed at all |
| A DEDICATED `make ze-rfc-reseal` is the only writer of `rfc/audit/`. `make ze-rfc-check` stays read-only and `make ze-rfc-index` stays confined to the ledger | Auto-reseal inside the check. Or fold the re-seal into `ze-rfc-index`, which was this spec's earlier position and is superseded | A check that writes cannot be trusted to report, and a regen target that writes evidence is the same failure one step removed. `ze-rfc-index` is run ROUTINELY: it is required after any tag move, and its freshness variant `run_check_fresh` (`:1725`) is reached from `make ze-doc-test` through `mk/inventory.mk:106`, so developers meet it for reasons that have nothing to do with an audit. Folding the re-seal into it would silently re-stamp hand-authored evidence during unrelated work, automating the exact blind re-stamp reflex this spec exists to remove. A dedicated target makes every write to an evidence file intentional and greppable. Cost: one make target, one discovery entry, and one extra command in the `shifted` remedy (R-8). Owner ruling 2026-07-29, A-7 |

## Known Limitations

- **No gate can prove a human read the RFC.** Everything here raises the cost of a false `enforced` and lowers the cost of an honest finding. None of it is proof of reading. Sampling by a second reader remains the only real check, and this spec makes sampling possible (coverage is published per RFC) without making it automatic.
- **The unit fingerprint cannot follow a call.** An assertion moved into a helper outside the tagged function is not covered by the unit hash. This is the same documented limit the edit-time hook carries (`.claude/hooks/pretool-writeedit.py:1714`), and it is precisely why the file-level hash is retained as the `shifted` signal rather than deleted.
- **The 930 outstanding audits are not performed here**, and no verdict is generated for them. That is deliberate and is the half of the void 2026-07-20 deferral row that was correct on the engineering. The follow-on fleet work belongs to the `rfcgate` umbrella. The single exception, settled 2026-07-29, is `RFC7606-5.1-2`, whose recorded value is illegal and therefore unrepairable without reading. Repairing a broken record is schema work. Judging an absent one is the drain, and the drain does not start here.
- **`unimplemented` verdicts remain judgements about code that only a reader can make.** The `code` map makes them falsifiable when the code moves. It does not make them right. Under `ai/rules/rfc-compliance.md` every one of them is void as authority and is a question for Thomas, which the skill text must say and no gate can enforce.

## RFC Documentation (Scope: protocol)

Not applicable: this spec adds no protocol code and no RFC-enforcing branch, so
there is no `// RFC NNNN Section X.Y` comment to add. The RFC-facing obligation
it does carry is the inverse one: nothing in the machinery may cause a tagged
test to be edited, and nothing may cause a compliance claim in
`docs/features/rfc-status.md` to change without a reader deciding it should.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-24 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes (the pre-commit gate, `ai/rules/git-safety.md`)
- [ ] Feature code integrated (`scripts/dev/rfc_requirements.py` reached from `make ze-rfc-check`, `make ze-rfc-index`, and `make ze-rfc-reseal`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination
- [ ] No verdict was generated for an audit nobody performed

### Quality Gates
- [ ] `make ze-rfc-check` green on the real tree
- [ ] `make ze-doc-test` green (the ledger and the skill are consistent)
- [ ] `scripts/dev/hook-parity-check.py` shows no golden change
- [ ] `make ze-lint-changed` clean

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional driving-surface tests for end-to-end behavior (no `.ci`: Scope is tooling)
- [ ] Interop tests for protocol features (N-A: no wire-visible change)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-rfcgate-3-audit-teeth.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-rfcgate-3-audit-teeth.md` only (commit A preserves the spec in history)

## Freshness Review 2026-07-29 (appended, implementation NOT started)

Written against the tree with child 1 COMMITTED (`2b1f84827`) and child 2 STAGED
but not committed. `make ze-rfc-check` is GREEN on that tree (364 selftests, 2720
gated across 166 enrolled, 2579 tags resolved). The umbrella (`:990`) says child 3
"begins only after child 2 is committed", so this is preparation, not a start.

### Citation corrections

`scripts/dev/rfc_requirements.py`: 1769 -> 4192 lines. Every number moved. Every
symbol survived, with no signature or semantic change to any of the four audit
functions this spec is built on.

| Symbol | Spec cited | Today | Shape |
|--------|-----------|-------|-------|
| `verdict_is_fresh` | `:1227-1236` | `:1877-1886` | unchanged |
| `load_audit` | `:1239-1249` | `:1889-1899` | unchanged |
| `tagged_unit_shas` | `:1252-1270` | `:1902-1920` | unchanged |
| `check_audit_freshness` | `:1273` | `:1923` | unchanged. Called from `run_check:4052` |
| ...its "sampled/total" docstring | `:1285-1287` | `:1935-1937` | unchanged |
| `check_coverage_ratchet` | `:955` | `:1547` | unchanged |
| `check_retired_requirements` | `:1007` | `:1657` | unchanged |
| `check_status_agreement` | `:1163` | `:1813` | unchanged |
| `_render_rollup` | `:1408` | `:2115` | **CHANGED** (child 2: `Nightly-only` column + subset-marker doctrine) |
| `render_ledger` | `:1465` | `:2251` | **CHANGED** (child 1: also renders `render_extraction_table`) |
| `check_ledger_fresh` | `:1578` | `:3906` | unchanged |
| `run_check` | `:1629` | `:3957` | **CHANGED** (4 new checks wired: evidence ratchet, extraction sign-off, extraction ratchet, drain floor) |
| `main` | `:1754` | `:4156` | **CHANGED** (4 modes -> 6) |
| `run_check_fresh` | `:1725` | `:4127` | unchanged |
| `_git_baseline_summary_stems` | `:763-794` | `:1188-1222` | unchanged |
| `_GO_TAG_RE` | `:148` | `:156` | unchanged (siblings `_CI_TAG_RE:157`, `_PY_TAG_RE:158` are new) |
| `_patched` / `_run_capturing` (`rfc_requirements_test.py`) | `:34` | `:55` / `:77` | unchanged |
| `_go_func_scopes` (`pretool-writeedit.py`) | `:1653` | `:1653` | unchanged |
| `_enclosing_tagged_scope` (same) | `:1689` | `:1689` | unchanged |
| `_RFC_TAG` (same) | `:1615` | `:1615` | unchanged |
| KNOWN LIMIT docstring (same) | `:1714` | `:1714` | unchanged |
| `rename_only_since_head` | `:286-299` | `:286-299` | unchanged |
| `reseal_rfc_audits` | `:318-409` | `:318-409` | unchanged |
| `verdict["tests"]` write (rename tool) | `:386` | `:386` | unchanged |
| `TestRFCRequirementsGate` (`..._gate_test.go`) | `:42` | `:42` | unchanged |
| `TestAdjRIBInRFC7606TreatAsWithdrawRemovesRoute` | `:269` | `:269` | unchanged (self-containment re-confirmed) |
| F18 (`HOOK-FRICTION.md`) | `:716-753` | `:716-753` | unchanged |
| `ze-rfc-audit.md` four-value table | `:67-72` | `:67-72` | unchanged |
| `ze-rfc-audit.md` "valuable outputs" | `:94` | `:96-97` | **was already off by 2** |
| `ze-rfc-audit.md` VOID clause | `:100-104` | `:100-104` | unchanged |
| `Makefile` `ze-rfc-check` / `ze-rfc-index` | `:437` / `:442` | `:437` / `:442` | unchanged |
| `mk/inventory.mk` `--check-fresh` | `:106` | `:106` | unchanged |
| `ai/INDEX.md` dev-tools rows | `:212` / `:213` | `:212` / `:213` | unchanged. `:214`/`:215` are new |
| `ai/INDEX.md` RFC keyword row | `:372` | `:374` | **moved** |
| `rfc-implementation-guide.md` targets / audit para | `:516` / `:523` | `:516` / `:521-525` | unchanged |

### Figures re-derived (driving `_collect_for_check` + `load_audit` on the current tree)

| Figure | Spec | Now | Verdict |
|--------|------|-----|---------|
| Enrolled RFCs | 166 (implied) | 166 | same |
| Gated + enrolled requirements | -- | 2720 | -- |
| Gated + enrolled + both-polarity | 974 | 974 | same |
| ...carrying a verdict | 44 | 44 | same |
| ...remaining | 930 across 129 RFCs | 930 across 129 RFCs | same |
| Coverage ratio | 4.5% | 4.52% | same |
| Audit files | 1 (`rfc7606`) | 1 | same |
| Audit entries | 52 | 52 | same |
| ...`enforced` / `unimplemented` / `implemented` | 49 / 2 / 1 | 49 / 2 / 1 | same |
| ...empty `tests` | 3 | 3 (same rids) | same |
| Dangling rids (verdict, no requirement) | -- | 0 | -- |
| Currently-stale verdicts | -- | 0 | -- |
| Tagged files / tags | 368 go, 4 `.ci`, 2575 total | **366 go + 6 `.ci` + 2 `.py` = 371 files, 2579 tags** | **CHANGED** |
| Go tags in exactly one func span | 2571 / 2571 | 2571 / 2571 (0 out, 0 multi, 0 overlaps) | same |
| Hand re-stamps of `rfc/audit/rfc7606.json` | 5 of 15 commits | 5 of 15 committed, **+1 staged (child 2) = 6 of a pending 16** | **CHANGED** |

Child 2's edit to `rfc/audit/rfc7606.json`, in full: one `reaudit_history` entry.
`RFC7606-5.1-2` note appended + one `tests` key added
(`test/plugin/rfc7606-relay-one-field.ci:3`). `RFC7606-5.1-3` note rewritten + two
`tests` keys added (`.../47-rfc7606-relay-shape-frr/check.py:28`,
`test/plugin/rfc7606-relay-one-field.ci:7`). No verdict value changed.

### Claims that are now FALSE (an implementer would be misled)

| ID | Where | The false claim | The evidence |
|----|-------|-----------------|--------------|
| F-1 | Current Behavior. Behavior to preserve. Integration Points | `main` "dispatches `--check`, `--check-fresh`, `--write`, `--selftest`". "the four existing exit behaviors". "There are then THREE invocation sites" | `main` (`:4156`) dispatches SIX modes. `Makefile:438,439,443,454,461` plus `mk/inventory.mk:106` are FIVE invocation sites today, six with `ze-rfc-reseal`. The A-7 property each preserves is unaffected |
| F-2 | Key insights. A-3. Files to Create | "the file-level fallback reserved for the **4 `.ci` tags**". `rfc_tagged_scope.py` = "top-level Go function spans ... the whole file for `.ci` and for a tag outside every span" | There are 8 non-Go tags: 6 `.ci` and **2 `.py`**. `.py` is not mentioned anywhere in this spec. It falls to whole-file scope only by accident (`_go_func_scopes` finds no `func` spans), and an interop `check.py` has a `check()` function that IS a natural unit |
| F-3 | Behavior to change | adding the audit section "makes the ledger's byte content depend on `rfc/audit/*.json` **for the first time**" | Not first. `render_ledger:2251` already calls `render_extraction_table:3647`, which reads `rfc/extraction/*.json`, a hand-authored evidence directory (child 1) |
| F-4 | D3. A-5. Task ("4 records"). Implementation Step 3 | the three empty-`tests` verdicts "are claims about CODE" whose `code` map is "a transcription, not a re-judgement" | `RFC7606-8-1` is `verdict: "enforced"` with an empty `tests` map on a `{not-applicable}` requirement. Its note: "there is genuinely no Ze code path that could satisfy or violate it ... there is no test to demand and no finding". It has no code to transcribe. Under this spec's OWN **AC-5** (`enforced` + empty `tests` -> fail) and **AC-6** (`enforced` without both polarities and without `{single-polarity}` -> fail) it is doubly illegal, and **AC-7's `code`-map remedy applies only to `unimplemented`**. Measured: it is the ONLY record failing AC-5 or AC-6, and both `unimplemented` records pass AC-7's annotation half. The schema as specified has no legal state for an honest `{not-applicable}` verdict. **This is a decision for Thomas, not the implementer**: either the enum/schema gains a state, or the record is re-judged, and re-judging a second record contradicts "the ONE re-judgement ... is the only audit performed in this spec" |
| F-5 | D1. A-1 | the grep "returns a single hit, `scripts/dev/rename_module_path.py:386`" | The quoted pattern returns ZERO hits. `:386` is `verdict["tests"] = fresh`, which it never matched. Conclusion holds and is stronger |
| F-6 | Files to Modify. Doc checklist 10b | "the contributor-facing **pair** of RFC targets (`:516`) becomes **three**" | The guide already names four targets (`:516` pair, `:581` `ze-rfc-extract`, `:596` `ze-rfc-extraction-status`). Line `:516` is already stale against child 1. Reseal makes five |
| F-7 | `ai/skills/ze-rfc-audit.md:94`. `ai/INDEX.md:372`. A-3 "368 files" | three line/count citations | `:96-97`, `:374`, 366 files. F-7 items are cosmetic. Listed so the implementer does not chase them |

### Collisions with what actually landed

| ID | Collision | Detail |
|----|-----------|--------|
| C-1 | **Reuse, not conflict** — the schema helpers already exist | Child 1's `_str_field` (`:2786`), `_reject_unknown_keys` (`:2797`), `parse_extraction_artifact` (`:2807`), `_validated_stem` (`:3090`) implement exactly AC-2's typed-field / closed-key-set / clean-exit-2 contract, and `_validated_stem` is AC-4's stem check. `run_extract_skeleton` (`:3138`) is the atomic staged-write pattern that `--reseal` copies (`.staging-*` + `os.replace`, with an age-gated sweep). Writing a second copy would violate this spec's own "exactly one definition" discipline |
| C-2 | **Precedent, not conflict** — a dedicated evidence-writing target already exists | `make ze-rfc-extract` (`Makefile:453`) writes `rfc/extraction/<stem>.json` and is separate from `ze-rfc-check` and `ze-rfc-index`. A-7's ruling now has a sibling in the tree rather than being the first of its kind |
| C-3 | Nothing this spec plans exists already under a different name | Verified by symbol sweep: no `check_audit_schema`, no verdict/finding ratchet, no note-citation check, no `--reseal`, no `ze-rfc-reseal`, no `rfc_tagged_scope.py`. `make ze-rfc-reseal` and `--reseal` return zero hits across `Makefile mk/ scripts/ ai/ docs/`. **AC-14/AC-16's premise is intact.** `scripts/dev/rename_module_path_test.py` DOES already exist, which the spec's Files to Create already hedged for |
| C-4 | **AC-22 / A-3: child 2 created a third carrier kind and a second gap** | (a) `.py` tags now exist (2), so "one definition of tagged scope" must answer for Go, `.ci` AND `.py`. (b) Worse: `c_test_weakening` (`.claude/hooks/pretool-writeedit.py:1806-1810`) gates on `_test\.go$` or (`.ci` under `/test/`) — **it never fires for `.py`**, so the two newly tagged `check.py` files carry RFC obligations that the edit-time guard does not protect at all. AC-22 unifies the SCOPE definition. It does not touch the hook's file predicate, and the two are now out of step. Nothing in either spec covers this |
| C-5 | **AC-24 vs child 2's rollup doctrine and `check_evidence_ratchet`** | `check_evidence_ratchet` (`:1599`) keys on TAGS only, so a verdict change cannot fire it — no gate-level conflict. The conflict is at the REPORTING layer. Child 2's `_render_rollup` states: "**Both** and **One polarity** are the polarity view: they answer which polarities exist, not which pipeline runs them ... **Nightly-only** is the tier view over the same rows — an overlapping subset marker ... **never a total to sum with the others**". AC-24 says a `weak`/`wrong`/`unimplemented` verdict "subtracts it from that RFC's proven count". Subtracting from `Both` contradicts that doctrine and silently changes `outstanding = one + missing`, which drives the "Enrollable now" list and the headline total. Adding a third overlapping marker column is consistent with the doctrine but is not what AC-24's word "subtracts" says. **Pick one before you implement.** Measured today the two features do not yet overlap: 0 of the 974 both-polarity requirements are nightly-only, and none of the 44 audited ones is |
| C-6 | **`_render_rollup`'s header is pinned by a second tool** | `scripts/dev/testing_health.py` `RFC_TABLE_HEADER` (`:90`) and `RFC_ROW` (`:95`, 9 capture groups) pin the table exactly, with the comment "a column change must fail loudly rather than silently yield zero". AC-18/AC-24 change that table. `make ze-test-health-check` (in `ze-regen-check-readonly`) will go red. `scripts/dev/testing_health.py` is absent from Files to Modify |
| C-7 | `load_audit` / `verdict_is_fresh` / `check_audit_freshness` shapes | **All three re-read 2026-07-29 and byte-unchanged in behaviour.** Every design assumption in D1-D3 and in the four-state freshness plan still holds |
| C-8 | Test-name collisions | None. 57 `Test*` classes exist in `rfc_requirements_test.py`. None matches this spec's ten planned class names. Note an adjacent existing `TestAuditFreshness` (`:1084`) beside the planned `TestAuditUnitFreshness` |

### What this review did NOT change

No acceptance criterion's meaning was altered, no AC was added or removed, and no
implementation was started. F-4 and C-4 to C-6 are recorded as findings. Their
disposition (schema state for an honest `{not-applicable}` verdict, `.py` scope,
the `Both`-column doctrine, and whether `testing_health.py` joins Files to Modify)
belongs to the owner. Re-run the derivations after child 2 is committed: nothing
here was measured against a committed tree for the staged half.

## Owner Ruling OR-1 (Thomas, 2026-07-29): F-4 is resolved by a legal state

**Decision: the verdict vocabulary gains a legal state for a requirement with
genuinely no reachable code path.** Taken in answer to F-4, which established that
`RFC7606-8-1` is `verdict: enforced` with an empty `tests` map on a
`{not-applicable}` requirement, and that this spec's AC-5, AC-6 and AC-7 between
them left it no legal landing state. Thomas chose this over re-judging the record
(which would contradict "the ONE re-judgement is the only audit performed in this
spec") and over re-deriving the annotation (which umbrella D4 forbids any child).

| Element | Requirement |
|---------|-------------|
| The state | A distinct verdict value meaning "no reachable code path can satisfy or violate this". It is NOT `enforced`: `enforced` means proven, and nothing is proven here |
| No `tests` map | Absent or empty is REQUIRED, because demanding a test for unreachable behaviour is the contradiction AC-5 exists to reject |
| A mandatory reason | The record states WHY no path exists, in prose, on the record. A state whose only content is its own name is the unfalsifiable entry AC-9 rejects |
| Agreement with the summary | The requirement's annotation in `rfc/short/<stem>.md` must independently say `{not-applicable}`. Two committed facts must agree, so the verdict cannot quietly diverge from the checklist |
| AC-5 stays strict | An `enforced` verdict with an empty `tests` map remains a hard failure. The new state is not a loophole in AC-5, it is the honest alternative to abusing it |

→ Constraint: this state must not become the cheap escape from AC-5. It is legal
only when the annotation agrees AND a reason is recorded: two committed facts, not
one word. `ai/rules/fail-closed-guards.md` -- a guard that cannot deny must at
least say something, and this state says precisely what it cannot check.
→ Constraint (OPEN, the implementer raises it, and does not decide it):
`ai/rules/rfc-compliance.md:53` voids `{not-applicable}` as AUTHORITY, so the
annotation this state agrees with is itself a classification the owner has voided.
That tension is real and this ruling does NOT resolve it. The ruling makes the
VERDICT honest about what the code does. It does not re-affirm the annotation.
Re-deriving that annotation is fleet-drain work under umbrella D4, and the honest
reading is that `RFC7606-8-1` will need looking at again when the drain reaches it.
→ Verified when this was recorded (2026-07-29): `verdict_is_fresh`
(`scripts/dev/rfc_requirements.py:1877-1886`) is still exact-equality over the
whole `tests` map, and `load_audit` (`:1889-1899`) still performs no schema
validation at all -- it returns `data.get("requirements", {})`. Both shapes this
ruling depends on are unchanged by children 1 and 2.

## Implementation Notes 2026-07-29 (appended, all nine phases implemented and green)

Append-only per `ai/rules/planning.md`. Nothing above is rewritten. This section records the
open decisions, the assumption outcomes, and two reds that are attributed elsewhere.

### Assumption outcomes

| ID | Outcome |
|----|---------|
| A-1 | **confirmed by execution.** `grep -rn 'verdict\["verdict"\]\|\.get("verdict"' scripts/` returned ZERO before, seven after. The rename tool now writes no fingerprint at all: it delegates, and `ResealDelegates.test_it_owns_no_second_copy_of_the_rule` asserts `verdict_is_fresh(`, `verdict["tests"] = ` and `tagged_unit_shas(` are all absent from it |
| A-2 | **confirmed.** Re-inventoried at implementation: one file, 52 entries, 49/2/1, the same three empty-`tests` rids, zero dangling. Migration was 4 records plus a 49-verdict `units` backfill, exactly the stated boundary |
| A-3 | **confirmed for Go, RESOLVED for non-Go.** 2571/2571 Go tags in exactly one span, 0 outside, 0 multi, 0 overlapping (re-measured through the shared leaf). `.py` is now file-scoped BY DECLARATION -- see C-4 |
| A-4 | **confirmed, and against a STRICTER population than the spec measured.** 0 of 49 notes fail the citation check when searched against the tagged UNIT text rather than the whole file, so the check ships at unit scope |
| A-5 | **broken as recorded, resolved by OR-1.** Two of the three were transcriptions as claimed. The third had nothing to transcribe. `RFC7606-8-1` is now `not-applicable` with a `no_code_path` reason |
| A-6 | **confirmed by execution.** 166 / 2720 / 974 / 44 / 930 across 129 / 4.52%, reproduced by the shipped code and published in the ledger |
| A-7 | **confirmed and tested.** `TestIndexNeverWritesAudit` snapshots `rfc/audit/` around `run_check` and `run_write` and asserts byte identity. Mutation-verified by making `run_write` call the writer |
| A-8 | **confirmed. The replacement value is `enforced`**, judged rather than mapped -- see below |

### The one re-judgement: `RFC7606-5.1-2` is `enforced`

Read `rfc/full/rfc7606.txt` §5.1 second bullet against all ten tagged tests. Four reasons, and
the last two are what make it a judgement rather than a rubber stamp:

1. The tests assert the RFC's own quantity (`NLRIBearingFieldCount` over every combination of the
   four fields, then at most one per emitted message), not a proxy for it.
2. Each of the three sender-side entry points -- `SplitCompliant`, `SplitWireUpdate`, and both
   arms of `buildFwdBody` -- has a fixture that guards BOTH that the input already FITS (so only
   shape can force the split) and that it starts non-compliant, so no assertion can pass for the
   wrong reason.
3. The negatives use genuinely compliant single-field inputs and assert POINTER IDENTITY of the
   passed-through bytes. An equal-but-copied payload fails them, so the pair discriminates
   instead of wearing two hats.
4. Would-it-fail is recorded from MUTATION rather than inferred: reverting either relay branch
   leaves one message where the tests require several, and `test/plugin/rfc7606-relay-one-field.ci`
   fails 4/4 with a message mismatch.

No escalation was needed, so Failure Routing's `RFC7606-5.1-2` row was not reached.

### C-4 resolved: `.py` is file-scoped by declaration, and the guard now sees it

(a) `scripts/dev/rfc_tagged_scope.py` owns `scope_reader(path)`, which returns Go-span scope for
`.go` and FILE scope for everything else. `.py` previously fell to whole-file scope only because
the span finder finds no `func` in Python -- the right answer for the wrong reason, and one that
would have changed silently the day anyone taught it about `def`. File scope is strictly MORE
sensitive than function scope, so declaring it can only over-trigger (a re-read), never
under-trigger (a false fresh). A Python span parser for two tags would have added a new
false-fresh surface for no measured benefit.

(b) The real hole was the hook's file predicate. `c_test_weakening` gated on `_test\.go$` or a
`/test/` `.ci`, so the two `check.py` files child 2 tagged carried RFC obligations the edit-time
guard did not protect AT ALL. It now also fires for any tag CARRIER that actually holds a tag,
with the carrier list read from the shared leaf and held against the scanner's own `CARRIERS` by
`TestTaggedScopeCoversEveryCarrier`. The generic weakening heuristic was deliberately NOT widened:
it counts Go and `.ci` shapes and would misread a Python scenario, and widening two rules when
only one has a demonstrated hole is how a guard earns a reputation for over-blocking.
Fixtures: `rfc-guard-covers-tagged-check-py`, `rfc-guard-untagged-py-unaffected`,
`rfc-guard-py-comment-edit-passes`. Hook golden unchanged (151/151).

### C-5 resolved: AC-24 subtracts in the AUDIT section, never from the polarity rollup

The **Both** column keeps its meaning and its number. AC-24's "proven count" is a new, separate
count in the new **Audit coverage** section. Three reasons:

1. Child 2's doctrine says **Both** and **One polarity** answer which polarities EXIST. A
   `weak`-verdict requirement genuinely has both, so removing it from that column would make the
   column false.
2. `scripts/dev/testing_health.py:353-359` asserts `gated - both == the annotation split` and
   RAISES rather than degrading. Subtracting from **Both** breaks that partition, and the only
   "fix" would be to teach an annotation split about audit verdicts -- semantically wrong.
3. AC-24's own wording locates the subtraction in "the derived audit-coverage section".

`outstanding = one + missing` is therefore untouched, and the "Enrollable now" figure is
unchanged. A per-requirement `**audit: <verdict>**` marker is added to the ledger's own
requirement rows (the same shape as child 2's `**nightly-only**`), so a reader scanning one
requirement sees the contradiction where the claim is made.

### C-6 resolved: `testing_health.py` does NOT join Files to Modify

`_render_rollup`'s header and row shape are byte-identical after this change
(`TestAuditTableCannotBeMistakenForTheRollup.test_the_rollup_header_it_pins_is_unchanged`), and
the new audit table is deliberately SIX cells so it cannot match the nine-cell `RFC_ROW`
(`..._test_no_audit_row_matches_the_health_tools_rollup_pattern`, mutation-verified by widening
the audit row to the matching shape). Proved by execution too: the health collector reads
`gated` as 2754, not 2754 + 974, so no audit row is being folded into its proof-density figure.

### Deviation: the verdict ratchet is presence-based, deliberately stricter than AC-13's wording

AC-13 says "a requirement carrying a FRESH verdict at HEAD". `check_audit_verdict_ratchet`
ratchets PRESENCE instead: any verdict that existed at HEAD cannot vanish, fresh or not. A
stale verdict must be RE-JUDGED, never deleted -- staleness is exactly the state in which
deletion is most tempting and least honest, so exempting it would aim the ratchet away from its
own case. AC-13's stated intent ("audit coverage is monotonic per requirement id") is met and
exceeded. Test: `TestAuditVerdictRatchet.test_a_stale_verdict_may_not_be_deleted_either`.

### Two reds attributed OUTSIDE this change (a concurrent child-4 session)

The umbrella's Sequencing Constraint says two children must never be in flight at once. During
this implementation a concurrent session held uncommitted edits to `rfc/short/rfc1035.md`,
`rfc3765.md`, `rfc4486.md` and `rfc5301.md` (child 4's D8 extraction), adding 34 gated MUSTs that
are neither proven nor annotated. Consequences, both measured:

| Red | Cause | Evidence it is not this change |
|-----|-------|-------------------------------|
| `make ze-test-health-check`, and `site_health_render_test.py` inside `go test ./scripts/dev/` | `collect_rfc` asserts `gated - both == annotation split`. The 34 new requirements land in the rollup's **No test** column, which that equation does not model, so it reads 2754 - 974 = 1780 against a split of 1746 | Driving `collect_rfc` with THIS code and HEAD's four summaries returns OK at 974 / 2720 -- the exact pre-change figures. The delta is 2754 - 2720 = 34, that is exactly those files |
| `ai/RFC-REQUIREMENTS.md` and `ai/CODE-TO-DOCS.md` regens absorb foreign rows | Both generators read the whole working tree, and both are REQUIRED (`check_ledger_fresh`, `make ze-doc-test`) | The ledger was already stale at session start for the same reason. `ai/CODE-TO-DOCS.md` was too, and one line of its diff (`rfc_tagged_scope.py`) is this change's |

Neither was "fixed" here. Teaching `testing_health.py`'s partition about un-annotated backlog
rows would edit a correct gate to accommodate another session's transient state, and resolving
those four stems is child 4's own scope (`plan/spec-rfcgate-4-ledger.md`, D8). The commit
sequencing is a decision for the supervising session.

### Mutation verification

37 mutations, 37 killed, 0 survivors. Every new check was disabled at its producing line and the
named test confirmed to flip red, then restored. Pass one reported six survivors: four were
mis-paired (the mutation and the test answered different questions), one was a redundant second
guard (both had to be disabled), and one was cross-case interference in the harness -- all six
were killed once re-paired and run one per process.

---

# Closure 2026-07-30 (appended, audit, goal validation, pre-commit verification)

Append-only per `ai/rules/planning.md`. Nothing above is rewritten.

**Every `file:line` below was read in this closure pass, not copied from the implementer's
report.** The module grew 1,769 -> 5,599 lines across children 1-3, so every line number in the
spec's original prose is stale and several the 2026-07-29 freshness review corrected have moved
again (`load_audit` `:1889` -> `:2115`, `check_audit_freshness` `:1923` -> `:2551`, `run_check`
`:3957` -> `:5325`, `main` `:4156` -> `:5561`). The symbol name is the anchor. The numbers below
are today's.

> ### Evidence pin, and a transient red (added 2026-07-30, after the second-pass review landed)
>
> **Every command output quoted in this closure section was captured at ONE state**, which this
> note names so no reader has to guess: the tree as it stood before the second-pass review's
> deletion of `verdict_is_fresh`, i.e. `scripts/dev/rfc_requirements.py` at 5,599 lines with that
> function present at `:1926`. At that state: `--check` exit 0, `--selftest` 520 tests OK, the 56
> + 100 targeted audit cases OK, `hook-parity-check.py` 151/151, `ResealDelegates` 4/4.
>
> **The deletion has since landed and the suite is RED as I write this.** Measured now:
> `python3 scripts/dev/rfc_requirements.py --selftest` -> `Ran 520 tests ... FAILED (errors=6)`,
> every error `AttributeError: module 'rfc_requirements' has no attribute 'verdict_is_fresh'`.
> The function is gone (`grep -c '^def verdict_is_fresh'` = 0) and its docstring corrections are
> in place, but ten call sites in `scripts/dev/rfc_requirements_test.py` still name it, in five
> methods across three classes: `TestFingerprint.test_verdict_fresh_when_nothing_changed`,
> `..._stale_when_requirement_sha_changes`, `..._stale_when_test_sha_changes`,
> `..._stale_when_test_disappears_or_appears`; `TestAuditFreshness.test_verdict_fresh_and_stale`;
> `TestNotApplicableTestsMapSpelling.test_the_file_level_rule_normalises_it_too`. The corrected
> docstring at `verdict_freshness` already points at a `TestTransitionalFileLevelRule` class that
> does not exist yet, so the re-pointing is mid-flight in the concurrent session that owns
> `scripts/`. **Not fixed here:** editing that file during another agent's edit would collide, and
> `scripts/` is outside this closure pass's scope. **Closure cannot proceed until it is green** --
> `commit_helper.py` gates on verify status, and this is a structural red, never a known-red.
>
> **The gate itself is unaffected, and that is evidence rather than luck.** Re-run after the
> deletion, `--check` still exits 0 and prints figures identical to the pinned state (2720 gated /
> 166 enrolled / 2579 tags; `audit: 49 proven, 3 audited-but-not-proven, of 52 verdict(s); 49 of
> 1344 auditable (3.65%)`). A function whose removal changes no gate output and breaks only its own
> tests is exactly what "dead code with a test suite over it" looks like from the outside.
>
> **Citation offsets.** The deletion shifted the module in two bands. Symbols from
> `_fingerprint_key` through `verdict_freshness` moved **-18**; `audit_freshness` and everything
> after it moved **-2**; `recorded_map` (`:1900`) and everything above it did not move. Re-pinned
> at the post-deletion state, for the symbols this section cites most:
>
> | Symbol | Cited below | Now |
> |--------|-------------|-----|
> | `recorded_map` | `:1900` | `:1900` (unchanged) |
> | `_fingerprint_key` | `:2011` | `:1993` |
> | `_sha_map` | `:2027` | `:2009` |
> | `_validate_verdict` | `:2055` | `:2037` |
> | `audit_stems` | `:2102` | `:2084` |
> | `load_audit` | `:2115` | `:2097` |
> | `load_audits` | `:2154` | `:2136` |
> | `check_audit_files` | `:2163` | `:2145` |
> | `check_audit_schema` | `:2182` | `:2164` |
> | `_verdict_claims` | `:2223` | `:2205` |
> | `unit_shas` | `:2343` | `:2325` |
> | `unit_scopes` | `:2386` | `:2368` |
> | `_unit_identity` | `:2419` | `:2401` |
> | `verdict_freshness` | `:2438` | `:2420`; **the AC-20 transitional branch is now labelled in the source at `:2471-2476`** |
> | `audit_freshness` | `:2498` | `:2496` |
> | `check_audit_freshness` | `:2551` | `:2549` |
> | `reseal_audits` | `:2622` | `:2620` |
> | `_write_audit` | `:2691` | `:2689` |
> | `run_reseal` | `:2736` | `:2734` |
> | `check_audit_disclosure` | `:2769` | `:2767` |
> | `check_audit_findings` | `:2870` | `:2868` |
> | `check_audit_verdict_ratchet` | `:2930` | `:2928` |
> | `check_audit_note` | `:2981` | `:2979` |
> | `polarity_covered` | `:3283` | `:3281` |
> | `audit_coverage` | `:3299` | `:3297` |
> | `_render_audit_coverage` | `:3379` | `:3377` |
> | `render_ledger` | `:3598` | `:3596` |
> | `check_ledger_fresh` | `:5274` | `:5272` |
> | `run_check` | `:5325` | `:5323` |
> | `run_write` | `:5516` | `:5514` |
> | `main` | `:5561` | `:5559` |
>
> The tables below are NOT rewritten to these numbers, per the append-only rule and because the
> re-pointing still in flight will move them again. Anchor on the symbol; use the two offsets.
>
> ### Red RESOLVED, same day, verified here
>
> The re-pointing landed while this section was being written. Re-measured immediately after:
> `--selftest` -> **`Ran 521 tests ... OK`, exit 0** (was 520 and 6 errors), and `--check` -> exit 0
> with the same four summary lines. `grep -c 'R.verdict_is_fresh' scripts/dev/rfc_requirements_test.py`
> is now **0**, and the class the corrected docstring promised exists:
> `TestTransitionalFileLevelRule` (`rfc_requirements_test.py:1058`), holding the four re-pointed
> cases (`test_verdict_fresh_when_nothing_changed`, `..._stale_when_requirement_sha_changes`,
> `..._stale_when_test_sha_changes`, `..._stale_when_test_disappears_or_appears`) plus a fifth that
> the old spelling could not express -- `test_a_recorded_units_map_leaves_the_transitional_branch`,
> which pins the boundary between the transitional branch and the unit rule. That is the +1 in
> 520 -> 521.
>
> So the ten assertions were re-pointed, not deleted (`ai/rules/no-test-deletion.md`), and they now
> drive the branch the gate actually executes. **The pinned evidence above therefore stands at the
> post-deletion state as well**, with two figures updated: 521 selftests rather than 520, and the
> line offsets in the table above. Every other output re-ran identical.

## Implementation Summary

### What Was Implemented

- **The record has a schema.** `load_audit` (`:2115-2151`) validates instead of `json.load`:
  closed top-level key set, `rfc` must match the filename, `reaudit_history` typed, then
  `_validate_verdict` (`:2055-2099`) per record -- closed verdict key set, the five-value enum
  (`:2070-2076`), typed `note` / `requirement_sha`, and `_sha_map` (`:2027-2052`) over each of
  `tests` / `units` / `code` with `_fingerprint_key` (`:2011-2024`) refusing any key that can
  read outside the tree. The cross-referential half is `check_audit_schema` (`:2182-2220`) plus
  `_verdict_claims` (`:2223-2306`). The file-level half is `check_audit_files` (`:2163-2179`)
  reached through `audit_stems` (`:2102-2112`), the direction nothing ever walked.
- **The verdict value is load-bearing in four places**, not one: disclosure
  (`check_audit_disclosure` `:2769-2819`), permanence (`check_audit_findings` `:2870-2927`),
  coverage monotonicity (`check_audit_verdict_ratchet` `:2930-2972`), and the note proxy
  (`check_audit_note` `:2981-3034`). All six audit checks plus `load_audits` are called from
  `run_check` (`:5425-5437`).
- **Freshness has four states.** `verdict_freshness` (`:2438-2495`) checks requirement text,
  then `code`, then `units`, then the file-level shift, so a real judgement change can never
  report as the cheap mechanical case. `_unit_identity` (`:2419-2435`) is the line-shift-invariant
  comparison. `audit_freshness` (`:2498-2548`) is the one derivation the gate, the ledger and
  `--reseal` all read.
- **The no-judgement re-stamp is mechanical.** `reseal_audits` (`:2622-2688`) is the single
  definition, `_write_audit` (`:2691-2733`) stages then `os.replace`s and re-validates the
  staged BYTES, `run_reseal` (`:2736-2763`) is `make ze-rfc-reseal` (`Makefile:458-459`), and
  `main` dispatches `--reseal` at `:5590-5591`.
- **One definition of the tagged unit.** `scripts/dev/rfc_tagged_scope.py` (new, `unit_at:143`,
  `scope_reader:51`, `go_func_scopes:107`, `is_tag_carrier:67`, `tag_scope:166`), imported by
  the gate (`unit_shas` `:2343-2383`) and by the edit-time hook
  (`.claude/hooks/pretool-writeedit.py:104-125`, used at `:1679`).
- **Coverage is derived and published.** `audit_coverage` (`:3299-3376`) with
  `polarity_covered` (`:3283-3296`) and `AuditCoverage` (`:3254-3280`), rendered by
  `_render_audit_coverage` (`:3379-3475`) into `ai/RFC-REQUIREMENTS.md` from `render_ledger`
  (`:3636`), and summarised on every clean run at `:5502-5512`.
- **The one existing audit file was migrated, not campaigned.** 52 records: `RFC7606-5.1-2`
  re-judged `enforced`, `RFC7606-8-1` moved to `not-applicable` + `no_code_path`, `code` maps
  transcribed for `RFC7606-5.1-1` and `RFC7606-5.4-1`, and a 49-verdict `units` backfill.
  Verified in this pass: 52 entries, 49 `enforced` / 2 `unimplemented` / 1 `not-applicable`,
  49 carrying `units`, 3 with an empty `tests` map, 0 dangling rids.

### Bugs Found/Fixed

Found by review during implementation, each with the test that now covers it. All five are
carried into `plan/learned/1297-rfcgate-3-audit-teeth.md` because none is visible from the code
alone.

| # | Defect | Fixed at | Covered by |
|---|--------|----------|-----------|
| 1 | An ABSENT `tests` key compared unequal to a computed `{}` (`None == {}` is False), so a `not-applicable` verdict authored the way `ai/skills/ze-rfc-audit.md` describes read STALE_UNIT permanently -- with an error text false in all three clauses and a `--reseal` that refused it | `recorded_map` (`:1900-1923`), called from `verdict_freshness` (`:2463-2465`) ~~and `verdict_is_fresh` (`:1939-1941`)~~ **corrected 2026-07-30: the second caller is the dead function being deleted (defect 6), so only the `verdict_freshness` call site is load-bearing** | `TestNotApplicableTestsMapSpelling` (4 cases), `TestNotApplicableWiring.test_run_check_accepts_the_documented_omitted_tests_map`, `..._test_reseal_neither_re_stamps_nor_refuses_it` |
| 6 | **`verdict_is_fresh` was DEAD CODE with eight tests over it, and two docstrings asserted the live path delegated to it.** `verdict_freshness` re-implements the pre-`units` transitional rule inline instead, and the two spellings had already diverged: only the live branch consults the `code` map | Found by the second-pass review 2026-07-30. The function is deleted, the two docstrings corrected, and its eight tests re-pointed at the live branch, by a parallel agent | The re-pointed cases in `TestFingerprint` / `TestAuditFreshness` / `TestNotApplicableTestsMapSpelling`, plus `TestAuditUnitFreshness.test_missing_units_falls_back_to_file_rule` which already drove the live branch |
| 2 | Audit coverage published a FLATTERING figure: 4.52% (44 of 974), because the denominator read polarity from tags alone and never `req.annotation`, so every `{single-polarity}` requirement fell outside `Auditable` while the schema was happy to judge it. Five recorded verdicts were counted in no column at all | `polarity_covered` (`:3283-3296`). The record view in `audit_coverage` (`:3353-3372`) no longer gated on `req.gated` | `TestAuditCoverageCountsEveryVerdict` (5 cases), `TestAuditCountersCoverTheWholeTree` |
| 3 | `run_check`'s own green line said every verdict it held was proven -- `0 audited-but-not-proven` -- over a record holding two `unimplemented` gaps and one `not-applicable`. It summed `r.findings` and discarded the worklist it had just computed | `:5502-5512` (`len(audit_worklist)`) | `TestAuditSummaryLineAgreesWithTheLedger` (2 cases) |
| 4 | `_unit_identity` as a SET would call deleting one of two same-unit tags "unchanged" -- a false FRESH, the one catastrophic outcome. The guard's docstring named the hazard. Nothing tested it | `_unit_identity` (`:2419-2435`, a multiset) | `TestUnitIdentityIsAMultiset` (3 cases) |
| 5 | `_write_audit` claimed to re-validate the written BYTES and validated the in-memory dict, which is not the same guarantee: `--check` reads the file | `:2727-2730` (re-read then validate) | `TestWriteAuditValidatesTheStagedBytes` (2 cases) |

### Documentation Updates

- `ai/skills/ze-rfc-audit.md` -- the five-value table (`:70-74`), the `not-applicable` row and
  its "not a shortcut past `enforced`" paragraph (`:80`), the field table naming `tests` /
  `units` / `no_code_path` / `upgrade_reason` (`:90-94`), and what is red versus free
  (`:106-109`). Held to the code by `TestVerdictVocabularyAgreesWithTheSkill` and
  `TestSkillDocumentsWhatTheSchemaAccepts`, so the producer's documentation cannot drift from
  the schema silently.
- `ai/rules/hook-mapping.md:119` -- the `rfc-tagged-test` row now records the derived carrier
  list and that `_enclosing_tagged_scope` delegates to `scripts/dev/rfc_tagged_scope.py`.
- `ai/INDEX.md` -- `make ze-rfc-reseal` in Dev Tools (`:220`), the `ze-rfc-index` row corrected
  to say it never writes `rfc/audit/` (`:217`), and the RFC keyword-map row (`:381`).
- `docs/contributing/rfc-implementation-guide.md:520,524,539` -- reseal named in the target
  list, in the source anchor, and in the "only command that writes `rfc/audit/`" paragraph.
- `docs/functional-tests.md` -- a new "What the tags cannot say: the audit record" section with
  a `<!-- source: scripts/dev/rfc_requirements.py -->` anchor naming
  `AUDIT_VERDICTS`/`check_audit_schema`/`check_audit_findings`.
- `ai/RFC-REQUIREMENTS.md:201-354` -- the generated `## Audit coverage` section and its
  `### Audited but not proven` worklist.
- `plan/deferrals/rfc-gate-regression-ratchets.md:8-11` -- the 2026-07-20 row recorded VOID as
  authority and superseded rather than cited.

### Deviations from Plan

1. **AC-13 is presence-based, deliberately stricter than its wording** (already recorded above
   under "Deviation"). `check_audit_verdict_ratchet` (`:2930-2972`) refuses the deletion of ANY
   verdict that existed at HEAD, fresh or not. Test:
   `TestAuditVerdictRatchet.test_a_stale_verdict_may_not_be_deleted_either`.
2. **The verdict enum has FIVE values, not four** -- owner ruling OR-1, recorded in full below.
3. **The rename tool's test class is `ResealDelegates`, not `TestRenameResealDelegates`.**
   `scripts/dev/rename_module_path_test.py` names its classes without the `Test` prefix
   throughout (`PlanEdits`, `Apply`, `GoTargets`), and unittest discovers by method name, so the
   spec's planned name would have been the only outlier in the file. Same four assertions,
   including `test_it_owns_no_second_copy_of_the_rule`.
4. **`.py` is file-scoped by declaration** (C-4), and the hook's carrier PREDICATE was widened
   while its generic weakening heuristic deliberately was not.
5. **`scripts/dev/testing_health.py` did not join Files to Modify** (C-6). Independently
   confirmed in this pass: `git diff HEAD -- scripts/dev/testing_health.py` contains zero
   occurrences of "audit", and `_render_rollup`'s pinned header is byte-identical
   (`TestAuditTableCannotBeMistakenForTheRollup.test_the_rollup_header_it_pins_is_unchanged`).
   That file IS modified in the working tree, by a concurrent session, for unrelated reasons.
6. **`verdict_is_fresh` is DELETED, not retained as "the pre-`units` spelling"** (added
   2026-07-30, second-pass review). The Behavior-to-preserve section and the function's own
   docstring both intended it to survive as the single spelling of the file-level rule, with
   `verdict_freshness` delegating to it. No delegation was ever written: the function had zero
   non-def call sites, `verdict_freshness` re-implements the rule inline at its `if not
   units_recorded` branch, and the two had already diverged on the `code` map. AC-20's behaviour
   is unchanged -- the live branch is what every real run has always executed -- so this removes
   dead code and eight tests' worth of false assurance rather than changing the gate. The eight
   tests are re-pointed at the live branch, not deleted (`ai/rules/no-test-deletion.md`).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-5: the three empty-`tests` verdicts were all "claims about CODE" whose `code` map would be a transcription | Two were. The third (`RFC7606-8-1`) is a claim about an ANNOTATION with neither code nor test to fingerprint, and the spec's own AC-5/AC-6/AC-7 left it no legal state | The 2026-07-29 freshness review, as falsified claim F-4 | Escalated rather than decided. Owner ruling OR-1 added a fifth verdict value. `plan/deferrals/` row not needed: the work landed in this spec |
| approach | The freshness comparison was first written key-by-key over the `units` map | A `file:line` key changes on any line shift, so every verdict in a shifted file compared unequal and reported STALE -- reproducing the exact false-stale class this spec exists to remove | Caught by `TestAuditUnitFreshness.test_line_shift_is_shifted_not_stale` during implementation | `_unit_identity` (`:2419-2435`) compares a multiset of `(file, unit-sha)` |
| approach | The documented `not-applicable` authoring path (omit `tests`) was never driven end to end, only the `{}` spelling | `None == {}` is False, so the documented spelling was permanently STALE with a remedy that was untrue in three clauses and a `--reseal` that refused it | Review, by following `ai/skills/ze-rfc-audit.md` as an author would | `recorded_map` (`:1900-1923`) normalises at comparison time as `_sha_map` already did at load time |
| escalation | Audit coverage was reported at the higher figure (4.52%) because its denominator excluded every annotated requirement the schema was willing to judge | The honest figure is 3.65% (49 of 1344). A flattering number came from a WRONG denominator, not from better coverage | Review, cross-checking the published figure against the record count | `polarity_covered` (`:3283-3296`). Both partitions documented on `AuditCoverage` (`:3254-3269`) |
| approach | AC-20's transitional rule was believed to be, and was DOCUMENTED as, a delegation: `verdict_freshness` "takes exactly the old file-level rule through `verdict_is_fresh`", kept "as one spelling so the pre-`units` behaviour cannot drift" | There was no delegation. `verdict_is_fresh` had ZERO non-def call sites. `verdict_freshness` re-implements the rule inline, and the two spellings had ALREADY drifted -- the live one consults the `code` map, the dead one does not | Second-pass review, 2026-07-30. Confirmed here by grep (four hits: one `def`, three prose) and by driving both: `('stale-unit', ['p.go:5'])` versus `True` on the same input | The function is deleted, both docstrings corrected, and its eight tests re-pointed at the live branch (parallel agent). **This closure pass had recorded the delegation as AC-20's evidence** -- the row is corrected above with a dated note rather than rewritten |
| escalation | An audit that reads a docstring's account of a call graph is reading a belief, not a wiring | Two docstrings asserted a delegation that no line of code performed, and eight passing tests made the dead function look exercised | The correction came from a review that grepped for CALL SITES instead of trusting the prose | Standing lesson, recorded in the learned summary's Gotchas: `ai/rules/no-fabrication.md` already bans citing a comment as design intent. This extends it to a comment's claim about which function calls which. Cite a call site, not a docstring |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| D1: the verdict value is inert -- make it load-bearing and still not punish an honest finding | Done | `check_audit_disclosure:2769`, `check_audit_findings:2870`, `check_audit_verdict_ratchet:2930`, `check_audit_note:2981`, `audit_coverage:3299` | Four gates plus the published count read `verdict["verdict"]`, which nothing read before. `weak` still exits 0 (`TestAuditFindings.test_weak_verdict_does_not_fail_the_gate`) |
| D2: no schema validation -- validate fail-closed, both directions | Done | `load_audit:2115` + `_validate_verdict:2055` (form), `check_audit_schema:2182` + `_verdict_claims:2223` (claims), `check_audit_files:2163` (the un-walked direction) | A malformed record is a clean exit 2, never a traceback (`TestAuditSchema.test_malformed_json_still_fails_closed`) |
| D3: permanently-fresh verdicts -- make an unfalsifiable verdict impossible | Done | `_verdict_claims:2262-2280` (`unimplemented` needs `code`), `:2282-2305` (`not-applicable` needs three agreeing facts), `verdict_freshness:2467-2475` (the `code` map stales) | The two `{gap}` records now cite producing spans. The third became `not-applicable` under OR-1 |
| D4: coverage is 4.5% and unmeasured -- publish it derived | Done | `audit_coverage:3299`, `_render_audit_coverage:3379`, `render_ledger:3636`, summary line `:5502` | Published as 49 of 1344 (3.65%) -- LOWER than the spec's 4.52% because the denominator was wrong, not because coverage fell |
| The false-stale problem: remove the human step from every no-judgement re-stamp | Done | `unit_shas:2343`, `verdict_freshness:2438`, `reseal_audits:2622`, `run_reseal:2736`, `Makefile:458` | `SHIFTED` names `make ze-rfc-reseal` and nothing else (`test_shifted_message_names_reseal_and_not_index`) |
| Explicitly out of scope: performing the outstanding audits | Held | -- | One re-judgement (`RFC7606-5.1-2`) and one owner-ruled state change (`RFC7606-8-1`). No verdict was generated for an audit nobody performed: `rfc/audit/` still holds exactly one file and 52 records |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `_validate_verdict:2070-2076`. `TestAuditSchema.test_unknown_verdict_value_fails`, `TestAuditSchemaWiring.test_run_check_fails_on_an_illegal_verdict_value` | Error names the rid, the offending value and `sorted(AUDIT_VERDICTS)`. The live `implemented` case was the fixture and is gone from the tree |
| AC-2 | Done | `_reject_unknown_keys` at `:2068`, `_str_field` at `:2077-2078`, `_sha_map` at `:2079-2080`. `test_missing_required_field_fails`, `test_unknown_key_in_verdict_fails`, `test_wrong_type_fails_closed_not_as_a_traceback`, `test_top_level_typo_is_not_discarded` | Reuses child 1's helpers (C-1) rather than a second copy |
| AC-3 | Done | `check_audit_schema:2210-2216`. `test_verdict_for_unknown_rid_fails`, `TestAuditSchemaWiring.test_run_check_fails_on_a_dangling_rid` | Also guards `req.rfc != rfc`, so a rid belonging to another RFC is caught |
| AC-4 | Done | `check_audit_files:2163-2179` + `audit_stems:2102`. `test_audit_file_for_unenrolled_stem_fails`, `test_audit_file_with_no_summary_fails`, `TestAuditFilesWiring` | Two distinct messages: no summary, versus not enrolled |
| AC-5 | Done | `_verdict_claims:2237-2248`. `test_enforced_with_empty_tests_fails`, `TestNotApplicableVerdict.test_ac5_stays_strict`, `TestAuditSchemaWiring.test_run_check_fails_on_enforced_with_no_tests` | The message points at the `not-applicable` alternative rather than leaving the author stuck |
| AC-6 | Done | `_verdict_claims:2249-2260`. `test_enforced_needs_both_polarities`, `test_single_polarity_annotation_exempts_it` | Names the MISSING polarity and the annotation that legalises one |
| AC-7 | Done | `_verdict_claims:2262-2280`. `test_unimplemented_needs_code_map`, `test_unimplemented_needs_gap_annotation`, `test_unimplemented_with_code_and_gap_passes` | Both halves are separate cases, as planned |
| AC-8 | Done | `verdict_freshness:2467-2475` + `audit_freshness:2531-2534`. `TestAuditCodeFingerprint` (4 cases) | Verified the live citations are real producers: `internal/component/bgp/wireu/split.go:456` is `buildUpdatePayload`, the function that orders MP_UNREACH before MP_REACH -- which is `RFC7606-5.1-1`'s `{gap}` |
| AC-9 | Done | `check_audit_disclosure:2769-2819`. `test_wrong_under_clean_supported_fails`, `test_wrong_with_disclosed_row_passes`, `test_unimplemented_under_clean_supported_fails`, `test_missing_row_fails`, `TestAuditDisclosureWiring` | Shares `row_discloses_a_gap` (`:1830`) with `check_status_agreement`, held by `test_one_definition_of_disclosure` |
| AC-10 | Done | `check_audit_disclosure:2798-2801` excludes `weak`. Nothing in `check_audit_findings` fires on recording | `test_weak_verdict_does_not_fail_the_gate`, `test_weak_is_not_gated_on_disclosure`, `test_downgrade_to_a_finding_is_free`, `TestAuditRatchetWiring.test_run_check_passes_on_a_freshly_recorded_weak_verdict` -- four independent proofs that honesty is free |
| AC-11 | Done | `check_audit_findings:2903-2910`. `test_deleted_finding_fails`, `TestAuditRatchetWiring.test_run_check_fails_when_a_finding_is_deleted` | |
| AC-12 | Done | `check_audit_findings:2911-2926`. `test_upgrade_without_unit_change_fails`, `test_upgrade_with_changed_unit_passes`, `test_upgrade_with_recorded_reason_passes`, `test_a_blank_reason_is_not_an_escape`, `TestAuditUpgradeGuardWiring` | A whitespace `upgrade_reason` is not an escape |
| AC-13 | Changed | `check_audit_verdict_ratchet:2930-2972`. `test_removed_verdict_fails`, `test_a_stale_verdict_may_not_be_deleted_either`, `TestAuditRatchetWiring.test_run_check_fails_when_a_verdict_is_deleted` | PRESENCE, not freshness: stricter than the AC's wording, meets its stated intent. Recorded under Deviations |
| AC-14 | Done | `check_audit_freshness:2582-2589`. `test_sibling_edit_is_shifted_not_stale`, `test_line_shift_is_shifted_not_stale`, `test_shifted_message_names_reseal_and_not_index`, `TestAuditUnitFreshnessWiring` | The message names `make ze-rfc-reseal` in those words and does NOT name `ze-rfc-index` (A-7) |
| AC-15 | Done | `check_audit_freshness:2590-2604` with `unit_scopes:2386`. `test_edit_inside_unit_is_stale`, `test_requirement_edit_is_distinguished`, `test_requirement_edit_wins_over_a_shift`, `test_stale_message_does_not_offer_the_mechanical_remedy` | Names each moved key and whether it resolved `func`- or `file`-scoped |
| AC-16 | Done | `reseal_audits:2622-2688`, `_write_audit:2691-2733`, `run_reseal:2736-2763`. `TestReseal` (10 cases), `TestResealOnlyTouchesShifted`, `TestIndexNeverWritesAudit` | `TestIndexNeverWritesAudit` snapshots `rfc/audit/` around `run_check` and `run_write` and asserts byte identity. `test_only_one_make_target_reseals` pins the single writer |
| AC-17 | Done | `check_audit_note:2981-3034` + `_NOTE_IDENT_RE:2978`. `test_note_must_cite_a_symbol_in_a_tagged_unit`, `test_one_matching_token_is_enough`, `test_a_symbol_only_in_a_sibling_function_does_not_count`, `test_short_words_do_not_satisfy_it`, `TestAuditNoteWiring` | Searched at UNIT scope, stricter than the population A-4 measured, and still 0 of 49 fail |
| AC-18 | Done | `_render_audit_coverage:3379-3475`, `render_ledger:3636`, `check_ledger_fresh:5274`. `TestAuditLedger` (6 cases) | Live evidence: `ai/RFC-REQUIREMENTS.md:201` `## Audit coverage`, `:203` the derived figure, `:354` the worklist heading |
| AC-19 | Done | `check_audit_verdict_ratchet:2937-2942` (set-of-ids, no ratio anywhere). `test_percentage_drop_from_new_tag_passes` | The percentage appears only in the ledger and the summary line, neither of which gates |
| AC-20 | Done | ~~`verdict_freshness:2477-2478` delegating to the unchanged `verdict_is_fresh:1926`~~ **Corrected 2026-07-30 (second-pass review): that delegation never existed.** The implementation is the LIVE transitional branch INSIDE `verdict_freshness` (`if not units_recorded: return (FRESH, []) if tests_recorded == test_shas else (STALE_UNIT, [])`), which re-implements the pre-`units` rule inline. Proof is `TestAuditUnitFreshness.test_missing_units_falls_back_to_file_rule` plus the eight cases being RE-POINTED at that branch by a parallel agent | 49 of 52 records carry `units` after the backfill. The 3 that legitimately cannot are the empty-`tests` ones. **Verified here 2026-07-30:** `grep -n 'verdict_is_fresh' scripts/dev/rfc_requirements.py` returns four hits, of which one is the `def` and three are docstring/comment prose -- ZERO non-def call sites. The two spellings had also already DIVERGED: driven directly, a verdict with `code` set, no `units` and unchanged `tests` whose producer moved yields `('stale-unit', ['p.go:5'])` from the live branch and `True` from the dead function, because only the live path consults the `code` map. `verdict_is_fresh` is being deleted, so no line number is cited for it |
| AC-21 | Partial | `rfc/audit/rfc7606.json` validates (52 records loaded through the validating parser in this pass) and `python3 scripts/dev/rfc_requirements.py --check` exits 0 first-hand -- **re-confirmed after the second-pass deletion landed, with byte-identical figures**. `--selftest` **520 tests OK before that deletion, 6 errors between it and the test re-pointing, and 521 tests OK after -- all three measured here (Evidence pin note above)** | **The `make ze-verify` clause was NOT re-run in this closure pass** (out of closure scope, and the caller's instruction). See "AC-21, stated plainly" below: the two reds the Implementation Notes attributed to a concurrent session no longer reproduce, but `ai/RFC-REQUIREMENTS.md` still carries that session's rows, which is a commit-sequencing decision for the supervising session |
| AC-22 | Done | `scripts/dev/rfc_tagged_scope.py` (`unit_at:143`). Hook import `.claude/hooks/pretool-writeedit.py:104-125`, used at `:1679`. `TestTaggedScopeCoversEveryCarrier.test_the_hook_uses_the_shared_definition`, `ResealDelegates.test_it_owns_no_second_copy_of_the_rule` | `python3 scripts/dev/hook-parity-check.py` re-run in this pass: **151/151 match, exit 0** -- no golden change |
| AC-23 | Done | `unit_shas:2370-2381` (an empty read and an empty extraction both raise). `TestTaggedScopeCorpus.test_every_tag_in_the_tree_resolves`, `test_every_go_tag_sits_in_exactly_one_span`, `TestAuditUnitFreshness.test_empty_extraction_is_an_error_not_a_hash` | Corpus test runs over the real tree, as `TestRealTree` does |
| AC-24 | Done | `audit_coverage:3353-3372` (subtraction in the audit section), `_render_audit_coverage:3419-3424`, summary line `:5502-5512`. `TestAuditLedger.test_weak_verdict_removes_proven_status`, `test_the_polarity_rollup_is_untouched`, `test_a_stale_verdict_is_not_published_as_proven`, `TestAuditSummaryLineAgreesWithTheLedger` | C-5 resolved as recorded: **Both** keeps its meaning and number. `proven` is a separate count. `proven` also requires FRESH, so a stale `enforced` is not published as proof |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestAuditSchema` (9 planned cases) | Done | `rfc_requirements_test.py:5599` | Shipped with 15, adding path-escape, wrong-type, top-level-typo, `rfc`-mismatch and missing-file cases |
| `TestAuditCodeFingerprint` (2) | Done | `:6207` | 4 cases. `test_a_deleted_producer_fails_closed` added |
| `TestAuditDisclosure` (2) | Done | `:6287` | 6 cases. `test_one_definition_of_disclosure` pins the shared helper |
| `TestAuditFindings` (5) | Done | `:6378` | 8 cases |
| `TestAuditVerdictRatchet` (3) | Done | `:6454` | 6 cases |
| `TestAuditUnitFreshness` (6) | Done | `:6051` | 13 cases |
| `TestAuditNote` (2) | Done | `:6330` | 5 cases |
| `TestReseal` (4) | Done | `:6532` | 10 cases. `test_no_staging_directory_is_left_behind` added |
| `TestAuditLedger` (4) | Done | `:6871` | 6 cases |
| `TestTaggedScopeCorpus` (1) | Done | `:7169` | 3 cases, including `test_the_live_audit_records_are_all_fresh` |
| `TestRenameResealDelegates` | Changed | `rename_module_path_test.py:430` as `ResealDelegates` | Renamed for file-local convention. 4 assertions, verified passing in this pass |
| Wiring: `TestAuditSchemaWiring`, `TestAuditDisclosureWiring`, `TestAuditRatchetWiring`, `TestAuditUpgradeGuardWiring`, `TestAuditUnitFreshnessWiring` | Done | `:6714`, `:6774`, `:6798`, `:6819`, `:6764` | Plus unplanned `TestAuditNoteWiring` (`:6789`), `TestAuditFilesWiring` (`:6840`), `TestNotApplicableWiring` (`:7487`) |
| Not in the plan, added by review | Added | `TestNotApplicableVerdict:5778`, `TestNotApplicableTestsMapSpelling:5877`, `TestUnitIdentityIsAMultiset:5980`, `TestAuditCoverageCountsEveryVerdict:6942`, `TestAuditSummaryLineAgreesWithTheLedger:7048`, `TestAuditCountersCoverTheWholeTree:7084`, `TestAuditTableCannotBeMistakenForTheRollup:7134`, `TestTaggedScopeCoversEveryCarrier:7230`, `TestVerdictVocabularyAgreesWithTheSkill:7268`, `TestFreshnessStatesAreTotal:7312`, `TestScopeReaderIsDeclared:7364`, `TestIndexNeverWritesAudit:7387`, `TestResealOnlyTouchesShifted:7447`, `TestWorklistMeaningNamesTheState:7534`, `TestWriteAuditValidatesTheStagedBytes:7589`, `TestSkillDocumentsWhatTheSchemaAccepts:7629` | 16 classes covering OR-1, the five review defects, and the four gate-invariant ratchets (vocabulary/state totality/carrier/scope-declaration) |
| Boundary tests: rows 2-5 (`tests` map size, `code` map size, polarities, note tokens) | Done | `tests`=0 for `enforced` -> `_verdict_claims:2240` (AC-5). `code`=0 for `unimplemented` -> `:2267` (AC-7). One polarity without annotation -> `:2251-2260` (AC-6). 0 matching note tokens -> `check_audit_note:3024` (AC-17) | Each "invalid below" has a named failing case. The `no_code_path` type boundary was added after review (`test_no_code_path_must_be_prose_not_any_json_value`) |
| Boundary tests: row 1 (sha length 16 / 15 / 17) | Changed | `_sha_map:2047-2050` enforces a NON-EMPTY string, not a 16-character length. `grep -n 'len(.*sha' scripts/dev/rfc_requirements.py` returns nothing. The only `[:16]` are the producers `requirement_sha:1893` and `test_sha:1897` | Verified at closure, and stated rather than glossed: a 15- or 17-character sha is ACCEPTED by the schema and then compares unequal to the computed value, so it resolves to STALE. That degrades toward MORE checking, which is the direction `ai/rules/fail-closed-guards.md` and this spec's own Security Review require, so it is a defensible design -- but it is not the length check the boundary row described, and no test pins 15 or 17. Not upgraded here: adding a validator at closure would be unreviewed new code |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `scripts/dev/rfc_requirements.py` | Done | Modified. 4,192 -> 5,599 lines |
| `scripts/dev/rename_module_path.py` | Done | Modified. `reseal_rfc_audits` delegates, `rename_only_since_head` kept as the extra per-file proof |
| `.claude/hooks/pretool-writeedit.py` | Done | Modified. `:104-125` loads the shared leaf, `:1759` derives the carrier predicate |
| `Makefile` | Done | Modified. `ze-rfc-reseal` at `:458-459`. `grep -rn 'reseal' Makefile mk/` returns exactly those two lines |
| `rfc/audit/rfc7606.json` | Done | Modified. +303 lines. 4 record corrections plus the 49-verdict `units` backfill, verified in this pass |
| `ai/RFC-REQUIREMENTS.md` | Done | Regenerated. `## Audit coverage` at `:201`. Also carries a concurrent session's rows (see AC-21) |
| `ai/skills/ze-rfc-audit.md` | Done | Modified. Held to the code by two gate tests |
| `ai/rules/hook-mapping.md` | Done | Modified. `:119` |
| `ai/INDEX.md` | Done | Modified. `:217`, `:220`, `:381` |
| `docs/contributing/rfc-implementation-guide.md` | Done | Modified. `:520`, `:524`, `:539` |
| `plan/deferrals/rfc-gate-regression-ratchets.md` | Done | Modified. `:8-11` |
| `scripts/dev/testing_health.py` | Changed | Deliberately NOT modified by this spec (C-6). Confirmed: zero "audit" occurrences in its diff vs HEAD |
| `scripts/dev/rfc_tagged_scope.py` | Done | Created (untracked). 11,106 bytes |
| `scripts/dev/rename_module_path_test.py` | Done | Pre-existed (C-3 hedged for this). `ResealDelegates` added at `:430` |
| `plan/deferrals/rfcgate-3-audit-teeth.md` | Not needed | Nothing is deferred out of this spec -- see "Deferrals Resolved" |
| `docs/functional-tests.md` | Added | Not in the plan's Files to Modify. The plan named `docs/architecture/testing/` "or `docs/functional-tests.md`" in Documentation checklist row 10, and this is where the RFC gate is described |

### Audit Summary
- **Total items:** 24 AC + 6 Task requirements + 16 files = 46
- **Done:** 42
- **Partial:** 1 (AC-21, the `make ze-verify` clause -- see below, needs the supervising session's call, not a code change)
- **Skipped:** 0
- **Changed:** 4 (AC-13 stricter, `TestRenameResealDelegates` -> `ResealDelegates`, `testing_health.py` deliberately untouched, `verdict_is_fresh` deleted rather than retained -- Deviation 6, added 2026-07-30)

**One gap found by this closure pass and NOT fixed here:** boundary-test row 1 (a 16-character
fingerprint sha, invalid at 15 and 17) has no validator and no test. `_sha_map` accepts any
non-empty string, so a wrong-length sha resolves to STALE rather than being refused -- fail-closed,
but not what the row specified. Raised for the independent review rather than patched at closure,
because a new validator plus its boundary cases is unreviewed implementation work and belongs on
the implementation model (`ai/rules/model-selection.md`). The Goal Gate "Boundary tests for all
numeric inputs" is therefore 4 of 5 rows.

### AC-21, stated plainly

AC-21 has three clauses. Two are verified first-hand in this pass:

| Clause | Evidence |
|--------|----------|
| `rfc/audit/rfc7606.json` validates | Loaded through `load_audit` in this pass: 52 records, all five-value-legal, 0 dangling rids, `_sha_map` clean on every `tests`/`units`/`code` map |
| `make ze-rfc-check` is green | `python3 scripts/dev/rfc_requirements.py --check` (what the target runs) exit 0, printing `2720 gated ... 166 enrolled; 2579 test tag(s) resolved` and `audit: 49 proven, 3 audited-but-not-proven, of 52 verdict(s); 49 of 1344 auditable requirement(s) audited (3.65%)` |
| `make ze-verify` passes | **NOT verified here.** A structural red opened and closed inside this pass: the deletion of `verdict_is_fresh` landed before its ten test call sites were re-pointed (6 `AttributeError`s), and the re-pointing then landed, returning `--selftest` to exit 0 at 521 tests -- both states measured and recorded in the Evidence pin note above. Separately, re-checked the two reds the Implementation Notes attributed to a concurrent session: `testing_health.collect_rfc()` now returns OK at `974 / 2720` on this tree (that session has since taught its partition about un-annotated backlog rows -- see its own comment at `scripts/dev/testing_health.py:345`), and `check_ledger_fresh` passes inside the exit-0 `--check`, so the ledger is currently fresh. What remains is that `ai/RFC-REQUIREMENTS.md`'s diff contains rows derived from four `rfc/short/*.md` files that session modified (`rfc1035`, `rfc3765`, `rfc4486`, `rfc5301`, still shown modified by `git status`). That is a commit-sequencing decision for the supervising session, not a defect in this change |

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Make the verdict value load-bearing **and still not punish the auditor who reports a bad test** | Functional (gate-level, through `run_check`) | Both directions proven by discriminating pairs: `TestAuditRatchetWiring.test_run_check_passes_on_a_freshly_recorded_weak_verdict` exits 0 on a fresh `weak`, while `..._test_run_check_fails_when_a_finding_is_deleted` exits 2 on its removal. `grep -c` over `scripts/`: the value is now read by six checks and was read by none |
| Validate the record **fail-closed** | Functional + negative | `TestAuditSchema.test_malformed_json_still_fails_closed` (clean exit 2, not a traceback), `test_wrong_type_fails_closed_not_as_a_traceback`, `test_fingerprint_key_may_not_escape_the_tree`, `TestAuditSchemaWiring.test_a_malformed_audit_file_exits_two_cleanly`. `TestRFCRequirementsFailsClosed` (Go wrapper) covers the same property at the make-target level |
| Make an **unfalsifiable verdict impossible to record** | Functional | The class is gone from the tree: of 3 empty-`tests` records, 2 now carry `code` maps naming real producing spans (`split.go:456` = `buildUpdatePayload`, verified by reading it) and 1 is `not-applicable` with a mandatory prose reason and an agreeing annotation. `TestAuditCodeFingerprint.test_editing_cited_producer_stales_verdict` proves the map bites. `TestNotApplicableVerdict.test_a_bare_state_is_refused` proves the new state is not a bare word |
| **Remove the human step from every re-stamp that carries no judgement** | Functional + reconstruction | `make ze-rfc-reseal` exists (`Makefile:458`) and is the ONLY writer: `TestIndexNeverWritesAudit.test_check_and_write_modes_never_touch_audit` snapshots `rfc/audit/` around both other modes and asserts byte identity. `test_only_one_make_target_reseals` greps the tree. The F18 shape is reconstructed as a fixture in `TestAuditUnitFreshness.test_sibling_edit_is_shifted_not_stale` and `test_line_shift_is_shifted_not_stale`, and `test_shifted_message_names_reseal_and_not_index` proves the remedy named is the one that works |
| **Publish the coverage figure derived from the data**, not maintained by hand | Generated artifact + freshness gate | `ai/RFC-REQUIREMENTS.md:201-354`, regenerated. `TestAuditLedger.test_stale_ledger_after_audit_edit_fails` proves an audit edit without regeneration reds `check_ledger_fresh`. No hand-maintained list exists: `TestAuditCountersCoverTheWholeTree` re-derives every published number from the live tree |
| The whole point: **a fleet performing the remaining audits produces evidence rather than decoration** | Reasoned bound, stated as such | Not provable by a test, and saying so is part of the design (`Known Limitations`: no gate can prove a human read the RFC). What IS proven: the cheap fakes are refused (AC-1..AC-7, AC-17), findings survive (AC-11..AC-13), the honest path is free (AC-10, four proofs), and the corpus is samplable per RFC (AC-18). Mutation verification -- 37 mutations, 37 killed -- is the evidence that each of those gates would actually fire |
| No verdict was generated for an audit nobody performed | Diff inspection | `rfc/audit/` holds one file and 52 records, unchanged in count. `git diff --stat` on it: +303 lines, all within the one file. Exactly one requirement was re-judged (`RFC7606-5.1-2`, four recorded reasons including a mutation) and one re-worded under an owner ruling |

## Deferrals Resolved

No deferral shard was created for this spec, and `plan/deferrals/rfcgate-3-audit-teeth.md` does
not exist. Everything that resembles a deferral is accounted for here:

| Row (candidate) | Final Status | Destination or evidence |
|-----------------|--------------|-------------------------|
| The outstanding audits (1,295 auditable requirements now carry no verdict) | Not a deferral | Never in scope: the Task says "Explicitly out of scope: performing the 930 audits", and the umbrella's D4 (`plan/spec-rfcgate-0-umbrella.md:74`) owns the fleet drain and forbids any child from opening it (`:87`). `ai/rules/deferral-tracking.md`: "Completing work that was never in scope (no record needed)" |
| OR-1's open tension: `ai/rules/rfc-compliance.md` voids `{not-applicable}` as authority (`:52` "Every earlier answer that pointed away from full compliance or full proof is VOID", with the `{gap}` / `{not-applicable}` row at `:58` and the classification gate at `:44`, OR-1's own text cites `:53`, which is the blank line under that statement -- corrected here 2026-07-30), so the annotation this verdict agrees with is itself voided | Carried forward on the record | Written onto the `RFC7606-8-1` record's own `note` in `rfc/audit/rfc7606.json`, which ends: "This ruling makes the VERDICT honest about what the code does. It does not re-affirm the annotation, and re-deriving that is fleet-drain work under the rfcgate umbrella's D4. RFC7606-8-1 needs looking at again when the drain reaches it." A record that must be re-read carries its own reason, where the next reader of that verdict will meet it |
| The two reds attributed to a concurrent child-4 session | Not this spec's | Re-checked in this pass: the `testing_health` red no longer reproduces (that session fixed its own partition) and the ledger is fresh under `--check`. The residual is commit sequencing (see AC-21) |
| C-4(b): the hook's generic weakening heuristic was not widened to `.py` | Decision, not a deferral | Recorded under "C-4 resolved": only the carrier PREDICATE had a demonstrated hole, and widening two rules when one is broken is how a guard earns a reputation for over-blocking. Three fixtures cover the fixed half |
| `plan/deferrals/rfc-gate-regression-ratchets.md`'s 2026-07-20 row | done (superseded) | Disposition updated in place at `:8-11`: VOID as authority, machinery half superseded by this spec. The row's own destination is unchanged |
| Boundary-test row 1: no 16-character length check on a fingerprint sha | **open -- not deferred** | In-scope work found by this closure pass (see Audit Summary). Deliberately NOT filed as a deferral: this spec stays OPEN until the Review Gate is clean anyway, so the item belongs in the implementation loop, not in a shard with a destination spec. Filing it would convert an open item into bookkeeping (`ai/rules/deferral-tracking.md`: "Filing work in a spec is NOT a close", `ai/rules/no-parking.md`) |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/plan/spec-rfcgate-3-audit-teeth-<session>.md` -- **not yet recorded** |
| `review_gate.py check` | **not run / BLOCKED.** Verified in this pass: `python3 scripts/dev/review_gate.py check --spec plan/spec-rfcgate-3-audit-teeth.md` reports "BLOCKED -- no independent-review artifact". An independent review is in flight in a concurrent session and owns this artifact |
| Reviewer lenses used | Owned by the review session. `commit_helper.py` runs `review_gate.py check` on the closure commit and will refuse it until a fresh, hash-pinned, CLEAN artifact exists (`ai/rules/critical-review.md`) |

### Findings fixed

The five defects in "Bugs Found/Fixed" above were all found by review DURING implementation and
are fixed with named tests. They are recorded there rather than duplicated here.

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The documented `not-applicable` authoring path produced a permanently red gate whose remedy text was untrue | `verdict_freshness` comparison of a raw `verdict.get("tests")` | `recorded_map:1900-1923` + `TestNotApplicableTestsMapSpelling` |
| 2 | ISSUE | Published audit coverage was higher than the truth because its denominator excluded annotated requirements | `audit_coverage` denominator | `polarity_covered:3283-3296` + `TestAuditCoverageCountsEveryVerdict` |
| 3 | ISSUE | The gate's own summary line contradicted its own ledger about how many verdicts are unproven | `run_check` summary | `:5502-5512` + `TestAuditSummaryLineAgreesWithTheLedger` |
| 4 | ISSUE | `_unit_identity` as a set admitted a false FRESH when one of two same-unit tags was deleted | `_unit_identity` | multiset at `:2419-2435` + `TestUnitIdentityIsAMultiset` |
| 5 | NOTE | `_write_audit`'s comment promised a byte-level re-read its code did not perform | `_write_audit` | `:2727-2730` + `TestWriteAuditValidatesTheStagedBytes` |
| 6 | ISSUE | `verdict_is_fresh` was dead code carrying eight tests, while two docstrings asserted the live path delegated to it -- and the two spellings had already diverged on the `code` map | `verdict_is_fresh` (deleted) and `verdict_freshness`'s transitional branch | Deletion + docstring corrections + eight tests re-pointed at the live branch, by a parallel agent. Found by the SECOND-pass review, that is after the first review loop reported clean, which is why `ai/rules/critical-review.md`'s "every fix is new code that needs a fresh pass" is not decoration |

## Pre-Commit Verification

Re-verified independently in this closure pass. Every command below was run now. Nothing in this
section is copied from the audit above.

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `scripts/dev/rfc_tagged_scope.py` | Yes | `ls -la`: `-rw-rw-r-- 1 thomas thomas 11106 Jul 30 00:34`. `git status --porcelain` shows `?? scripts/dev/rfc_tagged_scope.py` (new, must be in commit A) |
| `scripts/dev/rename_module_path_test.py` | Yes | `ls -la`: `-rw-rw-r-- 1 thomas thomas 22889 Jul 29 23:43`. Pre-existing, modified (+100 lines) |
| `plan/deferrals/rfcgate-3-audit-teeth.md` | No -- correctly absent | `ls` returns "No such file or directory". Created "only if something is genuinely deferred". Nothing is (see Deferrals Resolved) |
| `rfc/audit/rfc7606.json` | Yes | Loaded through the validating parser: 52 records, keys `['audited', 'reaudit_history', 'reaudit_note', 'requirements', 'rfc']`, 3 `reaudit_history` entries |
| `ai/RFC-REQUIREMENTS.md` (audit section) | Yes | `grep -n '^## Audit coverage'` -> `:201`. `grep -n '^### Audited but not proven'` -> `:354` |
| No `.ci` named in Wiring Test or Functional Tests | N-A | Scope is tooling. The Wiring Test table's own note records the N-A and names the driving surfaces instead |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-7 | The schema refuses each defect | `python3 -m unittest rfc_requirements_test.TestAuditSchema rfc_requirements_test.TestNotApplicableVerdict ...` -> **Ran 56 tests, OK** (this pass), including `test_unknown_verdict_value_fails`, `test_verdict_for_unknown_rid_fails`, `test_enforced_with_empty_tests_fails`, `test_unimplemented_needs_code_map` |
| AC-8 | A cited producer's edit stales the verdict, at symbol scope | Same run: `TestAuditCodeFingerprint` 4/4 OK. Producer citation read directly: `internal/component/bgp/wireu/split.go:456` is `func buildUpdatePayload(...)`, whose `aLen` arithmetic (`:463`) is the MP_UNREACH-before-MP_REACH ordering that IS `RFC7606-5.1-1`'s gap |
| AC-9, AC-10, AC-11, AC-12, AC-13, AC-17 | Disclosure, permanence, upgrade guard, ratchet, note proxy | Same 56-test run: `TestAuditDisclosure` 6/6, `TestAuditFindings` 8/8, `TestAuditVerdictRatchet` 6/6, `TestAuditNote` 5/5 -- all OK |
| AC-14..AC-16, AC-18..AC-20, AC-23, AC-24 | Freshness states, reseal, ledger, coverage, corpus | `python3 -m unittest` over the 27 remaining audit classes -> **Ran 100 tests, OK** (this pass) |
| AC-21 (2 of 3 clauses) | Record validates. `ze-rfc-check` green | `python3 scripts/dev/rfc_requirements.py --check` -> **exit 0**, four summary lines captured verbatim above. `--selftest` -> **Ran 520 tests, OK** |
| AC-21 (3rd clause) | `make ze-verify` | NOT run. Residual is commit sequencing, evidenced in "AC-21, stated plainly" |
| AC-22 | One definition of the tagged unit, hook behaviour unchanged | `python3 scripts/dev/hook-parity-check.py` -> **exit 0, "hook dispatcher golden check: 151/151 match"** (this pass). `grep -n rfc_tagged_scope .claude/hooks/pretool-writeedit.py` -> `:104`, `:113`, `:125`, `:1679` |
| AC-24 arithmetic | The published figure is derived and honest | Re-derived independently by driving `_collect_for_check` + `polarity_covered`: old tags-only rule gives 974 auditable / 44 audited (4.52%). The shipped rule gives **1344 / 49 (3.65%)**, delta 370 = `{single-polarity}` requirements with at least one tag. Eight verdicts sat outside the old subset. Five of them (`RFC7606-3.h-1`, `-3.h-2`, `-5.1-3`, `-7.14-2`, `-7.15-2`) were fresh `enforced` counted in no column and no worklist row |

### Wiring Verified (end-to-end)

Read each path rather than inferring it from the test name.

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `make ze-rfc-check` -> schema validation | N-A (tooling) | Yes: `run_check:5425-5428` calls `load_audits`, `check_audit_files`, `check_audit_schema`. Driven end-to-end by `TestAuditSchemaWiring` (6 cases) through `_run_capturing`, so removing the call fails the test |
| `make ze-rfc-check` -> verdict-disclosure check | N-A | Yes: `run_check:5430`. `TestAuditDisclosureWiring` (2 cases) |
| `make ze-rfc-check` -> finding + verdict ratchets | N-A | Yes: `run_check:5432-5437`, with `baseline_audits = _git_baseline_audits()` read once at `:5426`. `TestAuditRatchetWiring` (3 cases) |
| `make ze-rfc-check` -> upgrade guard | N-A | Yes: same `check_audit_findings` call at `:5432`. `TestAuditUpgradeGuardWiring` (2 cases) drives the upgrade shape specifically |
| `make ze-rfc-check` -> unit-level freshness | N-A | Yes: `run_check:5429` -> `check_audit_freshness:2551` -> `audit_freshness:2498` -> `verdict_freshness:2438`. `TestAuditUnitFreshnessWiring` |
| `make ze-rfc-check` -> note citation | N-A | Yes: `run_check:5431`. `TestAuditNoteWiring`. (Unplanned wiring row, added by the implementation) |
| `make ze-rfc-index` -> derived audit-coverage section | N-A | Yes: `run_write:5516` -> `render_ledger:3598` -> `_render_audit_coverage` at `:3636`. Live in `ai/RFC-REQUIREMENTS.md:201` |
| `make ze-rfc-reseal` -> `run_reseal` re-stamps only `shifted` | N-A | Yes: `Makefile:458-459` -> `main:5590-5591` -> `run_reseal:2736` -> `reseal_audits:2664` (`state != SHIFTED` and not transitional-with-proof -> refused). `TestResealOnlyTouchesShifted`, `TestReseal.test_a_stale_unit_is_refused`, `..._test_a_stale_requirement_is_refused` |
| `make ze-rfc-index` -> writes nothing under `rfc/audit/` | N-A | Yes: read `TestIndexNeverWritesAudit:7387` -- it snapshots the directory's bytes around `run_check` and `run_write` and compares, and `test_only_one_make_target_reseals` greps `Makefile`/`mk/` for `reseal` (returns the two `ze-rfc-reseal` lines only, re-confirmed by hand in this pass) |
| `go test ./scripts/dev` -> the Python tests run at all | N-A | Yes: `python_tests_test.go:43-45` lists `scripts/dev` in `pythonTestRoots`, and `TestPythonUnitTests:73` globs `*_test.py` under each root with a per-root non-empty assertion, so `rfc_requirements_test.py` and `rename_module_path_test.py` are both picked up automatically |
| `go test ./scripts/dev` -> the whole chain against the REAL tree | N-A | Yes: `rfc_requirements_gate_test.go:42` `TestRFCRequirementsGate`, `:56` `TestRFCLedgerFresh`, `:66` `TestRFCRequirementsSelftest`, `:79` `TestRFCRequirementsFailsClosed` -- unchanged wrappers. Their Python side was exercised directly in this pass (`--check` exit 0, `--selftest` 520 OK) |
| `python3 scripts/dev/rename_module_path.py` -> delegates to the shared re-seal | N-A | Yes: `ResealDelegates` (`rename_module_path_test.py:430`) run in this pass, 4/4 OK, including `test_the_proof_it_passes_is_rename_only_since_head` ("the predicate must be the real proof, not a lambda that says yes") and `test_it_owns_no_second_copy_of_the_rule` |
| Write/Edit of a tagged test -> hook uses the shared definition | N-A | Yes: `hook-parity-check.py` 151/151, exit 0, in this pass |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Nothing read `verdict["verdict"]` before. Six checks read it now (`check_audit_disclosure:2798`, `check_audit_findings:2899/2911`, `check_audit_verdict_ratchet`, `check_audit_note:3012`, `_verdict_claims:2233`, `audit_coverage:3364`). The rename tool writes no fingerprint at all -- `ResealDelegates.test_it_owns_no_second_copy_of_the_rule` (`rename_module_path_test.py:514`) asserts `verdict["tests"] = `, `tagged_unit_shas(` and a third token are absent from it. **2026-07-30: that third token is `verdict_is_fresh(`, whose function is being deleted (Deviation 6). The assertion is a STRING search over the rename tool's source, so it keeps passing either way, but the parallel agent owns whether to re-point it at a name that still exists** |
| A-2 | confirmed | Re-inventoried in this pass through the validating parser: ONE file, 52 entries, 49 `enforced` / 2 `unimplemented` / 1 `not-applicable`, the same 3 empty-`tests` rids, 0 dangling. Migration was 4 records + a 49-verdict `units` backfill, exactly the stated boundary |
| A-3 | confirmed (Go), resolved by declaration (non-Go) | `TestTaggedScopeCorpus.test_every_go_tag_sits_in_exactly_one_span` passes over the real tree in this pass. Non-Go is file-scoped BY DECLARATION via `scope_reader:51`, held by `TestScopeReaderIsDeclared` (3 cases) -- file scope is strictly MORE sensitive than span scope, so it can only over-trigger |
| A-4 | confirmed, against a stricter population | `check_audit_note` searches the tagged UNIT (`:3022`), not the whole file, and `--check` exits 0 over all 49 `enforced` records, so 0 of 49 fail at unit scope |
| A-5 | **broken**, resolved by OR-1 | Two of three were transcriptions. `RFC7606-8-1` had nothing to transcribe. Now `not-applicable` with a 400-character `no_code_path` reason and an agreeing `{not-applicable}` annotation at `rfc/short/rfc7606.md:356`. Mistake Log row recorded. Deviations entry recorded |
| A-6 | confirmed, and the figure it predicted was WRONG | The arithmetic is reproducible from the tool's own parse, which is what A-6 claimed. The number it predicted (44 of 974) was produced by a denominator that ignored annotations. The shipped, honest figure is 49 of 1344 (3.65%). Both derivations re-run by hand in this pass |
| A-7 | confirmed and tested | `TestIndexNeverWritesAudit` (2 cases) run in this pass. `grep -rn 'reseal' Makefile mk/` returns exactly `Makefile:458` and `:459` |
| A-8 | confirmed. Replacement value is `enforced` | Re-judged rather than mapped, with four recorded reasons the last two of which are non-mechanical (pointer-identity negatives, would-it-fail recorded from mutation, `test/plugin/rfc7606-relay-one-field.ci` failing 4/4 when a relay branch is reverted). `rfc/audit/rfc7606.json` `RFC7606-5.1-2` is `enforced` with `units`, and no escalation was reached |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 10 (test infrastructure): the audit record is validated, coverage published, `ze-rfc-reseal` the only writer | `docs/functional-tests.md` gained "What the tags cannot say: the audit record", whose anchor `<!-- source: scripts/dev/rfc_requirements.py -- AUDIT_VERDICTS/check_audit_schema/check_audit_findings -->` names symbols that exist at `:1967`, `:2182`, `:2870`. Its claims "A verdict that existed at HEAD cannot vanish" and "Audit coverage is monotonic per requirement id" match `check_audit_verdict_ratchet:2966-2971` | Yes |
| Row 10b (new make target discoverable) | `grep -n 'ze-rfc-reseal' ai/INDEX.md docs/contributing/rfc-implementation-guide.md` -> `ai/INDEX.md:220` (Dev Tools), `:381` (keyword map), guide `:520`, `:524`, `:539`. The `ai/INDEX.md:217` `ze-rfc-index` row was also corrected to state it never writes `rfc/audit/`, which matches `run_write:5516-5528` | Yes |
| Row 15 (generated inventory + hook map) | `ai/RFC-REQUIREMENTS.md:201` `## Audit coverage`. `ai/rules/hook-mapping.md:119` names the shared leaf and the derived carrier list, which matches `.claude/hooks/pretool-writeedit.py:1759` | Yes |
| Row 17 (the skill shows the record shape) | `ai/skills/ze-rfc-audit.md:70-74` (five values), `:90-94` (field table incl. `no_code_path`, `upgrade_reason`), `:80` (`not-applicable` is not a shortcut). Mechanically held to the code by `TestVerdictVocabularyAgreesWithTheSkill.test_the_skill_documents_exactly_the_gates_enum` and `TestSkillDocumentsWhatTheSchemaAccepts` (3 cases), all passing in this pass | Yes |
| Row 9 (RFC behavior newly proven?) answered No | `git status --porcelain -- rfc/short/` shows only the four files a CONCURRENT session modified (`rfc1035`, `rfc3765`, `rfc4486`, `rfc5301`). `rfc/short/rfc7606.md` is NOT modified, so no annotation and no compliance claim changed here. `docs/features/rfc-status.md` is likewise unmodified | Yes |
| Row 16 (source anchors naming the changed files) | The one new anchor is in `docs/functional-tests.md`, verified above. `docs/contributing/rfc-implementation-guide.md:524`'s anchor lists all five RFC make targets, which matches `Makefile:437`, `:442`, `:453`, `:461`, `:458` | Yes |

## Core Insight

**A state is not implemented until its DOCUMENTED authoring path has been driven end to end.**
OR-1's `not-applicable` verdict passed its schema tests, its claim tests, its wiring test and its
ledger test, and was still permanently broken -- because every test wrote `"tests": {}` while
`ai/skills/ze-rfc-audit.md` tells the author to omit the field, and `None == {}` is False. The
schema accepted both spellings. A consumer four hundred lines away did not, and the error it
produced was untrue in all three of its clauses, and it recommended a command that reproduced the
record. The normaliser that fixes it already existed for the load path. Two call sites bypassed
it because each was written before the state that needed it.

The generalisation, which is what makes this worth keeping: when the deliverable is a RECORD
FORMAT that humans and agents author from a document, the document is a code path. Test it the
way an author would follow it, not the way the schema permits.

**Sharpened 2026-07-30 by the second-pass review.** Of those two bypassing call sites, one was
never executed at all: `verdict_is_fresh` had zero non-def callers, eight tests, and two
docstrings asserting the live path delegated to it. So the same failure appeared twice in one
module, at two different altitudes -- a documented authoring path nobody drove, and a documented
CALL PATH nobody wired -- and in both cases a green suite was reporting on a spelling the
product does not use. That is this spec's own subject reproduced inside it: **a test suite over
dead code reads exactly like coverage, and a docstring's account of who calls whom is a belief,
not a wiring.** The audit-record machinery answers the first for RFC evidence by fingerprinting
the unit a tag governs. Nothing here answers the second, and a reviewer grepping for call sites
is still the only thing that catches it.
