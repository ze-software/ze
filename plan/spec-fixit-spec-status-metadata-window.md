# Spec: fixit-spec-status-metadata-window

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 1/2 |
| Deferral shard | - (no deferred item) |
| Updated | 2026-08-22 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`make ze-spec-status` reports `status: unknown` and `updated: unknown` for any
spec written the way `ai/rules/planning.md` prescribes, because the canonical
template and the inventory parser disagree about where the metadata table sits.

`extractField` (`scripts/status/spec_status.go`) scans only the first 10 lines of
a spec, by its own comment "to avoid matching unrelated tables further down".
`plan/TEMPLATE.md` opens with a six-line HTML authoring comment between the title
and the metadata table, which puts the `| Status |` row on line 12. The field is
never found, and `loadSpec` substitutes `unknown`.

Measured 2026-08-01: 16 of 211 specs carry the template comment, and those same
16 are exactly the 16 the tool reports as `unknown`. The correspondence is one to
one. Seven are pre-existing and were not created by the session that found this:

- `spec-fixit-dynamic-group-peer-config` (closed 2026-08-14; written without its `plan/` path for the same reason as the entry below)
- `spec-fixit-peers-from-tree-stale-shape` (closed 2026-08-14; written without its `plan/` path for the reason the last entry states)
- `spec-fixit-positional-arg-matching` (closed 2026-08-18; written without its `plan/` path for the same reason as the entries above)
- `plan/spec-fixit-zefs-diff-structural-ops.md`
- `plan/spec-rfcgate-2-deferred-nonunit-evidence-backfill.md`
- `spec-rfcgate-2-deferred-rs-replay-evidence` (closed 2026-08-03 in `15dac5bc4`; written without its `plan/` path because `spec-citation-check.py` reads any such path as a LIVE citation and the file is gone. Its record was retired with the learned corpus)
- `plan/spec-rfcgate-2-deferred-unrun-interop-trees.md`

Consequence: a spec reported `unknown` lands in the `other` bucket
(`specbucket.Category`), so it is counted in no in-progress, ready, or skeleton
total. Four open `fixit-` specs are therefore invisible in the inventory a
session reads to choose its next piece of work. The failure is silent, because
`unknown` reads as a data quirk rather than as a parse failure, and the tool
never says that it could not find the field.

The instruction and the tool contradict each other, which is why this keeps
happening. `ai/rules/planning.md` says "Always start from `plan/TEMPLATE.md`",
and doing exactly that produces a spec the inventory cannot read.

Goal: anchor the metadata parse on the table instead of on a line count. Find the
`| Field | Value |` header row and read the rows that follow it, or scan until
the first `## ` heading. Keep the original intent, which is to avoid matching the
trailing `| ... | Status |` header of the Assumptions, TDD and Interop tables
further down the file.

Fail-closed follow-up, in the same work (`ai/rules/evidence.md`): a
spec whose metadata table cannot be parsed at all must SAY so rather than report
`unknown`. Today an absent field and an unreadable file are indistinguishable,
and a zero-information answer is dressed as data. Decide whether that is a
warning on stderr or a non-zero exit, and pin it with a fixture.

Do NOT fix this by editing the 16 affected specs. Deleting the comment from each
one leaves the template still producing broken specs, and the next author hits it
again. The session that found this did strip the comment from the nine specs it
was creating at that moment, because the comment is template scaffolding that the
other 195 specs correctly drop, but that is authoring hygiene and it is not this
fix.

Provenance: found on 2026-08-01 while creating specs from the VyOS July 2026
comparison. Nine newly written specs were silently invisible to
`make ze-spec-status`, which is the mechanism that was supposed to make sure they
get implemented later.

### What already landed, and what this phase owes

The parse half shipped on 2026-08-18 in `58dc7f63e`, "fix(status): anchor the
spec metadata parse on its table". Nobody moved this spec off `skeleton`, so the
file still reads as unstarted work. That commit deleted `extractField`, added the
`specmeta` package, anchored the scan on the `| Field | Value |` header row, and
made an unreadable spec report `unparsed` on stdout and name itself on stderr.

Three holes are left, and this phase closes all three.

The first is the fixture the Task asks for. The fail-closed branch is pinned at
the helper (`specmeta.Rows`) and nowhere at the entry point. `ai/rules/evidence.md`
is explicit under "Test corollary": a unit test on the guard helper proves the
helper is correct and proves nothing about whether the caller reaches it. Nothing
drove `loadSpec` over a spec carrying no metadata table.

The second is one level out from the parse, and it is the same silent
under-report in the line the reader trusts most. `printTable` prints its
per-status counts from a hardcoded status list. A spec carrying a status absent
from that list is counted into the total and dropped from the breakdown, so the
summary reads as complete and is not. Measured 2026-08-22, before this phase:

```
Specs: 242 total (48 in-progress, 41 ready, 46 design, 93 skeleton, 11 blocked, 1 deferred)
```

Those six numbers sum to 240. Two specs carry `Status | done`, which
`.claude/hooks/validate-spec.sh` accepts and the reporting list does not name.
`verification` is worse and not yet visible: `ai/rules/planning.md` requires an
implementing session to set it before it commits, and the tool would drop it from
the summary and bucket it beside `blocked` and `deferred` as `other`, which is
where a reader looks for work nobody is carrying.

