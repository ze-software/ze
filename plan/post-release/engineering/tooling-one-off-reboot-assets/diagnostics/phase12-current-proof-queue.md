# Phase 12 current-source proof queue

Source-only audit, 2026-11-06, for MAIN session
`01a1a907-3278-7971-98dc-dbc41d4a54a5`. This is not phase closure. No Go,
gopls, formatter, lint, test, build, Docker, browser, interpreter or runtime
command was run. Only source/document reads and this scratch prose were made.
No foreign source was changed. MAIN owns all execution, serialized.

The controlling requirements are the corrected P-4/P-14/P-20/P-21/P-37 rows,
their concrete carrier rows, and Phase 12 at
`plan/post-release/engineering/spec-tooling-one-off-defects.md:1303`.
The older direct-observation table is historical: its statement that runSlug
has no bound is no longer the current implementation.

Read: AGENTS, META-DECISIONS, CORE/TRIGGERS, system and le-tooling doors;
documentation/testing/commands/context-economy rules; running-commands launcher,
lint/admission and package-test recipes; functional-tests executable identity;
benchmarking comparison/publication; patching-the-standard-library vendor page.
The explicit MAIN-only restriction overrides the ordinary gopls-first route.

## Evidence interpretation

`tooling-one-off-evidence.md` is canonical prose recording observations. It is
not the underlying log or an observation made by this audit. Its contemporary
rows record watchdog draft/load/promoted success (744ed7ab, 11659b2a,
8aac34c4), a one-iteration stress native smoke (b1bcb869), vendor publication
(47340d61c5), empty-stdlib publication (c98d76f37f), capture publication
(86cb0aa071), and the bounded current schema-family pass (7518e4ba).

The recovery companion explicitly says former Nov5 originals and the Nov2
state record are unavailable; recreating a directory does not recover logs,
overlays or captures. No raw Phase12 proof was recovered or opened here.
Its newly retained Nov6 timestamp/inventory/GoBGP results prove their stated
surfaces, not Phase12. Preserve previous valid observations; do not repeat an
unchanged check solely for a newer date. A required missing raw proof, changed
producer or genuinely unrun obligation needs its own explicitly bounded run.

## Current status and remaining work

