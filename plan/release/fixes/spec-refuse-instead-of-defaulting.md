# Spec: code that cannot answer refuses instead of returning a plausible value

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P1 |
| Release reason | Shipped producers answer a default, a zero or a constant where they cannot answer, so operators and peers act on values Ze never measured; several rows are RFC defects in implemented capabilities (RADIUS, LDP, IS-IS, ADD-PATH, LLGR) |
| Depends | spec-plugin-optional-dependency-refusal (closed; its refusal is `config.refuseAbsentOptionalProviders` and `registry.MissingDependencyError`, documented in `docs/architecture/plugin/feature-gates.md`; it owned the "not built reads as not configured" rows of the 2026-10-20 dependency census; this spec reuses its refusal for the two build-gated rows it owns) |
| Phase | 7/7 |
| Handoff | verify |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Remaining verification

Phase 7 is in its closing steps. Official round 2 is CLEAN, bounded round 3
over the lint-fix diff is CLEAN, and bounded round 4 over the test fix is
CLEAN (Review Gate). Two Go commits landed after round 2, both carrying
`Spec-work: refuse-instead-of-defaulting`:

| Commit | What it carries |
|--------|-----------------|
| `14d0fa156d` "refusal: fix the nine owned scoped-lint findings" | The nine owned findings of the first scoped lint ("Scoped lint result" below) |
| `ad05166600` "refusal: check the withdrawal builder's result where a test measures it" | The tenth owned finding, which the scoped lint rerun exposed (same section) |

The runs owed after the lint fix are done. Logs are in
`tmp/session/2026-11-12-014013/scratch/`:

| Run | Result |
|-----|--------|
| Scoped lint rerun | Run by the owner on his Mac on a still tree, twice (`refusal-lint-mac-ef1563f19f.md`). No finding remains on a line this spec wrote or caused |
| `system-date-ntp-forked` | PASS 1/1 on a build of the current tree (`job-refusal-ntp-forked-after-lint-042b8433.log`) |
| `./le doc check all` | Source-anchor step passes. Two steps fail, neither this spec's ("Documentation Updates") |

`./le repo compiles check` is not run, and it is not owed. A repo-wide compile
is a whole-tree check, which no agent starts, at closure included
(`ai/META-DECISIONS.md`, "How agents work"), and GitHub CI verifies the pushed
commit. The scoped evidence stands in its place. The changed packages
type-check with their tests (`job-vet-pkg-7dd5a45b.log`), and their package
tests pass. Both lint runs on the owner's Mac type-checked the code: at
`ef1563f19f`, every build of the 13 packages, the compile-out build included;
at `ad05166600`, the reactor package. Main runs `./le spec review record` for the
committing session (Review Gate, "Population for the committing session"),
commits A and B, and `./le spec citation`. Each earlier checkpoint below is
history.

## Scoped lint result, 2026-11-12

The run is
`tmp/session/2026-11-12-014013/scratch/job-refusal-final-scoped-lint-86c7c325.log`,
over `./cmd/ze/hub/... ./internal/component/cmd/show/... ./internal/component/plugin/registry/... ./internal/le/arch/refusebydefault/... ./internal/le/rfc/... ./internal/le/interoplab/bgp/... ./internal/component/bgp/reactor/... ./internal/component/bgp/plugins/rib/... ./internal/component/bgp/plugins/bmp/... ./internal/test/fixture/... ./internal/component/config/cli/... ./internal/plugins/iface/netlink/... ./internal/plugins/ntp/...`.

| Build | Result | Owned |
|-------|--------|-------|
| Host, Linux integration | 134 findings each, the same list | 8. Ownership by `git blame` of each flagged line is in `scratch/refusal-lint-ownership.tsv` (same directory): 8 owned, 105 written by other sessions' commits, 21 in other sessions' untracked files |
| Darwin, capability, appliance | Stopped on a typecheck error: `HandlerPanicsDropped` in `internal/component/plugin/doctor/checks_delivery.go` and `internal/component/plugin/server/doctor_snapshot.go`, another session's half-written doctor edit | 0 |
| Compile-out | That same error, plus `TestBuildTag_NTP_AbsentSyncInfoNil` in `cmd/ze/hub/build_tag_ntp_absent_test.go`, which stopped compiling when `2ea5329344` gave `pluginreg.GetNTPSyncInfo` a `CommandQuery` argument | 1: that `!ze_ntp` test proved nothing from `2ea5329344` on |

| Owned finding | Fix |
|---------------|-----|
| `nilnil`, `internal/plugins/ntp/register.go` `ntpReplyMetadata` | Split into `ntpReplyEnabled` (bool, error) and `ntpActiveMetadata`. `ntpSyncInfo` keeps the `NTPSyncProvider` contract: nil metadata with no refusal means an explicit inactive answer. `forwardNTPCommand` reads `enabled` itself before `ntpActiveMetadata` |
| `goconst` `count`, `register_commands.go`; `paramTypeCombine` in `executeNTPCommand` | `fieldPeers` and `fieldPeerCount` constants; parameters combined |
| `stringXbytes`, `command_active_config_test.go`; `rangeValCopy`, `startup_cleanup_test.go` | `bytes.Equal`; index the stack records |
| `errorlint` and `goconst` `source`, `internal/test/fixture/register_ntp_forked_state.go` | Wrap the last error with `%w` only when there is one; `ntpForkedSourceField` constant |
| `misspell`, `internal/component/bgp/plugins/rib/rib_pipeline_refusal_test.go` | `unrecognized-stage`; the asserted refusal text is unchanged in form |
| Compile-out, `cmd/ze/hub/build_tag_ntp_absent_test.go` | Passes a query that would answer inactive, and asserts the seam reads nothing and refuses when no provider is registered |
| Unused `jsonKeyError`, `internal/component/bgp/plugins/rib/rib.go`, orphaned by `820476c6ee` | Removed; nothing else in the package names it |

Main's evidence for the fixes, logs in the same scratch directory: NTP and
`cmd/show` pass under race (`job-unit-pkg-ba291acb.log`). In that log the RIB
package fails once on `TestSDKEventBusWholeFamilyFrames/best-change` (context
deadline at load average 32 to 45), and the hub package did not build because
of another session's `internal/plugins/ddos/flowspec` edit. The RIB rerun passes
(`job-unit-pkg-1cf6d23e.log`, which shows only the package `ok` line). The three
`TestBuildTag_NTP_Absent*` tests pass by name in the build without `ze_ntp`
(`job-unit-pkg-ntp-absent-7525ec60.log`). The six changed packages type-check
(`job-vet-pkg-7dd5a45b.log`, empty output). `internal/test/fixture` and
`internal/le/rfc` pass in `job-unit-pkg-e6c8cc4c.log`; their earlier failures in
`job-unit-pkg-c4a36b2f.log` were `exec: "le": executable file not found in $PATH`.

`TestInfraSSHOwnerDrainsBeforeStoreCloseOnStartupFailure`
(`cmd/ze/hub/infra_history_owner_test.go`) fails twice the same way in the
default build (`job-unit-pkg-c4a36b2f.log`, `job-unit-pkg-e6c8cc4c.log`): no
history publication before store Close after the SSH server's 5 s storage-write
drain. This spec's change does not reach it. The lint-fix hub change is in a
`!ze_ntp` file the default build excludes. This spec's committed hub changes
(`238bdb62e9`: `buildParamMeta` in `command_meta.go`, the web config path in
`service_web.go`; `cb44713011`: the two NTP build-tag tests) are not on the SSH
editor history path, and `238bdb62e9` predates the test file's last change
(`672832444f`, 2026-11-05). The candidates are foreign: `d7b8d92073` changed
`internal/component/ssh/ssh.go`, `cmd/ze/hub/audit.go` and
`cmd/ze/hub/main_reload.go` on 2026-11-12, and other sessions hold uncommitted
edits in `internal/component/plugin/server`. No before/after run in main proves either
side; that is the comparison that would.

The lint closure criterion is zero owned findings with each foreign finding
attributed, the same criterion the repository check (31 foreign) and the RFC
check (73 foreign) closed on. No acceptance criterion, Goal Gate or closure
checklist item here asks for zero lint findings in these packages whatever
their author.

At 03:49 `internal/component/plugin/server/enricher.go` (another session's
uncommitted edit) did not compile (`job-unit-pkg-34a32afe.log`), so the lint
rerun and the NTP rerun waited for its owner.

### Rerun after the lint fix

The fix landed as `14d0fa156d`. The scoped lint rerun could not produce a plan
in this checkout: five attempts after the fix were refused in planning because
an input they reached changed during planning, four times by other sessions'
edits and once by this session's own agent (the last is
`job-refusal-reactor-lint-241f07a4.log`). The owner ran the lint on his Mac on
a tree no session was writing to. The evidence record is
`refusal-lint-mac-ef1563f19f.md` in the same scratch directory, and it holds
both runs.

| Run | Scope | Host | Linux integration | Capability | Appliance | Compile-out |
|-----|-------|------|-------------------|------------|-----------|-------------|
| 1, at `ef1563f19f` | The 13 packages above | 24 | 25 | 1 | 0 | 0 |
| 2, at `ad05166600` | `./internal/component/bgp/reactor/...` | 0 | 0 | 0 | not in the plan | not in the plan |

Run 1 shows no finding in any file `14d0fa156d` changed. The compile-out build,
which did not compile `cmd/ze/hub/build_tag_ntp_absent_test.go` in the first
run, reports 0. No build reports `nilnil`, which settles the round-3 NOTE on
`ntpSyncInfo`.

Run 1 exposed one more finding of this spec: an errcheck finding in
`internal/component/bgp/reactor/forward_build_test.go`, on the
allocation-measuring call of `TestFwdWithdrawalLabelledCompatibility`.
`git blame` puts the line on `c972e77416` (2026-10-21), so the first ownership
pass (`refusal-lint-ownership.tsv`, by blame) called it foreign. The finding
exists only because this spec's `a7afabc1be` made `buildWithdrawalPayload`
return `(int, error)`, and the measured call discarded both results (Mistake
Log). `ad05166600` keeps the length and the error outside the measured closure
and checks them after it. The test passes under race
(`job-unit-pkg-a103619d.log`, which shows only the package `ok` line), and an
independent bounded review of the diff is CLEAN (Review Gate, round 4). The
capability build's one finding was the same line.

The other four non-fixture findings were traced by cause, not by blame, and
are not this spec's: three in `internal/component/plugin/registry/seal_test.go`,
written by `ae1aac228b` itself (`Spec-work: inventory-fast-registry-checks`),
and `migrateConfig` in `internal/component/config/cli/validate_corpus_test.go`,
orphaned by `ba7c8819df`. The remaining findings are in fixture files under
`internal/test/fixture/` that other specs own: four files committed in
`4c05449c18`, and three in lint-backlog-3's scope. The evidence record
attributes each one.

No finding remains on a line this spec wrote or caused. That meets the lint
closure criterion stated above.

Fresh main checks, 2026-11-11: repository check reports 31 foreign findings, all outside
the complete 846-path review population; weakened-test check reports 21 foreign
deletions in the concurrent MPLS test move. Citation check passes for 666 specs
with 65 baselined dangling citations and 15 foreign line-token warnings. The
retained logs are `job-refusal-final-repo-check-f6785901.log`,
`job-refusal-final-weakened-check-b6513fa8.log`, and
`job-refusal-final-scoped-source-checks-5fb20925.log` under this session's scratch.
Main's bounded `ls` confirms all three checker files, absent-CoS integration,
the ADD-PATH/NTP fixtures, all three named canonical tests and the FRR scenario
with both configuration files. The native review check still passes 591 files.

## Completed runtime checkpoint

BMP's stored AS_PATH and missing-next-hop repairs are committed in
`4a991e513e`; all 268 BMP race roots pass in
`job-refusal-bmp-stored-green-dc666453.log`.

The later runtime audit found that a forked BMP plugin could accept Loc-RIB
without its event bus. Startup and config verification now refuse that request
before configuration effects, naming the cause and internal-plugin fix. The
two missing-bus regressions first failed while useful available-bus controls
passed. The full retry then exposed one old disabling-reload fixture that
started with Loc-RIB enabled but supplied no bus. Its independently approved
correction preserves every assertion and passes the exact root in
`job-refusal-bmp-locrib-off-fixture-green-cb55598c.log`. Runtime admission and
the fixture correction are committed in `dfc2863cea`; all 273 BMP roots pass
under race with zero skips.

NTP CLI wiring is committed in `e62bc0e2a1`. The actual external baseline
reproduced `enabled=false` in `job-refusal-ntp-wired-baseline-a319bf40.log`.
The state-owner/query transport repair is committed in `2ea5329344`; all 383
roots across four packages pass under race with zero skips in
`job-refusal-ntp-boundary-race-9f5c7d40.log`, and independent bounded review is
CLEAN. Preparation then refused the final-save warning without a doctor code
and three SDK error returns without `opererr` recovery. The reviewed recovery
and cleanup correction is included in that commit. Main observed the callback failures in
`job-refusal-ntp-runtime-recovery-red-b32a7ded.log` and a real configured
worker leak in `job-refusal-ntp-startup-cleanup-red-82f32703.log`; all three
focused roots pass under race in
`job-refusal-ntp-runtime-recovery-green-4ebbb0be.log`. The final NTP package
passes all 71 roots under race with zero skips in 3.763 seconds in
`job-refusal-ntp-final-package-and-live-71bfbb66.log`. The same run's actual
external `system-date-ntp-forked` test passes in 1.1 seconds. The state, recovery
and lifetime repair is committed in `2ea5329344`.

An earlier live attempt in
`job-refusal-runtime-live-after-fib-doctor-9bdfbf35.log` failed first-party
compilation on the then undefined `withoutReaderData` at
`internal/le/hookruntime/bash.go`. The helper is now present. That attempt
reached no behavioral assertions. The
earlier FIB/VPP failure remains historical attempted-run evidence. Later
current-source builds and the NTP live pass supersede those build blockers.
The actual internal BMP collector proof passes 1/1 in
`job-refusal-bmp-internal-locrib-live-d641eed6.log`. The corrected external BMP
draft passes 1/1 in `job-refusal-bmp-external-after-crash-api-dfbdeee1.log`:
the daemon remains reachable, the refused process is absent, and its exact
cause and recovery remain asserted. Independent test judgment corrected the
old daemon-exit expectation against the nonfatal plugin policy and placed
each stream assertion under its producing command. The subsequent load run
completed 80 invocations without reproduction in
`job-refusal-runtime-drafts-stress-after-fixture-fix-d086478f.log`; exit 1 is
the reproducer's expected “not reproduced” result, and invocation 80 exits zero.
Full capture is
`tmp/stress-repro/plugin-draft-system-date-ntp-forked-bmp-locrib-external-refuses-66fc53f5c9d3446251bd5e7d387b20695a2e97516ec3ef16a3e7d7fc4ba29b12-20261111-210924-3167250360.log`.
Main promoted both fixtures without content changes to
`test/plugin/system-date-ntp-forked.ci` (SHA256
`5b8b621d54c4355dedfcdee089f4c2681b66cd7329a3b9859f24d465f721d901`)
and `test/plugin/bmp-locrib-external-refuses.ci` (SHA256
`c779732d3b49c33752003e4f37a0fe92483412bf083030c1c27888bba224f80d`).
Their selected canonical run passes 2/2 in
`job-refusal-promoted-runtime-fixtures-027d40a9.log`; unchanged promotion adds no
source-review round. The native review artifact includes both canonical paths.

Checker precision is committed in `60125ba1fb`. It recognizes only a direct
canonical `plugin.OperatorErrorResponse` refusal, including import aliases;
shadowed imports, impostors and successful responses remain findings. The
earlier regression red is `job-refusal-operator-response-red-a1ab3e18.log`.
The full checker race run passes all 13 roots with zero skips and reports
88.1% statement coverage; this is not a 100% claim.

The final named AC-7 report is
`job-refusal-final-named-report-45a00bc9.log`: 2207 refused and 72 annotated
shapes in 7109 files. Main validated all 54 rows and 125 actual producer
entries, including the exact AST anchors for BMP configure/verify rather than
their sibling callbacks. Both report populations join to zero owned hits in
`scratch/refusal-ac7-final-125-comparison.json`. The prior six-hit
54-row/124-entry comparison is superseded by this result.

Withdrawal error propagation is committed in `2558080335`. Its new source guard
first failed in `job-refusal-withdrawal-adapter-red-78d9ded2.log`; 32 selected
race roots pass in `job-refusal-withdrawal-adapter-green-93827d21.log`. Both real
forward callers now receive the builder's reason without losing buffer or
source ownership. The fresh native AC-7 report in
`job-refusal-ac7-clean-report-fcb16ac0.log` lists 2200 refused and 71 annotated
shapes across 7050 files. Main joined both populations against all 54 census
rows and 97 producing declarations and found zero owned matches. That is the
static checkpoint before the BMP/NTP repairs. The final 54-row/125-entry
comparison above supersedes it and covers the expanded owner/query/worker chain.

The checker direct-assignment repair is committed in `06a4a22cbc`; full race
retry `job-unit-pkg-ef92ffff.log` passes. Package coverage is 86.5%, with eight
uncovered blocks retained. RFC framework/fixture repair `62f9ff2116` and the
selected 67-root retry pass are recorded below. The current owned RFC scope
contains 62 IDs in `scratch/refusal-rfc-runtime-62-ids.txt`. Exact unit/producer
attribution of the earlier 117 findings was 37 owned and 80 foreign; the
withdrawal caller repair changed forwarding producers beyond the earlier
56-ID census. `55d4cb0991` commits the four BMP native proof pairs, first
RFC9069-5.3-1 dependency proof, five semantic audits, 17 mechanical shifts and
the renewed RFC1997 unit binding. All ten remaining forwarding unit proof
renewals now succeed in `job-refusal-remaining-forward-unit-proofs-8dea3305.log`.
`85854bc757` commits the final seven proof-record files, including the
functional renewal. The first renewal
baseline failed to build on foreign IKE `checkIPsecPeerMalformedMessage` in
`job-refusal-runtime-rfc-record-77170cc5.log`; no invalid record was written.
The helper is now present and later completed runs supersede that failed
attempt. The RFC1997 functional proof is now valid in
`job-refusal-withdrawal-ci-proof-after-fixture-fix-de37abbd.log`: its clean
baseline passed before the intended negative. All 16 current native record
tuples are complete and committed. Main regenerated 231 RFC shards. Its final
native check reports 73 global findings; the individual source/history audit in
`scratch/refusal-final-rfc-individual-ownership.json` and `.md` assigns all 73
to foreign work, with zero refusal-owned findings. Same-ID RFC1997 and RFC7947
interop SHIFT rows were individually traced to foreign history; ID matching
alone did not assign them here. This is zero owned debt, not a global pass.

`7b2c596952` commits the four netlink embedded-literal lint repairs and five race
controls. Narrow checker lint passes HOST and Linux integration with zero
issues in `job-refusal-checker-scoped-lint-1359b2fb.log`; no other lint
configuration is inferred. Earlier broader planning attempts refused changing
foreign `server/register_plugin_restart.go` and another source. The latest
13-package attempt refused because `core/sysctl/known.go` changed during
planning (`job-refusal-final-scoped-lint-stable-inputs-8d7938b4.log`);
`refusal-final-lint-after-known-map-settled` is queued. The
earlier repeated-scope argv grammar failure (`f6f2827c`) ran no lint and is
not a code finding. Final broader lint remains owed. The current documentation
check has one foreign completion-catalogue description drift; its other steps
validate 447 commands and all source anchors. Main's mandatory repo and
weakened-test prechecks reported only foreign in-progress findings; fresh
repo/weakened/citation metadata checks are queued. The untracked canonical news
fragment is ready with all six required headings. The spec stays open despite
its passing native review artifact; no closing commit is available. `cc190d045b` retains
the preceding runtime checkpoint. The successful functional proof and load-80
run supersede attempts stopped by a foreign fake in-process fixture; its owner
added the missing CLI handler at 20:59, and those changes are preserved. A
direct BMP interop retry reached zero scenarios when Docker info timed out
(`job-refusal-bmp-collector-after-repair-632ff3c0.log`). The properly
Ze-Docker-host-wrapped retry passes `bgp-addpath-best-path-pmacct` 1/1 in
`job-refusal-bmp-collector-ze-docker-host-59be18bd.log`, with recording
`X6KQB2X5XBQFRHNP6ZP5YR44AH`. Main read the actual scenario PASS; check, command
and lab exits are zero, the accelerator is KVM and no leftovers remain. The
direct Docker timeout was not a product failure, and no bypass was used.
Access's first existing PPP acceptance scenario now passes; its remaining five
are running as one Ze-VM batch, currently building. They have no results yet.

The current execution evidence supersedes the earlier blocked attempts retained
under Prior blocker and completed corpus repair. Every run below was executed
by main against the main checkout. No whole-tree verification is owed here.

