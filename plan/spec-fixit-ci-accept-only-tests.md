# Spec: fixit-ci-accept-only-tests

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/test/runner/record_parse.go` - the `.ci` `expect=`/`reject=` grammar
4. `docs/architecture/testing/ci-format.md` - operator-facing `.ci` format reference
5. The affected `test/parse/*.ci` files listed in Current Behavior

## Task

**[MEDIUM]** ~118 of ~1,429 functional `.ci` tests are governed by exit code ALONE:
they assert only `expect=exit:code=0` with no `expect=bgp/json/stdout/stderr/syslog`,
no `reject=`, no `file=/http=/action=`, and no embedded `set -e` shell doing its own
checks. `test/parse/ntp-config.ci` and `test/parse/geodns-config.ci` run
`ze config validate -` and assert only exit 0. These prove a config is ACCEPTED, not
that it parsed to the CORRECT tree: a parser that accepts `interval 300` but stores 0,
or silently drops a `source 0.0.0.0/0` block, still passes green. This is the
functional-suite analog of the repo's known "count-only assertion" mistake class.

Correctly EXCLUDED (NOT weak): tests whose real assertion lives in a `set -e` tmpfs
script (e.g. `test/managed/auth-reject.ci`) and tests using `reject=stdout:pattern=`.

Scope: (1) for parse-acceptance tests that matter, add a round-trip readback step
(`ze config dump -` / `--json`, or `ze config show`) asserting a representative value
via `expect=stdout:contains=` that proves the config parsed correctly; (2) where
value-level coverage already exists in unit tests, explicitly ANNOTATE the `.ci` as
accept-only rather than strengthen it; (3) add a lint that FLAGS a new accept-only
`.ci` so the class cannot grow silently.

Sibling `plan/spec-finish-ci-coverage.md` is about MISSING tests; this spec is about
WEAK ASSERTIONS in EXISTING tests. Reference, do not duplicate.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` - the `.ci` grammar and assertion kinds
  → Constraint: the accept-only annotation marker (step 2 of scope) must be documented here so authors discover it.
  → Decision: strengthen via a readback `cmd=` + `expect=stdout:contains=<value>` rather than inventing a new assertion kind; reuse the existing grammar.
- [ ] `ai/rules/testing.md` + `.claude/rules/memory.md` (count-only assertion entry) - the mistake class this spec generalizes
  → Constraint: an assertion must observe the produced VALUE, not just that the operation did not error.
- [ ] `ai/rules/functional-test-gate.md` - which suites a change must exercise
  → Constraint: strengthened `test/parse/*.ci` run under the existing `ze-test` parse suite; no new runner.

**Key insights:**
- `ze config dump -` reads a config from stdin and prints the PARSED tree (human or `--json`), so a readback assertion observes the stored value (`internal/component/config/cli/cmd_dump.go:18`). `ze config show <file> [path...]` is the path-scoped sibling (`cmd_show.go:19`).
- `ze config validate -` only prints a `configuration valid` banner; `expect=stdout:contains=configuration valid` is STILL accept-only (it observes no parsed value).
- The lint must reuse the SAME accept-only predicate this spec defines, so "strengthened" and "flagged" never disagree.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/runner/record_parse.go` - `parseLine` (:243), `parseExpect` (:451), `parseReject` (:586) populate `Record.ExpectExitCode`, `ExpectStdoutMatch`, `ExpectSyslog`, `RejectStderr`, `FileChecks`, etc.
  → Constraint: an accept-only record is one where the ONLY populated assertion field is `ExpectExitCode == 0` and no tmpfs `set -e` script and no `reject=` is present. This is the exact predicate the lint encodes.
- [ ] `test/parse/ntp-config.ci` - `ze config validate -`, `expect=exit:code=0` only; parses `interval 300`, `max-step 120`, two `server` blocks — none read back.
  → Constraint: representative readback value candidate = `interval 300` (a scalar that a broken parser could store as 0).
- [ ] `test/parse/geodns-config.ci` - `ze config validate -`, `expect=exit:code=0` only; parses two `listener`, two `host-set`, and `source 0.0.0.0/0` — none read back.
  → Constraint: representative readback value candidate = the `source 0.0.0.0/0` catch-all block (a structural element a broken parser could silently drop).
- [ ] `test/managed/auth-reject.ci` - EXCLUDED example: its real check is a `set -e` tmpfs script.
  → Constraint: the lint must NOT flag tmpfs-`set -e` tests as accept-only.
- [ ] `docs/architecture/testing/ci-format.md` - existing assertion kinds and format

**Behavior to preserve:**
- Every currently-passing `.ci` still passes; strengthening ADDS a readback step, it does not change the validated input.
- The `.ci` grammar (`expect=`/`reject=`/`cmd=`) is unchanged; strengthening uses only existing directives.
- Excluded tests (tmpfs `set -e`, `reject=stdout:pattern=`) keep their current shape.

**Behavior to change:**
- Add a readback `cmd=` + `expect=stdout:contains=` to the parse-acceptance `.ci` that matter; annotate the rest as accept-only; add a lint that fails on a NEW unannotated accept-only `.ci`.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Runner (real):** `EncodingTests.Discover` (`record_parse.go:68`) globs `*.ci`, `parseAndAdd` (:111) then `parseLine` (:243) populate the `Record` assertion fields the runner later checks.
- **Readback (real):** a strengthened `.ci` runs `cmd=foreground:seq=N:exec=ze config dump -:stdin=config`; `cmdDump` (`cmd_dump.go:18`) parses stdin and prints the stored tree, which `expect=stdout:contains=<value>` observes.
- **Lint:** a static scan over `test/**/*.ci` classifying each record's assertion fields.

### Transformation Path
1. `.ci` is parsed into a `Record` (which assertion fields are populated).
2. Lint applies the accept-only predicate (only `ExpectExitCode==0`, no tmpfs `set -e`, no `reject=`) and, for a NEW such file lacking the annotation marker, fails.
3. For a strengthened `.ci`: config stdin -> `ze config dump -` -> printed parsed tree -> `expect=stdout:contains=<representative value>` proves the value parsed.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| config stdin ↔ parsed tree | `ze config dump -` renders the stored tree for readback | [ ] |
| `.ci` record ↔ lint gate | accept-only predicate over `Record` assertion fields | [ ] |
| lint ↔ verify pipeline | new lint registered as a Make/verify gate, not run ad hoc | [ ] |

### Integration Points
- The affected `test/parse/*.ci` (and any other suite's accept-only `.ci` that matter), the readback command `ze config dump`/`show`, the new lint script, `docs/architecture/testing/ci-format.md` (annotation marker), and the verify/Make gate that runs the lint.

### Architectural Verification
- [ ] No bypassed layers (readback drives the real `ze config dump` parse path, not a test-only shim)
- [ ] No duplicated functionality (lint reuses the same accept-only predicate the strengthening targets; no second definition of "weak")
- [ ] Registration over hardcoding — the lint is registered as a verify/Make gate and the accept-only annotation is a documented `.ci` marker, not an ad-hoc grep the author must remember (`ai/rules/discovery-updates.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | `ze config dump -` prints the stored value for every scalar/block a readback needs | `cmd_dump.go:18` reads stdin and dumps the parsed tree (human + `--json`) | some values not rendered; use `--json` or `ze config show <path>` | run dump on ntp/geodns configs during implement | unvalidated |
| A-2 | The accept-only predicate is decidable from parsed `Record` fields alone | `record_parse.go` populates distinct fields per assertion kind (:451/:586) | predicate needs raw-text heuristics (tmpfs `set -e`) too | encode predicate + unit test it against ntp/geodns (flag) and auth-reject (no flag) | partially confirmed (tmpfs `set -e` needs a raw-text check) |
| A-3 | Not all ~118 need strengthening; some are unit-covered and annotate instead | scope item 2; count-only rule | over-strengthening duplicates unit coverage | triage the list; annotate where unit tests already assert the value | unvalidated (resolve during implement) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A readback reveals a real parser bug (value stored wrong / block dropped) | strengthened `.ci` fails on first run | Treat as a real defect: fix the parser at its source under this spec, never weaken the new assertion (`ai/rules/no-workarounds-for-missing-behavior.md`) |
| R-2 | Lint predicate mis-flags an EXCLUDED test (tmpfs `set -e`, `reject=`) | auth-reject.ci flagged | Encode the exclusions explicitly; unit-test the predicate against the known excluded example |
| R-3 | Scope creep across all ~118 in every suite | endless triage | This spec targets the parse-acceptance `.ci` that matter now + installs the lint; the lint prevents future growth so a full backfill need not be atomic |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| strengthened `ntp-config.ci` runs `ze config dump -` | -> | readback observes `interval 300` | `expect=stdout:contains=interval 300` in `test/parse/ntp-config.ci` |
| strengthened `geodns-config.ci` runs `ze config dump -` | -> | readback observes the `0.0.0.0/0` source block | `expect=stdout:contains=0.0.0.0/0` in `test/parse/geodns-config.ci` |
| lint scans a new unannotated accept-only `.ci` | -> | lint fails with the file path | `TestCIAcceptOnlyLintFlags` (predicate unit test) |
| lint scans an annotated / strengthened / excluded `.ci` | -> | lint passes | `TestCIAcceptOnlyLintAllows` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `test/parse/ntp-config.ci` after strengthening | validates AND a readback asserts a representative value (e.g. `interval 300`) proving the config parsed correctly |
| AC-2 | `test/parse/geodns-config.ci` after strengthening | validates AND a readback asserts a representative structural element (e.g. `source 0.0.0.0/0`) |
| AC-3 | An accept-only `.ci` that is unit-covered | annotated with the documented accept-only marker (reason cites the covering unit test), not spuriously strengthened |
| AC-4 | A NEW unannotated accept-only `.ci` added to `test/` | the lint FAILS naming the file; an annotated, strengthened, or excluded `.ci` passes |
| AC-5 | Lint | registered as a verify/Make gate and run by `make ze-test`; marker documented in `docs/architecture/testing/ci-format.md` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCIAcceptOnlyLintFlags` | `internal/test/runner/accept_only_lint_test.go` | unannotated accept-only `.ci` is flagged (AC-4) | |
| `TestCIAcceptOnlyLintAllows` | `internal/test/runner/accept_only_lint_test.go` | annotated / strengthened / tmpfs-`set -e` / `reject=` `.ci` pass (AC-4, R-2) | |

### Boundary Tests (predicate edges)
| Field | Case | Classified as |
|-------|------|---------------|
| assertions present | only `expect=exit:code=0` | accept-only (flag unless annotated) |
| assertions present | `expect=exit:code=0` + `expect=stdout:contains=<value>` | strengthened (allow) |
| tmpfs script | `set -e` present | excluded (allow) |
| reject present | `reject=stdout:pattern=` | excluded (allow) |

### Functional Tests
| Test | Suite | Validates | Status |
|------|-------|-----------|--------|
| `test/parse/ntp-config.ci` (strengthened) | parse | AC-1 readback of a scalar value | |
| `test/parse/geodns-config.ci` (strengthened) | parse | AC-2 readback of a structural block | |
| additional `test/parse/*.ci` triaged from the ~118 | parse | AC-1/AC-2/AC-3 (strengthen or annotate) | |

## Files to Modify
- ~~`scripts/dev/verify_ci_assertions.py` (new lint; non-test feature file) - encodes the accept-only predicate + exclusions, fails on a new unannotated accept-only `.ci`; wired as a verify/Make gate (registration over hardcoding)~~
- ~~`mk/` verify wiring (e.g. `mk/test.mk` or the `ze-test`/verify pipeline) - register the new lint gate~~
  - → AUTONOMOUS DEFAULT (2026-07-17): the lint is a **Go predicate + gate test in `internal/test/runner/`**, NOT a standalone Python script + mk wiring. `internal/test/runner/accept_only.go` (new, non-test feature code) exports the accept-only predicate over the real `Record` (`record.go:93`: `ExpectExitCode *int` :140, `ExpectStdoutMatch` :142, `RejectStderr` :134, `TmpfsFiles` :148) plus a raw-text `set -e` scan of `TmpfsFiles`; `internal/test/runner/accept_only_lint_test.go` (new) walks `test/**/*.ci`, reuses `parseAndAdd`/`Record`, and hosts `TestCIAcceptOnlyLintFlags`/`TestCIAcceptOnlyLintAllows`. Rationale: (a) reuses the existing `.ci` parser so there is ONE definition of "weak" (the Architectural Verification bullet forbids a second); a Python scanner would re-implement `.ci` parsing. (b) The TDD and Wiring tables already name Go tests in `internal/test/runner/`; a Python script cannot be exercised by a Go test — this reconciles the spec's internal inconsistency toward its dominant intent. (c) Least-change registration: a test under `internal/test/runner/` runs via `ze-unit-test`, on which `ze-test` already depends (`Makefile:274`), so NO new `mk/` gate wiring is required (satisfies AC-5's "registered as a verify/Make gate and run by `make ze-test`"). (d) `internal/test/runner/accept_only.go` is real `internal/*` feature code, not only tests. Thomas: override if you specifically want a language-agnostic repo-wide Python scanner instead.
- `docs/architecture/testing/ci-format.md` - document the accept-only annotation marker and the readback-strengthening pattern
- `test/parse/ntp-config.ci` - add the readback `cmd=` + `expect=stdout:contains=` (test/, so a script is also listed above)
- `test/parse/geodns-config.ci` - add the readback `cmd=` + `expect=stdout:contains=`
- ~~additional `test/parse/*.ci` (and other suites) triaged from the ~118 - strengthen or annotate~~
  - → AUTONOMOUS DEFAULT (2026-07-17): a full backfill of all ~118 is **NOT required for this spec to close**. This spec strengthens the two canonical parse-acceptance examples (`ntp-config.ci`, `geodns-config.ci`), lands at least one annotation example (AC-3), and installs the lint + marker + doc. Because the lint prevents the class from growing (R-3), the remaining ~116 are a follow-up backfill, not an atomic requirement. Rationale: smaller, self-contained scope per the decision protocol; matches R-3's stated mitigation. Any `test/parse/*.ci` the implementer opportunistically strengthens is a bonus, not a closure gate. Thomas: override if you want the full backfill inside this spec.

## Implementation Steps

### Implementation Phases
1. **Phase: predicate + lint (MANDATORY FIRST)** — encode the accept-only predicate (only `ExpectExitCode==0`, no tmpfs `set -e`, no `reject=`) and its exclusions in ~~`scripts/dev/verify_ci_assertions.py`~~ `internal/test/runner/accept_only.go` (see Files to Modify AUTONOMOUS DEFAULT); add `TestCIAcceptOnlyLintFlags`/`TestCIAcceptOnlyLintAllows` in `internal/test/runner/accept_only_lint_test.go`; ~~register the gate in the verify/Make pipeline~~ the gate is the walk-`test/**/*.ci` assertion inside that test, which runs under `ze-unit-test` (already a `ze-test` dependency, `Makefile:274`) — no new `mk/` wiring.
   - Verify: lint flags a synthetic accept-only fixture, allows ntp/geodns once strengthened, allows `auth-reject.ci`.
2. **Phase: define the annotation marker** — add a documented `.ci` comment marker (e.g. `# accept-only: unit-covered by <test>`) the lint treats as an allowlist entry; document it in `ci-format.md` (registration over hardcoding — discoverable, not a private grep).
3. **Phase: strengthen ntp + geodns** — add `cmd=foreground:seq=N:exec=ze config dump -:stdin=config` and `expect=stdout:contains=<representative value>` to each; confirm the readback observes the parsed value.
   - Verify: both `.ci` pass; deliberately break a value locally to confirm the readback would catch it (R-1 signal).
