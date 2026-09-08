---
title: How Ze reuses memory for BGP UPDATEs
date: 2026-08-04
author: Thomas Mangin
description: Following one BGP UPDATE through receive-buffer reuse, borrowed views, long-lived attributes and independently owned peer output.

deck: A borrowed slice can keep memory alive and still read the wrong UPDATE. Ze's allocation savings depend on knowing when that memory may be reused.

image: assets/blog/how-ze-manages-memory.svg
image-dark: assets/blog/how-ze-manages-memory-dark.svg
image-alt: One BGP UPDATE moves from pooled receive storage through borrowed read-only views; copy boundaries create peer-owned output and long-lived canonical attributes before each owner releases its storage.

---

I wrote ExaBGP so ordinary processes could use BGP without managing its sessions or parsing its wire messages. That separation gave applications convenient data to work with, and I paid attention to performance from the start. Large announcements and full-table decoding still exposed the cost of Python's object model, because an allocation for one attribute becomes an allocation for that attribute on every route.

Writing Ze in Go changed the cost of those operations, but a direct translation would have kept much of the same data movement. I wanted the received bytes to remain useful through the whole processing path, with copies where another owner or another representation needed them. Following one UPDATE shows where that saves memory and where borrowing would be unsafe.

*This article was co-authored with Claude and revised with OpenAI Codex. The architecture and design decisions are mine. Claude helped with implementation, tests and measurements, and with organising and drafting the original text.*

## One UPDATE, one receive buffer