The third is the SAME defect in a sibling reader, which `58dc7f63e` never
touched. `make ze-spec-status` runs `scripts/dev/spec-closure-check.py --list`
straight after the Go tool, and `_status` there reads the first 12 lines of a
spec and takes the first hit. Every branch in that detector is driven by the
status it returns, so a spec whose Status row falls outside the window is
reported `unknown`, is neither in-progress nor closed, and is skipped in silence.
Measured 2026-08-22 over all 284 spec files in `plan/` and its subdirectories:
one spec, `plan/spec-support-export.md`, has its Status row on line 14 behind a
warning line and the template comment, and the detector could not see it. Fixing
the Go half and leaving this one reads as fixed and is half fixed.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/evidence.md` - the fail-closed guard rule this spec's second half implements
  → Decision: the guard's test drives the ENTRY POINT, so the fixture compiles the tool and runs it over a `plan/` tree of its own, rather than calling the parse helper.
  → Constraint: a guard must fail closed or say something, and a zero value must never read as a legitimate answer. `unknown` for an unreadable file was such a zero.
- [ ] `docs/contributing/ze-style.md` - Go written for this repository
  → Constraint: state an invariant positively and say why, not what. The status list keeps a comment saying what a name in it buys, because the derived tail makes membership optional.

### RFC Summaries (Scope: protocol)
Not applicable: this spec changes a repository inventory tool. No wire format,
no protocol, no RFC obligation is reachable from it.

**Key insights:** (minimal context to resume after compaction)
- The parse half landed in `58dc7f63e`; only the entry-point fixture and the summary completeness remain.
- `spec_status.go` carries `//go:build ignore`, so no in-process test can call `loadSpec`. The entry-point test compiles the file and runs the binary.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `scripts/status/spec_status.go` - reads every `plan/spec-*.md`, resolves Status through `specmeta`, sorts, and prints the table or JSON. `loadSpec` sets `unparsed` and writes the stderr line when no table is found. `printTable` prints the per-status summary from a fixed list.
- [ ] `scripts/status/specmeta/specmeta.go` - `Rows` anchors on the `| Field | Value |` header and stops at the first `## ` heading. `Field` reads one row.
- [ ] `scripts/status/specbucket/specbucket.go` - `Category` maps a status to backlog, idea, or other. `SkeletonStale` applies the six-week TTL.
- [ ] `scripts/status/spec_status_test.go` - covers the bucket split and the TTL boundary. It calls nothing in `spec_status.go`.
- [ ] `.claude/hooks/validate-spec.sh` - the authoring gate. It accepts skeleton, design, ready, in-progress, verification, blocked, deferred, done.

**Behavior to preserve:** (unless the user explicitly said to change it)
- The stdout table layout and the JSON record shape, field for field. `scripts/dev/spec-closure-check.py` and the cadence target read this tool.
- `unparsed` sorts first, and an absent Status row inside a table that IS present still reports `unknown`. The two answers stay distinct.
- The exit code stays 0 for an unreadable spec. The tool reports an inventory; it is not a gate.

**Behavior to change:** (only what the user asked for)
- The printed summary names every status a spec carries, so its counts sum to the total.
- `verification` sorts with committed work and buckets as backlog, not as `other`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `make ze-spec-status` and `make ze-spec-status-json` run `go run scripts/status/spec_status.go`, with the repository root as the working directory.
- Input format: every file matching `plan/spec-*.md`, read as UTF-8 markdown.

### Transformation Path
1. `loadAllSpecs` globs `plan/spec-*.md` and skips `spec-template.md`.
2. `loadSpec` reads one file, calls `specmeta.Rows` for the metadata table, and `specmeta.Field` for Status, Depends, Phase and Updated.
3. `loadSpec` applies the fail-closed branch: no table gives `unparsed` plus a line on stderr, and an empty field inside a table that IS present gives `unknown`.
4. `specbucket.Category` assigns the bucket, `specbucket.SkeletonStale` applies the TTL, and `statusOrder` sorts.
5. `printTable` prints the summary line, the bucket counts, and three sections. `printJSON` prints one record per spec.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Spec author ↔ inventory | The `\| Field \| Value \|` metadata table at the top of a spec file | Yes: `TestRowsFindsTablePastAFixedWindow`, and the entry-point test drives a written fixture tree |
| Inventory ↔ reader | stdout summary and table, stderr warning, exit code 0 | Yes: `TestSpecStatusReportsAnUnreadableSpec` reads all three |
| Inventory ↔ closure advisory | `scripts/dev/spec-closure-check.py --list` runs after the tool in the same target. It does not read the tool's output; it re-reads every spec through its own `_status` | Yes: `_status` carried the same fixed-window defect and is fixed here, proven by `TestSpecClosureReadsStatusPastAFixedWindow` and by an old-versus-new comparison over all 284 spec files |

