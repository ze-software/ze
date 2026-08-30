# Spec: bgp-pcap-decode -- a pcap ze can write truthfully and read back

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | cli |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/bgp-pcap-decode.md` (create on the first deferral) |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| Handoff | - |
| Updated | 2026-08-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`ze bgp decode` accepts one hex string on the command line. Operators capture BGP
with tcpdump and with ze itself, and ze can decode neither capture.

Two problems, and the first blocks the second.

**Ze writes a pcap that lies.** `exportBGPPcap`
(`internal/plugins/diag/cmd/capture_raw.go`) declares `LinkTypeRaw` (101, raw
IPv4 or IPv6) and then writes bytes that are none of that. The bytes come from
`BGPRawCaptureRing`, filled by `notifyMessageReceiver` with `rawBytes`, which is
the BGP message BODY: the 19-byte header is already gone. Verified at the
producer. `session_read.go` passes `body` with `hdr.Type` as a separate argument,
`session_write.go` slices the body only once `n >= message.HeaderLen`, and
`notifyMessageReceiver` reads `rawBytes[0]` and `rawBytes[1]` as the NOTIFICATION
error code and subcode, which sit at offsets 19 and 20 of a whole message.

So the file has no IP header, no TCP header and no BGP header, while declaring
raw IP. Wireshark parses a BGP body as an IPv4 header and renders garbage.
Nothing can recover message type or message boundaries from it. Recorded in
`plan/journal/declared-format-contradicts-payload.md`.

`docs/architecture/diagnostics/packet-capture.md` documents this as a design
choice and the claim is false: it says the rings "hold no link layer, so they use
`LINKTYPE_RAW` (101)". 101 means raw IPv4 or IPv6, not "no link layer", and the
rings hold no IP layer either. The same false claim sits in the `LinkTypeRaw` doc
comment. The page reasons correctly that `DLT_RAW` "would produce a corrupt file"
for Ethernet frames; the same reasoning applied to bare BGP bodies is the
unwritten half.

**Ze cannot read a pcap at all.** There is a writer and no reader.

The goal is one round trip that holds: ze captures BGP, Wireshark opens it as
BGP, and `ze bgp decode` reads the same file back.

## What this spec owes

| Piece | Note |
|-------|------|
| A truthful writer | IP and TCP headers around each FULL BGP message, keeping `LINKTYPE_RAW` (101) so the existing declaration becomes true. Owner decision, 2026-08-15 |
| The header back | The capture point receives a header-stripped body. The 19-byte header is reconstructed at the tap, matching the sibling spec's decision |
| `internal/core/pcap` | A synthetic IP and TCP writer ALREADY exists in `internal/analyze/convert.go`. The new package absorbs it rather than inventing framing, and carries three of its defects forward as fixes |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| A reader with TCP reassembly | The real cost. No reassembly code exists anywhere in the repository |
| stdin and multi-payload | exabgp's decode reads hex lines from stdin; ze takes one argv string |

## Non-goals

The rendering of a decoded message is `plan/spec-bgp-decode-render.md`. This spec
delivers bytes to the decoder and does not decide how they are displayed. The two
specs share the `ze bgp decode` entry point and nothing else.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### Architecture Docs
- [ ] `ai/rules/plugins.md` - the plugin rule: registration, placement, transport, command surface and process boundary
- [ ] `docs/architecture/api/commands.md` - the verb-first API command paths, with JSON or text encoding
- [ ] `docs/architecture/mrt.md` - the three areas MRT (Multi-Threaded Routing Toolkit) support covers

- [ ] `docs/architecture/diagnostics/packet-capture.md` - the design doc for the capture feature this spec fixes
  → Constraint: the page carries `source:` anchors for `pcap.go`, `capture_raw.go`, `capture.go`, `capture_common.go`, both raw rings and both interface-capture files. Every anchor must be repointed when code moves, and `./le doc check verify` plus `./le doc wiring` own that.
  → Decision: the page's `LINKTYPE_RAW` justification is false and is corrected here, not preserved. It does not mention `internal/analyze/convert.go` at all, which is how a second pcap writer stayed invisible.

- [ ] `ai/rules/architecture.md` - tier placement and the core import direction
  → Constraint: `internal/core/pcap` must import nothing from `internal/component/` or `internal/plugins/`. Achievable: the writer needs only `encoding/binary`, `io`, `time` and `net/netip`.  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
  → Decision: no row in `internal/le/`; that manifest covers only component and plugin paths. `internal/component/bgp/cli`, `internal/plugins/diag/cmd` and `internal/analyze` importing it are all downward imports and legal.

- [ ] `ai/rules/cli.md` - the `-` stdin contract
  → Constraint: a user-supplied path must go through `internal/core/cliio`, never a raw `os` call.
  → Constraint: `./le dash-stdio check` is an AST taint pass whose `scanRoots` EXCLUDE `internal/core`. A raw `os.Open` inside `internal/core/pcap` would not be flagged, so the `-` handling MUST live at the CLI edge in `internal/component/bgp/cli`, which is scanned.  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->

- [ ] `ai/rules/interop-and-goal-validation.md` - proving the test discriminates
  → Constraint: `test/plugin/diag-capture.ci` exercises capture in JSON only and asserts no pcap bytes, so the format change breaks no existing `.ci` and gains no evidence from one. The discriminating test must be new: a file ze wrote, decoded back, asserting the messages.

**Key insights:** (minimal context to resume after compaction)

- A synthetic IP+TCP+BGP pcap writer already exists in `internal/analyze/convert.go`. Absorb it; do not invent framing.
- Its three defects: sequence and ack always 0, checksums 0, IPv6 records silently skipped.
- `exportBGPPcap` is ALSO the BFD exporter. BFD and L2TP are UDP payloads, so TCP/179 framing must not reach them.
- The full wire message IS available at both session capture points, but NOT at the ring. The header is reconstructed at the tap.
- No TCP reassembly exists in the repository. That is the largest single item here.

## Current Behavior (MANDATORY)

**Source files read:** (verified at the producer, 2026-08-15)

- [ ] `internal/plugins/diag/cmd/capture_raw.go` - `HandleCaptureRaw` implements `capture-raw [start|stop|dump] [l2tp|bgp|bfd] [pcap|json]`; `exportBGPPcap` writes the pcap for BOTH the BGP ring and the BFD ring
- [ ] `internal/plugins/diag/cmd/pcap.go` - `writePcapHeader`, `writePcapPacket`, `LinkTypeRaw`; unexported file-format code inside an edge plugin
- [ ] `internal/plugins/diag/cmd/capture_interface.go` - `writePcapPacketWithOrigLen`, a private sibling that already carries a separate original length; plus `formatIPv4Packet`, `formatIPv6Packet`, `skipIPv6ExtHeaders`, `formatTransport`, which parse Ethernet, 802.1Q, IPv4 IHL, the IPv6 extension chain and TCP flags but produce text rather than structs
- [ ] `internal/analyze/convert.go` - `writePcapGlobalHeader` and `writePcapBGPPacket` write pcap record plus IPv4 (20 bytes) plus TCP (20 bytes, ports 179, PSH and ACK) plus a whole BGP message, under `LINKTYPE_IPV4` (228), for `ze-analyse convert pcap`. Sequence and ack are always 0, checksums are 0, and IPv6 records are counted into `skippedV6` and dropped
- [ ] `internal/component/bgp/reactor/raw_capture.go` - `BGPRawCaptureRing`, 256 slots of 4096 bytes; `BGPRawCaptureEntry` carries timestamp, direction and data, and nothing else
- [ ] `internal/component/plugin/types_bgp.go` - `plugin.BGPRawCaptureEntry` carries timestamp and direction as strings, plus data
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - `notifyMessageReceiver` has `peerAddr` as its first parameter and builds `peerInfo` including the local address immediately above the ring append
- [ ] `internal/component/bgp/reactor/session_read.go` - `teeCapture` receives the whole wire message; `onMessageReceived` receives the body
- [ ] `internal/component/bgp/reactor/peer.go` - `(*Peer).tCPPorts` is the only reader of the real ports, and takes two mutexes
- [ ] `internal/component/bgp/cli/decode.go` - `cmdDecode` takes one hex argument; `decodeHexPacket` is the per-message decode a reader would feed
- [ ] `internal/core/cliio/cliio.go` - `IsStdin`, `ReadFile` (claims stdin once, capped at `MaxStdinBytes`, `ErrStdinClaimed` on a second claim), `OpenReader` (uncapped stream, same one-shot claim), `Create`, `WriteFile`, `SwapStreams`
- [ ] `internal/plugins/vrrp/packet/validate.go` - `StripIPv4Header`, the IHL-aware strip with named errors, repeated at `internal/plugins/rsvpte/transport_linux.go`

**Behavior to preserve:**

- `capture-raw start|stop|dump` keeps its grammar and its in-memory, non-persistent semantics.
- The L2TP pcap keeps its own framing: it is a UDP payload and must not gain TCP/179 headers.
- The BFD pcap likewise: `exportBGPPcap` currently serves it and must be split rather than shared.
- The interface capture keeps `linkTypeEthernet` and its own original-length handling.
- `ze bgp decode <hex>` keeps working with a single argv hex string.

**Behavior to change:**

- The BGP pcap gains IP and TCP framing and whole BGP messages, making its `LINKTYPE_RAW` declaration true.
- Sequence and ack numbers become monotonic per direction; checksums are computed; IPv6 peers are no longer skipped.
- `ze bgp decode` gains pcap input, stdin, and multi-payload hex.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

- Write side: a BGP message reaching `notifyMessageReceiver`, then the ring, then `show capture-raw dump bgp pcap`.
- Read side: a pcap file path or `-` given to `ze bgp decode`.
- Read side: hex lines on stdin, one message per line.

### Transformation Path

1. Write: the tap reconstructs the 19-byte header and appends the whole message plus peer metadata to the ring.
2. Write: `exportBGPPcap` frames each entry as IP plus TCP plus the message, with per-direction monotonic sequence numbers, and writes a pcap record under link type 101.
3. Read: the pcap reader parses the file header, then each record.
4. Read: per record, parse IP by version nibble, then TCP, and select the port-179 flows.
5. Read: reassemble each direction's byte stream in sequence order.
6. Read: frame BGP messages on the 19-byte header within the reassembled stream.
7. Read: each framed message goes to `decodeHexPacket`, unchanged.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Reactor to ring | `Append` with the whole message plus addresses | No |
| Ring to diag plugin | `plugin.BGPRawCaptureEntry`, a cross-boundary value type | No |
| Diag plugin to file | pcap bytes, base64 in the RPC response | No |
| File to CLI | `cliio.OpenReader`, path or `-` | No |
| Reader to decoder | one framed BGP message per call to `decodeHexPacket` | No |

### Integration Points

- `internal/core/pcap` becomes the single owner of the pcap file format; `internal/analyze/convert.go`, `internal/plugins/diag/cmd` and the new reader all use it.  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
- `decodeHexPacket` keeps its signature; the reader is a new producer of its input.
- `BGPRawCaptureRing.Append` gains the fields framing needs.

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | |
| No unintended coupling | No | |
| No duplicated functionality | No | |
| Zero-copy preserved where applicable | No | |
| Registration over hardcoding | No | |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Fabricated TCP ports are acceptable, so `(*Peer).tCPPorts` need not be called on the capture path. | `internal/analyze/convert.go` already fabricates ports 179 and 179; reading the real ports takes two mutexes on the hot path. | Captures show ports that never existed, which could mislead an operator correlating with a real tcpdump. | Confirm with the owner; state the fabrication in the docs page. | unvalidated |
| A-2 | Peer and local addresses are in scope at the ring append with no new locking. | `notifyMessageReceiver` takes `peerAddr` as its first parameter and builds `peerInfo` including the local address immediately above the append. | The framing needs a lookup that costs a mutex on the read path, and the design changes. | Read the function again at implementation time and confirm no new lock is taken. | unvalidated |
| A-3 | Wireshark reassembles BGP correctly once sequence numbers are monotonic per direction. | Wireshark's BGP dissector uses TCP stream reassembly, which needs consistent sequence numbers; the current all-zero values mark every record a retransmission. | The file still fails to show messages split across records, and the AC that says "opens as BGP" is unmet. | Open a generated file in Wireshark or tshark and assert the dissected message count. | unvalidated |
| A-4 | Link type 101 accepts both IPv4 and IPv6 by the version nibble, so one link type serves both. | The pcap LINKTYPE_RAW definition; the existing declaration in `exportBGPPcap`. | IPv6 peers need a separate link type or a separate file, and the IPv6 fix does not land as designed. | Generate an IPv6 capture and open it. | unvalidated |
| A-5 | Every BGP message in a capture can be framed from the reassembled stream alone. | The 19-byte header carries a length field, and the marker is all ones. | A capture starting mid-stream cannot find the first boundary, and the reader needs marker resynchronisation. | Decode a capture deliberately started mid-session. | unvalidated |
| A-6 | Moving the pcap writers into one package does not change the interface-capture or L2TP output. | Both have their own link types and their own callers; only the shared record and header writers move. | A diagnostic format an operator already depends on changes silently. | Byte-compare an interface capture and an L2TP capture before and after the move. | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | TCP framing leaks into the BFD or L2TP export, because `exportBGPPcap` serves BFD today. | A BFD capture opens as malformed TCP in Wireshark. | Split the exporter explicitly as the first step, with a test per protocol, before any framing work. |
| R-2 | A truncated message carries a real BGP header whose length exceeds the record, and Wireshark reports malformed. | Wireshark shows malformed on long UPDATEs; the ring slot is 4096 while an extended message reaches 65535. | Use the original-length field so the record declares the true length and the capture length separately. `writePcapPacketWithOrigLen` already exists and is promoted for this. |
| R-3 | TCP reassembly is underestimated: out-of-order segments, retransmissions, overlapping data and gaps are all normal in a real capture. | The reader works on ze's own files and fails on a tcpdump capture from a real network. | Test against a real tcpdump capture, not only round-tripped ze output. Fail closed on a gap rather than silently concatenating across it. |
| R-4 | The reader silently produces nothing for a capture it cannot parse, and the operator reads empty output as "no BGP here". | A capture known to contain BGP decodes to zero messages without an error. | Fail closed per `ai/rules/evidence.md`: report what was seen (records read, flows found, bytes reassembled) and why nothing framed. |
| R-5 | `internal/analyze/convert.go` tests pin link type 228 and the exact byte layout, so the move breaks them. | `TestWritePcapGlobalHeader` and `TestWritePcapBGPPacket` fail. | They move with the code and are updated deliberately. Unifying on 101 is a decision, recorded below, not an accident. |
| R-6 | The stdin single-claim guard fires when a path and `-` are both supplied. | `ErrStdinClaimed` reaches the user as an internal-looking error. | Reject the combination at argument parsing with a message naming both inputs. |
| R-7 | A malicious or corrupt pcap drives unbounded memory during reassembly. | Memory growth decoding an untrusted file. | Bound the per-flow reassembly buffer and the flow count; report when a bound is hit rather than growing. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A diagnostic artifact format, and the new reader. Nothing reaches a peer, nothing changes what the daemon puts on the wire, and no config is affected. The BFD and L2TP captures are the collateral to watch, since they share the exporter today. |
| How is it reverted? | Single commit revert. Files already written by the old code stay unreadable either way, since they were never decodable. |
| Who else touches this path? | `spec-improve-3-event-replay` (in-progress) works the reactor capture area; `plan/spec-bgp-decode-render.md` shares the `ze bgp decode` entry point and the header-reconstruction decision. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `show capture-raw dump bgp pcap` after a session exchanged messages | → | the split BGP exporter in `internal/plugins/diag/cmd` using `internal/core/pcap` | `test-bgp-pcap-roundtrip.ci` |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `ze bgp decode pcap <file>` | → | the pcap reader and reassembler | `TestReadPcapFramesMessages` |
| `ze bgp decode pcap -` with a capture on stdin | → | `cliio.OpenReader` at the CLI edge | `test-bgp-decode-pcap-stdin.ci` |
| Hex lines on stdin | → | the multi-payload path in `cmdDecode` | `test-bgp-decode-stdin-hex.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A session exchanges OPEN, UPDATE and KEEPALIVE, then `show capture-raw dump bgp pcap` | Each record holds an IP header, a TCP header and one whole BGP message including its 19-byte header, under link type 101 |
| AC-2 | The file from AC-1 opened with tshark | Every message is dissected as BGP, with the message count matching what was exchanged |
| AC-3 | Two messages captured in the same direction | Their TCP sequence numbers increase by the preceding message's length, so neither is marked a retransmission |
| AC-4 | Any record from AC-1 | The IP and TCP checksums are correct for the header and payload as written |
| AC-5 | A session with an IPv6 peer | Records are written as IPv6, none skipped, and the count matches |
| AC-6 | A message longer than the 4096-byte ring slot | The record declares the true original length and the shorter capture length, and tshark does not report malformed |
| AC-7 | `show capture-raw dump bfd pcap` and `show capture-raw dump l2tp pcap` | Output is byte-identical to before this change; no TCP or port-179 framing appears |
| AC-8 | `ze bgp decode pcap <tcpdump-capture>` for a capture containing a full BGP session | Every BGP message in the capture is decoded, in wire order, with its direction |
| AC-9 | A capture where one BGP message spans three TCP segments | The message is reassembled and decoded once, not three times and not dropped |
| AC-10 | A capture where one TCP segment carries three BGP messages | All three are framed and decoded |
| AC-11 | A capture with out-of-order or retransmitted segments | The stream is reassembled in sequence order and each message decoded exactly once |
| AC-12 | A capture containing non-BGP traffic alongside BGP | Non-BGP flows are ignored without error, and BGP flows are decoded |
| AC-13 | A capture with a gap in the TCP stream | The reader reports the gap and what it could not frame; it never concatenates across the gap and presents the result as valid |
| AC-14 | A capture that contains no BGP at all | The reader reports records read and flows examined, and exits non-zero rather than printing nothing |
| AC-15 | `ze bgp decode pcap -` with a capture piped on stdin | Decodes identically to the same capture given as a path |
| AC-16 | Several hex messages, one per line, piped on stdin | Each is decoded in order, matching exabgp's decode behavior |
| AC-17 | A path and `-` both supplied | Rejected at argument parsing with a message naming both, never an internal stdin-claimed error |
| AC-18 | A capture written by ze in AC-1, then read by `ze bgp decode pcap` | Every message decodes, proving the round trip |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | captures BGP on a live daemon and opens it in Wireshark | `capture-raw start bgp` → tap → ring → `dump bgp pcap` → `internal/core/pcap` → file | `test-bgp-pcap-roundtrip.ci` plus the tshark assertion |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| 2 | decodes a tcpdump capture from a colleague | file → `cliio.OpenReader` → reader → reassembly → framing → `decodeHexPacket` | `TestReadPcapFramesMessages` |
| 3 | pipes a capture through ze in a shell pipeline | stdin → `cliio.OpenReader` → same chain | `test-bgp-decode-pcap-stdin.ci` |
| 4 | pastes several hex messages at once, as with exabgp | stdin lines → `cmdDecode` multi-payload → `decodeHexPacket` | `test-bgp-decode-stdin-hex.ci` |
| 5 | captures with ze and decodes it back with ze | story 1 then story 2 | `test-bgp-pcap-roundtrip.ci` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPcapGlobalHeader` | `internal/core/pcap/pcap_test.go` | magic, version, snaplen and link type bytes | |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestPcapRecordWithOrigLen` | `internal/core/pcap/pcap_test.go` | capture length and original length differ correctly (AC-6) | |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestFrameIPv4TCP` | `internal/core/pcap/pcap_test.go` | IPv4 and TCP header layout and checksums (AC-4) | |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestFrameIPv6TCP` | `internal/core/pcap/pcap_test.go` | IPv6 framing under link type 101 (AC-5) | |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestSequenceMonotonicPerDirection` | `internal/core/pcap/pcap_test.go` | AC-3 | |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestReadPcapRecords` | `internal/core/pcap/reader_test.go` | file header and record iteration, both endiannesses | |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestReassembleInOrder` | `internal/core/pcap/reassemble_test.go` | AC-9 | |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestReassembleOutOfOrder` | `internal/core/pcap/reassemble_test.go` | AC-11 | |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestReassembleGapFailsClosed` | `internal/core/pcap/reassemble_test.go` | AC-13 | |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestReassembleBounded` | `internal/core/pcap/reassemble_test.go` | R-7, memory bound honoured | |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestFrameMultipleMessagesPerSegment` | `internal/core/pcap/reassemble_test.go` | AC-10 | |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestReadPcapFramesMessages` | `internal/component/bgp/cli/decode_pcap_test.go` | AC-8, end to end over a fixture | |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestNonBGPFlowsIgnored` | `internal/component/bgp/cli/decode_pcap_test.go` | AC-12 | |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestNoBGPFoundIsAnError` | `internal/component/bgp/cli/decode_pcap_test.go` | AC-14 | |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestPathAndStdinRejected` | `internal/component/bgp/cli/decode_pcap_test.go` | AC-17 | |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestRingCarriesFramingFields` | `internal/component/bgp/reactor/raw_capture_test.go` | the ring stores addresses and the whole message | |

