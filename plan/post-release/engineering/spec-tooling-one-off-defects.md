# Spec: Tooling, hook and test-harness one-off defects from the journal

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P3 |
| Release reason | Each row is a defect in `le`, the hooks, the commit and verify tooling, or the functional and editor test harness; none changes the shipped binary, so it does not block release correctness |
| Depends | - |
| Phase | 4/13 |
| Handoff | - |
| Updated | 2026-11-03 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Design approval — 2026-11-02

The owner answered the final gate exactly:

> Approve design (Recommended)

Approval covers terminal checks before recording owner-ruling RFC approvals,
attestations, owner-authorised review records and push preparation. Ordinary
judges, reviews without owner authorization, existing consumers and prepared push
scripts remain agent-usable. TTY detection is an operational boundary, not identity
proof. The design uses the existing lifecycle mechanisms and adds no new owner
record or `rfc authorise` command.

The implementable phases are ready. Implementation begins with phase 1, the
false-green harness refusals. Historical incidents and missing external evidence
remain open closure blockers; they do not prevent starting the approved phases.
Earlier references below to pending design approval describe the review history
and are superseded by this gate. This checkpoint records approval only; it claims
no implementation, runtime proof or incident closure.

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
| `container-build-leaves-root-owned-scratch` | 2026-11-04 | poster proof default entrypoint rewrites shared SSH config and chowns shared scratch |
| `gate-excludes-part-of-its-population` | 2026-11-04 | remaining site and PPPoE AC integration declarations after host-clock routing |
| `container-build-leaves-root-owned-scratch` | 2026-11-04 | web-config child cwd writes root-owned audit output into checkout |
| `error-path-discards-data-already-received` | 2026-11-04 | web-config startup failure removes its daemon diagnostic |
| `gate-red-where-nothing-blocks-on-it` | 2026-11-04 | tracked historical filter-a6 shell recipe is the new native-helper census red |
| `claim-outlives-the-evidence-it-cites` | 2026-11-03 | weakening approval keyed by unit name is reused for a later change |
| `earlier-guard-hides-the-better-error.md` | 2026-11-03 | `commit create hunks` with a zero-context patch reports an unowned row, not the patch failure |
| `gate-fires-outside-its-population.md` | 2026-11-03 | weakening detector `ignoreTagPattern` (`internal/le/test/weakened/detector.go`), a `//go:build ignore` inside a fixture string |
| `helper-bypassed-by-an-open-coded-copy` | 2026-11-02 | weakening and RFC gates scan Go test units twice |
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

## Reconciled design candidate — 2026-11-02

This section is a design candidate, not design approval or implementation evidence.
It supersedes the earlier AC wording and test plan for future work, while keeping
the original rows and evidence intact. The owner requested full-spec reconciliation;
no owned obligation is removed, transferred, or declared done here. The 138-entry
table is an ownership index, not a count of distinct defects or completed proofs.

### Execution constraint

Development and testing take place in the main checkout only, never in a worktree,
export, or copy. No agent runs any `./le verify` command, including `changed`.
GitHub CI/CD verifies a pushed commit. Agent evidence consists of directly run,
scoped tests of the reached packages, tags, and named functional cases. A build
broken by another session is reported and waits for that cause to clear; it does
not move to another tree. These current owner rules supersede contrary instructions
in the historical criteria, journal fixes, skills, and pages cited above.

This reconciliation ran no builds or tests and made no code, test, or journal
change. The parent's upstream check reported zero commits behind. Source inspection
is not an executed proof. Retirement of worktree-based testing does not erase the
owned reclamation incidents: research their current resource owner and provide an
owner-authorized reclamation route without starting verification or touching foreign
work. Silence, a timeout, or an old marker never grants ownership of a process.

### Evidence boundary and current-source audit

#### One-off owner decision — 2026-11-02

When asked whether owner-ruling handling needed reviewed provenance or
authentication, the owner replied exactly:

> no changes it works as it is

The owner then clarified exactly:

> making sure the auth can only be run from interractive terminal so it means I have run it

The parent recorded the standing rule in `ai/META-DECISIONS.md` at `8e9e99805c`:
owner-only authorization actions require an interactive terminal and refuse
noninteractive invocation before recording anything. Independent judge approvals
retain their agent route. The earlier decision therefore excludes authority and
provenance redesign, but includes this terminal-only gate. This is an operational
boundary, not cryptographic proof of identity. It approves no future coverage loss.
The October 18 occurrence remains recorded. P-9 owes the terminal boundary as well
as level/rule-metadata accounting. The direct producer reconciliation below selects
existing authorization routes and terminal detection, without a new ledger. Canonical
RFC approval identity, late publication and change binding remain in scope.

Parent session: `ses_e640edc6effeAAkQ560zO43PMs`; assigned scratch:
`tmp/session/2026-11-01-180915/scratch`. The owner's broader “ok do it all” request
sets the goal of design closure and implementation. This editing phase remains
spec-only; parent independent review and the main approval gate precede a status
transition from `design` or any implementation.

`S` means the producing function was read in this editing phase after locating its
symbol with `gopls`. `R` means an earlier audit report or parent observation is a
lead awaiting producer verification in the implementation audit. `P` means an
executed, retained proof; this phase adds none. A reported source repair remains
implemented-but-unverified, not done. The earlier Direct observations table retains
its own attribution and does not become a new observation by repetition here.

| Requirement | Audit status | Producer and observation | What would settle it |
|-------------|--------------|--------------------------|----------------------|
| Guarded removal | Partial, S | `internal/le/commit/script.go` `renderBlock` calls `renderWorkingTreeRemoval` after publication; the latter compares the staged working copy with the removed entry and preserves differing content or mode | P-5 through the generated script, including publication failure and concurrent edits |
| Late approval | Missing publication route, S | `internal/le/commit/rfcchange.go` `rfcChangeProblems` emits a trailer only when a recorded row matches a current change; `internal/le/commit/owed_trailers.go` `answering.answered` accepts `acts.approved[subject]` | P-30 must prove change binding and ordering, not only matching unit names |
| Closure review content | Partial, S | `internal/le/commit/review.go` `checkReviewSelected` reads selected hunks but otherwise calls `reviewHash` on the working-tree file, including historical spec-work paths | P-29 compares the review with the actual prospective committed content |
| Generated review identity | Partial, S/R | `reviewCheckCommand` in that file emits spec and file words, not an explicit review artifact identity; ambient-session selection is the earlier audit report | Trace the action and artifact resolver, then P-29 across two harness sessions |
| Owner attribution | Source observed; terminal-only boundary required | `Approve`/`ApproveInto` write existing rows; `ownerRuling` and `ownerRuledMove` interpret their reasons. `attestationTrailer`, `preparePush` and `recordReview` with OwnerAuthorised record existing owner instructions | Apply the terminal check at issuance as specified below, including supplied review authorization below the round cap; retain ordinary judge/review routes, consumers and prepared push-script execution |
| Editor timeout | Partial, S | `internal/component/cli/testing/runner.go` `testBudget` reads timeout/headroom; `runTestCase` returns on the timer without stopping or joining `runTestCaseWorker` | P-2 proves cancellation and worker completion before resources are released |
| Editor dimensions | Partial, S/R | `runTestCaseIn` consumes width/height options when present; earlier audit reports that parsing discards those dimensions and waits | Trace parser-to-consumer delivery before deciding where to repair P-2 |
| Stress capture name | Missing bound, S | `internal/le/test/stressrepro/run.go` `runSlug` joins every sanitized selector with no length limit | P-21 uses the recorded 91-selector burst and collision cases |
| Other owned surfaces | Unverified, R | The package leads below and original journal producers identify the investigation targets | Each occurrence needs its producer, current disposition, and its P-number proof before closure |

The recent history includes `4abc6d4740`, `95eb632699`, and `f2624c51e8` for the
execution constraints. Their presence is historical context, not proof of tooling
enforcement. This audit deliberately makes no aggregate Done/Partial/Missing count
for uninspected producers. The occurrence map is complete for the 138 index entries;
the semantic populations and historical evidence gaps listed below are not complete.

### Existing mechanisms and minimum extensions

These rows extend the earlier Existing Mechanisms section. A conditional proposal
depends on the named research result; it is not an instruction to patch an
unverified producer. The doors read for this pass cover tooling, commands, appliance,
hub, core, config, plugin infrastructure, API surfaces, BGP, access/security,
system plugins, routing, and forwarding. Their shared constraints remain: one
registry, explicit process ownership, external-plugin boundaries, one daemon store,
and no product dependency on `internal/le`.

| Owner | Existing mechanism and source | Minimum candidate extension |
|-------|-------------------------------|-----------------------------|
| A | Private-index snapshots, selected hunks, generated commit blocks, review population and Git-derived obligations; `docs/contributing/committing.md`, `internal/le/commit/review.go` `checkReviewSelected`, `rfcchange.go` `rfcChangeProblems`, `owed_trailers.go` `answering.answered` | Use the same selected content for recording, preparation, append, execution, and discharge. Extend the existing approval writer/consumer pair for a late approval, bound to the reviewed change. Keep guarded post-publication removal and exclusive message allocation. Do not recreate debt shards |
| B | Shared scheduler plus `.ci`, `.et`, and `.wb` adapters; `docs/architecture/testing/runner-architecture.md`; editor `testBudget` and `runTestCase` | Keep one scheduling policy and carry its context, capability requirements, and exclusive groups through adapters. Join canceled workers or isolate an uninterruptible case in an owned process after the blocking-call audit. Use explicit persistent stores only where the assertion needs disk state |
| C | Launcher executable selection, functional suite registration, stress capture names; `runSlug` and `harnessArgs` in `internal/le/test/stressrepro/run.go`; `docs/contributing/running-commands.md` | Preserve argv and the intended executable identity, not every obsolete build-name field. Bound the capture component with a readable prefix plus digest of the full selector identity; retain full selectors in the report. Other toolchain/report changes await their producer audit |
| D | Signal roots, owned process groups, Docker owner labels, job admission, guest bootstrap; runner architecture page, QEMU `Run` | Extend these existing lifetimes to missed callers. Stop and reap only owned work; report install progress from completed steps. Add no silence-based foreign-work killer. A reclamation action must not launch a suite |
| E | Native hook probes, spec status validation, session-state renderer, source-read evidence and test seeder; `docs/contributing/spec-workflow.md`, hook `writeGoPatterns`/`writeDesignEvidence` | Repair the verdict at its consumer, with positive and negative probe cases. Match panic exemptions per call. Prove subject/freshness rather than accepting an arbitrary read marker. Apply state invariants on direct edits as well as claim transitions |
| F | RFC profiles, extraction, correction records, native approval and discrimination routes; `coverage_ruling.go` `ownerRuledMove`, `docs/contributing/rfc-conformance-gates.md` | Keep mandatory and optional accounting distinct; use current profile membership, not citation reachability. Preserve unenrolled obligations in their correction records. Explicit kernel selection uses the compatible guest even for a host-buildable unit. Owner-ruling authority stays unchanged under the one-off decision |
| G | Registered special checkers, scenario runner, peer OPEN/session state and generated UPDATE builders; `docs/architecture/testing/interop.md`, `internal/test/peer` | Restore behavior and discriminating oracles through each registered checker, not another checker list. Carry negotiated session kind into generated bulk builders; leave deliberate raw-hex injection unchanged |
| H | Registered actions, generated manifests, source-aware fixtures, existing gate populations and browser/chaos capture paths; functional-test and runner pages | Derive membership from producers, preserve assertions, and repair stale consumers with their producer. Replace the job-admission fixture's timed overlap with a bounded state rendezvous if the reported source is confirmed. Gate greens alone do not resolve historical lint or load populations |
| I | Go content-ID comparison and the recorded native-build/install observations in Executable integrity incident | Retain bytes and boundary traces, identify the writer or storage boundary, then prove the attributable repair. No replacement integrity framework or successful rebuild substitutes for attribution |

The testing history in `plan/learned/DESIGN-HISTORY.md`, Testing infrastructure,
rules out count-only assertions, invented fixture shapes, and treating a green
parser as wire proof. Existing mechanisms are extended in place. No parallel
ledger, scheduler, approval authority, or test framework is proposed.

### Corrected acceptance criteria and proof matrix

Each P-number is a proof obligation, not the name of an existing passing test.
The concrete carrier plans below distinguish existing carriers from proposed tests
and unresolved populations. They are not a completed proof census. Implementation
must retain attributed red and green evidence. Every row remains open. An already-repaired producer needs
its existing regression proof checked, not a second implementation. The retained
owner-ruling occurrence has the explicit no-redesign plus terminal-gate disposition above. A tagged test
change still needs independent judgment and native discrimination evidence.

