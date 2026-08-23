---
title: How Ze keeps BGP traffic away from the garbage collector
date: 2026-08-04
author: Thomas Mangin
description: How Ze reuses buffers, borrows wire data and limits copies so repeated routing work creates little garbage.
deck: Ze controls allocation by making ownership and lifetime explicit: borrow immutable wire data, copy at ownership boundaries and return bounded storage.
---

The Common Gateway Interface let an HTTP server invoke an external program to produce a webpage. ExaBGP applied the same separation to BGP: it managed the sessions and the wire messages, while ordinary processes exchanged decoded events and route announcements with it.

That separation worked. Applications received convenient Python objects instead of parsing raw BGP bytes. Transcoding BGP data is expensive, so I wrote ExaBGP with performance in mind from the start, but there is only so much any program can do about the interpreter underneath it. Large announcements and full-table decoding showed where Python's object model spends its memory. An allocation which looks harmless on its own is repeated for every route, every attribute, every value and the cost becomes obvious.

Ze is written in Go and follows one rule about memory: the received wire representation stays immutable, readers borrow views of it while its owner is alive, a copy happens when the representation or the lifetime changes, and owned storage returns to a bounded pool. The backing arrays still belong to Go's managed heap. Keeping traffic away from the garbage collector, as the title puts it, means that the work Ze repeats thousands of times a second creates almost no new heap objects.

Rust developers will recognise their borrow checker in that description, without a compiler to enforce it. Zig and Odin developers will recognise arena allocation.

The rest of this article follows one BGP UPDATE through that design, from the receive buffer to the RIB and out to each peer, then shows the tests which keep the design intact.

*This article was co-authored with Claude. The architecture, design decisions, measurements and conclusions come from my work on ExaBGP and Ze. Claude helped organise the material and draft the text.*

## One UPDATE, one backing buffer

When processing BGP messages, the normal receive and forwarding path is:

```text
TCP connection
  reusable 4 KiB or 64 KiB receive storage
      v
  read-only views over the received bytes
      +--> lazy attribute and route iteration
      +--> route storage keeps compact attribute handles
      +--> unchanged forwarding borrows the received bytes
      +--> changed forwarding writes into peer-owned storage
      v
TCP write completes, then each owner releases its storage
```

The session reads directly into memory Ze has already allocated. The memory subsystem hands the connection a slice for the packet, and the read fills it.

