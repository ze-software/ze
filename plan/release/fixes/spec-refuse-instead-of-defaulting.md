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
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Remaining verification

The full native RIB package ran and exposed a shared fixture leak: an enabled
Adj-RIB-In owner correctly leaves a refusing lookup on exit, but
`startValidationReceiver` left it registered for later tests. The redist replay
test passed alone and failed after ASPA retention; the new untagged
`TestValidationReceiverRestoresUngatedSelection` then failed both teardown
assertions. Commit `1aa91d030b` makes the fixture unregister only after joining
its owner. Main's
native `test unit pkg path ./internal/component/bgp/plugins/rib/ run` selection
covering the new regression, all six existing
fixture callers and `TestRedistReplayEmitsUnderEmitLock` passes under the native
feature tags and race. Logs are `job-unit-pkg-77fea4ce.log` (isolation),
`job-unit-pkg-f0677e7f.log` (pair red), `job-unit-pkg-cf1bde44.log` (new red) and
`job-unit-pkg-59779559.log` (focused green) in the session scratch directory.
Independent test judgment and source review are clean. Existing caller inputs
and assertions, and the production fail-closed teardown, remain unchanged.
Main's full native `test unit pkg path ./internal/component/bgp/plugins/rib/`
retry also passes in `job-unit-pkg-f5c56d84.log`. That deterministic log path
was reused: its current contents are the passing retry, not retained evidence
of the initial 152 failures. The separate pair and new-regression red logs
above retain the fixture leak's observed failures.

Earlier checkpoint history: the dependency addition was coherent enough for the named
netlink scoped package proof to pass and its negative overlay to run. The later
verbose clean control could not start: foreign in-progress hook-runtime edits
left callers in `lossypipe.go` and `rawheavy.go` passing three arguments to
`admittedCommand`, whose current signature takes two. Preserve that owner's
work; this is a shared launcher build refusal, not a netlink test failure.

At the final NTP phase boundary, the repair is committed as `cb44713011` with
the scoped race tests and actual daemon functional test passing, as recorded
below. Independent source/docs review is clean. The final combined scoped lint
run could not produce a plan: foreign `setup/setup.go` changed after its wasip1
stage. No lint success is claimed. Resume with stable inputs for that scoped
check, then finish the already-recorded functional,
corpus, ADD-PATH/LDP and AC-7 report obligations. NTP is fixed; the full refusal
spec remains open while main finishes its scoped verification.

The earlier launcher attempt panicked during command registration before dispatch because
`internal/le/test/unit/pkg.go` supplies Help strings longer than the 60-character
limit (the first reported string is 83 characters at line 86). This file is
foreign dirty work owned by `spec-agent-economy-2-scoped-test`, session
`f4d858bd` and agent `a438d3331bd8f1e98`. The owning session must shorten its
Help strings before a current-source launcher could dispatch. That registration
blocker was subsequently cleared and the named launcher rebuilt for the NTP
proof below. The other commands listed here remain owed; no stale binary
supplies proof.

All four corpus fixtures passed their named draft runs, were promoted, and
landed in `e7a77d6e73`. The shared CPE renderer's focused race proof remains
green. The full corpus has not yet run after those promotions.

The owned pipeline parse-error defect is now repaired: `showPipeline` and
`bestPipeline` return `pipelineRefusal` through `refusePipelineArgs`, preserving
the parser diagnostic and adding the corrected count command. Main observed
`TestPipelineParseErrorsReachCommandVerdict` fail on the old successful error
payloads, then pass with six companion tests under race. Evidence is
`tmp/session/2026-11-11-014000/scratch/job-refusal-pipeline-ntp-red-0f4a55fa.log`
and `job-refusal-pipeline-green-191dfb31.log` in that directory. Independent
source review is clean; five existing test migrations and the functional
fixture change have independent judge approval. The attempted named functional
run stopped at the launcher registration panic, before any fixture ran.

Resume with these commands, through the current named launcher and in the main
thread:

- Run `./le test bgp plugin rib-filter-show` through the rebuilt named launcher.
- Run `TestRepositoryConfigsValidate` under the full feature tags and race.
- `INTEROP_SCENARIO=addpath-zero-id-frr ./le test integration interop` for the
  baseline, the zero-ID omission overlay, and the clean rebuild in that order.
  Preserve effective Go flags when adding the overlay. The overlay and its
  single changed producer are recorded below.