| AC / proof | Owner | Corrected observable and required proof |
|------------|-------|-----------------------------------------|
| AC-1 / P-1 | B | Unknown suite verbs, JSON engine kinds from file/stdin, and unconsumed positive or negative peer blocks fail at the real entry point with the input named. Exercise direct execution as well as decoding; retain the legacy non-orchestrated peer route. Distinguish help from invalid input |
| AC-2 / P-2 | B | `.et` width, height, waits and timeout reach their consumers or are refused. Test nondefault dimensions and explicit wait state; trace real tick costs rather than simulating away editor behavior. A timeout stops owned work and joins it before cleanup, with invalid and nonpositive budgets refused. Prove a hung case cannot continue changing state after its result |
| AC-3 / P-3 | B | Every return, including foreground timeout, stops and waits for owned background helpers before releasing leases. File- and pipe-routed peer scripts retain linger semantics. A passed interop assertion followed by cleanup failure remains a pass plus an explicit cleanup error in text, structured output and final status; it must not report an unqualified successful run |
| AC-4 / P-4 | C, A | Launcher, wrong-build refusal, conf-watchdog, stress reproduction and generated commit re-checks preserve argv and the correct executable/session identity. Invoke from a differently named build/session and use arguments containing spaces. Stress runs route to the registered suite and each invocation actually starts rather than attaching to a stale verdict. A quoted rollback diagnostic is not argv execution |
| AC-5 / P-5 | A | Generated `remove` deletes only unchanged working copies after successful publication, including two-block spec closure; it preserves changed bytes/modes, directories and failed-publication copies. Same-tag prepare/append calls allocate independent messages and do not duplicate obligations. Prove both ordinary and replace/append occurrences against the current Git-derived mechanism |
| AC-6 / P-6 | A | Every still-applicable review/approval obligation has a legitimate writer and an observable discharge. Failed validation creates no debt or spent approval. Historical incorrect/duplicate shard records cannot influence current answers. Test retry, failed publication and another session's correction through the successor, without restoring retired shard commands. Late RFC approval additionally owes P-30 |
| AC-7 / P-7 | D | Reclamation handles interrupted removal, directoryless owner markers and the standalone entry point without verification. Prove ownership/refusal for foreign resources. Cancellation reaches an in-flight stage and stops and reaps its owned process group within seconds, including a TERM-ignoring child; it does not wait for the next stage or the stage's normal timeout. Reuse the group-kill and bounded pipe-wait contract detailed under P-7 below. No agent invokes verify to prove it; use the scoped dispatcher/process fixture |
| AC-8 / P-8 | E | Stop classification, per-call panic exemption, subject/freshness design evidence, daemon-path proof demand and the four lost hook categories each get a current rule-to-consumer disposition and nonvacuous probes. Do not reinstate prose-sensitive deferral gates. Session start emits required context inside its 5-second harness budget; source digest, deferred-artifact fixture and fixture needle match the actual producer |
| AC-9 / P-9 | F, E | Mandatory/optional gaps retain their levels; rule metadata agrees with the body's obligation. Preserve owner-ruling interpretation without authority/provenance redesign. Gate existing owner-ruling RFC approval issuance, owner attestation, push preparation and spec review record whenever owner-authorised is supplied, before any writes, regardless of review round cap. Ordinary judge approvals, reviews without owner authorization, consumers and already-prepared push scripts remain noninteractive. Push preparation records the owner instruction; script execution preserves that owner-ordered route. A PTY can satisfy the operational boundary: no identity or future coverage-loss approval is inferred; P-30/P-31 remain independent obligations |
| AC-10 / P-10 | D | Guest cache sharing works with the supported symlink layout; attribute the xattr failure before changing mount policy. Native arm64 Linux selects available KVM rather than `hvf:tcg`. A failed required 9p mount prevents all later download/build steps. Relative appliance paths survive in-process gok directory changes. A kernel probe's `unknown` result names a probe failure, never a missing kernel feature |
| AC-11 / P-11 | B, D, E | Hook probe repositories hold readable committed originals. Both QEMU fixture incidents preserve a declared Go version and the intended fixture root. Wait-file callers and driver agree on a per-run marker. BFD restart uses the store whose persistence it asserts. Demonstrate each through its real fixture, without removing fail-closed producer checks |
| AC-12 / P-12 | G | Each of the fifteen historical special checkers named below has a live scenario and an assertion that fails when its feature is broken. IPsec logging uses the key its logger reads and the checker observes the intended event. A build-only Ze DUT stages the same valid inputs before image construction as a measured run; stale/missing staged inputs fail the proof. All five `test/plugin/` fixtures `bgp-rr-rpki-pending-withdraw-relay.ci`, `bgp-rr-rpki-revalidate-relay.ci`, `bgp-rs-rpki-pending-withdraw-relay.ci`, `bgp-rs-rpki-revalidate-relay.ci`, and `rpki-revalidate-late-sync.ci` must complete actual RTR synchronization from the observer's per-test release file and reach their route-state/relay assertions; listening alone is not synchronization |
| AC-13 / P-13 | B, H | Output capture drains concurrently beyond pipe capacity. Union timeout setup precedes worker start. Chaos captures use controlled clocks and total ordering at every relevant producer. Poster output is stable for fixed inputs. Prove each at the producer and capture boundary; repeated green snapshots alone are insufficient |
| AC-14 / P-14 | C | Resolve the original l2tp unused population, preserve owned patches through the supported vendoring route or refuse unsafe replacement, and link every schema module a test binary resolves. Derive tags/registration from the current producer. One l2tp lint pass is not proof that vendor or schema obligations were met |
| AC-15 / P-15 | E | Current verify-mode readers agree on one meaning. WIP state is checked on direct spec edits as well as claims. Functional selection accepts the documented form or its page is corrected to the real form with equivalent selection proof. Agent guidance obeys the current no-verify rule on every platform, rather than restoring the retired Linux-only restriction |
| AC-16 / P-16 | E | Replace absence-only criteria with positive behavior and explicit permitted survivors. Preserve the six historical spec repairs and the separate June technique/fuzz backfill populations. Prove git-safety enforcement for delegated renames. Mutation proofs never run destructive actions on operator state or edit a live shared guard; prove the actual seeder entry leaves the operator store unchanged even when its outer guard is bypassed |
| AC-17 / P-17 | A, B, G, H | Preserve tmpfs content and executable modes through parse and ordinary `.ci` routes, without mandating a particular pointer type. Activity styling follows light/dark theme. Git-quoted paths round-trip through snapshot, selection and script. GoBGP export reload reconciles OUT advertisements with source-backed expectations. CodeQL Go extraction excludes the intended cache population by an effective Go mechanism. Each subcase has independent proof |
| AC-18 / P-18 | D | Kernel containers and six unrooted compile/lab routes stop with their own verb and reap owned children before cleanup. Installer cancellation reports completed steps, pre-first-step and reboot-sensitive state; restore second-interrupt default handling after the first stop, the original N6 alternative to bounded waiting. Preserve terminal-attached sudo; identify unfinished work, start no later step, and never claim sudo exit proves privileged-child completion. Retain foreign resources |
| AC-19 / P-19 | H | Browser validation and recording drive the same login/config/save path. Generator tests invoke actual generators, probes have deliberate retained or retired coverage, workflow assertions match intended job policy, and discovery compares the same committed/working population as its producer. Retiring a test requires its judgment and replacement-coverage accounting, not merely an explanation |
| AC-20 / P-20 | C | Empty feature-tag derivation and successful-but-empty stdlib listing fail with cause. Comparison retains latest valid selection within DUTName/Family/ForceMP/Pass, including distinct and unrecorded passes. Markdown, HTML/table/SVG, PerformanceDoc and Trend validate every raw input before selection or output; any nonzero routes-lost refuses convergence/ranking, including an older lossy row a newer row would hide. Missing convergence is not explicit zero. Compare writes stdout, with no JSON comparison format; raw JSON/NDJSON retains diagnostic loss evidence. File publishers replace each file atomically, not all artifacts as one transaction |
| AC-21 / P-21 | C | A 91-selector invocation fits the filesystem component bound, preserves exact argv in evidence, and actually starts each requested run. Test different long lists with the same prefix, sanitization collisions and fresh invocations; use full-selector identity, not truncation alone |
| AC-22 / P-22 | E | Session-state output distinguishes session-attributed edits from whole-tree status. If attribution is unavailable, label the section as shared-checkout changes. A fixture with another session's paths cannot silently attribute them to this session |
| AC-23 / P-23 | F | Every emitted not-enrolled kind is accepted or explicitly migrated by the current reader. ISO/IEC 10589 remains outside conformance counts. Prove writer-reader agreement and publication, not only acceptance of one hand-written line |
| AC-24 / P-24 | E | The spec template and design check require an evidence-derived census of every reader and writer of a changed wire, including non-Go/outside-language carriers. Files to Modify is the edit set, not the population. A fixture that omits a known speaker must fail the coverage judgment |
| AC-25 / P-25 | F | Account separately for ASPA-profile and RFC 9117, 4506, 5080, 7383, 8247, 7519 and 1008. Enroll no unlisted document. Preserve each retired obligation and correction provenance. Verify RFC9117's reported repair, RFC7383's optional debt, and RFC8247 method14's reported missing SHOULD row against authoritative text and current profiles, with extraction/tag/discrimination proof where applicable |
| AC-26 / P-26 | H | Reconstruct each owned historical lint/gate population and resolve its members against current producers. Include post-OSPF host/Linux findings, capability and uncovered-file findings, wiring/audit/doc failures, all three September 17 fixtures, and plugin ownership. A baseline or a green gate at HEAD does not erase an unidentified member. Scoped repairs precede retained CI release evidence |
| AC-27 / P-27 | I | Recover the two damaged executables and traces, or observe recurrence across build/install/storage boundaries. Attribute the zeroed bytes to the responsible writer or boundary and demonstrate that repair. This remains evidence-blocked; neither an integrity check nor a successful rebuild closes it |
| AC-28 / P-28 | G | Real OPEN handshakes prove IPv4/iBGP, IPv4/eBGP, IPv6/iBGP and IPv6/eBGP generated bulk UPDATEs. Cover active/passive and ordinary/connmap paths: LOCAL_PREF present on ordinary iBGP and absent on eBGP. Deliberate raw-hex injection remains byte-exact. An encoder-only test cannot certify handshake-derived classification |
| AC-29 / P-29 | A | Review pins cover actual prospective committed content for historical spec-work paths, selected hunks, whole snapshots, earlier appended blocks and removals. Preparation and execution select the same artifact under different ambient sessions. Concurrent unselected edits neither stale a committed-content review nor become reviewed implicitly; missing/unreadable content refuses |
| AC-30 / P-30 | A | A late independent RFC approval reaches the applicable owed change without changing the test again or impersonating the owner. Bind target change, unit, reviewed content and publication order; reject stale/replayed approvals, wrong targets and approvals outside the pushed range. Do not let one unit-name approval clear a different change |
| AC-31 / P-31 | A | Approval entry validates canonical carrier/unit identity before writing. Package-clause names such as `airules.TestX` or `bgpconfig.<unit>` cannot record an unusable success when the consumer uses a directory-based key. Ambiguity and unknown units refuse; a valid row reaches both edit and commit consumers |
| AC-32 / P-32 | C, H | Site inventory and coupling judgments use a declared, complete source/platform population, not whatever plugins the host happened to link or another session left half-written. Resolve the committed-source mechanism under the main-checkout rule before implementation. Prove Linux-only registrations remain represented on darwin and unrelated dirty edits cannot become published inventory |
| AC-33 / P-33 | B, E | Derive the complete persistence/store-reader fixture population semantically. Disk-dependent cases use an isolated persistent store; memory-only cases retain memory isolation. Seed and start resolve the same effective root across dotted/uppercase env spellings. Seeder entry proves session ownership, symlink confinement and operator-store nonmutation; lexical path shape alone is insufficient |
| AC-34 / P-34 | H, E | Action contracts, homepage demos, renamed callers, lab grouping, partial-health fixtures, feature Doc review and rules fixture roots agree with their current producers. Assert membership/behavior rather than a frozen count. Renew evidence only after checking the claim; preserve the historical cause gap where a fixture repair has not attributed its old red |
| AC-35 / P-35 | H, B, D | Recover each owned load-red member and identify the exact wait, readiness event, race or resource bound that produced its failure. Prove the repair under the reached contention path. No adaptive host-load policy is approved; neither a larger blanket timeout nor 'green alone' is closure |
| AC-36 / P-36 | D | Job admission never signals foreign silent work. Preserve cancelable admission for owned work, and prove the QEMU guest inherits the host admission identity without a whole-tree 9p fingerprint. A blocked or unreadable identity is an explicit error, not permission to start or kill |
| AC-37 / P-37 | C | Determine which backup/scratch Go files are actual lint inputs and which are preserved evidence. The guard and producer use the same population, without deleting another session's backup or broadening lint to every scratch tree. Prove an excluded real input is caught and a non-input backup is not compiled |
| AC-38 / P-38 | H | Retired carrier/QEMU tests retain an explicit old-assertion-to-current-proof map and the governing retirement approval. QEMU asserted 17 names/count/order; terminal actions asserted metadata/prose. Successful dispatch is additional required proof, not a lost assertion. H reports a separate publication symlink escape at verifyAsset; parent verification and containment proof remain owed |
| AC-39 / P-39 | F | Explicit `kernel` selects the compatible Ze guest even for a host-buildable Go unit; without it, preserve current placement. Reuse the main-mounted guest. The same exact unit must report clean PASS, broken FAIL and restored PASS, including paired solo observations; skip, no execution and compile failure cannot become discrimination evidence |
| AC-40 / P-40 | B | Both `.et` and `.wb` adapters carry declared capabilities and exclusive groups from file to scheduler. An unavailable capability causes placement on a suitable Ze guest or an explicit refusal/skip naming the capability, never execution as an ordinary successful test. Same-group cases serialize through cleanup across participating invocations; different groups remain independent. A parse refusal must not silently discard a declaration. Prove each adapter separately |
| AC-41 / P-41 | A, F | `rfc/full/rfc7084.txt` is readable from the committed tree used for the RFC7084 summary, not only from an untracked local copy. The quote consumer can read that committed text and no RFC7084 row reports `no-rfc-text`. A local-only file must fail the committed-source check; no export build or test is needed |

### Concrete proof carriers and unresolved census

**Existing** means a carrier is present or explicitly identified in the historical
record; it does not mean its assertions or discrimination are verified here.
**Proposed** names a test to add or extend, not a passing test. A proposed package
test must drive the stated real entry, using owned fixtures, rather than replace
that entry with a mock verdict. Producer facts not in the S audit remain R leads;
the plans below specify what must be demonstrated, not which unverified repair to
apply. Breaks are future scoped discrimination plans, never permission to mutate
a shared guard, run destructive Git commands, or build/test another source tree.
An RFC-tagged carrier uses the native recorder and independent judgment.

#### Publication, placement, adapters and cancellation

Fresh source reads for these corrections: `markdown.go` `latestComparisonResults`
already keys by DUT/family/force-MP/pass, but `sortByConvergence` sorts raw times and
`Markdown` prints them without a loss column. `html.go` `writeHTMLTable` and
`writeSVGChart` assign best styling from those times. `internal/perf/cli/cmd_report.go`
`cmdReport` reads all JSON files and calls Markdown, HTML or PerformanceDoc without
a loss check. These are source observations, not new runtime reds. Preserve the
existing pass repair while adding the independent pre-selection loss refusal.

`unitNeedsGuest` in `internal/le/rfc/discriminate_guest.go` returns host placement
as soon as `MatchFile` succeeds; `requireGuestKernel` then refuses a kernel for
that host placement. `cmd_editor.go` `runEditorTests` ignores the callback context;
`cmd_web.go` schedules a `BaseTest` and calls `RunWBFileWithSession` without that
context. These observations justify checking the complete adapter chain, not
assuming the shared scheduler already receives the declarations.

| Proof | Carrier / status | Real entry and stimulus | Required assertion | Discriminating break |
|-------|------------------|-------------------------|--------------------|----------------------|
| P-20 loss | Proposed `TestComparisonRefusesLossBeforeSelection`, `internal/perf/report/comparison_loss_test.go`; `TestCompareCommandRefusesLoss`, `internal/perf/cli/cmd_report_loss_test.go`; Trend/track entry coverage | Direct Markdown/HTML/PerformanceDoc/Trend and compare default/md/html/doc; track md/html with an older lossy input outside --last. Use 100000 sent and 99999, 0, or 100000 received; older loss/newer clean with the same key in both orders; missing versus explicit-zero convergence | Invalid raw input refuses before any report byte, naming source/identity and loss. No table, SVG, best style, delta or rank for invalid convergence. A stdout I/O error propagates; it cannot roll back bytes already written | Remove validation or move it after dedup/--last; equate missing convergence with explicit zero |
| P-20 identity | Existing `TestComparisonRendersBothPassesOfOneDUT`, `TestComparisonUnrecordedPassIsNamed`, `TestComparisonOutputsKeepLatestResultPerDUT` in `internal/perf/report/report_test.go`; extend assertions if needed | Feed distinct live-forwarding and RIB-propagation passes of Ze plus an unrecorded pass and older/newer samples of one pass to each renderer | All distinct passes are named; latest selection applies only within one pass. This is separate from loss validity | Drop Pass from `comparisonKey`; falsely reject every repeated DUT name |
| P-20 raw/publication boundary | `internal/perf/result.go` ReadNDJSON/WriteNDJSON, compare JSON decoding and track input; reached file-copy publication in `internal/le/site/build.go` | Raw JSON/NDJSON round-trips loss as evidence. Input decoding preserves convergence presence before converting to report values. Invalid comparison cannot replace a previously published valid file; each file replacement completes or leaves its old bytes | Raw diagnostics are not a JSON comparison format. No site convergence parser was found by the targeted convergence-ms/routes-lost source search; do not invent one. Apply report validation before any reached report publication and verified-byte handling to copied artifacts | Default absent convergence to zero; replace destination before validation; claim a multi-artifact transaction or stdout rollback |
| P-39 placement | Proposed `TestRecordLinuxCapabilityUnitInGuest`, `internal/le/rfc/discriminate_guest_runtime_test.go`; existing units `TestGTSMTransmittedICMPErrorCarriesTTL255` and `TestGTSMTransmittedICMPv6ErrorCarriesHopLimit255`, `internal/component/gtsm/rfc5082_gtsm_linux_test.go` | Actual `le rfc record` on Linux without effective CAP_NET_ADMIN, explicitly selecting Ze's native-architecture runtime kernel; no-kernel control retains current placement | Both exact units execute with zero skips in clean/broken/restored and paired solo observations; recorder retains attributed red/green, kernel and unit identity. Host buildability cannot block explicit compatible guest selection | Restore host-first refusal; remove hop-limit route metric through recorder's scoped break. Skip, no execution or compile failure is not an attributed red |
| P-40 editor capabilities | Proposed `TestEditorAdapterCapabilityPlacement`, `internal/test/cli/cmd_editor_contract_test.go`, with proposed `test/editor/adapter-capability.et` | `le test editor` parses the capability-bearing file on a capable and an incapable execution target | Declaration reaches placement; capable case executes its positive editor assertion; incapable case moves to a suitable guest or explicitly refuses/skips without successful execution | Discard the parsed capability before scheduling |
| P-40 web capabilities | Proposed `TestWebAdapterCapabilityPlacement`, `internal/test/cli/cmd_web_contract_test.go`, with proposed `test/web/adapter-capability.wb` | `le test web` parses the capability-bearing file before daemon/browser startup on both targets | Same placement/refusal contract as editor, and no daemon/browser side effect before an incapable-target refusal | Start web execution before capability evaluation |
| P-40 editor exclusion | Proposed `TestEditorAdapterExclusiveLifetime`, `internal/test/cli/cmd_editor_contract_test.go`, with proposed `test/editor/adapter-exclusive.et` | Two separate editor invocations declare the same group; first holds a state rendezvous through cleanup, second waits; repeat with distinct groups and cancellation | No overlap through cleanup; distinct groups proceed; cancel releases only the owned lease. Use shared runner lease mechanism, not a new adapter lock | Drop group metadata or release before editor cleanup |
| P-40 web exclusion | Proposed `TestWebAdapterExclusiveLifetime`, `internal/test/cli/cmd_web_contract_test.go`, with proposed `test/web/adapter-exclusive.wb` | Two web invocations as above, holding owned server/session cleanup at the rendezvous | Same-group execution and cleanup serialize; independent groups proceed. Close only the owned session, join owned commands/server and report incomplete cleanup on cancellation; no unconditional detached-descendant claim | Release group before owned cleanup; swallow close error; inherited session replaces explicit owned session |
| P-41 committed text | Existing `TestQuoteBackfillRealCorpusHasNothingLeft`, `internal/le/rfc`; proposed `TestRFC7084TextIsCommitted`, `internal/le/rfc/quote_committed_source_test.go` | Read `rfc/full/rfc7084.txt` as a committed Git blob, pass that content to the quote consumer for the RFC7084 summary. Negative fixture supplies a local-only text but no committed blob | Committed text is readable and every RFC7084 quote has its source; local-only input fails rather than masking `no-rfc-text`. No source export, compile or broad corpus run required | Replace committed-object read with working-tree read; omit the committed source blob |
| P-7 in-flight cancellation | Proposed `TestDispatchCancellationReapsInflightStage`, `internal/le/verifydispatch/cancel_test.go`; existing `TestCanceledCommandKillsItsGrandchildren` and `TestCanceledCommandDoesNotWaitOnAnEscapedPipeHolder`, `internal/le/interoplab/process_cancel_test.go` | Scoped dispatcher fixture calls the real dispatch boundary with a root context; stage announces entry and launches an owned TERM-ignoring process group, then root is canceled. Repeat before stage start and during slot wait | In-flight context is canceled; no next stage starts; owned group is killed, direct child waited and descendants observed gone before lease cleanup. Foreign sentinel survives. No `le verify` invocation | Replace inherited root with Background; restore child-only kill; omit Wait before release |

P-7's finite bound comes from existing machinery, not a new timeout policy:
`plugin.KillGroupOnCancel`/`KillProcessGroup` sends SIGKILL to the owned group, so
TERM resistance cannot delay it. `systemProcessRunner.Run` in
`internal/le/interoplab/process.go` sets `WaitDelay` to `processWaitDelay` (two
seconds), bounding inherited-pipe waits. Its existing escaped-pipe test permits
that bound plus three seconds of scheduling margin; use the same five-second
fixture ceiling after the cancellation rendezvous, and its existing two-second
`pidGone` observation for orphan reaping. This is the proposed dispatch contract,
not a claim that dispatch already uses it. Assert Wait completion and group death
as well as elapsed time. WaitDelay alone neither kills nor reaps an escaped
descendant: the fixture owns and cleans up its escaped-pipe sentinel separately.
Installer terminal/sudo lifetime remains a separate P-18 contract, because a
terminal-sharing child cannot blindly be put in a background process group.

#### Other independently known cases

Names in the next table are **proposed** unless explicitly marked existing. A
multi-case row requires one independently discriminating subtest per named case;
one passing branch cannot discharge its siblings. `one_off_proof_test.go` paths
are proposed carrier locations in the named owning package, not existing files.