### Integration Points
- `specmeta.Rows` and `specmeta.Field` - the parse, already integrated by `58dc7f63e`.
- `specbucket.Category` - gains the `verification` status, and keeps its existing three bucket names.
- `mk/inventory.mk` - the two `go run` targets, unchanged.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The test drives the compiled tool, so it reaches `loadSpec` through `main`, `run` and `loadAllSpecs` rather than around them |
| No unintended coupling (components stay isolated) | Yes | The change stays inside `scripts/status`. Nothing under `internal/` or `cmd/` is touched |
| No duplicated functionality (extends existing, does not recreate) | Yes | The status vocabulary is not copied into a fourth place. The summary derives its tail from the statuses it counted |
| Zero-copy preserved where applicable (refs, not copies) | N-A | A build tool that reads 242 small files once per invocation is not on a wire path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No command, view, family, or handler is added. The one list that remains hardcoded is a REPORTING ORDER, and a status missing from it is now printed from the counted set rather than dropped |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `go build` accepts a file carrying `//go:build ignore` when that file is named on the command line | `mk/inventory.mk` runs `go run scripts/status/spec_status.go` on the same file | The entry-point test cannot compile the tool, and the fixture must instead drive the make target | The test compiles it and fails loudly if it cannot | confirmed |
| A-2 | The tool run outside a git repository reports `git-modified: unknown` rather than failing | `gitDate` returns "unknown" when `git log` errors | The fixture's Updated fallback is non-deterministic across machines | Fixture specs carry their own Updated row, and the test asserts on Status rather than on dates | confirmed |
| A-3 | Two specs on disk carry a status the reporting list does not name | `make ze-spec-status` summary counts sum to 240 against a 242 total | The summary defect would be latent rather than live, and the fixture alone would prove it | Re-measured 2026-08-22 from the tool's own output | confirmed |
| A-4 | `verification` is a status the workflow requires, not a legacy spelling | `ai/rules/planning.md` tells the implementing session to set it before committing, and `.claude/hooks/validate-spec.sh` accepts it | Bucketing it as backlog would invent a state | Both files read 2026-08-22 | confirmed |
| A-5 | `specmeta.Rows` and `_status` are the only two readers of a spec's metadata table that use a window | `grep -rn "splitlines()\[:` over `scripts/` names three sites: `rule_coverage.py` (rule files, a different population), `ste_check.py` (an 8-line prose head), and `spec-closure-check.py`. `validate.py` searches the whole document | A third reader would keep the defect alive after this phase closes | The grep, re-run 2026-08-22 | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Compiling the tool inside a test makes the package test slow | The `scripts/status` package test time rises | `specStatusBinary` runs once per `runSpecStatus` call, so the two entry-point tests compile three times between them, each into its own temp directory. Go's build cache makes every compile after the first cheap, and the package test measured 9.5s on 2026-08-22 |
| R-2 | The fixture tree drifts from what a real spec looks like, so the test passes over a shape no author writes | A real spec regresses while the test stays green | The fixture reproduces the template shape the defect came from, six-line authoring comment included |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing user-visible. `ze` does not read this tool. A wrong answer misleads a session choosing its next piece of work, which is the failure already in progress |
| How is it reverted? | A single commit revert. The tool has no state and no persisted artifact |
| Who else touches this path? | `scripts/dev/spec-closure-check.py` runs beside it in the same make target and reads specs itself. `mk/cadence.mk` runs the target as an advisory note |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-spec-status` over a spec with no metadata table | → | `loadSpec` fail-closed branch, `statusUnparsed` | `TestSpecStatusReportsAnUnreadableSpec` |
| `make ze-spec-status` over a spec whose status the reporting list does not name | → | `printTable` summary | `TestSpecStatusSummaryCountsEverySpec` |
| A spec at `Status: verification` | → | `specbucket.Category`, `statusOrder` | `TestSpecStatusBacklogSplit` |
| A spec written from `plan/TEMPLATE.md`, metadata table past line 10 | → | `specmeta.Rows`, `specmeta.Field` | `TestRowsFindsTablePastAFixedWindow` (existing test, landed `58dc7f63e`) |
| `make ze-spec-status` closure advisory over a spec whose Status row sits on line 14 | → | `_status` (`scripts/dev/spec-closure-check.py`) | `TestSpecClosureReadsStatusPastAFixedWindow` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A spec whose metadata table sits below a six-line authoring comment | Every metadata field is read, and the Status reported is the one in the table |
| AC-2 | The same spec carrying an Assumptions table whose header ends `\| Status \|` | The scan stops at the first `## ` heading, so no value comes from below it |
| AC-3 | A spec carrying no `\| Field \| Value \|` table at all, read through the tool's own entry point | Status reports `unparsed`, stderr names the file and what is missing, the row sorts first, and the exit code stays 0 |
| AC-4 | A spec carrying a status the reporting list does not name | The summary line names that status with its count, so the printed counts sum to the total |
| AC-5 | A spec at `Status: verification` | It counts as committed backlog and sorts with in-progress work, not with blocked and deferred |
| AC-6 | A spec whose Status row sits past any fixed line window, read by the closure advisory that runs in the same make target | The advisory reads the real status and judges the spec on it, rather than reporting `unknown` and skipping it |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRowsFindsTablePastAFixedWindow` | `scripts/status/specmeta/specmeta_test.go` | AC-1: the parse is anchored on the table, not on a line count | PASS (landed `58dc7f63e`, re-run 2026-08-22) |
| `TestRowsStopsAtTheFirstHeading` | `scripts/status/specmeta/specmeta_test.go` | AC-2: no field is read from a table below the first heading | PASS (landed `58dc7f63e`, re-run 2026-08-22) |
| `TestRowsReportsAMissingTable` | `scripts/status/specmeta/specmeta_test.go` | The helper half of AC-3: a missing table is distinguishable from a missing row | PASS (landed `58dc7f63e`, re-run 2026-08-22) |
| `TestSpecStatusReportsAnUnreadableSpec` | `scripts/status/spec_status_test.go` | AC-3 at the entry point: the compiled tool's stdout, stderr, and exit code | PASS, red against the removed guard |
| `TestSpecStatusSummaryCountsEverySpec` | `scripts/status/spec_status_test.go` | AC-4 and AC-5: the printed counts sum to the printed total, and a verification spec is filed under committed backlog | PASS, red before the fix |
| `TestSpecStatusBacklogSplit` | `scripts/status/spec_status_test.go` | AC-5: `verification` buckets as committed backlog | PASS, red before the fix |
| `TestCategory` | `scripts/status/specbucket/specbucket_test.go` | AC-5 at the sibling test that enumerates the same statuses. `verification` and `done` were both absent from it | PASS |
| `TestSpecClosureReadsStatusPastAFixedWindow` | `scripts/dev/spec_closure_check_test.go` | AC-6: the closure advisory reads a Status row on line 14 | PASS, red against the fixed window |

### Test Evidence

Red, before the fix, `go test -count=1 -run TestSpecStatus ./scripts/status/`:

```
--- FAIL: TestSpecStatusBacklogSplit (0.00s)
    spec_status_test.go:46: Category("verification") = "other", want "backlog"
--- FAIL: TestSpecStatusSummaryCountsEverySpec (0.35s)
    spec_status_test.go:294: the summary counts sum to 1 and claim a total of 3; 2 specs are missing from the breakdown:
        Specs: 3 total (1 in-progress)
    spec_status_test.go:299: the summary line never names "verification", so those specs are invisible in it:
    spec_status_test.go:299: the summary line never names "done", so those specs are invisible in it:
    spec_status_test.go:304: a spec at verification is filed under "── Other: blocked / deferred / unknown (2) ──"; it is committed work waiting on a reviewer
FAIL
```

`TestSpecStatusReportsAnUnreadableSpec` passed on that run, because the branch
it drives had already landed. Its discrimination was measured by deleting that
branch from `loadSpec` and running it again:

```
--- FAIL: TestSpecStatusReportsAnUnreadableSpec (0.84s)
    spec_status_test.go:213: stderr does not name the unreadable spec.
        want a line containing: spec-status: <fixture-tree>/spec-fixture-no-table.md has no '| Field | Value |' metadata table
        got:
    spec_status_test.go:226: status of fixture-no-table = "unknown", want "unparsed"
