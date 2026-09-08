# Spec: bgp-pcap-decode -- a pcap ze can write truthfully and read back

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | - |
| Phase | 7/7 |
| Handoff | - |
| Updated | 2026-09-06 |

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
| Reactor to ring | `Append` with the whole message plus addresses | Yes -- `TestRingCarriesFramingFields` reads the ring back |
| Ring to diag plugin | `plugin.BGPRawCaptureEntry`, a cross-boundary value type | Yes -- `TestExportBGPPcapRoundTrip` builds the entries and exports them |
| Diag plugin to file | pcap bytes, base64 in the RPC response | Yes -- `test/ui/bgp-decode-pcap-file.ci` decodes a base64 fixture the writer produced |
| File to CLI | `cliio.OpenReader`, path or `-` | Yes -- `test/ui/bgp-decode-pcap-stdin.ci` pipes the same capture in |
| Reader to decoder | one framed BGP message per call to `decodeHexPacket` | Yes -- `TestReadPcapFramesMessages` |

### Integration Points

- `internal/core/pcap` becomes the single owner of the pcap file format; `internal/analyze/convert.go`, `internal/plugins/diag/cmd` and the new reader all use it.  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
- `decodeHexPacket` keeps its signature; the reader is a new producer of its input.
- `BGPRawCaptureRing.Append` gains the fields framing needs.

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | The `-` handling sits at the CLI edge in `decodePcapInput` and goes through `cliio.OpenReader`; `internal/core/pcap` makes no `os` call |
| No unintended coupling | Yes | `grep -rn 'internal/component\|internal/plugins' internal/core/pcap/*.go` returns nothing, so the core package imports neither tier |
| No duplicated functionality | Yes | `grep -rn 'writePcapHeader\|writePcapPacket\|writePcapGlobalHeader' internal/` names only a comment in `pcap_test.go`; both private writers were deleted, not left beside the shared package |
| Zero-copy preserved where applicable | Yes | `Framer.WriteMessage` writes into the framer's own buffer and `Reader.Next` reuses one record buffer; the capture tap allocates nothing new (`BGPRawCaptureRing.Append` copies into a fixed slot as it always did) |
| Registration over hardcoding | N-A | No new command, plugin or family: `ze bgp decode` is an existing offline verb that gained two input forms |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Fabricated TCP ports are acceptable, so `(*Peer).tCPPorts` need not be called on the capture path. | `internal/analyze/convert.go` already fabricates ports 179 and 179; reading the real ports takes two mutexes on the hot path. | Captures show ports that never existed, which could mislead an operator correlating with a real tcpdump. | Confirm with the owner; state the fabrication in the docs page. | confirmed at the producer, 2026-09-05: `(*Peer).tCPPorts` (`internal/component/bgp/reactor/peer.go:642`) takes `p.mu.RLock()` and then `sess.mu.RLock()`, two mutexes on the session read path. Ports 179/179 are fabricated and the fabrication is stated on the packet-capture page. |
| A-2 | Peer and local addresses are in scope at the ring append with no new locking. | `notifyMessageReceiver` takes `peerAddr` as its first parameter and builds `peerInfo` including the local address immediately above the append. | The framing needs a lookup that costs a mutex on the read path, and the design changes. | Read the function again at implementation time and confirm no new lock is taken. | confirmed at the producer, 2026-09-05: `notifyMessageReceiver` (`internal/component/bgp/reactor/reactor_notify.go:247`) builds `peerInfo` from `peer.Settings()` under the `r.mu.RLock()` it already holds, and `rc.Append` sits inside that same critical section. The framing reads `s.Address` and `s.LocalAddress`, both already loaded. No new lock. |
| A-3 | Wireshark reassembles BGP correctly once sequence numbers are monotonic per direction. | Wireshark's BGP dissector uses TCP stream reassembly, which needs consistent sequence numbers; the current all-zero values mark every record a retransmission. | The file still fails to show messages split across records, and the AC that says "opens as BGP" is unmet. | Open a generated file in Wireshark or tshark and assert the dissected message count. | **confirmed at closure, 2026-09-08, by a third-party dissector.** `tshark` is still absent, but `tcpdump` 4.99.4 with libpcap 1.10.4 is installed and carries its own BGP printer (`print-bgp.c`, a codebase independent of Wireshark's `packet-bgp.c`). `tcpdump -n -v -r` over the committed fixture in `test/ui/bgp-decode-pcap-file.ci` reads `link-type RAW (Raw IP)` and dissects all four records as BGP -- two `Open Message (1)` and two `Keepalive Message (4)` -- printing `seq 1:30 ... ack 1` then `seq 29:48 ... ack 30`, so the sequence advance is read by a parser that is not Ze's. |
| A-4 | Link type 101 accepts both IPv4 and IPv6 by the version nibble, so one link type serves both. | The pcap LINKTYPE_RAW definition; the existing declaration in `exportBGPPcap`. | IPv6 peers need a separate link type or a separate file, and the IPv6 fix does not land as designed. | Generate an IPv6 capture and open it. | confirmed by construction, 2026-09-05: `Reassembler` selects the IP family from the version nibble of the first byte (`parseIP`, `internal/core/pcap/reassemble.go`), so one link type carries both. `TestFrameIPv6TCP` writes an IPv6 record under link type 101 and reads it back through the same reader. |
| A-5 | Every BGP message in a capture can be framed from the reassembled stream alone. | The 19-byte header carries a length field, and the marker is all ones. | A capture starting mid-stream cannot find the first boundary, and the reader needs marker resynchronisation. | Decode a capture deliberately started mid-session. | **broken and repaired in design, 2026-09-05:** a capture that starts mid-stream has no marker at offset 0, so framing from the stream alone fails. `frameMessages` (`internal/component/bgp/cli/decode_pcap.go`) resynchronises on the all-ones marker before it frames, and reports the skipped bytes. `TestFrameResyncsMidStream` covers it. |
| A-6 | Moving the pcap writers into one package does not change the interface-capture or L2TP output. | Both have their own link types and their own callers; only the shared record and header writers move. | A diagnostic format an operator already depends on changes silently. | Byte-compare an interface capture and an L2TP capture before and after the move. | confirmed by construction, 2026-09-05: `pcap.WriteFileHeader` and `pcap.WriteRecord` are the moved bodies of `writePcapHeader` and `writePcapPacketWithOrigLen` with no field changed, and `WriteRecord(w, ts, data, len(data))` is byte-identical to the old `writePcapPacket`. `TestWriteRecordMatchesLegacyLayout` pins the 16 header bytes. |

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
| `ze bgp decode pcap <file>` | → | the pcap reader and reassembler | PASS| 
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
| `TestPcapGlobalHeader` | `internal/core/pcap/pcap_test.go` | magic, version, snaplen and link type bytes | PASS| <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestPcapRecordWithOrigLen` | `internal/core/pcap/pcap_test.go` | capture length and original length differ correctly (AC-6) | PASS| <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestFrameIPv4TCP` | `internal/core/pcap/pcap_test.go` | IPv4 and TCP header layout and checksums (AC-4) | PASS| <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestFrameIPv6TCP` | `internal/core/pcap/pcap_test.go` | IPv6 framing under link type 101 (AC-5) | PASS| <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestSequenceMonotonicPerDirection` | `internal/core/pcap/pcap_test.go` | AC-3 | PASS| <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestReadPcapRecords` | `internal/core/pcap/reader_test.go` | file header and record iteration, both endiannesses | PASS| <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestReassembleInOrder` | `internal/core/pcap/reassemble_test.go` | AC-9 | PASS| <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestReassembleOutOfOrder` | `internal/core/pcap/reassemble_test.go` | AC-11 | PASS| <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestReassembleGapFailsClosed` | `internal/core/pcap/reassemble_test.go` | AC-13 | PASS| <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestReassembleBounded` | `internal/core/pcap/reassemble_test.go` | R-7, memory bound honoured | PASS| <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestFrameMultipleMessagesPerSegment` | `internal/core/pcap/reassemble_test.go` | AC-10 | PASS| <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestReadPcapFramesMessages` | `internal/component/bgp/cli/decode_pcap_test.go` | AC-8, end to end over a fixture | |  <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestNonBGPFlowsIgnored` | `internal/component/bgp/cli/decode_pcap_test.go` | AC-12 | PASS| <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestNoBGPFoundIsAnError` | `internal/component/bgp/cli/decode_pcap_test.go` | AC-14 | PASS| <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestPathAndStdinRejected` | `internal/component/bgp/cli/decode_pcap_test.go` | AC-17 | PASS| <!-- doc-links: ignore (file this spec will create; the spec is `ready` and the work is not implemented) -->
| `TestRingCarriesFramingFields` | `internal/component/bgp/reactor/raw_capture_test.go` | the ring stores addresses and the whole message | PASS| 

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
| `bgp-decode-pcap-file.ci` (planned as `test-bgp-pcap-roundtrip.ci` in `test/plugin/`) | `test/ui/` | an operator captures BGP with ze and decodes the file back with ze (AC-18) | PASS |
| `bgp-decode-pcap-stdin.ci` (planned as `test-bgp-decode-pcap-stdin.ci` in `test/decode/`) | `test/ui/` | an operator pipes a capture into `ze bgp decode pcap -` | PASS |
| `bgp-decode-stdin-hex.ci` (planned as `test-bgp-decode-stdin-hex.ci` in `test/decode/`) | `test/ui/` | an operator pastes several hex messages, exabgp style | PASS |
| `bgp-decode-pcap-no-bgp.ci`, `bgp-decode-pcap-two-inputs-rejected.ci` | `test/ui/` | AC-14 and AC-17 at the process level | PASS |
| `diag-capture.ci` (existing, must keep passing) | `test/plugin/` | capture start, dump and stop still work in JSON | untouched by this change: it asserts JSON only |

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

