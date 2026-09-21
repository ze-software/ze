---
title: How Ze reuses memory for BGP UPDATEs
date: 2026-08-04
author: Thomas Mangin
description: Go's garbage collector reclaims unused memory, but creating and copying data still has a cost. Ze's BGP UPDATE processing shows how we design to avoid that work.

deck: A garbage collector reclaims unused memory. We can still avoid creating much of that temporary data in the first place.

image: assets/blog/how-ze-manages-memory.svg
image-dark: assets/blog/how-ze-manages-memory-dark.svg
image-alt: Readers share a received BGP UPDATE while it remains in the cache. Changed messages and routes kept in the RIB use separate storage.

---

When I wrote [ExaBGP](https://github.com/Exa-Networks/exabgp), I wanted an ordinary program to be able to announce routes without implementing BGP itself. ExaBGP handled the protocol and converted received messages into text or JSON for the application. This made BGP available to those programs, but processing a full routing table involved creating Python objects for attributes repeated across many routes.

I paid attention to performance in ExaBGP, and moving to Go gave me an opportunity to reconsider how much data Ze would create and copy. Translating the same approach into a compiled language would still leave those repeated operations in place. Go's garbage collector reclaims memory the program no longer uses, but allocating temporary objects and later collecting them takes processing time too.

I wanted efficiency to be part of how we develop Ze, including decisions about memory that a garbage-collected language lets us overlook. Before adding an object or copying a message, we should be able to explain what that allocation or copy is for. BGP UPDATE processing is a useful place to apply this reasoning because an unnecessary operation can be repeated for every route.

Choosing a language with explicit memory management would leave us with the same design questions. Giving each reader its own copy makes ownership easier to reason about, but adds a copy for every reader. If readers can share unchanged data for a known period, we can avoid that cost in Go too. Efficient use of memory then depends on how we organise that sharing and reuse.

*This article was co-authored with Claude and revised with OpenAI Codex. The architecture and design decisions are mine. Claude helped with implementation, tests and measurements, and with organising and drafting the original text.*

## Use the data we already have

A BGP UPDATE arrives as a sequence of bytes containing route announcements or withdrawals, together with attributes such as the next hop and AS path. One way to process it would be to decode every attribute into a Go object, work with those objects, then encode them again when sending the route onwards. If the operation only uses the next hop, however, much of that decoding creates objects which serve no purpose.

We avoid creating those objects by keeping the received bytes in Ze's [WireUpdate](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/wire_update.go) and providing access to the attributes within them. Code inspecting the next hop can read that attribute without constructing a decoded representation of all the others. Several parts of Ze can also read the same stored message, so each reader does not require a separate copy. The input still has to be validated; the saving comes from how we represent it during processing.

When a message has to change before it is sent, we can avoid another temporary copy by writing the changed message directly into its output buffer. The caller supplies that storage to the encoder, instead of receiving a newly allocated message which it must then copy into place. This [buffer-first approach](https://github.com/ze-software/ze/blob/main/docs/architecture/buffer-architecture.md) makes the destination of the bytes part of the encoding operation.

## Reuse needs a clear stopping point

A buffer is an area of memory used to hold bytes, and we can reuse it for another message once processing has finished. Keeping available buffers in a pool therefore avoids allocating fresh storage for every message. Sharing those buffers between readers, as we do with `WireUpdate`, means we have to establish when the last reader has finished before returning one to the pool.

Garbage collection does not make this reuse safe. Returning a buffer to a pool leaves its underlying memory allocated, and a slice, a view of the bytes in that memory, can still refer to it. If another receive overwrites the buffer before that reader has finished, code expecting the old route will read a different message. The memory remains accessible throughout; its contents have changed too soon.

To prevent this, Ze has explicit [rules for how long readers can use shared bytes](https://github.com/ze-software/ze/blob/main/docs/architecture/memory/lifetime-contracts.md). An event handler, for example, can inspect a received UPDATE during its call, but retaining the message afterwards requires a snapshot in independent storage. The handler takes that copy before returning, so later processing can continue after the original buffer has been reused. This is a copy with a reason to exist.

A pool also occupies memory while its buffers sit unused. Keeping enough storage for the busiest moment indefinitely would reduce later allocations at the cost of retaining that memory during quieter periods, so Ze limits how much pool capacity it keeps. Fresh allocations can still occur when pooled storage is unavailable. These limits bound the storage retained by pools, rather than the daemon's total memory use.

## Store repeated route attributes once

Reusing message buffers reduces temporary allocation, but the routing information base, or RIB, retains routes long after their receive buffers have been reused. Copying every incoming packet would preserve the data, along with bytes the RIB no longer uses. Copying each route's attributes separately would avoid retaining whole packets but still store repeated values many times.

Many routes share an AS path or the same communities, which are values operators attach to routes for policy. Ze's [RIB storage](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/attrparse.go) takes advantage of that repetition by storing identical values for common attributes once. The first occurrence is copied into attribute storage, and later routes refer to that copy for as long as they use the value. This technique is called interning.

Because the sharing happens for each attribute, two routes with different preferences can still share the same AS path. A change to one attribute does not require another copy of all the others. The RIB retains the information independently of the receive buffer, while sharing the values which would otherwise occupy memory repeatedly across the routing table.

## Apply the same approach to text

Reusing the binary data does not avoid the cost of formatting it for an application. Ze still produces text, as ExaBGP does, and converting each route value into a string before joining those strings introduces another set of temporary allocations.

We can avoid those intermediate strings by building the result in one buffer. An address followed by a separator and a number, for example, can be written directly into consecutive bytes, without first turning the number into a separate string. Once the result has been consumed, the same buffer can be reset and used for the next value.

Ze's [`textbuf.Buffer`](https://github.com/ze-software/ze/blob/main/internal/core/textbuf/textbuf.go) provides these operations and includes 128 bytes of storage. A correctly initialised local buffer can therefore build short text without a separate heap allocation. Using the completed bytes immediately also avoids a copy of the final result.

If the text has to remain available after the buffer is reused, it must have independent storage, just as a retained UPDATE does. The caller can request an independent string: for short text, the implementation copies the built-in bytes; for text already held in a larger allocated array, it transfers that storage to the result. The [text buffer design](https://github.com/ze-software/ze/blob/main/docs/architecture/textbuf-string-building.md) explains which operation to use according to how long the result will be kept.

## Check that the intended saving exists

Choosing a design that avoids temporary storage does not establish how much memory its implementation allocates. Go's compiler can keep some values in a function's local storage while allocating others on the heap, and the result can change with the compiler. We therefore need to measure the compiled operations, even when the code appears to avoid allocation.

Ze's allocation assertions exercise repeated operations once their working storage is available, so they can detect a new temporary allocation on those paths. The [UPDATE rebuild tests](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_build_merge_test.go), for example, supply an available output buffer and check peer-specific attribute changes. They also check the output itself: the community-removal test verifies which values remain and their order, because an allocation saving would be useless if rebuilding the message changed its meaning.

Those checks apply to the operations they exercise. The complete forwarding path still allocates, including work around the rebuild, and session creation and growth of pools require memory too. Measuring a repeated operation separately lets us detect an avoidable allocation there without pretending that the entire daemon runs without allocating.

The original measurements published with this article used Go 1.26.5 on an Apple M4 Max running macOS on arm64, with allocation reporting and medians of five runs. These are retained historical microbenchmarks of the named operations, rather than a new measurement of the current tree.

The examples include BFD, the Bidirectional Forwarding Detection protocol, where Ze applies the same buffer reuse approach to control packets. The attribute measurements cover sharing a value already in the pool and reading it through a handle, the reference used to retrieve that stored value. Both operations reuse existing storage.

| Operation | Median time | Heap bytes | Heap allocations |
|---|---:|---:|---:|
| BFD packet encode and parse using a pool | 31.7 ns | 0 | 0 |
| Intern an existing attribute value | 145 ns | 0 | 0 |
| Read a stored attribute through its handle | 6.65 ns | 0 | 0 |

The [BFD benchmark](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/bench_test.go) acquires a packet buffer, writes and parses one unauthenticated control packet, then releases it. The [attribute-pool benchmarks](https://github.com/ze-software/ze/blob/main/internal/component/bgp/attrpool/benchmark_test.go) use the existing value `benchmark-data` for interning and handle lookup.

These results show that the named operations can reuse available storage without allocating. They do not measure storing a complete new route, TCP throughput or route convergence.

Reuse also adds development work: we have to define who owns the storage and when a reader must stop using it, as well as limit the capacity retained between uses. For a rarely used operation, that bookkeeping can cost more effort than the saving justifies. On a path repeated for each route, it lets us remove the same unnecessary work from every repetition.

*Last updated: 21 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/how-ze-manages-memory.md).*
