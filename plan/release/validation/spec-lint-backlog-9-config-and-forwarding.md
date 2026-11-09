# Spec: lint-backlog-9-config-and-forwarding

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | 5 findings of the full lint in config storage, the YANG CLI and the netlink interface backend, all in tests; the full lint holds the release (`plan/release/validation/spec-lint-backlog-0-umbrella.md`) |
| RFC gaps | - |
| Depends | - |
| Phase | 3/3 |
| Handoff | - |
| Updated | 2026-11-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Admission uses the one-off WIP exception in `plan/release/validation/spec-lint-backlog-0-umbrella.md`, "Owner's one-off WIP exception".

Child of `plan/release/validation/spec-lint-backlog-0-umbrella.md`. Clear the
findings CI's full `./le go lint run` reported on 2026-11-02 in
`internal/component/config/storage` and `config/yang/cli` (door
`ai/doors/config-yang.md`) and in `internal/plugins/iface/netlink` (door
`ai/doors/forwarding.md`). All five are in test files. By linter: modernize 3,
errcheck 1, misspell 1. Line numbers are the CI run's.

| File | Line | Linter |
|------|------|--------|
| `internal/component/config/storage/store_reader_test.go` | 255 | errcheck |
| `internal/component/config/yang/cli/yang_refusal_test.go` | 16 | misspell |
| `internal/plugins/iface/netlink/address_watch_linux_test.go` | 31 | modernize |
| `internal/plugins/iface/netlink/neighbor_watch_integration_linux_test.go` | 37 | modernize |
| `internal/plugins/iface/netlink/neighbor_watch_linux_test.go` | 17 | modernize |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/config-yang.md` - config storage and the YANG CLI
  → Constraint: a test that writes config state uses its own store, never the developer's (`suite-shares-one-persistent-store`)
- [ ] `ai/doors/forwarding.md` - the netlink backend
  → Constraint: select `internal/plugins/iface/netlink` explicitly for kernel proof; `integration iface` selects only the component.
- [ ] `docs/contributing/ze-go-style.md` - "The rules a tool enforces"

**Key insights:**
- The misspell in `yang_refusal_test.go` may be a quoted refusal message: if the producer's message is misspelled, fix the producer and the test together.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/yang/cli/yang_refusal_test.go` - the misspell site
- [ ] `internal/component/config/storage/store_reader_test.go` - the ignored error
- [ ] `.golangci.yml` - the linter settings

**Behavior to preserve:**
- Every test's assertions.

**Behavior to change:**
- Each finding above, fixed at its source.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Lint runner | `./le go lint run scope <packages>` | Lints a package scope | Applies: the evidence for this child |
| Scoped kernel tests | `internal/test/userns/userns_linux.go` `Enter`; native `job run` over `internal/plugins/iface/netlink` | Re-executes a test in private user, network and mount namespaces | Applies: select the plugin package explicitly; `internal/le/test/integration/gates.go` selects only `internal/component/iface/...` for `iface` |
| Style rule table | `docs/contributing/ze-go-style.md` | Names the fix each linter expects | Applies |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le go lint run scope "./internal/component/config/storage/... ./internal/component/config/yang/cli/... ./internal/plugins/iface/netlink/..."`

### Transformation Path
1. Read each finding; fix it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None | Test edits | No |

### Integration Points
- None beyond the packages listed.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The CI list is current for these packages | CI run of 2026-11-02 | Lines drifted | Source audit against the saved CI findings: four sites already corrected in the working tree; only the storage error receive remains. No fresh lint run, per parent serialization. | broken |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The netlink tests cannot run on the host | The run skips for lack of capability | Run them in the QEMU guest |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le go lint run scope` over the packages | → | golangci-lint | zero findings in the scoped run |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The scoped lint over the three packages | Zero findings |
| AC-2 | The config tests and the netlink tests (in the QEMU guest when the host lacks the capability) | Pass |
| AC-3 | Any `//nolint` added | Names its linter and its reason |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| The existing package tests | the changed packages | AC-2 | |