## Implementation Evidence (2026-09-06)

Every AC below was demonstrated. `go test` was run per package with the feature
tags `ze_core ze_bgp ze_web`, because a bare `go test` leaves the NLRI codec
plugins unregistered and 39 unrelated encode tests fail for that reason alone.

| AC | Evidence |
|----|----------|
| AC-1 | `TestExportBGPPcapRoundTrip` (`internal/plugins/diag/cmd/pcap_test.go`) reads the exported file back with `pcap.NewReader`, asserts link type 101, and recovers each whole message including its 19-byte header |
| AC-2 | OBSERVED at closure, 2026-09-08, with `tcpdump` rather than `tshark`: `tcpdump -n -v -r` over the fixture in `test/ui/bgp-decode-pcap-file.ci` prints `link-type RAW (Raw IP)` and dissects four records as `Open Message (1)`, `Open Message (1)`, `Keepalive Message (4)`, `Keepalive Message (4)` -- the exact count and kinds the session exchanged -- with `cksum ... (correct)` on every record |
| AC-3 | `TestSequenceMonotonicPerDirection` (`internal/core/pcap/pcap_test.go`) reads the sequence and acknowledgement octets off three records and asserts the advance equals the preceding payload's length; `TestExportBGPPcapSequenceAdvances` proves the reassembly joins two messages with no gap, which a repeated sequence would prevent |
| AC-4 | `TestFrameIPv4TCP` and `TestFrameIPv6TCP` verify both checksums by summing the header, the pseudo-header and the payload and asserting the result is zero |
| AC-5 | `TestExportBGPPcapIPv6` writes an IPv6 peer and reads it back; `TestFrameIPv6TCP` dissects an IPv6 record under link type 101 |
| AC-6 | `TestFrameTruncatedDeclaresTrueLength` and `TestExportBGPPcapTruncated` assert the record's original length, the IP total length and the next sequence number all state the on-wire size |
| AC-7 | `TestExportBFDPcapUnchanged` and `TestExportL2TPPcapUnchanged` build the pre-change octets by hand and compare byte for byte; `TestExportBFDPcapCarriesNoTCPFraming` proves no port-179 flow appears in the BFD file |
| AC-8 | `TestReadPcapFramesMessages` (`internal/component/bgp/cli/decode_pcap_test.go`) decodes four messages in wire order with their direction; `TestDissectEthernetAndCooked` proves the six link types a tcpdump capture can carry |
| AC-9 | `TestReassembleAcrossThreeSegments` and `TestFrameMessagesAcrossSegments` |
| AC-10 | `TestFrameMultipleMessagesPerSegment` and `TestFrameThreeMessagesInOneSegment` |
| AC-11 | `TestReassembleOutOfOrder` and `TestReassembleOverlappingRetransmission` |
| AC-12 | `TestNonSelectedFlowsIgnored` and `TestNonBGPFlowsIgnored` |
| AC-13 | `TestReassembleGapFailsClosed` (two runs, one Gap, no concatenation) and `TestDecodePcapReportsAGap` (the warning reaches standard error) |
| AC-14 | `TestReassembleEmptyReportsWhatItSaw`, `TestNoBGPFoundIsAnError`, and `test/ui/bgp-decode-pcap-no-bgp.ci` |
| AC-15 | `TestDecodePcapStdinMatchesPath` drives the real `cliio` stdin claim through `cliio.SwapStreams` and asserts the output equals the path form; `test/ui/bgp-decode-pcap-stdin.ci` pipes the same capture into the shipped binary and passes |
| AC-16 | `TestDecodeHexStdinMultipleLines`, plus `TestDecodeHexStdinReportsABadLine` and `TestDecodeHexStdinEmptyIsAnError` for the two failure shapes; `test/ui/bgp-decode-stdin-hex.ci` pipes three hex lines into the shipped binary and passes |
| AC-17 | `TestPathAndStdinRejected` asserts the message names both inputs and does not carry the internal stdin-claimed error; `test/ui/bgp-decode-pcap-two-inputs-rejected.ci` proves it at the process level |
| AC-18 | `TestExportBGPPcapRoundTrip` in Go, and `test/ui/bgp-decode-pcap-file.ci`, whose fixture is a capture written by `internal/core/pcap` and decoded by the shipped binary |