- `INTEROP_SCENARIO=ldp-init-unknown-tlv-frr ./le test integration interop`.
- Complete the remaining scoped checks and independent closure review.

The ledger below now attributes all 54 rows to canonical regressions, retained
history or the dependency spec. Full AC-1 through AC-13 execution and independent
closure review remain incomplete. The older
handover also names Linux, gnmic and BMP evidence to reconcile. The NTP audit
found an absent-provider path. Owner instruction (Thomas, 2026-11-11):
"fix NTP". This spec now owns that repair: an absent NTP state provider must
refuse `show system date`, while a registered inactive provider remains a valid
date-only answer. The prior exclusion below no longer applies to NTP. The
regression was observed red, with inactive and measured-state controls passing,
in `tmp/session/2026-11-11-014000/scratch/job-refusal-pipeline-ntp-red-0f4a55fa.log`.
The repair and independent source/docs review are complete. Main-thread race
proof passes the absent, inactive and measured-state command cases, the existing
date assertions, registry snapshot restoration and NTP-present build test in
`tmp/session/2026-11-11-014000/scratch/job-ntp-provider-green-8e407612.log`.
All three NTP-absent hub tests pass under `ze_core` without `ze_ntp` in
`tmp/session/2026-11-11-014000/scratch/job-ntp-provider-absent-fe550150.log`.
The actual daemon functional `system-date-show` passes 1/1 in
`tmp/session/2026-11-11-014000/scratch/job-ntp-system-date-functional-a1dd9b1c.log`.
The scoped lint run is still pending at this repair checkpoint. This evidence
covers the NTP repair, not the remaining acceptance criteria; the spec stays open.

The first draft reload attempt used `test reload`, which printed a namespace
error. No fixture ran. The correct registered route is `test bgp reload`.
No ADD-PATH or LDP verdict is claimed by this continuation. Neither a stale
image nor a no-build bypass is evidence for these owed runs.

## Retained row evidence audit, 2026-11-11

This ledger accounts for the 54 census rows above. It preserves the producer-red
provenance in `238bdb62e9` and `plan/handover/24-refuse-by-default-implemented-awaiting-closure.md`;
it does not claim those historical runs were repeated. The regression definitions
were located in the current checkout. The prior independent source review is
retained in this session state; closure still owes its independent Review Gate.

`H` means the historical producer-red claim is retained; `R` means a recent
main-thread red/green is recorded above. A named pending run is an evidence
obligation, not a newly observed product defect. The dependency-owned census rows
stay visible rather than being silently removed from the population.

