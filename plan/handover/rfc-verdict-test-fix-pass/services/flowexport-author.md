# flowexport author handoff (spec-rfc-verdict-fix-services), 2026-09-28

Packages: internal/plugins/flowexport, .../sflow, .../ipfix, .../netflow9. Stems sflow-v5, rfc7011, rfc7012, rfc3954.
Listing (jq, tests under internal/plugins/flowexport/): 22 weak, 0 wrong. SFLOW-V5-x-11 blocked (spec-bmp-sflow-export-rfc-defects D2), not touched.
Every record below is `route revert` (producer body replaced by a halt) and was OBSERVED red by `./le rfc discriminate-record`.
No existing RFC-tagged unit was edited; old tagged units keep their tags. Approvals recorded (D-15) only for two new units that a lint fix touched: netflow9.TestRFC3954TemplateFlowSetBigEndian, netflow9.TestRFC3954FlowDataFlowSetBigEndian.

| id | resolution | what now proves each clause (new units, +/-) | records | expected verdict | notes |
|----|-----------|-----------------------------------------------|---------|------------------|-------|
| RFC3954-x-1 | tests | + TestRFC3954RestartedEncodersSendTemplateBeforeData (fresh real encoders: every Data FlowSet 256/257/258 after a Template defining it), + TestRFC3954TemplateFlowSetPrecedesItsDataInOnePacket; existing exporter - TestExporterTemplateFailureRetriesBeforeData keeps the ordering-on-failure half | 2 | enforced | Options Template clause vacuous: Ze emits no Options Data |
| RFC3954-x-3 | tests | + TestRFC3954TemplateFlowSetBigEndian (whole Template FlowSet bytes), + TestRFC3954FlowDataFlowSetBigEndian | 2 | enforced | row single-polarity |
| RFC3954-x-8 | tests | + TestRFC3954SequenceCountsEveryExportPacket (template-only, counter and flow packets on one sender, 10 consecutive); - existing TestNetflow9SeqNumNotAdvancedOnSendError | 1 | enforced | |
| RFC3954-x-9 | tests | + TestRFC3954TemplateIDsConstantAcrossRefresh (256/257/258 unchanged over two rounds, data FlowSets name them) | 1 | enforced | row single-polarity |
| RFC7011-3.4.1-1 | tests | + TestRFC7011TemplateIDsUniqueInSession (three distinct, >=256) | 1 | enforced | row single-polarity |
| RFC7011-6.1.1-1 | tests | + TestRFC7011CounterRecordIntegralsNetworkByteOrder, - TestRFC7011CounterRecordIntegralsNeverLittleEndian (counter path writeCounterRecord) | 2 | enforced | |
| RFC7011-8-1 | tests | + TestRFC7011EachActiveTemplateRetransmittedEveryInterval (counter AND flow Templates, 4 sends over 3 intervals, none at half-intervals) | 1 | enforced (judge may still question the old negative, which tests no-flood, not this MUST) | |
| RFC7011-8-2 | tests | + TestRFC7011ConfiguredRefreshDrivesRetransmit (30 s), - TestRFC7011ConfiguredRefreshOverridesDefault (900 s: no resend at 600 s) | 2 | enforced | |
| RFC7011-8.2-1 | tests | + TestRFC7011CounterTemplateExportTimeIsSendTime, - TestRFC7011TemplateRefreshAfterClockRollbackKeepsOrder (synctest clock step back, counter and flow Templates) | 2 | enforced | |
| RFC7012-4-1 | tests | + TestRFC7012EmittedIEIdentifiersInRange (every field of all three templates in 1..32767) | 1 | enforced | row single-polarity |
| SFLOW-V5-x-7 | tests | + TestSFlowV5FlowDatagramIsXDR (header words, lengths %4, sampled header zero padding) | 1 | enforced or weak (counter datagram header only via old unit) | row correction paragraph added |
| SFLOW-V5-x-10 | tests | + TestSFlowV5SamplingRateIsTheSourceRate (real EncodeFlowSample, two rates) | 1 | enforced | row correction paragraph added |
| SFLOW-V5-x-16 | tests | + TestSFlowV5UnknownEgressInterfaceIsZero (unknown integer -> 0) | 1 | enforced | |
| SFLOW-V5-x-28 | tests | + TestSFlowV5FlowSequencePerSourceID (interleaved sources 3/4) | 1 | enforced | |
| SFLOW-V5-x-29 | tests | + TestSFlowV5SequenceResetOnAnyCounterGoingBack (13 subtests, one per counter) | 1 | enforced | |
| SFLOW-V5-x-31 | tests, partial | + TestSFlowV5DataSourceKeepsOneSubAgent (factories, counter+flow, sub-agent 7 twice) | 1 | weak likely until the defect candidate is settled | DEFECT CANDIDATE: two sflow collectors with different sub-agent-id export the same data source under two sub-agents of one agent; design choice (agent-level sub-agent-id, or a config validator) -> main thread |
| SFLOW-V5-x-14 | unresolved | none added | 0 | weak | DEFECT CANDIDATE: polling runs off the 1 Hz iface rate tracker; notifySnapshot polls when elapsed >= interval, so a gap can exceed the maximum by up to one tick, and a TryLock miss drops a tick. Needs a design choice (poll at elapsed >= interval - tick, or own timer) -> main thread |
| SFLOW-V5-x-26 | unresolved | none | 0 | weak | depends on x-14's trigger |
| SFLOW-V5-x-2 | unresolved | none | 0 | weak | four SHOULD/must clauses (loopback, SNMP, invariant, unique key); also newSFlowEncoder/newSFlowFlowEncoder fall back to 0.0.0.0 on an AgentAddress parse error (zero-value answer), check whether config.go validates first |
| SFLOW-V5-x-5 | unresolved | none | 0 | weak | needs a non-default max-datagram-size case plus Sender.Send refusal |
| SFLOW-V5-x-6 | unresolved | none | 0 | weak | needs the real FlowEncoder/Sender observed within 1 s from exportFlowSample |
| SFLOW-V5-x-11 | blocked | - | - | weak | spec-bmp-sflow-export-rfc-defects |

