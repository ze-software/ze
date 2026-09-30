# pppoe author handoff (spec-rfc-verdict-fix-access, package internal/component/l2tp/pppoe, stem rfc2516)

Author: subagent, 2026-09-29. No verdict stamped, nothing committed.

## Product change (test seam, no behavior change)

`internal/component/l2tp/pppoe/server.go`: two nil-by-default fields on `InterfaceServer`,
`pppoeCreateFn` and `devPPPSetupFn` (same shape as the existing `sendFrameFn`), routed through
two new helpers `createTransport` and `setupPPPChannel`, which `handlePADR` now calls instead of
`pppoeCreate` / `ppp.DevPPPSetup`. Production never sets them. They let an unprivileged test drive
the FRESH-admission path of `handlePADR` and observe the SESSION_ID and peer MAC the session
stage is bound to. An RFC 2516 §4/§6 quote comment sits above the `createTransport` call.
Because `handlePADR` changed, its three existing records were re-recorded (5.3-5 neg, 5.4-2 pos, 5.4-3 pos).

## Verdicts

| id | resolution | what now proves each clause (+/-) | records written | expected verdict | notes |
|----|-----------|------------------------------------|-----------------|------------------|-------|
| RFC2516-5.2-1 | tests | + TestRFC2516AdmittedPADSCarriesTheNewSessionsID (PADS from handlePADR echoes each host's Host-Uniq byte-for-byte); + TestBuildPADS now compares Host-Uniq bytes, not length; + TestBuildPADO (PADO, unchanged); - TestBuildNoHostUniqEcho (unchanged) | + Admitted (BuildPADS), + TestBuildPADS (BuildPADS) | strong | |
| RFC2516-5.2-2 | tests | + TestRFC2516PADOCarriesACNameAndServiceNames (handlePADI: exactly one AC-Name == configured, first Service-Name == PADI's, then the other offered); +/- TestPADOAlwaysCarriesServiceName (unchanged) | + PADOCarriesACName (BuildPADO) | strong | |
| RFC2516-5.2-4 | tests | + TestRFC2516UnservedServiceNameIsRefused (handlePADI sends nothing for an unserved name); - same unit (served name draws one PADO). Tags removed from TestServiceNameFilter (predicate only; approved D-15) | +/- (handlePADI) | strong | |
| RFC2516-5.4-2 | tests | + TestRFC2516UnservedServiceNameIsRefused (present, unserved Service-Name -> PADS with Service-Name-Error, SID 0, no session, no socket); + TestPADRWithoutServiceNameGetsServiceNameError (unchanged, re-recorded); - TestRFC2516AdmittedPADSCarriesTheNewSessionsID (served name -> non-zero SID, no Service-Name-Error). Negative tag removed from TestBuildPADS (builder-only; approved), its record deleted as orphan | + Unserved (handlePADR), + PADRWithoutServiceName (re-record), - Admitted (handlePADR) | strong | |
| RFC2516-5.4-3 | tests | + TestRFC2516AdmittedPADSCarriesTheNewSessionsID (fresh admission of two hosts: CODE 0x65, SID non-zero, == table SID, the two differ); + TestRFC2516PADSCarriesTheSessionsID (replay path, unchanged, re-recorded); - pppoeclient TestRFC2516PADSWithZeroSessionIDIsARefusal (unchanged) | + Admitted (handlePADR), + PADSCarriesTheSessionsID (re-record) | strong | |
| RFC2516-5.5-3 | tests (AC half); Host half queued: pppoeclient | + TestBuildPADT (tag moved here from x-6: CODE 0xa7, SID, unicast destination; approved D-15); + TestRFC2516SessionDownSendsPADTForThatSession; - TestRFC2516UnknownSessionIDTerminatesNothing | + TestBuildPADT (BuildPADT) | weak until the Host-sent PADT (pppoeclient/dialer.go::sendPADT dst + SID) is tagged | queued: pppoeclient |
| RFC2516-7-1 | tests (tags moved off the PADT-MAC test) | + TestRFC2516AdmittedPADSCarriesTheNewSessionsID (session socket bound once to the SID the PADS assigned); - TestRFC2516PADRReplayKeepsTheSessionID (retransmitted PADR -> same SID, no second socket, one session). 7-1 tags removed from TestHandlePADTVerifiesMACAndSID (PADT source-MAC matching, not the §6 sentence; approved D-15; now carries VALIDATES/PREVENTS) | + Admitted (createTransport), - Replay (matchLiveCookie) | strong | the kernel keeps the SID fixed after connect; Ze's share is the bind, which is asserted |
| RFC2516-7-3 | no change (AC half held); Host half queued: pppoeclient | + TestRFC2516SessionDownSendsPADTForThatSession; - TestRFC2516UnknownSessionIDTerminatesNothing | none | weak until a pppoeclient unit shows the Host stops using the session on LCP termination | queued: pppoeclient |
| RFC2516-x-4 | tests + marker removed | + TestParseEndOfListTag (first sentence, unchanged); + TestRFC2516NoSentFrameCarriesEndOfList (PADO and PADS carry no End-Of-List TAG, raw TLV walk); - same unit (PADI/PADR carrying End-Of-List TAG_LENGTH 4 -> reply carries none). `{single-polarity}` marker deleted from the row (markers are peeled, quote unchanged) | + (BuildPADO), - (parseTags) | strong | |
| RFC2516-x-5 | tests | + TestRFC2516RefusingPADSEchoesRelaySessionID (refusing PADS and admitting PADS from handlePADR carry the Relay-Session-Id unmodified); - same unit (refusing PADS with no relay in PADR carries none); TestRelaySessionIDEcho/NoEcho unchanged | +/- (BuildPADSError) | strong | |
| RFC2516-x-6 | tests + marker removed | + TestRFC2516AdmittedPADSCarriesTheNewSessionsID (session socket bound to the PADR's unicast source MAC, per host); - TestRFC2516SessionNeverBoundToANonUnicastPeer (broadcast and multicast-sourced PADR refused by ParseDiscovery before HandleDiscovery, no socket bound; unicast binds to itself). TestBuildPADT tag moved to 5.5-3. Wrong `{single-polarity}` marker deleted from the row | + Admitted (createTransport), - NonUnicast (ParseDiscovery) | strong | the negative mirrors discoveryReader's parse-then-dispatch inline; handlePADR itself relies on ParseDiscovery for the unicast guard |
| RFC2516-x-7 | tests | + TestRFC2516BuildersWriteTheSendersMAC (PADI, PADR, PADO, PADS, refusing PADS, PADT write the given sender MAC in SOURCE_ADDR); - receive-side broadcast/multicast source refusals (unchanged) | + (NewBuilder) | strong | |
| RFC2516-x-9 | queued: pppoeclient | only tag is pppoeclient/session_test.go::TestLCPConfigRequestMRU | none | unchanged | queued: pppoeclient |

Counts: tests 10 (5.2-1, 5.2-2, 5.2-4, 5.4-2, 5.4-3, 7-1, x-4, x-5, x-6, x-7); partly done + queued 2 (5.5-3, 7-3 Host halves); queued 1 (x-9). Rows corrected: 0 (two markers removed, quote text unchanged). Defects: 0. Blocked: 0.

## Records

`rfc/discrimination/rfc2516.json`: 21 records written by `./le rfc discriminate-record` (route revert), every one observed red; one orphan deleted by hand (RFC2516-5.4-2 negative on TestBuildPADS, whose tag is gone). Logs: `rec-RFC2516-*-Test*.log` in this directory; scripts `pppoe-rec.sh`, `pppoe-rec1.list`, `pppoe-rec1b.list`, `pppoe-rec2.list`. The first batch hit a transient go-cache wipe from another session ("already failing ... could not import"); the retry recorded all.

Approvals (P-1, D-15): pppoe.TestBuildPADS, pppoe.TestBuildPADT, pppoe.TestServiceNameFilter, pppoe.TestHandlePADTVerifiesMACAndSID.

## Tests run

`go test -count=1 ./internal/component/l2tp/pppoe/` under `./le job run label pppoe-t3`: ok (log `pppoe-t3.log`). gofmt clean. This was before the last approval/tag edits to discovery_test.go and server_test.go (comment-only plus the Host-Uniq bytes.Equal); the records in batch two re-ran those units green before their breaks.

## Gates owed (main thread)

`./le go lint run` over the package (the edit hook showed a typecheck error in `internal/component/l2tp/tunnel_fsm.go` `errSCCRQUnknownMandatoryAVP` from another session's in-progress l2tp work, not this change), `./le rfc check`, and the judge's reseal/stamp for rfc2516.

## Notes for the main thread

- `rfc/audit/rfc2516.json` shows a `./le rfc reseal` diff in the working tree that this author did not run (a judge's).
- The existing pppoeclient unit tagged by RFC2516-5.4-3 negative was not touched.

## Files changed

- internal/component/l2tp/pppoe/server.go
- internal/component/l2tp/pppoe/admission_rfc2516_test.go (new)
- internal/component/l2tp/pppoe/discovery_test.go
- internal/component/l2tp/pppoe/server_test.go
- rfc/short/rfc2516.md
- rfc/discrimination/rfc2516.json
