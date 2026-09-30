# BGP child: capability + fsm author (first pass), 2026-09-30

Packages: internal/core/bgp/capability (16 ids), then internal/component/bgp/fsm (12 ids).
No commit, no stamp. Records under per-stem flock. Logs: scratch/children/bgp/capfsm-*.log.

| id | resolution | what now proves each clause (+/-) | records written | expected verdict | notes |
|----|-----------|-----------------------------------|-----------------|------------------|-------|
| RFC8654-3-2 | tests | + TestRFC8654ExtendedMessageWireIsCode6Length0 (WriteTo writes octet 06 at offset, Code()==6, Parse(06 00) -> *ExtendedMessage) | 3-2 positive (revert writeCapabilityTo) | enforced (row keeps {single-polarity: positive}) | old tag on TestCapabilityCodeConstants left as is |
| RFC8654-3-3 | tests + row | + TestRFC8654ExtendedMessageWireIsCode6Length0 (exact 06 00, Len 2, parse ok); - TestRFC8654ExtendedMessageNonZeroLengthRefused (06 01 00 and 06 04 ... refused, ErrInvalidLength) | positive + negative (revert parseZeroLengthCapability) | enforced | removed the false {single-polarity} annotation from rfc/short/rfc8654.md: a refusal path exists (parseZeroLengthCapability) |
| RFC4724-3-2 | tests | + / - TestRFC4724ReceiverIgnoresReservedRestartFlags: clean nibble parses to R/time; nibble with reserved 0x3 set parses identical (R clear and set). Sender half stays on TestGracefulRestartEncodeReservedBits | positive + negative (revert parseGracefulRestart) | enforced | only 0x3 used: RFC 8538 assigned the N bit (0x4) |
| RFC4724-3-3 | tests | + / - TestRFC4724ReceiverIgnoresReservedAddressFamilyFlags: AF flags 0x00/0x80 vs |0x7F parse identical | positive + negative (revert parseGracefulRestart) | enforced | |
| RFC4760-8-1 | tests + row (was wrong) | + TestRFC4760OpenAdvertisesEveryConfiguredFamily (config ipv4/ipv6 unicast + l2vpn/evpn -> Ze's built OPEN carries one MP cap each; Negotiate with peer offering them -> supported); - TestRFC4760FamilyNotAdvertisedIsNotExchanged (ipv6/unicast mode disable -> no MP cap; Negotiate vs peer offering it -> unsupported) | positive + negative (revert reactor/config.go::parseFamiliesFromTree) | enforced | removed {single-polarity: positive} from rfc/short/rfc4760.md (a negative now exists). Old parse-level tag on capability TestParseCapabilities left (supplementary) |
| RFC4684-5-2 | tests | +/- TestRFC4684RTCAdvertisedAsMultiprotocolPair: ipv4/rtc configured -> OPEN carries MP (1,132), negotiated with a peer offering it; mode disable -> absent, not negotiated | positive + negative (revert parseFamiliesFromTree) | enforced | rfc/short/rfc4684.md Enrolment reason + Support remaining prose said 5-2 "carries no test": corrected |
| RFC7911-4-1 | tests (was wrong) | + TestRFC7911OpenCarriesOneAddPathInstance (default send/receive over 2 families -> exactly one code-69 instance listing both, mode 3); - TestRFC7911PerFamilyAddPathStaysOneInstance (per-family overrides only, 3 families, directions 2/1/3 -> still one instance with all three) | positive + negative (revert reactor/config_capabilities.go::parseAddPathFromTree) | enforced | old parse-level tags on TestAddPathCapability / TestAddPathMultipleFamilies left in place (judge may call them supplementary; moving them needs D-15 approvals) |

Files changed:
- internal/component/bgp/reactor/rfc_open_family_capability_test.go (new; reactor package, R6: new file, no other agent holds it)
- rfc/short/rfc4760.md (RFC4760-8-1 annotation removed)
- rfc/short/rfc4684.md (two prose cells)
- rfc/discrimination/rfc4760.json, rfc4684.json, rfc7911.json (records)
- internal/core/bgp/capability/rfc4724_reserved_bits_test.go (new)
- internal/core/bgp/capability/rfc8654_extended_message_test.go (new)
- rfc/short/rfc8654.md (RFC8654-3-3 annotation removed)
- rfc/discrimination/rfc4724.json, rfc/discrimination/rfc8654.json (records)

# Continuation 2 (2026-09-30)

| id | resolution | what now proves each clause (+/-) | records written | expected verdict | notes |
|----|-----------|-----------------------------------|-----------------|------------------|-------|
| RFC8950-4-2 | tests | + TestRFC8950PeerSupportDecidedPerPair (both sides list <1,1,2>,<1,128,2> -> NH AFI 2 for both, Negotiated and EncodingCaps); - TestRFC8950PeerWithoutThePairGetsNoIPv6NextHop (peer lacks a pair / lists <1,128,1> / no cap -> 0) | positive + negative (revert negotiated.go::Negotiate), log capfsm-rec3.log | enforced | file internal/core/bgp/capability/rfc8950_extnh_negotiation_test.go (new) |
| RFC5549-4-2 | tests | + TestRFC8950PeerSupportDecidedPerPair | positive (revert Negotiate) | enforced (row single-polarity positive, superseded by 8950-4-2) | negative tag removed from TestRFC8950PeerWithoutThePairGetsNoIPv6NextHop (approval D-15 recorded) |
| RFC4271-8.2.2-13 (defect part) | defect fixed (D-8) | OpenSent + Event 18 now -> Active (RFC 4271 §8.2.2 OpenSent: "changes its state to Active"), fsm.go handleOpenSent, quote above the statement; VIOLATION 1 removed from the fsm.go header (renumbered, one in-file reference fixed). Failing first: TestRFC4271ConnectRetryCounterQuietOnTCPFailureInConnectAndOpenSent (approved D-15; now table Connect->Idle, OpenSent->Active) and untagged TestFSMExhaustiveTransitions row red, then green (go test -race fsm ok) | negative re-recorded (revert fsm.go::handleOpenSent), capfsm-c2-rec-13.log | still weak: row's ConnectRetryTimer/resources/TCP-drop clauses and Event 25 not asserted (see 8.2.2-7..17 group) | docs: fsm-open-sent.md (table row, deviation section removed, note added), fsm-active.md (entry from OpenSent). Reactor: no code reads OpenSent->Idle specifically (peer_run.go callback only acts on Established edges) |
| RFC5492-3-2 | tests | + TestRFC5492UnrecognizedCapabilitiesIgnoredAtTheSession (handleOpen: codes 253/254 interleaved with MP IPv4 + required RR -> OpenConfirm, one KEEPALIVE, RR + IPv4 negotiated); - TestRFC5492UnrecognizedCapabilityNeverReadAsKnown (R1(b): code 254 carrying the MP IPv6 value -> accepted, IPv6 NOT negotiated) | +/- (revert reactor session_handlers.go::handleOpen), capfsm-c2-rec-5492.log | enforced | new file internal/component/bgp/reactor/rfc5492_open_session_test.go. HEAD negative on capability TestParseRejectsMalformedKnownCapabilityLength left in place (neighbour; judge may call supplementary) |
| RFC5492-4-1 | tests | + TestRFC5492RepeatedCapabilityInstancesAcceptedAtTheSession (RR x3, MP IPv4 x2, MP IPv6 x2 through handleOpen -> OpenConfirm, one KEEPALIVE, all negotiated) | positive (revert handleOpen) | enforced (row keeps {single-polarity: positive}) | session-level gap the audit named is closed |
| RFC5492-4-2 | tests | + TestRFC5492CapabilitiesSpreadOverSeveralParameters (3 Capabilities params: RR / ExtMsg / 254 -> OpenConfirm, both negotiated); - TestRFC5492EveryCapabilitiesParameterReadBeforeRefusing (required RR+ExtMsg; param pairs missing one -> NOTIFICATION 2/7 Data exactly the capability absent from ALL params, Idle) | +/- (revert handleOpen) | enforced | HEAD negative on TestOptionalParamRejectsTruncatedCapabilityTLV left (neighbour) |
| RFC4271-4.4-2 | tests | + TestRFC4271NonZeroNegotiatedHoldSendsPeriodicKeepalives (ze hold 90/keepalive 1 s vs peer hold 9 via handleOpen+handleKeepalive -> timers hold 9 s, KeepaliveTimer running, >=1 periodic KEEPALIVE in 1.6 s); - TestRFC4271ZeroNegotiatedHoldSendsNoPeriodicKeepalive (peer 0 / ze 0, keepalive 1 s configured -> timers hold 0, timer stopped, zero octets in 1.6 s) | + revert session_negotiate.go::negotiateWith; - revert fsm timer.go::StartKeepaliveTimer (capfsm-c2-rec-44.log) | enforced | new file internal/component/bgp/reactor/rfc4271_keepalive_session_test.go; old fsm timer_test.go tags left (supplementary). Closes the audit's "negotiated, not configured, hold reaches the timers + wire" gap |

## Continuation 2: unresolved (not started, no half-edits)

| id | state | what it needs |
|----|-------|---------------|
| RFC4271-8.2.2-7..12, 14..17 (10 ids) and the clause remainder of 8.2.2-13 | unresolved | Each quote lists NOTIFICATION / ConnectRetryTimer-to-zero / release resources / drop TCP (and for 12, 14 delete routes) actions the fsm package does not perform. Reactor-level units can prove NOTIFICATION code on the wire, Conn()==nil (drop) and the counter via Session.SetConnectRetryCounter. "Sets the ConnectRetryTimer to zero" has no Session/Timers observable (Ze's ConnectRetryTimer is the peer run loop, fsm.go ARCHITECTURAL NOTES) and "releases all BGP resources" is unbounded: main thread should decide R3 split (timer/resources clauses into their own rows) vs a peer-level test, once, for the whole group. 8.2.2-7 AutomaticStart (Event 3) is absent in Ze: R3 {gap} split candidate. |
| RFC7911-5-1, 5-2; RFC7752-3.2-1; RFC9552-5.2-7 | not started | budget |

## Continuation 2: files changed
- internal/core/bgp/capability/rfc8950_extnh_negotiation_test.go (RFC5549-4-2 negative tag line removed)
- internal/component/bgp/fsm/fsm.go (handleOpenSent Event 18 -> StateActive with §8.2.2 quote; header VIOLATION 1 removed, renumbered; in-file "VIOLATIONS 2" ref -> 1)
- internal/component/bgp/fsm/rfc4271_connect_retry_test.go (TestRFC4271ConnectRetryCounterQuietOnTCPFailureInConnectAndOpenSent: per-state expected target)
- internal/component/bgp/fsm/fsm_test.go (TestFSMExhaustiveTransitions OpenSent_TCPConnectionFails -> StateActive, comment)
- internal/component/bgp/reactor/rfc5492_open_session_test.go (new)
- internal/component/bgp/reactor/rfc4271_keepalive_session_test.go (new)
- docs/architecture/behavior/fsm-open-sent.md, docs/architecture/behavior/fsm-active.md
- rfc/discrimination/rfc8950.json, rfc5549.json, rfc4271.json, rfc5492.json (records)
- approvals: capability.TestRFC8950PeerWithoutThePairGetsNoIPv6NextHop, fsm.TestRFC4271ConnectRetryCounterQuietOnTCPFailureInConnectAndOpenSent

Verified: race fsm ok, capability ok; reactor TestRFC5492* and the two NegotiatedHold tests with race ok; lint (job capfsm-lint) on fsm, capability, reactor: 0 issues; ./le rfc check: owned stems show only STALE verdicts on the re-authored ids (expected, judges re-judge), no record or tag refusals (log capfsm-c2-rfccheck.log). OWED: full reactor race result (below); ./le verify worktree (main thread).
Full reactor race run (job capfsm-race-reactor): ok, 93 s.
