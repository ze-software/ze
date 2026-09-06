# Packet Capture

Ze captures two different things: control-plane messages it already decodes
(BGP, L2TP), and live frames off an interface. Both write pcap with a
stdlib-only writer, so an appliance needs no tcpdump and no libpcap.

<!-- source: internal/plugins/diag/cmd/capture.go -- decoded capture display -->
<!-- source: internal/plugins/diag/cmd/capture_raw.go -- raw capture activation and export -->
<!-- source: internal/plugins/diag/cmd/pcap.go -- BGP and BFD pcap framing -->
<!-- source: internal/plugins/diag/cmd/capture_common.go -- shared constants and helpers -->
<!-- source: internal/core/pcap/pcap.go -- the pcap file format, one owner -->
<!-- source: internal/core/pcap/frame.go -- synthetic IP and TCP headers -->
<!-- source: internal/core/pcap/reader.go -- reading a capture back -->
<!-- source: internal/core/pcap/reassemble.go -- TCP stream reassembly -->

## Two tiers

| Tier | Storage | Cost when off |
|------|---------|---------------|
| Decoded ring | numeric value types only, appended with no allocation | always on, bounded |
| Raw byte ring | fixed-size array slots, 1500 B for L2TP and 4096 B for BGP | nil pointer, opt-in through a debug command |

Raw capture is opt-in because the byte copy is the expensive half. Activation
uses `atomic.Pointer` with `CompareAndSwap`, so an RPC handler goroutine can
enable a ring that a reactor goroutine reads on the hot path.

<!-- source: internal/component/bgp/reactor/capture.go -- BGP decoded ring -->
<!-- source: internal/component/bgp/reactor/raw_capture.go -- BGP raw ring -->
<!-- source: internal/component/l2tp/capture.go -- L2TP control packet ring -->
<!-- source: internal/component/l2tp/raw_capture.go -- L2TP raw ring -->

The pcap writer is a 24-byte global header plus a 16-byte per-packet header,
written with the standard library alone. `internal/core/pcap` owns that format
and every capture in Ze goes through it: the BGP export, the BFD export, the
L2TP export and the live interface capture.

## The BGP capture is framed as IP and TCP

`LINKTYPE_RAW` (101) means each record begins with an IP header, and the version
nibble of its first byte says whether that header is IPv4 or IPv6. One link type
therefore carries both families, which is why an IPv6 peer needs no second file
and no second link type.

The BGP export gives each record a synthetic IPv4 or IPv6 header, a TCP header
and one whole BGP message, so the declaration is true and Wireshark dissects the
file as BGP. Three properties make the file readable:

- **The 19-byte BGP header travels with the body.** The capture tap receives a
  header-stripped body and the message type as a separate argument, so
  `BGPRawCaptureRing.Append` rebuilds the header there. RFC 4271 Section 4.1
  fixes the marker at all ones, the Length field counts the header, and the Type
  is the argument, so the reconstruction is byte-exact.
- **Sequence numbers advance by the payload.** Each direction keeps its own
  counter, and each record's acknowledgement names every byte the reverse
  direction has sent. Without this a reader marks every record a retransmission
  and reassembles nothing.
- **A truncated message says how long it really was.** The ring slot is 4096
  bytes and RFC 8654 allows a message of 65535, so a long UPDATE is captured
  short. The record's original length and the IP total length both state the
  on-wire size, so a reader shows truncation instead of reporting a malformed
  packet.

**The TCP ports are fabricated: both ends read 179.** A BGP session has one
ephemeral port, and reading the real pair costs two mutexes on the session read
path (`(*Peer).tCPPorts`). A ze-written capture is a readable reconstruction of
the session, not a packet-level record, so it cannot be correlated byte for byte
with a simultaneous tcpdump. The addresses are real: they come from the peer
settings the tap already holds.

A capture holds the peer's routing data. It holds no local secret, because a
TCP-MD5 key never appears on the wire and the capture records only what crossed
it.

## The BFD and L2TP captures carry no framing

BFD and L2TP are UDP payloads. Framing them as TCP on port 179 would turn a
correct capture into a malformed one, so each has its own exporter and each
writes its records with no headers added. `exportBFDPcap` and `exportL2TPPcap`
are byte-identical to what the shared exporter wrote before the BGP one gained
framing, and a test in each package pins that.

## Reading a capture back

`ze bgp decode pcap <file>` reads a capture and decodes every BGP message in it.
The file may be one ze wrote or one from somebody else's tcpdump: the reader
takes either byte order, either timestamp resolution, and the Ethernet, raw IP
and Linux cooked link types.

Reassembly is what the reader owes. It places out-of-order segments by sequence
number, drops a retransmission that repeats bytes already held, and REPORTS a
hole rather than reading across it. A capture started mid-session has no message
boundary at offset zero, so framing resynchronizes on the all-ones marker.

Both bounds a crafted file could push against are fixed: 256 TCP directions and
4 MiB for each of them. Reaching either is reported. A capture holding no BGP
exits non-zero and names what it examined, because an operator reads empty
output as "there is no BGP here".

## Live interface capture

`show capture interface` reads frames from AF_PACKET and compiles a tcpdump
filter expression to BPF.

<!-- source: internal/plugins/diag/cmd/capture_interface.go -- portable types, argument parser, text formatter -->
<!-- source: internal/plugins/diag/cmd/capture_interface_linux.go -- AF_PACKET reader and BPF compilation -->
<!-- source: internal/plugins/diag/cmd/capture_interface_other.go -- non-Linux stub -->

- `mdlayher/packet` with `packetcap/go-pcap/filter` was chosen over gopacket
  and pcapgo. The pair is lighter, needs no cgo, and composes with the existing
  pcap helpers. gopacket brings a large dependency tree for no gain here. Any
  later AF_PACKET use reuses `mdlayher/packet`.
- The output is both base64 pcap and one line of text per packet. An AI agent
  and an SSH session both need to read a capture with no external tool. The
  text format `TIMESTAMP PROTO SRC:PORT -> DST:PORT FLAGS LEN HEX` is a
  contract: an MCP tool schema depends on it.
- The link type is Ethernet (`DLT_EN10MB`, 1), not raw IP. AF_PACKET delivers
  the full frame including the 14-byte Ethernet header, so `DLT_RAW` would
  produce a corrupt file.
- One capture per interface at a time, guarded by a `sync.Map`. The load
  pattern is rare writes and lock-free reads, and concurrent captures contend
  on the AF_PACKET socket.
- Portable types, the argument parser and the text formatter live in a file
  with no build tag. Only AF_PACKET and the BPF compilation are Linux-tagged,
  so the tests run on a developer machine. A test file with no build tag that
  names a type from a `_linux.go` file fails to compile elsewhere.

## The filter compiler is a three-step API

`go-pcap/filter` has no single `Compile(expr)` entry point. The order is
`NewExpression(s).Compile()`, which returns a `Filter`, then `Filter.Compile()`,
which returns `[]bpf.Instruction`, then `bpf.Assemble()`, which returns
`[]bpf.RawInstruction`. The intermediate types differ at each step. The
expression parser does not cover every tcpdump form, and an unsupported filter
fails at compile time with a named error.