### Boundary Tests (numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| BGP message length in the header | 19-65535 | 65535 | 18 | 65536 |
| Ring slot capture length | 1-4096 | 4096 | 0 | 4097 (truncated, original length carries the truth) |
| IPv4 IHL | 5-15 | 15 | 4 | 16 |
| TCP data offset | 5-15 | 15 | 4 | 16 |
| Per-flow reassembly buffer | bounded, value set at design | the bound | N/A | one byte past the bound reports rather than grows |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-bgp-pcap-roundtrip.ci` | `test/plugin/` | an operator captures BGP with ze and decodes the file back with ze (AC-18) | |
| `test-bgp-decode-pcap-stdin.ci` | `test/decode/` | an operator pipes a capture into `ze bgp decode pcap -` | |
| `test-bgp-decode-stdin-hex.ci` | `test/decode/` | an operator pastes several hex messages, exabgp style | |
| `diag-capture.ci` (existing, must keep passing) | `test/plugin/` | capture start, dump and stop still work in JSON | |

#### Peer-block conventions for the new `.ci` (read before authoring)

Two guards landed in the shared checkout on 2026-08-15 and were still
UNCOMMITTED when this spec was written, so `git log` will not explain them. Both
bite a newly authored peer block. Confirm they are in the tree before assuming
either applies.

| Guard | Producer | What to do when authoring |
|-------|----------|---------------------------|
| RFC 4271 Section 5.1.3: a route is withheld from a peer when the NEXT_HOP is that peer's own address | `egressNextHopIsPeerOwn` and `originatedNextHopIsPeerOwn`, `internal/component/bgp/reactor/forward_next_hop.go` | Give `connection > remote > ip` and `connection > local > ip` DIFFERENT addresses. The old suite convention used one address for both, which makes `next-hop self` resolve to the peer's own address, and the route is withheld. Use 127.0.0.1 to 127.0.0.5 on this host |
| A directive inside a `stdin=<name>:terminator=` block that no consumer claims fails at parse time, naming file, line and reason | `internal/test/runner/peer_contract.go` | Every line in a peer block must be a directive a consumer claims. Such a line used to be dropped silently, so a typo now fails loudly rather than passing vacuously |

This matters twice here. `test-bgp-pcap-roundtrip.ci` needs a live session, so it
carries a peer block. AC-5 needs an IPv6 peer, and the distinct-address rule
applies there too: use `fd00::2` and a different local address rather than one
address for both ends. The framing this spec writes reads the peer and local
addresses out of the capture, so two ends that share an address would also make
the generated pcap describe a session from a host to itself.

### Interop Tests (Scope: protocol)

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | | | This spec changes no wire-visible behavior: it changes a diagnostic file format and adds a reader. The discriminating evidence is the tshark dissection in AC-2, which is a third-party parser reading ze's output, and the round trip in AC-18 | N/A |

## Files to Modify

- `internal/plugins/diag/cmd/capture_raw.go` - split the BGP exporter from the BFD exporter; frame BGP with IP and TCP
- `internal/plugins/diag/cmd/pcap.go` - remove the file-format code that moves to `internal/core/pcap`  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
- `internal/plugins/diag/cmd/capture_interface.go` - use the promoted original-length record writer
- `internal/plugins/diag/cmd/capture_raw_l2tp.go` - use the moved writers, output unchanged
- `internal/analyze/convert.go` - rewire off its private copies onto `internal/core/pcap`; fix the IPv6 skip  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
- `internal/analyze/convert_test.go` - the link-type and layout assertions move with the code
- `internal/component/bgp/reactor/raw_capture.go` - carry the whole message plus peer and local addresses
- `internal/component/bgp/reactor/reactor_notify.go` - reconstruct the header at the tap and pass the framing fields
- `internal/component/plugin/types_bgp.go` - the cross-boundary entry type gains the same fields
- `internal/component/bgp/cli/decode.go` - pcap input, stdin, and multi-payload
- `internal/component/bgp/cli/register.go` - help and subcommand hints for the new input forms
- `docs/architecture/diagnostics/packet-capture.md` - correct the false `LINKTYPE_RAW` justification, document the framing and the fabricated ports, repoint every source anchor
- `docs/guide/command-reference.md` - the new decode input forms
- `ai/INDEX.md` - discovery row for pcap decoding

## Files to Create

- `internal/core/pcap/pcap.go` - file header and record writers, link-type constants, IP and TCP framing  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
- `internal/core/pcap/reader.go` - file header and record reading  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
- `internal/core/pcap/reassemble.go` - TCP stream reassembly and BGP message framing  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
- `internal/core/pcap/pcap_test.go`, `reader_test.go`, `reassemble_test.go` - unit tests  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
- `internal/component/bgp/cli/decode_pcap.go` - the CLI edge: path or `-`, flow selection, per-message decode  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
- `internal/component/bgp/cli/decode_pcap_test.go` - unit tests  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
- `test/plugin/test-bgp-pcap-roundtrip.ci`, `test/decode/test-bgp-decode-pcap-stdin.ci`, `test/decode/test-bgp-decode-stdin-hex.ci` - functional tests  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
- a pcap fixture captured from a real BGP session, for the reader tests

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | `capture-raw` keeps its existing `ze:command`; no new command or leaf |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | Yes | `internal/component/bgp/cli/decode.go`, for pcap input and stdin. Offline `cmd/ze` tooling, so flag form is permitted there and only there |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md`: the pcap input takes a closed keyword before the path, never a bare positional that could collide |
| Editor autocomplete | N-A | Offline tooling; the runtime `show bgp decode` surface gains no new YANG node |
| Functional test for new RPC/API | Yes | `test-bgp-pcap-roundtrip.ci`, `test-bgp-decode-pcap-stdin.ci`, `test-bgp-decode-stdin-hex.ci` |
| Pipe completeness | N-A | `ze bgp decode` is offline tooling and does not route through `ApplyPipes` today; this spec does not change that surface |
| Env var registration | N-A | No new `environment/` leaf |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module, binary or certificate. The reader opens a file the operator names, which is an argument rather than a dependency |
| Prometheus counters/metrics | N-A | Capture is operator-triggered and non-persistent; a counter was considered and rejected as machinery nobody asked for |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new family, capability or attribute |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (decode a pcap; capture that opens in Wireshark) |
| 2 | Config syntax changed? | No | No YANG leaf added or changed |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, for the pcap and stdin input forms |
| 4 | API/RPC added/changed? | No | `capture-raw` keeps its response shape; only the bytes inside the base64 change |
| 5 | Plugin added/changed? | No | The diag plugin keeps its surface |
| 6 | Has a user guide page? | Yes | `docs/architecture/diagnostics/packet-capture.md` is the owning page and carries a false claim to correct |
| 7 | Wire format changed? | No | Nothing reaches a BGP peer. The pcap file format is a diagnostic artifact, not a wire format |
| 8 | Plugin SDK/protocol changed? | Yes | `internal/component/plugin/types_bgp.go` changes a cross-boundary value type, so `docs/architecture/api/process-protocol.md` is checked |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Header reconstruction relies on RFC 4271's all-ones marker but enforces no obligation |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, for the three new `.ci` tests and the pcap fixture |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`: exabgp decodes hex and stdin, and this closes that gap plus pcap |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` for the new `internal/core/pcap` package |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| 13 | Route metadata keys added/changed? | No | No metadata key touched |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration inventory changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/diagnostics/packet-capture.md` anchors `pcap.go`, `capture_raw.go`, `capture_common.go`, both raw rings and both interface files. Every one is repointed |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify every capture example on the packet-capture page against the handler after the split |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- entry points exist and fail honestly
   - Tests: `TestReadPcapFramesMessages`, `test-bgp-pcap-roundtrip.ci`
   - Files: `internal/core/pcap/` (stubs), `internal/component/bgp/cli/decode_pcap.go` (stub reached from `cmdDecode`)  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
   - Verify: `ze bgp decode pcap <file>` reaches the stub and reports not-implemented; the tests fail for that reason
