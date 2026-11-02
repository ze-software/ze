# Spec: Tooling, hook and test-harness one-off defects from the journal

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P3 |
| Release reason | Each row is a defect in `le`, the hooks, the commit and verify tooling, or the functional and editor test harness; none changes the shipped binary, so it does not block release correctness |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The 2026-10-23 journal sweep left about 265 open rows that fit none of the
thirteen root-cause specs. This spec owns the 70 in the development tooling:
`le` delegation, the commit and verification-debt tooling, the verify worktree
sweeper, the hooks, the RFC gates, the functional, editor and QEMU harnesses,
and a few process rows about agents. Each row is a claim until a red test
reproduces it at HEAD. Many rows cite retired `scripts/dev/*.py` or
`.claude/hooks/*` producers; those are re-read against the named Go successor
first, and a row the successor already fixes moves to "Rows found already fixed".

Left to root-cause specs: rows in classes T1 to T6 (machine-dependent verdicts,
prose claims, gate populations, shared-tree builds, vacuous greens, missing
causes) and the `tree-state-claim-published-unverified` rows (T2, T4).

A row that blocks release validation (the `.ci` runner answering 0 for an
unknown verb, `RunEngineSteps` skipping unknown kinds, the orphaned
`stdin=peer` block) is taken first, because a green run that ran nothing hides
shipped defects.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/committing.md`
- [ ] `docs/contributing/running-commands.md`
- [ ] `docs/functional-tests.md`
- [ ] `docs/architecture/testing/verify-freshness-scope.md`
- [ ] `plan/learned/HOOK-FRICTION.md`

## Current Behavior (MANDATORY)

Spot-checked on 2026-10-23: `zeTestParseRunCLI` still lives in
`internal/test/cli/cmd_bgp.go`; its unknown-verb exit is checked at AC-1. The
`.ci` parser now refuses an unknown directive (`internal/test/runner/parsing.go`,
"unknown directive"), so that row is fixed.

- [ ] `internal/test/cli/cmd_bgp.go` - `zeTestParseRunCLI`
- [ ] `internal/test/runner/engine_steps.go` - `RunEngineSteps`, `UnmarshalEngineSteps`
- [ ] `internal/test/runner/runner_exec.go` - `runTest`, `runOrchestrated`
- [ ] `internal/component/cli/testing/runner.go` - `.et` options
- [ ] `internal/le/commit/script.go` - `renderSharedIndexRepair`, `renderCommitBlock`
- [ ] `internal/le/commit/debt.go`, `internal/le/commit/actions.go` - `recordDebt`, `dischargeDebt`, `clearDebtWith`
- [ ] `internal/le/verify/` - `signalContext`, `sweepAbandoned`
- [ ] `internal/le/hookruntime/writeedit.go` - `writeGoPatterns`, `writeDesignEvidence`
- [ ] `internal/le/hookruntime/lifecycle.go` - `hookSessionStart`
- [ ] `internal/le/rfc/` - `checkGapCountAgreement`
- [ ] `internal/le/test/qemu/` - `Run.virtfsArgs`, `Run.acceleration`, `guestScript`
- [ ] `internal/le/interoplab/ipsec/ipsec.go` - `zeEnvironment`
- [ ] `internal/le/interoplab/check_special.go` - `specialCheckers`
- [ ] `internal/le/perf/bench.go` - `(*Bench).Run`
- [ ] `cmd/ze/le_build_name.go` - `refuseWrongBuildName`
- [ ] `internal/appliance/cmd_build.go` - `runGokInProcess`

## Existing Mechanisms (MANDATORY)

Each owned row is a defect in a tool that already exists, so every fix extends
that tool in place. The rows below name the mechanisms the commit and approval
rows touch; the rest are named by their journal row's Surface cell.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Owed-obligation answering | `internal/le/commit/owed_trailers.go` `(*answering).answered` | Clears an owed review, RFC approval or Linux limit on a commit: by attestation, or for an RFC approval by an in-range `RFC-approved:` unit or by the unit not being changed | extends: an approval recorded after the owing commit landed needs a route into it |
| Commit admission of RFC-tagged changes | `internal/le/commit/prepare.go`, the `rfc-change-ok` branch | Writes one `RFC-approval-owed:` trailer per changed unit with no recorded approval, and `RFC-approved:` for a unit the commit changes and that holds a recorded approval | extends |
| Trailer authorship guard | `internal/le/commit/input.go` `Message` | Refuses a subject or body line that starts with a declared trailer key, so only the tooling writes a trailer | does not apply: the guard stays, the fix is a tooling route |
| Owner attestation | `./le commit create attest <sha> "<words>"` (`docs/contributing/committing.md`) | The owner's words answer an owed obligation on one commit | does not apply: owner-only, never an agent route |
## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
`./le <verb>` from an agent or the owner; a Claude Code hook event; a `.ci`,
`.et` or interop scenario run.

### Transformation Path
1. Launcher resolves the build and session identity.
2. Native action runs (commit create, verify, test suite, hook).
3. Result reported, ledgers and shards written.

### Boundaries Crossed
`le` to git (private index); `le` to child processes (`ze`, helpers, QEMU, Docker).

### Integration Points
The verification-debt shards, the session directory, the hook registry.

### Architectural Verification
- [ ] A helper receives the argv and identity its caller ran with

## Risks & Assumptions

### Assumptions
- Sessions share one main checkout (`ai/META-DECISIONS.md`); fixes design inside that model.

### Risks
- Commit-tooling fixes run in every session; each needs a fixture proving the old failure.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `le test bgp <mistyped-verb>` | → | `zeTestParseRunCLI` | unit asserting a non-zero exit and usage |
| `./le commit create ... remove <path>` | → | generated script | fixture asserting the path is gone from disk after the commit |

## Acceptance Criteria

| AC | Rows | Observable |
|----|------|------------|
| AC-1 | runner unknown verb, unknown engine step kinds, orphan `stdin=peer` | Each refuses with a non-zero exit naming the input; none reports a pass |
| AC-2 | `.et` options and tick cost, `.et` wall clock | `option=width`, `height` and `wait=` are applied or refused; each `.et` test has a per-test timeout like `.ci` |
| AC-3 | runner reaping, interop image cleanup, `runOrchestrated` linger | Background helpers are waited for before ports are released; cleanup failure after a pass is reported as cleanup, not as a test failure; linger reads the routed input |
| AC-4 | `le` delegation x5 | A helper `le` gets the caller's build name and session identity; `stress-repro` names the right suite and never reuses a stale verdict |
| AC-5 | commit tooling x9 (remove leaves file, closed spec on disk, message path per tag, append duplicates, debt recorded before validation) | `remove` deletes the working-tree copy after the commit; the message path is unique per prepared commit; a debt row is written once, after validation |
| AC-6 | debt discharge and clear | A wrong discharge record can be corrected by a later session through a command; the push gate's clearance has a writer |
| AC-7 | verify sweeper x2, verify cancellation | An interrupted removal is reclaimed; an `.owner` marker without its directory is removed; TERM is observed within seconds |
| AC-8 | hook gates x2 lost in the Go port, Stop hook classifier, `writeGoPatterns` exemption, session-start cost x2 | Each hook category the fixture population describes is implemented; the panic exemption applies to the matching call only; session-start runs inside its 5 s budget |
| AC-9 | RFC gate level x2, rule metadata level, `./le rfc approve` attribution | Gap counts respect the row level; the rule metadata level agrees with its body; `rfc approve` refuses an owner attribution without the owner's authorisation |
| AC-10 | QEMU harness x4 (9p symlink xattr, arm64 KVM, bootstrap mount, relative appliance dir), Docker kernel probe | The guest build works with `cache` as a symlink; arm64 Linux uses KVM when present; a failed 9p mount stops the script; a relative `ZE_APPLIANCE_DIR` works; an `unknown` probe row is reported as a probe failure |
| AC-11 | fixtures x3 (hook probe tree, le-qemu-run-answers x2), wait-file driver, storage-bfd-restart store | Each fixture encodes a state its producer accepts; the BFD restart fixture runs its ze on a persistent store |
| AC-12 | interop checkers x15 lost bodies, IPsec interop log level, `Bench.Run` build-only | Each registered special checker asserts its feature; IPsec Ze runs at the level its checkers read; a build-only bench stages its inputs |
| AC-13 | `captureStdout` pipe, `TestUnionTimeout` race, chaos golden and demo poster stability | The helper drains the pipe concurrently; the test sets the interval before start; golden and poster outputs are byte-stable |
| AC-14 | lint `unused` on l2tp, `go mod vendor` reverting patches, YANG resolution in test binaries | Lint is green on `internal/component/l2tp/...`; vendoring re-applies owned patches or refuses; schema-dependent CLI tests link the modules they resolve |
| AC-15 | `ZE_VERIFY_MODE`, spec WIP cap, `./le test functional --pattern` | One variable carries one meaning; the WIP cap is enforced by the Go successor; the documented `--pattern` works or the doc is corrected |
| AC-16 | test-type backfill x3, process rows x3 (AC written as an absence, `git mv` by a subagent, mutation of a live guard x2) | Each is routed to the rule or gate that governs it (`ai/rules/testing.md`, `ai/rules/git-safety.md`), with a hook or fixture where a check is possible; a row with no checkable form is closed with the owning rule named |
| AC-17 | runner tmpfs on `Record`, activity widget dark theme, `checkSnapshot` quoted paths, GoBGP export reconcile, codeql `paths-ignore` | The `.ci` path carries `*tmpfs.Tmpfs` with its mode; the widget follows the page theme; a path git quotes commits (`ls-files -s -z`); the GoBGP lab peer re-advertises on export change or the scenario says why; the CodeQL Go job skips `gokrazy/modcache` |
| AC-18 | le stop x4 (`runDocker`, `installStopped`, `installCommand`, unrooted verbs) | A stop ends the container or child it started, reports truthfully what changed, and a second Ctrl-C reaches le |
| AC-19 | le test rows x5 (`demos/terminal/web-config` validator, `go run` of a non-main file, committed `zz` probes, `requireEveryJobAdvisory`, `le-discovery-answers` changed list) | Each test drives the path it claims, or is deleted with its probe; the workflow test and the workflow agree |
| AC-20 | zero answers x3 (feature tag set, perf comparison table, `listStdlib`) | An empty feature tag set, a duplicate `dut-name` and an empty `go list std` each refuse with the cause rather than read as data |
| AC-21 | stressrepro `runSlug` | A run of 91 selectors writes its capture under a bounded name and every invocation starts |
| AC-22 | session state `Uncommitted:` | The section lists only paths this session changed, or is renamed to say it is the whole tree |
| AC-23 | `rfc/not-enrolled.txt` kind | Every kind the ledger writes is one `./le rfc check` accepts; `iso-iec-10589` passes the check |
| AC-24 | spec speaker population | The spec template asks for every reader and writer of a wire found by search, not the files opened first |
| AC-25 | missing RFC summaries x8 | Each named document gets a summary and the retired rows return to it, or `rfc/corrections/` states why not |
| AC-26 | standing reds x6 (site goconst, cli-grammar unblocked, post-OSPF lint, repo check/audit/doc verify, le fixtures, `pluginOwnershipGate`) | Each gate is green at HEAD or runs in a blocking set with a baseline, and the le fixtures pass |
| AC-27 | executable bytes disagree with their Go content identity | Trace the corruption to its writer or storage boundary and demonstrate the repair; a successful rebuild alone does not close the incident |
| AC-28 | generated bulk UPDATE session attributes | Ordinary generated bulk announcements include LOCAL_PREF on iBGP and omit it on eBGP, proven through the real peer handshake for IPv4 and IPv6; intentional raw-hex injection remains unchanged |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| one red-first unit or fixture per AC | package of the producer | AC-1 to AC-16 |

### Functional Tests

| Test | AC |
|------|----|
| hook fixtures through `hookruntime.Probe` | AC-8 |

## Files to Modify
- The producer files under Current Behavior.

### Integration Checklist
- [ ] Each fix reached through the `./le` verb or hook an agent runs

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/contributing/committing.md` (AC-5, AC-6)
- [ ] `docs/functional-tests.md` (AC-15)