| Proof / independent case | Carrier / real entry | Stimulus → assertion | Discriminating break |
|--------------------------|----------------------|----------------------|----------------------|
| P-1 suite verb | `internal/test/cli/one_off_proof_test.go` `TestUnknownSuiteVerbRefused`; real suite CLI | Unknown verb and help control → named failure versus successful help | Default unknown verb to success |
| P-1 engine steps | `internal/test/runner/one_off_proof_test.go` `TestEngineStepUnknownKindRefused`; file, stdin and direct execution entries | Unknown JSON kind in each route → failure naming kind/input; valid step executes | Remove decoder default, then execution default separately |
| P-1 peer blocks | Same file `TestUnusedPeerBlockRefused`; actual ordinary and orchestrated `.ci` runner | Positive orphan and negative orphan blocks → refusal; consumed legacy peer block → genuine wire assertion | Skip unconsumed-block validation; bypass legacy peer execution |
| P-1 directive predecessor | Existing `TestCIMustFailFixturesAllFail`, `internal/test/runner/must_fail_test.go`; actual parser/run | Unknown directive → exact expected parse refusal, valid fixture reaches child | Ignore unknown directive or pass wrong failure reason |
| P-2 dimensions/waits/ticks | `internal/component/cli/testing/one_off_proof_test.go` `TestETOptionsReachExecution`; `RunETFile` over authored `.et` | Nondefault width and height change observed model/screen; explicit wait observes the awaited state; a command's real tick work completes before assertion | Drop width, height or wait independently; replace tick execution with no-op |
| P-2 lifetime/budget | Same file `TestETTimeoutJoinsWorker`; suite adapter plus registered owned per-case child entry, proposed from B's audit | Hung file I/O/lock operation and malformed/zero/negative budget → diagnostics and named failure; kill/reap and parent cleanup precede result and lease release; ordinary completion remains green. Cooperative context and stoppable timers alone cannot bound the reported blocking calls | Return a worker result before cleanup; omit child reap; accept a nonpositive budget. Process isolation remains a design candidate awaiting approval |
| P-2 browser startup | `internal/component/web/testing/one_off_proof_test.go` `TestFirstOpenHasOneBoundedContract`; actual first-open path | B candidate: apply existing concurrency headroom once, independent of verify mode; establish inherited deadline before server/browser startup. Readiness, viewport, locale, open, actions and waits consume remaining budget; waitLoad timeout fails. Separate bounded cleanup precedes lease release | Reset a phase budget; double headroom; ignore waitLoad failure; release before cleanup. Proposed proof covers delayed and never-ready startup in both environments |
| P-3 helper lifetime | `internal/test/runner/one_off_proof_test.go` `TestBackgroundReapedBeforeLeaseRelease`; `runTest` and `runOrchestrated` | Foreground timeout, early error and normal exit while helper holds port → Wait/group death precedes lease handoff | Kill without Wait or release first, independently in each route |
| P-3 linger | Same file `TestPeerLingerSurvivesStdinRouting`; actual file/pipe peer routes | Peer finishes assertions while foreground still uses session → connection persists until runner cleanup | Infer original script identity from rewritten stdin alone |
| P-3 cleanup reporting | `internal/le/interoplab/one_off_proof_test.go` `TestPassedAssertionReportsCleanupFailure`; `Suite.Run` | Passing checker then owned teardown failure → pass assertion plus cleanup error in text, structured output and overall unsuccessful run | Overwrite pass with assertion failure, or suppress cleanup error |
| P-4 launcher/build identity | Proposed `test/ui/launcher-argv-identity.ci`, invoking launcher and wrong-build refusal | Differently named valid executable, stale ambient build name and spaced argv → exact argument boundaries and actual intended binary | Reconstruct argv from joined text; inherit stale build-name guard |
| P-4 watchdog | Proposed `test/ui/conf-watchdog-executable-identity.ci`; watchdog helper entry | PATH contains wrong helper and intended binary has another name → intended helper runs with exact argv | Use ambient PATH instead of selected executable |
| P-4,21 stress entry | `internal/le/test/stressrepro/one_off_proof_test.go` `TestStressInvokesRegisteredSuiteFresh`; real stress action/child launch | Reload suite, selectors with spaces, two invocations → registered argv, distinct child starts, no stale verdict attachment | Hardcode old namespace; reuse previous invocation result |
| P-5 removal | `internal/le/commit/one_off_proof_test.go` `TestGeneratedRemovalPreservesChangedCopies`; generated ordinary and two-block closure scripts over owned fixture repositories | Unchanged file, changed bytes, mode-only edit, directory, publication failure → only unchanged successful-publication copy removed | Delete before publication; omit byte/mode comparison; remove directory recursively |
| P-5 message allocation | Same file `TestPrepareAppendMessageIsolation`; real create/append/replace entry | Same-tag prepares and appends → unique message files, earlier bytes retained, no duplicate obligation | Share same-tag filename; truncate earlier message; allocate without exclusivity |
| P-6 successor obligations | Same file `TestObligationsFollowSuccessfulPublication`; preparation and current Git-derived discharge | Invalid subject, invalid cover, failed publication, retry, foreign erroneous predecessor record → no premature debt/spent approval, legitimate discharge reachable | Record before validation; consume approval on failed publication; consult retired shard |
| P-7 reclaim | Proposed `test/ui/reclaim-owned-artifacts.ci`; standalone reclaim entry, not verify | Interrupted removal, directoryless owned marker, foreign marker and live owner → reclaim only authorized dead resources, no suite launched | Require directory before marker cleanup; treat dirty interrupted-removal state as foreign; ignore ownership |
| P-8 Stop/panic | `internal/le/hookruntime/one_off_proof_test.go` `TestHookVerdictBoundaries`; actual hook dispatcher | Each historically lost Stop category and mixed allowed/disallowed panic calls → current rule-specific verdict, exemption applies to one call | Whole-edit panic exemption; category default erases a required verdict |
| P-8 design/daemon evidence | Same file `TestDesignEvidenceNamesSubjectAndFreshness`, `TestDaemonPathDemandsFunctionalProof`; write hook entry | Wrong-subject/stale read marker and daemon-path edit with helper-only proof → refusal; matching fresh evidence/control accepted | Accept any source-read marker; omit daemon-path demand |
| P-8 four lost hooks | Same file `TestHistoricalHookIntentHasCurrentConsumer`; hook dispatcher | Commit deferral, test-first, delegation reminder, subagent context, each with violating/control fixture → current-rule disposition and nonvacuous verdict | Remove its current consumer. For retired prose-sensitive policy, assert explicit permitted behavior rather than reintroduce it; consumer decisions still need current-rule census |
| P-8 startup/fixtures | Same file `TestSessionStartWithinHarnessBudget`; existing `TestSessionStartRendersNoDeferredArtifact` and rendered-rule fixture | Actual session-start with absent derived artifact and current source digest/needle → required context within existing five-second harness budget, no derived-artifact work | Add blocking derivation; stale digest; require retired update-index text |
| P-9 levels/metadata | `internal/le/rfc/one_off_proof_test.go` `TestGapLevelAccounting`; real check/report stage; separate rule metadata fixture in `internal/le/hookruntime/one_off_proof_test.go` | MUST/SHOULD gaps including RFC9256/RFC9552; mismatched metadata/body → separate counts and named metadata mismatch | Count SHOULD as MUST; read metadata alone |
| P-9 terminal boundary | Existing rfc approve owner-ruling branch, commit create attest, commit push preparation and spec review record owner-authorised; proposed entry tests beside approve_test.go, commit/push_test.go and spec review tests, using human.IsTerminal | Both-terminal, piped-stdin, piped-stdout and detector-error cases; refusal leaves approval rows/messages/scripts/review artifacts unchanged and creates no directories. Review authorization below and above the cap requires the gate whenever supplied. Ordinary RFC/weakening approvals and reviews without owner authorization remain agent-callable; recorded consumers and prepared push scripts execute noninteractively | Remove gate, apply it only above the review cap, move it after a write, or gate ordinary reviews/judges/script execution. PTY acceptance is a detector limit, not identity proof; no new owner record or rfc authorise command |
| P-10 cache/KVM/mount | `internal/le/test/qemu/one_off_proof_test.go` `TestGuestBootstrapContracts`; actual QEMU command/bootstrap entries | Supported cache symlink layout, native arm64 Linux with KVM, required mount failure → usable cache or attributed xattr cause, KVM selected, no download/build after failed mount | Use hvf on Linux; leave ungrouped shell OR; change mount layout without testing symlink |
| P-10 cwd/probe | `internal/le/test/deployment/one_off_proof_test.go` `TestRelativeGokInputsSurviveCwd`; kernel probe action fixture | Relative appliance injection across `runGokInProcess` → correct file; failing probe → unknown with cause, not unsupported feature | Resolve relative path after cwd change; map probe error to absent capability |
| P-11 hook/QEMU fixtures | Existing hook allow probes and `test/ui/le-qemu-run-answers.ci`; proposed `TestFixtureRootsModelReadableSources` in `internal/test/fixture/one_off_proof_test.go` | Committed-original probes; QEMU fixture with declared Go version and poisoned outside root → correct acceptance/version from fixture root | Omit committed HEAD; omit go directive; read host module instead |
| P-11 wait-file | Existing `test/plugin/path-asn-filter-export-reject.ci` and `test/plugin/redistribute-export-modify.ci`; real peer/driver entries | Per-run release marker → expected route rejection/modification only after release | Producer/consumer disagree on filename; accept stale prior-run marker |
| P-11,33 BFD persistence | Existing `storage-bfd-restart` carrier; location from semantic census, not assumed from name | Actual seed/start/restart workflow → state read from isolated disk survives restart | Substitute memory-only store; restart with another root |
| P-12 IPsec log | `internal/le/interoplab/ipsec/one_off_proof_test.go` `TestLogKeyReachesPeerCheck`; actual scenario runner | Enable the logger using its registered key, trigger intended event → checker observes it | Set unused ZE_LOG_LEVEL spelling; accept unrelated log |
| P-12 build staging | Proposed `TestBuildOnlyStagesInputs` in the package owning `Bench.Run` (exact carrier path unresolved) | Build-only and measured entries with changed/missing inputs → same validated stage before image build; stale inputs refused | Place staging only in measured branch |
| P-13 pipe/union | `internal/test/runner/one_off_proof_test.go` `TestCaptureBeyondPipeCapacity`; existing `pkg/plugin/sdk/union_test.go` `TestUnionTimeout` | Actual capture writes beyond pipe capacity → all bytes and completion; private constructor supplies interval and maxPending before NewUnion starts sweepLoop, preserving public constructor defaults | Wait before drain; mutate interval or capacity after worker start |
| P-13 chaos/poster | Proposed `test/ui/chaos-capture-order.ci` and `test/ui/web-config-poster-stability.ci`; actual capture/render entries | H reports clock consumers in layout/detail/events/stream/timeline/sync/broadcast. Fixed clock plus tied LastActive peers and insertion-order permutations → stable peer-index eviction tie-break and byte-stable capture; same poster input twice is stable | Read wall clock in each reached consumer; omit eviction tie-break; introduce nondeterministic poster input |
| P-14 toolchain inputs | Proposed `test/ui/vendor-preserves-owned-patches.ci`, `test/ui/schema-test-registration.ci`; existing l2tp unused population remains census work | Supported vendor entry with owned patch → patch survives or operation refuses before replacement; test binary resolves each needed schema → all modules linked | Run raw vendor replacement first; drop generated schema registration import |
| P-15 state/selection/guidance | `internal/le/hookruntime/one_off_proof_test.go` `TestDirectSpecEditChecksWIP`; proposed `test/ui/functional-selection-contract.ci` | Direct edit/unclaimed status transition crossing WIP limit → same refusal as claim; documented selector → exact selected case; mode readers agree and guidance contains no agent verify route | Enforce WIP only at claim; ignore selector; interpret mode differently in a reader |
| P-16 positive criteria | Proposed `TestCriteriaNamePositiveBehavior` in `internal/le/spec/one_off_proof_test.go`; six historical spec repairs require exact-file census | Spec check on absence-only versus positive observable with permitted survivors → reject vacuous criterion, accept meaningful one | Treat deletion/absence alone as completion |
| P-16,33 seed isolation | `internal/test/fixture/one_off_proof_test.go` `TestSeederEntryCannotReachOperatorStore`; actual seeder entry with outer guard bypassed in owned fixture process | Session-owned binary, foreign binary, symlink escape, dotted/uppercase root aliases and operator sentinel → only owned isolated store changes; seed/start use identical effective root | Validate lexical shape only; resolve alias after ownership check; follow escaping symlink |
| P-16 delegated rename | `internal/le/hookruntime/one_off_proof_test.go` `TestDelegatedRenameUsesCommitRoute`; hook probe only | Delegated direct git rename versus generated commit route → direct staging refused, supported route allowed | Exempt delegated session from Git safety. Never execute destructive command for the proof |
| P-17 tmpfs | `internal/test/runner/one_off_proof_test.go` `TestTmpfsContentAndModeReachBothRoutes`; parse and ordinary `.ci` execution | Executable tmpfs script plus non-executable control → exact contents/modes retained, executable runs | Open-code content-only copy in ordinary runner |
| P-17 theme | `internal/component/cli/one_off_proof_test.go` `TestActivityStyleUsesTheme`; actual activity rendering | Light/dark themes and active/inactive states → theme-derived visible styling | Reintroduce light-only literal |
| P-17 Git paths | `internal/le/commit/one_off_proof_test.go` `TestQuotedPathsReachGeneratedScript`; snapshot/selection/script entry | Spaces, quotes, backslashes and non-ASCII → exact intended raw path selected/published, neighbors unchanged; NUL/tab/CR/newline → explicit refusal. Include earlier block/history/hunks/shared-index readers | Split raw entries as whitespace; leave Git quoting in path; omit refusal or decode differently between consumers |
| P-17 GoBGP | Proposed `test/interop/scenarios/bgp-export-reload-gobgp/` extension or successor carrier; existing scenario identity must be reconciled before writing | Actual SIGHUP/UpdateConfig, ordinary and secondary recipients, every transition in supplemental table → current OUT state with stable generation, no source UPDATE/refresh/reconnect | IN-only reload; omit disappearing-secondary withdrawal; emit withdrawal only in discarded `filtered`. Peer producer verification remains pending |
| P-17 CodeQL | Proposed `TestGoExtractionExcludesCache` in `internal/le/repo/codeql/selection_test.go`, plus future CI extraction proof | Derive package directories from tracked Go and each complete build context; explicitly classify cache/vendor/testdata/tool inputs. Refuse unclassified inputs, including future non-cache gokrazy. Before each traced build, canonical dependency directories must exclude gokrazy/modcache. Preserve bootstrap coverage and featuretags generator markers/suffixes; no permanent nine-root list | CI must show authored sentinel extracted/cache sentinel absent; explicit cache build fails exclusion proof and dependency into cache is refused. Symlink aliases cannot bypass the canonical-path check; no extraction proof ran here |
| P-18 kernel containers | Proposed `test/ui/kernel-container-cancel.ci`; owned kernel-build entry | Cancel during build → owned container gone before cleanup, foreign sentinel untouched | Kill Docker client only |
| P-18 installer | `internal/le/setup/docker/kernel/one_off_proof_test.go` `TestInstallCancellationReportsCompletedSteps`; installer entry and subprocess signal fixture | Stop before first step, after changed step and between module removal/copy; report changed/unfinished work and do-not-reboot guidance. First stop relays TERM and restores default signal handling; second interrupt ends le even when the step ignores TERM. No later step; terminal remains attached | Keep NotifyContext interception after first stop; claim killed sudo means privileged command finished; kill shared tty group; report nothing changed after a completed modifying step. This selects N6's second-interrupt alternative, not unconditional bounded privileged-tree reaping |
| P-18 six routes | Proposed `TestVerbRootReachesEveryOwnedPhase` in each reached deployment/QEMU/yangcontract package; exact package census still owed | Each of l2tp-ppp, gokrazy-l2tp, l2tp-scale, hugepages, docker-lab, yangcontract: cancel in build, admission wait, startup, running → bounded owned teardown, no success verdict | Fresh Background context at each reached phase; cancellation only between phases |
| P-19 browser | Proposed `test/web/recorded-config-save.wb`; shared authored browser journey with optional capture | H reports Basic HTTP validation versus recorder login/save/diff/navigation. Both must drive the same journey → saved config read back after authenticated navigation, with diff and capture settling proven | Validator bypasses login, asserts unsaved DOM only, or captures before settled state; current settling remains unproven |
| P-19 generators/workflows/discovery | `internal/test/fixture/one_off_proof_test.go` `TestToolFixturesReachTheirProducer`; existing `le-discovery-answers` | Actual plugin-import/YANG-glue generators produce expected file; advisory job policy keeps intentional evidence-nightly behavior; discovery uses producer's committed range despite dirty/unpushed differences | Execute non-main generator file; require every job advisory; compare working tree with committed-range producer |
| P-20 empty derivation | Proposed `TestEmptySuccessfulDerivationRefused` in each owning toolchain/stdlib package; exact carrier paths unresolved | Tag derivation returns empty and stdlib subprocess exits zero with empty listing → error with cause, not default empty set; nonempty control works | Accept empty success |
| P-21 filename bound | `internal/le/test/stressrepro/one_off_proof_test.go` `TestRunSlugBoundAndIdentity`; real capture file creation | 91 selectors, same-prefix long lists and sanitization collisions → component fits target filesystem, distinct identities and exact original argv in evidence | Truncate without digest; use entire list as component |
| P-22 session heading | Proposed `test/ui/session-state-attribution.ci`; real session renderer | Own and foreign edits, then unavailable attribution → accurately attributed paths or explicit shared-checkout heading | Label whole-tree diff as this session |
| P-23 not-enrolled | `internal/le/rfc/one_off_proof_test.go` `TestNotEnrolledWriterReaderAgreement`; writer → reader → publication | Every emitted kind including source-restricted ISO10589 → accepted/migrated without false conformance count | Reader omits writer's kind; include ISO10589 in counted set |
| P-24 speaker census | `internal/le/spec/one_off_proof_test.go` `TestWireCensusIncludesNonGoSpeakers`; actual spec/design judgment | Fixture with Go writer and shell/Python reader, omit each in turn → missing population refused; edit list alone insufficient | Derive population only from edited Go paths |
| P-25 eight documents | Proposed `TestOwnedDocumentDisposition` in `internal/le/rfc/one_off_proof_test.go`; current profiles, corrections, quote/extraction consumers | Separate ASPA-profile, 9117, 4506, 5080, 7383, 8247, 7519, 1008 cases → no unlisted enrollment, retired obligations accounted, optional debt retained, method14 SHOULD judged from source | Use citation reachability as enrollment; drop correction/optional row. Exact requirement-to-carrier census remains unresolved |
| P-26 named fixtures | Existing `test/ui/le-docvalid-answers.ci`, `test/ui/le-evidence-answers.ci`, `test/ui/le-job-admits.ci`, `test/ui/le-placement-answers.ci`; their real tool actions | Valid and violating doc/evidence/ownership fixtures → discriminating verdict; job case uses entry/attach/release rendezvous rather than three-second overlap | Accept planted invalid fixture; remove admission wait or replace rendezvous with blind sleep |
| P-26 lint populations | Existing originating logs named below; per-member carrier **unresolved** | Recover each diagnostic and run only its reached lint/test consumer → exact member resolved without baseline waiver | Reintroduce that diagnostic at its producer, not an unrelated lint failure |
| P-27 corruption | Existing named damaged archives/traces, currently unrecovered; future boundary capture is not itself repair proof | Compare retained build output, installed inode and failing executable → identify first boundary where content identity diverges and accountable writer | Reproduce the identified fault after attribution. No fabricated mutant or generic integrity check can replace missing cause |
| P-28 bulk peers | `internal/test/peer/one_off_proof_test.go` `TestGeneratedBulkAttributesFollowOPEN`; real active/passive ordinary/connmap handshake paths | IPv4/IPv6 × iBGP/eBGP × both connection modes/routes → capture LOCAL_PREF present only on ordinary iBGP; raw hex unchanged | Derive ASN/session kind from config default rather than actual OPEN; strip/add LOCAL_PREF universally |
| P-29 review selection | `internal/le/commit/one_off_proof_test.go` `TestReviewPinsProspectiveContent`; real record/prepare/append/generated re-check | One snapshot before checks; private index replays recorded HEAD, earlier blocks and current snapshots/hunks/removals. Recorder accepts blob/remove sources. Every CAS attempt checks explicit origin artifact against actual proposed tree, including historical paths, modes and deletions; unrelated live edits neither stale nor enter review | Hash live file; take a second snapshot; choose ambient artifact; omit retry check; use recorded HEAD alone instead of replayed blocks |
| P-30 late approval | Same file `TestLateApprovalDischargesOnlyBoundChange`; approved writer route → owed-range consumer | Optional full-SHA target in rfc approve; explicit repeatable create rfc-approval selection publishes paired approval/binding trailers, including empty approval-only commits. Only exact target/unit with same-carrier binding and valid earlier target in publication range discharges; unbound approval answers only its own commit | Unit-name-only matching; binding in another carrier; wrong order/range; prune on prepare/failure; reuse pre-rebase SHA for rewritten commit |
| P-31 canonical unit | Same file `TestApprovalWriterUsesConsumerIdentity`; actual approval command → edit and commit readers | Package-clause alias, directory-based key, ambiguous/unknown unit → refuse unusable alias/ambiguity before write; valid canonical row consumed by both | Write success using package-clause identity consumer cannot resolve |
| P-32 inventory | Proposed `internal/le/repo/inventory` reader and `internal/le/site/one_off_proof_test.go` `TestInventoryUsesCompleteCommittedPopulation`; site and coupling entry | H candidate below: one resolved committed snapshot. Proposed fixtures cover Linux registration on simulated darwin, every dependency, imported RunEngine, YANG outside registration package, ignored dirty edits and HEAD movement; missing/ambiguous/unsupported inputs refuse. Compare live census without freezing count | Use host-linked registry; read live coupling source/gates; resolve HEAD twice; omit unknown expression or dependency. Reader and proofs are unimplemented |
| P-33 persistence census | Existing named leads below; proposed `TestPersistenceFixturesUseOneEffectiveStore`, `internal/test/fixture/one_off_proof_test.go` | Each disk-reader/restart case and memory-only control → isolated persistent store where required, memory isolation otherwise, seed/start roots agree | Force all fixtures to memory or all to shared disk; strip only uppercase env alias |
| P-34 contracts/manifest | Existing `TestActionsCarryTheirContracts`, `TestHomeHeroDerivesFromTheManifest`; proposed `TestRenamedCallersReachRegisteredActions`, `internal/le/one_off_proof_test.go` | Add a registered member, exercise deployment/QEMU/readme-carrier and renamed doc-index/status callers → membership/contract checks adapt, every caller resolves | Freeze total count; retain stale action spelling |
| P-34 lab/health/rules/docs | Existing `TestPartialHealthExplainsScopedEvidence`, `le-rules-answers`, `le-vendor-web-answers`, `le-checks-answers`; proposed `TestLabPinReportDoesNotWrite`, `internal/le/lab/pin/one_off_proof_test.go` | Report group must not bump; partial-health fixture has real meta rows; dotted root poison isolated; feature Doc evidence matches content | Put writing bump in report group; invalid metadata fixture; leave dotted alias; reuse stale Doc hash. Historical render-update cause stays unproven |
| P-35 load | Existing named cases and missing log members listed below; per-member contention carrier **unresolved** | Reach each actual wait/race/resource bound under owned contention and explicit readiness → assertion passes because cause repaired | Reintroduce the identified wait/race; blanket timeout inflation cannot substitute for a break |
| P-36 admission | `internal/le/job/one_off_proof_test.go` `TestSilentForeignJobSurvives`; QEMU admission entry companion | Foreign silent process, canceled own waiter, guest carrying parent identity and unreadable identity → foreign survives, own wait ends, guest inherits identity without whole-tree 9p hash, unreadable refuses | Kill by silence; recompute InputHash over guest mount; treat identity error as permission |
| P-37 lint input | Proposed `TestLintInputGuardMatchesProducer` in owning lint package (path unresolved) | Real selected scratch Go input versus preserved non-input backup → real input checked, backup not compiled/deleted | Glob all scratch; blanket-exclude actual input |
| P-38 retirements | Existing retired carrier/QEMU assertion history; replacement proof map still owed | Retain retirement approvals: QEMU checked 17 names/count/order, terminal actions metadata/prose. Map those assertions to current proofs; successful dispatch is additional. Rooted artifact acquisition must feed retained verified bytes/staging to all consumers, including publication copy, as the final contract specifies | Escaping links, path swaps and in-place mutation cannot publish unverified bytes. Retirement alias tests cover a different boundary; a retained handle alone does not freeze its contents. Parent approval and runtime proof pending |