2. **Phase: split the exporter** -- BFD and L2TP off the BGP path, BEFORE any framing work
   - Tests: AC-7 byte-comparison tests for BFD and L2TP
   - Files: `internal/plugins/diag/cmd/capture_raw.go`, `capture_raw_l2tp.go`
   - Verify: both outputs byte-identical to before. This is R-1 and it goes first
3. **Phase: `internal/core/pcap` writer** -- absorb `convert.go`'s framing, fix its three defects  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
   - Tests: the `pcap_test.go` unit tests, including sequence, checksums and IPv6
   - Files: `internal/core/pcap/pcap.go`, `internal/analyze/convert.go`, `internal/analyze/convert_test.go`, `internal/plugins/diag/cmd/pcap.go`  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
   - Verify: `./le tier check`, and `ze-analyse convert pcap` output still opens
4. **Phase: the capture side** -- whole messages and framing fields into the ring
   - Tests: `TestRingCarriesFramingFields`, then AC-1 to AC-6
   - Files: `internal/component/bgp/reactor/raw_capture.go`, `reactor_notify.go`, `internal/component/plugin/types_bgp.go`
   - Verify: a generated file dissects in tshark
5. **Phase: the reader and reassembly** -- the largest item
   - Tests: the `reader_test.go` and `reassemble_test.go` suites, then AC-8 to AC-14
   - Files: `internal/core/pcap/reader.go`, `reassemble.go`  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
   - Verify: against a real tcpdump capture, not only round-tripped ze output (R-3)
