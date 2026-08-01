# 983 - feature-gate single-source manifest (feature-gates.txt)

## Context

While completing feature-gate-3-web (web compile-out), the user flagged that the
feature-gate metadata is hardcoded in many places, including the Python. Audit
confirmed: the single fact "tag `ze_<x>` gates package `internal/component/<x>`"
was hand-copied into FIVE disconnected places across four languages:

- `Makefile` `ZE_FEATURES` (the default-on tag list)
- `.golangci.yml` `build-tags` (lint build coverage)
- `internal/test/runner` `TestBuildTags` (functional-test ze binary)
- `scripts/codegen/plugin_imports.go` `featureTags` (gates `<pkg>/yang` into `all_<tag>.go`)
- `scripts/dev/dep_audit.py` `DISABLEABLE` (no always-on import of `<pkg>`)

The construction registry (runtime) was already dynamic, but the BUILD-time wiring
was not. The spec itself documented this as "four-place tag wiring" with a known
trap ("missing the `TestBuildTags` entry... every web `.ci` fails"). With five more
gate specs queued (gnmi/mcp/api/monitoring/protocols), the five-place hand-edit and
its trap were scheduled to repeat five times.

## Decisions

- **One manifest, consumers derive.** Repo-root `feature-gates.txt` holds the fact
  once, `<tag> <pkg>` per line. Adding a gate is a one-line edit there; everything
  else follows by convention: the gated YANG schema is `<pkg>/yang`; tags are
  `ze_<feature>`.
- **Program consumers read the manifest directly** (zero possible drift):
  - generator: `loadFeatureTags(root)` builds `featureTags` (`<pkg>/yang -> tag`).
  - dep_audit: `load_feature_gates(root)` builds `DISABLEABLE` (`<pkg> -> tag`).
  - Makefile: `ZE_FEATURES := $(shell awk '$$1 ~ /^ze_/ {print $$1}' ...)`.
  - test runner: `featureGateTags()` walks up to `go.mod`, reads the first column.
- **The one consumer that cannot self-derive is drift-gated, not hand-trusted.**
  `.golangci.yml` `build-tags` is static YAML (golangci-lint cannot read a file), so
  `dep_audit.py --check` adds `golangci_drift_gate`: the lint build-tags MUST equal
  `ze_core` + every manifest gate tag, and it names the missing tag on failure.
- **A discoverable rule, not tribal knowledge.** `ai/rules/feature-gate-registration.md`
  documents the manifest workflow + the two registration shapes (construction registry
  vs ssh-style seam) + banned patterns; `module-tiers.md`'s stale "five places" text
  was corrected to point at the manifest.

## Reusable lesson

When the SAME fact is hand-copied across build/test/lint/codegen configs in
different languages, make ONE data file the source of truth and have each consumer
DERIVE from it. Program consumers (Go/Python/Make) read it directly; a consumer that
genuinely cannot compute its own value (static YAML/TOML) gets a CI drift gate that
fails loud and names the fix, instead of a checklist comment that rots. The runtime
registry being dynamic does not mean the build-time wiring is: audit both halves.

## Gotchas

- Makefile `$(shell ...)`: a literal `#` in an awk script is parsed as a Make comment
  and truncates the line (`unterminated call to function 'shell'`). Match the `ze_`
  tag prefix (`$$1 ~ /^ze_/`) instead of excluding `#` comments, or escape as `\#`.
- The generator's `featureTags` keys are `<pkg>/yang`; deriving the suffix for every
  manifest line is safe even for a gate with no YANG package, because
  `discoverSchemaPackages` emits no import for it so the entry never matches.
- The pre-existing `// Design:` anchors on the feature-gate service files pointed at
  `docs/architecture/cli/plugin-modes.md` (BGP plugin CLI modes), not the construction
  registry. Repointed to `ai/rules/feature-gate-registration.md` (anchors to `ai/rules/`
  are an accepted convention, e.g. `bufpool.go`).

## Files

None recorded.
