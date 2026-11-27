---
category: platform
title: Giving ExaBGP scripts the performance Python could not
date: 2026-11-27
author: Thomas Mangin
description: How Ze redesigned its ExaBGP bridge around a positional netstring-style wire line, direct unicast rendering and a zero-allocation text buffer parser so ExaBGP scripts handle full routing tables.

deck: Codecs often take more CPU than the protocol they serve, and sending a full BGP table to twenty scripts made that cost hard to ignore.

---

In Ze, migrated ExaBGP scripts run as separate external processes so a slow or crashing script cannot stall the routing daemon. Every BGP UPDATE crossing that process boundary has to be encoded by `ze` and decoded by each script wrapper.

When twenty ExaBGP scripts subscribed to a million IPv4 routes, forwarding throughput fell by more than eighty per cent, and most of that extra CPU went into serialising and deserialising JSON.

*This article was drafted with Gemini. The architecture, design decisions and conclusions are mine. Models helped with implementation, measurements and drafting the text.*

## Codecs are often where the CPU goes

The reason I started building Ze was to give ExaBGP users the performance that a BGP engine written in a scripting language such as Python cannot deliver. Operators still want to drive routes from their existing scripts, while expecting the daemon underneath to handle full routing tables without stalling. Letting codec overhead dominate the ExaBGP bridge would defeat that goal.

Most systems engineers already know that JSON serialisation is expensive, even with a faster implementation such as Go's [`encoding/json/v2`](https://pkg.go.dev/encoding/json/v2). Across network software, encoding and decoding are frequently the most CPU-intensive part of the program, ahead of the protocol logic itself. A general text codec pays that cost on every byte it touches: the encoder walks structured fields, writes repeated key names, quotes strings and escapes control characters, and the parser scans every byte back out to find tokens, unescape strings, look up field names and allocate intermediate objects.

Inside `ze`, the BGP engine avoids that work on the forwarding path by keeping received messages in their wire form, as described in [How Ze reuses memory for BGP UPDATEs](../how-ze-manages-memory/). On the ExaBGP bridge, the initial full-JSON transport undid that saving at the process boundary.

For every BGP UPDATE, `ze` decoded all path attributes and NLRIs into roughly 1.5 to 2 KiB of JSON alongside the raw wire bytes in hexadecimal. The external RPC layer then wrapped batches of those strings in generic Go maps and marshalled them as JSON again, escaping every event a second time before encrypting one TLS copy per script process.

Each wrapper process received that frame, unmarshalled the JSON back into generic maps, and then decoded fields such as `AS_PATH`, extended communities and unknown attribute flags straight off the attached wire hex anyway to match ExaBGP's exact representation. After doing all of that work, the wrapper reflected those maps back out into ExaBGP's own spaced JSON or line text for the child script. Repeating that double round trip for twenty subscribers across a million routes saturated the host.

## Finding the middle ground between binary TLV and JSON

Binary protocols avoid most codec overhead by laying fields out in a fixed order or as Type, Length, Value (TLV) records. A reader jumps from one field to the next by offset and byte count, skips repeated key names and never scans payload bytes for delimiters. The drawback appears the first time a session misbehaves and you capture the pipe, because raw binary requires a decoder before a human can inspect it.

Plain text and JSON give you immediate readability on a pipe, while charging the scanning and key-name tax on every message. That experience comes straight from ExaBGP, whose API has long used a line-based text protocol so an engineer can watch a pipe and see what happened without reaching for a decoder. Keeping a line-based text format for Ze's high-volume internal links, and moving the UPDATE hot path off JSON once the bridge behaviour was settled, was the plan all along; the `base64` wire format had even been reserved in Ze's plugin schema ahead of time. I wanted the middle ground: a line format that stays readable in a capture while letting the parser work much like a binary TLV reader.

That design borrows directly from Daniel J. Bernstein's [netstrings](https://cr.yp.to/proto/netstrings.txt) and from the positional grammar Ze already uses for streamed command answers. Under `base64` delivery, each BGP UPDATE crosses the boundary as one quote-free ASCII line:

```text
upd <flags> <local-as> <remote-as> <conn-id> <n>:<peer> <n>:<local> <n>:<router-id> <n>:<extra-addpath> <n>:<b64-body>
```

A concrete IPv4 unicast UPDATE looks like this on the wire:

```text
upd 14 65000 65001 42 8:10.0.0.1 8:10.0.0.2 7:1.1.1.1 0: 36:AAAbQAEBAAJAAgAEAAH96EADAQoAAAEAAhgKAAA=
```

