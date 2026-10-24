# Spec: BGP one-off defects from the journal

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Each row is incorrect shipped BGP behaviour (wire, RIB, FSM, plugin relay) or a test that cannot prove it; none is an absent feature |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The 2026-10-23 journal sweep grouped 1,350 open rows into thirteen root-cause specs (S1 to S7, T1 to T6) and
left about 265 genuine one-offs: single defects whose mechanism belongs to no
root cause. This spec owns the 67 BGP ones: the reactor, the wire codecs, the
RIB, the route server relay, MRT, BMP's dedup memo and the BGP CLI decoders.

Every row is a claim by its author on the day it was written. The first step of
each AC is to read the producer at HEAD and reproduce the defect with a test
that goes red. A row found already fixed moves to the "Rows found already
fixed" table with the commit or symbol that fixed it; it is never silently
dropped.

Rows left to root-cause specs: the rpki origin-AS pair (S4), `Session.SendAnnounce`
(S5), the ExaBGP bridge argv rows (S7) and the ze-peer OPEN mirror rows (T5).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` and the BGP door named in `ai/doors/README.md`
- [ ] `docs/architecture/core-design.md` - WireUpdate, buffer-first encoding
- [ ] `docs/architecture/bgp/structural-forwarding.md` - the relay rails
- [ ] `plan/journal/README.md` - row format, how a row is closed

### RFC Summaries (Scope: protocol)
- [ ] `rfc/full/rfc4271.txt` Sections 4.3, 5, 8.2.2
- [ ] `rfc/full/rfc7606.txt` Section 3
- [ ] `rfc/full/rfc7311.txt` Section 3.2
- [ ] `rfc/full/rfc8277.txt` Section 2.4
- [ ] `rfc/full/rfc8654.txt` Section 6
- [ ] `rfc/full/rfc9252.txt` Section 5
- [ ] `rfc/full/rfc8669.txt` Section 3.1
- [ ] `rfc/full/rfc5082.txt` Section 3
- [ ] `rfc/full/rfc2545.txt` Section 3
- [ ] `rfc/full/rfc7999.txt`, `rfc/drafts/draft-simpson-idr-flowspec-redirect-02.txt`

## Current Behavior (MANDATORY)

Read at the producers named in each owned row. Spot-checked on 2026-10-23:
`isConnectionError` in `internal/component/bgp/plugins/rr/rr.go` still matches
only "broken pipe", "connection reset" and "use of closed network connection",
so row 2026-10-22 (`normal-state-logged-at-error`) is present.

- [ ] `internal/component/bgp/reactor/session_negotiate.go` - `buildOpen`
- [ ] `internal/component/bgp/reactor/session_prefix.go` - `checkPrefixLimits`, `forEachPrefixEntry`
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` - `AnnounceNLRIBatch`
- [ ] `internal/component/bgp/reactor/reactor_api_relay.go` - `buildRelayUpdate`
- [ ] `internal/component/bgp/reactor/forward_rs.go` - `reactorForwardRSSection`
- [ ] `internal/component/bgp/reactor/peer_bfd.go` - `stopBFDClient`
- [ ] `internal/component/bgp/reactor/link_scope.go` - `peerOnLink`
- [ ] `internal/component/bgp/reactor/reactor_iface.go` - `refreshPeerLinkScopes`
- [ ] `internal/component/bgp/fsm/fsm.go` - `FSM.change`, `FSM.Event`
- [ ] `internal/component/bgp/message/rfc7606.go` - `validateAttributeFlags`
- [ ] `internal/component/bgp/wireu/split.go` - `buildCombinedUpdates`
- [ ] `internal/component/bgp/rib/store.go` - `releaseRoute`
- [ ] `internal/component/bgp/plugins/rib/rib.go` - `insertPoolNLRIs`, `removePoolNLRIs`
- [ ] `internal/component/bgp/plugins/rib/rib_recovery.go` - `recoveryRoutes`, `recoverySentNLRI`
- [ ] `internal/component/bgp/plugins/rib/rib_replay.go` - `resendRoutesWithCursor`
- [ ] `internal/component/bgp/plugins/rib/pool/srv6sid.go` - `ExtractSRv6SIDFull`
- [ ] `internal/component/bgp/plugins/rr/rr.go` - `updateRouteWithMeta`, `isConnectionError`
- [ ] `internal/component/bgp/plugins/bmp/bmp_events.go` - `handleSenderUpdate`
- [ ] `internal/component/bgp/plugins/filter_community/blackhole.go` - `blackholeReadvertiseRefused`
- [ ] `internal/component/bgp/server/codec.go` - `decodeMPNLRIs`
- [ ] `internal/component/bgp/config/routeattr.go` - `parseRawAttribute`
- [ ] `internal/component/bgp/cli/decode_human.go` - `formatAttributesHuman`
- [ ] `internal/plugins/mrt/component.go` - `onPeerEstablished`, `onPeerClosed`

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A received UPDATE or OPEN on a peer's TCP session; an operator command
(`request peer`, `show bgp rib`, `ze bgp decode`); a config load or reload.