## Files to Modify
- The files in the Task table.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG, CLI, env, doctor, metrics, BGP family surface | N-A | Lint fixes; no surface changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1-15, 17 | Feature, config, CLI, API, plugin, wire, SDK, RFC, test infrastructure, comparison, architecture, metadata, metrics, inventory, examples | No | Lint fixes change no documented behavior |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/release/validation/spec-lint-backlog-9-config-and-forwarding.md` |

## Implementation Steps

1. Scoped lint; record the current list.
2. Fix the five findings.
3. Scoped lint zero; tests pass.

### Step 2 diagnosis

The saved CI errcheck finding is still present in `store_reader_test.go`,
`TestRuntimeWriteDoesNotWaitForADirectorySync`: its timeout branch joins the
runtime writer but discards the error it sends. The owning layer is the test.
The [workaround] is a suppression; the [source] fix checks the received error
with a nonfatal assertion, so the configuration writer is still joined even
when the runtime write failed. Suppression would hide useful failure evidence.
The existing timeout failure and success-path counter assertion stay intact.

The misspell is a comment, not refusal output, and is already corrected.
The three modernize sites already use flattened embedded fields. These existing
working-tree edits are preserved. A-1 was wrong about the current list; the
parent owns fresh lint and test evidence after the edits.

### Step 2 handoff

Patched `TestRuntimeWriteDoesNotWaitForADirectorySync` with
`assert.NoError(t, <-counted)`. The assertion is nonfatal so both workers are
joined on the failure path. No suppression or expectation change was added.
Scoped `git diff --check` passed; no test, lint, build or compile ran in this
agent. At this handoff AC-1 and AC-2 were unverified; the closure sections
below record the evidence that discharged them. AC-3 needs no new annotation.

The kernel cases are `TestIntegrationAddressWatchSignalsAddAndDelete` and
`TestIntegrationNeighbourWatchSeesPreexistingEntry`. Both call `userns.Enter`;
`childCommand` creates private user, network and mount namespaces and
`childSetup` prepares the mounts and loopback. A namespace startup failure
fails rather than skips. The observed host has `CapEff=0`, UID 1000 and no
seccomp filter; this does not establish whether private namespaces work.
The staged `tmp/kernel/build/vmlinuz` has runtime/amd64 metadata for version
7.3, matching `internal/appliance/kernel.version`; guest execution must still
verify the running kernel. The exact scoped QEMU command and remaining checks
are in this spec's per-session state handoff. No QEMU run occurred here.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Test contracts | Storage tests use their own store, error checks retain assertions, and any refusal-text correction matches its producer. |
| Kernel proof | Netlink tests execute with the required capability, in the QEMU guest when needed. A skip is not AC-2 evidence (R-1). |
| Config evidence | Three scopes are lint-clean, config and netlink tests pass, and annotations state reasons (AC-1–3). |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Three test scopes cleared | Record zero scoped lint findings and passing config tests, with reasoned annotations (AC-1–3). |
| Executed netlink proof | Cite interface integration results, using the QEMU guest if host capabilities are absent. Record skips separately (AC-2, R-1). |

### Security Review Checklist

| Concern | What to verify |
|---------|----------------|
| Persistent state | Storage tests use their own store and retain error assertions. |
| Kernel ownership | Netlink test modernization preserves assertions and cleanup ownership. Refusal-message fixes match the producer's behavior. |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Scoped lint zero; config package tests pass; explicit netlink kernel tests pass. Full-run evidence belongs to the umbrella's CI run.
- [ ] Targeted tests: the scoped package runs of `./le test unit plugins` and the config group, as `./le job run ... go test -race -tags ...` over the three scopes (with `integration` for the netlink integration files).

## lint9 implementation evidence

This phase corrected `color` in the method comment of
`TestYANGDataRefusalsAreReturned`, and flattened the embedded literals in
`TestIntegrationAddressWatchSignalsAddAndDelete`,
`TestIntegrationNeighbourWatchSeesPreexistingEntry`, and `neighUpdate`.
The source fix preserves every fixture value and assertion; a suppression
would only hide the finding. No suppression or product behavior changed.

The parent reported a pre-edit scoped lint with four findings in Linux and
integration and zero in Darwin. No storage finding was reported in that run.
The historical storage site read as a bare channel receive when inspected by
lint9. Another writer subsequently added the storage assertion and the Step 2
diagnosis above. That assertion is separate from lint9's four-file patch; the
parent must reconcile the historical finding with the current lint result.

With `tags` derived from `feature-gates.txt` plus `ze_core` and `CGO_ENABLED=1`,
this phase ran `./le --name session job run label lint9-unit quiet command go test -race -v -timeout 240s -tags "$tags" -skip '^TestIntegration' ./internal/component/config/storage/... ./internal/component/config/yang/cli/... ./internal/plugins/iface/netlink/...`.

| Scope | Observed result |
|-------|-----------------|
| YANG CLI | PASS, including `TestYANGDataRefusalsAreReturned` |
| Netlink non-integration selection | PASS, including address-watch lifecycle tests and all seven `TestNeighbourWatch` tests; `TestXFRMInterfaceProbeOnThisHost` skipped because its namespace probe received operation not permitted |
| Storage CLI | PASS |
| Storage | FAIL: package deadline after 240 seconds during `TestRemoveVersionPreservesEveryReference/tree/active`. Stack: `treeEncoding.barrier` through `installBytes`, waiting in `File.Sync`. The active test had just started, so this is not evidence of a four-minute wait in that test. `TestOpenRejectsForeignOwnerEvenForRoot` skipped because changing ownership requires root |

AC-1 remains owed: the parent runs post-edit scoped lint. AC-2 remains open:
storage did not finish and the two kernel watcher tests were excluded by the
explicit selector, not passed or skipped by the test runner. AC-3 adds no
suppression. No documentation contract changed.

Required kernel selection: `go test -v -tags "$tags" -run '^(TestIntegrationAddressWatchSignalsAddAndDelete|TestIntegrationNeighbourWatchSeesPreexistingEntry)$' ./internal/plugins/iface/netlink`, through native `job run`, on Ze's kernel. Both files carry only `linux`; no `integration` tag is needed for these two. Both call `userns.Enter` first, which fails on namespace startup denial rather than silently skipping. The parent owns this proof, with a pinned-kernel QEMU guest where needed, and the remaining storage proof.

The post-commit `repo compiles check` requirement conflicts with the whole-tree
ban: `docs/architecture/testing/tracked-build-gate.md`, "Flavors and why each
names a file", says it builds `./...` across flavors from an archive. It is
left unrun; the scoped tests above compile the edited test files.

## Implementation Summary

### What Was Implemented
- `da9a5bface` ("config: clear storage and netlink test lint"), five one-line test edits:
  - `store_reader_test.go` `TestRuntimeWriteDoesNotWaitForADirectorySync`: the timeout branch asserts `assert.NoError(t, <-counted)` (errcheck). Nonfatal, so `written` is still joined.
  - `yang_refusal_test.go` `TestYANGDataRefusalsAreReturned`: method comment `colour` to `color` (misspell). A comment; no producer text carries the word (`grep -rni colour internal/component/config/yang/cli/` is empty).
  - `address_watch_linux_test.go` `TestIntegrationAddressWatchSignalsAddAndDelete`, `neighbor_watch_integration_linux_test.go` `TestIntegrationNeighbourWatchSeesPreexistingEntry`, `neighbor_watch_linux_test.go` `neighUpdate`: embedded `LinkAttrs`/`Neigh` literals flattened to promoted fields (modernize; `go.mod` is go 1.27.3).
- Closure, two findings the 2026-11-09 scoped lint raised in netlink test files later commits added (`8d13a9dceb`, `a0b68101fc`), both inside this spec's scope:
  - `nduseropt_linux_test.go` `ndUserOptBody`: unparam, `ifindex` always 7. The parameter became the named constant `ndUserOptIfindex`, which the header write, `TestDecodeNDUserOpt`'s ifindex assertion and `TestHandleNDUserOptEmitsOption`'s `linkNames.Store` all read. Every assertion keeps its value.
  - `addr_p2p_absence_integration_linux_test.go`, the missing-device case: modernize embedlit, `&netlink.Dummy{Index: 2147483647}`.

### Bugs Found/Fixed
- None in product code. The storage timeout path discarded the runtime writer's error; it is now asserted.

### Documentation Updates
- None. No page anchors an edited test file: `grep -rlnE 'source: internal/(component/config/storage/store_reader_test|component/config/yang/cli/yang_refusal_test|plugins/iface/netlink/(address_watch_linux_test|neighbor_watch))' docs ai` hits only `docs/features/interfaces.md`, whose anchor names the unchanged product file `neighbor_watch_linux.go` (`WatchNeighbors`). `./le spec citation anchors spec plan/release/validation/spec-lint-backlog-9-config-and-forwarding.md` printed nothing, rc 0.

### Foreign reds
The 2026-11-09 `lint9-netlink` run was red in four tests in files this spec never touched. None holds this closure (`ai/rules/pre-release.md`):
- `TestVLANQoSEgressPCPOnWire`, `TestVLANQoSDSCPFullChain`: nft `table netdev` refused; already journaled in `plan/journal/unwired-feature.md` (no `CONFIG_NF_TABLES_NETDEV`).
- `TestBridgeSetSTPKernel`: `kernel reports IFLA_INFO_KIND "bridge\x00", want "bridge"`. The test helper `bridgeLinkInfo` keeps the kernel's trailing NUL. New row in `plan/journal/parser-refuses-a-form-its-producer-emits.md`, owned by `plan/release/validation/spec-doctor-coverage-2-home-cpe-batch.md`.
- `TestPPPoEClientAlreadyAbsentPair`: `initial address not installed`. New row in `plan/journal/package-red-not-reproduced.md`, owned by `plan/release/fixes/spec-ppp-l2tp-aaa-one-off-defects.md`.

### Deviations from Plan
- A-1 broken: four of the five CI sites were already corrected in the working tree; only the storage receive remained.
- Two findings outside the CI list of 2026-11-02 were fixed at closure (above): later commits added them to this spec's netlink scope, and AC-1 needs the scope at zero.
- The netlink kernel proof runs in private user namespaces (`userns.Enter`) on the host's Ze kernel, not in a QEMU guest; R-1 did not materialise.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1: the CI list of five was current | Four sites were already fixed in the tree | Source audit, Step 2 diagnosis | Recorded; no rule change warranted |
| assumption | The CI list was the whole scope | Later commits added two findings to the netlink scope | Scoped lint at closure, 2026-11-09 | Fixed at closure |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| errcheck `store_reader_test.go` | Done | `TestRuntimeWriteDoesNotWaitForADirectorySync` | `assert.NoError(t, <-counted)` |
| misspell `yang_refusal_test.go` | Done | `TestYANGDataRefusalsAreReturned` comment | comment only |
| modernize `address_watch_linux_test.go` | Done | `TestIntegrationAddressWatchSignalsAddAndDelete` | `&netlink.Dummy{Name: "aw0"}` |
| modernize `neighbor_watch_integration_linux_test.go` | Done | `TestIntegrationNeighbourWatchSeesPreexistingEntry` | `&netlink.Veth{Name: "w0", PeerName: "w1"}` |
| modernize `neighbor_watch_linux_test.go` | Done | `neighUpdate` | `NeighUpdate{Type, LinkIndex, IP, State}` |
| unparam `nduseropt_linux_test.go` (closure) | Done | `ndUserOptBody`, `ndUserOptIfindex` | `const ndUserOptIfindex = 7` |
| modernize `addr_p2p_absence_integration_linux_test.go` (closure) | Done | the missing-device `remover.link` | `&netlink.Dummy{Index: 2147483647}` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Scoped lint rerun, 2026-11-09: `./le --name session go lint run scope "./internal/component/config/storage/... ./internal/component/config/yang/cli/... ./internal/plugins/iface/netlink/..."` exit 0 after both closure fixes: host 0 issues, GOOS=linux with `integration` 0, darwin (2 packages) 0. It ran from a session build of le, because the committed le's scoped lint refused (`plan/journal/gate-fires-outside-its-population.md`, 2026-10-21 row); same golangci-lint flavor matrix | |
| AC-2 | Done | Scoped race runs, 2026-11-09: `lint9-config` (`go test -race -count=1`, all ze tags, storage and YANG CLI) exit 0; `lint9-netlink` (`-race -count=1 -v`, all ze tags plus `integration`) shows `TestIntegrationAddressWatchSignalsAddAndDelete`, `TestIntegrationNeighbourWatchSeesPreexistingEntry`, `TestDecodeNDUserOpt` and `TestHandleNDUserOptEmitsOption` PASS, not SKIP; its exit 1 comes from four foreign reds (see "Foreign reds") | Kernel watcher tests must PASS, not SKIP |
| AC-3 | Done | Neither `da9a5bface` nor the closure edits add a `//nolint` | The existing `//nolint:errcheck // Close always answers nil` in `TestIntegrationNeighbourWatchSeesPreexistingEntry` predates this spec and names its linter and reason |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Existing package tests | Done | storage, storage CLI, YANG CLI, netlink | AC-2 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| The five Task files | Done | `da9a5bface`; unchanged since (`git log da9a5bface..HEAD` over the five paths is empty) |
| `nduseropt_linux_test.go`, `addr_p2p_absence_integration_linux_test.go` | Done | Closure edits, commit A |