| Obligation | Current producer/carrier | Disposition and precise remaining gap |
|---|---|---|
| P-4 launcher argv/build identity | `le:767,783,830,889` uses quoted `"$@"`; cmd/ze root-launcher tests execute spaced/glob/empty/metacharacter argv against owned recording executables. `le_build_name.go` refuses requested/actual mismatch before dispatch. | Implemented in source. Existing real shell-entry units provide the carrier; do not invent the proposed launcher-argv-identity.ci as if present. Current scoped result/source binding and any missing causal proof remain to reconcile. Generated commit rechecks are the separate commit owner's scope. |
| P-4 watchdog | `cmd_exabgp.go` ordinary and foreground paths both use os.Executable and exaBGPServerArgs; a renamed basename gets explicit `le`. `ui_fixture_conf_watchdog_identity.go` hardlinks selected LE under a different name in a spaced directory, plants wrong PATH helper, checks mismatch refusal, drives real conf-watchdog, checks exact saved announce/withdraw frames and wrong-helper sentinel. Live `test/ui/conf-watchdog-executable-identity.ci` requires the marker. | Implemented; canonical prose records draft, 80 sequential load and promoted native success, landed 2496442cb3. This is ordinary wire-entry proof, not a separate foreground-debug run or VM/image proof. Do not falsely mark the already recorded 80-run result unrun. Exact retained bytes/source binding after evidence loss remain distinct. |
| P-4 stress suite/fresh execution | run.go harnessArgs derives the registered suite route; admitRun sets MayAttach=false; runClaimed exports/restores job.ParentKey. process.go invokes the current executable with shared argv() and inherits session identity. TestEverySuiteLaunchesItsRegisteredHarness and TestStressIterationsEachRunInsideOneSlot cover registry/admission. | Implemented. Current native prose records one actual version-extended child, not every suite or 91 native selectors. Admission test uses an owned Git fixture; MAIN must respect existing fixture ownership, never invoke Git mutations on the shared tree. |
| P-14 original L2TP unused pair | Journal bulk-rename-corruption 2026-08-10 names exactly writeAVPUint8/readAVPUint8 in avp.go. Neither symbol remains; avp.go:255 explains retirement. unwired-feature 2026-08-12 explicitly records their removal and earlier changed-scope/package proof. | Original pair source disposition recovered, not an unidentified symbol population. Current `unused` lint pass still needs appropriate recorded evidence if absent. The old row's wider 'every package the refactor touched' population is not enumerated here and cannot be certified by one L2TP pass. Do not conflate October typeDefFirst/unparam/goconst findings with this pair. No dead helper should be reintroduced just to manufacture RED. |
| P-14 vendor preservation | Setup.vendorGoDeps uses an owned sibling stage, runs vendor -o there, applies every stamp/patch against that stage, then publishVendor retains/replaces live vendor with rollback-on-failed-rename. Real local-module stale and missing-stamp fixtures observe old patched bytes remaining. Command-recording tests cover positive ordered reapplication/failure. | Implemented. Commit 47340d61c5 records landed chunk. No live repository setup-install run is recorded as proof and none is needed as a destructive test. Proposed vendor-preserves-owned-patches.ci is absent; existing native Setup-method fixtures are the concrete carrier. Positive real patched replacement/publication evidence and publication-error branch execution must be accounted for separately from simulated-command order. |
| P-14 schema registration | config/cli/all_import_test.go links plugin/all plus bgp/config seams. Domain registration remains init/tag driven. | Partial evidence/census. Historical row says 17 cases across deactivate/diff/history/migrate, with no 17 stable names. Canonical current family proof names deactivate/activation/diff/migration, explicitly excludes reconstruction of original17. History remains in the current affected family. Historical FlowSpecConfigToPlugin/VPN and flowspec_legacy_test.go are absent now; map their retirement to reviewed replacement proof rather than schedule nonexistent roots. Proposed schema-test-registration.ci is absent. Do not widen this into a schema architecture redesign. |
| P-20 empty feature tags | featuretags.DaemonTags returns ErrNoGateTags; DaemonBuildTags propagates it; root-launcher preparation refuses absent/empty manifests before compiler. | Implemented with positive/negative carriers. Fresh current evidence, where actually owed, should select those roots rather than broad builds. |
| P-20 empty stdlib | stdpatch/gen/tree.go listStdlib rejects successful empty decode with GOROOT/cause/fix. empty_listing_test.go uses real empty-success executable for listing and sameStdlib; old positive tree-serving test remains. | Implemented; canonical prose separately records real installed-GOROOT producer smoke and c98d76f37f publication. Neither a source read nor the old positive smoke is fresh negative runtime proof. |
| P-20 identity | report/markdown.go comparisonKey includes DUTName, Family, ForceMP, Pass; latestComparisonResults only deduplicates inside that key. Three existing report roots assert latest selection, both recorded passes, and separately named unrecorded pass across Markdown/HTML/doc. | Implemented in source; preserve this repair. Rejecting every repeated DUTName would violate the corrected criterion. Trend is chronological history, not the comparison dedup identity. |
| P-20 loss/convergence/reporters | Result.ComparisonError rejects any nonzero RoutesLost, then missing convergence; custom JSON decoding preserves missing/null versus explicit0. Markdown/HTML/PerformanceDoc/Trend validate raw input before grouping/output. cmdReport validates every decoded file before renderer; cmdTrack validates before --last and --check. | Implemented in source with exact direct and CLI tests listed below. No result of those tests is inferred here. Report tests exercise HTML table and SVG through HTML; no separate public SVG entry was found. Raw JSON/NDJSON writers preserve diagnostic loss and presence. Compare offers md/html/doc, not comparison JSON. |
| P-20 publication | internal/test/perfrunner.generateToFile writes sibling .new, checks generation/close and renames only on success. Native render reaches md stdout, html file, doc snapshot independently. report_validity_test feeds real decoding/rendering under publisher and proves old bytes survive invalid loss; rename-failure test preserves old directory contents. Site build.go copyPath similarly acquires rooted source, stages, closes and renames. | Implemented; scoped publication proofs still need evidence accounting. Per-file replacement only; stdout cannot roll back and shell redirection can truncate. Targeted production source search for convergence-ms/routes-lost in internal/le/site found no performance parser: do not invent one or claim a site convergence test. Generic rooted-copy carrier is separate from perfrunner report validity. |
| P-21 91 selectors | runSlugMax=96; full SHA256 of suite+NUL+original selector string, timestamp and os.CreateTemp suffix make bounded exclusive captures. argv() feeds both actual invocation and JSON evidence. TestCaptureIdentityRetainsLongSelectorsAndFreshStarts creates real child processes/files for 91 selectors, same-prefix change, slash/hyphen collision, fresh same selection and spaced selector; two actual starts per case. | Implemented and substantially concrete carrier. The spec's TestRunSlugBoundAndIdentity name is absent/superseded by this current root. This uses an assertion-bearing owned shell harness, not 91 live registered functional cases. Canonical one-child native smoke cannot be stretched to that stronger native population. Decide evidence from actual requested contract, preserving exact argv/start-count requirement. |
| P-37 selected lint input/backup | go/lint goList uses -e -deps -test and selected JSON fields; separates direct files from compiler closure, retains InvalidGoFiles/EmbedFiles, matches generated test main to actual owner. job.CheckLintInputs uses same lintIgnores as InputHash, checks lexical/canonical/outside/unreadable source. runHere plans before admission and rechecks hash after admission. | Implemented with real Go-list/compiler fixtures for excluded authored input, untouched nested-module invalid backup, ordinary/internal/external-test dependencies and canonical symlink/dangling target. No deletion/glob of foreign backup. Existing permitted noninput backup is positive survivor, not coverage exemption for selected source. |
| P-37 empty lint outputs | goList refuses successful empty stdout and no checkout files; plan refuses empty selected populations; coverage refuses empty tracked corpus. Intentionally explicit empty scope returns skipped matrix with no commands. | Implemented and existing TestRunnerRefusesMissingToolAndEmptyPopulationOutput. Preserve distinction between empty derivation (refuse) and deliberate no-package scope/changed-set (truthful no work). Scope-empty positive test must remain. |

