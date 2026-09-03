# Spec: fixit-locrib-peer-fields-contradict-rfc9069

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 1/1 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-09-03 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze publishes RFC 9069 (BMP Loc-RIB monitoring) as `Supported` with "None
outstanding" (`docs/features/rfc-status.md`, the RFC 9069 row) and enrols it as
"seven MUST-level requirements, all met" (`rfc/enrolled.txt`). Both statements
are false. Two rows of `rfc/short/rfc9069.md` state the OPPOSITE of the RFC, and
passing tests pin the non-conformant behavior:

| Row | Summary says | RFC 9069 says |
|-----|--------------|---------------|
| `RFC9069-x-3` `[MUST]` | "Peer Up for Loc-RIB has zero-length OPEN messages" | §5.2: "Sent OPEN Message: This is a fabricated BGP OPEN message. Capabilities MUST include the 4-octet ASN and all necessary capabilities to represent the Loc-RIB Route Monitoring messages." |
| `RFC9069-x-6` `[MUST]` | "Peer AS MUST be 0 for Loc-RIB (Peer Type 3)" | §5.1: "Peer Autonomous System (AS): Set to the primary router BGP autonomous system number (ASN)." |

Root cause is already recorded: `plan/deferrals/ad-hoc-2026-07-27-423eaa77.md`
row 14 predicted it. The rfc9069 summary was authored while
`rfc/full/rfc9069.txt` did not exist, so it was derived from Ze's code rather
than from the source, and `./le rfc check` then validated the summary against
itself.

**Goal:** make Ze's Loc-RIB monitoring conformant on both the sender and the
receiver side, correct every derived claim, and prove the result against a BMP
implementation that is not Ze.

### The six obligations

Each RFC sentence below was read in `rfc/full/rfc9069.txt`, and each producer
was read at the function named.

| # | RFC sentence (§) | Producer today | What is wrong |
|---|------------------|----------------|---------------|
| 1 | §5.1 "Peer Autonomous System (AS): Set to the primary router BGP autonomous system number (ASN)." | `locRIBPeerHeader` (`bmp_locrib.go`) | The struct literal sets `PeerType`, `Flags`, `PeerBGPID` and `TimestampSec` and no `PeerAS`, so every Loc-RIB header carries AS 0 |
| 2 | §5.2 as quoted above, plus §6.1.1 "Each emulated peer instance MUST send a Peer Up with the OPEN message indicating the address family capabilities." | `ensureLocRIBPeerUp` and `primeLocRIBPeerUp` (`bmp_locrib.go`) both call `writePeerUpLocked` (`sender.go`) with nil sent and nil received OPENs | Ze emits an OPEN-less Loc-RIB Peer Up. `TestPeerUpOnCacheMissNeverReachesTheCollector` (`peerup_openless_test.go`) already holds that a Peer Up without both OPENs is unparseable, because a collector locates the Information TLVs by walking past them. Ze's Loc-RIB Peer Up is therefore unreadable by a conformant collector |
| 3 | §5.2.1 "The default value of \"global\" MUST be used for the default Loc-RIB instance with a zero-filled distinguisher." | no Peer Up Information TLV is built for the Loc-RIB peer anywhere in `bmp_locrib.go` | Ruled below: Ze emits the TLV unconditionally with the value `global` |
| 4 | §5.3 "The Peer Down notification MUST use reason code 6." | `sendLocRIBPeerDown` (`bmp_locrib.go`) passes `PeerDownLocalNoNotify` | That constant is 2 (`msg.go`). Reason code 6 is not defined anywhere in Ze |
| 5 | §6.1.1 "A BMP receiver MUST process these capabilities to know which peer belongs to which address family." | `decodePeerUp` (`msg.go`) SKIPS OPEN extraction when `PeerType == PeerTypeLocRIB`; `bmpCompositeKey` (`bmp.go`) is router plus `peerAddressString`, which is `0.0.0.0` for every Loc-RIB peer | Two receiver defects. Ze cannot read a conformant sender's Loc-RIB Peer Up at all: it walks the fabricated OPEN bytes as Information TLVs. And every Loc-RIB instance of one router collapses onto one route-injection key, so two VRF instances overwrite each other |
| 6 | §5.1 "Peer BGP ID: ... otherwise, set to the global instance router-id." | `localRouterID` (`bmp_locrib.go`) walks `bp.openCache` and returns 0 when it is empty | A zero that reads as a valid answer (`ai/rules/principles.md`), in exactly the case §1.1 names: "VRF tables with no peers ... no preexisting BGP peers". The router-id is in config; the OPEN cache is the wrong source |

Obligation 5 is the receiver side, and it binds Ze: Ze runs a real BMP receiver
(`acceptLoop` and `DecodeMsg` in `bmp.go`).

### Two sites that are MET and merely unrowed

| Site | RFC sentence | Producer | What is owed |
|------|--------------|----------|--------------|
| §4.2 | "If locally sourced routes are communicated using BMP, they MUST be conveyed using the Loc-RIB Instance Peer Type." | `handleBestChange` (`bmp_locrib.go`) builds every message with `locRIBPeerHeader` | A new requirement id and both test polarities |
| §6.1.3 | "In case of any change that results in the alteration of behavior of an existing BMP session ... the session MUST be bounced with a Peer Down/Peer Up sequence." | `startSender` (`sender.go`) stops every session with a Termination and creates replacements | A new requirement id and both test polarities |

Neither is a defect. Both are obligations Ze meets with no row, which is
`ai/rules/rfc-compliance.md`'s "left unextracted" case.

### The RFC 8671 question, ruled

`rfc/extraction/rfc8671.json` is LANDED and signed (`signed-off: 2026-08-30`),
and its site `5.2:1` is excluded `binds-another-role`: RFC 8671 §5.2 "All
mandatory attributes, such as next hop, MUST be either zero or have an empty
length if they are unknown at the pre-policy phase completion." The producer
`peerHeaderFromEvent` (`bmp_events.go`) sets `PeerFlagO | PeerFlagL` in one
statement, so no message Ze emits carries O=1 with L=0 and the obligation has no
code path.

