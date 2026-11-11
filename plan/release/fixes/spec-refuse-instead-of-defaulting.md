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

Phase 7 remains open. The 54-row regression ledger and the completed native
runs below cover the implemented repairs. The retained AC-7 report exposed
three owned shapes; their explicit-error/ParseError-recording implementation is
now stable, while main formatting and green proof remain pending. AC-7 is
unchanged, and this implementation needs no waiver if its proof meets the
original criterion. The
AC-4 direct-state-assignment and forward package-alias repair is committed as
`06a4a22cbc`; the full race retry passes in `job-unit-pkg-ef92ffff.log`. This
reused path now holds the run after the forward-alias fix, superseding its
previous contents. Focused state/naming controls pass in
`job-refusal-state-and-rfc-green-f471a48b.log`. The package coverage run reports
86.5% in `job-refusal-state-coverage-2e5fed92.log`; its retained profile,
`refusal-state-coverage.out`, still has eight uncovered blocks. No 100% coverage
claim is made. Literal AC-7 remains open until main proves the stable
implementation described below; no waiver or owner answer is required if it
meets the original criterion.
The expanded carrier census still awaits main's final source verification.

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
a new daemon interop run. The full RFC package failed in
`job-unit-pkg-d25f06c4.log`: the naming rule still included excluded development
tool tests, and the package reached its 20-minute timeout. Maintenance in
`internal/le/rfc/names.go` (`testFileNameVerdicts`) and
`render_ledger_test.go` has independent source review and test-judge approval;
Focused naming controls now pass in
`job-refusal-state-and-rfc-green-f471a48b.log`; the full RFC retry remains owed.
The LOCAL_PREF final audit stamp completed on 2026-11-11. All four owned SHIFT
reseals are complete against the source head: RFC5549-4-1, RFC5549-4-4,
RFC7911-2-2 and RFC8950-4-1. LOCAL_PREF and ASPA audit changes and these reseals
remain dirty and uncommitted. The new AC-7 signature migration owes 16 native
unit/polarity renewals across RFC7311-3.4.3-1 through RFC7311-3.4.3-6, plus
independent rejudgment for requirements 2, 4 and 6. These pending renewals prevent
any claim that the whole expanded 43-ID scope is fresh. Scoped lint,
documentation checks and final source/review gates remain owed; native closure
review has not started.

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
- AC-7: the retained report listed 2155 refused and 72 annotated shapes in
  7000 Go files. Comparison with all 54 producers found two notify parse-error
  branches and the withdrawal builder's guarded zero. Main's new regression
  observed exactly those two notify hits and one builder hit in retained run
  `f0b4a2f9`; four callback controls passed. The stable patch makes
  `buildWithdrawalPayload` return a length and an explicit error with static
  failure classes; `fwdWithdrawal.of` releases its borrowed buffer on error.
  `Reactor.notifyOneMessage` records parse failure visibly and clears a stale
  record when the modified wire parses. Independent actual-patch generic/style
  review by `refusal_signature_judge` is CLEAN. Main formatting, focused green
  and the actual report comparison remain pending. Literal AC-7 is unchanged;
  proving this implementation meets it needs no owner waiver. The separate BMP
  send defect is committed and proved, including the actual BMP/pmacct pass.
  The retained source/report census is `refusal-ac7-census.md`; its three old
  matches describe the red baseline, not a verdict on the unproved new patch.
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

### Scoped execution queue

Execution stays in the main thread against current source in the main checkout.
The named native `test unit pkg` route supplies the checkout's feature tags,
timeout and race detector; its `run` keyword takes a regex.

1. Main formats and proves the stable AC-7 patch, then compares the actual
   report with owned producers under the unchanged criterion. The observed red
   and independent patch review are complete; no green is claimed yet.
