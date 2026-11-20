# Spec: BGP one-off defects from the journal

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P1 |
| Release reason | Each row is incorrect shipped BGP behaviour (wire, RIB, FSM, plugin relay) or a test that cannot prove it; none is an absent feature |
| Depends | - |
| Phase | AC-7 stored Partial-bit repair |
| Handoff | verify |
| Updated | 2026-11-20 |

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

Amended 2026-11-15 from the closure of `spec-ospfv3-views-every-address-family`, which
made `command.PluginShapesFromDecls` (`internal/component/command/answer_shape.go`) the
one conversion from a plugin's command declarations to its answer shapes and left the
BGP area untouched by owner constraint: `declareShapes`
(`internal/component/bgp/plugins/adj_rib_in/rib_shape_test.go`) still carries its own
copy of that loop (AC-63).

The RFC 4271 continuation begins with AC-7's reached stored-route defect.
Accepted optional transitive attributes lose their received Partial bit when
individually pooled values are reconstructed for ordinary selected output and
recovery. The AC-7 continuation below specifies this repair and its tests.
The other acceptance criteria remain recorded and open; starting this phase
does not assert that they have been implemented or independently re-audited.

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

Deployment's 2026-11-13 default reactor race/count-20 attempt exposed a recovery
packing recurrence owned by AC-5. The existing
`TestRecoveryPackLiveDepartureOneUpdatePerFit`
(`internal/component/bgp/reactor/forward_bucket_recovery_test.go`) failed six
times: all 300 elected prefixes arrived, but in 18, 21, 16, 10, 13 and 11 UPDATEs
where the unchanged assertion expects one. This is a packing failure, with no
observed route loss. The prior 2026-10-25 chunk-sizing fix remains historical
evidence, not proof that this run passed.

The command was `go test -v -race -count=20 -timeout=40m -tags "$tags" ./internal/component/bgp/reactor/...`
with CGO enabled and the patched native toolchain. The canonical feature-tag
recipe supplies `ze_core` and the declared `ze_` tags from `feature-gates.txt`.
The full attempt failed; the global timeout interrupted its seventh iteration.

Current source: `recovery_owed.go` `round` sizes chunks through
`recoveryChunkKeys` from the 500-key cap and `bufMuxStd.lendable`; `decide`
creates an admission per chunk. `relay_recovery.go` `queueRecovery` builds one
replacement at a time, and `admitRecovery` can shorten an admission when its
buffer pool runs dry. `forward_bucket.go` `recoveryBucketMerge` merges only
within that admission. These boundaries require investigation; the log has no
per-admission trace and does not prove why any particular count occurred.
Reproduce and attribute the failure without weakening the packing assertion.

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

## Existing Mechanisms (MANDATORY)