### Transformation Path
1. Session read, header and attribute validation (`message`, RFC 7606).
2. Per-family NLRI split (`nlrisplit`), announce and withdraw framings.
3. Reactor notify to plugins over the structured rail and the JSON rail.
4. RIB store, best path, route server relay, egress filters, UPDATE build.

### Boundaries Crossed
Wire to reactor; reactor to plugins (in-process or external, so a direct Go call
is not a substitute for the rail); plugin to reactor command dispatch.

### Integration Points
`nlrisplit.Get`/`SplitWithdrawn`, the filter registry, `rpc.StructuredEvent`,
the MRT and BMP subscribers.

### Architectural Verification
- [ ] No fix adds a second copy of a fact another package owns (S4)
- [ ] Every wire fix keeps buffer-first encoding

## Risks & Assumptions

### Assumptions
- Each row still describes HEAD until its red test proves it.

### Risks
- The route server rows (AC-5) share code with `spec-bgp-rs-replacement-on-withdrawal`; coordinate before editing `forward_rs.go`.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UPDATE with a labelled MP_UNREACH, Compatibility 0x800000 | → | `SplitWithdrawn` on every withdrawn walk | new `.ci` under `test/plugin/` asserting the withdrawal removes the route |
| Peer config with no `family` block | → | `buildOpen`, End-of-RIB | `.ci` asserting End-of-RIB for IPv4 unicast |
| Static route with raw AS_PATH `attribute` | → | `parseRawAttribute` | `.ci` asserting one AS_PATH in the sent UPDATE (hex) |

## Acceptance Criteria

Each AC is met when its test is red against HEAD before the fix and green after.