| Row | Producer or surface | Regression (canonical source) | Evidence disposition |
|-----|---------------------|-------------------------------|----------------------|
| 1 | `Server.appendDataPlaneWriter` starts with no FIB writer | `TestConfigPathAutoLoadRefusesMissingDataPlaneWriter` in `internal/component/plugin/server/startup_writer_refusal_test.go`; `TestReloadAutoLoadRefusesMissingDataPlaneWriter` in `internal/component/plugin/server/startup_writer_refusal_test.go` | R; exact main-thread red/green and controls above |
| 2 | `sendPostStartupTo` failure logged at DEBUG | `TestPostStartupFailureWarnsWhileRunning` in `internal/component/plugin/server/poststartup_report_test.go` | H; representative-input regression retained |
| 3 | `LIB.allocateLabelLocked` returns a bound label | `TestLDPLabelExhaustionRefusesSecondBinding` in `internal/plugins/ldp/rfc3031_label_test.go` | H; representative-input regression retained |
| 4 | gNMI `Server.Get` ignores `encoding` | `TestGetRefusesUnadvertisedEncoding` in `internal/component/gnmi/get_test.go`; `TestGetAnswersEachJSONEncoding` in `internal/component/gnmi/get_test.go` | H; changed `TestGnmicInterop` Docker proof pending |
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
| 20 | DHCPv6 `dadUnreported` on VPP | `TestDHCPv6AddressesWithoutDADOutcomeNotReadAsUnique` in `internal/plugins/iface/dhcp/dhcp_v6_dad_read_linux_test.go` | H; Linux DAD-outcome focused run pending |
| 21 | VPP `detailsToInfo` empty addresses | `TestVPPGetInterfaceReportsAddresses` in `internal/plugins/iface/vpp/query_addresses_test.go`; `TestVPPListInterfacesReportsAddresses` in `internal/plugins/iface/vpp/query_addresses_test.go` | H; representative-input regression retained |
| 22 | LG `routes/protocol/{name}` 0 routes for a name | `TestAPIProtocolNameSelectsItsSession` in `internal/component/lg/handler_api_dispatch_test.go` | H; representative-input regression retained |
| 23 | `serializeRouteItem` path ID compared with zero | `TestAddPathIdentifierZeroIsAnIdentifier` in `internal/component/bgp/plugins/rib/ribout_native_test.go` | H; ADD-PATH baseline/negative/clean lab pending |
| 24 | `vpnToJSON`, `LabeledUnicast.String` path ID compared with zero | `TestAddPathIdentifierZeroIsAnIdentifier` in `internal/component/bgp/format/text_test.go` | H; ADD-PATH baseline/negative/clean lab pending |
| 25 | ddos `emitCleared` zero target never closes | `TestClearedCarriesTheDetectedVictim` in `internal/plugins/ddos/detect/characterize_test.go` | H; representative-input regression retained |
| 26 | `NewINET` path ID 0 as absent | `TestAddPathIdentifierZeroIsAnIdentifier` in `internal/component/bgp/rib/route_test.go` | H; ADD-PATH baseline/negative/clean lab pending |
| 27 | gr egress nil state read as nothing stale | `TestLLGREgressFilter_NilStateWithdrawsEBGP` in `internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go`; `TestLLGREgressFilter_NilStateDepreferencesIBGP` in `internal/component/bgp/plugins/gr/rfc9494_gr_egress_test.go` | H; representative-input regression retained |
| 28 | `readOneHwmon` drops a 0.0 reading | `TestDetectThermal_ZeroReadingIsAReading` in `internal/component/host/thermal_linux_test.go` | H; Linux zero-reading focused run pending |
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
| 40 | BMP Loc-RIB ORIGIN literal IGP | `TestLocRIBRouteMonitoringCarriesTheBestPathAttributes` in `internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go`; `TestLocRIBAnnounceWithoutAttributesIsRefused` in `internal/component/bgp/plugins/bmp/rfc9069_bmp_locrib_test.go` | H; `bgp-addpath-best-path-pmacct` pending |
| 41 | bgp-rib `sameBest` ignores an attribute-only change of the winner | `TestRIBBestChangeOnAttributeOnlyChange` in `internal/component/bgp/plugins/rib/rfc4271_rib_bestchange_test.go` | H; representative-input regression retained |
| 42 | trafficusage `ebpfSupported` constant nil | `TestTrafficUsageReadinessCanFail` in `internal/plugins/trafficusage/doctor_test.go`; `TestNewAttacherLinux` in `internal/plugins/trafficusage/attach_linux_test.go` | H; Linux classifier and actual probe as non-root/root pending |
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
| 53 | rib `showPipeline`, `bestPipeline` parse error answered as an `{"error":...}` payload with status done | `TestPipelineParseErrorsReachCommandVerdict` in `internal/component/bgp/plugins/rib/rib_pipeline_refusal_test.go` | R; full RIB package and `rib-filter-show` daemon run pending |
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
- AC-7: run the current native `arch refuse-by-default report`, preferably its
  JSON form, and compare both `findings` and `annotated` by `file` and `fn`
  with every census producer. `runReport` returns zero even with findings; its
  exit status alone cannot satisfy this criterion. No current report is claimed.
- AC-8: retained unit/command evidence includes the recent absent-CoS and writer
  repairs. The full RIB package now passes after the fixture cleanup; updated
  daemon pipeline proof remains pending.
- AC-9: LDP Ze-kernel scenario and the changed gnmic default/PROTO contract
  remain pending; existing RADIUS interop evidence is retained.
- AC-10: ADD-PATH baseline, omitted-zero negative, clean rebuild remain pending.
- AC-11: thermal zero-reading Linux run remains pending; existing LLGR, DDoS
  and dynamic-AS producer-red evidence is retained.
- AC-12: TCX actual-kernel root/non-root proof passes with the explicit
  expectation test below. BMP/pmacct remains pending. The later L2TP and VPP
  stats passes below supersede their old unrun labels.