6. **Phase: the CLI edge** -- pcap input, stdin, multi-payload
   - Tests: AC-15 to AC-17, then the three `.ci` tests
   - Files: `internal/component/bgp/cli/decode.go`, `decode_pcap.go`, `register.go`
   - Verify: `./le dash-stdio check`, `./le functional decode`
7. **Phase: documentation and discovery** -- every row of the Documentation checklist
   - Files: the docs listed under Files to Modify, plus `ai/INDEX.md`
   - Verify: `./le doc check verify`, `./le doc wiring`

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:symbol |
| Feature completeness | All five user stories have a working path; story 5 is the round trip and is not assumed from stories 1 and 2 separately |
| Correctness | Checksums are computed over the bytes actually written; sequence numbers advance by payload length, not by record count |
| Collateral | BFD and L2TP captures byte-identical, proven by test rather than by reading the diff |
| Fail closed | A gap, a bound hit, or a capture with no BGP each report what was seen; none returns empty as success (`ai/rules/evidence.md`) |
| Data flow | Reassembly happens in `internal/core/pcap` only; the CLI edge owns `-` handling because the dash-stdio gate does not scan core |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| Rule: `ai/rules/architecture.md` | `internal/core/pcap` imports nothing from component or plugins |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| Rule: `ai/rules/no-layering.md` | `convert.go`'s private writers are DELETED, not left beside the shared package |
| Naming | The pcap input keyword precedes the path, per `ai/rules/cli.md` |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| One pcap file-format owner | `grep -rn 'writePcapHeader\|writePcapPacket\|writePcapGlobalHeader' internal/` names only `internal/core/pcap` |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| Truthful BGP capture | tshark dissects a generated file and reports the expected BGP message count |
| BFD and L2TP unchanged | byte-comparison test in the suite |
| The reader | `go test -race ./internal/core/pcap` |
| Round trip | `./le functional plugin` for `test-bgp-pcap-roundtrip.ci` |
| stdin honoured | `./le dash-stdio check` |
| Core tier respected | `./le tier check` |
| Docs corrected | `./le doc check verify`, `./le doc wiring` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | A pcap is untrusted input. Record lengths, IHL, TCP data offset and BGP length are all attacker-controlled and each must be bounds-checked before use |
| Resource exhaustion | Reassembly buffers and flow counts are bounded; a crafted capture must not drive unbounded memory (R-7) |
| Path handling | The file path comes from the operator and goes through `cliio`; no raw `os` call at the CLI edge |
| Information disclosure | A capture holds the peer's routing data. It holds no local secret: TCP-MD5 keys never appear on the wire. State this on the packet-capture page, matching how the `capture` container documents the same trade |
| Error leakage | A parse failure names the offset and the field, without echoing unbounded attacker-controlled bytes |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| tshark reports malformed | Check original length (R-2) and sequence numbers (A-3) before changing framing |
| BFD or L2TP output changed | Phase 2 was incomplete. Stop and finish the split before continuing |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

