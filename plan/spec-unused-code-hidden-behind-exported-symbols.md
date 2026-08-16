# Spec: unused-code-hidden-behind-exported-symbols

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/unused-code-hidden-behind-exported-symbols.md` |
| Updated | 2026-08-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`golangci-lint`'s `unused` and `unparam` linters skip an exported top-level
declaration, on the assumption that an external package can reach it. Once
`plan/future/spec-fixit-unexport-package-private-symbols.md` unexports a symbol,
that assumption stops holding, and the linter reports real findings that
were sitting there the whole time, invisible only because the symbol used to
be exported. This is not a rename defect: every rename compiles and every
package's tests stay green. It is a separate, pre-existing backlog of dead
code and needless parameters that the rename work is uncovering as a side
effect.

Reproduction, one instance: `golangci-lint run
./cmd/ze/internal/cmdutil/...` after `PrintCommandList` is unexported to
`printCommandList` in `cmd/ze/internal/cmdutil/cmdutil.go` reports
`func printCommandList is unused (unused)`. Before the rename the same run
reports nothing there, because `unused` does not check exported functions.

Producing function: `printCommandList` itself, in
`cmd/ze/internal/cmdutil/cmdutil.go` (never called, anywhere, once its
export stops protecting it). Two more shapes recur in the table below:
`writeAVPUint8` / `readAVPUint8` in `internal/component/l2tp/avp.go` for the
`unused` case, and `(*Builder).setAIGP` in
`internal/core/bgp/attribute/builder.go` for the `unparam` case (its
`*Builder` return value is never used by either caller).

## Required Reading

### Architecture Docs
- [ ] `ai/rules/simplicity.md` - deleting dead code is a cut, not an addition
  → Constraint: delete only what is confirmed unreachable; never guess
- [ ] `ai/rules/go-standards.md` - naming and API-contract conventions for the
  replacement signatures where an `unparam` finding removes a parameter
  → Constraint: a removed parameter changes every call site in the same commit

**Key insights:**
- The finding only exists BECAUSE `plan/future/spec-fixit-unexport-package-private-symbols.md`
  unexported the symbol. Fixing this spec before that one finishes would chase a
  moving target: more findings appear as more buckets close.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/internal/cmdutil/cmdutil.go` - `printCommandList`, the
  reproduction instance; a design pass must read it and every other row in
  the findings table to confirm each is genuinely unreachable, not reached
  through a build-tag-gated sibling `go vet` compiles but `golangci-lint`
  does not always check the same tag set

**Behavior to preserve:** every renamed symbol keeps its current name and
behavior; this spec is about code the rename made visible, not about the
rename itself.

