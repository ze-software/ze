# Spec: fixit-unexport-package-private-symbols

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 7/8 (bucket 7 is the remainder) |
| Updated | 2026-08-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Provenance

Reclassified as an improvement on 2026-08-14 at Thomas's instruction and moved
from `plan/` to `plan/future/`. Reason: `plan/future/README.md` names this exact
work as its own example of a cleanup with no wire-visible or operator-visible
effect. Every change is a rename, so no behaviour moves.

## Task

`./le repository check` reports 467 findings of the form `exported symbol X has no
cross-package non-test caller`. They are true: each names a symbol that is
exported but reached only from inside its own package. `check_cross_package_wiring`
in `internal/le/doc/wiring/checks.go` already suppresses the known false-positive shapes
(`*ForTest`, a type reached through its constants or as a struct field, a method
on an unexported receiver reached by interface dispatch), so what remains is a
real backlog of over-exported API surface.

The fix for each is one rename: `Foo` becomes `foo`. Nothing is deleted.

**This task is written to be executed by a small model.** Every trap is a
pre-check that HALTS, never a judgement. The safety mechanism is `gopls rename`
itself, which is type-aware across packages and test files and REFUSES an unsafe
rename with the reason. A refusal is a skip, not a problem to solve.

Worked example of a refusal, run against this tree:

```
$ gopls rename internal/component/config/transaction/solver.go:17:6 topologicalSort
gopls: transaction/solver.go:17:6: renaming "TopologicalSort" to "topologicalSort"
  would make it unexported
iface/operation_address_swap_test.go:29:20: breaking references from packages
  such as "github.com/ze-software/ze/internal/component/iface"
```

That symbol is skipped. No edit, no decision.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/commands.md` - which commands are allowed, and why a bare `go build` is refused by a hook
  → Constraint: use `go vet` to compile-check; `go build` without `-o bin/` is blocked
- [ ] `ai/rules/go-standards.md` - naming
  → Constraint: an unexported name keeps the same spelling with a lower-case first letter

**Key insights:**
- `gopls rename` uses one build context at a time. A reference that exists only
  in a `_linux.go` file is invisible to it on a darwin host, so the `GOOS=linux`
  compile in step 4 is what catches that, not the rename.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/doc/wiring/checks.go` - `check_cross_package_wiring()` produces the findings, `_has_cross_pkg_ref()` decides "wired"
- [ ] `internal/le/` native action tables - `ZE_FEATURES` at line 87 and the tag sets at 239, 243, 262

**Behavior to preserve:**
- Every symbol keeps its behavior. This is a rename, never a deletion, never a signature change.
- Generated files are not edited.

**Behavior to change:**
- 400-odd exported symbols become package-private.

## Data Flow (MANDATORY)

### Entry Point
- `./le repository check` reports the findings. The worklist is derived from its log.

### Transformation Path
1. Parse the log into rows of `file`, `line`, `symbol`.
2. `gopls rename -w` each one, or record its refusal.
3. Compile and test the package under every tag set and both operating systems.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| darwin build view ↔ linux build view | `GOOS=linux go vet` after the rename | No |