P-12 retains the four historical fixed-row claims separately: RSVP-TE `buildResv`
needs a real RESV capture whose FILTER_SPEC class is 10 (break: restore
SENDER_TEMPLATE); LDP `handleInit` needs a session proof that stays non-operational
until acceptance (break: advance before acceptance); LDP `DecodeInit` needs unknown
mandatory-TLV refusal and optional-TLV acceptance (break: ignore the U-bit rule).
Their exact current test carriers remain unresolved, rather than inferred from
the repair commit. The fourth claim, unknown `.ci` directives, has P-1 above.

#### Five RTR fixtures

All five carriers below are **existing** under `test/plugin/` and enter through
the functional plugin suite, its `.ci` runner, `le mock rtr --sync-release`, and
the named observer. Their contents were read for this correction. The shared
discriminating harness break is to anchor the mock at the repository root again:
each case must then fail synchronization, not pass because no route was observed.
After restoring per-test cwd, each owes its independent route assertion too.

| Carrier | Stimulus and expected route assertion | Separate behavior break |
|---------|---------------------------------------|-------------------------|
| `bgp-rr-rpki-pending-withdraw-relay.ci` | Withdraw held 10.0.0.0/24 before sync, then release VRP; RR sends no UPDATE for it and exactly the revalidated 10.0.1.0/24 fence | Remove adj-out ownership guard or retain canceled pending work |
| `bgp-rs-rpki-pending-withdraw-relay.ci` | Same pending-withdraw sequence through RS; assert its own expected wire path and single fence | Break RS cancellation/adj-out guard independently of RR |
| `bgp-rr-rpki-revalidate-relay.ci` | Hold destination down, receive NotFound path, re-enable destination with EOR only, release cache; RR revalidation delivers conn=2 seq=2 with reflection attributes | Disable RR replayValidation delivery |
| `bgp-rs-rpki-revalidate-relay.ci` | Same controlled sync through RS; expected conn=2 seq=2 has its RS path, no extra source announcement | Disable RS replayValidation delivery |
| `rpki-revalidate-late-sync.ci` | Observe accepted NotFound 10.0.1.0/24, release mismatching VRP; retained received bytes become Invalid and ineligible | Drop revalidation update into Adj-RIB-In |

#### Fifteen historical special checkers

Each existing scenario directory below is under `test/interop/scenarios/` and
enters via the integration interop selector, `Suite.Run`, then its registered
checker in `internal/le/interoplab/bgp/check_rfc.go`. This is the incident set,
not a registry count assertion. Every row needs a live attributed red/green;
pinned-peer rendering/schema verification is still pending for each row. The
stimuli and breaks are proof plans, not reports that those breaks were run.

| Existing checker | Existing scenario directory | Stimulus → required assertion | Discriminating break |
|------------------|-----------------------------|-------------------------------|----------------------|
| `checkAddPathReadvertiseCollision` | `bgp-addpath-readvertise-collision-frr` | BIRD replay and GoBGP live injection of the same prefix → FRR negotiates ADD-PATH receive (`addPathReceiveNegotiated`) and holds origins 65003 and 65004 under distinct IDs (`parseAddPathState`). Clear the FRR session, observe a higher established-session generation (`waitFRRNewSession`), then require replay to retain each origin's same ID (`samePathIdentifiers`); FRR and GoBGP sessions survive | Reuse one outbound ID for both origins; separately allocate new IDs on replay. Reject missing receive negotiation or an unobserved reconnect; a withdrawal assertion cannot substitute for replay identity |
| `checkLocalPrefStrip` | `bgp-local-pref-strip-gobgp` | Inject 10.54.0.0/24, 10.54.1.0/24 and 10.54.2.0/24 carrying LOCAL_PREF 200, including the post-EOR live rail → every prefix arrives at FRR and GoBGP, retaining AS 65004/injector next-hop and surviving sessions. Require strict complete wire capture on the actual destination connection with LOCAL_PREF absent. Direct source read: GoBGP v4.11.0 table_manager.go ProcessMessage removes LOCAL_PREF before storage for external non-RouteServerClient peers; this scenario's AS65003/peer65001 meets that condition, so RIB absence cannot certify Ze stripping | Deliberately leak LOCAL_PREF: wire assertion must fail even if GoBGP RIB still omits it. Missing/malformed/incomplete capture, wrong connection, suppressed prefix or replaced source also fails; RIB controls supplement the wire oracle |
| `checkMEDAcrossAS` | `bgp-med-across-as-gobgp` | Relay 10.60.0.0/24, 10.60.1.0/24 and 10.60.2.0/24 from AS 65004 carrying MED 100, and originate 10.60.9.0/24 at Ze with MED 42 → all four arrive at FRR and GoBGP. GoBGP retains each relayed source block but no received MED (`requireGoBGPAttributeAbsent`); Ze's own source block retains MED exactly 42 (`requireGoBGPAttributeValue`). Both sessions survive; this is propagation, not best-path comparison | Separately disable received-MED stripping across neighboring ASes, and strip Ze-originated MED as well. Each break must fail its own attribute assertion; suppressing routes must fail arrival controls |
| `checkMEDRemovalConfiguration` | `bgp-med-remove-configured-gobgp` | FRR AS 65005 originates 10.61.0.0/24 and control 10.61.1.0/24 with MED 100; only the first matches DROP-MED import policy → both reach internal GoBGP with FRR's source block intact. First has no MED, control retains exactly 100 (`requireGoBGPAttributeAbsent`/`requireGoBGPAttributeValue`); FRR and GoBGP sessions survive. Internal destination prevents automatic cross-AS stripping from satisfying the configured-removal assertion | Ignore the configured removal; separately strip MED from the unmatched control. Dropping either route or changing its source must also fail |
| `checkRelayWithdrawalShape` | `bgp-relay-withdraw-shape-frr` | FRR first receives the route with AS_PATH `65001 65004` (`requireFRRASPath`), then loses it after withdrawal. Require FRR's actual withdrawal decode (`frrDecodedWithdrawal`) and no attribute error in that same log (`requireNoAttributeError`), followed by a surviving session; RIB absence alone cannot distinguish a valid withdrawal from treat-as-withdraw | Omit the advertisement's Ze prepend; separately add AS_PATH to the withdraw-only UPDATE or otherwise corrupt withdrawal attributes. Route disappearance alone must not pass the latter break |
| `checkRFC2545NextHops` | `bgp-rfc2545-linklocal-nexthop-frr` | Both routes reach FRR over the same IPv6 session: on-link 2001:db8:5601::/48 has Ze's topology-owned global and link-local next hops and is installed via that link-local (`requireNextHopShape`, `requireRouteInstalledVia`); off-link 2001:db8:5602::/48 has only global 2001:db8:ffff::1 despite the configured link-local leaf. Session survives | Omit the on-link link-local or use the wrong topology address; separately include link-local on the off-link route. Parsing two addresses without installation cannot satisfy the on-link proof |
| `checkRFC7606MixedUpdate` | `bgp-rfc7606-relay-shape-frr` | Preannounce 198.51.100.0/24 and observe FRR installation; explicit release barrier then mixed withdrawal plus 203.0.113.0/24 announcement → first removed, second installed, split-shape proof and stable session | Duplicate NEXT_HOP in split announcement; independently drop withdrawal. Absent-before/absent-after cannot satisfy it |
| `checkRFC7606TypedNLRIDiscard` | `bgp-rfc7606-typed-nlri-discard` | Assigned type-2 plus unassigned EVPN type → strict speaker decode retains exact type-2 only, with full OPEN/Established and complete-capture controls | Forward unassigned type; remove retained type-2; truncate capture. Malformed decoder input must error, not resemble discard |
| `checkRFC7999Blackhole` | `bgp-rfc7999-blackhole-frr` | FRR covered/agreed, FRR uncovered, BIRD covered/unagreed announcements → kernel discard only first, positive forwarding for other two | Bypass prefix authorization or session agreement separately |
| `checkRouteServerASPath` | `bgp-route-server-frr` | FRR originates the route in AS 65002; Ze relays between RS clients → BIRD, the receiving observer, holds that prefix with AS_PATH exactly `65002`, without Ze ASN 65001 (`requireBIRDASPath`). Both FRR and BIRD sessions survive the relay | Prepend Ze ASN on the RS rail; separately suppress far-side delivery or lose either session. FRR's own originating RIB cannot substitute for BIRD's received path |
| `checkSelfNextHopWithheld` | `bgp-self-nexthop-withheld-frr` | Inject 10.11.0.0/24 with FRR's own NEXT_HOP first, then control 10.12.0.0/24 with injector NEXT_HOP 172.30.0.9 → FRR's incoming UPDATE decode contains control from Ze and no withheld prefix in the same captured log (`frrDecodedPrefix`). Control is also installed with the sole expected injector next hop (`parseFRRNextHops`, `requireSoleNextHop`); session survives. RIB absence alone is insufficient: FRR could reject a route Ze transmitted | Forward the self-next-hop path: its incoming decode must fail proof even if FRR rejects it from the RIB. Separately suppress control, change its next hop, or drop the session; each positive control must fail |
| `checkNoExportBoundary` | `bgp-wellknown-noexport-frr` | Inject NO_EXPORT 10.10.0.0/24, then control 10.11.0.0/24 carrying ordinary community 64985:100 → internal BIRD receives the NO_EXPORT route, while both BIRD and external FRR receive control. One FRR table answer contains control and excludes the NO_EXPORT prefix (`requireRouteWithheld`); both sessions survive | Disable external NO_EXPORT enforcement; separately suppress NO_EXPORT everywhere, which must fail BIRD's internal receipt. Blanket relay suppression must fail control delivery to both observers |
| `checkISISDynamicHostname` | `isis-p2p-frr` | Up adjacency and Ze LSP → FRR database names Ze from TLV137 and adjacency remains Up | Omit hostname TLV; accept any database name |
| `checkNoFamilyEndOfRIB` | `no-family-peer-eor-frr` | Keep asymmetric OPENs: Ze no MP, FRR IPv4 MP; route arrives → exact sender/family/VRF/current-session EOR decoded | Remove implicit family; accept stale/other-sender/other-family EOR log. Correct misleading header, not the OPENs |
| `checkNSSADefault` | `ospf-stub-nssa-frr` | Full NSSA adjacency without default-originate config → FRR installs Type-7 default via border router, no Type-5 leak | Gate default on absent config; flood Type-5 into NSSA |

Shared oracle extensions are proposed in
`internal/le/interoplab/bgp/check_oracle_contract_test.go`:
`TestInstalledRouteFieldsBelongToOneObject` drives the actual installed-route
predicate with wrong-prefix/protocol/state/next-hop and cross-object matches;
`TestRouteJSONRequiresPinnedSchema` drives `jsonHasRoute`/`nestedRoute` with empty
paths, wrong prefix, error JSON and unrelated valid JSON;
`TestSpeakerDiscardRequiresCompleteCapture` drives the speaker verdict path with
missing ze-open, incomplete capture and malformed/truncated wire. Break each
field/control independently. These helper tests supplement, never replace, the
fifteen live scenarios. Supplemental findings keep their audit-reported status.

The proof plan remains incomplete specifically for the June technique/fuzz
populations; authenticity of the bounded October reconstruction; six historical
spec repair files; missing raw lint context and still-unnamed load members despite
partial recovery in [the evidence companion](tooling-one-off-evidence.md);
retired-carrier replacement proofs; the full
persistence reader census; reached report/publication proof carriers; build-only and
stdlib/tag derivation carrier paths; six cancellation package
carriers; and the three fixed protocol carriers above. Each remains owned by its
P-number and research row. No generic proposed test name certifies these sets.

### Occurrence ownership map

The key is **class / date / surface and symptom**, not table position. Class names
resolve under `plan/journal/` (the historical `.md` suffix on the tool-help entry
names the same class). Every row inherits the P-number of each listed AC. Multiple
ACs require all those proofs. Package letters are research/implementation owners
within this spec, not transfers to another spec. Every entry remains open for proof;
the S/R distinctions above govern its evidence. Repeated dates are intentionally
retained. The extra symptom text distinguishes journal occurrences and hidden
sub-obligations where the short original surface collapsed them.