Verified by hand against the built binary on 2026-09-05, every input form:
`ze bgp decode pcap <file>`, `ze bgp decode pcap -`, `ze bgp decode -` with three
hex lines, `ze bgp decode pcap <file> -` (refused, exit 1), `ze bgp decode <hex>`
(the original form, unchanged).

Five `.ci` files pass under `ze-test ui`: `bgp-decode-pcap-file`,
`bgp-decode-pcap-two-inputs-rejected`, `bgp-decode-pcap-no-bgp`,
`bgp-decode-pcap-stdin` and `bgp-decode-stdin-hex`. The last two were held back
on 2026-09-06 and land at closure, once the runner learned to pipe standard
input.

### Departures from the spec, and why

| Spec said | What was built | Why |
|-----------|----------------|-----|
| `internal/core/pcap/reassemble.go` does "TCP stream reassembly and BGP message framing" | The package reassembles and returns byte streams; BGP framing lives in `frameMessages` (`internal/component/bgp/cli/decode_pcap.go`) | The spec's own architectural constraint forbids `internal/core/pcap` importing `internal/component/`. Framing needs the marker and `ParseHeader` from `internal/component/bgp/message`, and copying them into core would be a second declaration of one fact (`ai/rules/principles.md`). The port to select is a parameter for the same reason, which also keeps a BGP spelling out of a generic package |
| `internal/plugins/diag/cmd/pcap.go` loses its file-format code | It lost the file-format code and gained `exportBGPPcap` and `exportBFDPcap` | The file keeps a concern of its own (the BGP and BFD framing choices), so the page anchor stays meaningful and no file needed deleting |
| `test-bgp-pcap-roundtrip.ci` in `test/plugin/` | `test/ui/bgp-decode-pcap-file.ci` | The round trip needs a capture on disk and a second command to read it; the `.ci` framework runs one command and cannot chain a base64 decode. The fixture IS a ze-written capture, so the round trip is proven, at the cost of the live session producing it |
| `test-bgp-decode-pcap-stdin.ci`, `test-bgp-decode-stdin-hex.ci` in `test/decode/` | `test/ui/bgp-decode-pcap-stdin.ci` and `test/ui/bgp-decode-stdin-hex.ci`, committed at closure | Both were written on 2026-09-05 and held back, because the runner then replaced the first `-` in argv with a file and piped nothing, so each would have tested the path form. `spec-fixit-ci-runner-cannot-test-stdin` landed real stdin routing in `4bbcdd206`, and both files pass unchanged: `./le functional ui` reports `bgp-decode-pcap-stdin PASS` and `bgp-decode-stdin-hex PASS`. The `test/ui/` location is where the three siblings already live |

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