## Exact MAIN-only scoped command queue

These commands are OWED candidates, not executed results or instructions to
rerun already sound retained proof. MAIN first checks which named proofs remain
missing and whether source changed. Use the working-tree named launcher and
its admission route; never a shared HEAD-only answer to dirty source.
Use the current tag string already recorded by MAIN (`ze_le integration` plus
all feature-gates ze_ tags), `CGO_ENABLED=1` for these race tests, and a unique
label per command. Template:

```sh
CLAUDE_CODE_SESSION_ID=01a1a907-3278-7971-98dc-dbc41d4a54a5 CGO_ENABLED=1 \
  bin/le-tooling-resume-01a1a907/le job run label <unique-phase12-label> quiet command \
  go test -race -tags "$tooling_test_tags" -p=1 -count=1 -timeout=5m -v \
  -run '<exact selector below>' <exact package below>
```

`-count=1` is required for causal cuts to shell/exec-reached producers, whose
bytes are outside Go's test-result cache; read the RUN/PASS/FAIL names, skips,
child start counts and source manifests. Do not call ./le verify or whole-tree
compile/lint. All synthetic fixture compilation still runs only inside MAIN's
admitted test. New branch execution owes coverage, not a list of test names.

| Package | Exact bounded roots to account for |
|---|---|
| ./cmd/ze | `^(TestRootLaunchersBuildMissingBinary|TestLeLauncherNameBuildsPrivateBinary|TestLeLauncherCarriesTheBuildNameToNestedCalls|TestLeLauncherPreparationFailureNeverPublishes|TestInvokedBuildNameReadsTheDirectoryForANamedBuild|TestRefuseWrongBuildNameGuardsANamedSession)$` |
| ./internal/test/cli | `^TestExaBGPClientEnvReachesTheRunningLe$`; foreground helper has source wiring but no separate runtime proof established here |
| ./internal/le/test/stressrepro | `^(TestEverySuiteLaunchesItsRegisteredHarness|TestStressIterationsEachRunInsideOneSlot|TestCaptureIdentityRetainsLongSelectorsAndFreshStarts|TestTheBuiltDaemonHasTheFakeFilterPluginBesideIt|TestEveryInvocationFailingIsNoVerdict)$` |
| ./internal/le/setup | `^(TestVendoringRefusesBeforeLosingOwnedPatch|TestVendoringStaleStampPreservesOwnedPatch|TestVendoringReappliesOwnedVendorPatches|TestVendoringFailsWhenAnOwnedPatchCannotApply|TestVendorPatchRefusesAStaleReviewStamp|TestAFailedVendorFailsTheRun|TestVendorFixtureRecordingRejectsUnsafeTargets)$` |
| ./internal/le/vendorpatch | `^(TestVendorStampIsComparedWithModulesTxt|TestNetlinkXFRMPatchApplied|TestBubblesCursorPatchApplied|TestNclient4ShortIPv4PayloadGuard|TestPatchedVendorModulesMatchUpstream)$`; upstream root may need cached modules/network and is not an automatic setup/install authorization |
| ./internal/component/config/cli | `^(TestCmdDeactivate.*|TestCmdActivateRoundTrip|TestConfigDiff.*|TestCmdHistory.*|TestMigrate.*|TestConfigMigrate.*)$`; current semantic family, explicitly not reconstructed original17 |
| ./internal/le/repo/featuretags | `^(TestAManifestWithNoGateIsAnError|TestAnAbsentManifestIsAnError|TestDaemonBuildTagsAreTheManifestPlusTheBase|TestDaemonBuildTagsTakesTheBaseFromItsCaller)$` |
| ./internal/le/go/stdpatch/gen | `^(TestEmptySuccessfulStdlibListingRefused|TestStdpatchTreeServesTheWholeStdlib)$`; latter may be expensive, prefer already sound positive producer evidence when unchanged |
| ./internal/perf/report | `^(TestComparisonRefusesLossBeforeSelection|TestComparisonConvergencePresence|TestReportWriteFailurePropagates|TestRawLossDiagnosticRoundTrip|TestComparisonOutputsKeepLatestResultPerDUT|TestComparisonRendersBothPassesOfOneDUT|TestComparisonUnrecordedPassIsNamed)$` |
| ./internal/perf/cli | `^(TestCompareCommandRefusesLoss|TestTrackCommandValidatesBeforeLast|TestCompareCommandConvergencePresence|TestReportCommandsPropagateStdoutFailure)$` |
| ./internal/test/perfrunner | `^(TestReportPublicationRetainsValidOldBytes|TestReportReplacementFailureRetainsDestination|TestGenerateToFileNeverDestroysTheExistingReport|TestGenerateToFilePublishesOnlyACompleteResult)$` |
| ./internal/le/site | `^(TestArtifactCopyFailurePreservesPublishedBytes|TestArtifactReplacementFailurePreservesDestination|TestArtifactCopyDoesNotPublishLinks|TestArtifactCopyPublishesRootedLinkContent)$`; generic reached publication only |
| ./internal/le/go/lint | `^(TestLintInputPopulation|TestLintDependencyInputPopulation|TestLintCanonicalInputPopulation|TestLintAllowedAuthoredTestScope|TestRunnerRefusesMissingToolAndEmptyPopulationOutput|TestEmptyScopeRunsNoCommandsAndPrintsNothing|TestPlanRefusesUnmeasuredInputs|TestPlanRefusesInputChangesBetweenFlavors|TestPlanRefusesCoverageInputDrift|TestPlanStableInputsRetainExactMatrix)$` |
| ./internal/le/job | `^(TestInputHashRefusesMeasurementErrors|TestInputHashIncludesDeletedInputs|TestAdmissionRefusesUnmeasuredInputs|TestEveryInputTheLintLabelReadsVoidsItsShare|TestTheTreesTheLintLabelIgnoresDoNotVoidItsShare)$` |