FAIL
```

The fixture's own path is written `<fixture-tree>/` above rather than verbatim.
The test builds it under a temporary `plan/` of its own, and a literal
`plan/spec-*.md` inside this file is a citation to `make ze-spec-citation-check`,
which then reports a spec that exists only for the length of one test run.

The branch was restored and the whole tree is green, `go test -count=1 -v
./scripts/status/...`, 68 tests:

```
--- PASS: TestSpecStatusBacklogSplit (0.00s)
--- PASS: TestSkeletonTTLBoundary (0.00s)
--- PASS: TestSpecStatusReportsAnUnreadableSpec (0.90s)
--- PASS: TestSpecStatusSummaryCountsEverySpec (0.36s)
ok  	github.com/ze-software/ze/scripts/status	8.108s
ok  	github.com/ze-software/ze/scripts/status/specbucket	0.343s
ok  	github.com/ze-software/ze/scripts/status/specmeta	1.566s
```

`make ze-spec-status` over the real tree, after the fix. The seven counts sum to
242, which is the total it states. Four other sessions are working this checkout,
so the totals move between runs; what does not move is that the breakdown and the
total agree:

```
Specs: 242 total (48 in-progress, 42 ready, 46 design, 92 skeleton, 11 blocked, 1 deferred, 2 done)
Buckets: committed backlog 136 (design/ready/in-progress/verification) | idea capture 92 skeletons (32 past the 6-week TTL) | other 14
```

Nothing was written to stderr, so every spec on disk parses. The assertion the
test makes is the arithmetic rather than these numbers, so a status added later
is covered without editing it.

The closure advisory's own test, red against the 12-line window it used to read:

```
--- FAIL: TestSpecClosureReadsStatusPastAFixedWindow (0.13s)
    spec_closure_check_test.go:132: expected exit 3, got 0: the Status row sits on line 14 and the detector must still read it
        stderr:
FAIL
```

Green with the anchored parse, over all 13 tests that drive the detector:

```
--- PASS: TestSpecClosureFlagsCommittedButOpenSpec (0.20s)
--- PASS: TestSpecClosureReadsStatusPastAFixedWindow (0.19s)
--- PASS: TestSpecClosureIgnoresUnfinishedReviewGate (0.13s)
ok  	github.com/ze-software/ze/scripts/dev	2.245s
```

The change was measured against every spec file before it landed. Reading the
old parse and the new one over all 284 files under `plan/` and its
subdirectories, one answer differs and it differs in the right direction:

```
plan/spec-support-export.md: old='unknown' new='ready'
specs compared: 284 differing: 1
```

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Skeleton TTL age in days | 0 to unbounded | 42 (not yet stale) | N/A (an age is never negative) | 43 (stale), pinned by `TestSkeletonTTLBoundary` |

The parse itself takes no numeric input. The line count that caused this defect
is the number this spec removes.

### Functional Tests
Not applicable in the `.ci` sense: this spec changes no daemon code, so no
functional suite can reach it. The end-user surface is the `make ze-spec-status`
command, and `TestSpecStatusReportsAnUnreadableSpec` drives exactly that surface
by compiling the tool and running it over a written spec tree, reading stdout,
stderr and the exit code the operator sees.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestSpecStatusReportsAnUnreadableSpec` | `scripts/status/spec_status_test.go` | An author runs `make ze-spec-status` with a malformed spec on disk and is told which file the tool could not read | PASS |

### Interop Tests (Scope: protocol)
Not applicable: no protocol, no peer daemon, no wire-visible behavior.

## Files to Modify
- `scripts/status/spec_status.go` - `printTable` names every counted status; `statusOrder` places `verification`; the bucket headings say which statuses each holds
- `scripts/status/specbucket/specbucket.go` - `Category` buckets `verification` as committed backlog
- `scripts/status/spec_status_test.go` - the two entry-point tests, and the `verification` row in the bucket table
- `scripts/status/specbucket/specbucket_test.go` - the sibling test enumerating the same statuses, which omitted `verification` and `done`
- `scripts/dev/spec-closure-check.py` - `_status` anchors on the metadata table instead of reading the first 12 lines
- `scripts/dev/spec_closure_check_test.go` - the fixture whose Status row sits on line 14
- `plan/journal/gate-excludes-part-of-its-population.md` - one row for the summary defect found here

## Files to Create
None. Every file this phase needs already exists.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | A repository inventory tool holds no operator configuration |
| YANG validation constraints | N-A | No YANG leaf is added |
| YANG custom validators | N-A | No YANG leaf is added |
| CLI commands/flags | N-A | The tool's only flag is the existing `--json`, unchanged |
| CLI grammar (keyword before value) | N-A | No `ze` command is added |
| Editor autocomplete | N-A | No config leaf is added |
| Functional test for new RPC/API | N-A | No RPC and no API. The entry-point test drives the make surface instead |
| Pipe completeness | N-A | The tool prints its own table and does not route through `ApplyPipes` |
| Env var registration | N-A | No environment variable is read or added |
| Doctor check for runtime dependencies | N-A | No new file path, socket, service, port, or binary. The tool reads `plan/` and the `git` binary it already used |
| Prometheus counters/metrics | N-A | A build tool exports no metric |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The tool's output gains no feature. A count that was already claimed becomes true |
| 2 | Config syntax changed? | No | No config is read by this tool |
| 3 | CLI command added/changed? | No | `ze` gains no command. `make ze-spec-status` keeps its name and its flags |
| 4 | API/RPC added/changed? | No | No RPC exists here |
| 5 | Plugin added/changed? | No | No plugin is involved |
| 6 | Has a user guide page? | No | The make target is documented in `mk/inventory.mk`'s own quick reference, which still describes it correctly |
| 7 | Wire format changed? | No | No wire format is reachable |
| 8 | Plugin SDK/protocol changed? | No | `pkg/plugin` is untouched |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC obligation is reachable from a spec inventory |
| 10 | Test infrastructure changed? | No | Two tests are added to an existing package test. No runner, tag, or target changes |
| 11 | Affects daemon comparison? | No | `ze` behavior is unchanged |
| 12 | Internal architecture changed? | No | `scripts/status` files declare `// Design: (none -- build tool)`, so no architecture document owns them. Confirmed with `scripts/dev/spec_doc_anchors.py` |
| 13 | Route metadata keys added/changed? | No | No route metadata exists here |
| 14 | Prometheus counters added/changed? | No | No metric is defined |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | The SPEC inventory changes what it prints, and no registered inventory (`docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md`) describes this tool's summary line |
| 16 | Any changed source file referenced by existing doc source anchors? | No | `grep -rn "source: scripts/status\|source: scripts/dev/spec-closure" docs/ ai/` returns 10 anchors and every one names `scripts/status/verify_run.go`, which this phase does not touch. No anchor names `spec_status.go`, `specbucket`, `specmeta`, or `spec-closure-check.py` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | `grep -rn "ze-spec-status" docs/` returns nothing |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- drive the fail-closed branch from the entry point
   - Tests: `TestSpecStatusReportsAnUnreadableSpec`, `TestSpecStatusSummaryCountsEverySpec`
   - Files: `scripts/status/spec_status_test.go`
   - Verify: the fail-closed assertion passes against the landed code, and goes red when the `statusUnparsed` branch is removed. The summary assertion fails first, because the defect is live