This spec builds no new mechanism: each AC repairs one producer inside a
mechanism that already exists. The ones the owned rows reach so far:

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|-------------------------------------------|--------------------|---------------------------------------------------------------------|
| Recovery admission and packing | `internal/component/bgp/reactor/recovery_owed.go` `recoveryWorker.decide`; `forward_bucket.go` `recoveryBucketMerge` | Creates one admission per lookup chunk and packs compatible updates within that admission | Extends: AC-5 investigates the packing recurrence through this path while preserving route delivery and the existing assertion |
| Journal row ownership | `plan/journal/README.md`, "Owned rows"; `UnownedOpenRows` (`internal/le/spec/journal/owner.go`) | Every `open:` row is listed in one spec's `## Owned journal rows` table, and the commit gate refuses an unowned one | Extends: each AC closes its row with a `fixed:` marker naming the commit and test, or moves it to "Rows found already fixed" |
| Operator warnings on the report bus | `report.RaiseWarning` and `report.ClearWarning`, called from the reactor (`Peer.raiseOpQueueFull`, `peer.go`; `clearPrefixHold` in `Peer.cleanup`, `peer_run.go`) | A lasting peer condition is raised under the peer's address and listed by `show warnings`; `Peer.cleanup` clears the prefix-hold warning unconditionally, a no-op on an unraised subject | Extends: the `op-queue-full` row adds the missing clear half (queue drained below its cap, and `Peer.cleanup`); no second warning store |
| The plugin process boundary | `ai/doors/system.md`, "A plugin can run in another process"; `ai/doors/bgp-plugins.md`, "Boundaries" | The reactor reaches plugins, and plugins the reactor, through the EventBus, DirectBridge or a text command, never a plain Go call | Does not apply as a change: every fix in a plugin (rib, rr, bmp, filter_community, mrt) stays on those rails, so it works when the plugin runs external |
| Individually pooled attributes | `storage/attrparse.go` `ParseAttributes`; `storage/bundle.go` `BundlePool.Intern` | Shares attribute values and the comparable 48-byte non-AS_PATH bundle; dedicated optional transitive fields currently discard received flags | Extends for AC-7: retain four per-route Partial facts without duplicating values or changing bundle identity |
| Stored reconstruction and replacement | `storage/familyrib.go` `RouteEntry.ToWireBytes`, `entriesEqual`; `storage/routeentry.go` `Clone` | Rebuilds stored attributes, compares pooled handles and explicitly copies retained entries | Extends for AC-7: reconstruction, equality and cloning must preserve the independent received Partial facts |
| Immutable selected capture | `rib_bestchange.go` `prepareAdvertise`; `rib_recovery.go` `recoveryCandidateWithLabels`; `rib_selected_dispatch.go` `selectedDispatch.publish` | Captures reconstructed attributes and projects them into the selected-route RPC value | Preserves: exercise this actual receive-to-selected boundary without replacing source authorization or bypassing the process protocol |

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
| AC-58 | MRT abbreviated MP_REACH_NLRI | RFC 6396 Section 4.3.4: a TABLE_DUMP_V2 RIB entry's MP_REACH_NLRI holds only Next Hop Address Length and Next Hop Address, its length adjusted (cut at `reconstructWireAttrs` or `appendOtherAttrsWire`); a test reads the written attribute bytes of an IPv6 route and carries the RFC6396-4.3.4-2 tags |
| AC-59 | adj-rib-in JSON ingest `handleReceived` (moved from `spec-typed-identity-keys`, 2026-11-14) | A VPN route received through the JSON event path is stored per NLRI under its RD and prefix, as the raw path stores it through `installComplexNLRIs`; a unit sends two VPN NLRIs in one section and reads both back. A test change it needs goes to the `ze-test-judge` |
| AC-60 | `countPeerUpBarrier` by process name (moved from `spec-typed-identity-keys`, 2026-11-14) | A process started under an alias of a registration that requires the peer-up barrier counts toward the barrier, resolved through `plugin.RegistryNames` as `declaresSessionReady` does; a unit starts an aliased process and asserts the count |
| AC-61 | `recordCreatedPeerLocked` name collision (moved from `spec-typed-identity-keys`, 2026-11-14) | `create bgp peer` whose name from `peerConfigNameFor` is already an operator's peer at another address is refused with what, why and the fix, and the operator's peer in the running tree is unchanged; a unit creates the collision and reads the tree |
| AC-62 | MRT TABLE_DUMP_V2 ADD-PATH Path Identifier (RFC8050-4.1-1, `partial`; moved from `spec-rfc-verdict-test-fix-pass`, 2026-11-14) | RFC 8050 Section 4.1: "This adds a field to the RIB Entries record to store the Path Identifier when used with the RIB_IPV4_UNICAST_ADDPATH, RIB_IPV4_MULTICAST_ADDPATH, RIB_IPV6_UNICAST_ADDPATH, and RIB_IPV6_MULTICAST_ADDPATH subtypes." Today each entry's Path Identifier is 0, because `writeTableDumpV2` builds `RIBEntry` without `PathID` and `RIBDumpVisitor.OnRoute` carries none. Target: `writeTableDumpV2` (`internal/plugins/mrt/dump.go`) writes each RIB entry's Path Identifier from the route's own identifier, carried through `RIBDumpVisitor.OnRoute` (`internal/component/plugin/registry/interfaces.go`), never a constant; a test dumps two paths of one prefix and reads two distinct Path Identifiers; a non-author re-judges the verdict |
| AC-63 | adj-rib-in `declareShapes` copy of the Stage 1 shape conversion (moved from `spec-ospfv3-views-every-address-family`, 2026-11-15) | `declareShapes` (`internal/component/bgp/plugins/adj_rib_in/rib_shape_test.go`) registers `commandDecls()` through `command.PluginShapesFromDecls`, as Stage 1's `registerPluginShapes` and the OSPF `declareOSPFShapes` do, and carries no loop of its own; the adj-rib-in shape tests stay green and assert what they asserted before |

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
| `goroutine-waits-on-its-own-exit` | 2026-11-20 | peer-up replay lost while the update-delay hold owns the initial update |
| `silent-fall-through` | 2026-10-31 | static-route send error logged at DEBUG, later routes abandoned |
| `reference-checked-claim-unchecked` | 2026-10-31 | new-v6.ci RFC2545-3-2 tag unprovable; proof owed in an attribute unit test |
| `lock-held-across-a-join` | 2026-10-27 | handleBFDEvent holds openMu across negotiation publication |
| `injected-route-never-announced` | 2026-10-30 | filter-modified route never reaches the egress peer |
| `injected-route-never-announced` | 2026-10-30 | route added by a config reload never sent |
| `test-reads-producer-state-unlocked` | 2026-10-30 | rpki cache-restart test races `cacheGroup.load` |
| `test-asserts-superseded-behaviour` | 2026-10-30 | reactor RS tests lack the fast-path receive grant |
| `assertion-reads-state-the-scheduler-owns` | 2026-10-27 | TestRFC4271DelayOpenBothSidesHold closed-pipe deadline |
| `clock-bypass` | 2026-10-27 | reactor_connection.go forward write deadline |
| `retained-entry-still-answers-lookups` | 2026-10-27 | api-peer-create-delete-rib |
| `package-red-not-reproduced` | 2026-10-27 | api-peer-save reload generation |
| `guard-keyed-on-a-field-its-consumer-ignores` | 2026-10-26 | `validateOpenPeerAS` reads the ASN4 capability with `asn4 disable` |
| `test-against-broken-path` | 2026-10-26 | `bgp-linklocal-only-multihop-withdraw-frr` route setup `File exists` |
| `test-against-broken-path` | 2026-10-26 | FRR oracle fixture `rfc2545-joint-subnet-frr`: zebra's root not in vty group frrvty |
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
| `error-path-discards-data-already-received` | 2026-10-24 | `fwdBatchHandler` AIGP receipt discard |
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
| `declared-format-contradicts-payload` | 2026-10-30 | MRT TABLE_DUMP_V2 RIB entry carries MP_REACH_NLRI unabbreviated |
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
| `unwired-feature` | 2026-10-26 | `buildWithdrawNLRI` left with no production caller |
| `session-never-establishes-with-the-peer` | 2026-10-26 | FRR answers FSM Error in OPENSENT, labeled-withdraw scenario |
| `validated-value-discarded-by-its-caller` | 2026-09-03 | `announceAndTrack` lifetime |
| `validated-value-discarded-by-its-caller` | 2026-09-05 | `SendRoutes` discards `Commit` error |
| `validated-value-discarded-by-its-caller` | 2026-09-20 | `DecodeNLRIHex` discards `rest` |
| `zero-value-as-valid-answer` | 2026-10-10 | `peerSettingsFromOperationConfig` sets no `Name` |
| `zero-value-as-valid-answer` | 2026-09-01 | mvpn `parseBody` reads a Leaf A-D Route Key as an RD (zero-RD face fixed by `rdPresent`) |
| `expected-message-never-sent` | 2026-10-31 | `test/plugin/rpki-aspa-*` time out waiting for Ze's EOR |
| `bound-too-small-for-its-own-burst` | 2026-11-13 | AC-5: recovery departure packing, all routes received but UPDATE count exceeds fit |
| `guard-added-to-one-half-of-a-pair` | 2026-11-09 | `op-queue-full` report warning raised, never cleared |
| `parked-work-returns-out-of-order` | 2026-11-13 | forward pool `takeOverflowReleased`: a hold lowered mid-scan releases a later parked live batch ahead of an earlier one |
| `declared-format-contradicts-payload` | 2026-10-20 | AC-59: adj-rib-in `handleReceived` keys a VPN NLRI without its RD, stores one NLRI per section |
| `gate-excludes-part-of-its-population` | 2026-09-02 | AC-60: `countPeerUpBarrier` asks the registry by process name, not registration name |
| `two-owners-share-one-name` | 2026-10-10 | AC-61: `recordCreatedPeerLocked` overwrites an operator's peer of the same name |
| `silent-fall-through` | 2026-11-14 | `fwdPool.safeBatchHandle` keeps the session after a recovered batch panic, leaving a hole in what the peer was sent |
| `wait-outlives-the-process-it-waits-on` | 2026-11-14 | replay fence stays up when its owner never reports (`Peer.lowerReplayFence`) |
| `definite-non-delivery-kept-as-unknown-outcome` | 2026-11-20 | bgp-rs keeps an `errAdjOutSession` refusal in `rs.unknownOutputs` for good, so every later settlement and `request quiesce` fails |

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