| Class | Date | Surface / occurrence discriminator | Owner | AC / proof number |
|-------|------|------------------------------------|-------|-------------------|
| concurrent-session-corruption | 2026-10-30 | site Linux inventory builds live checkout | C | 32 |
| review-pin-hashes-uncommitted-content | 2026-10-30 | historical closure path hashes working tree | A | 29 |
| gate-verdict-depends-on-the-machine | 2026-10-29 | DNS system-name-server seed uses Linux-only writer | H | 34, 35 |
| gate-verdict-depends-on-the-machine | 2026-10-29 | coupling uses host plugin population | H | 32 |
| gate-verdict-depends-on-the-machine | 2026-10-27 | 043c682d24 load reds; each named case retained below | H | 35 |
| lint-contract-not-applied | 2026-10-27 | 043c682d24 aggregate lint population | H | 26 |
| command-name-breaks-the-cli-grammar | 2026-10-27 | readme-carrier plus deployment and QEMU vpn-kernel-test | H | 34, 38 |
| delegation-hands-the-callee-a-different-argv | 2026-10-27 | rollback diagnostic `$*` versus executed argv | C | 4 |
| lint-contract-not-applied | 2026-10-27 | radiussubscriber check_policer parse width/allowlist | H | 26 |
| lint-contract-not-applied | 2026-10-27 | 2f36373081 lint; all named sites | H | 26 |
| command-name-breaks-the-cli-grammar | 2026-10-27 | appliance kernel --jobs declaration | H | 34 |
| hardcoded-count-in-test | 2026-10-27 | TestActionsCarryTheirContracts membership | H | 34, 38 |
| plugin-list-hardcoded | 2026-10-27 | TestHomeHeroDerivesFromTheManifest | H | 34 |
| lint-contract-not-applied | 2026-10-27 | e6474aa200 aggregate lint population | H | 26 |
| lint-contract-not-applied | 2026-10-27 | terminaldemo test Docker ownership/placement | H | 26 |
| gate-verdict-depends-on-the-machine | 2026-10-27 | InputHash gitTimeout in 9p guest | D | 36 |
| gate-excludes-part-of-its-population | 2026-10-27 | unitNeedsGuest: linux-buildable CAP_NET_ADMIN proof | F | 39 |
| bulk-rename-corruption | 2026-10-27 | doc index and verify status write callers | H | 34 |
| bulk-rename-corruption | 2026-10-27 | QEMU names lose -test; action census assertion | H | 34, 38 |
| bulk-rename-corruption | 2026-10-27 | le-vendor-web-answers and le-checks-answers | H | 34 |
| package-red-not-reproduced | 2026-10-27 | harnesstool setup docker namespace panic | H | 34 |
| package-red-not-reproduced | 2026-10-27 | le-rules-answers render-update; env alias and old cause | H | 34 |
| published-value-drifts-from-the-behavior-it-describes | 2026-10-27 | hookSourcesDigest stale | E | 8 |
| published-value-drifts-from-the-behavior-it-describes | 2026-10-27 | wholetree-mentions ledger and current no-verify rule | E | 15 |
| read-fails-on-a-fact-the-caller-does-not-need | 2026-10-27 | TestSessionStartRendersNoDeferredArtifact | E | 8 |
| fixture-encodes-an-impossible-state | 2026-10-27 | TestPartialHealthExplainsScopedEvidence meta rows | H | 34 |
| commit-needs-an-uncommitted-symbol | 2026-10-27 | RFC7084 text must be committed/readable, not local-only | A, F | 41 |
| claim-outlives-the-evidence-it-cites | 2026-10-27 | features/ai-first-design.md Doc review | H | 34 |
| lint-contract-not-applied | 2026-10-27 | 0723d7d26a misspell, stringXbytes, typeDefFirst | H | 26 |
| session-never-establishes-with-the-peer | 2026-10-27 | RPKI relay RR/RS pending-withdraw/revalidate; mock RTR sync | G | 12 |
| artifact-bytes-disagree-with-identity | 2026-10-26 | native corruption: startup and embedded-schema executables | I | 27 |
| test-artifacts-land-in-the-repository-root | 2026-10-26 | lint-input guard and foreign Go backup | C | 37 |
| tool-help-contradicts-owner-rule | 2026-10-25 | verify changed guidance; superseded host-specific restriction | E | 15 |
| acceptance-criterion-written-as-an-absence | 2026-08-21 | show bgp summary survivors; six stale spec criteria | E | 16 |
| blanket-mechanism-hid-missing-cases | 2026-08-10 | Stop hook classifier categories | E | 8 |
| bulk-rename-corruption | 2026-08-10 | l2tp exported-to-unused population | C | 14 |
| closed-spec-survives-on-disk | 2026-09-12 | closure remove leaves spec copy | A | 5 |
| container-cannot-see-migrated-scratch | 2026-10-21 | Run.virtfsArgs cache symlink/xattr | D | 10 |
| delegation-hands-the-callee-a-different-argv | 2026-10-20 | conf-watchdog PATH helper identity | C | 4 |
| delegation-hands-the-callee-a-different-argv | 2026-10-13 | stress-repro reload namespace AND reused verdict | C | 4, 21 |
| delegation-hands-the-callee-a-different-argv | 2026-09-07 | launcher existing binary and POSIX argv fixtures | C | 4 |
| delegation-hands-the-callee-a-different-argv | 2026-09-04 | refuseWrongBuildName shared helper | C | 4 |
| delegation-hands-the-callee-a-different-argv | 2026-09-05 | generated commit review-check ambient session | A | 4, 29 |
| error-answered-with-another-errors-code | 2026-10-10 | DockerKernelRoute unknown versus absent | D | 10 |
| escape-hatch-scoped-wider-than-its-justification | 2026-09-27 | one allowed panic exempts whole edit | E | 8 |
| field-carries-two-meanings | 2026-10-03 | runOrchestrated original script versus routed stdin | B | 3 |
| field-carries-two-meanings | 2026-08-19 | ZE_VERIFY_MODE boolean versus former mode argument | E | 15 |
| fixture-encodes-an-impossible-state | 2026-10-05 | hook probeTree: three allow probes need committed HEAD | E | 11 |
| fixture-encodes-an-impossible-state | 2026-09-05 | le-qemu-run-answers missing go directive | D | 11 |
| fixture-encodes-an-impossible-state | 2026-09-17 | le-qemu-run-answers reads wrong root/version | D | 11 |
| gate-ignores-the-level-it-counts | 2026-09-30 | checkGapCountAgreement SHOULD counted as MUST | F | 9 |
| gate-ignores-the-level-it-counts | 2026-10-06 | RFC9256 and RFC9552 optional gaps | F | 9 |
| gate-owed-a-clearance-no-command-writes | 2026-11-02 | late RFC-approved publication | A | 30 |
| gate-owed-a-clearance-no-command-writes | 2026-09-08 | debt-discharge cannot supersede foreign bad record | A | 6 |
| gate-owed-a-clearance-no-command-writes | 2026-09-07 | debt-clear review/RFC writer plus unknown-gate clearance | A | 6 |
| guard-added-to-one-half-of-a-pair | 2026-10-10 | Bench.Run build-only staging | G | 12 |
| guard-added-to-one-half-of-a-pair | 2026-10-26 | generated bulk UPDATE session attributes | G | 28 |
| guard-added-to-one-half-of-a-pair | 2026-09-07 | runGating signal cleanup, binary set/raw coverage, child groups | D | 7, 18 |
| guard-added-to-one-half-of-a-pair | 2026-09-05 | verify dispatch loses cancellation before stage | D | 7 |
| invariant-enforced-by-an-absent-call-site | 2026-08-18 | WIP cap direct edit and unclaimed transition | E | 15 |
| invariant-enforced-by-an-absent-call-site | 2026-08-19 | editor wall clock plus et/wb caps and exclusive groups | B | 2, 40 |
| lifetime-ends-before-its-last-reader | 2026-10-22 | runTest/runOrchestrated timeout Kill without Wait before lease release | B | 3 |
| lifetime-ends-before-its-last-reader | 2026-10-23 | interop assertion pass followed by image-cleanup error | B | 3 |
| mutating-a-guard-runs-the-unguarded-path | 2026-08-31 | handlePacket forced-red live guard mutation | E | 16 |
| mutating-a-guard-runs-the-unguarded-path | 2026-08-10 | build recipe seed reaches operator etc/ze | E | 16, 33 |
| net-pipe-deadlock | 2026-09-03 | captureStdout beyond pipe capacity | B | 13 |
| non-idempotent-append-duplicates-the-record | 2026-08-19 | debt shard: d0ddef174 repeated create; 2f2eb832b create --replace, two journal occurrences | A | 5, 6 |
| non-idempotent-append-duplicates-the-record | 2026-08-29 | append overwrites earlier message | A | 5 |
| option-set-for-one-caller-changes-another | 2026-10-10 | runGokInProcess changes cwd; relative image injection | D | 10 |
| option-set-for-one-caller-changes-another | 2026-09-28 | wait-file and path-asn-filter-export-reject / redistribute-export-modify | B | 11 |
| option-set-for-one-caller-changes-another | 2026-10-19 | storage-bfd-restart persistent sequence | B | 11, 33 |
| output-not-byte-stable | 2026-08-15 | chaos golden relative clock; include total peer ordering audit | H | 13 |
| output-not-byte-stable | 2026-08-24 | web-config poster render stability | H | 13, 19 |
| owner-decision-attributed-without-the-owner | 2026-10-18 | rfc approve reason AND summary annotation authority | F | 9 |
| pointer-shared-across-the-names-it-indexes | 2026-09-04 | append: random script but shared message | A | 5 |
| pointer-shared-across-the-names-it-indexes | 2026-09-05 | same-tag ordinary create: second and third same-day journal occurrences | A | 5 |
| prohibition-broken-by-the-agent-that-carried-it | 2026-08-31 | delegated .ci rename stages through git mv | E | 16 |
| record-written-before-the-operation-succeeds | 2026-09-08 | recordDebt cover count before valid operation | A | 5, 6 |
| record-written-before-the-operation-succeeds | 2026-08-19 | debt append before subject validation | A | 5, 6 |
| refactor-removes-feature | 2026-09-03 | four gates: commit deferral, test-first, delegation reminder, subagent context | E | 8 |
| refactor-removes-feature | 2026-09-03 | design subject/freshness gate AND daemon-scoped .ci demand | E | 8, 24 |
| refactor-removes-feature | 2026-08-31 | checkNoFamilyEndOfRIB and fourteen sibling checker bodies/oracles | G | 12 |
| removal-leaves-the-file-on-disk | 2026-10-21 | sweeper interrupted-removal dirty-state classification | D | 7 |
| removal-leaves-the-file-on-disk | 2026-09-19 | sweeper directoryless marker AND standalone entry point | D | 7 |
| removal-leaves-the-file-on-disk | 2026-09-12 | renderSharedIndexRepair closure fourth instance | A | 5 |
| removal-leaves-the-file-on-disk | 2026-09-07 | renderSharedIndexRepair closure recurrence | A | 5 |
| removal-leaves-the-file-on-disk | 2026-09-06 | remove only drops index entry | A | 5 |
| replacement-written-before-it-is-validated | 2026-09-08 | go mod vendor reverts owned patches | C | 14 |
| rule-metadata-values-disagree-with-linter | 2026-09-12 | rule corpus MUST metadata versus SHOULD body | E | 9 |
| silent-fall-through | 2026-10-05 | engine-step file/stdin decoding and direct execution | B | 1 |
| silent-fall-through | 2026-09-08 | zeTestParseRunCLI unknown verb succeeds | B | 1 |
| silent-fall-through | 2026-09-07 | QEMU required 9p mount falls through | D | 10 |
| silent-fall-through | 2026-09-04 | YANG domain registration in test binaries | C | 14 |
| silent-fall-through | 2026-08-15 | orphan positive stdin=peer block; retain line-parser refusal | B | 1 |
| silent-fall-through | 2026-10-08 | arm64 Linux acceleration omits KVM | D | 10 |
| silent-fall-through | 2026-10-10 | editor options: timeout, width, height, wait | B | 2 |
| silent-fall-through | 2026-10-10 | processCmdWithDepth actual tick cost | B | 2 |
| silent-fall-through | 2026-10-10 | IPsec zeEnvironment log key consumer | G | 12 |
| test-gate-repeats-expensive-work | 2026-10-07 | functional --pattern documented but refused | E | 15 |
| test-gate-repeats-expensive-work | 2026-09-12 | hookSessionStart derived-artifact work | E | 8 |
| test-gate-repeats-expensive-work | 2026-09-11 | session-start harness 5-second timeout | E | 8 |
| test-mutates-a-running-subject | 2026-10-15 | TestUnionTimeout writes sweepInterval after NewUnion | B | 13 |
| test-type-not-backfilled | 2026-10-06 | RFC naming/tag debts; bounded nine-finding reconstruction, original proof log missing | E | 16 |
| test-type-not-backfilled | 2026-06-14 | new technique existing-code population | E | 16 |
| test-type-not-backfilled | 2026-06-14 | fuzz target equivalent-code population | E | 16 |
| helper-bypassed-by-an-open-coded-copy | 2026-08-14 | main .ci TmpfsFiles mode loss; retain parse-path predecessor proof | B | 17 |
| helper-bypassed-by-an-open-coded-copy | 2026-08-31 | activityStyle light literals in dark theme | H | 17 |
| parser-refuses-a-form-its-producer-emits | 2026-10-14 | checkSnapshot quoted Git paths | A | 17 |
| policy-change-never-re-evaluates-what-it-admitted | 2026-10-22 | GoBGP export UpdateConfig/OUT reconciliation | G | 17 |
| stop-leaves-the-work-running | 2026-10-11 | runDocker kernel container survives CLI kill | D | 18 |
| stop-leaves-the-work-running | 2026-10-20 | installStopped inaccurate progress/reboot guidance | D | 18 |
| stop-leaves-the-work-running | 2026-10-20 | installCommand TERM-ignore wait AND second interrupt | D | 18 |
| stop-leaves-the-work-running | 2026-10-20 | six unrooted routes: l2tp-ppp, gokrazy-l2tp, l2tp-scale, hugepages, docker-lab, yangcontract | D | 18 |
| test-against-broken-path | 2026-08-18 | browser web-config validator versus recording | H | 19 |
| test-against-broken-path | 2026-08-29 | plugin-import and YANG-glue generator tests run non-main file | H | 19 |
| test-against-broken-path | 2026-09-01 | TestProbeUnclaimedRoutes and TestScratchProducerCensus committed probes | H | 19, 38 |
| test-against-broken-path | 2026-09-05 | requireEveryJobAdvisory versus evidence-nightly | H | 19 |
| test-against-broken-path | 2026-09-28 | le-discovery-answers compares working tree, producer unpushed range | H | 19 |
| unwired-feature | 2026-09-02 | CodeQL Go extraction ignores paths-ignore | H | 17 |
| zero-value-as-valid-answer | 2026-08-26 | le toolchain empty derived feature tags | C | 20 |
| zero-value-as-valid-answer | 2026-09-21 | comparison drops second ze pass AND convergence publishing | C | 20 |
| zero-value-as-valid-answer | 2026-10-15 | listStdlib empty successful listing | C | 20 |
| bound-too-small-for-its-own-burst | 2026-10-22 | runSlug full selector list filename | C | 21 |
| bound-too-small-for-its-own-burst | 2026-10-25 | job stall window kills silent foreign gomu | D | 36 |
| bound-too-small-for-its-own-burst | 2026-11-02 | browser first-open startup bound only widened in verify mode | B | 2, 35 |
| declared-format-contradicts-payload | 2026-08-31 | session Uncommitted heading over shared tree | E | 22 |
| declared-format-contradicts-payload | 2026-09-01 | not-enrolled source-restricted kind for ISO10589 | F | 23 |
| gate-excludes-part-of-its-population | 2026-08-22 | wire speaker census, including non-Go readers/writers | E | 24 |
| gate-excludes-part-of-its-population | 2026-09-27 | eight documents; each disposition retained below | F | 25 |
| gate-red-where-nothing-blocks-on-it | 2026-08-30 | eighteen site goconst findings | H | 26 |
| gate-red-where-nothing-blocks-on-it | 2026-08-31 | CLI grammar release gating | H | 26 |
| gate-red-where-nothing-blocks-on-it | 2026-09-10 | post-OSPF 93 host, 104 Linux, two capability, one uncovered file | H | 26 |
| gate-red-where-nothing-blocks-on-it | 2026-09-10 | repo wiring, changed-test audit, doc drift/summary/anchors | H | 26 |
| gate-red-where-nothing-blocks-on-it | 2026-09-17 | le-docvalid-answers, le-evidence-answers AND le-job-admits | H | 26 |
| gate-red-where-nothing-blocks-on-it | 2026-10-05 | pluginOwnershipGate and le-placement-answers | H | 26 |
| published-value-drifts-from-the-behavior-it-describes | 2026-10-24 | lab pin report group contains writing bump | H | 34 |
| validate-accepts-what-start-refuses | 2026-10-26 | weakening approval accepts unusable package-clause unit key | A | 31 |
| reference-checked-claim-unchecked | 2026-10-26 | rendered-rule fixture names retired update index | E | 8 |

The four entries under Rows found already fixed also remain owned evidence claims:

| Class / date / surface | Owner | Proof still required |
|------------------------|-------|----------------------|
| silent-fall-through / 2026-09-06 / parseCIFile unknown directive | B | P-1: current parser refusal through a real file |
| guard-added-to-one-half-of-a-pair / 2026-09-21 / RSVP-TE buildResv filter | G | P-12: current producer, emitted filter and discriminating carrier |
| protocol-state-advanced-before-the-peer-confirmed / 2026-09-28 / LDP handleInit | G | P-12: acceptance barrier before operational and real session proof |
| silent-fall-through / 2026-09-21 / LDP DecodeInit U-bit | G | P-12: unknown mandatory TLV refusal and optional control |

### Research populations and evidence gaps

These are explicit open research deliverables. They cannot be closed by marking
their parent AC covered, by moving a journal row to fixed, or by a current green.
All source-repair statements in this table are **audit reports awaiting producer
verification** unless the S table above names the same fact.

