# 948 -- tiers-3-platform-in

## Context

Phase 3 of the tiers umbrella (`spec-tiers-0-umbrella.md`): relocate the three
config-driven engines that a feature DEPENDS ON from `internal/plugins/` to
`internal/component/`, so the directory tier matches the dependency direction the
tiers-1 gate enforces (axis B: "a feature depends on it" -> component). The
migration baseline (`scripts/dev/tier_migration_baseline.txt`) named the set
authoritatively: **`bfd, sysctl, sysrib`**. This is the move-IN direction (opposite
of tiers-2's edge-out), so the tool's RPC-drop guard does not apply. Importers prove
the dependency: BGP reactor (`peer_bfd.go`) + static depend on BFD; iface + firewall
on sysctl; the fib backends (kernel/p4/vpp) + fakefib on sysrib. The move had to be
performed by `scripts/dev/migrate_module.py`, not hand edits.

## Decisions

- Moved `sysctl` and `sysrib` as plain directory moves
  (`migrate_module.py <name> --to component --apply`); each regenerated `all.go`,
  proved 0 registrations dropped, and re-sorted imports with goimports.
- `bfd` was a MERGE. `internal/component/bfd/` already held the BFD CLI command
  (`cmd/`, discovered by the generator's `rpcRoot` scan because it calls
  `RegisterRPCs`), while the engine lived at `internal/plugins/bfd/`. The correct
  end state is the canonical subsystem layout -- engine at the root, CLI in `cmd/` --
  so the engine's files merged INTO `internal/component/bfd/` next to the existing
  `cmd/`. There was zero file-level conflict (the engine has no `cmd/`; the target
  had only `cmd/`).
- Extended `migrate_module.py` for this: `find_source` disambiguates a both-areas
  name via `--to` (source = the other area); a new conflict-checked merge path
  (`merge_conflicts`/`do_merge`) moves files into an existing destination and
  refuses on any real path collision; dry-run reports `filesystem MERGE`.
- Regenerated the migration baseline (`dep_audit.py --write-baseline`): it shrank
  3 -> 0. `dep_audit.py --check` now reports "engine placement clean; no exceptions
  (baseline empty)" -- the umbrella's stated end state for engine placement.

## Consequences

- `bfd, sysctl, sysrib` now under `internal/component/`; `pluginDirs` gained the
  three explicit `internal/component/<name>` entries (the whole-tree
  `internal/plugins` entry no longer covers them). `all.go` registration set
  preserved (0 dropped, 0 added) for every move.
- `go build ./...`, moved-package + importer unit tests, generator `--check`,
  golangci-lint (0 issues), `go test ./scripts/dev/`, and the two doc-index
  `--check`s all pass. 122 stale path refs fixed across `.go` comments / docs /
  `.mk` / `.ci` by a deterministic one-off fixer; `plan/` + `.claude/plan/` left
  untouched (owned by other work / historical).
- The empty baseline means any FUTURE engine landing in the wrong tier fails
  `make ze-verify` (`ze-tier-check`) with no grace -- the transitional period for
  engine placement is over.

## Gotchas

- Registration-preservation proof, forward vs backward: the tool normalised the
  post-move `all.go` blank-import set BACKWARD (dst->src) to diff against the
  pre-move set. For a MERGE that is wrong -- it remaps a pre-existing destination
  path (`component/bfd/cmd`, which never moved) back to a source path that was never
  in the before-set, producing a FALSE "dropped registration". Fix: normalise the
  BEFORE set FORWARD (src->dst); forward only touches the genuinely-moved subtree and
  leaves pre-existing destination paths put. Both directions use the same
  boundary-safe regex (`(?![A-Za-z0-9-])`) so a sibling like `<name>-cmd`/`<name>2`
  is never rewritten. Covered by a selftest merge fixture.
- A behaviour-preserving package move trips the changed-file-aware wiring gate
  (`verify_wiring_docs.py`). `check_wiring` reads the baseline via
  `git show HEAD:<current-path>`; for a moved file the NEW path has no HEAD content,
  so `old_names` is empty and EVERY exported symbol looks "added" -- surfacing every
  unwired helper/const the package carries. Fix at the gate (not a workaround):
  collect exported symbols REMOVED from deleted files in the same change and treat
  them as pre-existing relocations. This is general (helps every tier move) and still
  flags genuinely-new unwired symbols. Regression test added.
- The `.ci`-sleep ratchet (`test/.ci-sleep-baseline`) fires whenever ANY `test/*.ci`
  is in the change, then counts the WHOLE working tree. Touching `.ci` files for
  path fixes triggered it and exposed a stale baseline: committed HEAD already held
  424 sleeps while the baseline said 423. Corrected to 424 (no new sleep added by
  this work; working-tree count == HEAD count).
- Pre-existing, NOT tiers-3: `make ze-validate-commands` reports 5
  `ze-firewall-irr-cmd` commands with no handler (committed firewall-IRR feature).
  The `all.go` diff shows firewall/irr untouched and the validator lists zero
  bfd/sysctl/sysrib problems, so the migration adds nothing -- but the gate, once
  triggered by this change's command-source files, surfaces the firewall-IRR gap,
  which blocks `ze-verify` until the firewall-IRR feature closes it.

## Files

- `scripts/dev/migrate_module.py` -- merge support (find_source `--to`
  disambiguation, `merge_conflicts`/`do_merge`, `is_merge`/`conflicts` in the plan,
  merge-aware apply) + forward-normalised registration set-diff (`norm_fwd`)
- `scripts/dev/migrate_module_test.go` -- unchanged (runs `--selftest`, now covering
  the merge + conflict + forward-norm fixtures)
- `scripts/dev/verify_wiring_docs.py` + `_test.go` -- rename/relocation-aware wiring
  check + regression test
- `scripts/dev/tier_migration_baseline.txt` -- shrunk 3 -> 0 (empty)
- `test/.ci-sleep-baseline` -- 423 -> 424 (stale-baseline correction)
- `internal/component/{bfd,sysctl,sysrib}/` -- moved trees (bfd merged with `cmd/`)
- `internal/component/plugin/all/all.go`, `scripts/codegen/plugin_imports.go` -- regen + pluginDirs
- importers re-sorted: `cmd/ze/{hub/main_system.go,ze_core_dispatch.go}`,
  `internal/component/{firewall/engine.go,iface/config_sysctl.go,iface/register.go,bgp/reactor/*}`,
  `internal/plugins/fib/**`, `internal/plugins/static/*`, `internal/test/plugins/fakefib/*`
- `ai/INDEX.md`, `ai/INSTRUCTIONS.md`, `ai/CODE-TO-DOCS.md`, `docs/**`, `mk/test-fuzz.mk`, `test/**/*.ci` -- reference updates
