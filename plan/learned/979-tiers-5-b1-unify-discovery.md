# 979 - tiers-5 B-1: unify plugin discovery in the placement audit

## Context

The module-tier umbrella's Path B (full core/composition enforcement) is blocked by
three structural issues (`plan/spec-tiers-0-umbrella.md` "Phase 5 Hardening
Analysis"). Blocker B-1: the audit had no reliable "is this a plugin?" signal -- a
read-only probe that grepped per registration mechanism (registry.Register /
RegisterRPCs / RegisterBackend / sdk.*) produced 65 false mismatches, dominated by
`*-cmd` verb providers (which match none of those) being mis-sent to `core`. The
committed `dep_audit.py` advisory side-stepped this by labelling every 0-external dir
a "plugin candidate", which conflated genuine edge plugins, core libraries, and host
services.

## Decisions

- Chose to derive "wired as a plugin" from the **composition roots** -- the generated
  `internal/component/plugin/all/all.go` AND `cmd/ze` dispatch -- via the existing
  `is_registration_importer`, rather than parse `all.go` alone or grep per mechanism.
  A subsystem is wired iff `len(registration) > 0` (a composition root blank-imports
  something under it). This is the single signal that catches every plugin shape,
  including `*-cmd` providers wired only through dispatch.
- Chose to ADD advisory fields (`is_registered`, `is_engine`, `core_candidate`) and
  re-label the human report (REGISTERED PLUGINS / CORE CANDIDATES / SHARED LIBRARIES)
  rather than change the enforced engine gate, which already reuses the generator's
  `pluginDirs` and stays untouched. B-1 improves only the advisory; core/composition
  is still NOT gated (Path C).
- Chose to wire `dep_audit.py --selftest` into `make ze-tier-check`. The gate ran only
  `--check` before, so its own fixtures (engine placement, and now B-1 classification)
  never executed in verification.

## Consequences

- Plugins-area core candidates dropped 10 -> 1 (only `ifacenetlink`, a genuine
  registration=0/external=0 leaf, remains): the dispatch-wired command plugins are now
  correctly REGISTERED PLUGINS, not core-move candidates.
- The advisory is now trustworthy enough to drive later enforcement, but enforcement
  still waits on B-2 (BGP fuses a codec library with its engine, so "imports bgp"
  cannot tell codec use from engine dependence) and B-3 (host-services need a tier
  decision: a 4th `internal/host/` tier or fold-to-core).
- Any future audit that needs "is X a plugin?" must read the composition roots, never
  a per-mechanism registration grep.

## Gotchas

- An all.go-only parse is NOT sufficient: CLI command plugins marked `codegen:skip`
  are wired via `cmd/ze` dispatch, not `all.go`, so an all.go-only signal mis-sends
  them to core. The dispatch root is a first-class composition root and must be
  included (it already is, in `is_registration_importer`).
- `setup_features_*.go` under `cmd/ze` are NOT registration importers (they don't
  match the dispatch / `_imports.go` pattern), so `connect`/`local`/`provision`/
  `systemd` still report as SHARED LIBRARIES rather than wired plugins. This is an
  advisory-only imprecision, not a gate failure; revisit if/when enforcement extends.

## Files

- `scripts/dev/dep_audit.py` (classify reuses composition-root wiring; new fields;
  re-labelled report; `--selftest` B-1 fixtures)
- `Makefile` (`ze-tier-check` now runs `--selftest` before `--check`)
- `ai/rules/architecture.md` (advisory "is a plugin" half is now mechanical)
- `plan/spec-tiers-0-umbrella.md` (tiers-5 B-1 progress recorded)