4. **Phase: triage the rest** — walk the ~118; strengthen the ones that matter, annotate the unit-covered ones (A-3).
5. **Phase: run + triage defects** — if any readback fails, fix the parser at its source (R-1), never weaken the assertion.
6. **Full verification** — `make ze-test` (lint gate + strengthened parse suite).
7. **Complete spec** — audit tables, `plan/learned/NNN-<name>.md`, two-commit closure.

### Discovery-Update Obligation (`ai/rules/discovery-updates.md`)
- New gate: the lint registered in the verify/Make pipeline.
- Author doc: `docs/architecture/testing/ci-format.md` gains the annotation marker + strengthening pattern.
- Rule reinforced: `ai/rules/testing.md` count-only-assertion class, generalized to the functional suite.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint gate + strengthened parse suite)
- [ ] Lint flags a new unannotated accept-only `.ci` and allows annotated/strengthened/excluded ones
- [ ] Registration over hardcoding respected (lint is a registered gate; marker is documented)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary cases (accept-only, strengthened, tmpfs `set -e`, `reject=`) covered

## Notes
- Skeleton captured from the 2026-07-16 repository audit. Finding severity MEDIUM: ~118 of ~1,429 functional `.ci` tests are exit-code-only; `test/parse/ntp-config.ci` and `test/parse/geodns-config.ci` are the canonical examples. Excluded (not weak): tmpfs `set -e` scripts (`test/managed/auth-reject.ci`) and `reject=stdout:pattern=` tests.
- Sibling `plan/spec-finish-ci-coverage.md` covers MISSING tests; this spec covers WEAK ASSERTIONS in EXISTING tests — reference, do not duplicate.
- Verified `file:line`: `.ci` grammar `internal/test/runner/record_parse.go:243/451/586`; readback ~~`internal/component/config/cli/cmd_dump.go:18`, `cmd_show.go:19`~~.
  - → CITATION FIX (2026-07-17): `cmdDump` is at `cmd_dump.go:20` (line 18 is the import close), and it reads stdin + prints the parsed tree exactly as described. `ze config show` stdin readback lives in `openShowEditor` (`cmd_show.go:23`) with command entry `cmdShow` (`cmd_show.go:45`); line 19 was the blank line after imports. Behavior confirmed real; only the line offsets were corrected. The same `cmd_dump.go:18`→`:20` and `cmd_show.go:19`→`cmdShow :45`/`openShowEditor :23` corrections apply to the in-body citations (Key insights, Data Flow Entry Point, Assumption A-1). All `record_parse.go` line numbers (`68`/`111`/`243`/`451`/`586`) are exact.