## AC-7 continuation: stored Partial preservation

### Disposition and diagnosis

| Field | Finding |
|-------|---------|
| Date and checked tree | 2026-11-17, `7214efd7aa` |
| Original AC-7 framing | REWORK for this reached path: repairing typed `Flags()` alone cannot repair value-only storage reconstruction |
| Current source defect | `ParseAttributes` individually pools AGGREGATOR, COMMUNITIES, EXTENDED_COMMUNITIES and LARGE_COMMUNITIES without flags; `ToWireBytes` writes each with Partial clear |
| Reached consequence | `handleReceivedStructured` stores the accepted attributes; `prepareAdvertise` and `recoveryCandidateWithLabels` reconstruct them; `selectedDispatch.publish` transports that reconstruction. Later forwarding and Session normalization do not recover the lost bit |
| Source repair | Retain a private four-bit per-route mask, copy it in `Clone`, compare it in `entriesEqual`, and restore the corresponding bit only when reconstructing that present pooled attribute |
| Rejected workaround | A final-writer blanket Partial bit would corrupt complete attributes; consulting the old cache would add a second attribute authority and fail independently retained reconstruction |
| Alternatives | Adding flags to the comparable bundle would split same-value sharing and change its 48-byte contract. Per-route metadata fits the existing small-field padding on the current 64-bit layout; actual compiler size evidence is still owed |

