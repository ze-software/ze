---
title: How Ze reuses memory for BGP UPDATEs
date: 2026-08-04
author: Thomas Mangin
description: Moving ExaBGP's ideas to Go would still leave plenty of unnecessary copying. How I chose the representations and lifetimes for BGP data in Ze.

deck: An UPDATE has already arrived as bytes. Turning it into objects, then back into bytes, gives the program plenty to do before it has done anything useful with the route.

image: assets/blog/how-ze-manages-memory.svg
image-dark: assets/blog/how-ze-manages-memory-dark.svg
image-alt: One BGP UPDATE moves from pooled receive storage through borrowed read-only views; copy boundaries create peer-owned output and long-lived canonical attributes before each owner releases its storage.

---

When I wrote [ExaBGP](https://github.com/Exa-Networks/exabgp), the idea was much like the Common Gateway Interface used by web servers. CGI let an ordinary program produce a webpage without having to become an HTTP server, and ExaBGP let an ordinary program announce routes without having to become a BGP speaker. ExaBGP dealt with the protocol and passed useful data to the application.

Useful data, unfortunately, takes some work to produce. Transcoding BGP into text or JSON is expensive, and although I paid attention to performance from the start, large announcements and full-table decoding made Python's object costs hard to avoid. Creating a few objects for an attribute looks harmless until it happens for all the routes carrying that attribute.

Go gave Ze more room, but translating the same operations into Go would still leave it doing the same unnecessary copying. The UPDATE has arrived as bytes already. Building a complete collection of objects and then turning them back into bytes is a rather roundabout way to forward a route, especially if the operation only needed to inspect its next hop.

So I kept the received bytes as a representation Ze could use directly. That saves the decoding and copying which a reader does not need, but leaves a less convenient problem: several readers can use the same buffer, and eventually someone has to be allowed to overwrite it.

*This article was co-authored with Claude and revised with OpenAI Codex. The architecture and design decisions are mine. Claude helped with implementation, tests and measurements, and with organising and drafting the original text.*

## The bytes are already there

The [session read loop](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_read.go) gets reusable storage, reads the BGP header, checks the message length and fills the rest of the buffer. Standard messages fit within 4,096 octets. A speaker advertising the [BGP Extended Message Capability](https://www.rfc-editor.org/rfc/rfc8654.html) must be able to receive up to 65,535 octets, except for OPEN and KEEPALIVE, so Ze uses 4 KiB and 64 KiB receive storage.

Once receive-side validation has finished, the UPDATE is available as an immutable wire representation. Its readers can take views of attributes and prefixes, and iterate over the values they need. There is no requirement to build a second, fully decoded route first, which is the point of the [buffer-first architecture](https://github.com/ze-software/ze/blob/main/docs/architecture/buffer-architecture.md).

For example, reading the next hop should not manufacture community objects along the way. Both attributes are present in the packet and the communities can stay there until something has a use for them. This sounds obvious when put like that, but a convenient decoding API can make the extra work almost invisible to its caller.

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

A Go slice makes the view cheap: it refers to the existing buffer without copying the payload, and the garbage collector keeps the backing array alive. Unfortunately, keeping an array alive says nothing about which UPDATE is in it. If its owner returns the buffer to a pool and the next read overwrites it, the old slice will quite happily show the new packet.

There is no invalid memory access for Go to complain about. The memory is still there, and so is the reference, but the program is now reading someone else's route. This is why a garbage collector does not remove the need to decide who owns a reusable buffer and when readers must stop using it.

In Ze, the [recent UPDATE cache takes ownership before event delivery](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_notify.go). Normally it releases the storage after consumers have acknowledged the UPDATE and counted retentions have ended. A structured-event consumer which needs the bytes after its delivery call can take an owned [WireUpdate snapshot](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/wire_update.go); saving a slice does not extend the cache's lifetime protocol.

Even a counted retention has a limit. The cache has a safety valve for stalled entries which later completed entries have passed, because a consumer cannot be allowed to retain network storage indefinitely. If it is going to be slow, it needs bytes with an owner prepared to wait for it.

## Some copies are worth keeping

Making a copy for every consumer would get rid of some of this bookkeeping. It would also make all the readers which finish immediately pay for the ones which do not. I prefer borrowing for the former and an explicit ownership change for the latter, with the reason for a copy apparent at the place it happens.

Forwarding is a good example. If no policy edit, encoding conversion, path-identifier rewrite or message split is needed, [the body builder](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_body.go) can give the destination's send worker a view of the original body. The cache supplies its lifetime. Matching encoding contexts is one of the conditions, but it does not by itself mean the destination can receive the message unchanged.

When policy changes a body, later destinations may still need the original, so the result goes into separate storage. Ze records the edits, calculates the exact size and [writes the result into the destination peer's output buffer](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_build.go). Unchanged sections go straight from the input to that buffer alongside the new values. Copying the whole thing into temporary storage first would add another copy without helping either owner.

Each established destination has [64 output buffers](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_pool.go), cut from one backing array at its negotiated message size. A rebuilt send owns its buffer until its worker has finished with it. If another peer needs exactly the same result, Ze copies those bytes into that peer's buffer rather than rebuilding them, and the sends can finish independently. I kept that last copy deliberately; [One BGP UPDATE, many peers](../one-bgp-update-many-peers/) explains the comparison which led to the choice.

The slow destination is where borrowing becomes less attractive. A send may wait because the destination is congested, or because its initial route announcements have to leave first. Items entering the overflow queue [take an owned copy](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_body.go), even if the normal channel path could have borrowed the original body. Otherwise the queue could outlive the cache's safety valve and eventually transmit whatever a later receive had put in the buffer.

Overflow-pool storage supplies the copy when suitable space is available, and Ze allocates when it is not. I am quite happy to lose an allocation saving here. Sending the wrong route would be an absurd price to pay for keeping a counter at zero.

## A pool can also waste memory

Receive storage, peer output and overflow have different lifetimes, so they have separate pools. This also lets their capacity and exhaustion behaviour follow their users, rather than pretending that a receive buffer and a body waiting behind a congested peer are interchangeable demands.

A pool keeps memory around because the next operation may need it. That is useful during repeated traffic, but retaining everything the daemon has ever needed would be expensive, so the pools have budgets. Those budgets limit pooled capacity; allocation fallbacks mean they are not a fixed ceiling for the daemon's total memory use.

The demand estimate uses locally configured prefix maximums, summed across address families. These are the operator's limits, rather than a memory allowance negotiated with the other speaker. Before the initial table's [End-of-RIB markers](https://www.rfc-editor.org/rfc/rfc4724.html), the [calculation](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_pool_weight.go) uses the full prefix allowance, and afterwards it drops to a smaller burst allowance. The shared overflow budget also reserves for restart demand, as that large table may arrive again and have to leave through several destinations.

More memory gives a slow peer time to catch up, but it does not fix a peer which cannot keep up. The [forward congestion design](https://github.com/ze-software/ze/blob/main/docs/architecture/forward-congestion-pool.md) includes pressure controls and eventual session teardown. Continually growing the queue would only postpone dealing with the problem.

For temporary storage which is cheap to recreate, Go's [`sync.Pool`](https://pkg.go.dev/sync#Pool) is enough. It may discard a cached item, so its users have to work just as correctly with a fresh allocation. Ze's [Bidirectional Forwarding Detection packet pool](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/pool.go) is a small example: a fixed-capacity buffer suits the packet, the encoder writes into it, and the buffer goes back after use.

## Keeping a route after the packet has gone

The RIB cannot borrow the incoming packet for the lifetime of a route. It may need the route long after the receive buffer has been recycled, and retaining whole packets would also keep bytes it no longer needs. Copying every route's complete set of attributes would solve the lifetime problem, but leave all the repeated AS paths and communities occupying repeated storage.

Ze's RIB plugin therefore [interns attributes individually](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/storage/attrparse.go). Two routes which differ only in their Multi-Exit Discriminator can share their AS path and communities. One different value does not force everything else to be duplicated with it.

On the first occurrence, the [attribute pool](https://github.com/ze-software/ze/blob/main/internal/component/bgp/attrpool/pool.go) copies the value and returns a 32-bit handle. An equal value found later increments the existing reference count and gets the same handle. A live handle gives access to the canonical bytes, and releasing the last reference makes the slot available for reclamation. Here the first copy is doing something useful: route storage no longer depends on receive storage.

This sharing has its own housekeeping. Busy pools are partitioned by content hash, while attribute types with few distinct values use fewer partitions. Released values leave holes, so compaction moves live entries in batches and access continues through live handles. The [pool architecture](https://github.com/ze-software/ze/blob/main/docs/architecture/pool-architecture.md) has the details of those handles and moves.

Getting one of these lifetimes wrong can leave plausible-looking data in place until a later reuse, which is an unpleasant way for a bug to hide. With the `debug` build tag, [receive-buffer release](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session.go) overwrites returned bytes with `DE AD BE EF`. Attribute slots are poisoned too, and their slot numbers are not reused in debug builds, so a stale handle continues to fail as a dead slot.

The poison can itself be overwritten on reuse, and refusing to reuse slot numbers consumes extra capacity during a long debug run. These checks help expose mistakes; they cannot make a released view safe. The [lifetime contracts](https://github.com/ze-software/ze/blob/main/docs/architecture/memory/lifetime-contracts.md) describe those limits as well as the release rules.

## Let the caller supply the destination

There is little point being careful about copies in forwarding if every encoder first creates a byte sequence which its caller then copies into the network buffer. Ze's buffer-first encoders take caller-owned storage, and the UPDATE builder sizes its result before writing retained and replacement sections there. The caller already knows where the bytes need to end up.

Text formatting has much the same problem. Go's general formatter does useful work with dynamic formats, widths, precision and custom types, but many repeated operations in Ze already know the layout and the types. Returning a new string also gives the result independent storage even when it will be consumed immediately.

The [Zig formatting case study](https://ziglang.org/documentation/master/#Case-Study-print-in-Zig) shows how compile-time knowledge of the format and argument types can produce specialised writes. Go needs a different approach. Ze's [`textbuf.Buffer`](https://github.com/ze-software/ze/blob/main/internal/core/textbuf/textbuf.go) has typed append operations for values such as integers and addresses, a 128-byte inline array, and the escape-analysis technique also used by [`strings.Builder`](https://go.dev/src/strings/builder.go).

A local buffer initialised with `Reset` can build a result within that capacity without acquiring heap-backed result storage. What happens next depends on the method the caller chooses. `Slice` freezes the buffer and returns a borrowed string view which remains valid until reset or release. `String` returns owned text and empties the buffer: inline data is copied to survive reuse, while heap-backed data can transfer its storage.

I prefer an API which makes that choice visible. Code which consumes the result immediately can borrow it, and code which retains it can ask for ownership, without both being charged for a new string by default. The [text buffer design](https://github.com/ze-software/ze/blob/main/docs/architecture/textbuf-string-building.md) and [tests](https://github.com/ze-software/ze/blob/main/internal/core/textbuf/textbuf_test.go) cover the exact lifetimes.

## Go still has a garbage collector

Rust developers will recognise these ownership concerns, with the important difference that Go has no borrow checker to enforce Ze's release rules. The [Rust ownership guide](https://doc.rust-lang.org/book/ch04-01-what-is-ownership.html) and [Go garbage collector guide](https://go.dev/doc/gc-guide) explain their respective memory models. All of Ze's backing arrays remain on Go's managed heap.

I do not consider manual memory management a performance result by itself. If a program keeps cloning data to regain sole ownership, or carries several decoded versions of the same information, it still pays for those choices. A garbage-collected program which reuses storage can create very little new garbage on a repeated path. The language does not decide how many representations the programmer asks it to maintain.

Xavier Leroy discusses memory management and performance in his [interview about OCaml and systems programming](https://www.youtube.com/watch?v=9Cswiqrq6So). Uncertain ownership can encourage defensive copying, and reclaiming memory manually does not make those copies free. This is why I spent time on who needs independent storage in Ze, before treating each allocation as something to remove.

There are allocation assertions for repeated operations once their working storage is available. For UPDATE rebuilding, the [tests](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_build_merge_test.go) cover peer-specific attribute changes with a free output buffer. The community-removal case also checks that the requested values disappear and the others keep their order, which is rather more useful than a zero-allocation encoder producing the wrong communities. The wider [forwarding path](https://github.com/ze-software/ze/tree/main/internal/component/bgp/reactor) still allocates around the rebuild, as do session creation, maps, external serialisation and cold pool growth.

The original measurements published with this article used Go 1.26.5 on an Apple M4 Max running macOS on arm64, with allocation reporting and medians of five runs. These are retained historical microbenchmarks of the named operations, rather than a new measurement of the current tree.

| Operation | Median time | Heap bytes | Heap allocations |
|---|---:|---:|---:|
| BFD pooled encode and parse round trip | 31.7 ns | 0 | 0 |
| Intern an existing attribute value | 145 ns | 0 | 0 |
| Read a live attribute handle | 6.65 ns | 0 | 0 |

The [BFD benchmark](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/bench_test.go) acquires a packet buffer, writes and parses one unauthenticated control packet, then releases it. The [attribute-pool benchmarks](https://github.com/ze-software/ze/blob/main/internal/component/bgp/attrpool/benchmark_test.go) use the existing value `benchmark-data` for interning and handle lookup. They do not include a new route's complete attributes, TCP throughput or route convergence, and another compiler can move the timings. The zeroes cover these small operations with storage available; they do not turn Ze into a daemon which never allocates.

*Last updated: 11 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/how-ze-manages-memory.md).*