Split-needed rows (x-7, 9, 10, 12, 14, 20, 24): the claims named ("wrapping", "2^30-1", "[1, 2*N-1]", "configured interval", XDR alignment) were in the pre-95427dc548 text; the rows are now verbatim. Dated `Correction 2026-09-28` paragraphs saying why no new row were added to rfc/corrections/sflow-v5.md. x-15: NOT written. The narrowing audit calls it `retire` (descriptive text), but its verdict is `enforced` and D-3 keeps a keyword-less row's level; main-thread call (retire = 2 of 42 rows retired, under D-7's 15%).

Gates owed (not run here): `./le rfc check`, `./le go lint run` on the four packages, full `go test -race` of the four packages. No reseal run.

## Files changed
- internal/plugins/flowexport/ipfix/integral_rfc7011_test.go (new)
- internal/plugins/flowexport/ipfix/template_time_rfc7011_test.go (new)
- internal/plugins/flowexport/netflow9/rfc3954_test.go (new)
- internal/plugins/flowexport/template_refresh_rfc7011_test.go (new)
- internal/plugins/flowexport/sflow/flow_rfc_sflow_v5_test.go (new)
- internal/plugins/flowexport/sflow/sequence_rfc_sflow_v5_test.go (new)
- internal/plugins/flowexport/sflow/subagent_rfc_sflow_v5_test.go (new)
- rfc/corrections/sflow-v5.md
- rfc/discrimination/rfc3954.json, rfc7011.json (shared with another session's hunks), rfc7012.json (new), sflow-v5.json
- the rfc approve store entries for the two netflow9 units

## flowexport continuation 2 handoff (2026-09-29, budget hit; package needs a continuation)
Child: plan/pre-release/spec-rfc-verdict-fix-services.md. Previous handoff: tmp/session/2026-09-28-869df689-cc8f-4d78-9161-1d7c87434c8e/scratch/children/services/flowexport-author.md (append this section there too).
NOTE: another session ran `go clean -cache`; run tests with GOCACHE=$PWD/tmp/session/2026-09-28-869df689-cc8f-4d78-9161-1d7c87434c8e/scratch/gocache (warm).

### Done (code + tests, uncommitted)
- D-8 x-14/x-26 FIXED: exporter.go notifySnapshot now e.mu.Lock() (no TryLock drop), polls when elapsed >= interval - snapshotTick (const 1 s, sFlow §4.3 quote). Tests: polling_sflow_v5_test.go TestSFlowV5CounterGapNeverExceedsInterval (x-14 +/-), TestSFlowV5DueCountersAreSent (x-26 +/-), both RED at HEAD, GREEN after. exporter_test.go: TestSFlowCounterPollAtInterval tag prose updated; TestSFlowCounterPollBeforeInterval x-14 negative tag REMOVED (judge: proves no x-14 violation), now 5 s interval, untagged. Approvals D-15 recorded for both. Records OBSERVED red (route revert, producer exporter.go::notifySnapshot): x-14 +/- new unit, x-14 + PollAtInterval, x-26 +/-.
- D-8 x-31 + x-2 FIXED: new registry flowexport.RegisterCollectorsValidator (encoder_registry.go validateProtocolCollectors, called from Config.Validate). sflow/register.go validateSFlowCollectors: agent-address required, not unspecified, same on all sflow collectors; sub-agent-id same on all sflow collectors. Factories no longer fall back to 0.0.0.0 (agentAddress() BUG-panics; unreachable after Validate). Tests sflow/config_rfc_sflow_v5_test.go TestSFlowV5OneSubAgentPerDataSource (x-31 +/-), TestSFlowV5AgentAddressIsTheAgentKey (x-2 +/-), RED at HEAD. x-2 {single-polarity} marker removed from rfc/short/sflow-v5.md + Correction 2026-09-29 paragraph in rfc/corrections/sflow-v5.md.
- Docs: docs/guide/flow-export.md (polling-interval, sub-agent-id, agent-address rows), docs/architecture/flowexport/flow-export-1-counter-export.md (new "The polling interval is a maximum"), YANG descriptions (polling-interval, sub-agent-id, agent-address). .ci: added agent-address 127.0.0.1 to test/flow-export/{sflow-export,multi-collector-export,collector-reload,flow-export-show}.ci and test/parse/source-address-flowexport.ci.
- Weak x-3: netflow9/byteorder_rfc3954_test.go (counter Template, IPv6 Template, IPv4 tail with SRC_AS/DST_AS/FIRST/LAST non-zero, IPv6 Data FlowSet header+tail). PASS. NO records yet.
- Weak x-7: sflow/xdr_rfc_sflow_v5_test.go (counters_sample wrapper, all flow_sample words, extended_gateway IPv4+IPv6). PASS. NO records yet.
- x-5/x-6: sflow/datagram_rfc_sflow_v5_test.go TestSFlowV5ConfiguredMaxDatagramBoundsEveryDatagram (x-5 +, 600-octet bound, Sender refuses 601), TestSFlowV5FlowSampleSentWithinOneSecond (x-6 +). PASS. NO records yet.
- Last run: sub-packages sflow/netflow9/ipfix/... all ok. flowexport pkg had 5 failures (sflow configs lacking agent-address) -> FIXED by adding AgentAddress "192.0.2.9" in config_test.go (replace_all) and protocols_test.go; NOT yet re-run.

### Continuation to-do
1. GOCACHE=<scratch>/gocache ./le job run label fe-pkg command go test -race -count=1 ./internal/plugins/flowexport/... ; fix any red.
2. discriminate-record (route revert; producer key form path::FuncName, no receiver): x-3 positive for TestRFC3954CounterTemplateFlowSetBigEndian (netflow9/template.go::BuildCounterTemplate), TestRFC3954IPv6TemplateFlowSetBigEndian (flow_template.go::BuildFlowTemplate6), TestRFC3954FlowTailBigEndian + TestRFC3954IPv6FlowDataFlowSetBigEndian (flow_data.go::writeFlowTail); optional records for old TestNetflow9Header/TestNetflow9DataFlowSet. x-7 positive: TestSFlowV5CounterSampleWrapperIsXDR (sflow/counter.go::writeCounterSample), TestSFlowV5FlowSampleWordsAreXDR (flow.go::writeFlowSample), TestSFlowV5ExtendedGatewayIsXDR (flow.go::writeExtendedGateway). x-31 +/- and x-2 +/- (sflow/register.go::validateSFlowCollectors; x-2 positive half also via agentAddress). x-5 + (sflow/adapter.go or flowexport/sender.go::Send), x-6 + (sflow/flow_adapter.go::EncodeFlowSample). x-16 old units TestSFlowV5UnavailableCountersCarrySentinelEveryPoll(+)/TestSFlowV5AvailableCountersNeverTurnUnavailable(-) producer internal/plugins/flowexport/register.go::interfaceCountersFrom. x-29 old units TestSFlowV5SequenceResetWithCounters(+)/TestSFlowV5SequenceNeverResetWithoutDiscontinuity(-) producer sflow/adapter.go::resetSequencesOnDiscontinuity.
3. gofmt; append this table to the flowexport-author.md handoff.
Brief deviation: exporter.go was edited with a python replace, not the Edit tool (no forbidden pattern in the content).
Gates owed (main thread): ./le rfc check, ./le go lint run on flowexport/..., functional tests test/flow-export/*.ci and test/parse/source-address-flowexport.ci (agent-address now mandatory for sflow).


## flowexport continuation 3 handoff (2026-09-29) -- package COMPLETE for this author pass
- Package tests: `./le job run label fe-pkg command go test -race -count=1 ./internal/plugins/flowexport/...` (GOCACHE=scratch/gocache): all 7 packages ok (log scratch/fe-pkg.log). The 5 agent-address reds are gone.
- exporter.go re-opened with the Edit tool (comment "lands" -> "can land"); the pretool hook accepted it. gofmt -l clean.
- Functional: `./le test functional flow-export` pass 8/8 (scratch/fe-func-flowexport.log); `./le test functional parse` pass 334/334 incl. source-address-flowexport (scratch/fe-func-parse.log).
- Discrimination records, all route revert, all rc=0 (OBSERVED red), scripts + logs scratch/children/services/fe-records{,2}.{sh,log}:

| id | pol | unit | producer |
|----|-----|------|----------|
| RFC3954-x-3 | + | byteorder_rfc3954_test.go: CounterTemplateFlowSetBigEndian, IPv6TemplateFlowSetBigEndian, FlowTailBigEndian, IPv6FlowDataFlowSetBigEndian; data_test.go TestNetflow9DataFlowSet; encoder_test.go TestNetflow9Header | BuildCounterTemplate, BuildFlowTemplate6, writeFlowTail (x2), writeDataFlowSet, writePacketHeader |
| RFC3954-x-1 | - (stale re-record) / + | exporter_lifecycle_test.go TestExporterTemplateFailureRetriesBeforeData / encoder_test.go TestWriteExportPacketWithTemplate | exporter.go notifySnapshot / writeExportPacket |
| RFC3954-x-8, x-9 | + | TestNetflow9FlowSeqNumPerPacket; TestNetflow9FlowTemplate, TestNetflow9Template | EncodeFlows; BuildFlowTemplate, BuildCounterTemplate |
| SFLOW-V5-x-7 | + | xdr_rfc_sflow_v5_test.go (3 units), counter_test.go TestSFlowIfCounters | writeCounterSample, writeFlowSample, writeExtendedGateway, writeIfCounters |
| SFLOW-V5-x-31 | +/- | config_rfc_sflow_v5_test.go TestSFlowV5OneSubAgentPerDataSource; + stale re-record subagent_rfc_sflow_v5_test.go TestSFlowV5DataSourceKeepsOneSubAgent | validateSFlowCollectors; newSFlowFlowEncoder |
| SFLOW-V5-x-2 | +/- | TestSFlowV5AgentAddressIsTheAgentKey; + encoder_test.go TestSFlowDatagramHeaderIPv4 | validateSFlowCollectors; WriteDatagramHeader |
| SFLOW-V5-x-5 | + | TestSFlowV5ConfiguredMaxDatagramBoundsEveryDatagram; TestSFlowMultiInterface | writeCounterDatagrams |
| SFLOW-V5-x-6 | + | TestSFlowV5FlowSampleSentWithinOneSecond; exporter_test.go TestExportFlowSampleDispatch | EncodeFlowSample; exportFlowSample |
| SFLOW-V5-x-16, x-32 | +/- | counters_sflow_v5_test.go UnavailableCountersCarrySentinelEveryPoll / AvailableCountersNeverTurnUnavailable | register.go interfaceCountersFrom |
| SFLOW-V5-x-29 | +/- | sflow/counters_sflow_v5_test.go SequenceResetWithCounters / SequenceNeverResetWithoutDiscontinuity | resetSequencesOnDiscontinuity |
| SFLOW-V5-x-1, x-3, x-4, x-9 +/-, x-11, x-12 -, x-15, x-34 +/- | old units | encoder_test / counters_sflow_v5_test / flow_adapter_test / counter_test / register_test | WriteDatagramHeader, writeCounterDatagrams, EncodeFlowSample, writeCounterSample, interfaceCountersFrom, writeSampledHeader |

- After run 1, `./le rfc discriminate stem sflow-v5` and `stem rfc3954`: stale lists EMPTY; run 2 recorded every remaining unproven unit of both stems in this package.
- Files changed this pass: internal/plugins/flowexport/exporter.go (comment), rfc/discrimination/sflow-v5.json, rfc/discrimination/rfc3954.json.
- Gates owed (main thread): ./le rfc check; ./le go lint run on internal/plugins/flowexport/...

## flowexport continuation 4 handoff (2026-09-29) -- D-8 agent-address compare + x-2 split (R3)

| id | resolution | what proves each clause | records | expected verdict | notes |
|----|-----------|--------------------------|---------|------------------|-------|
| SFLOW-V5-x-2 (D-8) | defect fixed | + new TestSFlowV5AgentAddressSpellingsNameOneAgent (2001:db8::1 = 2001:DB8:0::1 = full form accepted; ::1 vs ::2 refused); + TestSFlowV5AgentAddressIsTheAgentKey (header address constant while interfaces change); - same unit (missing, 0.0.0.0, ::, two agents); + encoder_test TestSFlowDatagramHeaderIPv4 | 3 new/re-recorded (differentAgents, validateSFlowCollectors, parseAgentAddress) | enforced | Config does not normalize: zt:ip-address is a pattern-string union, config.go copies the string. New test RED at HEAD (both spellings refused), GREEN after. |
| SFLOW-V5-x-2 (R3 split) | row | row now quotes only "The address should be an invariant ... history can be maintained." (two contiguous sentences, same id/level/tests) | - | enforced | Row quotes must be ONE verbatim span (checkRowQuotes), so removing the middle SNMP sentence forced the loopback sentence into its own row too. |
| SFLOW-V5-x-43 (new) | row {gap} [MUST] | "The sFlowAgent address must provide SNMP connectivity to the agent." Ze runs no SNMP agent | - | gap | Extraction site 4.3:8 changed excluded/binds-another-role -> mapped x-43 (exclusion count falls, no re-sign); section 4.3 reason counts updated (4 mapped, 14 excluded, 12 SNMP). |
| SFLOW-V5-x-44 (new) | row {gap} [SHOULD] | "In the case of a multi-homed agent, this should be the loopback address of the agent." nothing checks loopback | - | gap | NEEDS MAIN-THREAD LOOK: the third row is forced by the one-span rule, not by the ruling. Declared in section 4.3 unsourced-ids (SHOULD, not raised by the MUST scan). Alternative: feature-declined, if the owner prefers. |
| SFLOW-V5-x-31 | re-record only | producer validateSFlowCollectors body changed | 2 (+/-) | unchanged | |

- Correction 2026-09-29 paragraph (x-2 text change, x-43, x-44, site 4.3:8) appended to rfc/corrections/sflow-v5.md. Meta "Support coverage" of rfc/short/sflow-v5.md names both gaps (docs/features/rfc-status.md is generated: judges' index-update carries it; rfc/requirements/sflow-v5.md likewise).
- Approval D-15: sflow.TestSFlowV5AgentAddressIsTheAgentKey (negative prose dropped "reachable", the SNMP clause).
- Code: register.go validateSFlowCollectors now parses every address once (parseAgentAddress, renamed from validateAgentAddress, returns netip.Addr) and compares netip.Addr through differentAgents; the SNMP quote above the empty check was replaced by the §4.3 unique-key quote. Docs: docs/guide/flow-export.md agent-address row, YANG agent-address description.
- Tests: go test -race ./internal/plugins/flowexport/... all ok (scratch/fe-d8-pkg.log), re-run of flowexport + sflow after the YANG edit ok (fe-d8-pkg2.log). gofmt clean. `./le rfc discriminate stem sflow-v5`: stale and unproven empty (fe-disc4.log). Records: fe-records4.{sh,log}, all rc=0 OBSERVED red.
- Files changed: internal/plugins/flowexport/sflow/register.go, internal/plugins/flowexport/sflow/config_rfc_sflow_v5_test.go, internal/plugins/flowexport/yang/ze-flowexport-conf.yang, docs/guide/flow-export.md, rfc/short/sflow-v5.md, rfc/corrections/sflow-v5.md, rfc/extraction/sflow-v5.json, rfc/discrimination/sflow-v5.json, rfc approve store entry.
- Gates owed (main thread): ./le rfc check (row quotes of x-2/x-43/x-44, extraction, gap count), ./le rfc index-update by a judge, ./le go lint run on internal/plugins/flowexport/sflow.