- AC-13: the ledger preserves all rows and regression attribution; platform
  runs above and independent final audit are still owed. The netlink stats
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

### Scoped execution queue

All commands run in the main thread through a current-source named launcher,
after the foreign dependency addition is coherent. The new registered
`test unit pkg` route supplies the checkout's feature tags, timeout and race
detector; its `run` keyword takes a regex. It accepts no arbitrary Go flags.

1. `test bgp plugin rib-filter-show`. The full native RIB package and focused
   pipeline regressions already passed and are not separately repeated.
2. `test unit pkg path ./internal/component/config/cli/ run '^TestRepositoryConfigsValidate$'`
   once after the four promoted fixtures and shared CPE renderer repair.
3. ADD-PATH baseline, the recorded zero-ID omission overlay, then the clean
   rebuild/run; LDP unknown-TLV scenario follows. Preserve effective `GOFLAGS`
   when adding the overlay. Also run `bgp-addpath-best-path-pmacct`, whose BMP
   attribute payload is an older owed surface of this change.
4. Obtain the actual AC-7 report and examine both refused and annotated
   producer matches. The historical check/stage phase-green and the registrar
   repair's retained passing tests do not require timestamp-refresh runs.
5. `test unit pkg path ./internal/component/host/ run '^TestDetectThermal_ZeroReadingIsAReading$'`
   and `test unit pkg path ./internal/plugins/iface/dhcp/ run '^TestDHCPv6AddressesWithoutDADOutcomeNotReadAsUnique$'`.
   Both use fixtures or scripted backends; these Linux tests need no host
   network capability or QEMU. The DHCP test's existing producer-red claim
   remains retained; no new mutation is required to refresh it.
6. `test unit pkg path ./internal/plugins/trafficusage/ run '^(TestTrafficUsageReadinessCanFail|TestNewAttacherLinux)$'`
   as the ordinary Linux user. The former checks classifier/doctor verdicts;
   the latter really asks the kernel and accepts a measured ready, absent or
   unknown answer. The actual root/non-root probe obligation is now complete
   through the explicit expectation test above; no repetition is owed.
7. For gnmic, use the documented direct `job run` recipe because the test needs
   the additional `docker` tag: full feature tags plus `docker`, `-race`,
   `-run '^TestGnmicInterop$'`, package `./internal/component/gnmi/`, and
   `ZE_REQUIRE_DOCKER=1`. Current `gnmic_docker_test.go` pins gnmic 0.49.1 by
   digest and checks JSON/JSON_IETF advertisement, explicit and default Get,
   PROTO refusal, Subscribe, Set and authentication. `labtest.Run` requires
   Docker on the test server's own network host: the server listens on
   loopback and the client uses host networking. A forwarded guest Docker
   socket cannot stand in for that network identity.

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

The 38 `RFC-approval-owed` trailers in `238bdb62e9` and the historical stale
verdict list remain to reconcile with current native RFC debt/audit metadata.
No blanket reseal, missing-log re-record or foreign audit edit is implied by
this ledger. Scoped lint, documentation checks and independent closure review
remain after the execution evidence; no whole-tree verification is added.


## Prior blocker and completed corpus repair

The continuation built the named launcher from the current checkout on
2026-11-11. The missing inventory fields below are a historical build failure,
not a current refusal to attempt the labs. ADD-PATH baseline, negative control,
clean rebuild and LDP execution remain owed.

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
mechanically updated tests. The four fixture drafts still await execution and
promotion. Concurrent commit `ab4a67d631` carried the shared-rendering paragraph
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

## Mistake Log