| AC | Rows | Observable |
|----|------|------------|
| AC-1 | flowspec keyword rename x3 | Flowspec config and update-text tests use `destination-ipv4`/`source-ipv4`; the old spelling is refused with a message naming the new keyword; `go test` of both packages green |
| AC-2 | hold/send-hold timer callbacks | Timer callback reaching `Session.Conn` does not deadlock on `Session.mu`; `TestRFC9687Event29ReleasesTheLivePeersRIB` completes under `-race` |
| AC-3 | teardown vs notify | `request peer teardown` and `notify` apply one Shutdown Communication contract (length and UTF-8 handling); a test sends the same 129-octet text through both and gets the same answer |
| AC-4 | `getPluginFamilies` | Families offered in OPEN are those configured for the peer plus documented auto-advertised ones, decided per peer; `flowspec-open-capability.ci` stays green and a peer with no flowspec config offers none |
| AC-5 | RS convergence, RS withdrawal replacement, flowspec-redirect hang, relay suppression x5 | The route server relay is deterministic: `modify-oversize-suppress.ci` and `forward-overflow-two-tier.ci` pass 200 consecutive runs; the 12 race/count-20 reactor failures are each fixed or attributed to a named owner spec |
| AC-6 | `resendRoutesWithCursor` | `clear bgp rib out` reports `resent` only for routes the writer accepted; a test with a refused `adjOutExpected` NLRI counts it as not resent |
| AC-7 | `Flags()` Partial bit | RFC 4271 Section 5: "it MUST NOT be set back to 0 by the current AS." A relayed known optional transitive attribute received with Partial set is sent with Partial set (hex assertion) |
| AC-8 | `validateAttributeFlags` | RFC 7606 Section 3: "If the value of either the Optional or Transitive bits in the Attribute Flags is in conflict with their specified values, then the attribute MUST be treated as malformed and the "treat-as-withdraw" approach used, unless the specification for the attribute mandates different handling for incorrect Attribute Flags." Every known attribute code is checked, derived from the attribute registry, not a hand list |
| AC-9 | `handleOpenSent` and siblings | RFC 4271 Section 8.2.2: "Any Start event (Events 1, 3-7) is ignored in the Established state." The same holds in OpenSent and OpenConfirm; a unit drives ManualStart in each state and asserts no state change and no counter change |
| AC-10 | `peerOnLink`, `refreshPeerLinkScopes` | RFC 2545 Section 3: "The link-local address shall be included in the Next Hop field if and only if the BGP speaker shares a common subnet with the entity identified by the global IPv6 address carried in the Network Address of Next Hop field and the peer the route is being advertised to." A relayed link-local next hop is kept only for a peer on the receiving interface, and a route already sent is re-advertised when the shared-subnet condition goes away |
| AC-11 | GoBGP SR Policy API | `bgp-sr-policy-gobgp` passes; the GoBGP change is kept as a diff against the upstream release applied at image build (`ai/META-DECISIONS.md`), or ze's scenario avoids the broken API with the reason recorded |
| AC-12 | impossible fixtures x5, rfc7752 negative test | Each fixture encodes a state the RFC allows (RFC 9012 Section 13 one Tunnel Egress Endpoint, RFC 9552 Node NLRI length, registered family names, decoders registered, next hop not the peer's own address); the RFC 7752 negative test asserts the length-sum reason, not any error |
| AC-13 | `appendFilterResultText` | The text event spells an MP NLRI that did not split (documented in `text-format.md`) and logs it; a unit asserts the text line |
| AC-14 | `stopBFDClient` | BFD Down in Established announces the down and the subscriber goroutine exits; a unit with the race drives it and checks no goroutine leak |
| AC-15 | `buildCombinedUpdates` | RFC 4271 Section 4.3: "An UPDATE message SHOULD NOT include the same address prefix in the WITHDRAWN ROUTES and Network Layer Reachability Information fields. However, a BGP speaker MUST be able to process UPDATE messages in this form." A received UPDATE withdrawing and announcing one prefix is relayed as an announcement only, for every family |
| AC-16 | `blackholeReadvertiseRefused` | RFC 7999 gate reads the attributes ze will send (egress tag, policy add, filter update), matching the command rail; a `.ci` adds 65535:666 by policy toward an unagreed eBGP peer and asserts no send |
| AC-17 | AIGP receive | RFC 7311 Section 3.2: "If a BGP path attribute is received that has the AIGP attribute codepoint but also has the transitive bit set, the attribute MUST be considered to be a malformed AIGP attribute and MUST be discarded as specified in this section." A received 0xC0 AIGP is discarded (hex in, no AIGP out) |
| AC-18 | reactor shutdown | No socket producer opens a new session after `Reactor.stop` starts; a unit races an inbound accept and a reconnect against stop |
| AC-19 | labelled withdrawal x5, `recoverySentNLRI` comment | RFC 8277 Section 2.4: "Upon transmission, the Compatibility field SHOULD be set to 0x800000." Every withdrawn-bytes walk (`reconcileReceivedNLRIs`, `forEachPrefixEntry`, `decodeMPNLRIs`, `nlritype.Retain`, `DecodeNLRIHex`) uses the withdrawal framing; the comment quotes the RFC byte-exact |
| AC-20 | flowspec redirect draft | `rfc/short/` summary for the draft exists with requirement rows bound to the existing tests (`./le rfc check` green) |
| AC-21 | `count offered` | A withdrawal of prefixes the family never held does not lower the count; a unit sends announce N, withdraw M unknown, announce 1 and asserts the limit trips |
| AC-22 | maximum-paths reload listener | `bgp-rib-maximum-paths-reload.ci` passes alone on a clean HEAD build; root cause named |
| AC-23 | `RUnlock` under recover, FSM callback panic, `releaseRoute` refcount | A recovered panic leaves no lock held (defer unlock); an FSM callback panic is recovered without "Unlock of unlocked RWMutex"; route release decrements under the store lock. One unit each, under `-race` |
| AC-24 | MRT EOR order, MRT callback type, capture timestamp | Initial-sync End-of-RIB follows the routes it brackets; MRT records state changes from the reactor's peer type; pcap timestamps keep sub-second precision |
| AC-25 | BMP dedup memo | A later UPDATE that contradicts an earlier one is sent even when its body matches an older one; a unit sends A, B, A and asserts three Route Monitoring messages |
| AC-26 | hold-down test pipe | `TestHoldDownRefusesInboundAndEndsOnStop` arms its deadline before the pipe can close; passes count=50 in the full package run |
| AC-27 | rr log level, redistribute consumer log | "no established peers" logs below ERROR; the consumer logs through the configured `ze.log` logger and a `warning` response is reported |
| AC-28 | FRR Prefix-SID | RFC 8669 Section 3.1: "It MUST be ignored when received for other BGP AFI/SAFI combinations." Ze's behaviour off labeled unicast is per the RFC and FRR's reset is recorded as a documented interop difference in `docs/architecture/testing/interop.md` |
| AC-29 | dynamic group listener | The listener binds only after its plugins are started, or the session-up event is replayed to a late subscriber; `attach-process-dynamic-group.ci` asserts the event |
| AC-30 | `buildOpen` 4096 | RFC 8654 Section 6: "This document changes the latter number to 65,535 for all messages except for OPEN and KEEPALIVE messages." An OPEN over 4096 octets is refused before send with an operator error naming the capability set |
| AC-31 | `ExtractSRv6SIDFull` | RFC 9252 Section 5: "The SRv6 Service SID is encoded as part of the SRv6 L3 Service TLV." A VPN route takes its SID from the L3 Service TLV only |
| AC-32 | `recoveryRoutes` | A source down withdraws from destinations not attached to bgp-rib, or logs that it cannot; a `.ci` asserts the WITHDRAW |
| AC-33 | JSON rail SAFI 4, unsplit families | The JSON rail binds and withdraws labels as the structured rail does; a family with no splitter is refused at negotiation or warned, never silently dropped |
| AC-34 | GTSM on macOS, TCP MD5 on FreeBSD | RFC 5082 Section 3: "The TTL field in all IP packets used for transmission of messages associated with GTSM-enabled protocol sessions MUST be set to 255." Where the platform cannot apply TTL, min-TTL or the MD5 key, config verify refuses with the reason |
| AC-35 | no `family` block | A peer without a family block sends End-of-RIB for the family it negotiates |
| AC-36 | `ze bgp decode` human, `show bgp rib best` VPN | Human decode prints every attribute (unknown codes as code plus hex); a VPN route renders as RD plus prefix |
| AC-37 | `AnnounceNLRIBatch` flowspec | A FlowSpec batch with no next hop is announced (RFC 8955 zero-length next hop); a `.ci` asserts the UPDATE |
| AC-38 | relay pool dry | A peer-up replay that meets an empty read-buffer pool waits and retries; no route is dropped; a unit with a pool of 1 replays 100 routes |
| AC-39 | raw AS_PATH | RFC 4271 Section 5: "The same attribute (attribute with the same type) cannot appear more than once within the Path Attributes field of a particular UPDATE message." A raw AS_PATH replaces the typed one or config verify refuses both |
| AC-40 | RFC8654-6-1 receive limit | The UPDATE receive limit follows the RFC 8654 Section 6 rule on an asymmetric session (one side advertises Extended Message); a reactor test drives both asymmetries |
| AC-41 | weak RFC tags x5 (RFC2545-3-1, RFC2545-3-3, RFC4271-5.1.2-3, RFC9830-4.2.1, RFC9252-7) | Each tag is backed by a test that drives the behavior its claim names (invalid IPv6 globals, shared subnet, empty AS_PATH, SR Policy receive validation, second Service TLV), or the tag is narrowed to what the body proves |
| AC-42 | BGP functional reds x5 (`bgp-gtsm-reject.ci`, `relay-withdraw-nexthop-self.ci`, `bgp-nexthop-self-local-auto-frr`, draft `rib-rail-two-octet-source.ci`, bgp-rib relay) | Each runs green with its cause named in the commit, or the product defect behind it is fixed |
| AC-43 | received LARGE_COMMUNITY, normal selected-route egress, cease subcode 6 page | A received route shows its LARGE_COMMUNITY in `rib show`; a selected Loc-RIB route reaches peers without RS or RR; the interface-event page states only what the code performs |
| AC-44 | discarded values x4 (`announce ... for`, `SendRoutes`, `DecodeNLRIHex` rest, LS Node Flag `T`/`A`) | The lifetime is applied, a `Commit` error reaches the caller with a true count, trailing NLRI bytes refuse, and the JSON key follows RFC 9552 Table 7 |
| AC-45 | `peerSettingsFromOperationConfig` | An API add-peer built from the reload config carries `Name`, as `PeersFromTree` sets it |
| AC-58 | mvpn unparsed route types | RFC 6514 Section 4.4: a Leaf A-D route (type 4) carries no Route Distinguisher, so `parseBody` reads an RD only for the route types whose layout holds one, and `String` names none for type 4 |
| AC-46 | `bufMux` budget | An idle 64K pool releases blocks so the 4K pool can grow; a session read under that load is not ended |
| AC-47 | MUP ext-community text | `ParseSingleExtCommunity` parses every administrator form and sub-type the MUP draft defines, and the comment says so |
| AC-48 | RFC 9552 MUST rows x5 | Each of the five rows carries a test or a recorded exclusion; `TestRealTreeIsGreen` passes |
| AC-49 | AS_PATH width x2 (`as-path-prepend`, `TranscodeASPath`) | RFC 6793 Section 4.2.2: a prepend on a two-octet session uses AS_TRANS and AS4_PATH; a malformed AS_PATH never consumes NLRI bytes (hex tests) |
| AC-50 | MRT TABLE_DUMP_V2 SAFI | RFC 6396 Section 4.3: non-unicast SAFIs use RIB_GENERIC or are refused; a test reads the written subtype |
| AC-51 | state reason text | The `<state-event>` reason is one token or the BNF and its three parsers accept the multi-word form; a test feeds `connection lost` |
| AC-52 | reactor races x2 (`PeerSettings` readers, `locRIB`/`sendCtxID`) | Every cross-goroutine read uses the accessor or the writer's lock; the race tests pass under `-race -count=20` |
| AC-53 | population gaps x5 (community Add buffer, forward rails, listener vs dynamic group, peer/group names, prefix-list ge/le) | Each check covers the member the row names, with a test that feeds that member |
| AC-54 | plugin `.ci` reds x2 | `bgp-rs-fastpath.ci` and `rpki-decorator-merge.ci` pass in an export of HEAD |
| AC-55 | initial-sync hold | A peer granting `send [ update ]` to a process that never reports does not hold its announces for `apiSyncTimeout` |
| AC-56 | prefix-list multi-entry test | The multi-entry test is fed the shape `ParseBGPSubtree` produces from config |
| AC-57 | ROUTE-REFRESH subtype | RFC 7313 Section 5: a ROUTE-REFRESH with an unknown Message Subtype "MUST be ignored"; a test sends subtype 3 and sees no reply and no error |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | AC |
|------|------|----|
| one red-first unit per AC above, named after the behaviour | the package of the producer named in the row | AC-1 to AC-39 |

### Functional Tests

| Test | AC |
|------|----|
| new `.ci` per wiring row, plus the named existing `.ci` files | AC-5, AC-16, AC-19, AC-22, AC-29, AC-32, AC-35, AC-37, AC-39 |

### Interop Tests (Scope: protocol)

| Scenario | AC |
|----------|----|
| `bgp-sr-policy-gobgp` | AC-11 |
| `bgp-prefix-sid-malformed-frr` | AC-28 |

## Files to Modify
- The producer files listed under Current Behavior, and the fixtures named in each owned row.

### Integration Checklist
- [ ] Each fix reaches the user through the real entry point (wire, command, config)

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/architecture/api/text-format.md` (AC-13)
- [ ] `docs/architecture/testing/interop.md` (AC-28)

## Implementation Steps
1. For each AC, read the producer at HEAD; if fixed, move its rows to "Rows found already fixed".
2. Write the red test, run it scoped, record the red.
3. Fix, run green, update the page the change makes wrong.
4. Close each row's journal cell with the fixing commit.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `bulk-rename-corruption` | 2026-09-13 | `flowspec_legacy_test.go` |
| `bulk-rename-corruption` | 2026-09-06 | flowspec `destination` keyword |
| `bulk-rename-corruption` | 2026-09-04 | `plugins/cmd/update` |
| `callback-reenters-held-lock` | 2026-10-22 | BGP hold/send-hold timer callbacks |
| `conflicting-feature-contracts` | 2026-10-21 | `request peer <sel> teardown` |
| `conflicting-feature-contracts` | 2026-10-04 | `Peer.getPluginFamilies` |
| `conflicting-feature-contracts` | 2026-10-22 | Reactor RS convergence tests |
| `counter-counts-the-wrong-packets` | 2026-10-13 | `resendRoutesWithCursor` |
| `derived-flags-overwrite-what-the-wire-carried` | 2026-09-20 | `Flags()` |
| `derived-flags-overwrite-what-the-wire-carried` | 2026-09-20 | `validateAttributeFlags` |
| `error-answered-with-another-errors-code` | 2026-10-05 | `handleOpenSent` |
| `escape-hatch-scoped-wider-than-its-justification` | 2026-10-04 | `peerOnLink` |
| `field-carries-two-meanings` | 2026-10-20 | GoBGP `pkg/apiutil/attribute.go` |
| `fixture-encodes-an-impossible-state` | 2026-10-05 | `rfc9012_tunnel_encap_carry_test.go` |
| `fixture-encodes-an-impossible-state` | 2026-09-06 | `test/parse/redistribute-family-filter.ci` |
| `fixture-encodes-an-impossible-state` | 2026-08-28 | reactor receive path, RFC 9552 |
| `fixture-encodes-an-impossible-state` | 2026-10-14 | reactor tests, mpls-vpn decoder |
| `fixture-encodes-an-impossible-state` | 2026-09-06 | `originatedNextHopIsPeerOwn` |
| `format-cannot-hold-a-state-the-tree-holds` | 2026-10-21 | `appendFilterResultText` |
| `goroutine-waits-on-its-own-exit` | 2026-10-13 | `Peer.stopBFDClient` |
| `guard-added-to-one-half-of-a-pair` | 2026-10-21 | `buildCombinedUpdates` |
| `guard-added-to-one-half-of-a-pair` | 2026-10-21 | `blackholeReadvertiseRefused` |
| `guard-added-to-one-half-of-a-pair` | 2026-10-06 | `TestRouteServerWithdrawalKeepsTheReplacementRoute` |
| `guard-added-to-one-half-of-a-pair` | 2026-09-05 | AIGP receive, `rfc7606.go` |
| `guard-added-to-one-half-of-a-pair` | 2026-08-11 | bgp reactor shutdown |
| `guard-added-to-one-half-of-a-pair` | 2026-10-03 | `reconcileReceivedNLRIs` |
| `guard-added-to-one-half-of-a-pair` | 2026-10-03 | `forEachPrefixEntry` |
| `guard-added-to-one-half-of-a-pair` | 2026-10-03 | `decodeMPNLRIs` |
| `guard-added-to-one-half-of-a-pair` | 2026-10-03 | `nlritype.Retain` |
| `guard-added-to-one-half-of-a-pair` | 2026-10-03 | `DecodeNLRIHex` |
| `implemented-draft-not-enrolled` | 2026-10-21 | `draft-simpson-idr-flowspec-redirect-02` |
| `invariant-enforced-by-an-absent-call-site` | 2026-08-23 | `count offered` |
| `invariant-enforced-by-an-absent-call-site` | 2026-08-23 | RFC 2545 link-local next hop |
| `listener-refuses-the-peer-its-config-names` | 2026-10-17 | `bgp-rib-maximum-paths-reload.ci` |
| `lock-released-by-hand-under-a-recover` | 2026-10-13 | `reactorForwardRSSection` |
| `marker-overtakes-the-queue-it-brackets` | 2026-10-13 | `TestMRTCollisionWinnerReservation` |
| `memo-suppresses-a-change-it-cannot-see` | 2026-08-30 | bmp `handleSenderUpdate` dedup |
| `mtime-granularity-stamp` | 2026-09-05 | raw capture timestamp |
| `negative-test-asserts-the-verdict-not-the-reason` | 2026-10-05 | `rfc7752_length_sums_test.go` |
| `net-pipe-deadlock` | 2026-08-15 | `TestHoldDownRefusesInboundAndEndsOnStop` |
| `normal-state-logged-at-error` | 2026-10-22 | `updateRouteWithMeta` |
| `normal-state-logged-at-error` | 2026-10-23 | adj-rib-in `applyDecision` WARN for a verdict on a withdrawn pending route |
| `paraphrase-becomes-the-authority` | 2026-10-13 | `recoverySentNLRI` |
| `queued-request-waits-for-an-unrelated-trigger` | 2026-10-21 | `flowspec-redirect-check-refused.ci` |
| `refcount-released-outside-the-lock` | 2026-08-18 | rib route store `releaseRoute` |
| `requirement-applied-outside-the-case-it-names` | 2026-10-20 | `bgp-prefix-sid-malformed-frr` |
| `session-up-before-its-subscriber` | 2026-08-15 | dynamic group listener |
| `silent-fall-through` | 2026-10-14 | `Session.buildOpen` |
| `silent-fall-through` | 2026-10-14 | `pool.ExtractSRv6SIDFull` |
| `silent-fall-through` | 2026-10-10 | `recoveryRoutes` |
| `silent-fall-through` | 2026-10-04 | `insertPoolNLRIs` SAFI 4 |
| `bound-too-small-for-its-own-burst` | 2026-10-21 | `bufMuxStd`/`bufMuxExt` shared byte budget |
| `comment-describes-superseded-behaviour` | 2026-10-21 | `ParseSingleExtCommunity` `mup` case |
| `concurrent-rfc-gate-stale` | 2026-08-25 | `rfc/short/rfc9552.md` five MUST rows with no test |
| `declared-format-contradicts-payload` | 2026-09-06 | `as-path-prepend` four-octet segment on a two-octet path |
| `declared-format-contradicts-payload` | 2026-10-05 | `TranscodeASPath` malformed AS_PATH consumes NLRI |
| `declared-format-contradicts-payload` | 2026-10-07 | MRT `writeTableDumpV2` ignores SAFI |
| `declared-format-contradicts-payload` | 2026-10-14 | `<state-event>` reason one token vs `connection lost` |
| `false-synchronization-claim` | 2026-08-23 | readers of mutable `PeerSettings` fields |
| `false-synchronization-claim` | 2026-10-05 | RIB `locRIB` and session `sendCtxID` races |
| `gate-excludes-part-of-its-population` | 2026-08-10 | `genericCommunityHandler` Add `Buf` unchecked |
| `gate-excludes-part-of-its-population` | 2026-08-11 | forward rails `overflowPending` misses the pool channel |
| `gate-excludes-part-of-its-population` | 2026-08-12 | per-peer listener swallows a dynamic group's inbound |
| `gate-excludes-part-of-its-population` | 2026-08-13 | peer and group name namespaces can collide |
| `gate-excludes-part-of-its-population` | 2026-08-18 | prefix-list `ge`/`le` missing combination |
| `gate-red-where-nothing-blocks-on-it` | 2026-10-10 | `bgp-rs-fastpath.ci`, `rpki-decorator-merge.ci` red in an export |
| `grace-bound-hides-a-missing-signal` | 2026-08-15 | initial-sync hold waits for a ready only bgp-rib sends |
| `green-that-could-not-have-been-red` | 2026-08-23 | `parsePrefixListEntries` multi-entry test feeds a shape no producer emits |
| `green-that-could-not-have-been-red` | 2026-08-31 | `handleRouteRefresh` unknown subtype not ignored |
| `silent-fall-through` | 2026-09-27 | GTSM socket setup, macOS |
| `silent-fall-through` | 2026-09-01 | `setTCPMD5Sig` FreeBSD |
| `silent-fall-through` | 2026-08-16 | peer with no `family` block |
| `silent-fall-through` | 2026-08-19 | `ze bgp decode` human |
| `silent-fall-through` | 2026-09-06 | `AnnounceNLRIBatch` |
| `silent-fall-through` | 2026-10-14 | `show bgp rib best` VPN |
| `suppression-decision-not-deterministic` | 2026-08-14 | plugin relay |
| `suppression-decision-not-deterministic` | 2026-08-15 | plugin relay |
| `suppression-decision-not-deterministic` | 2026-08-21 | plugin relay |
| `suppression-decision-not-deterministic` | 2026-09-19 | plugin relay |
| `suppression-decision-not-deterministic` | 2026-09-24 | `forward-overflow-two-tier.ci` |
| `transient-failure-treated-as-fatal` | 2026-08-15 | bgp-rs peer-up replay |
| `unguarded-goroutine-kills-the-process` | 2026-10-01 | bgp fsm callback panic |
| `wire-carries-both-arms-of-an-either-or` | 2026-10-18 | `parseRawAttribute` |
| `green-that-could-not-have-been-red` | 2026-10-05 | RFC8654-6-1 asymmetric receive limit |
| `published-value-drifts-from-the-behavior-it-describes` | 2026-09-21 | `lsNodeFlagBits.ToJSON` bit 6 `T` not `A` |
| `reference-checked-claim-unchecked` | 2026-10-06 | RFC2545-3-1 `nextHopGlobalOctets` |
| `reference-checked-claim-unchecked` | 2026-10-06 | RFC2545-3-3 `egressNextHopGlobalHalf` |
| `reference-checked-claim-unchecked` | 2026-10-06 | RFC4271-5.1.2-3 `ASPathEdit.recordPrepend` |
| `reference-checked-claim-unchecked` | 2026-10-13 | RFC9830-4.2.1-2/-6 `rfc9830_test.go` |
| `reference-checked-claim-unchecked` | 2026-10-13 | RFC9252-7-1/-2 `TestRFC9252FirstServiceTLVWins` |
| `test-against-broken-path` | 2026-08-10 | `test/plugin/bgp-gtsm-reject.ci` times out |
| `test-against-broken-path` | 2026-08-15 | `test/plugin/relay-withdraw-nexthop-self.ci` |
| `test-against-broken-path` | 2026-09-09 | `bgp-rib` as a relay |
| `test-against-broken-path` | 2026-10-06 | `bgp-nexthop-self-local-auto-frr` AIGP |
| `test-against-broken-path` | 2026-10-08 | draft `rib-rail-two-octet-source.ci` |
| `unwired-feature` | 2026-08-12 | received LARGE_COMMUNITY in `rib show` |
| `unwired-feature` | 2026-08-17 | normal selected-route egress |
| `unwired-feature` | 2026-09-07 | `interface-event-reactions.md` cease subcode 6 paragraph |
| `validated-value-discarded-by-its-caller` | 2026-09-03 | `announceAndTrack` lifetime |
| `validated-value-discarded-by-its-caller` | 2026-09-05 | `SendRoutes` discards `Commit` error |
| `validated-value-discarded-by-its-caller` | 2026-09-20 | `DecodeNLRIHex` discards `rest` |
| `zero-value-as-valid-answer` | 2026-10-10 | `peerSettingsFromOperationConfig` sets no `Name` |
| `zero-value-as-valid-answer` | 2026-09-01 | mvpn `parseBody` reads a Leaf A-D Route Key as an RD (zero-RD face fixed by `rdPresent`) |

## Rows found already fixed

| Class file | Date | Surface | Evidence |
|------------|------|---------|----------|
| none in this group at the 2026-10-23 sweep | - | - | - |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (red recorded per AC)
- [ ] Tests PASS
- [ ] Scoped runs: `./le job run go test ./internal/component/bgp/...`, `./le test functional plugin`