## Implementation Steps
1. Read each producer at HEAD (the Go successor for retired scripts); move a fixed row to "Rows found already fixed".
2. AC-1 first (release-validation blocker), then the rest.
3. Red test, fix, green, page edit, journal cell closed with the commit.

## Executable integrity incident

Two terminal-demo builds on 2026-10-26 exited successfully but produced damaged
executables. The first failed in `runtime.moduledataverify1` before initialization;
the second reported an unexpected EOF loading the embedded `ze-extensions.yang`.
The source module was nonempty.

| Executable | Zero-filled file interval | Stored Go content ID | Recomputed content ID |
|------------|---------------------------|----------------------|-----------------------|
| Startup failure | `0x2000000..0x2800000` | `YNJkZwklx-RBZlSDsYpq` | `WAQqHyi1zYO9fa0P_0X-` |
| Embedded-schema failure | `0x2400000..0x2540000` | `wk2MOJhdeFtdMaMaUTeB` | `KCklwPUbGXChvI1v3Iij` |

The read-only recomputation follows Go's `buildid.FindAndHash` and `HashToString`:
exclude the GNU note's descriptor, zero Go build-ID occurrences and encode the
first 120 SHA-256 bits. It agrees with the intact control. The mismatches place
the damage after Go computed the content hash, not necessarily after its later
build-ID rewrite. They do not distinguish source-inode damage, installation
copying or subsequent destination mutation.

