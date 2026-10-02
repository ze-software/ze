# Services AC-C2 record sweep, 2026-10-02

Scope: every verdict in the services child's owned stems (rfc1350, rfc2131, rfc2132, rfc2181,
rfc2347, rfc2348, rfc3954, rfc4035, rfc4578, rfc7011, rfc7012, rfc7440, rfc7534, rfc7858,
rfc7871, rfc792, rfc7950, rfc8414, rfc8484, rfc9728, sflow-v5; rfc1035, rfc2473 and rfc4213
have no audit file) that is `enforced` now and was not `enforced` at `519d9f4f2a~1` (the tree
before the child's first commit, `519d9f4f2a`, "tftpserver: no DATA resend on a duplicate
ACK (RFC 1350), verdict fixes"). 77 ids: rfc1350 6, rfc2131 4, rfc2132 1, rfc2181 5,
rfc2347 2, rfc2348 3, rfc3954 4, rfc4035 4, rfc4578 1, rfc7011 6, rfc7012 1, rfc7440 1,
rfc7534 2, rfc7858 3, rfc792 5, rfc7950 11, rfc8414 3, rfc8484 1, rfc9728 3, sflow-v5 11;
rfc7871 none. For each stem `./le rfc discriminate stem <stem> '|' json` was read for
`unproven` and `stale`, filtered to the 77 ids.

- 59 unproven units (rfc2132, rfc3954, rfc7950 and sflow-v5 ids already carry current records).
- Stale records: none in scope, and none at all over these stems. Orphans: none.
- No overlap with the BGP or OSPF judges' stems (rfc1071 in `core/probe` is OSPF's and was
  not touched).

Scripts: the routing sweep's, under `tmp/session/2026-10-02-e08980d7-b739-4095-ab66-53539559487d/scratch/c2rs/`:
`scope.sh services 519d9f4f2a~1 ...`, `plan.sh services`, `sweep.sh services <plan>` (revert
route, per-stem flock in `tmp/session/2026-09-28-869df689-cc8f-4d78-9161-1d7c87434c8e/scratch/children/ledger-<stem>.lock`,
first OBSERVED red stops). Results: `c2rs/services/sweep.tsv`, logs under `c2rs/services/logs/`.

## Results

Counts: 59 units owed, 59 OBSERVED reds written (one record per unit, all `route revert`,
all written by `./le rfc discriminate-record`), 0 green findings, 0 orphans, 0 not run.
Net records added over HEAD: rfc1350 +7, rfc2131 +4, rfc2181 +5, rfc2347 +4, rfc2348 +6,
rfc4035 +5, rfc4578 +2, rfc7011 +5, rfc7012 +1, rfc7440 +2, rfc7534 +2, rfc7858 +4,
rfc792 +2, rfc8414 +2, rfc8484 +2, rfc9728 +6 = 59. After the sweep `scope.sh services-after`
finds 0 unproven units and 0 stale records in scope.

Linux guest: RFC1350-4-3 positive `tftpserver/socket_integration_linux_test.go::TestListenTFTPLoopbackRoundTrip`
ran with `kernel ~/.cache/ze/runtime-kernel/7.2-runtime-arm64-runtime-8e843adc-cd0e7a40/vmlinuz`
and observed red on `handler.go::handleRRQ`.

Six units refused R-10 on the plan's producer (the id's other records sit on functions those
units never call) and were recorded in round 2 on the function the unit does call:
RFC7011-8-1 both (`exporter.go::notifySnapshot`, not `exportFlows`), RFC7011-8-2 both
(`config.go::Validate` / `ParseConfig`, not `notifySnapshot`), RFC7858-3.1-1
`TestDoTListener` (`secure.go::bindDoT`, not `DefaultSecureConfig`), RFC9728-5.1-1
`TestOAuth_Authenticate_MissingHeader` (`oauth.go::challengeError`, the 401 challenge carrying
`resource_metadata`). Plan overrides before round 1: RFC792-Echo-2/-3 on `core/probe/icmp.go`
(`BuildICMPEcho`, `icmpChecksum`) instead of the id's existing `iface/netlink` records, and
RFC2131-4.1-6 negative on `dhcpserver/handler.go::parseOptionAddr`.

