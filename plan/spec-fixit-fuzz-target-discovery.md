# Spec: fixit-fuzz-target-discovery

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
3. `mk/test-fuzz.mk` - the hand-maintained fuzz enumeration and runner (`ze-fuzz-test`, `ze-fuzz-one`)
4. Sibling `plan/spec-fixit-parser-fuzz-gaps.md` - the exact-package-path rule this spec generalizes
5. The un-run targets: `internal/plugins/isis/packet/fuzz_test.go`, `internal/plugins/ospf/packet/fuzz_test.go`

## Task

**[MEDIUM]** `mk/test-fuzz.mk` enumerates every fuzz target by hand, one `-fuzz=<Name> <pkg>`
line each (`mk/test-fuzz.mk:14-72`); its own comment concedes "multiple fuzz tests per package
require individual enumeration" (`mk/test-fuzz.mk:10`). The list has drifted: 69 distinct
`func Fuzz` names exist across 26 packages, but only ~60 are enumerated. The gap includes 10
wire-parser drivers that decode unauthenticated ISIS/OSPF neighbor Hellos and LSPs, so a parser
panic is a remote DoS, yet these fuzzers run only their seed corpus under plain `go test` and
never enter mutation exploration = false assurance.

Un-enumerated (verified: `grep isis|ospf mk/test-fuzz.mk` = 0):
- **ISIS** (`internal/plugins/isis/packet/fuzz_test.go`): `FuzzISISDecodePDU` (:48), `FuzzISISTLVIterator` (:91), `FuzzISISRoundTrip` (:145).
- **OSPF** (`internal/plugins/ospf/packet/fuzz_test.go`): `FuzzOSPFDecodePacket` (:7), `FuzzOSPFLSAIterator` (:15), `FuzzOSPFRoundTrip` (:27), `FuzzOSPFTEBody` (:42), `FuzzOSPFRIBody` (:62), `FuzzOSPFExtPrefixBody` (:84), `FuzzOSPFExtLinkBody` (:102).

Replace the hand-maintained list with **generated discovery**: a script walks the tree for
`func Fuzz`, resolves each to its exact package path, and emits one anchored `-fuzz=^<Name>$`
invocation per target. ISIS/OSPF and every future fuzzer are then auto-included by registration,
not by remembering to hand-edit the makefile. This spec fixes ENUMERATION; the siblings
`spec-fixit-parser-fuzz-gaps` and `spec-improve-8-fuzz-decode-context` ADD coverage (no conflict).

## Required Reading

### Architecture Docs
- [ ] `mk/test-fuzz.mk` - the enumeration and the `ze-fuzz-test` / `ze-fuzz-one` runners
  -> Constraint: use an **exact** single-package path per target, never `/...`. `internal/plugins/isis/yang` and `internal/plugins/ospf/yang` exist, so `./internal/plugins/isis/...` matches multiple packages and Go fuzz errors "matches more than one package".
  -> Constraint: within one package a target name may be a prefix of another; emit `-fuzz=^<Name>$` (Go fuzz takes a regexp) or the run errors "matches more than one fuzz target". The current file already does this with `$$` anchors (`mk/test-fuzz.mk:38,41,44`).
  -> Decision: keep the existing 10s-per-target budget (`-fuzztime=10s -timeout=60s`) so the discovery-driven `ze-fuzz-test` stays a short-budget stage suitable for scheduled CI.
- [ ] `plan/spec-fixit-parser-fuzz-gaps.md` - the sibling that established the exact-path rule
  -> Constraint: this spec is the structural fix that makes that sibling's newly-added targets discovered automatically instead of by another hand-edit.
- [ ] `ai/rules/discovery-updates.md`, `ai/rules/testing.md` - Back-Fill New Test Types
  -> Constraint: generated discovery IS a discovery-update: the source of truth becomes "a package has a `func Fuzz`", not a name list.

**Key insights:**
- Registration over hardcoding: a fuzzer is "registered" by existing as `func Fuzz` in a package; the gate discovers it, matching the project's small-core + registration pattern (`ai/rules/plugin-self-containment.md`).
- Scripts are Python by project convention (`scripts/dev/*.py`); the generator is a Python script, not shell, even though `rg '^func Fuzz'` describes the discovery.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `mk/test-fuzz.mk` - `ze-fuzz-test` runs ~60 hand-listed `$(GO_TEST) -fuzz=<Name> -fuzztime=10s -timeout=60s <pkg>` lines (:14-72); `ze-fuzz-one` runs one target (:79-80). No ISIS/OSPF line exists.
- [ ] `internal/plugins/isis/packet/fuzz_test.go` - 3 targets wrapping `DecodePDU` (`internal/plugins/isis/packet/pdu.go:48`) and `NewTLVIterator` (`internal/plugins/isis/packet/tlv.go:57`).
- [ ] `internal/plugins/ospf/packet/fuzz_test.go` - 7 targets wrapping `DecodePacket` (`internal/plugins/ospf/packet/header.go:208`), `NewLSAIterator` (`internal/plugins/ospf/packet/lsa.go:234`), and the TE/RI/ExtPrefix/ExtLink LSA body decoders.
- [ ] `Makefile:67` - `GO_TEST` definition the emitted lines must reuse; `Makefile:274` - `ze-test` includes `ze-fuzz-test`; `mk/test-release.mk:150` - release path calls `ze-fuzz-test`.

