# Packet Capture

Ze captures two different things: control-plane messages it already decodes
(BGP, L2TP), and live frames off an interface. Both write pcap with a
stdlib-only writer, so an appliance needs no tcpdump and no libpcap.

<!-- source: internal/plugins/diag/cmd/capture.go -- decoded capture display -->
<!-- source: internal/plugins/diag/cmd/capture_raw.go -- raw capture activation and export -->
<!-- source: internal/plugins/diag/cmd/pcap.go -- pcap writer -->
<!-- source: internal/plugins/diag/cmd/capture_common.go -- shared constants and helpers -->

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
written with the standard library alone. The control-plane rings hold no link
layer, so they use `LINKTYPE_RAW` (101).

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
