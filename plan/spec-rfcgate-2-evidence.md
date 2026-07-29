# Spec: rfcgate-2 -- wire-level evidence for RFC requirements

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | spec-rfcgate-1-extraction |
| Phase | 5/6 |
| Deferral shard | `plan/deferrals/rfcgate-2-evidence.md` |
| Updated | 2026-07-29 |

Part of the `rfcgate` spec set. Umbrella: `plan/spec-rfcgate-0-umbrella.md`.
Sibling: `plan/spec-rfcgate-1-extraction.md` (requirement extraction completeness),
which this spec depends on: extraction decides *which* obligations exist, this
spec decides *what kind of evidence* is allowed to prove one.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze binds RFC MUST-level requirements to tests with an `RFC requirement: <ID>
<polarity>` tag, and `make ze-rfc-check` treats a tag as proof. Today the entire
evidence base is in-process:

| Evidence carrier | Tag lines | Share |
|------------------|-----------|-------|
| Go unit tests (`*_test.go`) | 2582 | ~99.8% |
| Functional `.ci` tests | 4 | ~0.2% |
| Interop scenarios | 0 | 0% |
| Editor `.et` tests | 0 | 0% |

(Grep-verified over `internal/ pkg/ test/`; 2575 of those tags resolve to a known
requirement id, 2571 of them from `*_test.go`.)

So Ze's public compliance claims rest almost entirely on evidence that a peer
never sees. `ai/rules/interop-and-goal-validation.md` requires an interop test for
protocol behaviour, and `ai/rules/integration-completeness.md` states plainly that
a unit test proves the algorithm while only a functional test proves the feature is
reachable. A MUST about wire behaviour proven only by a Go table test is proven at
the wrong altitude.

The goal is **not** "tag interop scenarios". Interop currently runs in no
automated pipeline at all, and its runner fails open when Docker is missing, so a
tag placed there today would satisfy the gate with evidence nothing ever executes
-- strictly worse than the unit binding it replaced, which at least runs on every
push. The goal is therefore ordered:

1. Make non-unit evidence **execute** in an automated pipeline, fail-closed.
2. Only then let the scanner **see** it.
3. Make the ledger **show** how strong each piece of evidence is, rather than
   flattening a nightly-advisory scenario and a merge-gate unit test into one
   "proven" cell.
4. Ratchet the amount of non-unit evidence so it can only rise.
5. Prove the whole chain with a seed set of real bindings, `.ci`-first.

Estimated size of the eventual target: a keyword classifier over requirement text
(hand-calibrated on a 40-item sample, ~97% precision, poor recall) puts **at least
76%** of the 2720 gated MUSTs in the wire-visible class, sample-implied 85-90%.
That number sizes the problem and appears nowhere in a gate: it is a planning
input only, and this spec forbids deriving any pass/fail decision from it (see
Known Limitations, A-4).

## Required Reading

### Architecture Docs
- [ ] `ai/rules/interop-and-goal-validation.md` - what interop must prove, and the vacuity traps
  → Constraint: a passing interop/functional test is evidence ONLY if it goes RED when the behaviour is reverted; adding a test to already-working code never had a red phase, so its discrimination is unproven until forced.
  → Decision: when a receiver must accept both forms (RFC 7606 §5.1 shape), the interop test proves ACCEPTANCE, not correctness of the specific form, and the spec must say which.
- [ ] `ai/rules/integration-completeness.md` - unit vs `.ci` division of proof
  → Constraint: "a unit test is NOT a substitute for a `.ci` test"; both are required for a user-facing feature.
- [ ] `ai/rules/functional-test-gate.md` - which functional test type each change needs
  → Constraint: mutation-verify -- disable the producing function, confirm the test flips RED, revert. A test that exists is not a test that gates.
- [ ] `ai/rules/testing.md` - RFC-tagged tests, ledger regeneration, back-fill obligation
  → Constraint: any commit that adds, moves, deletes or re-tags a tagged test MUST regenerate `ai/RFC-REQUIREMENTS.md` in the SAME commit; `ze-rfc-check` fails on a stale ledger.
  → Constraint (Back-Fill New Test Types): introducing a new evidence kind obliges naming the applicable set and either back-filling it or recording the remainder as tracked backlog.
- [ ] `ai/rules/fail-closed-guards.md` - guards deny on a miss, or say something
  → Decision: an unrecognised evidence carrier must be refused loudly, never silently skipped -- which is exactly today's defect (`scan_tree` skips every non-`.go`/`.ci` file with no warning).
- [ ] `ai/rules/derive-not-hardcode.md` - enumerated data comes from one source
  → Constraint: the evidence-kind/tier mapping is declared once and derived everywhere (scanner, ledger, ratchet), never re-spelled per consumer.
- [ ] `ai/rules/spec-no-code.md` - tables and prose only
  → Constraint: this spec carries no code snippets in any language.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7606.md` - the only RFC with non-unit evidence today (the 4 `.ci` tags)
  → Constraint: §5.1 obliges receivers to accept an UPDATE mixing Withdrawn Routes with NLRI while forbidding senders to emit one, so no conforming peer can drive it -- this is the canonical "wire-visible but not peer-drivable" shape, and it is why a raw injector, not a peer daemon, is the right carrier for that class.

**Key insights:** (minimal context to resume after compaction)
- The hazard is ordering, not tagging: execution must land before the scanner can see the carrier, or the ratchet measures growth of unexecuted evidence.
- The scanner has TWO independent extension filters (tree scan and HEAD baseline). Editing one and not the other silently corrupts the coverage ratchet.
- `.ci` and `.et` share `terminator=` block semantics; `check.py` has no such concept and must never inherit it.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `scripts/dev/rfc_requirements.py` - the whole tag pipeline.
  - `TEST_ROOTS` (`:59`) = `internal`, `pkg`, `test`. Interop scenarios already live under a scanned root; only the extension filter excludes them.
  - `scan_tree` (`:540-566`) recognises exactly two shapes: `*_test.go` → `scan_go_tags` (`:492`), `*.ci` → `scan_ci_tags` (`:510`). Every other file is skipped with no warning and no counter.
  - `scan_ci_tags` (`:510-537`) matches `_CI_TAG_RE` (`:149`, `^#\s*RFC requirement:`) at line start after strip, and skips the body of any `terminator=` block (`:519-525`) because that content is raw file content, not `.ci` syntax.
  - `_git_baseline_tag_polarities` (`:797-855`) re-derives the HEAD baseline and applies its OWN extension filter at `:835` (`rel.endswith("_test.go") or rel.endswith(".ci")`), plus a mirrored prune at `:845`.
  - `_scan_tags_tolerant` (`:858-896`) picks the scanner by extension at `:879` and deliberately refuses a per-line fallback for non-Go files (`:884`), because a line-position-insensitive rescan of a `terminator=` format invents phantom tags.
  - `check_coverage_ratchet` (`:955-1004`) compares per-requirement polarity SETS against that baseline. Evidence kind is not part of the comparison, so a `.ci` tag replaced by a unit tag of the same polarity is invisible.
  - `render_ledger` (`:1465-1529`) emits `| Requirement | Level | § | Positive test | Negative test | Note |`. The test cells carry `file:line` only; nothing states what kind of test it is or whether anything runs it.
  - `rfc_coverage` / `_render_rollup` (`:1371-1462`) count `both` / `one` / `annotated` / `missing` per RFC, again with no notion of evidence kind.
- [ ] `test/interop/run.py` - BGP interop runner. `:121-134` probes `docker info` and, when unavailable, prints "Docker unavailable, skipping interop tests" and `sys.exit(0)` -- **fails open**. `:149-168` walks `test/interop/scenarios/*/`, skips any directory without `check.py` (`:158-161`), and runs `Scenario.setup()` then `Scenario.run_check()`.
- [ ] `test/ipsec-interop/run.py` - the same probe at `:110-120` exits **1**. Two sibling runners, opposite failure polarity, in the same repo.
- [ ] `test/interop/interop.py` - `Scenario.run_check` (`:1468-1488`) loads `check.py` through `importlib` and calls its module-level `check()`. Scenario dirs may be rendered into a temp copy (`:1463-1466`), but the source of truth on disk is the scenario directory. `:1284-1309` starts a raw BGP injector sidecar (`ze-test peer --decode` against a scenario-supplied `inject.msg`) whose comment names RFC 7606 §5.1 as its reason for existing.
- [ ] `scripts/status/verify_run.go` - `stagesForMode` (`:224-290`) is the single source of truth for `ze-verify` / `ze-verify-changed`, and the header comment (`:210-213`) states outright: "a gate that is not listed here runs NOWHERE, in CI or locally." Neither branch lists any interop target. `ze-rfc-check` IS listed in both (`:237`, `:259`).
- [ ] `mk/test-functional.mk` - `ze-functional-test` (`:187-190`) runs the suite list `encode plugin parse decode reload ui editor managed ...`. **`editor` is in it**, so `.et` tests execute inside a `ze-verify` stage, exactly like `.ci`.
- [ ] `mk/test-integration.mk` - `ze-interop-test` (`:35-37`) shells to `test/interop/run.py`. Its only automated caller is `ze-release-evidence` (`mk/test-release.mk:152-153`) through `run_if_docker`, a manual release-time target.
- [ ] `internal/test/runner/parsing.go` - `parseCIFile` (`:264-268`) reads a `.ci` through `tmpfs.ReadFrom` before any directive parsing, which is the real-parser behaviour `scan_ci_tags` models. This is the producer behind the terminator rule, and it has no analogue in Python.
- [ ] `scripts/dev/github_workflows_test.go` - pins `evidence-nightly.yml`'s shape: scheduled-only (`:279-291`), must run `ze-fuzz-test` and `ze-integration-test` by target name (`:303-314`), must not run `ze-qemu-integration-test` (`:310-313`), and **every** job must carry `continue-on-error: true` (`:316-325`). A new job inherits the advisory requirement automatically.

**Verified counts (this tree):**

