# 1183 -- fixit: fuzz-target discovery (generated enumeration)

## Context

`mk/test-fuzz.mk` enumerated every Go fuzz target by hand, one
`-fuzz=<Name> <pkg>` line each. The list had drifted: 72 distinct `func Fuzz`
names exist across 29 packages under `internal/`, but only ~62 were enumerated.
The gap included all 10 wire-parser drivers that decode unauthenticated
ISIS/OSPF neighbor Hellos and LSPs (`internal/plugins/{isis,ospf}/packet/fuzz_test.go`),
so those fuzzers ran only their seed corpus under plain `go test` and never
entered mutation exploration = false assurance for a remote-DoS-relevant parser.

Spec: `plan/spec-fixit-fuzz-target-discovery.md`. Fix ENUMERATION, not coverage
(the sibling `spec-fixit-parser-fuzz-gaps` adds coverage).

## Decisions

- **Generated-and-verified checked-in fragment, not per-run shell-out.** Chose
  the spec's autonomous default: `scripts/dev/fuzz-targets.py` walks `internal/`
  for `func Fuzz`, resolves each to its exact package dir, and writes the
  committed `mk/test-fuzz-targets.mk` (whole `ze-fuzz-test:` rule). `make generate`
  writes it; `--check` regenerates in memory and diffs the committed copy,
  exiting non-zero with `mk/test-fuzz-targets.mk is stale; run make generate`.
  This mirrors `scripts/codegen/plugin_imports.go --check` for the plugin
  composition root exactly: the exact target set is a reviewable committed
  artifact (auditable CI logs), and a forgotten `make generate` is a hard
  verify-gate red rather than a silently-skipped fuzzer.
- **Full anchoring on every target, not just prefix-colliders.** `render()`
  emits `-fuzz=^<Name>$$` for ALL 72 targets (make-escaped `$$` -> `$` so Go
  fuzz sees the fully-anchored regexp `^Name$`). The old file anchored only the
  three known prefix pairs with a trailing `$$`; uniform `^...$` is simpler and
  provably free of "matches more than one fuzz target" regardless of future
  name collisions. Go's `-fuzz` value is an unanchored regexp (substring match),
  which is exactly why `FuzzParseVPN` would otherwise also select
  `FuzzParseVPNAddPath`.
- **Exact single-package path, never `/...`.** The old file used `./pkg/...`
  for many targets. That is a latent bug for any tree with sibling packages:
  `./internal/plugins/isis/...` matches both `isis/packet` and `isis/yang` and
  Go fuzz errors "matches more than one package". The generator emits the exact
  directory (`./internal/plugins/isis/packet`), so `/...` never appears.
- **Two verify wirings, on purpose.** (1) `ze-fuzz-targets-check` make target
  (sibling of `ze-plugin-imports-check`) runs `--check`; `verify_wiring_docs.py`
  routes it via a new `is_fuzz_source(root, path)` predicate whenever a changed
  `internal/**/_test.go` declares `func Fuzz`, or the mk files / generator
  change (unreadable/deleted `_test.go` routes conservatively so a *removed*
  target still trips a stale fragment). (2) `ze-regen-check` gets
  `mk/test-fuzz-targets.mk` added to its git-diff freshness list, since
  `generate:` now writes the fragment. The `--check`-after-generate pattern used
  by the other regen scripts would be a no-op here (generate just wrote it), so
  the git-diff is what actually gates in that umbrella.
- **Determinism via `sorted(by (pkg, name))`.** Stable, reviewable, groups a
  package's targets together; the committed fragment does not churn.

## Gotchas

- The `include mk/test-fuzz-targets.mk` inside `mk/test-fuzz.mk` resolves
  relative to the make CWD (repo root), NOT the including file -- same as how
  the top Makefile includes `mk/test-fuzz.mk`. `.PHONY: ze-fuzz-test` stays in
  `mk/test-fuzz.mk` even though the rule body now lives in the fragment.
- Recipe lines in the generated fragment MUST be literal-TAB indented; the
  Python emits `"\t$(GO_TEST) ..."`. A space would be a silent make BLOCKER.
- Every emitted package's fuzz test files must build under `GO_TEST_TAGS`
  (`ze_core $(ZE_FEATURES) ...`, which includes `ze_isis`/`ze_ospf`/`ze_vrrp`);
  confirmed ISIS/OSPF compile under those tags. A fuzzer behind a feature tag
  absent from `ZE_FEATURES` would emit a dead invocation -- none today.

## Verification

- `scripts/dev/fuzz_targets_test.py` (run by `python_tests_test.go`): covers-all
  (discovered == grep ground truth, incl. 10 ISIS/OSPF), anchoring, exact path,
  and stale-detection driven through the real `--check` CLI (exit code IS the
  gate). Plus a self-test that the committed fragment is fresh.
- `TestVerifyWiringDocsRoutesFuzzTargetChanges` (Go): a `func Fuzz` test file,
  the mk fragment, and the generator route `ze-fuzz-targets-check`; a non-fuzz
  path does not.
- `make -n ze-fuzz-test` expands to 72 anchored `go test -fuzz=^Name$` lines
  with the feature tags; `make ze-fuzz-targets-check` passes on the fresh
  fragment; no OLD target dropped (diff vs `HEAD:mk/test-fuzz.mk`), exactly the
  10 ISIS/OSPF added.
- NOT run: the bounded `make ze-fuzz-test` mutation pass (72 x 10s) and R-1
  triage -- deferred to CI/drain per the parked-task constraint (no large suites).