| Completed surface | Canonical source and observed evidence |
|-------------------|----------------------------------------|
| Full RIB package and fixture isolation | `startValidationReceiver` now unregisters after joining its owner (`1aa91d030b`). `TestValidationReceiverRestoresUngatedSelection` first failed both teardown assertions; focused native race selection then passed, followed by the full native `test unit pkg path ./internal/component/bgp/plugins/rib/` run in `job-unit-pkg-f5c56d84.log`. This reused path now contains the passing retry; retained reds are `job-unit-pkg-f0677e7f.log` (ASPA-retention/redist pair) and `job-unit-pkg-cf1bde44.log` (new regression). |
| Pipeline refusal | `showPipeline` and `bestPipeline` return `pipelineRefusal` through `refusePipelineArgs` (`820476c6ee`). `TestPipelineParseErrorsReachCommandVerdict` failed on successful error payloads, then passed with six controls under race (`job-refusal-pipeline-ntp-red-0f4a55fa.log`, `job-refusal-pipeline-green-191dfb31.log`). The rebuilt daemon's `test bgp plugin rib-filter-show` passes 1/1 in `job-refusal-rib-filter-functional-c6d5829a.log`. |
| NTP absent-provider refusal | Owner instruction (Thomas, 2026-11-11): "fix NTP". `handleShowSystemDate` refuses an absent NTP state provider, and preserves a registered inactive provider's date-only answer (`cb44713011`). Absent/inactive/measured controls pass under race in `job-ntp-provider-green-8e407612.log`; three actual `ze_core` NTP-absent hub tests pass in `job-ntp-provider-absent-fe550150.log`; actual daemon `system-date-show` passes 1/1 in `job-ntp-system-date-functional-a1dd9b1c.log`. |
| Config corpus | All four fixture drafts passed, were promoted and committed in `e7a77d6e73`; shared CPE rendering is `dd6864599a`. Native `test unit pkg path ./internal/component/config/cli/ run '^TestRepositoryConfigsValidate$'` exits zero with package `ok` in `job-unit-pkg-edd5d381.log`. Main observed the selected running test; default output carries no individual PASS name. |
| ADD-PATH, LDP, BMP and platform controls | The later evidence subsection records the completed ADD-PATH matched baseline/omitted-zero negative/unmodified clean, LDP/FRR, BMP/pmacct, gnmic, Linux controls, direct netlink negative/clean and actual-kernel root/non-root TCX proof. These runs supersede the old pending labels. |

All log basenames in this section are under
`tmp/session/2026-11-11-014000/scratch/`. The earlier launcher Help-length panic,
foreign hook-runtime build mismatch, inventory staging failure and startup
barrier failure were attempted runs that reached no assertions; later current-source
runs completed the affected proofs. They are retained as history, rather than
current blockers. The RIB isolation and focused green logs remain
`job-unit-pkg-77fea4ce.log` and `job-unit-pkg-59779559.log`; independent test
judgment and source review approved that fixture repair. The earlier ADD-PATH
startup-barrier failure reached no zero-ID assertion; its full verdict is in
`scratch/interop-recordings/bgp/addpath-zero-id-frr-GLEEENUKK5KX3YMO2VEZ3ROJRD.jsonl`
under this session. The later passing and negative runs below supersede it.

The independent RFC4271-5.1.5-2 audit found a weak negative enrollment. The new
`TestLocalPrefExternalWireViolationRefused` in
`internal/le/interoplab/bgp/check_local_pref_negative_test.go` accepts the
stripped capture and refuses leaked LOCAL_PREF values 200 and 0 at the wire
semantic assertion. The native recorder first observed all three clean
subcases pass. Its per-run semantic break then made both leaked-attribute
subcases fail while the stripped control passed; the discrimination is recorded
in `rfc/discrimination/rfc4271/RFC4271-5.1.5-2.json`. This describes the recorder's
baseline-then-break order, with no post-break clean rerun claimed. The native
recorder's classification of interop Go unit tests was repaired at `CarrierFor`;
`TestInteropCatalogUnitCarrier` and its two existing carrier controls pass in
`job-refusal-rfc-carrier-green-8f9cce4b.log`. These checker fixtures do not claim
a new daemon interop run. `62f9ff2116` commits the framework/fixture repair,
LOCAL_PREF final audit stamp, ASPA and the four owned SHIFT reseals, plus the
additional RFC7084-4.2-24 renewal. The SHIFT IDs are RFC5549-4-1, RFC5549-4-4,
RFC7911-2-2 and RFC8950-4-1. `a7afabc1be` commits the 16 RFC7311 renewals and
related approved caller migrations; they are no longer pending work.

The full RFC retry completed in 1168.248 seconds, within its 20-minute limit,
but three roots failed because their intended interop/nightly fixtures still
used Go unit-test filenames. The current catalog requires real `check_*.go`
checkers for that role. The retained failed run is
`job-refusal-rfc-full-retry1-d25f06c4.log`. The old timeout attempt is historical;
`job-unit-pkg-d25f06c4.log` was overwritten by the full retry and cannot supply
its original raw output. All other executed roots passed in that retry.

The two fixture-input repairs preserve all assertions, workflow declarations
and tags, with independent approval. Main's selected 67 roots across six files
all pass under race in `job-refusal-rfc-fixture-green-48640de7.log`, including
`TestARequirementProvenOnlyByNightlyEvidenceIsMarkedOnItsOwnRow`,
`TestRFCSelftestEveryStageContributesAResult` and
`TestRFCSelftestRealTreeRowReportsOwnedCheckFailures`, the three failed roots.
Eight CarrierFor controls and ordinary misnamed-unit refusal controls remain
covered. `ledgerFixtureAt` / `realLedgerRender` cache the immutable actual
checkout corpus once per test binary and refuse a different root; the helper
change has independent approval.

The RFC package coverage run reports 37.6% in
`job-refusal-rfc-coverage-84f2cf44.log`; `refusal-rfc-coverage.out` includes three
new defensive blocks that remain uncovered. The earlier producer-history audit
added RFC1997 withdrawal coverage to the prior 47-ID population, producing the
48-ID checkpoint. Its 60-finding global attribution and ownership disposition
are retained in
`scratch/refusal-rfc-ownership-final.md` and
`scratch/refusal-rfc-final-48-ids.txt`. The current owned union is 62 IDs in
`scratch/refusal-rfc-runtime-62-ids.txt`. BMP semantic rejudgments, the corrected
disabling fixture's proof and the RFC1997 unit renewal are committed in
`55d4cb0991`; all ten remaining forwarding unit renewals succeed and the final
seven record files are committed in `85854bc757`. The RFC1997 functional renewal succeeds in
`job-refusal-withdrawal-ci-proof-after-fixture-fix-de37abbd.log`; all 16 native
record tuples are complete. The final native RFC check has 73 global findings,
all foreign by individual source/history audit, with zero owned findings in
`scratch/refusal-final-rfc-individual-ownership.json`. Matching an ID alone does
not assign every finding under it to this spec. Round-2 native review check
passes; only broader lint and fresh precommit checks remain open.

The expanded carrier audit found two more peer-AS readers. Monitor repair
`796dbf1f9d` and RPKI repair `84b2e6b526` are committed. Monitor
`formatMonitorLine` now decodes `rpc.RemoteAS`, renders a pre-negotiation peer
as AS unknown and retains the raw event on malformed identity. RPKI
`handleEvent` now asks `GetPeerASN` and `ASN` before validating a JSON UPDATE,
and counts unknown or malformed identity under the registered
`ze_rpki_updates_refused_total` reason labels. State-down cleanup still runs
without an AS. The RPKI regression first failed in
`job-refusal-rpki-as-red-9a5d152b.log`; selected monitor/RPKI race tests now pass
in `job-refusal-identity-green-5de424dd.log`, including known-AS validation,
tracking, publication and ASN-free cleanup controls. The full monitor package
also passes. The full RPKI package initially failed in
`job-unit-pkg-9fb6ac6b.log` because the preceding reload fixture left its stopped
Adj-RIB owner's intentional refusing lookup registered. The new
`TestRPKIReloadReceiverRestoresUngatedSelection` in
`rpki_reload_fixture_test.go` failed both after-join assertions in
`job-refusal-rpki-fixture-red-1577efe7.log`. `startRPKIReloadReceiver` now
unregisters only after joining that owner, preserving production's fail-closed
exit. The full RPKI retry passes in `job-unit-pkg-a7417a90.log`. Independent
review is clean for the source, fixture cleanup and new forked functional test.

The malformed/unknown RPKI UPDATE regression uses the handler seam because
`Session.processMessage` rejects pre-Established UPDATEs,
`validateOpenPeerAS` refuses AS zero, and `appendPeerJSON` emits a typed known AS
for established updates. Native traffic cannot produce that malformed JSON
through this normal producer path. The complementary actual
`test/plugin/rpki-event-valid-forked.ci` proves a known-AS UPDATE crosses the
forked plugin's JSON transport and publishes its valid prefix event.

Both that fixture and `test/plugin/monitor-remote-as-stream.ci` are promoted and
pass 2/2 in `job-refusal-identity-promoted-aafe844f.log`. Before promotion, native
stress ran 80 invocations with both exact selectors. It reported "not reproduced
in 80 invocation(s)" in `job-refusal-identity-stress-d339f917.log`; exit 1 is the
stress reproducer's expected result when no failure is reproduced. Full capture
is `tmp/stress-repro/plugin-draft-monitor-remote-as-stream-rpki-event-valid-forked-a58588da4cc09570f70d38bc6e41fcfe21cc3832e694d6d9848134dc08f30a0f-20261111-160316-2630899211.log`.
Main also observed the actual monitor baseline pass 1/1, an unknown-AS-only
formatter overlay fail before the peer OPEN with AS0 in the PTY transcript,
then an unmodified clean rebuild pass 2/2. Those baseline/negative tool outputs
have no retained full scratch log; the promoted clean and stress logs above are
the durable files. No path is invented for the missing full output.

## Retained row evidence audit, 2026-11-11

This ledger accounts for the 54 census rows above. It preserves the producer-red
provenance in `238bdb62e9` and `plan/handover/24-refuse-by-default-implemented-awaiting-closure.md`;
it does not claim those historical runs were repeated. The regression definitions
were located in the current checkout. The independent source reviews are
retained in the Review Gate below: official round 2 is CLEAN and main's native
record/check passes all 591 code files. Broader lint and fresh precommit results
remain pending; no closing commit is claimed.

`H` means the historical producer-red claim is retained; `R` means a recent
main-thread red/green is recorded above. A named pending run is an evidence
obligation, not a newly observed product defect. The dependency-owned census rows
stay visible rather than being silently removed from the population.

| Row | Producer or surface | Regression (canonical source) | Evidence disposition |
|-----|---------------------|-------------------------------|----------------------|
| 1 | `Server.appendDataPlaneWriter` starts with no FIB writer | `TestConfigPathAutoLoadRefusesMissingDataPlaneWriter` in `internal/component/plugin/server/startup_writer_refusal_test.go`; `TestReloadAutoLoadRefusesMissingDataPlaneWriter` in `internal/component/plugin/server/startup_writer_refusal_test.go` | R; exact main-thread red/green and controls above |
| 2 | `sendPostStartupTo` failure logged at DEBUG | `TestPostStartupFailureWarnsWhileRunning` in `internal/component/plugin/server/poststartup_report_test.go` | H; representative-input regression retained |
| 3 | `LIB.allocateLabelLocked` returns a bound label | `TestLDPLabelExhaustionRefusesSecondBinding` in `internal/plugins/ldp/rfc3031_label_test.go` | H; representative-input regression retained |
| 4 | gNMI `Server.Get` ignores `encoding` | `TestGetRefusesUnadvertisedEncoding` in `internal/component/gnmi/get_test.go`; `TestGetAnswersEachJSONEncoding` in `internal/component/gnmi/get_test.go` | H; current `TestGnmicInterop` and all nine subtests pass against same-host Docker |
| 5 | `verifyResponseMessageAuthenticator` true when absent | `TestRadiusDiscardsEAPAnswerWithoutMessageAuthenticator` in `internal/component/radius/rfc3579_client_test.go` | H; representative-input regression retained |
| 6 | `BGPConsumer.InjectRoute` warning read as announced=0 | `TestBGPConsumerAnnounceReachingNoPeerWarns` in `internal/component/bgp/redistribute/consumer_test.go` | H; representative-input regression retained |
| 7 | `cos.Resolve` nil maps with no resolver | `TestResolveAbsentProviderRefusesUnitProfile` in `internal/core/cos/resolve_absent_test.go` | R; exact main-thread red/green and controls above |
| 8 | mrt peer state handlers' failed type assertion | `TestPeerLifecycleCallbackReceivesPeerInfo` in `internal/component/bgp/reactor/reactor_notify_callback_test.go` | H; representative-input regression retained |
| 9 | `yangTypeToValueType` maps a union list key to `TypeString`, so `ValidateListKey` never checks its member types | `TestUnionListKeyJudgedAgainstEachMemberType` in `internal/component/config/union_key_test.go` | H; representative-input regression retained |
| 10 | doctor `collectSchemaListeners` falls back to a hardcoded list | `TestDoctorRefusesUnderivableListenerSet` in `internal/component/doctor/checks_listener_test.go`; `TestDoctorSchemaWithNoListenerServiceIsAnAnswer` in `internal/component/doctor/checks_listener_test.go` | H; representative-input regression retained |
| 11 | debug `setModuleSetting` drops trailing keywords | `TestDebugModuleSettingRefusesTrailingWords` in `internal/plugins/debug/debug_test.go` | H; representative-input regression retained |
| 12 | `captureRawStart` answers done when it cannot capture | `TestCaptureRawStartRefusesAProtocolThatDidNotStart` in `internal/plugins/diag/cmd/capture_raw_test.go` | H; representative-input regression retained |
| 13 | rib family with no splitter dropped at Debug | `TestRIBFamilyWithNoSplitterWarnsOnce` in `internal/component/bgp/plugins/rib/rib_unstorable_test.go` | H; representative-input regression retained |
| 14 | `validateYangType` default arm accepts | `TestValidateUncheckedKindsRefused` in `internal/component/config/yang/validator_lexical_test.go`; `TestRFC7950Decimal64Refused` in `internal/component/config/yang/rfc7950_validator_decimal64_test.go` | H; representative-input regression retained |
| 15 | from-Established FSM arm reports `connection lost` for every cause | `TestCloseCauseReachesTheStateEvent` in `internal/component/bgp/reactor/peer_close_cause_event_test.go` | H; representative-input regression retained |
| 16 | `RawJSON.MarshalJSON` quotes invalid JSON instead of refusing | `TestRawJSONRefusesNonJSON` in `internal/component/plugin/types_test.go`; `TestRawJSONRefusesAScalar` in `internal/component/plugin/types_test.go` | H; representative-input regression retained |
| 17 | text `AppendOpen` writes the OPEN's own AS as `remote as` | `TestTextOpenRemoteASNamesThePeer` in `internal/component/bgp/format/text_open_remote_as_test.go`; `TestFormatNonUpdateRoutesToDedicatedFormatters` in `internal/component/bgp/format/text_test.go` | H; representative-input regression retained |
| 18 | `appendNLRIJSONValue` writes a placeholder route on decode failure | `TestUndecodableNLRIIsNeverWrittenAsARoute` in `internal/component/bgp/format/nlri_decode_failure_json_test.go`; `TestUnclaimedOpaqueNLRIIsNeverWrittenAsARoute` in `internal/component/bgp/format/nlri_decode_failure_json_test.go` | H; representative-input regression retained |
| 19 | `interfaceExists` any error reads as absent | `TestInterfaceExistsReadErrorIsNotAbsence` in `internal/component/iface/config_apply_test.go` | H; representative-input regression retained |
| 20 | DHCPv6 `dadUnreported` on VPP | `TestDHCPv6AddressesWithoutDADOutcomeNotReadAsUnique` in `internal/plugins/iface/dhcp/dhcp_v6_dad_read_linux_test.go` | H; named Linux DAD-outcome regression passes under race |
| 21 | VPP `detailsToInfo` empty addresses | `TestVPPGetInterfaceReportsAddresses` in `internal/plugins/iface/vpp/query_addresses_test.go`; `TestVPPListInterfacesReportsAddresses` in `internal/plugins/iface/vpp/query_addresses_test.go` | H; representative-input regression retained |
| 22 | LG `routes/protocol/{name}` 0 routes for a name | `TestAPIProtocolNameSelectsItsSession` in `internal/component/lg/handler_api_dispatch_test.go` | H; representative-input regression retained |
| 23 | `serializeRouteItem` path ID compared with zero | `TestAddPathIdentifierZeroIsAnIdentifier` in `internal/component/bgp/plugins/rib/ribout_native_test.go` | H; ADD-PATH matched baseline/negative/clean now complete; exact logs below |
| 24 | `vpnToJSON`, `LabeledUnicast.String` path ID compared with zero | `TestAddPathIdentifierZeroIsAnIdentifier` in `internal/component/bgp/format/text_test.go` | H; ADD-PATH matched baseline/negative/clean now complete; exact logs below |
| 25 | ddos `emitCleared` zero target never closes | `TestClearedCarriesTheDetectedVictim` in `internal/plugins/ddos/detect/characterize_test.go` | H; representative-input regression retained |
| 26 | `NewINET` path ID 0 as absent | `TestAddPathIdentifierZeroIsAnIdentifier` in `internal/component/bgp/rib/route_test.go` | H; ADD-PATH matched baseline/negative/clean now complete; exact logs below |
| 27 | gr egress nil state read as nothing stale | `TestLLGREgressFilter_NilStateWithdrawsEBGP` in `internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go`; `TestLLGREgressFilter_NilStateDepreferencesIBGP` in `internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go` | H; representative-input regression retained |
| 28 | `readOneHwmon` drops a 0.0 reading | `TestDetectThermal_ZeroReadingIsAReading` in `internal/component/host/thermal_linux_test.go` | H; named Linux zero-reading regression passes under race |
| 29 | resolve `DefaultConfig` read failure as missing | `TestDefaultConfigReadFailureIsNotMissing` in `internal/core/resolve/resolve_test.go` | H; representative-input regression retained |
| 30 | `buildParamMeta` error becomes nil | `TestBuildParamMetaRefusesAnUnpublishableSchema` in `cmd/ze/hub/command_meta_test.go` | H; representative-input regression retained |
| 31 | web interface detail swaps config for live on error | `TestInterfaceDetailReadErrorIsNotConfiguredState` in `internal/component/web/page_interfaces_test.go` | H; representative-input regression retained |
| 32 | web interface list reads any `ListInterfaces` error as no backend | `TestInterfaceListReadErrorIsNotConfiguredState` in `internal/component/web/page_interfaces_test.go` | H; representative-input regression retained |
| 33 | bgp-rib name selector matches no session | `TestRIBShowSelectsASessionByItsName` in `internal/component/bgp/plugins/rib/rib_peer_name_test.go` | H; representative-input regression retained |
| 34 | KEEPALIVE metadata AS0 for a dynamic peer | `TestKeepaliveNamesTheNegotiatedASOfADynamicPeer` in `internal/component/bgp/reactor/reactor_notify_test.go` | H; representative-input regression retained |
| 35 | message before negotiation carries AS 0 for a dynamic peer, never "not yet known" | `TestPreNegotiationEventsDoNotNameAS0` in `internal/component/bgp/reactor/reactor_notify_test.go` | H; representative-input regression retained |
| 36 | web interface counters partial reads any `GetStats` error as no counters | `TestInterfaceCountersReadErrorIsNotUnavailable` in `internal/component/web/page_interfaces_test.go` | H; representative-input regression retained |
| 37 | `buildWithdrawalPayload` returns nothing for two families | `TestMixedFamilyUpdateRefusedIsWithdrawnPerFamily` in `internal/component/bgp/reactor/forward_withhold_mixed_test.go` | H; representative-input regression retained |
| 38 | `prefixListFromIRR` cut and counted success | `TestPrefixListFromIRRRefusesOversizedList` in `internal/component/bgp/plugins/filter_irr/filter_irr_test.go` | H; representative-input regression retained |
| 39 | LG `transformRoutes` `primary` from an absent key | `TestShowRIBRowsSayWhichPathWon` in `internal/component/bgp/plugins/rib/rib_pipeline_bestfact_test.go`; `TestShowRIBBestFactUnderElectionAndPeerDown` in `internal/component/bgp/plugins/rib/rib_pipeline_bestfact_test.go` | H; representative-input regression retained |
| 40 | BMP Loc-RIB ORIGIN literal IGP | `TestLocRIBRouteMonitoringCarriesTheBestPathAttributes` in `internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go`; `TestLocRIBAnnounceWithoutAttributesIsRefused` in `internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go` | H; native `bgp-addpath-best-path-pmacct` passes 1/1; exact log below |
| 41 | bgp-rib `sameBest` ignores an attribute-only change of the winner | `TestRIBBestChangeOnAttributeOnlyChange` in `internal/component/bgp/plugins/rib/rfc4271_rib_bestchange_test.go` | H; representative-input regression retained |
| 42 | trafficusage `ebpfSupported` constant nil | `TestTrafficUsageReadinessCanFail` in `internal/plugins/trafficusage/doctor_test.go`; `TestNewAttacherLinux` in `internal/plugins/trafficusage/attach_linux_test.go` | H; ordinary classifier/constructor pair passes under race; explicit root/non-root actual-kernel probe passes in `5fa5efe3d2`'s `TestTCXProbeRuntimeReadiness` |
| 43 | l2tp `checkHealth` cannot fail | `TestL2TPHealthReadsDataPlane` in `internal/component/l2tp/health_test.go`; `TestL2TPHealthHealthyOnlyWithDataPlane` in `internal/component/l2tp/health_test.go` | H; both named health tests also pass in later 2026-11-01 product job |
| 44 | `getConfigPathPlugins` starts plugins after `ErrMissingDependency` | `TestBootAutoLoadRefusesMissingHardDependency` in `internal/component/plugin/server/startup_autoload_test.go` | Dependency spec owns this census row; implemented in `30422ebe30`, closed in `a81034e794` |
| 45 | MRT table dump and `show system` NTP block skipped when not built | `TestSystemDateRefusesAbsentNTPProvider` in `internal/component/cmd/show/system_ntp_test.go` | R for NTP; MRT belongs to closed dependency spec and shares `ze_bgp` |
| 46 | ddos responder, exabgp bridge, flowexport enrichment declare no BGP dependency | `TestConfigNeedPathsExistInSchema` in `internal/plugins/flowexport/config_need_test.go` | Dependency spec owns this census row; declarations in `30422ebe30`, closure `a81034e794` |
| 47 | redistribute `destination`, ls_export `source` not checked against the registry | `TestRefuseRedistributeRulesNamesAnUndeclaredDestination` in `internal/component/config/loader_redistribute_root_test.go`; `TestDomainNamingAnUnpublishedSourceIsRefused` in `internal/component/bgp/plugins/ls_export/export_source_test.go` | Dependency source refusal in `30422ebe30`; redistribution destination red/green in `82c2cb97a7` belongs to `spec-fixit-redistribution-chain-drops-silently` |
| 48 | fib-vpp `newBackend` falls back to the `mockBackend` test double when VPP is unreachable | `TestOpenFibVPPWriterRefusesWithoutVPP` in `internal/plugins/fib/vpp/register_answer_test.go`; `TestShowFibVPPAnswersWhyVPPIsUnreachable` in `internal/plugins/fib/vpp/register_answer_test.go` | H; representative-input regression retained |
| 49 | VPP `readVPPIfaceStats` any stats read error read as no counters | `TestGetStatsReadErrorIsNotAbsence` in `internal/plugins/iface/vpp/apply_test.go` | H; later 2026-11-11 VPP monitor and alias jobs pass the named regression |
| 50 | `answering.answered` read failure answered as not answered | `TestAnsweredNamesACommitItCouldNotRead` in `internal/le/commit/owed_trailers_unread_test.go` | H; representative-input regression retained |
| 51 | rs `extractWireNLRIRecords` unknown encoding context answered as no wire UPDATE | `TestExtractWireNLRIRecordsRefusesAnUnknownContext` in `internal/component/bgp/plugins/rs/server_inventory_unknown_context_test.go` | H; representative-input regression retained |
| 52 | `checkVPPHealth` healthy when the VPP socket is absent | `TestVPPHealthCheckSocketMissing` in `internal/plugins/iface/vpp/register_test.go`; `TestVPPHealthCheckComponentNotRunning` in `internal/plugins/iface/vpp/health_test.go` | H; representative-input regression retained |
| 53 | rib `showPipeline`, `bestPipeline` parse error answered as an `{"error":...}` payload with status done | `TestPipelineParseErrorsReachCommandVerdict` in `internal/component/bgp/plugins/rib/rib_pipeline_refusal_test.go` | R; full native RIB package and rebuilt `rib-filter-show` daemon fixture pass |
| 54 | `commandRegistrars` does not classify `RegisterTextRenderer` / `MustRegisterTextRenderer`, added by this spec's 238bdb62e9; `TestEveryCommandRegistrarIsMatched` red | `TestEveryCommandRegistrarIsMatched` in `internal/le/plugin/imports/pluginimports_roots_test.go` | Historical red in `238bdb62e9`; source repair and passing plugin/imports tests retained in `a6f93cea5d` |

