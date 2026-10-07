# Test weakenings this commit accepts

| Test | Reason |
|------|--------|
| TestSelfOwnedPeerUpReplaysFlowSpecBeforeReady | Removed a mock assertion on the old readiness command spelling, not the FlowSpec replay assertions. `TestPeerReadySDKProducers` exercises actual SDK producers, command consumers and captured peer-session completion. |
| TestDispatchBGPPeerSessionReady | Removed the dispatch/mock echo of the retired receipt-free command. Actual readiness command consumption and session/producer fencing are covered by `TestPeerReadySDKProducers` and the peer-ready receipt tests. |
| dispatch_test | The assertion-count reduction is the deleted mock-only readiness test recorded above; other dispatch tests remain. |
| persist.newSessionReadyHarness | Removed the private command-string recorder with its mock-only tests; the replacement SDK producer tests consume the real command and observe the peer fence. |
| persist.record | Removed the recorder method with the retired persist readiness harness. |
| persist.reported | Removed the string-search method with the retired persist readiness harness. |
| addPeer | Removed setup used only by the retired persist command-string tests. |
| TestPersistReportsSessionReadyOnBothCompletionPaths | Retired the mock-only command-spelling test. `TestPeerReadySDKProducers` proves persist's empty-replay SDK-to-consumer completion and stale/current token behavior; it does not prove populated persist replay completion. The old command recorder did not prove consumer readiness either. |
| TestPersistDoesNotReportForAStaleGeneration | Retired the mock spelling check. Session/producer receipts now reject stale completion at the actual peer-ready consumer; the peer-ready receipt tests exercise that boundary. |
| TestPersistReportsSessionReadyFromThePeerUpEvent | Replaced a mock-only readiness command observation with the actual SDK producer and peer-session fence proof. |
| session_ready_test | The persist file's assertion count reflects replacement of its recorder-based tests, not removal of the real completion obligation. |
| TestRIBPluginEventLoopBlocking | Removed a test that required the known event-loop blocking delay and counted RPCs. `TestRIBPluginSentEventsSurviveReplay` instead drives the external SDK handshake and verifies that both exact route identities survive through the production refresh-command parsers. It does not claim TCP delivery or absence of blocking. |
| consumeRefreshCommand | Assertions moved into `parseRefreshCommand`; the wrapper still invokes them and retains its original per-group nonnil Wire assertion. Existing tagged callers lose no check. Only the new external event test accepts either valid attribute representation while asserting exact consumed NLRIs. |
| TestPeerUpReflectsExistingFlowSpecBeforeEOR | Removed only a mock assertion on the obsolete receipt-free readiness string. FlowSpec replay assertions remain; actual readiness consumption is proved by the SDK producer and peer-ready receipt tests. |
| TestPeerUpReplaysExistingFlowSpecBeforeEOR | Removed only a mock assertion on the obsolete receipt-free readiness string. FlowSpec replay assertions remain; actual readiness consumption is proved by the SDK producer and peer-ready receipt tests. |
| TestPeerUpSignalsSessionReady | Replaced the watchdog mock command capture with the watchdog case in `TestPeerReadySDKProducers`, which reaches the actual command consumer and peer fence. |
| server_test | The watchdog assertion-count reduction is the retired mock-only readiness test recorded above; other server tests remain. |
| TestNLRIWireAtRefusesAnNLRIPastTheBlock | Removed with the obsolete private `nlriWireAt` admission helper. The final writer now uses registered native splitters; `adjOutWrite.section` returns their boundary errors before any managed write. The old private slicing API no longer exists. |
| adj_rib_out_test | The count reduction records removal of the obsolete private NLRI slicing unit above. Ownership is now tested at actual writer and forwarding boundaries, not by seeding the former table. |
| probeWithdrawOnlyBody | Removed with the owner-approved retirement of the mock 200-allocation probe. The actual-writer nonallocation test constructs and retains its own 200-identifier UPDATE. |
| TestWithdrawOnlyUpdateFreesEveryIdentifierItBuys | Owner explicitly approved retirement of the incorrect allocation expectation and RFC7911 claim. `TestRSWithdrawalOwnershipUnknownPathIDsDoNotAllocate` proves no wire withdrawal and zero mappings while the 200-ID cached UPDATE is still retained. The surviving identifier-uniqueness test and claims remain unchanged. |
| rfc7911_zz_pathid_growth_probe_test | The count reduction is the approved obsolete probe removal above; its full-message boundary is retained at the actual forwarding/writer path rather than an updateHook mock. |
| rr.newSessionReadyHarness | Removed the private command-string recorder with the retired RR mock-only readiness tests; real SDK producer and peer-fence tests replace it. |
| rr.record | Removed the recorder method with the retired RR readiness harness. |
| rr.reported | Removed the string-search method with the retired RR readiness harness. |
| TestRRReportsSessionReadyOnEveryPathOutOfReplay | Retired the mock-only command-spelling test. `TestPeerReadySDKProducers` proves RR's empty-replay SDK-to-consumer completion and stale/current token behavior; it does not inject RR's replay-error exit. RIB-only populated/error carriers are not claimed as RR coverage. |
| TestRRDoesNotReportForAStaleGeneration | Replaced the obsolete receipt-free command observation with consumer-side stale-session and producer-receipt rejection tests. The production completion fence, not a captured string, decides readiness. |