The [session read loop](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_read.go) obtains a buffer from the receive pool, reads the BGP header into it, checks the message length and fills the remaining bytes. Standard BGP messages fit within 4,096 octets. A speaker which advertises the [BGP Extended Message Capability](https://www.rfc-editor.org/rfc/rfc8654.html) must be able to receive up to 65,535 octets, except for OPEN and KEEPALIVE messages. Ze uses 4 KiB and 64 KiB receive storage for those limits.

After receive-side validation, the UPDATE used by consumers is an immutable wire representation. Readers can take slices of its path attributes and prefixes, then use iterators to examine the values they need. The [buffer-first architecture](https://github.com/ze-software/ze/blob/main/docs/architecture/buffer-architecture.md) avoids constructing a second, fully decoded route merely to inspect it.

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

A slice over the buffer copies no payload bytes. Go's garbage collector keeps the backing array alive while a slice still refers to it, so the array cannot disappear underneath the reader. What a view cannot survive is reuse. Once the owner returns that buffer, another read may fill it with a different UPDATE, and the old slice will read those new bytes without any complaint from Go.

The [recent UPDATE cache takes ownership before event delivery](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_notify.go). It normally releases storage after consumers have acknowledged the UPDATE and counted retentions have ended. That is a lifetime protocol between components; holding a Go slice does not participate in it. A structured-event consumer which needs bytes beyond its delivery call can take an owned [WireUpdate snapshot](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/wire_update.go).

The cache also has a safety valve for stalled entries which later completed entries have passed. A counted retention therefore does not promise that network storage can be held indefinitely. The consequence for slow sends appears further down, where an UPDATE leaves the receive side's lifetime behind.

## When the route outlives its message

A routing information base, or RIB, may keep a route long after receive storage has been returned. Many routes have equal attribute values, so copying a complete set into every route would retain the same bytes repeatedly.

Ze's RIB plugin [interns individual attributes](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/attrparse.go). Interning stores a canonical value and gives its users a compact handle. Two routes with different Multi-Exit Discriminator values can still share their equal AS path and communities; changing one attribute does not force a duplicate of every other value.

The [attribute pool](https://github.com/ze-software/ze/blob/main/internal/component/bgp/attrpool/pool.go) copies a value on its first occurrence. A later equal value increments a reference count and returns a 32-bit handle to the existing storage. Reading a live handle borrows the canonical bytes, and releasing the final reference makes that slot available for reclamation. The RIB has acquired its own lifetime without keeping the incoming packet alive.

Busy attribute pools are partitioned by a content hash, while types with few distinct values use fewer partitions. Released values leave holes in the backing storage, and the compaction scheduler moves live entries in batches while retaining access through live handles. The [pool architecture](https://github.com/ze-software/ze/blob/main/docs/architecture/pool-architecture.md) describes the handle and compaction rules in more detail.

These rules still need diagnostics. With the `debug` build tag, [receive-buffer release](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session.go) overwrites returned bytes with the repeating pattern `DE AD BE EF`. Attribute slots are also poisoned, and debug builds refuse to reuse their slot numbers so a stale handle continues to fail as a dead slot. This helps expose a lifetime error during development; it does not make a borrowed slice safe after release, and the poison can itself be overwritten by later reuse. The [lifetime contracts](https://github.com/ze-software/ze/blob/main/docs/architecture/memory/lifetime-contracts.md) record those limits, including the extra slot capacity consumed by debug builds.

## From borrowed input to peer output

Forwarding has two different cases. If no policy change, encoding conversion, path-identifier rewrite or message split is required, [the forwarding body builder](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_body.go) can pass a view of the original body to the destination's send worker. That borrowing continues through the send, with the cache responsible for the source bytes.

A changed body needs another owner because later destinations may still need the original input. Ze records the changes for one destination, calculates the resulting size and [writes into that peer's output storage](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_build.go). Unchanged sections can be copied directly from the input; a replacement value need not pass through an intermediate whole-message buffer.

Each established destination has [64 output buffers](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_pool.go) cut from one backing array at its negotiated message size. A rebuilt send owns its buffer until the worker has finished with it. If another peer needs the same rebuilt bytes, Ze copies them into that peer's own buffer rather than giving both sends ownership of one mutable allocation. [One BGP UPDATE, many peers](../one-bgp-update-many-peers/) explains the comparison which permits that reuse.

A destination can fall behind, or need forwarded traffic held until its initial route announcements have been sent. Items entering the overflow queue [take an owned copy of their bodies](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_body.go), including bodies which the normal channel path would borrow. Otherwise a delayed send could outlive the cache's safety valve and transmit bytes from a recycled receive buffer. The copy uses overflow-pool storage when it fits; if that storage is unavailable, it allocates rather than risk sending another UPDATE's bytes.

## Capacity has an operational cost

Receive storage, modified output and queued overflow have separate pools because they have different release points. Their byte budgets limit retained pooled capacity, but those budgets are not a claim that the whole daemon has a fixed memory ceiling. Exhausted output pools and oversized overflow bodies can still fall back to heap allocation.

The capacity calculation also has to distinguish a peer sending its initial table from one sending occasional changes. Ze uses the locally configured prefix maximums, summed across address families, to estimate that demand. These are operator limits, rather than a memory allowance advertised by the remote peer.

The peer's initial routing updates end with an [End-of-RIB marker](https://www.rfc-editor.org/rfc/rfc4724.html). Ze's [demand calculation](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_pool_weight.go) uses the full prefix allowance before that stage and a smaller burst allowance afterwards. The shared overflow budget also reserves for restart demand, when a large set of routes may arrive again and need forwarding to several destinations.

The [forward congestion design](https://github.com/ze-software/ze/blob/main/docs/architecture/forward-congestion-pool.md) describes backpressure and session teardown under sustained pressure. A pool buys time for a slow consumer, and continued congestion eventually needs an operational response. It cannot be solved by retaining more memory without limit.

## The evidence stops at the measured path

The allocation checks cover repeated operations with their working storage already available. Ze's buffers still live on Go's managed heap, and session state, maps, external serialisation and cold pool growth still allocate.

For UPDATE rebuilding, [the allocation tests](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_build_merge_test.go) cover peer-specific attribute changes with a free output buffer. The community-removal case checks that the requested values disappear, the others survive in order and the warm rebuild allocates no heap objects. This is evidence about the rebuild, rather than the complete receive-to-TCP path.

The [attribute-pool benchmarks](https://github.com/ze-software/ze/blob/main/internal/component/bgp/attrpool/benchmark_test.go) isolate finding an already interned value and reading a live handle. The original measurements published with this article used Go 1.26.5 on an Apple M4 Max running macOS on arm64, with medians of five runs and allocation reporting. Both fixtures use the existing value `benchmark-data`, so these figures do not measure parsing or interning a new route's complete attributes.

| Operation | Median time | Heap bytes | Heap allocations |
|---|---:|---:|---:|
| Intern an existing value | 145 ns | 0 | 0 |
| Read a live attribute handle | 6.65 ns | 0 | 0 |

Those historical microbenchmarks say nothing about TCP throughput or route convergence, and compiler changes can alter their timings. Their useful scope is the ownership boundary they exercise. An UPDATE can leave receive storage while its route remains in the RIB, and a rebuilt send can finish without waiting for another peer to release its output. Following those owners is how I decide which copies Ze needs to keep.

*Last updated: 8 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/how-ze-manages-memory.md).*
