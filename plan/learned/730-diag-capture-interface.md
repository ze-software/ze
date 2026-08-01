# 730 -- diag-capture-interface

## Context

Gokrazy appliances lack tcpdump and other packet capture tools. Network-level debugging required SSH-ing into the appliance and using workarounds. The goal was a pure-Go live packet capture command accessible from the CLI, MCP, and web API, producing pcap or human-readable text output.

## Decisions

- Chose `mdlayher/packet` + `packetcap/go-pcap/filter` over gopacket/pcapgo because the combination is lighter and composable with no cgo. gopacket pulls a large dependency tree for no benefit given the existing pcap.go helpers.
- Chose dual output (base64 pcap + one-line text) over pcap-only because AI agents and SSH sessions need human-readable output without external tools.
- Chose `sync.Map` per-interface concurrency guard over a mutex-protected map because the load pattern (rare writes, lock-free reads) fits sync.Map's design. One active capture per interface prevents AF_PACKET socket contention.
- Chose Ethernet link type (DLT_EN10MB=1) over raw IP (DLT_RAW=101) because AF_PACKET delivers full Ethernet frames including the 14-byte header. Using DLT_RAW would produce corrupt pcap files.
- Split portable code (types, arg parser, text formatter) into `capture_interface.go` (no build tag) over keeping everything in `_linux.go` so tests run on macOS during development. Only AF_PACKET and BPF compilation stay in the Linux file.

## Consequences

- Third-party deps `mdlayher/packet` and `packetcap/go-pcap` are now in the vendor tree. Any future AF_PACKET use should reuse `mdlayher/packet` rather than adding another raw socket library.
- The text output format (`TIMESTAMP PROTO SRC:PORT -> DST:PORT FLAGS LEN HEX`) is now a contract for AI agent consumption. Changing the format requires coordinating with MCP tool schemas.
- The BPF filter compiler (`go-pcap/filter`) uses a tcpdump expression parser that may not support all tcpdump syntax. Complex filters may fail at compilation time with a clear error.

## Gotchas

- `go-pcap/filter` API is not a simple `Compile(expr)` function. The flow is `NewExpression(s).Compile()` (returns `Filter`), then `Filter.Compile()` (returns `[]bpf.Instruction`), then `bpf.Assemble()` (returns `[]bpf.RawInstruction`). The intermediate types are different.
- The `block-sprintf-new.sh` hook blocks `strconv.FormatUint` calls. Use `textbuf.Buffer.Hex()` for hex-formatting EtherType values (convert uint16 to [2]byte first).
- `unparam` linter flags `truncateBytes(data, maxBytes)` when maxBytes is always the same constant. Extract the constant and make the function single-argument.
- Test files without build tags that reference types from `_linux.go` files fail on macOS. Portable types must live in a non-tagged file.

## Files

- `internal/plugins/diag/cmd/capture_interface.go` -- portable types, arg parser, text formatter
- `internal/plugins/diag/cmd/capture_interface_linux.go` -- AF_PACKET handler, BPF compilation
- `internal/plugins/diag/cmd/capture_interface_other.go` -- platform stub
- `internal/component/cmd/show/capture_interface_test.go` -- portable unit tests
- `internal/component/cmd/show/capture_interface_linux_test.go` -- BPF filter tests
- `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` -- YANG tree update
- `test/plugin/show-capture-interface.ci` -- functional test
- `docs/features.md` -- feature table update
- `docs/guide/command-reference.md` -- command reference
- `docs/architecture/api/commands.md` -- RPC documentation
- `docs/guide/production-diagnostics.md` -- troubleshooting guide