**Ruling: the published exclusion STANDS, and this spec does not resign
`rfc/extraction/rfc8671.json`.** RFC 7854 §5 makes the choice a MAY ("A BMP
speaker may send pre-policy routes, post-policy routes, or both"), and
`ai/rules/rfc-compliance.md` forbids an agent from picking a MAY. Implementing
pre-policy Adj-RIB-Out export is a separable FEATURE with no data source in Ze
today, not a defect this spec walked into, so `ai/rules/rule-precedence.md`
homes it in its own spec and its own owner question. It is named in Known
Limitations.

One observation on the artifact, recorded and not acted on: the `excluded-kind`
is `binds-another-role`, and the role Ze declines is a MODE of the sender role it
does play, not a different role. The reason text is accurate at the producer, so
the verdict holds and no resign is owed. Changing the label alone would cost a
`resign-reason` and a bumped `signed-off` date (`rfc/extraction/README.md`) for
no change in meaning.

### The interim ledger state, and why the correction leads

`docs/features/rfc-status.md` and `rfc/enrolled.txt` are corrected DOWNWARD in
Phase 1, before any of the six fixes lands. A public claim that Ze meets seven
MUSTs when it meets one of them costs more standing than one that says which are
open, and `ai/rules/rfc-compliance.md` makes a stale claim pointing away from
compliance VOID rather than citable. The interim state exists only between
Phase 1 and Phase 8 of this same spec: at closure every one of the six is
implemented and proven, and no row reads `{gap}`.

**This is not a scope reduction.** The code is already non-conformant; the edit
makes the record true about a state that exists. Every one of the six carries an
AC below, and none is classified `{gap}` or `{not-applicable}` at closure.

**Constraint discovered at the gate, and it fixes the ORDER:**
`checkCoverageRatchet` (`internal/le/rfc/check_ratchets.go`) is monotonic on
polarity coverage: a requirement id that has a positive test at HEAD cannot stop
having one. `RFC9069-x-3` and `RFC9069-x-6` have positive tests today, on the
assertions that pin the WRONG behavior. So Phase 1 corrects the requirement TEXT
and the ledger prose only, and leaves every `RFC requirement:` tag in place. Each
tag is re-pointed onto the corrected assertion in the phase that lands its fix,
under the SAME id, so coverage is never interrupted.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - the Design owner of `bmp.go`, `bmp_events.go`, `msg.go`, `sender.go`, `header.go`, `state.go` and `tlv.go`
  → Constraint: every one of those seven files declares this page in its `// Design:` header, so a behavior change in any of them carries the page edit in the same phase (`ai/rules/documentation.md`)
  → Decision: the BMP plugin lifecycle described there is not changed by this spec; only the Loc-RIB message CONTENT and the receiver's decode path change
- [ ] `docs/guide/bmp.md` - the operator page for BMP, and the Design owner of `bmp_locrib.go`'s sibling `peerup_openless_test.go`
  → Constraint: the page states "one Loc-RIB Peer Up per RIB instance with ... a Loc-RIB Peer Down on shutdown" and does not describe the OPEN, the AS, the table name TLV or the reason code. Each becomes a page edit in the phase that lands it
- [ ] `docs/architecture/testing/interop.md` - the suites, the scenario structure, the sidecar mechanism, and the vacuity traps
  → Constraint: a scenario directory is NAMED and carries no numeric prefix. `interoplab.Discover` matches the directory name exactly
  → Constraint: before an interop test may be claimed to validate a change, the change is reverted, the artifact the test drives is REBUILT, the test is confirmed RED, the fix restored, and the RED recorded

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9069.md` - the Design owner of `bmp_locrib.go`, and the artifact two of whose rows this spec corrects
  → Constraint: `RFC9069-x-3` and `RFC9069-x-6` are corrected UNDER THE SAME ID. `checkRetiredRequirements` (`internal/le/rfc/check_ratchets.go`) refuses an id that was in the baseline and is no longer live, so an id is never renumbered or removed; its TEXT may be corrected
  → Constraint: `extractSection` (`internal/le/rfc/summary.go`) anchors an id to the FIRST section of this RFC the checklist line cites, and to `x` when it cites none. A new row citing "Section 4.2" therefore yields `RFC9069-4.2-1`; the seven existing rows cite headings rather than sections, which is why they are all `x-N`
- [ ] `rfc/full/rfc9069.txt` - the authority, §4.2, §5.1, §5.2, §5.2.1, §5.3, §6.1.1 and §6.1.3
  → Constraint: §5.2 requires the received OPEN to be a "Repeat of the same sent OPEN message", so the two OPEN fields carry identical bytes, not two encodings
  → Constraint: §5.2 says "Only include capabilities if they will be used for Loc-RIB monitoring messages", so the fabricated OPEN's Multiprotocol set is exactly the families the dump carries and nothing more
  → Decision: §5.2.1's "If the TLV is included, then it MUST also be included in the Peer Down notification" makes the Peer Up TLV and the Peer Down TLV one decision, not two
- [ ] `rfc/full/rfc8671.txt` §5.2 - the excluded site, read to rule on it
  → Decision: the exclusion stands; see the ruling in Task
- [ ] `rfc/extraction/README.md` - the sign-off contract for a landed extraction artifact
  → Constraint: changing a signed artifact's exclusion count needs a `resign-reason` and a bumped `signed-off` date. This spec changes neither, because it changes no exclusion

### Rules
- [ ] `ai/rules/rfc-compliance.md`
  → Constraint: a test that pins non-conformant behavior is the violation with a green bar on top. Fix the code FIRST, correct the test after; never the other way round
  → Constraint: full compliance plus a tagged test is already Thomas's answer. Nothing in this spec is put to him except the RFC 7854 §5 MAY named in Known Limitations
- [ ] `ai/rules/interop-and-goal-validation.md`
  → Constraint: the Peer Up change is wire-visible, so a scenario against another implementation is owed and cannot be omitted
- [ ] `ai/rules/principles.md`
  → Constraint: a value that is silently wrong must not be reachable. `localRouterID` returning 0 on an empty OPEN cache is that failure, and the fix reads a source that is always present rather than adding a second fallback

**Key insights:** (minimal context to resume after compaction)
- The false claim is in FOUR places that all derive from one bad summary: `rfc/short/rfc9069.md` rows x-3 and x-6, `rfc/requirements/rfc9069.md` (generated from them), `rfc/enrolled.txt`, and the `docs/features/rfc-status.md` row. Correct the summary and regenerate; do not hand-edit the generated file.
- Two tests pin the wrong behavior: `TestLocRIBPeerHeader` asserts `ph.PeerAS != 0` is an error, and `TestHandleBestChangeEmitsPeerUpThenRM` asserts both OPENs are zero-length. Both assertions are corrected under the same requirement tags.
- `dumpFamilies` (`bmp_locrib.go`) is a hardcoded `[IPv4Unicast, IPv6Unicast]`, and its comment justifies the hardcoding with "there is no negotiated family set to derive this from" BECAUSE the OPENs are zero-length. Fixing obligation 2 removes that justification, and the fabricated OPEN's Multiprotocol capability set becomes the single declaration both the OPEN and the dump read (`ai/rules/principles.md`). The comment goes stale in the same edit (`ai/rules/stale-comments.md`).
- The BMP plugin already receives the whole `bgp` config subtree (`WantsConfig` names `configRootBGP`), and both values obligations 1 and 6 need live in it: `bgp { session { asn { local } } }` and `bgp { router-id }`, both `mandatory true` in `internal/component/bgp/yang/ze-bgp-conf.yang`. Neither needs a new config leaf.
- The existing `bmp-frr` interop scenario proves nothing about interop: its collector is Ze's own `runBMPCollector` (`internal/le/interoplab/bgp/helper.go`), started from the `ze` image. There is no third-party BMP collector in the lab today.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/plugins/bmp/bmp_locrib.go` - builds the Loc-RIB per-peer header, the Peer Up, the initial dump, the End-of-RIB markers and the Peer Down. `locRIBPeerHeader` sets no `PeerAS`; `localRouterID` returns 0 on an empty OPEN cache; `ensureLocRIBPeerUp` and `primeLocRIBPeerUp` pass nil OPENs; `sendLocRIBPeerDown` sends reason 2; `dumpFamilies` is a package-level array of two families
- [ ] `internal/component/bgp/plugins/bmp/msg.go` - `decodePeerUp` skips OPEN extraction for `PeerTypeLocRIB`; the Peer Down reason constants stop at `PeerDownDeconfigured` = 5; `writePeerUp` and `writePeerDown` are the buffer-first encoders
- [ ] `internal/component/bgp/plugins/bmp/sender.go` - `writePeerUpLocked` takes `sentOpen, recvOpen []byte` and encodes whatever it is given; `startSender` bounces every session on a config apply
- [ ] `internal/component/bgp/plugins/bmp/bmp.go` - `bmpCompositeKey` is `router + ":" + peerAddressString(ph)`; `processPeerUp` logs and calls `bp.state.peerUp`; `parseSenderConfig` reads only the `bmp.sender` subtree out of the `bgp` section; `WantsConfig` names `bgp` and `environment`
- [ ] `internal/component/bgp/plugins/bmp/bmp_events.go` - `peerHeaderFromEvent` sets `PeerFlagO | PeerFlagL` in one statement for the sent direction; `primeLocRIBPeerUp` is called from the session-start path
- [ ] `internal/component/bgp/plugins/bmp/state.go` - `peerKey` is `(router, distinguisher, address)`; `monitoredPeer` carries `PeerAS`, `PeerBGPID`, `IsIPv6`, `IsUp`, `Reason` and no family set
- [ ] `internal/component/bgp/plugins/bmp/tlv.go` - the TLV encoder and `DecodeTLVs`
- [ ] `internal/component/bgp/plugins/bmp/bmp_locrib_test.go` - `TestLocRIBPeerHeader` and `TestHandleBestChangeEmitsPeerUpThenRM` carry the `RFC requirement:` tags for x-1, x-3, x-5, x-6 and x-7
- [ ] `internal/component/bgp/plugins/bmp/peerup_openless_test.go` - proves an OPEN-less Peer Up is suppressed rather than sent, for the Adj-RIB path
- [ ] `internal/component/bgp/message/open.go` - `message.Open` with `MyAS`, `ASN4`, `BGPIdentifier`, `HoldTime`, `OptionalParams`, and a buffer-first `WriteTo(buf, off) int`
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - `bgp { router-id }` is `mandatory true` and rejects 0.0.0.0; `bgp { session { asn { local } } }` is `mandatory true`
- [ ] `rfc/short/rfc9069.md` - seven rows, all `{single-polarity: positive}`, x-3 and x-6 stating the opposite of the RFC
- [ ] `rfc/full/rfc9069.txt` - §4.2, §5.1, §5.2, §5.2.1, §5.3, §6.1.1, §6.1.3
- [ ] `rfc/extraction/rfc8671.json` - site 5.2:1 excluded `binds-another-role`, signed 2026-08-30
- [ ] `internal/le/rfc/check_ratchets.go` - `checkCoverageRatchet`, `checkEvidenceRatchet`, `checkRetiredRequirements`
- [ ] `internal/le/rfc/summary.go` - `extractSection` and the id anchoring rule
- [ ] `internal/le/interoplab/bgp/check_extras.go` - the `bmp-frr` extra check reads `/tmp/bmp-status.json` from the `bmp` peer
- [ ] `internal/le/interoplab/bgp/prepare.go` - the `bmp` peer is started from the `ze` image running `interop-bgp bmp-collector`, conditional on `bmp {` appearing in `ze.conf`

**Behavior to preserve:**
- The End-of-RIB completion contract: every family the dump owes a marker for gets one, including a family the RIB stayed silent about (`closeDumpFamilies`, `TestMixedFamilyDumpClosesTheSilentFamily`).
- The per-dump correlation token, so a replay another subsystem requested is never claimed as this plugin's (`TestForeignReplayIsNotClaimedAsOurDump`).
- Exactly one Loc-RIB Peer Up per RIB instance per BMP session (`RFC9069-x-2`, `TestLocRIBSinglePeerUpPerInstance`), and the per-session `locRIBUpSent` retry-on-failure behavior.
- Flags 0 on the Loc-RIB per-peer header: F=0 means in-Loc-RIB, and V/L/A/O stay clear (`RFC9069-x-1`).
- Peer Address all-zero on the Loc-RIB per-peer header (`RFC9069-x-5`), which §5.1 does require.
- Adj-RIB Peer Up decoding: `decodePeerUp` keeps its mandatory two-OPEN parse for peer types 0 through 2.
- The buffer-first wire contract throughout: `write(buf, off) int`, no allocation in an encoder (`ai/rules/performance.md`).

**Behavior to change:**
- Loc-RIB per-peer header carries the configured local ASN instead of 0.
- Loc-RIB per-peer header takes its Peer BGP ID from config, not from the OPEN cache, and is never 0.
- Loc-RIB Peer Up carries a fabricated OPEN in both the sent and received positions.
- Loc-RIB Peer Up and Peer Down carry Peer Up / Peer Down Information TLV type 3 with the value `global`.
- Loc-RIB Peer Down uses reason code 6.
- `dumpFamilies` stops being an independent declaration and derives from the fabricated OPEN's capability set.
- `decodePeerUp` parses the Loc-RIB OPEN and the receiver records the per-peer family set.
- The Loc-RIB peer's route-injection identity keys on distinguisher and BGP ID rather than on the zero address.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

Three entry points, and the fix touches all three.

1. **Config apply.** The YANG tree delivers the `bgp` section as JSON to the BMP plugin's `OnConfigure` handler (`WantsConfig` names `configRootBGP`). Today only `bgp.bmp.sender` is unwrapped; `bgp.router-id` and `bgp.session.asn.local` arrive in the same payload and are discarded. Format at entry: JSON with every scalar delivered as a string, including numbers and booleans.
2. **A best-path change or an initial dump.** The RIB publishes a `(bgp-rib, best-change)` batch on the in-process EventBus; `handleBestChange` turns each entry into a Loc-RIB Route Monitoring message, preceded by the Loc-RIB Peer Up on the first message of a session. Format at entry: a structured best-change event carrying family, prefix, next hop and attributes.
3. **A collector's BMP stream arriving at Ze's receiver.** `acceptLoop` reads framed BMP messages off an accepted TCP connection and `DecodeMsg` dispatches them; a Peer Up with `PeerType == 3` from a conformant sender carries a fabricated OPEN today and Ze mis-parses it. Format at entry: raw BMP wire bytes.

### Transformation Path

1. Config: the `bgp` section JSON is unwrapped by `bgpSenderSection` in `bmp.go`, which gains the router-id and local-ASN leaves alongside the sender subtree, and stores them where the Loc-RIB header builder can read them.
2. Header build: `locRIBPeerHeader` takes the router-id and the local ASN and returns a `PeerHeader` with `PeerType` 3, Flags 0, zero Address, the configured `PeerAS` and the configured `PeerBGPID`.
3. OPEN fabrication: a new builder assembles a `message.Open` carrying `MyAS` (AS_TRANS when the ASN exceeds 16 bits), the configured `BGPIdentifier`, the ASN4 capability and one Multiprotocol capability per dumped family, and encodes it buffer-first into the session's scratch.
4. Family set: the same capability list is the source `dumpFamilies` reads, so the OPEN Ze advertises and the families the dump closes are one declaration.
5. Peer Up emission: `ensureLocRIBPeerUp` and `primeLocRIBPeerUp` pass the fabricated OPEN bytes to `writePeerUpLocked` in both the sent and received positions, and append the VRF/Table Name TLV.
6. Peer Down emission: `sendLocRIBPeerDown` sends reason code 6 and repeats the VRF/Table Name TLV.
7. Receiver decode: `decodePeerUp` extracts the two OPENs for `PeerTypeLocRIB` as it already does for the Adj-RIB peer types, parses the capability list, and hands the family set to `bmpState.peerUp`.
8. Receiver identity: `bmpCompositeKey` composes the Loc-RIB key from the router, the distinguisher and the Peer BGP ID rather than from the zero address.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ BMP plugin | `bgp` section JSON, all scalars as strings, unwrapped by `bgpSenderSection` | No |
| RIB plugin ↔ BMP plugin | in-process EventBus `(bgp-rib, best-change)`, synchronous delivery | No |
| BMP sender ↔ collector | BMP v3 wire bytes over TCP, per-session byte-bounded transmit queue | No |
| Remote BMP sender ↔ Ze receiver | BMP v3 wire bytes over TCP, `acceptLoop` and `DecodeMsg` | No |
| BMP receiver ↔ RIB | `InjectWireRoute(protocolBMP, peerKey, updateBody)` and `request bgp rib withdraw-protocol` | No |

### Integration Points

- `locRIBPeerHeader` (`bmp_locrib.go`) - every Loc-RIB message's header comes from this one function, so obligations 1 and 6 land in one place and reach Route Monitoring, Peer Up, End-of-RIB and Peer Down together.
- `writePeerUpLocked` (`sender.go`) - already takes both OPEN slices and a TLV-capable encoder; the Loc-RIB path stops passing nil.
- `DecodeTLVs` (`tlv.go`) - the existing TLV reader that the Peer Up Information TLV joins.
- `bgpSenderSection` (`bmp.go`) - the existing config unwrapper the two new leaves join.
- `message.Open` and its `WriteTo(buf, off) int` (`internal/component/bgp/message/open.go`) - the fabrication reuses Ze's own OPEN encoder rather than hand-assembling bytes.
- `capability.CodeMultiprotocol` and `capability.CodeASN4` (`internal/core/bgp/capability/`) - the capability codes the fabricated OPEN carries.

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `bgp.router-id` and `bgp.session.asn.local` arrive in the JSON the BMP plugin already receives, so no new config leaf and no new `WantsConfig` entry is needed | `WantsConfig` names `configRootBGP` (`bmp.go`); both leaves are `mandatory true` under `container bgp` in `internal/component/bgp/yang/ze-bgp-conf.yang` | The plugin needs a second config source or an event, and Phase 2 grows a delivery mechanism | A unit test that feeds the plugin a realistic `bgp` section JSON and asserts the header carries both values | unvalidated |
| A-2 | pmacct's BMP collector daemon runs in a container, accepts a BMP v3 session, and logs enough of a Loc-RIB Peer Up to assert on the OPEN, the Peer AS, the BGP ID and the table name | The lab starts arbitrary sidecar images by name (`interoplab.PeerConfig.Image`, `prepare.go`); no third-party BMP collector exists in the lab today | The sender-side interop scenario needs a different collector. Fallbacks in order: OpenBMP's collector image, then a scenario that captures the wire bytes with tcpdump and decodes them with a non-Ze parser | Phase 0 probes the image: start it, point Ze at it, read what it writes | unvalidated |
| A-3 | FRR 10.3.1's `bmpd` can be configured to send RFC 9069 Loc-RIB monitoring, so it can drive Ze's receiver | FRR is already in the lab at 10.3.1 (`docs/architecture/testing/interop.md`) and ships a BMP module | The receiver-side scenario needs GoBGP's BMP client instead, or a recorded conformant capture replayed at Ze's listener | Phase 0 probes the FRR image's `bmp monitor` configuration and reads what reaches Ze | unvalidated |
| A-4 | A local ASN above 65535 is represented in the fabricated OPEN as `AS_TRANS` in `MyAS` with the true value in the ASN4 capability, matching what Ze's own OPEN encoder does for a real session | `message.Open` carries both `MyAS` and `ASN4` and `AS_TRANS` is a named constant in `open.go` | A 4-byte-ASN router advertises a Loc-RIB peer a collector reads as AS 23456 | A unit test with a 4-byte local ASN asserting both fields | unvalidated |
| A-5 | Correcting the requirement TEXT while leaving every `RFC requirement:` tag in place does not trip `checkCoverageRatchet` or `checkEvidenceRatchet` | Both ratchets compare the set of polarities and evidence tiers per id between baseline and HEAD (`internal/le/rfc/check_ratchets.go`); neither reads the requirement text | Phase 1 cannot lead, and each ledger correction must land in the phase that fixes its obligation | Run `./le rfc check` after Phase 1 and read the output | unvalidated |
| A-6 | The families the Loc-RIB dump carries are knowable at Peer Up time, so the fabricated OPEN can advertise exactly them | `dumpFamilies` is a compile-time array today, and §5.2 says "Only include capabilities if they will be used" | The OPEN advertises a family the dump never carries, or the dump carries one the OPEN did not advertise; either breaks §6.1.1's "which peer belongs to which address family" | A test that drives a dump and compares the advertised capability set against the families that produced markers | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A collector that parsed Ze's OPEN-less Loc-RIB Peer Up by accident breaks when the OPEN appears | The sender-side interop scenario fails at the Peer Up | This is the fix, not a regression: the RFC requires the OPEN. Record the before/after in the scenario so the change is visible |
| R-2 | The receiver-side change to `bmpCompositeKey` changes the identity of EXISTING Adj-RIB peers, orphaning routes in `ribInPool` | An Adj-RIB peer's routes stop being withdrawn on Peer Down | Key only `PeerTypeLocRIB` on the new composition; peer types 0 through 2 keep the address-based key unchanged, and a test asserts the Adj-RIB key is byte-identical |
| R-3 | Phase 1's downward ledger correction is read by another session as a scope reduction, or as a new `{gap}` to act on | A review or a status report treats the interim rows as outstanding work | The correction text names this spec as the closer and states that closure leaves no row `{gap}`. The spec stays claimed until it closes |
| R-4 | The fabricated OPEN is built per Peer Up, allocating on the sender path | An allocation shows up in a sender benchmark | The OPEN is fabricated once when the family set and config are known, cached as bytes on the plugin, and written through the session's existing scratch buffer. Never rebuilt per session |
| R-5 | Neither pmacct nor FRR can be made to work, and the spec has no third-party peer | Phase 0 probes both and both fail | Report to the main thread and ask which way (`ai/rules/rfc-compliance.md`: which way, never whether). Do not close the spec with an interop row that names Ze's own collector |
| R-6 | `decodePeerUp` now parses OPENs for peer type 3, and a malformed or hostile Loc-RIB Peer Up reaches the OPEN parser | A fuzz failure in `fuzz_test.go` | The Loc-RIB path uses `extractBGPOpen`, the same bounds-checked reader the Adj-RIB path uses, and the existing fuzz target is extended to cover peer type 3 |
| R-7 | The `RFC9069-x-3` and `RFC9069-x-6` tags are moved to new tests rather than re-pointed onto corrected assertions, and coverage lapses between phases | `./le rfc check` reports "is no longer proven" | Re-point in place: the same test function keeps the tag, and only the assertion changes |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A BMP collector receives a Loc-RIB feed it cannot parse, or parses into the wrong peer identity. No BGP session is affected: the Loc-RIB path is monitoring output only. On the receiver side, a wrong `bmpCompositeKey` change could orphan routes injected from a monitored router, which is user-visible in `show bgp rib` |
| How is it reverted? | Single commit revert. Nothing persists across a restart and no config migration is involved. A collector that already stored the corrected Peer Up keeps a record of a peer whose AS and BGP ID change back, which is a collector-side cosmetic effect |
| Who else touches this path? | `plan/journal/unwired-feature.md` (BMP statistics-timeout), `plan/journal/memo-suppresses-a-change-it-cannot-see.md` (BMP sender dedup), `plan/journal/zero-value-as-valid-answer.md` (BMP sender config apply) each hold a live BMP row from `spec-rfcgate-6-supported-extraction-signoff`. None is this spec's subject and none is re-recorded here. The parent spec `plan/spec-rfcgate-6-supported-extraction-signoff.md` owns extraction sign-offs, and the corrected rfc9069 summary is the input its rfc9069 walk will need |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `bgp { bmp { sender { loc-rib true } } }` config apply plus a best-path change | → | `locRIBPeerHeader` reading the configured router-id and local ASN | `TestLocRIBHeaderReadsRouterIDAndASNFromConfig` |
| First Loc-RIB message on a collector session | → | `ensureLocRIBPeerUp` writing the fabricated OPEN through `writePeerUpLocked` | `TestLocRIBPeerUpCarriesFabricatedOpen` |
| A collector connecting after monitoring started | → | `primeLocRIBPeerUp` writing the same fabricated OPEN and TLV | `TestPrimedLocRIBPeerUpCarriesOpenAndTableName` |
| BMP sender teardown with Loc-RIB announced | → | `sendLocRIBPeerDown` emitting reason 6 with the table-name TLV | `TestLocRIBPeerDownUsesReasonSixWithTableName` |
| A conformant Loc-RIB Peer Up arriving at Ze's listener | → | `decodePeerUp` extracting the OPENs and `bmpState.peerUp` recording the family set | `TestReceiverParsesLocRIBPeerUpCapabilities` |
| A Route Monitoring message from a monitored Loc-RIB instance | → | `bmpCompositeKey` composing the key from distinguisher and BGP ID | `TestLocRIBRouteInjectionKeyedOnDistinguisherAndBGPID` |
| `./le integration interop bmp-locrib-pmacct` | → | the whole Loc-RIB sender path | scenario `bmp-locrib-pmacct` |
| An operator running `show bmp peers` with Loc-RIB monitoring on | → | `peersCommand` reporting the Loc-RIB peer's AS and BGP ID | `test/plugin/bmp-locrib-peer-fields.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Loc-RIB monitoring is on and the config carries `session { asn { local 65044 } }` | Every Loc-RIB per-peer header Ze emits carries Peer AS 65044, on Route Monitoring, Peer Up, End-of-RIB and Peer Down alike |
| AC-2 | Loc-RIB monitoring is on and the config carries `router-id 172.30.0.2` | Every Loc-RIB per-peer header carries Peer BGP ID 172.30.0.2, whether or not any BGP peer has ever come up, and the value is never 0 |
| AC-3 | The first Loc-RIB message on a collector session | The Peer Up carries a non-empty sent OPEN and a received OPEN with byte-identical content, each a well-formed BGP OPEN whose BGP Identifier is the configured router-id |
| AC-4 | The fabricated OPEN is decoded | It carries the 4-octet ASN capability, and exactly one Multiprotocol capability for each family the Loc-RIB dump can carry, and no other capability |
| AC-5 | The local ASN is 4259905000, above the 16-bit range | The fabricated OPEN's My AS field is AS_TRANS and the 4-octet ASN capability carries 4259905000 |
| AC-6 | A Loc-RIB Peer Up on the default instance, whose distinguisher is zero-filled | It carries Peer Up Information TLV type 3 with the UTF-8 value `global` |
| AC-7 | Loc-RIB monitoring is torn down after a Peer Up was announced | The Peer Down carries reason code 6 and repeats Peer Down Information TLV type 3 with the value `global` |
| AC-8 | The set of families the fabricated OPEN advertises is compared against the set of families the dump sends an End-of-RIB marker for | The two sets are equal, and both are read from one declaration |
| AC-9 | A conformant Loc-RIB Peer Up carrying a fabricated OPEN arrives at Ze's BMP listener | Ze extracts both OPEN messages, parses their Multiprotocol capabilities, and records the family set against that peer; no OPEN byte is mistaken for an Information TLV |
| AC-10 | Two Loc-RIB Peer Ups arrive from one router with different Peer Distinguishers and different Peer BGP IDs | Ze holds them as two distinct route-injection identities; a Route Monitoring message from one never overwrites the other's routes |
| AC-11 | An Adj-RIB Peer Up (peer type 0, 1 or 2) arrives | Its route-injection key is byte-identical to the key Ze produced before this spec |
| AC-12 | `rfc/short/rfc9069.md` is read after closure | `RFC9069-x-3` states the fabricated-OPEN requirement and `RFC9069-x-6` states the primary-ASN requirement, each under its original id; every row has a positive test; no row is `{gap}` or `{not-applicable}` |
| AC-13 | `rfc/short/rfc9069.md` is read after closure | It carries `RFC9069-4.2-1` for the locally-sourced-routes obligation and `RFC9069-6.1.3-1` for the session-bounce obligation, each with a positive and a negative test |
| AC-14 | `docs/features/rfc-status.md` and `rfc/enrolled.txt` are read after closure | Both describe what the code does: the fabricated OPEN, the primary ASN, the config-sourced router-id, the table-name TLV, reason code 6, and the receiver's capability processing. Neither claims a behavior that is absent |
| AC-15 | `./le integration interop bmp-locrib-pmacct` runs | A BMP collector that is not Ze reads Ze's Loc-RIB stream, reports the Peer Up as a Loc-RIB instance peer, and shows the AS, the BGP ID and the table name Ze was configured with |
| AC-16 | `./le integration interop bmp-locrib-receiver-frr` runs | Ze's BMP receiver accepts a third-party Loc-RIB feed, records its family set, and injects its routes under a distinguisher-and-BGP-ID identity |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables `loc-rib true` and points a third-party BMP collector at Ze | config apply -> `startLocRIB` -> initial dump -> `ensureLocRIBPeerUp` -> fabricated OPEN + TLV on the wire -> collector parses it | scenario `bmp-locrib-pmacct` |
| 2 | Runs `show bmp peers` while Loc-RIB monitoring is on | CLI -> `peersCommand` -> `bmpState` -> the Loc-RIB peer row | `test/plugin/bmp-locrib-peer-fields.ci` |
| 3 | Points a third-party router's Loc-RIB BMP feed at Ze's BMP listener and looks at the resulting routes | `acceptLoop` -> `DecodeMsg` -> `decodePeerUp` -> capability parse -> `bmpCompositeKey` -> `InjectWireRoute` -> `show bgp rib` | scenario `bmp-locrib-receiver-frr` |
| 4 | Disables `loc-rib` and watches the collector | config apply -> `sendLocRIBPeerDown` -> reason 6 + TLV -> collector marks the Loc-RIB instance down | scenario `bmp-locrib-pmacct`, teardown step |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLocRIBHeaderReadsRouterIDAndASNFromConfig` | `internal/component/bgp/plugins/bmp/rfc9069_test.go` | AC-1, AC-2: the header carries the configured ASN and router-id with an empty OPEN cache | |
| `TestLocRIBHeaderNeverCarriesZeroBGPID` | `internal/component/bgp/plugins/bmp/rfc9069_test.go` | AC-2 negative: with no config delivered, the plugin refuses to emit rather than emitting BGP ID 0 | |
| `TestLocRIBPeerHeader` (corrected) | `internal/component/bgp/plugins/bmp/bmp_locrib_test.go` | `RFC9069-x-6` re-pointed: the assertion becomes "Peer AS equals the configured local ASN". Flags, Address and BGP ID assertions unchanged | |
| `TestLocRIBPeerUpCarriesFabricatedOpen` | `internal/component/bgp/plugins/bmp/rfc9069_test.go` | AC-3, AC-4: both OPEN fields non-empty and byte-identical, ASN4 present, one Multiprotocol per dumped family | |
| `TestHandleBestChangeEmitsPeerUpThenRM` (corrected) | `internal/component/bgp/plugins/bmp/bmp_locrib_test.go` | `RFC9069-x-3` re-pointed: the assertion becomes "both OPENs are present and identical" | |
| `TestFabricatedOpenUsesASTransForFourByteASN` | `internal/component/bgp/plugins/bmp/rfc9069_test.go` | AC-5 | |
| `TestFabricatedOpenCarriesNoUnusedCapability` | `internal/component/bgp/plugins/bmp/rfc9069_test.go` | AC-4 negative: a capability Ze negotiates on real sessions but does not use for Loc-RIB is absent | |
| `TestLocRIBPeerUpCarriesGlobalTableName` | `internal/component/bgp/plugins/bmp/rfc9069_test.go` | AC-6 | |
| `TestPrimedLocRIBPeerUpCarriesOpenAndTableName` | `internal/component/bgp/plugins/bmp/rfc9069_test.go` | AC-3, AC-6 on the late-connecting-collector path | |
| `TestLocRIBPeerDownUsesReasonSixWithTableName` | `internal/component/bgp/plugins/bmp/rfc9069_test.go` | AC-7 | |
| `TestLocRIBPeerDownNeverUsesReasonTwo` | `internal/component/bgp/plugins/bmp/rfc9069_test.go` | AC-7 negative | |
| `TestAdvertisedFamiliesMatchDumpFamilies` | `internal/component/bgp/plugins/bmp/rfc9069_test.go` | AC-8: one declaration, read two ways, compared | |
| `TestReceiverParsesLocRIBPeerUpCapabilities` | `internal/component/bgp/plugins/bmp/rfc9069_test.go` | AC-9 | |
| `TestReceiverRejectsLocRIBPeerUpWithTruncatedOpen` | `internal/component/bgp/plugins/bmp/rfc9069_test.go` | AC-9 negative: a short OPEN is an error, never a silently empty capability set | |
| `TestLocRIBRouteInjectionKeyedOnDistinguisherAndBGPID` | `internal/component/bgp/plugins/bmp/rfc9069_test.go` | AC-10 | |
| `TestAdjRIBInjectionKeyUnchanged` | `internal/component/bgp/plugins/bmp/rfc9069_test.go` | AC-11: the peer-type 0-2 key is byte-identical to today's | |
| `TestLocallySourcedRoutesUseLocRIBPeerType` | `internal/component/bgp/plugins/bmp/rfc9069_test.go` | AC-13: `RFC9069-4.2-1` positive | |
| `TestNonLocalRoutesDoNotUseLocRIBPeerType` | `internal/component/bgp/plugins/bmp/rfc9069_test.go` | AC-13: `RFC9069-4.2-1` negative | |
| `TestBehaviorChangeBouncesTheSession` | `internal/component/bgp/plugins/bmp/rfc9069_test.go` | AC-13: `RFC9069-6.1.3-1` positive, a config change that alters behavior produces Peer Down then Peer Up | |
| `TestUnchangedConfigDoesNotBounceTheSession` | `internal/component/bgp/plugins/bmp/rfc9069_test.go` | AC-13: `RFC9069-6.1.3-1` negative | |
| `FuzzDecodeLocRIBPeerUp` | `internal/component/bgp/plugins/bmp/fuzz_test.go` | R-6: the new peer-type-3 OPEN parse is bounds-safe | |

### Boundary Tests (numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Local ASN in the fabricated OPEN My AS field | 1-65535 direct, above that AS_TRANS | 65535 direct | 0 (rejected by the YANG `asn` type) | 65536 becomes AS_TRANS 23456, with the true value in the ASN4 capability |
| ASN4 capability value | 1-4294967295 | 4294967295 | 0 (rejected by YANG) | N/A |
| Peer Down reason code | 1-6 defined by RFC 7854 and RFC 9069 | 6 for Loc-RIB | N/A | 7 is undefined and never emitted |
| VRF/Table Name TLV length | 1-255 bytes per §5.2.1 | 255 | 0 is refused, so the TLV is omitted rather than emitted empty | 256 is refused |
| Peer BGP ID | 1-4294967295, RFC 6286 forbids 0 | 4294967295 | 0 refused: the plugin declines to emit rather than sending a zero BGP ID | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bmp-locrib-peer-fields` | `test/plugin/bmp-locrib-peer-fields.ci` | An operator enables `loc-rib true` and runs `show bmp peers`; the Loc-RIB peer reports the configured AS and router-id, not zeros | |

### Interop Tests (Scope: protocol)

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bmp-locrib-pmacct` | `test/interop/scenarios/bmp-locrib-pmacct/` | pmacct BMP collector (A-2; fallbacks OpenBMP, then a non-Ze wire decoder) | A collector that is not Ze parses Ze's Loc-RIB Peer Up, reads the fabricated OPEN's capabilities, and reports the Peer AS, the Peer BGP ID and the `global` table name Ze was configured with. Covers AC-1 to AC-8 and AC-15 | |
| `bmp-locrib-receiver-frr` | `test/interop/scenarios/bmp-locrib-receiver-frr/` | FRR 10.3.1 `bmpd` with Loc-RIB monitoring (A-3; fallback GoBGP's BMP client) | Ze's BMP receiver accepts a third-party Loc-RIB feed, processes its OPEN capabilities, and keys its routes on distinguisher and BGP ID. Covers AC-9, AC-10 and AC-16 | |

**Vacuity discipline (`ai/rules/interop-and-goal-validation.md`):** for each
scenario, revert the fix, REBUILD the Ze image the scenario drives, confirm the
scenario goes RED, restore the fix, confirm GREEN, and paste the RED output into
the closure sections. A scenario added against already-working code has no
proven discrimination.

## Files to Modify

- `internal/component/bgp/plugins/bmp/bmp_locrib.go` - `locRIBPeerHeader` gains the ASN and takes the router-id from config; `localRouterID` is replaced by a config read; the fabricated OPEN builder; `dumpFamilies` derives from the advertised capability set; `sendLocRIBPeerDown` uses reason 6 and the TLV; the stale `dumpFamilies` comment is rewritten
- `internal/component/bgp/plugins/bmp/bmp.go` - `bgpSenderSection` unwraps `router-id` and `session.asn.local`; the plugin stores them; `bmpCompositeKey` composes the Loc-RIB identity from distinguisher and BGP ID and leaves the Adj-RIB composition unchanged; `processPeerUp` records the parsed family set
- `internal/component/bgp/plugins/bmp/msg.go` - the reason-6 constant with its RFC 9069 §5.3 citation; `decodePeerUp` extracts both OPENs for `PeerTypeLocRIB`; the stale comment at the peer-type branch is removed
- `internal/component/bgp/plugins/bmp/sender.go` - the Loc-RIB Peer Up carries the Information TLV; the Peer Down carries its TLV payload
- `internal/component/bgp/plugins/bmp/tlv.go` - the VRF/Table Name TLV type 3 constant and its RFC 9069 §5.2.1 citation
- `internal/component/bgp/plugins/bmp/state.go` - `monitoredPeer` gains the family set parsed from a Loc-RIB Peer Up
- `internal/component/bgp/plugins/bmp/bmp_locrib_test.go` - `TestLocRIBPeerHeader` and `TestHandleBestChangeEmitsPeerUpThenRM` corrected in place, keeping their `RFC requirement:` tags on the same ids
- `internal/component/bgp/plugins/bmp/fuzz_test.go` - the peer-type-3 Peer Up decode target
- `rfc/short/rfc9069.md` - x-3 and x-6 text corrected under the same ids; two new rows citing §4.2 and §6.1.3; the `{single-polarity: positive}` annotations rewritten so each cites the corrected producer
- `rfc/enrolled.txt` - the rfc9069 line rewritten to describe what Ze does
- `docs/features/rfc-status.md` - the RFC 9069 row: `Partial` with the six named in Phase 1, back to `Supported` with an accurate coverage sentence at closure
- `docs/guide/bmp.md` - the Loc-RIB section describes the fabricated OPEN, the Peer AS, the config-sourced router-id, the `global` table name and the reason-6 Peer Down
- `docs/architecture/core-design.md` - the Design owner of `bmp.go`, `bmp_events.go`, `msg.go`, `sender.go`, `header.go`, `state.go` and `tlv.go`: the BMP receiver's peer-identity composition and the Loc-RIB decode path change
- `internal/le/interoplab/bgp/prepare.go` - the sidecar wiring for the two new scenarios
- `internal/le/interoplab/bgp/check_extras.go` - the assertions for the two new scenarios
- `internal/le/interoplab/bgp/checkers.go` - the base checks for the two new scenarios
- `docs/architecture/testing/interop.md` - the scenario inventory rows and the new peer daemon

## Files to Create

- `internal/component/bgp/plugins/bmp/rfc9069_test.go` - the tagged tests for every requirement this spec touches, in the shape `rfc8671_test.go` already uses in this package
- `test/plugin/bmp-locrib-peer-fields.ci` - the operator-visible check
- `test/interop/scenarios/bmp-locrib-pmacct/ze.conf` and its collector configuration
- `test/interop/scenarios/bmp-locrib-receiver-frr/ze.conf` and `frr.conf`

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No new leaf. Both values this spec needs, `bgp { router-id }` and `bgp { session { asn { local } } }`, already exist and are `mandatory true`. Adding a BMP table-name leaf was considered and rejected: Ze has one Loc-RIB instance with a zero-filled distinguisher, so `global` is the whole answer (`ai/rules/simplicity.md`) |
| YANG validation constraints | N-A | No new leaf. The existing `router-id` leaf already carries `ze:validate "nonzero-ipv4"` |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | No | `show bmp peers` gains no new verb; its existing rows carry corrected values |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | Yes | `test/plugin/bmp-locrib-peer-fields.ci` |
| Pipe completeness | N-A | `peersCommand` already routes through the shared pipe path and gains no new output shape |
| Env var registration | N-A | No leaf under `environment/` changes |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module, binary or certificate. The BMP listener and the collector connections already exist |
| Prometheus counters/metrics | No | No new observable state. The Loc-RIB peer is already reported through `show bmp peers` |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new SAFI, capability or attribute. The fabricated OPEN carries capability codes Ze already implements (`capability.CodeASN4`, `capability.CodeMultiprotocol`) for families it already supports |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | No new feature. `docs/features.md` describes BMP Loc-RIB monitoring already and its description stays true |
| 2 | Config syntax changed? | No | No config syntax changes |
| 3 | CLI command added/changed? | No | No command changes |
| 4 | API/RPC added/changed? | No | No RPC changes |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` is checked for a Loc-RIB claim; the BMP page carries the detail |
| 6 | Has a user guide page? | Yes | `docs/guide/bmp.md`, the Loc-RIB section |
| 7 | Wire format changed? | Yes | `docs/guide/bmp.md` carries the BMP wire description for this plugin. `docs/architecture/wire/` covers BGP wire encoding and holds no BMP page; confirm with `./le spec citation anchors` rather than from memory |
| 8 | Plugin SDK/protocol changed? | No | The plugin's event and command surface is unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc9069.md`, `rfc/enrolled.txt`, and the `docs/features/rfc-status.md` RFC 9069 row, each with source anchors |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` for the new `.ci`, and `docs/architecture/testing/interop.md` for the two scenarios and the new peer daemon |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` is checked: a claim that Ze does RFC 9069 Loc-RIB monitoring is only true after this spec lands |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, the BMP sections declared by seven files' `// Design:` headers |
| 13 | Route metadata keys added/changed? | No | No metadata key changes. Loc-RIB Route Monitoring content is untouched |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration change. The plugin, its events and its commands are the same set |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED, run on 2026-08-30: `./le spec citation anchors spec plan/spec-fixit-locrib-peer-fields-contradict-rfc9069.md` reports no BLOCKING owner and two advisory mentions. `docs/guide/bmp.md` already carries `<!-- source: internal/component/bgp/plugins/bmp/bmp_locrib.go -->` and `<!-- source: rfc/short/rfc9069.md -->`, so it is in scope by anchor as well as by content. The two advisory mentions are UNAFFECTED and named here with their reason: `docs/guide/configuration.md` mentions `sender.go` for the BMP collector config block, and no config syntax changes; `docs/guide/vrrp.md` mentions `prepare.go` and `checkers.go` for the VRRP interop scenarios, and the two scenarios added here are BMP. Re-run the command after each phase, because the changed-file set grows |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/bmp.md` shows a `loc-rib true` configuration block and a sequence description; verify both against the parser and the emitter after the change |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- make the config values reachable and the tests fail for the right reason
   - Tests: `TestLocRIBHeaderReadsRouterIDAndASNFromConfig`, `TestLocRIBHeaderNeverCarriesZeroBGPID`
   - Files: `bmp.go` (`bgpSenderSection` unwrapping, plugin storage), `bmp_locrib.go` (`locRIBPeerHeader` signature), `rfc9069_test.go`
   - Verify: the plugin holds the router-id and local ASN after a config apply, and the two new tests fail because the header still carries zeros
2. **Phase: Ledger correction (leads, and lands as its own commit)** -- make the public record true before any fix
   - Tests: `./le rfc check` after the edit, reading the output for a ratchet complaint (A-5)
   - Files: `rfc/short/rfc9069.md` (x-3 and x-6 TEXT only, tags untouched), `rfc/enrolled.txt`, `docs/features/rfc-status.md`, then `./le rfc index-update` to regenerate `rfc/requirements/rfc9069.md` and `ai/RFC-REQUIREMENTS.md`
   - Verify: no id retired, no tag moved, no coverage lost. The row names this spec as its closer
3. **Phase: Per-peer header (obligations 1 and 6)** -- the ASN and the router-id
   - Tests: `TestLocRIBHeaderReadsRouterIDAndASNFromConfig`, `TestLocRIBHeaderNeverCarriesZeroBGPID`, and `TestLocRIBPeerHeader` corrected in place under `RFC9069-x-6`
   - Files: `bmp_locrib.go`, `bmp_locrib_test.go`, `docs/guide/bmp.md`
   - Verify: `localRouterID`'s OPEN-cache walk is gone rather than kept as a fallback (`ai/rules/no-layering.md`), and no path can produce a zero BGP ID
4. **Phase: Fabricated OPEN (obligation 2) and the family declaration** -- the largest phase
   - Tests: `TestLocRIBPeerUpCarriesFabricatedOpen`, `TestFabricatedOpenUsesASTransForFourByteASN`, `TestFabricatedOpenCarriesNoUnusedCapability`, `TestAdvertisedFamiliesMatchDumpFamilies`, `TestPrimedLocRIBPeerUpCarriesOpenAndTableName`, and `TestHandleBestChangeEmitsPeerUpThenRM` corrected in place under `RFC9069-x-3`
   - Files: `bmp_locrib.go`, `sender.go`, `bmp_locrib_test.go`, `rfc9069_test.go`, `docs/guide/bmp.md`
   - Verify: the OPEN is fabricated once and cached as bytes (R-4); `dumpFamilies` reads the advertised set rather than declaring its own; the comment justifying the hardcoded array is rewritten
5. **Phase: Table name TLV and reason code 6 (obligations 3 and 4)**
   - Tests: `TestLocRIBPeerUpCarriesGlobalTableName`, `TestLocRIBPeerDownUsesReasonSixWithTableName`, `TestLocRIBPeerDownNeverUsesReasonTwo`
   - Files: `tlv.go`, `msg.go`, `sender.go`, `bmp_locrib.go`, `docs/guide/bmp.md`
   - Verify: the Peer Up TLV and the Peer Down TLV are emitted from one decision, per §5.2.1's "If the TLV is included, then it MUST also be included in the Peer Down notification"
6. **Phase: Receiver (obligation 5)** -- decode the OPEN, key the identity
   - Tests: `TestReceiverParsesLocRIBPeerUpCapabilities`, `TestReceiverRejectsLocRIBPeerUpWithTruncatedOpen`, `TestLocRIBRouteInjectionKeyedOnDistinguisherAndBGPID`, `TestAdjRIBInjectionKeyUnchanged`, `FuzzDecodeLocRIBPeerUp`
   - Files: `msg.go`, `bmp.go`, `state.go`, `fuzz_test.go`, `rfc9069_test.go`, `docs/architecture/core-design.md`
   - Verify: the Adj-RIB key is byte-identical (R-2); a short OPEN is an error and never an empty capability set (`ai/rules/principles.md`)
7. **Phase: The two unrowed sites** -- give each an id and both polarities
   - Tests: `TestLocallySourcedRoutesUseLocRIBPeerType`, `TestNonLocalRoutesDoNotUseLocRIBPeerType`, `TestBehaviorChangeBouncesTheSession`, `TestUnchangedConfigDoesNotBounceTheSession`
   - Files: `rfc/short/rfc9069.md` (two new rows citing "Section 4.2" and "Section 6.1.3"), `rfc9069_test.go`, then `./le rfc index-update`
   - Verify: the ids land as `RFC9069-4.2-1` and `RFC9069-6.1.3-1`, each with both polarities, neither annotated `{single-polarity}`
8. **Phase: Interop and closure of the record**
   - Tests: scenarios `bmp-locrib-pmacct` and `bmp-locrib-receiver-frr`, plus `test/plugin/bmp-locrib-peer-fields.ci`
   - Files: the two scenario directories, `prepare.go`, `checkers.go`, `check_extras.go`, `docs/architecture/testing/interop.md`, `docs/functional-tests.md`, and the final pass over `rfc/enrolled.txt` and `docs/features/rfc-status.md`
   - Verify: the revert-rebuild-RED-restore-GREEN cycle is run per scenario and the RED output recorded. Phase 0 of this phase probes the two daemon images (A-2, A-3) before any scenario file is written; if both fail, report and ask which way (R-5)

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol, and the six obligations each map to at least one AC |
| Feature completeness | All four user stories have a working path; story 3 in particular needs a real third-party sender, not a Ze-to-Ze loop |
| Correctness | The received OPEN is a byte-for-byte repeat of the sent OPEN, not a re-encode. The advertised capability set equals the dump's family set. Reason code 6 is emitted only for Loc-RIB, never for an Adj-RIB peer |
| Naming | The requirement ids are `RFC9069-x-3`, `RFC9069-x-6`, `RFC9069-4.2-1` and `RFC9069-6.1.3-1`; no id renumbered or removed. Scenario directories carry no numeric prefix |
| Data flow | The router-id and the ASN reach the header from CONFIG only. No fallback to the OPEN cache survives anywhere |
| Rule: `ai/rules/principles.md` | No zero reads as a valid answer: an absent router-id makes the plugin decline to emit, with a log line, rather than sending 0 |
| Rule: `ai/rules/no-layering.md` | `localRouterID`'s OPEN-cache walk is DELETED, not kept beside the config read |
| Rule: `ai/rules/stale-comments.md` | The `dumpFamilies` comment, the `decodePeerUp` peer-type-3 comment, and `locRIBPeerHeader`'s doc comment all currently assert the behavior this spec removes. Each is rewritten in the phase that changes its code |
| Rule: `ai/rules/rfc-compliance.md` | Every changed MUST carries `// RFC 9069 Section X.Y: "<quoted requirement>"` directly above the enforcing code |
| Rule: `ai/rules/performance.md` | The fabricated OPEN is built once and written through existing scratch. No allocation is added to the per-message sender path |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Peer AS is the configured local ASN on every Loc-RIB message | `go test ./internal/component/bgp/plugins/bmp/ -run 'LocRIB.*Header'` |
| The Loc-RIB Peer Up carries two identical non-empty OPENs | `go test ./internal/component/bgp/plugins/bmp/ -run 'FabricatedOpen'` |
| Reason code 6 exists and is used | `grep -n 'PeerDownLocRIB' internal/component/bgp/plugins/bmp/msg.go` and the reason-6 tests |
| The table-name TLV appears in both directions | `go test ./internal/component/bgp/plugins/bmp/ -run 'TableName'` |
| The receiver parses a Loc-RIB OPEN | `go test ./internal/component/bgp/plugins/bmp/ -run 'ReceiverParses'` |
| No requirement id retired, no coverage lost | `./le rfc check` |
| Every requirement has a test binding | `./le rfc index-update` then read `rfc/requirements/rfc9069.md` for an empty Positive test cell |
| The ledger says what the code does | `./le spec citation anchors spec plan/spec-fixit-locrib-peer-fields-contradict-rfc9069.md` |
| Both interop scenarios exist and discriminate | `./le integration interop bmp-locrib-pmacct` and `./le integration interop bmp-locrib-receiver-frr`, each with its recorded RED |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | `decodePeerUp` now runs the OPEN parser on peer type 3 bytes from a remote sender. Every length read goes through `extractBGPOpen`'s bounds check, and a truncated or over-long OPEN is an error rather than a partial parse |
| Resource exhaustion | The capability list parsed from a remote OPEN is bounded by the OPEN's own declared length, which is bounded by the BMP message length. No unbounded slice is grown from a remote field |
| Error leakage | A decode failure logs the router and the failure kind, never the raw bytes |
| Identity confusion | The new Loc-RIB route-injection key is composed from remote-supplied distinguisher and BGP ID. Two routers must not be able to collide on one key: the router remote address stays the first component |
| Fail closed | An absent router-id makes the Loc-RIB Peer Up NOT be sent, matching the existing Adj-RIB behavior that suppresses an OPEN-less Peer Up rather than emitting a malformed one |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| `./le rfc check` reports a lost polarity or a retired id | The tag was moved rather than re-pointed. Restore the tag on the corrected assertion under the same id |
| Neither candidate third-party daemon works | Report to the main thread with what each did, and ask which way (R-5). Do not close with a Ze-to-Ze interop row |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The `dumpFamilies` comment is the clearest evidence that the two defects are one defect: it justifies a hardcoded family list with "there is no negotiated family set to derive this from" BECAUSE the OPENs are zero-length. Fixing the OPEN removes the justification and supplies the declaration, so obligations 2 and 8 close together rather than as two independent edits.
- `TestPeerUpOnCacheMissNeverReachesTheCollector` (`peerup_openless_test.go`, landed 2026-08-30) already establishes in this repository that an OPEN-less Peer Up is unparseable, and it establishes it for the Adj-RIB path. The Loc-RIB path was doing deliberately what the Adj-RIB path suppresses, on the strength of a summary line that reversed the RFC.
- The receiver defect is the more serious of the two halves and was not in the original find list: `decodePeerUp` SKIPS the OPEN for peer type 3, citing the same false summary. So Ze cannot read any conformant implementation's Loc-RIB Peer Up, and the error is silent: the OPEN bytes are walked as Information TLVs.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Read the router-id and the local ASN from the `bgp` config subtree the plugin already receives | Read them from the cached sent OPEN as `localRouterID` does today; add a new BMP config leaf for each; ask the reactor over an event | The config is present from the first apply, before any peer comes up, which is exactly the case §1.1 names ("VRF tables with no peers"). The OPEN cache is empty in that case, which is how the zero got produced. A new leaf would be a second declaration of a value the tree already carries (`ai/rules/principles.md`) |
| Emit the VRF/Table Name TLV unconditionally with the value `global` | Omit it, reading §5.2.1's "optionally included to support implementations that may not have defined a name" as permission; add a config leaf so an operator can name the instance | §5.2.1 admits two readings: the TLV is optional absent a configured name, or the default instance always carries `global`. Emitting it is conformant under BOTH readings, so the ambiguity does not need an owner ruling. §6.1.1 also says the receiver "uses the VRF/Table Name from the Peer Up information to associate a name with the Loc-RIB", which a collector cannot do if Ze stays silent. No config leaf: Ze has one Loc-RIB instance with a zero-filled distinguisher, so there is nothing to name (`ai/rules/simplicity.md`). The leaf arrives with VRF Loc-RIB instances, if they ever do |
| Derive the fabricated OPEN's capability set and `dumpFamilies` from one declaration | Keep `dumpFamilies` hardcoded and advertise the same two families in the OPEN by hand | Two hand-written lists of the same fact disagree eventually and nothing arbitrates them (`ai/rules/principles.md`). §5.2's "Only include capabilities if they will be used for Loc-RIB monitoring messages" and §6.1.1's "know which peer belongs to which address family" make the two facts the same fact by construction |
| Key only `PeerTypeLocRIB` on distinguisher plus BGP ID; leave peer types 0 through 2 on the address | Key every peer type on distinguisher plus address plus BGP ID | The Adj-RIB key is live: it addresses `ribInPool[bmpProtocolID]` and the `withdraw-protocol` command. Changing it orphans routes for a defect that only exists for Loc-RIB peers, whose address is zero by requirement (R-2) |
| Correct the ledger downward FIRST, keeping every requirement tag in place | Correct the ledger at closure, when it is true again; correct it and move the tags at once | A false public claim is corrected as soon as it is known, and `ai/rules/rfc-compliance.md` makes a stale claim pointing away from compliance void rather than citable. Keeping the tags is forced by `checkCoverageRatchet`, which is monotonic on polarity coverage and does not read requirement text |
| Leave `rfc/extraction/rfc8671.json` site 5.2:1 excluded and unsigned-again | Implement pre-policy Adj-RIB-Out export, making the obligation live and forcing a resign | RFC 7854 §5 makes the choice a MAY, and `ai/rules/rfc-compliance.md` forbids an agent from picking a MAY. It is a separable feature with no data source in Ze, so `ai/rules/rule-precedence.md` homes it in its own spec with its own owner question |

## Known Limitations

- **Pre-policy Adj-RIB-Out export is not implemented and is not in this spec.** RFC 7854 §5 makes it a MAY ("A BMP speaker may send pre-policy routes, post-policy routes, or both"), so it is Thomas's decision under `ai/rules/rfc-compliance.md`: implement it, skip it, or make it a config option. Until he answers, `rfc/extraction/rfc8671.json` site 5.2:1 stays excluded and the artifact keeps its 2026-08-30 sign-off. This is the ONE question this spec puts to the owner, and it is a MAY question, not a compliance question.
- **`rfc/extraction/rfc9069.json` does not exist**, so RFC 9069 has never had a section walk. That absence is what let a code-derived summary stand for a year. Writing the walk belongs to `plan/spec-rfcgate-6-supported-extraction-signoff.md`, which owns extraction sign-offs; this spec supplies the corrected summary that walk will read. Not a deferral: it is another spec's declared scope.
- **Multiple Loc-RIB instances are not implemented.** §6.1.1's "There MUST be at least one emulated peer for each Loc-RIB instance, such as with VRFs" is satisfied on the sender side by Ze having exactly one Loc-RIB and one emulated peer for it. The receiver side is made ready for many (AC-10), because a monitored router can have them. Ze gaining its own VRF Loc-RIB instances is feature work with no code path today.
- **Three live BMP journal rows are not this spec's subject** and are not re-recorded: `plan/journal/unwired-feature.md` (the unwired `statistics-timeout`), `plan/journal/memo-suppresses-a-change-it-cannot-see.md` (the sender dedup memo that survives a contradicting withdraw), and `plan/journal/zero-value-as-valid-answer.md` (the sender config apply that treats an empty collector set as "nothing to do"). None blocks any of the sixteen ACs.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

The citations this spec owes, one per changed obligation:

| Code site | Citation owed |
|-----------|---------------|
| `locRIBPeerHeader`, the `PeerAS` assignment | RFC 9069 Section 5.1: "Peer Autonomous System (AS): Set to the primary router BGP autonomous system number (ASN)." |
| `locRIBPeerHeader`, the `PeerBGPID` assignment | RFC 9069 Section 5.1: "Peer BGP ID: Set the ID to the router-id of the VRF instance if VRF is used; otherwise, set to the global instance router-id." |
| the fabricated OPEN builder | RFC 9069 Section 5.2: "Sent OPEN Message: This is a fabricated BGP OPEN message. Capabilities MUST include the 4-octet ASN and all necessary capabilities to represent the Loc-RIB Route Monitoring messages." |
| the received-OPEN assignment | RFC 9069 Section 5.2: "Received OPEN Message: Repeat of the same sent OPEN message." |
| the capability set the OPEN advertises | RFC 9069 Section 6.1.1: "Each emulated peer instance MUST send a Peer Up with the OPEN message indicating the address family capabilities." |
| the VRF/Table Name TLV constant and its emission | RFC 9069 Section 5.2.1: "The default value of \"global\" MUST be used for the default Loc-RIB instance with a zero-filled distinguisher." |
| the Peer Down TLV repeat | RFC 9069 Section 5.3: "The VRF/Table Name informational TLV MUST be included if it was in the Peer Up." |
| the reason-6 constant and its use | RFC 9069 Section 5.3: "The Peer Down notification MUST use reason code 6." |
| `decodePeerUp`'s peer-type-3 branch and the capability parse | RFC 9069 Section 6.1.1: "A BMP receiver MUST process these capabilities to know which peer belongs to which address family." |
| `bmpCompositeKey`'s Loc-RIB composition | RFC 9069 Section 6.1.1: "The BMP receiver identifies the Loc-RIB by the peer header distinguisher and BGP ID." |
| `handleBestChange`'s peer-type choice | RFC 9069 Section 4.2: "If locally sourced routes are communicated using BMP, they MUST be conveyed using the Loc-RIB Instance Peer Type." |
| `startSender`'s bounce | RFC 9069 Section 6.1.3: "In case of any change that results in the alteration of behavior of an existing BMP session ... the session MUST be bounced with a Peer Down/Peer Up sequence." |

Wire format documentation owed: the Loc-RIB Peer Up body with byte offsets
(Local Address 16, Local Port 2, Remote Port 2, Sent OPEN variable, Received
OPEN variable, Information TLVs variable) and the Peer Down body (Reason 1,
then TLVs), each as an ASCII diagram with the RFC section reference, in
`docs/guide/bmp.md`.

## Review Gate

<!-- Filled at implementation time by /ze-review, per .claude/rules/planning.md.
     Round 1 reviews the whole diff with at least two lenses; each later round
     reviews only the fixes the previous round made plus their sibling call
     sites, and each round's scope is written here BEFORE it runs. -->

### Round 1

| Scope | Lenses | BLOCKER | ISSUE | NOTE |
|-------|--------|---------|-------|------|
| | | | | |

### Round 2

| Scope | Lenses | BLOCKER | ISSUE | NOTE |
|-------|--------|---------|-------|------|
| | | | | |

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
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-16 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination
- [ ] `./le rfc check` clean: no id retired, no polarity lost, no evidence tier lost
- [ ] No row of `rfc/short/rfc9069.md` reads `{gap}` or `{not-applicable}` at closure

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)
- [ ] Each interop scenario's RED phase forced by reverting the fix and REBUILDING the image, and the RED output recorded

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