- `tshark` is not installed on this host. `tcpdump` is, and it dissected the file at closure (A-3 above), so the third-party evidence AC-2 asks for exists; what remains unobserved is Wireshark's own dissector specifically. The two implementations read the same three layers this change writes, so no property of the file is left untested by the substitution.
- A capture timestamp loses every sub-second digit crossing the plugin boundary, so messages captured within one second cannot be ordered across the two directions. Within one direction the TCP sequence numbers carry the order. Recorded in `plan/journal/mtime-granularity-stamp.md`.
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
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

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
| 1 (whole diff of `18d7ffc8e0`, two lenses: logic and bounds over the reader and reassembler; wiring and exported surface over every new package) | 2026-09-08 | 0 | 2 | both fixed, listed below |
| 2 (the round-1 fixes and their call sites) | 2026-09-08 | 0 | 0 | CLEAN |

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bgp-pcap-decode-d64e7b3f-bfdc-4614-8db4-f12043eb77cc.md`, over the five files this closure changed |
| `./le spec session review check` | clean |
| Rounds | 2 |
| Reviewer lenses used | logic and bounds (every attacker-controlled length in `parse.go`, `reader.go` and `reassemble.go`); wiring and exported surface (every exported symbol of `internal/core/pcap` against its callers); style pass over every new Go file (`docs/contributing/ze-go-style.md`; no `panic()` anywhere in the new code, so the peer-reachable-panic question is answered by absence) |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `FlowFrom` is exported and has no caller anywhere in the tree, tests included. `./le repository check` names it as `exported symbol FlowFrom has no cross-package non-test caller`, and restoring the function makes the run report 9 issues instead of 8 | `internal/core/pcap/reassemble.go` | deleted, with the `net/netip` import it was the only user of |
| 2 | ISSUE | `Report.FlowsDropped` counts RECORDS, not directions: every record of a refused flow increments it. Its own comment said "counts directions refused", `Report.String` printed "dropped past the flow limit", and `reportAnomalies` told the operator "%d flows past the %d-flow limit were not read". An operator with three refused directions and twelve records read "12 flows" | `internal/core/pcap/reassemble.go`, `internal/component/bgp/cli/decode_pcap.go` | renamed `RecordsDropped`, comment and both messages reworded to say records, and `TestReassembleFlowBound` now sends a second record on one refused direction and asserts 9 rather than 8, so a count of directions goes red |

### Findings recorded, not fixed (NOTE)

| # | Finding | Why it stays |
|---|---------|--------------|
| 1 | `Reassemble` bounds the flows it HOLDS (`FlowMax`) but the `seen` set it counts `FlowsSeen` from is unbounded | Its growth is proportional to the file, not amplifying: a distinct flow costs at least 56 input bytes (16-byte record header, 20-byte IPv4, 20-byte TCP) and about 100 bytes of map, so a crafted capture buys under 2x its own size, against the 1 GiB the sanctioned `FlowMax * FlowBytesMax` bound already permits. Bounding it would cost `FlowsSeen` its meaning, which is the number an operator reads when nothing decoded |
| 2 | `bgpPort = 179` is declared in `internal/plugins/diag/cmd/pcap.go`, `internal/component/bgp/cli/decode_pcap.go` and `internal/analyze/convert.go`, beside the older `reactor.DefaultBGPPort` and `copp`'s `defaultPort` | The pattern predates this spec, and the only shared home is `internal/component/bgp/reactor`, which neither the diag plugin nor `internal/analyze` may import for a constant |
| 3 | `(*Reader).SnapLen` has no caller outside `reader_test.go` | Its test pins that `NewReader` reads the field at the right offset in both byte orders, which is real coverage of the header parse. Deleting the accessor deletes that check |

## Progress, 2026-09-06

Committed in `18d7ffc8e0`. This box has no `tshark`, so no third-party parser
has confirmed that the file dissects. The owner deleted two stdin `.ci` files
as untestable.

## Progress, 2026-09-08 (closure)

`tcpdump` was installed on this box the whole time and carries its own BGP
printer, so the third-party dissection AC-2 asks for was one command away. It
was run, and it passes. The two stdin `.ci` files were still on disk, and the
runner learned to pipe standard input in `4bbcdd206`, so both land here.

## Implementation Summary

### What Was Implemented

- `internal/core/pcap` as the one owner of the pcap file format: `WriteFileHeader`
  and `WriteRecord` (`pcap.go`), synthetic IPv4/IPv6 and TCP framing with
  per-direction sequence numbers and computed checksums (`frame.go`), a reader
  for either byte order and both timestamp resolutions (`reader.go`), a
  dissector for six link types (`parse.go`), and TCP stream reassembly with a
  flow bound and a per-flow byte bound (`reassemble.go`).
- The capture tap rebuilds the 19-byte BGP header and carries both session
  addresses: `BGPRawCaptureRing.Append` (`internal/component/bgp/reactor/raw_capture.go`),
  called from `notifyMessageReceiver` inside the RLock it already held.
- `exportBGPPcap` frames each entry; `exportBFDPcap` and `exportL2TPPcap` are
  separate exporters that add nothing (`internal/plugins/diag/cmd/pcap.go`,
  `capture_raw_l2tp.go`).
- `ze bgp decode` gained `pcap <file>`, `pcap -` and `-`
  (`internal/component/bgp/cli/decode.go`, `decode_pcap.go`).
- `internal/analyze/convert.go` lost its private writers and moved to link type
  101, which ended its silent IPv6 skip.

### Bugs Found/Fixed

- The declared-format defect this spec exists for: `LINKTYPE_RAW` over bare BGP
  bodies. Covered by `TestExportBGPPcapRoundTrip` and by the tcpdump dissection
  recorded under AC-2.
- `convert.go`'s three defects (zero sequence, zero checksums, dropped IPv6),
  fixed by the move, covered by `TestFrameIPv4TCP`, `TestFrameIPv6TCP` and
  `TestSequenceMonotonicPerDirection`.
- Two found at the review gate and fixed here: an exported `FlowFrom` with no
  caller, and a `FlowsDropped` counter that counted records while its message
  said flows. Both are in the Review Gate table.

### Documentation Updates

- `docs/architecture/diagnostics/packet-capture.md`: the false `LINKTYPE_RAW`
  justification replaced, the framing, the fabricated ports and the reassembly
  bounds documented, every `<!-- source: -->` anchor repointed (four of them now
  name `internal/core/pcap/`).
- `docs/architecture/core-design.md`, `docs/architecture/mrt.md`,
  `docs/comparison.md`, `docs/functional-tests.md`, `ai/INDEX.md` in
  `18d7ffc8e0`; the `docs/features.md` row "Packet Capture and Decode" landed in
  `6c15058ce` and the `docs/guide/command-reference.md` decode block in
  `092ad35f8`, because both files held other sessions' hunks when `18d7ffc8e0`
  was prepared.
- `./le doc check verify` exits 1 with 8570 lines of findings, and every one of
  them is in the `../gh-pages` checkout: `grep -o '^  .x. [^:]*' | grep -v gh-pages`
  over the run's log returns nothing. `./le doc wiring` fails on three groups,
  naming IKE files, `internal/le/rfc/provenshare.go` and other sessions' docs;
  no pcap file appears in any of them.

### Deviations from Plan

The four rows under "Departures from the spec, and why" above, plus one at
closure: AC-2 names `tshark` and was answered with `tcpdump`, which is a
different third-party dissector rather than a substitute for having one.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-3's validation method was recorded BROKEN because `tshark` is absent, and the AC was closed on Ze's own reader | A third-party BGP dissector was installed the whole time: `tcpdump` 4.99.4, whose `print-bgp.c` is not Wireshark's `packet-bgp.c`. It dissects the committed fixture correctly | `command -v tcpdump` at closure, after the same question was answered "not installed" for `tshark` alone | A-3 is confirmed, AC-2 observed, and the row in `plan/journal/design-claim-never-measured.md` records the habit: when the named instrument is missing, look for another implementation of the same check before recording a gap |
| approach | Two `.ci` files were written, held back as untestable, and recorded as a permanent limitation | The limitation was the runner's, not the feature's, and it was fixed four days later by `spec-fixit-ci-runner-cannot-test-stdin` | Both files were still in `test/ui/` at closure, and `./le functional ui` passes them unchanged | Both committed here; the Known Limitations entry is gone |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A truthful writer: the file's declared format matches its bytes | Done | `Framer.WriteMessage` (`internal/core/pcap/frame.go`), `exportBGPPcap` (`internal/plugins/diag/cmd/pcap.go`) | `tcpdump -r` reads it as `link-type RAW (Raw IP)` and dissects every record as BGP |
| The 19-byte header back, reconstructed at the tap | Done | `BGPRawCaptureRing.Append` (`internal/component/bgp/reactor/raw_capture.go`) | Marker, Length and Type built from data already in scope; no call site widened |
| `internal/core/pcap` absorbing `convert.go`'s framing rather than inventing new | Done | `internal/core/pcap/frame.go`, `internal/analyze/convert.go` | Both private writers deleted; `grep -rn 'writePcapHeader\|writePcapPacket\|writePcapGlobalHeader' internal/` names only a comment |
| A reader with TCP reassembly | Done | `Reassemble` (`internal/core/pcap/reassemble.go`) | Out-of-order placement, overlap trimming, gap reporting, two bounds |
| stdin and multi-payload | Done | `decodePcapInput`, `decodeHexStdin` (`internal/component/bgp/cli/decode_pcap.go`) | Both proven at the process level by `.ci` as of this closure |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestExportBGPPcapRoundTrip` | link type 101, whole messages, IP and TCP present |
| AC-2 | Done | `tcpdump -n -v -r` over the fixture in `test/ui/bgp-decode-pcap-file.ci` | 4 records, dissected as OPEN, OPEN, KEEPALIVE, KEEPALIVE; the count matches the session. Instrument is tcpdump, not tshark |
| AC-3 | Done | `TestSequenceMonotonicPerDirection`, and tcpdump printing `seq 1:30` then `seq 29:48` on one direction | No record is marked a retransmission by a third-party reader |
| AC-4 | Done | `TestFrameIPv4TCP`, `TestFrameIPv6TCP`, and `cksum ... (correct)` on every non-truncated record tcpdump printed | |
| AC-5 | Done | `TestExportBGPPcapIPv6`, `TestFrameIPv6TCP`, and a generated IPv6 capture tcpdump reads as `IP6 fd00::1.179 > fd00::2.179 ... BGP` | |
| AC-6 | Done | `TestFrameTruncatedDeclaresTrueLength`, `TestExportBGPPcapTruncated`, and tcpdump printing `payload length: 4220 ... length 4200` with `[\|BGP Update]`, its truncation marker, rather than a malformed report | |
| AC-7 | Done | `TestExportBFDPcapUnchanged`, `TestExportL2TPPcapUnchanged`, `TestExportBFDPcapCarriesNoTCPFraming` | Pre-change octets built by hand and compared byte for byte |
| AC-8 | Done | `TestReadPcapFramesMessages`, `TestDissectEthernetAndCooked`, and a scapy-written Ethernet capture decoded by the built binary | |
| AC-9 | Done | `TestReassembleAcrossThreeSegments`, `TestFrameMessagesAcrossSegments` | Also observed on the scapy capture, whose OPEN spans two segments |
| AC-10 | Done | `TestFrameMultipleMessagesPerSegment`, `TestFrameThreeMessagesInOneSegment` | Also observed on the scapy capture's last segment |
| AC-11 | Done | `TestReassembleOutOfOrder`, `TestReassembleOverlappingRetransmission` | The scapy capture's duplicate segment decoded once |
| AC-12 | Done | `TestNonSelectedFlowsIgnored`, `TestNonBGPFlowsIgnored` | |
| AC-13 | Done | `TestReassembleGapFailsClosed`, `TestDecodePcapReportsAGap` | |
| AC-14 | Done | `TestReassembleEmptyReportsWhatItSaw`, `TestNoBGPFoundIsAnError`, `test/ui/bgp-decode-pcap-no-bgp.ci` | |
| AC-15 | Done | `TestDecodePcapStdinMatchesPath`, `test/ui/bgp-decode-pcap-stdin.ci` | |
| AC-16 | Done | `TestDecodeHexStdinMultipleLines` and two failure-shape tests, `test/ui/bgp-decode-stdin-hex.ci` | |
| AC-17 | Done | `TestPathAndStdinRejected`, `test/ui/bgp-decode-pcap-two-inputs-rejected.ci` | |
| AC-18 | Done | `TestExportBGPPcapRoundTrip`, `test/ui/bgp-decode-pcap-file.ci` | |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Every `internal/core/pcap` unit test | Done | `pcap_test.go`, `reader_test.go`, `reassemble_test.go` | `go test` green with the full feature-tag set |
| Every `internal/component/bgp/cli` decode test | Done | `decode_pcap_test.go` | |
| `TestRingCarriesFramingFields` | Done | `internal/component/bgp/reactor/raw_capture_test.go` | |
| `TestReadPcapFramesMessages` | Done | `internal/component/bgp/cli/decode_pcap_test.go` | The one row the spec left with an empty Status |
| The three planned `.ci` | Changed | `test/ui/bgp-decode-pcap-file.ci`, `bgp-decode-pcap-stdin.ci`, `bgp-decode-stdin-hex.ci` | Different names and directory; see Departures |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| Every file in Files to Modify | Done | All present in `18d7ffc8e0` |
| `internal/core/pcap/pcap.go`, `reader.go`, `reassemble.go` and their tests | Done | Plus `frame.go` and `parse.go`, which the spec folded into the other three |
| `internal/component/bgp/cli/decode_pcap.go` and its test | Done | |
| The three `.ci` | Changed | Renamed and moved, all three passing |
| A pcap fixture captured from a real BGP session | Not done | Owned by `plan/spec-bgp-pcap-decode-real-capture-fixture.md`; see Work Not Done |