| Mistake | Effect | Correction |
|---------|--------|------------|
| The CPE renderer extraction omitted the identity constant's VPN checker and test callers. Its first source review did not find those callers. | `./le test qemu ci test test/reload/static-bfd-profile-reload-deleted.ci` exited 1 while building the QEMU launcher: `check_vpn.go` still referenced `cpeIKEName`. No guest test ran. | Export the single shared `homecpe.CPEIdentity` declaration. Update `checkMgmtTunnelEd25519`, `TestSwanAuthVerdict` and the moved renderer test. Search every moved declaration for remaining callers, then run focused compilation before the guest retry. Independent followup source review is clean. Main-thread execution remains the proof. |

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

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The four rule shapes below cover every owned row | row-by-row reading 2026-10-23 | an owned row has no rule; add one before closing | each owned row's regression test plus a fixture per rule | unvalidated |
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
| `TestRadiusDiscardsEAPAnswerWithoutMessageAuthenticator` | `internal/component/radius/client_test.go` | AC-9, RFC 3579 Section 3.1 | |
| `TestLDPLabelExhaustionRefusesSecondBinding` | `internal/plugins/ldp/lib_test.go` | AC-9, RFC 3031 Section 3.14 | |
| `TestLDPInitUnknownTLVUClearNotifies` | `internal/plugins/ldp/wire_test.go` | AC-9, RFC 5036 Section 3.3 | |
| `TestISISHelloInvalidThreeWayStateDiscarded` | `internal/plugins/isis/rfc5303_invalid_state_wiring_test.go` | AC-9, RFC 5303 Section 3.2: `handleIIH` counts the discarded Hello | |
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
| `show-bgp-peer-bare-list` | `test/plugin/` | web or SSH `show bgp peer` with no key lists entries or names the path | |
| `addpath-identifier-zero-forwarded` | `test/plugin/` | route with identifier 0 under ADD-PATH reaches the peer with four identifier octets | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `addpath-zero-id-frr` | `test/interop/scenarios/`, checkers in `internal/le/interoplab/bgp/` (`scenarioAddPathZeroIDFRR`) | FRR | identifier 0 round trip under ADD-PATH | written, NOT RUN: the Docker lab must run Ze's kernel (macOS `ze` Colima profile or the Ze-kernel QEMU guest). Run: `INTEROP_SCENARIO=addpath-zero-id-frr ./le test integration interop`. It proves acceptance only (vacuity trap 4): Ze's identifier octets come from the session flag in `nlri.WriteNLRI` (reached from `reactor_api_batch.go` `writeBatchNLRI`), which this spec did not change, so reverting the spec's diff cannot redden it. Red phase owed: in `internal/core/bgp/nlri/nlri.go` `WriteNLRI`, change `if addPath && n.SupportsAddPath()` to `if addPath && n.SupportsAddPath() && n.PathID() != 0`, rerun the same command (the harness rebuilds the image from the tree), confirm the checker fails on FRR's missing `AddPath ID: RX 0,` path, restore, confirm green, and record the red |
| `radius-admin-eap-unsigned-freeradius` | `test/interop-radius/scenarios/`, checker `internal/le/interoplab/radius/check_unsigned_eap.go` | FreeRADIUS 3.2.10, through `test/interop-radius/unsigned-relay.pl` in its container (FreeRADIUS's `rad_encode` always signs over UDP) | EAP answer without Message-Authenticator is discarded, logged and counted; the same login with the signature kept completes | green 2026-10-23 (with the four other radius scenarios) |
| `ldp-init-unknown-tlv-frr` | `test/interop/scenarios/`, checker `internal/le/interoplab/bgp/check_ldp_init_unknown_tlv.go` | FRR ldpd; an nft rule in ze's container rewrites one TLV type in FRR's real Init (FRR cannot send an unknown TLV) | unknown U=0 TLV in Init draws an Unknown TLV notification FRR receives, and the Init is ignored (no OPERATIONAL until the rewrite is removed) | |

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
- `internal/test/cosabsent/cos_absent_integration_test.go` (actual absent-CoS binary, replacing the proposed `.ci` whose runner enables all feature gates), `test/plugin/show-bgp-peer-bare-list.ci`, `test/plugin/addpath-identifier-zero-forwarded.ci`
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
| 3 | CLI command added/changed? | No | le only, documented in its help |
| 4 | API/RPC added/changed? | Yes | gNMI Get refusal of unsupported encoding in `docs/architecture/api/` gNMI page |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/health-checks.md` for Unknown health |
| 7 | Wire format changed? | Yes | ADD-PATH identifier 0 presence in the wire page that describes ADD-PATH |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc3579.md`, `rfc3031.md`, `rfc5036.md`, `rfc5303.md`, `rfc7911.md`, `rfc9494.md`, `rfc2205.md` rows and `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/verify-freshness-scope.md` stage list |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/contributing/ze-go-style.md`: refusal over default, the annotation |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | the subsystem telemetry pages of any counter added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
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