The first two regressions are shown when a row names several; the journal Fix
cell retains the complete set. Row 24 additionally retains the negotiation-presence
cases in `internal/component/bgp/plugins/nlri/vpn/rfc4659_vpn_test.go` and
`internal/component/bgp/plugins/nlri/labeled/types_test.go`.

### Acceptance criteria still needing execution or reconciliation

- AC-1–AC-6: canonical fixtures, changed-line, annotation and invalid-input tests
  exist in `internal/le/arch/refusebydefault/refusebydefault_test.go`. Preserve
  the historical phase-green proof. Stage
  registration is pinned by `TestVerifyStagesIncludeRefuseByDefault` in
  `internal/le/verify/engine/stages_refusebydefault_test.go`. No `le verify` run
  is owed here.
  Direct-assignment and forward package-alias repairs are committed in
  `06a4a22cbc`; the full race retry in reused `job-unit-pkg-ef92ffff.log` now
  covers both. Focused state/naming green and the bounded 86.5% coverage result
  are recorded above.
- AC-7: `2558080335` propagates the builder's reason through `fwdWithdrawal.of`
  and both real warning callers. Its source guard failed before the fix; 32
  selected race roots pass afterwards. Main's later native report and exact
  54-row/97-producer join found zero owned refused or annotated matches. The
  final named report and exact 54-row/125-entry AST-anchored comparison also
  join both populations to zero owned hits after the BMP/NTP repairs. Report
  exit zero alone does not prove those semantic behaviors.
- AC-8: retained unit/command evidence includes the recent absent-CoS and writer
  repairs. JSON RPKI admission and monitor race controls, full RPKI after
  fixture isolation, and both promoted identity functional tests pass.
  The full RIB package passes after fixture cleanup, and the rebuilt
  daemon's `rib-filter-show` fixture passes 1/1.
- AC-9: the LDP Ze-kernel scenario passes 1/1. The changed gnmic default/PROTO
  contract passes with all nine interop subtests; RADIUS evidence is retained.
- AC-10: ADD-PATH matched logging-only baseline passes, the omitted-zero overlay
  fails with RX 1 surviving and RX 0 missing, and the unmodified clean run passes.
- AC-11: thermal zero-reading Linux regression passes under race; existing
  LLGR, DDoS and dynamic-AS producer-red evidence is retained.
- AC-12: TCX actual-kernel root/non-root proof passes with the explicit
  expectation test below. Native BMP/pmacct passes 1/1. The later L2TP and VPP
  stats passes below supersede their old unrun labels.
- AC-13: the ledger preserves all rows and regression attribution; the named
  platform runs are complete, while independent final audit remains owed. The netlink stats
  regression's observed negative and committed-source clean pass are recorded below.
  Rows 44, 46 and 47, plus MRT in row 45, are the dependency spec’s work by the
  existing Known Limitations and handover 24. NTP is explicitly brought back
  here by the owner instruction already recorded above.

### Later evidence replacing old unrun labels

The following logs were inspected for actual PASS lines; none was rerun here.
Canonical source and commands are retained so the session paths do not stand
alone as proof.

- PKI: `test/plugin/{pki-certificate-export-show,as112-dot-pki,geodns-dot-pki}.ci`
  all pass in `tmp/session/2026-11-01-175038/scratch/job-ac3d-plugin-6e5e755e.log`
  (`test bgp plugin`, named fixture population), and later in the Linux CI
  functional job retained as
  `tmp/session/2026-11-09-222002/scratch/ci-job-129519126920.log`. These resolve
  the three Linux-owed labels in handover 24; they do not prove gnmic or BMP.
- L2TP: `TestL2TPHealthReadsDataPlane` and
  `TestL2TPHealthHealthyOnlyWithDataPlane` in
  `internal/component/l2tp/health_test.go` pass in
  `tmp/session/2026-11-01-174538/scratch/job-home-cpe16-product-b60d37f8.log`.
  The same job has unrelated PPP failures, so this is a named-test pass, never
  a claim that the whole job passed.
- VPP: `TestGetStatsReadErrorIsNotAbsence` in
  `internal/plugins/iface/vpp/apply_test.go` passes in both
  `tmp/session/2026-11-11-005934/scratch/job-vpp-alias-green-e2d672f4.log` and
  `tmp/session/2026-11-11-010844/scratch/job-vpp-monitor-stop-green-e2d672f4.log`.
  The separate netlink `GetStats` obligation from handover 24 is not silently
  conflated with this VPP result.
- TCX: `TestTCXProbeRuntimeReadiness` in
  `internal/plugins/trafficusage/readiness_integration_linux_test.go`, committed
  as `5fa5efe3d2`, passes
  as root UID0 with measured ready and ENODEV, and as UID1000 with no effective
  capabilities, measured unknown and EPERM. The same host-built integration
  binary ran through `test qemu docker-lab kernel <matching-runtime-vmlinuz>`
  command mode on Ze7.3.0, once with `root` and
  `-tcx-probe-expect=ready`, then without `root` and
  `-tcx-probe-expect=unknown`. Both selected only this test with `-test.run`,
  `-test.v`, `-test.count=1` and `-test.timeout=2m`; neither requested run
  skipped, and both command/check/lab exits are zero. Both guests used kernel
  SHA256 `1305faf7734c0c309b0b1d97d37951df625f71e548bbdc04f7ff456f27d4c5e0`.
  Actual outputs are `tcx-root-runtime-19008.log` and
  `tcx-ordinary-runtime-57305.log` in this session's scratch; consoles are
  `tmp/qemu/docker-lab-console-181362749.log` and
  `tmp/qemu/docker-lab-console-3659063372.log`. Expectations were selected
  before the real probe; the assertions require the original kernel errno.
- Linux controls: `TestDetectThermal_ZeroReadingIsAReading`,
  `TestDHCPv6AddressesWithoutDADOutcomeNotReadAsUnique`,
  `TestTrafficUsageReadinessCanFail` and `TestNewAttacherLinux` all have explicit
  PASS lines in `job-refusal-linux-controls-384752ff.log` in session scratch.
  Main selected the host, DHCP and trafficusage package tests under native
  feature tags and race; no named control skipped.
- gNMI: `TestGnmicInterop` in
  `internal/component/gnmi/gnmic_docker_test.go` and all nine subtests pass in
  `job-refusal-gnmic-interop-608dd3c8.log` in session scratch, with no skip.
  Main used admitted `go test` with full feature tags plus `docker`, race and
  `ZE_REQUIRE_DOCKER=1` against the local same-host Docker daemon. The proof
  includes advertised JSON/JSON_IETF, explicit and default encoding Get,
  absent-path NotFound, unadvertised encoding Unimplemented, Subscribe sync,
  Set commit, and missing/wrong-token Unauthenticated controls.

### Completed native interop evidence

Main ran each named scenario through the current-source `test integration
interop` route with `INTEROP_SCENARIO` selecting the row. The matched ADD-PATH
baseline and negative both included the same logging-only `prepare.go` overlay;
the negative additionally changed only `nlri.WriteNLRI` to omit the four
identifier octets for identifier zero. This holds the diagnostic instrumentation
constant while testing the zero-ID writer. The final clean rebuilt the
unmodified source with neither overlay.

| Scenario and control | Actual result and retained log in session scratch |
|----------------------|--------------------------------------------------|
| `addpath-zero-id-frr`, logging-only baseline | 1 passed, 0 failed; `job-refusal-addpath-startup-debug-632ff3c0.log` |
| Same scenario, omitted-zero negative | 0 passed, 1 failed at assertion 4: FRR had one path, `AddPath ID: RX 1`, and no RX 0 path; `job-refusal-addpath-zero-negative-632ff3c0.log`. The failure first names the missing two-path count; it is not relabeled as a direct RX-0 string assertion. |
| Same scenario, unmodified clean | 1 passed, 0 failed; `job-refusal-addpath-clean-632ff3c0.log` |
| `ldp-init-unknown-tlv-frr` | 1 passed, 0 failed; `job-refusal-ldp-interop-632ff3c0.log` |
| `bgp-addpath-best-path-pmacct` | 1 passed, 0 failed; `job-refusal-bmp-best-path-interop-632ff3c0.log` |

The canonical ADD-PATH catalog rows are `scenarioAddPathZeroIDFRR` in
`internal/le/interoplab/bgp/checkers.go` and `check_extras.go`; they read
FRR's session, route count and RX identifiers. The LDP checker is
`checkLDPInitUnknownTLV`; the BMP/pmacct scenario uses `checkMEDWholeSet` in
`check_med_pmacct.go`. Scenario configs remain in
`test/interop/scenarios/`. The source census below accounts for the changed
producer and the independent peer readers; the logs record execution.

### Actual withdrawal-path guest proof

The first explicit-error regression observed two notify hits and one builder
hit in retained run `f0b4a2f9`, with four callback controls passing. Independent
actual-patch generic/style review was CLEAN before `a7afabc1be`; the later
caller-propagation follow-up still needs its own final proof and review.

Main ran `test/plugin/linklocal-only-multihop-withdraw.ci` in the Ze-kernel
QEMU lab. It passes 1/1 with every phase executed on kernel `7.3.0-ze`, SHA256
`1305faf7734c0c309b0b1d97d37951df625f71e548bbdc04f7ff456f27d4c5e0`, with
command/check/lab exit zero in `job-refusal-withdrawal-qemu-afde2315.log`.
The route-server fast path reaches `mods.SetWithdraw`, `fwdWithdrawal.of` and
`buildWithdrawalPayload`. The fixture first establishes the advertised prefix
and then observes its withdrawal, so a path that sent nothing cannot pass.
This proves that useful withdrawal path. An earlier report identified the
error-to-nil residual in `fwdWithdrawal.of`; `2558080335` fixes it, and the final
54-row/125-entry comparison records zero owned hits in both populations.

### Scoped execution queue

Execution stays in the main thread against current source in the main checkout.
The named native `test unit pkg` route supplies the checkout's feature tags,
timeout and race detector; its `run` keyword takes a regex.

1. Finish the queued broader 13-package lint attempt after changing foreign
   inputs settle. The narrow checker lint covers only HOST and Linux
   integration, both zero issues; the latest broad planning attempt refused
   a changed `core/sysctl/known.go` and produced no lint verdict.
2. Read the queued fresh repo, weakened-test and citation metadata results and
   complete the bounded file-existence record. Citers were already repointed:
   main's whole-tracked-path search found only this spec itself. Current docs
   have one foreign catalogue description drift, with 447 commands and all own
   source anchors validated. Preserve foreign work and attribute any findings
   individually; unknown queued results are not passes.
3. After those gates pass, use the repository commit workflow for the remaining
   functional-test piece and the two closing commits. The native round-2 review
   artifact/check already passes all 591 code files. Canonical 2/2, wrapped
   pmacct 1/1, all 80 loaded invocations, all 16 committed RFC tuples and the
   zero-owned individual RFC audit are complete. No behavioral proof needs a
   timestamp refresh, and no third source pass is required for these record
   corrections. The selected 67-root RFC fixture green does not relabel the
   retained failed full-package retry as passing. Keep this spec in progress
   until the pending checks and closing commits actually finish.

The handover's unqualified netlink `GetStats` item lacked a direct-method
regression. `netlinkBackend.GetStats` refuses missing links, read errors
and absent statistics, but the backend's existing test files contained no direct
`GetStats` call. `TestGetStatsInvalidName` in the iface package exercises only
name validation, and `TestGetInterface` reads statistics through a different
producer. Neither is relabelled as proof of those three `GetStats` branches.
The parent assigned a focused regression repair at this producer to
`netlink_stats_proof`. `internal/plugins/iface/netlink/stats_linux_test.go` now
declares `TestNetlinkGetStatsMissingLink`,
`TestNetlinkGetStatsReadErrorIsNotAbsence`, `TestNetlinkGetStatsMissingCounters`,
`TestNetlinkGetStatsZeroCounters` and `TestNetlinkGetStatsMeasuredCounters`.
They drive the real method through its `linkByNameForStats` lookup seam; the
production default remains `netlink.LinkByName`. Main's current-source named
`test unit pkg path ./internal/plugins/iface/netlink/ run '^TestNetlinkGetStats'`
exited zero, reporting the selected package `ok` in
`tmp/session/2026-11-11-014000/scratch/job-unit-pkg-d8ec4396.log`. The native
route supplies full feature tags and race. Its default output gives no
individual test names, so no five named PASS lines are claimed. Independent
source review by `reload_close` is clean.

The prepared negative overlay changes only the two refusal branches: another
read error is mislabeled as `ErrInterfaceNotFound`, and missing statistics
become a successful zero-counter answer. It is retained as
`tmp/session/2026-11-11-014000/scratch/netlink-getstats-negative-overlay.json`,
mapping to `netlink-getstats-negative.go.txt` beside it. Main executed it through
the Go child's overlay flag, without mutating shared source. In
`tmp/session/2026-11-11-014000/scratch/job-netlink-getstats-negative-d7c279be.log`,
both permission and wrapped-I/O subcases of
`TestNetlinkGetStatsReadErrorIsNotAbsence` fail on the fabricated absence, and
`TestNetlinkGetStatsMissingCounters` fails on the fabricated zero record.
The typed-missing-link, valid-zero and measured-counter controls pass in the
same negative run. The first verbose clean attempt stopped at the foreign
hook-runtime launcher compile failure before any test ran. After commit
`3a66d9d5cd`, main's registered job ran the same five named tests with verbose
output under native tags and race. Every test and both read-error subcases
pass in `job-netlink-getstats-committed-clean-5f83ba7a.log` in the same scratch
directory. The direct-method regression, observed negative and committed-source
clean control are complete; the full refusal spec stays open.
This is not a kernel-capability finding or a reason to repeat the already-passing
VPP test.

The 38 `RFC-approval-owed` trailers in `238bdb62e9` are historical:
`origin/main` already contains that commit. `readPersonActs` in
`internal/le/commit/owed_trailers.go` reads only the selected target's commits
outside upstream. They are not current push debt. The earlier global report
of 64 violations is superseded by the final global report's 60, whose ownership
classification is still active. The original 37-ID scope artifact remains
`scratch/refusal-rfc-closure-scope.md`; the current 47-ID population includes
the additional ASPA, AIGP and RFC7084 carriers stated above. LOCAL_PREF, ASPA,
all four owned SHIFT reseals and the RFC7311/RFC7084 renewals are committed in
`a7afabc1be` and `62f9ff2116`. No blanket foreign reseal or fresh owned-zero
verdict is implied. Final lint, documentation checks, source audits and formal
independent review remain owed; no whole-tree verification is added.


## Prior blocker and completed corpus repair

The following attempted-run history is superseded by the completed evidence
above. It preserves the causes and the original commands; none of its older
pending statements is a current execution queue.

The continuation built the named launcher from the current checkout on
2026-11-11. The missing inventory fields below are a historical build failure. The later
ADD-PATH baseline/negative/clean and LDP runs completed.

The config corpus repair shares the topology lab's CPE renderer with config
validation. `internal/test/labconfig/homecpe/render.go` owns `Render`, PKI
issuance and placeholder substitution. The corpus previously validated the
unrendered template. Excluding that file would hide the lab input, so the fix
moves the existing producer and validates its result. The three deliberate
refusal fixtures gain their existing diagnostic codes. The doctor fixture uses
table 100, outside the reserved ranges that `policyroute.validateActionTable`
refuses. Main-thread race tests passed for the shared renderer, its topology
callers, and `TestSweepRendersHomeCPE`, including the renderer-bypass negative
control. Evidence: `tmp/session/2026-11-11-014000/scratch/job-refusal-cpe-focused-c8e0a05a.log`.
Independent source review is clean, and the test judge approved all moved and
mechanically updated tests. The four fixture drafts subsequently passed and were promoted in `e7a77d6e73`. Concurrent commit `ab4a67d631` carried the shared-rendering paragraph
and source anchors in `docs/architecture/testing/interop.md`.

The 2026-11-11 main-thread attempt to run `addpath-zero-id-frr` stopped in
shared interop staging before any scenario executed: `0 passed, 0 failed`.
`StageBinaries` (`internal/le/interoplab/zebuild.go`) could not compile the
required `test/interop/le-linux`. The current `Snapshot` and `sourcePackage`
definitions in `internal/le/repo/inventory/source.go` lack fields used by:

- `assemblyheaders.go:76`: `Snapshot.assemblyPredefines`;
- `assemblyheaders.go:120`: `Snapshot.headerPresence`;
- `assemblylinkage.go:52`: `sourcePackage.assembly`;
- `assemblylocalinclude.go:42`: `Snapshot.localHeaders`.

The failing inventory work belongs to
`plan/post-release/engineering/spec-inventory-fast-registry-checks.md`, claimed
by session `03d43e18-c488-4511-8040-2650b2986bb3`; its claim file is
`tmp/session/.session-03d43e18-c488-4511-8040-2650b2986bb3`. The setup failure
is recorded in `tmp/session/2026-11-10-224742/scratch/refusal-addpath-interop.log`.
The same common staging prerequisite blocks the owed `ldp-init-unknown-tlv-frr`
lab; that scenario was not separately run. This is a compile blocker, not a
kernel-capability finding or a failed protocol assertion. No stale image or
`NoBuild` bypass was used.

Resume when the owning inventory session repairs the producer/consumer mismatch
and the current interop binaries compile. Run the ADD-PATH baseline, its negative
control, and a clean rebuild/run; then run the LDP scenario. The prepared
`tmp/session/2026-11-10-224742/scratch/refusal-addpath-zero-overlay.json` changes
only `nlri.WriteNLRI` to omit the four identifier octets when `PathID() == 0`.
Pass it through `GOFLAGS=-overlay=<absolute path>` for the negative run, preserving
other effective flags; remove that overlay for the clean rebuild. Neither the
mutation nor a baseline scenario completed in this session. The full spec still
needs its remaining evidence and independent review; this handback does not
declare its older phases or acceptance criteria complete.

### Completed repair evidence, 2026-11-11

- `a6c70ec923`: `cos.Resolve` refuses a named profile without a resolver,
  preserving no-profile and explicit opt-out controls. Main observed red then
  race green in `tmp/session/2026-11-10-235802/scratch/job-refusal-metadata-red-cos-green-3504c0bf.log`.