| Owner | Population or blocker | Retained lead and required next evidence |
|-------|-----------------------|------------------------------------------|
| A | Removal/message/debt predecessors | Removal has S evidence above. Reports say random exclusive messages and Git-derived debt repair the other predecessors. Verify allocation, failure ordering, duplicate/replace routes and current discharge consumer; keep every historical occurrence |
| A | Approval and review design | Final contract below selects exact SHA/unit binding, explicit approval publication, canonical same-carrier validation and prospective-tree review on every CAS attempt. Parent reviews all table/trailer readers, including semantic weakening and unchanged owner-ruling semantics, before implementation. Runtime entry proofs remain owed |
| B | Editor/web scheduler adapters | Final audit reports runTestCase result before cleanup, Close joining only queued dispatch, and noncancellable cmdLoadNew file I/O/AcquireLock. P-2's owned registered child is the candidate; context/timer changes alone are insufficient. Carry capability/exclusive metadata through existing scheduler records. No current family proof needs blanket privileges or exclusivity. Parent verification, approval and lifetime/adapter proofs remain owed |
| B | Browser startup | Final direct-source reconciliation below confirms existing concurrency headroom once independent of verify, deadline before server/browser startup, remaining-budget propagation, waitLoad failure and bounded owned cleanup. P-2 names the proposed carrier; parent review and executed readiness/cleanup proof remain owed. Broader descendant guarantees stay with sandbox AC-5; no adaptive host-load policy |
| B | Process completion | Confirm every early return in runTest/runOrchestrated, Kill-without-Wait and lease ordering. Check interop cleanup text and structured/final status after an assertion pass; the original incident does not prove a Docker race |
| B | Persistent-store census | Final B report semantically classifies concrete candidates `storage-ntp-restart`, `storage-tc-restart`, `ospf-state-through-daemon`, `traffic-vpp-reject-prio`, `traffic-vpp-reject-hfsc`, `traffic-vpp-reject-mark`, draft `audit-config-commit` and BFD. This is not certified failure or whole-program tagged-gopls census. Parent must verify each disk reader/restart and memory-only control; complete population proof remains open |
| B, E | Root and seed identity | B reports buildLGService maps any nonnil store to blobCertStore then selfcert LoadOrGenerateCert; memory TLS can mask a wrong seed. Candidate: same effective root and normalized dotted/uppercase aliases, with served certificate matching seeded store. Parent reports validateSessionBinary lexical checks without ownership/symlink confinement. Actual seeder-entry operator-store nonmutation and ownership/confinement proofs remain owed even with outer guard bypassed |
| C | Toolchain/report producers | Audit empty `listStdlib`, pass-aware comparison selection and all convergence publishing consumers. Bound runSlug as above. Verify source-repair reports for feature tags, vendoring, schema links and launcher delegation rather than rebuilding predecessors |
| C | Backup lint inputs | Determine whether the original Go backup is selected by lint before changing its guard. Preserve foreign evidence. No deletion, blanket ignore, or lint-all-scratch solution is approved |
| C, H | Site and coupling inventory | Direct reads confirm roadmap's resolve/tree/readBlobs object-reading mechanism, declarations.readPackage's AST/import-alias mechanism and coupling.parseTree's live-source walk. Extend them into internal/le/repo/inventory, not a host-registry substitute list. One resolved snapshot feeds membership, metadata, YANG, coupling source and gates. Select entry's transitive imports under full all-feature Linux le build.Context; support init helpers and observed expression forms in the final contract below, refusing unknowns/mutation. H's 104 lexical literals plus one runtime registration is an audit observation, not a frozen expected count or independently reproduced census |
| D | Reclaim and guest details | Standalone reclaim, directoryless marker and interrupted removal remain separate obligations. Attribute cache-symlink xattr failure. Check arm64 KVM selection, bootstrap `&&`/ungrouped `\|\|`, cwd restoration and unknown-probe attribution at their producers |
| D | Cancellation census | Trace all six named routes, kernel container teardown, install progress and second interrupt. Include build, slot wait, startup and running phases. Verify the reported 98592039b3 guest-parent admission repair. Silence never authorizes stopping foreign work |
| E | Hook behavior, not fixture labels | Four categories retain their original intent: commit deferral, test-first, delegation reminder, subagent context. Reconcile each with current rules before implementation; no prose-sensitive gate returns. Subject/freshness evidence and daemon-path demand are separate from door-read enforcement. Keep panic mixed-call, Stop-category and 5-second startup proofs |
| E | Backfill inventory | Six stale absence-based specs reportedly repaired at 9059161574; their diff/current replacement census remains owed. E reports a bounded 1720f2b6cd census: 4995 committed eligible files plus four archived probes yield seven committed and two probe naming findings; two other probes pass. The companion records all nine dispositions and distinguishes direct source/diff checks from E's unrepeated census. This is not the authentic job-enum-post-framing-canonical-rfc-check-f8aecd88.log or indexed final-proof archive, which remains missing. June 361e7c0b8f has one generic pattern; migration 2cff2050ac creates two rows without inventories. Neither proves two distinct original incidents nor completed backfills. Original June authoring/August migration conversation or owner-held technique/predicate/inventory evidence is still required for both obligations |
| E | State/template/guidance | Direct-edit WIP bypass, shared-tree heading, missing wire reader/writer census, retired fixture needle and source digest remain separate checks. Current guidance must remove all agent verify instructions, not merely those on Linux. Parent owns the rule synchronization; this edit changes only this spec |
| F | Eight-document accounting | ASPA-profile, RFC4506, RFC5080, RFC7519 and RFC1008 stay unenrolled with obligations/correction provenance preserved; RFC9117 existing repair and RFC7383 optional debt remain distinct. Method14 implementation and RFC8247 §3.1 normative span were directly read for the final contract; add the missing SHOULD row with fresh ID and extraction/tag/discrimination. No source read is conformance proof |
| F | Guest proof and authority | Direct read confirms unitNeedsGuest returns host-first and requireGuestKernel refuses a kernel for that placement. Explicit kernel selects a compatible guest; no-kernel behavior stays unchanged. Prove exact-unit execution and zero skips. The final owner-issuance contract below retains interpretation and existing records, gates owner-ruling reasons/attestation/push preparation/owner-authorised review recording, and preserves ordinary judge and review routes |
| G | Fifteen checkers | The historical fifteen are named in the carrier table below, matched to `specialCheckers` in `internal/le/interoplab/bgp/check_special.go`. OTC is a different registered checker and is not a member of this incident. Do not freeze the current registry's total count. Bodies reportedly restored; each historical member still owes its pinned-peer renderer and live discrimination proof. A restored function or synthetic log fixture is not a live oracle |
| G | IPsec, staging, RTR and generated peers | Parent reports ZE_LOG_LEVEL mismatch. Verify logger consumer, Bench.Run build-only guard and RTR sync handshake. Generated LOCAL_PREF lacks reported session classification: P-28 needs all four family/session combinations across active/passive and ordinary/connmap routes, plus unchanged raw-hex control |
| G | GoBGP reload | Read GoBGP v4.11.0 `UpdateConfig` and destination `Update.GetChanges`, including OUT bookkeeping. Establish the peer version actually used and expected wire transitions. Do not alter Ze or waive the scenario on an unverified peer explanation |
| H | Source-repair reports | Generator invocation, discovery, homepage, partial-health metadata, lab grouping, renamed callers and several lint sites are reported repaired. Verify each relevant producer and existing regression before closing its occurrence |
| H | Job-admission and rules fixtures | `le-job-admits` reportedly depends on a 3-second overlap; prove a bounded entry/attach/release rendezvous. Parent verified that exact-key env removal leaves dotted root able to override uppercase in the rules fixture. That supports a current isolation defect, not attribution of the historical render-update red |
| H | Publication/portability proofs | Final source reports supply candidates in the carrier table, not executed proof. CodeQL has three manual tagged ./... builds; rewriteCodeQL changes ze_core markers while preserving suffix, and paths-ignore does not exclude compiled Go. Extraction CI proof remains owed. DNS candidate below needs independent test-change judgment. Chaos, browser, poster and theme proofs remain owed; claims-count refusal is not loosened |
| H | Retirement and carrier containment | Preserve owner-approved retirements and map old names/count/order or metadata/prose assertions to replacement proofs. Successful dispatch is additional. Resumed source reads confirm verifyAsset/fileDigest path reuse, readCastFacts reopening and copyPath symlink copying. Final contract requires rooted acquisition plus retained verified bytes/staging; retirement alias tests are a separate boundary. Parent review and containment proof remain pending |
| H | Aggregate lint evidence | Partially recovered in the companion: A has 11 coordinate lines; B has 184 coordinate/153 normalized lines, not the historical 138 estimate or whole 1272-line log; C/D complete retained normalized old/new lists have 121/163 lines. Context/flavor/coordinate losses, B's cut suffix and uncaptured diagnostics remain gaps. Original post-OSPF tmp/session/2026-09-09-5bc855cc-5761-4012-8d90-7a9823029ab3/scratch/ospf-closure-lint.log remains missing; named repairs or current greens cannot close those populations |
| H, B, D | Load-red evidence | Partially recovered per companion and incident table below. A has five of seven web names; B has all seven reload timeout names and 21 of 22 web names. B web ID3 remains unknown because the historical numeric filter excluded it; never borrow A's ID3. Raw context and uncaptured members remain missing; P-35 cause/repair proofs remain open |
| I | Incident evidence | AC-27 incident 644a4e78-0122-4eeb-8642-eb50638a1641: scratch/ze-zeroed-segment.tar.gz, scratch/ze-zeroed-schema.tar.gz and actual traces remain unrecovered. Need another copy/host snapshot or observed recurrence retaining source/destination across the native boundary. Cause and attributable repair remain blocked |

The aggregate lint obligations include every named subpopulation from the journal:

| Historical set | Retained membership and remaining evidence gap |
|----------------|----------------------------------------------------|
| 0723d7d26a lint | Companion retains all 11 selected coordinate diagnostics, including cache misspell, gNMI stringXbytes, l2tp typeDefFirst, IKE undefined symbols and deployment/QEMU constants; selection omitted typecheck headers and raw context |
| e6474aa200 lint | Companion retains all 184 selected coordinate diagnostics (153 normalized) across CLI/config, DelayOpen, IKE/IPsec, terminaldemo, deployment/QEMU/RFC, l2tp/pppoe, fixtures, log, VPP/DHCP and other named sites. Historical approximately-140/138 estimates do not define the set; orderedTransportChild suffix, raw context/flavors and uncaptured lines remain missing |
| 2f36373081 lint | Complete recovered old.txt: 121 normalized diagnostics, including Suite.Run, waitRecord/waitStop, exclusive-group ifElseChain, site home goimports and terminaldemo .webp constants. Normalization removed coordinates/duplicates; original raw logs remain missing |
| 043c682d24 lint | Complete recovered new.txt: 163 normalized diagnostics, including delay_open, session misspell, BFD fixtures, check_delayopen siblings, IPsec, teardownAutomatic, bestfact, netlink and trafficusage. Preserve journal's check_srv6_identity lead separately; normalized selection cannot certify every raw log member |
| Post-OSPF gates | 93 host and 104 Linux/integration findings; two capability findings; one tracked file outside lint; check_paths_limit staticcheck/wastedassign; 24 wiring and 16 changed-test findings; 3759 doc drifts, eight summary-rule failures and source-anchor failures. Counts are historical claims awaiting the original logs, never current measurements |

The load evidence leads retain the other runs named by the package audit so they
cannot disappear inside the 043c682d24 short label. They remain research leads
where the original owned index did not enumerate them separately.

| Historical set | Cases / evidence still owed |
|----------------|-----------------------------|
| 043c682d24 | Companion retains TestOwedKeyGRPurgeRecoversSurvivor/end-of-rib/no-survivor and TestBFDNegotiatedPublicationJoinsSessionEnd (race and ordinary; peer_run_test.go:880: read loop did not finish while publication was paused), plus api-peer-create-delete-rib, peer-admin-shutdown, plugin-command-document-too-wide, update-text-sr-policy-addpath, debug-toggle and rbac-web-config-deny. Preserve the additional isolated rbac failure obligation; selected excerpt is not the whole log |
| 2f36373081 | TestInProcessSpeed; modify-oversize-suppress; rfc9234-rs-client-learned-not-upward; pipe-local-command; client-backup-start |
| e6474aa200 | staticcheck part 6 deadline remains historical, not a rerun instruction; retained PTY/SRv6 unit diagnostics, session-editor-commit-confirmed-accept, storage-bfd-restart, deprecated-bare-config and bfd-pinned-profile-edit-up-live. Seven recovered reload timeouts: bgp-bfd-profile-edit-live, bgp-bfd-removed-live, bgp-filter-irr-loaded-over-ssh, bgp-rpki-added-live, bgp-rpki-attach-live, commit-verify-reject, request-reload-promotes-operator-file. Companion retains 21 web names; web ID3 alone remains unidentified. Context/cause proof still owed |
| 0723d7d26a | login-on-tree, web-only-interrupt-graceful, web-tool-decode, tacacs-key-required, le-completion-shells, audit-config-commit, concurrent-config-commit, appliance-mpls-qemu and crash-capture-panic-harvest-qemu retained. Five recovered web names: admin-command-posts-argument, interface-configured-display, nav-telemetry, lg-error-fragment, interface-mac-override; two remain unnamed. TestLeafListSequencesAgree full trace remains missing beside retained package-budget panic |

#### Final B/H candidates: evidence boundary

The final B/H reports answer the source questions above as **audit reports**.
Parent producer verification remains pending except facts already marked S or
explicitly identified as direct reads in the consolidated contracts below. All
new carriers and breaks remain proposed; no reader, repair or proof was executed
by this documentation update. H reports no additional owner choice for its minimal
repairs, but main design approval still applies to the whole candidate.

| Proof | Reported source and proposed repair | Required proof / remaining work |
|-------|-------------------------------------|---------------------------------|
| P-34/P-35 DNS | H reports disabled_test TestResolverLeavesSystemNameServers calls Configure(path, stub), then reads the file; nonLinux writeResolvFile returns nil without creating it, while Servers reads recorded state. Candidate directly seeds exact bytes `# Generated by ze\nnameserver 192.0.2.53\n` in the temporary file before Configure and asserts initial bytes | Retain absent/enabled/disabled transactions, published list, Servers and unchanged-byte assertions. Existing Linux system/resolv_test covers writer bytes, precedence, lease withdrawal and foreign-file preservation. Independent test-change judge remains owed; no skip or production writer change |
| P-34 Doc review | H's substantive source review is now performed, per its final report; corrections remain outstanding. The old attestation predates declaration/docs changes | Correct docs/features/ai-first.md and help_ai.go producer claims listed below, then record truthful review evidence. Selected feature carriers do not prove universal runtime transport parity; no date bump now |
| P-33 LG persistence | B's nonnil-store → blobCertStore → LoadOrGenerateCert report explains why successful memory TLS is not seed proof | Proposed P-33 entry proof must compare served certificate to the seeded store, with identical effective root and normalized aliases; wrong-root/memory substitution must fail |

Historical payloads and provenance are retained in
[tooling-one-off-evidence.md](tooling-one-off-evidence.md). Its normalized lists
are historical data, never current lint output or causal attribution. Parent's
exact-string scans of conversations `1b0ec3e6-5946-4d37-8431-e1065e283551` and
`d75c882e-6276-40c7-b8cb-70c54a757f48` returned no hits: bounded unsuccessful
searches, not global absence. CI jobs `122128638570` at
`b0da8037c5c5532156aba1b4818f21ae515df6a7` and `122353330515` at
`58b85bad1660eb9d37d29e14f4614435796e88b2` are different-head corroboration only.

### Package G supplemental audit report

The following is **audit-reported research; parent producer verification is
pending**. These are not observed runtime reds, approved fixes, or grounds to close
source-repaired rows. They refine P-12 and P-17 without replacing their obligations.

| Surface | Reported finding / conditional candidate | Proof still owed |
|---------|-------------------------------------------|------------------|
| GoBGP v4.11.0 export reload | Changing `UpdateConfig` from IN to BOTH reportedly refreshes unchanged-best and current-candidate exports. It does not settle a disappearing destination for a secondary recipient: `sendSecondaryRoutes` computes old paths under the current export policy, which can reject all old paths; the current list is empty, `Table.update` deletes the destination, and OUT cannot enumerate it | Read the pinned producer and callers, then drive actual SIGHUP/UpdateConfig rather than invoking the route helper alone |
| GoBGP companion candidate | Conditional on producer verification: extend the no-selected-path branch using `hasPathAlreadyBeenSent`, emit the withdrawal in `paths` because the caller discards `filtered`, and reuse `updateRoutes` and its outgoing queue | Ordinary and secondary recipients: reject-to-accept and accept-to-reject, attribute changes, unchanged best, last-candidate removal, replacement, simultaneous import-removal/export-rejection, never-sent destinations, repeated reload and counters. Require stable session generation and no source UPDATE, refresh or reconnect |
| GoBGP proof boundary | The report supports only a candidate for stable-session, non-ADD-PATH unicast. ADD-PATH IDs/send-max, VRF, deferred families and repair of prior orphan advertisements are not established | Original four-peer configs, OPENs and captures remain missing. Recover them to determine which obligations reach those paths; preserve each as unverified. No universal reload-correctness claim or silent scope reduction |
| Mixed UPDATE checker | Preannounce 198.51.100.0/24, observe it in FRR, then cross an explicit release barrier before withdrawing it while installing 203.0.113.0/24 | Retain split-shape proof and prove the preexisting route was installed before the mixed update; an absent-before/absent-after observation cannot prove withdrawal |
| No-family EOR checker | Preserve asymmetric OPENs: Ze advertises no MP capability while FRR advertises IPv4 MP. Correct the header and identify the exact sender, family, VRF and current epoch | Read the pinned renderer and assert the intended EOR event in the current session, excluding stale or other-family output |
| Installed-route predicate | Substring matches do not prove a route was installed | Structured exact prefix, protocol, installed state and nexthop must occur in the same route object; reject wrong-prefix and partially matching objects |
| Native speaker / EVPN discard | Decoder reportedly clamps, stops or skips malformed input; `requireSpeakerEVPNDiscard` lacks capture-complete and ze-open controls | Strict decoding, exact retained type-2 route, full handshake and completed-capture controls. Truncated or malformed capture must refuse proof, not look like discard |
| Shared JSON route oracle | `jsonHasRoute`/`nestedRoute` reportedly infer presence from any paths/prefix key and absence from other valid JSON | Pin the peer output schema; test exact prefix, empty paths, wrong prefix, error JSON and unrelated valid JSON. Unknown schema is an error, not route absence |
| Fifteen-checker population | Bodies are restored according to the audit, but individual pinned-renderer and live discrimination proofs are incomplete. The current FRR pin is reported as 10.7.1; old 10.3.1 comments are not authority | Verify the actual lab pin and producer for each checker. Record a live red/green discrimination result per feature; no shared helper pass certifies all fifteen |

### Consolidated contracts for the main approval gate

These settle the alternatives using the existing mechanisms; they are proposed
implementation contracts, not implementation or executed proof. Direct source
reads in this resumed pass confirm the named producers below. H's complete
registration census and the remaining B/H report details retain their audit
attribution until the parent checks them. No source-only check closes a P-number.

#### A — approval, review and path identity

`ApproveInto` currently replaces a row by unit; `rfcChangeProblems` selects current
changes, and `answering.answered` uses a unit-only approval map. Extend those
writers/readers together. Do not create a second approval ledger or use owner
attestation as an independent judge's publication route.

| Contract | Selected design | Refusal and proof boundary |
|----------|-----------------|----------------------------|
| Approval writer | `le rfc approve` gains optional `commit <fullSHA>`. Validate that the target changes the named unit by reading target parent and target blobs. Extend the existing table with target identity; key targeted rows by exact `(changeSHA, unit)` and keep ordinary rows unbound | Require canonical full commit SHA, resolvable history, changed unit and unique carrier. Rebase rewrites identity and needs a new judge approval; no patch-id translation of approval |
| Canonical unit | Keep the existing directory-based key. Resolve ordinary approval against current/HEAD carriers, and targeted approval against target parent/commit carriers. Use existing carrier/ChangedUnits readers, including non-Go, deleted units and semantic weakening | Reject package-clause aliases, ambiguous directory keys, unknown units and unchanged target units before writing. Do not narrow the reader to live Go tests or structural weakening findings |
| Explicit publication | Extend ordinary `commit create` with repeatable `rfc-approval <revision> <unit>` selection, resolving each revision once to its full SHA. Permit an approval-only empty publication through the existing private-index publish route. It carries selected judge rows, never invented reason text | Unselected rows stay pending. Refuse missing/replayed selections and incompatible create policies; approval-only publication changes no files. Preparation and failed publication prune nothing |
| Binding trailers | Selected late row writes ordinary `RFC-approved: <unit>: <reason>` plus companion `RFC-approved-for: <fullSHA> <unit>` in the same carrier commit. Owed derivation stores exact change/unit binding, not a global unit map | Both trailers must occur in the same carrier; target must precede that carrier, and the applicable obligation/approval must belong to the range being published. Reject malformed/ambiguous pairing, later targets, wrong units, out-of-range approval and reuse for another change. Unbound trailers answer only their own commit |
| Consumption | Keep used-row pruning in successful post-publication bookkeeping. Landed detection includes target binding, so identical reasons on different changes do not collapse | Retry after failure preserves the row; retry after successful publication cannot spend it twice. Existing owner-ruling interpretation is unchanged; shared table readers retain its ordinary-row semantics without authority/provenance additions |
| Prospective review | Record HEAD once before content selection, snapshot each selected whole file once before any content gate, and use the same immutable blobs for hunks and gates. Build one prospective Git tree through a private index: recorded HEAD, earlier script blocks in order, current snapshots/hunks, then removals | Historical spec-work paths come from that tree, not the working copy. Recorder supports explicit blob and removal sources, including modes. Unreadable/missing objects refuse. Git objects and private indexes are not checkout exports or source copies |
| Generated review check | Pass explicit originating review artifact identity and actual proposed tree to the generated checker on every compare-and-swap publication attempt, after constructing that attempt's tree and before publication | Different ambient session cannot substitute an artifact. Concurrent HEAD changes retry only with another review check. Tests cover earlier blocks, selected hunks, deletion, mode change, missing blobs and concurrent unselected edits |
| Git paths | Use raw NUL-delimited `ls-files -s -z` entries and `update-index -z`; update snapshot checks, script block walker, hunk selection, history enumeration and shared-index synchronization consistently | Retain refusal of NUL, tab, CR and newline paths. Spaces, quotes, backslashes and non-ASCII must retain exact identity. No whitespace splitting or Git-quoted path left as a literal filename |

Direct `prepare.go` inspection places `checkVerificationGates` before the current
whole-file snapshot; `script.go` places ReviewCheck before renderPublish's CAS
loop. Those are the boundaries this plan moves. The semantic-approval work in
`internal/le/test/weakened` and `internal/le/commit/semantic_approval_test.go` is
shared-session work: coordinate its current owner and inspect the latest diff
before implementation. Do not overwrite or rebuild it from this report. Its
semantic-change approvals remain supported by the new table and snapshot readers.

#### B — execution, clocks and isolation

Direct reads confirm `runTestCase` returns on timeout while its worker continues;
the worker sends before deferred cleanup. `cmdLoadNew` calls `readFile` without
cancellation, and `store.acquire` takes blocking mutexes. `processCmdWithDepth`
adds a 900 ms slow-path wait and `settleWait` drains an entry snapshot, including
timer commands. Cooperative cancellation alone cannot bound these paths.