A diagnostic retained the linker output and traced cross-filesystem installation:
rename and `copy_file_range` returned `EXDEV`, followed by buffered writes.
Its source and destination matched. A subsequent native build retaining its
work directory also matched source and staged SHA-256
`4ad69e96ff72cbcd205e4e86dd5f69d768b02a5004db01daf1eac4a61d338c24`;
the staged Ze, recorder and runtime helper passed their content-ID comparisons.
Neither successful run establishes a repair.

Session `644a4e78-0122-4eeb-8642-eb50638a1641` retains the damaged executables as
`scratch/ze-zeroed-segment.tar.gz` and `scratch/ze-zeroed-schema.tar.gz`, plus
diagnostic traces. Preserve those archives until the incident is diagnosed.
On recurrence, retain the actual native build's source and compare both files
before execution. The responsible component remains unestablished.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `concurrent-session-corruption` | 2026-10-30 | site build's linux le cross-compiles the live checkout |
| `review-pin-hashes-uncommitted-content` | 2026-10-30 | closure review pin hashes the working tree, not the landed blob |
| `gate-verdict-depends-on-the-machine` | 2026-10-29 | dnsresolver system name-server test needs the Linux resolv.conf writer |
| `gate-verdict-depends-on-the-machine` | 2026-10-29 | coupling check judges with the host plugin list |
| `gate-verdict-depends-on-the-machine` | 2026-10-27 | verify changed at 043c682d24 load reds |
| `lint-contract-not-applied` | 2026-10-27 | changed-scope lint at 043c682d24 |
| `command-name-breaks-the-cli-grammar` | 2026-10-27 | readme-carrier and vpn-kernel-test names |
| `delegation-hands-the-callee-a-different-argv` | 2026-10-27 | `le` rollback refusal echoes `$*` |
| `lint-contract-not-applied` | 2026-10-27 | radiussubscriber check_policer.go parse not allowlisted |
| `lint-contract-not-applied` | 2026-10-27 | changed-scope lint at 2f36373081 |
| `command-name-breaks-the-cli-grammar` | 2026-10-27 | `ze appliance kernel --jobs` undeclared |
| `hardcoded-count-in-test` | 2026-10-27 | TestActionsCarryTheirContracts |
| `plugin-list-hardcoded` | 2026-10-27 | TestHomeHeroDerivesFromTheManifest |
| `lint-contract-not-applied` | 2026-10-27 | changed-scope lint at e6474aa200 |
| `lint-contract-not-applied` | 2026-10-27 | le-placement-answers docker exec in terminaldemo test |
| `gate-verdict-depends-on-the-machine` | 2026-10-27 | job `InputHash` whole-tree diff killed by `gitTimeout` in the 9p QEMU guest |
| `gate-excludes-part-of-its-population` | 2026-10-27 | `unitNeedsGuest` keeps a `//go:build linux` CAP_NET_ADMIN unit on a host that cannot unshare a namespace, so `./le rfc record` cannot re-take it there |
| `bulk-rename-corruption` | 2026-10-27 | le doc index and verify status `write` callers |
| `bulk-rename-corruption` | 2026-10-27 | qemu action names lost `-test`, TestQEMUActionsIncludeTheHostRun |
| `bulk-rename-corruption` | 2026-10-27 | le-vendor-web-answers, le-checks-answers |
| `package-red-not-reproduced` | 2026-10-27 | harnesstool `setup docker` namespace panic |
| `package-red-not-reproduced` | 2026-10-27 | le-rules-answers render-update |
| `published-value-drifts-from-the-behavior-it-describes` | 2026-10-27 | hookSourcesDigest stale |
| `published-value-drifts-from-the-behavior-it-describes` | 2026-10-27 | wholetree-mentions ledger |
| `read-fails-on-a-fact-the-caller-does-not-need` | 2026-10-27 | TestSessionStartRendersNoDeferredArtifact |
| `fixture-encodes-an-impossible-state` | 2026-10-27 | TestPartialHealthExplainsScopedEvidence |
| `commit-needs-an-uncommitted-symbol` | 2026-10-27 | rfc/full/rfc7084.txt untracked |
| `claim-outlives-the-evidence-it-cites` | 2026-10-27 | features/ai-first-design.md Doc review |
| `lint-contract-not-applied` | 2026-10-27 | misspell and gocritic findings at 0723d7d26a |
| `session-never-establishes-with-the-peer` | 2026-10-27 | RPKI relay tests, `le mock rtr` never syncs a VRP |
| `artifact-bytes-disagree-with-identity` | 2026-10-26 | native executable corruption after content hashing |
| `test-artifacts-land-in-the-repository-root` | 2026-10-26 | job lint-input guard encounters another session's Go backup |
| `tool-help-contradicts-owner-rule.md` | 2026-10-25 | `le verify changed` help and page grant agents a run the Linux-host rule withholds (moved from the closed spec-le-cli-usability) |
| `acceptance-criterion-written-as-an-absence` | 2026-08-21 | AC-9 grep for `show bgp summary` |
| `blanket-mechanism-hid-missing-cases` | 2026-08-10 | Stop hook classifier |
| `bulk-rename-corruption` | 2026-08-10 | lint `unused` on l2tp |
| `closed-spec-survives-on-disk` | 2026-09-12 | `./le commit create ... remove` |
| `container-cannot-see-migrated-scratch` | 2026-10-21 | `Run.virtfsArgs` |
| `delegation-hands-the-callee-a-different-argv` | 2026-10-20 | `conf-watchdog` helper |
| `delegation-hands-the-callee-a-different-argv` | 2026-10-13 | `./le stress-repro` |
| `delegation-hands-the-callee-a-different-argv` | 2026-09-07 | `le` launcher script |
| `delegation-hands-the-callee-a-different-argv` | 2026-09-04 | `refuseWrongBuildName` |
| `delegation-hands-the-callee-a-different-argv` | 2026-09-05 | `./le commit create` session identity |
| `error-answered-with-another-errors-code` | 2026-10-10 | `DockerKernelRoute` |
| `escape-hatch-scoped-wider-than-its-justification` | 2026-09-27 | `writeGoPatterns` |
| `field-carries-two-meanings` | 2026-10-03 | `runOrchestrated` |
| `field-carries-two-meanings` | 2026-08-19 | `ZE_VERIFY_MODE` |
| `fixture-encodes-an-impossible-state` | 2026-10-05 | `internal/le/ai/hooks/fixtures.go` |
| `fixture-encodes-an-impossible-state` | 2026-09-05 | `le-qemu-run-answers.ci` |
| `fixture-encodes-an-impossible-state` | 2026-09-17 | `le-qemu-run-answers.ci` |
| `gate-ignores-the-level-it-counts` | 2026-09-30 | `checkGapCountAgreement` |
| `gate-ignores-the-level-it-counts` | 2026-10-06 | `./le rfc check` |
| `gate-owed-a-clearance-no-command-writes` | 2026-11-02 | `RFC-approval-owed:` cleared only by attestation |
| `gate-owed-a-clearance-no-command-writes` | 2026-09-08 | `debt-discharge` |
| `gate-owed-a-clearance-no-command-writes` | 2026-09-07 | `./le commit debt-clear` |
| `guard-added-to-one-half-of-a-pair` | 2026-10-10 | `(*Bench).Run` |
| `guard-added-to-one-half-of-a-pair` | 2026-10-26 | `runMessageLoop` generated bulk UPDATE attributes |
| `guard-added-to-one-half-of-a-pair` | 2026-09-07 | `runGating` |
| `guard-added-to-one-half-of-a-pair` | 2026-09-05 | `le verify` cancellation |
| `invariant-enforced-by-an-absent-call-site` | 2026-08-18 | spec WIP cap |
| `invariant-enforced-by-an-absent-call-site` | 2026-08-19 | `.et` wall clock |
| `lifetime-ends-before-its-last-reader` | 2026-10-22 | `runTest` reaping |
| `lifetime-ends-before-its-last-reader` | 2026-10-23 | interop image cleanup |
| `mutating-a-guard-runs-the-unguarded-path` | 2026-08-31 | forced-RED on `handlePacket` |
| `mutating-a-guard-runs-the-unguarded-path` | 2026-08-10 | build recipe guard |
| `net-pipe-deadlock` | 2026-09-03 | `captureStdout` |
| `non-idempotent-append-duplicates-the-record` | 2026-08-19 | verification-debt shard |
| `non-idempotent-append-duplicates-the-record` | 2026-08-29 | `./le commit create append` |
| `option-set-for-one-caller-changes-another` | 2026-10-10 | `runGokInProcess` |
| `option-set-for-one-caller-changes-another` | 2026-09-28 | `plugin/wait-file` |
| `option-set-for-one-caller-changes-another` | 2026-10-19 | `storage-bfd-restart.ci` |
| `output-not-byte-stable` | 2026-08-15 | chaos web golden capture |
| `output-not-byte-stable` | 2026-08-24 | terminal demo poster |
| `owner-decision-attributed-without-the-owner` | 2026-10-18 | `./le rfc approve` |
| `pointer-shared-across-the-names-it-indexes` | 2026-09-04 | `./le commit create ... append` |
| `pointer-shared-across-the-names-it-indexes` | 2026-09-05 | `./le commit create` same tag |
| `prohibition-broken-by-the-agent-that-carried-it` | 2026-08-31 | subagent `git mv` of a `.ci` |
| `record-written-before-the-operation-succeeds` | 2026-09-08 | `recordDebt` |
| `record-written-before-the-operation-succeeds` | 2026-08-19 | verification-debt shard |
| `refactor-removes-feature` | 2026-09-03 | four hook gates |
| `refactor-removes-feature` | 2026-09-03 | spec-write design gate |
| `refactor-removes-feature` | 2026-08-31 | `checkNoFamilyEndOfRIB` and 14 checkers |
| `removal-leaves-the-file-on-disk` | 2026-10-21 | `sweepAbandoned` |
| `removal-leaves-the-file-on-disk` | 2026-09-19 | `sweepAbandoned` |
| `removal-leaves-the-file-on-disk` | 2026-09-12 | `renderSharedIndexRepair` |
| `removal-leaves-the-file-on-disk` | 2026-09-07 | `renderSharedIndexRepair` |
| `removal-leaves-the-file-on-disk` | 2026-09-06 | `./le commit create ... remove` |
| `replacement-written-before-it-is-validated` | 2026-09-08 | `go mod vendor` |
| `rule-metadata-values-disagree-with-linter` | 2026-09-12 | rule corpus level |
| `silent-fall-through` | 2026-10-05 | `RunEngineSteps` |
| `silent-fall-through` | 2026-09-08 | `zeTestParseRunCLI` |
| `silent-fall-through` | 2026-09-07 | `./le qemu run` 9p mount |
| `silent-fall-through` | 2026-09-04 | YANG resolution in test binaries |
| `silent-fall-through` | 2026-08-15 | orphan `stdin=peer` block |
| `silent-fall-through` | 2026-10-08 | `le test qemu run` arm64 accel |
| `silent-fall-through` | 2026-10-10 | `.et` runner options |
| `silent-fall-through` | 2026-10-10 | `processCmdWithDepth` tick cost |
| `silent-fall-through` | 2026-10-10 | IPsec interop `zeEnvironment` |
| `test-gate-repeats-expensive-work` | 2026-10-07 | `./le test functional --pattern` |
| `test-gate-repeats-expensive-work` | 2026-09-12 | `hookSessionStart` |
| `test-gate-repeats-expensive-work` | 2026-09-11 | session-start hook timeout |
| `test-mutates-a-running-subject` | 2026-10-15 | `TestUnionTimeout` |
| `test-type-not-backfilled` | 2026-10-06 | RFC filename/tag convention |
| `test-type-not-backfilled` | 2026-06-14 | new test type, existing code |
| `test-type-not-backfilled` | 2026-06-14 | fuzz target backfill |
| `helper-bypassed-by-an-open-coded-copy` | 2026-08-14 | functional runner `Record.TmpfsFiles` flattens the tmpfs block |
| `helper-bypassed-by-an-open-coded-copy` | 2026-08-31 | `activityStyle` light literals under the dark theme |
| `parser-refuses-a-form-its-producer-emits` | 2026-10-14 | `checkSnapshot` reads `git ls-files -s` without `-z` |
| `policy-change-never-re-evaluates-what-it-admitted` | 2026-10-22 | GoBGP `UpdateConfig` export change not re-advertised (lab peer) |
| `stop-leaves-the-work-running` | 2026-10-11 | `runDocker` kernel container survives a stop |
| `stop-leaves-the-work-running` | 2026-10-20 | `installStopped` always says "only partly made" |
| `stop-leaves-the-work-running` | 2026-10-20 | `installCommand` root step waited out with no WaitDelay |
| `stop-leaves-the-work-running` | 2026-10-20 | unrooted le verbs leave the compile or docker child running |
| `test-against-broken-path` | 2026-08-18 | `demos/terminal/web-config/` validator and recording drive different paths |
| `test-against-broken-path` | 2026-08-29 | `TestGeneratedPluginImportsCurrent`, `TestYANGGlueCurrent` `go run` a non-main file |
| `test-against-broken-path` | 2026-09-01 | `TestProbeUnclaimedRoutes`, `TestScratchProducerCensus` never deleted |
| `test-against-broken-path` | 2026-09-05 | `requireEveryJobAdvisory` against `evidence-nightly.yml` |
| `test-against-broken-path` | 2026-09-28 | `le-discovery-answers.ci` `gitChangedFiles` compares the working tree |
| `unwired-feature` | 2026-09-02 | `.github/workflows/codeql.yml` `paths-ignore` ignored for Go |
| `zero-value-as-valid-answer` | 2026-08-26 | le toolchain derived feature tag set answers empty |
| `zero-value-as-valid-answer` | 2026-09-21 | `latestComparisonResults` keeps one of two `ze` results |
| `zero-value-as-valid-answer` | 2026-10-15 | `listStdlib` empty listing read as no stdlib |
| `bound-too-small-for-its-own-burst` | 2026-10-22 | stressrepro `runSlug` joins every selector into one filename |
| `bound-too-small-for-its-own-burst` | 2026-10-25 | `le job run` stall window breaks a silent gomu run |
| `bound-too-small-for-its-own-burst` | 2026-11-02 | web test runner's first `agent-browser open` bound widened only in verify mode |
| `declared-format-contradicts-payload` | 2026-08-31 | session state file `Uncommitted:` section is a whole-tree `git status` |
| `declared-format-contradicts-payload` | 2026-09-01 | `rfc/not-enrolled.txt` `source-restricted` kind refused by `./le rfc check` |
| `gate-excludes-part-of-its-population` | 2026-08-22 | a spec's Files to Modify list read as the population of a wire's speakers |
| `gate-excludes-part-of-its-population` | 2026-09-27 | `rfc/short/` holds no summary for eight implemented documents |
| `gate-red-where-nothing-blocks-on-it` | 2026-08-30 | `./le verify lint run scope ./internal/le/site/...` 18 goconst findings |
| `gate-red-where-nothing-blocks-on-it` | 2026-08-31 | `./le cli-grammar` is in no blocking set |
| `gate-red-where-nothing-blocks-on-it` | 2026-09-10 | lint after OSPF closure: 93 host, 104 Linux findings |
| `gate-red-where-nothing-blocks-on-it` | 2026-09-10 | repository check, commit audit and doc check verify red over concurrent work |
| `gate-red-where-nothing-blocks-on-it` | 2026-09-17 | `le-docvalid-answers.ci`, `le-evidence-answers.ci` red at HEAD |
| `gate-red-where-nothing-blocks-on-it` | 2026-10-05 | `pluginOwnershipGate` added with no baseline, `le-placement-answers` red |
| `published-value-drifts-from-the-behavior-it-describes` | 2026-10-24 | `lab pin` filed under reports while `bump` writes |
| `validate-accepts-what-start-refuses` | 2026-10-26 | `./le test weakened approve` records a unit key (`airules.TestX`) the commit gate never matches |
| `reference-checked-claim-unchecked` | 2026-10-26 | hook fixture needle names the retired `./le ai rules update index` (`internal/le/ai/hooks/fixture_catalog.go`) |