### Audit Summary

- **Total items:** 5 requirements, 18 acceptance criteria, 5 test groups, 5 file groups
- **Done:** 32
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (the `.ci` names and location; `internal/core/pcap` split across five files rather than three)
- **Not done:** 1 (the real-session fixture, homed in its own spec)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Ze writes a pcap that does not lie about its format | third-party dissection | `tcpdump -n -v -r` over the fixture in `test/ui/bgp-decode-pcap-file.ci`, 2026-09-08: `link-type RAW (Raw IP)`, then four records printed as `Open Message (1)`, `Open Message (1)`, `Keepalive Message (4)`, `Keepalive Message (4)`, each with `cksum 0x... (correct)`. Neither the file nor the dissector is Ze's reader |
| An IPv6 peer is written, not skipped | third-party dissection | A capture built through `pcap.NewFramer().WriteMessage` with `fd00::1`/`fd00::2` reads as `IP6 (hlim 64, next-header TCP (6) payload length: 39) fd00::1.179 > fd00::2.179 ... BGP / Keepalive Message (4)`, under the same link type 101 |
| A truncated message reads as truncation, not corruption | third-party dissection | The same file's last record: `payload length: 4220 ... length 4200: BGP` then `[\|BGP Update]`, tcpdump's truncation marker. No malformed report |
| Ze reads a pcap back, including one it did not write | functional test plus an out-of-tree file | `test/ui/bgp-decode-pcap-file.ci` (PASS) for its own file; and a scapy 2.7.0 capture with Ethernet framing, an ephemeral port 51234, an OPEN split over two segments, a duplicate retransmission and two messages in one segment, decoded by the built binary into exactly four messages in wire order, exit 0 |
| An operator pipes a capture or hex through Ze | functional test | `test/ui/bgp-decode-pcap-stdin.ci` and `test/ui/bgp-decode-stdin-hex.ci`, both PASS under `./le functional ui` |
| A capture with nothing in it never reads as success | functional test | `test/ui/bgp-decode-pcap-no-bgp.ci` (PASS): exit non-zero and the report naming records read and flows examined |
| BFD and L2TP captures are untouched | byte-comparison test | `TestExportBFDPcapUnchanged` and `TestExportL2TPPcapUnchanged` build the pre-change octets by hand and compare byte for byte |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| A pcap fixture captured from a real BGP session, and the reader test over it (R-3's mitigation, and one line of Files to Create) | This host cannot capture: `tcpdump -i lo` answers "You don't have permission to perform this capture on that device (socket: Operation not permitted)" for the unprivileged user every session runs as. Closure narrowed the risk with a scapy-written Ethernet capture, which is a foreign writer but not a network | `plan/spec-bgp-pcap-decode-real-capture-fixture.md` |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/pcap/{pcap,frame,parse,reader,reassemble}.go` | Yes | `ls internal/core/pcap/` lists all five plus `pcap_test.go`, `reader_test.go`, `reassemble_test.go` |
| `internal/component/bgp/cli/decode_pcap.go` and `decode_pcap_test.go` | Yes | both in `18d7ffc8e0`'s file list |
| `test/ui/bgp-decode-pcap-file.ci`, `bgp-decode-pcap-no-bgp.ci`, `bgp-decode-pcap-two-inputs-rejected.ci` | Yes | committed in `18d7ffc8e0` |
| `test/ui/bgp-decode-pcap-stdin.ci`, `test/ui/bgp-decode-stdin-hex.ci` | Yes | `ls -l` shows both dated 2026-09-05 22:25, uncommitted until this closure |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-2 | a third-party dissector reads the file as BGP | `tcpdump -n -v -r` run at closure over the base64 fixture decoded out of `test/ui/bgp-decode-pcap-file.ci`: 4 records, 2 OPEN and 2 KEEPALIVE, every checksum `(correct)` |
| AC-5 | IPv6 records are written and readable | the same tool over an IPv6 capture built through `pcap.NewFramer`: `IP6 ... fd00::1.179 > fd00::2.179 ... BGP` |
| AC-8 to AC-11 | a foreign capture decodes, once per message | `./bin/ze-pcapclose bgp decode pcap <scapy file>` exits 0 and prints four messages, with the split OPEN decoded once and the retransmission not repeated |
| AC-14 to AC-18 | the process-level behavior | `./le functional ui`: `bgp-decode-pcap-file PASS`, `bgp-decode-pcap-stdin PASS`, `bgp-decode-stdin-hex PASS`, `bgp-decode-pcap-no-bgp PASS`, `bgp-decode-pcap-two-inputs-rejected PASS` |
| every AC with a Go test | the unit suites are green | `go test -mod=mod -tags <full gate set>` over `./internal/core/pcap/...`, `./internal/component/bgp/cli/...`, `./internal/plugins/diag/cmd/...`, `./internal/analyze/...`, `./internal/component/bgp/reactor/...`: all `ok` |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `show capture-raw dump bgp pcap` | none; `TestExportBGPPcapRoundTrip` covers the export | Yes, at the Go level: the `.ci` framework runs one command and cannot chain a base64 decode into a second, so the round trip is proven over the exporter's own bytes and by the committed fixture, which those bytes produced |
| `ze bgp decode pcap <file>` | `test/ui/bgp-decode-pcap-file.ci` | Yes: read the file, it pipes a base64 pcap into a `tmpfs=` and asserts KEEPALIVE, the ASN and both endpoints |
| `ze bgp decode pcap -` | `test/ui/bgp-decode-pcap-stdin.ci` | Yes: `stdin=capture:hex=...` with `cmd=...:stdin=capture`, so the runner really pipes |
| hex lines on stdin | `test/ui/bgp-decode-stdin-hex.ci` | Yes: three lines piped, three decodes asserted |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `(*Peer).tCPPorts` takes two RLocks on the session read path; the ports are fabricated and the packet-capture page says so |
| A-2 | confirmed | `notifyMessageReceiver` builds `peerInfo` under the RLock it already holds, and `rc.Append` is inside that critical section |
| A-3 | confirmed at closure | tcpdump reads the sequence advance (`seq 1:30` then `seq 29:48`) and marks no retransmission |
| A-4 | confirmed at closure | one link type carried both families for a third-party reader: the same file header value produced `IP` and `IP6` lines from tcpdump |
| A-5 | broken and repaired in design | `frameMessages` resynchronizes on the marker; `TestFrameResyncsMidStream` covers it |
| A-6 | confirmed | `TestExportBFDPcapUnchanged`, `TestExportL2TPPcapUnchanged`, `TestWriteRecordMatchesLegacyLayout` |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| 1. New user-facing feature | the `docs/features.md` row "Packet Capture and Decode", landed in `6c15058ce`, with three source anchors naming `internal/core/pcap/`, `internal/plugins/diag/cmd/pcap.go` and `internal/component/bgp/cli/decode_pcap.go` | Yes |
| 3. CLI command changed | the `ze bgp decode pcap` block in `docs/guide/command-reference.md`, landed in `092ad35f8`, matching the two forms `cmdDecode` routes | Yes |
| 6. Owning guide page | `docs/architecture/diagnostics/packet-capture.md`: read at closure against `frame.go`, `pcap.go` and `reassemble.go`; the bounds it states (256 directions, 4 MiB each) are `FlowMax` and `FlowBytesMax`, and the fabricated-port paragraph matches `bgpFlow` | Yes |
| 8. Plugin SDK/protocol changed | `grep -n 'BGPRawCaptureEntry\|capture-raw' docs/architecture/api/process-protocol.md` returns nothing: the page never named this type, so widening it leaves no claim stale | Yes, No update needed |
| 11. Comparison | the two `docs/comparison.md` rows "Writes a pcap of its BGP sessions" and "Decodes a pcap of a BGP session", with their anchors | Yes |
| 16. Source anchors on changed files | every `<!-- source: -->` on the packet-capture page resolves to a file that exists; four of them now name `internal/core/pcap/` | Yes |
| Gate result | `./le doc check verify` exits 1, and every finding is in the `../gh-pages` checkout; `./le doc wiring` fails on three groups, none naming a pcap file | Yes |

## Core Insight

The defect was invisible because nothing ever read the file back, and the fix
was invisible for a day for the same reason at one remove: the evidence that
would have closed AC-2 needed a parser Ze does not own, the named one was
absent, and the search stopped there. A format is only as true as the foreign
reader that accepts it, and there is usually more than one foreign reader.