2. **Phase: Summary completeness** -- the printed counts sum to the total
   - Tests: `TestSpecStatusSummaryCountsEverySpec`
   - Files: `scripts/status/spec_status.go`
   - Verify: red before, green after, and the tool's own output over the real `plan/` tree sums correctly
3. **Phase: The verification status** -- committed work is counted as committed work
   - Tests: `TestSpecStatusBacklogSplit`, `TestCategory`
   - Files: `scripts/status/specbucket/specbucket.go`, `scripts/status/spec_status.go`
   - Verify: red before, green after
4. **Phase: The sibling reader** -- the closure advisory in the same make target
   - Tests: `TestSpecClosureReadsStatusPastAFixedWindow`
   - Files: `scripts/dev/spec-closure-check.py`
   - Verify: red against the fixed window, green with the anchored parse, and the old and new parse compared over every spec file so the change moves exactly the answers it should

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1 and AC-2 are met by `58dc7f63e`; AC-3 to AC-6 each have an implementation and a test named in the TDD plan |
| Every reader of the metadata table | Two tools read it and both are fixed: `specmeta.Rows` in Go and `_status` in the closure advisory. `SPEC_STATUS_RE` (`scripts/dev/validate.py`) searches the whole document rather than a window, and the row it matches must START with `\| Status \|`, which no table below the metadata one does, so it never had this defect |
| Feature completeness | The tool's three surfaces all carry the answer: the summary line, the bucket sections, and the JSON record |
| Correctness | An absent table and an absent Status row give different answers. The summary counts sum to the total for every status a spec can carry, named or not |
| Naming | `unparsed` says the tool could not read the file. `unknown` says the table was read and the row was absent. Neither word is used for the other case |
| Data flow | The parse lives in `specmeta` alone. `spec_status.go` decides policy (fail closed, order, buckets) and `specmeta` decides nothing |
| Rule: `ai/rules/evidence.md` | The guard's test drives the entry point, and its discrimination is proven by removing the branch and recording the red |
| Rule: registration over hardcoding | The status reporting list is an order, not a registry. A status absent from it is printed, not dropped |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| No fixed-window scan is left in either reader of the metadata table | `grep -rn "splitlines()\[:12\]\|extractField\|first 10 lines" scripts/status/ scripts/dev/spec-closure-check.py` returns nothing |
| The entry-point fixture exists and drives the compiled tool | `grep -n "TestSpecStatusReportsAnUnreadableSpec" scripts/status/spec_status_test.go` |
| The whole spec tree parses | `make ze-spec-status` prints no `spec-status:` line on stderr and no `unparsed` row |
| The summary is complete | `make ze-spec-status` summary counts sum to the printed total |
| Tests pass | `GOTOOLCHAIN=go1.26.6 go test ./scripts/status/...` and `GOTOOLCHAIN=go1.26.6 go test -run TestSpecClosure ./scripts/dev/` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | The tool reads repository files an author wrote, not untrusted input. A malformed file must produce a named answer rather than a crash, which is the fail-closed branch itself |
| Resource exhaustion | The glob is bounded by the file count of `plan/`, and each file is read once into memory. No recursion and no unbounded loop is added |
| Error leakage | The stderr line names a repository path already visible in the working tree. No credential, token, or environment value reaches the output |
| Command execution | `gitDate` runs `git log` with the path as an argument rather than through a shell, so a filename cannot inject a command. Unchanged by this phase |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- A guard proven only at its helper is the shape this repository keeps meeting. The helper test here was correct and complete, and the entry point had never been driven at all. The cost of the missing test is not a wrong helper: it is that nobody could say whether `loadSpec` still called it.
- `//go:build ignore` on a single-file `go run` tool buys the tool a home beside its siblings and costs it every in-process test. The way back in is to compile the file and run the binary, which is what `scripts/checks/tracked_build_test.go` already does for the same reason.
- A hardcoded reporting order and a hardcoded vocabulary look identical in a diff. The first is a preference and the second is a duplicate. Deriving the tail keeps the order without keeping the duplicate.
- One make target ran two readers of the same table, in two languages, and fixing the Go one read as fixing the defect. The second reader was found by asking who else answers this question, not by reading the diff. A file format with more than one parser has as many copies of a parsing defect as it has parsers.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Compile the tool in the test and run it over a temporary `plan/` tree | Move `spec_status.go` into its own package so a test can call `loadSpec` in process | The move is a restructure of three files and two make targets to buy an in-process call. Compiling proves more: it drives `main`, the glob, and the exit code, which an in-process call never reaches |
| Print any status the reporting list does not name, sorted, after the named ones | Add every accepted status to the list | A fourth copy of the vocabulary drifts from the other three the moment one changes. Deriving the tail cannot drift |
| Name `verification` in the order and the bucket, but not `done` | Name both | A name in the list buys a POSITION. `verification` is committed work in flight and must sort with it. `done` is terminal, and the derived tail already prints it in the right place |