### Audit Summary
- **Total items:** 7 findings + 3 ACs
- **Done:** 10
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0 (A-1 and the two closure findings recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The three scopes carry no lint finding | scoped lint | `./le --name session go lint run scope "./internal/component/config/storage/... ./internal/component/config/yang/cli/... ./internal/plugins/iface/netlink/..."` exit 0 after both closure fixes: host 0 issues, GOOS=linux with `integration` 0, darwin (2 packages) 0. It ran from a session build of le, because the committed le's scoped lint refused (`plan/journal/gate-fires-outside-its-population.md`, 2026-10-21 row); same golangci-lint flavor matrix |
| The fixes keep every assertion and the tests green | scoped race tests, real-kernel tests | `lint9-config` (`go test -race -count=1`, all ze tags, storage and YANG CLI) exit 0; `lint9-netlink` (`-race -count=1 -v`, all ze tags plus `integration`) shows `TestIntegrationAddressWatchSignalsAddAndDelete`, `TestIntegrationNeighbourWatchSeesPreexistingEntry`, `TestDecodeNDUserOpt` and `TestHandleNDUserOptEmitsOption` PASS, not SKIP; its exit 1 comes from four foreign reds (see "Foreign reds"); `./le test weakened check base da9a5bface~1`: "clean (no unexplained test weakening)", 1250 test files |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None | Every finding and AC-1 to AC-3 is done | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/lint-backlog-9-config-and-forwarding-49da3d3d-44f5-443f-bc7a-73b9535a17f9.md` |
| `./le spec review check` | OK over the seven test files (round 2) |
| Rounds | 2: the `da9a5bface` diff, then the two closure edits |
| Reviewer lenses used | Size, logic (both workers joined on the timeout path; the constant feeds the header, the assertion and the link-name map alike), assertion preservation, security, style (no panic, no new non-test Go, no `//nolint`). Independent closure context, inline |
| Final result | 0 BLOCKER, 0 ISSUE |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | none | - | - |

NOTE: `./le repo check` reports one ISSUE outside this diff: the number-parse allowlist for `internal/test/dhcpprobe/step.go`, from `bd985af80f`.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| The seven edited test files | Yes | All modified, none created: `git show --stat da9a5bface` (5 files) and `git diff --stat` over the two closure files |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Zero scoped findings | `./le --name session go lint run scope "./internal/component/config/storage/... ./internal/component/config/yang/cli/... ./internal/plugins/iface/netlink/..."` exit 0 after both closure fixes: host 0 issues, GOOS=linux with `integration` 0, darwin (2 packages) 0. It ran from a session build of le, because the committed le's scoped lint refused (`plan/journal/gate-fires-outside-its-population.md`, 2026-10-21 row); same golangci-lint flavor matrix |
| AC-2 | Tests pass; the kernel tests executed | `lint9-config` (`go test -race -count=1`, all ze tags, storage and YANG CLI) exit 0; `lint9-netlink` (`-race -count=1 -v`, all ze tags plus `integration`) shows `TestIntegrationAddressWatchSignalsAddAndDelete`, `TestIntegrationNeighbourWatchSeesPreexistingEntry`, `TestDecodeNDUserOpt` and `TestHandleNDUserOptEmitsOption` PASS, not SKIP; its exit 1 comes from four foreign reds (see "Foreign reds") |
| AC-3 | No `//nolint` added | `git show da9a5bface` and the closure diff contain no `nolint` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le go lint run scope` over the three packages | none (lint) | `./le --name session go lint run scope "./internal/component/config/storage/... ./internal/component/config/yang/cli/... ./internal/plugins/iface/netlink/..."` exit 0 after both closure fixes: host 0 issues, GOOS=linux with `integration` 0, darwin (2 packages) 0. It ran from a session build of le, because the committed le's scoped lint refused (`plan/journal/gate-fires-outside-its-population.md`, 2026-10-21 row); same golangci-lint flavor matrix |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | Step 2 diagnosis: four sites were already fixed; Mistake Log row |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| 1-15, 17: No | A test-only diff changes no documented behaviour | Yes |
| 16: anchors | Only `docs/features/interfaces.md`, whose anchor names the unchanged `neighbor_watch_linux.go` | Yes |