Native functional entry commands, if their required raw/native proofs truly
remain missing, use rebuilt current harness artifacts and stable selectors:

```sh
./le --name tooling-resume-01a1a907 test ui conf-watchdog-executable-identity
./le --name tooling-resume-01a1a907 test bgp parse cli-config-deactivate-leaf cli-config-deactivate-container cli-config-activate cli-config-diff cli-config-diff-identical cli-config-history cli-config-migrate-default cli-config-migrate-format
./le --name tooling-resume-01a1a907 go lint run scope "./internal/component/l2tp/..."
```

Native stress grammar is `test stress-repro run suite <registered-suite>
test <shell-quoted-selector-string> iterations N parallel N burners N minutes
N timeout N any-failure`. Source derives suite routes; never substitute
positional IDs. Existing proof covers two real starts for each long-list
fixture plus the separately recorded native one-child smoke. There is no
existing native perf-comparison .ci in the searched UI population; an additive
native fixture for decoded loss/default/md/html/doc and track --last, plus
unrecorded/recorded positive controls, remains the explicit entry-point gap if
unit-driven real cmdReport/cmdTrack is insufficient for the closure criterion.
Draft any new .ci under test/draft/ui with ze-test, not in the live suite.

## Causal source cuts for genuinely missing discrimination

These are future plans only; no overlay or live mutation was made. MAIN freezes
exact pristine before bytes and manifests, uses reviewed source overlays where
possible, verifies nonempty applied diff, observes the named semantic failure,
restores exact bytes and observes the unchanged carrier pass. A compile/start
failure, forced checker verdict, altered expectation or simply returning an
error from a predicate is not causal RED.

