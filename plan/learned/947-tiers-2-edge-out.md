# 947 -- tiers-2-edge-out

## Context

Phase 2 of the tiers umbrella (`spec-tiers-0-umbrella.md`): relocate the five
import-independent edge engines from `internal/component/` to `internal/plugins/`
so the directory tier matches the dependency direction the tiers-1 gate enforces.
The migration baseline (`scripts/dev/tier_migration_baseline.txt`) named the set
authoritatively: `isis, ldp, rsvpte, flowexport, mrt` (NOT `mpls` -- the umbrella
prose said mpls, but mpls has no `sdk.NewWithConn`, so the engine gate never tracked
it; the Mistake Log correction to `mrt` is the source of truth). The move had to be
performed by the Python tool `scripts/dev/migrate_module.py`, not hand edits.

## Decisions

- Moved exactly the 5 baselined engines via `migrate_module.py <name> --to plugins
  --apply --allow-rpc-drop`, one at a time, each regenerating `all.go` through the
  generator. `mpls` stays in `component/` (forwarding helper, not an engine).
- Used `--allow-rpc-drop` after PROVING the drop was safe: the four RPC-bearing
  roots (isis/ldp/rsvpte/flowexport carry `pluginserver.RegisterRPCs` in their *root*
  package `cmd_show.go`, not a `cmd/` subpackage) are re-discovered by `discoverPlugins`
  once they live under the `internal/plugins` whole-tree pluginDir. So the RPC section
  loses the entry but the same package reappears in the plugin section -- registration
  preserved. The generator's `rpcRoot` did NOT need widening.
- Verified registration invariance by set-diffing `all.go`'s blank imports before/after
  (normalising the 5 moved prefixes back to component): 0 dropped. This set-diff, not
  the tool's conservative RPC guard, is the real behaviour-preserving proof.
- Re-sorted imports with `goimports -local github.com/ze-software/ze` after the
  move: the string rewrite changed alphabetical order (`component` < `plugins`), which
  `go build` ignores but the `goimports` golangci formatter enforces.
- Fixed stale path references in EVERY non-prose surface (`.go` comments, `.sh`, `.mk`,
  `.ci`, `.yang`, `docs/*.md`, `rfc/`), regenerated the two generated indexes
  (`ai/INSTRUCTIONS.md` arch lists via `arch_map.py`; `ai/CODE-TO-DOCS.md` via
  `code_to_docs.py`). Left `plan/` specs (other-owned) untouched.

## Consequences

- `isis, ldp, rsvpte, flowexport, mrt` now under `internal/plugins/`; the migration
  baseline shrank from 8 to 3 (only `bfd, sysctl, sysrib` -> tiers-3 remain);
  `dep_audit.py --check` is green with those 3 baselined.
- `mrt` is the one intended behaviour delta: it was registered ONLY via a direct
  `cmd/ze/hub` import (its root was absent from `all.go`); now under the plugins
  pluginDir it is discovered by `discoverPlugins` and appears in `all.go` (plugin count
  93->94). It becomes a first-class plugin registered in every binary importing
  `all.go`, not just hub. Registration is idempotent; engines only start on their
  ConfigRoots, so this is a promotion, not a regression.
- `isis/cli` and `isis/transport` also newly appear in `all.go` -- cosmetic only:
  both already executed their `init()` before the move (transport via the isis root's
  imports, cli via the `cmd/ze/ze_core_dispatch.go` blank import).
- `go build ./...`, moved-package unit tests, generator `--check`, the placement gate,
  and golangci-lint on the moved trees + importers all pass.

## Gotchas

- The tool's RPC-drop guard is deliberately conservative (flags any `RegisterRPCs` in
  a tree moving out of component). For root-package RPC registrations it is a false
  alarm -- the package survives via `discoverPlugins`. Always confirm with the `all.go`
  set-diff before passing `--allow-rpc-drop`; never trust the guard's pessimism OR
  optimism alone.
- The string-only import rewrite breaks import-group sort order. `goimports -w` with
  the project's `-local` prefix is mandatory after the move, else the lint gate fails
  even though the build is green.
- Two files (`mk/test-integration.mk`, `docs/functional-tests.md`) carried both the
  migration's path fixes AND a concurrent user's uncommitted work. They were EXCLUDED
  from the migration commit; their path fixes stay on disk for the owner to commit. A
  shared working tree forces this kind of split.
- `arch_map.py` and `code_to_docs.py` own generated lists -- never hand-edit; regenerate.
  `mpls` paths must NOT be swapped (it did not move); the rewrite regex excluded it.
- goimports vs generated `all.go`: running `goimports -w` over a target set that
  INCLUDES `all.go` strips the trailing blank line the generator emits, so the
  byte-exact `plugin_imports.go --check` (and `make ze-verify`) fails even though
  the build is green. Exclude `all.go` from goimports (it is generated; golangci-lint
  already exempts `// Code generated` files), or re-run the generator last. The
  hardened tool now excludes it.
- Review hardening of `migrate_module.py` (findings ISSUE-2/3/4, NOTE-5): the
  residual scan now covers all text surfaces (`.ci/.yang/docs/.go-comments`, not just
  scripts/mk); `--apply` runs goimports and diffs the `all.go` blank-import set to
  prove 0 registrations dropped; a Go wrapper (`migrate_module_test.go`) runs
  `--selftest` under `go test`.
- Shared-tree hazard (review BLOCKER-1): a concurrent commit while a tree is
  mid-move can leave HEAD missing files that survive (untracked) at the new path.
  `git add -A -- <new-tree>` recaptures them, but verify count old-vs-new and that
  the package still builds before committing.

## Files

- `scripts/dev/migrate_module.py` -- the deterministic mover (FS move + import rewrite
  + pluginDirs edit + generator run + goimports re-sort + all.go set-diff; dry-run
  default; `--selftest`; RPC-drop guard; repo-wide residual scan)
- `scripts/dev/migrate_module_test.go` -- Go wrapper running `--selftest` under `go test`
- `scripts/dev/tier_migration_baseline.txt` -- shrunk 8 -> 3
- `internal/plugins/{isis,ldp,rsvpte,flowexport,mrt}/` -- moved trees
- `internal/component/plugin/all/all.go`, `scripts/codegen/plugin_imports.go` -- regen + pluginDirs
- `cmd/ze/{ze_core_dispatch.go,hub/main.go}`, `internal/component/config/isis_*_test.go` -- importers re-sorted
- `ai/INSTRUCTIONS.md`, `ai/CODE-TO-DOCS.md`, `docs/*`, `rfc/short/rfc905.md`, `test/*.ci` -- reference updates
