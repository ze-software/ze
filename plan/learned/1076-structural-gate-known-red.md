# 1076 -- Structural gates are never a known-red

## Context

`make ze-verify` shipped red on `main` for a week. Commit `f5057cd2a` (learned
[[1070-forked-route-install]]) added `internal/plugins/routeinstall`, a pure
client-side library (no `sdk.NewWithConn`, no `init`/`register.go`, imports only
`internal/core/*` + `pkg/plugin/rpc`). The module-tier gate (`dep_audit.py`,
`non_engine_manifest_required`) correctly flagged it as an unclassified non-engine
placement in the plugin tier: a package under `internal/plugins/` that is neither
an engine nor registered needs a manifest row or must move to the mechanical tier.
The single gate red surfaced as two failing stages -- `ze-tier-check` and
`ze-unit-test-cached`'s `TestEnginePlacement`, which both run `dep_audit.py --check`.

The gate DETECTED it every run. It shipped anyway because the deterministic red was
logged in `plan/known-failures.md` as "pre-existing" and the owner-decision punted,
so any session could commit past it with `commit_helper.py --unverified`. The
known-failures / `--unverified` escape hatch was built for **flaky or environmental
TEST reds** (load-sensitive races, GC-pressure pool flakes, host-specific listener
probes) and had no notion of "this red is deterministic and structural."

## Decisions

- **routeinstall moved to `internal/core/rib/routeinstall`** (beside `locrib`, its
  in-process twin), NOT registered as a plugin. It has nothing to register; a
  `register.go` with an empty `init()` + a blank-import into the composition root
  would be a fake registration that violates `plugin-self-containment`. Core is
  outside `dep_audit.py`'s audited areas, so the gate passes with no manifest row
  and no fake wiring. `internal/core/ipc` already imports `pkg/plugin/rpc`, so the
  layering was legal. (HEAD's `ai/DOCS-TO-CODE.md` already referenced the core path
  -- the docs had anticipated the home.)
- **Structural gates are never bypassable.** `commit_helper.py create` now reads
  `tmp/ze-verify-failures.json` (`structural_gate_reds` / `STRUCTURAL_GATES`) and
  refuses to prepare a commit script while any deterministic structural gate is red
  -- `ze-lint`, `ze-tier-check`, `ze-vet-evidence`, `ze-plugin-boundary-check`,
  `ze-iface-resolution-check`, `ze-cli-grammar-check`, `ze-verify-wiring-docs` --
  EVEN with `--unverified`. Only flaky TEST stages stay bypassable.

## Consequences

- `verify_run.go` rewrites `tmp/ze-verify-failures.json` after EVERY run (green or
  red, unconditionally, marshalling all stages with their exit codes), so a green
  verify overwrites a stale red: a fixed-and-reverified gate clears automatically
  and the guard never false-blocks on lingering artifacts.
- Policy is documented where agents read it: `ai/rules/git-safety.md` ("Structural
  Gates Are Never Known-Red") and the `plan/known-failures.md` scope header. The
  file is now scoped to non-deterministic TEST reds only.

## Gotchas

- **A structural gate red is not "pre-existing noise" to scope around.** The
  >10-min / scope-to-changed carve-outs in `git-safety.md` are for flaky tests and
  other sessions' unrelated reds. A tier/lint/vet/boundary red means YOUR tree (or
  the shared tree) is structurally broken; fix it at the source.
- The classification is by STAGE NAME, not by the failure `Kind`: `ze-lint` reports
  `Kind: linter`, `ze-vet-evidence` `Kind: package`, `ze-verify-wiring-docs`
  `Kind: subcheck`, only the bare gates default to `Kind: stage`. Keep
  `STRUCTURAL_GATES` in sync with `verify_run.go` `stagesForMode`.

## Files

- `internal/core/rib/routeinstall/{sink.go,sink_test.go}` (moved from `internal/plugins/`)
- `internal/plugins/{ospf,isis}/spf_wiring.go` (import path), `.../spf/install.go` (comments)
- `scripts/dev/commit_helper.py` (`STRUCTURAL_GATES`, `structural_gate_reds`, `create` gate)
- `scripts/dev/commit_helper_test.go` (`TestCommitHelperStructuralGateNotBypassable`)
- `ai/rules/git-safety.md`, `plan/known-failures/RESOLVED.md`