<!-- LIVE: write immediately when you learn something. -->

- A second pcap writer existed the whole time, in `internal/analyze/convert.go`, and the owning architecture page does not mention it. The page documented one writer's behavior as if it were the subsystem's, which is how the two drifted into different link types and different correctness.
- The defect was invisible because no test ever read the file back. `test/plugin/diag-capture.ci` asserts JSON only, and there is no unit test over `exportBGPPcap` at all. A writer whose output nothing parses cannot be caught by writer tests.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| IP and TCP framing, keeping link type 101 | Synthetic Ethernet with link type 1; a private link type carrying bare BGP; matching `convert.go`'s 228 | 101 already declares raw IPv4 or IPv6, so adding real IP and TCP makes the existing declaration true instead of replacing it. Ethernet adds 14 bytes carrying no information. 228 is IPv4-only and would preserve the IPv6-skip defect. Owner decision, 2026-08-15 |
| Absorb `convert.go`'s writer rather than write new framing | A fresh implementation in `internal/core/pcap` | Uniformity: one framing implementation, one set of fixes. Writing a second would leave two shapes and repeat the drift that produced this defect |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| Unify both consumers on link type 101 | Leave `convert.go` on 228 | Two link types for one framing implementation is the state that caused this. 101 also fixes `convert.go`'s silent IPv6 skip for free, since the version nibble selects the family |
| Reconstruct the 19-byte header at the tap | Widen `MessageCallback` with a wire parameter; move the capture beside `teeCapture` | Byte-exact from data already in scope, and touches no call site. Widening changes six-plus sites and one caller has no wire bytes. Matches the sibling spec so both taps behave identically. Owner decision, 2026-08-15 |
| Fabricate TCP ports rather than read the real ones | Call `(*Peer).tCPPorts` at capture time | The real ports need two mutexes on the session read path. `convert.go` already fabricates. The fabrication is documented rather than hidden (A-1) |
| `-` handled at the CLI edge, not in `internal/core/pcap` | Handle stdin inside the reader | `./le dash-stdio check` deliberately excludes `internal/core` from its scan roots, so a raw `os.Open` there would pass unflagged. Putting it at the scanned edge keeps the gate meaningful |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| Split the BFD exporter from the BGP exporter first | Share one exporter with a protocol switch | BFD and L2TP are UDP payloads. A shared exporter that grew TCP framing is exactly how a correct BFD capture would silently become malformed (R-1) |

## Known Limitations

- The fabricated TCP ports mean a ze-written capture cannot be correlated byte-for-byte with a simultaneous tcpdump. It is a readable reconstruction, not a packet-level record.
- The 4096-byte ring slot still truncates extended messages up to 65535 bytes. This spec makes the truncation honest, by declaring the true original length, rather than removing it. Enlarging the ring is a separate memory-budget decision.
- Reassembly is per capture file and in memory. Very large captures are bounded by the flow limits rather than streamed.
- Only TCP port 179 flows are examined. A BGP session on a non-standard port needs the port supplied, which is not in this spec.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-18 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Review Gate

<!-- Filled by /ze-close via /ze-review. Do not delete this section. -->

| Run | Date | Blockers | Issues | Result |
|-----|------|----------|--------|--------|
| | | | | |