### Integration Points
- `internal/le/doc/wiring/checks.go` `check_cross_package_wiring()` - the same check verifies the fix

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | N-A | rename only |
| No unintended coupling (components stay isolated) | N-A | reduces surface |
| No duplicated functionality (extends existing, does not recreate) | N-A | no new code |
| Zero-copy preserved where applicable | N-A | no wire path |
| Registration over hardcoding | N-A | no plugin surface |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `gopls rename` refuses every unsafe rename rather than producing a broken tree | run against `TopologicalSort`, which it refused, naming the external test package | a broken build reaches a commit | the refusal above, plus step 4 compiling under every tag set | confirmed |
| A-2 | A reference visible only under a different GOOS is caught by the compile, not the rename | `gopls` uses one build context | a linux-only reference breaks the linux build | `GOOS=linux go vet` in step 4 | confirmed: `internal/chaos/peer/simulator_actions_iface_linux.go` (`ChaosResult`) and `internal/component/l2tp/ppp/ipv6_service_linux.go` (`IPv6ServiceConfig`, `NewIPv6Service`) each broke the linux view and were caught there, not by the rename |
| A-3 | No flagged symbol is reached by reflection or by name from a non-Go file | none of the findings is a struct field, which is what JSON tags reach | a runtime break no compiler sees | step 2's grep over non-Go files | broken: 653 of the 2015 worklist rows name a symbol that appears as a whole word in a tracked non-Go file, so step 2's pre-check skips them rather than renaming them |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A rename lands and the package no longer compiles for linux | `GOOS=linux go vet` fails | fix forward inside the same package, or re-export and skip the symbol |
| R-2 | 400 renames in one commit make review impossible | the diff spans every component | one package per commit, in worklist order |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A package fails to compile. No user-visible behavior changes, because no logic changes |
| How is it reverted? | Per-package commits, so one revert per package |
| Who else touches this path? | Any session working in the same package |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le repository check` | → | `validate.py` `check_cross_package_wiring()` | the finding for each renamed symbol is gone from its output |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A symbol on the worklist that `gopls rename` accepts | it is unexported, and its package compiles under every tag set for darwin and linux |
| AC-2 | A symbol `gopls rename` refuses | it is left exactly as it was, and the refusal is recorded with its reason |
| AC-3 | After every package is processed | `./le repository check` reports no `has no cross-package non-test caller` finding except those recorded under AC-2 |
| AC-4 | Any package touched | `go test -race <pkg>` passes |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| existing package tests | per package | AC-4 | |

### Functional Tests

A rename preserves behavior, so the coverage is the EXISTING functional suite
staying green rather than a new `.ci`. Run the suite that owns each package you
touch, and paste its result in the per-package commit.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `./le functional plugin` | `test/plugin/*.ci` | a plugin still loads, registers and answers its commands after its package is renamed | |
| `./le functional parse` | `test/parse/*.ci` | config still parses when `internal/component/config` symbols are unexported | |
| `./le functional encode` | `test/encode/*.ci` | wire encoding is byte-identical after a BGP package rename | |
| `./le functional ui` | `test/ui/*.ci` | CLI commands still dispatch after a `cmd/` or `internal/component/cli` rename | |
| `./le functional web` | `test/web/*.ci` | web routes still resolve after an `internal/component/web` rename | |

## Files to Modify
- roughly 300 Go files across `internal/`, `cmd/` and `pkg/`, one symbol per finding

## Files to Create
- none

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no config surface |
| YANG validation constraints | N-A | no config surface |
| YANG custom validators | N-A | no config surface |
| CLI commands/flags | N-A | no CLI surface |
| CLI grammar (keyword before value) | N-A | no CLI surface |
| Editor autocomplete | N-A | no config surface |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no route output |
| Env var registration | N-A | no env var |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Prometheus counters/metrics | N-A | no daemon state |
| BGP family surface | N-A | no protocol surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | internal API surface only |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | nothing under `pkg/` is renamed without a check that it is not SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | a `<!-- source: path -- Symbol -->` anchor naming a renamed symbol must follow it |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

**Work one package at a time, in worklist order. One package per commit.**

### Step 0: build the worklist, once

`check_cross_package_wiring()` in `internal/le/doc/wiring/checks.go` reads only the files
named by `--changed-file`, which defaults to `changed_files(root)` (the git
diff). A bare `./le repository check` on a clean tree therefore reports nothing, and
it is not the way to get the worklist. Name every non-test Go file under
`internal/` and `cmd/` instead. The check shells out one `grep -rlw` per exported
symbol, which costs about one second per file, so split the list and run the
parts at the same time.

```
mkdir -p tmp/unexport-chunks
find internal cmd -name '*.go' ! -name '*_test.go' | sort > tmp/unexport-chunks/gofiles.txt
split -n l/12 tmp/unexport-chunks/gofiles.txt tmp/unexport-chunks/c
for c in tmp/unexport-chunks/c*; do
  sed 's/^/--changed-file /' "$c" > "$c.args"
  ( xargs ./le verify lint rundocwiring/checks.go --root . < "$c.args" > "$c.log" 2>&1 ) &
done
wait

python3 - <<'PY' > tmp/unexport-chunks/worklist.tsv
import re, pathlib
for log in sorted(pathlib.Path('tmp/unexport-chunks').glob('*.log')):
    for l in log.read_text(errors='replace').splitlines():
        m = re.search(r'([^ :]+):(\d+): exported symbol (\w+) has no cross-package', l)
        if m:
            f, ln, s = m.group(1), m.group(2), m.group(3)
            print(f"{f}\t{ln}\t{s}\t{pathlib.Path(f).parent}")
PY
wc -l tmp/unexport-chunks/worklist.tsv
```

### Step 1: pick one package

Take every row whose fourth column is the same package. Process that package to
completion before starting another.

### Step 2: pre-checks that HALT (per symbol)

Skip the symbol, record it, and move on when ANY of these is true. Do not try to
make the rename possible.

| Check | Command | Skip when |
|-------|---------|-----------|
| Generated file | `head -1 <file>` | the first line contains `Code generated` |
| Named outside Go | `git grep -w <Symbol> -- ':!*.go' ':!vendor'` | any hit in `.md`, `.yang`, `.json`, or a script |
| SDK surface | `echo <file>` | the path starts with `pkg/` |

### Step 3: rename with gopls, which is the safety mechanism

```
FILE=<file>; LINE=<line>; SYM=<Symbol>
NEW=$(python3 -c "import sys;s=sys.argv[1];print(s[0].lower()+s[1:])" "$SYM")
COL=$(awk -v n="$LINE" 'NR==n{print index($0,"'"$SYM"'")}' "$FILE")
gopls rename -w "$FILE:$LINE:$COL" "$NEW"
```

`gopls` exits non-zero and prints the breaking reference when the rename is
unsafe. **That is a skip, not a failure to fix.** Record the symbol and its
reason, change nothing, and continue. It catches the cases a grep cannot: a
reference from another package's test file, a name that would collide with an
existing identifier, a method that satisfies an interface.

### Step 4: verify the package, all four build views

```
PKG=<package dir>
FEATURES=$(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')
for GOOS_V in darwin linux; do
  for TAGS in "ze_core ze_distro $FEATURES" "ze_test $FEATURES"; do
    GOOS=$GOOS_V go vet -tags "$TAGS" ./$PKG/... || echo "FAILED: $GOOS_V / $TAGS"
  done
done
go test -race ./$PKG
```

`go vet` compiles, so an undefined name fails here. **The `GOOS=linux` pass is
not optional**: `gopls` sees one build context, so a reference that exists only
in a `_linux.go` file is invisible to the rename and shows up only here. Eleven
symbols in the current worklist have their only reference in a platform-tagged
file.

Never run a bare `go build`: a hook refuses it unless it writes to `bin/`.

### Step 5: confirm the findings are gone

```
./le verify lint rundocwiring/checks.go --root . --changed-file <each file you renamed in>
```

### Step 6: commit that package

Use `internal/le/commit/prepare.go create`, one commit per package, listing every
file with `--file`. Read `ai/rules/git-safety.md` first. Never run `git add`,
`git commit`, or `git push` directly.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every worklist row is either renamed or recorded as skipped with its reason |
| Correctness | No signature, no behavior and no comment changed. Only identifiers |
| Data flow | The four build views all compiled for every touched package |
| Rule: `ai/rules/simplicity.md` | No symbol deleted, no API redesigned, no helper introduced |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The backlog is cleared | `./le repository check` shows no wiring finding outside the recorded skips |
| Nothing broke | `go test -race ./...` green for every touched package |
| The skips are recorded | one row per skipped symbol, with the gopls reason |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | None. No input path changes |

### Failure Routing

| Failure | Route To |
|---------|----------|
| `gopls rename` refuses | Skip the symbol and record it. This is expected, not a failure |
| `go vet` fails after a rename | Fix forward in that package, or re-export the symbol and record it as skipped |
| A test fails after a rename | Re-export the symbol and record it. Never edit the test to match |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The safety mechanism must be the tool, not the worklist. A grep classifier
  written for this spec agreed with `gopls` on the case it was checked against,
  but only `gopls` knows about interface satisfaction, identifier collision and
  external test packages at once.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `gopls rename` per symbol | `sed` over the package | `sed` cannot see an interface method, a collision, or another package's test. `gopls` refuses instead of breaking |
| Skip on refusal, never resolve | Rework the caller so the rename becomes possible | Reworking a caller is a design change, and this task must need no judgement |
| One package per commit | One commit for all 467 | A 300-file rename commit cannot be reviewed, and one bad rename cannot be reverted alone |
| Rename, never delete | Delete the unreachable symbol | Deleting needs a judgement about intent. Unexporting is reversible and behavior-preserving |

## Known Limitations
- Symbols that `gopls` refuses stay exported. That set is the honest remainder,
  and it is smaller than the 467 the gate reports.
- **`gopls` cannot refuse a rename it cannot see.** It runs with no build tags on
  the host GOOS, so a reference in a file carrying `//go:build ze_core` or
  `//go:build linux` is invisible to it, and the refusal that protects a
  cross-package test reference never fires. `internal/component/plugin`
  `AvailableInternalPlugins` was unexported this way and broke `cmd/ze`, whose
  only caller is `cmd/ze/main_test.go` under `//go:build ze_core`. A per-package
  `go vet` cannot catch it either, because it never compiles the other package.
  The catch is the whole-tree `go vet` over all four build views, run once after
  every package is processed.
- **Unexporting reveals dead code.** `golangci-lint`'s `unused` and `unparam`
  linters skip an exported declaration, so a symbol nothing calls reads as clean
  while it is exported and reports the moment it is not. 60-odd symbols were
  renamed, reported by the linter, and renamed back for this reason. They are
  recorded in `plan/spec-unused-code-hidden-behind-exported-symbols.md`, because
  deleting them is a judgement this rename-only task must not take.

## Remaining Work

Buckets 1 to 6 and 8 are processed. **Bucket 7 is untouched**: 170 symbols over
23 packages, listed in `tmp/unexport-bucket-7-remaining.tsv` (the driver is
`tmp/unexport-rename-driver.py`, the pre-check skip list is
`tmp/unexport-skipped-named-outside-go.tsv`, and the seven handoffs are in
`tmp/unexport-handoffs-buckets-1-6-8.md`). It was held back because
its packages share a `./PKG/...` vet scope with almost every other bucket, so it
had to run alone, and the session's budget ended first. The spec stays open for
it.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify current mode full` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written: the existing package tests and functional suites are the coverage. A rename adds no behavior, so it needs no new test
- [ ] Tests FAIL: not applicable in the usual direction. The failing state this task guards against is a rename that breaks the build, and step 4 produces it deliberately by compiling all four build views. Paste any failure and the symbol re-exported in response
- [ ] Tests PASS (paste output per package)
- [ ] Functional suite for each touched area green (paste output)
- [ ] Interop tests N-A: no protocol surface

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Journal row for anything this teaches
- [ ] **Commit A:** code + spec + journal row
- [ ] **Commit B:** `git rm plan/spec-fixit-unexport-package-private-symbols.md` only

## Pre-Commit Verification

This section covers buckets 1 to 6 and 8. Bucket 7 is open (Remaining Work).

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| none: the spec creates no file | N-A | `Files to Create` says none. The worklist artifacts are `tmp/unexport-bucket-7-remaining.tsv` and `tmp/unexport-rename-driver.py`, both present |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | An accepted symbol is unexported and its package compiles in every build view | `go vet ./internal/... ./cmd/...` under GOOS darwin and linux, with `ze_core ze_distro $(ZE_FEATURES)` and `ze_test $(ZE_FEATURES)`: clean apart from the pre-existing `noescape` finding in `internal/core/textbuf/textbuf.go`, which the pre-rename baseline reports identically |
| AC-2 | A refused symbol is untouched and its reason recorded | 139 refusals recorded in the per-bucket handoffs (`tmp/unexport-handoffs-buckets-1-6-8.md`), each carrying the `gopls` text |
| AC-3 | No wiring finding remains outside the recorded skips | Per-package `validate.py --changed-file` re-run in every phase. NOT yet true tree-wide: bucket 7's 170 rows stay open |
| AC-4 | Every touched package passes its tests | `go test -race ./<pkg>` green for all 161 processed packages, per phase |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le repository check` -> `check_cross_package_wiring()` | none: the check IS the test | Yes. `validate.py --changed-file` re-run per package, and the finding for each renamed symbol is gone |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `internal/component/config/transaction/solver.go` `TopologicalSort` refused, naming the external test package |
| A-2 | confirmed | `internal/chaos/peer/simulator_actions_iface_linux.go` and `internal/component/l2tp/ppp/ipv6_service_linux.go` broke only the linux view, caught by `GOOS=linux go vet` |
| A-3 | broken | 653 of 2015 rows name a symbol that appears as a whole word in a tracked non-Go file. Step 2 skips them |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Item 16, source anchors naming a renamed symbol | `check_source_anchor_stale_paths` and `check_source_anchor_line_numbers` run inside every per-package `validate.py`, and reported nothing | Yes |
| Items 1 to 15 and 17 | No user-facing surface changes: the change renames identifiers only, with no file move and no signature change | Yes |

## Work Inherited From a Deferral Row

<!-- The deferral directory was deleted on 2026-09-05. A row that named this spec as
     its destination is reproduced here, so the item and the reasoning behind it
     survive the directory. Each row is outstanding work this spec owns. -->

### From `fixit-unexport-package-private-symbols.md`, 2026-08-09

Deferred by `plan/spec-problem-journal.md`.

467 `exported symbol X has no cross-package non-test caller` findings from `./le repository check`