## Rows found already fixed

Rows from the whole one-off sweep found fixed at HEAD on 2026-10-23 that have no
other spec home here. Each needs its journal cell closed with this evidence by
the agent that closes fixed rows.

| Class file | Date | Surface | Evidence |
|------------|------|---------|----------|
| `silent-fall-through` | 2026-09-06 | `parseCIFile` | `internal/test/runner/parsing.go` now refuses with "unknown directive" |
| `guard-added-to-one-half-of-a-pair` | 2026-09-21 | RSVP-TE `buildResv` | `internal/plugins/rsvpte/build.go` `buildResv` writes the filter through `encodeFilterSpec` |
| `protocol-state-advanced-before-the-peer-confirmed` | 2026-09-28 | LDP `handleInit` | `internal/plugins/ldp/session.go`: `initAccepted` gates operational on the peer's KeepAlive (`onOperational`, RFC 5036 Section 2.5.3) |
| `silent-fall-through` | 2026-09-21 | LDP `DecodeInit` U-bit | `internal/plugins/ldp/wire.go`: `unknownTLVError` for an unknown TLV with the U bit clear, status `statusUnknownTLV` |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (red recorded per AC)
- [ ] Tests PASS
- [ ] Scoped runs: `./le job run go test ./internal/le/... ./internal/test/...`, `./le test functional ui`

