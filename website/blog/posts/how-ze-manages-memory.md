---
title: How Ze reuses memory for BGP UPDATEs
date: 2026-08-04
author: Thomas Mangin
description: What ExaBGP taught me about repeated allocation, and why Ze's memory design depends on representation and ownership as much as its choice of Go.

deck: Moving from Python to Go removed one ceiling. I still had to decide which bytes Ze could borrow, where a copy needed its own lifetime and when storage could be reused.

image: assets/blog/how-ze-manages-memory.svg
image-dark: assets/blog/how-ze-manages-memory-dark.svg
image-alt: One BGP UPDATE moves from pooled receive storage through borrowed read-only views; copy boundaries create peer-owned output and long-lived canonical attributes before each owner releases its storage.

---

The Common Gateway Interface let an HTTP server invoke an external program to produce a webpage. I applied much the same separation to [ExaBGP](https://github.com/Exa-Networks/exabgp): it managed BGP sessions and wire messages, while ordinary processes could consume events and announce routes without implementing the protocol themselves.

That separation gave applications convenient data to use, and I paid attention to performance from the start. Transcoding BGP data is expensive. Large announcements and full-table decoding exposed the cost of Python's object model, because an allocation which looks harmless for one attribute is repeated across the routes which carry it.

Writing Ze in Go removed some of those costs, but a direct translation would have preserved much of the same data movement. I wanted to change the representation as well. An UPDATE has already arrived as bytes, and turning it into a complete set of objects before deciding what to do with it can create another version of information the program already holds.

My starting point was to keep those bytes useful for as long as possible. Readers would borrow them, and a copy would have to answer a specific need for a different representation or a different owner. That decision reaches much further into Ze than the choice of a buffer pool, because the difficult part is deciding when the next operation is allowed to overwrite the memory.

*This article was co-authored with Claude and revised with OpenAI Codex. The architecture and design decisions are mine. Claude helped with implementation, tests and measurements, and with organising and drafting the original text.*

## Keep the received representation useful

The [session read loop](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_read.go) obtains reusable receive storage, reads the BGP header into it, checks the message length and fills the remaining bytes. Standard BGP messages fit within 4,096 octets. A speaker which advertises the [BGP Extended Message Capability](https://www.rfc-editor.org/rfc/rfc8654.html) must be able to receive up to 65,535 octets, except for OPEN and KEEPALIVE messages. Ze uses 4 KiB and 64 KiB receive storage for those limits.

After receive-side validation, consumers use an immutable wire representation of the UPDATE. A reader can take a view of its path attributes or prefixes and use an iterator to examine the values it needs. The [buffer-first architecture](https://github.com/ze-software/ze/blob/main/docs/architecture/buffer-architecture.md) lets an operation inspect an attribute without first constructing a second, fully decoded route.

I wanted that to be the normal way to read a message. If an operation needs the next hop, there is no reason for it to manufacture a list of community objects as a side effect. The wire representation contains both, and the reader can leave the communities alone until it has a use for them.

```text
receive pool -> validated UPDATE in cache-owned storage
                   |
                   +-> borrowed views for current processing
                   +-> canonical attribute storage for the RIB
                   +-> borrowed unchanged bodies for eligible sends
                   +-> peer-owned output for rebuilt bodies

cache eviction -> receive storage returned
send completion -> rebuilt output storage returned
route release  -> canonical attribute references released
```

A slice over a buffer copies no payload bytes, and Go's garbage collector keeps the backing array alive while a slice still refers to it. That is useful, but it answers only whether the memory still exists. Once the owner returns the buffer to its pool, another read may fill it with a different UPDATE, and the old slice will read those new bytes without any complaint from Go.

This is the part of the ownership rule which I cannot delegate to the language. Go sees a valid reference to a live array, while Ze needs to know whether it still contains the message that reference was meant to identify. Borrowing therefore has to end at a boundary the components agree on.

The [recent UPDATE cache takes ownership before event delivery](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_notify.go). It normally releases storage after consumers have acknowledged the UPDATE and counted retentions have ended. A structured-event consumer which needs bytes beyond its delivery call can take an owned [WireUpdate snapshot](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/wire_update.go). Keeping a slice alone does not extend the cache's lifetime protocol.

The cache also has a safety valve for stalled entries which later completed entries have passed. A counted retention cannot keep network storage indefinitely, so a slow consumer eventually needs storage with a different owner. That limit becomes important when the UPDATE is waiting to leave through another peer.

## Give each necessary copy a reason

Copying the UPDATE for every consumer would avoid some lifetime bookkeeping, but it would also repeat the same data movement for readers which finish immediately. I chose to keep borrowing available and make the ownership changes explicit. A snapshot buys independence from the receive buffer. A policy edit needs different bytes, while external text or JSON output needs another representation.

Forwarding can avoid a body copy when no policy change, encoding conversion, path-identifier rewrite or message split is required. In that case, [the forwarding body builder](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_body.go) passes a view of the original body to the destination's send worker, with the cache responsible for the source bytes. Matching encoding contexts alone are insufficient: the destination can still require another change before the message leaves.

A changed body needs its own storage because later destinations may still need the original input. Ze records the required edits, calculates the resulting size and [writes into the destination peer's output storage](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_build.go). Unchanged sections can go directly from the input into that buffer, alongside the replacement values. There is no need to copy the whole message into temporary storage and then copy that temporary result again.

Each established destination has [64 output buffers](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_pool.go) cut from one backing array at its negotiated message size. A rebuilt send owns its buffer until the worker has finished with it. When another peer needs the same rebuilt bytes, I chose to keep that peer's send independent too: Ze copies the earlier result into the second peer's buffer. [One BGP UPDATE, many peers](../one-bgp-update-many-peers/) explains why the saved rebuild was worth sharing while that final copy was worth keeping.

A send can also wait because its destination is congested, or because the destination's initial route announcements must leave first. Items entering the overflow queue [take an owned copy of their bodies](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_body.go), including bodies which the normal channel path would borrow. Otherwise the cache's safety valve could recycle the receive buffer while a queued send still referred to it.

That copy uses overflow-pool storage when it fits. If suitable storage is unavailable, Ze allocates. Avoiding an allocation cannot justify transmitting a later UPDATE's bytes through a stale view, and this is one reason a pool budget must never be presented as a fixed memory ceiling for the whole daemon.

## Reuse has a capacity cost

Receive storage and peer output have different release points, and queued overflow can remain live much longer than either normal operation. I kept their pools separate so their capacity and exhaustion behaviour could follow those lifetimes. A pool retains memory in the hope that the next operation will need it, which is a useful exchange for repeated traffic and an expensive one if the retained capacity has no limit.

The demand calculation also needs to account for what a session is doing. A peer sending its initial table creates different pressure from one sending occasional changes. Ze uses locally configured prefix maximums, summed across address families, to estimate that demand. Those values are operator limits, rather than a memory allowance advertised by the remote peer.

Before the initial table's [End-of-RIB markers](https://www.rfc-editor.org/rfc/rfc4724.html), the [demand calculation](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_pool_weight.go) uses the full prefix allowance. Afterwards it uses a smaller burst allowance. The shared overflow budget also reserves for restart demand, because a large table may arrive again and need forwarding to several destinations.

The [forward congestion design](https://github.com/ze-software/ze/blob/main/docs/architecture/forward-congestion-pool.md) describes the pressure controls and eventual session teardown. Retaining more memory buys time for a slow destination, but continued congestion needs an operational response. I would rather make that response part of the design than treat every exhausted pool as a request to keep growing.

Temporary storage which is cheap to recreate has a less demanding contract. Go's [`sync.Pool`](https://pkg.go.dev/sync#Pool) may discard a cached item, so an operation using it must remain correct when it receives freshly allocated storage. Ze's [Bidirectional Forwarding Detection packet pool](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/pool.go) is a small example: fixed-capacity buffers suit the packet size, and the encoder writes into the borrowed storage before it is returned.

## A route needs a longer lifetime

The routing information base, or RIB, may keep a route long after its receive buffer has been returned. Borrowing the packet is the wrong lifetime for that job, but copying every route's complete attributes would retain many equal values repeatedly. I wanted route storage to share those values without keeping the incoming packet alive.

Ze's RIB plugin [interns individual attributes](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/attrparse.go). Interning stores a canonical value and gives its users a compact handle. Two routes with different Multi-Exit Discriminator values can still share their equal AS path and communities, so one differing attribute does not force a duplicate of everything else.

The [attribute pool](https://github.com/ze-software/ze/blob/main/internal/component/bgp/attrpool/pool.go) copies a value on its first occurrence. A later equal value increments a reference count and returns a 32-bit handle to the existing storage. Reading a live handle borrows the canonical bytes, and releasing the final reference makes the slot available for reclamation. The copy has bought the route its own lifetime, and interning lets other routes share the cost.

Busy attribute pools are partitioned by a content hash, while types with few distinct values use fewer partitions. Released values leave holes in the backing storage, so compaction moves live entries in batches while retaining access through live handles. The [pool architecture](https://github.com/ze-software/ze/blob/main/docs/architecture/pool-architecture.md) describes the handle and compaction rules. Sharing saves storage, but it also creates reference counts and reclamation work which have to be accounted for.

I also wanted a lifetime mistake to have a chance of becoming visible during development. With the `debug` build tag, [receive-buffer release](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session.go) overwrites returned bytes with the repeating pattern `DE AD BE EF`. Attribute slots are also poisoned, and debug builds refuse to reuse their slot numbers so a stale handle continues to fail as a dead slot.

These are diagnostics, and their limits are part of the [lifetime contracts](https://github.com/ze-software/ze/blob/main/docs/architecture/memory/lifetime-contracts.md). The poison can be overwritten when memory is reused, and refusing to reuse attribute slot numbers consumes extra capacity in a long debug run. Neither gives a released slice a valid lifetime again.

## The destination belongs in the encoder's API

The same reasoning applies before a result reaches a pool. A convenient encoder can create a byte sequence which its caller immediately copies into a network buffer. If the caller already has the final storage, I want the encoder to write there instead. Ze's buffer-first encoders accept caller-owned storage, and the UPDATE builder sizes the result before writing its retained and replacement sections into the destination.

Formatting text has a related cost. Go's general formatter can interpret dynamic formats and handle widths, precision and custom types. A repeated formatting operation in Ze often knows both the layout and the types in advance, and a function which returns a new string has introduced another ownership boundary whether the caller needed one or not.

The [Zig formatting case study](https://ziglang.org/documentation/master/#Case-Study-print-in-Zig) is a useful example of a different language choice: compile-time knowledge of the format and argument types can produce specialised writes. Go needs another approach. Ze's [`textbuf.Buffer`](https://github.com/ze-software/ze/blob/main/internal/core/textbuf/textbuf.go) uses the escape-analysis technique also found in [`strings.Builder`](https://go.dev/src/strings/builder.go), with a 128-byte inline array and typed append operations for values such as integers and addresses.

A local buffer initialised with `Reset` can assemble a result within that inline capacity without acquiring heap-backed result storage. The extraction method then states what the caller needs. `Slice` freezes the buffer and returns a borrowed string view, valid until reset or release. `String` returns owned text and empties the buffer: inline data is copied so it can survive reuse, while heap-backed data can transfer its storage.

I find that distinction more useful than a general promise that string building is fast. A result consumed immediately can borrow, and a result retained elsewhere needs ownership. The [text buffer design](https://github.com/ze-software/ze/blob/main/docs/architecture/textbuf-string-building.md) and its [tests](https://github.com/ze-software/ze/blob/main/internal/core/textbuf/textbuf_test.go) describe the two lifetimes. They are the same choice we met at the receive buffer, in a smaller API.

## What the language can and cannot decide

Rust developers will recognise the ownership and borrowing concerns, although Go has no borrow checker to enforce Ze's release boundaries. The [Rust ownership guide](https://doc.rust-lang.org/book/ch04-01-what-is-ownership.html) and the [Go garbage collector guide](https://go.dev/doc/gc-guide) describe different memory models.

This is why I do not regard manual memory management as a performance result in itself. A program can clone data repeatedly to regain sole ownership, or keep several decoded representations of something it already has. A garbage-collected program can reuse its existing storage and create little new garbage on a repeated path. The lifetime design determines which of those operations the program asks the runtime to perform.

Xavier Leroy's [interview about OCaml and systems programming](https://www.youtube.com/watch?v=9Cswiqrq6So) discusses the relationship between memory management and performance. It is a useful companion to this design: uncertain ownership can encourage defensive copies, and those copies still cost something when reclamation is manual. My reason for making ownership explicit in Ze is to decide where independence is necessary and where sharing is safe, before trying to optimise the allocation which follows.

## Keep the allocation claim testable

A design rule is easy to lose as code changes. Ze has allocation assertions for repeated operations whose working storage is already available, so a new heap allocation in a guarded path can fail during development. The assertion has to name the operation it covers. Session creation, maps, external serialisation and cold pool growth still allocate, and all of Ze's backing arrays remain on Go's managed heap.

For UPDATE rebuilding, [the allocation tests](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_build_merge_test.go) cover peer-specific attribute changes with a free output buffer. The community-removal case checks that the requested values disappear and the others survive in order, alongside the warm rebuild's allocation count. That is a useful contract for the encoder, even though the wider [BGP forwarding path](https://github.com/ze-software/ze/tree/main/internal/component/bgp/reactor) still has allocations around it.

The original measurements published with this article used Go 1.26.5 on an Apple M4 Max running macOS on arm64, with medians of five runs and allocation reporting. They are historical microbenchmarks of isolated operations, retained here to show the scope of the claim.

| Operation | Median time | Heap bytes | Heap allocations |
|---|---:|---:|---:|
| BFD pooled encode and parse round trip | 31.7 ns | 0 | 0 |
| Intern an existing attribute value | 145 ns | 0 | 0 |
| Read a live attribute handle | 6.65 ns | 0 | 0 |

The [BFD benchmark](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/bench_test.go) acquires a packet buffer, writes and parses one unauthenticated control packet, then releases it. The [attribute-pool benchmarks](https://github.com/ze-software/ze/blob/main/internal/component/bgp/attrpool/benchmark_test.go) use the existing value `benchmark-data` for both interning and handle lookup. They do not measure a new route's complete attributes, TCP throughput or route convergence, and a compiler change can move the timings.

I am interested in keeping the ownership and allocation contracts intact when those timings move. Following an UPDATE through Ze exposes why the RIB needs a copy and why a changed send needs another buffer, while an immediate reader can keep borrowing. A direct port of ExaBGP would have brought its representation choices with it. Choosing Go gave me room to change them, but I still had to make those decisions.

*Last updated: 9 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/how-ze-manages-memory.md).*