- `2b90f8864b`: boot and reload refuse a missing forwarding writer. Declared
  `Registration.DataPlane` metadata supplies the absent writer and its existing
  typed build-tag provenance. The two new inventory tests were carried by
  concurrent commit `12720abe4c`; the remaining source and tests landed together.
  Main metadata/writer green: `tmp/session/2026-11-11-000258/scratch/job-refusal-reload-repairs-green-98a0e2b7.log`;
  typed provenance and existing selection controls:
  `tmp/session/2026-11-11-001540/scratch/job-refusal-final-vpp-parser-05244d6d.log`.
- `528fdc8fdc`: inline VLAN maps belong to the interface schema on exactly
  Ethernet, dummy, veth and bridge units; profile references stay gated by CoS.
  Missing-provider guidance removes those references without requiring absent
  schema. An actual `ze_core` daemon passes profile refusal and inline acceptance,
  and the resolved schema retains exactly the four prior interface kinds.
  Main race evidence, including affected resolver/interface consumers:
  `tmp/session/2026-11-11-004105/scratch/job-cos-absent-integration-green-3454b9a9.log`.
  Independent review approved the source and preserved test assertions.
- `4a66448ad9`: the two repaired silent-fall-through journal rows record their
  producer, fix commits and observed evidence, without taking foreign rows.
- All eight existing full-feature parser cases passed in
  `tmp/session/2026-11-10-224742/scratch/refusal-parser-consumers.log`:
  `cos-profile`, `cos-profile-dummy`, `cos-profile-conflict`, `cos-profile-invalid`,
  `cos-profile-not-found`, `iface-vlan-qos-parse`, `iface-vlan-qos-invalid`, and
  `iface-vpp-rejects-nonidentity-qos`.
- Full-feature race `TestResolveCoSForUnit` and `TestCoSProfileParse*` passed in
  `tmp/session/2026-11-11-005101/scratch/job-cos-schema-consumers-39925504.log`.
  The same job's `TestRepositoryConfigsValidate` is **red** after validating
  3,090 Ze and 241 ExaBGP inputs: eight errors across five fixtures, none in CoS.
  The remaining fixtures are `test/interop-topology/cpe/ze-cpe.conf` (four
  placeholder CA/WireGuard errors), `test/reload/dns-source-reload.ci` (second
  config lacks mode), `test/reload/reload-parse-error-single-prefix.ci` (intentional
  `zzqqxx` input), `test/ui/config-compiled-out-feature-names-tag.ci` (`vppqx`
  typo fixture), and `test/ui/doctor-procfs-netlink.ci` (reserved table 3001).
  No fixture expectation was weakened, and no full-corpus green is claimed.
  These findings also need disposition before claiming the required corpus
  evidence complete; the interop staging compile blocker remains decisive.

## Task

Root cause S2 of the journal backlog (owner order, 2026-10-23: turn the open journal rows
into root-cause specs). 46 open rows across five problem classes share one mechanism: a
function reaches a state where it has no answer (a lookup missed, an input it does not
recognise, a resource it cannot read, a value the protocol gives no "absent" spelling) and
its signature or its branch structure gives it no way to say so. It then returns what the
type defaults to, or what the author guessed: `true` for an absent authenticator, `nil` maps
for a missing resolver, "absent" for any link-read error, path identifier 0 as "no
identifier", a literal `StatusHealthy`, ORIGIN IGP. The caller cannot tell that value from
a real one, so the defect surfaces far from its cause and reads as data
(`ai/rules/principles.md`, first directive).

Why it matters for shipping Ze: these values reach peers (a label bound to two FECs, a VPN
NLRI misframed, an LLGR_STALE route sent to a non-LLGR peer), reach operators (a healthy
health check that cannot fail, `show` answering empty for a path it does not render), and
reach security boundaries (a RADIUS EAP answer with no Message-Authenticator accepted).