Position replaces the Type field of a binary TLV record. Every line opens with a three-letter verb closed by a space (`upd ` here, and `top `, `row `, `bad `, `end `, `wrn ` or `nay ` on Ze's streamed command answers), so the reader loads four bytes as a single 32-bit integer and dispatches in one comparison. Because the field order after the verb is fixed, no key name ever crosses the wire.

Variable text and binary fields carry their byte length in front of a colon (`<n>:<bytes>`), following the netstring idea with space separators in place of the trailing comma. An absent field writes `0:`, keeping the positional field count constant. The parser reads the count, checks that at least that many bytes remain in the line and slices straight over the value by integer arithmetic, without scanning inside the payload for delimiters or worrying about embedded spaces or newlines.

Plain unsigned integers (`flags`, `local-as`, `remote-as`, `conn-id`) omit the length prefix and end at the space after them. Because decimal digits can never contain a space, a count in front of a number buys the parser nothing: one fused loop accumulates `val = val*10 + digit` while walking to the space. In earlier measurements on Ze's command-answer framing, that plain fused form took 3.2 ns against 8.6 ns for a length-prefixed number, while saving two bytes per field.

The BGP UPDATE body itself (everything after the 19-octet BGP header, up to RFC 8654's 65,516-byte extended message limit) travels in standard base64. Base64 keeps the line printable and free of quotes or backslashes, so embedding it in a batched RPC frame requires no JSON escaping pass, and it is a third smaller than hexadecimal. The daemon encodes the raw UPDATE bytes straight into a stack-local scratch buffer without decoding attributes or NLRIs.

Keeping the encoder inside `ze` this simple also changes where the decoding CPU is spent. Because the main daemon writes only the positional header and the base64 wire bytes, the work of decoding BGP attributes and formatting ExaBGP output moves out of `ze` and into each external wrapper process. The operating system can schedule those wrapper processes onto separate CPU cores and bound their CPU usage independently of the routing daemon, and because external bus connections already run over TLS, that decoding and script work can even run on a separate machine down the line if a deployment needs to keep it off the router.

Only UPDATEs switch to this line. Low-frequency control and session events (`state`, `negotiated`, `open`, `notification`, `keepalive`, `refresh`) occur a handful of times over the life of a session, so they remain JSON with the raw message bytes attached in hexadecimal. Keeping rare events in JSON avoids maintaining a second set of wire decoders where CPU time is negligible. The ExaBGP wrapper opts into `base64` delivery automatically at startup, so migrated scripts get the faster path with no configuration change.

## Skipping intermediate maps on both sides

Changing the wire format removes half the waste if the daemon still marshals batches through generic maps or the wrapper still allocates maps to build ExaBGP's output. Both sides therefore had to drop intermediate map reflection.

On the daemon side, batched RPC delivery now writes event frames directly into pooled byte buffers, splitting oversized batches in FIFO order at the 16 MiB frame limit without constructing a temporary map per batch.

Inside the ExaBGP wrapper, hot IPv4 and IPv6 unicast UPDATEs (including ADD-PATH, End-of-RIB markers and packet or consolidated forms) bypass map construction altogether. The wrapper parses the decoded wire body into stack structs and writes ExaBGP's exact spaced JSON (`{ "key": value }`, `[ a, b ]`) or line text directly into a byte buffer, reusing pre-serialised negotiated capability bytes per peer and a bounded encoding-context cache.

Non-unicast address families (EVPN, FlowSpec, VPN, BGP-LS and the rest) and malformed or unsplit NLRI error paths fall back to the general wire-backed event renderer. That keeps the direct renderer small while covering full Internet routing tables, and differential tests confirm byte-identical output against the legacy JSON path across ExaBGP 4.2, 5.0 and 6.0.

## What changed in the benchmarks

The progression was measured on an Apple M4 Max running a Linux Docker host (4 vCPUs, 7.75 GiB RAM), with independent ExaBGP JSON script endpoints subscribed to BGP updates, state changes and negotiated capabilities. Every run completed with zero lost routes. The 100,000-route runs used three measured repeats after warmup (shown with spread); the 1,000,000-route runs used one measured repeat after warmup.

| Stage | 100k × 5 live | 100k × 20 live | 100k × 20 peak CPU | 1M × 5 live | 1M × 20 live | 1M × 20 RIB prop. | 1M × 20 `ze` peak RSS |
|---|---:|---:|---:|---:|---:|---:|---:|
| Baseline full JSON | 279 ms (358k r/s) | 907 ± 22 ms (110k r/s) | 377.7% | 3,796 ms (263k r/s) | 9,773 ms (102k r/s) | 9,419 ms (106k r/s) | 4,143 MiB |
| Positional `base64` `upd` + pooled batch + direct renderer | 180 ± 20 ms (556k r/s, 1.55×) | 301 ± 23 ms (332k r/s, 3.01×) | 135.6% (−64%) | 1,713 ms (584k r/s, 2.22×) | 2,679 ms (373k r/s, 3.65×) | 6,178 ms (162k r/s, 1.52×) | 3,232 MiB (−911 MiB) |
| Growable lossless delivery queue (64 to 65,536) + 512-event drains | 136 ± 16 ms (735k r/s, 2.05×) | 266 ± 20 ms (376k r/s, 3.41×) | not re-sampled | 1,504 ms (665k r/s, 2.52×) | 2,654 ms (377k r/s, 3.68×) | 5,490 ms (182k r/s, 1.72×) | not re-sampled |

Moving off full JSON delivered most of the twenty-script gain on its own. Live convergence at 100,000 routes sped up 3.01× while peak CPU fell by 64 per cent, live convergence at a million routes sped up 3.65× (cutting the marginal cost per script from roughly 409 ms to 64 ms), and `ze`'s peak resident memory dropped by 911 MiB, finishing below the zero-script peak of 3,425 MiB.

The second bottleneck in the table was already known beforehand, and there was no point implementing that optimisation while the JSON codec still dominated CPU time. Each external endpoint was fed through a fixed 64-slot Go channel draining 64 events per RPC batch, so packed full-table bursts backpressured the BGP forwarding loop while waiting for subscribers to acknowledge frames. Once the positional transport landed, replacing that fixed channel with a growable, bounded, lossless ring queue (growing from 64 up to 65,536 slots and shrinking back when drained, with 512-event batch drains) removed lockstep blocking. With both changes in place, one to five scripts sit within 2 to 6 per cent of zero-script forwarding speed at 100,000 routes (735k to 800k r/s), and twenty-script RIB propagation at a million routes returned to zero-script speed (182k r/s against 181k r/s).

A growable ring buffer also illustrates a limitation we cannot avoid in Go today. Go is a great language for a network daemon, and its goroutines make concurrent sessions and external endpoints much easier to structure, yet Go allocations cannot return an error when memory runs out. In a language such as Zig, where allocators are explicit and every allocation can fail cleanly, the ring could handle a failed growth by staying at its current size and falling back to backpressure without dropping a route or stopping the daemon. In Go, an out-of-memory failure on `make` terminates the process, so Ze has to keep queue growth inside a configured slot ceiling ahead of time instead of reacting when the host runs short of memory.

What remains at twenty scripts is the hardware ceiling of the test VM and the process model itself. Running `ze` alongside twenty concurrent Go wrapper processes on four vCPUs causes CPU contention during a live burst, which explains the gap between 377k r/s at twenty scripts and 796k r/s at zero scripts. Per-wrapper private physical memory also stays around 78 to 93 MiB per script (the read-only pages of the binary are shared in the page cache), because each ExaBGP endpoint runs in its own OS process for isolation.

## Folding the parser into `textbuf.Buffer`

Looking closely at the new `upd` parser after the benchmarks landed revealed a duplication inside Ze. The ExaBGP bridge had gained a single-pass fused accumulator parser for positional counted lines, while Ze's streamed command-answer protocol still parsed the exact same grammar with separate helpers that scanned each number twice (counting digits first, then calling `strconv.ParseUint`) and copied payloads into strings before slicing them.

In [How Ze reuses memory for BGP UPDATEs](../how-ze-manages-memory/), Ze's text buffer borrowed from DJB's [`stralloc`](https://cr.yp.to/lib/stralloc.html) to build text inside reusable storage. Bringing the netstring-style reader directly into that same buffer type completes the other half of the idea: one object builds and parses positional counted lines without copying or allocating.

To avoid allocating a fresh buffer when a wire line arrives from a socket or an RPC frame, the buffer can take ownership of an existing `[]byte` (or wrap an immutable `string` read-only) and advance an internal cursor across it:

```go
var wordUpd = textbuf.MakeWord4("upd")

var b textbuf.Buffer
b.Adopt(rawLine)
b.TrimTrailingLF()

word, ok := b.ReadWord4()
if !ok {
	return errInvalidPrefix
}
if word != wordUpd {
	return errInvalidPrefix
}
flags, err := b.ReadUintSpace()
peer, err := b.ReadCountedBytesSpace()
body, err := b.ReadCountedBytesEnd()
```

Go has no borrow checker like Rust, so the language cannot prove at compile time that a caller will never keep a returned slice past a buffer reset. Putting the parser inside the text buffer itself lets the type enforce the boundaries Go can check at runtime. The first read freezes the buffer against writes, so any accidental write panics immediately instead of overwriting bytes behind borrowed slices. Counted byte reads return three-index slices capped at the field's exact length, so appending to one parsed field allocates fresh memory rather than trampling the next field in the line. Wrapping an immutable string marks the buffer read-only, panicking if anything asks for a mutable byte slice and detaching the backing storage on reset or release so Go string memory can never be mutated.

Allocation tests confirm zero heap allocations (`0 B/op`, `0 allocs/op`) for parsing a full `upd` line from either a byte slice or a string, and fuzzing exercises hostile counts, overflows and malformed frames across the shared reader. Moving both the ExaBGP bridge and Ze's command-answer parser onto that single buffer type removed over 180 lines of duplicated parsing code so every high-volume text wire path shares one implementation.