DUAL review (both polarities of one id in one unit; the record is honest only when the half
its polarity names reaches the producer):
- RFC2131-4.1-6 `TestOptionsContainedWithinTheirField`: positive on `safeAppendOption` (the
  OFFER/ACK build the positive half reads), negative on `parseOptionAddr`, which the negative
  half calls itself on the truncated option. Accepted.
- RFC4035-4.6-3 `TestRFC4035_ResponseADBitDisregarded`: negative on
  `resolver.go::dnssecDecision`, which `query` calls for every response before its rcode
  check, so the negative's SERVFAIL+AD lookup reaches it. Accepted.
- Shared call, both halves read its result, accepted: RFC2347-x-1 and x-3 (one RRQ, one
  OACK), RFC2348-x-3 (every subtest goes through `parseRRQ`), RFC4578-2.1-2 (table over
  `parsePXEArch`), RFC9728-7.1-2 (every TLS-version subtest dials the one server built by
  `loadMCPTLSConfig`).

Producers a judge should look at:
- RFC2181-4.1-1 `geodns/server_test.go::TestRFC2181_UDPReplySourceAndPort` recorded on
  `core/dnsserver/manager.go::bind` (another package; the geodns server binds through it, so
  R-10 passed).
- RFC7858-8-1 both on `core/selfcert/selfcert.go::NewTLSConfig` (another package; the DoT
  listener's TLS config, including its minimum version, comes from it).

## Records written (paths relative to `internal/`, except `cmd/`)

| id | polarity | unit | producer |
|----|----------|------|----------|
| RFC1350-2-2 | negative | `plugins/tftpserver/handler_test.go::TestTFTPConcurrentLimit` | `plugins/tftpserver/handler.go::sendAndWaitACK` |
| RFC1350-2-2 | positive | `plugins/tftpserver/handler_test.go::TestTFTPReadLargeFile` | `plugins/tftpserver/handler.go::sendAndWaitACK` |
| RFC1350-4-3 | positive | `plugins/tftpserver/socket_integration_linux_test.go::TestListenTFTPLoopbackRoundTrip` | `plugins/tftpserver/handler.go::handleRRQ` |
| RFC1350-5-3 | positive | `plugins/tftpserver/handler_test.go::TestTFTPReadEmptyFile` | `plugins/tftpserver/handler.go::serveFile` |
| RFC1350-5-3 | positive | `plugins/tftpserver/handler_test.go::TestTFTPReadExact512` | `plugins/tftpserver/handler.go::serveFile` |
| RFC1350-6-1 | positive | `plugins/tftpserver/handler_test.go::TestTFTPRetransmitOnTimeout` | `plugins/tftpserver/handler.go::sendAndWaitACK` |
| RFC1350-7-1 | positive | `plugins/tftpserver/handler_test.go::TestTFTPRetransmitOnTimeout` | `plugins/tftpserver/handler.go::sendAndWaitACK` |
| RFC2131-4.1-6 | negative | `plugins/dhcpserver/rfc2131_test.go::TestOptionsContainedWithinTheirField` | `plugins/dhcpserver/handler.go::parseOptionAddr` |
| RFC2131-4.1-6 | positive | `plugins/dhcpserver/rfc2131_test.go::TestOptionsContainedWithinTheirField` | `plugins/dhcpserver/handler.go::safeAppendOption` |
| RFC2131-4.2-2 | positive | `plugins/dhcpserver/rfc2131_test.go::TestChaddrIdentifiesClientWithoutClientIdentifier` | `plugins/dhcpserver/handler.go::extractMAC` |
| RFC2131-4.3.1-2 | positive | `plugins/dhcpserver/rfc2131_test.go::TestUnconfiguredParametersOmitted` | `plugins/dhcpserver/handler.go::buildReply` |
| RFC2181-10.3-1 | positive | `plugins/geodns/server_test.go::TestRFC2181_NSCanonicalWithGlue` | `plugins/geodns/server.go::appendNS` |
| RFC2181-11-1 | positive | `plugins/geodns/server_test.go::TestRFC2181_WireNameLimits` | `plugins/geodns/config.go::checkName` |
| RFC2181-4.1-1 | positive | `plugins/geodns/server_test.go::TestRFC2181_UDPReplySourceAndPort` | `core/dnsserver/manager.go::bind` |
| RFC2181-5.4-2 | positive | `component/resolve/dns/cache_test.go::TestRFC2181_CacheReplacesRRSetNoMerge` | `component/resolve/dns/cache.go::put` |
| RFC2181-8-1 | positive | `plugins/geodns/config_test.go::TestRFC2181_TTLSignBitBound` | `plugins/geodns/config.go::parseTTL` |
| RFC2347-x-1 | negative | `plugins/tftpserver/rfc2347_option_negotiation_test.go::TestRFC2347ServerOACKOnlyRequestedOptions` | `plugins/tftpserver/handler.go::handleRRQ` |
| RFC2347-x-1 | positive | `plugins/tftpserver/rfc2347_option_negotiation_test.go::TestRFC2347ServerOACKOnlyRequestedOptions` | `plugins/tftpserver/handler.go::sendOACKAndWait` |
| RFC2347-x-3 | negative | `plugins/tftpserver/rfc2347_option_negotiation_test.go::TestRFC2347ServerIgnoresUnacknowledgedOption` | `plugins/tftpserver/handler.go::handleRRQ` |
| RFC2347-x-3 | positive | `plugins/tftpserver/rfc2347_option_negotiation_test.go::TestRFC2347ServerIgnoresUnacknowledgedOption` | `plugins/tftpserver/handler.go::handleRRQ` |
| RFC2348-x-3 | negative | `plugins/tftpserver/rfc2348_blksize_test.go::TestRFC2348BlksizeRangeEnforced` | `plugins/tftpserver/handler.go::parseRRQ` |
| RFC2348-x-3 | positive | `plugins/tftpserver/rfc2348_blksize_test.go::TestRFC2348BlksizeRangeEnforced` | `plugins/tftpserver/handler.go::parseRRQ` |
| RFC2348-x-4 | negative | `plugins/tftpserver/handler_test.go::TestTFTPReadExact512` | `plugins/tftpserver/handler.go::serveFile` |
| RFC2348-x-4 | positive | `plugins/tftpserver/handler_test.go::TestTFTPReadLargeFile` | `plugins/tftpserver/handler.go::serveFile` |
| RFC2348-x-5 | negative | `plugins/tftpserver/handler_test.go::TestTFTPReadLargeFile` | `plugins/tftpserver/handler.go::serveFile` |
| RFC2348-x-5 | positive | `plugins/tftpserver/handler_test.go::TestTFTPReadExact512` | `plugins/tftpserver/handler.go::serveFile` |
| RFC4035-3-6 | positive | `plugins/geodns/rfc4035_server_test.go::TestRFC4035_NoDNSSECAdditionalProcessing` | `plugins/geodns/server.go::answerQuestions` |
| RFC4035-4.1-1 | positive | `component/resolve/dns/rfc4035_test.go::TestRFC4035_QueryCarriesEDNS0DOAndClearAD` | `component/resolve/dns/resolver.go::query` |
| RFC4035-4.1-4 | positive | `component/resolve/dns/rfc4035_test.go::TestRFC4035_QueryCarriesEDNS0DOAndClearAD` | `component/resolve/dns/resolver.go::query` |
| RFC4035-4.6-3 | negative | `component/resolve/dns/rfc4035_test.go::TestRFC4035_ResponseADBitDisregarded` | `component/resolve/dns/resolver.go::dnssecDecision` |
| RFC4035-4.6-3 | positive | `component/resolve/dns/rfc4035_test.go::TestRFC4035_ResponseADBitDisregarded` | `component/resolve/dns/resolver.go::query` |
| RFC4578-2.1-2 | negative | `plugins/dhcpserver/handler_test.go::TestParsePXEArch` | `plugins/dhcpserver/handler.go::parsePXEArch` |
| RFC4578-2.1-2 | positive | `plugins/dhcpserver/handler_test.go::TestParsePXEArch` | `plugins/dhcpserver/handler.go::parsePXEArch` |
| RFC7011-3.4.1-1 | positive | `plugins/flowexport/ipfix/rfc7011_test.go::TestRFC7011TemplateIDAbove255` | `plugins/flowexport/ipfix/flow_template.go::buildFlowTemplate` |
| RFC7011-8-1 | negative | `plugins/flowexport/rfc7011_test.go::TestRFC7011TemplateNotRetransmittedBeforeInterval` | `plugins/flowexport/exporter.go::notifySnapshot` |
| RFC7011-8-1 | positive | `plugins/flowexport/rfc7011_test.go::TestRFC7011TemplateRetransmittedAtInterval` | `plugins/flowexport/exporter.go::notifySnapshot` |
| RFC7011-8-2 | negative | `plugins/flowexport/rfc7011_test.go::TestRFC7011TemplateRefreshRangeRejected` | `plugins/flowexport/config.go::Validate` |
| RFC7011-8-2 | positive | `plugins/flowexport/rfc7011_test.go::TestRFC7011TemplateRefreshConfigurable` | `plugins/flowexport/config.go::ParseConfig` |
| RFC7012-4-1 | positive | `plugins/flowexport/ipfix/flow_template_test.go::TestIPFIXFlowTemplate` | `plugins/flowexport/ipfix/flow_template.go::buildFlowTemplate` |
| RFC7440-3-1 | negative | `plugins/tftpserver/handler_test.go::TestTFTPParseRRQInvalid` | `plugins/tftpserver/handler.go::parseRRQ` |
| RFC7440-3-1 | positive | `plugins/tftpserver/handler_test.go::TestTFTPParseRRQ` | `plugins/tftpserver/handler.go::parseRRQ` |
| RFC7534-3.5-1 | positive | `plugins/as112/zones_test.go::TestZoneAnswer_ReverseZoneNoData` | `plugins/as112/zones.go::answerQuestions` |
| RFC7534-3.5-2 | positive | `plugins/as112/zones_test.go::TestZoneAnswer_ReverseZoneNoData` | `plugins/as112/zones.go::answerQuestions` |
| RFC7858-3.1-1 | positive | `core/dnsserver/secure_test.go::TestDoTListener` | `core/dnsserver/secure.go::bindDoT` |
| RFC7858-3.4-5 | positive | `core/dnsserver/secure_test.go::TestDoTRobustToIdleConnectionClose` | `core/dnsserver/secure.go::bindDoT` |
| RFC7858-8-1 | negative | `core/dnsserver/secure_test.go::TestDoTRejectsBelowTLS12` | `core/selfcert/selfcert.go::NewTLSConfig` |
| RFC7858-8-1 | positive | `core/dnsserver/secure_test.go::TestDoTListener` | `core/selfcert/selfcert.go::NewTLSConfig` |
| RFC792-Echo-2 | positive | `core/probe/icmp_test.go::TestRFC792EchoRequestCode` | `core/probe/icmp.go::BuildICMPEcho` |
| RFC792-Echo-3 | positive | `core/probe/icmp_test.go::TestRFC792ChecksumValid` | `core/probe/icmp.go::icmpChecksum` |
| RFC8414-2-1 | negative | `component/mcp/as_metadata_test.go::TestFetchASMetadata_MissingIssuer` | `component/mcp/as_metadata.go::asMetadataURL` |
| RFC8414-2-1 | positive | `component/mcp/as_metadata_test.go::TestFetchASMetadata_Success` | `component/mcp/as_metadata.go::fetchASMetadata` |
| RFC8484-6-2 | negative | `core/dnsserver/secure_test.go::TestDoHGetRejectsBadDNSParam` | `core/dnsserver/secure.go::dohRequestBody` |
| RFC8484-6-2 | positive | `core/dnsserver/secure_test.go::TestDoHListener` | `core/dnsserver/secure.go::dohRequestBody` |
| RFC9728-2-2 | positive | `component/mcp/oauth_e2e_test.go::TestNewStreamable_OAuth_MetadataEndpoint` | `component/mcp/oauth.go::writeResourceMetadata` |
| RFC9728-2-2 | positive | `component/mcp/oauth_test.go::TestResourceMetadata_Document` | `component/mcp/oauth.go::writeResourceMetadata` |
| RFC9728-5.1-1 | positive | `component/mcp/oauth_e2e_test.go::TestNewStreamable_OAuth_RejectsMissingBearer` | `component/mcp/streamable_auth.go::resourceMetadataURL` |
| RFC9728-5.1-1 | positive | `component/mcp/oauth_test.go::TestOAuth_Authenticate_MissingHeader` | `component/mcp/oauth.go::challengeError` |
| RFC9728-7.1-2 | negative | `cmd/ze/hub/service_mcp_rfc9728_test.go::TestRFC9728MCPServingTLSVersions` | `cmd/ze/hub/service_mcp.go::loadMCPTLSConfig` |
| RFC9728-7.1-2 | positive | `cmd/ze/hub/service_mcp_rfc9728_test.go::TestRFC9728MCPServingTLSVersions` | `cmd/ze/hub/service_mcp.go::loadMCPTLSConfig` |

Out of scope, noted: 84 ids in these stems hold unproven units but are outside AC-C2 (already
`enforced` at `519d9f4f2a~1`, or not `enforced` now); list in `c2rs/services/allunproven`
minus `newids`.

Files changed by this sweep: `rfc/discrimination/rfc1350.json`, `rfc2131.json`,
`rfc2181.json`, `rfc2347.json`, `rfc2348.json`, `rfc4035.json`, `rfc4578.json`,
`rfc7011.json`, `rfc7012.json`, `rfc7440.json`, `rfc7534.json`, `rfc7858.json`,
`rfc792.json`, `rfc8414.json`, `rfc8484.json`, `rfc9728.json` (records only, all written by
`./le rfc discriminate-record`), and this file. No test, code, audit or row was edited,
nothing stamped or committed. Gates owed by the main thread: `./le rfc check` over these stems.
`bin/le` printed "older than committed sources" on every run; it was not refreshed here.

## Judge (independent, 2026-10-02)

Scope re-derived with `scope.sh judge-services 519d9f4f2a~1` over the 21 stems: 77 ids
newly `enforced`, 0 unproven units and 0 stale records in scope. Net records over HEAD
match the sweep's +59 per stem; the diff removes no record except the one replaced below.

Re-recorded (observed red, `route revert`, replacing the old record):
- RFC8414-2-1 negative `as_metadata_test.go::TestFetchASMetadata_MissingIssuer`: from
  `as_metadata.go::asMetadataURL` to `as_metadata.go::fetchASMetadata`. The refusal the
  negative asserts ("missing issuer") is produced at as_metadata.go:104 inside
  `fetchASMetadata`; `asMetadataURL` only builds the request URL.

Producer spot-checks, accepted:
- RFC2181-4.1-1 positive on `core/dnsserver/manager.go::bind`: `bind` listens on the one
  configured endpoint, which is what makes the kernel source every UDP reply from the
  queried address, and geodns serves through it. Note: `TestRFC2181_UDPReplySourceAndPort`
  queries 127.0.0.1 from 127.0.0.1, so a wildcard bind would also pass it; the discriminating
  unit is `rfc2181_clarifications_test.go::TestRFC2181ReplySourcedFromTheQueriedAddress`
  (127.0.0.2 queried from 127.0.0.1), recorded on the same producer. Verdict holds.
- RFC7858-8-1 both polarities on `core/selfcert/selfcert.go::NewTLSConfig`: production DoT
  takes its config from it (`dnsserver/tlsmaterial.go:39,51`, `secure.go:258`), and it sets
  `MinVersion: tls.VersionTLS12` and leaves the suite list to crypto/tls. The records prove
  reach of that Ze-owned config. Go servers also default to a TLS 1.2 floor, so the
  stack-level refusal under 1.2 does not depend on that one field alone.
- RFC4035-4.1-1/-4 on `resolver.go::query` (`SetEdns0` at resolver.go:382);
  RFC9728-5.1-1 e2e on `streamable_auth.go::resourceMetadataURL` (the value
  `challengeError` puts in `resource_metadata`); RFC2131-4.1-6, RFC4035-4.6-3 DUAL reviews
  re-read.

`./le rfc check` (2026-10-02, after the re-record): 37 violations, none in this child's
stems (all in rfc2328, rfc9552, rfc9086, rfc4271, rfc4456, rfc8277, rfc9190).

AC-C2 for this child: met for all 59 units. No verdict changed and no audit file was stamped.
