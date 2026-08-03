# 945 -- tiers-1-rule-and-audit

## Context

The `internal/component/` vs `internal/plugins/` split was historical placement, not a defined rule: edge protocol engines (IS-IS, LDP, RSVP-TE, flow-export, MRT) sat in `component/` while platform plugins (the system RIB, BFD, sysctl) sat in `plugins/`. Phase 0/1 of the tiers umbrella (`spec-tiers-0-umbrella.md`): document the taxonomy and add a machine gate, with NO directory moves. The taxonomy is by dependency direction -- core = library, component = engine other plugins depend on, plugin = edge engine nothing depends on. Only the engine-placement subset ("Path C") is mechanical enough to enforce; core-vs-composition is advisory.

## Decisions

- Chose to enforce ONLY the engine rule (Path C): a `sdk.NewWithConn` engine must be in `component/` if a feature depends on it, else `plugins/`. core/composition reported but not gated, because BGP fuses a codec library with its engine and registration has multiple forms (full rule needs structural preconditions = Path B).
- Chose a transitional **migration baseline** (`scripts/dev/tier_migration_baseline.txt`) over either a red `ze-verify` or a permanent allowlist: it holds the 8 known misplaced engines, the gate fails on NEW misplacements and on STALE entries, so it can only shrink to zero. Each row is tagged with its resolving child spec.
- Chose engine-PACKAGE granularity (the dir containing `sdk.NewWithConn`), with nested sub-plugin namespaces excluded by parsing the generator's `pluginDirs`. This is the only way to separate "edge protocol in the wrong tier" from "BGP sub-plugin correctly nested" (blocker B-1 applies even to Path C).
- Chose to keep the gate in Python (extend `dep_audit.py`) per user directive, with a `--selftest` (project convention, cf. `audit-test-relaxation.py`) plus a thin Go test so it runs under `go test`/`ze-unit-test`.

## Consequences

- `make ze-tier-check` (in `_ze-verify-impl` and the changed variant) now blocks any new config-driven engine landing in the wrong tier; the rule `ai/rules/architecture.md` tells authors where to put new packages.
- The clean enforced set is exactly 8: `isis, ldp, rsvpte, flowexport, mrt` (→ plugins) and `bfd, sysctl, sysrib` (→ component). `mpls` is NOT enforced (it has no `sdk.NewWithConn`; it is a forwarding helper). These moves are child specs tiers-2 (component→plugins) and tiers-3 (plugins→component).
- An empty baseline = full engine-placement enforcement with zero exceptions; the baseline is regenerated after a move with `dep_audit.py --write-baseline`.

## Gotchas

- A naive top-level-dir gate flags `plugins/iface` (a grouping dir whose engine is the nested `iface/dhcp`) because a sibling backend is depended-on. Engine-package granularity with the depended-on check scoped to the engine package's own subtree fixes this -- `plugins/iface` is correctly NOT flagged.
- `mpls` looked like a 5th edge-out candidate but is not an engine; the engine probe is the source of truth, not the directory's apparent role.
- `CLAUDE.md`/`AGENTS.md` are generated from `ai/INSTRUCTIONS.md`, and `ai/rules/INDEX.md` from `rules_index.py`; the Before-You row and rule summary must be added to the sources and regenerated, never hand-edited (`ai/rules/repo-maintenance.md`).
- `--selftest` caught a real bug: `write_baseline` did not create its parent dir.

## Files

- `ai/rules/architecture.md` -- the canonical 3-tier rule + gate + baseline mechanism
- `scripts/dev/dep_audit.py` -- `--check` (Path C gate), `--write-baseline`, `--selftest`, pluginDirs parse
- `scripts/dev/tier_migration_baseline.txt` -- 8 baselined engines (transitional)
- `scripts/dev/dep_audit_gate_test.go` -- Go tests bringing the gate into `go test`
- `Makefile` -- `ze-tier-check` target wired into verify
- `ai/INDEX.md`, `ai/INSTRUCTIONS.md`, `docs/plugin-overview.md` -- discovery pointers + doc note