RFC 4271 Section 5 governs the baseline: "If a path with a recognized,
transitive optional attribute is accepted and passed along to other BGP peers
and the Partial bit in the Attribute Flags octet is set to 1 by some previous
AS, it MUST NOT be set back to 0 by the current AS."

The assessed path retains and readvertises the received attribute without an
operator replacement or removal. Explicit policies remain available. The
separate generic and community policy encoders require their own assessment
before whole AC-7 or RFC4271-5-4 closure; this storage repair does not silently
declare those paths correct or remove their features.

### Current research constraints

Follow-up source census found the same loss in automatic AS-width conversion,
LLGR community addition and Prefix-SID companion cleanup. These are additional
AC-7 implementation phases, not unrelated RFC features: each retains and
passes a received optional transitive attribute but writes a complete header.
The storage-only phase must not be presented as whole AC-7 completion.

| Additional default producer | Reached operation and required survivor |
|-----------------------------|----------------------------------------|
| `wireu/aspath_collapse.go` `AS4FamilyPlan.WriteTo` | OLD-source canonicalization keeps the surviving AGGREGATOR's Partial information while widening its value; RFC 6793's selected AGGREGATOR/AS4_AGGREGATOR owner and required discards still decide which information survives |
| `wireu/aspath_transcode.go` `TranscodeASPath` | Destination-width conversion retains the surviving AGGREGATOR's Partial bit, valid value, AS_TRANS/companion rules and unrelated NLRI/attributes |
| `wireu/aspath_slot.go` `recordAggregator` and `reactor/filter_delta_handlers.go` `genericAttrSetHandler` | The automatic generated replacement reaches the real modification handler with the same preservation obligation; no synthetic test handler or mod-count-only oracle |
| `plugins/gr/gr_egress.go` `LLGREgressFilter` and `plugins/filter_community/handler.go` `emitCommunity` | Automatic NO_EXPORT addition to an already received Partial COMMUNITY keeps that bit and the existing values; explicit community operations remain available |
| `reactor/forward_prefix_sid.go` `applyEgressPrefixSIDNextHop` and `prefixSIDNextHopHandler` | Automatic next-hop companion cleanup keeps the received Partial bit when useful TLVs survive; a legitimately suppressed whole attribute remains suppressed |

