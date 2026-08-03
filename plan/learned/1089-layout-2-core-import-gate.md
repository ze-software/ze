# 1089 -- layout-2-core-import-gate

## Context

`internal/core/` is the leaf tier but nothing enforced import direction out of
it: 5 files (10 import pairs) already reached into `internal/component/`, and
nothing stopped new ones. Separately, `plugin_process_boundary.go` hardcoded two
scan roots while `sdk.NewWithConn` engines also lived under
`internal/component/l2tp/plugins/` (4) and `internal/component/firewall/plugins/`
(1), so the silent-no-op-when-external bug class was unchecked there. Child 2 of
the spec-layout umbrella; both gaps closed as additive, baseline-protected gates.

## Decisions

- Extended `dep_audit.py --check` with a core import-direction gate over the
  existing `collect_edges` graph, over adding golangci `depguard` or a new
  script: one enforcement home, one import parser, already wired into
  `make ze-verify` via `ze-tier-check` (Makefile:288-290).
- Baseline (`scripts/dev/core_import_baseline.txt`) is PAIR-granular
  (importer file + imported package), not file-granular, so a baselined file
  gaining a NEW upward import still fails; every row carries a mandatory fix
  route (`hand-fixable` / `generator-fixable` / `needs-design`) and an illegal
  route fails the gate rather than silently baselining.
- Boundary-checker scan roots are DERIVED at runtime from the generator's
  `pluginDirs` + `nestedPluginDomains` (13 roots), over updating the hardcoded
  list: same single-discovery-source rule as tiers blocker B-1. Unparseable
  `pluginDirs` fails loud (exit 2), never scans nothing. `--print-roots` exposes
  the derived set for the parity test.

## Consequences

- New upward imports from core now fail `make ze-verify` naming the file, the
  package, and the rule; the 10 grandfathered pairs can only shrink.
- The widened boundary scan found ZERO unguarded call sites in the newly
  covered namespaces -- the l2tp/firewall gap was real but currently benign.
- Fixing the grandfathered pairs is routed: `resolve.go` hand-fixable,
  `ipc/yang/register.go` via `scripts/codegen/yang_glue.go`, the diagnostic
  `DoctorCheckContext` type coupling needs design (tiers-5 territory).

## Gotchas

- `qos-map`-style token confusion has a twin in anchors: `make ze-doc-test`
  failed on a `*` glob source anchor (`docs/features/rfc-status.md`) that
  `check_path_exists` (code_to_docs.py) never expands -- anchors must name
  a real file or directory, never a glob.
- The pretool hook bans string `+` concatenation even in `//go:build ignore`
  scripts; `path.Join` is the clean way to build derived root paths (and matches
  the generator's own `pluginSearchRoots`).
- `make ze-validate` is red in this environment for 6 pre-existing
  `../gh-pages/` anchors (sibling checkout absent); scope-to-changed applied.

## Files

- `scripts/dev/dep_audit.py` (core_direction_violations/gate + selftest fixtures)
- `scripts/dev/core_import_baseline.txt` (new, 10 rows)
- `scripts/checks/plugin_process_boundary.go` (+_test.go): loadScanRootsFrom,
  --print-roots, derivation selftest
- `ai/rules/architecture.md`, `ai/rules/plugins.md`,
  `docs/plugin-overview.md`, `docs/features/rfc-status.md`