| Fact | Value | How verified |
|------|-------|--------------|
| BGP interop scenario dirs | 104 | `ls -d test/interop/scenarios/*/` |
| IPsec interop scenario dirs | 10 | `ls -d test/ipsec-interop/scenarios/*/` (the umbrella's working note says 11; 10 is what this tree holds) |
| L2TP / PPPoE interop scenario dirs | 3 / 1 | same |
| `.et` editor tests | 164, of which 163 use `terminator=` | `find test/editor -name '*.et'`, grep |
| Existing `RFC requirement:` tags in any `.py` or `.et` | 0 | grep |
| Workflow files mentioning interop | 0 | grep over `.github/workflows/` |
| Per-scenario metadata file of any kind | none exists | scenario dirs hold `ze.conf`, peer conf, `check.py`, optional sidecar inputs |

**Behavior to preserve:**
- Every currently-resolving tag keeps resolving, at the same `file:line`, with the same polarity. This change adds carriers; it retires none.
- `scan_ci_tags`'s `terminator=` handling for `.ci` is unchanged.
- `_scan_tags_tolerant`'s refusal to run a per-line Go fallback over a non-Go file is unchanged.
- The ledger stays byte-stable for a given tree (the freshness gate depends on it).
- `ze-rfc-check` stays in both `stagesForMode` branches and keeps its current runtime budget within noise (~~today ~1.7s at HEAD, ~2.2s with the baseline read~~ -- corrected 2026-07-29 to the measured figures; see the Security Review Checklist's "Resource exhaustion" row for who measured what).

**Behavior to change:**
- `test/interop/run.py` stops fail-open on missing Docker.
- The tag scanner recognises interop `check.py` and `.et`, in both filters.
- The ledger gains an evidence-kind/tier column and a nightly-only marker.
- A new monotonic ratchet on non-unit evidence counts.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A human or agent writes an `RFC requirement: <ID> <polarity>` comment into a test file. The comment is the only input; there is no registry, no metadata file, no side channel.

### Transformation Path
1. `scan_tree` walks `internal/`, `pkg/`, `test/` and dispatches each file to a scanner by extension (today: two; after this spec: four carriers across three scanner behaviours).
2. Each scanner yields `Tag(rid, polarity, file, line)`.
3. `evaluate` joins tags to requirements parsed from `rfc/short/*.md` and reports coverage violations.
4. `_git_baseline_tag_polarities` independently re-derives the same join at HEAD, through its own extension filter, and `check_coverage_ratchet` diffs the two.
5. `render_ledger` writes `ai/RFC-REQUIREMENTS.md`; `check_ledger_fresh` fails when the committed file differs from the render.
6. `make ze-rfc-check` runs the whole set as a `ze-verify` stage.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Tag text ↔ scanner | line-oriented comment match, per-carrier semantics | Yes -- `_CI_TAG_RE:149`, `_GO_TAG_RE:148` read |
| Working tree ↔ git HEAD | second, independent extension filter at `:835` | Yes -- read; this is the two-filter hazard |
| Scanner ↔ ledger render | `Tag.file` suffix is the only signal of evidence kind available downstream | Yes -- `render_ledger:1515-1521` uses `t.file` verbatim |
| Ledger ↔ CI | `ze-rfc-check` in `stagesForMode`; the evidence itself in a separate pipeline | Yes -- `verify_run.go:224-290` read |
| Make target ↔ workflow | `evidence-nightly.yml` invokes make targets by name; `github_workflows_test.go` pins which | Yes |

### Integration Points
- `scripts/dev/rfc_requirements.py` -- scanner, baseline, ratchet, ledger render.
- `test/interop/run.py` -- Docker probe polarity.
- `.github/workflows/evidence-nightly.yml` -- new advisory job.
- `scripts/dev/github_workflows_test.go` -- pins the new job's presence and advisory status.
- `ai/RFC-REQUIREMENTS.md` -- generated output shape.
- `ai/rules/testing.md`, `ai/rules/rfc-compliance.md`, `docs/features/rfc-status.md` -- the discovery surfaces that must describe the new evidence kinds.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Tags stay the only input; no per-scenario metadata file is introduced, so no second source of truth appears beside the test text |
| No unintended coupling (components stay isolated) | Yes | The scanner learns file shapes, not scenario semantics; interop keeps its own runner and its own pipeline |
| No duplicated functionality (extends existing, does not recreate) | Yes | `.et` reuses `scan_ci_tags` rather than gaining a third terminator implementation; only `check.py` needs new scanning behaviour |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Python tooling, no wire path |
| Registration over hardcoding | Yes | One declared carrier table (extension → scanner → executing target → tier) drives scan, baseline, ledger and ratchet; no consumer re-spells the list (`ai/rules/derive-not-hardcode.md`) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `.et` tests execute inside a `ze-verify` stage, so `.et` evidence is verify-tier | `mk/test-functional.mk:190` lists `editor` in `ze-functional-test`'s suite list; `ze-functional-test` is in both `stagesForMode` branches (`verify_run.go:252`, `:277`) | `.et` would be nightly-or-worse tier and must not be introduced as verify-tier | Read both files; re-confirm during Phase 3 by asserting the tier table against `stagesForMode`'s golden | confirmed (design-time read) |
| A-2 | `check.py` is plain Python with `#` line comments and no `terminator=`-like construct | `Scenario.run_check` loads it via `importlib` (`interop.py:1468-1488`); sampled `test/interop/scenarios/ospf-auth-frr/check.py:1-11` | A naive line scanner would mis-scope tags | Tokenizer-based scanner + a fixture with a tag-shaped line inside a docstring and inside a string literal | unvalidated |
| A-3 | The two extension filters (`scan_tree:558-563`, baseline `:835`) are the only places a carrier list is spelled | Read both, plus `_scan_tags_tolerant:879` which dispatches by suffix (a third spelling) | A missed spelling silently corrupts the ratchet in the direction of false green | grep the module for every `endswith` on a test-file suffix during Phase 2; unit test asserting tree and baseline agree on a fixture tree | unvalidated |
| A-4 | The 76%/85-90% wire-visible estimate is a sizing input and never a gate input | Owner statement; classifier has ~97% precision but poor recall | A gate built on it would mis-classify a large minority of requirements and manufacture false obligations | Deliberate absence: no AC references the classifier; Deliverables check greps the implementation for any use of it | unvalidated |
| A-5 | Adding a job to `evidence-nightly.yml` needs no change to its scheduled-only / advisory pins beyond adding the new target assertion | `github_workflows_test.go:279-325` -- the advisory check iterates ALL jobs, the target check is an inclusion list | The workflow test fails or, worse, passes vacuously | Run `go test` on that file after the edit; assert the new job appears in the parsed job list | unvalidated |
| A-6 | No existing `check.py` or `.et` contains a line that would parse as a tag today | grep: 0 matches for `RFC requirement:` in `test/**/*.py` and `test/**/*.et` | The scanner extension would import tags nobody authored, and the first ratchet baseline would be wrong | Re-run the grep at the start of Phase 2, before the scanner change lands | confirmed (design-time grep) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A tag bound to a nightly-advisory test is weaker than one bound to a verify-stage test, and a single "proven" cell hides the difference | A requirement's only evidence is an interop scenario, yet the ledger reads identically to a unit-proven one | The ledger carries an explicit tier per test link AND a `nightly-only` marker on any requirement with no verify-tier evidence; the ratchet keeps verify-tier and nightly-tier counts as SEPARATE monotonic counters so nightly evidence can never be substituted to satisfy the verify-tier ratchet (AC-8, AC-11) |
| R-2 | Receiver-side MUSTs no conforming peer can drive (RFC 7606 §5.1 shape) look wire-visible but are untestable by a peer daemon | A scenario author cannot make FRR/BIRD emit the offending message | The injector sidecar already exists (`interop.py:1284-1309`, used by `test/interop/scenarios/47-rfc7606-relay-shape-frr`); the seed bindings must include one injector-driven requirement so the pattern is documented rather than rediscovered (~~AC-13~~ **AC-16**; corrected 2026-07-29, the ACs were renumbered after this row was written and AC-13 is now the ratchet-loss criterion) |
| R-3 | Vacuity: a `.ci` or interop binding satisfies `ze-rfc-check` whether or not the test discriminates (`ai/rules/interop-and-goal-validation.md`) | A seed binding passes on first run and was never seen red | Every seed binding is mutation-verified: disable the producing function, confirm RED, revert, confirm GREEN, and record the RED output in the closure Goal Validation (~~AC-14~~ **AC-17**; corrected 2026-07-29, same renumbering -- AC-14 is now the nightly-substitution criterion). No binding lands without a recorded red |
| R-4 | Phase ordering is violated under time pressure, landing the scanner before the pipeline | A `check.py` tag resolves while `ze-interop-test` still exits 0 on a Docker-less host | The tier table refuses a tag in a carrier whose executing target has no automated caller (AC-7); ACs are numbered so the phase-1 gates are prerequisites of phase-2 ACs |
| R-5 | The three other interop trees (ipsec 10, l2tp 3, pppoe 1) stay unrun, so a binding there would be invisible evidence | Someone tags an ipsec `check.py` and the gate accepts it | Those trees classify as `unrun` in the tier table and a tag in them is a hard error naming the fix (add the tree to nightly CI first); recorded as a deferral row, not silently allowed |
| R-6 | The Python tokenizer refuses a syntactically invalid `check.py` and the scan fails on an unrelated file | `ze-rfc-check` reds with a tokenize error | Fail closed with a named file and a clear message in the tree scan (consistent with the module's posture); the BASELINE path contributes no tags for that file, mirroring the existing `.ci` decision at `:884` |
| R-7 | The nightly interop job is red for unrelated Docker/image reasons and its advisory status trains everyone to ignore it | The job is red for consecutive nightlies | Out of scope to fix here, but the ledger's nightly-only marker means a persistently red nightly is visible as *weak evidence*, not as proof; the umbrella tracks promoting the job to blocking once a green baseline exists |
| R-8 | Ledger churn: adding a column rewrites every row, colliding with concurrent RFC work | A large `ai/RFC-REQUIREMENTS.md` diff conflicts on rebase | Land the ledger shape change (Phase 3) as its own commit, regenerate rather than hand-merge, and re-run `make ze-rfc-index` after any rebase |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | No runtime/user-visible behaviour: this is test-evidence tooling. The failure mode is epistemic -- a wrong tier mapping or a one-sided filter edit makes `ze-rfc-check` report proof that does not exist, which is worse than a red gate because it is silent. Secondary: `ze-verify` reds for everyone if the scanner raises on a file it previously skipped. |
| How is it reverted? | Single commit revert per phase. Phase 1 (runner + workflow) and Phase 2-4 (scanner/ledger/ratchet) are independently revertible. Seed bindings (Phase 5) revert by deleting comment lines and regenerating the ledger. The ratchet baseline is derived from HEAD, so a revert relaxes it automatically. |
| Who else touches this path? | `plan/spec-rfcgate-1-extraction.md` edits the same module's summary-parsing half (different functions, same file -- expect textual conflicts, not semantic ones). Any spec adding or moving an RFC-tagged test touches `ai/RFC-REQUIREMENTS.md`. **Cross-child sequencing, so a red gate from a sibling is not a surprise:** `plan/spec-rfcgate-4-ledger.md` (OC-1) arms a `check_unproven_support` gate inside the same `run_check` this spec extends, and that gate is RED on four stems (rfc1035, rfc3765, rfc4486, rfc5301) until they are resolved. `ze-rfc-check` runs in both verify modes, and `commit_helper.py create` refuses a script over a non-FRESH verify (`ai/rules/git-safety.md`, Step 1), so an armed-but-red gate would block EVERY commit in the repository, including the ones fixing it. ~~`ai/rules/git-safety.md` makes a structural red unbypassable.~~ (Corrected 2026-07-29: `ze-rfc-check` is NOT in `STRUCTURAL_GATES`, `scripts/dev/commit_helper.py:512-523`, so the red is `--unverified`-bypassable in mechanism. It is still not a legal route: `--unverified` covers a flaky or environmental red or one already logged, and `ai/rules/fix-dont-record.md` bans a `plan/known-failures/` shard for a deterministic reproducible failure. The conclusion is unchanged.) Sibling 4 therefore arms it no earlier than the commit resolving all four stems; before that it may only be called with an inert body. Nothing in this spec depends on that gate, and this spec must not arm it, work around it, or read a `check_unproven_support` red as its own defect. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A developer runs `make ze-interop-test` on a host without Docker | → | the Docker probe in `test/interop/run.py` | `TestInteropRunnerFailsClosedWithoutDocker` (Go test shelling to the runner with a PATH lacking docker, asserting non-zero exit) |
| The nightly schedule fires | → | the interop job in `evidence-nightly.yml` | `TestEvidenceNightlyRunsInterop` in `scripts/dev/github_workflows_test.go` (target present by name, job advisory) |
| A tag is written into an interop `check.py` | → | the Python carrier scanner in `rfc_requirements.py` | `test_scan_python_tags_found` (selftest fixture: tag resolves with correct `file:line` and polarity) |
| A tag is written into an `.et` file | → | `scan_ci_tags` reused for `.et` | `test_scan_et_reuses_ci_semantics` (tag outside a `terminator=` block resolves; one inside it does not) |
| `make ze-rfc-index` runs | → | the evidence-kind column in `render_ledger` | `test_ledger_row_carries_evidence_tier` (rendered row for a `.ci`-proven requirement shows the functional/verify tier) |
| `make ze-rfc-check` runs after non-unit evidence is deleted | → | the non-unit evidence ratchet | `test_non_unit_ratchet_fires_on_loss` (baseline has a `.ci` tag, tree does not, check returns an error naming the requirement) |
| A tag is written into an `unrun` carrier (`test/ipsec-interop/**/check.py`) | → | the tier table's fail-closed branch | `test_tag_in_unrun_carrier_is_refused` (error names the carrier and the required fix) |
| An RFC MUST is bound to a real `.ci` | → | the seed binding | `make ze-rfc-check` resolves it; the `.ci` itself is mutation-verified RED |

## Acceptance Criteria

Ordered. **AC-1..AC-3 are prerequisites of AC-4 onward**: no scanner change may
land while a carrier's executing target can pass without running.

**Phase 1 -- execution before evidence**

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `test/interop/run.py` runs on a host where `docker info` fails or docker is absent | Exits non-zero with a message naming Docker as the missing prerequisite, matching `test/ipsec-interop/run.py:110-120`. It never prints "skipping" and exits 0 |
| AC-2 | The nightly schedule fires | `evidence-nightly.yml` runs `make ze-interop-test` in its own job, `continue-on-error: true`, on a Docker-capable runner |
| AC-3 | `go test` runs over `scripts/dev/github_workflows_test.go` | A test asserts the interop target is present in `evidence-nightly.yml` by name and its job is advisory; the existing scheduled-only and fuzz/integration assertions still pass |

**Phase 2 -- the scanner sees the new carriers**

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-4 | A conforming tag comment sits at line start in `test/interop/scenarios/<s>/check.py` | `scan_tree` yields it with the correct requirement id, polarity, relative path and line number |
| AC-5 | A tag-shaped line sits inside a Python docstring or string literal in a `check.py` | It is NOT reported as a tag (only genuine comment tokens count) |
| AC-6 | A conforming tag sits in an `.et` file outside any `terminator=` block, and a second tag-shaped line sits inside one | The first resolves, the second does not: `.et` reuses `.ci` terminator semantics rather than a new implementation |
| AC-7 | A tag is written into a carrier whose executing target has no automated caller (`test/ipsec-interop/`, `test/l2tp-interop/`, `test/pppoe-interop/`) | `make ze-rfc-check` fails, naming the file and stating that the carrier's suite must be added to an automated pipeline before it can carry evidence |
| AC-8 | The working tree and the git HEAD baseline are scanned over the same fixture tree containing every carrier kind | Both produce the same tag set: the tree filter and the baseline filter agree, and a test fails if either is extended alone |
| AC-9 | A `check.py` the Python **tokenizer** cannot read (unterminated string, bad indentation, stray character) sits anywhere under a scanned root | The tree scan fails closed with an error naming the file; the baseline path contributes no tags for that file and does not crash. ~~"does not parse as Python"~~ (corrected 2026-07-29: the guard is at TOKENIZE level, which is the CORRECT level, because a comment is lexical -- `tokenize.generate_tokens` at `rfc_requirements.py:548`. A file that tokenizes but is not valid Python 3, e.g. a Python-2 `print 'x'` statement, IS scanned and its tags ARE collected; verified empirically. The AC text, not the behaviour, was wrong: demanding a full parse would refuse files whose comments are perfectly readable, and comment extraction is exactly what this scanner needs) |

**Phase 3 -- the ledger shows evidence strength**

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-10 | `make ze-rfc-index` regenerates the ledger | Every test link carries its evidence kind (unit / functional / editor / interop) and its execution tier (verify / nightly), derived from one declared carrier table |
| AC-11 | A requirement's only evidence is nightly-tier | Its ledger row carries an explicit `nightly-only` marker, and the per-RFC rollup counts nightly-only requirements in their own column, so nightly evidence is never presented as merge-gate evidence |
| AC-12 | The ledger is regenerated twice on an unchanged tree | Byte-identical output; `check_ledger_fresh` stays a meaningful gate |

**Phase 4 -- non-unit evidence can only rise**

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-13 | A commit removes the last non-unit tag from a requirement that had one at HEAD | `make ze-rfc-check` fails, naming the requirement and the lost evidence kind, and no annotation satisfies it |
| AC-14 | A commit converts a verify-tier non-unit tag into a nightly-tier one | The check still fails: verify-tier and nightly-tier counts ratchet independently, so nightly evidence cannot be substituted for verify evidence |
| AC-15 | A commit adds non-unit evidence | The check passes and the new counts become the baseline for the next commit |

**Phase 5 -- the chain is proven end to end**

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-16 | The seed binding set is in place | At least one gated MUST is proven by a `.ci` newly bound in this spec, at least one by an interop `check.py`, and at least one by an injector-driven interop scenario (the RFC 7606 §5.1 receiver-side shape). The seed set carries NO `.et` binding: that is a settled N-A (2026-07-29), not an unmet target, because no editor-visible RFC obligation exists to bind and inventing one would be a test written for the gate. `.et` support itself is still proven, by AC-6 (scanner) and AC-10 (ledger tier), which is where the carrier's correctness belongs |
| AC-17 | Each seed binding's producing function is disabled | The bound test flips RED; restoring it flips GREEN. The RED output is recorded in Goal Validation. A binding with no recorded RED does not land |
| AC-18 | A behaviour is reachable from both a `.ci` and an interop scenario | The seed binding is placed on the `.ci`, and the choice is stated: `.ci` runs on every push inside `ze-verify`, interop is nightly-advisory |

## End-to-End User Stories

The "user" here is a maintainer or agent reading Ze's compliance evidence.

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Asks "is this MUST proven on the wire?" | reads `ai/RFC-REQUIREMENTS.md` row → sees evidence kind + tier per test link | `test_ledger_row_carries_evidence_tier` |
| 2 | Asks "is this proof actually executed on every merge?" | reads the tier cell; `nightly-only` marker present or absent | `test_nightly_only_marker_rendered` |
| 3 | Binds a new MUST to an interop scenario | writes the tag comment in `check.py` → `make ze-rfc-index` → row appears with interop/nightly tier | `test_scan_python_tags_found` + a real seed binding |
| 4 | Deletes a wire-level test | `make ze-rfc-check` refuses the commit, naming the requirement and the lost kind | `test_non_unit_ratchet_fires_on_loss` |
| 5 | Runs the interop suite on a laptop without Docker | runner exits non-zero and says why, instead of reporting success | `TestInteropRunnerFailsClosedWithoutDocker` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_scan_python_tags_found` | `scripts/dev/rfc_requirements_test.py` | AC-4: a comment tag in `check.py` resolves with correct id/polarity/line | |
| `test_scan_python_tags_ignores_string_literals` | same | AC-5: tag-shaped text in a docstring or string is not a tag | |
| `test_scan_python_tags_rejects_invalid_syntax` | same | AC-9: unparseable Python fails closed with the file named | |
| `test_scan_et_reuses_ci_semantics` | same | AC-6: `.et` honours `terminator=` blocks, no third implementation | |
| `test_tree_and_baseline_filters_agree` | same | AC-8: both filters accept the same carrier set over a fixture tree | |
| `test_tag_in_unrun_carrier_is_refused` | same | AC-7: fail-closed on a carrier nothing executes | |
| `test_carrier_table_is_single_source` | same | Every consumer (scan, baseline, tolerant scan, ledger, ratchet) reads the one declared table | |
| `test_ledger_row_carries_evidence_tier` | same | AC-10 | |
| `test_nightly_only_marker_rendered` | same | AC-11 | |
| `test_ledger_render_is_stable` | same | AC-12: two renders byte-identical | |
| `test_non_unit_ratchet_fires_on_loss` | same | AC-13 | |
| `test_verify_tier_ratchet_rejects_nightly_substitution` | same | AC-14 | |
| `test_non_unit_ratchet_accepts_growth` | same | AC-15 | |
| `TestInteropRunnerFailsClosedWithoutDocker` | `test/interop/run_test.go` (new) or an existing runner-adjacent Go test | AC-1 | |
| `TestEvidenceNightlyRunsInterop` | `scripts/dev/github_workflows_test.go` | AC-2, AC-3 | |

Python tests live beside the tool as `*_test.py` and are picked up by
`TestPythonUnitTests` (`scripts/dev/python_tests_test.go`), per `ai/rules/testing.md`.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| non-unit evidence count (ratchet) | 0..N | baseline value | baseline-1 (must fail) | baseline+1 (must pass) |
| verify-tier non-unit count | 0..N | baseline value | baseline-1 (must fail) | baseline+1 (must pass) |
| tag line number | 1..len(file) | last line of file | line 0 (never emitted) | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| seed `.ci` binding | `test/plugin/*.ci` (existing file, new tag) | a gated MUST is proven by a test the user's daemon actually runs | |
| `.et` seed binding | `test/editor/**/*.et` | **N-A, settled 2026-07-29, not pending.** No seed binding is placed on an `.et`. Supporting `.et` as a carrier is this spec's deliverable and it is delivered (scanner, ledger tier, ratchet); what does not exist is a genuinely editor-visible RFC obligation to bind. Manufacturing one purely to exercise the plumbing would be a test written for the gate rather than for the requirement, which is the vacuity trap this spec already warns about and which `ai/rules/interop-and-goal-validation.md` names outright | settled N-A |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| existing peer-driven scenario (seed binding) | `test/interop/scenarios/` | FRR or BIRD | a wire-visible MUST is accepted/enforced by a real peer, not just by our own parser | |
| `47-rfc7606-relay-shape-frr` (seed binding) | `test/interop/scenarios/` | FRR + injector sidecar | the receiver-side §5.1 shape no conforming peer can emit, driven by `ze-test peer` against `inject.msg` | |

## Files to Modify
- `scripts/dev/rfc_requirements.py` - carrier table; Python carrier scanner; `.et` routed to `scan_ci_tags`; both extension filters (`:558-563` and `:835`) driven from the table; `_scan_tags_tolerant` dispatch; evidence tier in `render_ledger` and `_render_rollup`; new non-unit ratchet beside `check_coverage_ratchet`.
- `test/interop/run.py` - Docker probe exits non-zero (`:132-134`).
- `.github/workflows/evidence-nightly.yml` - new advisory interop job.
- `scripts/dev/github_workflows_test.go` - pin the new job.
- `ai/RFC-REQUIREMENTS.md` - regenerated (do not hand-edit).
- `rfc/audit/rfc7606.json` - **added 2026-07-29, not foreseen at design time.** Hand re-stamped, and it is the ONE file in this spec that must be. `verdict_is_fresh` (`scripts/dev/rfc_requirements.py:1640-1642`) is exact equality on the whole `tests` map, so this spec's own new `.ci` tags on `RFC7606-5.1-2` and `RFC7606-5.1-3` changed that map and staled verdicts whose requirement text and assertions were untouched. Nothing about what any test asserts changed. This is fresh evidence for `plan/spec-rfcgate-3-audit-teeth.md`, which already counts five prior mechanical re-stamps of this same file (`:72-80`) and files the pattern as F18 in `plan/learned/HOOK-FRICTION.md:716`; this spec makes it six.
- `ai/rules/testing.md` - RFC-tagged-test section: which carriers may hold a tag, and the verify/nightly tier distinction.
- `ai/rules/rfc-compliance.md` - the four ratchets table gains the non-unit evidence ratchet.
- `docs/features/rfc-status.md` - state that a status row backed only by nightly-tier evidence says so.
- `docs/functional-tests.md` - interop runner now fails closed; interop runs nightly.
- `test/plugin/*.ci`, `test/interop/scenarios/*/check.py` - seed tag comments only.

## Files to Create
- `scripts/dev/rfc_requirements_test.py` - unit tests above (if the module has no sibling test file yet; otherwise extend it).
- `test/interop/run_test.go` - fail-closed runner test (if no suitable Go home exists).
- `plan/deferrals/rfcgate-2-evidence.md` - deferral shard (ipsec/l2tp/pppoe trees, back-fill remainder).
- **Destination specs for the shard's live rows (added 2026-07-29, `ai/rules/deferral-tracking.md` requires each to exist on disk):**
  - `plan/spec-rfcgate-2-deferred-unrun-interop-trees.md` - rows 1-3.
  - `plan/spec-rfcgate-2-deferred-nonunit-evidence-backfill.md` - row 4, created when the umbrella was found to disclaim the work.
  - `plan/spec-rfcgate-2-deferred-rs-replay-evidence.md` - rows 5 and 7 (renamed from `...-rs-replay-transparency.md`, whose name asserted a defect that does not exist).

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Tooling only; no config surface |
| YANG validation constraints | No | as above |
| YANG custom validators | No | as above |
| CLI commands/flags | No | No new user-facing command; existing make targets only |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | No | No YANG leaf added |
| Functional test for new RPC/API | N-A | No RPC/API added |
| Pipe completeness | N-A | No command output |
| Env var registration | No | No new env var |
| Doctor check for runtime dependencies | No | Docker is a developer/CI prerequisite of an existing suite, not a daemon runtime dependency; the runner's own fail-closed exit is the check |
| Prometheus counters/metrics | No | No runtime metric |
| BGP family surface | N-A | No SAFI/capability/attribute change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Tooling/evidence change; nothing an operator configures |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/rfc-status.md`: rows whose proof is nightly-tier only must say so; no RFC behaviour changes, but the *strength* of several claims becomes explicit |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (interop fails closed, runs nightly), `docs/architecture/testing/` page covering the RFC gate if one exists |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin/event/command/inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` and `ai/` for anchors naming `rfc_requirements.py`, `test/interop/run.py`, `evidence-nightly.yml`; update each stale claim (notably any text asserting interop's skip-on-no-docker behaviour) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Any doc showing `make ze-interop-test` output must not imply a clean exit without Docker |

## Implementation Steps

1. **Phase 1: Wiring -- make the evidence executable (MANDATORY FIRST)**
   - Tests: `TestInteropRunnerFailsClosedWithoutDocker`, `TestEvidenceNightlyRunsInterop`
   - Files: `test/interop/run.py`, `.github/workflows/evidence-nightly.yml`, `scripts/dev/github_workflows_test.go`
   - Verify: both tests fail first (runner exits 0; workflow has no interop job), then pass. Nothing in the scanner has changed yet -- this phase is deliberately inert with respect to `ze-rfc-check`.
2. **Phase 2: Carrier table and scanner extension**
   - Tests: `test_carrier_table_is_single_source`, `test_scan_python_tags_*`, `test_scan_et_reuses_ci_semantics`, `test_tree_and_baseline_filters_agree`, `test_tag_in_unrun_carrier_is_refused`
   - Files: `scripts/dev/rfc_requirements.py`, `scripts/dev/rfc_requirements_test.py`
   - Verify: with zero tags in the new carriers (A-6), the tree's tag set is unchanged, so `ze-rfc-check` output must be identical before and after this phase. Any diff means an unintended tag was imported.
3. **Phase 3: Ledger evidence kind and tier**
   - Tests: `test_ledger_row_carries_evidence_tier`, `test_nightly_only_marker_rendered`, `test_ledger_render_is_stable`
   - Files: `scripts/dev/rfc_requirements.py`, regenerated `ai/RFC-REQUIREMENTS.md`
   - Verify: regenerate; the only semantic change is the new column and marker. Commit the regenerated ledger in the same commit (`ai/rules/testing.md`).
4. **Phase 4: Non-unit evidence ratchet**
   - Tests: `test_non_unit_ratchet_fires_on_loss`, `test_verify_tier_ratchet_rejects_nightly_substitution`, `test_non_unit_ratchet_accepts_growth`
   - Files: `scripts/dev/rfc_requirements.py`
   - Verify: with today's tree the baseline is 4 requirements' worth of `.ci` evidence; deleting one of the four `.ci` tags must red the gate.
5. **Phase 5: Seed bindings, `.ci` first**
   - Tests: the seed `.ci` and interop scenarios themselves, each mutation-verified
   - Files: `test/plugin/*.ci`, `test/interop/scenarios/*/check.py`, regenerated ledger
   - Verify: for each binding, disable the producing function → test RED (record output) → restore → GREEN. Then `make ze-rfc-check` and `make ze-verify`.
6. **Phase 6: Discovery surfaces**
   - Files: `ai/rules/testing.md`, `ai/rules/rfc-compliance.md`, `docs/features/rfc-status.md`, `docs/functional-tests.md`
   - Verify: `make ze-doc-test`, `make ze-doc-links`, `make ze-rules-condensed` regenerated and committed with the rule edits.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | A tag in each supported carrier resolves, renders, and ratchets -- all three, not just the scan |
| Two-filter hazard | `scan_tree`, `_git_baseline_tag_polarities`, and `_scan_tags_tolerant` all read the one carrier table; grep the module for any remaining literal `endswith("_test.go")` / `endswith(".ci")` outside it |
| Ordering | Phase 1 landed before Phase 2 in commit order, not merely in the spec text |
| Correctness | The tier of every carrier matches `stagesForMode` reality; if a suite leaves `ze-verify`, the tier table must red rather than silently downgrade evidence |
| No flattening | A nightly-only requirement is visibly marked; the rollup does not sum nightly and verify evidence into one number |
| Vacuity | Every seed binding has a recorded RED from mutation verification (`ai/rules/interop-and-goal-validation.md`) |
| Rule: `derive-not-hardcode.md` | No consumer re-spells the carrier list |
| Rule: `fail-closed-guards.md` | Unknown carrier, unrun carrier, and unparseable carrier each deny loudly; none returns an empty tag list silently |
| Rule: `no-fabrication.md` | Every file:line in this spec re-checked against the tree at implementation time |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Interop runner fails closed | Run it with docker removed from PATH; assert non-zero exit |
| Interop in nightly CI | `grep ze-interop-test .github/workflows/evidence-nightly.yml` and the pinning Go test passes |
| Scanner sees `check.py` and `.et` | Add a temporary tag to a scenario, run `make ze-rfc-index`, see the row; remove it |
| Both filters extended | `test_tree_and_baseline_filters_agree` passes; grep shows no stray literal suffix check |
| Ledger shows tier | `grep -c 'nightly' ai/RFC-REQUIREMENTS.md` non-zero after seed bindings |
| Ratchet is live | Delete one `.ci` tag, run `make ze-rfc-check`, confirm red; restore |
| Classifier is absent from the implementation | `grep -ri 'wire-visible\|classifier' scripts/dev/rfc_requirements.py` returns nothing that gates (A-4) |
| Seed bindings mutation-verified | RED output recorded per binding in Goal Validation |

### Security Review Checklist
| Check | What to look for |
|-------|------------------|
| Input validation | `check.py` files are read and tokenized, never executed, by the scanner. The scanner must not import or exec scenario code -- `Scenario.run_check` executes it inside the Docker lab, which is a different trust context |
| Resource exhaustion | Tokenizing ~104 scenario files plus 164 `.et` files must stay inside the gate's runtime budget. ~~keep `ze-rfc-check` within noise of its current ~2.2s~~ (corrected 2026-07-29: ~2.2s was a design-time estimate that nobody re-measured. Measured figures for the `--check` half: **2.64s at HEAD** and **2.77-3.21s on the working tree**, both measured by the two independent reviews of this spec; and **2.59 / 2.67 / 2.63s on the working tree** over three runs measured by the closure session on this host. The budget is therefore "under ~3.5s for `--check`", and the honest reading of the spread is that the added tokenization is at or below the run-to-run noise floor rather than demonstrably free. Note the make target `ze-rfc-check` is ~15s wall, dominated by the `--selftest` half (~12s), not by the scan) |
| Fail-open | The new carriers must never widen the "silently skipped" set that this spec exists to close |
| Error leakage | Scanner errors name repo-relative paths only |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior; if misunderstood → RESEARCH |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| `ze-rfc-check` output differs after Phase 2 with no tags added | STOP: an unintended tag was imported. Re-check A-6 |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The scanner's silence is the defect. `scan_tree` skipping a file with no warning is
  the same class of fail-open as the interop runner's `sys.exit(0)`: both report
  success over an absence. Closing one without the other just moves the lie.
- Evidence has two independent axes -- **kind** (what layer it exercises) and
  **tier** (whether anything executes it). Conflating them is how "we have interop
  coverage" becomes true and worthless at the same time.
- `.et` was the pleasant surprise: it looked like a third carrier needing new
  scanning, and turned out to be `.ci` semantics (163 of 164 files use
  `terminator=`) executing inside a `ze-verify` stage. It is the cheapest
  verify-tier non-unit carrier available, and it costs one dispatch-table row.
  What it lacks is a customer: no editor-visible RFC obligation exists to bind
  today, so the carrier ships supported and unbound, and that is a settled
  decision rather than an unfinished one (see Key Design Decisions). Cheap to
  have ready, expensive to rediscover.
- The injector sidecar means "no conforming peer can emit this" is not the end of
  interop testability. It is a *different peer*, not an absent one.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Execution (Phase 1) strictly before scanning (Phase 2) | Land the scanner first, wire CI after | A tag in an unexecuted carrier satisfies `ze-rfc-check` with nothing behind it -- worse than the unit binding it replaces, which runs on every push. Owner-settled |
| Interop as an ADVISORY nightly job | Blocking `ze-verify` stage | Rejected by the owner: would require Docker on every developer machine and materially slow verify. Recorded as rejected, not deferred |
| A tag in an `unrun` carrier is a hard error | Accept it and mark it "unrun" in the ledger | A marker is a note; a refusal is a guard. The `unrun` trees have no automated caller at all, so the marker would decorate evidence that never executes (`ai/rules/fail-closed-guards.md`) |
| Tag stays a source comment; no per-scenario metadata file | A `scenario.toml` / `meta.json` per scenario | No such file exists in any of the 104 scenarios, and a second source of truth beside the test text is exactly what `derive-not-hardcode.md` forbids. `# RFC requirement:` at line start already matches the existing `.ci` regex shape |
| `check.py` scanned via Python tokenization (comment tokens only) | Plain regex line scan | A regex cannot tell a comment from a `#` inside a string or docstring, and scenario checks are full of quoted protocol text. Tokenizing is the Python analogue of "a comment is only a comment where the real parser says so" |
| `terminator=` handling is NOT inherited by the Python scanner | Reuse `scan_ci_tags` for `check.py` | `terminator=` models the `.ci` runner's tmpfs blocks (`internal/test/runner/parsing.go:264-268`), where block content is raw file content. Python has no such construct: `check.py` is executed wholesale by `importlib` (`interop.py:1468-1488`), so every line is Python and skipping "block bodies" would drop real comments |
| `.et` DOES reuse `scan_ci_tags` | A third scanner | `.et` genuinely has `terminator=` blocks (163/164 files) and `#` comments; reusing the existing scanner keeps one implementation of the trap |
| Two independent monotonic counters (verify-tier, nightly-tier) | One "non-unit evidence" counter | A single counter lets a verify-tier `.ci` binding be swapped for a nightly interop binding with no signal -- a silent downgrade. Separate counters make the downgrade a red gate (R-1) |
| The wire-visible classifier is a sizing tool only | Gate on the classifier's wire-visible set | ~97% precision but poor recall, and a gate built on it would manufacture obligations for mis-classified requirements. Deliberately absent from every AC |
| Prefer `.ci` bindings over interop bindings | Bind wire behaviour to interop by default | `.ci` runs inside `ze-verify` on every push; interop will be nightly-advisory. Owner-settled |
| `.et` is a fully supported carrier that carries NO seed binding (settled 2026-07-29) | Manufacture an editor-visible RFC obligation so the `.et` path gets a real binding; or drop `.et` support from the carrier table since nothing uses it | Supporting the carrier is the deliverable, and it is delivered and proven by AC-6 and AC-10. Binding a requirement to an `.et` merely to exercise the plumbing inverts the relationship the whole spec is about: the test would exist for the gate rather than for the requirement, which is precisely the vacuity `ai/rules/interop-and-goal-validation.md` forbids and which this spec spends R-3 and AC-17 guarding against. Dropping `.et` support was rejected too: it is the cheapest verify-tier non-unit carrier available (one dispatch-table row), so the cost of having it ready is a row and the cost of not having it is a rediscovery. The binding becomes live the day a genuinely editor-visible obligation surfaces during the drain, and no spec change is needed for that |

## Known Limitations

- The IPsec (10), L2TP (3) and PPPoE (1) interop trees stay `unrun` after this
  spec: only `test/interop` gains a nightly job. Tags there are refused (AC-7).
  A row in `plan/deferrals/rfcgate-2-evidence.md` names the destination spec for
  wiring those trees into CI.
- The interop job lands **advisory**, so a red nightly blocks nothing. Promotion
  to blocking waits on a green baseline and belongs to the umbrella, not here.
- This spec adds carriers and a ratchet; it does not re-bind the existing ~2571
  unit-only requirements. The back-fill remainder is tracked, per
  `ai/rules/testing.md` "Back-Fill New Test Types", in the deferral shard with the
  classifier's estimate as its sizing input.
- Requirements that are genuinely internal (no wire form) keep unit-only evidence
  and are correct as they stand. Nothing here implies unit evidence is inferior in
  general -- only that it cannot prove a wire obligation.

**Added 2026-07-29, from the two independent reviews of this spec.** Each is a
limitation of what the delivered gate can see, not a task left undone:

- **The carrier table's tier is an assertion, and nothing pins it to reality.**
  `CARRIERS` declares `.ci` as `functional/verify` because `ze-functional-test` is
  in `stagesForMode`; no gate re-derives that from `stagesForMode`. A one-word edit
  to a tier cell silently downgrades or upgrades every claim in the repo without
  reddening anything, and re-labelling the ratchet baseline with today's table means
  the ratchet cannot see it either. The spec's own Critical Review Checklist asks a
  human to check tier-against-`stagesForMode`; that is the whole enforcement.
- **The ledger states a pipeline CLASS, not a pipeline RESULT.** `interop/nightly`
  tells a reader the proof is scheduled and advisory. It does not tell them whether
  the last nightly ran, or was green. A reader can separate nightly-tier evidence
  from verify-tier evidence, and cannot separate a nightly that passed from one that
  has been red for a month. R-7 anticipated the red-nightly hazard and the marker
  addresses only half of it.
- **`.ci`/`.et` claim merge-gate tier by EXTENSION, not by membership of a suite
  `ze-verify` runs.** During implementation this credited `test/draft/` and roughly
  59 files in suites no `ze-verify` stage executes, with three demonstrated silent
  ratchet evasions. The delivered scanner narrows this, but the residual principle
  stands: tier is inferred from a path shape, and a path shape is not a pipeline.
- **A test that goes red after an unrelated fix may have been green *because* of a
  bug.** Scenario 47's green had depended on an RFC 4271 Section 5.1.2 defect; when
  `8bb55e509` fixed that on 2026-07-25 the scenario went red, and the red was first
  read as a NEW route-server defect. It was not one. Nothing in this spec's tooling
  can detect this class -- only reading the producing code can.

## RFC Documentation (Scope: protocol)

Not applicable to the tooling changes. It applies to the Phase 5 seed bindings:
each newly bound requirement must already carry its `// RFC NNNN Section X.Y`
comment above the enforcing code, and the seed binding must not be the first time
that citation appears. If it is, the citation is added in the same commit.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-18 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes (the pre-commit gate; `ai/rules/git-safety.md`)
- [ ] Feature code integrated, not library-only: every new scanner path is reached by `make ze-rfc-check`
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)
- [ ] Every seed binding mutation-verified RED, output recorded

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-rfcgate-2-evidence.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-rfcgate-2-evidence.md` only

---

## Implementation Summary

### What Was Implemented

- **Phase 1, execution.** `test/interop/run.py`'s Docker probe now fails closed
  (`:132-140`), matching `test/ipsec-interop/run.py`. `.github/workflows/evidence-nightly.yml`
  gained an `interop` job (`:115-129`) running `make ze-interop-test`, `continue-on-error: true`.
- **Phase 2, carriers.** `scripts/dev/rfc_requirements.py` gained one declared carrier table,
  `CARRIERS` (`:641`), with `Carrier` (`:621`) and the tier constants `TIER_VERIFY` /
  `TIER_NIGHTLY` / `TIER_UNRUN` (`:616-618`). `carrier_for` (`:745`) is the single lookup; the
  tree scan (`:773`), the HEAD baseline (`:1163`) and the tolerant scan (`:1199`) all read it,
  and no literal suffix check survives anywhere in the module. `scan_python_tags` (`:524`)
  tokenizes `check.py`; `.et` routes to the existing `scan_ci_tags` (`:568`).
- **Phase 3, ledger.** `render_ledger` (`:1977`) emits a `kind/tier` cell per test link and a
  `**nightly-only**` marker (`:2044`); `_render_rollup` (`:1871`) carries the nightly-only
  count in its own column, and the legend is derived from `CARRIERS`, not authored.
- **Phase 4, ratchet.** `check_evidence_ratchet` (`:1355`) keyed by `kind/tier`, beside the
  existing `check_coverage_ratchet` (`:1303`).
- **Phase 5, seed bindings. All three clauses landed** (interop completed 2026-07-29).
  `.ci`: `test/plugin/rfc7606-relay-one-field.ci:3` and `:7` bind `RFC7606-5.1-2 positive` and
  `RFC7606-5.1-3 positive`. Peer-driven interop: `test/interop/scenarios/14-route-server-frr/check.py:8`
  binds `RFC7947-x-1 positive`, asserted at BIRD rather than out of Ze's own RIB. Injector-driven
  interop: `test/interop/scenarios/47-rfc7606-relay-shape-frr/check.py:28` binds
  `RFC7606-5.1-3 positive`, the §5.1 receiver-side shape no conforming sender may emit. Each is
  mutation-verified RED (Goal Validation 5a-5c). Both scenarios' `ze.conf` also gained
  `session/rs-client true` on each peer -- the leaf defaults to false, and its absence is what
  made scenario 47's red look like a daemon defect for two agents running. `.et` carries no
  binding by settled decision, not by omission.
- **Phase 6, discovery.** `ai/rules/testing.md` RFC-Tagged Tests section (carrier table, tier
  distinction, refusal rule, monotonicity, tokenization) plus the regenerated
  `ai/rules/CONDENSED.md`; `ai/rules/rfc-compliance.md` ratchets table; `docs/features/rfc-status.md`;
  `docs/functional-tests.md` BGP-interop row.

### Bugs Found/Fixed

- **The interop lab launched `ze` with a removed bare `ze <config>` form**, so every scenario
  in all four interop trees died at container health. Found by running the BGP lab for the
  first time since the launch form changed. Fixed across all four trees.
- **The `ai/rules/testing.md` edit did not survive condensation.** Five directives and the lead
  paragraph were wrapped across physical lines, and `scripts/dev/rules_condensed.py` keeps only
  a list item's FIRST physical line (`:144-146`), treating continuations as prose that
  `flush_prose` (`:106-114`) drops or truncates at 220 characters. The unrun-carrier directive
  lost its verb entirely: the digest said a tag in those trees *is*, and stopped, with the
  refusal gone. Fixed by re-authoring each directive onto one physical line; verified by
  reading the regenerated section, not by re-running the generator.

### Documentation Updates

- `docs/functional-tests.md` -- BGP interop row: `make ze-interop-test` named, fail-closed
  behavior stated, nightly/advisory pipeline stated. Verified against `test/interop/run.py:132-140`
  and `.github/workflows/evidence-nightly.yml:115-129`.
- `ai/rules/testing.md` + regenerated `ai/rules/CONDENSED.md` (`make ze-rules-condensed`).
- `docs/features/rfc-status.md`, `ai/rules/rfc-compliance.md` -- owned by the parallel session.
- `ai/RFC-REQUIREMENTS.md` -- regenerated, never hand-edited.
- `rfc/audit/rfc7606.json` -- hand re-stamped; see Files to Modify for why this one must be.

### Deviations from Plan

| # | Deviation | Why |
|---|-----------|-----|
| D-a | `rfc/audit/rfc7606.json` was hand-edited, and it was not in the design-time Files to Modify | `verdict_is_fresh` (`rfc_requirements.py:1640-1642`) is exact equality on the whole `tests` map, so this spec's own new `.ci` tags staled verdicts whose requirement text and assertions never changed. Sixth mechanical re-stamp of this file; fresh evidence for `plan/spec-rfcgate-3-audit-teeth.md` |
| D-b | The launch-form fix initially covered four Docker labs while its own comment claimed the sibling audit was complete | Eight executable sites were still on the removed form, including the shipped `docker/compose.yaml`. `ai/rules/before-writing-code.md`'s sibling call-site audit requires grepping the bare token across `.ci` `exec=`, embedded `tmpfs=` bodies, helper scripts and shipped compose files, not only the framework directive. The claim of completeness preceded the audit that would have justified it |
| D-c | `.ci`/`.et` initially claimed merge-gate tier by extension alone | That credited `test/draft/` and roughly 59 files in suites no `ze-verify` stage runs, with three demonstrated silent ratchet evasions. Narrowed during implementation; the residual limitation is recorded in Known Limitations |
| D-d | AC-9's text said "does not parse as Python"; the shipped guard is at TOKENIZE level | Tokenize is the CORRECT level -- a comment is lexical. The AC text was corrected rather than the behavior; a `print 'py2'` file tokenizes and is scanned, verified empirically |
| D-e | The seed set carries no `.et` binding | Settled N-A (2026-07-29), recorded in Key Design Decisions and the TDD Functional Tests table, not an unmet target |
| D-f | The RS-replay deferral was filed against a defect that does not exist | See the Mistake Log row below. The destination spec was repurposed onto the real gap rather than deleted |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | Scenario 47 failing at `check_route_no_as("10.0.0.0/24", "65001")` was read as a route-server defect: "the replay rail prepends Ze's AS because RS transparency is implemented only on the forward rail". Two deferral rows and a whole destination spec were filed on it | ONE prepend gate serves BOTH rails: `if facts.isEBGP && !facts.rsClient` (`reactor/reactor_api_forward.go:711`), reached by `RelayStoredRoute` via `reactor_api_relay.go:253` and by `ForwardUpdate` via `reactor_api_forward.go:358`. `facts.rsClient` comes only from the `session/rs-client` leaf (`reactor/peer_forward_facts.go:111` <- `reactor/config.go:266`), default `false` (`bgp/plugins/rs/yang/ze-rs-conf.yang:40-46`), and NO interop scenario sets it (`grep -rn rs-client test/interop/` is empty). Ze prepended correctly for a peer that was, as configured, a plain eBGP peer. The scenario's earlier green depended on an RFC 4271 Section 5.1.2 bug that `8bb55e509` fixed on 2026-07-25 | Independent review of this spec read the producing gate instead of the symptom | Deferral rows 5 and 6 rewritten and a row 7 added; the destination spec was repurposed onto the real gap (`RFC7947-x-1` has no non-unit evidence and no rs-client relay test) rather than deleted, and renamed to `plan/spec-rfcgate-2-deferred-rs-replay-evidence.md` because the old stem asserted a defect that does not exist |
| escalation | A test going red after an unrelated fix was read as a NEW defect | The test had been green BECAUSE of a bug. When `8bb55e509` fixed the RFC 4271 prepend, scenario 47's expectation stopped being satisfiable by the broken path | Same review | Recorded in Known Limitations as a general class no tooling in this spec can detect |
| approach | The `ai/rules/testing.md` directives were written as wrapped multi-line bullets | The condenser keeps only a list item's first physical line; five directives and the lead paragraph reached `CONDENSED.md` truncated, one losing its verb | Regenerating and READING the digest, which `ai/rules/rule-format.md` requires and which regenerating alone does not accomplish | Re-authored onto single physical lines; digest re-read and pasted into the closure report |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| 1. Non-unit evidence EXECUTES in an automated pipeline, fail-closed | Done | `test/interop/run.py:132-140`, `.github/workflows/evidence-nightly.yml:115-129` | Advisory by owner decision, not by omission |
| 2. Only then let the scanner SEE it | Done | `CARRIERS` `rfc_requirements.py:641`, `scan_python_tags:524` | Phase 1 committed before Phase 2 |
| 3. The ledger SHOWS evidence strength | Done | `render_ledger:1977`, `_render_rollup:1871`, marker at `:2044` | `kind/tier` per link; nightly-only in its own column |
| 4. Ratchet non-unit evidence so it can only rise | Done | `check_evidence_ratchet:1355` | Keyed by `kind/tier`, so a substitution at equal count still fires |
| 5. Prove the chain with a seed set, `.ci`-first | ~~Partial~~ **Done (2026-07-29)** | `test/plugin/rfc7606-relay-one-field.ci:3,:7`; `test/interop/scenarios/14-route-server-frr/check.py:8`; `test/interop/scenarios/47-rfc7606-relay-shape-frr/check.py:28` | All three clauses landed and each is mutation-verified RED (Goal Validation 5a/5b/5c). `.et` is a settled N-A, not a gap (Key Design Decisions) |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `test/interop/run.py:132-140`; `TestInteropRunnerFailsClosedWithoutDocker` (`test/interop/run_test.go:34`) | Exits non-zero naming Docker |
| AC-2 | Done | `.github/workflows/evidence-nightly.yml:115-129` | Own job, `continue-on-error: true` |
| AC-3 | Done | `TestEvidenceNightlyRunsInterop` (`scripts/dev/github_workflows_test.go:328`) | Scheduled-only and advisory pins at `:279`, `:303`, `:360` still pass |
| AC-4 | Done | `test_scan_python_tags_found` (`rfc_requirements_test.py:511`), `test_scan_python_tags_indented_comment_is_a_tag` (`:521`) | id/polarity/path/line |
| AC-5 | Done | `test_scan_python_tags_ignores_string_literals` (`:528`), `test_scan_python_tags_ignores_trailing_comment` (`:546`) | Tokenizer, not regex |
| AC-6 | Done | `test_scan_et_reuses_ci_semantics` (`:578`), `test_et_terminator_block_is_not_scanned` (`:587`) | No third implementation |
| AC-7 | Done | `test_tag_in_unrun_carrier_is_refused` (`:718`), `test_unclassified_scenario_check_is_refused_too` (`:733`); refusal at `rfc_requirements.py:799` | Names the file and the fix |
| AC-8 | Done | `test_tree_and_baseline_filters_agree` (`:784`), `test_baseline_prunes_the_same_dirs_as_the_tree_scanner` (`:1771`) | Both filters read `CARRIERS` |
| AC-9 | Done (text corrected) | `test_scan_python_tags_rejects_invalid_syntax` (`:551`); guard at `rfc_requirements.py:549-553` | AC wording corrected to TOKENIZE level; see D-d |
| AC-10 | Done | `test_ledger_row_carries_evidence_tier` (`:1277`), `test_interop_link_is_labelled_nightly` (`:1288`), `test_legend_is_derived_from_the_carrier_table` (`:1338`) | Live ledger shows 1351 `(unit/verify)` and 6 `(functional/verify)` cells |
| AC-11 | Done | `test_nightly_only_marker_rendered` (`:1333`), `test_no_nightly_marker_when_verify_evidence_exists` (`:1343`), `test_nightly_only_has_its_own_rollup_column` (`:1360`) | ~~Live count is 0 because no interop tag has landed yet~~ **Corrected 2026-07-29: still 0, better reason.** Two interop tags landed; neither requirement is nightly-only because both also hold verify-tier evidence. The marker is a subset marker over requirements, not a tag counter, and this is the first live case that distinguishes the two |
| AC-12 | Done | `test_ledger_render_is_stable` (`:1348`), `test_ledger_render_is_independent_of_input_order` (`:1207`) | |
| AC-13 | Done | `test_non_unit_ratchet_fires_on_loss` (`:1923`), `test_no_annotation_satisfies_the_ratchet` (`:1934`) | |
| AC-14 | Done | `test_verify_tier_ratchet_rejects_nightly_substitution` (`:1946`), `test_losing_nightly_while_keeping_verify_still_fires` (`:1975`) | |
| AC-15 | Done | `test_non_unit_ratchet_accepts_growth` (`:1984`), `test_holding_the_baseline_exactly_passes` (`:1994`) | |
| AC-16 | ~~Partial -- interop clause PENDING~~ **Done (2026-07-29)** | `.ci` clause: `test/plugin/rfc7606-relay-one-field.ci:3,:7`. Peer-driven interop clause: `test/interop/scenarios/14-route-server-frr/check.py:8` (`RFC7947-x-1 positive`). Injector-driven clause: `test/interop/scenarios/47-rfc7606-relay-shape-frr/check.py:28` (`RFC7606-5.1-3 positive`, the §5.1 receiver-side shape). `.et` clause: settled N-A (Key Design Decisions) | The prior closure pass held this Partial pending the RED output, which is the correct gate. All three required clauses now exist and resolve: `--check` reports `interop/nightly 2` and the ledger carries both cells (`:3686`, `:4041`) |
| AC-17 | ~~Partial -- interop half PENDING~~ **Done (2026-07-29)** | `.ci` half in the test's own header (~~`:45-50`~~ **`test/plugin/rfc7606-relay-one-field.ci:80-83`**, re-verified at closure): reversing `result.rawBodies`/`result.updates` at the end of `buildFwdBody` (`reactor/forward_body.go`) fails it 3/3 with "message mismatch". Interop halves recorded in Goal Validation 5b and 5c | Every binding has a recorded RED, which is AC-17's whole rule. The two interop REDs were produced by the implementing session against the real Docker labs; closure verified independently that both RED strings match their producing code (`interop.py:992`/`:86`, `interop.py:427`/`run.py:194`) rather than accepting them as prose -- see Goal Validation 5d |
| AC-18 | Done | ~~`test/plugin/rfc7606-relay-one-field.ci:11-19`~~ **`test/plugin/rfc7606-relay-one-field.ci:44-55`** (re-verified at closure; the header grew) states the choice and its two reasons | The `.ci` wins; the second reason is that scenario 47 cannot discriminate the split at all, since Section 5.1's third bullet obliges FRR to accept both forms. Scenario 47 therefore carries only the accept-on-receive binding (`5.1-3`), never `5.1-2` -- which the scenario's own header now states explicitly |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `test_scan_python_tags_found` | Done | `rfc_requirements_test.py:511` | |
| `test_scan_python_tags_ignores_string_literals` | Done | `:528` | |
| `test_scan_python_tags_rejects_invalid_syntax` | Done | `:551` | |
| `test_scan_et_reuses_ci_semantics` | Done | `:578` | |
| `test_tree_and_baseline_filters_agree` | Done | `:784` | |
| `test_tag_in_unrun_carrier_is_refused` | Done | `:718` | |
| `test_carrier_table_is_single_source` | Done | `:603` | plus `test_every_reader_exists` (`:632`) |
| `test_ledger_row_carries_evidence_tier` | Done | `:1277` | |
| `test_nightly_only_marker_rendered` | Done | `:1294` | |
| `test_ledger_render_is_stable` | Done | `:1348` | |
| `test_non_unit_ratchet_fires_on_loss` | Done | `:1923` | |
| `test_verify_tier_ratchet_rejects_nightly_substitution` | Done | `:1946` | |
| `test_non_unit_ratchet_accepts_growth` | Done | `:1984` | |
| `TestInteropRunnerFailsClosedWithoutDocker` | Done | `test/interop/run_test.go:34` | |
| `TestEvidenceNightlyRunsInterop` | Done | `scripts/dev/github_workflows_test.go:328` | |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `scripts/dev/rfc_requirements.py` | Done | carrier table, Python scanner, both filters, ledger tier, evidence ratchet |
| `test/interop/run.py` | Done | fail-closed probe |
| `.github/workflows/evidence-nightly.yml` | Done | advisory interop job |
| `scripts/dev/github_workflows_test.go` | Done | job pinned |
| `ai/RFC-REQUIREMENTS.md` | Done | regenerated |
| `ai/rules/testing.md` | Done | + regenerated `ai/rules/CONDENSED.md` |
| `ai/rules/rfc-compliance.md` | Done | parallel session |
| `docs/features/rfc-status.md` | Done | parallel session |
| `docs/functional-tests.md` | Done | BGP interop row |
| `rfc/audit/rfc7606.json` | Changed | NOT in the design-time list; see D-a |
| `scripts/dev/rfc_requirements_test.py` | Done | created |
| `test/interop/run_test.go` | Done | created |
| `plan/deferrals/rfcgate-2-evidence.md` | Done | created |
| `test/plugin/*.ci` seed tags | Done | `rfc7606-relay-one-field.ci` |
| `test/interop/scenarios/*/check.py` seed tags | ~~**Pending**~~ **Done (2026-07-29)** | Two tags landed: `14-route-server-frr/check.py:8` and `47-rfc7606-relay-shape-frr/check.py:28`. Both scenarios' `ze.conf` also gained `session/rs-client true` on each peer (14 at `:26`/`:41`, 47 at `:30`/`:45`), each with an inline comment recording that the leaf defaults to false -- the fix the Mistake Log's first row explains |

### Audit Summary

- **Total items:** 18 ACs + 15 TDD tests + 15 files = 48
- ~~**Done:** 46~~ **Done: 48** (2026-07-29 -- AC-16, AC-17 and the interop seed-tag file row closed when the interop bindings landed with their recorded REDs)
- ~~**Partial:** 2 (AC-16, AC-17 -- interop clauses only; both await the parallel session's RED output, neither is a scope reduction)~~ **Partial: 0**
- **Skipped:** 0
- **Changed:** 2 (AC-9 wording, `rfc/audit/rfc7606.json` added -- both in Deviations)

**Line-number drift, re-verified at closure 2026-07-29 (BLOCKING to record, not a defect in the work).**
`scripts/dev/rfc_requirements.py` and its test file are UNCOMMITTED and grew after the first
closure pass wrote its citations (the module is 3418 lines at HEAD and 4180 in the tree), so the
`file:line` refs in the Implementation Summary and in the AC rows above no longer land on the
symbol they name. **Every cited symbol and all 29 cited tests were re-verified to EXIST**; only
the numbers moved. Re-verified positions: `TIER_VERIFY/NIGHTLY/UNRUN` `:618-620` (was `:616-618`),
`scan_python_tags` `:526` (`:524`), `scan_ci_tags` `:570` (`:568`), `class Carrier` `:697` (`:621`),
`CARRIERS` `:720` (`:641`), `carrier_for` `:864` (`:745`), `scan_tree` `:905` (`:773`),
`_git_baseline_tag_polarities` `:1389` (`:1163`), `_scan_tags_tolerant` `:1431` (`:1199`),
`check_coverage_ratchet` `:1535` (`:1303`), `check_evidence_ratchet` `:1587` (`:1355`),
`_render_rollup` `:2103` (`:1871`), `render_ledger` `:2239` (`:1977`), the unrun refusal `:951`
(`:799`), the tokenize guard `:550-553` (`:548-553`). In the test file the shift is +3 below
line ~700 and +39 above ~1250. The durable anchor is the symbol name; a reader who greps for it
gets the right answer whatever the line has become. Phase 1's citations (`test/interop/run.py:132-140`,
`run_test.go:34`, `evidence-nightly.yml:115-129`, `github_workflows_test.go:328`/`:279`/`:303`/`:360`)
were re-verified and hold EXACTLY.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| 1. Non-unit evidence executes, fail-closed | functional (Go) | `TestInteropRunnerFailsClosedWithoutDocker` (`test/interop/run_test.go:34`) drives the runner with Docker absent and asserts a non-zero exit. Producing code `test/interop/run.py:132-140` |
| 2. The scanner sees the new carriers | tooling selftest | `python3 scripts/dev/rfc_requirements.py --selftest` -> `rfc_requirements selftest OK` (361 tests, re-run at closure). `--check` reports `evidence: unit/verify 2571, functional/verify 6, editor/verify 0, ~~interop/nightly 0~~ **interop/nightly 2**` -- corrected 2026-07-29 at closure, after the interop bindings landed. The six functional tags are visible where four were before this spec, and the interop carrier is now exercised by real tags rather than by fixtures alone |
| 3. The ledger shows evidence strength | generated artifact | `ai/RFC-REQUIREMENTS.md` carries 1351 `(unit/verify)`, 6 `(functional/verify)` and **2 `(interop/nightly)`** cells, a derived legend at `:11-18`, and the nightly-only rollup column explained at `:26`. ~~Zero `nightly-only` markers today because no interop tag has landed~~ -- **corrected 2026-07-29: the count is still 0, but for a stronger reason.** Two interop tags HAVE landed; neither requirement is nightly-only because each also carries verify-tier evidence (`RFC7606-5.1-3` has a unit test and the `.ci`; `RFC7947-x-1` has unit tests on both polarities). That is the marker behaving as designed -- it is a subset marker over requirements, not a counter of interop tags -- and it is exactly the distinction R-1 exists to preserve. The marker's rendering is proven by `test_nightly_only_marker_rendered`, not by the live count |
| 4. Non-unit evidence can only rise | tooling selftest | `test_non_unit_ratchet_fires_on_loss`, `test_verify_tier_ratchet_rejects_nightly_substitution`, `test_losing_nightly_while_keeping_verify_still_fires`, `test_moving_a_ci_tag_within_the_carrier_is_invisible` (`rfc_requirements_test.py:1923-2004`) |
| 5a. Seed binding, `.ci` clause (AC-16, AC-17) | functional `.ci` + mutation-verify | `test/plugin/rfc7606-relay-one-field.ci:3,:7` bind `RFC7606-5.1-2 positive` and `RFC7606-5.1-3 positive`. **Recorded RED:** reversing `result.rawBodies`/`result.updates` at the end of `buildFwdBody` (`reactor/forward_body.go`), after `supersedeKey` and the withdrawal flag are computed so only emission order changes, fails this test **3/3 with "message mismatch"**, the announcement being the only frame `conn=2` accepted (`test/plugin/rfc7606-relay-one-field.ci:45-50`) |
| 5b. Seed binding, peer-driven interop clause (AC-16) + its RED (AC-17) | interop + mutation-verify | **FILLED 2026-07-29.** `test/interop/scenarios/14-route-server-frr/check.py:8` binds `RFC7947-x-1 positive`. FRR originates 10.99.0.0/24, Ze relays it, and **BIRD** is asserted -- a second foreign daemon parsing the wire Ze emitted, so the transparency claim is never read back out of the speaker that built the path. Resolves as `interop/nightly` in the live ledger (`ai/RFC-REQUIREMENTS.md:4041`). **Recorded RED:** dropping `!facts.rsClient` from the prepend gate (`internal/component/bgp/reactor/reactor_api_forward.go:711`) fails the run at `✗ BIRD route 10.99.0.0/24 AS_PATH contains AS 65001` (run `14-mutant`); reverting returns PASS (`14-restored`); the un-mutated run (`14-after`) is PASS. The mutation targets the exact gate the requirement is about, so the binding is discriminating rather than merely present |
| 5c. Seed binding, injector-driven clause (AC-16) + its RED (AC-17) | interop + mutation-verify | **FILLED 2026-07-29.** `test/interop/scenarios/47-rfc7606-relay-shape-frr/check.py:28` binds `RFC7606-5.1-3 positive` -- the receiver-side §5.1 shape whose second bullet forbids any conforming SENDER to emit it, so the raw injector sidecar is the only carrier that can drive it against a real FRR. Resolves as `interop/nightly` (`ai/RFC-REQUIREMENTS.md:3686`). **Recorded RED:** mutating Ze to REJECT the mixed shape on receive fails the run at `✗ FAIL: FRR missing route 10.0.0.0/24` (run `47-mutant`); reverting returns PASS (`47-restored`). Full sequence: `47-before` FAIL (the pre-existing red, misdiagnosed -- see the Mistake Log), `47-after` PASS once `rs-client true` was set, `47-mutant` FAIL, `47-restored` PASS |
| 5d. Provenance of the four rows above (honesty about who observed what) | attribution | The seven Docker verdicts in 5b/5c were produced by the **implementing** session against the real labs; the closure session did not re-run Docker. What closure verified independently: both tags sit at line start in real comments (`14-.../check.py:8`, `47-.../check.py:28`); `rs-client true` is set on both peers in both scenarios' `ze.conf`; `python3 scripts/dev/rfc_requirements.py --check` exits 0 reporting `interop/nightly 2` (was 0), which also clears `check_ledger_fresh` and `check_audit_freshness`; `--selftest` is 361 tests OK; and **both RED strings match their producing code** -- `✗ BIRD route ... AS_PATH contains AS ...` is `interop.py:992` through `log_fail` (`interop.py:86`), `✗ FAIL: FRR missing route ...` is `interop.py:427`'s AssertionError surfaced by `run.py:194`. A reported RED whose text no producer can emit would be the tell this check exists to catch |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| Rows 1-3: wire `test/ipsec-interop/`, `test/l2tp-interop/`, `test/pppoe-interop/` into a pipeline and tier their carriers | deferred (live) | `plan/spec-rfcgate-2-deferred-unrun-interop-trees.md`. Correct interim state: `TIER_UNRUN` refuses a tag there (AC-7). **No shard edit needed** |
| Row 4: back-fill non-unit evidence for the ~2571 unit-only requirements | deferred (live) | **Re-homed 2026-07-29** to `plan/spec-rfcgate-2-deferred-nonunit-evidence-backfill.md` (created for it). Was `plan/spec-rfcgate-0-umbrella.md`, whose own D4 places the fleet drain outside the set and whose Constraint(D4) forbids a child opening the backlog -- a destination that disclaims the work is not a home. **Optional shard edit:** the row's parenthetical quotes `interop/nightly 0`, which the two new bindings made stale; the current split is `unit/verify 2571, functional/verify 6, editor/verify 0, interop/nightly 2`. The `2571` sizing figure that the row actually turns on is UNCHANGED, so this is an accuracy fix, not a scope change |
| Row 5: bind `RFC7606-5.1-3 positive` to scenario 47 | ~~deferred (live)~~ **done (2026-07-29)** | **The block is cleared and the work landed in THIS spec.** Its stated blocker was that scenario 47 was red for an unrelated reason, so tagging it would publish evidence for a failing test. The real cause was the missing `rs-client` leaf, not a defect; setting it turned the scenario green (`47-after` PASS) and the tag now sits at `test/interop/scenarios/47-rfc7606-relay-shape-frr/check.py:28`, resolving as `interop/nightly` (`ai/RFC-REQUIREMENTS.md:3686`) with a recorded RED (Goal Validation 5c). **Shard edit required:** Status `deferred` -> `done`, Destination -> `plan/learned/1296-rfcgate-2-evidence.md` |
| Row 6: "fix RS AS-path transparency on the replay rail" | done (superseded) | The defect does not exist (Mistake Log row 1). Kept in the shard with its refutation rather than deleted, and superseded by row 7. **No shard edit needed** |
| Row 7 (new): give `RFC7947-x-1` non-unit evidence **and** add an rs-client relay test | **SPLIT: first half done, second half still live** | Two separable items in one row, and only the first landed. **Done:** `RFC7947-x-1` now has non-unit evidence -- `test/interop/scenarios/14-route-server-frr/check.py:8`, `interop/nightly` in the ledger at `:4041`, mutation-verified RED (Goal Validation 5b). **Still live:** no rs-client **relay** test exists. Verified at closure, not assumed: `internal/component/bgp/reactor/reactor_api_relay_test.go` is unmodified in the working tree and `grep -n 'rsClient\|rs-client'` over it returns nothing, so nothing pins AS_PATH byte-identity through `RelayStoredRoute` to an RS-client destination. **Shard edit required:** keep Status `deferred` and Destination `plan/spec-rfcgate-2-deferred-rs-replay-evidence.md`, and narrow the What to the relay test alone with a dated note that the evidence half landed here. Marking the whole row done would silently drop a real test |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | (not yet recorded -- `scripts/dev/review_gate.py record --spec plan/spec-rfcgate-2-evidence.md ...`) |
| `review_gate.py check` | not run |
| Reviewer lenses used | Two independent reviews ran before closure: (a) rule/digest + deferral hygiene + spec-internal consistency, (b) protocol correctness of the RS-replay claim + gate-runtime measurement + tier-assertion analysis |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The `ai/rules/testing.md` edit did not survive condensation: five directives and the lead paragraph truncated mid-sentence, the unrun-carrier directive losing its verb entirely | `ai/rules/testing.md:54-79`, `ai/rules/CONDENSED.md` | Re-authored each directive onto one physical line; regenerated and re-read the digest |
| 2 | BLOCKER | The spec had no closure sections at all: no Goal Validation, no Implementation Audit, no Pre-Commit Verification, while AC-17 requires the recorded RED to live in Goal Validation | this spec | These sections |
| 3 | BLOCKER | The RS-replay deferral was premised on a defect that does not exist | `plan/spec-rfcgate-2-deferred-rs-replay-evidence.md` (renamed), deferral rows 5-7 | Premise refuted against `reactor_api_forward.go:711`; spec repurposed onto the real evidence gap |
| 4 | ISSUE | Deferral row 4's destination disclaims the work | `plan/deferrals/rfcgate-2-evidence.md:10` | Re-homed |
| 5 | ISSUE | Both new deferred specs carried the template placeholder `(or \`-\` if nothing deferred)` as if it were a real path | both deferred specs' metadata tables | Replaced with real values |
| 6 | ISSUE | Risks cited AC-13/AC-14 for clauses that are now AC-16/AC-17 | Risks R-2, R-3 | Struck through with dated corrections |
| 7 | ISSUE | AC-9 described a parse-level guard; the shipped guard is tokenize-level, which is correct | AC-9 | AC text corrected, behavior unchanged |
| 8 | ISSUE | The gate-runtime budget cited an unmeasured ~2.2s | Security Review Checklist; Behavior to preserve | Replaced with measured figures and attribution |
| 9 | ISSUE | The launch-form fix claimed a complete sibling audit while eight executable sites remained | `docker/compose.yaml` and seven others | Fixed; recorded as D-b |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `scripts/dev/rfc_requirements_test.py` | Yes | `ls -la` -> `-rw-rw-r-- 229089 Jul 29 15:23` |
| `test/interop/run_test.go` | Yes | `ls -la` -> `-rw-rw-r-- 2699 Jul 29 15:05` |
| `plan/deferrals/rfcgate-2-evidence.md` | Yes | `ls -la` -> `-rw-rw-r-- 2999 Jul 29 15:58` |
| `test/plugin/rfc7606-relay-one-field.ci` | Yes | read at closure; tags at `:3` and `:7`, mutation record at `:45-50` |
| `plan/spec-rfcgate-2-deferred-unrun-interop-trees.md` | Yes | read at closure |
| `plan/spec-rfcgate-2-deferred-rs-replay-evidence.md` | Yes | `ls -la` -> present; repurposed and renamed from `...-rs-replay-transparency.md` |
| `plan/spec-rfcgate-2-deferred-nonunit-evidence-backfill.md` | Yes | `ls -la` -> present; created for deferral row 4 |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Runner fails closed without Docker | `sed -n '124,140p' test/interop/run.py` shows `docker_ok = False` on `FileNotFoundError` then a non-zero exit path with an `error:` message naming Docker |
| AC-3 | The nightly job is pinned by name and advisory | `grep -n '^func Test' scripts/dev/github_workflows_test.go` -> `TestEvidenceNightlyRunsInterop:328` beside `TestEvidenceNightlyIsAdvisory:360` |
| AC-2 | The job exists and is advisory | `grep -n 'interop\|continue-on-error' .github/workflows/evidence-nightly.yml` -> job `interop:115`, `continue-on-error: true:117`, `run: make ze-interop-test:129` |
| AC-4..AC-9 | Scanner behavior | `python3 scripts/dev/rfc_requirements.py --selftest` -> `rfc_requirements selftest OK` (all named tests present per `grep -n '^    def test_'`) |
| AC-5/AC-9 | Tokenize level, not parse level | Executed `scan_python_tags` directly: a `print 'py2'` file yields `[Tag(rid='RFC7606-5.1-3', polarity='positive', line=2)]`; an unterminated-string file raises `ParseError: cannot tokenize as Python` |
| AC-8 | One carrier table, no stray suffix checks | `grep -n 'endswith("_test.go")\|endswith(".ci")\|endswith(".et")\|endswith("check.py")' scripts/dev/rfc_requirements.py` -> no matches (exit 1) |
| AC-10 | Tier cells rendered | `grep -c '(unit/verify)' ai/RFC-REQUIREMENTS.md` -> 1351; `grep -c '(functional/verify)'` -> 6 |
| AC-11 | Nightly-only is a separate column, never summed | `ai/RFC-REQUIREMENTS.md:26` -- "Read it as a subset marker, never as a total to sum with the others" |
| AC-13..AC-15 | Ratchet is live | `--check` prints the tier split: `unit/verify 2571, functional/verify 6, editor/verify 0, interop/nightly 0` |
| A-4 (no classifier) | Classifier absent from the implementation | `grep -rin 'wire-visible\|classifier' scripts/dev/rfc_requirements.py` -> no matches (exit 1) |
| AC-16/AC-17 | Interop clauses | ~~NOT VERIFIED -- pending the parallel session~~ **VERIFIED 2026-07-29.** Re-run at closure: `grep -rn 'RFC requirement:' test/interop/scenarios/*/check.py` now returns exactly two hits, both at line start in real comments -- `14-route-server-frr/check.py:8` (`RFC7947-x-1 positive`) and `47-rfc7606-relay-shape-frr/check.py:28` (`RFC7606-5.1-3 positive`). The earlier prose-only mention is gone. `python3 scripts/dev/rfc_requirements.py --check` -> exit 0, `interop/nightly 2` |
| AC-16 (ledger join) | Both interop tags actually RESOLVE to their requirement, not merely exist | `grep -n 'RFC7947-x-1\|RFC7606-5.1-3' ai/RFC-REQUIREMENTS.md` -> `:3686` carries `test/interop/scenarios/47-.../check.py:28 (interop/nightly)` beside a unit and a `.ci` link; `:4041` carries `test/interop/scenarios/14-.../check.py:8 (interop/nightly)`. A tag that scanned but failed to join would appear nowhere here |
| AC-17 (RED provenance) | The two recorded interop REDs are real output, not paraphrase | Traced each string to its producer: `✗ BIRD route 10.99.0.0/24 AS_PATH contains AS 65001` = `test/interop/interop.py:992` emitted through `log_fail` (`:86`, which supplies the `✗`); `✗ FAIL: FRR missing route 10.0.0.0/24` = `interop.py:427`'s `AssertionError` wrapped by `run.py:194` (`log_fail("FAIL: %s" % e)`). Both mutations target the producing code the binding is about -- the prepend gate `reactor_api_forward.go:711` for `RFC7947-x-1`, receive-side acceptance for `RFC7606-5.1-3` |
| Config precondition | The interop greens depend on a leaf that defaults to false, and that is recorded where a reader will hit it | `grep -rn 'rs-client' test/interop/` -> `session/rs-client true` on BOTH peers in BOTH scenarios (14 at `:26`/`:41`, 47 at `:30`/`:45`), each with an inline comment naming the YANG default. Before this, the tree-wide grep was empty -- which is precisely what made scenario 47's red look like a daemon defect |
| Ledger + audit freshness | The generated artifacts are current WITH the new tags, so no stale-ledger red is being deferred to a later commit | `--check` exit 0 runs `check_ledger_fresh` AND `check_audit_freshness` (read at `scripts/dev/rfc_requirements.py` `run_check`). `rfc/audit/rfc7606.json:231` carries the new fingerprint key `test/interop/scenarios/47-.../check.py:28` -> `57df538ba25f86d0`, so the sixth re-stamp covers the interop tag too |
| Selftest | Scanner behavior re-proven at closure, not quoted from the earlier pass | `python3 scripts/dev/rfc_requirements.py --selftest` -> `Ran 361 tests in 11.727s / OK / rfc_requirements selftest OK`, exit 0 |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| A developer runs the interop runner without Docker | `test/interop/run_test.go:34` (Go, not `.ci`) | Yes -- read; it manipulates PATH and asserts a non-zero exit, not merely that the process ran |
| The nightly schedule fires | `.github/workflows/evidence-nightly.yml:115-129` pinned by `github_workflows_test.go:328` | Yes -- read both; the job name, the make target and `continue-on-error` are each asserted |
| An RFC MUST bound to a real `.ci` | `test/plugin/rfc7606-relay-one-field.ci` | Yes -- read in full. It asserts the emitted bytes at `conn=2`, `seq=1`/`seq=2`, so it discriminates the split; the header records the mutation that reddens it |
| A tag in an `unrun` carrier | `rfc_requirements.py:799` + `test_tag_in_unrun_carrier_is_refused` | Yes -- the refusal is in `scan_tree`, on the tree path, not only in the ledger |
| A tag in an interop `check.py` | ~~(pending)~~ `test/interop/scenarios/14-route-server-frr/check.py:8` and `test/interop/scenarios/47-rfc7606-relay-shape-frr/check.py:28` | ~~No -- awaiting the parallel session~~ **Yes (2026-07-29)** -- read both files in full. The end-to-end path is now exercised by real tags, not only by fixtures: the tag is written, `scan_tree` yields it, `evaluate` joins it to a live requirement, `render_ledger` labels it `interop/nightly`, and `check_evidence_ratchet` accepted the rise from 0 to 2 (`--check` exit 0). That is every stage of the Data Flow section driven by one comment |
| The scenario the tag rides on actually asserts the requirement | `14-.../check.py:46` (`bird.check_route_no_as("10.99.0.0/24", "65001")`), `47-.../check.py:91-97` (Path 2 announce present, withdrawn half absent, session up) | Yes -- read both. Scenario 14 asserts AS_PATH transparency **at BIRD**, a foreign daemon, not out of Ze's own RIB; scenario 47 asserts FRR accepted the split output and that the withdrawn half never appeared. Neither is a bare "the process ran" check |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `.et` is verify-tier: `CARRIERS` gives `*.et` -> `editor/verify`; `mk/test-functional.mk:190` lists `editor`, and `ze-functional-test` is in both `stagesForMode` branches |
| A-2 | confirmed | `check.py` is plain Python with `#` comments and no `terminator=`: `test_scan_python_does_not_inherit_terminator` (`:564`) and `test_scan_python_tags_ignores_string_literals` (`:528`) both pass |
| A-3 | confirmed | The carrier list is spelled ONCE: `grep` for every literal suffix check returns no matches; `test_carrier_table_is_single_source` (`:603`) and `test_every_reader_exists` (`:632`) enumerate the readers |
| A-4 | confirmed | `grep -rin 'wire-visible\|classifier' scripts/dev/rfc_requirements.py` -> no matches. The estimate gates nothing |
| A-5 | confirmed | `--selftest` and the workflow Go tests pass with the new job present; `TestEvidenceNightlyIsScheduled` and `TestEvidenceNightlyIsAdvisory` still pass beside it |
| A-6 | confirmed | No pre-existing tag was imported: `--check` reports `editor/verify 0, interop/nightly 0`. The scanner extension added zero unauthored tags |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/functional-tests.md`: BGP interop "fails closed ... exits non-zero naming Docker" | `test/interop/run.py:132-140` -- `if not docker_ok:` then an `error:` message naming Docker and a non-zero exit | Yes |
| `docs/functional-tests.md`: "Runs nightly and advisory in `.github/workflows/evidence-nightly.yml`" | `evidence-nightly.yml:115` job `interop`, `:117` `continue-on-error: true`, `:129` `run: make ze-interop-test` | Yes |
| `ai/rules/testing.md` carrier table (kind/tier per carrier) | `CARRIERS` (`rfc_requirements.py:641`) and the tier constants (`:616-618`); the ledger legend at `ai/RFC-REQUIREMENTS.md:11-18` is derived from the same table | Yes |
| `ai/rules/testing.md` unrun-carrier refusal | `rfc_requirements.py:799` (`if found and carrier.tier == TIER_UNRUN`) | Yes |
| `ai/rules/testing.md` tokenization claim | `scan_python_tags` `rfc_requirements.py:548-553`; verified empirically (see AC Verified) | Yes |
| `ai/rules/CONDENSED.md` regenerated, not hand-edited | `make ze-rules-condensed` -> "wrote ai/rules/CONDENSED.md (95 rules, 363454 chars)"; every new directive re-read in the digest and confirmed intact | Yes |
| Doc rows answered "No" (config, CLI, API, plugin, wire, SDK) | Tooling-only change; no YANG leaf, command, RPC or wire path touched -- the Integration Checklist rows are all No/N-A with reasons | Yes |

## Core Insight

Evidence has two axes that look like one. **Kind** is which layer a test exercises; **tier**
is whether anything executes it. Flattening them is how "we have interop coverage" becomes
true and worthless in the same sentence, and it is why the ordering constraint in this spec
was not pedantry: a tag placed in an unexecuted carrier is strictly worse than the unit tag it
replaces, because it retires real proof in favour of a claim.

The delivered gate closes that for the carriers it knows. What it cannot close is the third
axis the reviews exposed: **tier is asserted, not derived**. `CARRIERS` says `.ci` is
verify-tier because a human read `stagesForMode`; nothing re-derives it, and the ratchet
baseline is re-labelled with the same table it is meant to police. A one-word edit moves every
claim in the repo without reddening anything. Two of the three defects found in review were
instances of the same shape -- a claim of completeness (the sibling audit, the carrier tier)
standing in for the check that would have justified it.
