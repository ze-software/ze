# Spec: rfcgate-3 audit teeth

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Depends | spec-rfcgate-1-extraction, spec-rfcgate-2-evidence (the umbrella's merge order 1, 2, 3, 4) |
| Phase | - |
| Deferral shard | `plan/deferrals/rfcgate-3-audit-teeth.md` |
| Updated | 2026-07-29 |

Part of the `rfcgate` spec set. Umbrella: `plan/spec-rfcgate-0-umbrella.md`.
Siblings are referenced by name only; this spec is independently implementable.

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
(`scripts/dev/rfc_requirements.py:1227-1236`) compares exactly two things:
`verdict.get("requirement_sha")` and `verdict.get("tests")`. It never reads
`verdict["verdict"]`. `grep -rn '\["verdict"\]\|\.get("verdict"' scripts/ mk/
Makefile` returns a single hit, `scripts/dev/rename_module_path.py:386`, and that
is a WRITE of `verdict["tests"]`, not a read of the value. So a verdict recorded
`weak` or `wrong` is treated by the gate exactly as `enforced` is. The skill that
produces these records says the opposite of what the gate does:
`ai/skills/ze-rfc-audit.md:94` states that "`weak` and `wrong` are the valuable
outputs. A run that returns all `enforced` on first pass has probably not read
anything." The one mechanism designed to surface a bad test writes its findings
into a field no code reads.

**D2. No schema validation.** `load_audit`
(`scripts/dev/rfc_requirements.py:1239-1249`) does a bare `json.load` then
returns `data.get("requirements", {})`, discarding every other key without
inspecting it. There is no field-presence check, no enum check on `verdict`, no
check that a recorded rid resolves to a real requirement of that RFC, and no
check that a requirement carrying tags has a non-empty `tests` map. The
vocabulary has already drifted: `rfc/audit/rfc7606.json` records `implemented`
for `RFC7606-5.1-2`, which is outside the four values defined at
`ai/skills/ze-rfc-audit.md:67-72` (`enforced`, `weak`, `wrong`, `unimplemented`).
Nothing noticed, because nothing looks.

**D3. Permanently-fresh verdicts.** Three of the 52 entries in
`rfc/audit/rfc7606.json` carry an empty `tests` map: `RFC7606-5.1-1`,
`RFC7606-5.4-1`, `RFC7606-8-1`. Their freshness test reduces to `{} == {}`, so
they can never go stale. They are claims about CODE (the gap is real, the
divergence is deliberate) fingerprinted against TESTS that do not exist, which
makes them structurally unfalsifiable. Under `ai/rules/rfc-compliance.md`
("Every earlier answer that pointed away from full compliance or full proof is
VOID", and specifically the row naming "A code comment or `rfc/audit/*.json`
verdict calling the deviation deliberate"), a permanently-fresh verdict blessing
a void annotation is the worst combination the system can produce.

**D4. Coverage is 4.5%.** Measured on the tree at 2026-07-29: 974 gated,
enrolled, both-polarity requirements exist; 44 carry a verdict; all 44 are
rfc7606, the only file in `rfc/audit/`. The remaining 930 span 129 RFCs. The
docstring of `check_audit_freshness`
(`scripts/dev/rfc_requirements.py:1285-1287`) makes a missing verdict
deliberately non-fatal: "the audit is sampled, the gate is total." That is the
right call, and it also means the semantic half of the gate covers one RFC in
166.

**The false-stale problem this must design around.** `tagged_unit_shas`
(`scripts/dev/rfc_requirements.py:1252-1270`) fingerprints the WHOLE ENCLOSING
FILE and keys on `file:line`, so a verdict goes stale on any edit anywhere in a
tagged file and on any line shift. Measured on the one existing audit: of 15
commits touching `rfc/audit/rfc7606.json`, five were mechanical re-stamps where
nothing about what a test asserts had changed (a nine-line header prepended,
shifting every key by +9; a module-path rename; sibling subtests added to the
same file; a helper signature change at unrelated call sites). Zero of those five
changed a verdict. The pattern is filed as F18 in
`plan/learned/HOOK-FRICTION.md:716-753`. Exactly one class is automated today:
`reseal_rfc_audits` (`scripts/dev/rename_module_path.py:318-409`) handles a
whole-repo string substitution, proving per file via `rename_only_since_head`
(`:286-299`) that the normalized content is identical under the rename before
re-sealing, and refusing any verdict whose `requirement_sha` moved. Scaled from
44 verdicts to 930 or more, the predictable failure mode is BLIND RE-STAMPING:
the badge stays green while nobody re-reads.

**Goal.** Give the audit record teeth and make its coverage measurable and
monotonic, so that a fleet performing the remaining 930 audits produces evidence
rather than decoration. Specifically: make the verdict value load-bearing without
punishing the auditor who reports a bad test; validate the record fail-closed;
make an unfalsifiable verdict impossible to record; remove the human step from
every re-stamp that carries no judgement; and publish the coverage figure derived
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
the judgement: a record that cannot be made legal without reading is repaired;
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
  → Decision: the four-value verdict vocabulary at `:67-72` is the schema enum; `implemented` in `rfc/audit/rfc7606.json` is drift, not a fifth value.
  → Constraint: `:94` says `weak` and `wrong` are the valuable outputs, so the gate must not make recording them the expensive path.
  → Constraint: `:100-104` says every annotation is VOID as authority, so an `unimplemented` verdict may not be treated as a settled decision by anything this spec builds.
- [ ] `ai/rules/rfc-compliance.md` - the owner directive that voids prior narrowing answers
  → Constraint: an `rfc/audit/*.json` verdict calling a deviation deliberate is void by default; the machinery may record it, must disclose it, and must never let it read as authority.
  → Constraint: choosing anything narrower than full compliance plus full proof is Thomas's call, so the gate escalates rather than self-approves.
- [ ] `ai/rules/fail-closed-guards.md` - the guard discipline this spec implements
  → Constraint: a guard must fail closed or say something; `load_audit` today does neither for anything except unparseable JSON.
  → Constraint: the zero-value trap is the concrete defect in D3 (`{} == {}` reads as a legitimate "fresh") and in a unit extractor that returns empty on failure.
  → Constraint: drive the guard from the entry point (`run_check`), not only the helper, so a check that stops being called fails a test.
- [ ] `ai/rules/derive-not-hardcode.md` - the coverage figure must be derived
  → Decision: audit coverage is rendered into `ai/RFC-REQUIREMENTS.md` from the audit files and the tags, never maintained by hand, and its staleness is caught by the existing `check_ledger_fresh`.
- [ ] `ai/rules/testing.md` - the published-versus-gated distinction
  → Decision: the shape prescribed for `ze-test-health-check` ("STRUCTURAL facts only ... volume counters are published, not gated") is the model for `weak` and `wrong` counts.
- [ ] `plan/learned/HOOK-FRICTION.md:716-753` - F18, the false-stale friction
  → Constraint: the proposed fix there is a message change; this spec does the structural version (a unit-level fingerprint) because at fleet scale a better message still leaves a human step in every no-judgement re-stamp.
- [ ] `ai/rules/spec-no-code.md` - spec style
  → Constraint: tables and prose only; field shapes below are described as tables, never as JSON or Python.

### RFC Summaries (Scope: protocol)
Not applicable. Scope is tooling: this spec changes the gate that measures RFC
evidence, and changes no protocol behavior and no tagged test. No RFC summary is
read, edited, or relied on for a protocol claim here.

**Key insights:** (minimal context to resume after compaction)
- The verdict field is written by a skill and read by nothing. Making it read is the whole point, but the reader must be designed so honesty is never the expensive path.
- The false-stale problem and the blind-re-stamp problem are the same problem: every no-judgement re-stamp trains the reflex that re-stamping is what you do. Removing the no-judgement class removes the training data.
- Re-measured 2026-07-29 with the hook's own scope rule (`_go_func_scopes`, `.claude/hooks/pretool-writeedit.py:1653`): **all 2571 scanned Go tags sit inside exactly one top-level function span.** Zero sit outside every span, zero sit inside more than one, and across the 368 tagged `*_test.go` files no two spans overlap. A function-level fingerprint is therefore total over the Go corpus today, with the file-level fallback reserved for the 4 `.ci` tags and for the day a Go tag lands outside a span.
- **The measurement trap, stated because it produced a wrong number once:** `_go_func_scopes` returns CHARACTER OFFSETS, not line numbers, so a tag's line number cannot be compared against a span. The second trap is which regex counts as a tag. The gate credits only `_GO_TAG_RE` (`scripts/dev/rfc_requirements.py:148`, a `//` comment at line start): 2571 tags, all in-span. The hook's broader `_RFC_TAG` (`:1615`) also matches the phrase in ordinary prose and finds 2574, of which one (`internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go:441`, a backticked mention inside a `test-relax:` comment sitting between two functions) is outside every span. That prose mention is NOT a scanned tag; it is what makes the hook widen to whole-file scope for that one file, which is the fallback working as designed. An earlier draft of this spec reported "2570 of 2571 with 1 outside" by mixing the two populations.
- Measured 2026-07-29: all 49 non-empty-`tests` verdicts in `rfc/audit/rfc7606.json` already name at least one identifier that occurs literally in one of their tagged files. Requiring that is free for the existing corpus.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `scripts/dev/rfc_requirements.py:1227` - `verdict_is_fresh`: compares `requirement_sha` and the `tests` map only; the verdict value is never read.
- [ ] `scripts/dev/rfc_requirements.py:1239` - `load_audit`: bare `json.load`, returns `data.get("requirements", {})`, no validation of any kind; a missing file returns an empty map (legal), a malformed file raises `ParseError` (fail-closed for syntax only).
- [ ] `scripts/dev/rfc_requirements.py:1252` - `tagged_unit_shas`: hashes the whole enclosing file, caches per file, keys `file:line`; documented as coarse on purpose.
- [ ] `scripts/dev/rfc_requirements.py:1273` - `check_audit_freshness`: iterates REQUIREMENTS, skips any with no verdict, fails on a stale one. Never iterates the audit file, so a verdict for an unknown rid is silently discarded.
- [ ] `scripts/dev/rfc_requirements.py:955` - `check_coverage_ratchet`: the monotonic-evidence pattern this spec copies, including its baseline scoping to RFCs enrolled at HEAD.
- [ ] `scripts/dev/rfc_requirements.py:1007` - `check_retired_requirements`: the deletion-is-the-cheapest-escape lesson, and the id-attribution care (longest prefix first, silent stems excluded).
- [ ] `scripts/dev/rfc_requirements.py:1163` - `check_status_agreement`: the existing disclosure cross-check against `docs/features/rfc-status.md`, with the fail-open it already fixed (a blank Remaining is not a disclosure).
- [ ] `scripts/dev/rfc_requirements.py:1408` - `_render_rollup`: the actionable per-RFC view; the audit columns are added here.
- [ ] `scripts/dev/rfc_requirements.py:1465` - `render_ledger`: composes the ledger; `check_ledger_fresh` (`:1578`) makes it monotonically regenerated.
- [ ] `scripts/dev/rfc_requirements.py:1629` - `run_check`: the entry point every new check must be wired into; `main` (`:1754`) dispatches `--check`, `--check-fresh`, `--write`, `--selftest`.
- [ ] `scripts/dev/rfc_requirements.py:763` - `_git_baseline_summary_stems`: the None-versus-empty polarity discipline every new baseline reader must copy.
- [ ] `scripts/dev/rename_module_path.py:286` - `rename_only_since_head`: proves a file differs from HEAD by nothing but the rename, under `rfc_requirements`'s own normalization.
- [ ] `scripts/dev/rename_module_path.py:318` - `reseal_rfc_audits`: the only automated re-stamp today; refuses on a changed `requirement_sha` or on any file where more than the rename moved; appends the previous `reaudit_note` to `reaudit_history`.
- [ ] `.claude/hooks/pretool-writeedit.py:1653` - `_go_func_scopes`: top-level func spans as CHARACTER OFFSETS (doc comment through closing brace, capped at the next func's doc comment), the two boundary bugs its docstring records, and the fact that the spans are not a partition of the file.
- [ ] `.claude/hooks/pretool-writeedit.py:1689` - `_enclosing_tagged_scope`: the proven definition of "the tagged unit", including the whole-file fallback when a tag sits outside every function span. Its docstring's "0 of 2515 Go tags" is a stale total (2571 today) but the polarity still holds; see Key insights.
- [ ] `scripts/dev/rfc_requirements_test.py:34` - `_patched` and `_run_capturing`: the gate-level test harness that drives `run_check` end to end with substituted module constants.
- [ ] `scripts/dev/rfc_requirements_gate_test.go:42` - `TestRFCRequirementsGate`, `TestRFCLedgerFresh`, `TestRFCRequirementsSelftest`, `TestRFCRequirementsFailsClosed`: the Go wrappers that put the Python gate into `go test`.
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_test.go:269` - `TestAdjRIBInRFC7606TreatAsWithdrawRemovesRoute`: the tagged unit whose verdict false-staled four times; read to confirm the unit is self-contained (it calls `newTestManager` and `message.SynthesizeWithdraw`, and its assertions are inline), which is what makes a unit-level fingerprint meaningful here.

**Behavior to preserve:**
- A MISSING verdict remains legal and non-fatal. Coverage is sampled; the gate is total. Nothing in this spec may turn "not yet audited" into a build failure, because that would force verdict generation, which is the one outcome the void deferral row correctly warned against.
- A stale verdict remains fatal, and the bias stays "a false stale costs a re-read, a false fresh ships an unenforced compliance claim".
- Every existing check in `run_check` keeps its current semantics and message. This spec adds checks and refines one freshness comparison; it does not relax any existing failure.
- `make ze-rfc-check` stays read-only. Nothing under `--check` writes to `rfc/audit/`.
- `make ze-rfc-index` also stays a non-writer of `rfc/audit/`. It renders the ledger and nothing else touches the evidence files, which the 2026-07-29 ruling makes a preserved property rather than an incidental one (A-7).
- The four existing exit behaviors of `main` and the wording of `check_audit_freshness`'s stale message for a genuine judgement change stay recognizable, so existing session knowledge and the F18 record still apply.
- `rfc/enrolled.txt`, the requirement id rules, and the tagged-test hook are untouched.

**Behavior to change:**
- `load_audit` gains a validating parse and a second consumer direction (iterating the audit file, not only the requirements).
- The freshness comparison gains a unit-level fingerprint and a third state.
- The verdict value becomes load-bearing, in the three distinct ways set out under Design.
- `ai/RFC-REQUIREMENTS.md` gains a derived audit-coverage section, which makes the ledger's byte content depend on `rfc/audit/*.json` for the first time.
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
6. Errors accumulate into `run_check`'s `errs` list and are printed with the existing formatting; exit 2 on any.
7. Independently, `render_ledger` folds the same audit data into a derived coverage section of `ai/RFC-REQUIREMENTS.md`, whose staleness is caught by the existing `check_ledger_fresh`.
8. Under `--reseal` only, reached from the dedicated `make ze-rfc-reseal` target and never from `--check` or `--write`, verdicts in the `shifted` state are re-stamped mechanically after proving unit identity, and the previous `reaudit_note` is preserved into `reaudit_history`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Skill (`/ze-rfc-audit`) → gate | A JSON file per RFC under `rfc/audit/`, hand-written by an agent; the schema is the contract and is validated on read | No |
| Gate → public compliance claim | `docs/features/rfc-status.md` rows, cross-checked by the existing `check_status_agreement` and by the new verdict-disclosure check | No |
| Gate → generated ledger | `ai/RFC-REQUIREMENTS.md`, written by `run_write`, guarded by `check_ledger_fresh` | No |
| Gate → edit-time hook | `.claude/hooks/pretool-writeedit.py` and the gate must agree on what "the tagged unit" is; a shared leaf module is the single definition | No |
| Gate → rename tool | `scripts/dev/rename_module_path.py` imports `rfc_requirements` as a library today and must keep exactly one copy of the fingerprint rule | No |
| Gate → git HEAD | `git show` / `git grep` / `git ls-tree` through the existing baseline helpers, with the None-versus-empty polarity discipline | No |

### Integration Points
- `run_check` (`scripts/dev/rfc_requirements.py:1629`) is where every new check is called; a check not called there is invisible to `make ze-rfc-check` and to `TestRFCRequirementsGate`.
- `render_ledger` and `_render_rollup` are where the coverage figure is derived; `check_ledger_fresh` is what makes the derivation mandatory.
- `main` (`:1754`) gains the `--reseal` mode. There are then THREE invocation sites: `Makefile:437` (`ze-rfc-check`, read-only) and `Makefile:442` (`ze-rfc-index`, ledger only) are unchanged in what they may write, and a new `ze-rfc-reseal` target is the sole caller of `--reseal` (A-7).
- `scripts/dev/rename_module_path.py` becomes a caller of the shared reseal rather than an owner of a second implementation.
- `.claude/hooks/pretool-writeedit.py` becomes a caller of the shared tagged-scope module, with `scripts/dev/hook-parity-check.py` proving its behavior is unchanged.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | Verify at implementation: every new check is reached from `run_check`, and each has a `_patched`-driven wiring test that fails if the call is removed |
| No unintended coupling (components stay isolated) | No | Verify at implementation: the shared tagged-scope module must not import the gate or the hook; it is a leaf both depend on |
| No duplicated functionality (extends existing, does not recreate) | No | Verify at implementation: `rename_only_since_head` keeps its rename-specific proof but the re-stamp loop is deleted in favor of the shared one; exactly one definition of a tagged unit exists after this change |
| Zero-copy preserved where applicable (refs, not copies) | No | Not applicable: this is Python tooling with no wire path. The performance constraint that does apply is per-file hash caching, which `tagged_unit_shas` already does and the unit extractor must also do |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them; no per-feature field, switch case, or factory added to a core/shared package (`ai/rules/plugin-self-containment.md`) | No | Verify at implementation: no per-RFC special case may appear anywhere. The verdict vocabulary is one enum constant read by every consumer, and the audit files are discovered by enrolment, never enumerated |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Nothing in the repo reads `verdict["verdict"]`, so making it load-bearing breaks no existing consumer | `grep -rn '\["verdict"\]\|\.get("verdict"' scripts/ mk/ Makefile` returns only `scripts/dev/rename_module_path.py:386`, a write of `verdict["tests"]` | The change would alter an existing behavior silently | Re-run the grep at implementation; assert in the wiring test that the rename tool never mutates `verdict` | unvalidated |
| A-2 | The only audit file is `rfc/audit/rfc7606.json`, holding 52 verdicts: 49 `enforced`, 2 `unimplemented`, 1 `implemented`, of which 3 have an empty `tests` map | Inventory run 2026-07-29 over `rfc/audit/` | The schema migration is larger than 4 records and the "machinery only" scope boundary is wrong | Re-run the inventory as the first implementation step | unvalidated |
| A-3 | A tagged Go test resolves to exactly one top-level function span, or to the documented whole-file fallback | Re-measured 2026-07-29 by driving `_go_func_scopes` over all 368 tagged `*_test.go` files, comparing CHARACTER OFFSETS (not line numbers): all 2571 tags scanned by `_GO_TAG_RE` sit inside exactly one span, none outside, none in two, and no two spans overlap; the 4 `.ci` tags are file-scoped | The unit fingerprint would under-fingerprint some tests, which is a false-fresh, the worst failure available | Measured directly at design time (above); the standing guard is a corpus test that runs the extractor over every tag in the tree and asserts a resolved unit or an explicit file-scope marker for each, never an empty result | confirmed (design-time measurement; corpus test keeps it true) |
| A-4 | Every existing `enforced` verdict's note already names at least one identifier occurring in one of its tagged files, so the note-cites-a-symbol check costs the existing corpus nothing | Measured 2026-07-29: 0 of 49 non-empty-`tests` verdicts fail it | The check would go red on honest existing records and would have to be introduced as advisory first | Re-run the measurement at implementation, before the check is made blocking | unvalidated |
| A-5 | The three empty-`tests` verdicts already name their producing code in prose, so requiring a machine-checkable `code` map is a transcription, not a re-judgement | `RFC7606-5.1-1` cites `wireu/split.go:440-476`; `RFC7606-5.4-1` cites `FamilyRIB.insertOpaque` and `buildFwdBody` | Filling the `code` map would require re-auditing those requirements, which is out of scope and must then be escalated | Read the three notes and confirm each names a file or symbol that exists | unvalidated |
| A-6 | The coverage figures (974 both-polarity gated requirements, 44 with a verdict, 930 remaining across 129 RFCs) are reproducible from the tool's own parse | Reproduced exactly 2026-07-29 by driving `_collect_for_check` and `load_audit` | The published figure would disagree with the ledger and the spec's premise would be misstated | The ledger render itself, once it publishes the figure; a unit test pins the arithmetic on a fixture | unvalidated |
| A-7 | ~~`make ze-rfc-index` is an acceptable writer of `rfc/audit/*.json`, by analogy with its existing ownership of the generated ledger~~ SUPERSEDED by the 2026-07-29 ruling: a DEDICATED `make ze-rfc-reseal` target is the only writer of `rfc/audit/*.json`, and neither `ze-rfc-check` nor `ze-rfc-index` writes there | Owner ruling 2026-07-29. `ze-rfc-index` is run routinely (it is required after any tag move, and its freshness variant `run_check_fresh` (`:1725`) is reached from `make ze-doc-test` through `mk/inventory.mk:106`), so letting it silently re-seal a hand-authored evidence file would AUTOMATE the blind re-stamp reflex this spec exists to remove. A dedicated target makes every write to an evidence file intentional and greppable, at a cost of one make target | Not applicable: the question is settled. The residual cost is the one new target and the habit of running it, which the `shifted` message names explicitly | The ruling itself; plus a test asserting that `--check` and `--write` write nothing under `rfc/audit/`, and that `--reseal` writes only `tests` and `reaudit_history` | confirmed (owner ruling 2026-07-29) |
| A-8 | The `/ze-rfc-audit` skill's four-value vocabulary is the intended enum, and `implemented` is drift rather than a fifth value someone meant | `ai/skills/ze-rfc-audit.md:67-72` defines four; one record uses a fifth. Settled by the 2026-07-29 ruling: `RFC7606-5.1-2` is a BROKEN RECORD, so re-judging it is in scope and is the sole audit work this spec permits (see Task) | The enum is wrong and the schema would reject valid records | The ruling settles the enum and the drift. It does NOT settle which legal value replaces `implemented`: that is a judgement made at implementation by reading `rfc/full/rfc7606.txt`, and anything other than `enforced` escalates per `ai/rules/rfc-compliance.md`. A blind rewrite of the value stays banned (see Known Limitations) | confirmed (owner ruling 2026-07-29; replacement value still to be judged) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | At fleet scale the dominant failure is a false `enforced` written without reading. No gate can prove a human read something | A batch of new verdicts whose notes are uniform in shape, name only the test function, and say nothing about what the assertion would do under non-compliance | Make the cheap fake expensive in the ways a machine CAN check (note must cite a symbol present in the tagged unit; `enforced` requires both polarities or a `{single-polarity}` annotation; upgrades require a changed unit or a stated reason) and make the corpus samplable (every note is in git, coverage is published per RFC) |
| R-2 | The unit extractor mis-resolves a span and under-fingerprints, shipping a false fresh | A unit sha equal to the file sha where a span was expected, or an empty extraction | Fail closed: an unresolvable span falls back to the FILE fingerprint and is recorded as file-scoped, never treated as "unchanged". An empty extraction is an error, never a hash input |
| R-3 | The note-cites-a-symbol check goes red when a tagged test is renamed, and people respond by pasting junk tokens into notes | Notes that consist of a bare identifier with no sentence | Accept the red: a renamed test IS a reason to re-read. Require only ONE matching token, and make the error name the tokens checked and the files searched so the fix is obvious |
| R-4 | Gating `wrong` on public disclosure creates a red the auditor cannot clear alone (it needs a `docs/features/rfc-status.md` edit) | An auditor reporting a finding and then reverting it to clear the build | The error names the exact row and the required change; and the finding ratchet means reverting the verdict is itself a failure, so the only exit is the docs edit |
| R-5 | The `code` map for `unimplemented` verdicts becomes a new false-stale class, since producer files churn far more than test files | Repeated re-seals of the same `unimplemented` verdict | Fingerprint the cited SYMBOL's span, not the producer file, using the same extractor; fall back to the file only when the symbol cannot be resolved, and say so in the error |
| R-6 | A ratchet keyed on a percentage would fail the build for adding a tagged test (the denominator grows) | A red gate on a commit that only added coverage | Ratchet the SET of audited requirement ids, never the ratio. The percentage is published and never gated. This is written into the AC list as an explicit negative test |
| R-7 | Baseline reads from git degrade (shallow clone, detached state) and a ratchet accuses everything | A wall of ratchet violations on a fresh clone | Copy the polarity discipline of `_git_baseline_summary_stems` (`:763-794`): return None on "could not look", and treat None as "no opinion", never as "nothing was there" |
| R-8 | Making the ledger depend on `rfc/audit/*.json` couples two regen paths, so an audit edit without `make ze-rfc-index` reds the build for a docs reason. Since A-7 split the re-seal into its own target, clearing a `shifted` verdict is now TWO commands: `make ze-rfc-reseal` rewrites `tests`, which in turn stales the ledger | `check_ledger_fresh` failing on commits that only touched an audit file, or a developer running the re-seal and being met with a second, different red | Acceptable and consistent with today's behavior for tags. Each message names the ONE command that clears it (`shifted` says `make ze-rfc-reseal`, a stale ledger says `make ze-rfc-index`), so neither red is a guessing game; the two-command remedy is the deliberate price of keeping every write to an evidence file intentional |
| R-9 | The shared tagged-scope module changes hook behavior as a side effect | A `hook-parity-check.py` golden diff | The golden table is the gate: the hook's exit codes must be byte-identical after the extraction. Re-bless only if a case changed intentionally, which here it must not |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible and no daemon behavior. The failure modes are a build gate that is too loud (false stale, ratchet noise on a degraded git baseline) or too quiet (a false-fresh verdict from a mis-resolved unit span, which is the serious one: it would let an unenforced compliance claim keep its badge) |
| How is it reverted? | Single commit revert. The only persisted artifacts are `rfc/audit/*.json` field additions (`units`, `code`, `upgrade_reason`) and the ledger's new section; both are additive and a revert leaves the old boolean freshness rule reading the fields it always read |
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
| AC-11 | A `weak` or `wrong` verdict present at HEAD is absent in the working tree | The gate fails. A finding may not be deleted; it is resolved by a fix or by retiring the requirement |
| AC-12 | A `weak` or `wrong` verdict at HEAD becomes `enforced` while every unit fingerprint is unchanged and no `upgrade_reason` is recorded | The gate fails. A finding cannot become proof without something changing |
| AC-13 | A requirement carrying a fresh verdict at HEAD carries none in the working tree | The gate fails. Audit coverage is monotonic per requirement id |
| AC-14 | A sibling test in a tagged file is edited, leaving the tagged unit byte-identical (the F18 case) | The gate does NOT report a judgement change. It reports the `shifted` state and names `make ze-rfc-reseal` as the remedy, in those words, and no human re-read is asked for. The message must NOT name `make ze-rfc-index`, which does not clear this state (A-7) |
| AC-15 | A line inside the tagged unit is changed | The gate fails as stale, and the message names WHICH unit changed, distinguishing it from a requirement-text change and from a sibling edit |
| AC-16 | `make ze-rfc-reseal` runs over an audit file holding `shifted`, `stale`, and `fresh` verdicts | Only `shifted` verdicts are re-stamped; `verdict`, `note`, `units`, `code`, and `requirement_sha` are byte-identical afterward for every record; the previous `reaudit_note` is preserved into `reaudit_history`. Neither `make ze-rfc-check` nor `make ze-rfc-index` writes anything under `rfc/audit/`, so the re-seal target is the only path by which an evidence file changes without a human editing it |
| AC-17 | An `enforced` verdict whose note names no identifier occurring in any of its tagged units | The gate fails, listing the tokens checked and the files searched |
| AC-18 | `ai/RFC-REQUIREMENTS.md` is regenerated | It carries a derived audit-coverage figure per RFC and repo-wide, plus the named `weak` and `wrong` worklist; editing an audit file without regenerating fails `check_ledger_fresh` |
| AC-19 | A new tagged requirement is added, lowering the audited PERCENTAGE while removing no verdict | The gate passes. The ratchet is on the set of audited ids, never on the ratio |
| AC-20 | A verdict recorded before this change, carrying no `units` field, is currently fresh at file level | The gate passes under the transitional rule and the backfill records its unit fingerprints without re-judging; a verdict that is NOT currently fresh is never backfilled |
| AC-21 | The whole tree after implementation | `rfc/audit/rfc7606.json` validates, `make ze-rfc-check` is green, and `make ze-verify` passes |
| AC-22 | The tagged-scope definition | Exactly one definition exists in the tree; the gate and `.claude/hooks/pretool-writeedit.py` both use it, and `scripts/dev/hook-parity-check.py` shows no golden change |
| AC-23 | The corpus of every `RFC requirement:` tag in the tree | The extractor resolves each to a unit span or an explicit file-scope marker; it never returns an empty result that would hash to a constant |
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
| `TestTaggedScopeCorpus.test_every_tag_in_the_tree_resolves` | same | AC-23, A-3; runs over the real tree like `TestRealTree` does today | |
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
`ai/rules/interop-and-goal-validation.md` is triggered by wire-visible change;
there is none here. The rule's vacuity guidance is nonetheless applied to this
spec's own tests: each new check has a negative case proving the gate goes red
when the defect is present and green when it is not, which is the same
discrimination requirement one layer up.

## Files to Modify
- `scripts/dev/rfc_requirements.py` - validating `load_audit`; the new checks (schema, disclosure, findings ratchet, verdict ratchet, upgrade guard, note citation); unit-level freshness with four states; the `--reseal` mode; the derived audit-coverage section in `_render_rollup` and `render_ledger`
- `scripts/dev/rename_module_path.py` - `reseal_rfc_audits` delegates to the shared re-seal instead of owning a second loop; `rename_only_since_head` is kept as the rename-specific extra proof
- `.claude/hooks/pretool-writeedit.py` - `_enclosing_tagged_scope` sources its span logic from the shared module rather than a private copy
- `Makefile` - a NEW `ze-rfc-reseal` target calls `--reseal`; `ze-rfc-check` and `ze-rfc-index` are both unchanged, the first staying read-only and the second staying a writer of the ledger only (A-7)
- `rfc/audit/rfc7606.json` - schema migration of 4 records: the `implemented` value on `RFC7606-5.1-2` corrected by targeted re-judgement, and a `code` map transcribed for the three empty-`tests` verdicts; plus the mechanical `units` backfill for verdicts that are currently fresh
- `ai/RFC-REQUIREMENTS.md` - regenerated; gains the audit-coverage section
- `ai/skills/ze-rfc-audit.md` - documents the enum as binding, the `units` and `code` fields, the `upgrade_reason` field, the escalation obligation on an `unimplemented` verdict, and the fact that `weak` and `wrong` are recorded without failing the build
- `ai/rules/hook-mapping.md` - the new gate rows, so the enforcement map stays complete
- `ai/INDEX.md` - the new `make ze-rfc-reseal` target in the Dev Tools table (beside `ze-rfc-check` at `:212` and `ze-rfc-index` at `:213`) and in the RFC keyword-map row (`:372`), per `ai/rules/discovery-updates.md`
- `docs/contributing/rfc-implementation-guide.md` - the contributor-facing pair of RFC targets (`:516`) becomes three, and the audit paragraph (`:523`) states which red each one clears
- `plan/deferrals/rfc-gate-regression-ratchets.md` - the 2026-07-20 row's disposition is updated to record that its answer is void as authority and that this spec supersedes the machinery half of it

## Files to Create
- `scripts/dev/rfc_tagged_scope.py` - the single definition of "the tagged unit": top-level Go function spans including their doc comments, the whole file for `.ci` and for a tag outside every span, imported by both the gate and the edit-time hook
- `scripts/dev/rename_module_path_test.py` - unit tests for the rename tool's delegation, if the file does not already exist at implementation time
- `plan/deferrals/rfcgate-3-audit-teeth.md` - this spec's deferral shard, created only if something is genuinely deferred

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No config surface; this is a build-time gate |
| YANG validation constraints | No | As above |
| YANG custom validators | No | As above |
| CLI commands/flags | No | `rfc_requirements.py` is a developer script, not a `ze` subcommand; its `--reseal` flag follows the existing `--check` / `--write` mode style |
| CLI grammar (keyword before value) | N-A | Not a `ze` command; `ai/rules/cli-grammar.md` governs the operator CLI |
| Editor autocomplete | No | No YANG leaf added |
| Functional test for new RPC/API | N-A | No RPC; the driving surfaces are named under Functional Tests |
| Pipe completeness | N-A | No `ze` command output |
| Env var registration | No | No new env var |
| Doctor check for runtime dependencies | No | No new runtime dependency; git is already required by the existing baseline readers |
| Prometheus counters/metrics | No | The counters this adds are published in a generated document, not exported at runtime |
| BGP family surface (new SAFI / capability / attribute) | No | No protocol surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Developer tooling only; `docs/features.md` describes the product |
| 2 | Config syntax changed? | No | None |
| 3 | CLI command added/changed? | No | No `ze` command |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | None |
| 6 | Has a user guide page? | No | The audience is contributors; see rows 10 and 15 |
| 7 | Wire format changed? | No | None |
| 8 | Plugin SDK/protocol changed? | No | None |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No requirement gains or loses proof here. The one record correction (`RFC7606-5.1-2`) restates an existing judgement in legal vocabulary and must not change what is claimed; if the re-judgement finds the claim wrong, that IS an RFC-status change and this row flips to Yes with a `docs/features/rfc-status.md` edit |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/` (or `docs/functional-tests.md` if that is where the RFC gate is described): the audit record is now validated, its coverage published, and the new `make ze-rfc-reseal` is the only writer of `rfc/audit/`. Also `docs/contributing/rfc-implementation-guide.md:516-523`, which already names `ze-rfc-check` and `ze-rfc-index` as the pair a contributor runs and must now name the third |
| 10b | New make target added? | Yes | `ai/rules/discovery-updates.md` makes a new make target a discovery obligation: add `make ze-rfc-reseal` to the `ai/INDEX.md` Dev Tools table beside `ze-rfc-check` (`:212`) and `ze-rfc-index` (`:213`), and to the RFC keyword-map row (`:372`), stating that it re-stamps `shifted` verdicts and writes nothing else. A target nobody can find is a target nobody runs, and the `shifted` red then has no remedy in practice |
| 11 | Affects daemon comparison? | No | None |
| 12 | Internal architecture changed? | No | No daemon architecture change |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `ai/RFC-REQUIREMENTS.md` is a generated inventory and gains a section; `ai/rules/hook-mapping.md` gains the new gate rows |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` and `ai/` for anchors naming `scripts/dev/rfc_requirements.py` and `.claude/hooks/pretool-writeedit.py`; update any claim about the file-level fingerprint, which becomes one of two |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/skills/ze-rfc-audit.md` shows the verdict record shape; it must show the new fields or it teaches the fleet to write records that fail the schema |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - put every new check on the entry point before it does anything
   - Tests: `TestAuditSchemaWiring`, `TestAuditDisclosureWiring`, `TestAuditRatchetWiring`, `TestAuditUpgradeGuardWiring`, `TestAuditUnitFreshnessWiring`
   - Files: `scripts/dev/rfc_requirements.py` (stub check functions called from `run_check`), `scripts/dev/rfc_requirements_test.py`
   - Verify: each wiring test drives `run_check` through `_patched` with an input the finished check must reject, and FAILS because the stub returns no error. Removing any call from `run_check` must keep it failing
2. **Phase: Validate the record** - the schema, fail closed, both directions
   - Tests: the `TestAuditSchema` class, `TestAuditCodeFingerprint`
   - Files: `scripts/dev/rfc_requirements.py`
   - Verify: AC-1 through AC-7 red then green; then run against the real tree and confirm the only failures are the 4 known rfc7606 records, which is A-2 validated by execution
3. **Phase: Migrate the one existing audit file** - 4 records, no campaign
   - Tests: `TestRFCRequirementsGate` (real tree) returns to green
   - Files: `rfc/audit/rfc7606.json`
   - Verify: the three `code` maps are transcribed from the notes' own citations (A-5); `RFC7606-5.1-2` is re-judged against `rfc/full/rfc7606.txt` rather than mapped mechanically, and if that re-judgement is anything other than `enforced`, STOP and escalate per `ai/rules/rfc-compliance.md` rather than recording a convenient value. Settled 2026-07-29: this ONE re-judgement is in scope precisely because the record is broken rather than unfavourable, and it is the only audit performed in this spec. No second requirement is judged here, whatever the reading of `RFC7606-5.1-2` turns up
4. **Phase: The tagged unit, extracted once** - shared module, hook parity held
   - Tests: `TestTaggedScopeCorpus`, `scripts/dev/hook-parity-check.py`
   - Files: `scripts/dev/rfc_tagged_scope.py`, `.claude/hooks/pretool-writeedit.py`
   - Verify: the corpus test resolves every tag in the tree (A-3); the hook golden is unchanged; the extractor never returns an empty result for a real file
5. **Phase: Four-state freshness** - the false-stale fix
   - Tests: the `TestAuditUnitFreshness` class
   - Files: `scripts/dev/rfc_requirements.py`
   - Verify: AC-14 and AC-15 red then green; the transitional no-`units` path keeps today's behavior exactly (AC-20); reconstruct the F18 commit's shape as a fixture and confirm it now reports `shifted`
6. **Phase: Mechanical re-seal** - remove the human step from the no-judgement class
   - Tests: the `TestReseal` class, `TestRenameResealDelegates`
   - Files: `scripts/dev/rfc_requirements.py`, `scripts/dev/rename_module_path.py`, `Makefile` (the new `ze-rfc-reseal` target), `ai/INDEX.md`
   - Verify: only `shifted` records change; every judgement field is byte-identical; `--check` and `--write` still write nothing under `rfc/audit/`, so `ze-rfc-reseal` is the only automated writer (A-7); the rename tool has no second copy of the rule
7. **Phase: The verdict value becomes load-bearing** - disclosure, ratchets, upgrade guard, note citation
   - Tests: `TestAuditDisclosure`, `TestAuditFindings`, `TestAuditVerdictRatchet`, `TestAuditNote`
   - Files: `scripts/dev/rfc_requirements.py`
   - Verify: AC-9 through AC-13 and AC-17 red then green; AC-10 explicitly proves a `weak` verdict passes; AC-19 explicitly proves the ratio is not ratcheted
8. **Phase: Publish the figure** - derived coverage in the ledger
   - Tests: the `TestAuditLedger` class
   - Files: `scripts/dev/rfc_requirements.py`, `ai/RFC-REQUIREMENTS.md`
   - Verify: the rendered figure reproduces the measured 44 of 974 (A-6) before any new audit is recorded, and `check_ledger_fresh` fails when an audit file is edited without regeneration
9. **Phase: Teach the producer** - the skill and the rule map
   - Tests: none mechanical; `make ze-doc-test` and `make ze-verify`
   - Files: `ai/skills/ze-rfc-audit.md`, `ai/rules/hook-mapping.md`, `docs/architecture/testing/`, `plan/deferrals/rfc-gate-regression-ratchets.md`
   - Verify: a reader following the skill produces a record that passes the schema on the first try, which is the only test that matters for a fleet

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at `file:line` and a named test |
| Feature completeness | Every user story path is a make target that actually runs the new code, not a helper reachable only from a test |
| Correctness | The four freshness states are mutually exclusive and total; no input falls through to a fifth, implicitly-fresh outcome |
| Fail-closed | Every new lookup uses an explicit presence check; no empty map, empty string, or unresolvable span reads as a legitimate answer (`ai/rules/fail-closed-guards.md`) |
| Incentive shape | Trace the cost of the honest path: recording `weak` must be green, and no new check may make reporting a finding more expensive than recording `enforced` |
| Naming | Verdict values match `ai/skills/ze-rfc-audit.md` exactly; field names are lower-kebab or the existing snake style of the file, consistently, and the skill shows the same names |
| Data flow | `--check` performs no write anywhere; `--reseal` writes only `tests` and `reaudit_history` |
| Ratchet safety | Every baseline reader distinguishes "could not look" from "nothing was there", per `_git_baseline_summary_stems` |
| Rule: `ai/rules/derive-not-hardcode.md` | No hand-maintained list of audited RFCs, verdict counts, or per-RFC coverage anywhere; all derived |
| Rule: `ai/rules/rfc-compliance.md` | Nothing in the machinery lets an `unimplemented` verdict read as an approved deviation; the skill text says to escalate |
| Registration over hardcoding | No per-RFC branch, no enumerated audit-file list; discovery is by enrolment and by directory |

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
| Input validation | `rfc/audit/*.json` is agent-authored input parsed by a build gate. Every field must be type-checked before use; a string where a map is expected must produce a clean error, never an exception trace or a silently skipped record |
| Resource exhaustion | The unit extractor reads every tagged file; it must cache per file exactly as `tagged_unit_shas` does today, and must not re-read a file per tag (366 tagged files, 2575 tags) |
| Path handling | Keys in `tests` and `code` are `file:line` strings from a JSON file and become filesystem reads. They must be treated as repo-relative and rejected if they escape the tree, since a verdict is not a trusted path source |
| Error leakage | A failure message names the rid, the field, and the file. It must not dump the whole record, which for these notes runs to thousands of characters |
| Guard cannot fail open | The one catastrophic outcome is a false fresh. Every unresolvable case must degrade toward MORE checking (file scope), never toward less |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Not applicable (Python); a syntax or import error routes to the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior; if misunderstood → RESEARCH |
| Lint failure | Fix inline (`ruff` runs on write); if architectural → DESIGN |
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
| A `wrong` or `unimplemented` verdict must be disclosed in `docs/features/rfc-status.md` | Treat disclosure as the annotation's job only | These verdicts mean the requirement is not proven, or not met. `check_status_agreement` already refuses to let a `{gap}` hide under a clean "Supported" row; a verdict saying the same thing must not be weaker than an annotation saying it. The red falls on the public claim, which is the right place, not on the auditor's honesty |
| Freshness gains a `shifted` state whose remedy is mechanical | Keep the boolean and improve the message (F18's own proposal); or drop the file-level fingerprint entirely | Improving the message leaves the human step in place, and the human step is what trains blind re-stamping. Dropping the file hash loses the one real class the unit hash misses (a helper in the same file changing under the tagged test). Three states keep both signals and route each to the cheapest correct response |
| The tagged unit is defined once, in a shared leaf module | Copy `_go_func_scopes` into the gate; or have the gate import the hook | `reseal_rfc_audits`'s own docstring names the failure: a second copy of the fingerprint rule that drifted would re-seal against a hash the gate does not compute. The gate must not depend on `.claude/`, and the hook must stay import-light, so a leaf both import is the only shape that keeps one definition |
| An empty-`tests` verdict must carry a `code` map | Time-based expiry of unfalsifiable verdicts; or ban empty-`tests` verdicts outright | Time-based staleness fires on a quiet repo and is evidence-free, which is against the grain of every other check here. Banning them would make `unimplemented` unrecordable, losing a real category. The notes already cite their producing code in prose, so requiring the citation in a machine-checkable field is transcription, and it makes the verdict falsifiable exactly when the gap might have closed |
| The ratchet is on the SET of audited requirement ids, never on the percentage | Ratchet the coverage ratio upward | A ratio ratchet fails the build for adding a tagged test, punishing coverage work. The set ratchet has no such perverse case, and the ratio remains useful as a published figure |
| Migration of the existing audit file is 4 records, and `RFC7606-5.1-2` is re-judged rather than mapped (settled 2026-07-29) | Mechanically rewrite `implemented` to `enforced`; or declare the re-judgement out of scope as "audit work" and leave the record illegal | The value is illegal, so this is a BROKEN RECORD, not an unfavourable finding: no legal verdict exists to preserve, which is why repairing it is schema work and not the start of the 930-verdict drain. Which legal value replaces it is still a judgement about whether nine tests enforce the requirement, so rewriting it blind is precisely the fabrication this machinery exists to prevent, and `ai/rules/rfc-compliance.md` makes the void-answer discipline explicit. Leaving it illegal was rejected too: the schema this spec writes would reject the only audit file in the tree, so the gate could not be armed at all |
| A DEDICATED `make ze-rfc-reseal` is the only writer of `rfc/audit/`; `make ze-rfc-check` stays read-only and `make ze-rfc-index` stays confined to the ledger | Auto-reseal inside the check; or fold the re-seal into `ze-rfc-index`, which was this spec's earlier position and is superseded | A check that writes cannot be trusted to report, and a regen target that writes evidence is the same failure one step removed. `ze-rfc-index` is run ROUTINELY: it is required after any tag move, and its freshness variant `run_check_fresh` (`:1725`) is reached from `make ze-doc-test` through `mk/inventory.mk:106`, so developers meet it for reasons that have nothing to do with an audit. Folding the re-seal into it would silently re-stamp hand-authored evidence during unrelated work, automating the exact blind re-stamp reflex this spec exists to remove. A dedicated target makes every write to an evidence file intentional and greppable. Cost: one make target, one discovery entry, and one extra command in the `shifted` remedy (R-8). Owner ruling 2026-07-29, A-7 |

## Known Limitations

- **No gate can prove a human read the RFC.** Everything here raises the cost of a false `enforced` and lowers the cost of an honest finding; none of it is proof of reading. Sampling by a second reader remains the only real check, and this spec makes sampling possible (coverage is published per RFC) without making it automatic.
- **The unit fingerprint cannot follow a call.** An assertion moved into a helper outside the tagged function is not covered by the unit hash. This is the same documented limit the edit-time hook carries (`.claude/hooks/pretool-writeedit.py:1714`), and it is precisely why the file-level hash is retained as the `shifted` signal rather than deleted.
- **The 930 outstanding audits are not performed here**, and no verdict is generated for them. That is deliberate and is the half of the void 2026-07-20 deferral row that was correct on the engineering. The follow-on fleet work belongs to the `rfcgate` umbrella. The single exception, settled 2026-07-29, is `RFC7606-5.1-2`, whose recorded value is illegal and therefore unrepairable without reading. Repairing a broken record is schema work; judging an absent one is the drain, and the drain does not start here.
- **`unimplemented` verdicts remain judgements about code that only a reader can make.** The `code` map makes them falsifiable when the code moves; it does not make them right. Under `ai/rules/rfc-compliance.md` every one of them is void as authority and is a question for Thomas, which the skill text must say and no gate can enforce.

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
- [ ] `make ze-verify` passes (the pre-commit gate; `ai/rules/git-safety.md`)
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
