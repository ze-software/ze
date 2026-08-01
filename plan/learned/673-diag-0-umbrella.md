# 673 -- diag-0-umbrella: Production Diagnostics via MCP

## What was built

13 ACs covering seven diagnostic phases, all exposed as MCP tools via the
existing auto-generation from `pluginserver.RegisterRPCs`. No MCP code changes.

### Phase summary

| Phase | Deliverables |
|-------|-------------|
| 1. Runtime state | L2TP observer/CQM/echo, interface counters, component/plugin status, traffic |
| 2. Event history | Global event ring, BGP peer FSM history, L2TP tunnel/session FSM history |
| 3. Metrics query | Prometheus text filter by name+labels, L2TP/BGP health summaries |
| 4. Packet capture | L2TP + BGP decoded capture rings (zero-alloc), raw byte capture for pcap export |
| 5. Active probes | Ping (raw ICMP), traceroute (CLI + daemon RPC), route lookup |
| 6. Health | Component health registry, HTTP /health handler (200/503) |
| 7. Log query | slog ring handler wired into logger, level/component/count filter |

Additional features added to close remaining gaps:
- Per-session traffic handler (`ze-l2tp-api:session-traffic`)
- VPP dataplane trace via CLI socket (`ze-show:vpp-trace-*`)
- Opt-in raw byte capture with pcap export (`ze-show:capture-raw`)

## Key design decisions

1. **CLI-first, MCP for free.** Every diagnostic is a YANG RPC registered as a CLI
   command. MCP auto-generation picks it up without code changes. This was the
   umbrella's core architectural insight and it held throughout all seven phases.

2. **Two-tier capture design.** Decoded capture rings store numeric value types only
   (zero-alloc append). Raw byte rings are opt-in, activated via debug command,
   with fixed-size array slots (1500B L2TP, 4096B BGP). String formatting deferred
   to snapshot time.

3. **Atomic pointer for runtime activation.** Raw capture rings use
   `atomic.Pointer` with `CompareAndSwap` for race-free activation from RPC
   handler goroutines while reactor goroutines read on the hot path.

4. **Child specs skipped.** All seven phases were implemented directly from the
   umbrella. The uniform pattern (ring buffer + handler + RegisterRPCs) made
   individual child specs unnecessary overhead for this scope.

## What went well

- The umbrella's inventory of "exists but not queryable" was accurate and guided
  implementation directly. No research needed beyond confirming function signatures.
- The L2TP observer's `eventRingPool` pattern (pre-allocated, fixed-size, snapshot
  returns copies) was reused for all new ring buffers without modification.
- Review passes caught real issues: VPP command injection, data race on raw capture
  pointer, missing BGP dump path. All fixed before commit.

## What was tricky

- VPP trace has no typed govpp API. The solution (CLI socket + text response) works
  but produces unstructured output. Input validation via regex prevents injection.
- Adding methods to `l2tp.Service` interface required updating three fake
  implementations across test files in different packages.
- `replace_all` edits that include the constant definition in scope cause
  self-referencing cycles. Must fix the definition line separately.

## Patterns to reuse

- **Ring buffer with atomic opt-in:** `atomic.Pointer[Ring]` field, `CompareAndSwap`
  for enable, `Store(nil)` for disable, `Load()` at read sites. Zero cost when
  disabled, no lock contention when enabled.
- **Pcap writer:** stdlib-only, 24-byte global header + 16-byte per-packet header.
  No external dependencies. `LINKTYPE_RAW` (101) for tool compatibility.
- **VPP CLI socket:** `net.DialTimeout("unix", path, timeout)`, write command,
  read with `bufio.Scanner` (4MB buffer), close per invocation. Env var override
  for test injection.

## Files

None recorded.
