# rpki author handoff (spec-rfc-verdict-fix-bgp, package internal/component/bgp/plugins/rpki)

Author: subagent, 2026-09-28. No verdict stamped, nothing committed. Every new or changed tag below has a
`discriminate-record` record (route revert, OBSERVED red). Package tests green under `-race`
(job-rpki-pkg2-37ecbabd.log). gofmt clean.

All new test functions live in NEW files, so no existing verdict shifted except the two edited units in
`rtr_session_test.go` (which shifts RFC8210-5.11-1, 5.11-4, 7-1, 7-3, 8.1-2, 8.4-1: `./le rfc reseal`, main
thread, when no other child holds uncommitted audit changes).

| id | resolution | what now proves each clause (+ / -) | records written | expected verdict | notes |
|----|-----------|--------------------------------------|-----------------|------------------|-------|
| RFC8210-12-1 | tests | + TestRTRFatalErrorReportDropsTheSession/"transport is closed": codes 0,1,3,5,6,7,8 each fail the sync and the cache reads EOF; - same unit/"load sent after the fatal report": report + full load on one connection, VRP stays NotFound, not synced | pos, neg (producer isFatalError) | enforced | code 4 left to the downgrade path; ze also drops on non-fatal code 2 (RFC 8.4 failover), not claimed |
| RFC8210-7-7 | tests | existing pos/neg for the downgrade-or-terminate clause; + new TestRTRUnknownVersionErrorReportIsNotAnswered (negative tag): v99 Error Report, codes 0 and 4, draws no byte back, sync fails, EOF | neg (readLoop) | enforced | two guards (handlePDU version exemption, readLoop report guard) each suffice alone |
| RFC8210-5.2-1 | tests | + TestRTRStartupSerialNotifyOfAnyVersionIsIgnored/v99 Serial Notify over net.Pipe: no report, Session ID and serial not adopted, sync completes with the EOD serial; - v99 Cache Response in the same window: errRtrUnsupportedProtocol + Error Report 4 | pos, neg (handlePDU); also re-recorded pos/neg on edited TestSerialNotifyIgnoredDuringStartup | enforced | "unexpected protocol version" clause now asserted |
| RFC8210-7-8 | row (retired) | merged into RFC8210-5.2-1 | none | row gone | Retired paragraph in new rfc/corrections/rfc8210.md; row removed from short; both tags deleted from TestSerialNotifyIgnoredDuringStartup (approved D-15); extraction 7:5 -> excluded duplicate-of 5.2-1, 7:6 -> 5.2-1; audit entry deleted by hand (README "drop the id from rfc/audit"); extraction resign-reason added, signed-off 2026-09-28 |
| RFC8210-8.3-1 | tests + mistag | + TestRTRCacheResetReloadsTheWholeSet: one-cache group (no more-preferred cache), wire sequence Reset, Serial, Cache Reset, Reset; new load replaces old set (old VRP NotFound, new Valid); - Serial Query answered with data: next query Serial, delta merged | pos, neg on new unit; pos, neg re-recorded on TestCacheResetTriggersResetQuery | enforced | mistag fixed: "parallel caches" prose replaced in TestCacheResetTriggersResetQuery (approved D-15). SHOULD (more-preferred caches) judged out, as the audit note says |
| RFC6810-6.3-1 | tests | same new unit and tags as 8.3-1 | pos, neg (new + edited unit) | enforced | |
| RFC8210-8.1-3 | tests | + TestRTRResetQueryWithoutFastResyncData: no remembered session, expired lease (serial 42), cache switch (standby with serial 42 and current lease, group holder = other cache), version downgrade (v2 Serial -> v1 Reset) each open with Reset Query; - same cache, current lease: Serial Query carrying Session ID 7 and serial 42 | pos, neg (prepareQuery) | enforced | cache-switch case isolated from the lease check by a current lease |
| RFC8210-5-1 | tests | + TestRTRReservedOctetsIgnoredOnReceipt: Cache Reset with reserved octets 0xFFFF still resets (serial 0); - 0xFFFF in Cache Response octets 2-3 is adopted as Session ID | pos, neg (handlePDU) | enforced | IPv4/IPv6 prefix receipt already held; Router Key is skipped whole; transmit clause already held |
| RFC6810-5.1-3 | tests | same unit: Cache Reset positive, IPv6 Prefix with header zero field, reserved flags and zero octet all ones authorizes 2001:db8::/32; negative as 5-1 | pos, neg | enforced | ze speaks no v0 (rtrVersionMin 1); rows carry {superseded} |
| RFC6810-6.1-1 | tests | + TestRTRPollsAtLeastHourly: EOD with zero interval fields leaves both waits <= 1h; running group re-queries (Reset then Serial) when a 1s wait ends | pos (pollDelay) | enforced (single-polarity kept) | |
| RFC6810-7-1 | tests | + TestRTRUnprotectedTCPFromConfig: parsed trusted-network entry defaults to 323, plugin-started session reads a clear-text v2 Reset Query and syncs, VRP used | pos (connectAndSync) | enforced (single-polarity kept) | port moved to the listener since 323 is privileged |
| RFC8210-9-1 | tests | same | pos | enforced (single-polarity kept) | |
| RFC8210-3-1 | tests | + same unit: unprotected TCP only to a declared trusted-network cache, data used; - `{}` and trusted-network false refused at parse | pos (connectAndSync), neg (parseRPKIConfig) | enforced | TLS path already held |
| RFC8210-10-1 | tests | + TestRTRValidationIgnoresWhichCacheSupplied: plugin startSessions, VRP from preferred vs from standby (preferred refused): Valid/Invalid/NotFound triple identical | pos, neg (validate.go Validate) | enforced, or weak if the judge reads the conditional "if data from multiple caches are held" as unreachable | ze holds one cache's set at a time (holder replacement) |
| RFC6811-2-1 | tests | + TestRouteStateIsSetFromTheLookup (validate_rfc6811_state_test.go): validateNLRIs return and validateCh request state equal the lookup for Valid/Invalid/NotFound; - VRP change Valid->Invalid gives Invalid, not the stale Valid | pos, neg (validateNLRIs) | enforced | adj_rib_in TestBatchValidateTypedMatchesString still tagged (another package, untouched) |
| DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-4-1 | tests | + TestASPAZeroBesideProvidersIsNoWildcard (aspa_verify_zero_test.go): {0,200} = {200} = Valid; - {0,999} = {999} = Invalid | pos, neg (verifyASPA) | enforced | |
| DRAFT-IETF-SIDROPS-8210BIS-5.12-1 (R-7) | tests | + TestASPAEmptyAnnouncementDrawsErrorReport9: one-provider announcement syncs with no byte sent back, provider attested; - zero-provider announcement: Error Report code 9 on the wire encapsulating the PDU, no authorization | pos, neg (parseASPAPDU) | enforced (record in the spec's R-7 table, stem un-enrolled) | new rfc/discrimination/draft-ietf-sidrops-8210bis.json created by the recorder |
| DRAFT-IETF-SIDROPS-8210BIS-5.12-3 (R-7) | tests | + TestASPAProviderListOfZeroOrMoreUniqueAscending: zero-provider withdrawal accepted and applied, unique ascending announcement accepted; - repeated or descending list: errASPAProviderList, nothing published | pos, neg (parseASPAPDU) | enforced (R-7 table) | |
| DRAFT-IETF-SIDROPS-8210BIS-7-2 (R-7) | tests | + TestRTRUnknownVersionErrorReportIsNotAnswered (negative tag) | neg (readLoop) | enforced (R-7 table) | |
| missing rfc8210 §6 SHOULD | row + tests | new row RFC8210-6-6; + TestRTRRetriesAtTheDefaultRetryIntervalBeforeAnySuccess: never-answered cache waits 600s; - after a success with EOD retry 30, a failure waits 30s | pos, neg (poll) | first judgement owed (default stamp mode) | unsourced-ids of section 6 |
| missing rfc8210 §9.2 SHOULD | row | new row RFC8210-9.2-12 {not-applicable: cache-side} | none | not-applicable (first judgement) | quote carries the MUST half, as RFC6810-7.2-9 does; unsourced-ids of 9.2 |
| missing rfc8210 §9.1 MAY | row | new row RFC8210-9.1-5 {not-applicable: SSH} | none | not-applicable (first judgement) | both MAY sentences in one span; unsourced-ids of 9.1 |

Not touched: 8210bis 5.12-2 / 5.12-7 missing rows (route owned by spec-rfc-requirement-reattribution per
the child's constraint).

Gates owed by the main thread: `./le rfc check` (my run showed only STALE verdicts to re-judge, SHIFTED ones
for reseal, and the extraction resign which is now fixed; no other finding on these stems),
`./le go lint run`, independent re-judge of every id above (`mode rejudge`; 6-6, 9.1-5, 9.2-12 first-stamp).

## Files changed

- internal/component/bgp/plugins/rpki/rtr_session_rfc8210_test.go (new)
- internal/component/bgp/plugins/rpki/rtr_transport_rfc8210_test.go (new)
- internal/component/bgp/plugins/rpki/validate_rfc6811_state_test.go (new)
- internal/component/bgp/plugins/rpki/aspa_verify_zero_test.go (new)
- internal/component/bgp/plugins/rpki/rtr_session_test.go (7-8 tags deleted, 8.3-1 prose corrected)
- rfc/short/rfc8210.md (7-8 removed; 6-6, 9.1-5, 9.2-12 added)
- rfc/corrections/rfc8210.md (new, Retired paragraph)
- rfc/extraction/rfc8210.json (7:5, 7:6 remapped; unsourced-ids in 6, 9.1, 9.2; signed-off, resign-reason)
- rfc/audit/rfc8210.json (RFC8210-7-8 entry deleted)
- rfc/discrimination/rfc8210.json, rfc/discrimination/rfc6810.json, rfc/discrimination/rfc6811.json,
  rfc/discrimination/draft-ietf-sidrops-aspa-verification.json (records added by the recorder)
- rfc/discrimination/draft-ietf-sidrops-8210bis.json (new, by the recorder)
- approvals recorded by `./le rfc approve unit` for TestSerialNotifyIgnoredDuringStartup, TestCacheResetTriggersResetQuery and four new units; no tracked test/weakened file holds them (the modified test/weakened files in git status are another session's)