| Contract | Selected design | Required discrimination |
|----------|-----------------|-------------------------|
| Editor process | Register `le mock editor-case` under existing mock dispatch. Parent owns immutable case content, absolute deadline, temporary resources and result/diagnostic FD IPC; no re-read of a mutable authored path in the child. Child sends its one result only after cooperative model/store close; parent publishes only after the sole Wait owner and cleanup finish | Hung read/lock, ordinary completion, parse error, panic, partial IPC, EOF, cancellation and cleanup failure. No result before cleanup and no post-result writes; never accept missing child result as success |
| Stop order | Start with `plugin.StartChild` and `KillGroupOnCancel`, preserving stable spawning-thread/Pdeathsig behavior. Request cooperative stop/diagnostics with a proposed two-second maximum grace, then cancel the owned group and wait; use the existing two-second bounded pipe wait separately from that grace | Use separate cooperative and hard-kill stages: canceling the CommandContext immediately invokes SIGKILL, so it cannot also be the diagnostic grace. Diagnostic writes cannot extend grace. A kernel-stuck child is an explicit unreaped failure retaining ownership, never a fake completed reap or free lease; bounded pipe wait is not a kernel-reap guarantee |
| Model runtime | Share runtime context and clock across model copies; extend existing core clock NewTimer/tick handling with cancellation and Stop. Default authored .et runs use real time, including real daemon confirmation windows; fake clock is for focused tests only | Stop timers on close, reject stale generations, and apply only completed command results. Prove timer expiry actually triggers behavior and late ticks cannot mutate a restarted model |
| Tick-cost obligation | Remove unnecessary harness waits and distinguish state-command completion from recurring timer scheduling. Do not eliminate all real ticks or replace them with no-ops. The old unconditional-real-tick elimination suggestion is superseded by this source-backed contract | Focused clock tests show the extra 900 ms wait is absent for timer scheduling; authored real-time cases still observe validation debounce/current generation and confirmation expiry. State-changing commands must finish before assertion, never time out into apparent success. Retain a deliberate dropped-tick failure and real daemon confirmation proof |
| .et options | Keep 80×24 and 30-second defaults. Validate positive dimensions/budgets and overflow; apply dimensions to every restarted model. Deliver authored wait milliseconds to the runtime, with cancellation, generation checks and real expiry | Independently drop width, height, wait, restart sizing or expiry; every break fails its own observable. Invalid/missing values refuse rather than fall back to success |
| Scheduler adapters | Reuse runner option arms for needs-linux/caps and exclusive grammar and carry them in Record from parser to scheduler. ParseFailed outranks skip. Capability placement/refusal happens before editor/server/browser effects; shared exclusive lease lasts through cleanup | Separate .et/.wb proofs for malformed declaration, capable/incapable target, same-group cross-invocation exclusion and different-group independence. No blanket privileges or exclusive group is assigned to a family |
| Browser budget | Scheduler concurrency determines effective authored budget once: factor 3 for concurrent and 1 for serial. One absolute case deadline begins before server/browser startup; readiness, viewport, locale, open, actions and waits are clamped to remaining time. waitLoad timeout is an error | Direct waitLoad read confirms current timeout returns nil. Test exhausted startup, slow phases and waitLoad nonsettling; verify-mode env cannot alter policy and no nested phase resets the budget |
| Browser ownership | Explicit owned session wins inherited session settings; normalize aliases before spawning. Close that session with a separate bounded cleanup context, propagate close errors, and join owned command/server processes before releasing leases | A close timeout/error is cleanup-incomplete, never an unqualified pass or proof of detached Chrome death. No close-all or foreign-process sweep. This incident needs no subreaper, Rust pipeline or quarantine; broader descendant guarantees belong to spec-test-sandbox AC-5 |
| Other lifetimes | Runner has one Wait owner on every return; interop cleanup failure makes overall result unsuccessful while preserving the assertion verdict. Union configuration, including maxPending and interval, is immutable in a private constructor before worker start | Early/normal/timeout/panic return tests; assertion-pass plus cleanup-error in text and structured result; union constructor/race proof without writing live configuration |
| Persistence | Convert the six active semantic disk-reader cases, draft audit-config-commit, BFD restart and LG only where their assertions require persistence; memory-only controls retain memory isolation. Normalize seed/start roots and compare LG served DER fingerprint with the seeded certificate | Wrong-root, memory substitution, restart, dotted/uppercase alias poison and seeder symlink/operator-store sentinel cases. The named set is an audit census, not certification of all tagged-program readers |

#### Lifecycle scope checked against the original incidents

Direct reads of `bound-too-small-for-its-own-burst.md` (2026-11-02),
`stop-leaves-the-work-running.md` (N6), and `spec-test-sandbox.md` AC-5 confirm
the scope correction reported by independent review
`ses_e6210a2f1ffeRnt11QyvjDR1dF`. The browser journal asks for startup headroom
independent of verify mode. `contended` currently tests VerifyModeEnabled;
`runAgentCore`/`runAgentOutput` start fresh Background timeouts, `waitLoad` returns
nil on expiry, `agentEnv` lets inherited session win, and Close discards errors.
`zeTestRunWebTest` starts its server before the case budget. Fix those producers
through the budget and ownership rows above. The sandbox's broader no-survivor
guarantee is not a prerequisite of this repair, and this spec does not claim it.

For the installer, `installCommand` deliberately retains terminal stdin/stderr
and relays TERM through sudo; `runDockerKernelInstall` defers NotifyContext's stop
until the entire install returns. Select N6's minimum repair: release that signal
registration on the first cancellation so the second interrupt takes its default
action. Apply the same root handling to reached callers of the installer command.
Keep completed-step state and report the active unfinished step before waiting,
including the warning not to reboot after an incomplete modifying sequence.
This restores operator escape; it does not promise that a second interrupt reaps
the privileged child. No shared tty process-group kill or background sudo group
is selected. The separate container and six rooted-verb teardown obligations stand.

#### Owner authorization through existing producers

Direct reads select four existing issuance routes: RFC `Approve` into the existing
approval file, `attestationTrailer` during `commit create attest`, and
`preparePush` for `commit push`, plus `recordReview` when owner-authorised is
supplied. `ownerRuling` already defines exactly which RFC
approval reasons can authorize a ruling at `ownerRuledMove`; reuse that predicate
at RFC issuance to require terminals only for those reasons. An ordinary judge
verdict is still agent-callable. Put the check before the writer, including the
targeted-approval extension, so the shared `ApproveInto` weakening writer does
not acquire a blanket terminal restriction. Reuse the current table and trailer
consumers; they do not require a terminal to consume a recorded authorization.
Gate attestation before message/script recording and push preparation before any
writes: preparation records the owner's instruction in the generated script.
That already-prepared push script remains agent-executable and noninteractive;
the gate does not change the existing owner-ordered push route or its checks.

Independent reviewer `ses_e636b0572ffe7X7pdz624IL8Dp` identified the fourth route.
Direct inspection confirms `recordReview` copies nonempty OwnerAuthorised into
the artifact even below reviewOwnerRoundCap. Gate whenever owner-authorised is
supplied, before any writes, not only when the round count requires authorization.
Ordinary review recording without it remains agent-callable; existing above-cap
authorization requirements still apply. Below-cap and above-cap nonterminal
cases must leave artifacts and directories unchanged, alongside terminal-positive
and ordinary noninteractive review controls.

Reuse `internal/le/le/human`'s `IsTerminal` detection of stdin and stdout, refusing
false or unanswered detection. Its current RequireTerminal/Check refusal text is
specific to full verification: extend its shared terminal check with an
authorization-appropriate refusal rather than printing a verify instruction or
inventing another detector. No environment/flag bypass. A PTY satisfies a TTY
check, so this records an operational restriction, not authenticated authorship.
F's proposed `rfc authorise` and new owner record are unnecessary and not selected.
Direct file editing is not made tamper-proof; that would be the authority redesign
the owner declined. Tests cover issuance and unchanged noninteractive consumption.

#### C — comparison validity and publication

`cmdReport` decodes result JSON and sends Markdown, HTML or PerformanceDoc directly
to stdout; it has no JSON comparison output. `Markdown` groups before writing,
HTML writes its header before grouping, PerformanceDoc deduplicates first, and
Trend writes its header before walking results. Put one report-validity check
before any output or selection in all four entries. `cmdTrack` currently applies
--last before Trend: validate the full decoded input before that filter as well.
Every raw row must be checked, including older duplicates and rows outside the
display window. Refuse nonzero RoutesLost with input identity and loss count;
do not discard invalid rows to manufacture a valid comparison.

`latestComparisonResults` already keys by DUTName/Family/ForceMP/Pass and selects
the latest timestamp, with later input winning ties. Retain that policy after
validation; PassUnrecorded remains a separate row. Preserve convergence presence
at JSON and NDJSON decoding, where the current integer field otherwise conflates
missing with zero. Carry that distinction to report validation without guessing
that a valid explicit zero is a missing measurement. Direct measured Result
producers must supply the corresponding presence fact; existing raw serializers
remain diagnostic evidence and retain loss, rather than becoming comparison gates.

No renderer may emit bytes for invalid inputs. Once a valid report is written to
stdout, an I/O failure is returned but already-written bytes cannot be rolled back.
Where tooling owns a destination file, validate/render before replacing it and
use temporary-file replacement per file; a failed replacement leaves its previous
bytes. `site/build.go` copyPath currently removes the target before writing and
copies symlinks: its reached artifact path must consume retained verified bytes
and replace each file as specified by H. There is no multi-artifact transaction.
The targeted site source search found no performance convergence parser; remove
the guessed JSON comparison/site schema requirement, while retaining validation
at any actual report publisher reached by implementation.

#### H/F — committed populations, publication and RFC proof

| Contract | Selected design | Required proof / retained limit |
|----------|-----------------|---------------------------------|
| Inventory membership | One resolved commit and blob batch; select the declared entry's transitive imports for all-feature Linux le with complete build.Context fields/tags, not host registry or a permanent root list. The same snapshot supplies composition imports, feature gates, site metadata and coupling source scans | Simulated darwin must retain target-Linux registrations; dirty edits and HEAD movement cannot mix sources. H reports 104 lexical literals plus one runtime registration: distinguish syntax occurrences from executed registration membership; do not freeze either count |
| Inventory expressions | Extend registration AST/import-identity resolution to init-called helpers, literals/concatenation, local and imported constants, slice references and append (including GR dependencies), getYANG and RunEngine closures/imported defining packages. Resolve required/optional dependencies, StartAfter, roots, description, source and YANG from the same blobs | Every reported expression form gets a fixture and current-tree completeness check. Unknown expressions/conditions, later metadata mutation, ambiguous ownership or missing blobs refuse. A literal without reachable registration is not an automatic member; runtime registration cannot be silently omitted |
| CodeQL population | Derive buildable package dirs from tracked Go for each workflow tag context and explicit cache/vendor/testdata/tool classifications; reject unclassified tracked Go. Check canonical dependency paths before each traced build, not only selected roots. Retain existing bootstrap and rewriteCodeQL tag markers/suffix behavior | Derived selection includes new authored roots and non-cache gokrazy without central list changes. CI extraction includes authored sentinel, excludes cache sentinel; deliberate explicit-cache extraction fails and a dependency/symlink into cache refuses. This is CI debt, not an observed extraction result |
| Artifact bytes | Acquire through rooted artifact I/O and retain the verified bytes or an owned immutable staging artifact for all parsers/publishers. Verify size/digest on that retained payload; never hash one path then reopen/copy another | Direct source shows verifyAsset returns a path after fileDigest, readCastFacts reopens it, and copyPath reproduces symlinks. Extend every reached consumer/copy path. Test escaping links, path swaps between check/use and in-place writes; an open FD alone does not prevent in-place byte changes. Publish exactly the verified payload or refuse |
| Browser journey | One authored login/edit/save/diff/confirm/readback journey serves validation and recording, with capture optional and explicit settling | Actual saved state and authenticated navigation, not Basic HTTP or unsaved DOM. Capture timing, bounded close/error propagation and owned command/server joins remain runtime proof debt; no detached-descendant prerequisite |
| Chaos and DNS | Inject clock through all state/render paths and sample once per operation; stable peer-index tie-break for LastActive eviction. DNS seeds the exact temporary resolv.conf bytes before Configure and retains current state/byte assertions | Clock consumer census includes layout/detail/events/stream/timeline/sync/broadcast. DNS requires independent test-change judgment and existing Linux writer proofs; no skip or production-writer change |
| AI-first declaration | H reports substantive source review completed; implementation must correct docs/features/ai-first.md and the corresponding help_ai.go producers before renewing Doc evidence | Correct overclaims of all CLI/MCP transport parity, unstaleable references, universal diagnostic fields and exact embedded-skill correctness; remove absent ai/NAVIGATION reference. Explain create prepares a script and CAS rechecks, not that create commits. Replace stale ze_announce/ze_withdraw handcrafted-tool claims, qualify generic args/grouping versus typed inputs, and remove blanket MCP-event support. help_ai.go printSummary/Recipes/CommonErrors carry related stale names. Original feature carriers prove selected paths only; corrections and truthful review date remain owed, not completed |
| RFC guest selection | Explicit kernel chooses guest only when the unit matches the guest context; without kernel preserve current host-first placement. Reuse existing observationRunner/QEMU route over main, not a checkout copy | Same exact Go unit must execute with clean PASS and broken FAIL, restored PASS and paired solo-run attribution. Skip/no-execution/compile-failure cannot write a record; incompatible explicit kernel refuses |
| RFC8247 method 14 | Add the missing SHOULD row with a fresh unused requirement ID and original normative span, then extraction/tag/discrimination evidence against the implemented method. Direct computeX509Auth read returns AuthMethodDigitalSig after negotiated hash selection | RFC8247 §3.1 states: “Digital Signature [RFC7427] is expected to be promoted as it provides hash function, signature format, and algorithm agility. Its current status is SHOULD.” Keep the original RFC whitespace/span in the row. This is authentication method 14, not DH group 14; no new RFC enrollment |
| Other RFC dispositions | Preserve the five unenrolled correction obligations (ASPA-profile, RFC4506, RFC5080, RFC7519, RFC1008), RFC9117 existing repair, RFC7383 optional debt and ISO10589 exclusion | Recheck existing carriers/corrections before closure; no current gate green erases these independent obligations. Owner-ruling authority remains untouched |

### Remaining design dependencies

| Dependency | Exact next action / effect |
|------------|----------------------------|
| Settled source contracts | Existing owner issuance routes, browser startup/owned cleanup, installer second-interrupt handling and comparison publication are selected above. No residual owner mechanism choice remains for these repairs; implementation and discriminating runtime evidence are still owed |
| Remaining proof mapping | P-12 build-only/fixed-protocol carriers, P-14 l2tp population and P-37 actual lint-input selection retain their named producer/carrier mapping work before certification. These are implementation/evidence obligations, not requests for another broad design census |
| Parent independent review and main approval | Review these contracts and their phase/file/test/docs plan against current producers, then record main approval before implementation. Process isolation and the metadata evaluator are proposed, not owner-approved by report receipt |
| Historical proof inputs | AC-27 archives/traces, post-OSPF log, remaining October raw contexts/members, authentic naming proof archive, June inventories and G four-peer configs/OPENs/captures remain the exact external blockers above |

Publication form, SHA binding, review tree, clock semantics, explicit-kernel
placement and inventory membership no longer have competing proposed alternatives
in this draft. Runtime tests and CI extraction are implementation debt, not
missing owner decisions. The external blockers prevent global ready/completion;
they do not erase implementable work. No adaptive load policy, unlisted enrollment,
baseline waiver, debt-shard restoration, checkout export or broad gate is selected.

### Implementation phases, files, tests and docs

This is the finite implementation order after parent review and main approval.
Files below are edit boundaries, not instructions to rewrite whole packages. A new
path is explicitly proposed. Within each phase, repair one reached package at a
time and preserve the independent P-number cases above. Before editing, inspect
current diffs and coordinate shared source ownership. Already-repaired producers
need their existing proof checked, not a second implementation.