## Research reconciliation — 2026-11-02

The owner selected full-spec reconciliation before implementation. Status remains
`design`. The original sweep description and criteria above are historical inputs
to this reconciliation, not an approved implementation plan for today's tree.

The owned table now contains 138 entries. Its original count of 70 no longer
describes that population. Some entries summarize multiple journal occurrences.
Each occurrence remains owed until its current producer and proof are reconciled.
An existing `fixed:` marker is a recorded claim, not a newly observed test pass.

### Research packages

These packages divide research within this spec. They neither transfer ownership
nor remove an obligation. AC-1 has its own initial runner audit.

| Package | Existing criteria | Additional ownership |
|---------|-------------------|----------------------|
| A: commit obligations | AC-5, AC-6, quoted paths in AC-17 | Late RFC approvals, committed-content review pins, approval identity validation, tracked RFC7084 text. Coordinate commit-script identity with C |
| B: editor and process harness | AC-2, AC-3, AC-11, pipe/race cases in AC-13, tmpfs in AC-17 | Browser startup bounds and the restart/store-reader fixture population |
| C: launcher and toolchain | AC-4, AC-14, AC-20, AC-21 | Site inventory built from committed source, rollback argument preservation, backup files and lint inputs |
| D: cancellation and guests | AC-7, AC-10, AC-18 | Functional gating cancellation, guest hashing, silent jobs and appliance working-directory restoration |
| E: hooks and process safeguards | AC-8, AC-15, AC-16, AC-22, AC-24 | Hook digest and fixture text, whole-tree ledger, deferred-artifact fixture and superseded verification-host rule |
| F: RFC accounting | AC-9, AC-23, AC-25 | Guest placement for namespace-dependent RFC proofs |
| G: interop and generated peers | AC-12, AC-28, GoBGP in AC-17 | RPKI mock synchronization and the recorded-fixed RSVP-TE/LDP occurrences |
| H: fixtures and gates | AC-19, AC-26, remaining AC-13/AC-17 cases | Remaining lint, grammar, rename, population, publication and fixture occurrences |
| I: executable integrity | AC-27 | Recovery of damaged bytes and attribution to the responsible writer or storage boundary |