## Known Limitations
- The status vocabulary still lives in three places: `ai/rules/planning.md` prose, the `case` in `.claude/hooks/validate-spec.sh`, and this tool. The two rule files disagree today, because the hook accepts `done` and the rule does not list it. This phase makes the tool correct whatever the vocabulary is, and it does not unify the three. Unifying them is a rules-and-hooks change with no home in a spec about the inventory parse.
- The tool still exits 0 for an unreadable spec. That is deliberate: it is an inventory, not a gate, and the authoring gate that refuses a malformed spec is `.claude/hooks/validate-spec.sh`.
- The two readers of the metadata table now apply the same rule and share no code, because one is Go and one is Python. The rule is stated in a comment in each, each naming the other. Nothing stops them drifting apart again except that both are now tested against a spec whose Status row sits past a fixed window.
- The inventory reads `plan/spec-*.md` and does not recurse. 41 specs live in `plan/future/` and `plan/to-review/` and are counted nowhere. For `plan/future/` that is correct and documented: its README says a spec there "moves back into `plan/` when the owner schedules it", so it is not scheduled work. `plan/to-review/` holds three specs and carries no README, so what the inventory owes them is an open question for the owner rather than a defect in the parse. This phase changes neither.

## RFC Documentation (Scope: protocol)

Not applicable. This spec implements no protocol behavior.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`scripts/status/*.go` reached from `mk/inventory.mk`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Lesson written as a row in `plan/journal/<class>.md`
- [ ] **Commit A:** code + tests + docs + spec + journal row
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented

- `TestSpecStatusReportsAnUnreadableSpec` (`scripts/status/spec_status_test.go`)
  drives the tool's own entry point over a written `plan/` tree and reads
  stdout, stderr and the exit code. It is the fixture the Task asked for: the
  fail-closed branch was pinned at `specmeta.Rows` and nowhere at `loadSpec`.
- `printTable` (`scripts/status/spec_status.go`) derives the tail of its
  reporting order from the statuses it counted, so the summary's counts sum to
  the total it prints.
- `verification` joins `statusOrder` (`spec_status.go`) at position 2 and
  `specbucket.Category` (`scripts/status/specbucket/specbucket.go`) as committed
  backlog.
- `_status` (`scripts/dev/spec-closure-check.py`) anchors on the
  `| Field | Value |` header row instead of reading the first 12 lines. It is
  the sibling reader `make ze-spec-status` runs straight after the Go tool.

### Bugs Found/Fixed

- The summary line under-reported. Six counts summed to 240 against a printed
  total of 242, because two specs carry `done` and the hardcoded order had never
  heard of it. Covered by `TestSpecStatusSummaryCountsEverySpec`.
- `verification` bucketed as `other`, so the status `ai/rules/planning.md`
  requires an implementing session to set sat beside blocked and deferred.
  Covered by `TestSpecStatusBacklogSplit` and `TestCategory`.
- The closure advisory read the first 12 lines and reported `unknown` for
  `plan/spec-support-export.md`, whose Status row sits on line 14. Covered by
  `TestSpecClosureReadsStatusPastAFixedWindow`.
- Found at closure: neither entry-point test could see its own producer change.
  `go test` keys a cached result on the files the TEST BINARY opened, and
  neither an exec'd compiler nor an exec'd interpreter is the test binary.
  `spec_status.go` carries `//go:build ignore` so it is outside the package's
  source hash, and `spec-closure-check.py` is not Go at all. Measured: with the
  cache warm, deleting the `statusUnparsed` branch from `loadSpec` left
  `./scripts/status/` reporting `ok (cached)`, and reverting `_status` to its
  12-line window left `./scripts/dev/` reporting the same. `specStatusBinary`
  and `runSpecClosure` now read their producer before running it, which is the
  pattern `scripts/status/verify_run_test.go` already applies to its own gate.

### Documentation Updates

None. The Documentation Update Checklist answers all 17 rows No, and row 16 is
re-verified at closure: `grep -rn "source: scripts/status\|source: scripts/dev/spec-closure"`
over `docs/` and `ai/` names no anchor on any file this spec touches.
`grep -rn "Committed backlog"` over the tree finds the string only in
`spec_status.go` and its own test, so no reader parses the headings that
changed. `make ze-doc-verify` was not needed: no doc was edited.

### Deviations from Plan

- The spec's Files to Modify did not name the test-cache defect, because nobody
  had looked for it. Both entry-point helpers changed at closure; no production
  file gained a line for it.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The implementation phase recorded each test's red-and-green pair from runs that had passed a distinct `-run` filter, which is a cache miss for a reason unrelated to the edit. That made the discrimination look proven when the cache would have hidden the same mutation on a repeated command | A test whose producer is exec'd is cached against a change to that producer, so a repeated command reports the previous verdict | The closure re-ran each mutation on the SAME command line the previous run used, and the second one answered `ok (cached)` | Both helpers read their producer, and every red-and-green pair below was re-measured with the cache warm |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Anchor the metadata parse on the table instead of on a line count | Done | `specmeta.Rows`, `scripts/status/specmeta/specmeta.go` | Landed `58dc7f63e`; re-verified at closure |