The fix has two halves. First, every owned producer gets a refusal path and its callers
handle it. Second, a change-set check, `./le arch refuse-by-default`, refuses the shapes
that produced the family, so new code cannot reintroduce it. Where zero really is the
answer that another branch relies on, the check requires the guard to be named
(`ai/rules/principles.md`: "A zero that another branch relies on is a guard, and it MUST be
named as one").

## Required Reading

### Architecture Docs
- [ ] `ai/rules/principles.md` - first directive, silently wrong values
  → Constraint: code that cannot answer MUST say so; a zero another branch relies on is a guard and is named as one
- [ ] `docs/architecture/core-design.md` - registration, plugin boundary
  → Constraint: a cross-boundary call that no-ops when the plugin runs external is this defect; the refusal crosses the boundary as an error answer, never as an empty success
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - the fixed verify stage population (`internal/le/verify/engine/stages.go`)
  → Decision: the new check is a change-set stage beside `arch enumeration` and `arch compound-guard`, so the tree's existing instances are reported, not a red on every commit
- [ ] `docs/contributing/ze-go-style.md` - error and naming conventions for the new error types
- [ ] `docs/architecture/plugin/feature-gates.md` - the not-built refusal spec-plugin-optional-dependency-refusal (closed) delivered
  → Constraint: rows 5 to 8 of the census are owned there; do not duplicate its refusal, call it

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc3579.md` - RADIUS EAP, Message-Authenticator
  → Constraint: RFC 3579 Section 3.1: "Access-Accept, Access-Reject or Access-Challenge packets including EAP-Message attribute(s) without a Message-Authenticator attribute SHOULD be silently discarded by the NAS."
- [ ] `rfc/short/rfc3031.md` - label uniqueness
  → Constraint: RFC 3031 Section 3.14: "In all other cases, Rd MUST NOT distribute to Ru bindings of the same label value to two different FECs."
- [ ] `rfc/short/rfc5036.md` - LDP unknown TLV handling
  → Constraint: RFC 5036 Section 3.3: "Upon receipt of an unknown TLV, if U is clear (=0), a notification MUST be returned to the message originator and the entire message MUST be ignored; if U is set (=1), the unknown TLV MUST be silently ignored and the rest of the message processed as if the unknown TLV did not exist."
- [ ] `rfc/short/rfc5303.md` - IS-IS three-way adjacency
  → Constraint: RFC 5303 Section 3.2: "If the option is present and contains invalid Adjacency Three-Way State, the PDU SHALL be discarded and no further action is taken."
- [ ] `rfc/short/rfc7911.md` - ADD-PATH identifier
  → Constraint: RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE message, the NLRI encoding MUST be extended by prepending the Path Identifier field, which is of four octets." Section 2: "A BGP speaker that receives a route should not assume that the identifier carries any particular semantics." Presence follows the negotiated capability, never the value
- [ ] `rfc/short/rfc9494.md` - LLGR
  → Constraint: RFC 9494 Section 4.3: "The route SHOULD NOT be advertised to any neighbor from which the Long-Lived Graceful Restart Capability has not been received."
- [ ] `rfc/short/rfc2205.md` - RSVP Path message
  → Constraint: RFC 2205 Section 3.1.3 grammar: "<sender descriptor> ::= <SENDER_TEMPLATE> <SENDER_TSPEC>"; a sender descriptor without SENDER_TSPEC is malformed
- [ ] `rfc/short/rfc4090.md` - RRO protection flags
  → Constraint: RFC 4090 Section 4.4, "Local protection available: 0x01": "Indicates that the link downstream of this node is protected via a local repair mechanism"; the flag reports state, so it is read from the bypass LSP's state, not from config
- [ ] `rfc/short/rfc9069.md` - BMP Loc-RIB Route Monitoring
  → Constraint: RFC 9069 Section 5.4 quotes RFC 7854 Section 4.6: "Following the common BMP header and per-peer header is a BGP Update PDU."; the PDU carries the best path's own attributes

**Key insights:**
- The family is a signature problem as often as a branch problem: `interfaceExists` returns `bool`, `allocateLabelLocked` returns `uint32`, `DefaultConfig` returns `string`. No branch can refuse through those signatures
- ADD-PATH presence by value is one fact coded at seven sites; presence must come from the negotiated capability carried with the route

## Current Behavior (MANDATORY)

**Source files read (verified 2026-10-23):**
- [ ] `internal/component/radius/packet.go` - `verifyResponseMessageAuthenticator` returns `true` when the attribute is absent (`if !present { return true }`); `dispatchResponse` in `client.go` then delivers an EAP-bearing Accept with no Message-Authenticator. The send side already refuses the mirror case (`carriesEAPMessage` check in the encoder)
- [ ] `internal/component/iface/config_apply.go` - `interfaceExists` is `GetLink` error equals nil, so any read failure reads as "absent" and the apply recreates a present device
- [ ] `internal/plugins/ldp/lib.go` - `allocateLabelLocked` loops over the label space and, when every label is in use, returns `l.nextLabel`, a label already bound; `EnsureLocal` stores it for a second FEC
- [ ] `internal/core/cos/cos.go` - `Resolve` returns nil maps and nil error when no resolver is registered, so a profile reference in a build without the cos plugin is accepted and does nothing
- [ ] `internal/plugins/trafficusage/attach_linux.go` - `ebpfSupported` is a constant nil, so the readiness diagnostic cannot fail on Linux
- [ ] `internal/component/l2tp/health.go` - `checkHealth` answers Healthy whenever the service is registered; nothing it reads can be false about the kernel data plane
- [ ] `internal/component/bgp/rib/route.go` - the index key sets `hasPathID` from `pathID != 0`; the same test appears in `bgp/format.go`, `plugins/rib/rib_replay.go`, `plugins/rib/rib_pipeline.go`, `format/text_json.go`, `plugins/rib/rib.go`
- [ ] `internal/component/plugin/server/startup_autoload.go` - `appendDataPlaneWriter` logs a WARN and starts with no FIB writer when no compiled-in plugin programs the configured backend

**Behavior to preserve:**
- Every existing explicit refusal and its message; the send-side RADIUS Message-Authenticator refusal
- Wire output of every family where the path identifier is negotiated (the fix changes presence for identifier 0 only)
- RFC 5036 U=1 unknown TLVs stay silently ignored (`capability_rfc5561_test.go`)

**Behavior to change:** every owned row's producer answers a refusal (typed error, a discard with a counted reason, or an explicit "unknown" state) instead of the plausible value. The row table below names each.

## Existing Mechanisms

| Mechanism | Location | Current behavior and use |
|-----------|----------|--------------------------|
| Optional provider refusal | `config.refuseAbsentOptionalProviders`, `registry.MissingDependencyError` | Refuses a configured need whose provider is unavailable; reuse the existing dependency contract where the remaining startup refusal fits it. |
| CoS resolver registration | `internal/core/cos/cos.go`, `Resolve` | Delegates candidate profile resolution to the registered plugin; extend the missing-provider branch to refuse a requested profile while preserving configurations that request none. |
| Data-plane writer discovery | `server.appendDataPlaneWriter`, `registry.PluginForDataPlane` | Derives the writer from registration; extend its missing-writer result into a startup refusal without inventing a second provider list. |
| Compiled-out plugin provenance | `repoinventory.Plugin`, `gatedtable.rowsOf`, `registry.GatedPluginTags` | Derives gate requirements from committed registrations and the manifest; preserve `Registration.DataPlane` in this existing route so a missing forwarding provider retains its identity and build tags. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Wire input (RADIUS response, LDP Init, IS-IS Hello, RSVP PATH, BGP UPDATE), config verify and apply, operator `show` and web pages, health and doctor checks, plugin RPC answers

### Transformation Path
1. The producer meets a state it cannot answer (lookup miss, unknown input, unreadable resource)
2. Today: it returns the type's zero, a constant or a fallback. After: it returns a typed refusal (`ErrNotFound`, `ErrNotBuilt` from the dependency spec, `ErrUnrecognised`, `ErrExhausted`) or the protocol's discard action
3. The caller branches on the refusal: config verify refuses the config, the wire handler discards and counts, the show handler answers an error naming the path, the health check reports Unknown or Degraded with the reason

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine and plugin | refusal travels as the RPC error answer, never as an empty success | No |
| Config verify and apply | verify refuses what apply cannot honour | No |
| Wire | protocol discard or notification per the quoted RFC sentence | No |

### Integration Points
- `internal/le/arch/` - the new `refusebydefault` package registers like `arch enumeration` (`internal/le/arch/enumeration/register.go`) and joins the stage list in `internal/le/verify/engine/stages.go`
- The dependency spec's not-built refusal - reused for `cos.Resolve` and `appendDataPlaneWriter`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | refusals use each producer's existing error return or gain one; no side channel |
| No unintended coupling (components stay isolated) | Yes | typed errors live in the producing package; callers match with `errors.Is` |
| No duplicated functionality (extends existing, does not recreate) | Yes | not-built refusal is the dependency spec's; ADD-PATH presence becomes one accessor replacing seven value tests |
| Zero-copy preserved where applicable (refs, not copies) | Yes | no encoding path gains a copy; presence is a flag already carried |
| Registration over hardcoding, outbound | Yes | the check registers through `leroot.Register`; no switch learns its name |
| Registration over hardcoding, inbound | Yes | the check's rule set is read from AST shapes, not from a list of package names; searched `stages.go` (one stage line is the declared population, as for every arch stage) |

## Changed-carrier census for design admission

This census follows the formats changed by `238bdb62e9`, then compares their
current writers and readers. It is separate from the 54-row defect ledger:
one format crosses several producers, transports and consumers, and an unchanged
reader still needs a preservation decision. Main's source verification of this
record remains owed; the table itself supplies no native proof or approval.

The search population was non-test Go under `cmd`, `internal` and `pkg`, plus
non-Go sources and embedded scripts under those roots, `test` and `examples`.
The original commit's path list and relevant hunks identified the format changes,
then current-source searches followed `PeerAS`, `RemoteASOf`, `GetPeerASN`,
`remote.as`, `remote as`, `HasPathID`, `HasAddPath`, `NewINET`, `path-id`,
`path-information`, `RawJSON`, `pipelineRefusal`, `graph`, `best` and `primary`.
The non-Go pass included extensionless `*.run` Python programs, `*.py`, `*.sh`,
`*.pl` and fixture `*.ci` bodies, including scripts embedded in Go string
literals. Generated web assets and vendored code did not define the population.
Positive controls were `appendPeerJSON` for the native JSON writer,
`parseTextOpen` for its text reader, `WriteNLRI` for wire framing,
`API._record` in the compatibility Python helper for a non-Go reader, and the
old-4.2 `api-attributes-path.run` for a non-Go command writer. Each was found
by the corresponding terms before the carrier was classified.

Paths below use these local prefixes: BGP is `internal/component/bgp/`, NLRI is
`internal/core/bgp/nlri/`, and bridge is `internal/exabgp/bridge/`. A preserved
site keeps its existing carrier and reads the explicit presence or negotiated
flag; it does not gain a value-based absence test.

| Carrier / AC | Writers and carriers found in source | Readers and migration or preservation decision |
|--------------|--------------------------------------|------------------------------------------------|
| Peer-AS identity, AC-11 | `pkg/plugin/rpc/remote_as.go`: `RemoteASOf`, `ASN`, `MarshalJSON`, `UnmarshalJSON`, `String`, `LogValue`, `IsZero`; `internal/component/plugin/types_bgp.go`: `PeerInfo.PeerAS` | The typed unknown survives an unfilled value and reflective JSON/text/log rendering. `TestRemoteASRendersThroughReflectiveConsumers` and `TestRemoteASDecodesWhatItEncodes` in `pkg/plugin/rpc/remote_as_test.go` distinguish known AS, omitted/null unknown and invalid AS zero. |
| Peer-AS event producers, AC-11 | BGP `reactor/reactor_notify.go`: `notifyPeerNegotiated`, `emitCongestionEvent`, `notifyOneMessage`; `reactor/peer_fsm_events.go`: `fsmEventPeerInfo`; `reactor/peer_run.go`: `runOnce`; `reactor/reactor_api.go`: `establishedPeerInfo`, `apiStateObserver.OnPeerClosed`, `reactorAPIAdapter.Peers`; `reactor/peer_removal.go`: `removalLocked`; `reactor/session_wire.go`: `dispatchObservedWire` | All construct or refresh `PeerInfo` through `RemoteASOf`; negotiation supplies the known value, while pre-negotiation events retain unknown. `TestKeepaliveNamesTheNegotiatedASOfADynamicPeer` and `TestPreNegotiationEventsDoNotNameAS0` in `reactor/reactor_notify_test.go` cover both states. |
| Typed event transport, AC-11 | BGP `server/events.go`: `getStructuredEvent`, `getStructuredStateEvent` copy the typed peer AS to `pkg/plugin/rpc/bridge.go` `StructuredEvent`; `PutStructuredEvent` clears it to typed unknown | DirectBridge readers retain the same distinction as JSON readers. BGP `plugins/rib/rib_structured.go` `handleReceivedStructured` and `plugins/rpki/rpki.go` `handleStructuredUpdate` refuse UPDATEs with no known AS. `plugins/rr/rr.go` `handleStructuredState` retains the last learned AS, and `plugins/rs/server.go` `parseStructuredState` sets an AS only when known. `plugins/bmp/bmp_events.go` `peerHeaderFromEvent` refuses a header with unknown AS. |
| Native JSON peer object, AC-11 | BGP `format/text.go` `appendPeerJSON` and `format/json.go` `peerMap` write `remote.as` only when `ASN` is known; event encoders reuse them | BGP `event.go` `GetPeerASN` parses the remote object into typed known/unknown and returns unreadable-object errors. `plugins/rib/rib.go` `handleReceived` refuses unknown UPDATE identity; `plugins/adj_rib_in/rib_validation_config.go` `validationPeerFromEvent` carries typed identity. `TestReceivedUpdateWithUnknownPeerASIsRefusedOnBothRails` in `plugins/rib/rib_unknown_peer_as_test.go` preserves known-AS route storage on both JSON and structured paths. |
| Native text peer header and OPEN, AC-11 | BGP `format/session_events.go` `appendRemoteASText`, `appendPeerTextPrefix`, `AppendConnected`, `AppendFSM`, `AppendSignal`; `format/text.go` `AppendOpen`, `AppendNotification`, `AppendKeepalive`, `AppendRouteRefresh`, `AppendEOR`, `AppendCongestion`; `format/text_human.go` `appendFilterResultText` / `appendStateChangeText` | Unknown is spelled `remote as unknown`; `AppendOpen` writes the OPEN's own ASN separately. BGP `plugins/rs/server_text.go` `parseTextOpen` reads the OPEN ASN even when the remote header is unknown; `parseTextState` keeps its state header parsing. `TestTextOpenRemoteASNamesThePeer` and the dynamic-OPEN cases in `plugins/rs/server_test.go` cover the split. |
| Native monitor reader, AC-11 | BGP JSON event writers feed `plugins/cmd/monitor/format.go` `formatMonitorLine`; `monitorEvent` now uses `rpc.RemoteAS` | Unknown and null identity render AS unknown; known identity keeps its AS, and invalid AS values preserve the full raw event. `format_remote_as_test.go` covers producer-generated OPEN/state/notification events, null and malformed input. Selected race controls, the full monitor package and actual SSH PTY/BGP `internal/test/fixture/monitor_remote_as_stream.go` `monitorRemoteASStream` pass. Its promoted fixture and paired forked RPKI fixture pass 2/2 after 80 native stress invocations; main also observed the actual unknown-AS-to-AS0 negative fail before peer OPEN. |
| JSON RPKI admission reader, AC-8/11 | BGP `plugins/rpki/rpki.go` `handleEvent` reads `GetPeerASN` and `ASN` before JSON projection; `SetMetricsRegistry` registers `ze_rpki_updates_refused_total` | Unknown and malformed UPDATE identity is counted by reason and reaches no validation, tracker or emitted event. A known AS 100 still produces valid origin/ASPA decisions, tracker entries and a validation event. State-down cleanup precedes UPDATE admission and remains ASN-free. `peer_as_admission_test.go` `TestJSONUpdateRequiresPeerAS` / `TestJSONPeerDownWithoutASClearsTrackedRoutes` pass in selected race proof. The full RPKI package passes after the independently reviewed after-join fixture cleanup, and `test/plugin/rpki-event-valid-forked.ci` passes the actual known-AS JSON transport path. Session admission and native JSON encoding justify the malformed-identity handler seam, as recorded above. |
| EPE state reader, AC-11 | BGP `plugins/epe/epe_source.go` `Source.State` reads `PeerInfoJSON` for an up event | Preserve the existing local/remote AS-zero refusal before storing a session or publishing its link-state identity. A valid up identity still populates the session; a non-up event removes its session before requiring AS fields. This site already has the needed admission guard and is retained as a permitted source-based survivor; no new EPE execution is claimed. |
| Peer-AS command consumers, AC-8/11 | BGP `plugins/cmd/peer/peer.go` `setRemoteAS`, `filterPeersBySelectorValue`; `health.go` `handleShowBGPHealth`; `prefix_update.go` `handleBgpPeerPrefixUpdate`; `plugins/cmd/policy/handler.go` `filterPeersByPolicySelector`; `config/loader.go` `peerLabel`; `internal/component/plugin/server/command.go` `selectorMatchesPeer` | Each asks `ASN` before using a number. Unknown identity remains unavailable to numeric-AS selection and prefix-update lookup; rows and labels do not publish AS zero as a learned identity. This preserves address/name selection and ordinary known-AS answers. |
| MRT peer-AS boundary, AC-11 | `internal/plugins/mrt/dump.go` `peerInfoToHeader` reads typed AS; BGP reactor wire observers retain raw BGP messages | Preserve MRT's existing undefined-AS zero marker at this format boundary; it is not fed back as a negotiated peer identity. `rfc6396_component_test.go` and the BGP `reactor/rfc6396_wire_capture_test.go` consumers retain the MRT contract. |
| ExaBGP JSON/text bridge, AC-10/11 | Bridge `bridge_event.go` `ReadEvent`, `Event.exabgpPeerASN`, `exabgpNLRIObject`, `exabgpPathInformation`; `bridge_event_json.go`, `bridge_event_text.go`, `bridge_event_state.go`, `bridge_event_fsm.go`; `bridge_wire_json.go` `WireUpdateToExabgpJSON` constructs typed identity from session facts | Preserve ExaBGP's peer-AS auto value zero when native Ze omits an unknown AS; known AS remains numeric. Native `path-id` becomes the ExaBGP `path-information` spelling, including zero. `TestUnknownPeerASIsExabgpAuto` in `bridge_event_fsm_test.go` checks both unknown and known encodings; existing bridge NLRI tests preserve the compatibility shape. |
| Unchanged Python readers, AC-10/11 | Bridge line encoders remain the writers seen by `test/exabgp-compat/etc/run/exabgp_api.py`, its `old-5.0` counterpart and `exabgp_packets.py` | `API.read_line` and `API._record` consume newline-framed ExaBGP `neighbor`/`message` objects; packet helpers compare ExaBGP JSON. Current and old `*.run` programs read those lines, and `stderr-1.py` / `syslog-1.py` forward them as text. Preserve that envelope and peer-AS auto zero through the bridge; these scripts do not migrate to native `RemoteAS` or the native Ze peer object. |
| Unchanged Python command writers and fixture readers, AC-10 | `test/exabgp-compat/{etc,old-5.0/etc,old-4.2/etc}/run/*.run`, including `api-attributes-path.run`, emit `path-information` commands; `test/plugin/addpath-identifier-zero-forwarded.ci` carries explicit zero-ID octets | Bridge command translation continues to feed the registered update grammar. BGP `plugins/cmd/update/update_text.go` `parsePathInfoFlat` records keyword presence in `HasPathID`; `plugins/cmd/update/update_text_nlri.go` `parseINETSection`, `parseINETNLRI`, `parseVPNNLRI`, `parseLabeledNLRI` and registry section parsers carry it. The functional fixture's receiver checks the four zero-ID octets; no Python or shell consumer is changed to infer presence from the value. |
| ADD-PATH presence authority, AC-3/10 | NLRI `base.go` `prefixNLRI.HasAddPath`; `inet.go` `NewINET`, `ParseINET`; `wire.go` `WireNLRI.HasAddPath`; `nlri.go` `HasPathID`, `SplitPathID` | BGP `rib/route.go` `Route.Index` asks `HasPathID`. BGP `route.go` `RouteKey` takes the presence flag, and `Route.AddPath` distinguishes a zero identifier from none. `TestAddPathIdentifierZeroIsAnIdentifier` in `rib/route_test.go` covers zero, maximum and absent identity. |
| BGP NLRI wire writers, AC-10 | NLRI `nlri.go` `WriteNLRI`, `LenWithContext`; BGP `reactor/reactor_api_batch.go` `writeBatchNLRI`; `message/update_build.go` `BuildUnicast`, `buildMPReach`; `message/update_build_grouped.go` `BuildGroupedUnicast`; `message/update_build_vpn.go` `buildVPNNLRIBytes`; `message/update_build_labeled.go` `BuildLabeledUnicastNLRIBytes` | Preserve destination-negotiated framing, independent of identifier value. `reactor/reactor_wire.go` `writeAnnounceUpdate` / `writeWithdrawUpdate` and `cli/encode.go` `encodeUnicastRoute` keep passing the negotiated flag to `WriteNLRI`; their locally constructed INET payloads carry no source identifier. The actual FRR checker independently reads RX 0/RX 1; its observed omitted-zero negative and clean are recorded above. |
| Other local INET constructors, AC-10 | BGP `plugins/cmd/announce/announce.go` `handleAnnounceUnicast` / `handleAnnounceBlackhole`, `plugins/cmd/commit/commit.go` `handleNamedCommitWithdraw`, `internal/perf/sender.go` `buildInlineBatch`, `buildMPBatch`, `buildWithdrawBatch`, `buildForceMPRoute` | Explicit false means the local payload has no source Path Identifier. Preserve the destination writer's independent negotiated framing; a local numeric zero does not suppress destination ADD-PATH. |
| RIB storage, show and replay, AC-10 | BGP `plugins/adj_rib_in/rib.go` structured/JSON installers and `rib_complex.go` `installComplexNLRIs` store `RawRoute.HasPathID`; `rib_commands.go` `show` uses it for keys. `plugins/rib/rib.go` `handleSent` / `sentNLRIHasPathID` recover JSON member presence; `rib_structured.go` sent/received handlers retain the session flag | `plugins/rib/rib_pipeline.go` `serializeRouteItem`, `rib_replay.go` `collectGroupedRibOutRoutesFiltered` / `formatCursorCommands`, and BGP `format.go` `FormatAnnounceCommand`, `FormatWithdrawCommand`, `RouteNLRIHex`, `writeNLRIModifiers` use `Route.AddPath`. `ribout_entry.go` `reconstructRoute` retains it. The canonical zero-ID tests in `plugins/rib/ribout_native_test.go`, `plugins/adj_rib_in/complex_key_test.go` and BGP `format_test.go` cover those consumers. |
| ADD-PATH event renderers, AC-10 | BGP `format/text_json.go` `appendNLRIJSONValue`, `appendNLRIJSON`, `appendUndecodedNLRIJSON` use `HasPathID`; `format/text_update.go` raw IPv4/full UPDATE writers use the encoding context | BGP `event.go` `parseRawFields` / `parseFamilyOps` retain family flags; `nlri.go` `ParseNLRIValue` extracts the value while callers retain presence separately. `policychain/external_intents.go` `announcedPrefixValue` uses a pointer for an optional `path-id`. The bridge reader above preserves zero; the fixture reader `internal/test/fixture/plugin_fixture_02_events.go` checks member presence. `format/text_test.go` covers zero, maximum and no-ADD-PATH rendering. |
| VPN, labeled and EVPN renderers, AC-10 | BGP `plugins/nlri/vpn/types.go` `VPN.HasAddPath`, `vpn.go` `vpnToJSON` / `formatVPNTextSingle`; `plugins/nlri/labeled/types.go` `LabeledUnicast.HasAddPath` / `String`, `json.go` `AppendJSON`; `plugins/nlri/evpn/types.go` concrete `HasAddPath` methods and `plugin.go` `evpnToJSON` | Parse-time flags decide presence; VPN/labeled text and JSON retain identifier zero. Canonical cases are in `vpn/rfc4659_vpn_test.go`, `labeled/types_test.go` (`TestLabeledUnicastStringKeepsAPathIdentifierOfZero`) and `format/text_test.go`. Payload `WriteTo` still excludes the outer identifier, which `WriteNLRI` supplies. |
| Other registered NLRI decoders, AC-10 | BGP `plugins/nlri/{flowspec,ls,mup,mvpn,rtc,srpolicy,vpls}` decoder entry points receive `addPath`; MVPN `sectionJSON` keeps the outer identifier beside its decoded NLRI. Core `flowspec` / `bgpls` and plugin MUP/MVPN/RTC/SRPolicy/VPLS `HasAddPath` methods retain their payload-level answer | Preserve the distinction between the decoded family payload and its outer framing. `DecodeNLRIHex`, `decodeBGPLSNLRI` and MVPN section walkers publish `path-id` from the supplied flag, including zero, even where the payload object itself carries none. No family is newly promised ADD-PATH support by adding the accessor. BGP `server/codec.go` registry decoder handlers continue forwarding the encoding flag. |
| Existing forwarding/validation readers, AC-10 | BGP `plugins/rs/server_inventory.go` `extractWireNLRIRecords`, `appendParsedRecords`, `appendOpaqueRecords`; `plugins/rr/withdrawal.go` wire/parsed walkers; `plugins/bmp/bmp_replay.go` `cacheAdjNLRIs`; `plugins/rpki/rpki.go` validation/tracker walkers | Preserve source-context and parsed-object framing rather than migrating these readers to identifier tests. Current `policyview.View.AddPath`, policychain section walkers, reactor forwarding/provenance/validation walkers, relay/recovery and sent-owner paths continue using explicit context or stored framing flags. The changed accessor does not replace the negotiated destination writer. Unknown source context is an explicit refusal in the RS regression named by ledger row 51. |
| Malformed NLRI / withdrawal action, AC-9/10 | BGP `format/text_json.go` `appendUndecodedNLRIJSON` publishes raw/error information instead of a placeholder route; `reactor/forward_build.go` `buildWithdrawalPayloads` splits the two-family withdrawal | Preserve decodable sibling routes and withdraw each refused family through its proper field. `TestUndecodableNLRIIsNeverWrittenAsARoute` and `TestMixedFamilyUpdateRefusedIsWithdrawnPerFamily` cover these paths. The committed builder returns an explicit error, and the QEMU advertised-prefix withdrawal path passes. The final report still finds `fwdWithdrawal.of` converting that error to nil; cause propagation to both callers remains assigned. |
| Raw command payload and RPC refusal, AC-8 | `internal/component/plugin/types.go` `RawJSON.MarshalJSON`; `dispatch.go` `builtDocument`, `WriteAnswer`, `writeUnbuiltAnswer`; `server/dispatch.go` `responseToDispatchOutput` | Objects, arrays and the no-data null form survive; invalid JSON and top-level scalars produce an error answer. `server/command.go` `routeToProcess` reads `pluginipc.ExecuteCommandValue` and preserves its error verdict. `TestRawJSONRefusesNonJSON`, `TestRawJSONRefusesAScalar` and `TestWriteAnswerRefusedPayloadStillAnswers` cover construction and the real terminator, so a refused scalar does not strand the caller. |
| RIB pipeline / graph answer, AC-8 | BGP `plugins/rib/rib_pipeline.go` `showPipeline`, `rib_pipeline_best.go` `bestPipeline`, `rib_topology.go` `refusePipelineArgs`, `commandAnswer`, `graphTerminal.drain` | Invalid arguments become an error verdict through `commandAnswer`. Valid graph output is a structured graph document; `plugins/rib/register.go` registers `renderGraphText`, and `internal/component/command/text_renderer.go` `applyRegisteredText` renders its key for text while JSON keeps the structure. `TestPipelineParseErrorsReachCommandVerdict`, `rib-filter-show.ci` and the owner-approved graph fixtures preserve useful answers. |
| LG measured winner and session names, AC-8/12 | BGP `plugins/rib/rib_pipeline_bestfact.go` `bestFactOf`, `writeBestFact`; `rib_pipeline.go` `serializeRouteItem` writes the recorded received-route best fact | `internal/component/lg/handler_api.go` `transformRoutes` maps `best` to birdwatcher `primary`; `protocolSession` resolves listed names or addresses before route/count/export queries. `TestShowRIBRowsSayWhichPathWon` and `TestAPIProtocolNameSelectsItsSession` keep the useful winner and listed-session answers. An unmatched session is 404; a known losing route remains primary false. |
| BMP Loc-RIB payload, AC-12 | BGP `plugins/rib/rib_bestchange.go` best-change publication carries attributes in `internal/core/bgp/ribevents/ribevents.go`; `plugins/bmp/bmp_locrib.go` `buildLocRIBUpdateBody`, `handleBestChange` encode and fan out the UPDATE | pmacct reads the best path's actual attributes. Missing announce attributes refuse; valid announces and withdrawals remain sent. `TestLocRIBRouteMonitoringCarriesTheBestPathAttributes`, `TestLocRIBAnnounceWithoutAttributesIsRefused`, the new send-failure control and native BMP/pmacct run cover the producer and peer. |
| Health/date answers, AC-8/12 | `internal/core/health/registry.go` `RegisterWhen` / `invokeCheck`, L2TP `checkHealth`, VPP `checkVPPHealth`, trafficusage `probeTCX` / `trafficUsageDiagnostic`; `internal/component/cmd/show/system.go` `handleShowSystemDate`, registry `GetNTPSyncInfo` | Preserve measured healthy/zero answers and omit inapplicable checks; failed measurement reports its cause. An absent NTP provider refuses, while registered inactive NTP contributes no metadata to the still-valid date. Canonical health, TCX and NTP controls are in the retained ledger and completed proof sections. No new native JSON health status is introduced here. |
| Protocol refusals with preserved encodings, AC-9/12 | RADIUS `dispatchResponse` / `verifyResponseMessageAuthenticator`, LDP `EnsureLocal` / `allocateLabelLocked` / `DecodeInit` and session unknown-TLV handling, IS-IS `handleIIH`, RSVP `handlePath` / `rroProtectionFlags`, BFD `pluginService.EnsureSession` | These change admission, notification, counting or measured flags through existing protocol encoders. FreeRADIUS/FRR and the unsigned relay, LDP nft rewrite and named regressions retain valid traffic controls; their wire layouts do not acquire a new field from this spec. The permitted input cases below state what continues to pass. |

## Permitted survivors and useful outcomes

These rows supplement the unchanged ACs. Each names the useful outcome and the
reason the surviving value or input is permitted; the linked canonical tests
supply assertions rather than an absence-only success.

| AC | Useful outcome and permitted survivors | Source contract and canonical control |
|----|---------------------------------------|---------------------------------------|
| AC-1 | A recognized switch case continues to answer its value. An unmatched nil-error result has no permission to pretend recognition; an intentional sentinel needs the named guard of AC-5. | `fileScan.defaultArm` / `fallOff`, `testdata/switch.go`, `TestRefuseByDefaultRefusesEachRuleFixture`. |
| AC-2 | A miss explicitly handed to the caller as a flag or error remains usable; a guessed hit does not. An observed absence remains distinct from an unreadable resource. | `answersPlausibly` / `recordsMiss`, `testdata/miss.go`; `TestInterfaceExistsReadErrorIsNotAbsence` and direct netlink missing-link versus read-error controls. |
| AC-3 | All carried identifier values, including zero and maximum uint32, remain usable through negotiated presence. No value-zero comparison is permitted as an absence decision because the field reserves no absent value. | `HasPathID` and `WriteNLRI`; `testdata/pathid.go` and the canonical zero-ID tests under AC-10. RFC 7911 Section 2: "A BGP speaker that receives a route should not assume that the identifier carries any particular semantics." |
| AC-4 | A healthy answer supported by a measured read remains permitted. A state boolean derived from measured data remains an answer; a literal needs a valid guard rather than pretending measurement. The full race retry after the forward package-alias fix passes; package statement coverage is 86.5%, with eight uncovered profile blocks retained. | `fileScan.function` / `expression`, `testdata/state.go` measured-health control; `TestL2TPHealthHealthyOnlyWithDataPlane`, `TestTrafficUsageReadinessCanFail`. |
| AC-5 | A guarded zero remains valid when its annotation names the visible guard the consumer actually uses. | `annotations` / `parseGuard` / `annotationFor`; `TestRefuseByDefaultAnnotationNeedsGuard` and `TestRefuseByDefaultGuardInSiblingFile`. |
| AC-6 | No unnamed guard survives, because the reviewer cannot identify the consumer that distinguishes the sentinel from an answer. | `RuleAnnotationNoGuard`, `TestRefuseByDefaultAnnotationNeedsGuard`. |
| AC-7 | The tree report continues listing unrelated refused and annotated shapes so their owners can judge them. Its exit zero is a reporting convention, not acceptance. Main's final named report joins to zero owned matches against all 54 rows and 125 producer entries in both populations, with configure/verify callback AST anchors validated. | `Report` / `runReport`, `TestRefuseByDefaultReportListsTree`, `job-refusal-final-named-report-45a00bc9.log` and `scratch/refusal-ac7-final-125-comparison.json`. |
| AC-8 | Successful reads, typed not-found, no requested CoS profile, a valid pipeline and an inactive registered NTP provider remain usable. These are real outcomes, unlike failed measurement or an absent provider. Valid command objects/arrays and no-data answers also survive scalar refusal. | `TestResolveAbsentProviderPermitsNoProfile`, `TestNetlinkGetStatsZeroCounters` / `MeasuredCounters`, `TestSystemDateAcceptsInactiveNTPProvider` / `CarriesMeasuredNTPState`, valid pipeline/count controls, and `RawJSON` controls above. |
| AC-9 | JSON/JSON_IETF gNMI Gets, valid debug syntax, an LDP unknown U=1 TLV, a free last label, a valid IS-IS state, complete RSVP PATH and IPv6 BFD with bind-v6 remain admitted. A non-EAP unsigned RADIUS answer survives only the existing explicit unsigned-response policy; it does not exempt unsigned EAP. | `TestGetAnswersEachJSONEncoding`, gnmic interop; LDP U=1 controls and `TestLDPLabelExhaustionRefusesSecondBinding`; IS-IS/RSVP controls; `TestIPv6SessionAcceptedWithBindV6`, `TestIPv4MappedPeerNeedsNoBindV6`; unsigned non-EAP control in `TestRadiusDiscardsEAPAnswerWithoutMessageAuthenticator`. RFC 5036 Section 3.3 permits U=1 processing while requiring U=0 notification and message rejection. |
| AC-10 | Under ADD-PATH, zero and maximum are identities; without it the prefix stays free of an identifier regardless of the stored number. Re-advertisement retains negotiated framing and Ze's destination identifier. | `TestAddPathIdentifierZeroIsAnIdentifier` in each canonical consumer, `TestLabeledUnicastStringKeepsAPathIdentifierOfZero`, `addpath-identifier-zero-forwarded.ci`, and the actual FRR baseline/negative/clean. RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE message, the NLRI encoding MUST be extended by prepending the Path Identifier field, which is of four octets." |
| AC-11 | Fresh LLGR routes survive missing stale state; thermal 0.0 is measured data; a clear names the resolved victim; a known peer AS stays numeric and a pre-negotiation event stays unknown. ExaBGP peers retain their documented auto-AS zero spelling at the compatibility boundary. Monitor preserves unknown as text and malformed input as raw data; RPKI still validates known-AS UPDATEs and removes routes on ASN-free state-down. EPE preserves its existing AS-zero up-event refusal while admitting valid identity and cleaning up non-up sessions. | `TestLLGREgressFilter_NilStateDoesNotSuppressFreshRoute`, thermal/DDoS/dynamic-AS tests in the ledger, `TestUnknownPeerASIsExabgpAuto`, monitor `TestFormatMonitorLineRemoteAS`, `TestFormatMonitorLineRemoteASNull` and `TestFormatMonitorLineRemoteASInvalid`, RPKI `TestJSONUpdateRequiresPeerAS` / `TestJSONPeerDownWithoutASClearsTrackedRoutes`, and EPE `Source.State`. These zeros have distinct explicit contracts, and none supplies a fabricated learned peer AS. |
| AC-12 | Measured readiness, actual addresses, the recorded best/losing route, the best path's own BMP attributes and established-bypass RRO flags remain reportable. An inapplicable health check has no row; failed measurement keeps its reason. A real all-zero counter record remains valid. | L2TP healthy/data-plane controls, root/non-root TCX probe, VPP address tests, `TestShowRIBRowsSayWhichPathWon`, BMP attribute and healthy-collector controls, `TestRFC4090FlagsSetOnceBypassEstablished`, direct netlink zero/measured controls. |
| AC-13 | Each owned row's normal input continues to produce the useful outcome its canonical regression names; each representative defect input must still discriminate against its producer break. Dependency-owned rows retain their separate owner and proof. | The 54-row ledger names each producer and regression. Historical reds remain attributed to `238bdb62e9`; later repairs retain their exact observed negative/clean evidence. The remaining RFC functional, loaded-run and closure-check obligations remain open despite source review and AC-7 being clean. |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The four rule shapes below cover every owned row | row-by-row reading 2026-10-23 | an owned row has no rule; add one before closing | each owned row's regression test plus a fixture per rule | broken as stated: dropped-error handling required a fifth rule and direct state writes required file-local assignment tracking; round-1 semantic defects also require producer proofs beyond the syntactic report |
| A-2 | A change-set check produces few false positives on new code | `arch compound-guard` precedent judges unpushed lines | authors meet a noisy check and annotate by reflex | run the check over the last 200 commits in report mode; count findings per true defect | confirmed 2026-10-23: `report` findings intersected with the lines `git diff -U0 a870c72164` (HEAD~200, to the working tree) changed: 10 findings over 200 commits, 6 true (a read failure or a wrong type read as "no" or absent: `filter_family/config.go` `exportChain`, `le/commit/owed_trailers.go` `answering.answered` x2 and `answering.reviewed`, `le/hookruntime/lifecycle.go` `linkedWorktree`, `le/verify/green.go` `provenParts`), 4 relied-on zeros that take the annotation (`reactor/policy_reload.go` `reconcileExportFinal` and `flushPolicyReplay`, `green.go` `recordGreen`, `exabgp/bridge/bridge_route_forms.go` `refuseMaskNotNumber`), 0 pure false positives: 1.7 findings per true defect. Reached after five precision cuts, each pinned by a fixture case: `false` beside a nil error (the flag hands the miss on), a branch that records the miss (field or element write, counter step, error handed to an outer variable), an answer that carries the error, a parse's own "no" (`Parse*`, `Atoi`), and a bool function literal (visitor, yield, poll predicate). The first cut measured 28 findings over the same range |
| A-3 | Every ADD-PATH consumer has the negotiated flag at hand | `AddPath` fields already present at the `format.go` and `rib_pipeline.go` sites | a consumer needs the flag threaded | gopls references on each value test | confirmed with exception: the `format.go` and `rib_pipeline.go` sites hold `AddPath`; four further sites found (`bgp.RouteKey`, the adj_rib_in show key, the `appendNLRIJSONValue` bare-prefix branch, `handleSent`), and the flag is threaded for `NewINET` callers (new `hasPathID` parameter) and `adj_rib_in.RawRoute.HasPathID`. Presence accessors for the AC-3 check: `nlri.HasPathID(n)` for NLRIs, `Route.AddPath` for `bgp.Route`, `RawRoute.HasPathID` |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Refusing where Ze accepted before breaks a working interop | interop red after a row fix | the RFC sentence decides; where the RFC is silent read FRR and BIRD and follow both |
| R-2 | Health checks reporting Unknown alarm operators | doctor output change | Unknown carries the reason and the command that measures it (`ai/rules/user-facing-errors.md`) |
| R-3 | Annotation becomes a reflex escape | many `refuse-by-default` annotations in one commit | the annotation requires a named guard and the check prints every annotation in the change set for the reviewer |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a producer refuses a value that was valid: a config rejected, a packet discarded, a show error |
| How is it reverted? | one commit per family; the check is one stage line |
| Who else touches this path? | `spec-plugin-optional-dependency-refusal` (startup autoload), `spec-startup-deferred-reapply-idempotency` (`interfaceExists` guards), the S1 schema spec (`validateYangType`) |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default check` over a change set holding a fixture of each rule | → | `refusebydefault.Check` | `TestRefuseByDefaultRefusesEachRuleFixture` |
| `./le verify current` stage list | → | `stages.go` stage `arch refuse-by-default` | `TestVerifyStagesIncludeRefuseByDefault` |
| RADIUS Access-Accept with EAP-Message and no Message-Authenticator on the socket | → | `dispatchResponse` | `TestRadiusDiscardsEAPAnswerWithoutMessageAuthenticator` |
| config naming a cos profile in a build without the cos plugin | → | actual `ze config validate` in a `ze_core && !ze_cos` build, with resolver defense checked separately | `internal/test/cosabsent/cos_absent_integration_test.go` (`TestConfigValidateWithoutCoSRefusesProfile`, `TestConfigValidateWithoutCoSAcceptsInlineMaps`, `TestInlineQoSSchemaKeepsItsFourInterfaceKindsWithoutCoS`); `cos.TestResolveNoResolverRefusesProfile` |
| LDP FEC allocation with the label space full | → | `EnsureLocal` through `allocateLabelLocked` | `TestLDPLabelExhaustionRefusesSecondBinding` |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A change adds a `default:` arm, or a fall-off after a switch, in a function with an `error` result that returns a nil error | `./le arch refuse-by-default check` refuses it, naming file, line and the rule |
| AC-2 | A change adds a branch on a lookup miss (`!ok` from a map index or type assertion, a nil registered function, a non-nil error from a read) that returns `true`, a zero value, or nil error | refused, naming the rule |
| AC-3 | A change compares an ADD-PATH path identifier with zero | refused, naming the negotiated-presence accessor to use |
| AC-4 | A change writes a state or health answer from a literal (`StatusHealthy` with no measured read in the function, a literal boolean into a state member) | refused |
| AC-5 | The same shapes carrying a `refuse-by-default:` annotation that names the guard the zero serves | accepted, and listed in the check's output |
| AC-6 | An annotation with no named guard | refused |
| AC-7 | `./le arch refuse-by-default report` over the tree | lists every instance in the tree; none of the owned rows' producers appears in it after this spec |
| AC-8 | Family F1 (lookup miss read as an answer): an unreadable link, a missing cos resolver, an unreadable instance name, a failed RPC publication, a destination the RIB never saw, a web interface read failure | each answers an error naming what could not be read and the command that fixes it; none answers "absent", empty or the default |
| AC-9 | Family F2 (unrecognised input accepted): a gNMI Get with PROTO encoding, `set debug module m level debug flag x`, an LDP Init with an unknown U=0 TLV, an IS-IS Hello with an invalid three-way state, an RSVP PATH with no SENDER_TSPEC, an IPv6 BFD session on a loop with no IPv6 socket, a RADIUS EAP answer with no Message-Authenticator, label space exhausted | each is refused or discarded as the quoted RFC sentence says (LDP: notification and message ignored; IS-IS and RADIUS: PDU discarded; LDP labels: no second binding), with a counter or an error naming the cause |
| AC-10 | Family F3 (zero as identity): ADD-PATH negotiated and a route with path identifier 0 | indexed, rendered and re-advertised as carrying identifier 0; without ADD-PATH no identifier is shown whatever the value |
| AC-11 | Family F3: a nil LLGR egress state, a 0.0 degC thermal reading, a DDoS clear for a resolved victim, a KEEPALIVE for a dynamic peer before its AS resolves | nil egress state refuses the stale route toward a non-LLGR peer; 0.0 is reported as a reading; the clear closes the incident; the event names the AS or says it is not yet known |
| AC-12 | Family F4 (constant or config reported as measured): l2tp health, traffic-usage eBPF readiness, LG `primary`, BMP Loc-RIB Route Monitoring, RSVP RRO local protection, VPP interface addresses | each reads the measured state; where it cannot, it answers Unknown with the reason |
| AC-13 | Every owned row in the table below | has a regression test that was red against the producer before its fix; the row's representative input no longer reproduces |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRefuseByDefaultRefusesEachRuleFixture` | `internal/le/arch/refusebydefault/refusebydefault_test.go` | AC-1 to AC-4, one testdata fixture per rule | |
| `TestRefuseByDefaultAnnotationNeedsGuard` | same | AC-5, AC-6 | |
| `TestRefuseByDefaultReportListsTree` | same | AC-7 | |
| `TestRadiusDiscardsEAPAnswerWithoutMessageAuthenticator` | `internal/component/radius/rfc3579_client_test.go` | AC-9, RFC 3579 Section 3.1 | |
| `TestLDPLabelExhaustionRefusesSecondBinding` | `internal/plugins/ldp/rfc3031_label_test.go` | AC-9, RFC 3031 Section 3.14 | |
| `TestLDPInitUnknownTLVUClearNotifies` | `internal/plugins/ldp/rfc5036_init_unknown_tlv_test.go` | AC-9, RFC 5036 Section 3.3 | |
| `TestISISHelloInvalidThreeWayStateDiscarded` | `internal/plugins/isis/hello_invalid_state_drop_test.go` | AC-9, RFC 5303 Section 3.2: `handleIIH` counts the discarded Hello | |
| `TestRFC5303InvalidStateLeavesAdjacencyUntouched` (existing, RFC5303-3.2-7) | `internal/plugins/isis/adjacency/rfc5303_invalid_state_test.go` | AC-9, RFC 5303 Section 3.2: the FSM discards before mutation | |
| `TestRFC2205PathWithoutSenderTSpecRefused` (existing, tagged), `TestRFC2205PathWithoutSenderTSpecDropped` (existing) | `internal/plugins/rsvpte/rfc2205_path_test.go`, `repairs_rfc2205_test.go` | AC-9, RFC 2205 Section 3.1.3 | |
| `TestRadiusUnsignedEAPDiscardCounted` | `internal/component/radius/metrics_test.go` | AC-9: the discard reaches the `ze_radius_unsigned_eap_discards_total` series | |
| `TestRFC4090FlagsClearUntilBypassEstablished`, `TestRFC4090RefreshTracksBypassState` (existing, RFC4090-6-3, 6-4) | `internal/plugins/rsvpte/rfc4090_frr_bypass_test.go` | AC-12: RRO local protection follows the bypass LSP's state | |
| `TestAddPathIdentifierZeroIsAnIdentifier` | `internal/component/bgp/rib/route_test.go` and each consumer package | AC-10, RFC 7911 Sections 2 and 3 | |
| `TestLLGREgressFilter_NilStateWithdrawsEBGP`, `_NilStateDepreferencesIBGP` (existing, RFC9494-4.3-3, 4.6-2, 4.6-3; planned as `TestLLGREgressNilStateRefusesStaleRoute`) | `internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go` | AC-11, RFC 9494 Section 4.3 | -> Decision: fixed at 6ec2a10bcf with these tests; a second test of the same branch would duplicate them |
| `TestDetectThermal_ZeroReadingIsAReading` | `internal/component/host/thermal_linux_test.go` | AC-11 | |
| `TestClearedCarriesTheDetectedVictim` | `internal/plugins/ddos/detect/characterize_test.go` | AC-11 | |
| `TestKeepaliveNamesTheNegotiatedASOfADynamicPeer` | `internal/component/bgp/reactor/reactor_notify_test.go` | AC-11 | |
| `TestInterfaceExistsReadErrorIsNotAbsence` | `internal/component/iface/config_apply_test.go` | AC-8 | |
| `TestL2TPHealthReadsDataPlane`, `TestTrafficUsageReadinessCanFail` | owning packages | AC-12 | |
| one regression test per remaining owned row, named in the row's fix commit | owning packages | AC-13 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| LDP local label | 16 to 1048575 | 1048575 (last free) | 15 | none free: refusal |
| ADD-PATH identifier | 0 to 4294967295 | 0 and 4294967295 both valid | N/A | N/A |
| BGP OPEN size is not owned here | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestConfigValidateWithoutCoSRefusesProfile`, `TestConfigValidateWithoutCoSAcceptsInlineMaps` | `internal/test/cosabsent/` | actual daemon built without CoS refuses the unavailable profile setting and still accepts inline VLAN maps | green under race, main-thread log `tmp/session/2026-11-11-004105/scratch/job-cos-absent-integration-green-3454b9a9.log`; binary build tags are checked before execution; schema exposure retains the original four interface kinds |
| `addpath-identifier-zero-forwarded` | `test/plugin/` | route with identifier 0 under ADD-PATH reaches the peer with four identifier octets | |

The proposed `test/plugin/show-bgp-peer-bare-list.ci` belongs to
`plan/release/fixes/spec-web-cli-config-one-off-defects.md` AC-20, as recorded in
`plan/handover/24-refuse-by-default-implemented-awaiting-closure.md`, Closure
checklist. That existing ownership removes the duplicate deliverable here; it
does not claim that the absent fixture was created or run by this spec.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `addpath-zero-id-frr` | `test/interop/scenarios/`, checkers in `internal/le/interoplab/bgp/` (`scenarioAddPathZeroIDFRR`) | FRR | identifier 0 round trip under ADD-PATH | matched logging-only baseline 1/1; omitted-zero writer negative fails with RX 1 surviving and RX 0 absent; unmodified-source clean 1/1, all on 2026-11-11. Exact commands, limits and logs are in Completed native interop evidence. |
| `radius-admin-eap-unsigned-freeradius` | `test/interop-radius/scenarios/`, checker `internal/le/interoplab/radius/check_unsigned_eap.go` | FreeRADIUS 3.2.10, through `test/interop-radius/unsigned-relay.pl` in its container (FreeRADIUS's `rad_encode` always signs over UDP) | EAP answer without Message-Authenticator is discarded, logged and counted; the same login with the signature kept completes | green 2026-10-23 (with the four other radius scenarios) |
| `ldp-init-unknown-tlv-frr` | `test/interop/scenarios/`, checker `internal/le/interoplab/bgp/check_ldp_init_unknown_tlv.go` | FRR ldpd; an nft rule in ze's container rewrites one TLV type in FRR's real Init (FRR cannot send an unknown TLV) | unknown U=0 TLV in Init draws an Unknown TLV notification FRR receives, and the Init is ignored (no OPERATIONAL until the rewrite is removed) | green 2026-11-11, 1/1; `job-refusal-ldp-interop-632ff3c0.log` |
| `bgp-addpath-best-path-pmacct` | `test/interop/scenarios/`, registered checker under `internal/le/interoplab/bgp/` | pmacct with the catalog's BGP peers | BMP collector receives the best-path Route Monitoring payload | green 2026-11-11, 1/1; `job-refusal-bmp-best-path-interop-632ff3c0.log` |

## Files to Modify
- `internal/component/radius/client.go`, `packet.go` - discard EAP answer without Message-Authenticator
- `internal/plugins/ldp/lib.go`, `wire.go` - exhaustion error; unknown U=0 TLV
- `internal/component/iface/config_apply.go` and every `Backend.GetLink` - typed not-found
- `internal/core/cos/cos.go` - not-built refusal from the dependency spec
- `internal/component/bgp/rib/route.go`, `bgp/format.go`, `plugins/rib/rib_replay.go`, `rib_pipeline.go`, `rib.go`, `format/text_json.go`, `plugins/nlri/vpn/vpn.go`, `internal/core/bgp/nlri/inet.go` - ADD-PATH presence from the capability
- every producer named in the Owned journal rows table
- `internal/le/verify/engine/stages.go` - one stage line
- `docs/architecture/testing/verify-freshness-scope.md`, `docs/contributing/ze-go-style.md` - the new stage and the annotation rule

## Files to Create
- `internal/le/arch/refusebydefault/` - `register.go`, `refusebydefault.go`, `actions.go`, tests and testdata
- `internal/test/cosabsent/cos_absent_integration_test.go` (actual absent-CoS binary, replacing the proposed `.ci` whose runner enables all feature gates), `test/plugin/addpath-identifier-zero-forwarded.ci`
- `test/interop/scenarios/addpath-zero-id-frr/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new config |
| YANG validation constraints | No | `validateYangType` default arm is fixed by refusing; S1 owns the decimal64 type |
| YANG custom validators | No | none added |
| CLI commands/flags | Yes | `./le arch refuse-by-default check` and `report`, registered through `leroot` |
| CLI grammar (keyword before value) | Yes | verbs follow `arch enumeration` |
| Editor autocomplete | N-A | le tool, no editor surface |
| Functional test for new RPC/API | N-A | no new RPC |
| Pipe completeness | Yes | shape `ShapeDoc` like `arch enumeration` |
| Env var registration | No | none |
| Doctor check for runtime dependencies | Yes | l2tp and traffic-usage health become measured; their doctor output changes |
| Prometheus counters/metrics | Yes | discard counters for RADIUS and IS-IS reuse existing per-protocol drop counters where present |
| BGP family surface (new SAFI / capability / attribute) | No | ADD-PATH presence only |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | refusals of existing features |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/architecture/api/commands.md`, `docs/guide/command-reference.md` and `command-catalogue.md`: structured graph answers, pipeline error verdicts, NTP provider refusal; new le actions documented in help/style |
| 4 | API/RPC added/changed? | Yes | gNMI Get refusal of unsupported encoding in `docs/architecture/api/` gNMI page |
| 5 | Plugin added/changed? | Yes | `docs/guide/bmp.md`, `docs/guide/rpki.md`, `docs/architecture/plugin/feature-gates.md`: stored route attributes, identity admission and absent forwarding-provider refusal |
| 6 | Has a user guide page? | Yes | `docs/guide/health-checks.md` for Unknown health |
| 7 | Wire format changed? | Yes | ADD-PATH identifier 0 presence in the wire page that describes ADD-PATH |
| 8 | Plugin SDK/protocol changed? | Yes | Native peer/event identity uses `rpc.RemoteAS`; JSON omits unknown AS, text says unknown, and BestChangeEntry carries owned stored attribute bytes. API/protocol pages and source contracts record these carriers |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc3579.md`, `rfc3031.md`, `rfc5036.md`, `rfc5303.md`, `rfc7911.md`, `rfc9494.md`, `rfc2205.md` rows and `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/verify-freshness-scope.md` stage list |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/contributing/ze-go-style.md`: refusal over default, the annotation |
| 13 | Route metadata keys added/changed? | Yes | `docs/architecture/route-types.md`, `docs/architecture/api/birdwatcher-compat.md`, `docs/guide/bmp.md`: explicit path-id presence, measured best/primary, and stored best-path attributes |
| 14 | Prometheus counters added/changed? | Yes | the subsystem telemetry pages of any counter added; `docs/guide/rpki.md` for `ze_rpki_updates_refused_total` and its `unknown-peer-as` / `malformed-peer-as` reason labels |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `arch refuse-by-default` registration, gNMI encoding advertisement, RPKI refusal metric and gated data-plane writer inventory; main owns the final derived inventory refresh |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | declared by the files named here and affected: `docs/architecture/ldp/mpls-ldp.md` (label exhaustion refusal, U=0 TLV notification), `docs/architecture/route-types.md` (ADD-PATH identifier 0 presence), `docs/architecture/traffic/cos-plugin.md` (refusal with no resolver), `docs/features/interfaces.md` (link read error no longer reads as absent), `docs/research/l2tpv2-ze-integration.md` (health reads the data plane; research page, re-read to confirm it states no current behaviour). Re-run `./le spec citation anchors spec plan/release/fixes/spec-refuse-instead-of-defaulting.md` per phase |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | verify `show bgp peer` examples on the web terminal page |

## Implementation Steps

1. **Phase: Wiring** - the check skeleton registered, the stage line added, fixtures written; `TestRefuseByDefaultRefusesEachRuleFixture` red
2. **Phase: the check** - the four rules over the change set and `report` over the tree; AC-1 to AC-7
3. **Phase: F2 protocol rows** - RADIUS, LDP, IS-IS, RSVP, BFD, gNMI, label exhaustion, withdrawal and IRR cap rows; each red first, RFC quote above the refusal, discrimination record per `ai/rules/rfc-compliance.md`
4. **Phase: F3 ADD-PATH** - one presence accessor, the seven value tests replaced, interop scenario
5. **Phase: F1 rows** - typed not-found and not-built refusals, callers updated
6. **Phase: F4 rows** - measured state and Unknown health
7. **Phase: report clean** - `./le arch refuse-by-default report` lists none of the owned producers

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every owned row has its regression test and a red recorded |
| Correctness | every refusal matches the quoted RFC sentence's action (discard, notify, refuse); no refusal where the RFC says ignore (LDP U=1) |
| Data flow | refusals cross the plugin boundary as error answers |
| Rule: principles | no new zero-as-answer introduced while fixing; each annotation names its guard |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| the check | `./le arch refuse-by-default check` over a fixture commit refuses |
| owned rows fixed | `./le arch refuse-by-default report` plus each row's test |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | RADIUS EAP answers without Message-Authenticator discarded; LDP and IS-IS malformed input discarded without state change |
| Fail open | no health or readiness check can answer Healthy without a measured read |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the producer; if misunderstood, back to research |
| Lint failure | Fix inline |
| Functional test fails | Check the AC: wrong AC to design, correct AC to implement |
| Audit finds a missing AC | Back to the relevant phase |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The not-built half of the family is already designed in the dependency spec; this spec owns the refusal shape, not a second not-built mechanism

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Change-set check with a tree report | a tree-wide gate red from day one | the tree holds instances owned by other specs; a tree-wide red blocks every commit; `arch compound-guard` is the precedent |
| Annotation names the guard | a bare `nolint` style escape | `ai/rules/principles.md` requires a relied-on zero to be named as a guard |
| ADD-PATH presence from the negotiated capability | a sentinel identifier | RFC 7911 reserves no value |
| `show bgp rib ... graph` answers `{"graph":{"nodes":[...],"edges":[...]}}` and refuses over `graph.MaxNodes`; the CLI draws the box through a text renderer keyed by the envelope key `graph`, registered from `bgp-rib` | keep the pre-rendered box text as a JSON string | `ai/rules/cli.md`: a payload is structured data, never finished text; `\| json` must show the structure |

Owner approval (Thomas, 2026-10-24): the functional tests rib-graph, rib-graph-filtered and rib-graph-best (fixture `ribGraph13`, `internal/test/fixture/plugin_fixture_13_rib.go`) may change to assert the structured payload instead of the pre-rendered box text.

## Known Limitations
- The rows of this family owned by `spec-plugin-optional-dependency-refusal` (2026-10-20 census rows: auto-load hard dependency, mrt not built, ddos/exabgp/flowexport undeclared BGP, redistribute and ls_export free strings) stay there. Owner instruction "fix NTP" on 2026-11-11 returns the NTP answer omission to this spec; its absence must be distinguished from a registered inactive provider.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "quoted requirement"` above each refusal this spec adds, using the
sentences quoted in Required Reading.

## Owned journal rows

Owner approval (Thomas, 2026-10-24): the doctor `collectSchemaListeners` row removes the hardcoded listener fallback and deletes `TestDoctorGnmiListenerIsProbed` (`TestDoctorGnmiDefaultEndpointIsProbed` stays); this spec's fix also covers AC-19 of `spec-daemon-runtime-one-off-defects.md`.

| Class file | Date | Surface (short) | Family |
|------------|------|-----------------|--------|
| `registry-contamination.md` | 2026-11-11 | `startValidationReceiver` leaves an enabled owner's refusing lookup | Verification fixture |
| `silent-fall-through.md` | 2026-10-21 | `Server.appendDataPlaneWriter` starts with no FIB writer | F1 |
| `silent-fall-through.md` | 2026-10-20 | `sendPostStartupTo` failure logged at DEBUG | F1 |
| `silent-fall-through.md` | 2026-10-20 | `LIB.allocateLabelLocked` returns a bound label | F2 |
| `silent-fall-through.md` | 2026-10-19 | gNMI `Server.Get` ignores `encoding` | F2 |
| `silent-fall-through.md` | 2026-10-18 | `verifyResponseMessageAuthenticator` true when absent | F2 |
| `silent-fall-through.md` | 2026-10-17 | `BGPConsumer.InjectRoute` warning read as announced=0 | F1 |
| `silent-fall-through.md` | 2026-10-15 | `cos.Resolve` nil maps with no resolver | F1 |
| `silent-fall-through.md` | 2026-10-13 | mrt peer state handlers' failed type assertion | F1 |
| `silent-fall-through.md` | 2026-10-13 | `yangTypeToValueType` maps a union list key to `TypeString`, so `ValidateListKey` never checks its member types | F2 |
| `silent-fall-through.md` | 2026-10-12 | doctor `collectSchemaListeners` falls back to a hardcoded list | F1 |
| `silent-fall-through.md` | 2026-10-08 | debug `setModuleSetting` drops trailing keywords | F2 |
| `silent-fall-through.md` | 2026-10-07 | `captureRawStart` answers done when it cannot capture | F1 |
| `silent-fall-through.md` | 2026-10-02 | rib family with no splitter dropped at Debug | F1 |
| `silent-fall-through.md` | 2026-09-21 | `validateYangType` default arm accepts | F2 |
| `constant-reported-as-measured-state.md` | 2026-10-13 | from-Established FSM arm reports `connection lost` for every cause | F2 |
| `declared-format-contradicts-payload.md` | 2026-08-19 | `RawJSON.MarshalJSON` quotes invalid JSON instead of refusing | F2 |
| `field-carries-two-meanings.md` | 2026-10-23 | text `AppendOpen` writes the OPEN's own AS as `remote as` | F3 |
| `zero-value-as-valid-answer.md` | 2026-10-22 | `appendNLRIJSONValue` writes a placeholder route on decode failure | F2 |
| `zero-value-as-valid-answer.md` | 2026-10-19 | `interfaceExists` any error reads as absent | F1 |
| `zero-value-as-valid-answer.md` | 2026-10-19 | DHCPv6 `dadUnreported` on VPP | F4 |
| `zero-value-as-valid-answer.md` | 2026-10-19 | VPP `detailsToInfo` empty addresses | F4 |
| `zero-value-as-valid-answer.md` | 2026-10-09 | LG `routes/protocol/{name}` 0 routes for a name | F1 |
| `zero-value-as-valid-answer.md` | 2026-09-20 | `serializeRouteItem` path ID compared with zero | F3 |
| `zero-value-as-valid-answer.md` | 2026-09-20 | `vpnToJSON`, `LabeledUnicast.String` path ID compared with zero | F3 |
| `zero-value-as-valid-answer.md` | 2026-09-08 | ddos `emitCleared` zero target never closes | F3 |
| `zero-value-as-valid-answer.md` | 2026-09-06 | `NewINET` path ID 0 as absent | F3 |
| `zero-value-as-valid-answer.md` | 2026-08-10 | gr egress nil state read as nothing stale | F3 |
| `zero-value-as-valid-answer.md` | 2026-08-12 | `readOneHwmon` drops a 0.0 reading | F3 |
| `zero-value-as-valid-answer.md` | 2026-09-25 | resolve `DefaultConfig` read failure as missing | F1 |
| `zero-value-as-valid-answer.md` | 2026-10-08 | `buildParamMeta` error becomes nil | F1 |
| `zero-value-as-valid-answer.md` | 2026-10-19 | web interface detail swaps config for live on error | F1 |
| `zero-value-as-valid-answer.md` | 2026-10-23 | web interface list reads any `ListInterfaces` error as no backend | F1 |
| `zero-value-as-valid-answer.md` | 2026-10-23 | bgp-rib name selector matches no session | F1 |
| `zero-value-as-valid-answer.md` | 2026-10-23 | KEEPALIVE metadata AS0 for a dynamic peer | F3 |
| `zero-value-as-valid-answer.md` | 2026-10-23 | message before negotiation carries AS 0 for a dynamic peer, never "not yet known" | F3 |
| `zero-value-as-valid-answer.md` | 2026-10-23 | web interface counters partial reads any `GetStats` error as no counters | F1 |
| `guard-addition-drops-what-it-refuses.md` | 2026-10-03 | `buildWithdrawalPayload` returns nothing for two families | F2 |
| `guard-addition-drops-what-it-refuses.md` | 2026-10-08 | `prefixListFromIRR` cut and counted success | F2 |
| `constant-reported-as-measured-state.md` | 2026-10-08 | LG `transformRoutes` `primary` from an absent key | F4 |
| `constant-reported-as-measured-state.md` | 2026-10-03 | BMP Loc-RIB ORIGIN literal IGP | F4 |
| `memo-suppresses-a-change-it-cannot-see.md` | 2026-10-23 | bgp-rib `sameBest` ignores an attribute-only change of the winner | F4 |
| `green-that-could-not-have-been-red.md` | 2026-08-31 | trafficusage `ebpfSupported` constant nil | F4 |
| `green-that-could-not-have-been-red.md` | 2026-08-31 | l2tp `checkHealth` cannot fail | F4 |
| `silent-fall-through.md` | 2026-10-20 | `getConfigPathPlugins` starts plugins after `ErrMissingDependency` | F1 |
| `silent-fall-through.md` | 2026-10-20 | MRT table dump and `show system` NTP block skipped when not built | F1 |
| `silent-fall-through.md` | 2026-10-20 | ddos responder, exabgp bridge, flowexport enrichment declare no BGP dependency | F1 |
| `silent-fall-through.md` | 2026-10-20 | redistribute `destination`, ls_export `source` not checked against the registry | F1 |
| `silent-fall-through.md` | 2026-10-23 | fib-vpp `newBackend` falls back to the `mockBackend` test double when VPP is unreachable | F1 |
| `zero-value-as-valid-answer.md` | 2026-10-23 | VPP `readVPPIfaceStats` any stats read error read as no counters | F4 |
| `zero-value-as-valid-answer.md` | 2026-10-23 | `answering.answered` read failure answered as not answered | F1 |
| `zero-value-as-valid-answer.md` | 2026-10-23 | rs `extractWireNLRIRecords` unknown encoding context answered as no wire UPDATE | F1 |
| `constant-reported-as-measured-state.md` | 2026-10-23 | `checkVPPHealth` healthy when the VPP socket is absent | F4 |
| `zero-value-as-valid-answer.md` | 2026-10-24 | rib `showPipeline`, `bestPipeline` parse error answered as an `{"error":...}` payload with status done | F1 |
| `guard-enumerates-instead-of-subtracting.md` | 2026-10-24 | `commandRegistrars` does not classify `RegisterTextRenderer` / `MustRegisterTextRenderer`, added by this spec's 238bdb62e9; `TestEveryCommandRegistrarIsMatched` red | - |

54 rows.

## Rows found already fixed

| Class file | Date | Surface (short) | Evidence (2026-10-23) |
|------------|------|-----------------|-----------------------|
| `silent-fall-through.md` | 2026-09-14 | `xfrmAEADName` default arm | `xfrmAEADName` (`ike/dataplane/xfrm_linux.go`) now returns `ErrNotSupported` for an unmapped name; the 128-bit ICV is documented as true of every mapped entry |
| `zero-value-as-valid-answer.md` | 2026-09-01 | mvpn `hasRD` | `hasRD` is gone; `MVPN.rdPresent` (`nlri/mvpn/types.go`) records presence at parse |
| `zero-value-as-valid-answer.md` | 2026-10-14 | `buildVPNNLRIBytes` path ID | `update_build_vpn.go` tests `ub.AddPath` alone; the row's own Fix cell says fixed |
| `constant-reported-as-measured-state.md` | 2026-08-22 | rpki `validation-enabled` and `running` literals | `rpki.go` writes `rp.active.Load()` and `rp.validationEnabled()` |
| `silent-fall-through.md` | 2026-09-21 | LDP `DecodeInit` skips unknown U=0 TLV | `DecodeInit` (`ldp/wire.go`) returns `unknownTLVError` and `(*Session).ignoreUnknownTLV` sends `statusUnknownTLV` (verified at HEAD 5762f6f2be); this spec added `TestLDPInitUnknownTLVUClearNotifies` (RFC5036-3.3-1 negative) and its record |
| `zero-value-as-valid-answer.md` | 2026-10-05 | IS-IS invalid three-way state becomes the zero action | `ReceiveHello` (`isis/adjacency/fsm.go`) rejects before mutation since af69577952, tagged `TestRFC5303InvalidStateLeavesAdjacencyUntouched` (RFC5303-3.2-7); this spec added the drop count in `handleIIH` (`Transport.CountDrop`) and `TestISISHelloInvalidThreeWayStateDiscarded` |
| `zero-value-as-valid-answer.md` | 2026-09-21 | rsvpte `handlePath` admits PATH without SENDER_TSPEC | d07c29546f: `handlePath` (`rsvpte/engine.go`) refuses `!msg.HasSenderTSpec`; `TestRFC2205PathWithoutSenderTSpecRefused`, `TestRFC2205PathWithoutSenderTSpecDropped` |
| `constant-reported-as-measured-state.md` | 2026-09-21 | rsvpte `rroProtectionFlags` from config | callers gate `rroProtectionFlags` on `engine.bypassEstablished` (`rsvpte/frr.go`, `engine.go`, `reservation.go`); `TestRFC4090FlagsClearUntilBypassEstablished`, `TestRFC4090RefreshTracksBypassState` (bypass set DOWN) |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete
- [ ] Scoped package and functional tests pass; GitHub CI owns whole-tree verification after a push
- [ ] Every A-N confirmed or broken
- [ ] Every item this spec did not do is a spec of its own, named here

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + journal rows re-pointed
- [ ] **Commit B:** `remove plan/release/fixes/spec-refuse-instead-of-defaulting.md` only

## Implementation Summary

### What Was Implemented

The registered `arch refuse-by-default` check judges changed source lines for
unmatched switch answers, lookup/read failures answered as values, identifier
zero used as absence, fabricated state, and dropped errors below WARN. Its
report includes both refused and annotated shapes. An annotation must name a
visible consumer guard; it cannot name its enclosing producer.

The 54-row ledger tracks the runtime repairs and the separately owned dependency
rows. Typed peer-AS identity and negotiated ADD-PATH presence travel through
native JSON/text, structured events, command output and compatibility boundaries.
Refusals preserve valid zero readings, real not-found results, explicitly empty
AS paths, and valid protocol/control inputs.

### Bugs Found/Fixed

Later verification repaired pipeline error verdicts, absent CoS and forwarding
providers, JSON RPKI/monitor identity readers, fixture teardown isolation, direct
state-member detection, and withdrawal error propagation through both forward
rails. Their exact negative and passing logs are retained above.

Round 1 found three additional product defects: BMP discarded authoritative
stored AS_PATH segments, BMP accepted an absent next hop without encoding a
route, and forked NTP state was mistaken for inactive central state. Source fixes
are committed in `4a991e513e`, `dfc2863cea` and `2ea5329344`, with main-thread
negative/clean evidence above. Official round 2 is CLEAN and its native check
passes all 591 code files. Load-80, unchanged promotion, canonical 2/2, wrapped
pmacct 1/1 and zero-owned RFC reconciliation are complete.

The 2026-11-12 scoped lint found nine owned defects, all from this spec's
commits: seven style findings and a `nilnil` answer in the NTP code of
`2ea5329344`, one misspelling from `820476c6ee`, and a compile-out test,
`TestBuildTag_NTP_AbsentSyncInfoNil`, that stopped compiling in the build
without `ze_ntp` when `2ea5329344` changed `GetNTPSyncInfo`. The unused
`jsonKeyError` constant that `820476c6ee` orphaned goes with them. All ten are
fixed in `14d0fa156d` ("Scoped lint result" above).

The lint rerun found one more defect of this spec: the allocation-measuring
call in `TestFwdWithdrawalLabelledCompatibility`
(`internal/component/bgp/reactor/forward_build_test.go`) discarded the length
and the error that `a7afabc1be` gave `buildWithdrawalPayload`, so a refused
framing could pass the zero-allocation check, because a failed call allocates
nothing either. `ad05166600` checks both after the measured closure. The rerun
shows no other finding on a line this spec wrote or caused ("Rerun after the
lint fix").

### Documentation Updates

The committed documentation covers typed identity and event formats
(`docs/architecture/api/{json-format,text-format,commands}.md`), explicit NLRI
presence (`docs/architecture/route-types.md`), protocol refusal and health guides,
and the changed-line check/annotation contract
(`docs/contributing/ze-go-style.md` and
`docs/architecture/testing/verify-freshness-scope.md`). Later changes document
RPKI refusal metrics, pipeline error verdicts, CoS and forwarding-provider refusal.
The source anchors name the producing functions. The BMP guide and NTP command
reference updates accompany their committed fixes and match the reviewed source.
The current `doc check all` result has only a foreign completion-catalogue
description drift; other steps validate 447 commands and all source anchors.
The earlier build-blocked attempts remain retained above. The lint-fix commit
repoints one source anchor in `docs/guide/command-reference.md` (NTP status) from
`ntpReplyMetadata` to `ntpReplyEnabled, ntpActiveMetadata`; no other page or Go
file names the removed function. `./le doc check all` ran after it
(`scratch/job-doc-check-all-506ada98.log`): the source-anchor step passes for
4562 code paths. Two steps fail, and neither is this spec's: the published wiki
command catalogue disagrees with the live one, and three `ai/digests/` files
(`mpls-signaling.md`, `subscriber.md`, `wire-and-pools.md`) cite
`ai/PACKAGE-MAP.md`, which `7629e90135` removed.

### Deviations from Plan

The actual absent-CoS binary integration test replaces the proposed functional
fixture because the normal fixture runner enables CoS. The owner explicitly
returned NTP to this spec with “fix NTP”. The BGP peer bare-list fixture has its
existing owner, `spec-web-cli-config-one-off-defects`; it is not silently counted
as delivered here. The check gained the fifth dropped-error shape and file-local
state assignment tracking after source evidence showed those gaps.

The full RFC package completed with three stale fixture failures. Independently
judged fixture-input corrections and the selected 67 affected roots provide the
retry evidence; this spec does not claim an all-green full-package run. An
archive-based compile check would violate the owner's main-checkout-only rule;
main's scoped package builds and executions are the applicable evidence.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | CPE renderer extraction missed the identity constant's VPN checker and test callers. | The moved declaration still had callers outside the extracted file. | QEMU launcher build failed on `cpeIKEName` before any guest assertion. | Export `homecpe.CPEIdentity`, update the callers and prove the shared renderer and guest path in main. Later evidence above supersedes the initial build failure. |
| assumption | A-1 expected four checker shapes to cover the population. | Below-WARN error dropping required a fifth shape, and direct state writes required lexical tracking. | Runtime and source audits exposed both gaps. | Add `droppedError` and `stateAssignments`, retain positive/negative fixtures, and require explicit row-level execution rather than treating report output as a semantic proof. |
| assumption | Attribute-level BMP tests accepted an ORIGIN-only stored fixture and reconstructed the path from a flat summary. | The publisher carries complete stored segment bytes; an empty path is a present mandatory attribute. | Closure round 1 traced `formatASPath`, `storedAttributes` and `buildLocRIBUpdateBody`. | Repair the source and normalize the partial fixtures only after independent judgment; retain segment and empty-path regressions. |
| assumption | An init-registered NTP provider was treated as proof that central nil state meant inactivity. | A supported fork owns its worker state in another process. | Closure round 1 followed external plugin startup and both central show readers. | Return to implementation for the existing generic command RPC path and accepted-config-based inactive decision. |
| approach | `2ea5329344` changed the signature of `pluginreg.GetNTPSyncInfo`, and its package runs covered only builds that link NTP. | The `!ze_ntp` file `cmd/ze/hub/build_tag_ntp_absent_test.go` calls the same seam, and it stopped compiling, so the compile-out proof was gone until the lint's compile-out build showed it. | 13-package scoped lint, compile-out build, 2026-11-12. | Fix the caller and run the three `TestBuildTag_NTP_Absent*` tests in the build without `ze_ntp`. A changed seam signature owes a build of every negative-tag file that calls it. |
| approach | The first lint ownership pass (`refusal-lint-ownership.tsv`) judged each finding by `git blame` of the flagged line, and called the errcheck finding in `forward_build_test.go` foreign because blame names `c972e77416`. | The finding exists only because this spec's `a7afabc1be` made `buildWithdrawalPayload` return `(int, error)`. The work a change owes is what it can reach, not the lines it wrote (`ai/rules/principles.md`). | The owner's Mac lint rerun at `ef1563f19f`, traced by cause. | Fixed in `ad05166600`. The four other non-fixture findings were traced by cause too, and are other specs' ("Rerun after the lint fix"). Ownership of a lint finding is judged by what made the line wrong, never by blame alone. |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Repair every owned producer and its callers. | Done | 54-row retained ledger, 125-entry producer map and changed-carrier census | Round-1 findings, runtime/interop proofs, native round-2 review/check, the nine owned lint findings (`14d0fa156d`, bounded round 3 CLEAN) and the tenth (`ad05166600`, bounded round 4 CLEAN) are complete. The lint rerun shows no finding on a line this spec wrote or caused, and `system-date-ntp-forked` passes 1/1 after the fix. No waiver or scope reduction. |
| Prevent the defect shapes in new changes. | Done in source; main evidence retained | `internal/le/arch/refusebydefault/{refusebydefault,state_assignments,dropped,annotation,report,register}.go` | Changed-line, annotation, state and registration controls are named above; no whole-tree verify run is required. |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `fileScan.defaultArm` / `fallOff`; `TestRefuseByDefaultRefusesEachRuleFixture` | Unmatched nil-error answers refused; matched cases preserved. |
| AC-2 | Done | `missOf`, `recordsMiss`, `answersPlausibly`; miss fixtures and netlink controls | Observable absence remains distinct from read failure. |
| AC-3 | Done | `RulePathIDZero`; `nlri.HasPathID` / `HasAddPath` | Negotiated presence replaces value-zero tests. |
| AC-4 | Done | `stateAssignments`, `stateValue`, `fileScan.expression` | Full race retry and 86.5% package coverage retained; uncovered blocks disclosed. |
| AC-5 | Done | `parseGuard`, `annotationFor`, `judgeFile` | Valid visible consumer guards are listed in Annotated. |
| AC-6 | Done | `RuleAnnotationNoGuard`, `namesEnclosing` | Missing, unknown and self-named guards refused. |
| AC-7 | Done at final named checkpoint | Main `job-refusal-final-named-report-45a00bc9.log` and `scratch/refusal-ac7-final-125-comparison.json` | 2207 refused + 72 annotated in 7109 files; exact 54-row/125-entry join finds zero owned matches in both populations. Report exit zero alone is not acceptance. |
| AC-8 | Done | Typed not-found, pipeline, absent-CoS/writer, monitor/RPKI and actual forked-NTP proofs | NTP owner transport and inactive/unavailable distinction are committed and reviewed CLEAN; load-80 and promoted canonical 2/2 pass in the recorded logs. |
| AC-9 | Done | RADIUS EAP discard controls; LDP/FRR 1/1; gnmic nine subtests; BFD/IS-IS/RSVP/label controls | Existing protocol-valid controls retained. |
| AC-10 | Done | Actual ADD-PATH baseline, omitted-zero expected failure, unmodified clean | Four octets for identifier zero reach FRR; non-ADD-PATH retains no source identifier. |
| AC-11 | Done | Typed dynamic-AS controls, monitor PTY/forked RPKI, thermal zero and LLGR/DDoS regressions | Compatibility auto-AS zero remains an explicit ExaBGP boundary contract. |
| AC-12 | Done | Actual-kernel TCX root/non-root, VPP/L2TP/netlink, BMP/pmacct and internal/external runtime proofs | Stored AS_PATH and missing-next-hop repairs pass 273 BMP race roots, zero skips. Load-80, promoted canonical 2/2 and properly wrapped pmacct 1/1 pass. |
| AC-13 | Done; final closing checks separate | 54-row regression ledger and exact 125-entry producer map | BMP/NTP negatives/clean, load-80 and all 16 committed native RFC tuples are complete. Final 73-finding source/history audit finds zero owned debt. Scoped lint ran: 8 owned findings plus one owned compile-out error, fixed in `14d0fa156d`; the rerun found one more owned finding, fixed in `ad05166600`, and no finding remains on a line this spec wrote or caused. |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Checker rule, guard, report and state fixtures | Present; main race evidence retained | `internal/le/arch/refusebydefault/` | Includes changed-line boundaries and valid measured/guard survivors. |
| RADIUS, label exhaustion, LDP unknown TLV, IS-IS and RSVP regressions | Present; ledger retains original producer-red attribution | Owning files in the TDD table above | LDP and gnmic native interop renewed in main; historical evidence is not presented as a rerun. |
| ADD-PATH zero/max/absent consumers and functional scenario | Present; actual interop negative/clean completed | Canonical consumer tests, `test/plugin/addpath-identifier-zero-forwarded.ci`, `test/interop/scenarios/addpath-zero-id-frr/` | Both writer and reader preserve negotiated framing. |
| Absent-CoS real binary and inline-map controls | Present; main race proof retained | `internal/test/cosabsent/cos_absent_integration_test.go` | Explicit binary-tag checks prevent a provider-present false proof. |
| Remaining census-row and numeric boundary controls | Present; 54-row ledger identifies canonical names | Owning packages in the retained row evidence audit | BMP/NTP negatives, package greens, runtime passes, load-80 and promoted canonical 2/2 are recorded above; the properly wrapped pmacct scenario passes 1/1. |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| Planned radius/LDP/iface/CoS and ADD-PATH producer files | Modified | Official round-2 population includes original `238bdb62e9`, which has no Spec-work trailer, and all 32 own commits through `cc190d045b`. |
| `internal/le/arch/refusebydefault/` and verify stage registration | Created/modified | Check, report, registration, five rule shapes, annotation judgment, fixtures and tests. |
| Absent-CoS integration, zero-ID functional test and FRR scenario | Created | Actual absent-build proof replaces the unsuitable all-feature fixture. |
| Style, stage-scope and source-linked subsystem docs | Modified; own anchors pass | Committed BMP/NTP docs match source; the current `./le doc check all` passes its source-anchor step, and its two failing steps (wiki command catalogue, `ai/PACKAGE-MAP.md` citations in three digests) are foreign. |

### Audit Summary

All 13 acceptance criteria are mapped. The three round-1 product findings are
fixed and official round 2 is SOURCE CLEAN. AC-7's final exact-owned checkpoint
covers 54 rows and 125 producer entries. Runtime load-80 and unchanged promotion
are complete, as are canonical 2/2, wrapped pmacct 1/1, all 16 committed native
RFC tuples and zero owned findings in the final individual RFC audit. The native
round-2 review check passes all 591 code files. The scoped lint ran on
2026-11-12; its nine owned defects are fixed in `14d0fa156d` (bounded round 3
CLEAN). The owner's rerun on his Mac exposed a tenth, fixed in `ad05166600`
(bounded round 4 CLEAN), and shows no finding left on a line this spec wrote
or caused. `system-date-ntp-forked` passes 1/1 after the fix, and
`./le doc check all` passes its source-anchor step, with two failing steps that
are not this spec's. `./le repo compiles check` is not run, under the owner's
rule against whole-tree checks; the scoped type-check, package and lint
evidence stands in its place ("Remaining verification"). No in-scope work is
skipped or approved for deferral. The spec stays open until the review record
and the closing commits finish.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Reject values the producer cannot know, while retaining useful answers. | Functional and interop | Actual ADD-PATH omitted-zero negative/clean, LDP/FRR, RADIUS retained proof, monitor/forked RPKI 2/2 plus 80 stress invocations, pipeline daemon fixture, actual-kernel TCX root/non-root and BMP/NTP results above. New runtime fixtures pass load-80 and promoted canonical 2/2; properly wrapped pmacct passes 1/1 with recording X6KQB2X5XBQFRHNP6ZP5YR44AH and no leftovers. The lint-fix commit `14d0fa156d` changes the `system-date-ntp-forked` fixture (constant, error wrap) and splits `ntpReplyMetadata` with the inactive answer unchanged; the rerun on a build of the current tree passes 1/1 (`scratch/job-refusal-ntp-forked-after-lint-042b8433.log`). |
| Prevent new instances of the defect shapes. | Native checker fixtures and source wiring | `TestRefuseByDefaultRefusesEachRuleFixture`, guard/state/changed-line controls and `TestVerifyStagesIncludeRefuseByDefault`; main race logs above. The checker is syntactic and cannot replace the runtime producer audit. |

## Work Not Done

No in-scope item is left undone. The closing steps in "Remaining verification"
(the review record, commits A and B) are this
closure's own steps, not work handed elsewhere.

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | Previous session: `tmp/review/refuse-instead-of-defaulting-01a1c842-0a58-7752-bbff-2a3f1633b66f.md`; clean, rounds=2, 591 code files. Independent source reports remain `scratch/refusal-closure-round2.md` and `.json`; round 3's is `tmp/session/2026-11-12-014013/scratch/refusal-closure-round3.md`. Main records the artifact keyed to the committing session immediately before the closing commits ("Population for the committing session"). |
| `./le spec review check` | PASS for session `01a1c842` over 591 code files. The committing session's check runs with its record. |
| Rounds | 4: two official passes and bounded rounds 3 and 4. Round 1 found three product groups; rounds 2, 3 and 4 found zero BLOCKER and zero ISSUE. Administrative corrections do not add a pass. |
| Reviewer lenses used | Correctness and wiring; security and edge cases; ownership and performance; test preservation; Go style; complete task/spec alignment. |

### Round 1 scope

The independent closure review covers original implementation `238bdb62e9`,
every commit carrying `Spec-work: refuse-instead-of-defaulting` through
`2558080335`, and current implementations and consumers. The original has no
Spec-work trailer and is explicitly included in
`scratch/refusal-complete-review.patch`. Main executes all tests and checks.
Unrelated dirty work is preserved and is not approved by this review.

The source pass traces changed panics to registration or validated internal
encoding choices; it finds no peer-controlled panic path. Buffer ownership and
source identity remain intact in `fwdWithdrawal.of`. Registry lookups release
locks before callbacks. Stored-attribute publication owns its snapshot rather
than exporting process-local pool handles. New refusal and metric reasons are
bounded classes. These conclusions do not clear the three product findings.

Main's mandatory `repo check` and `test weakened check` results are retained in
`job-refusal-review-repo-f6785901.log` and
`job-refusal-review-weakened-2e1e4fc8.log`. They reported nine unwired exports and
five test-change findings in foreign in-progress files. Main source attribution
found no owned item in those populations; this review does not approve the
foreign changes. Final lint, documentation and owned RFC reconciliation remain
owed. Archive-based compile execution is excluded by the owner's main-only rule.

### Round 2 source verdict

Official round 2 is SOURCE CLEAN: zero BLOCKER and zero ISSUE. It preserves the
original round-1 context and adds every subsequent own fix in
`scratch/refusal-round2-review-commits.txt`: 32 commits, 846 listed paths,
845 existing paths and 569 existing Go files. Original `238bdb62e9` is included
explicitly despite its missing Spec-work trailer. The complete patch is
`scratch/refusal-round2-complete-review.patch`, SHA256
`71c178166bb8830851b60ae2da59b9461e4bbe4991213fec02d4f02fb4f7368f`.
`scratch/refusal-closure-round2.json` retains the path-manifest and current
source hashes; foreign dirty work is outside approval.

Correctness, security, wiring, resource ownership/races, performance,
test-preservation and Go-style lenses found no remaining product finding.
The checker precision change permits only canonical typed refusals; BMP runtime
admission precedes side effects; NTP uses the running owner, preserves caller
authority/cancellation and stops workers/subscriptions on failure. This is a
source verdict, not a claim of completed closing commits. Main recorded the
native artifact above and its check passes all 591 code files, including the
two unchanged canonical promotions. The canonical 2/2 and properly wrapped
pmacct 1/1 runs pass; all 16 native RFC tuples are committed and individually
reconciled to zero owned debt. Broad lint and fresh precommit results remain
pending. Expanding abbreviated finding paths to full canonical source paths
was an administrative correction within round 2, not a third pass. Subsequent
native proof metadata updates changed no reviewed Go source.

### Round 3, bounded to the lint-fix diff

The diff is the nine files of the lint-fix commit ("Scoped lint result"):
`internal/plugins/ntp/{register.go,register_commands.go,command_active_config_test.go,startup_cleanup_test.go}`,
`internal/test/fixture/register_ntp_forked_state.go`,
`internal/component/bgp/plugins/rib/{rib.go,rib_pipeline_refusal_test.go}`,
`cmd/ze/hub/build_tag_ntp_absent_test.go` and one anchor line in
`docs/guide/command-reference.md`. A reviewer that did not write it returned
CLEAN to main, zero BLOCKER and zero ISSUE, with two comment-wording NOTEs that
were applied. The findings file is
`tmp/session/2026-11-12-014013/scratch/refusal-closure-round3.md`. The closure agent, also
not its author, read the same diff and agrees. `ntpSyncInfo` still answers nil
metadata with no refusal only after `ntpReplyEnabled` reads an explicit false,
and refuses a missing or non-boolean `enabled`; `forwardNTPCommand` checks
`enabled` itself before `ntpActiveMetadata`, so no path reaches the active
projection with an inactive answer. The fixture wraps a real cause with `%w`
and never formats a nil one. The test changes keep every assertion; the
misspelling fix changes the probe word and its expected text together. No
input, lock, goroutine or allocation path changes. No RFC tag or discrimination
record names any of the nine files.

### Round 4, bounded to the test fix

The scope is one hunk: the diff of
`internal/component/bgp/reactor/forward_build_test.go` in `ad05166600`, which
keeps the length and the error of the measured `buildWithdrawalPayload` call
outside the closure and checks them after it ("Rerun after the lint fix"). An
independent reviewer that did not write the change returned CLEAN: zero
BLOCKER and zero ISSUE.

### Population for the committing session

The closing commit's gate (`specWorkPopulation` and `judgeReviewCoverage`,
`internal/le/commit/review.go`) demands every code file of every commit whose
message carries the whole line `Spec-work: refuse-instead-of-defaulting`. Today
that is 34 commits and 129 code files, listed in
`tmp/session/2026-11-12-014013/scratch/refusal-gate-population.txt`. The
lint-fix commit adds `internal/component/bgp/plugins/rib/rib.go`, for 130.
`ad05166600` changes `forward_build_test.go`, which the population already
held, so at HEAD `ad05166600` the gate demands 130 code files over 37 commits
(`scratch/refusal-population-final.txt`).
Original `238bdb62e9` carries no trailer, so the gate does not search it; its
files stay covered by the round 1 and round 2 source reading but are not
demanded.

Against the previous session's artifact, HEAD's blob equals the pinned content
for 127 of the 129 files (per-file table:
`scratch/refusal-gate-pin-status.tsv`). Seven of those 127 differ in the
working tree only by the lint-fix hunks, which round 3 read. Two changed after
the review through other sessions' commits, and the closure agent read both
deltas:

| File | Foreign commit | What it changed | Reaches what this spec reviewed |
|------|----------------|-----------------|---------------------------------|
| `internal/component/plugin/server/startup_autoload.go` | `d7b8d92073` (`spec-doctor-coverage-12-plugin-infra`) | `appendDataPlaneWriter` now calls `registry.DataPlaneWriter`, and `ConfigDataPlane` wraps `iface.BackendNameFromTree` | Yes: the writer refusal `2b90f8864b` added moved into `registry.DataPlaneWriter` (`dataplane_writer.go`). Its gated-writer and no-writer branches and its three message parts are the reviewed text unchanged |
| `internal/plugins/cos/yang/ze-cos-conf.yang` | `857da9e798` (`spec-rfc-verdict-fix-access`) | Adds the `request class-of-service session` command container | No: `528fdc8fdc` moved the inline `qos-maps` grouping out of this module, and the foreign delta neither restores nor references it |

Main records the review for the committing session immediately before the
closing commits, over the code files of every commit carrying
`Spec-work: refuse-instead-of-defaulting`, `ad05166600` included. The argument
list is `tmp/session/2026-11-12-014013/scratch/refusal-review-record-final-args.txt`:
129 paths take the `file` form. One takes the `blob` form:
`internal/component/config/cli/validate_corpus_test.go`, pinned to its HEAD
blob `0f4f7457d6`, because the working tree carries another session's
uncommitted hunk in that file (ten deleted lines).

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE, fixed | Stored AS_PATH was discarded and rebuilt from a flat summary, omitting valid empty paths and changing AS_SET/mixed segments to AS_SEQUENCE. | `internal/component/bgp/plugins/bmp/bmp_locrib.go` `buildLocRIBUpdateBody`; `rib_attr_format.go` `formatASPath`; `rib_bestchange.go` `storedAttributes` | `4a991e513e`; negative/clean evidence above, source reviewed CLEAN in round 2. `dfc2863cea` additionally refuses an unavailable runtime feed before configuration effects. |
| 2 | ISSUE, fixed | A valid prefix and invalid next hop returned a successful attributes-only UPDATE; real inject/admission could reach it. | `bmp_locrib.go` `buildLocRIBUpdateBody`; `rib_commands.go` `injectRoute` / `candidateAdmitted`; `rib_bestchange.go` `internNextHop` | `4a991e513e`; explicit refusal and useful-next-hop controls pass, source reviewed CLEAN in round 2. |
| 3 | ISSUE, fixed | A supported forked NTP worker owned state outside the daemon, while central date/NTP shows read local nil as inactivity. | `internal/plugins/ntp/register.go` `ntpSyncInfo` / `runNTPPlugin`; `register.go` `handleShowSystemNTP` / `handleShowSystemNTPPeers`; registry `GetNTPSyncInfo` | `e62bc0e2a1` wires the CLI; `2ea5329344` implements owner queries, accepted-config inactivity, typed refusal and cleanup. Actual forked pass and source-CLEAN round 2 are recorded above. |

The initial BMP fixture migration draft was refused because `ASPath.WriteTo`
writes only a value and omitted the attribute header. The corrected draft uses
`WriteAttrTo`, including the present empty `40 02 00`. Independent source
judgment approves all 30 affected canonical consumers/helpers without removing
assertions, tags, selectors or waits. The exact report is
`scratch/bmp-stored-fixtures-judge.json`; main recorded its commands before the
fixture migration in `4a991e513e`. No approval command was run by the child.

## Pre-Commit Verification

This is a live closure record. The runs owed after the lint fix are recorded
below. `./le repo compiles check` is not run, under the owner's rule against
whole-tree checks, and it is not represented as a pass.

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| Planned checker files, absent-CoS integration, ADD-PATH/NTP fixtures, three canonical tests and FRR scenario | Yes | Main bounded `ls` on 2026-11-11 found every named file; FRR scenario contains `frr.conf` and `ze.conf`. |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1–AC-6 | Rule, annotation, direct-assignment and stage controls are implemented. | Main full checker race retry `job-unit-pkg-ef92ffff.log`; selected state/naming `job-refusal-state-and-rfc-green-f471a48b.log`. |
| AC-7 | Exact owned scope has no refused or annotated matches at the final named checkpoint. | Main `job-refusal-final-named-report-45a00bc9.log`; 54 rows/125 producer entries with actual AST anchors joined against all 2279 matches, zero owned results in `scratch/refusal-ac7-final-125-comparison.json`. |
| AC-8–AC-13 | Product fixes, behavioral proofs and native review/RFC checks are complete. | BMP/NTP negatives/clean, load-80 and canonical 2/2 pass; wrapped pmacct 1/1 passes with all exits zero. All 16 native tuples are committed; the individual 73-finding RFC audit has zero owned debt. The lint fixes pass their package tests ("Scoped lint result"). |
| Scoped lint | Zero owned findings after the lint-fix commit, each foreign finding attributed. | First run `scratch/job-refusal-final-scoped-lint-86c7c325.log` (session `2026-11-12-014013`): 134 findings, 8 owned, plus one owned compile-out error; ownership in `refusal-lint-ownership.tsv`. All nine fixed in `14d0fa156d`. Rerun by the owner on his Mac on a still tree (`scratch/refusal-lint-mac-ef1563f19f.md`): at `ef1563f19f` over the 13 packages, host 24, Linux integration 25, capability 1, appliance 0, compile-out 0, with one more owned finding (`forward_build_test.go` errcheck, owned by cause), fixed in `ad05166600`; at `ad05166600` over `./internal/component/bgp/reactor/...`, 0 in host, Linux integration and capability. No finding remains on a line this spec wrote or caused; every remaining one is attributed to another spec's commit. |
| Other process gates | All fresh precheck findings are foreign. | Repository check 31 foreign and weakened check 21 foreign (2026-11-11 logs above), citation check and bounded `ls` pass. After the lint fix: `system-date-ntp-forked` PASS 1/1 (`scratch/job-refusal-ntp-forked-after-lint-042b8433.log`); `./le doc check all` source-anchor step passes, two failing steps are foreign (`scratch/job-doc-check-all-506ada98.log`, "Documentation Updates"). `./le repo compiles check`: not run, because a repo-wide compile is a whole-tree check no agent starts (`ai/META-DECISIONS.md`, "How agents work"), and GitHub CI verifies the pushed commit. In its place: the changed packages type-check with their tests (`job-vet-pkg-7dd5a45b.log`) and their package tests pass, and the Mac lint type-checked every build of the 13 packages at `ef1563f19f`, compile-out included, and the reactor package at `ad05166600`. |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Zero-ID route across native sender/receiver | `test/plugin/addpath-identifier-zero-forwarded.ci` | Existing fixture explicitly checks zero identifier octets; actual FRR baseline/negative/clean is recorded above. |
| JSON peer identity and actual monitor output | `test/plugin/rpki-event-valid-forked.ci`, `test/plugin/monitor-remote-as-stream.ci` | Main promoted run 2/2; 80 native stress invocations; actual monitor producer-negative observed as recorded above. |
| Synthesized withdrawal through real appliance kernel | `test/plugin/linklocal-only-multihop-withdraw.ci` | Main QEMU 1/1, `job-refusal-withdrawal-qemu-afde2315.log`, recorded kernel hash above. |
| Runtime NTP owner and unavailable external BMP feed | `test/plugin/system-date-ntp-forked.ci`, `test/plugin/bmp-locrib-external-refuses.ci` | Main canonical selection passes 2/2 in `job-refusal-promoted-runtime-fixtures-027d40a9.log`; all 80 preceding loaded invocations complete without reproduction. Canonical content hashes above match the reviewed drafts. The lint-fix commit `14d0fa156d` changes the NTP fixture's Go side; the rerun on a build of the current tree passes 1/1 (`job-refusal-ntp-forked-after-lint-042b8433.log`). |
| Actual BMP collector | Native scenario `bgp-addpath-best-path-pmacct` uses the registered interop checker rather than a `.ci`. | Ze QEMU Docker-host/KVM run passes 1/1 in `job-refusal-bmp-collector-ze-docker-host-59be18bd.log`, recording X6KQB2X5XBQFRHNP6ZP5YR44AH; command/check/lab exits zero, no leftovers. |
| Rule registration, RADIUS socket, absent-CoS binary and LDP exhaustion | Wiring Test table uses Go/native socket or binary tests rather than `.ci` for these seams. | Exact named tests and retained main evidence above; no invented fixture. |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | Broken as stated; the original four shapes needed extension. | `droppedError` adds the fifth dropped-error rule and `stateAssignments` covers direct field writes. The report remains syntactic: round-1 semantic defects required the explicit producer-level controls now recorded above. |
| A-2 | Confirmed for the recorded sample, not a universal precision claim. | Historical 200-commit report: 10 findings, six true defects and four guarded zeros after the documented precision cuts. |
| A-3 | Confirmed with the recorded threading extension. | Negotiated flags reach `NewINET` callers, route/adj-RIB indexes and native/compatibility formatters; actual FRR zero-ID proof validates the wire path. |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Checker contract and stage wiring | `ScanFile`, `stateAssignments`, `droppedError`, `operatorErrorResponse`, `parseGuard`, `judgeFile`, registered actions and stage entry | Source matches style/stage pages; own source anchors pass. The current `./le doc check all` (`scratch/job-doc-check-all-506ada98.log`) has two foreign failing steps: the wiki command catalogue, and three digests citing the removed `ai/PACKAGE-MAP.md`. |
| Native identity, graph/pipeline and ADD-PATH formats | `appendPeerJSON`, `appendRemoteASText`, `formatMonitorLine`, `commandAnswer`, `applyRegisteredText`, `WriteNLRI` and `HasPathID` | Source-linked API/route/command pages cover changed carriers; current handler/shape checks validate 447 commands and source anchors pass. Citers are repointed; main's whole-tracked-path grep finds only this spec itself. Fresh native citation metadata check remains queued. |
| RPKI metrics, interface read/zero-counter semantics, CoS and writer dependencies | `SetMetricsRegistry`, `GetStats`, `Resolve`, `appendDataPlaneWriter` | Governing pages name the actual producers and preserve valid-control behavior. |
| BMP and NTP state authority | `storedAttributes` / `buildLocRIBUpdateBody`, `validateLocRIBRuntimeConfig`, forked `runNTPPlugin` and owner-query show readers | Committed guides, command reference and source contracts match the reviewed source. Load-80, unchanged promotion, canonical 2/2 and actual wrapped pmacct 1/1 pass. The NTP status anchor in `docs/guide/command-reference.md` names `ntpReplyEnabled, ntpActiveMetadata` after the lint-fix split, and the inactive and refusal answers the page describes are unchanged. `./le doc check all` after the fix passes its source-anchor step (`scratch/job-doc-check-all-506ada98.log`). |
| Runtime inventory/RFC generated material | Native registry/RFC generators own derived documents. | Main regenerated 231 RFC shards; its final native check has 73 global findings, individually audited as foreign with zero owned debt in `scratch/refusal-final-rfc-individual-ownership.json` and `.md`. No generated status page was hand-edited. |
| Operator news | `website/changes/fragments/refuse-instead-of-defaulting.md` follows the six-heading fragment contract. | Ready and untracked for commit A: front matter `spec`, `topic: Diagnostics` (a family in `website/data/topics.json`), `found-by: review`; the six headings in order. The lint fixes change nothing it states. |
| Citers of this spec | `git grep` for the spec path over the tracked tree, 2026-11-12 | Five citers are restated as the bare stem in the working tree, uncommitted, for commit A: `plan/handover/22-refuse-by-default-then-yang-source.md`, `plan/handover/23-plugin-optional-dependency-refusal-first.md`, `plan/handover/24-refuse-by-default-implemented-awaiting-closure.md`, `plan/release/fixes/spec-daemon-runtime-one-off-defects.md`, `plan/release/fixes/spec-dropped-error-0-umbrella.md`; each diff is that one line. No tracked file names the full path besides the spec. Every Go and `.ci` citer names the bare stem only. OWED after commit B: `./le spec citation`. |