### Direct observations

These observations were checked in the supervising session. Source inspection
does not establish runtime proof unless the row names an executed command.

| Requirement | Evidence | Remaining obligation |
|-------------|----------|----------------------|
| Unknown BGP test verb fails | `./le test bgp ac1-unknown-verb` printed `Unknown command: ac1-unknown-verb` and exited 0. `internal/test/cli/cmd_bgp.go` `zeTestParseRunCLI` returns nil, and `zeTestBgpMain` treats it as success | Separate invalid input from help and prove a nonzero result through the real entry point |
| Unknown engine-step kind fails | `./le mock engine-steps -`, with a JSON array containing kind 255 on stdin, exited 1 with `ZE-OBSERVER-FAIL`, `parse steps file`, and `step 1: unknown kind 255` | Retain the existing refusal in `internal/test/runner/engine_steps.go` `UnmarshalEngineSteps`. Verify the file-input and execution branches before closing the occurrence |
| Peer assertions have a consumer | `internal/test/runner/peer_contract.go` `blockPeerMode` returns unread when no peer command names the block. `validatePeerBlockRejects` refuses this only when rejection directives exist | Reproduce an orphan positive expectation and prove block ownership without breaking the legacy non-orchestrated path |
| Late RFC approval can reach a commit | `internal/le/commit/rfcchange.go` `rfcChangeProblems` emits a recorded approval only for a matching current change. `owed_trailers.go` `answering.answered` consumes approval trailers | Design the missing publication route and bind approval to the intended reviewed change |
| Review pins describe committed content | `internal/le/commit/review.go` `checkReviewSelected` hashes the working-tree path when the current hunks selection does not name it | Reconcile recording, preparation, appended blocks and execution against the same selected content |
| Owner authority is distinct from judge prose | `internal/le/rfc/approve.go` `ApproveInto` checks reason syntax. `coverage_ruling.go` `ownerRuledMove` accepts matching ruling identifiers extracted from approval and annotation text | Establish how owner authorization is evidenced at the consumer. Matching text alone does not establish authorship |
| Panic exemptions apply per call | `internal/le/hookruntime/writeedit.go` `writeGoPatterns` searches the entire edit for an allowed panic prefix before skipping the panic finding | Prove that an allowed call cannot exempt another call |
| Failed guest mounts stop bootstrap | `internal/le/test/qemu/run.go` `setupCommand` joins fragments with `&&` while fragments contain ungrouped `||` alternatives | Execute a failed-mount fixture and require that later download/build steps never start |

