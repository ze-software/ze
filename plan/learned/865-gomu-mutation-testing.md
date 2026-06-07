# 865 -- gomu-mutation-testing

## Context

Ze's 14,149 test functions provide coverage metrics but no measure of test quality. Mutation testing fills that gap by verifying tests actually detect code changes. Three Go mutation tools were evaluated: Gremlins (most mature, no parallelism safety), go-mutesting (most operators, no parallelism at all), and gomu (overlay-based, parallel-safe, incremental). gomu was chosen for its architecture despite being the youngest tool.

## Decisions

- Chose gomu over Gremlins: overlay-based execution (no in-place file modification) makes it safe for parallel workers; Gremlins rewrites files on disk
- Chose gomu over go-mutesting: go-mutesting has zero parallelism, making it unusable at Ze's scale
- Advisory-only over gating on threshold: gomu is under 1 year old with a single maintainer; false positives would erode trust before a baseline is established
- Thin Makefile wrapper over wrapper script: follows existing `mk/test-unit.mk` pattern where logic lives in the mk file
- Dropped per-component-group mutation targets: gomu has no package-path CLI argument; it always scans the entire module from the working directory
- `.gomuignore` path exclusion over waiting for `--tags` support: gomu has zero build tag support (no `--tags` flag, no `//go:build` inspection); path exclusion is the only workaround

## Consequences

- `cmd/ze/` files get zero mutation coverage because they use mutually exclusive build tags that gomu cannot handle
- gomu's `--incremental` + `--base-branch` maps well to Ze's `changed-pkgs.sh` workflow for scoped CI runs
- `GOMU_TIMEOUT=120` (vs gomu's 30s default) was set to accommodate bgp group test suites (~1:30); may need further tuning
- Contributing a `--tags` flag upstream is a natural follow-up (2 lines in gomu's engine.go + 1 CLI flag)

## Gotchas

- Make recipe `exit 0` in a `define`/`$(call)` block only exits that subshell; subsequent `@`-prefixed lines run as separate subshells and still execute. All logic must be in a single shell block (`;\ ` continuations) for early exit to work.
- gomu writes reports to the current directory by default, not a configurable output path. Reports must be moved to `tmp/` after each run.

## Files

- `mk/test-mutation.mk` (created) -- mutation testing make targets
- `.gomuignore` (created) -- exclusion patterns for gomu
- `Makefile` (modified) -- added `include mk/test-mutation.mk`
- `ai/INDEX.md` (modified) -- added mutation testing to Dev Tools and keyword map
- `ai/rules/testing.md` (modified) -- added Mutation Testing section