Zero-initialized storage metadata means no received Partial fact was retained;
it is not a universal wire rule for locally attached attributes. RFC 4271
Section 5 separately says that an optional transitive attribute attached by a
speaker other than the originator has Partial set. That attachment obligation
and existing explicit raw/header control are preserved. The source bytes of a
fresh replacement, rather than a stale previous route, decide retained flags.

- [ ] `docs/architecture/plugin/rib-storage-design.md`, received provenance and per-attribute storage sections
  → Decision: attribute values remain shared; received Partial is per-route metadata, separate from `Bundle` identity and source authorization.
- [ ] `storage/routeentry.go`, `storage/attrparse.go`, `storage/familyrib.go`
  → Constraint: update both the parser's populated constructor and explicit `Clone` copy; all five CIDR/opaque and ADD-PATH equality branches share `entriesEqual`.
- [ ] `rib_commands_community.go` `attachCommunity`
  → Constraint: appending a value to an existing attribute preserves its received Partial fact. A newly created attribute retains the current complete default; incoming replacement bytes establish fresh metadata.
- [ ] `rib_recovery.go`, `rib_bestchange.go`, `rib_selected_dispatch.go`
  → Constraint: selected and recovery output consume the reconstructed owned bytes. Message IDs and source receipts do not substitute original cache attributes or grant authorization.

### Phase acceptance and preservation

| Criterion | Required observable result | Permitted survivor and reason |
|-----------|----------------------------|------------------------------|
| AC-7-S1 | Each of the four dedicated optional transitive attributes retains its literal received Partial bit and value through parse, clone, original release and reconstruction | An independently stored complete attribute with identical values remains complete; shared values do not imply shared flags |
| AC-7-S2 | Mixed attributes retain their own independent Partial bits; flag-only replacements in CIDR and opaque storage, with and without ADD-PATH, change the reconstructed bytes | Repeating identical attributes remains a no-op; unknown attributes retain their existing full-header path and well-known/non-transitive Partial clearing remains correct |
| AC-7-S3 | A valid structured received IPv4 route is stored, elected and captured with its COMMUNITY Partial bit; actual selected dispatch projects the same attribute bytes and NLRI | A complete COMMUNITY control remains useful and complete; the passive test fixture does not claim a live Session grant, socket write or foreign acceptance |
| AC-7-S4 | Stored metadata survives supported in-place community attachment and retained ownership without a new attribute-value allocation scheme or larger bundle | Existing operator policies, source leases, provenance and process-boundary contracts remain supported |
| AC-7-S5 | Actual AS-width conversion and the ordinary generated-attribute rebuild preserve a valid retained AGGREGATOR's Partial flag and exact converted value | Retained complete twins, same-width/NEW fast paths, legal RFC 6793 selection/discard behavior and useful unrelated attributes/NLRI survive |
| AC-7-S6 | Conditional LLGR community addition and Prefix-SID companion cleanup preserve Partial on an existing surviving attribute | Useful retained values/TLVs survive; legitimate whole-attribute suppression and explicit operator operations remain supported |

### Implementation phases and validation

| Phase | Deliverable | Evidence owed by main |
|-------|-------------|-----------------------|
| AC-7 test-first | Two new regression files using existing storage and structured receive/selected-dispatch fixtures; no existing test expectation changes | Named semantic RED before production edits; the storage size diagnostic supplies the compiler's current layout |
| AC-7 source repair | `storage/routeentry.go`, `storage/attrparse.go`, `storage/familyrib.go`, owning documentation and the same regressions | Same named roots PASS; both changed storage/RIB packages, scoped lint and independent implementation review |
| AC-7 AS-width conversion | New wireu and reactor AGGREGATOR regressions, then the reached collapse/transcode/generated-header repairs and owning AS4/attribute documentation | Meaningful RED before each repair, exact flags/value GREEN, selected-owner and complete/same-width controls, scoped packages/lint and independent review |
| AC-7 conditional rewrites | New LLGR community and Prefix-SID cleanup regressions, then the existing handler repairs and owning documentation | Automatic caller-to-handler RED/GREEN with useful surviving values, complete controls and explicit-operation preservation; no raw Emit API ceiling |
| AC-7 proof integration | Independently judged requirement tags and matching native semantic records; rejudge every audit this change stales | Native current-cover results and accurate bounded audits in the coherent production commit; whole AC-7 wire proof remains owed until observed |