**Behavior to preserve:**
- The ~60 targets currently enumerated must still run, with the same 10s/60s budget and exact package paths (anchored where a name is a prefix of a sibling).
- `ze-fuzz-one FUZZ=... PKG=... TIME=...` keeps its manual single-target contract.
- The existing seed corpora and the exported decoder signatures the fuzzers call; this spec changes only how targets are enumerated, not the fuzzers.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- `make ze-fuzz-test` (the fuzz gate, `mk/test-fuzz.mk:12`) is the real entry point. It is what invokes `go test -fuzz=`; a target absent here reaches mutation exploration nowhere in CI. The generator's discovery over `func Fuzz` feeds this gate.

### Transformation Path
1. ~~`ze-fuzz-test` invokes the discovery generator (`scripts/dev/fuzz-targets.py`).~~ → SUPERSEDED (2026-07-17, checked-in default): `make generate` runs `scripts/dev/fuzz-targets.py`, which writes the committed `mk/test-fuzz-targets.mk`; `ze-fuzz-test` `include`s that fragment (no per-run generator shell-out). `fuzz-targets.py --check` runs in the verify gate (`ze-fuzz-targets-check`) to catch a stale fragment.
2. The generator walks `internal/` for `func Fuzz<Name>`, mapping each to the exact package directory that contains it.
3. For each (name, package) it emits `-fuzz=^<Name>$ -fuzztime=10s -timeout=60s <exact-pkg>`, anchoring the name and the package so neither matches "more than one".
4. Each emitted invocation runs `$(GO_TEST) -fuzz=...`; ISIS/OSPF and any future fuzzer are now included with zero makefile edits.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Filesystem <-> generator | walk `internal/` for `func Fuzz`, resolve exact package dirs | [ ] |
| Generator <-> fuzz gate | generator output drives the `ze-fuzz-test` recipe | [ ] |
| Gate <-> parser | each `go test -fuzz=^<Name>$ <pkg>` reaches `DecodePDU`/`DecodePacket`/iterators/LSA bodies | [ ] |

### Integration Points
- `mk/test-fuzz.mk` (recipe now discovery-driven), `scripts/dev/fuzz-targets.py` (new generator), `Makefile` `GO_TEST`, `mk/test-release.mk` (release fuzz stage), `docs/functional-tests.md` (target list / count). Registration over hardcoding: the gate discovers registered `func Fuzz` targets rather than a hardcoded name list.

## Risks & Assumptions

| ID | Assumption / Risk | Basis | If wrong |
|----|-------------------|-------|----------|
| A-1 | Every fuzz target's package resolves to exactly one non-`yang` package dir | `func Fuzz` names all live in leaf packet/decoder packages; `yang` subpkgs hold no fuzzers | generator must skip/deduplicate ambiguous matches |
| A-2 | Anchoring `-fuzz=^<Name>$` reproduces today's `$$`-anchored behaviour for prefix-colliding names | `mk/test-fuzz.mk:38,41,44` already anchor `FuzzParseVPN$$`/`FuzzParseFlowSpec$$`/`FuzzParseBGPLS$$` | un-anchored regexp errors "matches more than one fuzz target" |
| A-3 | ISIS/OSPF parsers are bounds-safe today; discovery gives regression protection, not a live crash fix | code reasoning per decoder (resolve during implement) | R-1: a run finds a reachable panic |
| R-1 | Enabling mutation on a never-fuzzed ISIS/OSPF target surfaces a real panic | crash on first `ze-fuzz-test` run | treat as a real defect: add the crashing seed to `testdata/fuzz/<Name>/` and fix the parser here |
| R-2 | Dynamic generation makes the recipe opaque / non-reproducible in CI logs | reviewer cannot see which targets ran | RESOLVED (2026-07-17): checked-in generated-and-verified `mk/test-fuzz-targets.mk` (see open-question default in Implementation Steps) makes the target set a committed, reviewable artifact; the `--check` freshness gate turns drift into a hard red |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `make ze-fuzz-test` | -> | discovery emits an invocation for every `func Fuzz` incl. all 10 ISIS/OSPF | `TestFuzzDiscoveryCoversAllTargets` |
| generator on a prefix-colliding name | -> | emits `-fuzz=^<Name>$` (anchored) | `TestFuzzDiscoveryAnchorsNames` |
| generator on a package with a `yang` sibling | -> | emits the exact packet-package path, never `/...` | `TestFuzzDiscoveryExactPackagePath` |
| `make ze-fuzz-targets-check` on a stale fragment | -> | `--check` regenerates in memory, diffs the committed `mk/test-fuzz-targets.mk`, exits non-zero | `TestFuzzDiscoveryCheckDetectsStale` |