### Constraints for the revised design

| Constraint | Effect on the existing criteria |
|------------|---------------------------------|
| Current owner rules govern historical journal instructions | Do not restore a superseded Linux-host restriction, debt shard, prose-sensitive gate or destructive cleanup heuristic |
| Every owned occurrence needs explicit proof | Extend the test plan beyond AC-16. Disambiguate repeated class/date/surface rows by their symptom |
| A recorded rule is not proof of repair | AC-16 must preserve concrete backfill populations and unsafe-mutation obligations |
| Test removal and weaker assertions need their governing review | AC-17 and AC-19 cannot close a defect merely by explaining missing behavior or deleting its test |
| RFC enrollment follows current profile scope | Reconcile all eight AC-25 documents without enrolling unlisted RFCs. Keep ISO10589 outside conformance figures |
| Scoped evidence and release evidence are distinct | Replace the broad test command above with reached packages, tags and named fixtures. A baseline does not discharge release reds |
| Existing mechanisms take precedence over historical representations | Judge tmpfs content/mode preservation and child executable identity, rather than requiring a pointer type or indiscriminate build-name propagation |

### AC-27 evidence blocker

The earlier statement that session `644a4e78-0122-4eeb-8642-eb50638a1641`
retains both damaged executables is not verified today. Its recorded directory,
`tmp/session/2026-10-22-644a4e78-0122-4eeb-8642-eb50638a1641/scratch/`,
currently lists only `appliance-amd64-proof/`. Neither named archive is present
there. A bounded recovery search reported no replacement location. This does not
prove that no backup exists.

The owner has been asked for another copy or a host snapshot. AC-27 remains open.
Recovery or a recurrence observed across the build/install boundaries is needed
to identify the cause. A successful rebuild or a new integrity check cannot,
by itself, demonstrate repair of the recorded incident.