| Keep the original intent: no value read from the trailing `\| ... \| Status \|` header of a table further down | Done | `specmeta.Rows` stops at the first `## ` heading | `TestRowsStopsAtTheFirstHeading` |
| Fail closed: an unparseable spec must SAY so | Done | `loadSpec`, `scripts/status/spec_status.go` | stderr line plus `unparsed`, exit code stays 0 |
| Pin the fail-closed answer with a fixture | Done | `TestSpecStatusReportsAnUnreadableSpec` | Drives the compiled tool, not the helper |
| Do NOT fix this by editing the affected specs | Done | No spec file was edited for its metadata | `make ze-spec-status` prints no stderr over 242 specs |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRowsFindsTablePastAFixedWindow` | `headerRE` anchors the scan; no line count remains |
| AC-2 | Done | `TestRowsStopsAtTheFirstHeading` | `Rows` breaks on the first `## ` |
| AC-3 | Done | `TestSpecStatusReportsAnUnreadableSpec` | stdout, stderr, sort position and exit code all asserted |
| AC-4 | Done | `TestSpecStatusSummaryCountsEverySpec` | The assertion is the arithmetic, not the words |
| AC-5 | Done | `TestSpecStatusBacklogSplit`, `TestCategory` | `statusOrder` returns 2, `Category` returns Backlog |
| AC-6 | Done | `TestSpecClosureReadsStatusPastAFixedWindow` | `_status` anchors on the header row |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestRowsFindsTablePastAFixedWindow` | Done | `scripts/status/specmeta/specmeta_test.go` | Landed `58dc7f63e` |
| `TestRowsStopsAtTheFirstHeading` | Done | `scripts/status/specmeta/specmeta_test.go` | Landed `58dc7f63e` |
| `TestRowsReportsAMissingTable` | Done | `scripts/status/specmeta/specmeta_test.go` | Landed `58dc7f63e` |
| `TestSpecStatusReportsAnUnreadableSpec` | Done | `scripts/status/spec_status_test.go` | Red with the `statusUnparsed` branch deleted |
| `TestSpecStatusSummaryCountsEverySpec` | Done | `scripts/status/spec_status_test.go` | Red with the derived tail removed |
| `TestSpecStatusBacklogSplit` | Done | `scripts/status/spec_status_test.go` | Red with `verification` out of `Category` |
| `TestCategory` | Done | `scripts/status/specbucket/specbucket_test.go` | Same mutation, same red |
| `TestSpecClosureReadsStatusPastAFixedWindow` | Done | `scripts/dev/spec_closure_check_test.go` | Red with `_status` reverted to its 12-line window |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `scripts/status/spec_status.go` | Done | `printTable` derived tail, `statusOrder`, bucket headings |
| `scripts/status/specbucket/specbucket.go` | Done | `Category` buckets `verification` |
| `scripts/status/spec_status_test.go` | Done | Two entry-point tests, plus the closure's cache fix |
| `scripts/status/specbucket/specbucket_test.go` | Done | `verification` and `done` rows added |
| `scripts/dev/spec-closure-check.py` | Done | `_status` anchored on the table |
| `scripts/dev/spec_closure_check_test.go` | Done | Line-14 fixture, plus the closure's cache fix |
| `plan/journal/gate-excludes-part-of-its-population.md` | Done | Two rows dated 2026-08-22: the summary defect, and the `plan/to-review/` population question |
| `plan/journal/test-cache-stale.md` | Added at closure | One row dated 2026-08-22 for the exec'd-producer cache defect the review found |

### Audit Summary
- **Total items:** 27
- **Done:** 27
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (both test helpers gained a producer read; recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A spec written from `plan/TEMPLATE.md` is read, not reported `unknown` | Data correctness | `make ze-spec-status` over 242 real specs writes nothing to stderr and prints no `unparsed` row. Before the parse fix, 16 specs read `unknown` |
| A spec the tool cannot read SAYS so rather than reporting a zero as data | Functional, at the entry point | `TestSpecStatusReportsAnUnreadableSpec` compiles the tool, runs it over a written tree, and asserts the stderr line, the `unparsed` status, the sort position and exit 0. Deleting the `statusUnparsed` branch reddens it with the cache warm |
| The one line a reader trusts is complete | Data correctness | `Specs: 242 total (48 in-progress, 42 ready, 46 design, 92 skeleton, 11 blocked, 1 deferred, 2 done)`. The seven counts sum to 242. `TestSpecStatusSummaryCountsEverySpec` asserts the arithmetic, so a status added later is covered without editing it |
| Committed work waiting on a reviewer is counted as committed work | Data correctness | `statusOrder("verification")` returns 2 and `specbucket.Category("verification")` returns Backlog. `TestSpecStatusBacklogSplit` and `TestCategory` red when `Category` loses the case |
| Both readers of the metadata table apply the same rule | Data correctness | `_status` and `specmeta.Rows` anchor on the same header row. Old versus new `_status` over all 283 spec files under `plan/`: one answer changes, `plan/spec-support-export.md` from `unknown` to `ready` |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| None | done | The spec metadata reads `Deferral shard \| - (no deferred item)`, and no file exists at `plan/deferrals/spec-fixit-spec-status-metadata-window.md`. Nothing to remove |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-spec-status-metadata-window-ebf5d9b3-b158-40df-bba0-32a51591883e.md` |
| `review_gate.py check` | clean |
| Rounds | 2 |
| Reviewer lenses used | automated pre-checks (`make ze-repository-check`, `audit-test-relaxation.py`), size, wiring, functional-test coverage, documentation drift, removed-behavior audit, comment invariants, data flow, logic correctness and guard audit, security, simplicity and altitude, project-rule cross-check, test discrimination by mutation |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The test cache does not observe `spec_status.go`. It carries `//go:build ignore`, so it is outside the test binary's source hash, and the compiler that reads it is an exec'd process. With the cache warm, deleting the `statusUnparsed` branch from `loadSpec` left `./scripts/status/` reporting `ok (cached)`: AC-3's and AC-4's evidence survived removing the code they test | `specStatusBinary`, `scripts/status/spec_status_test.go` | The helper reads the source before compiling it. The same mutation now reddens `TestSpecStatusReportsAnUnreadableSpec` on a repeated command line |
| 2 | BLOCKER | The same defect at the sibling reader's test: `spec-closure-check.py` is read by an exec'd interpreter, so reverting `_status` to its 12-line window left `./scripts/dev/` reporting `ok (cached)`. AC-6 rested on it | `runSpecClosure`, `scripts/dev/spec_closure_check_test.go` | The helper reads the detector before running it, which is what `scripts/status/verify_run_test.go` already does for its own gate. The same mutation now reddens the test with the cache warm |
| 3 | ISSUE | The file header still described the committed-backlog bucket as design/ready/in-progress after `verification` joined it (`ai/rules/stale-comments.md`) | `scripts/status/spec_status_test.go`, package comment | Rewritten to name the four statuses the bucket now holds |
| 4 | ISSUE | An unsourced count: the fixture comment said the template comment "made 12 specs invisible". The only measurement on record is 16 of 211 specs on 2026-08-01, in this spec's own Task | `templateShapedSpec`, `scripts/status/spec_status_test.go` | The number is gone. The comment states the mechanism, which the fixture demonstrates |