2. Complete the 16 native unit/polarity renewals for RFC7311-3.4.3-1 through -6
   and the assigned audit rejudgments for -2, -4 and -6. Mechanical caller
   migrations of `TestRFC7311DirectLinkCostNeverWrapsOrDisappears` and
   `TestAIGPForwardedMetricsReachFinalWire` have independent test-judge approval.
   Complete the full RFC retry and commit the completed LOCAL_PREF, ASPA and
   four owned SHIFT audit changes through the native route. Do not claim the
   expanded 43-ID scope fresh before its pending renewals are recorded.
3. Resolve the four scoped-lint modernize findings for embedded composite
   literals in this spec's `internal/plugins/iface/netlink/stats_linux_test.go`,
   then finish bounded lint, documentation checks, final source audits and
   independent review. The lint run has no passing verdict yet. Completed AC-4,
   full RPKI and promoted identity functional/stress proofs need no timestamp
   refresh. Closure and its formal Review Gate follow only when the remaining
   obligations are complete.

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

The 38 `RFC-approval-owed` trailers in `238bdb62e9` are historical: `origin/main`
already contains that commit. Current `readPersonActs` in
`internal/le/commit/owed_trailers.go` reads only the selected target's commits
outside upstream, so those published trailers are not current push debt.
Recent native checkpoint preparations reported `debt=nothing owed`; main still
checks `commit debt-status commit HEAD` for later commits. The historical stale
RFC verdict list remains a separate audit-metadata obligation. Main's earlier
native `rfc check` exited one with 64 violations in
`job-refusal-rfc-current-check-a3c28c29.log` in session scratch. These are not
all attributed to this spec. The helper-only `1aa91d030b` change shifted the ASPA audit verdict for unchanged
`TestASPARetentionReceiveRecovery` and `TestASPARetentionReplacementWithdrawal`;
its scoped mechanical reseal is prepared and awaits commit. The owned closure
population is the exact 37 requirement IDs derived in
`scratch/refusal-rfc-closure-scope.md` under this session. The LOCAL_PREF negative
enrollment, final LOCAL_PREF audit stamp and unit-carrier repair are recorded
above. All four owned SHIFT IDs, RFC5549-4-1, RFC5549-4-4, RFC7911-2-2 and
RFC8950-4-1, are resealed against the source head and await commit. The AC-7
caller migration adds the 16 RFC7311 renewals and three rejudgments recorded in
the current queue; the expanded 43-ID scope is not declared fresh. No blanket reseal, missing-log re-record or foreign
audit edit is implied by this ledger.
Scoped lint, documentation checks and independent closure review remain after
the execution evidence; no whole-tree verification is added.


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
| Malformed NLRI / withdrawal action, AC-9/10 | BGP `format/text_json.go` `appendUndecodedNLRIJSON` publishes raw/error information instead of a placeholder route; `reactor/forward_build.go` `buildWithdrawalPayloads` splits the two-family withdrawal | Preserve decodable sibling routes and withdraw each refused family through its proper field. `TestUndecodableNLRIIsNeverWrittenAsARoute` and `TestMixedFamilyUpdateRefusedIsWithdrawnPerFamily` cover these paths. The stable AC-7 patch gives `buildWithdrawalPayload` an explicit error result, and `fwdWithdrawal.of` releases the borrowed buffer on error; main green/report proof remains pending. |
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
| AC-7 | The tree report continues listing unrelated refused and annotated shapes so their owners can judge them. Its exit zero is a reporting convention, not acceptance. The three retained red-baseline rows now have a stable explicit-error/visible-recording patch; main must prove its actual report meets the original criterion. No waiver is needed if that proof passes. | `Report` / `runReport`, `TestRefuseByDefaultReportListsTree`, and the exact producer/consumer census recorded above. |
| AC-8 | Successful reads, typed not-found, no requested CoS profile, a valid pipeline and an inactive registered NTP provider remain usable. These are real outcomes, unlike failed measurement or an absent provider. Valid command objects/arrays and no-data answers also survive scalar refusal. | `TestResolveAbsentProviderPermitsNoProfile`, `TestNetlinkGetStatsZeroCounters` / `MeasuredCounters`, `TestSystemDateAcceptsInactiveNTPProvider` / `CarriesMeasuredNTPState`, valid pipeline/count controls, and `RawJSON` controls above. |
| AC-9 | JSON/JSON_IETF gNMI Gets, valid debug syntax, an LDP unknown U=1 TLV, a free last label, a valid IS-IS state, complete RSVP PATH and IPv6 BFD with bind-v6 remain admitted. A non-EAP unsigned RADIUS answer survives only the existing explicit unsigned-response policy; it does not exempt unsigned EAP. | `TestGetAnswersEachJSONEncoding`, gnmic interop; LDP U=1 controls and `TestLDPLabelExhaustionRefusesSecondBinding`; IS-IS/RSVP controls; `TestIPv6SessionAcceptedWithBindV6`, `TestIPv4MappedPeerNeedsNoBindV6`; unsigned non-EAP control in `TestRadiusDiscardsEAPAnswerWithoutMessageAuthenticator`. RFC 5036 Section 3.3 permits U=1 processing while requiring U=0 notification and message rejection. |
| AC-10 | Under ADD-PATH, zero and maximum are identities; without it the prefix stays free of an identifier regardless of the stored number. Re-advertisement retains negotiated framing and Ze's destination identifier. | `TestAddPathIdentifierZeroIsAnIdentifier` in each canonical consumer, `TestLabeledUnicastStringKeepsAPathIdentifierOfZero`, `addpath-identifier-zero-forwarded.ci`, and the actual FRR baseline/negative/clean. RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE message, the NLRI encoding MUST be extended by prepending the Path Identifier field, which is of four octets." |
| AC-11 | Fresh LLGR routes survive missing stale state; thermal 0.0 is measured data; a clear names the resolved victim; a known peer AS stays numeric and a pre-negotiation event stays unknown. ExaBGP peers retain their documented auto-AS zero spelling at the compatibility boundary. Monitor preserves unknown as text and malformed input as raw data; RPKI still validates known-AS UPDATEs and removes routes on ASN-free state-down. EPE preserves its existing AS-zero up-event refusal while admitting valid identity and cleaning up non-up sessions. | `TestLLGREgressFilter_NilStateDoesNotSuppressFreshRoute`, thermal/DDoS/dynamic-AS tests in the ledger, `TestUnknownPeerASIsExabgpAuto`, monitor `TestFormatMonitorLineRemoteAS`, `TestFormatMonitorLineRemoteASNull` and `TestFormatMonitorLineRemoteASInvalid`, RPKI `TestJSONUpdateRequiresPeerAS` / `TestJSONPeerDownWithoutASClearsTrackedRoutes`, and EPE `Source.State`. These zeros have distinct explicit contracts, and none supplies a fabricated learned peer AS. |
| AC-12 | Measured readiness, actual addresses, the recorded best/losing route, the best path's own BMP attributes and established-bypass RRO flags remain reportable. An inapplicable health check has no row; failed measurement keeps its reason. A real all-zero counter record remains valid. | L2TP healthy/data-plane controls, root/non-root TCX probe, VPP address tests, `TestShowRIBRowsSayWhichPathWon`, BMP attribute and healthy-collector controls, `TestRFC4090FlagsSetOnceBypassEstablished`, direct netlink zero/measured controls. |
| AC-13 | Each owned row's normal input continues to produce the useful outcome its canonical regression names; each representative defect input must still discriminate against its producer break. Dependency-owned rows retain their separate owner and proof. | The 54-row ledger names each producer and regression. Historical reds remain attributed to `238bdb62e9`; later repairs retain their exact observed negative/clean evidence. This does not convert pending literal AC-7 or remaining final audit work into completion. |

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
| 14 | Prometheus counters added/changed? | Yes | the subsystem telemetry pages of any counter added; `docs/guide/rpki.md` for `ze_rpki_updates_refused_total` and its `unknown-peer-as` / `malformed-peer-as` reason labels |
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
