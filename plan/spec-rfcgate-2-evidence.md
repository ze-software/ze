# Spec: rfcgate-2 -- wire-level evidence for RFC requirements

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Depends | spec-rfcgate-1-extraction |
| Phase | - |
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
- `ze-rfc-check` stays in both `stagesForMode` branches and keeps its current runtime budget within noise (today ~1.7s at HEAD, ~2.2s with the baseline read).

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
| R-2 | Receiver-side MUSTs no conforming peer can drive (RFC 7606 §5.1 shape) look wire-visible but are untestable by a peer daemon | A scenario author cannot make FRR/BIRD emit the offending message | The injector sidecar already exists (`interop.py:1284-1309`, used by `test/interop/scenarios/47-rfc7606-relay-shape-frr`); the seed bindings must include one injector-driven requirement so the pattern is documented rather than rediscovered (AC-13) |
| R-3 | Vacuity: a `.ci` or interop binding satisfies `ze-rfc-check` whether or not the test discriminates (`ai/rules/interop-and-goal-validation.md`) | A seed binding passes on first run and was never seen red | Every seed binding is mutation-verified: disable the producing function, confirm RED, revert, confirm GREEN, and record the RED output in the closure Goal Validation (AC-14). No binding lands without a recorded red |
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
| AC-9 | A `check.py` that does not parse as Python sits anywhere under a scanned root | The tree scan fails closed with an error naming the file; the baseline path contributes no tags for that file and does not crash |

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
- `ai/rules/testing.md` - RFC-tagged-test section: which carriers may hold a tag, and the verify/nightly tier distinction.
- `ai/rules/rfc-compliance.md` - the four ratchets table gains the non-unit evidence ratchet.
- `docs/features/rfc-status.md` - state that a status row backed only by nightly-tier evidence says so.
- `docs/functional-tests.md` - interop runner now fails closed; interop runs nightly.
- `test/plugin/*.ci`, `test/interop/scenarios/*/check.py` - seed tag comments only.

## Files to Create
- `scripts/dev/rfc_requirements_test.py` - unit tests above (if the module has no sibling test file yet; otherwise extend it).
- `test/interop/run_test.go` - fail-closed runner test (if no suitable Go home exists).
- `plan/deferrals/rfcgate-2-evidence.md` - deferral shard (ipsec/l2tp/pppoe trees, back-fill remainder).

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
| Resource exhaustion | Tokenizing ~104 scenario files plus 164 `.et` files must stay inside the gate's runtime budget; measure before/after and keep `ze-rfc-check` within noise of its current ~2.2s |
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