| Phase / ACs | Files and existing mechanism to extend | Named proof and documentation deliverable |
|-------------|----------------------------------------|-------------------------------------------|
| 1. False-green refusals — 1 | `internal/test/cli` suite dispatch and `internal/test/runner` parser/execution entries, including ordinary/orchestrated peer consumption | P-1 UnknownSuiteVerbRefused, EngineStepUnknownKindRefused, UnusedPeerBlockRefused and existing CIMustFailFixturesAllFail. Update `docs/architecture/testing/ci-format.md` and runner architecture before relying on these verdicts |
| 2. Editor lifetime/options — 2, 40 | `internal/component/cli/testing/{runner,parser,headless}.go`, model runtime/tick producers under `internal/component/cli`, `internal/test/cli/cmd_editor.go`; proposed `internal/le/mock/editorcase/` registration plus generated composition import | P-2 ETOptionsReachExecution and ETTimeoutJoinsWorker; P-40 editor placement/exclusion. Add child IPC/Wait/cleanup and real-time expiry cases; extend focused clock tests without replacing authored real-time behavior. Update runner architecture, ci-format and `docs/functional-tests.md` |
| 3. Shared process/store lifetime — 3, 11, 13, 33 | `internal/test/runner` runTest/runOrchestrated and capture; `internal/le/interoplab` Suite.Run; `pkg/plugin/sdk/union.go`; reached persistent fixtures in `internal/test/fixture` and named test carriers | P-3 cleanup/linger/reap, P-11 wait-file and BFD restart, P-13 pipe/union, P-33 effective-store/seeder/LG certificate proofs. Keep one Wait owner and immutable union construction. Update runner architecture, process-protocol, fixture store guidance |
| 4. Browser lifetime/journey — 2, 19, 35, 40 | `internal/component/web/testing/{runner,parser,expect}.go`, `internal/test/cli/cmd_web.go`, `internal/le/site/terminaldemo` validation/recording consumers; proposed shared journey carrier `test/web/recorded-config-save.wb` | FirstOpenHasOneBoundedContract and P-40 web placement/exclusion; login/edit/save/diff/confirm/readback plus settling. Prove startup deadline, factor once, remaining-budget propagation, explicit session precedence, close errors and joined owned commands/server. Update runner architecture, functional tests and terminal-demo validation description |
| 5. Approval, review and Git identity — 4, 5, 6, 17 paths, 29, 30, 31 | `internal/le/rfc/approve.go` and action grammar; `internal/le/test/weakened` shared table/carrier readers; `internal/le/commit/{actions,input,prepare,rfcchange,owed_trailers,review,snapshot,script}.go`, hunks; `internal/le/spec/review.go` recorder | P-5/P-6 publication failure/retry/removal, P-17 quoted paths, P-29 prospective tree/CAS, P-30 targeted late publication and P-31 canonical identity. Coordinate semantic-approval owner; preserve non-Go/deleted/semantic readers and owner-ruling behavior. Update committing, spec-workflow, RFC implementation/conformance guides and registered help |
| 6. Owned platform work — 7, 10, 18, 36 | Existing reclaim/verifydispatch, job admission, QEMU/deployment and `internal/le/setup/docker/kernel` producers named by D; all six l2tp-ppp/gokrazy-l2tp/l2tp-scale/hugepages/docker-lab/yangcontract routes | P-7 dispatcher fixture, P-10 cache/KVM/mount/cwd/probe, P-18 completed-step/second-interrupt/TERM-ignore and six-phase cancellation, P-36 foreign sentinel/guest identity. Retain terminal-safe installer distinction; no background group for terminal input. Update running-commands, runner architecture, QEMU and installer pages |
| 7. Hook and spec consumers — 8, 15, 16, 22, 24 | `internal/le/hookruntime`, `internal/le/spec` status/template and session-state consumers, `internal/test/fixture` seeder; canonical rule sources only when approved implementation reaches them | Existing P-8 category/panic/subject/freshness/daemon/startup probes, P-15 direct-edit WIP and selector, P-16 positive criteria/seeder/delegated rename, P-22 heading and P-24 non-Go speaker census. Update spec-workflow, functional tests and canonical agent guidance; do not revive retired prose-sensitive gates. June inventory closure remains blocked |
| 8. RFC accounting/recording — 9, 23, 25, 39, 41 | `internal/le/rfc` level/not-enrolled, approval and recording paths; `internal/le/commit/{attest,push,prepare}.go`, `internal/le/spec/review.go` and `internal/le/le/human`; `rfc/short/rfc8247.md`, extraction/discrimination and existing `internal/component/ike/engine/rfc7427_method_test.go`; committed RFC7084 text | P-9 terminal/no-write proofs for all four issuance routes, including review authorization below/above cap; ordinary judge/review/consumer and prepared push-script noninteractive controls. P-9 level/metadata, P-23 writer/reader, P-25 dispositions, P-39 explicit-kernel and P-41 text proofs remain. Update RFC/committing/spec-workflow/running-commands guides and authorization help; no enrollment or authority redesign |
| 9. Interop and generated peers — 12, 17 reload, 28 | `internal/le/interoplab/bgp` checker predicates and scenario entries, IPsec logger environment and Bench.Run staging; `internal/test/peer`; named five RTR fixtures and fifteen scenarios | P-12 per-checker live discrimination, RTR route assertions and staging/log proofs; P-28 real OPEN matrix; bounded GoBGP stable-session non-ADD-PATH candidate only. Retain original four-peer artifact blocker for wider scope. Update interop/functional docs and affected RFC proof records |
| 10. Committed inventories and extraction — 17 CodeQL, 32 | Proposed `internal/le/repo/inventory/` and `internal/le/repo/codeql/`; reuse/extract roadmap object reads and declarations AST resolution; `internal/le/plugin/coupling/scan.go`, site inventory, `.github/workflows/codeql.yml`, featuretags generator | P-32 complete committed population/refusal tests; P-17 derived inputs/dependency guard and future CI extraction sentinels. No compile/export/copy to read inventory. Update feature-gates/plugin inventory design and CodeQL workflow documentation; refresh generated inventory pages only after producer proof |
| 11. Publication, fixture and deterministic views — 13, 17 theme, 19, 26 named fixtures, 34, 38 | `internal/le/site/{demo,build}.go`, terminaldemo/chaos, DNS and action/manifest/home/lab-pin fixtures; `docs/features/ai-first.md`, help_ai.go printSummary/Recipes/CommonErrors and `features/ai-first-design.md` evidence | P-13 clock/poster, P-17 theme, P-19 generator/discovery/workflow, P-26 fixtures, P-34 contracts/DNS and H's completed source-review corrections, P-38 replacement/verified-byte proofs. Apply prose and help-producer corrections, validate selected-path claims, then truthful review date; no date-only refresh. Independent DNS judge remains owed |
| 12. Remaining tool inputs/reports — 4 launcher/watchdog/stress, 14, 20, 21, 37 | Launcher/helper selection and `internal/le/test/stressrepro/run.go`; existing toolchain/vendor/schema registration and lint input guard; `internal/perf/report`, `internal/perf/cli/cmd_report.go`, reached site convergence readers | P-4 exact argv/fresh entry, P-14 owned patches/schema/l2tp census, P-20 pass identity/empty refusal/loss before dedup across renderers, P-21 filename collisions and P-37 actual lint-input distinction. Update running-commands, performance/benchmarking and exact schema/vendor pages; finish consumer census before declaring publication proof complete |
| 13. Incident proof closure — 16 census, 26 aggregate, 27, 35 | Recovered evidence companion and original incident producers; no speculative generic integrity repair | Recover named originals or attributable recurrence for AC-27; reconcile every remaining June/lint/load member. Execute only the identified affected cases once cause is known. Keep unresolved obligations open; historical greens or current gate passes are not substitutes |

**Phase 7 current work is complete:** implementation, scoped independent review
and real-entry proof are complete, as recorded in the
[current hook evidence](tooling-one-off-evidence.md#contemporary-review-follow-up-not-recovered-historical-evidence).
Missing June technique/fuzz inventories and authentic October originals remain
Phase 13 obligations. This records Phase 7 completion only, not whole-spec or
journal closure.

Phases 1–12 contain implementable contracts and named proof debt; their exact
external dependencies stay explicit rather than blocking all useful work or being
silently omitted. Phase 13 cannot close without the missing evidence. A newly
found reader of a changed contract joins its owning phase and proof population.
The reached report/publication proofs and remaining historical carrier mappings
are completion prerequisites, not permission to certify only the named examples.

Run scoped package/tag tests and named functional selectors directly in main,
one CPU-intensive job at a time. No full suite, whole-tree compile, export build,
or `le verify` command is authorized. If another session breaks the needed source,
identify and coordinate that conflict instead of testing a copy. For each P-number
retain command, content identity, attributed red, repair, restored green, skips
and real-entry assertion. RFC records use the native recorder and independent
judgment. Parent owns commits and independent review of the full occurrence map;
future CI supplies extraction/release evidence. No journal row closes from this
design document alone.

### Integration Checklist

These are design applicability answers, not completed implementation checks.
They restore all twelve questions from `plan/templates/TEMPLATE.md`.

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new product RPC or config leaf is proposed. Changes enter existing le actions, fixture formats and report readers; the system door preserves the existing YANG authority |
| YANG validation constraints | N-A | No new YANG leaf. `.et`/`.wb` option validation is a parser contract under P-2/P-40, not a new YANG constraint |
| YANG custom validators | N-A | No new `ze:validate` function or dynamic product value is proposed |
| CLI commands/flags | Yes | `internal/le/commit/actions.go`, RFC actions, reclamation and test selectors: bind approved publication/placement forms to their existing registered entries and update help. P-4/P-15/P-30/P-39 prove reachability |
| CLI grammar (keyword before value) | Yes | New/changed le action forms must follow `ai/rules/cli.md`; preserve legacy forwarded perf flags supported by `internal/perf/cli/cmd_report.go`, rather than silently renaming them |
| Editor autocomplete | No | No product editor config/command leaf changes. `.et` dimensions/waits are harness options, not operator autocomplete values |
| Functional test for new RPC/API | N-A | No new public RPC/API; real tool-entry proofs remain required in the carrier table, including seeder and commit consumers |
| Pipe completeness | Yes | Approval/review/refusal, cleanup and loss answers must agree in text and structured output. P-3/P-20/P-29/P-30 reach these contracts; use shared command rendering where the action already uses it |
| Env var registration | Yes | Audit existing logger key and dotted/uppercase root aliases in P-12/P-33 and current mode readers in P-15. Reuse registered keys; no unregistered timeout or isolation knob is proposed |
| Doctor check for runtime dependencies | No | Placement uses existing Ze kernel/QEMU and capability machinery; web uses existing browser dependency and refusal. No new service, binary or socket dependency is proposed. A new dependency discovered during the blocking-call/placement audit must return here with its owning diagnostic |
| Prometheus counters/metrics | No | Loss and cleanup are existing report data, not new daemon metrics; GoBGP counter observations do not add Ze telemetry |
| BGP family surface (new SAFI / capability / attribute) | N-A | P-28 fixes generated test-peer attributes for already-supported families and session kinds; no new family/capability/attribute registration is proposed |

### Documentation Update Checklist

All seventeen template questions follow. Yes means an implementation obligation,
not that a page was edited here. Source-anchor search for the known reached files
found `docs/functional-tests.md`, runner architecture, RFC gates, committing,
spec workflow, RFC implementation guide and `docs/features/ai-first.md` references.
The full source-anchor/Design-header census must be refreshed against the final
approved edit population; the incomplete producer census cannot yield a final
claim of no other affected page. Existing stale export/verify instructions in
those pages do not authorize that execution route.

| # | Question | Applies? | File to update / source-aware reason |
|---|----------|----------|-------------------------------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/ai-first-design.md` Doc evidence (P-34); each reached feature declaration whose checker proof changes must be rejudged. `docs/features/ai-first.md` anchors commit actions. No source repair alone raises maturity |
| 2 | Config syntax changed? | No | Candidate changes test options and tool inputs, not product YANG. Runner architecture separates `.ci`/`.et`/`.wb` from product config; `docs/guide/configuration.md` and `docs/architecture/config/syntax.md` need no new syntax from the currently proposed changes |
| 3 | CLI command added/changed? | Yes | `docs/contributing/committing.md`: late approval and review content/identity; `docs/contributing/rfc-conformance-gates.md`: approved kernel-placement form; `docs/contributing/running-commands.md`: reclaim and execution constraints. Check `docs/guide/command-reference.md` and registered help for every reached public spelling |
| 4 | API/RPC added/changed? | No | No new product handler, event or RPC payload. P-3/P-20 structured tool reports are documented with their tool producers; `docs/architecture/api/commands.md` remains the product registry contract |
| 5 | Plugin added/changed? | No | No plugin registration or lifecycle change selected. RPKI/GoBGP scenarios exercise existing plugins and peer behavior; `docs/guide/plugins.md` needs change only if producer verification exposes a product contract change, which returns to design |
| 6 | Has a user guide page? | Yes | `docs/guide/benchmarking.md` and `docs/performance.md`: explicit loss refusal, pass-aware identity and invalid convergence; `docs/contributing/running-commands.md`: owned cancellation/admission. Generated performance text comes from `PerformanceDoc`, not hand-edited output alone |
| 7 | Wire format changed? | No | P-28 restores existing generated UPDATE attribute rules; raw-hex input remains exact. Checker repairs change evidence, not a new product encoding format. Confirm existing `docs/architecture/wire/` claims against any producer actually changed before marking this final |
| 8 | Plugin SDK/protocol changed? | No | No new SDK message or field is proposed. `sysproc.go` declares `docs/architecture/api/process-protocol.md` as its Design page: any lifetime edit there updates that existing lifecycle description under row 12, without claiming a new SDK wire protocol |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | Reached `rfc/short/` claims, extraction/audit/discrimination records and derived `docs/features/rfc-status.md` must reflect actual new proof, including recorder placement. Preserve the eight-document profile limits and committed `rfc/full/rfc7084.txt`; do not enroll unlisted documents |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `docs/architecture/testing/runner-architecture.md`, `docs/architecture/testing/ci-format.md`, `docs/architecture/testing/interop.md`: adapter options/caps/groups, worker lifetimes, store roots, cleanup status, exact selectors and source-backed oracles |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` needs an accuracy check for any claims reached by checker repairs; `docs/performance.md` and `docs/guide/benchmarking.md` must explain loss refusal and pass identity. No new performance numbers are authorized on an unknown host |
| 12 | Internal architecture changed? | Yes | Runner architecture for context/group lifetime; `docs/architecture/api/process-protocol.md` if its declared `sysproc.go` mechanism changes; `docs/architecture/system-architecture.md` is the Design page of `cmd_report.go`. Also `docs/architecture/testing/verify-freshness-scope.md` and spec workflow for current main-only/scoped-evidence constraints |
| 13 | Route metadata keys added/changed? | No | No key declaration changes in this candidate; RPKI tests assert existing state and received attributes. The system/BGP doors require metadata ownership to stay in the plugin, so `docs/architecture/meta/README.md` needs no new key entry |
| 14 | Prometheus counters added/changed? | No | No new metric registration. Existing route-loss fields and peer counter observations are not Prometheus additions; no new `docs/plugin-development/metrics.md` entry is proposed |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | P-32 changes source/platform inventory judgment; P-34 restores manifest-driven consumers. Recheck generated `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` against the approved committed-population producer; update command discovery/help for changed tool forms, not a second manual registry |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Observed anchors include `cmd_editor.go`/`cmd_web.go` in functional tests and runner architecture; `discriminate_guest.go` in RFC gates; commit review/actions/owed consumers in committing, spec workflow, RFC implementation guide and ai-first. Preserve all those pages in the implementation edit census. Final complete derivation remains open until producer/file census is complete; `cmd_report.go`'s Design page is explicitly retained in row 12 |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Check committing create/append/remove/approval examples, RFC record kernel examples, functional selector and stress-repro examples, runner `.et`/`.wb` options, and benchmarking compare flags against actual parsers/actions. Correct no-verify/main-only guidance in `docs/contributing/running-commands.md`, `docs/contributing/spec-workflow.md`, hook instructions and reached agent discovery sources; parent owns rule synchronization |

### Accounting result

All 138 original ownership-index entries have a stable class/date/surface mapping
to 41 corrected ACs, P-number proof obligations and owners A–I. The two same-day debt
occurrences, two September 5 same-tag occurrences, two sweeper halves, four lost
hook categories, six cancellation routes, eight RFC documents, fifteen checkers,
three September 17 fixtures and four recorded-fixed entries remain visible. This
is complete index accounting, not a completed semantic census or a clean audit.
June backfills, aggregate logs, replacement coverage,
the named proof/carrier mappings and AC-27 attribution remain open. The selected
contracts passed the final owner approval gate recorded above. The one-off
owner clarification retains owner-ruling interpretation and adds the terminal-only
authorization boundary through the existing producers identified above. No new
owner design decision remains for starting the approved implementation phases.
H's source review is
performed but prose/help corrections remain outstanding. Implementation is
approved, starting with phase 1; global completion remains blocked by the named
historical evidence and outstanding implementation/proof obligations.

### AC-26 / AC-3: runner-owned port input for private daemon fixtures

This is an appended test-tooling fixture input repair within the existing
AC-26 le-fixture obligation and AC-3 port-lifetime obligation, not a new WIP
claim or the separate CLI-history product change. No status, phase or other
acceptance criterion changes.

| Item | Design and evidence |
|------|---------------------|
| Observed blocker | Loaded invocation59 did not reach the unchanged shape-refusal assertions: `tmp/stress-repro/plugin-draft-plugin-shape-declaration-refused-47725c220e9408ffbd22ff8fca03e1fb5cd5cd693fef78a64fc3f6a9f95b3dc0-20261104-190926-3924740863.log:604` reports the daemon bind failure at `127.0.0.1:46760` |
| Producer and owning layer | `startFixtureDaemon` in `internal/test/fixture/plugin_fixture_11_alias.go` derived its BGP port as 12000 plus PID modulo 40000, bypassing the runner lease and including Linux outbound ephemeral ports; the owner is fixture input plumbing, not plugin shape behavior |
| Rejected workaround | Retrying startup, selecting another PID-derived port, changing a deadline or weakening refusal assertions hides the unleased-port input rather than repairing its producer |
| Source repair | Pass fixture args through `aliasDriver`, `commandCatalogPluginShape`, `configArchivePrune`, `localCommandDaemonSSH` and `pipeAliasCompletion` to `startFixtureDaemon`; reuse `parsePort(args, 0)` in `netfilter_fixture_flow_export.go`, accepting actual ports 1 through 65535 before launching, then set existing `envTestBGPPort` to that port |
| Carrier census | Draft plugin carriers: `command-catalog-plugin-shape`, `plugin-pipe-alias`, `plugin-pipe-alias-help`, `plugin-pipe-alias-namespaced`, `plugin-pipe-alias-collision`, `plugin-shape-declaration-refused`; draft UI carriers: `config-archive-prune`, `cli-local-command-daemon-ssh`, `plugin-pipe-alias-completion`. Add only existing `$PORT` to each fixture invocation. Preserve the shape draft's independently approved obsolete doc/map/tab wording removal |
| Reachability discovery | Search the bare `startFixtureDaemon(` token across `internal/test/fixture`: one definition and the five caller sites above. The two UI callers register their context-only fixture functions through `registerFixture`; migrate those functions to the supported context-plus-args pattern, preserving their contexts and all SSH completion/execution checks |
| Lifetime producer | `runTest` in `internal/test/runner/runner_exec.go` acquires `LeaseTestPorts` before expansion, stores the actual lease start in `rec.Port` and defers release until fixture execution returns. `startFixtureDaemon`'s Wait worker closes `done` after Wait, and `fixtureDaemon.stop` waits for `done` before returning, including its kill path |
| Preservation boundary | Keep contexts, callbacks, daemon cleanup, readiness and refusal/output checks, previous assertions and authored timeouts unchanged. No fallback, PID-selected port, retry, sleep, deadline change, new setting, validation layer, allocator, global lock or production behavior change |
| Documentation | Extend only the existing port-lease description in `docs/architecture/testing/runner-architecture.md` to explain fixture argument delivery and runner ownership through daemon stop/reap |

| Existing AC | Positive acceptance and permitted survivors |
|-------------|--------------------------------------------|
| AC-26 | Each named fixture starts its real daemon using the runner's actual leased port and reaches its original useful-output/refusal/archive/SSH completion and execution assertions under scoped smoke and 80-iteration draft contention proof. Valid plugin declarations and existing aliases continue answering; invalid shape/collision declarations remain refused because those are the behaviors under test. SSH still offers the declared alias beside built-ins and local-data commands still enforce daemon path confinement. No assertion is removed or narrowed |
| AC-3 | Runner ownership of the supplied port lasts until fixture completion, after daemon stop/reap. Existing bounded readiness and teardown remain permitted unchanged because this repair changes only port input, not process-lifetime policy |

Parent owns independent test judgment where needed, scoped draft smoke,
80-iteration `any-failure` stress proof, promotion and live-suite evidence.
The design record above did not claim execution. The following table records
the subsequently observed scoped results; no full-CI verdict is inferred.

#### Executed leased-port evidence

| Scope | Observed result | Evidence |
|-------|-----------------|----------|
| Six plugin draft carriers above | All 80 loaded invocations completed with exit 0; one burner and two concurrent invocations | `tmp/session/2026-11-03-231620/scratch/ci-leased-port-plugin80-evidence.json` and its native capture |
| Three UI draft carriers above | All 80 loaded invocations completed with exit 0; one burner and two concurrent invocations | `tmp/session/2026-11-03-231620/scratch/ci-leased-port-ui80-evidence.json` and its native capture |
| Promoted plugin entry points | All six named live cases passed with their original expectations | `tmp/session/2026-11-03-231620/scratch/job-ci-leased-port-plugin-live-c32d4069.log` |
| Promoted UI entry points | All three named live cases passed, including archive retention, SSH completion and path confinement | `tmp/session/2026-11-03-231620/scratch/job-ci-leased-port-ui-live-1b032926.log` |
| Independent input judgment | JoinJudge approved all nine canonical units after comparing live, draft and HEAD plus the five actual callers | The implementation commit carries the native `Weakening-approved` verdicts |