NOTEs, recorded and not blocking:

- The bucket-section heading and `specbucket.Other`'s doc comment enumerate the
  statuses `Category` derives by subtraction, so a status added later leaves
  both wrong while every row still lands in the right section. Deriving the
  heading would make it vary between runs, which costs a reader more than the
  drift does.
- R-1's mitigation claimed one compile shared by both entry-point tests.
  `specStatusBinary` runs per `runSpecStatus` call, so it compiles three times.
  The risk row is corrected above; the code is unchanged.
- `plan/to-review/` holds three tracked specs that neither reader counts. See
  Known Limitations: this is a question about the tool's POPULATION, and every
  AC here is about its PARSE. Recorded as a journal row and raised with the
  owner rather than decided here.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `scripts/status/spec_status.go` | Yes | `ls scripts/status/` lists it at 9.5K |
| `scripts/status/spec_status_test.go` | Yes | `ls scripts/status/` lists it at 13K |
| `scripts/status/specbucket/specbucket.go` | Yes | `git status --porcelain` reports it modified |
| `scripts/dev/spec-closure-check.py` | Yes | `git status --porcelain` reports it modified |
| `scripts/dev/spec_closure_check_test.go` | Yes | `git status --porcelain` reports it modified |
| `plan/journal/gate-excludes-part-of-its-population.md` | Yes | `git diff` shows two added rows dated 2026-08-22 |

The spec's Files to Create section reads None, and no `.ci` is named: this spec
changes a build tool, so no functional suite can reach it.

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | The parse is anchored on the table | `headerRE` in `specmeta.Rows` is `^\|\s*Field\s*\|\s*Value\s*\|`, and `grep -rn "splitlines()\[:12\]\|extractField\|first 10 lines" scripts/status/ scripts/dev/spec-closure-check.py` returns nothing |
| AC-2 | No value comes from below the first heading | `Rows` opens its loop with `if strings.HasPrefix(line, "## ") { break }` |
| AC-3 | An unreadable spec reports `unparsed`, names itself, sorts first, exit 0 | `TestSpecStatusReportsAnUnreadableSpec` PASS; red with the `loadSpec` branch deleted, cache warm |
| AC-4 | The summary counts sum to the total | `make ze-spec-status` line 1: 48+42+46+92+11+1+2 = 242 = the printed total. `TestSpecStatusSummaryCountsEverySpec` PASS; red with the derived tail removed |
| AC-5 | `verification` is committed backlog and sorts with in-progress | `statusOrder` returns 2; `Category` returns `Backlog`. `TestSpecStatusBacklogSplit` and `TestCategory` PASS; both red with the case removed |
| AC-6 | The closure advisory reads a Status row past a fixed window | `TestSpecClosureReadsStatusPastAFixedWindow` PASS; red with `_status` reverted, cache warm |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `make ze-spec-status` over a spec with no metadata table | N-A (Go entry-point test) | Yes: `TestSpecStatusReportsAnUnreadableSpec` compiles `spec_status.go` and runs the binary, so it reaches `loadSpec` through `main`, `run` and `loadAllSpecs` |
| `make ze-spec-status` over a spec whose status the reporting list does not name | N-A (Go entry-point test) | Yes: `TestSpecStatusSummaryCountsEverySpec` parses the tool's own summary line |
| A spec at `Status: verification` | N-A (unit) | Yes: `TestSpecStatusBacklogSplit` and the bucket assertion inside the entry-point test |
| A spec written from `plan/TEMPLATE.md` | N-A (unit) | Yes: `TestRowsFindsTablePastAFixedWindow` |
| `make ze-spec-status` closure advisory over a Status row on line 14 | N-A (Go test driving the script) | Yes: `TestSpecClosureReadsStatusPastAFixedWindow` runs `spec-closure-check.py` over a committed fixture repo and asserts exit 3 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `specStatusBinary` compiles `spec_status.go` by name and the tests pass, so `go build` accepts a `//go:build ignore` file named on the command line |
| A-2 | confirmed | The fixture tree is a temp directory outside any repository and each fixture spec carries its own Updated row; the tests assert Status, never a date |
| A-3 | confirmed | `make ze-spec-status` reports 2 specs at `done`, which the pre-fix reporting order did not name |
| A-4 | confirmed | The `case` in `.claude/hooks/validate-spec.sh` accepts `verification`, and `ai/rules/planning.md` tells the implementing session to set it before it commits |
| A-5 | confirmed | `grep -rn "splitlines()\[:" scripts/` now names two sites, `rule_coverage.py` (rule files, `key: value` meta lines, a different population and a different format) and `ste_check.py` (an 8-line prose head). `spec-closure-check.py` is off the list because it is fixed |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Row 16: no doc source anchor names a changed file | `grep -rn "source: scripts/status\|source: scripts/dev/spec-closure" docs/ ai/` names only `scripts/status/verify_run.go`, untouched here | Yes |
| Row 17: no doc shows examples for this area | `grep -rn "ze-spec-status" docs/` returns nothing | Yes |
| No doc enumerates the bucket membership that changed | `grep -rn "design/ready/in-progress" ai/ docs/ mk/` returns nothing, and `grep -rn "Committed backlog"` over the tree finds only `spec_status.go` and its test | Yes |
| `ai/INDEX.md` describes the target | Its row says "committed backlog vs skeleton idea capture, stale-skeleton flags", which names no status list and stays true | Yes |
| Rows 1-15: no user-facing, config, CLI, API, plugin, wire, RFC, metric or inventory surface changed | `ze` gains no command and reads none of this code; the only surface is `make ze-spec-status`, whose name and flags are unchanged | Yes |

## Core Insight

A test that drives its producer through `exec` is invisible to the one mechanism
everybody trusts to tell them the test still runs. `go test` keys a cached
verdict on the files the TEST BINARY opened, so a Python detector, a shell gate,
and a Go file behind `//go:build ignore` are all outside it. The failure is
worse than a missing test, because the bar is green and it names the test that
is supposed to be watching. The repair is one line and it is the same line every
time: read the producer before you run it.