**Behavior to change:** each flagged declaration is either deleted (if truly
dead) or has its needless parameter/return value removed (if `unparam`), or
is documented as a false positive (e.g. reached only under a build tag
`golangci-lint`'s default run does not set).

## Data Flow (MANDATORY)

### Entry Point
`golangci-lint run ./<pkg>/...`, run against a package after
`plan/future/spec-fixit-unexport-package-private-symbols.md` has unexported its
flagged symbols.

### Transformation Path
1. A symbol is unexported by that spec's `gopls rename -w` step.
2. `golangci-lint`'s `unused` and `unparam` analyzers, which skip exported
   top-level declarations, now evaluate the same declaration.
3. The analyzer finds no caller (`unused`) or an argument/return that never
   varies across callers (`unparam`), and reports it.
4. This spec's design pass reads the declaration, decides AC-1/AC-2/AC-3,
   and either deletes it, trims the signature, or records the false positive.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Lint default tag set ↔ package's full `ze_*` tag set | re-run `golangci-lint` with the package's feature tags before deleting a finding | No |

### Integration Points
- `plan/future/spec-fixit-unexport-package-private-symbols.md` is the producer of
  every finding this spec fixes; this spec cannot start its findings sweep
  until that spec's 8 buckets all close.
- `make ze-lint-changed` / `make ze-precommit-verify` re-run the same linter and are
  the mechanical proof a deletion introduced no new finding.

## Findings collected so far

All found by running `golangci-lint run ./<pkg>/...` on a package right
after `gopls rename -w` unexported the symbols named in
`plan/future/spec-fixit-unexport-package-private-symbols.md`, bucket 3
(`tmp/s/7f238818-c325-45a5-af3d-d8fb11e49a46/bucket-3.tsv`). Every row here
was left untouched: unexporting is a rename, and neither a signature change
nor a deletion is in scope for that spec.

| File | Symbol | Finding |
|------|--------|---------|
| `cmd/ze/hub/listener_migrate.go` | `newListenerMigrator` | `unparam`: parameter `web` always receives nil |
| `cmd/ze/internal/cmdutil/cmdutil.go` | `printCommandList` | `unused` |
| `internal/component/hub/schema.go` | `(*ConfigStore).setLive` | `unused` <!-- doc-links: ignore (internal/component/hub/schema.go was deleted with the orchestrator runtime in 8d92e9fab, so the symbol this row names is already gone) --> |
| `internal/component/hub/schema.go` | `(*ConfigStore).getLive` | `unused` <!-- doc-links: ignore (internal/component/hub/schema.go was deleted with the orchestrator runtime in 8d92e9fab, so the symbol this row names is already gone) --> |
| `internal/component/hub/schema.go` | `(*ConfigStore).getEdit` | `unused` <!-- doc-links: ignore (internal/component/hub/schema.go was deleted with the orchestrator runtime in 8d92e9fab, so the symbol this row names is already gone) --> |
| `internal/component/web/testing/runner.go` | `(*Browser).getText` | `unused` |
| `internal/plugins/debug/profile.go` | `(*Profile).toggleModule` | `unparam`: bool return value never used |
| `internal/core/bgp/attribute/builder.go` | `(*Builder).setAIGP` | `unparam`: `*Builder` return value never used |
| `internal/core/bgp/attribute/builder.go` | `(*Builder).setWire` | `unparam`: `*Builder` return value never used |
| `internal/core/bgp/attribute/builder.go` | `(*Builder).setAtomicAggregate` | `unused` |
| `internal/core/bgp/attribute/aspath.go` | `(*ASPath).checkedWriteToWithContext` | `unused` |
| `internal/core/bgp/attribute/simple.go` | `(*Aggregator).checkedWriteToWithContext` | `unused` |
| `internal/core/bgp/attribute/attribute.go` | `attrWireLenWithContext` | `unused` |
| `internal/core/bgp/attribute/text.go` | `errMissingCommunityValue`, `errMissingLargeCommunityValue` | `unused` |
| `internal/core/bgp/attribute/text.go` | `parseOriginText`, `parseCommunitiesText`, `parseLargeCommunitiesText` | `unused` |
| `internal/component/l2tp/avp_compound.go` | `writeAVPResultCode` | `unparam`: `mandatory` always receives `true` |
| `internal/component/l2tp/avp.go` | `writeAVPUint8`, `readAVPUint8` | `unused` |
| `internal/component/l2tp/handler_registry.go` | `poolStatsProvider` (var), `unregisterPrefixHandler`, `unregisterPrefixReleaser`, `registerPoolStatsProvider`, `getPoolStatsProvider` | `unused` |
| `internal/plugins/iface/dhcp/ifacedhcp.go` | `dHCPConfig` (type) | `unused` -- likely AC-2: its only use is in `dhcp_linux.go`, and every file in this package (test and non-test) carries `//go:build linux`, so a default-tag `golangci-lint` run on darwin sees no reference at all |
| `internal/plugins/isis/circuit/circuit.go` | `(*Circuit).advertisedHoldTime` | `unused` |

This table covers only bucket 3 of 8. The other 7 buckets touch different
packages and are likely to surface the same pattern; this spec's design
phase should re-run `golangci-lint` over every package
`plan/future/spec-fixit-unexport-package-private-symbols.md` touches, once all 8
buckets close, rather than trust this partial list.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | Validation |
|----|-----------|-------|-----------|
| A-1 | Each flagged declaration is genuinely dead, not reached under a build tag `golangci-lint`'s default invocation does not set | `go vet` under every tag set was green for all these packages, so the compiler sees no missing reference; `golangci-lint` ran with default tags only | re-run each finding with the package's full tag set before deleting anything |

### Risks

| ID | Risk | Mitigation |
|----|------|-----------|
| R-1 | Deleting a handler-registry function like `registerPoolStatsProvider` that a future caller (not yet written) was meant to use removes intended API, not dead code | check `git log` / `git blame` for the declaration's age and its sibling `unregisterX` before deleting either half of a pair |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A deletion that was actually reachable under a build tag breaks that platform's build; caught by `go vet` under the package's full tag set before commit |
| How is it reverted? | One package per commit, so one revert per package, same as the parent unexport spec |
| Who else touches this path? | Any session working the parent unexport spec's remaining buckets, which will surface more rows for this table |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-lint-changed` on a package this spec touches | → | the deleted/trimmed declaration | the finding is gone from `golangci-lint` output, and `make ze-unit-pkg-test PKG=<pkg>` still passes |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|--------------------|--------------------|
| AC-1 | A finding in the table above confirmed genuinely unreachable | the declaration is deleted, and its removal costs no caller anywhere in the tree |
| AC-2 | A finding confirmed reachable only under a build tag | recorded as a `golangci-lint` false positive, left in place, no code change |
| AC-3 | An `unparam` finding where the unused parameter/return carries real intent (e.g. an interface it must satisfy) | left in place with the reason recorded, not silently deleted |
| AC-4 | All 8 buckets of the parent spec have closed | this spec's design phase re-runs `golangci-lint` over the full touched-package set before implementation, not just the bucket-3 rows above |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| existing package tests, unchanged, stay green after a deletion | per touched package | AC-1, deletion removed no live path |
| a package's own tests, unchanged, stay green after an `unparam` trim | per touched package | AC-3, the trim removed no live behavior |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `make ze-functional-encode-test` | `test/encode/*.ci` | wire encoding stays byte-identical after `internal/core/bgp/attribute` findings are resolved | |
| `make ze-functional-plugin-test` | `test/plugin/*.ci` | a plugin still loads and answers its commands after `internal/plugins/debug` and `internal/component/l2tp` findings are resolved | |

## Files to Modify

- Each file named in the findings table, pending the design pass's AC-1/AC-2/AC-3 classification of its row.

## Files to Create

- None expected.

## Implementation Steps

1. Wait for every bucket of `plan/future/spec-fixit-unexport-package-private-symbols.md`
   to close, then re-run `golangci-lint` over every package it touched.
2. For each finding: read the declaration, check `git blame` for intent, and
   classify it AC-1, AC-2, or AC-3.
3. Delete (AC-1) or trim the signature (AC-3) one package per commit, the same
   granularity the parent spec used.
4. Record every AC-2 false positive next to the finding it explains, so a
   future lint run does not re-raise the same question.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every row in the findings table is classified AC-1, AC-2, or AC-3 |
| Correctness | A deletion removed no live call path; a trim changed every call site in the same commit |
| Rule: `ai/rules/simplicity.md` | Nothing is deleted on suspicion; each deletion is backed by the full-tag-set `go vet` check |

## Known Limitations

This skeleton is scoped to what bucket 3 (23 packages, 170 symbols) of the
parent unexport spec surfaced. It is not a full inventory of the codebase's
lint-invisible dead code; only symbols the parent spec touches are covered.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes

### TDD
- [ ] Tests written: none new; existing package tests and `make ze-functional-encode-test` /
  `make ze-functional-plugin-test` are the regression net
- [ ] Tests FAIL: not applicable in the usual direction; the failure this spec
  guards against is a deletion that removes a live path, and the full-tag-set
  `go vet` plus the existing suite produce it deliberately
- [ ] Tests PASS (paste output per package)

### Closure
- [ ] Deferral shard row closed