The first regressions are `TestKnownTransitivePartialSurvivesStorage` in
`storage/partial_storage_test.go` and
`TestReceivedPartialSurvivesSelectedCapture` in `partial_selected_test.go`.
The selected fixture uses a valid canonical four-octet AS_SEQUENCE containing
the source AS 65001, rather than claiming that an empty external AS_PATH was
accepted. It enters `handleReceivedStructured` through the existing fixture;
it must not construct a `StoredRoute` directly from the original attributes.

### Risks and assumptions for this phase

| ID | Risk or assumption | Basis / early signal | Validation or mitigation |
|----|--------------------|----------------------|--------------------------|
| A-7-1 | One mask byte fits the current route's padding without growth | Source alignment places the next uint64 at offset eight; this is not compiler evidence | Read actual `TestRouteEntrySizes` output before and after; preserve the existing 48-byte bundle check |
| R-7-1 | Shared handles hide a flag-only replacement | `entriesEqual` currently compares only handles | Exercise both replacement directions and all shared storage branches |
| R-7-2 | Clone or attachment loses or leaks per-route flags | Clone lists fields explicitly; attachment replaces a bundle | Hold complete and partial same-value entries simultaneously, release originals, append to the retained community and compare literal bytes |
| R-7-3 | A selected test bypasses the storage loss or invents source authority | A hand-built stored route can reuse the original attributes | Feed a real structured UPDATE and inspect owned capture plus dispatched RPC; keep passive provenance explicitly untrusted |

### Integration and documentation for this phase

| Surface | Disposition |
|---------|-------------|
| Attribute values and bundle pool | Existing pools and ownership, extended only with per-route flags |
| CLI, YANG, config, environment and public RPC | No new field or grammar; existing selected RPC already carries complete attribute wire bytes |
| Runtime dependency, refusal, diagnostic and metrics | No new resource, refusal or warning; no additional doctor check or counter is needed for retained wire metadata |
| Functional and interop | Existing fixtures remain; actual peer-visible AC-7 proof is required before whole wire/requirement closure and is not replaced by the selected transport test |
| Owning documentation | Update received-attribute reconstruction in `docs/architecture/plugin/rib-storage-design.md`, correct its obsolete 12-byte route statement, and replace the source limitation comments with the actual retained-bit contract |
| Discovery | Existing storage design source anchors and the two adjacent regression files identify the behavior; no second registry or report family |

## Critical Review Checklist

The current implementation round covers the AC-7 storage continuation and
its reached selected transport boundary. Other ACs remain open under their
existing scope; this checklist supplies no closure verdict for them.

| Lens | What to verify |
|------|----------------|
| Required useful output | Stored and selected output retain exact Partial/value bytes and the useful route, not merely an absence of an error |
| Independence and ownership | All four mask facts are independent across sharing, cloning, replacement and community attachment; no stale bit survives fresh incoming replacement |
| Baseline and extensions | Default complete controls, unknown/full-header attributes, forbidden Partial clearing, explicit policies and modern RFC behavior survive |
| Reachability and authority | The test reaches structured receive, storage and selected projection; no original-cache shortcut, fabricated grant or new public field |
| Bounds and layout | No new hot-path map, buffer or goroutine; bundle identity/size stays unchanged and actual route size evidence is recorded |
| Evidence and claims | Actual RED/GREEN, scoped package/lint results, independent test judgment, current native proof records and affected audits support only their observed boundary |

## Review Gate

### AC-7 implementation review runs

| Run | Scope | Independent reviewer | Result |
|-----|-------|----------------------|--------|
| Storage 1 | Three storage production files, owning storage documentation and the two new regression files; clone/release/equality, flag independence, useful selected projection and existing normalization | Independent `classic_daemon_publication_review` context | Source clean. Main observed semantic RED before repair, then all 13 storage leaves and both selected leaves PASS; actual layout stays 64/48 bytes. Complete changed-package, lint and proof integration remain owed |