Concrete test: `TestFuzzDiscoveryCoversAllTargets` (Python `scripts/dev/fuzz_targets_test.py` or a Go test invoking the generator) asserts the emitted set of `Fuzz*` names equals `grep -rl '^func Fuzz' internal/` resolved to names, and specifically contains `FuzzISISDecodePDU`, `FuzzISISTLVIterator`, `FuzzISISRoundTrip`, and the 7 `FuzzOSPF*`.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-fuzz-test` with the generator wired | every `func Fuzz` in `internal/` is run once, including all 10 ISIS/OSPF targets |
| AC-2 | Two targets in one package where one name prefixes another | each runs via `-fuzz=^<Name>$`; no "matches more than one fuzz target" error |
| AC-3 | A package with a `yang` sibling package | the emitted path is the exact packet package; no "matches more than one package" error |
| AC-4 | A new `func FuzzX` added to any package | it is picked up with no edit to `mk/test-fuzz.mk` (registration over hardcoding) |
| AC-5 | `ze-fuzz-test` budget | stays a short-budget stage (10s/target, 60s timeout) fit for scheduled CI; `docs/functional-tests.md` count reconciled |
| AC-6 | A `func Fuzz` added but `make generate` not run (checked-in default) | `make ze-fuzz-targets-check` (`fuzz-targets.py --check`) exits non-zero with `mk/test-fuzz-targets.mk is stale; run make generate`; after `make generate` it passes — mirrors `plugin_imports.go --check` for `all.go` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFuzzDiscoveryCoversAllTargets` | `scripts/dev/fuzz_targets_test.py` | AC-1, AC-4 | |
| `TestFuzzDiscoveryAnchorsNames` | `scripts/dev/fuzz_targets_test.py` | AC-2 | |
| `TestFuzzDiscoveryExactPackagePath` | `scripts/dev/fuzz_targets_test.py` | AC-3 | |
| `TestFuzzDiscoveryCheckDetectsStale` | `scripts/dev/fuzz_targets_test.py` | AC-6 | |

### Functional Tests
Test infrastructure only; no user-facing features. The discovery-driven gate is verified by a bounded `make ze-fuzz-test` run that reaches every ISIS/OSPF target; no `.ci` functional test applies.

## Files to Modify
- ~~`mk/test-fuzz.mk` - replace the hand-listed `-fuzz=<Name>` block with a discovery-driven recipe (non-test feature file)~~ → SUPERSEDED (2026-07-17, checked-in default): `mk/test-fuzz.mk` replaces its hand-list with `include mk/test-fuzz-targets.mk` and keeps only the `ze-fuzz-one` manual target (non-test feature file)
- `scripts/dev/fuzz-targets.py` - new discovery generator (Python per project convention); write mode (default, via `make generate`) + `--check` mode (freshness gate)
- `mk/test-fuzz-targets.mk` - NEW generated-and-verified checked-in fragment (`# Code generated by scripts/dev/fuzz-targets.py; DO NOT EDIT.`), one anchored `$(GO_TEST) -fuzz=^<Name>$$ ...` line per discovered target; `include`d by `mk/test-fuzz.mk` (non-test feature file)
- `Makefile` - add `ze-fuzz-targets-check` target (runs `fuzz-targets.py --check`, sibling of `ze-plugin-imports-check` at `:102-103`) and call the generator from `generate:` (`:98-100`)
- `scripts/dev/verify_wiring_docs.py` - add the `--check` invocation to the generated-file freshness list (alongside `plugin_imports.go --check` at `:718`) so a stale fragment fails the verify gate
- `docs/functional-tests.md` - reconcile the Fuzz Target Areas list / count to the discovered set (discovery update)
- ISIS/OSPF parser sources - only if R-1 fires and a fuzzer finds a defect

