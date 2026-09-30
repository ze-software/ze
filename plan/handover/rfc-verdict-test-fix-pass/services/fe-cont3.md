
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