Standard BGP messages are limited to 4,096 octets. A peer which advertises the [BGP Extended Message Capability](https://www.rfc-editor.org/rfc/rfc8654.html) can receive messages up to 65,535 octets, with the exception of OPEN and KEEPALIVE. Ze rounds those protocol limits into 4 KiB and 64 KiB storage classes.

The received UPDATE remains a read-only wire message. When Ze needs part of it, such as its path attributes or announced prefixes, it creates a byte-slice view over the relevant bytes in the receive buffer. Iterators examine attributes and prefixes one at a time. Requesting the next hop parses only that attribute; the communities and AS path stay exactly as they arrived.

Creating these slice views allocates no heap memory and copies no bytes. Each view is an ordinary Go slice over the receive buffer, so the garbage collector keeps those bytes alive for as long as anything still holds the view. What a view cannot survive is reuse. Once the receive path releases the buffer back to its pool, the next read fills the same memory, and a view kept past that point reads a later message. Go has no way to say where that release point is, so the rule lives in Ze's API.

Before the receive step returns, ownership of the buffer passes to Ze's recent UPDATE cache. The cache keeps the original bytes available to forwarding and other consumers. Normally it returns the buffer to the pool after every consumer has acknowledged the UPDATE and any explicit retention has been released; a safety valve can evict stalled entries.

## Copy at ownership boundaries

Copying an UPDATE for every consumer would move the same bytes again and again, so Ze copies when ownership or representation changes:

| Reason for a copy | Why the existing bytes cannot be reused |
|---|---|
| A consumer retains an UPDATE beyond the current processing step | Receive storage may be recycled when that step finishes |
| A filter changes an attribute or route | The source is immutable and other destinations may still need it |
| Two peers use different wire encodings | Negotiated capabilities can change AS number width or route framing |
| An attribute first enters long-lived route storage | Receive storage will be returned while the route may remain |
| An external consumer requests text or JSON | The requested representation differs from BGP wire bytes |

A consumer which needs a longer lifetime takes an owned snapshot; anything which finishes inside the current step keeps borrowing the original bytes. Forwarding borrows the input too, when source and destination negotiated the same encoding. Policy changes are written into storage owned by the destination peer, so the cost of copy-on-modify falls on the destinations which actually need different bytes.

A lifetime error has to be loud, and a Go slice carries no ownership information to make it so. In builds compiled with the `debug` tag, Ze overwrites a receive buffer with the repeating bytes `DE AD BE EF` before returning it to the pool. A stale reader then sees poison instead of bytes from a later UPDATE. Released attribute slots use the same guard. Without the `debug` tag, the compiler removes this diagnostic path.

## Storage follows lifetime

Receive input, peer output, slow fan-out and temporary scratch data have different owners. Giving them one pool would mix their limits and their failure behaviour.

| Storage class | Owner and lifetime | Behaviour at capacity |
|---|---|---|
| Receive storage | Receive path until parsing and forwarding release it | Bounded growth, then backpressure |
| Peer output | One destination until its TCP write completes | Peer-local fallback without shared mutable output |
| Fan-out overflow | Shared while slow consumers retain source data | Global pressure control and possible session closure |
| Re-creatable scratch data | One operation or goroutine | Recreate after the runtime discards pooled values |

Receive storage grows in blocks under one shared memory budget. Reuse favours existing blocks so completely returned blocks can be reclaimed. A slow peer cannot retain source data without limit. Near exhaustion, Ze applies backpressure and can close a session rather than let memory grow until the process fails.

Peer output follows another rule. Every established peer receives 64 buffers cut from one backing array. The buffer size is the only capability-dependent part: 4 KiB normally, or 64 KiB when the peer negotiates Extended Messages. Its send queue owns a buffer until the TCP write completes, then returns it to that peer. Mutable output never acquires a shared lifetime.

The maximum-prefix value comes from local per-family configuration rather than from a capability the peer announces. The total across all configured families contributes to the shared fan-out overflow budget, which models restart demand and falls to a smaller burst allowance after End-of-RIB.

Scratch values which are cheap to recreate can use Go's [`sync.Pool`](https://pkg.go.dev/sync#Pool). Any cached item may disappear without notice, so correctness must not depend on retrieving an item which was previously returned. Bidirectional Forwarding Detection packets are one example: their fixed maximum size suits equal reusable buffers, and encoding writes directly into borrowed packet storage.

Pooling exchanges allocation churn for retained capacity. It works well for repeated fixed-size operations with clear release points. Variable lifetimes, unbounded retention or ambiguous ownership need a different design.

The [buffer-first architecture](https://github.com/ze-software/ze/blob/main/docs/architecture/buffer-architecture.md), [pool architecture](https://github.com/ze-software/ze/blob/main/docs/architecture/pool-architecture.md) and [forward congestion design](https://github.com/ze-software/ze/blob/main/docs/architecture/forward-congestion-pool.md) record the current sizes, limits and exhaustion rules.

## Long-lived routes share canonical attributes

Receive storage solves short lifetimes. A routing information base, or RIB, keeps its routes long after the input has been released. Many of those routes share their origin, autonomous system path, local preference, next hop and communities. Copying a complete attribute set into every route would waste a great deal of memory.

Ze interns each attribute separately. Interning stores one canonical value and gives routes a compact handle to it. Routes with different Multi-Exit Discriminator values can still share every other equal attribute.

The first occurrence of each distinct value is copied into pool-owned storage. A later equal value increments a reference count, and the route keeps a 32-bit handle. Reading a live handle returns a view of the canonical bytes.

Busy attribute types are partitioned by a content hash so unrelated routes do not contend on one lock. Types with few possible values remain in one partition, because extra locks would consume memory without improving concurrency.

Released values leave holes, so incremental compaction moves live entries in small batches while preserving their handles. This avoids one large pause across the routing table.

## Write directly into the destination

Convenient encoders often create a byte sequence which the caller immediately copies into a network buffer. Ze gives its encoders caller-owned storage instead. They write at a supplied position and report how many bytes they used. Variable-length sections reserve their length field, write their contents, then fill in the length.

Filters record the changes required for one destination in reusable working storage allocated outside the destination loop. That storage is reset for the next peer while retaining its backing arrays. Common changes fit inline. Exceptional size may grow onto the heap, and counters make those cases visible.

The warm common path rejects unexplained allocation. Memory should come from the caller, a bounded pool or storage already owned by the operation. New ownership, exceptional input and cold pool growth may still require heap memory.

Go's general formatter supports dynamic format strings, widths, precision, reordered arguments and custom behaviour. That flexibility is valuable in a library, and in Ze's user interface, diagnostics and other non-critical paths.

The hot path already knows the layout and the types it must write. Parsing a format at run time and routing values through generic arguments adds work for flexibility the caller never uses. Calls which produce a string also need owned result storage. Ze therefore gives repeated formatting paths a smaller API with explicit allocation and lifetime rules.

Zig can keep formatted-string syntax without the same run-time machinery. Its compiler knows the format and the argument types, then generates specialised writes for the literal text and each value. A `%s` or `%d` placeholder needs no allocation of its own; only the destination may need to grow. The [Zig formatting case study](https://ziglang.org/documentation/master/#Case-Study-print-in-Zig) shows that generated structure.

Ze's `textbuf.Buffer` takes a Go-specific route. It uses the same escape-analysis technique as [`strings.Builder`](https://go.dev/src/strings/builder.go), then adds a 128-byte inline array and typed append operations for integers, addresses and protocol values.

`strings.Builder` returns its final string without copying, but its first write still needs backing storage. A local `textbuf.Buffer` can build a common result entirely on the goroutine stack.

Extraction makes ownership explicit. `Slice` freezes the buffer and returns a borrowed string view without allocating; the view is valid only until the buffer is reset or released. `String` returns owned text, copying inline data so it can outlive the buffer, though heap-backed data can transfer its storage instead. Tests guard both lifetime contracts, and pooled buffers reset for reuse.

Language choice does not supply this behaviour automatically. Rust makes ownership visible to the compiler, but cloning still allocates and copies. Go uses a tracing collector, yet bounded reuse can leave it with very little repeated work. The [Rust ownership guide](https://doc.rust-lang.org/book/ch04-01-what-is-ownership.html) and [Go garbage collector guide](https://go.dev/doc/gc-guide) describe different memory models, and unnecessary data movement is expensive under both.

In an [interview about OCaml and systems programming](https://www.youtube.com/watch?v=9Cswiqrq6So), Xavier Leroy makes a related point: manual memory management does not guarantee faster software. Uncertain ownership encourages copies made only to regain sole ownership. Garbage-collected languages can share data safely, and Rust has borrowing and shared ownership when it needs them. What decides the result is the lifetime design, decided well before the language on the label.

## Allocation is a contract

Ze uses allocation assertions for operations which should request no heap memory once their pools are warm. Benchmarks also measure time, but nanosecond results depend on the compiler and the machine. The durable claims concern ownership and allocation:

| Measured path | Warm-path contract |
|---|---|
| Peer-specific UPDATE modification with output storage available | No heap allocation during the rebuild |
| Local and pooled typed text assembly within retained capacity | No heap allocation |
| Pooled BFD encode and parse round trip | No heap allocation |
| Existing route-attribute intern and handle lookup | No heap allocation |

Supporting microbenchmarks were run with Go 1.26.5 on an Apple M4 Max running macOS on arm64. Each observation below is the median of five runs with allocation reporting. These figures describe isolated mechanisms rather than pass-or-fail performance targets:

| Operation | Median time | Heap bytes | Heap allocations |
|---|---:|---:|---:|
| BFD pooled encode and parse round trip | 31.7 ns | 0 | 0 |
| Deduplicate an existing BGP attribute | 145 ns | 0 | 0 |
| BGP attribute handle lookup | 6.65 ns | 0 | 0 |

The executable evidence lives in the [BFD packet benchmark](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/bench_test.go), [attribute-pool benchmarks](https://github.com/ze-software/ze/blob/main/internal/component/bgp/attrpool/benchmark_test.go) and [BGP hot-path tests](https://github.com/ze-software/ze/tree/main/internal/component/bgp/reactor).

These checks say nothing about route convergence, TCP behaviour or every allocation the daemon makes. Session creation, maps, control-plane state, external serialisation, pool growth and exceptional spill paths all still allocate. What the checks buy is that a regression in a guarded common path fails during development, instead of becoming unexplained operator pressure.

## What transfers to other systems

None of this is free. Reuse keeps capacity you have already paid for, and in exchange it asks you to say who owns what. Every pool needs a byte limit, every borrowed view needs a release point, every stale access needs a diagnostic which fails loudly during development, and running out of memory has to leave the process in a safe state. That is a lot of bookkeeping for an operation which runs twice. It is cheap for one which runs a million times.

If you want to apply this elsewhere, the useful exercise is to take one item, follow it from input to final output, and name its owner at every stage. Where the name changes, you have found a copy, and it is either one you need or one you can remove. Where the name is unclear, you have found the bug you will spend an afternoon on later.

Compared with ExaBGP's object path, an UPDATE in Ze enters one receive buffer, is examined through views, shares canonical attributes with the routes already in the table, and is copied only when a destination needs different bytes or a longer lifetime. Changing language removed one ceiling. Most of the rest came from changing the representation and the ownership model, which a direct port would have carried over untouched.

The [second article in this series](../one-bgp-update-many-peers/) follows one received UPDATE out to a hundred peers, where these ownership rules decide how often Ze is allowed to encode the message.

The garbage collector still runs. Most UPDATEs have already reused their storage and moved on without bothering it much.