## Implementation Steps
1. **Wiring first** - write `scripts/dev/fuzz-targets.py`: walk `internal/` for `func Fuzz`, resolve each to its exact package dir, emit `-fuzz=^<Name>$ -fuzztime=10s -timeout=60s <pkg>`. ~~Point `ze-fuzz-test` at it.~~ → (2026-07-17 checked-in default) have the generator WRITE `mk/test-fuzz-targets.mk` (with `$$`-escaped anchors) and add `--check`; call it from `make generate`; make `mk/test-fuzz.mk` `include` the fragment; add `ze-fuzz-targets-check` to the verify path (AC-6). Confirm all 10 ISIS/OSPF targets appear (AC-1).
2. **Anchoring + exact path** - handle prefix-colliding names (A-2) and `yang`-sibling packages (A-3); add the three discovery unit tests.
3. **Reconcile docs** - update `docs/functional-tests.md` list and count to the discovered set (discovery-updates rule).
4. **Run + triage** - bounded `make ze-fuzz-test`; if a crash surfaces (R-1), add the seed under `testdata/fuzz/<Name>/` and fix the parser here.
5. **Verify + close** - `make ze-test`, `plan/learned/NNN-<name>.md`, two-commit closure.

**Open question (resolve in `/ze-spec`):** dynamic run-time discovery (recipe calls the generator each run) versus a generated-and-verified checked-in `.mk` fragment gated for freshness like the composition root. Dynamic auto-includes with zero drift; generated-and-verified keeps CI logs explicit (R-2).

→ AUTONOMOUS DEFAULT (2026-07-17): the **CHECKED-IN generated-and-verified `.mk` fragment**. `scripts/dev/fuzz-targets.py` (a) in write mode (default, invoked from `generate:` at `Makefile:98-100`) emits `mk/test-fuzz-targets.mk` — a committed fragment with header `# Code generated by scripts/dev/fuzz-targets.py; DO NOT EDIT.` and one anchored `$(GO_TEST) -fuzz=^<Name>$$ -fuzztime=10s -timeout=60s <exact-pkg>` line per discovered target (`$$` = make-escaped `$`, as `mk/test-fuzz.mk:38` already does); and (b) in `--check` mode regenerates in memory and diffs the committed fragment, exiting non-zero with `mk/test-fuzz-targets.mk is stale; run make generate` on drift. This mirrors `scripts/codegen/plugin_imports.go --check` for the composition root `internal/component/plugin/all/all.go` exactly (message at `scripts/codegen/plugin_imports.go:583,668`; gate target `ze-plugin-imports-check` at `Makefile:102-103`; freshness wiring at `scripts/dev/verify_wiring_docs.py:718`). `mk/test-fuzz.mk` replaces its hand-list with `include mk/test-fuzz-targets.mk` and keeps only `ze-fuzz-one`; a new `ze-fuzz-targets-check` target (sibling of `ze-plugin-imports-check`) runs `--check` and is added to the verify path. Rationale: reproducible and auditable — the exact target set is a committed, reviewable artifact visible in CI logs (answers R-2), a forgotten `make generate` becomes a hard verify-gate red rather than a silently-skipped fuzzer, and it reuses the repo's established generated-and-verified pattern instead of an opaque per-run shell-out; it is the more conservative/auditable option. Thomas: override to pure dynamic run-time discovery if you prefer zero committed fragment and accept opaque recipe logs. Dependent sections (Data Flow, Wiring, AC, Files, Implementation Steps) reconciled below.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete - every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] `make ze-fuzz-test` runs every discovered target, including all 10 ISIS/OSPF
- [ ] Registration over hardcoding respected (discovery, not a name list)
- [ ] Discovery update done (`docs/functional-tests.md`)

### Quality Gates (SHOULD pass - defer with user approval)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary seeds (zero-length, truncated, oversized) present in the covered corpora

## Notes
- Skeleton captured from the 2026-07-16 repository audit. Verified: `mk/test-fuzz.mk` has 0 ISIS/OSPF references; 69 distinct `func Fuzz` names across 26 packages vs ~60 enumerated; `internal/plugins/{isis,ospf}/yang` exist (the `/...` hazard). Siblings `spec-fixit-parser-fuzz-gaps` and `spec-improve-8-fuzz-decode-context` add coverage; this spec fixes enumeration so written fuzzers actually run.
- 2026-07-17 recount (append-only; the numbers drifted since the 2026-07-16 audit, which is itself the bug this spec fixes): `grep -rhoE '^func Fuzz[A-Za-z0-9_]+' internal/ | sort -u` = 72 distinct names across 29 packages; `mk/test-fuzz.mk` enumerates 63 `$(GO_TEST) -fuzz=` lines; ISIS/OSPF references still 0; the 10 ISIS/OSPF targets are present in source (`internal/plugins/isis/packet/fuzz_test.go`, `internal/plugins/ospf/packet/fuzz_test.go`). `docs/functional-tests.md:1647` still says "57 fuzz targets" — also stale (AC-5 reconciles it). Grounding for the checked-in default: `scripts/codegen/plugin_imports.go --check` (stale message `:583,668`), `Makefile:98-103` (`generate:` / `ze-plugin-imports-check`), gate wiring `scripts/dev/verify_wiring_docs.py:718`.