* Launcher: replace the actual final argument forwarding with the old unquoted
  expansion in an owned launcher fixture; shell-entry argv tests must observe
  the broken spaced/empty/glob boundaries. Do not mutate the shared launcher.
* Watchdog: change helper selection from os.Executable to actual PATH lookup,
  or omit the needed explicit le root for renamed executable; native fixture
  must fail its wrong-helper/wire assertion. Ordinary and foreground routes
  are separate source sites; an ordinary run alone proves neither both modes.
* Stress freshness: break the actual parent-slot propagation or admission
  attachment setting and observe children attaching/not starting at the real
  admission carrier; do not fake child verdicts. Break argv() quoting to lose
  a spaced selector and observe shell child's exact-argument assertion.
* P-21: restore the unbounded slug to cause real filename creation failure;
  separately remove full-input digest and exclusive fresh creation to expose
  same-prefix/sanitization/fresh-run identity loss. Truncation alone cannot
  stand in for those independent collisions.
* Vendor: point real `go mod vendor` output at the fixture's live vendor before
  stamp validation; the real stale-version fixture must observe destruction
  of its owned prior patched bytes. Positive patch application cut must alter
  stage/apply ordering or drop the actual application, and inspect published
  bytes; do not force CheckStamp's return. Never perform a live repo vendor
  experiment. L2TP has historical retirement rather than a live producer cut.
* Schema: remove the specific needed registration edge only in an isolated
  compiler overlay and observe the unchanged current family fail on its
  missing module; a syntax/import-cycle failure is not a schema semantic RED.
  Reconstruct/map old17 and vanished FlowSpec carriers before claiming their
  discrimination, not by pretending current names are those historical cases.
* Empty derivation: remove actual no-gate/no-stdlib-results refusal and rerun
  the existing real-input fixtures; positive nonempty controls remain green.
* Reports: remove raw validation at each reached renderer/command or move it
  after latest selection/--last; older lossy rows must become wrongly emitted
  or ranked in unchanged tests. Separately lose decoded presence so missing
  becomes zero; drop Pass from actual comparisonKey to lose a distinct pass.
  Raw diagnostic round-trip must continue to retain loss. Break publication
  by opening/truncating destination before successful generation and observe
  old destination bytes change, never claim stdout rollback.
* Lint: omit -deps/-test or ordinary/test dependency source accumulation to
  expose the real imported excluded source; drop canonical input resolution
  to expose the real symlink target; remove/relocate the actual input check
  past admission to expose incorrect reuse. Real compiler positive fixtures
  must still compile the selected consumer and preserve invalid nested-module
  backup bytes. For empty output, remove actual goList emptiness handling and
  observe erroneous plan success; explicit empty-scope control stays valid.

## Handoff

No immediate source repair is established by this read. Most Phase12 producers
and concrete carriers are already implemented. Remaining work is exact proof
accounting/execution, native-entry coverage where required, historical schema
17/FlowSpec retirement mapping, and wider renamed-population membership.
Source/canonical evidence is sufficient to plan these without claiming a new
PASS or inventing old raw evidence. Preserve the huge dirty shared tree and
all other phase ownership; no inventory, GoBGP or browser work is included.
